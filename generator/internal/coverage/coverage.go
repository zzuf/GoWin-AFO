// Package coverage accounts for every ingested symbol without conflating
// inventory, source generation, callability, ABI verification, and execution.
package coverage

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/zzuf/GoWin-AFO/generator/internal/model"
)

type Metric struct {
	Numerator   int     `json:"numerator"`
	Denominator int     `json:"denominator"`
	Percent     float64 `json:"percent"`
}

type Report struct {
	SchemaVersion    int            `json:"schemaVersion"`
	Scope            string         `json:"scope"`
	GeneratorVersion string         `json:"generatorVersion"`
	ManifestHash     string         `json:"manifestHash"`
	TotalSymbols     int            `json:"totalSymbols"`
	BySource         map[string]int `json:"bySource"`
	ByNamespace      map[string]int `json:"byNamespace"`
	ByKind           map[string]int `json:"byKind"`
	ByArchitecture   map[string]int `json:"byArchitecture"`
	ByBackend        map[string]int `json:"byBackend"`
	ByStatus         map[string]int `json:"byStatus"`
	BySkipReason     map[string]int `json:"bySkipReason"`
	ExcludedBySource map[string]int `json:"excludedBySource,omitempty"`
	ABIVerified      int            `json:"abiVerified"`
	ABIUnverified    int            `json:"abiUnverified"`
	ABIMismatches    int            `json:"abiMismatches"`
	RuntimeTested    int            `json:"runtimeSmokeTested"`
	ManualOverrides  int            `json:"manualOverrides"`
	BridgeCallable   int            `json:"bridgeCallable"`
	Metrics          Metrics        `json:"metrics"`
}

type Metrics struct {
	SourceIngestionCoverage      Metric `json:"sourceIngestionCoverage"`
	ProjectionAccountingCoverage Metric `json:"projectionAccountingCoverage"`
	GoSourceGenerationCoverage   Metric `json:"goSourceGenerationCoverage"`
	PureGoCallableCoverage       Metric `json:"pureGoCallableCoverage"`
	AssemblyCallableCoverage     Metric `json:"assemblyCallableCoverage"`
	BridgeCallableCoverage       Metric `json:"bridgeCallableCoverage"`
	ABIVerifiedCoverage          Metric `json:"abiVerifiedCoverage"`
	RuntimeSmokeTestedCoverage   Metric `json:"runtimeSmokeTestedCoverage"`
}

func Build(inv model.Inventory) Report {
	hasOfficialSources := false
	for _, s := range inv.Symbols {
		if s.SourceType != "project-fixture" {
			hasOfficialSources = true
			break
		}
	}
	scope := "vertical-slice-fixture"
	if hasOfficialSources {
		scope = "official-windows-sources"
	}
	r := Report{SchemaVersion: 1, Scope: scope, GeneratorVersion: inv.GeneratorVersion, ManifestHash: inv.ManifestHash, BySource: map[string]int{}, ByNamespace: map[string]int{}, ByKind: map[string]int{}, ByArchitecture: map[string]int{}, ByBackend: map[string]int{}, ByStatus: map[string]int{}, BySkipReason: map[string]int{}, ExcludedBySource: map[string]int{}}
	accounted, generated, pure, assembly, bridge := 0, 0, 0, 0, 0
	for _, s := range inv.Symbols {
		if hasOfficialSources && s.SourceType == "project-fixture" {
			r.ExcludedBySource[s.SourceID]++
			continue
		}
		r.TotalSymbols++
		r.BySource[s.SourceID]++
		r.ByNamespace[s.Namespace]++
		r.ByKind[string(s.Kind)]++
		architectures := s.Availability.Architectures
		if len(architectures) == 0 {
			architectures = []string{s.Architecture}
		}
		for _, architecture := range architectures {
			r.ByArchitecture[architecture]++
		}
		r.ByBackend[string(s.Backend)]++
		r.ByStatus[string(s.Status)]++
		if s.Status.Valid() {
			accounted++
		}
		switch s.Status {
		case model.StatusGeneratedPureGo, model.StatusGeneratedAssembly, model.StatusGeneratedCGOBridge, model.StatusGeneratedTypeOnly, model.StatusGeneratedManualOverride:
			generated++
		}
		switch s.Status {
		case model.StatusGeneratedPureGo:
			pure++
		case model.StatusGeneratedAssembly:
			assembly++
		case model.StatusGeneratedCGOBridge:
			if s.ABIVerified {
				bridge++
				r.BridgeCallable++
			}
		}
		if !strings.HasPrefix(string(s.Status), "generated-") {
			r.BySkipReason[s.StatusReason]++
		}
		if s.ABIVerified {
			r.ABIVerified++
		}
		if s.RuntimeSmokeTested {
			r.RuntimeTested++
		}
		if s.ManualOverride {
			r.ManualOverrides++
		}
	}
	r.ABIUnverified = r.TotalSymbols - r.ABIVerified
	r.Metrics = Metrics{SourceIngestionCoverage: metric(r.TotalSymbols, r.TotalSymbols), ProjectionAccountingCoverage: metric(accounted, r.TotalSymbols), GoSourceGenerationCoverage: metric(generated, r.TotalSymbols), PureGoCallableCoverage: metric(pure, r.TotalSymbols), AssemblyCallableCoverage: metric(assembly, r.TotalSymbols), BridgeCallableCoverage: metric(bridge, r.TotalSymbols), ABIVerifiedCoverage: metric(r.ABIVerified, r.TotalSymbols), RuntimeSmokeTestedCoverage: metric(r.RuntimeTested, r.TotalSymbols)}
	return r
}

