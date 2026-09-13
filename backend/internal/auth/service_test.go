package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"cashflow-backend/internal/users"
)

// ----- in-memory fakes, local to this package's tests -----

type fakeUserStore struct {
	byEmail map[string]users.User
	byID    map[string]users.User
	nextID  int
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{byEmail: map[string]users.User{}, byID: map[string]users.User{}}
}

func (f *fakeUserStore) Create(_ context.Context, u users.User) (users.User, error) {
	if _, exists := f.byEmail[u.Email]; exists {
		return users.User{}, users.ErrEmailTaken
	}
	f.nextID++
	u.ID = fmt.Sprintf("user-%d", f.nextID)
	u.CreatedAt = time.Now()
	f.byEmail[u.Email] = u
	f.byID[u.ID] = u
	return u, nil
}

func (f *fakeUserStore) GetByEmail(_ context.Context, email string) (users.User, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return users.User{}, users.ErrNotFound
}

func (f *fakeUserStore) GetByID(_ context.Context, id string) (users.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return users.User{}, users.ErrNotFound
}

func (f *fakeUserStore) Delete(_ context.Context, id string) error {
	u, ok := f.byID[id]
	if !ok {
		return users.ErrNotFound
	}
	delete(f.byID, id)
	delete(f.byEmail, u.Email)
	return nil
}

type fakeRefreshStore struct {
	byHash map[string]RefreshToken
	nextID int
}

func newFakeRefreshStore() *fakeRefreshStore {
	return &fakeRefreshStore{byHash: map[string]RefreshToken{}}
}

func (f *fakeRefreshStore) Create(_ context.Context, rt RefreshToken) (RefreshToken, error) {
	f.nextID++
	rt.ID = fmt.Sprintf("refresh-%d", f.nextID)
	rt.CreatedAt = time.Now()
	f.byHash[rt.TokenHash] = rt
	return rt, nil
}

func (f *fakeRefreshStore) GetByHash(_ context.Context, hash string) (RefreshToken, error) {
	if rt, ok := f.byHash[hash]; ok {
		return rt, nil
	}
	return RefreshToken{}, ErrInvalidToken
}

func (f *fakeRefreshStore) Revoke(_ context.Context, id string, replacedBy *string) error {
	for hash, rt := range f.byHash {
		if rt.ID == id {
			now := time.Now()
			rt.RevokedAt = &now
			rt.ReplacedBy = replacedBy
			f.byHash[hash] = rt
		}
	}
	return nil
}

func newTestService() (*Service, *fakeUserStore, *fakeRefreshStore) {
	us := newFakeUserStore()
	rs := newFakeRefreshStore()
	return NewService(us, rs, NewTokenIssuer("test-key")), us, rs
}

// ----- Register -----

func TestService_Register_CreatesAUserAndReturnsASession(t *testing.T) {
	svc, _, _ := newTestService()

	result, err := svc.Register(context.Background(), "Ari@Example.com", "correct-horse", "Ari")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if result.User.Email != "ari@example.com" {
		t.Errorf("Email = %q, want normalized ari@example.com", result.User.Email)
	}
	if result.AccessToken == "" || result.RefreshToken == "" {
		t.Error("Register() did not return both an access token and a refresh token")
	}
}

func TestService_Register_RejectsADuplicateEmail(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()

	if _, err := svc.Register(ctx, "dup@example.com", "correct-horse", "A"); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if _, err := svc.Register(ctx, "dup@example.com", "correct-horse", "B"); !errors.Is(err, users.ErrEmailTaken) {
		t.Errorf("second Register() error = %v, want ErrEmailTaken", err)
	}
}

func TestService_Register_RejectsAShortPassword(t *testing.T) {
	svc, _, _ := newTestService()
	if _, err := svc.Register(context.Background(), "a@example.com", "short", "A"); !errors.Is(err, ErrPasswordTooShort) {
		t.Errorf("error = %v, want ErrPasswordTooShort", err)
	}
}

func TestService_Register_RejectsAnEmptyDisplayName(t *testing.T) {
	svc, _, _ := newTestService()
	if _, err := svc.Register(context.Background(), "a@example.com", "correct-horse", "   "); !errors.Is(err, ErrDisplayNameRequired) {
		t.Errorf("error = %v, want ErrDisplayNameRequired", err)
	}
}

// ----- Login -----

