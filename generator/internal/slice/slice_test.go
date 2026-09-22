package slice

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func projectRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRenderReconstructsCheckedInSlice(t *testing.T) {
	root := projectRoot(t)
	a, err := Render(root, "go-windows-api.local", "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(root, "go-windows-api.local", "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Files) != 14 {
		t.Fatalf("got %d files, want 13 Go files and a manifest", len(a.Files))
	}
	for path, data := range a.Files {
		want, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, want) {
			t.Errorf("rendered slice drift: %s", path)
		}
		if !bytes.Equal(data, b.Files[path]) {
			t.Errorf("nondeterministic render: %s", path)
		}
	}
}

func TestRenderUsesModulePath(t *testing.T) {
	tree, err := Render(projectRoot(t), "example.com/renamed", "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	for path, data := range tree.Files {
		if strings.HasSuffix(path, ".go") && bytes.Contains(data, []byte("go-windows-api.local/")) {
			t.Errorf("old module import in %s", path)
		}
	}
	got := tree.Files["bindings/win32/kernel32/zfunctions_kernel_windows.go"]
	if !bytes.Contains(got, []byte(`"example.com/renamed/runtime/winabi"`)) {
		t.Fatal("renamed module import missing")
	}
}

func TestWriteRestoresDeletedArtifactAndPreservesHandwrittenFiles(t *testing.T) {
	tree, err := Render(projectRoot(t), "go-windows-api.local", "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	manual := filepath.Join(root, "bindings", "win32", "foundation", "doc.go")
	if err := os.MkdirAll(filepath.Dir(manual), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manual, []byte("package foundation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(root, tree); err != nil {
		t.Fatal(err)
	}
	deleted := filepath.Join(root, "bindings", "win32", "foundation", "ztypes_base.go")
	if err := os.Remove(deleted); err != nil {
		t.Fatal(err)
	}
	if err := Write(root, tree); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(deleted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, tree.Files["bindings/win32/foundation/ztypes_base.go"]) {
		t.Fatal("deleted generated artifact was not restored")
	}
	if got, err := os.ReadFile(manual); err != nil || string(got) != "package foundation\n" {
		t.Fatalf("handwritten file changed: %q, %v", got, err)
	}
}

func TestRenderLockFailureLeavesSliceUntouched(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sources.lock.json"), []byte(`{"schemaVersion":1,"sources":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Render(root, "go-windows-api.local", "0.1.0")
	if err == nil || !strings.Contains(err.Error(), "microsoft-win32metadata") {
		t.Fatalf("expected pinned input failure, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "bindings")); !os.IsNotExist(err) {
		t.Fatalf("render created output on failure: %v", err)
	}
}

func TestWritePreflightFailureKeepsExistingArtifact(t *testing.T) {
	tree, err := Render(projectRoot(t), "go-windows-api.local", "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	kept := filepath.Join(root, "bindings", "wdk", "nt", "ztypes_nt.go")
	if err := os.MkdirAll(filepath.Dir(kept), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kept, []byte("previous"), 0o644); err != nil {
		t.Fatal(err)
	}
	obstruction := filepath.Join(root, "bindings", "winrt", "windows", "foundation", "zclasses_uri.go")
	if err := os.MkdirAll(obstruction, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Write(root, tree); err == nil {
		t.Fatal("expected write preflight failure")
	}
	if got, err := os.ReadFile(kept); err != nil || string(got) != "previous" {
		t.Fatalf("existing artifact changed after failed write: %q, %v", got, err)
	}
}

func TestWriteRejectsHandwrittenFileAtOwnedPath(t *testing.T) {
	tree, err := Render(projectRoot(t), "go-windows-api.local", "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "bindings", "win32", "foundation", "ztypes_base.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package foundation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(root, tree); err == nil {
		t.Fatal("expected collision with handwritten file")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "package foundation\n" {
		t.Fatalf("handwritten collision changed: %q, %v", got, err)
	}
}
