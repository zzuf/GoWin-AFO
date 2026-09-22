package source

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeJoin(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../x", "/x", "a/../../x"} {
		if _, err := safeJoin(root, name); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	if _, err := safeJoin(root, "a/b"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateLockRejectsIncompleteAndAmbiguousEntries(t *testing.T) {
	validSource := Locked{ID: "official", Type: "winmd", Package: "Package", Version: "1.0.0", Retrieval: "https://example.invalid/package.nupkg", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LicenseIdentifier: "MIT", Architectures: []string{"386", "amd64", "arm64"}, WindowsSDKVersion: "10.0.1.0", Files: []string{"metadata.winmd"}, Required: true}
	if err := validateLock(Lock{SchemaVersion: 1, Sources: []Locked{validSource}}); err != nil {
		t.Fatal(err)
	}
	duplicateArchitecture := validSource
	duplicateArchitecture.Architectures = []string{"amd64", "amd64"}
	if err := validateLock(Lock{SchemaVersion: 1, Sources: []Locked{duplicateArchitecture}}); err == nil {
		t.Fatal("duplicate architecture accepted")
	}
	unknownDependency := validSource
	unknownDependency.DependsOn = []string{"not-locked"}
	if err := validateLock(Lock{SchemaVersion: 1, Sources: []Locked{unknownDependency}}); err == nil {
		t.Fatal("unknown source dependency accepted")
	}
	cycleA, cycleB := validSource, validSource
	cycleA.ID, cycleB.ID = "a", "b"
	cycleA.DependsOn, cycleB.DependsOn = []string{"b"}, []string{"a"}
	if err := validateLock(Lock{SchemaVersion: 1, Sources: []Locked{cycleA, cycleB}}); err == nil {
		t.Fatal("source dependency cycle accepted")
	}
	unsafeFile := validSource
	unsafeFile.Files = []string{"../metadata.winmd"}
	if err := validateLock(Lock{SchemaVersion: 1, Sources: []Locked{unsafeFile}}); err == nil {
		t.Fatal("unsafe locked file accepted")
	}

	root := t.TempDir()
	b, err := json.Marshal(Lock{SchemaVersion: 1, Sources: []Locked{validSource}})
	if err != nil {
		t.Fatal(err)
	}
	b = append(b[:len(b)-1], []byte(`,"unknown":true}`)...)
	if err = os.WriteFile(filepath.Join(root, "sources.lock.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(root); err == nil {
		t.Fatal("unknown lock field accepted")
	}
}

func TestExtractSelectedRejectsZipSlipAndRequiresFiles(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, _ := z.Create("ok.txt")
	_, _ = w.Write([]byte("ok"))
	_ = z.Close()
	_ = f.Close()
	if err = extractSelected(p, filepath.Join(dir, "out"), []string{"missing"}); err == nil {
		t.Fatal("missing locked file accepted")
	}
	if err = extractSelected(p, filepath.Join(dir, "out2"), []string{"../bad"}); err == nil {
		t.Fatal("unsafe locked path accepted")
	}
}

func TestValidateLockRejectsMislabeledNuGetSource(t *testing.T) {
	s := Locked{ID: "official", Type: "winmd", Package: "Example", Version: "1.0.0", Retrieval: "https://api.nuget.org/v3-flatcontainer/example/1.0.0/example.1.0.0.nupkg", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LicenseIdentifier: "MIT", Architectures: []string{"amd64"}, WindowsSDKVersion: "10.0.1.0", Files: []string{"metadata.winmd"}}
	if err := validateLock(Lock{SchemaVersion: 1, Sources: []Locked{s}}); err != nil {
		t.Fatal(err)
	}
	for _, retrieval := range []string{
		"https://api.nuget.org/v3-flatcontainer/different/1.0.0/different.1.0.0.nupkg",
		"https://api.nuget.org/v3-flatcontainer/example/2.0.0/example.2.0.0.nupkg",
		"https://api.nuget.org/v3-flatcontainer/example/1.0.0/different.nupkg",
		"https://api.nuget.org/v3-flatcontainer/example/1.0.0/example.1.0.0.nupkg?override=1",
		"https://api.nuget.org/v3-flatcontainer/example",
	} {
		s.Retrieval = retrieval
		if err := validateLock(Lock{SchemaVersion: 1, Sources: []Locked{s}}); err == nil {
			t.Errorf("accepted mislabeled URL %s", retrieval)
		}
	}
}

func TestVerifySelectedAgainstArchiveRejectsTamperedExtraction(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "source.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, err := z.Create("metadata.winmd")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("official"))
	_ = z.Close()
	_ = f.Close()
	if err = os.WriteFile(filepath.Join(dir, "metadata.winmd"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = verifySelectedAgainstArchive(archive, dir, []string{"metadata.winmd"}); err == nil {
		t.Fatal("tampered extracted input passed verification")
	}
}
