package emit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zzuf/GoWin-AFO/generator/internal/metadata"
	"github.com/zzuf/GoWin-AFO/generator/internal/normalize"
)

func fixtureInventory(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestGoldenAndDeterministicRender(t *testing.T) {
	root := fixtureInventory(t)
	inv, err := metadata.LoadFixture(filepath.Join(root, "generator", "testdata", "e2e", "source.json"))
	if err != nil {
		t.Fatal(err)
	}
	inv.GeneratorVersion = "0.1.0"
	if err = normalize.Inventory(&inv); err != nil {
		t.Fatal(err)
	}
	overrides, err := metadata.LoadOverrides(filepath.Join(root, "overrides"))
	if err != nil {
		t.Fatal(err)
	}
	if err = metadata.ApplyOverrides(inv.Symbols, inv.Sources, overrides); err != nil {
		t.Fatal(err)
	}
	if err = normalize.Inventory(&inv); err != nil {
		t.Fatal(err)
	}
	a, err := Render(inv, "example.com/fixture")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(inv, "example.com/fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Files) != len(b.Files) {
		t.Fatal("file count changed across identical renders")
	}
	for name, left := range a.Files {
		if !bytes.Equal(left, b.Files[name]) {
			t.Fatalf("nondeterministic output %s", name)
		}
		if strings.Contains(string(left), root) {
			t.Fatalf("local absolute path leaked into %s", name)
		}
	}
	got := a.Files["bindings/generated/windows_win32_foundation/zcallbacks_generated.go"]
	want, err := os.ReadFile(filepath.Join(root, "generator", "testdata", "golden", "zcallbacks_generated.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden drift\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	renamed, err := Render(inv, "example.com/renamed-module")
	if err != nil {
		t.Fatal(err)
	}
	functions := renamed.Files["bindings/generated/windows_win32_foundation/zfunctions_windows.go"]
	if !bytes.Contains(functions, []byte(`"example.com/renamed-module/runtime/winabi"`)) {
		t.Fatal("module-path setting was not propagated to generated imports")
	}
}
