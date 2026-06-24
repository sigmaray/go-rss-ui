package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"go-rss-ui/app"
	"go-rss-ui/services"
	"go-rss-ui/validation"
	"golang.org/x/term"
)

func CommandClearUsers() {
	rowsAffected, err := services.ClearUsers()
	if err != nil {
		app.Logger.Fatal().Err(err).Msg("Failed to clear users table")
	}

	app.Logger.Info().Int64("rows_affected", rowsAffected).Msg("Successfully cleared users table")
}

func CommandSeedUsers() {
	SeedUsers()
}

func CommandCreateUser() {
	input := validation.UserInputCreate{
		Username:        readLine("Login: "),
		Password:        readPassword("Password: "),
		PasswordConfirm: readPassword("Confirm password: "),
	}

	if err := validation.ValidateStruct(input); err != nil {
		app.Logger.Fatal().Msg(validation.FormatValidationErrors(err))
	}

	user, err := services.CreateUser(input.Username, input.Password)
	if err != nil {
		app.Logger.Fatal().Err(err).Msg("Failed to create user")
	}

	app.Logger.Info().Str("username", user.Username).Msg("User created successfully")
}

func readLine(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			app.Logger.Fatal().Err(err).Msg("Error reading input")
		}
		app.Logger.Fatal().Msg("No input provided")
	}
	return strings.TrimSpace(scanner.Text())
}

func readPassword(prompt string) string {
	fmt.Print(prompt)
	password, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		app.Logger.Fatal().Err(err).Msg("Error reading password")
	}
	return string(password)
}

func SeedUsers() {
	result, err := services.EnsureAdminUser()
	if err != nil {
		app.Logger.Fatal().Err(err).Msg("Failed to ensure admin user")
	}

	if result.Created {
		app.Logger.Info().Msg("Admin user 'admin' created with password 'password'")
		return
	}

	app.Logger.Info().Msg("Admin user already exists")
}
