package commands

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"tg-replyer/internal/groups"
	"tg-replyer/internal/members"
)

// Sentinel errors for the commands domain.
var (
	ErrUnknownCommand = errors.New("unknown command")
	ErrBadArgs        = errors.New("bad arguments")
	ErrReservedName   = errors.New("reserved group name")
)

// Response carries a text reply back to the transport layer.
type Response struct {
	Text      string
	ParseMode string // "", "HTML", or "MarkdownV2"
}

// Router parses slash commands and dispatches to domain services.
// It has NO dependency on any transport package (Telegram, HTTP, etc.).
type Router struct {
	groups  *groups.Service
	tracker members.Tracker
}

// New creates a Router wired to the given domain services.
func New(groupsSvc *groups.Service, tracker members.Tracker) *Router {
	return &Router{
		groups:  groupsSvc,
		tracker: tracker,
	}
}

// Handle parses a raw slash-command string and dispatches to the
// appropriate use-case. chatID identifies the originating chat.
// Returns a Response suitable for any transport.
func (r *Router) Handle(ctx context.Context, chatID int64, text string) Response {
	parts, err := tokenize(text)
	if err != nil {
		return Response{Text: "Invalid command: " + err.Error()}
	}
	if len(parts) == 0 {
		return Response{Text: "Empty command."}
	}

	cmd := parts[0]
	switch cmd {
	case "/start":
		return r.handleStart()
	case "/group":
		return r.handleGroup(ctx, parts[1:])
	case "/reply":
		return r.handleReply(ctx, chatID, parts[1:])
	default:
		return Response{Text: fmt.Sprintf("Unknown command %q.", cmd)}
	}
}

func (r *Router) handleStart() Response {
	return Response{Text: "Welcome to TG-Replyer! Use /group and /reply to manage reply groups."}
}

func (r *Router) handleGroup(ctx context.Context, args []string) Response {
	if len(args) == 0 {
		return Response{Text: "Usage: /group set|delete|list"}
	}

	sub := args[0]
	switch sub {
	case "set":
		if len(args) < 3 {
			return Response{Text: "Usage: /group set <name> <@user1> ..."}
		}
		name := args[1]

		// Reject reserved group names.
		if strings.EqualFold(name, "all") {
			return Response{Text: fmt.Sprintf("Name %q is reserved and cannot be used as a group name.", name)}
		}

		tokens := args[2:]

		var mbrs []groups.Member
		for _, tok := range tokens {
			m, err := groups.ParseMember(tok)
			if err != nil {
				return Response{Text: fmt.Sprintf("Invalid member %q: must be an @username.", tok)}
			}
			mbrs = append(mbrs, m)
		}

		err := r.groups.Set(ctx, name, mbrs)
		if errors.Is(err, groups.ErrDuplicate) {
			return Response{Text: fmt.Sprintf("Group %q already exists.", name)}
		}
		if err != nil {
			return Response{Text: fmt.Sprintf("Error: %v", err)}
		}
		return Response{Text: fmt.Sprintf("Group %q created with %d member(s).", name, len(mbrs))}

	case "delete":
		if len(args) < 2 {
			return Response{Text: "Usage: /group delete <name>"}
		}
		name := args[1]
		err := r.groups.Delete(ctx, name)
		if errors.Is(err, groups.ErrNotFound) {
			return Response{Text: fmt.Sprintf("Group %q not found.", name)}
		}
		if err != nil {
			return Response{Text: fmt.Sprintf("Error: %v", err)}
		}
		return Response{Text: fmt.Sprintf("Group %q deleted.", name)}

	case "list":
		list, err := r.groups.List(ctx)
		if err != nil {
			return Response{Text: fmt.Sprintf("Error: %v", err)}
		}
		if len(list) == 0 {
			return Response{Text: "No groups defined."}
		}
		sort.Slice(list, func(i, j int) bool {
			return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
		})
		var sb strings.Builder
		for _, g := range list {
			names := make([]string, len(g.Members))
			for i, m := range g.Members {
				names[i] = m.DisplayName()
			}
			fmt.Fprintf(&sb, "- <b>%s</b>: %s\n", g.Name, strings.Join(names, ", "))
		}
		return Response{Text: sb.String(), ParseMode: "HTML"}

	default:
		return Response{Text: "Unknown sub-command. Usage: /group set|delete|list"}
	}
}

func (r *Router) handleReply(ctx context.Context, chatID int64, args []string) Response {
	if len(args) < 2 {
		return Response{Text: "Usage: /reply <target> <message>"}
	}

	target := args[0]
	message := strings.Join(args[1:], " ")

	// Check for reserved "all" keyword (case-insensitive).
	if strings.EqualFold(target, "all") {
		return r.handleReplyAll(ctx, chatID, message)
	}

	// Named group resolution.
	return r.handleReplyGroup(ctx, target, message)
}

// handleReplyAll resolves the "all" target from the known-member roster.
func (r *Router) handleReplyAll(ctx context.Context, chatID int64, message string) Response {
	known, err := r.tracker.List(ctx, chatID)
	if err != nil {
		return Response{Text: fmt.Sprintf("Error: %v", err)}
	}
	if len(known) == 0 {
		return Response{Text: "No members known yet in this chat. Members are tracked as they send messages."}
	}

	mentions := buildMentions(known)
	text := mentions + " " + message + "\n(roster may be incomplete \u2014 only known members mentioned)"
	return Response{Text: text}
}

// handleReplyGroup resolves a named group target.
func (r *Router) handleReplyGroup(ctx context.Context, groupName string, message string) Response {
	list, err := r.groups.List(ctx)
	if err != nil {
		return Response{Text: fmt.Sprintf("Error: %v", err)}
	}

	var target *groups.Group
	for i := range list {
		if list[i].Name == groupName {
			target = &list[i]
			break
		}
	}
	if target == nil {
		return Response{Text: fmt.Sprintf("Unknown target %q. Use a group name or \"all\".", groupName)}
	}
	if len(target.Members) == 0 {
		return Response{Text: fmt.Sprintf("Group %q is empty.", groupName)}
	}

	usernames := make([]string, len(target.Members))
	for i, m := range target.Members {
		usernames[i] = m.DisplayName()
	}
	return Response{Text: buildMentions(usernames) + " " + message}
}

// buildMentions joins usernames into a "@user1 @user2" string.
// Usernames that already start with @ are kept as-is; others get @ prepended.
func buildMentions(usernames []string) string {
	parts := make([]string, len(usernames))
	for i, u := range usernames {
		if strings.HasPrefix(u, "@") {
			parts[i] = u
		} else {
			parts[i] = "@" + u
		}
	}
	return strings.Join(parts, " ")
}
