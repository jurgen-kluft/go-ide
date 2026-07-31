package msdev

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	corepkg "github.com/jurgen-kluft/gcore"
	"github.com/jurgen-kluft/gide/denv"
)

func TestGeneratePackage(t *testing.T) {
	root := t.TempDir()
	pkg := &denv.Package{RepoName: "example"}
	project := &denv.DevProject{
		Name:         "example",
		RepoName:     "example",
		PackagePath:  root,
		BuildType:    denv.BuildTypeApplication,
		BuildTargets: denv.BuildTargetWindowsX64,
		Configs: []*denv.DevConfig{
			denv.NewDevConfig(denv.BuildTypeApplication, denv.NewDebugDevConfig()),
			denv.NewDevConfig(denv.BuildTypeApplication, denv.NewReleaseDevConfig()),
		},
		Dependencies: denv.NewDevProjectList(),
	}
	pkg.MainApps = map[string]*denv.DevProject{project.Name: project}
	pkg.MainLibs = map[string]*denv.DevProject{}
	pkg.Library = map[string]*denv.DevProject{}
	pkg.Unittests = map[string]*denv.DevProject{}
	pkg.TestLibs = map[string]*denv.DevProject{}

	outputDir := filepath.Join(root, "target", "msdev")
	options := WorkspaceOptions{
		OutputDir:    outputDir,
		BuildTarget:  denv.BuildTargetWindowsX64,
		BuildConfigs: []denv.BuildConfig{denv.NewDebugDevConfig()},
		ResolveVars: func(_ *denv.DevProject, _ denv.BuildTarget, _ denv.BuildConfig, vars *corepkg.Vars) {
			vars.Set("build.cpp_standard", "/std:c++20")
			vars.Set("compiler.warning_flags", "/W4")
			vars.Set("compiler.c.link.extra_flags", "/SUBSYSTEM:CONSOLE", "/ERRORREPORT:PROMPT", "/MACHINE:X64")
		},
	}

	workspace, err := BuildWorkspace(pkg, options)
	if err != nil {
		t.Fatalf("BuildWorkspace() error = %v", err)
	}
	if workspace.Name != "example" || workspace.StartupProject != "example" {
		t.Fatalf("workspace identity = %q/%q, want example/example", workspace.Name, workspace.StartupProject)
	}
	if len(workspace.Projects) != 1 || len(workspace.Projects[0].Configurations) != 1 {
		t.Fatalf("workspace projects/configurations = %d/%d, want 1/1", len(workspace.Projects), len(workspace.Projects[0].Configurations))
	}
	configuration := workspace.Projects[0].Configurations[0]
	if configuration.Options.CppStandard != VsCppStandard20 {
		t.Errorf("CppStandard = %v, want %v", configuration.Options.CppStandard, VsCppStandard20)
	}
	if configuration.Options.LinkErrorReporting != VsLinkErrorReportingPrompt {
		t.Errorf("LinkErrorReporting = %v, want %v", configuration.Options.LinkErrorReporting, VsLinkErrorReportingPrompt)
	}
	if configuration.TargetExtension != ".exe" {
		t.Errorf("TargetExtension = %q, want .exe", configuration.TargetExtension)
	}

	if err := GeneratePackage(pkg, options); err != nil {
		t.Fatalf("GeneratePackage() error = %v", err)
	}
	projectFile, err := os.ReadFile(filepath.Join(outputDir, "example.vcxproj"))
	if err != nil {
		t.Fatal(err)
	}
	generated := string(projectFile)
	for _, expected := range []string{
		"<LanguageStandard>stdcpp20</LanguageStandard>",
		"<WarningLevel>Level4</WarningLevel>",
		"<LinkErrorReporting>Prompt</LinkErrorReporting>",
		"<TargetMachine>MachineX64</TargetMachine>",
	} {
		if !strings.Contains(generated, expected) {
			t.Errorf("generated project does not contain %q", expected)
		}
	}
	if _, err := os.Stat(filepath.Join(outputDir, "example.sln")); err != nil {
		t.Fatal(err)
	}
}

func TestBuildWorkspaceRejectsNonWindowsTarget(t *testing.T) {
	_, err := BuildWorkspace(&denv.Package{RepoName: "example"}, WorkspaceOptions{
		OutputDir:   t.TempDir(),
		BuildTarget: denv.BuildTargetLinuxX64,
	})
	if err == nil || !strings.Contains(err.Error(), "Windows target") {
		t.Fatalf("BuildWorkspace() error = %v, want Windows target error", err)
	}
}

func TestBuildWorkspaceUsesPackageVars(t *testing.T) {
	pkg := testPackage(t.TempDir())
	pkg.SetGetVarsFunc(func(_ denv.BuildTarget, _ denv.BuildConfig, _ string, vars *corepkg.Vars) {
		vars.Set("build.cpp_standard", "/std:c++17")
		vars.Set("compiler.c.link.extra_flags", "/ERRORREPORT:PROMPT", "/MACHINE:X64")
	})
	workspace, err := BuildWorkspace(pkg, WorkspaceOptions{
		OutputDir:   t.TempDir(),
		BuildTarget: denv.BuildTargetWindowsX64,
		BuildConfigs: []denv.BuildConfig{
			denv.NewDebugConfig(),
		},
	})
	if err != nil {
		t.Fatalf("BuildWorkspace() error = %v", err)
	}
	if len(workspace.Projects) != 1 || len(workspace.Projects[0].Configurations) != 1 {
		t.Fatalf("workspace projects/configurations = %d/%d, want 1/1", len(workspace.Projects), len(workspace.Projects[0].Configurations))
	}
	configuration := workspace.Projects[0].Configurations[0].Options
	if configuration.CppStandard != VsCppStandard17 {
		t.Errorf("CppStandard = %v, want %v", configuration.CppStandard, VsCppStandard17)
	}
	if configuration.LinkErrorReporting != VsLinkErrorReportingPrompt {
		t.Errorf("LinkErrorReporting = %v, want %v", configuration.LinkErrorReporting, VsLinkErrorReportingPrompt)
	}
}

func testPackage(root string) *denv.Package {
	pkg := &denv.Package{RepoName: "example"}
	project := &denv.DevProject{
		Name:         "example",
		RepoName:     "example",
		PackagePath:  root,
		BuildType:    denv.BuildTypeApplication,
		BuildTargets: denv.BuildTargetWindowsX64,
		Configs: []*denv.DevConfig{
			denv.NewDevConfig(denv.BuildTypeApplication, denv.NewDebugConfig()),
		},
		Dependencies: denv.NewDevProjectList(),
	}
	pkg.MainApps = map[string]*denv.DevProject{project.Name: project}
	pkg.MainLibs = map[string]*denv.DevProject{}
	pkg.Library = map[string]*denv.DevProject{}
	pkg.Unittests = map[string]*denv.DevProject{}
	pkg.TestLibs = map[string]*denv.DevProject{}
	return pkg
}
