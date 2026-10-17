# Drift Detection Guide

owlctl provides drift detection across multiple VBR resource types. It compares the current live VBR configuration against a saved state snapshot to identify manual changes or unauthorized modifications.

## Supported Resource Types

| Resource Type | Snapshot Command | Diff Command |
|---------------|-----------------|--------------|
| Backup Jobs | `owlctl job apply` | `owlctl job diff` |
| Repositories | `owlctl repo snapshot` | `owlctl repo diff` |
| Scale-Out Repositories | `owlctl repo sobr-snapshot` | `owlctl repo sobr-diff` |
| Encryption Passwords | `owlctl encryption snapshot` | `owlctl encryption diff` |
| KMS Servers | `owlctl encryption kms-snapshot` | `owlctl encryption kms-diff` |
| Configuration Backup | `owlctl config-backup snapshot` | `owlctl config-backup diff` |

## How It Works

1. **Snapshot**: Capture the current VBR configuration into state (`state.json`)
2. **Drift Detection**: Compare the saved state against the live VBR configuration
3. **Classification**: Each drift is assigned a severity level (CRITICAL, WARNING, or INFO)
4. **Filtering**: Optionally filter output to show only security-relevant changes

### State File

State is stored in `state.json` in `$OWLCTL_SETTINGS_PATH`, or the current directory when unset (next to `settings.json`). Resources are scoped by instance so multiple VBR servers don't collide.

```json
{
  "version": 4,
  "instances": {
    "default": {
      "product": "vbr",
      "resources": {
        "Backup Job 1": {
          "type": "VBRJob",
          "id": "c07c7ea3-...",
          "name": "Backup Job 1",
          "lastApplied": "2026-02-01T14:30:00Z",
          "lastAppliedBy": "edwardhoward",
          "origin": "applied",
          "spec": { }
        }
      }
    }
  }
}
```

Commands run without `--instance` use the `"default"` instance. Named instances (e.g., `--instance vbr-prod`) use their own namespace.

> **Note**: State files are operational tools for drift detection, not compliance-grade audit logs. For compliance, use Git commit history + CI/CD logs + VBR audit logs.

## Job Drift Detection

### Single Job

```bash
owlctl job diff "Backup Job 1"
```

### All Jobs

```bash
owlctl job diff --all
```

### Example Output

```
Checking drift for job: Backup Job 1

CRITICAL: 2 security-relevant changes detected

Drift detected:
  CRITICAL ~ isDisabled: false (state) -> true (VBR)
  CRITICAL ~ storage.retentionPolicy.quantity: 3 (state) -> 1 (VBR)
  INFO ~ schedule.runAutomatically: false (state) -> true (VBR)

Summary:
  - 3 drifts detected
  - Highest severity: CRITICAL
  - Last applied: 2026-02-01 17:17:25
  - Last applied by: edwardhoward
```

## Repository Drift Detection

### Snapshot Repositories

```bash
# Single repository
owlctl repo snapshot "Default Backup Repository"

# All repositories
owlctl repo snapshot --all
```

### Detect Drift

```bash
# Single repository
owlctl repo diff "Default Backup Repository"

# All repositories
owlctl repo diff --all
```

### Example: Immutability Drift Detection

Repository immutability changes are critical security events. owlctl detects these with CRITICAL severity:

```bash
owlctl repo diff "Hard1"
```

**Output:**
```
Checking drift for repository: Hard1 (observed)

CRITICAL: 1 security-relevant changes detected

Drift detected:
  CRITICAL ~ repository.makeRecentBackupsImmutableDays: 7 (state) -> 5 (VBR)

Summary:
  - 1 drifts detected
  - Highest severity: CRITICAL
  - Exit code: 4
```

This ensures immutability reductions (e.g., 7 days → 5 days) trigger alerts in CI/CD pipelines and are visible with `--security-only` filters.

## Scale-Out Backup Repository (SOBR) Drift Detection

```bash
# Snapshot
owlctl repo sobr-snapshot --all

# Detect drift
owlctl repo sobr-diff --all
```

