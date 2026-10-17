package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/shapedthought/owlctl/config"
	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/resources"
	"github.com/shapedthought/owlctl/utils"
	"github.com/shapedthought/owlctl/vhttp"
	"github.com/spf13/cobra"
)

var (
	overlayFile string
	environment string
	dryRun      bool
	groupName   string
)

var applyCmd = &cobra.Command{
	Use:   "apply [config-file]",
	Short: "Apply a declarative job configuration",
	Long: `Apply a declarative job configuration with optional overlay.

The apply command creates or updates VBR jobs based on YAML configuration files.
It supports overlay files for environment-specific customization.

Examples:
  # Apply a job configuration
  owlctl job apply backup-job.yaml

  # Apply with overlay file
  owlctl job apply base-job.yaml --overlay prod-overlay.yaml

  # Apply with environment (uses overlay from owlctl.yaml)
  owlctl job apply base-job.yaml --env production

  # Dry run to preview changes
  owlctl job apply base-job.yaml --overlay prod-overlay.yaml --dry-run

  # Apply all specs in a group (from owlctl.yaml)
  owlctl job apply --group sql-tier

  # Dry run a group
  owlctl job apply --group sql-tier --dry-run

Overlay Resolution:
  1. If --overlay is specified, use that overlay file
  2. If --env is specified, use overlay from owlctl.yaml for that environment
  3. If owlctl.yaml exists and has currentEnvironment set, use that overlay
  4. Otherwise, apply base configuration without overlay
`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		warnDeprecatedOverlayShorthand(os.Args[1:])
		if groupName != "" {
			// Validate mutual exclusivity
			if len(args) > 0 {
				log.Fatal("Cannot use --group with a positional config file argument")
			}
			if overlayFile != "" {
				log.Fatal("Cannot use --group with --overlay (group defines its own overlay)")
			}
			if environment != "" {
				log.Fatal("Cannot use --group with --env (group defines its own overlay)")
			}
			applyGroup(groupName)
		} else if len(args) > 0 {
			applyJob(args[0])
		} else {
			log.Fatal("Provide a config file or use --group")
		}
	},
}

func applyJob(configFile string) {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

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

	// Determine which overlay to use
	var finalSpec resources.ResourceSpec
	if overlayFile != "" {
		// Explicit overlay file provided
		fmt.Printf("Applying overlay: %s\n", overlayFile)
		mergedSpec, err := resources.MergeYAMLFiles(configFile, overlayFile, resources.DefaultMergeOptions())
		if err != nil {
			log.Fatalf("Failed to merge with overlay: %v", err)
		}
		finalSpec = mergedSpec
	} else if environment != "" || needsConfigOverlay() {
		// Try to use overlay from owlctl.yaml
		overlayPath, err := getConfiguredOverlay()
		if err != nil {
			// If no overlay configured, just use base spec
			fmt.Printf("No overlay configured, applying base configuration\n")
			finalSpec = baseSpec
		} else {
			fmt.Printf("Applying environment overlay: %s\n", overlayPath)
			mergedSpec, err := resources.MergeYAMLFiles(configFile, overlayPath, resources.DefaultMergeOptions())
			if err != nil {
				log.Fatalf("Failed to merge with configured overlay: %v", err)
			}
			finalSpec = mergedSpec
		}
	} else {
		// No overlay specified
		fmt.Println("Applying base configuration (no overlay)")
		finalSpec = baseSpec
	}

	// Display merged configuration in dry-run mode
	if dryRun {
		fmt.Println("\n╔════════════════════════════════════════════════════════════════════╗")
		fmt.Println("║                         Dry Run Mode                               ║")
		fmt.Println("╚════════════════════════════════════════════════════════════════════╝")
		fmt.Println()
		fmt.Printf("Resource: %s (%s)\n", finalSpec.Metadata.Name, finalSpec.Kind)
		fmt.Println()

		// Fetch current job from VBR to show diff
		currentRaw, currentID, err := fetchCurrentJob(finalSpec.Metadata.Name, profile)
		if err != nil {
			log.Fatalf("Failed to fetch current job: %v", err)
		}

		if currentRaw == nil {
			// Job doesn't exist - will be created
			fmt.Println("⚠ Job not found in VBR - will be created")
			fmt.Println()
			showNewJobSummary(finalSpec)
			if plan := plannedJobStateForDryRun(finalSpec.Spec, nil); plan != "" {
				fmt.Printf("\nWould call: %s\n", plan)
			}
		} else {
			// Job exists - show diff
			fmt.Printf("✓ Current job found in VBR (ID: %s)\n", currentID)
			fmt.Println()

			var currentMap map[string]interface{}
			if err := json.Unmarshal(currentRaw, &currentMap); err != nil {
				log.Fatalf("Failed to unmarshal current job: %v", err)
			}
			showJobDiff(finalSpec, currentMap)
			if plan := plannedJobStateForDryRun(finalSpec.Spec, currentMap); plan != "" {
				fmt.Printf("\nWould call: %s\n", plan)
			}
		}

		fmt.Println("\n=== End Dry Run ===")
		fmt.Println("Note: This is a preview. No changes have been made.")
		fmt.Println("Remove --dry-run flag to apply these changes.")
		return
	}

	// Apply the job configuration
	if err := applyVBRJob(finalSpec, profile); err != nil {
		log.Fatalf("Failed to apply job: %v", err)
	}

	fmt.Printf("\n✓ Successfully applied job: %s\n", finalSpec.Metadata.Name)
}

