# COM

## Scope

The current COM implementation is a minimal consumer-side ABI runtime, not a projection of all COM.

| Feature | Current state |
|---|---|
| GUID / IID | `winabi.GUID` and a canonical parser implemented |
| IUnknown | Vtable slots 0–2, `QueryInterface`, `AddRef`, and `Release` implemented |
| Interface inheritance | Basic form with an embedded vtable prefix implemented |
| Apartment | `CoInitializeEx` / `CoUninitialize` and an OS-thread-pinned owner implemented |
| BSTR | Owned wrapper for `SysAllocStringLen` / `SysStringLen` / `SysFreeString` implemented |
| COM task memory | Owned wrapper for `CoTaskMemAlloc` / `CoTaskMemFree` implemented |
| Generated interface | Only the fixture's `IExample` vtable layout example |
| SAFEARRAY / VARIANT / PROPVARIANT | Not implemented |
| Coclass activation helper | General-purpose helper not implemented |
| Automation / IDispatch | Not implemented |
| Exposing a Go object as a COM server/callback | Not implemented |

This does not claim that every COM interface, inheritance relationship, method parameter, or IID in Windows SDK metadata has been generated. The full-source provider can inventory interfaces, but complete reconstruction of custom attributes and layouts, verification of every vtable against a C/C++ oracle, and a general-purpose method emitter are incomplete.

## Objects and vtables

A COM interface pointer is treated as a native object with a vtable pointer at its start. `IUnknownVTable` fixes this order:

| Slot | Method |
|---:|---|
| 0 | `QueryInterface` |
| 1 | `AddRef` |
| 2 | `Release` |

A derived interface uses the base vtable as a prefix and appends method slots in metadata order. The generator records slot indexes in the IR, but they must be checked against a header interface declaration or C++ oracle before the interface is officially classified as generated-callable.

Method calls use the integer/pointer ABI of `winabi.CallAddress`. Methods that this boundary cannot represent, including aggregate-by-value, float/vector, or special returns, are classified as requiring a C bridge/assembly or as unsupported.

## Reference ownership

An owned COM reference must be explicitly `Release`d. A finalizer is not the sole means of release. Callers must distinguish:

- A new owned reference returned by `QueryInterface` or a similar method
- A reference explicitly added with `AddRef`
- A reference temporarily borrowed as a parameter
- A callback/interface pointer retained by an API

Do not reuse an object pointer after `Release`, especially if its return value is zero. Do not copy a wrapper so that multiple parties appear to own it. The current low-level `IUnknown` is not a smart pointer and does not automatically guarantee unique ownership of a reference.

## Apartments and thread affinity

COM initialization is per OS thread. `EnterApartment` pins the current goroutine with `runtime.LockOSThread` and returns an owner only if `CoInitializeEx` succeeds. `Apartment.Close` calls `CoUninitialize` on the same goroutine and releases the thread lock.

Both `S_OK` and `S_FALSE` are preserved as success. If initialization fails because a different apartment model is already set, it returns no owner and releases the thread lock. Do not copy an `Apartment` or move it to another goroutine.

The basic pattern is:

```go
apartment, status := com.EnterApartment(com.COINIT_MULTITHREADED)
if status.Failed() {
    return status
}
defer apartment.Close()
```

## BSTR and task allocation

A `BSTR` has an explicit length and therefore preserves embedded NULs. `NewBSTR` checks that the number of UTF-16 code units fits in `UINT32` and keeps the source slice alive with `runtime.KeepAlive` until the native call finishes. `String` checks the length from `SysStringLen` against the address space and Go `int` before copying. `Close` clears the pointer and makes a second call a no-op.

`TaskMemory` holds a size and native pointer and calls `CoTaskMemFree` only once in `Close`. Pointers returned through COM out parameters can be released with `FreeTaskMemory`. Neither has a finalizer.

Ownership and clear routines for SAFEARRAY, VARIANT, and PROPVARIANT are not implemented, so APIs involving them are not classified as ergonomically usable.

## HRESULT

Raw COM methods return `HRESULT` as a signed 32-bit value. `Failed()` / `Err()` treat it as an error only when the severity bit indicates failure. They preserve the facility/code and `S_FALSE` on success. Do not use out parameters until HRESULT success has been checked.

## Callbacks and COM servers

There is currently no registry for exposing Go methods as COM objects. A future implementation needs at least:

- A mapping between native reference counts and Go owners
- Callback target lifetimes and release at shutdown
- Rejection of callbacks after release
- Recovery that prevents panics from crossing the ABI boundary
- Concurrent calls and reentrancy
- Apartment/thread affinity
- Native allocation that satisfies Go pointer retention rules
- Backend decisions for special method signatures

Generating only an equivalent of `syscall.NewCallback` without these guarantees does not constitute support.

## Verification and current limitations

Unit tests check the pointer-size-dependent offsets/sizes of the IUnknown vtable and the IID. Windows smoke tests non-destructively check apartments, an embedded-NUL BSTR round trip, and CoTaskMem allocation/free. These tests do not verify method order, marshaling, or threading models for arbitrary SDK interfaces. ABI-verified coverage for all COM APIs has not been achieved.
