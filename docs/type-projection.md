# Go type projection

## Principles

Type projection chooses a form with the same representation and meaning under the target Windows ABI, rather than merely one that compiles in Go. Do not emit a callable projection when layout, alignment, ownership, or calling convention cannot be proven. The raw layer preserves native names and widths; convenient string/slice/error/resource wrappers belong in a separate layer.

The current generator is still validating complex type rules with fixtures and has not projected every type in official WinMD. A provisional `GoType` produced during full-source ingestion is a candidate, not an ABI-verified public named type. A limited subset of official P/Invoke functions whose primitive/pointer projections are all resolved is emitted as raw functions. Do not substitute `uintptr` for unresolved types to promote them to callable.

## Basic LLP64 types

Do not confuse Windows `long` with pointer width.

| Native | Raw Go representation |
|---|---|
| `CHAR` | `int8` |
| `BYTE` | `uint8` |
| `SHORT` | `int16` |
| `USHORT` | `uint16` |
| `INT` | `int32` |
| `UINT` | `uint32` |
| `LONG` | `int32` |
| `ULONG` | `uint32` |
| `LONGLONG` | `int64` |
| `ULONGLONG` | `uint64` |
| `WCHAR` | `uint16` |
| `BOOL` | `int32` |
| `BOOLEAN` | `uint8` |
| `HRESULT` | Named `int32` |
| `NTSTATUS` | Named `int32` |
| `SIZE_T`, `UINT_PTR`, `ULONG_PTR` | Named types with `uintptr` width |
| `SSIZE_T`, `INT_PTR`, `LONG_PTR` | `int32` on 386; `int64` on amd64/arm64 |
| `HANDLE` | Pointer-sized opaque named value |

`runtime/winabi.SignedPointer` selects the signed pointer width with build constraints. Do not use Go `int` in place of C `int`, `LONG`, or `DWORD`. If a metadata signature lacks native-width information, exclude it from emission until the architecture and typedef attributes are established.

## Typedefs, aliases, and handles

Preserve `NativeTypedef` or a distinct native typedef as a Go named type. Use an alias only when types are proven truly interchangeable. `HANDLE`, `HWND`, `HKEY`, and similar types remain distinct named types even though they occupy the same machine word, reducing accidental assignment across APIs.

A null handle and `INVALID_HANDLE_VALUE` are different sentinels. An IR type can carry `CloseFunction` and `InvalidValue`; a symbol can carry ownership/allocator/deallocator information. Callers close explicitly instead of relying on an automatic finalizer. Establish borrowed, owned-on-success, retained, or other ownership per function rather than inferring it from the type name alone.

## Pointers and parameter semantics

The IR tracks pointer depth, optionality, element type, in/out/in-out direction, nullability, byte count, element count, retval, and `FreeWith` separately. It does not treat `T*`, optional `T*`, and `T**` as the same `uintptr`.

However, the WinMD provider does not yet fully reconstruct these custom attributes. In those cases, keep the symbol at `unsupported-projection` or type-only instead of generating an unsafe pointer wrapper.

If native code retains a Go pointer after the call, the Go heap pointer must not be left with it. This requires dedicated native allocation, a pinning policy, a callback registry, or a C bridge. Even pointers borrowed only during a call use `runtime.KeepAlive` inside the generated wrapper.

## Structs and architecture-specific layouts

Ordinary structs preserve field order and carry architecture-specific size, alignment, offsets, packing, and tail padding in the IR layout. Use a direct Go struct layout only when Go can represent it and it matches an ABI oracle.

Types that vary with pointer size can be split into files such as `ztypes_windows_386.go`, `ztypes_windows_amd64.go`, and `ztypes_windows_arm64.go`. The current vertical slice verifies `ARCH_WORD` and pointer-sized primitives on all three architectures.

If Go cannot represent the exact layout of a packed struct, over-aligned member, or anonymous record, use fixed byte storage with accessors, an architecture-specific safe representation, or a C bridge. Do not emit an incorrect ordinary struct.

## Unions

A union preserves maximum size and alignment and generates storage and typed accessors that do not automatically track the active member. The accessor's caller manages which member is valid and its lifetime.

The vertical-slice `LARGE_INTEGER` uses 8-byte-aligned storage and overlapping accessors on amd64/arm64. Because Go/386 cannot represent MSVC's 8-byte alignment as a type, it provides only size-correct storage and makes no claim of direct ABI safety. The `QueryPerformanceCounter` wrapper creates an 8-byte-aligned scratch buffer and copies the result through an accessor.

## Bit fields

The IR preserves storage type, bit offset, bit width, and signedness. A generated type has a raw storage word and getter/setter methods; the setter changes only the target mask. A signed field getter explicitly sign-extends from the field width.

The Clang AST provider currently inventories basic bit widths but has not established complete target-specific allocation units/offsets. Layout probes are needed before generating header-derived bit fields from the full source.

## Arrays

A fixed array preserves length and element type; emission stops if a value exceeds the range of a Go type. Do not disguise a flexible array member as `[1]T`. Separate the fixed header from a caller-owned buffer view and check the following when creating the view:

- The base pointer is not nil.
- The allocation is at least the header size.
- The element count does not exceed the remaining byte count.
- The count fits in Go `int` and the address space.

The view cannot outlive the original allocation and cannot be used when Windows retains it after the call.

## Strings

Raw-layer `PCWSTR`, `PWSTR`, `PCSTR`, and `PSTR` are borrowed types close to native pointers. A Win32 NUL-terminated UTF-16 helper detects and rejects NULs in a Go string rather than silently truncating it.

`BSTR` and `HSTRING` are length-prefixed and can preserve embedded NULs. Runtime wrappers pair native allocators and deallocators and require an explicit `Close`. An activation class name is a separate protocol boundary and rejects embedded NULs.

## Enums, flags, and constants

Preserve the underlying native type and signedness, and distinguish enums from flags enums. Do not discard multiple names with the same value. Separate architecture-dependent values with build constraints. Do not silently truncate values that do not fit the target Go type.

The current full WinMD provider only inventories raw constant blobs from literal fields; it has not finished converting every constant encoding and associated enum into a Go expression. Do not treat all official constants as generated.

## A/W APIs

Emit `FunctionA` and `FunctionW` separately under their native entry point names. Do not ambiguously reproduce the C preprocessor's `Function` alias in the raw layer. If a convenience alias defaulting to Unicode is added, identify it as a project-specific ergonomic API.

## ABI verification boundary

Go `unsafe.Sizeof` / `Alignof` / `Offsetof` tests are necessary but do not by themselves prove the header ABI. Set `ABIVerified` only after comparison with sizes, alignments, offsets, constants, GUIDs, vtable slots, and function types from a C/C++ oracle. There are currently Go layout tests and Windows smoke tests for fixtures, but no oracle results for the entire Windows SDK; do not claim that all types are ABI-verified.
