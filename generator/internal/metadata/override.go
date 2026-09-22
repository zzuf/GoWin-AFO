package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/zzuf/GoWin-AFO/generator/internal/model"
)

type OverrideDocument struct {
	SchemaVersion int        `json:"schemaVersion"`
	Overrides     []Override `json:"overrides"`
}

type Override struct {
	ID               string          `json:"overrideId"`
	SourceID         string          `json:"sourceId"`
	SDKVersionRange  VersionRange    `json:"sdkVersionRange"`
	SymbolID         string          `json:"symbolId"`
	Before           json.RawMessage `json:"before"`
	After            json.RawMessage `json:"after"`
	Reason           string          `json:"reason"`
	Evidence         []Evidence      `json:"evidence"`
	Test             string          `json:"test"`
	RemovalCondition string          `json:"removalCondition"`
}

type VersionRange struct {
	MinInclusive string `json:"minInclusive"`
	MaxInclusive string `json:"maxInclusive"`
}
type Evidence struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	SHA256    string `json:"sha256,omitempty"`
}

func LoadOverrides(root string) ([]Override, error) {
	var out []Override
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".json") || d.Name() == "schema.json" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		var doc OverrideDocument
		if err = dec.Decode(&doc); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if doc.SchemaVersion != 1 {
			return fmt.Errorf("%s: unsupported schema %d", path, doc.SchemaVersion)
		}
		for i := range doc.Overrides {
			if err = doc.Overrides[i].Validate(); err != nil {
				return fmt.Errorf("%s override %d: %w", path, i, err)
			}
			out = append(out, doc.Overrides[i])
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	for i := 1; i < len(out); i++ {
		if out[i-1].ID == out[i].ID {
			return nil, fmt.Errorf("duplicate override ID %q", out[i].ID)
		}
	}
	return out, nil
}

func (o Override) Validate() error {
	if o.ID == "" || o.SourceID == "" || o.SymbolID == "" || o.SDKVersionRange.MinInclusive == "" || o.SDKVersionRange.MaxInclusive == "" || len(o.Before) == 0 || len(o.After) == 0 || o.Reason == "" || len(o.Evidence) == 0 || o.Test == "" || o.RemovalCondition == "" {
		return errors.New("override lacks mandatory provenance, version, diff, test, or removal condition")
	}
	for _, e := range o.Evidence {
		if e.Kind == "" || e.Reference == "" {
			return errors.New("override evidence lacks kind or reference")
		}
	}
	if comparison, err := compareVersions(o.SDKVersionRange.MinInclusive, o.SDKVersionRange.MaxInclusive); err != nil || comparison > 0 {
		return errors.New("override SDK version range is invalid or reversed")
	}
	return nil
}

// ApplyOverrides only changes fields explicitly present in the typed replacement
// object and requires the recorded before object to match the current symbol.
func ApplyOverrides(symbols []model.Symbol, sources []model.Source, overrides []Override) error {
	byID := make(map[string]*model.Symbol, len(symbols))
	for i := range symbols {
		byID[symbols[i].ID] = &symbols[i]
	}
	versions := make(map[string]string, len(sources))
	for _, source := range sources {
		versions[source.ID] = source.Version
	}
	for _, o := range overrides {
		s := byID[o.SymbolID]
		if s == nil {
			return fmt.Errorf("override %s target %s not found", o.ID, o.SymbolID)
		}
		if s.SourceID != o.SourceID {
			return fmt.Errorf("override %s source mismatch", o.ID)
		}
		version, ok := versions[o.SourceID]
		if !ok {
			return fmt.Errorf("override %s source version is not present in inventory", o.ID)
		}
		inRange, err := versionInRange(version, o.SDKVersionRange)
		if err != nil {
			return fmt.Errorf("override %s version range: %w", o.ID, err)
		}
		if !inRange {
			return fmt.Errorf("override %s does not apply to source version %s", o.ID, version)
		}
		var before, after map[string]json.RawMessage
		if err := json.Unmarshal(o.Before, &before); err != nil {
			return fmt.Errorf("override %s before: %w", o.ID, err)
		}
		if err := json.Unmarshal(o.After, &after); err != nil {
			return fmt.Errorf("override %s after: %w", o.ID, err)
		}
		for key := range before {
			if !overrideBeforeFields[key] {
				return fmt.Errorf("override %s before.%s is not an allowed typed field", o.ID, key)
			}
		}
		for key := range after {
			if !overrideAfterFields[key] {
				return fmt.Errorf("override %s after.%s cannot be changed by an override", o.ID, key)
			}
		}
		if _, changesFunctionABI := after["function"]; changesFunctionABI && isCallableGeneratedStatus(s.Status) {
			return fmt.Errorf("override %s changes an emitted function ABI; projection capability must be rerun instead", o.ID)
		}
		currentBytes, _ := json.Marshal(s)
		var current map[string]json.RawMessage
		_ = json.Unmarshal(currentBytes, &current)
		for key, want := range before {
			got, ok := current[key]
			if !ok || !jsonEqual(got, want) {
				return fmt.Errorf("override %s stale before.%s", o.ID, key)
			}
		}
		for key, value := range after {
			current[key] = value
		}
		merged, _ := json.Marshal(current)
		var replacement model.Symbol
		if err := json.Unmarshal(merged, &replacement); err != nil {
			return fmt.Errorf("override %s replacement: %w", o.ID, err)
		}
		replacement.ManualOverride = true
		// An override is provenance, not a callability backend. Preserve the
		// projection status so a non-callable symbol cannot become generated just
		// because metadata was corrected, and so an existing callable wrapper is
		// not silently removed from the emitter. ManualOverrides is reported as an
		// independent coverage dimension.
		replacement.StatusReason = strings.TrimSuffix(replacement.StatusReason, ";") + ";typed-override:" + o.ID
		if err := replacement.Validate(); err != nil {
			return fmt.Errorf("override %s produced invalid symbol: %w", o.ID, err)
		}
		*s = replacement
	}
	return nil
}

