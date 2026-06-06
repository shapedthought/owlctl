# Imperative Mode Guide

Imperative mode provides direct API access to all Veeam products. Use it for quick operations, one-off tasks, and API exploration.

## Table of Contents

- [Overview](#overview)
- [GET Command](#get-command)
- [POST Command](#post-command)
- [PUT Command](#put-command)
- [Utils Commands](#utils-commands)
- [Output Formats](#output-formats)
- [Using with jq](#using-with-jq)
- [Using with Nushell](#using-with-nushell)
- [Editing Values with sd](#editing-values-with-sd)
- [Common Endpoints](#common-endpoints)
- [Best Practices](#best-practices)

## Overview

Imperative mode executes direct API operations against Veeam products. Each command maps to an HTTP method:

| Command | HTTP Method | Purpose |
|---------|-------------|---------|
| `owlctl get` | GET | Retrieve data |
| `owlctl post` | POST | Create resources, trigger operations |
| `owlctl put` | PUT | Update resources |

**When to use imperative mode:**
- Quick API queries
- Triggering one-off operations (start backup, stop job)
- Exploring API capabilities
- Working with products without declarative support (VB365, VONE, cloud)

**When to use declarative mode:**
- Infrastructure-as-code workflows
- Multi-environment deployments
- Drift detection and monitoring
- GitOps automation

See [Declarative Mode Guide](declarative-mode.md) for declarative workflows.

## GET Command

Retrieve data from the API.

### Basic Syntax

```bash
owlctl get <endpoint>
```

The endpoint is the path after the API version. owlctl adds the version prefix automatically based on your profile.

### Examples

```bash
# VBR - Get all jobs
owlctl get jobs

# VBR - Get specific job
owlctl get jobs/57b3baab-6237-41bf-add7-db63d41d984c

# VBR - Get all repositories
owlctl get backupInfrastructure/repositories

# VBR - Get managed servers
owlctl get backupInfrastructure/managedServers

# Enterprise Manager - Get jobs (with query parameter)
owlctl get jobs?format=entity

# VB365 - Get organizations
owlctl get organizations

# VONE - Get alarms
owlctl get alarms
```

### Understanding Endpoints

**Full VBR API endpoint:**
```
https://vbr.example.com:9419/api/v1/backupInfrastructure/managedServers
```

**owlctl command:**
```bash
owlctl get backupInfrastructure/managedServers
```

owlctl automatically adds:
- Protocol (`https://`)
- Hostname (from `OWLCTL_URL` or profile)
- Port (from profile)
- API version prefix (`/api/v1/`)

You only specify the resource path.

### Output

Default output is JSON:

```bash
owlctl get jobs
```

```json
{
  "data": [
    {
      "id": "c07c7ea3-0471-43a6-af57-c03c0d82354a",
      "name": "Backup Job 1",
      "type": "Backup",
      "isDisabled": false,
      ...
    }
  ]
}
```

### Output Formats

```bash
# JSON (default)
owlctl get jobs

# YAML
owlctl get jobs --yaml
```

## POST Command

Create resources or trigger operations.

### Basic Syntax

```bash
# Without payload
owlctl post <endpoint>

# With payload
owlctl post <endpoint> -f <file.json>
```

### Examples - Operations (No Payload)

```bash
# Start a backup job
owlctl post jobs/57b3baab-6237-41bf-add7-db63d41d984c/start

# Stop a backup job
owlctl post jobs/57b3baab-6237-41bf-add7-db63d41d984c/stop

# Retry a failed job
owlctl post jobs/57b3baab-6237-41bf-add7-db63d41d984c/retry

# Enable a job
owlctl post jobs/57b3baab-6237-41bf-add7-db63d41d984c/enable

# Disable a job
owlctl post jobs/57b3baab-6237-41bf-add7-db63d41d984c/disable
```

### Examples - Creating Resources (With Payload)

```bash
# Create a backup job
owlctl post jobs -f new-job.json

# Create a repository
owlctl post backupInfrastructure/repositories -f repository.json

# Create credentials
owlctl post backupInfrastructure/credentials -f creds.json
```

**Example payload (new-job.json):**
```json
{
  "name": "New Backup Job",
  "type": "Backup",
  "repository": {
    "id": "a1b2c3d4-..."
  },
  "schedule": {
    "enabled": true
  }
}
```

## PUT Command

Update existing resources.

### Basic Syntax

```bash
owlctl put <endpoint> -f <file.json>
```

PUT requires a payload file.

### Examples

```bash
# Update a backup job
owlctl put jobs/57b3baab-6237-41bf-add7-db63d41d984c -f updated-job.json

# Update a repository
owlctl put backupInfrastructure/repositories/<id> -f updated-repo.json

# Update credentials
owlctl put backupInfrastructure/credentials/<id> -f updated-creds.json
```

### GET vs PUT Object Differences (VBR Jobs)

**Important:** VBR GET and POST job objects have different structures. PUT uses the same structure as GET.

**Workflow:**
1. GET the job to see current configuration
2. Modify the JSON (GET structure)
3. PUT the modified JSON back

```bash
# 1. Get current job
owlctl get jobs/57b3baab-6237-41bf-add7-db63d41d984c > job.json

# 2. Edit job.json (change retention, schedule, etc.)

# 3. Put updated job back
owlctl put jobs/57b3baab-6237-41bf-add7-db63d41d984c -f job.json
```

**For creating new jobs (POST)**, you need to convert GET structure to POST structure. See [Utils - Job JSON Converter](#vbr-job-json-converter).

## Utils Commands

Utility commands for common tasks.

### VBR Job JSON Converter

Converts VBR GET job JSON to POST job JSON format.

**Why needed:** VBR API uses different JSON structures for GET (retrieve) vs POST (create) operations.

**Usage:**

```bash
# 1. Get a job
owlctl get jobs/57b3baab-6237-41bf-add7-db63d41d984c > job-get.json

# 2. Run utils converter
owlctl utils

# 3. Select "VBR Job JSON Converter"
# 4. Enter input file: job-get.json
# 5. Enter output file: job-post.json

# 6. Modify job-post.json as needed

# 7. Create new job
owlctl post jobs -f job-post.json
```

**Note:** PUT operations use GET structure, so this converter is only needed for POST (creating new jobs).

### Check Version

Check if you're running the latest owlctl release.

```bash
owlctl utils
# Select "Check Version"
```

Compares your version against the latest GitHub release.

## Output Formats

owlctl supports JSON and YAML output formats.

### JSON (Default)

```bash
owlctl get jobs
```

```json
{
  "data": [
    {
      "id": "...",
      "name": "Backup Job 1"
    }
  ]
}
```

### YAML

```bash
owlctl get jobs --yaml
```

```yaml
data:
  - id: "..."
    name: Backup Job 1
```

### Saving Output

```bash
# Save to file
owlctl get jobs > jobs.json
owlctl get jobs --yaml > jobs.yaml

# Pipe to tools
owlctl get jobs | jq '.data[0]'
owlctl get jobs --yaml | yq '.data[0]'
```

## Using with jq

[jq](https://stedolan.github.io/jq/) is a lightweight JSON processor perfect for parsing owlctl output.

### Installation

```bash
# macOS
brew install jq

# Linux (Debian/Ubuntu)
sudo apt-get install jq

# Windows (Chocolatey)
choco install jq
```

### Basic Usage

```bash
# Get all job names
owlctl get jobs | jq '.data[].name'

# Get jobs that are disabled
owlctl get jobs | jq '.data[] | select(.isDisabled == true)'

# Get first job's ID
owlctl get jobs | jq '.data[0].id'

# Filter by job type
owlctl get jobs | jq '.data[] | select(.type == "Backup")'

# Get specific fields
owlctl get jobs | jq '.data[] | {name: .name, type: .type, disabled: .isDisabled}'
```

### Useful jq Patterns

```bash
# See keys in an object (useful for exploring)
owlctl get jobs | jq 'keys'
owlctl get jobs | jq '.data[0] | keys'

# Count jobs
owlctl get jobs | jq '.data | length'

# Pretty print
owlctl get jobs | jq '.'

# Compact output
owlctl get jobs | jq -c '.data[]'

# Raw string output (no quotes)
owlctl get jobs | jq -r '.data[].name'

# Filter and format
owlctl get jobs | jq '.data[] | "\(.name) (\(.type))"'
```

### Complex Example (Enterprise Manager)

```bash
# Get VM names and IDs from all jobs
owlctl get jobs?format=entity | jq '.Jobs[].JobInfo.BackupJobInfo.Includes.ObjectInJobs[] | .Name, .ObjectInJobId'
```

## Using with Nushell

[Nushell](https://www.nushell.sh/) is a modern shell designed for structured data. It provides an excellent experience for working with API responses.

### Installation

```bash
# macOS
brew install nushell

# Linux
cargo install nu

# Windows
winget install nushell
# or
choco install nushell
```

Website: https://www.nushell.sh/book/installation.html

### Basic Usage

```bash
# Start Nushell
nu

# Set credentials
let-env OWLCTL_USERNAME = "administrator"
let-env OWLCTL_PASSWORD = "password"
let-env OWLCTL_URL = "vbr.example.com"

# Login
owlctl login

# Get jobs and parse JSON automatically
owlctl get jobs | from json

# Access the data array
owlctl get jobs | from json | get data

# Filter disabled jobs
owlctl get jobs | from json | get data | where isDisabled == true

# Select specific columns
owlctl get jobs | from json | get data | select name type isDisabled

# Convert to YAML
owlctl get jobs | from json | get data | to yaml

# Save as YAML
owlctl get jobs | from json | get data | to yaml | save jobs.yaml --raw
```

### Nushell Advantages

- Automatic table formatting for structured data
- Powerful filtering with `where`
- Easy column selection with `select`
- Format conversion built-in (`to yaml`, `to json`, `to csv`)
- Type-aware operations

### Nushell Modules

Create reusable command modules:

**v.nu:**
```nu
# Simplified owlctl commands
export def vget [url: string] {
    owlctl get $url | from json | get data
}

export def vlogin [] {
    owlctl login
}

# With environment variables included
export-env {
    let-env OWLCTL_USERNAME = "administrator"
    let-env OWLCTL_PASSWORD = "password"
    let-env OWLCTL_URL = "vbr.example.com"
}
```

**Usage:**
```bash
# Import module
use v.nu

# Use simplified commands
v vlogin
v vget jobs
v vget jobs | where isDisabled == false
```

### Nushell vs jq

| Feature | jq | Nushell |
|---------|-----|---------|
| JSON parsing | Excellent | Excellent |
| Tables | Manual formatting | Automatic |
| Filtering | `select()` expressions | `where` clause (more readable) |
| Learning curve | Steeper | Gentler |
| Speed | Very fast | Fast |
| Use case | Quick scripts | Interactive exploration |

## Editing Values with sd

[sd](https://crates.io/crates/sd) (written in Rust 🦀) is a find-and-replace tool that works like `sed` but with a simpler regex syntax. It's handy for changing a value in a saved response without opening an editor — useful when scripting imperative `PUT`/`POST` workflows.

```bash
# Check the current value first
cat job.json | jq '.name'

# Replace a value in a file
sd '"name": "Backup Job 2"' '"name": "Backup Job 12"' ./job.json

# Pipe owlctl output straight through sd
owlctl get jobs/<id> | sd '"name": "Old Name"' '"name": "New Name"' > job.json
```

**Installation:**
```bash
# macOS (Homebrew)
brew install sd

# Rust package manager
cargo install sd

# Windows (Chocolatey)
choco install sd-cli
```

## Common Endpoints

### VBR (Veeam Backup & Replication)

```bash
# Jobs
owlctl get jobs
owlctl get jobs/<id>
owlctl post jobs/<id>/start
owlctl post jobs/<id>/stop

# Infrastructure
owlctl get backupInfrastructure/repositories
owlctl get backupInfrastructure/scaleOutRepositories
owlctl get backupInfrastructure/managedServers
owlctl get backupInfrastructure/credentials

# Sessions
owlctl get sessions
owlctl get sessions/<id>

# Restore points
owlctl get restorePoints

# Configuration
owlctl get configuration/serverSettings
```

### Enterprise Manager

```bash
# Jobs
owlctl get jobs?format=entity
owlctl get jobs/<id>

# Repositories
owlctl get repositories

# Backup servers
owlctl get backupServers
```

### VB365

```bash
# Organizations
owlctl get organizations
owlctl get organizations/<id>

# Jobs
owlctl get jobs
owlctl post jobs/<id>/start

# Backup repositories
owlctl get backupRepositories

# Proxies
owlctl get proxies
```

### VONE

```bash
# Alarms
owlctl get alarms

# Reports
owlctl get reports

# Infrastructure
owlctl get infrastructure
```

## Best Practices

### 1. Save API Responses

```bash
# Save for reference
owlctl get jobs > jobs-backup-$(date +%Y%m%d).json

# Save before making changes
owlctl get jobs/57b3baab > job-before-change.json
owlctl put jobs/57b3baab -f updated-job.json
```

### 2. Use Descriptive File Names

```bash
# Bad
owlctl post jobs -f data.json

# Good
owlctl post jobs -f new-sql-backup-job.json
```

### 3. Preview Before Modifying

```bash
# Get current config
owlctl get jobs/57b3baab | jq '.'

# Make changes in editor, then apply
owlctl put jobs/57b3baab -f modified-job.json
```

### 4. Script Common Operations

```bash
#!/bin/bash
# start-all-jobs.sh

for job_id in $(owlctl get jobs | jq -r '.data[].id'); do
    echo "Starting job: $job_id"
    owlctl post jobs/$job_id/start
done
```

### 5. Use jq for Filtering

```bash
# Get only enabled jobs
owlctl get jobs | jq '.data[] | select(.isDisabled == false)'

# Get job IDs only
owlctl get jobs | jq -r '.data[].id'
```

### 6. Combine with Other Tools

```bash
# Count jobs by type
owlctl get jobs | jq -r '.data[].type' | sort | uniq -c

# Get job names in CSV
owlctl get jobs | jq -r '.data[] | [.name, .type] | @csv' > jobs.csv

# Monitor job status in loop
watch 'owlctl get jobs | jq ".data[] | {name: .name, status: .status}"'
```

## Troubleshooting

### Empty Response

**Problem:** Command succeeds but returns empty `{}`

**Causes:**
- Resource doesn't exist
- Wrong endpoint
- Permissions issue

**Solution:**
```bash
# Verify endpoint
owlctl get jobs  # Should show list

# Check profile
owlctl profile --get

# Verify authentication
owlctl login
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
```

### "No such file or directory" with -f Flag

**Problem:** `owlctl post jobs -f data.json` fails

**Solution:**
```bash
# Use absolute path
owlctl post jobs -f /full/path/to/data.json

# Or relative from current directory
owlctl post jobs -f ./data.json
```

## See Also

- [Command Reference](command-reference.md) - Quick command lookup
- [Authentication Guide](authentication.md) - Setup and credentials
- [Declarative Mode Guide](declarative-mode.md) - Infrastructure-as-code workflows
- [Getting Started](getting-started.md) - Complete setup guide
