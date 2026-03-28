package services

import (
	"bufio"
	"encoding/csv"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"

	"clubops_portal/fullstack/internal/models"
	"clubops_portal/fullstack/internal/store"
)

type MDMService struct {
	store *store.SQLiteStore
}

func NewMDMService(st *store.SQLiteStore) *MDMService { return &MDMService{store: st} }

func (s *MDMService) ImportRegionCSV(r io.Reader, versionLabel string, createdBy int64) (int64, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	cr := csv.NewReader(br)
	cr.FieldsPerRecord = -1
	rows := make([][3]string, 0, 512)
	count := 0
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, err
		}
		if len(rec) < 3 {
			return 0, errors.New("region csv requires state,county,city")
		}
		rows = append(rows, [3]string{rec[0], rec[1], rec[2]})
		count++
		if count > 5000 {
			return 0, errors.New("csv row limit exceeded (5000)")
		}
	}
	if count == 0 {
		return 0, errors.New("csv empty")
	}
	return s.store.InsertRegionVersion(versionLabel, createdBy, rows)
}

func (s *MDMService) ListRegionVersions() ([]models.RegionVersion, error) {
	return s.store.ListRegionVersions()
}

func (s *MDMService) GetRegionVersion(id int64) (*models.RegionVersion, []models.RegionNode, error) {
	return s.store.GetRegionVersion(id)
}

func (s *MDMService) UpdateRegionVersion(id int64, versionLabel string, rows [][3]string, createdBy int64) (int64, error) {
	cleanLabel := strings.TrimSpace(versionLabel)
	if cleanLabel == "" {
		return 0, errors.New("version_label required")
	}
	if len(rows) == 0 {
		return 0, errors.New("at least one region row required")
	}
	for _, row := range rows {
		if strings.TrimSpace(row[0]) == "" || strings.TrimSpace(row[1]) == "" || strings.TrimSpace(row[2]) == "" {
			return 0, errors.New("region rows require state,county,city")
		}
	}
	return s.store.UpdateRegionVersion(id, cleanLabel, rows, createdBy)
}

func (s *MDMService) ListDimensionVersions(dimensionName string) ([]models.DimensionVersion, error) {
	return s.store.ListDimensionVersions(dimensionName)
}

func (s *MDMService) ListRecentSalesFacts(limit int) ([]models.SalesFact, error) {
	return s.store.ListRecentSalesFacts(limit)
}

func (s *MDMService) ImportDimensionCSV(r io.Reader, dimensionName, versionLabel string, createdBy int64) (int64, error) {
	allowed := map[string]bool{"product": true, "customer": true, "channel": true, "region": true, "time": true}
	requiredCodeLength := map[string]int{"product": 5, "customer": 5, "channel": 5, "region": 4, "time": 5}
	dim := strings.ToLower(strings.TrimSpace(dimensionName))
	if !allowed[dim] {
		return 0, errors.New("dimension_name must be one of product,customer,channel,region,time")
	}
	codePattern := regexp.MustCompile(`^[A-Za-z0-9]+$`)
	br := bufio.NewReaderSize(r, 64*1024)
	cr := csv.NewReader(br)
	cr.FieldsPerRecord = -1
	rows := make([][2]string, 0, 512)
	count := 0
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, err
		}
		if len(rec) < 2 {
			return 0, errors.New("dimension csv requires code,value")
		}
		code := strings.TrimSpace(rec[0])
		value := strings.TrimSpace(rec[1])
		requiredLen := requiredCodeLength[dim]
		if !codePattern.MatchString(code) || len(code) != requiredLen {
			return 0, errors.New("code must be alphanumeric and exactly " + strconv.Itoa(requiredLen) + " chars for " + dim)
		}
		if value == "" {
			return 0, errors.New("value cannot be empty")
		}
		rows = append(rows, [2]string{strings.ToUpper(code), value})
		count++
		if count > 5000 {
			return 0, errors.New("csv row limit exceeded (5000)")
		}
	}
	if count == 0 {
		return 0, errors.New("csv empty")
	}
	return s.store.InsertDimensionVersion(dim, versionLabel, createdBy, rows)
}

func (s *MDMService) ImportSalesFactCSV(r io.Reader) (int, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	cr := csv.NewReader(br)
	count := 0
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, err
		}
		count++
		if count == 1 {
			continue
		}
		if count > 5001 {
			return 0, errors.New("csv row limit exceeded (5000)")
		}
		if len(rec) < 7 {
			return 0, errors.New("sales fact csv requires product_code,customer_code,channel_code,region_code,time_code,amount,transaction_date")
		}
		amountRows, err := s.store.InsertSalesFactFromCSVRow(rec)
		if err != nil {
			return 0, err
		}
		_ = amountRows
	}
	if count <= 1 {
		return 0, errors.New("csv empty")
	}
	return count - 1, nil
}
