# Writer brief for Claude (owlctl)

You are the **writer**. Ed (shapedthought) owns the repo. **The Manager reviews every PR.** There's one writer and one reviewer: don't review your own work as final, and don't edit a PR another writer is working on.

## Where the work comes from
- The queue is **GitHub issues**. Chat history and memory aren't.
- Work only from the **issue body** and the **spec comments it links to** (for example the spec comment on #156). If the spec is ambiguous or contradicts the code, stop and ask on the issue rather than guessing.
- **Never read or implement from `planning/`.** It's archived and not current (see #171), and it describes designs that have since shipped or been dropped.

## Order of truth
1. Release binary behaviour and `CHANGELOG.md`
2. `--help` output (Cobra command definitions in `cmd/`) and the resource registry (`cmd/export_registry.go`, `cmd/vbr_exports.go`)
3. `docs/`
4. Issues

If two of these disagree, the higher one wins. Note the disagreement in the PR description.

## Invariants (don't break these, and fix any example that breaks them)
- `--url` and `url:` in owlctl.yaml are a **hostname or IP only**, with no scheme or port (`vbr-prod.example.com`, not `https://…`). Requests prepend `https://` themselves.
- `--target` is **deprecated**. Use `--instance` and named instances in all new code and examples.
- **Target invariant, being fixed by #156 (not true yet):** `state.json` lives **next to settings** (`settings.json`/`profiles.json`, in `OWLCTL_SETTINGS_PATH` or the current directory). Today, with `OWLCTL_SETTINGS_PATH` unset, settings resolve to the current directory but state goes to `~/.owlctl/state.json`. Don't assume co-location in unrelated work, and don't make the split worse.
- `owlctl.yaml` is created by **`owlctl instance add`**, not by `owlctl init`.
- `state.json` is **not an audit log**. Its `history` is a convenience trail. Git history of the specs is the audit record.

## Definition of done (every PR)
- [ ] **One issue per PR.** Reference it in the PR body (`Fixes #N` or `Refs #N`).
- [ ] Tests added or updated for the change. `go test ./...` passes.
- [ ] `gofmt -l .` prints nothing, and `go vet ./...` is clean. `go build ./...` succeeds.
- [ ] A `CHANGELOG.md` entry under `## [Unreleased]` (Added / Changed / Deprecated / Fixed), with the issue number.
- [ ] Docs and help text updated where behaviour changed. No new embedded install pins (see #166).
- [ ] The PR description says what changed, what you tested, and anything you couldn't verify.

## Never
- Never commit secrets, tokens, `settings.json`, `profiles.json`, `headers.json`, `state.json` from a real server, or `.env` files.
- There are **no lab or Veeam credentials** here. Don't ask for them, invent them, or try to reach a Veeam server. Code that needs a live VBR is verified later by the lab pipeline (#167), not by you.
- Don't merge PRs, force-push, rebase shared branches, or change workflows, repo settings or secrets unless the issue explicitly asks for it.
- To resolve conflicts with master: `git fetch origin master && git merge origin/master`, resolve, commit, and push normally. Never rebase or force-push. In a CHANGELOG conflict, keep every entry.
