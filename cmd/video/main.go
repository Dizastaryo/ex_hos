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
	cfg.App.Port = "8002"

	logger := buildLogger(cfg.App.Env)
	defer logger.Sync()

	logger.Info("starting seeu video service",
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

	// Migrations are owned by `cmd/api`; video reuses the same database
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
	videoRepo := postgres.NewVideoRepository(db)
	videoCommentRepo := postgres.NewVideoCommentRepository(db)

	// Services
	videoService := service.NewVideoService(videoRepo, logger)

	// Handlers
	videoHandler := handler.NewVideoHandler(videoService, validate, logger)
	videoCommentHandler := handler.NewVideoCommentHandler(videoCommentRepo, logger)

	// Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		BodyLimit:    200 * 1024 * 1024, // 200MB for video uploads
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

	// Serve uploaded media
	app.Static("/uploads", "./uploads", fiber.Static{Browse: false, ByteRange: true})

	// Health check (BACK-12). Postgres ping + 503 если недоступен.
	app.Get("/health", func(c *fiber.Ctx) error {
		dbCtx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(dbCtx); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status":  "down",
				"service": "video",
				"db":      err.Error(),
				"time":    time.Now().UTC(),
			})
		}
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "video",
			"db":      "ok",
			"time":    time.Now().UTC(),
		})
	})

	// API v1 routes
	api := app.Group("/api/v1")

	// Videos
	api.Get("/videos/categories", videoHandler.GetCategories)
	api.Get("/videos/featured", videoHandler.GetFeatured)
	api.Get("/videos", middleware.OptionalAuth(jwtManager), videoHandler.ListVideos)
	api.Get("/videos/:id", middleware.OptionalAuth(jwtManager), videoHandler.GetVideo)
	api.Post("/videos", middleware.Auth(jwtManager, sessionStore, userRepo), videoHandler.CreateVideo)
	api.Delete("/videos/:id", middleware.Auth(jwtManager, sessionStore, userRepo), videoHandler.DeleteVideo)
	api.Get("/videos/:id/comments", middleware.OptionalAuth(jwtManager), videoCommentHandler.List)
	api.Post("/videos/:id/comments", middleware.Auth(jwtManager, sessionStore, userRepo), videoCommentHandler.Create)
	api.Delete("/video-comments/:id", middleware.Auth(jwtManager, sessionStore, userRepo), videoCommentHandler.Delete)
	api.Post("/videos/:id/view", middleware.Auth(jwtManager, sessionStore, userRepo), videoHandler.ViewVideo)
	api.Post("/videos/:id/like", middleware.Auth(jwtManager, sessionStore, userRepo), videoHandler.LikeVideo)
	api.Delete("/videos/:id/like", middleware.Auth(jwtManager, sessionStore, userRepo), videoHandler.UnlikeVideo)

	// Reels endpoints removed — every publication is now a unified post
	// served by the api service. See migration 000023_unify_posts_reels.

	// User content
	api.Get("/users/:id/videos", middleware.OptionalAuth(jwtManager), videoHandler.GetUserVideos)

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
	logger.Info("shutting down video service...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}
	logger.Info("video service stopped")
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
