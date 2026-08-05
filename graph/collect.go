package graph

import (
	"fmt"
	"path/filepath"
	"sort"

	corepkg "github.com/jurgen-kluft/go-core"
	"github.com/jurgen-kluft/go-ide/denv"
)

type ProjectFiles struct {
	Groups         []*FileEntryDict
	VirtualFolders *VirtualDirectories
}

func CollectProjectFiles(project *denv.DevProject, target denv.BuildTarget) (*ProjectFiles, error) {
	if project == nil {
		return nil, fmt.Errorf("project is nil")
	}

	groupsByRoot := make(map[string]*FileEntryDict)
	absolutePaths := make(map[string]bool)
	exclusionFilter := denv.NewExclusionFilter(target)

	for _, source := range project.CollectSourceDirs() {
		root := filepath.Join(source.Path.Root, source.Path.Base, source.Path.Sub)
		if err := collectGlob(groupsByRoot, absolutePaths, root, source.Glob, exclusionFilter.IsExcluded); err != nil {
			return nil, fmt.Errorf("collect source files from %q: %w", root, err)
		}
	}
	for _, include := range project.CollectIncludeDirs() {
		root := include.String()
		if err := collectGlob(groupsByRoot, absolutePaths, root, "**/*.*", func(string) bool { return false }); err != nil {
			return nil, fmt.Errorf("collect header files from %q: %w", root, err)
		}
	}

	roots := make([]string, 0, len(groupsByRoot))
	for root := range groupsByRoot {
		roots = append(roots, root)
	}
	sort.Strings(roots)

	groups := make([]*FileEntryDict, 0, len(roots))
	virtualFolders := NewVirtualDirectories(project.ProjectPath())
	for _, root := range roots {
		group := groupsByRoot[root]
		group.Sort()
		groups = append(groups, group)
		for _, file := range group.Values {
			virtualFolders.AddFile(file)
		}
	}
	virtualFolders.Sort()
	virtualFolders.AssignUUIDs(project.RepoName+"/"+project.Name, groups)
	return &ProjectFiles{Groups: groups, VirtualFolders: virtualFolders}, nil
}

func collectGlob(groups map[string]*FileEntryDict, absolutePaths map[string]bool, root, pattern string, isExcluded func(string) bool) error {
	root = corepkg.PathNormalize(root)
	pattern = corepkg.PathNormalize(pattern)
	matched := make([]string, 0)
	fileFunc := func(_ string, relativePath string) {
		if !isExcluded(relativePath) && corepkg.GlobMatching(relativePath, pattern) {
			matched = append(matched, relativePath)
		}
	}
	if err := corepkg.FileEnumerate(root, func(string, string) bool { return true }, fileFunc); err != nil {
		return err
	}
	sort.Strings(matched)
	for _, relativePath := range matched {
		absolutePath := filepath.Clean(filepath.Join(root, relativePath))
		if absolutePaths[absolutePath] {
			continue
		}
		absolutePaths[absolutePath] = true
		group := groups[root]
		if group == nil {
			group = NewFileEntryDict(root)
			groups[root] = group
		}
		group.Add(relativePath)
	}
	return nil
}
