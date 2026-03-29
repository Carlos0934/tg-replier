package groups

import (
	"context"
	"unicode"
)

// Member represents a group member identified by a @username handle.
// The mention-only model requires all members to have a valid username
// for in-chat @mention delivery.
type Member struct {
	Kind   string `json:"kind"`
	Handle string `json:"handle"`
}

// DisplayName returns the human-readable label for the member.
func (m Member) DisplayName() string {
	return m.Handle
}

// DeliveryTarget returns the @username handle used for mention delivery.
func (m Member) DeliveryTarget() string {
	return m.Handle
}

// Group represents a reply group with a name and list of members.
type Group struct {
	Name    string   `json:"name"`
	Members []Member `json:"members"`
}

// ParseMember classifies a raw token into a Member.
// Under the mention-only model, only @username tokens are accepted.
// Numeric chat IDs and bare words are rejected with ErrInvalidMember.
func ParseMember(token string) (Member, error) {
	if token == "" {
		return Member{}, ErrInvalidMember
	}

	// Starts with @ → username handle (must contain no spaces per spec)
	if token[0] == '@' && len(token) > 1 {
		for _, r := range token[1:] {
			if unicode.IsSpace(r) {
				return Member{}, ErrInvalidMember
			}
		}
		return Member{Kind: "username", Handle: token}, nil
	}

	return Member{}, ErrInvalidMember
}

// Repository abstracts persistence for reply groups.
type Repository interface {
	GetGroups(ctx context.Context) ([]Group, error)
	AddGroup(ctx context.Context, group Group) error
	RemoveGroup(ctx context.Context, name string) error
}
