package msdev

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	corepkg "github.com/jurgen-kluft/go-core"
	"github.com/jurgen-kluft/go-ide/denv"
	"github.com/jurgen-kluft/go-ide/graph"
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
		OutputDir:       outputDir,
		BuildTarget:     denv.BuildTargetWindowsX64,
		BuildConfigs:    []denv.BuildConfig{denv.NewDebugDevConfig()},
		OSTargetVersion: "10.0.22621.0",
		VisualStudio: graph.WorkspaceOptionsVs{
			PlatformToolset: "v142",
		},
		ResolveVars: func(_ *denv.DevProject, _ denv.BuildTarget, _ denv.BuildConfig, vars *corepkg.Vars) {
			vars.Set("build.cpp_standard", "/std:c++20")
			vars.Set("compiler.warning_flags", "/W4")
			vars.Set("compiler.c.link.extra_flags", "/SUBSYSTEM:CONSOLE", "/ERRORREPORT:PromptImmediately", "/MACHINE:X64")
		},
	}

	workspace, err := buildWorkspace(pkg, options)
	if err != nil {
		t.Fatalf("buildWorkspace() error = %v", err)
	}
	if workspace.Name != "example" || workspace.StartupProject != "example" {
		t.Fatalf("workspace identity = %q/%q, want example/example", workspace.Name, workspace.StartupProject)
	}
	if len(workspace.Projects) != 1 || len(workspace.Projects[0].Configurations) != 1 {
		t.Fatalf("workspace projects/configurations = %d/%d, want 1/1", len(workspace.Projects), len(workspace.Projects[0].Configurations))
	}
	configuration := workspace.Projects[0].Configurations[0]
	if configuration.OptionsVs.CppStandard != VsCppStandard20 {
		t.Errorf("CppStandard = %v, want %v", configuration.OptionsVs.CppStandard, VsCppStandard20)
	}
	if configuration.OptionsVs.LinkErrorReporting != VsLinkErrorReportingPrompt {
		t.Errorf("LinkErrorReporting = %v, want %v", configuration.OptionsVs.LinkErrorReporting, VsLinkErrorReportingPrompt)
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
		"<WindowsTargetPlatformVersion>10.0.22621.0</WindowsTargetPlatformVersion>",
		"<PlatformToolset>v142</PlatformToolset>",
		"<LanguageStandard>stdcpp20</LanguageStandard>",
		"<WarningLevel>Level4</WarningLevel>",
		"<LinkErrorReporting>PromptImmediately</LinkErrorReporting>",
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
	_, err := buildWorkspace(&denv.Package{RepoName: "example"}, WorkspaceOptions{
		OutputDir:   t.TempDir(),
		BuildTarget: denv.BuildTargetLinuxX64,
	})
	if err == nil || !strings.Contains(err.Error(), "Windows target") {
		t.Fatalf("buildWorkspace() error = %v, want Windows target error", err)
	}
}

func TestBuildWorkspaceUsesPackageVars(t *testing.T) {
	pkg := testPackage(t.TempDir())
	pkg.SetGetVarsFunc(func(_ denv.BuildTarget, _ denv.BuildConfig, _ string, vars *corepkg.Vars) {
		vars.Set("build.cpp_standard", "/std:c++17")
		vars.Set("compiler.c.link.extra_flags", "/ERRORREPORT:PromptImmediately", "/MACHINE:X64")
	})
	workspace, err := buildWorkspace(pkg, WorkspaceOptions{
		OutputDir:   t.TempDir(),
		BuildTarget: denv.BuildTargetWindowsX64,
		BuildConfigs: []denv.BuildConfig{
			denv.NewDebugConfig(),
		},
	})
	if err != nil {
		t.Fatalf("buildWorkspace() error = %v", err)
	}
	if len(workspace.Projects) != 1 || len(workspace.Projects[0].Configurations) != 1 {
		t.Fatalf("workspace projects/configurations = %d/%d, want 1/1", len(workspace.Projects), len(workspace.Projects[0].Configurations))
	}
	configuration := workspace.Projects[0].Configurations[0].OptionsVs
	if configuration.CppStandard != VsCppStandard17 {
		t.Errorf("CppStandard = %v, want %v", configuration.CppStandard, VsCppStandard17)
	}
	if configuration.LinkErrorReporting != VsLinkErrorReportingPrompt {
		t.Errorf("LinkErrorReporting = %v, want %v", configuration.LinkErrorReporting, VsLinkErrorReportingPrompt)
	}
}

