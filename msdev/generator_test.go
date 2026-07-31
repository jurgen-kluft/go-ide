package msdev

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jurgen-kluft/gide/denv"
)

func TestGenerateVs2022Workspace(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "hello")
	sourceDir := filepath.Join(projectDir, "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "main.cpp"), []byte("int main() { return 0; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "hello.h"), []byte("#pragma once\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	debug := denv.NewDevConfig(denv.BuildTypeApplication, denv.NewDebugDevConfig())
	debug.Defines.Add("HELLO_DEBUG")
	debug.IncludeDirs = append(debug.IncludeDirs, denv.PinnedPath{Root: projectDir, Sub: "include"})
	project := &denv.DevProject{
		Name:         "hello",
		RepoName:     "hello",
		PackagePath:  root,
		BuildType:    denv.BuildTypeApplication,
		BuildTargets: denv.BuildTargetWindowsX64,
		SourceDirs: []denv.PinnedGlobPath{
			denv.NewPinnedGlobPath(root, "hello", "source", "**/*.{cpp,h}"),
		},
		Configs:      []*denv.DevConfig{debug},
		Dependencies: denv.NewDevProjectList(),
	}

	outputDir := filepath.Join(root, "ide", "vs2022")
	workspace := Workspace{
		Name:           "hello-workspace",
		OutputDir:      outputDir,
		StartupProject: "hello",
		Projects: []Project{{
			Definition: project,
			Configurations: []ProjectConfiguration{
				{
					BuildConfig:     denv.NewDebugDevConfig(),
					OutputDir:       filepath.Join(root, "bin", "debug"),
					IntermediateDir: filepath.Join(root, "obj", "debug"),
					TargetName:      "hello",
					TargetExtension: ".exe",
					Options: VsConfiguration{
						CppStandard:      VsCppStandard20,
						WarningLevel:     VsWarningLevel4,
						DebugInformation: VsDebugInformationProgramDatabase,
						Subsystem:        VsSubsystemConsole,
						Machine:          VsMachineX64,
						CompileOptions:   VsCompileOptionMultiProcessor | VsCompileOptionTreatWarningsAsErrors,
						LinkOptions:      VsLinkOptionGenerateDebugInformation | VsLinkOptionDynamicBase,
					},
				},
				{
					BuildConfig: denv.NewReleaseDevConfig(),
					Options: VsConfiguration{
						CppStandard:  VsCppStandard20,
						Optimization: VsOptimizationSpeed,
						Machine:      VsMachineX64,
					},
				},
			},
		}},
	}

	if err := Generate(workspace); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	projectPath := filepath.Join(outputDir, "hello.vcxproj")
	filtersPath := filepath.Join(outputDir, "hello.vcxproj.filters")
	solutionPath := filepath.Join(outputDir, "hello-workspace.sln")
	projectXml := readTestFile(t, projectPath)
	filtersXml := readTestFile(t, filtersPath)
	solution := readTestFile(t, solutionPath)
	assertGolden(t, "hello.vcxproj.golden", projectXml, root)
	assertGolden(t, "hello.vcxproj.filters.golden", filtersXml, root)
	assertGolden(t, "hello.sln.golden", solution, root)

	assertValidXml(t, projectXml)
	assertValidXml(t, filtersXml)
	assertContains(t, projectXml, `<LanguageStandard>stdcpp20</LanguageStandard>`)
	assertContains(t, projectXml, `<WarningLevel>Level4</WarningLevel>`)
	assertContains(t, projectXml, `<Optimization>MaxSpeed</Optimization>`)
	assertContains(t, projectXml, `<PreprocessorDefinitions>HELLO_DEBUG;%(PreprocessorDefinitions)</PreprocessorDefinitions>`)
	assertContains(t, projectXml, `<ClCompile Include="..\..\hello\source\main.cpp"></ClCompile>`)
	assertContains(t, projectXml, `<ClInclude Include="..\..\hello\source\hello.h"></ClInclude>`)
	assertContains(t, filtersXml, `<Filter>source</Filter>`)
	assertContains(t, solution, "DebugDev|x64 = DebugDev|x64")
	assertContains(t, solution, "ReleaseDev|x64 = ReleaseDev|x64")
	assertContains(t, solution, ".DebugDev|x64.Build.0 = DebugDev|x64")

	before := map[string]string{
		projectPath:  projectXml,
		filtersPath:  filtersXml,
		solutionPath: solution,
	}
	if err := Generate(workspace); err != nil {
		t.Fatalf("second Generate() error = %v", err)
	}
	for path, expected := range before {
		if actual := readTestFile(t, path); actual != expected {
			t.Errorf("Generate() produced non-deterministic output for %s", filepath.Base(path))
		}
	}
}

func TestGenerateRejectsProjectWithoutConfigurations(t *testing.T) {
	project := &denv.DevProject{Name: "empty", Dependencies: denv.NewDevProjectList()}
	err := Generate(Workspace{
		Name:      "invalid",
		OutputDir: t.TempDir(),
		Projects:  []Project{{Definition: project}},
	})
	if err == nil || !strings.Contains(err.Error(), "requires at least one configuration") {
		t.Fatalf("Generate() error = %v, want missing configuration error", err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func assertValidXml(t *testing.T, value string) {
	t.Helper()
	decoder := xml.NewDecoder(strings.NewReader(value))
	for {
		if _, err := decoder.Token(); err != nil {
			if strings.Contains(err.Error(), "EOF") {
				return
			}
			t.Fatalf("generated XML is invalid: %v", err)
		}
	}
}

func assertContains(t *testing.T, value, expected string) {
	t.Helper()
	if !strings.Contains(value, expected) {
		t.Errorf("generated output does not contain %q\n%s", expected, value)
	}
}

func assertGolden(t *testing.T, name, actual, root string) {
	t.Helper()
	actual = strings.ReplaceAll(actual, windowsPath(root), "$ROOT")
	actual = strings.ReplaceAll(actual, root, "$ROOT")
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	expected := readTestFile(t, path)
	if actual != expected {
		t.Errorf("generated output differs from %s", path)
	}
}
