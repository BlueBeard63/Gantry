package main

import (
	"embed"
	"io/fs"
)

// The built frontend (gantry build/dev fill webdist/); embedded so the
// app ships as one exe. Release artifacts land in dist/ per os/arch.
//
//go:embed all:webdist
var distFS embed.FS

func dist() fs.FS {
	sub, err := fs.Sub(distFS, "webdist")
	if err != nil {
		panic(err)
	}
	return sub
}
