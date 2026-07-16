package main

import (
	"embed"
	"io/fs"
)

// The built frontend (gantry build fills dist/); embedded so the app
// ships as one exe.
//
//go:embed all:dist
var distFS embed.FS

func dist() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
