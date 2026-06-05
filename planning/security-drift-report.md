# Security Configuration Drift Analysis Report

**Product**: owlctl - Veeam CLI Configuration Management Tool
**Date**: February 2026
**Author**: Product Management
**Classification**: Internal - Strategic Planning

---

## Executive Summary

Customers have raised concerns about security-related configuration drift in their Veeam Backup & Replication (VBR) environments. The core risk is that security-critical settings -- such as repository immutability, backup encryption, and access controls -- may be gradually or silently changed over time, weakening an organization's ransomware resilience and compliance posture without detection.

owlctl currently provides state management and drift detection for VBR backup jobs. This report identifies all security-relevant VBR configuration areas that should be brought under state management, assesses their risk profile, evaluates API feasibility, and proposes a prioritized implementation roadmap.

---

## Customer Problem Statement

> "Security-related configurations in our Veeam environment are being changed over time without anyone noticing. Things like repository immutability settings, encryption policies, and retention periods can be silently modified -- either through human error, insider threats, or compromised accounts. By the time we discover the change, our environment may have been vulnerable for weeks or months."

### Attack Scenario

A typical ransomware attack against backup infrastructure follows this pattern:

1. **Reconnaissance**: Attacker gains access and identifies backup infrastructure
2. **Credential Escalation**: Obtains Veeam administrator privileges
3. **Silent Weakening**: Disables immutability, shortens retention, removes encryption
4. **Dwell Time**: Waits for clean backups to age out under shortened retention
5. **Execution**: Deploys ransomware, encrypts production, and deletes now-mutable backups

Steps 3 and 4 are the window where configuration drift detection would alert defenders before the attack reaches execution. The longer drift goes undetected, the more vulnerable the environment becomes.

---

## Security Configuration Categories

### Category 1: Backup Repository Security

**Risk Level: CRITICAL**

Repository configuration is the single most important security surface in a Veeam environment. If an attacker can modify repository settings, they can make backups deletable, disable immutability, or redirect backups to compromised storage.

#### 1.1 Repository Immutability Settings

| Aspect | Detail |
|--------|--------|
| **What** | Hardened repository immutability period (7-9999 days) |
| **Risk if drifted** | Backups become deletable by ransomware or malicious insiders |
| **How it could drift** | Admin disables for "maintenance", attacker modifies via API or console |
| **Detection priority** | P0 - Must detect within hours |
| **API endpoint** | `GET/PUT /api/v1/backupInfrastructure/repositories/{id}` |
| **API feasibility** | HIGH - Repository settings are fully readable via API |

Immutability is the last line of defense. According to Veeam's 2025 Ransomware Trends Report, 69% of organizations experienced at least one ransomware attack, and attackers increasingly target backup repositories directly. Drift from the intended immutability period -- even by a single day -- should trigger an alert.

#### 1.2 Repository Type and Configuration

| Aspect | Detail |
|--------|--------|
| **What** | Repository type (hardened Linux, Windows, NFS, SMB, object storage), path, max concurrent tasks |
| **Risk if drifted** | Repository swapped to non-hardened type, storage path changed to attacker-controlled location |
| **Detection priority** | P0 |
| **API endpoint** | `GET /api/v1/backupInfrastructure/repositories/{id}` |
| **API feasibility** | HIGH |

A repository type change from hardened Linux to standard Windows, for example, would silently remove all immutability guarantees.

#### 1.3 Scale-Out Repository Configuration

| Aspect | Detail |
|--------|--------|
| **What** | Extent membership, sealed mode, capacity tier settings, immutability on object storage |
| **Risk if drifted** | Extents removed, sealed mode disabled, capacity tier immutability changed |
| **Detection priority** | P0 |
| **API endpoint** | `GET/PUT /api/v1/backupInfrastructure/scaleout-repositories/{id}` |
| **API feasibility** | HIGH - Includes sealed mode enable/disable endpoints |

