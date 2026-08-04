package msdev

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

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
	if err := resolveProjectConfigurations(workspace.Projects); err != nil {
		return nil, err
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
		for _, devConfig := range project.Configs {
			if devConfig.BuildConfig.IsEqual(buildConfig) {
				*configuration = mergeDevConfig(*configuration, devConfig)
				break
			}
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

func mergeDevConfig(options VsConfiguration, config *denv.DevConfig) VsConfiguration {
	if config.Defines != nil {
		options.Defines = uniqueStrings(options.Defines, config.Defines.Values)
	}
	options.IncludePaths = uniqueStrings(options.IncludePaths, config.GetIncludeDirs())
	libPaths := make([]string, 0, len(config.LibPaths))
	for _, libPath := range config.LibPaths {
		libPaths = append(libPaths, libPath.String())
	}
	options.LibraryPaths = uniqueStrings(options.LibraryPaths, libPaths)
	options.Libraries = uniqueStrings(options.Libraries, config.Libs)
	return options
}

func resolveProjectConfigurations(projects []Project) error {
	byName := make(map[string]*Project, len(projects))
	localConfigurations := make(map[string][]ProjectConfiguration, len(projects))
	for i := range projects {
		name := projects[i].Definition.Name
		byName[name] = &projects[i]
		localConfigurations[name] = append([]ProjectConfiguration(nil), projects[i].Configurations...)
	}

	for i := range projects {
		dependencies, err := resolvedDependencyOrder(projects[i].Definition, byName)
		if err != nil {
			return err
		}
		resolved := make([]ProjectConfiguration, 0, len(projects[i].Configurations))
		for _, local := range localConfigurations[projects[i].Definition.Name] {
			options := VsConfiguration{}
			for _, dependency := range dependencies {
				if config, ok := findProjectConfiguration(localConfigurations[dependency.Definition.Name], local.BuildConfig); ok {
					options = mergeVsConfiguration(options, config.Options)
				}
			}
			local.Options = mergeVsConfiguration(options, local.Options)
			resolved = append(resolved, local)
		}
		projects[i].Configurations = resolved
	}
	return nil
}

func resolvedDependencyOrder(project *denv.DevProject, projects map[string]*Project) ([]*Project, error) {
	result := make([]*Project, 0)
	state := make(map[string]uint8)
	path := make([]string, 0)

	var visit func(*denv.DevProject, bool) error
	visit = func(current *denv.DevProject, root bool) error {
		if state[current.Name] == 1 {
			cycleStart := 0
			for i, name := range path {
				if name == current.Name {
					cycleStart = i
					break
				}
			}
			cycle := append(append([]string(nil), path[cycleStart:]...), current.Name)
			return fmt.Errorf("Visual Studio project dependency cycle: %s", strings.Join(cycle, " -> "))
		}
		if state[current.Name] == 2 {
			return nil
		}
		state[current.Name] = 1
		path = append(path, current.Name)
		var dependencies []*denv.DevProject
		if current.Dependencies != nil {
			dependencies = append(dependencies, current.Dependencies.Values...)
		}
		sort.Slice(dependencies, func(i, j int) bool { return dependencies[i].Name < dependencies[j].Name })
		for _, dependency := range dependencies {
			resolved, ok := projects[dependency.Name]
			if !ok {
				continue
			}
			if err := visit(resolved.Definition, false); err != nil {
				return err
			}
		}
		path = path[:len(path)-1]
		state[current.Name] = 2
		if !root {
			result = append(result, projects[current.Name])
		}
		return nil
	}

	if err := visit(project, true); err != nil {
		return nil, err
	}
	return result, nil
}

func findProjectConfiguration(configurations []ProjectConfiguration, buildConfig denv.BuildConfig) (ProjectConfiguration, bool) {
	for _, configuration := range configurations {
		if configuration.BuildConfig.IsEqual(buildConfig) {
			return configuration, true
		}
	}
	return ProjectConfiguration{}, false
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
