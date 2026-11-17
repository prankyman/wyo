//go:build wyotest

// Package buildinfo says whether this build was made for development and tests (the Go build tag wyotest) or is a
// release. It is the only place the tag is read; every package that behaves differently asks it.
package buildinfo

// TestBuild is true only in a build made with the wyotest tag.
const TestBuild = true