### Example: SOBR Encryption Drift Detection

SOBR encryption changes are critical security events. owlctl detects capacity tier encryption changes with CRITICAL severity:

```bash
owlctl repo sobr-diff "Scale-out Backup Repository 1"
```

**Output:**
```
Checking drift for scale-out repository: Scale-out Backup Repository 1 (observed)

CRITICAL: 1 security-relevant changes detected

Drift detected:
  CRITICAL ~ capacityTier.encryption.isEnabled: true (state) -> false (VBR)
  INFO ~ capacityTier.encryption.encryptionPasswordId: Removed from VBR

Summary:
  - 2 drifts detected
  - Highest severity: CRITICAL
  - Exit code: 4
```

Encryption disabled = CRITICAL. Encryption enabled = INFO (positive security change).

## Encryption Password Drift Detection

```bash
# Snapshot
owlctl encryption snapshot --all

# Detect drift
owlctl encryption diff --all
```

## KMS Server Drift Detection

```bash
# Snapshot
owlctl encryption kms-snapshot --all

# Detect drift
owlctl encryption kms-diff --all
```

## Configuration Backup Drift Detection

Configuration backup is a singleton resource (one set of settings per VBR server) — no name or `--all` flag is needed.

```bash
# Snapshot current configuration backup settings
owlctl config-backup snapshot

# Detect drift
owlctl config-backup diff

# Filter by severity
owlctl config-backup diff --severity warning
owlctl config-backup diff --security-only
```

Configuration backup drift uses its own severity map:

| Field | Severity |
|-------|----------|
| `isEnabled` | CRITICAL |
| `encryption.isEnabled` | CRITICAL |
| `backupRepositoryId` | WARNING |
| `encryption.passwordId` | WARNING |
| `restorePointsToKeep` | WARNING |
| `schedule` | INFO |
| `notifications` | INFO |

## Severity Classification

Every drift is classified by security impact:

| Level | Meaning | Action |
|-------|---------|--------|
| **CRITICAL** | Directly weakens data protection or ransomware resilience | Immediate investigation |
| **WARNING** | Weakens defense-in-depth or monitoring | Investigate within 24 hours |
| **INFO** | Operational change with low direct security impact | Review during next audit |

See [Security Alerting](security-alerting.md) for the full severity reference and value-aware classification details.

## Severity Filtering

### Show Only Critical Drifts

```bash
owlctl job diff --all --severity critical
```

### Show Security-Relevant Drifts (WARNING and above)

```bash
owlctl job diff --all --security-only
```

These flags work on all diff commands (`job diff`, `repo diff`, `repo sobr-diff`, `encryption diff`, `encryption kms-diff`).

## Custom Severity Configuration

Override default severity levels by placing a `severity-config.json` in `$OWLCTL_SETTINGS_PATH` or `~/.owlctl/`:

```json
{
  "job": {
    "isDisabled": "CRITICAL",
    "schedule": "WARNING"
  },
  "repository": {
    "type": "CRITICAL"
  },
  "sobr": {
    "isEnabled": "CRITICAL"
  },
  "encryption": {
    "hint": "WARNING"
  },
  "kms": {
    "type": "CRITICAL"
  }
}
```

Both short field names (`isDisabled`) and full dotted paths (`storage.retentionPolicy.quantity`) are supported.

## Exit Codes

All commands return structured exit codes for CI/CD integration:

### Diff Commands

| Code | Meaning |
|------|---------|
| `0` | No drift detected |
| `3` | Drift detected (INFO or WARNING severity) |
| `4` | Critical drift detected |
| `1` | Error occurred |

### Apply Commands

| Code | Meaning |
|------|---------|
| `0` | Applied successfully |
| `1` | Error occurred (API failure, invalid spec) |
| `5` | Partial apply — some resources succeeded, some failed (batch operations) |
| `6` | Resource not found — cannot apply (update-only resources like repos, SOBRs, KMS) |

### CI/CD Example (Diff)

