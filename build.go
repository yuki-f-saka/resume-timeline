package main

import "runtime/debug"

// buildVersion is stamped by the release build (-ldflags "-X main.buildVersion=…").
// Builds that skip the linker flag — `go build` during development, or
// `go install`, which cannot pass one — fall back to the module metadata Go
// embeds on its own.
//
// Note this is the version of the tool, unrelated to the Version type in
// version.go, which is one version of the document being compared.
var buildVersion = ""

// versionString is what -version prints: the release tag when there is one,
// otherwise whatever the toolchain recorded about this build.
func versionString() string {
	if buildVersion != "" {
		return buildVersion
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	// go install github.com/…@v0.1.0 records the tag here; a local build
	// records a pseudo-version, or "(devel)" when the toolchain could not
	// work one out — then the revision below is all there is.
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	rev, dirty := "", false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "devel"
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	if dirty {
		rev += "-dirty"
	}
	return "devel-" + rev
}
