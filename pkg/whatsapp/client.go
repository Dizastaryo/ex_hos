// Package whatsapp wraps the local `whatps` Node.js service that bridges to
// the user's real WhatsApp account via whatsapp-web.js. We use it as our OTP
// delivery channel: the bridge owner scans a QR once, then api hits its
// `/api/send` endpoint with `{number, message}` and the message arrives in
// the recipient's WhatsApp like any normal text.
//
// The client is thin on purpose — `whatps` already abstracts auth, queueing,
// reconnect; we only need a typed POST + status check.
package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNotReady — bridge is up but the WhatsApp client hasn't authenticated
// yet (QR not scanned, or session expired). Caller should fall back to
// dev-mode logging or surface a setup-required error to the user.
var ErrNotReady = errors.New("whatsapp bridge not ready")

// Client is safe for concurrent use; it holds only an http.Client and a URL.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a client pointing at the `whatps` server. baseURL example:
// "http://localhost:3000". Empty baseURL → returns nil → caller treats as
// disabled.
func New(baseURL string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return nil
	}
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type statusResponse struct {
	Ready bool `json:"ready"`
}

// IsReady polls the bridge's /api/status endpoint. Returns nil iff the
// bridge is alive AND the WhatsApp client is authenticated. Used as a
// pre-flight before SendOTP so we know whether to fall back to dev-mode.
func (c *Client) IsReady(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/status", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	var body statusResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("decode status: %w", err)
	}
	if !body.Ready {
		return ErrNotReady
	}
	return nil
}

// Send delivers `message` to a WhatsApp number. `phone` may be in any of
// `+79991234567` / `79991234567` / `+7 (999) 123-45-67` — we strip every
// non-digit before posting. The bridge expects the digit-only form.
func (c *Client) Send(ctx context.Context, phone, message string) error {
	digits := stripNonDigits(phone)
	if digits == "" {
		return errors.New("empty phone")
	}
	if message == "" {
		return errors.New("empty message")
	}

	body, err := json.Marshal(map[string]string{
		"number":  digits,
		"message": message,
	})
	if err != nil {
		return fmt.Errorf("marshal send body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/api/send", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("post /api/send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return ErrNotReady
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send failed: status %d body %q",
			resp.StatusCode, string(raw))
	}
	return nil
}

// stripNonDigits collapses a phone string to its digit form. Empty in →
// empty out. Public-ish (lowercase) for testing without re-exporting.
func stripNonDigits(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			out = append(out, c)
		}
	}
	return string(out)
}
