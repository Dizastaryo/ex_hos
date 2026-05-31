package handler

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type Response struct {
	Data  interface{} `json:"data"`
	Meta  interface{} `json:"meta,omitempty"`
	Error interface{} `json:"error"`
}

func respondSuccess(c *fiber.Ctx, status int, data interface{}, meta interface{}) error {
	return c.Status(status).JSON(Response{
		Data:  data,
		Meta:  meta,
		Error: nil,
	})
}

func respondError(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(Response{
		Data:  nil,
		Error: message,
	})
}

func respondValidationError(c *fiber.Ctx, err error) error {
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return respondError(c, fiber.StatusBadRequest, "validation error")
	}

	fields := make(map[string]string)
	for _, e := range validationErrors {
		fields[e.Field()] = e.Tag()
	}

	return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
		"data":  nil,
		"error": "validation failed",
		"fields": fields,
	})
}
