# Verification

## Layers of evidence

Verification results become stronger in the following order, but a higher layer does not implicitly replace a lower one.

1. Parser/normalizer unit tests
2. Generator golden tests and identical results from two generation runs
3. Cross-compilation of Go source for 386/amd64/arm64
4. Compile-time ABI assertions against C/C++ headers
5. Comparison of executed C/C++ probe results with Go layouts/values
6. Non-destructive runtime smoke tests on Windows

Go code compiling alone is not evidence of Windows ABI compatibility. Aggregate-by-value, float/vector, calling conventions, packed layouts, bit fields, callbacks, and COM vtables in particular require evidence from a native compiler.

## Go tests

Normal checks are:

```text
go test ./...
go vet ./...
gofmt -l .
```

CI builds/tests the generator and oracle on Linux, macOS, and Windows, and cross-compiles all packages and test binaries for `windows/386`, `windows/amd64`, and `windows/arm64` with `CGO_ENABLED=0`. Windows x86/x64 runners execute non-destructive smoke tests for raw Kernel32, COM, and WinRT.

The ECMA-335 parser has fuzz targets for compressed integers and the signature parser. Normal CI runs each target briefly as a smoke test; this does not replace long-running corpus fuzzing.

## ABI oracle

`tools/abi-oracle` generates deterministic C++17 probes from a reviewed JSON manifest.

```text
go run ./tools/abi-oracle validate --manifest tools/abi-oracle/testdata/probe-manifest.json
go run ./tools/abi-oracle generate --manifest tools/abi-oracle/testdata/probe-manifest.json --architecture amd64 --out probe.cpp
go run ./tools/abi-oracle validate --result actual.json
go run ./tools/abi-oracle compare --expected tools/abi-oracle/testdata/windows-sdk-10.0.26100-amd64.json --actual actual.json --out diff.json
go run ./cmd/winapiverify abi --all
```

JSON Schemas for the manifest/result are in `tools/abi-oracle/schema`. Probe IDs in the manifest must be stable and unique. Review include paths, defines, SDK version, profile, target architecture, and native expressions. Native expressions are C++ code, so do not insert external input directly into the manifest.

Generated probes output:

- `sizeof`, `alignof`, and `offsetof`
- Struct/union kind, size, and alignment
- Enum/macro/constant values
- Canonical GUID/IID/CLSID values
- Bit-field memory-order masks, bit offsets, and bit widths
- `std::is_same` compile-time assertions for function pointers and calling convention labels
- Method indexes of C-style COM vtables
- Target/preprocessor conditions

Wide integers are decimal strings to avoid JSON number precision problems. Probe headers contain the SDK version and canonical manifest SHA-256, but no generation date or absolute path. Results retain compiler identity as provenance. Comparison excludes only compiler identity so the same ABI facts from MSVC and clang-cl can be compared; SDK/profile/architecture/hash and every probe value are compared.

## Current fixture evidence

The Windows SDK `10.0.26100.0` fixture has checked-in JSON produced by executing x86/x64 probes with MSVC 19.44. It covers 5 records/unions, 2 constants, `IID_IUnknown`, 1 bit field, 2 function pointer types, 3 IUnknown vtable slots, and 2 conditions. This is evidence for a small vertical slice, not ABI verification of the entire SDK.

The ARM64 probe is cross-compiled with a target compiler to check the target guard and function type assertions. It is not executed on x64 runners, so no ARM64 result JSON or runtime-verified count is fabricated. An execution baseline for that target will be created only when an ARM64 hardware runner is added. ARM64EC has a distinct target status and is not equated with Go `arm64`.

## Comparison with Go layouts

To reflect an oracle result as “ABI verified” in the inventory, map its probe ID to an IR symbol ID/field ID and confirm that architecture/profile/sdkVersion/manifest hash match. Go tests compare `unsafe.Sizeof`, `unsafe.Alignof`, `unsafe.Offsetof`, bit masks from generated accessors, GUIDs, constants, and vtable indexes against the JSON.

The oracle comparator produces a machine-readable diff between executed C++ results and the checked-in baseline. `winapiverify` also checks the source lock, generated slice manifest and artifact hashes, and probe provenance; matches compatible facts such as record layouts and function types to normalized IR symbols; and writes `coverage/abi-latest.json`. Probe facts with no corresponding IR layout/symbol remain in `unmatchedFacts`, and not all generated Go symbols are marked verified. ARM64 is explicitly compile-only, and ARM64EC is unsupported.

The `--all` in `winapiverify abi --all` means all ABI evidence targets checked in for the vertical-slice fixture. It does not mean verification of every Windows SDK namespace or inventory symbol. The matching target is expected values in the normalized IR; it does not mean that every generated Go struct has been measured with `unsafe.Sizeof/Offsetof` or that every native call boundary has been executed. Those require separate Go layout tests and runtime smoke evidence.

## Runtime smoke tests

Normal CI exercises only non-destructive operations such as process/thread IDs, performance counters, system time, VirtualAlloc/VirtualFree, optional export availability, BSTR/HSTRING round trips, CoTaskMem, COM/WinRT apartments, and known WinRT activation factories.

Normal CI does not perform administrator operations, driver installation, service creation, registry writes, or system setting changes. Any registry additions are limited to read-only operations and known keys. Optional APIs are called only after an availability check.

## Handling failures

CI uses `vswhere` from Visual Studio Installer through `tools/ci/setup-msvc.cmd` to find an instance with C++ tools. It does not embed a fixed Visual Studio year/edition path. It checks the selected SDK version and header presence as well as the `vcvarsall` exit code. `tools/ci/setup-msvc.Tests.ps1` tests x86/x64 and failure paths for a missing SDK/locator.

For ABI mismatches, CI saves a JSON diff with expected/actual/path and the generated probe/compiler output as artifacts. If an SDK update changes layouts, GUIDs, signatures, or DLL mappings, do not rewrite expected JSON first to bypass the gate; inspect the official header/metadata diff, affected Go symbols, and whether an override is needed.

A successful cross-compile, an absent runtime runner, and an absent optional SDK are recorded separately. Unverified items are not treated as successes; they remain `ABIUnverified`, `external-sdk-not-installed`, or an appropriate non-callable status.
