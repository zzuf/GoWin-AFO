// Package verify validates pinned binding artifacts and compares normalized IR
// with independently produced Windows SDK ABI facts.
package verify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

const SchemaVersion = 1

var (
	sha256Pattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	decimalPattern = regexp.MustCompile(`^-?[0-9]+$`)
	guidPattern    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	maskPattern    = regexp.MustCompile(`^(?:[0-9a-f]{2})+$`)
)

// Report is the deterministic, machine-readable output of an ABI verification
// run. It deliberately contains no wall-clock time or local filesystem path.
type Report struct {
	SchemaVersion         int                  `json:"schemaVersion"`
	VerifierVersion       string               `json:"verifierVersion"`
	GeneratorVersion      string               `json:"generatorVersion"`
	InventoryManifestHash string               `json:"inventoryManifestHash"`
	Scope                 VerificationScope    `json:"scope"`
	Passed                bool                 `json:"passed"`
	SliceManifest         SliceManifestSummary `json:"sliceManifest"`
	Architectures         []ArchitectureResult `json:"architectures"`
	Oracles               []OracleSummary      `json:"oracles"`
	MatchedSymbols        []SymbolResult       `json:"matchedSymbols"`
	UnmatchedFacts        []UnmatchedFact      `json:"unmatchedFacts"`
	Diagnostics           []Diagnostic         `json:"diagnostics"`
	Mismatches            []Mismatch           `json:"mismatches"`
	Summary               Summary              `json:"summary"`
}

type VerificationScope struct {
	Name                          string `json:"name"`
	AllMeaning                    string `json:"allMeaning"`
	Comparison                    string `json:"comparison"`
	GeneratedGoLayoutMeasured     bool   `json:"generatedGoLayoutMeasured"`
	GeneratedGoCallBoundaryTested bool   `json:"generatedGoCallBoundaryTested"`
}

type SliceManifestSummary struct {
	InputCount                int `json:"inputCount"`
	SourceLockMatchCount      int `json:"sourceLockMatchCount"`
	DescriptorHashCount       int `json:"descriptorHashCount"`
	ArtifactCount             int `json:"artifactCount"`
	VerifiedArtifactHashCount int `json:"verifiedArtifactHashCount"`
}

type ArchitectureResult struct {
	Architecture    string `json:"architecture"`
	Status          string `json:"status"`
	Evidence        string `json:"evidence"`
	ABIFactCount    int    `json:"abiFactCount"`
	MatchedFacts    int    `json:"matchedFacts"`
	UnmatchedFacts  int    `json:"unmatchedFacts"`
	MismatchedFacts int    `json:"mismatchedFacts"`
	MismatchCount   int    `json:"mismatchCount"`
}

type OracleSummary struct {
	Architecture    string `json:"architecture"`
	SourceID        string `json:"sourceId"`
	SDKVersion      string `json:"sdkVersion"`
	Profile         string `json:"profile"`
	ManifestSHA256  string `json:"manifestSha256"`
	CompilerFamily  string `json:"compilerFamily"`
	CompilerVersion string `json:"compilerVersion"`
	ABIFactCount    int    `json:"abiFactCount"`
	MatchedFacts    int    `json:"matchedFacts"`
	UnmatchedFacts  int    `json:"unmatchedFacts"`
	MismatchedFacts int    `json:"mismatchedFacts"`
}

type SymbolResult struct {
	Architecture      string   `json:"architecture"`
	SymbolID          string   `json:"symbolId"`
	NativeName        string   `json:"nativeName"`
	Kind              string   `json:"kind"`
	Complete          bool     `json:"complete"`
	MatchedFactIDs    []string `json:"matchedFactIds"`
	UnmatchedFactIDs  []string `json:"unmatchedFactIds"`
	MismatchedFactIDs []string `json:"mismatchedFactIds"`
}

type UnmatchedFact struct {
	Architecture string `json:"architecture"`
	FactID       string `json:"factId"`
	Reason       string `json:"reason"`
}

type Diagnostic struct {
	Severity     string `json:"severity"`
	Code         string `json:"code"`
	Architecture string `json:"architecture,omitempty"`
	Message      string `json:"message"`
}

