# SDK 更新

## 原則

source version と SHA-256 は `sources.lock.json` だけで変更する。floating `latest`、取得日時、開発機固有 path を正本にしない。SDK/WDK/WinRT/Windows App SDK の更新は自動で main へ merge せず、生成、coverage、ABI、license、breaking change を pull request で review する。

## 更新候補の確認

```text
go run ./cmd/winapisource update --dry-run
```

この command は候補を報告するだけで lock を変更しない。NuGet meta package の推移依存を更新する場合は、実際に API payload を持つ全 package を個別 source ID/version/hash として固定する。Windows SDK local provider は runner に対象 version が導入済みかも確認する。

NuGet の配列順には依存せず、数値、4桁目の revision、prerelease label を比較する。stable pin には stable 版だけを提案し、prerelease pin は stable/prerelease の両方を候補にする。現在より古い版は提案せず、空・不正・4 MiB 超の応答は失敗させる。NuGet content index には unlisted 版も含まれるため、候補発見はサポート状態や採用可否の承認を意味しない。[NuGet versioning](https://learn.microsoft.com/en-us/nuget/concepts/package-versioning)、[content index](https://learn.microsoft.com/en-us/nuget/api/package-base-address-resource) を参照。

`.github/workflows/sdk-update.yml` は週次または手動で candidate discovery JSON を作るが、候補 lock を自動適用しない。その後の生成 patch、coverage delta、binding/runtime の file-level change、release-note draft は **現在の pin の再現性**についての artifact である。候補 version の hash を固定して生成・ABI 差分を作る段階と、symbol-level compatibility 判定は未実装である（`SDKUP-001`）。候補を採用する pull request では、以下の手動手順を実行する。

## 手動更新手順

1. 公式 package feed / SDK installer で version と公開 license を確認する。
2. artifact 全体を取得し SHA-256 を計算する。hash を推測・コピーしない。
3. source ID、type、package、version、retrieval、SHA-256、license identifier、architectures、SDK version、file list、dependency を lock へ反映する。
4. redistribution が許可されない SDK/header/NuGet payload を Git に追加しない。cache は ignore 対象のままにする。
5. `fetch` と `verify` を clean environment で実行する。
6. `generate --all` を実行し、inventory、generated Go/C、coverage を review する。
7. `winapicoverage diff` と regression gate を実行する。new symbol に `unclassified` や空 reason がないことを確認する。
8. x86/x64 の ABI probe を実行し、ARM64 を cross-compile する。ARM64 hardware がある場合だけ runtime result も更新する。
9. generated package を 386/amd64/arm64、cgo off/on の該当構成で compile する。
10. 非破壊 smoke test を Windows で実行する。
11. 同じ入力で二回生成し、二回目が clean であることを確認する。
12. source、coverage、ABI、breaking changes、known limitation、license/NOTICE、release note を一つの review にまとめる。

## Coverage regression

new API が増えると、正しく分類していても ABI verified percentage が下がる場合がある。baseline を機械的に下げず、未検証 symbol を kind/source/architecture/backend ごとに示し、probe 追加または明示的な未検証理由を決める。

projection accounting は常に 100% を保つ。parser が新 table/signature を解釈できない場合も黙って捨てず、`source-parse-error` または `unsupported-projection` と diagnostic を残す。source ingestion の分母自体が変わった場合は、provider が認識する table/attribute の差も説明する。

## ABI baseline 更新

ABI result は同じ manifest と target の native compiler 実測から作る。手入力で size/offset/GUID を合わせない。差分が公式 SDK の意図した変更なら、対象 source version と architecture を変えた新 baseline として追加し、古い SDK baseline を上書きしない。

function pointer assertion が失敗した場合は generated signature を callable に保ったまま expected type を緩めない。metadata、header、calling convention、target macro を調べ、projection/override/backend classification を修正する。

## Override の更新

SDK 更新で override の `before` が一致しなくなった場合、生成を止めるのが正しい。上流修正済みなら removal condition と evidence を確認して override と regression test を削除する。未修正なら version range と `before/after` を新 SDK に合わせ、header または ABI probe の根拠を追加する。根拠不明・無期限の override は追加しない。

## Release note

最低限、次を記載する。

- old/new source ID、version、SHA-256、SDK contract
- inventory symbol 数と source/kind/architecture ごとの差
- Pure Go/assembly/bridge/type-only/unsupported の増減
- ABI verified/unverified/mismatch と runtime execution の増減
- public Go API の追加・削除・rename・signature change
- required OS version、DLL、availability の変更
- manual override の追加/削除
- license/NOTICE の変更
- ARM64 等で compile-only の検証範囲

assessment workflow は release note を公開せず artifact に留める。review と明示的な release 操作を経るまで package/release を作成しない。
