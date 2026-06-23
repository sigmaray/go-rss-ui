package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"go-rss-ui/app"
	"go-rss-ui/services"
)

func CommandExecuteSQL() {
	sqlQuery := readSQLQueryFromInput()
	if strings.TrimSpace(sqlQuery) == "" {
		app.Logger.Fatal().Msg("SQL query cannot be empty")
	}

	result, err := services.RunSQLQuery(sqlQuery)
	if err != nil {
		app.Logger.Fatal().Err(err).Msg("Error executing SQL query")
	}

	printSQLQueryResult(result)
}

func readSQLQueryFromInput() string {
	if len(os.Args) > 2 {
		return strings.Join(os.Args[2:], " ")
	}

	fmt.Print("Enter SQL query (end with semicolon and newline or Ctrl+D):\n> ")
	scanner := bufio.NewScanner(os.Stdin)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		if strings.HasSuffix(strings.TrimSpace(line), ";") {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		app.Logger.Fatal().Err(err).Msg("Error reading input")
	}
	if len(lines) == 0 {
		app.Logger.Fatal().Msg("No SQL query provided")
	}

	return strings.Join(lines, " ")
}

func printSQLQueryResult(result services.SQLQueryResult) {
	if len(result.Columns) == 0 {
		fmt.Println("\nQuery executed successfully (no rows returned).")
		return
	}

	fmt.Printf("\nQuery executed successfully. Found %d row(s).\n\n", len(result.Rows))

	if len(result.Rows) == 0 {
		fmt.Println("(No rows returned)")
		return
	}

	colWidths := make([]int, len(result.Columns))
	for i, col := range result.Columns {
		colWidths[i] = len(col)
		for _, row := range result.Rows {
			valStr := fmt.Sprintf("%v", row[col])
			if len(valStr) > colWidths[i] {
				colWidths[i] = len(valStr)
			}
		}
		if colWidths[i] > 50 {
			colWidths[i] = 50
		}
	}

	for i, col := range result.Columns {
		if i > 0 {
			fmt.Print(" | ")
		}
		fmt.Printf("%-*s", colWidths[i], col)
	}
	fmt.Println()

	totalWidth := 0
	for i, width := range colWidths {
		if i > 0 {
			totalWidth += 3
		}
		totalWidth += width
	}
	fmt.Println(strings.Repeat("-", totalWidth))

	for _, row := range result.Rows {
		for i, col := range result.Columns {
			if i > 0 {
				fmt.Print(" | ")
			}

			val := row[col]
			valStr := "NULL"
			if val != nil {
				valStr = fmt.Sprintf("%v", val)
				if len(valStr) > 50 {
					valStr = valStr[:47] + "..."
				}
			}

			fmt.Printf("%-*s", colWidths[i], valStr)
		}
		fmt.Println()
	}
}
