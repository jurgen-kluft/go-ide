package msdev

import "github.com/jurgen-kluft/gide/graph"

type VsCStandard = graph.VsCStandard
type VsCppStandard = graph.VsCppStandard
type VsOptimization = graph.VsOptimization
type VsWarningLevel = graph.VsWarningLevel
type VsDebugInformation = graph.VsDebugInformation
type VsExceptionHandling = graph.VsExceptionHandling
type VsFloatingPointModel = graph.VsFloatingPointModel
type VsInlineExpansion = graph.VsInlineExpansion
type VsSubsystem = graph.VsSubsystem
type VsLinkErrorReporting = graph.VsLinkErrorReporting
type VsMachine = graph.VsMachine
type VsCompileOptions = graph.VsCompileOptions
type VsLinkOptions = graph.VsLinkOptions
type VsConfiguration = graph.VsConfiguration

const (
	VsCStandardDefault                    = graph.VsCStandardDefault
	VsCStandard11                         = graph.VsCStandard11
	VsCStandard17                         = graph.VsCStandard17
	VsCStandardLatest                     = graph.VsCStandardLatest
	VsCppStandardDefault                  = graph.VsCppStandardDefault
	VsCppStandard14                       = graph.VsCppStandard14
	VsCppStandard17                       = graph.VsCppStandard17
	VsCppStandard20                       = graph.VsCppStandard20
	VsCppStandardLatest                   = graph.VsCppStandardLatest
	VsOptimizationDefault                 = graph.VsOptimizationDefault
	VsOptimizationDisabled                = graph.VsOptimizationDisabled
	VsOptimizationSize                    = graph.VsOptimizationSize
	VsOptimizationSpeed                   = graph.VsOptimizationSpeed
	VsWarningLevelDefault                 = graph.VsWarningLevelDefault
	VsWarningLevelDisabled                = graph.VsWarningLevelDisabled
	VsWarningLevel1                       = graph.VsWarningLevel1
	VsWarningLevel2                       = graph.VsWarningLevel2
	VsWarningLevel3                       = graph.VsWarningLevel3
	VsWarningLevel4                       = graph.VsWarningLevel4
	VsDebugInformationNone                = graph.VsDebugInformationNone
	VsDebugInformationProgramDatabase     = graph.VsDebugInformationProgramDatabase
	VsExceptionHandlingDisabled           = graph.VsExceptionHandlingDisabled
	VsExceptionHandlingCpp                = graph.VsExceptionHandlingCpp
	VsFloatingPointDefault                = graph.VsFloatingPointDefault
	VsFloatingPointPrecise                = graph.VsFloatingPointPrecise
	VsFloatingPointStrict                 = graph.VsFloatingPointStrict
	VsFloatingPointFast                   = graph.VsFloatingPointFast
	VsInlineExpansionDefault              = graph.VsInlineExpansionDefault
	VsInlineExpansionDisabled             = graph.VsInlineExpansionDisabled
	VsInlineExpansionExplicit             = graph.VsInlineExpansionExplicit
	VsInlineExpansionAnySuitable          = graph.VsInlineExpansionAnySuitable
	VsSubsystemDefault                    = graph.VsSubsystemDefault
	VsSubsystemConsole                    = graph.VsSubsystemConsole
	VsSubsystemWindows                    = graph.VsSubsystemWindows
	VsLinkErrorReportingDefault           = graph.VsLinkErrorReportingDefault
	VsLinkErrorReportingPrompt            = graph.VsLinkErrorReportingPrompt
	VsLinkErrorReportingQueue             = graph.VsLinkErrorReportingQueue
	VsLinkErrorReportingSend              = graph.VsLinkErrorReportingSend
	VsLinkErrorReportingNone              = graph.VsLinkErrorReportingNone
	VsMachineDefault                      = graph.VsMachineDefault
	VsMachineX86                          = graph.VsMachineX86
	VsMachineX64                          = graph.VsMachineX64
	VsMachineArm                          = graph.VsMachineArm
	VsMachineArm64                        = graph.VsMachineArm64
	VsCompileOptionNone                   = graph.VsCompileOptionNone
	VsCompileOptionMultiProcessor         = graph.VsCompileOptionMultiProcessor
	VsCompileOptionTreatWarningsAsErrors  = graph.VsCompileOptionTreatWarningsAsErrors
	VsCompileOptionIntrinsicFunctions     = graph.VsCompileOptionIntrinsicFunctions
	VsCompileOptionSuppressStartupBanner  = graph.VsCompileOptionSuppressStartupBanner
	VsCompileOptionFullPathDiagnostics    = graph.VsCompileOptionFullPathDiagnostics
	VsCompileOptionColumnDiagnostics      = graph.VsCompileOptionColumnDiagnostics
	VsLinkOptionNone                      = graph.VsLinkOptionNone
	VsLinkOptionGenerateDebugInformation  = graph.VsLinkOptionGenerateDebugInformation
	VsLinkOptionDynamicBase               = graph.VsLinkOptionDynamicBase
	VsLinkOptionDataExecutionPrevention   = graph.VsLinkOptionDataExecutionPrevention
	VsLinkOptionSuppressStartupBanner     = graph.VsLinkOptionSuppressStartupBanner
	VsLinkOptionOptimizeReferences        = graph.VsLinkOptionOptimizeReferences
	VsLinkOptionComdatFolding             = graph.VsLinkOptionComdatFolding
	VsLinkOptionLinkTimeCodeGeneration    = graph.VsLinkOptionLinkTimeCodeGeneration
	VsLinkOptionDisableIncrementalLinking = graph.VsLinkOptionDisableIncrementalLinking
)

func mergeVsConfiguration(base, overlay VsConfiguration) VsConfiguration {
	if overlay.CStandard != VsCStandardDefault {
		base.CStandard = overlay.CStandard
	}
	if overlay.CppStandard != VsCppStandardDefault {
		base.CppStandard = overlay.CppStandard
	}
	if overlay.Optimization != VsOptimizationDefault {
		base.Optimization = overlay.Optimization
	}
	if overlay.WarningLevel != VsWarningLevelDefault {
		base.WarningLevel = overlay.WarningLevel
	}
	if overlay.DebugInformation != VsDebugInformationNone {
		base.DebugInformation = overlay.DebugInformation
	}
	if overlay.ExceptionHandling != VsExceptionHandlingDisabled {
		base.ExceptionHandling = overlay.ExceptionHandling
	}
	if overlay.FloatingPoint != VsFloatingPointDefault {
		base.FloatingPoint = overlay.FloatingPoint
	}
	if overlay.InlineExpansion != VsInlineExpansionDefault {
		base.InlineExpansion = overlay.InlineExpansion
	}
	if overlay.Subsystem != VsSubsystemDefault {
		base.Subsystem = overlay.Subsystem
	}
	if overlay.LinkErrorReporting != VsLinkErrorReportingDefault {
		base.LinkErrorReporting = overlay.LinkErrorReporting
	}
	if overlay.Machine != VsMachineDefault {
		base.Machine = overlay.Machine
	}
	base.CompileOptions |= overlay.CompileOptions
	base.LinkOptions |= overlay.LinkOptions
	base.Defines = uniqueStrings(base.Defines, overlay.Defines)
	base.IncludePaths = uniqueStrings(base.IncludePaths, overlay.IncludePaths)
	base.LibraryPaths = uniqueStrings(base.LibraryPaths, overlay.LibraryPaths)
	base.Libraries = uniqueStrings(base.Libraries, overlay.Libraries)
	return base
}
