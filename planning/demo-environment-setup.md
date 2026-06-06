# Demo Environment Setup: Azure DevOps + owlctl End-to-End

This document walks through setting up an Azure DevOps environment that demonstrates owlctl's full declarative workflow — from individual job management to group-based policy enforcement — using the groups/targets/profiles model.

## Prerequisites

- Azure DevOps organization (free tier works)
- VBR server accessible from your machine (or Docker host)
- Docker and Docker Compose installed (for self-hosted agent)
- Go installed locally (for initial bootstrap)
- owlctl v1.1.0+ binary (or built from source)

## Step 1: Create the Azure DevOps Project

1. Go to `https://dev.azure.com/yourorg`
2. **+ New project**
   - Name: `vbr-infrastructure`
   - Visibility: Private
   - Version control: Git
   - Work item process: Basic (or your preference)

## Step 2: Create Environments

Azure DevOps Environments give you approval gates, deployment history, exclusive locks (preventing concurrent applies to the same VBR), and a clear audit trail of what was deployed where and when.

### Create two environments

Go to **Pipelines > Environments** and create:

| Environment | Purpose |
|-------------|---------|
| `vbr-primary` | Primary production VBR — requires approval, exclusive lock |
| `vbr-dr` | Disaster recovery VBR — requires approval, exclusive lock |

For each environment:
1. Click **New environment**
2. Name it (e.g., `vbr-primary`)
3. Resource: **None** (owlctl handles connectivity directly)
4. Click **Create**

### Configure checks on each environment

**vbr-primary:**
1. Click `vbr-primary` > **Approvals and checks**
2. **+ Add check > Approvals** — add the required approvers
3. **+ Add check > Exclusive lock** — prevents two deployment runs from applying to production simultaneously. This is important because concurrent `owlctl apply` runs against the same VBR can cause race conditions in state management.

**vbr-dr:**
1. Click `vbr-dr` > **Approvals and checks**
2. **+ Add check > Approvals** — add the required approvers
3. **+ Add check > Exclusive lock**

### Why exclusive locks matter

Without an exclusive lock, two pipelines could simultaneously apply different specs to the same VBR server. The second apply might overwrite the first, and `state.json` would reflect whichever pipeline committed last. The exclusive lock serializes deployments per environment so this can't happen.

## Step 3: Create Variable Groups (per target)

Create one variable group per owlctl target. This ensures each pipeline stage connects to the correct VBR server with the correct credentials.

Go to **Pipelines > Library > + Variable group** and create:

### veeam-credentials-primary

| Variable | Value | Secret? |
|----------|-------|---------|
| `OWLCTL_USERNAME` | Primary VBR username | No |
| `OWLCTL_PASSWORD` | Primary VBR password | **Yes** |
| `OWLCTL_URL` | `https://vbr-primary.example.com:9419` | No |

### veeam-credentials-dr

| Variable | Value | Secret? |
|----------|-------|---------|
| `OWLCTL_USERNAME` | DR VBR username | No |
| `OWLCTL_PASSWORD` | DR VBR password | **Yes** |
| `OWLCTL_URL` | `https://vbr-dr.example.com:9419` | No |

For production, consider linking to Azure Key Vault instead of storing secrets directly in the variable group.

### Grant environment access to variable groups

Each variable group needs to be authorized for use in the pipelines. Under the variable group's **Pipeline permissions** tab, allow access to the relevant pipelines (or allow all pipelines during initial setup).

## Step 4: Initialize the Git Repository

Clone the new repo and set up the directory structure:

```bash
git clone https://dev.azure.com/yourorg/vbr-infrastructure/_git/vbr-infrastructure
cd vbr-infrastructure

# Create directory structure
mkdir -p .owlctl
mkdir -p specs/{jobs,repos,sobrs,kms,encryption}
mkdir -p profiles
mkdir -p overlays
mkdir -p scripts
mkdir -p pipelines
```

### Create .gitignore

