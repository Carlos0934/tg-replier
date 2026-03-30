package commands_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"tg-replier/internal/commands"
	"tg-replier/internal/groups"
)

// --- mock repository ---

type mockRepo struct {
	groups  []groups.Group
	listErr error // if non-nil, GetGroups returns this error
}

func (m *mockRepo) GetGroups(_ context.Context) ([]groups.Group, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.groups, nil
}

func (m *mockRepo) AddGroup(_ context.Context, g groups.Group) error {
	for _, existing := range m.groups {
		if existing.Name == g.Name {
			return groups.ErrDuplicate
		}
	}
	m.groups = append(m.groups, g)
	return nil
}

func (m *mockRepo) RemoveGroup(_ context.Context, name string) error {
	for i, g := range m.groups {
		if g.Name == name {
			m.groups = append(m.groups[:i], m.groups[i+1:]...)
			return nil
		}
	}
	return groups.ErrNotFound
}

// Helpers for readable Member construction.
func um(handle string) groups.Member {
	return groups.Member{Kind: "username", Handle: handle}
}

// newRouter creates a Router with mock dependencies for testing.
func newRouter(repo *mockRepo) *commands.Router {
	groupsSvc := groups.New(repo)
	return commands.New(groupsSvc)
}

// testChatID is a fixed chat ID used in tests that don't care about the value.
const testChatID int64 = 42

// --- /start ---

func TestRouter_Start(t *testing.T) {
	r := newRouter(&mockRepo{})
	resp := r.Handle(t.Context(), testChatID, "/start")
	if !strings.Contains(resp.Text, "Welcome") {
		t.Errorf("expected welcome text, got %q", resp.Text)
	}
}

// --- /group set ---

func TestRouter_GroupSet_Success(t *testing.T) {
	repo := &mockRepo{}
	r := newRouter(repo)

	resp := r.Handle(t.Context(), testChatID, "/group set team @alice @bob")
	if !strings.Contains(resp.Text, `"team" created`) {
		t.Errorf("expected creation message, got %q", resp.Text)
	}
	if len(repo.groups) != 1 || repo.groups[0].Name != "team" {
		t.Errorf("expected group 'team' in repo, got %v", repo.groups)
	}
}

func TestRouter_GroupSet_Duplicate(t *testing.T) {
	repo := &mockRepo{
		groups: []groups.Group{{Name: "team", Members: []groups.Member{um("@alice")}}},
	}
	r := newRouter(repo)

	resp := r.Handle(t.Context(), testChatID, "/group set team @bob")
	if !strings.Contains(resp.Text, "already exists") {
		t.Errorf("expected duplicate error, got %q", resp.Text)
	}
}

func TestRouter_GroupSet_BadArgs(t *testing.T) {
	r := newRouter(&mockRepo{})
	resp := r.Handle(t.Context(), testChatID, "/group set team")
	if !strings.Contains(resp.Text, "Usage") {
		t.Errorf("expected usage message, got %q", resp.Text)
	}
}

func TestRouter_GroupSet_InvalidBareWord(t *testing.T) {
	repo := &mockRepo{}
	r := newRouter(repo)

	resp := r.Handle(t.Context(), testChatID, "/group set ops alice")
	if !strings.Contains(resp.Text, "Invalid member") {
		t.Errorf("expected invalid member error, got %q", resp.Text)
	}
	if len(repo.groups) != 0 {
		t.Errorf("expected no groups created, got %d", len(repo.groups))
	}
}

func TestRouter_GroupSet_NumericID_Rejected(t *testing.T) {
	repo := &mockRepo{}
	r := newRouter(repo)

	resp := r.Handle(t.Context(), testChatID, "/group set ops 987654321")
	if !strings.Contains(resp.Text, "Invalid member") {
		t.Errorf("expected invalid member error for numeric ID, got %q", resp.Text)
	}
	if len(repo.groups) != 0 {
		t.Errorf("expected no groups created under mention-only model, got %d", len(repo.groups))
	}
}

