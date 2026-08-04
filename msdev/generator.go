package msdev

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strings"

	corepkg "github.com/jurgen-kluft/gcore"
	"github.com/jurgen-kluft/gide/denv"
	"github.com/jurgen-kluft/gide/graph"
)

const cppProjectTypeGuid = "{8BC9CEB8-8B4A-11D0-8D11-00A0C91BC942}"

type ProjectConfiguration = graph.ProjectConfiguration
type Project = graph.Project

func Generate(workspace Workspace) error {
	if err := validateWorkspace(&workspace); err != nil {
		return err
	}

	if err := os.MkdirAll(workspace.OutputDir, 0o755); err != nil {
		return fmt.Errorf("create Visual Studio output directory: %w", err)
	}

	projects := append([]*Project(nil), workspace.Projects...)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Definition.Name < projects[j].Definition.Name
	})

	projectGuids := make(map[string]string, len(projects))
	for _, project := range projects {
		projectGuids[project.Definition.Name] = stableGuid(project.Definition.RepoName + "/" + project.Definition.Name)
	}

	for _, project := range projects {
		files := project.Files
		if files == nil {
			var err error
			files, err = graph.CollectProjectFiles(project.Definition, workspace.BuildTarget)
			if err != nil {
				return fmt.Errorf("collect project files for %s: %w", project.Definition.Name, err)
			}
			project.Files = files
		}
		if err := generateProject(workspace, project, files, projectGuids); err != nil {
			return err
		}
		if err := generateFilters(workspace, project, files); err != nil {
			return err
		}
	}

	return generateSolution(workspace, projects, projectGuids)
}

func validateWorkspace(workspace *Workspace) error {
	if workspace.Name == "" {
		return errors.New("Visual Studio workspace name is required")
	}
	if workspace.OutputDir == "" {
		return errors.New("Visual Studio output directory is required")
	}
	if workspace.OptionsVs == nil {
		workspace.OptionsVs = &graph.WorkspaceOptionsVs{}
	}
	if workspace.OptionsVs.PlatformToolset == "" {
		workspace.OptionsVs.PlatformToolset = "v143"
	}
	if workspace.OSTargetVersion == "" {
		workspace.OSTargetVersion = "10.0"
	}
	if len(workspace.Projects) == 0 {
		return errors.New("Visual Studio workspace requires at least one project")
	}

	names := make(map[string]bool, len(workspace.Projects))
	for _, project := range workspace.Projects {
		if project.Definition == nil {
			return errors.New("Visual Studio project definition is nil")
		}
		if project.Definition.Name == "" {
			return errors.New("Visual Studio project name is required")
		}
		if names[project.Definition.Name] {
			return fmt.Errorf("duplicate Visual Studio project %q", project.Definition.Name)
		}
		names[project.Definition.Name] = true
		if len(project.Configurations) == 0 {
			return fmt.Errorf("Visual Studio project %q requires at least one configuration", project.Definition.Name)
		}
	}
	if workspace.StartupProject != "" && !names[workspace.StartupProject] {
		return fmt.Errorf("Visual Studio startup project %q is not in the workspace", workspace.StartupProject)
	}
	return nil
}

