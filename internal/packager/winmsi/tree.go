package winmsi

import (
	"fmt"
	"hash/fnv"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"installerbear/internal/packproject"
)

// fileNode is one <File>/<Component> pair — WiX best practice is one file
// per component, which is what keeps upgrade/repair semantics well-defined.
type fileNode struct {
	ID          string
	ComponentID string
	Name        string // the actual filename, e.g. "icon.png"
	Source      string // absolute build-host path
}

// dirNode is one <Directory>, recursively nested to mirror the payload's
// destination paths under INSTALLDIR.
type dirNode struct {
	ID    string
	Name  string // "" for a node whose Id alone identifies it to WiX (unused here; every node we create has a Name)
	Dirs  []*dirNode
	Files []fileNode
}

func (d *dirNode) child(name string, used map[string]bool) *dirNode {
	for _, c := range d.Dirs {
		if c.Name == name {
			return c
		}
	}
	c := &dirNode{ID: uniqueID(used, "dir_"+name), Name: name}
	d.Dirs = append(d.Dirs, c)
	return c
}

// buildDirTree walks the project's binary, optional license file, and every
// "windows"-targeted Payload entry into a directory tree rooted at
// INSTALLDIR, returning the tree, a flat list of every Component Id in it
// (for the <Feature>'s ComponentRefs — WiX has no "ref everything under
// this directory" shorthand, so the caller must enumerate them), and the
// main binary's own <File> Id (needed as the LaunchApplication
// CustomAction's FileKey when InstallExperience.LaunchAfterInstall is set).
func buildDirTree(proj *packproject.Project, bin packproject.BinaryEntry) (root *dirNode, allComponents []string, binaryFileID string, err error) {
	used := map[string]bool{"INSTALLDIR": true, "ProgramFiles64Folder": true, "LocalAppDataFolder": true, "TARGETDIR": true, "ProgramMenuFolder": true}
	root = &dirNode{ID: "INSTALLDIR", Name: proj.Identity.Name}

	addFile := func(dir *dirNode, name, source string) string {
		fileID := uniqueID(used, "file_"+name)
		compID := uniqueID(used, "cmp_"+name)
		dir.Files = append(dir.Files, fileNode{ID: fileID, ComponentID: compID, Name: name, Source: source})
		allComponents = append(allComponents, compID)
		return fileID
	}

	binaryFileID = addFile(root, proj.Windows.ExeName, proj.ResolvePath(bin.Path))
	if proj.Identity.LicenseFile != "" {
		addFile(root, "License.txt", proj.ResolvePath(proj.Identity.LicenseFile))
	}

	getOrCreateDir := func(destPath string) *dirNode {
		cur := root
		clean := filepath.ToSlash(filepath.Clean(destPath))
		if clean == "." || clean == "" {
			return cur
		}
		for _, part := range strings.Split(clean, "/") {
			if part == "" {
				continue
			}
			cur = cur.child(part, used)
		}
		return cur
	}

	for _, entry := range proj.Payload {
		if len(entry.OS) > 0 && !slices.Contains(entry.OS, "windows") {
			continue
		}
		src := proj.ResolvePath(entry.Source)

		if !entry.Recursive {
			addFile(getOrCreateDir(entry.Dest), filepath.Base(entry.Source), src)
			continue
		}

		walkErr := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, relErr := filepath.Rel(src, path)
			if relErr != nil {
				return relErr
			}
			if rel != "." && entry.ExcludesMatch(rel) {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			destDir := filepath.ToSlash(filepath.Join(entry.Dest, filepath.Dir(rel)))
			addFile(getOrCreateDir(destDir), filepath.Base(path), path)
			return nil
		})
		if walkErr != nil {
			return nil, nil, "", fmt.Errorf("walking payload %q: %w", entry.Source, walkErr)
		}
	}

	return root, allComponents, binaryFileID, nil
}

// uniqueID sanitizes base into a valid WiX identifier
// (^[A-Za-z_][A-Za-z0-9_.]*$, max 72 chars) and disambiguates it against
// every other Id already handed out, so two files/dirs whose names collide
// after sanitizing (e.g. differing only by a character WiX doesn't allow)
// still get distinct Ids.
func uniqueID(used map[string]bool, base string) string {
	id := sanitizeID(base)
	candidate := id
	for n := 2; used[candidate]; n++ {
		suffix := fmt.Sprintf("_%d", n)
		candidate = truncateID(id, 72-len(suffix)) + suffix
	}
	used[candidate] = true
	return candidate
}

func sanitizeID(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	id := b.String()
	if id == "" || (id[0] >= '0' && id[0] <= '9') {
		id = "_" + id
	}
	return truncateID(id, 72)
}

// truncateID shortens an over-length id deterministically, replacing the
// cut tail with a short hash so two different long names don't collapse
// into the same truncated prefix.
func truncateID(id string, maxLen int) string {
	if len(id) <= maxLen || maxLen <= 9 {
		if len(id) > maxLen {
			return id[:maxLen]
		}
		return id
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	suffix := fmt.Sprintf("_%08x", h.Sum32())
	return id[:maxLen-len(suffix)] + suffix
}
