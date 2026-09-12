package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	Env           string
	DatabaseURL   string
	AdminPassword string
	SessionSecret string
}

func LoadConfig() *Config {
	// Try loading .env file (ignore error if running in docker with env vars)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or failed to load; reading environment variables directly.")
	}

	port := getEnv("PORT", "8080")
	env := getEnv("ENV", "development")
	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/calendar_db?sslmode=disable")
	adminPass := getEnv("ADMIN_PASSWORD", "adminsecret123")
	sessionSecret := getEnv("SESSION_SECRET", "super-secret-key-32-chars-min-len!!")

	return &Config{
		Port:          port,
		Env:           env,
		DatabaseURL:   dbURL,
		AdminPassword: adminPass,
		SessionSecret: sessionSecret,
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
