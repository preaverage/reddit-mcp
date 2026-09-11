#!/usr/bin/env bash
set -euo pipefail

# Builds .mcpb bundles, the single-click install format for Claude Desktop.
# A bundle carries one platform's binary, so this produces one file each.
# https://github.com/modelcontextprotocol/mcpb

cd "$(dirname "$0")/.."

NAME="reddit-mcp"
DISPLAY_NAME="Reddit"
DESCRIPTION="Read Reddit through a session you create by logging in with a real browser. No API key required."
AUTHOR="preaverage"
REPOSITORY="https://github.com/preaverage/reddit-mcp"
VERSION="$(grep -oE 'version = "[^"]+"' cmd/mcp-server/main.go | head -1 | cut -d'"' -f2)"
OUT="dist/mcpb"
WORK="$OUT/.work"

PLATFORMS=(
  "linux-x64:linux:amd64:linux"
  "linux-arm64:linux:arm64:linux"
  "darwin-x64:darwin:amd64:darwin"
  "darwin-arm64:darwin:arm64:darwin"
  "win32-x64:windows:amd64:win32"
)

echo "building $NAME v$VERSION bundles"

rm -rf "$OUT"
mkdir -p "$OUT"

# The tool list comes from a running server so the manifest cannot drift.
probe="$WORK/probe"
mkdir -p "$WORK"
go build -trimpath -o "$probe" ./cmd/mcp-server
# Listed with writes on, so the manifest shows everything the bundle can do.
# Whether they are offered at runtime depends on the install-time checkbox.
TOOLS="$(REDDIT_MCP_ENABLE_WRITES=1 python3 scripts/tools-json.py "$probe")"

for entry in "${PLATFORMS[@]}"; do
  IFS=":" read -r target goos goarch platform <<<"$entry"

  stage="$WORK/$target"
  binary="reddit-mcp"

  if [ "$goos" = "windows" ]; then
    binary="reddit-mcp.exe"
  fi

  mkdir -p "$stage/server"

  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags="-s -w" -o "$stage/server/$binary" ./cmd/mcp-server

  chmod 755 "$stage/server/$binary"
  cp README.md LICENSE "$stage/"

  # Claude appends .exe on Windows, so the command never carries it.
  cat > "$stage/manifest.json" <<JSON
{
  "manifest_version": "0.3",
  "name": "$NAME",
  "display_name": "$DISPLAY_NAME",
  "version": "$VERSION",
  "description": "$DESCRIPTION",
  "author": {
    "name": "$AUTHOR",
    "url": "$REPOSITORY"
  },
  "homepage": "$REPOSITORY",
  "repository": {
    "type": "git",
    "url": "$REPOSITORY"
  },
  "license": "MIT",
  "keywords": ["reddit", "mcp"],
  "server": {
    "type": "binary",
    "entry_point": "server/$binary",
    "mcp_config": {
      "command": "\${__dirname}/server/reddit-mcp",
      "args": ["serve"],
      "env": {
        "REDDIT_MCP_ENABLE_WRITES": "\${user_config.enable_writes}"
      }
    }
  },
  "tools": $TOOLS,
  "tools_generated": false,
  "user_config": {
    "enable_writes": {
      "type": "boolean",
      "title": "Allow posting, editing, and deleting",
      "description": "Lets the model submit posts, reply, edit, and delete under your Reddit account. Leave this off to keep the server read only.",
      "required": false,
      "default": false
    }
  },
  "compatibility": {
    "claude_desktop": ">=0.10.0",
    "platforms": ["$platform"]
  }
}
JSON

  python3 -c "import json,sys; json.load(open('$stage/manifest.json'))"

  bundle="$PWD/$OUT/$NAME-$VERSION-$target.mcpb"
  (cd "$stage" && zip -qr "$bundle" .)

  echo "  $(basename "$bundle")  $(du -h "$bundle" | cut -f1)"
done

rm -rf "$WORK"

echo
echo "bundles in $OUT"
echo "install one by opening it in Claude Desktop"
