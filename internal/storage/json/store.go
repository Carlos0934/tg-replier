package jsonstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"tg-replyer/internal/groups"
)

// data is the on-disk JSON schema for groups.json.
type data struct {
	Groups map[string]groups.Group `json:"groups"`
}

// rosterEntry stores a single tracked member.
type rosterEntry struct {
	ChatID   int64     `json:"chat_id"`
	Username string    `json:"username"`
	LastSeen time.Time `json:"last_seen"`
}

// rosterData is the on-disk JSON schema for rosters.json.
type rosterData struct {
	// Chats maps chat ID (as string key) to its known members.
	Chats map[string][]rosterEntry `json:"chats"`
}

// legacyGroup is a shadow struct used only during unmarshal to detect the
// old "users" string-array shape alongside the new "members" shape.
type legacyGroup struct {
	Name    string          `json:"name"`
	Users   []string        `json:"users,omitempty"`
	Members []groups.Member `json:"members,omitempty"`
}

// legacyData mirrors the top-level data struct but uses legacyGroup.
type legacyData struct {
	Groups map[string]legacyGroup `json:"groups"`
}

// promoteLegacyUsers converts old-style string handles into typed Members.
// Under the mention-only model, only @-prefixed strings become username
// members. Numeric-only strings and bare words are promoted with a best-effort
// @-prefix so existing data is not silently lost.
func promoteLegacyUsers(raw []string) []groups.Member {
	members := make([]groups.Member, 0, len(raw))
	for _, s := range raw {
		if s == "" {
			continue
		}
		switch {
		case s[0] == '@':
			members = append(members, groups.Member{Kind: "username", Handle: s})
		default:
			// Best-effort: prepend @ to non-@ strings during legacy promotion.
			members = append(members, groups.Member{Kind: "username", Handle: "@" + s})
		}
	}
	return members
}

// Store manages persistence for groups and rosters via JSON flat files.
// It implements both groups.Repository and members.Tracker interfaces.
type Store struct {
	mu      sync.Mutex
	dataDir string
	path    string // groups.json path
	groups  map[string]groups.Group

	rosterPath string                   // rosters.json path
	rosters    map[string][]rosterEntry // keyed by chat ID string
}

// New initialises the store backed by groups.json and rosters.json inside dataDir.
// If the files do not exist they are created with empty structures.
// If they exist, they are loaded and parsed. Legacy "users" arrays are
// silently promoted to typed Members on load.
func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	fp := filepath.Join(dataDir, "groups.json")
	rp := filepath.Join(dataDir, "rosters.json")
	s := &Store{dataDir: dataDir, path: fp, rosterPath: rp}

	// --- Load groups ---
	_, err := os.Stat(fp)
	switch {
	case errors.Is(err, os.ErrNotExist):
		s.groups = make(map[string]groups.Group)
		if writeErr := s.flushGroups(); writeErr != nil {
			return nil, writeErr
		}
	case err != nil:
		return nil, err
	default:
		raw, readErr := os.ReadFile(fp)
		if readErr != nil {
			return nil, readErr
		}

		// Unmarshal into legacy shape to detect old "users" arrays.
		var ld legacyData
		if jsonErr := json.Unmarshal(raw, &ld); jsonErr != nil {
			return nil, jsonErr
		}

		s.groups = make(map[string]groups.Group, len(ld.Groups))
		for k, lg := range ld.Groups {
			g := groups.Group{Name: lg.Name, Members: lg.Members}
			// Promote legacy users if members is empty but users exists.
			if len(g.Members) == 0 && len(lg.Users) > 0 {
				g.Members = promoteLegacyUsers(lg.Users)
			}
			s.groups[k] = g
		}
	}

	// --- Load rosters ---
	_, err = os.Stat(rp)
	switch {
	case errors.Is(err, os.ErrNotExist):
		s.rosters = make(map[string][]rosterEntry)
		// Don't flush an empty file — it will be created on first Track.
	case err != nil:
		return nil, err
	default:
		raw, readErr := os.ReadFile(rp)
		if readErr != nil {
			return nil, readErr
		}
		var rd rosterData
		if jsonErr := json.Unmarshal(raw, &rd); jsonErr != nil {
			return nil, jsonErr
		}
		s.rosters = rd.Chats
		if s.rosters == nil {
			s.rosters = make(map[string][]rosterEntry)
		}
	}

	return s, nil
}

// --- groups.Repository implementation ---

// GetGroups returns all stored groups.
func (s *Store) GetGroups(_ context.Context) ([]groups.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]groups.Group, 0, len(s.groups))
	for _, g := range s.groups {
		result = append(result, g)
	}
	return result, nil
}

// AddGroup persists a new group. Returns groups.ErrDuplicate if a group
// with the same name already exists.
func (s *Store) AddGroup(_ context.Context, g groups.Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.groups[g.Name]; exists {
		return groups.ErrDuplicate
	}
	s.groups[g.Name] = g
	return s.flushGroups()
}

// RemoveGroup deletes a group by name. Returns groups.ErrNotFound if it
// does not exist.
func (s *Store) RemoveGroup(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.groups[name]; !exists {
		return groups.ErrNotFound
	}
	delete(s.groups, name)
	return s.flushGroups()
}

// --- members.Tracker implementation ---

// Track records a username as observed in the given chat.
// It deduplicates by username and updates last-seen on repeat sightings.
// Creates the roster file on first call if it does not exist.
func (s *Store) Track(_ context.Context, chatID int64, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%d", chatID)
	entries := s.rosters[key]

	now := time.Now().UTC()
	for i, e := range entries {
		if e.Username == username {
			entries[i].LastSeen = now
			entries[i].ChatID = chatID
			s.rosters[key] = entries
			return s.flushRosters()
		}
	}

	s.rosters[key] = append(entries, rosterEntry{ChatID: chatID, Username: username, LastSeen: now})
	return s.flushRosters()
}

// List returns all known usernames for the given chat.
func (s *Store) List(_ context.Context, chatID int64) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%d", chatID)
	entries := s.rosters[key]
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Username
	}
	return names, nil
}

// --- flush helpers ---

// flushGroups writes the groups state to disk. Caller must hold mu.
func (s *Store) flushGroups() error {
	raw, err := json.MarshalIndent(data{Groups: s.groups}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o644)
}

// flushRosters writes the roster state to disk. Caller must hold mu.
func (s *Store) flushRosters() error {
	raw, err := json.MarshalIndent(rosterData{Chats: s.rosters}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.rosterPath, raw, 0o644)
}
