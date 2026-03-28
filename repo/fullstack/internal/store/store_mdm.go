package store

import (
	"errors"
	"strconv"
	"strings"

	"clubops_portal/fullstack/internal/models"
)

func (s *SQLiteStore) InsertRegionVersion(label string, createdBy int64, rows [][3]string) (int64, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO region_versions (label, created_by) VALUES (?, ?)`, label, createdBy)
	if err != nil {
		return 0, err
	}
	verID, _ := res.LastInsertId()
	stmt, err := tx.Prepare(`INSERT INTO regions (version_id, state, county, city) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(verID, r[0], r[1], r[2]); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return verID, nil
}

func (s *SQLiteStore) ListRegionVersions() ([]models.RegionVersion, error) {
	rows, err := s.DB.Query(`SELECT id, label, created_by, created_at FROM region_versions ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.RegionVersion{}
	for rows.Next() {
		var rv models.RegionVersion
		if err := rows.Scan(&rv.ID, &rv.Label, &rv.CreatedBy, &rv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rv)
	}
	return out, nil
}

func (s *SQLiteStore) GetRegionVersion(id int64) (*models.RegionVersion, []models.RegionNode, error) {
	var rv models.RegionVersion
	if err := s.DB.QueryRow(`SELECT id, label, created_by, created_at FROM region_versions WHERE id = ?`, id).Scan(&rv.ID, &rv.Label, &rv.CreatedBy, &rv.CreatedAt); err != nil {
		return nil, nil, err
	}
	rows, err := s.DB.Query(`SELECT id, version_id, state, county, city FROM regions WHERE version_id = ? ORDER BY state, county, city, id`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	nodes := []models.RegionNode{}
	for rows.Next() {
		var node models.RegionNode
		if err := rows.Scan(&node.ID, &node.VersionID, &node.State, &node.County, &node.City); err != nil {
			return nil, nil, err
		}
		nodes = append(nodes, node)
	}
	return &rv, nodes, nil
}

func (s *SQLiteStore) UpdateRegionVersion(id int64, label string, rows [][3]string, createdBy int64) (int64, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var existing int64
	if err := tx.QueryRow(`SELECT id FROM region_versions WHERE id = ?`, id).Scan(&existing); err != nil {
		return 0, err
	}
	res, err := tx.Exec(`INSERT INTO region_versions (label, created_by) VALUES (?, ?)`, label, createdBy)
	if err != nil {
		return 0, err
	}
	newID, _ := res.LastInsertId()
	stmt, err := tx.Prepare(`INSERT INTO regions (version_id, state, county, city) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, row := range rows {
		if _, err := stmt.Exec(newID, row[0], row[1], row[2]); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return newID, nil
}

func (s *SQLiteStore) InsertDimensionVersion(dimensionName, label string, createdBy int64, rows [][2]string) (int64, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO mdm_dimension_versions (dimension_name, label, created_by) VALUES (?, ?, ?)`, dimensionName, label, createdBy)
	if err != nil {
		return 0, err
	}
	verID, _ := res.LastInsertId()
	stmt, err := tx.Prepare(`INSERT INTO mdm_dimensions (version_id, code, value) VALUES (?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(verID, r[0], r[1]); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return verID, nil
}

func (s *SQLiteStore) InsertSalesFact(f models.SalesFact) (int64, error) {
	res, err := s.DB.Exec(`INSERT INTO sales_facts (product_code, customer_code, channel_code, region_code, time_code, amount, transaction_date) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.ProductCode, f.CustomerCode, f.ChannelCode, f.RegionCode, f.TimeCode, f.Amount, f.TransactionDate)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *SQLiteStore) InsertSalesFactFromCSVRow(rec []string) (int64, error) {
	product := strings.TrimSpace(rec[0])
	customer := strings.TrimSpace(rec[1])
	channel := strings.TrimSpace(rec[2])
	region := strings.TrimSpace(rec[3])
	timeCode := strings.TrimSpace(rec[4])
	if ok, err := s.DimensionCodeExists("product", product); err != nil || !ok {
		return 0, errors.New("unknown product_code")
	}
	if ok, err := s.DimensionCodeExists("customer", customer); err != nil || !ok {
		return 0, errors.New("unknown customer_code")
	}
	if ok, err := s.DimensionCodeExists("channel", channel); err != nil || !ok {
		return 0, errors.New("unknown channel_code")
	}
	if ok, err := s.DimensionCodeExists("region", region); err != nil || !ok {
		return 0, errors.New("unknown region_code")
	}
	if ok, err := s.DimensionCodeExists("time", timeCode); err != nil || !ok {
		return 0, errors.New("unknown time_code")
	}
	amount, err := strconv.ParseFloat(strings.TrimSpace(rec[5]), 64)
	if err != nil {
		return 0, errors.New("invalid amount")
	}
	return s.InsertSalesFact(models.SalesFact{
		ProductCode:     product,
		CustomerCode:    customer,
		ChannelCode:     channel,
		RegionCode:      region,
		TimeCode:        timeCode,
		Amount:          amount,
		TransactionDate: strings.TrimSpace(rec[6]),
	})
}

func (s *SQLiteStore) DimensionCodeExists(dimensionName, code string) (bool, error) {
	var exists int
	err := s.DB.QueryRow(`SELECT COUNT(1)
		FROM mdm_dimensions d
		JOIN mdm_dimension_versions v ON v.id = d.version_id
		WHERE v.dimension_name = ?
		AND d.code = ?`, dimensionName, strings.ToUpper(code)).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (s *SQLiteStore) ListDimensionVersions(dimensionName string) ([]models.DimensionVersion, error) {
	query := `SELECT id, dimension_name, label, created_by, created_at FROM mdm_dimension_versions`
	args := []any{}
	if strings.TrimSpace(dimensionName) != "" {
		query += ` WHERE dimension_name = ?`
		args = append(args, dimensionName)
	}
	query += ` ORDER BY created_at DESC, id DESC`
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.DimensionVersion{}
	for rows.Next() {
		var v models.DimensionVersion
		if err := rows.Scan(&v.ID, &v.DimensionName, &v.Label, &v.CreatedBy, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *SQLiteStore) ListRecentSalesFacts(limit int) ([]models.SalesFact, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := s.DB.Query(`SELECT id, product_code, customer_code, channel_code, region_code, time_code, amount, transaction_date, created_at FROM sales_facts ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.SalesFact{}
	for rows.Next() {
		var f models.SalesFact
		if err := rows.Scan(&f.ID, &f.ProductCode, &f.CustomerCode, &f.ChannelCode, &f.RegionCode, &f.TimeCode, &f.Amount, &f.TransactionDate, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}
