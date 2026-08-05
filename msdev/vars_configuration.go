package msdev

import (
	"fmt"
	"strings"

	corepkg "github.com/jurgen-kluft/go-core"
)

func NewConfigurationFromVars(vars *corepkg.Vars) (*VsConfiguration, error) {
	if vars == nil {
		return nil, fmt.Errorf("cannot create Visual Studio configuration from nil vars")
	}

	resolved := vars.Copy()
	resolved.FinalResolve()

	config := &VsConfiguration{}
	var err error
	if config.CStandard, err = parseVsCStandard(resolved.GetFirstOrEmpty("build.c_standard")); err != nil {
		return nil, err
	}
	if config.CppStandard, err = parseVsCppStandard(resolved.GetFirstOrEmpty("build.cpp_standard")); err != nil {
		return nil, err
	}
	if config.Optimization, err = parseVsOptimization(resolved.GetFirstOrEmpty("build.optimize")); err != nil {
		return nil, err
	}
	if config.InlineExpansion, err = parseVsInlineExpansion(resolved.GetFirstOrEmpty("build.inline_expansion")); err != nil {
		return nil, err
	}

	if err := parseVsCompileSettings(config, resolved); err != nil {
		return nil, err
	}
	if err := parseVsLinkSettings(config, resolved); err != nil {
		return nil, err
	}

	config.Defines, err = parseVsDefines(valuesOf(resolved, "build.defines"))
	if err != nil {
		return nil, err
	}
	config.IncludePaths = uniqueNonEmpty(valuesOf(resolved, "build.includes"), valuesOf(resolved, "INCLUDE"))
	config.LibraryPaths = uniqueNonEmpty(valuesOf(resolved, "library.paths"), valuesOf(resolved, "LIBPATH"))
	config.Libraries = uniqueNonEmpty(valuesOf(resolved, "library.files"), valuesOf(resolved, "build.extra_libs"))

	return config, nil
}

func parseVsCompileSettings(config *VsConfiguration, vars *corepkg.Vars) error {
	flags := uniqueNonEmpty(
		valuesOf(vars, "compiler.flags"),
		valuesOf(vars, "compiler.warning_flags"),
		valuesOf(vars, "compiler.floating_point_flags"),
		valuesOf(vars, "compiler.debug_flags"),
		valuesOf(vars, "build.exception_handling"),
		valuesOf(vars, "build.intrinsic_functions"),
	)
	for _, flag := range flags {
		switch strings.ToLower(flag) {
		case "/c":
		case "/nologo":
			config.CompileOptions |= VsCompileOptionSuppressStartupBanner
		case "/diagnostics:column":
			config.CompileOptions |= VsCompileOptionColumnDiagnostics
		case "/fc":
			config.CompileOptions |= VsCompileOptionFullPathDiagnostics
		case "/mp":
			config.CompileOptions |= VsCompileOptionMultiProcessor
		case "/wx":
			config.CompileOptions |= VsCompileOptionTreatWarningsAsErrors
		case "/w":
			config.WarningLevel = VsWarningLevelDisabled
		case "/w1":
			config.WarningLevel = VsWarningLevel1
		case "/w2":
			config.WarningLevel = VsWarningLevel2
		case "/w3":
			config.WarningLevel = VsWarningLevel3
		case "/w4":
			config.WarningLevel = VsWarningLevel4
		case "/zi":
			config.DebugInformation = VsDebugInformationProgramDatabase
		case "/ehsc":
			config.ExceptionHandling = VsExceptionHandlingCpp
		case "/oi":
			config.CompileOptions |= VsCompileOptionIntrinsicFunctions
		case "/fp:precise":
			config.FloatingPoint = VsFloatingPointPrecise
		case "/fp:strict":
			config.FloatingPoint = VsFloatingPointStrict
		case "/fp:fast":
			config.FloatingPoint = VsFloatingPointFast
		default:
			return fmt.Errorf("unsupported Visual Studio compile option %q", flag)
		}
	}
	return nil
}

