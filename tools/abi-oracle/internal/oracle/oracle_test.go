package oracle

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixturePath(name string) string {
	return filepath.Join("..", "..", "testdata", name)
}

func TestGenerateGoldenAndDeterministic(t *testing.T) {
	manifest, err := LoadManifest(fixturePath("probe-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := Generate(manifest)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("identical manifests produced different C++")
	}
	want, err := os.ReadFile(fixturePath("probe-amd64.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, want) {
		t.Fatal("generated C++ differs from probe-amd64.cpp; regenerate the golden after review")
	}
	text := string(first)
	if strings.Contains(text, `C:\\`) || strings.Contains(text, "Generated at") {
		t.Fatal("generated C++ contains a local path or timestamp")
	}
}

func TestGenerateSortsProbeOrder(t *testing.T) {
	manifest, err := LoadManifest(fixturePath("probe-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := Generate(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for left, right := 0, len(manifest.Records)-1; left < right; left, right = left+1, right-1 {
		manifest.Records[left], manifest.Records[right] = manifest.Records[right], manifest.Records[left]
	}
	for left, right := 0, len(manifest.Functions)-1; left < right; left, right = left+1, right-1 {
		manifest.Functions[left], manifest.Functions[right] = manifest.Functions[right], manifest.Functions[left]
	}
	got, err := Generate(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("probe declaration order affected generated C++")
	}
}

func TestManifestRejectsDuplicateIDAndUnsafeHeader(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: 1,
		SourceID:      "test",
		SDKVersion:    "10.0.26100.0",
		Profile:       "windows-desktop",
		Architecture:  "amd64",
		Includes:      []string{"../private.h"},
		Values: []ValueProbe{
			{ID: "duplicate", NativeName: "A", Expression: "A"},
			{ID: "duplicate", NativeName: "B", Expression: "B"},
		},
	}
	err := manifest.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicate probe id") || !strings.Contains(err.Error(), "unsafe include") {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestCompareReportsABIPathAndIgnoresCompiler(t *testing.T) {
	expected, err := LoadResult(fixturePath("windows-sdk-10.0.26100-amd64.json"))
	if err != nil {
		t.Fatal(err)
	}
	actual := expected
	actual.Compiler = CompilerResult{Family: "clang-cl", Version: "18.1.6"}
	if comparison := Compare(expected, actual); !comparison.Equal {
		t.Fatalf("compiler provenance caused ABI mismatch: %#v", comparison.Mismatches)
	}
	actual.Records = append([]RecordResult(nil), expected.Records...)
	actual.Records[0].Size++
	comparison := Compare(expected, actual)
	if comparison.Equal || len(comparison.Mismatches) != 1 {
		t.Fatalf("comparison = %#v", comparison)
	}
	if comparison.Mismatches[0].Path != "/records/0/size" {
		t.Fatalf("mismatch path = %q", comparison.Mismatches[0].Path)
	}
}

func TestFixtureResultIsValid(t *testing.T) {
	for _, name := range []string{"windows-sdk-10.0.26100-386.json", "windows-sdk-10.0.26100-amd64.json"} {
		if _, err := LoadResult(fixturePath(name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
