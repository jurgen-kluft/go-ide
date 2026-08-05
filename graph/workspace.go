package graph

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/jurgen-kluft/go-ide/denv"
)

type Workspace struct {
	Name            string
	OutputDir       string
	BuildTarget     denv.BuildTarget
	OSTargetVersion string
	StartupProject  string
	OptionsVs       *WorkspaceOptionsVs
	OptionsXcode    *WorkspaceOptionsXcode
	Projects        []*Project
	ProjectGroups   *ProjectGroups
}

type WorkspaceOptions struct {
	Name            string
	OutputDir       string
	BuildTarget     denv.BuildTarget
	BuildConfigs    []denv.BuildConfig
	StartupProject  string
	OSTargetVersion string
	VisualStudio    WorkspaceOptionsVs
	Xcode           WorkspaceOptionsXcode
	ResolveVars     VarsResolver
}

type WorkspaceOptionsVs struct {
	PlatformToolset string
}

func NewWorkspace(name, outputDir string, buildTarget denv.BuildTarget) *Workspace {
	return &Workspace{
		Name:          name,
		OutputDir:     outputDir,
		BuildTarget:   buildTarget,
		Projects:      make([]*Project, 0),
		ProjectGroups: NewProjectGroups(),
	}
}

func (workspace *Workspace) AddProject(project *Project, groupPath string) {
	workspace.Projects = append(workspace.Projects, project)
	workspace.ProjectGroups.Add(project, groupPath)
	sort.Slice(workspace.Projects, func(i, j int) bool { return workspace.Projects[i].Name < workspace.Projects[j].Name })
}

func (workspace *Workspace) ValidateStructure() error {
	if workspace == nil {
		return errors.New("workspace is nil")
	}
	if workspace.Name == "" {
		return errors.New("workspace name is required")
	}
	if workspace.OutputDir == "" {
		return errors.New("workspace output directory is required")
	}
	if (workspace.OptionsVs == nil) == (workspace.OptionsXcode == nil) {
		return errors.New("workspace requires exactly one backend options payload")
	}
	if len(workspace.Projects) == 0 {
		return errors.New("workspace requires at least one project")
	}
	if workspace.ProjectGroups == nil || workspace.ProjectGroups.Root == nil {
		return errors.New("workspace project groups are not initialized")
	}
	names := make(map[string]bool, len(workspace.Projects))
	for _, project := range workspace.Projects {
		if project == nil {
			return errors.New("workspace contains a nil project")
		}
		if project.Name == "" {
			return errors.New("workspace project name is required")
		}
		if names[project.Name] {
			return fmt.Errorf("duplicate workspace project %q", project.Name)
		}
		names[project.Name] = true
		for _, configuration := range project.Configurations {
			if configuration == nil {
				return fmt.Errorf("workspace project %q contains a nil configuration", project.Name)
			}
			if (configuration.OptionsVs == nil) == (configuration.OptionsXcode == nil) {
				return fmt.Errorf("workspace project %q configuration %q requires exactly one backend options payload", project.Name, configuration.BuildConfig.String())
			}
			if workspace.OptionsVs != nil && configuration.OptionsVs == nil {
				return fmt.Errorf("workspace project %q configuration %q does not contain Visual Studio options", project.Name, configuration.BuildConfig.String())
			}
			if workspace.OptionsXcode != nil && configuration.OptionsXcode == nil {
				return fmt.Errorf("workspace project %q configuration %q does not contain Xcode options", project.Name, configuration.BuildConfig.String())
			}
		}
	}
	if workspace.StartupProject != "" && !names[workspace.StartupProject] {
		return fmt.Errorf("startup project %q is not in the workspace", workspace.StartupProject)
	}
	return workspace.ProjectGroups.ValidateAcyclic()
}

