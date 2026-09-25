package main

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestUnknownAPIDoesNotServeSPA(t *testing.T) {
	h := spaHandler(fstest.MapFS{"index.html": {Data: []byte("app")}})
	for _, path := range []string{"/api/v9/missing", "/admin/v1missing"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 || !strings.Contains(w.Header().Get("Content-Type"), "json") {
			t.Fatal(path, w.Code, w.Body)
		}
	}
}
func TestCLIHelpAndInvalidCommand(t *testing.T) {
	if err := run([]string{"--help"}, strings.NewReader(""), io.Discard); err != nil {
		t.Fatal(err)
	}
	if run([]string{"migrate", "typo"}, strings.NewReader(""), io.Discard) == nil {
		t.Fatal("unknown command accepted")
	}
}