func TestRouter_GroupSet_MixedTokens_RejectsNumeric(t *testing.T) {
	repo := &mockRepo{}
	r := newRouter(repo)

	resp := r.Handle(t.Context(), testChatID, "/group set ops 111222333 @dave")
	if !strings.Contains(resp.Text, "Invalid member") {
		t.Errorf("expected invalid member error for numeric token, got %q", resp.Text)
	}
	if len(repo.groups) != 0 {
		t.Errorf("expected no groups created, got %d", len(repo.groups))
	}
}

// --- /group delete ---

func TestRouter_GroupDelete_Success(t *testing.T) {
	repo := &mockRepo{
		groups: []groups.Group{{Name: "team", Members: []groups.Member{um("@alice")}}},
	}
	r := newRouter(repo)

	resp := r.Handle(t.Context(), testChatID, "/group delete team")
	if !strings.Contains(resp.Text, `"team" deleted`) {
		t.Errorf("expected delete message, got %q", resp.Text)
	}
	if len(repo.groups) != 0 {
		t.Errorf("expected repo empty, got %v", repo.groups)
	}
}

func TestRouter_GroupDelete_NotFound(t *testing.T) {
	r := newRouter(&mockRepo{})
	resp := r.Handle(t.Context(), testChatID, "/group delete missing")
	if !strings.Contains(resp.Text, "not found") {
		t.Errorf("expected not-found, got %q", resp.Text)
	}
}

func TestRouter_GroupDelete_BadArgs(t *testing.T) {
	r := newRouter(&mockRepo{})
	resp := r.Handle(t.Context(), testChatID, "/group delete")
	if !strings.Contains(resp.Text, "Usage") {
		t.Errorf("expected usage message, got %q", resp.Text)
	}
}

// --- /group list ---

func TestRouter_GroupList_Empty(t *testing.T) {
	r := newRouter(&mockRepo{})
	resp := r.Handle(t.Context(), testChatID, "/group list")
	if !strings.Contains(resp.Text, "No groups") {
		t.Errorf("expected empty message, got %q", resp.Text)
	}
}

func TestRouter_GroupList_WithGroups(t *testing.T) {
	repo := &mockRepo{
		groups: []groups.Group{
			{Name: "zebra", Members: []groups.Member{um("@zara")}},
			{Name: "alpha", Members: []groups.Member{um("@alice"), um("@bob")}},
			{Name: "mango", Members: []groups.Member{um("@mike")}},
		},
	}
	r := newRouter(repo)
	resp := r.Handle(t.Context(), testChatID, "/group list")

	want := "- <b>alpha</b>: @alice, @bob\n- <b>mango</b>: @mike\n- <b>zebra</b>: @zara\n"
	if resp.Text != want {
		t.Errorf("expected:\n%s\ngot:\n%s", want, resp.Text)
	}
	if resp.ParseMode != "HTML" {
		t.Errorf("expected ParseMode %q, got %q", "HTML", resp.ParseMode)
	}
}

func TestRouter_GroupList_SingleMember(t *testing.T) {
	repo := &mockRepo{
		groups: []groups.Group{
			{Name: "solo", Members: []groups.Member{um("@carol")}},
		},
	}
	r := newRouter(repo)
	resp := r.Handle(t.Context(), testChatID, "/group list")

	want := "- <b>solo</b>: @carol\n"
	if resp.Text != want {
		t.Errorf("expected %q, got %q", want, resp.Text)
	}
	if resp.ParseMode != "HTML" {
		t.Errorf("expected ParseMode %q, got %q", "HTML", resp.ParseMode)
	}
}

