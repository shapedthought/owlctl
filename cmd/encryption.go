package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/state"
	"github.com/shapedthought/owlctl/utils"
	"github.com/shapedthought/owlctl/vhttp"
	"github.com/spf13/cobra"
)

var (
	encSnapshotAll     bool
	encDiffAll         bool
	kmsSnapshotAll     bool
	kmsDiffAll         bool
	kmsApplyDryRun       bool
	kmsApplyGroupName    string
	kmsApplyOverlayFile  string
	kmsDiffGroupName     string

	// Export flags
	encExportOutput    string
	encExportDirectory string
	encExportAll       bool
	kmsExportOutput    string
	kmsExportDirectory string
	kmsExportAll       bool
	kmsExportAsOverlay bool
	kmsExportBasePath  string
)

var encryptionCmd = &cobra.Command{
	Use:   "encryption",
	Short: "Encryption password and KMS server management commands",
	Long: `Encryption related commands for state management and drift detection.

ONLY WORKS WITH VBR AT THE MOMENT.

Note: Only password metadata (IDs, hints) is tracked — never actual password values.

Subcommands:

Export encryption password metadata to YAML
  owlctl encryption export "My backup password"
  owlctl encryption export --all -d specs/encryption/

Snapshot encryption password inventory
  owlctl encryption snapshot "My backup password"
  owlctl encryption snapshot --all

Detect encryption password drift
  owlctl encryption diff "My backup password"
  owlctl encryption diff --all

KMS server management
  owlctl encryption kms-export "My KMS Server"
  owlctl encryption kms-export --all -d specs/kms/
  owlctl encryption kms-snapshot "My KMS Server"
  owlctl encryption kms-snapshot --all
  owlctl encryption kms-diff "My KMS Server"
  owlctl encryption kms-diff --all
  owlctl encryption kms-apply kms/my-kms.yaml
`,
}

// --- Encryption Password commands ---

var encSnapshotCmd = &cobra.Command{
	Use:   "snapshot [password-hint]",
	Short: "Snapshot encryption password metadata to state",
	Long: `Capture the current encryption password metadata and store it in state.
Only metadata (ID, hint, import status) is captured — never actual password values.

Examples:
  # Snapshot a single encryption password by hint
  owlctl encryption snapshot "My backup password"

  # Snapshot all encryption passwords
  owlctl encryption snapshot --all
`,
	Run: func(cmd *cobra.Command, args []string) {
		if encSnapshotAll {
			snapshotAllEncryptionPasswords()
		} else if len(args) > 0 {
			snapshotSingleEncryptionPassword(args[0])
		} else {
			log.Fatal("Provide password hint or use --all")
		}
	},
}

var encDiffCmd = &cobra.Command{
	Use:   "diff [password-hint]",
	Short: "Detect encryption password inventory drift",
	Long: `Compare current VBR encryption password inventory against the last snapshot
to detect removed, added, or modified password records.

When using --all, performs inventory-level checks:
  - Detects passwords removed from VBR (CRITICAL)
  - Detects passwords added since last snapshot
  - Detects metadata changes on existing passwords

Examples:
  # Check single password for drift
  owlctl encryption diff "My backup password"

  # Check entire encryption inventory
  owlctl encryption diff --all

Exit Codes:
  0 - No drift detected
  3 - Drift detected (INFO or WARNING)
  4 - Critical security drift detected
  1 - Error occurred`,
	Run: func(cmd *cobra.Command, args []string) {
		if encDiffAll {
			diffAllEncryptionPasswords()
		} else if len(args) > 0 {
			diffSingleEncryptionPassword(args[0])
		} else {
			log.Fatal("Provide password hint or use --all")
		}
	},
}

func snapshotSingleEncryptionPassword(hint string) {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	// Fetch all passwords and find by hint
	passwordList := vhttp.GetData[models.VbrEncryptionPasswordList]("encryptionPasswords", profile)

	var found *models.VbrEncryptionPasswordGet
	for i := range passwordList.Data {
		if passwordList.Data[i].Hint == hint {
			found = &passwordList.Data[i]
			break
		}
	}

	if found == nil {
		log.Fatalf("Encryption password with hint '%s' not found in VBR.", hint)
	}

	if err := saveEncryptionPasswordToState(found, nil); err != nil {
		log.Fatalf("Failed to save encryption password state: %v", err)
	}

	fmt.Printf("Snapshot saved for encryption password: %s\n", hint)
}

