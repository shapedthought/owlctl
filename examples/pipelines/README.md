# Azure DevOps Pipeline Templates

Ready-to-use Azure DevOps pipeline templates for VBR configuration management with owlctl.

## Recommended GitOps Workflow

These pipelines work together to enforce **main = desired state of your VBR environment**:

1. **Bootstrap** (run once) - Capture current VBR state into Git with [bootstrap.yml](bootstrap.yml)
2. **PR gate** - Set up [gitops-pr-gate.yml](gitops-pr-gate.yml) as Build Validation on main. Every PR that changes specs is checked against [policy rules](policies/policy-rules.yaml) and dry-run against VBR before merge.
3. **Deploy on merge** - Set up [gitops-deploy.yml](gitops-deploy.yml) as CI trigger on main. After merge, specs are applied in dependency order, then re-exported and committed back to Git.
4. **Ongoing assurance** (optional) - Add [detect-remediate.yml](detect-remediate.yml) or [nightly-compliance.yml](nightly-compliance.yml) to catch out-of-band changes.

```
Issue -> Branch -> Edit specs -> PR (policy + dry-run) -> Merge -> Apply -> Commit state
```

## Available Templates

| Template | Purpose | Trigger |
|----------|---------|---------|
| [bootstrap.yml](bootstrap.yml) | Capture current VBR state into Git (run once) | Manual |
| [retention-change.yml](retention-change.yml) | Declarative retention change walkthrough | Manual |
| [retention-change-gitops.yml](retention-change-gitops.yml) | GitOps retention change (reads/commits to Git) | Manual |
| [detect-remediate.yml](detect-remediate.yml) | Detect drift and auto-remediate | Scheduled (weekday 6AM UTC) |
| [pr-validation.yml](pr-validation.yml) | Validate spec changes before merge | Pull requests |
| [deployment.yml](deployment.yml) | Multi-stage deployment with approval gates | Push to master / manual |
| [nightly-compliance.yml](nightly-compliance.yml) | Generate compliance reports | Scheduled (nightly 2AM UTC) |
| [gitops-pr-gate.yml](gitops-pr-gate.yml) | Policy enforcement + validation before merge | Pull requests |
| [gitops-deploy.yml](gitops-deploy.yml) | Apply specs, commit state back to Git | Push to master/main |
| [policies/policy-rules.yaml](policies/policy-rules.yaml) | Configurable policy rules for PR gate | N/A (config file) |

## Prerequisites

### 1. Variable Group

Create a Variable Group named `veeam-credentials` in your Azure DevOps project:

1. Go to **Pipelines → Library → Variable groups**
2. Click **+ Variable group**
3. Name: `veeam-credentials`
4. Add variables:
   - `OWLCTL_USERNAME` - VBR API username
   - `OWLCTL_PASSWORD` - VBR API password (mark as **secret**)
   - `OWLCTL_URL` - VBR hostname or IP only, no scheme or port (e.g., `vbr.example.com`)
5. (Optional) `SLACK_WEBHOOK_URL` for notifications

**Security note:** Use Azure Key Vault linked Variable Groups for production environments.

### 2. Infrastructure Specs in Git

Organize your VBR specs in a directory structure:

```
infrastructure/
├── jobs/
│   ├── daily-backup.yaml
│   └── weekly-full.yaml
├── repos/
│   ├── default-repo.yaml
│   └── linux-repo.yaml
├── sobrs/
│   └── performance-tier.yaml
└── kms/
    └── azure-kms.yaml
```

Bootstrap from existing VBR configuration:

**Option 1: Use the bootstrap pipeline (recommended)**

Run [bootstrap.yml](bootstrap.yml) once — it exports all jobs, snapshots all resources, and commits everything to Git automatically.

**Option 2: Manual bootstrap**

```bash
# Export current configuration as YAML specs
owlctl job export --all -d infrastructure/jobs/

# Snapshot current state (records baseline in state.json)
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all
owlctl encryption snapshot --all
owlctl encryption kms-snapshot --all

# Commit specs and state to Git
git add infrastructure/ state.json
git commit -m "Bootstrap VBR declarative management"
```

### 3. Environment with Approvals (deployment.yml only)

For the deployment pipeline, create an Environment with approval gates:

