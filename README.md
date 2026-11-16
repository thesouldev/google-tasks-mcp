<div align="center">

# google-tasks-mcp

**An MCP server for Google Tasks.**

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8.svg?logo=go&logoColor=white)](https://go.dev)
[![MCP](https://img.shields.io/badge/MCP-server-black.svg)](https://modelcontextprotocol.io)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/thesouldev/google-tasks-mcp/pulls)

</div>

---

## Overview

google-tasks-mcp is a Model Context Protocol server that lets an AI agent read
and manage Google Tasks. It works with Claude Code, Codex, Claude Desktop,
Cursor, and any other MCP client. Changes sync to the Google Tasks apps and
Google Calendar.

It signs in to one Google account of your choosing, independent of any other
Google integration in your client. The OAuth scope is tasks only, so it can read
and write tasks and nothing else: no Gmail, no Drive, no Calendar.

## Features

- Create many tasks in one call, kept in the given order
- Refer to lists by title or id
- Subtasks, due dates, notes, and moving tasks between lists
- Search by text, due date range, or completion date
- Complete, reopen, or delete several tasks at once
- Tasks only OAuth scope, with the token cached locally
- A single binary with no runtime to install

## Getting started

### Prerequisites

- A Google account that holds your tasks
- A one-time Google OAuth client, see [OAuth client](#oauth-client)

### Installation

With Homebrew on macOS or Linux:

```bash
brew install thesouldev/tap/google-tasks-mcp
```

Or download the archive for your platform from
[Releases](https://github.com/thesouldev/google-tasks-mcp/releases) and put
`google-tasks-mcp` on your PATH. With Go installed, this also works:

```bash
go install github.com/thesouldev/google-tasks-mcp/cmd/google-tasks-mcp@latest
```

Then add it to your MCP client:

```bash
# Claude Code
claude mcp add google-tasks --scope user -- google-tasks-mcp

# Codex
codex mcp add google-tasks -- google-tasks-mcp
```

For Claude Desktop, Cursor, Gemini CLI, and other clients that take JSON
config:

```json
{
  "mcpServers": {
    "google-tasks": {
      "command": "google-tasks-mcp"
    }
  }
}
```

### OAuth client

Create a Desktop OAuth client in a browser signed in to the account that holds
your tasks.

1. Create or pick a project at the [Google Cloud console](https://console.cloud.google.com).
2. Enable the [Tasks API](https://console.cloud.google.com/apis/library/tasks.googleapis.com).
3. Open Google Auth Platform, click Get started, fill in the app name, choose
   the External audience, and add your contact email.
4. Open Clients, create an OAuth client of type Desktop, and download its JSON.
5. Save it as `~/.config/google-tasks-mcp/credentials.json`.

> [!IMPORTANT]
> The Tasks scope is sensitive. While the app is in Testing, Google issues a
> refresh token that expires after 7 days. Set the publishing status to In
> production so the token persists. Unverified is fine for personal use; you
> click past one warning screen at sign in.

An existing Desktop client with the Tasks API enabled works too.

### Sign in

The first tool call opens a browser tab to sign in to Google. Approve access,
and the request continues. The token is cached at
`~/.config/google-tasks-mcp/token.json` and refreshed automatically. If it
expires or is revoked, the next tool call signs in again.

To sign in ahead of time from a terminal:

```bash
google-tasks-mcp auth
```

### Usage

Ask your agent to add, find, complete, move, or delete tasks. Several tasks in
one request are created in one call, in the order given. When no list is named,
the agent picks the closest match from your existing lists.

Google Tasks stores a due date but no time of day, so any time given stays in
the task title. Tasks with a due date show on Google Calendar as all-day
items.

## Tools

| Tool | What it does |
| --- | --- |
| `list_task_lists` | your lists, the default list, and today's date |
| `list_tasks` | tasks across lists, filtered by list, text, due range, or completion |
| `create_tasks` | one or more tasks with notes, due dates, and subtasks |
| `update_task` | title, notes, due date, or status of one task |
| `complete_tasks` | mark tasks done, or reopen them |
| `delete_tasks` | delete tasks |
| `move_task` | to another list, under a parent, or after a sibling |
| `clear_completed` | hide completed tasks in a list |
| `create_task_list` | new list |
| `rename_task_list` | rename a list |
| `delete_task_list` | delete a list and its tasks |

Lists can be passed by title or id. Destructive tools carry the MCP
`destructiveHint` annotation.

## Configuration

Everything lives under `~/.config/google-tasks-mcp/`:

| File | Purpose |
| --- | --- |
| `credentials.json` | the OAuth client you create once |
| `token.json` | the cached token, written after sign in |

| Variable | Purpose |
| --- | --- |
| `GOOGLE_TASKS_MCP_CONFIG_DIR` | use a different directory |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | OAuth client, instead of `credentials.json` |
| `GOOGLE_TASKS_MCP_NO_BROWSER` | do not open a browser; the sign-in link is returned instead |

To sign out, run `google-tasks-mcp logout` and remove the app from your
[Google account permissions](https://myaccount.google.com/permissions).

## Development

```bash
git clone git@github.com:thesouldev/google-tasks-mcp.git
cd google-tasks-mcp
go test ./...
go build ./cmd/google-tasks-mcp
```

Releases are built by GoReleaser when a `v*` tag is pushed.

## Contributing

Contributions are welcome. Open an issue to discuss substantial changes before
sending a pull request.

## License

This project is distributed under the GNU General Public License v3.0. See
[LICENSE](LICENSE) for the full text.
