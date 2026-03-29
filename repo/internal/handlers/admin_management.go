package handlers

import (
	"encoding/json"
	"strconv"
	"strings"

	"clubops_portal/internal/models"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) updateClubProfile(c *fiber.Ctx) error {
	user := currentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid club id")
	}
	if user.Role != "admin" && (user.ClubID == nil || *user.ClubID != id) {
		return apiError(c, fiber.StatusForbidden, "forbidden", "You are not allowed to perform this action.")
	}
	tags := splitClean(c.FormValue("tags"))
	tagsJSON, _ := json.Marshal(tags)
	avatarPath := c.FormValue("avatar_path")
	if fh, err := c.FormFile("avatar"); err == nil && fh != nil && fh.Filename != "" {
		savedPath, err := saveAvatarFile(fh)
		if err != nil {
			return h.writeServiceError(c, err)
		}
		avatarPath = savedPath
	}
	club := models.Club{ID: id, Name: c.FormValue("name"), Tags: string(tagsJSON), AvatarPath: avatarPath, RecruitmentOpen: c.FormValue("recruitment_open") == "true", Description: c.FormValue("description")}
	if club.Name == "" {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "name required")
	}
	if err := h.store.UpdateClubProfile(club); err != nil {
		return h.writeServiceError(c, err)
	}
	return c.SendString("club updated")
}

func (h *Handler) createClub(c *fiber.Ctx) error {
	name := strings.TrimSpace(c.FormValue("name"))
	if name == "" {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "name required")
	}
	tags := splitClean(c.FormValue("tags"))
	tagsJSON, _ := json.Marshal(tags)
	recruitmentOpen := c.FormValue("recruitment_open", "true") == "true"
	id, err := h.store.InsertClub(models.Club{
		Name:            name,
		Tags:            string(tagsJSON),
		AvatarPath:      "",
		RecruitmentOpen: recruitmentOpen,
		Description:     c.FormValue("description"),
	})
	if err != nil {
		return h.writeServiceError(c, err)
	}
	c.Set("HX-Trigger", `{"clubsUpdated":true}`)
	return c.SendString("club created #" + strconv.FormatInt(id, 10))
}

func (h *Handler) updateUser(c *fiber.Ctx) error {
	targetID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid user id")
	}
	role := strings.TrimSpace(c.FormValue("role"))
	allowedRoles := map[string]bool{"member": true, "team_lead": true, "organizer": true, "admin": true}
	if !allowedRoles[role] {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid role")
	}
	clubID, err := parseOptionalInt64(c.FormValue("club_id"))
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid club_id")
	}
	if role == "team_lead" && clubID == nil {
		return apiError(c, fiber.StatusUnprocessableEntity, "validation_error", "team lead requires club assignment")
	}
	if err := h.store.UpdateUserRoleAndClub(targetID, role, clubID); err != nil {
		return h.writeServiceError(c, err)
	}
	return c.SendString("user updated")
}

func (h *Handler) upsertFlag(c *fiber.Ctx) error {
	user := currentUser(c)
	key := strings.TrimSpace(c.FormValue("flag_key"))
	if key == "" {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "flag_key required")
	}
	rolloutPct, err := strconv.Atoi(c.FormValue("rollout_pct", "100"))
	if err != nil || rolloutPct < 0 || rolloutPct > 100 {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "rollout_pct must be between 0 and 100")
	}
	if err := h.store.UpsertFeatureFlag(models.FeatureFlag{FlagKey: key, Enabled: c.FormValue("enabled") == "true", TargetScope: c.FormValue("target_scope", "global"), RolloutPct: rolloutPct, UpdatedBy: user.ID}); err != nil {
		return h.writeServiceError(c, err)
	}
	return c.SendString("flag updated")
}

func (h *Handler) evaluateFlag(c *fiber.Ctx) error {
	user := currentUser(c)
	if user == nil {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Authentication required.")
	}
	key := c.Params("key")
	return c.JSON(fiber.Map{"flag": key, "enabled": h.flags.IsEnabledForUser(key, user)})
}
