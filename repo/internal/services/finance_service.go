package services

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"clubops_portal/internal/models"
	"clubops_portal/internal/store"
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
	if strings.TrimSpace(accountCode) == "" {
		return 0, errors.New("account_code required")
	}
	if strings.TrimSpace(campusCode) == "" {
		return 0, errors.New("campus_code required")
	}
	if strings.TrimSpace(projectCode) == "" {
		return 0, errors.New("project_code required")
	}
	if periodType != "monthly" && periodType != "quarterly" {
		return 0, errors.New("period_type must be monthly or quarterly")
	}
	if err := validateBudgetPeriodStart(periodType, periodStart); err != nil {
		return 0, err
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

func validateBudgetPeriodStart(periodType, periodStart string) error {
	monthlyPattern := regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)
	quarterlyPattern := regexp.MustCompile(`^\d{4}-Q[1-4]$`)
	if periodType == "monthly" && !monthlyPattern.MatchString(periodStart) {
		return errors.New("period_start must be YYYY-MM for monthly")
	}
	if periodType == "quarterly" && !quarterlyPattern.MatchString(periodStart) {
		return errors.New("period_start must be YYYY-Q[1-4] for quarterly")
	}
	return nil
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
		if _, err := s.store.DB.Exec(`UPDATE budgets SET amount = ? WHERE id = ?`, proposedAmount, budgetID); err != nil {
			return 0, err
		}
		return 0, nil
	}
	if requesterRole != "organizer" && requesterRole != "admin" {
		return 0, errors.New("requester must be organizer or admin for >10% changes")
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

func (s *FinanceService) ApproveChange(changeID, reviewerID int64, approve bool, reviewerRole string) error {
	if reviewerRole != "admin" && reviewerRole != "organizer" {
		return errors.New("reviewer must be admin or organizer")
	}
	requesterID, err := s.store.GetBudgetChangeRequester(changeID)
	if err != nil {
		return err
	}
	if requesterID == reviewerID {
		return errors.New("reviewer must differ from requester")
	}
	requester, err := s.store.FindUserByID(requesterID)
	if err != nil {
		return err
	}
	reviewer, err := s.store.FindUserByID(reviewerID)
	if err != nil {
		return err
	}
	// Cross-role: organizer request -> admin reviews; admin request -> organizer reviews
	if requester.Role == "organizer" && reviewer.Role != "admin" {
		return errors.New("organizer requests must be reviewed by admin")
	}
	if requester.Role == "admin" && reviewer.Role != "organizer" {
		return errors.New("admin requests must be reviewed by organizer")
	}
	return s.store.ApproveBudgetChange(changeID, reviewerID, approve)
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
