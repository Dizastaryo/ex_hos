package handler

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/service"
	"github.com/seeu/backend/pkg/pagination"
	"go.uber.org/zap"
)

type RoomHandler struct {
	svc      *service.RoomService
	validate *validator.Validate
	logger   *zap.Logger
}

func NewRoomHandler(svc *service.RoomService, validate *validator.Validate, logger *zap.Logger) *RoomHandler {
	return &RoomHandler{svc: svc, validate: validate, logger: logger}
}

// GET /api/v1/rooms
func (h *RoomHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "30"))
	items, meta, err := h.svc.List(c.Context(), userID, page, limit)
	if err != nil {
		h.logger.Error("list rooms", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list rooms")
	}
	return respondSuccess(c, fiber.StatusOK, items, meta)
}

// POST /api/v1/rooms
func (h *RoomHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var req domain.CreateRoomRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}
	room, err := h.svc.Create(c.Context(), userID, &req)
	if err != nil {
		h.logger.Error("create room", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create room")
	}
	return respondSuccess(c, fiber.StatusCreated, room, nil)
}

// GET /api/v1/rooms/:id
func (h *RoomHandler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	room, err := h.svc.GetByID(c.Context(), id, userID)
	if err != nil {
		switch err {
		case domain.ErrRoomNotFound:
			return respondError(c, fiber.StatusNotFound, "room not found")
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "room is private")
		}
		h.logger.Error("get room", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get room")
	}
	return respondSuccess(c, fiber.StatusOK, room, nil)
}

// POST /api/v1/rooms/:id/join
func (h *RoomHandler) Join(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	room, err := h.svc.Join(c.Context(), id, userID)
	if err != nil {
		switch err {
		case domain.ErrRoomNotFound:
			return respondError(c, fiber.StatusNotFound, "room not found")
		case domain.ErrRoomClosed:
			return respondError(c, fiber.StatusGone, "room is closed")
		case domain.ErrPrivateRoom, domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "room is private — you must be invited")
		}
		h.logger.Error("join room", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to join room")
	}
	return respondSuccess(c, fiber.StatusOK, room, nil)
}

// GET /api/v1/rooms/:id/members
func (h *RoomHandler) GetMembers(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	members, err := h.svc.GetMembers(c.Context(), id, userID)
	if err != nil {
		switch err {
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "access denied")
		case domain.ErrRoomNotFound:
			return respondError(c, fiber.StatusNotFound, "room not found")
		}
		h.logger.Error("get room members", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get members")
	}
	return respondSuccess(c, fiber.StatusOK, members, nil)
}

// POST /api/v1/rooms/:id/invite
func (h *RoomHandler) InviteMember(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	var req domain.InviteMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}
	if err := h.svc.InviteMember(c.Context(), id, userID, req.UserID); err != nil {
		switch err {
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "only the creator can invite members")
		case domain.ErrRoomNotFound:
			return respondError(c, fiber.StatusNotFound, "room not found")
		}
		h.logger.Error("invite room member", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to invite member")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "invited"}, nil)
}

