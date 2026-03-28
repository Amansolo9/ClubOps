package API_tests

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"clubops_portal/fullstack/internal/services"
)

func TestMembersExportReturnsDecryptedCSV(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	cryptoSvc, err := services.NewCryptoService()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertMember(storeMember(1, cryptoSvc.Encrypt("alex@example.com"), cryptoSvc.Encrypt("123456"), "Alex Doe")); err != nil {
		t.Fatal(err)
	}
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("org-export", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org-export", "OrganizerPass123!")
	req := httptest.NewRequest(http.MethodGet, "/api/members/export", nil)
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(body), "alex@example.com") {
		t.Fatalf("expected decrypted export csv, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestOrganizerMemberCreationForcesOwnClubScope(t *testing.T) {
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
	if err := st.CreateUser("org-member-scope", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org-member-scope", "OrganizerPass123!")
	form := url.Values{}
	form.Set("club_id", "2")
	form.Set("full_name", "Scoped Member")
	form.Set("email", "scoped@example.com")
	form.Set("phone", "123")
	form.Set("join_date", "2026-03-01")
	form.Set("position_title", "Member")
	form.Set("is_active", "true")
	form.Set("group_name", "Alpha")
	form.Set("custom_fields", "{}")
	req := httptest.NewRequest(http.MethodPost, "/api/members", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected member create success, got %d body=%s", resp.StatusCode, string(body))
	}
	var clubID int64
	if err := st.DB.QueryRow(`SELECT club_id FROM members WHERE full_name = 'Scoped Member' ORDER BY id DESC LIMIT 1`).Scan(&clubID); err != nil {
		t.Fatal(err)
	}
	if clubID != 1 {
		t.Fatalf("expected organizer-created member to stay in own club 1, got %d", clubID)
	}
}

func TestMemberExportDeniedForMemberRole(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("MemberPassword1!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("member-export", hash, "member", nil); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "member-export", "MemberPassword1!")
	req := httptest.NewRequest(http.MethodGet, "/api/members/export", nil)
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 403 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 403 export denial, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestMemberImportRejectsInvalidCustomFields(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("org-badimport", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org-badimport", "OrganizerPass123!")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "members.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, err = part.Write([]byte("full_name,email,phone,join_date,position_title,is_active,group_name,custom_fields\nAlex Doe,alex@example.com,123,2026-03-01,Captain,true,Alpha,{bad-json}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/members/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 422 invalid custom_fields, got %d body=%s", resp.StatusCode, string(respBody))
	}
}

func TestMemberImportCSVSupportsExportShape(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("org-import", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org-import", "OrganizerPass123!")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "members.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, err = part.Write([]byte("id,full_name,email,phone,join_date,position_title,is_active,group_name,custom_fields\n1,Alex Doe,alex@example.com,123,2026-03-01,Captain,true,Alpha,\"{\"\"skill\"\":\"\"ops\"\"}\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/members/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 import success, got %d body=%s", resp.StatusCode, string(respBody))
	}
	rows, err := st.ListMembers(1, "", "Alex", "created_at")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].FullName != "Alex Doe" {
		t.Fatalf("expected imported member from export-shaped csv")
	}
}

func TestMemberImportCSVEnforcesRowLimit(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("org-limit", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org-limit", "OrganizerPass123!")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "members.csv")
	if err != nil {
		t.Fatal(err)
	}
	var csvBody strings.Builder
	csvBody.WriteString("full_name,email,phone,join_date,position_title,is_active,group_name,custom_fields\n")
	for i := 0; i < 5001; i++ {
		csvBody.WriteString("User,user@example.com,123,2026-03-01,Member,true,Alpha,{}\n")
	}
	_, err = part.Write([]byte(csvBody.String()))
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/members/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 422 row-limit response, got %d body=%s", resp.StatusCode, string(respBody))
	}
}

func TestMemberCreateAuditRedactsPII(t *testing.T) {
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
	form := url.Values{}
	form.Set("full_name", "Audit Target")
	form.Set("email", "audit-sensitive@example.com")
	form.Set("phone", "+1-555-444-3333")
	form.Set("join_date", "2026-03-01")
	form.Set("position_title", "Lead")
	form.Set("is_active", "true")
	form.Set("group_name", "A")
	form.Set("custom_fields", `{"ssn":"111-22-3333"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/members", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 create member, got %d body=%s", resp.StatusCode, string(body))
	}
	var after string
	if err := st.DB.QueryRow(`SELECT after_state FROM audit_logs WHERE path = '/api/members' ORDER BY id DESC LIMIT 1`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(after, "audit-sensitive@example.com") || strings.Contains(after, "+1-555-444-3333") || strings.Contains(after, "111-22-3333") {
		t.Fatalf("expected PII redaction in audit log after_state, got %s", after)
	}
}

func TestAdminMembersPageAndExportSupportAllClubs(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, err := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`INSERT OR IGNORE INTO clubs (id, name) VALUES (2, 'Club Two')`); err != nil {
		t.Fatal(err)
	}
	cryptoSvc, err := services.NewCryptoService()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertMember(storeMember(1, cryptoSvc.Encrypt("club1@example.com"), cryptoSvc.Encrypt("111"), "Club One Member")); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertMember(storeMember(2, cryptoSvc.Encrypt("club2@example.com"), cryptoSvc.Encrypt("222"), "Club Two Member")); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	pageReq := httptest.NewRequest(http.MethodGet, "/members?club_id=all", nil)
	addAuth(pageReq, auth)
	pageResp, err := app.Test(pageReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	pageBody, _ := io.ReadAll(pageResp.Body)
	if pageResp.StatusCode != 200 || !strings.Contains(string(pageBody), "Club One Member") || !strings.Contains(string(pageBody), "Club Two Member") {
		t.Fatalf("expected all-clubs member listing for admin, got %d body=%s", pageResp.StatusCode, string(pageBody))
	}
	exportReq := httptest.NewRequest(http.MethodGet, "/api/members/export?club_id=all", nil)
	addAuth(exportReq, auth)
	exportResp, err := app.Test(exportReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	exportBody, _ := io.ReadAll(exportResp.Body)
	if exportResp.StatusCode != 200 || !strings.Contains(string(exportBody), "Club One Member") || !strings.Contains(string(exportBody), "Club Two Member") {
		t.Fatalf("expected all-clubs export for admin, got %d body=%s", exportResp.StatusCode, string(exportBody))
	}
}

func TestMemberCustomFieldsEncryptedAtRestAndDecryptedOnExport(t *testing.T) {
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
	form := url.Values{}
	form.Set("full_name", "Encrypted Custom")
	form.Set("email", "enc@example.com")
	form.Set("phone", "123")
	form.Set("join_date", "2026-03-01")
	form.Set("position_title", "Lead")
	form.Set("is_active", "true")
	form.Set("group_name", "A")
	form.Set("custom_fields", `{"note":"sensitive-meta"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/members", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 create member, got %d body=%s", resp.StatusCode, string(body))
	}
	var stored string
	if err := st.DB.QueryRow(`SELECT custom_fields FROM members WHERE full_name = 'Encrypted Custom' ORDER BY id DESC LIMIT 1`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "enc:v1:") || strings.Contains(stored, "sensitive-meta") {
		t.Fatalf("expected encrypted custom_fields at rest, got %s", stored)
	}
	exportReq := httptest.NewRequest(http.MethodGet, "/api/members/export", nil)
	addAuth(exportReq, auth)
	exportResp, err := app.Test(exportReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	exportBody, _ := io.ReadAll(exportResp.Body)
	if exportResp.StatusCode != 200 || !strings.Contains(string(exportBody), "sensitive-meta") || strings.Contains(string(exportBody), "enc:v1:") {
		t.Fatalf("expected decrypted custom_fields in export, got %d body=%s", exportResp.StatusCode, string(exportBody))
	}
}

func TestLegacyPlaintextCustomFieldsAutoMigrated(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	cryptoSvc, err := services.NewCryptoService()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertMember(storeMember(1, cryptoSvc.Encrypt("legacy@example.com"), cryptoSvc.Encrypt("555"), "Legacy Plaintext")); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`UPDATE members SET custom_fields = ? WHERE full_name = 'Legacy Plaintext'`, `{"legacy":"plain"}`); err != nil {
		t.Fatal(err)
	}
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("legacy-org", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "legacy-org", "OrganizerPass123!")
	pageReq := httptest.NewRequest(http.MethodGet, "/members", nil)
	addAuth(pageReq, auth)
	pageResp, err := app.Test(pageReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	pageBody, _ := io.ReadAll(pageResp.Body)
	if pageResp.StatusCode != 200 || !strings.Contains(string(pageBody), "legacy") || !strings.Contains(string(pageBody), "plain") {
		t.Fatalf("expected readable custom_fields on page, got %d body=%s", pageResp.StatusCode, string(pageBody))
	}
	var stored string
	if err := st.DB.QueryRow(`SELECT custom_fields FROM members WHERE full_name = 'Legacy Plaintext'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "enc:v1:") {
		t.Fatalf("expected legacy plaintext custom_fields migration to encrypted marker, got %s", stored)
	}
}
