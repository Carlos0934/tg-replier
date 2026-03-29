package config

import (
	"errors"
	"os"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	BotToken string
	DataDir  string
}

// Load reads configuration from the environment and validates required fields.
// BOT_TOKEN is required; DATA_DIR defaults to "data" if not set.
func Load() (*Config, error) {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return nil, errors.New("BOT_TOKEN environment variable is required but not set")
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}

	return &Config{
		BotToken: token,
		DataDir:  dataDir,
	}, nil
}
