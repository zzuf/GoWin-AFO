# Contributing

このリポジトリへの変更は、生成物の量より ABI の正しさ、再現性、全 symbol の説明可能性を優先する。最初に `AGENTS.md`、`docs/architecture.md`、変更領域の文書を読むこと。

## 開発環境

基本の generator/test は Go 1.26 以降で Linux、macOS、Windows 上から実行できる。再現用 toolchain は `go.mod` / `go.work` の Go 1.26.8 に固定する。公式 source の取得には HTTPS access が必要である。WinRT source と ABI probe には対象 Windows SDK、WDK header probe には対象 WDK、native probe には MSVC または clang-cl が必要である。

```text
go run ./cmd/winapisource fetch
go run ./cmd/winapisource verify
go test ./...
```

SDK/NuGet/header を repository へ無断で vendor しない。license と再配布可否は `docs/licensing.md` を確認する。

## 変更する場所

generated header に `Code generated ... DO NOT EDIT.` とある file を直接編集しない。修正先は次のいずれかである。

- parser/provider: 公式入力の読み取り不足
- normalized IR/projection: 型、ABI、ownership、availability、status 判定
- emitter: Go/C/assembly source の構成
- typed override: 証拠がある upstream metadata defect
- runtime: 共通 ABI、COM、WinRT、ergonomic helper
- oracle/test: header による ABI 証明

namespace package を細かく保ち、foundation 型をコピーしない。raw ABI layer と ergonomic layer を混ぜない。通常 application package に `unsafe` を広げない。

## ABI 変更

Windows は LLP64 である。C `int`/`LONG`/`DWORD` を Go `int` に置き換えず、pointer-sized type だけを architecture 別にする。calling convention、float/vector、aggregate-by-value、varargs、callback を推測しない。利用可能 backend で証明できない function は callable wrapper を作らず reason 付き status にする。

type layout または function signature を変更するときは `tools/abi-oracle` manifest と regression test を追加する。manifest の native expression は実行される C++ source なので review 済み固定 text だけを使い、外部入力を挿入しない。

## Override checklist

新しい override は schema を満たすだけでは不十分で、次をすべて含める。

- 一意な override ID、source ID、symbol ID
- 対象 SDK version range
- 現在値と一致しなければ失敗する `before`
- 最小限の `after`
- なぜ metadata が誤り、Go ABI にどう影響するか
- 公式 header、ABI probe、または upstream issue の evidence
- 自動 regression test
- 上流修正後の具体的 removal condition

期限・根拠不明の override、symbol ID の wildcard、大量 symbol の一括上書きは受理しない。

## Test と generation

変更範囲に応じて最低限次を実行する。

```text
gofmt -w <changed-go-files>
go test ./...
go vet ./...
go run ./cmd/winapigen generate --all
go run ./cmd/winapicoverage check --fail-unclassified --fail-regression
go run ./cmd/winapigen generate --all
```

二回目の生成後に diff があってはならない。generated tree は temporary sibling で完成・検証後に置換されるべきで、失敗時に正常な既存 tree を壊さない。

Windows target は `386`、`amd64`、`arm64` を cross-compile する。bridge change は `CGO_ENABLED=0` で Pure Go packages が build できることと、`CGO_ENABLED=1` で該当 bridge が build できることを別々に確認する。Windows runtime test は非破壊 API だけを使う。

## Pull request

説明には source/version/hash、影響する symbol/status/backend、生成 package、coverage before/after、ABI evidence、実行した target/test、known limitation を記載する。生成された大量差分を parser/IR/emitter change と分離せず提出しない。

SDK update は `docs/sdk-updates.md` に従う。lock、generated diff、coverage diff、ABI diff、breaking API review、license/NOTICE をまとめ、workflow から main へ自動 merge しない。
