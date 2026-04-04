package API_tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"clubops_portal/internal/services"
)

func TestLoginAndCreateBudget(t *testing.T) {
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
	bform := url.Values{}
	bform.Set("club_id", "1")
	bform.Set("period_type", "monthly")
	bform.Set("period_start", "2026-03")
	bform.Set("account_code", "acct-1")
	bform.Set("campus_code", "camp-1")
	bform.Set("project_code", "proj-1")
	bform.Set("amount", "2000")
	breq := httptest.NewRequest(http.MethodPost, "/api/budgets", strings.NewReader(bform.Encode()))
	breq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(breq, auth)
	bresp, err := app.Test(breq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if bresp.StatusCode != 200 {
		body, _ := io.ReadAll(bresp.Body)
		t.Fatalf("expected 200 creating budget, got %d body=%s", bresp.StatusCode, string(body))
	}
}

func TestAdminBudgetCreationRequiresClubID(t *testing.T) {
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
	bform := url.Values{}
	bform.Set("period_type", "monthly")
	bform.Set("period_start", "2026-03")
	bform.Set("amount", "2000")
	breq := httptest.NewRequest(http.MethodPost, "/api/budgets", strings.NewReader(bform.Encode()))
	breq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(breq, auth)
	bresp, err := app.Test(breq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if bresp.StatusCode != 400 {
		body, _ := io.ReadAll(bresp.Body)
		t.Fatalf("expected 400 missing club_id for admin budget create, got %d body=%s", bresp.StatusCode, string(body))
	}
}

func TestAuthorizationMemberCannotCreateBudget(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	h, err := authSvc.HashPassword("MemberPassword1!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("member1", h, "member", nil); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "member1", "MemberPassword1!")

	bform := url.Values{}
	bform.Set("club_id", "1")
	bform.Set("period_type", "monthly")
	bform.Set("period_start", "2026-03")
	bform.Set("amount", "2000")
	breq := httptest.NewRequest(http.MethodPost, "/api/budgets", strings.NewReader(bform.Encode()))
	breq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(breq, auth)
	bresp, err := app.Test(breq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if bresp.StatusCode != 403 {
		body, _ := io.ReadAll(bresp.Body)
		t.Fatalf("expected 403, got %d body=%s", bresp.StatusCode, string(body))
	}
}

func TestOrganizerBudgetCreationForcesOwnClubScope(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`INSERT OR IGNORE INTO clubs (id, name) VALUES (2, 'Club Two')`); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("org-budget-scope", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org-budget-scope", "OrganizerPass123!")
	form := url.Values{}
	form.Set("club_id", "2")
	form.Set("period_type", "monthly")
	form.Set("period_start", "2026-03")
	form.Set("account_code", "acct-1")
	form.Set("campus_code", "camp-1")
	form.Set("project_code", "proj-1")
	form.Set("amount", "1500")
	req := httptest.NewRequest(http.MethodPost, "/api/budgets", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected budget create success, got %d body=%s", resp.StatusCode, string(body))
	}
	var clubID int64
	if err := st.DB.QueryRow(`SELECT club_id FROM budgets ORDER BY id DESC LIMIT 1`).Scan(&clubID); err != nil {
		t.Fatal(err)
	}
	if clubID != 1 {
		t.Fatalf("expected organizer budget to stay in own club 1, got %d", clubID)
	}
}

func TestAuditLogInsertedForMutation(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	h, err := authSvc.HashPassword("StrongAdmin123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdatePassword(1, h, false); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	bform := url.Values{}
	bform.Set("period_type", "monthly")
	bform.Set("period_start", "2026-03")
	bform.Set("account_code", "acct-1")
	bform.Set("campus_code", "camp-1")
	bform.Set("project_code", "proj-1")
	bform.Set("amount", "2000")
	breq := httptest.NewRequest(http.MethodPost, "/api/budgets", strings.NewReader(bform.Encode()))
	breq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(breq, auth)
	_, err = app.Test(breq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := st.DB.QueryRow(`SELECT COUNT(1) FROM audit_logs WHERE method = 'POST' AND path = '/api/budgets'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatalf("expected audit log row for budget mutation")
	}
}

func TestBudgetProjectionEndpoint(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	if _, err := st.DB.Exec(`INSERT INTO budgets (club_id, account_code, campus_code, project_code, period_type, period_start, amount, spent, created_by, status) VALUES (1, 'acct', 'camp', 'proj', 'monthly', '2026-03', 1000, 400, 1, 'active')`); err != nil {
		t.Fatal(err)
	}
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("org-proj", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org-proj", "OrganizerPass123!")
	req := httptest.NewRequest(http.MethodGet, "/api/budgets/1/projection?expected_remaining_spend=100", nil)
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 projection, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestBudgetProjectionHTMXReturnsReadableCard(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	if _, err := st.DB.Exec(`INSERT INTO budgets (club_id, account_code, campus_code, project_code, period_type, period_start, amount, spent, created_by, status) VALUES (1, 'acct', 'camp', 'proj', 'monthly', '2026-03', 1000, 400, 1, 'active')`); err != nil {
		t.Fatal(err)
	}
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("org-proj-htmx", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org-proj-htmx", "OrganizerPass123!")
	req := httptest.NewRequest(http.MethodGet, "/api/budgets/1/projection?expected_remaining_spend=100", nil)
	req.Header.Set("HX-Request", "true")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(body), "Projected end balance") {
		t.Fatalf("expected projection card html, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestAdminCannotSelfApproveOwnBudgetChange(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, _ := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`INSERT INTO budgets (id, club_id, account_code, campus_code, project_code, period_type, period_start, amount, spent, created_by, status) VALUES (10, 1, 'acct', 'camp', 'proj', 'monthly', '2026-03', 1000, 0, 1, 'active')`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`INSERT INTO budget_change_requests (id, budget_id, requested_by, proposed_amount, change_percent, reason, status) VALUES (10, 10, 1, 1200, 20, 'expansion', 'pending')`); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	form := url.Values{}
	form.Set("decision", "approve")
	req := httptest.NewRequest(http.MethodPost, "/api/budget_change_requests/10/review", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 422 self-approval denial, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestBudgetProjectionNotFound(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, _ := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	req := httptest.NewRequest(http.MethodGet, "/api/budgets/999/projection", nil)
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 404 missing budget, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestBudgetProjectionInvalidIDReturnsSchemaError(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, _ := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	req := httptest.NewRequest(http.MethodGet, "/api/budgets/not-a-number/projection", nil)
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400 invalid id, got %d body=%s", resp.StatusCode, string(body))
	}
	body, _ := io.ReadAll(resp.Body)
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("expected JSON error payload, got body=%s", string(body))
	}
	if payload["error_code"] != "validation_error" {
		t.Fatalf("expected validation_error code, got %#v", payload["error_code"])
	}
}

func TestTeamLeadWithoutClubCannotAccessScopedBudgetEndpoints(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("LeadPass12345!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("lead-noclub", hash, "team_lead", nil); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "lead-noclub", "LeadPass12345!")
	for _, path := range []string{"/partials/budgets/list", "/api/budgets/1/projection"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		addAuth(req, auth)
		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 403 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 403 for %s, got %d body=%s", path, resp.StatusCode, string(body))
		}
	}
}

func TestBudgetChangeReviewDuplicateSubmitReturnsNonSuccess(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, _ := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("budget-requester", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	requester, err := st.FindUserByUsername("budget-requester")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`INSERT INTO budgets (id, club_id, account_code, campus_code, project_code, period_type, period_start, amount, spent, created_by, status) VALUES (20, 1, 'acct', 'camp', 'proj', 'monthly', '2026-03', 1000, 0, ?, 'active')`, requester.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`INSERT INTO budget_change_requests (id, budget_id, requested_by, proposed_amount, change_percent, reason, status) VALUES (20, 20, ?, 1100, 10, 'adjust', 'pending')`, requester.ID); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	form := url.Values{}
	form.Set("decision", "approve")
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/budget_change_requests/20/review", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		addAuth(req, auth)
		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected first review submit success, got %d body=%s", resp.StatusCode, string(body))
		}
		if i == 1 && resp.StatusCode == 200 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected duplicate review submit to be rejected, got %d body=%s", resp.StatusCode, string(body))
		}
	}
}

func TestAdminBudgetsPageIncludesClubSelectorOnCreateForm(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, _ := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`INSERT OR IGNORE INTO clubs (id, name) VALUES (2, 'Club Two')`); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	req := httptest.NewRequest(http.MethodGet, "/budgets", nil)
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(body), "name=\"club_id\"") {
		t.Fatalf("expected admin budgets page to include club selector, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestBudgetCreateRejectsInvalidPeriodStartFormat(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, _ := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	form := url.Values{}
	form.Set("club_id", "1")
	form.Set("period_type", "monthly")
	form.Set("period_start", "2026/03")
	form.Set("account_code", "acct-1")
	form.Set("campus_code", "camp-1")
	form.Set("project_code", "proj-1")
	form.Set("amount", "1000")
	req := httptest.NewRequest(http.MethodPost, "/api/budgets", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 422 invalid period_start format, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestBudgetCreateRejectsMissingDimensionFields(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, _ := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	form := url.Values{}
	form.Set("club_id", "1")
	form.Set("period_type", "monthly")
	form.Set("period_start", "2026-03")
	form.Set("amount", "1000")
	req := httptest.NewRequest(http.MethodPost, "/api/budgets", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 422 missing dimension fields, got %d body=%s", resp.StatusCode, string(body))
	}
}
