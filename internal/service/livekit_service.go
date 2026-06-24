package service

import (
	"errors"
	"time"

	"github.com/livekit/protocol/auth"
)

// ErrLiveKitNotConfigured is returned when LiveKit credentials are missing —
// the live-stream feature is effectively disabled until they're set.
var ErrLiveKitNotConfigured = errors.New("livekit is not configured")

// LiveKitService issues short-lived access tokens that let a client join a
// LiveKit room. The backend never touches media — it only signs grants:
//   - broadcaster → canPublish=true  (publishes camera+mic)
//   - viewer      → canPublish=false (subscribe-only)
//
// The room name is the live-stream id, so each stream maps to exactly one room.
type LiveKitService struct {
	url       string
	apiKey    string
	apiSecret string
}

func NewLiveKitService(url, apiKey, apiSecret string) *LiveKitService {
	return &LiveKitService{url: url, apiKey: apiKey, apiSecret: apiSecret}
}

// Configured reports whether tokens can be issued.
func (s *LiveKitService) Configured() bool {
	return s.url != "" && s.apiKey != "" && s.apiSecret != ""
}

// URL is the ws[s]:// address clients connect to.
func (s *LiveKitService) URL() string { return s.url }

// Token mints a JWT for `identity` to join `room`. `name` is the display name
// shown to other participants. `canPublish` gates whether the client may push
// its own camera/mic (true for the broadcaster, false for viewers).
func (s *LiveKitService) Token(room, identity, name string, canPublish bool) (string, error) {
	if !s.Configured() {
		return "", ErrLiveKitNotConfigured
	}

	canSubscribe := true
	grant := &auth.VideoGrant{
		RoomJoin:     true,
		Room:         room,
		CanPublish:   &canPublish,
		CanSubscribe: &canSubscribe,
	}

	at := auth.NewAccessToken(s.apiKey, s.apiSecret)
	at.AddGrant(grant).
		SetIdentity(identity).
		SetName(name).
		SetValidFor(2 * time.Hour)

	return at.ToJWT()
}
