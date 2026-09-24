# WDK

## Policy

WDK metadata is treated as a separate source from Windows SDK metadata. The currently pinned input is `Microsoft.Windows.WDK.Win32Metadata` `0.13.25-experimental`, and the target Windows SDK is `10.0.26100.0`.

This project does not claim that the standard Go runtime can build or run Windows kernel drivers. Even when kernel types or constants can be represented in Go, it does not generate wrappers for calling kernel exports such as `ntoskrnl.exe`, `.sys`, HAL, or NDIS from an ordinary process.

## Classification dimensions

WDK symbols ultimately need to distinguish:

- Callable from an ordinary user-mode process
- Available in a UMDF runtime/context
- Type/constant only, but useful to user-mode code
- Kernel-mode-only
- Not executable with the standard Go runtime
- Incomplete upstream metadata
- Requires supplementation from a header/ABI probe

The current full-source provider can make only limited automatic decisions. When it recognizes a P/Invoke target as `ntoskrnl`, HAL, NDIS, or `.sys`, it classifies the symbol as `kernel-mode-only`. Other WDK types/methods generally remain `unsupported-projection` or type-only inventory until callability and layout are proven. Precise UMDF and user-mode classification is incomplete.

## Current vertical slice

`bindings/wdk/nt` contains these representative examples:

| Symbol | Current treatment | Reason |
|---|---|---|
| `UNICODE_STRING` | `generated-type-only` | Counted UTF-16 layout; buffer ownership is external |
| `OBJECT_ATTRIBUTES` | `generated-type-only` | Layout also useful to user-mode NT consumers; no kernel callability granted |
| `DRIVER_OBJECT` / `PDRIVER_OBJECT` | Opaque pointer / type-only | Does not turn a kernel object into a user-mode-allocatable struct |
| `IoCreateDevice` | `kernel-mode-only` | Requires a kernel driver execution environment |
| `IoDeleteDevice` | `kernel-mode-only` | Requires a kernel driver execution environment |

There are no call wrappers for `kernel-mode-only` functions. Being able to import a type-only symbol does not mean that its corresponding kernel routine can be called.

## SDK/WDK overlap

The source ID is part of a stable ID, so SDK and WDK provenance is preserved. The current generator detects symbols with matching namespace, kind, native name, canonical signature, and architecture, and adds a `duplicateProjectionOf` annotation to the later origin.

This is a foundation for avoiding simple duplicate generation, not a complete semantic merge. Selection of a canonical projection when typedef chains, custom attributes, layouts, or availability differ, and sharing through a foundation package, are not yet fully implemented. Source-specific coverage tracks both origins.

## Layout and architecture

WDK types also follow Windows LLP64 and architecture-specific layouts for 386/amd64/arm64. Pointer-sized fields, anonymous unions, bit fields, packing, and flexible arrays require proof from a header/ABI oracle before becoming direct Go structs. In particular, do not guess the internal layout of kernel objects across OS versions. Keep unpublished bodies, or bodies without meaning in user mode, opaque.

The Clang profile for WDK headers can set `_KERNEL_MODE=1` and a Windows target triple, but integration of WDK include paths, an NTDDI/WINVER matrix, SAL, and all layout probes into the full generation pipeline is incomplete.

## Function callability

Originating in the WDK does not by itself make a symbol kernel-only, nor does a DLL name alone establish user-mode safety. Promotion to callable requires checking at least:

- The export exists in a user-mode DLL.
- The target profile and minimum OS version.
- The calling convention and architecture ABI.
- Parameters are valid in user-mode address space.
- Handle/object ownership and IRQL/context constraints.
- Whether administrator access, driver installation, a service, or a system configuration change is required.
- The metadata signature matches the WDK header.

Kernel exports are not candidates for `purego-syscall`. UMDF APIs also require framework initialization and lifetime management, so a simple DLL call wrapper does not constitute support.

## NTSTATUS and errors

Raw results of WDK/NT APIs are preserved as `NTSTATUS int32`. They use the same signed comparison as `NT_SUCCESS(status)` and expose severity/facility/code. Even if a convenience helper to convert to a Win32 error is added later, it must preserve the original NTSTATUS.

## Testing and safety

Ordinary CI does not install drivers, create services, change the registry, or make kernel calls. Verification covers Go and C/C++ cross-compilation of type layouts and only non-destructive user-mode APIs. Tests check the presence of inventory/status/reason for kernel-only symbols; avoiding execution is itself a safety requirement.

Do not generate dereference helpers that assume a pointer refers to a kernel address, undocumented struct layouts, or guessed NT signatures. Leave incomplete metadata as `missing-upstream-metadata`, header parse failures as `source-parse-error`, and unprovable Go ABIs as `unsupported-go-abi`.

## Outstanding work

- Complete WDK inventory beyond all TypeDef/Field/MethodDef entries
- User-mode/UMDF/kernel-mode classification using custom attributes and header conditions
- Semantic deduplication of SDK/WDK types
- ABI oracle verification of all public WDK types
- A general-purpose emitter for user-mode WDK APIs
- A 386/amd64/arm64 and WINVER/NTDDI profile matrix for WDK headers

Until these are complete, do not present the entire WDK as supported or claim 100% WDK coverage.