func parseVsLinkSettings(config *VsConfiguration, vars *corepkg.Vars) error {
	flags := uniqueNonEmpty(valuesOf(vars, "compiler.c.link.flags"), valuesOf(vars, "compiler.c.link.extra_flags"))
	for _, flag := range flags {
		normalized := strings.ToLower(flag)
		switch normalized {
		case "/subsystem:console":
			config.Subsystem = VsSubsystemConsole
		case "/subsystem:windows":
			config.Subsystem = VsSubsystemWindows
		case "/errorreport:promptimmediately":
			config.LinkErrorReporting = VsLinkErrorReportingPrompt
		case "/errorreport:queuefornextlogin":
			config.LinkErrorReporting = VsLinkErrorReportingQueue
		case "/errorreport:senderrorreport":
			config.LinkErrorReporting = VsLinkErrorReportingSend
		case "/errorreport:noerrorreport":
			config.LinkErrorReporting = VsLinkErrorReportingNone
		case "/debug":
			config.LinkOptions |= VsLinkOptionGenerateDebugInformation
		case "/dynamicbase":
			config.LinkOptions |= VsLinkOptionDynamicBase
		case "/nxcompat":
			config.LinkOptions |= VsLinkOptionDataExecutionPrevention
		case "/nologo":
			config.LinkOptions |= VsLinkOptionSuppressStartupBanner
		case "/opt:ref":
			config.LinkOptions |= VsLinkOptionOptimizeReferences
		case "/opt:icf":
			config.LinkOptions |= VsLinkOptionComdatFolding
		case "/ltcg":
			config.LinkOptions |= VsLinkOptionLinkTimeCodeGeneration
		case "/incremental:no":
			config.LinkOptions |= VsLinkOptionDisableIncrementalLinking
		case "/machine:x86":
			config.Machine = VsMachineX86
		case "/machine:x64":
			config.Machine = VsMachineX64
		case "/machine:arm":
			config.Machine = VsMachineArm
		case "/machine:arm64":
			config.Machine = VsMachineArm64
		default:
			return fmt.Errorf("unsupported Visual Studio link option %q", flag)
		}
	}
	return nil
}

func parseVsCStandard(value string) (VsCStandard, error) {
	switch strings.ToLower(value) {
	case "":
		return VsCStandardDefault, nil
	case "/std:c11":
		return VsCStandard11, nil
	case "/std:c17":
		return VsCStandard17, nil
	case "/std:clatest":
		return VsCStandardLatest, nil
	default:
		return VsCStandardDefault, fmt.Errorf("unsupported Visual Studio C standard %q", value)
	}
}

func parseVsCppStandard(value string) (VsCppStandard, error) {
	switch strings.ToLower(value) {
	case "":
		return VsCppStandardDefault, nil
	case "/std:c++14":
		return VsCppStandard14, nil
	case "/std:c++17":
		return VsCppStandard17, nil
	case "/std:c++20":
		return VsCppStandard20, nil
	case "/std:c++latest":
		return VsCppStandardLatest, nil
	default:
		return VsCppStandardDefault, fmt.Errorf("unsupported Visual Studio C++ standard %q", value)
	}
}

func parseVsOptimization(value string) (VsOptimization, error) {
	switch strings.ToLower(value) {
	case "":
		return VsOptimizationDefault, nil
	case "/od":
		return VsOptimizationDisabled, nil
	case "/o1":
		return VsOptimizationSize, nil
	case "/o2":
		return VsOptimizationSpeed, nil
	default:
		return VsOptimizationDefault, fmt.Errorf("unsupported Visual Studio optimization %q", value)
	}
}

func parseVsInlineExpansion(value string) (VsInlineExpansion, error) {
	switch strings.ToLower(value) {
	case "":
		return VsInlineExpansionDefault, nil
	case "/ob0", "/b0":
		return VsInlineExpansionDisabled, nil
	case "/ob1", "/b1":
		return VsInlineExpansionExplicit, nil
	case "/ob2", "/b2", "/ob3", "/b3":
		return VsInlineExpansionAnySuitable, nil
	default:
		return VsInlineExpansionDefault, fmt.Errorf("unsupported Visual Studio inline expansion %q", value)
	}
}

func parseVsDefines(values []string) ([]string, error) {
	defines := make([]string, 0, len(values))
	for index := 0; index < len(values); index++ {
		value := values[index]
		if value == "" {
			continue
		}
		if strings.EqualFold(value, "/D") || strings.EqualFold(value, "-D") {
			index++
			if index == len(values) || values[index] == "" {
				return nil, fmt.Errorf("Visual Studio define option %q has no value", value)
			}
			defines = appendUnique(defines, values[index])
			continue
		}
		if strings.HasPrefix(strings.ToLower(value), "/d") || strings.HasPrefix(strings.ToLower(value), "-d") {
			value = value[2:]
		}
		defines = appendUnique(defines, value)
	}
	return defines, nil
}

func valuesOf(vars *corepkg.Vars, key string) []string {
	values, _ := vars.Get(key)
	return values
}

func uniqueNonEmpty(groups ...[]string) []string {
	result := make([]string, 0)
	for _, values := range groups {
		for _, value := range values {
			if value != "" {
				result = appendUnique(result, value)
			}
		}
	}
	return result
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
