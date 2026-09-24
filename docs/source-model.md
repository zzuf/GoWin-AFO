# Input source model

## The lock file is authoritative

`sources.lock.json` is the authoritative input. Neither the local cache nor the latest NuGet version is authoritative. Each source pins at least:

- `id`: source identifier used in provenance and stable IDs
- `type`: source type used to select a provider
- `package` and `version`
- `retrieval`: fixed NuGet URL or `windows-sdk://` locator
- `sha256` of the entire artifact
- `licenseIdentifier`
- `architectures` and `windowsSDKVersion`
- `files` to extract into the cache
- `dependsOn`
- `required`: whether absence causes fetch/verify failure

Retrieval dates are not recorded in the lock or generated artifacts. Inputs with the same version and hash are treated as identical.

## Currently pinned sources

| Source ID | Input | Version | SDK | Required |
|---|---|---:|---:|---:|
| `microsoft-win32metadata` | `Microsoft.Windows.SDK.Win32Metadata` | `71.0.26-preview` | `10.0.26100.0` | yes |
| `microsoft-wdkmetadata` | `Microsoft.Windows.WDK.Win32Metadata` | `0.13.25-experimental` | `10.0.26100.0` | yes |
| `windows-sdk-winrt` | UnionMetadata from `Microsoft.Windows.SDK.CPP` | `10.0.26100.7705` | `10.0.26100.0` | yes |
| `windows-app-sdk` | `Microsoft.WindowsAppSDK` meta package | `2.5.1` | `10.0.26100.0` target | yes |

The Windows App SDK lock currently pins only the meta package itself. Expanding the versions, hashes, and providers for the transitive packages containing APIs is not implemented. This is treated as a `missing-upstream-metadata` sentinel for incomplete upstream expansion, rather than an indication of whether the package is installed. WinRT `Windows.winmd` is a required source retrieved from a pinned official NuGet package; absence causes fetch/verify failure.

The installation directory remains `10.0.26100.0` even when the SDK servicing revision differs, so its name alone cannot pin the input. WinRT uses `c/UnionMetadata/10.0.26100.0/Windows.winmd` from `Microsoft.Windows.SDK.CPP/10.0.26100.7705` and locks the SHA-256 of the entire NuGet archive. The WinMD file's SHA-256, `e2dee80d011cb9fc1276a0bd9f244f7a58d5ca72fe906a56e90d61c68cf8601a`, has been confirmed to match the former installed-SDK input. The provider selects the sole WinMD from the lock's `files` and does not assume a filename at the cache root.

## Retrieval and verification

Standard operations are:

```text
go run ./cmd/winapisource fetch
go run ./cmd/winapisource verify
go run ./cmd/winapisource list
go run ./cmd/winapisource update --dry-run
```

`fetch` creates a temporary staging directory per source. It saves HTTPS artifacts up to a 1 GiB limit and extracts only files listed in the lock from ZIP after verifying the SHA-256 of the entire artifact. The Windows SDK locator checks `WindowsSdkDir`, then the standard Windows Kits directory. The cache is installed by directory rename so a partial cache is not exposed.

`verify` checks the artifact hash, source ID/version/hash in the cache, and presence of required files. It also compares extracted file bytes against the locked archive entry or SDK artifact. Missing or mismatched required sources fail the entire command. An absent optional source is reported as an explicit state and is not generated as though present.

`update --dry-run` only queries the NuGet version index and does not change the lock. SDK updates are not merged into main automatically.

## Provider contract

Every provider implements `Type()` and `Ingest(context, Request)` and returns:

- `Source` constructed from lock information
- An array of `Symbol` values with provenance
- A raw representation for debugging
- `Diagnostic` values for parser warnings/errors

External SDK providers additionally detect installation. Type-library providers return LIBID, coclass, interface, dispinterface, enum, record, alias, method/property/event, and IID/CLSID/DISPID as independent inventory items.

### WinMD provider

