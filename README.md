# go-windows-api

`go-windows-api` is a deterministic foundation for generating Go projections
from Microsoft Windows metadata instead of maintaining a large hand-written API
surface. It pins official inputs, inventories every discovered symbol with a
stable ID, normalizes a language-neutral ABI IR, emits namespace-sized packages,
records an explicit status/reason for every symbol, and verifies a representative
vertical slice against Windows headers and runtime smoke tests.

This repository is an implemented foundation and vertical slice, **not yet a
complete callable projection of every Windows API**. The current full-source pass
inventories 522,071 official symbols from pinned Win32, WDK, and WinRT metadata
(the 23-symbol project fixture is explicitly excluded from this denominator). Most are
conservatively classified `unsupported-projection` or `source-parse-error` until
their complete type/attribute projection is proven. It does not label those APIs
as generated or callable.

## Scope and the meaning of 100%

The provider model covers Windows SDK Win32 metadata, WDK metadata, SDK contract
WinMD, Windows App SDK packages, Clang AST JSON for header-only declarations,
typed manual overrides, and a separate type-library/external-SDK interface.
Office and other product TLBs have independent denominators and never inflate OS
coverage. Header and Windows App SDK ingestion are implemented as provider
boundaries but are not yet full production projections.

The phrase `projection accounting coverage = 100%` means only:

- every ingested symbol has a deterministic ID;
- every symbol has a valid status and reason; and
- `unclassified = 0`.

It does **not** mean 100% Pure Go callability, source generation, ABI verification,
or runtime execution. Those metrics are reported separately. The latest measured
full-source inventory is:

| Measure | Symbols |
|---|---:|
| Total official symbols inventoried | 522,071 |
| Generated Pure Go | 444 |
| Generated assembly | 0 |
| Generated C bridge | 0 |
| Generated type-only | 0 |
| Generated manual override | 0 |
| Kernel-mode-only | 1,714 |
| Source parse error | 80 |
| Unsupported projection | 519,361 |
| Unsupported Go ABI | 471 |
| Missing upstream metadata/provider locks | 1 |
| Unclassified | 0 |

Pure Go callable coverage is **444 / 522,071 (0.09%, rounded)**. Official-input
ABI and runtime verified coverage are currently 0%; the independently verified
vertical slice is reported separately and does not inflate those figures.

See [coverage/latest.md](coverage/latest.md) for the current manifest-specific
numbers and [docs/coverage.md](docs/coverage.md) for denominator definitions.

## Inputs and architectures

- `Microsoft.Windows.SDK.Win32Metadata` `71.0.26-preview`
- `Microsoft.Windows.WDK.Win32Metadata` `0.13.25-experimental`
- Windows SDK UnionMetadata `10.0.26100.0`, fetched from the pinned
  `Microsoft.Windows.SDK.CPP` NuGet `10.0.26100.7705` (not the runner's installed SDK)
- Windows App SDK meta package `2.5.1` (the API-bearing transitive graph is
  classified `missing-upstream-metadata` until every package receives its own lock/hash)

The target Go architectures are `windows/386`, `windows/amd64`, and
`windows/arm64`. AMD64 and 386 vertical-slice smoke tests execute on Windows;
ARM64 Go code and the native probe are cross-compiled but are not reported as executed.
Official per-symbol `SupportedArchitecture` attributes are not yet projected, so
the current official report records architecture as `neutral` rather than claiming
that every symbol is present on every target.
ARM64EC is a separate unsupported target and is never conflated with Go ARM64.

## Minimal raw example

```go
package main

import (
	"fmt"

	threading "github.com/zzuf/GoWin-AFO/bindings/generated/windows_win32_system_threading"
)

func main() {
	if !threading.IsGetCurrentProcessIdAvailable() {
		return
	}
	fmt.Println(threading.GetCurrentProcessId())
}
```

The module path is `github.com/zzuf/GoWin-AFO`, matching the confirmed GitHub
repository. `go.mod` remains the authoritative source for generated imports.
Publishing this source repository does not imply a stable API or full Windows
API coverage.

The raw layer preserves native names, widths, allocation, ownership, failure
rules, and optional last-error values. It does not turn a nonzero last-error value
into failure without first checking the function-specific result. A future
ergonomic layer may add Go strings, slices, errors, iterators, and resource
wrappers, but it is intentionally outside the completeness denominator.

## Runtime and safety

- System DLL loading is lazy and restricted to approved bare System32 DLL names;
  caller-controlled paths are rejected.
- UTF-16 helpers reject embedded NUL rather than silently truncating.
- The official generated subset currently excludes every pointer parameter or
  result until retention, alignment, SAL, and ownership attributes are projected.
  The verified vertical slice demonstrates explicit lifetime rules and
  `runtime.KeepAlive`; retained pointers and callbacks require a registry or bridge.
- Handles and COM references use explicit release; finalizers are not the sole
  ownership mechanism.
- COM apartment helpers lock the goroutine to its OS thread for apartment scope.
- WinRT includes HSTRING, initialization, `IInspectable`, activation factories,
  and a representative `Windows.Foundation.Uri` activation test. Parameterized
  IID computation is not guessed; unsupported cases return an explicit error.
- WDK kernel exports are never emitted as ordinary user-process calls. Useful
  records may be type-only, and the standard Go runtime is not presented as a
  kernel-driver runtime.
- `LARGE_INTEGER` demonstrates an important boundary: native x86 alignment is 8
  while Go/386 cannot express it. The 386 projection is storage-only and the QPC
  wrapper uses aligned scratch storage; it is not passed by value as a false ABI
  match.

## Reproduce

Use Go 1.26 or later; `go.mod` and `go.work` select the reproducible Go 1.26.8
toolchain. From a clean checkout on Windows (metadata fetch uses pinned NuGets;
compiling fresh native ABI probes additionally needs MSVC/clang-cl and SDK
`10.0.26100.0` headers):

```text
go run ./cmd/winapisource fetch
go run ./cmd/winapisource verify
go run ./cmd/winapigen generate --all
go run ./cmd/winapiverify abi --all
go run ./cmd/winapicoverage check --fail-unclassified --fail-regression
go test ./...
```

Run generation twice; the second run must have no repository diff. Inputs are
downloaded into ignored `sources/cache` only after SHA-256 verification. SDK,
WDK, NuGet, and header artifacts are not vendored. Generated output contains
source/version/generator/manifest hashes, never timestamps or local paths.

Generation also reconstructs the 13 reviewed runtime-smoke binding files under
`bindings/win32`, `bindings/wdk`, and `bindings/winrt` from version-reviewed
templates. Those fixtures are kept separate from official generation coverage.

For inspection:

```text
go run ./cmd/winapigen inspect --symbol Windows.Win32.Foundation.HANDLE
go run ./cmd/winapigen inspect --native CreateFileW
go run ./cmd/winapigen inspect --id sha256:... --json
go run ./cmd/winapicoverage report
```

Design details are in [docs/architecture.md](docs/architecture.md), ABI backend
rules in [docs/abi-backends.md](docs/abi-backends.md), verification in
[docs/verification.md](docs/verification.md), and current gaps in
[docs/known-limitations.md](docs/known-limitations.md).

See [docs/publishing.md](docs/publishing.md) for GitHub push/public-module setup
and [docs/updates/2026-09-23.md](docs/updates/2026-09-23.md) for the latest input,
parser, and verification changes. Push does not itself certify full API support.
