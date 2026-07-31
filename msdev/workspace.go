package msdev

import (
	"fmt"
	"path/filepath"
	"sort"

	corepkg "github.com/jurgen-kluft/gcore"
	"github.com/jurgen-kluft/gide/denv"
)

type VarsResolver func(project *denv.DevProject, buildTarget denv.BuildTarget, buildConfig denv.BuildConfig, vars *corepkg.Vars)

type WorkspaceOptions struct {
	Name                         string
	OutputDir                    string
	BuildTarget                  denv.BuildTarget
	BuildConfigs                 []denv.BuildConfig
	StartupProject               string
	HardwareID                   string
	PlatformToolset              string
	WindowsTargetPlatformVersion string
	ResolveVars                  VarsResolver
}

func BuildWorkspace(pkg *denv.Package, options WorkspaceOptions) (*Workspace, error) {
	if pkg == nil {
		return nil, fmt.Errorf("cannot build Visual Studio workspace from nil package")
	}
	if options.Name == "" {
		options.Name = pkg.RepoName
	}
	if options.OutputDir == "" {
		return nil, fmt.Errorf("Visual Studio output directory is required")
	}
	if !options.BuildTarget.Windows() || options.BuildTarget.Arch() == denv.BuildTargetArchNone {
		return nil, fmt.Errorf("Visual Studio generation requires one Windows target architecture")
	}
	if options.ResolveVars == nil {
		options.ResolveVars = func(_ *denv.DevProject, buildTarget denv.BuildTarget, buildConfig denv.BuildConfig, vars *corepkg.Vars) {
			pkg.GetVars(buildTarget, buildConfig, options.HardwareID, vars)
		}
	}

	projects := pkg.AllProjects()
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	workspace := &Workspace{
		Name:                         options.Name,
		OutputDir:                    options.OutputDir,
		PlatformToolset:              options.PlatformToolset,
		WindowsTargetPlatformVersion: options.WindowsTargetPlatformVersion,
		StartupProject:               options.StartupProject,
		Projects:                     make([]Project, 0, len(projects)),
	}

	for _, project := range projects {
		if !project.BuildTargets.HasOverlap(options.BuildTarget) {
			continue
		}

		configurations, err := buildProjectConfigurations(project, options)
		if err != nil {
			return nil, err
		}
		if len(configurations) == 0 {
			continue
		}
		workspace.Projects = append(workspace.Projects, Project{
			Definition:     project,
			Configurations: configurations,
		})
	}

	if len(workspace.Projects) == 0 {
		return nil, fmt.Errorf("package %q has no projects matching the requested Windows target and configurations", pkg.RepoName)
	}
	if workspace.StartupProject == "" {
		workspace.StartupProject = defaultStartupProject(workspace.Projects)
	}
	return workspace, nil
}

func GeneratePackage(pkg *denv.Package, options WorkspaceOptions) error {
	workspace, err := BuildWorkspace(pkg, options)
	if err != nil {
		return err
	}
	return Generate(*workspace)
}

func buildProjectConfigurations(project *denv.DevProject, options WorkspaceOptions) ([]ProjectConfiguration, error) {
	configs := make([]denv.BuildConfig, 0, len(project.Configs))
	for _, devConfig := range project.Configs {
		if configSelected(devConfig.BuildConfig, options.BuildConfigs) {
			configs = append(configs, devConfig.BuildConfig)
		}
	}
	sort.Slice(configs, func(i, j int) bool { return configs[i].VisualStudio() < configs[j].VisualStudio() })

	result := make([]ProjectConfiguration, 0, len(configs))
	for _, buildConfig := range configs {
		vars := corepkg.NewVars(corepkg.VarsFormatCurlyBraces)
		options.ResolveVars(project, options.BuildTarget, buildConfig, vars)
		configuration, err := NewConfigurationFromVars(vars)
		if err != nil {
			return nil, fmt.Errorf("create Visual Studio configuration for %s/%s: %w", project.Name, buildConfig.String(), err)
		}
		if configuration.Machine == VsMachineDefault {
			configuration.Machine = machineFromTarget(options.BuildTarget)
		}

		configName := buildConfig.VisualStudio()
		result = append(result, ProjectConfiguration{
			BuildConfig:     buildConfig,
			Options:         *configuration,
			OutputDir:       filepath.Join(options.OutputDir, "bin", configName),
			IntermediateDir: filepath.Join(options.OutputDir, "obj", project.Name, configName),
			TargetName:      project.Name,
			TargetExtension: targetExtension(project.BuildType),
		})
	}
	return result, nil
}

func configSelected(config denv.BuildConfig, requested []denv.BuildConfig) bool {
	if len(requested) == 0 {
		return true
	}
	for _, candidate := range requested {
		if candidate.Contains(config) || config.Contains(candidate) {
			return true
		}
	}
	return false
}

func machineFromTarget(target denv.BuildTarget) VsMachine {
	switch {
	case target.X86():
		return VsMachineX86
	case target.Arm32():
		return VsMachineArm
	case target.Arm64():
		return VsMachineArm64
	default:
		return VsMachineX64
	}
}

func targetExtension(buildType denv.BuildType) string {
	switch {
	case buildType.IsStaticLibrary(), buildType == denv.BuildTypeHeaderOnly:
		return ".lib"
	case buildType.IsDynamicLibrary():
		return ".dll"
	default:
		return ".exe"
	}
}

func defaultStartupProject(projects []Project) string {
	for _, project := range projects {
		if project.Definition.BuildType.IsExecutable() {
			return project.Definition.Name
		}
	}
	return projects[0].Definition.Name
}
