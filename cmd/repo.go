package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/user"
	"time"

	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/state"
	"github.com/shapedthought/owlctl/utils"
	"github.com/shapedthought/owlctl/vhttp"
	"github.com/spf13/cobra"
)

var (
	repoSnapshotAll    bool
	repoDiffAll        bool
	repoApplyDryRun      bool
	repoApplyGroupName   string
	repoApplyOverlayFile string
	repoDiffGroupName    string
	sobrSnapshotAll      bool
	sobrDiffAll          bool
	sobrApplyDryRun      bool
	sobrApplyGroupName   string
	sobrApplyOverlayFile string
	sobrDiffGroupName    string

	// Export flags
	repoExportOutput    string
	repoExportDirectory string
	repoExportAll       bool
	repoExportAsOverlay bool
	repoExportBasePath  string
	sobrExportOutput    string
	sobrExportDirectory string
	sobrExportAll       bool
	sobrExportAsOverlay bool
	sobrExportBasePath  string
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Repository management commands",
	Long: `Repository related commands for state management and drift detection.

ONLY WORKS WITH VBR AT THE MOMENT.

Subcommands:

Export repository configuration to YAML
  owlctl repo export "Default Backup Repository"
  owlctl repo export --all -d specs/repos/

Snapshot repository configuration
  owlctl repo snapshot "Default Backup Repository"
  owlctl repo snapshot --all

Detect configuration drift
  owlctl repo diff "Default Backup Repository"
  owlctl repo diff --all

Apply repository configuration
  owlctl repo apply repos/default-repo.yaml

Scale-out backup repositories
  owlctl repo sobr-export "Scale-out Backup Repository 1"
  owlctl repo sobr-export --all -d specs/sobrs/
  owlctl repo sobr-snapshot "Scale-out Backup Repository 1"
  owlctl repo sobr-snapshot --all
  owlctl repo sobr-diff "Scale-out Backup Repository 1"
  owlctl repo sobr-diff --all
  owlctl repo sobr-apply sobrs/sobr1.yaml
`,
}

var repoSnapshotCmd = &cobra.Command{
	Use:   "snapshot [repo-name]",
	Short: "Snapshot repository configuration to state",
	Long: `Capture the current configuration of a backup repository and store it in state.

Examples:
  # Snapshot a single repository
  owlctl repo snapshot "Default Backup Repository"

  # Snapshot all repositories
  owlctl repo snapshot --all
`,
	Run: func(cmd *cobra.Command, args []string) {
		if repoSnapshotAll {
			snapshotAllRepos()
		} else if len(args) > 0 {
			snapshotSingleRepo(args[0])
		} else {
			log.Fatal("Provide repository name or use --all")
		}
	},
}

var repoDiffCmd = &cobra.Command{
	Use:   "diff [repo-name]",
	Short: "Detect configuration drift from snapshot state",
	Long: `Compare current VBR repository configuration against the last snapshot state
to detect manual changes or drift.

Examples:
  # Check single repository for drift
  owlctl repo diff "Default Backup Repository"

  # Check all repositories
  owlctl repo diff --all

Exit Codes:
  0 - No drift detected
  3 - Drift detected (INFO or WARNING)
  4 - Critical security drift detected
  1 - Error occurred`,
	Run: func(cmd *cobra.Command, args []string) {
		if repoDiffGroupName != "" {
			if repoDiffAll {
				log.Fatal("Cannot use --group with --all")
			}
			if len(args) > 0 {
				log.Fatal("Cannot use --group with a positional repository name argument")
			}
			diffGroupResource(repoDiffGroupName, GroupDiffConfig{
				Kind:         "VBRRepository",
				DisplayName:  "repository",
				PluralName:   "repositories",
				FetchCurrent: fetchCurrentRepo,
				IgnoreFields: repoIgnoreFields,
				SeverityMap:  repoSeverityMap,
				RemediateCmd: "owlctl repo apply --group %s",
			})
		} else if repoDiffAll {
			diffAllRepos()
		} else if len(args) > 0 {
			diffSingleRepo(args[0])
		} else {
			log.Fatal("Provide repository name, use --all, or use --group")
		}
	},
}

func snapshotSingleRepo(repoName string) {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	// Fetch all repositories and find by name
	repoList := vhttp.GetData[models.VbrRepoList]("backupInfrastructure/repositories", profile)

	var found *models.VbrRepoGet
	for i := range repoList.Data {
		if repoList.Data[i].Name == repoName {
			found = &repoList.Data[i]
			break
		}
	}

	if found == nil {
		log.Fatalf("Repository '%s' not found in VBR.", repoName)
	}

	// Fetch the individual repository for full details
	endpoint := fmt.Sprintf("backupInfrastructure/repositories/%s", found.ID)
	repoData := vhttp.GetData[json.RawMessage](endpoint, profile)

	if err := saveRepoToState(repoName, found.ID, repoData); err != nil {
		log.Fatalf("Failed to save repository state: %v", err)
	}

	fmt.Printf("Snapshot saved for repository: %s\n", repoName)
}

