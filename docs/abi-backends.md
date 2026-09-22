# ABI backend

## Backend は callability の証拠である

function symbol は status だけでなく backend と判定理由を持つ。signature を機械的な `uintptr` 列へ変換して「対応済み」にしない。backend が実装・build・ABI 検証されていない場合、raw wrapper を出力せず `unsupported-go-abi` または `unsupported-projection` とする。

| Backend | 用途 | 現在の状態 |
|---|---|---|
| `purego-syscall` | integer、pointer、raw callback address の platform call | 小規模 Win32/COM/WinRT 縦切りで実装 |
| `purego-syscall-float` | float register ABI を扱う候補区分 | 列挙値のみ。call 実装・検証は未完成 |
| `assembly-trampoline` | architecture 別の float/special ABI trampoline | emitter と検証済み trampoline は未実装 |
| `cgo-bridge` | aggregate-by-value、vector、varargs、compiler-owned ABI | fixture の `FILETIME` 値渡し bridge だけ実装 |
| `type-only` | layout/descriptor のみで call boundary なし | 縦切りで実装 |
| `unsupported` | 利用可能 backend では証明不能 | wrapper を出力しない |

fixture の浮動小数点例 `FixtureFloatABI` は、trampoline が検証されていないため `unsupported-go-abi` である。assembly 対応率は実装済みと数えない。C bridge も任意 function を生成できる段階ではなく、一つの aggregate-by-value 縦切りで生成境界を検証しているだけである。

## Capability 判定

現在の独立 projection 判定は概ね次の優先順を使う。

1. varargs は fixed-signature C adapter が必要
2. `vectorcall`、`thiscall`、`fastcall` は compiler-owned bridge が必要
3. unknown calling convention は unsupported
4. 64 bit を超える scalar など表現不能な引数/戻り値は unsupported
5. vector または aggregate-by-value は C bridge
6. float 引数/戻り値は architecture trampoline
7. それ以外の integer/pointer signature は Pure Go syscall

metadata provider が出す capability はまず候補判定である。公式 full-source symbol でも、P/Invoke の DLL/entry point が固定され、全 parameter/return が primitive/pointer として解決でき、varargs、float/vector、aggregate-by-value、wide scalar by-value 等を含まない subset だけを `generated-purego` に昇格する。availability custom attribute の完全投影前は optional export として扱う。その他は calling convention、layout、ownership、failure rule が確定するまで emitter 対象にしない。

## Pure Go call runtime

Windows では `runtime/winabi` が `syscall.SyscallN` を隔離して呼び出す。返り値は `R1`、`R2`、thread-local last-error snapshot を保持する。非 Windows build は型と validation API を build できるが、native call は `ErrUnsupportedPlatform` を返す。

この backend の対象は integer/pointer ABI だけである。次を無条件に渡さない。

- float argument / return
- vector type と `__vectorcall`
- varargs
- struct/union/array の値渡し・値返し
- 128-bit scalar
- architecture 固有 register class
- compiler が layout を決める C++ 型
- lifetime registry を必要とする複雑 callback

内部 Go runtime への `go:linkname` は使わない。

## DLL と export 解決

system DLL 名は metadata 由来の固定 basename とし、slash、colon、`..`、非 ASCII path character を拒否する。bootstrap した `LoadLibraryExW` と `LOAD_LIBRARY_SEARCH_SYSTEM32` を使い、current directory と一般 `PATH` による search-order hijacking を避ける。

DLL と proc は `sync.Once` で lazy resolve する。package import 時に DLL を一括 load しない。optional export は `Is<Name>Available() bool` で解決可否を確認できる。app-local DLL と API set の個別 policy は未完成であり、現在の System32 loader を任意外部 SDK path の loader として流用しない。

## Raw error semantics

raw wrapper は Windows の戻り値を保つ。`SetLastError` metadata がある API だけ last-error snapshot を返し、その値が非 zero というだけで failure にしない。まず API ごとの `FailureRule`、たとえば `return == FALSE`、`return == NULL`、`return == INVALID_HANDLE_VALUE` を評価する。

`winabi.ErrorIfFalse`、`ErrorIfZero`、`ErrorIfInvalidHandle` は failure sentinel が確認された後だけ last error を解釈する。failure 時に last error が `ERROR_SUCCESS` の場合は、古い thread-local value を成功した error に見せず、`ErrNativeFailureWithoutLastError` を返す。

`HRESULT` と `NTSTATUS` は signed 32-bit raw value、severity、facility、code を保つ。`S_FALSE` を error にせず、NTSTATUS を Win32 error へ勝手に変換しない。checked/ergonomic layer は metadata または明示ルールがある API だけ Go `error` へ変換する。

## C bridge

C bridge の公開面は単純な C ABI とし、C++ class/template を Go へ直接公開しない。compiler に aggregate register/stack classification を任せる。現行 emitter は fixture の `FILETIME` 値渡しを static C function へ平坦化する例を生成する。

bridge file は `windows && cgo` build constraint を持つ。`!windows || !cgo` file は package を build 可能にするだけで、bridge-only API を偽装しない。将来の full bridge emitter では target compiler/SDK version/hash と ABI oracle を対応付ける必要がある。

## Assembly trampoline

assembly を採用する場合、386、amd64、arm64 ごとに register、stack、unwind、return value、callback transition を検証する。ARM64EC を Go `arm64` の別名として扱わない。現時点では検証済み trampoline がないので、float API を `generated-assembly` と数えない。

## Callback

現在の raw callback type は native function addressを表す named `uintptr` である。任意の Go closure を Windows callback に変換する機構ではない。Go callback object を実装するには、生存期間 registry、解放後 call 防止、panic の ABI 境界越え防止、concurrency、thread/apartment、native retained pointer の規則が必要である。それがない signature は raw address/type-only または unsupported とする。

## Architecture 上の注意

386、amd64、arm64 は同じ source symbol でも別の ABI decision を持てる。たとえば Go/386 で表現できない 8-byte alignment は aligned scratch または bridge を使う。cross-compile の成功は register ABI の実行検証ではないため、実機 runner がない architecture は coverage report で compile-verified と runtime-verified を分離する。
