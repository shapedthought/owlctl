# Azure DevOps Integration Guide

owlctl integrates with Azure DevOps Pipelines to provide automated configuration assurance for Veeam Backup & Replication environments. This guide covers what works today, what the complete integration looks like, and what features are needed to get there.

## Quick Start: Pipeline Templates

Ready-to-use pipeline templates are available in [`examples/pipelines/`](../examples/pipelines/):

| Template | Purpose |
|----------|---------|
| [detect-remediate.yml](../examples/pipelines/detect-remediate.yml) | Scheduled drift detection and auto-remediation |
| [pr-validation.yml](../examples/pipelines/pr-validation.yml) | PR validation gate with dry-run |
| [deployment.yml](../examples/pipelines/deployment.yml) | Multi-stage deployment with approval gates |
| [nightly-compliance.yml](../examples/pipelines/nightly-compliance.yml) | Compliance report generation |
| [gitops-pr-gate.yml](../examples/pipelines/gitops-pr-gate.yml) | Policy enforcement + validation before merge |
| [gitops-deploy.yml](../examples/pipelines/gitops-deploy.yml) | Apply specs on merge, commit state to Git |

See the [pipeline README](../examples/pipelines/README.md) for setup instructions.

## Current Integration

### Installing owlctl in a pipeline

Every pipeline installs owlctl with the shared step template
[`install-owlctl.yml`](../examples/pipelines/templates/install-owlctl.yml). It downloads the
release `.tar.gz` and its `.sha256`, verifies the checksum, extracts the binary and runs
`./owlctl --help` as a smoke test. Copy the template into your repo and pin `owlctlVersion`
to a release tag; the snippets below reference it as
`examples/pipelines/templates/install-owlctl.yml`, so adjust the path to match where you put it.

### What Works Today

owlctl is already usable in Azure DevOps Pipelines with these capabilities:

| Capability | Status | Notes |
|------------|--------|-------|
| CLI invocation in pipeline steps | Ready | Standard `script` or `Bash@3` tasks |
| Exit codes for pipeline gates | Ready | `0`=clean, `3`=warning drift, `4`=critical drift, `1`=error |
| Environment variable auth | Ready | `OWLCTL_USERNAME`, `OWLCTL_PASSWORD`, `OWLCTL_URL` |
| Severity filtering | Ready | `--severity critical`, `--security-only` flags |
| Scheduled drift scans | Ready | Pipelines support cron schedules with `always: true` |
| State file in Git | Ready | `state.json` can be committed and tracked |
| Human-readable output | Ready | Text output in pipeline logs |
| Declarative job management | Ready | `job export` → `job apply` → `job diff` for backup jobs |
| Declarative management (all resources) | Ready | `export` → `apply` → `diff` for repos, SOBRs, encryption, KMS (see [pipeline templates](../examples/pipelines/)) |
| JSON output for diff commands | Not yet | Diff commands only output text |
| Markdown report generation | Not yet | No `--format markdown` flag |
| Webhook notifications | Not yet | Must use pipeline scripts for notifications |
| SARIF/JUnit output | Not yet | No standard security format output |

### Basic Pipeline: Scheduled Drift Scan

This is the simplest integration — a scheduled pipeline that runs drift checks and fails on critical findings.

