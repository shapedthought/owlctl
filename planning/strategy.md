owlctl Declarative Infrastructure Management

  Strategic Analysis & Implementation Roadmap

  ---
  Executive Summary

  The Opportunity

  owlctl has achieved product-market fit as a lightweight API wrapper for Veeam administrators who
  need simple, cross-platform access to Veeam APIs. With 8 GitHub stars and active community
  usage, it fills a critical gap for Linux/macOS users and environments without PowerShell access.

  However, the infrastructure-as-code (IaC) landscape has evolved dramatically since owlctl's last
  update (October 2023). Veeam administrators now face:

  - Multi-environment complexity: Managing 5-50+ Veeam servers across dev/staging/prod
  - Compliance pressure: Audit requirements for change tracking and peer review
  - GitOps adoption: Teams want to version control infrastructure, not just code
  - Configuration drift: No systematic way to detect when environments diverge
  - Disaster recovery gaps: Backup configurations themselves aren't backed up declaratively

  The job templates feature proves owlctl users already want declarative workflows - they're
  extracting jobs to YAML, version controlling them, and recreating jobs from templates. They're
  just doing it manually without tooling support.

  Strategic Vision: "Terraform for Veeam"

  Transform owlctl from an imperative API wrapper into a declarative infrastructure management
  platform that enables:

  # veeam-prod.yaml
  apiVersion: owlctl.dev/v1
  kind: Job
  metadata:
    name: prod-database-backup
  spec:
    type: Backup
    repository: prod-repo-01
    virtualMachines:
      includes:
        - name: sql-prod-*
          folder: Production/Databases
    schedule:
      daily: "02:00"
    retention:
      days: 14

  # GitOps workflow
  owlctl apply veeam-prod.yaml          # Create or update to match config
  owlctl plan veeam-prod.yaml           # Show what would change
  owlctl diff                           # Detect drift from declared state
  git commit -m "Add prod DB backup"  # Version control

  Why Now?

  Technical readiness:
  - Job templates feature is 70% of declarative management already
  - Go 1.18+ generics enable type-safe resource abstraction
  - Clean architecture with clear extension points

  Market timing:
  - Terraform 1.6+ shows declarative IaC is table stakes
  - Kubernetes patterns (kubectl apply) are industry standard
  - Veeam lacks native declarative tooling (gap in portfolio)

  User pull:
  - MSPs managing hundreds of Veeam deployments need automation
  - Enterprise teams have change management requirements
  - DevOps teams expect IaC workflows for all infrastructure

  The Recommended Path

  Start with state-managed VBR jobs because:
  1. Builds on proven job templates success
  2. VBR is the most used Veeam product
  3. Jobs are the most frequently managed resource
  4. State file unlocks all future declarative features
  5. Doesn't break existing imperative commands

  3-month roadmap to minimal viable declarative platform:
  - Month 1: State management + idempotent job apply
  - Month 2: Drift detection + plan/apply workflow
  - Month 3: Multi-resource support (repositories, credentials)

  This positions owlctl as the only declarative management tool for Veeam while maintaining its core
   strength: simple, single-binary CLI with zero dependencies.

  ---
  Business Case for Declarative Management

  Problem Statement

  Current Pain Points

  1. Configuration Drift
  - Veeam environments diverge over time (manual changes, emergency fixes)
  - No automated way to detect differences between prod/staging/dev
  - Compliance failures when auditors ask "what changed and when?"

  2. Multi-Environment Management
  - Admin manually replicates job configs across 10+ Veeam servers
  - Copy-paste errors cause production incidents
  - No single source of truth for "standard backup job"

  3. Change Management Theater
  - Teams use spreadsheets to track Veeam changes
  - No peer review before modifying production jobs
  - Rollback means remembering what settings were before

  4. Disaster Recovery Blind Spot
  - Veeam config database backups exist, but...
  - Can't selectively restore one job's configuration
  - Can't preview what would be restored
  - Infrastructure-as-code provides human-readable DR docs

  5. Knowledge Silos
  - Senior admin knows "the right way" to configure jobs
  - Junior admins recreate jobs from scratch (inconsistently)
  - No templating beyond manual copy-paste

  Market Gap Analysis

  Existing solutions and their limitations:
  Tool: PowerShell cmdlets
  Declarative?: No
  Veeam Support: Full (Windows only)
  Limitations: Imperative, requires PowerShell expertise, not cross-platform
  ────────────────────────────────────────
  Tool: Terraform Veeam provider
  Declarative?: Yes
  Veeam Support: Partial (community)
  Limitations: Incomplete API coverage, complex setup, requires Terraform expertise
  ────────────────────────────────────────
  Tool: Ansible modules
  Declarative?: Semi
  Veeam Support: Limited (community)
  Limitations: Not true declarative, playbook complexity
  ────────────────────────────────────────
  Tool: Veeam ONE
  Declarative?: No
  Veeam Support: Monitoring only
  Limitations: No configuration management
  ────────────────────────────────────────
  Tool: Native Veeam UI
  Declarative?: No
  Veeam Support: Full
  Limitations: Manual, GUI-only, not version controllable
  owlctl's competitive advantage:
  - Simpler than Terraform: No HCL learning curve, no provider configuration complexity
  - More capable than PowerShell: Cross-platform, declarative, state-tracked
  - Veeam-native: Already handles auth, API versions, product differences
  - Zero dependencies: Single binary, works anywhere Veeam APIs are accessible

  Value Proposition by User Segment

  Solo Admin (5-15 Veeam Servers)

  Current workflow:
  1. Manually create job in prod Veeam
  2. Screenshot settings or write them down
  3. Log into staging Veeam, recreate manually
  4. Repeat for dev, DR site, etc.
  5. Hope nothing was missed in transcription

  With declarative owlctl:
  # saved-configs/standard-vm-backup.yaml
  apiVersion: owlctl.dev/v1
  kind: Job
  metadata:
    name: nightly-vm-backup
  spec:
    type: Backup
    # ... job config once, apply everywhere

  # Apply to all environments
  for env in prod staging dev dr; do
    owlctl profile use vbr-${env}
    owlctl apply standard-vm-backup.yaml
  done

  Value delivered:
  - Time savings: 30 minutes per job → 2 minutes
  - Consistency: Identical configs across environments
  - Documentation: YAML files are living documentation
  - Disaster recovery: Git repo is backup of all configs

  Enterprise Team (50+ Veeam Servers, Change Management)

  Current workflow:
  1. Admin proposes change in ServiceNow ticket
  2. Manager reviews (no actual code to review)
  3. Admin makes change in prod GUI
  4. Another admin "verifies" by clicking through settings
  5. Change logged in spreadsheet

  With declarative owlctl:
  # Developer workflow
  git checkout -b add-new-backup-job
  vim configs/new-app-backup.yaml
  owlctl plan configs/                    # Preview changes
  git commit -m "Add backup for new app"
  # Open pull request

  # Peer review in GitHub
  # Reviewer sees exact YAML diff
  # Approval in PR = change approval

  # Deployment
  owlctl apply configs/ --auto-approve
  git tag v2024.01.15-prod-deployment

  Value delivered:
  - Audit trail: Git history is immutable change log
  - Peer review: GitHub/GitLab PR workflow built-in
  - Rollback: git revert + owlctl apply = instant rollback
  - Compliance: SOX/HIPAA auditors see full change history
  - Approval workflow: PR approvals = change management

  ROI calculation:
  - Change management overhead: 4 hours/week → 30 minutes/week
  - Configuration errors: 2/month → 0.2/month (caught in PR review)
  - Audit prep time: 40 hours/year → 2 hours/year (git log is audit trail)
  - Annual savings: ~220 hours @ $75/hr = $16,500/year

  MSP (100-500+ Customer Veeam Deployments)

  Current workflow:
  1. Maintain "standard configurations" in wiki
  2. Technician deploys new customer Veeam
  3. Manually creates 15-20 backup jobs per wiki
  4. Quarterly "config audits" to find drift
  5. Spreadsheet tracking which customers have which configs

  With declarative owlctl:
  # Customer onboarding
  git clone internal/veeam-standard-configs
  cd customers/acme-corp
  owlctl profile add vbr-acme-prod --url acme-vbr.cloud
  owlctl apply ../../templates/standard-smb-backup-suite/

  # Quarterly drift detection
  owlctl diff --all-profiles > drift-report-q4.txt
  # Shows which customers diverged from standard

  # Config updates (new compliance requirement)
  vim templates/standard-smb-backup-suite/daily-vm-backup.yaml
  # Update retention from 14 days to 30 days
  for customer in customers/*/; do
    cd $customer
    owlctl apply ../../templates/standard-smb-backup-suite/
  done

  Value delivered:
  - Onboarding speed: 4 hours → 20 minutes
  - Consistency: All customers get proven configs
  - Drift detection: Automated quarterly audits
  - Compliance: Prove to customers their backups meet SLA
  - Scale: One engineer manages 500 deployments vs 50

  ROI calculation:
  - Customer onboarding: 4 hours → 0.5 hours (save 3.5 hours × 50 new customers/year = 175 hours)
  - Config audits: 200 hours/year → 10 hours/year (automated drift detection)
  - Update rollouts: 100 hours/year → 5 hours/year (automated apply)
  - Annual savings: ~460 hours @ $100/hr = $46,000/year

  Success Metrics

  Quantitative Metrics

  Operational efficiency:
  - Configuration deployment time: 30 min → 2 min (93% reduction)
  - Multi-environment updates: 4 hours → 15 minutes (94% reduction)
  - Config drift detection: Manual/quarterly → Automated/continuous

  Quality improvements:
  - Configuration errors: Reduce by 80% (caught in plan phase)
  - Consistency across environments: 65% similar → 99% identical
  - Time to rollback: 2 hours (restore from GUI) → 2 minutes (git revert + apply)

  Compliance & governance:
  - Audit preparation time: 40 hours/year → 2 hours/year
  - Change approval workflow: Manual tickets → Git PR approvals
  - Config documentation accuracy: Often outdated → Always current (config is docs)

  Qualitative Metrics

  User satisfaction:
  - "I can finally version control my Veeam configs"
  - "Onboarding new Veeam servers went from days to minutes"
  - "My manager can actually review changes now (in PRs)"

  Risk reduction:
  - Disaster recovery confidence (can rebuild from Git repo)
  - Reduced fear of making changes (easy rollback)
  - Elimination of "tribal knowledge" (configs are documented)

  Strategic positioning:
  - owlctl becomes essential tool vs nice-to-have
  - Community contribution (users share config templates)
  - Reference architecture for other Veeam tooling

  Success Criteria for MVP (3 months)

  Must achieve:
  - 10+ community users managing VBR jobs declaratively
  - 3 public testimonials about time savings
  - Zero breaking changes to existing imperative commands
  - Documentation with 5+ real-world config examples
  - GitHub stars increase from 8 → 25+

  Would be nice:
  - Blog post from Veeam community member
  - Integration with Veeam Vanguard workflows
  - Terraform users switch to owlctl for simplicity

  ---
  User Stories

  Epic: Declarative Veeam Infrastructure Management

  Persona 1: Sarah - Solo Admin Managing 5 Veeam Environments

  Background:
  - Manages VBR servers for prod, staging, dev, DR site, and branch office
  - Only Veeam admin at mid-sized company (500 VMs)
  - No formal change management, but wants consistency
  - Uses Git for scripts but Veeam configs are manual

  Story 1.1: Consistent Job Configuration Across Environments

  As Sarah, I want to define a backup job once in YAML and apply it to multiple Veeam servers, so
  that I can ensure prod, staging, and dev have identical backup configurations without manual
  replication.

  Acceptance criteria:
  Given I have a YAML file defining a backup job
  When I run `owlctl apply backup-job.yaml` on prod profile
  And I run `owlctl apply backup-job.yaml` on staging profile
  Then both environments have identical job configurations
  And I didn't have to use the GUI twice

  Value: Saves 25 minutes per job × 20 jobs × 4 environments = 33 hours/quarter

  Story 1.2: Detect Configuration Drift

  As Sarah, I want to compare my declared configs against actual Veeam state, so that I can
  identify when someone made manual changes in the GUI that broke consistency.

  Acceptance criteria:
  Given I have applied backup-job.yaml to prod
  When a colleague manually changes retention from 14 to 7 days in GUI
  And I run `owlctl diff`
  Then I see "retention.days: declared 14, actual 7"
  And I can decide to revert or update the YAML

  Value: Catches configuration errors before they cause compliance issues

  Story 1.3: Version Control Backup Configurations

  As Sarah, I want to store Veeam configurations in Git, so that I have an audit trail of changes
  and can rollback if needed.

  Acceptance criteria:
  Given I have veeam-configs/ directory under Git
  When I modify backup-job.yaml and commit
  Then I have a history of what changed, when, and why
  And I can `git revert` + `owlctl apply` to rollback

  Value: Meets compliance requirements, reduces rollback time from hours to minutes

  Persona 2: Marcus - Enterprise Architect at Large Financial Institution

  Background:
  - Oversees 50+ VBR servers across data centers
  - Subject to SOX compliance and change management
  - Has team of 8 Veeam admins (varying skill levels)
  - Requires peer review for all production changes

  Story 2.1: Peer Review Veeam Changes Before Production

  As Marcus, I want to review exact configuration changes in a pull request, so that I can
  approve/reject changes before they hit production Veeam servers.

  Acceptance criteria:
  Given junior admin wants to change backup retention
  When they create PR with modified job YAML
  Then I see exact diff: "retention: 14 days → 30 days"
  And I can comment "approved for compliance"
  And PR approval becomes change approval

  Value: Eliminates change management spreadsheets, catches errors in review

  Story 2.2: Automated Compliance Auditing

  As Marcus, I want to prove to auditors that backup configurations match policy, so that I can
  pass SOX audits without manual config exports.

  Acceptance criteria:
  Given policy requires 30-day retention for financial VMs
  When auditor requests proof
  Then I show YAML files declaring 30-day retention
  And `owlctl diff` output showing zero drift
  And Git log showing no unauthorized changes

  Value: Reduces audit prep from 40 hours to 2 hours, provides stronger evidence

  Story 2.3: Standardized Job Templates Across Team

  As Marcus, I want to define "gold standard" job configurations, so that junior admins deploy
  consistent configs without knowing all best practices.

  Acceptance criteria:
  Given I have templates/gold-standard-vm-backup.yaml
  When junior admin deploys new backup job
  Then they run `owlctl apply templates/gold-standard-vm-backup.yaml`
  And job is created with all best practices (GFS, encryption, etc.)
  And no tribal knowledge required

  Value: Reduces training time, eliminates configuration errors from inexperience

  Story 2.4: Multi-Environment Orchestration

  As Marcus, I want to define environment-specific configs that inherit from base templates, so
  that dev/staging/prod have appropriate differences but share common settings.

  Acceptance criteria:
  # base-template.yaml
  spec:
    type: Backup
    guestProcessing:
      appAware: true
      indexing: true

  # prod-overlay.yaml
  spec:
    retention: 30
    schedule: "daily: 02:00"

  # dev-overlay.yaml
  spec:
    retention: 7
    schedule: "weekly: saturday"

  When I run `owlctl apply base-template.yaml --overlay prod-overlay.yaml`
  Then prod gets 30-day retention with daily schedule
  When I run `owlctl apply base-template.yaml --overlay dev-overlay.yaml`
  Then dev gets 7-day retention with weekly schedule

  Value: Maintains consistency while allowing environment-specific differences

  Persona 3: Jessica - MSP Technical Lead Managing 200 Customer Deployments

  Background:
  - Manages Veeam for 200 SMB customers (1-3 servers each)
  - Needs standardization to scale operations
  - Quarterly compliance checks for all customers
  - High customer churn requires fast onboarding

  Story 3.1: Rapid Customer Onboarding

  As Jessica, I want to deploy standard backup suite to new customer in minutes, so that I can
  onboard 10+ new customers per month without dedicated deployment team.

  Acceptance criteria:
  Given new customer "Acme Corp" signs contract
  When I run:
    owlctl profile add acme-vbr --url acme-vbr.cloud
    owlctl apply templates/smb-standard-suite/
  Then Acme gets 12 standard backup jobs deployed
  And 2 backup repositories configured
  And 1 backup copy job to cloud repo
  And total deployment time < 20 minutes

  Value: Onboarding scales from 4 hours to 20 minutes = 10x efficiency

  Story 3.2: Bulk Configuration Updates

  As Jessica, I want to update all 200 customers to new retention policy, so that I can respond to
   compliance changes without touching 200 GUIs.

  Acceptance criteria:
  Given compliance now requires 60-day retention
  When I update templates/smb-standard-suite/daily-backup.yaml
  And I run `owlctl apply-all --profile-pattern "customer-*"`
  Then all 200 customers get updated retention
  And I have log of which succeeded/failed
  And total time < 2 hours

  Value: Policy updates scale from weeks to hours

  Story 3.3: Automated Drift Detection at Scale

  As Jessica, I want to detect which customers diverged from standard configs, so that I can
  prioritize remediation and bill for unauthorized changes.

  Acceptance criteria:
  Given 200 customers should have standard configs
  When I run `owlctl drift-report --all-customers`
  Then I get report:
    - 180 customers: zero drift ✓
    - 15 customers: minor drift (retention changed)
    - 5 customers: major drift (jobs deleted)
  And I can filter to "show me all major drift"

  Value: Quarterly config audits automated (200 hours → 10 hours)

  Story 3.4: Customer-Specific Customizations

  As Jessica, I want to override specific settings for customers with special requirements, so
  that I can maintain standard base while accommodating unique needs.

  Acceptance criteria:
  # customers/acme/overrides.yaml
  apiVersion: owlctl.dev/v1
  kind: ConfigOverride
  spec:
    profiles:
      - acme-vbr
    jobs:
      - name: daily-vm-backup
        retention: 90  # Acme requires 90 days
        schedule:
          daily: "23:00"  # Acme wants 11pm

  When I run `owlctl apply templates/ --override customers/acme/overrides.yaml`
  Then Acme gets standard jobs BUT with 90-day retention and 11pm schedule
  And other settings still match template

  Value: Balances standardization with customer-specific requirements

  ---
  Technical Architecture

  Design Principles

  1. Backward Compatibility First

  - Existing imperative commands (owlctl get, owlctl post) must continue working unchanged
  - Declarative features are additive, not replacements
  - Migration path from imperative to declarative is optional

  2. Simplicity Over Features

  - Avoid "second system syndrome" - don't over-engineer
  - YAML over HCL (lower learning curve, already used in job templates)
  - Local state files over remote backends (start simple, add complexity later)

  3. Veeam-Native Integration

  - Leverage existing profile system for product abstraction
  - Reuse authentication layer (OAuth + Basic Auth already solved)
  - Maintain zero-dependency single-binary distribution

  4. GitOps-Ready

  - Configuration files are human-readable YAML
  - State files are diffable (can see changes in Git)
  - Designed for version control workflows

  State Management Architecture

  Option Analysis: State Storage
  ┌────────────────┬────────────────────────────────┬───────────────────────────┬────────────────┐
  │     Option     │              Pros              │           Cons            │ Recommendation │
  ├────────────────┼────────────────────────────────┼───────────────────────────┼────────────────┤
  │ Local JSON     │ Simple, no dependencies, works │ Not suitable for teams,   │ Start here     │
  │ file           │  offline                       │ merge conflicts           │ (MVP)          │
  ├────────────────┼────────────────────────────────┼───────────────────────────┼────────────────┤
  │ Remote         │ Team collaboration, locking,   │ Requires cloud            │ Phase 2        │
  │ S3/Azure Blob  │ backup                         │ dependencies, complexity  │                │
  ├────────────────┼────────────────────────────────┼───────────────────────────┼────────────────┤
  │ Git repository │ Version controlled state,      │ Merge conflicts, no       │ Phase 3 option │
  │                │ built-in collaboration         │ locking                   │                │
  ├────────────────┼────────────────────────────────┼───────────────────────────┼────────────────┤
  │ Database       │ Queryable, relational          │ Overkill, binary format   │ Not            │
  │ (SQLite)       │                                │                           │ recommended    │
  └────────────────┴────────────────────────────────┴───────────────────────────┴────────────────┘
  State File Format (MVP)

  Decision: JSON for state (machine-readable), YAML for configs (human-readable)

  // .owlctl-state.json
  {
    "version": 1,
    "profile": "vbr-prod",
    "lastApplied": "2024-01-15T10:30:00Z",
    "resources": {
      "job.prod-database-backup": {
        "apiVersion": "owlctl.dev/v1",
        "kind": "Job",
        "status": "synced",
        "lastSync": "2024-01-15T10:30:00Z",
        "checksum": "sha256:abc123...",
        "apiId": "57b3baab-6237-41bf-add7-db63d41d984c",
        "declaredConfig": {
          // Full YAML config as applied
        },
        "actualConfig": {
          // What API returned last time
        },
        "drift": {
          "detected": false,
          "fields": []
        }
      }
    }
  }

  Key design decisions:
  - Checksum tracking: Detect when YAML file changes
  - Separate declared vs actual: Enable drift detection
  - API ID mapping: Track Veeam resource IDs
  - Status field: synced, drifted, error, pending

  File Format Decision: YAML vs HCL vs TOML
  Format: YAML
  Pros: Already used in job templates, human-friendly, no learning curve
  Cons: Whitespace-sensitive, no schema validation
  Decision: CHOSEN
  ────────────────────────────────────────
  Format: HCL
  Pros: Terraform familiarity, more structured
  Cons: Requires learning, no native Go parsing
  Decision: No - adds complexity
  ────────────────────────────────────────
  Format: TOML
  Pros: Simple syntax, better than YAML errors
  Cons: Less familiar to users
  Decision: No - less common
  Rationale for YAML:
  - Users already writing job-*.yaml files today
  - gopkg.in/yaml.v3 is mature and well-supported
  - Can add JSON Schema validation later for structure checking
  - Lowest friction for current owlctl users

  Config File Structure

  # backup-jobs.yaml
  apiVersion: owlctl.dev/v1
  kind: Job
  metadata:
    name: prod-database-backup
    labels:
      environment: production
      application: database
      compliance: sox
  spec:
    type: Backup
    description: "Production database VM backups"
    virtualMachines:
      includes:
        - type: VirtualMachine
          hostName: vcenter-prod.local
          name: sql-prod-01
        - type: VirtualMachine
          hostName: vcenter-prod.local
          name: sql-prod-02
    storage:
      repository:
        name: prod-repo-01
      retention:
        type: Days
        days: 30
      gfs:
        isEnabled: true
        weekly:
          isEnabled: true
          keepWeeks: 4
        monthly:
          isEnabled: true
          keepMonths: 12
    guestProcessing:
      appAware: true
      isEnabled: true
    schedule:
      dailyOptions:
        type: Everyday
        time: "02:00"
    isDisabled: false

  Design notes:
  - apiVersion/kind: Kubernetes-style versioning (future-proof)
  - metadata.labels: Enable filtering (owlctl diff --label compliance=sox)
  - spec: Mirrors VBR API structure but simplified
  - Repository by name: User-friendly (owlctl resolves to API ID)

  Reconciliation Engine Design

  Core Interface

  // pkg/resources/resource.go
  type Resource interface {
      // Parse YAML into typed struct
      Parse(yamlData []byte) error

      // Validate configuration before API calls
      Validate() error

      // Get current state from Veeam API
      Read(ctx context.Context) error

      // Create resource if doesn't exist
      Create(ctx context.Context) error

      // Update resource to match desired state
      Update(ctx context.Context) error

      // Delete resource
      Delete(ctx context.Context) error

      // Compare desired vs actual state
      Diff() (*DiffResult, error)

      // Metadata for state tracking
      Metadata() ResourceMetadata
  }

  type ResourceMetadata struct {
      APIVersion string
      Kind       string
      Name       string
      Labels     map[string]string
  }

  type DiffResult struct {
      HasDrift bool
      Fields   []FieldDiff
  }

  type FieldDiff struct {
      Path     string  // "spec.storage.retention.days"
      Declared interface{}
      Actual   interface{}
  }

  Implementation for VBR Jobs

  // pkg/resources/vbr_job.go
  type VbrJob struct {
      // Declarative config
      APIVersion string              `yaml:"apiVersion"`
      Kind       string              `yaml:"kind"`
      Metadata   ResourceMetadata    `yaml:"metadata"`
      Spec       models.VbrJobPost   `yaml:"spec"`

      // Runtime state
      profile    models.Profile
      apiID      string  // Veeam job ID
      actualSpec *models.VbrJobGet
  }

  func (j *VbrJob) Parse(yamlData []byte) error {
      return yaml.Unmarshal(yamlData, j)
  }

  func (j *VbrJob) Validate() error {
      // Check required fields
      if j.Spec.Name == "" {
          return fmt.Errorf("spec.name is required")
      }

      // Validate repository exists
      repos := vhttp.GetData[[]Repository]("backupInfrastructure/repositories", j.profile)
      if !repoExists(repos, j.Spec.Storage.Repository.Name) {
          return fmt.Errorf("repository %s not found", j.Spec.Storage.Repository.Name)
      }

      return nil
  }

  func (j *VbrJob) Read(ctx context.Context) error {
      // Try to find existing job by name
      jobs := vhttp.GetData[models.VbrJobsResp]("jobs", j.profile)

      for _, job := range jobs.Data {
          if job.Name == j.Metadata.Name {
              j.apiID = job.ID
              detail := vhttp.GetData[models.VbrJobGet]("jobs/"+job.ID, j.profile)
              j.actualSpec = &detail
              return nil
          }
      }

      // Job doesn't exist yet
      j.apiID = ""
      j.actualSpec = nil
      return nil
  }

  func (j *VbrJob) Create(ctx context.Context) error {
      // Convert declarative spec to API request
      payload, err := json.Marshal(j.Spec)
      if err != nil {
          return err
      }

      // POST to /jobs
      resp := vhttp.PostData("jobs", payload, j.profile)
      j.apiID = resp.ID
      return nil
  }

  func (j *VbrJob) Update(ctx context.Context) error {
      if j.apiID == "" {
          return fmt.Errorf("cannot update: job doesn't exist")
      }

      // PUT to /jobs/{id}
      payload, err := json.Marshal(j.Spec)
      if err != nil {
          return err
      }

      vhttp.PutData("jobs/"+j.apiID, payload, j.profile)
      return nil
  }

  func (j *VbrJob) Diff() (*DiffResult, error) {
      if j.actualSpec == nil {
          return &DiffResult{HasDrift: false}, nil  // Doesn't exist yet
      }

      result := &DiffResult{Fields: []FieldDiff{}}

      // Compare retention
      if j.Spec.Storage.Retention.Days != j.actualSpec.Storage.Retention.Days {
          result.Fields = append(result.Fields, FieldDiff{
              Path:     "spec.storage.retention.days",
              Declared: j.Spec.Storage.Retention.Days,
              Actual:   j.actualSpec.Storage.Retention.Days,
          })
      }

      // Compare schedule
      if j.Spec.Schedule.DailyOptions.Time != j.actualSpec.Schedule.DailyOptions.Time {
          result.Fields = append(result.Fields, FieldDiff{
              Path:     "spec.schedule.dailyOptions.time",
              Declared: j.Spec.Schedule.DailyOptions.Time,
              Actual:   j.actualSpec.Schedule.DailyOptions.Time,
          })
      }

      // ... more field comparisons

      result.HasDrift = len(result.Fields) > 0
      return result, nil
  }

  Apply Workflow

  // pkg/apply/applier.go
  type Applier struct {
      stateFile  *StateFile
      profile    models.Profile
      dryRun     bool
  }

  func (a *Applier) Apply(configFile string) error {
      // 1. Parse YAML config
      yamlData, _ := os.ReadFile(configFile)
      resource, err := a.parseResource(yamlData)
      if err != nil {
          return fmt.Errorf("parse error: %w", err)
      }

      // 2. Validate configuration
      if err := resource.Validate(); err != nil {
          return fmt.Errorf("validation error: %w", err)
      }

      // 3. Read current state from API
      if err := resource.Read(context.Background()); err != nil {
          return fmt.Errorf("read error: %w", err)
      }

      // 4. Determine action needed
      action := a.determineAction(resource)

      // 5. Show plan
      fmt.Printf("Plan: %s resource %s\n", action, resource.Metadata().Name)
      if diff, _ := resource.Diff(); diff.HasDrift {
          for _, field := range diff.Fields {
              fmt.Printf("  ~ %s: %v → %v\n", field.Path, field.Actual, field.Declared)
          }
      }

      // 6. Execute if not dry run
      if !a.dryRun {
          switch action {
          case "create":
              if err := resource.Create(context.Background()); err != nil {
                  return err
              }
              fmt.Println("✓ Created")
          case "update":
              if err := resource.Update(context.Background()); err != nil {
                  return err
              }
              fmt.Println("✓ Updated")
          case "no-op":
              fmt.Println("✓ No changes needed")
          }

          // 7. Update state file
          a.stateFile.UpdateResource(resource)
          a.stateFile.Save()
      }

      return nil
  }

  func (a *Applier) determineAction(r Resource) string {
      if r.apiID == "" {
          return "create"
      }

      diff, _ := r.Diff()
      if diff.HasDrift {
          return "update"
      }

      return "no-op"
  }

  Integration with Current Profile/Auth System

  Leveraging Existing Infrastructure

  No changes needed to:
  - profiles.json - Already handles multi-product configuration
  - settings.json - Already has profile selection
  - headers.json - Already caches auth tokens
  - vhttp package - Already handles OAuth and Basic Auth

  New additions:
  // settings.json (extended)
  {
    "selectedProfile": "vbr-prod",
    "apiNotSecure": true,
    "credsFileMode": false,
    "declarative": {
      "stateFile": ".owlctl-state.json",
      "autoApprove": false,
      "validateBeforeApply": true
    }
  }

  Profile-Scoped State

  Key insight: State files should be profile-specific

  .owlctl/
    state/
      vbr-prod.state.json      # Prod environment state
      vbr-staging.state.json   # Staging environment state
      vb365-prod.state.json    # VB365 product state

  Rationale:
  - Different profiles = different Veeam servers = different state
  - Prevents accidentally applying prod config to staging
  - Enables multi-environment management from single config repo

  Backward Compatibility Strategy

  Non-Breaking Integration

  Imperative commands (unchanged):
  owlctl get jobs              # Still works exactly as before
  owlctl post jobs/123/start   # Still works exactly as before
  owlctl profile use vbr-prod  # Still works exactly as before

  New declarative commands (additive):
  owlctl apply config.yaml     # New command
  owlctl plan config.yaml      # New command
  owlctl diff                  # New command

  Migration Path: Imperative → Declarative

  Export existing resources to YAML:
  # New command to generate declarative configs from existing jobs
  owlctl job export jobs > existing-jobs.yaml

  # Output: existing-jobs.yaml
  ---
  apiVersion: owlctl.dev/v1
  kind: Job
  metadata:
    name: nightly-vm-backup
  spec:
    # ... populated from GET /jobs/{id}
  ---
  apiVersion: owlctl.dev/v1
  kind: Job
  metadata:
    name: weekly-full-backup
  spec:
    # ... populated from GET /jobs/{id}

  Workflow:
  1. owlctl job export jobs > jobs.yaml - Export existing configs
  2. Commit to Git: git add jobs.yaml && git commit
  3. Make changes: vim jobs.yaml
  4. Preview: owlctl plan jobs.yaml
  5. Apply: owlctl apply jobs.yaml

  Coexistence Model

  Declarative doesn't replace imperative:
  - One-off operations still use owlctl get/post/put
  - Declarative for managed resources
  - Users choose which resources to manage declaratively

  Example:
  # Managed declaratively (in version control)
  owlctl apply backup-jobs.yaml

  # Still use imperative for ad-hoc operations
  owlctl post jobs/57b3baab.../start   # Start job manually
  owlctl get sessions --limit 10        # Quick status check

  Comparison: owlctl Declarative vs Alternatives

  owlctl Declarative vs Terraform Veeam Provider
  ┌───────────────────┬───────────────────────────────────┬──────────────────────────────────────┐
  │      Aspect       │         owlctl Declarative          │          Terraform Provider          │
  ├───────────────────┼───────────────────────────────────┼──────────────────────────────────────┤
  │ Learning curve    │ Minimal (YAML, familiar to owlctl   │ Steep (HCL, Terraform concepts)      │
  │                   │ users)                            │                                      │
  ├───────────────────┼───────────────────────────────────┼──────────────────────────────────────┤
  │ Setup complexity  │ Zero (single binary)              │ High (Terraform install, provider    │
  │                   │                                   │ config)                              │
  ├───────────────────┼───────────────────────────────────┼──────────────────────────────────────┤
  │ Veeam API         │ Eventually complete (native tool) │ Limited (community provider)         │
  │ coverage          │                                   │                                      │
  ├───────────────────┼───────────────────────────────────┼──────────────────────────────────────┤
  │ State management  │ Local files (simple)              │ Local or remote (complex)            │
  ├───────────────────┼───────────────────────────────────┼──────────────────────────────────────┤
  │ Dependencies      │ None                              │ Terraform binary required            │
  ├───────────────────┼───────────────────────────────────┼──────────────────────────────────────┤
  │ Integration       │ Native Veeam auth                 │ Requires provider configuration      │
  ├───────────────────┼───────────────────────────────────┼──────────────────────────────────────┤
  │ Multi-product     │ Built-in (profiles)               │ Separate providers                   │
  ├───────────────────┼───────────────────────────────────┼──────────────────────────────────────┤
  │ User base         │ Existing owlctl users               │ Terraform practitioners              │
  └───────────────────┴───────────────────────────────────┴──────────────────────────────────────┘
  When to use owlctl: Veeam-focused teams, simple deployments, rapid prototyping

  When to use Terraform: Multi-cloud IaC, complex orchestration, existing Terraform workflows

  Could owlctl generate Terraform?
  # Interesting idea for Phase 3
  owlctl job export jobs --format terraform > jobs.tf

  Allows users to start with owlctl, migrate to Terraform later if needed.

  owlctl Declarative vs kubectl/Kubernetes Patterns

  Similarities:
  - apiVersion and kind fields
  - owlctl apply mirrors kubectl apply
  - Label selectors for filtering

  Differences:
  - No controller/operator pattern (owlctl is client-side only)
  - No real-time reconciliation (apply on-demand)
  - State stored locally, not in API server

  Why not build a Kubernetes operator?
  - Adds complexity (requires K8s cluster)
  - owlctl's strength is simplicity
  - Client-side tool fits Veeam admin workflows better

  ---
  Phased Implementation Roadmap

  Phase 1: Foundation (Months 1-2)

  Goal: Establish declarative infrastructure with minimal viable features

  Milestone 1.1: State File Management (Week 1-2)

  - State file structure design
  - Load/save state file operations
  - Profile-scoped state files
  - State file locking (prevent concurrent writes)

  Deliverable: .owlctl-state.json file tracks resources

  Milestone 1.2: Resource Abstraction Layer (Week 2-3)

  - Resource interface definition
  - VbrJob resource implementation
  - YAML parsing with validation
  - Error handling framework

  Deliverable: Clean resource abstraction ready for multiple types

  Milestone 1.3: Basic Apply Command (Week 3-4)

  - owlctl apply <file> command
  - Create-if-missing logic
  - Idempotency (detect when no changes needed)
  - State file updates after apply

  Deliverable: Users can apply VBR job YAML files

  Milestone 1.4: Validation Layer (Week 4-5)

  - Pre-flight checks (repository exists, VMs exist)
  - Schema validation (required fields)
  - Helpful error messages
  - Dry-run mode (--dry-run)

  Deliverable: Errors caught before API calls, better UX

  Success Criteria Phase 1:

  - Can create VBR job from YAML
  - Re-running apply doesn't duplicate job
  - State file tracks job ID and checksum
  - Validation catches common errors
  - Zero breaking changes to existing commands

  Phase 2: Core Declarative Features (Months 2-3)

  Goal: Complete plan/apply workflow with drift detection

  Milestone 2.1: Update/Reconciliation (Week 6-7)

  - Detect when config file changes
  - Update existing job via PUT
  - Handle partial failures gracefully
  - Rollback on error

  Deliverable: Changing YAML and re-applying updates the job

  Milestone 2.2: Diff/Drift Detection (Week 7-8)

  - owlctl diff command
  - Compare declared vs actual state
  - Human-readable diff output
  - Filter drifted resources only

  Deliverable: owlctl diff shows what changed outside of owlctl

  Milestone 2.3: Plan Workflow (Week 8-9)

  - owlctl plan <file> command
  - Show what would change (create/update/no-op)
  - Color-coded output (green=create, yellow=update, gray=no-op)
  - Confirmation prompt for apply

  Deliverable: Terraform-like plan/apply safety

  Milestone 2.4: Multi-File Support (Week 9-10)

  - owlctl apply --directory ./configs
  - Batch processing of YAML files
  - Dependency ordering (repositories before jobs)
  - Summary report (3 created, 2 updated, 5 unchanged)

  Deliverable: Manage entire Veeam config as directory of YAML files

  Milestone 2.5: Export Command (Week 10-11)

  - owlctl job export jobs > jobs.yaml
  - Generate declarative YAML from existing resources
  - Migration path for current users
  - Preserve all job settings accurately

  Deliverable: Easy migration from imperative to declarative

  Success Criteria Phase 2:

  - Plan shows exactly what will change
  - Apply updates jobs without recreating
  - Diff detects manual GUI changes
  - Export generates valid YAML
  - Directory mode processes 50+ files efficiently

  Phase 3: Advanced Workflows (Months 4-6)

  Goal: Production-ready features for enterprise and MSP users

  Milestone 3.1: Multi-Resource Support (Week 12-14)

  - Repository resource type
  - Credential resource type
  - Backup Copy Job resource type
  - Cross-resource validation (job references repository)

  Deliverable: Manage repositories and credentials declaratively

  Milestone 3.2: Template/Overlay System (Week 14-16)

  - Base template + environment overlays
  - Variable substitution (${ENV}, ${CUSTOMER_NAME})
  - Inheritance model (dev inherits from base)
  - Merging strategy (deep merge for maps)

  Deliverable: DRY configs for multi-environment deployments

  Milestone 3.3: Label Selectors & Filtering (Week 16-17)

  - Label support in metadata
  - owlctl diff --label environment=prod
  - owlctl apply --label tier=critical
  - Bulk operations by label

  Deliverable: Manage subsets of infrastructure by labels

  Milestone 3.4: Advanced State Management (Week 17-18)

  - Remote state backends (S3, Azure Blob)
  - State locking for team collaboration
  - State encryption (protect sensitive data)
  - State migration tools

  Deliverable: Team-ready state management

  Milestone 3.5: CI/CD Integration (Week 19-20)

  - Exit codes for automation (0=success, 1=drift detected)
  - JSON output mode for parsing
  - --auto-approve flag for pipelines
  - Drift detection as CI check

  Deliverable: GitOps pipeline integration

  Milestone 3.6: Additional Veeam Products (Week 21-24)

  - VB365 resource types (Organizations, Jobs)
  - VONE resource types (Policies)
  - Cloud products (AWS/Azure/GCP)
  - Enterprise Manager support

  Deliverable: Full multi-product declarative management

  Success Criteria Phase 3:

  - Manage 5+ resource types
  - Templates reduce config duplication by 80%
  - Remote state supports team workflows
  - CI/CD pipelines enforce config compliance
  - Community examples for each Veeam product

  ---
  Feature Breakdown: Implementation Tasks

  Task Category 1: State Management

  Task 1.1: State File Structure and Serialization

  Scope: 1-2 days

  Acceptance criteria:
  Given a resource has been applied
  When the state file is written
  Then it contains:
    - Resource metadata (name, kind, apiVersion)
    - API ID mapping (Veeam job ID)
    - Declared config checksum
    - Last sync timestamp
    - Drift status

  Given a state file exists
  When owlctl starts
  Then it loads state without errors
  And handles missing state file gracefully

  Go implementation approach:
  // pkg/state/state.go
  type StateFile struct {
      Version      int                       `json:"version"`
      Profile      string                    `json:"profile"`
      LastModified time.Time                 `json:"lastModified"`
      Resources    map[string]ResourceState  `json:"resources"`
      filepath     string
  }

  type ResourceState struct {
      APIVersion     string                 `json:"apiVersion"`
      Kind           string                 `json:"kind"`
      Name           string                 `json:"name"`
      APIID          string                 `json:"apiId"`
      Checksum       string                 `json:"checksum"`
      LastSync       time.Time              `json:"lastSync"`
      Status         string                 `json:"status"`  // synced, drifted, error
      DeclaredConfig map[string]interface{} `json:"declaredConfig"`
      ActualConfig   map[string]interface{} `json:"actualConfig"`
  }

  func LoadState(profile string) (*StateFile, error) {
      path := stateFilePath(profile)
      if !fileExists(path) {
          return &StateFile{
              Version: 1,
              Profile: profile,
              Resources: make(map[string]ResourceState),
              filepath: path,
          }, nil
      }

      data, err := os.ReadFile(path)
      if err != nil {
          return nil, err
      }

      var state StateFile
      err = json.Unmarshal(data, &state)
      state.filepath = path
      return &state, err
  }

  func (s *StateFile) Save() error {
      s.LastModified = time.Now()
      data, err := json.MarshalIndent(s, "", "  ")
      if err != nil {
          return err
      }
      return os.WriteFile(s.filepath, data, 0644)
  }

  func (s *StateFile) UpdateResource(r Resource) {
      key := fmt.Sprintf("%s.%s", r.Kind(), r.Name())

      // Calculate config checksum
      configBytes, _ := yaml.Marshal(r.Spec())
      checksum := fmt.Sprintf("sha256:%x", sha256.Sum256(configBytes))

      s.Resources[key] = ResourceState{
          APIVersion:     r.APIVersion(),
          Kind:           r.Kind(),
          Name:           r.Name(),
          APIID:          r.APIID(),
          Checksum:       checksum,
          LastSync:       time.Now(),
          Status:         "synced",
          DeclaredConfig: r.Spec(),
          ActualConfig:   r.ActualSpec(),
      }
  }

  Testing strategy:
  // pkg/state/state_test.go
  func TestStateFile_SaveAndLoad(t *testing.T) {
      tempDir := t.TempDir()
      os.Setenv("OWLCTL_SETTINGS_PATH", tempDir)

      // Create state
      state := &StateFile{
          Version: 1,
          Profile: "test-profile",
          Resources: map[string]ResourceState{
              "Job.test-job": {
                  Kind: "Job",
                  Name: "test-job",
                  APIID: "abc-123",
              },
          },
      }

      // Save
      err := state.Save()
      assert.NoError(t, err)

      // Load
      loaded, err := LoadState("test-profile")
      assert.NoError(t, err)
      assert.Equal(t, "abc-123", loaded.Resources["Job.test-job"].APIID)
  }

  Task 1.2: State Locking Mechanism

  Scope: 1 day

  Acceptance criteria:
  Given owlctl apply is running
  When another owlctl apply starts
  Then the second process waits for lock
  And doesn't corrupt state file

  Given state file is locked
  When lock is older than 5 minutes
  Then assume stale lock and break it

  Go implementation:
  // pkg/state/lock.go
  type StateLock struct {
      filepath string
      lockfile string
  }

  func (s *StateFile) Lock() (*StateLock, error) {
      lockfile := s.filepath + ".lock"

      // Try to create lock file
      f, err := os.OpenFile(lockfile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
      if err != nil {
          // Lock exists, check if stale
          if isStale Luck(lockfile, 5*time.Minute) {
              os.Remove(lockfile)
              return s.Lock()  // Retry
          }
          return nil, fmt.Errorf("state file locked, another owlctl process running")
      }

      // Write PID to lock file
      fmt.Fprintf(f, "%d\n%s", os.Getpid(), time.Now().Format(time.RFC3339))
      f.Close()

      return &StateLock{
          filepath: s.filepath,
          lockfile: lockfile,
      }, nil
  }

  func (l *StateLock) Unlock() error {
      return os.Remove(l.lockfile)
  }

  Task 1.3: Profile-Scoped State Files

  Scope: 0.5 days

  Acceptance criteria:
  Given I have profiles "vbr-prod" and "vbr-staging"
  When I apply config to vbr-prod profile
  Then state is saved to .owlctl/state/vbr-prod.state.json
  And vbr-staging.state.json is not affected

  Given I switch profiles
  When I run owlctl diff
  Then it shows drift for the current profile only

  Implementation:
  func stateFilePath(profile string) string {
      settingsPath := utils.SettingPath()
      stateDir := filepath.Join(settingsPath, "state")
      os.MkdirAll(stateDir, 0755)
      return filepath.Join(stateDir, profile+".state.json")
  }

  Task Category 2: Resource Abstraction

  Task 2.1: Resource Interface Definition

  Scope: 1 day

  Acceptance criteria:
  Given a resource interface exists
  When I implement VbrJob resource
  Then it satisfies the interface
  And can be used by generic apply logic

  Go interface:
  // pkg/resources/resource.go
  package resources

  type Resource interface {
      // Metadata
      APIVersion() string
      Kind() string
      Name() string
      Labels() map[string]string

      // Configuration
      Spec() map[string]interface{}
      ActualSpec() map[string]interface{}

      // State
      APIID() string
      SetAPIID(id string)

      // Lifecycle operations
      Parse(yamlData []byte) error
      Validate(ctx context.Context) error
      Read(ctx context.Context) error
      Create(ctx context.Context) error
      Update(ctx context.Context) error
      Delete(ctx context.Context) error

      // Comparison
      Diff() (*DiffResult, error)
      Checksum() string
  }

  type DiffResult struct {
      HasDrift   bool
      Fields     []FieldDiff
  }

  type FieldDiff struct {
      Path      string
      Declared  interface{}
      Actual    interface{}
      DiffType  string  // "changed", "added", "removed"
  }

  // Factory for creating resources from YAML
  func NewResource(yamlData []byte, profile models.Profile) (Resource, error) {
      // Parse apiVersion and kind to determine resource type
      var meta struct {
          APIVersion string `yaml:"apiVersion"`
          Kind       string `yaml:"kind"`
      }

      if err := yaml.Unmarshal(yamlData, &meta); err != nil {
          return nil, err
      }

      // Factory pattern
      switch meta.Kind {
      case "Job":
          return NewVbrJob(yamlData, profile)
      case "Repository":
          return NewVbrRepository(yamlData, profile)
      case "Credential":
          return NewVbrCredential(yamlData, profile)
      default:
          return nil, fmt.Errorf("unknown resource kind: %s", meta.Kind)
      }
  }

  Design patterns:
  - Factory pattern: NewResource() creates correct type based on kind
  - Interface segregation: Could split into smaller interfaces later
  - Context propagation: All operations take context.Context for cancellation

  Task 2.2: VBR Job Resource Implementation

  Scope: 2-3 days (most complex resource)

  Acceptance criteria:
  Given a YAML file defining a VBR job
  When I parse it into VbrJob resource
  Then all fields are correctly mapped

  Given a VbrJob resource
  When I call Validate()
  Then it checks:
    - Required fields present
    - Repository exists in Veeam
    - VM names valid
    - Schedule is valid

  Given a VbrJob resource
  When I call Read()
  Then it fetches current job state from API
  And populates actualSpec

  Given a VbrJob doesn't exist
  When I call Create()
  Then it POSTs to /jobs
  And saves the API-returned job ID

  Given a VbrJob exists
  When I call Update()
  Then it PUTs to /jobs/{id}
  And preserves fields not in spec

  Given declared and actual differ
  When I call Diff()
  Then it returns specific field differences

  Implementation: (Already sketched in architecture section, expand with):

  // pkg/resources/vbr_job.go
  func (j *VbrJob) Validate(ctx context.Context) error {
      var errs []error

      // Required fields
      if j.Spec.Name == "" {
          errs = append(errs, fmt.Errorf("spec.name is required"))
      }

      // Validate repository exists
      if j.Spec.Storage.Repository.Name != "" {
          repos := vhttp.GetData[[]Repository]("backupInfrastructure/repositories", j.profile)
          found := false
          for _, repo := range repos {
              if repo.Name == j.Spec.Storage.Repository.Name {
                  // Resolve name to ID for API call
                  j.Spec.Storage.Repository.ID = repo.ID
                  found = true
                  break
              }
          }
          if !found {
              errs = append(errs, fmt.Errorf("repository '%s' not found",
  j.Spec.Storage.Repository.Name))
          }
      }

      // Validate VMs exist (optional, can be slow)
      // Could add --skip-vm-validation flag

      if len(errs) > 0 {
          return fmt.Errorf("validation failed: %v", errs)
      }
      return nil
  }

  func (j *VbrJob) Diff() (*DiffResult, error) {
      if j.actualSpec == nil {
          return &DiffResult{HasDrift: false}, nil
      }

      result := &DiffResult{Fields: []FieldDiff{}}

      // Use reflection or manual comparison
      // Manual comparison is more explicit and handles Veeam quirks

      if j.Spec.IsDisabled != j.actualSpec.IsDisabled {
          result.Fields = append(result.Fields, FieldDiff{
              Path:     "spec.isDisabled",
              Declared: j.Spec.IsDisabled,
              Actual:   j.actualSpec.IsDisabled,
              DiffType: "changed",
          })
      }

      // Storage differences
      if j.Spec.Storage.Retention.Days != j.actualSpec.Storage.Retention.Days {
          result.Fields = append(result.Fields, FieldDiff{
              Path:     "spec.storage.retention.days",
              Declared: j.Spec.Storage.Retention.Days,
              Actual:   j.actualSpec.Storage.Retention.Days,
              DiffType: "changed",
          })
      }

      // Deep comparison for complex nested structures
      if !reflect.DeepEqual(j.Spec.Schedule, j.actualSpec.Schedule) {
          // Could use more sophisticated diff here
          result.Fields = append(result.Fields, FieldDiff{
              Path:     "spec.schedule",
              Declared: j.Spec.Schedule,
              Actual:   j.actualSpec.Schedule,
              DiffType: "changed",
          })
      }

      result.HasDrift = len(result.Fields) > 0
      return result, nil
  }

  Testing with golden files:
  // pkg/resources/testdata/vbr-job-simple.yaml
  apiVersion: owlctl.dev/v1
  kind: Job
  metadata:
    name: test-backup
  spec:
    type: Backup
    # ... rest of config

  // pkg/resources/vbr_job_test.go
  func TestVbrJob_Parse(t *testing.T) {
      yamlData, _ := os.ReadFile("testdata/vbr-job-simple.yaml")

      job := &VbrJob{}
      err := job.Parse(yamlData)

      assert.NoError(t, err)
      assert.Equal(t, "test-backup", job.Metadata.Name)
      assert.Equal(t, "Backup", job.Spec.Type)
  }

  Task 2.3: VBR Repository Resource Implementation

  Scope: 1-2 days (simpler than Job)

  Acceptance criteria:
  apiVersion: owlctl.dev/v1
  kind: Repository
  metadata:
    name: prod-repo-01
  spec:
    type: LinuxLocal
    server:
      name: linux-repo-server
    path: /backup/veeam
    capacity:
      maxConcurrentJobs: 4

  Given a Repository YAML config
  When I apply it
  Then repository is created via POST /backupInfrastructure/repositories
  And subsequent applies update existing repository

  Implementation approach:
  - Similar to VbrJob but simpler API structure
  - Validate server exists before creating
  - Handle different repository types (Windows, Linux, S3, etc.)

  Task 2.4: Resource Registry Pattern

  Scope: 0.5 days

  Purpose: Allow adding new resource types without modifying core logic

  // pkg/resources/registry.go
  type ResourceFactory func([]byte, models.Profile) (Resource, error)

  var registry = make(map[string]ResourceFactory)

  func Register(kind string, factory ResourceFactory) {
      registry[kind] = factory
  }

  func NewResource(yamlData []byte, profile models.Profile) (Resource, error) {
      var meta struct {
          Kind string `yaml:"kind"`
      }
      yaml.Unmarshal(yamlData, &meta)

      factory, ok := registry[meta.Kind]
      if !ok {
          return nil, fmt.Errorf("unknown kind: %s", meta.Kind)
      }

      return factory(yamlData, profile)
  }

  // Auto-registration in init()
  func init() {
      Register("Job", NewVbrJob)
      Register("Repository", NewVbrRepository)
      Register("Credential", NewVbrCredential)
  }

  Task Category 3: Apply Command

  Task 3.1: Basic Apply Command Structure

  Scope: 1-2 days

  Acceptance criteria:
  owlctl apply job.yaml               # Apply single file
  owlctl apply job.yaml --dry-run     # Show what would happen
  owlctl apply --directory ./configs  # Apply all YAMLs in directory
  owlctl apply job.yaml --auto-approve  # Skip confirmation

  Implementation:
  // cmd/apply.go
  var applyCmd = &cobra.Command{
      Use:   "apply [file or directory]",
      Short: "Create or update resources to match configuration",
      Long: `Apply declarative YAML configuration to Veeam.

  Creates resources if they don't exist, updates if they differ from declared state.
  Idempotent - safe to run multiple times.`,
      Args: cobra.ExactArgs(1),
      Run: runApply,
  }

  var (
      dryRun      bool
      autoApprove bool
      directory   bool
  )

  func init() {
      applyCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would change without
  applying")
      applyCmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "Skip confirmation prompt")
      applyCmd.Flags().BoolVarP(&directory, "directory", "d", false, "Apply all YAML files in
  directory")
      rootCmd.AddCommand(applyCmd)
  }

  func runApply(cmd *cobra.Command, args []string) {
      path := args[0]

      profile := utils.GetProfile()
      state, err := state.LoadState(profile.Name)
      utils.IsErr(err)

      lock, err := state.Lock()
      utils.IsErr(err)
      defer lock.Unlock()

      applier := apply.NewApplier(state, profile, dryRun, autoApprove)

      if directory {
          err = applier.ApplyDirectory(path)
      } else {
          err = applier.ApplyFile(path)
      }

      utils.IsErr(err)
  }

  Task 3.2: Apply Logic with Plan Output

  Scope: 2 days

  Acceptance criteria:
  Given a job config that doesn't exist
  When I run owlctl apply job.yaml
  Then it shows:
    Plan: create resource job.prod-backup
    + Job "prod-backup" will be created
  And prompts for confirmation
  And creates job after approval

  Given a job exists with different retention
  When I run owlctl apply job.yaml
  Then it shows:
    Plan: update resource job.prod-backup
    ~ spec.storage.retention.days: 14 → 30
  And prompts for confirmation
  And updates job after approval

  Given a job matches declared state
  When I run owlctl apply job.yaml
  Then it shows:
    No changes needed
  And doesn't call API

  Implementation:
  // pkg/apply/applier.go
  type Applier struct {
      state       *state.StateFile
      profile     models.Profile
      dryRun      bool
      autoApprove bool
  }

  func (a *Applier) ApplyFile(filepath string) error {
      // Read YAML
      yamlData, err := os.ReadFile(filepath)
      if err != nil {
          return err
      }

      // Parse into resource
      resource, err := resources.NewResource(yamlData, a.profile)
      if err != nil {
          return fmt.Errorf("parse error: %w", err)
      }

      // Validate
      if err := resource.Validate(context.Background()); err != nil {
          return fmt.Errorf("validation error: %w", err)
      }

      // Read current state from API
      if err := resource.Read(context.Background()); err != nil {
          return fmt.Errorf("read error: %w", err)
      }

      // Determine action
      action := a.determineAction(resource)

      // Show plan
      a.printPlan(action, resource)

      // Confirm
      if !a.dryRun && !a.autoApprove {
          if !a.confirm() {
              fmt.Println("Apply cancelled")
              return nil
          }
      }

      // Execute
      if !a.dryRun {
          if err := a.execute(action, resource); err != nil {
              return err
          }

          // Update state
          a.state.UpdateResource(resource)
          a.state.Save()

          fmt.Printf("✓ %s completed\n", action)
      }

      return nil
  }

  func (a *Applier) determineAction(r resources.Resource) string {
      if r.APIID() == "" {
          return "create"
      }

      diff, _ := r.Diff()
      if diff.HasDrift {
          return "update"
      }

      return "no-op"
  }

  func (a *Applier) printPlan(action string, r resources.Resource) {
      switch action {
      case "create":
          fmt.Printf("Plan: %s %s %s\n",
              pterm.Green("create"), r.Kind(), r.Name())
          fmt.Printf("  %s %s \"%s\" will be created\n",
              pterm.Green("+"), r.Kind(), r.Name())

      case "update":
          fmt.Printf("Plan: %s %s %s\n",
              pterm.Yellow("update"), r.Kind(), r.Name())
          diff, _ := r.Diff()
          for _, field := range diff.Fields {
              fmt.Printf("  %s %s: %v → %v\n",
                  pterm.Yellow("~"), field.Path, field.Actual, field.Declared)
          }

      case "no-op":
          fmt.Printf("Plan: %s %s %s\n",
              pterm.Gray("no changes"), r.Kind(), r.Name())
          fmt.Println("  Resource matches declared state")
      }
  }

  func (a *Applier) execute(action string, r resources.Resource) error {
      ctx := context.Background()

      switch action {
      case "create":
          return r.Create(ctx)
      case "update":
          return r.Update(ctx)
      case "no-op":
          return nil
      default:
          return fmt.Errorf("unknown action: %s", action)
      }
  }

  func (a *Applier) confirm() bool {
      result, _ := pterm.DefaultInteractiveConfirm.Show("Apply these changes?")
      return result
  }

  Task 3.3: Directory Mode with Dependency Ordering

  Scope: 1-2 days

  Challenge: Repositories must exist before jobs

  Acceptance criteria:
  Given a directory with:
    - repository.yaml (Repository)
    - backup-job.yaml (Job referencing repository)
  When I run owlctl apply --directory ./configs
  Then repository is created first
  And job is created second
  And dependency order is automatic

  Implementation:
  func (a *Applier) ApplyDirectory(dirpath string) error {
      // Read all YAML files
      files, err := filepath.Glob(filepath.Join(dirpath, "*.yaml"))
      if err != nil {
          return err
      }

      // Parse all resources
      resources := []resources.Resource{}
      for _, file := range files {
          yamlData, _ := os.ReadFile(file)
          r, err := resources.NewResource(yamlData, a.profile)
          if err != nil {
              fmt.Printf("Skipping %s: %v\n", file, err)
              continue
          }
          resources = append(resources, r)
      }

      // Sort by dependency order
      sorted := a.sortByDependencies(resources)

      // Apply each in order
      stats := struct{ created, updated, unchanged, failed int }{}

      for _, r := range sorted {
          err := a.ApplyResource(r)
          if err != nil {
              fmt.Printf("✗ %s %s: %v\n", r.Kind(), r.Name(), err)
              stats.failed++
              continue
          }

          action := a.determineAction(r)
          switch action {
          case "create":
              stats.created++
          case "update":
              stats.updated++
          case "no-op":
              stats.unchanged++
          }
      }

      // Print summary
      fmt.Printf("\nSummary: %d created, %d updated, %d unchanged, %d failed\n",
          stats.created, stats.updated, stats.unchanged, stats.failed)

      return nil
  }

  func (a *Applier) sortByDependencies(resources []resources.Resource) []resources.Resource {
      // Simple dependency order for now
      // Repositories -> Credentials -> Jobs

      order := map[string]int{
          "Repository": 1,
          "Credential": 2,
          "Job":        3,
      }

      sort.Slice(resources, func(i, j int) bool {
          return order[resources[i].Kind()] < order[resources[j].Kind()]
      })

      return resources
  }

  Future enhancement: Detect circular dependencies, build DAG

  Task Category 4: Diff and Plan Commands

  Task 4.1: Diff Command for Drift Detection

  Scope: 1-2 days

  Acceptance criteria:
  owlctl diff                         # Show all drift
  owlctl diff job.yaml                # Show drift for specific resource
  owlctl diff --label env=prod        # Show drift for labeled resources
  owlctl diff --summary               # Just show which resources drifted

  Given I applied job.yaml yesterday
  And someone changed retention in GUI today
  When I run owlctl diff
  Then it shows:
    Job "prod-backup" has drifted
    ~ spec.storage.retention.days: 30 → 14

  Implementation:
  // cmd/diff.go
  var diffCmd = &cobra.Command{
      Use:   "diff [file]",
      Short: "Show differences between declared and actual state",
      Long: `Detect configuration drift by comparing declared YAML configs
  against actual Veeam API state.`,
      Run: runDiff,
  }

  var (
      summaryOnly bool
      labelFilter string
  )

  func init() {
      diffCmd.Flags().BoolVar(&summaryOnly, "summary", false, "Show only resource names, not
  details")
      diffCmd.Flags().StringVar(&labelFilter, "label", "", "Filter by label (key=value)")
      rootCmd.AddCommand(diffCmd)
  }

  func runDiff(cmd *cobra.Command, args []string) {
      profile := utils.GetProfile()
      state, err := state.LoadState(profile.Name)
      utils.IsErr(err)

      differ := diff.NewDiffer(state, profile)

      var result *diff.DiffResult
      if len(args) > 0 {
          // Diff specific file
          result, err = differ.DiffFile(args[0])
      } else {
          // Diff all resources in state
          result, err = differ.DiffAll()
      }
      utils.IsErr(err)

      // Print results
      if summaryOnly {
          differ.PrintSummary(result)
      } else {
          differ.PrintDetailed(result)
      }

      // Exit code: 0 if no drift, 1 if drift detected
      if result.HasDrift {
          os.Exit(1)
      }
  }

  // pkg/diff/differ.go
  type Differ struct {
      state   *state.StateFile
      profile models.Profile
  }

  type DiffResult struct {
      HasDrift  bool
      Resources []ResourceDiff
  }

  type ResourceDiff struct {
      Resource resources.Resource
      Drift    *resources.DiffResult
  }

  func (d *Differ) DiffAll() (*DiffResult, error) {
      result := &DiffResult{}

      for key, resState := range d.state.Resources {
          // Reconstruct resource from declared config in state
          configYAML, _ := yaml.Marshal(resState.DeclaredConfig)
          resource, err := resources.NewResource(configYAML, d.profile)
          if err != nil {
              continue
          }

          // Read current API state
          if err := resource.Read(context.Background()); err != nil {
              return nil, fmt.Errorf("failed to read %s: %w", key, err)
          }

          // Compare
          drift, _ := resource.Diff()
          if drift.HasDrift {
              result.HasDrift = true
              result.Resources = append(result.Resources, ResourceDiff{
                  Resource: resource,
                  Drift:    drift,
              })
          }
      }

      return result, nil
  }

  func (d *Differ) PrintDetailed(result *DiffResult) {
      if !result.HasDrift {
          fmt.Println(pterm.Green("✓ No drift detected - all resources match declared state"))
          return
      }

      fmt.Printf(pterm.Yellow("Drift detected in %d resource(s):\n\n"), len(result.Resources))

      for _, rd := range result.Resources {
          fmt.Printf("%s %s \"%s\"\n",
              pterm.Yellow("~"), rd.Resource.Kind(), rd.Resource.Name())

          for _, field := range rd.Drift.Fields {
              fmt.Printf("  %s %s\n", pterm.Gray("│"), field.Path)
              fmt.Printf("  %s   declared: %v\n", pterm.Gray("│"), field.Declared)
              fmt.Printf("  %s   actual:   %v\n", pterm.Gray("│"), field.Actual)
              fmt.Println()
          }
      }
  }

  Task 4.2: Plan Command (Dry-Run Apply)

  Scope: 0.5 days (reuses apply logic)

  Acceptance criteria:
  owlctl plan job.yaml       # Show what apply would do
  owlctl plan --directory ./configs  # Plan for directory

  Given a job config with changed retention
  When I run owlctl plan job.yaml
  Then it shows planned changes
  And doesn't actually apply them
  And exits with code 0

  Implementation:
  // cmd/plan.go
  var planCmd = &cobra.Command{
      Use:   "plan [file or directory]",
      Short: "Show what changes apply would make",
      Run: runPlan,
  }

  func runPlan(cmd *cobra.Command, args []string) {
      // Plan is just apply --dry-run
      dryRun = true
      autoApprove = true  // No confirmation needed for plan
      runApply(cmd, args)
  }

  Task Category 5: Export and Migration

  Task 5.1: Export Command to Generate YAML

  Scope: 2 days

  Acceptance criteria:
  owlctl job export jobs                    # Export all jobs
  owlctl job export jobs --name prod-*      # Export matching jobs
  owlctl job export jobs --output ./exports # Write to files
  owlctl job export job 57b3baab-...        # Export specific job ID

  Implementation:
  // cmd/export.go
  var exportCmd = &cobra.Command{
      Use:   "export [resource-type]",
      Short: "Export existing resources as declarative YAML",
      Long: `Generate declarative YAML configs from existing Veeam resources.

  Use this to migrate from imperative to declarative management.`,
      Args: cobra.ExactArgs(1),
      Run: runExport,
  }

  var (
      namePattern   string
      outputDir     string
      idFilter      string
  )

  func init() {
      exportCmd.Flags().StringVar(&namePattern, "name", "*", "Filter by name pattern")
      exportCmd.Flags().StringVar(&outputDir, "output", "", "Write files to directory (default:
  stdout)")
      exportCmd.Flags().StringVar(&idFilter, "id", "", "Export specific resource ID")
      rootCmd.AddCommand(exportCmd)
  }

  func runExport(cmd *cobra.Command, args []string) {
      resourceType := args[0]
      profile := utils.GetProfile()

      exporter := export.NewExporter(profile)

      switch resourceType {
      case "jobs":
          exporter.ExportJobs(namePattern, outputDir)
      case "repositories":
          exporter.ExportRepositories(namePattern, outputDir)
      default:
          fmt.Printf("Unknown resource type: %s\n", resourceType)
          os.Exit(1)
      }
  }

  // pkg/export/exporter.go
  func (e *Exporter) ExportJobs(pattern, outputDir string) error {
      // Get all jobs
      jobs := vhttp.GetData[models.VbrJobsResp]("jobs", e.profile)

      exported := 0
      for _, job := range jobs.Data {
          // Filter by pattern
          matched, _ := filepath.Match(pattern, job.Name)
          if !matched {
              continue
          }

          // Get full job details
          detail := vhttp.GetData[models.VbrJobGet]("jobs/"+job.ID, e.profile)

          // Convert to declarative format
          declarative := e.convertJobToDeclarative(detail)

          // Marshal to YAML
          yamlData, _ := yaml.Marshal(declarative)

          // Output
          if outputDir == "" {
              fmt.Println("---")
              fmt.Println(string(yamlData))
          } else {
              filename := filepath.Join(outputDir, sanitizeFilename(job.Name)+".yaml")
              os.WriteFile(filename, yamlData, 0644)
              fmt.Printf("Exported: %s\n", filename)
          }

          exported++
      }

      fmt.Printf("\nExported %d job(s)\n", exported)
      return nil
  }

  func (e *Exporter) convertJobToDeclarative(job models.VbrJobGet) map[string]interface{} {
      return map[string]interface{}{
          "apiVersion": "owlctl.dev/v1",
          "kind":       "Job",
          "metadata": map[string]interface{}{
              "name": job.Name,
              "labels": map[string]string{
                  "exported-from": "vbr",
                  "job-type":      job.Type,
              },
          },
          "spec": job,  // Could clean this up further
      }
  }

  ---
  Prioritization and Recommendation

  Decision Matrix: Which Feature First?
  ┌────────────────────┬──────────────────┬─────────────────┬────────────────┬───────────┬───────┐
  │      Feature       │    User Value    │   Technical     │      Risk      │  Effort   │ Score │
  │                    │                  │   Foundation    │                │           │       │
  ├────────────────────┼──────────────────┼─────────────────┼────────────────┼───────────┼───────┤
  │ State-managed job  │ High (solves     │ High (enables   │ Low (doesn't   │ Medium    │       │
  │ apply              │ duplication)     │ everything)     │ break          │ (2-3      │ 9/10  │
  │                    │                  │                 │ existing)      │ weeks)    │       │
  ├────────────────────┼──────────────────┼─────────────────┼────────────────┼───────────┼───────┤
  │ Drift detection    │ Medium           │ Medium (needs   │                │           │       │
  │ alone              │ (visibility      │ state)          │ Low            │ Low       │ 6/10  │
  │                    │ only)            │                 │                │           │       │
  ├────────────────────┼──────────────────┼─────────────────┼────────────────┼───────────┼───────┤
  │ Multi-resource     │ High (complete   │ Low (needs      │ Medium         │ High      │ 5/10  │
  │ support            │ platform)        │ apply first)    │                │           │       │
  ├────────────────────┼──────────────────┼─────────────────┼────────────────┼───────────┼───────┤
  │ Template/overlay   │ High (MSP use    │ Medium          │ Low            │ Medium    │ 7/10  │
  │ system             │ case)            │                 │                │           │       │
  ├────────────────────┼──────────────────┼─────────────────┼────────────────┼───────────┼───────┤
  │ Remote state       │ Low (team        │ Medium          │ Medium         │ Medium    │ 4/10  │
  │ backends           │ feature, niche)  │                 │                │           │       │
  ├────────────────────┼──────────────────┼─────────────────┼────────────────┼───────────┼───────┤
  │ Export command     │ Medium           │ High (generates │ Low            │ Low       │ 7/10  │
  │                    │ (migration aid)  │  configs)       │                │           │       │
  └────────────────────┴──────────────────┴─────────────────┴────────────────┴───────────┴───────┘
  The One Feature to Build First

  State-Managed VBR Job Apply (with Drift Detection)

  Scope: 3-4 weeks to MVP

  What you'll deliver:
  # Complete workflow
  owlctl job export jobs > existing-jobs.yaml   # Export current state
  owlctl apply existing-jobs.yaml           # Adopt into management
  vim existing-jobs.yaml                  # Make changes
  owlctl plan existing-jobs.yaml            # Preview changes
  owlctl apply existing-jobs.yaml           # Apply updates
  owlctl diff                               # Detect drift

  Why this first:

  1. Builds on Proven Success

  The job templates feature shows users already want this:
  - 70% of the functionality exists (YAML parsing, file composition)
  - Users are manually doing owlctl job template → edit → create
  - You're just adding state tracking and idempotency

  Minimal risk: Extending something that already works

  2. Unlocks Everything Else

  State management is the foundation:
  - Drift detection requires state (can't diff without baseline)
  - Plan/apply workflow requires state (to know what exists)
  - Multi-resource requires state (to track dependencies)
  - GitOps requires state (to version control infrastructure)

  Maximum leverage: Every future feature depends on this

  3. Immediate User Value

  Solo admin:
  # Define once
  owlctl job export jobs/57b3baab > prod-backup.yaml

  # Apply everywhere
  owlctl profile use vbr-staging && owlctl apply prod-backup.yaml
  owlctl profile use vbr-dev && owlctl apply prod-backup.yaml
  owlctl profile use vbr-dr && owlctl apply prod-backup.yaml
  Value: 30 min → 2 min per environment

  Enterprise team:
  git diff  # See exact changes in PR review
  owlctl plan # Preview before apply
  owlctl apply # Auditable deployment
  git log   # Complete audit trail
  Value: Change management + compliance solved

  4. Technical Foundation is Solid

  What you already have:
  - ✅ YAML parsing (yaml.v3)
  - ✅ VBR job models (VbrJobGet, VbrJobPost)
  - ✅ HTTP client (vhttp.GetData, vhttp.PostData)
  - ✅ Profile system (multi-environment support)
  - ✅ Authentication (OAuth + Basic Auth)

  What you need to add:
  - State file management (JSON serialization - trivial)
  - Resource interface (Go interface definition - 1 day)
  - Diff logic (struct comparison - 2 days)
  - Apply command (Cobra command - 2 days)

  Effort: 2-3 weeks for full implementation

  5. Clear Success Metrics

  Week 1-2: Foundation
  - State file saves/loads correctly
  - Resource interface defined
  - VbrJob implements interface

  Week 3: Core functionality
  - owlctl apply job.yaml creates job
  - Re-running apply is idempotent (no duplicate)
  - State file tracks job ID

  Week 4: Polish
  - owlctl diff shows drift
  - owlctl plan shows what would change
  - owlctl job export generates valid YAML

  Success: Community user reports "I'm managing 10 Veeam servers declaratively now"

  Implementation Roadmap (First 4 Weeks)

  Week 1: State Management Foundation

  Days 1-2: State file structure
  - Design state file JSON schema
  - Implement save/load functions
  - Add profile-scoped state files
  - Test state persistence

  Days 3-4: State locking
  - Implement file-based locking
  - Handle stale locks
  - Test concurrent access

  Day 5: Resource interface
  - Define Resource interface
  - Create factory pattern
  - Write interface documentation

  Deliverable: State file infrastructure complete

  Week 2: VBR Job Resource

  Days 1-2: VbrJob implementation
  - Parse YAML into VbrJob struct
  - Implement Read() (fetch from API)
  - Implement Create() (POST)
  - Map between YAML names and API IDs (repository name → ID)

  Days 3-4: Validation and diff
  - Validation logic (check repository exists)
  - Diff implementation (compare declared vs actual)
  - Handle nested structures (Storage, GuestProcessing, Schedule)

  Day 5: Testing
  - Unit tests with golden files
  - Integration test (real API call to dev VBR)
  - Edge case testing (missing fields, invalid refs)

  Deliverable: VbrJob resource fully functional

  Week 3: Apply Command

  Days 1-2: Basic apply logic
  - Cobra command structure
  - Parse config file
  - Determine action (create/update/no-op)
  - Execute action

  Days 3-4: Plan output
  - Pretty-print planned changes
  - Color-coded diff output (pterm)
  - Confirmation prompt
  - --dry-run and --auto-approve flags

  Day 5: Error handling
  - Validation errors (before API call)
  - API errors (network, auth, 404, 500)
  - Rollback on failure
  - User-friendly error messages

  Deliverable: owlctl apply job.yaml works end-to-end

  Week 4: Diff, Export, and Documentation

  Days 1-2: Diff command
  - owlctl diff shows all drift
  - owlctl diff job.yaml for specific file
  - Exit code 1 if drift detected (for CI)
  - Summary and detailed modes

  Day 3: Export command
  - owlctl job export jobs generates YAML
  - Filter by name pattern
  - Output to stdout or files
  - Clean up API response to declarative format

  Days 4-5: Documentation and examples
  - README with declarative workflow
  - Example YAML files (simple, complex, multi-file)
  - Migration guide (imperative → declarative)
  - Video demo or GIF

  Deliverable: Complete MVP ready for community testing

  Presenting to Stakeholders

  To the Engineering Team

  Architecture advantages:
  1. Clean separation of concerns
    - State management (pkg/state)
    - Resource abstraction (pkg/resources)
    - Apply orchestration (pkg/apply)
    - Commands (cmd/)
  2. Type-safe Go patterns
    - Interface-based resources (extensible)
    - Generics for state file (works with any resource)
    - Factory pattern for resource creation
  3. Reconciliation loop
  Read declared config
    ↓
  Validate (pre-flight checks)
    ↓
  Read actual state (API GET)
    ↓
  Diff (compare declared vs actual)
    ↓
  Plan (show what would change)
    ↓
  Execute (CREATE or UPDATE)
    ↓
  Update state file
  4. Error handling strategy
    - Fail fast on validation (before API calls)
    - Graceful degradation (skip invalid resources in directory mode)
    - Atomic operations (state updated only on success)
    - Rollback capability (state file is source of truth)
  5. Testing strategy
    - Golden file tests for YAML parsing
    - Mock API responses for unit tests
    - Integration tests against real dev VBR
    - Diff testing (snapshot actual vs declared)

  To the Veeam Admin Community

  Problem you're solving:
  "I manage 8 Veeam servers. When I create a new backup job, I have to log into each server's GUI
  and manually recreate the job 8 times. If I make a typo, I have inconsistent backups. If someone
   changes a job setting, I don't know unless I manually check. And when auditors ask 'what
  changed last quarter?' I have to dig through logs."

  Solution:
  "Define your backup job once in a YAML file. Apply it to all 8 servers in 30 seconds. Version
  control it in Git. See exactly what changed and when. Detect when someone made manual changes.
  Roll back in one command. Your Veeam infrastructure is now code."

  Real-world scenarios:

  Scenario 1: New application deployment
  # new-app-backup.yaml
  apiVersion: owlctl.dev/v1
  kind: Job
  metadata:
    name: acme-app-backup
  spec:
    virtualMachines:
      includes:
        - name: acme-app-*
    retention: 30
    schedule: "02:00"
  owlctl apply new-app-backup.yaml --profile vbr-prod
  owlctl apply new-app-backup.yaml --profile vbr-dr
  # 2 commands instead of 30 minutes of GUI clicking

  Scenario 2: Compliance change (retention 14→30 days)
  vim backup-jobs.yaml  # Change retention: 30
  git diff              # Review change
  git commit -m "Increase retention for compliance"
  owlctl apply backup-jobs.yaml
  # All jobs updated, change is documented in Git

  Scenario 3: Drift detection
  owlctl diff
  # Output: Job "prod-backup" drifted
  #   ~ retention.days: 30 → 7
  # Someone changed it manually, you found it in 2 seconds

  Time savings:
  - Job deployment: 30 min → 2 min (93% reduction)
  - Multi-environment updates: 4 hours → 15 min (94% reduction)
  - Configuration audits: Manual/quarterly → Automated/continuous
  - Rollback time: 2 hours → 2 minutes (98% reduction)

  Risk reduction:
  - Configuration errors: -80% (caught in validation)
  - Inconsistent environments: -95% (single source of truth)
  - Lost tribal knowledge: Eliminated (configs are self-documenting)

  ---
  Implementation Guidance

  Go Implementation Approach

  Recommended Libraries

  Core dependencies:
  // YAML parsing
  "gopkg.in/yaml.v3"  // Mature, full YAML 1.2 support

  // CLI framework
  "github.com/spf13/cobra"  // Already used, excellent

  // Terminal UI
  "github.com/pterm/pterm"  // Pretty output, interactive prompts, colors

  // HTTP client
  "net/http"  // Standard library (already used)

  // JSON Schema validation (optional, Phase 2)
  "github.com/xeipuuv/gojsonschema"

  // Diff library (optional, can use reflection)
  "github.com/google/go-cmp/cmp"  // Better diffs than reflect.DeepEqual

  Anti-recommendations:
  - ❌ HCL parsing (hashicorp/hcl) - Adds complexity, use YAML
  - ❌ ORM-style libraries - Direct API calls are clearer
  - ❌ Generic diff libraries - Veeam quirks need custom logic

  State Comparison Strategies

  Option 1: Manual field-by-field (Recommended for MVP)
  func (j *VbrJob) Diff() (*DiffResult, error) {
      result := &DiffResult{}

      // Explicit comparisons
      if j.Spec.Storage.Retention.Days != j.actualSpec.Storage.Retention.Days {
          result.Fields = append(result.Fields, FieldDiff{...})
      }

      // Pros: Explicit, handles Veeam quirks, clear logic
      // Cons: Verbose, must update when adding fields

      return result, nil
  }

  Option 2: Reflection-based (Phase 2 optimization)
  func (j *VbrJob) Diff() (*DiffResult, error) {
      diff := cmp.Diff(j.Spec, j.actualSpec,
          cmpopts.IgnoreFields(VbrJobGet{}, "ID", "Type"),  // Ignore computed fields
          cmpopts.EquateApprox(0.01, 0),  // Handle floating point
      )

      // Pros: Automatic, handles new fields
      // Cons: Harder to customize, debug

      return parseCmpDiff(diff), nil
  }

  Recommendation: Start with manual, migrate to go-cmp when patterns are clear

  Resource Abstraction Patterns

  Interface-based polymorphism:
  type Resource interface {
      Kind() string
      Create(ctx context.Context) error
      Update(ctx context.Context) error
      // ...
  }

  // Usage
  func Apply(r Resource) {
      if r.APIID() == "" {
          r.Create(ctx)  // Polymorphic - works for Job, Repository, etc.
      } else {
          r.Update(ctx)
      }
  }

  Type-specific logic via type switch:
  switch r := resource.(type) {
  case *VbrJob:
      // VBR-specific validation
      validateVbrJob(r)
  case *Vb365Org:
      // VB365-specific validation
      validateVb365Org(r)
  }

  Factory pattern for creation:
  func NewResource(yamlData []byte, profile models.Profile) (Resource, error) {
      var meta Metadata
      yaml.Unmarshal(yamlData, &meta)

      switch profile.Name {
      case "vbr":
          return newVbrResource(meta.Kind, yamlData, profile)
      case "vb365":
          return newVb365Resource(meta.Kind, yamlData, profile)
      }
  }

  Potential Gotchas

  1. API Eventual Consistency

  Problem: Job created but GET immediately after returns 404

  Solution:
  func (j *VbrJob) Create(ctx context.Context) error {
      // Create job
      resp := vhttp.PostData("jobs", payload, j.profile)
      j.apiID = resp.ID

      // Wait for eventual consistency
      err := j.waitForJobToExist(ctx, 30*time.Second)
      if err != nil {
          return fmt.Errorf("job created but not visible: %w", err)
      }

      return nil
  }

  func (j *VbrJob) waitForJobToExist(ctx context.Context, timeout time.Duration) error {
      deadline := time.Now().Add(timeout)
      for time.Now().Before(deadline) {
          if err := j.Read(ctx); err == nil && j.actualSpec != nil {
              return nil  // Job visible now
          }
          time.Sleep(1 * time.Second)
      }
      return fmt.Errorf("timeout waiting for job to exist")
  }

  2. Handling Secrets in Declarative Configs

  Problem: Passwords in YAML files committed to Git

  Solutions:

  Option A: External secret references
  spec:
    credentials:
      secretRef:
        name: veeam-creds
        # Resolved from environment or secret manager

  Option B: Credential resource (IDs only)
  # credential.yaml (not committed)
  kind: Credential
  metadata:
    name: sql-backup-account
  spec:
    username: DOMAIN\sqlbackup
    password: ${VCLI_SQL_PASSWORD}  # From env var

  # job.yaml (committed)
  kind: Job
  spec:
    guestProcessing:
      credentials:
        name: sql-backup-account  # Reference only

  Option C: .gitignore secrets
  # .gitignore
  *-secrets.yaml
  credentials/*.yaml

  # configs/job.yaml - Safe to commit
  # configs/job-secrets.yaml - Local only

  Recommendation: Start with Option C, add Option A in Phase 2

  3. Resource Dependencies and Ordering

  Problem: Job references repository that doesn't exist yet

  Solution 1: Dependency annotation
  kind: Job
  metadata:
    name: my-job
    annotations:
      owlctl.dev/depends-on: Repository/prod-repo-01
  spec:
    storage:
      repository:
        name: prod-repo-01

  Solution 2: Automatic dependency detection
  func (j *VbrJob) Dependencies() []string {
      deps := []string{}
      if j.Spec.Storage.Repository.Name != "" {
          deps = append(deps, "Repository/"+j.Spec.Storage.Repository.Name)
      }
      return deps
  }

  // Build dependency graph before apply
  func (a *Applier) buildDependencyGraph(resources []Resource) (*Graph, error) {
      graph := NewGraph()
      for _, r := range resources {
          graph.AddNode(r)
          for _, dep := range r.Dependencies() {
              graph.AddEdge(r, dep)
          }
      }

      if graph.HasCycle() {
          return nil, fmt.Errorf("circular dependency detected")
      }

      return graph, nil
  }

  Recommendation: Start with simple ordering (Repository → Credential → Job), add dependency graph
   in Phase 2

  4. Partial Failure Scenarios

  Problem: 10 jobs in directory, #5 fails, what happens to #6-10?

  Solution: Continue with summary
  func (a *Applier) ApplyDirectory(dir string) error {
      results := []ApplyResult{}

      for _, file := range yamlFiles {
          result := a.ApplyFile(file)
          results = append(results, result)

          // Continue even on error
          if result.Error != nil {
              fmt.Printf("✗ %s: %v\n", file, result.Error)
          }
      }

      // Summary
      success := count(results, func(r ApplyResult) { return r.Error == nil })
      failed := len(results) - success

      fmt.Printf("\nSummary: %d succeeded, %d failed\n", success, failed)

      if failed > 0 {
          return fmt.Errorf("%d resources failed to apply", failed)
      }
      return nil
  }

  Add --fail-fast flag for CI:
  if failFast && result.Error != nil {
      return result.Error  // Stop immediately
  }

  5. API Field Name Mismatches

  Problem: GET returns isEnabled, POST expects enabled

  Solution: Transform layer
  type VbrJobAPI struct {
      IsEnabled bool `json:"isEnabled"`  // API field name
  }

  type VbrJobSpec struct {
      Enabled bool `yaml:"enabled"`  // User-friendly YAML name
  }

  func (j *VbrJob) toAPIPayload() VbrJobAPI {
      return VbrJobAPI{
          IsEnabled: j.Spec.Enabled,  // Transform
      }
  }

  6. Large Response Payloads

  Problem: GET /jobs returns 1000 jobs, slow and memory-intensive

  Solution: Pagination and caching
  func (j *VbrJob) Read(ctx context.Context) error {
      // Don't fetch all jobs, search by name
      jobs := vhttp.GetData[VbrJobsResp]("jobs?nameFilter="+j.Metadata.Name, j.profile)

      if len(jobs.Data) == 0 {
          // Job doesn't exist
          return nil
      }

      // Fetch full details for specific job
      j.apiID = jobs.Data[0].ID
      detail := vhttp.GetData[VbrJobGet]("jobs/"+j.apiID, j.profile)
      j.actualSpec = &detail
      return nil
  }

  Add caching for repeated reads:
  type ResourceCache struct {
      mu    sync.RWMutex
      cache map[string]Resource
      ttl   time.Duration
  }

  func (c *ResourceCache) Get(key string) (Resource, bool) {
      c.mu.RLock()
      defer c.mu.RUnlock()
      r, ok := c.cache[key]
      return r, ok
  }

  Project Structure for Declarative Features

  owlctl/
  ├── cmd/
  │   ├── apply.go           # owlctl apply command
  │   ├── diff.go            # owlctl diff command
  │   ├── export.go          # owlctl job export command
  │   ├── plan.go            # owlctl plan command (alias for apply --dry-run)
  │   ├── get.go             # Existing imperative commands (unchanged)
  │   ├── post.go
  │   ├── ...
  │   └── root.go
  │
  ├── pkg/
  │   ├── state/             # NEW: State management
  │   │   ├── state.go       # StateFile struct, Load/Save
  │   │   ├── lock.go        # File locking
  │   │   └── state_test.go
  │   │
  │   ├── resources/         # NEW: Resource abstraction
  │   │   ├── resource.go    # Resource interface
  │   │   ├── registry.go    # Resource factory
  │   │   ├── vbr_job.go     # VBR Job resource
  │   │   ├── vbr_repository.go  # VBR Repository resource
  │   │   ├── vbr_credential.go  # VBR Credential resource
  │   │   ├── diff.go        # Diff logic
  │   │   ├── testdata/      # Golden file tests
  │   │   │   ├── vbr-job-simple.yaml
  │   │   │   ├── vbr-job-complex.yaml
  │   │   │   └── ...
  │   │   └── resource_test.go
  │   │
  │   ├── apply/             # NEW: Apply orchestration
  │   │   ├── applier.go     # Apply logic
  │   │   ├── planner.go     # Plan output
  │   │   ├── validator.go   # Pre-flight validation
  │   │   └── applier_test.go
  │   │
  │   ├── export/            # NEW: Export existing resources
  │   │   ├── exporter.go
  │   │   └── converters.go  # API → declarative format
  │   │
  │   └── diff/              # NEW: Drift detection
  │       ├── differ.go      # Diff orchestration
  │       ├── output.go      # Pretty printing
  │       └── differ_test.go
  │
  ├── vhttp/                 # Existing HTTP client (unchanged)
  │   ├── getData.go
  │   ├── sendData.go
  │   └── ...
  │
  ├── models/                # Existing models (extend as needed)
  │   ├── vbrjob.go          # VbrJobGet, VbrJobPost
  │   ├── profile.go
  │   └── ...
  │
  ├── utils/                 # Existing utilities (unchanged)
  │   ├── settings.go
  │   ├── profile.go
  │   └── ...
  │
  ├── examples/              # NEW: Example configs
  │   ├── simple-backup.yaml
  │   ├── gfs-backup.yaml
  │   ├── multi-job/
  │   │   ├── repository.yaml
  │   │   ├── job-1.yaml
  │   │   └── job-2.yaml
  │   └── ...
  │
  └── docs/                  # NEW: Documentation
      ├── declarative-guide.md
      ├── migration-guide.md
      └── config-reference.md

  Key organizational principles:
  - cmd/ - User-facing commands (thin, delegate to pkg/)
  - pkg/ - Core logic (thick, tested)
  - vhttp/, models/, utils/ - Existing code (unchanged)
  - examples/ - User-facing documentation
  - docs/ - Detailed guides

  Testing Strategy

  1. Golden File Tests for YAML Parsing

  // pkg/resources/testdata/vbr-job-simple.yaml
  apiVersion: owlctl.dev/v1
  kind: Job
  metadata:
    name: test-job
  spec:
    type: Backup
    storage:
      retention:
        days: 14

  // pkg/resources/vbr_job_test.go
  func TestVbrJob_Parse(t *testing.T) {
      tests := []struct {
          name      string
          file      string
          wantName  string
          wantDays  int
      }{
          {"simple", "vbr-job-simple.yaml", "test-job", 14},
          {"complex", "vbr-job-complex.yaml", "prod-backup", 30},
      }

      for _, tt := range tests {
          t.Run(tt.name, func(t *testing.T) {
              data, _ := os.ReadFile("testdata/" + tt.file)
              job := &VbrJob{}
              err := job.Parse(data)

              assert.NoError(t, err)
              assert.Equal(t, tt.wantName, job.Metadata.Name)
              assert.Equal(t, tt.wantDays, job.Spec.Storage.Retention.Days)
          })
      }
  }

  2. State Comparison Tests

  func TestVbrJob_Diff_DetectsChanges(t *testing.T) {
      job := &VbrJob{
          Spec: VbrJobSpec{
              Storage: Storage{Retention: Retention{Days: 30}},
          },
          actualSpec: &VbrJobGet{
              Storage: Storage{Retention: Retention{Days: 14}},
          },
      }

      diff, err := job.Diff()

      assert.NoError(t, err)
      assert.True(t, diff.HasDrift)
      assert.Len(t, diff.Fields, 1)
      assert.Equal(t, "spec.storage.retention.days", diff.Fields[0].Path)
      assert.Equal(t, 30, diff.Fields[0].Declared)
      assert.Equal(t, 14, diff.Fields[0].Actual)
  }

  3. Integration Tests (Real API)

  // Requires VBR_TEST_URL, VBR_TEST_USERNAME, VBR_TEST_PASSWORD env vars
  func TestVbrJob_CreateUpdateDelete_Integration(t *testing.T) {
      if testing.Short() {
          t.Skip("Skipping integration test")
      }

      profile := loadTestProfile()

      // Create job
      job := &VbrJob{
          Metadata: ResourceMetadata{Name: "test-job-" + randomString()},
          Spec:     loadTestJobSpec(),
          profile:  profile,
      }

      err := job.Create(context.Background())
      assert.NoError(t, err)
      assert.NotEmpty(t, job.APIID())

      defer job.Delete(context.Background())  // Cleanup

      // Update job
      job.Spec.Storage.Retention.Days = 30
      err = job.Update(context.Background())
      assert.NoError(t, err)

      // Verify update
      err = job.Read(context.Background())
      assert.NoError(t, err)
      assert.Equal(t, 30, job.actualSpec.Storage.Retention.Days)
  }

  Run integration tests:
  # Unit tests only
  go test ./pkg/...

  # Include integration tests
  VBR_TEST_URL=https://vbr-dev.local \
  VBR_TEST_USERNAME=admin \
  VBR_TEST_PASSWORD=password \
  go test ./pkg/... -v

  4. End-to-End CLI Tests

  func TestApplyCommand_E2E(t *testing.T) {
      // Create temp config file
      configFile := filepath.Join(t.TempDir(), "job.yaml")
      os.WriteFile(configFile, []byte(`
  apiVersion: owlctl.dev/v1
  kind: Job
  metadata:
    name: e2e-test-job
  spec:
    type: Backup
  `), 0644)

      // Run owlctl apply
      cmd := exec.Command("owlctl", "apply", configFile, "--auto-approve")
      output, err := cmd.CombinedOutput()

      assert.NoError(t, err)
      assert.Contains(t, string(output), "✓ Created")

      // Verify state file
      stateFile := ".owlctl/state/vbr-test.state.json"
      state, _ := state.LoadState("vbr-test")
      assert.Contains(t, state.Resources, "Job.e2e-test-job")
  }

  Example Config Format with Comments

  # Complete VBR Job Example
  # This demonstrates all available fields and their purpose

  apiVersion: owlctl.dev/v1  # Config schema version
  kind: Job                 # Resource type (Job, Repository, Credential)

  metadata:
    name: production-database-backup  # Unique name (must match Veeam job name)
    labels:
      environment: production          # Optional labels for filtering
      application: database
      tier: critical
      compliance: sox
    annotations:
      description: "Daily backup of production SQL servers"  # Documentation
      owner: "database-team@company.com"
      owlctl.dev/depends-on: "Repository/prod-repo-01"  # Dependency hint

  spec:
    # Job type: Backup, BackupCopy, Replica
    type: Backup

    # Human-readable description
    description: "Production database servers - nightly backups with GFS retention"

    # Job enabled/disabled
    isDisabled: false

    # Priority: false = normal, true = high
    isHighPriority: true

    # Virtual machines to back up
    virtualMachines:
      includes:
        # Option 1: Specific VMs by name
        - type: VirtualMachine
          hostName: vcenter-prod.company.local
          name: sql-prod-01
          objectId: vm-1234  # Optional: Veeam object ID (auto-resolved if omitted)

        # Option 2: VM folder (all VMs in folder)
        - type: VirtualMachineFolder
          hostName: vcenter-prod.company.local
          name: Production/Databases

        # Option 3: Pattern matching (if VBR supports)
        - type: VirtualMachine
          hostName: vcenter-prod.company.local
          name: sql-prod-*

      excludes:
        vms:
          - type: VirtualMachine
            hostName: vcenter-prod.company.local
            name: sql-prod-test  # Exclude test VM from backup

        disks:
          - diskId: "scsi0:2"  # Exclude specific disk

        templates:
          isEnabled: true
          excludeFromIncremental: true

    # Storage configuration
    storage:
      # Repository reference (by name, owlctl resolves to ID)
      repository:
        name: prod-repo-01
        # Alternatively, specify ID directly:
        # id: "abc123..."

      # Retention policy
      retention:
        type: Days          # Days, GFS, or Simple
        days: 14            # Keep restore points for 14 days
        keepLastSnapshot: true  # Always keep most recent

      # Grandfather-Father-Son retention
      gfs:
        isEnabled: true

        weekly:
          isEnabled: true
          keepWeeks: 4      # Keep 4 weekly restore points
          dayOfWeek: Saturday

        monthly:
          isEnabled: true
          keepMonths: 12    # Keep 12 monthly restore points
          dayOfMonth: LastDayOfMonth

        yearly:
          isEnabled: true
          keepYears: 7      # Keep 7 yearly restore points (compliance)
          dayOfYear: LastDayOfYear

      # Advanced backup options
      advancedSettings:
        backupMode: IncrementalBackup  # IncrementalBackup, FullBackup
        enableChangeBlockTracking: true

        storage:
          compressionLevel: Medium      # None, Low, Medium, High, Extreme
          storageOptimization: Local    # Local, LAN, WAN
          enableInlineDataDedup: true
          enableEncryption: true
          encryptionKey:
            name: prod-encryption-key   # Reference to encryption key resource

    # Guest OS processing (application-aware backups)
    guestProcessing:
      isEnabled: true
      appAware: true                    # Application-aware processing

      # SQL Server settings
      sqlSettings:
        transactionLogHandling: TruncateOnlyOnSuccessfulBackup
        backupLogsFrequencyMin: 15      # Transaction log backup every 15 min
        truncateLogs: true

      # Guest OS credentials
      credentials:
        # Reference to credential resource
        name: sql-backup-account
        # Or specify inline (not recommended - use credential resource)
        # username: DOMAIN\sqlbackup
        # passwordRef: ${VCLI_SQL_PASSWORD}  # From environment variable

      # Guest file system indexing
      indexing:
        isEnabled: true
        type: EntireVM                  # EntireVM or SelectedFolders
        selectedFolders:
          - "C:\\Data"
          - "D:\\Logs"

      # Pre/post scripts
      scripts:
        preJobScript:
          path: "C:\\Scripts\\pre-backup.bat"
          arguments: "prod"

        postJobScript:
          path: "C:\\Scripts\\post-backup.bat"
          arguments: "prod"
          runOnFailure: true

    # Schedule configuration
    schedule:
      # Daily schedule (runs every day at specified time)
      dailyOptions:
        type: Everyday                  # Everyday or Weekdays
        time: "02:00"                   # 2:00 AM
        timeZone: "Eastern Standard Time"

      # Alternatively: Monthly schedule
      # monthlyOptions:
      #   dayOfMonth: 1                 # First day of month
      #   time: "03:00"

      # Alternatively: Periodic schedule
      # periodicOptions:
      #   periodicallyEvery: 6          # Every 6 hours
      #   periodicallyEveryUnit: Hours  # Hours or Minutes

      # Retry settings
      retryOptions:
        isEnabled: true
        retryTimes: 3                   # Retry up to 3 times
        retryWaitInterval: 10           # Wait 10 minutes between retries

      # Terminate if exceeds window
      terminateIfExceedsWindow: true
      windowMinutes: 240                # 4-hour backup window

  # Example: Minimal job configuration
  # ---
  # apiVersion: owlctl.dev/v1
  # kind: Job
  # metadata:
  #   name: simple-backup
  # spec:
  #   type: Backup
  #   virtualMachines:
  #     includes:
  #       - type: VirtualMachine
  #         hostName: vcenter.local
  #         name: my-vm
  #   storage:
  #     repository:
  #       name: default-repo
  #     retention:
  #       days: 7
  #   schedule:
  #     dailyOptions:
  #       time: "22:00"

  # Example: Multi-file configuration
  # Separate concerns into multiple files, apply directory:
  #
  # configs/
  #   repository.yaml      # Repository resource
  #   credentials.yaml     # Credential resource
  #   job-base.yaml        # Job definition
  #
  # owlctl apply --directory configs/

  Backward Compatibility: Imperative → Declarative

  Migration Workflow

  Step 1: Export existing infrastructure
  # Export all jobs as YAML
  owlctl job export jobs --output ./configs

  # Output:
  # Exported: configs/nightly-vm-backup.yaml
  # Exported: configs/weekly-full-backup.yaml
  # Exported: configs/database-backup.yaml
  # Exported 3 job(s)

  Step 2: Review generated configs
  ls configs/
  # nightly-vm-backup.yaml
  # weekly-full-backup.yaml
  # database-backup.yaml

  cat configs/nightly-vm-backup.yaml
  # apiVersion: owlctl.dev/v1
  # kind: Job
  # metadata:
  #   name: nightly-vm-backup
  # spec:
  #   type: Backup
  #   ... (full config)

  Step 3: Adopt into declarative management
  # Initial apply adopts existing resources (no changes made)
  owlctl apply --directory configs/

  # Output:
  # Plan: no changes Job nightly-vm-backup (existing resource matches config)
  # Plan: no changes Job weekly-full-backup (existing resource matches config)
  # Plan: no changes Job database-backup (existing resource matches config)
  # Summary: 0 created, 0 updated, 3 unchanged

  State file now tracks these resources.

  Step 4: Make changes declaratively
  vim configs/database-backup.yaml
  # Change retention: 30

  owlctl plan configs/database-backup.yaml
  # Plan: update Job database-backup
  #   ~ spec.storage.retention.days: 14 → 30

  owlctl apply configs/database-backup.yaml
  # ✓ Updated

  Step 5: Version control
  git add configs/
  git commit -m "Adopt Veeam jobs into declarative management"
  git push

  Coexistence Examples

  Use case 1: Managed declaratively, operated imperatively
  # Managed in version control
  owlctl apply production-jobs.yaml

  # Ad-hoc operations still use imperative commands
  owlctl post jobs/57b3baab.../start         # Start job now
  owlctl get jobs/57b3baab.../sessions       # Check job status
  owlctl delete jobs/57b3baab.../sessions/123  # Delete session

  Use case 2: Hybrid - some resources declarative, others imperative
  # Production jobs: Declarative (change-controlled)
  owlctl apply configs/production/

  # Test/dev jobs: Imperative (ephemeral, no change management)
  owlctl post jobs --file test-job.json

  Use case 3: Declarative exports for documentation
  # Generate current state as documentation
  owlctl job export jobs > docs/current-veeam-jobs.yaml

  # Commit to repo (read-only documentation)
  git add docs/current-veeam-jobs.yaml
  git commit -m "Update Veeam job documentation"

  ---
  Conclusion and Next Steps

  Summary

  owlctl has a proven foundation and active user base. The job templates feature demonstrates that
  users already want declarative workflows - they're just doing it manually. By adding state
  management and idempotent apply operations, you unlock:

  For users:
  - Multi-environment consistency (define once, apply everywhere)
  - Version control (Git history is audit trail)
  - Drift detection (know when configs diverge)
  - Faster operations (30 min → 2 min)
  - Reduced errors (validation before API calls)

  For the project:
  - Strategic differentiation (only declarative Veeam tool)
  - Foundation for future features (all declarative features need state)
  - Community engagement (users contribute config templates)
  - Ecosystem integration (GitOps pipelines, CI/CD)

  Recommended first implementation:
  State-managed VBR job apply - 3-4 weeks to MVP

  Success criteria:
  - owlctl job export jobs > jobs.yaml generates valid configs
  - owlctl apply jobs.yaml creates or updates jobs idempotently
  - owlctl diff detects configuration drift
  - owlctl plan previews changes before applying
  - Zero breaking changes to existing imperative commands
  - 5+ community users managing Veeam declaratively

  Immediate Action Items

  Week 0 (Planning):
  - Review this analysis with stakeholders
  - Confirm scope and timeline for MVP
  - Set up dev/test VBR environment
  - Create GitHub project board for tracking

  Week 1 (Foundation):
  - Implement state file structure
  - Add profile-scoped state files
  - Implement file locking
  - Define Resource interface
  - Write comprehensive tests

  Week 2 (Core Resource):
  - Implement VbrJob resource (Parse, Validate, Read, Create, Update, Diff)
  - Handle YAML name → API ID resolution (repositories, VMs)
  - Golden file tests
  - Integration tests against dev VBR

  Week 3 (Apply Command):
  - Implement apply command (single file, directory)
  - Plan output with color-coded diffs
  - Confirmation prompts
  - Error handling and rollback
  - Idempotency testing (run apply twice, no duplicates)

  Week 4 (Polish and Release):
  - Diff command for drift detection
  - Export command for migration
  - Documentation (README, examples, migration guide)
  - Demo video or animated GIF
  - Community announcement (Veeam forums, Reddit, blog post)

  Community Engagement Strategy

  Pre-release:
  - Create discussion thread: "Declarative Veeam management - what would you use?"
  - Share architecture draft for feedback
  - Recruit 3-5 beta testers from different user segments

  Launch:
  - GitHub release with detailed changelog
  - Blog post: "Introducing Declarative Veeam Infrastructure with owlctl"
  - Reddit r/Veeam post with demo video
  - Tweet storm with examples

  Post-launch:
  - User success stories (case studies)
  - Community config template repository
  - Monthly "Config of the Month" showcase
  - Integration guides (GitLab CI, GitHub Actions, Jenkins)

  Long-Term Vision (6-12 Months)

  Phase 2 milestones:
  - Multi-resource support (repositories, credentials, backup copy jobs)
  - Template/overlay system (base + environment-specific)
  - Remote state backends (S3, Azure Blob)
  - Label selectors and filtering

  Phase 3 milestones:
  - Additional Veeam products (VB365, VONE, cloud products)
  - Configuration compliance scanning
  - Policy-as-code (OPA integration)
  - Terraform provider generation (owlctl job export --format terraform)

  Ultimate vision:
  owlctl becomes the de facto standard for declarative Veeam infrastructure management, bridging the
   gap between traditional backup administration and modern infrastructure-as-code practices. MSPs
   use it to manage thousands of deployments. Enterprises use it for change management. Solo
  admins use it to maintain consistency. Veeam officially recommends it.

  ---
  Ready to start building? Let's begin with Week 1: State Management Foundation. Would you like me
   to:

  1. Draft the initial Go code for state file implementation?
  2. Create the Resource interface definition with examples?
  3. Write the GitHub issue templates for community contributions?
  4. Design the state file JSON schema in detail?

  Let me know how you'd like to proceed!