package middleware

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORS возвращает middleware с whitelist'ом origin'ов.
// CORS_ALLOWED_ORIGINS env var, comma-separated. Пусто → allow-all (dev).
// Прод: CORS_ALLOWED_ORIGINS=https://seeu.kz,https://admin.seeu.kz
func CORS() fiber.Handler {
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	var whitelist []string
	if raw != "" {
		for _, o := range strings.Split(raw, ",") {
			if v := strings.TrimSpace(o); v != "" {
				whitelist = append(whitelist, v)
			}
		}
	}

	return cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			// Dev: пустой whitelist → разрешаем всё (флаттер с любого порта).
			if len(whitelist) == 0 {
				return true
			}
			for _, allowed := range whitelist {
				if allowed == origin {
					return true
				}
			}
			return false
		},
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Request-ID",
		ExposeHeaders:    "X-RateLimit-Limit,X-RateLimit-Remaining,X-Request-ID",
		AllowCredentials: false,
		MaxAge:           86400,
	})
}

