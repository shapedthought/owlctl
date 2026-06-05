# Getting Started with owlctl

This guide walks you through installing owlctl, setting up authentication, and choosing the right workflow for your needs.

## Prerequisites

- Access to a Veeam API endpoint (VBR, Enterprise Manager, VB365, VONE, or VB for cloud)
- API credentials (username and password)
- Network access to the Veeam server

## Installation

### Download

1. Go to the [Releases page](https://github.com/shapedthought/owlctl/releases)
2. Download the appropriate binary for your platform:
   - `owlctl-windows-amd64.tar.gz` - Windows
   - `owlctl-linux-amd64.tar.gz` - Linux
   - `owlctl-darwin-amd64.tar.gz` - macOS (Intel)
   - `owlctl-darwin-arm64.tar.gz` - macOS (Apple Silicon)

### Extract and Install

**macOS/Linux:**
```bash
tar xzf owlctl-linux-amd64.tar.gz
chmod +x owlctl

# Optional: move to a directory in your PATH
sudo mv owlctl /usr/local/bin/
```

**Windows (PowerShell):**
```powershell
tar xzf owlctl-windows-amd64.tar.gz

# Optional: move to an existing PATH directory
Move-Item owlctl.exe C:\Windows\System32\owlctl.exe
```

### Verify Checksum

Compare the archive checksum with the `.sha256` file in the release:

```bash
sha256sum owlctl-linux-amd64.tar.gz
```

## First-Time Setup

### 1. Initialize owlctl

Create the configuration files non-interactively:

```bash
# Basic init (creates files with defaults)
./owlctl init

# With specific directory
./owlctl init --output-dir ~/.owlctl/

# With flags for specific settings
./owlctl init --insecure
```

Init is non-interactive by default and outputs JSON to stdout. For legacy interactive mode, use `./owlctl init --interactive`.

This creates:
- `settings.json` - owlctl settings
- `profiles.json` - API profiles for each Veeam product (v1.0 format with all profiles)

**Configuration Location:**
- By default, files are created in the current directory
- Use `--output-dir` flag or set `OWLCTL_SETTINGS_PATH` environment variable to use a specific directory

**Available Flags:**
- `--insecure` - Skip TLS verification (sets `apiNotSecure: true`)
- `--output-dir <path>` - Specify where to write config files
- `--interactive` - Use legacy interactive prompts

**Subcommands:**
```bash
# Initialize only settings
./owlctl init settings --insecure

# Initialize only profiles
./owlctl init profiles
```

> **What `init` creates:** `settings.json` (owlctl settings) and `profiles.json` (Veeam product API profiles). It does **not** create `owlctl.yaml` or `state.json` — those come later.

### Configuration Files at a Glance

| File | Created by | Purpose |
|------|-----------|---------|
| `settings.json` | `owlctl init` | Active profile, TLS setting, default instance |
| `profiles.json` | `owlctl init` | Per-product API metadata (ports, endpoints, headers) |
| `headers.json` | `owlctl login` | Auth token for the current session |
| `owlctl.yaml` | `owlctl instance add` | Named instance and group definitions |
| `state.json` | first `snapshot` or `apply` | Drift detection baseline for managed resources |

### 2. Set Credentials

owlctl reads credentials from environment variables:

**Bash/Zsh (macOS/Linux):**
```bash
export OWLCTL_USERNAME="administrator"
export OWLCTL_PASSWORD="your-password"
export OWLCTL_URL="vbr.example.com"
```

**PowerShell (Windows):**
```powershell
$env:OWLCTL_USERNAME = "administrator"
$env:OWLCTL_PASSWORD = "your-password"
$env:OWLCTL_URL = "vbr.example.com"
```

**Notes:**
- `OWLCTL_URL` should be the hostname or IP without `https://` or port
- Domain users: Use format `DOMAIN\username` or `username@domain.com`
- Port is handled automatically based on the selected profile

### 3. Select Profile

Set the Veeam product you're connecting to:

```bash
# List available profiles
./owlctl profile --list

# Set profile directly (no prompts)
./owlctl profile --set vbr

# Get current profile (returns just "vbr")
./owlctl profile --get
```

> **Using instances?** When you use `--instance` or `owlctl instance set`, the profile is set automatically from the instance definition. You can skip this step if you're going straight to the instance-based workflow below.

**Available Profiles:**
- `vbr` - Veeam Backup & Replication (port 9419)
- `ent_man` - Enterprise Manager (port 9398)
- `vb365` - Veeam Backup for Microsoft 365 (port 4443)
- `vone` - Veeam ONE (port 1239)
- `aws` - Veeam Backup for AWS (port 11005)
- `azure` - Veeam Backup for Azure (port 443)
- `gcp` - Veeam Backup for GCP (port 13140)

### 4. Register an Instance (Recommended for Declarative Mode)

If you plan to use declarative mode, register your server as a named instance. This creates `owlctl.yaml` and lets you avoid passing connection flags on every command.

```bash
# Register a VBR instance (creates owlctl.yaml if it doesn't exist)
./owlctl instance add vbr-prod \
  --url vbr-prod.example.com \
  --product vbr \
  --description "Production VBR"

# Set it as the default — no --instance flag needed from here on
./owlctl instance set vbr-prod

# Verify
./owlctl instance get
```

**With separate credentials per instance** (for multi-server setups):
```bash
export OWLCTL_PROD_USERNAME="administrator"
export OWLCTL_PROD_PASSWORD="your-password"

./owlctl instance add vbr-prod \
  --url vbr-prod.example.com \
  --product vbr \
  --credential-ref PROD
```

**Managing multiple environments (e.g. VBR + Azure)?** Use `context` commands to switch between them:

```bash
./owlctl instance add azure-prod --url azure.example.com --product azure --credential-ref AZURE

# Switch context when working on a different environment
./owlctl context use azure-prod
./owlctl get policies

# Switch back
./owlctl context use vbr-prod
./owlctl context list    # see all environments, * marks the active one
```

> Skip this step if you're only using imperative mode with environment variables.

### 5. Login

Authenticate with the Veeam API:

```bash
./owlctl login
```

Tokens are stored securely:
- **Interactive sessions:** System keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service)
- **CI/CD pipelines:** Auto-authentication using environment variables (no keychain interaction)

