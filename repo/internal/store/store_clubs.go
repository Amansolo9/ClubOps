package store

import (
	"strings"

	"clubops_portal/internal/models"
)

func (s *SQLiteStore) InsertClub(club models.Club) (int64, error) {
	res, err := s.DB.Exec(`INSERT INTO clubs (name, tags, avatar_path, recruitment_open, description) VALUES (?, ?, ?, ?, ?)`, club.Name, club.Tags, club.AvatarPath, club.RecruitmentOpen, club.Description)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) ListClubs() ([]models.Club, error) {
	rows, err := s.DB.Query(`SELECT id, name, tags, avatar_path, recruitment_open, description FROM clubs ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Club{}
	for rows.Next() {
		var c models.Club
		var recruitmentOpen int
		if err := rows.Scan(&c.ID, &c.Name, &c.Tags, &c.AvatarPath, &recruitmentOpen, &c.Description); err != nil {
			return nil, err
		}
		c.RecruitmentOpen = recruitmentOpen == 1
		out = append(out, c)
	}
	return out, nil
}

func (s *SQLiteStore) UpdateClubProfile(club models.Club) error {
	_, err := s.DB.Exec(`UPDATE clubs SET name = ?, tags = ?, avatar_path = ?, recruitment_open = ?, description = ? WHERE id = ?`, club.Name, club.Tags, club.AvatarPath, club.RecruitmentOpen, club.Description, club.ID)
	return err
}

func (s *SQLiteStore) GetClubByID(id int64) (*models.Club, error) {
	var c models.Club
	var recruitmentOpen int
	err := s.DB.QueryRow(`SELECT id, name, tags, avatar_path, recruitment_open, description FROM clubs WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &c.Tags, &c.AvatarPath, &recruitmentOpen, &c.Description)
	if err != nil {
		return nil, err
	}
	c.RecruitmentOpen = recruitmentOpen == 1
	return &c, nil
}

func (s *SQLiteStore) ListRecruitingClubs(search string) ([]models.Club, error) {
	query := `SELECT id, name, tags, avatar_path, recruitment_open, description FROM clubs WHERE recruitment_open = 1`
	args := []any{}
	if strings.TrimSpace(search) != "" {
		query += ` AND (lower(name) LIKE ? OR lower(tags) LIKE ?)`
		s := "%" + strings.ToLower(strings.TrimSpace(search)) + "%"
		args = append(args, s, s)
	}
	query += ` ORDER BY name`
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Club{}
	for rows.Next() {
		var c models.Club
		var recruitmentOpen int
		if err := rows.Scan(&c.ID, &c.Name, &c.Tags, &c.AvatarPath, &recruitmentOpen, &c.Description); err != nil {
			return nil, err
		}
		c.RecruitmentOpen = recruitmentOpen == 1
		out = append(out, c)
	}
	return out, nil
}
