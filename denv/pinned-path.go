package denv

import (
	"path/filepath"
	"strings"

	corepkg "github.com/jurgen-kluft/go-core"
)

type PinnedPath struct {
	Root string // Root directory
	Base string // Base directory
	Sub  string // Sub directory
}

func (fp PinnedPath) String() string {
	return filepath.Join(fp.Root, fp.Base, fp.Sub)
}

func (fp PinnedPath) RelativeTo(root string) string {
	return corepkg.PathGetRelativeTo(fp.String(), root)
}

type PinnedPaths struct {
	Values []PinnedPath
	seen   map[string]bool
}

func NewPinnedPaths() *PinnedPaths {
	return &PinnedPaths{seen: make(map[string]bool)}
}

func (paths *PinnedPaths) Add(path PinnedPath) {
	key := strings.ToLower(path.String())
	if paths.seen[key] {
		return
	}
	paths.seen[key] = true
	paths.Values = append(paths.Values, path)
}

func (paths *PinnedPaths) Copy() *PinnedPaths {
	copy := NewPinnedPaths()
	for _, path := range paths.Values {
		copy.Add(path)
	}
	return copy
}

func (paths *PinnedPaths) Merge(other *PinnedPaths) {
	for _, path := range other.Values {
		paths.Add(path)
	}
}
