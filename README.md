# gide

Pure Go project descriptions, IDE generators, and IDE toolchain discovery.

## Packages

- `denv`: declarative packages, projects, configurations, targets, and source paths.
- `msdev`: typed Visual Studio configuration shared by SDK adapters and generators.
- `msdev/msvc`: Visual Studio and MSVC discovery.
- `msdev/winsdk`: Windows SDK discovery.

`gide` depends on `gcore` and does not depend on `ccode` or `csdk`.

## Visual Studio generation

`msdev.Generate` writes deterministic Visual Studio 2022 solutions, C++
projects, and filter files. A workspace combines `denv.DevProject`
declarations with typed `msdev.VsConfiguration` values supplied by an SDK
adapter such as `csdk`:

```go
err := msdev.Generate(msdev.Workspace{
	Name:      "example",
	OutputDir: "target/msdev",
	Projects: []msdev.Project{{
		Definition: project,
		Configurations: []msdev.ProjectConfiguration{{
			BuildConfig: denv.NewDebugDevConfig(),
			Options:     debugOptions,
		}},
	}},
})
```

The generator expands project source globs, merges matching `denv.DevConfig`
include paths, defines, and libraries, and translates typed compiler and linker
settings at the MSBuild boundary.

## Testing

```sh
GO111MODULE=off go test ./...
```

Windows SDK installation discovery is exercised only on Windows. Other package tests run on all supported hosts.
