# Azure DevOps Integration — Features Needed

These owlctl features would complete the Azure DevOps integration story. They are ordered by impact.

## 1. Declarative Management for All Resources (High Impact)

**Status:** Complete (Epic #42)

The `export` → `snapshot` → `apply` → `diff` lifecycle is now implemented for all resource types (jobs, repositories, SOBRs, encryption passwords, KMS servers). This enables automated remediation pipelines. See [pipeline templates](../../examples/pipelines/) for ready-to-use examples.

## 2. JSON Output for Diff Commands (High Impact)

**Status:** Not implemented
**Related issue:** Part of #37 (structured output)

Add `--output json` flag to all diff commands. The JSON schema should include:

```json
{
  "timestamp": "2026-02-01T06:00:00Z",
  "resource": "Backup Job 1",
  "resourceType": "VBRJob",
  "origin": "applied",
  "driftCount": 3,
  "highestSeverity": "CRITICAL",
  "securityDriftCount": 2,
  "drifts": [
    {
      "path": "isDisabled",
      "action": "modified",
      "stateValue": false,
      "vbrValue": true,
      "severity": "CRITICAL"
    }
  ],
  "metadata": {
    "lastApplied": "2026-02-01T14:30:00Z",
    "lastAppliedBy": "edwardhoward"
  }
}
```

**Why it matters:** Enables pipeline scripts to parse results with `jq`, build conditional logic, and forward structured data to external systems. The current text output requires fragile regex parsing.

## 3. Markdown Report Generation (High Impact)

**Status:** Planned as #38 (compliance report generation)

A `owlctl report` command (or `--output markdown` on diff commands) that produces a formatted Markdown report. Azure DevOps renders Markdown in build summaries via the `##vso[task.uploadsummary]` logging command, making drift results visible without downloading artifacts.

## 4. Unified Scan Command (Medium Impact)

**Status:** Not planned

A single command that runs all diff checks and produces a combined report:

```bash
owlctl scan --all --security-only --output json
```

This simplifies pipeline definitions from 5+ sequential commands to one. It could also enable the correlation engine (#36) to analyse cross-resource patterns.

## 5. JUnit XML Output (Medium Impact)

**Status:** Not planned

Azure DevOps has a built-in "Tests" tab that displays JUnit XML results with pass/fail counts, trends, and drill-down. If owlctl outputs drift results in JUnit format, each drift becomes a "test case":

```xml
<testsuites>
  <testsuite name="Job Drift" tests="3" failures="2">
    <testcase name="Backup Job 1 - isDisabled" classname="job.drift">
      <failure message="CRITICAL: false (state) -> true (VBR)" />
    </testcase>
  </testsuite>
</testsuites>
```

This gives free trending, history, and dashboard widgets via the `PublishTestResults@2` task.

## 6. SARIF Output (Medium Impact)

**Status:** Not planned

The Static Analysis Results Interchange Format (SARIF) is a standard for security tool output. Azure DevOps has a "Scans" tab (via the SARIF SAST extension) that displays SARIF results alongside code. If drift results map to config files tracked in Git, SARIF output would show drifts annotated on the relevant lines.

## 7. Azure DevOps Logging Command Support (Low Impact)

**Status:** Not planned

An `--ado` flag that wraps output with Azure DevOps logging commands automatically:

```bash
./owlctl job diff --all --ado
```

Would output:
```
##vso[task.logissue type=error]CRITICAL ~ isDisabled: false (state) -> true (VBR)
##vso[task.logissue type=warning]WARNING ~ schedule: modified
##vso[task.complete result=SucceededWithIssues;]2 security-relevant changes detected
```

This is a convenience — the same result can be achieved with shell scripting around the current text output, but native support reduces boilerplate.

## 8. Non-Interactive Mode Flag (Low Impact)

**Status:** Partially implemented (owlctl already uses env vars for auth)

An explicit `--non-interactive` or `--ci` flag that:
- Suppresses any interactive prompts
- Ensures clean stdout (no progress spinners or ANSI codes)
- Guarantees exit codes are set correctly
- Reads all configuration from environment variables or flags only

owlctl largely behaves this way already, but an explicit flag documents the intent and prevents future regressions.

---

## Feature Priority Matrix

| Feature | Azure DevOps Impact | Effort | Dependency |
|---------|-------------------|--------|------------|
| Declarative management (all resources) | Unlocks automated remediation pipelines | High | Epic #42 |
| JSON output for diffs | Unlocks programmatic parsing, webhook forwarding, dashboard data | Medium | None |
| Markdown report | Build summary integration, wiki dashboards, compliance artifacts | Medium | #38 |
| Unified scan command | Simplifies pipelines, enables correlation | Medium | #36 |
| JUnit XML output | Free trending/history via Tests tab | Low | JSON output |
| SARIF output | Security scan tab integration | Low | JSON output |
| ADO logging commands | Convenience for pipeline authors | Low | None |
| Non-interactive flag | CI/CD safety net | Low | None |