**Troubleshooting:**
- **TLS certificate errors**: If using self-signed certificates, see [Troubleshooting](#troubleshooting) below
- **Authentication failed**: Verify username format and credentials
- **Connection refused**: Check firewall rules and that the REST API is enabled

## Choose Your Workflow

owlctl supports two distinct modes of operation. Choose based on your use case.

### Imperative Mode

**Best for:**
- Quick API operations
- One-off tasks
- Exploring the API
- Products without declarative support (VB365, VONE, cloud products)

**Example: Start a backup job**
```bash
# List all jobs
./owlctl get jobs

# Get specific job details
./owlctl get jobs/57b3baab-6237-41bf-add7-db63d41d984c

# Start a job
./owlctl post jobs/57b3baab-6237-41bf-add7-db63d41d984c/start

# Get job status
./owlctl get jobs/57b3baab-6237-41bf-add7-db63d41d984c
```

**Key Commands:**
- `owlctl get <endpoint>` - Retrieve data
- `owlctl post <endpoint>` - Trigger operations (with optional `-f data.json`)
- `owlctl put <endpoint> -f data.json` - Update resources

See the [Imperative Mode Guide](imperative-mode.md) for complete documentation.

### Declarative Mode (VBR Only)

**Best for:**
- Infrastructure-as-code
- Version control and GitOps
- Multi-environment deployments (dev/staging/prod)
- Drift detection and security monitoring
- Automated remediation in CI/CD

**Example: Manage backup jobs with Git**

#### 1. Export Existing Configuration

```bash
# Export all resource types in one command (recommended for first-time setup)
./owlctl export --all -d infrastructure/

# Or export per resource type (useful when you need overlay support or selective exports)
./owlctl job export c07c7ea3-0471-43a6-af57-c03c0d82354a -o prod-backup.yaml
./owlctl job export --all -d jobs/
./owlctl repo export --all -d repos/
./owlctl repo sobr-export --all -d sobrs/
./owlctl encryption kms-export --all -d kms/
```

#### 2. Edit Configuration

Open `prod-backup.yaml` in your editor and make changes:

```yaml
apiVersion: owlctl.veeam.com/v1
kind: VBRJob
metadata:
  name: Production Database Backup
spec:
  description: Updated description
  storage:
    retentionPolicy:
      quantity: 30  # Changed from 7 to 30 days
  schedule:
    daily:
      localTime: "02:00"  # Changed from 22:00
```

#### 3. Preview Changes

```bash
# Dry-run to see what would change
./owlctl job apply prod-backup.yaml --dry-run
```

#### 4. Apply Configuration

```bash
# Apply changes to VBR
./owlctl job apply prod-backup.yaml
```

#### 5. Commit to Git

```bash
git add prod-backup.yaml
git commit -m "Update retention to 30 days for prod backup"
git push
```

#### 6. Snapshot State

Before drift detection can work, owlctl needs a baseline. Every `apply` saves state automatically. For resources you haven't applied yet (repositories, SOBRs, encryption, KMS), take a snapshot first:

```bash
# Snapshot all resource types at once (recommended)
./owlctl snapshot --all

# Or snapshot per resource type
./owlctl repo snapshot --all
./owlctl repo sobr-snapshot --all
./owlctl encryption snapshot --all
./owlctl encryption kms-snapshot --all
```

> **Why?** `diff` compares the live VBR configuration against the last saved state. Without a snapshot or prior apply, there is nothing to compare against and diff will report that resources are untracked. See the [State Management Guide](state-management.md) for a full explanation.

#### 7. Detect Drift

Someone makes a manual change in VBR? owlctl will detect it:

```bash
# Check for drift
./owlctl job diff "Production Database Backup"

# Check all jobs for security-relevant drift
./owlctl job diff --all --security-only
```

**Output:**
```
Checking drift for job: Production Database Backup

CRITICAL: 1 security-relevant changes detected

Drift detected:
  CRITICAL ~ storage.retentionPolicy.quantity: 30 (state) -> 7 (VBR)

Summary:
  - 1 drifts detected
  - Highest severity: CRITICAL
```

**Key Commands:**
- `owlctl job export <id>` - Export resource to YAML
- `owlctl job apply <file>` - Apply job configuration
- `owlctl job diff <name>` - Detect drift
- `owlctl repo apply <file>` - Apply repository configuration
- `owlctl repo sobr-apply <file>` - Apply SOBR configuration
- `owlctl encryption kms-apply <file>` - Apply KMS server configuration

See [Drift Detection Guide](drift-detection.md) for complete documentation.

## Group-Based Workflow (Declarative)

Groups are the recommended way to manage multiple specs with shared defaults and policy overrides. Define groups and instances in `owlctl.yaml`.

### 1. Export Specs and Register Instance

```bash
# Export existing jobs
./owlctl job export --all -d specs/jobs/

# Register instance (creates owlctl.yaml if it doesn't exist)
./owlctl instance add vbr-prod \
  --url vbr-prod.example.com \
  --product vbr \
  --credential-ref PROD \
  --description "Production VBR"

# Set as default — no --instance flag needed from here on
./owlctl instance set vbr-prod

# Add a groups section to owlctl.yaml
cat >> owlctl.yaml <<'EOF'

groups:
  sql-tier:
    description: SQL Server backups
    instance: vbr-prod
    profile: profiles/gold.yaml
    overlay: overlays/compliance.yaml
    specs:
      - specs/jobs/sql-vm-01.yaml
      - specs/jobs/sql-vm-02.yaml
EOF
```

### 2. Apply the Group

```bash
# List groups
./owlctl group list

# Dry-run first
./owlctl job apply --group sql-tier --dry-run

# Apply (instance activated from group definition)
./owlctl job apply --group sql-tier

# Drift check
./owlctl job diff --group sql-tier
```

### 3. Track in Git

```bash
git add owlctl.yaml profiles/ specs/ overlays/
git commit -m "Add group-based backup configuration"
git push
```

### Simpler Alternative: Single-File Overlay

For simpler setups without groups, use the `-o` flag directly:

```bash
./owlctl job apply base-backup.yaml --overlay overlays/prod.yaml
./owlctl job apply base-backup.yaml --overlay overlays/dev.yaml
```

See [Declarative Mode Guide](declarative-mode.md#groups) for the full groups reference.

## Next Steps

### Imperative Mode Users

- Read the [Imperative Mode Guide](imperative-mode.md) for detailed command reference
- Learn about [API profiles](../user_guide.md#profiles) for multi-product management
- Explore [output formatting](../user_guide.md#using-with-nushell) with jq and Nushell

### Declarative Mode Users

- **Start here:** [State Management Guide](state-management.md) - How state works, instance scoping, and bootstrapping drift detection
- [GitOps Workflows Guide](gitops-workflows.md) - Comprehensive CI/CD integration
- [Drift Detection](drift-detection.md) - Severity classification and filtering
- [Security Alerting](security-alerting.md) - Value-aware severity reference
- [Azure DevOps Integration](azure-devops-integration.md) for CI/CD
- [Pipeline templates](../examples/pipelines/) for automation

### Both

- Join the community on GitHub Discussions
- Report issues or request features on [GitHub Issues](https://github.com/shapedthought/owlctl/issues)
- Check the [Changelog](../README.md#change-log) for latest updates

## Troubleshooting

### TLS Certificate Errors

**Problem:** `x509: certificate signed by unknown authority`

**Solution:** Trust the certificate or use a CA bundle.

**Option 1: Skip verification (NOT recommended for production)**

Pass `--insecure` on any command, or set in `settings.json`:
```json
{
  "apiNotSecure": true
}
```

**Option 2: Trust the CA certificate (recommended)**

```bash
# Export VBR certificate
openssl s_client -connect vbr.example.com:9419 -showcerts > vbr-cert.pem

# macOS: Add to keychain
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain vbr-cert.pem

# Linux: Add to CA bundle
sudo cp vbr-cert.pem /usr/local/share/ca-certificates/vbr.crt
sudo update-ca-certificates
```

### Authentication Failures

**Problem:** `401 Unauthorized` or `failed to login`

**Possible causes:**
1. Incorrect credentials
2. Wrong username format (try `DOMAIN\user` or `user@domain.com`)
3. Account locked or disabled
4. REST API not enabled on VBR

**Check REST API is enabled (VBR):**
1. Open VBR console
2. Menu → Options → Network Traffic Rules
3. Ensure "RESTful API service" is enabled

### Connection Refused

**Problem:** `connection refused` or `no route to host`

**Check:**
1. Firewall rules allow access to the API port
2. VBR/product is running
3. REST API service is started
4. Network connectivity: `ping vbr.example.com`

### Profile Not Set

**Problem:** `profile not set`

**Solution:**
```bash
./owlctl profile --set vbr
```

### State File Corruption

**Problem:** `failed to parse state.json`

**Solution:**
```bash
# Backup current state
cp ~/.owlctl/state.json ~/.owlctl/state.json.backup

# Reset state (WARNING: loses drift detection history)
rm ~/.owlctl/state.json

# Re-snapshot resources
./owlctl snapshot --all
./owlctl config-backup snapshot          # Singleton — not included in snapshot --all
```

### Resource Not Found (Exit Code 6)

**Problem:** `owlctl repo apply` returns exit code 6

**Explanation:** Repositories, SOBRs, and KMS servers are **update-only**. They must be created in the VBR console first, then managed via owlctl.

**Solution:**
1. Create the resource in VBR console
2. Export it: `./owlctl repo export "Repository Name" -o repo.yaml`
3. Now you can apply changes: `./owlctl repo apply repo.yaml`

## Environment Variables Reference

| Variable | Required | Description |
|----------|----------|-------------|
| `OWLCTL_USERNAME` | Yes | API username |
| `OWLCTL_PASSWORD` | Yes | API password |
| `OWLCTL_URL` | Yes | Veeam server hostname/IP (without https:// or port) |
| `OWLCTL_SETTINGS_PATH` | No | Directory for config files (default: current directory) |
| `OWLCTL_CONFIG` | No | Path to owlctl.yaml |
| `OWLCTL_{REF}_USERNAME` | No | Username for named credential ref (e.g., `OWLCTL_PROD_USERNAME`) |
| `OWLCTL_{REF}_PASSWORD` | No | Password for named credential ref (e.g., `OWLCTL_PROD_PASSWORD`) |

## Exit Codes

### Apply Commands

| Code | Meaning |
|------|---------|
| `0` | Success - applied successfully |
| `1` | Error - API failure or invalid spec |
| `5` | Partial apply - some fields skipped (known immutable) |
| `6` | Resource not found - cannot apply (update-only resources) |

### Diff Commands

| Code | Meaning |
|------|---------|
| `0` | No drift detected |
| `3` | Drift detected (INFO or WARNING severity) |
| `4` | Critical drift detected (CRITICAL severity) |
| `1` | Error occurred |

Use exit codes in scripts:
```bash
./owlctl job diff --all --security-only
if [ $? -eq 4 ]; then
    echo "CRITICAL security drift detected!"
    exit 1
fi
```
