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

	// Services
	fileService := service.NewFileService(fileRepo, logger)

	// Handlers
	fileHandler := handler.NewFileHandler(fileService, validate, logger)

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
	api.Get("/files/trending", fileHandler.Trending)
	api.Get("/files", middleware.OptionalAuth(jwtManager), fileHandler.ListFiles)
	api.Get("/files/:id", middleware.OptionalAuth(jwtManager), fileHandler.GetFile)
	api.Get("/files/:id/download", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.DownloadFile)
	api.Get("/files/:id/preview", middleware.OptionalAuth(jwtManager), fileHandler.PreviewFile)
	api.Post("/files/upload", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.Upload)
	api.Post("/files", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.CreateFile)
	api.Delete("/files/:id", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.DeleteFile)
	api.Post("/files/:id/like", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.LikeFile)
	api.Delete("/files/:id/like", middleware.Auth(jwtManager, sessionStore, userRepo), fileHandler.UnlikeFile)

	// User files
	api.Get("/users/:id/files", middleware.OptionalAuth(jwtManager), fileHandler.GetUserFiles)

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
