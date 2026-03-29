package store

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (s *SQLiteStore) SeedDefaults() error {
	if _, err := s.DB.Exec(`INSERT OR IGNORE INTO clubs (id, name, tags, avatar_path, recruitment_open, description) VALUES (1, 'Default Club', '[]', '', 1, '')`); err != nil {
		return err
	}

	var existing int
	if err := s.DB.QueryRow(`SELECT COUNT(1) FROM users WHERE username = 'admin'`).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}

	bootstrapPassword := strings.TrimSpace(os.Getenv("APP_BOOTSTRAP_ADMIN_PASSWORD"))
	if bootstrapPassword == "" && strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "test") {
		bootstrapPassword = "ChangeMe12345!"
	}
	if bootstrapPassword == "" {
		return errors.New("APP_BOOTSTRAP_ADMIN_PASSWORD is required")
	}

	cost := 12
	if raw := strings.TrimSpace(os.Getenv("APP_BCRYPT_COST")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= bcrypt.MinCost && parsed <= bcrypt.MaxCost {
			cost = parsed
		}
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(bootstrapPassword), cost)
	if err != nil {
		return err
	}

	_, err = s.DB.Exec(`INSERT INTO users (username, password_hash, role, club_id, must_change_password, password_set_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"admin", string(hashBytes), "admin", nil, 1, time.Now())
	return err
}
