# owlctl Configuration Examples

This directory contains example configurations for owlctl's declarative management features.

## Recommended Directory Layout

```
examples/
├── owlctl.yaml                  # Groups, instances — main config
├── profiles/
│   ├── standard-db-backup.yaml  # kind: Profile — database defaults
│   └── standard-file-backup.yaml  # kind: Profile — file server defaults
├── overlays/
│   ├── enable-encryption.yaml   # kind: Overlay — enforce encryption
│   ├── retention-30d.yaml       # kind: Overlay — 30-day retention
│   └── weekend-schedule.yaml    # kind: Overlay — weekend window
├── specs/
│   ├── sql-daily.yaml           # kind: VBRJob  — thin spec (identity + overrides)
│   └── file-backup.yaml         # kind: VBRJob  — thin spec (identity only)
├── jobs/                        # Full standalone specs (no profile needed)
│   ├── database-backup.yaml
│   └── web-tier-backup.yaml
├── repos/                       # Repository specs
├── sobrs/                       # SOBR specs
├── kms/                         # KMS server specs
└── pipelines/                   # CI/CD templates
```

## Groups, Profiles, and Overlays

Groups are the recommended way to manage backup jobs. A group bundles thin specs with a shared **profile** (defaults) and optional **overlay** (policy patch).

### owlctl.yaml

```yaml
apiVersion: owlctl.veeam.com/v1
kind: Config

groups:
  db-tier:
    description: Database backups — encrypted, 14-day retention
    profile: profiles/standard-db-backup.yaml
    overlay: overlays/enable-encryption.yaml
    specs:
      - specs/sql-daily.yaml

  file-tier:
    description: File server backups — 7-day retention
    profile: profiles/standard-file-backup.yaml
    specs:
      - specs/file-backup.yaml

instances:
  primary:
    product: vbr
    url: vbr-prod.example.com    # hostname/IP only — no scheme, no port
    credentialRef: PROD          # reads OWLCTL_PROD_USERNAME / OWLCTL_PROD_PASSWORD
  dr:
    product: vbr
    url: vbr-dr.example.com
    credentialRef: DR
```

### How It Works

Each spec in a group is merged in three layers:

```
Profile  →  Spec  →  Overlay  =  Final Configuration
(defaults)  (identity + overrides)  (policy patch)
```

1. **Profile** (`kind: Profile`) provides base defaults — repository, retention, schedule, guest processing
2. **Spec** (`kind: VBRJob`) declares identity (name, objects) and any field overrides
3. **Overlay** (`kind: Overlay`) applies policy patches — encryption, retention extensions, schedule changes

The strategic merge engine:
- **Deep merges maps** — nested objects merge recursively
- **Replaces arrays** — arrays in higher layers replace lower layers entirely
- **Merges labels** — labels and annotations combine across layers

Example merge:
```yaml
# Profile (standard-db-backup.yaml)
spec:
  schedule:
    daily: "22:00"
  storage:
    retention:
      quantity: 14

# Spec (sql-daily.yaml) — overrides schedule only
spec:
  schedule:
    daily: "02:00"

# Merged Result
spec:
  schedule:
    daily: "02:00"    # From spec
  storage:
    retention:
      quantity: 14    # Preserved from profile
```

### Group Commands

```bash
# List all groups
owlctl group list

# Show group details (resolved paths, spec count)
owlctl group show db-tier

# Apply a group (dry-run first)
owlctl job apply --group db-tier --dry-run
owlctl job apply --group db-tier

# Apply to a specific VBR instance (overrides the group's pinned instance)
owlctl --instance primary job apply --group db-tier

# Drift check a group
owlctl --instance primary job diff --group db-tier
```

---

## Standalone Mode

For simpler setups that don't need groups, apply individual files directly with an optional overlay:

```bash
# Apply a full standalone spec
owlctl job apply jobs/database-backup.yaml

# Apply with a policy overlay
owlctl job apply jobs/database-backup.yaml --overlay overlays/retention-30d.yaml

# Preview the merged result
owlctl job plan jobs/database-backup.yaml --overlay overlays/enable-encryption.yaml --show-yaml
```

See [`jobs/`](jobs/) for full standalone spec examples.

---

## Other Resource Types

Repositories, SOBRs, and KMS servers follow the same spec format and support overlays with `-o`:

### Repositories

```bash
owlctl repo export "Default Backup Repository" -o base-repo.yaml
owlctl repo apply base-repo.yaml --overlay prod-repo-overlay.yaml
owlctl repo diff --all
```

See [`repos/`](repos/) for examples.

### Scale-Out Backup Repositories (SOBRs)

```bash
owlctl repo sobr-export "Scale-out Repository 1" -o base-sobr.yaml
owlctl repo sobr-apply base-sobr.yaml
owlctl repo sobr-diff --all
```

See [`sobrs/`](sobrs/) for examples.

### KMS Servers

```bash
owlctl encryption kms-export "Azure Key Vault" -o base-kms.yaml
owlctl encryption kms-apply base-kms.yaml
owlctl encryption kms-diff --all
```

See [`kms/`](kms/) for examples.

---

## CI/CD

See [`pipelines/`](pipelines/) for ready-to-use Azure DevOps pipeline templates covering:
- PR validation with `--dry-run`
- Nightly compliance checks with `job diff --security-only`
- Automated remediation workflows

---

## Next Steps

- [Drift Detection Guide](../docs/drift-detection.md) — monitoring configuration changes
- [Security Alerting](../docs/security-alerting.md) — value-aware severity classification
- [Azure DevOps Integration](../docs/azure-devops-integration.md) — CI/CD setup