1. Go to **Pipelines → Environments**
2. Click **New environment**
3. Name: `vbr-production`
4. Click the environment, then **⋮ → Approvals and checks**
5. Add **Approvals** and select approvers

### 4. Self-Hosted Agent (if needed)

If VBR is not internet-accessible, use a self-hosted agent:

1. **Pipelines → Agent pools → Add pool**
2. Install agent on a machine with network access to VBR
3. Update pipeline `pool` section:

```yaml
pool:
  name: 'VeeamInfraPool'  # Your agent pool name
```

## Quick Start

### Option 1: Copy and customize

1. Copy the template files to your repository
2. Update paths and variable names as needed
3. Create the pipeline in Azure DevOps

### Option 2: Import directly

1. Go to **Pipelines → New pipeline**
2. Select your repository
3. Choose **Existing Azure Pipelines YAML file**
4. Select the template path (e.g., `examples/pipelines/detect-remediate.yml`)

## Exit Code Reference

owlctl exit codes determine pipeline outcomes:

| Exit Code | Meaning | Pipeline Behavior |
|-----------|---------|-------------------|
| `0` | Success / No drift | Passes |
| `3` | Warning-level drift | Succeeds with issues |
| `4` | Critical drift | Fails (triggers remediation) |
| `5` | Partial apply | Succeeds with issues |
| `6` | Resource not found | Fails (manual recreation needed) |
| `1` | Error | Fails |

## Template Details

### bootstrap.yml

**Purpose:** Run-once pipeline to capture the current VBR environment into Git

This pipeline exports all backup jobs as YAML files and snapshots all resource types (repositories, SOBRs, encryption passwords, KMS servers) into `state.json`. Run this once to establish the GitOps baseline before using other pipelines.

**Single stage:**
1. **Bootstrap** - Build owlctl, export all jobs, snapshot all resources, commit to Git

**Result in repo:**
```
infrastructure/
└── jobs/
    ├── backup-job-1.yaml
    ├── weekly-full.yaml
    └── ...
state.json    ← contains state for ALL resource types
```

**Requirements:**
- Self-hosted agent with Go installed
- Git repo with write permissions (uses `persistCredentials: true`)

**Notes:**
- Commits with `[skip ci]` to prevent pipeline trigger loops
- Safe to re-run: if no changes are detected, the commit is skipped
- Only jobs have YAML export; repos, SOBRs, encryption, and KMS are captured in `state.json` only

### retention-change.yml

**Purpose:** Step-by-step walkthrough of declarative job management

This pipeline demonstrates the complete GitOps workflow for changing a backup job's retention policy. It is designed as a learning resource and starting point for teams adopting declarative configuration management.

**Stages:**
1. **Setup** - Build owlctl and verify VBR connectivity
2. **Export** - Capture current job config as base YAML and snapshot state
3. **Plan** - Preview the retention change with dry-run (no changes made)
4. **Apply** - Apply the retention overlay to VBR
5. **Verify** - Confirm VBR matches desired state (drift check)

**Customization:**
```yaml
variables:
  # Change these to match your environment
  - name: JOB_NAME
    value: 'My Backup Job'        # Name of the job to modify
  - name: DESIRED_RETENTION_DAYS
    value: '30'                    # Target retention in days
```

**What It Demonstrates:**
- Exporting live VBR configuration as YAML
- Creating minimal overlays (only the fields you want to change)
- Dry-run to preview changes before applying
- Apply with overlay merge (base + overlay = desired state)
- Post-apply verification with drift detection
- Exit code handling for CI/CD integration

### retention-change-gitops.yml

**Purpose:** GitOps deployment pipeline that reads from and commits back to Git

Unlike `retention-change.yml` (which is ephemeral), this pipeline treats Git as the single source of truth. It reads job YAML and `state.json` from the repo, applies changes to VBR, then commits the updated state and re-exported job spec back to Git.

**Prerequisites:**
- Run `bootstrap.yml` first to populate `infrastructure/` and `state.json`
- Self-hosted agent with Go installed

**Stages:**
1. **Plan** - Validate prerequisites, create overlay, dry-run (read-only)
2. **Apply and Commit** - Apply to VBR, re-export job YAML, commit updated state to Git
3. **Verify** - Pull latest commit and confirm zero drift

**Customization:**
```yaml
variables:
  - name: JOB_NAME
    value: 'My Backup Job'        # Must match a bootstrapped job
  - name: DESIRED_RETENTION_DAYS
    value: '30'                    # Target retention in days
```

