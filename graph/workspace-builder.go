package graph

import (
	"fmt"
	"sort"
	"strings"

	corepkg "github.com/jurgen-kluft/go-core"
	"github.com/jurgen-kluft/go-ide/denv"
)

type VarsResolver func(project *denv.DevProject, buildTarget denv.BuildTarget, buildConfig denv.BuildConfig, vars *corepkg.Vars)

type WorkspaceBuilderBackend struct {
	Name                       string
	ValidateTarget             func(denv.BuildTarget) error
	InitializeWorkspace        func(*Workspace, WorkspaceOptions)
	BuildProjectConfigurations func(*denv.DevProject, WorkspaceOptions) ([]*ProjectConfiguration, error)
	InitializeProject          func(*Project, WorkspaceOptions) (string, error)
	ResolveConfiguration       func(*Project, *ProjectConfiguration, []*ProjectConfiguration) error
}

func BuildWorkspace(pkg *denv.Package, options WorkspaceOptions, backend WorkspaceBuilderBackend) (*Workspace, error) {
	if pkg == nil {
		return nil, fmt.Errorf("cannot build %s workspace from nil package", backend.Name)
	}
	if options.Name == "" {
		options.Name = pkg.RepoName
	}
	if options.OutputDir == "" {
		return nil, fmt.Errorf("%s output directory is required", backend.Name)
	}
	if backend.ValidateTarget == nil || backend.BuildProjectConfigurations == nil || backend.ResolveConfiguration == nil {
		return nil, fmt.Errorf("%s workspace builder is incomplete", backend.Name)
	}
	if err := backend.ValidateTarget(options.BuildTarget); err != nil {
		return nil, err
	}
	if options.ResolveVars == nil {
		options.ResolveVars = func(_ *denv.DevProject, buildTarget denv.BuildTarget, buildConfig denv.BuildConfig, vars *corepkg.Vars) {
			pkg.GetVars(buildTarget, buildConfig, "", vars)
		}
	}

	workspace := NewWorkspace(options.Name, options.OutputDir, options.BuildTarget)
	workspace.OSTargetVersion = options.OSTargetVersion
	workspace.StartupProject = options.StartupProject
	if backend.InitializeWorkspace != nil {
		backend.InitializeWorkspace(workspace, options)
	}

	projects := pkg.AllProjects()
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	for _, definition := range projects {
		if !definition.BuildTargets.HasOverlap(options.BuildTarget) {
			continue
		}
		configurations, err := backend.BuildProjectConfigurations(definition, options)
		if err != nil {
			return nil, err
		}
		if len(configurations) == 0 {
			continue
		}
		project := NewProject(definition, options.OutputDir)
		project.Configurations = configurations
		groupPath := ""
		if backend.InitializeProject != nil {
			groupPath, err = backend.InitializeProject(project, options)
			if err != nil {
				return nil, err
			}
		}
		workspace.AddProject(project, groupPath)
	}
	if len(workspace.Projects) == 0 {
		return nil, fmt.Errorf("package %q has no projects matching the requested %s target and configurations", pkg.RepoName, backend.Name)
	}

	wireProjectDependencies(workspace.Projects)
	if err := resolveWorkspaceConfigurations(workspace.Projects, backend); err != nil {
		return nil, err
	}
	if workspace.StartupProject == "" {
		workspace.StartupProject = defaultStartupProject(workspace.Projects)
	}
	return workspace, nil
}

func ConfigSelected(config denv.BuildConfig, requested []denv.BuildConfig) bool {
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

func FindProjectConfiguration(configurations []*ProjectConfiguration, buildConfig denv.BuildConfig) (*ProjectConfiguration, bool) {
	for _, configuration := range configurations {
		if configuration.BuildConfig.IsEqual(buildConfig) {
			return configuration, true
		}
	}
	return nil, false
}

func wireProjectDependencies(projects []*Project) {
	byName := make(map[string]*Project, len(projects))
	for _, project := range projects {
		byName[project.Name] = project
	}
	for _, project := range projects {
		if project.Definition.Dependencies == nil {
			continue
		}
		for _, dependency := range project.Definition.Dependencies.Values {
			if resolved, ok := byName[dependency.Name]; ok {
				project.Dependencies.Add(resolved)
			}
		}
		project.Dependencies.Sort()
	}
}

func resolveWorkspaceConfigurations(projects []*Project, backend WorkspaceBuilderBackend) error {
	localConfigurations := make(map[*Project][]*ProjectConfiguration, len(projects))
	for _, project := range projects {
		localConfigurations[project] = append([]*ProjectConfiguration(nil), project.Configurations...)
	}
	for _, project := range projects {
		dependencies, err := resolvedDependencyOrder(project, backend.Name)
		if err != nil {
			return err
		}
		for _, local := range localConfigurations[project] {
			dependencyConfigurations := make([]*ProjectConfiguration, 0, len(dependencies))
			for _, dependency := range dependencies {
				if configuration, ok := FindProjectConfiguration(localConfigurations[dependency], local.BuildConfig); ok {
					dependencyConfigurations = append(dependencyConfigurations, configuration)
				}
			}
			if err := backend.ResolveConfiguration(project, local, dependencyConfigurations); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolvedDependencyOrder(project *Project, backendName string) ([]*Project, error) {
	result := make([]*Project, 0)
	state := make(map[*Project]uint8)
	path := make([]*Project, 0)
	var visit func(*Project, bool) error
	visit = func(current *Project, root bool) error {
		if state[current] == 1 {
			cycleStart := 0
			for index, candidate := range path {
				if candidate == current {
					cycleStart = index
					break
				}
			}
			cycle := make([]string, 0, len(path)-cycleStart+1)
			for _, candidate := range path[cycleStart:] {
				cycle = append(cycle, candidate.Name)
			}
			cycle = append(cycle, current.Name)
			return fmt.Errorf("%s project dependency cycle: %s", backendName, strings.Join(cycle, " -> "))
		}
		if state[current] == 2 {
			return nil
		}
		state[current] = 1
		path = append(path, current)
		for _, dependency := range current.Dependencies.Values {
			if err := visit(dependency, false); err != nil {
				return err
			}
		}
		path = path[:len(path)-1]
		state[current] = 2
		if !root {
			result = append(result, current)
		}
		return nil
	}
	if err := visit(project, true); err != nil {
		return nil, err
	}
	return result, nil
}

func defaultStartupProject(projects []*Project) string {
	for _, project := range projects {
		if project.TypeIsExe() {
			return project.Name
		}
	}
	return projects[0].Name
}
