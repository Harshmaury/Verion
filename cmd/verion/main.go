package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Harshmaury/verion/internal/auth"
	"github.com/Harshmaury/verion/internal/crypto"
	"github.com/Harshmaury/verion/internal/crypto/local"
	"github.com/Harshmaury/verion/internal/identity"
	"github.com/Harshmaury/verion/internal/identity/postgres"
	"github.com/Harshmaury/verion/internal/store"
	grpctransport "github.com/Harshmaury/verion/internal/transport/grpc"
	transporthttp "github.com/Harshmaury/verion/internal/transport/http"
)

func main() {
	ctx := context.Background()

	grpcAddr     := envOrDefault("VERION_GRPC_ADDR", ":50051")
	httpAddr     := envOrDefault("VERION_HTTP_ADDR", ":8080")
	masterKeyHex := mustEnv("VERION_MASTER_KEY")

	masterKey, err := decodeHexKey(masterKeyHex)
	if err != nil { log.Fatalf("VERION_MASTER_KEY: %v", err) }

	// ── Database ──────────────────────────────────────────────────────────────
	dbCfg := postgres.DefaultConfig()
	if v := os.Getenv("VERION_DB_HOST");     v != "" { dbCfg.Host = v }
	if v := os.Getenv("VERION_DB_PORT");     v != "" { if p, e := strconv.Atoi(v); e == nil { dbCfg.Port = p } }
	if v := os.Getenv("VERION_DB_NAME");     v != "" { dbCfg.Database = v }
	if v := os.Getenv("VERION_DB_USER");     v != "" { dbCfg.User = v }
	if v := os.Getenv("VERION_DB_PASSWORD"); v != "" { dbCfg.Password = v }
	if v := os.Getenv("VERION_DB_SSLMODE");  v != "" { dbCfg.SSLMode = v }

	db, err := postgres.New(ctx, dbCfg)
	if err != nil { log.Fatalf("connect postgres: %v", err) }
	defer db.Close()
	slog.Info("✓ postgres connected")

	// ── Repositories ──────────────────────────────────────────────────────────
	auditRepo      := postgres.NewAuditRepo(db)
	keyRepo        := postgres.NewKeyRepo(db)
	tenantRepo     := postgres.NewTenantRepo(db)
	identityRepo   := postgres.NewIdentityRepo(db, auditRepo)
	credentialRepo := postgres.NewCredentialRepo(db)

	repos := identity.Repositories{
		Tenants:     tenantRepo,
		Identities:  identityRepo,
		Keys:        keyRepo,
		Credentials: credentialRepo,
		Audit:       auditRepo,
	}

	// ── Crypto ────────────────────────────────────────────────────────────────
	keyStore  := local.New()
	cryptoCfg := crypto.DefaultConfig(masterKey)
	cryptoSvc := crypto.New(cryptoCfg, keyStore)
	slog.Info("✓ crypto service ready (local keystore — dev only)")

	// ── Services ──────────────────────────────────────────────────────────────
	tenantSvc   := identity.NewTenantService(&repos)
	identitySvc := identity.NewIdentityService(&repos, cryptoSvc)
	keySvc      := identity.NewKeyService(&repos, cryptoSvc)

	// ── Redis ─────────────────────────────────────────────────────────────────
	redisStore, err := store.New(ctx, envOrDefault("VERION_REDIS_URL", "redis://:verion_redis_secret@localhost:6379/0"))
	if err != nil { slog.Error("redis connection failed", "err", err); os.Exit(1) }
	defer redisStore.Close()
	slog.Info("✓ redis connected")

	// ── Session store ─────────────────────────────────────────────────────────
	sessionStore := store.NewSessionStore(redisStore.Client(), 24*time.Hour)
	slog.Info("✓ session store ready")

	// ── Token service ─────────────────────────────────────────────────────────
	tokenSvc, err := auth.NewTokenService(auth.DefaultTokenConfig())
	if err != nil { slog.Error("token service init failed", "err", err); os.Exit(1) }
	slog.Info("✓ token service ready")

	// ── WebAuthn ──────────────────────────────────────────────────────────────
	wauthnCfg := auth.WebAuthnConfig{
		RPID:          envOrDefault("VERION_WEBAUTHN_RPID", "localhost"),
		RPDisplayName: envOrDefault("VERION_WEBAUTHN_DISPLAY_NAME", "Verion"),
		RPOrigins:     []string{envOrDefault("VERION_WEBAUTHN_ORIGIN", "http://localhost:8080")},
	}
	wauthnSvc, err := auth.New(wauthnCfg, redisStore, identitySvc, keySvc, &repos)
	if err != nil { slog.Error("webauthn init failed", "err", err); os.Exit(1) }
	slog.Info("✓ webauthn service ready")

	wauthnHandler  := transporthttp.NewWebAuthnHandler(wauthnSvc, tokenSvc, sessionStore)
	loginHandler   := transporthttp.NewLoginHandler(wauthnSvc, tokenSvc, sessionStore)
	sessionHandler := transporthttp.NewSessionHandler(sessionStore)

	// ── gRPC ──────────────────────────────────────────────────────────────────
	grpcSrv := grpctransport.New(identitySvc, tenantSvc, keySvc)
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil { log.Fatalf("listen %s: %v", grpcAddr, err) }

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("gRPC server listening", "addr", grpcAddr)
		if err := grpcSrv.GRPCServer().Serve(lis); err != nil {
			slog.Error("gRPC server error", "err", err)
		}
	}()

	// ── HTTP gateway ──────────────────────────────────────────────────────────
	gw := transporthttp.New(httpAddr, identitySvc, tenantSvc, keySvc,
		tokenSvc, sessionStore, wauthnHandler, loginHandler, sessionHandler)

	go func() {
		slog.Info("HTTP gateway listening", "addr", httpAddr)
		if err := gw.Start(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP gateway error", "err", err)
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	<-quit
	slog.Info("shutdown signal received — draining...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := gw.Shutdown(shutCtx); err != nil {
		slog.Error("HTTP gateway shutdown error", "err", err)
	}
	grpcSrv.GRPCServer().GracefulStop()
	db.Close()
	slog.Info("✓ graceful shutdown complete")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" { log.Fatalf("required environment variable %s is not set", key) }
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
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
	case c >= '0' && c <= '9': return int(c - '0')
	case c >= 'a' && c <= 'f': return int(c-'a') + 10
	case c >= 'A' && c <= 'F': return int(c-'A') + 10
	}
	return -1
}
