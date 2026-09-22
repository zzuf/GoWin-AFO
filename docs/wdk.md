# WDK

## 方針

WDK metadata は Windows SDK metadata と別 source として扱う。現在固定している入力は `Microsoft.Windows.WDK.Win32Metadata` `0.13.25-experimental`、対象 Windows SDK は `10.0.26100.0` である。

標準 Go runtime で Windows kernel driver を構築・実行できるとは主張しない。カーネル用の型や定数が Go で表現できても、`ntoskrnl.exe`、`.sys`、HAL/NDIS 等の kernel export を通常 process から呼ぶ wrapper は生成しない。

## 分類軸

WDK symbol は最終的に次を区別する必要がある。

- 通常の user-mode process から呼出可能
- UMDF runtime/context で利用可能
- user-mode code にも有用な type/constant のみ
- kernel-mode-only
- 標準 Go runtime では実行不能
- upstream metadata 不完全
- header/ABI probe による補完が必要

現在の full-source provider が自動判定できるのは限定的である。P/Invoke target が `ntoskrnl`、HAL、NDIS、`.sys` と認識できる場合は `kernel-mode-only` にする。それ以外の WDK type/method は、callability と layout が証明されるまで原則 `unsupported-projection` または type-only inventory である。UMDF と user-mode の精密分類は未完成である。

## 現在の縦切り

`bindings/wdk/nt` は次の代表例を持つ。

| Symbol | 現在の扱い | 理由 |
|---|---|---|
| `UNICODE_STRING` | `generated-type-only` | counted UTF-16 layout。buffer ownership は外部 |
| `OBJECT_ATTRIBUTES` | `generated-type-only` | user-mode NT consumer にも有用な layout。kernel callability は付与しない |
| `DRIVER_OBJECT` / `PDRIVER_OBJECT` | opaque pointer / type-only | kernel object を user-mode allocatable struct にしない |
| `IoCreateDevice` | `kernel-mode-only` | kernel driver execution environment が必要 |
| `IoDeleteDevice` | `kernel-mode-only` | kernel driver execution environment が必要 |

`kernel-mode-only` function に call wrapper はない。type-only symbol を import できることは、対応する kernel routine を呼べるという意味ではない。

## SDK/WDK 重複

source ID は stable ID の一部なので、SDK と WDK の provenance を失わない。現行 generator は namespace、kind、native name、canonical signature、architecture が一致する symbol を検出し、後続 origin に `duplicateProjectionOf` annotation を付ける。

これは単純な重複生成を避けるための基礎であって、完全な semantic merge ではない。typedef chain、custom attribute、layout、availability が異なる場合にどちらを canonical projection とするか、foundation package へどう共有するかはまだ全面実装されていない。source-specific coverage は両方を追跡する。

## Layout と architecture

WDK type も Windows LLP64 と 386/amd64/arm64 の architecture 別 layout に従う。pointer-sized field、anonymous union、bit field、pack、flexible array は header/ABI oracle で証明してから direct Go struct にする。特に kernel object は OS version ごとの内部 layout を推測しない。公開されていない、または user-mode で意味のない body は opaque type に留める。

WDK header 用 Clang profile は `_KERNEL_MODE=1` と Windows target triple を設定できるが、WDK include path、NTDDI/WINVER matrix、SAL、全 layout probe を full generation pipeline に統合する作業は未完である。

## Function callability

WDK 由来というだけで kernel-only とは限らず、逆に DLL 名だけで user-mode safety が確定するわけでもない。callable へ昇格するには少なくとも次を確認する。

- export が user-mode DLL に存在する
- 対象 profile と minimum OS version
- calling convention と architecture ABI
- parameter が user-mode address space で有効
- handle/object ownership と IRQL/context 制約
- administrator、driver install、service、system configuration 変更を必要としないか
- metadata と WDK header の signature が一致するか

kernel export は `purego-syscall` 候補にしない。UMDF API も framework initialization と lifetime が必要なため、単純な DLL call wrapper だけで対応済みとしない。

## NTSTATUS と error

WDK/NT API の raw result は `NTSTATUS int32` として保持する。`NT_SUCCESS(status)` と同じ signed comparison を使い、severity/facility/code を参照できる。Win32 error へ変換する便利 helper を将来追加する場合も、元の NTSTATUS を失わない。

## テストと安全性

通常 CI では driver install、service 作成、registry 変更、kernel call を行わない。type layout の Go cross-compile と C/C++ cross-compile、user-mode で非破壊な API だけを検証対象とする。kernel-only symbol は inventory/status/reason の存在をテストし、実行しないこと自体を安全要件とする。

pointer が kernel address を指すと仮定した dereference helper、未文書 struct layout、推測した NT signature は生成しない。metadata 不完全なら `missing-upstream-metadata`、header parse failure なら `source-parse-error`、Go ABI が証明不能なら `unsupported-go-abi` として残す。

## 未達事項

- WDK 全 TypeDef/Field/MethodDef 以外を含む完全 inventory
- custom attribute と header 条件を使った user-mode/UMDF/kernel-mode 分類
- SDK/WDK 型の semantic deduplication
- WDK 全公開型の ABI oracle 検証
- user-mode WDK API の汎用 emitter
- WDK header の 386/amd64/arm64、WINVER/NTDDI profile matrix

これらが完了するまでは、WDK 全体をサポート済み、または WDK 網羅率 100% と表示しない。
