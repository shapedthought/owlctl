I need you to act as a senior product manager and Go systems architect working on owlctl - a CLI tool for interacting with Veeam APIs. This is a production tool with 8 GitHub stars used by Veeam administrators who need simple, accessible API access across multiple Veeam products.

**Current Project Context:**
- **Repository**: https://github.com/shapedthought/owlctl
- **Language**: Go (chosen for single-binary distribution with zero dependencies)
- **Purpose**: Simplify Veeam API interactions by handling authentication and providing consistent CLI interface
- **Supported APIs**: VBR, Enterprise Manager, VB365, VONE, VB for Azure/AWS/GCP
- **Current Commands**: login, get, post, put, profile, utils, job (template-based)
- **Last Update**: October 2023 (needs modernization)
- **Current Version**: 0.8.0-beta1
- **Key Design Principles**: Simple install, simple syntax, run anywhere/anything, complement (not replace) PowerShell cmdlets

**Target Users:**
- Veeam administrators who need quick API access
- DevOps engineers automating backup workflows  
- Infrastructure-as-Code practitioners managing multi-site Veeam deployments
- Users without PowerShell access (especially for AWS/Azure/GCP products)
- Multi-platform teams (Windows, Linux, macOS)

**Strategic Direction - Declarative Management:**
I want to evolve owlctl from an imperative API wrapper into a declarative infrastructure management tool for Veeam environments. This would enable:
- **Infrastructure-as-Code workflows**: Define desired state in YAML/HCL, apply changes
- **GitOps patterns**: Version control Veeam configurations, peer review changes
- **Drift detection**: Compare actual vs desired state across environments
- **Multi-environment consistency**: Manage dev/staging/prod Veeam infrastructure uniformly
- **Disaster recovery**: Declarative configs serve as documentation and recovery templates

Think: "Terraform for Veeam" or "kubectl apply" patterns, but leveraging owlctl's existing strengths.

**Feature Vision:**
Consider features that bridge the current imperative commands with declarative management:

**Phase 1 - Foundation:**
- State file management (current vs desired)
- Configuration templating and validation
- Dry-run/plan capabilities
- Basic diff/drift detection

**Phase 2 - Core Declarative Features:**
- Apply/reconcile operations
- Resource dependency management
- Idempotent operations
- Rollback capabilities

**Phase 3 - Advanced Workflows:**
- Multi-environment orchestration
- CI/CD pipeline integration
- Configuration migration tools
- Compliance and policy enforcement

**Your Task:**

1. **Create a Comprehensive Feature Epic Analysis**
   Include:
   - Clear business value for declarative management (why Veeam admins need this NOW)
   - User stories from perspectives of:
     - Solo admin managing 5+ Veeam environments
     - Enterprise team with change management requirements
     - MSP managing hundreds of customer Veeam deployments
   - Success metrics (config drift incidents reduced, deployment time, audit compliance)
   - Phased approach: what's the minimum viable declarative feature?
   - Technical architecture considerations:
     - State management (local files vs remote backends like S3/Azure Blob)
     - File formats (YAML vs HCL vs TOML - consider pros/cons)
     - Reconciliation engine design patterns
     - Backward compatibility with existing imperative commands
   - Integration with current profile/auth system
   - How this positions owlctl vs Terraform providers or Ansible modules

2. **Break Down Into Individual Implementation Tasks**
   For each subtask:
   - Scope it small enough to implement in 1-3 days but valuable enough to show real impact
   - Write clear acceptance criteria with example YAML/config snippets
   - Identify Go-specific patterns (struct tags for parsing, interfaces for resource types)
   - Consider how to handle API differences across Veeam products
   - Note any breaking changes and migration paths from imperative to declarative
   - Suggest testing strategies (include example config files as test fixtures)
   - Define the resource schema approach (typed structs vs dynamic maps)

3. **Prioritize and Recommend First Implementation**
   Before I start coding, analyze all the tasks and recommend which declarative feature to tackle first.
   
   Consider:
   - What's the smallest increment that demonstrates declarative value?
   - Which Veeam resource type is most painful to manage imperatively?
   - What builds toward a complete declarative system without over-engineering?
   - Should we start with read-only drift detection or full apply operations?
   - How do we avoid "second system syndrome" - keep it simple like current owlctl?
   
   Explain your reasoning as if presenting to both:
   - The engineering team (state management patterns, reconciliation loops, error handling)
   - Veeam admin community (time savings, reduced errors, auditability, version control benefits)

4. **Provide Implementation Guidance**
   For your recommended first task:
   - Suggest Go implementation approach:
     - Parsing libraries (yaml.v3, HCL, viper for config)
     - State comparison strategies (reflect, cmp, or custom diffing)
     - Resource abstraction patterns (interfaces for different Veeam products)
   - Identify potential gotchas:
     - API eventual consistency issues
     - Handling secrets in declarative configs
     - Resource dependencies and ordering
     - Partial failure scenarios
   - Recommend project structure for declarative features
   - Outline testing strategy (golden file tests, state comparison tests)
   - Suggest example config format with detailed comments
   - Consider backward compatibility: can imperative commands generate declarative configs?

**Additional Context:**
- Current open issue: Docker commands not working (Ubuntu image lacks wget)
- Tool is MIT licensed and community-driven
- Users value the zero-install, single-binary approach
- Given my background: extensive Terraform experience, building Terraform providers, infrastructure-as-code advocate
- Security critical: declarative configs must handle credentials safely
- Consider: should owlctl generate Terraform HCL instead of custom format? Trade-offs?

**Inspiration from Existing Tools:**
- Terraform: Plan/apply workflow, state management, resource graphs
- Kubernetes: Declarative desired state, reconciliation loops
- Pulumi: Multi-language support, but we'll stick with config files
- Ansible: Idempotency, playbook structure
- Crossplane: Composition patterns for complex resources

**Critical Questions to Address:**
1. What's the minimum viable declarative feature that provides immediate value?
2. Should we support both imperative commands AND declarative configs long-term?
3. How do we handle the "job templates" feature in a declarative world?
4. What resources should be declaratively managed first? (Jobs? Repositories? Credentials?)
5. How do we make migration from imperative to declarative workflows smooth?

**Output Format:**
Structure your response with clear sections using markdown headers. Provide:
- A phased roadmap showing quick wins leading to full declarative capability
- Concrete examples of what declarative Veeam config files would look like
- Architecture diagrams or pseudo-code for the reconciliation engine
- Comparison with alternative approaches (pure Terraform provider vs owlctl declarative layer)
- Clear recommendation on THE ONE FEATURE to start with and why it unlocks everything else
