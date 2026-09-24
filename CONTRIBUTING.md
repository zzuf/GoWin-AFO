# Contributing to GoWin-AFO

Changes to this repository must prioritize ABI correctness, reproducibility, and accounting for every symbol over the volume of generated output. Start by reading `AGENTS.md`, `docs/architecture.md`, and the documentation for the area you are changing.

## Development environment

The basic generator and tests run on Linux, macOS, and Windows with Go 1.26 or later. `go.mod` / `go.work` pin Go 1.26.8 for reproducibility. Fetching official sources requires HTTPS access. WinRT sources and ABI probes require the target Windows SDK; WDK header probes require the target WDK; native probes require MSVC or clang-cl.

```text
go run ./cmd/winapisource fetch
go run ./cmd/winapisource verify
go test ./...
```

Do not vendor SDKs, NuGet packages, or headers into the repository without permission. See `docs/licensing.md` for licensing and redistribution requirements.

## Where to make changes

Do not edit files whose generated header says `Code generated ... DO NOT EDIT.` directly. Make corrections in one of the following areas:

- Parser/provider: incomplete reading of official inputs.
- Normalized IR/projection: types, ABI, ownership, availability, and status classification.
- Emitter: the structure of Go, C, or assembly source.
- Typed override: an upstream metadata defect supported by evidence.
- Runtime: shared ABI, COM, WinRT, and ergonomic helpers.
- Oracle/test: ABI evidence derived from headers.

Keep namespace packages small and do not duplicate foundation types. Keep the raw ABI and ergonomic layers separate. Do not spread `unsafe` into ordinary application packages.

## ABI changes

Windows uses LLP64. Do not replace C `int`/`LONG`/`DWORD` with Go `int`; only pointer-sized types should vary by architecture. Do not guess calling conventions, float/vector handling, aggregates passed by value, varargs, or callbacks. If a function cannot be proven correct with an available backend, assign a status with a reason instead of generating a callable wrapper.

When changing a type layout or function signature, add a `tools/abi-oracle` manifest and a regression test. Native expressions in the manifest become executable C++ source: use only reviewed, fixed text, and never insert external input.

## Override checklist

Meeting the schema is not sufficient for a new override. Every override must include:

- A unique override ID, source ID, and symbol ID.
- The affected SDK version range.
- A `before` value that causes failure if it does not match the current value.
- A minimal `after` value.
- An explanation of why the metadata is incorrect and how it affects the Go ABI.
- Evidence from an official header, ABI probe, or upstream issue.
- An automated regression test.
- A specific removal condition once the upstream issue is fixed.

Overrides with no clear lifetime or evidence, wildcard symbol IDs, and bulk overwrites of many symbols are not accepted.

## Tests and generation

Run at least the following checks, as appropriate for the scope of the change:

```text
gofmt -w <changed-go-files>
go test ./...
go vet ./...
go run ./cmd/winapigen generate --all
go run ./cmd/winapicoverage check --fail-unclassified --fail-regression
go run ./cmd/winapigen generate --all
```

The second generation must introduce no diff. Complete and validate the generated tree in a temporary sibling directory before replacing the destination. A failure must not damage the existing valid tree.

Cross-compile the Windows targets `386`, `amd64`, and `arm64`. For bridge changes, separately verify that Pure Go packages build with `CGO_ENABLED=0` and the relevant bridge builds with `CGO_ENABLED=1`. Windows runtime tests must use only nondestructive APIs.

## Pull request

Describe the source/version/hash, affected symbols/statuses/backends, generated packages, coverage before and after the change, ABI evidence, targets/tests executed, and known limitations. Keep large generated diffs distinct from parser/IR/emitter changes in the submission.

Follow `docs/sdk-updates.md` for SDK updates. Include the lock, generated diff, coverage diff, ABI diff, breaking API review, and license/NOTICE changes together. Workflows must not automatically merge into main.
