package capability

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bogachenko/agentkit/core/session"
	"github.com/bogachenko/agentkit/core/skill"
	"github.com/bogachenko/agentkit/core/tool"
)

// SnapshotRevision identifies the exact registry/profile state used to resolve one invocation.
type SnapshotRevision string

// Validate prevents anonymous snapshots from reaching prompt assembly or runtime enforcement.
func (r SnapshotRevision) Validate() error {
	if strings.TrimSpace(string(r)) == "" {
		return fmt.Errorf("capability snapshot revision is required")
	}

	return nil
}

// ToolMode describes effective access to a registered tool for one invocation.
type ToolMode string

const (
	ToolModeDisabled     ToolMode = "disabled"
	ToolModeEnabled      ToolMode = "enabled"
	ToolModeAskBeforeUse ToolMode = "ask_before_use"
)

// Validate rejects unknown policy values before they influence model context or execution.
func (m ToolMode) Validate() error {
	switch m {
	case ToolModeDisabled, ToolModeEnabled, ToolModeAskBeforeUse:
		return nil
	default:
		return fmt.Errorf("unsupported tool mode %q", string(m))
	}
}

// ConditionalPromptSection declares capability-owned context and its explicit availability predicates.
type ConditionalPromptSection struct {
	ID                string
	Order             int
	Markdown          string
	RequiresAllTools  []tool.Name
	RequiresAnyTools  []tool.Name
	RequiresAllSkills []skill.ID
	RequiresAnySkills []skill.ID
}

// Validate keeps dynamic prompt dependencies deterministic and free from duplicate references.
func (s ConditionalPromptSection) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("conditional prompt section id is required")
	}
	if s.ID != strings.TrimSpace(s.ID) {
		return fmt.Errorf("conditional prompt section id must not contain surrounding whitespace")
	}
	if s.Order < 0 {
		return fmt.Errorf("conditional prompt section %q order must not be negative", s.ID)
	}
	if strings.TrimSpace(s.Markdown) == "" {
		return fmt.Errorf("conditional prompt section %q markdown is required", s.ID)
	}
	if len(s.RequiresAllTools) == 0 && len(s.RequiresAnyTools) == 0 && len(s.RequiresAllSkills) == 0 && len(s.RequiresAnySkills) == 0 {
		return fmt.Errorf("conditional prompt section %q requires at least one tool or skill predicate", s.ID)
	}

	if err := validateUniqueToolNames(s.ID, "requires_all_tools", s.RequiresAllTools); err != nil {
		return err
	}
	if err := validateUniqueToolNames(s.ID, "requires_any_tools", s.RequiresAnyTools); err != nil {
		return err
	}
	if err := validateUniqueSkillIDs(s.ID, "requires_all_skills", s.RequiresAllSkills); err != nil {
		return err
	}
	if err := validateUniqueSkillIDs(s.ID, "requires_any_skills", s.RequiresAnySkills); err != nil {
		return err
	}

	return nil
}

// PromptSection is a resolved dynamic section whose predicates were satisfied by the snapshot resolver.
type PromptSection struct {
	ID       string
	Order    int
	Markdown string
}

// Validate prevents unresolved or empty sections from entering the model instruction.
func (s PromptSection) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("prompt section id is required")
	}
	if s.ID != strings.TrimSpace(s.ID) {
		return fmt.Errorf("prompt section id must not contain surrounding whitespace")
	}
	if s.Order < 0 {
		return fmt.Errorf("prompt section %q order must not be negative", s.ID)
	}
	if strings.TrimSpace(s.Markdown) == "" {
		return fmt.Errorf("prompt section %q markdown is required", s.ID)
	}

	return nil
}

// EffectiveTool is a registered tool that may be exposed to the model and runtime for one invocation.
type EffectiveTool struct {
	CapabilityID ID
	Contract     tool.Contract
	Mode         ToolMode
}

// Validate guarantees that disabled tools are represented by absence, never by a contradictory snapshot entry.
func (t EffectiveTool) Validate() error {
	if err := t.CapabilityID.Validate(); err != nil {
		return err
	}
	if err := t.Contract.Validate(); err != nil {
		return err
	}
	if err := t.Mode.Validate(); err != nil {
		return err
	}
	if t.Mode == ToolModeDisabled {
		return fmt.Errorf("effective tool %q cannot use disabled mode", string(t.Contract.Name))
	}

	return nil
}

// EffectiveSkill is a validated, loaded skill that may contribute context for one invocation.
type EffectiveSkill struct {
	Skill skill.Skill
}

// Validate prevents metadata-only or partially loaded skills from entering the snapshot.
func (s EffectiveSkill) Validate() error {
	return s.Skill.Validate()
}

// StaleReferenceKind identifies a session profile reference that no longer resolves in a live registry.
type StaleReferenceKind string

const (
	StaleReferenceTool  StaleReferenceKind = "tool"
	StaleReferenceSkill StaleReferenceKind = "skill"
)

