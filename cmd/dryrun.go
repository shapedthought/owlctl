package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
)

// Dry-run actions reported by every apply command.
const (
	dryRunWouldCreate = "would-create"
	dryRunWouldUpdate = "would-update"
	dryRunNoChange    = "no-change"
)

// dryRunNoChangesMessage is the single "nothing to do" line printed by every
// apply --dry-run. CI scripts grep for the "No changes detected" prefix.
const dryRunNoChangesMessage = "No changes detected. Resource is already in desired state."

// DryRunChange is one field that apply would change.
type DryRunChange struct {
	Path     string      `json:"path"`
	Old      interface{} `json:"old"`
	New      interface{} `json:"new"`
	Severity Severity    `json:"severity"`
}

// DryRunReport is the result of an apply --dry-run, shared by job apply and the
// generic resource apply commands. It is rendered as text by printDryRun or as
// JSON by writeDryRunJSON.
type DryRunReport struct {
	Resource  string         `json:"resource"`
	Kind      string         `json:"kind"`
	Action    string         `json:"action"` // would-create | would-update | no-change
	Changes   []DryRunChange `json:"changes"`
	Skipped   []SkippedField `json:"skipped"`
	PostApply string         `json:"postApply,omitempty"`

	// Spec is the configuration that would be created (would-create only). It is
	// summarised in text output and omitted from JSON.
	Spec map[string]interface{} `json:"-"`
}

// newDryRunChanges converts FieldChanges to DryRunChanges, assigning severity
// from sm (INFO when sm is nil or has no entry).
func newDryRunChanges(changes []FieldChange, sm SeverityMap) []DryRunChange {
	out := make([]DryRunChange, 0, len(changes))
	for _, c := range changes {
		out = append(out, DryRunChange{Path: c.Path, Old: c.OldValue, New: c.NewValue, Severity: sm.GetSeverity(c.Path)})
	}
	return out
}