```bash
cat > .gitignore << 'EOF'
# owlctl state (committed for production audit trail)
# state.json

# Credentials
.env
credentials.json
*.key
*.pem

# owlctl auth artifacts
headers.json

# Local development
.DS_Store
Thumbs.db
*.swp
*~

# IDE
.vscode/
.idea/

# Logs
*.log

# owlctl binary
owlctl
owlctl.exe
EOF
```

### Create owlctl.yaml

```bash
cat > owlctl.yaml << 'EOF'
apiVersion: owlctl.veeam.com/v1
kind: Config

# Groups bundle specs with a shared profile and overlay for batch operations.
# Apply a group: owlctl job apply --group db-tier
# Diff a group:  owlctl job diff --group db-tier
groups:
  db-tier:
    description: Database backup group — gold-tier retention and compliance policy
    profile: profiles/gold.yaml
    overlay: overlays/compliance.yaml
    specs:
      - specs/jobs/sql-vm-01.yaml
      - specs/jobs/sql-vm-02.yaml

  web-tier:
    description: Web server backups — standard retention
    profile: profiles/standard.yaml
    specs:
      - specs/jobs/web-frontend.yaml
      - specs/jobs/web-api.yaml

# Targets define named VBR server connections.
# Use --target to switch: owlctl job apply --group db-tier --target primary
targets:
  primary:
    url: https://vbr-primary.example.com:9419
    description: Primary production VBR server

  dr:
    url: https://vbr-dr.example.com:9419
    description: Disaster recovery site
EOF
```

**Key concepts:**
- **Groups** bundle related specs with an optional shared profile and overlay. `owlctl job apply --group db-tier` applies all specs in the group with the 3-way merge (profile → spec → overlay).
- **Targets** name VBR server connections. `--target primary` overrides `OWLCTL_URL` so the same group definition can be applied to multiple VBR servers.
- **Profiles** (`kind: Profile`) set organizational defaults — the base layer.
- **Overlays** (`kind: Overlay`) enforce policy — the top layer that wins over everything.

### Initialize owlctl config files

```bash
# Point owlctl config to the .owlctl directory within the repo
export OWLCTL_SETTINGS_PATH=./.owlctl/

owlctl init
owlctl profile --set vbr
```

This creates `.owlctl/settings.json` and `.owlctl/profiles.json`. Both are safe to commit (no credentials).

### Create helper scripts

**scripts/apply-group.sh:**
```bash
cat > scripts/apply-group.sh << 'SCRIPT'
#!/bin/bash
set -e

GROUP="${1:?Usage: apply-group.sh <group-name>}"

echo "=== Applying group: $GROUP ==="
owlctl job apply --group "$GROUP"

echo "=== Applying non-job resources ==="
for spec in specs/repos/*.yaml; do
    [ -f "$spec" ] || continue
    echo "Applying: $spec"
    owlctl repo apply "$spec"
done

for spec in specs/sobrs/*.yaml; do
    [ -f "$spec" ] || continue
    echo "Applying: $spec"
    owlctl repo sobr-apply "$spec"
done

for spec in specs/kms/*.yaml; do
    [ -f "$spec" ] || continue
    echo "Applying: $spec"
    owlctl encryption kms-apply "$spec"
done

echo "=== Done ==="
SCRIPT
chmod +x scripts/apply-group.sh
```

**scripts/snapshot-all.sh:**
```bash
cat > scripts/snapshot-all.sh << 'SCRIPT'
#!/bin/bash
set -e

echo "=== Snapshotting all resources ==="
owlctl job snapshot --all
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all
owlctl encryption snapshot --all
owlctl encryption kms-snapshot --all
echo "=== Done ==="
SCRIPT
chmod +x scripts/snapshot-all.sh
```

### Initial commit

```bash
git add .owlctl/settings.json .owlctl/profiles.json
git add .gitignore owlctl.yaml scripts/ specs/ profiles/ overlays/
git commit -m "Initialize vbr-infrastructure repo structure"
git push
```

