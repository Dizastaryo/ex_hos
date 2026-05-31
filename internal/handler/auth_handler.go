package handler

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
	"go.uber.org/zap"
)

type AuthHandler struct {
	authService *service.AuthService
	validate    *validator.Validate
	logger      *zap.Logger
}

func NewAuthHandler(authService *service.AuthService, validate *validator.Validate, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		validate:    validate,
		logger:      logger,
	}
}

// SendOTP godoc
// POST /api/v1/auth/send-otp
func (h *AuthHandler) SendOTP(c *fiber.Ctx) error {
	var req domain.SendOTPRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}

	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}

	if err := h.authService.SendOTP(c.Context(), req.Phone); err != nil {
		if err == domain.ErrRateLimited {
			return respondError(c, fiber.StatusTooManyRequests,
				"слишком много запросов, попробуйте позже")
		}
		h.logger.Error("send otp", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to send OTP")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "OTP sent successfully"}, nil)
}

// VerifyOTP godoc
// POST /api/v1/auth/verify-otp
func (h *AuthHandler) VerifyOTP(c *fiber.Ctx) error {
	var req domain.VerifyOTPRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}

	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}

	tokens, err := h.authService.VerifyOTP(c.Context(), &req)
	if err != nil {
		if err == domain.ErrUnauthorized {
			return respondError(c, fiber.StatusUnauthorized, "invalid OTP code")
		}
		if err == domain.ErrConsentRequired {
			return respondError(c, fiber.StatusUnprocessableEntity, "consent_required")
		}
		h.logger.Error("verify otp", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to verify OTP")
	}

	return respondSuccess(c, fiber.StatusOK, tokens, nil)
}

// Refresh godoc
// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}

	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}

	tokens, err := h.authService.Refresh(c.Context(), req.RefreshToken)
	if err != nil {
		if err == domain.ErrTokenExpired || err == domain.ErrTokenInvalid {
			return respondError(c, fiber.StatusUnauthorized, "invalid or expired refresh token")
		}
		h.logger.Error("refresh token", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to refresh token")
	}

	return respondSuccess(c, fiber.StatusOK, tokens, nil)
}

// Logout godoc
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	c.BodyParser(&req)

	authHeader := c.Get("Authorization")
	var accessToken string
	if len(authHeader) > 7 {
		accessToken = authHeader[7:]
	}

	if err := h.authService.Logout(c.Context(), userID, accessToken, req.RefreshToken); err != nil {
		h.logger.Warn("logout error", zap.Error(err))
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "logged out successfully"}, nil)
}