func snapshotAllRepos() {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	repoList := vhttp.GetData[models.VbrRepoList]("backupInfrastructure/repositories", profile)

	if len(repoList.Data) == 0 {
		fmt.Println("No repositories found.")
		return
	}

	fmt.Printf("Snapshotting %d repositories...\n", len(repoList.Data))

	for _, repo := range repoList.Data {
		// Fetch full details for each repo
		endpoint := fmt.Sprintf("backupInfrastructure/repositories/%s", repo.ID)
		repoData := vhttp.GetData[json.RawMessage](endpoint, profile)

		if err := saveRepoToState(repo.Name, repo.ID, repoData); err != nil {
			fmt.Printf("Warning: Failed to save state for '%s': %v\n", repo.Name, err)
			continue
		}

		fmt.Printf("  Snapshot saved: %s\n", repo.Name)
	}

	stateMgr := state.NewManager()
	fmt.Printf("\nState updated: %s\n", stateMgr.GetStatePath())
}

func saveRepoToState(name, id string, rawData json.RawMessage) error {
	return saveResourceToState("VBRRepository", name, id, rawData)
}

func diffSingleRepo(repoName string) {
	loadSeverityOverrides()
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	// Load from state
	stateMgr := state.NewManager()
	resource, err := stateMgr.GetResource(repoName)
	if err != nil {
		log.Fatalf("Repository '%s' not found in state. Has it been snapshotted?\n", repoName)
	}

	if resource.Type != "VBRRepository" {
		log.Fatalf("Resource '%s' is not a repository (type: %s).\n", repoName, resource.Type)
	}

	// Show (observed) label for monitored-only resources
	originLabel := ""
	if resource.Origin == "observed" {
		originLabel = " (observed)"
	}
	fmt.Printf("Checking drift for repository: %s%s\n\n", repoName, originLabel)

	// Fetch current from VBR
	endpoint := fmt.Sprintf("backupInfrastructure/repositories/%s", resource.ID)
	currentRaw := vhttp.GetData[json.RawMessage](endpoint, profile)

	var currentMap map[string]interface{}
	if err := json.Unmarshal(currentRaw, &currentMap); err != nil {
		log.Fatalf("Failed to unmarshal current repository data: %v", err)
	}

	// Compare, classify, filter
	drifts := detectDrift(resource.Spec, currentMap, repoIgnoreFields)
	drifts = classifyDrifts(drifts, repoSeverityMap)
	minSev := parseSeverityFlag()
	drifts = filterDriftsBySeverity(drifts, minSev)

	if len(drifts) == 0 {
		fmt.Println(noDriftMessage("Repository matches snapshot state.", minSev))
		os.Exit(0)
	}

	// Display drift
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
	printRemediationGuidance(BuildRepoGuidance(repoName, resource.Origin))

	os.Exit(exitCodeForDrifts(drifts))
}

func diffAllRepos() {
	loadSeverityOverrides()
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	stateMgr := state.NewManager()
	resources, err := stateMgr.ListResources("VBRRepository")
	if err != nil {
		log.Fatalf("Failed to load state: %v\n", err)
	}

	if len(resources) == 0 {
		fmt.Println("No repositories in state.")
		return
	}

	fmt.Printf("Checking %d repositories for drift...\n\n", len(resources))

	minSev := parseSeverityFlag()
	cleanCount := 0
	driftedApplied := 0
	driftedObserved := 0
	var allDrifts []Drift

	for _, resource := range resources {
		// Fetch current from VBR
		endpoint := fmt.Sprintf("backupInfrastructure/repositories/%s", resource.ID)
		currentRaw := vhttp.GetData[json.RawMessage](endpoint, profile)

		var currentMap map[string]interface{}
		if err := json.Unmarshal(currentRaw, &currentMap); err != nil {
			fmt.Printf("  %s: Failed to unmarshal repository data: %v\n", resource.Name, err)
			continue
		}

		// Detect, classify, filter
		drifts := detectDrift(resource.Spec, currentMap, repoIgnoreFields)
		drifts = classifyDrifts(drifts, repoSeverityMap)
		drifts = filterDriftsBySeverity(drifts, minSev)

		// Show origin label for observed resources
		originLabel := ""
		if resource.Origin == "observed" {
			originLabel = " (observed)"
		}

		if len(drifts) > 0 {
			maxSev := getMaxSeverity(drifts)
			fmt.Printf("  %s %s%s: %d drifts detected\n", maxSev, resource.Name, originLabel, len(drifts))
			allDrifts = append(allDrifts, drifts...)
			if resource.Origin == "observed" {
				driftedObserved++
			} else {
				driftedApplied++
			}
		} else {
			fmt.Printf("  %s%s: No drift\n", resource.Name, originLabel)
			cleanCount++
		}
	}

	if len(allDrifts) > 0 {
		printSecuritySummary(allDrifts)
	}
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  - %d repositories clean\n", cleanCount)
	if driftedApplied > 0 {
		fmt.Printf("  - %d repositories drifted — remediate with: owlctl repo apply <spec>.yaml\n", driftedApplied)
	}
	if driftedObserved > 0 {
		fmt.Printf("  - %d repositories drifted (observed, monitor-only) — export then apply a spec to enable remediation\n", driftedObserved)
	}

	totalDrifted := driftedApplied + driftedObserved
	if totalDrifted > 0 {
		os.Exit(exitCodeForDrifts(allDrifts))
	}
	os.Exit(0)
}

