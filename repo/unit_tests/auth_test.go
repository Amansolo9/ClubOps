package unit_tests

import (
	"strings"
	"testing"
	"time"

	"clubops_portal/internal/services"
)

func TestAuthLockoutAfterFiveAttempts(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	auth := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, _ := auth.HashPassword("secret123456")
	if err := st.CreateUser("tester", hash, "organizer", nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		_, _, _ = auth.Login("tester", "bad-pass")
	}
	u, err := st.FindUserByUsername("tester")
	if err != nil {
		t.Fatal(err)
	}
	if u.LockedUntil == nil || u.LockedUntil.Before(time.Now()) {
		t.Fatalf("expected account to be locked")
	}
}

func TestAuthLockedLoginReturnsGenericFailure(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	auth := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, _ := auth.HashPassword("secret123456")
	if err := st.CreateUser("locked-generic", hash, "organizer", nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		_, _, _ = auth.Login("locked-generic", "bad-pass")
	}
	if _, _, err := auth.Login("locked-generic", "secret123456"); err == nil {
		t.Fatalf("expected locked account to reject login")
	} else if !strings.EqualFold(strings.TrimSpace(err.Error()), "invalid credentials") {
		t.Fatalf("expected generic invalid credentials message, got %q", err.Error())
	}
}

func TestSessionSlidingRefresh(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	auth := services.NewAuthService(st, 2*time.Second, 5, 15*time.Minute)
	current := time.Date(2026, time.March, 28, 12, 0, 0, 0, time.UTC)
	auth.SetNowFunc(func() time.Time { return current })
	hash, _ := auth.HashPassword("slidepass1234")
	if err := st.CreateUser("slider", hash, "member", nil); err != nil {
		t.Fatal(err)
	}
	token, _, err := auth.Login("slider", "slidepass1234")
	if err != nil {
		t.Fatal(err)
	}
	first, err := st.GetSession(token)
	if err != nil {
		t.Fatal(err)
	}
	current = current.Add(1200 * time.Millisecond)
	if _, err := auth.CurrentUserByToken(token); err != nil {
		t.Fatal(err)
	}
	second, err := st.GetSession(token)
	if err != nil {
		t.Fatal(err)
	}
	if !second.ExpiresAt.After(first.ExpiresAt) {
		t.Fatalf("expected session expiry to be refreshed")
	}
}

func TestPasswordPolicyRequiresMinLength(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	auth := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	if _, err := auth.HashPassword("short"); err == nil {
		t.Fatalf("expected short password rejection")
	}
}

func TestLoginAllowsForcedPasswordChangeWorkflow(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	auth := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, _ := auth.HashPassword("secret123456")
	if err := st.CreateUser("forcepass", hash, "member", nil); err != nil {
		t.Fatal(err)
	}
	user, err := st.FindUserByUsername("forcepass")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdatePassword(user.ID, hash, true); err != nil {
		t.Fatal(err)
	}
	token, current, err := auth.Login("forcepass", "secret123456")
	if err != nil {
		t.Fatalf("expected login session for forced password change, got %v", err)
	}
	if token == "" || current == nil || !current.MustChangePass {
		t.Fatalf("expected must-change session to be returned")
	}
}

func TestPasswordExpiryForcesAdminPasswordChangeSession(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	auth := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, _ := auth.HashPassword("expiredpass12")
	if err := st.CreateUser("expired-admin", hash, "admin", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`UPDATE users SET password_set_at = ? WHERE username = ?`, time.Now().Add(-181*24*time.Hour), "expired-admin"); err != nil {
		t.Fatal(err)
	}
	token, user, err := auth.Login("expired-admin", "expiredpass12")
	if err != nil {
		t.Fatalf("expected expired admin login to return constrained session: %v", err)
	}
	if token == "" || user == nil || !user.MustChangePass {
		t.Fatalf("expected must-change session for expired admin")
	}
	stored, err := st.FindUserByUsername("expired-admin")
	if err != nil {
		t.Fatal(err)
	}
	if !stored.MustChangePass {
		t.Fatalf("expected stored must_change_password to be set")
	}
}

func TestPasswordExpiryDoesNotBlockNonAdminLogin(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	auth := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, _ := auth.HashPassword("memberpass123")
	if err := st.CreateUser("stale-member", hash, "member", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`UPDATE users SET password_set_at = ? WHERE username = ?`, time.Now().Add(-181*24*time.Hour), "stale-member"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := auth.Login("stale-member", "memberpass123"); err != nil {
		t.Fatalf("expected non-admin login to succeed despite stale password_set_at: %v", err)
	}
}

func TestExpiredSessionRejected(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	auth := services.NewAuthService(st, time.Second, 5, 15*time.Minute)
	hash, _ := auth.HashPassword("sessionpass12")
	if err := st.CreateUser("session-expiry", hash, "member", nil); err != nil {
		t.Fatal(err)
	}
	token, _, err := auth.Login("session-expiry", "sessionpass12")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`UPDATE sessions SET expires_at = ? WHERE token = ?`, time.Now().Add(-time.Minute), token); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.CurrentUserByToken(token); err == nil {
		t.Fatalf("expected expired session rejection")
	}
}

func TestPasswordChangeRevokesActiveSessions(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	auth := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, _ := auth.HashPassword("change-pass-123")
	if err := st.CreateUser("change-owner", hash, "member", nil); err != nil {
		t.Fatal(err)
	}
	token, user, err := auth.Login("change-owner", "change-pass-123")
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.ChangePassword(user.ID, "change-pass-456"); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.CurrentUserByToken(token); err == nil {
		t.Fatalf("expected session revocation after password change")
	}
}

func TestAdminResetRevokesTargetSessions(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	auth := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, _ := auth.HashPassword("reset-pass-123")
	if err := st.CreateUser("reset-target", hash, "member", nil); err != nil {
		t.Fatal(err)
	}
	token, user, err := auth.Login("reset-target", "reset-pass-123")
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.AdminResetPassword(user.ID, "temp-reset-789"); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.CurrentUserByToken(token); err == nil {
		t.Fatalf("expected session revocation after admin reset")
	}
}

func TestAuthStateTransitionsAreAudited(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	auth := services.NewAuthService(st, 30*time.Minute, 1, 15*time.Minute)
	hash, _ := auth.HashPassword("audit-pass-123")
	if err := st.CreateUser("audit-user", hash, "member", nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := auth.Login("audit-user", "wrong-pass"); err == nil {
		t.Fatalf("expected invalid credentials")
	}
	if _, _, err := auth.Login("audit-user", "audit-pass-123"); err == nil {
		t.Fatalf("expected lock to still block login")
	}
	u, err := st.FindUserByUsername("audit-user")
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.AdminResetPassword(u.ID, "audit-pass-456"); err != nil {
		t.Fatal(err)
	}
	if err := auth.ChangePassword(u.ID, "audit-pass-789"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/auth/login-lock-state", "/auth/admin-reset-password", "/auth/change-password"} {
		var count int
		if err := st.DB.QueryRow(`SELECT COUNT(1) FROM audit_logs WHERE path = ?`, path).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			t.Fatalf("expected audit log for %s", path)
		}
	}
}