// applyGroup applies all specs in a named group from owlctl.yaml
func applyGroup(group string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load owlctl.yaml: %v", err)
	}
	cfg.WarnDeprecatedFields()

	groupCfg, err := cfg.GetGroup(group)
	if err != nil {
		log.Fatalf("Group error: %v", err)
	}

	// Activate instance if configured on the group
	profile := activateGroupInstance(cfg, groupCfg)

	// Resolve effective specs (Specs + SpecsDir)
	specsList := resolveGroupSpecs(cfg, groupCfg)

	if len(specsList) == 0 {
		log.Fatalf("Group %q has no specs defined", group)
	}

	// Resolve paths relative to owlctl.yaml
	profilePath := ""
	if groupCfg.Profile != "" {
		profilePath = cfg.ResolvePath(groupCfg.Profile)
	}
	overlayPath := ""
	if groupCfg.Overlay != "" {
		overlayPath = cfg.ResolvePath(groupCfg.Overlay)
	}

	fmt.Printf("Applying group: %s (%d specs)\n", group, len(specsList))
	if instanceFlag != "" {
		fmt.Printf("  Instance: %s (from --instance flag)\n", instanceFlag)
	} else if groupCfg.Instance != "" {
		fmt.Printf("  Instance: %s\n", groupCfg.Instance)
	}
	if profilePath != "" {
		fmt.Printf("  Profile: %s\n", groupCfg.Profile)
	}
	if overlayPath != "" {
		fmt.Printf("  Overlay: %s\n", groupCfg.Overlay)
	}
	fmt.Println()

	// Pre-load profile and overlay once to avoid repeated disk I/O
	var profileSpec, overlaySpec *resources.ResourceSpec
	opts := resources.DefaultMergeOptions()

	if profilePath != "" {
		p, err := resources.LoadResourceSpec(profilePath)
		if err != nil {
			log.Fatalf("Failed to load profile %s: %v", profilePath, err)
		}
		profileSpec = &p
	}
	if overlayPath != "" {
		o, err := resources.LoadResourceSpec(overlayPath)
		if err != nil {
			log.Fatalf("Failed to load overlay %s: %v", overlayPath, err)
		}
		overlaySpec = &o
	}

	var results []GroupApplyResult

	for _, specRelPath := range specsList {
		specPath := cfg.ResolvePath(specRelPath)
		result := GroupApplyResult{SpecPath: specRelPath}

		// Load spec
		spec, err := resources.LoadResourceSpec(specPath)
		if err != nil {
			result.Error = fmt.Errorf("failed to load spec: %w", err)
			results = append(results, result)
			continue
		}

		// Merge with cached profile/overlay
		mergedSpec, err := resources.ApplyGroupMergeFromSpecs(spec, profileSpec, overlaySpec, opts)
		if err != nil {
			result.Error = fmt.Errorf("merge failed: %w", err)
			results = append(results, result)
			continue
		}

		result.ResourceName = mergedSpec.Metadata.Name

		// Validate kind
		if mergedSpec.Kind != resources.KindVBRJob {
			result.Error = fmt.Errorf("unsupported kind: %s (expected VBRJob)", mergedSpec.Kind)
			results = append(results, result)
			continue
		}

		// Check existence BEFORE apply to correctly determine created vs updated
		existingRaw, _, err := fetchCurrentJob(mergedSpec.Metadata.Name, profile)
		if err != nil {
			result.Error = fmt.Errorf("failed to fetch current job: %w", err)
			results = append(results, result)
			continue
		}
		existedBefore := existingRaw != nil

		if dryRun {
			// Show dry-run preview for this spec
			fmt.Printf("--- %s ---\n", specRelPath)
			fmt.Printf("Resource: %s (%s)\n", mergedSpec.Metadata.Name, mergedSpec.Kind)

			if !existedBefore {
				fmt.Println("  Would be created (not found in VBR)")
				result.Action = "would-create"
			} else {
				fmt.Printf("  Found in VBR — would be updated\n")
				result.Action = "would-update"
			}
			var existingMap map[string]interface{}
			if existingRaw != nil {
				if err := json.Unmarshal(existingRaw, &existingMap); err != nil {
					result.Error = fmt.Errorf("failed to parse current job: %w", err)
					results = append(results, result)
					continue
				}
			}
			if plan := plannedJobStateForDryRun(mergedSpec.Spec, existingMap); plan != "" {
				fmt.Printf("  Would call: %s\n", plan)
			}
			fmt.Println()
		} else {
			// Apply the job
			if err := applyVBRJob(mergedSpec, profile); err != nil {
				result.Error = err
			} else {
				if existedBefore {
					result.Action = "updated"
				} else {
					result.Action = "created"
				}
			}
		}

		results = append(results, result)
	}

	printGroupApplySummary(group, results)

	// Determine exit code
	successCount := 0
	for _, r := range results {
		if r.Error == nil {
			successCount++
		}
	}

	if successCount == 0 {
		os.Exit(ExitError)
	} else if successCount < len(results) {
		os.Exit(ExitPartialApply)
	}
	// All succeeded — exit 0 (default)
}

