package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/repository/redis"
)

type RateLimitConfig struct {
	MaxRequests int
	Window      time.Duration
	KeyPrefix   string
}

func RateLimit(sessionStore *redis.SessionStore, cfg RateLimitConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.IP()
		userID := GetUserID(c)

		var key string
		if userID != "" {
			key = fmt.Sprintf("%s:user:%s", cfg.KeyPrefix, userID)
		} else {
			key = fmt.Sprintf("%s:ip:%s", cfg.KeyPrefix, ip)
		}

		count, err := sessionStore.SetRateLimit(c.Context(), key, cfg.MaxRequests, cfg.Window)
		if err != nil {
			// On redis error, allow request through
			return c.Next()
		}

		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", cfg.MaxRequests))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, int64(cfg.MaxRequests)-count)))

		if count > int64(cfg.MaxRequests) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"data":  nil,
				"error": "too many requests, please try again later",
			})
		}

		return c.Next()
	}
}

func AuthRateLimit(sessionStore *redis.SessionStore) fiber.Handler {
	return RateLimit(sessionStore, RateLimitConfig{
		MaxRequests: 20,
		Window:      time.Minute,
		KeyPrefix:   "rate:auth",
	})
}

func APIRateLimit(sessionStore *redis.SessionStore) fiber.Handler {
	return RateLimit(sessionStore, RateLimitConfig{
		MaxRequests: 300,
		Window:      time.Minute,
		KeyPrefix:   "rate:api",
	})
}

func UploadRateLimit(sessionStore *redis.SessionStore) fiber.Handler {
	return RateLimit(sessionStore, RateLimitConfig{
		MaxRequests: 30,
		Window:      time.Minute,
		KeyPrefix:   "rate:upload",
	})
}

// BACK-4: специализированные limiter'ы для самых spam-проне endpoint'ов.
// Глобальный APIRateLimit (300/min) — fallback; эти срабатывают раньше и
// бьют по конкретной операции от конкретного user'а (key включает userID).
//
// FollowRateLimit: 10/min — нормальный юзер не подписывается чаще; бот
// который пытается отфолловить 100 аккаунтов за минуту → блок на 11-м.
func FollowRateLimit(sessionStore *redis.SessionStore) fiber.Handler {
	return RateLimit(sessionStore, RateLimitConfig{
		MaxRequests: 10,
		Window:      time.Minute,
		KeyPrefix:   "rate:follow",
	})
}

// LikeRateLimit: 60/min — лайк может быть «двойной тап» спам, но 60 за
// минуту — это уже бот-поведение.
func LikeRateLimit(sessionStore *redis.SessionStore) fiber.Handler {
	return RateLimit(sessionStore, RateLimitConfig{
		MaxRequests: 60,
		Window:      time.Minute,
		KeyPrefix:   "rate:like",
	})
}

// CommentRateLimit: 20/min — комментарии редки, 20 за минуту — спамер.
func CommentRateLimit(sessionStore *redis.SessionStore) fiber.Handler {
	return RateLimit(sessionStore, RateLimitConfig{
		MaxRequests: 20,
		Window:      time.Minute,
		KeyPrefix:   "rate:comment",
	})
}

// PostCreateRateLimit: 5/min — создание постов редкое; 5/min — щедрый кап.
func PostCreateRateLimit(sessionStore *redis.SessionStore) fiber.Handler {
	return RateLimit(sessionStore, RateLimitConfig{
		MaxRequests: 5,
		Window:      time.Minute,
		KeyPrefix:   "rate:post-create",
	})
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