func metric(n, d int) Metric {
	p := 0.0
	if d == 0 {
		p = 100
	} else {
		p = math.Round((float64(n) * 10000 / float64(d))) / 100
	}
	return Metric{Numerator: n, Denominator: d, Percent: p}
}

// Validate rejects truncated, stale, or internally inconsistent reports. CI
// must never accept an empty JSON object as a vacuous 0/0 coverage success.
func Validate(r Report) error {
	var problems []string
	if r.SchemaVersion != 1 {
		problems = append(problems, "schemaVersion must be 1")
	}
	if r.Scope != "official-windows-sources" && r.Scope != "vertical-slice-fixture" {
		problems = append(problems, "scope is missing or unknown")
	}
	if r.GeneratorVersion == "" {
		problems = append(problems, "generatorVersion is required")
	}
	if len(r.ManifestHash) != 64 {
		problems = append(problems, "manifestHash must be a SHA-256")
	} else if _, err := hex.DecodeString(r.ManifestHash); err != nil {
		problems = append(problems, "manifestHash is not hexadecimal")
	}
	if r.TotalSymbols <= 0 {
		problems = append(problems, "totalSymbols must be positive")
	}
	for name, values := range map[string]map[string]int{"bySource": r.BySource, "byNamespace": r.ByNamespace, "byKind": r.ByKind, "byBackend": r.ByBackend, "byStatus": r.ByStatus} {
		if sum, negative := sumCounts(values); negative || sum != r.TotalSymbols {
			problems = append(problems, fmt.Sprintf("%s must contain nonnegative counts summing to totalSymbols", name))
		}
	}
	if architectureCount, negative := sumCounts(r.ByArchitecture); negative || architectureCount < r.TotalSymbols {
		problems = append(problems, "byArchitecture must contain nonnegative counts covering every symbol")
	}
	if _, negative := sumCounts(r.ExcludedBySource); negative {
		problems = append(problems, "excludedBySource contains a negative count")
	}
	for status := range r.ByStatus {
		s := model.Status(status)
		if s != model.StatusUnclassified && !s.Valid() {
			problems = append(problems, "byStatus contains unknown status "+status)
		}
	}
	if r.ABIVerified < 0 || r.ABIUnverified < 0 || r.ABIVerified+r.ABIUnverified != r.TotalSymbols {
		problems = append(problems, "ABI verified and unverified counts must partition totalSymbols")
	}
	if r.RuntimeTested < 0 || r.RuntimeTested > r.TotalSymbols || r.ManualOverrides < 0 || r.ManualOverrides > r.TotalSymbols || r.ABIMismatches < 0 {
		problems = append(problems, "verification, runtime, override, or mismatch count is outside its valid range")
	}
	if r.BridgeCallable < 0 || r.BridgeCallable > r.ByStatus[string(model.StatusGeneratedCGOBridge)] || r.BridgeCallable > r.ABIVerified {
		problems = append(problems, "bridgeCallable must be an ABI-verified subset of generated-cgo-bridge")
	}
	generated := r.ByStatus[string(model.StatusGeneratedPureGo)] + r.ByStatus[string(model.StatusGeneratedAssembly)] + r.ByStatus[string(model.StatusGeneratedCGOBridge)] + r.ByStatus[string(model.StatusGeneratedTypeOnly)] + r.ByStatus[string(model.StatusGeneratedManualOverride)]
	if skipped, negative := sumCounts(r.BySkipReason); negative || skipped != r.TotalSymbols-generated {
		problems = append(problems, "bySkipReason must account for every non-generated symbol")
	}
	expected := map[string]struct {
		got  Metric
		want Metric
	}{
		"sourceIngestionCoverage":      {r.Metrics.SourceIngestionCoverage, metric(r.TotalSymbols, r.TotalSymbols)},
		"projectionAccountingCoverage": {r.Metrics.ProjectionAccountingCoverage, metric(r.TotalSymbols-r.ByStatus[string(model.StatusUnclassified)], r.TotalSymbols)},
		"goSourceGenerationCoverage":   {r.Metrics.GoSourceGenerationCoverage, metric(generated, r.TotalSymbols)},
		"pureGoCallableCoverage":       {r.Metrics.PureGoCallableCoverage, metric(r.ByStatus[string(model.StatusGeneratedPureGo)], r.TotalSymbols)},
		"assemblyCallableCoverage":     {r.Metrics.AssemblyCallableCoverage, metric(r.ByStatus[string(model.StatusGeneratedAssembly)], r.TotalSymbols)},
		"bridgeCallableCoverage":       {r.Metrics.BridgeCallableCoverage, metric(r.BridgeCallable, r.TotalSymbols)},
		"abiVerifiedCoverage":          {r.Metrics.ABIVerifiedCoverage, metric(r.ABIVerified, r.TotalSymbols)},
		"runtimeSmokeTestedCoverage":   {r.Metrics.RuntimeSmokeTestedCoverage, metric(r.RuntimeTested, r.TotalSymbols)},
	}
	for name, pair := range expected {
		if pair.got != pair.want {
			problems = append(problems, name+" does not match its source counts")
		}
	}
	if len(problems) != 0 {
		sort.Strings(problems)
		return fmt.Errorf("invalid coverage report: %s", strings.Join(problems, "; "))
	}
	return nil
}