```yaml
# azure-pipelines-drift-scan.yml
trigger: none

schedules:
  - cron: "0 6 * * 1-5"
    displayName: "Weekday 6AM UTC drift scan"
    branches:
      include:
        - master
    always: true  # Run even with no code changes

pool:
  vmImage: 'ubuntu-latest'

variables:
  - group: 'veeam-credentials'  # Contains OWLCTL_USERNAME, OWLCTL_PASSWORD, OWLCTL_URL

steps:
  - template: examples/pipelines/templates/install-owlctl.yml  # pin owlctlVersion to a release tag

  - script: ./owlctl init && ./owlctl profile --set vbr && ./owlctl login
    displayName: 'Authenticate to VBR'
    env:
      OWLCTL_USERNAME: $(OWLCTL_USERNAME)
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
      OWLCTL_URL: $(OWLCTL_URL)

  - script: |
      ./owlctl job diff --all --security-only
      JOB_EXIT=$?

      ./owlctl repo diff --all --security-only
      REPO_EXIT=$?

      ./owlctl repo sobr-diff --all --security-only
      SOBR_EXIT=$?

      ./owlctl encryption diff --all --security-only
      ENC_EXIT=$?

      ./owlctl encryption kms-diff --all --security-only
      KMS_EXIT=$?

      # Fail pipeline if any CRITICAL drift found (exit code 4)
      for CODE in $JOB_EXIT $REPO_EXIT $SOBR_EXIT $ENC_EXIT $KMS_EXIT; do
        if [ $CODE -eq 4 ]; then
          echo "##vso[task.logissue type=error]CRITICAL security drift detected"
          exit 1
        fi
      done

      # Warn if any non-critical drift found (exit code 3)
      for CODE in $JOB_EXIT $REPO_EXIT $SOBR_EXIT $ENC_EXIT $KMS_EXIT; do
        if [ $CODE -eq 3 ]; then
          echo "##vso[task.logissue type=warning]Security drift detected (non-critical)"
          echo "##vso[task.complete result=SucceededWithIssues;]Drift detected"
          exit 0
        fi
      done

      echo "No security drift detected"
    displayName: 'Run drift detection'
    env:
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
```

### Credentials Management

Azure DevOps provides several options for storing VBR credentials securely:

**Variable Groups (recommended for most teams)**

1. Go to Pipelines > Library > Variable groups
2. Create a group called `veeam-credentials`
3. Add variables: `OWLCTL_USERNAME`, `OWLCTL_PASSWORD` (mark as secret), `OWLCTL_URL`
4. Reference in pipeline with `- group: 'veeam-credentials'`

**Azure Key Vault (recommended for enterprise)**

1. Store secrets in Azure Key Vault
2. Link a Variable Group to the Key Vault under Pipelines > Library
3. Secrets are fetched at pipeline runtime with full audit logging

```yaml
variables:
  - group: 'owlctl-keyvault-linked'  # Linked to Azure Key Vault
```

**Secure Files (for settings.json and profiles.json)**

```yaml
steps:
  - task: DownloadSecureFile@1
    name: owlctlSettings
    inputs:
      secureFile: 'owlctl-settings.json'

  - task: DownloadSecureFile@1
    name: owlctlProfiles
    inputs:
      secureFile: 'owlctl-profiles.json'

  - script: |
      mkdir -p $HOME/.owlctl
      cp $(owlctlSettings.secureFilePath) $HOME/.owlctl/settings.json
      cp $(owlctlProfiles.secureFilePath) $HOME/.owlctl/profiles.json
      ./owlctl profile --set vbr
      ./owlctl login
    env:
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
```

### Exit Codes and Pipeline Logic

owlctl's exit codes map directly to Azure DevOps pipeline outcomes:

| owlctl Exit Code | Pipeline Result | Action |
|----------------|----------------|--------|
| `0` | Succeeded | No drift — pipeline passes |
| `3` | SucceededWithIssues | INFO/WARNING drift — pipeline passes with warning |
| `4` | Failed | CRITICAL drift — pipeline fails, blocks deployment |
| `1` | Failed | Error — pipeline fails, needs investigation |

Use Azure DevOps logging commands to control how results appear:

```bash
# Mark step as succeeded with issues (yellow warning)
echo "##vso[task.complete result=SucceededWithIssues;]Non-critical drift found"

# Log a warning that appears in the pipeline summary
echo "##vso[task.logissue type=warning]3 WARNING-level drifts detected"

# Log an error
echo "##vso[task.logissue type=error]CRITICAL: Encryption disabled on production job"

# Tag the build for filtering
echo "##vso[build.addbuildtag]drift-detected"
echo "##vso[build.addbuildtag]critical-drift"
```

### Multi-Stage Pipeline: Scan, Report, Notify

