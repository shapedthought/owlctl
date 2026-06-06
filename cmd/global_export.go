package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/shapedthought/owlctl/config"
	"github.com/shapedthought/owlctl/resources"
	"github.com/shapedthought/owlctl/state"
	"github.com/shapedthought/owlctl/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	globalExportAll          bool
	globalExportDir          string
	globalExportAllInstances bool
	globalExportFromState    bool
)

var globalExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all resource types to declarative YAML",
	Long: `Export all resource types for the active instance to declarative YAML files,
organised into one folder per resource type under the output directory. With
--all-instances, each instance is written to its own subfolder
(<instance>/<resource-type>/).

This command is typically used to initialise a GitOps repository from an existing
Veeam environment, or to take a point-in-time export of all configurations.

Output structure:
  exports/
    jobs/
      daily-backup.yaml
    repos/
      primary-repo.yaml
    sobrs/
      scale-out-repo.yaml
    encryption/
      my-password.yaml
    kms/
      kms-server.yaml

Use --all-instances to export all instances defined in owlctl.yaml.
Use --from-state to generate YAML from state.json without contacting the VBR server.

Examples:
  # Export all resources for the active instance
  owlctl export --all

  # Export to a specific directory
  owlctl export --all -d ./exports

  # Export all instances
  owlctl export --all --all-instances -d ./exports

  # Export from state (offline, no server required)
  owlctl export --all --from-state -d ./exports
`,
	Run: func(cmd *cobra.Command, args []string) {
		if !globalExportAll {
			log.Fatal("Use --all to export all resource types. To export a single job use 'owlctl job export <id>'.")
		}

		outDir := globalExportDir
		if outDir == "" {
			outDir = "exports"
		}

		if globalExportAllInstances {
			runExportAllInstances(outDir, globalExportFromState)
		} else {
			runExportForActiveInstance(outDir, globalExportFromState)
		}
	},
}

func runExportForActiveInstance(outDir string, fromState bool) {
	if fromState {
		runExportFromState(outDir)
		return
	}

	settings := utils.ReadSettings()
	product := settings.SelectedProfile

	pe := findProductExporter(product)
	if pe == nil {
		log.Fatalf("No export support registered for product %q. Export is currently only supported for VBR.", product)
	}

	profile := utils.GetCurrentProfile()
	instanceName := os.Getenv("OWLCTL_ACTIVE_INSTANCE")
	if instanceName == "" {
		instanceName = "default"
	}

	fmt.Printf("Exporting all resources for instance %q (product: %s) to %s...\n\n", instanceName, product, outDir)

	for _, exporter := range pe.Resources {
		if exporter.ExportAll == nil {
			continue
		}

		subDir := filepath.Join(outDir, exporter.FolderName)
		fmt.Printf("  [%s]\n", exporter.FolderName)
		if err := exporter.ExportAll(subDir, profile); err != nil {
			fmt.Printf("  [%s] Warning: export failed: %v\n", exporter.FolderName, err)
		}
		fmt.Println()
	}

	fmt.Printf("Export complete. Files written to: %s\n", outDir)
}

// runExportFromState generates YAML from state.json without contacting the VBR server.
// Uses InstanceState.Product to determine the correct folder structure.
// When --all-instances is set, iterates all instances in state; otherwise uses the active instance.
func runExportFromState(outDir string) {
	stateMgr := state.NewManager()
	st, err := stateMgr.Load()
	if err != nil {
		log.Fatalf("Failed to load state: %v", err)
	}

	if len(st.Instances) == 0 {
		fmt.Println("No instances in state. Run 'owlctl snapshot --all' first.")
		return
	}

	// Determine which instances to iterate
	var instanceNames []string
	if globalExportAllInstances {
		for name := range st.Instances {
			instanceNames = append(instanceNames, name)
		}
		sort.Strings(instanceNames)
	} else {
		active := os.Getenv("OWLCTL_ACTIVE_INSTANCE")
		if active == "" {
			active = "default"
		}
		instanceNames = []string{active}
	}

	for _, instName := range instanceNames {
		inst, ok := st.Instances[instName]
		if !ok || inst == nil {
			fmt.Printf("Instance %q not found in state.\n", instName)
			continue
		}

		if len(instanceNames) > 1 {
			fmt.Printf("\n=== Instance: %s (product: %s) ===\n", instName, inst.Product)
		}

		instanceOutDir := outDir
		if globalExportAllInstances {
			instanceOutDir = filepath.Join(outDir, instName)
		}

		exportInstanceFromState(instName, inst, instanceOutDir)
	}
}

// fromStateIgnoreFields maps each resource kind to the ignore-field set its live
// exporter strips, so `export --from-state` produces the same YAML as a live export.
var fromStateIgnoreFields = map[string]map[string]bool{
	"VBRJob":                 jobIgnoreFields,
	"VBRRepository":          repoIgnoreFields,
	"VBRScaleOutRepository":  sobrIgnoreFields,
	"VBREncryptionPassword":  encryptionIgnoreFields,
	"VBRKmsServer":           kmsIgnoreFields,
	"VBRConfigurationBackup": configBackupIgnoreFields,
}