Scale-out repositories add complexity because security settings exist at both the SOBR level and individual extent level. Object storage extents may have their own immutability policies (e.g., S3 Object Lock) that must also be monitored.

---

### Category 2: Encryption Configuration

**Risk Level: CRITICAL**

Encryption protects backup data at rest. If encryption is disabled or encryption passwords are changed/deleted, backup data becomes readable to attackers with storage access, and recovery may become impossible.

#### 2.1 Encryption Passwords

| Aspect | Detail |
|--------|--------|
| **What** | Existence and status of encryption passwords used by backup jobs |
| **Risk if drifted** | Password deleted or changed, making encrypted backups unrecoverable |
| **Detection priority** | P0 |
| **API endpoint** | `GET /api/v1/encryptionPasswords` |
| **API feasibility** | HIGH - Can enumerate passwords (not retrieve actual values) |

State management should track the set of encryption password IDs and their associated hints/tags. If a password referenced by active backup jobs is deleted or changed, this is a critical security event.

#### 2.2 KMS Server Configuration

| Aspect | Detail |
|--------|--------|
| **What** | Key Management System server connections, certificates |
| **Risk if drifted** | KMS server removed or reconfigured, breaking key retrieval |
| **Detection priority** | P1 |
| **API endpoint** | `GET /api/v1/kmsServers` |
| **API feasibility** | HIGH |

Organizations using external KMS for encryption key management need assurance that KMS server configuration has not been tampered with.

#### 2.3 Job-Level Encryption Settings

| Aspect | Detail |
|--------|--------|
| **What** | Per-job encryption enabled/disabled flag and password ID reference |
| **Risk if drifted** | Encryption silently disabled on a job, new backups stored in plaintext |
| **Detection priority** | P0 |
| **API endpoint** | Already available via job spec: `storage.advancedSettings.storageData.encryption` |
| **API feasibility** | HIGH - Already in existing job state management |

This is partially covered by existing job drift detection but should be explicitly flagged as a security-critical field with elevated alerting.

---

### Category 3: Access Control and Credentials

**Risk Level: HIGH**

Credential compromise is the primary vector for backup infrastructure attacks. Monitoring credential configuration prevents both insider threats and external attackers from establishing persistent access.

#### 3.1 Users and Roles (RBAC)

| Aspect | Detail |
|--------|--------|
| **What** | User accounts, role assignments (Backup Administrator, Security Administrator, Restore Operator, etc.) |
| **Risk if drifted** | Privilege escalation, unauthorized admin accounts created, security roles weakened |
| **Detection priority** | P0 |
| **API endpoint** | Users and Roles section in VBR REST API |
| **API feasibility** | HIGH |

New admin accounts appearing, role changes, or the removal of separation-of-duties controls (e.g., merging Backup Admin and Security Admin roles) are high-priority security events.

#### 3.2 Credentials Records

| Aspect | Detail |
|--------|--------|
| **What** | Stored credentials for managed servers, guest processing, proxies |
| **Risk if drifted** | Credentials replaced with attacker-controlled accounts, credential records deleted breaking operations |
| **Detection priority** | P1 |
| **API endpoint** | `GET /api/v1/credentials` |
| **API feasibility** | MEDIUM - Can enumerate credentials (IDs, types, usernames) but not passwords |

State management should track the inventory of credential records (ID, username, type) to detect unauthorized additions, removals, or username changes.

#### 3.3 TLS Certificates and SSH Fingerprints

| Aspect | Detail |
|--------|--------|
| **What** | TLS certificates for API and component communication, SSH fingerprints for Linux servers |
| **Risk if drifted** | Man-in-the-middle attacks, compromised communication channels |
| **Detection priority** | P1 |
| **API endpoint** | Connection section in VBR REST API |
| **API feasibility** | MEDIUM |

Certificate changes should be infrequent and planned. Any unexpected certificate change warrants investigation.

---

### Category 4: Backup Job Security Settings

**Risk Level: HIGH**

