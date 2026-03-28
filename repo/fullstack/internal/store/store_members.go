package store

import (
	"strings"

	"clubops_portal/fullstack/internal/models"
)

func (s *SQLiteStore) InsertMember(m models.Member) (int64, error) {
	res, err := s.DB.Exec(`INSERT INTO members (club_id, full_name, email_encrypted, phone_encrypted, join_date, position_title, is_active, group_name, custom_fields) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ClubID, m.FullName, m.EmailEncrypted, m.PhoneEncrypted, m.JoinDate, m.PositionTitle, m.IsActive, m.GroupName, m.CustomFields)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) GetMemberByID(id int64) (*models.Member, error) {
	var m models.Member
	var isActive int
	if err := s.DB.QueryRow(`SELECT id, club_id, full_name, email_encrypted, phone_encrypted, join_date, position_title, is_active, group_name, custom_fields, created_at FROM members WHERE id = ?`, id).
		Scan(&m.ID, &m.ClubID, &m.FullName, &m.EmailEncrypted, &m.PhoneEncrypted, &m.JoinDate, &m.PositionTitle, &isActive, &m.GroupName, &m.CustomFields, &m.CreatedAt); err != nil {
		return nil, err
	}
	m.IsActive = isActive == 1
	return &m, nil
}

func (s *SQLiteStore) UpdateMember(m models.Member) error {
	_, err := s.DB.Exec(`UPDATE members SET full_name = ?, email_encrypted = ?, phone_encrypted = ?, join_date = ?, position_title = ?, is_active = ?, group_name = ?, custom_fields = ? WHERE id = ?`,
		m.FullName, m.EmailEncrypted, m.PhoneEncrypted, m.JoinDate, m.PositionTitle, m.IsActive, m.GroupName, m.CustomFields, m.ID)
	return err
}

func (s *SQLiteStore) ListMembers(clubID int64, group, search, sortBy string) ([]models.Member, error) {
	return s.ListMembersPagedScoped(&clubID, group, search, sortBy, 0, 0)
}

func (s *SQLiteStore) ListMembersPaged(clubID int64, group, search, sortBy string, limit, offset int) ([]models.Member, error) {
	return s.ListMembersPagedScoped(&clubID, group, search, sortBy, limit, offset)
}

func (s *SQLiteStore) ListMembersPagedScoped(clubID *int64, group, search, sortBy string, limit, offset int) ([]models.Member, error) {
	query := `SELECT id, club_id, full_name, email_encrypted, phone_encrypted, join_date, position_title, is_active, group_name, custom_fields, created_at FROM members WHERE 1=1`
	args := []any{}
	if clubID != nil {
		query += ` AND club_id = ?`
		args = append(args, *clubID)
	}
	if group != "" {
		query += ` AND group_name = ?`
		args = append(args, group)
	}
	if search != "" {
		query += ` AND lower(full_name) LIKE ?`
		args = append(args, "%"+strings.ToLower(search)+"%")
	}
	orderCol := "created_at"
	sortMap := map[string]string{
		"created_at":     "created_at",
		"full_name":      "full_name",
		"join_date":      "join_date",
		"position_title": "position_title",
		"is_active":      "is_active",
	}
	if mapped, ok := sortMap[sortBy]; ok {
		orderCol = mapped
	}
	query += ` ORDER BY ` + orderCol + ` DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
		if offset > 0 {
			query += ` OFFSET ?`
			args = append(args, offset)
		}
	}
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Member{}
	for rows.Next() {
		var m models.Member
		var isActive int
		if err := rows.Scan(&m.ID, &m.ClubID, &m.FullName, &m.EmailEncrypted, &m.PhoneEncrypted, &m.JoinDate, &m.PositionTitle, &isActive, &m.GroupName, &m.CustomFields, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.IsActive = isActive == 1
		out = append(out, m)
	}
	return out, nil
}
