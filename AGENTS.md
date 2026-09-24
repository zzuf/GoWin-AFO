# GoWin-AFO repository instructions

GoWin-AFO generates Windows API bindings. Treat generated files as build
artifacts whose source of truth is the pinned official input and the generator.

- Do not edit generated code directly. Fix the generator, normalized IR,
  typed overrides, or ABI runtime instead.
- Generation must be deterministic. Never embed timestamps, machine-specific
  absolute paths, random values, or unstable iteration order in artifacts.
- Run `gofmt` on all generated and handwritten Go source.
- Never guess an ABI. If a signature cannot be proved, do not project it as
  callable; classify it with an explicit reason code.
- Every skipped or non-callable symbol must have a documented status and reason.
- Every new override must include evidence, the affected SDK version range, a
  removal condition, and a regression test.
- Restrict `unsafe` to generated ABI code, ABI runtime code, and verification
  code. Keep ergonomic packages safe by default.
- Do not use unsafe DLL search paths. System DLLs must be resolved through a
  System32-safe loader; app-local DLLs require an explicit trusted path policy.
- Compare coverage before and after generation. CI must fail when an SDK update
  lowers projection-accounting coverage or ABI-verification coverage.
- Do not leave partial generated trees on failure. Generate into a temporary
  sibling directory, validate it, and replace the destination atomically.
- Preserve native integer widths and Windows LLP64 semantics. Do not substitute
  Go `int` for C `int`, `LONG`, `DWORD`, or other fixed-width native types.
- Keep raw ABI projections separate from ergonomic wrappers.
- Before finishing work, run relevant tests, regenerate twice, inspect the
  generation diff, and review the machine-readable coverage report.

Generated files are identified by a `Code generated ... DO NOT EDIT.` header.
