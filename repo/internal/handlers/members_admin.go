package handlers

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"clubops_portal/internal/models"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) createMember(c *fiber.Ctx) error {
	user := currentUser(c)
	var clubID int64
	if user.Role == "admin" {
		formClub := strings.TrimSpace(c.FormValue("club_id"))
		if formClub == "" {
			return apiError(c, fiber.StatusBadRequest, "validation_error", "club_id required")
		}
		id, err := strconv.ParseInt(formClub, 10, 64)
		if err != nil {
			return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid club_id")
		}
		clubID = id
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
	fullName := c.FormValue("full_name")
	email := c.FormValue("email")
	phone := c.FormValue("phone")
	joinDate := c.FormValue("join_date")
	if err := validateMemberCoreFields(fullName, email, phone, joinDate); err != nil {
		return apiError(c, fiber.StatusUnprocessableEntity, "validation_error", err.Error())
	}
	_, err := h.store.InsertMember(models.Member{ClubID: clubID, FullName: fullName, EmailEncrypted: h.crypto.Encrypt(email), PhoneEncrypted: h.crypto.Encrypt(phone), JoinDate: joinDate, PositionTitle: c.FormValue("position_title"), IsActive: c.FormValue("is_active", "true") == "true", GroupName: c.FormValue("group_name"), CustomFields: h.encryptCustomFields(custom)})
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
	fullName := c.FormValue("full_name")
	email := c.FormValue("email")
	phone := c.FormValue("phone")
	joinDate := c.FormValue("join_date")
	if err := validateMemberCoreFields(fullName, email, phone, joinDate); err != nil {
		return apiError(c, fiber.StatusUnprocessableEntity, "validation_error", err.Error())
	}
	member.FullName = fullName
	member.EmailEncrypted = h.crypto.Encrypt(email)
	member.PhoneEncrypted = h.crypto.Encrypt(phone)
	member.JoinDate = joinDate
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
			return apiError(c, fiber.StatusForbidden, "club_scope_required", "club scope required")
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

	// Audit log: record that a PII export occurred
	clubScope := "all"
	if clubID != nil {
		clubScope = strconv.FormatInt(*clubID, 10)
	}
	_ = h.store.InsertAuditLog(&user.ID, "GET", "/api/members/export", "members", clubScope,
		nil, map[string]any{"action": "csv_export", "row_count": len(members), "club_id": clubScope})

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
	header, err := r.Read()
	if err != nil {
		if err == io.EOF {
			return apiError(c, fiber.StatusBadRequest, "validation_error", "csv empty")
		}
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Request could not be processed.")
	}
	hasLeadingID, err := validateMemberImportHeader(header)
	if err != nil {
		return apiError(c, fiber.StatusUnprocessableEntity, "validation_error", err.Error())
	}
	rowNum := 1
	inserted := 0
	errorsOut := [][]string{{"row", "error"}}
	var clubID int64
	if user.Role == "admin" {
		parsed, err := strconv.ParseInt(strings.TrimSpace(c.FormValue("club_id")), 10, 64)
		if err != nil {
			return apiError(c, fiber.StatusBadRequest, "validation_error", "invalid club_id")
		}
		clubID = parsed
	} else {
		if user.ClubID == nil {
			return apiError(c, fiber.StatusForbidden, "club_scope_required", "club scope required")
		}
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
		if rowNum > 5001 {
			errorsOut = append(errorsOut, []string{strconv.Itoa(rowNum), "row limit exceeded (5000)"})
			break
		}
		memberRow, err := parseMemberImportRow(row, hasLeadingID)
		if err != nil {
			errorsOut = append(errorsOut, []string{strconv.Itoa(rowNum), err.Error()})
			continue
		}
		if err := validateMemberCoreFields(memberRow[0], memberRow[1], memberRow[2], memberRow[3]); err != nil {
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
	if rowNum == 1 {
		return apiError(c, fiber.StatusBadRequest, "validation_error", "csv empty")
	}
	if len(errorsOut) > 1 {
		var b strings.Builder
		w := csv.NewWriter(&b)
		for _, rec := range errorsOut {
			_ = w.Write(rec)
		}
		w.Flush()
		reportCSV := b.String()
		if c.Get("HX-Request") == "true" {
			reportPath, err := saveMemberImportErrorReport(reportCSV)
			if err != nil {
				return h.writeServiceError(c, err)
			}
			return c.Status(422).SendString(`<div class="rounded border border-amber-300 bg-amber-50 px-3 py-2 text-amber-800">Import completed with row errors. <a class="underline font-medium" href="` + reportPath + `" download>Download error report CSV</a></div>`)
		}
		c.Set("Content-Type", "text/csv")
		c.Set("Content-Disposition", "attachment; filename=member_import_errors.csv")
		return c.Status(422).SendString(reportCSV)
	}
	return c.SendString("members imported: " + strconv.Itoa(inserted))
}

func validateMemberCoreFields(fullName, email, phone, joinDate string) error {
	if strings.TrimSpace(fullName) == "" {
		return errors.New("full_name required")
	}
	if strings.TrimSpace(email) == "" {
		return errors.New("email required")
	}
	if strings.TrimSpace(phone) == "" {
		return errors.New("phone required")
	}
	if strings.TrimSpace(joinDate) == "" {
		return errors.New("join_date required")
	}
	if _, err := time.Parse("2006-01-02", strings.TrimSpace(joinDate)); err != nil {
		return errors.New("join_date must be YYYY-MM-DD")
	}
	return nil
}
