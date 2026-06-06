package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/resources"
	"github.com/shapedthought/owlctl/utils"
	"github.com/shapedthought/owlctl/vhttp"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	exportOutput     string
	exportDirectory  string
	exportAll        bool
	exportSimplified bool
	exportAsOverlay  bool
	exportBasePath   string
)

var exportCmd = &cobra.Command{
	Use:   "export [job-id]",
	Short: "Export VBR jobs to declarative YAML format",
	Long: `Export existing VBR backup jobs to declarative YAML configuration files.

Examples:
  # Export single job to stdout
  owlctl export 57b3baab-6237-41bf-add7-db63d41d984c

  # Export single job to file
  owlctl export 57b3baab-6237-41bf-add7-db63d41d984c -o backup.yaml

  # Export as overlay (minimal patch with only changed fields)
  owlctl export 57b3baab-6237-41bf-add7-db63d41d984c --as-overlay -o overlay.yaml

  # Export as overlay with specific base
  owlctl export 57b3baab-6237-41bf-add7-db63d41d984c --as-overlay --base base/defaults.yaml -o overlay.yaml

  # Export all jobs to current directory
  owlctl export --all

  # Export all jobs to specific directory
  owlctl export --all -d ./configs/
`,
	Run: func(cmd *cobra.Command, args []string) {
		settings := utils.ReadSettings()
		profile := utils.GetCurrentProfile()

		if settings.SelectedProfile != "vbr" {
			log.Fatal("This command only works with VBR at the moment.")
		}

		if exportAll {
			exportAllJobs(profile)
		} else {
			if len(args) == 0 {
				log.Fatal("Job ID is required. Use --all to export all jobs.")
			}
			exportSingleJob(args[0], profile)
		}
	},
}

func exportSingleJob(jobID string, profile models.Profile) {
	// Fetch job from VBR as raw JSON (preserves all fields regardless of job type)
	endpoint := fmt.Sprintf("jobs/%s", jobID)
	rawData := vhttp.GetData[json.RawMessage](endpoint, profile)

	// Extract name and ID from raw JSON
	var meta struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rawData, &meta); err != nil {
		log.Fatalf("Failed to parse job response: %v", err)
	}

	// Convert to declarative YAML
	yamlContent, err := convertJobToYAML(meta.Name, meta.ID, rawData)
	if err != nil {
		log.Fatalf("Failed to convert job to YAML: %v", err)
	}

	// Output to file or stdout
	if exportOutput != "" {
		if err := os.WriteFile(exportOutput, yamlContent, 0644); err != nil {
			log.Fatalf("Failed to write to file: %v", err)
		}
		fmt.Printf("Exported job to %s\n", exportOutput)
	} else {
		fmt.Println(string(yamlContent))
	}
}

func exportAllJobs(profile models.Profile) {
	outputDir := exportDirectory
	if outputDir == "" {
		outputDir = "."
	}
	if err := exportAllJobsToDir(outputDir, profile); err != nil {
		log.Fatalf("Export failed: %v", err)
	}
}

