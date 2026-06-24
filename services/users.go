package services

import (
	"errors"
	"fmt"
	"strings"

	"go-rss-ui/database"
	"go-rss-ui/models"
	"go-rss-ui/validation"
	"gorm.io/gorm"
)

type EnsureAdminUserResult struct {
	User    models.User
	Created bool
}

func ClearUsers() (int64, error) {
	ensurePrimaryDatabase()

	result := database.DB.Exec("DELETE FROM users")
	return result.RowsAffected, result.Error
}

func CreateUser(username, password string) (models.User, error) {
	ensurePrimaryDatabase()

	input := validation.UserInput{
		Username: strings.TrimSpace(username),
		Password: password,
	}
	if err := validation.ValidateStruct(input); err != nil {
		return models.User{}, fmt.Errorf("%s", validation.FormatValidationErrors(err))
	}

	var existingUser models.User
	if err := database.DB.Where("username = ?", input.Username).First(&existingUser).Error; err == nil {
		return models.User{}, errors.New("username already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.User{}, err
	}

	user := models.User{Username: input.Username, Password: input.Password}
	if err := database.DB.Create(&user).Error; err != nil {
		if isUniqueConstraintError(err) {
			return models.User{}, errors.New("username already exists")
		}
		return models.User{}, err
	}

	return user, nil
}

func EnsureAdminUser() (EnsureAdminUserResult, error) {
	ensurePrimaryDatabase()

	var user models.User
	result := database.DB.Where("username = ?", "admin").First(&user)

	switch {
	case errors.Is(result.Error, gorm.ErrRecordNotFound):
		adminUser := models.User{Username: "admin", Password: "password"}
		if err := database.DB.Create(&adminUser).Error; err != nil {
			return EnsureAdminUserResult{}, err
		}

		return EnsureAdminUserResult{
			User:    adminUser,
			Created: true,
		}, nil
	case result.Error != nil:
		return EnsureAdminUserResult{}, result.Error
	default:
		return EnsureAdminUserResult{
			User: user,
		}, nil
	}
}