Beyond the job-level settings already under state management, specific security-relevant fields require elevated monitoring and dedicated alerting.

#### 4.1 Retention Policy

| Aspect | Detail |
|--------|--------|
| **What** | Retention period (days or restore points), GFS policy |
| **Risk if drifted** | Shortened retention allows clean backups to expire, leaving only compromised ones |
| **Detection priority** | P0 |
| **API endpoint** | Already in job spec: `storage.retentionPolicy` |
| **API feasibility** | HIGH - Already in existing state management |

Retention reduction is one of the most common pre-ransomware attack modifications. A change from 30 days to 3 days gives the attacker a much shorter wait before all clean recovery points are gone.

#### 4.2 Job Enabled/Disabled Status

| Aspect | Detail |
|--------|--------|
| **What** | Whether the backup job is enabled and scheduled to run |
| **Risk if drifted** | Jobs silently disabled, creating gaps in backup coverage |
| **Detection priority** | P0 |
| **API endpoint** | Already in job spec: `isDisabled` |
| **API feasibility** | HIGH - Already in existing state management |

#### 4.3 Schedule Integrity

| Aspect | Detail |
|--------|--------|
| **What** | Backup schedule, run-automatically flag, retry settings |
| **Risk if drifted** | Schedule changed to reduce backup frequency, retries disabled |
| **Detection priority** | P1 |
| **API endpoint** | Already in job spec: `schedule` |
| **API feasibility** | HIGH - Already in existing state management |

#### 4.4 Guest Processing and Application-Aware Settings

| Aspect | Detail |
|--------|--------|
| **What** | VSS processing, application-aware backup for databases and applications |
| **Risk if drifted** | Application-consistent backups disabled, resulting in crash-consistent only -- databases may be unrecoverable |
| **Detection priority** | P1 |
| **API endpoint** | Already in job spec: `guestProcessing.appAwareProcessing` |
| **API feasibility** | HIGH - Already in existing state management |

#### 4.5 Backup Target Repository Assignment

| Aspect | Detail |
|--------|--------|
| **What** | Which repository a job writes to |
| **Risk if drifted** | Job redirected from hardened/immutable repository to standard repository |
| **Detection priority** | P0 |
| **API endpoint** | Already in job spec: `storage.backupRepositoryId` |
| **API feasibility** | HIGH - Already in existing state management |

This is particularly dangerous when combined with repository drift. An attacker could create a non-hardened repository and redirect jobs to it.

---

### Category 5: Infrastructure Security

**Risk Level: HIGH**

The backup infrastructure components themselves must be monitored to ensure managed servers, proxies, and network configurations haven't been tampered with.

#### 5.1 Managed Servers Inventory

| Aspect | Detail |
|--------|--------|
| **What** | List of managed servers (vSphere, Linux, Windows), their connection settings |
| **Risk if drifted** | Rogue servers added, legitimate servers removed, connection settings changed |
| **Detection priority** | P1 |
| **API endpoint** | `GET /api/v1/backupInfrastructure/managedServers` |
| **API feasibility** | HIGH |

An unauthorized managed server could be used as a pivot point or to inject malicious data into the backup chain.

#### 5.2 Backup Proxy Configuration

| Aspect | Detail |
|--------|--------|
| **What** | Proxy server inventory, transport mode, max concurrent tasks |
| **Risk if drifted** | Rogue proxy added (data exfiltration vector), proxy removed (performance degradation hiding attack) |
| **Detection priority** | P2 |
| **API endpoint** | `GET /api/v1/backupInfrastructure/proxies` |
| **API feasibility** | HIGH |

#### 5.3 Traffic Rules

| Aspect | Detail |
|--------|--------|
| **What** | Network traffic encryption and throttling rules between components |
| **Risk if drifted** | Traffic encryption disabled, allowing network-level data interception |
| **Detection priority** | P1 |
| **API endpoint** | Traffic Rules section in VBR REST API |
| **API feasibility** | HIGH |