```yaml
# azure-pipelines-drift-full.yml
trigger: none

schedules:
  - cron: "0 6 * * *"
    displayName: "Daily 6AM UTC"
    branches:
      include:
        - master
    always: true

pool:
  vmImage: 'ubuntu-latest'

variables:
  - group: 'veeam-credentials'

stages:
  - stage: Scan
    displayName: 'Drift Detection'
    jobs:
      - job: DriftScan
        displayName: 'Scan all resources'
        steps:
          - template: examples/pipelines/templates/install-owlctl.yml

          - script: |
              ./owlctl init && ./owlctl profile --set vbr && ./owlctl login
            displayName: 'Setup owlctl'
            env:
              OWLCTL_USERNAME: $(OWLCTL_USERNAME)
              OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
              OWLCTL_URL: $(OWLCTL_URL)

          - script: |
              CRITICAL=0
              OUTPUT=""

              for CMD in \
                "job diff --all" \
                "repo diff --all" \
                "repo sobr-diff --all" \
                "encryption diff --all" \
                "encryption kms-diff --all"
              do
                RESULT=$(./owlctl $CMD 2>&1) || true
                EXIT=$?
                OUTPUT="$OUTPUT\n--- owlctl $CMD ---\n$RESULT\n"
                if [ $EXIT -eq 4 ]; then
                  CRITICAL=1
                fi
              done

              echo -e "$OUTPUT" > $(Build.ArtifactStagingDirectory)/drift-report.txt
              echo "##vso[task.setvariable variable=hasCritical;isOutput=true]$CRITICAL"

              if [ $CRITICAL -eq 1 ]; then
                echo "##vso[task.logissue type=error]CRITICAL drift detected"
                echo "##vso[build.addbuildtag]critical-drift"
                exit 1
              fi
            displayName: 'Run drift scan'
            name: scan
            env:
              OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)

          - publish: $(Build.ArtifactStagingDirectory)
            artifact: DriftReport
            condition: always()
            displayName: 'Publish drift report'

  - stage: Notify
    displayName: 'Notifications'
    dependsOn: Scan
    condition: failed()
    jobs:
      - job: NotifyTeam
        steps:
          - download: current
            artifact: DriftReport

          - script: |
              curl -X POST -H 'Content-type: application/json' \
                --data '{
                  "text": "CRITICAL: VBR security drift detected. Pipeline: $(Build.DefinitionName) Build: $(Build.BuildNumber). Investigate immediately."
                }' \
                $(SLACK_WEBHOOK_URL)
            displayName: 'Send Slack alert'
            env:
              SLACK_WEBHOOK_URL: $(SLACK_WEBHOOK_URL)
```

### PR Validation: Config File Changes

If VBR configuration files (YAML job definitions, severity config) are stored in Git, a PR validation pipeline can verify changes before merge:

```yaml
# azure-pipelines-pr-validation.yml
trigger: none

pr:
  branches:
    include:
      - master
  paths:
    include:
      - infrastructure/**
      - overlays/**

pool:
  vmImage: 'ubuntu-latest'

variables:
  - group: 'veeam-credentials'

steps:
  - template: examples/pipelines/templates/install-owlctl.yml

  - script: |
      ./owlctl init && ./owlctl profile --set vbr && ./owlctl login
    displayName: 'Setup owlctl'
    env:
      OWLCTL_USERNAME: $(OWLCTL_USERNAME)
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
      OWLCTL_URL: $(OWLCTL_URL)

  - script: |
      # Validate all changed YAML files
      for file in $(git diff --name-only origin/master...HEAD -- '*.yaml' '*.yml'); do
        echo "Validating: $file"
        ./owlctl job plan "$file" --show-yaml
        if [ $? -ne 0 ]; then
          echo "##vso[task.logissue type=error]Validation failed for $file"
          exit 1
        fi
      done
    displayName: 'Validate job configurations'
    env:
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
```

Set this as a **required build validation** under Repos > Branches > Branch Policies for `master` to block PRs that contain invalid configurations.

### Self-Hosted Agents

For environments where the VBR server is not internet-accessible, use a self-hosted agent on the same network:

```yaml
pool:
  name: 'VeeamInfraPool'  # Self-hosted agent pool
  demands:
    - Agent.OS -equals Linux
```

Self-hosted agents are installed on a machine within your network. This avoids exposing the VBR API to the internet. The agent communicates outbound to Azure DevOps (no inbound ports required).

