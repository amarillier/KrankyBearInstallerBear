package packproject

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Save marshals the project back to YAML at path. Used by the GUI's
// Save/Save As; the CLI only ever reads project files via Load.
func Save(proj *Project, path string) error {
	data, err := yaml.Marshal(proj)
	if err != nil {
		return fmt.Errorf("encoding project file: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing project file: %w", err)
	}
	return nil
}