// needsConfigOverlay checks if we should try to use owlctl.yaml overlay
func needsConfigOverlay() bool {
	// Try to load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return false
	}

	// Check if there's a current environment with an overlay
	if cfg.CurrentEnvironment != "" {
		cfg.WarnDeprecatedFields()
		_, err := cfg.GetEnvironmentOverlay("")
		return err == nil
	}

	return false
}

// getConfiguredOverlay gets the overlay path from owlctl.yaml
func getConfiguredOverlay() (string, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load owlctl.yaml: %w", err)
	}

	cfg.WarnDeprecatedFields()

	// Use explicit environment if specified
	env := environment
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

// jobApplyConfig defines how to apply job configurations via the generic infrastructure
var jobApplyConfig = ResourceApplyConfig{
	Kind:           "VBRJob",
	Endpoint:       "jobs",
	IgnoreFields:   jobIgnoreFields,
	Mode:           ApplyCreateOrUpdate,
	FetchCurrent:   fetchCurrentJob,
	PreparePayload: prepareJobPayload,
	PostApply:      applyJobState,
	PlanPostApply:  plannedJobState,
}

// fetchCurrentJob retrieves a job by name from VBR.
// Returns (rawJSON, id, nil) if found, (nil, "", nil) if not found.
func fetchCurrentJob(name string, profile models.Profile) (json.RawMessage, string, error) {
	// List all jobs (summary only)
	type JobsResponse struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}

	response := vhttp.GetData[JobsResponse]("jobs", profile)

	for _, job := range response.Data {
		if job.Name == name {
			// Fetch full details by ID (list returns summary only)
			endpoint := fmt.Sprintf("jobs/%s", job.ID)
			jobData := vhttp.GetData[json.RawMessage](endpoint, profile)
			return jobData, job.ID, nil
		}
	}

	return nil, "", nil // Not found (not an error)
}

