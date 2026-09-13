package auth

import (
	"net/http"
	"time"

	"cashflow-backend/internal/users"
)

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type userResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

type authResponse struct {
	User                 userResponse `json:"user"`
	AccessToken          string       `json:"accessToken"`
	AccessTokenExpiresAt string       `json:"accessTokenExpiresAt"`
	RefreshToken         string       `json:"refreshToken"`
}

func toUserResponse(u users.User) userResponse {
	return userResponse{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName}
}

func toAuthResponse(result AuthResult) authResponse {
	return authResponse{
		User:                 toUserResponse(result.User),
		AccessToken:          result.AccessToken,
		AccessTokenExpiresAt: result.AccessExpiresAt.UTC().Format(time.RFC3339),
		RefreshToken:         result.RefreshToken,
	}
}

// RegisterHandler handles POST /auth/register: create a user and sign them in.
func RegisterHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		result, err := svc.Register(r.Context(), req.Email, req.Password, req.DisplayName)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusCreated, toAuthResponse(result))
	}
}

// LoginHandler handles POST /auth/login.
func LoginHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		result, err := svc.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, toAuthResponse(result))
	}
}

// RefreshHandler handles POST /auth/refresh: rotate a refresh token for a new
// access + refresh pair. Public (no bearer token needed — the refresh token
// itself is the credential).
func RefreshHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req refreshRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.RefreshToken == "" {
			respondJSON(w, http.StatusBadRequest, errorBody("refreshToken is required"))
			return
		}

		result, err := svc.Refresh(r.Context(), req.RefreshToken)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, toAuthResponse(result))
	}
}

// LogoutHandler handles POST /auth/logout (behind RequireAuth): revoke the
// presented refresh token.
func LogoutHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req logoutRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		if err := svc.Logout(r.Context(), req.RefreshToken); err != nil {
			writeServiceError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// MeHandler handles GET /auth/me (behind RequireAuth): return the calling
// user's own profile — proof that the token binds to exactly one user.
func MeHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			respondJSON(w, http.StatusUnauthorized, errorBody("not authenticated"))
			return
		}

		u, err := svc.CurrentUser(r.Context(), userID)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, toUserResponse(u))
	}
}

// DeleteMeHandler handles DELETE /auth/me (behind RequireAuth): permanently
// deletes the calling user's account and everything they own (Phase 8
// self-service account deletion — see Service.DeleteAccount). Irreversible;
// the app is expected to have its own confirmation step before calling this.
func DeleteMeHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			respondJSON(w, http.StatusUnauthorized, errorBody("not authenticated"))
			return
		}

		if err := svc.DeleteAccount(r.Context(), userID); err != nil {
			writeServiceError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
