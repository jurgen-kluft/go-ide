package xcode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	corepkg "github.com/jurgen-kluft/go-core"
	"github.com/jurgen-kluft/go-ide/denv"
	"github.com/jurgen-kluft/go-ide/graph"
)

func TestBuildWorkspaceAndGeneratePackage(t *testing.T) {
	root := t.TempDir()
	buildConfig := denv.NewDebugConfig()
	library := &denv.DevProject{
		Name:         "library",
		RepoName:     "library",
		PackagePath:  root,
		BuildType:    denv.BuildTypeStaticLibrary,
		BuildTargets: denv.BuildTargetMacArm64,
		Configs:      []*denv.DevConfig{denv.NewDevConfig(denv.BuildTypeStaticLibrary, buildConfig)},
		Dependencies: denv.NewDevProjectList(),
	}
	library.Configs[0].Defines.Add("LIBRARY_DEFINE")
	application := &denv.DevProject{
		Name:         "application",
		RepoName:     "application",
		PackagePath:  root,
		BuildType:    denv.BuildTypeApplication,
		BuildTargets: denv.BuildTargetMacArm64,
		Configs:      []*denv.DevConfig{denv.NewDevConfig(denv.BuildTypeApplication, buildConfig)},
		Dependencies: denv.NewDevProjectList(),
	}
	application.Dependencies.Add(library)
	pkg := &denv.Package{
		RepoName:  "example",
		MainApps:  map[string]*denv.DevProject{application.Name: application},
		MainLibs:  map[string]*denv.DevProject{library.Name: library},
		Library:   map[string]*denv.DevProject{},
		Unittests: map[string]*denv.DevProject{},
		TestLibs:  map[string]*denv.DevProject{},
	}
	var hardwareID string
	pkg.SetGetVarsFunc(func(_ denv.BuildTarget, _ denv.BuildConfig, hardware string, vars *corepkg.Vars) {
		hardwareID = hardware
		vars.Set("build.cpp_standard", "-std=c++20")
		vars.Set("build.defines", "PACKAGE_DEFINE")
		vars.Set("compiler.flags", "-Wall")
	})
	outputDir := filepath.Join(root, "xcode")
	options := WorkspaceOptions{
		OutputDir:       outputDir,
		BuildTarget:     denv.BuildTargetMacArm64,
		BuildConfigs:    []denv.BuildConfig{buildConfig},
		OSTargetVersion: "14.0",
	}
	workspace, err := buildWorkspace(pkg, options)
	if err != nil {
		t.Fatalf("BuildWorkspace() error = %v", err)
	}
	if hardwareID != "" {
		t.Errorf("Package.GetVars hardware ID = %q, want empty", hardwareID)
	}
	if workspace.OptionsXcode.CppStd != CppStd20 {
		t.Errorf("workspace C++ standard = %v, want C++20", workspace.OptionsXcode.CppStd)
	}
	if workspace.StartupProject != application.Name {
		t.Errorf("startup project = %q, want %q", workspace.StartupProject, application.Name)
	}
	var applicationConfiguration *XcodeConfiguration
	for _, project := range workspace.Projects {
		if project.Name == application.Name {
			applicationConfiguration = project.Configurations[0].OptionsXcode
		}
	}
	if applicationConfiguration == nil || !contains(applicationConfiguration.CppDefines.Values, "PACKAGE_DEFINE") || !contains(applicationConfiguration.CppDefines.Values, "LIBRARY_DEFINE") || !contains(applicationConfiguration.CppFlags.Values, "-Wall") {
		t.Fatalf("application configuration = %#v, want resolved package Vars", applicationConfiguration)
	}
	if value, ok := setting(applicationConfiguration.XcodeSettings, "MACOSX_DEPLOYMENT_TARGET"); !ok || value != "14.0" {
		t.Errorf("macOS deployment target = %q, %v, want 14.0", value, ok)
	}
	if err := GeneratePackage(pkg, options); err != nil {
		t.Fatalf("GeneratePackage() error = %v", err)
	}
	for _, path := range []string{
		filepath.Join(outputDir, "example.xcworkspace", "contents.xcworkspacedata"),
		filepath.Join(outputDir, "application.xcodeproj", "project.pbxproj"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("generated file %q: %v", path, err)
		}
	}
}

func TestBuildWorkspaceRejectsNonAppleTarget(t *testing.T) {
	_, err := buildWorkspace(&denv.Package{RepoName: "example"}, WorkspaceOptions{
		OutputDir:   t.TempDir(),
		BuildTarget: denv.BuildTargetWindowsX64,
	})
	if err == nil || !strings.Contains(err.Error(), "macOS or iOS") {
		t.Fatalf("BuildWorkspace() error = %v, want Apple target error", err)
	}
}

