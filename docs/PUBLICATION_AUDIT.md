# Local publication and reliability audit

Date: 2026-10-14. Base commit: `9c1583b`. This document describes local checks and uncommitted changes, not a successful deployment or a new GitHub Actions run.

## Public source and secrets

- The initial preparation checked an empty remote repository. The current checkout now contains the initial commit, whose reachable history was scanned with Gitleaks 8.30.1: no leaks found.
- The intended public files were exported to a separate temporary repository and staged there for publication checks. The workspace changes remain uncommitted. No push, repository visibility change, deployment, or real provider generation was performed.
- Gitleaks scans the public candidate with redacted output. Public file contents are also checked for exact matches to the private local credentials and administrator ID without printing them. No matches were found.
- `.env`, deployment data, runtime credentials/media, backups, archives, private keys, logs, and local tool directories are excluded. The local `.env` remains private; only the empty `.env.example` is published. Ignore rules do not remove old commits.
- The public archive contains source, MIT license, documentation, CI, and the empty template; no real `.env`, runtime data, or Git metadata.

## CI and runtime fixes

- CI copies `.env.example` to `.env` before Compose validation. `--env-file` alone does not supply the `.env` referenced by the service's `env_file`. The earlier local check was masked by the deployment `.env` present in the workspace; the fix was reproduced in an isolated directory with no `.env`.
- Incoming image downloads are capped at 20 MiB and video downloads at 50 MiB. JSON API responses are capped at 64 MiB and error bodies at 1 MiB. Declared lengths and streaming bodies are checked before oversized content can be saved.
- POST requests for images, videos, and key creation have no automatic retry or redirect replay, including transport/read failures, 429/503 responses, and 307/308 redirects. GET and idempotent DELETE requests retain transient retries. Manual resubmission after an ambiguous response may still duplicate a paid operation.
- Finalization persists its pending status, spend snapshot, and confirmed revocation. Failed spend recording or remote revocation retains key state and blocks new generation keys. Retries run once per minute, on the next request, and after restart, including same-period pending finalization. Only an actual HTTP 404 means a key is already absent; an error message containing `404` is insufficient.
- If a key is already absent upstream, recovery preserves a known spend snapshot without inventing a zero record for unknown spend. Failed persistence of a new key prevents serving it and attempts compensating revocation. Provider expiry remains a safeguard if remote creation succeeded but the response or durable storage was lost.
- A delayed rollover cannot revoke a valid key already created for the new period.
- Generation callbacks check the owner, chat, and original message before download/edit/video/regeneration/variation actions. The owner's edit button remains usable.
- Metadata periods and budget/reset messages use the configured timezone and spend limit.
- Earlier credential safeguards remain: raw successful API responses are excluded from decoding errors, and video authorization is restricted to the configured API origin, including redirects.

## Validation

- `go test -race -p 1 ./...`: passed, including new regression tests for POST ambiguity, pending revoke recovery, accounting failure, corrupted state, already-deleted keys, failed key persistence, delayed rollover, callback ownership, and download limits.
- `CGO_ENABLED=0 go vet -p 1 ./...`: passed.
- Static Linux binary build: passed.
- Clean-directory Compose reproduction: failed without `.env` as expected; passed after the credential-free template copy.
- `git diff --check`: passed.
- Publication script, staged-public-file scan, and reachable Git history scan: passed.

The local toolchain is Go 1.27.1. Checks use serial package execution to stay within the environment's disk quota. Docker daemon access is unavailable, so the Docker image was not built or started locally. No real Telegram login, management operation, paid generation, or live recovery was tested. GitHub CI has not been rerun for these uncommitted changes. The existing state JSON remains compatible; new recovery fields are optional.