// exportAllJobsToDir exports all VBR jobs to the specified directory.
// Registry-compatible: used by the global export command.
func exportAllJobsToDir(outDir string, profile models.Profile) error {
	type JobListItem struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type JobList struct {
		Data []JobListItem `json:"data"`
	}

	jobs := vhttp.GetData[JobList]("jobs", profile)

	if len(jobs.Data) == 0 {
		fmt.Println("No jobs found")
		return nil
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	successCount := 0
	failedCount := 0

	fmt.Printf("Exporting %d jobs...\n", len(jobs.Data))

	for i, job := range jobs.Data {
		endpoint := fmt.Sprintf("jobs/%s", job.ID)
		rawData := vhttp.GetData[json.RawMessage](endpoint, profile)

		yamlContent, err := convertJobToYAMLFull(job.Name, job.ID, rawData)
		if err != nil {
			fmt.Printf("Warning: Failed to convert job %s: %v\n", job.Name, err)
			failedCount++
			continue
		}

		filename := sanitizeFilename(job.Name) + ".yaml"
		fp := filepath.Join(outDir, filename)

		if err := os.WriteFile(fp, yamlContent, 0644); err != nil {
			fmt.Printf("Warning: Failed to write %s: %v\n", filename, err)
			failedCount++
			continue
		}

		successCount++
		fmt.Printf("  [%d/%d] Exported %s\n", i+1, len(jobs.Data), filename)
	}

	fmt.Printf("\nExport complete: %d successful, %d failed\n", successCount, failedCount)
	return nil
}

func convertJobToYAML(name, id string, rawData json.RawMessage) ([]byte, error) {
	if exportAsOverlay {
		return convertJobToYAMLOverlay(name, id, rawData, exportBasePath)
	}
	if exportSimplified {
		return convertJobToYAMLSimplified(name, id, rawData)
	}
	return convertJobToYAMLFull(name, id, rawData)
}

func convertJobToYAMLFull(name, id string, rawData json.RawMessage) ([]byte, error) {
	// Unmarshal raw JSON directly to map (preserves all fields from any job type)
	var jobMap map[string]interface{}
	if err := json.Unmarshal(rawData, &jobMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	// Create full resource spec with complete job object
	resourceSpec := resources.ResourceSpec{
		APIVersion: "owlctl.veeam.com/v1",
		Kind:       "VBRJob",
		Metadata: resources.Metadata{
			Name: name,
		},
		Spec: jobMap,
	}

	// Add header comment
	header := fmt.Sprintf("# VBR Job Configuration (Full Export)\n# Exported from VBR\n# Job ID: %s\n# API Version: 1.3-rev1\n#\n# This export contains the complete job configuration.\n# All fields from the VBR API are preserved.\n\n", id)

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(resourceSpec)
	if err != nil {
		return nil, err
	}

	// Combine header and YAML
	result := []byte(header)
	result = append(result, yamlBytes...)

	return result, nil
}

func convertJobToYAMLSimplified(name, id string, rawData json.RawMessage) ([]byte, error) {
	// Simplified export is VM-specific — unmarshal to typed struct internally
	var vbrJob models.VbrJobGet
	if err := json.Unmarshal(rawData, &vbrJob); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job for simplified export: %w", err)
	}

	// Create resolver for name resolution
	resolver := resources.NewResolver()

	// Build simplified spec
	spec := resources.VBRJobSpec{
		Type:        vbrJob.Type,
		Description: vbrJob.Description,
		IsDisabled:  vbrJob.IsDisabled,
	}

	// Convert VMs
	for _, include := range vbrJob.VirtualMachines.Includes {
		spec.Objects = append(spec.Objects, resources.JobObject{
			Type:     include.Type,
			Name:     include.Name,
			HostName: include.HostName,
		})
	}

	// Resolve repository name
	repoName, err := resolver.ResolveRepositoryName(vbrJob.Storage.BackupRepositoryID)
	if err != nil {
		// If we can't resolve, use the ID
		repoName = vbrJob.Storage.BackupRepositoryID
	}
	spec.Repository = repoName

	// Convert schedule
	if vbrJob.Schedule.RunAutomatically {
		spec.Schedule = &resources.JobSchedule{
			Enabled: true,
		}
		if vbrJob.Schedule.Daily.IsEnabled {
			spec.Schedule.Daily = vbrJob.Schedule.Daily.LocalTime
		}
		if vbrJob.Schedule.Retry.IsEnabled {
			spec.Schedule.Retry = &struct {
				Enabled bool `yaml:"enabled" json:"enabled"`
				Times   int  `yaml:"times,omitempty" json:"times,omitempty"`
				Wait    int  `yaml:"wait,omitempty" json:"wait,omitempty"`
			}{
				Enabled: true,
				Times:   vbrJob.Schedule.Retry.RetryCount,
				Wait:    vbrJob.Schedule.Retry.AwaitMinutes,
			}
		}
	}

	// Convert storage settings
	spec.Storage = &resources.JobStorageSettings{
		Compression: vbrJob.Storage.AdvancedSettings.StorageData.CompressionLevel,
		Encryption:  vbrJob.Storage.AdvancedSettings.StorageData.Encryption.IsEnabled,
		Retention: &struct {
			Type     string `yaml:"type,omitempty" json:"type,omitempty"`
			Quantity int    `yaml:"quantity,omitempty" json:"quantity,omitempty"`
		}{
			Type:     vbrJob.Storage.RetentionPolicy.Type,
			Quantity: vbrJob.Storage.RetentionPolicy.Quantity,
		},
	}

	// Create full resource spec
	resourceSpec := resources.ResourceSpec{
		APIVersion: "owlctl.veeam.com/v1",
		Kind:       "VBRJob",
		Metadata: resources.Metadata{
			Name: name,
		},
		Spec: make(map[string]interface{}),
	}

	// Convert spec to map
	specBytes, err := yaml.Marshal(spec)
	if err != nil {
		return nil, err
	}

	var specMap map[string]interface{}
	if err := yaml.Unmarshal(specBytes, &specMap); err != nil {
		return nil, err
	}
	resourceSpec.Spec = specMap

	// Add header comment
	header := fmt.Sprintf("# VBR Job Configuration\n# Exported from VBR\n# Job ID: %s\n\n", id)

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(resourceSpec)
	if err != nil {
		return nil, err
	}

	// Combine header and YAML
	result := []byte(header)
	result = append(result, yamlBytes...)

	return result, nil
}

func convertJobToYAMLOverlay(name, id string, rawData json.RawMessage, basePath string) ([]byte, error) {
	var baseSpec map[string]interface{}

	// Load base if provided
	if basePath != "" {
		baseData, err := os.ReadFile(basePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read base file: %w", err)
		}

		var baseResource resources.ResourceSpec
		if err := yaml.Unmarshal(baseData, &baseResource); err != nil {
			return nil, fmt.Errorf("failed to parse base file: %w", err)
		}
		baseSpec = baseResource.Spec
	}

	// Unmarshal raw JSON directly to map
	var jobMap map[string]interface{}
	if err := json.Unmarshal(rawData, &jobMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	// Calculate overlay (differences from base)
	var overlaySpec map[string]interface{}
	if baseSpec != nil {
		// Diff against provided base
		overlaySpec = calculateDiff(baseSpec, jobMap)
	} else {
		// No base provided - create minimal overlay with commonly changed fields
		overlaySpec = extractCommonFields(jobMap)
	}

	// Create resource spec
	resourceSpec := resources.ResourceSpec{
		APIVersion: "owlctl.veeam.com/v1",
		Kind:       "VBRJob",
		Metadata: resources.Metadata{
			Name: name,
		},
		Spec: overlaySpec,
	}

	// Add header comment
	header := "# VBR Job Overlay\n"
	if basePath != "" {
		header += fmt.Sprintf("# Base: %s\n", basePath)
	}
	header += fmt.Sprintf("# Job ID: %s\n", id)
	header += "#\n# This overlay contains only the fields that differ from the base.\n# Apply with: owlctl job apply base.yaml --overlay this-file.yaml\n\n"

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(resourceSpec)
	if err != nil {
		return nil, err
	}

	// Combine header and YAML
	result := []byte(header)
	result = append(result, yamlBytes...)

	return result, nil
}

// calculateDiff compares job against base and returns only differences
func calculateDiff(base, job map[string]interface{}) map[string]interface{} {
	diff := make(map[string]interface{})

	for key, jobValue := range job {
		// Skip read-only fields
		if key == "id" {
			continue
		}

		baseValue, existsInBase := base[key]

		if !existsInBase {
			// Field only in job, include it
			diff[key] = jobValue
			continue
		}

		// Both exist - check if different
		if !deepEqual(baseValue, jobValue) {
			// Different - add to diff
			switch jv := jobValue.(type) {
			case map[string]interface{}:
				// Recursively diff nested objects
				if bv, ok := baseValue.(map[string]interface{}); ok {
					nestedDiff := calculateDiff(bv, jv)
					if len(nestedDiff) > 0 {
						diff[key] = nestedDiff
					}
				} else {
					diff[key] = jobValue
				}
			default:
				diff[key] = jobValue
			}
		}
	}

	return diff
}

// extractCommonFields extracts commonly changed fields when no base provided
// This is intentionally minimal - it only extracts the most frequently modified fields
// (description, retention policy, and daily schedule). If you need more fields in the
// overlay, provide a base template with --base to get a full diff calculation.
func extractCommonFields(job map[string]interface{}) map[string]interface{} {
	overlay := make(map[string]interface{})

	// Include description if present
	if desc, ok := job["description"].(string); ok && desc != "" {
		overlay["description"] = desc
	}

	// Include storage retention if present
	if storage, ok := job["storage"].(map[string]interface{}); ok {
		if retentionPolicy, ok := storage["retentionPolicy"].(map[string]interface{}); ok {
			overlay["storage"] = map[string]interface{}{
				"retentionPolicy": retentionPolicy,
			}
		}
	}

	// Include schedule if present
	if schedule, ok := job["schedule"].(map[string]interface{}); ok {
		if daily, ok := schedule["daily"].(map[string]interface{}); ok {
			overlay["schedule"] = map[string]interface{}{
				"daily": daily,
			}
		}
	}

	return overlay
}

// deepEqual compares two values for equality with numeric type normalization.
// JSON unmarshals numbers as float64, while YAML unmarshals them as int.
// This prevents false diffs like 1 (int) != 1.0 (float64).
func deepEqual(a, b interface{}) bool {
	aNum, aIsNum := tryParseFloat64(a)
	bNum, bIsNum := tryParseFloat64(b)
	if aIsNum && bIsNum {
		return aNum == bNum
	}
	return reflect.DeepEqual(a, b)
}

func sanitizeFilename(name string) string {
	// Replace invalid filename characters with hyphens
	reg := regexp.MustCompile(`[<>:"/\\|?*]`)
	sanitized := reg.ReplaceAllString(name, "-")

	// Replace spaces with hyphens
	sanitized = strings.ReplaceAll(sanitized, " ", "-")

	// Convert to lowercase
	sanitized = strings.ToLower(sanitized)

	// Remove multiple consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	sanitized = reg.ReplaceAllString(sanitized, "-")

	// Trim hyphens from start and end
	sanitized = strings.Trim(sanitized, "-")

	return sanitized
}

func init() {
	// Register job export as subcommand of jobsCmd (preferred location)
	addExportFlags(jobExportCmd)
	jobsCmd.AddCommand(jobExportCmd)
}

// addExportFlags registers the common export flags on a command
func addExportFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().StringVarP(&exportDirectory, "directory", "d", "", "Output directory for bulk export")
	cmd.Flags().BoolVar(&exportAll, "all", false, "Export all jobs")
	cmd.Flags().BoolVar(&exportSimplified, "simplified", false, "Export simplified format (legacy)")
	cmd.Flags().BoolVar(&exportAsOverlay, "as-overlay", false, "Export as overlay (minimal patch)")
	cmd.Flags().StringVar(&exportBasePath, "base", "", "Base template to diff against (for overlay export)")
}

// jobExportCmd is the preferred location under 'owlctl job export'
var jobExportCmd = &cobra.Command{
	Use:   "export [job-id]",
	Short: "Export VBR jobs to declarative YAML format",
	Long: `Export existing VBR backup jobs to declarative YAML configuration files.

Examples:
  # Export single job to stdout
  owlctl job export 57b3baab-6237-41bf-add7-db63d41d984c

  # Export single job to file
  owlctl job export 57b3baab-6237-41bf-add7-db63d41d984c -o backup.yaml

  # Export as overlay (minimal patch with only changed fields)
  owlctl job export 57b3baab-6237-41bf-add7-db63d41d984c --as-overlay -o overlay.yaml

  # Export as overlay with specific base
  owlctl job export 57b3baab-6237-41bf-add7-db63d41d984c --as-overlay --base base/defaults.yaml -o overlay.yaml

  # Export all jobs to current directory
  owlctl job export --all

  # Export all jobs to specific directory
  owlctl job export --all -d ./configs/
`,
	Run: exportCmd.Run,
}
