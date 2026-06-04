package skill

import (
	"fmt"
	"io/fs"
	"strings"
)

// Loader reads explicitly declared skill files without owning manifest parsing or skill selection.
type Loader struct {
	FS fs.FS
}

// Load reads instruction and reference files from an explicit manifest and validates the result.
func (l Loader) Load(manifest Manifest) (Skill, error) {
	if l.FS == nil {
		return Skill{}, fmt.Errorf("skill loader fs is required")
	}

	if err := manifest.Validate(); err != nil {
		return Skill{}, err
	}

	instructions, err := l.readTextFile(manifest.InstructionFile)
	if err != nil {
		return Skill{}, fmt.Errorf("load skill %q instructions: %w", string(manifest.ID), err)
	}

	references := make([]Reference, 0, len(manifest.ReferenceFiles))
	for _, path := range manifest.ReferenceFiles {
		content, err := l.readTextFile(path)
		if err != nil {
			return Skill{}, fmt.Errorf("load skill %q reference %q: %w", string(manifest.ID), path, err)
		}

		references = append(references, Reference{
			Path:    path,
			Content: content,
		})
	}

	value := Skill{
		Manifest:     manifest,
		Instructions: instructions,
		References:   references,
	}

	if err := value.Validate(); err != nil {
		return Skill{}, err
	}

	return value, nil
}

// readTextFile centralizes content checks so empty files cannot masquerade as valid skill context.
func (l Loader) readTextFile(path string) (string, error) {
	content, err := fs.ReadFile(l.FS, path)
	if err != nil {
		return "", err
	}

	text := string(content)
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("file %q is empty", path)
	}

	return text, nil
}
