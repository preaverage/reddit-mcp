#!/usr/bin/env bash
set -euo pipefail

# Publishes what build-npm.sh staged. Platform packages go first so the main
# package never points at versions that do not exist yet.

cd "$(dirname "$0")/.."

OUT="dist/npm"
BASE="reddit-mcp"
OTP="${1:-}"

# An account with two-factor publishing needs a code per package. Passing one
# in covers every publish here; without it npm prompts, which needs a terminal.
publish_args=(--access public)

if [ -n "$OTP" ]; then
  publish_args+=(--otp "$OTP")
fi

if [ ! -d "$OUT" ]; then
  echo "nothing staged; run scripts/build-npm.sh first" >&2
  exit 1
fi

# Skip anything already on the registry so a half-finished run can resume.
published() {
  npm view "$1@$2" version >/dev/null 2>&1
}

for dir in "$OUT"/*/; do
  name="$(basename "$dir")"

  if [ "$name" = "$BASE" ]; then
    continue
  fi

  full="$(python3 -c "import json,sys; print(json.load(open('$dir/package.json'))['name'])")"
  version="$(python3 -c "import json,sys; print(json.load(open('$dir/package.json'))['version'])")"

  if published "$full" "$version"; then
    echo "skipping $full@$version, already on the registry"
    continue
  fi

  echo "publishing $full@$version"
  npm publish "$dir" "${publish_args[@]}"
done

main_name="$(python3 -c "import json; print(json.load(open('$OUT/$BASE/package.json'))['name'])")"
main_version="$(python3 -c "import json; print(json.load(open('$OUT/$BASE/package.json'))['version'])")"

if published "$main_name" "$main_version"; then
  echo "skipping $main_name@$main_version, already on the registry"
  exit 0
fi

echo "publishing $main_name@$main_version"
npm publish "$OUT/$BASE" "${publish_args[@]}"
