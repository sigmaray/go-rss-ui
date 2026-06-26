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
	case "automigrate":
		commands.CommandMigrate()
	case "db-create":
		commands.CommandCreateDB()
	case "db-drop":
		commands.CommandDropDB()
	case "db-dump-structure":
		commands.CommandDumpDBStructure()
	case "drop-all-tables":
		commands.CommandDropAllTables()
	case "execute-sql":
		commands.CommandExecuteSQL()
	case "feeds-fetch":
		commands.CommandFetchFeeds()
	case "feeds-seed":
		commands.CommandSeedFeeds()
	case "migrate":
		commands.CommandMigrate()
	case "migrate-down":
		commands.CommandMigrateDown()
	case "migrate-status":
		commands.CommandMigrateStatus()
	case "s", "server":
		return false
	case "users-clear":
		commands.CommandClearUsers()
	case "users-create":
		commands.CommandCreateUser()
	case "users-seed":
		commands.CommandSeedUsers()
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
	fmt.Println("  automigrate       - Create tables in database using AutoMigrate")
	fmt.Println("  db-create         - Create the application database")
	fmt.Println("  db-drop           - Delete the application database")
	fmt.Println("  db-dump-structure - Dump database structure to structure.sql file")
	fmt.Println("  drop-all-tables   - Drop all tables in the database")
	fmt.Println("  execute-sql       - Execute SQL query (provide query as argument or via stdin)")
	fmt.Println("  feeds-fetch       - Fetch and process all RSS feeds")
	fmt.Println("  feeds-seed        - Create default RSS feeds")
	fmt.Println("  migrate           - Run pending database migrations (goose)")
	fmt.Println("  migrate-down      - Roll back the last database migration")
	fmt.Println("  migrate-status    - Show database migration status")
	fmt.Println("  server, s         - Start the web server")
	fmt.Println("  users-clear       - Clear all data from users table")
	fmt.Println("  users-create      - Create a new user interactively")
	fmt.Println("  users-seed        - Create a standard admin user")
}
