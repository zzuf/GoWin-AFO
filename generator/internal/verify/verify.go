package verify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go-windows-api.local/generator/internal/model"
)

type OracleInput struct {
	Architecture string
	Path         string
	Required     bool
}

type Options struct {
	Root             string
	Output           string
	SliceManifest    string
	SourceLock       string
	ProbeManifest    string
	OracleResults    []OracleInput
	Inventory        model.Inventory
	GeneratorVersion string
	VerifierVersion  string
}

type VerificationError struct {
	MismatchCount int
}

func (e VerificationError) Error() string {
	return fmt.Sprintf("ABI verification failed with %d mismatch(es)", e.MismatchCount)
}

// FindRoot locates the project without depending on the caller's package.
func FindRoot(start string) (string, error) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	root, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, lockErr := os.Stat(filepath.Join(root, "sources.lock.json")); lockErr == nil {
			return root, nil
		}
		parent := filepath.Dir(root)
		if parent == root {
			return "", errors.New("sources.lock.json not found in this directory or a parent")
		}
		root = parent
	}
}

// Run performs verification and writes the report before returning a mismatch
// error. Therefore CI receives a useful artifact even on ABI failure.
func Run(ctx context.Context, options Options) (Report, error) {
	root, err := FindRoot(options.Root)
	if err != nil {
		return Report{}, err
	}
	options.Root = root
	options = withDefaults(options)
	report := Report{
		SchemaVersion:         SchemaVersion,
		VerifierVersion:       options.VerifierVersion,
		GeneratorVersion:      options.GeneratorVersion,
		InventoryManifestHash: options.Inventory.ManifestHash,
		Scope: VerificationScope{
			Name:                          "vertical-slice-fixture",
			AllMeaning:                    "all checked-in ABI evidence for the vertical slice, not all Windows SDK namespaces",
			Comparison:                    "normalized IR expectations versus independently compiled C/C++ oracle facts",
			GeneratedGoLayoutMeasured:     false,
			GeneratedGoCallBoundaryTested: false,
		},
		Architectures:  []ArchitectureResult{},
		Oracles:        []OracleSummary{},
		MatchedSymbols: []SymbolResult{},
		UnmatchedFacts: []UnmatchedFact{},
		Diagnostics:    []Diagnostic{},
		Mismatches:     []Mismatch{},
		Summary:        Summary{MatchedSymbolIDs: []string{}},
	}

	if err = ctx.Err(); err != nil {
		return report, err
	}

	slice, sliceErr := loadSliceManifest(resolve(root, options.SliceManifest))
	lock, lockErr := loadSourceLock(resolve(root, options.SourceLock))
	if sliceErr != nil {
		addLoadMismatch(&report, "slice-manifest-invalid", "/sliceManifest", "valid schema-v1 manifest", sliceErr, root)
	}
	if lockErr != nil {
		addLoadMismatch(&report, "source-lock-invalid", "/sourceLock", "valid schema-v1 source lock", lockErr, root)
	}
	lockedSDKs := map[string]bool{}
	if sliceErr == nil && lockErr == nil {
		lockedSDKs = verifySliceAndLock(root, slice, lock, options.GeneratorVersion, &report)
	}

	probe, probeErr := loadProbeManifest(resolve(root, options.ProbeManifest))
	if probeErr != nil {
		addLoadMismatch(&report, "oracle-manifest-invalid", "/oracleManifest", "valid reviewed probe manifest", probeErr, root)
	}

	accepted := map[string]OracleResult{}
	architectureState := map[string]ArchitectureResult{}
	for _, input := range options.OracleResults {
		if err = ctx.Err(); err != nil {
			return report, err
		}
		state := ArchitectureResult{Architecture: input.Architecture, Status: "unavailable", Evidence: "no-valid-executed-oracle-result"}
		path := resolve(root, input.Path)
		result, loadErr := loadOracle(path)
		if loadErr != nil {
			if errors.Is(loadErr, os.ErrNotExist) && !input.Required {
				report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "warning", Code: "oracle-result-unavailable", Architecture: input.Architecture, Message: "no accepted executed oracle result was supplied"})
				architectureState[input.Architecture] = state
				continue
			}
			addLoadMismatchArchitecture(&report, "oracle-result-invalid", input.Architecture, "/oracles/"+input.Architecture, "valid executed oracle result", loadErr, root)
			architectureState[input.Architecture] = state
			continue
		}
		if result.Architecture != input.Architecture {
			report.Mismatches = append(report.Mismatches, Mismatch{Code: "oracle-architecture-mismatch", Architecture: input.Architecture, Path: "/oracles/" + input.Architecture + "/architecture", Expected: input.Architecture, Actual: result.Architecture})
			architectureState[input.Architecture] = state
			continue
		}
		if probeErr != nil {
			architectureState[input.Architecture] = state
			continue
		}
		provenanceMismatches := validateOracleProvenance(probe, result)
		if len(provenanceMismatches) != 0 {
			for i := range provenanceMismatches {
				provenanceMismatches[i].Architecture = input.Architecture
			}
			report.Mismatches = append(report.Mismatches, provenanceMismatches...)
			architectureState[input.Architecture] = state
			continue
		}
		if len(lockedSDKs) != 0 && !lockedSDKs[result.SDKVersion] {
			report.Mismatches = append(report.Mismatches, Mismatch{Code: "oracle-sdk-not-locked", Architecture: input.Architecture, Path: "/oracles/" + input.Architecture + "/sdkVersion", Expected: "SDK version referenced by a source-locked slice input", Actual: result.SDKVersion})
			architectureState[input.Architecture] = state
			continue
		}
		accepted[input.Architecture] = result
		state.Status = "executed"
		state.Evidence = "validated-executed-oracle-result"
		state.ABIFactCount = result.FactCount()
		architectureState[input.Architecture] = state
	}

	architectures := make([]string, 0, len(accepted))
	for architecture := range accepted {
		architectures = append(architectures, architecture)
	}
	sort.Strings(architectures)
	for _, architecture := range architectures {
		result := accepted[architecture]
		comparison := compareInventory(options.Inventory, probe, result)
		report.MatchedSymbols = append(report.MatchedSymbols, comparison.Symbols...)
		report.UnmatchedFacts = append(report.UnmatchedFacts, comparison.Unmatched...)
		report.Mismatches = append(report.Mismatches, comparison.Mismatches...)
		matchedCount := 0
		for _, symbol := range comparison.Symbols {
			matchedCount += len(symbol.MatchedFactIDs)
		}
		state := architectureState[architecture]
		state.MatchedFacts = matchedCount
		state.UnmatchedFacts = len(comparison.Unmatched)
		state.MismatchedFacts = comparison.MismatchedFacts
		architectureState[architecture] = state
		report.Oracles = append(report.Oracles, OracleSummary{
			Architecture: architecture, SourceID: result.SourceID,
			SDKVersion: result.SDKVersion, Profile: result.Profile,
			ManifestSHA256: result.ManifestSHA256,
			CompilerFamily: result.Compiler.Family, CompilerVersion: result.Compiler.Version,
			ABIFactCount: result.FactCount(), MatchedFacts: matchedCount,
			UnmatchedFacts: len(comparison.Unmatched), MismatchedFacts: comparison.MismatchedFacts,
		})
		if len(comparison.Unmatched) != 0 {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "info", Code: "oracle-facts-outside-fixture-ir", Architecture: architecture, Message: fmt.Sprintf("%d ABI facts have no comparable fixture IR fact and remain unverified", len(comparison.Unmatched))})
		}
	}

	if _, exists := architectureState["arm64"]; !exists {
		if compiled, evidence := hasARM64CompiledObject(root, probe, probeErr); compiled {
			architectureState["arm64"] = ArchitectureResult{Architecture: "arm64", Status: "compile-only", Evidence: evidence}
		} else if hasARM64CompileOnlyGate(root) {
			architectureState["arm64"] = ArchitectureResult{Architecture: "arm64", Status: "configured-only", Evidence: "configured-cross-compile-gate-no-validated-object"}
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "info", Code: "arm64-compile-gate-configured", Architecture: "arm64", Message: "ARM64 cross-compilation is configured, but this run has no validated COFF object evidence"})
		} else {
			architectureState["arm64"] = ArchitectureResult{Architecture: "arm64", Status: "unavailable", Evidence: "no-compile-or-execution-evidence"}
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "warning", Code: "arm64-abi-evidence-unavailable", Architecture: "arm64", Message: "no ARM64 compile-only gate or executed oracle result was found"})
		}
	}
	if _, exists := architectureState["386"]; !exists {
		architectureState["386"] = ArchitectureResult{Architecture: "386", Status: "unavailable", Evidence: "no-valid-executed-oracle-result"}
	}
	if _, exists := architectureState["amd64"]; !exists {
		architectureState["amd64"] = ArchitectureResult{Architecture: "amd64", Status: "unavailable", Evidence: "no-valid-executed-oracle-result"}
	}
	architectureState["arm64ec"] = ArchitectureResult{Architecture: "arm64ec", Status: "unsupported", Evidence: "distinct-target-not-supported-by-the-go-toolchain"}

	finalizeReport(&report, architectureState)
	if options.Output != "" {
		b, marshalErr := MarshalReport(report)
		if marshalErr != nil {
			return report, marshalErr
		}
		if writeErr := writeAtomic(resolve(root, options.Output), b); writeErr != nil {
			return report, writeErr
		}
	}
	if !report.Passed {
		return report, VerificationError{MismatchCount: len(report.Mismatches)}
	}
	return report, nil
}

