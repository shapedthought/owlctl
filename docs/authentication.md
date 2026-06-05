# Authentication and Setup

This guide covers initializing owlctl, managing profiles, and setting up authentication for all Veeam products.

## Table of Contents

- [First-Time Setup](#first-time-setup)
- [Profiles](#profiles)
- [Authentication Modes](#authentication-modes)
- [Logging In](#logging-in)
- [Switching Between Products](#switching-between-products)
- [Troubleshooting](#troubleshooting)

## First-Time Setup

### Initialize owlctl

Init is non-interactive by default for GitOps/CI/CD compatibility.

Run the init command to create configuration files:

```bash
# Non-interactive init (default)
./owlctl init

# With specific settings
./owlctl init --insecure --output-dir ~/.owlctl/

# Legacy interactive mode
./owlctl init --interactive
```

**Available Flags:**
- `--insecure` - Skip TLS verification (sets `apiNotSecure: true`)
- `--output-dir <path>` - Specify config file directory
- `--interactive` - Use legacy interactive prompts

**Files created:**
- `settings.json` - owlctl settings and preferences
- `profiles.json` - API profiles for each Veeam product (v1.0 format)

**Output:** Init now outputs JSON to stdout for automation/piping:
```json
{
  "version": "1.0",
  "settings": {...},
  "profiles": {...},
  "files": {
    "settings": "path/to/settings.json",
    "profiles": "path/to/profiles.json"
  }
}
```

### Configuration File Location

By default, files are created in the current directory. To use a custom location:

**Set before running init:**
```bash
# Bash/Zsh
export OWLCTL_SETTINGS_PATH="$HOME/.owlctl/"
mkdir -p ~/.owlctl
./owlctl init

# PowerShell
$env:OWLCTL_SETTINGS_PATH = "$HOME\.owlctl\"
New-Item -ItemType Directory -Path $HOME\.owlctl -Force
.\owlctl init
```

**Example paths:**
- Windows: `C:\Users\UserName\.owlctl\`
- macOS/Linux: `/home/username/.owlctl/`

**Note:** The directory must exist before running `init`.

## Profiles

Profiles define connection settings for each Veeam product. Each profile contains:
- API version headers
- Port numbers
- Authentication URL patterns
- Optional username and address (creds file mode only)

### Available Profiles

| Profile | Product | Port | Notes |
|---------|---------|------|-------|
| `vbr` | Veeam Backup & Replication | 9419 | OAuth authentication |
| `ent_man` | Enterprise Manager | 9398 | Basic authentication |
| `vb365` | Veeam Backup for Microsoft 365 | 4443 | OAuth authentication |
| `vone` | Veeam ONE | 1239 | OAuth authentication |
| `aws` | Veeam Backup for AWS | 11005 | OAuth authentication |
| `azure` | Veeam Backup for Azure | 443 | OAuth authentication |
| `gcp` | Veeam Backup for GCP | 13140 | OAuth authentication |

### Profile Commands

Profile commands require explicit arguments (no interactive prompts).

```bash
# List all available profiles
./owlctl profile --list
./owlctl profile -l

# Get current active profile (returns clean output: "vbr")
./owlctl profile --get
./owlctl profile -g

# Set active profile (requires explicit argument)
./owlctl profile --set vbr
./owlctl profile -s vbr

# View profile details
./owlctl profile --profile vbr
./owlctl profile -p vbr
```

**Breaking Change:** `./owlctl profile --set` now requires an argument. Previously it prompted interactively.

### Profile Structure (v1.0 Format)

profiles.json uses versioned format with all profiles in one file.

Example structure:

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
    },
    "ent_man": {
      "product": "EnterpriseManager",
      "apiVersion": "",
      "port": 9398,
      "endpoints": {
        "auth": "/api/sessionMngr/?v=latest",
        "apiPrefix": "/api"
      },
      "authType": "basic",
      "headers": {
        "accept": "application/json",
        "contentType": "application/json",
        "xAPIVersion": ""
      }
    }
  }
}
```

**Note:** Credentials are never stored in profiles.json. Always use environment variables.

### API Versions

Current default versions (as of October 2023):

| Product | Version | API Version |
|---------|---------|-------------|
| VBR | 13.0 | 1.3-rev1 |
| Enterprise Manager | 12.0 | - |
| VONE | 12.0 | v2.1 |
| VB365 | 7.0 | - |
| VB for AWS | 5.0 | 1.4-rev0 |
| VB for Azure | 5.0 | - |
| VB for GCP | 1.0 | 1.2-rev0 |

**To change API version:** Edit the `profiles.json` file and update the `api_version` or `x-api-version` field.

**Veeam API documentation:** https://www.veeam.com/documentation-guides-datasheets.html

## Credentials and Token Storage

Credentials always come from environment variables. Tokens are stored securely in the system keychain.

### Setting Credentials

owlctl requires three environment variables for authentication:

**Bash/Zsh (macOS/Linux):**
```bash
export OWLCTL_USERNAME="administrator"
export OWLCTL_PASSWORD="your-password"
export OWLCTL_URL="vbr.example.com"
export OWLCTL_SETTINGS_PATH="$HOME/.owlctl/"  # Optional
```

**PowerShell (Windows):**
```powershell
$env:OWLCTL_USERNAME = "administrator"
$env:OWLCTL_PASSWORD = "your-password"
$env:OWLCTL_URL = "vbr.example.com"
$env:OWLCTL_SETTINGS_PATH = "$HOME\.owlctl\"  # Optional
```

**Environment Variables:**

| Variable | Required | Description |
|----------|----------|-------------|
| `OWLCTL_USERNAME` | Yes* | API username (may need `DOMAIN\user` format) |
| `OWLCTL_PASSWORD` | Yes* | API password |
| `OWLCTL_URL` | Yes* | Server hostname/IP (without `https://` or port) |
| `OWLCTL_TOKEN` | No | Explicit authentication token (bypasses auto-auth) |
| `OWLCTL_SETTINGS_PATH` | No | Config file directory (default: current directory) |
| `OWLCTL_FILE_KEY` | No | File keyring password (for non-interactive systems) |

*Not required if `OWLCTL_TOKEN` is set

### Secure Token Storage

Authentication tokens are stored in your system's secure keychain instead of plaintext files.

#### Token Resolution Priority

owlctl attempts authentication in this order:

1. **`OWLCTL_TOKEN` environment variable** (highest priority)
   - For explicit token control
   - Useful for service accounts or long-lived tokens
   ```bash
   export OWLCTL_TOKEN="your-long-lived-token"
   ./owlctl get jobs
   ```

2. **System keychain** (interactive sessions only)
   - **macOS:** Keychain Access
   - **Windows:** Credential Manager
   - **Linux:** Secret Service (GNOME Keyring, KWallet)
   - Token stored encrypted by OS
   - Persists across owlctl sessions
   ```bash
   ./owlctl login  # Token stored in keychain
   ./owlctl get jobs  # Uses token from keychain
   ```

3. **Auto-authenticate** (CI/CD and non-TTY environments)
   - Detects non-interactive sessions automatically
   - Authenticates on-demand using `OWLCTL_USERNAME`/`OWLCTL_PASSWORD`/`OWLCTL_URL`
   - No keychain interaction on headless systems
   ```bash
   # GitHub Actions, GitLab CI, Jenkins, etc.
   ./owlctl get jobs  # Auto-authenticates using env vars
   ```

#### File-Based Keyring (Fallback)

If system keychain is unavailable, owlctl uses encrypted file storage (`~/.owlctl/owlctl-keyring`):

**Interactive systems:**
```bash
./owlctl login
# Prompts: Enter password for owlctl file keyring: _
```

**CI/CD systems:**
```bash
export OWLCTL_FILE_KEY="your-secure-password"
./owlctl login  # Uses OWLCTL_FILE_KEY for encryption
```

### Authentication Flow

**Interactive Session (Local Development):**
```bash
# 1. Set credentials
export OWLCTL_USERNAME="admin"
export OWLCTL_PASSWORD="pass"
export OWLCTL_URL="vbr.local"

# 2. Set profile
./owlctl profile --set vbr

# 3. Login (stores token in keychain)
./owlctl login
# Token stored in system keychain

# 4. Make API calls (uses keychain token)
./owlctl get jobs
./owlctl post jobs/<id>/start
```

**CI/CD Pipeline:**
```yaml
# GitHub Actions example
- name: Check VBR drift
  env:
    OWLCTL_USERNAME: ${{ secrets.VBR_USERNAME }}
    OWLCTL_PASSWORD: ${{ secrets.VBR_PASSWORD }}
    OWLCTL_URL: ${{ secrets.VBR_URL }}
  run: |
    ./owlctl profile --set vbr
    ./owlctl job diff --all --security-only
    # Auto-authenticates, no keychain interaction
```

**Explicit Token Control:**
```bash
# Use specific token
export OWLCTL_TOKEN="your-service-account-token"
./owlctl get jobs  # Bypasses keychain and auto-auth
```

## Logging In

After setting up credentials and selecting a profile, authenticate:

```bash
./owlctl login
```

**On success:**
- Stores authentication token in system keychain (interactive) or authenticates on-demand (CI/CD)
- Token used for all subsequent API calls
- Token valid until expiration

**Interactive Sessions:**
- Token stored in system keychain
- Persists across terminal sessions
- Encrypted by operating system

**CI/CD Environments:**
- Auto-authenticates using environment variables
- No keychain storage (non-TTY detection)
- Authenticates on-demand for each command

### Authentication Types by Product

**OAuth (most products):**
- VBR, VB365, VONE, AWS, Azure, GCP
- Uses Bearer token
- Token stored in system keychain

**Basic Auth (Enterprise Manager only):**
- Enterprise Manager uses session-based authentication
- Session token in `X-RestSvcSessionId` header
- Different URL pattern than other products

## Switching Between Products

### Method 1: Switch Credentials and Profile

```bash
# Connect to VBR
export OWLCTL_USERNAME="admin" OWLCTL_PASSWORD="pass1" OWLCTL_URL="vbr.example.com"
./owlctl profile --set vbr
./owlctl login
./owlctl get jobs

# Switch to VB365
export OWLCTL_USERNAME="admin" OWLCTL_PASSWORD="pass2" OWLCTL_URL="vb365.example.com"
./owlctl profile --set vb365
./owlctl login
./owlctl get organizations
```

Each profile gets its own token in the keychain (keyed by profile name).

### Method 2: Multiple Config Directories

Use separate directories for different environments:

```bash
# Production VBR
export OWLCTL_SETTINGS_PATH="$HOME/.owlctl/prod/"
export OWLCTL_USERNAME="prod-admin" OWLCTL_PASSWORD="prod-pass" OWLCTL_URL="vbr-prod.local"
./owlctl profile --set vbr
./owlctl login

# Development VBR
export OWLCTL_SETTINGS_PATH="$HOME/.owlctl/dev/"
export OWLCTL_USERNAME="dev-admin" OWLCTL_PASSWORD="dev-pass" OWLCTL_URL="vbr-dev.local"
./owlctl profile --set vbr
./owlctl login
```

### Method 3: Shell Functions for Quick Switching

```bash
# Add to ~/.bashrc or ~/.zshrc
vbr-prod() {
    export OWLCTL_USERNAME="prod-admin"
    export OWLCTL_PASSWORD="$VBR_PROD_PASS"
    export OWLCTL_URL="vbr-prod.local"
    ./owlctl profile --set vbr
    ./owlctl login
}

vbr-dev() {
    export OWLCTL_USERNAME="dev-admin"
    export OWLCTL_PASSWORD="$VBR_DEV_PASS"
    export OWLCTL_URL="vbr-dev.local"
    ./owlctl profile --set vbr
    ./owlctl login
}

# Usage
vbr-prod
./owlctl get jobs

vbr-dev
./owlctl get jobs
```

## Troubleshooting

### "Profile not set" Error

**Problem:** You tried to login without setting a profile.

**Solution:**
```bash
./owlctl profile --set vbr
./owlctl login
```

### "Authentication failed" or 401 Unauthorized

**Possible causes:**
1. Incorrect username or password
2. Wrong username format
3. Account locked or disabled
4. REST API not enabled

**Solutions:**

**Try different username formats:**
```bash
# Standard
export OWLCTL_USERNAME="administrator"

# Domain user (format 1)
export OWLCTL_USERNAME="DOMAIN\\administrator"

# Domain user (format 2)
export OWLCTL_USERNAME="administrator@domain.com"
```

**Check VBR REST API is enabled:**
1. Open VBR Console
2. Menu → Options → Network Traffic Rules
3. Ensure "RESTful API service" is checked

### "Connection refused" or "Dial tcp" Errors

**Possible causes:**
1. Wrong hostname/IP
2. Firewall blocking port
3. REST API service not running
4. Wrong profile selected

**Solutions:**

**Verify connectivity:**
```bash
# Test network connection
ping vbr.example.com

# Test port is open (requires telnet/nc)
telnet vbr.example.com 9419
# or
nc -zv vbr.example.com 9419
```

**Verify profile and URL:**
```bash
# Check current profile
./owlctl profile --get

# Verify OWLCTL_URL doesn't include protocol or port
echo $OWLCTL_URL
# Should be: vbr.example.com
# NOT: https://vbr.example.com:9419
```

### TLS Certificate Errors

**Problem:** `x509: certificate signed by unknown authority`

**Option 1: Skip TLS verification (NOT recommended for production)**

Edit `settings.json`:
```json
{
  "apiNotSecure": true
}
```

**Option 2: Trust the CA certificate (recommended)**

See [Getting Started Guide - Troubleshooting](getting-started.md#tls-certificate-errors) for detailed instructions.

### Files Created in Wrong Directory

**Problem:** `settings.json` and `profiles.json` not in expected location.

**Solution:**

1. Set `OWLCTL_SETTINGS_PATH` before running `init`:
```bash
export OWLCTL_SETTINGS_PATH="$HOME/.owlctl/"
mkdir -p ~/.owlctl
./owlctl init
```

2. Or move files manually:
```bash
mkdir -p ~/.owlctl
mv settings.json profiles.json ~/.owlctl/
export OWLCTL_SETTINGS_PATH="$HOME/.owlctl/"
```

### Token Expired

**Problem:** API calls failing with authentication errors after working previously.

**Solution:** Re-login to refresh the token in keychain:
```bash
./owlctl login
```

The keychain token is automatically refreshed.

### Token Storage Issues

**Problem:** `failed to open keyring` or `no authentication method available`

**Context:** System keychain unavailable (headless Linux, Docker, etc.)

**Solution:**

**Option 1: Use explicit token**
```bash
export OWLCTL_TOKEN="your-long-lived-token"
./owlctl get jobs
```

**Option 2: Set file keyring password**
```bash
# CI/CD environments
export OWLCTL_FILE_KEY="your-secure-password"
./owlctl login

# Interactive systems
./owlctl login
# Prompts for file keyring password
```

### CI/CD Auto-Authentication Not Working

**Problem:** Commands fail in CI/CD pipeline

**Cause:** Missing environment variables

**Solution:** Verify all required variables are set:
```yaml
# GitHub Actions
env:
  OWLCTL_USERNAME: ${{ secrets.VBR_USERNAME }}
  OWLCTL_PASSWORD: ${{ secrets.VBR_PASSWORD }}
  OWLCTL_URL: ${{ secrets.VBR_URL }}
```

Check environment variables in pipeline:
```bash
echo "Username: $OWLCTL_USERNAME"
echo "URL: $OWLCTL_URL"
# Don't echo password for security
```

## Best Practices

### Security

1. **Environment variables for credentials** - Never store credentials in configuration files
2. **Use secrets management** in CI/CD - Azure Key Vault, GitHub Secrets, AWS Secrets Manager, etc.
3. **Rotate credentials regularly** - Especially for service accounts
4. **Use dedicated service accounts** for automation - Separate accounts for CI/CD vs interactive use
5. **Trust system keychain** - Let owlctl manage token storage securely
6. **Use `OWLCTL_TOKEN` for long-lived tokens** - Service accounts with explicit token control

**What to commit to Git:**
- ✅ `settings.json` (no secrets)
- ✅ `profiles.json` (no credentials in v1.0 format)
- ✅ Configuration YAML files
- ❌ Never commit tokens or credentials
- ❌ state.json (optional - depends on workflow)

### Organization

1. **Use custom settings path** to keep config centralized:
   ```bash
   export OWLCTL_SETTINGS_PATH="$HOME/.owlctl/"
   ```

2. **Name config directories** clearly for multiple environments:
   - `~/.owlctl/prod/` - Production environment
   - `~/.owlctl/dev/` - Development environment
   - `~/.owlctl/customer-a/` - Customer-specific config

3. **Document environment setup** in project README:
   ```markdown
   # Setup
   export OWLCTL_SETTINGS_PATH="$HOME/.owlctl/prod/"
   export OWLCTL_USERNAME="prod-admin"
   export OWLCTL_URL="vbr-prod.example.com"
   ```

### Automation

1. **Set credentials in CI/CD secrets** - Use platform-native secret stores
2. **Let auto-authentication work** - owlctl detects CI/CD environments automatically
3. **Set `OWLCTL_SETTINGS_PATH` to build directory** to avoid conflicts
4. **Pass profile as argument** (not interactive):
```bash
#!/bin/bash
# CI/CD automation script
export OWLCTL_SETTINGS_PATH="./owlctl-config/"

# Non-interactive init
./owlctl init --output-dir ./owlctl-config/

# Set profile with argument (not interactive)
./owlctl profile --set vbr

# Auto-authenticates using OWLCTL_USERNAME/PASSWORD/URL from secrets
./owlctl job diff --all --security-only
```

5. **Handle exit codes properly** for drift detection:
```bash
./owlctl job diff --all --security-only
EXIT_CODE=$?
if [ $EXIT_CODE -eq 4 ]; then
    echo "CRITICAL drift detected"
    exit 1
fi
```

## See Also

- [Getting Started Guide](getting-started.md) - Complete setup walkthrough
- [Command Reference](command-reference.md) - Quick command reference
- [User Guide](../user_guide.md) - Full user documentation
