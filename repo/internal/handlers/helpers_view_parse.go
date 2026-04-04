package handlers

import (
	"encoding/json"
	"errors"
	"strings"

	"clubops_portal/internal/models"

	"github.com/gofiber/fiber/v2"
)

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

func parseMemberImportRow(row []string, hasLeadingID bool) ([]string, error) {
	if hasLeadingID {
		if len(row) != 9 {
			return nil, errors.New("row must contain exactly 9 columns for export-shaped csv")
		}
		trimmed := make([]string, 8)
		copy(trimmed, row[1:9])
		return trimmed, nil
	}
	if len(row) != 8 {
		return nil, errors.New("row must contain exactly 8 columns")
	}
	trimmed := make([]string, 8)
	copy(trimmed, row[:8])
	return trimmed, nil
}

func validateMemberImportHeader(header []string) (bool, error) {
	normalized := make([]string, len(header))
	for i := range header {
		normalized[i] = strings.ToLower(strings.TrimSpace(header[i]))
	}
	plain := []string{"full_name", "email", "phone", "join_date", "position_title", "is_active", "group_name", "custom_fields"}
	withID := []string{"id", "full_name", "email", "phone", "join_date", "position_title", "is_active", "group_name", "custom_fields"}
	if len(normalized) == len(plain) {
		for i := range plain {
			if normalized[i] != plain[i] {
				return false, errors.New("csv header must match full_name,email,phone,join_date,position_title,is_active,group_name,custom_fields or id-prefixed export schema")
			}
		}
		return false, nil
	}
	if len(normalized) == len(withID) {
		for i := range withID {
			if normalized[i] != withID[i] {
				return false, errors.New("csv header must match full_name,email,phone,join_date,position_title,is_active,group_name,custom_fields or id-prefixed export schema")
			}
		}
		return true, nil
	}
	return false, errors.New("csv header must match full_name,email,phone,join_date,position_title,is_active,group_name,custom_fields or id-prefixed export schema")
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