func sumCounts(values map[string]int) (sum int, negative bool) {
	for _, value := range values {
		if value < 0 {
			negative = true
		}
		sum += value
	}
	return sum, negative
}

type Delta struct {
	TotalSymbols                int            `json:"totalSymbols"`
	ABIVerified                 int            `json:"abiVerified"`
	ABIUnverified               int            `json:"abiUnverified"`
	ABIMismatches               int            `json:"abiMismatches"`
	RuntimeTested               int            `json:"runtimeSmokeTested"`
	ManualOverrides             int            `json:"manualOverrides"`
	BridgeCallable              int            `json:"bridgeCallable"`
	BySource                    map[string]int `json:"bySource"`
	ByNamespace                 map[string]int `json:"byNamespace"`
	ByKind                      map[string]int `json:"byKind"`
	ByArchitecture              map[string]int `json:"byArchitecture"`
	ByStatus                    map[string]int `json:"byStatus"`
	ByBackend                   map[string]int `json:"byBackend"`
	BySkipReason                map[string]int `json:"bySkipReason"`
	SourceIngestionPercent      float64        `json:"sourceIngestionPercent"`
	ProjectionAccountingPercent float64        `json:"projectionAccountingPercent"`
	GoSourceGenerationPercent   float64        `json:"goSourceGenerationPercent"`
	PureGoCallablePercent       float64        `json:"pureGoCallablePercent"`
	AssemblyCallablePercent     float64        `json:"assemblyCallablePercent"`
	BridgeCallablePercent       float64        `json:"bridgeCallablePercent"`
	ABIVerifiedPercent          float64        `json:"abiVerifiedPercent"`
	RuntimeTestedPercent        float64        `json:"runtimeSmokeTestedPercent"`
}

