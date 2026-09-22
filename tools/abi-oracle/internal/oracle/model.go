// Package oracle generates small, reviewable C++ ABI probes from declarative
// manifests and compares their machine-readable results.
package oracle

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

const (
	SchemaVersion    = 1
	GeneratorVersion = "0.1.0"
)

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.:-]*$`)
	headerPattern     = regexp.MustCompile(`^[A-Za-z0-9_+./-]+$`)
)

// Manifest is an executable specification of facts that must be obtained from
// the Microsoft headers. Expressions and native type names are trusted C++
// source and therefore must only come from reviewed repository manifests.
type Manifest struct {
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

// Result is emitted by a compiled probe. Values that may exceed JSON's exact
// integer range are deliberately represented as strings.
type Result struct {
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

func LoadManifest(path string) (Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Manifest{}, errors.New("decode manifest: trailing JSON value")
	}
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func (m Manifest) Validate() error {
	var problems []string
	if m.SchemaVersion != SchemaVersion {
		problems = append(problems, fmt.Sprintf("schemaVersion must be %d", SchemaVersion))
	}
	for label, value := range map[string]string{
		"sourceId": m.SourceID, "sdkVersion": m.SDKVersion, "profile": m.Profile,
	} {
		if value == "" {
			problems = append(problems, label+" is required")
		}
	}
	switch m.Architecture {
	case "386", "amd64", "arm64", "arm64ec":
	default:
		problems = append(problems, "architecture must be 386, amd64, arm64, or arm64ec")
	}
	if len(m.Includes) == 0 {
		problems = append(problems, "at least one include is required")
	}
	for _, include := range m.Includes {
		if !headerPattern.MatchString(include) || strings.Contains(include, "..") {
			problems = append(problems, fmt.Sprintf("unsafe include %q", include))
		}
	}
	for name := range m.Defines {
		if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(name) {
			problems = append(problems, fmt.Sprintf("invalid define name %q", name))
		}
	}
	for name, value := range m.Defines {
		if !safeCPPFragment(value) {
			problems = append(problems, fmt.Sprintf("define %q has an empty or multiline value", name))
		}
	}
	seen := map[string]bool{}
	check := func(kind, id string, required map[string]string) {
		if !identifierPattern.MatchString(id) {
			problems = append(problems, fmt.Sprintf("%s has invalid id %q", kind, id))
		} else if seen[id] {
			problems = append(problems, fmt.Sprintf("duplicate probe id %q", id))
		}
		seen[id] = true
		for label, value := range required {
			if strings.TrimSpace(value) == "" {
				problems = append(problems, fmt.Sprintf("%s %q: %s is required", kind, id, label))
			} else if !safeCPPFragment(value) {
				problems = append(problems, fmt.Sprintf("%s %q: %s must be a single C++ source line", kind, id, label))
			}
		}
	}
	for _, p := range m.Records {
		check("record", p.ID, map[string]string{"nativeName": p.NativeName, "kind": p.Kind})
		if p.Kind != "struct" && p.Kind != "union" {
			problems = append(problems, fmt.Sprintf("record %q: kind must be struct or union", p.ID))
		}
		fieldSeen := map[string]bool{}
		for _, f := range p.Fields {
			if f.ID == "" || f.NativeName == "" {
				problems = append(problems, fmt.Sprintf("record %q has an incomplete field", p.ID))
			}
			if fieldSeen[f.ID] {
				problems = append(problems, fmt.Sprintf("record %q has duplicate field id %q", p.ID, f.ID))
			}
			fieldSeen[f.ID] = true
		}
	}
	for _, p := range m.Values {
		check("value", p.ID, map[string]string{"nativeName": p.NativeName, "expression": p.Expression})
	}
	for _, p := range m.GUIDs {
		check("guid", p.ID, map[string]string{"nativeName": p.NativeName, "expression": p.Expression})
	}
	for _, p := range m.BitFields {
		check("bit field", p.ID, map[string]string{"nativeName": p.NativeName, "record": p.Record, "field": p.Field})
	}
	for _, p := range m.Functions {
		check("function", p.ID, map[string]string{"nativeName": p.NativeName, "expression": p.Expression, "expectedType": p.ExpectedType, "callingConvention": p.CallingConvention})
	}
	for _, p := range m.VTables {
		check("vtable", p.ID, map[string]string{"interface": p.Interface, "vtableType": p.VTableType, "method": p.Method})
	}
	for _, p := range m.Conditions {
		check("condition", p.ID, map[string]string{"nativeName": p.NativeName, "expression": p.Expression})
	}
	if len(problems) != 0 {
		sort.Strings(problems)
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func safeCPPFragment(value string) bool {
	return strings.TrimSpace(value) != "" && !strings.ContainsAny(value, "\x00\r\n")
}
