package services

import (
	"errors"

	"go-rss-ui/database"
	"go-rss-ui/models"
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