// --- Repository Apply command ---

// repoApplyConfig defines how to apply repository configurations
var repoApplyConfig = ResourceApplyConfig{
	Kind:         "VBRRepository",
	Endpoint:     "backupInfrastructure/repositories",
	IgnoreFields: repoIgnoreFields,
	Mode:         ApplyUpdateOnly,
	FetchCurrent: fetchCurrentRepo,
}

// fetchCurrentRepo retrieves a repository by name from VBR
func fetchCurrentRepo(name string, profile models.Profile) (json.RawMessage, string, error) {
	repoList := vhttp.GetData[models.VbrRepoList]("backupInfrastructure/repositories", profile)

	for _, repo := range repoList.Data {
		if repo.Name == name {
			// Fetch full details
			endpoint := fmt.Sprintf("backupInfrastructure/repositories/%s", repo.ID)
			repoData := vhttp.GetData[json.RawMessage](endpoint, profile)
			return repoData, repo.ID, nil
		}
	}

	return nil, "", nil // Not found (not an error)
}

var repoApplyCmd = &cobra.Command{
	Use:   "apply [spec-file]",
	Short: "Apply a repository configuration to VBR",
	Long: `Apply a declarative repository configuration to VBR.

This command updates an existing repository with the configuration from a YAML spec file.
Repositories cannot be created via the API - they must be created in the VBR console first.

Examples:
  # Apply a repository configuration
  owlctl repo apply repos/default-repo.yaml

  # Preview changes without applying (dry-run)
  owlctl repo apply repos/default-repo.yaml --dry-run

Exit Codes:
  0 - Success
  1 - Error (API failure, invalid spec)
  6 - Resource not found (repository doesn't exist in VBR)
`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if repoApplyGroupName != "" {
			if len(args) > 0 {
				log.Fatal("Cannot use --group with a positional spec file argument")
			}
			if repoApplyOverlayFile != "" {
				log.Fatal("Cannot use --group with --overlay (group defines its own overlay)")
			}
			applyGroupResource(repoApplyGroupName, repoApplyConfig, repoApplyDryRun)
		} else if len(args) > 0 {
			settings := utils.ReadSettings()
			profile := utils.GetCurrentProfile()

			if settings.SelectedProfile != "vbr" {
				log.Fatal("This command only works with VBR at the moment.")
			}

			result := applyWithOptionalOverlay(args[0], repoApplyOverlayFile, repoApplyConfig, profile, repoApplyDryRun)
			if result.Error != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", result.Error)
				outcome := DetermineApplyOutcome([]ApplyResult{result})
				os.Exit(ExitCodeForOutcome(outcome))
			}

			if result.DryRun {
				return // Dry-run output already printed
			}

			fmt.Printf("\nSuccessfully %s repository: %s\n", result.Action, result.ResourceName)
		} else {
			log.Fatal("Provide a spec file or use --group")
		}
	},
}

// --- Scale-Out Backup Repository commands ---

// sobrApplyConfig defines how to apply SOBR configurations
var sobrApplyConfig = ResourceApplyConfig{
	Kind:         "VBRScaleOutRepository",
	Endpoint:     "backupInfrastructure/scaleOutRepositories",
	IgnoreFields: sobrIgnoreFields,
	Mode:         ApplyUpdateOnly,
	FetchCurrent: fetchCurrentSobr,
}

