package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type module struct{ Path, Version, Dir string }
type npmPackage struct{ Name, Version, License string }
type lockfile struct {
	Packages map[string]npmPackage `json:"packages"`
}
type spdxPackage struct {
	ID        string `json:"SPDXID"`
	Name      string `json:"name"`
	Version   string `json:"versionInfo"`
	Download  string `json:"downloadLocation"`
	Analyzed  bool   `json:"filesAnalyzed"`
	Concluded string `json:"licenseConcluded"`
	Declared  string `json:"licenseDeclared"`
	Copyright string `json:"copyrightText"`
}
type document struct {
	Version   string `json:"spdxVersion"`
	License   string `json:"dataLicense"`
	ID        string `json:"SPDXID"`
	Name      string `json:"name"`
	Namespace string `json:"documentNamespace"`
	Creation  struct {
		Creators []string `json:"creators"`
		Created  string   `json:"created"`
	} `json:"creationInfo"`
	Packages []spdxPackage `json:"packages"`
}

func main() {
	out := flag.String("out", "", "metadata output directory")
	revision := flag.String("revision", "", "source revision")
	flag.Parse()
	if err := run(*out, *revision, os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, "dependency inventory failed:", err)
		os.Exit(1)
	}
}

func run(out, revision string, input io.Reader) error {
	if out == "" || revision == "" {
		return errors.New("out and revision required")
	}
	doc := document{Version: "SPDX-2.3", License: "CC0-1.0", ID: "SPDXRef-DOCUMENT", Name: "iolink-offline-source-dependencies", Namespace: "https://iolink.invalid/sbom/" + revision}
	doc.Creation.Creators = []string{"Tool: iolink-offline-sbom"}
	doc.Creation.Created = time.Now().UTC().Format(time.RFC3339)
	var notices strings.Builder
	notices.WriteString("Source dependency notices (Go and web lockfile, including build dependencies).\nLicenseConcluded remains NOASSERTION: no legal determination is made.\nImage OS and database packages are identified by image manifests, not analyzed here.\n\n")
	appendPackage := func(name, version, declared, dir string) error {
		if version == "" {
			version = "NOASSERTION"
		}
		if declared == "" {
			declared = "NOASSERTION"
		}
		doc.Packages = append(doc.Packages, spdxPackage{ID: fmt.Sprintf("SPDXRef-Package-%d", len(doc.Packages)+1), Name: name, Version: version, Download: "NOASSERTION", Concluded: "NOASSERTION", Declared: declared, Copyright: "NOASSERTION"})
		fmt.Fprintf(&notices, "=== %s %s (declared: %s) ===\n", name, version, declared)
		return appendNotices(&notices, dir)
	}
	decoder := json.NewDecoder(input)
	for {
		var item module
		if err := decoder.Decode(&item); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("Go module JSON: %w", err)
		}
		if err := appendPackage("go:"+item.Path, item.Version, "", item.Dir); err != nil {
			return err
		}
	}
	raw, err := os.ReadFile("web/package-lock.json")
	if err != nil {
		return err
	}
	var lock lockfile
	if err := json.Unmarshal(raw, &lock); err != nil {
		return err
	}
	paths := make([]string, 0, len(lock.Packages))
	for path := range lock.Packages {
		if path != "" {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		item := lock.Packages[path]
		name := item.Name
		if name == "" {
			_, name, _ = strings.Cut(path, "node_modules/")
		}
		if err := appendPackage("npm:"+name, item.Version, item.License, filepath.Join("web", path)); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(out, "licenses.txt"), []byte(notices.String()), 0644); err != nil {
		return err
	}
	raw, err = json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "sbom.spdx.json"), append(raw, '\n'), 0644)
}

func appendNotices(out *strings.Builder, dir string) error {
	if dir == "" {
		out.WriteString("Notice source unavailable.\n\n")
		return nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		out.WriteString("Not installed on this build platform; consult declared upstream license.\n\n")
		return nil
	}
	if err != nil {
		return err
	}
	found := false
	for _, entry := range entries {
		name := strings.ToUpper(entry.Name())
		if entry.IsDir() || !(strings.HasPrefix(name, "LICENSE") || strings.HasPrefix(name, "LICENCE") || strings.HasPrefix(name, "COPYING") || strings.HasPrefix(name, "NOTICE")) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "--- %s ---\n%s\n", entry.Name(), raw)
		found = true
	}
	if !found {
		out.WriteString("No root notice file found; consult upstream distribution.\n")
	}
	out.WriteByte('\n')
	return nil
}