func isCallableGeneratedStatus(status model.Status) bool {
	switch status {
	case model.StatusGeneratedPureGo, model.StatusGeneratedAssembly, model.StatusGeneratedCGOBridge:
		return true
	default:
		return false
	}
}

var overrideAfterFields = map[string]bool{
	"goName": true, "type": true, "function": true, "constant": true,
	"guid": true, "availability": true, "ownership": true, "attributes": true,
}

var overrideBeforeFields = map[string]bool{
	"nativeName": true, "canonicalSignature": true, "goName": true, "type": true,
	"function": true, "constant": true, "guid": true, "availability": true,
	"ownership": true, "attributes": true,
}

func versionInRange(version string, r VersionRange) (bool, error) {
	min, err := compareVersions(version, r.MinInclusive)
	if err != nil {
		return false, err
	}
	max, err := compareVersions(version, r.MaxInclusive)
	if err != nil {
		return false, err
	}
	return min >= 0 && max <= 0, nil
}

func compareVersions(left, right string) (int, error) {
	type parsedVersion struct {
		core       []int
		prerelease []string
	}
	parse := func(value string) (parsedVersion, error) {
		value = strings.SplitN(value, "+", 2)[0]
		pieces := strings.SplitN(value, "-", 2)
		parts := strings.Split(pieces[0], ".")
		out := make([]int, len(parts))
		for i, part := range parts {
			if part == "" {
				return parsedVersion{}, fmt.Errorf("invalid version %q", value)
			}
			n, err := strconv.Atoi(part)
			if err != nil || n < 0 {
				return parsedVersion{}, fmt.Errorf("invalid version %q", value)
			}
			out[i] = n
		}
		parsed := parsedVersion{core: out}
		if len(pieces) == 2 {
			if pieces[1] == "" {
				return parsedVersion{}, fmt.Errorf("invalid version %q", value)
			}
			parsed.prerelease = strings.Split(pieces[1], ".")
			for _, identifier := range parsed.prerelease {
				if identifier == "" {
					return parsedVersion{}, fmt.Errorf("invalid version %q", value)
				}
			}
		}
		return parsed, nil
	}
	a, err := parse(left)
	if err != nil {
		return 0, err
	}
	b, err := parse(right)
	if err != nil {
		return 0, err
	}
	length := len(a.core)
	if len(b.core) > length {
		length = len(b.core)
	}
	for i := 0; i < length; i++ {
		av, bv := 0, 0
		if i < len(a.core) {
			av = a.core[i]
		}
		if i < len(b.core) {
			bv = b.core[i]
		}
		if av < bv {
			return -1, nil
		}
		if av > bv {
			return 1, nil
		}
	}
	if len(a.prerelease) == 0 && len(b.prerelease) == 0 {
		return 0, nil
	}
	if len(a.prerelease) == 0 {
		return 1, nil
	}
	if len(b.prerelease) == 0 {
		return -1, nil
	}
	for i := 0; i < len(a.prerelease) && i < len(b.prerelease); i++ {
		av, aerr := strconv.Atoi(a.prerelease[i])
		bv, berr := strconv.Atoi(b.prerelease[i])
		switch {
		case aerr == nil && berr == nil:
			if av < bv {
				return -1, nil
			}
			if av > bv {
				return 1, nil
			}
		case aerr == nil:
			return -1, nil
		case berr == nil:
			return 1, nil
		default:
			if comparison := strings.Compare(a.prerelease[i], b.prerelease[i]); comparison != 0 {
				return comparison, nil
			}
		}
	}
	if len(a.prerelease) < len(b.prerelease) {
		return -1, nil
	}
	if len(a.prerelease) > len(b.prerelease) {
		return 1, nil
	}
	return 0, nil
}

func jsonEqual(a, b []byte) bool {
	var av, bv any
	if json.Unmarshal(a, &av) != nil || json.Unmarshal(b, &bv) != nil {
		return false
	}
	aa, _ := json.Marshal(av)
	bb, _ := json.Marshal(bv)
	return bytes.Equal(aa, bb)
}