func withDefaults(options Options) Options {
	if options.Output == "" {
		options.Output = filepath.Join("coverage", "abi-latest.json")
	}
	if options.SliceManifest == "" {
		options.SliceManifest = filepath.Join("bindings", "slice.manifest.json")
	}
	if options.SourceLock == "" {
		options.SourceLock = "sources.lock.json"
	}
	if options.ProbeManifest == "" {
		options.ProbeManifest = filepath.Join("tools", "abi-oracle", "testdata", "probe-manifest.json")
	}
	if options.OracleResults == nil {
		options.OracleResults = []OracleInput{
			{Architecture: "386", Path: filepath.Join("tools", "abi-oracle", "testdata", "windows-sdk-10.0.26100-386.json"), Required: true},
			{Architecture: "amd64", Path: filepath.Join("tools", "abi-oracle", "testdata", "windows-sdk-10.0.26100-amd64.json"), Required: true},
		}
	}
	if options.VerifierVersion == "" {
		options.VerifierVersion = "0.1.0"
	}
	if options.GeneratorVersion == "" {
		options.GeneratorVersion = "unknown"
	}
	return options
}

func resolve(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

type sliceManifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	Generator     string          `json:"generator"`
	Inputs        []sliceInput    `json:"inputs"`
	Artifacts     []sliceArtifact `json:"artifacts"`
}

