package unit_tests

import (
	"strings"
	"testing"

	"clubops_portal/internal/services"
)

func TestMDMImportDimensionRejectsBadCode(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	mdm := services.NewMDMService(st)
	_, err := mdm.ImportDimensionCSV(strings.NewReader("bad-code,Value\n"), "product", "v1", 1)
	if err == nil {
		t.Fatalf("expected invalid code rejection")
	}
}

func TestMDMImportDimensionRejectsWrongFixedLengthCode(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	mdm := services.NewMDMService(st)
	for _, tc := range []struct {
		dimension string
		csv       string
	}{{"product", "PROD12,Product Too Long\n"}, {"customer", "CUS,Customer Too Short\n"}, {"channel", "CHANNEL,Channel Too Long\n"}, {"region", "REG11,Region Too Long\n"}, {"time", "T1,Time Too Short\n"}} {
		if _, err := mdm.ImportDimensionCSV(strings.NewReader(tc.csv), tc.dimension, "len-check-"+tc.dimension, 1); err == nil {
			t.Fatalf("expected fixed-length rejection for %s", tc.dimension)
		}
	}
}

func TestMDMSalesFactRequiresKnownDimensionCodes(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	mdm := services.NewMDMService(st)
	for _, tc := range []struct {
		dimension string
		csv       string
	}{{"product", "PROD1,Product One\n"}, {"customer", "CUST1,Customer One\n"}, {"channel", "CHAN1,Channel One\n"}, {"region", "REG1,Region One\n"}, {"time", "TIME1,Time One\n"}} {
		if _, err := mdm.ImportDimensionCSV(strings.NewReader(tc.csv), tc.dimension, "seed-"+tc.dimension, 1); err != nil {
			t.Fatalf("seed %s: %v", tc.dimension, err)
		}
	}
	if _, err := mdm.ImportSalesFactCSV(strings.NewReader("product_code,customer_code,channel_code,region_code,time_code,amount,transaction_date\nUNKNOWN,CUST1,CHAN1,REG1,TIME1,10,2026-03-01\n")); err == nil {
		t.Fatalf("expected referential integrity rejection")
	}
}

func TestSalesFactImportSuccess(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	mdm := services.NewMDMService(st)
	for _, tc := range []struct {
		dimension string
		csv       string
	}{{"product", "PROD1,Product One\n"}, {"customer", "CUST1,Customer One\n"}, {"channel", "CHAN1,Channel One\n"}, {"region", "REG1,Region One\n"}, {"time", "TIME1,Time One\n"}} {
		if _, err := mdm.ImportDimensionCSV(strings.NewReader(tc.csv), tc.dimension, "seed-ok-"+tc.dimension, 1); err != nil {
			t.Fatalf("seed %s: %v", tc.dimension, err)
		}
	}
	count, err := mdm.ImportSalesFactCSV(strings.NewReader("product_code,customer_code,channel_code,region_code,time_code,amount,transaction_date\nPROD1,CUST1,CHAN1,REG1,TIME1,10,2026-03-01\n"))
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one imported sales fact, got %d", count)
	}
}

func TestRegionVersionLifecycle(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	mdm := services.NewMDMService(st)
	versionID, err := mdm.ImportRegionCSV(strings.NewReader("CA,Orange,Irvine\nCA,Los Angeles,Pasadena\n"), "spring-2026", 1)
	if err != nil {
		t.Fatal(err)
	}
	versions, err := mdm.ListRegionVersions()
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) == 0 {
		t.Fatalf("expected region versions")
	}
	version, rows, err := mdm.GetRegionVersion(versionID)
	if err != nil {
		t.Fatal(err)
	}
	if version.Label != "spring-2026" || len(rows) != 2 {
		t.Fatalf("expected imported region version details")
	}
	newID, err := mdm.UpdateRegionVersion(versionID, "spring-2026-rev2", [][3]string{{"CA", "Orange", "Anaheim"}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	version, rows, err = mdm.GetRegionVersion(newID)
	if err != nil {
		t.Fatal(err)
	}
	if version.Label != "spring-2026-rev2" || len(rows) != 1 || rows[0].City != "Anaheim" {
		t.Fatalf("expected updated region version rows")
	}
}
