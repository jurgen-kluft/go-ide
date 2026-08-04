package msdev

import (
	"reflect"
	"strings"
	"testing"

	corepkg "github.com/jurgen-kluft/gcore"
)

func TestNewConfigurationFromVars(t *testing.T) {
	vars := corepkg.NewVars(corepkg.VarsFormatCurlyBraces)
	vars.Set("msvc.cl.warnings_level3", "/W3")
	vars.Set("msvc.cl.warnings_are_errors", "/WX")
	vars.Set("msvc.cl.generatedebuginfo", "/Zi")
	vars.Set("msvc.cl.usefloatingpointprecise", "/fp:precise")
	vars.Set("msvc.cl.enable_exceptionhandling", "/EHsc")
	vars.Set("msvc.cl.enable_intrinsicfunctions", "/Oi")
	vars.Set("msvc.cl.optimize_speed", "/O2")
	vars.Set("msvc.cl.usecpp20", "/std:c++20")
	vars.Set("msvc.cl.usec17", "/std:c17")
	vars.Set("msvc.link.generatedebuginfo", "/DEBUG")

	vars.Set("compiler.warning_flags", "{msvc.cl.warnings_level3}", "{msvc.cl.warnings_are_errors}")
	vars.Set("compiler.debug_flags", "{msvc.cl.generatedebuginfo}")
	vars.Set("compiler.floating_point_flags", "{msvc.cl.usefloatingpointprecise}")
	vars.Set("compiler.flags", "/c", "/nologo", "/diagnostics:column", "/FC", "/MP", "{compiler.warning_flags}", "{compiler.floating_point_flags}", "{compiler.debug_flags}")
	vars.Set("build.exception_handling", "{msvc.cl.enable_exceptionhandling}")
	vars.Set("build.intrinsic_functions", "{msvc.cl.enable_intrinsicfunctions}")
	vars.Set("build.inline_expansion", "/Ob2")
	vars.Set("build.optimize", "{msvc.cl.optimize_speed}")
	vars.Set("build.cpp_standard", "{msvc.cl.usecpp20}")
	vars.Set("build.c_standard", "{msvc.cl.usec17}")
	vars.Set("compiler.c.link.extra_flags", "/SUBSYSTEM:CONSOLE", "/ERRORREPORT:PromptImmediately", "/DYNAMICBASE", "/NXCOMPAT", "/nologo", "/MACHINE:X64", "{msvc.link.generatedebuginfo}", "/OPT:REF", "/OPT:ICF", "/LTCG", "/INCREMENTAL:NO")
	vars.Set("build.defines", "/D", "TARGET_PC", "/D_DEBUG", "TARGET_PC")
	vars.Set("build.includes", "project/include", "shared/include")
	vars.Set("INCLUDE", "sdk/include", "shared/include")
	vars.Set("library.paths", "project/lib")
	vars.Set("LIBPATH", "sdk/lib", "project/lib")
	vars.Set("library.files", "project.lib")
	vars.Set("build.extra_libs", "system.lib", "project.lib")

	config, err := NewConfigurationFromVars(vars)
	if err != nil {
		t.Fatalf("NewConfigurationFromVars failed: %v", err)
	}

	if config.CStandard != VsCStandard17 {
		t.Errorf("CStandard = %v, want %v", config.CStandard, VsCStandard17)
	}
	if config.CppStandard != VsCppStandard20 {
		t.Errorf("CppStandard = %v, want %v", config.CppStandard, VsCppStandard20)
	}
	if config.Optimization != VsOptimizationSpeed {
		t.Errorf("Optimization = %v, want %v", config.Optimization, VsOptimizationSpeed)
	}
	if config.WarningLevel != VsWarningLevel3 {
		t.Errorf("WarningLevel = %v, want %v", config.WarningLevel, VsWarningLevel3)
	}
	if config.DebugInformation != VsDebugInformationProgramDatabase {
		t.Errorf("DebugInformation = %v, want %v", config.DebugInformation, VsDebugInformationProgramDatabase)
	}
	if config.ExceptionHandling != VsExceptionHandlingCpp {
		t.Errorf("ExceptionHandling = %v, want %v", config.ExceptionHandling, VsExceptionHandlingCpp)
	}
	if config.FloatingPoint != VsFloatingPointPrecise {
		t.Errorf("FloatingPoint = %v, want %v", config.FloatingPoint, VsFloatingPointPrecise)
	}
	if config.InlineExpansion != VsInlineExpansionAnySuitable {
		t.Errorf("InlineExpansion = %v, want %v", config.InlineExpansion, VsInlineExpansionAnySuitable)
	}
	if config.Subsystem != VsSubsystemConsole {
		t.Errorf("Subsystem = %v, want %v", config.Subsystem, VsSubsystemConsole)
	}
	if config.LinkErrorReporting != VsLinkErrorReportingPrompt {
		t.Errorf("LinkErrorReporting = %v, want %v", config.LinkErrorReporting, VsLinkErrorReportingPrompt)
	}
	if config.Machine != VsMachineX64 {
		t.Errorf("Machine = %v, want %v", config.Machine, VsMachineX64)
	}

	compileOptions := []VsCompileOptions{
		VsCompileOptionMultiProcessor,
		VsCompileOptionTreatWarningsAsErrors,
		VsCompileOptionIntrinsicFunctions,
		VsCompileOptionSuppressStartupBanner,
		VsCompileOptionFullPathDiagnostics,
		VsCompileOptionColumnDiagnostics,
	}
	for _, option := range compileOptions {
		if !config.CompileOptions.Has(option) {
			t.Errorf("CompileOptions does not contain %v", option)
		}
	}

	linkOptions := []VsLinkOptions{
		VsLinkOptionGenerateDebugInformation,
		VsLinkOptionDynamicBase,
		VsLinkOptionDataExecutionPrevention,
		VsLinkOptionSuppressStartupBanner,
		VsLinkOptionOptimizeReferences,
		VsLinkOptionComdatFolding,
		VsLinkOptionLinkTimeCodeGeneration,
		VsLinkOptionDisableIncrementalLinking,
	}
	for _, option := range linkOptions {
		if !config.LinkOptions.Has(option) {
			t.Errorf("LinkOptions does not contain %v", option)
		}
	}

	assertStringSlice(t, "Defines", config.Defines, []string{"TARGET_PC", "_DEBUG"})
	assertStringSlice(t, "IncludePaths", config.IncludePaths, []string{"project/include", "shared/include", "sdk/include"})
	assertStringSlice(t, "LibraryPaths", config.LibraryPaths, []string{"project/lib", "sdk/lib"})
	assertStringSlice(t, "Libraries", config.Libraries, []string{"project.lib", "system.lib"})
}