func generateProject(workspace Workspace, project *Project, files *graph.ProjectFiles, projectGuids map[string]string) error {
	filename := filepath.Join(workspace.OutputDir, project.Definition.Name+".vcxproj")
	workspaceOptions := visualStudioWorkspaceOptions(workspace)
	return writeXml(filename, func(document *xmlDocument) error {
		document.start("Project", attr("DefaultTargets", "Build"), attr("ToolsVersion", "17.0"), attr("xmlns", "http://schemas.microsoft.com/developer/msbuild/2003"))

		document.start("ItemGroup", attr("Label", "ProjectConfigurations"))
		for _, config := range sortedConfigurations(project.Configurations) {
			name, platform := configurationIdentity(config)
			document.start("ProjectConfiguration", attr("Include", name+"|"+platform))
			document.element("Configuration", name)
			document.element("Platform", platform)
			document.end("ProjectConfiguration")
		}
		document.end("ItemGroup")

		document.start("PropertyGroup", attr("Label", "Globals"))
		document.element("ProjectGuid", projectGuids[project.Definition.Name])
		document.element("Keyword", "Win32Proj")
		document.element("RootNamespace", project.Definition.Name)
		document.element("WindowsTargetPlatformVersion", workspace.OSTargetVersion)
		document.end("PropertyGroup")
		document.empty("Import", attr("Project", `$(VCTargetsPath)\Microsoft.Cpp.Default.props`))

		for _, config := range sortedConfigurations(project.Configurations) {
			name, platform := configurationIdentity(config)
			condition := configurationCondition(name, platform)
			document.start("PropertyGroup", attr("Condition", condition), attr("Label", "Configuration"))
			document.element("ConfigurationType", configurationType(project.Definition.BuildType))
			document.element("UseDebugLibraries", boolText(config.BuildConfig.IsDebug()))
			document.element("PlatformToolset", workspaceOptions.PlatformToolset)
			document.element("CharacterSet", "Unicode")
			document.end("PropertyGroup")
		}

		document.empty("Import", attr("Project", `$(VCTargetsPath)\Microsoft.Cpp.props`))
		document.empty("PropertyGroup", attr("Label", "UserMacros"))

		for _, config := range sortedConfigurations(project.Configurations) {
			name, platform := configurationIdentity(config)
			condition := configurationCondition(name, platform)
			effective := effectiveOptions(project.Definition, config)

			document.start("PropertyGroup", attr("Condition", condition))
			if config.OutputDir != "" {
				document.element("OutDir", trailingWindowsSlash(config.OutputDir))
			}
			if config.IntermediateDir != "" {
				document.element("IntDir", trailingWindowsSlash(config.IntermediateDir))
			}
			if config.TargetName != "" {
				document.element("TargetName", config.TargetName)
			}
			if config.TargetExtension != "" {
				document.element("TargetExt", config.TargetExtension)
			}
			if effective.LinkOptions.Has(VsLinkOptionDisableIncrementalLinking) {
				document.element("LinkIncremental", "false")
			}
			document.end("PropertyGroup")

			document.start("ItemDefinitionGroup", attr("Condition", condition))
			writeCompileOptions(document, effective)
			if project.Definition.BuildType.IsStaticLibrary() {
				document.empty("Lib")
			} else {
				writeLinkOptions(document, effective)
			}
			document.end("ItemDefinitionGroup")
		}

		if files != nil && len(files.Groups) > 0 {
			document.start("ItemGroup")
			for _, group := range files.Groups {
				for _, file := range group.Values {
					relPath, err := group.GetRelativePath(file, workspace.OutputDir)
					if err != nil {
						return err
					}
					document.empty(visualStudioFileTag(file), attr("Include", windowsPath(relPath)))
				}
			}
			document.end("ItemGroup")
		}

		var dependencies []*denv.DevProject
		if project.Definition.Dependencies != nil {
			dependencies = append(dependencies, project.Definition.Dependencies.Values...)
			sort.Slice(dependencies, func(i, j int) bool { return dependencies[i].Name < dependencies[j].Name })
		}
		if len(dependencies) > 0 {
			document.start("ItemGroup")
			for _, dependency := range dependencies {
				guid, ok := projectGuids[dependency.Name]
				if !ok {
					continue
				}
				document.start("ProjectReference", attr("Include", dependency.Name+".vcxproj"))
				document.element("Project", guid)
				document.end("ProjectReference")
			}
			document.end("ItemGroup")
		}

		document.empty("Import", attr("Project", `$(VCTargetsPath)\Microsoft.Cpp.targets`))
		document.end("Project")
		return nil
	})
}

