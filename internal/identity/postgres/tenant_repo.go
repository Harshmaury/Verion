package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Harshmaury/verion/internal/identity"
)

const (
	sqlTenantInsert = `
		INSERT INTO tenants (name, slug, tier, status, settings, data_region)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id,name,slug,tier,status,settings,data_region,created_at,updated_at`

	sqlTenantGetByID = `
		SELECT id,name,slug,tier,status,settings,data_region,created_at,updated_at
		FROM tenants WHERE id=$1`

	sqlTenantGetBySlug = `
		SELECT id,name,slug,tier,status,settings,data_region,created_at,updated_at
		FROM tenants WHERE slug=$1`

	sqlTenantUpdate = `
		UPDATE tenants SET name=$2,tier=$3,settings=$4,data_region=$5,updated_at=now()
		WHERE id=$1
		RETURNING id,name,slug,tier,status,settings,data_region,created_at,updated_at`

	sqlTenantUpdateStatus = `
		UPDATE tenants SET status=$2,updated_at=now() WHERE id=$1`
)

type TenantRepo struct{ db *DB }

func NewTenantRepo(db *DB) *TenantRepo { return &TenantRepo{db: db} }

func (r *TenantRepo) Create(ctx context.Context, t *identity.Tenant) (*identity.Tenant, error) {
	settings := t.Settings
	if settings == nil {
		settings = []byte("{}")
	}
	// Tenants are not tenant-scoped — use pool directly
	row := r.db.pool.QueryRow(ctx, sqlTenantInsert,
		t.Name, t.Slug, string(t.Tier), string(t.Status), settings, t.DataRegion)
	result, err := scanTenant(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, identity.ErrAlreadyExists
		}
		return nil, fmt.Errorf("postgres: create tenant: %w", err)
	}
	return result, nil
}

func (r *TenantRepo) GetByID(ctx context.Context, id string) (*identity.Tenant, error) {
	row := r.db.pool.QueryRow(ctx, sqlTenantGetByID, id)
	t, err := scanTenant(row)
	if err != nil {
		if isNotFound(err) {
			return nil, &identity.NotFoundError{EntityType: "tenant", ID: id}
		}
		return nil, fmt.Errorf("postgres: get tenant: %w", err)
	}
	return t, nil
}

func (r *TenantRepo) GetBySlug(ctx context.Context, slug string) (*identity.Tenant, error) {
	row := r.db.pool.QueryRow(ctx, sqlTenantGetBySlug, slug)
	t, err := scanTenant(row)
	if err != nil {
		if isNotFound(err) {
			return nil, &identity.NotFoundError{EntityType: "tenant", ID: slug}
		}
		return nil, fmt.Errorf("postgres: get tenant by slug: %w", err)
	}
	return t, nil
}

func (r *TenantRepo) Update(ctx context.Context, t *identity.Tenant) (*identity.Tenant, error) {
	row := r.db.pool.QueryRow(ctx, sqlTenantUpdate,
		t.ID, t.Name, string(t.Tier), t.Settings, t.DataRegion)
	result, err := scanTenant(row)
	if err != nil {
		if isNotFound(err) {
			return nil, &identity.NotFoundError{EntityType: "tenant", ID: t.ID}
		}
		return nil, fmt.Errorf("postgres: update tenant: %w", err)
	}
	return result, nil
}

func (r *TenantRepo) UpdateStatus(ctx context.Context, id string, status identity.TenantStatus) error {
	_, err := r.db.pool.Exec(ctx, sqlTenantUpdateStatus, id, string(status))
	if err != nil {
		return fmt.Errorf("postgres: update tenant status: %w", err)
	}
	return nil
}

func (r *TenantRepo) List(ctx context.Context, filter identity.TenantFilter) ([]*identity.Tenant, error) {
	q := `SELECT id,name,slug,tier,status,settings,data_region,created_at,updated_at FROM tenants WHERE 1=1`
	args := []any{}
	i := 1
	if filter.Status != nil {
		q += fmt.Sprintf(" AND status=$%d", i); args = append(args, string(*filter.Status)); i++
	}
	if filter.Tier != nil {
		q += fmt.Sprintf(" AND tier=$%d", i); args = append(args, string(*filter.Tier)); i++
	}
	q += " ORDER BY created_at DESC"
	if filter.Limit > 0 {
		q += fmt.Sprintf(" LIMIT $%d", i); args = append(args, filter.Limit); i++
	}
	if filter.Offset > 0 {
		q += fmt.Sprintf(" OFFSET $%d", i); args = append(args, filter.Offset)
	}
	rows, err := r.db.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: list tenants: %w", err)
	}
	defer rows.Close()
	var tenants []*identity.Tenant
	for rows.Next() {
		t, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		tenants = append(tenants, t)
	}
	return tenants, rows.Err()
}

// ── scan helpers ──────────────────────────────────────────────────────────────

type rowScanner interface{ Scan(dest ...any) error }

func scanTenant(row rowScanner) (*identity.Tenant, error) {
	var t identity.Tenant
	var tier, status string
	var createdAt, updatedAt time.Time
	err := row.Scan(&t.ID, &t.Name, &t.Slug, &tier, &status,
		&t.Settings, &t.DataRegion, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	t.Tier = identity.TenantTier(tier)
	t.Status = identity.TenantStatus(status)
	t.CreatedAt = createdAt
	t.UpdatedAt = updatedAt
	return &t, nil
}

// conn interface subset used in withTenantConn callbacks — satisfies *pgxpool.Conn
var _ *pgxpool.Conn = (*pgxpool.Conn)(nil)
