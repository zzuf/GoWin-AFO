# SDK Updates

## Principles

Change source versions and SHA-256 values only in `sources.lock.json`. Do not treat a floating `latest`, retrieval date, or developer-machine-specific path as authoritative. Do not automatically merge SDK/WDK/WinRT/Windows App SDK updates into main; review generation, coverage, ABI, license, and breaking changes in a pull request.

## Checking for Update Candidates

```text
go run ./cmd/winapisource update --dry-run
```

This command only reports candidates; it does not change the lock file. When updating transitive dependencies of a NuGet meta-package, pin every package that actually contains an API payload with its own source ID, version, and hash. For the local Windows SDK provider, also confirm that the target version is installed on the runner.

Compare numeric components, the fourth revision component, and prerelease labels without relying on NuGet array order. Suggest only stable versions for a stable pin, and both stable and prerelease versions for a prerelease pin. Do not suggest versions older than the current one, and fail on empty, invalid, or larger-than-4-MiB responses. Because the NuGet content index also includes unlisted versions, finding a candidate does not imply approval of its support status or suitability for adoption. See [NuGet versioning](https://learn.microsoft.com/en-us/nuget/concepts/package-versioning) and the [content index](https://learn.microsoft.com/en-us/nuget/api/package-base-address-resource).

`.github/workflows/sdk-update.yml` produces candidate discovery JSON weekly or on demand, but does not automatically apply a candidate lock file. The subsequent generated patch, coverage delta, binding/runtime file-level changes, and release-note draft are artifacts about **reproducibility of the current pin**. The stage that pins a candidate version's hash and produces generation and ABI diffs, along with symbol-level compatibility assessment, is not implemented (`SDKUP-001`). Run the following manual procedure in a pull request that adopts a candidate.

## Manual Update Procedure

1. Confirm the version and published license in the official package feed / SDK installer.
2. Retrieve the entire artifact and calculate its SHA-256. Do not guess or copy a hash.
3. Record the source ID, type, package, version, retrieval information, SHA-256, license identifier, architectures, SDK version, file list, and dependencies in the lock file.
4. Do not add SDK/header/NuGet payloads to Git if redistribution is not permitted. Keep the cache ignored.
5. Run `fetch` and `verify` in a clean environment.
6. Run `generate --all` and review the inventory, generated Go/C, and coverage.
7. Run `winapicoverage diff` and the regression gate. Confirm that no new symbol has `unclassified` status or an empty reason.
8. Run the x86/x64 ABI probes and cross-compile for ARM64. Update runtime results only if ARM64 hardware is available.
9. Compile generated packages for the applicable 386/amd64/arm64 configurations with cgo off/on.
10. Run nondestructive smoke tests on Windows.
11. Generate twice from the same inputs and confirm that the second run leaves a clean diff.
12. Combine source, coverage, ABI, breaking changes, known limitations, license/NOTICE, and release notes in one review.

## Coverage Regression

When new APIs are added, the ABI-verified percentage can fall even if they are classified correctly. Do not mechanically lower the baseline. Show unverified symbols by kind/source/architecture/backend, then decide whether to add probes or document explicit reasons for their unverified status.

Keep projection accounting at 100% at all times. Even if the parser cannot interpret a new table or signature, do not silently discard it; record `source-parse-error` or `unsupported-projection` with a diagnostic. If the source-ingestion denominator itself changes, also explain the differences in tables/attributes recognized by the provider.

## Updating the ABI Baseline

Create ABI results from native compiler measurements for the same manifest and target. Do not manually adjust sizes, offsets, or GUIDs to match. If a difference is an intended official SDK change, add a new baseline for the affected source version and architecture; do not overwrite the old SDK baseline.

If a function-pointer assertion fails, do not relax the expected type while leaving the generated signature callable. Investigate the metadata, header, calling convention, and target macros, then correct the projection, override, or backend classification.

## Updating Overrides

If an SDK update causes an override's `before` value to stop matching, generation should stop. If upstream has fixed the issue, check the removal condition and evidence, then remove the override and regression test. If it remains unfixed, adapt the version range and `before/after` values to the new SDK, and add supporting header or ABI-probe evidence. Do not add overrides without evidence or an end condition.

## Release Notes

Include at least the following:

- old/new source IDs, versions, SHA-256 values, and SDK contracts
- inventory symbol count and differences by source/kind/architecture
- changes in Pure Go/assembly/bridge/type-only/unsupported counts
- changes in ABI verified/unverified/mismatch and runtime execution counts
- additions, removals, renames, and signature changes in the public Go API
- changes to required OS versions, DLLs, and availability
- additions/removals of manual overrides
- license/NOTICE changes
- scope of compile-only verification on ARM64 and similar targets

The assessment workflow does not publish release notes; it retains them as an artifact. Do not create a package/release until review and an explicit release operation have taken place.
