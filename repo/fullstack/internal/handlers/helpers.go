package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"clubops_portal/fullstack/internal/models"

	"github.com/gofiber/fiber/v2"
)

const customFieldsEncPrefix = "enc:v1:"

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

func splitClean(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := []string{}
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func parseMemberImportRow(row []string) ([]string, error) {
	if len(row) >= 9 {
		trimmed := make([]string, 8)
		copy(trimmed, row[1:9])
		return trimmed, nil
	}
	if len(row) >= 8 {
		trimmed := make([]string, 8)
		copy(trimmed, row[:8])
		return trimmed, nil
	}
	return nil, errors.New("requires full_name,email,phone,join_date,position_title,is_active,group_name,custom_fields (optional leading id allowed)")
}

func imagesFromJSON(raw string) []string {
	if raw == "" {
		return nil
	}
	out := []string{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func (h *Handler) render(c *fiber.Ctx, template string, data fiber.Map, layout ...string) error {
	if data == nil {
		data = fiber.Map{}
	}
	data["CSRFToken"] = c.Locals("csrf_token")
	return c.Render(template, data, layout...)
}

func buildClubViews(clubs []models.Club) []clubView {
	out := make([]clubView, 0, len(clubs))
	for _, club := range clubs {
		out = append(out, buildClubView(&club))
	}
	return out
}

func buildClubView(club *models.Club) clubView {
	if club == nil {
		return clubView{}
	}
	return clubView{ID: club.ID, Name: club.Name, TagsRaw: strings.Join(tagsFromJSON(club.Tags), ", "), Tags: tagsFromJSON(club.Tags), AvatarPath: club.AvatarPath, RecruitmentOpen: club.RecruitmentOpen, Description: club.Description}
}

func tagsFromJSON(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err == nil {
		return tags
	}
	return splitClean(raw)
}

func parseRegionRowsCSV(raw string) ([][3]string, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	rows := make([][3]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		parts := splitClean(trimmed)
		if len(parts) != 3 {
			return nil, errors.New("rows_csv requires state,county,city per line")
		}
		rows = append(rows, [3]string{parts[0], parts[1], parts[2]})
	}
	if len(rows) == 0 {
		return nil, errors.New("at least one region row required")
	}
	return rows, nil
}

func regionRowsToCSV(rows []models.RegionNode) string {
	if len(rows) == 0 {
		return ""
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, row.State+","+row.County+","+row.City)
	}
	return strings.Join(lines, "\n")
}

func saveAvatarFile(fh *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return "", errors.New("avatar must be jpg or png")
	}
	if fh.Size > 2*1024*1024 {
		return "", errors.New("avatar must be <= 2MB")
	}
	dir := filepath.Join(".", "fullstack", "static", "uploads", "avatars")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := strconv.FormatInt(time.Now().UnixNano(), 10) + "_" + filepath.Base(fh.Filename)
	target := filepath.Join(dir, name)
	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()
	out, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := out.ReadFrom(src); err != nil {
		return "", err
	}
	return "/static/uploads/avatars/" + name, nil
}

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

func (h *Handler) encryptCustomFields(raw string) string {
	return customFieldsEncPrefix + h.crypto.Encrypt(raw)
}

func (h *Handler) decryptAndMaybeMigrateCustomFields(member *models.Member) string {
	stored := strings.TrimSpace(member.CustomFields)
	if stored == "" {
		return "{}"
	}
	if strings.HasPrefix(stored, customFieldsEncPrefix) {
		plain, _ := h.crypto.Decrypt(strings.TrimPrefix(stored, customFieldsEncPrefix))
		if json.Valid([]byte(plain)) {
			return plain
		}
		return "{}"
	}
	if json.Valid([]byte(stored)) {
		member.CustomFields = h.encryptCustomFields(stored)
		_ = h.store.UpdateMember(*member)
		return stored
	}
	decrypted, _ := h.crypto.Decrypt(stored)
	if json.Valid([]byte(decrypted)) {
		member.CustomFields = h.encryptCustomFields(decrypted)
		_ = h.store.UpdateMember(*member)
		return decrypted
	}
	return "{}"
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