---

## GitOps Workflow

The recommended production setup uses two pipelines to enforce **main = desired state of VBR**. Changes flow through a controlled lifecycle: issue, branch, PR, validate, merge, apply, commit state.

### Lifecycle

```
1. Create issue      -> "Change retention to 30 days for prod jobs"
2. Create branch     -> feature/retention-30d
3. Edit specs        -> Modify infrastructure/jobs/prod-backup.yaml
4. Open PR           -> gitops-pr-gate.yml runs automatically
   a. Policy check   -> Blocks if encryption/immutability disabled
   b. Dry-run        -> Validates specs against live VBR
   c. Drift check    -> Reports any unexpected drift (informational)
5. Review & merge    -> Approver reviews pipeline results and merges
6. Deploy            -> gitops-deploy.yml runs automatically
   a. Apply          -> Specs applied in dependency order (KMS > repos > SOBRs > jobs)
   b. Commit state   -> Re-exported specs + state.json committed to Git
   c. Verify         -> Drift check confirms clean apply
```

### Setting Up

**Step 1: Bootstrap your environment**

Run [bootstrap.yml](../examples/pipelines/bootstrap.yml) once to export all current VBR configuration into Git. This creates the `infrastructure/` directory with job specs and `state.json` with resource snapshots.

**Step 2: Configure PR gate**

1. Create a pipeline from [gitops-pr-gate.yml](../examples/pipelines/gitops-pr-gate.yml)
2. Go to **Repos > Branches > master > Branch policies**
3. Under **Build Validation**, add the pipeline as a required check

This ensures every spec change is validated against [policy rules](../examples/pipelines/policies/policy-rules.yaml) and dry-run against VBR before merge.

**Step 3: Configure deploy pipeline**

1. Create a pipeline from [gitops-deploy.yml](../examples/pipelines/gitops-deploy.yml)
2. It triggers automatically on push to main (when `infrastructure/` files change)
3. Create an Environment named `vbr-production` with approval gates for the Deploy stage

**Step 4: Add ongoing assurance (optional)**

Add [detect-remediate.yml](../examples/pipelines/detect-remediate.yml) on a schedule to catch out-of-band changes, or [nightly-compliance.yml](../examples/pipelines/nightly-compliance.yml) for audit reports.

### Policy Enforcement

The PR gate evaluates changed specs against configurable rules in [policies/policy-rules.yaml](../examples/pipelines/policies/policy-rules.yaml). Rules use `yq` to inspect YAML fields — no VBR connection needed.

**Block rules** fail the pipeline (PR cannot merge):
- Job encryption disabled
- Job disabled via GitOps
- SOBR immutability disabled
- SOBR capacity tier encryption disabled

**Warn rules** allow merge with a warning:
- Retention policy changed
- Job schedule disabled

Custom rules can be added to `policy-rules.yaml` — see the [pipeline README](../examples/pipelines/README.md) for syntax.

### State Commit Pattern

After applying specs, the deploy pipeline re-exports job YAML and snapshots all resources, then commits the result back to Git with `[skip ci]`. This ensures:

- Git always reflects VBR's authoritative state (including fields VBR normalizes or rejects)
- `state.json` is always current for drift detection
- The commit history provides an audit trail of every deployment

---

## Complete Integration

Epic #42 is now complete, enabling full declarative management with automated remediation across all VBR resource types:

- **Epic #23** — Configuration Assurance (detection and alerting for all VBR resource types) ✅ Complete
- **Epic #42** — Declarative Resource Management and Remediation (export, snapshot, apply for all resource types) ✅ Complete

### Declarative Management Workflow

Every VBR resource type now supports the full lifecycle:

```
export → snapshot → apply → diff
```

**Day 1: Establish baseline**
```yaml
steps:
  - script: |
      # Export current VBR configuration to YAML specs
      ./owlctl repo export --all -o infrastructure/repos/
      ./owlctl repo sobr-export --all -o infrastructure/sobrs/
      ./owlctl encryption export --all -o infrastructure/encryption/
      ./owlctl encryption kms-export --all -o infrastructure/kms/

      # Snapshot resources as desired state (no VBR changes)
      ./owlctl repo snapshot --all
      ./owlctl repo sobr-snapshot --all
      ./owlctl encryption snapshot --all
      ./owlctl encryption kms-snapshot --all
      ./owlctl config-backup snapshot
    displayName: 'Bootstrap declarative management'
```

