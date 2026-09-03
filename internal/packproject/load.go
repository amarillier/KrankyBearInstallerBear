package packproject

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load reads, defaults, and validates a project file, resolving its BaseDir
// to the file's own directory — what every backend resolves relative
// Source/LicenseFile/icon paths against. Used by the CLI, where a project
// that doesn't build cleanly should be rejected up front.
func Load(path string) (*Project, error) {
	proj, err := parseAndDefault(path)
	if err != nil {
		return nil, err
	}
	if err := proj.Validate(proj.BaseDir); err != nil {
		return nil, fmt.Errorf("invalid project file: %w", err)
	}
	return proj, nil
}

// LoadLenient parses and defaults a project file without validating it.
// Used by the GUI's Open so a work-in-progress project (missing a version,
// say) can still be reopened and finished rather than refusing to load at
// all — the GUI validates for real right before an actual build instead.
func LoadLenient(path string) (*Project, error) {
	return parseAndDefault(path)
}

func parseAndDefault(path string) (*Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading project file: %w", err)
	}

	var proj Project
	if err := yaml.Unmarshal(data, &proj); err != nil {
		return nil, fmt.Errorf("parsing project file: %w", err)
	}

	proj.Defaults()

	baseDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("resolving project file directory: %w", err)
	}
	proj.BaseDir = baseDir

	return &proj, nil
}
