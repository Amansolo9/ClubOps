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

func TestUnauthenticatedBudgetAPIRejected(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/budgets", strings.NewReader("period_type=monthly&period_start=2026-03&amount=1000"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 401, got %d body=%s", resp.StatusCode, string(body))
	}
	body, _ := io.ReadAll(resp.Body)
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("expected JSON auth error payload, got body=%s", string(body))
	}
	if payload["error_code"] != "unauthorized" {
		t.Fatalf("expected unauthorized code, got %#v", payload["error_code"])
	}
}

func TestAuthenticatedMutationWithoutCSRFRejected(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, _ := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	req := httptest.NewRequest(http.MethodPost, "/api/budgets", strings.NewReader("period_type=monthly&period_start=2026-03&amount=1000"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: auth.Session})
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: auth.CSRF})
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 403 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected CSRF 403, got %d body=%s", resp.StatusCode, string(body))
	}
	body, _ := io.ReadAll(resp.Body)
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("expected JSON csrf error payload, got body=%s", string(body))
	}
	if payload["error_code"] != "csrf_invalid" {
		t.Fatalf("expected csrf_invalid code, got %#v", payload["error_code"])
	}
}

func TestPasswordPolicyRejectsShortRegistration(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()

	form := url.Values{}
	form.Set("username", "shortpass")
	form.Set("password", "short")
	form.Set("role", "member")
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "{") {
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("expected JSON error payload, got body=%s", string(body))
		}
		if payload["error_code"] != "validation_error" {
			t.Fatalf("expected validation_error code, got %#v", payload["error_code"])
		}
		if msg, _ := payload["message"].(string); strings.TrimSpace(msg) == "" {
			t.Fatalf("expected user-facing message in payload")
		}
	} else if trimmed == "" {
		t.Fatalf("expected non-empty user-facing validation message")
	}
}

func TestDuplicateRegistrationReturnsConflict(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	form := url.Values{}
	form.Set("username", "dupe-user")
	form.Set("password", "StrongPass123!")
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected first registration success, got %d", resp.StatusCode)
	}
	req2 := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp2, err := app.Test(req2, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != 409 {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 409 duplicate registration, got %d body=%s", resp2.StatusCode, string(body))
	}
}

