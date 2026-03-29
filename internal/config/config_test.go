package config_test

import (
	"strings"
	"testing"

	"tg-replyer/internal/config"
)

func TestLoad_TokenPresent(t *testing.T) {
	t.Setenv("BOT_TOKEN", "test-token-123")
	t.Setenv("DATA_DIR", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.BotToken != "test-token-123" {
		t.Errorf("expected BotToken %q, got %q", "test-token-123", cfg.BotToken)
	}
}

func TestLoad_TokenMissing(t *testing.T) {
	t.Setenv("BOT_TOKEN", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing BOT_TOKEN, got nil")
	}
	// Spec requires the error message to contain "BOT_TOKEN"
	if !strings.Contains(err.Error(), "BOT_TOKEN") {
		t.Errorf("error message should contain %q, got: %v", "BOT_TOKEN", err)
	}
}

func TestLoad_DataDirDefault(t *testing.T) {
	t.Setenv("BOT_TOKEN", "test-token")
	t.Setenv("DATA_DIR", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.DataDir != "data" {
		t.Errorf("expected DataDir %q, got %q", "data", cfg.DataDir)
	}
}

func TestLoad_DataDirCustom(t *testing.T) {
	t.Setenv("BOT_TOKEN", "test-token")
	t.Setenv("DATA_DIR", "custom-dir")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.DataDir != "custom-dir" {
		t.Errorf("expected DataDir %q, got %q", "custom-dir", cfg.DataDir)
	}
}
