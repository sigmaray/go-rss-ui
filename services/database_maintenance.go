package services

import (
	"fmt"
	"os"
	"os/exec"

	"go-rss-ui/config"
	"go-rss-ui/database"
)

type DropAllTablesResult struct {
	DroppedCount int
	Errors       []string
	TableNames   []string
}

func MigrateDatabase() error {
	return database.RunMigrations(GetAppDSN())
}

func RollbackMigration() error {
	return database.RollbackMigration(GetAppDSN())
}

func MigrationStatus() error {
	return database.MigrationStatus(GetAppDSN())
}

func DropAllTables() (DropAllTablesResult, error) {
	ensurePrimaryDatabase()

	var tables []struct {
		TableName string `gorm:"column:tablename"`
	}

	if err := database.DB.Raw(`
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename
	`).Scan(&tables).Error; err != nil {
		return DropAllTablesResult{}, fmt.Errorf("failed to get list of tables: %w", err)
	}

	result := DropAllTablesResult{
		TableNames: make([]string, 0, len(tables)),
	}

	if len(tables) == 0 {
		return result, nil
	}

	for _, table := range tables {
		tableName := table.TableName
		result.TableNames = append(result.TableNames, tableName)

		query := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", quoteIdentifier(tableName))
		if err := database.DB.Exec(query).Error; err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", tableName, err.Error()))
			continue
		}

		result.DroppedCount++
	}

	return result, nil
}

func DropDatabase() error {
	dbname := GetDBName()
	sqlDB, err := openAdminSQLDB()
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()

	_, _ = sqlDB.Exec(`
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = $1
		AND pid <> pg_backend_pid()
	`, dbname)

	_, err = sqlDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", quoteIdentifier(dbname)))
	return err
}

func CreateDatabase() (bool, error) {
	dbname := GetDBName()
	sqlDB, err := openAdminSQLDB()
	if err != nil {
		return false, err
	}
	defer func() { _ = sqlDB.Close() }()

	var exists bool
	err = sqlDB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)",
		dbname,
	).Scan(&exists)
	if err != nil {
		return false, err
	}

	if exists {
		return false, nil
	}

	_, err = sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", quoteIdentifier(dbname)))
	if err != nil {
		return false, err
	}

	return true, nil
}

func DumpDBStructure() error {
	host, user, password, dbname, port := config.GetDBConfig()
	sslmode := config.GetEnvOrDefault("GO_RSS_UI_DB_SSLMODE", "disable")

	cmd := exec.Command(
		"pg_dump",
		"--host", host,
		"--port", port,
		"--username", user,
		"--dbname", dbname,
		"--schema-only",
		"--no-owner",
		"--no-privileges",
		"--format", "plain",
	)

	cmd.Env = append(os.Environ(), "PGPASSWORD="+password)
	if sslmode != "disable" {
		cmd.Env = append(cmd.Env, "PGSSLMODE="+sslmode)
	}

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to dump database structure: %w", err)
	}

	if err := os.WriteFile("structure.sql", output, 0644); err != nil {
		return fmt.Errorf("failed to write structure.sql file: %w", err)
	}

	return nil
}
