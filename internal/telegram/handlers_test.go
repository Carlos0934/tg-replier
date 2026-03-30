package telegram

import (
	"context"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"tg-replier/internal/commands"
)

// TestHandleCommand_NilMessage verifies the nil-message guard doesn't panic.
func TestHandleCommand_NilMessage(t *testing.T) {
	b := &Bot{}
	update := &models.Update{} // Message is nil
	// Must not panic even with nil router, because the nil-message guard returns early.
	b.handleCommand(t.Context(), nil, update)
}

// TestDefaultHandler_NilMessage verifies the default handler doesn't panic.
func TestDefaultHandler_NilMessage(t *testing.T) {
	b := &Bot{}
	update := &models.Update{}
	b.defaultHandler(t.Context(), nil, update)
}

// --- spy router for delegation tests ---

// spyRouter records calls to Handle and returns a canned response.
type spyRouter struct {
	called     bool
	lastText   string
	lastChatID int64
	response   commands.Response
}

func (s *spyRouter) Handle(_ context.Context, chatID int64, text string) commands.Response {
	s.called = true
	s.lastChatID = chatID
	s.lastText = text
	return s.response
}

// --- spy sender for send-message tests ---

type spySender struct {
	called bool
	params *bot.SendMessageParams
}

func (s *spySender) SendMessage(_ context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	s.called = true
	s.params = params
	return &models.Message{}, nil
}

// TestHandleCommand_DelegatesToRouter proves that when a valid Telegram
// update arrives, handleCommand forwards the message text and chatID to the
// commandHandler (Router) and does NOT implement business logic itself.
func TestHandleCommand_DelegatesToRouter(t *testing.T) {
	spy := &spyRouter{
		response: commands.Response{Text: "routed ok"},
	}
	ss := &spySender{}

	b := &Bot{router: spy, sender: ss}

	update := &models.Update{
		Message: &models.Message{
			Text: "/group set team @alice",
			Chat: models.Chat{ID: 42},
			From: &models.User{Username: "commander"},
		},
	}

	b.handleCommand(t.Context(), nil, update)

	if !spy.called {
		t.Fatal("handleCommand did not delegate to the router")
	}
	if spy.lastText != "/group set team @alice" {
		t.Errorf("router received %q, want %q", spy.lastText, "/group set team @alice")
	}
	if spy.lastChatID != 42 {
		t.Errorf("router received chatID %d, want 42", spy.lastChatID)
	}
	if !ss.called {
		t.Fatal("sender was not called")
	}
	if ss.params.Text != "routed ok" {
		t.Errorf("sender received text %q, want %q", ss.params.Text, "routed ok")
	}
}

// TestHandleCommand_RouterReceivesExactText ensures the transport layer
// passes the raw message text and chatID through without modification.
func TestHandleCommand_RouterReceivesExactText(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		chatID int64
	}{
		{"start command", "/start", 1},
		{"group with args", "/group delete mygroup", 2},
		{"reply with message", "/reply team hello world", 3},
		{"unknown command", "/foobar baz", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := &spyRouter{
				response: commands.Response{Text: "ok"},
			}
			ss := &spySender{}
			b := &Bot{router: spy, sender: ss}

			update := &models.Update{
				Message: &models.Message{
					Text: tt.text,
					Chat: models.Chat{ID: tt.chatID},
					From: &models.User{Username: "tester"},
				},
			}

			b.handleCommand(t.Context(), nil, update)

			if !spy.called {
				t.Fatal("router was not called")
			}
			if spy.lastText != tt.text {
				t.Errorf("router received %q, want %q", spy.lastText, tt.text)
			}
			if spy.lastChatID != tt.chatID {
				t.Errorf("router received chatID %d, want %d", spy.lastChatID, tt.chatID)
			}
		})
	}
}

// TestHandleCommand_ForwardsParseMode proves parse mode forwarding.
func TestHandleCommand_ForwardsParseMode(t *testing.T) {
	tests := []struct {
		name          string
		parseMode     string
		wantParseMode models.ParseMode
	}{
		{"HTML parse mode forwarded", "HTML", models.ParseModeHTML},
		{"MarkdownV2 parse mode forwarded", "MarkdownV2", models.ParseModeMarkdown},
		{"empty parse mode omitted", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := &spyRouter{
				response: commands.Response{
					Text:      "formatted output",
					ParseMode: tt.parseMode,
				},
			}
			ss := &spySender{}
			b := &Bot{router: spy, sender: ss}

			update := &models.Update{
				Message: &models.Message{
					Text: "/group list",
					Chat: models.Chat{ID: 99},
					From: &models.User{Username: "tester"},
				},
			}

			b.handleCommand(t.Context(), nil, update)

			if !ss.called {
				t.Fatal("sender was not called")
			}
			if ss.params.ParseMode != tt.wantParseMode {
				t.Errorf("ParseMode = %q, want %q", ss.params.ParseMode, tt.wantParseMode)
			}
			if ss.params.Text != "formatted output" {
				t.Errorf("Text = %q, want %q", ss.params.Text, "formatted output")
			}
		})
	}
}