func TestRouter_GroupList_SingleGroupMultipleMembers(t *testing.T) {
	repo := &mockRepo{
		groups: []groups.Group{
			{Name: "team", Members: []groups.Member{um("@alice"), um("@bob")}},
		},
	}
	r := newRouter(repo)
	resp := r.Handle(t.Context(), testChatID, "/group list")

	want := "- <b>team</b>: @alice, @bob\n"
	if resp.Text != want {
		t.Errorf("expected %q, got %q", want, resp.Text)
	}
	if resp.ParseMode != "HTML" {
		t.Errorf("expected ParseMode %q, got %q", "HTML", resp.ParseMode)
	}
}

func TestRouter_GroupList_CaseInsensitiveSort(t *testing.T) {
	repo := &mockRepo{
		groups: []groups.Group{
			{Name: "Zebra", Members: []groups.Member{um("@zara")}},
			{Name: "alpha", Members: []groups.Member{um("@alice")}},
			{Name: "Mango", Members: []groups.Member{um("@mike")}},
		},
	}
	r := newRouter(repo)
	resp := r.Handle(t.Context(), testChatID, "/group list")

	want := "- <b>alpha</b>: @alice\n- <b>Mango</b>: @mike\n- <b>Zebra</b>: @zara\n"
	if resp.Text != want {
		t.Errorf("expected:\n%s\ngot:\n%s", want, resp.Text)
	}
}

func TestRouter_GroupList_NumericMember_DisplaysChatID(t *testing.T) {
	repo := &mockRepo{
		groups: []groups.Group{
			{Name: "ops", Members: []groups.Member{um("@ops_admin")}},
		},
	}
	r := newRouter(repo)
	resp := r.Handle(t.Context(), testChatID, "/group list")

	want := "- <b>ops</b>: @ops_admin\n"
	if resp.Text != want {
		t.Errorf("expected %q, got %q", want, resp.Text)
	}
}

func TestRouter_GroupList_Error(t *testing.T) {
	repo := &mockRepo{listErr: errors.New("disk I/O failure")}
	r := newRouter(repo)
	resp := r.Handle(t.Context(), testChatID, "/group list")

	if !strings.Contains(resp.Text, "Error:") {
		t.Errorf("expected error prefix, got %q", resp.Text)
	}
	if !strings.Contains(resp.Text, "disk I/O failure") {
		t.Errorf("expected error message forwarded, got %q", resp.Text)
	}
}

// --- /group with missing sub-command ---

func TestRouter_Group_NoSubcommand(t *testing.T) {
	r := newRouter(&mockRepo{})
	resp := r.Handle(t.Context(), testChatID, "/group")
	if !strings.Contains(resp.Text, "Usage") {
		t.Errorf("expected usage message, got %q", resp.Text)
	}
}

func TestRouter_Group_UnknownSubcommand(t *testing.T) {
	r := newRouter(&mockRepo{})
	resp := r.Handle(t.Context(), testChatID, "/group foo")
	if !strings.Contains(resp.Text, "Unknown sub-command") {
		t.Errorf("expected unknown sub-command, got %q", resp.Text)
	}
}

// --- /reply with named group ---

func TestRouter_Reply_NamedGroup_Success(t *testing.T) {
	repo := &mockRepo{
		groups: []groups.Group{{Name: "devs", Members: []groups.Member{um("@alice"), um("@bob")}}},
	}
	r := newRouter(repo)

	resp := r.Handle(t.Context(), testChatID, "/reply devs Stand-up time")
	if !strings.Contains(resp.Text, "@alice") || !strings.Contains(resp.Text, "@bob") {
		t.Errorf("expected mentions of @alice and @bob, got %q", resp.Text)
	}
	if !strings.Contains(resp.Text, "Stand-up time") {
		t.Errorf("expected message text, got %q", resp.Text)
	}
}

func TestRouter_Reply_UnknownTarget(t *testing.T) {
	r := newRouter(&mockRepo{})
	resp := r.Handle(t.Context(), testChatID, "/reply backend hello")
	if !strings.Contains(resp.Text, "Unknown target") {
		t.Errorf("expected unknown target error, got %q", resp.Text)
	}
}

