package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type ScannerRepository struct {
	db *DB
}

func NewScannerRepository(db *DB) *ScannerRepository {
	return &ScannerRepository{db: db}
}

// ── Resolve (real profiles) ──────────────────────────────────────────────────

// ResolveScanProfile returns the REAL profile for a device_hash.
func (r *ScannerRepository) ResolveScanProfile(ctx context.Context, publicIDHex string) (*domain.ScanProfile, error) {
	sp := &domain.ScanProfile{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, username, full_name, avatar_url, bio, is_verified, device_public_id
		FROM users
		WHERE device_public_id = $1
		  AND device_public_id <> ''
		  AND scan_enabled = TRUE
		LIMIT 1`,
		publicIDHex,
	).Scan(&sp.UserID, &sp.Username, &sp.FullName, &sp.AvatarURL, &sp.Bio, &sp.IsVerified, &sp.DeviceHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("resolve scan profile: %w", err)
	}
	return sp, nil
}

// ResolveScanProfiles batch-resolves multiple device hashes into real profiles.
func (r *ScannerRepository) ResolveScanProfiles(ctx context.Context, publicIDHexes []string) ([]*domain.ScanProfile, error) {
	if len(publicIDHexes) == 0 {
		return nil, nil
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, username, full_name, avatar_url, bio, is_verified, device_public_id
		FROM users
		WHERE device_public_id = ANY($1)
		  AND device_public_id <> ''
		  AND scan_enabled = TRUE`,
		publicIDHexes,
	)
	if err != nil {
		return nil, fmt.Errorf("batch resolve scan profiles: %w", err)
	}
	defer rows.Close()

	var result []*domain.ScanProfile
	for rows.Next() {
		sp := &domain.ScanProfile{}
		if err := rows.Scan(&sp.UserID, &sp.Username, &sp.FullName, &sp.AvatarURL, &sp.Bio, &sp.IsVerified, &sp.DeviceHash); err != nil {
			return nil, fmt.Errorf("scan profile row: %w", err)
		}
		result = append(result, sp)
	}
	return result, nil
}

// ── Scan-profile settings ────────────────────────────────────────────────────

// UpdateScanProfile updates scan_enabled for the user.
func (r *ScannerRepository) UpdateScanProfile(ctx context.Context, userID string, req *domain.UpdateScanProfileRequest) error {
	if req.ScanEnabled == nil {
		return nil
	}
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET scan_enabled = $1 WHERE id = $2`,
		*req.ScanEnabled, userID,
	)
	return err
}

// ── Connect QR tokens ────────────────────────────────────────────────────────

// CreateConnectToken generates a short-lived token for QR-based access.
func (r *ScannerRepository) CreateConnectToken(ctx context.Context, userID string, ttl time.Duration) (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, fmt.Errorf("generate random token: %w", err)
	}
	token := hex.EncodeToString(b)
	expiresAt := time.Now().Add(ttl)

	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO connect_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)`,
		userID, token, expiresAt,
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("insert connect token: %w", err)
	}
	return token, expiresAt, nil
}

// ValidateConnectToken checks token validity and returns the owner's user_id.
func (r *ScannerRepository) ValidateConnectToken(ctx context.Context, token string) (string, error) {
	var userID string
	err := r.db.Pool.QueryRow(ctx, `
		SELECT user_id FROM connect_tokens
		WHERE token = $1
		  AND used = false
		  AND expires_at > NOW()`,
		token,
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrNotFound
		}
		return "", fmt.Errorf("validate connect token: %w", err)
	}
	return userID, nil
}

// MarkConnectTokenUsed marks the token as used.
func (r *ScannerRepository) MarkConnectTokenUsed(ctx context.Context, token, usedByUserID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE connect_tokens SET used = true, used_by = $2
		WHERE token = $1`,
		token, usedByUserID,
	)
	return err
}

// HasChatBetween checks if a 1-on-1 chat exists between two users.
func (r *ScannerRepository) HasChatBetween(ctx context.Context, userA, userB string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM chat_members cm1
			JOIN chat_members cm2 ON cm1.chat_id = cm2.chat_id
			JOIN chats c ON c.id = cm1.chat_id
			WHERE cm1.user_id = $1 AND cm2.user_id = $2
			  AND c.is_group = false
		)`,
		userA, userB,
	).Scan(&exists)
	return exists, err
}

// GetUserByPrivateDeviceHash resolves userID by private device hash (whitelist check).
func (r *ScannerRepository) GetUserByPrivateDeviceHash(ctx context.Context, privateIDHex, viewerID string) (string, error) {
	var userID string
	err := r.db.Pool.QueryRow(ctx, `
		SELECT u.id FROM users u
		WHERE u.device_private_id = $1
		  AND u.device_private_id <> ''
		  AND u.scan_enabled = TRUE
		  AND EXISTS (
		      SELECT 1 FROM device_private_whitelist w
		      WHERE w.owner_id = u.id AND w.allowed_id = $2
		  )
		LIMIT 1`,
		privateIDHex, viewerID,
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrNotFound
		}
		return "", fmt.Errorf("get user by private device hash: %w", err)
	}
	return userID, nil
}

// CleanupOldTokens deletes expired tokens.
func (r *ScannerRepository) CleanupOldTokens(ctx context.Context) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM connect_tokens WHERE expires_at < NOW() - INTERVAL '10 minutes'`)
	return err
}
