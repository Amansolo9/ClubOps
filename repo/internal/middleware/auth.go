package middleware

import (
	"strings"

	"clubops_portal/internal/models"
	"clubops_portal/internal/services"

	"github.com/gofiber/fiber/v2"
)

func AttachCurrentUser(auth *services.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Cookies("session_token")
		if token == "" {
			return c.Next()
		}
		u, err := auth.CurrentUserByToken(token)
		if err == nil {
			c.Locals("user", u)
		}
		return c.Next()
	}
}

func RequireAuth(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		u := c.Locals("user")
		if u == nil {
			if strings.HasPrefix(c.Path(), "/api") {
				return writeAPIError(c, fiber.StatusUnauthorized, "unauthorized", "Authentication required.")
			}
			return c.Redirect("/login")
		}
		user := u.(*models.User)
		if user.MustChangePass && c.Path() != "/change-password" && !strings.HasPrefix(c.Path(), "/api/auth/") {
			if strings.HasPrefix(c.Path(), "/api") {
				return writeAPIError(c, fiber.StatusForbidden, "password_change_required", "Password change required.")
			}
			return c.Redirect("/change-password")
		}
		c.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		c.Set("Pragma", "no-cache")
		c.Set("Expires", "0")
		if len(roles) > 0 {
			allowed := false
			for _, r := range roles {
				if user.Role == r {
					allowed = true
					break
				}
			}
			if !allowed {
				if strings.HasPrefix(c.Path(), "/api") {
					return writeAPIError(c, fiber.StatusForbidden, "forbidden", "You are not allowed to perform this action.")
				}
				return c.Status(fiber.StatusForbidden).SendString("forbidden")
			}
		}
		if user.Role == "team_lead" && user.ClubID == nil {
			if strings.HasPrefix(c.Path(), "/api") {
				return writeAPIError(c, fiber.StatusForbidden, "club_scope_required", "Team lead missing club assignment.")
			}
			return c.Status(fiber.StatusForbidden).SendString("team lead missing club assignment")
		}
		if user.Role != "admin" && user.ClubID != nil {
			c.Locals("scope_club_id", *user.ClubID)
		}
		return c.Next()
	}
}