---

### Category 6: Security and Compliance Features

**Risk Level: HIGH**

VBR 13 introduced dedicated security features that must themselves be protected from drift.

#### 6.1 Security & Compliance Analyzer Settings

| Aspect | Detail |
|--------|--------|
| **What** | Which compliance checks are enabled, their pass/fail thresholds |
| **Risk if drifted** | Compliance checks disabled, masking other security drift |
| **Detection priority** | P1 |
| **API endpoint** | Security section in VBR REST API |
| **API feasibility** | MEDIUM - Needs investigation of specific response models |

If the compliance analyzer itself is disabled, it creates a blind spot that could hide other security changes.

#### 6.2 Malware Detection Configuration

| Aspect | Detail |
|--------|--------|
| **What** | Malware scanning settings, YARA rule configuration, threat detection sensitivity |
| **Risk if drifted** | Malware scanning disabled, allowing compromised backups to be used for restore |
| **Detection priority** | P1 |
| **API endpoint** | Malware Detection section in VBR REST API |
| **API feasibility** | HIGH |

#### 6.3 Configuration Backup Settings

| Aspect | Detail |
|--------|--------|
| **What** | VBR configuration backup schedule, encryption, target location |
| **Risk if drifted** | Config backup disabled, leaving no recovery path for the backup server itself |
| **Detection priority** | P1 |
| **API endpoint** | Configuration Backup section in VBR REST API |
| **API feasibility** | HIGH |

---

### Category 7: Notification and Audit Configuration

**Risk Level: MEDIUM**

Notification settings are a secondary security concern but are important because disabling them can hide other security events.

#### 7.1 Email Notification Settings

| Aspect | Detail |
|--------|--------|
| **What** | SMTP server, notification recipients, which events trigger notifications |
| **Risk if drifted** | Notifications disabled, security alerts suppressed |
| **Detection priority** | P2 |
| **API endpoint** | General Options section in VBR REST API |
| **API feasibility** | HIGH |

#### 7.2 SNMP and Event Forwarding

| Aspect | Detail |
|--------|--------|
| **What** | SNMP trap destinations, syslog forwarding, SIEM integration |
| **Risk if drifted** | Audit trail broken, security events not forwarded to SIEM |
| **Detection priority** | P2 |
| **API endpoint** | General Options section in VBR REST API |
| **API feasibility** | HIGH |

#### 7.3 Global Exclusions

| Aspect | Detail |
|--------|--------|
| **What** | VM exclusion patterns applied globally to all jobs |
| **Risk if drifted** | Critical VMs added to exclusion list, silently dropping them from all backups |
| **Detection priority** | P1 |
| **API endpoint** | Global Exclusions section in VBR REST API |
| **API feasibility** | HIGH |

---

## Prioritized Implementation Roadmap

### Phase 1: Critical Security Drift (Recommended Next)

These items have the highest security impact and the best API feasibility. They represent the configurations most likely to be targeted in a pre-ransomware attack.

| # | Resource Type | Key Fields | API Endpoint | Effort |
|---|---------------|------------|--------------|--------|
| 1 | Backup Repositories | Immutability period, type, path, settings | `/api/v1/backupInfrastructure/repositories` | Medium |
| 2 | Scale-Out Repositories | Extents, sealed mode, capacity tier | `/api/v1/backupInfrastructure/scaleout-repositories` | Medium |
| 3 | Encryption Passwords | Password inventory (IDs, hints) | `/api/v1/encryptionPasswords` | Low |
| 4 | Job Encryption (enhancement) | Elevated alerting for encryption.isEnabled changes | Existing job state | Low |
| 5 | Job Retention (enhancement) | Elevated alerting for retention reductions | Existing job state | Low |
| 6 | Job Target Repository (enhancement) | Alert if job moved off hardened repo | Existing job state + repo lookup | Low |