## Step 5: Bootstrap from Live VBR

Connect to your VBR server and export the current state:

```bash
export OWLCTL_SETTINGS_PATH=./.owlctl/
export OWLCTL_USERNAME="your-username"
export OWLCTL_PASSWORD="your-password"
export OWLCTL_URL="https://vbr-primary.example.com:9419"

owlctl login

# Export all backup jobs as YAML specs
owlctl job export --all -d specs/jobs/

# Snapshot all resource types into state.json
owlctl job snapshot --all
owlctl repo snapshot --all
owlctl repo sobr-snapshot --all
owlctl encryption snapshot --all
owlctl encryption kms-snapshot --all

# Export repos and SOBRs if you want YAML specs for them too
owlctl repo export --all -d specs/repos/
owlctl repo sobr-export --all -d specs/sobrs/
owlctl encryption kms-export --all -d specs/kms/

# Commit the baseline
git add specs/ .owlctl/state.json
git commit -m "Bootstrap: capture current VBR state"
git push
```

### Create a profile and an overlay

Profiles set organizational defaults that apply to every spec in a group. Overlays enforce policy overrides that take the highest precedence.

**profiles/gold.yaml** (kind: Profile — base defaults):
```yaml
apiVersion: owlctl.veeam.com/v1
kind: Profile
metadata:
  name: gold-standard
  labels:
    tier: gold
    managed-by: owlctl
spec:
  description: Gold standard backup policy
  storage:
    retentionPolicy:
      type: Days
      quantity: 14
    advancedSettings:
      storageData:
        compressionLevel: Optimal
        enableInlineDataDedup: false
      backupModeType: Incremental
  schedule:
    daily:
      isEnabled: true
      localTime: "22:00"
      dailyKind: Everyday
    retry:
      isEnabled: true
      retryCount: 3
      awaitMinutes: 10
    runAutomatically: false
```

**profiles/standard.yaml** (kind: Profile — lighter defaults):
```yaml
apiVersion: owlctl.veeam.com/v1
kind: Profile
metadata:
  name: standard
  labels:
    tier: standard
    managed-by: owlctl
spec:
  storage:
    retentionPolicy:
      type: Days
      quantity: 7
    advancedSettings:
      storageData:
        compressionLevel: Optimal
      backupModeType: Incremental
  schedule:
    daily:
      isEnabled: true
      localTime: "23:00"
      dailyKind: Everyday
    retry:
      isEnabled: true
      retryCount: 2
      awaitMinutes: 10
```

**overlays/compliance.yaml** (kind: Overlay — policy enforcement):
```yaml
apiVersion: owlctl.veeam.com/v1
kind: Overlay
metadata:
  name: compliance-patch
  labels:
    compliance: internal-policy
spec:
  storage:
    retentionPolicy:
      quantity: 30
```

**How the 3-way merge works:**

For a spec in the `db-tier` group:
1. **Profile** (`profiles/gold.yaml`) sets retention to 14 days, compression to Optimal, schedule at 22:00
2. **Spec** (`specs/jobs/sql-vm-01.yaml`) provides identity (name, VM targets, repository) — overrides profile where they overlap
3. **Overlay** (`overlays/compliance.yaml`) bumps retention to 30 days — wins over both profile and spec

The spec's `metadata.name` is always preserved from the spec file, never from profile or overlay. Labels merge additively across all layers.

Commit the profiles and overlay:

```bash
git add profiles/ overlays/
git commit -m "Add gold/standard profiles and compliance overlay"
git push
```

## Step 6: Set Up the Self-Hosted Agent (if VBR is not internet-accessible)

If your VBR server is on a private network, use the Docker-based self-hosted agent from `docker/azure-agent/`.

### Create a PAT

1. Azure DevOps > Profile icon > **Personal access tokens**
2. **+ New Token**
   - Name: `owlctl-demo-agent`
   - Scopes: **Agent Pools (Read & Manage)**
3. Copy the token

### Start the agent