```bash
#!/bin/bash
owlctl job diff --all --security-only
EXIT_CODE=$?

if [ $EXIT_CODE -eq 4 ]; then
    echo "CRITICAL security drift detected!"
    # Send alert to security team
    curl -X POST "$SLACK_WEBHOOK" \
      -d '{"text":"CRITICAL: VBR security drift detected. Immediate investigation required."}'
    exit 1
elif [ $EXIT_CODE -eq 3 ]; then
    echo "WARNING: Security drift detected, review required."
fi
```

### CI/CD Example (Apply)

```bash
#!/bin/bash
# Single resource apply
owlctl repo apply repos/default-repo.yaml
EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
    echo "Repository applied successfully"
elif [ $EXIT_CODE -eq 6 ]; then
    echo "Resource not found - must be created in VBR console first"
    exit 1
else
    echo "Apply failed with error"
    exit 1
fi
```

```bash
#!/bin/bash
# Batch apply (multiple files) - exit code 5 possible
for spec in repos/*.yaml; do
    owlctl repo apply "$spec"
    RESULT=$?
    if [ $RESULT -ne 0 ]; then
        FAILED=$((FAILED + 1))
    else
        SUCCESS=$((SUCCESS + 1))
    fi
done

if [ $FAILED -gt 0 ] && [ $SUCCESS -gt 0 ]; then
    echo "Partial success: $SUCCESS applied, $FAILED failed"
    exit 5  # Partial apply
elif [ $FAILED -gt 0 ]; then
    echo "All applies failed"
    exit 1
fi
```

## Dry-Run Mode

All apply commands support `--dry-run` to preview changes without making any modifications:

```bash
# Preview repository changes
owlctl repo apply repos/default-repo.yaml --dry-run

# Preview SOBR changes
owlctl repo sobr-apply sobrs/sobr1.yaml --dry-run

# Preview KMS server changes
owlctl encryption kms-apply kms/my-kms.yaml --dry-run
```

### Example Output

```
=== Dry Run Mode ===
Resource: Default Backup Repository (VBRRepository)
Action: Would UPDATE existing resource

Changes that would be applied:
  INFO ~ description: "Created by Veeam Backup" -> "Updated description"
  INFO ~ repository.maxTaskCount: 4 -> 8
  CRITICAL ~ repository.makeRecentBackupsImmutableDays: 7 -> 14

3 field(s) would be changed.

=== End Dry Run ===
No changes made. Remove --dry-run flag to apply.
```

`job apply --dry-run` and every other apply command (`repo apply`, `repo sobr-apply`, `encryption kms-apply`, `config-backup apply`) print the same format: `  <SEVERITY> <~|+|-> <path>: <old> -> <new>`, where `~` is a changed value, `+` a value being set where there was none, and `-` a value being unset. Severity comes from the same map as `diff` (INFO where a field has none). Values are never truncated. When nothing differs, every command prints `No changes detected. Resource is already in desired state.`

### Machine-readable output

Add `--output json` to a single-file `--dry-run` to get one JSON document on stdout (banners and warnings go to stderr):

```bash
owlctl job apply job.yaml --dry-run --output json
```

```json
{
  "resource": "DB Backup",
  "kind": "VBRJob",
  "action": "would-update",
  "changes": [
    {"path": "description", "old": "a", "new": "b", "severity": "INFO"}
  ],
  "skipped": [],
  "postApply": "POST jobs/<id>/disable (isDisabled: false -> true)"
}
```

`action` is `would-create`, `would-update` or `no-change`. `--output json` requires `--dry-run` and is not supported with `--group`. Dry-run exits `0` whether or not changes are pending; use `job diff` / `repo diff` for drift exit codes (`3`/`4`).

### CI/CD Validation Stage

Use dry-run for safe PR validation before applying changes:

```yaml
# Azure DevOps Pipeline Example
stages:
  - stage: Validate
    jobs:
      - job: DryRun
        steps:
          - script: |
              for spec in repos/*.yaml; do
                ./owlctl repo apply "$spec" --dry-run
                if [ $? -ne 0 ]; then
                  echo "Validation failed for $spec"
                  exit 1
                fi
              done
            displayName: 'Preview Changes (Dry Run)'

  - stage: Apply
    dependsOn: Validate
    condition: succeeded()
    jobs:
      - job: ApplyChanges
        steps:
          - script: ./owlctl repo apply repos/*.yaml
            displayName: 'Apply Configuration'
```

