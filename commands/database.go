package commands

import (
	"fmt"
	"strings"

	"go-rss-ui/app"
	"go-rss-ui/services"
)

type DropAllTablesResult = services.DropAllTablesResult

func GetAdminDSN() string {
	return services.GetAdminDSN()
}

func GetAppDSN() string {
	return services.GetAppDSN()
}

func GetDBName() string {
	return services.GetDBName()
}

func CommandMigrate() {
	if err := services.MigrateDatabase(); err != nil {
		app.Logger.Fatal().Err(err).Msg("Failed to migrate database")
	}

	app.Logger.Info().Msg("Database migration completed successfully")
}

func CommandMigrateStatus() {
	if err := services.MigrationStatus(); err != nil {
		app.Logger.Fatal().Err(err).Msg("Failed to get migration status")
	}
}

func CommandMigrateDown() {
	if err := services.RollbackMigration(); err != nil {
		app.Logger.Fatal().Err(err).Msg("Failed to rollback migration")
	}

	app.Logger.Info().Msg("Database migration rolled back successfully")
}

func DropAllTables() (DropAllTablesResult, error) {
	return services.DropAllTables()
}

func CommandDropAllTables() {
	result, err := DropAllTables()
	if err != nil {
		app.Logger.Fatal().Err(err).Msg("Failed to drop all tables")
	}

	if len(result.TableNames) == 0 {
		app.Logger.Info().Msg("No tables found in database")
		return
	}

	for _, tableName := range result.TableNames {
		hadError := false
		for _, errMsg := range result.Errors {
			if strings.HasPrefix(errMsg, tableName+":") {
				hadError = true
				app.Logger.Error().Str("table", tableName).Msg("Failed to drop table")
				break
			}
		}
		if !hadError {
			app.Logger.Info().Str("table", tableName).Msg("Table dropped successfully")
		}
	}

	if len(result.Errors) > 0 {
		errorMsg := fmt.Sprintf(
			"Dropped %d table(s), but encountered errors: %s",
			result.DroppedCount,
			strings.Join(result.Errors, "; "),
		)
		app.Logger.Fatal().Msg(errorMsg)
	}

	app.Logger.Info().Int("count", result.DroppedCount).Msg("All tables dropped successfully")
}

func CommandDropDB() {
	dbname := GetDBName()
	if err := services.DropDatabase(); err != nil {
		app.Logger.Fatal().Err(err).Str("database", dbname).Msg("Failed to drop database")
	}

	app.Logger.Info().Str("database", dbname).Msg("Database dropped successfully")
}

func CommandCreateDB() {
	dbname := GetDBName()
	created, err := services.CreateDatabase()
	if err != nil {
		app.Logger.Fatal().Err(err).Str("database", dbname).Msg("Failed to create database")
	}

	if !created {
		app.Logger.Info().Str("database", dbname).Msg("Database already exists")
		return
	}

	app.Logger.Info().Str("database", dbname).Msg("Database created successfully")
}

func DumpDBStructure() error {
	return services.DumpDBStructure()
}

func CommandDumpDBStructure() {
	if err := DumpDBStructure(); err != nil {
		app.Logger.Fatal().Err(err).Msg("Failed to dump database structure")
	}

	app.Logger.Info().Str("file", "structure.sql").Msg("Database structure dumped successfully")
}
