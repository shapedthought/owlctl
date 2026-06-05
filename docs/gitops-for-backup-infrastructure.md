# Why GitOps for Backup Infrastructure

> **This is the conceptual guide** — the case for managing Veeam backup infrastructure as code and how owlctl fits the wider infrastructure-as-code ecosystem. For hands-on setup, see the [GitOps Workflows Guide](gitops-workflows.md) (GitHub Actions, Azure DevOps, GitLab CI) and the [Azure DevOps Integration Guide](azure-devops-integration.md). Ready-to-use pipelines live in [`examples/pipelines/`](../examples/pipelines/).

## Executive Summary

owlctl is an open-source, community-supported CLI tool that brings GitOps workflows to Veeam Backup & Replication. It enables teams to manage backup infrastructure declaratively -- exporting configurations as YAML, tracking state, detecting drift, and remediating changes through CI/CD pipelines.

This document explains why API-driven GitOps for backup infrastructure is a natural extension of modern DevOps practice, how owlctl enables this for Veeam environments through Azure DevOps, and how this approach aligns with industry-wide patterns already established across the infrastructure ecosystem.

**owlctl is not an official Veeam product.** It is a community open-source project that uses Veeam's publicly documented REST APIs. It is not supported by Veeam R&D, QA, or product support. This is consistent with how the broader backup and infrastructure industry operates -- vendor APIs are public, and the ecosystem of tools built on them is largely open-source and community-driven.

---

## Why APIs Are the Foundation of Modern Infrastructure Management

Every major infrastructure platform is managed through APIs. This is not a trend -- it is the established standard for how organisations operate at scale.

### The Pattern

The workflow is the same regardless of the vendor or domain:

1. **Declare** desired state in version-controlled files (YAML, HCL, JSON)
2. **Compare** desired state against live state via API
3. **Plan** what changes are needed
4. **Apply** changes through the API
5. **Detect drift** when live state diverges from desired state

This is the pattern behind Terraform, Kubernetes, ArgoCD, Ansible, and every GitOps tool in production today. It works because vendors expose APIs, and the community builds tooling on top of them.

### Why This Pattern Exists

**Scale demands automation.** Manual configuration through GUIs does not scale. When an organisation manages tens or hundreds of backup jobs, repositories, and encryption settings, configuration changes must be reviewable, auditable, and repeatable. APIs make this possible. GUIs do not.

**Compliance requires auditability.** Git provides a complete audit trail of every configuration change: who changed what, when, why (commit message), and who approved it (pull request review). This is the same audit trail that organisations already rely on for application code and infrastructure-as-code.

**Drift is inevitable.** In any environment, configurations change -- through manual intervention, emergency fixes, or mistakes. Without automated drift detection, these changes go unnoticed until they cause a failure. API-driven tools can continuously compare desired state against live state and alert on divergence.

**Consistency across environments.** The same configuration patterns that work for dev/test should be promotable to production with environment-specific overrides. This is standard practice for application deployments and should be standard practice for backup infrastructure.

### Who Uses This Pattern

| Tool | Domain | Pattern | API Dependency |
|------|--------|---------|----------------|
| **Terraform** | Infrastructure provisioning | Declare resources in HCL, plan/apply via vendor APIs | 4,000+ provider plugins wrapping vendor APIs |
| **ArgoCD** | Application delivery | Declare desired state in Git, reconcile against Kubernetes API | Kubernetes API |
| **Crossplane** | Infrastructure from Kubernetes | CRDs reconciled against cloud provider APIs | AWS, Azure, GCP APIs |
| **Ansible** | Configuration management | Declare desired state in YAML playbooks, apply via vendor APIs/modules | Vendor APIs and SSH |
| **Helm** | Kubernetes package management | Declare application configuration as charts, apply to Kubernetes API | Kubernetes API |
| **Flux CD** | GitOps toolkit | Continuous reconciliation of Git state against Kubernetes API | Kubernetes API |

Every one of these tools is a wrapper around vendor APIs. Terraform providers are literally described by HashiCorp as "a Terraform-specific wrapper for an API client." ArgoCD's entire purpose is to reconcile Git-declared state against the Kubernetes API. The pattern is universal because APIs are universal.

---

## Open Source Is How Enterprise Infrastructure Tooling Works

A common concern is whether open-source tools are appropriate for enterprise infrastructure. The answer is clear: **open-source tools are the foundation of modern enterprise infrastructure.** The most critical tools in any organisation's stack are open source.

### Enterprise-Critical Open-Source Tools

