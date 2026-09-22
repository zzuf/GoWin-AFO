package verify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-windows-api.local/generator/internal/metadata"
	"go-windows-api.local/generator/internal/model"
	"go-windows-api.local/generator/internal/normalize"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func fixtureInventory(t *testing.T) model.Inventory {
	t.Helper()
	root := repositoryRoot(t)
	inventory, err := metadata.LoadFixture(filepath.Join(root, "generator", "testdata", "e2e", "source.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = normalize.Inventory(&inventory); err != nil {
		t.Fatal(err)
	}
	return inventory
}

func TestCheckedInOracleResultsValidateIndependently(t *testing.T) {
	root := repositoryRoot(t)
	for _, architecture := range []string{"386", "amd64"} {
		result, err := loadOracle(filepath.Join(root, "tools", "abi-oracle", "testdata", "windows-sdk-10.0.26100-"+architecture+".json"))
		if err != nil {
			t.Fatalf("%s: %v", architecture, err)
		}
		if result.Architecture != architecture || result.FactCount() != 47 {
			t.Fatalf("%s result: architecture=%q facts=%d", architecture, result.Architecture, result.FactCount())
		}
	}
}

func TestOracleDecoderRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	root := repositoryRoot(t)
	original, err := os.ReadFile(filepath.Join(root, "tools", "abi-oracle", "testdata", "windows-sdk-10.0.26100-amd64.json"))
	if err != nil {
		t.Fatal(err)
	}
	withUnknown := bytes.Replace(original, []byte(`"schemaVersion": 1,`), []byte(`"schemaVersion": 1, "unexpected": true,`), 1)
	unknownPath := filepath.Join(t.TempDir(), "unknown.json")
	if err = os.WriteFile(unknownPath, withUnknown, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = loadOracle(unknownPath); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("loadOracle(unknown field) = %v", err)
	}
	trailingPath := filepath.Join(t.TempDir(), "trailing.json")
	if err = os.WriteFile(trailingPath, append(original, []byte("{}")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = loadOracle(trailingPath); err == nil || !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("loadOracle(trailing JSON) = %v", err)
	}
}

func TestOracleDecoderRejectsMissingZeroValuedRequiredField(t *testing.T) {
	root := repositoryRoot(t)
	original, err := os.ReadFile(filepath.Join(root, "tools", "abi-oracle", "testdata", "windows-sdk-10.0.26100-amd64.json"))
	if err != nil {
		t.Fatal(err)
	}
	missing := bytes.Replace(original, []byte(`, "index": 1`), nil, 1)
	if bytes.Equal(missing, original) {
		t.Fatal("test fixture no longer contains the expected vtable index spelling")
	}
	path := filepath.Join(t.TempDir(), "missing-index.json")
	if err = os.WriteFile(path, missing, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = loadOracle(path); err == nil || !strings.Contains(err.Error(), "/vtables/0/index is required") {
		t.Fatalf("loadOracle(missing required index) = %v", err)
	}
}

func TestRunCheckedInSliceIsDeterministic(t *testing.T) {
	root := repositoryRoot(t)
	output := filepath.Join(t.TempDir(), "abi-latest.json")
	options := Options{
		Root: root, Output: output, Inventory: fixtureInventory(t),
		GeneratorVersion: "0.1.0", VerifierVersion: "test",
	}
	first, err := Run(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Run(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("identical inputs produced different ABI reports")
	}
	if !first.Passed || !second.Passed || first.Summary.ABIFactCount != 94 || first.Summary.MatchedABIFactCount != 34 {
		t.Fatalf("unexpected report summary: %#v", first.Summary)
	}
	if first.Summary.MatchedSymbolCount != 4 || first.Summary.VerifiedSymbolArchitectureCount != 8 {
		t.Fatalf("unexpected matched symbols: %#v", first.Summary)
	}
	status := map[string]string{}
	for _, architecture := range first.Architectures {
		status[architecture.Architecture] = architecture.Status
	}
	if status["386"] != "executed" || status["amd64"] != "executed" || (status["arm64"] != "compile-only" && status["arm64"] != "configured-only") || status["arm64ec"] != "unsupported" {
		t.Fatalf("architecture statuses = %#v", status)
	}
	if first.Scope.Name != "vertical-slice-fixture" || first.Scope.GeneratedGoLayoutMeasured || first.Scope.GeneratedGoCallBoundaryTested {
		t.Fatalf("verification scope = %#v", first.Scope)
	}
	if bytes.Contains(firstBytes, []byte(root)) {
		t.Fatal("report contains a local absolute project path")
	}
}

func TestRunEmitsReportAndFailsOnIRLayoutMismatch(t *testing.T) {
	root := repositoryRoot(t)
	inventory := fixtureInventory(t)
	for symbolIndex := range inventory.Symbols {
		symbol := &inventory.Symbols[symbolIndex]
		if symbol.NativeName != "FILETIME" || symbol.Type == nil {
			continue
		}
		for layoutIndex := range symbol.Type.Layouts {
			if symbol.Type.Layouts[layoutIndex].Architecture == "amd64" {
				symbol.Type.Layouts[layoutIndex].Size++
			}
		}
	}
	output := filepath.Join(t.TempDir(), "abi-mismatch.json")
	report, err := Run(context.Background(), Options{
		Root: root, Output: output, Inventory: inventory,
		GeneratorVersion: "0.1.0", VerifierVersion: "test",
	})
	var verificationError VerificationError
	if !errors.As(err, &verificationError) {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Passed || report.Summary.MismatchedABIFactCount != 1 || verificationError.MismatchCount != 1 {
		t.Fatalf("mismatch report = %#v; error = %#v", report.Summary, verificationError)
	}
	if _, statErr := os.Stat(output); statErr != nil {
		t.Fatalf("failure report was not emitted: %v", statErr)
	}
	if len(report.Mismatches) != 1 || report.Mismatches[0].FactID != "record.FILETIME.size" || report.Mismatches[0].Architecture != "amd64" {
		t.Fatalf("mismatches = %#v", report.Mismatches)
	}
}

func TestVerifySliceAndLockChecksDescriptorSourceAndArtifactHashes(t *testing.T) {
	root := t.TempDir()
	artifactPath := filepath.Join(root, "bindings", "fixture.go")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifactHash, err := fileSHA256(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := "slice-v1"
	descriptorSum := sha256.Sum256([]byte(descriptor))
	upstreamHash := strings.Repeat("a", 64)
	manifest := sliceManifest{
		SchemaVersion: 1, Generator: "winapigen v0.1.0",
		Inputs:    []sliceInput{{SourceID: "source", Version: "1.0.0", SHA256: upstreamHash, SliceDescriptor: descriptor, SliceDescriptorSHA256: hex.EncodeToString(descriptorSum[:])}},
		Artifacts: []sliceArtifact{{Path: "bindings/fixture.go", SHA256: artifactHash}},
	}
	lock := sourceLock{SchemaVersion: 1, Sources: []lockedSource{{ID: "source", Version: "1.0.0", SHA256: upstreamHash, WindowsSDKVersion: "10.0.1"}}}
	report := Report{Mismatches: []Mismatch{}}
	sdks := verifySliceAndLock(root, manifest, lock, "0.1.0", &report)
	if len(report.Mismatches) != 0 || report.SliceManifest.SourceLockMatchCount != 1 || report.SliceManifest.DescriptorHashCount != 1 || report.SliceManifest.VerifiedArtifactHashCount != 1 || !sdks["10.0.1"] {
		t.Fatalf("verification = %#v; SDKs = %#v", report, sdks)
	}
	manifest.Artifacts[0].SHA256 = strings.Repeat("b", 64)
	report = Report{Mismatches: []Mismatch{}}
	verifySliceAndLock(root, manifest, lock, "0.1.0", &report)
	if len(report.Mismatches) != 1 || report.Mismatches[0].Code != "slice-artifact-hash-mismatch" {
		t.Fatalf("artifact mismatch = %#v", report.Mismatches)
	}
}

func TestSafeArtifactPathRejectsTraversalAndAbsolutePaths(t *testing.T) {
	for _, path := range []string{"../bindings/a.go", "bindings/../a.go", `bindings\\a.go`, filepath.Join(repositoryRoot(t), "bindings", "a.go"), "coverage/a.json"} {
		if safeArtifactPath(path) {
			t.Errorf("safeArtifactPath(%q) = true", path)
		}
	}
	if !safeArtifactPath("bindings/win32/foundation/ztypes.go") {
		t.Fatal("valid repository-relative binding path was rejected")
	}
}

func TestWriteAtomicReplacesExistingReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := writeAtomic(path, []byte("first\n")); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(path, []byte("second\n")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "second\n" {
		t.Fatalf("report = %q", got)
	}
	if _, err = os.Stat(path + ".old"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale backup remains: %v", err)
	}
}
