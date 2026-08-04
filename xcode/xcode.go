package xcode

import (
	"path/filepath"
	"strings"

	corepkg "github.com/jurgen-kluft/gcore"
	"github.com/jurgen-kluft/gide/denv"
	"github.com/jurgen-kluft/gide/graph"
)

type FileType = graph.FileType
type FileEntry = graph.FileEntry
type FileEntryDict = graph.FileEntryDict
type VirtualDirectory = graph.VirtualDirectory
type VirtualDirectories = graph.VirtualDirectories

const (
	FileTypeNone      = graph.FileTypeNone
	FileTypeCppSource = graph.FileTypeCppSource
	FileTypeCSource   = graph.FileTypeCSource
)

func NewFileEntry() *FileEntry {
	return graph.NewFileEntry("")
}

func NewFileEntryInit(path string) *FileEntry {
	return graph.NewFileEntry(path)
}

func NewFileEntryDict(path string) *FileEntryDict {
	return graph.NewFileEntryDict(path)
}

func NewVirtualFolders(diskPath string) *VirtualDirectories {
	return graph.NewVirtualDirectories(diskPath)
}

type OrderedSettings struct {
	Keys   []string
	Values []string
	index  map[string]int
}

func NewOrderedSettings() *OrderedSettings {
	return &OrderedSettings{index: make(map[string]int)}
}

func (settings *OrderedSettings) Set(key, value string) {
	if index, ok := settings.index[key]; ok {
		settings.Values[index] = value
		return
	}
	settings.index[key] = len(settings.Keys)
	settings.Keys = append(settings.Keys, key)
	settings.Values = append(settings.Values, value)
}

func (settings *OrderedSettings) Copy() *OrderedSettings {
	copy := NewOrderedSettings()
	for index, key := range settings.Keys {
		copy.Set(key, settings.Values[index])
	}
	return copy
}

func (settings *OrderedSettings) Merge(other *OrderedSettings) {
	for index, key := range other.Keys {
		settings.Set(key, other.Values[index])
	}
}

type StringValues struct {
	Values []string
	seen   map[string]bool
}

func NewStringValues() *StringValues {
	return &StringValues{seen: make(map[string]bool)}
}

func (values *StringValues) Add(value string) {
	key := strings.ToLower(value)
	if value == "" || values.seen[key] {
		return
	}
	values.seen[key] = true
	values.Values = append(values.Values, value)
}

func (values *StringValues) Copy() *StringValues {
	copy := NewStringValues()
	for _, value := range values.Values {
		copy.Add(value)
	}
	return copy
}

func (values *StringValues) Merge(other *StringValues) {
	for _, value := range other.Values {
		values.Add(value)
	}
}

type PinnedPaths struct {
	Values []denv.PinnedPath
	seen   map[string]bool
}

func NewPinnedPaths() *PinnedPaths {
	return &PinnedPaths{seen: make(map[string]bool)}
}

func (paths *PinnedPaths) Add(path denv.PinnedPath) {
	key := strings.ToLower(path.String())
	if paths.seen[key] {
		return
	}
	paths.seen[key] = true
	paths.Values = append(paths.Values, path)
}

func (paths *PinnedPaths) Copy() *PinnedPaths {
	copy := NewPinnedPaths()
	for _, path := range paths.Values {
		copy.Add(path)
	}
	return copy
}

func (paths *PinnedPaths) Merge(other *PinnedPaths) {
	for _, path := range other.Values {
		paths.Add(path)
	}
}

type XcodeConfigurationUUIDs struct {
	ProjectConfigUuid corepkg.UUID
	TargetUuid        corepkg.UUID
	TargetConfigUuid  corepkg.UUID
}

type XcodeConfigurationResolved struct {
	OutputTarget *FileEntry
	BuildTmpDir  *FileEntry
}

