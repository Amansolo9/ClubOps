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

	"clubops_portal/fullstack/internal/models"
	"clubops_portal/fullstack/internal/services"
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

func TestReviewCreateDuplicateSubmitIsNonIdempotent(t *testing.T) {
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
		if resp.StatusCode != 200 {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected duplicate submit to succeed as non-idempotent behavior, got %d body=%s", resp.StatusCode, string(respBody))
		}
	}
	var count int
	if err := st.DB.QueryRow(`SELECT COUNT(1) FROM reviews WHERE fulfilled_order_id = ?`, orderID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 reviews after duplicate submit, got %d", count)
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
