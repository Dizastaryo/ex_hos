package handler

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/internal/service"
)

// AdminHandler powers the admin web bundle (admin.seeu.kz). All routes here
// must be mounted behind middleware.Auth + middleware.AdminOnly.
type AdminHandler struct {
	pool       interface{} // unused — kept for future direct-SQL admin ops
	userRepo   *postgres.UserRepository
	reportRepo *postgres.ReportRepository
	auditRepo  *postgres.AuditRepository
	audioRepo  *postgres.AudioRepository
	userSvc    *service.UserService
	logger     *zap.Logger
}

func NewAdminHandler(
	userRepo *postgres.UserRepository,
	reportRepo *postgres.ReportRepository,
	auditRepo *postgres.AuditRepository,
	audioRepo *postgres.AudioRepository,
	userSvc *service.UserService,
	logger *zap.Logger,
) *AdminHandler {
	return &AdminHandler{
		userRepo:   userRepo,
		reportRepo: reportRepo,
		auditRepo:  auditRepo,
		audioRepo:  audioRepo,
		userSvc:    userSvc,
		logger:     logger,
	}
}

// audit writes one row asynchronously-style: errors are logged but do NOT
// abort the user-facing response. Audit must never block ban/unban.
func (h *AdminHandler) audit(
	c *fiber.Ctx,
	action, targetType, targetID string,
	metadata map[string]any,
) {
	adminID, _ := c.Locals("user_id").(string)
	if err := h.auditRepo.Log(c.Context(), adminID, action, targetType, targetID, metadata); err != nil {
		h.logger.Warn("audit log failed",
			zap.String("action", action),
			zap.String("target_id", targetID),
			zap.Error(err))
	}
}

// ListReports godoc
// GET /api/v1/admin/reports?status=pending&limit=50
func (h *AdminHandler) ListReports(c *fiber.Ctx) error {
	status := c.Query("status", "pending")
	limit := c.QueryInt("limit", 50)
	if limit > 200 {
		limit = 200
	}
	offset := c.QueryInt("offset", 0)

	reports, err := h.reportRepo.AdminList(c.Context(), status, limit, offset)
	if err != nil {
		h.logger.Error("admin list reports", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list reports")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": reports}, nil)
}

// UpdateReportStatus godoc
// POST /api/v1/admin/reports/:id/dismiss
// POST /api/v1/admin/reports/:id/actioned
func (h *AdminHandler) DismissReport(c *fiber.Ctx) error {
	return h.setReportStatus(c, "dismissed")
}

func (h *AdminHandler) ActionReport(c *fiber.Ctx) error {
	return h.setReportStatus(c, "actioned")
}

