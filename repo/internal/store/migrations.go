package store

import "strings"

func (s *SQLiteStore) AutoMigrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS clubs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			tags TEXT NOT NULL DEFAULT '[]',
			avatar_path TEXT NOT NULL DEFAULT '',
			recruitment_open INTEGER NOT NULL DEFAULT 1,
			description TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL,
			club_id INTEGER,
			failed_attempts INTEGER NOT NULL DEFAULT 0,
			locked_until DATETIME,
			must_change_password INTEGER NOT NULL DEFAULT 0,
			password_set_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (club_id) REFERENCES clubs(id)
		);`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);`,
		`CREATE TABLE IF NOT EXISTS budgets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			club_id INTEGER NOT NULL,
			account_code TEXT NOT NULL DEFAULT '',
			campus_code TEXT NOT NULL DEFAULT '',
			project_code TEXT NOT NULL DEFAULT '',
			period_type TEXT NOT NULL,
			period_start TEXT NOT NULL,
			amount REAL NOT NULL,
			spent REAL NOT NULL DEFAULT 0,
			threshold_alert INTEGER NOT NULL DEFAULT 0,
			created_by INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (club_id) REFERENCES clubs(id),
			FOREIGN KEY (created_by) REFERENCES users(id)
		);`,
		`CREATE TABLE IF NOT EXISTS budget_change_requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			budget_id INTEGER NOT NULL,
			requested_by INTEGER NOT NULL,
			proposed_amount REAL NOT NULL,
			change_percent REAL NOT NULL,
			reason TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			reviewed_by INTEGER,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (budget_id) REFERENCES budgets(id),
			FOREIGN KEY (requested_by) REFERENCES users(id),
			FOREIGN KEY (reviewed_by) REFERENCES users(id)
		);`,
		`CREATE TABLE IF NOT EXISTS region_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label TEXT NOT NULL,
			created_by INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (created_by) REFERENCES users(id)
		);`,
		`CREATE TABLE IF NOT EXISTS regions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version_id INTEGER NOT NULL,
			state TEXT NOT NULL,
			county TEXT NOT NULL,
			city TEXT NOT NULL,
			FOREIGN KEY (version_id) REFERENCES region_versions(id)
		);`,
		`CREATE TABLE IF NOT EXISTS fulfilled_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			club_id INTEGER NOT NULL,
			site_id INTEGER NOT NULL,
			member_id INTEGER NOT NULL,
			owner_user_id INTEGER NOT NULL DEFAULT 1,
			service_label TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'fulfilled',
			fulfilled_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (club_id) REFERENCES clubs(id),
			FOREIGN KEY (owner_user_id) REFERENCES users(id)
		);`,
		`CREATE TABLE IF NOT EXISTS mdm_dimension_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			dimension_name TEXT NOT NULL,
			label TEXT NOT NULL,
			created_by INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (created_by) REFERENCES users(id)
		);`,
		`CREATE TABLE IF NOT EXISTS mdm_dimensions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version_id INTEGER NOT NULL,
			code TEXT NOT NULL,
			value TEXT NOT NULL,
			UNIQUE(version_id, code),
			FOREIGN KEY (version_id) REFERENCES mdm_dimension_versions(id)
		);`,
		`CREATE TABLE IF NOT EXISTS sales_facts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			product_code TEXT NOT NULL,
			customer_code TEXT NOT NULL,
			channel_code TEXT NOT NULL,
			region_code TEXT NOT NULL,
			time_code TEXT NOT NULL,
			amount REAL NOT NULL,
			transaction_date TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS credit_rule_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version_label TEXT NOT NULL,
			formula_json TEXT NOT NULL,
			makeup_enabled INTEGER NOT NULL DEFAULT 0,
			retake_enabled INTEGER NOT NULL DEFAULT 0,
			effective_from TEXT NOT NULL,
			effective_to TEXT,
			created_by INTEGER NOT NULL,
			is_active INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (created_by) REFERENCES users(id)
		);`,
		`CREATE TABLE IF NOT EXISTS credit_issued (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			member_id INTEGER NOT NULL,
			rule_version_id INTEGER NOT NULL,
			base_score REAL NOT NULL,
			makeup_used INTEGER NOT NULL DEFAULT 0,
			retake_used INTEGER NOT NULL DEFAULT 0,
			calculated_credit REAL NOT NULL,
			immutable_hash TEXT NOT NULL,
			issued_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(member_id, rule_version_id),
			FOREIGN KEY (rule_version_id) REFERENCES credit_rule_versions(id)
		);`,
		`CREATE TABLE IF NOT EXISTS reviews (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			club_id INTEGER NOT NULL,
			fulfilled_order_id INTEGER,
			site_id INTEGER NOT NULL,
			member_id INTEGER NOT NULL,
			reviewer_id INTEGER NOT NULL,
			stars INTEGER NOT NULL,
			tags TEXT NOT NULL DEFAULT '[]',
			comment TEXT,
			image_paths TEXT,
			appeal_status TEXT NOT NULL DEFAULT 'none',
			hidden_reason TEXT,
			hidden_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (fulfilled_order_id) REFERENCES fulfilled_orders(id)
		);`,
		`CREATE TABLE IF NOT EXISTS members (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			club_id INTEGER NOT NULL,
			full_name TEXT NOT NULL,
			email_encrypted TEXT NOT NULL,
			phone_encrypted TEXT NOT NULL,
			join_date TEXT NOT NULL DEFAULT '',
			position_title TEXT NOT NULL DEFAULT '',
			is_active INTEGER NOT NULL DEFAULT 1,
			group_name TEXT NOT NULL DEFAULT '',
			custom_fields TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (club_id) REFERENCES clubs(id)
		);`,
		`CREATE TABLE IF NOT EXISTS feature_flags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			flag_key TEXT NOT NULL UNIQUE,
			enabled INTEGER NOT NULL DEFAULT 0,
			target_scope TEXT NOT NULL DEFAULT 'global',
			rollout_pct INTEGER NOT NULL DEFAULT 100,
			updated_by INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (updated_by) REFERENCES users(id)
		);`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			method TEXT NOT NULL,
			path TEXT NOT NULL,
			entity TEXT,
			entity_id TEXT,
			before_state TEXT,
			after_state TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			retention_until DATETIME NOT NULL
		);`,
	}

	for _, stmt := range stmts {
		if _, err := s.DB.Exec(stmt); err != nil {
			return err
		}
	}
	indexStmts := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_reviews_fulfilled_order_reviewer ON reviews(fulfilled_order_id, reviewer_id)`,
	}
	for _, stmt := range indexStmts {
		if _, err := s.DB.Exec(stmt); err != nil {
			return err
		}
	}
	alterStmts := []string{
		`ALTER TABLE users ADD COLUMN must_change_password INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN password_set_at DATETIME NOT NULL DEFAULT '1970-01-01T00:00:00Z'`,
		`ALTER TABLE clubs ADD COLUMN tags TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE clubs ADD COLUMN avatar_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE clubs ADD COLUMN recruitment_open INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE clubs ADD COLUMN description TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE reviews ADD COLUMN club_id INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE reviews ADD COLUMN tags TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE reviews ADD COLUMN hidden_reason TEXT`,
		`ALTER TABLE reviews ADD COLUMN hidden_at DATETIME`,
		`ALTER TABLE reviews ADD COLUMN fulfilled_order_id INTEGER`,
		`ALTER TABLE fulfilled_orders ADD COLUMN owner_user_id INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE credit_rule_versions ADD COLUMN effective_from TEXT NOT NULL DEFAULT '1970-01-01'`,
		`ALTER TABLE credit_rule_versions ADD COLUMN effective_to TEXT`,
		`ALTER TABLE budgets ADD COLUMN account_code TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE budgets ADD COLUMN campus_code TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE budgets ADD COLUMN project_code TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE feature_flags ADD COLUMN rollout_pct INTEGER NOT NULL DEFAULT 100`,
		`ALTER TABLE members ADD COLUMN join_date TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN position_title TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN is_active INTEGER NOT NULL DEFAULT 1`,
	}
	for _, stmt := range alterStmts {
		if _, err := s.DB.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	triggerStmts := []string{
		`DROP TRIGGER IF EXISTS trg_audit_logs_no_update;`,
		`DROP TRIGGER IF EXISTS trg_audit_logs_no_early_delete;`,
		`CREATE TRIGGER trg_audit_logs_no_update
		BEFORE UPDATE ON audit_logs
		BEGIN
			SELECT RAISE(ABORT, 'audit_logs are append-only');
		END;`,
		`CREATE TRIGGER trg_audit_logs_no_early_delete
		BEFORE DELETE ON audit_logs
		WHEN julianday(replace(substr(CAST(OLD.retention_until AS TEXT), 1, 19), 'T', ' ')) >= julianday('now')
		BEGIN
			SELECT RAISE(ABORT, 'audit_logs cannot be deleted before retention expiry');
		END;`,
	}
	for _, stmt := range triggerStmts {
		if _, err := s.DB.Exec(stmt); err != nil {
			return err
		}
	}
	_, _ = s.DB.Exec(`UPDATE users SET password_set_at = CURRENT_TIMESTAMP WHERE password_set_at = '1970-01-01T00:00:00Z'`)
	return nil
}
