package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"clubops_portal/fullstack/internal/models"
	"clubops_portal/fullstack/internal/store"
)

type CreditService struct {
	store *store.SQLiteStore
}

type CreditFormula struct {
	Weight       float64 `json:"weight"`
	MakeupBonus  float64 `json:"makeup_bonus"`
	RetakeFactor float64 `json:"retake_factor"`
	Thresholds   []CreditThreshold `json:"thresholds"`
	Deductions   []CreditDeduction `json:"deductions"`
}

type CreditThreshold struct {
	MinScore float64 `json:"min_score"`
	Bonus    float64 `json:"bonus"`
}

type CreditDeduction struct {
	MaxScore float64 `json:"max_score"`
	Amount   float64 `json:"amount"`
}

func NewCreditService(st *store.SQLiteStore) *CreditService { return &CreditService{store: st} }

func (s *CreditService) CreateRule(version string, formula CreditFormula, makeupEnabled, retakeEnabled bool, effectiveFrom string, effectiveTo *string, createdBy int64, active bool) (int64, error) {
	if err := validateCreditRule(version, formula, effectiveFrom, effectiveTo); err != nil {
		return 0, err
	}
	b, _ := json.Marshal(formula)
	return s.store.InsertCreditRule(models.CreditRuleVersion{
		VersionLabel:  version,
		FormulaJSON:   string(b),
		MakeupEnabled: makeupEnabled,
		RetakeEnabled: retakeEnabled,
		EffectiveFrom: effectiveFrom,
		EffectiveTo:   effectiveTo,
		CreatedBy:     createdBy,
		IsActive:      active,
	})
}

func (s *CreditService) IssueCredit(memberID int64, baseScore float64, makeupUsed bool, retakeUsed bool, txnDate string) (int64, float64, error) {
	if baseScore < 0 {
		return 0, 0, errors.New("base_score cannot be negative")
	}
	rule, err := s.store.GetCreditRuleForDate(txnDate)
	if err != nil {
		return 0, 0, errors.New("no active rule version")
	}
	var f CreditFormula
	if err := json.Unmarshal([]byte(rule.FormulaJSON), &f); err != nil {
		return 0, 0, err
	}
	credit := baseScore * f.Weight
	if makeupUsed {
		if !rule.MakeupEnabled {
			return 0, 0, errors.New("makeup not allowed by active rule")
		}
		credit += f.MakeupBonus
	}
	if retakeUsed {
		if !rule.RetakeEnabled {
			return 0, 0, errors.New("retake not allowed by active rule")
		}
		if f.RetakeFactor == 0 {
			f.RetakeFactor = 1
		}
		credit *= f.RetakeFactor
	}
	for _, th := range f.Thresholds {
		if baseScore >= th.MinScore {
			credit += th.Bonus
		}
	}
	for _, d := range f.Deductions {
		if baseScore <= d.MaxScore {
			credit -= d.Amount
		}
	}
	checksum := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%0.3f:%v:%v", memberID, rule.ID, credit, makeupUsed, retakeUsed)))
	immutable := hex.EncodeToString(checksum[:])
	id, err := s.store.InsertIssuedCredit(models.CreditIssued{
		MemberID:         memberID,
		RuleVersionID:    rule.ID,
		BaseScore:        baseScore,
		MakeupUsed:       makeupUsed,
		RetakeUsed:       retakeUsed,
		CalculatedCredit: credit,
		ImmutableHash:    immutable,
	})
	if err != nil {
		return 0, 0, err
	}
	return id, credit, nil
}

func validateCreditRule(version string, formula CreditFormula, effectiveFrom string, effectiveTo *string) error {
	if version == "" {
		return errors.New("version is required")
	}
	if formula.Weight <= 0 {
		return errors.New("weight must be positive")
	}
	if formula.RetakeFactor < 0 {
		return errors.New("retake_factor cannot be negative")
	}
	fromDate, err := time.Parse("2006-01-02", effectiveFrom)
	if err != nil {
		return errors.New("effective_from must be YYYY-MM-DD")
	}
	if effectiveTo != nil {
		toDate, err := time.Parse("2006-01-02", *effectiveTo)
		if err != nil {
			return errors.New("effective_to must be YYYY-MM-DD")
		}
		if toDate.Before(fromDate) {
			return errors.New("effective_to cannot be before effective_from")
		}
	}
	lastMin := -1.0
	for _, threshold := range formula.Thresholds {
		if threshold.MinScore < 0 {
			return errors.New("threshold min_score cannot be negative")
		}
		if threshold.Bonus < 0 {
			return errors.New("threshold bonus cannot be negative")
		}
		if threshold.MinScore < lastMin {
			return errors.New("thresholds must be sorted by min_score ascending")
		}
		lastMin = threshold.MinScore
	}
	lastMax := -1.0
	for _, deduction := range formula.Deductions {
		if deduction.MaxScore < 0 {
			return errors.New("deduction max_score cannot be negative")
		}
		if deduction.Amount < 0 {
			return errors.New("deduction amount cannot be negative")
		}
		if deduction.MaxScore < lastMax {
			return errors.New("deductions must be sorted by max_score ascending")
		}
		lastMax = deduction.MaxScore
	}
	return nil
}
