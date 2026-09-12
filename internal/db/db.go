package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InitDB(databaseURL string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database url: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 2
	config.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err == nil {
		if pingErr := pool.Ping(ctx); pingErr == nil {
			log.Println("Connected to database")
			if err := runMigrations(pool); err != nil {
				log.Printf("Migration warning: %v", err)
			}
			return pool, nil
		} else {
			pool.Close()
			dbName := config.ConnConfig.Database
			if ensureDatabaseExists(databaseURL, dbName) {
				pool, err = pgxpool.NewWithConfig(ctx, config)
				if err == nil && pool.Ping(ctx) == nil {
					log.Printf("Connected to database '%s'", dbName)
					if err := runMigrations(pool); err != nil {
						log.Printf("Migration warning: %v", err)
					}
					return pool, nil
				}
			}
			return nil, fmt.Errorf("database ping failed: %w", pingErr)
		}
	}

	return nil, fmt.Errorf("unable to connect to database: %w", err)
}

func ensureDatabaseExists(databaseURL, dbName string) bool {
	if dbName == "" || dbName == "postgres" {
		return false
	}

	adminConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return false
	}
	adminConfig.ConnConfig.Database = "postgres"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	adminPool, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		return false
	}
	defer adminPool.Close()

	query := fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)
	_, err = adminPool.Exec(ctx, query)
	if err != nil {
		return false
	}

	log.Printf("Database '%s' created", dbName)
	return true
}

func runMigrations(pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	schemaSQL := `
	CREATE TABLE IF NOT EXISTS kalender_tasks (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		task_date DATE NOT NULL,
		start_time TIME NOT NULL,
		end_time TIME NOT NULL,
		title VARCHAR(255) NOT NULL,
		description TEXT,
		is_booked BOOLEAN DEFAULT FALSE,
		requested_by_name VARCHAR(255),
		requested_by_email VARCHAR(255),
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	ALTER TABLE kalender_tasks ADD COLUMN IF NOT EXISTS requested_by_name VARCHAR(255);
	ALTER TABLE kalender_tasks ADD COLUMN IF NOT EXISTS requested_by_email VARCHAR(255);

	CREATE INDEX IF NOT EXISTS idx_kalender_tasks_date ON kalender_tasks(task_date);
	`
	_, err := pool.Exec(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM kalender_tasks").Scan(&count)
	if err == nil && count == 0 {
		migrationFile := "migrations/001_create_tasks.sql"
		if content, err := os.ReadFile(migrationFile); err == nil {
			_, _ = pool.Exec(ctx, string(content))
			log.Println("Database seeded from migration file")
		} else {
			seedSQL := `
			INSERT INTO kalender_tasks (task_date, start_time, end_time, title, description, is_booked)
			VALUES 
			  (CURRENT_DATE, '08:00', '10:00', 'Discovery Meeting', 'Project scope', false),
			  (CURRENT_DATE, '10:30', '12:00', 'Architecture Review', 'Schema design', false),
			  (CURRENT_DATE, '13:00', '15:00', 'Backend Development', 'API integration', true),
			  (CURRENT_DATE + INTERVAL '1 day', '09:00', '11:00', 'UI Component Styling', 'Layout design', false),
			  (CURRENT_DATE + INTERVAL '1 day', '14:00', '16:00', 'Team Sync', 'Sprint review', true),
			  (CURRENT_DATE + INTERVAL '3 days', '08:30', '11:30', 'Code Audit', 'Performance tuning', true),
			  (CURRENT_DATE + INTERVAL '5 days', '10:00', '12:00', 'Sprint Retrospective', 'Backlog grooming', false);
			`
			_, _ = pool.Exec(ctx, seedSQL)
			log.Println("Database seeded with initial records")
		}
	}

	return nil
}