The specs are committed to Git. State records each resource with `origin: "applied"`, meaning owlctl knows what the configuration *should* look like and can remediate drift.

### Automated Detect and Remediate

The key pipeline enabled by Epic #42. Detects drift, auto-remediates what VBR allows, and reports what it can't fix.

```yaml
# azure-pipelines-remediate.yml
trigger: none

schedules:
  - cron: "0 6 * * 1-5"
    displayName: "Weekday 6AM UTC"
    branches:
      include:
        - master
    always: true

pool:
  vmImage: 'ubuntu-latest'

variables:
  - group: 'veeam-credentials'

stages:
  - stage: Detect
    displayName: 'Detect Drift'
    jobs:
      - job: DriftScan
        steps:
          - template: examples/pipelines/templates/install-owlctl.yml

          - script: |
              ./owlctl init && ./owlctl profile --set vbr && ./owlctl login
            displayName: 'Setup'
            env:
              OWLCTL_USERNAME: $(OWLCTL_USERNAME)
              OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
              OWLCTL_URL: $(OWLCTL_URL)

          - script: |
              CRITICAL=0
              for CMD in \
                "job diff --all --security-only" \
                "repo diff --all --security-only" \
                "repo sobr-diff --all --security-only" \
                "encryption diff --all --security-only" \
                "encryption kms-diff --all --security-only"
              do
                ./owlctl $CMD 2>&1 || true
                if [ $? -eq 4 ]; then CRITICAL=1; fi
              done
              echo "##vso[task.setvariable variable=hasCritical;isOutput=true]$CRITICAL"
              if [ $CRITICAL -eq 1 ]; then
                echo "##vso[build.addbuildtag]critical-drift"
              fi
            displayName: 'Scan for drift'
            name: scan
            env:
              OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)

  - stage: Remediate
    displayName: 'Auto-Remediate'
    dependsOn: Detect
    condition: eq(dependencies.Detect.outputs['DriftScan.scan.hasCritical'], '1')
    jobs:
      - job: ApplySpecs
        steps:
          - checkout: self

          - template: examples/pipelines/templates/install-owlctl.yml

          - script: |
              ./owlctl init && ./owlctl profile --set vbr && ./owlctl login
            displayName: 'Setup'
            env:
              OWLCTL_USERNAME: $(OWLCTL_USERNAME)
              OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
              OWLCTL_URL: $(OWLCTL_URL)

          - script: |
              PARTIAL=0

              # Apply all specs from Git
              for spec in infrastructure/repos/*.yaml; do
                ./owlctl repo apply "$spec"
                if [ $? -eq 5 ]; then PARTIAL=1; fi
              done

              for spec in infrastructure/sobrs/*.yaml; do
                ./owlctl repo sobr-apply "$spec"
                if [ $? -eq 5 ]; then PARTIAL=1; fi
              done

              for spec in infrastructure/jobs/*.yaml; do
                ./owlctl job apply "$spec"
                if [ $? -eq 5 ]; then PARTIAL=1; fi
              done

              if [ $PARTIAL -eq 1 ]; then
                echo "##vso[task.logissue type=warning]Partial remediation — some fields rejected by VBR"
                echo "##vso[task.complete result=SucceededWithIssues;]Partial remediation"
              fi
            displayName: 'Apply desired state'
            env:
              OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)

  - stage: Verify
    displayName: 'Post-Remediation Check'
    dependsOn: Remediate
    jobs:
      - job: VerifyClean
        steps:
          - template: examples/pipelines/templates/install-owlctl.yml

          - script: |
              ./owlctl init && ./owlctl profile --set vbr && ./owlctl login
            displayName: 'Setup'
            env:
              OWLCTL_USERNAME: $(OWLCTL_USERNAME)
              OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
              OWLCTL_URL: $(OWLCTL_URL)

          - script: |
              ./owlctl job diff --all
              ./owlctl repo diff --all
              ./owlctl repo sobr-diff --all
              ./owlctl encryption diff --all
              ./owlctl encryption kms-diff --all
            displayName: 'Verify no remaining drift'
            env:
              OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)

  - stage: Notify
    displayName: 'Notifications'
    dependsOn:
      - Remediate
      - Verify
    condition: |
      or(
        failed(),
        eq(dependencies.Remediate.result, 'SucceededWithIssues')
      )
    jobs:
      - job: Alert
        steps:
          - script: |
              curl -X POST -H 'Content-type: application/json' \
                --data '{
                  "text": "VBR drift detected and remediation attempted. Some fields may require manual intervention. Pipeline: $(Build.DefinitionName) Build: $(Build.BuildNumber)"
                }' \
                $(SLACK_WEBHOOK_URL)
            displayName: 'Notify team'
            env:
              SLACK_WEBHOOK_URL: $(SLACK_WEBHOOK_URL)
```