| Tool | License | GitHub Stars | Enterprise Adoption |
|------|---------|-------------|---------------------|
| **Kubernetes** | Apache 2.0 | ~120,000 | Foundation of cloud-native infrastructure; CNCF graduated project |
| **Terraform** | BSL 1.1 / OpenTofu: MPL 2.0 | ~43,000 | Dominant IaC tool; 4,000+ providers |
| **Helm** | Apache 2.0 | ~29,300 | Kubernetes package manager; CNCF graduated; contributors from Google, Microsoft, Samsung, VMware, IBM |
| **Ansible** | GPL 3.0 | ~63,000 | 4,000,000+ customer systems automated worldwide |
| **ArgoCD** | Apache 2.0 | ~21,900 | Running in ~60% of Kubernetes clusters; CNCF graduated |
| **Flux CD** | Apache 2.0 | ~7,800 | CNCF graduated GitOps toolkit |

Kubernetes -- the platform that runs the majority of modern enterprise workloads -- is open source. Helm -- the tool that packages and deploys applications onto Kubernetes -- is open source. ArgoCD -- the tool that implements GitOps for those deployments -- is open source. Terraform -- the tool that provisions the infrastructure underneath -- is open source (or source-available under BSL, with OpenTofu as the fully open fork under the Linux Foundation).

These are not peripheral tools. They are the foundation. Enterprises build their entire operational model on open-source infrastructure tooling, supplementing with commercial support contracts where needed.

The same logic applies to backup infrastructure automation. The APIs are provided by the vendor. The tooling built on those APIs follows the same open-source patterns that the rest of the infrastructure ecosystem relies on.

---

## Backup Vendors and Open-Source Automation

owlctl is not an anomaly. Every major backup vendor has community-driven, open-source automation tooling built on their APIs. This is the normal way that backup automation ecosystems develop.

### Industry Landscape

