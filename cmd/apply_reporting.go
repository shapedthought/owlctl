package cmd

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
)

// FieldChange represents a single field that changed during apply
type FieldChange struct {
	Path     string      // Dotted path like "storage.retentionPolicy.quantity"
	OldValue interface{} // Value before apply (from VBR)
	NewValue interface{} // Value after apply (from spec)
}

// computeFieldChanges compares two maps and returns the fields that differ.
// It performs a deep comparison and returns dotted paths for nested fields.
func computeFieldChanges(existing, desired map[string]interface{}, ignoreFields map[string]bool) []FieldChange {
	var changes []FieldChange
	computeChangesRecursive("", existing, desired, ignoreFields, &changes)

	// Sort by path for consistent output
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Path < changes[j].Path
	})

	return changes
}

// computeChangesRecursive recursively compares maps and collects differences.
// Note: This only reports fields present in 'desired' that differ from 'existing'.
// Fields in 'existing' but not in 'desired' are intentionally not reported as changes
// because the apply operation uses deep merge, which preserves such fields from VBR.
func computeChangesRecursive(prefix string, existing, desired map[string]interface{}, ignoreFields map[string]bool, changes *[]FieldChange) {
	// Check all keys in desired (fields being set by the spec)
	for key, newVal := range desired {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		// Skip ignored fields (check both short name and full path)
		if ignoreFields[key] || ignoreFields[path] {
			continue
		}

		oldVal, exists := existing[key]

		if !exists {
			// New field being added
			*changes = append(*changes, FieldChange{
				Path:     path,
				OldValue: nil,
				NewValue: newVal,
			})
			continue
		}

		// Both exist - compare values
		if !applyValuesEqual(oldVal, newVal) {
			// Check if both are maps - recurse
			oldMap, oldIsMap := oldVal.(map[string]interface{})
			newMap, newIsMap := newVal.(map[string]interface{})

			if oldIsMap && newIsMap {
				computeChangesRecursive(path, oldMap, newMap, ignoreFields, changes)
			} else {
				// Scalar or type change
				*changes = append(*changes, FieldChange{
					Path:     path,
					OldValue: oldVal,
					NewValue: newVal,
				})
			}
		}
	}
}

// applyValuesEqual compares two values for equality.
// Handles numeric type mismatches (JSON float64 vs YAML int) by comparing
// as float64 when both values are numeric.
func applyValuesEqual(a, b interface{}) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Numeric comparison: JSON unmarshals numbers as float64, YAML as int.
	aNum, aIsNum := tryFloat64(a)
	bNum, bIsNum := tryFloat64(b)
	if aIsNum && bIsNum {
		return aNum == bNum
	}

	// Use reflect.DeepEqual for complex types
	return reflect.DeepEqual(a, b)
}

// tryFloat64 attempts to convert a value to float64 for numeric comparison.
func tryFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	default:
		return 0, false
	}
}

