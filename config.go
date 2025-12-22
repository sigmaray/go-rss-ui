package main

import (
	"fmt"
	"log"
	"os"

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