func visualStudioWorkspaceOptions(workspace Workspace) graph.WorkspaceOptionsVs {
	options := graph.WorkspaceOptionsVs{PlatformToolset: "v143"}
	if workspace.OptionsVs != nil {
		if workspace.OptionsVs.PlatformToolset != "" {
			options.PlatformToolset = workspace.OptionsVs.PlatformToolset
		}
	}
	return options
}

func writeCompileOptions(document *xmlDocument, options VsConfiguration) {
	document.start("ClCompile")
	writeOptional(document, "LanguageStandard_C", cStandard(options.CStandard))
	writeOptional(document, "LanguageStandard", cppStandard(options.CppStandard))
	writeOptional(document, "Optimization", optimization(options.Optimization))
	writeOptional(document, "WarningLevel", warningLevel(options.WarningLevel))
	writeOptional(document, "DebugInformationFormat", debugInformation(options.DebugInformation))
	writeOptional(document, "ExceptionHandling", exceptionHandling(options.ExceptionHandling))
	writeOptional(document, "FloatingPointModel", floatingPoint(options.FloatingPoint))
	writeOptional(document, "InlineFunctionExpansion", inlineExpansion(options.InlineExpansion))
	writeInheritedList(document, "PreprocessorDefinitions", options.Defines)
	writeInheritedList(document, "AdditionalIncludeDirectories", options.IncludePaths)
	writeBoolOption(document, "MultiProcessorCompilation", options.CompileOptions, VsCompileOptionMultiProcessor)
	writeBoolOption(document, "TreatWarningAsError", options.CompileOptions, VsCompileOptionTreatWarningsAsErrors)
	writeBoolOption(document, "IntrinsicFunctions", options.CompileOptions, VsCompileOptionIntrinsicFunctions)
	writeBoolOption(document, "SuppressStartupBanner", options.CompileOptions, VsCompileOptionSuppressStartupBanner)
	writeBoolOption(document, "UseFullPaths", options.CompileOptions, VsCompileOptionFullPathDiagnostics)
	if options.CompileOptions.Has(VsCompileOptionColumnDiagnostics) {
		document.element("DiagnosticsFormat", "Column")
	}
	document.end("ClCompile")
}

func writeLinkOptions(document *xmlDocument, options VsConfiguration) {
	document.start("Link")
	writeOptional(document, "SubSystem", subsystem(options.Subsystem))
	writeOptional(document, "LinkErrorReporting", linkErrorReporting(options.LinkErrorReporting))
	writeOptional(document, "TargetMachine", machine(options.Machine))
	writeInheritedList(document, "AdditionalLibraryDirectories", options.LibraryPaths)
	writeInheritedList(document, "AdditionalDependencies", options.Libraries)
	writeLinkBoolOption(document, "GenerateDebugInformation", options.LinkOptions, VsLinkOptionGenerateDebugInformation)
	writeLinkBoolOption(document, "RandomizedBaseAddress", options.LinkOptions, VsLinkOptionDynamicBase)
	writeLinkBoolOption(document, "DataExecutionPrevention", options.LinkOptions, VsLinkOptionDataExecutionPrevention)
	writeLinkBoolOption(document, "SuppressStartupBanner", options.LinkOptions, VsLinkOptionSuppressStartupBanner)
	writeLinkBoolOption(document, "EnableCOMDATFolding", options.LinkOptions, VsLinkOptionComdatFolding)
	writeLinkBoolOption(document, "LinkTimeCodeGeneration", options.LinkOptions, VsLinkOptionLinkTimeCodeGeneration)
	if options.LinkOptions.Has(VsLinkOptionOptimizeReferences) {
		document.element("OptimizeReferences", "true")
	}
	document.end("Link")
}

