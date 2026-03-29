package API_tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"clubops_portal/internal/services"
)

func TestFlagsMutationWritesAuditLog(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, err := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	req := httptest.NewRequest(http.MethodPost, "/api/flags", strings.NewReader("flag_key=credit_engine_v2&enabled=true&target_scope=role:organizer&rollout_pct=50"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 flag update, got %d body=%s", resp.StatusCode, string(body))
	}
	var count int
	if err := st.DB.QueryRow(`SELECT COUNT(1) FROM audit_logs WHERE method = 'POST' AND path = '/api/flags'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatalf("expected audit log row for flag mutation")
	}
}
