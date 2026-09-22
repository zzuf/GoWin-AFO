# 検証

## 証拠の層

検証結果は次の順に強くなるが、上位が下位を暗黙に置き換えるわけではない。

1. parser/normalizer unit test
2. generator golden test と二回生成の同一性
3. Go source の 386/amd64/arm64 cross-compile
4. C/C++ header に対する compile-time ABI assertion
5. C/C++ probe 実行結果と Go layout/value の比較
6. Windows 上の非破壊 runtime smoke test

Go code がコンパイルできるだけでは Windows ABI compatible としない。特に aggregate-by-value、float/vector、calling convention、packed layout、bit field、callback、COM vtable は native compiler の証拠を要求する。

## Go test

通常の確認は次である。

```text
go test ./...
go vet ./...
gofmt -l .
```

CI は Linux、macOS、Windows で generator と oracle を build/test し、`CGO_ENABLED=0` で `windows/386`、`windows/amd64`、`windows/arm64` の全 package と test binary を cross-compile する。Windows x86/x64 runner では raw Kernel32、COM、WinRT の非破壊 smoke test を実行する。

ECMA-335 parser は compressed integer と signature parser の fuzz target を持つ。通常 CI は各 target を短時間実行する smoke であり、長時間 corpus fuzzing の代用ではない。

## ABI oracle

`tools/abi-oracle` は review 済み JSON manifest から決定的な C++17 probe を生成する。

```text
go run ./tools/abi-oracle validate --manifest tools/abi-oracle/testdata/probe-manifest.json
go run ./tools/abi-oracle generate --manifest tools/abi-oracle/testdata/probe-manifest.json --architecture amd64 --out probe.cpp
go run ./tools/abi-oracle validate --result actual.json
go run ./tools/abi-oracle compare --expected tools/abi-oracle/testdata/windows-sdk-10.0.26100-amd64.json --actual actual.json --out diff.json
go run ./cmd/winapiverify abi --all
```

manifest/result の JSON Schema は `tools/abi-oracle/schema` にある。manifest の probe ID は安定かつ一意でなければならない。include path、define、SDK version、profile、target architecture、native expression を review する。native expression は C++ code なので、外部入力をそのまま manifest に入れない。

生成された probe は次を出力する。

- `sizeof`、`alignof`、`offsetof`
- struct/union の kind、size、alignment
- enum/macro/constant value
- GUID/IID/CLSID の canonical value
- bit-field の memory-order mask、bit offset、bit width
- function pointer の `std::is_same` compile-time assertion と calling convention label
- C-style COM vtable の method index
- target/preprocessor condition

wide integer は JSON number の精度問題を避けるため decimal string とする。probe header は SDK version と canonical manifest SHA-256 を持つが、生成日時や absolute path を持たない。結果は compiler identity を provenance として保存する。比較時は MSVC と clang-cl の同じ ABI facts を比較できるよう compiler identity だけを除外し、SDK/profile/architecture/hash と全 probe value は比較する。

## 現在の fixture evidence

Windows SDK `10.0.26100.0` の fixture は MSVC 19.44 で x86/x64 probe を実行して作成した checked-in JSON を持つ。対象は 5 records/unions、2 constants、`IID_IUnknown`、1 bit field、2 function pointer types、IUnknown の 3 vtable slots、2 conditions である。これは小規模縦切りの証拠であって、SDK 全体の ABI verification ではない。

ARM64 probe は target compiler で cross-compile し、target guard と function type assertion を確認する。x64 runner では実行しないため ARM64 result JSON や runtime verified 数を捏造しない。ARM64 hardware runner を追加したときだけ同 target の実行 baseline を作る。ARM64EC は独立 target status であり、Go `arm64` と同一視しない。

## Go layout との比較

oracle result を「ABI verified」と inventory に反映するには、probe ID を IR symbol ID/field ID に対応付け、architecture/profile/sdkVersion/manifest hash が一致することを確認する。Go test は `unsafe.Sizeof`、`unsafe.Alignof`、`unsafe.Offsetof`、generated accessor の bit mask、GUID、constant、vtable index を JSON と比較する。

oracle comparator は実行した C++ result と checked-in baseline の機械可読差分を作る。`winapiverify` はさらに、source lock、生成 slice manifest と artifact hash、probe provenance を検証し、record layout、function type 等で対応可能な fact を normalized IR symbol へ照合して `coverage/abi-latest.json` を出力する。対応する IR layout/symbol がない probe fact は `unmatchedFacts` に残し、全 generated Go symbol を検証済みにしない。ARM64 は compile-only、ARM64EC は unsupported と明示する。

`winapiverify abi --all` の `--all` は vertical-slice fixture に checked-in された全 ABI evidence target を意味する。Windows SDK 全 namespace や全 inventory symbol の検証ではない。また照合対象は normalized IR の期待値であり、生成 Go struct を `unsafe.Sizeof/Offsetof` で全件実測したことや、全 native call boundary を実行したことを意味しない。これらは別の Go layout test/runtime smoke evidence として追加する必要がある。

## Runtime smoke

通常 CI で実行するのは process/thread ID、performance counter、system time、VirtualAlloc/VirtualFree、optional export availability、BSTR/HSTRING round trip、CoTaskMem、COM/WinRT apartment、既知 WinRT activation factory など非破壊操作だけである。

管理者権限、driver install、service 作成、registry 書込み、system setting 変更は通常 CI で行わない。registry を追加する場合は read-only operation と既知 key に限定する。optional API は availability check 後だけ呼ぶ。

## Failure の扱い

ABI mismatch は expected/actual/path を持つ JSON diff と、生成 probe/compiler output を artifact として保存する。SDK 更新で layout、GUID、signature、DLL mapping が変わった場合、expected JSON を先に書き換えて gate を回避せず、公式 header/metadata の差分、影響する Go symbol、override の要否を確認する。

cross-compile だけ成功、runtime runner 不在、optional SDK 不在は別々に記録する。検証できなかったものを成功扱いにせず、`ABIUnverified`、`external-sdk-not-installed`、または適切な非 callable status のまま残す。
