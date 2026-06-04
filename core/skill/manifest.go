package skill

import (
	"fmt"
	"strings"
)

// Manifest declares skill files and metadata without coupling core to YAML parsing.
type Manifest struct {
	ID              ID
	Name            string
	Description     string
	Version         string
	InstructionFile string
	ReferenceFiles  []string
	Tags            []string
}

// Validation blocks incomplete skill declarations before any files are loaded.
func (m Manifest) Validate() error {
	if err := m.ID.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("skill name is required for %q", string(m.ID))
	}

	if strings.TrimSpace(m.Description) == "" {
		return fmt.Errorf("skill description is required for %q", string(m.ID))
	}

	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("skill version is required for %q", string(m.ID))
	}

	if strings.TrimSpace(m.InstructionFile) == "" {
		return fmt.Errorf("skill instruction file is required for %q", string(m.ID))
	}

	if err := validateRelativeFilePath(m.InstructionFile); err != nil {
		return fmt.Errorf("skill %q instruction file: %w", string(m.ID), err)
	}

	seenReferences := map[string]struct{}{}
	for index, file := range m.ReferenceFiles {
		if err := validateRelativeFilePath(file); err != nil {
			return fmt.Errorf("skill %q reference file %d: %w", string(m.ID), index, err)
		}

		normalized := strings.TrimSpace(file)
		if _, exists := seenReferences[normalized]; exists {
			return fmt.Errorf("skill %q has duplicate reference file %q", string(m.ID), normalized)
		}

		seenReferences[normalized] = struct{}{}
	}

	seenTags := map[string]struct{}{}
	for index, tag := range m.Tags {
		normalized := strings.TrimSpace(tag)
		if normalized == "" {
			return fmt.Errorf("skill %q tag %d is empty", string(m.ID), index)
		}

		if _, exists := seenTags[normalized]; exists {
			return fmt.Errorf("skill %q has duplicate tag %q", string(m.ID), normalized)
		}

		seenTags[normalized] = struct{}{}
	}

	return nil
}

// Path validation prevents skill manifests from escaping their declared skill directory.
func validateRelativeFilePath(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return fmt.Errorf("file path is required")
	}

	if strings.HasPrefix(trimmed, "/") {
		return fmt.Errorf("absolute paths are not allowed")
	}

	parts := strings.Split(trimmed, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("unsafe path segment %q", part)
		}
	}

	return nil
}
