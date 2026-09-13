package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAuth_AllowsAValidTokenAndSetsTheUserIDInContext(t *testing.T) {
	issuer := NewTokenIssuer("test-key")
	token, _, err := issuer.IssueAccessToken("user-42")
	if err != nil {
		t.Fatalf("IssueAccessToken() error = %v", err)
	}

	var gotUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, _ = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	RequireAuth(issuer)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotUserID != "user-42" {
		t.Errorf("user id in context = %q, want user-42", gotUserID)
	}
}

func TestRequireAuth_RejectsAMissingAuthorizationHeader(t *testing.T) {
	issuer := NewTokenIssuer("test-key")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run")
	})

	rec := httptest.NewRecorder()
	RequireAuth(issuer)(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_RejectsAnInvalidToken(t *testing.T) {
	issuer := NewTokenIssuer("test-key")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer garbage")
	rec := httptest.NewRecorder()

	RequireAuth(issuer)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuth_RejectsANonBearerAuthorizationHeader(t *testing.T) {
	issuer := NewTokenIssuer("test-key")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()

	RequireAuth(issuer)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
