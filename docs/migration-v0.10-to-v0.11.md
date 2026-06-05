# Migration Guide: v0.10.x to v0.11.0

**Impact:** Configuration files must be regenerated (clean break)
**Upgrade Time:** ~2 minutes

---

## Quick Upgrade

### 1. Backup Old Configs (Optional)
```bash
cp -r ~/.owlctl ~/.owlctl.old
```

### 2. Regenerate Profiles
```bash
owlctl init profiles
```

### 3. Regenerate Settings (if needed)
```bash
owlctl init settings          # Default: secure settings
owlctl init settings --insecure  # For lab environments
```

### 4. Set Environment Variables
```bash
export OWLCTL_USERNAME="your-username"
export OWLCTL_PASSWORD="your-password"
export OWLCTL_URL="your-vbr-server"
```

### 5. Re-Login
```bash
owlctl login
```

### 6. Test
```bash
owlctl get jobs
owlctl profile --list
```

---

## Breaking Changes Summary

| Change | Impact | Migration |
|--------|--------|-----------|
| Non-interactive init | Scripts work unchanged; interactive users add `--interactive` flag | Update scripts if using interactive mode |
| Profile commands | Must pass arguments instead of prompting | Add profile name as argument |
| Secure token storage | System keychain instead of plaintext files | Just re-login after upgrade |
| profiles.json v1.0 | Completely new structure | Regenerate with `owlctl init profiles` |
| Removed CredsFileMode | Credentials always from environment variables | Set environment variables |

---

## Breaking Changes in Detail

### 1. Non-Interactive Init by Default

`owlctl init` is now non-interactive by default and outputs JSON for piping/scripting.

**Before (v0.10.x):**
```bash
$ owlctl init
Use creds file mode? (y/N): _
Allow insecure TLS? (y/N): _
```

**After (v0.11.0):**
```bash
$ owlctl init
{
  "version": "1.0",
  "settings": { ... },
  "profiles": { ... },
  "files": { ... }
}
```

**For interactive mode:**
```bash
$ owlctl init --interactive
```

**Script update example:**
```bash
# Before (v0.10.x)
echo "n" | owlctl init  # Pipe answers to prompts

# After (v0.11.0)
owlctl init  # Non-interactive by default
# or
owlctl init --insecure  # For lab environments
```

### 2. Profile Commands Require Arguments

**Before (v0.10.x):**
```bash
$ owlctl profile --set
Enter profile name: vbr
```

**After (v0.11.0):**
```bash
$ owlctl profile --set vbr

$ owlctl profile --list
{"currentProfile":"vbr","availableProfiles":["vbr","vb365",...]}

$ owlctl profile --list --table   # Human-readable format
```

### 3. Secure Token Storage (System Keychain)

Tokens now stored in system keychain instead of plaintext `headers.json`:
- **macOS:** Keychain Access
- **Windows:** Credential Manager
- **Linux:** Secret Service (GNOME Keyring, KWallet)

**Authentication priority:**
```
1. OWLCTL_TOKEN environment variable (highest priority)
2. System keychain (interactive sessions)
3. Auto-authenticate with OWLCTL_USERNAME/OWLCTL_PASSWORD/OWLCTL_URL (CI/CD)
```

**File-based keyring fallback** (for systems without keychain):
```bash
# CI/CD environments
export OWLCTL_FILE_KEY="your-secure-password"
owlctl login

# Interactive systems
owlctl login
# Prompts: Enter password for owlctl file keyring: _
```

### 4. profiles.json v1.0 Format

Complete restructure with versioned format and multi-profile support.

**Before (v0.10.x):** Flat single-profile file
**After (v0.11.0):** Versioned map of all profiles

```json
{
  "version": "1.0",
  "currentProfile": "vbr",
  "profiles": {
    "vbr": {
      "product": "VeeamBackupReplication",
      "apiVersion": "1.3-rev1",
      "port": 9419,
      "endpoints": { "auth": "/api/oauth2/token", "apiPrefix": "/api/v1" },
      "authType": "oauth",
      "headers": { ... }
    }
  }
}
```

**Migration:** `owlctl init profiles`

### 5. Removed CredsFileMode

Credentials now always come from environment variables. No more `--creds-file` option.

```bash
export OWLCTL_USERNAME="administrator"
export OWLCTL_PASSWORD="your-password"
export OWLCTL_URL="vbr.example.com"
```

---

## Upgrade Scenarios

### Interactive User (Local Development)

```bash
cp -r ~/.owlctl ~/.owlctl.old   # Backup
owlctl init profiles
owlctl init settings
export OWLCTL_USERNAME="administrator"
export OWLCTL_PASSWORD="your-password"
export OWLCTL_URL="vbr-lab.local"
owlctl login
owlctl get jobs  # Test
```

