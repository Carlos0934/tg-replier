package members

import "context"

// Tracker maintains a cache of known members in a chat.
type Tracker interface {
	// Track records a username as observed in the given chat.
	// Implementations must deduplicate by username and update last-seen time.
	Track(ctx context.Context, chatID int64, username string) error

	// List returns all known usernames for the given chat.
	List(ctx context.Context, chatID int64) ([]string, error)
}
