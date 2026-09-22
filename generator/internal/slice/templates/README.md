# Reviewed vertical-slice templates

These templates are the source of the 13 small bindings under
`bindings/win32`, `bindings/wdk`, and `bindings/winrt`. The general metadata
emitter produces `bindings/generated` separately. The templates preserve the
slice's reviewed ABI decisions; they do not count as additional official
metadata projections in the coverage report.

The current evidence basis is the exact Win32, WDK, and WinRT source versions
and SHA-256 values in `sources.lock.json`, checked by `slice.verifyPins` before
rendering. Each output records its source version and slice descriptor hash;
`bindings/slice.manifest.json` records the source hashes and the hashes of the
rendered files. A descriptor hash identifies its stable descriptor name, not
the template contents; the artifact hashes cover actual rendered bytes. The
Windows SDK ABI version for the architecture-specific QPC
call is `10.0.26100.0`.

The affected version range is the exact pinned versions in `slice.go`; a
source-lock update must fail until the templates and native ABI evidence are
reviewed. The regression tests are `generator/internal/slice/slice_test.go`,
`generator/slice_integration_test.go`, the binding layout tests, and the native
ABI oracle run by verification. Remove a template when the general metadata
emitter can reproduce that binding from locked inputs with the same verified
ABI and provenance; until then, the reviewed template remains authoritative.

Upstream licensing and notices are in `docs/licensing.md` and `NOTICE`. These
templates are project-owned transformations of the pinned metadata and ABI
evidence, under the repository's Apache-2.0 license.
