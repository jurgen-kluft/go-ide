package xcode

import (
	"github.com/jurgen-kluft/gide/denv"
	"github.com/jurgen-kluft/gide/graph"
)

type FileType = graph.FileType
type FileEntry = graph.FileEntry
type FileEntryDict = graph.FileEntryDict
type VirtualDirectory = graph.VirtualDirectory
type VirtualDirectories = graph.VirtualDirectories

type OrderedSettings = graph.OrderedSettings
type StringValues = graph.StringValues
type PinnedPaths = graph.PinnedPaths
type XcodeConfigurationUUIDs = graph.XcodeConfigurationUUIDs
type XcodeConfigurationResolved = graph.XcodeConfigurationResolved
type XcodeConfiguration = graph.XcodeConfiguration
type XcodeConfigurationList = graph.XcodeConfigurationList
type XcodeProjectConfig = graph.XcodeProjectConfig
type ProjectResolved = graph.ProjectResolved

const (
	FileTypeNone      = graph.FileTypeNone
	FileTypeCppSource = graph.FileTypeCppSource
	FileTypeCSource   = graph.FileTypeCSource
)

func NewFileEntry() *FileEntry {
	return graph.NewFileEntry("")
}

func NewFileEntryInit(path string) *FileEntry {
	return graph.NewFileEntry(path)
}

func NewFileEntryDict(path string) *FileEntryDict {
	return graph.NewFileEntryDict(path)
}

func NewVirtualFolders(diskPath string) *VirtualDirectories {
	return graph.NewVirtualDirectories(diskPath)
}

func NewOrderedSettings() *OrderedSettings {
	return graph.NewOrderedSettings()
}

func NewStringValues() *StringValues {
	return graph.NewStringValues()
}

func NewPinnedPaths() *PinnedPaths {
	return graph.NewPinnedPaths()
}

func NewXcodeConfiguration(buildTarget denv.BuildTarget, buildConfig denv.BuildConfig) *XcodeConfiguration {
	return graph.NewXcodeConfiguration(buildTarget, buildConfig)
}

func NewXcodeConfigurationList() *XcodeConfigurationList {
	return graph.NewXcodeConfigurationList()
}

func NewXcodeProjectConfig() *XcodeProjectConfig {
	return graph.NewXcodeProjectConfig()
}

func NewProjectResolved() *ProjectResolved {
	return graph.NewProjectResolved()
}
