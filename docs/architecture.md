# Architecture

## Overview

The design consolidates differences among inputs into a normalized IR with provenance, then derives output, coverage, and verification from that IR rather than generating Go directly from official inputs.

```text
sources.lock.json
       |
       v
winapisource -> sources/cache/<source-id> -> provider raw dump
                                               |
                                               v
                                      normalized Inventory
                                      (stable ID + reason)
                                               |
                                  typed override + re-normalize
                                               |
                         +---------------------+------------------+
                         |                     |                  |
                         v                     v                  v
                    Go emitter            C bridge          coverage JSON
                         |                                        |
                         v                                        v
              bindings/generated                     CI accounting gates
                         |
                         v
          runtime/winabi, runtime/com, runtime/winrt
```

The fixture is a small vertical slice exercising every feature, selected explicitly with `winapigen generate --fixture`. The full-source pass over official Win32/WDK WinMD also carries a conservative subset of P/Invoke functions whose types can be resolved using only fixed-width integers through to Pure Go raw wrappers. At this stage, custom attributes have not been projected, so pointers, wide scalars, special ABIs, and WinRT/COM methods are not included in the official callable subset. Symbols with unresolved type layouts or other unresolved details do not reach the emitter; only their inventory, status, and reason are published. The CLI rejects an unspecified scope and does not implicitly replace the full tree with fixture generation.

## Layers

### Source manager

`generator/internal/source` reads the lock file and obtains artifacts from fixed URLs or a Windows SDK installation into a staging directory. After verifying the SHA-256 of the entire artifact, it extracts only files listed in the lock and replaces `sources/cache/<source-id>`. During verification, it compares the extracted files actually read against the hashed archive entries again. All four sources in the current `generate --all` are required; missing or modified sources and hash mismatches cause failure.

### Provider

`generator/internal/metadata.Provider` is the entry point for each source type. Win32, WDK, and WinRT have WinMD providers using `microsoft/go-winmd`; headers have a Clang AST JSON provider; and type libraries have a JSON interchange provider. Windows App SDK and external SDKs have boundaries defined by interfaces and the source lock, but the provider that expands API payloads from transitive NuGet packages is incomplete.

Providers return `Source`, `Symbol`, a raw representation, and diagnostics. Parser failures are not silently ignored; where possible, they remain as `source-parse-error` or diagnostics.

### Normalized IR

The `Inventory` in `generator/internal/model` is the central data structure. Each `Symbol` has a source, namespace, kind, native name, architecture, ABI profile, canonical signature, generic arity, status/backend and reason, and provenance.

A stable ID is the SHA-256 of the following strings in order, separated by NUL bytes:

```text
source ID
namespace
symbol kind
native name
architecture
canonical signature
generic arity
ABI profile
```

Normalization regularizes whitespace, rejects ID collisions, and sorts sources, symbols, diagnostics, and provenance into stable order. Finally, the SHA-256 of the inventory JSON, which contains neither timestamps nor absolute paths, becomes the manifest hash.

### Override

Overrides do not inject arbitrary code into the IR. An override identifies a target symbol ID and source ID and records the expected current value in `before` and the change in `after` as JSON objects. The SDK version range, reason, evidence from a header/ABI probe/upstream issue, regression test, and removal condition after an upstream fix are required. A stale override whose `before` does not match stops generation.

### Projection and emitter

Projection classifies function signatures using a capability matrix. The type emitter separately outputs ordinary types, architecture-specific types, union storage/accessors, bit-field getters/setters, flexible-array headers/views, callback addresses, and COM vtables. The official function emitter currently handles only fixed-width integer raw P/Invoke functions without pointers, wide scalars passed by value, or unresolved named types. Because availability attributes have not yet been fully projected, official wrappers generate lazy resolution and availability checks for optional exports.

Namespaces are split into separate packages so importing one package does not compile every Windows API. The general emitter currently writes to `bindings/generated/<normalized-namespace>`, and the bridge writes to `bridge/generated`. The 13 files in `bindings/win32`, `bindings/wdk`, and `bindings/winrt` are reviewed reference slices for runtime smoke/layout tests. A dedicated stage rebuilds them from `generator/internal/slice/templates` and verifies their hashes against a manifest. Changes to source pins require review, and imports derive from the root go.mod. They are not counted as the full SDK surface generated by the general emitter.

### ABI runtime

`runtime/winabi` isolates DLL resolution, integer/pointer calls, last error, HRESULT/NTSTATUS, GUID, pointer width, and UTF-16 boundaries. Above it, `runtime/com` and `runtime/winrt` make native ownership and apartment/thread affinity explicit. Raw generated packages preserve native returns and pointers; ergonomic helpers are separated into the runtime.

## Determinism and atomic updates

- Sort sources, symbols, files, and imports explicitly instead of depending on map iteration.
- Generated headers contain the generator version, source ID/version, and manifest hash, but no date or local absolute path.
- Pass Go source through `go/format` before output.
- Complete `bindings/generated`, `bridge/generated`, reviewed slices, inventory/coverage, and requested raw dumps in a staging tree within the workspace.
- Do not replace a tree without the ownership marker `.winapigen.json`, with extra files absent from the marker, with symlinks, or with handwritten files lacking generated headers.
- Preflight every output destination before renaming them in sequence; on ordinary errors, restore the previous output. If rollback fails, retain the backup and report its location.
- Keep inventory and coverage reports synchronized within the stage. A durable journal for process crashes between multiple renames is not implemented, so the update is not claimed to be equivalent to a single atomic OS operation.

## Error and coverage design

IR validation rejects `unclassified`. Symbols that are not generated also need a status and reason. Coverage counts ingestion, accounting, source generation, backend callability, ABI verification, and runtime execution separately. The most important CI conditions are 100% projection accounting, 0 unclassified symbols, 0 ABI mismatches, and 0 generation drift, with the denominator stated for each metric.

However, the current full-source inventory does not cover every WinMD table or custom attribute. Thus, accounting at 100% currently describes only the set that has been inventoried; it does not prove completeness across all SDK symbols.

## Trust boundaries and safeguards

Input artifacts are not trusted before hash verification. Downloads use HTTPS and have a 1 GiB size limit. Only ZIP entries listed in the lock are extracted, and absolute paths and `..` are rejected. Generated paths are also prevented from escaping the staging root.

At runtime, system DLL basenames and ASCII export names are validated. The loader uses System32 and does not search the current directory or general `PATH`. A trusted absolute path policy for app-local DLLs is not implemented, so loading app-local DLLs from external input is currently unavailable.

`unsafe` is confined to the ABI runtime, generated raw layer, and verification. Pointer, callback, COM reference, HSTRING/BSTR/CoTaskMem lifetimes are not hidden, and finalizers are not the sole means of release.

## Dependency direction

Dependencies generally flow in one direction: `bindings -> runtime/winabi`, `runtime/com -> runtime/winabi`, and `runtime/winrt -> runtime/com + runtime/winabi`. Foundation types live in a shared package rather than being redefined across namespaces. The generator emits from the IR and does not import generated bindings. This separation lets the generator build and run tests on Linux/macOS, while build tags restrict Windows ABI calls to Windows.
