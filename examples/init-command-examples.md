# Init Command Examples

Quick reference for the non-interactive `init` command.

## Basic Usage

### Simple Init (Defaults)

```bash
# Non-interactive, creates files with defaults
./owlctl init
```

**Output:**
```json
{
  "settings": {
    "selectedProfile": "vbr",
    "apiNotSecure": false,
    "credsFileMode": false
  },
  "profiles": [...],
  "files": {
    "settings": "settings.json",
    "profiles": "profiles.json"
  }
}
```

### Init with Custom Directory

```bash
# Write files to specific directory
./owlctl init --output-dir ~/.owlctl/

# Write to build directory (CI/CD)
./owlctl init --output-dir $CI_PROJECT_DIR/.owlctl/

# Write to relative path
./owlctl init --output-dir ./config/
```

### Init with Configuration Flags

```bash
# Skip TLS verification (insecure)
./owlctl init --insecure

# Enable credentials file mode
./owlctl init --creds-file

# Combine flags
./owlctl init --insecure --creds-file --output-dir ~/.owlctl/
```

## Subcommands

### Initialize Settings Only

```bash
# Create only settings.json
./owlctl init settings

# With flags
./owlctl init settings --insecure --creds-file --output-dir ~/.owlctl/
```

**Output:**
```json
{
  "settings": {
    "selectedProfile": "vbr",
    "apiNotSecure": true,
    "credsFileMode": false
  },
  "file": "/home/user/.owlctl/settings.json"
}
```

### Initialize Profiles Only

```bash
# Create only profiles.json
./owlctl init profiles

# With output directory
./owlctl init profiles --output-dir ~/.owlctl/
```

**Output:**
```json
{
  "profiles": [
    {
      "name": "vbr",
      "port": "9419",
      ...
    },
    ...
  ],
  "file": "/home/user/.owlctl/profiles.json"
}
```

## JSON Output and Piping

### Extract Specific Fields

```bash
# Get settings only
./owlctl init | jq '.settings'

# Get profiles only
./owlctl init | jq '.profiles'

# Get file paths
./owlctl init | jq '.files'

# Count profiles
./owlctl init | jq '.profiles | length'
# Output: 7
```

### Save to Custom Locations

```bash
# Split settings and profiles into different files
./owlctl init | jq '.settings' > /opt/owlctl/settings.json
./owlctl init | jq '.profiles' > /opt/owlctl/profiles.json

# Save to environment-specific directories
ENV=prod
./owlctl init --insecure | jq '.settings' > ~/.owlctl/$ENV/settings.json
./owlctl init | jq '.profiles' > ~/.owlctl/$ENV/profiles.json
```

### Conditional Init

```bash
# Only init if files don't exist
if [ ! -f ~/.owlctl/settings.json ]; then
    ./owlctl init --output-dir ~/.owlctl/
fi

# Check if init was successful
if ./owlctl init --output-dir ~/.owlctl/ > /dev/null 2>&1; then
    echo "Init successful"
else
    echo "Init failed"
    exit 1
fi
```

## CI/CD Examples

### GitHub Actions

```yaml
name: VBR Drift Detection
on: [push]

jobs:
  check-drift:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Download owlctl
        run: |
          curl -fsSL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz | tar xz
          chmod +x owlctl

      - name: Initialize owlctl
        run: |
          ./owlctl init --output-dir .owlctl/ --insecure
          ./owlctl profile --set vbr
        env:
          OWLCTL_SETTINGS_PATH: ${{ github.workspace }}/.owlctl/

      - name: Login and check drift
        run: |
          ./owlctl login
          ./owlctl job diff --all --security-only
        env:
          OWLCTL_USERNAME: ${{ secrets.VBR_USERNAME }}
          OWLCTL_PASSWORD: ${{ secrets.VBR_PASSWORD }}
          OWLCTL_URL: ${{ secrets.VBR_URL }}
          OWLCTL_SETTINGS_PATH: ${{ github.workspace }}/.owlctl/
```

### Azure DevOps

