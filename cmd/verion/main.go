package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Harshmaury/verion/internal/crypto"
	"github.com/Harshmaury/verion/internal/crypto/local"
	"github.com/Harshmaury/verion/internal/identity"
	"github.com/Harshmaury/verion/internal/identity/postgres"
	grpctransport "github.com/Harshmaury/verion/internal/transport/grpc"
)

func main() {
	ctx := context.Background()

	// ── 1. Config from environment ────────────────────────────────────────────
	grpcAddr     := envOrDefault("VERION_GRPC_ADDR", ":50051")
	masterKeyHex := mustEnv("VERION_MASTER_KEY")

	masterKey, err := decodeHexKey(masterKeyHex)
	if err != nil {
		log.Fatalf("VERION_MASTER_KEY: %v", err)
	}

	// ── 2. Database ───────────────────────────────────────────────────────────
	dbCfg := postgres.DefaultConfig()
	if v := os.Getenv("VERION_DB_HOST");     v != "" { dbCfg.Host = v }
	if v := os.Getenv("VERION_DB_PORT");     v != "" { if p, e := strconv.Atoi(v); e == nil { dbCfg.Port = p } }
	if v := os.Getenv("VERION_DB_NAME");     v != "" { dbCfg.Database = v }
	if v := os.Getenv("VERION_DB_USER");     v != "" { dbCfg.User = v }
	if v := os.Getenv("VERION_DB_PASSWORD"); v != "" { dbCfg.Password = v }
	if v := os.Getenv("VERION_DB_SSLMODE");  v != "" { dbCfg.SSLMode = v }

	db, err := postgres.New(ctx, dbCfg)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()
	log.Println("✓ postgres connected")

	// ── 3. Repositories ───────────────────────────────────────────────────────
	auditRepo    := postgres.NewAuditRepo(db)
	keyRepo      := postgres.NewKeyRepo(db)
	tenantRepo   := postgres.NewTenantRepo(db)
	identityRepo := postgres.NewIdentityRepo(db, auditRepo)

	repos := &identity.Repositories{
		Tenants:    tenantRepo,
		Identities: identityRepo,
		Keys:       keyRepo,
		Audit:      auditRepo,
	}

	// ── 4. Crypto service ─────────────────────────────────────────────────────
	keyStore  := local.New()
	cryptoCfg := crypto.DefaultConfig(masterKey)
	cryptoSvc := crypto.New(cryptoCfg, keyStore)
	log.Println("✓ crypto service ready (local keystore — dev only)")

	// ── 5. Service layer ──────────────────────────────────────────────────────
	tenantSvc   := identity.NewTenantService(repos)
	identitySvc := identity.NewIdentityService(repos, cryptoSvc)
	keySvc      := identity.NewKeyService(repos, cryptoSvc)

	// ── 6. gRPC server ────────────────────────────────────────────────────────
	grpcServer := grpctransport.New(identitySvc, tenantSvc, keySvc)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("listen %s: %v", grpcAddr, err)
	}

	// ── 7. Serve with graceful shutdown ───────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Printf("Verion gRPC server listening on %s\n", grpcAddr)
		if err := grpcServer.GRPCServer().Serve(lis); err != nil {
			log.Printf("grpc serve: %v", err)
		}
	}()

	<-quit
	log.Println("shutdown signal received — draining...")

	stopped := make(chan struct{})
	go func() {
		grpcServer.GRPCServer().GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		log.Println("✓ graceful shutdown complete")
	case <-time.After(30 * time.Second):
		log.Println("shutdown timeout — forcing stop")
		grpcServer.GRPCServer().Stop()
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func decodeHexKey(hex string) ([]byte, error) {
	if len(hex) != 64 {
		return nil, fmt.Errorf("must be 64 hex characters (32 bytes), got %d", len(hex))
	}
	b := make([]byte, 32)
	for i := 0; i < 32; i++ {
		hi := hexNibble(hex[i*2])
		lo := hexNibble(hex[i*2+1])
		if hi < 0 || lo < 0 {
			return nil, fmt.Errorf("invalid hex character at position %d", i*2)
		}
		b[i] = byte(hi<<4 | lo)
	}
	return b, nil
}

func hexNibble(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}