```bash
cd /path/to/owlctl/docker/azure-agent

cp .env.example .env
# Edit .env with your values:
#   AZP_URL=https://dev.azure.com/yourorg
#   AZP_TOKEN=your-pat-here
#   AZP_POOL=Default
#   OWLCTL_USERNAME=your-username
#   OWLCTL_PASSWORD=your-password
#   OWLCTL_URL=https://vbr-primary.example.com:9419

docker compose build
docker compose up -d
```

Verify the agent shows as online:
**Project Settings > Agent pools > Default > Agents**

### Pool configuration for pipelines

If using a self-hosted agent:
```yaml
pool:
  name: 'Default'  # Or your custom pool name
```

If using Microsoft-hosted agents:
```yaml
pool:
  vmImage: 'ubuntu-latest'
```

## Step 7: Add Pipeline Definitions

### pipelines/deployment.yml (group-based deployment)

This is the main deployment pipeline. It applies groups to the primary VBR target, using the Azure DevOps Environment for approval gates and exclusive locks.

```yaml
trigger:
  branches:
    include: [master]
  paths:
    include: ['specs/**', 'profiles/**', 'overlays/**', 'owlctl.yaml']

pool:
  name: 'Default'  # Or vmImage: 'ubuntu-latest'

stages:
  - stage: DeployPrimary
    displayName: 'Deploy to Primary VBR'
    jobs:
      - deployment: deploy_primary
        displayName: 'Apply groups to primary VBR'
        environment: 'vbr-primary'         # Requires approval + exclusive lock
        variables:
          - group: 'veeam-credentials-primary'
        strategy:
          runOnce:
            deploy:
              steps:
                - checkout: self

                - script: |
                    curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
                    tar xzf owlctl.tar.gz
                    chmod +x owlctl
                    export OWLCTL_SETTINGS_PATH=./.owlctl/
                    ./owlctl profile --set vbr
                    ./owlctl login

                    echo "=== Applying db-tier group ==="
                    ./owlctl job apply --group db-tier

                    echo "=== Applying web-tier group ==="
                    ./owlctl job apply --group web-tier

                    echo "=== Applying non-job resources ==="
                    for spec in specs/repos/*.yaml; do
                      [ -f "$spec" ] || continue
                      ./owlctl repo apply "$spec"
                    done

                    for spec in specs/sobrs/*.yaml; do
                      [ -f "$spec" ] || continue
                      ./owlctl repo sobr-apply "$spec"
                    done

                    for spec in specs/kms/*.yaml; do
                      [ -f "$spec" ] || continue
                      ./owlctl encryption kms-apply "$spec"
                    done
                  displayName: 'Apply groups to primary VBR'
                  env:
                    OWLCTL_USERNAME: $(OWLCTL_USERNAME)
                    OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
                    OWLCTL_URL: $(OWLCTL_URL)

            postRouteTraffic:
              steps:
                - script: |
                    export OWLCTL_SETTINGS_PATH=./.owlctl/
                    ./owlctl job diff --group db-tier --security-only
                    DB_EXIT=$?
                    ./owlctl job diff --group web-tier --security-only
                    WEB_EXIT=$?
                    if [ $DB_EXIT -eq 4 ] || [ $WEB_EXIT -eq 4 ]; then
                      echo "##vso[task.logissue type=error]CRITICAL drift after deployment"
                      exit 1
                    fi
                  displayName: 'Post-deploy drift check'
                  env:
                    OWLCTL_USERNAME: $(OWLCTL_USERNAME)
                    OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
                    OWLCTL_URL: $(OWLCTL_URL)

  - stage: DeployDR
    displayName: 'Deploy to DR VBR'
    dependsOn: DeployPrimary
    jobs:
      - deployment: deploy_dr
        displayName: 'Apply groups to DR VBR'
        environment: 'vbr-dr'              # Requires approval + exclusive lock
        variables:
          - group: 'veeam-credentials-dr'
        strategy:
          runOnce:
            deploy:
              steps:
                - checkout: self

                - script: |
                    curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
                    tar xzf owlctl.tar.gz
                    chmod +x owlctl
                    export OWLCTL_SETTINGS_PATH=./.owlctl/
                    ./owlctl profile --set vbr
                    ./owlctl login

                    echo "=== Applying db-tier group ==="
                    ./owlctl job apply --group db-tier

                    echo "=== Applying web-tier group ==="
                    ./owlctl job apply --group web-tier
                  displayName: 'Apply groups to DR VBR'
                  env:
                    OWLCTL_USERNAME: $(OWLCTL_USERNAME)
                    OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
                    OWLCTL_URL: $(OWLCTL_URL)
```

