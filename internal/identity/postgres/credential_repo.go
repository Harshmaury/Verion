package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Harshmaury/verion/internal/identity"
)

// CredentialRepo implements identity.CredentialRepository using PostgreSQL.
type CredentialRepo struct{ db *DB }

// NewCredentialRepo constructs a CredentialRepo.
func NewCredentialRepo(db *DB) *CredentialRepo { return &CredentialRepo{db: db} }

// Compile-time assertion.
var _ identity.CredentialRepository = (*CredentialRepo)(nil)

func (r *CredentialRepo) Create(ctx context.Context, cred *identity.Credential) (*identity.Credential, error) {
	err := r.db.withTenantConn(ctx, cred.TenantID, func(conn *pgxpool.Conn) error {
		query := `
			INSERT INTO credentials (
				identity_id, tenant_id, key_id, type, status,
				data, data_iv, aaguid, credential_id, sign_count,
				name, device_info, created_at
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW()
			)
			RETURNING id, created_at`
		return conn.QueryRow(ctx, query,
			cred.IdentityID, cred.TenantID, cred.KeyID,
			string(cred.Type), string(cred.Status),
			cred.Data, cred.DataIV, cred.AAGUID,
			cred.CredentialID, cred.SignCount,
			cred.Name, cred.DeviceInfo,
		).Scan(&cred.ID, &cred.CreatedAt)
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, identity.ErrAlreadyExists
		}
		return nil, fmt.Errorf("credential repo: create: %w", err)
	}
	return cred, nil
}

func (r *CredentialRepo) GetByID(ctx context.Context, tenantID, id string) (*identity.Credential, error) {
	var cred identity.Credential
	err := r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		return conn.QueryRow(ctx, `
			SELECT id, identity_id, tenant_id, key_id, type, status,
				data, data_iv, aaguid, credential_id, sign_count,
				name, device_info, last_used_at, created_at, expires_at
			FROM credentials WHERE id=$1 AND tenant_id=$2`,
			id, tenantID,
		).Scan(
			&cred.ID, &cred.IdentityID, &cred.TenantID, &cred.KeyID,
			&cred.Type, &cred.Status, &cred.Data, &cred.DataIV,
			&cred.AAGUID, &cred.CredentialID, &cred.SignCount,
			&cred.Name, &cred.DeviceInfo, &cred.LastUsedAt,
			&cred.CreatedAt, &cred.ExpiresAt,
		)
	})
	if err != nil {
		if isNotFound(err) {
			return nil, identity.ErrCredentialNotFound
		}
		return nil, fmt.Errorf("credential repo: get by id: %w", err)
	}
	return &cred, nil
}

func (r *CredentialRepo) GetWebAuthnByCredentialID(ctx context.Context, tenantID string, credentialID []byte) (*identity.Credential, error) {
	var cred identity.Credential
	err := r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		return conn.QueryRow(ctx, `
			SELECT id, identity_id, tenant_id, key_id, type, status,
				data, data_iv, aaguid, credential_id, sign_count,
				name, device_info, last_used_at, created_at, expires_at
			FROM credentials WHERE credential_id=$1 AND tenant_id=$2`,
			credentialID, tenantID,
		).Scan(
			&cred.ID, &cred.IdentityID, &cred.TenantID, &cred.KeyID,
			&cred.Type, &cred.Status, &cred.Data, &cred.DataIV,
			&cred.AAGUID, &cred.CredentialID, &cred.SignCount,
			&cred.Name, &cred.DeviceInfo, &cred.LastUsedAt,
			&cred.CreatedAt, &cred.ExpiresAt,
		)
	})
	if err != nil {
		if isNotFound(err) {
			return nil, identity.ErrCredentialNotFound
		}
		return nil, fmt.Errorf("credential repo: get webauthn by credential id: %w", err)
	}
	return &cred, nil
}

func (r *CredentialRepo) ListByIdentity(ctx context.Context, tenantID, identityID string, status *identity.CredentialStatus) ([]*identity.Credential, error) {
	query := `
		SELECT id, identity_id, tenant_id, key_id, type, status,
			data, data_iv, aaguid, credential_id, sign_count,
			name, device_info, last_used_at, created_at, expires_at
		FROM credentials WHERE identity_id=$1 AND tenant_id=$2`
	args := []any{identityID, tenantID}
	if status != nil {
		query += " AND status=$3"
		args = append(args, string(*status))
	}

	rows, err := r.db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("credential repo: list by identity: %w", err)
	}
	defer rows.Close()

	var creds []*identity.Credential
	for rows.Next() {
		var c identity.Credential
		if err := rows.Scan(
			&c.ID, &c.IdentityID, &c.TenantID, &c.KeyID,
			&c.Type, &c.Status, &c.Data, &c.DataIV,
			&c.AAGUID, &c.CredentialID, &c.SignCount,
			&c.Name, &c.DeviceInfo, &c.LastUsedAt,
			&c.CreatedAt, &c.ExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("credential repo: scan: %w", err)
		}
		creds = append(creds, &c)
	}
	return creds, rows.Err()
}

func (r *CredentialRepo) UpdateSignCount(ctx context.Context, tenantID, credentialID string, newCount int64) error {
	_, err := r.db.pool.Exec(ctx,
		`UPDATE credentials SET sign_count=$1 WHERE id=$2 AND tenant_id=$3`,
		newCount, credentialID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("credential repo: update sign count: %w", err)
	}
	return nil
}

func (r *CredentialRepo) UpdateLastUsed(ctx context.Context, tenantID, credentialID string, at time.Time) error {
	_, err := r.db.pool.Exec(ctx,
		`UPDATE credentials SET last_used_at=$1 WHERE id=$2 AND tenant_id=$3`,
		at, credentialID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("credential repo: update last used: %w", err)
	}
	return nil
}

func (r *CredentialRepo) Revoke(ctx context.Context, tenantID, credentialID string) error {
	_, err := r.db.pool.Exec(ctx,
		`UPDATE credentials SET status='revoked' WHERE id=$1 AND tenant_id=$2`,
		credentialID, tenantID,
	)
	if err != nil {
		return fmt.Errorf("credential repo: revoke: %w", err)
	}
	return nil
}
