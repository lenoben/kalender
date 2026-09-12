package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
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

// GetTasksByDate handles GET /api/tasks?date=YYYY-MM-DD
func (h *TasksHandler) GetTasksByDate(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, task_date::text, start_time::text, end_time::text, title, COALESCE(description, ''), is_booked, COALESCE(requested_by_name, ''), COALESCE(requested_by_email, ''), created_at
		FROM tasks
		WHERE task_date = $1
		ORDER BY start_time ASC
	`

	rows, err := h.db.Query(ctx, query, dateStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.APIResponse{Success: false, Message: "Database query error"})
		return
	}
	defer rows.Close()

	tasks := []models.Task{}
	for rows.Next() {
		var t models.Task
		var startTimeRaw, endTimeRaw string
		if err := rows.Scan(&t.ID, &t.TaskDate, &startTimeRaw, &endTimeRaw, &t.Title, &t.Description, &t.IsBooked, &t.RequestedByName, &t.RequestedByEmail, &t.CreatedAt); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.APIResponse{Success: false, Message: "Scan error"})
			return
		}
		if len(startTimeRaw) >= 5 {
			t.StartTime = startTimeRaw[:5]
		} else {
			t.StartTime = startTimeRaw
		}
		if len(endTimeRaw) >= 5 {
			t.EndTime = endTimeRaw[:5]
		} else {
			t.EndTime = endTimeRaw
		}
		tasks = append(tasks, t)
	}

	isAdmin := h.auth.IsAdmin(r)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.APIResponse{
		Success: true,
		Data: map[string]any{
			"date":     dateStr,
			"tasks":    tasks,
			"is_admin": isAdmin,
		},
	})
}

// PublicRequestSlot handles POST /api/request-slot (Public visitor booking request)
func (h *TasksHandler) PublicRequestSlot(w http.ResponseWriter, r *http.Request) {
	var req models.PublicBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.APIResponse{Success: false, Message: "Invalid JSON request format"})
		return
	}

	if req.TaskDate == "" || req.StartTime == "" || req.EndTime == "" || req.RequestedByName == "" || req.RequestedByEmail == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.APIResponse{Success: false, Message: "Please provide date, start/end time, your name, and email."})
		return
	}

	if req.Title == "" {
		req.Title = "Time Request: " + req.RequestedByName
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Insert as booked task reserved by outsider
	query := `
		INSERT INTO tasks (task_date, start_time, end_time, title, description, is_booked, requested_by_name, requested_by_email)
		VALUES ($1, $2, $3, $4, $5, true, $6, $7)
		RETURNING id, task_date::text, start_time::text, end_time::text, title, COALESCE(description, ''), is_booked, COALESCE(requested_by_name, ''), COALESCE(requested_by_email, ''), created_at
	`

	var newTask models.Task
	var startTimeRaw, endTimeRaw string
	err := h.db.QueryRow(ctx, query, req.TaskDate, req.StartTime, req.EndTime, req.Title, req.Description, req.RequestedByName, req.RequestedByEmail).Scan(
		&newTask.ID, &newTask.TaskDate, &startTimeRaw, &endTimeRaw, &newTask.Title, &newTask.Description, &newTask.IsBooked, &newTask.RequestedByName, &newTask.RequestedByEmail, &newTask.CreatedAt,
	)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.APIResponse{Success: false, Message: "Failed to submit booking request: " + err.Error()})
		return
	}

	if len(startTimeRaw) >= 5 {
		newTask.StartTime = startTimeRaw[:5]
	}
	if len(endTimeRaw) >= 5 {
		newTask.EndTime = endTimeRaw[:5]
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(models.APIResponse{
		Success: true,
		Message: "Booking request submitted successfully!",
		Data:    newTask,
	})
}

// CreateTask handles POST /api/tasks (Protected Admin Endpoint)
func (h *TasksHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.APIResponse{Success: false, Message: "Invalid JSON format"})
		return
	}

	if req.TaskDate == "" || req.StartTime == "" || req.EndTime == "" || req.Title == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.APIResponse{Success: false, Message: "Missing required fields (date, start_time, end_time, title)"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO tasks (task_date, start_time, end_time, title, description, is_booked, requested_by_name, requested_by_email)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, task_date::text, start_time::text, end_time::text, title, COALESCE(description, ''), is_booked, COALESCE(requested_by_name, ''), COALESCE(requested_by_email, ''), created_at
	`

	var newTask models.Task
	var startTimeRaw, endTimeRaw string
	err := h.db.QueryRow(ctx, query, req.TaskDate, req.StartTime, req.EndTime, req.Title, req.Description, req.IsBooked, req.RequestedByName, req.RequestedByEmail).Scan(
		&newTask.ID, &newTask.TaskDate, &startTimeRaw, &endTimeRaw, &newTask.Title, &newTask.Description, &newTask.IsBooked, &newTask.RequestedByName, &newTask.RequestedByEmail, &newTask.CreatedAt,
	)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.APIResponse{Success: false, Message: "Failed to create task: " + err.Error()})
		return
	}

	if len(startTimeRaw) >= 5 {
		newTask.StartTime = startTimeRaw[:5]
	}
	if len(endTimeRaw) >= 5 {
		newTask.EndTime = endTimeRaw[:5]
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(models.APIResponse{
		Success: true,
		Message: "Task created successfully",
		Data:    newTask,
	})
}

// ToggleTaskBooking handles PUT /api/tasks/{id}/toggle (Protected Admin Endpoint)
func (h *TasksHandler) ToggleTaskBooking(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	if taskID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.APIResponse{Success: false, Message: "Missing task ID"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	query := `UPDATE tasks SET is_booked = NOT is_booked WHERE id = $1 RETURNING is_booked`
	var newBookedState bool
	err := h.db.QueryRow(ctx, query, taskID).Scan(&newBookedState)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.APIResponse{Success: false, Message: "Failed to update booking status"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.APIResponse{
		Success: true,
		Message: "Booking status toggled",
		Data:    map[string]any{"is_booked": newBookedState},
	})
}

// DeleteTask handles DELETE /api/tasks/{id} (Protected Admin Endpoint)
func (h *TasksHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	if taskID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.APIResponse{Success: false, Message: "Missing task ID"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_, err := h.db.Exec(ctx, "DELETE FROM tasks WHERE id = $1", taskID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.APIResponse{Success: false, Message: "Failed to delete task"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.APIResponse{
		Success: true,
		Message: "Task deleted successfully",
	})
}
