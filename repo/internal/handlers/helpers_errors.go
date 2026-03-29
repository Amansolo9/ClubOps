package handlers

import (
	"database/sql"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) writeServiceError(c *fiber.Ctx, err error) error {
	if err == nil {
		return c.SendStatus(fiber.StatusNoContent)
	}
	status, code, message := classifyServiceError(err)
	reqID := strconv.FormatInt(time.Now().UnixNano(), 36)
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_DEBUG_ERRORS")), "true") {
		log.Printf("service_error request_id=%s path=%s method=%s code=%s detail=%v", reqID, c.Path(), c.Method(), code, err)
	} else {
		log.Printf("service_error request_id=%s path=%s method=%s code=%s", reqID, c.Path(), c.Method(), code)
	}
	if strings.HasPrefix(c.Path(), "/api") {
		return apiError(c, status, code, message)
	}
	return c.Status(status).SendString(message)
}

func apiError(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message, "error_code": code, "message": message})
}

func classifyServiceError(err error) (int, string, string) {
	if errors.Is(err, sql.ErrNoRows) {
		return fiber.StatusNotFound, "not_found", "Resource not found."
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(msg, "forbidden") || strings.Contains(msg, "unauthorized") {
		return fiber.StatusForbidden, "forbidden", "You are not allowed to perform this action."
	}
	if strings.Contains(msg, "not found") {
		return fiber.StatusNotFound, "not_found", "Resource not found."
	}
	if strings.Contains(msg, "already") || strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") || strings.Contains(msg, "immutable") {
		return fiber.StatusConflict, "conflict", "Request conflicts with existing data."
	}
	if strings.Contains(msg, "invalid") || strings.Contains(msg, "required") || strings.Contains(msg, "must") || strings.Contains(msg, "cannot") || strings.Contains(msg, "closed") {
		return fiber.StatusUnprocessableEntity, "validation_error", err.Error()
	}
	return fiber.StatusBadRequest, "bad_request", "Request could not be processed."
}
