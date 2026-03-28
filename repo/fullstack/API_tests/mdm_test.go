package API_tests

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"clubops_portal/fullstack/internal/services"
)

func TestRegionVersionAPIGetAndUpdate(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	mdm := services.NewMDMService(st)
	versionID, err := mdm.ImportRegionCSV(strings.NewReader("CA,Orange,Irvine\n"), "spring-2026", 1)
	if err != nil {
		t.Fatal(err)
	}
	adminHash, _ := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	getReq := httptest.NewRequest(http.MethodGet, "/api/regions/"+strconv.FormatInt(versionID, 10), nil)
	addAuth(getReq, auth)
	getResp, err := app.Test(getReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if getResp.StatusCode != 200 {
		body, _ := io.ReadAll(getResp.Body)
		t.Fatalf("expected 200 region get, got %d body=%s", getResp.StatusCode, string(body))
	}
	form := url.Values{}
	form.Set("version_label", "spring-2026-r2")
	form.Set("rows_csv", "CA,Orange,Anaheim")
	postReq := httptest.NewRequest(http.MethodPost, "/api/regions/"+strconv.FormatInt(versionID, 10), strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuth(postReq, auth)
	postResp, err := app.Test(postReq, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if postResp.StatusCode != 200 {
		body, _ := io.ReadAll(postResp.Body)
		t.Fatalf("expected 200 region update, got %d body=%s", postResp.StatusCode, string(body))
	}
}

func TestMDMImportRegionsMissingFileUsesSchemaError(t *testing.T) {
	app, st := setupApp(t)
	defer st.Close()
	adminHash, _ := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute).HashPassword("StrongAdmin123!")
	if err := st.UpdatePassword(1, adminHash, false); err != nil {
		t.Fatal(err)
	}
	auth := login(t, app, "admin", "StrongAdmin123!")
	req := httptest.NewRequest(http.MethodPost, "/api/regions/import", nil)
	addAuth(req, auth)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400 missing file, got %d body=%s", resp.StatusCode, string(body))
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
