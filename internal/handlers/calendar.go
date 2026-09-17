package handlers

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go-supabase-calendar/internal/auth"
	"go-supabase-calendar/internal/models"
)

type CalendarHandler struct {
	db        *pgxpool.Pool
	auth      *auth.AuthManager
	templates *template.Template
}

func NewCalendarHandler(db *pgxpool.Pool, auth *auth.AuthManager, templates *template.Template) *CalendarHandler {
	return &CalendarHandler{
		db:        db,
		auth:      auth,
		templates: templates,
	}
}

func (h *CalendarHandler) RenderCalendar(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")

	year := now.Year()
	month := int(now.Month())

	if y, err := strconv.Atoi(yearStr); err == nil && y >= 2000 && y <= 2100 {
		year = y
	}
	if m, err := strconv.Atoi(monthStr); err == nil && m >= 1 && m <= 12 {
		month = m
	}

	calendarData, err := h.buildCalendarMonth(r.Context(), year, month, h.auth.IsAdmin(r))
	if err != nil {
		log.Printf("Error building calendar: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if err := h.templates.ExecuteTemplate(w, "base.html", calendarData); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Template Execution Error", http.StatusInternalServerError)
	}
}

func (h *CalendarHandler) buildCalendarMonth(ctx context.Context, year, month int, isAdmin bool) (*models.CalendarMonth, error) {
	firstOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	weekday := int(firstOfMonth.Weekday())
	gridStart := firstOfMonth.AddDate(0, 0, -weekday)

	totalDaysSoFar := weekday + lastOfMonth.Day()
	remainingInGrid := (7 - (totalDaysSoFar % 7)) % 7
	if totalDaysSoFar+remainingInGrid < 35 {
		remainingInGrid += 7
	}
	gridEnd := lastOfMonth.AddDate(0, 0, remainingInGrid)

	tasksByDate, err := h.fetchTasksForDateRange(ctx, gridStart.Format("2006-01-02"), gridEnd.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}

	todayStr := time.Now().Format("2006-01-02")

	var weeks [][]models.DaySummary
	var currentWeek []models.DaySummary
	var agenda []models.DaySummary

	currDate := gridStart
	for !currDate.After(gridEnd) {
		dateStr := currDate.Format("2006-01-02")
		dayTasks := tasksByDate[dateStr]

		availCount := 0
		bookedCount := 0
		for _, t := range dayTasks {
			if t.IsBooked {
				bookedCount++
			} else {
				availCount++
			}
		}

		status := "none"
		if len(dayTasks) > 0 {
			if availCount > 0 {
				status = "available"
			} else {
				status = "booked"
			}
		}

		isCurrentMonth := currDate.Month() == time.Month(month)

		daySummary := models.DaySummary{
			Date:           dateStr,
			DayNumber:      currDate.Day(),
			Weekday:        currDate.Format("Mon"),
			LongLabel:      currDate.Format("Monday, January 2"),
			IsCurrentMonth: isCurrentMonth,
			IsToday:        dateStr == todayStr,
			IsPast:         dateStr < todayStr,
			Status:         status,
			TaskCount:      len(dayTasks),
			AvailableCount: availCount,
			BookedCount:    bookedCount,
			Tasks:          dayTasks,
		}

		currentWeek = append(currentWeek, daySummary)
		if isCurrentMonth && len(dayTasks) > 0 {
			agenda = append(agenda, daySummary)
		}

		if len(currentWeek) == 7 {
			weeks = append(weeks, currentWeek)
			currentWeek = []models.DaySummary{}
		}

		currDate = currDate.AddDate(0, 0, 1)
	}

	prevMonthTime := firstOfMonth.AddDate(0, -1, 0)
	nextMonthTime := firstOfMonth.AddDate(0, 1, 0)

	return &models.CalendarMonth{
		Year:        year,
		Month:       time.Month(month),
		MonthNumber: month,
		MonthName:   firstOfMonth.Format("January"),
		PrevYear:    prevMonthTime.Year(),
		PrevMonth:   int(prevMonthTime.Month()),
		NextYear:    nextMonthTime.Year(),
		NextMonth:   int(nextMonthTime.Month()),
		Weeks:       weeks,
		Agenda:      agenda,
		IsAdmin:     isAdmin,
	}, nil
}

func (h *CalendarHandler) fetchTasksForDateRange(ctx context.Context, startDate, endDate string) (map[string][]models.Task, error) {
	rows, err := h.db.Query(ctx, `SELECT `+taskColumns+` FROM kalender_tasks WHERE task_date >= $1 AND task_date <= $2 ORDER BY task_date ASC, start_time ASC`, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]models.Task)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		result[t.TaskDate] = append(result[t.TaskDate], t)
	}

	return result, rows.Err()
}