**Estimated scope**: New `VBRRepository` and `VBRScaleOutRepository` resource types in state management, plus security-aware alerting layer on existing job drift detection.

### Phase 2: Access Control and Infrastructure

| # | Resource Type | Key Fields | API Endpoint | Effort |
|---|---------------|------------|--------------|--------|
| 7 | Users and Roles | User inventory, role assignments | Users and Roles API | Medium |
| 8 | Managed Servers | Server inventory, connection settings | `/api/v1/backupInfrastructure/managedServers` | Medium |
| 9 | Credentials | Credential inventory (IDs, types, usernames) | `/api/v1/credentials` | Low |
| 10 | KMS Servers | KMS server inventory and configuration | `/api/v1/kmsServers` | Low |
| 11 | Traffic Rules | Network encryption settings | Traffic Rules API | Low |

### Phase 3: Security Features and Monitoring

| # | Resource Type | Key Fields | API Endpoint | Effort |
|---|---------------|------------|--------------|--------|
| 12 | Malware Detection | Scan settings, YARA rules | Malware Detection API | Medium |
| 13 | Configuration Backup | Schedule, encryption, target | Configuration Backup API | Low |
| 14 | Global Exclusions | Excluded VM patterns | Global Exclusions API | Low |
| 15 | Notification Settings | SMTP, SNMP, event forwarding | General Options API | Low |
| 16 | Compliance Analyzer | Enabled checks, thresholds | Security API | Medium |

### Phase 4: Enhanced Security Intelligence

| # | Capability | Description | Effort |
|---|-----------|-------------|--------|
| 17 | Security Severity Levels | Classify drift as INFO/WARNING/CRITICAL based on security impact | Medium |
| 18 | Correlation Engine | Detect multi-resource attack patterns (e.g., immutability disabled + retention shortened + encryption removed) | High |
| 19 | Webhook/SIEM Integration | Push security drift alerts to external systems | Medium |
| 20 | Compliance Report Generation | Produce audit-ready reports showing security posture over time | Medium |

---

## Security Drift Severity Classification

Not all drift is equal. The following classification should be applied to drift detection output to help operations teams prioritize response:

### CRITICAL (Immediate Action Required)

Changes that directly weaken ransomware resilience or data protection:

- Repository immutability period reduced or disabled
- Backup encryption disabled on any job
- Encryption password deleted while referenced by active jobs
- Job target repository changed from hardened to non-hardened
- Retention policy reduced below organizational minimum
- Backup job disabled
- Admin account created or role escalated
- Configuration backup disabled

### WARNING (Investigate Within 24 Hours)

Changes that weaken defense-in-depth or monitoring:

- KMS server configuration changed
- Guest processing/application-aware backup disabled
- Managed server added or removed
- Credential records changed
- Traffic encryption rules modified
- Malware detection settings changed
- Backup schedule frequency reduced
- Global exclusion rules modified

### INFO (Review During Next Audit)

Changes that affect operations but have lower direct security impact:

- Notification settings changed
- Proxy configuration changed
- Backup window modified
- Compression or deduplication settings changed
- Script pre/post commands modified

---

## Risk Assessment Summary

| Category | Current Coverage | Risk Without Coverage | Implementation Complexity |
|----------|-----------------|----------------------|--------------------------|
| Repository Immutability | None | **CRITICAL** - Primary ransomware defense | Medium |
| Encryption Settings | Partial (job-level only) | **CRITICAL** - Data at rest exposure | Low-Medium |
| Access Control (RBAC) | None | **HIGH** - Insider threat / privilege escalation | Medium |
| Credential Management | None | **HIGH** - Unauthorized access vector | Low |
| Job Security Settings | Covered (basic drift) | **MEDIUM** - Needs severity classification | Low (enhancement) |
| Infrastructure Inventory | None | **HIGH** - Rogue component detection | Medium |
| Security Features | None | **HIGH** - Defense-in-depth degradation | Medium |
| Notification/Audit | None | **MEDIUM** - Alert suppression | Low |

