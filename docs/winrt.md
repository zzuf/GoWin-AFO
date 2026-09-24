# WinRT

## Scope

The current `runtime/winrt` implements the foundational ABI for WinRT consumers:

- `RoInitialize` / `RoUninitialize`
- Creation, reading, and deletion of owned `HSTRING` values
- `IInspectable` with an `IUnknown` prefix
- `GetIids`, `GetRuntimeClassName`, and `GetTrustLevel`
- `IActivationFactory` and `ActivateInstance`
- `RoGetActivationFactory` / `RoActivateInstance`
- A small generated example for the `Windows.Foundation.Uri` activation factory

These form a vertical slice of Windows SDK `10.0.26100.0`, not a generated projection of the entire Windows contract WinMD.

## Apartments

WinRT initialization also belongs to an OS thread. `EnterApartment` pins the goroutine to an OS thread and returns an owner after `RoInitialize` succeeds. `Close` calls `RoUninitialize` on the same goroutine and releases the thread lock. STA/MTA is selected explicitly with `RO_INIT_SINGLETHREADED` or `RO_INIT_MULTITHREADED`.

The full-source provider does not yet fully ingest runtime class metadata for threading models and marshaling behavior. Do not assume generated wrappers can move freely between goroutines.

## HSTRING

`HString` owns a handle created by `WindowsCreateString` and calls `WindowsDeleteString` exactly once in `Close`. It checks the length returned by `WindowsGetStringRawBuffer` before copying to a Go string. HSTRING itself is length-prefixed and preserves embedded NULs.

A runtime class name identifies an activation protocol, so empty strings and embedded NULs are rejected. This avoids ambiguity at the string boundary. Do not use an HSTRING handle after `Close`; `runtime.KeepAlive` preserves the wrapper lifetime during native calls.

## IInspectable and activation

`IInspectableVTable` has these slots after the three IUnknown slots:

| Slot | Method |
|---:|---|
| 3 | `GetIids` |
| 4 | `GetRuntimeClassName` |
| 5 | `GetTrustLevel` |

The result of `GetIids` is copied into a Go slice before the native array is freed with `CoTaskMemFree`. A runtime class name is returned as an owned HSTRING. Successful activation factory/instance results are owned COM references that the caller must `Release`.

```go
apartment, status := winrt.EnterApartment(winrt.RO_INIT_MULTITHREADED)
if status.Failed() {
    return status
}
defer apartment.Close()

factory, status := winrt.GetActivationFactory("Windows.Foundation.Uri")
if status.Failed() {
    return status
}
defer factory.Release()
```

## Metadata projection

The WinRT provider treats `c/UnionMetadata/10.0.26100.0/Windows.winmd` from the official `Microsoft.Windows.SDK.CPP/10.0.26100.7705` as a required input with a pinned version and archive hash. It does not depend on whether the runner has an installed SDK or on its servicing revision. The current WinMD reader can inventory the basic kinds of WinRT interfaces, runtime classes, delegates, generic arity, properties, and events, but does not fully project the following semantic information:

- Default interface and activation factory attributes
- Contract versions and deprecation
- Threading models and marshaling behavior
- Method parameter ownership/nullability
- Event add/remove pairing and `EventRegistrationToken`
- Async interfaces and progress/completion handlers
- Closed generic instantiations and signature grammar
- Ergonomic projection of collection interfaces

Consequently, full-source WinRT symbols currently remain `unsupported-projection` / type-only inventory by default and are not sent to the emitter as callable.

## Generics and parameterized IIDs

The IID of a WinRT generic interface is not simply the GUID of the generic type; it requires a parameterized IID calculation based on the canonical WinRT signature and the specified namespace. The current `ParameterizedIID` intentionally returns `ErrParameterizedIIDUnsupported` and a zero GUID. Do not call `QueryInterface` with a guessed IID.

Generation of closed generic metadata and runtime calculation for arbitrary type arguments are not implemented. Until this is verified, generic collections, async operations, and delegates are not counted as supported.

## Delegates, events, and async

Delegate objects, event registration/removal, `IAsyncAction`, `IAsyncOperation<T>`, and progress/completion handlers are not implemented. They involve COM references, callback lifetimes, apartment transitions, completion races, panic containment, and parameterized IIDs. Do not bypass the ABI with only a function pointer or channel wrapper.

## HRESULT and ownership

WinRT raw calls preserve HRESULT and use its severity bit to determine failure. They also retain success statuses. Treat out pointers/HSTRINGs as owned only after success; do not casually use values that may be non-nil/nonzero on failure. Explicit `Release` is the baseline for IInspectable/IActivationFactory, and explicit `Close` for HString, rather than finalizers.

## Verification

Unit tests check IInspectable vtable offsets, explicit failure for parameterized IIDs, and NUL rejection in activation names. Windows smoke tests check an embedded-NUL HSTRING round trip and acquisition/Release of the `Windows.Foundation.Uri` activation factory. They do not verify execution across every WinRT runtime class, contract, or architecture.