func snapshotAllEncryptionPasswords() {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	passwordList := vhttp.GetData[models.VbrEncryptionPasswordList]("encryptionPasswords", profile)

	if len(passwordList.Data) == 0 {
		fmt.Println("No encryption passwords found.")
		return
	}

	fmt.Printf("Snapshotting %d encryption passwords...\n", len(passwordList.Data))

	// Build hint counts to detect duplicates
	hintCounts := make(map[string]int, len(passwordList.Data))
	for _, p := range passwordList.Data {
		hintCounts[p.Hint]++
	}

	for i := range passwordList.Data {
		p := &passwordList.Data[i]
		if err := saveEncryptionPasswordToState(p, hintCounts); err != nil {
			fmt.Printf("Warning: Failed to save state for '%s': %v\n", p.Hint, err)
			continue
		}

		displayName := p.Hint
		if hintCounts[p.Hint] > 1 {
			displayName = fmt.Sprintf("%s-%s", p.Hint, p.ID)
		}
		fmt.Printf("  Snapshot saved: %s\n", displayName)
	}

	stateMgr := state.NewManager()
	fmt.Printf("\nState updated: %s\n", stateMgr.GetStatePath())
}

func saveEncryptionPasswordToState(p *models.VbrEncryptionPasswordGet, hintCounts map[string]int) error {
	// Marshal to JSON then to map for state storage
	pBytes, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("failed to marshal password data: %w", err)
	}

	name := p.Hint
	if name == "" {
		name = p.ID
	} else if hintCounts != nil && hintCounts[p.Hint] > 1 {
		// Hints are not guaranteed unique; append ID to avoid overwriting
		name = fmt.Sprintf("%s-%s", name, p.ID)
	}

	return saveResourceToState("VBREncryptionPassword", name, p.ID, json.RawMessage(pBytes))
}

