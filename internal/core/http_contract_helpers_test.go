package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// assertHTTPContract sends response data over stdin only; failure output never contains the body.
func assertHTTPContract(t *testing.T, spec, method, route string, status int, contentType string, raw []byte) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("HTTP contract helper could not locate repository")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
	python := os.Getenv("IOLINK_CONTRACT_PYTHON")
	if python == "" {
		python = filepath.Join(root, ".venv/contracts/bin/python")
	}
	if _, err := exec.LookPath(python); err != nil {
		t.Fatal("HTTP contract Python unavailable; run make docs-tools or set IOLINK_CONTRACT_PYTHON")
	}
	if len(raw) > 0 && !json.Valid(raw) {
		t.Fatalf("HTTP contract response is not JSON: %s %s status=%d", method, route, status)
	}
	input := struct {
		Spec        string          `json:"spec"`
		Method      string          `json:"method"`
		Route       string          `json:"route"`
		Status      int             `json:"status"`
		ContentType string          `json:"content_type"`
		HasBody     bool            `json:"has_body"`
		Body        json.RawMessage `json:"body"`
	}{spec, method, route, status, contentType, len(raw) > 0, nil}
	if input.HasBody {
		input.Body = raw
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal("HTTP contract input could not be encoded")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, python, filepath.Join(root, "scripts/validate_http_response.py"))
	command.Stdin = bytes.NewReader(payload)
	output, runErr := command.Output()
	var result struct {
		Valid  bool `json:"valid"`
		Issues []struct {
			Keyword    string `json:"keyword"`
			SchemaPath []any  `json:"schema_path"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("HTTP contract validator unavailable or malformed result; run make docs-tools: %s %s status=%d", method, route, status)
	}
	if runErr != nil || !result.Valid || len(result.Issues) != 0 {
		t.Fatalf("HTTP contract rejected response: %s %s status=%d issues=%v", method, route, status, result.Issues)
	}
}