// fetchCurrentSobr retrieves a SOBR by name from VBR
func fetchCurrentSobr(name string, profile models.Profile) (json.RawMessage, string, error) {
	sobrList := vhttp.GetData[models.VbrSobrList]("backupInfrastructure/scaleOutRepositories", profile)

	for _, sobr := range sobrList.Data {
		if sobr.Name == name {
			// Fetch full details
			endpoint := fmt.Sprintf("backupInfrastructure/scaleOutRepositories/%s", sobr.ID)
			sobrData := vhttp.GetData[json.RawMessage](endpoint, profile)
			return sobrData, sobr.ID, nil
		}
	}

	return nil, "", nil // Not found (not an error)
}

var sobrApplyCmd = &cobra.Command{
	Use:   "sobr-apply [spec-file]",
	Short: "Apply a scale-out repository configuration to VBR",
	Long: `Apply a declarative scale-out repository configuration to VBR.

This command updates an existing SOBR with the configuration from a YAML spec file.
SOBRs cannot be created via the API - they must be created in the VBR console first.

Examples:
  # Apply a SOBR configuration
  owlctl repo sobr-apply sobrs/sobr1.yaml

  # Preview changes without applying (dry-run)
  owlctl repo sobr-apply sobrs/sobr1.yaml --dry-run

Exit Codes:
  0 - Success
  1 - Error (API failure, invalid spec)
  6 - Resource not found (SOBR doesn't exist in VBR)
`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if sobrApplyGroupName != "" {
			if len(args) > 0 {
				log.Fatal("Cannot use --group with a positional spec file argument")
			}
			if sobrApplyOverlayFile != "" {
				log.Fatal("Cannot use --group with --overlay (group defines its own overlay)")
			}
			applyGroupResource(sobrApplyGroupName, sobrApplyConfig, sobrApplyDryRun)
		} else if len(args) > 0 {
			settings := utils.ReadSettings()
			profile := utils.GetCurrentProfile()

			if settings.SelectedProfile != "vbr" {
				log.Fatal("This command only works with VBR at the moment.")
			}

			result := applyWithOptionalOverlay(args[0], sobrApplyOverlayFile, sobrApplyConfig, profile, sobrApplyDryRun)
			if result.Error != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", result.Error)
				outcome := DetermineApplyOutcome([]ApplyResult{result})
				os.Exit(ExitCodeForOutcome(outcome))
			}

			if result.DryRun {
				return // Dry-run output already printed
			}

			fmt.Printf("\nSuccessfully %s scale-out repository: %s\n", result.Action, result.ResourceName)
		} else {
			log.Fatal("Provide a spec file or use --group")
		}
	},
}

var sobrSnapshotCmd = &cobra.Command{
	Use:   "sobr-snapshot [sobr-name]",
	Short: "Snapshot scale-out backup repository configuration to state",
	Long: `Capture the current configuration of a scale-out backup repository and store it in state.

Examples:
  # Snapshot a single SOBR
  owlctl repo sobr-snapshot "Scale-out Backup Repository 1"

  # Snapshot all SOBRs
  owlctl repo sobr-snapshot --all
`,
	Run: func(cmd *cobra.Command, args []string) {
		if sobrSnapshotAll {
			snapshotAllSobrs()
		} else if len(args) > 0 {
			snapshotSingleSobr(args[0])
		} else {
			log.Fatal("Provide SOBR name or use --all")
		}
	},
}

var sobrDiffCmd = &cobra.Command{
	Use:   "sobr-diff [sobr-name]",
	Short: "Detect configuration drift for scale-out backup repositories",
	Long: `Compare current VBR scale-out backup repository configuration against the
last snapshot state to detect manual changes or drift.

Examples:
  # Check single SOBR for drift
  owlctl repo sobr-diff "Scale-out Backup Repository 1"

  # Check all SOBRs
  owlctl repo sobr-diff --all

Exit Codes:
  0 - No drift detected
  3 - Drift detected (INFO or WARNING)
  4 - Critical security drift detected
  1 - Error occurred`,
	Run: func(cmd *cobra.Command, args []string) {
		if sobrDiffGroupName != "" {
			if sobrDiffAll {
				log.Fatal("Cannot use --group with --all")
			}
			if len(args) > 0 {
				log.Fatal("Cannot use --group with a positional SOBR name argument")
			}
			diffGroupResource(sobrDiffGroupName, GroupDiffConfig{
				Kind:         "VBRScaleOutRepository",
				DisplayName:  "scale-out repository",
				PluralName:   "scale-out repositories",
				FetchCurrent: fetchCurrentSobr,
				IgnoreFields: sobrIgnoreFields,
				SeverityMap:  sobrSeverityMap,
				RemediateCmd: "owlctl repo sobr-apply --group %s",
			})
		} else if sobrDiffAll {
			diffAllSobrs()
		} else if len(args) > 0 {
			diffSingleSobr(args[0])
		} else {
			log.Fatal("Provide SOBR name, use --all, or use --group")
		}
	},
}

