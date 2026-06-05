# Agent Governance Layer: Bridging AI Agents and Enterprise Systems

**Status:** Internal strategy document
**Date:** February 2026
**Updated:** February 2026 — Portunus integration, market landscape, resolved open questions
**Context:** owlctl + Portunus + MCP + OpenClaw / AI agent integration

---

## The Problem

AI agents (OpenClaw, Claude, Copilot, custom MCP clients) can now execute real actions on enterprise infrastructure through tools like owlctl. They can modify backup job configurations, change retention policies, disable encryption, and alter repository settings — all through natural language commands or autonomous decision-making.

The technology works. The governance does not exist.

Today the gap is filled by CI/CD pipelines (Azure DevOps branch policies, environment approval gates, policy-rules.yaml checked by yq). This is process-based governance — it works when humans follow the process, but it provides no enforcement when an AI agent calls `owlctl job apply` directly via shell access or MCP.

The core question: **How do you let AI agents operate enterprise systems with the same rigour that enterprise teams expect from human operators?**

This is not a theoretical concern. The moment an AI agent has shell access (OpenClaw) or tool access (MCP) to owlctl, every governance control that depends on "the human will follow the process" becomes irrelevant.

---

## What Exists Today

### Existing Frameworks and Where They Fall Short

| Framework | What It Covers | What It Misses |
|-----------|---------------|----------------|
| **NIST AI RMF** | Organizational AI risk management | Action-level enforcement — tells you what governance *should* exist, not how to enforce it at the execution boundary |
| **NIST Zero Trust (SP 800-207)** | Never trust, always verify for network/resource access | Not designed for AI agent intent evaluation — an agent can have valid credentials but make a policy-violating decision |
| **OWASP Top 10 for LLMs** | Identifies "Excessive Agency" as a risk (LLM-08) | Frames it as a vulnerability to mitigate, not an architecture to build |
| **OWASP Agentic AI Threats** | Threat taxonomy for multi-agent systems | Identifies threats but doesn't prescribe the governance architecture |
| **Gartner AI TRiSM** | Market category for AI trust/risk/security | Focused on model behaviour (bias, hallucination), not on action-level governance for agentic AI |
| **OPA (Open Policy Agent)** | Policy-as-code for cloud-native systems | Mature engine, but no canonical pattern for evaluating AI agent tool calls |
| **MCP (Model Context Protocol)** | Standardised agent-to-tool interface | Defines the interface but explicitly does not define governance — servers handle auth ad hoc |
| **NeMo Guardrails** | Programmable rails for LLM applications | Designed for conversational flows, not for governing tool calls against infrastructure APIs |
| **LangChain/LangGraph** | Basic tool permissions, human-approval nodes | Framework-specific, not enterprise-grade |

### The Specific Gap

Every component of a governance solution exists in isolation:

- **Policy engines** exist (OPA, Cerbos, Permit.io) — mature, battle-tested in cloud-native
- **Tool interfaces** exist (MCP) — standardised, gaining adoption
- **Agent frameworks** exist (OpenClaw, LangGraph, CrewAI) — functional, growing
- **Threat models** exist (OWASP) — comprehensive for awareness

The gap has narrowed significantly since early 2025. Generic MCP governance gateways are now entering the market (see Market Landscape below). However, **domain-specific governed MCP servers for enterprise infrastructure** — where the governance layer understands the semantics of the system it's protecting — remain wide open.

---

## Market Landscape (February 2026)

The MCP governance gateway space has moved rapidly. Several products now address parts of the architecture described in this document. None cover the full picture for domain-specific infrastructure like backup and data protection.

### Generic MCP Gateways and Identity Layers

**Gravitee AI Agent Management (v4.10)**

The most complete generic offering. Gravitee's [MCP Proxy](https://www.gravitee.io/blog/mcp-proxy-unified-governance-for-agents-tools) is protocol-native — it inspects MCP payloads to understand which tools, methods, and prompts are being invoked. Key capabilities:

- MCP-level request inspection (not just HTTP method/endpoint)
- Rate limiting, traffic shaping, and usage quotas per agent
- Full audit logging of all tool invocations with chain-of-thought visibility
- "AI IAM" — treats MCP as a first-class identity concern
- LLM proxy for provider routing and cost management

