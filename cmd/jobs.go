package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/shapedthought/owlctl/config"
	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/resources"
	"github.com/shapedthought/owlctl/state"
	"github.com/shapedthought/owlctl/utils"
	"github.com/shapedthought/owlctl/vhttp"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var (
	folder         string
	customTemplate string
)

var jobsCmd = &cobra.Command{
	Use:   "job",
	Short: "Job",
	Long: `Job related commands.

ONLY WORKS WITH VBR AT THE MOMENT.

Subcommands:

Create Templates
owlctl job template <job_id>

Create job from job file
owlctl create job abc-job.yaml

Create jobs from folder
owlctl create job -f .\path\to\jobs-folder

	`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Template
		if args[0] == "template" {
			getTemplates(args, folder)
		} else if args[0] == "create" {
			createJob(args, folder, customTemplate)
		}
	},
}

func getTemplates(args []string, folder string) {

	// get the job data
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with vbr at the moment.")
	}

	ju := fmt.Sprintf("jobs/%s", args[1])

	varJobs := vhttp.GetData[models.VbrJobGet](ju, profile)

	// save base job file
	utils.SaveData(&varJobs, fmt.Sprintf("job-template-%s", varJobs.Name))

	// save storage data 
	utils.SaveData(&varJobs.Storage, fmt.Sprintf("storage-%s", varJobs.Name))

	// save guest processing data
	utils.SaveData(&varJobs.GuestProcessing, fmt.Sprintf("guest-processing-%s", varJobs.Name))

	// save schedule data
	utils.SaveData(&varJobs.Schedule, fmt.Sprintf("schedule-%s", varJobs.Name))

	// VirtualMachines.Includes already has correct type after model fix
	varVms := models.VirtualMachines{}
	varVms.Includes = varJobs.VirtualMachines.Includes
	varVms.Excludes = varJobs.VirtualMachines.Excludes

	var saveJob models.VbrJob

	saveJob.Name = varJobs.Name
	saveJob.Description = varJobs.Description
	saveJob.Type = varJobs.Type
	saveJob.IsDisabled = varJobs.IsDisabled
	saveJob.VirtualMachines = varVms

	jobName := fmt.Sprintf("job-%s", varJobs.Name)
	// save job data
	utils.SaveData(&saveJob, jobName) 

	fmt.Println("Job templates created.")

}

func getSettingsPath(cp string) string {
	settingsPath := os.Getenv("OWLCTL_SETTINGS_PATH")

	var sf string
	
	if len(cp) > 0 {
		sf = cp
	} else {
		sf = "job-template.yaml"
	}

	if len(settingsPath) != 0 {
		if runtime.GOOS == "windows" {
			if !strings.HasSuffix(settingsPath, "\\") {
				settingsPath = settingsPath + "\\" + sf
			} else {
				settingsPath = settingsPath + sf
			}
		} else {
			if !strings.HasSuffix(settingsPath, "/") {
				settingsPath = settingsPath + "/" + sf
			} else {
				settingsPath = settingsPath + sf
			}
		}
	} else {
		log.Fatal("OWLCTL_SETTINGS_PATH not set")
	}

	return settingsPath
}


