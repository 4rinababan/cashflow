package ledger

import (
	"net/http"
	"strconv"
	"time"

	"cashflow-backend/internal/auth"
	"cashflow-backend/internal/workspaces"
)

const dateLayout = "2006-01-02"

// ----- response/request shapes -----

type accountResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Currency     string `json:"currency"`
	Archived     bool   `json:"archived"`
	BalanceMinor int64  `json:"balanceMinor"`
	CreatedAt    string `json:"createdAt"`
}

func toAccountResponse(a AccountBalance) accountResponse {
	return accountResponse{
		ID:           a.ID,
		Name:         a.Name,
		Currency:     a.Currency,
		Archived:     a.ArchivedAt != nil,
		BalanceMinor: a.BalanceMinor,
		CreatedAt:    a.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type createAccountRequest struct {
	Name     string `json:"name"`
	Currency string `json:"currency"`
}

type updateAccountRequest struct {
	Name     *string `json:"name"`
	Archived *bool   `json:"archived"`
}

type categoryResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Archived  bool   `json:"archived"`
	CreatedAt string `json:"createdAt"`
}

func toCategoryResponse(c Category) categoryResponse {
	return categoryResponse{
		ID:        c.ID,
		Name:      c.Name,
		Kind:      string(c.Kind),
		Archived:  c.ArchivedAt != nil,
		CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type createCategoryRequest struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type updateCategoryRequest struct {
	Name     *string `json:"name"`
	Archived *bool   `json:"archived"`
}

type transactionResponse struct {
	ID          string `json:"id"`
	AccountID   string `json:"accountId"`
	CategoryID  string `json:"categoryId"`
	Direction   string `json:"direction"`
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
	OccurredAt  string `json:"occurredAt"`
	Note        string `json:"note"`
	CreatedBy   string `json:"createdByUserId"`
}

func toTransactionResponse(t Transaction) transactionResponse {
	return transactionResponse{
		ID:          t.ID,
		AccountID:   t.AccountID,
		CategoryID:  t.CategoryID,
		Direction:   string(t.Direction),
		AmountMinor: t.AmountMinor,
		Currency:    t.Currency,
		OccurredAt:  t.OccurredAt.Format(dateLayout),
		Note:        t.Note,
		CreatedBy:   t.CreatedByUserID,
	}
}

type createTransactionRequest struct {
	AccountID   string `json:"accountId"`
	CategoryID  string `json:"categoryId"`
	Direction   string `json:"direction"`
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
	OccurredAt  string `json:"occurredAt"`
	Note        string `json:"note"`
}

type updateTransactionRequest struct {
	AccountID   *string `json:"accountId"`
	CategoryID  *string `json:"categoryId"`
	Direction   *string `json:"direction"`
	AmountMinor *int64  `json:"amountMinor"`
	OccurredAt  *string `json:"occurredAt"`
	Note        *string `json:"note"`
}

type transactionListResponse struct {
	Items  []transactionResponse `json:"items"`
	Total  int                   `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}

// requireWorkspace pulls the current workspace ID (scoping) and the caller's
// user ID (for CreatedByUserID) out of the request context — put there by
// auth.RequireAuth and workspaces.RequireMembership, which every route below
// is wrapped in, in that order.
func requireWorkspace(w http.ResponseWriter, r *http.Request) (workspaceID, userID string, ok bool) {
	workspaceID, wOK := workspaces.WorkspaceIDFromContext(r.Context())
	userID, uOK := auth.UserIDFromContext(r.Context())
	if !wOK || !uOK {
		respondJSON(w, http.StatusUnauthorized, errorBody("not authenticated"))
		return "", "", false
	}
	return workspaceID, userID, true
}

// ----- accounts -----

func CreateAccountHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, userID, ok := requireWorkspace(w, r)
		if !ok {
			return
		}
		var req createAccountRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		created, err := svc.CreateAccount(r.Context(), workspaceID, userID, req.Name, req.Currency)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusCreated, toAccountResponse(AccountBalance{Account: created}))
	}
}

func ListAccountsHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, _, ok := requireWorkspace(w, r)
		if !ok {
			return
		}

		accounts, err := svc.ListAccounts(r.Context(), workspaceID)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		items := make([]accountResponse, 0, len(accounts))
		for _, a := range accounts {
			items = append(items, toAccountResponse(a))
		}
		respondJSON(w, http.StatusOK, items)
	}
}

func GetAccountHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, _, ok := requireWorkspace(w, r)
		if !ok {
			return
		}

		a, err := svc.GetAccount(r.Context(), workspaceID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, toAccountResponse(a))
	}
}

func UpdateAccountHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, _, ok := requireWorkspace(w, r)
		if !ok {
			return
		}
		var req updateAccountRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		updated, err := svc.UpdateAccount(r.Context(), workspaceID, r.PathValue("id"), req.Name, req.Archived)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, toAccountResponse(AccountBalance{Account: updated}))
	}
}

func DeleteAccountHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, _, ok := requireWorkspace(w, r)
		if !ok {
			return
		}

		if err := svc.DeleteAccount(r.Context(), workspaceID, r.PathValue("id")); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ----- categories -----

func CreateCategoryHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, userID, ok := requireWorkspace(w, r)
		if !ok {
			return
		}
		var req createCategoryRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		created, err := svc.CreateCategory(r.Context(), workspaceID, userID, req.Name, Direction(req.Kind))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		respondJSON(w, http.StatusCreated, toCategoryResponse(created))
	}
}

func ListCategoriesHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, _, ok := requireWorkspace(w, r)
		if !ok {
			return
		}

		categories, err := svc.ListCategories(r.Context(), workspaceID)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		items := make([]categoryResponse, 0, len(categories))
		for _, c := range categories {
			items = append(items, toCategoryResponse(c))
		}
		respondJSON(w, http.StatusOK, items)
	}
}

func UpdateCategoryHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, _, ok := requireWorkspace(w, r)
		if !ok {
			return
		}
		var req updateCategoryRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		updated, err := svc.UpdateCategory(r.Context(), workspaceID, r.PathValue("id"), req.Name, req.Archived)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, toCategoryResponse(updated))
	}
}

func DeleteCategoryHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, _, ok := requireWorkspace(w, r)
		if !ok {
			return
		}

		if err := svc.DeleteCategory(r.Context(), workspaceID, r.PathValue("id")); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ----- transactions -----

func CreateTransactionHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, userID, ok := requireWorkspace(w, r)
		if !ok {
			return
		}
		var req createTransactionRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		occurredAt, err := time.Parse(dateLayout, req.OccurredAt)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, errorBody("occurredAt must be a YYYY-MM-DD date"))
			return
		}

		created, err := svc.CreateTransaction(r.Context(), workspaceID, userID, CreateTransactionInput{
			AccountID:   req.AccountID,
			CategoryID:  req.CategoryID,
			Direction:   Direction(req.Direction),
			AmountMinor: req.AmountMinor,
			Currency:    req.Currency,
			OccurredAt:  occurredAt,
			Note:        req.Note,
		})
		if err != nil {
			writeServiceError(w, err)
			return
		}
		respondJSON(w, http.StatusCreated, toTransactionResponse(created))
	}
}

func GetTransactionHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, _, ok := requireWorkspace(w, r)
		if !ok {
			return
		}

		t, err := svc.GetTransaction(r.Context(), workspaceID, r.PathValue("id"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, toTransactionResponse(t))
	}
}

// ListTransactionsHandler supports optional query filters: accountId,
// categoryId, from, to (YYYY-MM-DD), limit, offset.
func ListTransactionsHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, _, ok := requireWorkspace(w, r)
		if !ok {
			return
		}

		q := r.URL.Query()
		filter := TransactionFilter{
			AccountID:  q.Get("accountId"),
			CategoryID: q.Get("categoryId"),
		}
		if raw := q.Get("from"); raw != "" {
			parsed, err := time.Parse(dateLayout, raw)
			if err != nil {
				respondJSON(w, http.StatusBadRequest, errorBody("from must be a YYYY-MM-DD date"))
				return
			}
			filter.From = &parsed
		}
		if raw := q.Get("to"); raw != "" {
			parsed, err := time.Parse(dateLayout, raw)
			if err != nil {
				respondJSON(w, http.StatusBadRequest, errorBody("to must be a YYYY-MM-DD date"))
				return
			}
			filter.To = &parsed
		}
		if raw := q.Get("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil {
				filter.Limit = n
			}
		}
		if raw := q.Get("offset"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil {
				filter.Offset = n
			}
		}

		items, total, err := svc.ListTransactions(r.Context(), workspaceID, filter)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		responseItems := make([]transactionResponse, 0, len(items))
		for _, t := range items {
			responseItems = append(responseItems, toTransactionResponse(t))
		}

		limit := filter.Limit
		if limit <= 0 || limit > maxListLimit {
			limit = defaultListLimit
		}
		respondJSON(w, http.StatusOK, transactionListResponse{
			Items:  responseItems,
			Total:  total,
			Limit:  limit,
			Offset: filter.Offset,
		})
	}
}

func UpdateTransactionHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, _, ok := requireWorkspace(w, r)
		if !ok {
			return
		}
		var req updateTransactionRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		in := UpdateTransactionInput{
			AccountID:   req.AccountID,
			CategoryID:  req.CategoryID,
			AmountMinor: req.AmountMinor,
			Note:        req.Note,
		}
		if req.Direction != nil {
			d := Direction(*req.Direction)
			in.Direction = &d
		}
		if req.OccurredAt != nil {
			parsed, err := time.Parse(dateLayout, *req.OccurredAt)
			if err != nil {
				respondJSON(w, http.StatusBadRequest, errorBody("occurredAt must be a YYYY-MM-DD date"))
				return
			}
			in.OccurredAt = &parsed
		}

		updated, err := svc.UpdateTransaction(r.Context(), workspaceID, r.PathValue("id"), in)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, toTransactionResponse(updated))
	}
}

func DeleteTransactionHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, _, ok := requireWorkspace(w, r)
		if !ok {
			return
		}

		if err := svc.DeleteTransaction(r.Context(), workspaceID, r.PathValue("id")); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
