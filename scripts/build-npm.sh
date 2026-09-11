#!/usr/bin/env bash
set -euo pipefail

# Builds the npm packages that let users run the server with `npx`.
# The main package holds a launcher; one package per platform holds a binary.
# Change SCOPE to whichever npm account or org publishes this.

cd "$(dirname "$0")/.."

SCOPE="@preaverage"
BASE="reddit-mcp"
PACKAGE_NAME="$SCOPE/$BASE"
DESCRIPTION="MCP server for Reddit that authenticates through a real browser instead of API keys"
REPOSITORY="https://github.com/preaverage/reddit-mcp"
VERSION="$(grep -oE 'version = "[^"]+"' cmd/mcp-server/main.go | head -1 | cut -d'"' -f2)"
OUT="dist/npm"

PLATFORMS=(
  "linux-x64:linux:amd64:linux:x64"
  "linux-arm64:linux:arm64:linux:arm64"
  "darwin-x64:darwin:amd64:darwin:x64"
  "darwin-arm64:darwin:arm64:darwin:arm64"
  "win32-x64:windows:amd64:win32:x64"
)

echo "building $PACKAGE_NAME v$VERSION"

rm -rf "$OUT"
mkdir -p "$OUT"

optional_deps=""

for entry in "${PLATFORMS[@]}"; do
  IFS=":" read -r target goos goarch npm_os npm_cpu <<<"$entry"

  dir="$OUT/$BASE-$target"
  binary="reddit-mcp"

  if [ "$goos" = "windows" ]; then
    binary="reddit-mcp.exe"
  fi

  mkdir -p "$dir"

  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags="-s -w" -o "$dir/$binary" ./cmd/mcp-server

  chmod 755 "$dir/$binary"

  cat > "$dir/package.json" <<JSON
{
  "name": "$PACKAGE_NAME-$target",
  "version": "$VERSION",
  "description": "$DESCRIPTION ($target binary)",
  "repository": {
    "type": "git",
    "url": "git+$REPOSITORY.git"
  },
  "license": "MIT",
  "os": ["$npm_os"],
  "cpu": ["$npm_cpu"],
  "files": ["$binary"]
}
JSON

  optional_deps="$optional_deps    \"$PACKAGE_NAME-$target\": \"$VERSION\",\n"
  echo "  $PACKAGE_NAME-$target  $(du -h "$dir/$binary" | cut -f1)"
done

main="$OUT/$BASE"
mkdir -p "$main/bin"
cp npm/cli.js "$main/bin/cli.js"
chmod 755 "$main/bin/cli.js"
cp README.md "$main/README.md"
cp LICENSE "$main/LICENSE"

cat > "$main/package.json" <<JSON
{
  "name": "$PACKAGE_NAME",
  "version": "$VERSION",
  "description": "$DESCRIPTION",
  "repository": {
    "type": "git",
    "url": "git+$REPOSITORY.git"
  },
  "license": "MIT",
  "keywords": ["mcp", "reddit", "model-context-protocol"],
  "bin": {
    "$BASE": "bin/cli.js"
  },
  "files": ["bin", "README.md", "LICENSE"],
  "engines": {
    "node": ">=18"
  },
  "optionalDependencies": {
$(printf "$optional_deps" | sed '$ s/,$//')
  }
}
JSON

echo
echo "staged in $OUT"
echo "publish with: scripts/publish-npm.sh"