The primary parser is currently `github.com/microsoft/go-winmd/winmd`. It opens PE/ECMA-335 metadata, records table counts in the raw dump, and turns `TypeDef`, their `Field` and `MethodDef` ranges, plus `Property`, `Event`, and `GenericParam` into symbols. It reads DLL, entry point, calling convention, and SetLastError from P/Invoke `ImplMap` and assigns provisional vtable indexes to COM/WinRT interface methods. Signature parse failures and table row decode failures are retained as stable `source-parse-error` symbols and diagnostics rather than discarded.

This is not yet a complete semantic projection of WinMD. NativeArrayInfo, MemorySize, SupportedArchitecture, Deprecated, NativeTypedef, RetVal, FreeWith, associated enums, nullability, and contract/threading/marshaling custom attributes are not comprehensively interpreted. A raw table count for `MemberRef`, for example, does not mean its rows are inventoried as symbols.

`generator/internal/ecma335` does not replace the upstream reader. It is a supplementary implementation with stronger bounds checks for compressed integers, blobs, coded indexes, method signatures, and related constructs.

### Clang header provider

The header provider reads Clang's `-Xclang -ast-dump=json` rather than parsing C/C++ declarations with regular expressions. Target triples distinguish 386/amd64/arm64, and defines distinguish desktop/appcontainer/WDK profiles. It currently creates a basic inventory of records, unions, enums, typedefs, functions, constants, and bit-field widths, but classifies most as `unsupported-projection` and emits no callable source until safe C semantic projection is complete.

Macro tokens, complete provenance for preprocessor conditions, pack/layout dumps, SAL, and expression semantics for inline functions are not integrated. Clang process addresses and machine-local paths are removed from debug AST dumps; source locations retain only basename/line/column.

### Type libraries and external SDKs

The current type-library provider does not load native TLBs. It validates deterministic JSON interchange produced by an external reader. Product APIs such as Office are separate `typelib` sources outside OS coverage and are initially inventory-only.

An `ExternalSDKProvider` interface exists for WebView2 and similar SDKs, but no concrete provider is implemented. Sources not installed are not fabricated as OS symbols; they are counted as `external-sdk-not-installed`.

## Raw dumps, IR dumps, and inspect

`winapigen generate --all --dump-raw` saves provider raw dumps under `coverage/raw/<source-id>.json`. The normalized `coverage/inventory.json.gz` is a deterministic gzip IR dump, which `winapigen inspect` searches by qualified name, native name, or stable ID.

```text
go run ./cmd/winapigen inspect --symbol Windows.Win32.Foundation.HANDLE
go run ./cmd/winapigen inspect --native CreateFileW
go run ./cmd/winapigen inspect --id sha256:... --json
```

Raw dumps and IR are debugging artifacts, not a mechanism for vendoring upstream binaries into the repository without regard to redistribution terms.

## Provenance and duplicates

Each symbol carries a source ID, input file, and metadata table/row or header location. An identical native declaration in SDK and WDK is not collapsed into one by removing its source ID. The current implementation detects duplicates with matching namespace, kind, name, signature, and architecture and marks subsequent symbols with `duplicateProjectionOf`. This preserves provenance while detecting duplicates; it is not semantic deduplication that fully resolves layout equivalence or source precedence.

## Manual overrides

Override JSON uses schema version 1 and requires all of the following:

- Override ID
- Source ID
- Inclusive SDK version range
- Stable symbol ID
- Before and after values
- Reason
- Evidence from a header, ABI probe, or upstream issue
- Regression test identifier
- Removal condition after an upstream fix

On application, `before` is compared against the current IR, so an SDK update that changes an assumption fails rather than silently overwriting it. After application, the original projection status/backend is retained, and `manualOverride=true` is counted separately. An override that changes the ABI implementation of an already callable function is rejected so it cannot bypass capability checks.

## Input security

Files whose names are absent from the lock are not extracted. Absolute paths, path traversal, unsafe archive entries, oversized artifacts, and hash mismatches are rejected. Local absolute source-file paths are not embedded in generated headers. SDK/header/NuGet contents with unclear license terms are not automatically added to version control outside the cache.