**What the environments give you here:**
- `vbr-primary` pauses for mandatory approval; the exclusive lock ensures only one deployment runs at a time
- `vbr-dr` deploys after primary succeeds, with its own approval gate
- Each stage in the Azure DevOps UI shows deployment history: what commit was deployed, when, and by whom

### pipelines/detect-remediate.yml (group-aware drift scan)

```yaml
trigger: none

schedules:
  - cron: "0 6 * * 1-5"
    displayName: "Weekday 6AM UTC drift scan"
    branches:
      include: [master]
    always: true

pool:
  name: 'Default'

stages:
  - stage: DriftScan
    displayName: 'Detect drift in primary VBR'
    jobs:
      - deployment: scan_primary
        displayName: 'Scan primary VBR'
        environment: 'vbr-primary'         # Exclusive lock prevents scan during deploy
        variables:
          - group: 'veeam-credentials-primary'
        strategy:
          runOnce:
            deploy:
              steps:
                - checkout: self

                - script: |
                    curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
                    tar xzf owlctl.tar.gz
                    chmod +x owlctl
                    export OWLCTL_SETTINGS_PATH=./.owlctl/
                    ./owlctl profile --set vbr
                    ./owlctl login

                    CRITICAL=0

                    # Group-aware job drift detection
                    echo "=== Checking db-tier group ==="
                    ./owlctl job diff --group db-tier --security-only || EXIT_CODE=$?
                    if [ "${EXIT_CODE:-0}" -eq 4 ]; then
                      CRITICAL=1
                      echo "##vso[task.logissue type=error]CRITICAL drift in db-tier group"
                    fi
                    EXIT_CODE=0

                    echo "=== Checking web-tier group ==="
                    ./owlctl job diff --group web-tier --security-only || EXIT_CODE=$?
                    if [ "${EXIT_CODE:-0}" -eq 4 ]; then
                      CRITICAL=1
                      echo "##vso[task.logissue type=error]CRITICAL drift in web-tier group"
                    fi
                    EXIT_CODE=0

                    # Non-job resource drift detection
                    for CMD in \
                      "repo diff --all --security-only" \
                      "repo sobr-diff --all --security-only" \
                      "encryption diff --all --security-only" \
                      "encryption kms-diff --all --security-only"
                    do
                      ./owlctl $CMD || EXIT_CODE=$?
                      if [ "${EXIT_CODE:-0}" -eq 4 ]; then
                        CRITICAL=1
                        echo "##vso[task.logissue type=error]CRITICAL drift in: $CMD"
                      fi
                      EXIT_CODE=0
                    done

                    if [ $CRITICAL -eq 1 ]; then
                      echo "##vso[build.addbuildtag]critical-drift"
                      exit 1
                    fi
                  displayName: 'Run drift scan'
                  env:
                    OWLCTL_USERNAME: $(OWLCTL_USERNAME)
                    OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
                    OWLCTL_URL: $(OWLCTL_URL)
```

**Why use the environment here:** The exclusive lock on `vbr-primary` means a drift scan won't run while a deployment is in progress (and vice versa). Without this, a drift scan mid-deploy would report false positives.

### pipelines/pr-validation.yml (group dry-run)

PR validation runs a dry-run of all groups to validate spec changes before merge.

