package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateV2Fixtures writes the benchmark fixtures as JSON files for use with curl/hyperfine.
// Run with: go test -run=TestGenerateV2Fixtures ./...
func TestGenerateV2Fixtures(t *testing.T) {
	dir := filepath.Join("fixtures", "v2")
	os.MkdirAll(dir, 0o755)

	fixtures := []struct {
		name string
		data any
	}{
		{"typecheck-trivial.json", TypecheckV2Request{Files: singleFileFixture(fixtureTrivial), EntryPoints: []string{"/index.tsx"}, Version: benchVersion}},
		{"typecheck-small.json", TypecheckV2Request{Files: singleFileFixture(fixtureSmallComponent), EntryPoints: []string{"/index.tsx"}, Version: benchVersion}},
		{"typecheck-medium.json", TypecheckV2Request{Files: singleFileFixture(fixtureMediumComponent), EntryPoints: []string{"/index.tsx"}, Version: benchVersion}},
		{"typecheck-multifile.json", TypecheckV2Request{Files: fixtureMultiFile, EntryPoints: []string{"/index.tsx"}, Version: benchVersion}},
		{"build-trivial.json", BuildV2Request{Files: singleFileFixture(fixtureTrivial), EntryPoint: "/index.tsx", Version: benchVersion}},
		{"build-small.json", BuildV2Request{Files: singleFileFixture(fixtureSmallComponent), EntryPoint: "/index.tsx", Version: benchVersion}},
		{"build-medium.json", BuildV2Request{Files: singleFileFixture(fixtureMediumComponent), EntryPoint: "/index.tsx", Version: benchVersion}},
		{"build-multifile.json", BuildV2Request{Files: fixtureMultiFile, EntryPoint: "/index.tsx", Version: benchVersion}},
	}

	for _, f := range fixtures {
		data, err := json.Marshal(f.data)
		if err != nil {
			t.Fatalf("Failed to marshal %s: %v", f.name, err)
		}
		path := filepath.Join(dir, f.name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("Failed to write %s: %v", f.name, err)
		}
		t.Logf("Wrote %s (%d bytes)", path, len(data))
	}
}
