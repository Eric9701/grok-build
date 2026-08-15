package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

// UserGroup is a named set of users for Group Assignment of managed models.
type UserGroup struct {
	GroupID   string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserGroupBrief is a list row with optional counts for Admin UI.
type UserGroupBrief struct {
	GroupID     string `json:"group_id"`
	Name        string `json:"name"`
	MemberCount int    `json:"member_count"`
	ModelCount  int    `json:"model_count"`
}

// EffectiveModelSource explains how a model entered the Effective Model Set.
type EffectiveModelSource struct {
	ModelID   string `json:"model_id"`
	Via       string `json:"via"` // "direct" | "group"
	GroupID   string `json:"group_id,omitempty"`
	GroupName string `json:"group_name,omitempty"`
}

// EffectiveModels is the Admin preview of a user's Effective Model Set.
type EffectiveModels struct {
	UserID       string                 `json:"user_id"`
	ModelIDs     []string               `json:"model_ids"`
	DefaultModel string                 `json:"default_model"`
	Sources      []EffectiveModelSource `json:"sources"`
}

var (
	ErrUserGroupNotFound = errors.New("user group not found")
	ErrUserGroupNameTaken = errors.New("user group name already taken")
)

// CreateUserGroup inserts a new group with a generated group_id.
func (m *MySQLStore) CreateUserGroup(name string) (*UserGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("name required")
	}
	g := &UserGroup{
		GroupID: "grp-" + uuid.NewString(),
		Name:    name,
	}
	_, err := m.db.Exec(`INSERT INTO user_groups (group_id, name) VALUES (?, ?)`, g.GroupID, g.Name)
	if err != nil {
		if isMySQLDuplicate(err) {
			return nil, ErrUserGroupNameTaken
		}
		return nil, err
	}
	return m.GetUserGroup(g.GroupID)
}

// UpdateUserGroup renames a group (case-insensitive unique via collation).
func (m *MySQLStore) UpdateUserGroup(groupID, name string) (*UserGroup, error) {
	name = strings.TrimSpace(name)
	if groupID == "" || name == "" {
		return nil, errors.New("group_id and name required")
	}
	res, err := m.db.Exec(`UPDATE user_groups SET name = ? WHERE group_id = ?`, name, groupID)
	if err != nil {
		if isMySQLDuplicate(err) {
			return nil, ErrUserGroupNameTaken
		}
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// name may be unchanged → still verify exists
		if _, err := m.GetUserGroup(groupID); err != nil {
			return nil, err
		}
	}
	return m.GetUserGroup(groupID)
}

func (m *MySQLStore) GetUserGroup(groupID string) (*UserGroup, error) {
	row := m.db.QueryRow(`SELECT group_id, name, created_at, updated_at FROM user_groups WHERE group_id = ?`, groupID)
	var g UserGroup
	if err := row.Scan(&g.GroupID, &g.Name, &g.CreatedAt, &g.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserGroupNotFound
		}
		return nil, err
	}
	return &g, nil
}

func (m *MySQLStore) DeleteUserGroup(groupID string) error {
	res, err := m.db.Exec(`DELETE FROM user_groups WHERE group_id = ?`, groupID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserGroupNotFound
	}
	return nil
}

func (m *MySQLStore) ListUserGroups() ([]UserGroupBrief, error) {
	rows, err := m.db.Query(`
		SELECT g.group_id, g.name,
			(SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = g.group_id) AS member_count,
			(SELECT COUNT(*) FROM group_models gmod WHERE gmod.group_id = g.group_id) AS model_count
		FROM user_groups g
		ORDER BY g.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserGroupBrief
	for rows.Next() {
		var b UserGroupBrief
		if err := rows.Scan(&b.GroupID, &b.Name, &b.MemberCount, &b.ModelCount); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (m *MySQLStore) ListGroupMemberIDs(groupID string) ([]string, error) {
	rows, err := m.db.Query(`SELECT user_id FROM group_members WHERE group_id = ? ORDER BY user_id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetGroupMembers replaces membership for a group (empty = clear).
func (m *MySQLStore) SetGroupMembers(groupID string, userIDs []string) error {
	if _, err := m.GetUserGroup(groupID); err != nil {
		return err
	}
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM group_members WHERE group_id = ?`, groupID); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, id := range userIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if _, err := tx.Exec(`INSERT INTO group_members (group_id, user_id) VALUES (?, ?)`, groupID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (m *MySQLStore) ListGroupModelIDs(groupID string) ([]string, error) {
	rows, err := m.db.Query(`SELECT model_id FROM group_models WHERE group_id = ? ORDER BY model_id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetGroupModels replaces Group Assignment for a group (empty = clear).
func (m *MySQLStore) SetGroupModels(groupID string, modelIDs []string) error {
	if _, err := m.GetUserGroup(groupID); err != nil {
		return err
	}
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM group_models WHERE group_id = ?`, groupID); err != nil {
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
		if _, err := tx.Exec(`INSERT INTO group_models (group_id, model_id) VALUES (?, ?)`, groupID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListEffectiveModels returns Direct ∪ Group Assignments with source rows.
func (m *MySQLStore) ListEffectiveModels(userID string) (*EffectiveModels, error) {
	out := &EffectiveModels{
		UserID:   userID,
		ModelIDs: []string{},
		Sources:  []EffectiveModelSource{},
	}

	direct, err := m.ListUserModelIDs(userID)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, a := range direct {
		out.Sources = append(out.Sources, EffectiveModelSource{
			ModelID: a.ModelID,
			Via:     "direct",
		})
		if _, ok := seen[a.ModelID]; !ok {
			seen[a.ModelID] = struct{}{}
			out.ModelIDs = append(out.ModelIDs, a.ModelID)
		}
		if a.IsDefault {
			out.DefaultModel = a.ModelID
		}
	}

	rows, err := m.db.Query(`
		SELECT gmod.model_id, g.group_id, g.name
		FROM group_members gm
		INNER JOIN user_groups g ON g.group_id = gm.group_id
		INNER JOIN group_models gmod ON gmod.group_id = gm.group_id
		WHERE gm.user_id = ?
		ORDER BY g.name, gmod.model_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var src EffectiveModelSource
		if err := rows.Scan(&src.ModelID, &src.GroupID, &src.GroupName); err != nil {
			return nil, err
		}
		src.Via = "group"
		out.Sources = append(out.Sources, src)
		if _, ok := seen[src.ModelID]; !ok {
			seen[src.ModelID] = struct{}{}
			out.ModelIDs = append(out.ModelIDs, src.ModelID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func isMySQLDuplicate(err error) bool {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		return me.Number == 1062
	}
	return false
}
