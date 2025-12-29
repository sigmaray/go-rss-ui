package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

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

// CommandSeedUsers creates a standard admin user
func CommandSeedUsers() {
	ConnectDatabase()
	SeedUsers()
}

// CommandSeedFeeds creates default RSS feeds
func CommandSeedFeeds() {
	ConnectDatabase()
	result := SeedFeeds()
	log.Printf("Seeded feeds: %d created, %d already existed, %d errors", result.Created, result.Existed, result.Errors)
}

// SeedUsers creates admin user if it doesn't exist
func SeedUsers() {
	// Seed admin user
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

// GetDefaultFeeds returns the list of default RSS feeds to seed
func GetDefaultFeeds() []string {
	return []string{
		"https://feeds.bbci.co.uk/news/rss.xml",
		"http://rss.cnn.com/rss/cnn_topstories.rss",
		"https://www.wired.com/feed/rss",
		"https://habr.com/ru/rss/articles/?fl=ru",
	}
}

// SeedFeedsResult contains the results of seeding feeds
type SeedFeedsResult struct {
	Created int
	Existed int
	Errors  int
}

// SeedFeeds creates default RSS feeds if they don't exist
// Returns statistics about the operation
func SeedFeeds() SeedFeedsResult {
	return SeedFeedsWithURLs(GetDefaultFeeds())
}

// SeedFeedsWithURLs creates RSS feeds from the provided URLs if they don't exist
// Returns statistics about the operation
func SeedFeedsWithURLs(feedURLs []string) SeedFeedsResult {
	result := SeedFeedsResult{}

	for _, feedURL := range feedURLs {
		var feed Feed
		dbResult := DB.Where("url = ?", feedURL).First(&feed)
		if dbResult.Error == gorm.ErrRecordNotFound {
			feed := Feed{URL: feedURL}
			if err := DB.Create(&feed).Error; err != nil {
				log.Printf("Failed to create feed %s: %v", feedURL, err)
				result.Errors++
			} else {
				log.Printf("Feed created: %s", feedURL)
				result.Created++
			}
		} else if dbResult.Error != nil {
			log.Printf("Failed to check for existing feed %s: %v", feedURL, dbResult.Error)
			result.Errors++
		} else {
			log.Printf("Feed already exists: %s", feedURL)
			result.Existed++
		}
	}

	return result
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

// CommandFetchFeeds fetches all RSS feeds and processes their items
func CommandFetchFeeds() {
	ConnectDatabase()

	log.Println("Starting feed fetch...")
	itemsCreated, itemsUpdated, errors := processAllFeeds()
	log.Printf("Feed fetch completed: %d items created, %d items updated, %d errors", itemsCreated, itemsUpdated, errors)
}

// CommandExecuteSQL executes a SQL query from command line
func CommandExecuteSQL() {
	ConnectDatabase()

	// Get SQL query from command line arguments or stdin
	var sqlQuery string
	if len(os.Args) > 2 {
		// SQL query provided as arguments (join all arguments after "execute-sql")
		sqlQuery = strings.Join(os.Args[2:], " ")
	} else {
		// Read from stdin
		fmt.Print("Enter SQL query (end with semicolon and newline or Ctrl+D):\n> ")
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			line := scanner.Text()
			lines = append(lines, line)
			// Check if line ends with semicolon (end of query)
			if strings.HasSuffix(strings.TrimSpace(line), ";") {
				break
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatalf("Error reading input: %v", err)
		}
		if len(lines) == 0 {
			log.Fatal("No SQL query provided")
		}
		sqlQuery = strings.Join(lines, " ")
	}

	if strings.TrimSpace(sqlQuery) == "" {
		log.Fatal("SQL query cannot be empty")
	}

	// Execute SQL query
	var results []map[string]interface{}
	rows, err := DB.Raw(sqlQuery).Rows()
	if err != nil {
		log.Fatalf("Error executing SQL query: %v", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		log.Fatalf("Error getting columns: %v", err)
	}

	// Scan rows
	for rows.Next() {
		// Create a slice of interface{} to hold column values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			log.Fatalf("Error scanning row: %v", err)
		}

		// Create a map for this row
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string for better representation
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("Error iterating rows: %v", err)
	}

	// Print results
	if len(columns) == 0 {
		// Query doesn't return rows (INSERT, UPDATE, DELETE, etc.)
		fmt.Println("\nQuery executed successfully (no rows returned).")
		return
	}

	fmt.Printf("\nQuery executed successfully. Found %d row(s).\n\n", len(results))

	if len(results) > 0 {
		// Calculate column widths for better formatting
		colWidths := make([]int, len(columns))
		for i, col := range columns {
			colWidths[i] = len(col)
			for _, row := range results {
				valStr := fmt.Sprintf("%v", row[col])
				if len(valStr) > colWidths[i] {
					colWidths[i] = len(valStr)
				}
			}
			// Limit max width
			if colWidths[i] > 50 {
				colWidths[i] = 50
			}
		}

		// Print header
		for i, col := range columns {
			if i > 0 {
				fmt.Print(" | ")
			}
			fmt.Printf("%-*s", colWidths[i], col)
		}
		fmt.Println()

		// Print separator
		totalWidth := 0
		for i, width := range colWidths {
			if i > 0 {
				totalWidth += 3 // " | "
			}
			totalWidth += width
		}
		fmt.Println(strings.Repeat("-", totalWidth))

		// Print rows
		for _, row := range results {
			for i, col := range columns {
				if i > 0 {
					fmt.Print(" | ")
				}
				val := row[col]
				valStr := "NULL"
				if val != nil {
					valStr = fmt.Sprintf("%v", val)
					// Truncate if too long
					if len(valStr) > 50 {
						valStr = valStr[:47] + "..."
					}
				}
				fmt.Printf("%-*s", colWidths[i], valStr)
			}
			fmt.Println()
		}
	} else {
		fmt.Println("(No rows returned)")
	}
}
