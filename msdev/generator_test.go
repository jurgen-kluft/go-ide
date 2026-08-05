package msdev

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jurgen-kluft/go-ide/denv"
	"github.com/jurgen-kluft/go-ide/graph"
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
			denv.NewPinnedGlobPath(root, "hello", "source", "**/*.cpp"),
			denv.NewPinnedGlobPath(root, "hello", "source", "**/*.h"),
		},
		Configs:      []*denv.DevConfig{debug},
		Dependencies: denv.NewDevProjectList(),
	}

	outputDir := filepath.Join(root, "ide", "vs2022")
	workspace := Workspace{
		Name:           "hello-workspace",
		OutputDir:      outputDir,
		BuildTarget:    denv.BuildTargetWindowsX64,
		StartupProject: "hello",
		Projects: []*Project{{
			Definition: project,
			Configurations: []*ProjectConfiguration{
				{
					BuildConfig:     denv.NewDebugDevConfig(),
					OutputDir:       filepath.Join(root, "bin", "debug"),
					IntermediateDir: filepath.Join(root, "obj", "debug"),
					TargetName:      "hello",
					TargetExtension: ".exe",
					OptionsVs: &VsConfiguration{
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
					OptionsVs: &VsConfiguration{
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
	assertContains(t, filtersXml, `<ClCompile Include="..\..\hello\source\main.cpp"></ClCompile>`)
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

func TestGenerateProjectAndFiltersIncludeHeaders(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "ide", "vs2022")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	workspace := Workspace{OutputDir: outputDir}
	project := &Project{Definition: &denv.DevProject{Name: "hello"}}
	group := graph.NewFileEntryDict(filepath.Join(root, "hello"))
	header := group.Add(filepath.Join("include", "public", "hello.h"))
	virtualFolders := graph.NewVirtualDirectories(group.Path)
	virtualFolders.AddFile(header)
	virtualFolders.Sort()
	virtualFolders.AssignUUIDs("hello", []*graph.FileEntryDict{group})
	headers := &graph.ProjectFiles{Groups: []*graph.FileEntryDict{group}, VirtualFolders: virtualFolders}

	if err := generateProject(workspace, project, headers, map[string]string{"hello": stableGuid("hello")}); err != nil {
		t.Fatalf("generateProject() error = %v", err)
	}
	if err := generateFilters(workspace, project, headers); err != nil {
		t.Fatalf("generateFilters() error = %v", err)
	}

	projectXml := readTestFile(t, filepath.Join(outputDir, "hello.vcxproj"))
	filtersXml := readTestFile(t, filepath.Join(outputDir, "hello.vcxproj.filters"))
	assertValidXml(t, projectXml)
	assertValidXml(t, filtersXml)
	assertContains(t, projectXml, `<ClInclude Include="..\..\hello\include\public\hello.h"></ClInclude>`)
	assertContains(t, filtersXml, `<Filter Include="include">`)
	assertContains(t, filtersXml, `<Filter Include="include\public">`)
	assertContains(t, filtersXml, `<UniqueIdentifier>`+header.Parent.UUID.ForVisualStudio()+`</UniqueIdentifier>`)
	assertContains(t, filtersXml, `<ClInclude Include="..\..\hello\include\public\hello.h">`)
	assertContains(t, filtersXml, `<Filter>include\public</Filter>`)
}

func TestWriteXmlPreservesLiteralApostrophes(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "apostrophes.xml")
	condition := `'$(Configuration)'=='Debug' && 1 < 2`
	if err := writeXml(filename, func(document *xmlDocument) error {
		document.start("Project", attr("Condition", condition))
		document.element("Value", "it's <valid> & escaped")
		document.element("Literal", "&#39;")
		document.end("Project")
		return nil
	}); err != nil {
		t.Fatalf("writeXml() error = %v", err)
	}

	generated := readTestFile(t, filename)
	assertValidXml(t, generated)
	assertContains(t, generated, `Condition="'$(Configuration)'=='Debug' &amp;&amp; 1 &lt; 2"`)
	assertContains(t, generated, `<Value>it's &lt;valid&gt; &amp; escaped</Value>`)
	assertContains(t, generated, `<Literal>&amp;#39;</Literal>`)
	if strings.Contains(generated, "&#39;") {
		t.Errorf("generated XML contains numeric apostrophe references\n%s", generated)
	}
}

func TestGenerateRejectsProjectWithoutConfigurations(t *testing.T) {
	project := &denv.DevProject{Name: "empty", Dependencies: denv.NewDevProjectList()}
	err := Generate(Workspace{
		Name:      "invalid",
		OutputDir: t.TempDir(),
		Projects:  []*Project{{Definition: project}},
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
	actual = strings.ReplaceAll(actual, "\r\n", "\n")
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
	expected = strings.ReplaceAll(expected, "\r\n", "\n")
	if actual != expected {
		t.Errorf("generated output %s differs from %s", actual, expected)
	}
}
