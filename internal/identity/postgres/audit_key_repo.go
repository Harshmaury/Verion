package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Harshmaury/verion/internal/identity"
)

// ═════════════════════════════════════════════════════════════════════════════
// AuditRepo — append-only audit log
// ═════════════════════════════════════════════════════════════════════════════

const sqlAuditInsert = `
	INSERT INTO audit_events
		(tenant_id,event_type,entity_type,entity_id,
		 actor_id,actor_type,ip_address,user_agent,
		 session_id,request_id,before_state,after_state,
		 metadata,success,error_code,occurred_at)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	RETURNING id`

type AuditRepo struct{ db *DB }

func NewAuditRepo(db *DB) *AuditRepo { return &AuditRepo{db: db} }

func (r *AuditRepo) Insert(ctx context.Context, event *identity.AuditEvent) error {
	return r.db.withTenantConn(ctx, event.TenantID, func(conn *pgxpool.Conn) error {
		return r.insertConn(ctx, conn, event)
	})
}

func (r *AuditRepo) insertConn(ctx context.Context, conn *pgxpool.Conn, event *identity.AuditEvent) error {
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now()
	}
	if event.Metadata == nil {
		event.Metadata = []byte("{}")
	}
	return conn.QueryRow(ctx, sqlAuditInsert,
		event.TenantID, event.EventType, event.EntityType, event.EntityID,
		event.ActorID, event.ActorType, event.IPAddress, event.UserAgent,
		event.SessionID, event.RequestID, event.BeforeState, event.AfterState,
		event.Metadata, event.Success, event.ErrorCode, event.OccurredAt,
	).Scan(&event.ID)
}

func (r *AuditRepo) insertTx(ctx context.Context, tx pgx.Tx, event *identity.AuditEvent) error {
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now()
	}
	if event.Metadata == nil {
		event.Metadata = []byte("{}")
	}
	return tx.QueryRow(ctx, sqlAuditInsert,
		event.TenantID, event.EventType, event.EntityType, event.EntityID,
		event.ActorID, event.ActorType, event.IPAddress, event.UserAgent,
		event.SessionID, event.RequestID, event.BeforeState, event.AfterState,
		event.Metadata, event.Success, event.ErrorCode, event.OccurredAt,
	).Scan(&event.ID)
}

func (r *AuditRepo) GetByID(ctx context.Context, tenantID, id string) (*identity.AuditEvent, error) {
	q := `SELECT id,tenant_id,event_type,entity_type,entity_id,
	             actor_id,actor_type,ip_address,user_agent,
	             session_id,request_id,before_state,after_state,
	             metadata,success,error_code,occurred_at
	      FROM audit_events WHERE tenant_id=$1 AND id=$2`
	var event *identity.AuditEvent
	err := r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		e, err := scanAuditEvent(conn.QueryRow(ctx, q, tenantID, id))
		if err != nil {
			return err
		}
		event = e
		return nil
	})
	return event, err
}

func (r *AuditRepo) ListByEntity(ctx context.Context, tenantID, entityType, entityID string, limit, offset int) ([]*identity.AuditEvent, error) {
	q := `SELECT id,tenant_id,event_type,entity_type,entity_id,
	             actor_id,actor_type,ip_address,user_agent,
	             session_id,request_id,before_state,after_state,
	             metadata,success,error_code,occurred_at
	      FROM audit_events
	      WHERE tenant_id=$1 AND entity_type=$2 AND entity_id=$3
	      ORDER BY occurred_at DESC LIMIT $4 OFFSET $5`
	return r.queryEvents(ctx, tenantID, q, tenantID, entityType, entityID, limit, offset)
}

func (r *AuditRepo) ListByActor(ctx context.Context, tenantID, actorID string, limit, offset int) ([]*identity.AuditEvent, error) {
	q := `SELECT id,tenant_id,event_type,entity_type,entity_id,
	             actor_id,actor_type,ip_address,user_agent,
	             session_id,request_id,before_state,after_state,
	             metadata,success,error_code,occurred_at
	      FROM audit_events
	      WHERE tenant_id=$1 AND actor_id=$2
	      ORDER BY occurred_at DESC LIMIT $3 OFFSET $4`
	return r.queryEvents(ctx, tenantID, q, tenantID, actorID, limit, offset)
}

