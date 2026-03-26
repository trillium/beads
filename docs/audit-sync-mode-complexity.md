# Audit: Sync Mode Complexity in Beads

**Wanted Item:** w-bd-004
**Date:** 2026-03-04
**Status:** Complete — all recommendations implemented

## Executive Summary

Beads' sync subsystem was significantly simplified through refactoring in v0.50-v0.61. The old multi-mode architecture (git-portable, belt-and-suspenders, dolt-native) has been fully collapsed to Dolt-native sync. All vestigial sync mode scaffolding has been removed.

### What Was Removed (v0.50-v0.61)

- `SyncMode` type, `SyncTrigger` type, `GetSyncMode()`, `SyncConfig` struct
- `sync.go` sync mode validation functions and tests
- `sync.mode` and `sync.git-remote` config keys (no longer in yaml-only keys list)
- `validateSyncConfig` renamed to `validateFederationConfig` (validates federation/remote config only)
- Stale comments referencing `sync.mode=dolt-native` across config and test files
- Old `bd sync` command (replaced by `bd dolt push`/`bd dolt pull`)
- SQLite backend, JSONL sync, 3-way merge, tombstones, storage factory, daemon stubs
- Dead git-portable sync functions
- JSONL sync-branch pipeline (~11,000 lines)

## Remaining Architecture (Active, Well-Structured)

### Push/Pull Routing (Justified)

**File:** `internal/storage/dolt/store.go`

Each of `Push()`, `ForcePush()`, and `Pull()` has a 3-way routing decision:
1. **Git-protocol remote** (SSH, git+https://) → shell out to `dolt push/pull` CLI
2. **Hosted Dolt with remoteUser** → `CALL DOLT_PUSH('--user', ...)` via SQL
3. **Default** (DoltHub, S3, GCS, file) → `CALL DOLT_PUSH(?, ?)` via SQL

This routing is necessary — the three paths exist because Dolt has genuinely different authentication mechanisms. The optional refactor to extract a `execDoltRemoteOp` helper remains a minor improvement opportunity.

### Federation Peer System (Active, Well-Structured)

**Files:** `internal/storage/dolt/federation.go`, `internal/storage/dolt/credentials.go`

Peer-to-peer sync, credential management (AES-GCM), sync status tracking. No changes needed.

### Conflict Resolution Configuration (Active, Clean)

**File:** `internal/config/sync.go`

Four conflict strategies (`newest`, `ours`, `theirs`, `manual`) and four field-level strategies. Used by federation `Sync()`. No changes needed.

### Sovereignty Tiers (Active, Clean)

**File:** `internal/config/sync.go`

Four sovereignty tiers (T1-T4) for federation access control. No changes needed.

### Tracker SyncEngine (Active, Good Abstraction)

**File:** `internal/tracker/engine.go`

Shared sync engine for external trackers (Linear, GitLab, Jira). Separate from Dolt sync. No changes needed.

## Historical Timeline

- **v0.50.3**: Tracker sync code unified via shared SyncEngine (~800 lines removed)
- **v0.51.0**: SQLite backend, JSONL sync, 3-way merge, tombstones removed. `bd sync` replaced by `bd dolt push`/`bd dolt pull`
- **v0.52.0**: Dead git-portable sync functions removed (#1793)
- **v0.53.0**: JSONL sync-branch pipeline removed (~11,000 lines)
- **v0.55.0**: Dead sync mode scaffolding removed (SyncMode type, validation, config keys)
- **v0.60.0**: Auto-increment reset workaround removed via UUID PK migration
- **v0.61.0**: Final cleanup — vestigial `sync.mode` references removed from config code, comments, and docs