type XcodeConfiguration struct {
	BuildTarget   denv.BuildTarget
	BuildConfig   denv.BuildConfig
	CppDefines    *StringValues
	CppFlags      *StringValues
	IncludeDirs   *PinnedPaths
	LinkFlags     *StringValues
	XcodeSettings *OrderedSettings
	GenDataXcode  XcodeConfigurationUUIDs
	Resolved      *XcodeConfigurationResolved
}

func NewXcodeConfiguration(buildTarget denv.BuildTarget, buildConfig denv.BuildConfig) *XcodeConfiguration {
	configuration := &XcodeConfiguration{
		BuildTarget:   buildTarget,
		BuildConfig:   buildConfig,
		CppDefines:    NewStringValues(),
		CppFlags:      NewStringValues(),
		IncludeDirs:   NewPinnedPaths(),
		LinkFlags:     NewStringValues(),
		XcodeSettings: NewOrderedSettings(),
		Resolved: &XcodeConfigurationResolved{
			OutputTarget: NewFileEntry(),
			BuildTmpDir:  NewFileEntry(),
		},
	}
	configuration.initializeDefaults()
	return configuration
}

func (configuration *XcodeConfiguration) initializeDefaults() {
	os := configuration.BuildTarget.Os()
	switch {
	case configuration.BuildTarget.AppleiOS():
		configuration.XcodeSettings.Set("SDKROOT", "iphoneos")
		configuration.XcodeSettings.Set("SUPPORTED_PLATFORMS", "iphonesimulator iphoneos")
		configuration.XcodeSettings.Set("IPHONEOS_DEPLOYMENT_TARGET", "10.1")
	case os.Mac():
		configuration.CppDefines.Add("TARGET_MAC")
		configuration.CppDefines.Add("UNICODE")
		configuration.CppDefines.Add("_UNICODE")
		configuration.LinkFlags.Add("-ObjC")
		configuration.XcodeSettings.Set("SDKROOT", "macosx")
		configuration.XcodeSettings.Set("SUPPORTED_PLATFORMS", "macosx")
		configuration.XcodeSettings.Set("MACOSX_DEPLOYMENT_TARGET", "10.15")
	}
	if configuration.BuildConfig.IsDebug() {
		configuration.XcodeSettings.Set("DEBUG_INFORMATION_FORMAT", "dwarf")
		configuration.XcodeSettings.Set("GCC_GENERATE_DEBUGGING_SYMBOLS", "YES")
		configuration.XcodeSettings.Set("GCC_OPTIMIZATION_LEVEL", "0")
		configuration.XcodeSettings.Set("ONLY_ACTIVE_ARCH", "YES")
	} else {
		configuration.XcodeSettings.Set("DEBUG_INFORMATION_FORMAT", "dwarf-with-dsym")
		configuration.XcodeSettings.Set("GCC_GENERATE_DEBUGGING_SYMBOLS", "NO")
		configuration.XcodeSettings.Set("GCC_OPTIMIZATION_LEVEL", "s")
		configuration.XcodeSettings.Set("ONLY_ACTIVE_ARCH", "NO")
		configuration.XcodeSettings.Set("LLVM_LTO", "YES")
		configuration.XcodeSettings.Set("DEAD_CODE_STRIPPING", "YES")
	}
	configuration.XcodeSettings.Set("ENABLE_TESTABILITY", "YES")
	configuration.XcodeSettings.Set("CODE_SIGN_IDENTITY", "-")
	configuration.XcodeSettings.Set("ALWAYS_SEARCH_USER_PATHS", "NO")
}

func (configuration *XcodeConfiguration) Copy() *XcodeConfiguration {
	return &XcodeConfiguration{
		BuildTarget:   configuration.BuildTarget,
		BuildConfig:   configuration.BuildConfig,
		CppDefines:    configuration.CppDefines.Copy(),
		CppFlags:      configuration.CppFlags.Copy(),
		IncludeDirs:   configuration.IncludeDirs.Copy(),
		LinkFlags:     configuration.LinkFlags.Copy(),
		XcodeSettings: configuration.XcodeSettings.Copy(),
		GenDataXcode:  configuration.GenDataXcode,
		Resolved:      &XcodeConfigurationResolved{OutputTarget: NewFileEntry(), BuildTmpDir: NewFileEntry()},
	}
}

