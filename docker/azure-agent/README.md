# Azure DevOps Agent in Docker

This directory contains a Docker setup for running an Azure DevOps self-hosted agent to test owlctl pipelines locally.

## Why Use This?

- Test Azure DevOps pipelines locally without committing to the remote repository
- Validate pipeline YAML changes before pushing
- Develop and debug pipeline scripts in an isolated environment
- Avoid installing the Azure DevOps agent directly on your machine

## Prerequisites

1. **Docker and Docker Compose** installed on your machine
2. **Azure DevOps account** with permission to create agent pools
3. **Personal Access Token (PAT)** with `Agent Pools (Read & Manage)` scope
4. **VBR server** accessible from your Docker host (for testing owlctl commands)

## Quick Start

### 1. Create a Personal Access Token (PAT)

1. Go to your Azure DevOps organization: `https://dev.azure.com/yourorg`
2. Click your profile icon → **Personal access tokens**
3. Click **+ New Token**
4. Name: `owlctl-docker-agent`
5. Scopes: **Agent Pools (Read & Manage)**
6. Click **Create** and **copy the token** (you won't see it again)

### 2. Configure Environment Variables

```bash
# Copy the example file
cp .env.example .env

# Edit .env with your values
nano .env
```

Fill in:
```bash
AZP_URL=https://dev.azure.com/yourorganization
AZP_TOKEN=your_pat_token_here
AZP_POOL=Default
OWLCTL_USERNAME=your_vbr_username
OWLCTL_PASSWORD=your_vbr_password
OWLCTL_URL=https://vbr.example.com:9419
```

### 3. Build and Start the Agent

```bash
# Build the Docker image
docker compose build

# Start the agent
docker compose up -d

# View logs
docker compose logs -f
```

You should see output like:
```
Configuring Azure DevOps agent...
Starting Azure DevOps agent...
Listening for Jobs
```

### 4. Verify Agent is Online

1. Go to your Azure DevOps organization
2. Navigate to **Project Settings → Agent pools**
3. Click your pool name (e.g., "Default")
4. You should see your agent listed with a green status

### 5. Test with a Pipeline

Create a simple test pipeline in your Azure DevOps project:

```yaml
# azure-pipelines-test.yml
trigger: none

pool:
  name: 'Default'  # Must match AZP_POOL in .env
  # No architecture demand needed - agent works on ARM64 and AMD64

steps:
  - script: |
      echo "Testing owlctl agent"
      go version
      git --version
    displayName: 'Check environment'

  - script: |
      cd $(Agent.WorkFolder)/_owlctl
      go build -o owlctl
      ./owlctl --version
    displayName: 'Build owlctl'

  - script: |
      cd $(Agent.WorkFolder)/_owlctl
      ./owlctl init
      ./owlctl profile --list
    displayName: 'Test owlctl commands'
    env:
      OWLCTL_USERNAME: $(OWLCTL_USERNAME)
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
      OWLCTL_URL: $(OWLCTL_URL)
```

## Usage

### Start Agent
```bash
docker compose up -d
```

### Stop Agent
```bash
docker compose down
```

### View Logs
```bash
docker compose logs -f azure-agent
```

### Rebuild After Changes
```bash
docker compose down
docker compose build --no-cache
docker compose up -d
```

### Access Agent Container
```bash
docker compose exec azure-agent bash
```

### Clean Up Everything
```bash
docker compose down -v  # Removes volumes
```

## Testing owlctl Pipelines

Pipelines clone the owlctl repository from GitHub and build from source, just like they would in production. This approach:

1. **Tests the real workflow** - Same as production pipelines
2. **No mount dependencies** - Works with any repository
3. **Verifies clean builds** - Ensures no local-only dependencies

### Alternative: Use Local Source for Development

If you're developing owlctl and want faster iteration, you can mount your local source:

```yaml
# docker-compose.yml
volumes:
  - ../../:/home/agent/local-owlctl:ro
```

Then in your pipeline, replace the `git clone` step with:
```yaml
- script: |
    cp -r /home/agent/local-owlctl $(Build.SourcesDirectory)/owlctl
    cd $(Build.SourcesDirectory)/owlctl
    go build -o owlctl
  displayName: 'Build from local source'
```

This skips the clone and builds your local changes immediately.

### Example: Test Drift Detection Pipeline

```yaml
# test-drift-detection.yml
trigger: none

pool:
  name: 'Default'

variables:
  - group: 'veeam-credentials'  # Optional: Use variable group instead of .env

steps:
  - checkout: none

  - script: |
      # Clone and build owlctl
      git clone --branch master --depth 1 https://github.com/shapedthought/owlctl.git $(Build.SourcesDirectory)/owlctl
      cd $(Build.SourcesDirectory)/owlctl
      go build -o owlctl

      # Initialize owlctl (creates config files in current directory)
      ./owlctl init
      ./owlctl profile --set vbr
      ./owlctl login
    displayName: 'Setup owlctl'
    env:
      OWLCTL_USERNAME: $(OWLCTL_USERNAME)
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
      OWLCTL_URL: $(OWLCTL_URL)

  - script: |
      cd $(Build.SourcesDirectory)/owlctl
      ./owlctl job diff --all --security-only
      EXIT_CODE=$?

      if [ $EXIT_CODE -eq 4 ]; then
        echo "##vso[task.logissue type=error]CRITICAL drift detected"
        exit 1
      elif [ $EXIT_CODE -eq 3 ]; then
        echo "##vso[task.logissue type=warning]Warning-level drift detected"
      fi
    displayName: 'Run drift detection'
    env:
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
```

**Important:** Combine `owlctl init`, `profile --set`, and `login` in a single script block. owlctl creates configuration files in the current directory, and separate steps won't see those files (each step runs in a new shell session).

## Using with Pipeline Templates

You can test the ready-made pipeline templates from `examples/pipelines/`:

1. Copy a template to your Azure DevOps repository
2. Update the `pool` section to use your agent pool:
   ```yaml
   pool:
     name: 'Default'  # Your local agent pool
   ```
3. Queue the pipeline manually or via trigger

The templates will run on your local Docker agent, allowing you to debug and iterate quickly.

## Pipeline Best Practices

### owlctl Configuration Files

owlctl creates configuration files (`settings.json`, `profiles.json`) in the current directory by default. In pipelines:

**✅ Correct - Combined steps:**
```yaml
- script: |
    cd $(Build.SourcesDirectory)/owlctl
    ./owlctl init
    ./owlctl profile --set vbr
    ./owlctl login
  env:
    OWLCTL_USERNAME: $(OWLCTL_USERNAME)
    OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
    OWLCTL_URL: $(OWLCTL_URL)
```

**❌ Incorrect - Separate steps:**
```yaml
- script: ./owlctl init
- script: ./owlctl profile --set vbr  # ❌ Can't find profiles.json
```

Each Azure DevOps pipeline step runs in a new shell session, so configuration files created in one step won't be visible to the next. **Always combine owlctl initialization and login in a single script block.**

### Environment Variables

The owlctl credentials are set as environment variables in docker-compose.yml, but they don't automatically propagate to pipeline script steps. Set them explicitly using the `env:` section in each step that needs them:

```yaml
- script: |
    ./owlctl login
  env:
    OWLCTL_USERNAME: $(OWLCTL_USERNAME)
    OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
    OWLCTL_URL: $(OWLCTL_URL)
```

The `OWLCTL_PASSWORD` should be set in subsequent steps that call owlctl commands after login:

```yaml
- script: |
    ./owlctl job diff --all
  env:
    OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)  # Required for authenticated API calls
```

## Troubleshooting

### Agent Won't Start

**Check logs:**
```bash
docker compose logs azure-agent
```

**Common issues:**
- Invalid PAT token → Regenerate token with correct scopes
- Wrong organization URL → Verify AZP_URL format
- Agent pool doesn't exist → Create pool in Azure DevOps first

### Agent Offline in Azure DevOps

**Restart the container:**
```bash
docker compose restart azure-agent
```

**Check agent configuration:**
```bash
docker compose exec azure-agent cat .agent
```

### owlctl Commands Fail

**Check VBR connectivity:**
```bash
docker compose exec azure-agent bash
curl -k $OWLCTL_URL
```

**Check credentials:**
```bash
docker compose exec azure-agent env | grep VCLI
```

### Pipeline Can't Clone owlctl

**Check network connectivity:**
```bash
docker compose exec azure-agent bash -c "git clone --depth 1 https://github.com/shapedthought/owlctl.git /tmp/test-clone"
```

**Use a specific branch or fork:**
```yaml
variables:
  - name: owlctlRepo
    value: 'https://github.com/yourfork/owlctl.git'
  - name: owlctlBranch
    value: 'your-branch'
```

### Go Build Failures

**Check Go version:**
```bash
docker compose exec azure-agent go version
```

**Clean Go cache:**
```bash
docker compose exec azure-agent go clean -cache
```

### Authentication Failures

```
Error: failed to login
```

- Verify `OWLCTL_URL` includes port (e.g., `https://vbr.example.com:9419`)
- Check username format (may need `DOMAIN\user`)
- Ensure VBR REST API is enabled
- Verify credentials are set in pipeline environment variables

### Profile Not Found

```
Error: open profiles.json: no such file or directory
```

- Ensure `owlctl init` and `owlctl profile --set` are in the same script block
- Don't split owlctl initialization across multiple pipeline steps
- Check that you're in the correct directory (`cd $(Build.SourcesDirectory)/owlctl`)

## Architecture Considerations

The Docker agent automatically detects and uses the appropriate architecture:
- **Apple Silicon (M1/M2/M3)**: Runs natively on ARM64
- **Intel Mac**: Runs on AMD64/X64
- **Linux**: Matches host architecture

### Pipeline Architecture Demands

By default, the example pipelines don't specify architecture, allowing them to run on any agent. If you need to enforce a specific architecture:

```yaml
pool:
  name: 'Default'
  demands:
    - Agent.OSArchitecture -equals ARM64  # For Apple Silicon
    # OR
    - Agent.OSArchitecture -equals X64    # For Intel/AMD
```

**When to use architecture demands:**
- Testing platform-specific behavior
- Ensuring consistency across pipeline runs
- Working with architecture-specific dependencies

**For owlctl testing:** Architecture doesn't matter - Go produces portable binaries and owlctl works identically on ARM64 and X64.

## Advanced Configuration

### Use Custom Agent Pool

1. Create a new agent pool in Azure DevOps:
   - Project Settings → Agent pools → Add pool
   - Name: `owlctl-testing`
   - Grant access to your project

2. Update `.env`:
   ```bash
   AZP_POOL=owlctl-testing
   ```

3. Restart agent:
   ```bash
   docker compose restart
   ```

### Mount Local owlctl Config

If you want to use your existing owlctl configuration:

```yaml
# docker-compose.yml
volumes:
  - ~/.owlctl:/home/agent/.owlctl:ro
```

### Run Multiple Agents

```bash
# Scale to 3 agents
docker compose up -d --scale azure-agent=3
```

Each agent will register with a unique name (hostname-based).

### Use Pre-Built owlctl Binary

If you want to test with a specific owlctl release instead of building from source:

```dockerfile
# Add to Dockerfile after Go installation
ARG TARGETARCH
RUN if [ "$TARGETARCH" = "arm64" ]; then \
        OWLCTL_ARCH="linux-arm64"; \
    else \
        OWLCTL_ARCH="linux-amd64"; \
    fi && \
    curl -sL https://github.com/shapedthought/owlctl/releases/download/v0.12.1-beta1/owlctl-${OWLCTL_ARCH} -o /usr/local/bin/owlctl && \
    chmod +x /usr/local/bin/owlctl
```

Then in your pipeline:
```yaml
steps:
  - script: owlctl --version
    displayName: 'Test owlctl'
```

## Network Considerations

### VBR on Same Docker Network

If you're running VBR in Docker, add the agent to the same network:

```yaml
# docker-compose.yml
networks:
  owlctl-test:
    external: true
    name: vbr_network
```

### VBR on Host Machine

The agent can reach host services via `host.docker.internal`:

```bash
# .env
OWLCTL_URL=https://host.docker.internal:9419
```

### VBR on Remote Server

Ensure Docker host can reach the VBR server. Test connectivity:

```bash
docker compose exec azure-agent curl -k https://vbr.example.com:9419
```

## Security Notes

- **Never commit `.env` to Git** (it's in `.gitignore` by default)
- Store PAT tokens securely (use Azure Key Vault in production)
- Use read-only VBR credentials for testing when possible
- The agent runs as a non-root user (`agent`) for security

## Clean Up

### Remove Agent from Azure DevOps

The agent automatically deregisters when the container stops gracefully:

```bash
docker compose down
```

### Manual Deregistration

If the agent shows as offline in Azure DevOps after `docker compose down`:

1. Go to Project Settings → Agent pools → Your pool
2. Click the agent → **Delete**

Or force removal via container:

```bash
docker compose run --rm azure-agent ./config.sh remove --unattended --auth pat --token "$AZP_TOKEN"
```

## Common Pipeline Patterns

### Full Pipeline with Error Handling

```yaml
steps:
  - checkout: none

  - script: |
      set -e  # Exit on any error

      # Clone and build
      git clone --branch master --depth 1 \
        https://github.com/shapedthought/owlctl.git \
        $(Build.SourcesDirectory)/owlctl
      cd $(Build.SourcesDirectory)/owlctl
      go build -o owlctl

      # Initialize and authenticate
      ./owlctl init
      ./owlctl profile --set vbr
      ./owlctl login

      echo "✓ owlctl ready"
    displayName: 'Setup owlctl'
    env:
      OWLCTL_USERNAME: $(OWLCTL_USERNAME)
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
      OWLCTL_URL: $(OWLCTL_URL)

  - script: |
      cd $(Build.SourcesDirectory)/owlctl

      # Run all drift checks
      CRITICAL=0

      for CMD in \
        "job diff --all --security-only" \
        "repo diff --all --security-only" \
        "repo sobr-diff --all --security-only" \
        "encryption diff --all --security-only" \
        "encryption kms-diff --all --security-only"
      do
        echo "Running: owlctl $CMD"
        ./owlctl $CMD || EXIT_CODE=$?

        if [ $EXIT_CODE -eq 4 ]; then
          CRITICAL=1
          echo "##vso[task.logissue type=error]CRITICAL drift in: $CMD"
        fi
      done

      if [ $CRITICAL -eq 1 ]; then
        echo "##vso[build.addbuildtag]critical-drift"
        exit 1
      fi
    displayName: 'Drift detection scan'
    env:
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
```

### Test Specific Branch or Fork

```yaml
variables:
  - name: owlctlRepo
    value: 'https://github.com/yourfork/owlctl.git'
  - name: owlctlBranch
    value: 'feature-branch'

steps:
  - script: |
      git clone --branch $(owlctlBranch) --depth 1 $(owlctlRepo) $(Build.SourcesDirectory)/owlctl
      cd $(Build.SourcesDirectory)/owlctl
      go build -o owlctl
```

### Use Pre-Built Binary Instead of Building

```yaml
steps:
  - script: |
      # Detect architecture
      ARCH=$(uname -m)
      if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
        OWLCTL_ARCH="linux-arm64"
      else
        OWLCTL_ARCH="linux-amd64"
      fi

      # Download pre-built binary
      curl -sL https://github.com/shapedthought/owlctl/releases/download/v0.12.1-beta1/owlctl-${OWLCTL_ARCH} -o owlctl
      chmod +x owlctl

      # Initialize
      ./owlctl init
      ./owlctl profile --set vbr
      ./owlctl login
    displayName: 'Setup owlctl (pre-built)'
    env:
      OWLCTL_USERNAME: $(OWLCTL_USERNAME)
      OWLCTL_PASSWORD: $(OWLCTL_PASSWORD)
      OWLCTL_URL: $(OWLCTL_URL)
```

## See Also

- [Azure DevOps Integration Guide](../../docs/azure-devops-integration.md)
- [Pipeline Templates](../../examples/pipelines/)
- [test-pipeline.yml](./test-pipeline.yml) - Reference implementation
- [Microsoft Docs: Run a self-hosted agent in Docker](https://learn.microsoft.com/en-us/azure/devops/pipelines/agents/docker)
