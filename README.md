# TG-Replyer

TG-Replyer is a small Telegram bot for group mentions.

It lets you:
- save named mention groups
- mention a saved group with one command
- mention **all known members** of the current chat with a reserved target: `all`

The bot does **not** send direct messages. It only posts mention messages inside the current Telegram chat.

## What the bot does

### Named groups
You can save groups like `devs`, `ops`, or `design` and reuse them later.

### Dynamic `all`
`all` is a special target.

It means: **all members known by the bot in the current chat**.

Because Telegram Bot API does not provide a full chat roster on demand, the bot builds this list passively from:
- regular messages
- join events

So `all` is based on the bot's local cache, not Telegram's full live member list.

## Commands

| Command | Example | Description |
|---|---|---|
| `/start` | `/start` | Show basic help |
| `/group set` | `/group set devs @alice @bob` | Create or replace a named group |
| `/group delete` | `/group delete devs` | Delete a named group |
| `/group list` | `/group list` | List saved groups |
| `/reply` | `/reply devs Standup time` | Mention a saved group or `all` in the current chat |

## Reply behavior

`/reply <target> <message>` supports:
- a saved group name
- the reserved target `all`

Examples:

```text
/reply devs Standup time
/reply all Heads up!
/reply "team alpha" "deploy in 5 minutes"
```

### Important notes

- `all` is reserved and can't be used as a group name
- mentions happen in the same chat where the command was used
- the bot only mentions users it can identify by username in its known roster or saved groups
- when `all` is incomplete, the bot warns about it

## Setup

### Requirements
- Go 1.25+
- a bot token from [@BotFather](https://t.me/BotFather)

### Run locally

```bash
git clone <repo-url>
cd tg-replyer
cp .env.example .env
# set BOT_TOKEN in .env
source .env && go run .
```

You can also export environment variables directly instead of sourcing `.env`.

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `BOT_TOKEN` | Yes | — | Telegram bot token |
| `DATA_DIR` | No | `data` | Directory for JSON storage |

## Data storage

The bot uses JSON files in `DATA_DIR` to store:
- saved mention groups
- known chat rosters for the `all` target

These runtime files are ignored by git.

## Architecture

```text
main.go
internal/
├── commands/      command parsing and use-case routing
├── config/        env configuration
├── groups/        named group domain
├── members/       known-member tracking contract
├── storage/json/  JSON persistence
└── telegram/      Telegram transport adapter
```

## Current limitations

- Telegram bots cannot fetch the full live member list of a group on demand via Bot API
- `all` only covers members known by the bot while it has been present in the chat
- users without a username are harder to mention reliably in the current model

## Project goal

Keep the bot simple, predictable, and useful for small team/group mention workflows.
