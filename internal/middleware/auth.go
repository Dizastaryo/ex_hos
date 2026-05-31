package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/internal/repository/redis"
	jwtpkg "github.com/seeu/backend/pkg/jwt"
)

const (
	UserIDKey  = "user_id"
	IsAdminKey = "is_admin"
)

func Auth(jwtManager *jwtpkg.Manager, sessionStore *redis.SessionStore, userRepo *postgres.UserRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := extractToken(c)
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"data":  nil,
				"error": "missing or invalid authorization token",
			})
		}

		claims, err := jwtManager.ValidateAccessToken(token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"data":  nil,
				"error": "invalid or expired token",
			})
		}

		blacklisted, err := sessionStore.IsTokenBlacklisted(c.Context(), token)
		if err == nil && blacklisted {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"data":  nil,
				"error": "token has been revoked",
			})
		}

		// Block banned accounts at the gate. We pay one tiny SELECT per request
		// to ensure that bans take effect immediately, even on tokens issued
		// before the ban.
		flags, err := userRepo.GetAuthFlags(c.Context(), claims.UserID)
		if err == nil && flags.IsBanned {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"data":  nil,
				"error": "account_disabled",
			})
		}

		c.Locals(UserIDKey, claims.UserID)
		c.Locals(IsAdminKey, flags.IsAdmin)
		return c.Next()
	}
}

// AdminOnly must be chained after Auth — it relies on the IsAdminKey local
// that Auth populates.
func AdminOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		isAdmin, _ := c.Locals(IsAdminKey).(bool)
		if !isAdmin {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"data":  nil,
				"error": "admin access required",
			})
		}
		return c.Next()
	}
}

func OptionalAuth(jwtManager *jwtpkg.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := extractToken(c)
		if token != "" {
			if claims, err := jwtManager.ValidateAccessToken(token); err == nil {
				c.Locals(UserIDKey, claims.UserID)
			}
		}
		return c.Next()
	}
}

func extractToken(c *fiber.Ctx) string {
	authHeader := c.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return parts[1]
		}
	}

	// Cookie fallback (used for media downloads).
	if cookie := c.Cookies("access_token"); cookie != "" {
		return cookie
	}

	// Query-param fallback for protocols that don't allow custom headers
	// (browser WebSocket handshake, image/video tags with auth-protected sources).
	return c.Query("token")
}

func GetUserID(c *fiber.Ctx) string {
	userID, _ := c.Locals(UserIDKey).(string)
	return userID
}
