package xcode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jurgen-kluft/gide/denv"
)

func TestNewWorkspaceInitializesGraph(t *testing.T) {
	workspace := NewWorkspace("example", t.TempDir(), denv.BuildTargetMacUniversal)
	if workspace.CppStd != CppStd17 {
		t.Fatalf("workspace CppStd = %#v, want C++17 defaults", workspace.CppStd)
	}
	if workspace.ProjectList == nil || workspace.ProjectGroups == nil || workspace.ProjectGroups.Root == nil {
		t.Fatal("workspace graph is not initialized")
	}

	second := NewProject("zeta", denv.BuildTypeStaticLibrary, t.TempDir(), workspace.GenerateAbsPath)
	first := NewProject("alpha", denv.BuildTypeApplication, t.TempDir(), workspace.GenerateAbsPath)
	workspace.AddProject(second, "libraries")
	workspace.AddProject(first, "applications")
	if workspace.ProjectList.Values[0] != first || workspace.ProjectList.Values[1] != second {
		t.Errorf("project order = %q, %q, want alpha, zeta", workspace.ProjectList.Values[0].Name, workspace.ProjectList.Values[1].Name)
	}
	if first.Group == nil || first.Group.Path != "applications" {
		t.Errorf("project group = %#v, want applications", first.Group)
	}
}

func TestXcodeConfigurationDefaults(t *testing.T) {
	mac := NewXcodeConfiguration(denv.BuildTargetMacArm64, denv.NewDebugConfig())
	if value, ok := setting(mac.XcodeSettings, "SDKROOT"); !ok || value != "macosx" {
		t.Errorf("macOS SDKROOT = %q, %v, want macosx", value, ok)
	}
	if value, ok := setting(mac.XcodeSettings, "MACOSX_DEPLOYMENT_TARGET"); !ok || value != "10.15" {
		t.Errorf("macOS deployment target = %q, %v", value, ok)
	}
	if !contains(mac.CppDefines.Values, "TARGET_MAC") || !contains(mac.LinkFlags.Values, "-ObjC") {
		t.Errorf("macOS defaults = defines %v, link flags %v", mac.CppDefines.Values, mac.LinkFlags.Values)
	}

	ios := NewXcodeConfiguration(denv.BuildTargetAppleiOS, denv.NewReleaseConfig())
	if value, ok := setting(ios.XcodeSettings, "SDKROOT"); !ok || value != "iphoneos" {
		t.Errorf("iOS SDKROOT = %q, %v, want iphoneos", value, ok)
	}
	if _, ok := setting(ios.XcodeSettings, "MACOSX_DEPLOYMENT_TARGET"); ok {
		t.Error("iOS configuration contains macOS deployment target")
	}
	if value, ok := setting(ios.XcodeSettings, "LLVM_LTO"); !ok || value != "YES" {
		t.Errorf("release LLVM_LTO = %q, %v, want YES", value, ok)
	}
}

func TestXcodeConfigurationResolve(t *testing.T) {
	outputDir := t.TempDir()
	project := NewProject("example", denv.BuildTypeStaticLibrary, t.TempDir(), outputDir)
	local := NewXcodeConfiguration(denv.BuildTargetMacUniversal, denv.NewDebugConfig())
	local.CppDefines.Add("LOCAL")
	dependency := NewXcodeConfiguration(denv.BuildTargetMacUniversal, denv.NewDebugConfig())
	dependency.CppDefines.Add("DEPENDENCY")
	dependency.IncludeDirs.Add(denv.PinnedPath{Root: outputDir, Base: "dependency", Sub: "include"})

	resolved := local.Resolve(project, outputDir, dependency)
	if !contains(resolved.CppDefines.Values, "LOCAL") || !contains(resolved.CppDefines.Values, "DEPENDENCY") {
		t.Errorf("resolved defines = %v", resolved.CppDefines.Values)
	}
	if len(resolved.IncludeDirs.Values) != 1 {
		t.Errorf("resolved include dirs = %v, want dependency include", resolved.IncludeDirs.Values)
	}
	if !strings.Contains(resolved.Resolved.OutputTarget.Path, filepath.Join("x64|arm64", "libexample.a")) {
		t.Errorf("resolved output path = %q, want universal static library", resolved.Resolved.OutputTarget.Path)
	}
	if resolved.Resolved.BuildTmpDir.Path == "" {
		t.Error("resolved temporary directory is empty")
	}
}

