package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestSeedFeedsWithURLs(t *testing.T) {
	tests := []struct {
		name           string
		feedURLs       []string
		setup          func(*gorm.DB)
		expectedResult SeedFeedsResult
		description    string
	}{
		{
			name:     "create new feeds",
			feedURLs: []string{"https://example.com/feed1.xml", "https://example.com/feed2.xml"},
			expectedResult: SeedFeedsResult{
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
				// Pre-create a feed
				db.Create(&Feed{URL: "https://example.com/feed1.xml"})
			},
			expectedResult: SeedFeedsResult{
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
				// Pre-create one feed
				db.Create(&Feed{URL: "https://example.com/feed1.xml"})
			},
			expectedResult: SeedFeedsResult{
				Created: 1,
				Existed: 1,
				Errors:  0,
			},
			description: "Should create new feed and skip existing",
		},
		{
			name:     "empty URLs",
			feedURLs: []string{},
			expectedResult: SeedFeedsResult{
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
			DB = db

			if tt.setup != nil {
				tt.setup(db)
			}

			result := SeedFeedsWithURLs(tt.feedURLs)

			assert.Equal(t, tt.expectedResult.Created, result.Created, "Created count should match")
			assert.Equal(t, tt.expectedResult.Existed, result.Existed, "Existed count should match")
			assert.Equal(t, tt.expectedResult.Errors, result.Errors, "Errors count should match")

			// Verify feeds were actually created
			var feedCount int64
			db.Model(&Feed{}).Count(&feedCount)
			expectedCount := int64(tt.expectedResult.Created + tt.expectedResult.Existed)
			assert.Equal(t, expectedCount, feedCount, "Total feed count should match")
		})
	}
}

func TestSeedFeeds(t *testing.T) {
	db := setupTestDB(t)
	DB = db

	// Test that SeedFeeds uses GetDefaultFeeds
	result := SeedFeeds()

	// Should create some feeds (at least some from default list)
	assert.Greater(t, result.Created+result.Existed, 0, "Should process at least some default feeds")

	// Verify default feeds
	defaultFeeds := GetDefaultFeeds()
	assert.Greater(t, len(defaultFeeds), 0, "Should have default feeds")
}

func TestGetDefaultFeeds(t *testing.T) {
	feeds := GetDefaultFeeds()

	assert.Greater(t, len(feeds), 0, "Should return at least one feed")
	
	// Check that all feeds are valid URLs (at least start with http:// or https://)
	for _, feed := range feeds {
		assert.True(t, 
			len(feed) > 0 && (feed[:7] == "http://" || feed[:8] == "https://"),
			"Feed URL should be valid: %s", feed)
	}
}

func TestSeedUsers(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*gorm.DB)
		description string
	}{
		{
			name: "create new admin user",
			setup: func(db *gorm.DB) {
				// No existing user
			},
			description: "Should create admin user when it doesn't exist",
		},
		{
			name: "skip existing admin user",
			setup: func(db *gorm.DB) {
				// Pre-create admin user
				db.Create(&User{Username: "admin", Password: "existingpassword"})
			},
			description: "Should skip creating admin user when it already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			DB = db

			if tt.setup != nil {
				tt.setup(db)
			}

			// Count users before
			var countBefore int64
			db.Model(&User{}).Count(&countBefore)

			// Seed users
			SeedUsers()

			// Count users after
			var countAfter int64
			db.Model(&User{}).Count(&countAfter)

			// Should have at least one user (admin)
			assert.GreaterOrEqual(t, countAfter, int64(1), "Should have at least one user")

			// Verify admin user exists
			var adminUser User
			err := db.Where("username = ?", "admin").First(&adminUser).Error
			assert.NoError(t, err, "Admin user should exist")
			assert.Equal(t, "admin", adminUser.Username, "Username should be 'admin'")
			
			// If user was just created, password should be 'password'
			// If user already existed, it should have the original password
			if countAfter > countBefore {
				assert.True(t, adminUser.CheckPassword("password"), "Password should be 'password' for newly created user")
			} else {
				// User already existed, check original password
				assert.True(t, adminUser.CheckPassword("existingpassword"), "Password should remain as original for existing user")
			}
		})
	}
}
