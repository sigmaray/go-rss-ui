package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Load loads environment variables from .env file
func Load() {
	// Try to load .env file, but don't fail if it doesn't exist
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: .env file not found, using environment variables or defaults")
	}
}

// GetDSN constructs a PostgreSQL DSN string from environment variables
func GetDSN() string {
	// First, check if RSS_DATABASE_URL is set (takes precedence)
	if dsn := os.Getenv("RSS_DATABASE_URL"); dsn != "" {
		return dsn
	}

	// Otherwise, construct DSN from individual parameters
	host := GetEnvOrDefault("RSS_DB_HOST", "localhost")
	user := GetEnvOrDefault("RSS_DB_USER", "postgres")
	password := GetEnvOrDefault("RSS_DB_PASSWORD", "postgres")
	dbname := GetEnvOrDefault("RSS_DB_NAME", "go_rss_ui_2")
	port := GetEnvOrDefault("RSS_DB_PORT", "5432")
	sslmode := GetEnvOrDefault("RSS_DB_SSLMODE", "disable")
	timezone := GetEnvOrDefault("RSS_DB_TIMEZONE", "Asia/Shanghai")

	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		host, user, password, dbname, port, sslmode, timezone)
}

// GetDBConfig returns individual database configuration parameters
func GetDBConfig() (host, user, password, dbname, port string) {
	host = GetEnvOrDefault("RSS_DB_HOST", "localhost")
	user = GetEnvOrDefault("RSS_DB_USER", "postgres")
	password = GetEnvOrDefault("RSS_DB_PASSWORD", "postgres")
	dbname = GetEnvOrDefault("RSS_DB_NAME", "go_rss_ui_2")
	port = GetEnvOrDefault("RSS_DB_PORT", "5432")
	return
}

// GetEnvOrDefault returns the value of an environment variable or a default value
func GetEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getBoolEnv(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	value = strings.ToLower(strings.TrimSpace(value))
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

// GetBackgroundFetchEnabled returns whether background feed fetching is enabled
// Returns true by default if the variable is not set or empty
func GetBackgroundFetchEnabled() bool {
	return getBoolEnv("RSS_BACKGROUND_FETCH_ENABLED", true)
}

// GetBackgroundFetchInterval returns the background fetch interval in seconds
// Returns 60 by default if the variable is not set or invalid
func GetBackgroundFetchInterval() int {
	value := os.Getenv("RSS_BACKGROUND_FETCH_INTERVAL")
	if value == "" {
		return 60
	}
	interval, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || interval <= 0 {
		log.Printf("Warning: Invalid RSS_BACKGROUND_FETCH_INTERVAL value '%s', using default 60 seconds", value)
		return 60
	}
	return interval
}

// IsCypressMode returns whether the application is running in Cypress mode
// Returns true if RSS_CYPRESS environment variable is set to "true"
func IsCypressMode() bool {
	return getBoolEnv("RSS_CYPRESS", false)
}

// GetRedisHost returns Redis host from environment variable
// Returns "localhost" by default
func GetRedisHost() string {
	return GetEnvOrDefault("RSS_REDIS_HOST", "localhost")
}

// GetRedisPort returns Redis port from environment variable
// Returns "6379" by default
func GetRedisPort() string {
	return GetEnvOrDefault("RSS_REDIS_PORT", "6379")
}

// GetRedisPassword returns Redis password from environment variable
// Returns empty string by default
func GetRedisPassword() string {
	return os.Getenv("RSS_REDIS_PASSWORD")
}

// GetRedisAddr returns Redis address in format "host:port"
func GetRedisAddr() string {
	return fmt.Sprintf("%s:%s", GetRedisHost(), GetRedisPort())
}

// GetServerPort returns the server port from environment variable
// Returns 8082 by default if the variable is not set or invalid
func GetServerPort() string {
	value := os.Getenv("RSS_PORT")
	if value == "" {
		return "8082"
	}
	// Validate that it's a valid port number
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port <= 0 || port > 65535 {
		log.Printf("Warning: Invalid RSS_PORT value '%s', using default 8082", value)
		return "8082"
	}
	return value
}

// IsProduction returns true when the app is running in production mode.
func IsProduction() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("RSS_ENV")), "production") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("GIN_MODE")), "release")
}

// GetSessionSecret returns the configured session secret.
// In production, RSS_SESSION_SECRET must be set and should be at least 32 characters long.
// In non-production, a random ephemeral secret is generated when the variable is missing.
func GetSessionSecret() string {
	secret := strings.TrimSpace(os.Getenv("RSS_SESSION_SECRET"))
	if secret != "" {
		if len(secret) < 32 {
			log.Printf("Warning: RSS_SESSION_SECRET is shorter than 32 characters")
		}
		return secret
	}

	if IsProduction() {
		log.Fatal("RSS_SESSION_SECRET must be set in production and should be at least 32 characters long")
	}

	generatedSecret := make([]byte, 32)
	if _, err := rand.Read(generatedSecret); err != nil {
		log.Fatal("failed to generate development session secret: ", err)
	}

	log.Printf("Warning: RSS_SESSION_SECRET is not set; using an ephemeral development secret")
	return base64.StdEncoding.EncodeToString(generatedSecret)
}

// GetSessionSecure returns whether secure cookies should be used.
// Secure cookies are enabled automatically in production and can be overridden explicitly.
func GetSessionSecure() bool {
	if os.Getenv("RSS_SESSION_SECURE") != "" {
		return getBoolEnv("RSS_SESSION_SECURE", false)
	}

	return IsProduction()
}
