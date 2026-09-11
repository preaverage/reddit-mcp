# reddit-mcp

[![npm](https://img.shields.io/npm/v/@preaverage/reddit-mcp)](https://www.npmjs.com/package/@preaverage/reddit-mcp)
[![license](https://img.shields.io/npm/l/@preaverage/reddit-mcp)](LICENSE)

Read Reddit from any MCP client. You log in once in a real browser, and the server handles everything after that with ordinary HTTP requests. There is no API key to get and no Reddit app to register.

## Why this one

Most Reddit MCP servers ask you to register a Reddit app and paste in a client ID and secret. This one asks you to sign in.

A browser window opens, you log in, and that is the setup. Requests then come from your own account, so your home feed and your saved posts work. The browser closes when login finishes, and it only comes back if your session expires.

## Quick start

Sign in once, which opens a browser window:

```sh
npx -y @preaverage/reddit-mcp login
```

Then point Claude Code at it:

```sh
claude mcp add --scope user --transport stdio reddit -- npx -y @preaverage/reddit-mcp
```

The user scope makes it available in every project rather than only the directory you ran that in.

Any client that reads a JSON config takes the same command:

```json
{
  "mcpServers": {
    "reddit": {
      "command": "npx",
      "args": ["-y", "@preaverage/reddit-mcp"]
    }
  }
}
```

### Codex

```sh
codex mcp add reddit -- npx -y @preaverage/reddit-mcp
```

Editing `~/.codex/config.toml` by hand works too:

```toml
[mcp_servers.reddit]
command = "npx"
args = ["-y", "@preaverage/reddit-mcp"]
```

The first login downloads Camoufox, a hardened Firefox, along with the Playwright driver, so give it a minute.

### Claude Desktop

Download the `.mcpb` bundle for your platform from the releases page and open it. Claude Desktop installs it in one click, with no terminal involved, and the install dialog has a checkbox for allowing writes. Sign in afterwards through the `login` tool.

### From source

```sh
go build -o reddit-mcp ./cmd/mcp-server
./reddit-mcp login
claude mcp add --scope user reddit -- /path/to/reddit-mcp
```

## Tools

Reading Reddit:

| Tool | What it does |
| --- | --- |
| `get_me` | Your profile and karma |
| `get_user` | Anyone's public profile |
| `get_post` | One post, by URL, `t3_` fullname, or bare id |
| `get_post_comments` | A post with its comment tree |
| `get_more_comments` | Expand branches the tree cut off |
| `get_user_posts` | Someone's posts, or your own |
| `get_user_comments` | Someone's comments, or your own |
| `get_saved_posts` | Posts you saved |
| `get_subreddit_posts` | A subreddit feed, or your home feed |
| `get_subreddit` | Subreddit description and subscriber count |
| `search_posts` | Search posts, optionally inside one subreddit |

Writing to Reddit, which is off until you turn it on:

| Tool | What it does |
| --- | --- |
| `submit_post` | Create a text post or a link post in a subreddit |
| `reply` | Comment on a post, or reply to another comment |
| `edit` | Rewrite the body of your own text post or comment |
| `delete` | Permanently remove your own post or comment |

These four act publicly under your account, so the server leaves them out unless you ask for them. Without that a client cannot see them, let alone call them.

### Turning on writes

Set `REDDIT_MCP_ENABLE_WRITES` to `1` where your client keeps environment variables for the server.

Claude Code, using `-e`:

```sh
claude mcp add --scope user reddit -e REDDIT_MCP_ENABLE_WRITES=1 -- npx -y @preaverage/reddit-mcp
```

Codex, using `--env` before the `--`:

```sh
codex mcp add reddit --env REDDIT_MCP_ENABLE_WRITES=1 -- npx -y @preaverage/reddit-mcp
```

If you already added the server without it, remove and add it again.

Claude Desktop asks during installation instead. Tick "Allow posting, editing, and deleting" in the install dialog, or change it later in the extension's settings.

Clients with a JSON config take an `env` block:

```json
{
  "mcpServers": {
    "reddit": {
      "command": "npx",
      "args": ["-y", "@preaverage/reddit-mcp"],
      "env": { "REDDIT_MCP_ENABLE_WRITES": "1" }
    }
  }
}
```

The same in `~/.codex/config.toml`:

```toml
[mcp_servers.reddit.env]
REDDIT_MCP_ENABLE_WRITES = "1"
```

Running the binary yourself needs nothing special:

```sh
REDDIT_MCP_ENABLE_WRITES=1 reddit-mcp serve
```

Ask for `session_status` to see whether it took. It reports `writes_enabled`, so you can tell a missing variable apart from a client that never restarted.

Managing your login:

| Tool | What it does |
| --- | --- |
| `session_status` | Whether you are logged in and when the token expires |
| `login` | Opens the browser login flow without leaving your client |
| `logout` | Signs out on Reddit, then deletes the session and browser profile |

### Common options

| Option | Applies to | Notes |
| --- | --- | --- |
| `raw` | every tool that calls Reddit | Returns Reddit's own JSON instead of the trimmed version |
| `limit` | listings | 1 to 100, default 25 |
| `after` | listings | Cursor from the previous response |
| `sort` | listings, comments | Such as `hot`, `new`, `top` |
| `time` | `top` and `controversial` | `hour` through `all` |

Listings return an `after` cursor for the next page. Comment trees mark anything they cut off with `more_ids`, which `get_more_comments` turns back into real comments.

`reply`, `edit`, and `delete` take whatever identifies the target: a post URL, a comment URL, a bare post id, or a `t1_`/`t3_` fullname. A comment URL points at that comment rather than the post it sits under. When Reddit refuses a write, such as a rate limit or a subreddit that requires flair, its own reason comes back verbatim. Reddit reports a delete as successful whether or not the item was yours, so `delete` checks afterwards and tells you which it was.

## How it works

1. When you log in, Camoufox opens through Playwright with a profile that persists. Once your session cookies appear, the server records them along with the exact headers and user agent the browser sent.
2. Every request after that goes out through [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) using a Firefox TLS fingerprint, your cookies, and those same headers in Firefox's order. They reach `oauth.reddit.com` with the web app's own bearer token.
3. The token lasts about a day. Shortly before it expires the server loads `reddit.com` once, which is enough for Reddit to issue a fresh one. If that fails it reopens Camoufox in the background. It asks you to log in again only if the browser profile itself has been signed out.

## Command line

| Command | What it does |
| --- | --- |
| `reddit-mcp serve` | Runs the MCP server on stdio. This is the default |
| `reddit-mcp login` | Signs you in, or says who you already are. Add `-force` to sign in again |
| `reddit-mcp whoami` | Prints your profile, to check the saved session works |
| `reddit-mcp refresh` | Renews cookies in the background using the saved profile |
| `reddit-mcp logout` | Signs out on Reddit and deletes everything stored locally |

## Files

| Path | Holds |
| --- | --- |
| `~/.config/reddit-mcp/session.json` | Cookies, headers, user agent. Readable only by you |
| `~/.cache/reddit-mcp/profile` | The browser profile that stays logged in |
| `~/.cache/reddit-mcp/camoufox` | The browser itself |
| `~/.cache/ms-playwright-go` | The Playwright driver |

| Variable | Effect |
| --- | --- |
| `REDDIT_MCP_SESSION` | Keeps the session file somewhere other than the default path |
| `REDDIT_MCP_ENABLE_WRITES` | Set to `1` or `true` to offer the four tools that post, edit, and delete |

## Privacy

Your session file holds real credentials, so it is written readable only by your user account. Nothing else on disk keeps them.

No tool returns a cookie or a token, in `raw` mode or otherwise. `get_me` strips your email address, payment flags, and linked accounts before the raw response goes out. `logout` revokes the session with Reddit before deleting the local files, so the cookies stop working even if someone recovers them.

A default install can only read. The four tools that write are not registered unless you set `REDDIT_MCP_ENABLE_WRITES` to `1` or `true`, which means a client cannot post, edit, or delete by accident. Nothing votes or touches your messages either way.

Bear in mind what enabling writes means. The model can publish under your name, and `delete` cannot be undone.

## Requirements

Node 18 or newer if you install through npx. Go 1.26 or newer if you build from source.

Linux, macOS, and Windows all work, on both Intel and ARM, except Windows on ARM, which Camoufox does not build for.

## Development

```sh
go test ./...
go vet ./...
```

Publishing to npm builds one package per platform, each holding a cross-compiled binary, plus a small launcher package that picks the right one. Set the scope at the top of the build script if you publish under a different account.

```sh
scripts/build-npm.sh
scripts/publish-npm.sh
```

Desktop bundles are built the same way, one `.mcpb` per platform, ready to attach to a release. The tool list inside each manifest comes from a running server, so it cannot drift from the code.

```sh
scripts/build-mcpb.sh
```

MIT licensed.

The code is split by job. `internal/browser` drives Camoufox and captures the login, `internal/reddit` is the HTTP client and the Reddit API, `internal/session` stores what login produced, and `internal/server` exposes it all as MCP tools.