func generateFilters(workspace Workspace, project *Project, files *graph.ProjectFiles) error {
	filename := filepath.Join(workspace.OutputDir, project.Definition.Name+".vcxproj.filters")
	return writeXml(filename, func(document *xmlDocument) error {
		document.start("Project", attr("ToolsVersion", "4.0"), attr("xmlns", "http://schemas.microsoft.com/developer/msbuild/2003"))

		if len(files.VirtualFolders.Folders) > 1 {
			document.start("ItemGroup")
			for _, folder := range files.VirtualFolders.Folders {
				if folder == files.VirtualFolders.Root {
					continue
				}
				document.start("Filter", attr("Include", windowsPath(folder.Path)))
				document.element("UniqueIdentifier", folder.UUID.ForVisualStudio())
				document.end("Filter")
			}
			document.end("ItemGroup")
		}

		if len(files.Groups) > 0 {
			document.start("ItemGroup")
			for _, group := range files.Groups {
				for _, file := range group.Values {
					relPath, err := group.GetRelativePath(file, workspace.OutputDir)
					if err != nil {
						return err
					}
					tag := visualStudioFileTag(file)
					document.start(tag, attr("Include", windowsPath(relPath)))
					if file.Parent != files.VirtualFolders.Root {
						document.element("Filter", windowsPath(file.Parent.Path))
					}
					document.end(tag)
				}
			}
			document.end("ItemGroup")
		}

		document.end("Project")
		return nil
	})
}

func visualStudioFileTag(file *graph.FileEntry) string {
	switch file.Type {
	case graph.FileTypeCSource, graph.FileTypeCppSource, graph.FileTypeIxx, graph.FileTypeMxx:
		return "ClCompile"
	case graph.FileTypeCuHeader, graph.FileTypeCuSource:
		return "CudaCompile"
	default:
		return "ClInclude"
	}
}

func generateSolution(workspace Workspace, projects []*Project, projectGuids map[string]string) error {
	var buffer bytes.Buffer
	buffer.WriteString("Microsoft Visual Studio Solution File, Format Version 12.00\r\n")
	buffer.WriteString("# Visual Studio Version 17\r\n")
	buffer.WriteString("VisualStudioVersion = 17.0.31314.256\r\n")
	buffer.WriteString("MinimumVisualStudioVersion = 10.0.40219.1\r\n")

	orderedProjects := append([]*Project(nil), projects...)
	if workspace.StartupProject != "" {
		sort.SliceStable(orderedProjects, func(i, j int) bool {
			return orderedProjects[i].Definition.Name == workspace.StartupProject
		})
	}
	for _, project := range orderedProjects {
		name := project.Definition.Name
		fmt.Fprintf(&buffer, "Project(\"%s\") = \"%s\", \"%s.vcxproj\", \"%s\"\r\n", cppProjectTypeGuid, name, name, projectGuids[name])
		buffer.WriteString("EndProject\r\n")
	}

	identities := solutionConfigurations(projects)
	buffer.WriteString("Global\r\n")
	buffer.WriteString("\tGlobalSection(SolutionConfigurationPlatforms) = preSolution\r\n")
	for _, identity := range identities {
		fmt.Fprintf(&buffer, "\t\t%s = %s\r\n", identity, identity)
	}
	buffer.WriteString("\tEndGlobalSection\r\n")
	buffer.WriteString("\tGlobalSection(ProjectConfigurationPlatforms) = postSolution\r\n")
	for _, project := range projects {
		guid := projectGuids[project.Definition.Name]
		available := make(map[string]bool)
		for _, config := range project.Configurations {
			name, platform := configurationIdentity(config)
			available[name+"|"+platform] = true
		}
		for _, identity := range identities {
			if !available[identity] {
				continue
			}
			fmt.Fprintf(&buffer, "\t\t%s.%s.ActiveCfg = %s\r\n", guid, identity, identity)
			fmt.Fprintf(&buffer, "\t\t%s.%s.Build.0 = %s\r\n", guid, identity, identity)
		}
	}
	buffer.WriteString("\tEndGlobalSection\r\n")
	buffer.WriteString("EndGlobal\r\n")

	filename := filepath.Join(workspace.OutputDir, workspace.Name+".sln")
	if err := os.WriteFile(filename, buffer.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write Visual Studio solution: %w", err)
	}
	return nil
}

