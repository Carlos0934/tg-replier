package telegram

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"tg-replier/internal/commands"
	"tg-replier/internal/config"
	"tg-replier/internal/groups"
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

// meGetter abstracts the GetMe call so startup can be unit-tested
// without a live Telegram API.
type meGetter interface {
	GetMe(ctx context.Context) (*models.User, error)
}

// Bot wraps the Telegram client and delegates command handling to
// a commandHandler (typically commands.Router). It is a pure transport adapter.
type Bot struct {
	client      *bot.Bot
	router      commandHandler
	sender      messageSender // defaults to client; override in tests
	meGetter    meGetter      // defaults to client; override in tests
	botUsername string        // cached from GetMe at startup
}

// New creates a Bot, initialises the Telegram client and sender adapter,
// wires the commands router, and registers all command handlers.
// Call Start to begin polling.
func New(cfg *config.Config, groupsSvc *groups.Service) (*Bot, error) {
	b := &Bot{}

	opts := []bot.Option{
		bot.WithDefaultHandler(b.defaultHandler),
	}

	client, err := bot.New(cfg.BotToken, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating telegram bot: %w", err)
	}
	b.client = client
	b.sender = client
	b.meGetter = client

	b.router = commands.New(groupsSvc)
	b.registerHandlers()

	return b, nil
}

// Start resolves the bot username via GetMe and begins polling for updates.
// It blocks until ctx is cancelled.
func (b *Bot) Start(ctx context.Context) error {
	me, err := b.meGetter.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("resolving bot username: %w", err)
	}
	b.botUsername = me.Username
	b.client.Start(ctx)
	return nil
}
