# Changelog

All notable changes to owlctl will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `examples/pipelines/lab-verification/`: Azure DevOps example pipeline that verifies owlctl against a lab VBR (login, get, export, `job apply --dry-run` drift check, optional guarded apply cycle and backup run), with tests that parse the YAML and check for IPs and secrets (#167)
- `owlctl export --all` and `owlctl snapshot --all` now include the configuration backup singleton: export writes `config-backup/configuration-backup.yaml`, and `export --from-state` maps `VBRConfigurationBackup` to `config-backup/` instead of `unknown/`. The export registry gains an `IsSingleton` flag; help text and docs no longer carry the "not included" caveats (#163)
- `--output json` for `job apply`, `repo apply`, `repo sobr-apply`, `encryption kms-apply` and `config-backup apply` with `--dry-run`: prints `{"resource","kind","action","changes":[{"path","old","new","severity"}],"skipped","postApply"}` on stdout, with banners and warnings on stderr. Requires `--dry-run`; not supported with `--group` (#187)

### Fixed
- `job apply --dry-run` printed a literal `Would call: POST jobs/{id}/disable`. It now shows the real job id, or `POST jobs/<new id>/disable (after create)` when the job does not exist yet (#187)
- When VBR refuses a job enable/disable, `job apply` printed the same message twice (`Warning: ...` then `Failed to apply job: ...`). It now prints the `Warning:` line once, still exits 1, and does not print `Successfully applied` (#187)
- `owlctl job apply` no longer sends `null` entries in `guestProcessing.guestCredentials.credentialsPerMachine` (a bare `-` in YAML), which VBR rejected with 400 "Credentials Per Machine must not be empty". Nulls are dropped; if nothing remains, the live job's value is kept on update, or the key is omitted on create (#147)
- `owlctl init` (and `init settings` / `init profiles`) now creates `--output-dir` (mode 0700) if missing, and exits non-zero with an error naming the path or file if a directory or file can't be written, instead of printing "Initialized successfully" with nothing written. `utils.SaveJson`/`SaveData` no longer discard write errors, and `utils.SaveJsonWithError` returns them (#169)
- `owlctl job apply` now honours `isDisabled` on create and update: VBR ignores it on POST/PUT, so apply reads the job's actual state back and calls `POST jobs/{id}/disable` or `/enable` when it differs. If VBR refuses (for example HTTP 400 for unscheduled jobs), apply prints a warning with VBR's message and exits non-zero instead of reporting success. `--dry-run` shows the planned enable/disable call. The enable/disable call is skipped, in apply and in dry-run, when `isDisabled` is set to `skip` for jobs in `remediation-config.yaml` (#186)

### Changed
- `--dry-run` now uses one shared printer for `job apply` and all generic apply commands: one `No changes detected. Resource is already in desired state.` line, one change format `  <SEVERITY> <~|+|-> <path>: <old> -> <new>` (severity from the resource's drift map, INFO if none), and one `N field(s) would be changed.` summary. String values are no longer truncated at 47 characters in apply output; `job diff` and `job plan` output is unchanged. `job apply --dry-run` drops the box banner, the `(current)`/`(new)` labels, the category headings and the `Summary:` line (#187)
- README changelog table now includes 1.2.1, 1.2.2 and 1.3.0 and links to `CHANGELOG.md` as the full record (#183)
- `examples/pipelines/templates/install-owlctl.yml` now defaults `owlctlVersion` to `v1.3.0` (#170)
- New `CONTRIBUTING.md` with a **Releasing** checklist, including updating the README changelog table (#183)

## [1.3.0] - 2026-10-10

Non-breaking minor release covering everything on `master` since v1.2.2. The old forms of `owlctl export <job-id>` and `-o` for `--overlay` keep working for this release with a deprecation warning on stderr.

### Added
- `owlctl config-backup` singleton resource: `snapshot`, `diff`, `export`, `apply` (#149)
- State format v4, scoped per instance (`instances["<name>"].resources`); v1-v3 files are migrated automatically on first load (#151)
- `owlctl context list/use/current`, kubectl-style aliases over named instances (#153)
- `owlctl state list/show/path` inspection commands (#153)
- Global `owlctl export --all` and `owlctl snapshot --all` across all resource types, with `--all-instances` and `export --from-state` (#152, #162)
- `owlctl instance add/remove/set/get/unset` for managing instances in `owlctl.yaml` (#162)
- `--overlay` on `repo apply`, `repo sobr-apply`, and `encryption kms-apply` (#146)
- `owlctl job export <id>` as the preferred home of single-job export (#146)
- Auth token storage tests (#148)
- Docs overhaul: getting started, tutorial, declarative mode, state management, command reference (#154, #155, #158, #159, #160, #162)
- `owlctl state migrate` moves a legacy `~/.owlctl/state.json` to the current state location, with validation, a timestamped backup, `--dry-run`, `--force`, `--from`/`--to` (#156)
- `examples/pipelines/templates/install-owlctl.yml`: shared Azure DevOps install step with an `owlctlVersion` parameter (default is the explicit release tag `v1.2.2`, defined only there; bump it during release prep). Handles `latest` vs tag URLs, downloads the `.tar.gz` and `.sha256`, verifies the checksum (building the `<hash>  <file>` line itself, since release `.sha256` files contain only the hash), extracts, and smoke-tests with `./owlctl --help` (#166)
- `scripts/check-release-urls.sh` fails on any 404 among owlctl release asset URLs (`.tar.gz`/`.zip`/`.sha256`; bare base URLs are skipped) in `docs/` and `examples/` (and the template's URLs). It runs in CI via `.github/workflows/release-urls.yml` on PRs that touch `docs/` or `examples/` (#166, #178)

### Changed
- **`owlctl export --all` now exports every resource type** (one folder per type) instead of only jobs. Use `owlctl job export --all` for the old jobs-only behaviour (#146, #162)
- CLI consistency pass: single-job export, apply, and overlay flags follow the same shape across jobs, repos, SOBRs, and KMS servers (#146)
- All `examples/pipelines/*.yml` install owlctl through the shared template instead of an inline `curl` of a non-existent bare `owlctl-linux-amd64` asset (#166)
- The GitHub Actions example in `docs/gitops-workflows.md` now verifies the checksum with a valid `<hash>  <file>` line (#166)
- Docs and examples download the real `.tar.gz` (and Windows `.zip`) release assets; the Azure DevOps guide drops the `v0.12.1-beta1` pins and the "Current Integration (v0.12.1)" heading (#166)
- The default state location now follows settings: with `OWLCTL_SETTINGS_PATH` unset, `state.json` resolves to the current directory next to `settings.json` instead of `~/.owlctl/`. A legacy `~/.owlctl/state.json` is detected and warned about, and moved only by `owlctl state migrate` (#156)
- `owlctl state path` prints only the absolute resolved path on stdout; the legacy-state note goes to stderr so `$(owlctl state path)` is script-safe (#156)
- Read-only commands (`diff`, `state list/show/history`) fall back to a legacy `~/.owlctl/state.json` when no state exists at the resolved path, so drift checks never pass silently. Writes go only to the resolved path; the legacy file is never modified (#156)
- `owlctl state migrate` rejects sources that are not owlctl state (version 1-4 structure) and leaves them untouched (#156)
- `examples/pipelines/gitops-deploy.yml` sets `OWLCTL_SETTINGS_PATH` so `git add state.json` commits state (#156)
- Note: `config-backup` is not yet included in `export --all` / `snapshot --all` (#163)

### Deprecated
- `owlctl export <job-id>` (with `-o`, `--as-overlay`, `--base`) is a deprecated alias of `owlctl job export <job-id>`. It behaves identically, prints `Warning: 'owlctl export <id>' is deprecated and will be removed in a future release; use 'owlctl job export <id>'.` to stderr, and will be removed in the next minor release. `export --all` is unaffected by the alias (see Changed)
- `-o` as shorthand for `--overlay` on `job apply` and `job plan` prints `Warning: -o for --overlay is deprecated; use --overlay.` to stderr and will be removed in the next minor release. It was not carried over to the newer repo/SOBR/KMS/config-backup `--overlay` flags
- `--target` remains deprecated (use `--instance`); this is not new in 1.3.0

### Fixed
- `examples/pipelines/deployment.yml` dry-run validation never tolerated exit code 6 because `$?` was re-read after the first test; the exit code is now captured once (#166)
- Install steps in docs and pipelines 404ed on the non-existent bare `owlctl-linux-amd64` asset (#166)
- `state.NewManager()` no longer creates `~/.owlctl` as a side effect, and the active product is read from the same `settings.json` the rest of owlctl uses (#156)

## [1.2.2] - 2026-03-07

### Fixed
- Apply commands reporting false changes for numeric fields where values are identical, e.g. `retryCount: 3 -> 3` (#142)
  - JSON unmarshals numbers as `float64`, YAML as `int`; added numeric type normalization in value comparison

## [1.2.1] - 2026-03-06

### Fixed
- `repo apply`, `repo sobr-apply`, and `encryption kms-apply` failing with HTTP 400 "'Id' must not be empty" (#139)
  - VBR API requires the `id` field in PUT request bodies; moved id injection from job-specific logic to generic apply infrastructure

## [1.2.0] - 2026-02-11

### Added

#### Multi-Instance Connection Management (Epic #135)
- Named instances in `owlctl.yaml` for multi-server automation with per-instance product type, credentials, and TLS settings
- `--instance <name>` persistent flag on all commands — replaces deprecated `--target`
- `instance list` and `instance show <name>` commands
- `credentialRef` support for per-instance environment variable prefixes (`OWLCTL_{ref}_USERNAME` / `OWLCTL_{ref}_PASSWORD`)
- Per-instance keychain token storage
- Group-level `instance` and `specsDir` fields for directory-based spec discovery

#### YAML Export for All Resource Types (#92-#95)
- `--yaml` output for `repo export`, `repo sobr-export`, `encryption export`, `encryption kms-export`

#### Generic Job Pipeline (#131)
- Refactored job pipeline to generic `map[string]interface{}` pattern, matching repos/SOBRs/KMS
- All job types now export/apply with full API fidelity (not just VM backup jobs)

### Changed
- API error messages now include HTTP method, full URL, and response body for all vhttp functions
- `--target` flag deprecated in favor of `--instance` (prints warning)
- `SendData` now treats non-2xx responses as fatal errors (previously only printed status)

### Fixed
- Repeated authentication on Windows (#138)
- CI workflow permissions security fix (#126)

### Testing
- Added test coverage for drift detection and job security severity (#132)
- Added test coverage for group merge paths (#133)
- Added test coverage for state package (#119)
- Added GitHub Actions CI workflow

## [1.1.0] - 2026-02-09

### Epic #96: Groups, Targets & Profile/Overlay System

This release introduces group-based batch operations, named VBR targets, and profile/overlay support for standardised backup policy management across all declarative resource types.

### Added

#### Groups, Profiles & Overlays (Phase 1)
- `--group <name>` flag on `job apply` and `job diff` — batch-apply or drift-check all specs in a group
- `group list` and `group show <name>` commands for inspecting configured groups
- `kind: Profile` and `kind: Overlay` YAML types for standardisation and policy patches
- 3-way strategic merge: profile (base defaults) → spec (identity + exceptions) → overlay (policy patch)
- `groups` section in owlctl.yaml with `profile`, `overlay`, and `specs` references
- `Kind` constants and `IsMixinKind()`/`IsResourceKind()` helpers in resources package

#### Named VBR Targets (Phase 2)
- `targets` section in owlctl.yaml for named VBR server connections
- `--target <name>` persistent flag on all commands — overrides `OWLCTL_URL` for the session
- `target list` command with table and `--json` output

#### Extended Resource Group Support (Phase 3)
- `--group <name>` flag on `repo apply` / `repo diff`
- `--group <name>` flag on `repo sobr-apply` / `repo sobr-diff`
- `--group <name>` flag on `encryption kms-apply` / `encryption kms-diff`

### Changed
- Deprecated `currentEnvironment` and `environments` fields in owlctl.yaml (deprecation warning emitted when present)
- Restructured examples directory for groups/profiles/overlays model
- Updated all documentation for groups/targets/profiles model

## [1.0.0] - 2026-02-07

### Rebrand: vcli is now owlctl

This release marks the rebrand from **vcli** to **owlctl** and the project's first stable release.

See the [Migration Guide](docs/migration-vcli-to-owlctl.md) for upgrade instructions.

### Changed
- Renamed binary from `vcli` to `owlctl`
- Renamed Go module from `github.com/shapedthought/vcli` to `github.com/shapedthought/owlctl`
- Renamed all environment variables: `VCLI_*` → `OWLCTL_*`
- Renamed config directory: `~/.vcli/` → `~/.owlctl/`
- Renamed config file: `vcli.yaml` → `owlctl.yaml`
- Updated API version string: `vcli.veeam.com/v1` → `owlctl.veeam.com/v1`
- Updated keyring service name from `vcli` to `owlctl`
- Updated all pipeline templates, Docker files, and GitHub Actions
- Updated all documentation and examples

### Fixed
- Debug file write in job apply now gated behind `OWLCTL_DEBUG` env var with 0600 permissions
- Fixed incorrect `owlctl job export` remediation guidance to `owlctl export`
- Fixed typo in get command help text
- Updated stale hardcoded version in Check Version utility

## [0.12.1-beta1] - 2026-02-06

### Fixed
- Fix job diff to use raw JSON instead of typed struct (#89)
  - Resolves false-positive drift caused by Go struct serialization differences
  - Job diff now compares raw JSON from state against raw JSON from VBR API

## [0.12.0-beta1] - 2026-02-05

### Added
- Add diff preview to plan and apply --dry-run commands (#82)
  - `job plan` and `job apply --dry-run` now show a field-by-field diff preview
- Add job snapshot command to capture existing jobs into state (#84)
  - `owlctl job snapshot <name>` / `--all` to baseline job state without applying

### Changed
- Expand severity maps for repositories, SOBRs, and KMS servers (#86, #87)
  - Repositories, SOBRs, and KMS servers now have full value-aware severity classification
  - Matches the depth of job severity maps
- Update documentation for expanded severity maps (#86)

### Fixed
- Fix plan command to display full export format
- Fix encryption format mismatch in job apply/export

## [0.11.0-beta1] - 2026-02-04

### Epic #66: Modernized Authentication & Automation

This release delivers major improvements to security, automation workflows, and CI/CD integration through a **clean break** from v0.10.x configuration format.

⚠️ **BREAKING CHANGES** - See [Migration Guide](docs/migration-v0.10-to-v0.11.md)

**Quick upgrade:**
```bash
owlctl init profiles  # Regenerate configs
owlctl login          # Done!
```

### Breaking Changes

#### 1. Non-Interactive Init by Default
- `owlctl init` now non-interactive by default (outputs JSON for scripting)
- Interactive mode available via `--interactive` flag
- Automation-first design for CI/CD workflows
- **Migration:** CI/CD scripts work unchanged; interactive users add `--interactive` flag

#### 2. Profile Commands Take Arguments
- Profile management commands now require arguments instead of prompting
- `owlctl profile --set vbr` (was: interactive prompt)
- JSON output by default; `--table` flag for human-readable format
- **Migration:** Add profile name as argument in scripts

#### 3. Secure Token Storage
- Tokens stored in system keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service)
- File-based fallback with encryption for systems without keychain
- Auto-authentication in CI/CD environments (non-TTY detection)
- Explicit token control via `OWLCTL_TOKEN` environment variable
- **Security:** No more plaintext tokens in `headers.json`
- **Migration:** Just `owlctl login` after upgrade

#### 4. profiles.json v1.0 Format
- Completely restructured configuration format with versioning
- All profiles in single file (easy switching)
- Logical grouping of endpoints and headers
- Proper types (port as number, not string)
- **Migration:** Regenerate with `owlctl init profiles`

#### 5. Removed CredsFileMode
- Credentials now **always** from environment variables
- No more `--creds-file` option
- Consistent with 12-factor app principles and CI/CD standards
- **Migration:** Set `OWLCTL_USERNAME`, `OWLCTL_PASSWORD`, `OWLCTL_URL` environment variables

### Added

#### Authentication Improvements
- System keychain integration for secure token storage
- Hybrid token resolution: `OWLCTL_TOKEN` → keychain → auto-auth
- CI/CD environment detection (skips keychain on headless systems)
- File-based keyring with encryption as fallback
- Password prompting for file keyring in interactive sessions
- `OWLCTL_FILE_KEY` environment variable for non-interactive file keyring

#### Configuration Management
- Versioned profiles.json format (`"version": "1.0"`)
- Multi-profile support in single configuration file
- Structured endpoint and header configuration
- `init profiles` command for profile-only initialization
- `init settings` command for settings-only initialization

#### Commands
- `owlctl init` - Non-interactive with JSON output by default
- `owlctl init --interactive` - Legacy interactive mode
- `owlctl init profiles` - Generate only profiles.json
- `owlctl init settings` - Generate only settings.json
- `owlctl profile --list --table` - Human-readable profile list
- `owlctl profile --set <name>` - Argument-based profile switching

### Changed

#### Behavior Changes
- Default init mode is now non-interactive (JSON output)
- Profile commands require arguments instead of prompting
- Tokens never stored in plaintext files
- Configuration file permissions: `0600` (owner-only)
- Authentication flows through system keychain in interactive sessions
- Auto-authentication in CI/CD without keychain interaction

#### API Version Updates
- VBR: `1.1-rev0` → `1.3-rev1`
- VB365: `v6/v7` → `v8` (consistent version)
- VB for AWS: `1.3-rev0` → `1.4-rev0`
- VB for Azure: `v4` → `v5`
- VB for GCP: `1.1-rev0` → `1.2-rev0`
- VONE: `1.0-rev1` → `1.0-rev2`

### Removed
- `--creds-file` flag and CredsFileMode support
- Plaintext token storage in headers.json
- Interactive prompts from default init behavior
- Legacy profiles.json format (pre-v1.0)

### Security
- **Critical:** Hardcoded file keyring password replaced with `OWLCTL_FILE_KEY` env var + interactive prompt
- **Critical:** Configuration file permissions changed from `0644` to `0600`
- **Critical:** Tokens stored in OS-encrypted keychain instead of plaintext files
- Tilde expansion bug fixed in file keyring path resolution
- Token validation improved (JWT detection + length checks)

### Fixed
- Tilde expansion in file keyring directory path
- Token validation logic (flawed OR condition)
- Flag description inconsistencies (`apiNotSecure` vs `skipTLSVerify`)
- VB365 API version consistency (mixed v6/v7 → v8)
- Removed deprecated `authenticate()` method
- Removed unused code and comments

### Documentation
- Complete documentation restructure and improvements
- New focused guides: Authentication, Imperative Mode, Declarative Mode, State Management, Troubleshooting
- Comprehensive examples for jobs, repositories, SOBRs, KMS servers, and overlays
- Added Getting Started guide for new users
- Created Command Reference quick lookup guide
- Transformed user_guide.md into navigable index page
- **New:** Migration guide (docs/migration-v0.10-to-v0.11.md)
- CI/CD pipeline migration guide
- Troubleshooting common upgrade issues

### Dependencies
- Updated `jose2go` from v1.5.0 to v1.7.0 (fixes Snyk vulnerabilities)

### Design Philosophy: Why Clean Break?

We chose a clean break over backward compatibility:
- **Simpler:** 2-minute config regeneration vs complex migration logic
- **Cleaner:** No legacy format support to maintain
- **Better UX:** Clear errors, single fix that always works
- **Faster delivery:** Ship improvements immediately
- **Industry standard:** Major tools (Terraform, Kubernetes) make clean breaks

### Upgrade Notes

**For all users:**
1. Backup configs (optional): `cp -r ~/.owlctl ~/.owlctl.old`
2. Regenerate: `owlctl init profiles`
3. Re-login: `owlctl login`
4. Test: `owlctl get jobs`

**For CI/CD:**
- Environment variables work unchanged
- Update profile commands to pass arguments: `owlctl profile --set vbr`
- See [Azure DevOps Integration Guide](docs/azure-devops-integration.md)

**Detailed guidance:**
- [Migration Guide](docs/migration-v0.10-to-v0.11.md)

## [0.10.0-beta1] - 2024-01-15

### Epic #42: Declarative Resource Management & Remediation

Complete infrastructure-as-code workflow for VBR with drift detection and security alerting.

### Added

#### Core Features
- **Multi-Resource Declarative Management**
  - Jobs, repositories, SOBRs, encryption passwords, and KMS servers
  - Export resources to YAML format
  - Apply configurations with dry-run support
  - State management for drift detection

#### Export Commands
- `owlctl export <job-id|name>` - Export jobs to YAML (full, simplified, or overlay format)
- `owlctl export --all -d <dir>` - Export all jobs to directory
- `owlctl repo export <name>` - Export repository configuration
- `owlctl repo sobr-export <name>` - Export SOBR configuration
- `owlctl encryption export <name>` - Export encryption password metadata
- `owlctl encryption kms-export <name>` - Export KMS server configuration
- Export flags: `--as-overlay`, `--base`, `--simplified`

#### State Management
- `owlctl repo snapshot <name>` - Capture current repository configuration
- `owlctl repo sobr-snapshot <name>` - Capture current SOBR configuration
- `owlctl encryption snapshot <name>` - Capture current encryption password
- `owlctl encryption kms-snapshot <name>` - Capture current KMS server
- `--all` flag for bulk snapshots
- State file (`state.json`) tracks configurations and origins
- Adopt commands for bringing existing resources under management

#### Apply Commands
- `owlctl job apply <file>` - Apply job configuration (create or update)
- `owlctl repo apply <file>` - Apply repository configuration (update-only)
- `owlctl repo sobr-apply <file>` - Apply SOBR configuration (update-only)
- `owlctl encryption kms-apply <file>` - Apply KMS configuration (update-only)
- Apply flags: `--dry-run`, `-o/--overlay`, `--env`
- Support for configuration overlays (base + environment-specific)

#### Drift Detection
- `owlctl job diff <name>` - Detect job configuration drift
- `owlctl repo diff <name>` - Detect repository configuration drift
- `owlctl repo sobr-diff <name>` - Detect SOBR configuration drift
- `owlctl encryption diff <name>` - Detect encryption password drift
- `owlctl encryption kms-diff <name>` - Detect KMS server drift
- `--all` flag for bulk drift detection
- `--severity <level>` filtering (critical, warning, info)
- `--security-only` flag for security-relevant drifts

#### Security-Aware Severity Classification
- **CRITICAL** severity for security-impacting changes:
  - Encryption disabled
  - Immutability disabled or reduced
  - GFS retention removed
  - Backup chain truncated
  - Application-aware processing disabled
  - Malware detection disabled
- **WARNING** severity for important changes:
  - Retention reduced
  - Compression changed
  - Schedule modified
  - Repository changed
  - Immutability period shortened
- **INFO** severity for minor changes:
  - Description updated
  - Labels changed
  - Non-security settings modified

#### Configuration Overlays
- Strategic merge system for base + overlay configurations
- Support for multiple environments (prod, dev, staging)
- `owlctl.yaml` environment configuration file
- Overlay resolution priority: explicit `-o` > `--env` > `currentEnvironment`
- `owlctl job plan <file>` - Preview merged configuration
- `--show-yaml` flag for full merged output

#### Exit Codes
- **Apply commands:**
  - 0 = Success
  - 1 = Error (API failure, invalid spec)
  - 5 = Partial apply (some fields skipped)
  - 6 = Resource not found (update-only resources)
- **Diff commands:**
  - 0 = No drift
  - 3 = Drift detected (INFO/WARNING)
  - 4 = Critical drift detected
  - 1 = Error occurred

### Changed
- Job export now produces full API fidelity by default (300+ fields)
- Simplified export available via `--simplified` flag for basic jobs
- State management integrated into apply workflow (automatic snapshots)

### Documentation
- New comprehensive drift detection guide
- Security alerting documentation with severity reference
- State management guide
- Azure DevOps integration examples
- Pipeline templates for drift monitoring and deployment

## [0.9.0-beta1] - 2023-10-15

### Added
- Initial declarative job management (jobs only)
- Job export command
- Job apply command with overlay support
- Job plan command
- Basic configuration overlay system
- Environment configuration via owlctl.yaml

## [0.7.0] - 2023-08-01

### Added
- Job template system for creating jobs from templates
- Multi-file job templates with folder support
- Template creation from existing jobs

## [0.5.0] - 2023-05-01

### Added
- Utils command with multiple utilities
- VBR Job JSON Converter (GET to POST format)
- Check Version utility

### Changed
- Improved error handling and user feedback

## [0.4.0] - 2023-03-01

### Added
- PUT command for updating resources
- File-based payload support for POST/PUT
- YAML output format option

## [0.3.0] - 2023-01-01

### Added
- Profile system for multiple Veeam products
- Support for Enterprise Manager, VB365, VONE, cloud products
- Profile management commands (list, get, set, view)
- Creds file mode for faster profile switching

## [0.2.0] - 2022-11-01

### Added
- POST command for triggering operations and creating resources
- Environmental mode for credential management
- TLS certificate verification skip option

## [0.1.0] - 2022-09-01

### Added
- Initial release
- GET command for retrieving data from Veeam APIs
- Basic authentication for VBR
- Login command with token management
- Profile system for VBR
- JSON output format
- Settings and profiles configuration files

### Supported Products
- Veeam Backup & Replication (VBR)
- Enterprise Manager
- Veeam Backup for Microsoft 365
- Veeam ONE
- Veeam Backup for AWS
- Veeam Backup for Azure
- Veeam Backup for GCP

## Version Numbering

owlctl follows [Semantic Versioning](https://semver.org/):
- **MAJOR** version for incompatible API changes
- **MINOR** version for new functionality in a backward compatible manner
- **PATCH** version for backward compatible bug fixes
- **Beta** releases (e.g., 0.10.0-beta1) for preview features

## See Also

- [GitHub Releases](https://github.com/shapedthought/owlctl/releases) - Binary downloads and release notes
- [Getting Started](docs/getting-started.md) - Setup guide
- [Documentation](docs/) - All guides and references
