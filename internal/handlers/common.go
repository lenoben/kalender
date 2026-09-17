package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"go-supabase-calendar/internal/models"
)

const taskColumns = `id, task_date::text, start_time::text, end_time::text, title, COALESCE(description, ''), is_booked, COALESCE(requested_by_name, ''), COALESCE(requested_by_email, ''), created_at`

const (
	maxTitleLen       = 255
	maxNameLen        = 255
	maxDescriptionLen = 2000
	maxBodyBytes      = 1 << 16
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func writeJSON(w http.ResponseWriter, status int, body models.APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, models.APIResponse{Success: false, Message: message})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(dst)
}

// scanTask reads one row selected with taskColumns and normalises times to HH:MM.
func scanTask(row pgx.Row) (models.Task, error) {
	var t models.Task
	err := row.Scan(&t.ID, &t.TaskDate, &t.StartTime, &t.EndTime, &t.Title, &t.Description, &t.IsBooked, &t.RequestedByName, &t.RequestedByEmail, &t.CreatedAt)
	t.StartTime = trimSeconds(t.StartTime)
	t.EndTime = trimSeconds(t.EndTime)
	return t, err
}

func trimSeconds(raw string) string {
	if len(raw) >= 5 {
		return raw[:5]
	}
	return raw
}

// validateSlot checks the shared date/time/title fields of a task payload.
func validateSlot(date, start, end, title, description string) error {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return errors.New("Invalid date, expected YYYY-MM-DD")
	}
	startT, err := time.Parse("15:04", start)
	if err != nil {
		return errors.New("Invalid start time, expected HH:MM")
	}
	endT, err := time.Parse("15:04", end)
	if err != nil {
		return errors.New("Invalid end time, expected HH:MM")
	}
	if !endT.After(startT) {
		return errors.New("End time must be after start time")
	}
	if strings.TrimSpace(title) == "" {
		return errors.New("Title is required")
	}
	if len(title) > maxTitleLen {
		return errors.New("Title is too long")
	}
	if len(description) > maxDescriptionLen {
		return errors.New("Description is too long")
	}
	return nil
}

func validateRequester(name, email string) error {
	if strings.TrimSpace(name) == "" || len(name) > maxNameLen {
		return errors.New("Please provide your name")
	}
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return errors.New("Please provide a valid email address")
	}
	return nil
}
