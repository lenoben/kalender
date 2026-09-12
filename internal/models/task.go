package models

import (
	"time"
)

type Task struct {
	ID               string    `json:"id"`
	TaskDate         string    `json:"task_date"`  // YYYY-MM-DD
	StartTime        string    `json:"start_time"` // HH:MM
	EndTime          string    `json:"end_time"`   // HH:MM
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	IsBooked         bool      `json:"is_booked"`
	RequestedByName  string    `json:"requested_by_name,omitempty"`
	RequestedByEmail string    `json:"requested_by_email,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type DaySummary struct {
	Date           string `json:"date"` // YYYY-MM-DD
	DayNumber      int    `json:"day_number"`
	IsCurrentMonth bool   `json:"is_current_month"`
	IsToday        bool   `json:"is_today"`
	Status         string `json:"status"` // "available" (green), "booked" (red), "none" (gray)
	TaskCount      int    `json:"task_count"`
	AvailableCount int    `json:"available_count"`
	BookedCount    int    `json:"booked_count"`
	Tasks          []Task `json:"tasks,omitempty"`
}

type CalendarMonth struct {
	Year        int            `json:"year"`
	Month       time.Month     `json:"month"`
	MonthNumber int            `json:"month_number"`
	MonthName   string         `json:"month_name"`
	PrevYear    int            `json:"prev_year"`
	PrevMonth   int            `json:"prev_month"`
	NextYear    int            `json:"next_year"`
	NextMonth   int            `json:"next_month"`
	Weeks       [][]DaySummary `json:"weeks"`
	IsAdmin     bool           `json:"is_admin"`
}

type CreateTaskRequest struct {
	TaskDate         string `json:"task_date"`
	StartTime        string `json:"start_time"`
	EndTime          string `json:"end_time"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	IsBooked         bool   `json:"is_booked"`
	RequestedByName  string `json:"requested_by_name"`
	RequestedByEmail string `json:"requested_by_email"`
}

type PublicBookingRequest struct {
	TaskDate         string `json:"task_date"`
	StartTime        string `json:"start_time"`
	EndTime          string `json:"end_time"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	RequestedByName  string `json:"requested_by_name"`
	RequestedByEmail string `json:"requested_by_email"`
}

type LoginRequest struct {
	Password string `json:"password"`
}

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}
