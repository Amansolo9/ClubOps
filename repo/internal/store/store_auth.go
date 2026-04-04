package store

import (
	"database/sql"
	"time"

	"clubops_portal/internal/models"
)

func (s *SQLiteStore) FindUserByUsername(username string) (*models.User, error) {
	q := `SELECT id, username, password_hash, role, club_id, failed_attempts, locked_until, must_change_password, password_set_at, created_at FROM users WHERE username = ?`
	u := &models.User{}
	var clubID sql.NullInt64
	var locked sql.NullTime
	var mustChange int
	if err := s.DB.QueryRow(q, username).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &clubID, &u.FailedAttempts, &locked, &mustChange, &u.PasswordSetAt, &u.CreatedAt); err != nil {
		return nil, err
	}
	u.MustChangePass = mustChange == 1
	if clubID.Valid {
		u.ClubID = &clubID.Int64
	}
	if locked.Valid {
		u.LockedUntil = &locked.Time
	}
	return u, nil
}

func (s *SQLiteStore) FindUserByID(id int64) (*models.User, error) {
	q := `SELECT id, username, password_hash, role, club_id, failed_attempts, locked_until, must_change_password, password_set_at, created_at FROM users WHERE id = ?`
	u := &models.User{}
	var clubID sql.NullInt64
	var locked sql.NullTime
	var mustChange int
	if err := s.DB.QueryRow(q, id).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &clubID, &u.FailedAttempts, &locked, &mustChange, &u.PasswordSetAt, &u.CreatedAt); err != nil {
		return nil, err
	}
	u.MustChangePass = mustChange == 1
	if clubID.Valid {
		u.ClubID = &clubID.Int64
	}
	if locked.Valid {
		u.LockedUntil = &locked.Time
	}
	return u, nil
}

func (s *SQLiteStore) UpdateUserLockState(userID int64, failedAttempts int, lockedUntil *time.Time) error {
	_, err := s.DB.Exec(`UPDATE users SET failed_attempts = ?, locked_until = ? WHERE id = ?`, failedAttempts, lockedUntil, userID)
	return err
}

func (s *SQLiteStore) CreateSession(token string, userID int64, expires time.Time) error {
	_, err := s.DB.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`, token, userID, expires)
	return err
}

func (s *SQLiteStore) RefreshSession(token string, expires time.Time) error {
	_, err := s.DB.Exec(`UPDATE sessions SET expires_at = ? WHERE token = ?`, expires, token)
	return err
}

func (s *SQLiteStore) GetSession(token string) (*models.Session, error) {
	sess := &models.Session{}
	if err := s.DB.QueryRow(`SELECT token, user_id, expires_at, created_at FROM sessions WHERE token = ?`, token).Scan(&sess.Token, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt); err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *SQLiteStore) DeleteSession(token string) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (s *SQLiteStore) CreateUser(username, passwordHash, role string, clubID *int64) error {
	_, err := s.DB.Exec(`INSERT INTO users (username, password_hash, role, club_id, password_set_at, must_change_password) VALUES (?, ?, ?, ?, ?, 0)`, username, passwordHash, role, clubID, time.Now())
	return err
}

func (s *SQLiteStore) UpdatePassword(userID int64, hash string, mustChange bool) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE users SET password_hash = ?, password_set_at = ?, must_change_password = ? WHERE id = ?`, hash, time.Now(), mustChange, userID)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (s *SQLiteStore) SetMustChangePassword(userID int64, mustChange bool) error {
	_, err := s.DB.Exec(`UPDATE users SET must_change_password = ? WHERE id = ?`, mustChange, userID)
	return err
}

func (s *SQLiteStore) ListUsers(clubID *int64) ([]models.User, error) {
	query := `SELECT id, username, password_hash, role, club_id, failed_attempts, locked_until, must_change_password, password_set_at, created_at FROM users`
	args := []any{}
	if clubID != nil {
		query += ` WHERE club_id = ?`
		args = append(args, *clubID)
	}
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []models.User{}
	for rows.Next() {
		var u models.User
		var club sql.NullInt64
		var locked sql.NullTime
		var mustChange int
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &club, &u.FailedAttempts, &locked, &mustChange, &u.PasswordSetAt, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.MustChangePass = mustChange == 1
		if club.Valid {
			u.ClubID = &club.Int64
		}
		if locked.Valid {
			u.LockedUntil = &locked.Time
		}
		list = append(list, u)
	}
	return list, nil
}

func (s *SQLiteStore) UpdateUserRoleAndClub(userID int64, role string, clubID *int64) error {
	_, err := s.DB.Exec(`UPDATE users SET role = ?, club_id = ? WHERE id = ?`, role, clubID, userID)
	return err
}
