# WinRT

## 対応範囲

現在の `runtime/winrt` は WinRT consumer の基礎 ABI を実装する。

- `RoInitialize` / `RoUninitialize`
- owned `HSTRING` の作成、読み取り、削除
- `IUnknown` を prefix に持つ `IInspectable`
- `GetIids`、`GetRuntimeClassName`、`GetTrustLevel`
- `IActivationFactory` と `ActivateInstance`
- `RoGetActivationFactory` / `RoActivateInstance`
- `Windows.Foundation.Uri` activation factory の小規模 generated example

これらは Windows SDK `10.0.26100.0` の縦切りであり、Windows contract WinMD 全体の generated projection ではない。

## Apartment

WinRT initialization も OS thread に属する。`EnterApartment` は goroutine を OS thread に固定し、`RoInitialize` 成功後に owner を返す。`Close` は同じ goroutine で `RoUninitialize` を呼び、thread lock を解除する。STA/MTA は `RO_INIT_SINGLETHREADED` と `RO_INIT_MULTITHREADED` で明示する。

runtime class metadata の threading model と marshaling behavior を full-source provider が全面的に取り込む処理はまだない。生成 wrapper が自由に goroutine 間移動できるとは仮定しない。

## HSTRING

`HString` は `WindowsCreateString` で作られた handle を所有し、`Close` で `WindowsDeleteString` を一度だけ呼ぶ。`WindowsGetStringRawBuffer` が返した length を検査してから Go string へ copy する。HSTRING 自体は長さ付きなので embedded NUL を保持する。

runtime class name は activation protocol の識別子であるため、空文字列と embedded NUL を拒否する。これにより文字列境界の曖昧化を避ける。HSTRING handle を `Close` 後に利用してはならず、native call 中の wrapper lifetime は `runtime.KeepAlive` で保つ。

## IInspectable と activation

`IInspectableVTable` は IUnknown の 3 slot に続いて次の順序を持つ。

| Slot | Method |
|---:|---|
| 3 | `GetIids` |
| 4 | `GetRuntimeClassName` |
| 5 | `GetTrustLevel` |

`GetIids` の結果は Go slice へ copy した後、native 配列を `CoTaskMemFree` する。runtime class name は owned HSTRING として返す。activation factory/instance の成功結果は owned COM reference なので、呼出側が `Release` する。

```go
apartment, status := winrt.EnterApartment(winrt.RO_INIT_MULTITHREADED)
if status.Failed() {
    return status
}
defer apartment.Close()

factory, status := winrt.GetActivationFactory("Windows.Foundation.Uri")
if status.Failed() {
    return status
}
defer factory.Release()
```

## Metadata projection

WinRT provider は SDK `UnionMetadata/10.0.26100.0/Windows.winmd` を version/hash 固定した required input として扱う。現在の WinMD reader は WinRT interface、runtime class、delegate、generic arity、property、event の基本種別を inventory 化できるが、次の意味情報を完全には投影していない。

- default interface と activation factory attribute
- contract version と deprecation
- threading model と marshaling behavior
- method parameter の ownership/nullability
- event add/remove pairing と `EventRegistrationToken`
- async interface と progress/completion handler
- closed generic instantiation と signature grammar
- collection interface の ergonomic projection

そのため full-source WinRT symbol は現在原則 `unsupported-projection` / type-only inventory であり、callable として emitter へ流さない。

## Generic と parameterized IID

WinRT generic interface の IID は単なる generic type の GUID ではなく、canonical WinRT signature と規定 namespace に基づく parameterized IID 計算が必要である。現在の `ParameterizedIID` は意図的に `ErrParameterizedIIDUnsupported` を返し、zero GUID を返す。推測した IID で `QueryInterface` を行わない。

closed generic metadata の生成と任意 type argument の runtime 計算は未実装である。この項目が検証されるまで generic collection、async operation、delegate を対応済みと数えない。

## Delegate、event、async

delegate object、event registration/removal、`IAsyncAction`、`IAsyncOperation<T>`、progress/completion handler は未実装である。これらには COM reference、callback lifetime、apartment transition、completion race、panic containment、parameterized IID が関係する。単なる function pointer や channel wrapper で ABI を省略しない。

## HRESULT と所有権

WinRT raw call は HRESULT を保持し、failure を severity bit で判定する。成功時の status も捨てない。out pointer/HSTRING は成功後だけ owner として採用し、failure 時に nil/zero でない可能性を安易に使用しない。IInspectable/IActivationFactory は finalizer ではなく明示 `Release`、HString は明示 `Close` を基本とする。

## 検証

unit test は IInspectable vtable offset、parameterized IID の明示 failure、activation name の NUL rejection を確認する。Windows smoke test は HSTRING の embedded-NUL round trip と `Windows.Foundation.Uri` の activation factory 取得・Release を確認する。これは WinRT 全 runtime class、contract、architecture の実行検証ではない。
