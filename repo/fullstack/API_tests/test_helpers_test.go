package API_tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"clubops_portal/fullstack/internal/handlers"
	"clubops_portal/fullstack/internal/middleware"
	"clubops_portal/fullstack/internal/models"
	"clubops_portal/fullstack/internal/services"
	"clubops_portal/fullstack/internal/store"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
)

func setupApp(t *testing.T) (*fiber.App, *store.SQLiteStore) {
	t.Helper()
	st, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AutoMigrate(); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("APP_ENCRYPTION_KEY", "test-encryption-key"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("APP_BCRYPT_COST", "4"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("APP_ENV", "test"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("APP_BOOTSTRAP_ADMIN_PASSWORD", "ChangeMe12345!"); err != nil {
		t.Fatal(err)
	}
	if err := st.SeedDefaults(); err != nil {
		t.Fatal(err)
	}
	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	financeSvc := services.NewFinanceService(st)
	creditSvc := services.NewCreditService(st)
	reviewSvc := services.NewReviewService(st, "../static/uploads")
	mdmSvc := services.NewMDMService(st)
	auditSvc := services.NewAuditService(st)
	cryptoSvc, err := services.NewCryptoService()
	if err != nil {
		t.Fatal(err)
	}

	engine := html.New("../views", ".html")
	app := fiber.New(fiber.Config{Views: engine})
	app.Use(middleware.CSRFProtection())
	app.Use(middleware.AttachCurrentUser(authSvc))
	app.Use(middleware.AuditTrail(auditSvc, st))
	flagSvc := services.NewFlagService(st)
	h := handlers.NewHandler(st, authSvc, financeSvc, creditSvc, reviewSvc, mdmSvc, cryptoSvc, flagSvc)
	h.RegisterRoutes(app)
	return app, st
}

type authCookies struct {
	Session string
	CSRF    string
}

func login(t *testing.T, app *fiber.App, username, password string) authCookies {
	t.Helper()
	form := url.Values{}
	form.Set("username", username)
	form.Set("password", password)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 302 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 302 on login, got %d body=%s", resp.StatusCode, string(body))
	}
	auth := authCookies{}
	for _, c := range resp.Cookies() {
		if c.Name == "session_token" {
			auth.Session = c.Value
		}
		if c.Name == "csrf_token" {
			auth.CSRF = c.Value
		}
	}
	if auth.Session == "" {
		t.Fatal("expected session cookie")
	}
	if auth.CSRF == "" {
		t.Fatal("expected csrf cookie")
	}
	return auth
}

func addAuth(req *http.Request, auth authCookies) {
	req.AddCookie(&http.Cookie{Name: "session_token", Value: auth.Session})
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: auth.CSRF})
	req.Header.Set("X-CSRF-Token", auth.CSRF)
}

func storeMember(clubID int64, emailEncrypted, phoneEncrypted, fullName string) models.Member {
	return models.Member{ClubID: clubID, FullName: fullName, EmailEncrypted: emailEncrypted, PhoneEncrypted: phoneEncrypted, JoinDate: "2026-03-01", PositionTitle: "Captain", IsActive: true, GroupName: "Alpha", CustomFields: "{}"}
}

func int64Ptr(v int64) *int64 { return &v }