func diffSingleEncryptionPassword(hint string) {
	loadSeverityOverrides()
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	// Load from state
	stateMgr := state.NewManager()
	resource, err := stateMgr.GetResource(hint)
	if err != nil {
		log.Fatalf("Encryption password '%s' not found in state. Has it been snapshotted?\n", hint)
	}

	if resource.Type != "VBREncryptionPassword" {
		log.Fatalf("Resource '%s' is not an encryption password (type: %s).\n", hint, resource.Type)
	}

	// Show (observed) label for monitored-only resources
	originLabel := ""
	if resource.Origin == "observed" {
		originLabel = " (observed)"
	}
	fmt.Printf("Checking drift for encryption password: %s%s\n\n", hint, originLabel)

	// Fetch current from VBR by ID
	passwordList := vhttp.GetData[models.VbrEncryptionPasswordList]("encryptionPasswords", profile)

	var currentMap map[string]interface{}
	found := false
	for _, p := range passwordList.Data {
		if p.ID == resource.ID {
			pBytes, err := json.Marshal(p)
			if err != nil {
				log.Fatalf("Failed to marshal current password data: %v", err)
			}
			if err := json.Unmarshal(pBytes, &currentMap); err != nil {
				log.Fatalf("Failed to unmarshal current password data: %v", err)
			}
			found = true
			break
		}
	}

	if !found {
		fmt.Printf("CRITICAL - Encryption password '%s' (ID: %s) has been removed from VBR!\n", hint, resource.ID)
		fmt.Println("\nThis password may be referenced by active backup jobs.")
		fmt.Println("Encrypted backups using this password may become unrecoverable.")
		os.Exit(4) // CRITICAL drift
	}

	// Compare, classify, filter
	drifts := detectDrift(resource.Spec, currentMap, encryptionIgnoreFields)
	drifts = classifyDrifts(drifts, encryptionSeverityMap)
	minSev := parseSeverityFlag()
	drifts = filterDriftsBySeverity(drifts, minSev)

	if len(drifts) == 0 {
		fmt.Println(noDriftMessage("Encryption password matches snapshot state.", minSev))
		os.Exit(0)
	}

	printSecuritySummary(drifts)
	fmt.Println("Drift detected:")
	for _, drift := range drifts {
		printDriftWithSeverity(drift)
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("  - %d drifts detected\n", len(drifts))
	fmt.Printf("  - Highest severity: %s\n", getMaxSeverity(drifts))
	if resource.Origin == "applied" {
		fmt.Printf("  - Last applied: %s\n", resource.LastApplied.Format("2006-01-02 15:04:05"))
		fmt.Printf("  - Last applied by: %s\n", resource.LastAppliedBy)
	} else {
		fmt.Printf("  - Last snapshot: %s\n", resource.LastApplied.Format("2006-01-02 15:04:05"))
		fmt.Printf("  - Last snapshot by: %s\n", resource.LastAppliedBy)
	}

	// Show guidance based on origin
	printRemediationGuidance(BuildEncryptionGuidance(hint, resource.Origin))

	os.Exit(exitCodeForDrifts(drifts))
}

func diffAllEncryptionPasswords() {
	loadSeverityOverrides()
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	stateMgr := state.NewManager()
	stateResources, err := stateMgr.ListResources("VBREncryptionPassword")
	if err != nil {
		log.Fatalf("Failed to load state: %v\n", err)
	}

	// Fetch current inventory from VBR
	passwordList := vhttp.GetData[models.VbrEncryptionPasswordList]("encryptionPasswords", profile)

	// Build lookup maps by ID
	stateByID := make(map[string]*state.Resource)
	for _, r := range stateResources {
		stateByID[r.ID] = r
	}

	currentByID := make(map[string]map[string]interface{})
	currentHintByID := make(map[string]string)
	for _, p := range passwordList.Data {
		pBytes, err := json.Marshal(p)
		if err != nil {
			fmt.Printf("Warning: Failed to marshal password '%s': %v\n", p.Hint, err)
			continue
		}
		var pMap map[string]interface{}
		if err := json.Unmarshal(pBytes, &pMap); err != nil {
			fmt.Printf("Warning: Failed to unmarshal password '%s': %v\n", p.Hint, err)
			continue
		}
		currentByID[p.ID] = pMap
		currentHintByID[p.ID] = p.Hint
	}

	if len(stateResources) == 0 {
		fmt.Println("No encryption passwords in state.")
		return
	}

	fmt.Printf("Checking encryption password inventory...\n")
	fmt.Printf("  State: %d passwords, VBR: %d passwords\n\n", len(stateResources), len(passwordList.Data))

	minSev := parseSeverityFlag()
	driftedCount := 0
	cleanCount := 0
	var allDrifts []Drift

	// Check for removed passwords (in state but not in VBR) — always CRITICAL
	for id, stateRes := range stateByID {
		if _, exists := currentByID[id]; !exists {
			removedDrift := Drift{Path: "inventory", Action: "removed", State: stateRes.Name, Severity: SeverityCritical}
			if severityRank(SeverityCritical) >= severityRank(minSev) {
				fmt.Printf("  CRITICAL - %s (ID: %s): Removed from VBR\n", stateRes.Name, id)
				allDrifts = append(allDrifts, removedDrift)
				driftedCount++
			}
		}
	}

	// Check for added passwords (in VBR but not in state) — INFO
	for id, hint := range currentHintByID {
		if _, exists := stateByID[id]; !exists {
			addedDrift := Drift{Path: "inventory", Action: "added", VBR: hint, Severity: SeverityInfo}
			if severityRank(SeverityInfo) >= severityRank(minSev) {
				fmt.Printf("  INFO + %s (ID: %s): Added since last snapshot\n", hint, id)
				allDrifts = append(allDrifts, addedDrift)
				driftedCount++
			}
		}
	}

	// Check field-level drift on matching passwords
	driftedApplied := 0
	driftedObserved := 0
	for id, stateRes := range stateByID {
		if currentMap, exists := currentByID[id]; exists {
			drifts := detectDrift(stateRes.Spec, currentMap, encryptionIgnoreFields)
			drifts = classifyDrifts(drifts, encryptionSeverityMap)
			drifts = filterDriftsBySeverity(drifts, minSev)

			// Show origin label for observed resources
			originLabel := ""
			if stateRes.Origin == "observed" {
				originLabel = " (observed)"
			}

			if len(drifts) > 0 {
				fmt.Printf("  %s%s: %d drifts detected\n", stateRes.Name, originLabel, len(drifts))
				for _, d := range drifts {
					printDriftWithSeverity(d)
				}
				allDrifts = append(allDrifts, drifts...)
				if stateRes.Origin == "observed" {
					driftedObserved++
				} else {
					driftedApplied++
				}
				driftedCount++
			} else {
				fmt.Printf("  %s%s: No drift\n", stateRes.Name, originLabel)
				cleanCount++
			}
		}
	}

	if len(allDrifts) > 0 {
		printSecuritySummary(allDrifts)
	}
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  - %d passwords clean\n", cleanCount)
	if driftedApplied > 0 {
		// Encryption passwords cannot be modified via API
		fmt.Printf("  - %d passwords drifted — manual remediation required in VBR console\n", driftedApplied)
	}
	if driftedObserved > 0 {
		fmt.Printf("  - %d passwords drifted (observed) — read-only; remediate manually in the VBR console\n", driftedObserved)
	}
	// Also count inventory changes (added/removed) which were counted earlier
	inventoryChanges := driftedCount - driftedApplied - driftedObserved
	if inventoryChanges > 0 {
		fmt.Printf("  - %d inventory changes detected\n", inventoryChanges)
	}

	if driftedCount > 0 {
		os.Exit(exitCodeForDrifts(allDrifts))
	}
	os.Exit(0)
}

// --- KMS Server commands ---

// kmsApplyConfig defines how to apply KMS server configurations
var kmsApplyConfig = ResourceApplyConfig{
	Kind:         "VBRKmsServer",
	Endpoint:     "kmsServers",
	IgnoreFields: kmsIgnoreFields,
	Mode:         ApplyUpdateOnly,
	FetchCurrent: fetchCurrentKmsServer,
}

// fetchCurrentKmsServer retrieves a KMS server by name from VBR
func fetchCurrentKmsServer(name string, profile models.Profile) (json.RawMessage, string, error) {
	kmsList := vhttp.GetData[models.VbrKmsServerList]("kmsServers", profile)

	for _, kms := range kmsList.Data {
		if kms.Name == name {
			// Marshal the KMS server to JSON
			kmsData, err := json.Marshal(kms)
			if err != nil {
				return nil, "", fmt.Errorf("failed to marshal KMS server: %w", err)
			}
			return json.RawMessage(kmsData), kms.ID, nil
		}
	}

	return nil, "", nil // Not found (not an error)
}

var kmsApplyCmd = &cobra.Command{
	Use:   "kms-apply [spec-file]",
	Short: "Apply a KMS server configuration to VBR",
	Long: `Apply a declarative KMS server configuration to VBR.

This command updates an existing KMS server with the configuration from a YAML spec file.
KMS servers cannot be created via the API - they must be created in the VBR console first.

Examples:
  # Apply a KMS server configuration
  owlctl encryption kms-apply kms/my-kms.yaml

  # Preview changes without applying (dry-run)
  owlctl encryption kms-apply kms/my-kms.yaml --dry-run

Exit Codes:
  0 - Success
  1 - Error (API failure, invalid spec)
  6 - Resource not found (KMS server doesn't exist in VBR)
`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if kmsApplyGroupName != "" {
			if len(args) > 0 {
				log.Fatal("Cannot use --group with a positional spec file argument")
			}
			if kmsApplyOverlayFile != "" {
				log.Fatal("Cannot use --group with --overlay (group defines its own overlay)")
			}
			applyGroupResource(kmsApplyGroupName, kmsApplyConfig, kmsApplyDryRun)
		} else if len(args) > 0 {
			settings := utils.ReadSettings()
			profile := utils.GetCurrentProfile()

			if settings.SelectedProfile != "vbr" {
				log.Fatal("This command only works with VBR at the moment.")
			}

			result := applyWithOptionalOverlay(args[0], kmsApplyOverlayFile, kmsApplyConfig, profile, kmsApplyDryRun)
			if result.Error != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", result.Error)
				outcome := DetermineApplyOutcome([]ApplyResult{result})
				os.Exit(ExitCodeForOutcome(outcome))
			}

			if result.DryRun {
				return // Dry-run output already printed
			}

			fmt.Printf("\nSuccessfully %s KMS server: %s\n", result.Action, result.ResourceName)
		} else {
			log.Fatal("Provide a spec file or use --group")
		}
	},
}

