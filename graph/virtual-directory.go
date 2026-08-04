package graph

import (
	"hash/fnv"
	"path/filepath"
	"sort"
	"strings"

	corepkg "github.com/jurgen-kluft/gcore"
)

type VirtualDirectory struct {
	DiskPath string
	Path     string
	Children []*VirtualDirectory
	Files    []*FileEntry
	Parent   *VirtualDirectory
	UUID     corepkg.UUID
}

type VirtualDirectories struct {
	DiskPath string
	Folders  []*VirtualDirectory
	Root     *VirtualDirectory
	byPath   map[string]*VirtualDirectory
}

func NewVirtualDirectories(diskPath string) *VirtualDirectories {
	root := &VirtualDirectory{Children: make([]*VirtualDirectory, 0), Files: make([]*FileEntry, 0)}
	return &VirtualDirectories{
		DiskPath: filepath.Clean(diskPath),
		Folders:  []*VirtualDirectory{root},
		Root:     root,
		byPath:   map[string]*VirtualDirectory{".": root},
	}
}

func (folders *VirtualDirectories) AddFile(file *FileEntry) {
	parent := folders.folder(filepath.Dir(file.Path))
	file.Parent = parent
	parent.Files = append(parent.Files, file)
}

func (folders *VirtualDirectories) AssignUUIDs(namespace string, groups []*FileEntryDict) {
	for _, directory := range folders.Folders {
		directory.UUID = StableUUID(namespace, "directory", directory.Path)
	}
	for _, group := range groups {
		for _, file := range group.Values {
			identity := filepath.Join(group.Path, file.Path)
			file.UUID = StableUUID(namespace, "file", identity)
			file.BuildUUID = StableUUID(namespace, "build", identity)
		}
	}
}

func (folders *VirtualDirectories) EnsureUUIDs(namespace string, groups []*FileEntryDict) {
	for _, directory := range folders.Folders {
		if directory.UUID == 0 {
			directory.UUID = StableUUID(namespace, "directory", directory.Path)
		}
	}
	for _, group := range groups {
		for _, file := range group.Values {
			identity := filepath.Join(group.Path, file.Path)
			if file.UUID == 0 {
				file.UUID = StableUUID(namespace, "file", identity)
			}
			if file.BuildUUID == 0 {
				file.BuildUUID = StableUUID(namespace, "build", identity)
			}
		}
	}
}

func (folders *VirtualDirectories) Sort() {
	var sortDirectory func(*VirtualDirectory)
	sortDirectory = func(directory *VirtualDirectory) {
		sort.Slice(directory.Children, func(i, j int) bool { return directory.Children[i].Path < directory.Children[j].Path })
		sort.Slice(directory.Files, func(i, j int) bool { return directory.Files[i].Path < directory.Files[j].Path })
		for _, child := range directory.Children {
			sortDirectory(child)
		}
	}
	sortDirectory(folders.Root)
	sort.Slice(folders.Folders, func(i, j int) bool { return folders.Folders[i].Path < folders.Folders[j].Path })
}

func StableUUID(parts ...string) corepkg.UUID {
	hash := fnv.New64a()
	for _, part := range parts {
		_, _ = hash.Write([]byte(strings.ReplaceAll(filepath.Clean(part), `\`, "/")))
		_, _ = hash.Write([]byte{0})
	}
	return corepkg.UUID(hash.Sum64())
}

func (folders *VirtualDirectories) folder(path string) *VirtualDirectory {
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return folders.Root
	}
	if existing, ok := folders.byPath[path]; ok {
		return existing
	}
	parent := folders.folder(filepath.Dir(path))
	directory := &VirtualDirectory{
		DiskPath: filepath.Join(folders.DiskPath, path),
		Path:     path,
		Parent:   parent,
		Children: make([]*VirtualDirectory, 0),
		Files:    make([]*FileEntry, 0),
	}
	folders.byPath[path] = directory
	folders.Folders = append(folders.Folders, directory)
	parent.Children = append(parent.Children, directory)
	return directory
}