func TestWorkspaceValidate(t *testing.T) {
	workspace := NewWorkspace("example", t.TempDir(), denv.BuildTargetMacX64)
	project := NewProject("example", denv.BuildTypeApplication, t.TempDir(), workspace.GenerateAbsPath)
	configuration := NewXcodeConfiguration(workspace.BuildTarget, denv.NewDebugConfig()).Resolve(project, workspace.GenerateAbsPath)
	project.Resolved.Configs.Add(configuration)
	workspace.AddProject(project, "applications")
	if err := workspace.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	workspace.ProjectGroups.Root.Children = append(workspace.ProjectGroups.Root.Children, workspace.ProjectGroups.Root)
	if err := workspace.Validate(); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("Validate() error = %v, want project group cycle", err)
	}
}

func TestWorkspaceValidateRejectsIncompleteWorkspace(t *testing.T) {
	workspace := NewWorkspace("example", t.TempDir(), denv.BuildTargetWindowsX64)
	if err := workspace.Validate(); err == nil || !strings.Contains(err.Error(), "macOS or iOS") {
		t.Fatalf("Validate() error = %v, want Apple target error", err)
	}

	workspace.BuildTarget = denv.BuildTargetMacX64
	if err := workspace.Validate(); err == nil || !strings.Contains(err.Error(), "at least one project") {
		t.Fatalf("Validate() error = %v, want missing project error", err)
	}
}

func TestXcodeGeneratorGenerate(t *testing.T) {
	outputDir := t.TempDir()
	workspace := NewWorkspace("example", outputDir, denv.BuildTargetMacArm64)
	project := NewProject("example", denv.BuildTypeStaticLibrary, t.TempDir(), outputDir)
	configuration := NewXcodeConfiguration(workspace.BuildTarget, denv.NewDebugConfig()).Resolve(project, outputDir)
	project.Resolved.Configs.Add(configuration)
	workspace.AddProject(project, "libraries")

	if err := NewXcodeGenerator(workspace).Generate(); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	for _, path := range []string{
		filepath.Join(outputDir, "example.xcworkspace", "contents.xcworkspacedata"),
		filepath.Join(outputDir, "example.xcodeproj", "project.pbxproj"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("generated file %q: %v", path, err)
		}
	}
}

func TestXcodeProjectCollectsAndEmitsSharedGraph(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "example")
	sourceDir := filepath.Join(projectDir, "source", "nested")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "main.cpp"), []byte("int main() { return 0; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	definition := &denv.DevProject{
		Name:        "example",
		RepoName:    "example",
		PackagePath: root,
		BuildType:   denv.BuildTypeApplication,
		SourceDirs: []denv.PinnedGlobPath{
			denv.NewPinnedGlobPath(root, "example", "source", "**/*.cpp"),
		},
	}

	outputDir := filepath.Join(root, "ide")
	workspace := NewWorkspace("example", outputDir, denv.BuildTargetMacArm64)
	project := NewProject("example", definition.BuildType, projectDir, outputDir)
	if err := project.CollectFiles(definition, workspace.BuildTarget); err != nil {
		t.Fatalf("CollectFiles() error = %v", err)
	}
	configuration := NewXcodeConfiguration(workspace.BuildTarget, denv.NewDebugConfig()).Resolve(project, outputDir)
	project.Resolved.Configs.Add(configuration)
	workspace.AddProject(project, "applications")

	if len(project.SrcFileGroups) != 1 || len(project.SrcFileGroups[0].Values) != 1 {
		t.Fatalf("source groups = %#v, want one collected source", project.SrcFileGroups)
	}
	file := project.SrcFileGroups[0].Values[0]
	if file.Parent == nil || file.Parent.UUID == 0 {
		t.Fatalf("collected file parent = %#v, want shared virtual directory UUID", file.Parent)
	}
	if err := NewXcodeGenerator(workspace).Generate(); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	pbx := readXcodeTestFile(t, filepath.Join(outputDir, "example.xcodeproj", "project.pbxproj"))
	if !strings.Contains(pbx, file.Parent.UUID.ForXCode()) {
		t.Fatalf("project.pbxproj does not contain shared directory UUID %s", file.Parent.UUID.ForXCode())
	}
}

func readXcodeTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func setting(settings *OrderedSettings, key string) (string, bool) {
	for index, candidate := range settings.Keys {
		if candidate == key {
			return settings.Values[index], true
		}
	}
	return "", false
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