func (workspace *Workspace) Validate() error {
	if workspace != nil && workspace.OptionsXcode != nil {
		if !workspace.BuildTarget.Mac() && !workspace.BuildTarget.AppleiOS() {
			return errors.New("Xcode workspace requires a macOS or iOS build target")
		}
		if err := workspace.ValidateStructure(); err != nil {
			return err
		}
		for _, project := range workspace.Projects {
			if project.Resolved == nil || project.FirstXcodeConfiguration() == nil {
				return fmt.Errorf("Xcode project %q has no resolved configurations", project.Name)
			}
			if project.Settings == nil || project.Dependencies == nil || project.Files == nil || project.Files.VirtualFolders == nil {
				return fmt.Errorf("Xcode project %q is not initialized", project.Name)
			}
			for _, configuration := range project.XcodeConfigurations() {
				if configuration == nil || configuration.Resolved == nil || configuration.Resolved.OutputTarget == nil || configuration.Resolved.OutputTarget.Path == "" || configuration.Resolved.BuildTmpDir == nil || configuration.Resolved.BuildTmpDir.Path == "" {
					return fmt.Errorf("Xcode project %q has an unresolved configuration", project.Name)
				}
			}
		}
		return nil
	}
	return workspace.ValidateStructure()
}

type Project struct {
	Definition      *denv.DevProject
	Name            string
	BuildType       denv.BuildType
	ProjectAbsPath  string
	GenerateAbsPath string
	Files           *ProjectFiles
	ResourceGroups  []*FileEntryDict
	Dependencies    *ProjectList
	Configurations  []*ProjectConfiguration
	Group           *ProjectGroup
	GroupPath       string
	Settings        *ProjectSettings
	Resolved        *ProjectResolved
}

type ProjectResolved struct {
	HasOutputTarget bool
	PchHeader       *FileEntry
	GenDataXcode    *XcodeProjectConfig
}

func NewProjectResolved() *ProjectResolved {
	return &ProjectResolved{GenDataXcode: NewXcodeProjectConfig()}
}

type ProjectSettings struct {
	Group              string
	IsGuiApp           bool
	CppAsObjCpp        bool
	MultiThreadedBuild bool
	PchHeader          string
	Xcode              struct {
		BundleIdentifier string
	}
}

func NewProject(definition *denv.DevProject, outputDir string) *Project {
	project := &Project{
		Definition:      definition,
		GenerateAbsPath: outputDir,
		Dependencies:    NewProjectList(),
		Configurations:  make([]*ProjectConfiguration, 0),
		ResourceGroups:  make([]*FileEntryDict, 0),
	}
	if definition != nil {
		project.Name = definition.Name
		project.BuildType = definition.BuildType
		project.ProjectAbsPath = definition.ProjectPath()
	}
	return project
}

func (project *Project) CollectFiles(definition *denv.DevProject, target denv.BuildTarget) error {
	project.Definition = definition
	if definition != nil {
		project.Name = definition.Name
		project.BuildType = definition.BuildType
		project.ProjectAbsPath = definition.ProjectPath()
	}
	files, err := CollectProjectFiles(definition, target)
	if err != nil {
		return err
	}
	project.Files = files
	return nil
}

func (project *Project) TypeIsExe() bool {
	return project.BuildType.IsExecutable()
}

func (project *Project) TypeIsDll() bool {
	return project.BuildType.IsDynamicLibrary()
}

func (project *Project) TypeIsLib() bool {
	return project.BuildType.IsStaticLibrary() || project.BuildType.IsDynamicLibrary()
}

func (project *Project) TypeIsExeOrDll() bool {
	return project.TypeIsExe() || project.TypeIsDll()
}

func (project *Project) XcodeProjectName() string {
	return project.Name
}

func (project *Project) XcodeProjectBuildType() denv.BuildType {
	return project.BuildType
}

func (project *Project) AddXcodeConfiguration(configuration *XcodeConfiguration) {
	project.Configurations = append(project.Configurations, &ProjectConfiguration{
		BuildTarget:  configuration.BuildTarget,
		BuildConfig:  configuration.BuildConfig,
		OptionsXcode: configuration,
		OutputTarget: configuration.Resolved.OutputTarget,
		BuildTmpDir:  configuration.Resolved.BuildTmpDir,
	})
}

