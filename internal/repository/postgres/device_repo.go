package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/seeu/backend/internal/domain"
)

type DeviceRepository struct {
	db *DB
}

func NewDeviceRepository(db *DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

// Create сохраняет новое устройство (вызывается при генерации в админке).
func (r *DeviceRepository) Create(ctx context.Context, d *domain.BleDevice) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO ble_devices (id, serial_number, public_id_hex, private_id_hex, is_active, notes)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		d.ID, d.SerialNumber, d.PublicIDHex, d.PrivateIDHex, d.IsActive, d.Notes,
	)
	if err != nil {
		return fmt.Errorf("create ble_device: %w", err)
	}
	return nil
}

// GetBySerial ищет устройство по серийному номеру (для привязки юзером по QR).
// Возвращает ErrNotFound если не найдено или устройство деактивировано.
func (r *DeviceRepository) GetBySerial(ctx context.Context, serial string) (*domain.BleDevice, error) {
	d := &domain.BleDevice{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, serial_number, public_id_hex, private_id_hex, is_active, notes, created_at
		FROM ble_devices
		WHERE serial_number = $1 AND is_active = TRUE`,
		serial,
	).Scan(
		&d.ID, &d.SerialNumber, &d.PublicIDHex, &d.PrivateIDHex,
		&d.IsActive, &d.Notes, &d.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get device by serial: %w", err)
	}
	return d, nil
}

// IsSerialBound проверяет, привязан ли данный серийник к любому юзеру.
// Проверяем через public_id_hex: если хоть один юзер имеет этот public_id — занят.
func (r *DeviceRepository) IsPublicIDTaken(ctx context.Context, publicIDHex string) (bool, error) {
	var count int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users WHERE device_public_id = $1`,
		publicIDHex,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check public id taken: %w", err)
	}
	return count > 0, nil
}

// AdminListFilter параметры для GET /admin/devices.
type AdminListDevicesFilter struct {
	Query  string
	Limit  int
	Offset int
}

// AdminList возвращает список устройств для admin-панели (без private_id_hex).
func (r *DeviceRepository) AdminList(ctx context.Context, f AdminListDevicesFilter) ([]*domain.BleDevice, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	q := `
		SELECT id, serial_number, public_id_hex, is_active, notes, created_at
		FROM ble_devices
		WHERE 1=1`
	args := []any{}
	idx := 0
	next := func(v any) string {
		idx++
		args = append(args, v)
		return fmt.Sprintf("$%d", idx)
	}
	if f.Query != "" {
		p := next("%" + f.Query + "%")
		q += " AND (serial_number ILIKE " + p + " OR notes ILIKE " + p + ")"
	}
	q += " ORDER BY created_at DESC"
	q += " LIMIT " + next(f.Limit) + " OFFSET " + next(f.Offset)

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("admin list devices: %w", err)
	}
	defer rows.Close()

	var result []*domain.BleDevice
	for rows.Next() {
		d := &domain.BleDevice{}
		if err := rows.Scan(
			&d.ID, &d.SerialNumber, &d.PublicIDHex,
			&d.IsActive, &d.Notes, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan device row: %w", err)
		}
		result = append(result, d)
	}
	return result, nil
}

// AdminExport возвращает все активные устройства с private_id_hex для CSV-прошивки.
// Вызывается только из защищённого admin-only endpoint.
func (r *DeviceRepository) AdminExport(ctx context.Context) ([]*domain.BleDeviceExportRow, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT serial_number, public_id_hex, private_id_hex
		FROM ble_devices
		WHERE is_active = TRUE
		ORDER BY serial_number`)
	if err != nil {
		return nil, fmt.Errorf("admin export devices: %w", err)
	}
	defer rows.Close()

	var result []*domain.BleDeviceExportRow
	for rows.Next() {
		row := &domain.BleDeviceExportRow{}
		if err := rows.Scan(&row.SerialNumber, &row.PublicIDHex, &row.PrivateIDHex); err != nil {
			return nil, fmt.Errorf("scan export row: %w", err)
		}
		result = append(result, row)
	}
	return result, nil
}

// ── Private Whitelist ─────────────────────────────────────────────────────────

// GetPrivateWhitelist возвращает список пользователей которым ownerID разрешил
// видеть себя в private BLE-режиме. Возвращает только взаимных подписчиков.
func (r *DeviceRepository) GetPrivateWhitelist(ctx context.Context, ownerID string) ([]*domain.PrivateWhitelistEntry, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT u.id, u.username, u.full_name, COALESCE(u.avatar_url, '')
		FROM device_private_whitelist w
		JOIN users u ON u.id = w.allowed_id
		WHERE w.owner_id = $1
		ORDER BY u.username`,
		ownerID,
	)
	if err != nil {
		return nil, fmt.Errorf("get private whitelist: %w", err)
	}
	defer rows.Close()

	var result []*domain.PrivateWhitelistEntry
	for rows.Next() {
		e := &domain.PrivateWhitelistEntry{}
		if err := rows.Scan(&e.UserID, &e.Username, &e.FullName, &e.AvatarURL); err != nil {
			return nil, fmt.Errorf("scan whitelist row: %w", err)
		}
		result = append(result, e)
	}
	return result, nil
}

// SetPrivateWhitelist заменяет whitelist ownerID на allowedIDs.
// allowedIDs должны быть взаимными подписчиками — сервисный слой проверяет это до вызова.
// Работает транзакционно: DELETE old + INSERT new.
func (r *DeviceRepository) SetPrivateWhitelist(ctx context.Context, ownerID string, allowedIDs []string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`DELETE FROM device_private_whitelist WHERE owner_id = $1`, ownerID,
	); err != nil {
		return fmt.Errorf("delete old whitelist: %w", err)
	}

	for _, uid := range allowedIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO device_private_whitelist (owner_id, allowed_id) VALUES ($1, $2)
			 ON CONFLICT DO NOTHING`,
			ownerID, uid,
		); err != nil {
			return fmt.Errorf("insert whitelist entry %s: %w", uid, err)
		}
	}

	return tx.Commit(ctx)
}

// FilterMutualFollowers из списка candidateIDs возвращает только тех кто
// взаимно подписан с ownerID. Используется для валидации перед SetPrivateWhitelist.
func (r *DeviceRepository) FilterMutualFollowers(ctx context.Context, ownerID string, candidateIDs []string) ([]string, error) {
	if len(candidateIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT f1.following_id
		FROM follows f1
		JOIN follows f2
		  ON f2.follower_id = f1.following_id
		 AND f2.following_id = f1.follower_id
		WHERE f1.follower_id = $1
		  AND f1.following_id = ANY($2::uuid[])`,
		ownerID, candidateIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("filter mutual followers: %w", err)
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan mutual id: %w", err)
		}
		result = append(result, id)
	}
	return result, nil
}

// IsInPrivateWhitelist проверяет что viewerID есть в whitelist ownerID.
func (r *DeviceRepository) IsInPrivateWhitelist(ctx context.Context, ownerID, viewerID string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM device_private_whitelist
			WHERE owner_id = $1 AND allowed_id = $2
		)`, ownerID, viewerID,
	).Scan(&exists)
	return exists, err
}

// SetActive деактивирует устройство (мягкое удаление).
func (r *DeviceRepository) SetActive(ctx context.Context, id string, active bool) error {
	tag, err := r.db.Pool.Exec(ctx,
		`UPDATE ble_devices SET is_active = $1 WHERE id = $2`,
		active, id,
	)
	if err != nil {
		return fmt.Errorf("set device active: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
