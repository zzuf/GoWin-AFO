# Known limitations

This repository is a vertical slice of a comprehensive generation platform, not completed bindings for the entire Windows API. The limitations below are reflected openly in coverage and status.

## Source ingestion

- The WinMD reader conservatively inventories TypeDef/Field/MethodDef/Property/Event/GenericParam, but does not reconstruct the meaning of every ECMA-335 table, coded index, and custom attribute for every API feature.
- NativeArrayInfo, MemorySize, SupportedArchitecture, FreeWith, RetVal, nullability, associated enums, and availability from Win32 metadata are not projected onto every symbol.
- The Clang provider has not run the full target/profile matrix for Windows SDK headers. Full ingestion of macros, inline functions, SAL, pack, anonymous aggregates, and bit fields remains incomplete.
- There is no native TLB/OLB/DLL resource reader. The current type-library provider is a vertical slice using a JSON interchange/interface.
- The Windows App SDK meta package is pinned, but fetching, resolving conflicts among, and projecting the complete set of transitive NuGet packages containing API payloads is incomplete.
- Office and other external product APIs are not included in Windows OS coverage.

## Generation

- Besides the fixture, the general emitter outputs only the fixed-width integer subset of official P/Invoke functions without pointers. It does not turn pointers whose SAL/ownership/retention/alignment have not been projected, 64-bit values that occupy multiple words on x86, or every type and function in the full-source Win32/WDK/WinRT inventory into callable Go packages.
- The current `projection accounting coverage = 100%` is the classification rate for the inventoried set, not completion of every official SDK symbol.
- There is no verified assembly trampoline. Float/vector/varargs/special aggregate ABIs are not reduced uniformly to `uintptr` calls.
- The generated C bridge is a small fixture; it does not implement every special signature or C++ flattening.
- Not every header pattern for packed structs, complex unions/anonymous aggregates, signed bit fields, or flexible arrays has been verified.
- The ergonomic layer consists of a few runtime helpers; it does not provide Go string/slice/error/iterator/context wrappers for the entire raw API.
- The 13 runtime-smoke reference files in `bindings/win32`, `bindings/wdk`, and `bindings/winrt` are rebuilt by a dedicated reviewed template stage and hash-verified against `bindings/slice.manifest.json` (`SLICEGEN-001` resolved). They are not full-source projections derived from the general IR and are excluded from official generation rates. A change to an input pin requires another template/ABI review.

## COM

- Only consumer foundations for IUnknown, apartments, BSTR, and CoTaskMem are implemented.
- SAFEARRAY, VARIANT, PROPVARIANT, IDispatch, general coclass activation, and connection points are not implemented.
- There is no lifetime/reference-count/panic/thread registry for exposing Go objects as COM callbacks/servers.
- Oracle verification of all generated interface inheritance and vtable order is incomplete.

## WinRT

- Only foundations for HSTRING, RoInitialize, IInspectable, and activation are implemented.
- Parameterized IIDs return a not-implemented error. Guessed IIDs are not generated.
- General generation of delegates, event tokens, async actions/operations, progress/completion, collections, and closed generics is not implemented.
- Contract versions, threading models, marshaling behavior, and deprecation are not projected onto every runtime class.

## WDK

- Kernel-only exports are not made callable from ordinary Go processes. The standard Go runtime does not support kernel drivers.
- Fully automatic classification of user-mode/UMDF/type-only/kernel-only is incomplete.
- Provenance for SDK/WDK duplicates is retained, but complete semantic merging, including typedefs/custom attributes/layouts, is not implemented.
- There are no ABI probes for the full WINVER/NTDDI/architecture matrix of WDK headers.

## Verification

- The checked-in ABI oracle contains only executed x86/x64 results for a small Windows SDK 10.0.26100.0 fixture. It checks 5 records/unions, 2 constants, 1 GUID, 1 bit field, 2 function signatures, and 3 IUnknown slots, but independent oracle comparisons for packed records, callback ABI, float ABI, aggregate value ABI, flexible arrays, and the generated C bridge are not implemented (`ABI-001`).
- ARM64 is cross-compiled with C++/Go, but the probe/runtime is not executed on an x64-hosted runner. It is not reported as ARM64 runtime-verified.
- ARM64EC is recognized as a distinct ABI, but no callable backend is offered because there is no ordinary Go target.
- Automatic mapping between oracle results and all inventory symbols/Go layouts is incomplete, so ABI verified coverage is limited.
- Runtime smoke tests cover only a small number of non-destructive APIs. Administrator operations, drivers, services, registry writes, and system setting changes are not tested.
- The SDK update workflow goes only as far as discovering new versions and confirming reproducibility of the current pins. Automatically applying candidate locks/hashes to generate source and ABI diffs is not implemented, and the breaking report is currently file-level, not a semantic API compatibility report (`SDKUP-001`).

## Platform and distribution

- The module path has been migrated to `github.com/zzuf/GoWin-AFO` to match the specified repository. Publishing the source on GitHub does not guarantee a stable API/release; coverage and ABI limitations still apply.
- When an optional external SDK provider is absent, it is classified as `external-sdk-not-installed`. Pinned Windows SDK/WinRT sources are required, so their absence causes fetch/verify failure. Unexpanded transitive Windows App SDK API packages are distinguished as `missing-upstream-metadata`.

- Bindings/bridge/reports/raw dumps are all staged before sequential renames, and ordinary write errors roll back every target (improving the ordinary failure path of `ATOMIC-001`). If rollback itself fails, the backup is retained and its location included in the error. A durable journal for automatic recovery from a process crash/power loss between multiple renames is not implemented (`ATOMIC-CRASH-001`). In that case, inspect the remaining `.winapigen-stage-*` backups to recover, then regenerate to check consistency.
- SDK/header/NuGet payloads are subject to license and redistribution terms and generally are not vendored into the repository. Clean generation requires network access and, for WinRT/ABI verification, the relevant Windows SDK installation.
- The System32 loader is implemented, but a trusted absolute-path policy for arbitrary app-local DLLs is not.

## Avoiding false claims of completion

Unimplemented items are not assigned `generated-*`. They receive statuses such as `unsupported-go-abi`, `unsupported-projection`, `kernel-mode-only`, `missing-upstream-metadata`, and `external-sdk-not-installed` with reasons. If a new source/provider cannot recognize symbols, do not silently omit them from the denominator; update diagnostics and the ingestion limitations.
