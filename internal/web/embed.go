// Package web embeds the compiled admin frontend so iolinkd ships as a
// single binary. go:embed cannot reach outside this package directory, so
// scripts/embed-frontend.sh mirrors the web/ submodule's dist/ here before
// every build; without a frontend build it falls back to a placeholder page.
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
