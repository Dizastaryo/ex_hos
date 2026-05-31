package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// SecurityHeaders returns a middleware that adds standard hardening headers
// to every response. Cheap insurance against common web attack classes —
// most are no-ops for our JSON API but become load-bearing for the static
// `/uploads/...` paths and `/legal/...` HTML.
//
// `enableHSTS` should be true ONLY when the service is fronted by HTTPS
// (prod / staging). Sending HSTS over plain HTTP makes browsers stick
// with HTTPS for the lifetime of the cache, which breaks local dev.
func SecurityHeaders(enableHSTS bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// X-Frame-Options: forbid every iframe embedding. We're a mobile app
		// + admin web; never legitimately rendered inside another origin.
		c.Set("X-Frame-Options", "DENY")

		// X-Content-Type-Options: stops browsers from guessing a different
		// MIME than what the server declared. Critical for /uploads where
		// a malicious upload could otherwise be rendered as a script.
		c.Set("X-Content-Type-Options", "nosniff")

		// Referrer-Policy: sane default — share origin on same-protocol
		// nav, nothing on cross-origin downgrade. Doesn't leak query
		// strings or paths to third-party trackers.
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// X-XSS-Protection=0 (modern OWASP advice). The legacy "1; mode=block"
		// has documented bypass-via-reflection issues; the right defence is
		// CSP, not the deprecated browser filter.
		c.Set("X-XSS-Protection", "0")

		// Permissions-Policy: opt out of features the API/web doesn't use.
		// Keeps the surface area for embedded-content abuse minimal even
		// if a CSP misconfiguration ever lets external HTML render.
		c.Set("Permissions-Policy",
			"camera=(), microphone=(), geolocation=(), payment=()")

		// Content-Security-Policy: API responses are JSON — CSP can be
		// strict by default. `frame-ancestors 'none'` mirrors X-Frame-Options
		// on browsers that prefer CSP. Static media (/uploads) inherit this
		// header but are usually loaded via <img>/<video> tags from our own
		// origin, which `default-src 'self'` permits.
		c.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data: blob:; "+
				"media-src 'self' blob:; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'")

		if enableHSTS {
			// 1 year + subdomains. Once a browser sees this it'll refuse
			// HTTP for the next year — ONLY safe when prod is fully HTTPS.
			c.Set("Strict-Transport-Security",
				"max-age=31536000; includeSubDomains")
		}

		return c.Next()
	}
}
