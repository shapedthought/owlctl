package cmd

import (
	"encoding/json"
	"fmt"
	"os/user"
	"strings"
	"time"

	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/remediation"
	"github.com/shapedthought/owlctl/resources"
	"github.com/shapedthought/owlctl/state"
	"github.com/shapedthought/owlctl/vhttp"
)

// ApplyMode determines how apply handles missing resources
type ApplyMode int

const (
	// ApplyUpdateOnly only allows updating existing resources (PUT only).
	// Returns an error if the resource doesn't exist.
	// Used for repos, SOBRs, KMS servers which are created via VBR console.
	ApplyUpdateOnly ApplyMode = iota

	// ApplyCreateOrUpdate creates new resources or updates existing ones (POST or PUT).
	// Used for jobs which can be created via API.
	ApplyCreateOrUpdate
)

// ResourceApplyConfig defines how to apply a specific resource type
type ResourceApplyConfig struct {
	// Kind is the expected resource kind (e.g., "VBRJob", "VBRRepository")
	Kind string

	// Endpoint is the API endpoint (e.g., "jobs", "backupInfrastructure/repositories")
	Endpoint string

	// IgnoreFields are fields to exclude from the PUT payload (read-only fields)
	IgnoreFields map[string]bool

	// Mode determines whether POST is allowed for new resources
	Mode ApplyMode

	// FetchCurrent retrieves the current resource from VBR by name.
	// Returns (rawJSON, resourceID, error).
	// If not found, returns (nil, "", nil) - not an error.
	FetchCurrent func(name string, profile models.Profile) (json.RawMessage, string, error)

	// PreparePayload optionally transforms the merged spec before sending.
	// If nil, the merged spec is sent as-is.
	PreparePayload func(spec, existing map[string]interface{}) (map[string]interface{}, error)

	// PostCreate is called after creating a new resource (ApplyCreateOrUpdate mode only).
	// Returns the resource ID from the API response.
	// If nil, a default implementation extracts "id" from the response.
	PostCreate func(spec map[string]interface{}, profile models.Profile, endpoint string) (string, error)

	// PostApply is called after a successful create or update with the desired
	// spec and the resource ID. A returned error fails the apply (state is still
	// recorded, since the resource itself was written).
	PostApply func(spec map[string]interface{}, id string, profile models.Profile) error

	// PlanPostApply optionally describes the PostApply action for --dry-run.
	// existing is nil when the resource would be created. Returns "" for no action.
	PlanPostApply func(spec, existing map[string]interface{}) string
}

// ApplyResult contains the result of an apply operation
type ApplyResult struct {
	ResourceName string
	ResourceID   string
	Action       string         // "created", "updated", "would-create", "would-update"
	NotFound     bool           // True if resource not found in update-only mode
	Changes      []FieldChange  // Fields that were changed
	Skipped      []SkippedField // Fields that were skipped due to policy/known immutability
	DryRun       bool           // True if this was a dry-run (no changes made)
	Error        error
}

// applyResource applies a resource spec file to VBR using the provided config.
// It loads the spec from disk, then delegates to applyResourceSpec.
func applyResource(specFile string, cfg ResourceApplyConfig, profile models.Profile, dryRun bool) ApplyResult {
	spec, err := resources.LoadResourceSpec(specFile)
	if err != nil {
		return ApplyResult{Error: fmt.Errorf("failed to load spec file: %w", err), DryRun: dryRun}
	}
	return applyResourceSpec(spec, cfg, profile, dryRun, nil)
}

// applyWithOptionalOverlay applies a resource spec with an optional overlay merge.
// Used by all apply commands that support --overlay.
func applyWithOptionalOverlay(specFile, overlayFile string, cfg ResourceApplyConfig, profile models.Profile, dryRun bool) ApplyResult {
	if overlayFile != "" {
		fmt.Printf("Applying overlay: %s\n", overlayFile)
		mergedSpec, err := resources.MergeYAMLFiles(specFile, overlayFile, resources.DefaultMergeOptions())
		if err != nil {
			return ApplyResult{Error: fmt.Errorf("failed to merge with overlay: %w", err), DryRun: dryRun}
		}
		return applyResourceSpec(mergedSpec, cfg, profile, dryRun, nil)
	}
	return applyResource(specFile, cfg, profile, dryRun)
}

