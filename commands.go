package main

import (
	"bufio"
	"fmt"
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
		appLogger.Fatal().Err(result.Error).Msg("Failed to clear users table")
	}

	appLogger.Info().Int64("rows_affected", result.RowsAffected).Msg("Successfully cleared users table")
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
	appLogger.Info().
		Int("created", result.Created).
		Int("existed", result.Existed).
		Int("errors", result.Errors).
		Msg("Seeded feeds")
}

// SeedUsers creates admin user if it doesn't exist
func SeedUsers() {
	// Seed admin user
	var user User
	result := DB.Where("username = ?", "admin").First(&user)
	if result.Error == gorm.ErrRecordNotFound {
		adminUser := User{Username: "admin", Password: "password"}
		if err := DB.Create(&adminUser).Error; err != nil {
			appLogger.Fatal().Err(err).Msg("Failed to create admin user")
		}
		appLogger.Info().Msg("Admin user 'admin' created with password 'password'")
	} else if result.Error != nil {
		appLogger.Fatal().Err(result.Error).Msg("Failed to check for existing user")
	} else {
		appLogger.Info().Msg("Admin user already exists")
	}
}

// GetDefaultFeeds returns the list of default RSS feeds to seed
func GetDefaultFeeds() []string {
	return []string{
		"https://feeds.bbci.co.uk/news/rss.xml",
		"http://rss.cnn.com/rss/cnn_topstories.rss",
		"https://www.wired.com/feed/rss",
		"https://habr.com/ru/rss/articles/?fl=ru",
		"https://www.telegraph.co.uk/rss.xml",
		"https://abcnews.go.com/abcnews/topstories",
		"https://feeds.content.dowjones.io/public/rss/RSSWorldNews",
		"https://feeds.skynews.com/feeds/rss/home.xml",
		"https://www.theguardian.com/international/rss",
		"https://feeds.nbcnews.com/nbcnews/public/news",
		"https://www.theguardian.com/world/rss",
		"https://xkcd.com/atom.xml",
		"https://fedoramagazine.org/feed/",
		"https://planet.gnome.org//rss20.xml",
		"https://hacks.mozilla.org/feed/",
		// "https://www.reddit.com/r/news/.rss",
		// "http://www.reddit.com/.rss",
		// "https://www.reddit.com/r/FoodPorn/.rss",
		// "https://www.reddit.com/r/food/.rss",
		// "https://www.reddit.com/r/Outdoors/.rss",
		"https://www.reddit.com/r/all/new/.rss",
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
				appLogger.Error().Err(err).Str("feed_url", feedURL).Msg("Failed to create feed")
				result.Errors++
			} else {
				appLogger.Info().Str("feed_url", feedURL).Msg("Feed created")
				result.Created++
			}
		} else if dbResult.Error != nil {
			appLogger.Error().Err(dbResult.Error).Str("feed_url", feedURL).Msg("Failed to check for existing feed")
			result.Errors++
		} else {
			appLogger.Debug().Str("feed_url", feedURL).Msg("Feed already exists")
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
		appLogger.Fatal().Err(err).Msg("Failed to connect to database")
	}

	// Run AutoMigrate for all models
	err = db.AutoMigrate(&User{}, &Feed{}, &Item{})
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to migrate database")
	}

	appLogger.Info().Msg("Database migration completed successfully")
}

// DropAllTablesResult contains the result of dropping all tables
type DropAllTablesResult struct {
	DroppedCount int
	Errors       []string
	TableNames   []string
}

// DropAllTables drops all tables in the database and returns the result
// This is a shared function used by both web handler and CLI command
func DropAllTables() (DropAllTablesResult, error) {
	// Get list of all tables in the public schema
	var tables []struct {
		TableName string `gorm:"column:tablename"`
	}

	// Query pg_tables to get all user tables in public schema
	if err := DB.Raw(`
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

	// Drop all tables using CASCADE to automatically drop dependent objects
	for _, table := range tables {
		tableName := table.TableName
		result.TableNames = append(result.TableNames, tableName)
		
		// Quote table name to handle special characters
		// Using CASCADE to automatically drop dependent objects (constraints, indexes, etc.)
		if err := DB.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s" CASCADE`, tableName)).Error; err != nil {
			errorMsg := fmt.Sprintf("%s: %s", tableName, err.Error())
			result.Errors = append(result.Errors, errorMsg)
		} else {
			result.DroppedCount++
		}
	}

	return result, nil
}

