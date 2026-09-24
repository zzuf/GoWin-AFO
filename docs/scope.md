# Scope

## Current state

This repository is not a completed collection of bindings for the entire Windows API. Based on Windows SDK `10.0.26100.0`, the current implementation is a small end-to-end vertical slice connecting retrieval, hash verification, WinMD reading, normalized IR, typed overrides, deterministic Go output, runtime ABI support, and coverage accounting.

The committed vertical slice includes Win32 foundation types and some Kernel32 APIs, foundational COM/WinRT runtimes, and WDK type-only examples. `generator/testdata/e2e/source.json` contains 23 validation records, but this is the project's own fixture and must not count in the denominator for official Microsoft API coverage.

`winapigen generate --all` conservatively inventories `TypeDef`, `Field`, `MethodDef`, `Property`, `Event`, and `GenericParam` from retrieved WinMD. Of the official metadata's P/Invoke functions, it outputs only the fixed-width integer subset with fixed DLL/entry points and without pointers, 64-bit values passed by value, special ABIs, or unresolved named types as `generated-purego` under namespace-specific `bindings/generated` packages. Pointers are not generated until SAL, retention period, ownership, and x86 alignment are interpreted. Most other symbols remain classified with reasons, so full Go projection of Win32, WDK, WinRT, and Windows App SDK has not been achieved.

## Target API surface

Long-term targets are:

- Public Win32 APIs in the Windows SDK
- Interfaces, GUIDs, vtables, and ownership information for COM consumers
- Windows Runtime / WinRT contract metadata and runtime classes
- WDK types, constants, and APIs proven usable in user mode
- Windows App SDK WinMD and native APIs
- Macros, inline functions, bit fields, packing, and conditional declarations in SDK headers that are absent from WinMD
- Independent inventories of TLB, OLB, and DLL-embedded type libraries
- External SDK providers for WebView2, DirectX Agility SDK, DirectStorage, and similar products

External product type libraries and SDKs have separate source IDs and coverage denominators from Windows OS APIs. Office, for example, is not mixed into OS coverage.

## Currently implemented vertical slice

| Area | Current implementation | Not guaranteed |
|---|---|---|
| Source | NuGet / local Windows SDK retrieval pinned by version and SHA-256, selected file extraction, reverification | Automatic discovery of arbitrary SDK installations; all transitive App SDK API dependencies |
| WinMD | PE/ECMA-335 reading with `microsoft/go-winmd`; conservative inventory of TypeDef/Field/MethodDef/Property/Event/GenericParam | Complete interpretation of custom attributes, symbolization of every table row, complete layouts |
| Header | Clang AST JSON provider and 386/amd64/arm64 target/profile settings | Integrated execution over all SDK headers; safe Go translation of macro expansion results and inline functions |
| Type library | JSON interchange and provider interface | Native TLB/OLB/DLL resource reader; Automation marshaling |
| IR | Stable IDs, provenance, types, functions, availability, ownership, status/backend | Complete reconstruction of every official metadata attribute |
| Go output | Deterministic generation of fixture types/unions/bit fields/flexible arrays/COM/C bridge and a provable simple P/Invoke subset from official Win32 metadata into namespace packages | Callable output for every official WinMD type, function, and namespace |
| Runtime | System32-only lazy loader, integer/pointer calls, HRESULT/NTSTATUS, UTF-16, IUnknown, BSTR, CoTaskMem, HSTRING, IInspectable, activation | Special ABIs in general, COM servers, SAFEARRAY/VARIANT, WinRT async/events/generic IIDs |
| WDK | Type-only projection of representative types; explicit classification of kernel-only functions | Building and running kernel drivers with the standard Go runtime |

## “Complete coverage” and percentages

Complete coverage in this project does not mean that everything is callable from Pure Go. It means that for every symbol recognized from official inputs, the stable ID, provenance, architecture/profile, generation status or non-generation reason, and ABI verification status are machine-readable.

Do not conflate these metrics:

- source ingestion coverage: rate of inventorying input recognized by the provider
- projection accounting coverage: rate with a valid status within the current inventory
- Go source generation coverage: rate for which Go source was emitted
- Pure Go callable coverage: rate actually callable with the Pure Go backend
- assembly callable coverage: rate callable with verified assembly trampolines
- bridge callable coverage: rate callable through a C bridge
- ABI verified coverage: rate that passed the ABI oracle or equivalent verification
- runtime smoke-tested coverage: rate exercised by non-destructive smoke tests on Windows

The fixture may have `projection accounting coverage = 100%`, but that is not 100% of the entire Windows SDK. The current source-ingestion metric also uses records already in the inventory as its denominator; it does not prove completeness against all table rows in WinMD. Do not advertise “100%” as coverage of the entire official SDK.

## Status classification

Every inventory symbol has a status and a nonempty reason. `unclassified` is invalid during normalization and fails the coverage gate.

| Status | Meaning |
|---|---|
| `generated-purego` | Callable through the corresponding runtime as an integer/pointer ABI, or a safe Pure Go projection |
| `generated-assembly` | Uses an architecture-specific verified assembly trampoline |
| `generated-cgo-bridge` | Requires a generated C ABI bridge |
| `generated-type-only` | Types, constants, or descriptors only; no claim of callability |
| `generated-manual-override` | Override applied with evidence, SDK range, test, and removal condition |
| `unsupported-go-abi` | No available Go-side backend can prove the ABI |
| `unsupported-projection` | Input inventoried, but safe language projection is not implemented |
| `kernel-mode-only` | Kernel API that cannot be called from an ordinary user-mode Go process |
| `missing-upstream-metadata` | Required official metadata is missing |
| `external-sdk-not-installed` | Optional external or local SDK is not installed |
| `undocumented-out-of-scope` | Excluded to avoid guessing, such as undocumented APIs |
| `license-restricted` | Redistribution or processing is restricted by license |
| `source-parse-error` | Corrupt input or input that the parser cannot interpret safely |

`unsupported-projection` is an additional conservative state in the current IR and does not indicate implementation. Even for `generated-*`, do not infer callability without checking the backend and build tags.

## Target architectures and profiles

The configured targets are `windows/386`, `windows/amd64`, and `windows/arm64`. Profiles distinguish `windows-desktop`, `windows-appcontainer`, and `windows-wdk`. The fixture and some cross-compilation/layout tests are designed to cover these three architectures, but that does not mean runtime execution on ARM64 hardware or ABI execution verification for the entire SDK has been completed. ARM64EC is not treated as ordinary Go `arm64` and is currently out of scope.

## Explicit non-guarantees

- Do not guess signatures for undocumented NT APIs.
- Do not expose kernel-only symbols as callable from ordinary Go processes.
- Do not flatten float, vector, varargs, aggregate-by-value, or special callback ABIs into uniform `uintptr` sequences.
- Do not emit packed structs or alignments that Go cannot express as ordinary structs that merely look the same.
- Do not provide stubs that make bridge APIs appear usable with `CGO_ENABLED=0`.
- Do not eagerly load DLLs or every API merely because a package is imported.
- Raw APIs do not necessarily provide convenient Go `error`, strings, slices, or automatic resource ownership.

## Security boundaries

System DLLs are limited to basenames from pinned metadata and lazily loaded with `LOAD_LIBRARY_SEARCH_SYSTEM32`. External input is not passed unchecked as a DLL or export name. NUL-terminated Win32 strings reject embedded NULs, while BSTR/HSTRING retain length-prefixed semantics. Go pointer lifetimes are managed with `runtime.KeepAlive` and explicit ownership. Pointers or callbacks retained by native code after a call are not classified as safe until dedicated allocation/registry/bridge support is complete.
