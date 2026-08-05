package xcode

import (
	"fmt"
	"path/filepath"
	"sort"

	corepkg "github.com/jurgen-kluft/go-core"
	"github.com/jurgen-kluft/go-ide/denv"
	"github.com/jurgen-kluft/go-ide/graph"
)

type CppStdType = graph.CppStdType

type Workspace = graph.Workspace
type WorkspaceOptions = graph.WorkspaceOptions
type VarsResolver = graph.VarsResolver
type ProjectSettings = graph.ProjectSettings
type Project = graph.Project
type ProjectList = graph.ProjectList
type ProjectGroup = graph.ProjectGroup
type ProjectGroups = graph.ProjectGroups

const (
	CppStdDefault = graph.CppStdDefault
	CppStd11      = graph.CppStd11
	CppStd14      = graph.CppStd14
	CppStd17      = graph.CppStd17
	CppStd20      = graph.CppStd20
	CppStdLatest  = graph.CppStdLatest
)

func GeneratePackage(pkg *denv.Package, options WorkspaceOptions) error {
	workspace, err := buildWorkspace(pkg, options)
	if err != nil {
		return err
	}
	return Generate(workspace)
}

func buildWorkspace(pkg *denv.Package, options WorkspaceOptions) (*Workspace, error) {
	requestedCppStd := options.OptionsXcode.CppStd
	var workspace *Workspace
	return graph.BuildWorkspace(pkg, options, graph.WorkspaceBuilderBackend{
		Name: "Xcode",
		ValidateTarget: func(target denv.BuildTarget) error {
			if !target.Mac() && !target.AppleiOS() {
				return fmt.Errorf("Xcode generation requires a macOS or iOS target")
			}
			return nil
		},
		InitializeWorkspace: func(sharedWorkspace *graph.Workspace, options graph.WorkspaceOptions) {
			workspace = sharedWorkspace
			xcodeOptions := options.OptionsXcode
			if xcodeOptions.CppStd == graph.CppStdDefault {
				xcodeOptions.CppStd = graph.CppStd17
			}
			workspace.OptionsXcode = &xcodeOptions
		},
		BuildProjectConfigurations: func(project *denv.DevProject, options graph.WorkspaceOptions) ([]*graph.ProjectConfiguration, error) {
			configurations, cppStd, err := buildProjectConfigurations(project, options)
			if err == nil && requestedCppStd == graph.CppStdDefault && cppStd != graph.CppStdDefault {
				workspace.OptionsXcode.CppStd = cppStd
			}
			return configurations, err
		},
		InitializeProject: func(project *graph.Project, options graph.WorkspaceOptions) (string, error) {
			initializeXcodeProject(project)
			if err := project.CollectFiles(project.Definition, options.BuildTarget); err != nil {
				return "", fmt.Errorf("collect Xcode project %s files: %w", project.Name, err)
			}
			if project.TypeIsExe() {
				return "applications", nil
			}
			return "libraries", nil
		},
		ResolveConfiguration: resolveProjectConfiguration,
	})
}

func newProject(name string, buildType denv.BuildType, projectDir, outputDir string) *Project {
	project := &graph.Project{
		Name:            name,
		BuildType:       buildType,
		ProjectAbsPath:  projectDir,
		GenerateAbsPath: outputDir,
	}
	initializeXcodeProject(project)
	return project
}

func newWorkspace(name, outputDir string, buildTarget denv.BuildTarget) *Workspace {
	workspace := graph.NewWorkspace(name, outputDir, buildTarget)
	workspace.OptionsXcode = &graph.WorkspaceOptionsXcode{CppStd: graph.CppStd17}
	return workspace
}

func initializeXcodeProject(project *Project) {
	settings := &graph.ProjectSettings{}
	settings.Xcode.BundleIdentifier = "$(PRODUCT_BUNDLE_IDENTIFIER)"
	project.Files = &graph.ProjectFiles{
		Groups:         []*graph.FileEntryDict{graph.NewFileEntryDict(project.ProjectAbsPath)},
		VirtualFolders: graph.NewVirtualDirectories(project.ProjectAbsPath),
	}
	project.ResourceGroups = []*graph.FileEntryDict{graph.NewFileEntryDict(project.ProjectAbsPath)}
	project.Settings = settings
	project.Dependencies = graph.NewProjectList()
	if project.Configurations == nil {
		project.Configurations = make([]*graph.ProjectConfiguration, 0)
	}
	project.Resolved = graph.NewProjectResolved()
	project.Resolved.HasOutputTarget = project.BuildType.IsExecutable() || project.BuildType.IsStaticLibrary() || project.BuildType.IsDynamicLibrary()
	project.Resolved.GenDataXcode.XcodeProj.Init(filepath.Join(project.GenerateAbsPath, project.Name+".xcodeproj"), true)
	project.Resolved.GenDataXcode.PbxProj = filepath.Join(project.Resolved.GenDataXcode.XcodeProj.Path, "project.pbxproj")
}

func buildProjectConfigurations(project *denv.DevProject, options WorkspaceOptions) ([]*graph.ProjectConfiguration, CppStdType, error) {
	configs := make([]denv.BuildConfig, 0, len(project.Configs))
	for _, devConfig := range project.Configs {
		if graph.ConfigSelected(devConfig.BuildConfig, options.BuildConfigs) {
			configs = append(configs, devConfig.BuildConfig)
		}
	}
	sort.Slice(configs, func(i, j int) bool { return configs[i].String() < configs[j].String() })
	result := make([]*graph.ProjectConfiguration, 0, len(configs))
	cppStd := CppStdDefault
	for _, buildConfig := range configs {
		vars := corepkg.NewVars(corepkg.VarsFormatCurlyBraces)
		options.ResolveVars(project, options.BuildTarget, buildConfig, vars)
		configuration, parsedCppStd, err := NewConfigurationFromVars(vars, options.BuildTarget, buildConfig)
		if err != nil {
			return nil, CppStdDefault, fmt.Errorf("create Xcode configuration for %s/%s: %w", project.Name, buildConfig.String(), err)
		}
		if parsedCppStd != CppStdDefault {
			cppStd = parsedCppStd
		}
		if options.OSTargetVersion != "" {
			if options.BuildTarget.AppleiOS() {
				configuration.XcodeSettings.Set("IPHONEOS_DEPLOYMENT_TARGET", options.OSTargetVersion)
			} else {
				configuration.XcodeSettings.Set("MACOSX_DEPLOYMENT_TARGET", options.OSTargetVersion)
			}
		}
		for _, devConfig := range project.Configs {
			if devConfig.BuildConfig.IsEqual(buildConfig) {
				mergeXcodeDevConfig(configuration, devConfig)
				break
			}
		}
		result = append(result, &graph.ProjectConfiguration{
			BuildTarget:  options.BuildTarget,
			BuildConfig:  buildConfig,
			OptionsXcode: configuration,
		})
	}
	return result, cppStd, nil
}

func resolveProjectConfiguration(project *graph.Project, local *graph.ProjectConfiguration, dependencies []*graph.ProjectConfiguration) error {
	xcodeDependencies := make([]*graph.XcodeConfiguration, 0, len(dependencies))
	for _, dependency := range dependencies {
		xcodeDependencies = append(xcodeDependencies, dependency.OptionsXcode)
	}
	resolved := local.OptionsXcode.Resolve(project, project.GenerateAbsPath, xcodeDependencies...)
	local.OptionsXcode = resolved
	local.OutputTarget = resolved.Resolved.OutputTarget
	local.BuildTmpDir = resolved.Resolved.BuildTmpDir
	return nil
}