// Validate rejects unknown stale-reference categories.
func (k StaleReferenceKind) Validate() error {
	switch k {
	case StaleReferenceTool, StaleReferenceSkill:
		return nil
	default:
		return fmt.Errorf("unsupported stale reference kind %q", string(k))
	}
}

// StaleReference records unresolved configuration without silently mutating user profile state.
type StaleReference struct {
	Kind   StaleReferenceKind
	ID     string
	Reason string
}

// Validate keeps diagnostics attributable and actionable.
func (r StaleReference) Validate() error {
	if err := r.Kind.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("stale %s reference id is required", string(r.Kind))
	}
	if r.ID != strings.TrimSpace(r.ID) {
		return fmt.Errorf("stale %s reference id must not contain surrounding whitespace", string(r.Kind))
	}
	if strings.TrimSpace(r.Reason) == "" {
		return fmt.Errorf("stale %s reference %q reason is required", string(r.Kind), r.ID)
	}

	return nil
}

// EffectiveCapabilitiesSnapshot is the immutable, provider-neutral capability context for one invocation.
type EffectiveCapabilitiesSnapshot struct {
	sessionID       session.ID
	revision        SnapshotRevision
	tools           []EffectiveTool
	skills          []EffectiveSkill
	promptSections  []PromptSection
	staleReferences []StaleReference
}

// NewEffectiveCapabilitiesSnapshot validates, sorts, and defensively copies one resolved capability set.
func NewEffectiveCapabilitiesSnapshot(
	sessionID session.ID,
	revision SnapshotRevision,
	tools []EffectiveTool,
	skills []EffectiveSkill,
	promptSections []PromptSection,
	staleReferences []StaleReference,
) (EffectiveCapabilitiesSnapshot, error) {
	if err := sessionID.Validate(); err != nil {
		return EffectiveCapabilitiesSnapshot{}, err
	}
	if err := revision.Validate(); err != nil {
		return EffectiveCapabilitiesSnapshot{}, err
	}

	clonedTools, err := normalizeEffectiveTools(tools)
	if err != nil {
		return EffectiveCapabilitiesSnapshot{}, err
	}
	clonedSkills, err := normalizeEffectiveSkills(skills)
	if err != nil {
		return EffectiveCapabilitiesSnapshot{}, err
	}
	clonedSections, err := normalizePromptSections(promptSections)
	if err != nil {
		return EffectiveCapabilitiesSnapshot{}, err
	}
	clonedStaleReferences, err := normalizeStaleReferences(staleReferences)
	if err != nil {
		return EffectiveCapabilitiesSnapshot{}, err
	}

	return EffectiveCapabilitiesSnapshot{
		sessionID:       sessionID,
		revision:        revision,
		tools:           clonedTools,
		skills:          clonedSkills,
		promptSections:  clonedSections,
		staleReferences: clonedStaleReferences,
	}, nil
}

// SessionID returns the session identity bound to this snapshot.
func (s EffectiveCapabilitiesSnapshot) SessionID() session.ID {
	return s.sessionID
}

// Revision returns the exact registry/profile revision used by the resolver.
func (s EffectiveCapabilitiesSnapshot) Revision() SnapshotRevision {
	return s.revision
}

// Tools returns a defensive copy in stable tool-name order.
func (s EffectiveCapabilitiesSnapshot) Tools() []EffectiveTool {
	return cloneEffectiveTools(s.tools)
}

// Skills returns a defensive copy in stable skill-id order.
func (s EffectiveCapabilitiesSnapshot) Skills() []EffectiveSkill {
	return cloneEffectiveSkills(s.skills)
}

// PromptSections returns a defensive copy ordered by Order and then ID.
func (s EffectiveCapabilitiesSnapshot) PromptSections() []PromptSection {
	return append([]PromptSection(nil), s.promptSections...)
}

// StaleReferences returns unresolved profile references without exposing internal storage.
func (s EffectiveCapabilitiesSnapshot) StaleReferences() []StaleReference {
	return append([]StaleReference(nil), s.staleReferences...)
}

// Tool returns one effective tool without exposing mutable schema maps from the snapshot.
func (s EffectiveCapabilitiesSnapshot) Tool(name tool.Name) (EffectiveTool, bool) {
	index := sort.Search(len(s.tools), func(index int) bool {
		return string(s.tools[index].Contract.Name) >= string(name)
	})
	if index >= len(s.tools) || s.tools[index].Contract.Name != name {
		return EffectiveTool{}, false
	}

	return cloneEffectiveTool(s.tools[index]), true
}

// HasTool reports whether a tool is part of the effective invocation set.
func (s EffectiveCapabilitiesSnapshot) HasTool(name tool.Name) bool {
	_, exists := s.Tool(name)
	return exists
}

