package telegram

import (
	"context"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// registerHandlers wires command handlers onto the bot client.
func (b *Bot) registerHandlers() {
	b.client.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, b.handleCommand)
	b.client.RegisterHandler(bot.HandlerTypeMessageText, "/group", bot.MatchTypePrefix, b.handleCommand)
	b.client.RegisterHandler(bot.HandlerTypeMessageText, "/reply", bot.MatchTypePrefix, b.handleCommand)
}

// defaultHandler is the default message handler. It processes any
// non-command messages that arrive.
func (b *Bot) defaultHandler(ctx context.Context, _ *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
}

// handleCommand delegates every slash command to the commands.Router and
// sends the response text back via Telegram.
//
// In group/supergroup chats, the command must include @botusername
// (e.g., /reply@mybot). Bare commands and commands addressed to other
// bots are silently ignored. In private chats, no suffix is required.
func (b *Bot) handleCommand(ctx context.Context, _ *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	text := update.Message.Text

	// In group/supergroup chats, enforce @botusername addressing.
	chatType := update.Message.Chat.Type
	if chatType == "group" || chatType == "supergroup" {
		text = b.normalizeGroupCommand(text)
		if text == "" {
			return // not addressed to us — silently ignore
		}
	}

	resp := b.router.Handle(ctx, update.Message.Chat.ID, text)

	params := &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   resp.Text,
	}
	if resp.ParseMode != "" {
		params.ParseMode = models.ParseMode(resp.ParseMode)
	}

	b.sender.SendMessage(ctx, params)
}

// normalizeGroupCommand checks that the command token in text contains
// @botusername. If so, it returns the text with the @botusername stripped
// from the command token. Otherwise, it returns "" to signal the message
// should be silently ignored.
func (b *Bot) normalizeGroupCommand(text string) string {
	if b.botUsername == "" {
		return "" // safety: no cached username means we can't verify addressing
	}

	// Extract the first whitespace-delimited token (the command token).
	cmdToken := text
	rest := ""
	if idx := strings.IndexByte(text, ' '); idx >= 0 {
		cmdToken = text[:idx]
		rest = text[idx:] // preserves leading space + rest of message
	}

	// Check for @suffix in the command token (e.g., "/reply@mybot").
	atIdx := strings.IndexByte(cmdToken, '@')
	if atIdx < 0 {
		return "" // bare command in group — ignore
	}

	targetBot := cmdToken[atIdx+1:]
	if !strings.EqualFold(targetBot, b.botUsername) {
		return "" // addressed to a different bot — ignore
	}

	// Strip @botusername from the command token.
	normalized := cmdToken[:atIdx] + rest
	return normalized
}
