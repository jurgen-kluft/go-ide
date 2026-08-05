package msdev

import (
	"fmt"
	"path/filepath"
	"sort"

	corepkg "github.com/jurgen-kluft/go-core"
	"github.com/jurgen-kluft/go-ide/denv"
	"github.com/jurgen-kluft/go-ide/graph"
)

type Workspace = graph.Workspace
type WorkspaceOptions = graph.WorkspaceOptions
type VarsResolver = graph.VarsResolver

func BuildWorkspace(pkg *denv.Package, options WorkspaceOptions) (*Workspace, error) {
	return graph.BuildWorkspace(pkg, options, graph.WorkspaceBuilderBackend{
		Name: "Visual Studio",
		ValidateTarget: func(target denv.BuildTarget) error {
			if !target.Windows() || target.Arch() == denv.BuildTargetArchNone {
				return fmt.Errorf("Visual Studio generation requires one Windows target architecture")
			}
			return nil
		},
		InitializeWorkspace: func(workspace *graph.Workspace, options graph.WorkspaceOptions) {
			visualStudioOptions := options.VisualStudio
			workspace.OptionsVs = &visualStudioOptions
		},
		BuildProjectConfigurations: buildProjectConfigurations,
		ResolveConfiguration:       resolveProjectConfiguration,
	})
}

func GeneratePackage(pkg *denv.Package, options WorkspaceOptions) error {
	workspace, err := BuildWorkspace(pkg, options)
	if err != nil {
		return err
	}
	return Generate(*workspace)
}

func buildProjectConfigurations(project *denv.DevProject, options WorkspaceOptions) ([]*ProjectConfiguration, error) {
	configs := make([]denv.BuildConfig, 0, len(project.Configs))
	for _, devConfig := range project.Configs {
		if graph.ConfigSelected(devConfig.BuildConfig, options.BuildConfigs) {
			configs = append(configs, devConfig.BuildConfig)
		}
	}
	sort.Slice(configs, func(i, j int) bool { return configs[i].VisualStudio() < configs[j].VisualStudio() })

	result := make([]*ProjectConfiguration, 0, len(configs))
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
		result = append(result, &ProjectConfiguration{
			BuildTarget:     options.BuildTarget,
			BuildConfig:     buildConfig,
			OptionsVs:       configuration,
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

func resolveProjectConfiguration(_ *graph.Project, local *ProjectConfiguration, dependencies []*ProjectConfiguration) error {
	options := VsConfiguration{}
	for _, dependency := range dependencies {
		options = mergeVsConfiguration(options, *dependency.OptionsVs)
	}
	merged := mergeVsConfiguration(options, *local.OptionsVs)
	local.OptionsVs = &merged
	return nil
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
