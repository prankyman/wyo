// Package version holds the version of the wyo server.
package version

// Version is the release version. The build stamps it with
// -ldflags "-X github.com/prankyman/wyo/internal/version.Version=<v>"; an unstamped build reads "dev".
var Version = "dev"
