// Package assets embeds the compiled React UI into the binary.
package assets

import (
	"embed"
	"io/fs"
	"net/http"
)

// UI is the embedded file system containing the compiled React app.
// The `dist` directory is produced by `npm run build` in the ui/ directory.
//
//go:embed dist
var UI embed.FS

// DistFS returns an http.FileSystem scoped to the dist/ subdirectory.
func DistFS() http.FileSystem {
	sub, err := fs.Sub(UI, "dist")
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}