func (configuration *XcodeConfiguration) Merge(other *XcodeConfiguration) {
	configuration.CppDefines.Merge(other.CppDefines)
	configuration.CppFlags.Merge(other.CppFlags)
	configuration.IncludeDirs.Merge(other.IncludeDirs)
	configuration.LinkFlags.Merge(other.LinkFlags)
	configuration.XcodeSettings.Merge(other.XcodeSettings)
}

func (configuration *XcodeConfiguration) Resolve(project *Project, outputDir string, dependencies ...*XcodeConfiguration) *XcodeConfiguration {
	resolved := configuration.Copy()
	for _, dependency := range dependencies {
		resolved.Merge(dependency)
	}
	configName := configuration.String() + "_" + configuration.BuildTarget.Arch().String()
	resolved.Resolved.BuildTmpDir.Init(filepath.Join(outputDir, "build_tmp", configName, project.Name), true)
	settings := configuration.BuildTarget.Os().Settings()
	filename := project.Name
	outputRoot := "bin"
	switch {
	case project.TypeIsDll():
		filename = settings.DllTargetPrefix + project.Name + settings.DllTargetSuffix
	case project.TypeIsLib():
		filename = settings.LibTargetPrefix + project.Name + settings.LibTargetSuffix
		outputRoot = "lib"
	default:
		filename = settings.ExeTargetPrefix + project.Name + settings.ExeTargetSuffix
	}
	resolved.Resolved.OutputTarget.Init(filepath.Join(outputDir, outputRoot, project.Name, configName, filename))
	return resolved
}

func (configuration *XcodeConfiguration) String() string {
	return configuration.BuildConfig.String()
}

type XcodeConfigurationList struct {
	Values []*XcodeConfiguration
}

func NewXcodeConfigurationList() *XcodeConfigurationList {
	return &XcodeConfigurationList{Values: make([]*XcodeConfiguration, 0)}
}

func (configurations *XcodeConfigurationList) Add(configuration *XcodeConfiguration) {
	configurations.Values = append(configurations.Values, configuration)
}

func (configurations *XcodeConfigurationList) First() *XcodeConfiguration {
	if configurations == nil || len(configurations.Values) == 0 {
		return nil
	}
	return configurations.Values[0]
}

func (configurations *XcodeConfigurationList) Get(buildConfig denv.BuildConfig) (*XcodeConfiguration, bool) {
	for _, configuration := range configurations.Values {
		if configuration.BuildConfig.IsEqual(buildConfig) {
			return configuration, true
		}
	}
	return nil, false
}

type XcodeProjectConfig struct {
	XcodeProj                 *FileEntry
	PbxProj                   string
	InfoPlistFile             string
	Uuid                      corepkg.UUID
	TargetUuid                corepkg.UUID
	TargetProductUuid         corepkg.UUID
	ConfigListUuid            corepkg.UUID
	TargetConfigListUuid      corepkg.UUID
	DependencyProxyUuid       corepkg.UUID
	DependencyTargetUuid      corepkg.UUID
	DependencyTargetProxyUuid corepkg.UUID
}

func NewXcodeProjectConfig() *XcodeProjectConfig {
	return &XcodeProjectConfig{XcodeProj: NewFileEntry()}
}

type ProjectResolved struct {
	HasOutputTarget bool
	Configs         *XcodeConfigurationList
	PchHeader       *FileEntry
	GenDataXcode    *XcodeProjectConfig
}

func NewProjectResolved() *ProjectResolved {
	return &ProjectResolved{Configs: NewXcodeConfigurationList(), GenDataXcode: NewXcodeProjectConfig()}
}
