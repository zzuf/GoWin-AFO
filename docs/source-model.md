# 入力 source model

## Lock file が正本

入力の正本は `sources.lock.json` である。ローカル cache や NuGet の latest version は正本ではない。各 source は少なくとも次を固定する。

- `id`: provenance と安定 ID の一部になる source 識別子
- `type`: provider の選択に使う source type
- `package` と `version`
- `retrieval`: 固定 NuGet URL または `windows-sdk://` locator
- artifact 全体の `sha256`
- `licenseIdentifier`
- `architectures` と `windowsSDKVersion`
- cache へ抽出する `files`
- `dependsOn`
- `required`: 欠落を fetch/verify failure にするか

取得日時は lock と generated artifact に記録しない。version と hash が同じなら同じ入力として扱う。

## 現在固定している source

| Source ID | 入力 | Version | SDK | Required |
|---|---|---:|---:|---:|
| `microsoft-win32metadata` | `Microsoft.Windows.SDK.Win32Metadata` | `71.0.26-preview` | `10.0.26100.0` | yes |
| `microsoft-wdkmetadata` | `Microsoft.Windows.WDK.Win32Metadata` | `0.13.25-experimental` | `10.0.26100.0` | yes |
| `windows-sdk-winrt` | SDK `UnionMetadata/Windows.winmd` | `10.0.26100.0` | `10.0.26100.0` | yes |
| `windows-app-sdk` | `Microsoft.WindowsAppSDK` meta package | `2.5.1` | `10.0.26100.0` target | yes |

Windows App SDK の lock は現在 meta package 本体だけを固定している。API を持つ推移 package 群の version/hash/provider 展開は未実装であり、インストール有無ではなく upstream 展開不足を示す `missing-upstream-metadata` sentinel として扱う。WinRT の `Windows.winmd` は固定した Windows SDK から取得する required source であり、欠落時は fetch/verify を失敗させる。

## 取得と検証

標準操作は次のとおりである。

```text
go run ./cmd/winapisource fetch
go run ./cmd/winapisource verify
go run ./cmd/winapisource list
go run ./cmd/winapisource update --dry-run
```

`fetch` は source ごとに temporary staging directory を作る。HTTPS artifact は 1 GiB を上限として保存し、artifact 全体の SHA-256 が一致してから lock に列挙した file だけを ZIP から抽出する。Windows SDK locator は `WindowsSdkDir`、次に標準の Windows Kits directory を参照する。cache の install は directory rename で行い、中途半端な cache を公開しない。

`verify` は artifact hash、cache の source ID/version/hash、必須 file の存在に加え、抽出済み file の byte 列を lock 済み archive entry または SDK artifact と比較する。required source の欠落・不一致は command 全体を失敗させる。optional source の不在は明示状態として返し、存在するかのように生成しない。

`update --dry-run` は NuGet version index を照会するだけで lock を変更しない。SDK update を main へ自動 merge する設計ではない。

## Provider contract

すべての provider は `Type()` と `Ingest(context, Request)` を実装し、次を返す。

- lock 情報から作った `Source`
- provenance 付きの `Symbol` 配列
- debug 用の raw 表現
- parser warning/error の `Diagnostic`

外部 SDK provider は install detection を追加し、type library provider は LIBID、coclass、interface、dispinterface、enum、record、alias、method/property/event、IID/CLSID/DISPID を独立 inventory として返す。

### WinMD provider

現在は `github.com/microsoft/go-winmd/winmd` を主 parser とする。PE/ECMA-335 metadata を開き、table count を raw dump に残し、`TypeDef`、その範囲内の `Field` と `MethodDef`、さらに `Property`、`Event`、`GenericParam` を symbol 化する。P/Invoke の `ImplMap` から DLL、entry point、calling convention、SetLastError を読み、COM/WinRT interface method には暫定 vtable index を付ける。signature parse failure と table row decode failure は、捨てずに安定した `source-parse-error` symbol と diagnostic にする。

これはまだ WinMD の完全な意味投影ではない。custom attribute の NativeArrayInfo、MemorySize、SupportedArchitecture、Deprecated、NativeTypedef、RetVal、FreeWith、associated enum、nullability、contract/threading/marshaling を全面的に解釈していない。`MemberRef` などは raw table count があることと symbol inventory 済みであることは同義ではない。

`generator/internal/ecma335` は upstream reader を置き換える parser ではなく、compressed integer、blob、coded index、method signature など、bounds check を強める補助実装である。

### Clang header provider

header provider は C/C++ declaration を正規表現で解析せず、Clang の `-Xclang -ast-dump=json` を読む。target triple は 386/amd64/arm64 を分け、desktop/appcontainer/WDK profile の define を分ける。現在は record、union、enum、typedef、function、constant と bit-field width の基本 inventory を作るが、ほとんどを `unsupported-projection` とし、安全な C semantic projection が完成するまで callable source を出さない。

macro token、preprocessor condition の完全な provenance、pack/layout dump、SAL、inline function の式 semantics は未統合である。debug AST dump からは Clang process address と machine-local path を除き、source location は basename/line/column だけを保持する。

### Type library と外部 SDK

現在の type library provider は native TLB loader ではなく、外部 reader が作る決定的 JSON interchange を検査する。Office などの product API は `typelib` source として OS coverage から分離し、初期状態では inventory-only である。

WebView2 等を追加する `ExternalSDKProvider` interface はあるが、具体 provider は未実装である。未導入 source を OS symbol として捏造せず、`external-sdk-not-installed` として集計する。

## Raw dump、IR dump、inspect

`winapigen generate --all --dump-raw` は provider raw dump を `coverage/raw/<source-id>.json` に保存する。正規化後の `coverage/inventory.json.gz` は決定的な gzip IR dump であり、`winapigen inspect` は qualified name、native name、stable ID でこれを検索する。

```text
go run ./cmd/winapigen inspect --symbol Windows.Win32.Foundation.HANDLE
go run ./cmd/winapigen inspect --native CreateFileW
go run ./cmd/winapigen inspect --id sha256:... --json
```

raw dump と IR は debug artifact であり、再配布条件を無視して upstream binary を repository へ vendor する仕組みではない。

## Provenance と重複

各 symbol は source ID、input file、metadata table/row または header location を持つ。SDK と WDK の同じ native declaration は、source ID を消して一つへ潰さない。現行実装は namespace、kind、name、signature、architecture が一致する重複を検出し、後続 symbol に `duplicateProjectionOf` を付ける。これは出自を保った重複検出であり、layout equivalence や優先 source を完全に解決する semantic deduplication ではない。

## Manual override

override JSON は schema version 1 とし、次の全項目を必須にする。

- override ID
- source ID
- SDK version の inclusive 範囲
- stable symbol ID
- 変更前と変更後
- 理由
- header、ABI probe、upstream issue の根拠
- 回帰テスト識別子
- upstream 修正後の削除条件

適用時は `before` を現在 IR と比較するため、SDK 更新で前提が変われば黙って上書きせず失敗する。適用後は元の projection status/backend を保持し、`manualOverride=true` を独立集計する。すでに callable な function の ABI 本体を override が変更する場合は capability 判定を迂回できないよう拒否する。

## 入力セキュリティ

lock にない filename は抽出しない。absolute path、path traversal、unsafe archive entry、oversize artifact、hash mismatch を拒否する。source file の path は generated header へローカル絶対 path として埋め込まない。ライセンス条件が不明な SDK/header/NuGet 内容を cache 外の version control へ自動追加しない。
