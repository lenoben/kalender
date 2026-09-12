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

	log.Printf("Starting Go Supabase Multi-Task Calendar App in %s mode...", cfg.Env)

	// Initialize Database Pool
	pool, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer pool.Close()

	// Parse Templates
	tmplPattern := filepath.Join("templates", "*.html")
	funcMap := template.FuncMap{
		"seq": func(n int) []int {
			result := make([]int, n)
			for i := range result {
				result[i] = i + 1
			}
			return result
		},
	}

	tmpl, err := template.New("base").Funcs(funcMap).ParseGlob(tmplPattern)
	if err != nil {
		log.Fatalf("Failed to parse templates pattern '%s': %v", tmplPattern, err)
	}

	// Auth Manager & Handlers
	authManager := auth.NewAuthManager(cfg)
	calendarHandler := handlers.NewCalendarHandler(pool, authManager, tmpl)
	tasksHandler := handlers.NewTasksHandler(pool, authManager)

	// Chi Router Setup
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Static Files Route
	workDir, _ := os.Getwd()
	filesDir := http.Dir(filepath.Join(workDir, "static"))
	FileServer(r, "/static", filesDir)

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

// FileServer conveniently sets up a static file server route
func FileServer(r chi.Router, path string, root http.FileSystem) {
	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", http.StatusMovedPermanently).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := filepath.Clean(rctx.RoutePattern())
		pathPrefix = pathPrefix[:len(pathPrefix)-2]
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}
