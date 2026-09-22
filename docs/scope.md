# スコープ

## 現在地

このリポジトリは、Windows API 全体の完成済みバインディング集ではない。現在の実装は、Windows SDK `10.0.26100.0` を基準に、取得、ハッシュ検証、WinMD 読み取り、正規化 IR、型付き override、決定的な Go 出力、実行時 ABI、網羅性集計を接続した小規模な end-to-end 縦切りである。

コミット済みの縦切りは、Win32 の foundation 型と一部の Kernel32 API、COM/WinRT の基礎 runtime、WDK の type-only 例を含む。`generator/testdata/e2e/source.json` は 23 個の検証用レコードを持つが、これはこのプロジェクト自身の fixture であり、Microsoft 公式 API の網羅率の分母へ数えてはならない。

`winapigen generate --all` は取得済み WinMD の `TypeDef`、`Field`、`MethodDef`、`Property`、`Event`、`GenericParam` を保守的にインベントリ化する。公式 metadata の P/Invoke のうち、DLL/entry point が固定され、pointer、64-bit値渡し、特殊 ABI、未解決の名前付き型を含まない固定整数 subset だけを `generated-purego` として namespace 別 `bindings/generated` へ出力する。pointer は SAL、保持期間、ownership、x86 alignment を解釈するまで生成しない。それ以外の大多数は理由付きで分類したまま残すため、全 Win32、全 WDK、全 WinRT、Windows App SDK の Go 投影は未達である。

## 目標とする API 面

長期的な対象は次のとおりである。

- Windows SDK の公開 Win32 API
- COM consumer 用の interface、GUID、vtable、所有権情報
- Windows Runtime / WinRT の contract metadata と runtime class
- WDK の型、定数、およびユーザーモードで利用可能と証明できる API
- Windows App SDK の WinMD と native API
- WinMD にない SDK header の macro、inline、bit field、pack、条件付き宣言
- TLB、OLB、DLL 埋め込み type library の独立したインベントリ
- WebView2、DirectX Agility SDK、DirectStorage などの外部 SDK provider

外部製品の type library と外部 SDK は、Windows OS API と別の source ID と別のカバレッジ分母を持つ。Office などを OS 網羅率へ混ぜない。

## 現在実装されている縦切り

| 領域 | 現在の実装 | 保証しないこと |
|---|---|---|
| Source | version と SHA-256 を固定した NuGet / ローカル Windows SDK 取得、選択ファイル抽出、再検証 | 任意の SDK install の自動発見、App SDK の推移依存 API 一式 |
| WinMD | `microsoft/go-winmd` による PE/ECMA-335 読み取り、TypeDef/Field/MethodDef/Property/Event/GenericParam の保守的インベントリ | custom attribute の完全な意味付け、全 table の全シンボル化、完全な layout |
| Header | Clang AST JSON provider と 386/amd64/arm64 target/profile 設定 | SDK header 全体の統合実行、macro 展開結果と inline の安全な Go 変換 |
| Type library | JSON interchange と provider interface | native TLB/OLB/DLL resource reader、Automation marshaling |
| IR | 安定 ID、provenance、型、関数、availability、ownership、status/backend | 公式 metadata の全属性を完全に復元すること |
| Go 出力 | fixture の型/union/bit field/flexible-array/COM/C bridge と、公式 Win32 metadata の証明可能な単純 P/Invoke subset を namespace package へ決定的に生成 | 公式 WinMD 全型・全関数・全 namespace の callable 出力 |
| Runtime | System32 限定 lazy loader、整数/ポインター call、HRESULT/NTSTATUS、UTF-16、IUnknown、BSTR、CoTaskMem、HSTRING、IInspectable、activation | 特殊 ABI 全般、COM server、SAFEARRAY/VARIANT、WinRT async/event/generic IID |
| WDK | 代表型の type-only 投影、kernel-only 関数の明示分類 | 標準 Go runtime でのカーネルドライバー生成・実行 |

## 「完全網羅」と百分率

このプロジェクトでいう完全網羅は「すべてを Pure Go で呼べる」という意味ではない。公式入力から認識した全シンボルについて、安定 ID、出自、architecture/profile、生成状態または非生成理由、ABI 検証状態が機械可読であることを意味する。

