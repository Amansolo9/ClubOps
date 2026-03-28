package unit_tests

import (
	"testing"

	"clubops_portal/fullstack/internal/services"
)

func TestCreditImmutableAfterIssue(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	credit := services.NewCreditService(st)
	_, err := credit.CreateRule("v1", services.CreditFormula{Weight: 1, MakeupBonus: 2, RetakeFactor: 0.9}, true, true, "2026-01-01", nil, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := credit.IssueCredit(22, 80, false, false, "2026-03-01"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := credit.IssueCredit(22, 90, true, false, "2026-03-01"); err == nil {
		t.Fatalf("expected immutable issuance rejection")
	}
}

func TestCreditRuleEffectiveDateSelection(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	credit := services.NewCreditService(st)
	end := "2026-06-30"
	_, err := credit.CreateRule("v1", services.CreditFormula{Weight: 1}, true, true, "2026-01-01", &end, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := credit.IssueCredit(33, 80, false, false, "2026-07-15"); err == nil {
		t.Fatalf("expected no active rule for out-of-range date")
	}
}

func TestCreditThresholdsAndDeductionsApplied(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	credit := services.NewCreditService(st)
	formula := services.CreditFormula{Weight: 1, Thresholds: []services.CreditThreshold{{MinScore: 90, Bonus: 10}}, Deductions: []services.CreditDeduction{{MaxScore: 100, Amount: 5}}}
	_, err := credit.CreateRule("v-threshold", formula, false, true, "2026-01-01", nil, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	_, value, err := credit.IssueCredit(44, 95, false, true, "2026-03-01")
	if err != nil {
		t.Fatal(err)
	}
	if value != 100 {
		t.Fatalf("expected threshold bonus and deduction application, got %v", value)
	}
}

func TestCreditRuleRejectsInvalidRanges(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	credit := services.NewCreditService(st)
	formula := services.CreditFormula{Weight: 1, Thresholds: []services.CreditThreshold{{MinScore: 95, Bonus: 3}}, Deductions: []services.CreditDeduction{{MaxScore: 90, Amount: -1}}}
	if _, err := credit.CreateRule("bad-range", formula, false, false, "2026-01-01", nil, 1, true); err == nil {
		t.Fatalf("expected threshold range validation failure")
	}
}