func TestNewConfigurationFromVarsRejectsUnsupportedValues(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		wantError string
	}{
		{name: "C++ standard", key: "build.cpp_standard", value: "/std:c++23", wantError: "C++ standard"},
		{name: "optimization", key: "build.optimize", value: "/Ox", wantError: "optimization"},
		{name: "compile option", key: "compiler.flags", value: "/unknown", wantError: "compile option"},
		{name: "link option", key: "compiler.c.link.extra_flags", value: "/unknown", wantError: "link option"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			vars := corepkg.NewVars(corepkg.VarsFormatCurlyBraces)
			vars.Set(test.key, test.value)

			_, err := NewConfigurationFromVars(vars)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want error containing %q", err, test.wantError)
			}
		})
	}
}

func TestNewConfigurationFromVarsRejectsDefineWithoutValue(t *testing.T) {
	vars := corepkg.NewVars(corepkg.VarsFormatCurlyBraces)
	vars.Set("build.defines", "/D")

	_, err := NewConfigurationFromVars(vars)
	if err == nil || !strings.Contains(err.Error(), "has no value") {
		t.Fatalf("error = %v, want missing define value error", err)
	}
}

func TestNewConfigurationFromVarsDoesNotMutateVars(t *testing.T) {
	vars := corepkg.NewVars(corepkg.VarsFormatCurlyBraces)
	vars.Set("msvc.cl.usecpp17", "/std:c++17")
	vars.Set("build.cpp_standard", "{msvc.cl.usecpp17}")

	if _, err := NewConfigurationFromVars(vars); err != nil {
		t.Fatalf("NewConfigurationFromVars failed: %v", err)
	}
	if got := vars.GetFirstOrEmpty("build.cpp_standard"); got != "{msvc.cl.usecpp17}" {
		t.Fatalf("input vars mutated: build.cpp_standard = %q", got)
	}
}

func assertStringSlice(t *testing.T, name string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %#v, want %#v", name, got, want)
	}
}