```yaml
trigger: none

pr:
  branches:
    include: [master]
  paths:
    include: ['specs/**', 'profiles/**', 'overlays/**', 'owlctl.yaml']

pool:
  name: 'Default'

stages:
  - stage: Validate
    displayName: 'Validate PR against primary VBR'
    jobs:
      - deployment: validate
        displayName: 'Dry-run all groups'
        environment: 'vbr-primary'
        variables:
          - group: 'veeam-credentials-primary'
        strategy:
          runOnce:
            deploy:
              steps:
                - checkout: self

                - script: |
                    curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
                    tar xzf owlctl.tar.gz
                    chmod +x owlctl
                    export OWLCTL_SETTINGS_PATH=./.owlctl/
                    ./owlctl profile --set vbr
                    ./owlctl login

                    FAILED=0

                    echo "=== Validating db-tier group ==="
                    ./owlctl job apply --group db-tier --dry-run || FAILED=$((FAILED + 1))

                    echo "=== Validating web-tier group ==="
                    ./owlctl job apply --group web-tier --dry-run || FAILED=$((FAILED + 1))

                    echo "=== Validating non-job resources ==="
                    for spec in specs/repos/*.yaml; do
                      [ -f "$spec" ] || continue
                      echo "Validating: $spec"
                      ./owlctl repo apply "$spec" --dry-run || FAILED=$((FAILED + 1))
                    done

                    if [ $FAILED -gt 0 ]; then
                      echo "##vso[task.logissue type=error]$FAILED validation(s) failed"
                      exit 1
                    fi
                  displayName: 'Dry-run all groups and specs'
                  env:
                    OWLCTL_USERNAME: $(OWLCTL_USERNAME)
                    OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
                    OWLCTL_URL: $(OWLCTL_URL)
```

### pipelines/bootstrap.yml and pipelines/nightly-compliance.yml

Copy these from `examples/pipelines/` and update:
- `pool` to match your agent setup
- Variable group to `veeam-credentials-primary`
- Add `environment: 'vbr-primary'` with a `deployment` job if you want them to respect the exclusive lock (recommended for bootstrap since it writes state)

Commit the pipelines:

```bash
git add pipelines/
git commit -m "Add Azure DevOps pipeline definitions with group-based deployment"
git push
```

## Step 8: Create Pipelines in Azure DevOps

For each pipeline file:

1. **Pipelines > New pipeline**
2. Select **Azure Repos Git**
3. Select the `vbr-infrastructure` repo
4. Choose **Existing Azure Pipelines YAML file**
5. Select the path (e.g., `pipelines/deployment.yml`)
6. **Save** (or **Run** for bootstrap)

### Pipeline-specific setup

**pr-validation.yml — Add as branch policy:**
1. **Repos > Branches > master > Branch policies**
2. Under **Build Validation**, click **+**
3. Select the PR validation pipeline
4. Set to **Required** and **Immediately when updated**

**detect-remediate.yml — Verify schedule:**
The schedule is defined in YAML. After creating the pipeline, check:
**Pipeline > Triggers > Scheduled** — should show the cron schedule.

### Verify environment wiring

After running a pipeline once, check:
1. **Pipelines > Environments > vbr-primary**
2. You should see deployment history entries
3. The **Approvals and checks** tab shows the approval gate and exclusive lock

## Step 9: Demo Walkthrough

With everything set up, the demo follows a 4-act structure: individual management, group policy, CI/CD automation, and multi-target deployment.

### Act 1: Baseline — Individual Job Management

This act demonstrates managing individual jobs with owlctl — export, detect drift, remediate.

**1. Export two real jobs from VBR:**

```bash
export OWLCTL_SETTINGS_PATH=./.owlctl/
export OWLCTL_USERNAME="your-username"
export OWLCTL_PASSWORD="your-password"
export OWLCTL_URL="https://vbr-primary.example.com:9419"
owlctl login

# Export two specific jobs
owlctl job export <sql-vm-01-id> -o specs/jobs/sql-vm-01.yaml
owlctl job export <sql-vm-02-id> -o specs/jobs/sql-vm-02.yaml

# Snapshot them into state
owlctl job snapshot --all
```

