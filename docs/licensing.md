# Licensing and redistribution

This document records the repository's conservative licensing policy and the
licenses observed for its inputs. It is an engineering compliance record, not
legal advice. The exact license files shipped with the exact pinned artifact
always control over this summary.

The initial inventory was checked on **2026-08-23**. On **2026-09-23**, the
updated Win32 Metadata and Windows App SDK license/NOTICE payloads were compared
byte-for-byte with the prior pins and were unchanged. Versions are pinned in
[`sources.lock.json`](../sources.lock.json); hashes, package identities, and
license files must be rechecked whenever a pin changes.

## Project code

The handwritten generator, normalized IR, runtime, bridge templates, tests,
documentation, and ABI-oracle generator in this repository are licensed under
the [Apache License 2.0](../LICENSE). New files should carry an SPDX identifier
where the file format permits it:

```text
SPDX-License-Identifier: Apache-2.0
```

Apache-2.0 applies only to copyrightable work contributed to this repository.
It does not relicense Microsoft metadata, SDK/WDK headers, NuGet payloads,
type libraries, compiler components, or other third-party material.

## Inputs and their treatment

| Input | License evidence checked | Repository policy |
| --- | --- | --- |
| `Microsoft.Windows.SDK.Win32Metadata` | The [win32metadata repository license](https://github.com/microsoft/win32metadata/blob/main/LICENSE) is MIT, but expressly says it does not change the license of the SDK headers used to produce the metadata. The pinned NuGet package carries `sdk_license.txt` and requires acceptance of the Microsoft Windows SDK license; see the [versioned NuGet license](https://www.nuget.org/packages/Microsoft.Windows.SDK.Win32Metadata/71.0.26-preview/License). | Fetch the exact pinned package, verify SHA-256, extract only into the source cache, and use it as a build input. Do **not** commit or publish the `.nupkg`, `.winmd`, or `sdk_license.txt` as repository/release content. Preserve source ID, version, hash, and both license layers in generated manifests. |
| `Microsoft.Windows.WDK.Win32Metadata` | The [wdkmetadata repository license](https://github.com/microsoft/wdkmetadata/blob/main/LICENSE) is MIT and contains the same warning about original WDK/SDK headers. The [pinned NuGet license](https://www.nuget.org/packages/Microsoft.Windows.WDK.Win32Metadata/0.13.25-experimental/License) is Microsoft Windows SDK license terms, not MIT-only. The official [EWDK terms](https://learn.microsoft.com/en-us/legal/windows-sdk/license-terms-ewdk) likewise limit redistribution to designated distributable code. | Same fetch-and-cache rule as Win32 metadata. Never infer that an MIT GitHub repository makes the WDK-derived NuGet payload or WDK headers MIT. Do not redistribute WDK input artifacts. |
| Windows SDK contract/union WinMD | The pinned installed-SDK artifact is governed by the license accompanying that Windows SDK. The NuGet alternative, [`Microsoft.Windows.SDK.Contracts`](https://www.nuget.org/packages/Microsoft.Windows.SDK.Contracts), links to the [Windows SDK license](https://aka.ms/WinSDKLicenseURL) and requires license acceptance. | Prefer an installed SDK or an exact package selected by the source provider. Record which one was used. Cache locally only; do not vendor or attach the contract WinMD to releases. |
| Windows SDK headers and import libraries | Governed by the license shipped with the installed SDK. Microsoft permits redistribution only for files designated by the applicable `REDIST.TXT`/distributable list; headers are not assumed distributable. The win32metadata license's [header disclaimer](https://github.com/microsoft/win32metadata/blob/main/LICENSE) is explicit. | Parse in place with Clang. Store hashes, include roots expressed symbolically, target/profile flags, and derived IR. Never copy the headers, import libraries, or substantial header text into this repository. Never publish them merely because the generated Go output is Apache-2.0. |
| WDK headers and libraries | Governed by the accompanying WDK/EWDK terms; the [EWDK license](https://learn.microsoft.com/en-us/legal/windows-sdk/license-terms-ewdk) identifies only designated files as distributable. | Parse only on a licensed build machine. Do not vendor headers, libraries, kits, or compiler payloads. CI may cache them privately subject to the applicable terms, but public artifacts must contain only permitted outputs. |
| `Microsoft.WindowsAppSDK` and component NuGets | The open-source [WindowsAppSDK repository](https://github.com/microsoft/WindowsAppSDK/blob/main/LICENSE) is MIT. NuGet payloads instead carry [Microsoft Windows App SDK terms](https://www.nuget.org/packages/Microsoft.WindowsAppSDK/2.5.1/License) and a package `NOTICE.txt`. Those terms identify files binplaced with an application as redistributable subject to their conditions; they do not grant blanket permission to republish the entire package as source input. | Resolve the meta-package to exact component versions and hash every package. Use WinMDs and native metadata from a local cache. Do not commit component `.nupkg`, WinMD, DLL, PRI, XAML, license, or notice payloads. If a sample/release later ships app-runtime binaries, use the package-supported deployment mechanism and reproduce the exact package license/notice beside them. |
| Windows OS TLB/OLB or type libraries embedded in DLLs | The license is that of the Windows component or external product containing the type library; there is no blanket project license for all TLBs. Office and other products have separate terms. | Read installed files for local inventory only. Record product, version, file hash, LIBID, and license reference. Do not commit or redistribute a TLB/OLB/DLL unless its exact license expressly permits it. If permission is absent or unclear, classify the source/symbol `license-restricted`; do not merge it into Windows OS coverage. |
| External SDKs (WebView2, DirectX Agility SDK, DirectStorage, and future providers) | Package-specific. A GitHub source license and a binary/NuGet license may differ. | Each provider must expose license identifiers and notice paths in its manifest, verify the exact package, and default to non-vendoring. An unavailable or unclear license is an explicit `license-restricted` or `external-sdk-not-installed` classification, never an unrecorded skip. |
| ECMA-335 specification | The official [ECMA-335 page](https://ecma-international.org/publications-and-standards/standards/ecma-335/) publishes the standard. The [sixth-edition document](https://ecma-international.org/wp-content/uploads/ECMA-335_6th_edition_june_2012.pdf) contains its own copyright notice and conditions. | Implement the format; do not vendor or modify the specification text. Link to the official copy. Short section references in source comments are identifiers, not copies of the specification. |
| Clang/LLVM toolchain | The upstream project is [Apache-2.0 WITH LLVM-exception](https://github.com/llvm/llvm-project/blob/main/LICENSE.TXT), with separately licensed material identified in its tree. | Invoke an installed/pinned tool. Do not vendor the toolchain. If a release ever bundles Clang/LLVM, reproduce the exact distribution's licenses and third-party notices rather than relying on this summary. |

The NuGet Gallery page and `sources.lock.json` are discovery/provenance records,
not substitutes for the license inside a package. Fetch/update code must fail if
the expected license file disappears or its hash changes without review.

## Generated bindings

Generated Go files contain project-authored projection structure and facts
derived from the selected official inputs. The project-authored portions are
offered under Apache-2.0, subject to any rights that continue to apply to the
inputs. A generated-file header must therefore include all of the following,
without embedding a local path or timestamp:

- `Code generated ... DO NOT EDIT.`;
- generator version;
- source ID and exact source version;
- source manifest hash;
- a link to this licensing document and `NOTICE`;
- `SPDX-License-Identifier: Apache-2.0` for the project-authored output.

The SPDX line is not a representation that Microsoft has relicensed its API
metadata or SDK content. Generated output must not contain copied header prose,
sample implementations, SDK binaries, WinMD blobs, or package license text.
Identifier names, canonical signatures, layout facts, GUIDs, and constants must
retain provenance so downstream users can identify the relevant input terms.

If an input requires a notice in generated output, the emitter must select the
notice from typed source metadata and add it to the distribution `NOTICE`; it
must not silently omit the notice or paste an unversioned boilerplate.

## ABI oracle artifacts

The C/C++ probe generator and project-authored probe templates are Apache-2.0.
Generated probe source may include SDK headers by name but must not copy header
contents. Probe executables, compiler intermediates, import libraries, and SDK
files are CI-local artifacts and are not release artifacts.

Machine-readable JSON containing measured sizes, alignments, offsets, values,
vtable indices, and compiler/SDK identities may be published under the project
license with input provenance attached. It must not contain preprocessed header
bodies or other substantial copied source. If a compiler or SDK adds a notice
requirement for a particular output, that requirement controls.

## Open-source software used or evaluated

### Incorporated dependency

The generator currently depends on
[`github.com/microsoft/go-winmd`](https://github.com/microsoft/go-winmd) at the
exact pseudo-version recorded in `go.mod`. It is MIT licensed; see its
[LICENSE](https://github.com/microsoft/go-winmd/blob/main/LICENSE). Source and
binary redistributions that include a substantial portion must retain its
copyright and MIT permission notice. `NOTICE` includes that attribution.

Any transitive dependency added to the shipped generator/runtime must be
recorded from `go list -m -json all` and its module archive. A URL or SPDX label
alone is not sufficient when the license requires reproducing its text.

### Reference/oracle only

The projects audited in [`existing-projects.md`](existing-projects.md) are not
copied merely because they were studied. In particular, generated output from
`windows-rs` or `CsWin32` is comparison data, never a primary source. Their
licenses still matter if code is later incorporated:

- [`microsoft/windows-rs`](https://github.com/microsoft/windows-rs):
  MIT OR Apache-2.0; see
  [`license-mit`](https://github.com/microsoft/windows-rs/blob/master/license-mit)
  and
  [`license-apache-2.0`](https://github.com/microsoft/windows-rs/blob/master/license-apache-2.0).
- [`microsoft/CsWin32`](https://github.com/microsoft/CsWin32):
  [MIT](https://github.com/microsoft/CsWin32/blob/main/LICENSE).
- [`golang.org/x/sys/windows`](https://github.com/golang/sys/tree/master/windows):
  [BSD-3-Clause](https://github.com/golang/sys/blob/master/LICENSE).
- Deployment Theory binding projects: MIT in each repository. They are design
  and differential-test references unless a future change documents an
  incorporated file and preserves its license.

No source code from those reference-only projects is presently relicensed as
project code.

## NOTICE and attribution

[`NOTICE`](../NOTICE) records direct third-party attribution and distinguishes
incorporated code from fetched-but-not-distributed inputs. Apache-2.0 does not
turn acknowledgements into additional license restrictions. Release packaging
must include `LICENSE`, `NOTICE`, and any notices required by actually bundled
third-party components.

Microsoft names and logos are trademarks. Attribution must not imply Microsoft
sponsorship or endorsement; see the
[Microsoft Trademark and Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks).

## Release and update checklist

Before accepting a source update or publishing an artifact:

1. Verify package/source identity, version, SHA-256, file list, dependencies,
   license identifier, and notice file against `sources.lock.json`.
2. Diff the old and new license/notice texts. A changed license requires an
   explicit review; a version bump alone is not approval.
3. Confirm that SDK, WDK, NuGet, WinMD, headers, TLBs, OLBs, DLLs, import
   libraries, and compiler payloads are absent from the repository and source
   release unless the exact license expressly allows that distribution.
4. Regenerate `NOTICE` for every dependency actually compiled into a published
   generator, bridge, helper, example, or runtime artifact.
5. Scan generated files for local absolute paths, copied header comments,
   package payloads, and unexpected binary data.
6. Preserve the exact license and notice files for any Windows App SDK runtime
   binary intentionally redistributed using Microsoft's supported deployment
   mechanism.
7. If any right or obligation remains unclear, do not publish the input or
   derived artifact. Record `license-restricted` with a source-specific reason
   and request legal review.
