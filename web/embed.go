// Package web embeds the compiled admin frontend so iolinkd ships as a
// single binary. The frontend engineer builds web/admin → web/admin/dist;
// the placeholder index.html keeps the Go build green before that happens.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:admin/dist
var embedded embed.FS

// Admin returns the admin console filesystem rooted at its index.html.
func Admin() (fs.FS, error) {
	sub, err := fs.Sub(embedded, "admin/dist")
	if err != nil {
		return nil, err
	}
	return sub, nil
}
