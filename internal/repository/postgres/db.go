package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type DB struct {
	Pool *pgxpool.Pool
}

type Config struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int
	// SlowQueryThreshold (BACK-11) — query'и дольше этого порога логируются
	// как WARN. Если nil — slow-query log disabled (no overhead).
	SlowQueryLogger    *zap.Logger
	SlowQueryThreshold time.Duration
}

// slowQueryTracer (BACK-11 / BUG-14) — реализация pgx.QueryTracer интерфейса.
// На QueryStart запоминает start-time + SQL, на QueryEnd сравнивает elapsed
// с threshold и логирует WARN с превью SQL (первые 120 символов).
//
// BUG-14: фильтр DDL — миграции при startup создают индексы за секунды
// (CREATE INDEX CONCURRENTLY etc), это floods log на каждом restart. DDL
// statements skip'аются. Sandboxed CREATE EXTENSION тоже skip.
type slowQueryTracer struct {
	logger    *zap.Logger
	threshold time.Duration
}

type slowQueryStartInfo struct {
	start time.Time
	sql   string
}
type slowQueryStartKey struct{}

func (t *slowQueryTracer) TraceQueryStart(
	ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData,
) context.Context {
	return context.WithValue(ctx, slowQueryStartKey{}, slowQueryStartInfo{
		start: time.Now(),
		sql:   data.SQL,
	})
}

func (t *slowQueryTracer) TraceQueryEnd(
	ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData,
) {
	info, ok := ctx.Value(slowQueryStartKey{}).(slowQueryStartInfo)
	if !ok {
		return
	}
	elapsed := time.Since(info.start)
	if elapsed < t.threshold {
		return
	}
	// BUG-14: skip DDL — миграции / schema_migrations checks / CREATE INDEX.
	// Trim leading whitespace + uppercase compare первого слова.
	trimmed := strings.TrimSpace(info.sql)
	if len(trimmed) >= 6 {
		head := strings.ToUpper(trimmed[:6])
		switch head[:6] {
		case "CREATE", "ALTER ", "DROP T", "DROP I", "DROP C", "TRUNCA":
			return // DDL — миграции, не logируем
		}
		// 4-буквенные keywords: VACUUM, BEGIN/COMMIT (тоже шумные на startup)
		head4 := head[:4]
		if head4 == "VACU" || head4 == "ANAL" {
			return
		}
	}
	// Превью SQL — первые 120 символов чтобы log не разрастался.
	preview := trimmed
	if len(preview) > 120 {
		preview = preview[:120] + "…"
	}
	t.logger.Warn("slow database query",
		zap.Duration("elapsed", elapsed),
		zap.String("tag", data.CommandTag.String()),
		zap.String("sql", preview),
		zap.Error(data.Err),
	)
}

func NewDB(ctx context.Context, cfg Config) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns / 2)
	poolConfig.MaxConnLifetime = time.Duration(cfg.ConnMaxLifetime) * time.Second
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// BACK-11: подключаем slow-query tracer если caller дал logger.
	if cfg.SlowQueryLogger != nil && cfg.SlowQueryThreshold > 0 {
		poolConfig.ConnConfig.Tracer = &slowQueryTracer{
			logger:    cfg.SlowQueryLogger,
			threshold: cfg.SlowQueryThreshold,
		}
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	db.Pool.Close()
}

func (db *DB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// WithTx (BACK-1) — выполняет fn в pgx.Tx с автоматическим
// Commit/Rollback. Если fn возвращает err — rollback и err пробрасывается.
// Если panic — rollback + re-panic. Используется service-уровнем для
// multi-step ops (INSERT + UPDATE counters): либо всё успешно, либо
// rollback оставляет таблицы консистентными.
func (db *DB) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("fn err=%w rollback err=%v", err, rbErr)
		}
		return err
	}
	return tx.Commit(ctx)
}
