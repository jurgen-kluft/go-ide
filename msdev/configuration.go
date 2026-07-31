package msdev

type VsCStandard uint8

const (
	VsCStandardDefault VsCStandard = iota
	VsCStandard11
	VsCStandard17
	VsCStandardLatest
)

type VsCppStandard uint8

const (
	VsCppStandardDefault VsCppStandard = iota
	VsCppStandard14
	VsCppStandard17
	VsCppStandard20
	VsCppStandardLatest
)

type VsOptimization uint8

const (
	VsOptimizationDefault VsOptimization = iota
	VsOptimizationDisabled
	VsOptimizationSize
	VsOptimizationSpeed
)

type VsWarningLevel uint8

const (
	VsWarningLevelDefault VsWarningLevel = iota
	VsWarningLevelDisabled
	VsWarningLevel1
	VsWarningLevel2
	VsWarningLevel3
	VsWarningLevel4
)

type VsDebugInformation uint8

const (
	VsDebugInformationNone VsDebugInformation = iota
	VsDebugInformationProgramDatabase
)

type VsExceptionHandling uint8

const (
	VsExceptionHandlingDisabled VsExceptionHandling = iota
	VsExceptionHandlingCpp
)

type VsFloatingPointModel uint8

const (
	VsFloatingPointDefault VsFloatingPointModel = iota
	VsFloatingPointPrecise
	VsFloatingPointStrict
	VsFloatingPointFast
)

type VsInlineExpansion uint8

const (
	VsInlineExpansionDefault VsInlineExpansion = iota
	VsInlineExpansionDisabled
	VsInlineExpansionExplicit
	VsInlineExpansionAnySuitable
)

type VsSubsystem uint8

const (
	VsSubsystemDefault VsSubsystem = iota
	VsSubsystemConsole
	VsSubsystemWindows
)

type VsMachine uint8

const (
	VsMachineDefault VsMachine = iota
	VsMachineX86
	VsMachineX64
	VsMachineArm
	VsMachineArm64
)

type VsCompileOptions uint64

const (
	VsCompileOptionNone           VsCompileOptions = 0
	VsCompileOptionMultiProcessor VsCompileOptions = 1 << (iota - 1)
	VsCompileOptionTreatWarningsAsErrors
	VsCompileOptionIntrinsicFunctions
	VsCompileOptionSuppressStartupBanner
	VsCompileOptionFullPathDiagnostics
	VsCompileOptionColumnDiagnostics
)

func (options VsCompileOptions) Has(option VsCompileOptions) bool {
	return options&option == option
}

type VsLinkOptions uint64

const (
	VsLinkOptionNone                     VsLinkOptions = 0
	VsLinkOptionGenerateDebugInformation VsLinkOptions = 1 << (iota - 1)
	VsLinkOptionDynamicBase
	VsLinkOptionDataExecutionPrevention
	VsLinkOptionSuppressStartupBanner
	VsLinkOptionOptimizeReferences
	VsLinkOptionComdatFolding
	VsLinkOptionLinkTimeCodeGeneration
	VsLinkOptionDisableIncrementalLinking
)

func (options VsLinkOptions) Has(option VsLinkOptions) bool {
	return options&option == option
}

type VsConfiguration struct {
	CStandard         VsCStandard
	CppStandard       VsCppStandard
	Optimization      VsOptimization
	WarningLevel      VsWarningLevel
	DebugInformation  VsDebugInformation
	ExceptionHandling VsExceptionHandling
	FloatingPoint     VsFloatingPointModel
	InlineExpansion   VsInlineExpansion
	Subsystem         VsSubsystem
	Machine           VsMachine
	CompileOptions    VsCompileOptions
	LinkOptions       VsLinkOptions
	Defines           []string
	IncludePaths      []string
	LibraryPaths      []string
	Libraries         []string
}
