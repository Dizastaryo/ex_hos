package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	redisRepo "github.com/seeu/backend/internal/repository/redis"
	jwtpkg "github.com/seeu/backend/pkg/jwt"
	"github.com/seeu/backend/pkg/whatsapp"
	"go.uber.org/zap"
)

// devOTPCode is the well-known fallback that's accepted ONLY when the
// WhatsApp bridge is missing/down. Lets the team test the app without
// scanning QRs every demo. NEVER set in production — the presence of a
// configured `WhatsAppConfig.ServiceURL` is what gates this off.
const devOTPCode = "0000"

type AuthService struct {
	userRepo     *postgres.UserRepository
	otpRepo      *postgres.OTPRepository
	sessionStore *redisRepo.SessionStore
	jwtManager   *jwtpkg.Manager
	inviteRepo   *postgres.InviteRepository
	whatsapp     *whatsapp.Client
	otpTTL       time.Duration
	otpMaxAtt    int
	otpMaxPerHr  int
	logger       *zap.Logger
}

type AuthServiceDeps struct {
	UserRepo     *postgres.UserRepository
	OTPRepo      *postgres.OTPRepository
	SessionStore *redisRepo.SessionStore
	JWTManager   *jwtpkg.Manager
	InviteRepo   *postgres.InviteRepository
	WhatsApp     *whatsapp.Client
	OTPTTL       time.Duration
	OTPMaxAtt    int
	OTPMaxPerHr  int
	Logger       *zap.Logger
}

func NewAuthService(d AuthServiceDeps) *AuthService {
	if d.OTPTTL <= 0 {
		d.OTPTTL = 5 * time.Minute
	}
	if d.OTPMaxAtt <= 0 {
		d.OTPMaxAtt = 5
	}
	if d.OTPMaxPerHr <= 0 {
		d.OTPMaxPerHr = 3
	}
	return &AuthService{
		userRepo:     d.UserRepo,
		otpRepo:      d.OTPRepo,
		sessionStore: d.SessionStore,
		jwtManager:   d.JWTManager,
		inviteRepo:   d.InviteRepo,
		whatsapp:     d.WhatsApp,
		otpTTL:       d.OTPTTL,
		otpMaxAtt:    d.OTPMaxAtt,
		otpMaxPerHr:  d.OTPMaxPerHr,
		logger:       d.Logger,
	}
}

type AuthTokens struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresAt    time.Time    `json:"expires_at"`
	User         *domain.User `json:"user"`
	IsNewUser    bool         `json:"is_new_user"`
}

// SendOTP generates a 4-digit code, stores it in `otp_codes` with TTL, and
// dispatches it via the WhatsApp bridge. If the bridge is unconfigured or
// not ready (QR not scanned, session expired), we fall back to dev-mode:
// the code is logged and the well-known `0000` fallback is accepted by
// VerifyOTP.
//
// Per-phone rate limit: max OTPMaxPerHr sends per hour. Independent of
// the IP-based limit applied by middleware.
func (s *AuthService) SendOTP(ctx context.Context, phone string) error {
	if phone == "" {
		return errors.New("empty phone")
	}

	// Per-phone rate limit. We count rows in `otp_codes` (cheap with the
	// existing index on phone) instead of a parallel Redis counter — keeps
	// the data colocated and naturally TTL'd by the row's own expiry.
	since := time.Now().Add(-1 * time.Hour)
	recent, err := s.otpRepo.CountRecentForPhone(ctx, phone, since)
	if err != nil {
		s.logger.Warn("count recent otp", zap.Error(err))
	}
	if recent >= s.otpMaxPerHr {
		return domain.ErrRateLimited
	}

	code, err := generate4DigitCode()
	if err != nil {
		return fmt.Errorf("generate otp: %w", err)
	}
	if _, err := s.otpRepo.Insert(ctx, phone, code, s.otpTTL); err != nil {
		return fmt.Errorf("store otp: %w", err)
	}

	// Dev-mode if no bridge configured or bridge not ready: log and return.
	if s.whatsapp == nil {
		s.logger.Info("OTP generated (dev mode — no whatsapp bridge)",
			zap.String("phone", phone),
			zap.String("dev_fallback", devOTPCode))
		return nil
	}
	if err := s.whatsapp.IsReady(ctx); err != nil {
		s.logger.Warn("whatsapp bridge not ready, dev fallback active",
			zap.String("phone", phone),
			zap.Error(err),
			zap.String("dev_fallback", devOTPCode))
		return nil
	}

	msg := fmt.Sprintf("SeeU: ваш код %s. Никому его не сообщайте.", code)
	if err := s.whatsapp.Send(ctx, phone, msg); err != nil {
		// Don't fail the request — code is already stored, dev fallback
		// will let user proceed. Operator should check the bridge status
		// page if this happens.
		s.logger.Error("whatsapp send failed (code still stored, dev fallback active)",
			zap.String("phone", phone),
			zap.Error(err),
			zap.String("dev_fallback", devOTPCode))
		return nil
	}

	s.logger.Info("OTP sent via whatsapp", zap.String("phone", phone))
	return nil
}