var kmsSnapshotCmd = &cobra.Command{
	Use:   "kms-snapshot [kms-name]",
	Short: "Snapshot KMS server configuration to state",
	Long: `Capture the current KMS server configuration and store it in state.

Examples:
  # Snapshot a single KMS server
  owlctl encryption kms-snapshot "My KMS Server"

  # Snapshot all KMS servers
  owlctl encryption kms-snapshot --all
`,
	Run: func(cmd *cobra.Command, args []string) {
		if kmsSnapshotAll {
			snapshotAllKmsServers()
		} else if len(args) > 0 {
			snapshotSingleKmsServer(args[0])
		} else {
			log.Fatal("Provide KMS server name or use --all")
		}
	},
}

var kmsDiffCmd = &cobra.Command{
	Use:   "kms-diff [kms-name]",
	Short: "Detect KMS server configuration drift",
	Long: `Compare current VBR KMS server configuration against the last snapshot
to detect changes or drift.

Examples:
  # Check single KMS server for drift
  owlctl encryption kms-diff "My KMS Server"

  # Check all KMS servers
  owlctl encryption kms-diff --all

Exit Codes:
  0 - No drift detected
  3 - Drift detected (INFO or WARNING)
  4 - Critical security drift detected
  1 - Error occurred`,
	Run: func(cmd *cobra.Command, args []string) {
		if kmsDiffGroupName != "" {
			if kmsDiffAll {
				log.Fatal("Cannot use --group with --all")
			}
			if len(args) > 0 {
				log.Fatal("Cannot use --group with a positional KMS server name argument")
			}
			diffGroupResource(kmsDiffGroupName, GroupDiffConfig{
				Kind:         "VBRKmsServer",
				DisplayName:  "KMS server",
				FetchCurrent: fetchCurrentKmsServer,
				IgnoreFields: kmsIgnoreFields,
				SeverityMap:  kmsSeverityMap,
				RemediateCmd: "owlctl encryption kms-apply --group %s",
			})
		} else if kmsDiffAll {
			diffAllKmsServers()
		} else if len(args) > 0 {
			diffSingleKmsServer(args[0])
		} else {
			log.Fatal("Provide KMS server name, use --all, or use --group")
		}
	},
}

func snapshotSingleKmsServer(name string) {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	kmsList := vhttp.GetData[models.VbrKmsServerList]("kmsServers", profile)

	var found *models.VbrKmsServerGet
	for i := range kmsList.Data {
		if kmsList.Data[i].Name == name {
			found = &kmsList.Data[i]
			break
		}
	}

	if found == nil {
		log.Fatalf("KMS server '%s' not found in VBR.", name)
	}

	kBytes, err := json.Marshal(found)
	if err != nil {
		log.Fatalf("Failed to marshal KMS server data: %v", err)
	}

	if err := saveResourceToState("VBRKmsServer", found.Name, found.ID, json.RawMessage(kBytes)); err != nil {
		log.Fatalf("Failed to save KMS server state: %v", err)
	}

	fmt.Printf("Snapshot saved for KMS server: %s\n", name)
}