func snapshotSingleSobr(sobrName string) {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	sobrList := vhttp.GetData[models.VbrSobrList]("backupInfrastructure/scaleOutRepositories", profile)

	var found *models.VbrSobrGet
	for i := range sobrList.Data {
		if sobrList.Data[i].Name == sobrName {
			found = &sobrList.Data[i]
			break
		}
	}

	if found == nil {
		log.Fatalf("Scale-out repository '%s' not found in VBR.", sobrName)
	}

	endpoint := fmt.Sprintf("backupInfrastructure/scaleOutRepositories/%s", found.ID)
	sobrData := vhttp.GetData[json.RawMessage](endpoint, profile)

	if err := saveResourceToState("VBRScaleOutRepository", sobrName, found.ID, sobrData); err != nil {
		log.Fatalf("Failed to save SOBR state: %v", err)
	}

	fmt.Printf("Snapshot saved for scale-out repository: %s\n", sobrName)
}

func snapshotAllSobrs() {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	sobrList := vhttp.GetData[models.VbrSobrList]("backupInfrastructure/scaleOutRepositories", profile)

	if len(sobrList.Data) == 0 {
		fmt.Println("No scale-out repositories found.")
		return
	}

	fmt.Printf("Snapshotting %d scale-out repositories...\n", len(sobrList.Data))

	for _, sobr := range sobrList.Data {
		endpoint := fmt.Sprintf("backupInfrastructure/scaleOutRepositories/%s", sobr.ID)
		sobrData := vhttp.GetData[json.RawMessage](endpoint, profile)

		if err := saveResourceToState("VBRScaleOutRepository", sobr.Name, sobr.ID, sobrData); err != nil {
			fmt.Printf("Warning: Failed to save state for '%s': %v\n", sobr.Name, err)
			continue
		}

		fmt.Printf("  Snapshot saved: %s\n", sobr.Name)
	}

	stateMgr := state.NewManager()
	fmt.Printf("\nState updated: %s\n", stateMgr.GetStatePath())
}