// CommandDropAllTables drops all tables in the database
func CommandDropAllTables() {
	ConnectDatabase()

	result, err := DropAllTables()
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to drop all tables")
	}

	if len(result.TableNames) == 0 {
		appLogger.Info().Msg("No tables found in database")
		return
	}

	// Log each table drop
	for _, tableName := range result.TableNames {
		// Check if this table had an error
		hadError := false
		for _, errMsg := range result.Errors {
			if strings.HasPrefix(errMsg, tableName+":") {
				hadError = true
				appLogger.Error().Str("table", tableName).Msg("Failed to drop table")
				break
			}
		}
		if !hadError {
			appLogger.Info().Str("table", tableName).Msg("Table dropped successfully")
		}
	}

	if len(result.Errors) > 0 {
		errorMsg := fmt.Sprintf("Dropped %d table(s), but encountered errors: %s", result.DroppedCount, strings.Join(result.Errors, "; "))
		appLogger.Error().Msg(errorMsg)
		os.Exit(1)
	}

	appLogger.Info().Int("count", result.DroppedCount).Msg("All tables dropped successfully")
}

// CommandDropDB drops the application database
func CommandDropDB() {
	dbname := getDBName()
	adminDSN := getAdminDSN()

	// Connect to postgres database using GORM
	db, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{})
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to connect to postgres database")
	}

	// Get underlying sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to get database connection")
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			appLogger.Error().Err(err).Msg("Error closing database connection")
		}
	}()

	// Terminate all connections to the target database
	_, err = sqlDB.Exec(fmt.Sprintf(`
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = '%s'
		AND pid <> pg_backend_pid();
	`, dbname))
	if err != nil {
		appLogger.Warn().Err(err).Msg("Failed to terminate connections")
	}

	// Drop the database (quote identifier to handle special characters)
	_, err = sqlDB.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbname))
	if err != nil {
		appLogger.Fatal().Err(err).Str("database", dbname).Msg("Failed to drop database")
	}

	appLogger.Info().Str("database", dbname).Msg("Database dropped successfully")
}

// CommandCreateDB creates the application database
func CommandCreateDB() {
	dbname := getDBName()
	adminDSN := getAdminDSN()

	// Connect to postgres database using GORM
	db, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{})
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to connect to postgres database")
	}

	// Get underlying sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to get database connection")
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			appLogger.Error().Err(err).Msg("Error closing database connection")
		}
	}()

	// Check if database already exists
	var exists bool
	err = sqlDB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)",
		dbname,
	).Scan(&exists)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to check if database exists")
	}

	if exists {
		appLogger.Info().Str("database", dbname).Msg("Database already exists")
		return
	}

	// Create the database (quote identifier to handle special characters)
	_, err = sqlDB.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, dbname))
	if err != nil {
		appLogger.Fatal().Err(err).Str("database", dbname).Msg("Failed to create database")
	}

	appLogger.Info().Str("database", dbname).Msg("Database created successfully")
}

// CommandFetchFeeds fetches all RSS feeds and processes their items
func CommandFetchFeeds() {
	ConnectDatabase()

	appLogger.Info().Msg("Starting feed fetch...")
	itemsCreated, itemsUpdated, errors := processAllFeeds()
	appLogger.Info().
		Int("items_created", itemsCreated).
		Int("items_updated", itemsUpdated).
		Int("errors", errors).
		Msg("Feed fetch completed")
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
			appLogger.Fatal().Err(err).Msg("Error reading input")
		}
		if len(lines) == 0 {
			appLogger.Fatal().Msg("No SQL query provided")
		}
		sqlQuery = strings.Join(lines, " ")
	}

	if strings.TrimSpace(sqlQuery) == "" {
		appLogger.Fatal().Msg("SQL query cannot be empty")
	}

	// Execute SQL query
	var results []map[string]interface{}
	rows, err := DB.Raw(sqlQuery).Rows()
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Error executing SQL query")
	}
	defer func() {
		if err := rows.Close(); err != nil {
			appLogger.Error().Err(err).Msg("Error closing rows")
		}
	}()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Error getting columns")
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
			appLogger.Fatal().Err(err).Msg("Error scanning row")
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
		appLogger.Fatal().Err(err).Msg("Error iterating rows")
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

