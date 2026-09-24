# Coverage and accounting

## What is counted

This project counts “obtained,” “Go source generated,” “callable,” “ABI verified,” and “executed” as separate facts. A standalone `100%` figure is prohibited; always give the metric name, numerator, and denominator.

| Metric | Numerator | Denominator |
|---|---|---|
| source ingestion coverage | input records inventoried by the provider | input records recognized by the provider in this run |
| projection accounting coverage | symbols with a valid status and reason | inventory symbols |
| Go source generation coverage | symbols with a `generated-*` status | inventory symbols |
| Pure Go callable coverage | `generated-purego` | inventory symbols |
| assembly callable coverage | `generated-assembly` | inventory symbols |
| bridge callable coverage | `generated-cgo-bridge` and verified by the target ABI oracle | inventory symbols |
| ABI verified coverage | symbols matching the oracle for the target architecture | inventory symbols |
| runtime smoke-tested coverage | symbols exercised non-destructively on Windows | inventory symbols |

The current WinMD provider does not fully reconstruct every ECMA-335 table/custom attribute into native symbols. Thus, source ingestion coverage measures the set recognized by the provider; it does not prove completeness against the true total number of Windows SDK symbols.

## Reports

`coverage/latest.json` is the machine-readable report, and `coverage/latest.md` presents the same contents for people. In addition to the generator version and manifest hash, the JSON contains counts by source, namespace, kind, architecture, backend, status, and skip reason. ABI mismatches, manual overrides, unverified ABI counts, and runtime execution counts are separate fields.

```text
go run ./cmd/winapicoverage report
go run ./cmd/winapicoverage report --format json
go run ./cmd/winapicoverage diff coverage/baseline.json coverage/latest.json
go run ./cmd/winapicoverage check --fail-unclassified
go run ./cmd/winapicoverage check --fail-regression
```

Refer to the checked-in JSON for report values. If copying figures into a README or release note, include the manifest hash and do not combine different sources/profiles. Normal fixture generation uses 23 test symbols, but this fixture must not be included in Microsoft official API coverage. With `generate --all`, the inventory retains fixture provenance while automatically excluding `project-fixture` from the official report denominator and recording `scope=official-windows-sources` and `excludedBySource`. Read a fixture-only report separately under `scope=vertical-slice-fixture`.

## Status accounting

Every symbol has either a generated status or an explicit non-generated status with a nonempty reason. `unclassified` is not a valid status and fails both IR validation and the CI gate. `source-parse-error`, `missing-upstream-metadata`, `kernel-mode-only`, and `unsupported-go-abi` are accounted for, but are not implemented or callable.

`generated-type-only` means only that a layout, constant, or interface descriptor was emitted; it does not count a function as callable. `generated-cgo-bridge` counts as bridge callable only when a `windows && cgo` implementation has been generated and the target compiler ABI verified. A manual override is not a callability backend, so it retains the pre-override projection status and is counted separately in `manualOverrides`. `generated-manual-override` is reserved as a valid status for a future use in which the override itself constitutes a generated artifact/backend; it is not assigned automatically for a mere metadata correction.

## CI gates

The four most important gates are:

- projection accounting coverage = 100% of inventory
- unclassified symbols = 0
- ABI mismatches = 0
- generation drift = 0

`--fail-regression` compares against `coverage/baseline.json` and rejects decreases in projection accounting, Go source generation, Pure Go, assembly, ABI-verified C bridge, ABI verification, or runtime smoke coverage rates; decreases in total inventory count; and increases in ABI mismatches. The decision uses integer fractions, not rounded display percentages. `diff` also publishes changes in counts by source, namespace, kind, architecture, status, backend, skip reason, and verification as JSON.

Do not lower the baseline merely to pass the gate. Update it only in an SDK update pull request that reviews the source lock, generated diff, coverage delta, ABI evidence, and known increases in unverified cases together. Changes that hide previous denominators by changing a source ID or profile also require regression review.

## Relationship to the ABI oracle

Fixture results from `tools/abi-oracle` are independent evidence from C++ header probes. `go run ./cmd/winapiverify abi --all` checks the provenance of checked-in x86/x64 results against corresponding IR symbols/layout facts and produces `coverage/abi-latest.json`. Probes without a corresponding IR fact remain unmatched and do not increase `abiVerified` across the main coverage inventory. ARM64 cross-compilation proves signature/layout expressions compile; it does not count as ARM64 runtime execution.

## Reading the report

- The denominator for the Pure Go rate includes types, constants, and kernel-only symbols. Show a separate kind filter if a function-only rate is needed.
- If a symbol common to architectures has an architecture-specific ID, count each target separately.
- Report type libraries such as Office and external SDKs such as WebView2 separately from Windows OS sources.
- Retain provenance for duplicate metadata. Do not make source ingestion counts appear smaller by removing SDK/WDK duplicates.
- Keep deprecated/dangerous symbols in availability and status reasons rather than deleting them.
