package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"cashflow-backend/internal/auth"
	"cashflow-backend/internal/ledger"
	"cashflow-backend/internal/mail"
	"cashflow-backend/internal/receipts"
	"cashflow-backend/internal/reports"
	"cashflow-backend/internal/users"
	"cashflow-backend/internal/workspaces"
)

// ----- minimal in-memory fakes, just enough to exercise routing/wiring;
// internal/auth's own tests cover the business logic in depth. -----

type memoryUserStore struct {
	byEmail map[string]users.User
	byID    map[string]users.User
	nextID  int
}

func newMemoryUserStore() *memoryUserStore {
	return &memoryUserStore{byEmail: map[string]users.User{}, byID: map[string]users.User{}}
}

func (m *memoryUserStore) Create(_ context.Context, u users.User) (users.User, error) {
	if _, exists := m.byEmail[u.Email]; exists {
		return users.User{}, users.ErrEmailTaken
	}
	m.nextID++
	u.ID = fmt.Sprintf("user-%d", m.nextID)
	u.CreatedAt = time.Now()
	m.byEmail[u.Email] = u
	m.byID[u.ID] = u
	return u, nil
}

func (m *memoryUserStore) GetByEmail(_ context.Context, email string) (users.User, error) {
	if u, ok := m.byEmail[email]; ok {
		return u, nil
	}
	return users.User{}, users.ErrNotFound
}

func (m *memoryUserStore) GetByID(_ context.Context, id string) (users.User, error) {
	if u, ok := m.byID[id]; ok {
		return u, nil
	}
	return users.User{}, users.ErrNotFound
}

func (m *memoryUserStore) Delete(_ context.Context, id string) error {
	u, ok := m.byID[id]
	if !ok {
		return users.ErrNotFound
	}
	delete(m.byID, id)
	delete(m.byEmail, u.Email)
	return nil
}

type memoryRefreshStore struct {
	byHash map[string]auth.RefreshToken
	nextID int
}

func newMemoryRefreshStore() *memoryRefreshStore {
	return &memoryRefreshStore{byHash: map[string]auth.RefreshToken{}}
}

func (m *memoryRefreshStore) Create(_ context.Context, rt auth.RefreshToken) (auth.RefreshToken, error) {
	m.nextID++
	rt.ID = fmt.Sprintf("refresh-%d", m.nextID)
	rt.CreatedAt = time.Now()
	m.byHash[rt.TokenHash] = rt
	return rt, nil
}

func (m *memoryRefreshStore) GetByHash(_ context.Context, hash string) (auth.RefreshToken, error) {
	if rt, ok := m.byHash[hash]; ok {
		return rt, nil
	}
	return auth.RefreshToken{}, auth.ErrInvalidToken
}

func (m *memoryRefreshStore) Revoke(_ context.Context, id string, replacedBy *string) error {
	for hash, rt := range m.byHash {
		if rt.ID == id {
			now := time.Now()
			rt.RevokedAt = &now
			rt.ReplacedBy = replacedBy
			m.byHash[hash] = rt
		}
	}
	return nil
}

// ----- minimal in-memory workspace fakes: internal/workspaces' own tests
// (once added) would cover Service's own rules; these just need to satisfy
// the store interfaces so the router can be wired end to end, including a
// real join against memoryUserStore for member email/display name, the same
// shape as the real Postgres store's JOIN. -----

type memoryWorkspaceDB struct {
	users      *memoryUserStore
	workspaces map[string]workspaces.Workspace
	members    []workspaces.Member
	invites    map[string]workspaces.Invite
	nextID     int
}

func newMemoryWorkspaceDB(userStore *memoryUserStore) *memoryWorkspaceDB {
	return &memoryWorkspaceDB{
		users:      userStore,
		workspaces: map[string]workspaces.Workspace{},
		invites:    map[string]workspaces.Invite{},
	}
}

func (d *memoryWorkspaceDB) genID(prefix string) string {
	d.nextID++
	return fmt.Sprintf("%s-%d", prefix, d.nextID)
}

type memoryWorkspaceStore struct{ db *memoryWorkspaceDB }

func (s *memoryWorkspaceStore) Create(_ context.Context, w workspaces.Workspace) (workspaces.Workspace, error) {
	w.ID = s.db.genID("ws")
	w.CreatedAt = time.Now()
	s.db.workspaces[w.ID] = w
	return w, nil
}

func (s *memoryWorkspaceStore) GetByID(_ context.Context, id string) (workspaces.Workspace, error) {
	w, ok := s.db.workspaces[id]
	if !ok {
		return workspaces.Workspace{}, workspaces.ErrNotFound
	}
	return w, nil
}

func (s *memoryWorkspaceStore) UpdateName(_ context.Context, id, name string) (workspaces.Workspace, error) {
	w, ok := s.db.workspaces[id]
	if !ok {
		return workspaces.Workspace{}, workspaces.ErrNotFound
	}
	w.Name = name
	s.db.workspaces[id] = w
	return w, nil
}

func (s *memoryWorkspaceStore) Delete(_ context.Context, id string) error {
	delete(s.db.workspaces, id)
	kept := s.db.members[:0]
	for _, m := range s.db.members {
		if m.WorkspaceID != id {
			kept = append(kept, m)
		}
	}
	s.db.members = kept
	return nil
}

type memoryMemberStore struct{ db *memoryWorkspaceDB }

