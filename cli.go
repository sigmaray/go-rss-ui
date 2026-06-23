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
		return false
	}

	command := os.Args[1]
	switch command {
	case "clear-users":
		commands.CommandClearUsers()
	case "seed-users":
		commands.CommandSeedUsers()
	case "seed-feeds":
		commands.CommandSeedFeeds()
	case "migrate":
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
		fmt.Println("\nAvailable commands:")
		printAvailableCommands()
		os.Exit(1)
	}

	return true
}

func showStartupInfo() {
	port := config.GetServerPort()
	fmt.Println("=" + strings.Repeat("=", 70) + "=")
	fmt.Println("  Go RSS UI Application")
	fmt.Println("=" + strings.Repeat("=", 70) + "=")
	fmt.Println()
	fmt.Printf("Starting web server on http://localhost:%s\n", port)
	fmt.Println()
	fmt.Println("When you run the application without a command, it starts the web server.")
	fmt.Printf("You can access the application in your browser at http://localhost:%s\n", port)
	fmt.Println()
	fmt.Println("Available CLI commands:")
	fmt.Println()
	printAvailableCommands()
	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 70) + "=")
	fmt.Println()
}

func printAvailableCommands() {
	fmt.Println("  clear-users  - Clear all data from users table")
	fmt.Println("  seed-users   - Create a standard admin user")
	fmt.Println("  seed-feeds   - Create default RSS feeds")
	fmt.Println("  fetch-feeds  - Fetch and process all RSS feeds")
	fmt.Println("  execute-sql  - Execute SQL query (provide query as argument or via stdin)")
	fmt.Println("  migrate      - Create tables in database using AutoMigrate")
	fmt.Println("  drop-db      - Delete the application database")
	fmt.Println("  drop-all-tables - Drop all tables in the database")
	fmt.Println("  create-db    - Create the application database")
	fmt.Println("  dump-db-structure - Dump database structure to structure.sql file")
}
