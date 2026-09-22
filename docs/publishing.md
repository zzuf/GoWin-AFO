# GitHub への公開準備

公開先は所有者が指定した `git@github.com:zzuf/GoWin-AFO.git`。最初の整備時点には remote がなかったため仮 module path を使用したが、現在は `github.com/zzuf/GoWin-AFO` へ移行している。ソースを push することと、全 API の完成版を release することは別である。

## 公開対象

- generator/runtime/test/生成バインディング、固定 source lock、coverage、ABI evidence、LICENSE/NOTICE を含める。
- `sources/cache`、`.cache`、SDK/WDK/WinMD/NuGet 入力、probe executable、認証情報は含めない。
- `coverage/inventory.json.gz` は公式入力から作った大規模な IR。API documentation や SDK payload の代替ではない。Git 履歴の肥大化を避けるため、更新時に理由のない全量差分を作らない。
- `.gitattributes` が generated text の LF と gzip binary 扱いを固定する。

## Push 前の確認

```text
go mod verify
go test ./...
go vet ./...
go run ./cmd/winapisource verify
go run ./cmd/winapigen generate --all
go run ./cmd/winapiverify abi --all
go run ./cmd/winapicoverage check --fail-unclassified --fail-regression
git diff --exit-code
git status --short
```

同じ生成・ABI検証をもう一度実行して差分がないことを確認する。GitHub Actions の初回実行結果も確認する。ローカル cross-compile を macOS/Linux の実行検証と混同しない。

## Remote と module path

`origin` には下記 URL を使う。既存 repository なら、まず `git ls-remote <URL>` で履歴の有無を確認する。既存履歴を force-push で上書きしない。

```text
git remote add origin git@github.com:zzuf/GoWin-AFO.git
git push -u origin main
```

module path は `go.mod` の `github.com/zzuf/GoWin-AFO` が正本である。将来 repository を移転する場合も、手書き source の import と docs を同時に移行し、生成ファイルは直接置換せず emitter/template から再生成する。明示的な fixture/golden test に残る旧仮 path はテスト入力であり、公開 package の import ではない。全 test/ABI/generation gate を再実行してから tag を作る。

GitHub 認証は Git credential manager、SSH、または GitHub CLI で設定する。token を source、remote URL、ログへ埋め込まない。CI の checkout 認証情報は後続コマンドへ永続化しない。