type sliceInput struct {
	SourceID              string `json:"sourceId"`
	Version               string `json:"version"`
	SHA256                string `json:"sha256"`
	SliceDescriptor       string `json:"sliceDescriptor"`
	SliceDescriptorSHA256 string `json:"sliceDescriptorSha256"`
}

type sliceArtifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type sourceLock struct {
	SchemaVersion int            `json:"schemaVersion"`
	Sources       []lockedSource `json:"sources"`
}

type lockedSource struct {
	ID                string   `json:"id"`
	Type              string   `json:"type"`
	Package           string   `json:"package"`
	Version           string   `json:"version"`
	Retrieval         string   `json:"retrieval"`
	SHA256            string   `json:"sha256"`
	LicenseIdentifier string   `json:"licenseIdentifier"`
	Architectures     []string `json:"architectures"`
	WindowsSDKVersion string   `json:"windowsSDKVersion"`
	Files             []string `json:"files"`
	DependsOn         []string `json:"dependsOn"`
	Required          bool     `json:"required"`
}

func loadSliceManifest(path string) (sliceManifest, error) {
	var manifest sliceManifest
	if err := decodeStrict(path, &manifest); err != nil {
		return manifest, err
	}
	if err := validateContainerRequiredFields(path, "inputs", []string{"schemaVersion", "generator", "inputs", "artifacts"}, []string{"sourceId", "version", "sha256", "sliceDescriptor", "sliceDescriptorSha256"}, "artifacts", []string{"path", "sha256"}); err != nil {
		return manifest, err
	}
	var problems []string
	if manifest.SchemaVersion != 1 {
		problems = append(problems, "schemaVersion must be 1")
	}
	if manifest.Generator == "" {
		problems = append(problems, "generator is required")
	}
	if manifest.Inputs == nil || len(manifest.Inputs) == 0 {
		problems = append(problems, "inputs must not be empty")
	}
	if manifest.Artifacts == nil || len(manifest.Artifacts) == 0 {
		problems = append(problems, "artifacts must not be empty")
	}
	seenDescriptors := map[string]bool{}
	for index, input := range manifest.Inputs {
		if input.SourceID == "" || input.Version == "" || input.SliceDescriptor == "" {
			problems = append(problems, fmt.Sprintf("input %d has an empty required property", index))
		}
		if !sha256Pattern.MatchString(input.SHA256) || !sha256Pattern.MatchString(input.SliceDescriptorSHA256) {
			problems = append(problems, fmt.Sprintf("input %d has an invalid SHA-256", index))
		}
		if seenDescriptors[input.SliceDescriptor] {
			problems = append(problems, fmt.Sprintf("duplicate slice descriptor %q", input.SliceDescriptor))
		}
		seenDescriptors[input.SliceDescriptor] = true
	}
	seenArtifacts := map[string]bool{}
	for index, artifact := range manifest.Artifacts {
		if !safeArtifactPath(artifact.Path) {
			problems = append(problems, fmt.Sprintf("artifact %d has unsafe path %q", index, artifact.Path))
		}
		if !sha256Pattern.MatchString(artifact.SHA256) {
			problems = append(problems, fmt.Sprintf("artifact %d has an invalid SHA-256", index))
		}
		if seenArtifacts[artifact.Path] {
			problems = append(problems, fmt.Sprintf("duplicate artifact path %q", artifact.Path))
		}
		seenArtifacts[artifact.Path] = true
	}
	if len(problems) != 0 {
		sort.Strings(problems)
		return manifest, errors.New(strings.Join(problems, "; "))
	}
	return manifest, nil
}

