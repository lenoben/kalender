package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"go-supabase-calendar/internal/config"
	"go-supabase-calendar/internal/models"
)

const AdminCookieName = "admin_session"

type AuthManager struct {
	adminPassword string
}

func NewAuthManager(cfg *config.Config) *AuthManager {
	return &AuthManager{
		adminPassword: cfg.AdminPassword,
	}
}

// IsAdmin checks if request contains valid admin credentials
func (am *AuthManager) IsAdmin(r *http.Request) bool {
	// Check cookie
	cookie, err := r.Cookie(AdminCookieName)
	if err == nil && cookie.Value == "authenticated" {
		return true
	}

	// Check custom header (useful for API clients)
	if headerPass := r.Header.Get("X-Admin-Password"); headerPass != "" && headerPass == am.adminPassword {
		return true
	}

	return false
}

// AdminAuthMiddleware guards protected endpoints
func (am *AuthManager) AdminAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !am.IsAdmin(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(models.APIResponse{
				Success: false,
				Message: "Unauthorized: Admin password required to modify tasks.",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LoginHandler validates password and sets cookie
func (am *AuthManager) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.APIResponse{
			Success: false,
			Message: "Invalid JSON request body",
		})
		return
	}

	if req.Password != am.adminPassword {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(models.APIResponse{
			Success: false,
			Message: "Invalid admin password",
		})
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     AdminCookieName,
		Value:    "authenticated",
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.APIResponse{
		Success: true,
		Message: "Authenticated successfully as admin",
	})
}

// LogoutHandler clears admin session
func (am *AuthManager) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     AdminCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.APIResponse{
		Success: true,
		Message: "Logged out successfully",
	})
}
