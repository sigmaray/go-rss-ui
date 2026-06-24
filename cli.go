package main

import (
	"fmt"
	"os"
	"strings"

	"go-rss-ui/commands"
	"go-rss-ui/config"
)

func runCLICommand() bool {
	if len(os.Args) <= 1 {
		showHelp()
		return true
	}

	command := os.Args[1]
	switch command {
	case "s", "server":
		return false
	case "clear-users":
		commands.CommandClearUsers()
	case "seed-users":
		commands.CommandSeedUsers()
	case "seed-feeds":
		commands.CommandSeedFeeds()
	case "migrate":
		commands.CommandMigrate()
	case "migrate-status":
		commands.CommandMigrateStatus()
	case "migrate-down":
		commands.CommandMigrateDown()
	case "automigrate":
		commands.CommandMigrate()
	case "drop-db":
		commands.CommandDropDB()
	case "drop-all-tables":
		commands.CommandDropAllTables()
	case "create-db":
		commands.CommandCreateDB()
	case "fetch-feeds":
		commands.CommandFetchFeeds()
	case "execute-sql":
		commands.CommandExecuteSQL()
	case "dump-db-structure":
		commands.CommandDumpDBStructure()
	default:
		fmt.Println("Unknown command:", command)
		fmt.Println()
		showHelp()
		os.Exit(1)
	}

	return true
}

func showHelp() {
	fmt.Println("Go RSS UI Application")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  go run . server          Start the web server")
	fmt.Println("  go run . <command>       Run a CLI command")
	fmt.Println()
	fmt.Println("Available commands:")
	printAvailableCommands()
}

func showStartupInfo() {
	port := config.GetServerPort()
	fmt.Println("=" + strings.Repeat("=", 70) + "=")
	fmt.Println("  Go RSS UI Application")
	fmt.Println("=" + strings.Repeat("=", 70) + "=")
	fmt.Println()
	fmt.Printf("Starting web server on http://localhost:%s\n", port)
	fmt.Printf("You can access the application in your browser at http://localhost:%s\n", port)
	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 70) + "=")
	fmt.Println()
}

func printAvailableCommands() {
	fmt.Println("  server, s    - Start the web server")
	fmt.Println("  clear-users  - Clear all data from users table")
	fmt.Println("  seed-users   - Create a standard admin user")
	fmt.Println("  seed-feeds   - Create default RSS feeds")
	fmt.Println("  fetch-feeds  - Fetch and process all RSS feeds")
	fmt.Println("  execute-sql  - Execute SQL query (provide query as argument or via stdin)")
	fmt.Println("  migrate        - Run pending database migrations (goose)")
	fmt.Println("  migrate-status - Show database migration status")
	fmt.Println("  migrate-down   - Roll back the last database migration")
	fmt.Println("  automigrate    - Create tables in database using AutoMigrate")
	fmt.Println("  drop-db      - Delete the application database")
	fmt.Println("  drop-all-tables - Drop all tables in the database")
	fmt.Println("  create-db    - Create the application database")
	fmt.Println("  dump-db-structure - Dump database structure to structure.sql file")
}