func snapshotAllKmsServers() {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	kmsList := vhttp.GetData[models.VbrKmsServerList]("kmsServers", profile)

	if len(kmsList.Data) == 0 {
		fmt.Println("No KMS servers found.")
		return
	}

	fmt.Printf("Snapshotting %d KMS servers...\n", len(kmsList.Data))

	// Build name counts to detect duplicates
	nameCounts := make(map[string]int, len(kmsList.Data))
	for _, k := range kmsList.Data {
		nameCounts[k.Name]++
	}

	for _, k := range kmsList.Data {
		kBytes, err := json.Marshal(k)
		if err != nil {
			fmt.Printf("Warning: Failed to marshal KMS server '%s': %v\n", k.Name, err)
			continue
		}

		resourceName := k.Name
		if nameCounts[k.Name] > 1 {
			resourceName = fmt.Sprintf("%s-%s", k.Name, k.ID)
		}

		if err := saveResourceToState("VBRKmsServer", resourceName, k.ID, json.RawMessage(kBytes)); err != nil {
			fmt.Printf("Warning: Failed to save state for '%s': %v\n", resourceName, err)
			continue
		}

		fmt.Printf("  Snapshot saved: %s\n", resourceName)
	}

	stateMgr := state.NewManager()
	fmt.Printf("\nState updated: %s\n", stateMgr.GetStatePath())
}

