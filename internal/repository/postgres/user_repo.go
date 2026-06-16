package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/seeu/backend/internal/domain"
)

type UserRepository struct {
	db *DB
}

func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

// AuthFlags is the minimal slice of user state the auth middleware needs to
// decide whether to let the request through. Kept narrow on purpose so the
// query stays tiny.
type AuthFlags struct {
	IsAdmin  bool
	IsBanned bool
}

func (r *UserRepository) GetAuthFlags(ctx context.Context, id string) (AuthFlags, error) {
	var f AuthFlags
	err := r.db.Pool.QueryRow(ctx,
		`SELECT is_admin, is_banned FROM users WHERE id = $1`, id,
	).Scan(&f.IsAdmin, &f.IsBanned)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return f, domain.ErrUserNotFound
		}
		return f, fmt.Errorf("get auth flags: %w", err)
	}
	return f, nil
}

// AdminListUsersFilter narrows the admin user search.
type AdminListUsersFilter struct {
	Query      string
	OnlyBanned bool
	OnlyAdmins bool
	Limit      int
	Offset     int
}

func (r *UserRepository) AdminListUsers(ctx context.Context, f AdminListUsersFilter) ([]*domain.User, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	q := `
		SELECT id, username, phone, full_name, bio, avatar_url, gender,
		       is_private, is_verified, is_admin, is_banned, banned_reason,
		       posts_count, followers_count, following_count, created_at, updated_at
		FROM users
		WHERE 1=1`
	argIdx := 0
	args := []any{}
	nextArg := func(val any) string {
		argIdx++
		args = append(args, val)
		return fmt.Sprintf("$%d", argIdx)
	}
	if f.Query != "" {
		p := nextArg("%" + f.Query + "%")
		q += " AND (username ILIKE " + p + " OR full_name ILIKE " + p + " OR phone ILIKE " + p + ")"
	}
	if f.OnlyBanned {
		q += " AND is_banned = true"
	}
	if f.OnlyAdmins {
		q += " AND is_admin = true"
	}
	pLimit := nextArg(f.Limit)
	pOffset := nextArg(f.Offset)
	q += " ORDER BY created_at DESC LIMIT " + pLimit + " OFFSET " + pOffset

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("admin list users: %w", err)
	}
	defer rows.Close()

	var out []*domain.User
	for rows.Next() {
		u := &domain.User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Phone, &u.FullName, &u.Bio, &u.AvatarURL,
			&u.Gender, &u.IsPrivate, &u.IsVerified, &u.IsAdmin, &u.IsBanned, &u.BannedReason,
			&u.PostsCount, &u.FollowersCount, &u.FollowingCount, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("admin list users scan: %w", err)
		}
		out = append(out, u)
	}
	return out, nil
}

func (r *UserRepository) SetBanned(ctx context.Context, id string, banned bool, reason string) error {
	if banned {
		_, err := r.db.Pool.Exec(ctx,
			`UPDATE users SET is_banned = true, banned_at = NOW(), banned_reason = $2 WHERE id = $1`,
			id, reason)
		if err != nil {
			return fmt.Errorf("ban user: %w", err)
		}
		return nil
	}
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET is_banned = false, banned_at = NULL, banned_reason = '' WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("unban user: %w", err)
	}
	return nil
}

func (r *UserRepository) SetAdmin(ctx context.Context, id string, isAdmin bool) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE users SET is_admin = $2 WHERE id = $1`, id, isAdmin)
	if err != nil {
		return fmt.Errorf("set admin: %w", err)
	}
	return nil
}

// SetVerified — PROFILE-5. Голубая галочка для подтверждённых аккаунтов.
// Управляется admin-эндпоинтом.
func (r *UserRepository) SetVerified(ctx context.Context, id string, verified bool) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE users SET is_verified = $2 WHERE id = $1`, id, verified)
	if err != nil {
		return fmt.Errorf("set verified: %w", err)
	}
	return nil
}

