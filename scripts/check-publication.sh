#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
command -v gitleaks >/dev/null || { echo 'Install Gitleaks 8.30.1 or newer first.' >&2; exit 1; }

# Inspect the index, not the working tree: local .env and deployment data stay private.
stage_dir=$(mktemp -d)
trap 'rm -rf "$stage_dir"' EXIT
blocked=0
while IFS= read -r -d '' path; do
  case "$path" in
    .env.example) ;;
    .env|.env.*|*/.env|*/.env.*|.envrc|*/.envrc|data/*|runtime/*|backups/*|.git/*|.aws/*|.codex/*|.agents/*|dist/*|*.pem|*.key|*.p12|*.pfx|*.dump|*.sql|*.bak|*.zip|*.tar|*.tar.gz|*.tgz|*.7z|*.log|*id_rsa*|*id_ed25519*)
      printf 'Private/generated file in Git index: %s\n' "$path" >&2
      blocked=1 ;;
  esac
done < <(git ls-files --cached -z)
if [[ "$blocked" != 0 ]]; then exit 1; fi
if [[ -z "$(git ls-files --cached)" ]]; then
  echo 'No files in Git index; stage the intended public files before checking.' >&2
  exit 1
fi
git checkout-index --all --prefix="$stage_dir/"
gitleaks dir "$stage_dir" --redact --no-banner
if git rev-parse --verify HEAD >/dev/null 2>&1; then
  gitleaks git . --log-opts=--all --redact --no-banner
else
  echo 'No commits yet; Git history scan skipped.'
fi
echo 'Publication scan passed. Review remaining personal identifiers manually.'
