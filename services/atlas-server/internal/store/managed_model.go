package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ManagedModel is a cloud-configured model catalog entry.
type ManagedModel struct {
	ID              string
	Model           string
	Name            string
	Description     string
	BaseURL         string
	APIBackend      string
	APIKeyEnc       string
	ContextWindow   int64
	OwnedBy         string
	Enabled         bool
	ExtraJSON       string // optional raw JSON for extra fields
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UserModelAssignment links a user to a managed model.
type UserModelAssignment struct {
	UserID    string
	ModelID   string
	IsDefault bool
}

var (
	ErrManagedModelNotFound = errors.New("managed model not found")
	ErrManagedModelExists   = errors.New("managed model already exists")
)

// ManagedModelStore persists cloud model catalog + per-user assignments.
type ManagedModelStore interface {
	ListManagedModels() ([]ManagedModel, error)
	GetManagedModel(id string) (*ManagedModel, error)
	UpsertManagedModel(m ManagedModel) error
	DeleteManagedModel(id string) error

	ListModelsForUser(userID string) ([]ManagedModel, error)
	ListUserModelIDs(userID string) ([]UserModelAssignment, error)
	SetUserModels(userID string, modelIDs []string, defaultID string) error

	ListUsersBrief() ([]UserBrief, error)
}

// UserBrief is a minimal user row for admin assignment UI.
type UserBrief struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

// ListManagedModels returns all managed models (enabled and disabled).
func (m *MySQLStore) ListManagedModels() ([]ManagedModel, error) {
	rows, err := m.db.Query(`SELECT id, model, name, description, base_url, api_backend, api_key_enc,
		context_window, owned_by, enabled, IFNULL(extra_json,''), created_at, updated_at
		FROM managed_models ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanManagedModels(rows)
}

func (m *MySQLStore) GetManagedModel(id string) (*ManagedModel, error) {
	row := m.db.QueryRow(`SELECT id, model, name, description, base_url, api_backend, api_key_enc,
		context_window, owned_by, enabled, IFNULL(extra_json,''), created_at, updated_at
		FROM managed_models WHERE id = ?`, id)
	mm, err := scanManagedModel(row)
	if err == sql.ErrNoRows {
		return nil, ErrManagedModelNotFound
	}
	return mm, err
}

func (m *MySQLStore) UpsertManagedModel(mm ManagedModel) error {
	if mm.ID == "" || mm.Model == "" {
		return errors.New("id and model required")
	}
	if mm.Name == "" {
		mm.Name = mm.ID
	}
	if mm.APIBackend == "" {
		mm.APIBackend = "messages"
	}
	if mm.ContextWindow <= 0 {
		mm.ContextWindow = 200000
	}
	if mm.OwnedBy == "" {
		mm.OwnedBy = "atlas"
	}
	enabled := 0
	if mm.Enabled {
		enabled = 1
	}
	_, err := m.db.Exec(`INSERT INTO managed_models
		(id, model, name, description, base_url, api_backend, api_key_enc, context_window, owned_by, enabled, extra_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''))
		ON DUPLICATE KEY UPDATE
			model=VALUES(model), name=VALUES(name), description=VALUES(description),
			base_url=VALUES(base_url), api_backend=VALUES(api_backend),
			api_key_enc=IF(VALUES(api_key_enc)='', api_key_enc, VALUES(api_key_enc)),
			context_window=VALUES(context_window), owned_by=VALUES(owned_by),
			enabled=VALUES(enabled),
			extra_json=IF(VALUES(extra_json) IS NULL OR VALUES(extra_json)='', extra_json, VALUES(extra_json))`,
		mm.ID, mm.Model, mm.Name, mm.Description, mm.BaseURL, mm.APIBackend, mm.APIKeyEnc,
		mm.ContextWindow, mm.OwnedBy, enabled, mm.ExtraJSON,
	)
	return err
}

func (m *MySQLStore) DeleteManagedModel(id string) error {
	res, err := m.db.Exec(`DELETE FROM managed_models WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrManagedModelNotFound
	}
	return nil
}

func (m *MySQLStore) ListModelsForUser(userID string) ([]ManagedModel, error) {
	rows, err := m.db.Query(`SELECT mm.id, mm.model, mm.name, mm.description, mm.base_url, mm.api_backend, mm.api_key_enc,
		mm.context_window, mm.owned_by, mm.enabled, IFNULL(mm.extra_json,''), mm.created_at, mm.updated_at
		FROM managed_models mm
		INNER JOIN user_models um ON um.model_id = mm.id
		WHERE um.user_id = ? AND mm.enabled = 1
		ORDER BY um.is_default DESC, mm.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanManagedModels(rows)
}

func (m *MySQLStore) ListUserModelIDs(userID string) ([]UserModelAssignment, error) {
	rows, err := m.db.Query(`SELECT user_id, model_id, is_default FROM user_models WHERE user_id = ? ORDER BY model_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserModelAssignment
	for rows.Next() {
		var a UserModelAssignment
		var def int
		if err := rows.Scan(&a.UserID, &a.ModelID, &def); err != nil {
			return nil, err
		}
		a.IsDefault = def != 0
		out = append(out, a)
	}
	return out, rows.Err()
}

func (m *MySQLStore) SetUserModels(userID string, modelIDs []string, defaultID string) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM user_models WHERE user_id = ?`, userID); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, id := range modelIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		isDef := 0
		if defaultID != "" && id == defaultID {
			isDef = 1
		}
		if _, err := tx.Exec(`INSERT INTO user_models (user_id, model_id, is_default) VALUES (?, ?, ?)`,
			userID, id, isDef); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (m *MySQLStore) ListUsersBrief() ([]UserBrief, error) {
	rows, err := m.db.Query(`SELECT user_id, email, first_name, last_name FROM users ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserBrief
	for rows.Next() {
		var u UserBrief
		var fn, ln string
		if err := rows.Scan(&u.UserID, &u.Email, &fn, &ln); err != nil {
			return nil, err
		}
		u.Name = strings.TrimSpace(fn + " " + ln)
		out = append(out, u)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanManagedModel(s scanner) (*ManagedModel, error) {
	var mm ManagedModel
	var enabled int
	err := s.Scan(&mm.ID, &mm.Model, &mm.Name, &mm.Description, &mm.BaseURL, &mm.APIBackend,
		&mm.APIKeyEnc, &mm.ContextWindow, &mm.OwnedBy, &enabled, &mm.ExtraJSON, &mm.CreatedAt, &mm.UpdatedAt)
	if err != nil {
		return nil, err
	}
	mm.Enabled = enabled != 0
	return &mm, nil
}

func scanManagedModels(rows *sql.Rows) ([]ManagedModel, error) {
	var out []ManagedModel
	for rows.Next() {
		mm, err := scanManagedModel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *mm)
	}
	return out, rows.Err()
}
