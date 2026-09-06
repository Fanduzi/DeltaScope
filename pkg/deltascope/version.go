// Package deltascope exposes the stable public audit API.
// input: build metadata consumers and public version/logo queries
// output: reported build version from Go module/VCS info, DefaultVersion fallback, and ASCII logo values for CLIs and services
// pos: public package metadata alongside the stable audit entrypoint
// note: if this file changes, update this header and module README.md.
package deltascope

import "runtime/debug"

const (
	// DefaultVersion is the repository's current default semantic version.
	// It is used only when Go build information is absent.
	DefaultVersion = "v0.510.4"

	// Logo is the canonical ASCII DeltaScope banner used by human-facing commands.
	Logo = "    ____       ____        _____                     \n" +
		"   / __ \\___  / / /_____ _/ ___/_________  ____  ___ \n" +
		"  / / / / _ \\/ / __/ __ `/\\__ \\/ ___/ __ \\/ __ \\/ _ \\\n" +
		" / /_/ /  __/ / /_/ /_/ /___/ / /__/ /_/ / /_/ /  __/\n" +
		"/_____/\\___/_/\\__/\\__,_//____/\\___/\\____/ .___/\\___/ \n" +
		"                                       /_/           "

	modulePath = "github.com/Fanduzi/DeltaScope"
)

// ReportedVersion returns the version string for this DeltaScope build.
// Tagged and pseudo-version module builds report that module version.
// Devel source builds report VCS revision as devel-<rev> or devel-<rev>-dirty.
// DefaultVersion is used only when build information is absent.
func ReportedVersion() string {
	info, ok := debug.ReadBuildInfo()
	return reportedVersion(info, ok)
}

func reportedVersion(info *debug.BuildInfo, ok bool) string {
	if !ok || info == nil {
		return DefaultVersion
	}
	version, isMain := deltascopeModule(info)
	if version != "" && version != "(devel)" {
		return version
	}
	if isMain {
		return develVersion(info)
	}
	if version == "(devel)" {
		return "devel"
	}
	return DefaultVersion
}

func deltascopeModule(info *debug.BuildInfo) (version string, isMain bool) {
	if info.Main.Path == modulePath {
		return info.Main.Version, true
	}
	for _, dep := range info.Deps {
		if dep != nil && dep.Path == modulePath {
			return dep.Version, false
		}
	}
	return "", false
}

func develVersion(info *debug.BuildInfo) string {
	revision, modified := "", ""
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}
	if revision == "" {
		return "devel"
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if modified == "true" {
		return "devel-" + revision + "-dirty"
	}
	return "devel-" + revision
}
