package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Harshmaury/verion/internal/identity"
)

const (
	sqlIdentityInsert = `
		INSERT INTO identities
			(tenant_id,type,display_name,handle,status,attributes,attributes_iv,created_by,version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,1)
		RETURNING id,tenant_id,type,display_name,handle,status,
		          primary_key_id,attributes,attributes_iv,
		          created_by,version,created_at,updated_at,deactivated_at`

	sqlIdentityGetByID = `
		SELECT id,tenant_id,type,display_name,handle,status,
		       primary_key_id,attributes,attributes_iv,
		       created_by,version,created_at,updated_at,deactivated_at
		FROM identities WHERE tenant_id=$1 AND id=$2`

	sqlIdentityGetByHandle = `
		SELECT id,tenant_id,type,display_name,handle,status,
		       primary_key_id,attributes,attributes_iv,
		       created_by,version,created_at,updated_at,deactivated_at
		FROM identities WHERE tenant_id=$1 AND handle=$2`

	sqlIdentityUpdate = `
		UPDATE identities
		SET display_name=$3,attributes=$4,attributes_iv=$5,version=version+1,updated_at=now()
		WHERE tenant_id=$1 AND id=$2 AND version=$6
		RETURNING id,tenant_id,type,display_name,handle,status,
		          primary_key_id,attributes,attributes_iv,
		          created_by,version,created_at,updated_at,deactivated_at`

	sqlIdentityUpdateStatus = `
		UPDATE identities
		SET status=$3,
		    deactivated_at=CASE WHEN $3 IN ('deactivated','archived') THEN now() ELSE deactivated_at END,
		    updated_at=now()
		WHERE tenant_id=$1 AND id=$2`

	sqlIdentitySetPrimaryKey = `
		UPDATE identities SET primary_key_id=$3,updated_at=now()
		WHERE tenant_id=$1 AND id=$2`

	sqlIdentityCount = `SELECT COUNT(*) FROM identities WHERE tenant_id=$1`
)

type IdentityRepo struct {
	db    *DB
	audit *AuditRepo
}

func NewIdentityRepo(db *DB, audit *AuditRepo) *IdentityRepo {
	return &IdentityRepo{db: db, audit: audit}
}

func (r *IdentityRepo) Create(ctx context.Context, ident *identity.Identity) (*identity.Identity, error) {
	var result *identity.Identity
	err := r.db.withTenantTx(ctx, ident.TenantID, func(tx pgx.Tx) error {
		attrs := ident.Attributes
		if attrs == nil {
			attrs = []byte{}
		}
		row := tx.QueryRow(ctx, sqlIdentityInsert,
			ident.TenantID, string(ident.Type), ident.DisplayName, ident.Handle,
			string(ident.Status), attrs, []byte{}, ident.CreatedBy)
		scanned, err := scanIdentity(row)
		if err != nil {
			if isUniqueViolation(err) {
				return identity.ErrHandleTaken
			}
			return fmt.Errorf("postgres: insert identity: %w", err)
		}
		result = scanned

		afterJSON, _ := json.Marshal(map[string]any{
			"id": result.ID, "type": result.Type,
			"display_name": result.DisplayName, "handle": result.Handle,
			"status": result.Status,
		})
		return r.audit.insertTx(ctx, tx, &identity.AuditEvent{
			TenantID: ident.TenantID, EventType: identity.AuditEventIdentityCreated,
			EntityType: "identity", EntityID: result.ID,
			ActorID: ident.CreatedBy, AfterState: afterJSON,
			Metadata: []byte("{}"), Success: true, OccurredAt: time.Now(),
		})
	})
	return result, err
}

func (r *IdentityRepo) GetByID(ctx context.Context, tenantID, id string) (*identity.Identity, error) {
	var result *identity.Identity
	err := r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		row := conn.QueryRow(ctx, sqlIdentityGetByID, tenantID, id)
		scanned, err := scanIdentity(row)
		if err != nil {
			if isNotFound(err) {
				return &identity.NotFoundError{EntityType: "identity", ID: id}
			}
			return fmt.Errorf("postgres: get identity: %w", err)
		}
		result = scanned
		return nil
	})
	return result, err
}

