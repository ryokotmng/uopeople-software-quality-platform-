package auth

import (
	"errors"
	"path/filepath"
	"testing"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := New(filepath.Join(t.TempDir(), "users.json"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc
}

func TestRegisterThenAuthenticateSucceeds(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Register("alice", "correct-horse"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, err := svc.Authenticate("alice", "correct-horse"); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
}

func TestAuthenticateValid(t *testing.T) {
	svc := newTestService(t)
	svc.Register("alice", "correct-horse")
	if !svc.Valid("alice", "correct-horse") {
		t.Fatal("Valid() = false for correct credentials, want true")
	}
}

func TestAuthenticateInvalid(t *testing.T) {
	svc := newTestService(t)
	svc.Register("alice", "correct-horse")
	if svc.Valid("alice", "wrong-password") {
		t.Error("Valid() = true for a wrong password, want false")
	}
	if svc.Valid("ghost", "whatever") {
		t.Error("Valid() = true for an unknown user, want false")
	}
}

func TestAuthenticateWrongPasswordFails(t *testing.T) {
	svc := newTestService(t)
	svc.Register("alice", "correct-horse")
	if _, err := svc.Authenticate("alice", "battery-staple"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthenticateUnknownUserIsIndistinguishable(t *testing.T) {
	svc := newTestService(t)
	// An unknown username must fail with the same error as a wrong
	// password, never revealing that the account does not exist (NFR-2).
	if _, err := svc.Authenticate("nobody", "whatever"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestRegisterDuplicateFails(t *testing.T) {
	svc := newTestService(t)
	svc.Register("alice", "correct-horse")
	if _, err := svc.Register("alice", "another"); !errors.Is(err, ErrUserExists) {
		t.Fatalf("want ErrUserExists, got %v", err)
	}
}

func TestUsersPersistAcrossReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")

	svc1, _ := New(path)
	svc1.Register("alice", "correct-horse")

	svc2, err := New(path) // reload from disk
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, err := svc2.Authenticate("alice", "correct-horse"); err != nil {
		t.Fatalf("Authenticate after reload: %v", err)
	}
}
