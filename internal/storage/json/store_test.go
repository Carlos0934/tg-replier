package jsonstorage_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"tg-replier/internal/groups"
	jsonstorage "tg-replier/internal/storage/json"
)

// Compile-time checks.
var _ groups.Repository = (*jsonstorage.Store)(nil)

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