### Dry-Run Behavior

- **Read-only API calls**: Dry-run fetches current state from VBR to compute changes, but makes no modifications (no PUT/POST)
- **No state updates**: State file is not modified in dry-run mode
- **Same exit codes**: Returns the same exit codes as regular apply (e.g., `6` if resource not found)
- **Full change preview**: Shows exactly what would change if applied

## Ignored Fields

Each resource type has fields that are excluded from drift detection because they are read-only or frequently changing (e.g., `lastRun`, `nextRun`, `statistics`, `id`). These are defined internally and cannot be overridden.

## Known-Immutable Fields and Remediation Config

owlctl automatically detects and skips fields that are known to be rejected by the VBR API. Additionally, you can configure custom field policies using `remediation-config.yaml`.

### Known-Immutable Fields

Some VBR resource fields cannot be changed via the API after creation. owlctl includes built-in hints for these fields:

| Resource | Field | Reason |
|----------|-------|--------|
| Repository | `type` | Repository type cannot be changed after creation |
| Repository | `path` | Changing path requires storage migration |
| Repository | `host.id` | Host cannot be changed |
| SOBR | `performanceTier.extents` | Extents must be managed in VBR console |
| KMS Server | `type` | KMS server type cannot be changed after creation |

When apply encounters drift in a known-immutable field, it skips sending that change and shows a human-readable explanation instead of a raw API error.

### Custom remediation-config.yaml

Create a `remediation-config.yaml` file to define custom field policies. See [examples/remediation-config.yaml](../examples/remediation-config.yaml) for a complete example.

**Locations checked (in order):**
1. `$OWLCTL_SETTINGS_PATH/remediation-config.yaml`
2. `~/.owlctl/remediation-config.yaml`

```yaml
# remediation-config.yaml
repository:
  maxTaskCount: remediable    # Normal field - attempt to apply (default)
  description: remediable
  path: skip                  # Skip - don't attempt, known to fail
  type: skip                  # Skip - don't attempt, known to fail

sobr:
  performanceTier.extents: skip

job:
  storage.backupRepositoryId: skip  # Org policy: repo changes need manual review

kms:
  type: skip
```

### Field Policies

| Policy | Behavior |
|--------|----------|
| `remediable` | Send change to VBR (default for all fields) |
| `skip` | Don't send change to VBR; preserve existing value |

### Policy Priority

1. **User config** (`remediation-config.yaml`) - highest priority
2. **Built-in known-immutable hints**
3. **Default** (`remediable`) - lowest priority

User config can override built-in hints. For example, if testing shows a field is actually mutable in your VBR version, set it to `remediable` to override the hint.

### Example Output with Skipped Fields

```
Applying: Default Backup Repository

  Applied: description: "Original" -> "Updated description"
  Applied: repository.maxTaskCount: 4 -> 8

Skipped fields (known immutable or policy-configured):
  - type
    Repository type cannot be changed after creation. Recreate the repository in VBR console.
  - path
    Changing path requires storage migration. Perform in VBR console.

2 field(s) applied.
```

In dry-run mode, skipped fields are shown separately:

```
=== Dry Run Mode ===
Resource: Default Backup Repository (VBRRepository)
Action: Would UPDATE existing resource

Changes that would be applied:
  INFO ~ description: "Original" -> "Updated description"
  INFO ~ repository.maxTaskCount: 4 -> 8

2 field(s) would be changed.

Skipped (not sent to VBR):
  - type
    Repository type cannot be changed after creation. Recreate the repository in VBR console.

=== End Dry Run ===
No changes made. Remove --dry-run flag to apply.
```

### Missing Config File

If no `remediation-config.yaml` exists, owlctl uses defaults:
- Built-in known-immutable hints apply
- All other fields are `remediable`

This is not an error — remediation config is optional.
