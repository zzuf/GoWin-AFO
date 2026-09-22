# アーキテクチャ

## 全体像

中心は「公式入力から直接 Go を出す」のではなく、入力ごとの差を provenance 付きの正規化 IR に集約し、同じ IR から出力、網羅性、検証を導く構成である。

```text
sources.lock.json
       |
       v
winapisource -> sources/cache/<source-id> -> provider raw dump
                                               |
                                               v
                                      normalized Inventory
                                      (stable ID + reason)
                                               |
                                  typed override + re-normalize
                                               |
                         +---------------------+------------------+
                         |                     |                  |
                         v                     v                  v
                    Go emitter            C bridge          coverage JSON
                         |                                        |
                         v                                        v
              bindings/generated                     CI accounting gates
                         |
                         v
          runtime/winabi, runtime/com, runtime/winrt
```

fixture は全 feature を通る小規模縦切りであり、`winapigen generate --fixture` で明示的に選ぶ。加えて公式 Win32/WDK WinMD の full-source pass は、P/Invoke のうち固定整数だけで型解決できる保守的 subset を Pure Go raw wrapper まで流す。custom attribute が未投影の現段階では、pointer、wide scalar、特殊 ABI、WinRT/COM method を公式 callable subset へ入れない。型 layout 等が未解決の symbol は emitter へ流さず、inventory/status/reason だけを公開する。CLI は scope 未指定を拒否し、full tree を暗黙の fixture generation で置換しない。

## レイヤー

### Source manager

`generator/internal/source` は lock file を読み、固定 URL または Windows SDK install から artifact を staging directory へ取得する。artifact 全体の SHA-256 を確認した後、lock に列挙されたファイルだけを抽出し、`sources/cache/<source-id>` へ置換する。検証時には実際に読む展開済みファイルを hash 済み archive entry と再比較する。現行 `generate --all` の4 sourceはすべて required で、欠落・改変・hash mismatchは失敗する。

### Provider

`generator/internal/metadata.Provider` が source type ごとの入口である。Win32、WDK、WinRT は `microsoft/go-winmd` を使う WinMD provider、header は Clang AST JSON provider、type library は JSON interchange provider を持つ。Windows App SDK と外部 SDK は interface と source lock 上の境界があるが、推移 NuGet の API payload を展開する provider は未完成である。

provider は `Source`、`Symbol`、raw 表現、diagnostic を返す。parser failure を黙殺せず、可能な範囲で `source-parse-error` または diagnostic に残す。

### 正規化 IR

`generator/internal/model` の `Inventory` が中心データである。各 `Symbol` は source、namespace、kind、native name、architecture、ABI profile、canonical signature、generic arity、status/backend と理由、provenance を持つ。

安定 ID は次の順序の文字列を NUL で区切り、SHA-256 にしたものである。

```text
source ID
namespace
symbol kind
native name
architecture
canonical signature
generic arity
ABI profile
```

正規化は whitespace を整え、ID collision を拒否し、source、symbol、diagnostic、provenance を安定順序へ並べる。最後に timestamp や absolute path を含まない inventory JSON の SHA-256 を manifest hash とする。

### Override

override は IR へ任意コードを注入する仕組みではない。対象 symbol ID と source ID を指定し、現在値に一致すべき `before` と、変更する `after` を JSON object で記録する。SDK version 範囲、理由、header/ABI probe/upstream issue の根拠、回帰テスト、上流修正後の削除条件が必須である。`before` が一致しない stale override は生成を止める。

### Projection と emitter

projection は function signature を capability matrix に分類する。型 emitter は通常型、architecture 別型、union storage/accessor、bit-field getter/setter、flexible-array header/view、callback address、COM vtable を分割して出力する。公式 function emitter は現在、pointer、wide scalar by-value、未解決 named type を含まない固定整数 raw P/Invoke だけを対象とする。availability attribute の完全投影前なので、公式 wrapper は optional export として lazy resolve/availability check を生成する。

namespace は個別 package に分け、1 package の import で全 Windows API がコンパイルされる構造を避ける。汎用 emitter の現在の出力先は `bindings/generated/<正規化namespace>`、bridge は `bridge/generated` である。`bindings/win32`、`bindings/wdk`、`bindings/winrt` の13ファイルは runtime smoke/layout test 用のレビュー済み参照 slice で、`generator/internal/slice/templates` から専用 stage で再構築し manifest で hash 検証する。source pin 変更時はレビューを要求し、import は root go.mod 由来である。汎用 emitter の生成済み全 SDK surface とは数えない。

### ABI runtime

`runtime/winabi` は DLL 解決、整数/ポインター call、last error、HRESULT/NTSTATUS、GUID、pointer width、UTF-16 境界を隔離する。`runtime/com` と `runtime/winrt` はこの層の上で native ownership と apartment/thread affinity を明示する。raw generated package は native return と pointer を保ち、ergonomic helper は runtime 側へ分離する。

## 決定性と更新の原子性

- map iteration に依存せず、source、symbol、file、import を明示的に sort する。
- generated header は generator version、source ID/version、manifest hash を持ち、日時とローカル絶対 path を持たない。
- Go source は出力前に `go/format` を通す。
- `bindings/generated`、`bridge/generated`、reviewed slice、inventory/coverage、要求された raw dump を workspace 内の staging tree で全て完成させる。
- 所有 marker `.winapigen.json` がない tree、marker にない追加 file、symlink、generated header のない手書き file は置換しない。
- 全出力先を preflight してから順次 rename し、通常 error は既存出力を戻す。rollback 失敗時は backup を削除せず場所を報告する。
- inventory と coverage report は stage 内で同期する。複数 rename の間の process crash に対する永続 journal は未実装であり、一つの OS atomic operation と同等とは主張しない。

## エラーとカバレッジの設計

`unclassified` は IR validation で受理されない。非生成 symbol にも status と reason が必要である。coverage は ingestion、accounting、source generation、backend callability、ABI verification、runtime execution を別々に数える。CI の最重要条件は projection accounting 100%、unclassified 0、ABI mismatch 0、generation drift 0 であり、各分母が何かを併記する。

ただし、現在の full-source inventory は WinMD の全 table/custom attribute を網羅していない。このため現段階の accounting 100% は「現在 inventory 化できた集合」についての性質であり、SDK 全シンボル完全性の証明ではない。

## Trust boundary と安全策

入力 artifact は hash 検証前に信頼しない。download は HTTPS、size 上限は 1 GiB、zip entry は lock に列挙されたものだけとし、absolute path と `..` を拒否する。生成 path も staging root 外への escape を拒否する。

runtime では system DLL basename と ASCII export name を検証する。current directory や一般の `PATH` を検索せず、System32 loader を使う。app-local DLL の trusted absolute path policy は未実装なので、現時点で外部入力による app-local load は提供しない。

`unsafe` は ABI runtime、generated raw layer、検証へ閉じ込める。pointer、callback、COM reference、HSTRING/BSTR/CoTaskMem は lifetime を隠さず、finalizer を唯一の解放手段にしない。

## 依存方向

依存は原則として `bindings -> runtime/winabi`、`runtime/com -> runtime/winabi`、`runtime/winrt -> runtime/com + runtime/winabi` の一方向である。foundation 型を共有 package に置き、namespace 間で同じ基礎型を再定義しない。generator は generated bindings を import せず、IR から出力する。この分離により、generator は Linux/macOS でも build/test でき、Windows ABI 呼出だけが build tag で Windows に限定される。