### Apply Exit Codes

With Epic #42, apply commands return additional exit codes that pipelines can use:

| Exit Code | Command | Meaning | Pipeline Action |
|-----------|---------|---------|-----------------|
| `0` | diff | No drift | Pass |
| `0` | apply | All fields accepted by VBR | Pass |
| `3` | diff | INFO/WARNING drift | SucceededWithIssues |
| `4` | diff | CRITICAL drift | Fail — trigger remediation |
| `5` | apply | Partial — some fields accepted, some rejected by VBR | SucceededWithIssues — notify team |
| `6` | apply | Resource destroyed — deleted from VBR | Fail — manual recreation needed |
| `1` | any | Error | Fail — investigate |

### API-First Remediation

owlctl follows the Kubernetes/Terraform model for remediation: **send all changes to VBR and let the API decide what's mutable**. Apply sends the desired state from the Git-tracked YAML spec, and VBR accepts or rejects each field. owlctl reports the outcome:

```
Applying: Default Backup Repository
  Applied: maxTaskCount: 1 -> 4
  Applied: description: "old" -> "updated"
  Rejected by VBR: type: WinLocal -> LinuxHardened
    (field is immutable after creation)

2 fields applied, 1 field rejected.
```

This avoids maintaining field-level mutability maps that must stay in sync across VBR versions. The API is the authority on what can be changed. An optional `remediation-config.yaml` can pre-filter fields to suppress known failures, but it's not required.

### Full Resource Coverage

With Epic #23 Phases 2-4 complete, drift scans and remediation cover the entire VBR security surface:

```yaml
steps:
  - script: |
      # Current
      ./owlctl job diff --all --security-only
      ./owlctl repo diff --all --security-only
      ./owlctl repo sobr-diff --all --security-only
      ./owlctl encryption diff --all --security-only
      ./owlctl encryption kms-diff --all --security-only
      ./owlctl config-backup diff --security-only      # Singleton — no --all flag

      # Epic #23 Phase 2 (planned — #29, #30, #31, #32)
      ./owlctl rbac diff --all --security-only
      ./owlctl infra servers diff --all --security-only
      ./owlctl credentials diff --all --security-only
      ./owlctl traffic diff --all --security-only

      # Epic #23 Phase 3 (planned — #33, #34, #35)
      ./owlctl malware diff --all --security-only
      ./owlctl notifications diff --all --security-only
    displayName: 'Full security scan'
```

### Deployment Gate with Remediation

A multi-stage pipeline that validates before deploying, applies changes, and verifies afterward:

```yaml
stages:
  - stage: PreDeployCheck
    displayName: 'Pre-Deploy Compliance Check'
    jobs:
      - job: DriftGate
        steps:
          - template: templates/drift-scan.yml
            parameters:
              severity: 'critical'
              failOnDrift: true

  - stage: Deploy
    displayName: 'Deploy Configuration'
    dependsOn: PreDeployCheck
    condition: succeeded()
    jobs:
      - job: ApplyConfig
        steps:
          - script: |
              # Apply all resource specs from Git
              for spec in infrastructure/jobs/*.yaml; do
                ./owlctl job apply "$spec"
              done
              for spec in infrastructure/repos/*.yaml; do
                ./owlctl repo apply "$spec"
              done
            displayName: 'Apply VBR configuration'
            env:
              OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)

  - stage: PostDeployVerify
    displayName: 'Post-Deploy Verification'
    dependsOn: Deploy
    jobs:
      - job: VerifyNoDrift
        steps:
          - template: templates/drift-scan.yml
            parameters:
              severity: 'warning'
              failOnDrift: false
```

