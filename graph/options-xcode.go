package graph

import (
	"path/filepath"

	corepkg "github.com/jurgen-kluft/go-core"
	"github.com/jurgen-kluft/go-ide/denv"
)

type WorkspaceOptionsXcode struct {
	CppStd CppStdType
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
	CppDefines    *corepkg.ValueSet
	CppFlags      *corepkg.ValueSet
	IncludeDirs   *denv.PinnedPaths
	LinkFlags     *corepkg.ValueSet
	XcodeSettings *corepkg.KeyValueSet
	GenDataXcode  XcodeConfigurationUUIDs
	Resolved      *XcodeConfigurationResolved
}

func NewXcodeConfiguration(buildTarget denv.BuildTarget, buildConfig denv.BuildConfig) *XcodeConfiguration {
	configuration := &XcodeConfiguration{
		BuildTarget:   buildTarget,
		BuildConfig:   buildConfig,
		CppDefines:    corepkg.NewValueSet(),
		CppFlags:      corepkg.NewValueSet(),
		IncludeDirs:   denv.NewPinnedPaths(),
		LinkFlags:     corepkg.NewValueSet(),
		XcodeSettings: corepkg.NewKeyValueSet(),
		Resolved: &XcodeConfigurationResolved{
			OutputTarget: NewFileEntry(""),
			BuildTmpDir:  NewFileEntry(""),
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
		Resolved:      &XcodeConfigurationResolved{OutputTarget: NewFileEntry(""), BuildTmpDir: NewFileEntry("")},
	}
}

func (configuration *XcodeConfiguration) Merge(other *XcodeConfiguration) {
	configuration.CppDefines.Merge(other.CppDefines)
	configuration.CppFlags.Merge(other.CppFlags)
	configuration.IncludeDirs.Merge(other.IncludeDirs)
	configuration.LinkFlags.Merge(other.LinkFlags)
	configuration.XcodeSettings.Merge(other.XcodeSettings)
}

type XcodeProjectView interface {
	XcodeProjectName() string
	XcodeProjectBuildType() denv.BuildType
}

func (configuration *XcodeConfiguration) Resolve(project XcodeProjectView, outputDir string, dependencies ...*XcodeConfiguration) *XcodeConfiguration {
	resolved := configuration.Copy()
	for _, dependency := range dependencies {
		resolved.Merge(dependency)
	}
	projectName := project.XcodeProjectName()
	buildType := project.XcodeProjectBuildType()
	configName := configuration.String() + "_" + configuration.BuildTarget.Arch().String()
	resolved.Resolved.BuildTmpDir.Init(filepath.Join(outputDir, "build_tmp", configName, projectName), true)
	settings := configuration.BuildTarget.Os().Settings()
	filename := projectName
	outputRoot := "bin"
	switch {
	case buildType.IsDynamicLibrary():
		filename = settings.DllTargetPrefix + projectName + settings.DllTargetSuffix
	case buildType.IsStaticLibrary():
		filename = settings.LibTargetPrefix + projectName + settings.LibTargetSuffix
		outputRoot = "lib"
	default:
		filename = settings.ExeTargetPrefix + projectName + settings.ExeTargetSuffix
	}
	resolved.Resolved.OutputTarget.Init(filepath.Join(outputDir, outputRoot, projectName, configName, filename))
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
	return &XcodeProjectConfig{XcodeProj: NewFileEntry("")}
}