**What It Demonstrates:**
- Reading job specs and state from Git (not live VBR export)
- Deriving job filenames using the same sanitization as `owlctl export`
- Committing updated state and job YAML back to Git with `[skip ci]`
- Re-exporting from VBR after apply to capture authoritative state
- End-to-end verification that VBR matches the committed Git state

### detect-remediate.yml

**Purpose:** Automated drift detection and remediation

**Stages:**
1. **Detect** - Scan all resources for security drift
2. **Remediate** - Apply specs from Git (only if drift found)
3. **Verify** - Confirm remediation was successful
4. **Notify** - Alert team if manual intervention needed

**Customization:**
```yaml
variables:
  - name: specsPath
    value: 'my-custom-path'  # Change infrastructure path
  - name: owlctlVersion
    value: 'v1.1.0'  # Pin to specific version
```

### pr-validation.yml

**Purpose:** Gate PRs that modify infrastructure specs

**Stages:**
1. **Validate** - Dry-run all changed YAML files
2. **DriftCheck** - Check for unexpected drift in existing resources
3. **Summary** - Report validation results

**Branch Policy Setup:**
1. Go to **Repos → Branches → master → ⋮ → Branch policies**
2. Under **Build Validation**, click **+**
3. Select the PR validation pipeline
4. Enable **Require** and **Immediately when updated**

### deployment.yml

**Purpose:** Controlled deployment with approval gates

**Stages:**
1. **PreDeploy** - Validate all specs with dry-run
2. **Deploy** - Apply configuration (with optional approval)
3. **PostDeploy** - Verify changes were applied

**Manual Trigger:**
```yaml
trigger: none  # Change to enable auto-trigger on push
```

### nightly-compliance.yml

**Purpose:** Generate compliance reports for audit

**Output:**
- Markdown report attached to build summary
- Artifact: `ComplianceReport/compliance-report.md`
- Optional: Wiki page update

**Enable Wiki Update:**
```yaml
variables:
  - name: updateWiki
    value: 'true'
  - name: wikiPath
    value: '/Compliance/VBR-Drift-Report'
```

### gitops-pr-gate.yml

**Purpose:** Policy enforcement and validation gate for PRs that modify infrastructure specs

Unlike `pr-validation.yml`, this pipeline adds a **PolicyCheck** stage before dry-run. Policy rules are evaluated with `yq` against the changed YAML files — no VBR connection needed. Blocked specs (e.g., encryption disabled) never reach VBR.

**Stages:**
1. **PolicyCheck** - Evaluate changed specs against `policies/policy-rules.yaml` (block or warn)
2. **Validate** - Dry-run changed specs against live VBR
3. **DriftCheck** - Check existing resources for unexpected drift (informational)
4. **Summary** - Report validation results

**Branch Policy Setup:**
1. Go to **Repos > Branches > master > Branch policies**
2. Under **Build Validation**, click **+**
3. Select the `gitops-pr-gate` pipeline
4. Enable **Require** and **Immediately when updated**

**Customization:**
```yaml
variables:
  - name: policyFile
    value: 'examples/pipelines/policies/policy-rules.yaml'  # Path to your policy rules
```

### gitops-deploy.yml

**Purpose:** Apply all specs on merge to main, then commit updated state back to Git

This pipeline enforces **main = desired state**. After merge, all specs are applied in dependency order (KMS > repos > SOBRs > jobs), then specs are re-exported and state is committed back to Git with `[skip ci]`.

**Stages:**
1. **PreDeploy** - Dry-run all specs (catches issues from concurrent merges)
2. **Deploy** - Apply specs in dependency order (uses `vbr-production` environment for approval gates)
3. **CommitState** - Re-export jobs, snapshot all resources, commit state to Git
4. **PostDeploy** - Drift check to verify everything applied cleanly

**Requirements:**
- Environment `vbr-production` with approval gates configured
- Git repo write permissions (`persistCredentials: true` on checkout)

**Key differences from `deployment.yml`:**
- Adds CommitState stage so Git stays the authoritative audit trail
- Applies in dependency order (KMS > repos > SOBRs > jobs)
- Commits with `[skip ci]` to prevent re-triggering

### policies/policy-rules.yaml

**Purpose:** Configurable policy rules evaluated during PR validation

