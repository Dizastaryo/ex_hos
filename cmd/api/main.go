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
	fiberws "github.com/gofiber/websocket/v2"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/seeu/backend/config"
	"github.com/seeu/backend/internal/handler"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
	redisRepo "github.com/seeu/backend/internal/repository/redis"
	"github.com/seeu/backend/internal/service"
	"github.com/seeu/backend/internal/ws"
	jwtpkg "github.com/seeu/backend/pkg/jwt"
	"github.com/seeu/backend/pkg/storage"
	"github.com/seeu/backend/pkg/whatsapp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := buildLogger(cfg.App.Env)
	defer logger.Sync()

	logger.Info("starting seeu api server",
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
		// BACK-11: log queries > 100ms как WARN в api.err.log.
		SlowQueryLogger:    logger,
		SlowQueryThreshold: 100 * time.Millisecond,
	})
	if err != nil {
		logger.Fatal("connect to database", zap.Error(err))
	}
	defer db.Close()
	logger.Info("connected to postgresql")

	// Run migrations
	if err := runMigrations(cfg.Database.URL, logger); err != nil {
		logger.Fatal("run migrations", zap.Error(err))
	}

	// In-memory cache & session store
	cache, err := redisRepo.NewCache("")
	if err != nil {
		logger.Fatal("init cache", zap.Error(err))
	}
	defer cache.Close()
	logger.Info("in-memory cache ready")

	sessionStore := redisRepo.NewSessionStore(cache)

	// R2 cloud storage (optional — falls back to local disk if not configured)
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
	postRepo := postgres.NewPostRepository(db)
	storyRepo := postgres.NewStoryRepository(db)
	commentRepo := postgres.NewCommentRepository(db)
	likeRepo := postgres.NewLikeRepository(db)
	followRepo := postgres.NewFollowRepository(db)
	notifRepo := postgres.NewNotificationRepository(db)
	highlightRepo := postgres.NewHighlightRepository(db)
	chatRepo := postgres.NewChatRepository(db)
	callRepo := postgres.NewCallRepository(db)
	reportRepo := postgres.NewReportRepository(db)
	blockRepo := postgres.NewBlockRepository(db)
	restrictionRepo := postgres.NewRestrictionRepository(db)
	closeFriendsRepo := postgres.NewCloseFriendsRepository(db)
	inviteRepo := postgres.NewInviteRepository(db)
	playlistRepo := postgres.NewPlaylistRepository(db)
	auditRepo := postgres.NewAuditRepository(db)
	frRepo := postgres.NewFollowRequestRepository(db)
	searchHistoryRepo := postgres.NewSearchHistoryRepository(db)
	otpRepo := postgres.NewOTPRepository(db)
	userStatsRepo := postgres.NewUserStatsRepository(db)

	// WebSocket Hub — must be created before services that emit realtime events.
	wsHub := ws.NewHub(logger)
	// BUG-23: periodic metrics dump (5min). Stop при shutdown.
	wsHub.StartMetricsReporter()
	defer wsHub.StopMetricsReporter()
	// Presence hook: на connect/disconnect обновляем users.last_seen_at +
	// broadcast'им user.presence WS-event всем онлайн-юзерам. Frontend
	// фильтрует на стороне (показывает только если user_id релевантен —
	// открыт его профиль или чат с ним).
	wsHub.PresenceHook = func(userID string) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := userRepo.SetLastSeen(bgCtx, userID); err != nil {
			logger.Warn("set last_seen_at", zap.Error(err), zap.String("user_id", userID))
		}
		isOnline := wsHub.IsOnline(userID)
		wsHub.Broadcast("user.presence", map[string]any{
			"user_id":      userID,
			"is_online":    isOnline,
			"last_seen_at": time.Now().UTC().Format(time.RFC3339Nano),
		})
	}

	// WhatsApp bridge for OTP delivery (nil ⇒ dev-mode fallback in AuthService).
	whatsappClient := whatsapp.New(cfg.WhatsApp.ServiceURL)
	if whatsappClient != nil {
		logger.Info("whatsapp bridge configured", zap.String("url", cfg.WhatsApp.ServiceURL))
	} else {
		logger.Info("whatsapp bridge disabled (dev mode — OTP fallback = 0000)")
	}

	// Services
	authService := service.NewAuthService(service.AuthServiceDeps{
		UserRepo:     userRepo,
		OTPRepo:      otpRepo,
		SessionStore: sessionStore,
		JWTManager:   jwtManager,
		InviteRepo:   inviteRepo,
		WhatsApp:     whatsappClient,
		OTPTTL:       time.Duration(cfg.OTP.CodeTTLMinutes) * time.Minute,
		OTPMaxAtt:    cfg.OTP.MaxAttempts,
		OTPMaxPerHr:  cfg.OTP.MaxPerHour,
		Logger:       logger,
	})
	// userService инициализируется ниже, после device repos (нужен scannerRepo).
	var userService *service.UserService
	// MediaService must come before post/story services that take it as a dep
	// to release dedup refs on delete.
	mediaService := service.NewMediaService(db.Pool, logger, r2Client)
	postService := service.NewPostService(postRepo, userRepo, followRepo, cache, wsHub, mediaService, logger)
	storyService := service.NewStoryService(storyRepo, userRepo, followRepo, notifRepo, cache, wsHub, mediaService, logger)
	commentService := service.NewCommentService(commentRepo, postRepo, notifRepo, cache, wsHub, logger)
	likeService := service.NewLikeService(likeRepo, postRepo, commentRepo, storyRepo, notifRepo, userStatsRepo, cache, wsHub, logger)
	followService := service.NewFollowService(followRepo, frRepo, userRepo, notifRepo, blockRepo, cache, wsHub, logger)
	notifService := service.NewNotificationService(notifRepo, cache, logger)
	highlightService := service.NewHighlightService(highlightRepo, userRepo, followRepo, cache, logger)
	searchService := service.NewSearchService(userRepo, postRepo, logger)
	sborRepoForChat := postgres.NewSborRepository(db)
	chatService := service.NewChatService(chatRepo, sborRepoForChat, blockRepo, wsHub, logger)

	// CHAT-10.3: при подключении WS клиента — replay undelivered messages.
	// `wsHub.RegisterHook` зовётся уже после того как клиент добавлен в
	// map'у клиентов, поэтому SendToUser в ReplayUndeliveredFor найдёт
	// sender'ов (если они тоже online) и эмитнет chat.delivered.
	wsHub.RegisterHook = func(userID string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		chatService.ReplayUndeliveredFor(ctx, userID)
	}

	// CHAT-11 janitor: периодически удаляет expired-сообщения.
	// Тикает каждые 60 сек. Не блокирует main goroutine. Logs counts
	// чтобы было видно в api.err.log сколько сообщений ушло.
	//
	// BUG-13: graceful shutdown — janitor завершается когда janitorCtx
	// canceled (signal handler делает cancel при SIGINT). Раньше goroutine
	// застревал в `for range ticker.C` навсегда и shutdownContext-30s
	// expired не мог его прибить → main exit с in-flight DB-операцией.
	janitorCtx, janitorCancel := context.WithCancel(context.Background())
	defer janitorCancel()
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-janitorCtx.Done():
				logger.Info("chat janitor: shutting down")
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(janitorCtx, 30*time.Second)
				n, err := chatService.PurgeExpired(ctx)
				cancel()
				if err != nil {
					if janitorCtx.Err() != nil {
						// Cancel'нули ровно во время purge — не warn.
						return
					}
					logger.Warn("chat janitor: purge failed", zap.Error(err))
					continue
				}
				if n > 0 {
					logger.Info("chat janitor: purged expired messages",
						zap.Int64("count", n))
				}
			}
		}
	}()

	exportService := service.NewExportService(db.Pool, logger)
	reportService := service.NewReportService(reportRepo, logger)
	blockService := service.NewBlockService(blockRepo, userRepo, followRepo, logger)
	restrictionService := service.NewRestrictionService(restrictionRepo, userRepo, logger)
	closeFriendsService := service.NewCloseFriendsService(closeFriendsRepo, userRepo, logger)

	// BLE Device & Scanner
	deviceRepo := postgres.NewDeviceRepository(db)
	scannerRepo := postgres.NewScannerRepository(db)
	deviceService := service.NewDeviceService(deviceRepo, userRepo, cfg.Device.Secret, logger)
	// Пересоздаём userService с scannerRepo + statsRepo
	userService = service.NewUserService(userRepo, followRepo, frRepo, scannerRepo, userStatsRepo, wsHub, cache, logger)

	// Handlers
	authHandler := handler.NewAuthHandler(authService, validate, logger)
	userHandler := handler.NewUserHandler(userService, postService, followService, exportService, deviceService, validate, logger)
	postHandler := handler.NewPostHandler(postService, validate, logger)
	storyHandler := handler.NewStoryHandler(storyService, validate, logger)
	commentHandler := handler.NewCommentHandler(commentService, validate, logger)
	likeHandler := handler.NewLikeHandler(likeService, logger)
	followHandler := handler.NewFollowHandler(followService, logger)
	notifHandler := handler.NewNotificationHandler(notifService, logger)
	highlightHandler := handler.NewHighlightHandler(highlightService, validate, logger)
	mediaHandler := handler.NewMediaHandler(mediaService, r2Client, logger)
	searchHandler := handler.NewSearchHandler(searchService, searchHistoryRepo, logger)
	audioRepo := postgres.NewAudioRepository(db)
	audioHandler := handler.NewAudioHandler(audioRepo, userStatsRepo, logger)
	chatHandler := handler.NewChatHandler(chatService, logger)
	wsHandler := handler.NewWSHandler(wsHub, chatRepo, userRepo, callRepo, notifRepo, followRepo, audioRepo, logger)
	callHandler := handler.NewCallHandler(callRepo, logger)
	aiMasksRepo := postgres.NewAIMasksRepository(db)
	aiStylRepo := postgres.NewAIStylizationsRepository(db)
	aiHandler := handler.NewAIHandlerWithDeps(aiMasksRepo, aiStylRepo, logger)
	dailyPromptHandler := handler.NewDailyPromptHandler()
	reportHandler := handler.NewReportHandler(reportService, validate, logger)
	adminHandler := handler.NewAdminHandler(userRepo, reportRepo, auditRepo, audioRepo, deviceRepo, userService, deviceService, logger)
	blockHandler := handler.NewBlockHandler(blockService, logger)
	restrictionHandler := handler.NewRestrictionHandler(restrictionService, logger)
	closeFriendsHandler := handler.NewCloseFriendsHandler(closeFriendsService, logger)
	inviteHandler := handler.NewInviteHandler(inviteRepo, logger)
	playlistHandler := handler.NewPlaylistHandler(playlistRepo, logger)
	sborRepo := postgres.NewSborRepository(db)
	sborService := service.NewSborService(sborRepo, chatRepo, wsHub, logger)
	sborHandler := handler.NewSborHandler(sborService, validate, logger)
	roomRepo := postgres.NewRoomRepository(db)
	roomService := service.NewRoomService(roomRepo, wsHub, logger)
	roomHandler := handler.NewRoomHandler(roomService, validate, logger)
	stickerRepo := postgres.NewStickerRepository(db)
	stickerHandler := handler.NewStickerHandler(stickerRepo, r2Client, logger)
	scannerService := service.NewScannerService(scannerRepo, userRepo, notifRepo, userStatsRepo, wsHub, logger)
	scannerHandler := handler.NewScannerHandler(scannerService, logger)

	// Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		BodyLimit:    110 * 1024 * 1024, // 110MB for video uploads
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"data":  nil,
				"error": err.Error(),
			})
		},
	})

	// Global middleware
	app.Use(middleware.Recovery(logger))
	app.Use(middleware.JSONCharsetUTF8())
	app.Use(middleware.Logger(logger))
	app.Use(middleware.CORS())
	app.Use(middleware.SecurityHeaders(cfg.App.Env != "local"))

	// Serve uploaded media files (ByteRange enables HTTP Range for video streaming)
	app.Static("/uploads", "./uploads", fiber.Static{
		Browse:    false,
		ByteRange: true,
	})

	// Legal documents (Privacy Policy, Terms of Service) — required for App Store.
	app.Static("/legal", "./legal")
	app.Get("/privacy", func(c *fiber.Ctx) error {
		return c.SendFile("./legal/privacy.html")
	})
	app.Get("/terms", func(c *fiber.Ctx) error {
		return c.SendFile("./legal/terms.html")
	})

	// Health check (BACK-12) — реально проверяет dependencies. 503 если
	// Postgres недоступен или uploads-dir не writable. Frontend смотрит на
	// status code: 200=OK, 503=service down.
	app.Get("/health", func(c *fiber.Ctx) error {
		checks := fiber.Map{"db": "ok"}
		if r2Client != nil {
			checks["storage"] = "r2"
		} else {
			checks["storage"] = "local"
		}
		statusCode := fiber.StatusOK
		// DB ping с коротким таймаутом — не блочим health-check надолго.
		dbCtx, dbCancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer dbCancel()
		if err := db.Ping(dbCtx); err != nil {
			checks["db"] = err.Error()
			statusCode = fiber.StatusServiceUnavailable
		}
		// Uploads-dir writable only when using local storage.
		if r2Client == nil {
			probe := fmt.Sprintf("uploads/.health_%d", time.Now().UnixNano())
			if f, err := os.Create(probe); err != nil {
				checks["storage"] = err.Error()
				statusCode = fiber.StatusServiceUnavailable
			} else {
				f.Close()
				os.Remove(probe)
			}
		}
		return c.Status(statusCode).JSON(fiber.Map{
			"status":  map[bool]string{true: "ok", false: "down"}[statusCode == fiber.StatusOK],
			"version": "1.0.0",
			"time":    time.Now().UTC(),
			"checks":  checks,
		})
	})

	// API v1 routes
	api := app.Group("/api/v1")

	// Auth routes (rate limited)
	auth := api.Group("/auth", middleware.AuthRateLimit(sessionStore))
	auth.Post("/send-otp", authHandler.SendOTP)
	auth.Post("/verify-otp", authHandler.VerifyOTP)
	auth.Post("/refresh", authHandler.Refresh)
	auth.Post("/logout", middleware.Auth(jwtManager, sessionStore, userRepo), authHandler.Logout)

	// API rate limit for all remaining routes
	api.Use(middleware.APIRateLimit(sessionStore))

	// User routes
	users := api.Group("/users")
	users.Get("/me", middleware.Auth(jwtManager, sessionStore, userRepo), userHandler.GetMe)
	users.Put("/me", middleware.Auth(jwtManager, sessionStore, userRepo), userHandler.UpdateMe)
	users.Delete("/me", middleware.Auth(jwtManager, sessionStore, userRepo), userHandler.DeleteMe)
	users.Get("/me/calls", middleware.Auth(jwtManager, sessionStore, userRepo), callHandler.GetMyCalls)
	// BUG-5: догонка pending звонков при reconnect. Frontend дёргает после
	// успешного re-connect'а WS чтобы не потерять invite пришедший в downtime.
	users.Get("/me/calls/pending", middleware.Auth(jwtManager, sessionStore, userRepo), callHandler.GetPendingCalls)
	users.Get("/me/export", middleware.Auth(jwtManager, sessionStore, userRepo), userHandler.ExportMe)
	users.Get("/me/mutuals", middleware.Auth(jwtManager, sessionStore, userRepo), userHandler.GetMutuals)
	users.Get("/me/blocks", middleware.Auth(jwtManager, sessionStore, userRepo), blockHandler.ListBlocked)
	users.Get("/me/follow-requests", middleware.Auth(jwtManager, sessionStore, userRepo), followHandler.ListMyFollowRequests)
	users.Post("/me/device", middleware.Auth(jwtManager, sessionStore, userRepo), userHandler.BindMyDevice)
	users.Delete("/me/device", middleware.Auth(jwtManager, sessionStore, userRepo), userHandler.UnbindMyDevice)
	users.Put("/me/scan-profile", middleware.Auth(jwtManager, sessionStore, userRepo), userHandler.UpdateScanProfile)
	users.Get("/me/private-whitelist", middleware.Auth(jwtManager, sessionStore, userRepo), userHandler.GetPrivateWhitelist)
	users.Put("/me/private-whitelist", middleware.Auth(jwtManager, sessionStore, userRepo), userHandler.SetPrivateWhitelist)
	users.Post("/:username/block", middleware.Auth(jwtManager, sessionStore, userRepo), blockHandler.Block)
	users.Delete("/:username/block", middleware.Auth(jwtManager, sessionStore, userRepo), blockHandler.Unblock)
	users.Get("/me/restrictions", middleware.Auth(jwtManager, sessionStore, userRepo), restrictionHandler.ListRestricted)
	users.Post("/:username/restrict", middleware.Auth(jwtManager, sessionStore, userRepo), restrictionHandler.Restrict)
	users.Delete("/:username/restrict", middleware.Auth(jwtManager, sessionStore, userRepo), restrictionHandler.Unrestrict)
	users.Get("/me/close-friends", middleware.Auth(jwtManager, sessionStore, userRepo), closeFriendsHandler.List)
	users.Post("/:username/close-friend", middleware.Auth(jwtManager, sessionStore, userRepo), closeFriendsHandler.Add)
	users.Delete("/:username/close-friend", middleware.Auth(jwtManager, sessionStore, userRepo), closeFriendsHandler.Remove)
	// Static-segment lookups must come before /:username catch-all.
	users.Get("/by-device/:publicId", middleware.OptionalAuth(jwtManager), userHandler.GetByDevice)
	// BUG-17: приватный BLE-id (mode=0x01) лукап. Whitelist'ed search по
	// follow'ed-юзерам, auth required (anti brute-force).
	users.Get("/by-device-private/:privateId",
		middleware.Auth(jwtManager, sessionStore, userRepo),
		userHandler.GetByDevicePrivate)
	users.Get("/:username", middleware.OptionalAuth(jwtManager), userHandler.GetByUsername)
	users.Get("/:username/posts", middleware.OptionalAuth(jwtManager), userHandler.GetUserPosts)
	users.Get("/:username/saved", middleware.Auth(jwtManager, sessionStore, userRepo), userHandler.GetSavedPosts)
	users.Get("/:username/followers", middleware.OptionalAuth(jwtManager), userHandler.GetFollowers)
	users.Get("/:username/following", middleware.OptionalAuth(jwtManager), userHandler.GetFollowing)
	// BACK-4: tighter follow rate-limit (10/min) поверх глобального APIRateLimit.
	users.Post("/:username/follow", middleware.Auth(jwtManager, sessionStore, userRepo), middleware.FollowRateLimit(sessionStore), followHandler.Follow)
	users.Delete("/:username/follow", middleware.Auth(jwtManager, sessionStore, userRepo), middleware.FollowRateLimit(sessionStore), followHandler.Unfollow)

	// Follow request inbox actions (target accepts/declines requests
	// addressed to themselves).
	api.Post("/follow-requests/:id/accept",
		middleware.Auth(jwtManager, sessionStore, userRepo),
		followHandler.AcceptFollowRequest)
	api.Post("/follow-requests/:id/decline",
		middleware.Auth(jwtManager, sessionStore, userRepo),
		followHandler.DeclineFollowRequest)

	// Post routes
	// BACK-4: post-create rate-limit (5/min).
	api.Post("/posts", middleware.Auth(jwtManager, sessionStore, userRepo), middleware.PostCreateRateLimit(sessionStore), postHandler.CreatePost)
	api.Get("/posts/:id", middleware.OptionalAuth(jwtManager), postHandler.GetPost)
	api.Delete("/posts/:id", middleware.Auth(jwtManager, sessionStore, userRepo), postHandler.DeletePost)
	api.Get("/feed", middleware.Auth(jwtManager, sessionStore, userRepo), postHandler.GetFeed)
	api.Get("/explore", middleware.OptionalAuth(jwtManager), postHandler.GetExplore)

	// Like routes
	api.Post("/posts/:id/react", middleware.Auth(jwtManager, sessionStore, userRepo), postHandler.React)
	api.Post("/posts/:id/view", middleware.Auth(jwtManager, sessionStore, userRepo), postHandler.MarkViewed)
	api.Delete("/posts/:id/react", middleware.Auth(jwtManager, sessionStore, userRepo), postHandler.Unreact)
	// BACK-4: like rate-limit (60/min) на все like-endpoint'ы.
	api.Post("/posts/:id/like", middleware.Auth(jwtManager, sessionStore, userRepo), middleware.LikeRateLimit(sessionStore), likeHandler.LikePost)
	api.Delete("/posts/:id/like", middleware.Auth(jwtManager, sessionStore, userRepo), middleware.LikeRateLimit(sessionStore), likeHandler.UnlikePost)
	api.Post("/stories/:id/like", middleware.Auth(jwtManager, sessionStore, userRepo), middleware.LikeRateLimit(sessionStore), likeHandler.LikeStory)
	api.Delete("/stories/:id/like", middleware.Auth(jwtManager, sessionStore, userRepo), middleware.LikeRateLimit(sessionStore), likeHandler.UnlikeStory)
	api.Post("/comments/:id/like", middleware.Auth(jwtManager, sessionStore, userRepo), middleware.LikeRateLimit(sessionStore), likeHandler.LikeComment)
	api.Delete("/comments/:id/like", middleware.Auth(jwtManager, sessionStore, userRepo), middleware.LikeRateLimit(sessionStore), likeHandler.UnlikeComment)

	// Save routes
	api.Post("/posts/:id/save", middleware.Auth(jwtManager, sessionStore, userRepo), likeHandler.SavePost)
	api.Delete("/posts/:id/save", middleware.Auth(jwtManager, sessionStore, userRepo), likeHandler.UnsavePost)

	// Comment routes
	api.Get("/posts/:id/comments", middleware.OptionalAuth(jwtManager), commentHandler.GetComments)
	// BACK-4: comment rate-limit (20/min).
	api.Post("/posts/:id/comments", middleware.Auth(jwtManager, sessionStore, userRepo), middleware.CommentRateLimit(sessionStore), commentHandler.CreateComment)
	api.Delete("/comments/:id", middleware.Auth(jwtManager, sessionStore, userRepo), commentHandler.DeleteComment)
	api.Get("/comments/:id/replies", middleware.OptionalAuth(jwtManager), commentHandler.GetReplies)

	// Story routes
	api.Post("/stories", middleware.Auth(jwtManager, sessionStore, userRepo), storyHandler.CreateStory)
	api.Get("/stories/feed", middleware.Auth(jwtManager, sessionStore, userRepo), storyHandler.GetStoryFeed)
	api.Get("/stories/:username", middleware.OptionalAuth(jwtManager), storyHandler.GetUserStories)
	api.Delete("/stories/:id", middleware.Auth(jwtManager, sessionStore, userRepo), storyHandler.DeleteStory)
	api.Post("/stories/:id/view", middleware.Auth(jwtManager, sessionStore, userRepo), storyHandler.ViewStory)
	api.Get("/stories/:id/viewers", middleware.Auth(jwtManager, sessionStore, userRepo), storyHandler.GetStoryViewers)
	api.Post("/stories/:id/react", middleware.Auth(jwtManager, sessionStore, userRepo), storyHandler.React)
	api.Delete("/stories/:id/react", middleware.Auth(jwtManager, sessionStore, userRepo), storyHandler.Unreact)
	api.Post("/stories/:id/poll-vote", middleware.Auth(jwtManager, sessionStore, userRepo), storyHandler.VotePoll)

	// Highlight routes
	api.Get("/highlights/:username", middleware.OptionalAuth(jwtManager), highlightHandler.GetHighlights)
	api.Post("/highlights", middleware.Auth(jwtManager, sessionStore, userRepo), highlightHandler.CreateHighlight)
	api.Put("/highlights/:id", middleware.Auth(jwtManager, sessionStore, userRepo), highlightHandler.UpdateHighlight)
	api.Delete("/highlights/:id", middleware.Auth(jwtManager, sessionStore, userRepo), highlightHandler.DeleteHighlight)

	// Media routes
	api.Post("/media/upload",
		middleware.Auth(jwtManager, sessionStore, userRepo),
		middleware.UploadRateLimit(sessionStore),
		mediaHandler.Upload,
	)

	// Video thumbnail generation
	api.Post("/media/video-thumbnail",
		middleware.Auth(jwtManager, sessionStore, userRepo),
		mediaHandler.VideoThumbnail,
	)

	// Daily prompt — deterministic per-day Russian question shown above feed.
	api.Get("/daily-prompt", middleware.OptionalAuth(jwtManager), dailyPromptHandler.GetDailyPrompt)

	// Search
	api.Get("/search", middleware.OptionalAuth(jwtManager), searchHandler.Search)
	api.Get("/search/history",
		middleware.Auth(jwtManager, sessionStore, userRepo),
		searchHandler.SearchHistory)
	api.Delete("/search/history",
		middleware.Auth(jwtManager, sessionStore, userRepo),
		searchHandler.DeleteSearchHistory)

	// Audio tracks
	api.Get("/audio-tracks", middleware.OptionalAuth(jwtManager), audioHandler.GetTracks)
	api.Post("/audio-tracks", middleware.Auth(jwtManager, sessionStore, userRepo), audioHandler.CreateTrack)
	api.Get("/audio-tracks/me", middleware.Auth(jwtManager, sessionStore, userRepo), audioHandler.ListMine)
	// MUSIC-3/4: smart-playlists + daily-mix. ORDER важен — specific paths
	// до :id чтобы Fiber не матчил «recent» как id.
	api.Get("/audio-tracks/recent", middleware.Auth(jwtManager, sessionStore, userRepo), audioHandler.ListRecent)
	api.Get("/audio-tracks/liked", middleware.Auth(jwtManager, sessionStore, userRepo), audioHandler.ListLiked)
	api.Get("/audio-tracks/daily-mix", middleware.Auth(jwtManager, sessionStore, userRepo), audioHandler.DailyMix)
	api.Get("/audio-tracks/:id", middleware.OptionalAuth(jwtManager), audioHandler.GetTrackByID)
	api.Post("/audio-tracks/:id/play", middleware.Auth(jwtManager, sessionStore, userRepo), audioHandler.RecordPlay)
	api.Post("/audio-tracks/:id/like", middleware.Auth(jwtManager, sessionStore, userRepo), audioHandler.LikeTrack)
	api.Delete("/audio-tracks/:id/like", middleware.Auth(jwtManager, sessionStore, userRepo), audioHandler.UnlikeTrack)

	// Playlists (Music v2)
	playlists := api.Group("/playlists", middleware.Auth(jwtManager, sessionStore, userRepo))
	playlists.Get("/me", playlistHandler.ListMine)
	playlists.Post("/", playlistHandler.Create)
	playlists.Get("/:id", playlistHandler.Get)
	playlists.Patch("/:id", playlistHandler.Update)
	playlists.Delete("/:id", playlistHandler.Delete)
	playlists.Post("/:id/tracks", playlistHandler.AddTrack)
	playlists.Delete("/:id/tracks/:trackId", playlistHandler.RemoveTrack)

	// Trending tags
	api.Get("/tags/trending", middleware.OptionalAuth(jwtManager), audioHandler.GetTrendingTags)

	// Сборы
	sbory := api.Group("/sbory", middleware.Auth(jwtManager, sessionStore, userRepo))
	sbory.Post("/", sborHandler.Create)
	sbory.Get("/", sborHandler.List)
	sbory.Get("/me", sborHandler.ListMine)
	sbory.Get("/bookmarked", sborHandler.ListBookmarked)
	sbory.Get("/:id", sborHandler.GetByID)
	sbory.Patch("/:id", sborHandler.Update)
	sbory.Delete("/:id", sborHandler.Cancel)
	sbory.Post("/:id/join", sborHandler.Join)
	sbory.Delete("/:id/join", sborHandler.Leave)
	sbory.Post("/:id/bookmark", sborHandler.ToggleBookmark)
	sbory.Post("/:id/requests", sborHandler.SubmitRequest)
	sbory.Delete("/:id/requests", sborHandler.CancelRequest)
	sbory.Get("/:id/requests", sborHandler.ListRequests)
	sbory.Post("/:id/requests/:reqID/approve", sborHandler.ApproveRequest)
	sbory.Get("/:id/members", sborHandler.GetMembers)
	sbory.Post("/:id/requests/:reqID/reject", sborHandler.RejectRequest)

	// Rooms (Discord-style voice + text rooms)
	rooms := api.Group("/rooms", middleware.Auth(jwtManager, sessionStore, userRepo))
	rooms.Get("/", roomHandler.List)
	rooms.Post("/", roomHandler.Create)
	rooms.Get("/invites/me", roomHandler.GetMyInvites)
	rooms.Post("/invites/:inviteId/accept", roomHandler.AcceptInvite)
	rooms.Post("/invites/:inviteId/decline", roomHandler.DeclineInvite)
	rooms.Get("/:id", roomHandler.GetByID)
	rooms.Delete("/:id", roomHandler.Close)
	rooms.Post("/:id/join", roomHandler.Join)
	rooms.Delete("/:id/join", roomHandler.Leave)
	rooms.Post("/:id/voice", roomHandler.JoinVoice)
	rooms.Delete("/:id/voice", roomHandler.LeaveVoice)
	rooms.Patch("/:id/mute", roomHandler.ToggleMute)
	rooms.Get("/:id/members", roomHandler.GetMembers)
	rooms.Get("/:id/candidates", roomHandler.GetCandidates)
	rooms.Post("/:id/invite", roomHandler.InviteMember)
	rooms.Delete("/:id/members/:userId", roomHandler.RemoveMember)
	rooms.Patch("/:id", roomHandler.Update)
	rooms.Post("/:id/admins/:userId", roomHandler.GrantAdmin)
	rooms.Delete("/:id/admins/:userId", roomHandler.RevokeAdmin)
	rooms.Get("/:id/messages", roomHandler.GetMessages)
	rooms.Post("/:id/messages", roomHandler.SendMessage)
	rooms.Post("/:id/messages/:msgId/react", roomHandler.ReactMessage)

	// Stickers
	stickersGroup := api.Group("/stickers", middleware.Auth(jwtManager, sessionStore, userRepo))
	stickersGroup.Post("/remove-bg", stickerHandler.RemoveBg)
	stickersGroup.Get("/", stickerHandler.List)
	stickersGroup.Post("/", stickerHandler.Create)
	stickersGroup.Delete("/:id", stickerHandler.Delete)

	// Notifications
	api.Get("/notifications", middleware.Auth(jwtManager, sessionStore, userRepo), notifHandler.GetNotifications)
	api.Put("/notifications/read", middleware.Auth(jwtManager, sessionStore, userRepo), notifHandler.MarkAllRead)
	api.Put("/notifications/:id/read", middleware.Auth(jwtManager, sessionStore, userRepo), notifHandler.MarkRead)

	// Chat routes
	chats := api.Group("/chats", middleware.Auth(jwtManager, sessionStore, userRepo))
	chats.Get("/", chatHandler.ListChats)
	chats.Post("/", chatHandler.CreateChat)
	chats.Get("/:id/messages", chatHandler.GetMessages)
	chats.Post("/:id/messages", chatHandler.SendMessage)
	chats.Patch("/:id/messages/:message_id", chatHandler.EditMessage)
	chats.Put("/:id/read", chatHandler.MarkRead)
	// Group-chat management
	chats.Put("/:id", chatHandler.UpdateGroup)
	chats.Get("/:id/members", chatHandler.GetGroupMembers)
	chats.Post("/:id/members", chatHandler.AddGroupMember)
	chats.Delete("/:id/members/:user_id", chatHandler.RemoveGroupMember)
	chats.Delete("/:id/leave", chatHandler.LeaveGroup)
	chats.Put("/:id/members/:user_id/role", chatHandler.ChangeMemberRole)
	chats.Put("/:id/pin", chatHandler.PinMessage)
	chats.Put("/:id/user-pin", chatHandler.TogglePinConversation)
	chats.Patch("/:id/archive", chatHandler.ArchiveChat)
	chats.Patch("/:id/mute", chatHandler.MuteChat)
	chats.Delete("/:id", chatHandler.HideConversation)

	// Reactions on chat messages — separate group because the URL is
	// keyed by message id, not chat id.
	chatMsgs := api.Group("/chat-messages",
		middleware.Auth(jwtManager, sessionStore, userRepo))
	chatMsgs.Post("/:id/react", chatHandler.React)
	chatMsgs.Delete("/:id/react", chatHandler.Unreact)
	chatMsgs.Delete("/:id", chatHandler.DeleteMessage)

	// AI routes
	api.Post("/ai/generate-filter", middleware.Auth(jwtManager, sessionStore, userRepo), aiHandler.GenerateFilter)
	api.Post("/ai/mask", middleware.Auth(jwtManager, sessionStore, userRepo), aiHandler.GenerateMask)
	api.Get("/ai/masks", middleware.Auth(jwtManager, sessionStore, userRepo), aiHandler.ListMasks)
	api.Delete("/ai/masks/:id", middleware.Auth(jwtManager, sessionStore, userRepo), aiHandler.DeleteMask)
	api.Post("/ai/stylize", middleware.Auth(jwtManager, sessionStore, userRepo), aiHandler.Stylize)
	api.Get("/ai/stylizations", middleware.Auth(jwtManager, sessionStore, userRepo), aiHandler.ListStylizations)
	api.Post("/ai/caption", middleware.Auth(jwtManager, sessionStore, userRepo), aiHandler.GenerateCaption)

	// Reports (content moderation)
	api.Post("/reports", middleware.Auth(jwtManager, sessionStore, userRepo), reportHandler.Create)

	// Invites (referral codes)
	api.Get("/invites/me/list", middleware.Auth(jwtManager, sessionStore, userRepo), inviteHandler.MyInvites)
	api.Post("/invites", middleware.Auth(jwtManager, sessionStore, userRepo), inviteHandler.Create)
	api.Get("/invites/:code", inviteHandler.Lookup) // public — shown on auth screen

	// Scanner (BLE bracelet likes)
	scanner := api.Group("/scanner", middleware.Auth(jwtManager, sessionStore, userRepo))
	scanner.Post("/like", scannerHandler.PostLike)
	scanner.Delete("/like/:deviceHash", scannerHandler.DeleteLike)
	scanner.Get("/likes/received", scannerHandler.GetReceivedLikes)
	scanner.Get("/likes/sent", scannerHandler.GetSentLikes)

	// Admin routes — for the admin.seeu.kz front-end. AdminOnly is applied AFTER
	// Auth, so unauthenticated calls get 401 and authenticated non-admins get 403.
	admin := api.Group("/admin",
		middleware.Auth(jwtManager, sessionStore, userRepo),
		middleware.AdminOnly(),
	)
	admin.Get("/reports", adminHandler.ListReports)
	admin.Post("/reports/:id/dismiss", adminHandler.DismissReport)
	admin.Post("/reports/:id/actioned", adminHandler.ActionReport)
	admin.Get("/users", adminHandler.ListUsers)
	admin.Post("/users/:id/ban", adminHandler.BanUser)
	admin.Post("/users/:id/unban", adminHandler.UnbanUser)
	admin.Post("/users/:id/verify", adminHandler.VerifyUser)
	admin.Post("/users/:id/unverify", adminHandler.UnverifyUser)
	admin.Delete("/users/:id", adminHandler.DeleteUser)
	admin.Get("/metrics", adminHandler.Metrics)
	admin.Get("/metrics/timeseries", adminHandler.MetricsTimeSeries)
	admin.Get("/audit-log", adminHandler.AuditLog)
	admin.Get("/audio-tracks", adminHandler.ListAudioTracks)
	admin.Post("/audio-tracks/:id/approve", adminHandler.ApproveAudioTrack)
	admin.Post("/audio-tracks/:id/reject", adminHandler.RejectAudioTrack)

	// BLE devices — генерация, список, CSV-экспорт для прошивки
	admin.Post("/devices/generate", adminHandler.GenerateDevices)
	admin.Get("/devices/export.csv", adminHandler.ExportDevicesCSV)
	admin.Get("/devices", adminHandler.ListDevices)
	admin.Delete("/devices/:id", adminHandler.DeactivateDevice)

	// WebSocket — requires upgrade check middleware before the actual handler
	api.Get("/ws",
		middleware.Auth(jwtManager, sessionStore, userRepo),
		fiberws.New(wsHandler.Handle),
	)

	// 404 handler
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"data":  nil,
			"error": "endpoint not found",
		})
	})

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		// `0.0.0.0:port` — слушаем на всех интерфейсах, чтобы устройства в
		// одной локальной сети (телефон ↔ ноут-сервер по Wi-Fi) могли достучаться.
		addr := "0.0.0.0:" + cfg.App.Port
		logger.Info("server listening", zap.String("addr", addr))
		if err := app.Listen(addr); err != nil {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	<-quit
	logger.Info("shutting down server...")

	// BUG-13: останавливаем janitor ДО app.Shutdown — он держит DB-ctx,
	// app.Shutdown ждёт drain in-flight requests, потом мы закрываем DB.
	janitorCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}

	logger.Info("server stopped")
}

// runMigrations is the canonical migrator — `cmd/api` is the only service
// that runs `migrate up` on boot. `cmd/video` and `cmd/library` share the
// same database but no longer touch migrations to keep startup quiet (the
// previous setup hit the same migration set three times per restart and
// logged "no change" twice).
//
// Override with MIGRATE_ON_START=false to skip even from api (useful for
// staging/production where migrations run as a separate deploy step).
func runMigrations(databaseURL string, logger *zap.Logger) error {
	if os.Getenv("MIGRATE_ON_START") == "false" {
		logger.Info("skipping migrations (MIGRATE_ON_START=false)")
		return nil
	}
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	logger.Info("migrations applied successfully")
	return nil
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
