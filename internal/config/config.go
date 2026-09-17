package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

const (
	defaultAdminPassword = "adminsecret123"
	defaultSessionSecret = "super-secret-key-32-chars-min-len!!"
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

	cfg := &Config{
		Port:          getEnv("PORT", "8080"),
		Env:           getEnv("ENV", "development"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/calendar_db?sslmode=disable"),
		AdminPassword: getEnv("ADMIN_PASSWORD", defaultAdminPassword),
		SessionSecret: getEnv("SESSION_SECRET", defaultSessionSecret),
	}

	if cfg.IsProduction() {
		if cfg.AdminPassword == defaultAdminPassword {
			log.Println("WARNING: ADMIN_PASSWORD is using the default value. Set a strong password in production.")
		}
		if cfg.SessionSecret == defaultSessionSecret || len(cfg.SessionSecret) < 32 {
			log.Println("WARNING: SESSION_SECRET is default or shorter than 32 characters. Admin sessions can be forged.")
		}
	}

	return cfg
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