func diffSingleKmsServer(name string) {
	loadSeverityOverrides()
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	stateMgr := state.NewManager()
	resource, err := stateMgr.GetResource(name)
	if err != nil {
		log.Fatalf("KMS server '%s' not found in state. Has it been snapshotted?\n", name)
	}

	if resource.Type != "VBRKmsServer" {
		log.Fatalf("Resource '%s' is not a KMS server (type: %s).\n", name, resource.Type)
	}

	// Show (observed) label for monitored-only resources
	originLabel := ""
	if resource.Origin == "observed" {
		originLabel = " (observed)"
	}
	fmt.Printf("Checking drift for KMS server: %s%s\n\n", name, originLabel)

	// Fetch current from VBR
	kmsList := vhttp.GetData[models.VbrKmsServerList]("kmsServers", profile)

	var currentMap map[string]interface{}
	found := false
	for _, k := range kmsList.Data {
		if k.ID == resource.ID {
			kBytes, err := json.Marshal(k)
			if err != nil {
				log.Fatalf("Failed to marshal current KMS server data: %v", err)
			}
			if err := json.Unmarshal(kBytes, &currentMap); err != nil {
				log.Fatalf("Failed to unmarshal current KMS server data: %v", err)
			}
			found = true
			break
		}
	}

	if !found {
		fmt.Printf("CRITICAL - KMS server '%s' (ID: %s) has been removed from VBR!\n", name, resource.ID)
		os.Exit(4) // CRITICAL drift
	}

	// Compare, classify, filter
	drifts := detectDrift(resource.Spec, currentMap, kmsIgnoreFields)
	drifts = classifyDrifts(drifts, kmsSeverityMap)
	minSev := parseSeverityFlag()
	drifts = filterDriftsBySeverity(drifts, minSev)

	if len(drifts) == 0 {
		fmt.Println(noDriftMessage("KMS server matches snapshot state.", minSev))
		os.Exit(0)
	}

	printSecuritySummary(drifts)
	fmt.Println("Drift detected:")
	for _, drift := range drifts {
		printDriftWithSeverity(drift)
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("  - %d drifts detected\n", len(drifts))
	fmt.Printf("  - Highest severity: %s\n", getMaxSeverity(drifts))
	if resource.Origin == "applied" {
		fmt.Printf("  - Last applied: %s\n", resource.LastApplied.Format("2006-01-02 15:04:05"))
		fmt.Printf("  - Last applied by: %s\n", resource.LastAppliedBy)
	} else {
		fmt.Printf("  - Last snapshot: %s\n", resource.LastApplied.Format("2006-01-02 15:04:05"))
		fmt.Printf("  - Last snapshot by: %s\n", resource.LastAppliedBy)
	}

	// Show guidance based on origin
	printRemediationGuidance(BuildKmsGuidance(name, resource.Origin))

	os.Exit(exitCodeForDrifts(drifts))
}

func diffAllKmsServers() {
	loadSeverityOverrides()
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	stateMgr := state.NewManager()
	stateResources, err := stateMgr.ListResources("VBRKmsServer")
	if err != nil {
		log.Fatalf("Failed to load state: %v\n", err)
	}

	// Fetch current from VBR
	kmsList := vhttp.GetData[models.VbrKmsServerList]("kmsServers", profile)

	// Build lookup maps by ID
	stateByID := make(map[string]*state.Resource)
	for _, r := range stateResources {
		stateByID[r.ID] = r
	}

	currentByID := make(map[string]map[string]interface{})
	currentNameByID := make(map[string]string)
	for _, k := range kmsList.Data {
		kBytes, err := json.Marshal(k)
		if err != nil {
			fmt.Printf("Warning: Failed to marshal KMS server '%s': %v\n", k.Name, err)
			continue
		}
		var kMap map[string]interface{}
		if err := json.Unmarshal(kBytes, &kMap); err != nil {
			fmt.Printf("Warning: Failed to unmarshal KMS server '%s': %v\n", k.Name, err)
			continue
		}
		currentByID[k.ID] = kMap
		currentNameByID[k.ID] = k.Name
	}

	if len(stateResources) == 0 {
		fmt.Println("No KMS servers in state.")
		return
	}

	fmt.Printf("Checking KMS server inventory...\n")
	fmt.Printf("  State: %d servers, VBR: %d servers\n\n", len(stateResources), len(kmsList.Data))

	minSev := parseSeverityFlag()
	driftedCount := 0
	cleanCount := 0
	var allDrifts []Drift

	// Check for removed KMS servers (in state but not in VBR) — CRITICAL
	for id, stateRes := range stateByID {
		if _, exists := currentByID[id]; !exists {
			removedDrift := Drift{Path: "inventory", Action: "removed", State: stateRes.Name, Severity: SeverityCritical}
			if severityRank(SeverityCritical) >= severityRank(minSev) {
				fmt.Printf("  CRITICAL - %s (ID: %s): Removed from VBR\n", stateRes.Name, id)
				allDrifts = append(allDrifts, removedDrift)
				driftedCount++
			}
		}
	}

	// Check for added KMS servers (in VBR but not in state) — INFO
	for id, name := range currentNameByID {
		if _, exists := stateByID[id]; !exists {
			addedDrift := Drift{Path: "inventory", Action: "added", VBR: name, Severity: SeverityInfo}
			if severityRank(SeverityInfo) >= severityRank(minSev) {
				fmt.Printf("  INFO + %s (ID: %s): Added since last snapshot\n", name, id)
				allDrifts = append(allDrifts, addedDrift)
				driftedCount++
			}
		}
	}

	// Check field-level drift on matching servers
	driftedApplied := 0
	driftedObserved := 0
	for id, stateRes := range stateByID {
		if currentMap, exists := currentByID[id]; exists {
			drifts := detectDrift(stateRes.Spec, currentMap, kmsIgnoreFields)
			drifts = classifyDrifts(drifts, kmsSeverityMap)
			drifts = filterDriftsBySeverity(drifts, minSev)

			// Show origin label for observed resources
			originLabel := ""
			if stateRes.Origin == "observed" {
				originLabel = " (observed)"
			}

			if len(drifts) > 0 {
				fmt.Printf("  %s%s: %d drifts detected\n", stateRes.Name, originLabel, len(drifts))
				for _, d := range drifts {
					printDriftWithSeverity(d)
				}
				allDrifts = append(allDrifts, drifts...)
				if stateRes.Origin == "observed" {
					driftedObserved++
				} else {
					driftedApplied++
				}
				driftedCount++
			} else {
				fmt.Printf("  %s%s: No drift\n", stateRes.Name, originLabel)
				cleanCount++
			}
		}
	}

	if len(allDrifts) > 0 {
		printSecuritySummary(allDrifts)
	}
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  - %d KMS servers clean\n", cleanCount)
	if driftedApplied > 0 {
		fmt.Printf("  - %d KMS servers drifted — remediate with: owlctl encryption kms-apply <spec>.yaml\n", driftedApplied)
	}
	if driftedObserved > 0 {
		fmt.Printf("  - %d KMS servers drifted (observed, monitor-only) — export then apply a spec to enable remediation\n", driftedObserved)
	}
	// Also count inventory changes (added/removed) which were counted earlier
	inventoryChanges := driftedCount - driftedApplied - driftedObserved
	if inventoryChanges > 0 {
		fmt.Printf("  - %d inventory changes detected\n", inventoryChanges)
	}

	if driftedCount > 0 {
		os.Exit(exitCodeForDrifts(allDrifts))
	}
	os.Exit(0)
}

// --- Encryption Password Export command ---

var encExportCmd = &cobra.Command{
	Use:   "export [password-hint]",
	Short: "Export encryption password metadata to declarative YAML",
	Long: `Export existing VBR encryption password metadata to declarative YAML configuration files.

Only metadata (ID, hint, import status) is exported — never actual password values.

Examples:
  # Export single encryption password to stdout
  owlctl encryption export "My backup password"

  # Export single encryption password to file
  owlctl encryption export "My backup password" -o password.yaml

  # Export all encryption passwords to a directory
  owlctl encryption export --all -d specs/encryption/
`,
	Run: func(cmd *cobra.Command, args []string) {
		settings := utils.ReadSettings()
		profile := utils.GetCurrentProfile()

		if settings.SelectedProfile != "vbr" {
			log.Fatal("This command only works with VBR at the moment.")
		}

		cfg := ResourceExportConfig{
			Kind:            "VBREncryptionPassword",
			DisplayName:     "encryption password",
			PluralName:      "encryption passwords",
			IgnoreFields:    encryptionIgnoreFields,
			FetchSingle:     fetchEncryptionPasswordRaw,
			FetchByID:       fetchEncryptionPasswordByID,
			ListAll:         listAllEncryptionPasswords,
			SanitizeSpec:    sanitizeEncryptionPassword,
			SupportsOverlay: false,
		}

		if encExportAll {
			exportAllResources(cfg, profile, encExportDirectory, false, "")
		} else if len(args) > 0 {
			exportSingleResource(args[0], cfg, profile, encExportOutput, false, "")
		} else {
			log.Fatal("Provide password hint or use --all")
		}
	},
}

// --- KMS Server Export command ---

var kmsExportCmd = &cobra.Command{
	Use:   "kms-export [kms-name]",
	Short: "Export KMS server configuration to declarative YAML",
	Long: `Export existing VBR KMS server configurations to declarative YAML configuration files.

Examples:
  # Export single KMS server to stdout
  owlctl encryption kms-export "My KMS Server"

  # Export single KMS server to file
  owlctl encryption kms-export "My KMS Server" -o kms.yaml

  # Export all KMS servers to a directory
  owlctl encryption kms-export --all -d specs/kms/

  # Export as overlay (diff against a base file)
  owlctl encryption kms-export "My KMS Server" --as-overlay --base base-kms.yaml -o overlay.yaml
`,
	Run: func(cmd *cobra.Command, args []string) {
		settings := utils.ReadSettings()
		profile := utils.GetCurrentProfile()

		if settings.SelectedProfile != "vbr" {
			log.Fatal("This command only works with VBR at the moment.")
		}

		cfg := ResourceExportConfig{
			Kind:            "VBRKmsServer",
			DisplayName:     "KMS server",
			PluralName:      "KMS servers",
			IgnoreFields:    kmsIgnoreFields,
			FetchSingle:     fetchCurrentKmsServer,
			ListAll:         listAllKmsServers,
			SupportsOverlay: true,
		}

		if kmsExportAll {
			exportAllResources(cfg, profile, kmsExportDirectory, kmsExportAsOverlay, kmsExportBasePath)
		} else if len(args) > 0 {
			exportSingleResource(args[0], cfg, profile, kmsExportOutput, kmsExportAsOverlay, kmsExportBasePath)
		} else {
			log.Fatal("Provide KMS server name or use --all")
		}
	},
}

// fetchEncryptionPasswordRaw fetches an encryption password by hint and returns raw JSON
func fetchEncryptionPasswordRaw(hint string, profile models.Profile) (json.RawMessage, string, error) {
	passwordList := vhttp.GetData[models.VbrEncryptionPasswordList]("encryptionPasswords", profile)

	for _, p := range passwordList.Data {
		if p.Hint == hint {
			pBytes, err := json.Marshal(p)
			if err != nil {
				return nil, "", fmt.Errorf("failed to marshal password data: %w", err)
			}
			return json.RawMessage(pBytes), p.ID, nil
		}
	}

	return nil, "", nil // Not found
}

// fetchEncryptionPasswordByID fetches an encryption password by ID (used for bulk export
// where disambiguated names like "<hint>-<id>" don't match the raw hint field)
func fetchEncryptionPasswordByID(id string, profile models.Profile) (json.RawMessage, error) {
	passwordList := vhttp.GetData[models.VbrEncryptionPasswordList]("encryptionPasswords", profile)

	for _, p := range passwordList.Data {
		if p.ID == id {
			pBytes, err := json.Marshal(p)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal password data: %w", err)
			}
			return json.RawMessage(pBytes), nil
		}
	}

	return nil, nil // Not found
}