// prepareJobPayload applies job-specific business logic to the merged spec before sending.
// This handles guest credential defaults and VM exclude cleanup using map-based operations,
// making it safe for all job types (VM, file, NAS, etc.).
func prepareJobPayload(spec, existing map[string]interface{}) (map[string]interface{}, error) {
	// Guest credential defaults: if no credsType and no credentials, set useAgentManagementCredentials
	if gp, ok := spec["guestProcessing"].(map[string]interface{}); ok {
		if gc, ok := gp["guestCredentials"].(map[string]interface{}); ok {
			_, hasCredsType := gc["credsType"]
			_, hasCreds := gc["credentials"]

			// If credsType is empty string, treat as absent
			if ct, ok := gc["credsType"].(string); ok && ct == "" {
				hasCredsType = false
			}

			if !hasCredsType && !hasCreds {
				gc["useAgentManagementCredentials"] = true
			}

			cleanCredentialsPerMachine(gc, existingCredentialsPerMachine(existing))
		}
	}

	// Clean VM excludes: if excludes.disks has no valid entries, clear it
	if vms, ok := spec["virtualMachines"].(map[string]interface{}); ok {
		if excludes, ok := vms["excludes"].(map[string]interface{}); ok {
			if disks, ok := excludes["disks"].([]interface{}); ok && len(disks) > 0 {
				hasValidExcludes := false
				for _, diskEntry := range disks {
					if de, ok := diskEntry.(map[string]interface{}); ok {
						if d, ok := de["disks"].([]interface{}); ok && len(d) > 0 {
							hasValidExcludes = true
							break
						}
					}
				}
				if !hasValidExcludes {
					delete(excludes, "disks")
				}
			}
		}
	}

	return spec, nil
}

// stripNilElements returns list without nil elements.
func stripNilElements(list []interface{}) []interface{} {
	out := make([]interface{}, 0, len(list))
	for _, v := range list {
		if v != nil {
			out = append(out, v)
		}
	}
	return out
}

// existingCredentialsPerMachine returns the live job's non-empty
// guestProcessing.guestCredentials.credentialsPerMachine, or nil.
func existingCredentialsPerMachine(existing map[string]interface{}) []interface{} {
	gp, _ := existing["guestProcessing"].(map[string]interface{})
	gc, _ := gp["guestCredentials"].(map[string]interface{})
	list, _ := gc["credentialsPerMachine"].([]interface{})
	if list = stripNilElements(list); len(list) == 0 {
		return nil
	}
	return list
}

// cleanCredentialsPerMachine drops nil entries (from bare YAML "-" items) because
// VBR rejects them with 400 "Credentials Per Machine must not be empty". If nothing
// is left, it falls back to the live job's value, or omits the key. Never sends [].
func cleanCredentialsPerMachine(gc map[string]interface{}, fallback []interface{}) {
	raw, present := gc["credentialsPerMachine"]
	if !present {
		return
	}
	list, _ := raw.([]interface{})
	if list = stripNilElements(list); len(list) > 0 {
		gc["credentialsPerMachine"] = list
		return
	}
	if len(fallback) > 0 {
		gc["credentialsPerMachine"] = fallback
		return
	}
	delete(gc, "credentialsPerMachine")
}

// applyVBRJob creates or updates a VBR job based on the specification.
// Delegates to the generic applyResourceSpec infrastructure.
func applyVBRJob(spec resources.ResourceSpec, profile models.Profile) error {
	// Ensure the payload name matches metadata.name so the API lookup key
	// and the body sent to VBR stay consistent (e.g. after overlay changes).
	if spec.Spec == nil {
		spec.Spec = make(map[string]interface{})
	}
	if spec.Metadata.Name != "" {
		spec.Spec["name"] = spec.Metadata.Name
	}

	result := applyResourceSpec(spec, jobApplyConfig, profile, false, nil)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func init() {
	applyCmd.Flags().StringVarP(&overlayFile, "overlay", "o", "", "Overlay file to merge with base configuration")
	applyCmd.Flags().StringVar(&environment, "env", "", "Environment to use (looks up overlay from owlctl.yaml)")
	applyCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying them")
	applyCmd.Flags().StringVar(&groupName, "group", "", "Apply all specs in named group (from owlctl.yaml)")

	jobsCmd.AddCommand(applyCmd)
}
