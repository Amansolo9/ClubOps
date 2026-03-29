package unit_tests

import (
	"testing"

	"clubops_portal/internal/store"
)

func TestSeedDefaultsRequiresBootstrapPasswordOutsideTestMode(t *testing.T) {
	st, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.AutoMigrate(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_BOOTSTRAP_ADMIN_PASSWORD", "")
	if err := st.SeedDefaults(); err == nil {
		t.Fatalf("expected bootstrap password requirement outside test mode")
	}
}
