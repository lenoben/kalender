package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go-supabase-calendar/internal/auth"
	"go-supabase-calendar/internal/models"
)

type TasksHandler struct {
	db   *pgxpool.Pool
	auth *auth.AuthManager
}

func NewTasksHandler(db *pgxpool.Pool, auth *auth.AuthManager) *TasksHandler {
	return &TasksHandler{
		db:   db,
		auth: auth,
	}
}

func (h *TasksHandler) GetTasksByDate(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid date, expected YYYY-MM-DD")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.db.Query(ctx, `SELECT `+taskColumns+` FROM kalender_tasks WHERE task_date = $1 ORDER BY start_time ASC`, dateStr)
	if err != nil {
		log.Printf("GetTasksByDate query: %v", err)
		writeError(w, http.StatusInternalServerError, "Database query error")
		return
	}
	defer rows.Close()

	isAdmin := h.auth.IsAdmin(r)

	tasks := []models.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			log.Printf("GetTasksByDate scan: %v", err)
			writeError(w, http.StatusInternalServerError, "Scan error")
			return
		}
		if !isAdmin {
			// Requester contact details are only visible to the admin
			t.RequestedByEmail = ""
		}
		tasks = append(tasks, t)
	}

	writeJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data: map[string]any{
			"date":     dateStr,
			"tasks":    tasks,
			"is_admin": isAdmin,
		},
	})
}

func (h *TasksHandler) PublicRequestSlot(w http.ResponseWriter, r *http.Request) {
	var req models.PublicBookingRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	req.RequestedByName = strings.TrimSpace(req.RequestedByName)
	req.RequestedByEmail = strings.TrimSpace(req.RequestedByEmail)
	req.StartTime, req.EndTime = trimSeconds(req.StartTime), trimSeconds(req.EndTime)
	if strings.TrimSpace(req.Title) == "" {
		req.Title = "Time Request: " + req.RequestedByName
	}

	if err := validateRequester(req.RequestedByName, req.RequestedByEmail); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateSlot(req.TaskDate, req.StartTime, req.EndTime, req.Title, req.Description); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	newTask, err := scanTask(h.db.QueryRow(ctx, `
		INSERT INTO kalender_tasks (task_date, start_time, end_time, title, description, is_booked, requested_by_name, requested_by_email)
		VALUES ($1, $2, $3, $4, $5, true, $6, $7)
		RETURNING `+taskColumns,
		req.TaskDate, req.StartTime, req.EndTime, req.Title, req.Description, req.RequestedByName, req.RequestedByEmail,
	))
	if err != nil {
		log.Printf("PublicRequestSlot insert: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to submit request")
		return
	}

	writeJSON(w, http.StatusCreated, models.APIResponse{
		Success: true,
		Message: "Booking request submitted successfully",
		Data:    newTask,
	})
}

func (h *TasksHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTaskRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	req.StartTime, req.EndTime = trimSeconds(req.StartTime), trimSeconds(req.EndTime)
	if err := validateSlot(req.TaskDate, req.StartTime, req.EndTime, req.Title, req.Description); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	newTask, err := scanTask(h.db.QueryRow(ctx, `
		INSERT INTO kalender_tasks (task_date, start_time, end_time, title, description, is_booked, requested_by_name, requested_by_email)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''))
		RETURNING `+taskColumns,
		req.TaskDate, req.StartTime, req.EndTime, req.Title, req.Description, req.IsBooked, req.RequestedByName, req.RequestedByEmail,
	))
	if err != nil {
		log.Printf("CreateTask insert: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}

	writeJSON(w, http.StatusCreated, models.APIResponse{
		Success: true,
		Message: "Task created successfully",
		Data:    newTask,
	})
}

func (h *TasksHandler) ToggleTaskBooking(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	if !uuidPattern.MatchString(taskID) {
		writeError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var newBookedState bool
	err := h.db.QueryRow(ctx, `UPDATE kalender_tasks SET is_booked = NOT is_booked WHERE id = $1 RETURNING is_booked`, taskID).Scan(&newBookedState)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "Task not found")
		return
	}
	if err != nil {
		log.Printf("ToggleTaskBooking: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update booking status")
		return
	}

	writeJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Booking status updated",
		Data:    map[string]any{"is_booked": newBookedState},
	})
}

func (h *TasksHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	if !uuidPattern.MatchString(taskID) {
		writeError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tag, err := h.db.Exec(ctx, "DELETE FROM kalender_tasks WHERE id = $1", taskID)
	if err != nil {
		log.Printf("DeleteTask: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to delete task")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "Task not found")
		return
	}

	writeJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Task deleted successfully",
	})
}