```yaml
trigger:
  - master

pool:
  vmImage: 'ubuntu-latest'

steps:
  # Pin owlctlVersion to a release tag in the template
  - template: pipelines/templates/install-owlctl.yml

  - script: |
      ./owlctl init --output-dir $(Build.SourcesDirectory)/.owlctl/ --insecure
      ./owlctl profile --set vbr
    displayName: 'Initialize owlctl'
    env:
      OWLCTL_SETTINGS_PATH: $(Build.SourcesDirectory)/.owlctl/

  - script: |
      ./owlctl login
      ./owlctl job diff --all --security-only
    displayName: 'Check for drift'
    env:
      OWLCTL_USERNAME: $(OWLCTL_USERNAME)
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
      OWLCTL_URL: $(OWLCTL_URL)
      OWLCTL_SETTINGS_PATH: $(Build.SourcesDirectory)/.owlctl/
```

### GitLab CI

```yaml
vbr-drift-check:
  stage: test
  script:
    - curl -fsSL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz | tar xz
    - chmod +x owlctl
    - ./owlctl init --output-dir .owlctl/ --insecure
    - ./owlctl profile --set vbr
    - ./owlctl login
    - ./owlctl job diff --all --security-only
  variables:
    OWLCTL_SETTINGS_PATH: $CI_PROJECT_DIR/.owlctl/
    OWLCTL_USERNAME: $VBR_USERNAME
    OWLCTL_PASSWORD: $VBR_PASSWORD
    OWLCTL_URL: $VBR_URL
```

### Jenkins Pipeline

```groovy
pipeline {
    agent any

    environment {
        OWLCTL_SETTINGS_PATH = "${WORKSPACE}/.owlctl/"
        OWLCTL_USERNAME = credentials('vbr-username')
        OWLCTL_PASSWORD = credentials('vbr-password')
        OWLCTL_URL = credentials('vbr-url')
    }

    stages {
        stage('Setup owlctl') {
            steps {
                sh '''
                    curl -fsSL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz | tar xz
                    chmod +x owlctl
                    ./owlctl init --output-dir ${OWLCTL_SETTINGS_PATH} --insecure
                    ./owlctl profile --set vbr
                '''
            }
        }

        stage('Check Drift') {
            steps {
                sh '''
                    ./owlctl login
                    ./owlctl job diff --all --security-only
                '''
            }
        }
    }
}
```

## Docker Examples

### Dockerfile with Init

```dockerfile
FROM alpine:latest

# Install dependencies
RUN apk add --no-cache bash curl jq

# Download owlctl
RUN curl -fsSL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz | tar xz -C /usr/local/bin owlctl \
    && chmod +x /usr/local/bin/owlctl

# Create config directory
RUN mkdir -p /owlctl-config

# Initialize owlctl non-interactively
RUN owlctl init --output-dir /owlctl-config/ --insecure

# Set environment variable
ENV OWLCTL_SETTINGS_PATH=/owlctl-config/

# Default profile
RUN owlctl profile --set vbr

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
```

### Docker Entrypoint

```bash
#!/bin/bash
# entrypoint.sh
set -e

# Login if credentials provided
if [ -n "$OWLCTL_USERNAME" ] && [ -n "$OWLCTL_PASSWORD" ] && [ -n "$OWLCTL_URL" ]; then
    echo "Logging in to VBR..."
    owlctl login
fi

# Execute command
exec "$@"
```

### Docker Compose

```yaml
version: '3.8'

services:
  owlctl:
    build: .
    environment:
      - OWLCTL_USERNAME=${VBR_USERNAME}
      - OWLCTL_PASSWORD=${VBR_PASSWORD}
      - OWLCTL_URL=${VBR_URL}
      - OWLCTL_SETTINGS_PATH=/owlctl-config/
    volumes:
      - ./specs:/specs
    command: owlctl job diff --all --security-only
```

## Script Examples

### Bash Setup Script

```bash
#!/bin/bash
# setup-owlctl.sh
set -e

VCLI_DIR="${HOME}/.owlctl"
VCLI_BIN="${HOME}/.local/bin/owlctl"

# Download owlctl if not exists
if [ ! -f "$VCLI_BIN" ]; then
    echo "Downloading owlctl..."
    mkdir -p "$(dirname "$VCLI_BIN")"
    curl -fsSL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64.tar.gz | tar xz -C "$(dirname "$VCLI_BIN")" owlctl
    chmod +x "$VCLI_BIN"
fi

# Initialize owlctl
echo "Initializing owlctl..."
"$VCLI_BIN" init --output-dir "$VCLI_DIR/" --insecure

# Set profile
echo "Setting profile to VBR..."
OWLCTL_SETTINGS_PATH="$VCLI_DIR/" "$VCLI_BIN" profile --set vbr

echo "Setup complete. Config files at: $VCLI_DIR"
echo "Set environment variables:"
echo "  export OWLCTL_USERNAME='your-username'"
echo "  export OWLCTL_PASSWORD='your-password'"
echo "  export OWLCTL_URL='vbr.example.com'"
echo "  export OWLCTL_SETTINGS_PATH='$VCLI_DIR/'"
```

