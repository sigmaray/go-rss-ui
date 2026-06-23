package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go-rss-ui/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	err = db.AutoMigrate(&models.User{}, &models.Feed{}, &models.Item{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func TestUser_CheckPassword(t *testing.T) {
	tests := []struct {
		name           string
		plainPassword  string
		setupUser      func(*gorm.DB) *models.User
		expectedResult bool
		description    string
	}{
		{
			name:          "correct password",
			plainPassword: "password123",
			setupUser: func(db *gorm.DB) *models.User {
				user := &models.User{Username: "testuser", Password: "password123"}
				db.Create(user)
				return user
			},
			expectedResult: true,
			description:    "Should return true for correct password",
		},
		{
			name:          "incorrect password",
			plainPassword: "wrongpassword",
			setupUser: func(db *gorm.DB) *models.User {
				user := &models.User{Username: "testuser", Password: "password123"}
				db.Create(user)
				return user
			},
			expectedResult: false,
			description:    "Should return false for incorrect password",
		},
		{
			name:          "empty password",
			plainPassword: "",
			setupUser: func(db *gorm.DB) *models.User {
				user := &models.User{Username: "testuser", Password: "password123"}
				db.Create(user)
				return user
			},
			expectedResult: false,
			description:    "Should return false for empty password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			user := tt.setupUser(db)

			var reloadedUser models.User
			db.First(&reloadedUser, user.ID)

			result := reloadedUser.CheckPassword(tt.plainPassword)
			assert.Equal(t, tt.expectedResult, result, tt.description)
		})
	}
}

func TestUser_BeforeSave(t *testing.T) {
	tests := []struct {
		name            string
		initialPassword string
		expectedHashed  bool
		description     string
	}{
		{
			name:            "plain password gets hashed",
			initialPassword: "plainpassword",
			expectedHashed:  true,
			description:     "Plain password should be hashed before save",
		},
		{
			name:            "empty password not hashed",
			initialPassword: "",
			expectedHashed:  false,
			description:     "Empty password should not be hashed",
		},
		{
			name:            "already hashed password not double-hashed",
			initialPassword: "$2a$10$abcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOPQRST",
			expectedHashed:  true,
			description:     "Already hashed password should not be double-hashed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)

			user := &models.User{
				Username: "testuser",
				Password: tt.initialPassword,
			}

			err := db.Create(user).Error
			assert.NoError(t, err, "Should create user without error")

			var reloadedUser models.User
			db.First(&reloadedUser, user.ID)

			isHashed := len(reloadedUser.Password) == 60 &&
				len(reloadedUser.Password) >= 4 &&
				reloadedUser.Password[0] == '$' &&
				reloadedUser.Password[1] == '2' &&
				(reloadedUser.Password[2] == 'a' || reloadedUser.Password[2] == 'b' || reloadedUser.Password[2] == 'y') &&
				reloadedUser.Password[3] == '$'

			if tt.expectedHashed {
				assert.True(t, isHashed, tt.description)
				if len(tt.initialPassword) == 60 && tt.initialPassword[0] == '$' {
					assert.Equal(t, tt.initialPassword, reloadedUser.Password, "Already hashed password should remain unchanged")
				}
			} else {
				assert.Equal(t, "", reloadedUser.Password, tt.description)
			}
		})
	}
}

func TestUser_BeforeSave_UpdatePassword(t *testing.T) {
	db := setupTestDB(t)

	user := &models.User{Username: "testuser", Password: "oldpassword"}
	err := db.Create(user).Error
	assert.NoError(t, err)

	var initialUser models.User
	db.First(&initialUser, user.ID)
	initialHashedPassword := initialUser.Password

	user.Password = "newpassword"
	err = db.Save(user).Error
	assert.NoError(t, err)

	var updatedUser models.User
	db.First(&updatedUser, user.ID)

	assert.NotEqual(t, initialHashedPassword, updatedUser.Password, "New password should be different from old")
	assert.True(t, updatedUser.CheckPassword("newpassword"), "New password should be correct")
	assert.False(t, updatedUser.CheckPassword("oldpassword"), "Old password should not work")
}

func TestUser_BeforeSave_UpdateWithoutPassword(t *testing.T) {
	db := setupTestDB(t)

	user := &models.User{Username: "testuser", Password: "password123"}
	err := db.Create(user).Error
	assert.NoError(t, err)

	var initialUser models.User
	db.First(&initialUser, user.ID)
	initialHashedPassword := initialUser.Password

	err = db.Model(&user).Select("username").Updates(map[string]interface{}{"username": "newusername"}).Error
	assert.NoError(t, err)

	var updatedUser models.User
	db.First(&updatedUser, user.ID)

	assert.Equal(t, "newusername", updatedUser.Username, "Username should be updated")
	assert.Equal(t, initialHashedPassword, updatedUser.Password, "Password should remain unchanged")
	assert.True(t, updatedUser.CheckPassword("password123"), "Original password should still work")
}