func createJob(args []string, folder string, customTemplate string) {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with vbr at the moment.")
	}

	var templateFile models.VbrJobPost

	settingsPath := getSettingsPath(customTemplate)

	getYaml, err := os.Open(settingsPath)
	if err != nil {
		log.Fatal("Error opening job-template.yaml")
	}

	b, err := io.ReadAll(getYaml)
	utils.IsErr(err)

	err = yaml.Unmarshal(b, &templateFile)
	utils.IsErr(err)

	api_url := utils.GetAddress(profile, settings)

	if len(folder) > 0  {

		// get all files in folder
		fmt.Println("Reading folder: ", folder)
		files, err := os.ReadDir(folder)
		if err != nil {
			log.Fatal("Error reading folder")
		}

		if len(files) == 0 {
			log.Fatal("No files found in folder")
		}

		var vbrJob models.VbrJob 

		for _, file := range files {
			fp := fmt.Sprintf("%s%s", folder, file.Name())
			if strings.Contains(fp, "job") {
				getYaml, err := os.Open(fp)
				if err != nil {
					log.Fatalf("Error opening job file: %v", err)
				}

				b, err := io.ReadAll(getYaml)
				utils.IsErr(err)

				err = yaml.Unmarshal(b, &vbrJob)
				utils.IsErr(err)

				templateFile.Type = vbrJob.Type
				templateFile.Name = vbrJob.Name
				templateFile.Description = vbrJob.Description
				templateFile.VirtualMachines = vbrJob.VirtualMachines
			}
			
			if strings.Contains(fp, "storage") {
				fmt.Println("Storage file found: ", file.Name())
				var vbrStorage models.Storage

				getYaml, err := os.Open(fp)
				if err != nil {
					log.Fatalf("Error opening storage file: %v", err)
				}

				b, err := io.ReadAll(getYaml)
				utils.IsErr(err)

				err = yaml.Unmarshal(b, &vbrStorage)
				utils.IsErr(err)

				templateFile.Storage = vbrStorage
			} 
			
			if strings.Contains(fp, "guest-processing") {
				fmt.Println("Guest processing file found: ", file.Name())
				var vbrGuestProcessing models.GuestProcessing

				getYaml, err := os.Open(fp)
				if err != nil {
					log.Fatalf("Error opening guest-processing file: %v", err)
				}

				b, err := io.ReadAll(getYaml)
				utils.IsErr(err)

				err = yaml.Unmarshal(b, &vbrGuestProcessing)
				utils.IsErr(err)

				templateFile.GuestProcessing = vbrGuestProcessing
			} 
			
			if strings.Contains(fp, "schedule") {
				fmt.Println("Schedule file found: ", file.Name())
				var vbrSchedule models.Schedule

				getYaml, err := os.Open(fp)
				if err != nil {
					log.Fatalf("Error opening schedule file: %v", err)
				}

				b, err := io.ReadAll(getYaml)
				utils.IsErr(err)

				err = yaml.Unmarshal(b, &vbrSchedule)
				utils.IsErr(err)

				templateFile.Schedule = vbrSchedule
			}
		}
	} else if len(args) > 1 {

		var vbrJob models.VbrJob 

		getYaml, err := os.Open(args[1])
		
		utils.IsErr(err)

		b, err := io.ReadAll(getYaml)
		utils.IsErr(err)

		err = yaml.Unmarshal(b, &vbrJob)
		utils.IsErr(err)

		utils.SaveJson(&templateFile, fmt.Sprintf("%s-job", vbrJob.Name))

		templateFile.Type = vbrJob.Type
		templateFile.Name = vbrJob.Name
		templateFile.Description = vbrJob.Description
		templateFile.VirtualMachines = vbrJob.VirtualMachines

	} else {
		fmt.Printf("No file or folder passed")
	}

	sendData, err := json.Marshal(templateFile)
	utils.IsErr(err)

	connstring := fmt.Sprintf("https://%v:%v%v%v/%v", api_url, profile.Port, "/api/", profile.APIVersion, "/jobs")

	r, err := http.NewRequest("POST", connstring, bytes.NewReader(sendData))

	utils.IsErr(err)
	headers := utils.ReadHeader[models.SendHeader]()
	r.Header.Add("x-api-version", profile.Headers.XAPIVersion)
	r.Header.Add("Content-Type", "application/json")

	r.Header.Add("Authorization", "Bearer "+headers.AccessToken)

	client := vhttp.Client(settings.ApiNotSecure)
	res, err := client.Do(r)
	if err != nil {
		fmt.Printf("Error sending HTTP request %v\n", err)
		return
	}

	defer res.Body.Close()

	fmt.Println("Status Code:", res.StatusCode)
	fmt.Println("Status:", res.Status)

}

var (
	diffAll       bool
	diffGroupName string
	snapshotAll   bool
)

