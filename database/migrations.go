package database

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

const migrationsDir = "migrations"

func openSQLDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return db, nil
}

func configureGoose() error {
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	return nil
}

func RunMigrations(dsn string) error {
	if err := configureGoose(); err != nil {
		return err
	}

	db, err := openSQLDB(dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func RollbackMigration(dsn string) error {
	if err := configureGoose(); err != nil {
		return err
	}

	db, err := openSQLDB(dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := goose.Down(db, migrationsDir); err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	return nil
}

func MigrationStatus(dsn string) error {
	if err := configureGoose(); err != nil {
		return err
	}

	db, err := openSQLDB(dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := goose.Status(db, migrationsDir); err != nil {
		return fmt.Errorf("failed to get migration status: %w", err)
	}

	return nil
}
