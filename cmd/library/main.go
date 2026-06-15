package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/seeu/backend/config"
	"github.com/seeu/backend/internal/handler"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
	redisRepo "github.com/seeu/backend/internal/repository/redis"
	"github.com/seeu/backend/internal/service"
	jwtpkg "github.com/seeu/backend/pkg/jwt"
	"github.com/seeu/backend/pkg/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	cfg.App.Port = "8003"

	logger := buildLogger(cfg.App.Env)
	defer logger.Sync()

	logger.Info("starting seeu library service",
		zap.String("env", cfg.App.Env),
		zap.String("port", cfg.App.Port),
	)

	// Database
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := postgres.NewDB(ctx, postgres.Config{
		URL:                cfg.Database.URL,
		MaxOpenConns:       cfg.Database.MaxOpenConns,
		MaxIdleConns:       cfg.Database.MaxIdleConns,
		ConnMaxLifetime:    cfg.Database.ConnMaxLifetime,
		SlowQueryLogger:    logger,
		SlowQueryThreshold: 100 * time.Millisecond,
	})
	if err != nil {
		logger.Fatal("connect to database", zap.Error(err))
	}
	defer db.Close()
	logger.Info("connected to postgresql")

	// Migrations are owned by `cmd/api`; library reuses the same database
	// schema and trusts api to keep it current.

	// In-memory cache & session store
	cache, err := redisRepo.NewCache("")
	if err != nil {
		logger.Fatal("init cache", zap.Error(err))
	}
	defer cache.Close()
	logger.Info("in-memory cache ready")

	sessionStore := redisRepo.NewSessionStore(cache)

	// JWT
	jwtManager := jwtpkg.NewManager(
		cfg.JWT.AccessSecret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessExpHours,
		cfg.JWT.RefreshExpDays,
	)

	// Validator
	validate := validator.New()

	// Repositories
	userRepo := postgres.NewUserRepository(db)
	fileRepo := postgres.NewFileRepository(db)
	userStatsRepo := postgres.NewUserStatsRepository(db)
	readingRepo := postgres.NewReadingRepository(db)
	collectionRepo := postgres.NewCollectionRepository(db)

	// R2 cloud storage
	var r2Client *storage.R2
	if cfg.R2.IsConfigured() {
		var r2Err error
		r2Client, r2Err = storage.NewR2(cfg.R2.Endpoint, cfg.R2.AccessKey, cfg.R2.SecretKey, cfg.R2.Bucket, cfg.R2.PublicURL)
		if r2Err != nil {
			logger.Fatal("init r2 storage", zap.Error(r2Err))
		}
		logger.Info("r2 cloud storage enabled", zap.String("bucket", cfg.R2.Bucket))
	} else {
		logger.Info("r2 not configured — using local disk storage")
	}

	// Services
	fileService := service.NewFileService(fileRepo, readingRepo, userStatsRepo, logger, r2Client)

	// Convert existing pending files in the background (non-blocking)
	go fileService.BatchConvertPending(context.Background())

	// Handlers
	fileHandler := handler.NewFileHandler(fileService, validate, logger)
	readingHandler := handler.NewReadingHandler(readingRepo, validate, logger)
	collectionHandler := handler.NewCollectionHandler(collectionRepo, fileService, validate, logger)

	// Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		BodyLimit:    500 * 1024 * 1024, // 500MB for file uploads
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"data": nil, "error": err.Error()})
		},
	})

	// Global middleware
	app.Use(middleware.Recovery(logger))
	app.Use(middleware.JSONCharsetUTF8())
	app.Use(middleware.Logger(logger))
	app.Use(middleware.CORS())
	app.Use(middleware.SecurityHeaders(cfg.App.Env != "local"))

	// Serve uploaded files
	app.Static("/uploads", "./uploads", fiber.Static{Browse: false, ByteRange: true})

	// Health check (BACK-12). Postgres ping + 503 если недоступен.
	app.Get("/health", func(c *fiber.Ctx) error {
		dbCtx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(dbCtx); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status":  "down",
				"service": "library",
				"db":      err.Error(),
				"time":    time.Now().UTC(),
			})
		}
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "library",
			"db":      "ok",
			"time":    time.Now().UTC(),
		})
	})

	// API v1 routes
	api := app.Group("/api/v1")

	// Files
	api.Get("/files/categories", fileHandler.GetCategories)
	api.Get("/files/trending", middleware.OptionalAuth(jwtManager), fileHandler.Trending)
	api.Get("/files/authors/popular", fileHandler.PopularAuthors)
	api.Get("/files/stats/formats", fileHandler.FormatStats)
	api.Get("/files/suggestions", fileHandler.SearchSuggestions)
	api.Get("/files/social-picks", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.SocialPicks)
	api.Get("/files", middleware.OptionalAuth(jwtManager), fileHandler.ListFiles)
	api.Get("/files/:id", middleware.OptionalAuth(jwtManager), fileHandler.GetFile)
	api.Get("/files/:id/download", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.DownloadFile)
	api.Get("/files/:id/preview", middleware.OptionalAuth(jwtManager), fileHandler.PreviewFile)
	api.Get("/files/:id/text", middleware.OptionalAuth(jwtManager), fileHandler.GetText)
	api.Get("/files/:id/pdf", middleware.OptionalAuth(jwtManager), fileHandler.GetPDF)
	api.Get("/files/:id/pdf-status", middleware.OptionalAuth(jwtManager), fileHandler.GetPdfStatus)
	api.Post("/files/:id/re-extract", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.ReExtractText)
	api.Post("/files/upload", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.Upload)
	api.Post("/files", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.CreateFile)
	api.Patch("/files/:id", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.UpdateFile)
	api.Delete("/files/:id", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.DeleteFile)
	api.Post("/files/:id/view", middleware.OptionalAuth(jwtManager), fileHandler.TrackView)
	api.Put("/files/:id/rating", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.RateFile)
	api.Get("/files/:id/rating", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.GetUserRating)
	api.Get("/files/:id/reviews", middleware.OptionalAuth(jwtManager), fileHandler.GetFileReviews)
	api.Get("/files/:id/related", middleware.OptionalAuth(jwtManager), fileHandler.GetRelatedFiles)
	api.Post("/files/:id/like", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.LikeFile)
	api.Delete("/files/:id/like", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.UnlikeFile)

	// Reading progress, bookmarks, status (all require auth)
	auth := middleware.Auth(jwtManager, sessionStore, userRepo)
	api.Put("/files/:id/progress", auth, readingHandler.UpsertProgress)
	api.Get("/files/:id/progress", auth, readingHandler.GetProgress)
	api.Get("/files/:id/bookmarks", auth, readingHandler.GetBookmarks)
	api.Post("/files/:id/bookmarks", auth, readingHandler.CreateBookmark)
	api.Delete("/files/bookmarks/:bookmarkId", auth, readingHandler.DeleteBookmark)
	api.Get("/files/:id/reading-status", auth, readingHandler.GetReadingStatus)
	api.Put("/files/:id/reading-status", auth, readingHandler.UpsertReadingStatus)
	api.Delete("/files/:id/reading-status", auth, readingHandler.DeleteReadingStatus)
	api.Get("/files/:id/notes", auth, readingHandler.GetFileNote)
	api.Put("/files/:id/notes", auth, readingHandler.UpsertFileNote)
	api.Delete("/files/:id/notes", auth, readingHandler.DeleteFileNote)
	api.Get("/files/:id/pages-progress", auth, readingHandler.GetPageProgress)
	api.Put("/files/:id/pages-progress", auth, readingHandler.UpsertPageProgress)

	// /users/me/* MUST be registered BEFORE /users/:id/* to avoid wildcard shadowing
	api.Get("/users/me/reading-stats", auth, readingHandler.GetReadingStats)
	api.Get("/users/me/reading-list", auth, readingHandler.GetReadingList)
	api.Get("/users/me/recently-read", auth, readingHandler.GetRecentlyRead)
	api.Get("/users/me/recommendations", auth, fileHandler.Recommendations)
	api.Get("/users/me/recently-viewed", auth, fileHandler.GetRecentlyViewed)
	api.Get("/reading/leaderboard", middleware.OptionalAuth(jwtManager), readingHandler.GetLeaderboard)
	api.Get("/reading/activity", auth, readingHandler.GetReadingActivity)
	api.Get("/users/me/reading-goal", auth, readingHandler.GetReadingGoal)
	api.Put("/users/me/reading-goal", auth, readingHandler.UpsertReadingGoal)
	api.Delete("/users/me/reading-goal", auth, readingHandler.DeleteReadingGoal)

	// User files (wildcard :id — must come after /users/me/*)
	api.Get("/users/:id/files", middleware.OptionalAuth(jwtManager), fileHandler.GetUserFiles)

	// File stats (owner only)
	api.Get("/files/:id/stats", auth, collectionHandler.GetFileStats)

	// Collections (all require auth)
	api.Get("/collections", auth, collectionHandler.ListCollections)
	api.Post("/collections", auth, collectionHandler.CreateCollection)
	api.Get("/collections/:id", auth, collectionHandler.GetCollection)
	api.Put("/collections/:id", auth, collectionHandler.UpdateCollection)
	api.Delete("/collections/:id", auth, collectionHandler.DeleteCollection)
	api.Post("/collections/:id/files", auth, collectionHandler.AddFile)
	api.Delete("/collections/:id/files/:fileId", auth, collectionHandler.RemoveFile)

	// 404
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"data": nil, "error": "endpoint not found"})
	})

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		addr := "0.0.0.0:" + cfg.App.Port
		logger.Info("server listening", zap.String("addr", addr))
		if err := app.Listen(addr); err != nil {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	<-quit
	logger.Info("shutting down library service...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}
	logger.Info("library service stopped")
}

func buildLogger(env string) *zap.Logger {
	var cfg zap.Config
	if env == "production" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	logger, err := cfg.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build logger: %v", err))
	}
	return logger
}