func loadSourceLock(path string) (sourceLock, error) {
	var lock sourceLock
	if err := decodeStrict(path, &lock); err != nil {
		return lock, err
	}
	if err := validateContainerRequiredFields(path, "sources", []string{"schemaVersion", "sources"}, []string{"id", "type", "package", "version", "retrieval", "sha256", "licenseIdentifier", "architectures", "windowsSDKVersion", "files", "dependsOn", "required"}, "", nil); err != nil {
		return lock, err
	}
	var problems []string
	if lock.SchemaVersion != 1 {
		problems = append(problems, "schemaVersion must be 1")
	}
	if lock.Sources == nil || len(lock.Sources) == 0 {
		problems = append(problems, "sources must not be empty")
	}
	seen := map[string]bool{}
	for index, source := range lock.Sources {
		if source.ID == "" || source.Type == "" || source.Package == "" || source.Version == "" || source.Retrieval == "" || source.LicenseIdentifier == "" || source.WindowsSDKVersion == "" {
			problems = append(problems, fmt.Sprintf("source %d has an empty required property", index))
		}
		if !sha256Pattern.MatchString(source.SHA256) {
			problems = append(problems, fmt.Sprintf("source %q has an invalid SHA-256", source.ID))
		}
		if source.Architectures == nil || len(source.Architectures) == 0 || source.Files == nil || len(source.Files) == 0 || source.DependsOn == nil {
			problems = append(problems, fmt.Sprintf("source %q has a missing required array", source.ID))
		}
		if seen[source.ID] {
			problems = append(problems, fmt.Sprintf("duplicate source ID %q", source.ID))
		}
		seen[source.ID] = true
	}
	if len(problems) != 0 {
		sort.Strings(problems)
		return lock, errors.New(strings.Join(problems, "; "))
	}
	return lock, nil
}

func validateContainerRequiredFields(path, firstArray string, rootFields, firstFields []string, secondArray string, secondFields []string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var root map[string]json.RawMessage
	if err = json.Unmarshal(b, &root); err != nil {
		return err
	}
	var problems []string
	require := func(object map[string]json.RawMessage, objectPath string, fields []string) {
		for _, field := range fields {
			if _, exists := object[field]; !exists {
				problems = append(problems, objectPath+"/"+field+" is required")
			}
		}
	}
	require(root, "", rootFields)
	checkArray := func(name string, fields []string) {
		if name == "" {
			return
		}
		var values []map[string]json.RawMessage
		if raw, exists := root[name]; exists && json.Unmarshal(raw, &values) == nil {
			for index, value := range values {
				require(value, fmt.Sprintf("/%s/%d", name, index), fields)
			}
		}
	}
	checkArray(firstArray, firstFields)
	checkArray(secondArray, secondFields)
	if len(problems) != 0 {
		sort.Strings(problems)
		return fmt.Errorf("missing required JSON properties: %s", strings.Join(problems, "; "))
	}
	return nil
}

