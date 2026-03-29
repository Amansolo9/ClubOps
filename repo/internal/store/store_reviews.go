package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"clubops_portal/internal/models"
)

func (s *SQLiteStore) InsertReview(r models.Review) (int64, error) {
	res, err := s.DB.Exec(`INSERT INTO reviews (club_id, fulfilled_order_id, site_id, member_id, reviewer_id, stars, tags, comment, image_paths, appeal_status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ClubID, r.FulfilledOrderID, r.SiteID, r.MemberID, r.ReviewerID, r.Stars, r.Tags, r.Comment, r.ImagePaths, r.AppealStatus)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) ListReviews(clubID *int64) ([]models.Review, error) {
	query := `SELECT id, club_id, fulfilled_order_id, site_id, member_id, reviewer_id, stars, tags, comment, image_paths, appeal_status, hidden_reason, hidden_at, created_at FROM reviews`
	args := []any{}
	if clubID != nil {
		query += ` WHERE club_id = ?`
		args = append(args, *clubID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Review{}
	for rows.Next() {
		var r models.Review
		var hidden sql.NullString
		var hiddenAt sql.NullTime
		var fulfilledOrderID sql.NullInt64
		if err := rows.Scan(&r.ID, &r.ClubID, &fulfilledOrderID, &r.SiteID, &r.MemberID, &r.ReviewerID, &r.Stars, &r.Tags, &r.Comment, &r.ImagePaths, &r.AppealStatus, &hidden, &hiddenAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		if fulfilledOrderID.Valid {
			r.FulfilledOrderID = &fulfilledOrderID.Int64
		}
		if hidden.Valid {
			r.HiddenReason = &hidden.String
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *SQLiteStore) ListReviewsByReviewer(reviewerID int64) ([]models.Review, error) {
	rows, err := s.DB.Query(`SELECT id, club_id, fulfilled_order_id, site_id, member_id, reviewer_id, stars, tags, comment, image_paths, appeal_status, hidden_reason, hidden_at, created_at FROM reviews WHERE reviewer_id = ? ORDER BY created_at DESC`, reviewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Review{}
	for rows.Next() {
		var r models.Review
		var hidden sql.NullString
		var hiddenAt sql.NullTime
		var fulfilledOrderID sql.NullInt64
		if err := rows.Scan(&r.ID, &r.ClubID, &fulfilledOrderID, &r.SiteID, &r.MemberID, &r.ReviewerID, &r.Stars, &r.Tags, &r.Comment, &r.ImagePaths, &r.AppealStatus, &hidden, &hiddenAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		if fulfilledOrderID.Valid {
			r.FulfilledOrderID = &fulfilledOrderID.Int64
		}
		if hidden.Valid {
			r.HiddenReason = &hidden.String
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *SQLiteStore) GetReviewByID(id int64) (*models.Review, error) {
	var r models.Review
	var hidden sql.NullString
	var hiddenAt sql.NullTime
	var fulfilledOrderID sql.NullInt64
	if err := s.DB.QueryRow(`SELECT id, club_id, fulfilled_order_id, site_id, member_id, reviewer_id, stars, tags, comment, image_paths, appeal_status, hidden_reason, hidden_at, created_at FROM reviews WHERE id = ?`, id).
		Scan(&r.ID, &r.ClubID, &fulfilledOrderID, &r.SiteID, &r.MemberID, &r.ReviewerID, &r.Stars, &r.Tags, &r.Comment, &r.ImagePaths, &r.AppealStatus, &hidden, &hiddenAt, &r.CreatedAt); err != nil {
		return nil, err
	}
	if fulfilledOrderID.Valid {
		r.FulfilledOrderID = &fulfilledOrderID.Int64
	}
	if hidden.Valid {
		r.HiddenReason = &hidden.String
	}
	return &r, nil
}

func (s *SQLiteStore) GetFulfilledOrderByID(id int64) (*models.FulfilledOrder, error) {
	var order models.FulfilledOrder
	if err := s.DB.QueryRow(`SELECT id, club_id, site_id, member_id, owner_user_id, service_label, status, fulfilled_at, created_at FROM fulfilled_orders WHERE id = ?`, id).
		Scan(&order.ID, &order.ClubID, &order.SiteID, &order.MemberID, &order.OwnerUserID, &order.ServiceLabel, &order.Status, &order.FulfilledAt, &order.CreatedAt); err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *SQLiteStore) InsertFulfilledOrder(order models.FulfilledOrder) (int64, error) {
	res, err := s.DB.Exec(`INSERT INTO fulfilled_orders (club_id, site_id, member_id, owner_user_id, service_label, status, fulfilled_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, order.ClubID, order.SiteID, order.MemberID, order.OwnerUserID, order.ServiceLabel, order.Status, order.FulfilledAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) ListFulfilledOrders(clubID *int64, ownerUserID *int64, limit int) ([]models.FulfilledOrder, error) {
	query := `SELECT id, club_id, site_id, member_id, owner_user_id, service_label, status, fulfilled_at, created_at FROM fulfilled_orders`
	args := []any{}
	where := ""
	if clubID != nil {
		where = "club_id = ?"
		args = append(args, *clubID)
	}
	if ownerUserID != nil {
		if where != "" {
			where += " AND "
		}
		where += "owner_user_id = ?"
		args = append(args, *ownerUserID)
	}
	if where != "" {
		query += " WHERE " + where
	}
	if limit <= 0 {
		limit = 50
	}
	query += fmt.Sprintf(" ORDER BY fulfilled_at DESC, id DESC LIMIT %d", limit)
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.FulfilledOrder{}
	for rows.Next() {
		var order models.FulfilledOrder
		if err := rows.Scan(&order.ID, &order.ClubID, &order.SiteID, &order.MemberID, &order.OwnerUserID, &order.ServiceLabel, &order.Status, &order.FulfilledAt, &order.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, order)
	}
	return out, nil
}

func (s *SQLiteStore) ReviewExistsForOrder(orderID, reviewerID int64) (bool, error) {
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(1) FROM reviews WHERE fulfilled_order_id = ? AND reviewer_id = ?`, orderID, reviewerID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *SQLiteStore) AppealReview(id int64, reviewerID int64, clubScope *int64) error {
	var createdAt time.Time
	var hiddenAt sql.NullTime
	var owner int64
	var clubID int64
	var status string
	if err := s.DB.QueryRow(`SELECT created_at, hidden_at, reviewer_id, club_id, appeal_status FROM reviews WHERE id = ?`, id).Scan(&createdAt, &hiddenAt, &owner, &clubID, &status); err != nil {
		return err
	}
	if owner != reviewerID {
		return errors.New("only review author can appeal")
	}
	if clubScope != nil && *clubScope != clubID {
		return errors.New("forbidden by club scope")
	}
	if status != "hidden" {
		return errors.New("appeal allowed only for hidden reviews")
	}
	reference := createdAt
	if hiddenAt.Valid {
		reference = hiddenAt.Time
	}
	if time.Since(reference) > 7*24*time.Hour {
		return errors.New("appeal window closed")
	}
	_, err := s.DB.Exec(`UPDATE reviews SET appeal_status='pending' WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) SetReviewModeration(id int64, hidden bool, reason string) error {
	if hidden {
		_, err := s.DB.Exec(`UPDATE reviews SET appeal_status='hidden', hidden_reason = ?, hidden_at = CURRENT_TIMESTAMP WHERE id = ?`, reason, id)
		return err
	}
	_, err := s.DB.Exec(`UPDATE reviews SET appeal_status='visible', hidden_reason = ?, hidden_at = NULL WHERE id = ?`, reason, id)
	return err
}
