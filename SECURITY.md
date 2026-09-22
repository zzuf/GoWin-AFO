# Security policy

## 脆弱性の報告

DLL search hijacking、生成された signature/layout の ABI corruption、use-after-free/double-free、callback lifetime、COM reference count、archive path traversal、source hash bypass、生成器への code injection は security issue として扱う。

公開 issue に exploit detail、proof-of-concept、未公開の脆弱 package/version を書かない。Git hosting が private security advisory を提供する場合はそれを使用する。private channel が利用できない場合は、影響範囲だけを公開 issue で知らせ、maintainer が非公開連絡方法を提示するまで詳細を保留する。実在しない security email や応答期限はここでは約束しない。

報告には可能な範囲で次を含める。

- 影響する source ID、SDK version、symbol ID、architecture/profile
- generator/runtime/bridge の version または commit
- expected/actual ABI、crash または loading path
- 再現に必要な最小 manifest/test。Microsoft 再配布不可 artifact は添付しない
- 攻撃者が制御できる入力と必要権限
- suggested mitigation があればその内容

## 対応対象

security fix は原則として main branch の現行 generator/runtime を対象とする。公開 release と support window が定義されるまでは、古い snapshot の継続 support を保証しない。Microsoft SDK/WDK/Windows 自体の脆弱性は Microsoft の reporting process へ報告する。

## Threat model と防御

### Source supply chain

公式入力は `sources.lock.json` の固定 HTTPS locator/version/SHA-256 で識別する。hash 検証前の artifact を parser の信頼入力や生成正本にしない。download size を制限し、ZIP から lock に列挙した相対 path だけを抽出する。optional SDK がない場合に別 version へ黙って fallback しない。

source lock、provider manifest、override、ABI probe expression は code-review 対象である。特に ABI manifest の expression は C++ code であり、untrusted metadata/API request/PR artifact から動的に作らない。CI token は read-only を基本とし、SDK assessment workflow は merge/publish permission を持たない。

### DLL loading

system DLL は metadata 由来の許可された basename に限定し、System32-safe search policy と lazy resolution を使う。current directory と一般の `PATH` を system DLL discovery に使わない。DLL/export 名を user input からそのまま渡さない。app-local DLL support を追加する場合は trusted absolute root、canonical path、署名/配布 policy を別に設計する。

### Native memory と ownership

Go pointer を native code が call 後も保持する API は専用 native allocation/bridge/registry がない限り callable にしない。temporary UTF-16 buffer は call 終了まで `runtime.KeepAlive` し、NUL-terminated API は embedded NUL を拒否する。length/element count の乗算と `uintptr`/Go `int` 変換を overflow check する。可変 buffer retry には上限を置く。

handle、BSTR、HSTRING、CoTaskMem、COM reference は ownership と deallocator を保持し、明示 `Close`/`Release` を基本にする。finalizer を唯一の解放手段にせず、close/release 後 pointer を clear して二重解放を防ぐ。

### ABI boundary

不明な signature を `uintptr` 列へ押し込まない。float/vector、varargs、aggregate-by-value、特殊 callback/calling convention は oracle で証明した bridge/trampoline、または `unsupported-go-abi` とする。panic を callback/COM ABI 境界外へ出さない。callback registry は retained lifetime、concurrency、shutdown、解放後 call を扱うまで公開しない。

kernel-only/undocumented API を通常 process から実行可能に見せない。ABI mismatch、unclassified symbol、generation drift、coverage regression は CI failure とする。

## Security test の制約

通常 CI で driver/service install、registry write、system configuration change、administrator-only operation を行わない。脆弱性再現が破壊的な場合は isolated disposable Windows VM で行い、repository CI へそのまま追加しない。credential、private symbol、licensed SDK payload を test artifact/log に含めない。