func normalizeEffectiveTools(values []EffectiveTool) ([]EffectiveTool, error) {
	result := cloneEffectiveTools(values)
	seen := make(map[tool.Name]struct{}, len(result))
	for index, value := range result {
		if err := value.Validate(); err != nil {
			return nil, fmt.Errorf("effective tool %d: %w", index, err)
		}
		if _, exists := seen[value.Contract.Name]; exists {
			return nil, fmt.Errorf("duplicate effective tool %q", string(value.Contract.Name))
		}
		seen[value.Contract.Name] = struct{}{}
	}

	sort.Slice(result, func(i, j int) bool {
		return string(result[i].Contract.Name) < string(result[j].Contract.Name)
	})
	return result, nil
}

func normalizeEffectiveSkills(values []EffectiveSkill) ([]EffectiveSkill, error) {
	result := cloneEffectiveSkills(values)
	seen := make(map[skill.ID]struct{}, len(result))
	for index, value := range result {
		if err := value.Validate(); err != nil {
			return nil, fmt.Errorf("effective skill %d: %w", index, err)
		}
		id := value.Skill.Manifest.ID
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("duplicate effective skill %q", string(id))
		}
		seen[id] = struct{}{}
	}

	sort.Slice(result, func(i, j int) bool {
		return string(result[i].Skill.Manifest.ID) < string(result[j].Skill.Manifest.ID)
	})
	return result, nil
}

func normalizePromptSections(values []PromptSection) ([]PromptSection, error) {
	result := append([]PromptSection(nil), values...)
	seen := make(map[string]struct{}, len(result))
	for index, value := range result {
		if err := value.Validate(); err != nil {
			return nil, fmt.Errorf("prompt section %d: %w", index, err)
		}
		if _, exists := seen[value.ID]; exists {
			return nil, fmt.Errorf("duplicate prompt section %q", value.ID)
		}
		seen[value.ID] = struct{}{}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Order == result[j].Order {
			return result[i].ID < result[j].ID
		}
		return result[i].Order < result[j].Order
	})
	return result, nil
}

func normalizeStaleReferences(values []StaleReference) ([]StaleReference, error) {
	result := append([]StaleReference(nil), values...)
	seen := make(map[string]struct{}, len(result))
	for index, value := range result {
		if err := value.Validate(); err != nil {
			return nil, fmt.Errorf("stale reference %d: %w", index, err)
		}
		key := string(value.Kind) + "\x00" + value.ID
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate stale %s reference %q", string(value.Kind), value.ID)
		}
		seen[key] = struct{}{}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Kind == result[j].Kind {
			return result[i].ID < result[j].ID
		}
		return result[i].Kind < result[j].Kind
	})
	return result, nil
}

func validateUniqueToolNames(sectionID string, field string, values []tool.Name) error {
	seen := make(map[tool.Name]struct{}, len(values))
	for index, name := range values {
		if err := name.Validate(); err != nil {
			return fmt.Errorf("conditional prompt section %q %s[%d]: %w", sectionID, field, index, err)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("conditional prompt section %q has duplicate %s tool %q", sectionID, field, string(name))
		}
		seen[name] = struct{}{}
	}
	return nil
}

func validateUniqueSkillIDs(sectionID string, field string, values []skill.ID) error {
	seen := make(map[skill.ID]struct{}, len(values))
	for index, id := range values {
		if err := id.Validate(); err != nil {
			return fmt.Errorf("conditional prompt section %q %s[%d]: %w", sectionID, field, index, err)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("conditional prompt section %q has duplicate %s skill %q", sectionID, field, string(id))
		}
		seen[id] = struct{}{}
	}
	return nil
}

func cloneEffectiveTools(values []EffectiveTool) []EffectiveTool {
	if values == nil {
		return nil
	}
	result := make([]EffectiveTool, len(values))
	for index, value := range values {
		result[index] = cloneEffectiveTool(value)
	}
	return result
}

func cloneEffectiveTool(value EffectiveTool) EffectiveTool {
	value.Contract.InputSchema = cloneStringAnyMap(value.Contract.InputSchema)
	value.Contract.OutputSchema = cloneStringAnyMap(value.Contract.OutputSchema)
	return value
}

func cloneEffectiveSkills(values []EffectiveSkill) []EffectiveSkill {
	if values == nil {
		return nil
	}
	result := make([]EffectiveSkill, len(values))
	for index, value := range values {
		result[index] = EffectiveSkill{Skill: cloneSkill(value.Skill)}
	}
	return result
}

func cloneSkill(value skill.Skill) skill.Skill {
	value.Manifest.ReferenceFiles = append([]string(nil), value.Manifest.ReferenceFiles...)
	value.Manifest.Tags = append([]string(nil), value.Manifest.Tags...)
	value.References = append([]skill.Reference(nil), value.References...)
	return value
}

func cloneStringAnyMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = cloneSchemaValue(item)
	}
	return result
}

func cloneSchemaValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneStringAnyMap(typed)
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = cloneSchemaValue(item)
		}
		return result
	case []string:
		return append([]string(nil), typed...)
	default:
		return typed
	}
}
