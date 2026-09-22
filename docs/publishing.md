# GitHub への公開準備

この checkout の最初の整備時点では remote がない。公開先・所有者・公開範囲を推測して repository を作成しない。ソースを push することと、全 API の完成版を release することは別である。

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

GitHub repository URL が確定したら、その URL を `origin` に設定する。既存 repository なら、まず `git ls-remote <URL>` で履歴の有無を確認する。既存履歴を force-push で上書きしない。

```text
git remote add origin <confirmed-github-repository-url>
git push -u origin main
```

`go-windows-api.local` は現在の仮 module path であり、この状態は GitHub の source repository として push できるが、公開 Go module のリリースではない。公開 module にする場合、`go.mod` を正本として `github.com/<owner>/<repository>` に変更し、手書き Go source とドキュメント中の import を同時に移行してから再生成する。生成ファイルは直接置換せず、emitter/template に module path を渡す。全 test/ABI/generation gate を再実行してから tag を作る。

GitHub 認証は Git credential manager、SSH、または GitHub CLI で設定する。token を source、remote URL、ログへ埋め込まない。CI の checkout 認証情報は後続コマンドへ永続化しない。
