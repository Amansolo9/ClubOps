package handlers

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"clubops_portal/fullstack/internal/models"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) createMember(c *fiber.Ctx) error {
	user := currentUser(c)
	clubID := int64(1)
	if user.Role == "admin" {
		if formClub := c.FormValue("club_id"); formClub != "" {
			id, err := strconv.ParseInt(formClub, 10, 64)
			if err != nil {
				return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid club_id")
			}
			clubID = id
		}
	} else {
		if user.ClubID == nil {
			return apiError(c, fiber.StatusForbidden, "club_scope_required", "club scope required")
		}
		clubID = *user.ClubID
	}

	custom := c.FormValue("custom_fields", "{}")
	if !json.Valid([]byte(custom)) {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "custom_fields must be valid JSON")
	}
	_, err := h.store.InsertMember(models.Member{ClubID: clubID, FullName: c.FormValue("full_name"), EmailEncrypted: h.crypto.Encrypt(c.FormValue("email")), PhoneEncrypted: h.crypto.Encrypt(c.FormValue("phone")), JoinDate: c.FormValue("join_date"), PositionTitle: c.FormValue("position_title"), IsActive: c.FormValue("is_active", "true") == "true", GroupName: c.FormValue("group_name"), CustomFields: h.encryptCustomFields(custom)})
	if err != nil {
		return h.writeServiceError(c, err)
	}
	return c.SendString("member created")
}

func (h *Handler) updateMember(c *fiber.Ctx) error {
	user := currentUser(c)
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid member id")
	}
	member, err := h.store.GetMemberByID(id)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	if user.Role != "admin" && (user.ClubID == nil || *user.ClubID != member.ClubID) {
		return apiError(c, fiber.StatusForbidden, "forbidden", "You are not allowed to perform this action.")
	}
	custom := c.FormValue("custom_fields", "{}")
	if !json.Valid([]byte(custom)) {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "custom_fields must be valid JSON")
	}
	member.FullName = c.FormValue("full_name")
	member.EmailEncrypted = h.crypto.Encrypt(c.FormValue("email"))
	member.PhoneEncrypted = h.crypto.Encrypt(c.FormValue("phone"))
	member.JoinDate = c.FormValue("join_date")
	member.PositionTitle = c.FormValue("position_title")
	member.IsActive = c.FormValue("is_active", "true") == "true"
	member.GroupName = c.FormValue("group_name")
	member.CustomFields = h.encryptCustomFields(custom)
	if err := h.store.UpdateMember(*member); err != nil {
		return h.writeServiceError(c, err)
	}
	return c.SendString("member updated")
}

func (h *Handler) exportMembersCSV(c *fiber.Ctx) error {
	user := currentUser(c)
	if err := requireManagedClub(user); err != nil {
		return apiError(c, fiber.StatusForbidden, "forbidden", "You are not allowed to perform this action.")
	}
	var clubID *int64
	if user.Role == "admin" {
		requested := strings.TrimSpace(c.Query("club_id"))
		switch requested {
		case "", "all":
			clubID = nil
		default:
			parsed, err := strconv.ParseInt(requested, 10, 64)
			if err != nil {
				return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid club_id")
			}
			clubID = &parsed
		}
	} else {
		clubID = user.ClubID
		if clubID == nil {
			def := int64(1)
			clubID = &def
		}
	}
	members, err := h.store.ListMembersPagedScoped(clubID, c.Query("group"), c.Query("search"), c.Query("sort", "created_at"), 0, 0)
	if err != nil {
		return h.writeServiceError(c, err)
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"id", "full_name", "email", "phone", "join_date", "position_title", "is_active", "group_name", "custom_fields"})
	for _, m := range members {
		email, _ := h.crypto.Decrypt(m.EmailEncrypted)
		phone, _ := h.crypto.Decrypt(m.PhoneEncrypted)
		custom := h.decryptAndMaybeMigrateCustomFields(&m)
		active := "false"
		if m.IsActive {
			active = "true"
		}
		_ = w.Write([]string{strconv.FormatInt(m.ID, 10), m.FullName, email, phone, m.JoinDate, m.PositionTitle, active, m.GroupName, custom})
	}
	w.Flush()
	c.Set("Content-Type", "text/csv")
	return c.SendString(b.String())
}

func (h *Handler) importMembersCSV(c *fiber.Ctx) error {
	user := currentUser(c)
	if err := requireManagedClub(user); err != nil {
		return apiError(c, fiber.StatusForbidden, "forbidden", "You are not allowed to perform this action.")
	}
	fh, err := c.FormFile("file")
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "file required")
	}
	f, err := fh.Open()
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Request could not be processed.")
	}
	defer f.Close()
	r := csv.NewReader(f)
	rowNum := 0
	inserted := 0
	errorsOut := [][]string{{"row", "error"}}
	clubID := int64(1)
	if user.ClubID != nil {
		clubID = *user.ClubID
	}
	for {
		row, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return apiError(c, fiber.StatusBadRequest, "bad_request", "Request could not be processed.")
		}
		rowNum++
		if rowNum == 1 {
			continue
		}
		if rowNum > 5001 {
			errorsOut = append(errorsOut, []string{strconv.Itoa(rowNum), "row limit exceeded (5000)"})
			break
		}
		memberRow, err := parseMemberImportRow(row)
		if err != nil {
			errorsOut = append(errorsOut, []string{strconv.Itoa(rowNum), err.Error()})
			continue
		}
		custom := memberRow[7]
		if custom == "" {
			custom = "{}"
		}
		if !json.Valid([]byte(custom)) {
			errorsOut = append(errorsOut, []string{strconv.Itoa(rowNum), "invalid custom_fields json"})
			continue
		}
		isActive := strings.ToLower(strings.TrimSpace(memberRow[5])) != "false"
		_, err = h.store.InsertMember(models.Member{ClubID: clubID, FullName: memberRow[0], EmailEncrypted: h.crypto.Encrypt(memberRow[1]), PhoneEncrypted: h.crypto.Encrypt(memberRow[2]), JoinDate: memberRow[3], PositionTitle: memberRow[4], IsActive: isActive, GroupName: memberRow[6], CustomFields: h.encryptCustomFields(custom)})
		if err != nil {
			errorsOut = append(errorsOut, []string{strconv.Itoa(rowNum), err.Error()})
			continue
		}
		inserted++
	}
	if rowNum <= 1 {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "csv empty")
	}
	if len(errorsOut) > 1 {
		var b strings.Builder
		w := csv.NewWriter(&b)
		for _, rec := range errorsOut {
			_ = w.Write(rec)
		}
		w.Flush()
		c.Set("Content-Type", "text/csv")
		c.Set("Content-Disposition", "attachment; filename=member_import_errors.csv")
		return c.Status(422).SendString(b.String())
	}
	return c.SendString("members imported: " + strconv.Itoa(inserted))
}

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