func diffSingleSobr(sobrName string) {
	loadSeverityOverrides()
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	stateMgr := state.NewManager()
	resource, err := stateMgr.GetResource(sobrName)
	if err != nil {
		log.Fatalf("Scale-out repository '%s' not found in state. Has it been snapshotted?\n", sobrName)
	}

	if resource.Type != "VBRScaleOutRepository" {
		log.Fatalf("Resource '%s' is not a scale-out repository (type: %s).\n", sobrName, resource.Type)
	}

	// Show (observed) label for monitored-only resources
	originLabel := ""
	if resource.Origin == "observed" {
		originLabel = " (observed)"
	}
	fmt.Printf("Checking drift for scale-out repository: %s%s\n\n", sobrName, originLabel)

	endpoint := fmt.Sprintf("backupInfrastructure/scaleOutRepositories/%s", resource.ID)
	currentRaw := vhttp.GetData[json.RawMessage](endpoint, profile)

	var currentMap map[string]interface{}
	if err := json.Unmarshal(currentRaw, &currentMap); err != nil {
		log.Fatalf("Failed to unmarshal current SOBR data: %v", err)
	}

	// Compare, classify, filter
	drifts := detectDrift(resource.Spec, currentMap, sobrIgnoreFields)
	drifts = classifyDrifts(drifts, sobrSeverityMap)
	minSev := parseSeverityFlag()
	drifts = filterDriftsBySeverity(drifts, minSev)

	if len(drifts) == 0 {
		fmt.Println(noDriftMessage("Scale-out repository matches snapshot state.", minSev))
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
	printRemediationGuidance(BuildSobrGuidance(sobrName, resource.Origin))

	os.Exit(exitCodeForDrifts(drifts))
}

func diffAllSobrs() {
	loadSeverityOverrides()
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	stateMgr := state.NewManager()
	resources, err := stateMgr.ListResources("VBRScaleOutRepository")
	if err != nil {
		log.Fatalf("Failed to load state: %v\n", err)
	}

	if len(resources) == 0 {
		fmt.Println("No scale-out repositories in state.")
		return
	}

	fmt.Printf("Checking %d scale-out repositories for drift...\n\n", len(resources))

	minSev := parseSeverityFlag()
	cleanCount := 0
	driftedApplied := 0
	driftedObserved := 0
	var allDrifts []Drift

	for _, resource := range resources {
		endpoint := fmt.Sprintf("backupInfrastructure/scaleOutRepositories/%s", resource.ID)
		currentRaw := vhttp.GetData[json.RawMessage](endpoint, profile)

		var currentMap map[string]interface{}
		if err := json.Unmarshal(currentRaw, &currentMap); err != nil {
			fmt.Printf("  %s: Failed to unmarshal SOBR data: %v\n", resource.Name, err)
			continue
		}

		// Detect, classify, filter
		drifts := detectDrift(resource.Spec, currentMap, sobrIgnoreFields)
		drifts = classifyDrifts(drifts, sobrSeverityMap)
		drifts = filterDriftsBySeverity(drifts, minSev)

		// Show origin label for observed resources
		originLabel := ""
		if resource.Origin == "observed" {
			originLabel = " (observed)"
		}

		if len(drifts) > 0 {
			maxSev := getMaxSeverity(drifts)
			fmt.Printf("  %s %s%s: %d drifts detected\n", maxSev, resource.Name, originLabel, len(drifts))
			allDrifts = append(allDrifts, drifts...)
			if resource.Origin == "observed" {
				driftedObserved++
			} else {
				driftedApplied++
			}
		} else {
			fmt.Printf("  %s%s: No drift\n", resource.Name, originLabel)
			cleanCount++
		}
	}

	if len(allDrifts) > 0 {
		printSecuritySummary(allDrifts)
	}
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  - %d scale-out repositories clean\n", cleanCount)
	if driftedApplied > 0 {
		fmt.Printf("  - %d scale-out repositories drifted — remediate with: owlctl repo sobr-apply <spec>.yaml\n", driftedApplied)
	}
	if driftedObserved > 0 {
		fmt.Printf("  - %d scale-out repositories drifted (observed, monitor-only) — export then apply a spec to enable remediation\n", driftedObserved)
	}

	totalDrifted := driftedApplied + driftedObserved
	if totalDrifted > 0 {
		os.Exit(exitCodeForDrifts(allDrifts))
	}
	os.Exit(0)
}

// saveResourceToState is a shared helper for saving any resource type to state
func saveResourceToState(resourceType, name, id string, rawData json.RawMessage) error {
	var spec map[string]interface{}
	if err := json.Unmarshal(rawData, &spec); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}

	stateMgr := state.NewManager()

	currentUser := "unknown"
	if usr, err := user.Current(); err == nil {
		currentUser = usr.Username
	}

	// Try to load existing resource to preserve history
	var existingHistory []state.ResourceEvent
	if existing, err := stateMgr.GetResource(name); err == nil {
		existingHistory = existing.History
	}

	resource := &state.Resource{
		Type:          resourceType,
		ID:            id,
		Name:          name,
		LastApplied:   time.Now(),
		LastAppliedBy: currentUser,
		Origin:        "observed",
		Spec:          spec,
		History:       existingHistory,
	}

	// Record snapshot event
	resource.AddEvent(state.NewEvent("snapshotted", currentUser))

	if err := stateMgr.UpdateResource(resource); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	return nil
}

// --- Repository Export commands ---

var repoExportCmd = &cobra.Command{
	Use:   "export [repo-name]",
	Short: "Export repository configuration to declarative YAML",
	Long: `Export existing VBR backup repositories to declarative YAML configuration files.

Examples:
  # Export single repository to stdout
  owlctl repo export "Default Backup Repository"

  # Export single repository to file
  owlctl repo export "Default Backup Repository" -o repo.yaml

  # Export all repositories to a directory
  owlctl repo export --all -d specs/repos/

  # Export as overlay (diff against a base file)
  owlctl repo export "Default Backup Repository" --as-overlay --base base-repo.yaml -o overlay.yaml
`,
	Run: func(cmd *cobra.Command, args []string) {
		settings := utils.ReadSettings()
		profile := utils.GetCurrentProfile()

		if settings.SelectedProfile != "vbr" {
			log.Fatal("This command only works with VBR at the moment.")
		}

		cfg := ResourceExportConfig{
			Kind:            "VBRRepository",
			DisplayName:     "repository",
			PluralName:      "repositories",
			IgnoreFields:    repoIgnoreFields,
			FetchSingle:     fetchCurrentRepo,
			FetchByID:       fetchRepoByID,
			ListAll:         listAllRepos,
			SupportsOverlay: true,
		}

		if repoExportAll {
			exportAllResources(cfg, profile, repoExportDirectory, repoExportAsOverlay, repoExportBasePath)
		} else if len(args) > 0 {
			exportSingleResource(args[0], cfg, profile, repoExportOutput, repoExportAsOverlay, repoExportBasePath)
		} else {
			log.Fatal("Provide repository name or use --all")
		}
	},
}

