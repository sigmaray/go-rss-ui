package commands_test

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go-rss-ui/app"
	"go-rss-ui/commands"
	"go-rss-ui/database"
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

func initTestLogger() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	app.Logger = &logger
}

func TestSeedFeedsWithURLs(t *testing.T) {
	initTestLogger()

	tests := []struct {
		name           string
		feedURLs       []string
		setup          func(*gorm.DB)
		expectedResult commands.SeedFeedsResult
		description    string
	}{
		{
			name:     "create new feeds",
			feedURLs: []string{"https://example.com/feed1.xml", "https://example.com/feed2.xml"},
			expectedResult: commands.SeedFeedsResult{
				Created: 2,
				Existed: 0,
				Errors:  0,
			},
			description: "Should create 2 new feeds",
		},
		{
			name:     "skip existing feeds",
			feedURLs: []string{"https://example.com/feed1.xml"},
			setup: func(db *gorm.DB) {
				db.Create(&models.Feed{URL: "https://example.com/feed1.xml"})
			},
			expectedResult: commands.SeedFeedsResult{
				Created: 0,
				Existed: 1,
				Errors:  0,
			},
			description: "Should skip existing feed",
		},
		{
			name:     "mixed new and existing",
			feedURLs: []string{"https://example.com/feed1.xml", "https://example.com/feed2.xml"},
			setup: func(db *gorm.DB) {
				db.Create(&models.Feed{URL: "https://example.com/feed1.xml"})
			},
			expectedResult: commands.SeedFeedsResult{
				Created: 1,
				Existed: 1,
				Errors:  0,
			},
			description: "Should create new feed and skip existing",
		},
		{
			name:     "empty URLs",
			feedURLs: []string{},
			expectedResult: commands.SeedFeedsResult{
				Created: 0,
				Existed: 0,
				Errors:  0,
			},
			description: "Should handle empty URL list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			database.DB = db

			if tt.setup != nil {
				tt.setup(db)
			}

			result := commands.SeedFeedsWithURLs(tt.feedURLs)

			assert.Equal(t, tt.expectedResult.Created, result.Created, "Created count should match")
			assert.Equal(t, tt.expectedResult.Existed, result.Existed, "Existed count should match")
			assert.Equal(t, tt.expectedResult.Errors, result.Errors, "Errors count should match")

			var feedCount int64
			db.Model(&models.Feed{}).Count(&feedCount)
			expectedCount := int64(tt.expectedResult.Created + tt.expectedResult.Existed)
			assert.Equal(t, expectedCount, feedCount, "Total feed count should match")
		})
	}
}

func TestSeedFeeds(t *testing.T) {
	initTestLogger()

	db := setupTestDB(t)
	database.DB = db

	result := commands.SeedFeeds()

	assert.Greater(t, result.Created+result.Existed, 0, "Should process at least some default feeds")

	defaultFeeds := commands.GetDefaultFeeds()
	assert.Greater(t, len(defaultFeeds), 0, "Should have default feeds")
}

func TestGetDefaultFeeds(t *testing.T) {
	feeds := commands.GetDefaultFeeds()

	assert.Greater(t, len(feeds), 0, "Should return at least one feed")

	for _, feed := range feeds {
		assert.True(t,
			len(feed) > 0 && (feed[:7] == "http://" || feed[:8] == "https://"),
			"Feed URL should be valid: %s", feed)
	}
}

func TestSeedUsers(t *testing.T) {
	initTestLogger()

	tests := []struct {
		name        string
		setup       func(*gorm.DB)
		description string
	}{
		{
			name: "create new admin user",
			setup: func(db *gorm.DB) {
			},
			description: "Should create admin user when it doesn't exist",
		},
		{
			name: "skip existing admin user",
			setup: func(db *gorm.DB) {
				db.Create(&models.User{Username: "admin", Password: "existingpassword"})
			},
			description: "Should skip creating admin user when it already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			database.DB = db

			if tt.setup != nil {
				tt.setup(db)
			}

			var countBefore int64
			db.Model(&models.User{}).Count(&countBefore)

			commands.SeedUsers()

			var countAfter int64
			db.Model(&models.User{}).Count(&countAfter)

			assert.GreaterOrEqual(t, countAfter, int64(1), "Should have at least one user")

			var adminUser models.User
			err := db.Where("username = ?", "admin").First(&adminUser).Error
			assert.NoError(t, err, "Admin user should exist")
			assert.Equal(t, "admin", adminUser.Username, "Username should be 'admin'")

			if countAfter > countBefore {
				assert.True(t, adminUser.CheckPassword("password"), "Password should be 'password' for newly created user")
			} else {
				assert.True(t, adminUser.CheckPassword("existingpassword"), "Password should remain as original for existing user")
			}
		})
	}
}
