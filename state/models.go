package state

import "time"

// CurrentStateVersion is the latest state file format version.
// Increment when changing the Resource schema and add a migration in manager.go.
const CurrentStateVersion = 4

// DefaultMaxHistoryEvents is the maximum number of history events to keep per resource
const DefaultMaxHistoryEvents = 20

// InstanceState holds all managed resources for a single named instance.
type InstanceState struct {
	Product   string               `json:"product,omitempty"` // e.g. "vbr", "azure" — used for export folder structure
	Resources map[string]*Resource `json:"resources"`
}

// State represents the owlctl state file structure
//
// WARNING: State files are mutable and NOT suitable for compliance or audit.
// For compliance, use Git commit history + CI/CD logs + VBR audit logs.
// State is purely operational - used for drift detection and tracking what was last applied.
type State struct {
	Version   int                       `json:"version"`
	Instances map[string]*InstanceState `json:"instances"`
	// Resources is retained for v3→v4 migration only; nil after migration.
	Resources map[string]*Resource `json:"resources,omitempty"`
}

// ResourceEvent represents an action taken on a resource
type ResourceEvent struct {
	Action    string    `json:"action"`           // "snapshotted", "applied", "created"
	Timestamp time.Time `json:"timestamp"`        // When the action occurred
	User      string    `json:"user"`             // Who performed the action
	Fields    []string  `json:"fields,omitempty"` // Fields that were changed (for apply/created)
	Partial   bool      `json:"partial,omitempty"` // Reserved for future partial-apply support; currently always false
}

// Resource represents a managed resource in state
type Resource struct {
	Type          string                 `json:"type"`                  // e.g., "VBRJob"
	ID            string                 `json:"id"`                    // VBR resource ID
	Name          string                 `json:"name"`                  // Resource name
	LastApplied   time.Time              `json:"lastApplied"`           // When it was last applied
	LastAppliedBy string                 `json:"lastAppliedBy"`         // User who applied it
	Origin        string                 `json:"origin"`                // "applied" (declarative) or "observed" (snapshot)
	Spec          map[string]interface{} `json:"spec"`                  // The applied configuration
	History       []ResourceEvent        `json:"history,omitempty"`     // Audit trail of actions
}

// NewState creates a new empty state
func NewState() *State {
	return &State{
		Version:   CurrentStateVersion,
		Instances: make(map[string]*InstanceState),
	}
}

// getInstance returns the InstanceState for the given key, creating it if needed.
// Handles nil entries that may result from JSON unmarshaling of "instances": {"<key>": null}.
func (s *State) getInstance(instance string) *InstanceState {
	if s.Instances == nil {
		s.Instances = make(map[string]*InstanceState)
	}
	inst, ok := s.Instances[instance]
	if !ok || inst == nil {
		inst = &InstanceState{Resources: make(map[string]*Resource)}
		s.Instances[instance] = inst
	}
	if inst.Resources == nil {
		inst.Resources = make(map[string]*Resource)
	}
	return inst
}

// GetResource retrieves a resource by instance and name
func (s *State) GetResource(instance, name string) (*Resource, bool) {
	inst, ok := s.Instances[instance]
	if !ok || inst == nil || inst.Resources == nil {
		return nil, false
	}
	resource, ok := inst.Resources[name]
	return resource, ok
}

// SetResource adds or updates a resource within the given instance
func (s *State) SetResource(instance string, resource *Resource) {
	s.getInstance(instance).Resources[resource.Name] = resource
}

// DeleteResource removes a resource from the given instance
func (s *State) DeleteResource(instance, name string) {
	inst, ok := s.Instances[instance]
	if !ok || inst == nil || inst.Resources == nil {
		return
	}
	delete(inst.Resources, name)
}

// ListResources returns all resources of a given type within the given instance.
// Pass an empty resourceType to return all resources.
func (s *State) ListResources(instance, resourceType string) []*Resource {
	inst, ok := s.Instances[instance]
	if !ok || inst == nil || inst.Resources == nil {
		return nil
	}
	var resources []*Resource
	for _, resource := range inst.Resources {
		if resourceType == "" || resource.Type == resourceType {
			resources = append(resources, resource)
		}
	}
	return resources
}

// AddEvent records a new event to the resource's history and prunes old events
func (r *Resource) AddEvent(event ResourceEvent) {
	// Prepend new event (most recent first)
	r.History = append([]ResourceEvent{event}, r.History...)

	// Prune to max length
	if len(r.History) > DefaultMaxHistoryEvents {
		r.History = r.History[:DefaultMaxHistoryEvents]
	}
}

// NewEvent creates a new ResourceEvent with the current timestamp and user
func NewEvent(action string, user string) ResourceEvent {
	return ResourceEvent{
		Action:    action,
		Timestamp: time.Now(),
		User:      user,
	}
}

// NewEventWithFields creates a new ResourceEvent with changed fields
func NewEventWithFields(action string, user string, fields []string, partial bool) ResourceEvent {
	return ResourceEvent{
		Action:    action,
		Timestamp: time.Now(),
		User:      user,
		Fields:    fields,
		Partial:   partial,
	}
}
