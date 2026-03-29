package store

import (
	"encoding/json"
	"fmt"
	"time"
)

func (s *SQLiteStore) InsertAuditLog(userID *int64, method, path, entity, entityID string, before, after any) error {
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	retentionUntil := time.Now().AddDate(2, 0, 0).UTC().Format("2006-01-02 15:04:05")
	_, err := s.DB.Exec(`INSERT INTO audit_logs (user_id, method, path, entity, entity_id, before_state, after_state, retention_until)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, userID, method, path, entity, entityID, string(beforeJSON), string(afterJSON), retentionUntil)
	return err
}

func (s *SQLiteStore) CleanupAuditLogs() error {
	_, err := s.DB.Exec(`DELETE FROM audit_logs WHERE julianday(replace(substr(CAST(retention_until AS TEXT), 1, 19), 'T', ' ')) < julianday('now')`)
	return err
}

func (s *SQLiteStore) FetchEntitySnapshot(entity, id string) (map[string]any, error) {
	if id == "" {
		return nil, nil
	}
	tables := map[string]string{
		"budgets":                "budgets",
		"budget_change_requests": "budget_change_requests",
		"reviews":                "reviews",
		"credit_rule_versions":   "credit_rule_versions",
		"region_versions":        "region_versions",
		"members":                "members",
		"clubs":                  "clubs",
		"flags":                  "feature_flags",
	}
	table, ok := tables[entity]
	if !ok {
		return nil, nil
	}
	query := fmt.Sprintf(`SELECT * FROM %s WHERE id = ?`, table)
	rows, err := s.DB.Query(query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	if !rows.Next() {
		return nil, nil
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for i, c := range cols {
		out[c] = vals[i]
	}
	return out, nil
}