// applyFormatValue formats a value for display
func applyFormatValue(v interface{}) string {
	if v == nil {
		return "<none>"
	}

	switch val := v.(type) {
	case string:
		// Truncate long strings
		if len(val) > 50 {
			return fmt.Sprintf("\"%s...\"", val[:47])
		}
		return fmt.Sprintf("\"%s\"", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case float64:
		// Check if it's a whole number
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case []interface{}:
		return fmt.Sprintf("[%d items]", len(val))
	case map[string]interface{}:
		return fmt.Sprintf("{%d fields}", len(val))
	default:
		return fmt.Sprintf("%v", val)
	}
}

// printApplyChanges prints the field changes that were applied
func printApplyChanges(changes []FieldChange, resourceName string, success bool) {
	if len(changes) == 0 {
		fmt.Println("  No field changes detected.")
		return
	}

	fmt.Printf("\nApplying: %s\n\n", resourceName)

	for _, change := range changes {
		oldStr := applyFormatValue(change.OldValue)
		newStr := applyFormatValue(change.NewValue)

		if success {
			fmt.Printf("  Applied: %s: %s -> %s\n", change.Path, oldStr, newStr)
		} else {
			fmt.Printf("  Attempted: %s: %s -> %s\n", change.Path, oldStr, newStr)
		}
	}

	fmt.Println()
	if success {
		fmt.Printf("%d field(s) applied.\n", len(changes))
	} else {
		fmt.Printf("%d field(s) attempted.\n", len(changes))
	}
}

// printDryRunUpdate prints what would change in dry-run mode for an update
func printDryRunUpdate(resourceName, resourceKind string, changes []FieldChange) {
	fmt.Println("\n=== Dry Run Mode ===")
	fmt.Printf("Resource: %s (%s)\n", resourceName, resourceKind)
	fmt.Println("Action: Would UPDATE existing resource")
	fmt.Println()

	if len(changes) == 0 {
		fmt.Println("No changes detected. Resource is already in desired state.")
	} else {
		fmt.Println("Changes that would be applied:")
		for _, change := range changes {
			oldStr := applyFormatValue(change.OldValue)
			newStr := applyFormatValue(change.NewValue)
			fmt.Printf("  ~ %s: %s -> %s\n", change.Path, oldStr, newStr)
		}
		fmt.Printf("\n%d field(s) would be changed.\n", len(changes))
	}

	fmt.Println("\n=== End Dry Run ===")
	fmt.Println("No changes made. Remove --dry-run flag to apply.")
}

// printDryRunCreate prints what would be created in dry-run mode
func printDryRunCreate(resourceName, resourceKind string, spec map[string]interface{}) {
	fmt.Println("\n=== Dry Run Mode ===")
	fmt.Printf("Resource: %s (%s)\n", resourceName, resourceKind)
	fmt.Println("Action: Would CREATE new resource")
	fmt.Println()

	// Show key fields from the spec
	fmt.Println("Configuration to be created:")
	printSpecSummary(spec, "  ")

	fmt.Println("\n=== End Dry Run ===")
	fmt.Println("No changes made. Remove --dry-run flag to apply.")
}

// printSpecSummary prints a summary of key fields in a spec
func printSpecSummary(spec map[string]interface{}, indent string) {
	// List of key fields to display (in order)
	keyFields := []string{"name", "description", "type", "isEnabled", "isDisabled"}

	for _, field := range keyFields {
		if val, ok := spec[field]; ok {
			fmt.Printf("%s%s: %s\n", indent, field, applyFormatValue(val))
		}
	}

	// Show nested object summaries
	for key, val := range spec {
		// Skip already printed fields
		isKeyField := false
		for _, kf := range keyFields {
			if key == kf {
				isKeyField = true
				break
			}
		}
		if isKeyField {
			continue
		}

		switch v := val.(type) {
		case map[string]interface{}:
			fmt.Printf("%s%s: {%d fields}\n", indent, key, len(v))
		case []interface{}:
			fmt.Printf("%s%s: [%d items]\n", indent, key, len(v))
		}
	}
}

// printNotFoundGuidance prints guidance when a resource is not found
func printNotFoundGuidance(resourceType, resourceName, applyCmd string) {
	fmt.Fprintf(
		os.Stderr,
		"\nError: %s \"%s\" no longer exists in VBR.\n"+
			"Cannot apply — recreate it manually in the VBR console, then re-apply:\n"+
			"  %s\n",
		resourceType, resourceName, applyCmd,
	)
}

// RemediationGuidance contains the guidance to show after diff based on origin
type RemediationGuidance struct {
	Origin       string // "applied" or "observed"
	ResourceType string // e.g., "repository", "job"
	ResourceName string
	ApplyCmd     string // Command to remediate (for applied resources)
	ExportCmd    string // Command to export (for observed resources)
	ManageCmd    string // Command to bring an observed resource under management ("" for read-only resources)
}

// printRemediationGuidance prints appropriate guidance based on resource origin
func printRemediationGuidance(g RemediationGuidance) {
	switch g.Origin {
	case "applied":
		// Check if this is a read-only resource type
		if strings.HasPrefix(g.ApplyCmd, "# ") || g.ApplyCmd == "" {
			// Read-only resource (e.g., encryption passwords)
			fmt.Printf("\nThis %s cannot be modified via API. Manual remediation required in VBR console.\n", g.ResourceType)
		} else {
			fmt.Printf("\nTo remediate, run:\n")
			fmt.Printf("  %s\n", g.ApplyCmd)
		}
	case "observed":
		if g.ManageCmd == "" {
			// Read-only resource (e.g. encryption passwords) — cannot be applied.
			fmt.Printf("\nThis resource is read-only and monitored only. Remediate manually in the VBR console.\n")
		} else {
			fmt.Printf("\nThis resource is monitored only (origin: observed). To bring it under declarative management:\n")
			fmt.Printf("  1. Export: %s\n", g.ExportCmd)
			fmt.Printf("  2. Apply:  %s\n", g.ManageCmd)
		}
	default:
		// Legacy resource without origin - show snapshot command
		fmt.Printf("\nThe %s has drifted from the snapshot configuration.\n", g.ResourceType)
		fmt.Printf("\nTo update the snapshot, run:\n")
		fmt.Printf("  %s\n", getSnapshotCommand(g.ResourceType, g.ResourceName))
	}
}

// getSnapshotCommand returns the full snapshot command for a resource type
func getSnapshotCommand(resourceType, resourceName string) string {
	switch resourceType {
	case "VBRRepository", "repository":
		return fmt.Sprintf("owlctl repo snapshot \"%s\"", resourceName)
	case "VBRScaleOutRepository", "scale-out repository":
		return fmt.Sprintf("owlctl repo sobr-snapshot \"%s\"", resourceName)
	case "VBRJob", "job":
		return fmt.Sprintf("owlctl job snapshot \"%s\"", resourceName)
	case "VBREncryptionPassword", "encryption password":
		return fmt.Sprintf("owlctl encryption snapshot \"%s\"", resourceName)
	case "VBRKmsServer", "KMS server":
		return fmt.Sprintf("owlctl encryption kms-snapshot \"%s\"", resourceName)
	default:
		return fmt.Sprintf("owlctl %s snapshot \"%s\"", strings.ToLower(resourceType), resourceName)
	}
}

// BuildRepoGuidance creates guidance for repository diff
func BuildRepoGuidance(resourceName, origin string) RemediationGuidance {
	return RemediationGuidance{
		Origin:       origin,
		ResourceType: "repository",
		ResourceName: resourceName,
		ApplyCmd:     fmt.Sprintf("owlctl repo apply repos/%s.yaml", sanitizeFileName(resourceName)),
		ExportCmd:    fmt.Sprintf("owlctl repo export \"%s\" -o repos/%s.yaml", resourceName, sanitizeFileName(resourceName)),
		ManageCmd:    fmt.Sprintf("owlctl repo apply repos/%s.yaml", sanitizeFileName(resourceName)),
	}
}

// BuildSobrGuidance creates guidance for SOBR diff
func BuildSobrGuidance(resourceName, origin string) RemediationGuidance {
	return RemediationGuidance{
		Origin:       origin,
		ResourceType: "scale-out repository",
		ResourceName: resourceName,
		ApplyCmd:     fmt.Sprintf("owlctl repo sobr-apply sobrs/%s.yaml", sanitizeFileName(resourceName)),
		ExportCmd:    fmt.Sprintf("owlctl repo sobr-export \"%s\" -o sobrs/%s.yaml", resourceName, sanitizeFileName(resourceName)),
		ManageCmd:    fmt.Sprintf("owlctl repo sobr-apply sobrs/%s.yaml", sanitizeFileName(resourceName)),
	}
}

// BuildJobGuidance creates guidance for job diff
func BuildJobGuidance(resourceName, origin string) RemediationGuidance {
	return RemediationGuidance{
		Origin:       origin,
		ResourceType: "job",
		ResourceName: resourceName,
		ApplyCmd:     fmt.Sprintf("owlctl job apply jobs/%s.yaml", sanitizeFileName(resourceName)),
		ExportCmd:    fmt.Sprintf("owlctl export \"%s\" -o jobs/%s.yaml", resourceName, sanitizeFileName(resourceName)),
		ManageCmd:    fmt.Sprintf("owlctl job apply jobs/%s.yaml", sanitizeFileName(resourceName)),
	}
}

// BuildEncryptionGuidance creates guidance for encryption password diff
func BuildEncryptionGuidance(resourceName, origin string) RemediationGuidance {
	return RemediationGuidance{
		Origin:       origin,
		ResourceType: "encryption password",
		ResourceName: resourceName,
		// Encryption passwords are read-only in VBR API - empty ApplyCmd/ManageCmd triggers read-only message
		ApplyCmd:  "",
		ExportCmd: fmt.Sprintf("owlctl encryption export \"%s\" -o encryption/%s.yaml", resourceName, sanitizeFileName(resourceName)),
		ManageCmd: "",
	}
}

// BuildKmsGuidance creates guidance for KMS server diff
func BuildKmsGuidance(resourceName, origin string) RemediationGuidance {
	return RemediationGuidance{
		Origin:       origin,
		ResourceType: "KMS server",
		ResourceName: resourceName,
		ApplyCmd:     fmt.Sprintf("owlctl encryption kms-apply kms/%s.yaml", sanitizeFileName(resourceName)),
		ExportCmd:    fmt.Sprintf("owlctl encryption kms-export \"%s\" -o kms/%s.yaml", resourceName, sanitizeFileName(resourceName)),
		ManageCmd:    fmt.Sprintf("owlctl encryption kms-apply kms/%s.yaml", sanitizeFileName(resourceName)),
	}
}

// SkippedField represents a field that was skipped during apply
type SkippedField struct {
	Path   string
	Reason string
}

// printSkippedFields prints fields that were skipped due to policy or known immutability
func printSkippedFields(skipped []SkippedField) {
	if len(skipped) == 0 {
		return
	}

	fmt.Println("\nSkipped fields (known immutable or policy-configured):")
	for _, s := range skipped {
		fmt.Printf("  - %s\n", s.Path)
		if s.Reason != "" {
			fmt.Printf("    %s\n", s.Reason)
		}
	}
}

// printDryRunUpdateWithSkipped prints dry-run output including skipped fields
func printDryRunUpdateWithSkipped(resourceName, resourceKind string, changes []FieldChange, skipped []SkippedField) {
	fmt.Println("\n=== Dry Run Mode ===")
	fmt.Printf("Resource: %s (%s)\n", resourceName, resourceKind)
	fmt.Println("Action: Would UPDATE existing resource")
	fmt.Println()

	if len(changes) == 0 && len(skipped) == 0 {
		fmt.Println("No changes detected. Resource is already in desired state.")
	} else {
		if len(changes) > 0 {
			fmt.Println("Changes that would be applied:")
			for _, change := range changes {
				oldStr := applyFormatValue(change.OldValue)
				newStr := applyFormatValue(change.NewValue)
				fmt.Printf("  ~ %s: %s -> %s\n", change.Path, oldStr, newStr)
			}
			fmt.Printf("\n%d field(s) would be changed.\n", len(changes))
		}

		if len(skipped) > 0 {
			fmt.Println("\nSkipped (not sent to VBR):")
			for _, s := range skipped {
				fmt.Printf("  - %s\n", s.Path)
				if s.Reason != "" {
					fmt.Printf("    %s\n", s.Reason)
				}
			}
		}
	}

	fmt.Println("\n=== End Dry Run ===")
	fmt.Println("No changes made. Remove --dry-run flag to apply.")
}

// sanitizeFileName converts a resource name to a safe filename.
// Handles spaces, special characters, and avoids common problematic patterns.
func sanitizeFileName(name string) string {
	// Normalize case and trim whitespace
	result := strings.ToLower(strings.TrimSpace(name))

	// Replace problematic characters with hyphens
	replacer := strings.NewReplacer(
		" ", "-",
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	result = replacer.Replace(result)

	// Collapse multiple consecutive hyphens into one
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}

	// Trim leading/trailing hyphens
	result = strings.Trim(result, "-")

	// Ensure non-empty result
	if result == "" {
		result = "resource"
	}

	return result
}
