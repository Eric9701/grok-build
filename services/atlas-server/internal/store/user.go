package store

import (
	"errors"
	"time"
)

// User is a persisted Atlas account.
type User struct {
	UserID        string
	Email         string
	PasswordHash  string
	FirstName     string
	LastName      string
	PrincipalType string
	PrincipalID   string
	MachineCode   string
	CreatedAt     time.Time
}

// UserProfile is the public user shape returned by /v1/user and APIs.
type UserProfile struct {
	UserID                    string `json:"userId"`
	Email                     string `json:"email"`
	FirstName                 string `json:"firstName"`
	LastName                  string `json:"lastName"`
	PrincipalType             string `json:"principalType"`
	PrincipalID               string `json:"principalId"`
	CodingDataRetentionOptOut bool   `json:"codingDataRetentionOptOut"`
	SubscriptionTier          string `json:"subscriptionTier,omitempty"`
}

// ToProfile converts a User to the CLI-facing profile.
func (u *User) ToProfile() UserProfile {
	return UserProfile{
		UserID:                    u.UserID,
		Email:                     u.Email,
		FirstName:                 u.FirstName,
		LastName:                  u.LastName,
		PrincipalType:             u.PrincipalType,
		PrincipalID:               u.PrincipalID,
		CodingDataRetentionOptOut: false,
		SubscriptionTier:          "SuperGrokPro",
	}
}

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserExists        = errors.New("user already exists")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrSessionNotFound   = errors.New("session not found")
	ErrSessionExpired    = errors.New("session expired")
	ErrMachineCodeInUse  = errors.New("machine code already in use")
	ErrMachineCodeNeeded = errors.New("machine code required")
)

// UserStore persists accounts and web login sessions.
type UserStore interface {
	Migrate() error
	CreateUser(u User) error
	GetUserByEmail(email string) (*User, error)
	GetUserByID(userID string) (*User, error)
	GetUserByMachineCode(code string) (*User, error)
	EnsureBootstrapUser(u User, plainPassword string) error

	CreateSession(userID string, ttl time.Duration) (token string, err error)
	GetUserBySession(token string) (*User, error)
	DeleteSession(token string) error
}
