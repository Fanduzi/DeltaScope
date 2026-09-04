// Package deltascope verifies the public build-version helper.
// input: synthetic Go module and VCS build-info fixtures plus the live test binary
// output: assertions that untagged, devel, and pseudo-version builds do not claim DefaultVersion as the sole version, tagged versions are preserved, and DefaultVersion is used only when build info is absent
// pos: public version-string seam for CLI, server, and MCP binaries
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestReportedVersionFromBuildInfo(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
		info *debug.BuildInfo
		want string
	}{
		{
			name: "absent build info uses DefaultVersion",
			ok:   false,
			want: DefaultVersion,
		},
		{
			name: "nil build info uses DefaultVersion",
			ok:   true,
			info: nil,
			want: DefaultVersion,
		},
		{
			name: "tagged main module reports the release tag",
			ok:   true,
			info: &debug.BuildInfo{
				Main: debug.Module{Path: modulePath, Version: "v0.510.3"},
			},
			want: "v0.510.3",
		},
		{
			name: "pseudo-version main module reports the pseudo-version",
			ok:   true,
			info: &debug.BuildInfo{
				Main: debug.Module{
					Path:    modulePath,
					Version: "v0.510.4-0.20260904112233-abcdefabcdef",
				},
			},
			want: "v0.510.4-0.20260904112233-abcdefabcdef",
		},
		{
			name: "dirty pseudo-version main module reports the module version",
			ok:   true,
			info: &debug.BuildInfo{
				Main: debug.Module{
					Path:    modulePath,
					Version: "v0.510.4-0.20260904112233-abcdefabcdef+dirty",
				},
			},
			want: "v0.510.4-0.20260904112233-abcdefabcdef+dirty",
		},
		{
			name: "devel main module with revision does not claim DefaultVersion",
			ok:   true,
			info: &debug.BuildInfo{
				Main: debug.Module{Path: modulePath, Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abcdefabcdefdeadbeef"},
					{Key: "vcs.modified", Value: "false"},
				},
			},
			want: "devel-abcdefabcdef",
		},
		{
			name: "devel main module with short revision keeps the full hash",
			ok:   true,
			info: &debug.BuildInfo{
				Main: debug.Module{Path: modulePath, Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abc1234"},
				},
			},
			want: "devel-abc1234",
		},
		{
			name: "dirty devel main module appends dirty",
			ok:   true,
			info: &debug.BuildInfo{
				Main: debug.Module{Path: modulePath, Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abcdefabcdefdeadbeef"},
					{Key: "vcs.modified", Value: "true"},
				},
			},
			want: "devel-abcdefabcdef-dirty",
		},
		{
			name: "devel main module without vcs reports devel",
			ok:   true,
			info: &debug.BuildInfo{
				Main: debug.Module{Path: modulePath, Version: "(devel)"},
			},
			want: "devel",
		},
		{
			name: "empty main version with vcs reports devel revision",
			ok:   true,
			info: &debug.BuildInfo{
				Main: debug.Module{Path: modulePath, Version: ""},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abcdefabcdefdeadbeef"},
				},
			},
			want: "devel-abcdefabcdef",
		},
		{
			name: "dependency tagged version is used when this module is not main",
			ok:   true,
			info: &debug.BuildInfo{
				Main: debug.Module{Path: "example.com/consumer", Version: "v1.2.3"},
				Deps: []*debug.Module{
					{Path: modulePath, Version: "v0.500.0"},
				},
			},
			want: "v0.500.0",
		},
		{
			name: "dependency pseudo-version is used when this module is not main",
			ok:   true,
			info: &debug.BuildInfo{
				Main: debug.Module{Path: "example.com/consumer", Version: "v1.2.3"},
				Deps: []*debug.Module{
					{Path: modulePath, Version: "v0.510.4-0.20260904112233-abcdefabcdef"},
				},
			},
			want: "v0.510.4-0.20260904112233-abcdefabcdef",
		},
		{
			name: "dependency devel does not claim DefaultVersion",
			ok:   true,
			info: &debug.BuildInfo{
				Main: debug.Module{Path: "example.com/consumer", Version: "(devel)"},
				Deps: []*debug.Module{
					{Path: modulePath, Version: "(devel)"},
				},
			},
			want: "devel",
		},
		{
			name: "unknown module without this dependency uses DefaultVersion",
			ok:   true,
			info: &debug.BuildInfo{
				Main: debug.Module{Path: "example.com/consumer", Version: "v1.2.3"},
			},
			want: DefaultVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reportedVersion(tt.info, tt.ok)
			if got != tt.want {
				t.Fatalf("reportedVersion() = %q, want %q", got, tt.want)
			}
			if isUntaggedVersion(tt.info, tt.ok) && got == DefaultVersion {
				t.Fatalf("untagged/devel/pseudo-version build claimed DefaultVersion %q as the sole version", DefaultVersion)
			}
		})
	}
}

func TestReportedVersionLiveBuildDoesNotClaimDefaultReleaseWhenUntagged(t *testing.T) {
	got := ReportedVersion()
	if got == "" {
		t.Fatal("ReportedVersion() must not be empty")
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		if got != DefaultVersion {
			t.Fatalf("absent live build info: ReportedVersion() = %q, want DefaultVersion %q", got, DefaultVersion)
		}
		return
	}

	if !isUntaggedVersion(info, true) {
		return
	}
	if got == DefaultVersion {
		t.Fatalf("live untagged/devel/pseudo-version build claimed DefaultVersion %q as the sole version; build info version=%q", DefaultVersion, info.Main.Version)
	}
	if want := reportedVersion(info, true); got != want {
		t.Fatalf("ReportedVersion() = %q, want %q from live build info", got, want)
	}
}

func isUntaggedVersion(info *debug.BuildInfo, ok bool) bool {
	if !ok || info == nil {
		return false
	}
	version, _ := deltascopeModule(info)
	if version == "(devel)" || version == "" && info.Main.Path == modulePath {
		return true
	}
	return isPseudoVersion(version)
}

func isPseudoVersion(version string) bool {
	// Go pseudo-versions embed yyyymmddhhmmss before the VCS hash.
	parts := strings.Split(version, "-")
	if len(parts) < 3 {
		return false
	}
	stamp := parts[len(parts)-2]
	if len(stamp) >= 14 {
		stamp = stamp[len(stamp)-14:]
	}
	if len(stamp) != 14 {
		return false
	}
	for _, c := range stamp {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
