package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// getAdminDSN returns DSN for connecting to postgres database (for admin operations)
func getAdminDSN() string {
	host, user, password, _, port := GetDBConfig()
	sslmode := getEnvOrDefault("DB_SSLMODE", "disable")
	// Connect to postgres database for admin operations
	return fmt.Sprintf("host=%s user=%s password=%s dbname=postgres port=%s sslmode=%s",
		host, user, password, port, sslmode)
}

// getAppDSN returns DSN for connecting to application database
func getAppDSN() string {
	return GetDSN()
}

// getDBName extracts database name from configuration
func getDBName() string {
	_, _, _, dbname, _ := GetDBConfig()
	return dbname
}

// CommandClearUsers clears all data from users table
func CommandClearUsers() {
	ConnectDatabase()
	
	result := DB.Exec("DELETE FROM users")
	if result.Error != nil {
		log.Fatalf("Failed to clear users table: %v", result.Error)
	}
	
	log.Printf("Successfully cleared %d records from users table", result.RowsAffected)
}

// CommandSeed creates a standard admin user
func CommandSeed() {
	ConnectDatabase()
	
	var user User
	result := DB.Where("username = ?", "admin").First(&user)
	if result.Error == gorm.ErrRecordNotFound {
		adminUser := User{Username: "admin", Password: "password"}
		if err := DB.Create(&adminUser).Error; err != nil {
			log.Fatalf("Failed to create admin user: %v", err)
		}
		log.Println("Admin user 'admin' created with password 'password'")
	} else if result.Error != nil {
		log.Fatalf("Failed to check for existing user: %v", result.Error)
	} else {
		log.Println("Admin user already exists")
	}
}

// CommandMigrate creates tables in the database using AutoMigrate
func CommandMigrate() {
	dsn := getAppDSN()
	
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	
	// Run AutoMigrate for all models
	err = db.AutoMigrate(&User{}, &Feed{}, &Item{})
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	
	log.Println("Database migration completed successfully")
}

// CommandDropDB drops the application database
func CommandDropDB() {
	dbname := getDBName()
	adminDSN := getAdminDSN()
	
	// Connect to postgres database using GORM
	db, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to postgres database: %v", err)
	}
	
	// Get underlying sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get database connection: %v", err)
	}
	defer sqlDB.Close()
	
	// Terminate all connections to the target database
	_, err = sqlDB.Exec(fmt.Sprintf(`
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = '%s'
		AND pid <> pg_backend_pid();
	`, dbname))
	if err != nil {
		log.Printf("Warning: Failed to terminate connections: %v", err)
	}
	
	// Drop the database (quote identifier to handle special characters)
	_, err = sqlDB.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbname))
	if err != nil {
		log.Fatalf("Failed to drop database: %v", err)
	}
	
	log.Printf("Database '%s' dropped successfully", dbname)
}

// CommandCreateDB creates the application database
func CommandCreateDB() {
	dbname := getDBName()
	adminDSN := getAdminDSN()
	
	// Connect to postgres database using GORM
	db, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to postgres database: %v", err)
	}
	
	// Get underlying sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get database connection: %v", err)
	}
	defer sqlDB.Close()
	
	// Check if database already exists
	var exists bool
	err = sqlDB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)",
		dbname,
	).Scan(&exists)
	if err != nil {
		log.Fatalf("Failed to check if database exists: %v", err)
	}
	
	if exists {
		log.Printf("Database '%s' already exists", dbname)
		return
	}
	
	// Create the database (quote identifier to handle special characters)
	_, err = sqlDB.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, dbname))
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}
	
	log.Printf("Database '%s' created successfully", dbname)
}

