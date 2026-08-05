package graph

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

type VsLinkErrorReporting uint8

const (
	VsLinkErrorReportingDefault VsLinkErrorReporting = iota
	VsLinkErrorReportingPrompt
	VsLinkErrorReportingQueue
	VsLinkErrorReportingSend
	VsLinkErrorReportingNone
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
	CStandard          CStdType
	CppStandard        CppStdType
	Optimization       VsOptimization
	WarningLevel       VsWarningLevel
	DebugInformation   VsDebugInformation
	ExceptionHandling  VsExceptionHandling
	FloatingPoint      VsFloatingPointModel
	InlineExpansion    VsInlineExpansion
	Subsystem          VsSubsystem
	LinkErrorReporting VsLinkErrorReporting
	Machine            VsMachine
	CompileOptions     VsCompileOptions
	LinkOptions        VsLinkOptions
	Defines            []string
	IncludePaths       []string
	LibraryPaths       []string
	Libraries          []string
}
