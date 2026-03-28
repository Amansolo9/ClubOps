package unit_tests

import (
	"testing"
	"time"

	"clubops_portal/fullstack/internal/models"
	"clubops_portal/fullstack/internal/services"
)

func sqliteTime(v time.Time) string {
	return v.UTC().Format("2006-01-02 15:04:05")
}

func TestFeatureFlagUpsertAndList(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	err := st.UpsertFeatureFlag(models.FeatureFlag{FlagKey: "new_ui", Enabled: true, TargetScope: "global", RolloutPct: 100, UpdatedBy: 1})
	if err != nil {
		t.Fatal(err)
	}
	flags, err := st.ListFeatureFlags()
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != 1 || flags[0].FlagKey != "new_ui" {
		t.Fatalf("expected one feature flag")
	}
}

func TestFeatureFlagRoleAndRolloutEvaluation(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	err := st.UpsertFeatureFlag(models.FeatureFlag{FlagKey: "credit_engine_v2", Enabled: true, TargetScope: "role:organizer", RolloutPct: 0, UpdatedBy: 1})
	if err != nil {
		t.Fatal(err)
	}
	flagSvc := services.NewFlagService(st)
	organizer := &models.User{Username: "org-user", Role: "organizer"}
	if flagSvc.IsEnabledForUser("credit_engine_v2", organizer) {
		t.Fatalf("expected rollout 0 to disable feature")
	}
	err = st.UpsertFeatureFlag(models.FeatureFlag{FlagKey: "credit_engine_v2", Enabled: true, TargetScope: "role:organizer", RolloutPct: 100, UpdatedBy: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !flagSvc.IsEnabledForUser("credit_engine_v2", organizer) {
		t.Fatalf("expected organizer to receive feature at 100%% rollout")
	}
	member := &models.User{Username: "mem-user", Role: "member"}
	if flagSvc.IsEnabledForUser("credit_engine_v2", member) {
		t.Fatalf("expected member outside role target")
	}
}

func TestUndefinedFeatureFlagDefaultsDisabled(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	flagSvc := services.NewFlagService(st)
	if flagSvc.IsEnabledForUser("missing_flag", &models.User{Username: "user1", Role: "member"}) {
		t.Fatalf("expected undefined flag to default disabled")
	}
}

func TestAuditCleanupRemovesExpiredRows(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	_, err := st.DB.Exec(`INSERT INTO audit_logs (method, path, retention_until) VALUES ('POST', '/expired', ?)`, sqliteTime(time.Now().Add(-time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CleanupAuditLogs(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := st.DB.QueryRow(`SELECT COUNT(1) FROM audit_logs WHERE path = '/expired'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected expired audit log cleanup")
	}
}

func TestAuditLogsAreAppendOnlyAtDBLayer(t *testing.T) {
	st := setupStore(t)
	defer st.Close()

	if _, err := st.DB.Exec(`INSERT INTO audit_logs (method, path, retention_until) VALUES ('POST', '/audit-future', ?)`, sqliteTime(time.Now().Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`UPDATE audit_logs SET path = '/tampered' WHERE path = '/audit-future'`); err == nil {
		t.Fatalf("expected UPDATE on audit_logs to be blocked")
	}
	if _, err := st.DB.Exec(`DELETE FROM audit_logs WHERE path = '/audit-future'`); err == nil {
		t.Fatalf("expected DELETE before retention expiry to be blocked")
	}

	if _, err := st.DB.Exec(`INSERT INTO audit_logs (method, path, retention_until) VALUES ('POST', '/audit-expired', ?)`, sqliteTime(time.Now().Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`DELETE FROM audit_logs WHERE path = '/audit-expired'`); err != nil {
		t.Fatalf("expected direct delete of expired audit row to be allowed: %v", err)
	}
	if _, err := st.DB.Exec(`INSERT INTO audit_logs (method, path, retention_until) VALUES ('POST', '/audit-expired', ?)`, sqliteTime(time.Now().Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	if err := st.CleanupAuditLogs(); err != nil {
		t.Fatalf("expected retention cleanup path to delete expired rows: %v", err)
	}
	var remaining int
	if err := st.DB.QueryRow(`SELECT COUNT(1) FROM audit_logs WHERE path = '/audit-expired'`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("expected expired audit row to be removed by retention cleanup")
	}
}
