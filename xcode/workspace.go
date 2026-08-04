package xcode

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/jurgen-kluft/gide/denv"
	"github.com/jurgen-kluft/gide/graph"
)

type CppStdType uint8

const (
	CppStdUnknown CppStdType = iota
	CppStd11
	CppStd14
	CppStd17
	CppStd20
	CppStdLatest
)

type Workspace struct {
	WorkspaceName   string
	GenerateAbsPath string
	BuildTarget     denv.BuildTarget
	CppStd          CppStdType
	ProjectList     *ProjectList
	ProjectGroups   *ProjectGroups
}

func NewWorkspace(name, outputDir string, buildTarget denv.BuildTarget) *Workspace {
	return &Workspace{
		WorkspaceName:   name,
		GenerateAbsPath: outputDir,
		BuildTarget:     buildTarget,
		CppStd:          CppStd17,
		ProjectList:     NewProjectList(),
		ProjectGroups:   NewProjectGroups(),
	}
}

func (workspace *Workspace) AddProject(project *Project, group string) {
	workspace.ProjectList.Add(project)
	workspace.ProjectGroups.Add(project, group)
	sort.Slice(workspace.ProjectList.Values, func(i, j int) bool {
		return workspace.ProjectList.Values[i].Name < workspace.ProjectList.Values[j].Name
	})
}

func (workspace *Workspace) Validate() error {
	if workspace == nil {
		return errors.New("Xcode workspace is nil")
	}
	if workspace.WorkspaceName == "" {
		return errors.New("Xcode workspace name is required")
	}
	if workspace.GenerateAbsPath == "" {
		return errors.New("Xcode workspace output directory is required")
	}
	if !workspace.BuildTarget.Mac() && !workspace.BuildTarget.AppleiOS() {
		return errors.New("Xcode workspace requires a macOS or iOS build target")
	}
	if workspace.ProjectList == nil || len(workspace.ProjectList.Values) == 0 {
		return errors.New("Xcode workspace requires at least one project")
	}
	if workspace.ProjectGroups == nil || workspace.ProjectGroups.Root == nil {
		return errors.New("Xcode workspace project groups are not initialized")
	}
	for _, project := range workspace.ProjectList.Values {
		if project == nil {
			return errors.New("Xcode workspace contains a nil project")
		}
		if project.Resolved == nil || project.Resolved.Configs == nil || project.Resolved.Configs.First() == nil {
			return fmt.Errorf("Xcode project %q has no resolved configurations", project.Name)
		}
		if project.Settings == nil || project.Dependencies == nil || project.VirtualFolders == nil {
			return fmt.Errorf("Xcode project %q is not initialized", project.Name)
		}
		for _, configuration := range project.Resolved.Configs.Values {
			if configuration == nil || configuration.Resolved == nil || configuration.Resolved.OutputTarget == nil || configuration.Resolved.OutputTarget.Path == "" || configuration.Resolved.BuildTmpDir == nil || configuration.Resolved.BuildTmpDir.Path == "" {
				return fmt.Errorf("Xcode project %q has an unresolved configuration", project.Name)
			}
		}
	}
	return validateProjectGroup(workspace.ProjectGroups.Root, make(map[*ProjectGroup]bool))
}

func validateProjectGroup(group *ProjectGroup, visiting map[*ProjectGroup]bool) error {
	if visiting[group] {
		return fmt.Errorf("Xcode workspace project group cycle at %q", group.Path)
	}
	visiting[group] = true
	for _, child := range group.Children {
		if child == nil {
			return fmt.Errorf("Xcode workspace project group %q contains a nil child", group.Path)
		}
		if err := validateProjectGroup(child, visiting); err != nil {
			return err
		}
	}
	delete(visiting, group)
	return nil
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

type Project struct {
	Name            string
	BuildType       denv.BuildType
	ProjectAbsPath  string
	GenerateAbsPath string
	Group           *ProjectGroup
	SrcFileGroups   []*graph.FileEntryDict
	ResFileGroups   []*graph.FileEntryDict
	VirtualFolders  *graph.VirtualDirectories
	Settings        *ProjectSettings
	Dependencies    *ProjectList
	Resolved        *ProjectResolved
}

func NewProject(name string, buildType denv.BuildType, projectDir, outputDir string) *Project {
	settings := &ProjectSettings{}
	settings.Xcode.BundleIdentifier = "$(PRODUCT_BUNDLE_IDENTIFIER)"
	project := &Project{
		Name:            name,
		BuildType:       buildType,
		ProjectAbsPath:  projectDir,
		GenerateAbsPath: outputDir,
		SrcFileGroups:   []*graph.FileEntryDict{graph.NewFileEntryDict(projectDir)},
		ResFileGroups:   []*graph.FileEntryDict{graph.NewFileEntryDict(projectDir)},
		VirtualFolders:  graph.NewVirtualDirectories(projectDir),
		Settings:        settings,
		Dependencies:    NewProjectList(),
		Resolved:        NewProjectResolved(),
	}
	project.Resolved.HasOutputTarget = buildType.IsExecutable() || buildType.IsStaticLibrary() || buildType.IsDynamicLibrary()
	project.Resolved.GenDataXcode.XcodeProj.Init(filepath.Join(outputDir, name+".xcodeproj"), true)
	project.Resolved.GenDataXcode.PbxProj = filepath.Join(project.Resolved.GenDataXcode.XcodeProj.Path, "project.pbxproj")
	return project
}

func (project *Project) CollectFiles(definition *denv.DevProject, target denv.BuildTarget) error {
	files, err := graph.CollectProjectFiles(definition, target)
	if err != nil {
		return err
	}
	project.SrcFileGroups = files.Groups
	project.VirtualFolders = files.VirtualFolders
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

type ProjectList struct {
	Values []*Project
}

func NewProjectList() *ProjectList {
	return &ProjectList{Values: make([]*Project, 0)}
}

func (projects *ProjectList) Add(project *Project) {
	projects.Values = append(projects.Values, project)
}

type ProjectGroup struct {
	Path     string
	Children []*ProjectGroup
	Projects []*Project
	Parent   *ProjectGroup
}

func NewProjectGroup(path string) *ProjectGroup {
	return &ProjectGroup{Path: path, Children: make([]*ProjectGroup, 0), Projects: make([]*Project, 0)}
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
	project.Settings.Group = path
	group.Projects = append(group.Projects, project)
}

func (groups *ProjectGroups) group(path string) *ProjectGroup {
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
