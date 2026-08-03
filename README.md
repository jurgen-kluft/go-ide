# gide

Pure Go project descriptions, IDE generators, and IDE toolchain discovery.

## Packages

- `denv`: declarative packages, projects, configurations, targets, and source paths.
- `msdev`: typed Visual Studio configuration shared by SDK adapters and generators.
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

## Testing

```sh
GO111MODULE=off go test ./...
```

Windows SDK installation discovery is exercised only on Windows. Other package tests run on all supported hosts.