func Diff(before, after Report) Delta {
	return Delta{
		TotalSymbols:                after.TotalSymbols - before.TotalSymbols,
		ABIVerified:                 after.ABIVerified - before.ABIVerified,
		ABIUnverified:               after.ABIUnverified - before.ABIUnverified,
		ABIMismatches:               after.ABIMismatches - before.ABIMismatches,
		RuntimeTested:               after.RuntimeTested - before.RuntimeTested,
		ManualOverrides:             after.ManualOverrides - before.ManualOverrides,
		BridgeCallable:              after.BridgeCallable - before.BridgeCallable,
		BySource:                    mapDelta(before.BySource, after.BySource),
		ByNamespace:                 mapDelta(before.ByNamespace, after.ByNamespace),
		ByKind:                      mapDelta(before.ByKind, after.ByKind),
		ByArchitecture:              mapDelta(before.ByArchitecture, after.ByArchitecture),
		ByStatus:                    mapDelta(before.ByStatus, after.ByStatus),
		ByBackend:                   mapDelta(before.ByBackend, after.ByBackend),
		BySkipReason:                mapDelta(before.BySkipReason, after.BySkipReason),
		SourceIngestionPercent:      percentDelta(before.Metrics.SourceIngestionCoverage, after.Metrics.SourceIngestionCoverage),
		ProjectionAccountingPercent: percentDelta(before.Metrics.ProjectionAccountingCoverage, after.Metrics.ProjectionAccountingCoverage),
		GoSourceGenerationPercent:   percentDelta(before.Metrics.GoSourceGenerationCoverage, after.Metrics.GoSourceGenerationCoverage),
		PureGoCallablePercent:       percentDelta(before.Metrics.PureGoCallableCoverage, after.Metrics.PureGoCallableCoverage),
		AssemblyCallablePercent:     percentDelta(before.Metrics.AssemblyCallableCoverage, after.Metrics.AssemblyCallableCoverage),
		BridgeCallablePercent:       percentDelta(before.Metrics.BridgeCallableCoverage, after.Metrics.BridgeCallableCoverage),
		ABIVerifiedPercent:          percentDelta(before.Metrics.ABIVerifiedCoverage, after.Metrics.ABIVerifiedCoverage),
		RuntimeTestedPercent:        percentDelta(before.Metrics.RuntimeSmokeTestedCoverage, after.Metrics.RuntimeSmokeTestedCoverage),
	}
}

func percentDelta(before, after Metric) float64 {
	return round2(after.Percent - before.Percent)
}
func mapDelta(a, b map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range a {
		out[k] -= v
	}
	for k, v := range b {
		out[k] += v
	}
	for k, v := range out {
		if v == 0 {
			delete(out, k)
		}
	}
	return out
}
func round2(v float64) float64 { return math.Round(v*100) / 100 }