### CI/CD Pipeline

Most pipelines need minimal or no changes.

**No changes needed:**
- Environment variables (already standard)
- `owlctl init` (already non-interactive)
- `owlctl get/post/put` commands

**Update profile commands:**
```yaml
# Before
- run: echo "aws" | owlctl profile --set

# After
- run: owlctl profile --set aws
```

**GitHub Actions example:**
```yaml
name: Drift Detection
on:
  schedule:
    - cron: '0 */6 * * *'

jobs:
  check-drift:
    runs-on: ubuntu-latest
    steps:
      - name: Download owlctl
        run: |
          wget https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64
          chmod +x owlctl-linux-amd64
          mv owlctl-linux-amd64 owlctl

      - name: Check drift
        env:
          OWLCTL_USERNAME: ${{ secrets.OWLCTL_USERNAME }}
          OWLCTL_PASSWORD: ${{ secrets.OWLCTL_PASSWORD }}
          OWLCTL_URL: ${{ secrets.OWLCTL_URL }}
        run: ./owlctl job diff --all --security-only
```

### Docker Container

```dockerfile
FROM ubuntu:latest
RUN wget https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64 && \
    chmod +x owlctl-linux-amd64 && \
    mv owlctl-linux-amd64 /usr/local/bin/owlctl
RUN owlctl init profiles
# At runtime, pass credentials via environment variables
```

---

## API Version Updates

v0.11.0 includes updated API versions:

| Product | v0.10.x | v0.11.0 |
|---------|---------|---------|
| VBR | 1.1-rev0 | 1.3-rev1 |
| VB365 | v6/v7 (mixed) | v8 |
| VB for AWS | 1.3-rev0 | 1.4-rev0 |
| VB for Azure | v4 | v5 |
| VB for GCP | 1.1-rev0 | 1.2-rev0 |
| VONE | 1.0-rev1 | 1.0-rev2 |

These updates are automatically included when you run `owlctl init profiles`.

---

## Testing Your Migration

### Step 1: Test Locally

```bash
./owlctl init --output-dir /tmp/owlctl-test/
ls -la /tmp/owlctl-test/
OWLCTL_SETTINGS_PATH=/tmp/owlctl-test/ ./owlctl profile --set vbr
OWLCTL_SETTINGS_PATH=/tmp/owlctl-test/ ./owlctl profile --get
rm -rf /tmp/owlctl-test/
```

### Step 2: Test in CI/CD

```yaml
- job: Test_Migration
  steps:
    - script: |
        ./owlctl init --output-dir .owlctl/
        TEST_PROFILE=$(OWLCTL_SETTINGS_PATH=.owlctl/ ./owlctl profile --set vbr && OWLCTL_SETTINGS_PATH=.owlctl/ ./owlctl profile --get)
        if [ "$TEST_PROFILE" != "vbr" ]; then
          echo "Migration test failed"
          exit 1
        fi
        echo "Migration test passed"
```

---

## Rollback Plan

### Option 1: Use Legacy Mode

```bash
./owlctl init --interactive
```

### Option 2: Downgrade to v0.10.x

v0.10.x predates the rebrand, so it uses `vcli` and `~/.vcli`:

```bash
cp -r ~/.owlctl.old ~/.vcli
wget https://github.com/shapedthought/owlctl/releases/download/v0.10.0-beta1/vcli-linux
chmod +x vcli-linux
./vcli-linux get jobs
```

**Warning:** v0.10.x is no longer maintained.

---

## Troubleshooting

### Error: Invalid profiles.json format

```
Error: Invalid profiles.json format (legacy v0.10.x detected)
Run 'owlctl init profiles' to regenerate
```

**Fix:** `owlctl init profiles && owlctl login`

### Error: Authentication failed

```
Authentication failed: OWLCTL_USERNAME or OWLCTL_URL environment variable not set
```

**Fix:**
```bash
export OWLCTL_USERNAME="administrator"
export OWLCTL_PASSWORD="your-password"
export OWLCTL_URL="vbr.example.com"
owlctl login
```

### File keyring password prompt (Linux/headless)

```
Enter password for owlctl file keyring: _
```

**CI/CD fix:** `export OWLCTL_FILE_KEY="your-secure-password"`

### Profile commands return JSON

This is expected. For human-readable output: `owlctl profile --list --table`

---

## Getting Help

- **Documentation:** [GitHub Repository](https://github.com/shapedthought/owlctl)
- **Getting Started:** [getting-started.md](getting-started.md)
- **Issues:** [GitHub Issues](https://github.com/shapedthought/owlctl/issues)
