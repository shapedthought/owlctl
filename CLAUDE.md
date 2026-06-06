# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

owlctl is a CLI tool written in Go for interacting with Veeam APIs across multiple products (VBR, Enterprise Manager, VB365, VONE, VB for Azure/AWS/GCP).

**Key Features:**
- **Imperative Mode**: Direct API operations (GET, POST, PUT) for all Veeam products
- **Declarative Mode** (VBR only): Infrastructure-as-code management with GitOps workflows
  - Export/apply configurations for jobs, repositories, SOBRs, encryption, KMS
  - State management and drift detection with security-aware severity classification
  - Configuration overlays for multi-environment deployments
  - Named instances for multi-server automation (`--instance`)
  - Groups with specsDir for directory-based spec discovery
  - CI/CD-ready exit codes and automation-friendly output

## Building and Running

### Build from source

```bash
# Build for current platform
go build -o owlctl

# Windows
go build -o owlctl.exe
```

### First-time setup

```bash
# Initialize owlctl (creates settings.json and profiles.json)
./owlctl init

# Login to an API
./owlctl login
```

## Architecture

### Core Structure

The codebase follows a typical Cobra CLI application structure:

- **main.go**: Entry point that delegates to `cmd.Execute()`
- **cmd/**: Contains all CLI commands (login, get, post, put, profile, utils, jobs, init)
- **vhttp/**: HTTP client logic for OAuth login and API requests
- **models/**: Go structs for API requests/responses and configuration
- **utils/**: Shared utilities for file I/O, JSON/YAML handling, and path management

### Authentication Flow

owlctl reads credentials from environment variables:
- `OWLCTL_USERNAME`, `OWLCTL_PASSWORD`, `OWLCTL_URL` (default)
- `OWLCTL_{ref}_USERNAME` / `OWLCTL_{ref}_PASSWORD` when an instance uses a `credentialRef`

The login process:
1. Reads profile and settings from JSON files
2. Authenticates using OAuth or Basic Auth (Enterprise Manager)
3. Saves token to `headers.json` for subsequent requests

### Profile System

Each Veeam product has a profile in `profiles.json` containing:
- API version and headers (including `x-api-version`)
- Port and URL endpoints for authentication
- Optional username and address (for creds file mode)

Profiles: `vbr`, `ent_man`, `vb365`, `vone`, `aws`, `azure`, `gcp`

### HTTP Request Pattern

All API requests follow this pattern (see `vhttp/getData.go` and `vhttp/sendData.go`):
1. Load profile and settings
2. Build connection string from profile + endpoint
3. Add authentication header (Bearer token or session ID)
4. Execute request with optional TLS verification skip

### Job Templates System (VBR only) - Legacy

The job command (cmd/jobs.go) provides templating for VBR backup jobs:
- **Template creation**: `owlctl job template <job_id>` extracts job configuration into separate YAML files
- **Job creation**: Merges base template with specific job files to create new jobs
- Supports folder-based multi-file templates or single-file job definitions
- Base template stored in `OWLCTL_SETTINGS_PATH/job-template.yaml`

**Note:** This is legacy functionality. New development should use the declarative resource management system (see below).

### Declarative Resource Management (VBR only) - Epic #42

owlctl supports declarative, infrastructure-as-code management for VBR resources with GitOps workflows. This is the primary feature set for production use.

#### Supported Resource Types

| Resource | Export | Apply | Snapshot | Diff |
|----------|--------|-------|----------|------|
| Backup Jobs | `export <id>` | `job apply` | (implicit on apply) | `job diff` |
| Repositories | `repo export` | `repo apply` | `repo snapshot` | `repo diff` |
| Scale-Out Repos | `repo sobr-export` | `repo sobr-apply` | `repo sobr-snapshot` | `repo sobr-diff` |
| Encryption Passwords | `encryption export` | N/A (read-only) | `encryption snapshot` | `encryption diff` |
| KMS Servers | `encryption kms-export` | `encryption kms-apply` | `encryption kms-snapshot` | `encryption kms-diff` |
| Config Backup (singleton) | `config-backup export` | `config-backup apply` | `config-backup snapshot` | `config-backup diff` |

**Global commands (operate across all resource types at once):**
- `owlctl export --all [-d <dir>] [--all-instances] [--from-state]` - Export every resource type into one folder per resource type under the output dir (`--all-instances` adds an `<instance>/` level). `--from-state` generates YAML offline from `state.json` without contacting VBR.
- `owlctl snapshot --all [--all-instances]` - Snapshot every resource type (except jobs, which are declarative-managed) into `state.json` in one operation.

**State inspection commands:**
- `owlctl state list [--instance <name>]` - Table of all tracked resources (Instance, Name, Type, Origin, LastApplied)
- `owlctl state show <name>` - Full detail + spec for one resource
- `owlctl state history <name>` - Audit trail of actions (snapshotted/created/applied) with changed fields
- `owlctl state path` - Print the `state.json` path

**Context commands (kubectl-style aliases over `instance`):**
- `owlctl context list` / `current` - List instances / show active context
- `owlctl context use <name>` - Set active context; `owlctl context use -` clears it

#### Commands

**Export commands:**
- `owlctl job export <job-id>` - Export job to full YAML (300+ fields)
- `owlctl job export --all` - Export all jobs
- `owlctl job export --as-overlay` - Export minimal overlay patch
- `owlctl repo export <name>` / `--all` - Export repositories (`--as-overlay` supported)
- `owlctl repo sobr-export <name>` / `--all` - Export SOBRs (`--as-overlay` supported)
- `owlctl encryption export <name>` / `--all` - Export encryption passwords (read-only, no overlay)
- `owlctl encryption kms-export <name>` / `--all` - Export KMS servers (`--as-overlay` supported)

**Apply commands:**
- `owlctl job apply <file>` - Apply job configuration (create or update)
- `owlctl repo apply <file>` - Apply repository configuration (update-only)
- `owlctl repo sobr-apply <file>` - Apply SOBR configuration (update-only)
- `owlctl encryption kms-apply <file>` - Apply KMS server configuration (update-only)

All apply commands support:
- `-o/--overlay <file>` - Apply with overlay merge
- `--dry-run` - Preview changes without applying

**Snapshot commands:**
- `owlctl repo snapshot <name>` / `--all` - Snapshot repository state
- `owlctl repo sobr-snapshot <name>` / `--all` - Snapshot SOBR state
- `owlctl encryption snapshot <name>` / `--all` - Snapshot encryption password inventory
- `owlctl encryption kms-snapshot <name>` / `--all` - Snapshot KMS server state

**Diff commands:**
- `owlctl job diff <name>` / `--all` - Detect job drift
- `owlctl repo diff <name>` / `--all` - Detect repository drift
- `owlctl repo sobr-diff <name>` / `--all` - Detect SOBR drift
- `owlctl encryption diff <name>` / `--all` - Detect encryption password drift
- `owlctl encryption kms-diff <name>` / `--all` - Detect KMS server drift

All diff commands support:
- `--severity <level>` - Filter by minimum severity (critical, warning, info)
- `--security-only` - Show only WARNING and CRITICAL drifts

#### State Management

**State File Location:**
- `$OWLCTL_SETTINGS_PATH/state.json` or `~/.owlctl/state.json`

**State File Structure (v4 — current):**
```json
{
  "version": 4,
  "instances": {
    "default": {
      "product": "vbr",
      "resources": {
        "Resource Name": {
          "type": "VBRJob|VBRRepository|VBRSOBR|VBREncryptionPassword|VBRKmsServer",
          "id": "resource-uuid",
          "name": "Resource Name",
          "lastApplied": "2026-02-01T14:30:00Z",
          "lastAppliedBy": "username",
          "origin": "applied|observed",
          "spec": { }
        }
      }
    },
    "vbr-prod": { "product": "vbr", "resources": { } }
  }
}
```
Resources are scoped under `instances["<instance-name>"]`. When no `--instance` is active, state reads/writes use the `"default"` key. v1–v3 files with a flat `resources:` map are migrated to `instances["default"]` on first load.

**State Package (`state/`):**
- `state/manager.go` - State file management (load, save, atomic writes)
- `state/models.go` - State data structures
- `state/lock.go` - File locking for concurrent access

**State Operations:**
- State is automatically updated on successful `apply` (origin: "applied")
- State is explicitly updated on `snapshot` commands (origin: "observed")
- Each resource also carries a `history` audit trail (added in v3); inspect with `owlctl state history <name>`
- Drift detection compares state spec vs live VBR configuration
- State is NOT for audit compliance (use Git history + CI/CD logs + VBR audit logs)

#### Drift Detection System

**Core Implementation:**
- `cmd/drift.go` - Generic drift detection engine, severity classification, filtering
- `cmd/job_security.go` - Job-specific value-aware severity and cross-resource validation
- `cmd/severity_config.go` - Customizable severity overrides via `severity-config.json`

**Severity Classification:**
- **CRITICAL** - Directly weakens data protection (e.g., job disabled, encryption disabled, retention reduced)
- **WARNING** - Weakens defense-in-depth (e.g., schedule modified, guest processing disabled)
- **INFO** - Operational change with low security impact

**Value-Aware Severity:**
The system considers the *direction* of change, not just the field:
- `isDisabled: true` (job disabled) = CRITICAL
- `isDisabled: false` (job re-enabled) = WARNING
- Encryption disabled = CRITICAL, enabled = INFO
- Retention reduced = CRITICAL, increased = WARNING

**Cross-Resource Validation:**
When a job's repository changes, the system cross-references repository state to detect if the job was moved off a hardened repository (LinuxHardened type).

**Security Summary Header:**
When security-relevant drifts exist, a summary is displayed:
```
CRITICAL: 2 security-relevant changes detected
```

#### Exit Codes

**Apply Commands:**
| Code | Meaning |
|------|---------|
| `0` | Success - Applied successfully |
| `1` | Error - API failure, invalid spec |
| `5` | Partial apply - Some fields skipped (known immutable) |
| `6` | Resource not found - Cannot apply (update-only resources) |

**Diff Commands:**
| Code | Meaning |
|------|---------|
| `0` | No drift detected |
| `3` | Drift detected (INFO or WARNING severity) |
| `4` | Critical drift detected (CRITICAL severity) |
| `1` | Error occurred |

**Usage in CI/CD:**
```bash
owlctl job diff --all --security-only
EXIT_CODE=$?
if [ $EXIT_CODE -eq 4 ]; then
    echo "CRITICAL security drift detected!"
    # Alert security team
    exit 1
fi
```

#### Configuration Overlay System

**Strategic Merge:**
- Base configuration + overlay = merged configuration
- Maps are deep-merged recursively
- Arrays are replaced (overlay replaces base)
- Labels/annotations are combined

**Example:**
```bash
# Base has common settings, overlay has environment-specific overrides
owlctl job apply base-job.yaml --overlay prod-overlay.yaml

# Preview merged result
owlctl job plan base-job.yaml --overlay prod-overlay.yaml --show-yaml
```

**Overlay Resolution Priority:**
1. Explicit `-o/--overlay` flag (highest)
2. `--env` flag (planned - not yet implemented)
3. `currentEnvironment` from owlctl.yaml (planned - not yet implemented)
4. No overlay (base config only)

#### Multi-Instance Connection Management

Named instances in `owlctl.yaml` support multi-server automation with per-instance product type, credentials, token caching, and TLS settings.

**Instance Schema (in owlctl.yaml):**
```yaml
instances:
  vbr-prod:
    product: vbr
    url: vbr-prod.example.com      # hostname/IP only — no scheme or port
    port: 9419                      # optional, override product default
    insecure: true                  # optional, override global setting
    credentialRef: PROD             # reads OWLCTL_PROD_USERNAME / OWLCTL_PROD_PASSWORD
    description: Production VBR
```

**Credential Resolution:**
- If `credentialRef` set: reads `OWLCTL_{ref}_USERNAME` / `OWLCTL_{ref}_PASSWORD`
- If empty: falls back to `OWLCTL_USERNAME` / `OWLCTL_PASSWORD`

**Usage:**
```bash
# Global --instance flag
owlctl --instance vbr-prod get jobs
owlctl --instance vbr-prod login

# Instance on groups (automatic activation)
# groups:
#   prod-jobs:
#     instance: vbr-prod
#     specsDir: specs/jobs/
owlctl job apply --group prod-jobs

# Instance management commands
owlctl instance list
owlctl instance show vbr-prod
```

**Design: Env Var Activation Pattern**
- `ActivateInstance()` sets `OWLCTL_URL`, `OWLCTL_USERNAME`, `OWLCTL_PASSWORD`, `OWLCTL_KEYCHAIN_KEY` env vars
- `utils.OverrideSettings()` changes effective `SelectedProfile` and `ApiNotSecure`
- Zero changes to vhttp call sites — CLI is single-threaded, one instance active per process

**`--target` is deprecated** — prints a warning. Cannot be used with `--instance`.

**Groups with `specsDir`:**
```yaml
groups:
  auto-specs:
    specsDir: specs/jobs/    # all *.yaml files included as specs
  mixed:
    specs:
      - explicit.yaml
    specsDir: specs/more/    # combined with explicit specs
```

#### Key Files

**Commands:**
- `cmd/export.go` - Export jobs to YAML
- `cmd/export_resource.go` - Generic export infrastructure (repos, SOBRs, encryption, KMS)
- `cmd/apply.go` - Apply job configurations
- `cmd/apply_resource.go` - Generic apply infrastructure
- `cmd/drift.go` - Core drift detection engine
- `cmd/job_security.go` - Value-aware severity and cross-resource validation
- `cmd/repo.go` - Repository and SOBR management
- `cmd/encryption.go` - Encryption password and KMS server management
- `cmd/severity_config.go` - Custom severity configuration
- `cmd/instance.go` - Instance add/remove/set/get/unset/list/show commands
- `cmd/context.go` - kubectl-style `context list/use/current` (aliases over instance state)
- `cmd/group_resource.go` - Group apply/diff with instance activation + specsDir helpers
- `cmd/group.go` - Group list/show commands
- `cmd/global_export.go` / `cmd/global_snapshot.go` - `export --all` / `snapshot --all` across all resource types
- `cmd/state.go` - `state list/show/history/path` inspection commands
- `cmd/configbackup.go` - VBR config-backup singleton resource (snapshot/diff/export/apply)
- `cmd/export_registry.go` / `cmd/vbr_exports.go` - Resource-type registry driving global export/snapshot

**Configuration:**
- `config/owlctl_config.go` - VCLIConfig, InstanceConfig, GroupConfig, config loading
- `config/instance.go` - ResolvedInstance, ResolveInstance, ActivateInstance

**State Management:**
- `state/manager.go` - State file CRUD operations
- `state/models.go` - State data structures
- `state/lock.go` - Concurrent access protection

**Resources:**
- `resources/` - Resource loading and validation

**Documentation:**
- `docs/getting-started.md` - Installation, setup, first commands (start here)
- `docs/tutorial.md` - Core declarative loop walkthrough (export → edit → apply → diff)
- `docs/declarative-mode.md` - Full declarative mode guide (instances, groups, exports)
- `docs/state-management.md` - State file mechanics, instance scoping, drift detection setup
- `docs/command-reference.md` - Quick command reference
- `docs/drift-detection.md` - Comprehensive drift detection guide
- `docs/security-alerting.md` - Value-aware severity reference
- `docs/imperative-mode.md` - GET/POST/PUT operations, output formatting
- `docs/authentication.md` - Credentials, token storage, multi-product
- `docs/gitops-for-backup-infrastructure.md` - Conceptual case for GitOps (the "why")
- `docs/gitops-workflows.md` - GitHub Actions, Azure DevOps, GitLab CI
- `docs/azure-devops-integration.md` - CI/CD integration guide
- `docs/troubleshooting.md` - Common issues and fixes
- `examples/pipelines/` - Ready-to-use Azure DevOps pipeline templates

## Settings File Management

owlctl uses `OWLCTL_SETTINGS_PATH` environment variable to locate configuration files:
- If not set, files are created/read from the current directory
- Path normalization handled in `utils.SettingPath()` (adds trailing slash/backslash)
- Required files: `settings.json`, `profiles.json`, `headers.json` (created after login)

## Common Development Commands

```bash
# Build
go build -o owlctl

# Run without building
go run main.go <command>

# Imperative mode examples
./owlctl init
./owlctl profile --list
./owlctl login
./owlctl get jobs
./owlctl post jobs/57b3baab-6237-41bf-add7-db63d41d984c/start

# Declarative mode examples (VBR only)
./owlctl job export <job-id> -o job.yaml
./owlctl job apply job.yaml --dry-run
./owlctl job diff --all --security-only
./owlctl repo snapshot --all
./owlctl repo diff --all
./owlctl repo sobr-export --all -d sobrs/
./owlctl encryption kms-snapshot --all
```

## Key Implementation Details

### Generic Type Usage

The codebase uses Go generics for flexible header handling:
- `utils.ReadHeader[T]()` supports both `models.SendHeader` (OAuth) and `models.BasicAuthModel` (Enterprise Manager)
- `vhttp.GetData[T]()` allows type-safe API response unmarshaling

### API Version Handling

Different products require different API version headers:
- VBR: `x-api-version: 1.1-rev0`
- VONE: `x-api-version: 1.0-rev2`
- VB365: No x-api-version header
- See `cmd/init.go` for current version mappings

### Enterprise Manager Specifics

Enterprise Manager uses different authentication than other products:
- Basic auth instead of OAuth
- Session token in `X-RestSvcSessionId` header instead of Bearer token
- Different URL pattern (no `/api/` prefix before version)

## Important Patterns

### Adding New Commands

Follow the Cobra pattern in existing files:
1. Create command struct with `Use`, `Short`, `Long`, `Run` function
2. Register in `init()` with `rootCmd.AddCommand(yourCmd)`
3. Use `utils.GetProfile()` and `utils.ReadSettings()` for configuration
4. Call `vhttp.GetData()` or `vhttp.PostData()` for API operations

### Error Handling

Use `utils.IsErr(err)` which calls `log.Fatal()` on error. This is consistent across the codebase but note that it terminates the program immediately rather than returning errors.

### JSON/YAML Output

Commands support both JSON and YAML output formats. Default is JSON unless `--yaml` flag is used.

## Working in this Repository

### How to Work Effectively Here

**Understand the goal, not just the task.** Before implementing, ask: what outcome does this change serve? A task framed as "fix this file" is better understood as "ensure a new user can complete first-time setup without confusion." That framing surfaces adjacent problems (like a broken link three sections away) that a narrow interpretation misses.

**Think from the user's perspective.** When evaluating documentation or CLI behaviour, mentally run through it as someone who has never used the tool before. Does the flow make sense? Is there an assumption that isn't stated? Would `owlctl init` logically imply that all config files are created?

**Ask "what else is affected?" before finishing.** After completing any change, pause and ask: what else in the codebase or docs references or depends on what I just changed? A new validation rule in code may invalidate examples in three different docs. A doc change may leave another doc contradicting it.

**Play devil's advocate on your own proposals.** Before implementing a plan, consider: what's wrong with this approach? What could go wrong? What am I assuming that might not be true? This is especially important for changes to core flows like authentication, state management, or instance activation.

**Surface constraints proactively.** If you discover a constraint while working (e.g. `--url` rejects schemes, `--target` is deprecated, `TryReadSettings()` must be used before init), note it and check whether any existing code or docs violate it — don't just fix the immediate instance.

### Proactive Cross-Checks

After completing any task, consider these second-order effects before finishing:

**After code changes:**
- Do documentation examples or README snippets reference the changed behaviour? Are there flags, syntax, or URL formats that would now fail new validation?
- Do analogous commands follow the same pattern? If a bug existed in one, it likely exists in others.
- Does CLAUDE.md accurately describe the changed code (state format, command list, key files)?
- Run an incremental jscodemunch reindex so the symbol index stays current: use `mcp__jscodemunch__index_repo` with `url: shapedthought/owlctl` and `incremental: true`.

**After documentation changes:**
- Do other docs duplicate or contradict the updated content?
- Does the README still accurately reflect the new workflow?
- Are there broken links to files that no longer exist (e.g. `user_guide.md`)?

**After adding a new command or feature:**
- Does `docs/getting-started.md` need to reflect the new workflow path?
- Does `docs/command-reference.md` need a new entry?
- Are there examples elsewhere using the old/deprecated approach that should be updated?

### Session Start — Load Vault Context

At the start of every session, proactively search the Obsidian vault to load relevant context:

```bash
~/.npm-global/bin/qmd query "owlctl <topic>"
```

Key vault notes: `owlctl/architecture.md`, `owlctl/veeam-api-notes.md`, `owlctl/known-issues.md`
Vault path: `/Users/edwardhoward/Documents/obsidian vault/`
Use the `/load-owlctl` skill as a fallback if not done automatically.

### Keeping Long-Term Memory Current

The vault notes and MEMORY.md are only useful if they stay accurate. Update them when:

- **Architecture changes** — new commands, redesigned flows, new config file ownership rules → update `owlctl/architecture.md`
- **Veeam API behaviour discovered** — new quirks, undocumented constraints, corrected port/auth info → update `owlctl/veeam-api-notes.md`
- **Bugs fixed or debt introduced** — move fixed items to "Fixed (recent)" in `owlctl/known-issues.md`, add new debt entries
- **MEMORY.md becomes stale** — if a note in MEMORY.md is wrong or outdated, correct it immediately rather than working around it

After updating any vault note, re-index:
```bash
~/.npm-global/bin/qmd update && ~/.npm-global/bin/qmd embed
```

### Documentation Consistency Rules

These invariants must hold across all docs, README, and CLAUDE.md examples:

- **`--url` / `url:` in owlctl.yaml** — hostname/IP only, no scheme. `vbr-prod.example.com` ✓, `https://vbr-prod.example.com` ✗
- **`--target` is deprecated** — new examples must use `--instance` or named instances
- **`owlctl.yaml`** — created by `owlctl instance add`, not by `owlctl init`
- **`state.json`** — created by the first `snapshot` or `apply`, not by `owlctl init`
- **Profile selection** — set automatically when using instances; docs should not require `owlctl profile --set` as a prerequisite for instance-based workflows
- **State format** — v4, instance-scoped (`instances["<name>"].resources`), not flat `resources:`
- **`TryReadSettings()`** — safe to call before init (returns zero-value if file absent); use this instead of `ReadSettings()` in any code that runs before init or in recovery commands