// listAllEncryptionPasswords returns all encryption passwords as ResourceListItems.
// Duplicate hints are disambiguated by appending the ID.
func listAllEncryptionPasswords(profile models.Profile) ([]ResourceListItem, error) {
	passwordList := vhttp.GetData[models.VbrEncryptionPasswordList]("encryptionPasswords", profile)

	// Count hints to detect duplicates
	hintCounts := make(map[string]int, len(passwordList.Data))
	for _, p := range passwordList.Data {
		hintCounts[p.Hint]++
	}

	items := make([]ResourceListItem, len(passwordList.Data))
	for i, p := range passwordList.Data {
		name := p.Hint
		if name == "" {
			name = p.ID
		} else if hintCounts[p.Hint] > 1 {
			name = fmt.Sprintf("%s-%s", name, p.ID)
		}
		items[i] = ResourceListItem{ID: p.ID, Name: name}
	}
	return items, nil
}

// sanitizeEncryptionPassword defensively strips any sensitive fields from the spec
func sanitizeEncryptionPassword(spec map[string]interface{}) {
	delete(spec, "password")
	delete(spec, "secret")
}

// listAllKmsServers returns all KMS servers as ResourceListItems
func listAllKmsServers(profile models.Profile) ([]ResourceListItem, error) {
	kmsList := vhttp.GetData[models.VbrKmsServerList]("kmsServers", profile)
	items := make([]ResourceListItem, len(kmsList.Data))
	for i, k := range kmsList.Data {
		items[i] = ResourceListItem{ID: k.ID, Name: k.Name}
	}
	return items, nil
}

