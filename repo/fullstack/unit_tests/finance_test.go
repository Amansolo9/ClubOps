package unit_tests

import (
	"testing"
	"time"

	"clubops_portal/fullstack/internal/services"
)

func TestBudgetApprovalWorkflowOverTenPercent(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	finance := services.NewFinanceService(st)
	auth := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, _ := auth.HashPassword("OrganizerPass123!")
	if err := st.CreateUser("organizer-approve", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	organizer, err := st.FindUserByUsername("organizer-approve")
	if err != nil {
		t.Fatal(err)
	}
	bID, err := finance.CreateBudget(1, "acct-1", "camp-1", "proj-1", "monthly", "2026-03", 1000, 1)
	if err != nil {
		t.Fatal(err)
	}
	changeID, err := finance.RequestBudgetChange(bID, organizer.ID, 1200, "expansion", "organizer", organizer.ClubID)
	if err != nil {
		t.Fatal(err)
	}
	if changeID == 0 {
		t.Fatalf("expected approval request for >10%% change")
	}
	if err := finance.ApproveChange(changeID, 1, true); err != nil {
		t.Fatal(err)
	}
	b, err := st.GetBudgetByID(bID)
	if err != nil {
		t.Fatal(err)
	}
	if b.Amount != 1200 {
		t.Fatalf("expected updated amount, got %v", b.Amount)
	}
}

func TestBudgetScopeDeniedForTeamLead(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	finance := services.NewFinanceService(st)
	bID, err := finance.CreateBudget(1, "acct-1", "camp-1", "proj-1", "monthly", "2026-03", 1000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := finance.RequestBudgetChange(bID, 2, 1200, "scope violation", "team_lead", int64Ptr(2)); err == nil {
		t.Fatalf("expected team lead scope rejection")
	}
}

func TestBudgetChangeUnderTenPercentUpdatesDirectly(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	finance := services.NewFinanceService(st)
	bID, err := finance.CreateBudget(1, "acct-1", "camp-1", "proj-1", "monthly", "2026-03", 1000, 1)
	if err != nil {
		t.Fatal(err)
	}
	changeID, err := finance.RequestBudgetChange(bID, 1, 1080, "minor tweak", "admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	if changeID != 0 {
		t.Fatalf("expected direct update with no request id")
	}
	b, err := st.GetBudgetByID(bID)
	if err != nil {
		t.Fatal(err)
	}
	if b.Amount != 1080 {
		t.Fatalf("expected direct amount update, got %v", b.Amount)
	}
}

func TestBudgetChangeOverTenPercentRequiresOrganizerRequester(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	finance := services.NewFinanceService(st)
	bID, err := finance.CreateBudget(1, "acct-1", "camp-1", "proj-1", "monthly", "2026-03", 1000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := finance.RequestBudgetChange(bID, 1, 1200, "too large", "admin", nil); err == nil {
		t.Fatalf("expected >10%% change to require organizer requester")
	}
}

func TestBudgetChangeRejectionKeepsAmount(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	finance := services.NewFinanceService(st)
	auth := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	hash, _ := auth.HashPassword("OrganizerPass123!")
	if err := st.CreateUser("organizer-reject", hash, "organizer", int64Ptr(1)); err != nil {
		t.Fatal(err)
	}
	organizer, err := st.FindUserByUsername("organizer-reject")
	if err != nil {
		t.Fatal(err)
	}
	bID, err := finance.CreateBudget(1, "acct-1", "camp-1", "proj-1", "monthly", "2026-03", 1000, 1)
	if err != nil {
		t.Fatal(err)
	}
	changeID, err := finance.RequestBudgetChange(bID, organizer.ID, 1200, "expansion", "organizer", organizer.ClubID)
	if err != nil {
		t.Fatal(err)
	}
	if err := finance.ApproveChange(changeID, 1, false); err != nil {
		t.Fatal(err)
	}
	b, err := st.GetBudgetByID(bID)
	if err != nil {
		t.Fatal(err)
	}
	if b.Amount != 1000 {
		t.Fatalf("expected original amount to remain after rejection")
	}
}

func TestBudgetThresholdAlertToggle(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	finance := services.NewFinanceService(st)
	bID, err := finance.CreateBudget(1, "acct-1", "camp-1", "proj-1", "monthly", "2026-03", 1000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`UPDATE budgets SET spent = 900 WHERE id = ?`, bID); err != nil {
		t.Fatal(err)
	}
	if err := finance.RefreshThresholdAlerts(); err != nil {
		t.Fatal(err)
	}
	b, err := st.GetBudgetByID(bID)
	if err != nil {
		t.Fatal(err)
	}
	if !b.ThresholdAlert {
		t.Fatalf("expected threshold alert to be set")
	}
	if _, err := st.DB.Exec(`UPDATE budgets SET spent = 400 WHERE id = ?`, bID); err != nil {
		t.Fatal(err)
	}
	if err := finance.RefreshThresholdAlerts(); err != nil {
		t.Fatal(err)
	}
	b, err = st.GetBudgetByID(bID)
	if err != nil {
		t.Fatal(err)
	}
	if b.ThresholdAlert {
		t.Fatalf("expected threshold alert to clear")
	}
	var workerAuditCount int
	if err := st.DB.QueryRow(`SELECT COUNT(1) FROM audit_logs WHERE method = 'WORKER' AND path = '/workers/budget-thresholds' AND entity = 'budgets' AND entity_id = ?`, bID).Scan(&workerAuditCount); err != nil {
		t.Fatal(err)
	}
	if workerAuditCount < 2 {
		t.Fatalf("expected worker threshold updates to be audited")
	}
}
