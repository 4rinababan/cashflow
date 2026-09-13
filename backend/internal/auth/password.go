package auth

import "golang.org/x/crypto/bcrypt"

// bcryptCost trades hashing time for brute-force resistance. 12 is a common
// modern default; raise it later if hashing becomes cheap relative to
// available attacker hardware, not before.
const bcryptCost = 12

// HashPassword returns a salted bcrypt hash suitable for storage.
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword reports whether plain matches the stored bcrypt hash.
func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

const minPasswordLength = 8

// ValidatePassword enforces the minimum bar for a new/changed password.
func ValidatePassword(password string) error {
	if len(password) < minPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}