func effectiveOptions(project *denv.DevProject, config *ProjectConfiguration) VsConfiguration {
	options := VsConfiguration{}
	if config.OptionsVs != nil {
		options = *config.OptionsVs
	}
	for _, devConfig := range project.Configs {
		if devConfig.BuildConfig.IsEqual(config.BuildConfig) {
			return mergeDevConfig(options, devConfig)
		}
	}
	return options
}

func uniqueStrings(groups ...[]string) []string {
	result := make([]string, 0)
	seen := make(map[string]bool)
	for _, group := range groups {
		for _, value := range group {
			if value == "" || seen[strings.ToLower(value)] {
				continue
			}
			seen[strings.ToLower(value)] = true
			result = append(result, value)
		}
	}
	return result
}

func sortedConfigurations(configs []*ProjectConfiguration) []*ProjectConfiguration {
	result := append([]*ProjectConfiguration(nil), configs...)
	sort.Slice(result, func(i, j int) bool {
		leftName, leftPlatform := configurationIdentity(result[i])
		rightName, rightPlatform := configurationIdentity(result[j])
		return leftName+"|"+leftPlatform < rightName+"|"+rightPlatform
	})
	return result
}

func solutionConfigurations(projects []*Project) []string {
	set := make(map[string]bool)
	for _, project := range projects {
		for _, config := range project.Configurations {
			name, platform := configurationIdentity(config)
			set[name+"|"+platform] = true
		}
	}
	result := make([]string, 0, len(set))
	for identity := range set {
		result = append(result, identity)
	}
	sort.Strings(result)
	return result
}

func configurationIdentity(config *ProjectConfiguration) (string, string) {
	machine := VsMachineDefault
	if config.OptionsVs != nil {
		machine = config.OptionsVs.Machine
	}
	return config.BuildConfig.VisualStudio(), platform(machine)
}

func configurationCondition(name, platform string) string {
	return "'$(Configuration)|$(Platform)'=='" + name + "|" + platform + "'"
}

func configurationType(buildType denv.BuildType) string {
	switch {
	case buildType.IsStaticLibrary(), buildType == denv.BuildTypeHeaderOnly:
		return "StaticLibrary"
	case buildType.IsDynamicLibrary():
		return "DynamicLibrary"
	default:
		return "Application"
	}
}

func stableGuid(value string) string {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(value))
	return corepkg.UUID(hash.Sum64()).ForVisualStudio()
}

