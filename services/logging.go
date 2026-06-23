package services

import (
	"encoding/json"
	"fmt"
	"time"

	"go-rss-ui/app"
)

const (
	fetchLogsRedisKey    = "app:fetch-logs"
	itemsCreatedStatsKey = "app:items:created:stats"
	maxLogSize           = 1000
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // "success" or "error"
	FeedURL   string    `json:"feed_url"`
	Message   string    `json:"message"`
}

// AddLogEntry adds a log entry to Redis storage
// Maintains maximum of 1000 entries by removing oldest entries
func AddLogEntry(logType, feedURL, message string) {
	if app.RedisClient == nil {
		// If Redis is not available, skip logging
		return
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Type:      logType,
		FeedURL:   feedURL,
		Message:   message,
	}

	// Serialize to JSON
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		// If serialization fails, log error but don't block
		app.Logger.Error().Err(err).Msg("Failed to serialize log entry")
		return
	}

	// Use pipeline for atomicity
	pipe := app.RedisClient.Pipeline()
	pipe.LPush(app.RedisCtx, fetchLogsRedisKey, string(entryJSON))
	pipe.LTrim(app.RedisCtx, fetchLogsRedisKey, 0, maxLogSize-1)
	_, _ = pipe.Exec(app.RedisCtx) // Ignore Redis errors to not block execution
}

// GetLogEntries returns all log entries from Redis
func GetLogEntries() []LogEntry {
	var entries []LogEntry

	if app.RedisClient == nil {
		// If Redis is not available, return empty slice
		return entries
	}

	// Get logs from Redis (up to 1000 entries)
	logs, err := app.RedisClient.LRange(app.RedisCtx, fetchLogsRedisKey, 0, maxLogSize-1).Result()
	if err != nil {
		// If error occurs, log it but return empty slice
		app.Logger.Error().Err(err).Msg("Failed to get fetch logs from Redis")
		return entries
	}

	// Parse JSON logs
	// Redis LPUSH adds to the left, so LRANGE 0 999 returns newest first
	for _, logJSON := range logs {
		var entry LogEntry
		if err := json.Unmarshal([]byte(logJSON), &entry); err != nil {
			// If parsing fails, skip this entry
			app.Logger.Error().Err(err).Msg("Failed to parse fetch log entry")
			continue
		}
		entries = append(entries, entry)
	}

	return entries
}

// IncrementItemsCreatedStats increments the counter for items created in the current hour
func IncrementItemsCreatedStats() {
	if app.RedisClient == nil {
		return
	}

	// Get current hour in format: YYYY-MM-DD-HH
	now := time.Now()
	hourKey := fmt.Sprintf("%s:%s", itemsCreatedStatsKey, now.Format("2006-01-02-15"))

	// Increment counter for this hour
	app.RedisClient.Incr(app.RedisCtx, hourKey)

	// Set expiration to 48 hours (to keep data for 24 hours + buffer)
	app.RedisClient.Expire(app.RedisCtx, hourKey, 48*time.Hour)
}

// GetItemsCreatedStats returns statistics for items created in the last 24 hours
// Returns a map where key is hour (format: "YYYY-MM-DD HH:00") and value is count
func GetItemsCreatedStats() map[string]int64 {
	stats := make(map[string]int64)

	if app.RedisClient == nil {
		return stats
	}

	// Get keys for last 24 hours
	now := time.Now()
	for i := 0; i < 24; i++ {
		hourTime := now.Add(-time.Duration(i) * time.Hour)
		hourKey := fmt.Sprintf("%s:%s", itemsCreatedStatsKey, hourTime.Format("2006-01-02-15"))

		// Get count for this hour
		count, err := app.RedisClient.Get(app.RedisCtx, hourKey).Int64()
		if err == nil {
			// Format hour for display: "YYYY-MM-DD HH:00"
			displayKey := hourTime.Format("2006-01-02 15:00")
			stats[displayKey] = count
		}
	}

	return stats
}
