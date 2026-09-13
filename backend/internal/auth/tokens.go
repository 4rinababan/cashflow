package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// AccessTokenTTL is intentionally short — a stolen access token is only
	// useful for this long. RefreshTokenTTL is long because it's what the app
	// actually holds onto between logins, and rotates on every use.
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 30 * 24 * time.Hour
)

// AccessClaims is the JWT payload for an access token. Subject is the user's ID.
type AccessClaims struct {
	jwt.RegisteredClaims
}

// TokenIssuer signs and verifies access tokens (JWT/HS256) and generates
// opaque refresh tokens. now is overridable in tests to exercise expiry.
type TokenIssuer struct {
	signingKey []byte
	now        func() time.Time
}

func NewTokenIssuer(signingKey string) *TokenIssuer {
	return &TokenIssuer{signingKey: []byte(signingKey), now: time.Now}
}

// IssueAccessToken signs a short-lived JWT for userID.
func (t *TokenIssuer) IssueAccessToken(userID string) (token string, expiresAt time.Time, err error) {
	now := t.now().UTC()
	expiresAt = now.Add(AccessTokenTTL)

	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.signingKey)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// VerifyAccessToken returns the subject (user ID) of a valid, unexpired token
// signed with this issuer's key, or ErrInvalidToken.
func (t *TokenIssuer) VerifyAccessToken(tokenString string) (string, error) {
	claims := &AccessClaims{}
	parsed, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return t.signingKey, nil
	})
	if err != nil || !parsed.Valid {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}

// NewRefreshToken returns a random opaque token for the client (raw) and its
// sha256 hash for storage — the raw value is never persisted, so a database
// leak does not hand out usable refresh tokens.
func NewRefreshToken() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashToken(raw), nil
}

// HashToken is the storage form of a raw refresh token.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// GenerateSigningKey returns a random key suitable for JWT_SIGNING_KEY. It
// exists for local development convenience (config.Load allows an empty key
// in "development") — production must set a real, persisted value, since a
// key generated at process start invalidates every access token (and would
// have to be shared across replicas) on every restart.
func GenerateSigningKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
