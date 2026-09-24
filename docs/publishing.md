# GoWin-AFO の GitHub 公開

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

プロジェクト名は `GoWin-AFO` とし、`project.yaml` に記録する。module path は `go.mod` の `github.com/zzuf/GoWin-AFO` が正本である。将来 repository を移転する場合も、手書き source の import と docs を同時に移行し、生成ファイルは直接置換せず emitter/template から再生成する。fixture/golden test の `example.com/fixture` や `example.com/renamed` は移転可能性を検証するテスト入力であり、公開 package の import ではない。全 test/ABI/generation gate を再実行してから tag を作る。

`winapigen`、`winapisource`、`winapicoverage`、`winapiverify`、`abi-oracle` は GoWin-AFO 内の tool 名として維持する。Go package、native symbol、source ID、artifact 名も表示名の統一だけでは変更しない。

## JSON schema の識別子

schema の `$id` は仮ローカルドメインから、この repository 内の対応ファイルの raw URL へ移行した。外部 validator の登録や `$ref` に旧 ID を指定している場合は、以下の URL へ更新する。`schemaVersion: 1` とデータ形式・検証制約は変更していない。

| Schema | `$id` |
|---|---|
| Source lock | `https://raw.githubusercontent.com/zzuf/GoWin-AFO/main/sources/manifests/source-lock.schema.json` |
| Override | `https://raw.githubusercontent.com/zzuf/GoWin-AFO/main/overrides/schema.json` |
| ABI oracle manifest | `https://raw.githubusercontent.com/zzuf/GoWin-AFO/main/tools/abi-oracle/schema/manifest.schema.json` |
| ABI oracle result | `https://raw.githubusercontent.com/zzuf/GoWin-AFO/main/tools/abi-oracle/schema/result.schema.json` |

再現性が必要な検証では、この可変ブランチの URL から実行時に取得するのではなく、対象 commit の checkout 内にある schema を使う。

GitHub 認証は Git credential manager、SSH、または GitHub CLI で設定する。token を source、remote URL、ログへ埋め込まない。CI の checkout 認証情報は後続コマンドへ永続化しない。
