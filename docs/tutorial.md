# Tutorial: Manage a Backup Job as Code

This is the core declarative loop in owlctl: **export → edit → apply → detect drift**. By the end you'll have taken one existing VBR backup job, captured it as a YAML file you can commit to Git, changed it through owlctl, and detected when someone changes it behind your back.

It uses a single job and a single VBR server — no overlays, groups, or instances. Those build on this loop and are covered in the [Declarative Mode Guide](declarative-mode.md) once you're comfortable here.

## Before you start

Complete the [Getting Started Guide](getting-started.md) first so that:

- owlctl is installed and initialised (`owlctl init`)
- your credentials are set (`OWLCTL_USERNAME` / `OWLCTL_PASSWORD` / `OWLCTL_URL`)
- you can log in: `owlctl login`
- `owlctl get jobs` returns data

Declarative mode is **VBR only**.

## Step 1 — Find the job and export it

List your jobs to get the ID of the one you want to manage:

```bash
owlctl get jobs | jq '.data[] | {name, id}'
```

Export that job to a YAML file. The job export subcommand takes the job **ID**:

```bash
mkdir -p jobs
owlctl job export 57b3baab-6237-41bf-add7-db63d41d984c -o jobs/db-backup.yaml
```

You now have the job's full configuration (300+ fields) in `jobs/db-backup.yaml`. This file is the *desired state* — commit it to Git:

```bash
git add jobs/db-backup.yaml
git commit -m "Capture DB Backup job as code"
```

## Step 2 — Edit the desired state

Open `jobs/db-backup.yaml` and change something. For example, increase the restore-point retention:

```yaml
# ...
storage:
  retentionPolicy:
    type: Days
    quantity: 30        # was 14
# ...
```

Save the file. Nothing has happened on the VBR server yet — you've only changed your local desired state.

## Step 3 — Preview the change

Always preview before applying. `--dry-run` shows exactly which fields would change, and makes no API calls that modify VBR:

```bash
owlctl job apply jobs/db-backup.yaml --dry-run
```

```
Applying: DB Backup

  storage.retentionPolicy.quantity: 14 → 30

No changes made. Remove --dry-run flag to apply.
```

If the diff isn't what you expected, fix the YAML and preview again.

## Step 4 — Apply

Apply the change for real:

```bash
owlctl job apply jobs/db-backup.yaml
```

Apply does two things on success:

1. **Updates the job in VBR** to match your YAML.
2. **Records the configuration in owlctl's state** (`state.json`) with `origin: applied`.

That second part matters: **applying is what lets `diff` work later.** State is owlctl's record of what the configuration *should* be, and drift detection compares live VBR against it. A job you exported but never applied has no state baseline, so `diff` has nothing to compare against.

> By default owlctl writes state to `~/.owlctl/state.json` — run `owlctl state path` to confirm. To keep it under version control with your specs, set `OWLCTL_SETTINGS_PATH` to your project directory before running owlctl, then commit `state.json` so drift detection behaves the same for everyone on the team. See [State Management](state-management.md) for details.

## Step 5 — Detect drift

Now imagine someone disables the job or shortens its retention directly in the VBR console. Detect it:

```bash
owlctl job diff "DB Backup"
```

owlctl compares the live job against your applied state and reports each difference with a severity (CRITICAL / WARNING / INFO). Check every managed job at once with `--all`, and focus on security-relevant changes with `--security-only`:

```bash
owlctl job diff --all --security-only
```

Drift commands return **exit codes** designed for CI/CD:

| Code | Meaning |
|------|---------|
| `0` | No drift |
| `3` | Drift detected (INFO / WARNING) |
| `4` | Critical drift (security-impacting) |
| `1` | Error |

To bring VBR back in line with your committed desired state, re-apply:

```bash
owlctl job apply jobs/db-backup.yaml
```

## Monitor without managing

If you want drift detection on a job but don't intend to change it through owlctl, snapshot it instead of applying:

```bash
owlctl job snapshot "DB Backup"
```

This records the current live configuration in state with `origin: observed`. `diff` will then report drift, but owlctl won't modify the job. To promote a monitored job to fully managed later, export it and `apply` the spec (Step 1 → Step 4).

## Where to go next

You now have the full loop. Build on it with:

- **[Declarative Mode Guide](declarative-mode.md)** — overlays for per-environment overrides, groups for batch operations, named instances for multiple servers
- **[State Management](state-management.md)** — how `state.json` works, instance scoping, inspecting state (`owlctl state list/show/history`)
- **[Drift Detection](drift-detection.md)** and **[Security Alerting](security-alerting.md)** — severity classification and value-aware security rules
- **[GitOps Workflows](gitops-workflows.md)** — run this loop automatically in CI/CD