func (r *AuditRepo) ListByTenant(ctx context.Context, tenantID string, filter identity.AuditFilter) ([]*identity.AuditEvent, error) {
	q := `SELECT id,tenant_id,event_type,entity_type,entity_id,
	             actor_id,actor_type,ip_address,user_agent,
	             session_id,request_id,before_state,after_state,
	             metadata,success,error_code,occurred_at
	      FROM audit_events WHERE tenant_id=$1`
	args := []any{tenantID}
	i := 2
	if filter.EventType != nil {
		q += fmt.Sprintf(" AND event_type=$%d", i); args = append(args, *filter.EventType); i++
	}
	if filter.EntityType != nil {
		q += fmt.Sprintf(" AND entity_type=$%d", i); args = append(args, *filter.EntityType); i++
	}
	if filter.Since != nil {
		q += fmt.Sprintf(" AND occurred_at>=$%d", i); args = append(args, *filter.Since); i++
	}
	if filter.Until != nil {
		q += fmt.Sprintf(" AND occurred_at<=$%d", i); args = append(args, *filter.Until); i++
	}
	q += " ORDER BY occurred_at DESC"
	if filter.Limit > 0 {
		q += fmt.Sprintf(" LIMIT $%d", i); args = append(args, filter.Limit); i++
	}
	if filter.Offset > 0 {
		q += fmt.Sprintf(" OFFSET $%d", i); args = append(args, filter.Offset)
	}
	return r.queryEvents(ctx, tenantID, q, args...)
}

func (r *AuditRepo) queryEvents(ctx context.Context, tenantID, q string, args ...any) ([]*identity.AuditEvent, error) {
	var events []*identity.AuditEvent
	err := r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		rows, err := conn.Query(ctx, q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			e, err := scanAuditEvent(rows)
			if err != nil {
				return err
			}
			events = append(events, e)
		}
		return rows.Err()
	})
	return events, err
}

func scanAuditEvent(row rowScanner) (*identity.AuditEvent, error) {
	var e identity.AuditEvent
	var actorType *string
	err := row.Scan(
		&e.ID, &e.TenantID, &e.EventType, &e.EntityType, &e.EntityID,
		&e.ActorID, &actorType, &e.IPAddress, &e.UserAgent,
		&e.SessionID, &e.RequestID, &e.BeforeState, &e.AfterState,
		&e.Metadata, &e.Success, &e.ErrorCode, &e.OccurredAt,
	)
	if err != nil {
		return nil, err
	}
	if actorType != nil {
		at := identity.ActorType(*actorType)
		e.ActorType = &at
	}
	return &e, nil
}

// ═════════════════════════════════════════════════════════════════════════════
// KeyRepo
// ═════════════════════════════════════════════════════════════════════════════

const (
	sqlKeyInsert = `
		INSERT INTO identity_keys
			(identity_id,tenant_id,key_type,purpose,algorithm,
			 public_key,public_key_jwk,key_ref,fingerprint,
			 status,valid_from,valid_until)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id,identity_id,tenant_id,key_type,purpose,algorithm,
		          public_key,public_key_jwk,key_ref,fingerprint,
		          status,valid_from,valid_until,rotated_at,rotated_to,created_at`

	sqlKeySelect = `SELECT id,identity_id,tenant_id,key_type,purpose,algorithm,
	                       public_key,public_key_jwk,key_ref,fingerprint,
	                       status,valid_from,valid_until,rotated_at,rotated_to,created_at
	                FROM identity_keys`

	sqlKeyRotate          = `UPDATE identity_keys SET status='rotated',rotated_at=now(),rotated_to=$3 WHERE tenant_id=$1 AND id=$2`
	sqlKeyRevoke          = `UPDATE identity_keys SET status='revoked' WHERE tenant_id=$1 AND id=$2`
	sqlKeyMarkCompromised = `UPDATE identity_keys SET status='compromised' WHERE tenant_id=$1 AND id=$2`
)

type KeyRepo struct{ db *DB }

func NewKeyRepo(db *DB) *KeyRepo { return &KeyRepo{db: db} }