// applyResourceSpec applies a pre-loaded ResourceSpec to VBR using the provided config.
// It handles fetching existing resources, merging, and state updates.
// If dryRun is true, it fetches current state (read-only) and displays what would change,
// but makes no modifications to VBR and does not update state.
// If cachedRemCfg is non-nil, it is used instead of loading remediation config from disk.
func applyResourceSpec(spec resources.ResourceSpec, cfg ResourceApplyConfig, profile models.Profile, dryRun bool, cachedRemCfg *remediation.Config) ApplyResult {
	result := ApplyResult{DryRun: dryRun}

	result.ResourceName = spec.Metadata.Name

	// Validate resource kind
	if spec.Kind != cfg.Kind {
		result.Error = fmt.Errorf("invalid resource kind: expected %s, got %s", cfg.Kind, spec.Kind)
		return result
	}

	// Fetch existing resource by name
	existingRaw, existingID, err := cfg.FetchCurrent(spec.Metadata.Name, profile)
	if err != nil {
		result.Error = fmt.Errorf("failed to fetch current resource: %w", err)
		return result
	}

	resourceExists := existingRaw != nil && existingID != ""

	// Use cached remediation config if provided, otherwise load from disk
	remediationCfg := cachedRemCfg
	if remediationCfg == nil {
		var remediationErr error
		remediationCfg, remediationErr = remediation.LoadConfig()
		if remediationErr != nil {
			fmt.Printf("Warning: Failed to load remediation config: %v (using defaults)\n", remediationErr)
		}
	}

	if !resourceExists {
		// Resource doesn't exist
		if cfg.Mode == ApplyUpdateOnly {
			// Update-only mode: error on missing resource
			result.NotFound = true
			result.Error = fmt.Errorf("resource '%s' not found in VBR (update-only mode)", spec.Metadata.Name)
			return result
		}

		// ApplyCreateOrUpdate mode: create new resource
		// Remove ignored fields from spec
		cleanedSpec := cleanSpec(spec.Spec, cfg.IgnoreFields)

		// Apply payload transformation if defined
		if cfg.PreparePayload != nil {
			cleanedSpec, err = cfg.PreparePayload(cleanedSpec, nil)
			if err != nil {
				result.Error = fmt.Errorf("failed to prepare payload: %w", err)
				return result
			}
		}

		if dryRun {
			// Dry-run mode: show what would be created
			printDryRunCreate(spec.Metadata.Name, cfg.Kind, cleanedSpec)
			result.Action = "would-create"
			printPlannedPostApply(cfg, spec.Spec, nil)
			return result
		}

		// Create the resource
		fmt.Printf("Creating new %s: %s\n", cfg.Kind, spec.Metadata.Name)
		var newID string
		if cfg.PostCreate != nil {
			newID, err = cfg.PostCreate(cleanedSpec, profile, cfg.Endpoint)
		} else {
			newID, err = defaultPostCreate(cleanedSpec, profile, cfg.Endpoint)
		}
		if err != nil {
			result.Error = fmt.Errorf("failed to create resource: %w", err)
			return result
		}

		result.ResourceID = newID
		result.Action = "created"
		fmt.Printf("Created %s with ID: %s\n", cfg.Kind, newID)

	} else {
		// Resource exists: update it
		result.ResourceID = existingID

		// Parse existing resource into map
		var existingMap map[string]interface{}
		if err := json.Unmarshal(existingRaw, &existingMap); err != nil {
			result.Error = fmt.Errorf("failed to parse existing resource: %w", err)
			return result
		}

		// Deep merge spec into existing (spec values override existing)
		mergedSpec, err := resources.DeepMergeMaps(existingMap, spec.Spec)
		if err != nil {
			result.Error = fmt.Errorf("failed to merge specs: %w", err)
			return result
		}

		// Remove ignored fields
		mergedSpec = cleanSpec(mergedSpec, cfg.IgnoreFields)

		// Compute field changes for reporting
		allChanges := computeFieldChanges(existingMap, mergedSpec, cfg.IgnoreFields)

		// Filter changes based on remediation policy
		toApply, toSkip := filterChangesWithRemediation(remediationCfg, cfg.Kind, allChanges)
		result.Changes = toApply
		result.Skipped = toSkip

		// Restore existing values for skipped fields (don't change them)
		mergedSpec = restoreSkippedFields(mergedSpec, existingMap, toSkip)

		// VBR API requires the id field in PUT request bodies. cleanSpec strips it
		// (it's in IgnoreFields for drift/export), so restore it from the existing resource.
		if id, ok := existingMap["id"]; ok {
			mergedSpec["id"] = id
		}

		// Apply payload transformation if defined
		if cfg.PreparePayload != nil {
			mergedSpec, err = cfg.PreparePayload(mergedSpec, existingMap)
			if err != nil {
				result.Error = fmt.Errorf("failed to prepare payload: %w", err)
				return result
			}
		}

		if dryRun {
			// Dry-run mode: show what would change (including skipped)
			printDryRunUpdateWithSkipped(spec.Metadata.Name, cfg.Kind, result.Changes, result.Skipped)
			result.Action = "would-update"
			printPlannedPostApply(cfg, spec.Spec, existingMap)
			return result
		}

		// Print changes being applied and skipped
		printApplyChanges(result.Changes, spec.Metadata.Name, true)
		printSkippedFields(result.Skipped)

		// PUT the updated resource
		endpoint := fmt.Sprintf("%s/%s", cfg.Endpoint, existingID)
		_, err = vhttp.PutDataWithError(endpoint, mergedSpec, profile)
		if err != nil {
			result.Error = fmt.Errorf("failed to update resource: %w", err)
			return result
		}

		result.Action = "updated"
	}

	// Reconcile post-write state (e.g. job enabled/disabled). The resource was
	// written, so state is still recorded below; the error fails the apply.
	var postApplyErr error
	if cfg.PostApply != nil && !dryRun {
		postApplyErr = cfg.PostApply(spec.Spec, result.ResourceID, profile)
		if postApplyErr != nil {
			fmt.Printf("Warning: %v\n", postApplyErr)
		}
	}

	// Update state with origin: "applied" (skip in dry-run mode)
	if !dryRun {
		// Extract field names from changes for audit trail
		changedFields := extractFieldNames(result.Changes)
		// Use "created" action for new resources, "applied" for updates
		action := "applied"
		if result.Action == "created" {
			action = "created"
		}
		if err := updateResourceStateWithAction(spec, result.ResourceID, cfg.Kind, changedFields, action); err != nil {
			// Log warning but don't fail the apply
			fmt.Printf("Warning: Failed to update state: %v\n", err)
		}
	}

	result.Error = postApplyErr
	return result
}

