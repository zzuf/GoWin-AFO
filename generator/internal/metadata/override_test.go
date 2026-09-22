package metadata

import (
	"encoding/json"
	"testing"

	"go-windows-api.local/generator/internal/model"
)

func TestOverrideRequiresEvidenceAndDetectsStaleBefore(t *testing.T) {
	o := Override{ID: "o", SourceID: "s", SDKVersionRange: VersionRange{"1", "2"}, SymbolID: "id", Before: json.RawMessage(`{"nativeName":"F"}`), After: json.RawMessage(`{"goName":"Fixed"}`), Reason: "header disagrees", Evidence: []Evidence{{Kind: "header", Reference: "x.h:1"}}, Test: "TestF", RemovalCondition: "upstream fixes issue"}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	symbols := []model.Symbol{{ID: "id", SourceID: "s", NativeName: "Other"}}
	sources := []model.Source{{ID: "s", Version: "1.0.0"}}
	if err := ApplyOverrides(symbols, sources, []Override{o}); err == nil {
		t.Fatal("stale before accepted")
	}
}

func TestFixtureManualOverride(t *testing.T) {
	symbols := []model.Symbol{{ID: "sha256:717ca350486e67f7edae8b70055dd8adb0266f4be3c7df9c0a58aa1b3ea75fbb", SourceID: "fixture-e2e-v1", NativeName: "BROKEN_METADATA_TYPE", GoName: "BROKEN_METADATA_TYPE", Kind: model.KindNativeTypedef, Architecture: "neutral", ABIProfile: "windows-desktop", CanonicalSignature: "typedef uint32 BROKEN_METADATA_TYPE", Status: model.StatusGeneratedTypeOnly, StatusReason: "fixture", Backend: model.BackendTypeOnly, BackendReason: "fixture"}}
	o := Override{ID: "fixture", SourceID: "fixture-e2e-v1", SDKVersionRange: VersionRange{"1", "1"}, SymbolID: symbols[0].ID, Before: json.RawMessage(`{"goName":"BROKEN_METADATA_TYPE"}`), After: json.RawMessage(`{"goName":"CorrectedMetadataType"}`), Reason: "fixture", Evidence: []Evidence{{Kind: "abi-probe", Reference: "fixture"}}, Test: "this test", RemovalCondition: "fixture retired"}
	sources := []model.Source{{ID: "fixture-e2e-v1", Version: "1.0.0"}}
	if err := ApplyOverrides(symbols, sources, []Override{o}); err != nil {
		t.Fatal(err)
	}
	if symbols[0].GoName != "CorrectedMetadataType" || !symbols[0].ManualOverride || symbols[0].Status != model.StatusGeneratedTypeOnly {
		t.Fatalf("override not applied: %+v", symbols[0])
	}
}

func TestOverrideRejectsVersionOutsideRangeAndForbiddenMutation(t *testing.T) {
	symbol := model.Symbol{ID: "id", SourceID: "s", NativeName: "F", GoName: "F", Kind: model.KindTypedef, Architecture: "neutral", ABIProfile: "p", CanonicalSignature: "F", Status: model.StatusGeneratedTypeOnly, StatusReason: "ok", Backend: model.BackendTypeOnly, BackendReason: "type"}
	base := Override{ID: "o", SourceID: "s", SDKVersionRange: VersionRange{"1.0.0", "1.9.0"}, SymbolID: "id", Before: json.RawMessage(`{"goName":"F"}`), After: json.RawMessage(`{"goName":"Fixed"}`), Reason: "evidence", Evidence: []Evidence{{Kind: "header", Reference: "fixture.h"}}, Test: "Test", RemovalCondition: "upstream"}
	if err := ApplyOverrides([]model.Symbol{symbol}, []model.Source{{ID: "s", Version: "2.0.0"}}, []Override{base}); err == nil {
		t.Fatal("out-of-range override applied")
	}
	base.SDKVersionRange = VersionRange{"2.0.0", "2.0.0"}
	base.After = json.RawMessage(`{"sourceId":"other"}`)
	if err := ApplyOverrides([]model.Symbol{symbol}, []model.Source{{ID: "s", Version: "2.0.0"}}, []Override{base}); err == nil {
		t.Fatal("identity mutation applied")
	}
}

func TestOverrideCannotBypassCallableCapabilityClassification(t *testing.T) {
	symbol := model.Symbol{ID: "id", SourceID: "s", NativeName: "F", GoName: "F", Kind: model.KindFunction, Architecture: "neutral", ABIProfile: "p", CanonicalSignature: "F()", Status: model.StatusGeneratedPureGo, StatusReason: "proven", Backend: model.BackendPureGoSyscall, BackendReason: "integer ABI", Function: &model.Function{CallingConvention: "stdcall", Return: model.Type{Kind: model.KindPrimitive, GoType: "uint32"}}}
	after := json.RawMessage(`{"function":{"callingConvention":"vectorcall","return":{"kind":"primitive","goType":"float64"}}}`)
	o := Override{ID: "o", SourceID: "s", SDKVersionRange: VersionRange{"1", "1"}, SymbolID: "id", Before: json.RawMessage(`{"nativeName":"F"}`), After: after, Reason: "attempted ABI change", Evidence: []Evidence{{Kind: "abi-probe", Reference: "probe.json"}}, Test: "Test", RemovalCondition: "upstream"}
	if err := ApplyOverrides([]model.Symbol{symbol}, []model.Source{{ID: "s", Version: "1"}}, []Override{o}); err == nil {
		t.Fatal("callable function ABI override bypassed capability classification")
	}
}

func TestOverrideVersionComparisonIncludesPrerelease(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{{"71.0.14-preview", "71.0.14-rc", -1}, {"71.0.14-rc.2", "71.0.14-rc.10", -1}, {"71.0.14-preview", "71.0.14", -1}, {"71.0.14", "71.0.14-preview", 1}, {"71.0.14-preview", "71.0.14-preview", 0}}
	for _, tc := range cases {
		got, err := compareVersions(tc.left, tc.right)
		if err != nil || got != tc.want {
			t.Errorf("compareVersions(%q, %q) = %d, %v; want %d", tc.left, tc.right, got, err, tc.want)
		}
	}
}
