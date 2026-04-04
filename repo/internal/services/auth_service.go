package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"clubops_portal/internal/models"
	"clubops_portal/internal/store"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	store        *store.SQLiteStore
	sessionTTL   time.Duration
	maxAttempts  int
	lockDuration time.Duration
	bcryptCost   int
	nowFn        func() time.Time
}

func NewAuthService(st *store.SQLiteStore, sessionTTL time.Duration, maxAttempts int, lockDuration time.Duration) *AuthService {
	cost := 12
	if raw := strings.TrimSpace(os.Getenv("APP_BCRYPT_COST")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= bcrypt.MinCost && parsed <= bcrypt.MaxCost {
			cost = parsed
		}
	}
	return &AuthService{store: st, sessionTTL: sessionTTL, maxAttempts: maxAttempts, lockDuration: lockDuration, bcryptCost: cost, nowFn: time.Now}
}

func (s *AuthService) SetNowFunc(nowFn func() time.Time) {
	if nowFn == nil {
		s.nowFn = time.Now
		return
	}
	s.nowFn = nowFn
}

func (s *AuthService) now() time.Time {
	if s.nowFn == nil {
		return time.Now()
	}
	return s.nowFn()
}

func (s *AuthService) HashPassword(raw string) (string, error) {
	if len(strings.TrimSpace(raw)) < 12 {
		return "", errors.New("password must be at least 12 characters")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(raw), s.bcryptCost)
	return string(b), err
}

func (s *AuthService) Login(username, password string) (string, *models.User, error) {
	u, err := s.store.FindUserByUsername(username)
	if err != nil {
		return "", nil, errors.New("invalid credentials")
	}
	now := s.now()
	if u.LockedUntil != nil && u.LockedUntil.After(now) {
		log.Printf("auth_login_locked username=%s user_id=%d locked_until=%s", u.Username, u.ID, u.LockedUntil.UTC().Format(time.RFC3339))
		return "", nil, errors.New("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		before := map[string]any{"failed_attempts": u.FailedAttempts, "locked_until": lockUntilForAudit(u.LockedUntil)}
		attempts := u.FailedAttempts + 1
		var until *time.Time
		if attempts >= s.maxAttempts {
			lock := now.Add(s.lockDuration)
			until = &lock
			attempts = 0
		}
		_ = s.store.UpdateUserLockState(u.ID, attempts, until)
		after := map[string]any{"failed_attempts": attempts, "locked_until": lockUntilForAudit(until)}
		s.auditUserTransition(u.ID, "POST", "/auth/login-lock-state", before, after)
		return "", nil, errors.New("invalid credentials")
	}
	if u.FailedAttempts != 0 || u.LockedUntil != nil {
		before := map[string]any{"failed_attempts": u.FailedAttempts, "locked_until": lockUntilForAudit(u.LockedUntil)}
		after := map[string]any{"failed_attempts": 0, "locked_until": nil}
		s.auditUserTransition(u.ID, "POST", "/auth/login-unlock-state", before, after)
	}
	_ = s.store.UpdateUserLockState(u.ID, 0, nil)
	if u.Role == "admin" && now.Sub(u.PasswordSetAt) > 180*24*time.Hour {
		before := map[string]any{"must_change_password": u.MustChangePass}
		_ = s.store.SetMustChangePassword(u.ID, true)
		u.MustChangePass = true
		after := map[string]any{"must_change_password": true}
		s.auditUserTransition(u.ID, "POST", "/auth/must-change-password", before, after)
	}
	token, err := randomToken(32)
	if err != nil {
		return "", nil, err
	}
	expires := now.Add(s.sessionTTL)
	if err := s.store.CreateSession(token, u.ID, expires); err != nil {
		return "", nil, err
	}
	return token, u, nil
}

func (s *AuthService) CurrentUserByToken(token string) (*models.User, error) {
	sess, err := s.store.GetSession(token)
	if err != nil {
		return nil, err
	}
	now := s.now()
	if now.After(sess.ExpiresAt) {
		_ = s.store.DeleteSession(token)
		return nil, errors.New("session expired")
	}
	_ = s.store.RefreshSession(token, now.Add(s.sessionTTL))
	return s.store.FindUserByID(sess.UserID)
}

func (s *AuthService) Logout(token string) error {
	return s.store.DeleteSession(token)
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *AuthService) Register(username, password, role string, clubID *int64) error {
	hash, err := s.HashPassword(password)
	if err != nil {
		return err
	}
	return s.store.CreateUser(username, hash, role, clubID)
}

func (s *AuthService) ChangePassword(userID int64, newPassword string) error {
	hash, err := s.HashPassword(newPassword)
	if err != nil {
		return err
	}
	before, _ := s.store.FindUserByID(userID)
	if err := s.store.UpdatePassword(userID, hash, false); err != nil {
		return err
	}
	beforeState := map[string]any{}
	if before != nil {
		beforeState["must_change_password"] = before.MustChangePass
	}
	afterState := map[string]any{"must_change_password": false, "sessions_revoked": true}
	s.auditUserTransition(userID, "POST", "/auth/change-password", beforeState, afterState)
	return nil
}

func (s *AuthService) AdminResetPassword(targetUserID int64, tempPassword string) error {
	hash, err := s.HashPassword(tempPassword)
	if err != nil {
		return err
	}
	before, _ := s.store.FindUserByID(targetUserID)
	if err := s.store.UpdatePassword(targetUserID, hash, true); err != nil {
		return err
	}
	beforeState := map[string]any{}
	if before != nil {
		beforeState["must_change_password"] = before.MustChangePass
	}
	afterState := map[string]any{"must_change_password": true, "sessions_revoked": true}
	s.auditUserTransition(targetUserID, "POST", "/auth/admin-reset-password", beforeState, afterState)
	return nil
}

func (s *AuthService) auditUserTransition(userID int64, method, path string, before, after map[string]any) {
	entityID := strconv.FormatInt(userID, 10)
	_ = s.store.InsertAuditLog(nil, method, path, "users", entityID, before, after)
}

func lockUntilForAudit(v *time.Time) any {
	if v == nil {
		return nil
	}
	return v.UTC().Format(time.RFC3339)
}