func TestService_Login_SucceedsWithCorrectCredentials(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	if _, err := svc.Register(ctx, "a@example.com", "correct-horse", "A"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	result, err := svc.Login(ctx, "A@Example.com", "correct-horse")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.User.Email != "a@example.com" {
		t.Errorf("Email = %q, want a@example.com", result.User.Email)
	}
}

func TestService_Login_FailsWithWrongPassword(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	if _, err := svc.Register(ctx, "a@example.com", "correct-horse", "A"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if _, err := svc.Login(ctx, "a@example.com", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("error = %v, want ErrInvalidCredentials", err)
	}
}

func TestService_Login_FailsForAnUnknownEmailWithTheSameError(t *testing.T) {
	svc, _, _ := newTestService()
	_, err := svc.Login(context.Background(), "nobody@example.com", "whatever1")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("error = %v, want ErrInvalidCredentials (must not reveal the email is unregistered)", err)
	}
}

// ----- Refresh -----

func TestService_Refresh_RotatesTheTokenAndTheOldOneStopsWorking(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	registered, err := svc.Register(ctx, "a@example.com", "correct-horse", "A")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	rotated, err := svc.Refresh(ctx, registered.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if rotated.RefreshToken == registered.RefreshToken {
		t.Error("Refresh() returned the same refresh token instead of a new one")
	}

	if _, err := svc.Refresh(ctx, registered.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("reusing the old (rotated-away) refresh token: error = %v, want ErrInvalidToken", err)
	}

	if _, err := svc.Refresh(ctx, rotated.RefreshToken); err != nil {
		t.Errorf("using the new refresh token: error = %v, want nil", err)
	}
}

func TestService_Refresh_RejectsAnUnknownToken(t *testing.T) {
	svc, _, _ := newTestService()
	if _, err := svc.Refresh(context.Background(), "not-a-real-token"); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("error = %v, want ErrInvalidToken", err)
	}
}

func TestService_Refresh_RejectsAnExpiredToken(t *testing.T) {
	svc, _, rs := newTestService()
	ctx := context.Background()
	registered, err := svc.Register(ctx, "a@example.com", "correct-horse", "A")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	hash := HashToken(registered.RefreshToken)
	expired := rs.byHash[hash]
	expired.ExpiresAt = time.Now().Add(-time.Hour)
	rs.byHash[hash] = expired

	if _, err := svc.Refresh(ctx, registered.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("error = %v, want ErrInvalidToken", err)
	}
}

// ----- Logout -----

func TestService_Logout_RevokesTheRefreshTokenSoItCannotBeUsedAgain(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	registered, err := svc.Register(ctx, "a@example.com", "correct-horse", "A")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if err := svc.Logout(ctx, registered.RefreshToken); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	if _, err := svc.Refresh(ctx, registered.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("refreshing after logout: error = %v, want ErrInvalidToken", err)
	}
}

func TestService_Logout_IsIdempotent(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	registered, err := svc.Register(ctx, "a@example.com", "correct-horse", "A")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if err := svc.Logout(ctx, registered.RefreshToken); err != nil {
		t.Fatalf("first Logout() error = %v", err)
	}
	if err := svc.Logout(ctx, registered.RefreshToken); err != nil {
		t.Errorf("second Logout() error = %v, want nil (idempotent)", err)
	}
}

// ----- CurrentUser (ownership) -----

func TestService_CurrentUser_ReturnsExactlyTheOwningUser(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	a, err := svc.Register(ctx, "a@example.com", "correct-horse", "A")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if _, err := svc.Register(ctx, "b@example.com", "correct-horse", "B"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, err := svc.CurrentUser(ctx, a.User.ID)
	if err != nil {
		t.Fatalf("CurrentUser() error = %v", err)
	}
	if got.Email != "a@example.com" {
		t.Errorf("Email = %q, want a@example.com (not user B's)", got.Email)
	}
}

// ----- DeleteAccount (Phase 8) -----

func TestService_DeleteAccount_RemovesTheUserSoCurrentUserThenFails(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	registered, err := svc.Register(ctx, "a@example.com", "correct-horse", "A")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if err := svc.DeleteAccount(ctx, registered.User.ID); err != nil {
		t.Fatalf("DeleteAccount() error = %v", err)
	}

	if _, err := svc.CurrentUser(ctx, registered.User.ID); !errors.Is(err, users.ErrNotFound) {
		t.Errorf("CurrentUser() after delete: error = %v, want ErrNotFound", err)
	}
}

func TestService_DeleteAccount_DoesNotAffectAnotherUser(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	a, err := svc.Register(ctx, "a@example.com", "correct-horse", "A")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	b, err := svc.Register(ctx, "b@example.com", "correct-horse", "B")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if err := svc.DeleteAccount(ctx, a.User.ID); err != nil {
		t.Fatalf("DeleteAccount() error = %v", err)
	}

	if _, err := svc.CurrentUser(ctx, b.User.ID); err != nil {
		t.Errorf("CurrentUser() for user B after deleting user A: error = %v, want nil", err)
	}
}

func TestService_DeleteAccount_OfAnUnknownIDReturnsNotFound(t *testing.T) {
	svc, _, _ := newTestService()
	if err := svc.DeleteAccount(context.Background(), "no-such-user"); !errors.Is(err, users.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}