type Mismatch struct {
	Code         string `json:"code"`
	Architecture string `json:"architecture,omitempty"`
	SymbolID     string `json:"symbolId,omitempty"`
	FactID       string `json:"factId,omitempty"`
	Path         string `json:"path"`
	Expected     string `json:"expected"`
	Actual       string `json:"actual"`
}

type Summary struct {
	OracleCount                     int      `json:"oracleCount"`
	ABIFactCount                    int      `json:"abiFactCount"`
	MatchedABIFactCount             int      `json:"matchedAbiFactCount"`
	UnmatchedABIFactCount           int      `json:"unmatchedAbiFactCount"`
	MismatchedABIFactCount          int      `json:"mismatchedAbiFactCount"`
	MatchedSymbolCount              int      `json:"matchedSymbolCount"`
	VerifiedSymbolCount             int      `json:"verifiedSymbolCount"`
	VerifiedSymbolArchitectureCount int      `json:"verifiedSymbolArchitectureCount"`
	MatchedSymbolIDs                []string `json:"matchedSymbolIds"`
	MismatchCount                   int      `json:"mismatchCount"`
}

// OracleResult mirrors the public oracle result schema instead of importing
// the ABI oracle implementation. This keeps the verification gate independent
// from the producer that generated the evidence.
type OracleResult struct {
	SchemaVersion  int               `json:"schemaVersion"`
	SourceID       string            `json:"sourceId"`
	SDKVersion     string            `json:"sdkVersion"`
	Profile        string            `json:"profile"`
	Architecture   string            `json:"architecture"`
	ManifestSHA256 string            `json:"manifestSha256"`
	Compiler       CompilerResult    `json:"compiler"`
	Records        []RecordResult    `json:"records"`
	Values         []ValueResult     `json:"values"`
	GUIDs          []GUIDResult      `json:"guids"`
	BitFields      []BitFieldResult  `json:"bitFields"`
	Functions      []FunctionResult  `json:"functions"`
	VTables        []VTableResult    `json:"vtables"`
	Conditions     []ConditionResult `json:"conditions"`
}

type CompilerResult struct {
	Family  string `json:"family"`
	Version string `json:"version"`
}

type RecordResult struct {
	ID         string        `json:"id"`
	NativeName string        `json:"nativeName"`
	Kind       string        `json:"kind"`
	Size       uint64        `json:"size"`
	Alignment  uint64        `json:"alignment"`
	Fields     []FieldResult `json:"fields"`
}

type FieldResult struct {
	ID         string `json:"id"`
	NativeName string `json:"nativeName"`
	Offset     uint64 `json:"offset"`
}

type ValueResult struct {
	ID         string `json:"id"`
	NativeName string `json:"nativeName"`
	Value      string `json:"value"`
}

type GUIDResult struct {
	ID         string `json:"id"`
	NativeName string `json:"nativeName"`
	Value      string `json:"value"`
}

type BitFieldResult struct {
	ID         string `json:"id"`
	NativeName string `json:"nativeName"`
	MaskHex    string `json:"maskHex"`
	BitOffset  uint64 `json:"bitOffset"`
	BitWidth   uint64 `json:"bitWidth"`
}

type FunctionResult struct {
	ID                string `json:"id"`
	NativeName        string `json:"nativeName"`
	TypeCompatible    bool   `json:"typeCompatible"`
	CallingConvention string `json:"callingConvention"`
}

type VTableResult struct {
	ID        string `json:"id"`
	Interface string `json:"interface"`
	Method    string `json:"method"`
	Index     uint64 `json:"index"`
}

type ConditionResult struct {
	ID         string `json:"id"`
	NativeName string `json:"nativeName"`
	Value      bool   `json:"value"`
}

// ProbeManifest is also duplicated intentionally: the consumer recomputes the
// canonical probe hash without calling the producer package.
type ProbeManifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	SourceID      string            `json:"sourceId"`
	SDKVersion    string            `json:"sdkVersion"`
	Profile       string            `json:"profile"`
	Architecture  string            `json:"architecture"`
	Defines       map[string]string `json:"defines,omitempty"`
	Includes      []string          `json:"includes"`
	Records       []RecordProbe     `json:"records,omitempty"`
	Values        []ValueProbe      `json:"values,omitempty"`
	GUIDs         []GUIDProbe       `json:"guids,omitempty"`
	BitFields     []BitFieldProbe   `json:"bitFields,omitempty"`
	Functions     []FunctionProbe   `json:"functions,omitempty"`
	VTables       []VTableProbe     `json:"vtables,omitempty"`
	Conditions    []ConditionProbe  `json:"conditions,omitempty"`
}

