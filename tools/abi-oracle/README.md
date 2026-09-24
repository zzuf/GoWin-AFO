# GoWin-AFO ABI oracle

`abi-oracle` turns a reviewed JSON manifest into deterministic C++ source. The
source includes Microsoft SDK/WDK headers, proves function pointer types with
`static_assert`, and emits layout and value facts as JSON. A compiled Go package
is never treated as ABI evidence by itself.

The checked-in fixture covers a deliberately small Windows SDK 10.0.26100.0
slice: five records/unions, two constants, `IID_IUnknown`, one bit field, two
function signatures, three `IUnknown` vtable slots, and two preprocessor/target
conditions. It is not a claim that the complete SDK has been verified.

## Commands

```text
go run ./tools/abi-oracle validate --manifest tools/abi-oracle/testdata/probe-manifest.json
go run ./tools/abi-oracle generate --manifest tools/abi-oracle/testdata/probe-manifest.json --architecture amd64 --out probe.cpp
go run ./tools/abi-oracle validate --result oracle.json
go run ./tools/abi-oracle compare --expected tools/abi-oracle/testdata/windows-sdk-10.0.26100-amd64.json --actual oracle.json --out abi-diff.json
go run ./cmd/winapiverify abi --all
```

Compile `probe.cpp` with MSVC or clang-cl against the exact SDK selected by the
source lock. `/W4 /WX /permissive-` is used in CI. An x64 host can execute x86
and x64 probes. ARM64 is cross-compiled to prove declarations, target guards,
and `static_assert`s; a JSON result must not be labeled runtime-verified until
the ARM64 executable has run on an ARM64 host.

Probe manifests contain reviewed native expressions and are executable C++
inputs. Never accept an untrusted manifest from a pull request artifact or API.
The generator validates identifiers and include paths, but it intentionally does
not try to sanitize C++ expressions.

Schemas are in `schema/manifest.schema.json` and `schema/result.schema.json`.
Potentially wide C values are serialized as decimal strings. Bit masks are byte
hex in Windows memory order, accompanied by a bit offset and population width.
The comparison ignores compiler identity but compares SDK, profile,
architecture, manifest hash, and every ABI fact.

`winapiverify` is the independent consumer: it validates source/slice
provenance and maps the subset of oracle facts with matching normalized IR into
`coverage/abi-latest.json`. Facts with no matching IR remain explicitly
unmatched; their presence is not turned into a blanket ABI-verified claim.
Its `--all` flag means every checked-in vertical-slice evidence target, not all
Windows SDK namespaces. The report compares normalized IR expectations with the
C/C++ oracle; it is not a measurement of every generated Go `unsafe` layout or
every native call boundary.
