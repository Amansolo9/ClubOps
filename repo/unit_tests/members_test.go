package unit_tests

import (
	"strconv"
	"testing"

	"clubops_portal/internal/models"
)

func TestListMembersPaginationBoundaries(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	for i, name := range []string{"Alex", "Brooke", "Casey"} {
		_, err := st.InsertMember(models.Member{ClubID: 1, FullName: name, EmailEncrypted: "e" + strconv.Itoa(i), PhoneEncrypted: "p" + strconv.Itoa(i), JoinDate: "2026-03-01", PositionTitle: "Member", IsActive: true, GroupName: "A", CustomFields: "{}"})
		if err != nil {
			t.Fatal(err)
		}
	}
	firstPage, err := st.ListMembersPaged(1, "", "", "created_at", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage) != 2 {
		t.Fatalf("expected first page size 2, got %d", len(firstPage))
	}
	lastPage, err := st.ListMembersPaged(1, "", "", "created_at", 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(lastPage) != 1 {
		t.Fatalf("expected last page size 1, got %d", len(lastPage))
	}
	emptyPage, err := st.ListMembersPaged(1, "", "", "created_at", 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(emptyPage) != 0 {
		t.Fatalf("expected empty page for high offset, got %d", len(emptyPage))
	}
}

func TestRecruitingToggleAffectsPublicListing(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	club, err := st.GetClubByID(1)
	if err != nil {
		t.Fatal(err)
	}
	club.RecruitmentOpen = false
	if err := st.UpdateClubProfile(*club); err != nil {
		t.Fatal(err)
	}
	closedList, err := st.ListRecruitingClubs("")
	if err != nil {
		t.Fatal(err)
	}
	if len(closedList) != 0 {
		t.Fatalf("expected no recruiting clubs when recruitment is closed")
	}
	club.RecruitmentOpen = true
	if err := st.UpdateClubProfile(*club); err != nil {
		t.Fatal(err)
	}
	openList, err := st.ListRecruitingClubs("")
	if err != nil {
		t.Fatal(err)
	}
	if len(openList) == 0 {
		t.Fatalf("expected recruiting club to reappear after reopening")
	}
}

func TestListMembersSupportsExtendedSortColumns(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	_, err := st.InsertMember(models.Member{ClubID: 1, FullName: "A", EmailEncrypted: "e1", PhoneEncrypted: "p1", JoinDate: "2026-03-01", PositionTitle: "Coach", IsActive: true, GroupName: "A", CustomFields: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.InsertMember(models.Member{ClubID: 1, FullName: "B", EmailEncrypted: "e2", PhoneEncrypted: "p2", JoinDate: "2026-04-01", PositionTitle: "Analyst", IsActive: false, GroupName: "A", CustomFields: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	for _, sortBy := range []string{"join_date", "position_title", "is_active"} {
		rows, err := st.ListMembersPaged(1, "", "", sortBy, 10, 0)
		if err != nil {
			t.Fatalf("sort %s failed: %v", sortBy, err)
		}
		if len(rows) < 2 {
			t.Fatalf("expected rows for sort %s", sortBy)
		}
	}
}