func (r *KeyRepo) Create(ctx context.Context, key *identity.IdentityKey) (*identity.IdentityKey, error) {
	var result *identity.IdentityKey
	err := r.db.withTenantConn(ctx, key.TenantID, func(conn *pgxpool.Conn) error {
		row := conn.QueryRow(ctx, sqlKeyInsert,
			key.IdentityID, key.TenantID, string(key.KeyType), string(key.Purpose),
			key.Algorithm, key.PublicKey, key.PublicKeyJWK, key.KeyRef,
			key.Fingerprint, string(key.Status), key.ValidFrom, key.ValidUntil)
		k, err := scanKey(row)
		if err != nil {
			return fmt.Errorf("postgres: insert key: %w", err)
		}
		result = k
		return nil
	})
	return result, err
}

func (r *KeyRepo) GetByID(ctx context.Context, tenantID, id string) (*identity.IdentityKey, error) {
	return r.getKey(ctx, tenantID, sqlKeySelect+" WHERE tenant_id=$1 AND id=$2", tenantID, id)
}

func (r *KeyRepo) GetByFingerprint(ctx context.Context, tenantID, fp string) (*identity.IdentityKey, error) {
	return r.getKey(ctx, tenantID, sqlKeySelect+" WHERE tenant_id=$1 AND fingerprint=$2", tenantID, fp)
}

func (r *KeyRepo) GetActiveByPurpose(ctx context.Context, tenantID, identityID string, purpose identity.KeyPurpose) (*identity.IdentityKey, error) {
	return r.getKey(ctx, tenantID,
		sqlKeySelect+" WHERE tenant_id=$1 AND identity_id=$2 AND purpose=$3 AND status='active' LIMIT 1",
		tenantID, identityID, string(purpose))
}

func (r *KeyRepo) getKey(ctx context.Context, tenantID, q string, args ...any) (*identity.IdentityKey, error) {
	var result *identity.IdentityKey
	err := r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		k, err := scanKey(conn.QueryRow(ctx, q, args...))
		if err != nil {
			if isNotFound(err) {
				return identity.ErrKeyNotFound
			}
			return err
		}
		result = k
		return nil
	})
	return result, err
}

func (r *KeyRepo) ListByIdentity(ctx context.Context, tenantID, identityID string, status *identity.KeyStatus) ([]*identity.IdentityKey, error) {
	q := sqlKeySelect + " WHERE tenant_id=$1 AND identity_id=$2"
	args := []any{tenantID, identityID}
	if status != nil {
		q += " AND status=$3"
		args = append(args, string(*status))
	}
	q += " ORDER BY created_at DESC"
	var keys []*identity.IdentityKey
	err := r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		rows, err := conn.Query(ctx, q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			k, err := scanKey(rows)
			if err != nil {
				return err
			}
			keys = append(keys, k)
		}
		return rows.Err()
	})
	return keys, err
}

func (r *KeyRepo) Rotate(ctx context.Context, tenantID, oldKeyID, newKeyID string) error {
	return r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		_, err := conn.Exec(ctx, sqlKeyRotate, tenantID, oldKeyID, newKeyID)
		return err
	})
}

func (r *KeyRepo) Revoke(ctx context.Context, tenantID, keyID string) error {
	return r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		_, err := conn.Exec(ctx, sqlKeyRevoke, tenantID, keyID)
		return err
	})
}

func (r *KeyRepo) MarkCompromised(ctx context.Context, tenantID, keyID string) error {
	return r.db.withTenantConn(ctx, tenantID, func(conn *pgxpool.Conn) error {
		_, err := conn.Exec(ctx, sqlKeyMarkCompromised, tenantID, keyID)
		return err
	})
}

func scanKey(row rowScanner) (*identity.IdentityKey, error) {
	var k identity.IdentityKey
	var keyType, purpose, status, algorithm string
	var createdAt time.Time
	err := row.Scan(
		&k.ID, &k.IdentityID, &k.TenantID,
		&keyType, &purpose, &algorithm,
		&k.PublicKey, &k.PublicKeyJWK, &k.KeyRef, &k.Fingerprint,
		&status, &k.ValidFrom, &k.ValidUntil,
		&k.RotatedAt, &k.RotatedTo, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	k.KeyType = identity.KeyType(keyType)
	k.Purpose = identity.KeyPurpose(purpose)
	k.Status = identity.KeyStatus(status)
	k.Algorithm = algorithm
	k.CreatedAt = createdAt
	return &k, nil
}
