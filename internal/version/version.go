// Package version is populated at build time via -ldflags.
package version

import (
	"fmt"
	"runtime"
)

var (
	Version   = "dev"
	Revision  = "unknown"
	Branch    = "unknown"
	BuildUser = "unknown"
	BuildDate = "unknown"
)

func GoVersion() string { return runtime.Version() }

func Print(name string) string {
	return fmt.Sprintf("%s, version %s (branch: %s, revision: %s)\n  build user: %s\n  build date: %s\n  go version: %s",
		name, Version, Branch, Revision, BuildUser, BuildDate, GoVersion())
}

type Info struct {
	Version, Revision, Branch, BuildUser, BuildDate, GoVersion string
}

func Snapshot() Info {
	return Info{
		Version:   Version,
		Revision:  Revision,
		Branch:    Branch,
		BuildUser: BuildUser,
		BuildDate: BuildDate,
		GoVersion: GoVersion(),
	}
}
