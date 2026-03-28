package middleware

import (
	"bytes"
	"encoding/json"
	"mime"
	"net/url"
	"strconv"
	"strings"

	"clubops_portal/fullstack/internal/models"
	"clubops_portal/fullstack/internal/services"
	"clubops_portal/fullstack/internal/store"

	"github.com/gofiber/fiber/v2"
)

func AuditTrail(audit *services.AuditService, st *store.SQLiteStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		method := c.Method()
		if method != fiber.MethodPost && method != fiber.MethodPut && method != fiber.MethodDelete {
			return c.Next()
		}
		path := c.Path()
		entity, id := audit.ParseEntity(path)
		before, _ := st.FetchEntitySnapshot(entity, id)
		err := c.Next()

		var after any
		if method == fiber.MethodDelete {
			after = nil
		} else {
			if id == "" {
				after = sanitizeAuditPayload(path, c.Get("Content-Type"), c.Body())
			} else {
				after, _ = st.FetchEntitySnapshot(entity, id)
			}
		}
		var userID *int64
		if u := c.Locals("user"); u != nil {
			uid := u.(*models.User).ID
			userID = &uid
		}
		if id == "" {
			parts := strings.Split(strings.Trim(path, "/"), "/")
			if len(parts) > 1 {
				if _, convErr := strconv.Atoi(parts[len(parts)-1]); convErr == nil {
					id = parts[len(parts)-1]
				}
			}
		}
		audit.Write(userID, method, path, before, after)
		return err
	}
}

func sanitizeAuditPayload(path, contentType string, body []byte) any {
	if strings.HasPrefix(path, "/login") || strings.HasPrefix(path, "/api/auth/") {
		return map[string]any{"redacted": "auth payload omitted"}
	}
	if len(body) == 0 {
		return map[string]any{}
	}
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType == "application/x-www-form-urlencoded" {
		vals, err := url.ParseQuery(string(body))
		if err != nil {
			return map[string]any{"raw": "unparseable form payload"}
		}
		allow := auditAllowlistForPath(path)
		out := map[string]any{}
		for k, v := range vals {
			lk := strings.ToLower(k)
			if isSensitiveAuditField(lk) {
				out[k] = "[REDACTED]"
			} else if len(allow) > 0 && allow[lk] {
				if len(v) == 1 {
					out[k] = v[0]
				} else {
					out[k] = v
				}
			} else if len(v) == 1 {
				out[k] = "[REDACTED]"
			} else {
				out[k] = "[REDACTED]"
			}
		}
		return out
	}
	if mediaType == "application/json" {
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		var payload any
		if err := dec.Decode(&payload); err != nil {
			return map[string]any{"raw": "unparseable json payload"}
		}
		allow := auditAllowlistForPath(path)
		return redactJSONWithAllowlist(payload, allow)
	}
	if strings.HasPrefix(mediaType, "multipart/") {
		return map[string]any{"redacted": "multipart payload omitted"}
	}
	return map[string]any{"raw": "payload omitted"}
}

func redactJSONWithAllowlist(v any, allow map[string]bool) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range t {
			lk := strings.ToLower(k)
			if isSensitiveAuditField(lk) {
				out[k] = "[REDACTED]"
			} else if len(allow) > 0 && allow[lk] {
				out[k] = val
			} else {
				out[k] = "[REDACTED]"
			}
		}
		return out
	case []any:
		arr := make([]any, 0, len(t))
		for _, it := range t {
			arr = append(arr, redactJSONWithAllowlist(it, allow))
		}
		return arr
	default:
		return "[REDACTED]"
	}
}

func isSensitiveAuditField(field string) bool {
	if strings.HasSuffix(field, "_id") || field == "id" {
		return true
	}
	return field == "password" || field == "new_password" || field == "temp_password" || field == "token" || field == "session_token" || field == "authorization" || field == "email" || field == "phone" || field == "custom_fields" || field == "comment"
}

func auditAllowlistForPath(path string) map[string]bool {
	allow := map[string]map[string]bool{
		"/api/budgets":                       {"period_type": true, "period_start": true, "amount": true, "account_code": true, "campus_code": true, "project_code": true},
		"/api/flags":                         {"flag_key": true, "enabled": true, "target_scope": true, "rollout_pct": true},
		"/api/credit_rules":                  {"version": true, "weight": true, "makeup_bonus": true, "retake_factor": true, "effective_from": true, "effective_to": true, "makeup_enabled": true, "retake_enabled": true, "thresholds_json": true, "deductions_json": true},
		"/api/budgets/projection":            {"expected_remaining_spend": true},
		"/api/budget_change_requests/review": {"decision": true},
	}
	if exact, ok := allow[path]; ok {
		return exact
	}
	if strings.HasSuffix(path, "/change") {
		return map[string]bool{"proposed_amount": true, "reason": true}
	}
	if strings.HasSuffix(path, "/spend") {
		return map[string]bool{"spent": true}
	}
	if strings.HasSuffix(path, "/projection") {
		return map[string]bool{"expected_remaining_spend": true}
	}
	if strings.HasSuffix(path, "/moderate") {
		return map[string]bool{"decision": true, "reason": true}
	}
	if strings.HasSuffix(path, "/appeal") {
		return map[string]bool{}
	}
	if strings.HasPrefix(path, "/api/regions") {
		return map[string]bool{"version_label": true}
	}
	if strings.HasPrefix(path, "/api/users/") {
		return map[string]bool{"role": true}
	}
	if strings.HasPrefix(path, "/api/clubs/") {
		return map[string]bool{"name": true, "tags": true, "recruitment_open": true, "description": true}
	}
	return map[string]bool{}
}
