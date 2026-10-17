# Lab verification pipeline

An Azure DevOps pipeline that proves a released owlctl binary works end to end against a **lab** VBR: login, `get`, export, drift detection with `job apply --dry-run`, and optionally a guarded apply cycle and a real backup run. It was run green against VBR 13.1 with owlctl v1.3.0.

**Point it at a lab VBR only.** Never use production credentials or a production server.

## Prerequisites

- A self-hosted **Linux** agent that can reach the lab VBR, with `jq` installed.
- A variable group containing:
  - `OWLCTL_URL`: hostname or IP only, no scheme or port (`vbr-lab.example.com`, not `https://...`)
  - `OWLCTL_USERNAME`
  - `OWLCTL_PASSWORD`, marked as a **secret**
- A disposable lab backup job (parameter `testJob`).
- For the backup stage, a repository in the lab (parameter `backupRepo`) that the test job writes to.

## Files

| File | Purpose |
|------|---------|
| `lab-verification.yml` | The pipeline |
| `templates/owlctl-login.yml` | Initialises settings in a temp dir, logs in, stores the token as a secret variable |
| `../templates/install-owlctl.yml` | Shared install template (download, checksum, install). Reused, not copied |

## Stages

| Stage | Runs | What it does |
|-------|------|--------------|
| `ReadOnly` | always | `get` checks (jobs, repositories, SOBRs, credentials, encryption), then `job`/`repo`/`repo sobr-export`/`encryption` `export --all` and top-level `export --all`. Fails if nothing was exported or an export contains a `password:` value. Writes to a fresh `$(Build.ArtifactStagingDirectory)/exports` and publishes it as the `exports` build artifact, only when the step (including the password scan) succeeded |
| `Drift` | always | Exports the test job, asserts `job apply --dry-run` reports `No changes detected`, then changes `description` in a copy and asserts the dry-run shows exactly one change, on `description` |
| `ApplyCycle` | `runApplyCycle: true` | Creates `<testJob>-apply` from the test job if absent, applies a changed description to it, verifies, reverts, and asserts a clean dry-run and that `<testJob>` itself did not change |
| `Backup` | `runBackup: true` | Starts `<testJob>` once, waits for it to finish (while status is `Running` or lastResult is `None`), requires `Success` or `Warning`, and requires the run to register (status `Running` or a changed `lastRun`) and a new restore point on `backupRepo`. Re-logs in about every 10 minutes while polling, since owlctl does not refresh the token |

## Parameters

| Parameter | Default | Meaning |
|-----------|---------|---------|
| `testJob` | `owlctl-test-job` | Name of the disposable lab job |
| `backupRepo` | `lab-hardened-repo` | Repository the test job backs up to |
| `owlctlVersion` | `v1.4.0` | Release tag to install |
| `runApplyCycle` | `false` | Run the `ApplyCycle` stage |
| `runBackup` | `false` | Run the `Backup` stage |

## Pointing it at your own lab

1. Replace `<your-lab-agent-pool>` and `<your-lab-variable-group>` in `lab-verification.yml`.
2. Create the variable group with the three variables above.
3. Create a disposable backup job in the lab and set `testJob` to its name (or accept the default name).
4. Create a pipeline from the file in Azure DevOps and run it manually. Enable `runApplyCycle` and `runBackup` once the read-only stages are green.

Authentication uses a token: the login template sets `OWLCTL_TOKEN` from `owlctl login --output-token` as a secret variable, which avoids a keychain lookup that can hang on a headless agent. Settings and state are kept together in `$(Agent.TempDirectory)/owlctl/` via `OWLCTL_SETTINGS_PATH`.

### Optional: run on pull requests

The pipeline ships with `trigger: none` and `pr: none`, and has no `schedules:`. To run it on PRs, replace `pr: none` with, for example:

```yaml
pr:
  branches:
    include: [master]
  paths:
    include: [cmd/**, state/**, config/**, examples/pipelines/**]
```

Keep `runApplyCycle` and `runBackup` off for PR runs.

## Guardrails

- **Lab only.** Use an account scoped to the lab with the minimum role needed.
- **Disposable test job.** Nothing in the pipeline deletes anything on VBR. (The ReadOnly step only clears its own `exports` folder in the agent working area.)
- Description edits only support a single-line `description:`. The pipeline fails early if the export uses a block scalar (`|` or `>`).
- The apply cycle writes only to `<testJob>-apply`, and checks the job name before applying. VBR ignores `isDisabled` on job create and refuses `/disable` for unscheduled jobs (owlctl handles this since #186), so the clone **may stay enabled**. It is created without a schedule, so it will not run by itself.
- `runBackup` starts **one backup per manual run**. On immutable (hardened) repositories those backups are retained until the immutability period ends.
- No schedules. Don't add a scheduled or auto-remediation pipeline until this one has been green on several consecutive runs.
- Secrets come only from the variable group; the password and token are secret variables and are never echoed.