func TestBuildWorkspaceResolvesDependencyConfigurations(t *testing.T) {
	root := t.TempDir()
	buildConfig := denv.NewDebugConfig()
	newProject := func(name string, buildType denv.BuildType) *denv.DevProject {
		config := denv.NewDevConfig(buildType, buildConfig)
		config.Defines.Add(strings.ToUpper(name) + "_DEFINE")
		config.IncludeDirs = append(config.IncludeDirs, denv.PinnedPath{Root: root, Base: name, Sub: "include"})
		config.LibPaths = append(config.LibPaths, denv.PinnedPath{Root: root, Base: name, Sub: "lib"})
		config.Libs = append(config.Libs, name+".lib")
		return &denv.DevProject{
			Name:         name,
			RepoName:     name,
			PackagePath:  root,
			BuildType:    buildType,
			BuildTargets: denv.BuildTargetWindowsX64,
			Configs:      []*denv.DevConfig{config},
			Dependencies: denv.NewDevProjectList(),
		}
	}

	base := newProject("base", denv.BuildTypeStaticLibrary)
	middle := newProject("middle", denv.BuildTypeStaticLibrary)
	side := newProject("side", denv.BuildTypeStaticLibrary)
	app := newProject("app", denv.BuildTypeApplication)
	middle.Dependencies.Add(base)
	side.Dependencies.Add(base)
	app.Dependencies.Add(side)
	app.Dependencies.Add(middle)

	pkg := &denv.Package{
		RepoName:  "example",
		MainApps:  map[string]*denv.DevProject{app.Name: app},
		MainLibs:  map[string]*denv.DevProject{side.Name: side, middle.Name: middle, base.Name: base},
		Library:   map[string]*denv.DevProject{},
		Unittests: map[string]*denv.DevProject{},
		TestLibs:  map[string]*denv.DevProject{},
	}
	workspace, err := buildWorkspace(pkg, WorkspaceOptions{
		OutputDir:   filepath.Join(root, "msdev"),
		BuildTarget: denv.BuildTargetWindowsX64,
		ResolveVars: func(project *denv.DevProject, _ denv.BuildTarget, _ denv.BuildConfig, vars *corepkg.Vars) {
			switch project.Name {
			case "base":
				vars.Set("compiler.warning_flags", "/W2")
				vars.Set("compiler.flags", "/MP")
			case "middle":
				vars.Set("compiler.warning_flags", "/W3")
				vars.Set("compiler.flags", "/WX")
			case "side":
				vars.Set("compiler.warning_flags", "/W1")
				vars.Set("compiler.flags", "/nologo")
			case "app":
				vars.Set("compiler.warning_flags", "/W4")
				vars.Set("compiler.flags", "/FC")
			}
		},
	})
	if err != nil {
		t.Fatalf("buildWorkspace() error = %v", err)
	}

	var appProject *Project
	for _, project := range workspace.Projects {
		if project.Definition.Name == app.Name {
			appProject = project
			break
		}
	}
	appOptions := appProject.Configurations[0].OptionsVs
	for _, expected := range []string{"BASE_DEFINE", "MIDDLE_DEFINE", "SIDE_DEFINE", "APP_DEFINE"} {
		if !containsString(appOptions.Defines, expected) {
			t.Errorf("resolved defines %v do not contain %q", appOptions.Defines, expected)
		}
	}
	for _, expected := range []string{"base.lib", "middle.lib", "side.lib", "app.lib"} {
		if !containsString(appOptions.Libraries, expected) {
			t.Errorf("resolved libraries %v do not contain %q", appOptions.Libraries, expected)
		}
	}
	for _, projectName := range []string{"base", "middle", "side", "app"} {
		includePath := filepath.Join(root, projectName, "include")
		if !containsString(appOptions.IncludePaths, includePath) {
			t.Errorf("resolved include paths %v do not contain %q", appOptions.IncludePaths, includePath)
		}
		libraryPath := filepath.Join(root, projectName, "lib")
		if !containsString(appOptions.LibraryPaths, libraryPath) {
			t.Errorf("resolved library paths %v do not contain %q", appOptions.LibraryPaths, libraryPath)
		}
	}
	if appOptions.WarningLevel != VsWarningLevel4 {
		t.Errorf("resolved WarningLevel = %v, want %v", appOptions.WarningLevel, VsWarningLevel4)
	}
	for _, option := range []VsCompileOptions{VsCompileOptionMultiProcessor, VsCompileOptionTreatWarningsAsErrors, VsCompileOptionSuppressStartupBanner, VsCompileOptionFullPathDiagnostics} {
		if !appOptions.CompileOptions.Has(option) {
			t.Errorf("resolved CompileOptions %v do not contain %v", appOptions.CompileOptions, option)
		}
	}

	if err := os.MkdirAll(workspace.OutputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectGuids := map[string]string{
		"app":    stableGuid("app"),
		"middle": stableGuid("middle"),
		"side":   stableGuid("side"),
		"base":   stableGuid("base"),
	}
	if err := generateProject(*workspace, appProject, nil, projectGuids); err != nil {
		t.Fatalf("generateProject() error = %v", err)
	}
	generated := readTestFile(t, filepath.Join(workspace.OutputDir, "app.vcxproj"))
	assertContains(t, generated, `<PreprocessorDefinitions>BASE_DEFINE;MIDDLE_DEFINE;SIDE_DEFINE;APP_DEFINE;%(PreprocessorDefinitions)</PreprocessorDefinitions>`)
	assertContains(t, generated, `<AdditionalDependencies>base.lib;middle.lib;side.lib;app.lib;%(AdditionalDependencies)</AdditionalDependencies>`)
	assertContains(t, generated, `<ProjectReference Include="middle.vcxproj">`)
	assertContains(t, generated, `<ProjectReference Include="side.vcxproj">`)
	if strings.Contains(generated, `<ProjectReference Include="base.vcxproj">`) {
		t.Errorf("generated project contains transitive project reference\n%s", generated)
	}
	if strings.Count(generated, "BASE_DEFINE") != 1 || strings.Count(generated, "base.lib") != 1 {
		t.Errorf("generated project inherited shared dependency more than once\n%s", generated)
	}
}

func TestBuildWorkspaceRejectsDependencyCycle(t *testing.T) {
	buildConfig := denv.NewDebugConfig()
	newProject := func(name string) *denv.DevProject {
		return &denv.DevProject{
			Name:         name,
			RepoName:     name,
			BuildType:    denv.BuildTypeStaticLibrary,
			BuildTargets: denv.BuildTargetWindowsX64,
			Configs:      []*denv.DevConfig{denv.NewDevConfig(denv.BuildTypeStaticLibrary, buildConfig)},
			Dependencies: denv.NewDevProjectList(),
		}
	}
	a := newProject("a")
	b := newProject("b")
	a.Dependencies.Add(b)
	b.Dependencies.Add(a)
	pkg := workspaceTestPackage(map[string]*denv.DevProject{"a": a, "b": b})

	_, err := buildWorkspace(pkg, WorkspaceOptions{
		OutputDir:   t.TempDir(),
		BuildTarget: denv.BuildTargetWindowsX64,
		ResolveVars: func(_ *denv.DevProject, _ denv.BuildTarget, _ denv.BuildConfig, _ *corepkg.Vars) {},
	})
	if err == nil || !strings.Contains(err.Error(), "a -> b -> a") {
		t.Fatalf("buildWorkspace() error = %v, want dependency cycle", err)
	}
}

func TestBuildWorkspaceRequiresExactDependencyConfig(t *testing.T) {
	dependencyConfig := denv.NewDevConfig(denv.BuildTypeStaticLibrary, denv.NewDebugDevConfig())
	dependencyConfig.Defines.Add("DEPENDENCY_DEV")
	dependency := &denv.DevProject{
		Name:         "dependency",
		RepoName:     "dependency",
		BuildType:    denv.BuildTypeStaticLibrary,
		BuildTargets: denv.BuildTargetWindowsX64,
		Configs:      []*denv.DevConfig{dependencyConfig},
		Dependencies: denv.NewDevProjectList(),
	}
	app := &denv.DevProject{
		Name:         "app",
		RepoName:     "app",
		BuildType:    denv.BuildTypeApplication,
		BuildTargets: denv.BuildTargetWindowsX64,
		Configs:      []*denv.DevConfig{denv.NewDevConfig(denv.BuildTypeApplication, denv.NewDebugConfig())},
		Dependencies: denv.NewDevProjectList(),
	}
	app.Dependencies.Add(dependency)
	pkg := workspaceTestPackage(map[string]*denv.DevProject{"dependency": dependency})
	pkg.MainApps[app.Name] = app

	workspace, err := buildWorkspace(pkg, WorkspaceOptions{
		OutputDir:   t.TempDir(),
		BuildTarget: denv.BuildTargetWindowsX64,
		ResolveVars: func(_ *denv.DevProject, _ denv.BuildTarget, _ denv.BuildConfig, _ *corepkg.Vars) {},
	})
	if err != nil {
		t.Fatalf("buildWorkspace() error = %v", err)
	}
	for _, project := range workspace.Projects {
		if project.Definition.Name == app.Name && containsString(project.Configurations[0].OptionsVs.Defines, "DEPENDENCY_DEV") {
			t.Errorf("Debug configuration inherited non-exact DebugDev dependency configuration")
		}
	}
}

func workspaceTestPackage(libraries map[string]*denv.DevProject) *denv.Package {
	return &denv.Package{
		RepoName:  "example",
		MainApps:  map[string]*denv.DevProject{},
		MainLibs:  libraries,
		Library:   map[string]*denv.DevProject{},
		Unittests: map[string]*denv.DevProject{},
		TestLibs:  map[string]*denv.DevProject{},
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
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
