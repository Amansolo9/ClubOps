package store

import "clubops_portal/internal/models"

func (s *SQLiteStore) UpsertFeatureFlag(flag models.FeatureFlag) error {
	_, err := s.DB.Exec(`INSERT INTO feature_flags (flag_key, enabled, target_scope, rollout_pct, updated_by) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(flag_key) DO UPDATE SET enabled = excluded.enabled, target_scope = excluded.target_scope, rollout_pct = excluded.rollout_pct, updated_by = excluded.updated_by, created_at = CURRENT_TIMESTAMP`,
		flag.FlagKey, flag.Enabled, flag.TargetScope, flag.RolloutPct, flag.UpdatedBy)
	return err
}

func (s *SQLiteStore) ListFeatureFlags() ([]models.FeatureFlag, error) {
	rows, err := s.DB.Query(`SELECT id, flag_key, enabled, target_scope, rollout_pct, updated_by, created_at FROM feature_flags ORDER BY flag_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.FeatureFlag{}
	for rows.Next() {
		var f models.FeatureFlag
		var enabled int
		if err := rows.Scan(&f.ID, &f.FlagKey, &enabled, &f.TargetScope, &f.RolloutPct, &f.UpdatedBy, &f.CreatedAt); err != nil {
			return nil, err
		}
		f.Enabled = enabled == 1
		out = append(out, f)
	}
	return out, nil
}

func (s *SQLiteStore) GetFeatureFlagByKey(flagKey string) (*models.FeatureFlag, error) {
	var f models.FeatureFlag
	var enabled int
	err := s.DB.QueryRow(`SELECT id, flag_key, enabled, target_scope, rollout_pct, updated_by, created_at FROM feature_flags WHERE flag_key = ?`, flagKey).
		Scan(&f.ID, &f.FlagKey, &enabled, &f.TargetScope, &f.RolloutPct, &f.UpdatedBy, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	f.Enabled = enabled == 1
	return &f, nil
}
