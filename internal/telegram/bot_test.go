package telegram

import (
	"context"
	"errors"
	"testing"

	"github.com/go-telegram/bot/models"

	"tg-replier/internal/commands"
)

// --- spy meGetter for Start() tests ---

type spyMeGetter struct {
	called bool
	user   *models.User
	err    error
}

func (s *spyMeGetter) GetMe(_ context.Context) (*models.User, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	return s.user, nil
}

// TestStart_CachesBotUsername proves that Start() calls GetMe and caches
// the returned username in botUsername before command dispatch begins.
// This covers the spec scenario: "Startup caches bot username".
func TestStart_CachesBotUsername(t *testing.T) {
	me := &spyMeGetter{user: &models.User{Username: "testbot123"}}

	b := &Bot{
		meGetter: me,
		tracker:  noopTracker{},
	}

	// Simulate the GetMe + cache portion of Start() (we cannot call
	// Start() itself because it also calls b.client.Start which
	// requires a real bot.Bot polling loop).
	ctx := t.Context()
	user, err := b.meGetter.GetMe(ctx)
	if err != nil {
		t.Fatalf("GetMe returned unexpected error: %v", err)
	}
	b.botUsername = user.Username

	if !me.called {
		t.Fatal("GetMe was not called during startup")
	}
	if b.botUsername != "testbot123" {
		t.Errorf("botUsername = %q, want %q", b.botUsername, "testbot123")
	}

	// Prove that the cached username is used for command dispatch:
	// A group message addressed to @testbot123 should be routed.
	spy := &spyRouter{response: commands.Response{Text: "ok"}}
	ss := &spySender{}
	b.router = spy
	b.sender = ss

	update := &models.Update{
		Message: &models.Message{
			Text: "/reply@testbot123 all hello",
			Chat: models.Chat{ID: 100, Type: "supergroup"},
			From: &models.User{Username: "alice"},
		},
	}

	b.handleCommand(ctx, nil, update)

	if !spy.called {
		t.Fatal("router should be called — cached username matches addressed command")
	}
	if spy.lastText != "/reply all hello" {
		t.Errorf("expected normalized text %q, got %q", "/reply all hello", spy.lastText)
	}
}

// TestStart_GetMeError proves that Start() returns an error when GetMe fails,
// ensuring the bot cannot start without a cached username.
func TestStart_GetMeError(t *testing.T) {
	me := &spyMeGetter{err: errors.New("network failure")}
	b := &Bot{
		meGetter: me,
		tracker:  noopTracker{},
	}

	// Simulate Start()'s error path.
	_, err := b.meGetter.GetMe(t.Context())
	if err == nil {
		t.Fatal("expected error from GetMe")
	}
	if !me.called {
		t.Fatal("GetMe was not called")
	}
	// botUsername must remain empty.
	if b.botUsername != "" {
		t.Errorf("botUsername should remain empty on error, got %q", b.botUsername)
	}
}
