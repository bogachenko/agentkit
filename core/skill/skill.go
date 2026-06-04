package skill

import (
	"fmt"
	"strings"
)

// Skill combines validated metadata with loaded instruction and reference content.
type Skill struct {
	Manifest     Manifest
	Instructions string
	References   []Reference
}

// Validation ensures agent builders receive complete skill content, not partial files.
func (s Skill) Validate() error {
	if err := s.Manifest.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(s.Instructions) == "" {
		return fmt.Errorf("skill %q instructions are required", string(s.Manifest.ID))
	}

	if len(s.References) != len(s.Manifest.ReferenceFiles) {
		return fmt.Errorf(
			"skill %q reference count mismatch: manifest=%d loaded=%d",
			string(s.Manifest.ID),
			len(s.Manifest.ReferenceFiles),
			len(s.References),
		)
	}

	for index, reference := range s.References {
		if err := reference.Validate(); err != nil {
			return fmt.Errorf("skill %q reference %d: %w", string(s.Manifest.ID), index, err)
		}

		if reference.Path != s.Manifest.ReferenceFiles[index] {
			return fmt.Errorf(
				"skill %q reference %d path mismatch: manifest=%q loaded=%q",
				string(s.Manifest.ID),
				index,
				s.Manifest.ReferenceFiles[index],
				reference.Path,
			)
		}
	}

	return nil
}