// RecordConsent stamps the time the user accepted Terms / Privacy Policy.
// Called on every successful OTP verify when AcceptsTerms=true so we always
// know the most recent consent point.
func (r *UserRepository) RecordConsent(ctx context.Context, id string) error {
	_, err := r.db.Pool.Exec(ctx, `UPDATE users SET consents_accepted_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("record consent: %w", err)
	}
	return nil
}

// DeleteByID removes a user. ON DELETE CASCADE on FKs takes care of posts,
// stories, comments, likes, follows, saved posts, notifications, chat
// messages and library files belonging to that user.
func (r *UserRepository) DeleteByID(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (username, phone, full_name, bio, avatar_url, website, gender, device_public_id, device_private_id, is_private)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at`

	err := r.db.Pool.QueryRow(ctx, query,
		user.Username,
		user.Phone,
		user.FullName,
		user.Bio,
		user.AvatarURL,
		user.Website,
		user.Gender,
		user.DevicePublicID,
		user.DevicePrivateID,
		user.IsPrivate,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, username, phone, full_name, bio, avatar_url, website,
		       gender, date_of_birth, device_public_id, device_private_id,
		       is_private, is_verified, posts_count, followers_count, following_count,
		       last_seen_at, COALESCE(hide_last_seen, false),
		       COALESCE(channel_about, ''), COALESCE(channel_banner_url, ''),
		       COALESCE(scan_alias, ''), COALESCE(scan_avatar_url, ''), COALESCE(scan_emoji, ''), COALESCE(scan_status, ''), COALESCE(scan_enabled, true),
		       created_at, updated_at
		FROM users WHERE id = $1`

	return r.scanUser(ctx, query, id)
}

func (r *UserRepository) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	query := `
		SELECT id, username, phone, full_name, bio, avatar_url, website,
		       gender, date_of_birth, device_public_id, device_private_id,
		       is_private, is_verified, posts_count, followers_count, following_count,
		       last_seen_at, COALESCE(hide_last_seen, false),
		       COALESCE(channel_about, ''), COALESCE(channel_banner_url, ''),
		       COALESCE(scan_alias, ''), COALESCE(scan_avatar_url, ''), COALESCE(scan_emoji, ''), COALESCE(scan_status, ''), COALESCE(scan_enabled, true),
		       created_at, updated_at
		FROM users WHERE phone = $1`

	return r.scanUser(ctx, query, phone)
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	query := `
		SELECT id, username, phone, full_name, bio, avatar_url, website,
		       gender, date_of_birth, device_public_id, device_private_id,
		       is_private, is_verified, posts_count, followers_count, following_count,
		       last_seen_at, COALESCE(hide_last_seen, false),
		       COALESCE(channel_about, ''), COALESCE(channel_banner_url, ''),
		       COALESCE(scan_alias, ''), COALESCE(scan_avatar_url, ''), COALESCE(scan_emoji, ''), COALESCE(scan_status, ''), COALESCE(scan_enabled, true),
		       created_at, updated_at
		FROM users WHERE username = $1`

	return r.scanUser(ctx, query, username)
}

// SetLastSeen обновляет users.last_seen_at = NOW(). Вызывается из WS Hub'а
// при register/unregister — даёт «в сети» / «был N мин назад» в UI.
func (r *UserRepository) SetLastSeen(ctx context.Context, userID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET last_seen_at = NOW() WHERE id = $1`,
		userID,
	)
	return err
}

// SetDevicePublicID атомарно привязывает BLE-метку к юзеру. Если другой
// юзер уже владеет этим device_public_id — вернёт ErrAlreadyExists. Пустая
// строка отвязывает текущий чип.
// Deprecated: используй SetDeviceIDs (принимает оба id сразу).
func (r *UserRepository) SetDevicePublicID(ctx context.Context, userID, publicID string) error {
	return r.SetDeviceIDs(ctx, userID, publicID, "")
}

// SetDeviceIDs атомарно записывает device_public_id + device_private_id юзеру.
// Пустые строки — очищают соответствующее поле (отвязка браслета).
// Если другой юзер уже использует тот же publicIDHex — ErrAlreadyExists.
func (r *UserRepository) SetDeviceIDs(ctx context.Context, userID, publicIDHex, privateIDHex string) error {
	if publicIDHex == "" {
		_, err := r.db.Pool.Exec(ctx,
			`UPDATE users SET device_public_id = '', device_private_id = '' WHERE id = $1`,
			userID,
		)
		return err
	}
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE users
		SET device_public_id = $2, device_private_id = $3
		WHERE id = $1
		  AND NOT EXISTS (
		      SELECT 1 FROM users
		      WHERE device_public_id = $2 AND id <> $1
		  )`,
		userID, publicIDHex, privateIDHex,
	)
	if err != nil {
		return fmt.Errorf("set device ids: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrAlreadyExists
	}
	return nil
}

// GetByDevicePrivateIDAmongUsers — BUG-17. Резолвит приватный BLE-id
// (mode=0x01 packet — hex который шлёт чип в privacy-mode) в одного из
// `allowedUserIDs` (обычно: список follow'ed/friends'ов viewer'а).
// Privacy: searching по private_id ограничено whitelist'ом чтобы
// не raises'ить чужие приватные ID через brute-force.
//
// Возвращает nil/ErrUserNotFound если match не найден среди allowed.
// Пустой allowedUserIDs → сразу ErrUserNotFound (защита от broad-search).
func (r *UserRepository) GetByDevicePrivateIDAmongUsers(
	ctx context.Context, privateID string, allowedUserIDs []string,
) (*domain.User, error) {
	if len(allowedUserIDs) == 0 || privateID == "" {
		return nil, domain.ErrUserNotFound
	}
	query := `
		SELECT id, username, phone, full_name, bio, avatar_url, website,
		       gender, date_of_birth, device_public_id, device_private_id,
		       is_private, is_verified, posts_count, followers_count, following_count,
		       last_seen_at, COALESCE(hide_last_seen, false),
		       COALESCE(channel_about, ''), COALESCE(channel_banner_url, ''),
		       created_at, updated_at
		FROM users
		WHERE device_private_id = $1
		  AND device_private_id <> ''
		  AND id = ANY($2::uuid[])
		LIMIT 1`
	user := &domain.User{}
	err := r.db.Pool.QueryRow(ctx, query, privateID, allowedUserIDs).Scan(
		&user.ID, &user.Username, &user.Phone,
		&user.FullName, &user.Bio, &user.AvatarURL, &user.Website,
		&user.Gender, &user.DateOfBirth, &user.DevicePublicID, &user.DevicePrivateID,
		&user.IsPrivate, &user.IsVerified, &user.PostsCount,
		&user.FollowersCount, &user.FollowingCount,
		&user.LastSeenAt, &user.HideLastSeen,
		&user.ChannelAbout, &user.ChannelBannerURL,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("get by private id: %w", err)
	}
	return user, nil
}

// GetByDevicePublicID резолвит BLE device public_id_hex в scan-профиль юзера.
// scan_enabled = false → ErrUserNotFound (сервер-сайд privacy toggle).
// Возвращает полный User; caller берёт только scan_alias/scan_avatar_url/device_public_id.
func (r *UserRepository) GetByDevicePublicID(ctx context.Context, publicID string) (*domain.User, error) {
	query := `
		SELECT id, username, phone, full_name, bio, avatar_url, website,
		       gender, date_of_birth, device_public_id, device_private_id,
		       is_private, is_verified, posts_count, followers_count, following_count,
		       last_seen_at, COALESCE(hide_last_seen, false),
		       COALESCE(channel_about, ''), COALESCE(channel_banner_url, ''),
		       COALESCE(scan_alias, ''), COALESCE(scan_avatar_url, ''), COALESCE(scan_emoji, ''), COALESCE(scan_status, ''), COALESCE(scan_enabled, true),
		       created_at, updated_at
		FROM users
		WHERE device_public_id = $1
		  AND device_public_id <> ''
		  AND scan_enabled = TRUE`
	return r.scanUser(ctx, query, publicID)
}

func (r *UserRepository) scanUser(ctx context.Context, query string, arg interface{}) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.Pool.QueryRow(ctx, query, arg).Scan(
		&user.ID, &user.Username, &user.Phone,
		&user.FullName, &user.Bio, &user.AvatarURL, &user.Website,
		&user.Gender, &user.DateOfBirth, &user.DevicePublicID, &user.DevicePrivateID,
		&user.IsPrivate, &user.IsVerified, &user.PostsCount,
		&user.FollowersCount, &user.FollowingCount,
		&user.LastSeenAt, &user.HideLastSeen,
		&user.ChannelAbout, &user.ChannelBannerURL,
		&user.ScanAlias, &user.ScanAvatarURL, &user.ScanEmoji, &user.ScanStatus, &user.ScanEnabled,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET full_name = $1, bio = $2, website = $3, is_private = $4,
		    avatar_url = $5, gender = $6, device_public_id = $7, device_private_id = $8,
		    username = $9, hide_last_seen = $10,
		    channel_about = $11, channel_banner_url = $12, updated_at = NOW()
		WHERE id = $13
		RETURNING updated_at`

	err := r.db.Pool.QueryRow(ctx, query,
		user.FullName,
		user.Bio,
		user.Website,
		user.IsPrivate,
		user.AvatarURL,
		user.Gender,
		user.DevicePublicID,
		user.DevicePrivateID,
		user.Username,
		user.HideLastSeen,
		user.ChannelAbout,
		user.ChannelBannerURL,
		user.ID,
	).Scan(&user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrUserNotFound
		}
		if isUniqueViolation(err) {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("update user: %w", err)
	}

	return nil
}

func (r *UserRepository) IncrementPostsCount(ctx context.Context, userID string, delta int) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET posts_count = GREATEST(posts_count + $1, 0) WHERE id = $2`,
		delta, userID)
	return err
}

func (r *UserRepository) IncrementFollowersCount(ctx context.Context, userID string, delta int) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET followers_count = GREATEST(followers_count + $1, 0) WHERE id = $2`,
		delta, userID)
	return err
}

