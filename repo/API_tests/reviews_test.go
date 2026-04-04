package API_tests

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"clubops_portal/internal/models"
	"clubops_portal/internal/services"
)

func TestModerationRouteRejectsTeamLead(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	orderID, err := st.InsertFulfilledOrder(models.FulfilledOrder{ClubID: 1, SiteID: 101, MemberID: 1001, OwnerUserID: 1, ServiceLabel: "Seeded", Status: "fulfilled", FulfilledAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`INSERT INTO reviews (club_id, fulfilled_order_id, site_id, member_id, reviewer_id, stars, tags, comment, image_paths, appeal_status) VALUES (1, ?, 101, 1001, 1, 5, '[]', 'ok', '[]', 'none')`, orderID); err != nil {
		t.Fatal(err)
	}
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("LeadPass12345!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("lead-mod", hash, "team_lead", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "lead-mod", "LeadPass12345!")
	form := url.Values{}
	form.Set("decision", "hide")
	form.Set("reason", "policy")
	req := httptest.NewRequest(http.MethodPost, "/api/reviews/1/moderate", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 403 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 403 moderation denial, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestReviewUploadRejectsInvalidFileType(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("MemberPassword1!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("member-review", hash, "member", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	member, err := st.FindUserByUsername("member-review")
	if err != nil {
		t.Fatal(err)
	}
	orderID, err := st.InsertFulfilledOrder(models.FulfilledOrder{ClubID: 1, SiteID: 111, MemberID: 222, OwnerUserID: member.ID, ServiceLabel: "Photo Review", Status: "fulfilled", FulfilledAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "member-review", "MemberPassword1!")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range map[string]string{"fulfilled_order_id": strconv.FormatInt(orderID, 10), "stars": "5", "tags": "communication", "comment": "ok"} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("images", "proof.gif")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("gif-data")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/reviews", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 422 invalid image type, got %d body=%s", resp.StatusCode, string(respBody))
	}
}

func TestReviewUploadRejectsMismatchedImageContent(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("MemberPassword2!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("member-review-content", hash, "member", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	member, err := st.FindUserByUsername("member-review-content")
	if err != nil {
		t.Fatal(err)
	}
	orderID, err := st.InsertFulfilledOrder(models.FulfilledOrder{ClubID: 1, SiteID: 112, MemberID: 223, OwnerUserID: member.ID, ServiceLabel: "Photo Review", Status: "fulfilled", FulfilledAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "member-review-content", "MemberPassword2!")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range map[string]string{"fulfilled_order_id": strconv.FormatInt(orderID, 10), "stars": "5", "tags": "communication", "comment": "ok"} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("images", "proof.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("not-a-real-image")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/reviews", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 422 invalid image content, got %d body=%s", resp.StatusCode, string(respBody))
	}
}

func TestReviewCreationRequiresOrderOwnership(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("MemberPassword1!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("order-owner", hash, "member", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("other-member", hash, "member", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	owner, err := st.FindUserByUsername("order-owner")
	if err != nil {
		t.Fatal(err)
	}
	orderID, err := st.InsertFulfilledOrder(models.FulfilledOrder{ClubID: 1, SiteID: 301, MemberID: 901, OwnerUserID: owner.ID, ServiceLabel: "Club Visit", Status: "fulfilled", FulfilledAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "other-member", "MemberPassword1!")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range map[string]string{"fulfilled_order_id": strconv.FormatInt(orderID, 10), "stars": "5", "tags": "communication", "comment": "ok"} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/reviews", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 403 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 403 order ownership denial, got %d body=%s", resp.StatusCode, string(respBody))
	}
}

func TestOrganizerWithoutClubCannotCreateReview(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("org-create-review-noscope", hash, "organizer", nil); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org-create-review-noscope", "OrganizerPass123!")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range map[string]string{"fulfilled_order_id": "1", "stars": "5", "tags": "communication", "comment": "ok"} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/reviews", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 403 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 403 no-club organizer review create denial, got %d body=%s", resp.StatusCode, string(respBody))
	}
}

func TestReviewCreateInvalidOrderUsesSchemaError(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("MemberPassword1!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("member-invalid-order", hash, "member", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "member-invalid-order", "MemberPassword1!")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range map[string]string{"fulfilled_order_id": "bad", "stars": "5", "tags": "communication", "comment": "ok"} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/reviews", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400 invalid order id, got %d body=%s", resp.StatusCode, string(respBody))
	}
	respBody, _ := io.ReadAll(resp.Body)
	var payload map[string]any
	if err := json.Unmarshal(respBody, &payload); err != nil {
		t.Fatalf("expected JSON error payload, got body=%s", string(respBody))
	}
	if payload["error_code"] != "validation_error" {
		t.Fatalf("expected validation_error code, got %#v", payload["error_code"])
	}
}

func TestReviewCreateDuplicateSubmitReturnsConflict(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("MemberPassword1!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("member-nonidempotent", hash, "member", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	member, err := st.FindUserByUsername("member-nonidempotent")
	if err != nil {
		t.Fatal(err)
	}
	orderID, err := st.InsertFulfilledOrder(models.FulfilledOrder{ClubID: 1, SiteID: 111, MemberID: 222, OwnerUserID: member.ID, ServiceLabel: "Photo Review", Status: "fulfilled", FulfilledAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "member-nonidempotent", "MemberPassword1!")
	for i := 0; i < 2; i++ {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		for key, value := range map[string]string{"fulfilled_order_id": strconv.FormatInt(orderID, 10), "stars": "5", "tags": "communication", "comment": "ok"} {
			if err := writer.WriteField(key, value); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/reviews", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		addAuth(req, auth)
		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && resp.StatusCode != 200 {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected first submit to succeed, got %d body=%s", resp.StatusCode, string(respBody))
		}
		if i == 1 && resp.StatusCode != 409 {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected duplicate submit conflict, got %d body=%s", resp.StatusCode, string(respBody))
		}
	}
	var count int
	if err := st.DB.QueryRow(`SELECT COUNT(1) FROM reviews WHERE fulfilled_order_id = ?`, orderID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 review after duplicate submit guard, got %d", count)
	}
}

func TestMemberCannotCreateFulfilledOrder(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("MemberPassword1!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("member-orders", hash, "member", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "member-orders", "MemberPassword1!")
	form := url.Values{}
	form.Set("site_id", "301")
	form.Set("member_id", "901")
	form.Set("service_label", "Mentoring Session")
	createReq := httptest.NewRequest(http.MethodPost, "/api/fulfilled-orders", strings.NewReader(form.Encode()))
	createReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(createReq, auth)
	createResp, err := app.Test(createReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if createResp.StatusCode != 403 {
		body, _ := io.ReadAll(createResp.Body)
		t.Fatalf("expected fulfilled order create forbidden for member, got %d body=%s", createResp.StatusCode, string(body))
	}
}

func TestOrganizerCanCreateFulfilledOrderAndSeeOptionList(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("org-orders", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	cryptoSvc, err := services.NewCryptoService()
	if err != nil {
		t.Fatal(err)
	}
	memberID, err := st.InsertMember(storeMember(1, cryptoSvc.Encrypt("ordermember@example.com"), cryptoSvc.Encrypt("123"), "Order Member"))
	if err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org-orders", "OrganizerPass123!")
	form := url.Values{}
	form.Set("site_id", "301")
	form.Set("member_id", strconv.FormatInt(memberID, 10))
	form.Set("service_label", "Mentoring Session")
	createReq := httptest.NewRequest(http.MethodPost, "/api/fulfilled-orders", strings.NewReader(form.Encode()))
	createReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(createReq, auth)
	createResp, err := app.Test(createReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if createResp.StatusCode != 200 {
		body, _ := io.ReadAll(createResp.Body)
		t.Fatalf("expected organizer fulfilled order create success, got %d body=%s", createResp.StatusCode, string(body))
	}
	optionsReq := httptest.NewRequest(http.MethodGet, "/partials/fulfilled-orders/options", nil)
	addAuth(optionsReq, auth)
	optionsResp, err := app.Test(optionsReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	optionsBody, _ := io.ReadAll(optionsResp.Body)
	if optionsResp.StatusCode != 200 || !strings.Contains(string(optionsBody), "Mentoring Session") {
		t.Fatalf("expected fulfilled order options list to include new record, got %d body=%s", optionsResp.StatusCode, string(optionsBody))
	}
}

func TestOrganizerWithoutClubCannotAccessReviewsPartial(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("org-reviews-noscope", hash, "organizer", nil); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org-reviews-noscope", "OrganizerPass123!")
	req := httptest.NewRequest(http.MethodGet, "/partials/reviews/list", nil)
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 403 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 403 reviews partial for organizer without club, got %d body=%s", resp.StatusCode, string(body))
	}
}
