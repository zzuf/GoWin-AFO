# Go 型投影

## 原則

型投影は「Go でコンパイルできる形」ではなく「対象 Windows ABI で同じ表現と意味を持つ形」を採用する。証明できない layout、alignment、ownership、calling convention は callable として出力しない。raw 層は native name と幅を保ち、便利な string/slice/error/resource wrapper は別層に置く。

現在の generator は複雑な型原則を fixture で検証している段階であり、公式 WinMD の全型を投影済みではない。full-source ingest 中の暫定 `GoType` は候補であって ABI 検証済みの public named type ではない。一方、公式 P/Invoke で primitive/pointer projection がすべて解決できる限定 subset は raw function として出力する。未解決型を `uintptr` で代用して callable に昇格させない。

## LLP64 の基本型

Windows の `long` と pointer width を混同しない。

| Native | Go raw 表現 |
|---|---|
| `CHAR` | `int8` |
| `BYTE` | `uint8` |
| `SHORT` | `int16` |
| `USHORT` | `uint16` |
| `INT` | `int32` |
| `UINT` | `uint32` |
| `LONG` | `int32` |
| `ULONG` | `uint32` |
| `LONGLONG` | `int64` |
| `ULONGLONG` | `uint64` |
| `WCHAR` | `uint16` |
| `BOOL` | `int32` |
| `BOOLEAN` | `uint8` |
| `HRESULT` | named `int32` |
| `NTSTATUS` | named `int32` |
| `SIZE_T`, `UINT_PTR`, `ULONG_PTR` | `uintptr` 幅の named type |
| `SSIZE_T`, `INT_PTR`, `LONG_PTR` | 386 では `int32`、amd64/arm64 では `int64` |
| `HANDLE` | pointer-sized opaque named value |

`runtime/winabi.SignedPointer` は build constraint で signed pointer width を選ぶ。C `int`、`LONG`、`DWORD` の代わりに Go `int` を使わない。metadata signature に native-width 情報が不足する場合は、architecture と typedef 属性が確定するまで出力対象にしない。

## typedef、alias、handle

`NativeTypedef` または distinct native typedef は Go の named type として保つ。本当に interchange 可能と証明できる型だけを alias とする。`HANDLE`、`HWND`、`HKEY` などは同じ machine word でも別の named type とし、誤った API 間代入を減らす。

null handle と `INVALID_HANDLE_VALUE` は別 sentinel である。IR の type は `CloseFunction` と `InvalidValue`、symbol は ownership/allocator/deallocator を持てる。close は自動 finalizer に依存せず呼出側が明示する。関数ごとに borrowed、owned-on-success、retained などを確定し、型名だけから ownership を推測しない。

## Pointer と parameter semantics

IR は pointer depth、optional、element type、in/out/in-out、nullability、byte count、element count、retval、`FreeWith` を分けて持つ。`T*`、optional `T*`、`T**` を同じ `uintptr` と見なさない。

ただし、これらの custom attribute を WinMD provider が全面的に復元する処理は未完成である。その場合、unsafe pointer wrapper を生成するのではなく `unsupported-projection` または type-only に留める。

native が call 後も Go pointer を保持する場合、Go heap pointer を渡しっぱなしにしてはならない。専用 native allocation、pinning 方針、callback registry、または C bridge が必要である。call 中だけ借用する pointer も generated wrapper 内で `runtime.KeepAlive` を使う。

## Struct と architecture 別 layout

通常 struct は field 順序を保ち、architecture ごとの size、alignment、offset、pack、tail padding を IR layout に持つ。Go struct で表現可能で ABI oracle と一致した場合だけ direct layout とする。

pointer-size によって変わる型は `ztypes_windows_386.go`、`ztypes_windows_amd64.go`、`ztypes_windows_arm64.go` のように分離できる。現行縦切りでは `ARCH_WORD` と pointer-sized primitive を三 architecture で検証する。

packed struct、over-aligned member、anonymous record の layout が Go で正確に表現できない場合は、固定 byte storage と accessor、architecture 別 safe representation、または C bridge を選ぶ。誤った通常 struct を出力しない。

## Union

union は最大 size と最大 alignment を保持し、active member を自動追跡しない storage と typed accessor を生成する。accessor の呼出側が、どの member が有効かと lifetime を管理する。

縦切りの `LARGE_INTEGER` は、amd64/arm64 では 8-byte aligned storage と overlapping accessor を使う。Go/386 は MSVC の 8-byte alignment を型として表現できないため、size-correct storage のみを提供し、直接 ABI 安全とは主張しない。`QueryPerformanceCounter` wrapper は 8-byte aligned scratch buffer を作り、結果を accessor 経由でコピーする。

## Bit field

IR は storage type、bit offset、bit width、signedness を保持する。生成型は raw storage word と getter/setter を持ち、setter は対象 mask だけを変更する。signed field の getter は field width から明示的に符号拡張する。

Clang AST provider は現在 bit width の基本 inventory までで、ターゲットごとの完全な allocation unit/offset を確定していない。header 由来 bit field を full-source で生成する前に layout probe が必要である。

## Array

fixed array は長さと element type を保持し、値が Go の型範囲を超える場合は出力を止める。flexible array member は `[1]T` に偽装しない。固定 header と caller-owned buffer view を分け、view 作成時に次を検査する。

- base pointer が nil でないこと
- allocation が header size 以上であること
- element count が残り byte 数を超えないこと
- count が Go `int` と address space に収まること

view は元 allocation より長生きできず、Windows が call 後も保持する用途には使えない。

## String

raw 層の `PCWSTR`、`PWSTR`、`PCSTR`、`PSTR` は native pointer に近い borrowed type とする。Win32 の NUL 終端 UTF-16 helper は Go string 内の NUL を検出してエラーにし、暗黙に切り詰めない。

`BSTR` と `HSTRING` は長さ付きであるため、埋め込み NUL を保持できる。runtime wrapper は native allocator/deallocator を対にし、explicit `Close` を要求する。activation class name は別の protocol boundary なので埋め込み NUL を拒否する。

## Enum、flags、constant

underlying native type と signedness を保ち、enum と flags enum を区別する。同じ値の複数名を削除しない。architecture dependent 値は build constraint で分ける。Go の target type に収まらない値を暗黙 truncate しない。

現在の full WinMD provider は literal field の raw constant blob を inventory 化するだけで、すべての constant encoding と associated enum を Go expression に変換し終えてはいない。公式全定数を生成済みとは扱わない。

## A/W API

`FunctionA` と `FunctionW` は native entry point 名のまま別々に出力する。C preprocessor の `Function` alias を raw 層で曖昧に再現しない。Unicode を標準とする便利 alias を追加する場合は、プロジェクト独自の ergonomic API と明示する。

## ABI 検証境界

Go の `unsafe.Sizeof` / `Alignof` / `Offsetof` test は必要だが、それだけで header ABI の証明にはならない。C/C++ oracle から得た size、alignment、offset、constant、GUID、vtable slot、function type と比較して初めて `ABIVerified` を立てる。現在、fixture の Go layout test と Windows smoke test はあるが、Windows SDK 全体の oracle 結果はまだないため、全型 ABI 検証済みとは主張しない。