// newDryRunChangesFromDrifts converts classified job drifts (state = desired,
// VBR = current) to DryRunChanges, sorted by path.
func newDryRunChangesFromDrifts(drifts []Drift) []DryRunChange {
	out := make([]DryRunChange, 0, len(drifts))
	for _, d := range drifts {
		c := DryRunChange{Path: d.Path, Severity: d.Severity}
		switch d.Action {
		case "modified":
			c.Old, c.New = d.VBR, d.State
		case "removed": // in the spec but not in VBR: will be added
			c.New = d.State
		case "added": // in VBR but not in the spec: will be unset
			c.Old = d.VBR
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// newDryRunUpdateReport builds the report for an update of an existing resource.
func newDryRunUpdateReport(name, kind string, changes []DryRunChange, skipped []SkippedField, postApply string) *DryRunReport {
	action := dryRunWouldUpdate
	if len(changes) == 0 && len(skipped) == 0 && postApply == "" {
		action = dryRunNoChange
	}
	if changes == nil {
		changes = []DryRunChange{}
	}
	if skipped == nil {
		skipped = []SkippedField{}
	}
	return &DryRunReport{Resource: name, Kind: kind, Action: action, Changes: changes, Skipped: skipped, PostApply: postApply}
}

// newDryRunCreateReport builds the report for creating a new resource.
func newDryRunCreateReport(name, kind string, spec map[string]interface{}, postApply string) *DryRunReport {
	return &DryRunReport{
		Resource: name, Kind: kind, Action: dryRunWouldCreate,
		Changes: []DryRunChange{}, Skipped: []SkippedField{},
		PostApply: postApply, Spec: spec,
	}
}

// formatDryRunChange renders one change line: "  <SEVERITY> <~|+|-> <path>: <old> -> <new>".
// "+" means the field has no current value, "-" means it will be unset.
func formatDryRunChange(c DryRunChange) string {
	op := "~"
	switch {
	case c.Old == nil && c.New != nil:
		op = "+"
	case c.New == nil && c.Old != nil:
		op = "-"
	}
	sev := c.Severity
	if sev == "" {
		sev = SeverityInfo
	}
	return fmt.Sprintf("  %s %s %s: %s -> %s", sev, op, c.Path, applyFormatValue(c.Old), applyFormatValue(c.New))
}

// printDryRun renders a DryRunReport as text.
func printDryRun(w io.Writer, r *DryRunReport) {
	fmt.Fprintln(w, "\n=== Dry Run Mode ===")
	fmt.Fprintf(w, "Resource: %s (%s)\n", r.Resource, r.Kind)
	if r.Action == dryRunWouldCreate {
		fmt.Fprintln(w, "Action: Would CREATE new resource")
	} else {
		fmt.Fprintln(w, "Action: Would UPDATE existing resource")
	}
	fmt.Fprintln(w)

	switch {
	case r.Action == dryRunWouldCreate:
		fmt.Fprintln(w, "Configuration to be created:")
		printSpecSummaryTo(w, r.Spec, "  ")
	case len(r.Changes) == 0 && len(r.Skipped) == 0:
		fmt.Fprintln(w, dryRunNoChangesMessage)
	default:
		if len(r.Changes) > 0 {
			fmt.Fprintln(w, "Changes that would be applied:")
			for _, c := range r.Changes {
				fmt.Fprintln(w, formatDryRunChange(c))
			}
			fmt.Fprintf(w, "\n%d field(s) would be changed.\n", len(r.Changes))
		}
		if len(r.Skipped) > 0 {
			fmt.Fprintln(w, "\nSkipped (not sent to VBR):")
			for _, s := range r.Skipped {
				fmt.Fprintf(w, "  - %s\n", s.Path)
				if s.Reason != "" {
					fmt.Fprintf(w, "    %s\n", s.Reason)
				}
			}
		}
	}

	if r.PostApply != "" {
		fmt.Fprintf(w, "\nWould call: %s\n", r.PostApply)
	}

	fmt.Fprintln(w, "\n=== End Dry Run ===")
	fmt.Fprintln(w, "No changes made. Remove --dry-run flag to apply.")
}

// writeDryRunJSON renders a DryRunReport as indented JSON.
func writeDryRunJSON(w io.Writer, r *DryRunReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// --- --output handling ---

var (
	// dryRunJSON is true while a command runs with --output json.
	dryRunJSON bool
	// dryRunJSONOut is the real stdout the JSON document is written to.
	dryRunJSONOut io.Writer = os.Stdout
	realStdout    *os.File
)

// beginDryRunOutput validates --output and, for json, redirects everything else
// printed to stdout (banners, "Applying overlay:", warnings) to stderr so stdout
// carries only the JSON document. The returned func restores stdout.
func beginDryRunOutput(format string, dryRun bool) func() {
	switch strings.ToLower(format) {
	case "", "text":
		return func() {}
	case "json":
		if !dryRun {
			log.Fatal("--output json is only supported together with --dry-run")
		}
		dryRunJSON = true
		realStdout = os.Stdout
		dryRunJSONOut = realStdout
		os.Stdout = os.Stderr
		return func() {
			os.Stdout = realStdout
			dryRunJSON = false
		}
	default:
		log.Fatalf("Invalid --output %q (use text or json)", format)
		return func() {}
	}
}

const dryRunOutputUsage = "Dry-run output format: text (default) or json (requires --dry-run)"

// applyWithDryRunOutput runs fn with --output handled: for json, stdout carries
// only the dry-run JSON document.
func applyWithDryRunOutput(format string, dryRun bool, fn func() ApplyResult) ApplyResult {
	restore := beginDryRunOutput(format, dryRun)
	defer restore()
	return fn()
}

// rejectGroupJSONOutput exits if --output json is combined with --group, which
// applies several resources and so has no single JSON document.
func rejectGroupJSONOutput(format string) {
	if strings.EqualFold(format, "json") {
		log.Fatal("Cannot use --group with --output json")
	}
}

// emitDryRun writes the report as JSON when --output json is active, else as text.
func emitDryRun(r *DryRunReport) {
	if dryRunJSON {
		if err := writeDryRunJSON(dryRunJSONOut, r); err != nil {
			log.Fatalf("Failed to write JSON: %v", err)
		}
		return
	}
	printDryRun(os.Stdout, r)
}
