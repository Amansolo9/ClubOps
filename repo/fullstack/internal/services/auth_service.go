package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"clubops_portal/fullstack/internal/models"
	"clubops_portal/fullstack/internal/store"

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
		return "", nil, errors.New("account locked, try later")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		attempts := u.FailedAttempts + 1
		var until *time.Time
		if attempts >= s.maxAttempts {
			lock := now.Add(s.lockDuration)
			until = &lock
			attempts = 0
		}
		_ = s.store.UpdateUserLockState(u.ID, attempts, until)
		return "", nil, errors.New("invalid credentials")
	}
	_ = s.store.UpdateUserLockState(u.ID, 0, nil)
	if u.Role == "admin" && now.Sub(u.PasswordSetAt) > 180*24*time.Hour {
		return "", nil, errors.New("password expired; reset required")
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
	return s.store.UpdatePassword(userID, hash, false)
}

func (s *AuthService) AdminResetPassword(targetUserID int64, tempPassword string) error {
	hash, err := s.HashPassword(tempPassword)
	if err != nil {
		return err
	}
	return s.store.UpdatePassword(targetUserID, hash, true)
}
