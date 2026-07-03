package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go-rss-ui/app"
	"go-rss-ui/services"
)

// ZerologEntry represents a log entry from Redis
type ZerologEntry struct {
	Level   string                 `json:"level"`
	Time    string                 `json:"time"`
	Service string                 `json:"service,omitempty"`
	Message string                 `json:"message"`
	FeedURL string                 `json:"feed_url,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Extra   map[string]interface{} `json:"-"`
	RawJSON string                 `json:"-"`
}

func ShowLogs(c *gin.Context) {
	entries := services.GetLogEntries()
	// Redis LPUSH adds to the left, so LRANGE 0 999 returns newest first
	// No need to reverse - newest logs are already at the beginning

	data := getTemplateData(c, gin.H{
		"title":   "Feed Fetching Log",
		"entries": entries,
	})
	c.HTML(http.StatusOK, "logs.html", data)
}

func ShowZerolog(c *gin.Context) {
	var entries []ZerologEntry

	// Get filter level from query parameter, default to "all"
	filterLevel := c.DefaultQuery("level", "all")

	if app.RedisClient == nil {
		data := getTemplateData(c, gin.H{
			"title":       "Zerolog Logs",
			"entries":     entries,
			"filterLevel": filterLevel,
			"error":       "Redis client is not available",
		})
		c.HTML(http.StatusOK, "zerolog.html", data)
		return
	}

	// Get logs from Redis (up to 1000 entries)
	logs, err := app.RedisClient.LRange(app.RedisCtx, "app:logs", 0, 999).Result()
	if err != nil {
		app.Logger.Error().Err(err).Msg("Failed to get logs from Redis")
		data := getTemplateData(c, gin.H{
			"title":       "Zerolog Logs",
			"entries":     entries,
			"filterLevel": filterLevel,
			"error":       fmt.Sprintf("Failed to get logs from Redis: %v", err),
		})
		c.HTML(http.StatusOK, "zerolog.html", data)
		return
	}

	// Parse JSON logs
	// Redis LPUSH adds to the left, so LRANGE 0 999 returns newest first
	// No need to reverse - newest logs are already at the beginning
	for _, logJSON := range logs {
		var entry ZerologEntry
		if err := json.Unmarshal([]byte(logJSON), &entry); err != nil {
			// If parsing fails, create a basic entry
			entry = ZerologEntry{
				Level:   "unknown",
				Message: "Failed to parse log entry",
				RawJSON: logJSON,
			}
		} else {
			entry.RawJSON = logJSON
		}

		// Filter by level if not "all"
		if filterLevel == "all" || strings.EqualFold(entry.Level, filterLevel) {
			entries = append(entries, entry)
		}
	}

	data := getTemplateData(c, gin.H{
		"title":       "Zerolog Logs",
		"entries":     entries,
		"filterLevel": filterLevel,
	})
	c.HTML(http.StatusOK, "zerolog.html", data)
}

func ShowChart(c *gin.Context) {
	itemsStats := services.GetItemsCreatedStats()
	successStats, errorStats := services.GetFeedFetchStats()

	now := time.Now()
	labels := make([]string, 24)
	itemsData := make([]int64, 24)
	feedSuccessData := make([]int64, 24)
	feedErrorData := make([]int64, 24)

	for i := 0; i < 24; i++ {
		hourTime := now.Add(-time.Duration(23-i) * time.Hour)
		hourLabel := hourTime.Format("2006-01-02 15:00")
		labels[i] = hourLabel

		if count, ok := itemsStats[hourLabel]; ok {
			itemsData[i] = count
		}
		if count, ok := successStats[hourLabel]; ok {
			feedSuccessData[i] = count
		}
		if count, ok := errorStats[hourLabel]; ok {
			feedErrorData[i] = count
		}
	}

	pageData := getTemplateData(c, gin.H{
		"title": "Charts",
		"chartData": gin.H{
			"labels": labels,
			"data":   itemsData,
		},
		"feedChartData": gin.H{
			"labels":  labels,
			"success": feedSuccessData,
			"error":   feedErrorData,
		},
	})

	c.HTML(http.StatusOK, "chart.html", pageData)
}
