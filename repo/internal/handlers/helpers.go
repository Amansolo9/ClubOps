package handlers

import (
	"errors"
	"strconv"
	"strings"

	"clubops_portal/internal/models"

	"github.com/gofiber/fiber/v2"
)

func scopedClubIDForUser(user *models.User, c *fiber.Ctx) *int64 {
	if user == nil || user.Role == "admin" {
		return nil
	}
	if scope := scopedClubID(c); scope != nil {
		return scope
	}
	return user.ClubID
}

func requireManagedClub(user *models.User) error {
	if user == nil || user.Role == "admin" || user.Role == "member" {
		return nil
	}
	if user.ClubID == nil {
		return errors.New("club scope required")
	}
	return nil
}

func scopedClubID(c *fiber.Ctx) *int64 {
	v := c.Locals("scope_club_id")
	if v == nil {
		return nil
	}
	id := v.(int64)
	return &id
}

func currentUser(c *fiber.Ctx) *models.User {
	u, ok := c.Locals("user").(*models.User)
	if !ok {
		return nil
	}
	return u
}

func userClubID(user *models.User) *int64 {
	if user == nil {
		return nil
	}
	return user.ClubID
}

func parseOptionalInt64(raw string) (*int64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func parseInt64WithDefault(raw string, def int64) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return def, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}
