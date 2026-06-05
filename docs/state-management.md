# State Management Guide

owlctl maintains state in `state.json` to enable drift detection and track declarative resource management. This guide explains how state management works and best practices for using it.

> **New to owlctl?** Complete [Getting Started](getting-started.md) first — it covers installation, credentials, instance setup, and your first snapshot before returning here.

## Table of Contents

- [Overview](#overview)
- [What is State?](#what-is-state)
- [State File Location](#state-file-location)
- [State File Format](#state-file-format)
- [Instance Scoping](#instance-scoping)
- [Creating State](#creating-state)
- [State Origins](#state-origins)
- [Updating State](#updating-state)
- [Using State for Drift Detection](#using-state-for-drift-detection)
- [State in Git Workflows](#state-in-git-workflows)
- [State File Management](#state-file-management)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## Overview

State management enables:
- **Drift detection** - Compare live VBR configuration against desired state
- **Change tracking** - Know when configurations were last captured
- **Origin tracking** - Distinguish between applied and observed resources
- **GitOps workflows** - Version control your infrastructure state

## What is State?

State is owlctl's record of the last known configuration for each managed resource. When you snapshot or apply a resource, owlctl saves its configuration to `state.json`. Later, the diff command compares the current VBR configuration against this saved state to detect drift.

**Key concepts:**
- **Snapshot** - Captures current VBR configuration and saves to state (origin: `observed`)
- **Apply** - Updates VBR and automatically snapshots the new configuration (origin: `applied`)
- **Drift** - Differences between state and current VBR configuration

## State File Location

By default, `state.json` is created in the current directory. Use `OWLCTL_SETTINGS_PATH` to set a custom location:

```bash
# Set custom location
export OWLCTL_SETTINGS_PATH="$HOME/.owlctl/"

# state.json will be created at:
# $HOME/.owlctl/state.json
```

**Important:** The state file must be in the same directory as `settings.json` and `profiles.json`.

## State File Format

`state.json` (v4) organises resources by **instance**, then by **resource name**. Each instance also records which product it belongs to, enabling multi-product support (VBR, Azure, AWS, etc.) without name collisions.

```json
{
  "version": 4,
  "instances": {
    "default": {
      "product": "vbr",
      "resources": {
        "Database Backup": {
          "type": "VBRJob",
          "id": "57b3baab-6237-41bf-add7-db63d41d984c",
          "name": "Database Backup",
          "lastApplied": "2025-01-15T10:30:00Z",
          "lastAppliedBy": "admin",
          "origin": "applied",
          "spec": { ... }
        },
        "Default Backup Repository": {
          "type": "VBRRepository",
          "id": "a1b2c3d4-5678-90ab-cdef-1234567890ab",
          "name": "Default Backup Repository",
          "lastApplied": "2025-01-15T10:31:00Z",
          "lastAppliedBy": "admin",
          "origin": "observed",
          "spec": { ... }
        }
      }
    },
    "vbr-prod": {
      "product": "vbr",
      "resources": {
        "Database Backup": {
          "type": "VBRJob",
          ...
        }
      }
    }
  }
}
```

**Field definitions:**
- **version** - State file format version (current: 4)
- **instances** - Map of instance name → instance state
- **instances[name].product** - Product identifier (`vbr`, `azure`, `aws`, etc.)
- **instances[name].resources** - Map of resource name → resource
- **type** - Resource kind (e.g. `VBRJob`, `VBRRepository`, `VBRConfigurationBackup`)
- **id** - VBR resource ID (UUID)
- **name** - Resource name
- **lastApplied** - ISO 8601 timestamp of last snapshot or apply
- **lastAppliedBy** - OS username of who ran the command
- **origin** - How the resource entered state management (see below)
- **spec** - Full resource configuration as a JSON object

### Automatic migration

State files from earlier versions are migrated automatically on first load:
- **v1→v2**: `origin` field populated (`VBRJob` → `"applied"`, others → `"observed"`)
- **v2→v3**: `history` field introduced (no data migration needed)
- **v3→v4**: flat `resources` map moved into `instances["default"]`

No manual steps are required. The migrated state is written back on the next save.

## Instance Scoping

State v4 automatically scopes all resources by the active instance. This means the same resource name (e.g. `"Production Backup"`) can exist independently in multiple instances without collision.

### How it works

When you run a command without `--instance` (and no default is set), resources are stored under the `"default"` instance key. When you use `--instance <name>` or set a default with `owlctl instance set <name>`, resources are stored under that instance's key:

```bash
# Stored under instances["default"]
owlctl repo snapshot --all

# Stored under instances["vbr-prod"]
owlctl --instance vbr-prod repo snapshot --all

# Stored under instances["vbr-dr"]
owlctl --instance vbr-dr repo snapshot --all
```

### Drift detection is instance-aware

Diff commands automatically compare against the state for the active instance:

```bash
# Checks instances["vbr-prod"] state against live vbr-prod
owlctl --instance vbr-prod job diff --all

# Checks instances["vbr-dr"] state against live vbr-dr
owlctl --instance vbr-dr repo diff --all
```

> **Important**: Snapshot and diff must use the same instance. A snapshot taken with `--instance vbr-prod` is stored under `instances["vbr-prod"]`. Running `diff` without `--instance` (or with a different instance) looks in the wrong namespace and will report no baseline instead of the correct drift.
>
> The simplest way to avoid this is `owlctl instance set vbr-prod` — once set, all commands use that instance automatically and consistently.

### Multi-instance snapshot workflow

```bash
# Snapshot all instances defined in owlctl.yaml
owlctl --instance vbr-prod repo snapshot --all
owlctl --instance vbr-prod encryption kms-snapshot --all
owlctl --instance vbr-prod config-backup snapshot

owlctl --instance vbr-dr repo snapshot --all
owlctl --instance vbr-dr encryption kms-snapshot --all
owlctl --instance vbr-dr config-backup snapshot

# Commit the full state (all instances are in one state.json)
git add state.json
git commit -m "Snapshot all VBR instances"
git push
```

### Product tracking

Each instance in state records its product (`vbr`, `azure`, `aws`, etc.), set automatically when the first resource is written. This enables future tooling (such as `export --all`) to correctly organise exported files without requiring `owlctl.yaml` to be present.

## Creating State

### Snapshot Commands

Snapshot captures the current VBR configuration and saves it to state without making changes.

**Jobs:**
```bash
# Jobs are automatically snapshotted on apply
# Manual snapshot not needed for jobs
```

**Repositories:**
```bash
# Snapshot single repository
owlctl repo snapshot "Default Backup Repository"

# Snapshot all repositories
owlctl repo snapshot --all
```

**Scale-Out Repositories:**
```bash
# Snapshot single SOBR
owlctl repo sobr-snapshot "SOBR-Production"

# Snapshot all SOBRs
owlctl repo sobr-snapshot --all
```

**Encryption Passwords:**
```bash
# Snapshot single encryption password
owlctl encryption snapshot "Production Encryption Key"

# Snapshot all encryption passwords
owlctl encryption snapshot --all
```

**KMS Servers:**
```bash
# Snapshot single KMS server
owlctl encryption kms-snapshot "Azure Key Vault"

# Snapshot all KMS servers
owlctl encryption kms-snapshot --all
```

### Bringing Existing Resources Under Management

To start tracking a resource that already exists in VBR, capture it into state, then choose how to manage it:

- **Monitor only** — `snapshot` records the current configuration with `origin: observed`. Drift is detected, but owlctl never modifies the resource.
- **Manage declaratively** — `export` the resource to YAML, commit it, then `apply` it. Apply writes `origin: applied`, enabling remediation by re-applying the spec.

```bash
# Monitor an existing repository (observe only)
owlctl repo snapshot "Default Backup Repository"

# ...or bring it under declarative management
owlctl repo export "Default Backup Repository" -o repos/default.yaml
owlctl repo apply repos/default.yaml
```

> **Encryption passwords** are read-only in the VBR API — they can only be snapshotted (`observed`), never applied.

### Apply Commands

Apply automatically snapshots the configuration after successfully updating VBR.

```bash
# Apply job (automatically snapshots on success)
owlctl job apply job.yaml

# Apply repository (automatically snapshots on success)
owlctl repo apply repo.yaml

# Apply SOBR (automatically snapshots on success)
owlctl repo sobr-apply sobr.yaml

# Apply KMS server (automatically snapshots on success)
owlctl encryption kms-apply kms.yaml
```

**Note:** Apply snapshots automatically on success. Jobs can also be snapshotted directly with `owlctl job snapshot` (jobs are excluded from the global `snapshot --all`).

## State Origins

The `origin` field in state tracks how a resource entered declarative management:

| Origin | Meaning | How Created |
|--------|---------|-------------|
| `applied` | Configuration was applied from YAML | `owlctl job apply`, `owlctl repo apply`, etc. |
| `observed` | Captured by a snapshot (monitor-only) | `owlctl repo snapshot`, `owlctl encryption snapshot`, etc. |

**Origin is informational only** - it doesn't affect drift detection or other operations. It helps you understand how each resource entered state. (Re-applying an `observed` resource's exported spec promotes it to `applied`.)

## Updating State

State is automatically updated when you:

1. **Apply a configuration** - Updates state with new configuration (origin: `applied`)
2. **Take a snapshot** - Updates state with current VBR configuration (origin: `observed`)

**Manual state updates:**
To refresh state for all resources:
```bash
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all
owlctl encryption snapshot --all
owlctl encryption kms-snapshot --all
```

**Important:** Jobs are only updated via apply. You cannot manually snapshot jobs.

## Using State for Drift Detection

Drift detection compares the current VBR configuration against state:

```bash
# Detect drift for single resource
owlctl job diff "Database Backup"
owlctl repo diff "Default Backup Repository"

# Detect drift for all resources
owlctl job diff --all
owlctl repo diff --all
owlctl repo sobr-diff --all
owlctl encryption diff --all
owlctl encryption kms-diff --all
```

**How it works:**
1. owlctl reads the resource from `state.json`
2. owlctl fetches the current configuration from VBR API
3. owlctl compares the two configurations field-by-field
4. owlctl reports any differences with severity classification

**Prerequisites:**
- Resource must exist in state.json
- VBR must be accessible
- Profile must be set correctly

**If state doesn't exist:**
```bash
# Error: No state found for "Database Backup"
# Solution: Take a snapshot or apply a configuration first
owlctl repo snapshot "Default Backup Repository"
```

See [Drift Detection Guide](drift-detection.md) for complete details.

## State in Git Workflows

### Committing State to Git

**Recommended approach:**
```bash
# 1. Export all configurations to YAML
owlctl job export --all -d specs/jobs/
owlctl repo export --all -d specs/repos/
owlctl repo sobr-export --all -d specs/sobrs/
owlctl encryption kms-export --all -d specs/kms/

# 2. Snapshot all resources
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all
owlctl encryption snapshot --all
owlctl encryption kms-snapshot --all

# 3. Commit both specs and state
git add specs/ state.json
git commit -m "Snapshot VBR configuration"
git push
```

**Benefits:**
- Full audit trail of infrastructure changes
- Point-in-time recovery of configurations
- Team visibility into infrastructure state
- Foundation for GitOps workflows

### GitOps Workflow Example

**Daily snapshot automation:**
```bash
#!/bin/bash
# daily-snapshot.sh

# Take snapshots
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all
owlctl encryption snapshot --all
owlctl encryption kms-snapshot --all

# Commit if changes detected
if ! git diff --quiet state.json; then
    git add state.json
    git commit -m "Daily snapshot: $(date +%Y-%m-%d)"
    git push
fi
```

**CI/CD drift detection:**
```yaml
# Azure DevOps pipeline
- script: |
    owlctl job diff --all --security-only
    EXIT_CODE=$?
    if [ $EXIT_CODE -eq 4 ]; then
      echo "##vso[task.logissue type=error]CRITICAL drift detected"
      exit 1
    fi
  displayName: 'Detect Security Drift'
```

### State File Merge Conflicts

If multiple people update state simultaneously, Git merge conflicts can occur.

**Resolution strategy:**
```bash
# 1. Accept the version with most recent timestamps
git checkout --theirs state.json
git add state.json

# 2. Re-snapshot to ensure state is current
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all
owlctl encryption snapshot --all
owlctl encryption kms-snapshot --all

# 3. Commit the resolved state
git add state.json
git commit -m "Resolve state merge conflict with fresh snapshot"
```

**Prevention:**
- Use a single automation pipeline for state updates
- Coordinate manual state updates across team
- Consider separate state files per environment

## State File Management

### Backing Up State

**Manual backup:**
```bash
# Backup before major changes
cp state.json state.json.backup-$(date +%Y%m%d)
```

**Automated backup:**
```bash
# Keep last 7 days of state backups
#!/bin/bash
BACKUP_DIR="$HOME/.owlctl/backups"
mkdir -p "$BACKUP_DIR"

# Create backup
cp state.json "$BACKUP_DIR/state-$(date +%Y%m%d-%H%M%S).json"

# Keep only last 7 days
find "$BACKUP_DIR" -name "state-*.json" -mtime +7 -delete
```

### Restoring State

**From backup:**
```bash
# Restore from backup
cp state.json.backup-20240115 state.json
```

**From Git:**
```bash
# Restore from specific commit
git show abc123:state.json > state.json
```

**Rebuild from VBR:**
```bash
# If state is lost, rebuild from current VBR configuration
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all
owlctl encryption snapshot --all
owlctl encryption kms-snapshot --all
```

### Cleaning State

Remove resources that no longer exist in VBR:

**Manual cleanup:**
```bash
# Edit state.json and remove entries for deleted resources
vim state.json
```

**Automated cleanup (future feature):**
```bash
# Not yet implemented
# owlctl state clean --remove-missing
```

### Migrating State

**Moving to a new environment:**
```bash
# 1. Export state from old environment
cp $HOME/.owlctl/state.json state-old-env.json

# 2. Set up new environment
export OWLCTL_SETTINGS_PATH="$HOME/.owlctl-new/"
owlctl init

# 3. Take fresh snapshots in new environment
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all
owlctl encryption snapshot --all
owlctl encryption kms-snapshot --all
```

**Don't copy state between environments** - Resource IDs differ between VBR servers.

## Troubleshooting

### State File Not Found

**Problem:** owlctl can't find state.json

**Solutions:**
```bash
# Check OWLCTL_SETTINGS_PATH
echo $OWLCTL_SETTINGS_PATH

# Create state with snapshots
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all
owlctl encryption snapshot --all
owlctl encryption kms-snapshot --all
```

### Resource Not in State

**Problem:** Diff command reports "No state found for resource"

**Solution:**
```bash
# Snapshot the resource first to record it in state
owlctl repo snapshot "Resource Name"

# Then diff will work
owlctl repo diff "Resource Name"
```

### State Out of Sync

**Problem:** State doesn't match current VBR configuration

**Solution:**
```bash
# Re-snapshot to refresh state
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all
owlctl encryption snapshot --all
owlctl encryption kms-snapshot --all
```

### Corrupt State File

**Problem:** state.json is malformed or corrupt

**Solutions:**
```bash
# Validate JSON syntax
cat state.json | jq '.'

# Restore from backup
cp state.json.backup state.json

# Or rebuild from VBR
rm state.json
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all
owlctl encryption snapshot --all
owlctl encryption kms-snapshot --all
```

### State Shows Drift After Apply

**Problem:** Diff shows drift immediately after applying

**Possible causes:**
1. Some fields may not be updateable via API (VBR normalizes values)
2. VBR may add default values not in the spec
3. Timing issue (state captured before VBR fully updated)

**Solutions:**
```bash
# Re-snapshot after a short delay
sleep 5
owlctl repo snapshot "Resource Name"

# Check if drift is INFO severity (may be acceptable)
owlctl repo diff "Resource Name"
```

## Best Practices

### 1. Commit State to Git

Always version control your state file alongside configuration specs:
```bash
git add specs/ state.json
git commit -m "Update VBR configuration"
git push
```

### 2. Snapshot Regularly

Schedule regular snapshots to keep state current:
```bash
# Daily cron job
0 2 * * * cd /path/to/owlctl && ./daily-snapshot.sh
```

### 3. Snapshot Before Changes

Take a snapshot before making manual changes in VBR console:
```bash
# Before making changes in VBR console
owlctl repo snapshot --all

# Make changes in VBR console

# Check what changed
owlctl repo diff --all
```

### 4. Snapshot Existing Resources First

When starting declarative management, snapshot existing resources to seed state (origin: `observed`), then export and apply the ones you want to manage:
```bash
# Capture all existing resources into state (monitor-only baseline)
owlctl snapshot --all

# Commit initial state
git add state.json
git commit -m "Capture existing VBR resources into declarative state"
git push
```

### 5. Separate State by Instance

State v4 scopes resources by instance automatically — you don't need separate state files per environment. Define instances in `owlctl.yaml` and use `--instance`:

```bash
# Production VBR — stored under instances["vbr-prod"]
owlctl --instance vbr-prod repo snapshot --all

# DR VBR — stored under instances["vbr-dr"]
owlctl --instance vbr-dr repo snapshot --all
```

If you prefer separate state files (e.g. for isolated CI/CD runners), you can still use `OWLCTL_SETTINGS_PATH`:
```bash
export OWLCTL_SETTINGS_PATH="$HOME/.owlctl/prod/"
owlctl repo snapshot --all
```

### 6. Backup State Before Major Changes

```bash
# Before major infrastructure changes
cp state.json state.json.backup-$(date +%Y%m%d)

# Make changes
owlctl repo apply repo.yaml

# Verify
owlctl repo diff --all
```

### 7. Don't Edit State Manually

Let owlctl manage state.json. Manual edits can corrupt the file or introduce inconsistencies.

**Exception:** Removing entries for deleted resources is safe.

### 8. Keep State in Sync

If you make changes directly in VBR console, update state immediately:
```bash
# After console changes
owlctl repo snapshot "Resource Name"
owlctl job diff "Job Name"  # Verify no unexpected drift
```

### 9. Use State for Disaster Recovery

State combined with YAML specs enables full infrastructure recovery:
```bash
# 1. Store in Git
git clone https://github.com/company/vbr-config.git
cd vbr-config

# 2. Apply all configurations
for job in specs/jobs/*.yaml; do
    owlctl job apply "$job"
done

for repo in specs/repos/*.yaml; do
    owlctl repo apply "$repo"
done
```

### 10. Monitor State File Size

Large environments can produce large state files. Monitor and compress if needed:
```bash
# Check state file size
ls -lh state.json

# Compress for long-term storage
gzip < state.json > state-$(date +%Y%m%d).json.gz
```

## See Also

- [Declarative Mode Guide](declarative-mode.md) - Infrastructure-as-code workflows
- [Drift Detection Guide](drift-detection.md) - Configuration monitoring
- [Security Alerting](security-alerting.md) - Severity classification
- [Azure DevOps Integration](azure-devops-integration.md) - CI/CD automation
- [Getting Started](getting-started.md) - Initial setup
