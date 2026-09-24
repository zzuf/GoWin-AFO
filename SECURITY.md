# GoWin-AFO security policy

## Reporting vulnerabilities

Treat DLL search hijacking, ABI corruption in generated signatures/layouts, use-after-free/double-free, callback lifetime issues, COM reference-counting issues, archive path traversal, source hash bypasses, and code injection into the generator as security issues.

Do not post exploit details, proof-of-concept code, or undisclosed vulnerable packages/versions in public issues. Use private security advisories if the Git hosting service provides them. If no private channel is available, report only the affected area in a public issue and withhold details until a maintainer provides a private contact method. This policy does not promise a nonexistent security email address or response deadline.

Include the following in a report where possible:

- Affected source ID, SDK version, symbol ID, and architecture/profile.
- Generator/runtime/bridge version or commit.
- Expected and actual ABI behavior, crash details, or loading path.
- The smallest manifest/test needed to reproduce the issue. Do not attach Microsoft artifacts that may not be redistributed.
- Attacker-controlled inputs and required privileges.
- Suggested mitigations, if any.

## Supported scope

Security fixes generally target the current generator/runtime on the main branch. Continued support for older snapshots is not guaranteed until public releases and support windows are defined. Report vulnerabilities in Microsoft SDK/WDK/Windows itself through Microsoft's reporting process.

## Threat model and defenses

### Source supply chain

Official inputs are identified by the pinned HTTPS locator/version/SHA-256 in `sources.lock.json`. Do not treat an artifact as trusted parser input or an authoritative generation source before verifying its hash. Limit download sizes and extract only the relative ZIP paths listed in the lock. Do not silently fall back to another version when an optional SDK is unavailable.

Source locks, provider manifests, overrides, and ABI probe expressions require code review. In particular, ABI manifest expressions are C++ code and must not be constructed dynamically from untrusted metadata, API requests, or PR artifacts. CI tokens are read-only by default, and the SDK assessment workflow has no merge/publish permissions.

### DLL loading

Restrict system DLLs to approved basenames derived from metadata, using a System32-safe search policy and lazy resolution. Do not use the current directory or the general `PATH` to discover system DLLs. Do not pass DLL/export names directly from user input. Adding app-local DLL support requires a separate design for trusted absolute roots, canonical paths, and signing/distribution policies.

### Native memory and ownership

Do not make APIs that retain Go pointers after a call callable without dedicated native allocation, a bridge, or a registry. Keep temporary UTF-16 buffers alive through the end of the call with `runtime.KeepAlive`, and reject embedded NUL for NUL-terminated APIs. Check length/element-count multiplication and conversions between `uintptr` and Go `int` for overflow. Bound variable-buffer retry loops.

Track ownership and deallocators for handles, BSTR, HSTRING, CoTaskMem, and COM references, and use explicit `Close`/`Release` as the default. Do not rely on finalizers as the only release mechanism. Clear pointers after closing/releasing them to prevent double-free.

### ABI boundary

Do not force unknown signatures into a sequence of `uintptr` values. Float/vector arguments, varargs, aggregates passed by value, and special callbacks/calling conventions require an oracle-verified bridge/trampoline or classification as `unsupported-go-abi`. Do not let panics escape callback/COM ABI boundaries. Do not expose a callback registry until it handles retained lifetimes, concurrency, shutdown, and calls after release.

Do not present kernel-only or undocumented APIs as callable from an ordinary process. ABI mismatches, unclassified symbols, generation drift, and coverage regressions must fail CI.

## Security test constraints

Ordinary CI must not install drivers/services, write to the registry, change system configuration, or perform administrator-only operations. Reproduce destructive vulnerabilities in isolated, disposable Windows VMs; do not add those reproductions directly to repository CI. Do not include credentials, private symbols, or licensed SDK payloads in test artifacts/logs.