**2. Show the YAML specs:**

```bash
cat specs/jobs/sql-vm-01.yaml
```

Point out the key fields: `metadata.name`, `spec.storage.retentionPolicy`, `spec.schedule`, `spec.virtualMachines`.

**3. Introduce drift — manually change something in VBR console:**

Open the VBR console and reduce retention from 14 days to 3 days on one of the jobs. This simulates an unauthorized manual change.

**4. Detect drift:**

```bash
owlctl job diff sql-vm-01
```

Expected output shows:
- CRITICAL severity: retention reduced (14 → 3)
- The security summary header: `CRITICAL: 1 security-relevant change detected`

**5. Remediate:**

```bash
owlctl job apply specs/jobs/sql-vm-01.yaml
```

The job is restored to the declared state. Run diff again to confirm clean:

```bash
owlctl job diff sql-vm-01
# Exit code 0 — no drift
```

**Takeaway:** Individual management works, but you're managing files one at a time. With 20+ jobs, this gets tedious. That's where groups come in.

---

### Act 2: Policy Overlay — Group Management

This act demonstrates how groups enforce consistent policy across multiple jobs with the 3-way merge.

**6. Show the profile (already created in Step 5):**

```bash
cat profiles/gold.yaml
```

Explain: "This `kind: Profile` sets organizational defaults — 14-day retention, Optimal compression, daily schedule at 22:00. It's the base layer."

**7. Show the overlay (already created in Step 5):**

```bash
cat overlays/compliance.yaml
```

Explain: "This `kind: Overlay` bumps retention to 30 days. It's the top layer — it wins over both profile and spec. This is how you enforce compliance policy without editing individual specs."

**8. Show the group definition in owlctl.yaml:**

```bash
cat owlctl.yaml
```

Point out the `db-tier` group: it bundles `sql-vm-01.yaml` and `sql-vm-02.yaml` with `profiles/gold.yaml` and `overlays/compliance.yaml`.

**9. Preview what the group will apply:**

```bash
owlctl job apply --group db-tier --dry-run
```

This shows the merged result for each spec in the group without applying anything. The output shows:
- Each spec's resolved name
- Whether it would be created or updated
- The merge result (profile defaults + spec identity + overlay policy)

**10. Apply the group:**

```bash
owlctl job apply --group db-tier
```

All specs in the group are applied with the 3-way merge. The summary table shows the result for each spec.

**11. Verify with group diff:**

```bash
owlctl job diff --group db-tier
```

Exit code 0 — all jobs in the group match their declared state (profile + spec + overlay merged).

**Takeaway:** Groups let you manage policies across many jobs. Add a new job to the group by adding one line to `owlctl.yaml` — it automatically inherits the profile defaults and overlay policy.

---

### Act 3: Ongoing Compliance — CI/CD Integration

This act demonstrates the automated drift detection and remediation pipeline.

**12. Introduce drift again:**

Open the VBR console and disable one of the database backup jobs. This simulates someone making an emergency change that violates policy.

**13. Trigger the drift scan pipeline:**

Run the `detect-remediate` pipeline manually (or wait for the scheduled run).

The pipeline runs `owlctl job diff --group db-tier --security-only` and detects:
- CRITICAL: `isDisabled` changed from `false` to `true`
- Pipeline fails with exit code 4
- Azure DevOps tags the build with `critical-drift`

**14. Show the pipeline output:**

The pipeline log shows the drift classification:
```
CRITICAL: 1 security-relevant change detected
  isDisabled: false → true  [CRITICAL]
```

**15. Remediate via the deployment pipeline:**

Merge to master triggers the deployment pipeline, which runs `owlctl job apply --group db-tier` and restores the job to its declared state.

Alternatively, show how a team member could remediate immediately:

