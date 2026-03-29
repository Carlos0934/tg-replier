package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"tg-replyer/internal/config"
	"tg-replyer/internal/groups"
	jsonstorage "tg-replyer/internal/storage/json"
	"tg-replyer/internal/telegram"
)

func main() {
	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// 2. Storage adapter (implements both groups.Repository and members.Tracker)
	store, err := jsonstorage.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	// 3. Domain services
	groupsSvc := groups.New(store)

	// 4. Telegram transport (tracker injected for passive member tracking)
	b, err := telegram.New(cfg, groupsSvc, store)
	if err != nil {
		log.Fatalf("bot: %v", err)
	}

	// 5. Graceful shutdown via OS signals
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Println("bot starting...")
	b.Start(ctx)
	log.Println("bot stopped")
}