---

## Competitive Context

### How Other IaC Tools Handle Security Drift

**Terraform** tracks state but does not differentiate security-critical fields from others. All drift is treated equally. Security teams must build external tooling (Sentinel policies, OPA rules) to classify severity.

**Ansible** is stateless by design and cannot detect drift at all without additional tooling (ansible-cmdb, custom fact scripts).

**Pulumi** offers policy-as-code (CrossGuard) for preventing insecure configurations but does not have built-in severity classification for drift.

**owlctl opportunity**: By building security-aware drift detection with built-in severity classification, owlctl would offer a differentiated capability that none of the major IaC tools provide out of the box for backup infrastructure. This directly addresses the customer concern without requiring external policy engines.

---

## Recommendations

1. **Prioritize repository immutability state management** -- This is the single highest-impact security feature and directly addresses the customer concern. It should be the immediate next work item.

2. **Introduce security severity levels in drift output** -- Modify the existing `owlctl job diff` output to classify fields as CRITICAL/WARNING/INFO. This is a low-effort enhancement to the existing system.

3. **Add repository and encryption resources to state management** -- These are the Phase 1 items with the best risk-to-effort ratio.

4. **Design for CI/CD security gates** -- Ensure drift detection exit codes and output formats support automated security pipelines (e.g., fail a pipeline if CRITICAL drift is detected).

5. **Document the security monitoring workflow** -- Provide customers with a clear guide for setting up scheduled drift detection with alerting, complementing VBR's built-in Security & Compliance Analyzer.

6. **Consider SIEM/webhook integration early** -- Even if not implemented immediately, design the drift detection output to be easily consumed by external security tools.

---

## Appendix A: VBR REST API Endpoint Coverage

The following VBR REST API v1.3-rev1 sections are relevant to security state management:

| API Section | Security Relevance | Current owlctl Support |
|-------------|-------------------|---------------------|
| Jobs | Job-level encryption, retention, scheduling | Yes (state + drift) |
| Repositories | Immutability, repository type, settings | Read only (no state) |
| Scale-Out Repositories | Sealed mode, extent membership | No |
| Encryption | Password inventory, KMS servers | No |
| Credentials | Credential records inventory | No |
| Users and Roles | RBAC configuration | No |
| Security | Compliance analyzer settings | No |
| Malware Detection | Scan configuration, YARA rules | No |
| Configuration Backup | Config backup schedule and encryption | No |
| General Options | Notifications, event forwarding | No |
| Traffic Rules | Network encryption | No |
| Global Exclusions | VM exclusion patterns | No |
| Connection | TLS certificates, SSH fingerprints | No |
| Managed Servers | Infrastructure server inventory | Read only (no state) |

## Appendix B: References

- [Veeam Security Best Practice Guide - Protect](https://bp.veeam.com/security/Design-and-implementation/Protect.html)
- [Veeam Hardened Repository Documentation](https://helpcenter.veeam.com/docs/vbr/userguide/hardened_repository.html)
- [Veeam Immutability Overview](https://www.veeam.com/blog/veeam-immutability-everything-you-need-to-know.html)
- [Veeam Technical Guide to Ransomware Protection](https://www.veeam.com/blog/guide-to-ransomware-protection.html)
- [Securing Backup Infrastructure - VBR User Guide](https://helpcenter.veeam.com/docs/vbr/userguide/securing_backup_infrastructure.html)
- [VBR 13 REST API Reference (v1.3-rev1)](https://helpcenter.veeam.com/references/vbr/13/rest/1.3-rev1/tag/SectionOverview/index.html)
- [VBR 13 REST API - Encryption Endpoints](https://helpcenter.veeam.com/references/vbr/13/rest/1.3-rev0/tag/Encryption/index.html)
- [VBR 13 Security & Anti-Ransomware Features](https://vinfrastructure.it/2025/11/security-and-anti-ransomware-features-in-veeam-backup-replication-13/)
