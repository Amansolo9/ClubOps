package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const csrfCookieName = "csrf_token"

func CSRFProtection() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Cookies(csrfCookieName)
		if token == "" {
			generated, err := randomCSRFToken(32)
			if err != nil {
				if strings.HasPrefix(c.Path(), "/api") {
					return writeAPIError(c, fiber.StatusInternalServerError, "internal_error", "Request could not be processed.")
				}
				return c.Status(fiber.StatusInternalServerError).SendString("csrf init failed")
			}
			token = generated
			c.Cookie(&fiber.Cookie{Name: csrfCookieName, Value: token, HTTPOnly: false, Secure: c.Protocol() == "https", Path: "/", SameSite: "Lax"})
		}
		c.Locals("csrf_token", token)
		if isMutatingMethod(c.Method()) && c.Cookies("session_token") != "" && !isCSRFExemptPath(c.Path()) {
			requestToken := strings.TrimSpace(c.Get("X-CSRF-Token"))
			if requestToken == "" {
				requestToken = strings.TrimSpace(c.FormValue("csrf_token"))
			}
			if requestToken == "" || requestToken != token {
				if strings.HasPrefix(c.Path(), "/api") {
					return writeAPIError(c, http.StatusForbidden, "csrf_invalid", "CSRF token invalid.")
				}
				return c.Status(http.StatusForbidden).SendString("csrf token invalid")
			}
		}
		return c.Next()
	}
}

func isMutatingMethod(method string) bool {
	switch method {
	case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete:
		return true
	default:
		return false
	}
}

func isCSRFExemptPath(path string) bool {
	return path == "/login" || path == "/register"
}

func randomCSRFToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