// printPlannedPostApply prints the dry-run preview of the config's PostApply action.
func printPlannedPostApply(cfg ResourceApplyConfig, spec, existing map[string]interface{}) {
	if cfg.PlanPostApply == nil {
		return
	}
	if plan := cfg.PlanPostApply(spec, existing); plan != "" {
		fmt.Printf("  Would call: %s\n", plan)
	}
}

// extractFieldNames returns just the field paths from a list of changes
func extractFieldNames(changes []FieldChange) []string {
	fields := make([]string, len(changes))
	for i, c := range changes {
		fields[i] = c.Path
	}
	return fields
}

// cleanSpec removes ignored fields from a spec
func cleanSpec(spec map[string]interface{}, ignoreFields map[string]bool) map[string]interface{} {
	cleaned := make(map[string]interface{})
	for k, v := range spec {
		if ignoreFields[k] {
			continue
		}
		cleaned[k] = v
	}
	return cleaned
}

// filterChangesWithRemediation applies remediation policy to filter changes
func filterChangesWithRemediation(cfg *remediation.Config, resourceKind string, changes []FieldChange) ([]FieldChange, []SkippedField) {
	if cfg == nil {
		// No config loaded - apply all changes
		return changes, nil
	}

	// Convert to remediation.FieldChange for filtering
	remChanges := make([]remediation.FieldChange, len(changes))
	for i, c := range changes {
		remChanges[i] = remediation.FieldChange{
			Path:     c.Path,
			OldValue: c.OldValue,
			NewValue: c.NewValue,
		}
	}

	toApply, toSkip := cfg.FilterChanges(resourceKind, remChanges)

	// Convert back to cmd types
	appliedChanges := make([]FieldChange, len(toApply))
	for i, c := range toApply {
		appliedChanges[i] = FieldChange{
			Path:     c.Path,
			OldValue: c.OldValue,
			NewValue: c.NewValue,
		}
	}

	skippedFields := make([]SkippedField, len(toSkip))
	for i, s := range toSkip {
		skippedFields[i] = SkippedField{
			Path:   s.Path,
			Reason: s.Reason,
		}
	}

	return appliedChanges, skippedFields
}

