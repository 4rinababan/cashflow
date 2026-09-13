// Package users owns the user record and its storage. It knows nothing about
// passwords, tokens, or HTTP — that's internal/auth, which depends on this
// package, not the other way around.
package users

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"
)

var (
	// ErrEmailTaken is returned by Store.Create when the email is already registered.
	ErrEmailTaken = errors.New("email already registered")
	// ErrNotFound is returned when no user matches the lookup.
	ErrNotFound = errors.New("user not found")
	// ErrInvalidEmail is returned by NormalizeEmail for a malformed address.
	ErrInvalidEmail = errors.New("invalid email address")
)

// User is one account holder. Every future ledger row (accounts, categories,
// transactions, receipts — Phase 3+) is owned by exactly one User.ID.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	DisplayName  string
	CreatedAt    time.Time
	UpdatedAt    *time.Time
}

// NormalizeEmail lower-cases and trims an address and checks it is at least
// well-formed, so uniqueness/lookups are case-insensitive without needing a
// database extension (e.g. citext).
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if _, err := mail.ParseAddress(email); err != nil {
		return "", ErrInvalidEmail
	}
	return email, nil
}

// Store persists users. The Postgres implementation lives in postgres_store.go;
// tests use an in-memory fake so internal/auth's tests don't need a database.
type Store interface {
	Create(ctx context.Context, u User) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
	// Delete removes the user row (Phase 8 self-service account deletion).
	// Every other table a user owns has an ON DELETE CASCADE foreign key
	// back to users(id) — see backend/migrations — so this one delete is
	// enough to remove all of that user's accounts, categories,
	// transactions, refresh tokens, and receipt-scan audit rows too.
	Delete(ctx context.Context, id string) error
}
