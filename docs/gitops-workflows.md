# GitOps Workflows with owlctl

A comprehensive guide to managing Veeam Backup & Replication infrastructure using GitOps principles with owlctl.

## Table of Contents

- [What is GitOps with owlctl?](#what-is-gitops-with-owlctl)
- [Benefits](#benefits)
- [Repository Structure](#repository-structure)
- [Configuration Management](#configuration-management)
- [CI/CD Platform Integration](#cicd-platform-integration)
  - [GitHub Actions](#github-actions)
  - [Azure DevOps](#azure-devops)
  - [GitLab CI](#gitlab-ci)
- [Security Best Practices](#security-best-practices)
- [Common Patterns](#common-patterns)
- [State Management in GitOps](#state-management-in-gitops)
- [Troubleshooting](#troubleshooting)

## What is GitOps with owlctl?

GitOps is a way to manage Veeam Backup & Replication infrastructure using Git as the single source of truth. owlctl enables GitOps workflows through:

- ✅ **Declarative configuration** - YAML specs define desired state
- ✅ **State management** - Drift detection catches unauthorized changes
- ✅ **Non-interactive commands** - CI/CD friendly automation
- ✅ **Secure token handling** - System keychain with auto-auth fallback
- ✅ **Multi-environment support** - Configuration overlays for dev/staging/prod

**The GitOps Loop:**
```
┌─────────────────────────────────────────────────┐
│  1. Developer commits YAML to Git               │
│  2. CI/CD pipeline triggered                    │
│  3. owlctl applies configuration to VBR           │
│  4. owlctl snapshots state for drift detection    │
│  5. Scheduled drift checks catch manual changes │
│  6. Alerts sent if drift detected               │
└─────────────────────────────────────────────────┘
```

## Benefits

### 1. Version Control
- Complete history of infrastructure changes
- Who changed what, when, and why
- Easy rollback to previous configurations
- Branch-based development and testing

### 2. Audit Trail
- Git commit history provides immutable audit log
- Signed commits for non-repudiation
- PR approvals track authorization
- CI/CD logs capture execution details

### 3. Consistency Across Environments
- Single source of truth in Git
- Environment-specific overlays prevent drift
- Same configuration process everywhere
- Reduced human error

### 4. Automation
- CI/CD handles all deployments
- No manual VBR console changes
- Automated drift detection and alerting
- Self-healing through auto-remediation

### 5. Disaster Recovery
- Rebuild entire VBR infrastructure from Git
- Infrastructure-as-code enables rapid recovery
- Test DR procedures in dev environments
- Version-controlled backup strategies

### 6. Collaboration
- Pull request reviews before production changes
- Team visibility into all infrastructure changes
- Documentation in commit messages
- Knowledge sharing through code

## Repository Structure

### Recommended Directory Layout

```
vbr-infrastructure/
├── .github/
│   └── workflows/
│       ├── pr-validation.yml      # Validate PRs before merge
│       ├── deploy-prod.yml        # Deploy to production
│       ├── drift-detection.yml    # Scheduled drift checks
│       └── auto-remediate.yml     # Auto-fix drift
├── .owlctl/
│   ├── settings.json              # owlctl settings (commit this)
│   └── profiles.json              # API profiles (commit this)
├── profiles/
│   ├── gold.yaml                  # kind: Profile — high retention, encryption
│   └── standard.yaml              # kind: Profile — standard defaults
├── specs/
│   ├── jobs/
│   │   ├── sql-vm-01.yaml
│   │   ├── web-frontend.yaml
│   │   └── fileserver-backup.yaml
│   ├── repos/
│   │   ├── production-repo.yaml
│   │   └── archive-repo.yaml
│   ├── sobrs/
│   │   └── scale-out-repo.yaml
│   ├── kms/
│   │   └── azure-keyvault.yaml
│   └── encryption/
│       └── backup-encryption.yaml
├── overlays/
│   └── compliance.yaml            # kind: Overlay — policy overrides
├── scripts/
│   ├── apply-all.sh               # Helper script to apply all configs
│   └── snapshot-all.sh            # Helper script to snapshot all resources
├── .gitignore
├── owlctl.yaml                      # Groups and targets configuration
├── README.md
└── CHANGELOG.md
```

### What to Commit

✅ **Always Commit:**
- Configuration specs (all YAML files in `specs/` and `overlays/`)
- `profiles.json` - API configuration (no credentials)
- `settings.json` - owlctl behavior settings
- Pipeline definitions (`.github/workflows/`, `.azure-pipelines/`, `.gitlab-ci.yml`)
- Helper scripts
- Documentation (README, CHANGELOG)

❌ **Never Commit:**
- `state.json` - See [State Management](#state-management-in-gitops) section
- Credentials or tokens
- `.env` files with secrets
- `headers.json` (no longer exists)

⚠️ **Optional (Team Decision):**
- `state.json` - Provides audit trail but creates merge conflicts (see guidance below)

### .gitignore Example

```gitignore
# owlctl state (decision: don't commit for this team)
state.json
state.json.backup

# Credentials
.env
credentials.json
*.key
*.pem

# Local development
.DS_Store
Thumbs.db
*.swp
*.swo
*~

# IDE
.vscode/
.idea/
*.iml

# Logs
*.log
```

## Configuration Management

### profiles.json in Version Control

**Current Format** (safe to commit):

```json
{
  "version": "1.0",
  "currentProfile": "vbr",
  "profiles": {
    "vbr": {
      "product": "VeeamBackupReplication",
      "apiVersion": "1.3-rev1",
      "port": 9419,
      "endpoints": {
        "auth": "/api/oauth2/token",
        "apiPrefix": "/api/v1"
      },
      "authType": "oauth",
      "headers": {
        "accept": "application/json",
        "contentType": "application/x-www-form-urlencoded",
        "xAPIVersion": "1.3-rev1"
      }
    }
  }
}
```

**Safe to commit** - Contains only technical API configuration, no credentials.

### settings.json Per Environment

```json
{
  "apiNotSecure": false
}
```

**Commit this** - Controls owlctl behavior, contains no secrets.

**Environment-specific settings:**
- Production: `"apiNotSecure": false` (require valid certificates)
- Development: `"apiNotSecure": true` (allow self-signed certs)

### Credentials Management

**Golden Rule: Never commit credentials to Git.**

Use CI/CD platform secret stores:

**GitHub Actions:**
```yaml
env:
  OWLCTL_USERNAME: ${{ secrets.VBR_USERNAME }}
  OWLCTL_PASSWORD: ${{ secrets.VBR_PASSWORD }}
  OWLCTL_URL: ${{ secrets.VBR_URL }}
```

**Azure DevOps:**
```yaml
env:
  OWLCTL_USERNAME: $(vbrUsername)
  OWLCTL_PASSWORD: $(vbrPassword)
  OWLCTL_URL: $(vbrUrl)
```

**GitLab CI:**
```yaml
variables:
  OWLCTL_USERNAME: $VBR_USERNAME
  OWLCTL_PASSWORD: $VBR_PASSWORD
  OWLCTL_URL: $VBR_URL
```

### owlctl.yaml for Groups and Instances

Optional file for group-based deployments and multi-server targeting. Use `owlctl instance add` to register instances without hand-editing the file:

```bash
owlctl instance add vbr-dev     --url vbr-dev.example.com     --product vbr --credential-ref DEV     --description "Development VBR"
owlctl instance add vbr-staging --url vbr-staging.example.com --product vbr --credential-ref STAGING --description "Staging VBR"
owlctl instance add vbr-prod    --url vbr-prod.example.com    --product vbr --credential-ref PROD    --description "Production VBR"
owlctl instance add vbr-dr      --url vbr-dr.example.com      --product vbr --credential-ref DR      --description "DR site"
```

Then add your `groups:` section to `owlctl.yaml`. The resulting file looks like:

```yaml
# owlctl.yaml - Groups and instances
apiVersion: owlctl.veeam.com/v1
kind: Config

instances:
  vbr-dev:
    product: vbr
    url: vbr-dev.example.com
    credentialRef: DEV
    description: Development VBR
  vbr-staging:
    product: vbr
    url: vbr-staging.example.com
    credentialRef: STAGING
    description: Staging VBR
  vbr-prod:
    product: vbr
    url: vbr-prod.example.com
    credentialRef: PROD
    description: Production VBR
  vbr-dr:
    product: vbr
    url: vbr-dr.example.com
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

  web-tier:
    description: Web server backups
    instance: vbr-prod
    profile: profiles/standard.yaml
    specs:
      - specs/jobs/web-frontend.yaml
```

Then in pipelines:
```bash
# Apply a group (instance activated from group definition)
owlctl job apply --group sql-tier

# Apply same group to DR instance
owlctl job apply --group sql-tier --instance vbr-dr
```

## CI/CD Platform Integration

### GitHub Actions

#### Complete Deployment Pipeline

**File:** `.github/workflows/deploy-prod.yml`

```yaml
name: Deploy to Production

on:
  push:
    branches: [main]
    paths:
      - 'specs/**'
      - 'overlays/prod/**'

jobs:
  deploy:
    runs-on: ubuntu-latest
    environment: production  # Requires approval in GitHub settings

    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Download owlctl
        run: |
          curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
          tar xzf owlctl.tar.gz
          chmod +x owlctl

      - name: Verify owlctl checksum
        run: |
          curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz.sha256 -o owlctl.tar.gz.sha256
          sha256sum -c owlctl.tar.gz.sha256

      - name: Configure owlctl
        env:
          OWLCTL_USERNAME: ${{ secrets.VBR_USERNAME }}
          OWLCTL_PASSWORD: ${{ secrets.VBR_PASSWORD }}
          OWLCTL_URL: ${{ secrets.VBR_URL }}
          OWLCTL_SETTINGS_PATH: ./.owlctl/
        run: |
          ./owlctl profile --set vbr
          ./owlctl login

      - name: Apply job configurations
        run: |
          for job in specs/jobs/*.yaml; do
            echo "Applying: $job"
            ./owlctl job apply "$job" --overlay "overlays/prod/$(basename $job)" --dry-run
            ./owlctl job apply "$job" --overlay "overlays/prod/$(basename $job)"
          done

      - name: Apply repository configurations
        run: |
          for repo in specs/repos/*.yaml; do
            echo "Applying: $repo"
            ./owlctl repo apply "$repo" --dry-run
            ./owlctl repo apply "$repo"
          done

      - name: Apply SOBR configurations
        run: |
          for sobr in specs/sobrs/*.yaml; do
            echo "Applying: $sobr"
            ./owlctl repo sobr-apply "$sobr" --dry-run
            ./owlctl repo sobr-apply "$sobr"
          done

      - name: Verify no drift
        run: |
          ./owlctl job diff --all --security-only
          EXIT_CODE=$?
          if [ $EXIT_CODE -eq 4 ]; then
            echo "::error::CRITICAL drift detected after deployment!"
            exit 1
          fi
```

#### Pull Request Validation

**File:** `.github/workflows/pr-validation.yml`

```yaml
name: PR Validation

on:
  pull_request:
    branches: [main]
    paths:
      - 'specs/**'
      - 'overlays/**'

jobs:
  validate:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout PR
        uses: actions/checkout@v4

      - name: Download owlctl
        run: |
          curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
          tar xzf owlctl.tar.gz
          chmod +x owlctl

      - name: Configure owlctl
        env:
          OWLCTL_USERNAME: ${{ secrets.VBR_USERNAME_DEV }}
          OWLCTL_PASSWORD: ${{ secrets.VBR_PASSWORD_DEV }}
          OWLCTL_URL: ${{ secrets.VBR_URL_DEV }}
          OWLCTL_SETTINGS_PATH: ./.owlctl/
        run: |
          ./owlctl profile --set vbr
          ./owlctl login

      - name: Find changed YAML files
        id: changed-files
        uses: tj-actions/changed-files@v45  # Pin to specific version
        with:
          files: |
            specs/**/*.yaml
            overlays/**/*.yaml

      - name: Validate changed specs (dry-run)
        if: steps.changed-files.outputs.any_changed == 'true'
        run: |
          echo "Changed files: ${{ steps.changed-files.outputs.all_changed_files }}"

          FAILED=0
          for file in ${{ steps.changed-files.outputs.all_changed_files }}; do
            # Skip overlay files
            if [[ "$file" == overlays/* ]]; then
              continue
            fi

            echo "Validating: $file"

            case "$file" in
              specs/jobs/*)
                ./owlctl job apply "$file" --dry-run
                ;;
              specs/repos/*)
                ./owlctl repo apply "$file" --dry-run
                ;;
              specs/sobrs/*)
                ./owlctl repo sobr-apply "$file" --dry-run
                ;;
              specs/kms/*)
                ./owlctl encryption kms-apply "$file" --dry-run
                ;;
              *)
                echo "Unknown file type: $file"
                ;;
            esac

            if [ $? -ne 0 ]; then
              FAILED=$((FAILED + 1))
              echo "::error::Validation failed for $file"
            fi
          done

          if [ $FAILED -gt 0 ]; then
            echo "::error::$FAILED file(s) failed validation"
            exit 1
          fi

      - name: Comment PR with validation results
        if: always()
        uses: actions/github-script@v7
        with:
          script: |
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: '✅ All configurations validated successfully via dry-run against dev VBR environment.'
            })
```

#### Automated Drift Detection

**File:** `.github/workflows/drift-detection.yml`

```yaml
name: Drift Detection

on:
  schedule:
    - cron: '0 */6 * * *'  # Every 6 hours
  workflow_dispatch:        # Manual trigger

jobs:
  detect-drift:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Download owlctl
        run: |
          curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
          tar xzf owlctl.tar.gz
          chmod +x owlctl

      - name: Configure owlctl
        env:
          OWLCTL_USERNAME: ${{ secrets.VBR_USERNAME }}
          OWLCTL_PASSWORD: ${{ secrets.VBR_PASSWORD }}
          OWLCTL_URL: ${{ secrets.VBR_URL }}
          OWLCTL_SETTINGS_PATH: ./.owlctl/
        run: |
          ./owlctl profile --set vbr
          ./owlctl login

      - name: Check for critical drift
        id: drift-check
        run: |
          set +e  # Don't exit on error

          echo "## Drift Detection Report" > drift-report.md
          echo "" >> drift-report.md
          echo "**Timestamp:** $(date -u '+%Y-%m-%d %H:%M UTC')" >> drift-report.md
          echo "" >> drift-report.md

          CRITICAL=0

          echo "### Backup Jobs" >> drift-report.md
          ./owlctl job diff --all --security-only > job-drift.txt 2>&1
          JOB_EXIT=$?
          cat job-drift.txt >> drift-report.md
          echo "" >> drift-report.md
          if [ $JOB_EXIT -eq 4 ]; then CRITICAL=1; fi

          echo "### Repositories" >> drift-report.md
          ./owlctl repo diff --all --security-only > repo-drift.txt 2>&1
          REPO_EXIT=$?
          cat repo-drift.txt >> drift-report.md
          echo "" >> drift-report.md
          if [ $REPO_EXIT -eq 4 ]; then CRITICAL=1; fi

          echo "### Scale-Out Repositories" >> drift-report.md
          ./owlctl repo sobr-diff --all --security-only > sobr-drift.txt 2>&1
          SOBR_EXIT=$?
          cat sobr-drift.txt >> drift-report.md
          echo "" >> drift-report.md
          if [ $SOBR_EXIT -eq 4 ]; then CRITICAL=1; fi

          echo "### Configuration Backup" >> drift-report.md
          ./owlctl config-backup diff --security-only > cb-drift.txt 2>&1
          CB_EXIT=$?
          cat cb-drift.txt >> drift-report.md
          echo "" >> drift-report.md
          if [ $CB_EXIT -eq 4 ]; then CRITICAL=1; fi

          echo "critical=$CRITICAL" >> $GITHUB_OUTPUT

      - name: Create Issue on Critical Drift
        if: steps.drift-check.outputs.critical == '1'
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const report = fs.readFileSync('drift-report.md', 'utf8');

            github.rest.issues.create({
              owner: context.repo.owner,
              repo: context.repo.repo,
              title: '🚨 CRITICAL Drift Detected',
              body: report,
              labels: ['drift', 'critical', 'security']
            });

      - name: Send Slack notification
        if: steps.drift-check.outputs.critical == '1'
        env:
          SLACK_WEBHOOK: ${{ secrets.SLACK_WEBHOOK_URL }}
        run: |
          curl -X POST -H 'Content-type: application/json' \
            --data '{"text":"🚨 CRITICAL VBR drift detected! Check GitHub Issues for details."}' \
            "$SLACK_WEBHOOK"
```

### Azure DevOps

owlctl provides comprehensive Azure DevOps integration. See the dedicated [Azure DevOps Integration Guide](azure-devops-integration.md) for:

- Pre-built pipeline templates (PR validation, deployment, drift detection, compliance)
- Variable group configuration
- Secure file handling
- Multi-stage deployments with approval gates

**Quick Example:**

```yaml
# azure-pipelines.yml
trigger:
  branches:
    include: [main]
  paths:
    include: ['specs/**', 'overlays/prod/**']

pool:
  vmImage: 'ubuntu-latest'

variables:
  - group: 'veeam-credentials'

steps:
  - script: |
      curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
      tar xzf owlctl.tar.gz
      chmod +x owlctl
      ./owlctl profile --set vbr
      ./owlctl login
    displayName: 'Setup owlctl'
    env:
      OWLCTL_USERNAME: $(vbrUsername)
      OWLCTL_PASSWORD: $(vbrPassword)
      OWLCTL_URL: $(vbrUrl)

  - script: |
      for job in specs/jobs/*.yaml; do
        ./owlctl job apply "$job" --dry-run
        ./owlctl job apply "$job"
      done
    displayName: 'Deploy configurations'
```

For complete examples, see: [examples/pipelines/](../examples/pipelines/)

### GitLab CI

#### Complete Deployment Pipeline

**File:** `.gitlab-ci.yml`

```yaml
stages:
  - validate
  - deploy
  - verify

variables:
  OWLCTL_VERSION: "latest"
  OWLCTL_SETTINGS_PATH: "./.owlctl/"

.owlctl-setup: &owlctl-setup
  before_script:
    - apt-get update && apt-get install -y curl
    # 'latest' and pinned tags use different GitHub release URL forms
    - |
      if [ "$OWLCTL_VERSION" = "latest" ]; then
        DL="https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz"
      else
        DL="https://github.com/shapedthought/owlctl/releases/download/${OWLCTL_VERSION}/owlctl-linux-amd64.tar.gz"
      fi
    - curl -sL "$DL" -o owlctl.tar.gz
    - tar xzf owlctl.tar.gz
    - chmod +x owlctl
    - ./owlctl profile --set vbr
    - ./owlctl login

validate-pr:
  stage: validate
  <<: *owlctl-setup
  variables:
    OWLCTL_USERNAME: $VBR_USERNAME_DEV
    OWLCTL_PASSWORD: $VBR_PASSWORD_DEV
    OWLCTL_URL: $VBR_URL_DEV
  script:
    - |
      FAILED=0
      for job in specs/jobs/*.yaml; do
        echo "Validating: $job"
        ./owlctl job apply "$job" --dry-run
        if [ $? -ne 0 ]; then
          FAILED=$((FAILED + 1))
        fi
      done

      if [ $FAILED -gt 0 ]; then
        echo "ERROR: $FAILED file(s) failed validation"
        exit 1
      fi
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'

deploy-production:
  stage: deploy
  <<: *owlctl-setup
  variables:
    OWLCTL_USERNAME: $VBR_USERNAME
    OWLCTL_PASSWORD: $VBR_PASSWORD
    OWLCTL_URL: $VBR_URL
  script:
    # Apply jobs
    - |
      for job in specs/jobs/*.yaml; do
        echo "Applying: $job"
        overlay="overlays/prod/$(basename $job)"
        if [ -f "$overlay" ]; then
          ./owlctl job apply "$job" --overlay "$overlay"
        else
          ./owlctl job apply "$job"
        fi
      done

    # Apply repositories
    - |
      for repo in specs/repos/*.yaml; do
        echo "Applying: $repo"
        ./owlctl repo apply "$repo"
      done

    # Apply SOBRs
    - |
      for sobr in specs/sobrs/*.yaml; do
        echo "Applying: $sobr"
        ./owlctl repo sobr-apply "$sobr"
      done
  environment:
    name: production
  rules:
    - if: '$CI_COMMIT_BRANCH == "main"'
      when: manual  # Require manual approval

verify-deployment:
  stage: verify
  <<: *owlctl-setup
  variables:
    OWLCTL_USERNAME: $VBR_USERNAME
    OWLCTL_PASSWORD: $VBR_PASSWORD
    OWLCTL_URL: $VBR_URL
  script:
    - ./owlctl job diff --all --security-only
    - |
      EXIT_CODE=$?
      if [ $EXIT_CODE -eq 4 ]; then
        echo "ERROR: CRITICAL drift detected after deployment"
        exit 1
      fi
  rules:
    - if: '$CI_COMMIT_BRANCH == "main"'
  needs: [deploy-production]

drift-detection:
  stage: verify
  <<: *owlctl-setup
  variables:
    OWLCTL_USERNAME: $VBR_USERNAME
    OWLCTL_PASSWORD: $VBR_PASSWORD
    OWLCTL_URL: $VBR_URL
  script:
    - ./owlctl job diff --all --security-only > drift-report.txt 2>&1
    - ./owlctl repo diff --all --security-only >> drift-report.txt 2>&1
    - ./owlctl repo sobr-diff --all --security-only >> drift-report.txt 2>&1
    - |
      if grep -q "CRITICAL" drift-report.txt; then
        echo "CRITICAL drift detected!"
        cat drift-report.txt
        exit 1
      fi
  artifacts:
    reports:
      dotenv: drift-report.txt
    when: always
    expire_in: 30 days
  rules:
    - if: '$CI_PIPELINE_SOURCE == "schedule"'
```

#### Scheduled Drift Detection

Add this to your `.gitlab-ci.yml` or create a schedule in GitLab UI:

```yaml
# Schedule in GitLab UI:
# CI/CD > Schedules > New schedule
# Interval: 0 */6 * * * (every 6 hours)
# Target branch: main
# Variables: (none needed, uses project variables)
```

## Security Best Practices

### 1. Secrets Management

**DO:**
- ✅ Store credentials in CI/CD secret stores (GitHub Secrets, Azure Key Vault, GitLab Variables)
- ✅ Use separate service accounts per environment (prod/staging/dev)
- ✅ Rotate credentials regularly (quarterly minimum)
- ✅ Use least-privilege access (VBR roles for specific operations)
- ✅ Enable MFA on service accounts where possible

**DON'T:**
- ❌ Commit credentials to Git (even in private repos)
- ❌ Print credentials in logs (`echo $OWLCTL_PASSWORD`)
- ❌ Share credentials between environments
- ❌ Use personal accounts for automation
- ❌ Store tokens in long-lived files

### 2. Token Lifecycle Management

**Short-lived tokens (recommended):**
```yaml
steps:
  - name: Login and operate
    run: |
      ./owlctl login
      ./owlctl job diff --all
      # Token automatically discarded after job
```

**Explicit token control:**
```yaml
steps:
  - name: Get token
    run: |
      export OWLCTL_TOKEN=$(./owlctl login --output-token)
      ./owlctl job diff --all
      ./owlctl repo diff --all
      unset OWLCTL_TOKEN
```

**Never:**
```yaml
# BAD - Token persists across jobs
- name: Login
  run: ./owlctl login

- name: Later job
  run: ./owlctl get jobs  # Uses stale token
```

### 3. Git Security

**Commit Signing:**
```bash
# Configure GPG signing
git config --global user.signingkey YOUR_KEY_ID
git config --global commit.gpgsign true

# All commits now signed
git commit -m "Update backup retention"
```

**Branch Protection:**
- Require PR reviews before merge (minimum 2 reviewers for production)
- Require status checks to pass (PR validation, tests)
- Require signed commits
- Restrict who can push to main/master
- Require linear history (no merge commits)

**CODEOWNERS file:**
```
# .github/CODEOWNERS
# Require security team approval for critical changes

specs/jobs/*           @security-team @backup-team
specs/kms/*            @security-team
overlays/prod/*        @security-team @senior-engineers
.github/workflows/*    @devops-team @security-team
```

### 4. Audit Logging

**Git audit trail:**
```bash
# View who changed what
git log --pretty=format:"%h %an %ae %ad %s" -- specs/jobs/critical-backup.yaml

# View complete history of a file
git log -p -- specs/jobs/critical-backup.yaml

# Find when a specific change was made
git log -S "retentionPolicy" -- specs/jobs/*.yaml
```

**CI/CD logging:**
- Enable detailed logging for owlctl operations
- Retain pipeline logs for compliance period (90 days minimum)
- Export logs to SIEM for security monitoring
- Alert on failed deployments or critical drift

**VBR audit logs:**
- Enable VBR audit logging
- Configure log forwarding to central logging system
- Correlate owlctl apply operations with VBR audit events
- Monitor for configuration changes not from owlctl

### 5. Access Control

**Principle of least privilege:**
```yaml
# Bad - Over-privileged
VBR_ADMIN_CREDENTIALS  # Can do anything

# Good - Scoped permissions
VBR_BACKUP_OPERATOR    # Can modify jobs only
VBR_REPO_ADMIN         # Can modify repos only
```

**Separate environments:**
```
Production:  prod-vbr-svc@company.com  (restricted access)
Staging:     stg-vbr-svc@company.com   (broader access)
Development: dev-vbr-svc@company.com   (open access)
```

### 6. Sensitive Data Handling

**Never commit:**
- Passwords or API tokens
- Private keys or certificates
- Connection strings with embedded credentials
- `.env` files

**Use placeholder values in committed configs:**
```yaml
# specs/kms/azure-keyvault.yaml
apiVersion: owlctl.veeam.com/v1
kind: VBRKmsServer
metadata:
  name: Azure Key Vault Production
spec:
  endpoint: "${KMS_ENDPOINT}"  # Replaced at deploy time
  credentials:
    tenantId: "${AZURE_TENANT_ID}"
    clientId: "${AZURE_CLIENT_ID}"
    # clientSecret: provided via OWLCTL_KMS_SECRET env var
```

### 7. Drift Detection as Security Control

**Treat drift as a security event:**
```yaml
# In drift detection job
- name: Alert on ANY drift
  if: steps.drift-check.outputs.critical == '1'
  run: |
    # Send to security SIEM
    curl -X POST $SIEM_WEBHOOK \
      -d '{"severity":"high","message":"VBR drift detected"}'

    # Create incident ticket
    curl -X POST $TICKETING_API \
      -d '{"title":"VBR Configuration Drift","priority":"high"}'
```

**Automated remediation with approval:**
```yaml
- name: Propose auto-remediation
  if: steps.drift-check.outputs.critical == '1'
  run: |
    # Create PR to update state
    gh pr create --title "Auto-remediate VBR drift" \
      --body "Detected drift - applying current Git state"
```

## Common Patterns

### Pattern 1: Pull Request Validation

**Goal:** Validate all configuration changes before merging to main.

**Implementation:**
1. Run `owlctl apply --dry-run` on changed files
2. Test against dev VBR environment
3. Comment results on PR
4. Block merge if validation fails

**GitHub Actions example:**
```yaml
name: PR Validation

on: pull_request

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup owlctl
        run: |
          curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
          tar xzf owlctl.tar.gz
          chmod +x owlctl

      - name: Configure owlctl
        env:
          OWLCTL_USERNAME: ${{ secrets.VBR_USERNAME_DEV }}
          OWLCTL_PASSWORD: ${{ secrets.VBR_PASSWORD_DEV }}
          OWLCTL_URL: ${{ secrets.VBR_URL_DEV }}
        run: |
          ./owlctl profile --set vbr
          ./owlctl login

      - name: Validate changed specs
        run: |
          # Get list of changed YAML files
          git diff --name-only origin/main...HEAD | \
            grep -E '^specs/.*\.yaml$' | \
            while read file; do
              echo "Validating: $file"
              ./owlctl job apply "$file" --dry-run || exit 1
            done
```

**Exit codes:**
- `0` - Validation passed, PR can merge
- `1` - Validation failed, block PR
- `6` - Resource doesn't exist (new resource, needs manual review)

### Pattern 2: Automated Drift Detection with Alerting

**Goal:** Detect unauthorized changes and alert security team.

**Implementation:**
1. Run drift checks on schedule (every 6 hours)
2. Check only security-relevant drift (`--security-only`)
3. Create GitHub issue on critical drift
4. Send Slack/Teams notification
5. Optionally trigger auto-remediation

**Complete example in GitHub Actions section above.**

**Key points:**
- Use exit codes: `4` = critical, `3` = warning, `0` = no drift
- Filter with `--security-only` to reduce noise
- Separate detection from remediation (manual approval)

### Pattern 3: Multi-Target Deployment with Groups

**Goal:** Deploy groups to dev → staging → prod using named targets with progressive rollout.

**owlctl.yaml:**
```yaml
apiVersion: owlctl.veeam.com/v1
kind: Config

groups:
  sql-tier:
    description: SQL Server backup group
    profile: profiles/gold.yaml
    overlay: overlays/compliance.yaml
    specs:
      - specs/jobs/sql-vm-01.yaml
      - specs/jobs/sql-vm-02.yaml

targets:
  dev:
    url: https://vbr-dev.example.com
    description: Development VBR
  staging:
    url: https://vbr-staging.example.com
    description: Staging VBR
  prod:
    url: https://vbr-prod.example.com
    description: Production VBR
```

**GitHub Actions example:**
```yaml
name: Multi-Target Deploy

on:
  push:
    branches: [main]

jobs:
  deploy-dev:
    runs-on: ubuntu-latest
    environment: development
    steps:
      - uses: actions/checkout@v4
      - name: Setup owlctl
        run: |
          curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
          tar xzf owlctl.tar.gz
          chmod +x owlctl

      - name: Deploy to dev
        env:
          OWLCTL_DEV_USERNAME: ${{ secrets.VBR_USERNAME_DEV }}
          OWLCTL_DEV_PASSWORD: ${{ secrets.VBR_PASSWORD_DEV }}
        run: |
          ./owlctl --instance vbr-dev login
          ./owlctl job apply --group sql-tier --instance vbr-dev

  deploy-staging:
    runs-on: ubuntu-latest
    environment: staging
    needs: deploy-dev
    steps:
      - uses: actions/checkout@v4
      - name: Setup owlctl
        run: |
          curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
          tar xzf owlctl.tar.gz
          chmod +x owlctl

      - name: Deploy to staging
        env:
          OWLCTL_STAGING_USERNAME: ${{ secrets.VBR_USERNAME_STAGING }}
          OWLCTL_STAGING_PASSWORD: ${{ secrets.VBR_PASSWORD_STAGING }}
        run: |
          ./owlctl --instance vbr-staging login
          ./owlctl job apply --group sql-tier --instance vbr-staging

  deploy-prod:
    runs-on: ubuntu-latest
    environment: production  # Requires manual approval in GitHub
    needs: deploy-staging
    steps:
      - uses: actions/checkout@v4
      - name: Setup owlctl
        run: |
          curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
          tar xzf owlctl.tar.gz
          chmod +x owlctl

      - name: Deploy to production
        env:
          OWLCTL_PROD_USERNAME: ${{ secrets.VBR_USERNAME }}
          OWLCTL_PROD_PASSWORD: ${{ secrets.VBR_PASSWORD }}
        run: |
          ./owlctl --instance vbr-prod login
          ./owlctl job apply --group sql-tier --instance vbr-prod

      - name: Verify deployment
        run: |
          ./owlctl job diff --group sql-tier --instance vbr-prod
          if [ $? -ne 0 ]; then
            echo "::error::Drift detected after production deployment"
            exit 1
          fi
```

**Benefits:**
- Progressive rollout (dev → staging → prod)
- Approval gates for production
- Group-based deployment with profile+overlay merge
- Named instances with per-instance credentials and token caching
- Rollback by reverting Git commit

### Pattern 4: Auto-Remediation with Approval

**Goal:** Automatically fix drift but require human approval.

**Implementation:**
1. Drift detection job runs on schedule
2. If drift found, create PR to trigger remediation
3. PR requires approval before merge
4. Merge triggers deployment pipeline
5. Deployment applies Git state to VBR

**Workflow:**
```
Drift Detected → Create PR → Security Review → Approve → Merge → Auto-Deploy
```

**GitHub Actions drift detection (modified):**
```yaml
- name: Create remediation PR
  if: steps.drift-check.outputs.critical == '1'
  env:
    GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  run: |
    # Create branch
    git checkout -b "auto-remediate-$(date +%Y%m%d-%H%M%S)"

    # Add drift report
    echo "## Drift Detected" > REMEDIATION.md
    echo "" >> REMEDIATION.md
    cat drift-report.md >> REMEDIATION.md
    git add REMEDIATION.md
    git commit -m "Auto-remediation: Drift detected"
    git push origin HEAD

    # Create PR
    gh pr create \
      --title "🔧 Auto-Remediation: VBR Drift Detected" \
      --body "$(cat REMEDIATION.md)" \
      --label "drift,auto-remediation" \
      --reviewer security-team
```

### Pattern 5: Compliance Reporting

**Goal:** Generate compliance reports showing VBR drift status.

**Implementation:**
1. Run drift checks across all resource types
2. Generate Markdown report
3. Upload as pipeline artifact
4. Optionally publish to wiki or confluence

**GitHub Actions example:**
```yaml
name: Compliance Report

on:
  schedule:
    - cron: '0 2 * * MON'  # Weekly, Monday 2AM

jobs:
  generate-report:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup owlctl
        run: |
          curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
          tar xzf owlctl.tar.gz
          chmod +x owlctl

      - name: Configure owlctl
        env:
          OWLCTL_USERNAME: ${{ secrets.VBR_USERNAME }}
          OWLCTL_PASSWORD: ${{ secrets.VBR_PASSWORD }}
          OWLCTL_URL: ${{ secrets.VBR_URL }}
        run: |
          ./owlctl profile --set vbr
          ./owlctl login

      - name: Generate compliance report
        run: |
          cat > compliance-report.md <<'EOF'
          # VBR Compliance Report

          **Generated:** $(date -u '+%Y-%m-%d %H:%M UTC')
          **Environment:** Production VBR

          ## Executive Summary

          This report shows configuration drift detected in the Veeam Backup & Replication environment.

          ---

          EOF

          echo "## Backup Jobs" >> compliance-report.md
          ./owlctl job diff --all >> compliance-report.md 2>&1
          echo "" >> compliance-report.md

          echo "## Repositories" >> compliance-report.md
          ./owlctl repo diff --all >> compliance-report.md 2>&1
          echo "" >> compliance-report.md

          echo "## Scale-Out Repositories" >> compliance-report.md
          ./owlctl repo sobr-diff --all >> compliance-report.md 2>&1
          echo "" >> compliance-report.md

          echo "## Encryption" >> compliance-report.md
          ./owlctl encryption diff --all >> compliance-report.md 2>&1
          echo "" >> compliance-report.md

          echo "## Configuration Backup" >> compliance-report.md
          ./owlctl config-backup diff >> compliance-report.md 2>&1
          echo "" >> compliance-report.md

      - name: Upload report
        uses: actions/upload-artifact@v4
        with:
          name: compliance-report-${{ github.run_number }}
          path: compliance-report.md
          retention-days: 90
```

**Azure DevOps equivalent:**
See [examples/pipelines/nightly-compliance.yml](../examples/pipelines/nightly-compliance.yml)

## State Management in GitOps

### The State File Dilemma

owlctl uses `state.json` to track applied configurations for drift detection. Should you commit it to Git?

### Option 1: Commit state.json (Recommended for Production)

**Pros:**
- ✅ Complete audit trail of state changes
- ✅ Team visibility into current applied state
- ✅ Enables state-based drift detection
- ✅ Disaster recovery - rebuild from Git
- ✅ Historical view of infrastructure evolution

**Cons:**
- ❌ Merge conflicts with concurrent changes
- ❌ Large file size over time
- ❌ Requires conflict resolution strategy

**When to use:**
- Production environments with controlled deployment
- Compliance requirements for audit trail
- Sequential deployments (one at a time)

**Best practices:**
```gitignore
# Don't ignore state.json
# state.json  # REMOVED - we commit this

# But ignore backups
state.json.backup
state.json.*
```

**Handle merge conflicts:**
```bash
# If conflict occurs
git checkout --ours state.json   # Use our version
./owlctl repo snapshot --all       # Re-snapshot current VBR state
git add state.json
git commit -m "Resolve state conflict - re-snapshotted from VBR"
```

### Option 2: Don't Commit state.json (Dev/Test Environments)

**Pros:**
- ✅ No merge conflicts
- ✅ Simpler Git history
- ✅ Smaller repository size

**Cons:**
- ❌ No audit trail of state
- ❌ Each developer needs to snapshot independently
- ❌ Can't track state evolution over time

**When to use:**
- Development environments
- Testing environments
- Rapid iteration workflows

**Setup:**
```gitignore
# .gitignore
state.json
state.json.backup
```

**Workflow:**
```bash
# Each developer/pipeline run
./owlctl repo snapshot --all       # Create local state
./owlctl job diff --all            # Check drift against local state
```

### Option 3: Hybrid Approach (Recommended)

**Strategy:**
- Commit state.json for production
- Don't commit for dev/staging
- Use separate repositories or branches per environment

**Structure:**
```
vbr-prod/                # Production repo
├── specs/
├── .owlctl/state.json    # Committed
└── .gitignore          # Doesn't ignore state.json

vbr-dev/                 # Development repo
├── specs/
├── .owlctl/state.json    # Not committed (in .gitignore)
└── .gitignore          # Ignores state.json
```

### State in CI/CD Pipelines

**Pattern 1: Per-job state (no persistence)**
```yaml
steps:
  - name: Apply and check
    run: |
      ./owlctl job apply specs/jobs/backup.yaml
      # State created in memory, discarded after job
      ./owlctl job diff "Backup Job"
```

**Pattern 2: Artifact-based state**
```yaml
steps:
  - name: Restore previous state
    uses: actions/download-artifact@v4
    with:
      name: vbr-state
      path: .owlctl/

  - name: Apply configs
    run: ./owlctl job apply specs/jobs/*.yaml

  - name: Save new state
    uses: actions/upload-artifact@v4
    with:
      name: vbr-state
      path: .owlctl/state.json
```

**Pattern 3: Git-committed state**
```yaml
steps:
  - name: Apply configs
    run: ./owlctl job apply specs/jobs/*.yaml

  - name: Commit updated state
    run: |
      git add .owlctl/state.json
      git commit -m "Update state after deployment [skip ci]"
      git push
```

### Recommendation by Environment

| Environment | Commit state.json? | Rationale |
|-------------|-------------------|-----------|
| **Production** | ✅ Yes | Audit trail, compliance, DR |
| **Staging** | ⚠️ Maybe | Depends on compliance needs |
| **Development** | ❌ No | Avoid conflicts, rapid iteration |
| **CI/CD** | 📦 Artifact | Persist between jobs, don't commit |

## Troubleshooting

### Issue 1: Pipeline Hangs or Waits for Input

**Problem:** CI/CD pipeline stalls indefinitely.

**Cause:** owlctl command is waiting for interactive input.

**Solution:**

```yaml
# ❌ BAD - Waits for profile name
./owlctl profile --set

# ✅ GOOD - Provides argument
./owlctl profile --set vbr
```

**Other common causes:**
```yaml
# ❌ BAD - Interactive init
./owlctl init

# ✅ GOOD - Non-interactive init
./owlctl init --output-dir ./.owlctl/
```

### Issue 2: Authentication Fails in Pipeline

**Problem:** `failed to authenticate` or `no authentication method available`

**Diagnosis:**
```yaml
- name: Debug authentication
  run: |
    echo "Username set: ${OWLCTL_USERNAME:+yes}"
    echo "Password set: ${OWLCTL_PASSWORD:+yes}"
    echo "URL: $OWLCTL_URL"
    # Don't echo actual password!
```

**Common causes:**

**1. Missing environment variables:**
```yaml
# ❌ Forgot to set variables
steps:
  - run: ./owlctl login  # Fails

# ✅ Set all required variables
steps:
  - run: ./owlctl login
    env:
      OWLCTL_USERNAME: ${{ secrets.VBR_USERNAME }}
      OWLCTL_PASSWORD: ${{ secrets.VBR_PASSWORD }}
      OWLCTL_URL: ${{ secrets.VBR_URL }}
```

**2. Profile not set:**
```yaml
# ❌ Missing profile command
- run: ./owlctl login

# ✅ Set profile first
- run: |
    ./owlctl profile --set vbr
    ./owlctl login
```

**3. Wrong secret names:**
```yaml
# ❌ Typo in secret name
OWLCTL_USERNAME: ${{ secrets.VBR_USRNAME }}  # Wrong

# ✅ Correct secret name
OWLCTL_USERNAME: ${{ secrets.VBR_USERNAME }}  # Correct
```

### Issue 3: Profile Not Set Error

**Problem:** `profile not set` error in pipeline.

**Solution:**
```yaml
# Always set profile before login
- name: Configure owlctl
  run: |
    ./owlctl profile --set vbr
    ./owlctl login
```

**Profile commands require explicit argument:**
```bash
# ❌ OLD (v0.10.x) - Interactive
owlctl profile --set
# Waited for input: _

# ✅ Correct - Non-interactive
owlctl profile --set vbr
```

### Issue 4: Drift Shows Immediately After Apply

**Problem:** `owlctl job diff` shows drift right after successful apply.

**Possible causes:**

**1. Immutable fields:**
Some VBR fields can't be updated via API:
```bash
./owlctl job apply job.yaml
# Exit code 5 - Partial apply (some fields skipped)

./owlctl job diff "Job Name"
# Shows drift on immutable fields
```

**Solution:** This is expected. Document immutable fields.

**2. VBR adds default values:**
VBR may add fields not in your spec:
```yaml
# Your spec
spec:
  schedule:
    daily:
      localTime: "22:00"

# VBR adds
spec:
  schedule:
    daily:
      localTime: "22:00"
      daysOfWeek: ["Monday", "Tuesday", ...]  # Added by VBR
```

**Solution:** Export current config and update your spec.

**3. State out of sync:**
```bash
# Solution: Re-snapshot after apply
./owlctl job apply job.yaml
sleep 2
./owlctl repo snapshot --all
./owlctl job diff --all  # Should show no drift
```

### Issue 5: Resource Not Found (Exit Code 6)

**Problem:** Apply fails with "resource not found" error.

**Cause:** Repositories, SOBRs, and KMS servers are **update-only**. They must be created in VBR console first.

**Solution:**
```bash
# 1. Create resource in VBR console
#    (Use GUI to create repository)

# 2. Export to get YAML
./owlctl repo export "New Repository" -o repo.yaml

# 3. Now apply works
./owlctl repo apply repo.yaml
```

**Jobs** support creation via apply:
```bash
# Jobs can be created
./owlctl job apply new-job.yaml  # Creates if doesn't exist
```

### Issue 6: Merge Conflicts in state.json

**Problem:** Git merge conflict in state.json.

**Cause:** Multiple people/pipelines modified state concurrently.

**Solution 1: Use ours and re-snapshot**
```bash
git checkout --ours .owlctl/state.json
git add .owlctl/state.json
./owlctl repo snapshot --all  # Re-snapshot from VBR
git add .owlctl/state.json
git commit -m "Resolve state conflict - re-snapshot from VBR"
```

**Solution 2: Use theirs**
```bash
git checkout --theirs .owlctl/state.json
git add .owlctl/state.json
git commit -m "Resolve state conflict - accept remote state"
```

**Prevention:** Use artifact-based state or don't commit state.json in dev environments.

### Issue 7: Large state.json File

**Problem:** state.json growing very large, slowing Git operations.

**Cause:** State tracks full resource configurations.

**Solutions:**

**1. Archive old state periodically:**
```bash
# Monthly cleanup script
git log --pretty=format:"%H" -- .owlctl/state.json | tail -n 1 > state-archive-$(date +%Y%m).txt
git commit -m "Archive old state references"
```

**2. Use Git LFS:**
```bash
# .gitattributes
.owlctl/state.json filter=lfs diff=lfs merge=lfs -text

git lfs install
git lfs track ".owlctl/state.json"
git add .gitattributes
git commit -m "Track state.json with Git LFS"
```

**3. Don't commit state.json** (see State Management section)

### Issue 8: Pipeline Secrets Not Available

**Problem:** Secrets work in some jobs but not others.

**GitHub Actions:**
```yaml
# ❌ Secrets not inherited by reusable workflow
jobs:
  call-deploy:
    uses: ./.github/workflows/deploy.yml
    # Missing: secrets: inherit

# ✅ Pass secrets explicitly
jobs:
  call-deploy:
    uses: ./.github/workflows/deploy.yml
    secrets: inherit
```

**Azure DevOps:**
```yaml
# ❌ Variable group not linked
variables:
  - name: myVar
    value: 'myValue'

# ✅ Link variable group
variables:
  - group: 'veeam-credentials'
```

### Issue 9: Wrong VBR Environment

**Problem:** Applied config to production instead of dev.

**Cause:** Wrong OWLCTL_URL in environment variables.

**Prevention:**
```yaml
# Use GitHub Environments
jobs:
  deploy-prod:
    environment: production  # Requires approval + correct secrets
    steps:
      - name: Deploy
        env:
          OWLCTL_URL: ${{ secrets.VBR_URL }}  # production URL
```

**Detection:**
```yaml
# Add environment verification
- name: Verify environment
  run: |
    if [ "$OWLCTL_URL" != "vbr-prod.company.com" ]; then
      echo "::error::Wrong VBR URL! Expected production."
      exit 1
    fi
```

### Issue 10: owlctl Binary Not Found

**Problem:** `owlctl: command not found` in pipeline.

**Cause:** Binary not downloaded or not in PATH.

**Solution:**
```yaml
# Download and make executable
- name: Install owlctl
  run: |
    curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
    tar xzf owlctl.tar.gz
    chmod +x owlctl

# Use with explicit path
- name: Use owlctl
  run: ./owlctl get jobs  # Note: ./owlctl not just owlctl
```

**Alternative - add to PATH:**
```yaml
- name: Install owlctl to PATH
  run: |
    curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz -o owlctl.tar.gz
    tar xzf owlctl.tar.gz
    chmod +x owlctl
    sudo mv owlctl /usr/local/bin/

- name: Use owlctl
  run: owlctl get jobs  # Now in PATH
```

## See Also

- [Why GitOps for Backup Infrastructure](gitops-for-backup-infrastructure.md) - The conceptual case for this approach
- [Getting Started Guide](getting-started.md) - Basic owlctl setup
- [Declarative Mode Guide](declarative-mode.md) - Declarative commands reference
- [Azure DevOps Integration](azure-devops-integration.md) - Detailed Azure DevOps guide
- [Drift Detection Guide](drift-detection.md) - Comprehensive drift detection
- [Security Alerting](security-alerting.md) - Severity classification reference
- [State Management](state-management.md) - State file deep dive
- [Pipeline Examples](../examples/pipelines/) - Ready-to-use pipeline templates

---

**Next Steps:**
1. Choose your CI/CD platform (GitHub Actions, Azure DevOps, GitLab)
2. Set up secrets in your platform
3. Create your repository structure
4. Copy a pipeline template from this guide
5. Commit your first VBR configuration
6. Watch automation work! 🚀
