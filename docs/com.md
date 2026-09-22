# COM

## 対応範囲

現在の COM 実装は consumer 側の最小 ABI runtime であり、COM 全体の投影ではない。

| 機能 | 現在の状態 |
|---|---|
| GUID / IID | `winabi.GUID` と canonical parser を実装 |
| IUnknown | vtable slot 0–2、`QueryInterface`、`AddRef`、`Release` を実装 |
| Interface inheritance | vtable prefix を埋め込む基本形を実装 |
| Apartment | `CoInitializeEx` / `CoUninitialize` と OS-thread pin owner を実装 |
| BSTR | `SysAllocStringLen` / `SysStringLen` / `SysFreeString` の owned wrapper を実装 |
| COM task memory | `CoTaskMemAlloc` / `CoTaskMemFree` の owned wrapper を実装 |
| Generated interface | fixture の `IExample` vtable layout 例のみ |
| SAFEARRAY / VARIANT / PROPVARIANT | 未実装 |
| coclass activation helper | 一般化未実装 |
| Automation / IDispatch | 未実装 |
| Go object を COM server/callback として公開 | 未実装 |

Windows SDK metadata 内の全 COM interface、継承、method parameter、IID を生成済みとは主張しない。full-source provider は interface を inventory 化できるが、custom attribute と layout の完全復元、C/C++ oracle による全 vtable 検証、汎用 method emitter は未完成である。

## Object と vtable

COM interface pointer は先頭に vtable pointer を持つ native object として扱う。`IUnknownVTable` は次の順序を固定する。

| Slot | Method |
|---:|---|
| 0 | `QueryInterface` |
| 1 | `AddRef` |
| 2 | `Release` |

派生 interface は base vtable を prefix とし、その後へ metadata order の method slot を追加する。生成器は slot index を IR に保持するが、正式に generated-callable とする前に header の interface declaration または C++ oracle と照合する必要がある。

method 呼出は `winabi.CallAddress` の integer/pointer ABI を使う。aggregate-by-value、float/vector、特殊 return など、この境界で表現できない method は C bridge/assembly または unsupported に分類する。

## Reference ownership

owned COM reference は必ず明示的に `Release` する。finalizer は唯一の解放手段ではない。次を呼出側が区別する。

- `QueryInterface` などが返す新しい owned reference
- `AddRef` で明示的に追加した reference
- parameter として一時的に借用した reference
- API が retained する callback/interface pointer

`Release` 後、特に返り値が zero の object pointer を再利用しない。wrapper をコピーして複数 owner に見せない。現在の低レベル `IUnknown` は smart pointer ではないため、reference の一意 ownership を自動保証しない。

## Apartment と thread affinity

COM initialization は OS thread 単位である。`EnterApartment` は現在の goroutine を `runtime.LockOSThread` で固定し、`CoInitializeEx` が成功した場合だけ owner を返す。`Apartment.Close` は同じ goroutine 上で `CoUninitialize` を呼び、thread lock を解除する。

`S_OK` と `S_FALSE` はどちらも成功として保持する。異なる apartment model がすでに設定されて失敗した場合、owner は返さず thread lock を解除する。`Apartment` を copy したり別 goroutine へ移動したりしてはならない。

基本形は次のとおりである。

```go
apartment, status := com.EnterApartment(com.COINIT_MULTITHREADED)
if status.Failed() {
    return status
}
defer apartment.Close()
```

## BSTR と task allocator

`BSTR` は明示長を持つため embedded NUL を保持する。`NewBSTR` は UTF-16 code unit 数が `UINT32` に収まることを確認し、source slice を native call 終了まで `runtime.KeepAlive` する。`String` は `SysStringLen` の length を address space と Go `int` に対して検査してから copy する。`Close` は pointer を clear し、二度目を no-op にする。

`TaskMemory` は size と native pointer を保持し、`Close` で一度だけ `CoTaskMemFree` する。COM out parameter が返した pointer は `FreeTaskMemory` で解放できる。どちらにも finalizer はない。

SAFEARRAY、VARIANT、PROPVARIANT の ownership と clear routine は未実装なので、それらを含む API を ergonomic に扱えるとは分類しない。

## HRESULT

raw COM method は `HRESULT` を signed 32-bit のまま返す。severity bit が failure の場合だけ `Failed()` / `Err()` が error と見なす。facility/code と成功時の `S_FALSE` を失わない。out parameter は HRESULT 成功を確認するまで使用しない。

## Callback / COM server

Go method を COM object として公開する registry は現在存在しない。将来実装する場合は少なくとも次が必要である。

- native reference count と Go owner の対応
- callback target の生存期間と shutdown 時解放
- 解放後 callback の拒否
- panic を ABI 境界外へ出さない recovery
- concurrent call と reentrancy
- apartment/thread affinity
- Go pointer retention rule を満たす native allocation
- special method signature の backend 判定

これらを満たさずに `syscall.NewCallback` 相当だけを生成して対応済みとはしない。

## 検証と現在の限界

unit test は IUnknown vtable の pointer-size に応じた offset/size と IID を確認する。Windows smoke test は apartment、embedded-NUL BSTR round trip、CoTaskMem allocation/free を非破壊に確認する。これは任意の SDK interface の method order、marshaling、threading model の検証ではない。全 COM API の ABI verified coverage は未達である。
