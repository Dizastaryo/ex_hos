package handler

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/internal/service"
	"go.uber.org/zap"
)

type CollectionHandler struct {
	repo        *postgres.CollectionRepository
	fileService *service.FileService
	validate    *validator.Validate
	logger      *zap.Logger
}

func NewCollectionHandler(repo *postgres.CollectionRepository, fileService *service.FileService, validate *validator.Validate, logger *zap.Logger) *CollectionHandler {
	return &CollectionHandler{repo: repo, fileService: fileService, validate: validate, logger: logger}
}

// GET /api/v1/collections
func (h *CollectionHandler) ListCollections(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	cols, err := h.repo.GetUserCollections(c.Context(), userID)
	if err != nil {
		h.logger.Error("list collections", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to list collections")
	}
	if cols == nil {
		cols = []*domain.Collection{}
	}
	return respondSuccess(c, fiber.StatusOK, cols, nil)
}

// POST /api/v1/collections
func (h *CollectionHandler) CreateCollection(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	var req domain.CreateCollectionRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}
	col := &domain.Collection{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		CoverFileID: req.CoverFileID,
	}
	if err := h.repo.CreateCollection(c.Context(), col); err != nil {
		h.logger.Error("create collection", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to create collection")
	}
	return respondSuccess(c, fiber.StatusCreated, col, nil)
}

// GET /api/v1/collections/:id
func (h *CollectionHandler) GetCollection(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id := c.Params("id")
	col, err := h.repo.GetCollectionByID(c.Context(), id, userID)
	if err != nil {
		h.logger.Error("get collection", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get collection")
	}
	if col == nil {
		return respondError(c, fiber.StatusNotFound, "collection not found")
	}
	return respondSuccess(c, fiber.StatusOK, col, nil)
}

// PUT /api/v1/collections/:id
func (h *CollectionHandler) UpdateCollection(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id := c.Params("id")
	var req domain.UpdateCollectionRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}
	col := &domain.Collection{
		ID:          id,
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		CoverFileID: req.CoverFileID,
	}
	if err := h.repo.UpdateCollection(c.Context(), col); err != nil {
		h.logger.Error("update collection", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to update collection")
	}
	return respondSuccess(c, fiber.StatusOK, col, nil)
}

// DELETE /api/v1/collections/:id
func (h *CollectionHandler) DeleteCollection(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	id := c.Params("id")
	if err := h.repo.DeleteCollection(c.Context(), id, userID); err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to delete collection")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// POST /api/v1/collections/:id/files
func (h *CollectionHandler) AddFile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	collectionID := c.Params("id")
	var req domain.AddCollectionFileRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.validate.Struct(&req); err != nil {
		return respondValidationError(c, err)
	}
	if err := h.repo.AddFile(c.Context(), collectionID, req.FileID, userID); err != nil {
		h.logger.Error("add file to collection", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to add file")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// DELETE /api/v1/collections/:id/files/:fileId
func (h *CollectionHandler) RemoveFile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	collectionID := c.Params("id")
	fileID := c.Params("fileId")
	if err := h.repo.RemoveFile(c.Context(), collectionID, fileID, userID); err != nil {
		return respondError(c, fiber.StatusInternalServerError, "failed to remove file")
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// GET /api/v1/files/:id/stats  (owner only)
func (h *CollectionHandler) GetFileStats(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	fileID := c.Params("id")

	// Check ownership via fileService
	file, err := h.fileService.GetFile(c.Context(), fileID)
	if err != nil {
		if err == domain.ErrFileNotFound {
			return respondError(c, fiber.StatusNotFound, "file not found")
		}
		return respondError(c, fiber.StatusInternalServerError, "failed to get file")
	}
	if file.UserID != userID {
		return respondError(c, fiber.StatusForbidden, "only the owner can view stats")
	}

	stats, err := h.repo.GetFileStats(c.Context(), fileID)
	if err != nil {
		h.logger.Error("file stats", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "failed to get stats")
	}
	return respondSuccess(c, fiber.StatusOK, stats, nil)
}