func (h *AdminHandler) setReportStatus(c *fiber.Ctx, status string) error {
	id := c.Params("id")
	reviewerID, _ := c.Locals("user_id").(string)
	if err := h.reportRepo.UpdateStatus(c.Context(), id, status, reviewerID); err != nil {
		h.logger.Error("admin update report", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to update report")
	}
	h.audit(c, "report."+status, "report", id, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

// ListUsers godoc
// GET /api/v1/admin/users?q=...&banned=true&limit=50
func (h *AdminHandler) ListUsers(c *fiber.Ctx) error {
	users, err := h.userRepo.AdminListUsers(c.Context(), postgres.AdminListUsersFilter{
		Query:      c.Query("q"),
		OnlyBanned: c.QueryBool("banned"),
		OnlyAdmins: c.QueryBool("admins"),
		Limit:      c.QueryInt("limit", 50),
		Offset:     c.QueryInt("offset", 0),
	})
	if err != nil {
		h.logger.Error("admin list users", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list users")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": users}, nil)
}

type banRequest struct {
	Reason string `json:"reason"`
}

func (h *AdminHandler) BanUser(c *fiber.Ctx) error {
	var req banRequest
	_ = c.BodyParser(&req) // body is optional
	id := c.Params("id")
	if err := h.userRepo.SetBanned(c.Context(), id, true, req.Reason); err != nil {
		h.logger.Error("ban user", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to ban")
	}
	meta := map[string]any{}
	if req.Reason != "" {
		meta["reason"] = req.Reason
	}
	h.audit(c, "user.ban", "user", id, meta)
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *AdminHandler) UnbanUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.userRepo.SetBanned(c.Context(), id, false, ""); err != nil {
		h.logger.Error("unban user", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to unban")
	}
	h.audit(c, "user.unban", "user", id, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

// VerifyUser — PROFILE-5. Помечает юзера как verified (голубая галочка).
// POST /api/v1/admin/users/:id/verify
func (h *AdminHandler) VerifyUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.userRepo.SetVerified(c.Context(), id, true); err != nil {
		h.logger.Error("verify user", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to verify")
	}
	h.audit(c, "user.verify", "user", id, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

// UnverifyUser — PROFILE-5. Снимает verified.
// POST /api/v1/admin/users/:id/unverify
func (h *AdminHandler) UnverifyUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.userRepo.SetVerified(c.Context(), id, false); err != nil {
		h.logger.Error("unverify user", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to unverify")
	}
	h.audit(c, "user.unverify", "user", id, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

// DeleteUser godoc
// DELETE /api/v1/admin/users/:id
// Hard-delete a user. Same cascading semantics as self-delete but invoked by
// an admin (e.g. confirmed CSAM, repeated violations).
func (h *AdminHandler) DeleteUser(c *fiber.Ctx) error {
	id := c.Params("id")
	// Snapshot username before delete so the audit row stays meaningful
	// even after the user row is gone.
	var username string
	if u, err := h.userRepo.GetByID(c.Context(), id); err == nil && u != nil {
		username = u.Username
	}
	if err := h.userSvc.DeleteAccount(c.Context(), id); err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		h.logger.Error("admin delete user", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to delete user")
	}
	meta := map[string]any{}
	if username != "" {
		meta["username"] = username
	}
	h.audit(c, "user.delete", "user", id, meta)
	return c.SendStatus(fiber.StatusNoContent)
}

// Metrics godoc
// GET /api/v1/admin/metrics
func (h *AdminHandler) Metrics(c *fiber.Ctx) error {
	m, err := h.reportRepo.AdminMetrics(c.Context())
	if err != nil {
		h.logger.Error("admin metrics", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to load metrics")
	}
	return respondSuccess(c, fiber.StatusOK, m, nil)
}

// MetricsTimeSeries godoc
// GET /api/v1/admin/metrics/timeseries?days=30
// Returns daily DAU / signups / new-posts buckets. Used by the dashboard chart.
func (h *AdminHandler) MetricsTimeSeries(c *fiber.Ctx) error {
	days := c.QueryInt("days", 30)
	rows, err := h.reportRepo.AdminTimeSeries(c.Context(), days)
	if err != nil {
		h.logger.Error("admin metrics timeseries", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to load timeseries")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": rows}, nil)
}

// ── Audio moderation ──────────────────────────────────────────────────────

// GET /api/v1/admin/audio-tracks?status=pending&limit=50
func (h *AdminHandler) ListAudioTracks(c *fiber.Ctx) error {
	tracks, err := h.audioRepo.AdminList(c.Context(),
		c.Query("status", "pending"),
		c.QueryInt("limit", 50),
		c.QueryInt("offset", 0),
	)
	if err != nil {
		h.logger.Error("admin list audio", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list audio")
	}
	if tracks == nil {
		tracks = nil
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": tracks}, nil)
}

// POST /api/v1/admin/audio-tracks/:id/approve
func (h *AdminHandler) ApproveAudioTrack(c *fiber.Ctx) error {
	id := c.Params("id")
	reviewerID, _ := c.Locals("user_id").(string)
	if err := h.audioRepo.SetStatus(c.Context(), id, "approved", "", reviewerID); err != nil {
		if err == domain.ErrNotFound {
			return respondError(c, fiber.StatusNotFound, "track not found")
		}
		h.logger.Error("approve audio track", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to approve")
	}
	h.audit(c, "audio.approve", "audio_track", id, nil)
	return c.SendStatus(fiber.StatusNoContent)
}

// POST /api/v1/admin/audio-tracks/:id/reject  body: {reason}
func (h *AdminHandler) RejectAudioTrack(c *fiber.Ctx) error {
	id := c.Params("id")
	reviewerID, _ := c.Locals("user_id").(string)
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.BodyParser(&req)
	if err := h.audioRepo.SetStatus(c.Context(), id, "rejected", req.Reason, reviewerID); err != nil {
		if err == domain.ErrNotFound {
			return respondError(c, fiber.StatusNotFound, "track not found")
		}
		h.logger.Error("reject audio track", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to reject")
	}
	meta := map[string]any{}
	if req.Reason != "" {
		meta["reason"] = req.Reason
	}
	h.audit(c, "audio.reject", "audio_track", id, meta)
	return c.SendStatus(fiber.StatusNoContent)
}

// AuditLog godoc
// GET /api/v1/admin/audit-log?admin_id=...&action=...&target_type=...&limit=50&offset=0
func (h *AdminHandler) AuditLog(c *fiber.Ctx) error {
	rows, err := h.auditRepo.List(c.Context(), postgres.AuditListFilter{
		AdminID:    c.Query("admin_id"),
		Action:     c.Query("action"),
		TargetType: c.Query("target_type"),
		Limit:      c.QueryInt("limit", 50),
		Offset:     c.QueryInt("offset", 0),
	})
	if err != nil {
		h.logger.Error("admin audit list", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list audit log")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": rows}, nil)
}
