package middleware

import "github.com/gofiber/fiber/v2"

func writeAPIError(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": message, "error_code": code, "message": message})
}
