// Package auth provides user registration and credential-based
// authentication for the platform.
//
// The surface is intentionally minimal: local username/password only.
// SSO and MFA are out of scope for the initial iteration, but the
// Service API is stable enough to layer them on later. Passwords are
// stored only as PBKDF2-HMAC-SHA256 hashes with a per-user random salt
// (NFR-1); authentication failures are deliberately indistinguishable so
// they never reveal whether a username exists (NFR-2).
//
// Persistence uses a JSON file in the standard library only, keeping the
// initial implementation dependency-free.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Password hashing parameters. The iteration count follows OWASP's
// guidance for PBKDF2-HMAC-SHA256.
const (
	iterations = 210_000
	keyLength  = 32
	saltLength = 16
)

// ErrInvalidCredentials is returned for any failed authentication,
// regardless of whether the username exists (NFR-2).
var ErrInvalidCredentials = errors.New("invalid username or password")

// ErrUserExists is returned when registering a duplicate username.
var ErrUserExists = errors.New("username already exists")

// dummySalt is used on the "user not found" path so a failed login runs
// the same key derivation work whether or not the username exists,
// reducing username enumeration via timing (NFR-2).
var dummySalt = make([]byte, saltLength)

// User is the persisted account record. The password is never stored;
// only its salt and derived hash are.
type User struct {
	Username   string    `json:"username"`
	Salt       string    `json:"salt"` // base64
	Hash       string    `json:"hash"` // base64
	Iterations int       `json:"iterations"`
	CreatedAt  time.Time `json:"created_at"`
}

// Service is the authentication service backed by a JSON file.
type Service struct {
	path  string
	mu    sync.RWMutex
	users map[string]User
}

// New loads (or initializes) a Service whose users are stored at path.
func New(path string) (*Service, error) {
	s := &Service{path: path, users: map[string]User{}}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil // empty store
	}
	if err != nil {
		return err
	}
	var users []User
	if err := json.Unmarshal(data, &users); err != nil {
		return err
	}
	for _, u := range users {
		s.users[u.Username] = u
	}
	return nil
}

// save writes the store atomically (write-then-rename) so a crash never
// leaves a half-written user file.
func (s *Service) save() error {
	users := make([]User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Register creates a new user with a freshly salted PBKDF2 hash.
func (s *Service) Register(username, password string) (*User, error) {
	if username == "" || password == "" {
		return nil, errors.New("username and password are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[username]; exists {
		return nil, ErrUserExists
	}

	salt := make([]byte, saltLength)
	rand.Read(salt)
	hash, err := pbkdf2.Key(sha256.New, password, salt, iterations, keyLength)
	if err != nil {
		return nil, err
	}

	user := User{
		Username:   username,
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Hash:       base64.StdEncoding.EncodeToString(hash),
		Iterations: iterations,
		CreatedAt:  time.Now().UTC(),
	}
	s.users[username] = user
	if err := s.save(); err != nil {
		delete(s.users, username)
		return nil, err
	}
	return &user, nil
}

// EnsureUser creates the user if absent, or leaves it untouched if it
// already exists. Used to seed the administrative account on startup.
func (s *Service) EnsureUser(username, password string) error {
	_, err := s.Register(username, password)
	if errors.Is(err, ErrUserExists) {
		return nil
	}
	return err
}

// Valid reports whether the credentials authenticate successfully. It is
// a boolean convenience over Authenticate for callers (and the /login
// handler) that only need a yes/no answer.
func (s *Service) Valid(username, password string) bool {
	_, err := s.Authenticate(username, password)
	return err == nil
}

// Authenticate verifies a username/password pair. It returns
// ErrInvalidCredentials for both unknown users and wrong passwords so
// the two cases are indistinguishable to a caller (NFR-2).
func (s *Service) Authenticate(username, password string) (*User, error) {
	s.mu.RLock()
	user, ok := s.users[username]
	s.mu.RUnlock()

	if !ok {
		// Equalize timing against the found-user path, then fail.
		_, _ = pbkdf2.Key(sha256.New, password, dummySalt, iterations, keyLength)
		return nil, ErrInvalidCredentials
	}

	salt, err := base64.StdEncoding.DecodeString(user.Salt)
	if err != nil {
		return nil, err
	}
	want, err := base64.StdEncoding.DecodeString(user.Hash)
	if err != nil {
		return nil, err
	}
	iter := user.Iterations
	if iter == 0 {
		iter = iterations
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return nil, ErrInvalidCredentials
	}
	return &user, nil
}
