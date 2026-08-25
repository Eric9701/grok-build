package store

import (
	"database/sql"
	"fmt"
	"time"
)

// refreshPersister is the durable half of AuthStore (MySQL in production).
type refreshPersister interface {
	SaveRefresh(r RefreshRecord) error
	GetRefresh(token string) (*RefreshRecord, error)
	RevokeRefresh(token string) error
}

// NewAuthStore keeps RFC 8628 device codes in process memory (15-minute
// login window) and persists refresh tokens in MySQL so CLI silent refresh
// survives atlas-server restarts.
func NewAuthStore(mysql *MySQLStore) Store {
	return newAuthStore(mysql)
}

func newAuthStore(persist refreshPersister) Store {
	return &authStore{
		MemoryStore: NewMemoryStore(),
		persist:     persist,
	}
}

type authStore struct {
	*MemoryStore
	persist refreshPersister
}

func (s *authStore) SaveRefresh(r RefreshRecord) error {
	return s.persist.SaveRefresh(r)
}

func (s *authStore) GetRefresh(token string) (*RefreshRecord, error) {
	return s.persist.GetRefresh(token)
}

func (s *authStore) RevokeRefresh(token string) error {
	return s.persist.RevokeRefresh(token)
}

func (m *MySQLStore) SaveRefresh(r RefreshRecord) error {
	_, err := m.db.Exec(
		`INSERT INTO refresh_tokens (token, user_id, email, client_id, scope, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		r.Token, r.UserID, r.Email, r.ClientID, r.Scope, r.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}
	return nil
}

func (m *MySQLStore) GetRefresh(token string) (*RefreshRecord, error) {
	var rec RefreshRecord
	err := m.db.QueryRow(
		`SELECT token, user_id, email, client_id, scope, expires_at
		 FROM refresh_tokens WHERE token = ?`, token,
	).Scan(&rec.Token, &rec.UserID, &rec.Email, &rec.ClientID, &rec.Scope, &rec.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, ErrRefreshNotFound
	}
	if err != nil {
		return nil, err
	}
	if time.Now().After(rec.ExpiresAt) {
		_, _ = m.db.Exec(`DELETE FROM refresh_tokens WHERE token = ?`, token)
		return nil, ErrRefreshExpired
	}
	return &rec, nil
}

func (m *MySQLStore) RevokeRefresh(token string) error {
	_, err := m.db.Exec(`DELETE FROM refresh_tokens WHERE token = ?`, token)
	return err
}

func (m *MySQLStore) migrateRefreshTokens() error {
	collation, err := m.usersCollation()
	if err != nil {
		return err
	}
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS refresh_tokens (
			token VARCHAR(128) NOT NULL,
			user_id VARCHAR(64) NOT NULL,
			email VARCHAR(255) NOT NULL DEFAULT '',
			client_id VARCHAR(255) NOT NULL DEFAULT '',
			scope VARCHAR(512) NOT NULL DEFAULT '',
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (token),
			KEY idx_refresh_tokens_user (user_id),
			KEY idx_refresh_tokens_expires (expires_at),
			CONSTRAINT fk_refresh_tokens_user FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=%s`, collation)
	_, err = m.db.Exec(q)
	return err
}

func (m *MySQLStore) usersCollation() (string, error) {
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
		return "", err
	}
	return collation, nil
}
