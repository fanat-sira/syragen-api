# Local publication audit

Date: 2026-10-14. This document describes local preparation, not a deployment or a GitHub release.

## Scope and results

- The target repository `fanat-sira/syragen-api` was fetched as a separate read-only mirror. It had no advertised refs and zero reachable commits. There was no existing history to scan or rewrite.
- The supplied workspace has no usable Git metadata. The publication candidate was staged in a separate temporary repository, without creating a commit. No commit, push, visibility change, or deployment was performed.
- Local `.env` credentials and administrator ID were compared against every file in the public candidate, without printing their values. No matches were found. The real `.env` remains local with mode `0600`.
- `data/` and `runtime/` were empty at inspection. Both complete directories, environment variants, private keys, backups, logs, archives, local tool directories, and build outputs are excluded from publication and Docker build context.
- Gitleaks 8.30.1 scanned the staged publication candidate with redacted output and reported no leaks. The scanner binary download was checked against the release SHA256 manifest.
- Manual source review found no real user records, private domains, IP addresses, home paths, or personal contact details in the public candidate. Syragen and the requested GitHub repository link are retained. Tests use synthetic IDs and credentials; the realistic-looking test bot token was replaced with an explicit dummy value.

## Changes

- Added an MIT license, security guidance, a clone/configure/run README, and publication checks.
- Expanded Git and Docker exclusions while preserving local deployment configuration and data.
- Added GitHub checks for tests, vet, build, Compose validation, Docker build, and staged-file/history secret scans. These workflows have not run on GitHub yet.
- Removed raw API response bodies from JSON decoding errors, since management API responses can contain new keys.
- Limited video download authorization to the configured API origin and stripped it on redirects to other origins. Added regression tests for external hosts, misleading URLs, ports, downgrade, and redirects, plus malformed key responses.

## Validation

- Baseline `go test ./...`: passed.
- Final `go test -race -p 1 ./...`: passed, including the new security tests. An earlier parallel run exceeded the environment's disk quota; the serial run succeeded after removing only the audit's own temporary cache.
- `CGO_ENABLED=0 go vet -p 1 ./...`: passed.
- Static Linux binary build from the separate clean public candidate: passed. Startup with a clean environment correctly rejected missing credentials before contacting providers.
- `docker compose --env-file .env.example config --quiet`: passed.
- Publication shell syntax, staged-file secret scan, and private-file rejection: passed.

## Verification limits

The local Go toolchain was Go 1.27.1. Docker daemon access was unavailable, so the Docker image was not built or started locally. CI includes that check for a future push. No real Telegram login, management API call, paid image/video generation, or deployed recovery test was performed. The README states the existing integration, retry, rollover, and configuration limitations without claiming live compatibility.

Before your deployment, configure your own credentials and verify `/start`, an image request, a video request, `/settings`, `/spend`, and access denial for an unauthorized Telegram account. Keep one instance per token/data directory and review any new files before a later publication.
