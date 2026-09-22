package coverage

import (
	"strings"
	"testing"

	"github.com/zzuf/GoWin-AFO/generator/internal/model"
)

func TestMetricsStaySeparate(t *testing.T) {
	inv := model.Inventory{GeneratorVersion: "v", ManifestHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Symbols: []model.Symbol{{SourceID: "s", Namespace: "n", Kind: model.KindFunction, Architecture: "amd64", Status: model.StatusGeneratedPureGo, StatusReason: "ok", Backend: model.BackendPureGoSyscall, ABIVerified: true, RuntimeSmokeTested: true}, {SourceID: "s", Namespace: "n", Kind: model.KindFunction, Architecture: "amd64", Status: model.StatusUnsupportedGoABI, StatusReason: "float", Backend: model.BackendUnsupported}}}
	r := Build(inv)
	if r.Metrics.ProjectionAccountingCoverage.Percent != 100 || r.Metrics.PureGoCallableCoverage.Percent != 50 || r.Metrics.ABIVerifiedCoverage.Percent != 50 {
		t.Fatalf("%+v", r.Metrics)
	}
	if err := Check(r, nil, true, false); err != nil {
		t.Fatal(err)
	}
}

func TestMarkdownPublishesAllAccountingDimensions(t *testing.T) {
	r := Build(model.Inventory{GeneratorVersion: "v", ManifestHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Symbols: []model.Symbol{{SourceID: "s", SourceType: "win32-winmd", Namespace: "n", Kind: model.KindFunction, Architecture: "amd64", Status: model.StatusUnsupportedGoABI, StatusReason: "special-abi", Backend: model.BackendUnsupported}}})
	markdown := Markdown(r)
	for _, section := range []string{"Symbols by source", "Symbols by namespace", "Symbols by kind", "Symbols by architecture", "Skipped symbols by reason", "Verification accounting"} {
		if !strings.Contains(markdown, section) {
			t.Errorf("Markdown omitted %q", section)
		}
	}
}

func TestDiffIncludesEveryAccountingDimension(t *testing.T) {
	before := Build(model.Inventory{GeneratorVersion: "v", ManifestHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Symbols: []model.Symbol{{SourceID: "old", SourceType: "win32-winmd", Namespace: "n1", Kind: model.KindStruct, Architecture: "386", Status: model.StatusUnsupportedProjection, StatusReason: "old-reason", Backend: model.BackendTypeOnly}}})
	after := Build(model.Inventory{GeneratorVersion: "v", ManifestHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Symbols: []model.Symbol{{SourceID: "new", SourceType: "win32-winmd", Namespace: "n2", Kind: model.KindFunction, Architecture: "amd64", Status: model.StatusGeneratedPureGo, StatusReason: "generated", Backend: model.BackendPureGoSyscall, ABIVerified: true, RuntimeSmokeTested: true}}})
	delta := Diff(before, after)
	if delta.BySource["old"] != -1 || delta.BySource["new"] != 1 || delta.ByNamespace["n1"] != -1 || delta.ByKind[string(model.KindFunction)] != 1 || delta.ByArchitecture["amd64"] != 1 || delta.BySkipReason["old-reason"] != -1 {
		t.Fatalf("incomplete delta: %+v", delta)
	}
	if delta.ABIVerified != 1 || delta.RuntimeTested != 1 || delta.PureGoCallablePercent != 100 {
		t.Fatalf("verification/metric delta missing: %+v", delta)
	}
}
func TestRegressionGate(t *testing.T) {
	base := Build(model.Inventory{GeneratorVersion: "v", ManifestHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Symbols: []model.Symbol{{SourceID: "s", SourceType: "win32-winmd", Namespace: "n", Kind: model.KindStruct, Architecture: "neutral", Status: model.StatusGeneratedTypeOnly, StatusReason: "ok", Backend: model.BackendTypeOnly, ABIVerified: true}, {SourceID: "s", SourceType: "win32-winmd", Namespace: "n", Kind: model.KindStruct, Architecture: "neutral", Status: model.StatusGeneratedTypeOnly, StatusReason: "ok", Backend: model.BackendTypeOnly, ABIVerified: true}}})
	now := base
	now.ABIVerified = 1
	now.ABIUnverified = 1
	now.Metrics.ABIVerifiedCoverage = metric(1, 2)
	if err := Check(now, &base, true, true); err == nil {
		t.Fatal("ABI coverage regression accepted")
	}
}

func TestValidateRejectsEmptyReport(t *testing.T) {
	if err := Validate(Report{}); err == nil {
		t.Fatal("empty report passed validation")
	}
}

func TestRatioRegressionIsNotHiddenByRoundedPercent(t *testing.T) {
	baseline := metric(444, 600000)
	current := metric(443, 600000)
	if baseline.Percent != current.Percent {
		t.Fatalf("test requires equal displayed percentages, got %v and %v", baseline.Percent, current.Percent)
	}
	if !ratioLess(current, baseline) {
		t.Fatal("exact ratio regression was hidden by display rounding")
	}
}

func TestOfficialScopeExcludesVerticalSliceFixture(t *testing.T) {
	inv := model.Inventory{Symbols: []model.Symbol{
		{SourceID: "fixture", SourceType: "project-fixture", Namespace: "n", Kind: model.KindFunction, Architecture: "neutral", Status: model.StatusGeneratedPureGo, StatusReason: "fixture", Backend: model.BackendPureGoSyscall},
		{SourceID: "official", SourceType: "win32-winmd", Namespace: "n", Kind: model.KindFunction, Architecture: "neutral", Status: model.StatusUnsupportedProjection, StatusReason: "pending", Backend: model.BackendUnsupported},
	}}
	r := Build(inv)
	if r.Scope != "official-windows-sources" || r.TotalSymbols != 1 || r.BySource["fixture"] != 0 || r.ExcludedBySource["fixture"] != 1 {
		t.Fatalf("fixture leaked into official denominator: %+v", r)
	}
	if r.Metrics.PureGoCallableCoverage.Numerator != 0 {
		t.Fatal("fixture callable leaked into official numerator")
	}
}