var diffCmd = &cobra.Command{
	Use:   "diff [job-name]",
	Short: "Detect configuration drift from applied state",
	Long: `Compare current VBR job configuration against the last applied state
to detect manual changes or drift.

Each drift is classified by severity: CRITICAL, WARNING, or INFO.

Examples:
  # Check single job for drift
  owlctl job diff SQL-Backup-Job

  # Check all jobs
  owlctl job diff --all

  # Show only security-relevant drifts
  owlctl job diff --all --security-only

  # Show only critical drifts
  owlctl job diff --all --severity critical

Exit Codes:
  0 - No drift detected
  3 - Drift detected (INFO or WARNING)
  4 - Critical security drift detected
  1 - Error occurred`,
	Run: func(cmd *cobra.Command, args []string) {
		if diffGroupName != "" {
			// Validate mutual exclusivity
			if diffAll {
				log.Fatal("Cannot use --group with --all")
			}
			if len(args) > 0 {
				log.Fatal("Cannot use --group with a positional job name argument")
			}
			diffGroup(diffGroupName)
		} else if diffAll {
			diffAllJobs()
		} else if len(args) > 0 {
			diffSingleJob(args[0])
		} else {
			log.Fatal("Provide job name, use --all, or use --group")
		}
	},
}

var snapshotCmd = &cobra.Command{
	Use:   "snapshot [job-name]",
	Short: "Capture current job configuration into state for drift monitoring",
	Long: `Snapshot captures the current configuration of a job from VBR and saves it
to state with origin: "observed". This allows drift detection without actively
managing the job via apply.

Use this to:
- Track jobs created manually in VBR
- Monitor jobs created before owlctl adoption
- Observe configuration changes without management

Snapshots enable drift detection via 'owlctl job diff' but do not allow
remediation via 'owlctl job apply' (use 'apply' to actively manage jobs).

Examples:
  # Snapshot a single job
  owlctl job snapshot "SQL Backup Job"

  # Snapshot all jobs
  owlctl job snapshot --all

After snapshotting, use 'owlctl job diff' to detect drift:
  owlctl job diff "SQL Backup Job"
  owlctl job diff --all --security-only`,
	Run: func(cmd *cobra.Command, args []string) {
		if snapshotAll {
			snapshotAllJobs()
		} else if len(args) > 0 {
			snapshotSingleJob(args[0])
		} else {
			log.Fatal("Provide job name or use --all")
		}
	},
}