func (r *IdentityRepo) GetByHandle(ctx context.Context, tenantID, handle string) (*identity.Identity, error) {
	var result *identity.Identity
	err := r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		row := conn.QueryRow(ctx, sqlIdentityGetByHandle, tenantID, handle)
		scanned, err := scanIdentity(row)
		if err != nil {
			if isNotFound(err) {
				return &identity.NotFoundError{EntityType: "identity", ID: handle}
			}
			return fmt.Errorf("postgres: get identity by handle: %w", err)
		}
		result = scanned
		return nil
	})
	return result, err
}

func (r *IdentityRepo) Update(ctx context.Context, ident *identity.Identity) (*identity.Identity, error) {
	var result *identity.Identity
	err := r.db.withTenantConn(ctx, ident.TenantID, func(conn *pgxpool.Conn) error {
		row := conn.QueryRow(ctx, sqlIdentityUpdate,
			ident.TenantID, ident.ID, ident.DisplayName,
			ident.Attributes, []byte{}, ident.Version)
		scanned, err := scanIdentity(row)
		if err != nil {
			if isNotFound(err) {
				return &identity.VersionConflictError{EntityType: "identity", ID: ident.ID, ExpectedVersion: ident.Version}
			}
			return fmt.Errorf("postgres: update identity: %w", err)
		}
		result = scanned
		return nil
	})
	return result, err
}

func (r *IdentityRepo) UpdateStatus(ctx context.Context, tenantID, id string, status identity.IdentityStatus) error {
	return r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		_, err := conn.Exec(ctx, sqlIdentityUpdateStatus, tenantID, id, string(status))
		return err
	})
}

func (r *IdentityRepo) SetPrimaryKey(ctx context.Context, tenantID, identityID, keyID string) error {
	return r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		_, err := conn.Exec(ctx, sqlIdentitySetPrimaryKey, tenantID, identityID, keyID)
		return err
	})
}

func (r *IdentityRepo) List(ctx context.Context, tenantID string, filter identity.IdentityFilter) ([]*identity.Identity, error) {
	q := `SELECT id,tenant_id,type,display_name,handle,status,
	             primary_key_id,attributes,attributes_iv,
	             created_by,version,created_at,updated_at,deactivated_at
	      FROM identities WHERE tenant_id=$1`
	args := []any{tenantID}
	i := 2
	if filter.Type != nil {
		q += fmt.Sprintf(" AND type=$%d", i); args = append(args, string(*filter.Type)); i++
	}
	if filter.Status != nil {
		q += fmt.Sprintf(" AND status=$%d", i); args = append(args, string(*filter.Status)); i++
	}
	order := "created_at"
	if filter.OrderBy != "" {
		order = filter.OrderBy
	}
	dir := "ASC"
	if filter.Desc {
		dir = "DESC"
	}
	q += fmt.Sprintf(" ORDER BY %s %s", order, dir)
	if filter.Limit > 0 {
		q += fmt.Sprintf(" LIMIT $%d", i); args = append(args, filter.Limit); i++
	}
	if filter.Offset > 0 {
		q += fmt.Sprintf(" OFFSET $%d", i); args = append(args, filter.Offset)
	}
	var results []*identity.Identity
	err := r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		rows, err := conn.Query(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("postgres: list identities: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			ident, err := scanIdentity(rows)
			if err != nil {
				return err
			}
			results = append(results, ident)
		}
		return rows.Err()
	})
	return results, err
}

func (r *IdentityRepo) Count(ctx context.Context, tenantID string, filter identity.IdentityFilter) (int64, error) {
	var count int64
	err := r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		return conn.QueryRow(ctx, sqlIdentityCount, tenantID).Scan(&count)
	})
	return count, err
}

func scanIdentity(row rowScanner) (*identity.Identity, error) {
	var ident identity.Identity
	var idType, status string
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&ident.ID, &ident.TenantID, &idType, &ident.DisplayName, &ident.Handle, &status,
		&ident.PrimaryKeyID, &ident.Attributes, &ident.Attributes,
		&ident.CreatedBy, &ident.Version, &createdAt, &updatedAt, &ident.DeactivatedAt,
	)
	if err != nil {
		return nil, err
	}
	ident.Type = identity.IdentityType(idType)
	ident.Status = identity.IdentityStatus(status)
	ident.CreatedAt = createdAt
	ident.UpdatedAt = updatedAt
	return &ident, nil
}
