package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// JSONCharsetUTF8 ensures every response with Content-Type "application/json"
// gets the explicit "; charset=utf-8" suffix. Without it some HTTP clients
// (notably Dart's Dio) may fall back to latin1 decoding, turning Cyrillic
// text into replacement characters (�).
func JSONCharsetUTF8() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := c.Next(); err != nil {
			return err
		}
		ct := string(c.Response().Header.ContentType())
		if strings.HasPrefix(ct, "application/json") && !strings.Contains(ct, "charset") {
			c.Response().Header.SetContentType("application/json; charset=utf-8")
		}
		return nil
	}
}
