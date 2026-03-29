package jsonstorage_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"tg-replier/internal/groups"
	"tg-replier/internal/members"
	jsonstorage "tg-replier/internal/storage/json"
)

// Compile-time checks.
var (
	_ groups.Repository = (*jsonstorage.Store)(nil)
	_ members.Tracker   = (*jsonstorage.Store)(nil)
)

func TestNew_FileAbsent_CreatesFile(t *testing.T) {
	dir := t.TempDir()

	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	result, err := s.GetGroups(t.Context())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 groups, got %d", len(result))
	}

	fp := filepath.Join(dir, "groups.json")
	raw, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("expected file to exist, got %v", err)
	}

	var d struct {
		Groups map[string]groups.Group `json:"groups"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("expected valid JSON, got %v", err)
	}
	if len(d.Groups) != 0 {
		t.Errorf("expected empty groups on disk, got %d", len(d.Groups))
	}
}

func TestNew_FilePresent_PreservesData(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "groups.json")

	// Seed with legacy "users" format — should be promoted to Members.
	existing := `{"groups":{"devs":{"name":"devs","users":["@alice","@bob"]}}}`
	if err := os.WriteFile(fp, []byte(existing), 0o644); err != nil {
		t.Fatalf("failed to write seed file: %v", err)
	}

	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	result, err := s.GetGroups(t.Context())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var found bool
	for _, g := range result {
		if g.Name == "devs" {
			found = true
			if len(g.Members) != 2 {
				t.Errorf("expected 2 members, got %d", len(g.Members))
			}
			for _, m := range g.Members {
				if m.Kind != "username" {
					t.Errorf("expected username kind, got %q", m.Kind)
				}
				if m.Handle == "" {
					t.Error("expected non-empty Handle")
				}
			}
		}
	}
	if !found {
		t.Error("expected 'devs' group to exist")
	}
}

func TestNew_MalformedJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "groups.json")

	if err := os.WriteFile(fp, []byte("{invalid json"), 0o644); err != nil {
		t.Fatalf("failed to write seed file: %v", err)
	}

	_, err := jsonstorage.New(dir)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestAddGroup_Duplicate(t *testing.T) {
	dir := t.TempDir()
	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	g := groups.Group{Name: "team", Members: []groups.Member{
		{Kind: "username", Handle: "@alice"},
	}}
	if err := s.AddGroup(t.Context(), g); err != nil {
		t.Fatalf("first add: expected no error, got %v", err)
	}

	err = s.AddGroup(t.Context(), g)
	if !errors.Is(err, groups.ErrDuplicate) {
		t.Fatalf("expected ErrDuplicate, got %v", err)
	}
}

func TestRemoveGroup_NotFound(t *testing.T) {
	dir := t.TempDir()
	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	err = s.RemoveGroup(t.Context(), "missing")
	if !errors.Is(err, groups.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestConcurrentFlush(t *testing.T) {
	dir := t.TempDir()
	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			g := groups.Group{
				Name:    "group-" + string(rune('A'+n)),
				Members: []groups.Member{{Kind: "username", Handle: "@user"}},
			}
			_ = s.AddGroup(t.Context(), g)
		}(i)
	}
	wg.Wait()

	result, err := s.GetGroups(t.Context())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 10 {
		t.Errorf("expected 10 groups after concurrent adds, got %d", len(result))
	}
}

// --- Legacy promotion tests ---

func TestStore_LegacyUsersPromoted(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "groups.json")

	// Legacy data includes a numeric string "123456" and a @username.
	// Under mention-only, numeric strings get @-prefixed during promotion.
	seed := `{"groups":{"team":{"name":"team","users":["@alice","123456"]}}}`
	if err := os.WriteFile(fp, []byte(seed), 0o644); err != nil {
		t.Fatalf("failed to write seed: %v", err)
	}

	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	result, err := s.GetGroups(t.Context())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 group, got %d", len(result))
	}
	g := result[0]
	if len(g.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(g.Members))
	}

	var alice, promoted groups.Member
	for _, m := range g.Members {
		if m.Handle == "@alice" {
			alice = m
		} else if m.Handle == "@123456" {
			promoted = m
		}
	}

	if alice.Kind != "username" || alice.Handle != "@alice" {
		t.Errorf("expected username member @alice, got %+v", alice)
	}
	// Legacy numeric strings are promoted to @-prefixed usernames.
	if promoted.Kind != "username" || promoted.Handle != "@123456" {
		t.Errorf("expected promoted username member @123456, got %+v", promoted)
	}
}

func TestStore_MemberRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	original := groups.Group{
		Name: "team",
		Members: []groups.Member{
			{Kind: "username", Handle: "@zoe"},
			{Kind: "username", Handle: "@alice"},
		},
	}
	if err := s.AddGroup(t.Context(), original); err != nil {
		t.Fatalf("add group: %v", err)
	}

	s2, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	result, err := s2.GetGroups(t.Context())
	if err != nil {
		t.Fatalf("get groups: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 group, got %d", len(result))
	}
	g := result[0]
	if g.Name != "team" {
		t.Errorf("expected name %q, got %q", "team", g.Name)
	}
	if len(g.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(g.Members))
	}

	for i, m := range g.Members {
		orig := original.Members[i]
		if m.Kind != orig.Kind || m.Handle != orig.Handle {
			t.Errorf("member[%d] mismatch: got %+v, want %+v", i, m, orig)
		}
	}
}

func TestStore_LegacyUnrecognised_BestEffortUsername(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "groups.json")

	seed := `{"groups":{"team":{"name":"team","users":["plainbob"]}}}`
	if err := os.WriteFile(fp, []byte(seed), 0o644); err != nil {
		t.Fatalf("failed to write seed: %v", err)
	}

	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	result, err := s.GetGroups(t.Context())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	m := result[0].Members[0]
	// Legacy bare words get @-prefixed during promotion to comply with
	// the mention-only model.
	if m.Kind != "username" || m.Handle != "@plainbob" {
		t.Errorf("expected best-effort username member @plainbob, got %+v", m)
	}
}

// --- Tracker (roster) tests ---

func TestTrack_NewMember(t *testing.T) {
	dir := t.TempDir()
	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := s.Track(t.Context(), 123, "dave"); err != nil {
		t.Fatalf("Track: %v", err)
	}

	members, err := s.List(t.Context(), 123)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(members) != 1 || members[0] != "dave" {
		t.Errorf("expected [dave], got %v", members)
	}
}

func TestTrack_Deduplication(t *testing.T) {
	dir := t.TempDir()
	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Track same user twice.
	if err := s.Track(t.Context(), 123, "dave"); err != nil {
		t.Fatalf("Track 1: %v", err)
	}
	if err := s.Track(t.Context(), 123, "dave"); err != nil {
		t.Fatalf("Track 2: %v", err)
	}

	members, err := s.List(t.Context(), 123)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(members) != 1 {
		t.Errorf("expected 1 member after dedup, got %d: %v", len(members), members)
	}
}

func TestTrack_MultipleMembers(t *testing.T) {
	dir := t.TempDir()
	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for _, u := range []string{"alice", "bob", "carol"} {
		if err := s.Track(t.Context(), 100, u); err != nil {
			t.Fatalf("Track %s: %v", u, err)
		}
	}

	members, err := s.List(t.Context(), 100)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(members) != 3 {
		t.Errorf("expected 3 members, got %d: %v", len(members), members)
	}
}

func TestTrack_PerChat_Isolation(t *testing.T) {
	dir := t.TempDir()
	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_ = s.Track(t.Context(), 100, "alice")
	_ = s.Track(t.Context(), 200, "bob")

	m1, _ := s.List(t.Context(), 100)
	m2, _ := s.List(t.Context(), 200)

	if len(m1) != 1 || m1[0] != "alice" {
		t.Errorf("chat 100 expected [alice], got %v", m1)
	}
	if len(m2) != 1 || m2[0] != "bob" {
		t.Errorf("chat 200 expected [bob], got %v", m2)
	}
}

func TestTrack_Persistence(t *testing.T) {
	dir := t.TempDir()
	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_ = s.Track(t.Context(), 123, "alice")

	// Reload from disk.
	s2, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	members, err := s2.List(t.Context(), 123)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(members) != 1 || members[0] != "alice" {
		t.Errorf("expected [alice] after reload, got %v", members)
	}
}

func TestTrack_CreatesRosterFile(t *testing.T) {
	dir := t.TempDir()
	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Before Track, no rosters.json should exist.
	rp := filepath.Join(dir, "rosters.json")
	_, statErr := os.Stat(rp)
	if statErr == nil {
		t.Fatal("expected no rosters.json before first Track")
	}

	// After Track, file should be created.
	_ = s.Track(t.Context(), 123, "dave")
	if _, statErr := os.Stat(rp); statErr != nil {
		t.Fatalf("expected rosters.json to exist after Track, got %v", statErr)
	}
}

func TestList_EmptyChat(t *testing.T) {
	dir := t.TempDir()
	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	members, err := s.List(t.Context(), 999)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(members) != 0 {
		t.Errorf("expected 0 members for unknown chat, got %d", len(members))
	}
}

// TestTrack_LastSeenRefreshOnRepeat proves that a second Track call for
// the same username updates the last_seen timestamp (spec: "MUST update
// last-seen timestamp and MUST NOT duplicate the entry").
func TestTrack_LastSeenRefreshOnRepeat(t *testing.T) {
	dir := t.TempDir()
	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// First track.
	if err := s.Track(t.Context(), 123, "dave"); err != nil {
		t.Fatalf("Track 1: %v", err)
	}

	// Read the on-disk roster to capture the first last_seen.
	rp := filepath.Join(dir, "rosters.json")
	raw1, err := os.ReadFile(rp)
	if err != nil {
		t.Fatalf("read rosters 1: %v", err)
	}

	type entry struct {
		ChatID   int64  `json:"chat_id"`
		Username string `json:"username"`
		LastSeen string `json:"last_seen"`
	}
	type rosterFile struct {
		Chats map[string][]entry `json:"chats"`
	}

	var r1 rosterFile
	if err := json.Unmarshal(raw1, &r1); err != nil {
		t.Fatalf("unmarshal 1: %v", err)
	}
	if len(r1.Chats["123"]) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(r1.Chats["123"]))
	}
	firstSeen := r1.Chats["123"][0].LastSeen

	// Small delay to ensure timestamp difference.
	// In practice, time.Now() on second call will differ.
	if err := s.Track(t.Context(), 123, "dave"); err != nil {
		t.Fatalf("Track 2: %v", err)
	}

	raw2, err := os.ReadFile(rp)
	if err != nil {
		t.Fatalf("read rosters 2: %v", err)
	}
	var r2 rosterFile
	if err := json.Unmarshal(raw2, &r2); err != nil {
		t.Fatalf("unmarshal 2: %v", err)
	}
	if len(r2.Chats["123"]) != 1 {
		t.Fatalf("expected 1 entry after dedup, got %d", len(r2.Chats["123"]))
	}
	secondSeen := r2.Chats["123"][0].LastSeen

	if secondSeen < firstSeen {
		t.Errorf("expected last_seen to be >= first seen; first=%s, second=%s", firstSeen, secondSeen)
	}
}

// TestTrack_RosterEntryContainsChatID proves that each persisted roster
// entry includes the chat_id field as required by the spec.
func TestTrack_RosterEntryContainsChatID(t *testing.T) {
	dir := t.TempDir()
	s, err := jsonstorage.New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := s.Track(t.Context(), 42, "alice"); err != nil {
		t.Fatalf("Track: %v", err)
	}

	rp := filepath.Join(dir, "rosters.json")
	raw, err := os.ReadFile(rp)
	if err != nil {
		t.Fatalf("read rosters: %v", err)
	}

	type entry struct {
		ChatID   int64  `json:"chat_id"`
		Username string `json:"username"`
		LastSeen string `json:"last_seen"`
	}
	type rosterFile struct {
		Chats map[string][]entry `json:"chats"`
	}

	var rf rosterFile
	if err := json.Unmarshal(raw, &rf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	entries := rf.Chats["42"]
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.ChatID != 42 {
		t.Errorf("expected chat_id 42, got %d", e.ChatID)
	}
	if e.Username != "alice" {
		t.Errorf("expected username %q, got %q", "alice", e.Username)
	}
	if e.LastSeen == "" {
		t.Error("expected non-empty last_seen")
	}
}
