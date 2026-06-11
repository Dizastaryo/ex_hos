package postgres

import (
	"context"
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

// ScannerLikeRow — строка для списка входящих лайков.
type ScannerLikeRow struct {
	LikerID       string    `db:"liker_id"`
	LikerUsername string    `db:"username"`
	LikerFullName string    `db:"full_name"`
	LikerAvatar   string    `db:"avatar_url"`
	LikerVerified bool      `db:"is_verified"`
	CreatedAt     time.Time `db:"created_at"`
}

// UpsertLike ставит лайк от likerID на targetUserID.
// Если лайк уже стоит — ничего не делает (idempotent).
// Возвращает isNew=true если это новый лайк (не дубль).
func (r *ScannerRepository) UpsertLike(ctx context.Context, likerID, targetUserID string) (isNew bool, err error) {
	tag, err := r.db.Pool.Exec(ctx, `
		INSERT INTO scanner_likes (liker_id, target_user_id)
		VALUES ($1, $2)
		ON CONFLICT (liker_id, target_user_id) DO NOTHING`,
		likerID, targetUserID,
	)
	if err != nil {
		return false, fmt.Errorf("upsert scanner like: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// DeleteLike убирает лайк likerID → targetUserID.
func (r *ScannerRepository) DeleteLike(ctx context.Context, likerID, targetUserID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		DELETE FROM scanner_likes WHERE liker_id = $1 AND target_user_id = $2`,
		likerID, targetUserID,
	)
	return err
}

// GetReceivedLikes возвращает список тех, кто лайкнул targetUserID.
// Возвращает реальные данные лайкера — target видит с кем контактировать.
func (r *ScannerRepository) GetReceivedLikes(ctx context.Context, targetUserID string, limit, offset int) ([]*ScannerLikeRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT sl.liker_id, u.username, u.full_name, u.avatar_url, u.is_verified, sl.created_at
		FROM scanner_likes sl
		JOIN users u ON u.id = sl.liker_id
		WHERE sl.target_user_id = $1
		ORDER BY sl.created_at DESC
		LIMIT $2 OFFSET $3`,
		targetUserID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get received likes: %w", err)
	}
	defer rows.Close()

	var result []*ScannerLikeRow
	for rows.Next() {
		row := &ScannerLikeRow{}
		if err := rows.Scan(
			&row.LikerID, &row.LikerUsername, &row.LikerFullName,
			&row.LikerAvatar, &row.LikerVerified, &row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan like row: %w", err)
		}
		result = append(result, row)
	}
	return result, nil
}

// CountReceivedLikes возвращает количество входящих лайков (для badge).
func (r *ScannerRepository) CountReceivedLikes(ctx context.Context, targetUserID string) (int, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM scanner_likes WHERE target_user_id = $1`,
		targetUserID,
	).Scan(&count)
	return count, err
}

// GetSentLikeTargets возвращает scan-профили тех, кому likerID поставил лайк.
// Возвращает только scan_alias (реальный аккаунт не раскрывается лайкеру).
func (r *ScannerRepository) GetSentLikeTargets(ctx context.Context, likerID string, limit, offset int) ([]*domain.ScanProfile, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT u.device_public_id, u.scan_alias, u.scan_avatar_url
		FROM scanner_likes sl
		JOIN users u ON u.id = sl.target_user_id
		WHERE sl.liker_id = $1
		ORDER BY sl.created_at DESC
		LIMIT $2 OFFSET $3`,
		likerID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get sent likes: %w", err)
	}
	defer rows.Close()

	var result []*domain.ScanProfile
	for rows.Next() {
		sp := &domain.ScanProfile{}
		if err := rows.Scan(&sp.DeviceHash, &sp.ScanAlias, &sp.ScanAvatarURL); err != nil {
			return nil, fmt.Errorf("scan sent like row: %w", err)
		}
		result = append(result, sp)
	}
	return result, nil
}

// GetUserByScanLike резолвит targetUserID по device_public_id для операции лайка.
// scan_enabled проверяется — нельзя лайкать того, кто отключил видимость.
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

// UpdateScanProfile обновляет scan-профиль пользователя.
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
	if req.ScanAlias != "" {
		setClauses = append(setClauses, "scan_alias = "+next(req.ScanAlias))
	}
	if req.ScanAvatarURL != "" {
		setClauses = append(setClauses, "scan_avatar_url = "+next(req.ScanAvatarURL))
	}
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
