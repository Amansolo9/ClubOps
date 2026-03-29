package store

import (
	"database/sql"
	"errors"

	"clubops_portal/internal/models"
)

func (s *SQLiteStore) InsertBudget(b models.Budget) (int64, error) {
	res, err := s.DB.Exec(`INSERT INTO budgets (club_id, account_code, campus_code, project_code, period_type, period_start, amount, spent, created_by, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ClubID, b.AccountCode, b.CampusCode, b.ProjectCode, b.PeriodType, b.PeriodStart, b.Amount, b.Spent, b.CreatedBy, b.Status)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) ListBudgets(clubID *int64) ([]models.Budget, error) {
	query := `SELECT id, club_id, account_code, campus_code, project_code, period_type, period_start, amount, spent, threshold_alert, created_by, status, created_at FROM budgets`
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
	out := []models.Budget{}
	for rows.Next() {
		var b models.Budget
		var threshold int
		if err := rows.Scan(&b.ID, &b.ClubID, &b.AccountCode, &b.CampusCode, &b.ProjectCode, &b.PeriodType, &b.PeriodStart, &b.Amount, &b.Spent, &threshold, &b.CreatedBy, &b.Status, &b.CreatedAt); err != nil {
			return nil, err
		}
		b.ThresholdAlert = threshold == 1
		out = append(out, b)
	}
	return out, nil
}

func (s *SQLiteStore) GetBudgetByID(id int64) (*models.Budget, error) {
	var b models.Budget
	var threshold int
	err := s.DB.QueryRow(`SELECT id, club_id, account_code, campus_code, project_code, period_type, period_start, amount, spent, threshold_alert, created_by, status, created_at FROM budgets WHERE id = ?`, id).
		Scan(&b.ID, &b.ClubID, &b.AccountCode, &b.CampusCode, &b.ProjectCode, &b.PeriodType, &b.PeriodStart, &b.Amount, &b.Spent, &threshold, &b.CreatedBy, &b.Status, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	b.ThresholdAlert = threshold == 1
	return &b, nil
}

func (s *SQLiteStore) UpdateBudgetThreshold(id int64, alert bool) error {
	_, err := s.DB.Exec(`UPDATE budgets SET threshold_alert = ? WHERE id = ?`, alert, id)
	return err
}

func (s *SQLiteStore) UpdateBudgetSpent(id int64, spent float64) error {
	_, err := s.DB.Exec(`UPDATE budgets SET spent = ? WHERE id = ?`, spent, id)
	return err
}

func (s *SQLiteStore) InsertBudgetChangeRequest(r models.BudgetChangeRequest) (int64, error) {
	res, err := s.DB.Exec(`INSERT INTO budget_change_requests (budget_id, requested_by, proposed_amount, change_percent, reason, status) VALUES (?, ?, ?, ?, ?, ?)`,
		r.BudgetID, r.RequestedBy, r.ProposedAmount, r.ChangePercent, r.Reason, r.Status)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) ListPendingBudgetChanges(clubID *int64) ([]models.BudgetChangeRequest, error) {
	query := `SELECT bcr.id, bcr.budget_id, bcr.requested_by, bcr.proposed_amount, bcr.change_percent, bcr.reason, bcr.status, bcr.reviewed_by, bcr.created_at
		FROM budget_change_requests bcr
		JOIN budgets b ON b.id = bcr.budget_id
		WHERE bcr.status = 'pending'`
	args := []any{}
	if clubID != nil {
		query += ` AND b.club_id = ?`
		args = append(args, *clubID)
	}
	query += ` ORDER BY bcr.created_at DESC`
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.BudgetChangeRequest{}
	for rows.Next() {
		var it models.BudgetChangeRequest
		var reviewed sql.NullInt64
		if err := rows.Scan(&it.ID, &it.BudgetID, &it.RequestedBy, &it.ProposedAmount, &it.ChangePercent, &it.Reason, &it.Status, &reviewed, &it.CreatedAt); err != nil {
			return nil, err
		}
		if reviewed.Valid {
			it.ReviewedBy = &reviewed.Int64
		}
		out = append(out, it)
	}
	return out, nil
}

func (s *SQLiteStore) GetBudgetChangeRequester(changeID int64) (int64, error) {
	var requesterID int64
	if err := s.DB.QueryRow(`SELECT requested_by FROM budget_change_requests WHERE id = ?`, changeID).Scan(&requesterID); err != nil {
		return 0, err
	}
	return requesterID, nil
}

func (s *SQLiteStore) ApproveBudgetChange(changeID, adminID int64, approve bool) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var budgetID int64
	var amount float64
	if err := tx.QueryRow(`SELECT budget_id, proposed_amount FROM budget_change_requests WHERE id = ? AND status = 'pending'`, changeID).Scan(&budgetID, &amount); err != nil {
		return err
	}
	status := "rejected"
	if approve {
		status = "approved"
		if _, err := tx.Exec(`UPDATE budgets SET amount = ? WHERE id = ?`, amount, budgetID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE budget_change_requests SET status = ?, reviewed_by = ? WHERE id = ?`, status, adminID, changeID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLiteStore) CanAccessBudget(user *models.User, budgetID int64) (bool, error) {
	b, err := s.GetBudgetByID(budgetID)
	if err != nil {
		return false, err
	}
	if user.Role != "admin" {
		if user.Role == "member" {
			return false, nil
		}
		if user.ClubID == nil {
			return false, errors.New("club scope required")
		}
		if user.ClubID != nil && b.ClubID != *user.ClubID {
			return false, nil
		}
	}
	return true, nil
}
