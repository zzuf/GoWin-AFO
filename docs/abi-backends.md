# ABI backends

## A backend is evidence of callability

Each function symbol has a backend and a decision reason as well as a status. Do not mechanically convert a signature into a sequence of `uintptr` values and call it "supported." If a backend has not been implemented, built, and ABI-verified, do not emit a raw wrapper; classify the symbol as `unsupported-go-abi` or `unsupported-projection`.

| Backend | Purpose | Current state |
|---|---|---|
| `purego-syscall` | Platform calls with integer, pointer, or raw callback addresses | Implemented for a small Win32/COM/WinRT vertical slice |
| `purego-syscall-float` | Candidate category for float register ABI | Enum value only; call implementation and verification are incomplete |
| `assembly-trampoline` | Architecture-specific trampoline for float/special ABIs | Emitter and verified trampolines are not implemented |
| `cgo-bridge` | Aggregate-by-value, vector, varargs, or compiler-owned ABI | Only the fixture's by-value `FILETIME` bridge is implemented |
| `type-only` | Layout/descriptor only, without a call boundary | Implemented in the vertical slice |
| `unsupported` | Cannot be proven with an available backend | No wrapper is emitted |

The fixture's floating-point example `FixtureFloatABI` is `unsupported-go-abi` because its trampoline has not been verified. Do not count assembly support as implemented. The C bridge is not yet able to generate arbitrary functions; it only verifies the generation boundary for one aggregate-by-value vertical slice.

## Capability decisions

The current independent projection decision generally follows this priority order:

1. Varargs require a fixed-signature C adapter.
2. `vectorcall`, `thiscall`, and `fastcall` require a compiler-owned bridge.
3. An unknown calling convention is unsupported.
4. Unrepresentable arguments/returns, such as scalars wider than 64 bits, are unsupported.
5. Vectors or aggregates passed by value require a C bridge.
6. Float arguments/returns require an architecture-specific trampoline.
7. Other integer/pointer signatures use Pure Go syscalls.

A capability reported by the metadata provider is initially only a candidate decision. Even for official full-source symbols, only the subset with fixed P/Invoke DLL and entry point names, with every parameter/return resolved as a primitive or pointer, and without varargs, float/vector, aggregate-by-value, or wide scalar by-value values is promoted to `generated-purego`. Until availability custom attributes are fully projected, exports are treated as optional. Other symbols are excluded from emission until their calling convention, layout, ownership, and failure rule are established.

## Pure Go call runtime

On Windows, `runtime/winabi` isolates calls to `syscall.SyscallN`. The result retains `R1`, `R2`, and a thread-local last-error snapshot. Non-Windows builds can build the types and validation API, but native calls return `ErrUnsupportedPlatform`.

This backend covers only integer/pointer ABIs. Do not pass the following through it unconditionally:

- Float arguments or returns
- Vector types and `__vectorcall`
- Varargs
- Structs, unions, or arrays passed or returned by value
- 128-bit scalars
- Architecture-specific register classes
- C++ types whose layout is determined by the compiler
- Complex callbacks that require a lifetime registry

It does not use `go:linkname` into the internal Go runtime.

## DLL and export resolution

System DLL names are fixed basenames from metadata; slashes, colons, `..`, and non-ASCII path characters are rejected. The loader uses a bootstrapped `LoadLibraryExW` with `LOAD_LIBRARY_SEARCH_SYSTEM32` to avoid search-order hijacking through the current directory or general `PATH`.

DLLs and procedures are resolved lazily with `sync.Once`. Package import does not load all DLLs at once. `Is<Name>Available() bool` lets callers check whether an optional export can be resolved. Separate policies for app-local DLLs and API sets are incomplete; do not reuse the current System32 loader for arbitrary external SDK paths.

## Raw error semantics

Raw wrappers preserve Windows return values. Only APIs with `SetLastError` metadata return a last-error snapshot, and a nonzero snapshot alone does not mean failure. First evaluate the API-specific `FailureRule`, such as `return == FALSE`, `return == NULL`, or `return == INVALID_HANDLE_VALUE`.

`winabi.ErrorIfFalse`, `ErrorIfZero`, and `ErrorIfInvalidHandle` interpret last error only after the failure sentinel has been confirmed. If last error is `ERROR_SUCCESS` on failure, they return `ErrNativeFailureWithoutLastError`; they do not turn success codes or stale thread-local state into misleading errors.

`HRESULT` and `NTSTATUS` retain their signed 32-bit raw values, severity, facility, and code. Do not treat `S_FALSE` as an error or silently convert NTSTATUS to a Win32 error. The checked/ergonomic layer converts to Go `error` only for APIs with metadata or an explicit rule.

## C bridge

The public surface of a C bridge is a simple C ABI; C++ classes and templates are not exposed directly to Go. The compiler handles aggregate register/stack classification. The current emitter generates an example that flattens the fixture's by-value `FILETIME` into a static C function.

Bridge files have a `windows && cgo` build constraint. Files with `!windows || !cgo` only keep the package buildable; they do not pretend to provide bridge-only APIs. A future full bridge emitter will need to pair target compiler/SDK versions and hashes with an ABI oracle.

## Assembly trampolines

If assembly is used, register, stack, unwind, return-value, and callback transitions must be verified separately for 386, amd64, and arm64. Do not treat ARM64EC as an alias for Go `arm64`. There are currently no verified trampolines, so float APIs are not counted as `generated-assembly`.

## Callbacks

The current raw callback type is a named `uintptr` representing a native function address. It is not a mechanism for converting arbitrary Go closures into Windows callbacks. Implementing a Go callback object requires rules for a lifetime registry, preventing calls after release, preventing panics from crossing the ABI boundary, concurrency, thread/apartment affinity, and retained native pointers. Without those rules, classify a signature as a raw address/type-only or unsupported.

## Architecture considerations

The same source symbol may have separate ABI decisions for 386, amd64, and arm64. For example, 8-byte alignment that Go/386 cannot represent uses aligned scratch storage or a bridge. A successful cross-compile does not verify the register ABI at runtime, so the coverage report separates compile-verified from runtime-verified for architectures without a hardware runner.
