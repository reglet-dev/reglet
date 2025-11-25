// Package version provides build version information for Reglet.
package version

import "runtime"

var (
	// Version is the semantic version (set by build flags)
	Version = "dev"
	// Commit is the git commit hash (set by build flags)
	Commit = "unknown"
	// BuildDate is the build date (set by build flags)
	BuildDate = "unknown"
)

// Info contains version and build information
type Info struct {
	Version   string
	Commit    string
	BuildDate string
	GoVersion string
	Platform  string
}

// Get returns the version information
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// String returns a formatted version string
func (i Info) String() string {
	return i.Version
}

// Full returns a detailed version string with all build information
func (i Info) Full() string {
	return i.Version + " (" + i.Commit + ") built " + i.BuildDate + " " + i.GoVersion + " " + i.Platform
}
