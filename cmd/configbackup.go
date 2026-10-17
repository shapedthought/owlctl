package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/resources"
	"github.com/shapedthought/owlctl/state"
	"github.com/shapedthought/owlctl/utils"
	"github.com/shapedthought/owlctl/vhttp"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// configBackupStateKey is the fixed state key for the singleton configuration backup resource.
const configBackupStateKey = "ConfigurationBackup"

// configBackupEndpoint is the VBR API endpoint for configuration backup settings.
const configBackupEndpoint = "configBackup"

// configBackupExportFile is the fixed file name written by 'export --all'.
const configBackupExportFile = "configuration-backup.yaml"

// fetchConfigBackup retrieves the live configuration backup settings. It is a
// variable so tests can substitute a mock instead of calling VBR.
var fetchConfigBackup = func(profile models.Profile) json.RawMessage {
	return vhttp.GetData[json.RawMessage](configBackupEndpoint, profile)
}

var (
	configBackupExportOutput string
	configBackupApplyOverlay string
	configBackupApplyDryRun  bool
	configBackupApplyOutput  string
	configBackupDiffSeverity string
	configBackupSecurityOnly bool
)

// configBackupCmd is the parent command for configuration backup management.
var configBackupCmd = &cobra.Command{
	Use:   "config-backup",
	Short: "VBR configuration backup settings management",
	Long: `Manage VBR configuration backup settings using declarative state.

Configuration backup is a singleton resource — there is one set of settings per VBR server.
No name or --all flag is required.

Subcommands:

  Snapshot current settings to state:
    owlctl config-backup snapshot

  Detect drift between state and live VBR:
    owlctl config-backup diff
    owlctl config-backup diff --severity warning
    owlctl config-backup diff --security-only

  Export current settings to YAML:
    owlctl config-backup export
    owlctl config-backup export -o config-backup.yaml

  Apply settings from a YAML file:
    owlctl config-backup apply config-backup.yaml
    owlctl config-backup apply config-backup.yaml --dry-run
    owlctl config-backup apply config-backup.yaml --overlay prod-overlay.yaml
`,
}

// ---- snapshot ----------------------------------------------------------------

var configBackupSnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Snapshot configuration backup settings to state",
	Long:  `Fetches the current VBR configuration backup settings and saves them to state.json.`,
	Run: func(cmd *cobra.Command, args []string) {
		snapshotConfigBackup()
	},
}

func snapshotConfigBackup() {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	rawData := fetchConfigBackup(profile)

	if err := saveConfigBackupToState(rawData); err != nil {
		log.Fatalf("Failed to save configuration backup state: %v", err)
	}

	stateMgr := state.NewManager()
	fmt.Printf("Configuration backup settings snapshot saved.\nState: %s\n", stateMgr.GetStatePath())
}

func saveConfigBackupToState(rawData json.RawMessage) error {
	var spec map[string]interface{}
	if err := json.Unmarshal(rawData, &spec); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}

	stateMgr := state.NewManager()

	currentUser := "unknown"
	if usr, err := user.Current(); err == nil {
		currentUser = usr.Username
	}

	var existingHistory []state.ResourceEvent
	if existing, err := stateMgr.GetResource(configBackupStateKey); err == nil {
		existingHistory = existing.History
	}

	resource := &state.Resource{
		Type:          resources.KindVBRConfigurationBackup,
		ID:            configBackupEndpoint,
		Name:          configBackupStateKey,
		LastApplied:   time.Now(),
		LastAppliedBy: currentUser,
		Origin:        "observed",
		Spec:          spec,
		History:       existingHistory,
	}

	resource.AddEvent(state.NewEvent("snapshotted", currentUser))

	if err := stateMgr.UpdateResource(resource); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	return nil
}

// ---- diff --------------------------------------------------------------------

var configBackupDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Detect drift in configuration backup settings",
	Long: `Compares the snapshotted configuration backup settings in state against the live VBR configuration.

Exit codes:
  0 = No drift
  3 = Drift detected (INFO or WARNING)
  4 = Critical drift detected
  1 = Error
`,
	Run: func(cmd *cobra.Command, args []string) {
		diffConfigBackup()
	},
}

func diffConfigBackup() {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	stateMgr := state.NewManager()
	stateEntry, err := stateMgr.GetResource(configBackupStateKey)
	if err != nil {
		fmt.Println("No snapshot found for configuration backup. Run 'owlctl config-backup snapshot' first.")
		os.Exit(ExitError)
	}

	if stateEntry.Type != resources.KindVBRConfigurationBackup {
		log.Fatalf("State entry '%s' is not a configuration backup resource (type: %s).", configBackupStateKey, stateEntry.Type)
	}

	rawData := vhttp.GetData[json.RawMessage](configBackupEndpoint, profile)

	var liveSpec map[string]interface{}
	if err := json.Unmarshal(rawData, &liveSpec); err != nil {
		log.Fatalf("Failed to parse configuration backup response: %v", err)
	}

	drifts := detectDrift(stateEntry.Spec, liveSpec, configBackupIgnoreFields)
	drifts = classifyDrifts(drifts, configBackupSeverityMap)

	minSev := SeverityInfo
	if configBackupSecurityOnly {
		minSev = SeverityWarning
	} else if configBackupDiffSeverity != "" {
		switch configBackupDiffSeverity {
		case "critical":
			minSev = SeverityCritical
		case "warning":
			minSev = SeverityWarning
		}
	}

	drifts = filterDriftsBySeverity(drifts, minSev)

	if len(drifts) == 0 {
		fmt.Println(noDriftMessage("Configuration backup settings match state.", minSev))
		os.Exit(ExitSuccess)
	}

	fmt.Println("Configuration Backup Settings drift:")
	for _, d := range drifts {
		printDriftWithSeverity(d)
	}

	os.Exit(exitCodeForDrifts(drifts))
}

// ---- export ------------------------------------------------------------------

var configBackupExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export configuration backup settings to YAML",
	Long:  `Fetches the current VBR configuration backup settings and exports them as a declarative YAML spec.`,
	Run: func(cmd *cobra.Command, args []string) {
		exportConfigBackup()
	},
}

// buildConfigBackupYAML converts raw API data into the declarative YAML spec
// (with runtime fields stripped) written by 'config-backup export'.
func buildConfigBackupYAML(rawData json.RawMessage) ([]byte, error) {
	var spec map[string]interface{}
	if err := json.Unmarshal(rawData, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse configuration backup response: %w", err)
	}

	// Strip runtime fields before export
	for field := range configBackupIgnoreFields {
		delete(spec, field)
	}

	resourceSpec := resources.ResourceSpec{
		APIVersion: "owlctl.veeam.com/v1",
		Kind:       resources.KindVBRConfigurationBackup,
		Metadata: resources.Metadata{
			Name: configBackupStateKey,
		},
		Spec: spec,
	}

	header := "# VBR Configuration Backup Settings\n# Singleton resource — one per VBR server\n#\n# Apply with: owlctl config-backup apply <file>\n\n"

	yamlBytes, err := yaml.Marshal(resourceSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to YAML: %w", err)
	}

	return append([]byte(header), yamlBytes...), nil
}

func exportConfigBackup() {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	output, err := buildConfigBackupYAML(fetchConfigBackup(profile))
	if err != nil {
		log.Fatal(err)
	}

	if configBackupExportOutput != "" {
		if err := os.WriteFile(configBackupExportOutput, output, 0644); err != nil {
			log.Fatalf("Failed to write file: %v", err)
		}
		fmt.Printf("Configuration backup settings exported to %s\n", configBackupExportOutput)
	} else {
		fmt.Print(string(output))
	}
}