var sobrExportCmd = &cobra.Command{
	Use:   "sobr-export [sobr-name]",
	Short: "Export scale-out repository configuration to declarative YAML",
	Long: `Export existing VBR scale-out backup repositories to declarative YAML configuration files.

Examples:
  # Export single SOBR to stdout
  owlctl repo sobr-export "Scale-out Backup Repository 1"

  # Export single SOBR to file
  owlctl repo sobr-export "Scale-out Backup Repository 1" -o sobr.yaml

  # Export all SOBRs to a directory
  owlctl repo sobr-export --all -d specs/sobrs/

  # Export as overlay (diff against a base file)
  owlctl repo sobr-export "Scale-out Backup Repository 1" --as-overlay --base base-sobr.yaml -o overlay.yaml
`,
	Run: func(cmd *cobra.Command, args []string) {
		settings := utils.ReadSettings()
		profile := utils.GetCurrentProfile()

		if settings.SelectedProfile != "vbr" {
			log.Fatal("This command only works with VBR at the moment.")
		}

		cfg := ResourceExportConfig{
			Kind:            "VBRScaleOutRepository",
			DisplayName:     "scale-out repository",
			PluralName:      "scale-out repositories",
			IgnoreFields:    sobrIgnoreFields,
			FetchSingle:     fetchCurrentSobr,
			FetchByID:       fetchSobrByID,
			ListAll:         listAllSobrs,
			SupportsOverlay: true,
		}

		if sobrExportAll {
			exportAllResources(cfg, profile, sobrExportDirectory, sobrExportAsOverlay, sobrExportBasePath)
		} else if len(args) > 0 {
			exportSingleResource(args[0], cfg, profile, sobrExportOutput, sobrExportAsOverlay, sobrExportBasePath)
		} else {
			log.Fatal("Provide SOBR name or use --all")
		}
	},
}

// fetchRepoByID retrieves a repository by ID (used for bulk export)
func fetchRepoByID(id string, profile models.Profile) (json.RawMessage, error) {
	endpoint := fmt.Sprintf("backupInfrastructure/repositories/%s", id)
	repoData := vhttp.GetData[json.RawMessage](endpoint, profile)
	return repoData, nil
}

// fetchSobrByID retrieves a SOBR by ID (used for bulk export)
func fetchSobrByID(id string, profile models.Profile) (json.RawMessage, error) {
	endpoint := fmt.Sprintf("backupInfrastructure/scaleOutRepositories/%s", id)
	sobrData := vhttp.GetData[json.RawMessage](endpoint, profile)
	return sobrData, nil
}

// listAllRepos returns all repositories as ResourceListItems
func listAllRepos(profile models.Profile) ([]ResourceListItem, error) {
	repoList := vhttp.GetData[models.VbrRepoList]("backupInfrastructure/repositories", profile)
	items := make([]ResourceListItem, len(repoList.Data))
	for i, repo := range repoList.Data {
		items[i] = ResourceListItem{ID: repo.ID, Name: repo.Name}
	}
	return items, nil
}

// listAllSobrs returns all SOBRs as ResourceListItems
func listAllSobrs(profile models.Profile) ([]ResourceListItem, error) {
	sobrList := vhttp.GetData[models.VbrSobrList]("backupInfrastructure/scaleOutRepositories", profile)
	items := make([]ResourceListItem, len(sobrList.Data))
	for i, sobr := range sobrList.Data {
		items[i] = ResourceListItem{ID: sobr.ID, Name: sobr.Name}
	}
	return items, nil
}

// exportAllReposToDir exports all repositories to the specified directory.
// Registry-compatible: used by the global export command.
func exportAllReposToDir(outDir string, profile models.Profile) error {
	cfg := ResourceExportConfig{
		Kind:            "VBRRepository",
		DisplayName:     "repository",
		PluralName:      "repositories",
		IgnoreFields:    repoIgnoreFields,
		FetchSingle:     fetchCurrentRepo,
		FetchByID:       fetchRepoByID,
		ListAll:         listAllRepos,
		SupportsOverlay: true,
	}
	exportAllResources(cfg, profile, outDir, false, "")
	return nil
}

