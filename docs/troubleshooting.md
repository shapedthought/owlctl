# Troubleshooting Guide

Common problems and solutions for owlctl. This guide covers authentication, connection, API, state management, and operational issues.

## Table of Contents

- [Quick Diagnostics](#quick-diagnostics)
- [Authentication Issues](#authentication-issues)
- [Connection Issues](#connection-issues)
- [TLS Certificate Issues](#tls-certificate-issues)
- [Profile Issues](#profile-issues)
- [API Issues](#api-issues)
- [State Management Issues](#state-management-issues)
- [Drift Detection Issues](#drift-detection-issues)
- [Apply Issues](#apply-issues)
- [Export Issues](#export-issues)
- [Overlay Issues](#overlay-issues)
- [Environment Variable Issues](#environment-variable-issues)
- [Platform-Specific Issues](#platform-specific-issues)

## Quick Diagnostics

Start troubleshooting with these quick checks:

```bash
# 1. Check owlctl version
owlctl utils
# Select "Check Version"

# 2. Verify environment variables
echo $OWLCTL_USERNAME
echo $OWLCTL_URL
echo $OWLCTL_SETTINGS_PATH

# 3. Check current profile
owlctl profile --get

# 4. Test connectivity
ping vbr.example.com
nc -zv vbr.example.com 9419

# 5. Verify authentication
owlctl login
```

## Authentication Issues

### 401 Unauthorized

**Problem:** Login fails with "Authentication failed" or 401 Unauthorized.

**Possible causes:**
1. Incorrect username or password
2. Wrong username format
3. Account locked or disabled
4. REST API not enabled (VBR)

**Solutions:**

**Try different username formats:**
```bash
# Standard
export OWLCTL_USERNAME="administrator"

# Domain user (format 1)
export OWLCTL_USERNAME="DOMAIN\\\\administrator"

# Domain user (format 2)
export OWLCTL_USERNAME="administrator@domain.com"

# Enterprise Manager format
export OWLCTL_USERNAME="administrator@"
```

**Check VBR REST API is enabled:**
1. Open VBR Console
2. Menu → Options → Network Traffic Rules
3. Ensure "RESTful API service" is checked
4. If unchecked, enable it and try again

**Verify credentials:**
```bash
# Test credentials manually
curl -k -u "username:password" https://vbr.example.com:9419/api/v1/jobs
```

**Check account status:**
- Verify account isn't locked in Active Directory
- Confirm account has appropriate VBR permissions
- Test login with Veeam Console to verify credentials

### Token Expired

**Problem:** API calls failing with authentication errors after working previously.

**Solution:** Re-login to get a fresh token:
```bash
owlctl login
```

**Note:** Tokens expire after a period of inactivity (varies by product).

### Profile Not Set

**Problem:** "Profile not set" error when trying to login.

**Solution:**
```bash
# Set the profile first
owlctl profile --set vbr

# Then login
owlctl login
```

### Token Storage Issues

**Problem:** `failed to open keyring` or `no authentication method available`

**Context:** System keychain unavailable (headless Linux, Docker, restricted environments)

**Solutions:**

**Option 1: Use explicit token (recommended for CI/CD)**
```bash
export OWLCTL_TOKEN="your-long-lived-token"
owlctl get jobs
```

**Option 2: File-based keyring with password**
```bash
# CI/CD environments
export OWLCTL_FILE_KEY="your-secure-password"
owlctl login

# Interactive systems
owlctl login
# Prompts: Enter password for owlctl file keyring: _
```

**Option 3: Auto-authenticate (requires credentials)**
```bash
# Set all three credentials
export OWLCTL_USERNAME="admin"
export OWLCTL_PASSWORD="pass"
export OWLCTL_URL="vbr.local"

# owlctl auto-authenticates when keychain unavailable
owlctl get jobs
```

### CI/CD Auto-Authentication Not Working

**Problem:** Commands fail in pipeline with "no authentication method available"

**Cause:** Missing required environment variables

**Solution:**

Check all required variables are set in CI/CD secrets:
```yaml
# GitHub Actions example
env:
  OWLCTL_USERNAME: ${{ secrets.VBR_USERNAME }}
  OWLCTL_PASSWORD: ${{ secrets.VBR_PASSWORD }}
  OWLCTL_URL: ${{ secrets.VBR_URL }}
```

Verify variables in pipeline:
```bash
echo "Username: $OWLCTL_USERNAME"
echo "URL: $OWLCTL_URL"
# Don't echo password for security

# Test authentication
owlctl login
owlctl get jobs
```

### Profile Command Requires Argument

**Problem:** `owlctl profile --set` hangs or returns "profile name required" error

**Breaking Change:** Profile commands now require explicit arguments (no interactive prompts)

**Old command (v0.10.x):**
```bash
owlctl profile --set
# Prompted: Enter profile name: _
```

**New command:**
```bash
owlctl profile --set vbr  # Provide argument
owlctl profile -s vbr     # Short form
```

**In scripts:**
```bash
# Update CI/CD pipelines
- script: owlctl profile --set vbr  # Add profile name
  displayName: 'Set Profile'
```

## Connection Issues

### Connection Refused

**Problem:** "Connection refused" or "dial tcp: connection refused"

**Possible causes:**
1. Wrong hostname/IP
2. Firewall blocking port
3. REST API service not running
4. Wrong profile selected (wrong port)

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

**Verify URL format:**
```bash
# Check OWLCTL_URL doesn't include protocol or port
echo $OWLCTL_URL
# Should be: vbr.example.com
# NOT: https://vbr.example.com:9419
```

**Check profile and port:**
```bash
# Verify correct profile is set
owlctl profile --get

# Check profile port matches product
owlctl profile --profile vbr
# VBR should be port 9419
# Enterprise Manager should be port 9398
```

**Check firewall rules:**
- Verify firewall allows connections to VBR port
- Test from another machine to isolate network issues
- Check Windows Firewall or iptables rules

### Timeout

**Problem:** Requests timeout without response.

**Possible causes:**
1. Network latency
2. VBR server overloaded
3. Large dataset causing slow response

**Solutions:**
```bash
# Test with a simple endpoint
owlctl get jobs | jq '.data | length'

# Check VBR server performance in console
# Look for high CPU or memory usage

# Try connecting from VBR server itself (local)
# to eliminate network issues
```

### DNS Resolution Failed

**Problem:** "no such host" error

**Solutions:**
```bash
# Test DNS resolution
nslookup vbr.example.com

# Use IP address instead of hostname
export OWLCTL_URL="192.168.0.123"
owlctl login

# Check /etc/hosts or C:\Windows\System32\drivers\etc\hosts
```

## TLS Certificate Issues

### Certificate Signed by Unknown Authority

**Problem:** `x509: certificate signed by unknown authority`

**Option 1: Skip TLS verification (NOT recommended for production)**

Edit `settings.json`:
```json
{
  "apiNotSecure": true
}
```

**Option 2: Trust the CA certificate (recommended)**

**Windows:**
1. Export certificate from VBR Console
2. Double-click certificate file
3. Install to "Trusted Root Certification Authorities"

**Linux:**
```bash
# Get certificate
echo | openssl s_client -connect vbr.example.com:9419 | \
    openssl x509 > vbr.crt

# Install certificate (Ubuntu/Debian)
sudo cp vbr.crt /usr/local/share/ca-certificates/
sudo update-ca-certificates

# Install certificate (RHEL/CentOS)
sudo cp vbr.crt /etc/pki/ca-trust/source/anchors/
sudo update-ca-trust
```

**macOS:**
```bash
# Get certificate
echo | openssl s_client -connect vbr.example.com:9419 | \
    openssl x509 > vbr.crt

# Install to keychain
sudo security add-trusted-cert -d -r trustRoot \
    -k /Library/Keychains/System.keychain vbr.crt
```

### Certificate Hostname Mismatch

**Problem:** "certificate is valid for X, not Y"

**Causes:**
- Using IP instead of hostname in OWLCTL_URL
- Certificate issued for different hostname

**Solutions:**
```bash
# Use the hostname from the certificate
export OWLCTL_URL="vbr.company.local"  # Match certificate CN

# Or skip TLS verification (not recommended for production)
```

## Profile Issues

### Files Created in Wrong Directory

**Problem:** `settings.json` and `profiles.json` not in expected location.

**Solution:**

1. Set `OWLCTL_SETTINGS_PATH` before running `init`:
```bash
export OWLCTL_SETTINGS_PATH="$HOME/.owlctl/"
mkdir -p ~/.owlctl
owlctl init
```

2. Or move files manually:
```bash
mkdir -p ~/.owlctl
mv settings.json profiles.json ~/.owlctl/
export OWLCTL_SETTINGS_PATH="$HOME/.owlctl/"

# Note: headers.json no longer exists
# Tokens stored in system keychain instead
```

### Profile Not Found

**Problem:** owlctl can't find the profile you specified.

**Solution:**
```bash
# List available profiles
owlctl profile --list

# Use exact profile name
owlctl profile --set vbr  # Not "VBR" or "vbr-prod"
```

### API Version Mismatch

**Problem:** API calls fail with version-related errors.

**Solution:**
Update API version in profiles.json:
```bash
# Edit profiles.json
vim $OWLCTL_SETTINGS_PATH/profiles.json

# Find the profile and update api_version or x-api-version
# VBR 12: "api_version": "1.1-rev2"
# VBR 13: "api_version": "1.3-rev1"
```

## API Issues

### Empty Response

**Problem:** Command succeeds but returns empty `{}`

**Causes:**
- Resource doesn't exist
- Wrong endpoint
- Permissions issue

**Solutions:**
```bash
# Verify endpoint works
owlctl get jobs  # Should show list

# Check profile
owlctl profile --get

# Verify authentication
owlctl login

# Test with known good endpoint
owlctl get backupInfrastructure/repositories
```

### JSON Parse Error

**Problem:** `invalid character '<' looking for beginning of value`

**Causes:**
- VBR returned HTML error page
- Wrong endpoint
- API service not running

**Solution:**
```bash
# Save response to see error
owlctl get jobs > response.txt
cat response.txt  # Check if HTML error page

# Common causes:
# - Wrong API version in URL
# - Service not running
# - Incorrect endpoint
```

### No Such File or Directory (with -f flag)

**Problem:** `owlctl post jobs -f data.json` fails

**Solutions:**
```bash
# Use absolute path
owlctl post jobs -f /full/path/to/data.json

# Or relative from current directory
owlctl post jobs -f ./data.json

# Verify file exists
ls -la data.json
```

### Rate Limiting

**Problem:** API calls fail with 429 Too Many Requests

**Solution:**
```bash
# Add delays between requests
for id in $(owlctl get jobs | jq -r '.data[].id'); do
    owlctl post jobs/$id/start
    sleep 2  # Wait 2 seconds between requests
done
```

## State Management Issues

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

# Verify state file was created
ls -la $OWLCTL_SETTINGS_PATH/state.json
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

## Drift Detection Issues

### Drift Detected After Apply

**Problem:** Diff shows drift immediately after applying

**Possible causes:**
1. Some fields may not be updateable via API
2. VBR may add default values not in the spec
3. VBR may normalize certain values (e.g., schedule times)

**Solutions:**
```bash
# Re-snapshot after a short delay
sleep 5
owlctl repo snapshot "Resource Name"

# Check drift severity (may be INFO and acceptable)
owlctl repo diff "Resource Name"

# Compare full configurations
owlctl repo export "Resource Name" -o current.yaml
diff applied.yaml current.yaml
```

### False Positive Drift

**Problem:** Diff reports drift but configurations look identical

**Causes:**
- Field order differences (not actual drift)
- Type differences (string "30" vs int 30)
- VBR adds computed fields not in spec

**Solutions:**
```bash
# Check severity - may be INFO (non-critical)
owlctl job diff "Job Name"

# Use --security-only to filter out INFO drifts
owlctl job diff --all --security-only

# Compare with exported current config
owlctl job export "Job Name" -o current.yaml
diff spec.yaml current.yaml
```

### No Drift Detection for Jobs

**Problem:** Can't manually snapshot jobs

**Explanation:**
Jobs are automatically snapshotted when you apply them. Manual job snapshots are not supported.

**Solution:**
```bash
# Apply the job to snapshot it
owlctl job apply job.yaml

# Then diff will work
owlctl job diff "Job Name"
```

## Apply Issues

### Resource Not Found (Exit Code 6)

**Problem:** Apply fails with "resource not found"

**Causes:**
Repositories, SOBRs, and KMS servers are update-only. They must be created in VBR console first.

**Solution:**
```bash
# 1. Create the resource in VBR console
# 2. Then apply can update it
owlctl repo apply repo.yaml  # Now succeeds

# Jobs support creation via apply
owlctl job apply job.yaml  # Creates if doesn't exist
```

### Partial Apply (Exit Code 5)

**Problem:** Apply succeeds but some fields were skipped

**Causes:**
Some fields may not be updateable via the VBR API.

**Solution:**
Review the output to see which fields were skipped. If they're important, you may need to update them manually in VBR console.

```bash
# Check which fields were skipped
owlctl repo apply repo.yaml --dry-run

# Update non-updateable fields manually in VBR console
```

### Apply Fails with Validation Error

**Problem:** Apply fails with "invalid configuration" or validation error

**Solutions:**
```bash
# Validate YAML syntax
cat job.yaml | yq '.'

# Check for required fields
owlctl job plan job.yaml --show-yaml

# Compare with exported working job
owlctl job export <working-job-id> -o working.yaml
diff job.yaml working.yaml

# Common issues:
# - Missing required fields
# - Invalid field values
# - Wrong resource type
```

### Apply Succeeds but Changes Not Visible

**Problem:** Apply reports success but VBR console doesn't show changes

**Solutions:**
```bash
# Refresh VBR console
# Press F5 or close/reopen

# Verify via API
owlctl get jobs/<job-id>

# Check if change is actually supported
# Some fields may be display-only
```

## Export Issues

### Export Returns Empty Configuration

**Problem:** Export command succeeds but produces minimal/empty YAML

**Causes:**
- Resource doesn't exist
- Wrong resource ID or name
- Permissions issue

**Solutions:**
```bash
# List all resources first
owlctl get jobs | jq '.data[] | {id, name}'

# Export by exact name
owlctl job export "Exact Job Name" -o job.yaml

# Export by ID
owlctl job export 57b3baab-6237-41bf-add7-db63d41d984c -o job.yaml

# Verify resource exists
owlctl get jobs/<job-id>
```

### Export Fails for Specific Resource

**Problem:** Export works for some resources but not others

**Solutions:**
```bash
# Check resource type
owlctl get jobs/<id> | jq '.type'

# Some job types may have export limitations
# Try simplified export for basic jobs
owlctl job export <id> --simplified -o job.yaml

# Check VBR logs for API errors
```

## Overlay Issues

### Overlay Not Being Applied

**Problem:** Overlay seems to be ignored

**Solutions:**

1. Check overlay resolution priority:
```bash
# Explicit -o flag has highest priority
owlctl job apply base.yaml --overlay overlay.yaml

# --env flag looks up environment in owlctl.yaml
owlctl job apply base.yaml --env production

# currentEnvironment in owlctl.yaml is used if no flags
owlctl job apply base.yaml
```

2. Verify owlctl.yaml exists and is in search path:
```bash
# Check OWLCTL_CONFIG
echo $OWLCTL_CONFIG

# Check current directory
ls -la owlctl.yaml

# Check home directory
ls -la ~/.owlctl/owlctl.yaml
```

3. Use `--show-yaml` to see the actual merged result:
```bash
owlctl job plan base.yaml --overlay overlay.yaml --show-yaml
```

### Unexpected Merge Results

**Problem:** Merged configuration doesn't look right

**Solutions:**

1. Remember: arrays are replaced, not merged
```yaml
# If your overlay has an objects array,
# it replaces the entire base array
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

**Problem:** Labels from base not appearing in merged config

**Solutions:**

1. Ensure both base and overlay use `metadata.labels` field
2. Labels should be at same level in both files:
```yaml
# Both files should have:
metadata:
  labels:
    key: value
```

## Environment Variable Issues

### Environment Variables Not Set

**Problem:** owlctl can't find credentials

**Solutions:**

**Bash/Zsh:**
```bash
# Set variables
export OWLCTL_USERNAME="administrator"
export OWLCTL_PASSWORD="password"
export OWLCTL_URL="vbr.example.com"

# Verify they're set
echo $OWLCTL_USERNAME
echo $OWLCTL_URL

# Make permanent (add to ~/.bashrc or ~/.zshrc)
echo 'export OWLCTL_USERNAME="administrator"' >> ~/.bashrc
```

**PowerShell:**
```powershell
# Set variables
$env:OWLCTL_USERNAME = "administrator"
$env:OWLCTL_PASSWORD = "password"
$env:OWLCTL_URL = "vbr.example.com"

# Verify they're set
$env:OWLCTL_USERNAME
$env:OWLCTL_URL

# Make permanent
[Environment]::SetEnvironmentVariable("OWLCTL_USERNAME", "administrator", "User")
```

### Variables Set But Not Working

**Problem:** Variables are set but owlctl doesn't see them

**Solutions:**
```bash
# Check for typos
env | grep OWLCTL

# Ensure no spaces around =
export OWLCTL_USERNAME="admin"  # Correct
export OWLCTL_USERNAME = "admin"  # Wrong

# Restart shell after setting variables
```

### Settings Path Not Found

**Problem:** owlctl can't find settings files

**Solutions:**
```bash
# Check OWLCTL_SETTINGS_PATH
echo $OWLCTL_SETTINGS_PATH

# Ensure directory exists
mkdir -p ~/.owlctl

# Set before running init
export OWLCTL_SETTINGS_PATH="$HOME/.owlctl/"
owlctl init

# Verify files are in correct location
ls -la $OWLCTL_SETTINGS_PATH/
```

## Platform-Specific Issues

### Windows PowerShell Escaping

**Problem:** Commands with special characters fail

**Solutions:**
```powershell
# Use double quotes for paths with spaces
owlctl job export <id> -o "C:\Users\Name\My Documents\job.yaml"

# Escape backslashes in paths
owlctl job export <id> -o "C:\\Users\\Name\\job.yaml"

# Use forward slashes (PowerShell accepts them)
owlctl job export <id> -o "C:/Users/Name/job.yaml"
```

### Linux Permissions

**Problem:** Permission denied errors

**Solutions:**
```bash
# Make owlctl executable
chmod +x owlctl

# Run with sudo if needed (not recommended)
sudo ./owlctl login

# Better: Fix ownership
sudo chown $USER:$USER owlctl
chmod +x owlctl
```

### macOS Gatekeeper

**Problem:** "owlctl cannot be opened because the developer cannot be verified"

**Solutions:**
```bash
# Remove quarantine attribute
xattr -d com.apple.quarantine owlctl

# Or allow in System Preferences
# System Preferences → Security & Privacy → Allow anyway
```

### Path Issues (All Platforms)

**Problem:** `owlctl: command not found`

**Solutions:**

**Use explicit path:**
```bash
./owlctl login
/path/to/owlctl login
```

**Add to PATH:**
```bash
# Linux/macOS
export PATH="$PATH:/path/to/owlctl"
echo 'export PATH="$PATH:/path/to/owlctl"' >> ~/.bashrc

# Windows PowerShell
$env:PATH += ";C:\path\to\owlctl"
```

## Getting Help

If you can't find a solution here:

1. **Check existing issues:** https://github.com/shapedthought/owlctl/issues
2. **Create a new issue:** https://github.com/shapedthought/owlctl/issues/new
3. **Include diagnostics:**
   - owlctl version (`owlctl utils` → Check Version)
   - Operating system
   - Veeam product and version
   - Error messages (full text)
   - Steps to reproduce

**Useful diagnostic commands:**
```bash
# Version info
owlctl utils  # Select "Check Version"

# Environment
env | grep OWLCTL

# Profile info
owlctl profile --get
owlctl profile --profile vbr

# Connectivity test
ping vbr.example.com
nc -zv vbr.example.com 9419

# API test
curl -k https://vbr.example.com:9419/api/v1/jobs
```

## See Also

- [Getting Started Guide](getting-started.md) - Initial setup
- [Authentication Guide](authentication.md) - Authentication troubleshooting
- [Imperative Mode Guide](imperative-mode.md) - API command troubleshooting
- [Declarative Mode Guide](declarative-mode.md) - Apply/export troubleshooting
- [State Management Guide](state-management.md) - State troubleshooting
- [Drift Detection Guide](drift-detection.md) - Drift detection issues
