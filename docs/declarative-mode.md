# Declarative Mode Guide

Declarative mode enables infrastructure-as-code workflows for Veeam Backup & Replication. Define configurations in YAML, track in Git, deploy to multiple environments, and detect drift.

## Table of Contents

- [Overview](#overview)
- [When to Use Declarative Mode](#when-to-use-declarative-mode)
- [Resource Types](#resource-types)
- [Key Concepts](#key-concepts)
- [Export Resources](#export-resources)
- [Apply Configurations](#apply-configurations)
- [State Management](#state-management)
- [Drift Detection](#drift-detection)
- [Configuration Overlays](#configuration-overlays)
- [Groups](#groups)
- [Instances](#instances)
- [Targets (Deprecated)](#targets-deprecated)
- [Strategic Merge Behavior](#strategic-merge-behavior)
- [Multi-Instance Workflow](#multi-instance-workflow)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

Declarative mode allows you to:
- Define backup infrastructure as YAML configuration files
- Organize specs into groups with shared profiles and overlays
- Target multiple VBR servers from a single configuration
- Track configurations in version control (Git)
- Detect configuration drift from desired state
- Automate deployments with CI/CD pipelines

**When to use declarative mode:**
- Infrastructure-as-code workflows
- Group-based deployments across multiple VBR servers
- Drift detection and monitoring
- GitOps automation
- Configuration standardization
- Disaster recovery planning

**When to use imperative mode:**
- Quick API queries
- Triggering one-off operations (start backup, stop job)
- Exploring API capabilities
- Working with products without declarative support

See [Imperative Mode Guide](imperative-mode.md) for imperative workflows.

## When to Use Declarative Mode

### Use Declarative Mode When:

✅ **Multi-environment deployments**
Deploy the same backup job to dev, staging, and production with environment-specific settings.

✅ **Configuration drift detection**
Monitor for unauthorized changes to backup infrastructure.

✅ **GitOps workflows**
Track all infrastructure changes in Git with full audit trail.

✅ **Standardization and compliance**
Enforce consistent backup policies across the organization.

✅ **Disaster recovery**
Quickly rebuild backup infrastructure from version-controlled configuration.

### Use Imperative Mode When:

❌ One-off operations (start/stop jobs)

❌ Quick API exploration

❌ Immediate troubleshooting

❌ Products without declarative support (VB365, VONE, Enterprise Manager)

## Resource Types

owlctl supports declarative management for these VBR resource types:

| Resource | Commands | Create Support | Notes |
|----------|----------|----------------|-------|
| **Jobs** | `export`, `apply`, `diff` | ✅ Yes | Full create/update support |
| **Repositories** | `repo export`, `repo apply`, `repo snapshot`, `repo diff` | ❌ No | Update-only (create in VBR console) |
| **Scale-Out Repositories (SOBRs)** | `repo sobr-export`, `repo sobr-apply`, `repo sobr-snapshot`, `repo sobr-diff` | ❌ No | Update-only (create in VBR console) |
| **Encryption Passwords** | `encryption export`, `encryption snapshot`, `encryption diff` | ❌ No | Read-only (password values never exposed) |
| **KMS Servers** | `encryption kms-export`, `encryption kms-apply`, `encryption kms-snapshot`, `encryption kms-diff` | ❌ No | Update-only (create in VBR console) |
| **Configuration Backup** | `config-backup export`, `config-backup apply`, `config-backup snapshot`, `config-backup diff` | N/A | Singleton resource — one set of settings per server |

## Key Concepts

### 1. Specs (Base Configuration)
A spec YAML file defines the identity and settings of a single resource (job, repository, SOBR, etc.). Specs use resource kinds like `VBRJob`, `VBRRepository`, `VBRSOBR`, or `VBRKmsServer`.

### 2. Overlays
Overlay files (`kind: Overlay`) contain policy-level changes that merge on top of a spec. They have the highest priority in the merge order.

### 3. Profiles
Profile files (`kind: Profile`) define organizational defaults (retention standards, compression settings, schedule patterns). They provide a foundation that specs build upon. Profiles have the lowest priority in the merge order.

### 4. Groups
A group bundles multiple specs with a shared profile and overlay. Defined in `owlctl.yaml`, groups enable batch apply and drift detection with a single command (`--group`).

### 5. Instances
Instances define named server connections in `owlctl.yaml` with product type, URL, credentials, and TLS settings. Use `--instance` on any command to target a specific server. Groups can reference an instance directly.

### 5b. Targets (Deprecated)
Targets are the legacy version of instances. They only set the URL. Use instances instead for full credential and product type support.

### 6. Strategic Merge
owlctl uses strategic merge to combine layers. The merge order is:
1. **Profile** (defaults) — lowest priority
2. **Spec** (identity and exceptions)
3. **Overlay** (policy patch) — highest priority

Maps are merged recursively, arrays are replaced, and labels are combined across all layers.

### 7. State Management
owlctl maintains state in `state.json` to track:
- Last known configuration of each resource
- When snapshots were taken
- Configuration origin (applied vs observed)

### 8. Drift Detection
Compare live VBR configuration against desired state to detect unauthorized changes with security-aware severity classification.

## Export Resources

Export existing VBR resources to YAML format for declarative management.

### Jobs

```bash
# Export single job by ID
owlctl job export 57b3baab-6237-41bf-add7-db63d41d984c -o my-job.yaml

# Export single job by name
owlctl job export "Database Backup" -o my-job.yaml

# Export all jobs to directory
owlctl job export --all -d ./jobs/

# Export as overlay (minimal changes only)
owlctl job export <id> --as-overlay -o overlay.yaml

# Export as overlay with base comparison
owlctl job export <id> --as-overlay --base base-job.yaml -o overlay.yaml

# Legacy simplified format (20-30 fields)
owlctl job export <id> --simplified -o job.yaml
```

**Full export (default):** Captures all VBR API fields (300+) ensuring complete job fidelity.

**Overlay export:** Exports minimal overlay format containing only fields that differ from base.

**Simplified export (legacy):** Minimal configuration (20-30 fields) for basic jobs only.

### Repositories

```bash
# Export single repository by name
owlctl repo export "Default Backup Repository" -o repo.yaml

# Export all repositories
owlctl repo export --all -d ./repos/

# Export as overlay (minimal diff from base)
owlctl repo export "Default Backup Repository" --as-overlay --base base-repo.yaml -o overlay.yaml
```

### Scale-Out Repositories (SOBRs)

```bash
# Export single SOBR by name
owlctl repo sobr-export "SOBR-Production" -o sobr.yaml

# Export all SOBRs
owlctl repo sobr-export --all -d ./sobrs/

# Export as overlay
owlctl repo sobr-export "SOBR-Production" --as-overlay --base base-sobr.yaml -o overlay.yaml
```

### Encryption Passwords

```bash
# Export single encryption password by name
owlctl encryption export "Production Encryption Key" -o enc.yaml

# Export all encryption passwords
owlctl encryption export --all -d ./encryption/
```

**Note:** Password values are never exported. Only metadata (name, description, hint) is captured. Overlay export is not supported for encryption passwords.

### KMS Servers

```bash
# Export single KMS server by name
owlctl encryption kms-export "Azure Key Vault" -o kms.yaml

# Export all KMS servers
owlctl encryption kms-export --all -d ./kms/

# Export as overlay
owlctl encryption kms-export "Azure Key Vault" --as-overlay --base base-kms.yaml -o overlay.yaml
```

## Apply Configurations

Apply YAML configurations to VBR with optional dry-run preview.

### Jobs (Create or Update)

```bash
# Apply single job
owlctl job apply my-job.yaml

# Preview changes without applying (recommended)
owlctl job apply my-job.yaml --dry-run

# Apply with overlay
owlctl job apply base-job.yaml --overlay prod-overlay.yaml

# Apply all specs in a group (profile + spec + overlay merge)
owlctl job apply --group sql-tier
```

**Note:** Jobs support both creation (POST) and updates (PUT).

### Repositories (Update-Only)

```bash
# Apply repository configuration
owlctl repo apply repo.yaml

# Preview changes
owlctl repo apply repo.yaml --dry-run

# Apply with overlay
owlctl repo apply base-repo.yaml --overlay prod-repo-overlay.yaml
```

**Important:** Repositories must be created in VBR console first. Apply only updates existing repositories.

### Scale-Out Repositories (Update-Only)

```bash
# Apply SOBR configuration
owlctl repo sobr-apply sobr.yaml

# Preview changes
owlctl repo sobr-apply sobr.yaml --dry-run
```

**Important:** SOBRs must be created in VBR console first. Apply only updates existing SOBRs.

### KMS Servers (Update-Only)

```bash
# Apply KMS server configuration
owlctl encryption kms-apply kms.yaml

# Preview changes
owlctl encryption kms-apply kms.yaml --dry-run
```

**Important:** KMS servers must be created in VBR console first. Apply only updates existing KMS servers.

### Exit Codes

Apply commands return specific exit codes for automation:

| Code | Meaning | Action |
|------|---------|--------|
| `0` | Success | Continue |
| `1` | Error (API failure, invalid spec) | Fix and retry |
| `5` | Partial apply (some fields skipped) | Review skipped fields |
| `6` | Resource not found (update-only) | Create in VBR console first |

**Example usage in scripts:**
```bash
owlctl repo apply repo.yaml
if [ $? -eq 6 ]; then
    echo "Repository doesn't exist - create it in VBR console first"
    exit 1
fi
```

## State Management

owlctl records the configuration of each managed resource in `state.json`. Applying a resource snapshots it automatically (`origin: applied`); `snapshot` captures a resource for monitoring without changing it (`origin: observed`). Drift detection compares live VBR against this recorded state.

State is scoped per instance (state v4), so the same resource name can exist independently across instances without collision.

See the **[State Management Guide](state-management.md)** for the full reference: the `state.json` format, the complete snapshot command list, instance scoping, origins, and automatic migration. The [Command Reference](command-reference.md) lists every snapshot command.

## Drift Detection

`diff` compares the live VBR configuration against your recorded state and classifies each change by security impact — **CRITICAL** (exit `4`), **WARNING**/**INFO** (exit `3`), or no drift (exit `0`). Filter with `--severity <level>` or `--security-only`.

```bash
owlctl job diff --all --security-only       # security-relevant drift across all jobs
owlctl repo diff "Default Backup Repository"
owlctl config-backup diff
```

See **[Drift Detection](drift-detection.md)** for the severity model and filtering, **[Security Alerting](security-alerting.md)** for value-aware severity rules, and the **[Command Reference](command-reference.md)** for every diff command and its exit codes.

## Configuration Overlays

Overlays enable multi-environment deployments from a single base template.

### Creating Overlays

Overlays contain only the fields you want to override from the base configuration.

**Example Base Configuration:**
```yaml
# base-backup.yaml
apiVersion: owlctl.veeam.com/v1
kind: VBRJob
metadata:
  name: database-backup
  labels:
    app: database
    managed-by: owlctl
spec:
  type: VSphereBackup
  description: Database backup job
  repository: default-repo
  storage:
    compression: Optimal
    retention:
      type: Days
      quantity: 7
  schedule:
    enabled: true
    daily: "22:00"
    retry:
      enabled: true
      times: 3
      wait: 10
  objects:
    - type: VirtualMachine
      name: db-server
      hostName: 192.168.0.14
```

**Production Overlay:**
```yaml
# overlays/prod-overlay.yaml
apiVersion: owlctl.veeam.com/v1
kind: VBRJob
metadata:
  name: database-backup
  labels:
    env: production
spec:
  description: Production database backup (30-day retention)
  repository: prod-repo
  storage:
    retention:
      quantity: 30  # Override: 7 days -> 30 days
  schedule:
    daily: "02:00"  # Override: 22:00 -> 02:00
```

**Development Overlay:**
```yaml
# overlays/dev-overlay.yaml
apiVersion: owlctl.veeam.com/v1
kind: VBRJob
metadata:
  name: database-backup
  labels:
    env: development
spec:
  description: Development database backup (3-day retention)
  repository: dev-repo
  storage:
    retention:
      quantity: 3   # Override: 7 days -> 3 days
  schedule:
    daily: "23:00"  # Override: 22:00 -> 23:00
```

### Preview Merged Configuration

Use the `plan` command to preview the merged configuration:

```bash
# Preview base configuration (no overlay)
owlctl job plan base-backup.yaml

# Preview with production overlay
owlctl job plan base-backup.yaml --overlay overlays/prod-overlay.yaml

# Preview with development overlay
owlctl job plan base-backup.yaml --overlay overlays/dev-overlay.yaml

# Show full merged YAML
owlctl job plan base-backup.yaml --overlay overlays/prod-overlay.yaml --show-yaml
```

**Output:**
The plan command displays:
- Resource name and type
- Base configuration file
- Overlay file (if used)
- Labels (merged from base + overlay)
- Key configuration fields
- Storage settings
- Schedule settings
- Backup objects list

### Apply with Overlay

```bash
# Preview changes (recommended)
owlctl job apply base-backup.yaml --overlay overlays/prod-overlay.yaml --dry-run

# Apply to production
owlctl job apply base-backup.yaml --overlay overlays/prod-overlay.yaml

# Apply to development
owlctl job apply base-backup.yaml --overlay overlays/dev-overlay.yaml
```

## Groups

Groups bundle multiple specs with a shared profile, overlay, and optional instance, enabling batch apply and drift detection with a single command. Groups are defined in `owlctl.yaml`.

### owlctl.yaml Schema

```yaml
# owlctl.yaml
apiVersion: owlctl.veeam.com/v1
kind: Config

instances:
  vbr-prod:
    product: vbr
    url: https://vbr-prod.example.com
    credentialRef: PROD                  # reads OWLCTL_PROD_USERNAME / OWLCTL_PROD_PASSWORD
    description: Production VBR server
  vbr-dr:
    product: vbr
    url: https://vbr-dr.example.com
    credentialRef: DR
    description: Disaster recovery site

groups:
  sql-tier:
    description: SQL Server backup group
    instance: vbr-prod                   # target this group at vbr-prod
    profile: profiles/gold.yaml          # kind: Profile — base defaults
    overlay: overlays/compliance.yaml    # kind: Overlay — policy patch
    specs:
      - specs/jobs/sql-vm-01.yaml
      - specs/jobs/sql-vm-02.yaml

  web-tier:
    description: Web server backups
    instance: vbr-prod
    profile: profiles/standard.yaml
    specsDir: specs/web-jobs/            # all *.yaml files in this directory

  dr-jobs:
    description: DR site backup jobs
    instance: vbr-dr
    specsDir: specs/dr-jobs/
```

### Group Fields

| Field | Description |
|-------|-------------|
| `description` | Human-readable description |
| `instance` | Named instance to target (from `instances:` section) |
| `profile` | Path to Profile YAML (base defaults, lowest merge priority) |
| `overlay` | Path to Overlay YAML (policy patch, highest merge priority) |
| `specs` | Explicit list of spec file paths |
| `specsDir` | Directory path; all `*.yaml` files are included as specs |

If both `specs` and `specsDir` are set, they are combined (explicit specs first, then directory contents).

### Profile vs Overlay

| | Profile (`kind: Profile`) | Overlay (`kind: Overlay`) |
|---|---|---|
| **Purpose** | Organizational defaults | Policy patch / customization |
| **Merge priority** | Lowest (foundation) | Highest (wins over all) |
| **Use case** | Retention standards, compression, schedule patterns | Environment-specific overrides, compliance tweaks |

### 3-Way Merge Order

When a group is applied, owlctl merges three layers:

1. **Profile** — provides base defaults (lowest priority)
2. **Spec** — defines resource identity and exceptions
3. **Overlay** — applies policy patches (highest priority)

```
Profile (defaults)  →  Spec (identity)  →  Overlay (policy)  =  Final Config
```

Maps are deep-merged at each step. Arrays are replaced. Labels are combined across all layers. `metadata.name` always comes from the spec.

### Configuration File Locations

owlctl searches for `owlctl.yaml` in this order:
1. Path in `OWLCTL_CONFIG` environment variable
2. Current directory (`./owlctl.yaml`)
3. Home directory (`~/.owlctl/owlctl.yaml`)

File paths within `owlctl.yaml` (specs, profiles, overlays) are resolved relative to the directory containing `owlctl.yaml`.

### Group Commands

```bash
# List all groups
owlctl group list

# Show group details (resolved paths, spec count)
owlctl group show sql-tier
```

### Apply with --group

Apply all specs in a group in one command. Each spec is merged with the group's profile and overlay before being sent to VBR.

```bash
# Dry-run first (recommended)
owlctl job apply --group sql-tier --dry-run

# Apply
owlctl job apply --group sql-tier
```

`--group` is available on `job apply` and `job diff`. Under `owlctl job`, group operations currently support only `VBRJob` specs; groups containing other resource kinds will fail validation.

### Diff with --group

Check drift for all specs in a group against live VBR state. Group diff does **not** require `state.json` — the group definition (profile + spec + overlay) is the source of truth.

```bash
owlctl job diff --group sql-tier
```

### Mutual Exclusivity

`--group` cannot be combined with:
- Positional file arguments
- `-o/--overlay`
- `--env`
- `--all`

### Overlay-Only Apply (No Group)

You can still apply a single file with an overlay, without using groups:

```bash
owlctl job apply base-job.yaml --overlay prod-overlay.yaml
```

This is useful for one-off operations or simple setups that don't need the full group model.

## Instances

Instances define named server connections with product type, URL, credentials, and TLS settings. They enable multi-server automation from a single `owlctl.yaml`.

### Creating Instances (Recommended)

Use `owlctl instance add` to register instances without hand-editing YAML:

```bash
owlctl instance add vbr-prod \
  --url vbr-prod.example.com \
  --product vbr \
  --credential-ref PROD \
  --description "Production VBR server"

owlctl instance add vbr-dr \
  --url vbr-dr.example.com \
  --product vbr \
  --credential-ref DR \
  --description "Disaster recovery site"
```

This creates or updates `owlctl.yaml` and prints which environment variables to set for credentials. Use `--force` to overwrite an existing instance.

### Instance Schema (Reference)

The equivalent YAML written to `owlctl.yaml`:

```yaml
instances:
  vbr-prod:
    product: vbr                         # Required: vbr, ent_man, vb365, vone, aws, azure, gcp
    url: vbr-prod.example.com            # Required: server address
    port: 9419                           # Optional: override product default port
    insecure: true                       # Optional: override global TLS setting
    credentialRef: PROD                  # Optional: env var prefix for credentials
    description: Production VBR server   # Optional: human-readable description
  vbr-dr:
    product: vbr
    url: vbr-dr.example.com
    credentialRef: DR
    description: Disaster recovery site
```

### Credential Resolution

When `credentialRef` is set, credentials are read from prefixed environment variables:
- `OWLCTL_{ref}_USERNAME` (e.g., `OWLCTL_PROD_USERNAME`)
- `OWLCTL_{ref}_PASSWORD` (e.g., `OWLCTL_PROD_PASSWORD`)

When `credentialRef` is not set, falls back to `OWLCTL_USERNAME` / `OWLCTL_PASSWORD`.

### Instance Commands

```bash
# Add an instance
owlctl instance add vbr-prod --url vbr-prod.example.com --product vbr --credential-ref PROD

# Remove an instance (also clears DefaultInstance in settings.json if it was the default)
owlctl instance remove vbr-prod

# Set a default — no --instance flag needed on subsequent commands
owlctl instance set vbr-prod

# Show the current default
owlctl instance get

# Clear the default
owlctl instance unset

# List all instances (* marks the default)
owlctl instance list

# Show instance details
owlctl instance show vbr-prod
```

### Default instance

`owlctl instance set` persists the active instance to `settings.json`. Once set, all commands use it automatically — no `--instance` flag needed on every invocation. The `--instance` flag still takes precedence when explicitly provided.

```bash
owlctl instance set vbr-prod
owlctl login                      # uses vbr-prod
owlctl repo snapshot --all        # uses vbr-prod
owlctl job diff --all             # uses vbr-prod

owlctl --instance vbr-dr job diff --all   # override for this command only
```

### Using --instance

The `--instance` flag is a persistent flag available on all commands. It activates the instance's URL, credentials, product type, and per-instance token caching.

```bash
# Login to a specific instance
owlctl --instance vbr-prod login

# Run any command against an instance
owlctl --instance vbr-prod get jobs
owlctl --instance vbr-dr get jobs

# Apply group to a specific instance
owlctl job apply --group sql-tier --instance vbr-prod
owlctl job apply --group sql-tier --instance vbr-dr
```

### Instance on Groups

Groups can specify an `instance` field, eliminating the need for `--instance` on every command:

```yaml
groups:
  prod-jobs:
    instance: vbr-prod
    specsDir: specs/prod-jobs/
```

```bash
# Instance is activated automatically from the group definition
owlctl job apply --group prod-jobs
owlctl job diff --group prod-jobs
```

### Per-Instance Token Caching

Each instance gets its own keychain entry (e.g., `instance:vbr-prod`, `instance:vbr-dr`). This prevents token collisions when multiple instances share the same product type.

## Targets (Deprecated)

> **Deprecated:** Use [Instances](#instances) instead. Targets only set the URL; instances also handle product type, credentials, per-instance token caching, and TLS settings.

Targets define named VBR server connections for URL-only switching.

### Target Schema

```yaml
# In owlctl.yaml (deprecated — migrate to instances)
targets:
  primary:
    url: https://vbr-prod.example.com
    description: Production VBR server
  dr:
    url: https://vbr-dr.example.com
    description: Disaster recovery site
```

### Using --target

```bash
# These work but print a deprecation warning
owlctl job apply --group sql-tier --target primary
owlctl job diff --group sql-tier --target dr
```

`--target` and `--instance` cannot be used together.

## Strategic Merge Behavior

Understanding how overlays merge with base configurations:

### Scalar Values

Overlay value replaces base value.

```yaml
# Base
quantity: 7

# Overlay
quantity: 30

# Result
quantity: 30
```

### Nested Objects (Maps)

Deep merge - overlays are applied recursively.

```yaml
# Base
storage:
  compression: Optimal
  retention:
    type: Days
    quantity: 7

# Overlay
storage:
  retention:
    quantity: 30

# Result
storage:
  compression: Optimal      # Preserved from base
  retention:
    type: Days              # Preserved from base
    quantity: 30            # Overridden
```

### Arrays

Overlay array replaces base array completely.

```yaml
# Base
objects:
  - name: vm1
  - name: vm2

# Overlay
objects:
  - name: vm3

# Result
objects:
  - name: vm3              # Base array replaced
```

**Important:** Arrays are replaced, not merged. Include all desired items in the overlay array.

### Labels and Annotations

Combined (merged) from base and overlay.

```yaml
# Base
labels:
  app: database
  managed-by: owlctl

# Overlay
labels:
  env: production

# Result
labels:
  app: database           # From base
  managed-by: owlctl        # From base
  env: production         # From overlay
```

## Multi-Instance Workflow

### Project Structure

```
vbr-infrastructure/
├── owlctl.yaml                    # Instances, groups
├── profiles/
│   ├── gold.yaml                  # kind: Profile — high retention, encryption
│   └── standard.yaml              # kind: Profile — standard defaults
├── specs/
│   ├── jobs/
│   │   ├── sql-vm-01.yaml         # kind: VBRJob
│   │   └── sql-vm-02.yaml
│   ├── web-jobs/                  # specsDir target
│   │   ├── web-frontend.yaml
│   │   └── web-api.yaml
│   └── repos/
│       └── production-repo.yaml   # kind: VBRRepository
├── overlays/
│   └── compliance.yaml            # kind: Overlay — policy patch
└── state.json
```

### Complete Workflow Example

```bash
# 1. Export existing jobs as specs
owlctl job export --all -d specs/jobs/

# 2. Create a profile with organizational defaults
# profiles/gold.yaml (kind: Profile)

# 3. Create an overlay for policy overrides
# overlays/compliance.yaml (kind: Overlay)

# 4. Define instances and groups in owlctl.yaml
cat > owlctl.yaml <<'EOF'
apiVersion: owlctl.veeam.com/v1
kind: Config

instances:
  vbr-prod:
    product: vbr
    url: https://vbr-prod.example.com
    credentialRef: PROD
    description: Production VBR
  vbr-dr:
    product: vbr
    url: https://vbr-dr.example.com
    credentialRef: DR
    description: DR site

groups:
  sql-tier:
    description: SQL Server backup group
    instance: vbr-prod
    profile: profiles/gold.yaml
    overlay: overlays/compliance.yaml
    specs:
      - specs/jobs/sql-vm-01.yaml
      - specs/jobs/sql-vm-02.yaml

  sql-tier-dr:
    description: SQL Server backup group (DR)
    instance: vbr-dr
    profile: profiles/gold.yaml
    overlay: overlays/compliance.yaml
    specs:
      - specs/jobs/sql-vm-01.yaml
      - specs/jobs/sql-vm-02.yaml

  web-tier:
    description: Web server backups
    instance: vbr-prod
    profile: profiles/standard.yaml
    specsDir: specs/web-jobs/
EOF

# 5. Set up credentials
export OWLCTL_PROD_USERNAME="admin" OWLCTL_PROD_PASSWORD="prod-pass"
export OWLCTL_DR_USERNAME="admin" OWLCTL_DR_PASSWORD="dr-pass"

# 6. Preview the group
owlctl group show sql-tier

# 7. Dry-run apply
owlctl job apply --group sql-tier --dry-run

# 8. Apply to production (instance activated from group)
owlctl job apply --group sql-tier

# 9. Apply same specs to DR
owlctl job apply --group sql-tier-dr

# 10. Drift check
owlctl job diff --group sql-tier
owlctl job diff --group sql-tier-dr

# 11. Or use --instance to override
owlctl --instance vbr-prod get jobs
owlctl --instance vbr-dr get jobs

# 12. Commit to version control
git add .
git commit -m "Add instance-based backup configuration"
git push
```

### Simpler Alternative: Single-File Overlay

For simpler setups that don't need groups, use the `-o` flag directly:

```bash
owlctl job apply base-backup.yaml --overlay overlays/prod.yaml --dry-run
owlctl job apply base-backup.yaml --overlay overlays/prod.yaml
```

### Bootstrap Declarative Management

Start managing existing VBR infrastructure declaratively:

```bash
# 1. Export current VBR state
owlctl job export --all -d specs/jobs/
owlctl repo export --all -d specs/repos/
owlctl repo sobr-export --all -d specs/sobrs/
owlctl encryption kms-export --all -d specs/kms/

# 2. Snapshot current state
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all
owlctl encryption snapshot --all
owlctl encryption kms-snapshot --all

# 3. Commit to Git (state.json and specs)
git add specs/ state.json
git commit -m "Bootstrap VBR declarative management"
git push

# 4. Verify no drift
owlctl job diff --all
owlctl repo diff --all
owlctl repo sobr-diff --all
owlctl encryption diff --all
owlctl encryption kms-diff --all
```

## Best Practices

### 1. Keep Base Templates DRY
Only include common settings in base files. Environment-specific values go in overlays.

**Good:**
```yaml
# base-backup.yaml - common settings only
spec:
  type: VSphereBackup
  storage:
    compression: Optimal
```

**Bad:**
```yaml
# base-backup.yaml - don't include environment-specific values
spec:
  repository: prod-repo  # This varies by environment!
```

### 2. Small Overlays
Override only what differs per environment.

**Good:**
```yaml
# prod-overlay.yaml
spec:
  repository: prod-repo
  storage:
    retention:
      quantity: 30
```

**Bad:**
```yaml
# prod-overlay.yaml - duplicates too much from base
spec:
  type: VSphereBackup  # Unnecessary - same as base
  storage:
    compression: Optimal  # Unnecessary - same as base
    retention:
      type: Days  # Unnecessary - same as base
      quantity: 30  # This is the only change needed
```

### 3. Use Labels
Tag resources with environment, app, and managed-by labels.

```yaml
metadata:
  labels:
    app: database
    env: production
    managed-by: owlctl
    team: infrastructure
```

### 4. Version Control
Commit both base and overlays to Git.

```bash
git add base-backup.yaml overlays/ owlctl.yaml
git commit -m "Update production retention to 30 days"
git push
```

### 5. Preview First
Always use plan or --dry-run before applying.

```bash
# Preview merged configuration
owlctl job plan base-backup.yaml --overlay prod-overlay.yaml --show-yaml

# Dry-run apply
owlctl job apply base-backup.yaml --overlay prod-overlay.yaml --dry-run

# Apply for real
owlctl job apply base-backup.yaml --overlay prod-overlay.yaml
```

### 6. Consistent Naming
Use clear, descriptive names for base files and overlays.

**Good:**
- `base-database-backup.yaml`
- `overlays/prod-database.yaml`
- `overlays/dev-database.yaml`

**Bad:**
- `backup1.yaml`
- `overlay.yaml`
- `prod.yaml`

### 7. Document Changes
Use Git commit messages to explain configuration changes.

```bash
git commit -m "Increase production retention to 30 days to meet compliance requirements"
```

### 8. Test in Dev First
Apply to a dev instance before production.

```bash
# 1. Test in development
owlctl job apply --group sql-tier --instance vbr-dev --dry-run
owlctl job apply --group sql-tier --instance vbr-dev

# 2. Verify in development
# Run test backup, verify results

# 3. Apply to production
owlctl job apply --group sql-tier --instance vbr-prod --dry-run
owlctl job apply --group sql-tier --instance vbr-prod
```

### 9. Regular Drift Scans
Schedule drift detection to catch unauthorized changes.

```bash
# Daily drift scan
owlctl job diff --all --security-only
owlctl repo diff --all --security-only
owlctl repo sobr-diff --all --security-only
owlctl encryption kms-diff --all --security-only
```

### 10. Snapshot Before Changes
Snapshot resources before making changes.

```bash
# Before applying changes
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all

# Apply changes
owlctl repo apply repo.yaml
owlctl repo sobr-apply sobr.yaml

# Verify no unexpected drift
owlctl repo diff --all
owlctl repo sobr-diff --all
```

## Troubleshooting

### Overlay Not Being Applied

**Problem:** Overlay seems to be ignored.

**Solutions:**
1. When using `--group`, verify the group's overlay path in `owlctl.yaml`:
   ```bash
   owlctl group show <name>  # Shows resolved overlay path
   ```
2. When using `-o` directly, verify the overlay file path exists
3. Verify owlctl.yaml is in the search path:
   - Check `OWLCTL_CONFIG` environment variable
   - Check current directory (`./owlctl.yaml`)
   - Check home directory (`~/.owlctl/owlctl.yaml`)
4. Use `--show-yaml` to see the actual merged result:
   ```bash
   owlctl job plan base.yaml --overlay overlay.yaml --show-yaml
   ```

### Unexpected Merge Results

**Problem:** Merged configuration doesn't look right.

**Solutions:**
1. Remember: arrays are replaced, not merged
   ```yaml
   # If your overlay has an objects array, it replaces the entire base array
   ```
2. Use `--show-yaml` to see full merged result:
   ```bash
   owlctl job plan base.yaml --overlay overlay.yaml --show-yaml
   ```
3. Check that overlay `kind` matches base `kind`:
   ```yaml
   # Base and overlay must both be: kind: VBRJob
   ```

### Labels Not Combining

**Problem:** Labels from base not appearing in merged config.

**Solutions:**
1. Ensure both base and overlay use `metadata.labels` field
2. Labels should be at same level in both files:
   ```yaml
   # Both files should have:
   metadata:
     labels:
       key: value
   ```

### Resource Not Found (Exit Code 6)

**Problem:** Apply fails with "resource not found" error.

**Solution:**
Repositories, SOBRs, and KMS servers are update-only. Create them in VBR console first:
```bash
# This will fail if repository doesn't exist
owlctl repo apply repo.yaml  # Exit code 6

# Create the repository in VBR console first, then:
owlctl repo apply repo.yaml  # Now succeeds
```

### Drift Detected After Apply

**Problem:** Diff shows drift immediately after applying.

**Solutions:**
1. Some fields may not be updateable via API (skipped during apply)
2. Check for fields with exit code 5 (partial apply)
3. VBR may normalize certain values (e.g., schedule times)
4. Review the specific drifts - may be INFO severity and acceptable

### State File Out of Sync

**Problem:** State file doesn't match VBR.

**Solution:**
Re-snapshot resources to refresh state:
```bash
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all
owlctl encryption snapshot --all
owlctl encryption kms-snapshot --all
```

### Encryption Password Apply Not Supported

**Problem:** Can't find `encryption apply` command.

**Explanation:**
Encryption passwords are read-only. Password values are never exposed by the VBR API, so updates aren't supported. Only metadata (name, description, hint) can be exported for documentation purposes.

## See Also

- [Command Reference](command-reference.md) - Quick command lookup
- [Drift Detection Guide](drift-detection.md) - Complete drift detection reference
- [Security Alerting](security-alerting.md) - Severity classification details
- [Azure DevOps Integration](azure-devops-integration.md) - CI/CD pipeline examples
- [Authentication Guide](authentication.md) - Setup and credentials
- [Getting Started](getting-started.md) - Complete setup guide
- [Pipeline Templates](../examples/pipelines/) - Ready-to-use automation