// exportAllSobrsToDir exports all SOBRs to the specified directory.
// Registry-compatible: used by the global export command.
func exportAllSobrsToDir(outDir string, profile models.Profile) error {
	cfg := ResourceExportConfig{
		Kind:            "VBRScaleOutRepository",
		DisplayName:     "scale-out repository",
		PluralName:      "scale-out repositories",
		IgnoreFields:    sobrIgnoreFields,
		FetchSingle:     fetchCurrentSobr,
		FetchByID:       fetchSobrByID,
		ListAll:         listAllSobrs,
		SupportsOverlay: true,
	}
	exportAllResources(cfg, profile, outDir, false, "")
	return nil
}

// snapshotAllReposForRegistry is a registry-compatible wrapper for snapshotAllRepos.
func snapshotAllReposForRegistry(profile models.Profile) error {
	snapshotAllRepos()
	return nil
}

// snapshotAllSobrsForRegistry is a registry-compatible wrapper for snapshotAllSobrs.
func snapshotAllSobrsForRegistry(profile models.Profile) error {
	snapshotAllSobrs()
	return nil
}

func init() {
	// Repo export flags
	repoExportCmd.Flags().StringVarP(&repoExportOutput, "output", "o", "", "Output file (default: stdout)")
	repoExportCmd.Flags().StringVarP(&repoExportDirectory, "directory", "d", "", "Output directory for bulk export")
	repoExportCmd.Flags().BoolVar(&repoExportAll, "all", false, "Export all repositories")
	repoExportCmd.Flags().BoolVar(&repoExportAsOverlay, "as-overlay", false, "Export as overlay (minimal patch)")
	repoExportCmd.Flags().StringVar(&repoExportBasePath, "base", "", "Base template to diff against (for overlay export)")

	// SOBR export flags
	sobrExportCmd.Flags().StringVarP(&sobrExportOutput, "output", "o", "", "Output file (default: stdout)")
	sobrExportCmd.Flags().StringVarP(&sobrExportDirectory, "directory", "d", "", "Output directory for bulk export")
	sobrExportCmd.Flags().BoolVar(&sobrExportAll, "all", false, "Export all scale-out repositories")
	sobrExportCmd.Flags().BoolVar(&sobrExportAsOverlay, "as-overlay", false, "Export as overlay (minimal patch)")
	sobrExportCmd.Flags().StringVar(&sobrExportBasePath, "base", "", "Base template to diff against (for overlay export)")

	repoSnapshotCmd.Flags().BoolVar(&repoSnapshotAll, "all", false, "Snapshot all repositories")
	repoDiffCmd.Flags().BoolVar(&repoDiffAll, "all", false, "Check drift for all repositories in state")
	repoDiffCmd.Flags().StringVar(&repoDiffGroupName, "group", "", "Check drift for all specs in named group (from owlctl.yaml)")
	addSeverityFlags(repoDiffCmd)
	repoApplyCmd.Flags().BoolVar(&repoApplyDryRun, "dry-run", false, "Preview changes without applying them")
	repoApplyCmd.Flags().StringVar(&repoApplyGroupName, "group", "", "Apply all specs in named group (from owlctl.yaml)")
	repoApplyCmd.Flags().StringVar(&repoApplyOverlayFile, "overlay", "", "Overlay file to merge with base configuration")

	sobrSnapshotCmd.Flags().BoolVar(&sobrSnapshotAll, "all", false, "Snapshot all scale-out repositories")
	sobrDiffCmd.Flags().BoolVar(&sobrDiffAll, "all", false, "Check drift for all scale-out repositories in state")
	sobrDiffCmd.Flags().StringVar(&sobrDiffGroupName, "group", "", "Check drift for all specs in named group (from owlctl.yaml)")
	addSeverityFlags(sobrDiffCmd)
	sobrApplyCmd.Flags().BoolVar(&sobrApplyDryRun, "dry-run", false, "Preview changes without applying them")
	sobrApplyCmd.Flags().StringVar(&sobrApplyGroupName, "group", "", "Apply all specs in named group (from owlctl.yaml)")
	sobrApplyCmd.Flags().StringVar(&sobrApplyOverlayFile, "overlay", "", "Overlay file to merge with base configuration")

	repoCmd.AddCommand(repoExportCmd)
	repoCmd.AddCommand(repoSnapshotCmd)
	repoCmd.AddCommand(repoDiffCmd)
	repoCmd.AddCommand(repoApplyCmd)
	repoCmd.AddCommand(sobrExportCmd)
	repoCmd.AddCommand(sobrSnapshotCmd)
	repoCmd.AddCommand(sobrDiffCmd)
	repoCmd.AddCommand(sobrApplyCmd)
	rootCmd.AddCommand(repoCmd)
}
