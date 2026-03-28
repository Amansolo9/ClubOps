package API_tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"clubops_portal/fullstack/internal/models"
	"clubops_portal/fullstack/internal/services"
)

func TestCreditIssueFeatureFlagGateAtAPI(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("org-credit", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	cryptoSvc, err := services.NewCryptoService()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertMember(storeMember(1, cryptoSvc.Encrypt("credit@example.com"), cryptoSvc.Encrypt("555"), "Credit Member")); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org-credit", "OrganizerPass123!")
	req := httptest.NewRequest(http.MethodPost, "/api/credits/issue", strings.NewReader("member_id=1&base_score=80"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 403 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected feature gate 403, got %d body=%s", resp.StatusCode, string(body))
	}
}

func TestDuplicateCreditIssuanceReturnsConflict(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	if err := st.UpsertFeatureFlag(models.FeatureFlag{FlagKey: "credit_engine_v2", Enabled: true, TargetScope: "role:organizer", RolloutPct: 100, UpdatedBy: 1}); err != nil {
		t.Fatal(err)
	}
	creditSvc := services.NewCreditService(st)
	if _, err := creditSvc.CreateRule("v1", services.CreditFormula{Weight: 1}, true, true, "2026-01-01", nil, 1, true); err != nil {
		t.Fatal(err)
	}
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, err := authSvc.HashPassword("OrganizerPass123!")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser("org-credit-conflict", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	cryptoSvc, err := services.NewCryptoService()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertMember(storeMember(1, cryptoSvc.Encrypt("dupecredit@example.com"), cryptoSvc.Encrypt("777"), "Credit Duplicate")); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "org-credit-conflict", "OrganizerPass123!")
	body := "member_id=1&base_score=80&txn_date=2026-03-01"
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/credits/issue", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		addAuth(req, auth)
		resp, err := app.Test(req, 5000)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && resp.StatusCode != 200 {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected first issue success, got %d body=%s", resp.StatusCode, string(respBody))
		}
		if i == 1 && resp.StatusCode != 409 {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected second issue conflict, got %d body=%s", resp.StatusCode, string(respBody))
		}
	}
}
