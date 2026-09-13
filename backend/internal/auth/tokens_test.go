package auth

import (
	"testing"
	"time"
)

func TestTokenIssuer_IssueAndVerifyAccessToken(t *testing.T) {
	issuer := NewTokenIssuer("test-key")

	token, expiresAt, err := issuer.IssueAccessToken("user-1")
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}
	if !expiresAt.After(time.Now()) {
		t.Error("expiresAt is not in the future")
	}

	subject, err := issuer.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("VerifyAccessToken() error = %v", err)
	}
	if subject != "user-1" {
		t.Errorf("subject = %q, want user-1", subject)
	}
}

func TestTokenIssuer_RejectsATokenSignedWithADifferentKey(t *testing.T) {
	tokenFromOther, _, _ := NewTokenIssuer("other-key").IssueAccessToken("user-1")

	if _, err := NewTokenIssuer("test-key").VerifyAccessToken(tokenFromOther); err == nil {
		t.Error("VerifyAccessToken() error = nil, want an error for a token signed with a different key")
	}
}

func TestTokenIssuer_RejectsAnExpiredToken(t *testing.T) {
	issuer := NewTokenIssuer("test-key")
	issuer.now = func() time.Time { return time.Now().Add(-2 * AccessTokenTTL) }

	expired, _, err := issuer.IssueAccessToken("user-1")
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	issuer.now = time.Now
	if _, err := issuer.VerifyAccessToken(expired); err == nil {
		t.Error("VerifyAccessToken() error = nil, want an error for an expired token")
	}
}

func TestTokenIssuer_RejectsGarbage(t *testing.T) {
	if _, err := NewTokenIssuer("test-key").VerifyAccessToken("not-a-jwt"); err == nil {
		t.Error("VerifyAccessToken() error = nil, want an error for garbage input")
	}
}

func TestNewRefreshToken_ProducesUniqueValuesAndAMatchingHash(t *testing.T) {
	raw1, hash1, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken() error = %v", err)
	}
	raw2, hash2, err := NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken() error = %v", err)
	}

	if raw1 == raw2 {
		t.Error("two calls to NewRefreshToken() produced the same raw token")
	}
	if hash1 != HashToken(raw1) {
		t.Error("hash does not match HashToken(raw)")
	}
	if hash1 == hash2 {
		t.Error("two different raw tokens hashed to the same value")
	}
}

func TestGenerateSigningKey_ProducesUniqueValues(t *testing.T) {
	a, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey() error = %v", err)
	}
	b, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey() error = %v", err)
	}
	if a == b {
		t.Error("two calls to GenerateSigningKey() produced the same key")
	}
}
