package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-supabase-calendar/internal/config"
	"go-supabase-calendar/internal/models"
)

const (
	AdminCookieName = "admin_session"
	sessionTTL      = 24 * time.Hour
)

type AuthManager struct {
	adminPassword string
	secret        []byte
	secureCookies bool
}

func NewAuthManager(cfg *config.Config) *AuthManager {
	return &AuthManager{
		adminPassword: cfg.AdminPassword,
		secret:        []byte(cfg.SessionSecret),
		secureCookies: cfg.IsProduction(),
	}
}

// IsAdmin checks if request contains valid admin credentials
func (am *AuthManager) IsAdmin(r *http.Request) bool {
	// Signed session cookie
	if cookie, err := r.Cookie(AdminCookieName); err == nil && am.validToken(cookie.Value) {
		return true
	}

	// Custom header (useful for API clients)
	if headerPass := r.Header.Get("X-Admin-Password"); headerPass != "" && am.passwordMatches(headerPass) {
		return true
	}

	return false
}

func (am *AuthManager) passwordMatches(candidate string) bool {
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(am.adminPassword)) == 1
}

// newToken returns "<expiry-unix>.<base64 hmac>" so the session cannot be forged without the secret.
func (am *AuthManager) newToken(expires time.Time) string {
	payload := strconv.FormatInt(expires.Unix(), 10)
	return payload + "." + am.sign(payload)
}

func (am *AuthManager) validToken(token string) bool {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	if !hmac.Equal([]byte(sig), []byte(am.sign(payload))) {
		return false
	}
	expiry, err := strconv.ParseInt(payload, 10, 64)
	return err == nil && time.Now().Unix() < expiry
}

func (am *AuthManager) sign(payload string) string {
	mac := hmac.New(sha256.New, am.secret)
	mac.Write([]byte("admin|" + payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// AdminAuthMiddleware guards protected endpoints
func (am *AuthManager) AdminAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !am.IsAdmin(r) {
			writeJSON(w, http.StatusUnauthorized, models.APIResponse{
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
	var req models.LoginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.APIResponse{Success: false, Message: "Invalid JSON request body"})
		return
	}

	if !am.passwordMatches(req.Password) {
		// Slow down brute-force attempts
		time.Sleep(500 * time.Millisecond)
		writeJSON(w, http.StatusUnauthorized, models.APIResponse{Success: false, Message: "Invalid admin password"})
		return
	}

	expires := time.Now().Add(sessionTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     AdminCookieName,
		Value:    am.newToken(expires),
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   am.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "Authenticated successfully as admin"})
}

// LogoutHandler clears admin session
func (am *AuthManager) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     AdminCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   am.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, models.APIResponse{Success: true, Message: "Logged out successfully"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
