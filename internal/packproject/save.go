package packproject

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Marshal encodes proj as installerbear.yaml would store it. Exposed
// separately from Save so the GUI can compare a fresh encoding against a
// prior one (its own "has anything changed since last save/load"
// unsaved-changes check) without going through a real file.
func Marshal(proj *Project) ([]byte, error) {
	data, err := yaml.Marshal(proj)
	if err != nil {
		return nil, fmt.Errorf("encoding project file: %w", err)
	}
	return data, nil
}

// Save marshals the project back to YAML at path. Used by the GUI's
// Save/Save As; the CLI only ever reads project files via Load.
func Save(proj *Project, path string) error {
	data, err := Marshal(proj)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing project file: %w", err)
	}
	return nil
}