func (r *UserRepository) IncrementFollowingCount(ctx context.Context, userID string, delta int) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET following_count = GREATEST(following_count + $1, 0) WHERE id = $2`,
		delta, userID)
	return err
}

func (r *UserRepository) SearchByUsername(ctx context.Context, query string, limit, offset int) ([]*domain.User, error) {
	q := `
		SELECT id, username, phone, full_name, bio, avatar_url, website,
		       gender, date_of_birth, device_public_id, device_private_id,
		       is_private, is_verified, posts_count, followers_count, following_count,
		       last_seen_at, COALESCE(hide_last_seen, false),
		       COALESCE(channel_about, ''), COALESCE(channel_banner_url, ''),
		       created_at, updated_at
		FROM users
		WHERE username ILIKE $1 OR full_name ILIKE $1
		ORDER BY followers_count DESC
		LIMIT $2 OFFSET $3`

	return r.scanUsers(ctx, q, "%"+query+"%", limit, offset)
}

func (r *UserRepository) GetFollowers(ctx context.Context, userID string, limit, offset int) ([]*domain.User, error) {
	query := `
		SELECT u.id, u.username, u.phone, u.full_name, u.bio, u.avatar_url, u.website,
		       u.gender, u.date_of_birth, u.device_public_id, u.device_private_id,
		       u.is_private, u.is_verified, u.posts_count, u.followers_count, u.following_count,
		       u.last_seen_at, COALESCE(u.hide_last_seen, false),
		       COALESCE(u.channel_about, ''), COALESCE(u.channel_banner_url, ''),
		       u.created_at, u.updated_at
		FROM users u
		INNER JOIN follows f ON f.follower_id = u.id
		WHERE f.following_id = $1
		ORDER BY f.created_at DESC
		LIMIT $2 OFFSET $3`

	return r.scanUsers(ctx, query, userID, limit, offset)
}

func (r *UserRepository) GetFollowing(ctx context.Context, userID string, limit, offset int) ([]*domain.User, error) {
	query := `
		SELECT u.id, u.username, u.phone, u.full_name, u.bio, u.avatar_url, u.website,
		       u.gender, u.date_of_birth, u.device_public_id, u.device_private_id,
		       u.is_private, u.is_verified, u.posts_count, u.followers_count, u.following_count,
		       u.last_seen_at, COALESCE(u.hide_last_seen, false),
		       COALESCE(u.channel_about, ''), COALESCE(u.channel_banner_url, ''),
		       u.created_at, u.updated_at
		FROM users u
		INNER JOIN follows f ON f.following_id = u.id
		WHERE f.follower_id = $1
		ORDER BY f.created_at DESC
		LIMIT $2 OFFSET $3`

	return r.scanUsers(ctx, query, userID, limit, offset)
}

func (r *UserRepository) scanUsers(ctx context.Context, query string, args ...interface{}) ([]*domain.User, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u := &domain.User{}
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Phone, &u.FullName, &u.Bio,
			&u.AvatarURL, &u.Website, &u.Gender, &u.DateOfBirth,
			&u.DevicePublicID, &u.DevicePrivateID,
			&u.IsPrivate, &u.IsVerified,
			&u.PostsCount, &u.FollowersCount, &u.FollowingCount,
			&u.LastSeenAt, &u.HideLastSeen,
			&u.ChannelAbout, &u.ChannelBannerURL,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
