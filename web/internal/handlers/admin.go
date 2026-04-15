package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (a *App) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	serveTemplate(w, a.Files, "templates/admin.html")
}

func (a *App) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.DB.ListUsers(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	type userResponse struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		IsAdmin   bool   `json:"is_admin"`
		CreatedAt string `json:"created_at"`
	}
	resp := make([]userResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, userResponse{
			ID:        u.ID,
			Username:  u.Username,
			IsAdmin:   u.IsAdmin,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (a *App) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if a.License != nil {
		allowed, err := a.License.AllowUserCreation(r.Context())
		if err != nil {
			a.Log.Warn("license check failed", "err", err)
		}
		if !allowed {
			http.Error(w, "user creation is disabled by your license", http.StatusForbidden)
			return
		}
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}

	// Provision upload service token first; roll back if anything else fails.
	token, err := a.Upload.CreateToken(r.Context(), req.Username)
	if err != nil {
		http.Error(w, "failed to provision upload token: "+err.Error(), http.StatusBadGateway)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		a.Upload.DeleteToken(r.Context(), req.Username)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	user, err := a.DB.CreateUser(r.Context(), req.Username, string(hash), req.Username, token, req.IsAdmin)
	if err != nil {
		a.Upload.DeleteToken(r.Context(), req.Username)
		http.Error(w, "failed to create user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"is_admin": user.IsAdmin,
	})
}

func (a *App) handleEntitlements(w http.ResponseWriter, r *http.Request) {
	allowUserCreation := true
	if a.License != nil {
		var err error
		allowUserCreation, err = a.License.AllowUserCreation(r.Context())
		if err != nil {
			a.Log.Warn("license check failed", "err", err)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"allow_user_creation": allowUserCreation,
		"sdk_available":       a.License != nil,
	})
}

func (a *App) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	tokenName, err := a.DB.DeleteUser(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if tokenName == "" {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Best-effort revoke; the user is already removed from the db.
	a.Upload.DeleteToken(r.Context(), tokenName)

	w.WriteHeader(http.StatusNoContent)
}
