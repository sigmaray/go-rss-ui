package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// LoadConfig loads environment variables from .env file
func LoadConfig() {
	// Try to load .env file, but don't fail if it doesn't exist
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: .env file not found, using environment variables or defaults")
	}
}

// GetDSN constructs a PostgreSQL DSN string from environment variables
func GetDSN() string {
	// First, check if DATABASE_URL is set (takes precedence)
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}

	// Otherwise, construct DSN from individual parameters
	host := getEnvOrDefault("DB_HOST", "localhost")
	user := getEnvOrDefault("DB_USER", "postgres")
	password := getEnvOrDefault("DB_PASSWORD", "postgres")
	dbname := getEnvOrDefault("DB_NAME", "go_rss_ui_2")
	port := getEnvOrDefault("DB_PORT", "5432")
	sslmode := getEnvOrDefault("DB_SSLMODE", "disable")
	timezone := getEnvOrDefault("DB_TIMEZONE", "Asia/Shanghai")

	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		host, user, password, dbname, port, sslmode, timezone)
}

// GetDBConfig returns individual database configuration parameters
func GetDBConfig() (host, user, password, dbname, port string) {
	host = getEnvOrDefault("DB_HOST", "localhost")
	user = getEnvOrDefault("DB_USER", "postgres")
	password = getEnvOrDefault("DB_PASSWORD", "postgres")
	dbname = getEnvOrDefault("DB_NAME", "go_rss_ui_2")
	port = getEnvOrDefault("DB_PORT", "5432")
	return
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// GetBackgroundFetchEnabled returns whether background feed fetching is enabled
// Returns true by default if the variable is not set or empty
func GetBackgroundFetchEnabled() bool {
	value := os.Getenv("BACKGROUND_FETCH_ENABLED")
	if value == "" {
		return true
	}
	// Parse as boolean (case-insensitive)
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

// GetBackgroundFetchInterval returns the background fetch interval in seconds
// Returns 60 by default if the variable is not set or invalid
func GetBackgroundFetchInterval() int {
	value := os.Getenv("BACKGROUND_FETCH_INTERVAL")
	if value == "" {
		return 60
	}
	interval, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || interval <= 0 {
		log.Printf("Warning: Invalid BACKGROUND_FETCH_INTERVAL value '%s', using default 60 seconds", value)
		return 60
	}
	return interval
}