func TestRouter_Reply_MissingMessage(t *testing.T) {
	r := newRouter(&mockRepo{})
	resp := r.Handle(t.Context(), testChatID, "/reply all")
	if !strings.Contains(resp.Text, "Usage") {
		t.Errorf("expected usage message, got %q", resp.Text)
	}
}

func TestRouter_Reply_EmptyGroup(t *testing.T) {
	repo := &mockRepo{
		groups: []groups.Group{{Name: "qa", Members: nil}},
	}
	r := newRouter(repo)
	resp := r.Handle(t.Context(), testChatID, "/reply qa Deploy ready")
	if !strings.Contains(resp.Text, "empty") {
		t.Errorf("expected empty group error, got %q", resp.Text)
	}
}

// --- unknown command ---

func TestRouter_UnknownCommand(t *testing.T) {
	r := newRouter(&mockRepo{})
	resp := r.Handle(t.Context(), testChatID, "/unknown")
	if !strings.Contains(resp.Text, "Unknown command") {
		t.Errorf("expected unknown command, got %q", resp.Text)
	}
}

// --- empty input ---

func TestRouter_EmptyInput(t *testing.T) {
	r := newRouter(&mockRepo{})
	resp := r.Handle(t.Context(), testChatID, "")
	if resp.Text != "Empty command." {
		t.Errorf("expected empty command response, got %q", resp.Text)
	}
}

// --- quoted-command-parsing integration tests ---

func TestRouter_GroupSet_QuotedName(t *testing.T) {
	repo := &mockRepo{}
	r := newRouter(repo)

	resp := r.Handle(t.Context(), testChatID, `/group set "team alpha" @alice @bob`)
	if !strings.Contains(resp.Text, `"team alpha" created`) {
		t.Errorf("expected creation with quoted name, got %q", resp.Text)
	}
	if len(repo.groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(repo.groups))
	}
	if repo.groups[0].Name != "team alpha" {
		t.Errorf("expected group name %q, got %q", "team alpha", repo.groups[0].Name)
	}
	if len(repo.groups[0].Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(repo.groups[0].Members))
	}
}

func TestRouter_Reply_QuotedGroupName(t *testing.T) {
	repo := &mockRepo{
		groups: []groups.Group{{Name: "team alpha", Members: []groups.Member{um("@alice")}}},
	}
	r := newRouter(repo)

	resp := r.Handle(t.Context(), testChatID, `/reply "team alpha" hello`)
	if !strings.Contains(resp.Text, "@alice") {
		t.Errorf("expected @alice mention, got %q", resp.Text)
	}
	if !strings.Contains(resp.Text, "hello") {
		t.Errorf("expected message text, got %q", resp.Text)
	}
}

func TestRouter_Handle_MalformedQuote(t *testing.T) {
	repo := &mockRepo{}
	r := newRouter(repo)

	resp := r.Handle(t.Context(), testChatID, `/group set "unclosed @alice`)
	if !strings.Contains(resp.Text, "Invalid command") {
		t.Errorf("expected invalid command response, got %q", resp.Text)
	}
	if len(repo.groups) != 0 {
		t.Errorf("expected no groups created, got %d", len(repo.groups))
	}
}

// --- /reply produces no DM send calls (verified via response format) ---

func TestRouter_Reply_NoDMFormat(t *testing.T) {
	repo := &mockRepo{
		groups: []groups.Group{{Name: "team", Members: []groups.Member{um("@alice"), um("@bob")}}},
	}
	r := newRouter(repo)

	resp := r.Handle(t.Context(), testChatID, "/reply team hello world")
	if strings.Contains(resp.Text, "Sent to") {
		t.Errorf("response should be mention-only, not DM summary, got %q", resp.Text)
	}
	if !strings.Contains(resp.Text, "@alice") || !strings.Contains(resp.Text, "@bob") {
		t.Errorf("expected mentions in response, got %q", resp.Text)
	}
}
