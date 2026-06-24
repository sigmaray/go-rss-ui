package services

import (
	"database/sql"
	"fmt"
	"strings"

	"go-rss-ui/config"
	"go-rss-ui/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ensurePrimaryDatabase() {
	if database.DB == nil {
		database.Connect()
	}
}

func GetAdminDSN() string {
	host, user, password, _, port := config.GetDBConfig()
	sslmode := config.GetEnvOrDefault("RSS_DB_SSLMODE", "disable")

	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=postgres port=%s sslmode=%s",
		host,
		user,
		password,
		port,
		sslmode,
	)
}

func GetAppDSN() string {
	return config.GetDSN()
}

func GetDBName() string {
	_, _, _, dbname, _ := config.GetDBConfig()
	return dbname
}

func openAdminSQLDB() (*sql.DB, error) {
	db, err := gorm.Open(postgres.Open(GetAdminDSN()), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db.DB()
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "duplicate key") ||
		strings.Contains(errStr, "unique constraint") ||
		strings.Contains(errStr, "23505") ||
		strings.Contains(errStr, "unique constraint failed")
}
