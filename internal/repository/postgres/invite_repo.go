package postgres

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/seeu/backend/internal/domain"
)

type Invite struct {
	ID        string     `json:"id"`
	InviterID string     `json:"inviter_id"`
	Code      string     `json:"code"`
	UsedByID  *string    `json:"used_by_id,omitempty"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`

	// Populated when joined.
	Inviter *domain.UserShort `json:"inviter,omitempty"`
}

type InviteRepository struct {
	db *DB
}

func NewInviteRepository(db *DB) *InviteRepository {
	return &InviteRepository{db: db}
}

// generateCode produces a short, URL-friendly invite code (10 chars of base32 without padding).
func generateCode() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	c := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
	return strings.ToLower(c[:10]), nil
}

// Create creates a fresh invite code for a user. Tries up to 5 times in the
// astronomically unlikely event of a collision.
func (r *InviteRepository) Create(ctx context.Context, inviterID string) (*Invite, error) {
	var lastErr error
	for i := 0; i < 5; i++ {
		code, err := generateCode()
		if err != nil {
			return nil, fmt.Errorf("generate code: %w", err)
		}
		inv := &Invite{}
		err = r.db.Pool.QueryRow(ctx, `
			INSERT INTO invites (inviter_id, code)
			VALUES ($1, $2)
			RETURNING id, inviter_id, code, used_by_id, used_at, created_at`,
			inviterID, code,
		).Scan(&inv.ID, &inv.InviterID, &inv.Code, &inv.UsedByID, &inv.UsedAt, &inv.CreatedAt)
		if err == nil {
			return inv, nil
		}
		if isUniqueViolation(err) {
			lastErr = err
			continue
		}
		return nil, fmt.Errorf("create invite: %w", err)
	}
	return nil, fmt.Errorf("create invite: collision after 5 tries: %w", lastErr)
}

// LookupByCode returns the invite + the inviter user, or ErrNotFound.
// Does NOT mark the invite as used.
func (r *InviteRepository) LookupByCode(ctx context.Context, code string) (*Invite, error) {
	inv := &Invite{Inviter: &domain.UserShort{}}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT i.id, i.inviter_id, i.code, i.used_by_id, i.used_at, i.created_at,
		       u.id, u.username, u.full_name, u.avatar_url, u.is_verified
		FROM invites i
		JOIN users u ON u.id = i.inviter_id
		WHERE i.code = $1`, strings.ToLower(strings.TrimSpace(code)),
	).Scan(&inv.ID, &inv.InviterID, &inv.Code, &inv.UsedByID, &inv.UsedAt, &inv.CreatedAt,
		&inv.Inviter.ID, &inv.Inviter.Username, &inv.Inviter.FullName, &inv.Inviter.AvatarURL, &inv.Inviter.IsVerified)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("lookup invite: %w", err)
	}
	return inv, nil
}

// Claim marks an invite as used by a specific (just-registered) user.
// Idempotent: re-claiming the same invite by the same user is a no-op; claiming
// an already-used invite returns ErrAlreadyExists.
func (r *InviteRepository) Claim(ctx context.Context, code, newUserID string) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE invites
		SET used_by_id = $2, used_at = NOW()
		WHERE code = $1 AND used_by_id IS NULL`,
		strings.ToLower(strings.TrimSpace(code)), newUserID)
	if err != nil {
		return fmt.Errorf("claim invite: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Either no such invite, or already used by someone else.
		var existing string
		err := r.db.Pool.QueryRow(ctx, `SELECT used_by_id::text FROM invites WHERE code = $1`,
			code).Scan(&existing)
		if err != nil {
			return domain.ErrNotFound
		}
		if existing == newUserID {
			return nil
		}
		return domain.ErrAlreadyExists
	}
	return nil
}

// ListByInviter returns invites issued by a given user with their claim status,
// newest first.
func (r *InviteRepository) ListByInviter(ctx context.Context, inviterID string, limit, offset int) ([]*Invite, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, inviter_id, code, used_by_id, used_at, created_at
		FROM invites
		WHERE inviter_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, inviterID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	defer rows.Close()
	var out []*Invite
	for rows.Next() {
		i := &Invite{}
		if err := rows.Scan(&i.ID, &i.InviterID, &i.Code, &i.UsedByID, &i.UsedAt, &i.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan invite: %w", err)
		}
		out = append(out, i)
	}
	return out, nil
}
