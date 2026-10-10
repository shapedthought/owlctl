package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/shapedthought/owlctl/state"
	"github.com/spf13/cobra"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "State management commands",
	Long: `Commands for viewing and managing owlctl state.

State is stored locally and tracks which resources have been applied or snapshotted.
`,
}

var stateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tracked resources in state",
	Long: `Display a summary table of every resource currently tracked in state.

Shows all instances. Use --instance to filter to a specific instance.

Examples:
  # List all tracked resources
  owlctl state list

  # List resources for a specific instance
  owlctl state list --instance vbr-prod
`,
	Run: func(cmd *cobra.Command, args []string) {
		listStateResources()
	},
}

var stateShowCmd = &cobra.Command{
	Use:   "show <resource-name>",
	Short: "Show full detail for a tracked resource",
	Long: `Display detailed information for a single resource in state, including
metadata and the full spec that was last applied or snapshotted.

Examples:
  # Show detail for a repository
  owlctl state show "Default Backup Repository"

  # Show detail for a job
  owlctl state show "Production Database Backup"
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		showStateResource(args[0])
	},
}

var statePathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the state file path",
	Long: `Print the path to the state.json file owlctl is using.

Useful for debugging, CI environments, or scripting.

Examples:
  owlctl state path
`,
	Run: func(cmd *cobra.Command, args []string) {
		stateMgr := state.NewManagerQuiet()
		p := stateMgr.GetStatePath()
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		if legacy := stateMgr.LegacyStatePath(); legacy != "" {
			fmt.Printf("%s (legacy: %s; run 'owlctl state migrate')\n", p, legacy)
			return
		}
		fmt.Println(p)
	},
}

var (
	migrateFrom   string
	migrateTo     string
	migrateForce  bool
	migrateDryRun bool
)

var stateMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Move legacy ~/.owlctl/state.json to the current state location",
	Long: `Move a state file from the legacy location (~/.owlctl/state.json) to where
owlctl now keeps state (next to settings.json: OWLCTL_SETTINGS_PATH or the
current directory).

The source is validated, backed up as state.json.bak-<UTC timestamp> next to
the source, then moved. Refuses to overwrite an existing destination unless
--force is given (the destination is backed up first).

Examples:
  owlctl state migrate --dry-run
  owlctl state migrate
  owlctl state migrate --from ~/old/state.json --to ./state.json --force
`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		res, err := state.MigrateState(state.MigrateOptions{
			From: migrateFrom, To: migrateTo, Force: migrateForce, DryRun: migrateDryRun,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if res.NothingToMigrate {
			fmt.Printf("Nothing to migrate: no state file at %s\n", res.From)
			return
		}
		prefix := "Done: "
		if migrateDryRun {
			prefix = "[dry-run] would "
		}
		fmt.Printf("%sback up %s -> %s\n", prefix, res.From, res.SourceBackup)
		if res.DestBackup != "" {
			fmt.Printf("%sback up existing %s -> %s\n", prefix, res.To, res.DestBackup)
		}
		fmt.Printf("%smove %s -> %s\n", prefix, res.From, res.To)
	},
}

var stateHistoryCmd = &cobra.Command{
	Use:   "history <resource-name>",
	Short: "Show audit history for a resource",
	Long: `Display the audit trail of actions taken on a resource.

Each entry shows:
- Action type (snapshotted, created, applied)
- Timestamp
- User who performed the action
- Fields changed (for apply/create actions)

Examples:
  # Show history for a repository
  owlctl state history "Default Backup Repository"

  # Show history for a job
  owlctl state history "Backup Job 1"
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		showResourceHistory(args[0])
	},
}

func listStateResources() {
	stateMgr := state.NewManager()

	if !stateMgr.StateExists() {
		fmt.Println("No state file found. Run a snapshot or apply command to initialise state.")
		fmt.Printf("Expected location: %s\n", stateMgr.GetStatePath())
		return
	}

	st, err := stateMgr.Load()
	if err != nil {
		log.Fatalf("Failed to load state: %v", err)
	}

	if len(st.Instances) == 0 {
		fmt.Println("State file exists but contains no tracked resources.")
		return
	}

	// Collect all instance names sorted
	instanceNames := make([]string, 0, len(st.Instances))
	for name := range st.Instances {
		instanceNames = append(instanceNames, name)
	}
	sort.Strings(instanceNames)

	// Filter by --instance flag if set
	filterInstance := instanceFlag
	if filterInstance != "" {
		filtered := instanceNames[:0]
		for _, n := range instanceNames {
			if n == filterInstance {
				filtered = append(filtered, n)
			}
		}
		if len(filtered) == 0 {
			fmt.Printf("No resources found for instance '%s'.\n", filterInstance)
			return
		}
		instanceNames = filtered
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "INSTANCE\tNAME\tTYPE\tORIGIN\tLAST APPLIED")

	total := 0
	for _, instName := range instanceNames {
		inst := st.Instances[instName]
		if inst == nil || len(inst.Resources) == 0 {
			continue
		}

		// Sort resource names for stable output
		resourceNames := make([]string, 0, len(inst.Resources))
		for name := range inst.Resources {
			resourceNames = append(resourceNames, name)
		}
		sort.Strings(resourceNames)

		for _, resName := range resourceNames {
			r := inst.Resources[resName]
			lastApplied := r.LastApplied.Format("2006-01-02 15:04")
			if r.LastApplied.IsZero() {
				lastApplied = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				instName, r.Name, r.Type, r.Origin, lastApplied)
			total++
		}
	}
	w.Flush()

	fmt.Printf("\n%d resource(s) tracked", total)
	if filterInstance != "" {
		fmt.Printf(" in instance '%s'", filterInstance)
	}
	fmt.Printf(" (state v%d: %s)\n", st.Version, stateMgr.GetStatePath())
}

