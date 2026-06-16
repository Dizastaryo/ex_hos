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

// ── Likes (scanner) ─────────────────────────────────────────────────────────

// WaveRow — row for incoming likes list.
type WaveRow struct {
	WaverID       string    `db:"waver_id"`
	WaverUsername string    `db:"username"`
	WaverFullName string    `db:"full_name"`
	WaverAvatar   string    `db:"avatar_url"`
	WaverVerified bool      `db:"is_verified"`
	CreatedAt     time.Time `db:"created_at"`
}

// ScannerLikeRow — legacy alias.
type ScannerLikeRow = WaveRow

// GetUserByDeviceHash resolves targetUserID by device_public_id.
func (r *ScannerRepository) GetUserByDeviceHash(ctx context.Context, publicIDHex string) (string, error) {
	var userID string
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id FROM users
		WHERE device_public_id = $1
		  AND device_public_id <> ''
		  AND scan_enabled = TRUE
		LIMIT 1`,
		publicIDHex,
	).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrNotFound
		}
		return "", fmt.Errorf("get user by device hash: %w", err)
	}
	return userID, nil
}

// DailyWavesCount returns how many likes the user sent today.
func (r *ScannerRepository) DailyWavesCount(ctx context.Context, waverID string) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM scanner_waves
		WHERE waver_id = $1
		  AND created_at >= CURRENT_DATE`,
		waverID,
	).Scan(&count)
	return count, err
}

// LastWaveAt returns when userA last liked userB.
func (r *ScannerRepository) LastWaveAt(ctx context.Context, waverID, targetUserID string) (time.Time, error) {
	var t time.Time
	err := r.db.Pool.QueryRow(ctx, `
		SELECT created_at FROM scanner_waves
		WHERE waver_id = $1 AND target_user_id = $2
		ORDER BY created_at DESC
		LIMIT 1`,
		waverID, targetUserID,
	).Scan(&t)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return t, nil
}

// InsertWave creates a new like record.
func (r *ScannerRepository) InsertWave(ctx context.Context, waverID, targetUserID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO scanner_waves (waver_id, target_user_id)
		VALUES ($1, $2)`,
		waverID, targetUserID,
	)
	return err
}

// HasMutualLike checks if both users have liked each other.
func (r *ScannerRepository) HasMutualLike(ctx context.Context, userA, userB string) (bool, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT waver_id) FROM scanner_waves
		WHERE (waver_id = $1 AND target_user_id = $2)
		   OR (waver_id = $2 AND target_user_id = $1)`,
		userA, userB,
	).Scan(&count)
	return count >= 2, err
}

// GetMatches returns users who mutually liked each other.
func (r *ScannerRepository) GetMatches(ctx context.Context, userID string, limit, offset int) ([]*WaveRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT u.id, u.username, u.full_name, u.avatar_url, u.is_verified,
		       GREATEST(sw1.created_at, sw2.created_at) AS matched_at
		FROM scanner_waves sw1
		JOIN scanner_waves sw2 ON sw1.waver_id = sw2.target_user_id
		                       AND sw1.target_user_id = sw2.waver_id
		JOIN users u ON u.id = sw1.waver_id
		WHERE sw1.target_user_id = $1
		  AND sw1.waver_id != $1
		GROUP BY u.id, u.username, u.full_name, u.avatar_url, u.is_verified,
		         sw1.created_at, sw2.created_at
		ORDER BY GREATEST(sw1.created_at, sw2.created_at) DESC
		LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get matches: %w", err)
	}
	defer rows.Close()

	var result []*WaveRow
	for rows.Next() {
		row := &WaveRow{}
		if err := rows.Scan(
			&row.WaverID, &row.WaverUsername, &row.WaverFullName,
			&row.WaverAvatar, &row.WaverVerified, &row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan match row: %w", err)
		}
		result = append(result, row)
	}
	return result, nil
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

// GetReceivedWaves returns list of users who liked targetUserID.
func (r *ScannerRepository) GetReceivedWaves(ctx context.Context, targetUserID string, limit, offset int) ([]*WaveRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT DISTINCT ON (sw.waver_id)
		       sw.waver_id, u.username, u.full_name, u.avatar_url, u.is_verified, sw.created_at
		FROM scanner_waves sw
		JOIN users u ON u.id = sw.waver_id
		WHERE sw.target_user_id = $1
		ORDER BY sw.waver_id, sw.created_at DESC`,
		targetUserID,
	)
	if err != nil {
		return nil, fmt.Errorf("get received waves: %w", err)
	}
	defer rows.Close()

	var all []*WaveRow
	for rows.Next() {
		row := &WaveRow{}
		if err := rows.Scan(
			&row.WaverID, &row.WaverUsername, &row.WaverFullName,
			&row.WaverAvatar, &row.WaverVerified, &row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan wave row: %w", err)
		}
		all = append(all, row)
	}

	// Sort by most recent
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].CreatedAt.After(all[i].CreatedAt) {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	if offset >= len(all) {
		return nil, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

// CountReceivedWaves returns count of distinct likers.
func (r *ScannerRepository) CountReceivedWaves(ctx context.Context, targetUserID string) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT waver_id) FROM scanner_waves WHERE target_user_id = $1`,
		targetUserID,
	).Scan(&count)
	return count, err
}