func (s *memoryMemberStore) Add(_ context.Context, m workspaces.Member) error {
	m.CreatedAt = time.Now()
	s.db.members = append(s.db.members, m)
	return nil
}

func (s *memoryMemberStore) Remove(_ context.Context, workspaceID, userID string) error {
	for i, m := range s.db.members {
		if m.WorkspaceID == workspaceID && m.UserID == userID {
			s.db.members = append(s.db.members[:i], s.db.members[i+1:]...)
			return nil
		}
	}
	return workspaces.ErrNotFound
}

func (s *memoryMemberStore) Get(_ context.Context, workspaceID, userID string) (workspaces.Member, error) {
	for _, m := range s.db.members {
		if m.WorkspaceID == workspaceID && m.UserID == userID {
			return m, nil
		}
	}
	return workspaces.Member{}, workspaces.ErrNotFound
}

func (s *memoryMemberStore) ListForWorkspace(_ context.Context, workspaceID string) ([]workspaces.MemberWithUser, error) {
	var result []workspaces.MemberWithUser
	for _, m := range s.db.members {
		if m.WorkspaceID != workspaceID {
			continue
		}
		u, ok := s.db.users.byID[m.UserID]
		if !ok {
			continue
		}
		result = append(result, workspaces.MemberWithUser{Member: m, Email: u.Email, DisplayName: u.DisplayName})
	}
	return result, nil
}

func (s *memoryMemberStore) ListForUser(_ context.Context, userID string) ([]workspaces.WorkspaceWithRole, error) {
	var result []workspaces.WorkspaceWithRole
	for _, m := range s.db.members {
		if m.UserID != userID {
			continue
		}
		w, ok := s.db.workspaces[m.WorkspaceID]
		if !ok {
			continue
		}
		result = append(result, workspaces.WorkspaceWithRole{Workspace: w, Role: m.Role})
	}
	return result, nil
}

type memoryInviteStore struct{ db *memoryWorkspaceDB }

func (s *memoryInviteStore) Create(_ context.Context, i workspaces.Invite) (workspaces.Invite, error) {
	i.ID = s.db.genID("invite")
	i.CreatedAt = time.Now()
	s.db.invites[i.TokenHash] = i
	return i, nil
}

func (s *memoryInviteStore) GetByTokenHash(_ context.Context, tokenHash string) (workspaces.Invite, error) {
	i, ok := s.db.invites[tokenHash]
	if !ok {
		return workspaces.Invite{}, workspaces.ErrNotFound
	}
	return i, nil
}

func (s *memoryInviteStore) MarkAccepted(_ context.Context, id string) error {
	for hash, i := range s.db.invites {
		if i.ID == id {
			now := time.Now()
			i.AcceptedAt = &now
			s.db.invites[hash] = i
			return nil
		}
	}
	return workspaces.ErrNotFound
}

// fakeMailer records every message it's asked to send instead of opening a
// real network connection — internal/mail's own tests already prove the real
// SMTPMailer's wire protocol against a fake SMTP server.
type fakeMailer struct{ sent []mail.Message }

func (m *fakeMailer) Send(_ context.Context, msg mail.Message) error {
	m.sent = append(m.sent, msg)
	return nil
}

// inviteTokenFromEmail extracts the raw invite token from the "/invites/{token}"
// link inviteEmail() builds — mirroring what a real invitee would get by
// clicking the link in their inbox.
func inviteTokenFromEmail(t *testing.T, body string) string {
	t.Helper()
	const marker = "/invites/"
	idx := strings.Index(body, marker)
	if idx == -1 {
		t.Fatalf("invite email body has no /invites/ link: %s", body)
	}
	rest := body[idx+len(marker):]
	end := strings.IndexAny(rest, `")`)
	if end == -1 {
		t.Fatalf("could not find end of invite token in: %s", rest)
	}
	return rest[:end]
}

// ----- minimal in-memory ledger fakes: internal/ledger's own tests cover the
// business logic; these just need to satisfy the store interfaces so the
// router can be wired end to end. -----

type memoryLedgerDB struct {
	accounts     map[string]ledger.Account
	categories   map[string]ledger.Category
	transactions map[string]ledger.Transaction
	nextID       int
}

func newMemoryLedgerDB() *memoryLedgerDB {
	return &memoryLedgerDB{
		accounts:     map[string]ledger.Account{},
		categories:   map[string]ledger.Category{},
		transactions: map[string]ledger.Transaction{},
	}
}

func (d *memoryLedgerDB) genID(prefix string) string {
	d.nextID++
	return fmt.Sprintf("%s-%d", prefix, d.nextID)
}

type memoryAccountStore struct{ db *memoryLedgerDB }

