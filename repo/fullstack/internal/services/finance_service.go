package services

import (
	"errors"
	"math"
	"strconv"
	"time"

	"clubops_portal/fullstack/internal/models"
	"clubops_portal/fullstack/internal/store"
)

type FinanceService struct {
	store *store.SQLiteStore
}

func NewFinanceService(st *store.SQLiteStore) *FinanceService {
	return &FinanceService{store: st}
}

func (s *FinanceService) CreateBudget(clubID int64, accountCode, campusCode, projectCode, periodType, periodStart string, amount float64, userID int64) (int64, error) {
	if amount <= 0 {
		return 0, errors.New("amount must be positive")
	}
	if periodType != "monthly" && periodType != "quarterly" {
		return 0, errors.New("period_type must be monthly or quarterly")
	}
	return s.store.InsertBudget(models.Budget{
		ClubID:      clubID,
		AccountCode: accountCode,
		CampusCode:  campusCode,
		ProjectCode: projectCode,
		PeriodType:  periodType,
		PeriodStart: periodStart,
		Amount:      amount,
		Spent:       0,
		CreatedBy:   userID,
		Status:      "active",
	})
}

func (s *FinanceService) RequestBudgetChange(budgetID, requesterID int64, proposedAmount float64, reason string, requesterRole string, requesterClubID *int64) (int64, error) {
	b, err := s.store.GetBudgetByID(budgetID)
	if err != nil {
		return 0, err
	}
	if requesterRole == "team_lead" {
		if requesterClubID == nil {
			return 0, errors.New("team lead missing club assignment")
		}
		if *requesterClubID != b.ClubID {
			return 0, errors.New("forbidden by club scope")
		}
	}
	if requesterRole != "admin" && requesterRole != "organizer" && requesterRole != "team_lead" {
		return 0, errors.New("insufficient role")
	}
	if b.Amount == 0 {
		return 0, errors.New("budget amount cannot be zero")
	}
	if proposedAmount <= 0 {
		return 0, errors.New("proposed amount must be positive")
	}
	changePct := math.Abs((proposedAmount - b.Amount) / b.Amount * 100)
	if changePct <= 10 {
		if requesterRole != "admin" && requesterRole != "organizer" {
			return 0, errors.New("insufficient role")
		}
		if _, err := s.store.DB.Exec(`UPDATE budgets SET amount = ? WHERE id = ?`, proposedAmount, budgetID); err != nil {
			return 0, err
		}
		return 0, nil
	}
	if requesterRole != "organizer" {
		return 0, errors.New(">10% change requests must be submitted by organizer")
	}
	return s.store.InsertBudgetChangeRequest(models.BudgetChangeRequest{
		BudgetID:       budgetID,
		RequestedBy:    requesterID,
		ProposedAmount: proposedAmount,
		ChangePercent:  changePct,
		Reason:         reason,
		Status:         "pending",
	})
}

func (s *FinanceService) ListBudgets(clubID *int64) ([]models.Budget, error) {
	return s.store.ListBudgets(clubID)
}

func (s *FinanceService) ListPendingChanges(clubID *int64) ([]models.BudgetChangeRequest, error) {
	return s.store.ListPendingBudgetChanges(clubID)
}

func (s *FinanceService) RecordSpend(budgetID int64, spent float64) error {
	if spent < 0 {
		return errors.New("spent must be non-negative")
	}
	budget, err := s.store.GetBudgetByID(budgetID)
	if err != nil {
		return err
	}
	if err := s.store.UpdateBudgetSpent(budgetID, spent); err != nil {
		return err
	}
	alert := budget.Amount > 0 && (spent/budget.Amount) >= 0.85
	return s.store.UpdateBudgetThreshold(budgetID, alert)
}

func (s *FinanceService) ApproveChange(changeID, adminID int64, approve bool) error {
	requesterID, err := s.store.GetBudgetChangeRequester(changeID)
	if err != nil {
		return err
	}
	if requesterID == adminID {
		return errors.New("reviewer must differ from requester")
	}
	return s.store.ApproveBudgetChange(changeID, adminID, approve)
}

func (s *FinanceService) StartThresholdWorker(every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for range ticker.C {
		_ = s.RefreshThresholdAlerts()
	}
}

func (s *FinanceService) RefreshThresholdAlerts() error {
	budgets, err := s.store.ListBudgets(nil)
	if err != nil {
		return err
	}
	for _, b := range budgets {
		alert := b.Amount > 0 && (b.Spent/b.Amount) >= 0.85
		if alert != b.ThresholdAlert {
			if err := s.store.UpdateBudgetThreshold(b.ID, alert); err != nil {
				return err
			}
			before := map[string]any{"threshold_alert": b.ThresholdAlert}
			after := map[string]any{"threshold_alert": alert}
			_ = s.store.InsertAuditLog(nil, "WORKER", "/workers/budget-thresholds", "budgets", strconv.FormatInt(b.ID, 10), before, after)
		}
	}
	return nil
}

func (s *FinanceService) Projection(b models.Budget, expectedRemainingSpend float64) float64 {
	return b.Amount - (b.Spent + expectedRemainingSpend)
}
