package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// Admin returns the admin console filesystem rooted at its index.html.
func Admin() (fs.FS, error) {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, err
	}
	return sub, nil
}
