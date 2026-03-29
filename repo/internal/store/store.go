package store

import (
	"database/sql"
	"errors"

	"clubops_portal/internal/models"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	DB *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return nil, err
	}
	return &SQLiteStore{DB: db}, nil
}

func (s *SQLiteStore) Close() error { return s.DB.Close() }

func (s *SQLiteStore) InsertCreditRule(v models.CreditRuleVersion) (int64, error) {
	res, err := s.DB.Exec(`INSERT INTO credit_rule_versions (version_label, formula_json, makeup_enabled, retake_enabled, effective_from, effective_to, created_by, is_active) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		v.VersionLabel, v.FormulaJSON, v.MakeupEnabled, v.RetakeEnabled, v.EffectiveFrom, v.EffectiveTo, v.CreatedBy, v.IsActive)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) GetCreditRuleForDate(txnDate string) (*models.CreditRuleVersion, error) {
	r := &models.CreditRuleVersion{}
	var effectiveTo sql.NullString
	var makeupEnabled int
	var retakeEnabled int
	var isActive int
	err := s.DB.QueryRow(`SELECT id, version_label, formula_json, makeup_enabled, retake_enabled, effective_from, effective_to, created_by, created_at, is_active
		FROM credit_rule_versions
		WHERE is_active = 1
		AND effective_from <= ?
		AND (effective_to IS NULL OR effective_to >= ?)
		ORDER BY effective_from DESC, id DESC LIMIT 1`, txnDate, txnDate).
		Scan(&r.ID, &r.VersionLabel, &r.FormulaJSON, &makeupEnabled, &retakeEnabled, &r.EffectiveFrom, &effectiveTo, &r.CreatedBy, &r.CreatedAt, &isActive)
	if err != nil {
		return nil, err
	}
	r.MakeupEnabled = makeupEnabled == 1
	r.RetakeEnabled = retakeEnabled == 1
	r.IsActive = isActive == 1
	if effectiveTo.Valid {
		r.EffectiveTo = &effectiveTo.String
	}
	return r, nil
}

func (s *SQLiteStore) InsertIssuedCredit(c models.CreditIssued) (int64, error) {
	res, err := s.DB.Exec(`INSERT INTO credit_issued (member_id, rule_version_id, base_score, makeup_used, retake_used, calculated_credit, immutable_hash) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.MemberID, c.RuleVersionID, c.BaseScore, c.MakeupUsed, c.RetakeUsed, c.CalculatedCredit, c.ImmutableHash)
	if err != nil {
		if err.Error() != "" {
			return 0, err
		}
		return 0, errors.New("credit already issued and immutable")
	}
	return res.LastInsertId()
}