func showStateResource(resourceName string) {
	stateMgr := state.NewManager()

	if !stateMgr.StateExists() {
		fmt.Println("No state file found. Run a snapshot or apply command to initialise state.")
		fmt.Printf("Expected location: %s\n", stateMgr.GetStatePath())
		return
	}

	// Load state directly to search all instances
	st, err := stateMgr.Load()
	if err != nil {
		log.Fatalf("Failed to load state: %v", err)
	}

	// Find the resource across all instances (prefer active instance if duplicated)
	var found *state.Resource
	var foundInstance string

	activeInst := instanceFlag
	if activeInst == "" {
		if envInst := os.Getenv("OWLCTL_ACTIVE_INSTANCE"); envInst != "" {
			activeInst = envInst
		} else {
			activeInst = "default"
		}
	}

	// First pass: look in active instance
	if inst, ok := st.Instances[activeInst]; ok && inst != nil {
		if r, ok := inst.Resources[resourceName]; ok {
			found = r
			foundInstance = activeInst
		}
	}

	// Second pass: search all instances if not found in active
	if found == nil {
		for instName, inst := range st.Instances {
			if inst == nil {
				continue
			}
			if r, ok := inst.Resources[resourceName]; ok {
				found = r
				foundInstance = instName
				break
			}
		}
	}

	if found == nil {
		log.Fatalf("Resource '%s' not found in state.", resourceName)
	}

	fmt.Printf("Resource: %s\n", found.Name)
	fmt.Printf("Instance: %s\n", foundInstance)
	fmt.Printf("Type:     %s\n", found.Type)
	fmt.Printf("ID:       %s\n", found.ID)
	fmt.Printf("Origin:   %s\n", found.Origin)
	if !found.LastApplied.IsZero() {
		fmt.Printf("Last Applied: %s by %s\n", found.LastApplied.Format("2006-01-02 15:04:05"), found.LastAppliedBy)
	}
	fmt.Printf("Spec Fields: %d\n", len(found.Spec))

	fmt.Println("\nSpec:")
	specJSON, err := json.MarshalIndent(found.Spec, "  ", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal spec: %v", err)
	}
	fmt.Printf("  %s\n", string(specJSON))
}

func showResourceHistory(resourceName string) {
	stateMgr := state.NewManager()

	resource, err := stateMgr.GetResource(resourceName)
	if err != nil {
		log.Fatalf("Resource '%s' not found in state.", resourceName)
	}

	fmt.Printf("History for %s (%s):\n\n", resource.Name, resource.Type)

	if len(resource.History) == 0 {
		fmt.Println("  No history recorded.")
		fmt.Println()
		fmt.Println("Note: History tracking was added in state version 3.")
		fmt.Println("Existing resources will start recording history on the next action.")
		return
	}

	for _, event := range resource.History {
		timestamp := event.Timestamp.Format("2006-01-02 15:04:05")

		if len(event.Fields) > 0 {
			fieldInfo := fmt.Sprintf("%d field(s)", len(event.Fields))
			if event.Partial {
				fieldInfo += " [partial]"
			}
			fmt.Printf("  %s - %s by %s (%s)\n", timestamp, event.Action, event.User, fieldInfo)

			maxFields := 3
			for i, field := range event.Fields {
				if i >= maxFields {
					fmt.Printf("      ... and %d more\n", len(event.Fields)-maxFields)
					break
				}
				fmt.Printf("      - %s\n", field)
			}
		} else {
			fmt.Printf("  %s - %s by %s\n", timestamp, event.Action, event.User)
		}
	}

	fmt.Printf("\nShowing %d event(s). Max retained: %d\n", len(resource.History), state.DefaultMaxHistoryEvents)
}

func init() {
	stateCmd.AddCommand(stateListCmd)
	stateCmd.AddCommand(stateShowCmd)
	stateCmd.AddCommand(statePathCmd)
	stateCmd.AddCommand(stateHistoryCmd)
	stateMigrateCmd.Flags().StringVar(&migrateFrom, "from", "", "Source state file (default: ~/.owlctl/state.json)")
	stateMigrateCmd.Flags().StringVar(&migrateTo, "to", "", "Destination state file (default: resolved state path)")
	stateMigrateCmd.Flags().BoolVar(&migrateForce, "force", false, "Replace an existing destination (backed up first)")
	stateMigrateCmd.Flags().BoolVar(&migrateDryRun, "dry-run", false, "Show what would happen without changing anything")
	stateCmd.AddCommand(stateMigrateCmd)
	rootCmd.AddCommand(stateCmd)
}
