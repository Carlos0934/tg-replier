package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"tg-replier/internal/members"
)

// registerHandlers wires command handlers onto the bot client.
func (b *Bot) registerHandlers() {
	b.client.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, b.handleCommand)
	b.client.RegisterHandler(bot.HandlerTypeMessageText, "/group", bot.MatchTypePrefix, b.handleCommand)
	b.client.RegisterHandler(bot.HandlerTypeMessageText, "/reply", bot.MatchTypePrefix, b.handleCommand)
}

// defaultHandler tracks members passively on every incoming message and
// processes join events. Any message with a non-empty From.Username is
// recorded in the roster cache. New chat members from join events are
// also tracked.
func (b *Bot) defaultHandler(ctx context.Context, _ *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	// Track message sender.
	if update.Message.From != nil && update.Message.From.Username != "" {
		_ = b.tracker.Track(ctx, chatID, update.Message.From.Username)
	}

	// Track new chat members from join events.
	for _, u := range update.Message.NewChatMembers {
		if u.Username != "" {
			_ = b.tracker.Track(ctx, chatID, u.Username)
		}
	}
}

// handleCommand delegates every slash command to the commands.Router and
// sends the response text back via Telegram.
// It also passively tracks the sender before routing the command.
func (b *Bot) handleCommand(ctx context.Context, _ *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	// Passive member tracking for command senders too.
	if update.Message.From != nil && update.Message.From.Username != "" {
		_ = b.tracker.Track(ctx, update.Message.Chat.ID, update.Message.From.Username)
	}

	resp := b.router.Handle(ctx, update.Message.Chat.ID, update.Message.Text)

	params := &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   resp.Text,
	}
	if resp.ParseMode != "" {
		params.ParseMode = models.ParseMode(resp.ParseMode)
	}

	b.sender.SendMessage(ctx, params)
}

// noopTracker is a no-op implementation of members.Tracker for backward
// compatibility when no tracker is provided.
type noopTracker struct{}

func (noopTracker) Track(_ context.Context, _ int64, _ string) error  { return nil }
func (noopTracker) List(_ context.Context, _ int64) ([]string, error) { return nil, nil }

var _ members.Tracker = noopTracker{}
