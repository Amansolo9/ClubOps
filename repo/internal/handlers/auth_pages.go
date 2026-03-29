package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) dashboard(c *fiber.Ctx) error {
	return h.render(c, "index", fiber.Map{"User": currentUser(c)}, "layouts/main")
}

func (h *Handler) publicRecruiting(c *fiber.Ctx) error {
	search := c.Query("q")
	clubs, err := h.store.ListRecruitingClubs(search)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	return h.render(c, "recruiting", fiber.Map{"Clubs": buildClubViews(clubs), "User": currentUser(c), "Query": search}, "layouts/main")
}

func (h *Handler) loginPage(c *fiber.Ctx) error {
	return h.render(c, "login", fiber.Map{}, "layouts/main")
}

func (h *Handler) changePasswordPage(c *fiber.Ctx) error {
	return h.render(c, "change_password", fiber.Map{"User": currentUser(c)}, "layouts/main")
}

func (h *Handler) loginAction(c *fiber.Ctx) error {
	token, user, err := h.auth.Login(c.FormValue("username"), c.FormValue("password"))
	if err != nil {
		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusUnauthorized).SendString(err.Error())
		}
		return h.render(c.Status(fiber.StatusUnauthorized), "login", fiber.Map{"Error": err.Error()}, "layouts/main")
	}
	c.Cookie(&fiber.Cookie{Name: "session_token", Value: token, HTTPOnly: true, Secure: c.Protocol() == "https", Path: "/", SameSite: "Lax"})
	redirectPath := "/"
	if user != nil && user.MustChangePass {
		redirectPath = "/change-password"
	}
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", redirectPath)
		return c.SendStatus(fiber.StatusNoContent)
	}
	return c.Redirect(redirectPath)
}

func (h *Handler) register(c *fiber.Ctx) error {
	if err := h.auth.Register(c.FormValue("username"), c.FormValue("password"), "member", nil); err != nil {
		return h.writeServiceError(c, err)
	}
	return c.SendString("registered")
}

func (h *Handler) logout(c *fiber.Ctx) error {
	token := c.Cookies("session_token")
	_ = h.auth.Logout(token)
	c.ClearCookie("session_token")
	return c.Redirect("/login")
}

func (h *Handler) changePassword(c *fiber.Ctx) error {
	user := currentUser(c)
	if err := h.auth.ChangePassword(user.ID, c.FormValue("new_password")); err != nil {
		return h.writeServiceError(c, err)
	}
	c.Set("HX-Redirect", "/")
	return c.SendString("password changed")
}

func (h *Handler) adminResetPassword(c *fiber.Ctx) error {
	target, err := strconv.ParseInt(c.FormValue("user_id"), 10, 64)
	if err != nil {
		return c.Status(400).SendString("invalid user_id")
	}
	if err := h.auth.AdminResetPassword(target, c.FormValue("temp_password")); err != nil {
		return h.writeServiceError(c, err)
	}
	return c.SendString("password reset")
}
