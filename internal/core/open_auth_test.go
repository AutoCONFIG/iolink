package core

import "testing"

func TestCanonicalOpenQuery(t *testing.T) {
	got, err := CanonicalOpenQuery("b=two&a=hello%20world&a=&b=one")
	if err != nil { t.Fatal(err) }
	if got != "a=&a=hello%20world&b=one&b=two" { t.Fatalf("canonical query = %q", got) }
}

func TestCanonicalOpenPathRejectsAmbiguity(t *testing.T) {
	for _, path := range []string{"/open/v1/../ponds", "/open/v1/%2e%2e/ponds", "/open/v1/%2fponds", "/open/v1\\ponds"} {
		if _, err := CanonicalOpenPath(path); err == nil { t.Fatalf("accepted ambiguous path %q", path) }
	}
}

func TestCanonicalOpenPathKeepsEncodedSafePath(t *testing.T) {
	got, err := CanonicalOpenPath("/open/v1/ponds/%E4%B8%AD")
	if err != nil { t.Fatal(err) }
	if got != "/open/v1/ponds/%E4%B8%AD" { t.Fatalf("path = %q", got) }
}