func (s *memoryAccountStore) Create(_ context.Context, a ledger.Account) (ledger.Account, error) {
	a.ID = s.db.genID("acct")
	a.CreatedAt = time.Now()
	s.db.accounts[a.ID] = a
	return a, nil
}
func (s *memoryAccountStore) GetByID(_ context.Context, workspaceID, id string) (ledger.Account, error) {
	a, ok := s.db.accounts[id]
	if !ok || a.WorkspaceID != workspaceID {
		return ledger.Account{}, ledger.ErrNotFound
	}
	return a, nil
}
func (s *memoryAccountStore) ListByWorkspace(_ context.Context, workspaceID string) ([]ledger.AccountBalance, error) {
	var result []ledger.AccountBalance
	for _, a := range s.db.accounts {
		if a.WorkspaceID == workspaceID {
			result = append(result, ledger.AccountBalance{Account: a})
		}
	}
	return result, nil
}
func (s *memoryAccountStore) GetBalance(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (s *memoryAccountStore) Update(_ context.Context, a ledger.Account) (ledger.Account, error) {
	if _, ok := s.db.accounts[a.ID]; !ok {
		return ledger.Account{}, ledger.ErrNotFound
	}
	s.db.accounts[a.ID] = a
	return a, nil
}
func (s *memoryAccountStore) Delete(_ context.Context, workspaceID, id string) error {
	a, ok := s.db.accounts[id]
	if !ok || a.WorkspaceID != workspaceID {
		return ledger.ErrNotFound
	}
	delete(s.db.accounts, id)
	return nil
}
func (s *memoryAccountStore) HasTransactions(context.Context, string, string) (bool, error) {
	return false, nil
}

type memoryCategoryStore struct{ db *memoryLedgerDB }

func (s *memoryCategoryStore) Create(_ context.Context, c ledger.Category) (ledger.Category, error) {
	c.ID = s.db.genID("cat")
	c.CreatedAt = time.Now()
	s.db.categories[c.ID] = c
	return c, nil
}
func (s *memoryCategoryStore) CreateMany(ctx context.Context, cs []ledger.Category) error {
	for _, c := range cs {
		if _, err := s.Create(ctx, c); err != nil {
			return err
		}
	}
	return nil
}
func (s *memoryCategoryStore) GetByID(_ context.Context, workspaceID, id string) (ledger.Category, error) {
	c, ok := s.db.categories[id]
	if !ok || c.WorkspaceID != workspaceID {
		return ledger.Category{}, ledger.ErrNotFound
	}
	return c, nil
}
func (s *memoryCategoryStore) ListByWorkspace(_ context.Context, workspaceID string) ([]ledger.Category, error) {
	var result []ledger.Category
	for _, c := range s.db.categories {
		if c.WorkspaceID == workspaceID {
			result = append(result, c)
		}
	}
	return result, nil
}
func (s *memoryCategoryStore) Update(_ context.Context, c ledger.Category) (ledger.Category, error) {
	if _, ok := s.db.categories[c.ID]; !ok {
		return ledger.Category{}, ledger.ErrNotFound
	}
	s.db.categories[c.ID] = c
	return c, nil
}
func (s *memoryCategoryStore) Delete(_ context.Context, workspaceID, id string) error {
	c, ok := s.db.categories[id]
	if !ok || c.WorkspaceID != workspaceID {
		return ledger.ErrNotFound
	}
	delete(s.db.categories, id)
	return nil
}
func (s *memoryCategoryStore) HasTransactions(context.Context, string, string) (bool, error) {
	return false, nil
}

type memoryTransactionStore struct{ db *memoryLedgerDB }

func (s *memoryTransactionStore) Create(_ context.Context, t ledger.Transaction) (ledger.Transaction, error) {
	t.ID = s.db.genID("txn")
	t.CreatedAt = time.Now()
	s.db.transactions[t.ID] = t
	return t, nil
}
func (s *memoryTransactionStore) GetByID(_ context.Context, workspaceID, id string) (ledger.Transaction, error) {
	txn, ok := s.db.transactions[id]
	if !ok || txn.WorkspaceID != workspaceID {
		return ledger.Transaction{}, ledger.ErrNotFound
	}
	return txn, nil
}
func (s *memoryTransactionStore) ListByWorkspace(_ context.Context, workspaceID string, _ ledger.TransactionFilter) ([]ledger.Transaction, int, error) {
	var result []ledger.Transaction
	for _, txn := range s.db.transactions {
		if txn.WorkspaceID == workspaceID {
			result = append(result, txn)
		}
	}
	return result, len(result), nil
}
func (s *memoryTransactionStore) Update(_ context.Context, t ledger.Transaction) (ledger.Transaction, error) {
	if _, ok := s.db.transactions[t.ID]; !ok {
		return ledger.Transaction{}, ledger.ErrNotFound
	}
	s.db.transactions[t.ID] = t
	return t, nil
}
func (s *memoryTransactionStore) SoftDelete(_ context.Context, workspaceID, id string) error {
	txn, ok := s.db.transactions[id]
	if !ok || txn.WorkspaceID != workspaceID {
		return ledger.ErrNotFound
	}
	now := time.Now()
	txn.DeletedAt = &now
	s.db.transactions[id] = txn
	return nil
}

// ----- minimal in-memory receipts fakes: internal/receipts' own tests cover
// the business logic and the AI endpoint request/response handling. -----

type memoryScanStore struct {
	records []receipts.ScanRecord
	times   []time.Time
}

func (s *memoryScanStore) CountSince(_ context.Context, userID string, since time.Time) (int, error) {
	count := 0
	for i, r := range s.records {
		if r.UserID == userID && !s.times[i].Before(since) {
			count++
		}
	}
	return count, nil
}

func (s *memoryScanStore) Record(_ context.Context, r receipts.ScanRecord) error {
	s.records = append(s.records, r)
	s.times = append(s.times, time.Now())
	return nil
}

type stubScanner struct {
	draft receipts.Draft
	err   error
}

func (s *stubScanner) Scan(context.Context, []byte, string) (receipts.Draft, error) {
	return s.draft, s.err
}

// ----- minimal in-memory reports fake: internal/reports' own tests cover
// Service's default-range/validation logic; this wraps the SAME
// memoryLedgerDB the ledger fakes write to (just like the real Postgres
// store reads the real ledger tables), so a router-level test can prove
// reports reflect actual posted transactions end to end. -----

type memoryReportsStore struct{ db *memoryLedgerDB }

func (s *memoryReportsStore) Summary(_ context.Context, workspaceID string, from, to time.Time) (reports.Summary, error) {
	summary := reports.Summary{From: from, To: to}
	for _, tx := range s.db.transactions {
		if tx.WorkspaceID != workspaceID || tx.DeletedAt != nil {
			continue
		}
		if tx.OccurredAt.Before(from) || tx.OccurredAt.After(to) {
			continue
		}
		if tx.Direction == ledger.DirectionIncome {
			summary.IncomeMinor += tx.AmountMinor
		} else {
			summary.ExpenseMinor += tx.AmountMinor
		}
		summary.TransactionCount++
	}
	return summary, nil
}

func (s *memoryReportsStore) ByCategory(_ context.Context, workspaceID string, from, to time.Time) ([]reports.CategoryBreakdown, error) {
	totals := map[string]*reports.CategoryBreakdown{}
	for _, tx := range s.db.transactions {
		if tx.WorkspaceID != workspaceID || tx.DeletedAt != nil {
			continue
		}
		if tx.OccurredAt.Before(from) || tx.OccurredAt.After(to) {
			continue
		}
		cat, ok := s.db.categories[tx.CategoryID]
		if !ok {
			continue
		}
		b, exists := totals[tx.CategoryID]
		if !exists {
			b = &reports.CategoryBreakdown{CategoryID: cat.ID, CategoryName: cat.Name, Kind: string(cat.Kind)}
			totals[tx.CategoryID] = b
		}
		b.TotalMinor += tx.AmountMinor
		b.TransactionCount++
	}
	result := make([]reports.CategoryBreakdown, 0, len(totals))
	for _, b := range totals {
		result = append(result, *b)
	}
	return result, nil
}

func (s *memoryReportsStore) MonthlyTrend(_ context.Context, workspaceID string, _ []string, rangeStart, rangeEnd time.Time) (map[string]reports.MonthlyTrendPoint, error) {
	result := map[string]reports.MonthlyTrendPoint{}
	for _, tx := range s.db.transactions {
		if tx.WorkspaceID != workspaceID || tx.DeletedAt != nil {
			continue
		}
		if tx.OccurredAt.Before(rangeStart) || !tx.OccurredAt.Before(rangeEnd) {
			continue
		}
		label := tx.OccurredAt.Format("2006-01")
		p := result[label]
		p.Month = label
		if tx.Direction == ledger.DirectionIncome {
			p.IncomeMinor += tx.AmountMinor
		} else {
			p.ExpenseMinor += tx.AmountMinor
		}
		result[label] = p
	}
	return result, nil
}

func newTestDeps(pinger Pinger) Deps {
	return newTestDepsWithScanLimit(pinger, 10)
}

func newTestDepsWithMailer(pinger Pinger, mailer mail.Mailer) (Deps, *memoryWorkspaceDB) {
	tokens := auth.NewTokenIssuer("test-signing-key-not-for-production")
	userStore := newMemoryUserStore()
	authService := auth.NewService(userStore, newMemoryRefreshStore(), tokens)

	ledgerDB := newMemoryLedgerDB()
	ledgerService := ledger.NewService(&memoryAccountStore{ledgerDB}, &memoryCategoryStore{ledgerDB}, &memoryTransactionStore{ledgerDB})

	wsDB := newMemoryWorkspaceDB(userStore)
	workspaceService := workspaces.NewService(
		&memoryWorkspaceStore{wsDB}, &memoryMemberStore{wsDB}, &memoryInviteStore{wsDB}, mailer,
	)
	workspaceService.AppBaseURL = "https://app.cashflow.test"

	authService.SetAfterRegister(func(ctx context.Context, userID, displayName string) {
		ws, err := workspaceService.CreatePersonalWorkspace(ctx, userID, displayName)
		if err != nil {
			return
		}
		_ = ledgerService.SeedDefaultCategories(ctx, ws.ID, userID)
	})
	authService.SetBeforeDeleteAccount(workspaceService.RemoveUserFromAllWorkspaces)

	scanner := &stubScanner{draft: receipts.Draft{Merchant: "Test Merchant", TotalMinor: 1000, Currency: "IDR", Confidence: 0.9}}
	receiptService := receipts.NewService(scanner, &memoryScanStore{}, 10)

	return Deps{
		Pinger:           pinger,
		AuthService:      authService,
		TokenIssuer:      tokens,
		LedgerService:    ledgerService,
		ReceiptService:   receiptService,
		ReportsService:   reports.NewService(&memoryReportsStore{db: ledgerDB}),
		WorkspaceService: workspaceService,
	}, wsDB
}

func newTestDepsWithScanLimit(pinger Pinger, dailyScanLimit int) Deps {
	tokens := auth.NewTokenIssuer("test-signing-key-not-for-production")
	userStore := newMemoryUserStore()
	authService := auth.NewService(userStore, newMemoryRefreshStore(), tokens)

	ledgerDB := newMemoryLedgerDB()
	ledgerService := ledger.NewService(&memoryAccountStore{ledgerDB}, &memoryCategoryStore{ledgerDB}, &memoryTransactionStore{ledgerDB})

	wsDB := newMemoryWorkspaceDB(userStore)
	workspaceService := workspaces.NewService(
		&memoryWorkspaceStore{wsDB}, &memoryMemberStore{wsDB}, &memoryInviteStore{wsDB}, &fakeMailer{},
	)

	authService.SetAfterRegister(func(ctx context.Context, userID, displayName string) {
		ws, err := workspaceService.CreatePersonalWorkspace(ctx, userID, displayName)
		if err != nil {
			return
		}
		_ = ledgerService.SeedDefaultCategories(ctx, ws.ID, userID)
	})
	authService.SetBeforeDeleteAccount(workspaceService.RemoveUserFromAllWorkspaces)

	scanner := &stubScanner{draft: receipts.Draft{Merchant: "Test Merchant", TotalMinor: 1000, Currency: "IDR", Confidence: 0.9}}
	receiptService := receipts.NewService(scanner, &memoryScanStore{}, dailyScanLimit)

	return Deps{
		Pinger:           pinger,
		AuthService:      authService,
		TokenIssuer:      tokens,
		LedgerService:    ledgerService,
		ReceiptService:   receiptService,
		ReportsService:   reports.NewService(&memoryReportsStore{db: ledgerDB}),
		WorkspaceService: workspaceService,
	}
}

func TestRouter_RegisterThenMeReturnsTheSameUser(t *testing.T) {
	deps := newTestDeps(fakePinger{})
	router := NewRouter(deps)

	registerBody, _ := json.Marshal(map[string]string{
		"email": "ari@example.com", "password": "correct-horse", "displayName": "Ari",
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(registerBody)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var registerResp struct {
		AccessToken string `json:"accessToken"`
		User        struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &registerResp); err != nil {
		t.Fatalf("decode register response: %v", err)
	}

	meRec := httptest.NewRecorder()
	meReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+registerResp.AccessToken)
	router.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("/auth/me status = %d, want %d, body=%s", meRec.Code, http.StatusOK, meRec.Body.String())
	}
	if !bytesContains(meRec.Body.Bytes(), "ari@example.com") {
		t.Errorf("/auth/me body = %s, want it to contain the registered email", meRec.Body.String())
	}
}

func TestRouter_MeWithoutATokenIsUnauthorized(t *testing.T) {
	router := NewRouter(newTestDeps(fakePinger{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/me", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRouter_MeWithAnotherUsersTokenNeverReturnsTheWrongUser(t *testing.T) {
	deps := newTestDeps(fakePinger{})
	router := NewRouter(deps)

	registerAs := func(email string) string {
		body, _ := json.Marshal(map[string]string{"email": email, "password": "correct-horse", "displayName": "Name"})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body)))
		var resp struct {
			AccessToken string `json:"accessToken"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		return resp.AccessToken
	}

	tokenA := registerAs("a@example.com")
	_ = registerAs("b@example.com")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+tokenA)
	router.ServeHTTP(rec, req)

	if bytesContains(rec.Body.Bytes(), "b@example.com") {
		t.Fatalf("user A's token returned user B's data: %s", rec.Body.String())
	}
	if !bytesContains(rec.Body.Bytes(), "a@example.com") {
		t.Fatalf("expected user A's own email, got: %s", rec.Body.String())
	}
}

func bytesContains(haystack []byte, needle string) bool {
	return bytes.Contains(haystack, []byte(needle))
}

// registerAndGetWorkspace registers a new user and returns their access token
// plus the ID of the personal workspace auto-created at registration (every
// brand-new user has exactly one at this point).
func registerAndGetWorkspace(t *testing.T, router http.Handler, email string) (token, workspaceID string) {
	t.Helper()
	token = registerAndGetToken(t, router, email)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/workspaces", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /workspaces status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var list []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode workspaces: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(workspaces) = %d, want exactly 1 personal workspace right after registration", len(list))
	}
	return token, list[0].ID
}

func registerAndGetToken(t *testing.T, router http.Handler, email string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": "correct-horse", "displayName": "Name"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	return resp.AccessToken
}

func TestRouter_RegisterSeedsDefaultCategories(t *testing.T) {
	router := NewRouter(newTestDeps(fakePinger{}))
	token, workspaceID := registerAndGetWorkspace(t, router, "seed@example.com")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/workspaces/"+workspaceID+"/categories", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET .../categories status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var categories []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &categories); err != nil {
		t.Fatalf("decode categories: %v", err)
	}
	if len(categories) != 6 {
		t.Errorf("len(categories) = %d, want 6 default categories seeded at registration", len(categories))
	}
}

func TestRouter_AccountsAreIsolatedPerWorkspace(t *testing.T) {
	router := NewRouter(newTestDeps(fakePinger{}))
	tokenA, wsA := registerAndGetWorkspace(t, router, "a@example.com")
	tokenB, wsB := registerAndGetWorkspace(t, router, "b@example.com")

	createAccount := func(token, workspaceID, name string) {
		body, _ := json.Marshal(map[string]string{"name": name, "currency": "IDR"})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/workspaces/"+workspaceID+"/accounts", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("POST .../accounts status = %d, body=%s", rec.Code, rec.Body.String())
		}
	}
	createAccount(tokenA, wsA, "A's Cash")
	createAccount(tokenB, wsB, "B's Cash")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/workspaces/"+wsA+"/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+tokenA)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET .../accounts status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if bytesContains(rec.Body.Bytes(), "B's Cash") {
		t.Fatalf("workspace A's account list leaked workspace B's account: %s", rec.Body.String())
	}
	if !bytesContains(rec.Body.Bytes(), "A's Cash") {
		t.Fatalf("workspace A's account list is missing its own account: %s", rec.Body.String())
	}
}

func TestRouter_GetAccountForAnotherWorkspaceReturns404(t *testing.T) {
	router := NewRouter(newTestDeps(fakePinger{}))
	tokenA, wsA := registerAndGetWorkspace(t, router, "a2@example.com")
	tokenB, wsB := registerAndGetWorkspace(t, router, "b2@example.com")

	body, _ := json.Marshal(map[string]string{"name": "Secret Account", "currency": "IDR"})
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/workspaces/"+wsA+"/accounts", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+tokenA)
	router.ServeHTTP(createRec, createReq)
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created account: %v", err)
	}

	// User B is authenticated but not a member of workspace A at all — every
	// nested route under it must 404, including one for their own workspace ID.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/workspaces/"+wsA+"/accounts/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("user B fetching workspace A's account: status = %d, want 404 (never reveal it exists)", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/workspaces/"+wsB+"/accounts/"+created.ID, nil)
	req2.Header.Set("Authorization", "Bearer "+tokenB)
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("user B fetching A's account ID under their own workspace: status = %d, want 404", rec2.Code)
	}
}

func scanReceiptRequest(t *testing.T, token string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="image"; filename="receipt.jpg"`)
	header.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}
	_, _ = part.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0})
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/receipts/scan", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func TestRouter_ScanReceiptWithoutATokenIsUnauthorized(t *testing.T) {
	router := NewRouter(newTestDeps(fakePinger{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, scanReceiptRequest(t, ""))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRouter_ScanReceiptReturnsADraftAndNeverTouchesTheLedger(t *testing.T) {
	router := NewRouter(newTestDeps(fakePinger{}))
	token, workspaceID := registerAndGetWorkspace(t, router, "scan@example.com")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, scanReceiptRequest(t, token))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !bytesContains(rec.Body.Bytes(), "Test Merchant") {
		t.Errorf("body = %s, want the scanned draft", rec.Body.String())
	}

	txRec := httptest.NewRecorder()
	txReq := httptest.NewRequest(http.MethodGet, "/workspaces/"+workspaceID+"/transactions", nil)
	txReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(txRec, txReq)
	if bytesContains(txRec.Body.Bytes(), "Test Merchant") {
		t.Errorf("a scan must never itself create a transaction: %s", txRec.Body.String())
	}
}

func TestRouter_ScanReceiptEnforcesTheDailyLimit(t *testing.T) {
	router := NewRouter(newTestDepsWithScanLimit(fakePinger{}, 1))
	token := registerAndGetToken(t, router, "scanlimit@example.com")

	first := httptest.NewRecorder()
	router.ServeHTTP(first, scanReceiptRequest(t, token))
	if first.Code != http.StatusOK {
		t.Fatalf("first scan status = %d, body=%s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, scanReceiptRequest(t, token))
	if second.Code != http.StatusTooManyRequests {
		t.Errorf("second scan status = %d, want %d once the daily limit is reached", second.Code, http.StatusTooManyRequests)
	}
}

func TestRouter_ReportsRequireAuth(t *testing.T) {
	router := NewRouter(newTestDeps(fakePinger{}))

	for _, path := range []string{"/workspaces/any-id/reports/summary", "/workspaces/any-id/reports/by-category", "/workspaces/any-id/reports/monthly-trend"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without a token: status = %d, want %d", path, rec.Code, http.StatusUnauthorized)
		}
	}
}

func TestRouter_ReportsSummaryReflectsRealLedgerActivity(t *testing.T) {
	router := NewRouter(newTestDeps(fakePinger{}))
	token, workspaceID := registerAndGetWorkspace(t, router, "reports@example.com")

	accBody, _ := json.Marshal(map[string]string{"name": "Cash", "currency": "IDR"})
	accRec := httptest.NewRecorder()
	accReq := httptest.NewRequest(http.MethodPost, "/workspaces/"+workspaceID+"/accounts", bytes.NewReader(accBody))
	accReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(accRec, accReq)
	var acc struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(accRec.Body.Bytes(), &acc)

	catRec := httptest.NewRecorder()
	catReq := httptest.NewRequest(http.MethodGet, "/workspaces/"+workspaceID+"/categories", nil)
	catReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(catRec, catReq)
	var categories []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	_ = json.Unmarshal(catRec.Body.Bytes(), &categories)
	var salaryID string
	for _, c := range categories {
		if c.Name == "Salary" {
			salaryID = c.ID
		}
	}

	today := time.Now().UTC().Format("2006-01-02")
	txBody, _ := json.Marshal(map[string]any{
		"accountId": acc.ID, "categoryId": salaryID, "direction": "income",
		"amountMinor": 500000, "currency": "IDR", "occurredAt": today,
	})
	txRec := httptest.NewRecorder()
	txReq := httptest.NewRequest(http.MethodPost, "/workspaces/"+workspaceID+"/transactions", bytes.NewReader(txBody))
	txReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(txRec, txReq)
	if txRec.Code != http.StatusCreated {
		t.Fatalf("create transaction status = %d, body=%s", txRec.Code, txRec.Body.String())
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/workspaces/"+workspaceID+"/reports/summary", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var summary struct {
		IncomeMinor int64 `json:"incomeMinor"`
		NetMinor    int64 `json:"netMinor"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.IncomeMinor != 500000 || summary.NetMinor != 500000 {
		t.Errorf("summary = %+v, want incomeMinor=500000 netMinor=500000", summary)
	}
}

func TestRouter_DeleteMeRequiresAuth(t *testing.T) {
	router := NewRouter(newTestDeps(fakePinger{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/auth/me", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRouter_DeleteMeRemovesTheAccountAndDoesNotAffectAnotherUser(t *testing.T) {
	router := NewRouter(newTestDeps(fakePinger{}))
	tokenA := registerAndGetToken(t, router, "deleteme@example.com")
	tokenB := registerAndGetToken(t, router, "keepme@example.com")

	delRec := httptest.NewRecorder()
	delReq := httptest.NewRequest(http.MethodDelete, "/auth/me", nil)
	delReq.Header.Set("Authorization", "Bearer "+tokenA)
	router.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /auth/me status = %d, body=%s", delRec.Code, delRec.Body.String())
	}

	meRec := httptest.NewRecorder()
	meReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+tokenA)
	router.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusNotFound {
		t.Errorf("/auth/me for the deleted user: status = %d, want 404", meRec.Code)
	}

	otherRec := httptest.NewRecorder()
	otherReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	otherReq.Header.Set("Authorization", "Bearer "+tokenB)
	router.ServeHTTP(otherRec, otherReq)
	if otherRec.Code != http.StatusOK {
		t.Errorf("/auth/me for the untouched user B: status = %d, want 200", otherRec.Code)
	}
	if !bytesContains(otherRec.Body.Bytes(), "keepme@example.com") {
		t.Errorf("user B's own data should be unaffected by user A's deletion: %s", otherRec.Body.String())
	}
}

func TestRouter_DeleteMeRemovesASolelyOwnedWorkspaceButNotASharedOne(t *testing.T) {
	mailer := &fakeMailer{}
	deps, wsDB := newTestDepsWithMailer(fakePinger{}, mailer)
	router := NewRouter(deps)

	ownerToken, personalWsID := registerAndGetWorkspace(t, router, "solo-delete@example.com")
	memberToken, _ := registerAndGetWorkspace(t, router, "shared-delete-member@example.com")

	createBody, _ := json.Marshal(map[string]string{"name": "Shared With Member"})
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/workspaces", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+ownerToken)
	router.ServeHTTP(createRec, createReq)
	var shared struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(createRec.Body.Bytes(), &shared)

	meRec := httptest.NewRecorder()
	meReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+memberToken)
	router.ServeHTTP(meRec, meReq)
	var me struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(meRec.Body.Bytes(), &me)

	if err := (&memoryMemberStore{wsDB}).Add(context.Background(), workspaces.Member{
		WorkspaceID: shared.ID, UserID: me.ID, Role: workspaces.RoleMember,
	}); err != nil {
		t.Fatalf("seed shared membership: %v", err)
	}

	delRec := httptest.NewRecorder()
	delReq := httptest.NewRequest(http.MethodDelete, "/auth/me", nil)
	delReq.Header.Set("Authorization", "Bearer "+ownerToken)
	router.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /auth/me status = %d, body=%s", delRec.Code, delRec.Body.String())
	}

	if _, ok := wsDB.workspaces[personalWsID]; ok {
		t.Errorf("solely-owned personal workspace %s still exists after its only member's account was deleted", personalWsID)
	}
	if _, ok := wsDB.workspaces[shared.ID]; !ok {
		t.Errorf("shared workspace %s was deleted, want it to survive for its other member", shared.ID)
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/workspaces/"+shared.ID+"/accounts", nil)
	listReq.Header.Set("Authorization", "Bearer "+memberToken)
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Errorf("remaining member accessing the shared workspace after the owner's deletion: status = %d, want 200, body=%s", listRec.Code, listRec.Body.String())
	}
}

// ----- Phase 9: workspaces (shared ledgers) -----

func TestRouter_CreateWorkspaceThenListIncludesItWithOwnerRole(t *testing.T) {
	router := NewRouter(newTestDeps(fakePinger{}))
	token, _ := registerAndGetWorkspace(t, router, "owner@example.com")

	body, _ := json.Marshal(map[string]string{"name": "Keluarga Budi"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/workspaces", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /workspaces status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !bytesContains(rec.Body.Bytes(), `"role":"owner"`) {
		t.Errorf("created workspace response = %s, want role=owner", rec.Body.String())
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/workspaces", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(listRec, listReq)

	var list []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode workspaces: %v", err)
	}
	// The personal workspace from registration, plus the new shared one.
	if len(list) != 2 {
		t.Fatalf("len(workspaces) = %d, want 2 (personal + shared)", len(list))
	}
}

func TestRouter_InviteAcceptThenBothMembersShareTheSameLedger(t *testing.T) {
	mailer := &fakeMailer{}
	deps, _ := newTestDepsWithMailer(fakePinger{}, mailer)
	router := NewRouter(deps)

	ownerToken, _ := registerAndGetWorkspace(t, router, "umkm-owner@example.com")
	memberToken, _ := registerAndGetWorkspace(t, router, "umkm-member@example.com")

	createBody, _ := json.Marshal(map[string]string{"name": "Warung Ibu"})
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/workspaces", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+ownerToken)
	router.ServeHTTP(createRec, createReq)
	var shared struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &shared); err != nil {
		t.Fatalf("decode created workspace: %v", err)
	}

	inviteBody, _ := json.Marshal(map[string]string{"email": "umkm-member@example.com"})
	inviteRec := httptest.NewRecorder()
	inviteReq := httptest.NewRequest(http.MethodPost, "/workspaces/"+shared.ID+"/invites", bytes.NewReader(inviteBody))
	inviteReq.Header.Set("Authorization", "Bearer "+ownerToken)
	router.ServeHTTP(inviteRec, inviteReq)
	if inviteRec.Code != http.StatusAccepted {
		t.Fatalf("POST .../invites status = %d, body=%s", inviteRec.Code, inviteRec.Body.String())
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("len(mailer.sent) = %d, want exactly 1 invite email", len(mailer.sent))
	}
	token := inviteTokenFromEmail(t, mailer.sent[0].HTMLBody)

	acceptBody, _ := json.Marshal(map[string]string{"token": token})
	acceptRec := httptest.NewRecorder()
	acceptReq := httptest.NewRequest(http.MethodPost, "/workspaces/invites/accept", bytes.NewReader(acceptBody))
	acceptReq.Header.Set("Authorization", "Bearer "+memberToken)
	router.ServeHTTP(acceptRec, acceptReq)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("POST /workspaces/invites/accept status = %d, body=%s", acceptRec.Code, acceptRec.Body.String())
	}

	// Owner posts an account into the shared workspace...
	accBody, _ := json.Marshal(map[string]string{"name": "Kas Warung", "currency": "IDR"})
	accRec := httptest.NewRecorder()
	accReq := httptest.NewRequest(http.MethodPost, "/workspaces/"+shared.ID+"/accounts", bytes.NewReader(accBody))
	accReq.Header.Set("Authorization", "Bearer "+ownerToken)
	router.ServeHTTP(accRec, accReq)
	if accRec.Code != http.StatusCreated {
		t.Fatalf("owner POST .../accounts status = %d, body=%s", accRec.Code, accRec.Body.String())
	}

	// ...and the newly accepted member can see it too.
	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/workspaces/"+shared.ID+"/accounts", nil)
	listReq.Header.Set("Authorization", "Bearer "+memberToken)
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("member GET .../accounts status = %d, body=%s", listRec.Code, listRec.Body.String())
	}
	if !bytesContains(listRec.Body.Bytes(), "Kas Warung") {
		t.Errorf("member's account list = %s, want to see the owner's shared account", listRec.Body.String())
	}
}

func TestRouter_MemberCannotInviteOrRemoveMembers(t *testing.T) {
	mailer := &fakeMailer{}
	deps, wsDB := newTestDepsWithMailer(fakePinger{}, mailer)
	router := NewRouter(deps)

	ownerToken, _ := registerAndGetWorkspace(t, router, "perm-owner@example.com")
	memberToken, memberWsID := registerAndGetWorkspace(t, router, "perm-member@example.com")

	createBody, _ := json.Marshal(map[string]string{"name": "Shared"})
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/workspaces", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+ownerToken)
	router.ServeHTTP(createRec, createReq)
	var shared struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(createRec.Body.Bytes(), &shared)

	// Add the member directly (bypassing the invite flow, since we're testing
	// authorization once already a member, not the invite flow itself).
	memberUserID := ""
	for _, m := range wsDB.members {
		if m.WorkspaceID == memberWsID {
			memberUserID = m.UserID
		}
	}
	if memberUserID == "" {
		t.Fatalf("could not resolve the member's own user ID from their personal workspace")
	}
	if err := (&memoryMemberStore{wsDB}).Add(context.Background(), workspaces.Member{
		WorkspaceID: shared.ID, UserID: memberUserID, Role: workspaces.RoleMember,
	}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	inviteBody, _ := json.Marshal(map[string]string{"email": "someone-else@example.com"})
	inviteRec := httptest.NewRecorder()
	inviteReq := httptest.NewRequest(http.MethodPost, "/workspaces/"+shared.ID+"/invites", bytes.NewReader(inviteBody))
	inviteReq.Header.Set("Authorization", "Bearer "+memberToken)
	router.ServeHTTP(inviteRec, inviteReq)
	if inviteRec.Code != http.StatusForbidden {
		t.Errorf("member inviting: status = %d, want 403", inviteRec.Code)
	}

	removeRec := httptest.NewRecorder()
	removeReq := httptest.NewRequest(http.MethodDelete, "/workspaces/"+shared.ID+"/members/"+memberUserID, nil)
	removeReq.Header.Set("Authorization", "Bearer "+memberToken)
	router.ServeHTTP(removeRec, removeReq)
	if removeRec.Code != http.StatusForbidden {
		t.Errorf("member removing a member: status = %d, want 403", removeRec.Code)
	}
}

func TestRouter_OwnerCannotBeRemoved(t *testing.T) {
	router := NewRouter(newTestDeps(fakePinger{}))
	ownerToken, ownerWsID := registerAndGetWorkspace(t, router, "solo-owner@example.com")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	router.ServeHTTP(rec, req)
	var me struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &me)

	removeRec := httptest.NewRecorder()
	removeReq := httptest.NewRequest(http.MethodDelete, "/workspaces/"+ownerWsID+"/members/"+me.ID, nil)
	removeReq.Header.Set("Authorization", "Bearer "+ownerToken)
	router.ServeHTTP(removeRec, removeReq)

	if removeRec.Code != http.StatusForbidden {
		t.Errorf("owner removing themselves: status = %d, want 403 (ErrCannotRemoveOwner)", removeRec.Code)
	}
}
