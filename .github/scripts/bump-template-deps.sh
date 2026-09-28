#!/usr/bin/env bash
#
# Bumps npm dependency versions declared in commands/templates/package.json_template
# to their latest published version. The file is a Go text/template, not valid JSON,
# so versions are located and replaced as plain text (npm package names never contain
# regex metacharacters we'd need to escape, but we still use literal substring
# replacement to avoid any risk of that).

set -euo pipefail

FILE="${1:-commands/templates/package.json_template}"
# Comma-separated package names to leave unchanged (empty or unset: exclude nothing).
EXCLUDE_DEPS="${EXCLUDE_DEPS-}"

changed=false

excluded() {
  local pkg="$1" item
  local IFS=','
  for item in $EXCLUDE_DEPS; do
    item="${item#"${item%%[![:space:]]*}"}"
    item="${item%"${item##*[![:space:]]}"}"
    [[ -n "$item" && "$item" == "$pkg" ]] && return 0
  done
  return 1
}

# Only scan inside the devDependencies block, so top-level fields that happen to look
# like a version string (e.g. "version": "1.0.0") are never mistaken for a package.
start=$(grep -n '"devDependencies"' "$FILE" | head -1 | cut -d: -f1)
end=$(awk -v s="$start" 'NR > s && /^[[:space:]]*}/ { print NR; exit }' "$FILE")

while IFS=: read -r lineno content; do
  name=$(sed -E 's/^[[:space:]]*"([^"]+)".*/\1/' <<<"$content")
  prefix=$(sed -E 's/.*:[[:space:]]*"([~^]?)[0-9].*/\1/' <<<"$content")
  current=$(sed -E 's/.*"[~^]?([0-9]+\.[0-9]+\.[0-9]+)".*/\1/' <<<"$content")

  if excluded "$name"; then
    echo "Skipping $name (excluded)"
    continue
  fi

  latest=$(npm view "$name" version)

  if [[ "$latest" != "$current" ]]; then
    echo "Bumping $name: $current -> $latest"
    awk -v ln="$lineno" -v old="\"${prefix}${current}\"" -v new="\"${prefix}${latest}\"" '
      NR == ln {
        idx = index($0, old)
        if (idx > 0) {
          $0 = substr($0, 1, idx - 1) new substr($0, idx + length(old))
        }
      }
      { print }
    ' "$FILE" > "$FILE.tmp"
    mv "$FILE.tmp" "$FILE"
    changed=true
  fi
done < <(grep -nE '^[[:space:]]*"[^"]+"[[:space:]]*:[[:space:]]*"[~^]?[0-9]+\.[0-9]+\.[0-9]+"' "$FILE" | awk -v s="$start" -v e="$end" -F: '$1 > s && $1 < e')

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  echo "changed=${changed}" >> "$GITHUB_OUTPUT"
fi