func TestBuildProjectConfigurationsUsesIOSTargetVersion(t *testing.T) {
	project := &denv.DevProject{
		Name:      "library",
		BuildType: denv.BuildTypeStaticLibrary,
		Configs: []*denv.DevConfig{
			denv.NewDevConfig(denv.BuildTypeStaticLibrary, denv.NewDebugConfig()),
		},
	}
	configurations, _, err := buildProjectConfigurations(project, WorkspaceOptions{
		BuildTarget:     denv.BuildTargetAppleiOS,
		OSTargetVersion: "17.0",
		ResolveVars:     func(_ *denv.DevProject, _ denv.BuildTarget, _ denv.BuildConfig, _ *corepkg.Vars) {},
	})
	if err != nil {
		t.Fatalf("buildProjectConfigurations() error = %v", err)
	}
	settings := configurations[0].OptionsXcode.XcodeSettings
	if value, ok := setting(settings, "IPHONEOS_DEPLOYMENT_TARGET"); !ok || value != "17.0" {
		t.Errorf("iOS deployment target = %q, %v, want 17.0", value, ok)
	}
}

func TestNewWorkspaceInitializesGraph(t *testing.T) {
	workspace := newWorkspace("example", t.TempDir(), denv.BuildTargetMacUniversal)
	if workspace.OptionsXcode == nil || workspace.OptionsXcode.CppStd != CppStd17 {
		t.Fatalf("workspace Xcode options = %#v, want C++17 defaults", workspace.OptionsXcode)
	}
	if workspace.Projects == nil || workspace.ProjectGroups == nil || workspace.ProjectGroups.Root == nil {
		t.Fatal("workspace graph is not initialized")
	}

	second := newProject("zeta", denv.BuildTypeStaticLibrary, t.TempDir(), workspace.OutputDir)
	first := newProject("alpha", denv.BuildTypeApplication, t.TempDir(), workspace.OutputDir)
	workspace.AddProject(second, "libraries")
	workspace.AddProject(first, "applications")
	if workspace.Projects[0] != first || workspace.Projects[1] != second {
		t.Errorf("project order = %q, %q, want alpha, zeta", workspace.Projects[0].Name, workspace.Projects[1].Name)
	}
	if first.Group == nil || first.Group.Path != "applications" {
		t.Errorf("project group = %#v, want applications", first.Group)
	}
}

func TestXcodeConfigurationDefaults(t *testing.T) {
	mac := graph.NewXcodeConfiguration(denv.BuildTargetMacArm64, denv.NewDebugConfig())
	if value, ok := setting(mac.XcodeSettings, "SDKROOT"); !ok || value != "macosx" {
		t.Errorf("macOS SDKROOT = %q, %v, want macosx", value, ok)
	}
	if value, ok := setting(mac.XcodeSettings, "MACOSX_DEPLOYMENT_TARGET"); !ok || value != "10.15" {
		t.Errorf("macOS deployment target = %q, %v", value, ok)
	}
	if !contains(mac.CppDefines.Values, "TARGET_MAC") || !contains(mac.LinkFlags.Values, "-ObjC") {
		t.Errorf("macOS defaults = defines %v, link flags %v", mac.CppDefines.Values, mac.LinkFlags.Values)
	}

	ios := graph.NewXcodeConfiguration(denv.BuildTargetAppleiOS, denv.NewReleaseConfig())
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
	project := newProject("example", denv.BuildTypeStaticLibrary, t.TempDir(), outputDir)
	local := graph.NewXcodeConfiguration(denv.BuildTargetMacUniversal, denv.NewDebugConfig())
	local.CppDefines.Add("LOCAL")
	dependency := graph.NewXcodeConfiguration(denv.BuildTargetMacUniversal, denv.NewDebugConfig())
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
	workspace := newWorkspace("example", t.TempDir(), denv.BuildTargetMacX64)
	project := newProject("example", denv.BuildTypeApplication, t.TempDir(), workspace.OutputDir)
	configuration := graph.NewXcodeConfiguration(workspace.BuildTarget, denv.NewDebugConfig()).Resolve(project, workspace.OutputDir)
	project.AddXcodeConfiguration(configuration)
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
	workspace := newWorkspace("example", t.TempDir(), denv.BuildTargetWindowsX64)
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
	workspace := newWorkspace("example", outputDir, denv.BuildTargetMacArm64)
	project := newProject("example", denv.BuildTypeStaticLibrary, t.TempDir(), outputDir)
	configuration := graph.NewXcodeConfiguration(workspace.BuildTarget, denv.NewDebugConfig()).Resolve(project, outputDir)
	project.AddXcodeConfiguration(configuration)
	workspace.AddProject(project, "libraries")

	if err := Generate(workspace); err != nil {
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
	workspace := newWorkspace("example", outputDir, denv.BuildTargetMacArm64)
	project := newProject("example", definition.BuildType, projectDir, outputDir)
	if err := project.CollectFiles(definition, workspace.BuildTarget); err != nil {
		t.Fatalf("CollectFiles() error = %v", err)
	}
	configuration := graph.NewXcodeConfiguration(workspace.BuildTarget, denv.NewDebugConfig()).Resolve(project, outputDir)
	project.AddXcodeConfiguration(configuration)
	workspace.AddProject(project, "applications")

	if len(project.Files.Groups) != 1 || len(project.Files.Groups[0].Values) != 1 {
		t.Fatalf("source groups = %#v, want one collected source", project.Files.Groups)
	}
	file := project.Files.Groups[0].Values[0]
	if file.Parent == nil || file.Parent.UUID == 0 {
		t.Fatalf("collected file parent = %#v, want shared virtual directory UUID", file.Parent)
	}
	if err := Generate(workspace); err != nil {
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

func setting(settings *corepkg.KeyValueSet, key string) (string, bool) {
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