// DELETE /api/v1/rooms/:id/members/:userId
func (h *RoomHandler) RemoveMember(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	targetID := c.Params("userId")
	if err := h.svc.RemoveMember(c.Context(), id, userID, targetID); err != nil {
		switch err {
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "only the creator can remove members")
		case domain.ErrRoomNotFound:
			return respondError(c, fiber.StatusNotFound, "room not found")
		}
		h.logger.Error("remove room member", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to remove member")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "member removed"}, nil)
}

// DELETE /api/v1/rooms/:id/join
func (h *RoomHandler) Leave(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	if err := h.svc.Leave(c.Context(), id, userID); err != nil {
		switch err {
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "creator cannot leave — close the room instead")
		case domain.ErrRoomNotFound:
			return respondError(c, fiber.StatusNotFound, "room not found")
		}
		h.logger.Error("leave room", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to leave room")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "left room"}, nil)
}

// POST /api/v1/rooms/:id/voice
func (h *RoomHandler) JoinVoice(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	if err := h.svc.JoinVoice(c.Context(), id, userID); err != nil {
		switch err {
		case domain.ErrNotInRoom:
			return respondError(c, fiber.StatusForbidden, "you must be a room member to join voice")
		case domain.ErrRoomNotFound:
			return respondError(c, fiber.StatusNotFound, "room not found")
		}
		h.logger.Error("join voice", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to join voice")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "joined voice"}, nil)
}

// DELETE /api/v1/rooms/:id/voice
func (h *RoomHandler) LeaveVoice(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	if err := h.svc.LeaveVoice(c.Context(), id, userID); err != nil {
		if err == domain.ErrNotInVoice {
			return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "not in voice"}, nil)
		}
		h.logger.Error("leave voice", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to leave voice")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "left voice"}, nil)
}

// PATCH /api/v1/rooms/:id/mute
func (h *RoomHandler) ToggleMute(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	muted, err := h.svc.ToggleMute(c.Context(), id, userID)
	if err != nil {
		if err == domain.ErrNotInRoom {
			return respondError(c, fiber.StatusForbidden, "not in room")
		}
		h.logger.Error("toggle mute", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to toggle mute")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"is_muted": muted}, nil)
}

// PATCH /api/v1/rooms/:id
func (h *RoomHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	var req domain.UpdateRoomRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}
	room, err := h.svc.Update(c.Context(), id, userID, &req)
	if err != nil {
		switch err {
		case domain.ErrRoomNotFound:
			return respondError(c, fiber.StatusNotFound, "room not found")
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "only admins can edit this room")
		}
		h.logger.Error("update room", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to update room")
	}
	return respondSuccess(c, fiber.StatusOK, room, nil)
}

// POST /api/v1/rooms/:id/admins/:userId
func (h *RoomHandler) GrantAdmin(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	targetID := c.Params("userId")
	if err := h.svc.SetAdmin(c.Context(), id, userID, targetID, true); err != nil {
		switch err {
		case domain.ErrRoomNotFound:
			return respondError(c, fiber.StatusNotFound, "room not found")
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "only the creator can grant admin")
		case domain.ErrNotInRoom:
			return respondError(c, fiber.StatusBadRequest, "user is not a member")
		}
		h.logger.Error("grant admin", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to grant admin")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "admin granted"}, nil)
}

// DELETE /api/v1/rooms/:id/admins/:userId
func (h *RoomHandler) RevokeAdmin(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	targetID := c.Params("userId")
	if err := h.svc.SetAdmin(c.Context(), id, userID, targetID, false); err != nil {
		switch err {
		case domain.ErrRoomNotFound:
			return respondError(c, fiber.StatusNotFound, "room not found")
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "only the creator can revoke admin")
		case domain.ErrNotInRoom:
			return respondError(c, fiber.StatusBadRequest, "user is not a member")
		}
		h.logger.Error("revoke admin", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to revoke admin")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "admin revoked"}, nil)
}

// DELETE /api/v1/rooms/:id
func (h *RoomHandler) Close(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	if err := h.svc.Close(c.Context(), id, userID); err != nil {
		switch err {
		case domain.ErrRoomNotFound:
			return respondError(c, fiber.StatusNotFound, "room not found")
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "only creator can close room")
		}
		h.logger.Error("close room", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to close room")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "room closed"}, nil)
}

// GET /api/v1/rooms/:id/messages
func (h *RoomHandler) GetMessages(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "50"))
	items, meta, err := h.svc.GetMessages(c.Context(), id, userID, page, limit)
	if err != nil {
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "access denied")
		}
		h.logger.Error("get room messages", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get messages")
	}
	return respondSuccess(c, fiber.StatusOK, items, meta)
}

// POST /api/v1/rooms/:id/messages
func (h *RoomHandler) SendMessage(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.GetUserID(c)
	var req struct {
		Text string `json:"text"`
	}
	if err := c.BodyParser(&req); err != nil || req.Text == "" {
		return respondError(c, fiber.StatusBadRequest, "text is required")
	}
	msg, err := h.svc.SendMessage(c.Context(), id, userID, req.Text)
	if err != nil {
		switch err {
		case domain.ErrRoomNotFound:
			return respondError(c, fiber.StatusNotFound, "room not found")
		case domain.ErrRoomClosed:
			return respondError(c, fiber.StatusGone, "room is closed")
		case domain.ErrForbidden:
			return respondError(c, fiber.StatusForbidden, "you are not a member of this room")
		}
		h.logger.Error("send room message", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to send message")
	}
	return respondSuccess(c, fiber.StatusCreated, msg, nil)
}