type RecordProbe struct {
	ID         string       `json:"id"`
	NativeName string       `json:"nativeName"`
	Kind       string       `json:"kind"`
	Fields     []FieldProbe `json:"fields,omitempty"`
}

type FieldProbe struct {
	ID         string `json:"id"`
	NativeName string `json:"nativeName"`
}

type ValueProbe struct {
	ID         string `json:"id"`
	NativeName string `json:"nativeName"`
	Expression string `json:"expression"`
	Unsigned   bool   `json:"unsigned,omitempty"`
}

type GUIDProbe struct {
	ID         string `json:"id"`
	NativeName string `json:"nativeName"`
	Expression string `json:"expression"`
}

type BitFieldProbe struct {
	ID         string `json:"id"`
	NativeName string `json:"nativeName"`
	Record     string `json:"record"`
	Field      string `json:"field"`
}

type FunctionProbe struct {
	ID                string `json:"id"`
	NativeName        string `json:"nativeName"`
	Expression        string `json:"expression"`
	ExpectedType      string `json:"expectedType"`
	CallingConvention string `json:"callingConvention"`
}

type VTableProbe struct {
	ID         string `json:"id"`
	Interface  string `json:"interface"`
	VTableType string `json:"vtableType"`
	Method     string `json:"method"`
}

type ConditionProbe struct {
	ID         string `json:"id"`
	NativeName string `json:"nativeName"`
	Expression string `json:"expression"`
}

func loadOracle(path string) (OracleResult, error) {
	var result OracleResult
	if err := decodeStrict(path, &result); err != nil {
		return result, fmt.Errorf("decode ABI result: %w", err)
	}
	if err := validateOracleRequiredFields(path); err != nil {
		return result, err
	}
	if err := result.Validate(); err != nil {
		return result, err
	}
	return result, nil
}

func validateOracleRequiredFields(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var root map[string]json.RawMessage
	if err = json.Unmarshal(b, &root); err != nil {
		return err
	}
	var problems []string
	require := func(object map[string]json.RawMessage, path string, names ...string) {
		for _, name := range names {
			if _, ok := object[name]; !ok {
				problems = append(problems, path+"/"+name+" is required")
			}
		}
	}
	require(root, "", "schemaVersion", "sourceId", "sdkVersion", "profile", "architecture", "manifestSha256", "compiler", "records", "values", "guids", "bitFields", "functions", "vtables", "conditions")
	if raw, ok := root["compiler"]; ok {
		var compiler map[string]json.RawMessage
		if json.Unmarshal(raw, &compiler) == nil {
			require(compiler, "/compiler", "family", "version")
		}
	}
	checkArray := func(name string, fields ...string) {
		raw, ok := root[name]
		if !ok {
			return
		}
		var values []map[string]json.RawMessage
		if json.Unmarshal(raw, &values) != nil {
			return
		}
		for index, value := range values {
			require(value, fmt.Sprintf("/%s/%d", name, index), fields...)
		}
	}
	checkArray("records", "id", "nativeName", "kind", "size", "alignment", "fields")
	if raw, ok := root["records"]; ok {
		var records []map[string]json.RawMessage
		if json.Unmarshal(raw, &records) == nil {
			for recordIndex, record := range records {
				var fields []map[string]json.RawMessage
				if json.Unmarshal(record["fields"], &fields) != nil {
					continue
				}
				for fieldIndex, field := range fields {
					require(field, fmt.Sprintf("/records/%d/fields/%d", recordIndex, fieldIndex), "id", "nativeName", "offset")
				}
			}
		}
	}
	checkArray("values", "id", "nativeName", "value")
	checkArray("guids", "id", "nativeName", "value")
	checkArray("bitFields", "id", "nativeName", "maskHex", "bitOffset", "bitWidth")
	checkArray("functions", "id", "nativeName", "typeCompatible", "callingConvention")
	checkArray("vtables", "id", "interface", "method", "index")
	checkArray("conditions", "id", "nativeName", "value")
	if len(problems) != 0 {
		sort.Strings(problems)
		return fmt.Errorf("invalid ABI result schema: %s", strings.Join(problems, "; "))
	}
	return nil
}

