package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// DeviceStatus is the lifecycle of a device-code session.
type DeviceStatus string

const (
	DevicePending  DeviceStatus = "pending"
	DeviceApproved DeviceStatus = "approved"
	DeviceDenied   DeviceStatus = "denied"
	DeviceExpired  DeviceStatus = "expired"
	DeviceConsumed DeviceStatus = "consumed"
)

// DeviceSession is an RFC 8628 device authorization grant.
type DeviceSession struct {
	DeviceCode              string
	UserCode                string
	ClientID                string
	Scope                   string
	Status                  DeviceStatus
	ExpiresAt               time.Time
	IntervalSecs            int
	UserID                  string
	Email                   string
	VerificationURI         string
	VerificationURIComplete string
}

// RefreshRecord tracks issued refresh tokens.
type RefreshRecord struct {
	Token     string
	UserID    string
	Email     string
	ClientID  string
	Scope     string
	ExpiresAt time.Time
}

// Store is the persistence surface used by auth and future modules.
type Store interface {
	CreateDevice(s DeviceSession) error
	GetByDeviceCode(code string) (*DeviceSession, error)
	GetByUserCode(code string) (*DeviceSession, error)
	ApproveByUserCode(userCode, userID, email string) error
	DenyByUserCode(userCode string) error
	MarkConsumed(deviceCode string) error
	SaveRefresh(r RefreshRecord) error
	GetRefresh(token string) (*RefreshRecord, error)
	RevokeRefresh(token string) error
}

// MemoryStore is an in-process implementation suitable for local mock auth.
type MemoryStore struct {
	mu       sync.RWMutex
	byDevice map[string]*DeviceSession
	byUser   map[string]*DeviceSession
	refresh  map[string]*RefreshRecord
}

// NewMemoryStore constructs an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		byDevice: make(map[string]*DeviceSession),
		byUser:   make(map[string]*DeviceSession),
		refresh:  make(map[string]*RefreshRecord),
	}
}

func (m *MemoryStore) CreateDevice(s DeviceSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := s
	m.byDevice[s.DeviceCode] = &cp
	m.byUser[normalizeUserCode(s.UserCode)] = &cp
	return nil
}

func (m *MemoryStore) GetByDeviceCode(code string) (*DeviceSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.byDevice[code]
	if !ok {
		return nil, fmt.Errorf("device_code not found")
	}
	out := *s
	if time.Now().After(out.ExpiresAt) && out.Status == DevicePending {
		out.Status = DeviceExpired
	}
	return &out, nil
}

func (m *MemoryStore) GetByUserCode(code string) (*DeviceSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.byUser[normalizeUserCode(code)]
	if !ok {
		return nil, fmt.Errorf("user_code not found")
	}
	out := *s
	if time.Now().After(out.ExpiresAt) && out.Status == DevicePending {
		out.Status = DeviceExpired
	}
	return &out, nil
}

func (m *MemoryStore) ApproveByUserCode(userCode, userID, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byUser[normalizeUserCode(userCode)]
	if !ok {
		return fmt.Errorf("user_code not found")
	}
	if time.Now().After(s.ExpiresAt) {
		s.Status = DeviceExpired
		return fmt.Errorf("user_code expired")
	}
	if s.Status != DevicePending {
		return fmt.Errorf("user_code not pending")
	}
	s.Status = DeviceApproved
	s.UserID = userID
	s.Email = email
	return nil
}

func (m *MemoryStore) DenyByUserCode(userCode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byUser[normalizeUserCode(userCode)]
	if !ok {
		return fmt.Errorf("user_code not found")
	}
	s.Status = DeviceDenied
	return nil
}

func (m *MemoryStore) MarkConsumed(deviceCode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.byDevice[deviceCode]
	if !ok {
		return fmt.Errorf("device_code not found")
	}
	s.Status = DeviceConsumed
	return nil
}

func (m *MemoryStore) SaveRefresh(r RefreshRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := r
	m.refresh[r.Token] = &cp
	return nil
}

func (m *MemoryStore) GetRefresh(token string) (*RefreshRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.refresh[token]
	if !ok {
		return nil, ErrRefreshNotFound
	}
	if time.Now().After(r.ExpiresAt) {
		return nil, ErrRefreshExpired
	}
	out := *r
	return &out, nil
}

func (m *MemoryStore) RevokeRefresh(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.refresh, token)
	return nil
}

func normalizeUserCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// RandomHex returns n random bytes as hex.
func RandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// RandomUserCode returns an RFC8628-style user code like ABCD-EFGH.
func RandomUserCode() (string, error) {
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
