package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// OTPRecord — minimal projection of `otp_codes` row used by AuthService.
type OTPRecord struct {
	ID        string
	Phone     string
	Code      string
	ExpiresAt time.Time
	Used      bool
	Attempts  int
	CreatedAt time.Time
}

type OTPRepository struct {
	db *DB
}

func NewOTPRepository(db *DB) *OTPRepository {
	return &OTPRepository{db: db}
}

// Insert stores a new OTP code with TTL = ttl. Returns the row id.
func (r *OTPRepository) Insert(ctx context.Context, phone, code string, ttl time.Duration) (string, error) {
	expiresAt := time.Now().Add(ttl)
	var id string
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO otp_codes (phone, code, expires_at)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		phone, code, expiresAt,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert otp: %w", err)
	}
	return id, nil
}

// LatestActive returns the most recent unused, non-expired code for phone.
// `pgx.ErrNoRows` is returned as a domain-friendly nil pair (caller checks).
func (r *OTPRepository) LatestActive(ctx context.Context, phone string) (*OTPRecord, error) {
	row := &OTPRecord{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, phone, code, expires_at, used, attempts, created_at
		 FROM otp_codes
		 WHERE phone = $1 AND used = false AND expires_at > NOW()
		 ORDER BY created_at DESC
		 LIMIT 1`, phone,
	).Scan(&row.ID, &row.Phone, &row.Code, &row.ExpiresAt, &row.Used, &row.Attempts, &row.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("latest active otp: %w", err)
	}
	return row, nil
}

// MarkUsed flips used=true for a single row id. Idempotent.
func (r *OTPRepository) MarkUsed(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE otp_codes SET used = true WHERE id = $1`, id)
	return err
}

// IncrementAttempts +1 to the attempts counter — called on every failed verify
// for the active code. Caller decides at what number to lock out.
func (r *OTPRepository) IncrementAttempts(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE otp_codes SET attempts = attempts + 1 WHERE id = $1`, id)
	return err
}

// CountRecentForPhone counts OTPs requested for phone within the lookback
// window. Used for per-phone rate-limit (e.g. ≤3 per hour). Independent of
// code validity — covers the abuse case "fire 100 send-OTPs per minute".
func (r *OTPRepository) CountRecentForPhone(ctx context.Context, phone string, since time.Time) (int, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM otp_codes WHERE phone = $1 AND created_at > $2`,
		phone, since,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count recent otp: %w", err)
	}
	return n, nil
}
