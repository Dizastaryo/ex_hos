package handler

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
)

type InviteHandler struct {
	repo   *postgres.InviteRepository
	logger *zap.Logger
}

func NewInviteHandler(repo *postgres.InviteRepository, logger *zap.Logger) *InviteHandler {
	return &InviteHandler{repo: repo, logger: logger}
}

// Create godoc
// POST /api/v1/invites
//
// Issues a fresh invite code for the authenticated user.
func (h *InviteHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	inv, err := h.repo.Create(c.Context(), userID)
	if err != nil {
		h.logger.Error("create invite", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create invite")
	}
	return respondSuccess(c, fiber.StatusCreated, inv, nil)
}

// Lookup godoc
// GET /api/v1/invites/:code
//
// Public: shown on the auth screen so the new user can see who invited them.
// Returns ErrNotFound (404) if the code does not exist; the response includes
// a `used` flag if it has already been claimed (the invitee can still proceed —
// claiming on registration is the canonical step).
func (h *InviteHandler) Lookup(c *fiber.Ctx) error {
	code := c.Params("code")
	inv, err := h.repo.LookupByCode(c.Context(), code)
	if err != nil {
		if err == domain.ErrNotFound {
			return respondError(c, fiber.StatusNotFound, "invite not found")
		}
		h.logger.Error("lookup invite", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to lookup invite")
	}
	used := inv.UsedByID != nil
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"code":    inv.Code,
		"inviter": inv.Inviter,
		"used":    used,
	}, nil)
}

// MyInvites godoc
// GET /api/v1/invites/me
func (h *InviteHandler) MyInvites(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	items, err := h.repo.ListByInviter(c.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("list invites", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list invites")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"items": items}, nil)
}
