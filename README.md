# gide

Pure Go project descriptions, IDE generators, and IDE toolchain discovery.

## Packages

- `denv`: declarative packages, projects, configurations, targets, and source paths.
- `graph`: shared workspace, project, file, and configuration graph construction.
- `msdev`: typed Visual Studio configuration shared by SDK adapters and generators.
- `xcode`: deterministic Xcode workspace and project generation.
- `msdev/msvc`: Visual Studio and MSVC discovery.
- `msdev/winsdk`: Windows SDK discovery.

## Dependencies

- `gcore`

## Visual Studio generation

`msdev.GeneratePackage` resolves package variables into typed configurations
and writes deterministic Visual Studio 2022 solutions, C++ projects, and
filter files:

```go
err := msdev.GeneratePackage(pkg, msdev.WorkspaceOptions{
	OutputDir:    "target/msdev",
	BuildTarget: denv.BuildTargetWindowsX64,
	BuildConfigs: []denv.BuildConfig{
		denv.NewDebugConfig(),
		denv.NewReleaseConfig(),
	},
})
```

`msdev.NewConfigurationFromVars` exposes strict `gcore.Vars` parsing, while
`msdev.BuildWorkspace` exposes the intermediate workspace for customization.
The generator expands project source globs, merges matching `denv.DevConfig`
include paths, defines, and libraries, and translates typed compiler and linker
settings at the MSBuild boundary.

## Xcode generation

Xcode exposes the same package-level interface and shared workspace options:

```go
err := xcode.GeneratePackage(pkg, xcode.WorkspaceOptions{
	OutputDir:       "target/xcode",
	BuildTarget:     denv.BuildTargetMacArm64,
	BuildConfigs:    []denv.BuildConfig{denv.NewDebugConfig()},
	OSTargetVersion: "14.0",
	Xcode: graph.WorkspaceOptionsXcode{
		CppStd: graph.CppStd20,
	},
})
```

Both backends also provide `BuildWorkspace` when callers need to inspect or
customize the shared graph before generation. Backend-specific settings stay
under the typed `VisualStudio` and `Xcode` option fields.

## Testing

```sh
GO111MODULE=off go test ./...
```

Windows SDK installation discovery is exercised only on Windows. Other package tests run on all supported hosts.