// TestHandleCommand_NoDMSendCalls verifies that no DM send calls happen.
// The sender should only be called once (the reply in the same chat).
func TestHandleCommand_NoDMSendCalls(t *testing.T) {
	spy := &spyRouter{response: commands.Response{Text: "mentioned"}}
	ss := &spySender{}
	b := &Bot{router: spy, sender: ss}

	update := &models.Update{
		Message: &models.Message{
			Text: "/reply all hello",
			Chat: models.Chat{ID: 789},
			From: &models.User{Username: "sender"},
		},
	}

	b.handleCommand(t.Context(), nil, update)

	// Exactly 1 SendMessage call — the reply in the originating chat.
	if !ss.called {
		t.Fatal("sender was not called at all")
	}
	// ChatID must be the originating chat, not a user DM.
	if ss.params.ChatID != int64(789) {
		t.Errorf("SendMessage ChatID = %v, want 789 (originating chat)", ss.params.ChatID)
	}
}

// --- Group command addressing tests ---

// TestHandleCommand_BareCommandInGroup_Ignored verifies that a bare command
// (without @botusername) in a group chat is silently ignored.
func TestHandleCommand_BareCommandInGroup_Ignored(t *testing.T) {
	spy := &spyRouter{response: commands.Response{Text: "should not see this"}}
	ss := &spySender{}
	b := &Bot{router: spy, sender: ss, botUsername: "mybot"}

	update := &models.Update{
		Message: &models.Message{
			Text: "/reply all hello",
			Chat: models.Chat{ID: 100, Type: "group"},
			From: &models.User{Username: "alice"},
		},
	}

	b.handleCommand(t.Context(), nil, update)

	if spy.called {
		t.Error("router should NOT be called for bare command in group")
	}
	if ss.called {
		t.Error("sender should NOT be called for bare command in group")
	}
}

// TestHandleCommand_AddressedCommandInGroup_Routed verifies that
// /reply@mybot in a group chat is normalized to /reply and routed.
func TestHandleCommand_AddressedCommandInGroup_Routed(t *testing.T) {
	spy := &spyRouter{response: commands.Response{Text: "ok"}}
	ss := &spySender{}
	b := &Bot{router: spy, sender: ss, botUsername: "mybot"}

	update := &models.Update{
		Message: &models.Message{
			Text: "/reply@mybot all hello",
			Chat: models.Chat{ID: 100, Type: "supergroup"},
			From: &models.User{Username: "alice"},
		},
	}

	b.handleCommand(t.Context(), nil, update)

	if !spy.called {
		t.Fatal("router should be called for addressed command in group")
	}
	if spy.lastText != "/reply all hello" {
		t.Errorf("expected normalized text %q, got %q", "/reply all hello", spy.lastText)
	}
	if !ss.called {
		t.Fatal("sender should be called")
	}
}

// TestHandleCommand_OtherBotInGroup_Ignored verifies that a command
// addressed to a different bot in a group is silently ignored.
func TestHandleCommand_OtherBotInGroup_Ignored(t *testing.T) {
	spy := &spyRouter{response: commands.Response{Text: "should not see this"}}
	ss := &spySender{}
	b := &Bot{router: spy, sender: ss, botUsername: "mybot"}

	update := &models.Update{
		Message: &models.Message{
			Text: "/reply@otherbot all hello",
			Chat: models.Chat{ID: 100, Type: "group"},
			From: &models.User{Username: "alice"},
		},
	}

	b.handleCommand(t.Context(), nil, update)

	if spy.called {
		t.Error("router should NOT be called for command addressed to different bot")
	}
	if ss.called {
		t.Error("sender should NOT be called for command addressed to different bot")
	}
}

// TestHandleCommand_BareCommandInPrivate_Processed verifies that a bare
// command (without @botusername) in a private chat is processed normally.
func TestHandleCommand_BareCommandInPrivate_Processed(t *testing.T) {
	spy := &spyRouter{response: commands.Response{Text: "ok"}}
	ss := &spySender{}
	b := &Bot{router: spy, sender: ss, botUsername: "mybot"}

	update := &models.Update{
		Message: &models.Message{
			Text: "/reply all hello",
			Chat: models.Chat{ID: 100, Type: "private"},
			From: &models.User{Username: "alice"},
		},
	}

	b.handleCommand(t.Context(), nil, update)

	if !spy.called {
		t.Fatal("router should be called for bare command in private chat")
	}
	if spy.lastText != "/reply all hello" {
		t.Errorf("expected exact text %q, got %q", "/reply all hello", spy.lastText)
	}
}

// TestHandleCommand_CaseInsensitiveAddressing verifies that the @botusername
// check is case-insensitive (Telegram usernames are case-insensitive).
func TestHandleCommand_CaseInsensitiveAddressing(t *testing.T) {
	spy := &spyRouter{response: commands.Response{Text: "ok"}}
	ss := &spySender{}
	b := &Bot{router: spy, sender: ss, botUsername: "MyBot"}

	update := &models.Update{
		Message: &models.Message{
			Text: "/reply@mybot all hello",
			Chat: models.Chat{ID: 100, Type: "group"},
			From: &models.User{Username: "alice"},
		},
	}

	b.handleCommand(t.Context(), nil, update)

	if !spy.called {
		t.Fatal("router should be called — case-insensitive match")
	}
	if spy.lastText != "/reply all hello" {
		t.Errorf("expected normalized text %q, got %q", "/reply all hello", spy.lastText)
	}
}
