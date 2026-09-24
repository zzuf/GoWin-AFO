# 既存プロジェクト監査

この文書は、本プロジェクトの設計・実装で参照する既存実装を、一次ソースを中心に監査した記録である。初回調査日は **2026-08-23**。2026-09-23 に採用依存と公式入力の更新を追記した。明記のない比較プロジェクトの更新状況は初回調査時点の記録である。件数は、特記しない限り各プロジェクト自身の README が公表する値であり、本プロジェクトが再集計して保証した値ではない。

## 2026-09-23 採用更新

- `microsoft/go-winmd` を [`16e7d31aeb8a`](https://github.com/microsoft/go-winmd/commit/16e7d31aeb8af4dd324c5c452e2263aa50b0f257) に固定。旧 `d6ac2179ffeb` 以降の generic/SZARRAY/signature 改善を利用し、API rename を adapter 側で吸収する。最低 Go 1.26 が必要。解析可能になったことと callable ABI が証明できたことは分離する。
- Win32 metadata を [71.0.26-preview](https://www.nuget.org/packages/Microsoft.Windows.SDK.Win32Metadata/71.0.26-preview)、Windows App SDK meta package を [2.5.1](https://www.nuget.org/packages/Microsoft.WindowsAppSDK/2.5.1) に更新。package を取得して hash と依存を確認し、license/NOTICE は旧 pin と byte 同一だった。
- WDK metadata は `0.13.25-experimental` を維持。ABI oracle は引き続き installed SDK `10.0.26100.0` を使用する。WinRT 入力は同一 byte 列を含む公式 `Microsoft.Windows.SDK.CPP/10.0.26100.7705` に取得元を固定した（[CI 修正記録](updates/2026-09-23-ci.md)）。新しい SDK が存在しても、未検証の header/contract へ自動切替しない。

ここでいう「再利用」は、コードまたはライブラリを依存関係として取り込むことを指す。「比較オラクル」は、同じ公式入力から得た名前、署名、レイアウト、診断、実行結果を差分比較する相手であり、相手の生成結果を一次ソースとしてコピーすることではない。Microsoft の WinMD、Windows SDK/WDK、Windows App SDK が一次入力であり、`windows-rs`、CsWin32 および Deployment Theory の生成済みバインディングは一次入力にしない。ライセンスと再配布方針は [licensing.md](licensing.md) を参照。

## 結論

| プロジェクト | ライセンス | 主入力 | 主要 API 面 | 本プロジェクトでの役割 |
| --- | --- | --- | --- | --- |
| `microsoft/go-winmd` | MIT | ECMA-335 WinMD | メタデータ読取、限定的 Go 生成 | **直接依存として再利用**。不足分は本プロジェクト側の IR と再現テストで補う |
| `microsoft/win32metadata` | MIT（入力 SDK は別条件） | Windows SDK ヘッダー等 | Win32、COM | **公式一次入力と生成規則の根拠**。NuGet WinMD を取得して読む |
| `microsoft/wdkmetadata` | MIT（入力 WDK は別条件） | WDK/SDK ヘッダー等 | WDK native 型・API | **公式一次入力**。SDK との出自を保持して正規化する |
| `microsoft/windows-rs` | MIT OR Apache-2.0 | WinMD | Win32、COM、WinRT | 比較オラクル、投影・テスト設計の参考。生成物はコピーしない |
| `microsoft/CsWin32` | MIT | Win32Metadata 互換 WinMD | Win32、COM | 比較オラクル、診断・friendly overload 設計の参考。生成物はコピーしない |
| `deploymenttheory/go-winmd` | MIT | ECMA-335 WinMD | メタデータ読取 | parser の差分・fuzz オラクル。既存の直接依存を置き換えない |
| `deploymenttheory/go-bindings-win32` | MIT | Win32Metadata | Win32、COM | Go 投影・診断の比較オラクル。raw/ergonomic 分離や 386 などは独自実装 |
| `deploymenttheory/go-bindings-wdk` | MIT | WDK + Win32Metadata | WDK、user-mode Nt/Rtl、type-only | SDK/WDK 重複解決と kernel-only 処理の比較オラクル |
| `deploymenttheory/go-bindings-winrt` | MIT | Windows SDK Contracts | WinRT | WinRT IID、イベント、async、generic 投影の比較オラクル |
| `deploymenttheory/go-bindings-windowsappsdk` | MIT | Windows App SDK NuGet 群 | Windows App SDK WinRT/native | NuGet fan-out、名前空間循環、bootstrap の比較オラクル |
| `deploymenttheory/go-bindings-wmi` | MIT | 実機 CIM repository の snapshot | WMI | 将来の独立 WMI provider の設計参考。OS API 網羅率へ混ぜない |
| `golang.org/x/sys/windows` | BSD-3-Clause | 手書き `//sys` 宣言 | curated Win32 subset | 安全な loader、last-error、実運用 API の挙動オラクル。インベントリには使わない |

監査したどのプロジェクトも、本タスクが要求する「すべての取り込みシンボルへの安定 ID、全件の backend/status 分類、`unclassified = 0`、x86/x64/ARM64 の独立 C/C++ ABI oracle、Win32/WDK/WinRT/App SDK/header/TLB をまたぐ一体的な coverage」をそのまま満たしてはいない。このため、既存の包括的な投影規則とテストを比較に利用しながら、公式入力、正規化 IR、分類、coverage、ABI 検証は本プロジェクトが保持する。

## 監査方法と保証範囲

- ライセンスは各リポジトリの LICENSE と、Microsoft の場合は成果物に同梱される別ライセンスを区別した。GitHub リポジトリが MIT でも、SDK/WDK/NuGet payload が MIT になるとは限らない。
- 更新状況は GitHub の commit/release を確認した。リリースがない場合は「なし」とし、推測した版番号を付けていない。
- ABI 検証は、(1) メタデータ同士の整合性、(2) 生成コードの compile、(3) Windows 上の runtime test、(4) MSVC/Clang と SDK ヘッダーを使う独立な `sizeof`/`alignof`/`offsetof`/署名 oracle、を区別した。前 3 者だけを (4) と同等には扱わない。
- 「既知の skip」は公開設定、diagnostics baseline、コードまたは文書で確認できたものだけを記した。網羅的な skip 一覧が公開されていない場合は「不明」とした。
- GitHub の既定ブランチは可変であるため、更新状況には immutable commit link を併記した。設計のリンクは可読性のため既定ブランチを指す。

## Microsoft プロジェクト

### `microsoft/go-winmd`

- **ライセンスと更新状況:** [MIT](https://github.com/microsoft/go-winmd/blob/main/LICENSE)。調査時 HEAD は [`d6ac2179ffeb2d1dae3fcea365f9ea24d92ae1f1`](https://github.com/microsoft/go-winmd/commit/d6ac2179ffeb2d1dae3fcea365f9ea24d92ae1f1)、2026-08-22。GitHub Release は確認できなかった。
- **入力と API 面:** PE/CLI 内の ECMA-335 metadata streams/tables、blob/signature、custom attribute を読む Go ライブラリである。[`winmd` package](https://github.com/microsoft/go-winmd/tree/main/winmd) は Win32/WinRT の特定 API 面を生成する完成品ではなく、言語中立に近い WinMD reader である。リポジトリ内の [`cmd/gowinmd`](https://github.com/microsoft/go-winmd/tree/main/cmd/gowinmd) は Go 出力の実験的 front-end である。
- **アーキテクチャと構成:** parser は host/target 非依存。`gowinmd` の golden 出力は 386、amd64、arm64 を扱うが、それは全 Windows ABI 面の保証ではない。reader、table row/model、signature/custom-attribute decoder と CLI/golden を分離している。
- **型・関数・COM・WinRT:** TypeRef/TypeDef、Field、MethodDef、MemberRef、Constant、CustomAttribute、ImplMap、generic/signature、architecture attribute など、投影に必要な基礎情報を読む。P/Invoke 関数の抽出は可能。COM と WinRT は metadata として読めるが、完全な COM vtable/runtime や WinRT activation/generic IID projection は提供しない。
- **ABI 検証:** ECMA-335 parser tests と生成 golden はあるが、SDK header を MSVC/Clang で測る独立 `sizeof`/`offsetof`/calling-convention oracle はない。[CI](https://github.com/microsoft/go-winmd/blob/main/.github/workflows/test.yml) も parser/generator tests が中心である。
- **生成物の構成:** `gowinmd` は選択された名前空間・関数から Go 型と `mkwinsyscall` 向け宣言を出す。完成した全 namespace の checked-in binding tree ではない。
- **既知の skip/制約:** 現行 `gowinmd` は、wildcard 選択時に ImplMap を持たない method を P/Invoke として生成せず、nested type の表現も本タスクの provenance/ABI IR と同じではない。varargs、packed/bit-field、特殊 calling convention、COM/WinRT runtime を全面的に解決する保証は公開されていない。parser が保持していても emitter が投影しない情報を同一視しない。
- **テスト:** table/index/blob/signature/custom attribute と malformed input の unit tests、Windows metadata fixture、386/amd64/arm64 の golden がある。公開 test tree から fuzzing と全 SDK ABI probe の有無は確認できない。
- **採用判断:** `go.mod` で pin した **ECMA-335 reader を直接再利用**する。自前 parser の無用な重複を避け、足りない row/attribute/diagnostic は、最小再現 fixture と upstream 可能な変更として追加する。`gowinmd` emitter は本タスクの安定 ID、全件分類、backend matrix を満たさないため、そのまま採用せず比較対象にする。

### `microsoft/win32metadata`

- **ライセンスと更新状況:** repository code は [MIT](https://github.com/microsoft/win32metadata/blob/main/LICENSE) だが、同 LICENSE は入力 header のライセンスを変更しないと明記する。調査時 HEAD は [`d0f363037c6987790b3548b7f82f382034d86bf2`](https://github.com/microsoft/win32metadata/commit/d0f363037c6987790b3548b7f82f382034d86bf2)、2026-08-18。最新公開 release は [`v71.0.20-preview`](https://github.com/microsoft/win32metadata/releases/tag/v71.0.20-preview)、2026-08-19。本プロジェクトの実際の pin は `sources.lock.json` が正であり、調査時最新と自動的に一致させない。
- **入力と API 面:** Windows SDK headers と補助設定を解析し、`Microsoft.Windows.SDK.Win32Metadata` NuGet の `Windows.Win32.winmd` を作る Microsoft 公式 project。[architecture](https://github.com/microsoft/win32metadata/blob/main/docs/architecture.md) は scraper、metadata transforms、WinMD generation の流れを説明し、[projection guidance](https://github.com/microsoft/win32metadata/blob/main/docs/projections.md) は consumer 側の解釈を説明する。対象は Win32 functions、DLL/entry point、struct/union/enum/constant/typedef/handle/callback、GUID、COM interface と custom attributes。WinRT contract metadata の代替ではない。
- **アーキテクチャと構成:** x86、x64、ARM64 の差異を `SupportedArchitecture` と architecture-specific definitions に保持する。単一共通定義へ誤って畳み込まず、consumer が target architecture で filter する設計である。生成 NuGet は名前空間化された一つの WinMD を主要成果物とし、Go/C#/Rust binding layout は規定しない。
- **型・関数・COM・WinRT:** native width、pointer、array、P/Invoke、SetLastError、NativeArrayInfo、MemorySize、NativeTypedef、FreeWith、RetVal、associated enum、deprecation/availability 相当を表す属性を含む。COM interface inheritance と method order を表す。WinRT runtime class/activation は対象外。
- **ABI 検証:** [`IntegrityTests.cs`](https://github.com/microsoft/win32metadata/blob/main/tests/Windows.Win32.Tests/IntegrityTests.cs) は metadata integrity、duplicate/missing reference、attribute 等を検査し、[`InterfaceTests.cs`](https://github.com/microsoft/win32metadata/blob/main/tests/Windows.Win32.Tests/InterfaceTests.cs) は COM/interface 情報を検査する。baseline/PDB/JSON を使った回帰検査もある。一方、全公開型を各 target の MSVC/Clang header と照合する独立かつ機械可読な `sizeof`/`alignof`/`offsetof` oracle は repository tests から確認できない。
- **生成物の構成:** 公式成果物は NuGet 内 WinMD と関連 metadata/config。言語 binding は consumer が生成する。
- **既知の skip/制約:** [`scraper.settings.rsp`](https://github.com/microsoft/win32metadata/blob/main/generation/WinSDK/scraper.settings.rsp) に header、symbol、macro の include/exclude/allowlist と手動補正がある。C preprocessor macro、inline、bit-field、compiler intrinsic、解析不能 signature は WinMD だけでは表現・収録できない場合がある。exclude の理由を本プロジェクトの status に引き継がず、公式 metadata にない header symbol も header provider で inventory 化する必要がある。
- **テスト:** .NET unit/integrity tests、interface/method-order tests、生成 baseline の差分検出。生成した Go ABI/runtime test はこの repository の責任範囲ではない。
- **採用判断:** **Win32 の公式一次ソース**として pin/hash verify した NuGet WinMD を読む。metadata repo の設定と tests は provenance と差分調査の根拠にする。公式 WinMD にない header-only symbol を「存在しない」とせず、Clang provider または `missing-upstream-metadata` として分類する。

### `microsoft/wdkmetadata`

- **ライセンスと更新状況:** repository code は [MIT](https://github.com/microsoft/wdkmetadata/blob/main/LICENSE) だが WDK/SDK 入力は別条件。調査時 HEAD は [`fbb3a13785073a707c6171bffbd5522438031732`](https://github.com/microsoft/wdkmetadata/commit/fbb3a13785073a707c6171bffbd5522438031732)、2026-08-05。最新公開 release は [`v0.13.25`](https://github.com/microsoft/wdkmetadata/releases/tag/v0.13.25)、2024-11-13。NuGet の experimental suffix を含む実 pin は `sources.lock.json` で固定する。
- **入力と API 面:** WDK headers と Win32 metadata/tooling を使い、`Microsoft.Windows.WDK.Win32Metadata` を生成する公式 project。[build orchestration](https://github.com/microsoft/wdkmetadata/blob/main/BuildTools/BuildTools.proj) と [`Windows.Wdk.proj`](https://github.com/microsoft/wdkmetadata/blob/main/generation/WDK/Windows.Wdk.proj) が source/build dependency を示す。WDK native types、enums、constants、callbacks、functions、COM-style interfaces を含み得るが、WinRT provider ではない。
- **アーキテクチャと構成:** WDK profile と target architecture に応じた定義を WinMD へ収録する。SDK と共通の型参照を含むため、consumer は assembly/source provenance を失わず deduplicate する必要がある。言語別 package tree は生成しない。
- **型・関数・COM・WinRT:** kernel/user-mode にまたがる型・定数・function metadata を提供する。ただし metadata にあることは、通常の Go user process から DLL export を呼べることや Go driver を作れることを意味しない。COM-like interface は native interface として扱い、WinRT projection と混同しない。
- **ABI 検証:** [`IntegrityTests.cs`](https://github.com/microsoft/wdkmetadata/blob/main/tests/Windows.Wdk.Tests/IntegrityTests.cs) は metadata integrity と baseline を検査する。全型・全 arch の独立 MSVC/Clang `sizeof`/`offsetof` oracle と user/kernel execution classification は確認できない。
- **生成物の構成:** NuGet 内 WDK WinMD が中心。Go binding layout/backend は consumer の責任である。
- **既知の skip/制約:** [`scraper.settings.rsp`](https://github.com/microsoft/wdkmetadata/blob/main/generation/WDK/scraper.settings.rsp) を機械集計すると、調査時は `--exclude` 784 entries と `--remap` 2,034 entries がある。これは最終 symbol の skip 総数ではなく、重複置換や手動補完も含む scraper 設定件数である。理由と粒度は項目ごとに同一ではない。header-only、compiler-specific、kernel-only、metadata 不完全を一つの skip にまとめず再分類する必要がある。また古い README の SDK/WDK Build 22000 記述を current tool pin とみなさず、調査時 [`BuildTools.proj`](https://github.com/microsoft/wdkmetadata/blob/main/BuildTools/BuildTools.proj) の `10.0.22621.755` と実 package manifest を版ごとに優先する。
- **テスト:** build/integrity/baseline tests。通常 user process での安全な call、UMDF、kernel-only、type-only を全件判定する runtime suite は確認できない。
- **採用判断:** **WDK の公式一次ソース**として別 source ID で取り込む。SDK と同名型を単純二重生成せず provenance を保持して正規化する。DLL export、documented execution environment、header evidence を用いて `user-mode`、`UMDF`、`type-only`、`kernel-mode-only`、`metadata incomplete` を本プロジェクトで独自分類する。

### `microsoft/windows-rs`

- **ライセンスと更新状況:** [MIT](https://github.com/microsoft/windows-rs/blob/master/license-mit) OR [Apache-2.0](https://github.com/microsoft/windows-rs/blob/master/license-apache-2.0)。調査時 HEAD は [`acaaa37be53e68f8c1be575d24dcbebce1e92b1a`](https://github.com/microsoft/windows-rs/commit/acaaa37be53e68f8c1be575d24dcbebce1e92b1a)、2026-08-18。最新公開 release は [`73`](https://github.com/microsoft/windows-rs/releases/tag/73)、2026-02-17。
- **入力と API 面:** Windows metadata を [`windows-bindgen`](https://github.com/microsoft/windows-rs/blob/master/docs/crates/windows-bindgen.md) で Rust へ投影する。Win32、COM、WinRT を広く扱い、runtime/core/support crates を分離する。[dependency overview](https://github.com/microsoft/windows-rs/blob/master/docs/dependencies.md) が crates の関係を示す。
- **アーキテクチャと構成:** `windows`（idiomatic projection）、`windows-sys`（低水準）、`windows-core`、`windows-future`、`windows-collections` 等へ分割し、namespace feature により不要面を compile しない。Rust がサポートする複数 Windows targets を扱うが、Go の 386/amd64/arm64 backend が正しいことの証明にはならない。
- **型・関数・COM・WinRT:** native types/functions/constants/callback、COM interface/vtable/QueryInterface、WinRT runtime class/activation、HSTRING、delegate/event、async、collection、generic/parameterized interface を実装する。Win32 と WinRT の最も包括的な比較対象の一つである。
- **ABI 検証:** [`windows-clang`](https://github.com/microsoft/windows-rs/blob/master/docs/crates/windows-clang.md) と bindgen tests は header/Clang を用いた差分、RDL/golden、C++/WinRT interop、target 別 compile/runtime tests を持つ。[test workflow](https://github.com/microsoft/windows-rs/blob/master/.github/workflows/test.yml) と [generation workflow](https://github.com/microsoft/windows-rs/blob/master/.github/workflows/gen.yml) は再生成 drift を検査する。ただし本タスクと同じ全 symbol × 全 architecture の machine-readable `sizeof`/`alignof`/`offsetof` coverage report が公開されているとは確認できない。
- **生成物の構成:** crates と feature を namespace/dependency 単位に分割し、低水準 `windows-sys` と ergonomic `windows` を分ける。巨大単一 package を避ける設計の有力な参考である。
- **既知の skip/制約:** bindgen は選択された metadata/header surface の reachable types を生成する。header allowlist 外、library mapping がない function、表現できない rich variadic、特殊 calling convention（例: 一部 `__fastcall`）、複雑な C/C++ ABI を無条件には公開しない。exact skip 件数と全理由の単一公開台帳は確認できない。
- **テスト:** codegen golden/RDL、Clang/header 差分、crate compile、Win32/COM/WinRT runtime/interop、複数 target workflow。これらは強い比較材料だが、Rust ABI 成功を Go FFI 成功へ外挿しない。
- **採用判断:** COM/WinRT projection、parameterized IID、namespace feature partition、low-level/ergonomic split、generation drift test の **比較オラクル**として使う。生成 Rust をコピーしたり真の metadata として再解析したりしない。Go runtime/FFI capability 判定と ABI oracle は独自に実装する。

### `microsoft/CsWin32`

- **ライセンスと更新状況:** [MIT](https://github.com/microsoft/CsWin32/blob/main/LICENSE)。調査時 HEAD は [`b0f1e799e6aa793eccffafa6ba8164ca45a9266f`](https://github.com/microsoft/CsWin32/commit/b0f1e799e6aa793eccffafa6ba8164ca45a9266f)、2026-08-14。調査時最新 NuGet は [`Microsoft.Windows.CsWin32 0.3.321`](https://www.nuget.org/packages/Microsoft.Windows.CsWin32/0.3.321)、2026-08-17。NuGet が示す同版の GitHub release URL は調査時 404 であり、最後に実体を確認できた GitHub Release は [`v0.3.298`](https://github.com/microsoft/CsWin32/releases/tag/v0.3.298)、2026-06-22 だった。この三つを混同しない。0.3.321 の versioned NuGet page は Win32Metadata `>= 71.0.14-preview`、WDK metadata `>= 0.13.25-experimental`、Win32Docs `>= 0.1.42-alpha` を依存入力として示す。
- **入力と API 面:** `Microsoft.Windows.SDK.Win32Metadata` または [third-party compatible metadata](https://microsoft.github.io/CsWin32/docs/3rdPartyMetadata.html) を入力にする C# incremental source generator。[Generator](https://github.com/microsoft/CsWin32/blob/main/src/Microsoft.Windows.CsWin32/Generator.cs) が要求された API と依存型を生成する。調査時 HEAD の [`Directory.Packages.props`](https://github.com/microsoft/CsWin32/blob/main/Directory.Packages.props) は Win32Metadata `71.0.14-preview`、WDK metadata `0.13.25-experimental`、Win32Docs `0.1.42-alpha` を固定する。Win32 P/Invoke、types、constants、COM、metadata 化された一部 macro と friendly overload が対象で、Windows SDK contract WinRT projection ではない。
- **アーキテクチャと構成:** NuGet source generator と analyzer を consumer build に組み込み、`NativeMethods.txt` の選択から partial classes/types を生成する。architecture-specific API は target/要求方法によって filter/diagnostic される。[architecture-specific API guidance](https://microsoft.github.io/CsWin32/docs/ArchSpecificAPIs.html) によれば、明示要求の不一致は `PInvoke005`、wildcard の不一致は省略となる。
- **型・関数・COM・WinRT:** C# P/Invoke、struct/union/enum/handle、delegate、COM interface、friendly overload、SafeHandle 等を生成する。WinRT runtime class、activation、parameterized IID の包括的 generator ではない。
- **ABI 検証:** generator unit/golden と [`FullGenerationTests.cs`](https://github.com/microsoft/CsWin32/blob/main/test/Microsoft.Windows.CsWin32.Tests/FullGenerationTests.cs) による AnyCPU/x86/x64/arm64 Roslyn compile、Windows runtime/COM tests がある。一方、integration test 内の native C/C++ fixture は限定的であり、全 generated type/signature を target ごとの MSVC/Clang SDK header と測定比較する独立 machine-readable ABI oracle は確認できない。.NET marshaler/source-generator の成功を Go syscall ABI の証拠にはしない。
- **生成物の構成:** consumer compilation の source generator/AddSource または build task として要求 API と transitive dependencies を生成する。複数 file mode は型ごとの `{Type}.g.cs`、`Delegates.g.cs`、P/Invoke partial class 等、`emitSingleFile=true` は `NativeMethods.g.cs` となるため、全 SDK を checked-in namespace packages として出す構成ではない。friendly overload と raw extern を同じ生成 ecosystem 内で層別化する。
- **既知の skip/制約:** [`Generator.Invariants.cs`](https://github.com/microsoft/CsWin32/blob/main/src/Microsoft.Windows.CsWin32/Generator.Invariants.cs) は常時または marshaling mode で生成しない symbol と代替理由を持つ。例は `GetLastError`、legacy/overridden integer・layout types、vectored-exception helpers、marshaling mode の `IUnknown`/`IDispatch`/`VARIANT` である。architecture mismatch、metadata にない macro/inline/header symbol、C#/.NET で安全に投影できない construct、要求から reachable でない API も生成されない。wildcard skip は明示要求時 diagnostic と挙動が異なるため、本プロジェクトの全件 inventory/status にはそのまま流用できない。[`GeneratorOptions.cs`](https://github.com/microsoft/CsWin32/blob/main/src/Microsoft.Windows.CsWin32/GeneratorOptions.cs) の既定 `WideCharOnly=true` は ANSI variant を省略して W suffix を外すため、本プロジェクトの A/W 両方を native name で保持する方針とも異なる。既知 skip の全件 machine-readable ledger は確認できない。
- **テスト:** source-generator unit/golden、AnyCPU/x86/x64/arm64 compile、diagnostic、bit-field/flexible-array/callback tests、[`ComRuntimeTests.cs`](https://github.com/microsoft/CsWin32/blob/main/test/GenerationSandbox.Tests/ComRuntimeTests.cs) を含む COM/Direct2D/WMI/Shell の Windows runtime tests。[build workflow](https://github.com/microsoft/CsWin32/blob/main/.github/workflows/build.yml) がこれらを編成する。公開 suite は C# projection 品質には有用だが、WDK kernel classification、一般 WinRT projection、Go callbacks/FFI は範囲外。
- **採用判断:** metadata attribute 解釈、architecture diagnostic、friendly overload、SafeHandle、COM テストの比較オラクルにする。生成 C# は一次入力にせず、C# marshaling 前提も移植しない。本プロジェクトは raw/ergonomic 分離、Go ABI backend、header/WDK/WinRT classification を独自に持つ。

## Deployment Theory プロジェクト

### `deploymenttheory/go-winmd`

- **ライセンスと更新状況:** [MIT](https://github.com/deploymenttheory/go-winmd/blob/main/LICENSE)。調査時 HEAD は [`c9712c91de8ba2b634eac01edd2a280db234968e`](https://github.com/deploymenttheory/go-winmd/commit/c9712c91de8ba2b634eac01edd2a280db234968e)、2026-08-18。最新公開 release は [`v1.0.0`](https://github.com/deploymenttheory/go-winmd/releases/tag/v1.0.0)、2026-08-14。
- **入力と API 面:** [README](https://github.com/deploymenttheory/go-winmd) によれば、標準ライブラリだけで PE/CLI、`#~`/`#-`、String/Blob/GUID heaps、45 table の sizing と主要 22 table、signature/generic、constant/custom attribute を読む ECMA-335 reader。Win32/WinRT binding generator そのものではない。
- **アーキテクチャと構成:** `pkg/winmd`、NuGet source helper、update command、pinned testdata を分離する。parser は target-independent。実 WinMD を hash/version pin して test する設計である。
- **型・関数・COM・WinRT:** TypeRef/TypeDef、Field/MethodDef/MemberRef、signature、generic/custom attribute 等を materialize するため各 API 面の raw metadata は読めるが、COM vtable/runtime と WinRT activation projection は提供しない。
- **ABI 検証:** reader correctness と real-WinMD traversal tests はあるが、生成言語型や C ABI の oracle はない。
- **生成物の構成:** binding code は生成しない。update/fetch tool と fixture が中心。
- **既知の skip/制約:** README の non-goals は lazy row access、任意 coded-index tag の一般 API、`#US`、BYREF/multidimensional array の完全 decode 等。README にある他 parser との比較は執筆時点の記述であり、更新された `microsoft/go-winmd` の現状を証明しないため採否根拠にしない。
- **テスト:** truncated/malformed bounds、table/signature/custom-attribute、pinned `Windows.Win32.winmd` と `UniversalApiContract` の大規模 traversal。公表される traversal 件数は project self-report である。
- **採用判断:** hostile-input handling、real-fixture regression、parser fuzz corpus の **差分オラクル**として利用価値がある。ただし本プロジェクトは既に `microsoft/go-winmd` を pin しており、二つの reader を production path に重ねない。必要な改善は再現 test を付け、可能なら upstream 可能な形で行う。

### `deploymenttheory/go-bindings-win32`

- **ライセンスと更新状況:** [MIT](https://github.com/deploymenttheory/go-bindings-win32/blob/main/LICENSE)。調査時 HEAD は [`5421f07c62f07e5ebfc2c009c12de9bf0ead2c5f`](https://github.com/deploymenttheory/go-bindings-win32/commit/5421f07c62f07e5ebfc2c009c12de9bf0ead2c5f)、2026-08-21。最新公開 release は [`v0.3.1`](https://github.com/deploymenttheory/go-bindings-win32/releases/tag/v0.3.1)、2026-08-14。
- **入力と API 面:** [README](https://github.com/deploymenttheory/go-bindings-win32) によれば Win32Metadata を native Go reader で読み、Win32 functions/types/constants/callbacks/COM を生成する。README 自称値は 324 packages、約 17.7k functions、約 43.7k COM methods、約 16.5k structs。
- **アーキテクチャと構成:** `bindings/win32/<namespace>`、共通 `bindings/runtime/win32`、generator/metadata、acceptance に分離する。生成 file は `windows && (amd64 || arm64)` で、386 は対象外。namespace package 分割は巨大 import を避ける有力な参考である。
- **型・関数・COM・WinRT:** structs/unions/enums/handles、inline syscall wrappers、typed COM vtables/methods、callbacks、文字列/bool/error/slice/handle の ergonomic helper を含む。WinRT surface は対象外。raw と ergonomic な変換が本タスクの要求ほど明確に別 module/package ではない。
- **ABI 検証:** [`acceptance/abi_generated_test.go`](https://github.com/deploymenttheory/go-bindings-win32/blob/main/acceptance/abi_generated_test.go) は 426 sampled structs について metadata から計算した amd64 expected layout を Go `unsafe` と比較する。README はより大きな generated ABI assertion 数も述べるが、sample file と区別する。これは regression として有用だが、同じ metadata から両辺を作るため独立 MSVC/Clang header oracle ではない。live acceptance tests は syscall/COM behavior を検証する。
- **生成物の構成:** namespace packages、runtime、diagnostics baseline、acceptance。生成コードを repository に保持し deterministic regenerate/diff を行う。
- **既知の skip/制約:** [diagnostics baseline](https://github.com/deploymenttheory/go-bindings-win32/blob/main/metadata/diagnostics-baseline.json) に packed struct、by-value struct、float/special ABI 等の generator diagnostics を固定する。amd64/arm64 のみで、386 decorated/calling convention は扱わない。全 symbol を本タスクの status 語彙へ分類した coverage ではない。
- **テスト:** [acceptance suite](https://github.com/deploymenttheory/go-bindings-win32/tree/main/acceptance) に COM、COM parameters/events、handle、informational HRESULT、retval、slice、loader、out parameter、struct argument 等がある。generator/emitter unit tests と drift/diagnostics ratchet もある。
- **採用判断:** namespace partition、metadata-to-Go mapping、last-error/HRESULT、typed COM、diagnostics ratchet の比較オラクルにする。公式 WinMD 以外の生成結果を source にせず、386、独立 ABI oracle、全 backend/status、strict raw layer は独自実装する。

### `deploymenttheory/go-bindings-wdk`

- **ライセンスと更新状況:** [MIT](https://github.com/deploymenttheory/go-bindings-wdk/blob/main/LICENSE)。調査時 HEAD は [`447a2d591fcba9725fa5bb4bf3486b21bcf19ab1`](https://github.com/deploymenttheory/go-bindings-wdk/commit/447a2d591fcba9725fa5bb4bf3486b21bcf19ab1)、2026-08-04。最新公開 release は [`v0.1.1`](https://github.com/deploymenttheory/go-bindings-wdk/releases/tag/v0.1.1)、2026-07-16。
- **入力と API 面:** [README](https://github.com/deploymenttheory/go-bindings-wdk) によれば WDK metadata と pinned Win32 metadata を同時に読み、WDK-origin symbols を生成し、共通型は Win32 module 参照にする。user-mode で export される Nt/Rtl 等と WDK types/constants/enums、kernel-only function に必要な型を扱う。
- **アーキテクチャと構成:** generated package は [representative package](https://github.com/deploymenttheory/go-bindings-wdk/blob/main/bindings/wdk/system/systemservices/doc.go) のとおり amd64/arm64 build tag。Win32 module を dependency にし、WDK package tree と runtime/metadata/acceptance を分離する。386 は対象外。
- **型・関数・COM・WinRT:** WDK structs/enums/constants/callbacks と user-mode DLL export function を生成する。kernel-only function は通常の Go callable wrapper として出さず、関連型は保持する。WinRT provider ではない。
- **ABI 検証:** [`acceptance/abi_generated_test.go`](https://github.com/deploymenttheory/go-bindings-wdk/blob/main/acceptance/abi_generated_test.go) は 477 sampled structs の metadata-derived amd64 expected layout を比較する。同じ metadata に依存するため、独立 C compiler oracle ではない。ntdll live test は user-mode subset の有効性を確認する。
- **生成物の構成:** `bindings/wdk/<namespace>`、Win32 module import、diagnostics baseline、acceptance。
- **既知の skip/制約:** DLL user export のない kernel function を call wrapper から除外する。experimental upstream metadata、packed/special ABI、amd64/arm64 限定が制約。`kernel-mode-only`、`type-only`、`metadata incomplete` 等を本タスクの全件 status として公開する仕組みではない。
- **テスト:** generator/unit、metadata diagnostics ratchet、sampled ABI、safe user-mode Nt/Rtl acceptance。driver execution test はなく、ないことが正しい。
- **採用判断:** SDK/WDK cross-assembly reference と「kernel function を通常 process callable と偽らない」設計の比較オラクルにする。本プロジェクトは provenance-preserving dedup、より細かい WDK status、386/cross compile、independent oracle を追加する。

### `deploymenttheory/go-bindings-winrt`

- **ライセンスと更新状況:** [MIT](https://github.com/deploymenttheory/go-bindings-winrt/blob/main/LICENSE)。調査時 HEAD は [`85dfdd59d190875e6a0c204d5805182515a30f9d`](https://github.com/deploymenttheory/go-bindings-winrt/commit/85dfdd59d190875e6a0c204d5805182515a30f9d)、2026-08-18。最新公開 release は [`v0.6.0`](https://github.com/deploymenttheory/go-bindings-winrt/releases/tag/v0.6.0)、2026-07-31。
- **入力と API 面:** [README](https://github.com/deploymenttheory/go-bindings-winrt) によれば `Microsoft.Windows.SDK.Contracts` の Windows SDK contract WinMD を読み、全取り込み namespace を Go package へ投影する。README 自称値は 282 generated packages。
- **アーキテクチャと構成:** amd64/arm64 の generated packages、committed normalized IR、runtime、namespace packages、acceptance、diagnostics baseline。closed generic instantiation を monomorphize し、package dependency を管理する。
- **型・関数・COM・WinRT:** IUnknown/IInspectable、HSTRING、runtime class、interface、factory/statics、delegate/event、async `Await`、collections、generic interface、parameterized IID、value structs を扱う。COM は WinRT ABI の基盤として扱い、Win32 P/Invoke generator ではない。
- **ABI 検証:** slot/IID/generic signature tests、generated layout tests、Windows live activation/event/async/collection tests がある。[float/struct ABI test](https://github.com/deploymenttheory/go-bindings-winrt/blob/main/acceptance/abi_float_struct_test.go) は amd64 syscall/XMM behavior を重点確認する。arm64 は ABI が異なる。全 WinRT signature を C++/WinRT oracle と照合する machine-readable coverage は確認できない。
- **生成物の構成:** namespace packages、WinRT runtime、committed IR、diagnostics baseline、acceptance。循環を避ける runtime/foundation 分離がある。
- **既知の skip/制約:** [diagnostics baseline](https://github.com/deploymenttheory/go-bindings-winrt/blob/main/metadata/diagnostics-baseline.json) は delegate-returning methods、array、float ABI、wide by-value structs 等を追跡する。Go map projection、任意の composable/custom class、全 architecture は未対応または制約付き。slot を壊さないため skipped member を comment/diagnostic として保持する点は重要である。
- **テスト:** [acceptance](https://github.com/deploymenttheory/go-bindings-winrt/tree/main/acceptance) に calendar、delegates/events、async、collections、toast、BLE、speech、IID/vtable tests がある。破壊的でない subset と環境依存 test を分ける必要がある。
- **採用判断:** HSTRING、activation、events、async、closed generics、parameterized IID、vtable slot preservation の比較オラクルにする。generated code はコピーせず、任意 generic、architecture/backend status、C++ oracle は本プロジェクトが実装する。

### `deploymenttheory/go-bindings-windowsappsdk`

- **ライセンスと更新状況:** [MIT](https://github.com/deploymenttheory/go-bindings-windowsappsdk/blob/main/LICENSE)。調査時 HEAD は [`03ba1cac83e3f6274d5308d49dce714b0d6be3f6`](https://github.com/deploymenttheory/go-bindings-windowsappsdk/commit/03ba1cac83e3f6274d5308d49dce714b0d6be3f6)、2026-08-19。最新公開 release は [`v0.1.0`](https://github.com/deploymenttheory/go-bindings-windowsappsdk/releases/tag/v0.1.0)、2026-08-03。
- **入力と API 面:** [README](https://github.com/deploymenttheory/go-bindings-windowsappsdk) によれば `Microsoft.WindowsAppSDK` meta-package と component fan-out（36 WinMDs）を解決し、Windows App SDK WinRT/native surface を投影する。WinRT と Win32 binding modules に依存する。README 自称値は 77 namespaces、4,374 types、64 packages/309 files。
- **アーキテクチャと構成:** 調査時は amd64 only。14 個の循環 XAML namespaces を SCC としてまとめ、runtime/bootstrap、WinRT/Win32 dependencies、generated packages、acceptance を分離する。Windows App SDK runtime/bootstrap の installed state が live execution に必要である。
- **型・関数・COM・WinRT:** App Lifecycle、Windowing、Notifications、MRT、XAML 等の runtime classes/interfaces/events/delegates と native bootstrap 関係を扱う。外部 WebView2 metadata は package 外 dependency で、Windows SDK WinRT と provenance を区別する必要がある。
- **ABI 検証:** [`internal/verify/abi_test.go`](https://github.com/deploymenttheory/go-bindings-windowsappsdk/blob/main/internal/verify/abi_test.go) は metadata name、IID/vtable slot 等を pin し、live window/events/layout tests が実 runtime を確認する。全 native layout/signature の独立 C/C++ oracle は確認できない。
- **生成物の構成:** package graph/SCC、runtime/bootstrap、metadata IR/diagnostics、generated bindings、acceptance。
- **既知の skip/制約:** [diagnostics baseline](https://github.com/deploymenttheory/go-bindings-windowsappsdk/blob/main/metadata/diagnostics-baseline.json) は調査時 135 diagnostics（README はうち 114 を intentional と説明）、missing WebView2 external package、delegate typedef/returned delegate getter policy 等を追跡する。Go object derivation/custom XAML control は COM aggregation を要し未対応。arm64/386 は対象外。
- **テスト:** [acceptance](https://github.com/deploymenttheory/go-bindings-windowsappsdk/tree/main/acceptance) に real window、styling、UI events、apartment、layout、string array 等がある。headless CI と installed runtime 条件を分離する必要がある。
- **採用判断:** NuGet meta-package fan-out、component provenance、XAML SCC、bootstrap/runtime detection、external dependency diagnostics の比較オラクルにする。本プロジェクトは official component packages を直接 pin し、Windows SDK との衝突を provenance-aware IR で解決し、arm64/386 と status coverage を独自に扱う。

### `deploymenttheory/go-bindings-wmi`

- **ライセンスと更新状況:** [MIT](https://github.com/deploymenttheory/go-bindings-wmi/blob/main/LICENSE)。調査時 HEAD は [`c09788679926f8e3ba7c40719e5212e638a91774`](https://github.com/deploymenttheory/go-bindings-wmi/commit/c09788679926f8e3ba7c40719e5212e638a91774)、2026-08-18。最新公開 release は [`v1.0.0`](https://github.com/deploymenttheory/go-bindings-wmi/releases/tag/v1.0.0)、2026-07-22。
- **入力と API 面:** [README](https://github.com/deploymenttheory/go-bindings-wmi) によれば、実 Windows CIM repository を query し、committed JSON snapshot から WMI class/method/property binding を生成する。`root/cimv2`、`StandardCimv2`、`SecurityCenter2`、curated virtualization v2、HGS、`dmmap` の 6 namespaces が対象で、WinMD/SDK header の native API inventory とは別系統である。
- **アーキテクチャと構成:** capture tool → versioned JSON snapshot → deterministic generator → namespace packages → runtime/acceptance。generated files は Windows build tag。実効 target architecture は Win32 dependency と toolchain の対応にも依存し、README から独立した完全 target list は確認できないため推測しない。
- **型・関数・COM・WinRT:** typed WMI class structs/fields、enums/bitmasks、query/get/method wrappers。runtime は IWbem COM、VARIANT/BSTR/SAFEARRAY を使う。Win32 P/Invoke/WinRT runtime class の包括 generator ではない。
- **ABI 検証:** schema snapshot/generation tests と live WMI acceptance はあるが、native struct/calling convention の C ABI oracle はない。主な互換対象は host の CIM schema と COM automation marshalling である。
- **生成物の構成:** namespace-specific generated packages、metadata JSON snapshots（例: [`root.cimv2.json`](https://github.com/deploymenttheory/go-bindings-wmi/blob/main/metadata/cim/root.cimv2.json)）、capture/generator、runtime、acceptance。
- **既知の skip/制約:** host/Windows edition により schema が変化する。対象は 6 captured namespaces、virtualization は curated subset。`dmmap` の capture/query は SYSTEM 権限を要する場合があり、通常 CI へ入れられない。Office/third-party WMI providers を Windows OS metadata coverage に混ぜられない。
- **テスト:** [acceptance](https://github.com/deploymenttheory/go-bindings-wmi/tree/main/acceptance) は実 query/method/variant conversion を検査する。環境・権限依存 test と pure snapshot/generator test を分ける。
- **採用判断:** 将来の WMI provider における capture provenance、schema diff、COM automation conversion の設計参考にする。本タスクの Win32/WDK/WinRT/App SDK の projection coverage へ件数を加えず、別 source/coverage とする。

## Go 標準周辺

### `golang.org/x/sys/windows`

- **ライセンスと更新状況:** [BSD-3-Clause](https://github.com/golang/sys/blob/master/LICENSE)。`windows` path の調査時最新 commit は [`effdf5d9636b622bdffd9eb2df870dd3e93075b9`](https://github.com/golang/sys/commit/effdf5d9636b622bdffd9eb2df870dd3e93075b9)、2026-08-20。調査時最新 module tag は `v0.47.0`。GitHub Release object ではなく module tag である。
- **入力と API 面:** [`syscall_windows.go`](https://github.com/golang/sys/blob/master/windows/syscall_windows.go) 等の手書き `//sys` declarations と types/constants が source of truth で、公式 metadata の全件 inventory ではない。[`mksyscall.go`](https://github.com/golang/sys/blob/master/windows/mksyscall.go) が generator invocation を定義する。長年使われる curated Win32 subset、registry、service、security、networking 等を提供する。
- **アーキテクチャと構成:** `windows` root と `registry`、`svc`、`mkwinsyscall` 等の subpackages、386/amd64/arm/arm64 specific type files、generated [`zsyscall_windows.go`](https://github.com/golang/sys/blob/master/windows/zsyscall_windows.go) 等からなる。全 Microsoft namespace に対応する package partition ではない。
- **型・関数・COM・WinRT:** 多数の native handles/types/functions/constants と callback helper を提供する。COM は GUID/VARIANT 等の限定的 primitive/use-case が中心で、metadata-driven full COM interface generator ではない。WinRT runtime class/generic projection、WDK full surface は対象外。
- **ABI 検証:** Go compile/runtime tests と実 Windows API で長期に検証されているが、SDK 全 symbol の MSVC/Clang machine-readable layout/signature oracle や coverage accounting はない。architecture-specific handwritten layout は個別 test/review に依存する。
- **生成物の構成:** handwritten declarations/types と `mkwinsyscall` generated `z*` files、domain subpackages。API 選択は人手であり、公式 source update による自動全件差分は出ない。
- **既知の skip/制約:** curated subset なので、収録されない API の全件 skip reason/stable ID はない。COM/WinRT/WDK、header-only macro/inline、特殊 ABI を網羅しない。これは欠陥というより package scope である。
- **テスト:** [`syscall_windows_test.go`](https://github.com/golang/sys/blob/master/windows/syscall_windows_test.go) を含む広い Windows runtime tests、registry/service/security/domain tests、architecture builds。通常 CI に不適切な破壊的/管理者操作は本プロジェクトの smoke test へ無条件に移植しない。
- **採用判断:** [`dll_windows.go`](https://github.com/golang/sys/blob/master/windows/dll_windows.go) の `NewLazySystemDLL` による System32-safe load、lazy proc、last-error semantics、UTF-16/handle helper を挙動オラクルとする。`Proc.Call` の error は primary return の失敗条件を確認してから使う。正確なコードを採用する場合だけ BSD notice を保持する。手書き curated list を本プロジェクトの source inventory にはしない。

## 再利用と独立実装の境界

本プロジェクトで production dependency として直ちに再利用するのは `microsoft/go-winmd` の reader である。公式 Win32/WDK/App SDK repository は「OSS の生成済み binding」ではなく、pin された Microsoft metadata/NuGet を取得する一次 source provenance として扱う。

次は比較オラクルとして利用するが、生成物をコピーしない。

- `windows-rs`: Win32/COM/WinRT の名前、signature、namespace dependency、parameterized IID、runtime behavior。
- CsWin32: metadata attribute interpretation、architecture diagnostic、friendly/raw layering、COM/PInvoke behavior。
- Deployment Theory bindings: Go 固有の package split、syscall/COM/WinRT runtime、diagnostics ratchet、SDK/WDK dedup、NuGet fan-out。
- `x/sys/windows`: safe DLL loading、last-error、UTF-16/handle lifetime、実 API behavior。

次は本プロジェクトが独立に実装・所有する。

- source ID、version/hash/file inventory と provider interface;
- canonical signature に基づく stable symbol ID と provenance-preserving normalized IR;
- 全 symbol の status/reason/backend classification と `unclassified = 0` gate;
- Pure Go、assembly、generated C bridge、type-only、unsupported の capability decision;
- Clang AST による header macro/inline/bit-field/packed/flexible-array 補完;
- x86/x64/ARM64 の独立 MSVC/clang-cl ABI oracle と machine-readable mismatch;
- projection、generation、ABI、runtime を分離した coverage と regression gate;
- raw ABI layer と ergonomic layer の明示的分離。

この境界により、既存実装の成熟した知見を活用しつつ、「別言語で生成できた」「Go で compile した」「同じ metadata から計算した layout が一致した」だけを Windows ABI の完全な証拠にしない。
