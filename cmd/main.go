package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go-supabase-calendar/internal/auth"
	"go-supabase-calendar/internal/config"
	"go-supabase-calendar/internal/db"
	"go-supabase-calendar/internal/handlers"
)

func main() {
	cfg := config.LoadConfig()

	log.Printf("Starting application in %s mode...", cfg.Env)

	// Initialize Database Pool
	pool, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer pool.Close()

	// Parse Templates
	tmplPattern := filepath.Join("templates", "*.html")
	tmpl, err := template.New("base").ParseGlob(tmplPattern)
	if err != nil {
		log.Fatalf("Failed to parse templates pattern '%s': %v", tmplPattern, err)
	}

	// Auth Manager & Handlers
	authManager := auth.NewAuthManager(cfg)
	calendarHandler := handlers.NewCalendarHandler(pool, authManager, tmpl)
	tasksHandler := handlers.NewTasksHandler(pool, authManager)

	// Chi Router Setup
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(securityHeaders)

	// Static Files Route
	workDir, _ := os.Getwd()
	staticDir := filepath.Join(workDir, "static")
	FileServer(r, "/static", http.Dir(staticDir))

	// PWA: service worker must be served from the root to control the whole app
	r.Get("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(staticDir, "sw.js"))
	})
	r.Get("/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(staticDir, "manifest.webmanifest"))
	})
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	// Public Routes
	r.Get("/", calendarHandler.RenderCalendar)
	r.Get("/calendar", calendarHandler.RenderCalendar)
	r.Get("/api/tasks", tasksHandler.GetTasksByDate)
	r.Post("/api/request-slot", tasksHandler.PublicRequestSlot)
	r.Post("/api/login", authManager.LoginHandler)
	r.Post("/api/logout", authManager.LogoutHandler)

	// Protected Admin Routes
	r.Group(func(r chi.Router) {
		r.Use(authManager.AdminAuthMiddleware)
		r.Post("/api/tasks", tasksHandler.CreateTask)
		r.Put("/api/tasks/{id}/toggle", tasksHandler.ToggleTaskBooking)
		r.Delete("/api/tasks/{id}", tasksHandler.DeleteTask)
	})

	// Server Listener
	serverAddr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server listening on http://localhost:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped cleanly.")
}

// FileServer serves a static directory under the given URL prefix (e.g. "/static")
func FileServer(r chi.Router, prefix string, root http.FileSystem) {
	fs := http.StripPrefix(prefix, http.FileServer(root))
	r.Get(prefix, http.RedirectHandler(prefix+"/", http.StatusMovedPermanently).ServeHTTP)
	r.Get(prefix+"/*", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fs.ServeHTTP(w, r)
	})
}

// securityHeaders adds conservative browser security headers to every response
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}
