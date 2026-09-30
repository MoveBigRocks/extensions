package catalog_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Catalog consumers select install scope before loading the bundle. Keep that
// discovery contract consistent with every public source manifest.
func TestPublicCatalogMatchesSourceManifests(t *testing.T) {
	var catalog struct {
		Publisher string              `json:"publisher"`
		Bundles   []map[string]string `json:"bundles"`
	}
	readJSON(t, "public-bundles.json", &catalog)
	paths, err := filepath.Glob("../*/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no public source manifests found")
	}
	manifests := make(map[string]map[string]json.RawMessage, len(paths))
	for _, path := range paths {
		var manifest map[string]json.RawMessage
		readJSON(t, path, &manifest)
		manifests[filepath.Base(filepath.Dir(path))] = manifest
	}
	seen := make(map[string]bool)
	for _, bundle := range catalog.Bundles {
		slug := bundle["slug"]
		if slug == "" || seen[slug] {
			t.Errorf("empty or duplicate catalog slug %q", slug)
		}
		seen[slug] = true
		manifest, ok := manifests[slug]
		if !ok {
			t.Errorf("catalog bundle %q has no source manifest", slug)
			continue
		}
		if bundle["sourceDir"] != slug {
			t.Errorf("%s: sourceDir must name its source directory", slug)
		}
		for _, field := range []string{"slug", "name", "kind", "scope", "risk", "runtimeClass", "publisher"} {
			var actual string
			if err := json.Unmarshal(manifest[field], &actual); err != nil {
				t.Fatalf("%s.%s: %v", slug, field, err)
			}
			want := bundle[field]
			if field == "publisher" {
				want = catalog.Publisher
			}
			if want == "" || actual != want {
				t.Errorf("%s.%s: catalog=%q manifest=%q", slug, field, want, actual)
			}
		}
	}
	for slug := range manifests {
		if !seen[slug] {
			t.Errorf("public source %q is missing from the catalog", slug)
		}
	}
}

func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}