func trailingWindowsSlash(path string) string {
	path = windowsPath(path)
	if !strings.HasSuffix(path, `\`) {
		path += `\`
	}
	return path
}

func windowsPath(path string) string {
	return strings.ReplaceAll(path, "/", `\`)
}

func platform(value VsMachine) string {
	switch value {
	case VsMachineX86:
		return "Win32"
	case VsMachineArm:
		return "ARM"
	case VsMachineArm64:
		return "ARM64"
	default:
		return "x64"
	}
}

func machine(value VsMachine) string {
	switch value {
	case VsMachineX86:
		return "MachineX86"
	case VsMachineArm:
		return "MachineARM"
	case VsMachineArm64:
		return "MachineARM64"
	case VsMachineX64:
		return "MachineX64"
	default:
		return ""
	}
}

func cStandard(value VsCStandard) string {
	return map[VsCStandard]string{VsCStandard11: "stdc11", VsCStandard17: "stdc17", VsCStandardLatest: "stdclatest"}[value]
}

func cppStandard(value VsCppStandard) string {
	return map[VsCppStandard]string{VsCppStandard14: "stdcpp14", VsCppStandard17: "stdcpp17", VsCppStandard20: "stdcpp20", VsCppStandardLatest: "stdcpplatest"}[value]
}

func optimization(value VsOptimization) string {
	return map[VsOptimization]string{VsOptimizationDisabled: "Disabled", VsOptimizationSize: "MinSpace", VsOptimizationSpeed: "MaxSpeed"}[value]
}

func warningLevel(value VsWarningLevel) string {
	return map[VsWarningLevel]string{VsWarningLevelDisabled: "TurnOffAllWarnings", VsWarningLevel1: "Level1", VsWarningLevel2: "Level2", VsWarningLevel3: "Level3", VsWarningLevel4: "Level4"}[value]
}

func debugInformation(value VsDebugInformation) string {
	return map[VsDebugInformation]string{VsDebugInformationProgramDatabase: "ProgramDatabase"}[value]
}

func exceptionHandling(value VsExceptionHandling) string {
	return map[VsExceptionHandling]string{VsExceptionHandlingDisabled: "false", VsExceptionHandlingCpp: "Sync"}[value]
}

func floatingPoint(value VsFloatingPointModel) string {
	return map[VsFloatingPointModel]string{VsFloatingPointPrecise: "Precise", VsFloatingPointStrict: "Strict", VsFloatingPointFast: "Fast"}[value]
}

func inlineExpansion(value VsInlineExpansion) string {
	return map[VsInlineExpansion]string{VsInlineExpansionDisabled: "Disabled", VsInlineExpansionExplicit: "OnlyExplicitInline", VsInlineExpansionAnySuitable: "AnySuitable"}[value]
}

func subsystem(value VsSubsystem) string {
	return map[VsSubsystem]string{VsSubsystemConsole: "Console", VsSubsystemWindows: "Windows"}[value]
}

func linkErrorReporting(value VsLinkErrorReporting) string {
	return map[VsLinkErrorReporting]string{
		VsLinkErrorReportingPrompt: "PromptImmediately",
		VsLinkErrorReportingQueue:  "QueueForNextLogin",
		VsLinkErrorReportingSend:   "SendErrorReport",
		VsLinkErrorReportingNone:   "NoErrorReport",
	}[value]
}

func writeOptional(document *xmlDocument, name, value string) {
	if value != "" {
		document.element(name, value)
	}
}

func writeInheritedList(document *xmlDocument, name string, values []string) {
	if len(values) > 0 {
		document.element(name, strings.Join(values, ";")+";%("+name+")")
	}
}

func writeBoolOption(document *xmlDocument, name string, options VsCompileOptions, option VsCompileOptions) {
	if options.Has(option) {
		document.element(name, "true")
	}
}

func writeLinkBoolOption(document *xmlDocument, name string, options VsLinkOptions, option VsLinkOptions) {
	if options.Has(option) {
		document.element(name, "true")
	}
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

type xmlDocument struct {
	encoder *xml.Encoder
}

func writeXml(filename string, generate func(*xmlDocument) error) error {
	var buffer bytes.Buffer
	buffer.WriteString(xml.Header)
	encoder := xml.NewEncoder(&buffer)
	encoder.Indent("", "  ")
	document := &xmlDocument{encoder: encoder}
	if err := generate(document); err != nil {
		return err
	}
	if err := encoder.Flush(); err != nil {
		return err
	}
	buffer.WriteByte('\n')
	output := bytes.ReplaceAll(buffer.Bytes(), []byte("&#39;"), []byte("'"))
	if err := os.WriteFile(filename, output, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(filename), err)
	}
	return nil
}

func attr(name, value string) xml.Attr {
	return xml.Attr{Name: xml.Name{Local: name}, Value: value}
}

func (document *xmlDocument) start(name string, attributes ...xml.Attr) {
	_ = document.encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: name}, Attr: attributes})
}

func (document *xmlDocument) end(name string) {
	_ = document.encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: name}})
}

func (document *xmlDocument) empty(name string, attributes ...xml.Attr) {
	document.start(name, attributes...)
	document.end(name)
}

func (document *xmlDocument) element(name, value string) {
	document.start(name)
	_ = document.encoder.EncodeToken(xml.CharData(value))
	document.end(name)
}
