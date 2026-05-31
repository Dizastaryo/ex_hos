package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// AuditRepository writes & reads admin action audit rows.
type AuditRepository struct {
	db *DB
}

func NewAuditRepository(db *DB) *AuditRepository {
	return &AuditRepository{db: db}
}

// Log records one admin action. metadata is marshalled to JSONB; nil → '{}'.
//
// Errors are logged at the call site rather than propagated to the user — an
// audit-log failure should not block the action itself, but it is a soft bug
// the operator wants to know about.
func (r *AuditRepository) Log(
	ctx context.Context,
	adminID, action, targetType, targetID string,
	metadata map[string]any,
) error {
	var meta []byte
	if metadata == nil {
		meta = []byte("{}")
	} else {
		var err error
		meta, err = json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("marshal audit metadata: %w", err)
		}
	}

	// target_id is UUID — pass nil if empty so PG stores NULL instead of '' (cast error).
	var tid any
	if targetID != "" {
		tid = targetID
	}

	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO admin_audit_log (admin_id, action, target_type, target_id, metadata)
		VALUES ($1, $2, $3, $4, $5)`,
		adminID, action, targetType, tid, meta,
	)
	if err != nil {
		return fmt.Errorf("insert audit row: %w", err)
	}
	return nil
}

type AuditListFilter struct {
	AdminID    string
	Action     string
	TargetType string
	Limit      int
	Offset     int
}

// List returns audit rows newest-first, joined to the admin user for the UI.
// Each row is a map ready to be serialized to JSON.
func (r *AuditRepository) List(ctx context.Context, f AuditListFilter) ([]map[string]any, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}

	q := `
		SELECT a.id, a.action, a.target_type, a.target_id, a.metadata, a.created_at,
		       u.id, u.username, u.avatar_url
		FROM admin_audit_log a
		LEFT JOIN users u ON u.id = a.admin_id
		WHERE 1=1`
	args := []any{}
	if f.AdminID != "" {
		args = append(args, f.AdminID)
		q += fmt.Sprintf(" AND a.admin_id = $%d", len(args))
	}
	if f.Action != "" {
		args = append(args, f.Action)
		q += fmt.Sprintf(" AND a.action = $%d", len(args))
	}
	if f.TargetType != "" {
		args = append(args, f.TargetType)
		q += fmt.Sprintf(" AND a.target_type = $%d", len(args))
	}
	args = append(args, f.Limit, f.Offset)
	q += fmt.Sprintf(" ORDER BY a.created_at DESC LIMIT $%d OFFSET $%d",
		len(args)-1, len(args))

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("admin audit list: %w", err)
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var (
			id, action, targetType                  string
			targetID                                *string
			metaJSON                                []byte
			created                                 time.Time
			adminID, adminUsername, adminAvatar     *string
		)
		if err := rows.Scan(&id, &action, &targetType, &targetID, &metaJSON, &created,
			&adminID, &adminUsername, &adminAvatar); err != nil {
			return nil, fmt.Errorf("audit list scan: %w", err)
		}

		var meta map[string]any
		if len(metaJSON) > 0 {
			_ = json.Unmarshal(metaJSON, &meta)
		}
		if meta == nil {
			meta = map[string]any{}
		}

		row := map[string]any{
			"id":          id,
			"action":      action,
			"target_type": targetType,
			"target_id":   derefStr(targetID),
			"metadata":    meta,
			"created_at":  created,
			"admin": map[string]any{
				"id":         derefStr(adminID),
				"username":   derefStr(adminUsername),
				"avatar_url": derefStr(adminAvatar),
			},
		}
		out = append(out, row)
	}
	return out, nil
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
