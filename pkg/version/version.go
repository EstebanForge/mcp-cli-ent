package version

import (
	"fmt"
	"runtime"
)

// Version information
var (
	Version   = "dev"
	Commit    = "unknown"
	Date      = "unknown"
	GoVersion = runtime.Version()
)

// String returns the version string
func String() string {
	return fmt.Sprintf("%s (commit: %s, built: %s, go: %s)", Version, Commit, Date, GoVersion)
}

// PrintVersion prints version information to stdout
func PrintVersion() {
	fmt.Printf("mcp-cli-ent %s\n", String())
}
