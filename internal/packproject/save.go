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
//
// When proj came from a real file (proj.sourceNode is set - see load.go),
// this preserves any hand-typed "#" comments from that file rather than
// silently dropping them, by merging them onto a fresh encoding of proj's
// current field values - see comments.go for exactly how. A Project with
// no sourceNode (built by hand: a brand-new project, the generated
// sample, every test) has no original comments to preserve, so this is a
// plain struct marshal, unchanged from before.
func Marshal(proj *Project) ([]byte, error) {
	if proj.sourceNode == nil {
		data, err := yaml.Marshal(proj)
		if err != nil {
			return nil, fmt.Errorf("encoding project file: %w", err)
		}
		return data, nil
	}

	var fresh yaml.Node
	if err := fresh.Encode(proj); err != nil {
		return nil, fmt.Errorf("encoding project file: %w", err)
	}
	mergeComments(proj.sourceNode, &fresh)

	data, err := yaml.Marshal(&fresh)
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