| Vendor | GitHub Presence | Key Open-Source Tools | Official Support |
|--------|----------------|----------------------|-----------------|
| **Rubrik** | [github.com/rubrikinc](https://github.com/rubrikinc) | PowerShell SDK, Python SDK, Go SDK, Terraform providers (CDM + Polaris) | Mixed: RSC PowerShell SDK is supported; CDM SDKs are community/best-effort |
| **Commvault** | [github.com/Commvault](https://github.com/Commvault) | Terraform provider, Ansible collection (Galaxy), Python SDK (CVPySDK) | Community-supported; published on Terraform Registry and Ansible Galaxy |
| **Cohesity** | [github.com/cohesity](https://github.com/cohesity) | Python SDK, Ansible role (5/5 on Galaxy), Terraform provider | Community-supported; Terraform provider previously hosted by HashiCorp |
| **Veritas** | [github.com/VeritasOS](https://github.com/VeritasOS) | Ansible roles for NetBackup deployment, REST API code samples | Community-supported under MIT license |
| **Dell EMC** | Community repos | Postman collections for NetWorker, DataDomain, Avamar APIs | Community-maintained |
| **Veeam** | [github.com/VeeamHub](https://github.com/VeeamHub) | PowerShell scripts (322 stars), Go SDK, Ansible collection, Terraform samples, Grafana dashboards, Postman collections | Community-supported; "not created by Veeam R&D or validated by Veeam Q&A" |

### Key Observations

**Every vendor follows the same model.** The vendor provides REST APIs and documentation. The community (which often includes vendor employees acting in a community capacity) builds SDKs, CLI tools, Terraform providers, and Ansible modules on top of those APIs. These tools are published on GitHub under community-supported licenses.

**Rubrik is the closest comparison.** Rubrik's ecosystem includes official and community SDKs across PowerShell, Python, and Go, plus Terraform providers for both their on-premises (CDM) and cloud (Polaris/RSC) platforms. Their CDM SDKs and Terraform provider are community/best-effort supported -- the same model as owlctl.

**No backup vendor provides a fully supported GitOps tool.** Terraform providers exist for Rubrik, Commvault, and Cohesity, enabling basic IaC workflows. But none of them provide security-aware drift detection, value-aware severity classification, cross-resource validation, or CI/CD-ready exit codes. These are the capabilities that differentiate owlctl.

### VeeamHub: Veeam's Community Ecosystem

Veeam actively encourages API-driven automation. The VeeamHub GitHub organisation ([github.com/VeeamHub](https://github.com/VeeamHub)) hosts 39+ repositories covering:

- **PowerShell scripts** (322 stars) -- The most popular VeeamHub repository
- **Go SDK** ([veeam-vbr-sdk-go](https://github.com/VeeamHub/veeam-vbr-sdk-go)) -- Auto-generated from the VBR OpenAPI specification
- **Ansible collection** ([veeam-ansible](https://github.com/VeeamHub/veeam-ansible)) -- Published on Ansible Galaxy as `veeamhub.veeam`
- **Terraform samples** ([veeam-terraform](https://github.com/VeeamHub/veeam-terraform)) -- Sample HCL for deploying Veeam infrastructure
- **Postman collections** -- Interactive API exploration
- **Grafana dashboards** -- Monitoring and observability
- **Security tools** -- Honeypot/decoy, vulnerability scanning, hardened repository guides

VeeamHub projects are explicitly described as community-supported: "Scripts in this repository are community driven projects and are not created by Veeam R&D or validated by Veeam Q&A. They are maintained by community members which may or not be Veeam employees."

owlctl sits alongside these projects in the Veeam community ecosystem. It uses the same publicly documented VBR REST API (port 9419, OAuth2 authentication, OpenAPI3 specification) that Veeam introduced in v11 specifically to enable automation.

---

## What owlctl Enables for Azure DevOps

owlctl brings the declare/compare/apply pattern to Veeam backup infrastructure, with first-class Azure DevOps integration.

### Declarative Backup Management

Instead of manually configuring backup jobs through the VBR console, teams declare desired state in YAML files stored in Git:

```yaml
# infrastructure/jobs/database-backup.yaml
name: "Database Daily Backup"
description: "Production database backup - managed by owlctl"
isDisabled: false
storage:
  backupRepositoryId: "repo-uuid"
  retentionType: "Days"
  retentionPeriod: 30
  enableEncryption: true
schedule:
  runAutomatically: true
  daily:
    isEnabled: true
    localTime: "22:00"
```

This file is version-controlled, peer-reviewed through pull requests, and applied through CI/CD pipelines.

### The Full Lifecycle

| Stage | Command | What It Does |
|-------|---------|-------------|
| **Export** | `owlctl job export --all` | Captures current VBR configuration as YAML specs |
| **Snapshot** | `owlctl repo snapshot --all` | Records current state as the desired baseline |
| **Apply** | `owlctl job apply spec.yaml` | Applies desired configuration to VBR via API |
| **Diff** | `owlctl job diff --all` | Compares desired state against live VBR configuration |
| **Plan** | `owlctl job plan spec.yaml` | Previews changes without applying |

### Security-Aware Drift Detection

owlctl does not simply compare fields. It understands the security implications of changes:

| Change | Severity | Why |
|--------|----------|-----|
| Job disabled | **CRITICAL** | Directly weakens data protection |
| Encryption disabled | **CRITICAL** | Exposes backup data |
| Retention reduced | **CRITICAL** | Reduces recovery window |
| Schedule modified | **WARNING** | Weakens defense-in-depth |
| Job re-enabled | **WARNING** | Operational change, positive direction |
| Description changed | **INFO** | No security impact |

The system considers the *direction* of change, not just whether a field changed. Disabling encryption is CRITICAL. Enabling encryption is INFO. This value-aware severity classification is unique to owlctl -- no other backup automation tool provides this level of security intelligence.

### CI/CD-Ready Exit Codes

owlctl's exit codes map directly to Azure DevOps pipeline outcomes:

| Exit Code | Meaning | Pipeline Result |
|-----------|---------|----------------|
| `0` | No drift / success | Succeeded |
| `3` | Warning-level drift | SucceededWithIssues |
| `4` | Critical drift | Failed (blocks deployment) |
| `1` | Error | Failed (investigate) |
| `5` | Partial apply | SucceededWithIssues |
| `6` | Resource not found | Failed (manual intervention) |

These exit codes enable automated pipeline gates:

```yaml
# In an Azure DevOps pipeline
- script: |
    ./owlctl job diff --all --security-only
    EXIT_CODE=$?
    if [ $EXIT_CODE -eq 4 ]; then
      echo "##vso[task.logissue type=error]CRITICAL security drift detected"
      exit 1
    fi
  displayName: 'Security compliance gate'
```

### Azure DevOps Pipeline Templates

owlctl includes ready-to-use Azure DevOps pipeline templates:

| Template | Purpose |
|----------|---------|
| **detect-remediate.yml** | Scheduled drift detection with automatic remediation |
| **pr-validation.yml** | Validate configuration changes before merge |
| **deployment.yml** | Multi-stage deployment with approval gates |
| **nightly-compliance.yml** | Generate compliance reports attached to build summaries |

These templates follow the same patterns that teams already use for application deployments -- scheduled scans, PR gates, staged rollouts, and compliance reporting. The only difference is the target: backup infrastructure instead of application code.

### Supported Resources

| Resource | Export | Apply | Diff | Snapshot |
|----------|--------|-------|------|----------|
| Backup Jobs | Yes | Yes | Yes | Implicit |
| Repositories | Yes | Yes | Yes | Yes |
| Scale-Out Repositories | Yes | Yes | Yes | Yes |
| Encryption Passwords | Yes | Read-only | Yes | Yes |
| KMS Servers | Yes | Yes | Yes | Yes |
| Configuration Backup | Yes | Yes | Yes | Yes (singleton) |

---

## The Case for GitOps in Backup Infrastructure

### What GitOps Provides That GUIs Cannot

| Capability | GUI Console | GitOps with owlctl |
|-----------|-------------|------------------|
| **Change history** | Veeam audit log (limited detail) | Full Git history with diffs, commit messages, PR reviews |
| **Change approval** | Manual process / external tickets | Pull request reviews with required approvals |
| **Rollback** | Manual reconfiguration | `git revert` + `owlctl job apply` |
| **Multi-environment promotion** | Manual replication | YAML overlays: base config + environment-specific patches |
| **Drift detection** | Manual comparison | Automated scheduled scans with severity classification |
| **Compliance evidence** | Screenshots / manual documentation | Pipeline artifacts, build history, Git audit trail |
| **Disaster recovery of config** | Re-enter everything manually | `owlctl job apply` from Git-tracked specs |
| **Peer review** | Informal / verbal | Mandatory PR reviews before changes reach production |

### Real-World Scenarios

**Scenario 1: Someone disables encryption on a production backup job.**
- Without owlctl: Change goes unnoticed until the next audit (weeks or months).
- With owlctl: Nightly drift scan detects CRITICAL severity change, alerts the security team, and optionally auto-remediates by re-applying the desired state from Git.

**Scenario 2: A new environment needs the same backup configuration as production.**
- Without owlctl: Manually recreate every job, repository, and setting. Hope nothing is missed.
- With owlctl: Apply the same YAML specs with an environment-specific overlay. Identical configuration, guaranteed.

**Scenario 3: An audit requires evidence of backup configuration compliance.**
- Without owlctl: Manually document current settings. No history of changes.
- With owlctl: Git history shows every change, who approved it, and when. Pipeline artifacts show nightly compliance scan results.

---

## How This Compares

### owlctl vs. Other Backup Vendor Tools

| Capability | owlctl (Veeam) | Rubrik Terraform | Commvault Terraform | Cohesity Ansible |
|-----------|-------------|-----------------|--------------------|-----------------|
| **Declarative YAML specs** | Yes | HCL (Terraform) | HCL (Terraform) | YAML (Ansible) |
| **State management** | Yes (state.json) | Yes (tfstate) | Yes (tfstate) | No |
| **Drift detection** | Yes | `terraform plan` | `terraform plan` | No |
| **Security-aware severity** | Yes (CRITICAL/WARNING/INFO) | No | No | No |
| **Value-aware classification** | Yes (direction of change matters) | No | No | No |
| **Cross-resource validation** | Yes (e.g., hardened repo detection) | No | No | No |
| **CI/CD exit codes** | Yes (0/3/4/5/6) | Yes (0/1/2) | Yes (0/1/2) | No |
| **Configuration overlays** | Yes | Terraform workspaces | Terraform workspaces | Variable files |
| **Purpose-built for backup** | Yes | Partially | Partially | Partially |

owlctl's security-aware drift detection -- understanding that disabling encryption is CRITICAL while enabling it is INFO, or that moving a job off a hardened repository is more significant than changing a description -- is unique in the backup automation space. Generic IaC tools like Terraform detect that a field changed but cannot classify the security implications.

---

## Getting Started

### Prerequisites

- Veeam Backup & Replication v11+ with REST API enabled (port 9419)
- Azure DevOps organisation with a self-hosted or Microsoft-hosted agent
- VBR API credentials (username, password, server URL)

### Day 1: Establish Baseline

```bash
# Install owlctl
curl -sL https://github.com/shapedthought/owlctl/releases/latest/download/owlctl-linux-amd64 -o owlctl
chmod +x owlctl

# Authenticate
./owlctl init && ./owlctl profile --set vbr && ./owlctl login

# Export current VBR configuration as YAML
./owlctl job export --all -d infrastructure/jobs/
./owlctl repo export --all -d infrastructure/repos/
./owlctl repo sobr-export --all -d infrastructure/sobrs/
./owlctl encryption export --all -d infrastructure/encryption/
./owlctl encryption kms-export --all -d infrastructure/kms/

# Snapshot current state as desired baseline
./owlctl repo snapshot --all
./owlctl repo sobr-snapshot --all
./owlctl encryption snapshot --all
./owlctl encryption kms-snapshot --all
./owlctl config-backup snapshot          # Singleton — no --all needed

# Commit to Git
git add infrastructure/ state.json
git commit -m "Bootstrap VBR declarative management"
git push
```

### Day 2: Enable Automated Drift Detection

Import the `detect-remediate.yml` pipeline template from `examples/pipelines/` into your Azure DevOps project. Configure a variable group with VBR credentials. The pipeline runs on a schedule and alerts on security-relevant drift.

### Day 3+: Manage Changes Through Git

All future changes flow through pull requests:

1. Modify a YAML spec in a feature branch
2. Open a pull request -- PR validation pipeline runs `--dry-run`
3. Peer review and approve
4. Merge to master -- deployment pipeline applies changes to VBR
5. Post-deployment verification confirms no drift

This is the same workflow teams already use for application code and infrastructure-as-code. The only new element is the target: Veeam backup infrastructure.

---

## Summary

Using APIs to manage infrastructure declaratively is not new or experimental. It is how modern organisations operate. Kubernetes, Terraform, Helm, ArgoCD, and Ansible are all open-source tools that wrap vendor APIs to enable version-controlled, peer-reviewed, automated infrastructure management. Every major backup vendor -- Rubrik, Commvault, Cohesity, Veritas -- has community-driven open-source tooling built on their APIs.

owlctl extends this established pattern to Veeam Backup & Replication. It uses Veeam's publicly documented REST APIs to bring GitOps workflows to backup infrastructure, with first-class Azure DevOps integration. It adds domain-specific intelligence -- security-aware drift detection, value-aware severity classification, cross-resource validation -- that generic IaC tools cannot provide.

The question is not whether backup infrastructure should be managed through APIs and GitOps. The rest of the industry has already answered that. The question is whether your organisation is ready to apply the same rigour to backup configuration that it already applies to application code and cloud infrastructure.

---

## References

### Veeam API Documentation
- [VBR REST API Overview](https://helpcenter.veeam.com/docs/backup/vbr_rest/overview.html)
- [KB4311: Veeam API Integration Guide](https://www.veeam.com/kb4311)
- [Unleashing Modern Automation with RESTful APIs in V11](https://www.veeam.com/blog/v11-modern-automation-with-restful.html)
- [Managing Backups as Code -- Veeam with HashiCorp Terraform](https://www.veeam.com/blog/veeam-hashicorp-terraform.html)

### Community Ecosystem
- [VeeamHub GitHub](https://github.com/VeeamHub) -- 39+ community repositories
- [Rubrik GitHub](https://github.com/rubrikinc) -- SDKs and Terraform providers
- [Commvault GitHub](https://github.com/Commvault) -- Terraform provider and Ansible collection
- [Cohesity GitHub](https://github.com/cohesity) -- SDKs, Ansible role, Terraform provider
- [VeritasOS GitHub](https://github.com/VeritasOS) -- NetBackup Ansible and API samples

### Open-Source Infrastructure Tools
- [Kubernetes](https://github.com/kubernetes/kubernetes) -- Apache 2.0, ~120k stars, CNCF graduated
- [Terraform](https://github.com/hashicorp/terraform) -- BSL 1.1, ~43k stars
- [OpenTofu](https://github.com/opentofu/opentofu) -- MPL 2.0, ~20k stars, Linux Foundation
- [Helm](https://github.com/helm/helm) -- Apache 2.0, ~29k stars, CNCF graduated
- [ArgoCD](https://github.com/argoproj/argo-cd) -- Apache 2.0, ~22k stars, CNCF graduated
- [Ansible](https://github.com/ansible/ansible) -- GPL 3.0, ~63k stars
- [Crossplane](https://github.com/crossplane/crossplane) -- Apache 2.0, ~9k stars, CNCF graduated

### owlctl
- [owlctl GitHub Repository](https://github.com/shapedthought/owlctl)
- [Azure DevOps Integration Guide](./azure-devops-integration.md)
- [Drift Detection Guide](./drift-detection.md)
- [Security Alerting Reference](./security-alerting.md)
- [Pipeline Templates](../examples/pipelines/)
