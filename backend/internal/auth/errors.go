package auth

import "errors"

var (
	// ErrInvalidCredentials covers both "no such user" and "wrong password" —
	// deliberately the same error so a login failure never tells an attacker
	// which one it was.
	ErrInvalidCredentials = errors.New("invalid email or password")
	// ErrInvalidToken covers a missing, malformed, expired, or revoked token
	// (access or refresh) — same reasoning: don't reveal which.
	ErrInvalidToken        = errors.New("invalid or expired token")
	ErrPasswordTooShort    = errors.New("password too short")
	ErrDisplayNameRequired = errors.New("display name is required")
)