func Check(r Report, baseline *Report, failUnclassified, failRegression bool) error {
	if err := Validate(r); err != nil {
		return err
	}
	if baseline != nil {
		if err := Validate(*baseline); err != nil {
			return fmt.Errorf("invalid coverage baseline: %w", err)
		}
	}
	var problems []string
	if failUnclassified && r.ByStatus[string(model.StatusUnclassified)] != 0 {
		problems = append(problems, fmt.Sprintf("unclassified symbols = %d", r.ByStatus[string(model.StatusUnclassified)]))
	}
	if r.ABIMismatches != 0 {
		problems = append(problems, fmt.Sprintf("ABI mismatches = %d", r.ABIMismatches))
	}
	if r.Metrics.ProjectionAccountingCoverage.Numerator != r.Metrics.ProjectionAccountingCoverage.Denominator {
		problems = append(problems, "projection accounting coverage is not 100%")
	}
	if failRegression {
		if baseline == nil {
			problems = append(problems, "coverage baseline is required for regression check")
		} else {
			if r.Scope != baseline.Scope {
				problems = append(problems, "coverage scope changed")
			}
			if r.TotalSymbols < baseline.TotalSymbols {
				problems = append(problems, "inventoried symbol count regressed")
			}
			if ratioLess(r.Metrics.ProjectionAccountingCoverage, baseline.Metrics.ProjectionAccountingCoverage) {
				problems = append(problems, "projection accounting coverage regressed")
			}
			if ratioLess(r.Metrics.ABIVerifiedCoverage, baseline.Metrics.ABIVerifiedCoverage) {
				problems = append(problems, "ABI verified coverage regressed")
			}
			if ratioLess(r.Metrics.GoSourceGenerationCoverage, baseline.Metrics.GoSourceGenerationCoverage) {
				problems = append(problems, "Go source generation coverage regressed")
			}
			if ratioLess(r.Metrics.PureGoCallableCoverage, baseline.Metrics.PureGoCallableCoverage) {
				problems = append(problems, "Pure Go callable coverage regressed")
			}
			if ratioLess(r.Metrics.AssemblyCallableCoverage, baseline.Metrics.AssemblyCallableCoverage) {
				problems = append(problems, "assembly callable coverage regressed")
			}
			if ratioLess(r.Metrics.BridgeCallableCoverage, baseline.Metrics.BridgeCallableCoverage) {
				problems = append(problems, "bridge callable coverage regressed")
			}
			if ratioLess(r.Metrics.RuntimeSmokeTestedCoverage, baseline.Metrics.RuntimeSmokeTestedCoverage) {
				problems = append(problems, "runtime smoke-tested coverage regressed")
			}
			if r.ABIMismatches > baseline.ABIMismatches {
				problems = append(problems, "ABI mismatch count increased")
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("coverage check failed: %s", strings.Join(problems, "; "))
	}
	return nil
}

func ratioLess(current, baseline Metric) bool {
	return int64(current.Numerator)*int64(baseline.Denominator) < int64(baseline.Numerator)*int64(current.Denominator)
}

func MarshalJSON(r Report) ([]byte, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func Markdown(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Coverage report\n\nScope: `%s`\n\nManifest: `%s`\n\nTotal inventoried symbols: **%d**\n\n", r.Scope, r.ManifestHash, r.TotalSymbols)
	fmt.Fprintf(&b, "No bare \"100%%\" claim is made: every percentage below names its denominator.\n\n")
	b.WriteString("| Metric | Numerator | Denominator | Percent |\n|---|---:|---:|---:|\n")
	rows := []struct {
		name string
		m    Metric
	}{{"Source ingestion coverage", r.Metrics.SourceIngestionCoverage}, {"Projection accounting coverage", r.Metrics.ProjectionAccountingCoverage}, {"Go source generation coverage", r.Metrics.GoSourceGenerationCoverage}, {"Pure Go callable coverage", r.Metrics.PureGoCallableCoverage}, {"Assembly callable coverage", r.Metrics.AssemblyCallableCoverage}, {"C bridge callable coverage", r.Metrics.BridgeCallableCoverage}, {"ABI verified coverage", r.Metrics.ABIVerifiedCoverage}, {"Runtime smoke-tested coverage", r.Metrics.RuntimeSmokeTestedCoverage}}
	for _, row := range rows {
		fmt.Fprintf(&b, "| %s | %d | %d | %.2f%% |\n", row.name, row.m.Numerator, row.m.Denominator, row.m.Percent)
	}
	b.WriteString("\n## Status accounting\n\n| Status | Symbols |\n|---|---:|\n")
	for _, k := range sortedKeys(r.ByStatus) {
		fmt.Fprintf(&b, "| `%s` | %d |\n", k, r.ByStatus[k])
	}
	b.WriteString("\n## Callable backends\n\n| Backend | Symbols |\n|---|---:|\n")
	for _, k := range sortedKeys(r.ByBackend) {
		fmt.Fprintf(&b, "| `%s` | %d |\n", k, r.ByBackend[k])
	}
	b.WriteString("\n## Verification accounting\n\n| Evidence | Symbols |\n|---|---:|\n")
	fmt.Fprintf(&b, "| ABI verified | %d |\n", r.ABIVerified)
	fmt.Fprintf(&b, "| ABI unverified | %d |\n", r.ABIUnverified)
	fmt.Fprintf(&b, "| ABI mismatches | %d |\n", r.ABIMismatches)
	fmt.Fprintf(&b, "| Runtime smoke-tested | %d |\n", r.RuntimeTested)
	fmt.Fprintf(&b, "| Manual overrides | %d |\n", r.ManualOverrides)
	fmt.Fprintf(&b, "| ABI-verified C bridge callable | %d |\n", r.BridgeCallable)
	writeCountTable(&b, "Symbols by source", "Source", r.BySource)
	writeCountTable(&b, "Symbols by namespace", "Namespace", r.ByNamespace)
	writeCountTable(&b, "Symbols by kind", "Kind", r.ByKind)
	writeCountTable(&b, "Symbols by architecture", "Architecture", r.ByArchitecture)
	writeCountTable(&b, "Skipped symbols by reason", "Reason", r.BySkipReason)
	writeCountTable(&b, "Excluded symbols by source", "Source", r.ExcludedBySource)
	return b.String()
}

func writeCountTable(b *strings.Builder, title, keyHeader string, values map[string]int) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## %s\n\n| %s | Symbols |\n|---|---:|\n", title, keyHeader)
	for _, k := range sortedKeys(values) {
		fmt.Fprintf(b, "| `%s` | %d |\n", strings.ReplaceAll(k, "|", "\\|"), values[k])
	}
}
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