func safeArtifactPath(path string) bool {
	if path == "" || strings.Contains(path, "\\") || filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	return clean == path && path != "." && !strings.HasPrefix(path, "../") && strings.HasPrefix(path, "bindings/")
}

func verifySliceAndLock(root string, manifest sliceManifest, lock sourceLock, generatorVersion string, report *Report) map[string]bool {
	report.SliceManifest.InputCount = len(manifest.Inputs)
	report.SliceManifest.ArtifactCount = len(manifest.Artifacts)
	if generatorVersion != "" && generatorVersion != "unknown" {
		expected := "winapigen v" + generatorVersion
		if manifest.Generator != expected {
			report.Mismatches = append(report.Mismatches, Mismatch{Code: "slice-generator-version-mismatch", Path: "/sliceManifest/generator", Expected: expected, Actual: manifest.Generator})
		}
	}
	locked := make(map[string]lockedSource, len(lock.Sources))
	for _, source := range lock.Sources {
		locked[source.ID] = source
	}
	lockedSDKs := map[string]bool{}
	for index, input := range manifest.Inputs {
		path := fmt.Sprintf("/sliceManifest/inputs/%d", index)
		descriptorHash := sha256.Sum256([]byte(input.SliceDescriptor))
		actualDescriptorHash := hex.EncodeToString(descriptorHash[:])
		if actualDescriptorHash == input.SliceDescriptorSHA256 {
			report.SliceManifest.DescriptorHashCount++
		} else {
			report.Mismatches = append(report.Mismatches, Mismatch{Code: "slice-descriptor-hash-mismatch", Path: path + "/sliceDescriptorSha256", Expected: input.SliceDescriptorSHA256, Actual: actualDescriptorHash})
		}
		source, ok := locked[input.SourceID]
		if !ok {
			report.Mismatches = append(report.Mismatches, Mismatch{Code: "slice-source-not-locked", Path: path + "/sourceId", Expected: "source ID present in sources.lock.json", Actual: input.SourceID})
			continue
		}
		matched := true
		if source.Version != input.Version {
			matched = false
			report.Mismatches = append(report.Mismatches, Mismatch{Code: "slice-source-version-mismatch", Path: path + "/version", Expected: source.Version, Actual: input.Version})
		}
		if source.SHA256 != input.SHA256 {
			matched = false
			report.Mismatches = append(report.Mismatches, Mismatch{Code: "slice-source-hash-mismatch", Path: path + "/sha256", Expected: source.SHA256, Actual: input.SHA256})
		}
		if matched {
			report.SliceManifest.SourceLockMatchCount++
			lockedSDKs[source.WindowsSDKVersion] = true
		}
	}
	for index, artifact := range manifest.Artifacts {
		actual, hashErr := fileSHA256(filepath.Join(root, filepath.FromSlash(artifact.Path)))
		if hashErr != nil {
			addLoadMismatch(report, "slice-artifact-unreadable", fmt.Sprintf("/sliceManifest/artifacts/%d/sha256", index), artifact.SHA256, hashErr, root)
			continue
		}
		if actual != artifact.SHA256 {
			report.Mismatches = append(report.Mismatches, Mismatch{Code: "slice-artifact-hash-mismatch", Path: fmt.Sprintf("/sliceManifest/artifacts/%d/sha256", index), Expected: artifact.SHA256, Actual: actual})
			continue
		}
		report.SliceManifest.VerifiedArtifactHashCount++
	}
	return lockedSDKs
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func validateOracleProvenance(base ProbeManifest, result OracleResult) []Mismatch {
	manifest := sortedProbeManifest(base)
	manifest.Architecture = result.Architecture
	manifest = sortedProbeManifest(manifest)
	canonical, err := json.Marshal(manifest)
	if err != nil {
		return []Mismatch{{Code: "oracle-manifest-canonicalization-failed", Path: "/oracle/manifestSha256", Expected: "canonical probe hash", Actual: err.Error()}}
	}
	sum := sha256.Sum256(canonical)
	expectedHash := hex.EncodeToString(sum[:])
	var mismatches []Mismatch
	appendStringMismatch := func(code, path, expected, actual string) {
		if expected != actual {
			mismatches = append(mismatches, Mismatch{Code: code, Path: path, Expected: expected, Actual: actual})
		}
	}
	appendStringMismatch("oracle-source-mismatch", "/oracle/sourceId", manifest.SourceID, result.SourceID)
	appendStringMismatch("oracle-sdk-mismatch", "/oracle/sdkVersion", manifest.SDKVersion, result.SDKVersion)
	appendStringMismatch("oracle-profile-mismatch", "/oracle/profile", manifest.Profile, result.Profile)
	appendStringMismatch("oracle-manifest-hash-mismatch", "/oracle/manifestSha256", expectedHash, result.ManifestSHA256)
	mismatches = append(mismatches, compareProbeShape(manifest, result)...)
	return mismatches
}

func compareProbeShape(manifest ProbeManifest, result OracleResult) []Mismatch {
	var mismatches []Mismatch
	check := func(kind string, expected map[string]string, actual map[string]string) {
		keys := map[string]bool{}
		for id := range expected {
			keys[id] = true
		}
		for id := range actual {
			keys[id] = true
		}
		ordered := make([]string, 0, len(keys))
		for id := range keys {
			ordered = append(ordered, id)
		}
		sort.Strings(ordered)
		for _, id := range ordered {
			expectedValue, expectedOK := expected[id]
			actualValue, actualOK := actual[id]
			if !expectedOK {
				expectedValue = "<missing>"
			}
			if !actualOK {
				actualValue = "<missing>"
			}
			if expectedValue != actualValue {
				mismatches = append(mismatches, Mismatch{Code: "oracle-probe-shape-mismatch", FactID: id, Path: "/oracle/" + kind + "/" + id, Expected: expectedValue, Actual: actualValue})
			}
		}
	}
	recordExpected, recordActual := map[string]string{}, map[string]string{}
	for _, record := range manifest.Records {
		parts := []string{record.NativeName, record.Kind}
		for _, field := range record.Fields {
			parts = append(parts, field.ID+"="+field.NativeName)
		}
		recordExpected[record.ID] = strings.Join(parts, "|")
	}
	for _, record := range result.Records {
		parts := []string{record.NativeName, record.Kind}
		fields := append([]FieldResult(nil), record.Fields...)
		sort.Slice(fields, func(i, j int) bool { return fields[i].ID < fields[j].ID })
		for _, field := range fields {
			parts = append(parts, field.ID+"="+field.NativeName)
		}
		recordActual[record.ID] = strings.Join(parts, "|")
	}
	check("records", recordExpected, recordActual)
	valueExpected, valueActual := map[string]string{}, map[string]string{}
	for _, value := range manifest.Values {
		valueExpected[value.ID] = value.NativeName
	}
	for _, value := range result.Values {
		valueActual[value.ID] = value.NativeName
	}
	check("values", valueExpected, valueActual)
	guidExpected, guidActual := map[string]string{}, map[string]string{}
	for _, guid := range manifest.GUIDs {
		guidExpected[guid.ID] = guid.NativeName
	}
	for _, guid := range result.GUIDs {
		guidActual[guid.ID] = guid.NativeName
	}
	check("guids", guidExpected, guidActual)
	bitExpected, bitActual := map[string]string{}, map[string]string{}
	for _, field := range manifest.BitFields {
		bitExpected[field.ID] = field.NativeName
	}
	for _, field := range result.BitFields {
		bitActual[field.ID] = field.NativeName
	}
	check("bitFields", bitExpected, bitActual)
	functionExpected, functionActual := map[string]string{}, map[string]string{}
	for _, function := range manifest.Functions {
		functionExpected[function.ID] = function.NativeName + "|" + function.CallingConvention
	}
	for _, function := range result.Functions {
		functionActual[function.ID] = function.NativeName + "|" + function.CallingConvention
	}
	check("functions", functionExpected, functionActual)
	vtableExpected, vtableActual := map[string]string{}, map[string]string{}
	for _, table := range manifest.VTables {
		vtableExpected[table.ID] = table.Interface + "|" + table.Method
	}
	for _, table := range result.VTables {
		vtableActual[table.ID] = table.Interface + "|" + table.Method
	}
	check("vtables", vtableExpected, vtableActual)
	conditionExpected, conditionActual := map[string]string{}, map[string]string{}
	for _, condition := range manifest.Conditions {
		conditionExpected[condition.ID] = condition.NativeName
	}
	for _, condition := range result.Conditions {
		conditionActual[condition.ID] = condition.NativeName
	}
	check("conditions", conditionExpected, conditionActual)
	return mismatches
}

func sortedProbeManifest(manifest ProbeManifest) ProbeManifest {
	out := manifest
	out.Defines = make(map[string]string, len(manifest.Defines))
	for key, value := range manifest.Defines {
		out.Defines[key] = value
	}
	out.Includes = append([]string(nil), manifest.Includes...)
	out.Records = append([]RecordProbe(nil), manifest.Records...)
	for index := range out.Records {
		out.Records[index].Fields = append([]FieldProbe(nil), out.Records[index].Fields...)
		sort.Slice(out.Records[index].Fields, func(i, j int) bool { return out.Records[index].Fields[i].ID < out.Records[index].Fields[j].ID })
	}
	out.Values = append([]ValueProbe(nil), manifest.Values...)
	out.GUIDs = append([]GUIDProbe(nil), manifest.GUIDs...)
	out.BitFields = append([]BitFieldProbe(nil), manifest.BitFields...)
	out.Functions = append([]FunctionProbe(nil), manifest.Functions...)
	out.VTables = append([]VTableProbe(nil), manifest.VTables...)
	out.Conditions = append([]ConditionProbe(nil), manifest.Conditions...)
	sort.Slice(out.Records, func(i, j int) bool { return out.Records[i].ID < out.Records[j].ID })
	sort.Slice(out.Values, func(i, j int) bool { return out.Values[i].ID < out.Values[j].ID })
	sort.Slice(out.GUIDs, func(i, j int) bool { return out.GUIDs[i].ID < out.GUIDs[j].ID })
	sort.Slice(out.BitFields, func(i, j int) bool { return out.BitFields[i].ID < out.BitFields[j].ID })
	sort.Slice(out.Functions, func(i, j int) bool { return out.Functions[i].ID < out.Functions[j].ID })
	sort.Slice(out.VTables, func(i, j int) bool { return out.VTables[i].ID < out.VTables[j].ID })
	sort.Slice(out.Conditions, func(i, j int) bool { return out.Conditions[i].ID < out.Conditions[j].ID })
	return out
}

func hasARM64CompileOnlyGate(root string) bool {
	b, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "abi.yml"))
	if err != nil {
		return false
	}
	text := string(b)
	configuredTarget := strings.Contains(text, "vcvars: amd64_arm64") ||
		strings.Contains(text, "--target=arm64-pc-windows-msvc")
	configuredCompileOnly := strings.Contains(text, "/c /Fo:") || strings.Contains(text, "/c /Fo")
	return strings.Contains(text, "goarch: arm64") && configuredTarget &&
		strings.Contains(text, "execute: false") && configuredCompileOnly
}