func (project *Project) XcodeConfigurations() []*XcodeConfiguration {
	configurations := make([]*XcodeConfiguration, 0, len(project.Configurations))
	for _, configuration := range project.Configurations {
		if configuration != nil && configuration.OptionsXcode != nil {
			configurations = append(configurations, configuration.OptionsXcode)
		}
	}
	return configurations
}

func (project *Project) FirstXcodeConfiguration() *XcodeConfiguration {
	for _, configuration := range project.Configurations {
		if configuration != nil && configuration.OptionsXcode != nil {
			return configuration.OptionsXcode
		}
	}
	return nil
}

type ProjectConfiguration struct {
	BuildTarget     denv.BuildTarget
	BuildConfig     denv.BuildConfig
	OptionsVs       *VsConfiguration
	OptionsXcode    *XcodeConfiguration
	OutputDir       string
	IntermediateDir string
	TargetName      string
	TargetExtension string
	OutputTarget    *FileEntry
	BuildTmpDir     *FileEntry
}

type ProjectList struct {
	Values []*Project
}

func NewProjectList() *ProjectList {
	return &ProjectList{Values: make([]*Project, 0)}
}

func (projects *ProjectList) Add(project *Project) {
	projects.Values = append(projects.Values, project)
}

func (projects *ProjectList) Sort() {
	sort.Slice(projects.Values, func(i, j int) bool {
		return projects.Values[i].Name < projects.Values[j].Name
	})
}

func (projects *ProjectList) Get(name string) (*Project, bool) {
	for _, project := range projects.Values {
		if project.Name == name {
			return project, true
		}
	}
	return nil, false
}

type ProjectGroup struct {
	Path     string
	Children []*ProjectGroup
	Projects []*Project
	Parent   *ProjectGroup
}

func NewProjectGroup(path string) *ProjectGroup {
	return &ProjectGroup{
		Path:     path,
		Children: make([]*ProjectGroup, 0),
		Projects: make([]*Project, 0),
	}
}

type ProjectGroups struct {
	Root   *ProjectGroup
	byPath map[string]*ProjectGroup
}

func NewProjectGroups() *ProjectGroups {
	root := NewProjectGroup("<group_root>")
	return &ProjectGroups{Root: root, byPath: map[string]*ProjectGroup{".": root}}
}

func (groups *ProjectGroups) Add(project *Project, path string) {
	group := groups.group(path)
	project.Group = group
	project.GroupPath = path
	group.Projects = append(group.Projects, project)
	sort.Slice(group.Projects, func(i, j int) bool { return group.Projects[i].Name < group.Projects[j].Name })
}

func (groups *ProjectGroups) ValidateAcyclic() error {
	if groups == nil || groups.Root == nil {
		return errors.New("project groups are not initialized")
	}
	return validateProjectGroup(groups.Root, make(map[*ProjectGroup]bool))
}

func (groups *ProjectGroups) group(path string) *ProjectGroup {
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return groups.Root
	}
	if group, ok := groups.byPath[path]; ok {
		return group
	}
	parent := groups.group(filepath.Dir(path))
	group := NewProjectGroup(path)
	group.Parent = parent
	parent.Children = append(parent.Children, group)
	sort.Slice(parent.Children, func(i, j int) bool { return parent.Children[i].Path < parent.Children[j].Path })
	groups.byPath[path] = group
	return group
}

func validateProjectGroup(group *ProjectGroup, visiting map[*ProjectGroup]bool) error {
	if visiting[group] {
		return fmt.Errorf("project group cycle at %q", group.Path)
	}
	visiting[group] = true
	for _, child := range group.Children {
		if child == nil {
			return fmt.Errorf("project group %q contains a nil child", group.Path)
		}
		if err := validateProjectGroup(child, visiting); err != nil {
			return err
		}
	}
	delete(visiting, group)
	return nil
}
