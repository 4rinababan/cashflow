package auth

import "testing"

func TestHashPassword_VerifyPassword_RoundTrip(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !VerifyPassword(hash, "correct-horse-battery") {
		t.Error("VerifyPassword() = false, want true for the correct password")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Error("VerifyPassword() = true, want false for the wrong password")
	}
}

func TestValidatePassword_RejectsShortPasswords(t *testing.T) {
	if err := ValidatePassword("short"); err == nil {
		t.Error(`ValidatePassword("short") error = nil, want ErrPasswordTooShort`)
	}
	if err := ValidatePassword("long-enough-password"); err != nil {
		t.Errorf("ValidatePassword() error = %v, want nil", err)
	}
}
