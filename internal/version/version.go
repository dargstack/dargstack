package version

import "runtime/debug"

// Set via ldflags at build time by goreleaser.
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

func init() {
	if Version != "dev" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		Version = v
	}
}
