package telegram

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"tg-replyer/internal/commands"
	"tg-replyer/internal/config"
	"tg-replyer/internal/groups"
	"tg-replyer/internal/members"
)

// commandHandler abstracts slash-command routing so the transport layer
// can be tested with a stub that records calls.
type commandHandler interface {
	Handle(ctx context.Context, chatID int64, text string) commands.Response
}

// messageSender abstracts the send-message call so handleCommand can be
// tested without a real Telegram client.
type messageSender interface {
	SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
}

// Bot wraps the Telegram client and delegates command handling to
// a commandHandler (typically commands.Router). It is a pure transport adapter.
type Bot struct {
	client  *bot.Bot
	router  commandHandler
	sender  messageSender // defaults to client; override in tests
	tracker members.Tracker
}

// New creates a Bot, initialises the Telegram client and sender adapter,
// wires the commands router, and registers all command handlers.
// Call Start to begin polling.
func New(cfg *config.Config, groupsSvc *groups.Service, tracker members.Tracker) (*Bot, error) {
	b := &Bot{tracker: tracker}

	opts := []bot.Option{
		bot.WithDefaultHandler(b.defaultHandler),
	}

	client, err := bot.New(cfg.BotToken, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating telegram bot: %w", err)
	}
	b.client = client
	b.sender = client

	b.router = commands.New(groupsSvc, tracker)
	b.registerHandlers()

	return b, nil
}

// Start begins polling for updates. It blocks until ctx is cancelled.
func (b *Bot) Start(ctx context.Context) {
	b.client.Start(ctx)
}
