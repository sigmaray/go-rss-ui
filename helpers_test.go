package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsUniqueConstraintError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expected    bool
		description string
	}{
		{
			name:        "nil error",
			err:         nil,
			expected:    false,
			description: "Should return false for nil error",
		},
		{
			name:        "duplicate key error",
			err:         errors.New("duplicate key value violates unique constraint"),
			expected:    true,
			description: "Should return true for duplicate key error",
		},
		{
			name:        "unique constraint error",
			err:         errors.New("UNIQUE constraint failed"),
			expected:    true,
			description: "Should return true for unique constraint error",
		},
		{
			name:        "PostgreSQL error code 23505",
			err:         errors.New("error code 23505: unique_violation"),
			expected:    true,
			description: "Should return true for PostgreSQL error code 23505",
		},
		{
			name:        "case insensitive matching",
			err:         errors.New("DUPLICATE KEY VALUE VIOLATES UNIQUE CONSTRAINT"),
			expected:    true,
			description: "Should match case-insensitively",
		},
		{
			name:        "other error",
			err:         errors.New("some other error"),
			expected:    false,
			description: "Should return false for other errors",
		},
		{
			name:        "connection error",
			err:         errors.New("connection refused"),
			expected:    false,
			description: "Should return false for connection errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUniqueConstraintError(tt.err)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

func TestGeneratePageNumbers(t *testing.T) {
	tests := []struct {
		name         string
		currentPage  int64
		totalPages   int64
		expectedLen  int
		description  string
		validateFunc func([]interface{}) bool
	}{
		{
			name:        "single page",
			currentPage: 1,
			totalPages:  1,
			expectedLen: 1,
			description: "Should return single page for totalPages=1",
			validateFunc: func(pages []interface{}) bool {
				return len(pages) == 1 && pages[0].(int64) == 1
			},
		},
		{
			name:        "few pages (all shown)",
			currentPage: 2,
			totalPages:  5,
			expectedLen: 5,
			description: "Should show all pages when totalPages <= 7",
			validateFunc: func(pages []interface{}) bool {
				return len(pages) == 5
			},
		},
		{
			name:        "exactly 7 pages",
			currentPage: 4,
			totalPages:  7,
			expectedLen: 7,
			description: "Should show all 7 pages",
			validateFunc: func(pages []interface{}) bool {
				return len(pages) == 7
			},
		},
		{
			name:        "many pages - current at start",
			currentPage: 1,
			totalPages:  20,
			expectedLen: 0, // Variable length, just check it's reasonable
			description: "Should show first pages with ellipsis",
			validateFunc: func(pages []interface{}) bool {
				// Should start with 1 and end with 20
				firstPage := pages[0].(int64)
				lastPage := pages[len(pages)-1].(int64)
				return firstPage == 1 && lastPage == 20 && len(pages) >= 5
			},
		},
		{
			name:        "many pages - current in middle",
			currentPage: 10,
			totalPages:  20,
			expectedLen: 0, // Variable length
			description: "Should show pages around current with ellipsis",
			validateFunc: func(pages []interface{}) bool {
				// Should have current page and reasonable structure
				hasCurrent := false
				for _, p := range pages {
					if p == int64(10) {
						hasCurrent = true
					}
				}
				return hasCurrent && len(pages) >= 5
			},
		},
		{
			name:        "many pages - current at end",
			currentPage: 20,
			totalPages:  20,
			expectedLen: 0, // Variable length
			description: "Should show last pages with ellipsis",
			validateFunc: func(pages []interface{}) bool {
				// Should end with 20
				lastPage := pages[len(pages)-1].(int64)
				return lastPage == 20 && len(pages) >= 5
			},
		},
		{
			name:        "zero total pages",
			currentPage: 1,
			totalPages:  0,
			expectedLen: 0,
			description: "Should return empty slice for zero total pages",
			validateFunc: func(pages []interface{}) bool {
				return len(pages) == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generatePageNumbers(tt.currentPage, tt.totalPages)
			
			if tt.expectedLen > 0 {
				assert.Equal(t, tt.expectedLen, len(result), tt.description)
			} else if tt.totalPages > 0 {
				// For variable length cases, just check it's reasonable
				assert.Greater(t, len(result), 0, "Should return at least one page")
			}
			
			if tt.validateFunc != nil {
				assert.True(t, tt.validateFunc(result), "Custom validation failed for: "+tt.description)
			}
			
			// Additional validation: check that all page numbers are valid
			for i, page := range result {
				if pageNum, ok := page.(int64); ok {
					if pageNum != -1 { // -1 is ellipsis marker
						assert.GreaterOrEqual(t, pageNum, int64(1), "Page number should be >= 1 at index %d", i)
						assert.LessOrEqual(t, pageNum, tt.totalPages, "Page number should be <= totalPages at index %d", i)
					}
				}
			}
		})
	}
}

func TestGetLogEntries(t *testing.T) {
	// Clear log entries before test
	logMutex.Lock()
	logEntries = []LogEntry{}
	logMutex.Unlock()

	// Add some test entries
	addLogEntry("success", "https://example.com/feed.xml", "Test message 1")
	addLogEntry("error", "https://example.com/feed2.xml", "Test message 2")
	addLogEntry("success", "https://example.com/feed3.xml", "Test message 3")

	entries := getLogEntries()

	assert.Equal(t, 3, len(entries), "Should return 3 log entries")
	assert.Equal(t, "success", entries[0].Type, "First entry should be success")
	assert.Equal(t, "error", entries[1].Type, "Second entry should be error")
	assert.Equal(t, "success", entries[2].Type, "Third entry should be success")

	// Verify entries are copies (modifying shouldn't affect original)
	entries[0].Type = "modified"
	entries2 := getLogEntries()
	assert.Equal(t, "success", entries2[0].Type, "Original entries should not be modified")
}

func TestAddLogEntry(t *testing.T) {
	// Clear log entries before test
	logMutex.Lock()
	logEntries = []LogEntry{}
	logMutex.Unlock()

	// Add entries with different messages to track which ones are removed
	for i := 0; i < maxLogSize+10; i++ {
		addLogEntry("success", "https://example.com/feed.xml", fmt.Sprintf("Test message %d", i))
	}

	entries := getLogEntries()

	// Should only keep maxLogSize entries
	assert.Equal(t, maxLogSize, len(entries), "Should maintain max log size")
	
	// Oldest entries (0-9) should be removed, newest entries (10-1009) should remain
	// First entry should be "Test message 10" (the 11th entry added)
	assert.Equal(t, "Test message 10", entries[0].Message, "Oldest entries should be removed, keeping newest")
	// Last entry should be the last one added
	assert.Equal(t, fmt.Sprintf("Test message %d", maxLogSize+9), entries[len(entries)-1].Message, "Newest entries should be kept")
}
