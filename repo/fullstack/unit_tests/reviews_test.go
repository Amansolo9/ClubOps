package unit_tests

import (
	"mime/multipart"
	"testing"
	"time"

	"clubops_portal/fullstack/internal/services"
)

func TestReviewValidationAndAppealWindow(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	reviewSvc := services.NewReviewService(st, t.TempDir())
	if _, err := reviewSvc.CreateReviewScoped(1, 1, 1, 10, 6, []string{"communication"}, "great", nil); err == nil {
		t.Fatalf("expected stars validation failure")
	}
	longComment := make([]byte, 501)
	for i := range longComment {
		longComment[i] = 'a'
	}
	if _, err := reviewSvc.CreateReviewScoped(1, 1, 1, 10, 4, []string{"communication"}, string(longComment), nil); err == nil {
		t.Fatalf("expected comment length validation failure")
	}
	id, err := reviewSvc.CreateReviewScoped(1, 1, 1, 10, 4, []string{"communication"}, "ok", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`UPDATE reviews SET created_at = ? WHERE id = ?`, time.Now().AddDate(0, 0, -8), id); err != nil {
		t.Fatal(err)
	}
	if err := reviewSvc.AppealReview(id, 10, nil); err == nil {
		t.Fatalf("expected appeal window validation")
	}
}

func TestReviewAppealOwnershipEnforced(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	reviewSvc := services.NewReviewService(st, t.TempDir())
	id, err := reviewSvc.CreateReviewScoped(1, 1, 1, 77, 4, []string{"attendance"}, "ok", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := reviewSvc.ModerateReview(id, true, "policy"); err != nil {
		t.Fatal(err)
	}
	if err := reviewSvc.AppealReview(id, 12, nil); err == nil {
		t.Fatalf("expected owner-only appeal rejection")
	}
}

func TestAppealRequiresHiddenStatus(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	reviewSvc := services.NewReviewService(st, t.TempDir())
	id, err := reviewSvc.CreateReviewScoped(1, 1, 1, 10, 4, []string{"attendance"}, "ok", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := reviewSvc.AppealReview(id, 10, nil); err == nil {
		t.Fatalf("expected hidden-status precondition")
	}
	if err := reviewSvc.ModerateReview(id, true, "policy"); err != nil {
		t.Fatal(err)
	}
	if err := reviewSvc.AppealReview(id, 10, nil); err != nil {
		t.Fatalf("expected appeal to pass after hidden status: %v", err)
	}
}

func TestReviewModerationRequiresReason(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	reviewSvc := services.NewReviewService(st, "../static/uploads")
	id, err := reviewSvc.CreateReviewScoped(1, 1, 1, 10, 4, []string{"attendance"}, "ok", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := reviewSvc.ModerateReview(id, true, "   "); err == nil {
		t.Fatalf("expected moderation reason validation")
	}
}

func TestAppealWindowUsesHiddenAtForHiddenReviews(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	reviewSvc := services.NewReviewService(st, t.TempDir())
	id, err := reviewSvc.CreateReviewScoped(1, 1, 1, 10, 4, []string{"attendance"}, "ok", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB.Exec(`UPDATE reviews SET created_at = ? WHERE id = ?`, time.Now().AddDate(0, 0, -30), id); err != nil {
		t.Fatal(err)
	}
	if err := reviewSvc.ModerateReview(id, true, "policy"); err != nil {
		t.Fatal(err)
	}
	if err := reviewSvc.AppealReview(id, 10, nil); err != nil {
		t.Fatalf("expected appeal window to use hidden_at after moderation, got %v", err)
	}
}

func TestReviewRejectsMoreThanFiveImages(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	reviewSvc := services.NewReviewService(st, t.TempDir())
	files := []*multipart.FileHeader{{Filename: "1.jpg", Size: 10}, {Filename: "2.jpg", Size: 10}, {Filename: "3.jpg", Size: 10}, {Filename: "4.jpg", Size: 10}, {Filename: "5.jpg", Size: 10}, {Filename: "6.jpg", Size: 10}}
	if _, err := reviewSvc.CreateReviewScoped(1, 1, 1, 10, 5, []string{"attendance"}, "ok", files); err == nil {
		t.Fatalf("expected max image count validation")
	}
}

func TestReviewRejectsImageOverSizeLimit(t *testing.T) {
	st := setupStore(t)
	defer st.Close()
	reviewSvc := services.NewReviewService(st, t.TempDir())
	files := []*multipart.FileHeader{{Filename: "large.jpg", Size: 3 * 1024 * 1024}}
	if _, err := reviewSvc.CreateReviewScoped(1, 1, 1, 10, 5, []string{"attendance"}, "ok", files); err == nil {
		t.Fatalf("expected per-image size validation")
	}
}