func diffSingleJob(jobName string) {
	loadSeverityOverrides()
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	// Load from state
	stateMgr := state.NewManager()
	resource, err := stateMgr.GetResource(jobName)
	if err != nil {
		log.Fatalf("Job '%s' not found in state. Has it been applied?\n", jobName)
	}

	// Show (observed) label for monitored-only resources
	originLabel := ""
	if resource.Origin == "observed" {
		originLabel = " (observed)"
	}
	fmt.Printf("Checking drift for job: %s%s\n\n", jobName, originLabel)

	// Fetch current from VBR as raw JSON (matches snapshot storage format)
	endpoint := fmt.Sprintf("jobs/%s", resource.ID)
	currentRaw := vhttp.GetData[json.RawMessage](endpoint, profile)

	var currentMap map[string]interface{}
	if err := json.Unmarshal(currentRaw, &currentMap); err != nil {
		log.Fatalf("Failed to unmarshal current job data: %v", err)
	}

	// Compare, classify, enhance, filter
	drifts := detectDrift(resource.Spec, currentMap, jobIgnoreFields)
	drifts = classifyDrifts(drifts, jobSeverityMap)
	drifts = enhanceJobDriftSeverity(drifts)
	drifts = checkRepoHardeningDrift(drifts, resource.Spec)
	minSev := parseSeverityFlag()
	drifts = filterDriftsBySeverity(drifts, minSev)

	if len(drifts) == 0 {
		fmt.Println(noDriftMessage("Job matches applied state.", minSev))
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
	printRemediationGuidance(BuildJobGuidance(jobName, resource.ID, resource.Origin))

	os.Exit(exitCodeForDrifts(drifts))
}

func diffAllJobs() {
	loadSeverityOverrides()
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	stateMgr := state.NewManager()
	resources, err := stateMgr.ListResources("VBRJob")
	if err != nil {
		log.Fatalf("Failed to load state: %v\n", err)
	}

	if len(resources) == 0 {
		fmt.Println("No jobs in state.")
		return
	}

	fmt.Printf("Checking %d jobs for drift...\n\n", len(resources))

	minSev := parseSeverityFlag()
	cleanCount := 0
	driftedApplied := 0
	driftedObserved := 0
	var allDrifts []Drift

	for _, resource := range resources {
		// Fetch current from VBR as raw JSON (matches snapshot storage format)
		endpoint := fmt.Sprintf("jobs/%s", resource.ID)
		currentRaw := vhttp.GetData[json.RawMessage](endpoint, profile)

		var currentMap map[string]interface{}
		if err := json.Unmarshal(currentRaw, &currentMap); err != nil {
			fmt.Printf("  %s: Failed to unmarshal job data: %v\n", resource.Name, err)
			continue
		}

		// Detect, classify, enhance, filter
		drifts := detectDrift(resource.Spec, currentMap, jobIgnoreFields)
		drifts = classifyDrifts(drifts, jobSeverityMap)
		drifts = enhanceJobDriftSeverity(drifts)
		drifts = checkRepoHardeningDrift(drifts, resource.Spec)
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

	totalDrifted := driftedApplied + driftedObserved
	if totalDrifted > 0 {
		fmt.Println()
		printSecuritySummary(allDrifts)
	}

	fmt.Printf("Summary:\n")
	fmt.Printf("  - %d jobs clean\n", cleanCount)
	if driftedApplied > 0 {
		fmt.Printf("  - %d jobs drifted — remediate with: owlctl job apply <spec>.yaml\n", driftedApplied)
	}
	if driftedObserved > 0 {
		fmt.Printf("  - %d jobs drifted (observed, monitor-only) — export then apply a spec to enable remediation\n", driftedObserved)
	}

	if totalDrifted > 0 {
		os.Exit(exitCodeForDrifts(allDrifts))
	}
	os.Exit(0)
}

// diffGroup compares merged group specs (profile+spec+overlay) against live VBR state.
// Unlike state-based diff, group diff does NOT require state.json — the group definition
// IS the source of truth.
func diffGroup(group string) {
	loadSeverityOverrides()

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

	// Resolve paths
	profilePath := ""
	if groupCfg.Profile != "" {
		profilePath = cfg.ResolvePath(groupCfg.Profile)
	}
	overlayPath := ""
	if groupCfg.Overlay != "" {
		overlayPath = cfg.ResolvePath(groupCfg.Overlay)
	}

	fmt.Printf("Checking drift for group: %s (%d specs)\n", group, len(specsList))
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

	minSev := parseSeverityFlag()
	cleanCount := 0
	driftedCount := 0
	notFoundCount := 0
	errorCount := 0
	var allDrifts []Drift

	for _, specRelPath := range specsList {
		specPath := cfg.ResolvePath(specRelPath)

		// Load spec
		spec, err := resources.LoadResourceSpec(specPath)
		if err != nil {
			fmt.Printf("  %s: Failed to load spec: %v\n", specRelPath, err)
			errorCount++
			continue
		}

		// Compute desired state from group merge using cached profile/overlay
		desiredSpec, err := resources.ApplyGroupMergeFromSpecs(spec, profileSpec, overlaySpec, opts)
		if err != nil {
			fmt.Printf("  %s: Failed to merge: %v\n", specRelPath, err)
			errorCount++
			continue
		}

		jobName := desiredSpec.Metadata.Name

		// Fetch current from VBR
		currentRaw, _, fetchErr := fetchCurrentJob(jobName, profile)
		if fetchErr != nil {
			fmt.Printf("  %s: Failed to fetch current job: %v\n", jobName, fetchErr)
			errorCount++
			continue
		}
		if currentRaw == nil {
			fmt.Printf("  %s: Not found in VBR (would be created by apply)\n", jobName)
			notFoundCount++
			continue
		}

		var currentMap map[string]interface{}
		if err := json.Unmarshal(currentRaw, &currentMap); err != nil {
			fmt.Printf("  %s: Failed to unmarshal current job: %v\n", jobName, err)
			errorCount++
			continue
		}

		// Compare merged desired spec against live VBR
		drifts := detectDrift(desiredSpec.Spec, currentMap, jobIgnoreFields)
		drifts = classifyDrifts(drifts, jobSeverityMap)
		drifts = enhanceJobDriftSeverity(drifts)
		drifts = checkRepoHardeningDrift(drifts, desiredSpec.Spec)
		drifts = filterDriftsBySeverity(drifts, minSev)

		if len(drifts) > 0 {
			maxSev := getMaxSeverity(drifts)
			fmt.Printf("  %s %s: %d drifts detected\n", maxSev, jobName, len(drifts))
			allDrifts = append(allDrifts, drifts...)
			driftedCount++
		} else {
			fmt.Printf("  %s: No drift\n", jobName)
			cleanCount++
		}
	}

	// Summary
	if driftedCount > 0 {
		fmt.Println()
		printSecuritySummary(allDrifts)
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("  - %d jobs clean\n", cleanCount)
	if driftedCount > 0 {
		fmt.Printf("  - %d jobs drifted — remediate with: owlctl job apply --group %s\n", driftedCount, group)
	}
	if notFoundCount > 0 {
		fmt.Printf("  - %d jobs not found in VBR (would be created by apply)\n", notFoundCount)
	}
	if errorCount > 0 {
		fmt.Printf("  - %d specs failed to evaluate (see errors above)\n", errorCount)
	}

	if errorCount > 0 {
		os.Exit(ExitError)
	}
	if driftedCount > 0 {
		os.Exit(exitCodeForDrifts(allDrifts))
	}
	os.Exit(0)
}

// snapshotSingleJob captures a single job's current configuration into state
func snapshotSingleJob(jobName string) {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	// Fetch job by name using generic fetch
	jobData, jobID, fetchErr := fetchCurrentJob(jobName, profile)
	if fetchErr != nil {
		log.Fatalf("Failed to fetch job '%s': %v", jobName, fetchErr)
	}
	if jobData == nil {
		log.Fatalf("Job '%s' not found in VBR.", jobName)
	}
	if err := saveJobToState(jobName, jobID, jobData); err != nil {
		log.Fatalf("Failed to save job state: %v", err)
	}

	fmt.Printf("Snapshot saved for job: %s\n", jobName)
}

// snapshotAllJobs captures all jobs' current configurations into state
func snapshotAllJobs() {
	settings := utils.ReadSettings()
	profile := utils.GetCurrentProfile()

	if settings.SelectedProfile != "vbr" {
		log.Fatal("This command only works with VBR at the moment.")
	}

	type JobListItem struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type JobListResponse struct {
		Data []JobListItem `json:"data"`
	}

	jobList := vhttp.GetData[JobListResponse]("jobs", profile)

	if len(jobList.Data) == 0 {
		fmt.Println("No jobs found.")
		return
	}

	fmt.Printf("Snapshotting %d jobs...\n", len(jobList.Data))

	for _, job := range jobList.Data {
		// Fetch full details for each job
		endpoint := fmt.Sprintf("jobs/%s", job.ID)
		jobData := vhttp.GetData[json.RawMessage](endpoint, profile)

		if err := saveJobToState(job.Name, job.ID, jobData); err != nil {
			fmt.Printf("Warning: Failed to save state for '%s': %v\n", job.Name, err)
			continue
		}

		fmt.Printf("  Snapshot saved: %s\n", job.Name)
	}

	stateMgr := state.NewManager()
	fmt.Printf("\nState updated: %s\n", stateMgr.GetStatePath())
}

// saveJobToState saves a job to state with origin: "observed"
func saveJobToState(name, id string, rawData json.RawMessage) error {
	return saveResourceToState("VBRJob", name, id, rawData)
}

func init() {
	diffCmd.Flags().BoolVar(&diffAll, "all", false, "Check drift for all jobs in state")
	diffCmd.Flags().StringVar(&diffGroupName, "group", "", "Check drift for all specs in named group (from owlctl.yaml)")
	addSeverityFlags(diffCmd)
	jobsCmd.AddCommand(diffCmd)

	snapshotCmd.Flags().BoolVar(&snapshotAll, "all", false, "Snapshot all jobs")
	jobsCmd.AddCommand(snapshotCmd)

	jobsCmd.Flags().StringVarP(&folder, "folder", "f", "", "folder input (legacy template system)")
	jobsCmd.Flags().StringVarP(&customTemplate, "template", "t", "", "custom template (legacy template system)")
	rootCmd.AddCommand(jobsCmd)
}