// exportAllEncryptionToDir exports all encryption passwords to the specified directory.
// Registry-compatible: used by the global export command.
func exportAllEncryptionToDir(outDir string, profile models.Profile) error {
	cfg := ResourceExportConfig{
		Kind:            "VBREncryptionPassword",
		DisplayName:     "encryption password",
		PluralName:      "encryption passwords",
		IgnoreFields:    encryptionIgnoreFields,
		FetchSingle:     fetchEncryptionPasswordRaw,
		FetchByID:       fetchEncryptionPasswordByID,
		ListAll:         listAllEncryptionPasswords,
		SanitizeSpec:    sanitizeEncryptionPassword,
		SupportsOverlay: false,
	}
	exportAllResources(cfg, profile, outDir, false, "")
	return nil
}

// exportAllKmsToDir exports all KMS servers to the specified directory.
// Registry-compatible: used by the global export command.
func exportAllKmsToDir(outDir string, profile models.Profile) error {
	cfg := ResourceExportConfig{
		Kind:            "VBRKmsServer",
		DisplayName:     "KMS server",
		PluralName:      "KMS servers",
		IgnoreFields:    kmsIgnoreFields,
		FetchSingle:     fetchCurrentKmsServer,
		ListAll:         listAllKmsServers,
		SupportsOverlay: true,
	}
	exportAllResources(cfg, profile, outDir, false, "")
	return nil
}

// snapshotAllEncryptionForRegistry is a registry-compatible wrapper for snapshotAllEncryptionPasswords.
func snapshotAllEncryptionForRegistry(profile models.Profile) error {
	snapshotAllEncryptionPasswords()
	return nil
}

// snapshotAllKmsForRegistry is a registry-compatible wrapper for snapshotAllKmsServers.
func snapshotAllKmsForRegistry(profile models.Profile) error {
	snapshotAllKmsServers()
	return nil
}

func init() {
	// Encryption export flags
	encExportCmd.Flags().StringVarP(&encExportOutput, "output", "o", "", "Output file (default: stdout)")
	encExportCmd.Flags().StringVarP(&encExportDirectory, "directory", "d", "", "Output directory for bulk export")
	encExportCmd.Flags().BoolVar(&encExportAll, "all", false, "Export all encryption passwords")

	// KMS export flags
	kmsExportCmd.Flags().StringVarP(&kmsExportOutput, "output", "o", "", "Output file (default: stdout)")
	kmsExportCmd.Flags().StringVarP(&kmsExportDirectory, "directory", "d", "", "Output directory for bulk export")
	kmsExportCmd.Flags().BoolVar(&kmsExportAll, "all", false, "Export all KMS servers")
	kmsExportCmd.Flags().BoolVar(&kmsExportAsOverlay, "as-overlay", false, "Export as overlay (minimal patch)")
	kmsExportCmd.Flags().StringVar(&kmsExportBasePath, "base", "", "Base template to diff against (for overlay export)")

	encSnapshotCmd.Flags().BoolVar(&encSnapshotAll, "all", false, "Snapshot all encryption passwords")
	encDiffCmd.Flags().BoolVar(&encDiffAll, "all", false, "Check drift for all encryption passwords in state")
	addSeverityFlags(encDiffCmd)
	kmsSnapshotCmd.Flags().BoolVar(&kmsSnapshotAll, "all", false, "Snapshot all KMS servers")
	kmsDiffCmd.Flags().BoolVar(&kmsDiffAll, "all", false, "Check drift for all KMS servers in state")
	kmsDiffCmd.Flags().StringVar(&kmsDiffGroupName, "group", "", "Check drift for all specs in named group (from owlctl.yaml)")
	addSeverityFlags(kmsDiffCmd)
	kmsApplyCmd.Flags().BoolVar(&kmsApplyDryRun, "dry-run", false, "Preview changes without applying them")
	kmsApplyCmd.Flags().StringVar(&kmsApplyGroupName, "group", "", "Apply all specs in named group (from owlctl.yaml)")
	kmsApplyCmd.Flags().StringVar(&kmsApplyOverlayFile, "overlay", "", "Overlay file to merge with base configuration")

	encryptionCmd.AddCommand(encExportCmd)
	encryptionCmd.AddCommand(encSnapshotCmd)
	encryptionCmd.AddCommand(encDiffCmd)
	encryptionCmd.AddCommand(kmsExportCmd)
	encryptionCmd.AddCommand(kmsSnapshotCmd)
	encryptionCmd.AddCommand(kmsDiffCmd)
	encryptionCmd.AddCommand(kmsApplyCmd)
	rootCmd.AddCommand(encryptionCmd)
}
