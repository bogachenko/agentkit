package skill

import (
	"fmt"
	"strings"
)

// Reference keeps supporting skill content explicit instead of hiding it inside prompts.
type Reference struct {
	Path    string
	Content string
}

// Validation prevents empty reference documents from silently entering agent context.
func (r Reference) Validate() error {
	if err := validateRelativeFilePath(r.Path); err != nil {
		return err
	}

	if strings.TrimSpace(r.Content) == "" {
		return fmt.Errorf("skill reference %q content is required", r.Path)
	}

	return nil
}