// restoreSkippedFields restores existing values for fields that should not be changed.
// This ensures skipped fields retain their current VBR values rather than being overwritten.
func restoreSkippedFields(merged, existing map[string]interface{}, skipped []SkippedField) map[string]interface{} {
	if len(skipped) == 0 {
		return merged
	}

	// Build a map of skipped paths to their existing values
	for _, s := range skipped {
		restoreFieldValue(merged, existing, s.Path)
	}

	return merged
}

// restoreFieldValue restores a single field's value from existing to merged.
// Handles dotted paths like "storage.retentionPolicy.type".
func restoreFieldValue(merged, existing map[string]interface{}, path string) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return
	}

	// Navigate to the parent in both maps
	mergedParent := merged
	existingParent := existing
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if m, ok := mergedParent[part].(map[string]interface{}); ok {
			mergedParent = m
		} else {
			fmt.Printf("Warning: Skipped field path '%s' not found in merged spec\n", path)
			return
		}
		if e, ok := existingParent[part].(map[string]interface{}); ok {
			existingParent = e
		} else {
			fmt.Printf("Warning: Skipped field path '%s' not found in existing resource\n", path)
			return
		}
	}

	// Restore the final field
	lastPart := parts[len(parts)-1]
	if existingVal, ok := existingParent[lastPart]; ok {
		mergedParent[lastPart] = existingVal
	} else {
		fmt.Printf("Warning: Skipped field '%s' not found in existing resource\n", path)
	}
}

// defaultPostCreate creates a new resource via POST and extracts the ID from the response
func defaultPostCreate(spec map[string]interface{}, profile models.Profile, endpoint string) (string, error) {
	// Use PostDataWithError to get proper error handling instead of log.Fatal
	responseBytes, err := vhttp.PostDataWithError(endpoint, spec, profile)
	if err != nil {
		return "", err
	}

	// Extract ID from response
	var response struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if response.ID == "" {
		return "", fmt.Errorf("no ID in response")
	}
	return response.ID, nil
}

// updateResourceState saves the applied configuration to state (wrapper for backward compatibility)
func updateResourceState(spec resources.ResourceSpec, resourceID, resourceType string, changedFields []string) error {
	return updateResourceStateWithAction(spec, resourceID, resourceType, changedFields, "applied")
}

// updateResourceStateWithAction saves the applied configuration to state with a specific action type
func updateResourceStateWithAction(spec resources.ResourceSpec, resourceID, resourceType string, changedFields []string, action string) error {
	stateMgr := state.NewManager()

	// Get current user
	currentUser := "unknown"
	if usr, err := user.Current(); err == nil {
		currentUser = usr.Username
	}

	// Try to load existing resource to preserve history
	var existingHistory []state.ResourceEvent
	if existing, err := stateMgr.GetResource(spec.Metadata.Name); err == nil {
		existingHistory = existing.History
	}

	// Create state resource
	resource := &state.Resource{
		Type:          resourceType,
		ID:            resourceID,
		Name:          spec.Metadata.Name,
		LastApplied:   time.Now(),
		LastAppliedBy: currentUser,
		Origin:        "applied",
		Spec:          spec.Spec,
		History:       existingHistory,
	}

	// Record event with changed fields
	resource.AddEvent(state.NewEventWithFields(action, currentUser, changedFields, false))

	// Update state
	if err := stateMgr.UpdateResource(resource); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	fmt.Printf("State updated: %s\n", stateMgr.GetStatePath())
	return nil
}
