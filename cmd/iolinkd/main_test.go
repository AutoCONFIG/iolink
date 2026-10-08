package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"git.hyhy.fun/rsplab/iolink/internal/appapi"
)

func TestFailureOutputHidesUntrustedErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"configuration", &startupError{errors.New("IOLINK_LOG_LEVEL must be debug, info, warn or error")}, "IOLINK_LOG_LEVEL must be debug, info, warn or error\n"},
		{"runtime", errors.New("postgres://user:secret@host/database"), "iolinkd stopped; check configuration and diagnostics\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			printFailure(&output, tc.err)
			if output.String() != tc.want {
				t.Fatalf("unexpected failure output: %q", output.String())
			}
		})
	}
}

func TestUnknownAPIDoesNotServeSPA(t *testing.T) {
	h := spaHandler(fstest.MapFS{"index.html": {Data: []byte("app")}})
	for _, path := range []string{"/api/v9/missing", "/admin/v1missing", "/user/v1missing"} {
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

func TestProductionMuxMountsBothApplicationAPIVersions(t *testing.T) {
	root := http.NewServeMux()
	api := appapi.New(appapi.Config{}, appapi.Deps{}, nil)
	mountApplicationAPI(root, api.Routes())
	root.Handle("/", spaHandler(fstest.MapFS{"index.html": {Data: []byte("app")}}))
	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/ponds"},
		{http.MethodPost, "/api/v2/devices/test/telemetry"},
		{http.MethodGet, "/api/v2/devices/test/model/latest"},
		{http.MethodGet, "/api/v2/devices/test/history"},
	} {
		response := httptest.NewRecorder()
		root.ServeHTTP(response, httptest.NewRequest(request.method, request.path, nil))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s did not reach auth middleware: status=%d", request.method, request.path, response.Code)
		}
	}
}