次の指標を混同しない。

- source ingestion coverage: provider が認識した入力に対するインベントリ化率
- projection accounting coverage: 現在の inventory 内で有効な status が付いた率
- Go source generation coverage: Go source を出力した率
- Pure Go callable coverage: Pure Go backend で実際に呼べる率
- assembly callable coverage: 検証済み assembly trampoline で呼べる率
- bridge callable coverage: C bridge で呼べる率
- ABI verified coverage: ABI oracle または同等の検証を通った率
- runtime smoke-tested coverage: Windows 上で非破壊 smoke test を実行した率

現在の fixture に対して `projection accounting coverage = 100%` となることはあり得るが、それは Windows SDK 全体の 100% ではない。現行の source-ingestion 指標も、すでに inventory に入ったレコードを分母としており、WinMD 内の全 table 行を分母にした完全性証明ではない。公式全 SDK の網羅を示すものとして「100%」を掲げてはならない。

## ステータス分類

すべての inventory symbol は status と非空の reason を持つ。`unclassified` は正規化時に無効であり、coverage gate でも失敗対象である。

| Status | 意味 |
|---|---|
| `generated-purego` | 対応 runtime で整数/ポインター ABI として呼出可能、または安全な Pure Go 投影 |
| `generated-assembly` | architecture 別に検証済み assembly trampoline を使用 |
| `generated-cgo-bridge` | 生成された C ABI bridge が必要 |
| `generated-type-only` | 型・定数・descriptor のみ。呼出可能とは主張しない |
| `generated-manual-override` | 根拠、SDK 範囲、テスト、削除条件を持つ override 適用済み |
| `unsupported-go-abi` | Go 側の利用可能な backend では ABI を証明できない |
| `unsupported-projection` | 入力は inventory 化したが、安全な言語投影が未実装 |
| `kernel-mode-only` | 通常のユーザーモード Go process から呼べないカーネル API |
| `missing-upstream-metadata` | 必要な公式 metadata が欠落 |
| `external-sdk-not-installed` | optional な外部 SDK / local SDK が未導入 |
| `undocumented-out-of-scope` | 未文書 API など、推測を避けるため対象外 |
| `license-restricted` | 再配布または処理がライセンス上制限される |
| `source-parse-error` | 入力破損または parser が安全に解釈できない |

`unsupported-projection` は現行 IR が持つ追加の保守的状態であり、対応済みではない。`generated-*` であっても、backend と build tag を見ずに callability を推測してはならない。

## 対象 architecture と profile

設定上の対象は `windows/386`、`windows/amd64`、`windows/arm64` である。profile は `windows-desktop`、`windows-appcontainer`、`windows-wdk` を区別する。現時点では fixture と一部の cross-compile/layout test がこの三 architecture を通す設計だが、ARM64 の実機実行や全 SDK の ABI 実行検証まで完了したことを意味しない。ARM64EC は通常の Go `arm64` と同一視せず、現在は対象外である。

## 明示的な非保証

- undocumented NT API の signature を推測しない。
- kernel-only symbol を通常の Go process から呼出可能として公開しない。
- float、vector、varargs、aggregate-by-value、特殊 callback を一律の `uintptr` 列へ落とさない。
- packed struct や Go で表現不能な alignment を、見た目だけ同じ通常 struct として出力しない。
- `CGO_ENABLED=0` で bridge API が使えるように見せる stub を提供しない。
- import しただけで DLL や全 API を eager load しない。
- raw API が便利な Go `error`、string、slice、resource ownership を自動的に与えるとは限らない。

## セキュリティ境界

system DLL は固定 metadata 由来の basename に限定し、`LOAD_LIBRARY_SEARCH_SYSTEM32` で lazy load する。外部入力を DLL 名や export 名として無検証で渡さない。NUL 終端 Win32 string は埋め込み NUL を拒否し、BSTR/HSTRING は長さ付きの意味を保持する。Go pointer の lifetime は `runtime.KeepAlive` と明示 ownership で管理し、native 側が call 後も保持する pointer や callback は、専用 allocation/registry/bridge が完成するまで安全と分類しない。
