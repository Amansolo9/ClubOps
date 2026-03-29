package unit_tests

import (
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
