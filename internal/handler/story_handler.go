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

type StoryHandler struct {
	storyService *service.StoryService
	validate     *validator.Validate
	logger       *zap.Logger
}

func NewStoryHandler(storyService *service.StoryService, validate *validator.Validate, logger *zap.Logger) *StoryHandler {
	return &StoryHandler{
		storyService: storyService,
		validate:     validate,
		logger:       logger,
	}
}

// CreateStory godoc
// POST /api/v1/stories
func (h *StoryHandler) CreateStory(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req domain.CreateStoryRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}

	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}

	story, err := h.storyService.Create(c.Context(), userID, &req)
	if err != nil {
		h.logger.Error("create story", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create story")
	}

	return respondSuccess(c, fiber.StatusCreated, story, nil)
}

// GetStoryFeed godoc
// GET /api/v1/stories/feed
func (h *StoryHandler) GetStoryFeed(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	groups, err := h.storyService.GetFeed(c.Context(), userID)
	if err != nil {
		h.logger.Error("get story feed", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get story feed")
	}

	return respondSuccess(c, fiber.StatusOK, groups, nil)
}

// GetUserStories godoc
// GET /api/v1/stories/:username
// Query param: include_expired=true — возвращает все сторис владельца (для highlights picker).
func (h *StoryHandler) GetUserStories(c *fiber.Ctx) error {
	username := c.Params("username")
	viewerID := middleware.GetUserID(c)

	var stories interface{}
	var err error

	if c.QueryBool("include_expired", false) {
		stories, err = h.storyService.GetAllByUsername(c.Context(), username, viewerID)
	} else {
		stories, err = h.storyService.GetByUsername(c.Context(), username, viewerID)
	}

	if err != nil {
		if err == domain.ErrUserNotFound {
			return respondError(c, fiber.StatusNotFound, "user not found")
		}
		if err == domain.ErrPrivateAccount || err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "access denied")
		}
		h.logger.Error("get user stories", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get stories")
	}

	return respondSuccess(c, fiber.StatusOK, stories, nil)
}

// DeleteStory godoc
// DELETE /api/v1/stories/:id
func (h *StoryHandler) DeleteStory(c *fiber.Ctx) error {
	storyID := c.Params("id")
	userID := middleware.GetUserID(c)

	if err := h.storyService.Delete(c.Context(), storyID, userID); err != nil {
		if err == domain.ErrStoryNotFound {
			return respondError(c, fiber.StatusNotFound, "story not found")
		}
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "access denied")
		}
		h.logger.Error("delete story", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to delete story")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "story deleted"}, nil)
}

// ViewStory godoc
// POST /api/v1/stories/:id/view
func (h *StoryHandler) ViewStory(c *fiber.Ctx) error {
	storyID := c.Params("id")
	viewerID := middleware.GetUserID(c)

	if err := h.storyService.AddView(c.Context(), storyID, viewerID); err != nil {
		if err == domain.ErrStoryNotFound {
			return respondError(c, fiber.StatusNotFound, "story not found")
		}
		h.logger.Error("view story", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to record view")
	}

	return respondSuccess(c, fiber.StatusOK, fiber.Map{"message": "view recorded"}, nil)
}

// GetStoryViewers godoc
// GET /api/v1/stories/:id/viewers
func (h *StoryHandler) GetStoryViewers(c *fiber.Ctx) error {
	storyID := c.Params("id")
	userID := middleware.GetUserID(c)
	page, limit := pagination.ParsePage(c.Query("page", "1"), c.Query("limit", "20"))

	viewers, meta, err := h.storyService.GetViewers(c.Context(), storyID, userID, page, limit)
	if err != nil {
		if err == domain.ErrStoryNotFound {
			return respondError(c, fiber.StatusNotFound, "story not found")
		}
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "access denied")
		}
		h.logger.Error("get story viewers", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get viewers")
	}

	return respondSuccess(c, fiber.StatusOK, viewers, meta)
}

// React godoc
// POST /api/v1/stories/:id/react   body: {"emoji": "👍"}
func (h *StoryHandler) React(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	storyID := c.Params("id")

	var req struct {
		Emoji string `json:"emoji"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	emoji := req.Emoji
	if emoji == "" {
		return respondError(c, fiber.StatusBadRequest, "emoji is required")
	}
	if len(emoji) > 16 {
		return respondError(c, fiber.StatusBadRequest, "emoji too long")
	}

	counts, err := h.storyService.SetReaction(c.Context(), storyID, userID, emoji)
	if err != nil {
		if err == domain.ErrStoryNotFound {
			return respondError(c, fiber.StatusNotFound, "story not found")
		}
		h.logger.Error("react story", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to react")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"story_id":    storyID,
		"reactions":   counts,
		"my_reaction": emoji,
	}, nil)
}

// VotePoll godoc (STORY-3)
// POST /api/v1/stories/:id/poll-vote   body: {"option_index": 0|1}
// Записать голос viewer'а на интерактивный poll-overlay стори. Возвращает
// обновлённые счётчики + my_vote. Один голос на user per story; повторный
// POST перезаписывает previous vote.
func (h *StoryHandler) VotePoll(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	storyID := c.Params("id")

	var req struct {
		OptionIndex int `json:"option_index"`
	}
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.OptionIndex != 0 && req.OptionIndex != 1 {
		return respondError(c, fiber.StatusBadRequest, "option_index must be 0 or 1")
	}

	poll, err := h.storyService.VotePoll(c.Context(), storyID, userID, req.OptionIndex)
	if err != nil {
		if err == domain.ErrStoryNotFound {
			return respondError(c, fiber.StatusNotFound, "story not found")
		}
		if err == domain.ErrForbidden {
			return respondError(c, fiber.StatusForbidden, "cannot vote on own poll")
		}
		if err == domain.ErrInvalidInput {
			return respondError(c, fiber.StatusBadRequest, "story has no poll")
		}
		h.logger.Error("vote poll", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to vote")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"story_id": storyID,
		"poll":     poll,
	}, nil)
}

// Unreact godoc
// DELETE /api/v1/stories/:id/react
func (h *StoryHandler) Unreact(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	storyID := c.Params("id")

	counts, err := h.storyService.RemoveReaction(c.Context(), storyID, userID)
	if err != nil {
		if err == domain.ErrStoryNotFound {
			return respondError(c, fiber.StatusNotFound, "story not found")
		}
		h.logger.Error("unreact story", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to unreact")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"story_id":  storyID,
		"reactions": counts,
	}, nil)
}
