package users

import "testing"

func TestNormalizeEmail_LowerCasesAndTrims(t *testing.T) {
	got, err := NormalizeEmail("  Ari.Decent@Example.COM  ")
	if err != nil {
		t.Fatalf("NormalizeEmail() error = %v", err)
	}
	if got != "ari.decent@example.com" {
		t.Errorf("NormalizeEmail() = %q, want ari.decent@example.com", got)
	}
}

func TestNormalizeEmail_RejectsMalformedAddresses(t *testing.T) {
	cases := []string{"", "not-an-email", "missing-domain@", "@missing-local.com", "spaces in@email.com"}
	for _, raw := range cases {
		if _, err := NormalizeEmail(raw); err == nil {
			t.Errorf("NormalizeEmail(%q) error = nil, want ErrInvalidEmail", raw)
		}
	}
}
