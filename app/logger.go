package app

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"go-rss-ui/config"
)

// Global logger and Redis client
var (
	Logger      *zerolog.Logger
	RedisClient *redis.Client
	RedisCtx    context.Context
)

const redisLogsKey = "app:logs"

// RedisLogWriter implements io.Writer to write logs to Redis
type RedisLogWriter struct {
	client *redis.Client
	ctx    context.Context
	key    string
	maxLen int64
}

func (w *RedisLogWriter) Write(p []byte) (n int, err error) {
	// Parse JSON to check log level
	var logData map[string]interface{}
	if err := json.Unmarshal(p, &logData); err != nil {
		return len(p), nil // Ignore parsing errors
	}

	// Filter only info and error levels
	level, ok := logData["level"].(string)
	if !ok {
		return len(p), nil
	}

	if level == "info" || level == "error" {
		// Use pipeline for atomicity
		pipe := w.client.Pipeline()
		pipe.LPush(w.ctx, w.key, string(p))
		pipe.LTrim(w.ctx, w.key, 0, w.maxLen-1)
		_, _ = pipe.Exec(w.ctx) // Ignore Redis errors to not block logging
	}

	return len(p), nil
}

// InitLogger initializes zerolog with Redis writer
func InitLogger() {
	RedisCtx = context.Background()

	// Create Redis client
	redisAddr := config.GetRedisAddr()
	password := config.GetRedisPassword()

	opts := &redis.Options{
		Addr: redisAddr,
	}
	if password != "" {
		opts.Password = password
	}

	RedisClient = redis.NewClient(opts)

	// Test Redis connection
	_, err := RedisClient.Ping(RedisCtx).Result()
	if err != nil {
		RedisClient = nil
		// If Redis is not available, log to stdout only
		tempLogger := zerolog.New(os.Stdout).With().
			Timestamp().
			Str("service", "go-rss-ui").
			Logger()
		tempLogger.Warn().Err(err).Msg("Failed to connect to Redis, logging to stdout only")
		Logger = &tempLogger
		return
	}

	// Create Redis writer
	redisWriter := &RedisLogWriter{
		client: RedisClient,
		ctx:    RedisCtx,
		key:    redisLogsKey,
		maxLen: 1000,
	}

	// MultiWriter: log to both stdout and Redis
	multi := io.MultiWriter(
		zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339},
		redisWriter,
	)

	logger := zerolog.New(multi).With().
		Timestamp().
		Str("service", "go-rss-ui").
		Logger()

	Logger = &logger
	Logger.Info().Msg("Logger initialized with Redis support")
}
