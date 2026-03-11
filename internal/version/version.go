package version

// Set via ldflags at build time by goreleaser.
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)
