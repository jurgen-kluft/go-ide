package xcode

import (
	"fmt"
	"strings"

	corepkg "github.com/jurgen-kluft/go-core"
	"github.com/jurgen-kluft/go-ide/denv"
	"github.com/jurgen-kluft/go-ide/graph"
)

func NewConfigurationFromVars(vars *corepkg.Vars, buildTarget denv.BuildTarget, buildConfig denv.BuildConfig) (*XcodeConfiguration, CppStdType, error) {
	if vars == nil {
		return nil, CppStdUnknown, fmt.Errorf("cannot create Xcode configuration from nil vars")
	}
	resolved := vars.Copy()
	resolved.FinalResolve()
	configuration := graph.NewXcodeConfiguration(buildTarget, buildConfig)

	for _, value := range xcodeValuesOf(resolved, "build.defines") {
		value = strings.TrimPrefix(value, "-D")
		if value != "" && value != "-D" {
			configuration.CppDefines.Add(value)
		}
	}
	for _, key := range []string{"compiler.flags", "compiler.warning_flags", "compiler.floating_point_flags", "compiler.debug_flags"} {
		for _, value := range xcodeValuesOf(resolved, key) {
			configuration.CppFlags.Add(value)
		}
	}
	for _, value := range append(xcodeValuesOf(resolved, "build.includes"), xcodeValuesOf(resolved, "INCLUDE")...) {
		configuration.IncludeDirs.Add(denv.PinnedPath{Root: value})
	}
	for _, key := range []string{"compiler.c.link.flags", "compiler.c.link.extra_flags"} {
		for _, value := range xcodeValuesOf(resolved, key) {
			configuration.LinkFlags.Add(value)
		}
	}
	for _, value := range append(xcodeValuesOf(resolved, "library.paths"), xcodeValuesOf(resolved, "LIBPATH")...) {
		configuration.LinkFlags.Add("-L" + value)
	}
	for _, value := range append(xcodeValuesOf(resolved, "library.files"), xcodeValuesOf(resolved, "build.extra_libs")...) {
		configuration.LinkFlags.Add(value)
	}

	cppStd, err := parseXcodeCppStandard(resolved.GetFirstOrEmpty("build.cpp_standard"))
	if err != nil {
		return nil, CppStdUnknown, err
	}
	return configuration, cppStd, nil
}

func mergeXcodeDevConfig(configuration *XcodeConfiguration, config *denv.DevConfig) {
	if config.Defines != nil {
		for _, define := range config.Defines.Values {
			configuration.CppDefines.Add(define)
		}
	}
	for _, includeDir := range config.IncludeDirs {
		configuration.IncludeDirs.Add(includeDir)
	}
	for _, libraryPath := range config.LibPaths {
		configuration.LinkFlags.Add("-L" + libraryPath.String())
	}
	for _, library := range config.Libs {
		configuration.LinkFlags.Add(library)
	}
}

func parseXcodeCppStandard(value string) (CppStdType, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.TrimPrefix(normalized, "-std=")
	normalized = strings.TrimPrefix(normalized, "gnu++")
	normalized = strings.TrimPrefix(normalized, "c++")
	switch normalized {
	case "":
		return CppStdUnknown, nil
	case "11":
		return CppStd11, nil
	case "14":
		return CppStd14, nil
	case "17":
		return CppStd17, nil
	case "20":
		return CppStd20, nil
	case "latest", "2b", "23":
		return CppStdLatest, nil
	default:
		return CppStdUnknown, fmt.Errorf("unsupported Xcode C++ standard %q", value)
	}
}

func xcodeValuesOf(vars *corepkg.Vars, key string) []string {
	values, _ := vars.Get(key)
	return values
}