func hasARM64CompiledObject(root string, manifest ProbeManifest, manifestErr error) (bool, string) {
	if manifestErr != nil {
		return false, ""
	}
	b, err := os.ReadFile(filepath.Join(root, "tools", "abi-oracle", "out", "probe-arm64.obj"))
	if err != nil || len(b) < 20 || binary.LittleEndian.Uint16(b[:2]) != 0xaa64 {
		return false, ""
	}
	manifest.Architecture = "arm64"
	canonical, err := json.Marshal(sortedProbeManifest(manifest))
	if err != nil {
		return false, ""
	}
	sum := sha256.Sum256(canonical)
	if !bytes.Contains(b, []byte(hex.EncodeToString(sum[:]))) {
		return false, ""
	}
	return true, "validated-arm64-coff-object-with-current-probe-hash-no-execution-result"
}

func finalizeReport(report *Report, states map[string]ArchitectureResult) {
	sort.Slice(report.MatchedSymbols, func(i, j int) bool {
		if report.MatchedSymbols[i].Architecture != report.MatchedSymbols[j].Architecture {
			return report.MatchedSymbols[i].Architecture < report.MatchedSymbols[j].Architecture
		}
		return report.MatchedSymbols[i].SymbolID < report.MatchedSymbols[j].SymbolID
	})
	for index := range report.MatchedSymbols {
		sort.Strings(report.MatchedSymbols[index].MatchedFactIDs)
		sort.Strings(report.MatchedSymbols[index].UnmatchedFactIDs)
		sort.Strings(report.MatchedSymbols[index].MismatchedFactIDs)
	}
	sort.Slice(report.UnmatchedFacts, func(i, j int) bool {
		if report.UnmatchedFacts[i].Architecture != report.UnmatchedFacts[j].Architecture {
			return report.UnmatchedFacts[i].Architecture < report.UnmatchedFacts[j].Architecture
		}
		return report.UnmatchedFacts[i].FactID < report.UnmatchedFacts[j].FactID
	})
	sort.Slice(report.Diagnostics, func(i, j int) bool {
		a, b := report.Diagnostics[i], report.Diagnostics[j]
		if a.Architecture != b.Architecture {
			return a.Architecture < b.Architecture
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
	sort.Slice(report.Mismatches, func(i, j int) bool {
		a, b := report.Mismatches[i], report.Mismatches[j]
		if a.Architecture != b.Architecture {
			return a.Architecture < b.Architecture
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.FactID != b.FactID {
			return a.FactID < b.FactID
		}
		return a.Code < b.Code
	})
	sort.Slice(report.Oracles, func(i, j int) bool { return report.Oracles[i].Architecture < report.Oracles[j].Architecture })
	for architecture, state := range states {
		for _, mismatch := range report.Mismatches {
			if mismatch.Architecture == architecture {
				state.MismatchCount++
			}
		}
		report.Architectures = append(report.Architectures, state)
	}
	sort.Slice(report.Architectures, func(i, j int) bool {
		return report.Architectures[i].Architecture < report.Architectures[j].Architecture
	})

	matchedIDs := map[string]bool{}
	verifiedIDs := map[string]bool{}
	for _, symbol := range report.MatchedSymbols {
		matchedIDs[symbol.SymbolID] = true
		if symbol.Complete {
			verifiedIDs[symbol.SymbolID] = true
			report.Summary.VerifiedSymbolArchitectureCount++
		}
	}
	for id := range matchedIDs {
		report.Summary.MatchedSymbolIDs = append(report.Summary.MatchedSymbolIDs, id)
	}
	sort.Strings(report.Summary.MatchedSymbolIDs)
	report.Summary.OracleCount = len(report.Oracles)
	report.Summary.MatchedSymbolCount = len(matchedIDs)
	report.Summary.VerifiedSymbolCount = len(verifiedIDs)
	report.Summary.MismatchCount = len(report.Mismatches)
	for _, oracle := range report.Oracles {
		report.Summary.ABIFactCount += oracle.ABIFactCount
		report.Summary.MatchedABIFactCount += oracle.MatchedFacts
		report.Summary.UnmatchedABIFactCount += oracle.UnmatchedFacts
		report.Summary.MismatchedABIFactCount += oracle.MismatchedFacts
	}
	report.Passed = len(report.Mismatches) == 0
}

func addLoadMismatch(report *Report, code, path, expected string, err error, root string) {
	report.Mismatches = append(report.Mismatches, Mismatch{Code: code, Path: path, Expected: expected, Actual: sanitizeError(err, root)})
}

func addLoadMismatchArchitecture(report *Report, code, architecture, path, expected string, err error, root string) {
	report.Mismatches = append(report.Mismatches, Mismatch{Code: code, Architecture: architecture, Path: path, Expected: expected, Actual: sanitizeError(err, root)})
}

func sanitizeError(err error, root string) string {
	message := err.Error()
	root = filepath.Clean(root)
	message = strings.ReplaceAll(message, root, "<project-root>")
	message = strings.ReplaceAll(message, filepath.ToSlash(root), "<project-root>")
	return message
}

func MarshalReport(report Report) ([]byte, error) {
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, data) {
		return nil
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".winapiverify-")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err = temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err = temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err = temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return replaceFile(tempName, path)
}

func replaceFile(tempName, path string) error {
	if err := os.Rename(tempName, path); err == nil {
		return nil
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return os.Rename(tempName, path)
	} else if err != nil {
		return err
	}
	backup := path + ".old"
	if _, err := os.Stat(backup); err == nil {
		return fmt.Errorf("refusing to replace report while stale backup exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(path, backup); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		_ = os.Rename(backup, path)
		return err
	}
	return os.Remove(backup)
}
