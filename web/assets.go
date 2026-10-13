package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var files embed.FS

func Assets() fs.FS {
	out, e := fs.Sub(files, "dist")
	if e != nil {
		panic(e)
	}
	return out
}
