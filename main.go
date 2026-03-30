package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"tg-replier/internal/config"
	"tg-replier/internal/groups"
	jsonstorage "tg-replier/internal/storage/json"
	"tg-replier/internal/telegram"
)

var version = "dev"

func main() {
	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// 2. Storage adapter (implements groups.Repository)
	store, err := jsonstorage.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	// 3. Domain services
	groupsSvc := groups.New(store)

	// 4. Telegram transport
	b, err := telegram.New(cfg, groupsSvc, version)
	if err != nil {
		log.Fatalf("bot: %v", err)
	}

	// 5. Graceful shutdown via OS signals
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Println("bot starting...")
	if err := b.Start(ctx); err != nil {
		log.Fatalf("bot start: %v", err)
	}
	log.Println("bot stopped")
}
