package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/shapedthought/owlctl/config"
	"github.com/shapedthought/owlctl/utils"
	"github.com/spf13/cobra"
)

var (
	globalSnapshotAll          bool
	globalSnapshotAllInstances bool
)

var globalSnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Snapshot all resource types to state",
	Long: `Snapshot all resource types for the active instance into state.json.

Use this command to baseline an entire environment at once, rather than
running individual snapshot commands for each resource type.

Repositories, SOBRs, encryption passwords, and KMS servers are snapshotted.
Jobs are skipped here — they are baselined automatically by 'job apply' and
can be snapshotted individually with 'job snapshot'. Configuration backup is
not included; snapshot it with 'config-backup snapshot'.

Examples:
  # Snapshot all resources for the active instance
  owlctl snapshot --all

  # Snapshot all resources across all configured instances
  owlctl snapshot --all --all-instances
`,
	Run: func(cmd *cobra.Command, args []string) {
		if !globalSnapshotAll {
			log.Fatal("Use --all to snapshot all resource types. Individual resource snapshot commands are also available (e.g. 'owlctl repo snapshot --all').")
		}
		if globalSnapshotAllInstances {
			runSnapshotAllInstances()
		} else {
			runSnapshotForActiveInstance()
		}
	},
}

func runSnapshotForActiveInstance() {
	settings := utils.ReadSettings()
	product := settings.SelectedProfile

	pe := findProductExporter(product)
	if pe == nil {
		log.Fatalf("No snapshot support registered for product %q. Snapshot is currently only supported for VBR.", product)
	}

	profile := utils.GetCurrentProfile()
	instanceName := os.Getenv("OWLCTL_ACTIVE_INSTANCE")
	if instanceName == "" {
		instanceName = "default"
	}

	fmt.Printf("Snapshotting all resources for instance %q (product: %s)...\n\n", instanceName, product)

	for _, exporter := range pe.Resources {
		if exporter.SnapshotAll == nil {
			fmt.Printf("  [%s] Skipping (use 'job apply' to manage jobs declaratively)\n", exporter.FolderName)
			continue
		}

		fmt.Printf("  [%s] Snapshotting...\n", exporter.FolderName)
		if err := exporter.SnapshotAll(profile); err != nil {
			fmt.Printf("  [%s] Warning: snapshot failed: %v\n", exporter.FolderName, err)
		}
	}

	fmt.Println("\nSnapshot complete.")
}

func runSnapshotAllInstances() {
	iterateAllInstances(func(name string, resolved *config.ResolvedInstance) {
		fmt.Printf("\n=== Instance: %s (product: %s) ===\n", name, resolved.Product)
		runSnapshotForActiveInstance()
	})
}

// iterateAllInstances loads all instances from owlctl.yaml, activates each in turn,
// and calls fn. Instances that fail to resolve or activate are skipped with a warning.
func iterateAllInstances(fn func(name string, resolved *config.ResolvedInstance)) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load owlctl.yaml: %v", err)
	}

	names := cfg.ListInstances()
	if len(names) == 0 {
		log.Fatal("No instances defined in owlctl.yaml. Add instances under 'instances:' to use --all-instances.")
	}

	for _, name := range names {
		resolved, err := config.ResolveInstance(cfg, name)
		if err != nil {
			fmt.Printf("Warning: Skipping instance %q: %v\n", name, err)
			continue
		}
		if err := config.ActivateInstance(resolved); err != nil {
			fmt.Printf("Warning: Failed to activate instance %q: %v\n", name, err)
			continue
		}
		fn(name, resolved)
	}
}

func init() {
	globalSnapshotCmd.Flags().BoolVar(&globalSnapshotAll, "all", false, "Snapshot all resource types")
	globalSnapshotCmd.Flags().BoolVar(&globalSnapshotAllInstances, "all-instances", false, "Snapshot all configured instances from owlctl.yaml")
	rootCmd.AddCommand(globalSnapshotCmd)
}
