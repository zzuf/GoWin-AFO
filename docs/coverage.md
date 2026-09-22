# 網羅性と accounting

## 何を数えるか

このプロジェクトでは「取得した」「Go source を生成した」「呼べる」「ABI を検証した」「実行した」を別の事実として数える。単独の `100%` 表記は禁止し、必ず metric 名、分子、分母を併記する。

| Metric | 分子 | 分母 |
|---|---|---|
| source ingestion coverage | provider が inventory 化した入力レコード | provider が今回認識した入力レコード |
| projection accounting coverage | 有効な status と reason を持つ symbol | inventory symbol |
| Go source generation coverage | `generated-*` status の symbol | inventory symbol |
| Pure Go callable coverage | `generated-purego` | inventory symbol |
| assembly callable coverage | `generated-assembly` | inventory symbol |
| bridge callable coverage | `generated-cgo-bridge` かつ対象 ABI oracle で検証済み | inventory symbol |
| ABI verified coverage | 対象 architecture の oracle と一致した symbol | inventory symbol |
| runtime smoke-tested coverage | Windows 上で非破壊実行した symbol | inventory symbol |

現在の WinMD provider は全 ECMA-335 table/custom attribute を完全な native symbol へ復元していない。このため source ingestion coverage は「provider が認識した集合」についての指標であり、Windows SDK の真の全シンボル数に対する完成証明ではない。

## Report

`coverage/latest.json` が機械可読 report、`coverage/latest.md` が同じ内容の人間向け表示である。JSON は generator version と manifest hash のほか、source、namespace、kind、architecture、backend、status、skip reason ごとの件数を持つ。ABI mismatch、manual override、ABI 未検証数、runtime 実行数も別フィールドである。

```text
go run ./cmd/winapicoverage report
go run ./cmd/winapicoverage report --format json
go run ./cmd/winapicoverage diff coverage/baseline.json coverage/latest.json
go run ./cmd/winapicoverage check --fail-unclassified
go run ./cmd/winapicoverage check --fail-regression
```

レポートの値は checked-in JSON を参照すること。README や release note に数値を転記する場合は manifest hash も併記し、別の source/profile を合算しない。通常の fixture generation は 23 個のテスト symbol を使うが、この fixture を Microsoft 公式 API の網羅率へ混ぜてはならない。`generate --all` では inventory に fixture の provenance を保持しつつ、公式 report の分母から `project-fixture` を自動除外し、`scope=official-windows-sources` と `excludedBySource` を記録する。fixture 単独 report は `scope=vertical-slice-fixture` として別に読む。

## Status accounting

全 symbol は生成 status または明示的な非生成 status と非空の reason を持つ。`unclassified` は有効 status ではなく、IR validation と CI gate の両方で失敗する。`source-parse-error`、`missing-upstream-metadata`、`kernel-mode-only`、`unsupported-go-abi` も accounting 済みではあるが、対応済み・呼出可能ではない。

`generated-type-only` は layout/constant/interface descriptor を出力できたことだけを表す。function を callable として数えない。`generated-cgo-bridge` は `windows && cgo` の実体が生成され、対象 compiler ABI が検証された場合だけ bridge callable に数える。manual override は callability backend ではないため、適用前の projection status を保持し、独立した `manualOverrides` 件数で数える。`generated-manual-override` は override 自体が一つの生成 artifact/backend を構成する将来用途の有効 status として予約し、単なる metadata correction へ自動付与しない。

## CI gate

最重要 gate は次の四つである。

- projection accounting coverage = inventory の 100%
- unclassified symbols = 0
- ABI mismatches = 0
- generation drift = 0

`--fail-regression` は `coverage/baseline.json` と比較し、projection accounting、Go source generation、Pure Go、assembly、ABI 検証済み C bridge、ABI verification、runtime smoke の各 coverage 率の低下、総 inventory 数の低下、ABI mismatch の増加を拒否する。判定には丸めた表示 percent ではなく整数の分数を使う。`diff` は source、namespace、kind、architecture、status、backend、skip reason と検証件数の増減も JSON で公開する。

baseline は gate を通すために都合よく下げない。SDK update pull request でのみ、source lock、生成差分、coverage delta、ABI evidence、既知の未検証増加を同時に review して更新する。source ID や profile を変えて以前の分母を見えなくする変更も regression review の対象である。

## ABI oracle との関係

`tools/abi-oracle` の fixture result は C++ header probe の独立した証拠である。`go run ./cmd/winapiverify abi --all` は checked-in x86/x64 result の provenance と、対応可能な IR symbol/layout fact を照合して `coverage/abi-latest.json` を作る。対応する IR fact がない probe は unmatched のままにし、main coverage inventory の `abiVerified` を一括で増やさない。ARM64 の cross-compile は signature/layout 式がコンパイルできた証拠であり、ARM64 runtime 実行数には含めない。

## Report を読むときの注意

- Pure Go 率の分母には型、定数、kernel-only symbol も含まれる。function だけの率が必要なら kind filter を別途表示する。
- architecture 共通 symbol が architecture 別 ID を持つ場合、各 target を別件として数える。
- Office 等の type library、WebView2 等の外部 SDK は Windows OS source と別 report にする。
- duplicate metadata は provenance を保持する。SDK/WDK の重複を消して source ingestion 数を小さく見せない。
- deprecated/dangerous symbol は削除せず availability と status reason に残す。
