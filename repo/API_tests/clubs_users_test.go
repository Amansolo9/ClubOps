package API_tests

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"clubops_portal/internal/services"
)

func TestOrganizerCannotUpdateAnotherClubProfile(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.DB.Exec(`INSERT OR IGNORE INTO clubs (id, name) VALUES (2, 'Club Two')`)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("org1", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org1", "OrganizerPass123!")
	form := url.Values{}
	form.Set("name", "Updated Club")
	req := httptest.NewRequest(http.MethodPost, "/api/clubs/2/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 403 {
		t.Fatalf("expected 403 for cross-club update, got %d", resp.StatusCode)
	}
}

func TestTeamLeadCanUpdateOwnClubProfile(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("LeadPass12345!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("lead1", hash, "team_lead", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "lead1", "LeadPass12345!")
	form := url.Values{}
	form.Set("name", "Default Club")
	form.Set("description", "Updated by lead")
	req := httptest.NewRequest(http.MethodPost, "/api/clubs/1/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 for own club update, got %d body=%s", resp.StatusCode, string(body))
	}
	club, err := st.GetClubByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if club.Description != "Updated by lead" {
		t.Fatalf("expected updated club description, got %q", club.Description)
	}
}

func TestAdminClubCreateAndListWorkflow(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, _ := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	form := url.Values{}
	form.Set("name", "North Campus Club")
	form.Set("tags", "campus,north")
	form.Set("description", "New campus org")
	req := httptest.NewRequest(http.MethodPost, "/api/clubs", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 club create, got %d body=%s", resp.StatusCode, string(body))
	}
	pageReq := httptest.NewRequest(http.MethodGet, "/clubs", nil)
	addAuth(pageReq, auth)
	pageResp, err := app.Test(pageReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	pageBody, _ := io.ReadAll(pageResp.Body)
	if pageResp.StatusCode != 200 || !strings.Contains(string(pageBody), "North Campus Club") {
		t.Fatalf("expected clubs page to include new club, got %d body=%s", pageResp.StatusCode, string(pageBody))
	}
}

func TestAdminUserManagementUpdatesRoleAndClub(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, _ := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("MemberPassword1!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("promote-me", hash, "member", nil); err != nil {
		t.Fatal(err)
	}
	user, err := st.FindUserByUsername("promote-me")
	if err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	form := url.Values{}
	form.Set("role", "team_lead")
	form.Set("club_id", "1")
	req := httptest.NewRequest(http.MethodPost, "/api/users/"+strconv.FormatInt(user.ID, 10), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 user update, got %d body=%s", resp.StatusCode, string(body))
	}
	updated, err := st.FindUserByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Role != "team_lead" || updated.ClubID == nil || *updated.ClubID != 1 {
		t.Fatalf("expected updated team lead scope")
	}
}

func TestUsersPageIncludesAdminResetForm(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, _ := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(body), "/api/auth/admin-reset") {
		t.Fatalf("expected users page to include admin reset form, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestClubProfileAvatarRejectsMismatchedImageContent(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("LeadPass12345!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("lead-avatar", hash, "team_lead", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "lead-avatar", "LeadPass12345!")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("name", "Default Club"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("avatar", "avatar.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("not-a-jpeg")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/clubs/1/profile", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 422 invalid avatar content, got %d body=%s", resp.StatusCode, string(respBody))
	}
}
