package graph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jurgen-kluft/go-ide/denv"
)

func TestFileEntryClassification(t *testing.T) {
	for path, expected := range map[string]FileType{
		"source/main.c":   FileTypeCSource,
		"source/main.cpp": FileTypeCppSource,
		"include/main.h":  FileTypeCppHeader,
		"source/main.mm":  FileTypeObjCpp,
		"source/main.ixx": FileTypeIxx,
	} {
		if actual := NewFileEntry(path).Type; actual != expected {
			t.Errorf("NewFileEntry(%q).Type = %v, want %v", path, actual, expected)
		}
	}
}

func TestVirtualDirectoriesAssignStableUUIDs(t *testing.T) {
	group := NewFileEntryDict(t.TempDir())
	file := group.Add(filepath.Join("nested", "main.cpp"))
	folders := NewVirtualDirectories(group.Path)
	folders.AddFile(file)
	folders.Sort()
	folders.AssignUUIDs("repo/project", []*FileEntryDict{group})

	directoryUUID := file.Parent.UUID
	fileUUID := file.UUID
	buildUUID := file.BuildUUID
	folders.AssignUUIDs("repo/project", []*FileEntryDict{group})
	if file.Parent.Path != "nested" || file.Parent.Parent != folders.Root {
		t.Fatalf("file parent = %#v, want nested directory under root", file.Parent)
	}
	if file.Parent.UUID != directoryUUID || file.UUID != fileUUID || file.BuildUUID != buildUUID {
		t.Fatal("AssignUUIDs produced unstable identities")
	}
	if file.UUID == file.BuildUUID {
		t.Fatal("file and build UUIDs must have distinct identities")
	}
}

func TestCollectProjectFilesIncludesHeadersAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "example")
	sourceDir := filepath.Join(projectDir, "source")
	includeDir := filepath.Join(projectDir, "include")
	for _, directory := range []string{sourceDir, filepath.Join(includeDir, "public")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path string) {
		t.Helper()
		if err := os.WriteFile(path, []byte("// test\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(sourceDir, "main.cpp"))
	write(filepath.Join(includeDir, "public", "example.h"))

	config := denv.NewDevConfig(denv.BuildTypeStaticLibrary, denv.NewDebugDevConfig())
	config.IncludeDirs = append(config.IncludeDirs, denv.PinnedPath{Root: root, Base: "example", Sub: "include"})
	project := &denv.DevProject{
		Name:        "example",
		RepoName:    "example",
		PackagePath: root,
		BuildType:   denv.BuildTypeStaticLibrary,
		SourceDirs: []denv.PinnedGlobPath{
			denv.NewPinnedGlobPath(root, "example", "source", "**/*.*"),
			denv.NewPinnedGlobPath(root, "example", "source", "**/*.cpp"),
		},
		Configs: []*denv.DevConfig{config},
	}

	files, err := CollectProjectFiles(project, denv.BuildTargetWindowsX64)
	if err != nil {
		t.Fatalf("CollectProjectFiles() error = %v", err)
	}
	if len(files.Groups) != 2 {
		t.Fatalf("group count = %d, want source and include groups", len(files.Groups))
	}
	count := 0
	for _, group := range files.Groups {
		count += len(group.Values)
	}
	if count != 2 {
		t.Fatalf("file count = %d, want 2 deduplicated files", count)
	}
	var header *FileEntry
	for _, group := range files.Groups {
		for _, file := range group.Values {
			if file.Type == FileTypeCppHeader {
				header = file
			}
		}
	}
	if header == nil || header.Parent == nil || header.Parent.Path != "public" {
		t.Fatalf("header parent = %#v, want public virtual directory", header)
	}
}