func loadProbeManifest(path string) (ProbeManifest, error) {
	var manifest ProbeManifest
	if err := decodeStrict(path, &manifest); err != nil {
		return manifest, fmt.Errorf("decode ABI probe manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func decodeStrict(path string, target any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err = dec.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err = dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return fmt.Errorf("trailing JSON: %w", err)
	}
	return nil
}

func (r OracleResult) Validate() error {
	var problems []string
	if r.SchemaVersion != SchemaVersion {
		problems = append(problems, fmt.Sprintf("schemaVersion must be %d", SchemaVersion))
	}
	for label, value := range map[string]string{
		"sourceId": r.SourceID, "sdkVersion": r.SDKVersion, "profile": r.Profile,
		"architecture": r.Architecture, "manifestSha256": r.ManifestSHA256,
		"compiler.family": r.Compiler.Family, "compiler.version": r.Compiler.Version,
	} {
		if strings.TrimSpace(value) == "" {
			problems = append(problems, label+" is required")
		}
	}
	if !validArchitecture(r.Architecture) {
		problems = append(problems, "architecture must be 386, amd64, arm64, or arm64ec")
	}
	if !sha256Pattern.MatchString(r.ManifestSHA256) {
		problems = append(problems, "manifestSha256 must be 64 lowercase hexadecimal characters")
	}
	if r.Compiler.Family != "msvc" && r.Compiler.Family != "clang-cl" {
		problems = append(problems, "compiler.family must be msvc or clang-cl")
	}
	if r.Records == nil {
		problems = append(problems, "records is required")
	}
	if r.Values == nil {
		problems = append(problems, "values is required")
	}
	if r.GUIDs == nil {
		problems = append(problems, "guids is required")
	}
	if r.BitFields == nil {
		problems = append(problems, "bitFields is required")
	}
	if r.Functions == nil {
		problems = append(problems, "functions is required")
	}
	if r.VTables == nil {
		problems = append(problems, "vtables is required")
	}
	if r.Conditions == nil {
		problems = append(problems, "conditions is required")
	}

	seen := map[string]string{}
	checkID := func(kind, id string) {
		if strings.TrimSpace(id) == "" {
			problems = append(problems, kind+" has an empty id")
			return
		}
		if previous, ok := seen[id]; ok {
			problems = append(problems, fmt.Sprintf("duplicate result id %q (%s and %s)", id, previous, kind))
		}
		seen[id] = kind
	}
	for _, record := range r.Records {
		checkID("record", record.ID)
		if record.NativeName == "" {
			problems = append(problems, fmt.Sprintf("record %q has an empty nativeName", record.ID))
		}
		if record.Kind != "struct" && record.Kind != "union" {
			problems = append(problems, fmt.Sprintf("record %q kind must be struct or union", record.ID))
		}
		if record.Alignment == 0 {
			problems = append(problems, fmt.Sprintf("record %q has zero alignment", record.ID))
		}
		if record.Fields == nil {
			problems = append(problems, fmt.Sprintf("record %q fields is required", record.ID))
		}
		fieldSeen := map[string]bool{}
		for _, field := range record.Fields {
			if field.ID == "" || field.NativeName == "" {
				problems = append(problems, fmt.Sprintf("record %q has a field with missing id or nativeName", record.ID))
			}
			if fieldSeen[field.ID] {
				problems = append(problems, fmt.Sprintf("record %q has duplicate field id %q", record.ID, field.ID))
			}
			fieldSeen[field.ID] = true
		}
	}
	for _, value := range r.Values {
		checkID("value", value.ID)
		if value.NativeName == "" || !decimalPattern.MatchString(value.Value) {
			problems = append(problems, fmt.Sprintf("value %q has invalid nativeName or decimal value", value.ID))
		}
	}
	for _, guid := range r.GUIDs {
		checkID("guid", guid.ID)
		if guid.NativeName == "" || !guidPattern.MatchString(guid.Value) {
			problems = append(problems, fmt.Sprintf("guid %q has invalid nativeName or value", guid.ID))
		}
	}
	for _, field := range r.BitFields {
		checkID("bit field", field.ID)
		if field.NativeName == "" || !maskPattern.MatchString(field.MaskHex) {
			problems = append(problems, fmt.Sprintf("bit field %q has invalid nativeName or maskHex", field.ID))
		}
	}
	for _, function := range r.Functions {
		checkID("function", function.ID)
		if function.NativeName == "" || function.CallingConvention == "" {
			problems = append(problems, fmt.Sprintf("function %q has missing nativeName or callingConvention", function.ID))
		}
		if !function.TypeCompatible {
			problems = append(problems, fmt.Sprintf("function %q is not type-compatible", function.ID))
		}
	}
	for _, table := range r.VTables {
		checkID("vtable", table.ID)
		if table.Interface == "" || table.Method == "" {
			problems = append(problems, fmt.Sprintf("vtable %q has missing interface or method", table.ID))
		}
	}
	for _, condition := range r.Conditions {
		checkID("condition", condition.ID)
		if condition.NativeName == "" {
			problems = append(problems, fmt.Sprintf("condition %q has an empty nativeName", condition.ID))
		}
	}
	if len(problems) != 0 {
		sort.Strings(problems)
		return fmt.Errorf("invalid ABI result: %s", strings.Join(problems, "; "))
	}
	return nil
}

func (m ProbeManifest) Validate() error {
	var problems []string
	if m.SchemaVersion != SchemaVersion {
		problems = append(problems, fmt.Sprintf("schemaVersion must be %d", SchemaVersion))
	}
	for label, value := range map[string]string{
		"sourceId": m.SourceID, "sdkVersion": m.SDKVersion,
		"profile": m.Profile, "architecture": m.Architecture,
	} {
		if strings.TrimSpace(value) == "" {
			problems = append(problems, label+" is required")
		}
	}
	if !validArchitecture(m.Architecture) {
		problems = append(problems, "architecture must be 386, amd64, arm64, or arm64ec")
	}
	if m.Includes == nil || len(m.Includes) == 0 {
		problems = append(problems, "at least one include is required")
	}
	seen := map[string]string{}
	check := func(kind, id string, values ...string) {
		if strings.TrimSpace(id) == "" {
			problems = append(problems, kind+" has an empty id")
		} else if previous, ok := seen[id]; ok {
			problems = append(problems, fmt.Sprintf("duplicate probe id %q (%s and %s)", id, previous, kind))
		} else {
			seen[id] = kind
		}
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				problems = append(problems, fmt.Sprintf("%s %q has an empty required property", kind, id))
			}
		}
	}
	for _, record := range m.Records {
		check("record", record.ID, record.NativeName, record.Kind)
		if record.Kind != "struct" && record.Kind != "union" {
			problems = append(problems, fmt.Sprintf("record %q kind must be struct or union", record.ID))
		}
		for _, field := range record.Fields {
			check("record field", record.ID+":"+field.ID, field.NativeName)
		}
	}
	for _, value := range m.Values {
		check("value", value.ID, value.NativeName, value.Expression)
	}
	for _, guid := range m.GUIDs {
		check("guid", guid.ID, guid.NativeName, guid.Expression)
	}
	for _, field := range m.BitFields {
		check("bit field", field.ID, field.NativeName, field.Record, field.Field)
	}
	for _, function := range m.Functions {
		check("function", function.ID, function.NativeName, function.Expression, function.ExpectedType, function.CallingConvention)
	}
	for _, table := range m.VTables {
		check("vtable", table.ID, table.Interface, table.VTableType, table.Method)
	}
	for _, condition := range m.Conditions {
		check("condition", condition.ID, condition.NativeName, condition.Expression)
	}
	if len(problems) != 0 {
		sort.Strings(problems)
		return fmt.Errorf("invalid ABI probe manifest: %s", strings.Join(problems, "; "))
	}
	return nil
}

func validArchitecture(value string) bool {
	switch value {
	case "386", "amd64", "arm64", "arm64ec":
		return true
	default:
		return false
	}
}

func (r OracleResult) FactCount() int {
	count := len(r.Values) + len(r.GUIDs) + len(r.VTables) + len(r.Conditions)
	count += len(r.BitFields) * 3
	count += len(r.Functions) * 2
	for _, record := range r.Records {
		count += 2 + len(record.Fields)
	}
	return count
}
