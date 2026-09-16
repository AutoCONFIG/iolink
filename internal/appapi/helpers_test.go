package appapi

import (
	"io"
	"log/slog"
	"strings"
)

func testLogger() *slog.Logger { return slog.Default() }

func bytesReader(s string) io.Reader { return strings.NewReader(s) }
