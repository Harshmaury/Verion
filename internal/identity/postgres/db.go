package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct{ pool *pgxpool.Pool }

type Config struct {
	Host, Database, User, Password, SSLMode string
	Port                                    int
	MaxOpenConns, MaxIdleConns              int32
	ConnMaxLifetime                         time.Duration
}

func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		c.Host, c.Port, c.Database, c.User, c.Password, c.SSLMode)
}

func DefaultConfig() *Config {
	return &Config{Host: "localhost", Port: 5432, Database: "verion",
		User: "verion", Password: "verion_dev_secret", SSLMode: "disable",
		MaxOpenConns: 25, MaxIdleConns: 5, ConnMaxLifetime: 5 * time.Minute}
}

func New(ctx context.Context, cfg *Config) (*DB, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("postgres: parse config: %w", err)
	}
	poolCfg.MaxConns = cfg.MaxOpenConns
	poolCfg.MinConns = cfg.MaxIdleConns
	poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	poolCfg.BeforeAcquire = func(ctx context.Context, conn *pgx.Conn) bool {
		return conn.Ping(ctx) == nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping failed: %w", err)
	}
	return &DB{pool: pool}, nil
}

func (db *DB) Close()              { db.pool.Close() }
func (db *DB) Pool() *pgxpool.Pool { return db.pool }

// withTenantConn — acquires *pgxpool.Conn, sets RLS tenant context, calls fn.
func (db *DB) withTenantConn(ctx context.Context, tenantID string, fn func(*pgxpool.Conn) error) error {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("postgres: acquire connection: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID); err != nil {
		return fmt.Errorf("postgres: set tenant context: %w", err)
	}
	return fn(conn)
}

// withTenantTx — begins a transaction, sets RLS tenant context, calls fn,
// commits on success or rolls back on error.
func (db *DB) withTenantTx(ctx context.Context, tenantID string, fn func(pgx.Tx) error) error {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("postgres: acquire connection: %w", err)
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin transaction: %w", err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("postgres: set tenant context in tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool { return containsPGCode(err, "23505") }
func isNotFound(err error) bool        { return err != nil && err.Error() == "no rows in result set" }

func containsPGCode(err error, code string) bool {
	type pgErr interface{ SQLState() string }
	type unwrapper interface{ Unwrap() error }
	for e := err; e != nil; {
		if pg, ok := e.(pgErr); ok {
			return pg.SQLState() == code
		}
		if u, ok := e.(unwrapper); ok {
			e = u.Unwrap()
		} else {
			break
		}
	}
	return false
}
