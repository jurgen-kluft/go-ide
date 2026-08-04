package graph

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jurgen-kluft/gide/denv"
)

func TestWorkspaceOrdersProjectsAndBuildsGroups(t *testing.T) {
	workspace := NewWorkspace("example", t.TempDir(), denv.BuildTargetMacX64)
	workspace.OptionsXcode = &WorkspaceOptionsXcode{CppStd: CppStd17}
	zeta := NewProject(&denv.DevProject{Name: "zeta", BuildType: denv.BuildTypeStaticLibrary}, workspace.OutputDir)
	alpha := NewProject(&denv.DevProject{Name: "alpha", BuildType: denv.BuildTypeApplication}, workspace.OutputDir)

	workspace.AddProject(zeta, "libraries/core")
	workspace.AddProject(alpha, "applications")

	if workspace.Projects[0] != alpha || workspace.Projects[1] != zeta {
		t.Fatalf("project order = %q, %q, want alpha, zeta", workspace.Projects[0].Name, workspace.Projects[1].Name)
	}
	if zeta.Group == nil || zeta.Group.Path != filepath.Join("libraries", "core") || zeta.Group.Parent == nil || zeta.Group.Parent.Path != "libraries" {
		t.Fatalf("zeta group = %#v, want libraries/core hierarchy", zeta.Group)
	}
	if err := workspace.ValidateStructure(); err != nil {
		t.Fatalf("ValidateStructure() error = %v", err)
	}
}

func TestWorkspaceValidateStructureRejectsDuplicatesAndGroupCycles(t *testing.T) {
	workspace := NewWorkspace("example", t.TempDir(), denv.BuildTargetWindowsX64)
	workspace.OptionsVs = &WorkspaceOptionsVs{}
	workspace.AddProject(NewProject(&denv.DevProject{Name: "duplicate"}, workspace.OutputDir), "")
	workspace.AddProject(NewProject(&denv.DevProject{Name: "duplicate"}, workspace.OutputDir), "")
	if err := workspace.ValidateStructure(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("ValidateStructure() error = %v, want duplicate project error", err)
	}

	workspace.Projects = workspace.Projects[:1]
	workspace.ProjectGroups.Root.Children = append(workspace.ProjectGroups.Root.Children, workspace.ProjectGroups.Root)
	if err := workspace.ValidateStructure(); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("ValidateStructure() error = %v, want project group cycle", err)
	}
}

func TestWorkspaceRejectsMixedBackendPayloads(t *testing.T) {
	workspace := NewWorkspace("example", t.TempDir(), denv.BuildTargetWindowsX64)
	workspace.OptionsVs = &WorkspaceOptionsVs{}
	project := NewProject(&denv.DevProject{Name: "example"}, workspace.OutputDir)
	project.Configurations = []*ProjectConfiguration{{
		BuildConfig:  denv.NewDebugConfig(),
		OptionsXcode: NewXcodeConfiguration(denv.BuildTargetMacX64, denv.NewDebugConfig()),
	}}
	workspace.AddProject(project, "")

	if err := workspace.ValidateStructure(); err == nil || !strings.Contains(err.Error(), "Visual Studio options") {
		t.Fatalf("ValidateStructure() error = %v, want backend mismatch", err)
	}
}
