package groups

import (
	"context"
	"errors"
	"fmt"
)

// Sentinel errors for the groups domain.
var (
	ErrDuplicate     = errors.New("group already exists")
	ErrNotFound      = errors.New("group not found")
	ErrInvalidMember = errors.New("invalid member token: must be an @username")
)

// Service encapsulates group management use-cases.
type Service struct {
	repo Repository
}

// New creates a Service backed by the given repository.
func New(repo Repository) *Service {
	return &Service{repo: repo}
}

// Set creates a new group with the given name and members.
// Returns ErrDuplicate if a group with that name already exists.
// Returns ErrInvalidMember if any member has an empty DeliveryTarget.
func (s *Service) Set(ctx context.Context, name string, members []Member) error {
	for _, m := range members {
		if m.DeliveryTarget() == "" {
			return ErrInvalidMember
		}
	}

	existing, err := s.repo.GetGroups(ctx)
	if err != nil {
		return fmt.Errorf("listing groups: %w", err)
	}
	for _, g := range existing {
		if g.Name == name {
			return ErrDuplicate
		}
	}
	return s.repo.AddGroup(ctx, Group{Name: name, Members: members})
}

// Delete removes the group with the given name.
// Returns ErrNotFound if no such group exists.
func (s *Service) Delete(ctx context.Context, name string) error {
	err := s.repo.RemoveGroup(ctx, name)
	if err != nil {
		return err
	}
	return nil
}

// List returns all groups.
func (s *Service) List(ctx context.Context) ([]Group, error) {
	return s.repo.GetGroups(ctx)
}
