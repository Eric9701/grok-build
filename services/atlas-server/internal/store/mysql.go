package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// MySQLStore implements UserStore on MySQL.
type MySQLStore struct {
	db *sql.DB
}

// OpenMySQL connects and returns a MySQL-backed UserStore.
func OpenMySQL(dsn string) (*MySQLStore, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mysql ping: %w", err)
	}
	return &MySQLStore{db: db}, nil
}

// Close releases the database pool.
func (m *MySQLStore) Close() error {
	if m == nil || m.db == nil {
		return nil
	}
	return m.db.Close()
}

// Migrate creates required tables.
func (m *MySQLStore) Migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			user_id VARCHAR(64) PRIMARY KEY,
			email VARCHAR(255) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			first_name VARCHAR(128) NOT NULL DEFAULT '',
			last_name VARCHAR(128) NOT NULL DEFAULT '',
			principal_type VARCHAR(32) NOT NULL DEFAULT 'User',
			principal_id VARCHAR(64) NOT NULL,
			machine_code VARCHAR(32) NULL UNIQUE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_users_machine_code (machine_code)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token VARCHAR(64) PRIMARY KEY,
			user_id VARCHAR(64) NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_sessions_user (user_id),
			INDEX idx_sessions_expires (expires_at),
			CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS telemetry_traces (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			user_id VARCHAR(64) NULL,
			email VARCHAR(255) NULL,
			team_id VARCHAR(64) NULL,
			content_type VARCHAR(128) NOT NULL DEFAULT '',
			body LONGBLOB NOT NULL,
			body_size INT UNSIGNED NOT NULL DEFAULT 0,
			client_ip VARCHAR(64) NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_traces_user_created (user_id, created_at),
			INDEX idx_traces_email_created (email, created_at),
			INDEX idx_traces_created (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS task_reports (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			user_id VARCHAR(64) NULL,
			email VARCHAR(255) NULL,
			team_id VARCHAR(64) NULL,
			subagent_id VARCHAR(128) NULL,
			parent_session_id VARCHAR(128) NULL,
			child_session_id VARCHAR(128) NULL,
			subagent_type VARCHAR(128) NOT NULL DEFAULT '',
			model VARCHAR(128) NULL,
			description TEXT NULL,
			prompt MEDIUMTEXT NULL,
			status VARCHAR(32) NOT NULL DEFAULT '',
			success TINYINT(1) NOT NULL DEFAULT 0,
			duration_ms BIGINT UNSIGNED NOT NULL DEFAULT 0,
			tool_calls INT UNSIGNED NOT NULL DEFAULT 0,
			turns INT UNSIGNED NOT NULL DEFAULT 0,
			tokens_used BIGINT UNSIGNED NOT NULL DEFAULT 0,
			artifacts JSON NULL,
			artifact_count INT UNSIGNED NOT NULL DEFAULT 0,
			cwd VARCHAR(1024) NULL,
			worktree_path VARCHAR(1024) NULL,
			error TEXT NULL,
			started_at VARCHAR(40) NULL,
			completed_at VARCHAR(40) NULL,
			client_ip VARCHAR(64) NULL,
			client_version VARCHAR(64) NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_reports_user_created (user_id, created_at),
			INDEX idx_reports_email_created (email, created_at),
			INDEX idx_reports_agent (subagent_type),
			INDEX idx_reports_created (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS session_signals (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			user_id VARCHAR(64) NULL,
			email VARCHAR(255) NULL,
			team_id VARCHAR(64) NULL,
			session_id VARCHAR(128) NOT NULL,
			client_type VARCHAR(64) NULL,
			total_turns BIGINT NOT NULL DEFAULT 0,
			tool_call_count BIGINT NOT NULL DEFAULT 0,
			error_count BIGINT NOT NULL DEFAULT 0,
			primary_model_id VARCHAR(128) NULL,
			payload JSON NOT NULL,
			client_ip VARCHAR(64) NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_signals_session_created (session_id, created_at),
			INDEX idx_signals_user_created (user_id, created_at),
			INDEX idx_signals_email_created (email, created_at),
			INDEX idx_signals_created (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, q := range stmts {
		if _, err := m.db.Exec(q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	if err := m.migrateManagedModels(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := m.migrateUserGroups(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := m.migrateTaskReportClientVersion(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func (m *MySQLStore) migrateTaskReportClientVersion() error {
	_, err := m.db.Exec(`ALTER TABLE task_reports ADD COLUMN client_version VARCHAR(64) NULL`)
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "duplicate column") || strings.Contains(msg, "1060") {
		return nil
	}
	return err
}

// migrateManagedModels creates managed_models / user_models with the same
// varchar collation as users.user_id so foreign keys succeed on existing DBs
// (init-mysql.sql uses utf8mb4_unicode_ci; bare CREATE on MySQL 8 often gets
// utf8mb4_0900_ai_ci — Error 1215).
func (m *MySQLStore) migrateManagedModels() error {
	collation := "utf8mb4_unicode_ci"
	var detected sql.NullString
	_ = m.db.QueryRow(`
		SELECT COLLATION_NAME
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'users'
		  AND COLUMN_NAME = 'user_id'
	`).Scan(&detected)
	if detected.Valid && detected.String != "" {
		collation = detected.String
	}
	if err := validateMySQLCollation(collation); err != nil {
		return err
	}

	createManaged := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS managed_models (
			id VARCHAR(128) PRIMARY KEY,
			model VARCHAR(128) NOT NULL,
			name VARCHAR(255) NOT NULL DEFAULT '',
			description TEXT NULL,
			base_url VARCHAR(1024) NOT NULL DEFAULT '',
			api_backend VARCHAR(64) NOT NULL DEFAULT 'messages',
			api_key_enc TEXT NOT NULL,
			context_window BIGINT NOT NULL DEFAULT 200000,
			owned_by VARCHAR(64) NOT NULL DEFAULT 'atlas',
			enabled TINYINT(1) NOT NULL DEFAULT 1,
			extra_json JSON NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=%s`, collation)
	if _, err := m.db.Exec(createManaged); err != nil {
		return err
	}
	// Rewrite an earlier CREATE that landed on a different default collation.
	if _, err := m.db.Exec(fmt.Sprintf(
		`ALTER TABLE managed_models CONVERT TO CHARACTER SET utf8mb4 COLLATE %s`, collation,
	)); err != nil {
		return err
	}

	createUserModels := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS user_models (
			user_id VARCHAR(64) NOT NULL,
			model_id VARCHAR(128) NOT NULL,
			is_default TINYINT(1) NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, model_id),
			INDEX idx_user_models_model (model_id),
			CONSTRAINT fk_user_models_user FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
			CONSTRAINT fk_user_models_model FOREIGN KEY (model_id) REFERENCES managed_models(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=%s`, collation)
	if _, err := m.db.Exec(createUserModels); err != nil {
		return err
	}
	return nil
}

// migrateUserGroups creates user_groups / group_members / group_models with the
// same collation as users.user_id (see migrateManagedModels).
func (m *MySQLStore) migrateUserGroups() error {
	collation := "utf8mb4_unicode_ci"
	var detected sql.NullString
	_ = m.db.QueryRow(`
		SELECT COLLATION_NAME
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'users'
		  AND COLUMN_NAME = 'user_id'
	`).Scan(&detected)
	if detected.Valid && detected.String != "" {
		collation = detected.String
	}
	if err := validateMySQLCollation(collation); err != nil {
		return err
	}

	createGroups := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS user_groups (
			group_id VARCHAR(64) NOT NULL,
			name VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (group_id),
			UNIQUE KEY uk_user_groups_name (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=%s`, collation)
	if _, err := m.db.Exec(createGroups); err != nil {
		return err
	}

	createMembers := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS group_members (
			group_id VARCHAR(64) NOT NULL,
			user_id VARCHAR(64) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (group_id, user_id),
			INDEX idx_group_members_user (user_id),
			CONSTRAINT fk_group_members_group FOREIGN KEY (group_id) REFERENCES user_groups(group_id) ON DELETE CASCADE,
			CONSTRAINT fk_group_members_user FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=%s`, collation)
	if _, err := m.db.Exec(createMembers); err != nil {
		return err
	}

	createGroupModels := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS group_models (
			group_id VARCHAR(64) NOT NULL,
			model_id VARCHAR(128) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (group_id, model_id),
			INDEX idx_group_models_model (model_id),
			CONSTRAINT fk_group_models_group FOREIGN KEY (group_id) REFERENCES user_groups(group_id) ON DELETE CASCADE,
			CONSTRAINT fk_group_models_model FOREIGN KEY (model_id) REFERENCES managed_models(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=%s`, collation)
	if _, err := m.db.Exec(createGroupModels); err != nil {
		return err
	}
	return nil
}

func validateMySQLCollation(name string) error {
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return fmt.Errorf("unexpected MySQL collation name: %q", name)
	}
	if name == "" {
		return fmt.Errorf("empty MySQL collation name")
	}
	return nil
}

func (m *MySQLStore) CreateUser(u User) error {
	if u.PrincipalType == "" {
		u.PrincipalType = "User"
	}
	if u.PrincipalID == "" {
		u.PrincipalID = u.UserID
	}
	_, err := m.db.Exec(
		`INSERT INTO users (user_id, email, password_hash, first_name, last_name, principal_type, principal_id, machine_code)
		 VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''))`,
		u.UserID, strings.ToLower(strings.TrimSpace(u.Email)), u.PasswordHash,
		u.FirstName, u.LastName, u.PrincipalType, u.PrincipalID, u.MachineCode,
	)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			if strings.Contains(err.Error(), "email") {
				return ErrUserExists
			}
			return ErrMachineCodeInUse
		}
		return err
	}
	return nil
}

func (m *MySQLStore) GetUserByEmail(email string) (*User, error) {
	return m.scanUser(`SELECT user_id, email, password_hash, first_name, last_name, principal_type, principal_id, IFNULL(machine_code,''), created_at
		FROM users WHERE email = ?`, strings.ToLower(strings.TrimSpace(email)))
}

func (m *MySQLStore) GetUserByID(userID string) (*User, error) {
	return m.scanUser(`SELECT user_id, email, password_hash, first_name, last_name, principal_type, principal_id, IFNULL(machine_code,''), created_at
		FROM users WHERE user_id = ?`, userID)
}

func (m *MySQLStore) GetUserByMachineCode(code string) (*User, error) {
	normalized := normalizeMachineCode(code)
	if normalized == "" {
		return nil, ErrMachineCodeNeeded
	}
	return m.scanUser(`SELECT user_id, email, password_hash, first_name, last_name, principal_type, principal_id, IFNULL(machine_code,''), created_at
		FROM users WHERE machine_code = ?`, normalized)
}

func (m *MySQLStore) EnsureBootstrapUser(u User, plainPassword string) error {
	existing, err := m.GetUserByID(u.UserID)
	if err == nil && existing != nil {
		return nil
	}
	if err != nil && err != ErrUserNotFound {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	if u.PrincipalType == "" {
		u.PrincipalType = "User"
	}
	if u.PrincipalID == "" {
		u.PrincipalID = u.UserID
	}
	return m.CreateUser(u)
}

func (m *MySQLStore) CreateSession(userID string, ttl time.Duration) (string, error) {
	token, err := randomHexToken(32)
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(ttl)
	if _, err := m.db.Exec(
		`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, expires,
	); err != nil {
		return "", err
	}
	return token, nil
}

func (m *MySQLStore) GetUserBySession(token string) (*User, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrSessionNotFound
	}
	var userID string
	var expires time.Time
	err := m.db.QueryRow(
		`SELECT user_id, expires_at FROM sessions WHERE token = ?`, token,
	).Scan(&userID, &expires)
	if err == sql.ErrNoRows {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	if time.Now().After(expires) {
		_, _ = m.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
		return nil, ErrSessionExpired
	}
	return m.GetUserByID(userID)
}

func (m *MySQLStore) DeleteSession(token string) error {
	_, err := m.db.Exec(`DELETE FROM sessions WHERE token = ?`, strings.TrimSpace(token))
	return err
}

func (m *MySQLStore) scanUser(query string, args ...any) (*User, error) {
	var u User
	var machineCode string
	err := m.db.QueryRow(query, args...).Scan(
		&u.UserID, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName,
		&u.PrincipalType, &u.PrincipalID, &machineCode, &u.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	u.MachineCode = machineCode
	return &u, nil
}

// HashPassword returns a bcrypt hash for a plaintext password.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword compares a bcrypt hash with plaintext.
func CheckPassword(hash, plain string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		return ErrInvalidPassword
	}
	return nil
}

// GenerateMachineCode returns an RFC8628-style machine code for a user account.
func GenerateMachineCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	chars := make([]byte, 8)
	for i := 0; i < 8; i++ {
		chars[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(chars[0:4]) + "-" + string(chars[4:8]), nil
}

func normalizeMachineCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// SetMachineCodeIfEmpty assigns a machine code when the user has none.
func (m *MySQLStore) SetMachineCodeIfEmpty(userID, machineCode string) error {
	res, err := m.db.Exec(
		`UPDATE users SET machine_code = ? WHERE user_id = ? AND (machine_code IS NULL OR machine_code = '')`,
		machineCode, userID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil
	}
	return nil
}

// ListUsersDetail returns all users with full detail for admin management.
func (m *MySQLStore) ListUsersDetail() ([]UserDetail, error) {
	rows, err := m.db.Query(
		`SELECT user_id, email, first_name, last_name, COALESCE(machine_code,''), created_at FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []UserDetail
	for rows.Next() {
		var u UserDetail
		var t time.Time
		if err := rows.Scan(&u.UserID, &u.Email, &u.FirstName, &u.LastName, &u.MachineCode, &t); err != nil {
			return nil, err
		}
		u.CreatedAt = t.Format(time.RFC3339)
		users = append(users, u)
	}
	return users, rows.Err()
}

func randomHexToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