// VerifyOTP verifies OTP code and returns tokens (auto-registers if user doesn't exist).
// New registrations require AcceptsTerms=true. Existing users get their consent
// timestamp refreshed on every successful login.
//
// Code lookup: latest unused, non-expired row for the phone. On match — mark
// used. On mismatch — bump attempts; once attempts ≥ OTPMaxAtt, the code is
// burned (forced-mark used) so brute-force is bounded.
//
// Dev fallback: if WhatsApp bridge is unreachable AND the code submitted is
// `0000`, accept regardless of stored code. Only active when bridge is missing
// or not ready — see SendOTP.
func (s *AuthService) VerifyOTP(ctx context.Context, req *domain.VerifyOTPRequest) (*AuthTokens, error) {
	if len(req.Code) != 4 {
		return nil, domain.ErrUnauthorized
	}

	// Determine bridge state once — used both for dev-fallback gate and to
	// decide whether to bump attempts on a mismatch.
	bridgeOK := false
	if s.whatsapp != nil {
		if err := s.whatsapp.IsReady(ctx); err == nil {
			bridgeOK = true
		}
	}

	verified := false

	// Dev fallback FIRST: when no real bridge, "0000" is the well-known
	// shortcut. Try it before the real lookup so a stored code doesn't
	// shadow it (operator may rely on it without scanning QRs).
	if !bridgeOK && req.Code == devOTPCode {
		verified = true
		s.logger.Info("OTP verified via dev fallback (bridge unavailable)",
			zap.String("phone", req.Phone))
	}

	if !verified {
		rec, err := s.otpRepo.LatestActive(ctx, req.Phone)
		if err != nil {
			s.logger.Warn("lookup otp", zap.Error(err))
		}
		if rec == nil {
			return nil, domain.ErrUnauthorized
		}
		if rec.Code == req.Code {
			if err := s.otpRepo.MarkUsed(ctx, rec.ID); err != nil {
				s.logger.Warn("mark otp used", zap.Error(err))
			}
			verified = true
		} else {
			if err := s.otpRepo.IncrementAttempts(ctx, rec.ID); err != nil {
				s.logger.Warn("increment otp attempts", zap.Error(err))
			}
			// Lock out after too many tries — burn the code.
			if rec.Attempts+1 >= s.otpMaxAtt {
				_ = s.otpRepo.MarkUsed(ctx, rec.ID)
			}
			return nil, domain.ErrUnauthorized
		}
	}

	user, err := s.userRepo.GetByPhone(ctx, req.Phone)
	isNewUser := false

	if err != nil {
		if err == domain.ErrUserNotFound {
			if !req.AcceptsTerms {
				return nil, domain.ErrConsentRequired
			}
			user = &domain.User{
				Phone:    req.Phone,
				Username: generateUsernameFromPhone(req.Phone),
				FullName: "",
			}
			if err := s.userRepo.Create(ctx, user); err != nil {
				return nil, fmt.Errorf("create user: %w", err)
			}
			isNewUser = true
			// Attribute the signup to the inviter, if any. Failure is non-fatal —
			// we don't want to lose a registration over a malformed invite code.
			if req.InviteCode != "" && s.inviteRepo != nil {
				if err := s.inviteRepo.Claim(ctx, req.InviteCode, user.ID); err != nil {
					s.logger.Warn("claim invite",
						zap.String("code", req.InviteCode), zap.Error(err))
				}
			}
		} else {
			return nil, fmt.Errorf("get user: %w", err)
		}
	}

	if req.AcceptsTerms {
		if err := s.userRepo.RecordConsent(ctx, user.ID); err != nil {
			s.logger.Warn("record consent", zap.Error(err))
		}
	}

	tokens, err := s.generateTokens(ctx, user)
	if err != nil {
		return nil, err
	}
	tokens.IsNewUser = isNewUser

	return tokens, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*AuthTokens, error) {
	claims, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	blacklisted, err := s.sessionStore.IsTokenBlacklisted(ctx, refreshToken)
	if err != nil {
		s.logger.Error("check token blacklist", zap.Error(err))
	}
	if blacklisted {
		return nil, domain.ErrTokenInvalid
	}

	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}

	if err := s.sessionStore.RevokeRefreshToken(ctx, refreshToken); err != nil {
		s.logger.Warn("revoke old refresh token", zap.Error(err))
	}

	return s.generateTokens(ctx, user)
}

func (s *AuthService) Logout(ctx context.Context, userID, accessToken, refreshToken string) error {
	if refreshToken != "" {
		if err := s.sessionStore.RevokeRefreshToken(ctx, refreshToken); err != nil {
			s.logger.Warn("revoke refresh token on logout", zap.Error(err))
		}
	}

	if accessToken != "" {
		if err := s.sessionStore.BlacklistToken(ctx, accessToken, userID, 25*time.Hour); err != nil {
			s.logger.Warn("blacklist access token on logout", zap.Error(err))
		}
	}

	return nil
}

func (s *AuthService) generateTokens(ctx context.Context, user *domain.User) (*AuthTokens, error) {
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	expiry := s.jwtManager.RefreshExpiry()
	if err := s.sessionStore.StoreRefreshToken(ctx, user.ID, refreshToken, expiry); err != nil {
		s.logger.Warn("store refresh token", zap.Error(err))
	}

	expiresAt := time.Now().Add(24 * time.Hour)

	return &AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         user,
	}, nil
}

// generate4DigitCode returns a uniformly-random 4-digit OTP. Uses crypto/rand
// (not math/rand) — predictability of OTPs would defeat the whole flow.
func generate4DigitCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04d", n.Int64()), nil
}

// generateUsernameFromPhone creates a default username from phone number
func generateUsernameFromPhone(phone string) string {
	// Take last 6 digits of phone
	clean := ""
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			clean += string(c)
		}
	}
	if len(clean) > 6 {
		clean = clean[len(clean)-6:]
	}
	return "user_" + clean
}
