package services

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go-rss-ui/app"
)

func initTestLogger() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	app.Logger = &logger
}

func TestGetLogEntries(t *testing.T) {
	app.InitLogger()
	initTestLogger()

	if app.RedisClient == nil {
		t.Skip("Redis client is not available, skipping test")
	}

	app.RedisClient.Del(app.RedisCtx, "app:fetch-logs")

	AddLogEntry("success", "https://example.com/feed.xml", "Test message 1")
	AddLogEntry("error", "https://example.com/feed2.xml", "Test message 2")
	AddLogEntry("success", "https://example.com/feed3.xml", "Test message 3")

	entries := GetLogEntries()

	assert.Equal(t, 3, len(entries), "Should return 3 log entries")
	assert.Equal(t, "success", entries[0].Type, "First entry should be success (newest)")
	assert.Equal(t, "error", entries[1].Type, "Second entry should be error")
	assert.Equal(t, "success", entries[2].Type, "Third entry should be success (oldest)")

	entries[0].Type = "modified"
	entries2 := GetLogEntries()
	assert.Greater(t, len(entries2), 0, "Should have entries after modification")
	if len(entries2) > 0 {
		assert.Equal(t, "success", entries2[0].Type, "Original entries in Redis should not be modified")
	}

	app.RedisClient.Del(app.RedisCtx, "app:fetch-logs")
}

func TestAddLogEntry(t *testing.T) {
	app.InitLogger()
	initTestLogger()

	if app.RedisClient == nil {
		t.Skip("Redis client is not available, skipping test")
	}

	app.RedisClient.Del(app.RedisCtx, "app:fetch-logs")

	for i := 0; i < maxLogSize+10; i++ {
		AddLogEntry("success", "https://example.com/feed.xml", fmt.Sprintf("Test message %d", i))
	}

	entries := GetLogEntries()

	assert.Equal(t, maxLogSize, len(entries), "Should maintain max log size")
	assert.Equal(t, fmt.Sprintf("Test message %d", maxLogSize+9), entries[0].Message, "Newest entry should be first")
	assert.Equal(t, "Test message 10", entries[len(entries)-1].Message, "Oldest kept entry should be last")

	app.RedisClient.Del(app.RedisCtx, "app:fetch-logs")
}

func TestFeedFetchStats(t *testing.T) {
	app.InitLogger()
	initTestLogger()

	if app.RedisClient == nil {
		t.Skip("Redis client is not available, skipping test")
	}

	now := time.Now()
	hourSuffix := now.Format("2006-01-02-15")
	successKey := fmt.Sprintf("%s:%s", feedFetchSuccessStatsKey, hourSuffix)
	errorKey := fmt.Sprintf("%s:%s", feedFetchErrorStatsKey, hourSuffix)
	app.RedisClient.Del(app.RedisCtx, successKey, errorKey)

	IncrementFeedFetchStats(true)
	IncrementFeedFetchStats(true)
	IncrementFeedFetchStats(false)

	successStats, errorStats := GetFeedFetchStats()
	displayKey := now.Format("2006-01-02 15:00")

	assert.Equal(t, int64(2), successStats[displayKey])
	assert.Equal(t, int64(1), errorStats[displayKey])

	app.RedisClient.Del(app.RedisCtx, successKey, errorKey)
}

func TestItemsCreatedDailyStats(t *testing.T) {
	app.InitLogger()
	initTestLogger()

	if app.RedisClient == nil {
		t.Skip("Redis client is not available, skipping test")
	}

	now := time.Now()
	dayKey := fmt.Sprintf("%s:%s", itemsCreatedDailyStatsKey, now.Format("2006-01-02"))
	app.RedisClient.Del(app.RedisCtx, dayKey)

	IncrementItemsCreatedStats()
	IncrementItemsCreatedStats()
	IncrementItemsCreatedStats()

	stats := GetItemsCreatedDailyStats()
	displayKey := now.Format("2006-01-02")

	assert.Equal(t, int64(3), stats[displayKey])

	labels, data := GetItemsCreatedDailyChartData()
	assert.Equal(t, itemsCreatedDailyDays, len(labels))
	assert.Equal(t, itemsCreatedDailyDays, len(data))
	assert.Equal(t, displayKey, labels[len(labels)-1])
	assert.Equal(t, int64(3), data[len(data)-1])

	app.RedisClient.Del(app.RedisCtx, dayKey)
}