func TestForcedPasswordChangeBlocksPagesUntilChanged(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("ForcePass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("force-me", hash, "member", nil); err != nil {
		t.Fatal(err)
	}
	user, err := st.FindUserByUsername("force-me")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdatePassword(user.ID, hash, true); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "force-me", "ForcePass123!")
	req := httptest.NewRequest(http.MethodGet, "/reviews", nil)
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 302 {
		t.Fatalf("expected redirect while password change required, got %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/change-password" {
		t.Fatalf("expected redirect to /change-password, got %q", location)
	}
	form := url.Values{}
	form.Set("new_password", "NewForcePass123!")
	changeReq := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(form.Encode()))
	changeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(changeReq, auth)
	changeResp, err := app.Test(changeReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if changeResp.StatusCode != 200 {
		body, _ := io.ReadAll(changeResp.Body)
		t.Fatalf("expected password change success, got %d body=%s", changeResp.StatusCode, string(body))
	}
}

func TestExpiredAdminCanRecoverViaChangePasswordPage(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("ExpiredAdmin123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdatePassword(1, hash, false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`UPDATE users SET password_set_at = ? WHERE id = 1`, time.Now().Add(-181*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "ExpiredAdmin123!")
	blockedReq := httptest.NewRequest(http.MethodGet, "/users", nil)
	addAuth(blockedReq, auth)
	blockedResp, err := app.Test(blockedReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if blockedResp.StatusCode != 302 || blockedResp.Header.Get("Location") != "/change-password" {
		t.Fatalf("expected redirect to change-password for expired admin, got status=%d location=%q", blockedResp.StatusCode, blockedResp.Header.Get("Location"))
	}
	pageReq := httptest.NewRequest(http.MethodGet, "/change-password", nil)
	addAuth(pageReq, auth)
	pageResp, err := app.Test(pageReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	pageBody, _ := io.ReadAll(pageResp.Body)
	if pageResp.StatusCode != 200 || !strings.Contains(string(pageBody), "/api/auth/change-password") {
		t.Fatalf("expected change-password page with form, got %d body=%s", pageResp.StatusCode, string(pageBody))
	}
	form := url.Values{}
	form.Set("new_password", "RecoveredAdmin123!")
	changeReq := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", strings.NewReader(form.Encode()))
	changeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(changeReq, auth)
	changeResp, err := app.Test(changeReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if changeResp.StatusCode != 200 {
		body, _ := io.ReadAll(changeResp.Body)
		t.Fatalf("expected recovery password change success, got %d body=%s", changeResp.StatusCode, string(body))
	}
	retryReq := httptest.NewRequest(http.MethodGet, "/users", nil)
	addAuth(retryReq, auth)
	retryResp, err := app.Test(retryReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if retryResp.StatusCode != 200 {
		body, _ := io.ReadAll(retryResp.Body)
		t.Fatalf("expected admin access restored after password change, got %d body=%s", retryResp.StatusCode, string(body))
	}
}

func TestAdminOnlyRoutesRejectOrganizer(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("org-admincheck", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org-admincheck", "OrganizerPass123!")
	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/flags"},
		{method: http.MethodPost, path: "/api/auth/admin-reset", body: "user_id=1&temp_password=ResetPass123!"},
		{method: http.MethodPost, path: "/api/credit_rules", body: "version=v1&weight=1&effective_from=2026-01-01"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		addAuth(req, auth)
		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 403 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 403 for %s %s, got %d body=%s", tc.method, tc.path, resp.StatusCode, string(body))
		}
	}
}

func TestRegistrationCannotEscalateRole(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	form := url.Values{}
	form.Set("username", "selflead")
	form.Set("password", "StrongPass123!")
	form.Set("role", "team_lead")
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected registration success as member, got %d", resp.StatusCode)
	}
	u, err := st.FindUserByUsername("selflead")
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != "member" {
		t.Fatalf("expected forced member role, got %s", u.Role)
	}
}

func TestRegistrationIgnoresClubAssignment(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	if _, err := st.DB.Exec(`INSERT OR IGNORE INTO clubs (id, name) VALUES (2, 'Club Two')`); err != nil {
		t.Fatal(err)
	}
	form := url.Values{}
	form.Set("username", "selfclub")
	form.Set("password", "StrongPass123!")
	form.Set("club_id", "2")
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected registration success, got %d body=%s", resp.StatusCode, string(body))
	}
	u, err := st.FindUserByUsername("selfclub")
	if err != nil {
		t.Fatal(err)
	}
	if u.ClubID != nil {
		t.Fatalf("expected public register to ignore club_id assignment")
	}
}

func TestAuthenticatedResponsesSetNoStoreAndLogoutClearsSession(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, _ := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	pageReq := httptest.NewRequest(http.MethodGet, "/users", nil)
	addAuth(pageReq, auth)
	pageResp, err := app.Test(pageReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if pageResp.Header.Get("Cache-Control") == "" || !strings.Contains(pageResp.Header.Get("Cache-Control"), "no-store") {
		t.Fatalf("expected no-store cache header on authenticated page, got %q", pageResp.Header.Get("Cache-Control"))
	}
	logoutReq := httptest.NewRequest(http.MethodPost, "/logout", nil)
	addAuth(logoutReq, auth)
	logoutResp, err := app.Test(logoutReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	cookieHeader := strings.ToLower(strings.Join(logoutResp.Header.Values("Set-Cookie"), ";"))
	if !strings.Contains(cookieHeader, "session_token=") {
		t.Fatalf("expected logout to clear session cookie, headers=%v", logoutResp.Header.Values("Set-Cookie"))
	}
}

func TestMemberCannotAccessScopedPages(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	h, err := authSvc.HashPassword("MemberPassword1!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("member2", h, "member", nil); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "member2", "MemberPassword1!")

	req1 := httptest.NewRequest(http.MethodGet, "/members", nil)
	addAuth(req1, auth)
	r1, err := app.Test(req1, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if r1.StatusCode != 403 {
		t.Fatalf("expected 403 for /members, got %d", r1.StatusCode)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/partials/budgets/list", nil)
	addAuth(req2, auth)
	r2, err := app.Test(req2, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if r2.StatusCode != 403 {
		t.Fatalf("expected 403 for budget partial, got %d", r2.StatusCode)
	}
}

func TestAuditRedactsAuthPayload(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	form := url.Values{}
	form.Set("username", "nouser")
	form.Set("password", "SecretPassword123!")
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, _ = app.Test(req, 5000)
	var after string
	if err := st.DB.QueryRow(`SELECT after_state FROM audit_logs WHERE path = '/login' ORDER BY id DESC LIMIT 1`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(after, "SecretPassword123!") || strings.Contains(strings.ToLower(after), "password") {
		t.Fatalf("expected auth payload to be redacted, got: %s", after)
	}
}
