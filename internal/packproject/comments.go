package packproject

import "gopkg.in/yaml.v3"

// comments.go preserves hand-typed "#..." comments in an installerbear.yaml
// across a load/save round-trip - a real gap Allan found: he'd edited
// sample-installerbear.yaml's own header comment block, opened the result
// in the GUI, saved, and the comments were silently gone. Deliberately a
// manual-editing-only capability, per his own framing: there's no GUI for
// adding/editing a comment, this only stops the app from destroying one a
// human typed directly into the file. Comments follow ordinary YAML syntax
// ("#" to end of line, exactly like a shell script), not some custom
// convention.
//
// How it works: parseAndDefault (load.go) decodes the file through a
// yaml.Node tree first (not just straight into the struct), and stashes
// that tree on Project.sourceNode. Marshal (save.go), when sourceNode is
// set, encodes the current struct into a *fresh* node tree (no comments -
// yaml.Node.Encode builds one from scratch) and then walks sourceNode and
// the fresh tree together, copying each comment from sourceNode onto the
// matching node in the fresh tree wherever a match can be found, before
// marshaling the fresh tree. A Project built by hand (BaseDir unset,
// sourceNode nil - a brand-new project, the generated sample, every test)
// has nothing to preserve, so Marshal falls back to a plain struct
// marshal exactly as before.
//
// Matching is by key name for a mapping (struct field) and by a natural
// per-shape key for a sequence (PayloadEntry by Source, BinaryEntry by
// OS+Arch, FileAssociation by Extension; a plain scalar sequence like
// Targets by its own value) - not by position, so a comment survives
// reordering, not just untouched edits elsewhere in the file. A comment
// that was only ever attached to a list item since deleted has nothing
// left to attach to and is simply dropped - there's no sensible place to
// put it, the same as Inno/NSIS or any other tool would do with a fully
// generic "diff a value tree, keep annotations" approach. This is
// deliberately best-effort, not a guarantee: heavily restructuring a
// project file by hand (renaming keys `#` comments were pinned to, moving
// a payload entry's own Source, etc.) between an app-driven load and save
// can lose a comment that a byte-for-byte diff tool would have kept -
// acceptable, since this is a convenience for notes/temporarily-disabled
// entries, not a full round-trip-preserving YAML editor.
func mergeComments(old, fresh *yaml.Node) {
	if old == nil || fresh == nil {
		return
	}
	// Unwrap the document node both the raw parse and Node.Encode wrap
	// their real content in.
	if old.Kind == yaml.DocumentNode && len(old.Content) == 1 {
		old = old.Content[0]
	}
	if fresh.Kind == yaml.DocumentNode && len(fresh.Content) == 1 {
		fresh = fresh.Content[0]
	}
	if old.Kind != fresh.Kind {
		return // shape mismatch (e.g. a field's type changed) - nothing sensible to merge
	}

	switch fresh.Kind {
	case yaml.ScalarNode:
		copyNodeComments(old, fresh)

	case yaml.MappingNode:
		oldKeys := make(map[string]*yaml.Node, len(old.Content)/2)
		oldVals := make(map[string]*yaml.Node, len(old.Content)/2)
		for i := 0; i+1 < len(old.Content); i += 2 {
			oldKeys[old.Content[i].Value] = old.Content[i]
			oldVals[old.Content[i].Value] = old.Content[i+1]
		}
		for i := 0; i+1 < len(fresh.Content); i += 2 {
			freshKey := fresh.Content[i]
			freshVal := fresh.Content[i+1]
			if oldKey, ok := oldKeys[freshKey.Value]; ok {
				copyNodeComments(oldKey, freshKey)
				mergeComments(oldVals[freshKey.Value], freshVal)
			}
		}

	case yaml.SequenceNode:
		matchedOld := make([]bool, len(old.Content))
		for _, freshItem := range fresh.Content {
			key := sequenceItemKey(freshItem)
			if key == "" {
				continue // no natural key for this shape - can't match safely, leave uncommented
			}
			for i, oldItem := range old.Content {
				if matchedOld[i] || sequenceItemKey(oldItem) != key {
					continue
				}
				copyNodeComments(oldItem, freshItem)
				mergeComments(oldItem, freshItem)
				matchedOld[i] = true
				break
			}
		}
	}
}

// copyNodeComments copies every comment field yaml.v3 tracks on a node.
// Safe to call even when the two nodes otherwise differ (e.g. a value
// changed) - comments and value are independent.
func copyNodeComments(from, to *yaml.Node) {
	if from == nil || to == nil {
		return
	}
	to.HeadComment = from.HeadComment
	to.LineComment = from.LineComment
	to.FootComment = from.FootComment
}

// sequenceItemKey returns a short string identifying one sequence item for
// comment-preservation matching, so an item keeps its comment even if the
// list around it was reordered/added to/removed from elsewhere. Returns ""
// for a shape this doesn't recognize, which mergeComments treats as "can't
// match this safely" rather than guessing.
func sequenceItemKey(n *yaml.Node) string {
	switch n.Kind {
	case yaml.ScalarNode:
		// A plain scalar list (Targets is the only one Project has today) -
		// the value itself is the natural key.
		return "scalar:" + n.Value
	case yaml.MappingNode:
		fields := make(map[string]string, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			fields[n.Content[i].Value] = n.Content[i+1].Value
		}
		switch {
		case fields["source"] != "":
			return "source:" + fields["source"] // PayloadEntry
		case fields["os"] != "" && fields["arch"] != "":
			return "bin:" + fields["os"] + "/" + fields["arch"] // BinaryEntry
		case fields["extension"] != "":
			return "ext:" + fields["extension"] // FileAssociation
		}
	}
	return ""
}
