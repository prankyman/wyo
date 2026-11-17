package webui

import (
	"embed"
	"io/fs"
)

// dist holds the interface that `vite build` wrote. The all: prefix keeps the files whose names begin with an underscore,
// such as SvelteKit's _app folder, which embed would otherwise skip.
//
//go:embed all:dist
var dist embed.FS

// Embedded returns the built interface: index.html at its root, the rest of the build beside it.
func Embedded() fs.FS {
	return MustSub(dist, "dist")
}

// MustSub returns the folder dir of fsys as a file system of its own. It panics when dir is not a valid folder name,
// which is a mistake in the program and not a state it can be in at run time.
func MustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic("webui: " + err.Error())
	}

	return sub
}