### PowerShell Setup Script

```powershell
# setup-owlctl.ps1
$ErrorActionPreference = "Stop"

$VcliDir = "$HOME\.owlctl"
$VcliBin = "$HOME\.owlctl\owlctl.exe"

# Download owlctl if not exists
if (-not (Test-Path $VcliBin)) {
    Write-Host "Downloading owlctl..."
    $Zip = Join-Path $env:TEMP "owlctl-windows-amd64.zip"
    Invoke-WebRequest -Uri "https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-windows-amd64.zip" -OutFile $Zip
    New-Item -ItemType Directory -Force -Path $VcliDir | Out-Null
    Expand-Archive -Path $Zip -DestinationPath $VcliDir -Force
    Remove-Item $Zip
}

# Initialize owlctl
Write-Host "Initializing owlctl..."
& $VcliBin init --output-dir $VcliDir --insecure

# Set profile
Write-Host "Setting profile to VBR..."
$env:OWLCTL_SETTINGS_PATH = $VcliDir
& $VcliBin profile --set vbr

Write-Host "Setup complete. Config files at: $VcliDir"
Write-Host "Set environment variables:"
Write-Host "  `$env:OWLCTL_USERNAME = 'your-username'"
Write-Host "  `$env:OWLCTL_PASSWORD = 'your-password'"
Write-Host "  `$env:OWLCTL_URL = 'vbr.example.com'"
Write-Host "  `$env:OWLCTL_SETTINGS_PATH = '$VcliDir'"
```

### Multi-Environment Setup

```bash
#!/bin/bash
# multi-env-setup.sh
set -e

ENVIRONMENTS=("prod" "dev" "staging")

for ENV in "${ENVIRONMENTS[@]}"; do
    echo "Setting up $ENV environment..."

    # Create directory
    mkdir -p ~/.owlctl/$ENV

    # Initialize
    ./owlctl init --output-dir ~/.owlctl/$ENV/ --insecure

    # Set profile
    OWLCTL_SETTINGS_PATH=~/.owlctl/$ENV/ ./owlctl profile --set vbr

    echo "$ENV setup complete"
done

echo "All environments configured"
echo "Usage: OWLCTL_SETTINGS_PATH=~/.owlctl/prod/ ./owlctl login"
```

## Legacy Interactive Mode

For backward compatibility during migration period:

```bash
# Use legacy interactive mode (deprecated)
./owlctl init --interactive
```

**Output:**
```
⚠️  WARNING: Interactive mode is deprecated and will be removed in v0.12.0
   Use 'owlctl init --interactive' to explicitly enable interactive mode
   Or use non-interactive mode: 'owlctl init --insecure --creds-file'

Allow insecure TLS? (y/N): n
Use Creds file mode? (y/N): n
Initialized, ensure all environment variables are set.
```

**Note:** Non-interactive mode is the recommended approach. Update scripts to use non-interactive mode.

## Command Reference

```bash
# Main command
owlctl init [flags]
owlctl init [command]

# Subcommands
owlctl init settings [flags]     # Initialize only settings.json
owlctl init profiles [flags]     # Initialize only profiles.json

# Flags
--insecure                     # Skip TLS verification
--creds-file                   # Enable credentials file mode
--output-dir string            # Directory for config files
--interactive                  # Legacy interactive mode (deprecated)

# Global flags
-h, --help                     # Help for init command
```

## See Also

- [Migration Guide](../docs/migration-v0.10-to-v0.11.md) - Migrating from v0.10.x
- [Getting Started](../docs/getting-started.md) - Complete setup guide
- [Authentication Guide](../docs/authentication.md) - Authentication workflows
- [Azure DevOps Integration](../docs/azure-devops-integration.md) - CI/CD pipeline templates
