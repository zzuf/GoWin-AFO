# 既知の制約

このリポジトリは包括的生成基盤の縦切りであり、Windows API 全体の完成済みバインディングではない。以下は隠さず coverage/status に反映する。

## Source ingestion

- WinMD reader は TypeDef/Field/MethodDef/Property/Event/GenericParam を保守的に inventory 化するが、全 ECMA-335 table、coded index、custom attribute の意味を全 API feature へ復元していない。
- Win32 metadata の NativeArrayInfo、MemorySize、SupportedArchitecture、FreeWith、RetVal、nullability、associated enum、availability を全 symbol へ投影していない。
- Windows SDK header の全 target/profile matrix を Clang provider で実行していない。macro、inline、SAL、pack、anonymous aggregate、bit field の full ingestion は未達である。
- native TLB/OLB/DLL resource reader はない。現在の type-library provider は JSON interchange/interface の縦切りである。
- Windows App SDK meta package は固定されているが、API payload を持つ推移 NuGet 一式の取得・衝突解決・投影は未完成である。
- Office や外部製品 API は Windows OS coverage に含めていない。

## Generation

- 汎用 emitter は fixture に加え、公式 P/Invoke のうち pointer を含まない固定整数 subset だけを出力する。SAL/ownership/retention/alignment 未投影の pointer、x86 multiword になる64-bit値、full-source Win32/WDK/WinRT inventory の全型・全関数を callable Go package にしていない。
- 現在の `projection accounting coverage = 100%` は inventory に入った集合の分類率であり、公式 SDK 全シンボルの完成率ではない。
- verified assembly trampoline はない。float/vector/varargs/特殊 aggregate ABI を一律 `uintptr` call にしない。
- generated C bridge は小規模 fixture であり、全特殊 signature や C++ flattening を実装していない。
- packed struct、複雑な union/anonymous aggregate、signed bit field、flexible array の全 header pattern を検証していない。
- ergonomic layer は一部 runtime helper だけで、raw API 全体の Go string/slice/error/iterator/context wrapper を提供しない。
- `bindings/win32`、`bindings/wdk`、`bindings/winrt` の13個の runtime-smoke 参照ファイルは専用のレビュー済み template stage で再構築し、`bindings/slice.manifest.json` で hash 検証する（`SLICEGEN-001` 解消）。汎用 IR から導出した full-source projection ではないため公式生成率には含めない。入力 pin が変わる場合は template/ABI の再レビューを要求する。

## COM

- IUnknown、apartment、BSTR、CoTaskMem の consumer 基礎だけを実装している。
- SAFEARRAY、VARIANT、PROPVARIANT、IDispatch、一般 coclass activation、connection point は未実装である。
- Go object を COM callback/server として公開する lifetime/reference-count/panic/thread registry はない。
- 全 generated interface inheritance と vtable order の oracle 検証は未達である。

## WinRT

- HSTRING、RoInitialize、IInspectable、activation の基礎だけを実装している。
- parameterized IID は未実装 error を返す。推測 IID を生成しない。
- delegate、event token、async action/operation、progress/completion、collection、closed generic の汎用生成は未実装である。
- contract version、threading model、marshaling behavior、deprecation を全 runtime class へ投影していない。

## WDK

- kernel-only export を通常の Go process から callable にしない。標準 Go runtime による kernel driver support はない。
- user-mode/UMDF/type-only/kernel-only の完全自動分類は未完成である。
- SDK/WDK duplicate の provenance は保持するが、typedef/custom attribute/layout を含む完全 semantic merge は未達である。
- WDK header の全 WINVER/NTDDI/architecture ABI probe はない。

## Verification

- checked-in ABI oracle は Windows SDK 10.0.26100.0 の小規模 fixture の x86/x64 実行結果だけである。5 record/union、2 constant、1 GUID、1 bit field、2 function signature、IUnknown の3 slotを検査するが、packed record、callback ABI、float ABI、aggregate value ABI、flexible array、generated C bridgeの独立oracle比較は未実装である（`ABI-001`）。
- ARM64 は C++/Go cross-compile を行うが、x64 hosted runner では probe/runtime を実行しない。ARM64 runtime-verified とは表示しない。
- ARM64EC は独立 ABI と認識するが、通常 Go target がないため callable backend は提供しない。
- oracle result と全 inventory symbol/Go layout の自動 mapping は未完成であり、ABI verified coverage は限定的である。
- runtime smoke は非破壊な少数 API に限る。管理者操作、driver、service、registry write、system setting change は試験しない。
- SDK update workflow は新 version の discovery と現在の pin の再現性確認までである。candidate の lock/hash を自動適用して生成・ABI 差分を作る段階は未実装で、breaking report も現在 file-level で semantic API compatibility report ではない（`SDKUP-001`）。

## Platform と配布

- module path `go-windows-api.local` は公開前の仮 path である。public release 前に単一設定元の `go.mod` と生成 import を計画的に変更する必要がある。
- optional な外部 SDK provider がない場合は `external-sdk-not-installed` として分類する。固定 Windows SDK/WinRT source は required なので欠落を fetch/verify failure にし、Windows App SDK の transitive API package 未展開は `missing-upstream-metadata` として区別する。

- bindings/bridge/report/raw dump を全て stage してから順次 rename し、通常の書き込み error は全対象を rollback する（`ATOMIC-001` の通常失敗経路を改善）。rollback 自体が失敗した場合は backup を残し、その場所を error に含める。複数 rename の間の process crash/power loss を自動復旧する永続 journal は未実装（`ATOMIC-CRASH-001`）。その場合は残った `.winapigen-stage-*` の backup を確認して復旧し、再生成で整合性を検査する。
- SDK/header/NuGet payload は license と再配布条件に従い、原則 repository に vendor しない。clean generation は network access と、WinRT/ABI 検証には該当 Windows SDK install を必要とする。
- System32 loader は実装済みだが、任意 app-local DLL の trusted absolute-path policy は未実装である。

## 完了と誤認しないために

未実装項目は `generated-*` にせず、`unsupported-go-abi`、`unsupported-projection`、`kernel-mode-only`、`missing-upstream-metadata`、`external-sdk-not-installed` 等の reason 付き status にする。新しい source/provider が認識できない symbol を黙って分母から落とさず、diagnostic と ingestion limitation を更新する。