### JSON Output for Machine Consumption

**Planned feature:** `--output json` flag on all diff commands

With JSON output, pipelines can parse results programmatically:

```yaml
steps:
  - script: |
      ./owlctl job diff --all --output json > job-drift.json
      ./owlctl repo diff --all --output json > repo-drift.json

      # Parse with jq for conditional logic
      CRITICAL_COUNT=$(jq '[.drifts[] | select(.severity == "CRITICAL")] | length' job-drift.json)
      echo "##vso[task.setvariable variable=criticalCount]$CRITICAL_COUNT"
    displayName: 'Structured drift scan'
```

### Markdown Reports Attached to Build Summary

**Required feature:** `--output markdown` flag or `owlctl report` command (#38)

Azure DevOps can render Markdown reports directly in the build summary tab:

```yaml
steps:
  - script: |
      ./owlctl report --format markdown > $(System.DefaultWorkingDirectory)/drift-summary.md
      echo "##vso[task.uploadsummary]$(System.DefaultWorkingDirectory)/drift-summary.md"
    displayName: 'Generate compliance report'
```

### Multi-Resource Correlation

**Required feature:** Correlation engine (#36)

```yaml
steps:
  - script: |
      ./owlctl correlate --all --output json > correlation.json
      PATTERNS=$(jq '.patterns | length' correlation.json)
      if [ $PATTERNS -gt 0 ]; then
        echo "##vso[task.logissue type=error]$PATTERNS coordinated change patterns detected"
        echo "##vso[build.addbuildtag]correlated-drift"
        exit 1
      fi
    displayName: 'Check for coordinated changes'
```

### Dashboard Integration

Azure DevOps dashboards can display drift trends using several approaches:

**Build History Widget** — Shows pass/fail trend for the drift scan pipeline. A failing build = drift detected.

**Test Results Trend** — If owlctl outputs JUnit XML format in a future release, the Test Results Trend widget would show drift counts over time.

**Wiki Auto-Update** — Pipeline publishes the latest report to an Azure DevOps wiki page:

```yaml
steps:
  - script: |
      REPORT=$(./owlctl report --format markdown)
      az devops configure --defaults \
        organization=$(System.CollectionUri) \
        project="$(System.TeamProject)"
      echo "$REPORT" | az repos wiki page update \
        --wiki "project-wiki" \
        --path "/Compliance/VBR-Drift-Report" \
        --content "$REPORT"
    displayName: 'Update wiki dashboard'
    env:
      AZURE_DEVOPS_EXT_PAT: $(System.AccessToken)
```

---

## Reference: Pipeline Templates

Complete, ready-to-use pipeline templates are available in [`examples/pipelines/`](../examples/pipelines/). The templates below show reusable steps that can be embedded in your own pipelines.

### Template: Reusable Drift Scan Step

Create a template that can be included in any pipeline:

```yaml
# templates/drift-scan.yml
parameters:
  - name: severity
    type: string
    default: 'critical'
  - name: failOnDrift
    type: boolean
    default: true

steps:
  - script: |
      CRITICAL=0

      for CMD in \
        "job diff --all --severity ${{ parameters.severity }}" \
        "repo diff --all --severity ${{ parameters.severity }}" \
        "repo sobr-diff --all --severity ${{ parameters.severity }}" \
        "encryption diff --all --severity ${{ parameters.severity }}" \
        "encryption kms-diff --all --severity ${{ parameters.severity }}"
      do
        ./owlctl $CMD 2>&1 || true
        if [ $? -eq 4 ]; then
          CRITICAL=1
        fi
      done

      if [ $CRITICAL -eq 1 ] && [ "${{ parameters.failOnDrift }}" = "True" ]; then
        echo "##vso[task.logissue type=error]Critical drift detected"
        exit 1
      fi
    displayName: 'owlctl drift scan'
    env:
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
```

Use in a pipeline:

```yaml
steps:
  - template: templates/drift-scan.yml
    parameters:
      severity: 'warning'
      failOnDrift: true
```

### Template: Remediate from Git Specs

A reusable step that applies all YAML specs from a directory structure:

```yaml
# templates/remediate.yml
parameters:
  - name: specsPath
    type: string
    default: 'infrastructure'

steps:
  - script: |
      PARTIAL=0
      DESTROYED=0

      # Apply job specs
      for spec in ${{ parameters.specsPath }}/jobs/*.yaml; do
        [ -f "$spec" ] || continue
        ./owlctl job apply "$spec"
        EXIT=$?
        if [ $EXIT -eq 5 ]; then PARTIAL=1; fi
        if [ $EXIT -eq 6 ]; then DESTROYED=1; fi
      done

      # Apply repo specs
      for spec in ${{ parameters.specsPath }}/repos/*.yaml; do
        [ -f "$spec" ] || continue
        ./owlctl repo apply "$spec"
        EXIT=$?
        if [ $EXIT -eq 5 ]; then PARTIAL=1; fi
        if [ $EXIT -eq 6 ]; then DESTROYED=1; fi
      done

      # Apply SOBR specs
      for spec in ${{ parameters.specsPath }}/sobrs/*.yaml; do
        [ -f "$spec" ] || continue
        ./owlctl repo sobr-apply "$spec"
        EXIT=$?
        if [ $EXIT -eq 5 ]; then PARTIAL=1; fi
        if [ $EXIT -eq 6 ]; then DESTROYED=1; fi
      done

      if [ $DESTROYED -eq 1 ]; then
        echo "##vso[task.logissue type=error]One or more resources deleted from VBR — manual recreation required"
        exit 1
      fi

      if [ $PARTIAL -eq 1 ]; then
        echo "##vso[task.logissue type=warning]Partial remediation — some fields rejected by VBR API"
        echo "##vso[task.complete result=SucceededWithIssues;]Partial remediation"
      fi
    displayName: 'Apply desired state from Git'
    env:
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
```

### Template: Nightly Compliance Report

```yaml
# azure-pipelines-nightly-compliance.yml
trigger: none

schedules:
  - cron: "0 2 * * *"
    displayName: "Nightly 2AM UTC"
    branches:
      include:
        - master
    always: true

pool:
  vmImage: 'ubuntu-latest'

variables:
  - group: 'veeam-credentials'

steps:
  - template: examples/pipelines/templates/install-owlctl.yml

  - script: |
      ./owlctl init && ./owlctl profile --set vbr && ./owlctl login
    displayName: 'Setup'
    env:
      OWLCTL_USERNAME: $(OWLCTL_USERNAME)
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
      OWLCTL_URL: $(OWLCTL_URL)

  - script: |
      # Capture all drift output
      {
        echo "# VBR Compliance Report"
        echo "**Generated:** $(date -u '+%Y-%m-%d %H:%M UTC')"
        echo "**Pipeline:** $(Build.DefinitionName) #$(Build.BuildNumber)"
        echo ""

        echo "## Job Configuration"
        ./owlctl job diff --all 2>&1 || true
        echo ""

        echo "## Repository Configuration"
        ./owlctl repo diff --all 2>&1 || true
        echo ""

        echo "## Scale-Out Repositories"
        ./owlctl repo sobr-diff --all 2>&1 || true
        echo ""

        echo "## Encryption Inventory"
        ./owlctl encryption diff --all 2>&1 || true
        echo ""

        echo "## KMS Servers"
        ./owlctl encryption kms-diff --all 2>&1 || true
      } > $(Build.ArtifactStagingDirectory)/compliance-report.md

      echo "##vso[task.uploadsummary]$(Build.ArtifactStagingDirectory)/compliance-report.md"
    displayName: 'Generate compliance report'
    env:
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)

  - publish: $(Build.ArtifactStagingDirectory)
    artifact: ComplianceReport
    displayName: 'Publish report'
    condition: always()
```
