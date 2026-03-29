package unit_tests

import (
	"os"
	"testing"

	"clubops_portal/internal/store"
)

func setupStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	st, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AutoMigrate(); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("APP_ENCRYPTION_KEY", "test-key-material"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("APP_BCRYPT_COST", "4"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("APP_ENV", "test"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("APP_BOOTSTRAP_ADMIN_PASSWORD", "ChangeMe12345!"); err != nil {
		t.Fatal(err)
	}
	if err := st.SeedDefaults(); err != nil {
		t.Fatal(err)
	}
	return st
}

func int64Ptr(v int64) *int64 { return &v }
