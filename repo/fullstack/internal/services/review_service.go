package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"clubops_portal/fullstack/internal/models"
	"clubops_portal/fullstack/internal/store"
)

type ReviewService struct {
	store     *store.SQLiteStore
	uploadDir string
}

func NewReviewService(st *store.SQLiteStore, uploadDir string) *ReviewService {
	_ = os.MkdirAll(uploadDir, 0o755)
	return &ReviewService{store: st, uploadDir: uploadDir}
}

func (s *ReviewService) CreateReview(siteID, memberID, reviewerID int64, stars int, comment string, files []*multipart.FileHeader) (int64, error) {
	return s.CreateReviewScoped(1, siteID, memberID, reviewerID, stars, nil, comment, files)
}

func (s *ReviewService) CreateReviewForOrder(orderID, reviewerID int64, stars int, tags []string, comment string, files []*multipart.FileHeader) (int64, error) {
	order, err := s.store.GetFulfilledOrderByID(orderID)
	if err != nil {
		return 0, err
	}
	if strings.ToLower(strings.TrimSpace(order.Status)) != "fulfilled" {
		return 0, errors.New("review requires a fulfilled order")
	}
	id, err := s.CreateReviewScoped(order.ClubID, order.SiteID, order.MemberID, reviewerID, stars, tags, comment, files)
	if err != nil {
		return 0, err
	}
	if _, err := s.store.DB.Exec(`UPDATE reviews SET fulfilled_order_id = ? WHERE id = ?`, order.ID, id); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *ReviewService) CreateReviewScoped(clubID, siteID, memberID, reviewerID int64, stars int, tags []string, comment string, files []*multipart.FileHeader) (int64, error) {
	if stars < 1 || stars > 5 {
		return 0, errors.New("stars must be between 1 and 5")
	}
	if len(comment) > 500 {
		return 0, errors.New("comment must be <= 500 chars")
	}
	if len(files) > 5 {
		return 0, errors.New("max 5 images allowed")
	}
	allowedTags := map[string]bool{"professionalism": true, "communication": true, "attendance": true, "safety": true, "leadership": true}
	normalizedTags := []string{}
	for _, tag := range tags {
		t := strings.ToLower(strings.TrimSpace(tag))
		if t == "" {
			continue
		}
		if !allowedTags[t] {
			return 0, errors.New("invalid review tag")
		}
		normalizedTags = append(normalizedTags, t)
	}
	sort.Strings(normalizedTags)
	paths := []string{}
	for _, fh := range files {
		if fh.Size > 2*1024*1024 {
			return 0, errors.New("each image must be <= 2MB")
		}
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			return 0, errors.New("invalid image type")
		}
		savedName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(fh.Filename))
		target := filepath.Join(s.uploadDir, savedName)
		if err := saveFileHeader(fh, target); err != nil {
			return 0, err
		}
		paths = append(paths, "/static/uploads/"+savedName)
	}
	b, _ := json.Marshal(paths)
	tagJSON, _ := json.Marshal(normalizedTags)
	return s.store.InsertReview(models.Review{
		ClubID:       clubID,
		SiteID:       siteID,
		MemberID:     memberID,
		ReviewerID:   reviewerID,
		Stars:        stars,
		Tags:         string(tagJSON),
		Comment:      comment,
		ImagePaths:   string(b),
		AppealStatus: "none",
	})
}

func (s *ReviewService) AppealReview(reviewID int64, userID int64, clubScope *int64) error {
	return s.store.AppealReview(reviewID, userID, clubScope)
}

func (s *ReviewService) ListReviews(clubID *int64) ([]models.Review, error) {
	return s.store.ListReviews(clubID)
}

func (s *ReviewService) ModerateReview(reviewID int64, hide bool, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return errors.New("moderation reason required")
	}
	return s.store.SetReviewModeration(reviewID, hide, reason)
}

func saveFileHeader(fh *multipart.FileHeader, target string) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = out.ReadFrom(src)
	return err
}