Rules are evaluated against changed YAML spec files using `yq`. Each rule specifies a resource kind, a field path, and an action (`block` or `warn`).

**Default block rules (hard failures):**
- Job encryption disabled
- Job disabled via GitOps
- SOBR immutability disabled
- SOBR capacity tier encryption disabled

**Default warn rules (allow with flag):**
- Retention policy changed
- Job schedule disabled

**Adding custom rules:**
```yaml
rules:
  - name: my-custom-rule
    resource: VBRJob           # Matches kind: field
    path: .spec.some.field     # yq path to check
    operator: equals           # equals or exists
    value: "unwanted-value"    # value to match (for equals)
    action: block              # block or warn
    message: "Custom message"
```

## Troubleshooting

### Authentication Failures

```
Error: failed to login
```

- Verify `OWLCTL_URL` is the hostname/IP only — no scheme or port (e.g., `vbr.example.com`, not `https://vbr:9419`)
- Check username format (may need `DOMAIN\user`)
- Ensure VBR REST API is enabled

### Resource Not Found (Exit 6)

```
Error: resource 'X' not found in VBR (update-only mode)
```

- Resource was deleted from VBR
- Must recreate in VBR console, then re-apply the exported spec

### TLS Certificate Errors

For self-signed certificates, you have two options:

**Option 1: Install CA certificate on agent (recommended)**

Install the VBR server's CA certificate in the agent's system trust store. owlctl will automatically trust certificates validated by the system trust store.

**Linux agent:**
```bash
# Ubuntu/Debian
sudo cp vbr-ca.crt /usr/local/share/ca-certificates/
sudo update-ca-certificates

# RHEL/CentOS
sudo cp vbr-ca.crt /etc/pki/ca-trust/source/anchors/
sudo update-ca-trust
```

**Windows agent:**
```powershell
# Import to Trusted Root Certification Authorities
Import-Certificate -FilePath vbr-ca.crt -CertStoreLocation Cert:\LocalMachine\Root
```

**Option 2: Skip TLS verification (not recommended for production)**

Set `apiNotSecure: true` in settings.json. This bypasses certificate validation entirely and should only be used for testing.

```json
{
  "apiNotSecure": true,
  "credsFileMode": false
}
```

**Warning:** Disabling TLS verification exposes credentials to man-in-the-middle attacks and should never be used in production environments.

### Pipeline Permissions

If wiki update fails:
1. Go to **Project Settings → Repositories → Security**
2. Grant **Contribute** permission to build service account

## Hardening Pipeline Security

The templates in this directory are self-contained for ease of adoption. In production, you should prevent users from modifying pipeline definitions to bypass policy checks.

### Protect pipeline files with required reviewers

Add `examples/pipelines/**` and `policies/**` to your branch policy path filters so changes to pipeline YAML and policy rules go through the same PR gate. Use required reviewers (Azure DevOps) or CODEOWNERS (GitHub) to require security team approval for these paths.

### Use extends templates from a protected repo (recommended)

Azure DevOps supports [template references from other repositories](https://learn.microsoft.com/en-us/azure/devops/pipelines/process/templates). Move the policy and validation logic into a separate, locked-down repository and reference it with `extends`. Users can customize parameters but cannot skip or modify the enforcement stages.

```yaml
# In the team's repo — cannot bypass the gate
resources:
  repositories:
    - repository: pipeline-templates
      type: git
      name: MyProject/pipeline-templates  # Restricted write access

extends:
  template: templates/gitops-gate.yml@pipeline-templates
  parameters:
    specsPath: infrastructure/
    policyFile: policies/policy-rules.yaml
```

The `pipeline-templates` repo has restricted write access — only the platform/security team can modify it.

### Restrict pipeline editing permissions

Under **Pipelines > Security**, limit who can edit pipeline definitions. This prevents users from modifying the pipeline through the Azure DevOps UI even if they can't push changes to the YAML file.

### Environment approvals are separate from pipeline code

The `vbr-production` environment approval gate is configured in the Azure DevOps UI, not in the pipeline YAML. Even if someone modifies the pipeline definition, they cannot bypass environment approvals.

## See Also

- [Drift Detection Guide](../../docs/drift-detection.md)
- [Azure DevOps Integration](../../docs/azure-devops-integration.md)
- [Security Alerting](../../docs/security-alerting.md)
