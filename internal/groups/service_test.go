package groups_test

import (
	"context"
	"errors"
	"testing"

	"tg-replier/internal/groups"
)

// mockRepo is a simple in-memory groups.Repository for testing.
type mockRepo struct {
	groups []groups.Group
}

func (m *mockRepo) GetGroups(_ context.Context) ([]groups.Group, error) {
	return m.groups, nil
}

func (m *mockRepo) AddGroup(_ context.Context, g groups.Group) error {
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

func TestService_Set_Success(t *testing.T) {
	repo := &mockRepo{}
	svc := groups.New(repo)

	members := []groups.Member{
		{Kind: "username", Handle: "@alice"},
		{Kind: "username", Handle: "@bob"},
	}
	if err := svc.Set(t.Context(), "team", members); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(repo.groups))
	}
	if repo.groups[0].Name != "team" {
		t.Errorf("expected group name %q, got %q", "team", repo.groups[0].Name)
	}
}

func TestService_Set_Duplicate(t *testing.T) {
	repo := &mockRepo{
		groups: []groups.Group{{Name: "team", Members: []groups.Member{
			{Kind: "username", Handle: "@alice"},
		}}},
	}
	svc := groups.New(repo)

	err := svc.Set(t.Context(), "team", []groups.Member{{Kind: "username", Handle: "@bob"}})
	if !errors.Is(err, groups.ErrDuplicate) {
		t.Fatalf("expected ErrDuplicate, got %v", err)
	}
}

func TestService_Set_NumericID_Rejected(t *testing.T) {
	repo := &mockRepo{}
	svc := groups.New(repo)

	members := []groups.Member{
		{Kind: "username", Handle: ""},
	}
	err := svc.Set(t.Context(), "ops", members)
	if !errors.Is(err, groups.ErrInvalidMember) {
		t.Fatalf("expected ErrInvalidMember for empty handle, got %v", err)
	}
	if len(repo.groups) != 0 {
		t.Errorf("expected no groups created, got %d", len(repo.groups))
	}
}

func TestService_Set_Username(t *testing.T) {
	repo := &mockRepo{}
	svc := groups.New(repo)

	members := []groups.Member{
		{Kind: "username", Handle: "@bob"},
	}
	if err := svc.Set(t.Context(), "devs", members); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got := repo.groups[0].Members[0]
	if got.Kind != "username" || got.Handle != "@bob" {
		t.Errorf("expected username member @bob, got %+v", got)
	}
}

func TestService_Set_InvalidToken(t *testing.T) {
	repo := &mockRepo{}
	svc := groups.New(repo)

	// A member with empty DeliveryTarget should be rejected.
	members := []groups.Member{
		{Kind: "", Handle: ""},
	}
	err := svc.Set(t.Context(), "team", members)
	if !errors.Is(err, groups.ErrInvalidMember) {
		t.Fatalf("expected ErrInvalidMember, got %v", err)
	}
	if len(repo.groups) != 0 {
		t.Errorf("expected no groups created, got %d", len(repo.groups))
	}
}

func TestService_Delete_Success(t *testing.T) {
	repo := &mockRepo{
		groups: []groups.Group{{Name: "team", Members: []groups.Member{
			{Kind: "username", Handle: "@alice"},
		}}},
	}
	svc := groups.New(repo)

	if err := svc.Delete(t.Context(), "team"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(repo.groups))
	}
}

func TestService_Delete_NotFound(t *testing.T) {
	repo := &mockRepo{}
	svc := groups.New(repo)

	err := svc.Delete(t.Context(), "missing")
	if !errors.Is(err, groups.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestService_List_Empty(t *testing.T) {
	repo := &mockRepo{}
	svc := groups.New(repo)

	result, err := svc.List(t.Context())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 groups, got %d", len(result))
	}
}

func TestService_List_WithGroups(t *testing.T) {
	repo := &mockRepo{
		groups: []groups.Group{
			{Name: "team", Members: []groups.Member{{Kind: "username", Handle: "@alice"}}},
			{Name: "devs", Members: []groups.Member{{Kind: "username", Handle: "@bob"}}},
		},
	}
	svc := groups.New(repo)

	result, err := svc.List(t.Context())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 groups, got %d", len(result))
	}
}

// --- ParseMember tests ---

func TestParseMember_NumericID_Rejected(t *testing.T) {
	_, err := groups.ParseMember("123456789")
	if !errors.Is(err, groups.ErrInvalidMember) {
		t.Fatalf("expected ErrInvalidMember for numeric ID under mention-only model, got %v", err)
	}
}

func TestParseMember_Username(t *testing.T) {
	m, err := groups.ParseMember("@alice")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if m.Kind != "username" || m.Handle != "@alice" {
		t.Errorf("expected username member @alice, got %+v", m)
	}
}

func TestParseMember_BareWord_Rejected(t *testing.T) {
	_, err := groups.ParseMember("alice")
	if !errors.Is(err, groups.ErrInvalidMember) {
		t.Fatalf("expected ErrInvalidMember, got %v", err)
	}
}

func TestParseMember_Empty_Rejected(t *testing.T) {
	_, err := groups.ParseMember("")
	if !errors.Is(err, groups.ErrInvalidMember) {
		t.Fatalf("expected ErrInvalidMember, got %v", err)
	}
}

func TestParseMember_AtOnly_Rejected(t *testing.T) {
	_, err := groups.ParseMember("@")
	if !errors.Is(err, groups.ErrInvalidMember) {
		t.Fatalf("expected ErrInvalidMember, got %v", err)
	}
}

func TestParseMember_UsernameWithSpaces_Rejected(t *testing.T) {
	cases := []string{
		"@alice smith",
		"@alice\tsmith",
		"@alice\nsmith",
		"@ spacefirst",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			_, err := groups.ParseMember(tc)
			if !errors.Is(err, groups.ErrInvalidMember) {
				t.Errorf("ParseMember(%q): expected ErrInvalidMember, got %v", tc, err)
			}
		})
	}
}
