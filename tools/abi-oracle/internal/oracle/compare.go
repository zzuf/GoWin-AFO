package oracle

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

var (
	resultSHA256Pattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	resultDecimalPattern = regexp.MustCompile(`^-?[0-9]+$`)
	resultGUIDPattern    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	resultMaskPattern    = regexp.MustCompile(`^(?:[0-9a-f]{2})+$`)
)

type Comparison struct {
	SchemaVersion int        `json:"schemaVersion"`
	Equal         bool       `json:"equal"`
	Mismatches    []Mismatch `json:"mismatches"`
}

type Mismatch struct {
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

func LoadResult(path string) (Result, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	var result Result
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&result); err != nil {
		return Result{}, fmt.Errorf("decode ABI result: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return Result{}, errors.New("decode ABI result: trailing JSON value")
	}
	if err := result.Validate(); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (r Result) Validate() error {
	var problems []string
	if r.SchemaVersion != SchemaVersion {
		problems = append(problems, fmt.Sprintf("schemaVersion must be %d", SchemaVersion))
	}
	for label, value := range map[string]string{
		"sourceId": r.SourceID, "sdkVersion": r.SDKVersion, "profile": r.Profile,
		"architecture": r.Architecture, "manifestSha256": r.ManifestSHA256,
		"compiler.family": r.Compiler.Family, "compiler.version": r.Compiler.Version,
	} {
		if value == "" {
			problems = append(problems, label+" is required")
		}
	}
	switch r.Architecture {
	case "386", "amd64", "arm64", "arm64ec":
	default:
		problems = append(problems, "architecture is invalid")
	}
	if !resultSHA256Pattern.MatchString(r.ManifestSHA256) {
		problems = append(problems, "manifestSha256 must be lowercase SHA-256")
	}
	if r.Compiler.Family != "msvc" && r.Compiler.Family != "clang-cl" {
		problems = append(problems, "compiler.family must be msvc or clang-cl")
	}
	seen := map[string]string{}
	check := func(kind, id string) {
		if id == "" {
			problems = append(problems, kind+" has empty id")
			return
		}
		if previous, ok := seen[id]; ok {
			problems = append(problems, fmt.Sprintf("duplicate result id %q (%s and %s)", id, previous, kind))
		}
		seen[id] = kind
	}
	for _, p := range r.Records {
		check("record", p.ID)
		if p.NativeName == "" || (p.Kind != "struct" && p.Kind != "union") || p.Size == 0 || p.Alignment == 0 {
			problems = append(problems, fmt.Sprintf("record %q has invalid identity, kind, size, or alignment", p.ID))
		}
		fieldIDs := map[string]bool{}
		for _, field := range p.Fields {
			if field.ID == "" || field.NativeName == "" {
				problems = append(problems, fmt.Sprintf("record %q has an empty field id", p.ID))
			}
			if fieldIDs[field.ID] {
				problems = append(problems, fmt.Sprintf("record %q has duplicate field id %q", p.ID, field.ID))
			}
			fieldIDs[field.ID] = true
		}
	}
	for _, p := range r.Values {
		check("value", p.ID)
		if p.NativeName == "" || !resultDecimalPattern.MatchString(p.Value) {
			problems = append(problems, fmt.Sprintf("value %q is invalid", p.ID))
		}
	}
	for _, p := range r.GUIDs {
		check("guid", p.ID)
		if p.NativeName == "" || !resultGUIDPattern.MatchString(p.Value) {
			problems = append(problems, fmt.Sprintf("GUID %q is invalid", p.ID))
		}
	}
	for _, p := range r.BitFields {
		check("bit field", p.ID)
		if p.NativeName == "" || p.BitWidth == 0 || !resultMaskPattern.MatchString(p.MaskHex) {
			problems = append(problems, fmt.Sprintf("bit field %q is invalid", p.ID))
		}
	}
	for _, p := range r.Functions {
		check("function", p.ID)
		if p.NativeName == "" || p.CallingConvention == "" {
			problems = append(problems, fmt.Sprintf("function %q has incomplete identity", p.ID))
		}
		if !p.TypeCompatible {
			problems = append(problems, fmt.Sprintf("function %q is not type-compatible", p.ID))
		}
	}
	for _, p := range r.VTables {
		check("vtable", p.ID)
		if p.Interface == "" || p.Method == "" {
			problems = append(problems, fmt.Sprintf("vtable fact %q has incomplete identity", p.ID))
		}
	}
	for _, p := range r.Conditions {
		check("condition", p.ID)
		if p.NativeName == "" {
			problems = append(problems, fmt.Sprintf("condition %q has empty native name", p.ID))
		}
	}
	if len(problems) != 0 {
		sort.Strings(problems)
		return fmt.Errorf("invalid ABI result: %s", strings.Join(problems, "; "))
	}
	return nil
}

// Compare reports all semantic ABI differences. Compiler identity is retained
// as provenance in each result but intentionally excluded: MSVC and clang-cl
// must be able to prove the same ABI facts.
func Compare(expected, actual Result) Comparison {
	expected = sortedResult(expected)
	actual = sortedResult(actual)
	expected.Compiler = CompilerResult{}
	actual.Compiler = CompilerResult{}

	var left, right any
	leftBytes, _ := json.Marshal(expected)
	rightBytes, _ := json.Marshal(actual)
	_ = json.Unmarshal(leftBytes, &left)
	_ = json.Unmarshal(rightBytes, &right)

	comparison := Comparison{SchemaVersion: SchemaVersion, Mismatches: []Mismatch{}}
	compareValue("", left, right, &comparison.Mismatches)
	sort.Slice(comparison.Mismatches, func(i, j int) bool {
		return comparison.Mismatches[i].Path < comparison.Mismatches[j].Path
	})
	comparison.Equal = len(comparison.Mismatches) == 0
	return comparison
}

func compareValue(path string, expected, actual any, out *[]Mismatch) {
	leftMap, leftIsMap := expected.(map[string]any)
	rightMap, rightIsMap := actual.(map[string]any)
	if leftIsMap && rightIsMap {
		keys := make(map[string]bool, len(leftMap)+len(rightMap))
		for key := range leftMap {
			keys[key] = true
		}
		for key := range rightMap {
			keys[key] = true
		}
		ordered := make([]string, 0, len(keys))
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			compareValue(path+"/"+key, leftMap[key], rightMap[key], out)
		}
		return
	}
	leftSlice, leftIsSlice := expected.([]any)
	rightSlice, rightIsSlice := actual.([]any)
	if leftIsSlice && rightIsSlice {
		maximum := len(leftSlice)
		if len(rightSlice) > maximum {
			maximum = len(rightSlice)
		}
		for i := 0; i < maximum; i++ {
			var left, right any
			if i < len(leftSlice) {
				left = leftSlice[i]
			}
			if i < len(rightSlice) {
				right = rightSlice[i]
			}
			compareValue(fmt.Sprintf("%s/%d", path, i), left, right, out)
		}
		return
	}
	if reflect.DeepEqual(expected, actual) {
		return
	}
	*out = append(*out, Mismatch{Path: path, Expected: jsonText(expected), Actual: jsonText(actual)})
}

func jsonText(value any) string {
	if value == nil {
		return "<missing>"
	}
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(b)
}

func sortedResult(r Result) Result {
	r.Records = append([]RecordResult(nil), r.Records...)
	for i := range r.Records {
		r.Records[i].Fields = append([]FieldResult(nil), r.Records[i].Fields...)
		sort.Slice(r.Records[i].Fields, func(a, b int) bool { return r.Records[i].Fields[a].ID < r.Records[i].Fields[b].ID })
	}
	r.Values = append([]ValueResult(nil), r.Values...)
	r.GUIDs = append([]GUIDResult(nil), r.GUIDs...)
	r.BitFields = append([]BitFieldResult(nil), r.BitFields...)
	r.Functions = append([]FunctionResult(nil), r.Functions...)
	r.VTables = append([]VTableResult(nil), r.VTables...)
	r.Conditions = append([]ConditionResult(nil), r.Conditions...)
	sort.Slice(r.Records, func(i, j int) bool { return r.Records[i].ID < r.Records[j].ID })
	sort.Slice(r.Values, func(i, j int) bool { return r.Values[i].ID < r.Values[j].ID })
	sort.Slice(r.GUIDs, func(i, j int) bool { return r.GUIDs[i].ID < r.GUIDs[j].ID })
	sort.Slice(r.BitFields, func(i, j int) bool { return r.BitFields[i].ID < r.BitFields[j].ID })
	sort.Slice(r.Functions, func(i, j int) bool { return r.Functions[i].ID < r.Functions[j].ID })
	sort.Slice(r.VTables, func(i, j int) bool { return r.VTables[i].ID < r.VTables[j].ID })
	sort.Slice(r.Conditions, func(i, j int) bool { return r.Conditions[i].ID < r.Conditions[j].ID })
	return r
}

func MarshalComparison(comparison Comparison) ([]byte, error) {
	b, err := json.MarshalIndent(comparison, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
