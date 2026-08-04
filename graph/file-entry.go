package graph

import (
	"path/filepath"
	"sort"
	"strings"

	corepkg "github.com/jurgen-kluft/gcore"
)

type FileType uint8

const (
	FileTypeNone FileType = iota
	FileTypeCppHeader
	FileTypeCppSource
	FileTypeCSource
	FileTypeCuHeader
	FileTypeCuSource
	FileTypeObjC
	FileTypeObjCpp
	FileTypeIxx
	FileTypeMxx
)

type FileEntry struct {
	Path      string
	Type      FileType
	Parent    *VirtualDirectory
	UUID      corepkg.UUID
	BuildUUID corepkg.UUID
}

func NewFileEntry(path string) *FileEntry {
	entry := &FileEntry{}
	entry.Init(path)
	return entry
}

func (entry *FileEntry) Init(path string, _ ...bool) {
	entry.Path = path
	if path != "" {
		entry.Path = filepath.Clean(path)
	}
	entry.Type = FileTypeNone
	switch strings.ToLower(filepath.Ext(path)) {
	case ".h", ".hh", ".hpp", ".hxx", ".inl":
		entry.Type = FileTypeCppHeader
	case ".c":
		entry.Type = FileTypeCSource
	case ".cc", ".cpp", ".cxx":
		entry.Type = FileTypeCppSource
	case ".cuh":
		entry.Type = FileTypeCuHeader
	case ".cu":
		entry.Type = FileTypeCuSource
	case ".m":
		entry.Type = FileTypeObjC
	case ".mm":
		entry.Type = FileTypeObjCpp
	case ".ixx":
		entry.Type = FileTypeIxx
	case ".mxx":
		entry.Type = FileTypeMxx
	}
}

func (entry *FileEntry) IsSource() bool {
	switch entry.Type {
	case FileTypeCppSource, FileTypeCSource, FileTypeCuSource, FileTypeObjC, FileTypeObjCpp, FileTypeIxx, FileTypeMxx:
		return true
	default:
		return false
	}
}

type FileEntryDict struct {
	Path   string
	Dict   map[string]int
	Keys   []string
	Values []*FileEntry
}

func NewFileEntryDict(path string) *FileEntryDict {
	return &FileEntryDict{Path: filepath.Clean(path), Dict: make(map[string]int)}
}

func (files *FileEntryDict) Add(path string) *FileEntry {
	path = filepath.Clean(path)
	if index, ok := files.Dict[path]; ok {
		return files.Values[index]
	}
	entry := NewFileEntry(path)
	files.Dict[path] = len(files.Values)
	files.Keys = append(files.Keys, path)
	files.Values = append(files.Values, entry)
	return entry
}

func (files *FileEntryDict) GetAbsPath(entry *FileEntry) string {
	if files.Path == "" || files.Path == "." {
		return entry.Path
	}
	return filepath.Join(files.Path, entry.Path)
}

func (files *FileEntryDict) GetRelativePath(entry *FileEntry, path string) (string, error) {
	return filepath.Rel(path, files.GetAbsPath(entry))
}

func (files *FileEntryDict) Sort() {
	sort.Slice(files.Values, func(i, j int) bool { return files.Values[i].Path < files.Values[j].Path })
	files.Dict = make(map[string]int, len(files.Values))
	files.Keys = files.Keys[:0]
	for index, entry := range files.Values {
		files.Dict[entry.Path] = index
		files.Keys = append(files.Keys, entry.Path)
	}
}
