package middleware

import (
	"strings"
	"testing"
)

func TestRedactedJSON_RedactsKnownKeys(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"top-level access_token", `{"access_token":"secret123","user":"alice"}`},
		{"refresh_token", `{"refresh_token":"abc.def.ghi"}`},
		{"nested token", `{"data":{"access_token":"deep","ok":true}}`},
		{"password field", `{"password":"hunter2"}`},
		{"api_key snake", `{"api_key":"sk-12345"}`},
		{"apiKey camel", `{"apiKey":"sk-12345"}`},
		{"otp code", `{"phone":"+7999","code":"1234"}`},
		{"authorization header echo", `{"authorization":"Bearer xyz"}`},
		{"mixed case key still matches with fixed casing", `{"secret":"shh"}`},
	}
	for _, tc := range cases {
		got := string(redactedJSON([]byte(tc.in)))
		if strings.Contains(got, "secret123") ||
			strings.Contains(got, "abc.def.ghi") ||
			strings.Contains(got, "deep") ||
			strings.Contains(got, "hunter2") ||
			strings.Contains(got, "sk-12345") ||
			strings.Contains(got, "1234") ||
			strings.Contains(got, "Bearer xyz") ||
			strings.Contains(got, "shh") {
			t.Errorf("%s: secret leaked through redaction: %s", tc.name, got)
		}
		if !strings.Contains(got, "[redacted]") {
			t.Errorf("%s: expected [redacted] marker in output, got %s", tc.name, got)
		}
	}
}

func TestRedactedJSON_PreservesSafeFields(t *testing.T) {
	in := `{"username":"alice","posts_count":42,"is_verified":true}`
	got := string(redactedJSON([]byte(in)))
	if !strings.Contains(got, `"username":"alice"`) ||
		!strings.Contains(got, `"posts_count":42`) ||
		!strings.Contains(got, `"is_verified":true`) {
		t.Errorf("safe fields mangled: %s", got)
	}
}

func TestRedactedJSON_HandlesEmptyAndNonJSON(t *testing.T) {
	if got := redactedJSON(nil); got != nil {
		t.Errorf("nil input should pass through, got %v", got)
	}
	if got := string(redactedJSON([]byte("not json"))); got != "not json" {
		t.Errorf("non-json should pass through, got %s", got)
	}
}

func TestIsSensitivePath(t *testing.T) {
	yes := []string{
		"/api/v1/auth/send-otp",
		"/api/v1/auth/verify-otp",
		"/api/v1/auth/refresh",
		"/api/v1/me/password",
		"/api/v1/ai/generate-filter",
	}
	no := []string{
		"/api/v1/feed",
		"/api/v1/posts/abc/comments",
		"/api/v1/users/alice",
		"/health",
	}
	for _, p := range yes {
		if !isSensitivePath(p) {
			t.Errorf("%s should be sensitive", p)
		}
	}
	for _, p := range no {
		if isSensitivePath(p) {
			t.Errorf("%s should NOT be sensitive", p)
		}
	}
}
