package middleware

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Paths whose request bodies are sacrificed wholesale — anything in these
// requests is sensitive (OTP code, password, refresh token).
var sensitiveBodyPaths = []string{
	"/api/v1/auth/send-otp",
	"/api/v1/auth/verify-otp",
	"/api/v1/auth/refresh",
	"/api/v1/auth/logout",
	"/api/v1/me/password",
	"/api/v1/ai/", // any AI endpoint — bodies may carry user prompts/credentials
}

// Field names whose JSON values must never appear in logs. We redact at the
// raw-JSON level so unparseable / streaming / nested bodies still get scrubbed.
// The regex deliberately matches both top-level and nested occurrences.
var sensitiveKeyRe = regexp.MustCompile(
	`"(access_token|refresh_token|password|new_password|old_password|otp|code|api_key|apiKey|secret|authorization|cookie|set-cookie|token)"\s*:\s*"[^"]*"`)

// redactedJSON returns a copy of b with sensitive JSON values replaced by
// "[redacted]". Non-JSON bodies pass through unchanged.
func redactedJSON(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	return sensitiveKeyRe.ReplaceAll(b, []byte(`"$1":"[redacted]"`))
}

func isSensitivePath(p string) bool {
	for _, prefix := range sensitiveBodyPaths {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

var (
	netLogMu   sync.Mutex
	netLogFile *os.File
)

func openNetLog() *os.File {
	netLogMu.Lock()
	defer netLogMu.Unlock()
	if netLogFile != nil {
		return netLogFile
	}
	dir := "logs"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	path := filepath.Join(dir, "requests.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil
	}
	netLogFile = f
	return netLogFile
}

func writeNetLog(entry map[string]any) {
	f := openNetLog()
	if f == nil {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	netLogMu.Lock()
	defer netLogMu.Unlock()
	f.Write(data)
	f.Write([]byte("\n"))
}

const maxLogBody = 4 * 1024

func snippet(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if len(b) > maxLogBody {
		b = b[:maxLogBody]
	}
	return string(b)
}

func Logger(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		requestID := uuid.New().String()
		c.Set("X-Request-ID", requestID)
		c.Locals("request_id", requestID)

		start := time.Now()
		reqBody := append([]byte(nil), c.Body()...)

		err := c.Next()

		duration := time.Since(start)
		status := c.Response().StatusCode()

		fields := []zap.Field{
			zap.String("request_id", requestID),
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.Int("status", status),
			zap.Duration("duration", duration),
			zap.String("ip", c.IP()),
			zap.String("user_agent", c.Get("User-Agent")),
		}

		if userID := GetUserID(c); userID != "" {
			fields = append(fields, zap.String("user_id", userID))
		}

		if err != nil {
			fields = append(fields, zap.Error(err))
		}

		if status >= 500 {
			logger.Error("request completed", fields...)
		} else if status >= 400 {
			logger.Warn("request completed", fields...)
		} else {
			logger.Info("request completed", fields...)
		}

		// Persist request/response to JSONL for offline inspection.
		ct := strings.ToLower(c.Get("Content-Type"))
		respCT := strings.ToLower(string(c.Response().Header.ContentType()))
		entry := map[string]any{
			"ts":         start.UTC().Format(time.RFC3339Nano),
			"request_id": requestID,
			"method":     c.Method(),
			"path":       c.Path(),
			"query":      c.Context().QueryArgs().String(),
			"status":     status,
			"duration_ms": duration.Milliseconds(),
			"ip":         c.IP(),
			"user_agent": c.Get("User-Agent"),
			"req_ct":     ct,
			"resp_ct":    respCT,
		}
		if userID := GetUserID(c); userID != "" {
			entry["user_id"] = userID
		}
		if err != nil {
			entry["error"] = err.Error()
		}
		// Only log textual bodies; skip binary uploads/downloads. For
		// auth/credential endpoints we drop the body entirely; for everything
		// else we run a JSON-aware key redaction so tokens leaked through
		// nested response payloads don't end up on disk.
		sensitive := isSensitivePath(c.Path())
		if isTextual(ct) {
			if sensitive {
				entry["req_body"] = "[redacted]"
			} else {
				entry["req_body"] = snippet(redactedJSON(reqBody))
			}
		}
		if isTextual(respCT) {
			if sensitive {
				entry["resp_body"] = "[redacted]"
			} else {
				entry["resp_body"] = snippet(redactedJSON(c.Response().Body()))
			}
		}
		writeNetLog(entry)

		return err
	}
}

func isTextual(ct string) bool {
	if ct == "" {
		return false
	}
	if strings.HasPrefix(ct, "application/json") ||
		strings.HasPrefix(ct, "application/x-www-form-urlencoded") ||
		strings.HasPrefix(ct, "text/") {
		return true
	}
	return false
}

func Recovery(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic recovered",
					zap.Any("panic", r),
					zap.String("path", c.Path()),
					zap.String("method", c.Method()),
				)
				err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"data":  nil,
					"error": "internal server error",
				})
			}
		}()

		return c.Next()
	}
}