```bash
owlctl job apply --group db-tier
```

**Takeaway:** The CI/CD pipeline continuously monitors for drift. Critical changes (disabled jobs, reduced retention, removed encryption) are flagged immediately. The deployment pipeline can auto-remediate on the next merge.

---

### Act 4: Multi-Target Deployment (Bonus)

This act extends Act 2 by applying the same group to the DR VBR target.

**16. Apply the same group to the DR target:**

```bash
export OWLCTL_URL="https://vbr-dr.example.com:9419"
owlctl login
owlctl job apply --group db-tier
```

The same profile + spec + overlay merge is applied to the DR VBR. Both sites now have identical policy.

**17. Verify across both targets:**

```bash
# Check primary
export OWLCTL_URL="https://vbr-primary.example.com:9419"
owlctl login
owlctl job diff --group db-tier
# Exit code 0

# Check DR
export OWLCTL_URL="https://vbr-dr.example.com:9419"
owlctl login
owlctl job diff --group db-tier
# Exit code 0
```

**18. Show the pipeline handles this automatically:**

The `deployment.yml` pipeline has two stages: `DeployPrimary` and `DeployDR`. After primary is approved and applied, the DR stage runs with its own credentials and approval gate.

**Takeaway:** The same group definition drives both targets. Change the profile or overlay once in Git, and both VBR servers are updated through the pipeline.

## Summary: Without Groups vs. With Groups

| Without Groups | With Groups |
|---------------|-------------|
| Apply each spec file individually | `owlctl job apply --group db-tier` applies all specs at once |
| Copy-paste retention/schedule into every spec | Profile sets defaults; overlay enforces policy |
| Manual per-file overlay matching | Group definition binds specs to their profile and overlay |
| Drift check each job by name | `owlctl job diff --group db-tier` checks all group members |
| Adding a new job = new spec + new overlay + update scripts | Adding a new job = new spec + one line in `owlctl.yaml` |
| Policy change = edit N overlay files | Policy change = edit one overlay file |

## Final Repo Structure

After completing all steps, your repo should look like:

```
vbr-infrastructure/
├── .owlctl/
│   ├── settings.json
│   ├── profiles.json
│   └── state.json
├── specs/
│   ├── jobs/
│   │   ├── sql-vm-01.yaml         # kind: VBRJob — identity + VM targets
│   │   ├── sql-vm-02.yaml
│   │   ├── web-frontend.yaml
│   │   ├── web-api.yaml
│   │   └── ...
│   ├── repos/
│   │   └── production-repo.yaml
│   ├── sobrs/
│   │   └── scale-out-repo.yaml
│   └── kms/
│       └── azure-keyvault.yaml
├── profiles/
│   ├── gold.yaml                   # kind: Profile — gold-tier defaults
│   └── standard.yaml               # kind: Profile — standard defaults
├── overlays/
│   └── compliance.yaml             # kind: Overlay — policy enforcement
├── pipelines/
│   ├── bootstrap.yml
│   ├── pr-validation.yml
│   ├── deployment.yml
│   ├── detect-remediate.yml
│   └── nightly-compliance.yml
├── scripts/
│   ├── apply-group.sh
│   └── snapshot-all.sh
├── .gitignore
├── owlctl.yaml                     # Groups + targets configuration
└── README.md
```

### Azure DevOps resources created

| Resource | Name |
|----------|------|
| Environments | `vbr-primary`, `vbr-dr` |
| Variable Groups | `veeam-credentials-primary`, `veeam-credentials-dr` |
| Pipelines | `deployment`, `detect-remediate`, `pr-validation`, `bootstrap`, `nightly-compliance` |
| Branch Policy | PR validation required on `master` |

## Teardown

To clean up after the demo:

```bash
# Stop the Docker agent
cd /path/to/owlctl/docker/azure-agent
docker compose down

# Delete the Azure DevOps project (or just the pipelines/environments)
# Do this manually in the Azure DevOps UI
```
