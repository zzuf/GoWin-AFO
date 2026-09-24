# Publishing GoWin-AFO on GitHub

The publication target specified by the owner is `git@github.com:zzuf/GoWin-AFO.git`. The initial setup used a temporary module path because no remote was configured, but the project has since migrated to `github.com/zzuf/GoWin-AFO`. Pushing the source is not the same as releasing a complete implementation of all APIs.

## Publication contents

- Include the generator/runtime/tests/generated bindings, pinned source lock, coverage, ABI evidence, and LICENSE/NOTICE.
- Exclude `sources/cache`, `.cache`, SDK/WDK/WinMD/NuGet inputs, probe executables, and credentials.
- `coverage/inventory.json.gz` is a large IR derived from official inputs. It is not a substitute for API documentation or SDK payloads. Avoid unjustified full-file changes during updates to limit Git history growth.
- `.gitattributes` enforces LF for generated text and binary handling for gzip files.

## Pre-push checks

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

Repeat the same generation and ABI verification to confirm there is no diff. Check the first GitHub Actions run as well. Do not confuse local cross-compilation with execution testing on macOS/Linux.

## Remote and module path

Use the following URL for `origin`. For an existing repository, first check for history with `git ls-remote <URL>`. Do not overwrite existing history with a force push.

```text
git remote add origin git@github.com:zzuf/GoWin-AFO.git
git push -u origin main
```

The project name is `GoWin-AFO`, recorded in `project.yaml`. The authoritative module path is `github.com/zzuf/GoWin-AFO` in `go.mod`. If the repository moves in the future, migrate handwritten source imports and documentation together, and regenerate generated files through their emitters/templates instead of replacing text directly. `example.com/fixture` and `example.com/renamed` in fixture/golden tests are inputs that verify relocation support, not imports used by published packages. Rerun all test/ABI/generation gates before creating a tag.

Keep `winapigen`, `winapisource`, `winapicoverage`, `winapiverify`, and `abi-oracle` as tool names within GoWin-AFO. Do not change Go packages, native symbols, source IDs, or artifact names merely to align display names.

## JSON schema identifiers

Schema `$id` values have migrated from a temporary local domain to the raw URLs of their corresponding files in this repository. Update external validator registrations or `$ref` values that use the old IDs to the URLs below. `schemaVersion: 1`, data formats, and validation constraints are unchanged.

| Schema | `$id` |
|---|---|
| Source lock | `https://raw.githubusercontent.com/zzuf/GoWin-AFO/main/sources/manifests/source-lock.schema.json` |
| Override | `https://raw.githubusercontent.com/zzuf/GoWin-AFO/main/overrides/schema.json` |
| ABI oracle manifest | `https://raw.githubusercontent.com/zzuf/GoWin-AFO/main/tools/abi-oracle/schema/manifest.schema.json` |
| ABI oracle result | `https://raw.githubusercontent.com/zzuf/GoWin-AFO/main/tools/abi-oracle/schema/result.schema.json` |

For reproducible validation, use the schemas in a checkout of the target commit instead of fetching them at runtime from these mutable branch URLs.

Configure GitHub authentication through Git Credential Manager, SSH, or the GitHub CLI. Do not embed tokens in source, remote URLs, or logs. Do not persist CI checkout credentials for subsequent commands.