// exportConfigBackupToDir writes configuration-backup.yaml into outDir.
// Registry-compatible: used by the global export command.
func exportConfigBackupToDir(outDir string, profile models.Profile) error {
	output, err := buildConfigBackupYAML(fetchConfigBackup(profile))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", outDir, err)
	}
	fp := filepath.Join(outDir, configBackupExportFile)
	if err := os.WriteFile(fp, output, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", fp, err)
	}
	fmt.Printf("  Exported %s\n", fp)
	return nil
}

// snapshotConfigBackupForRegistry snapshots the configuration backup settings
// into state. Registry-compatible: used by the global snapshot command.
func snapshotConfigBackupForRegistry(profile models.Profile) error {
	if err := saveConfigBackupToState(fetchConfigBackup(profile)); err != nil {
		return fmt.Errorf("failed to save configuration backup state: %w", err)
	}
	fmt.Println("  Configuration backup settings snapshot saved.")
	return nil
}

// ---- apply -------------------------------------------------------------------

var configBackupApplyCmd = &cobra.Command{
	Use:   "apply <file>",
	Short: "Apply configuration backup settings from a YAML file",
	Long: `Applies configuration backup settings from a declarative YAML spec to VBR.

Uses PUT /api/v1/configurationBackup to update settings.

Supports --overlay for environment-specific overrides and --dry-run to preview changes.
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		settings := utils.ReadSettings()
		profile := utils.GetCurrentProfile()

		if settings.SelectedProfile != "vbr" {
			log.Fatal("This command only works with VBR at the moment.")
		}

		cfg := ResourceApplyConfig{
			Kind:         resources.KindVBRConfigurationBackup,
			Endpoint:     configBackupEndpoint,
			IgnoreFields: configBackupIgnoreFields,
			Mode:         ApplyUpdateOnly,
			SeverityMap:  configBackupSeverityMap,
			FetchCurrent: func(name string, profile models.Profile) (json.RawMessage, string, error) {
				rawData := vhttp.GetData[json.RawMessage](configBackupEndpoint, profile)
				return rawData, configBackupEndpoint, nil
			},
		}

		result := applyWithDryRunOutput(configBackupApplyOutput, configBackupApplyDryRun, func() ApplyResult {
			return applyWithOptionalOverlay(args[0], configBackupApplyOverlay, cfg, profile, configBackupApplyDryRun)
		})

		if result.Error != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", result.Error)
			outcome := DetermineApplyOutcome([]ApplyResult{result})
			os.Exit(ExitCodeForOutcome(outcome))
		}

		if result.DryRun {
			return // dry-run output already printed
		}

		fmt.Printf("\nSuccessfully updated configuration backup settings.\n")
	},
}

func init() {
	rootCmd.AddCommand(configBackupCmd)

	configBackupCmd.AddCommand(configBackupSnapshotCmd)

	configBackupCmd.AddCommand(configBackupDiffCmd)
	configBackupDiffCmd.Flags().StringVar(&configBackupDiffSeverity, "severity", "", "Minimum severity to show (critical, warning, info)")
	configBackupDiffCmd.Flags().BoolVar(&configBackupSecurityOnly, "security-only", false, "Show only WARNING and CRITICAL drifts")

	configBackupCmd.AddCommand(configBackupExportCmd)
	configBackupExportCmd.Flags().StringVarP(&configBackupExportOutput, "output", "o", "", "Output file path (default: stdout)")

	configBackupCmd.AddCommand(configBackupApplyCmd)
	configBackupApplyCmd.Flags().StringVar(&configBackupApplyOverlay, "overlay", "", "Overlay file to merge with the spec before applying")
	configBackupApplyCmd.Flags().BoolVar(&configBackupApplyDryRun, "dry-run", false, "Preview changes without applying")
	configBackupApplyCmd.Flags().StringVar(&configBackupApplyOutput, "output", "", dryRunOutputUsage)
}