Gravitee is a commercial API management platform. Their [State of AI Agent Security 2026](https://www.gravitee.io/blog/state-of-ai-agent-security-2026-report-when-adoption-outpaces-control) report found only 14.4% of organisations report all AI agents going live with full security/IT approval.

**Strata Maverics AI Identity Gateway**

Focused specifically on [identity federation for MCP servers](https://www.strata.io/agentic-identity-sandbox/securing-mcp-servers-at-scale-how-to-govern-ai-agents-with-an-enterprise-identity-fabric/). Rather than building a full proxy, Strata federates MCP servers through enterprise identity infrastructure (OAuth/OIDC, SAML, Entra ID). Key capabilities:

- Short-lived, scoped credentials issued at runtime (agents never hold long-lived tokens)
- Policy-as-code authorization with human-in-the-loop approval for sensitive actions
- Every agent decision and MCP-initiated API call logged for auditability
- Enterprise IDP integration (Entra ID, Okta, Ping)

Strata's [research](https://www.strata.io/blog/agentic-identity/the-ai-agent-identity-crisis-new-research-reveals-a-governance-gap/) found only 23% of organisations have a formal enterprise-wide strategy for agent identity management.

**Other generic gateways:** [Composio](https://composio.dev/blog/mcp-gateways-guide), [MintMCP](https://www.mintmcp.com/blog/enterprise-ai-infrastructure-mcp), and several others are building MCP gateway products with varying levels of maturity. The space is crowding quickly at the generic layer.

### Domain-Specific Governed MCP Servers

**Itential MCP Server (Network Infrastructure)**

The most relevant comparator to what we're building. [Itential](https://www.itential.com/cloud-platform/itential-mcp-server/) does for network infrastructure (Cisco, Juniper, Palo Alto, etc.) what we're proposing for Veeam infrastructure: AI agents invoke workflows through MCP, but every request is schema-validated, passes through RBAC/SSO/audit guardrails, and goes through approval pipelines before touching infrastructure. Key capabilities:

- AI agents can provision infrastructure and execute Day 2 operations (updates, compliance checks, deprovisioning)
- Every request schema-validated and passed through RBAC, SSO, and audit guardrails
- Platform policies and approvals determine execution eligibility
- FlowMCP Server (enterprise version) adds persona-based access control, multi-instance management, and virtual MCP servers
- Works with any MCP-compatible agent (Claude, ChatGPT, custom agents)

Itential demonstrates that the domain-specific governed MCP server model works and has enterprise demand. Their tagline — "Connect AI Agents to Enterprise Infrastructure — with Governance, Context, & Control" — describes exactly what we're building for a different domain.

### Platform-Level Initiatives

**[Microsoft Zero Trust for the Agentic Workforce](https://www.microsoft.com/en-us/security/blog/2025/05/19/microsoft-extends-zero-trust-to-secure-the-agentic-workforce/)** — Microsoft has extended Zero Trust principles to cover AI agents as "non-human identities" (NHIs), treating them with the same governance rigour as human identities. Their guidance: inventory agents, assign clear ownership, govern access, and apply consistent security standards.

**[Cisco Zero Trust in the Era of Agentic AI](https://blogs.cisco.com/security/zero-trust-in-the-era-of-agentic-ai)** — Cisco's position paper on applying Zero Trust to agentic AI. Focuses on network-level controls and identity but acknowledges the need for action-level governance.

### Competitive Analysis: What They Have vs. What We Have

| Capability | Gravitee | Strata | Itential | Portunus (Proposed) |
|-----------|----------|--------|----------|-------------------|
| MCP protocol inspection | Yes | No (identity layer) | Yes | Planned (Phase 1) |
| Identity federation (OAuth/OIDC) | Yes | Yes (core focus) | Yes | No (own key system) |
| Rate limiting / traffic shaping | Yes | No | Yes | Planned (Phase 4) |
| Audit trail | Yes | Yes | Yes | Planned (Phase 3) |
| Approval workflows | No | Yes (HITL) | Yes (platform policies) | Planned (Phase 3) |
| Domain-specific policy | No | No | Yes (networking) | **Yes (data protection)** |
| Declarative operations (diff/apply/export) | No | No | Yes (workflows) | **Yes (owlctl)** |
| Value-aware security classification | No | No | No | **Yes (severity engine)** |
| Credential isolation (built-in) | No (BYO IdP) | No (BYO IdP) | No (BYO IdP) | **Yes (PostgreSQL encrypted)** |
| Multi-product Veeam support | No | No | No | **Yes (VBR, VB365, VONE, etc.)** |
| Self-contained deployment | No (SaaS/platform) | No (SaaS/platform) | No (platform) | **Yes (single binary + PostgreSQL)** |

### Market Positioning

The generic MCP gateway market is getting crowded. Competing with Gravitee, Strata, and the wave of startups entering this space on generic governance features would be a losing strategy — they have more resources and head start on the horizontal layer.

The opportunity is in **domain-specific governed MCP servers for enterprise infrastructure**. This is the Itential model: deep understanding of the domain (network infrastructure for Itential, data protection infrastructure for us), combined with governed agent access.

**Our differentiation:**

1. **Semantic understanding of data protection** — The governance layer knows that `encryption.isEnabled: false` is a CRITICAL security change and `description: "new name"` is INFO. Generic gateways treat both as "a PUT request."

2. **Declarative operations** — owlctl's diff/apply/export/snapshot layer provides infrastructure-as-code semantics. Generic gateways proxy raw API calls with no concept of desired state, drift detection, or idempotent apply.

3. **Self-contained credential management** — Portunus already encrypts and manages Veeam credentials with a tiered key system. Generic gateways require customers to bring their own identity provider and credential store.

4. **Single-vendor data protection coverage** — Portunus handles VBR, VB365, VONE, Enterprise Manager, and all Veeam cloud products through one gateway. No other product in the market provides governed agent access across the full Veeam portfolio.

**Strategic options:**

- **Option A: Standalone** — Portunus is a self-contained governed MCP server for Veeam. Handles identity, policy, audit, and Veeam-specific governance internally. Simplest for customers who only need Veeam governance.
- **Option B: Behind a generic gateway** — Portunus sits behind Gravitee or similar as the domain-specific backend. The generic gateway handles identity federation, rate limiting, and cross-platform audit. Portunus handles Veeam-specific policy, credential management, and declarative operations. Better for enterprises that already run a generic gateway.
- **Option C: Both** — Portunus works standalone for smaller deployments and can optionally federate with a generic gateway for enterprise deployments. This is the recommended approach — it doesn't force a dependency on a third-party gateway while remaining compatible with the emerging ecosystem.

---

## The Proposed Pattern: Agentic Zero Trust

### Why This Name

The concept is the application of **Zero Trust Architecture principles to AI agent actions on enterprise systems**. We propose the term **Agentic Zero Trust** because:

1. **It builds on established trust** — Zero Trust (NIST SP 800-207) is well-understood by enterprise security teams. Framing agent governance as an extension of ZTA communicates the concept immediately.
2. **It captures the core principle** — Every agent action is evaluated against policy, regardless of the agent's identity, prior approvals, or the tool's capabilities. No implicit trust.
3. **It distinguishes from "AI Guardrails"** — Guardrails has become an overloaded term conflating content safety, output filtering, and action governance. Agentic Zero Trust is specifically about governing actions on real systems.
4. **It's architecturally precise** — ZTA defines Policy Decision Points (PDP) and Policy Enforcement Points (PEP). Agentic Zero Trust applies these concepts to the agent-to-tool execution boundary.

**Industry convergence:** Microsoft, Cisco, and Strata are all now using "Zero Trust" language in the context of AI agents (see Market Landscape). Microsoft's framing — "make every AI agent a first-class identity and govern it with the same rigour as human identities" — aligns directly with this principle. The term "Agentic Zero Trust" is not yet claimed by a specific vendor or standard, but the concept is gaining traction across the industry. An academic paper, ["Agent-Aware Zero Trust: A Framework for Securing Agentic AI in SASE and Cloud Architectures"](https://computerfraudsecurity.com/index.php/journal/article/view/926), has been published formalising the concept.

### Core Principles

**1. Every action is evaluated, every time**

No agent has standing permission to execute actions. Every tool invocation is evaluated against the current policy, current context, and current risk level. An agent that applied a job config successfully five minutes ago must still pass policy evaluation for the next apply.

**2. Policy is external to both agent and tool**

The agent does not decide what it's allowed to do. The tool does not decide who can use it. Policy is maintained separately, evaluated by an independent engine, and enforced at the boundary between agent and tool.

**3. Permissions are scoped and contextual**

An agent's allowed actions depend on:
- **Identity** — Which agent/user is making the request
- **Tool** — Which operation is being invoked (diff vs. apply vs. export)
- **Parameters** — What arguments are being passed (which instance, which resource)
- **Context** — Time of day, current drift state, recent actions, environment (dev vs. prod)

**4. High-risk actions require escalation**

The policy engine classifies actions by risk. Low-risk actions (diff, export, read-only operations) proceed automatically. High-risk actions (apply to production, disable encryption, change retention) require human approval through a defined workflow.

**5. Every action is audited immutably**

Every tool invocation, policy decision, approval, and outcome is recorded in an immutable audit log. This log is separate from Git history and captures the full chain: who requested, what policy evaluated, whether it was approved, what the tool returned, and what changed.

---

## Portunus: The Existing Foundation

### What Portunus Is

[Portunus](https://github.com/shapedthought/portunus) (named after the Roman god of keys and doors) is an existing project that already solves several of the hardest problems in this architecture. It is a **unified API gateway for all Veeam products** built in Rust with Actix, backed by PostgreSQL.

**Current capabilities:**

| Capability | Description |
|-----------|-------------|
| **Unified API** | Single endpoint (`/papi/{service_name}/{endpoint}`) proxies to VBR, VB365, VONE, Enterprise Manager, VBAWS, VBAZURE, VBGCP |
| **Credential isolation** | Veeam passwords encrypted in PostgreSQL. Agents never see raw credentials — they authenticate with Portunus API keys only. |
| **Tiered key system** | Root Master Key → Master Keys → User Keys. User keys have per-product boolean permissions and admin/non-admin (read-only vs read-write). Keys expire after N days. |
| **Session management** | Automatic OAuth token refresh, session keepalive (Enterprise Manager), retry logic with configurable limits |
| **Service health** | Services tracked as ok/retry/error. Failed services go offline and retry automatically. Manual reset available. |
| **Deployable** | Docker, Kubernetes, Helm ready. Single binary (Rust). |

**Architecture:**

```
┌──────────┐     API Key      ┌───────────┐     OAuth/Basic     ┌─────────┐
│  Client  │ ───────────────→ │ Portunus  │ ─────────────────→  │  VBR    │
│          │ ←─────────────── │ (Actix)   │ ←───────────────── │         │
└──────────┘    JSON response └─────┬─────┘    Token refresh    └─────────┘
                                    │                            ┌─────────┐
                                    │ ─────────────────────────→ │  VB365  │
                                    │                            └─────────┘
                                    │                            ┌─────────┐
                                    │ ─────────────────────────→ │  VONE   │
                                    │                            └─────────┘
                                    │                            ┌─────────┐
                                    └──────────────────────────→ │  etc.   │
                                                                 └─────────┘
                              ┌───────────┐
                              │ PostgreSQL │ ← encrypted creds, keys, service state
                              └───────────┘
```

### What Portunus Already Solves

Mapping Portunus capabilities against the Agentic Zero Trust requirements:

| Requirement | Portunus Status | Notes |
|------------|----------------|-------|
| Credentials only work through the proxy | **Done** | Agents get Portunus API keys. Veeam credentials never leave the server. |
| Single entry point for all products | **Done** | One gateway for VBR, VB365, VONE, Enterprise Manager, VBAWS, VBAZURE, VBGCP |
| Scoped permissions | **Partial** | Per-product booleans + admin/non-admin. Needs per-action and per-instance granularity. |
| Fail-closed | **Done** | Services in error state refuse requests. Configurable retry limits. |
| Session management | **Done** | Token refresh, keepalive, retry — agents never handle OAuth flows. |
| Deployable in enterprise environments | **Done** | Docker, K8s, Helm. PostgreSQL backend. |
| Policy-as-code evaluation | **Not yet** | Permissions are static booleans, not contextual policy rules. |
| MCP interface | **Not yet** | REST API only. Needs MCP server layer. |
| Approval workflows | **Not yet** | No escalation mechanism for high-risk operations. |
| Structured audit trail | **Not yet** | Info-level logging exists but not structured, queryable audit events. |
| owlctl declarative operations | **Not yet** | Proxies raw Veeam API calls. Doesn't expose owlctl's declarative layer (apply, diff, export). |

### Why Portunus Is the Right Starting Point

Building the governance layer on Portunus rather than from scratch means:

1. **Credential isolation is already solved** — This is the hardest part of "credentials that only work through the proxy." Portunus encrypts Veeam passwords in PostgreSQL and issues its own API keys. An AI agent with a Portunus key cannot extract the underlying Veeam credentials.

2. **Multi-product support is already solved** — Portunus handles VBR, VB365, VONE, Enterprise Manager, and all cloud products. The governance layer inherits this breadth immediately.

3. **The key hierarchy maps to agent permissions** — Root Master Key → Master Keys → User Keys is a natural fit for: Platform team → Team-level policies → Agent-specific keys with scoped permissions.

4. **Service health and retry logic exist** — Fail-closed behaviour is already implemented. The governance layer doesn't need to build connection management.

5. **It's Rust** — Performance, memory safety, and single-binary deployment. The governance additions (policy evaluation, audit logging) won't compromise the gateway's reliability.

---

## Architecture: Portunus as MCP Governance Gateway

### Revised Architecture

Rather than a standalone MCP governance proxy wrapping owlctl, Portunus evolves to become the governance gateway. It gains three new capabilities: an MCP interface, a policy engine, and declarative operation support (owlctl integration).

```
┌─────────────────────────────────────────────────────────────────┐
│  AI Agent (OpenClaw / Claude / Copilot / Custom)                │
│  "Change retention to 30 days for prod-backup"                  │
└──────────────────────────┬──────────────────────────────────────┘
                           │ MCP tool call
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│  Portunus                                                       │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ MCP Interface (NEW)                                       │  │
│  │  Exposes governed tools to AI agents                      │  │
│  └──────────────────────────┬────────────────────────────────┘  │
│                              ▼                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────────┐ │
│  │ Tool Router  │→│ Policy Engine │→│ Approval Workflow       │ │
│  │ (existing +  │  │ (NEW)        │  │ (NEW)                  │ │
│  │  declarative)│  │              │  │ Slack / Teams / ITSM   │ │
│  └──────────────┘  └──────┬───────┘  └────────────┬───────────┘ │
│                            │                       │             │
│  ┌─────────────────────────▼───────────────────────▼───────────┐ │
│  │ Audit Logger (NEW — PostgreSQL, structured, queryable)      │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌──────────────────────────────┐  ┌───────────────────────────┐ │
│  │ Existing Portunus Core       │  │ Declarative Layer (NEW)   │ │
│  │  - API key management        │  │  - owlctl diff/apply/     │ │
│  │  - Credential encryption     │  │    export via subprocess  │ │
│  │  - Token refresh             │  │  - Spec file management   │ │
│  │  - Service health            │  │  - State tracking         │ │
│  │  - REST proxy (/papi/)       │  │                           │ │
│  └──────────────┬───────────────┘  └─────────────┬─────────────┘ │
│                  │                                │               │
└──────────────────┼────────────────────────────────┼───────────────┘
                   │ HTTPS (existing)               │ Shell exec
                   ▼                                ▼
┌──────────────────────────┐  ┌──────────────────────────────────┐
│  Veeam APIs              │  │  owlctl CLI                      │
│  (VBR, VB365, VONE, etc.)│  │  (declarative operations only)   │
└──────────────────────────┘  └──────────────────────────────────┘
```

### Two Access Paths Through Portunus

Portunus serves two distinct types of operations, both governed by the same policy engine:

**1. Imperative operations (existing Portunus path)**

Direct API calls proxied to Veeam products. These already work today via `/papi/{service}/{endpoint}`.

```
Agent → MCP tool: "get-jobs" → Portunus → GET /api/v1/jobs → VBR
Agent → MCP tool: "start-job" → Portunus → POST /api/v1/jobs/{id}/start → VBR
```

The policy engine evaluates the HTTP method and endpoint to determine risk: GET is low, POST/PUT/DELETE is high.

**2. Declarative operations (new owlctl path)**

Infrastructure-as-code operations that use owlctl's declarative layer. Portunus invokes owlctl as a subprocess.

```
Agent → MCP tool: "diff-all-jobs" → Portunus → owlctl job diff --all → structured result
Agent → MCP tool: "apply-spec" → Portunus → owlctl job apply spec.yaml → apply result
Agent → MCP tool: "export-job" → Portunus → owlctl job export {id} → YAML spec
```

owlctl inherits Portunus's credentials via environment variables (`OWLCTL_URL`, `OWLCTL_USERNAME`, `OWLCTL_PASSWORD`) set from the service entry in PostgreSQL. The agent never sees these values.

### MCP Tool Definitions

The MCP interface exposes purpose-built tools. Agents never see raw shell access or raw API endpoints.

| MCP Tool | Portunus Path | Risk Level | Default Policy |
|----------|--------------|------------|----------------|
| `veeam-list-services` | Internal | Low | Always allow |
| `veeam-get` | `/papi/{svc}/{endpoint}` | Low | Allow with audit |
| `veeam-post` | `/papi/{svc}/{endpoint}` | High | Escalate for prod |
| `veeam-put` | `/papi/{svc}/{endpoint}` | High | Escalate for prod |
| `veeam-delete` | `/papi/{svc}/{endpoint}` | Critical | Always escalate |
| `owlctl-diff` | `owlctl diff` subprocess | Low | Always allow |
| `owlctl-export` | `owlctl export` subprocess | Low | Allow with audit |
| `owlctl-snapshot` | `owlctl snapshot` subprocess | Low | Allow with audit |
| `owlctl-plan` | `owlctl apply --dry-run` subprocess | Low | Always allow |
| `owlctl-apply` | `owlctl apply` subprocess | High | Escalate for prod |

### Component Responsibilities

**MCP Interface (NEW)**

Exposes Portunus as an MCP server. AI agents discover available tools, their parameters, and their descriptions through the standard MCP protocol. The interface translates MCP tool calls into either Portunus REST proxy calls or owlctl subprocess invocations.

**Tool Router (EXTENDED)**

The existing Portunus request routing (`/papi/{service}/{endpoint}`) is extended with declarative operation routing. The router determines whether a request goes to the REST proxy path or the owlctl subprocess path based on the MCP tool name.

**Policy Engine (NEW)**

Evaluates every tool invocation against policy rules before execution. Uses an embedded policy evaluator (internal to start, with OPA/Rego as a future option for enterprise deployments). Policy inputs include:

**Policy Engine**

Evaluates every tool invocation against policy rules before execution. Uses OPA/Rego (or a similar policy-as-code engine) for flexibility and auditability. Policy inputs include:

```rego
# Example policy input for an owlctl-apply call
input = {
  "agent": {
    "id": "openclaw-ops-bot",
    "user": "edward",
    "channel": "slack:#vbr-ops"
  },
  "tool": "owlctl-apply",
  "parameters": {
    "resource_type": "job",
    "spec_file": "infrastructure/jobs/prod-backup.yaml",
    "instance": "vbr-prod",
    "overlay": null
  },
  "context": {
    "time": "2026-02-14T14:30:00Z",
    "environment": "production",
    "recent_applies": 2,
    "last_drift_check": "2026-02-14T06:00:00Z",
    "spec_changes": {
      "fields_changed": ["storage.retentionPolicy.quantity"],
      "encryption_changed": false,
      "immutability_changed": false
    }
  }
}
```

Policy decisions:

```rego
# Block: encryption or immutability changes always require approval
deny["Encryption changes require security team approval"] {
  input.parameters.resource_type == "job"
  input.context.spec_changes.encryption_changed
}

# Allow: read-only operations always pass
allow {
  input.tool == "owlctl-diff"
}

# Allow: dev instance applies auto-approved
allow {
  input.tool == "owlctl-apply"
  input.parameters.instance == "vbr-dev"
}

# Escalate: prod applies require human approval
escalate["Production apply requires approval"] {
  input.tool == "owlctl-apply"
  input.parameters.instance == "vbr-prod"
}
```

**Approval Workflow**

When the policy engine returns `escalate`, the governance proxy holds the action and sends an approval request to the configured channel (Slack, Teams, email, or a queue). The request includes:

- What the agent wants to do (human-readable summary)
- What the dry-run shows (run `--dry-run` automatically before requesting approval)
- Which policy triggered the escalation
- Approve / Deny buttons with timeout

The action only executes after explicit approval. Approvals are logged with approver identity and timestamp.

**Audit Logger**

Records every interaction:

```json
{
  "timestamp": "2026-02-14T14:30:05Z",
  "agent_id": "openclaw-ops-bot",
  "user": "edward",
  "tool": "owlctl-apply",
  "parameters": {
    "resource_type": "job",
    "spec_file": "infrastructure/jobs/prod-backup.yaml",
    "instance": "vbr-prod"
  },
  "policy_decision": "escalate",
  "policy_reason": "Production apply requires approval",
  "approval": {
    "approver": "edward",
    "channel": "slack:#vbr-ops",
    "timestamp": "2026-02-14T14:31:12Z",
    "decision": "approved"
  },
  "execution": {
    "exit_code": 0,
    "duration_ms": 3200,
    "fields_applied": 1,
    "fields_rejected": 0
  }
}
```

This audit trail is queryable and separate from Git history. It captures the full decision chain: agent intent, policy evaluation, human approval, and system outcome.

---

## How This Applies to the owlctl + Portunus Ecosystem

### Current State

```
Human → Azure DevOps Pipeline → owlctl → VBR
         (governance here)

Human → Portunus API key → Portunus → VBR / VB365 / VONE / etc.
                            (auth + proxy, no governance)
```

Two separate paths exist. Pipeline-based governance covers planned Git-driven changes but not ad-hoc or agent-driven operations. Portunus provides credential isolation and multi-product access but has no policy evaluation or action-level governance.

### Proposed State

```
Human or AI Agent → MCP → Portunus → owlctl → VBR
                            ↓          ↓
                      Policy Engine    Veeam APIs (VB365, VONE, etc.)
                      Approval Workflow
                      Audit Log
```

Portunus becomes the single governed entry point for both agent-driven and human-driven operations across all Veeam products. The pipeline-based governance remains for Git-driven workflows (PRs, merge deploys). Both enforce the same policy intent.

### Implementation Path

**Phase 1: MCP interface + read-only tools**

Add an MCP server interface to Portunus that exposes read-only operations:
- `veeam-get` — Proxy GET requests to any registered Veeam service
- `owlctl-diff` — Run drift detection via owlctl subprocess
- `owlctl-export` — Export resource configurations
- `veeam-list-services` — List registered services and their health

No policy engine needed yet — these are read-only. This phase validates the MCP integration, proves the owlctl subprocess pattern, and provides immediate value (agents can query VBR state through Portunus).

**Portunus changes:** Add MCP server module (Rust MCP SDK or custom implementation over stdio/SSE). Add owlctl subprocess executor that injects credentials from PostgreSQL as env vars.

**Phase 2: Write tools + embedded policy engine**

Add write operations behind an embedded policy evaluator:
- `owlctl-apply` — Apply specs with policy evaluation
- `owlctl-plan` — Dry-run (always allowed, useful for agents to preview)
- `veeam-post` / `veeam-put` / `veeam-delete` — Proxied write operations with policy check

Extend Portunus's existing key permission model from static booleans to contextual rules:

```
Current:  { "vbr": true, "admin": true }
Extended: { "vbr": true, "admin": true, "policy": "ops-team-prod" }
```

The `policy` field references a named policy that defines what operations this key allows, at what risk levels, and for which services. Start with a simple internal evaluator (YAML/JSON rules, similar to owlctl's policy-rules.yaml). OPA/Rego can be added later as an external engine for enterprise deployments.

**Portunus changes:** Add policy evaluation middleware. Extend user key model with policy reference. Add `policies` table to PostgreSQL. Add structured audit events table.

**Phase 3: Approval workflows + queryable audit trail**

Add approval integration for escalated actions:
- Slack / Teams webhook for approval requests with Approve/Deny buttons
- ServiceNow / Jira Service Management integration for ITSM-heavy environments
- Timeout and fallback behaviour (fail-closed: if no response within N minutes, deny)

Build the structured audit trail in PostgreSQL:
- Every tool invocation, policy decision, approval, and outcome recorded
- Queryable via a new Portunus API endpoint (`/audit/...`) and/or MCP tool (`audit-query`)
- Retention policy configurable per deployment

**Portunus changes:** Add approval workflow module with webhook integrations. Add `audit_events` table. Add audit query API/MCP tool.

**Phase 4: Multi-agent, multi-service orchestration**

Support agent workflows that span multiple services and instances: "Check drift across all prod VBR instances and remediate any critical findings, then verify VB365 backup status." Portunus already manages multiple services — the governance layer evaluates each action independently against the calling agent's policy.

Add agent-level features:
- Rate limiting per agent key (prevent runaway automation)
- Concurrent action limits (only one apply per service at a time)
- Cross-service correlation (agent applied to VBR-1 and VBR-2 in the same session — log as related actions)

**Portunus changes:** Add rate limiting middleware. Add action locking per service. Add session/correlation tracking.

---

## The Broader Principle

### Beyond owlctl

The MCP governance proxy pattern is not specific to owlctl or Veeam. It applies to any enterprise system that AI agents need to operate:

- **Infrastructure as Code** — Terraform, Ansible, Pulumi
- **Cloud platforms** — AWS, Azure, GCP management APIs
- **CI/CD systems** — Pipeline creation, deployment triggers
- **Monitoring and incident response** — PagerDuty, Datadog, ServiceNow
- **Identity and access management** — Entra ID, Okta

Every one of these systems faces the same gap: they have APIs, they have CLI tools, and AI agents can call them. None of them have a governance layer designed for agentic access.

### Agentic Zero Trust as a Design Principle

Agentic Zero Trust is not a product. It's a design principle for enterprise AI adoption:

> **Every AI agent action on an enterprise system must be independently evaluated against policy, scoped to the minimum required permissions, subject to approval workflows proportional to risk, and recorded in an immutable audit trail — regardless of the agent's identity, the tool's capabilities, or any prior authorisations.**

This principle produces specific architectural requirements:

1. **No direct tool access** — Agents interact through governed interfaces, not raw APIs or shells
2. **Policy-as-code** — Governance rules are versioned, testable, and auditable (not UI toggles)
3. **Scoped tool definitions** — The governance layer exposes purpose-built operations, not generic capabilities
4. **Contextual evaluation** — Policy considers not just "who" and "what" but "when", "where", and "why"
5. **Proportional escalation** — Low-risk actions flow automatically; high-risk actions require human judgement
6. **Immutable audit** — The complete chain from agent intent to system outcome is recorded

### Why This Matters Now

The window between "AI agents can operate enterprise systems" and "enterprise teams are comfortable letting them" is the adoption gap. Organisations that solve this gap will:

- Adopt AI-driven operations faster (with confidence, not fear)
- Maintain compliance posture as automation increases
- Have defensible audit trails for regulated environments
- Scale human expertise through AI without scaling risk

Organisations that don't solve this gap will either:
- Block AI agent adoption entirely (losing competitive advantage)
- Allow ungoverned agent access (creating unacceptable risk)

The governance layer is what turns "AI can do this" into "AI is allowed to do this, and we can prove it."

---

## Relation to Existing Projects

### Portunus Capabilities That Carry Forward

| Portunus Feature | Governance Layer Use |
|-----------------|---------------------|
| Encrypted credential storage (PostgreSQL) | Agents never see Veeam passwords — solves bypass prevention |
| Tiered key system (Root → Master → User) | Maps to Platform team → Team policies → Agent-specific keys |
| Per-product permissions (vbr/vb365/vone booleans) | Foundation for scoped access — extended with per-action granularity |
| Admin/non-admin distinction | Foundation for read-only vs read-write — extended with contextual risk evaluation |
| Key expiry (N days) | Time-bounded agent access — agents must re-enrol periodically |
| Service health tracking (ok/retry/error) | Fail-closed behaviour — agents cannot reach unhealthy services |
| Token refresh and session management | Agents never handle OAuth flows — Portunus manages all Veeam sessions |
| REST proxy (`/papi/{svc}/{endpoint}`) | Imperative operations continue to work — governance wraps existing path |
| Docker / K8s / Helm deployment | Enterprise deployment model already established |

### owlctl Capabilities That Feed Into Portunus

| owlctl Feature | Governance Layer Use |
|---------------|---------------------|
| `--instance` flag | Portunus service entries map directly to owlctl instances |
| `--dry-run` flag | Auto-preview before approval requests |
| Exit codes (0/3/4/5/6) | Policy engine inputs (was the last diff clean?) |
| `policy-rules.yaml` | Seed rules for Portunus policy engine (same concepts, different enforcement point) |
| `state.json` | Context input for policy evaluation (last apply time, drift state) |
| Groups + specsDir | Define what an agent can operate on (group = permission scope) |

### How the Three Systems Work Together

```
┌─────────────────────────────────────────────────────────────┐
│                    Git Repository                            │
│  infrastructure/    policy-rules.yaml    state.json          │
│       ↕ (PR gate)       ↕ (shared)         ↕ (commit)       │
├─────────────────────────────────────────────────────────────┤
│  Azure DevOps Pipelines                                      │
│  gitops-pr-gate.yml    gitops-deploy.yml                     │
│  (Git-driven governance — planned changes)                   │
├─────────────────────────────────────────────────────────────┤
│  Portunus                                                    │
│  MCP interface + Policy engine + Audit trail                 │
│  (Agent-driven governance — ad-hoc and automated changes)    │
├─────────────────────────────────────────────────────────────┤
│  owlctl                                                      │
│  (Declarative operations — apply, diff, export, snapshot)    │
├─────────────────────────────────────────────────────────────┤
│  Veeam Products                                              │
│  VBR    VB365    VONE    Enterprise Manager    Cloud          │
└─────────────────────────────────────────────────────────────┘
```

- **Pipelines** govern the Git-driven workflow (PRs, merge deploys)
- **Portunus** governs the agent-driven workflow (MCP, OpenClaw, ad-hoc)
- **owlctl** provides the declarative operations both paths use
- **Policy rules** are shared — the same intent enforced at both boundaries
- **Git** remains the source of truth for desired state

---

## Design Decisions (Resolved)

1. **Generic, not owlctl-specific.** Portunus already supports all Veeam products. The governance layer wraps both imperative API calls (existing `/papi/` proxy) and declarative operations (owlctl subprocess). The MCP interface, policy engine, and audit trail are product-agnostic — they govern any tool invocation that flows through Portunus. Veeam is the first domain; the same Portunus architecture could proxy other enterprise systems in the future.

2. **Embedded policy engine to start, external OPA later.** The policy evaluator starts as an internal Rust module reading rules from PostgreSQL (or YAML config). This keeps Portunus as a single binary with zero external dependencies beyond PostgreSQL. For enterprise deployments that already run OPA, a future phase adds OPA as an optional external policy decision point — Portunus calls OPA's REST API instead of evaluating rules internally. The interface is the same; the engine is swappable.

3. **Approval workflows plug into existing ITSM.** The approval module is webhook-based: when a policy escalates an action, Portunus sends a structured payload to a configurable webhook URL. This supports Slack (interactive messages), Teams (adaptive cards), ServiceNow (incident/change request creation), and Jira Service Management (issue creation) without Portunus needing to know the specifics of each system. A simple Slack integration ships first; ITSM adapters are configuration, not code changes.

4. **Fail-closed, always.** If Portunus is unavailable, no agent actions proceed. This is non-negotiable for enterprise use. Portunus's existing service health model (ok/retry/error) already implements this pattern for downstream Veeam services — the same principle applies to Portunus itself. High availability is achieved through standard infrastructure (load balancer, multiple replicas, PostgreSQL HA) rather than weakening the security model.

5. **Credential scoping prevents bypass.** Agents receive Portunus API keys only. They never have Veeam credentials (`OWLCTL_USERNAME`, `OWLCTL_PASSWORD`, `OWLCTL_URL`). Portunus injects these into the owlctl subprocess from its encrypted database. An agent with shell access (OpenClaw) could call owlctl directly, but without credentials it cannot authenticate. The Veeam credentials exist only inside Portunus's PostgreSQL instance. This is already how Portunus works today — no new mechanism needed.

## Remaining Open Questions

1. **MCP transport: stdio or SSE?** stdio is simpler (agent spawns Portunus as subprocess) but doesn't support remote access. SSE (Server-Sent Events over HTTP) supports remote agents but needs its own auth layer. SSE is likely the right choice since Portunus is already a network service, and the existing API key system provides authentication.

2. **How does owlctl get spec files?** For declarative operations, owlctl needs YAML spec files on disk. Options: Portunus maintains a Git checkout of the infrastructure repo, agent uploads spec content in the MCP tool call (Portunus writes to temp file), or Portunus fetches from a Git URL on demand.

3. **State file management.** owlctl reads/writes `state.json` locally. When Portunus invokes owlctl as a subprocess, where does state live? Options: Portunus manages state in PostgreSQL and injects it as a file for each subprocess call, or Portunus maintains a working directory per service with persistent state.

4. **Portunus versioning and migration.** Portunus was last updated in April 2023. The Rust ecosystem has moved significantly (Actix 4 → dependencies may need updating, SQLX 0.6 → 0.8, etc.). Phase 0 should include a dependency audit and modernisation pass before adding new features.

---

## Summary

- **The gap is real and immediate**: AI agents can operate enterprise systems today; governance for this does not exist as a standard pattern.
- **The pattern is Agentic Zero Trust**: Apply Zero Trust principles (evaluate every action, scope permissions, require proportional approval, audit everything) to the boundary between AI agents and enterprise tools.
- **Portunus is the foundation, not a greenfield build**: Credential isolation, multi-product access, tiered key management, fail-closed behaviour, and enterprise deployment are already solved. The governance layer is an extension of what Portunus does, not a rewrite.
- **Three new capabilities turn Portunus into a governance gateway**: MCP interface (agent access), policy engine (contextual action evaluation), and structured audit trail (immutable, queryable).
- **owlctl provides the declarative layer**: Apply, diff, export, snapshot operations flow through Portunus as subprocess calls, inheriting credential isolation and policy enforcement automatically.
- **This is bigger than Veeam**: The same architecture applies to any enterprise system that AI agents need to operate. Portunus + owlctl is the first implementation; the pattern is generic.