// GetSentWaveTargets returns real profiles of people the user liked.
func (r *ScannerRepository) GetSentWaveTargets(ctx context.Context, waverID string, limit, offset int) ([]*domain.ScanProfile, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT DISTINCT ON (sw.target_user_id)
		       u.id, u.username, u.full_name, u.avatar_url, u.bio, u.is_verified, u.device_public_id
		FROM scanner_waves sw
		JOIN users u ON u.id = sw.target_user_id
		WHERE sw.waver_id = $1
		ORDER BY sw.target_user_id, sw.created_at DESC`,
		waverID,
	)
	if err != nil {
		return nil, fmt.Errorf("get sent waves: %w", err)
	}
	defer rows.Close()

	var result []*domain.ScanProfile
	for rows.Next() {
		sp := &domain.ScanProfile{}
		if err := rows.Scan(&sp.UserID, &sp.Username, &sp.FullName, &sp.AvatarURL, &sp.Bio, &sp.IsVerified, &sp.DeviceHash); err != nil {
			return nil, fmt.Errorf("scan sent wave row: %w", err)
		}
		result = append(result, sp)
	}

	if offset >= len(result) {
		return nil, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

// UnseenWavesCount returns count of likes since user last viewed.
func (r *ScannerRepository) UnseenWavesCount(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM scanner_waves sw
		JOIN users u ON u.id = $1
		WHERE sw.target_user_id = $1
		  AND sw.created_at > u.likes_seen_at`,
		userID,
	).Scan(&count)
	return count, err
}

// MarkWavesSeen updates users.likes_seen_at = NOW().
func (r *ScannerRepository) MarkWavesSeen(ctx context.Context, userID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET likes_seen_at = NOW() WHERE id = $1`, userID)
	return err
}

// UpdateScanProfile updates the scan-profile fields.
func (r *ScannerRepository) UpdateScanProfile(ctx context.Context, userID string, req *domain.UpdateScanProfileRequest) error {
	q := `UPDATE users SET`
	args := []any{}
	idx := 0
	next := func(v any) string {
		idx++
		args = append(args, v)
		return fmt.Sprintf("$%d", idx)
	}
	setClauses := []string{}
	if req.ScanEnabled != nil {
		setClauses = append(setClauses, "scan_enabled = "+next(*req.ScanEnabled))
	}
	if len(setClauses) == 0 {
		return nil
	}
	for i, c := range setClauses {
		if i > 0 {
			q += ","
		}
		q += " " + c
	}
	q += " WHERE id = " + next(userID)
	_, err := r.db.Pool.Exec(ctx, q, args...)
	return err
}

// ── Connect QR tokens ────────────────────────────────────────────────────────

// CreateConnectToken generates a short-lived token for QR-based connect.
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

// ── Proximity heartbeats ─────────────────────────────────────────────────────

// ReportHeartbeat saves visible device hashes for proximity validation.
func (r *ScannerRepository) ReportHeartbeat(ctx context.Context, userID string, visibleHashes []string) error {
	if len(visibleHashes) == 0 {
		return nil
	}
	// Delete old heartbeats for this user
	_, _ = r.db.Pool.Exec(ctx,
		`DELETE FROM scanner_heartbeats WHERE user_id = $1`, userID)

	// Insert new ones
	for _, hash := range visibleHashes {
		_, err := r.db.Pool.Exec(ctx, `
			INSERT INTO scanner_heartbeats (user_id, visible_hash, reported_at)
			VALUES ($1, $2, NOW())`,
			userID, hash,
		)
		if err != nil {
			return fmt.Errorf("insert heartbeat: %w", err)
		}
	}
	return nil
}

// AreNearby checks if userA recently reported seeing userB's device (or vice versa).
// "Recently" = within the last 90 seconds.
func (r *ScannerRepository) AreNearby(ctx context.Context, userA, userB string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM scanner_heartbeats sh
			JOIN users u ON u.device_public_id = sh.visible_hash AND u.id = $2
			WHERE sh.user_id = $1
			  AND sh.reported_at > NOW() - INTERVAL '90 seconds'
		)`,
		userA, userB,
	).Scan(&exists)
	return exists, err
}

// CleanupOldHeartbeats deletes heartbeats older than 2 minutes.
func (r *ScannerRepository) CleanupOldHeartbeats(ctx context.Context) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM scanner_heartbeats WHERE reported_at < NOW() - INTERVAL '2 minutes'`)
	return err
}

// CleanupOldTokens deletes expired tokens.
func (r *ScannerRepository) CleanupOldTokens(ctx context.Context) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM connect_tokens WHERE expires_at < NOW() - INTERVAL '5 minutes'`)
	return err
}

// ── Legacy compat aliases ────────────────────────────────────────────────────

func (r *ScannerRepository) DailyLikesCount(ctx context.Context, likerID string) (int, error) {
	return r.DailyWavesCount(ctx, likerID)
}

func (r *ScannerRepository) UpsertLike(ctx context.Context, likerID, targetUserID string) (bool, error) {
	return true, r.InsertWave(ctx, likerID, targetUserID)
}

func (r *ScannerRepository) DeleteLike(ctx context.Context, likerID, targetUserID string) error {
	return nil
}

func (r *ScannerRepository) GetReceivedLikes(ctx context.Context, targetUserID string, limit, offset int) ([]*WaveRow, error) {
	return r.GetReceivedWaves(ctx, targetUserID, limit, offset)
}

func (r *ScannerRepository) CountReceivedLikes(ctx context.Context, targetUserID string) (int, error) {
	return r.CountReceivedWaves(ctx, targetUserID)
}

func (r *ScannerRepository) GetSentLikeTargets(ctx context.Context, likerID string, limit, offset int) ([]*domain.ScanProfile, error) {
	return r.GetSentWaveTargets(ctx, likerID, limit, offset)
}

func (r *ScannerRepository) UnseenLikesCount(ctx context.Context, userID string) (int, error) {
	return r.UnseenWavesCount(ctx, userID)
}

func (r *ScannerRepository) MarkLikesSeen(ctx context.Context, userID string) error {
	return r.MarkWavesSeen(ctx, userID)
}

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
