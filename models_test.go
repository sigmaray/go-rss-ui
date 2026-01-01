package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Auto-migrate models
	err = db.AutoMigrate(&User{}, &Feed{}, &Item{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func TestUser_CheckPassword(t *testing.T) {
	tests := []struct {
		name           string
		plainPassword  string
		setupUser      func(*gorm.DB) *User
		expectedResult bool
		description    string
	}{
		{
			name:          "correct password",
			plainPassword: "password123",
			setupUser: func(db *gorm.DB) *User {
				user := &User{Username: "testuser", Password: "password123"}
				db.Create(user)
				return user
			},
			expectedResult: true,
			description:    "Should return true for correct password",
		},
		{
			name:          "incorrect password",
			plainPassword: "wrongpassword",
			setupUser: func(db *gorm.DB) *User {
				user := &User{Username: "testuser", Password: "password123"}
				db.Create(user)
				return user
			},
			expectedResult: false,
			description:    "Should return false for incorrect password",
		},
		{
			name:          "empty password",
			plainPassword: "",
			setupUser: func(db *gorm.DB) *User {
				user := &User{Username: "testuser", Password: "password123"}
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

			// Reload user from database to get the hashed password
			var reloadedUser User
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

			user := &User{
				Username: "testuser",
				Password: tt.initialPassword,
			}

			// BeforeSave is called automatically by GORM
			err := db.Create(user).Error
			assert.NoError(t, err, "Should create user without error")

			// Reload user from database
			var reloadedUser User
			db.First(&reloadedUser, user.ID)

			// Check if password is hashed (bcrypt hashes are 60 characters and start with $2a$, $2b$, or $2y$)
			isHashed := len(reloadedUser.Password) == 60 &&
				len(reloadedUser.Password) >= 4 &&
				reloadedUser.Password[0] == '$' &&
				reloadedUser.Password[1] == '2' &&
				(reloadedUser.Password[2] == 'a' || reloadedUser.Password[2] == 'b' || reloadedUser.Password[2] == 'y') &&
				reloadedUser.Password[3] == '$'

			if tt.expectedHashed {
				assert.True(t, isHashed, tt.description)
				// If it was already hashed, it should remain the same
				if len(tt.initialPassword) == 60 && tt.initialPassword[0] == '$' {
					assert.Equal(t, tt.initialPassword, reloadedUser.Password, "Already hashed password should remain unchanged")
				}
			} else {
				// For empty password, it should remain empty
				assert.Equal(t, "", reloadedUser.Password, tt.description)
			}
		})
	}
}

func TestUser_BeforeSave_UpdatePassword(t *testing.T) {
	db := setupTestDB(t)

	// Create user with initial password
	user := &User{Username: "testuser", Password: "oldpassword"}
	err := db.Create(user).Error
	assert.NoError(t, err)

	// Get the hashed password
	var initialUser User
	db.First(&initialUser, user.ID)
	initialHashedPassword := initialUser.Password

	// Update password
	user.Password = "newpassword"
	err = db.Save(user).Error
	assert.NoError(t, err)

	// Reload and check
	var updatedUser User
	db.First(&updatedUser, user.ID)

	// New password should be hashed and different from old
	assert.NotEqual(t, initialHashedPassword, updatedUser.Password, "New password should be different from old")
	assert.True(t, updatedUser.CheckPassword("newpassword"), "New password should be correct")
	assert.False(t, updatedUser.CheckPassword("oldpassword"), "Old password should not work")
}

func TestUser_BeforeSave_UpdateWithoutPassword(t *testing.T) {
	db := setupTestDB(t)

	// Create user with password
	user := &User{Username: "testuser", Password: "password123"}
	err := db.Create(user).Error
	assert.NoError(t, err)

	// Get the hashed password
	var initialUser User
	db.First(&initialUser, user.ID)
	initialHashedPassword := initialUser.Password

	// Update username using Select to only update username field, not password
	// This is the correct way to update without changing password
	err = db.Model(&user).Select("username").Updates(map[string]interface{}{"username": "newusername"}).Error
	assert.NoError(t, err)

	// Reload and check
	var updatedUser User
	db.First(&updatedUser, user.ID)

	// Username should be updated
	assert.Equal(t, "newusername", updatedUser.Username, "Username should be updated")

	// Password should remain unchanged
	assert.Equal(t, initialHashedPassword, updatedUser.Password, "Password should remain unchanged")
	assert.True(t, updatedUser.CheckPassword("password123"), "Original password should still work")
}