// exportInstanceFromState writes YAML files for all resources in a state instance.
func exportInstanceFromState(instanceName string, inst *state.InstanceState, outDir string) {
	// Build a Kind → FolderName map across all registered products. We don't key
	// on inst.Product: state migrated from v3 doesn't have Product set, which
	// would otherwise return no exporter and bucket every resource under "unknown/".
	kindToFolder := make(map[string]string)
	for i := range exportRegistry {
		for _, e := range exportRegistry[i].Resources {
			kindToFolder[e.Kind] = e.FolderName
		}
	}

	// Group resources by kind/folder
	type entry struct {
		name   string
		res    *state.Resource
		folder string
	}
	var entries []entry
	for name, res := range inst.Resources {
		folder, ok := kindToFolder[res.Type]
		if !ok {
			folder = "unknown"
		}
		entries = append(entries, entry{name: name, res: res, folder: folder})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].folder != entries[j].folder {
			return entries[i].folder < entries[j].folder
		}
		return entries[i].name < entries[j].name
	})

	successCount := 0
	failedCount := 0

	for _, e := range entries {
		subDir := filepath.Join(outDir, e.folder)
		if err := os.MkdirAll(subDir, 0755); err != nil {
			fmt.Printf("Warning: Failed to create directory %s: %v\n", subDir, err)
			failedCount++
			continue
		}

		// Marshal spec back to raw JSON then convert to YAML resource spec
		specBytes, err := json.Marshal(e.res.Spec)
		if err != nil {
			fmt.Printf("Warning: Failed to marshal spec for %q: %v\n", e.name, err)
			failedCount++
			continue
		}

		resourceSpec := resources.ResourceSpec{
			APIVersion: "owlctl.veeam.com/v1",
			Kind:       e.res.Type,
			Metadata:   resources.Metadata{Name: e.name},
		}
		if err := json.Unmarshal(specBytes, &resourceSpec.Spec); err != nil {
			fmt.Printf("Warning: Failed to unmarshal spec for %q: %v\n", e.name, err)
			failedCount++
			continue
		}

		// Strip the same ignore fields (id, uniqueId, runtime timestamps, ...) that
		// live exports remove, so --from-state YAML matches a live export.
		if fields, ok := fromStateIgnoreFields[e.res.Type]; ok {
			removeIgnoreFields(resourceSpec.Spec, fields)
		}

		appliedLabel := "Last applied"
		if e.res.Origin == "observed" {
			appliedLabel = "Last snapshotted"
		}
		header := fmt.Sprintf("# %s Configuration (from state)\n# Instance: %s\n# Resource ID: %s\n# %s: %s\n\n",
			e.res.Type, instanceName, e.res.ID, appliedLabel, e.res.LastApplied.Format("2006-01-02 15:04:05"))

		yamlBytes, err := yaml.Marshal(resourceSpec)
		if err != nil {
			fmt.Printf("Warning: Failed to marshal YAML for %q: %v\n", e.name, err)
			failedCount++
			continue
		}

		fp := filepath.Join(subDir, sanitizeFilename(e.name)+".yaml")
		content := append([]byte(header), yamlBytes...)
		if err := os.WriteFile(fp, content, 0644); err != nil {
			fmt.Printf("Warning: Failed to write %s: %v\n", fp, err)
			failedCount++
			continue
		}

		successCount++
		fmt.Printf("  Exported %s/%s\n", e.folder, sanitizeFilename(e.name)+".yaml")
	}

	fmt.Printf("\nExport complete: %d successful, %d failed. Files written to: %s\n", successCount, failedCount, outDir)
}

func runExportAllInstances(outDir string, fromState bool) {
	if fromState {
		// --from-state with --all-instances iterates state directly (no owlctl.yaml needed)
		runExportFromState(outDir)
		return
	}

	iterateAllInstances(func(name string, resolved *config.ResolvedInstance) {
		fmt.Printf("\n=== Instance: %s (product: %s) ===\n", name, resolved.Product)
		instanceOutDir := filepath.Join(outDir, name)
		runExportForActiveInstance(instanceOutDir, false)
	})
}

func init() {
	globalExportCmd.Flags().BoolVar(&globalExportAll, "all", false, "Export all resource types")
	globalExportCmd.Flags().StringVarP(&globalExportDir, "dir", "d", "", "Output directory (default: ./exports)")
	globalExportCmd.Flags().BoolVar(&globalExportAllInstances, "all-instances", false, "Export all configured instances from owlctl.yaml")
	globalExportCmd.Flags().BoolVar(&globalExportFromState, "from-state", false, "Export from state.json instead of live API (offline)")
	rootCmd.AddCommand(globalExportCmd)
}
