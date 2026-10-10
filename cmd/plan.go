package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/shapedthought/owlctl/config"
	"github.com/shapedthought/owlctl/resources"
	"github.com/shapedthought/owlctl/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	planOverlayFile string
	planEnvironment string
	planShowYAML    bool
)

var planCmd = &cobra.Command{
	Use:   "plan [config-file]",
	Short: "Preview merged configuration without applying",
	Long: `Plan shows the final merged configuration that would be applied.

The plan command is useful for:
- Previewing overlay merges before applying
- Validating YAML configuration syntax
- Understanding what the final job configuration will look like
- CI/CD pipeline validation (non-destructive)

Examples:
  # Preview base configuration
  owlctl job plan backup-job.yaml

  # Preview with overlay
  owlctl job plan base-job.yaml --overlay prod-overlay.yaml

  # Preview with environment overlay
  owlctl job plan base-job.yaml --env production

  # Show full YAML output
  owlctl job plan base-job.yaml --overlay prod-overlay.yaml --show-yaml

Note: This command shows the merged configuration but does not compare
against current VBR state. Full drift detection will be available in Phase 2.
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		warnDeprecatedOverlayShorthand(os.Args[1:])
		planJob(args[0])
	},
}

func planJob(configFile string) {
	settings := utils.ReadSettings()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	// Load base configuration
	baseSpec, err := resources.LoadResourceSpec(configFile)
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	// Validate resource type
	if baseSpec.Kind != "VBRJob" {
		log.Fatalf("Unsupported resource kind: %s (only VBRJob is supported)", baseSpec.Kind)
	}

	// Determine which overlay to use (same logic as apply command)
	var finalSpec resources.ResourceSpec
	var overlayUsed string

	if planOverlayFile != "" {
		// Explicit overlay file provided
		overlayUsed = planOverlayFile
		mergedSpec, err := resources.MergeYAMLFiles(configFile, planOverlayFile, resources.DefaultMergeOptions())
		if err != nil {
			log.Fatalf("Failed to merge with overlay: %v", err)
		}
		finalSpec = mergedSpec
	} else if planEnvironment != "" || needsPlanConfigOverlay() {
		// Try to use overlay from owlctl.yaml
		overlayPath, err := getPlanConfiguredOverlay()
		if err != nil {
			// If no overlay configured, just use base spec
			overlayUsed = "none"
			finalSpec = baseSpec
		} else {
			overlayUsed = overlayPath
			mergedSpec, err := resources.MergeYAMLFiles(configFile, overlayPath, resources.DefaultMergeOptions())
			if err != nil {
				log.Fatalf("Failed to merge with configured overlay: %v", err)
			}
			finalSpec = mergedSpec
		}
	} else {
		// No overlay specified
		overlayUsed = "none"
		finalSpec = baseSpec
	}

	// Display plan header
	fmt.Println("╔════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                     Configuration Plan Preview                     ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Printf("Resource Name: %s\n", finalSpec.Metadata.Name)
	fmt.Printf("Resource Type: %s\n", finalSpec.Kind)
	fmt.Printf("Base Config:   %s\n", configFile)
	fmt.Printf("Overlay:       %s\n", overlayUsed)
	fmt.Println()

	// Show labels if present
	if len(finalSpec.Metadata.Labels) > 0 {
		fmt.Println("Labels:")
		for k, v := range finalSpec.Metadata.Labels {
			fmt.Printf("  %s: %s\n", k, v)
		}
		fmt.Println()
	}

	// Fetch current job from VBR to show diff
	profile := utils.GetCurrentProfile()
	currentRaw, currentID, err := fetchCurrentJob(finalSpec.Metadata.Name, profile)
	if err != nil {
		log.Fatalf("Failed to fetch current job: %v", err)
	}

	if currentRaw == nil {
		// Job doesn't exist - will be created
		fmt.Println("⚠ Job not found in VBR - will be created")
		fmt.Println()
		showNewJobSummary(finalSpec)
	} else {
		// Job exists - show diff
		fmt.Printf("✓ Current job found in VBR (ID: %s)\n", currentID)
		fmt.Println()

		var currentMap map[string]interface{}
		if err := json.Unmarshal(currentRaw, &currentMap); err != nil {
			log.Fatalf("Failed to unmarshal current job: %v", err)
		}
		showJobDiff(finalSpec, currentMap)
	}

	// Show full YAML if requested
	if planShowYAML {
		fmt.Println("\nFull YAML Configuration:")
		fmt.Println("─────────────────────────────────────────────────────────────────")
		yamlBytes, err := yaml.Marshal(finalSpec)
		if err != nil {
			log.Fatalf("Failed to marshal YAML: %v", err)
		}
		fmt.Println(string(yamlBytes))
		fmt.Println("─────────────────────────────────────────────────────────────────")
	}

	// Show next steps
	fmt.Println("\nNext Steps:")
	fmt.Printf("  To apply this configuration:\n")
	if overlayUsed != "none" {
		if planOverlayFile != "" {
			fmt.Printf("    owlctl job apply %s --overlay %s\n", configFile, planOverlayFile)
		} else {
			fmt.Printf("    owlctl job apply %s --env %s\n", configFile, planEnvironment)
		}
	} else {
		fmt.Printf("    owlctl job apply %s\n", configFile)
	}
	fmt.Println()
}

// needsPlanConfigOverlay checks if we should try to use owlctl.yaml overlay
func needsPlanConfigOverlay() bool {
	cfg, err := config.LoadConfig()
	if err != nil {
		return false
	}

	if cfg.CurrentEnvironment != "" {
		cfg.WarnDeprecatedFields()
		_, err := cfg.GetEnvironmentOverlay("")
		return err == nil
	}

	return false
}

// getPlanConfiguredOverlay gets the overlay path from owlctl.yaml
func getPlanConfiguredOverlay() (string, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load owlctl.yaml: %w", err)
	}

	cfg.WarnDeprecatedFields()

	env := planEnvironment
	if env == "" {
		env = cfg.CurrentEnvironment
	}

	if env == "" {
		return "", fmt.Errorf("no environment specified and no currentEnvironment in owlctl.yaml")
	}

	overlayPath, err := cfg.GetEnvironmentOverlay(env)
	if err != nil {
		return "", err
	}

	return overlayPath, nil
}

// showNewJobSummary displays configuration for a job that will be created
func showNewJobSummary(spec resources.ResourceSpec) {
	fmt.Println("Configuration to be created:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show key fields
	if desc, ok := spec.Spec["description"].(string); ok && desc != "" {
		fmt.Printf("  Description: %s\n", desc)
	}
	if typ, ok := spec.Spec["type"].(string); ok {
		fmt.Printf("  Job Type:    %s\n", typ)
	}

	// Storage
	if storage, ok := spec.Spec["storage"].(map[string]interface{}); ok {
		if ret, ok := storage["retentionPolicy"].(map[string]interface{}); ok {
			quantityVal, quantityOk := ret["quantity"]
			typeVal, typeOk := ret["type"]

			quantityStr := "N/A"
			if quantityOk && quantityVal != nil {
				quantityStr = fmt.Sprintf("%v", quantityVal)
			}

			typeStr := "N/A"
			if typeOk && typeVal != nil {
				if str, ok := typeVal.(string); ok {
					typeStr = str
				} else {
					typeStr = fmt.Sprintf("%v", typeVal)
				}
			}

			fmt.Printf("  Retention:   %s %s\n", quantityStr, typeStr)
		}
	}

	// Schedule
	if schedule, ok := spec.Spec["schedule"].(map[string]interface{}); ok {
		if dailyObj, ok := schedule["daily"].(map[string]interface{}); ok {
			if isEnabled, ok := dailyObj["isEnabled"].(bool); ok && isEnabled {
				if localTime, ok := dailyObj["localTime"].(string); ok {
					fmt.Printf("  Schedule:    Daily at %s\n", localTime)
				}
			}
		}
	}

	// Objects count
	if virtualMachines, ok := spec.Spec["virtualMachines"].(map[string]interface{}); ok {
		if includes, ok := virtualMachines["includes"].([]interface{}); ok {
			fmt.Printf("  Objects:     %d VM(s)\n", len(includes))
		}
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// showJobDiff compares desired spec with current VBR job (as map) and displays differences
func showJobDiff(desiredSpec resources.ResourceSpec, currentMap map[string]interface{}) {
	// Convert both to comparable maps
	desiredMap := desiredSpec.Spec

	// Detect drifts (desired is "state", current is "VBR")
	drifts := detectDrift(desiredMap, currentMap, jobIgnoreFields)

	if len(drifts) == 0 {
		fmt.Println("Changes to be applied:")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("  No changes detected - configuration matches current VBR state")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		return
	}

	// Classify drifts by severity
	drifts = classifyDrifts(drifts, jobSeverityMap)

	// Group drifts by category for better readability
	storageDrifts := []Drift{}
	scheduleDrifts := []Drift{}
	objectDrifts := []Drift{}
	otherDrifts := []Drift{}

	for _, d := range drifts {
		if strings.HasPrefix(d.Path, "storage") {
			storageDrifts = append(storageDrifts, d)
		} else if strings.HasPrefix(d.Path, "schedule") {
			scheduleDrifts = append(scheduleDrifts, d)
		} else if strings.HasPrefix(d.Path, "virtualMachines") {
			objectDrifts = append(objectDrifts, d)
		} else {
			otherDrifts = append(otherDrifts, d)
		}
	}

	fmt.Println("Changes to be applied:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Show drifts by category
	if len(storageDrifts) > 0 {
		fmt.Println("\nStorage Settings:")
		for _, d := range storageDrifts {
			printPlanDrift(d)
		}
	}

	if len(scheduleDrifts) > 0 {
		fmt.Println("\nSchedule Settings:")
		for _, d := range scheduleDrifts {
			printPlanDrift(d)
		}
	}

	if len(objectDrifts) > 0 {
		fmt.Println("\nBackup Objects:")
		for _, d := range objectDrifts {
			printPlanDrift(d)
		}
	}

	if len(otherDrifts) > 0 {
		fmt.Println("\nOther Settings:")
		for _, d := range otherDrifts {
			printPlanDrift(d)
		}
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Summary
	criticalCount := 0
	warningCount := 0
	infoCount := 0
	for _, d := range drifts {
		switch d.Severity {
		case SeverityCritical:
			criticalCount++
		case SeverityWarning:
			warningCount++
		case SeverityInfo:
			infoCount++
		}
	}

	fmt.Printf("\nSummary: %d field(s) will be changed", len(drifts))
	if criticalCount > 0 || warningCount > 0 {
		fmt.Printf(" (%d critical, %d warning, %d info)", criticalCount, warningCount, infoCount)
	}
	fmt.Println()
}

// printPlanDrift prints a single drift in plan format, labeling values as current (VBR) and new (desired)
func printPlanDrift(drift Drift) {
	// For plan, we show: current (VBR) -> new (desired from YAML)
	// Drift detection also compares desired (state) to current (VBR) and shows it as: state -> VBR
	// The comparison direction is the same, we just label the values differently for clarity

	sev := string(drift.Severity)

	switch drift.Action {
	case "modified":
		desiredStr := formatValue(drift.State) // What we want (from YAML)
		currentStr := formatValue(drift.VBR)   // What's currently in VBR
		fmt.Printf("  %s ~ %s: %s (current) -> %s (new)\n", sev, drift.Path, currentStr, desiredStr)
	case "removed":
		// Field exists in YAML but not in VBR - will be added when applying
		desiredStr := formatValue(drift.State)
		fmt.Printf("  %s + %s: Will be added with value %s\n", sev, drift.Path, desiredStr)
	case "added":
		// Field exists in VBR but not in YAML - will be removed/unset when applying
		currentStr := formatValue(drift.VBR)
		fmt.Printf("  %s - %s: Will be removed/unset (current: %s)\n", sev, drift.Path, currentStr)
	}
}

func init() {
	planCmd.Flags().StringVarP(&planOverlayFile, "overlay", "o", "", "Overlay file to merge with base configuration")
	planCmd.Flags().StringVar(&planEnvironment, "env", "", "Environment to use (looks up overlay from owlctl.yaml)")
	planCmd.Flags().BoolVar(&planShowYAML, "show-yaml", false, "Display full merged YAML configuration")

	jobsCmd.AddCommand(planCmd)
}
