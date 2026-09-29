# 実装計画書：パイプラインの骨格

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-09-29 |
| Review date | 2026-09-29 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`01_requirements.md` で定義したパイプラインの骨格を実装する。段階間で受け渡す共通データ型、4 つの interface、各段階を順に呼び出すパイプライン（`internal/pipeline`）、テスト用の fake、秘密情報を保持する `Secret` 型を、`02_architecture.md`（以下「architecture」）の設計どおりに新設する。これにより後続タスク（#3・#4・#5・#6・#7）が fake を相手に単体で開発・テストできる状態を作る。

### 1.2. 実装原則

- architecture §1.1 の設計原則 5 項目に従う。
- 段階の責務は各パッケージに閉じ込め、パイプラインは interface だけに依存する（architecture §1.1-1）。
- プロバイダ固有の知識を共通型・interface・パイプラインに持ち込まない（AC-05、architecture §1.1-3）。
- ユニットテストは外部コマンド・LLM API・Webhook を一切呼ばない。テストは fake と標準ライブラリのみで行う。
- Go のコメント・識別子・文字列リテラルは英語で書く（計画本文は日本語）。
- Go のソース（テストを含む）に `AC-NN` / `F-NNN` を書かない。AC 対応は本計画にのみ記録する（`runplan.md` の pre-commit チェック）。
- 各フェーズの完了時に `make fmt` → `make test` → `make lint` を実行し、通した状態で進める。
- 各テストは、対応する対策や分岐を実装時に壊し、テストが失敗することを確認して、その旨をコミットメッセージに記録する（CLAUDE.md「Testing Strategy」）。本計画ではこの義務を各フェーズのタスクとして明記する。

### 1.3. 既存コード調査結果

リポジトリの現状を HEAD `4a2fde5` で確認した。

- **Go ソース**: `cmd/yt2column/main.go` のみ（空の `main()`、`cmd/yt2column/main.go:4`）。`internal/` 以下のパッケージ・テスト・テストヘルパーは存在せず、再利用できる既存コンポーネントはない。本タスクで変更・削除する既存シンボルはなく、更新が必要になる既存テストもない（純粋な追加）。
- **package_reference.md**: `docs/dev/developer_guide/package_reference.md:7` が「No packages exist yet」のまま。本タスクで新設する 10 パッケージ（`internal/secret`・`internal/transcript`・`internal/transcript/testutil`・`internal/llm`・`internal/llm/testutil`・`internal/writer`・`internal/writer/testutil`・`internal/publisher`・`internal/publisher/testutil`・`internal/pipeline`）を登録する。登録は同ファイル冒頭の「パッケージを追加するコミットと同じコミットで更新する」規則に従い、各パッケージを新設するフェーズで行う（フェーズ 5 にまとめない）。最初の行を追加するフェーズ 1 では、冒頭の「No packages exist yet」の記述と「Move each entry here」の案内も更新する。
- **project_overview.md**: 旧表記の残存箇所を `rg` で列挙した（HEAD `4a2fde5`、`docs/dev/project_overview.md` のみを対象。`requirements_process.md:106` の `System Structure` は無関係な見出しのため対象外）。
  - `docs/dev/project_overview.md:37`: `Generate(ctx context.Context, req GenerateRequest) (string, error)`
  - `docs/dev/project_overview.md:39`: `// GenerateRequest: System, User, MaxOutputTokens, Temperature など、プロバイダ共通の最小限の項目のみ`
  - あわせて `project_overview.md:33` の責務の記述（「テキストを返す」）と、想定ディレクトリ構成（`project_overview.md:68-83`、`internal/secret/` が未記載）を architecture 付録A のとおり更新する（フェーズ 5）。
- **`Secret` の構築経路**: 新設の `internal/secret` はリーフパッケージで `value func() string` を非公開フィールドに持ち、公開コンストラクタは `New` のみ。構築経路は `New` とゼロ値の 2 経路に限定され、ゼロ値は `Reveal()` がエラーを返して拒否する（AC-23）。同一パッケージ内の複合リテラルによる直接構築は、型定義を `secret.go` に集約して目視で確認できる範囲に限定する（architecture §3.3 に追加の構造は生じない）。等値判定は要件がないため追加しない（YAGNI、architecture §9）。
- **ツールチェーン（解決済み・前提タスク不要）**: golangci-lint のピンは 3 箇所とも既に `v2.13.2` で、以前のタスクで更新済み（`Makefile:9`・`.pre-commit-config.yaml:23`・`.github/workflows/ci.yml:86`）。ローカルの Go は 1.27.1、`go.mod` は `go 1.26.5`。`make test` は `go test -tags test -race`（`Makefile:63-64`）、`make lint` は `--build-tags test`（`Makefile:10`）で実行され、`//go:build test` の fake は両方のゲートでコンパイルされる。本番バイナリの `make build` は `./cmd/yt2column` のみをビルドする（`Makefile:53-55`）。
- **新規の外部依存**: なし。標準ライブラリのみを使う（depguard の `deps` ルール、`.golangci.yml:53-59`）。
- **検証済みの Go 標準ライブラリの挙動**: 本計画のテスト設計が依存する `fmt`・`log/slog`・`encoding/json` の挙動を、ローカルの Go 1.27.1（`go version` で確認）で、一時的な検証用モジュール（`go run .`、2026-09-29 実行）を使って確認した。CI は `go-version-file: go.mod` により go 1.26.5 で実行される（`.github/workflows/ci.yml:57`）。主要な出力は次のとおり（`Secret` は `Format`・`String()`・`GoString()`・`LogValue`・`MarshalJSON` を実装し、元の値を `func() string` で保持する型）。
  - `fmt` は `Formatter` を実装した値について、`%s`・`%v`・`%+v`・`%#v`・`%q`・`%d`・`%x` などの指定子で `Format` を呼ぶ。`%p`（非ポインタ値）は `Format` を呼ばず `%!p(...)` の形になる（元の値は現れない）。
  - 非公開フィールドに `Secret` を持つ構造体を `%v`・`%+v`・`%#v` で表示すると、クロージャの関数アドレスのみが出力され元の値は現れない。
  - slog の TextHandler は、属性として渡した `Secret` を `LogValuer` で解決して `[REDACTED]` を出力し、非公開フィールド経由の構造体では `fmt` と同様にリフレクション表示（関数アドレスのみ）になる。
  - `encoding/json` は `MarshalJSON` により `"[REDACTED]"` を出力し、非公開フィールドは出力に含めない。

  ```
  direct  %v   = [REDACTED]
  direct  %#v  = [REDACTED]
  direct  %p   = %!p(main.Secret={0x...})   （元の値は現れない）
  public  %+v  = {S:[REDACTED]}
  public  %#v  = main.Public{S:[REDACTED]}
  private %+v  = {s:{v:0x...}}              （クロージャの関数アドレスのみ）
  json public  = {"S":"[REDACTED]"}
  json private = {}
  slog direct  = ... secret=[REDACTED]
  slog private = ... struct={s:{v:0x...}}
  ```
- **architecture との整合**: 本計画は architecture §8 の実装優先順位（フェーズ 1〜5）をそのまま採用する。architecture の修正が必要な不整合は調査で見つかっていない。

## 2. 実装ステップ (Implementation Steps)

各フェーズの完了条件は「そのフェーズの `make fmt` → `make test` → `make lint` が通ること」。architecture §8 の実装優先順位に沿い、依存の向きの下から順に実装する。ステップは `X-Y` 形式で表す（X: フェーズ番号、Y: フェーズ内の連番）。テスト関数名は §5 の受け入れ基準の検証表にまとめる（各フェーズのタスクでは重複して列挙しない）。

### フェーズ 1: `internal/secret`（秘密情報型）

**対象ファイル**
- 新設: `internal/secret/secret.go`・`internal/secret/secret_test.go`
- 変更: `docs/dev/developer_guide/package_reference.md`

**タスク**
- [x] **ステップ 1-1**: `internal/secret/secret.go` を作成し、architecture §3.3 の型定義どおりに実装する。
  - 元の値を `func() string` クロージャで保持する（`01_requirements.md` §5 の制約）。
  - `New(value string) (Secret, error)`: 空文字列はエラーにする（AC-22）。拒否のエラーはパッケージレベルの静的センチネル（`errors.New`）とする（err113 が `%w` のない `fmt.Errorf` を拒否するため）。
  - `Reveal() (string, error)`: ゼロ値（nil クロージャ）はエラーにする（AC-23）。元の値を返す経路はこのメソッドのみにする（AC-21）。
  - `Format(f fmt.State, verb rune)`: すべての書式指定子で `[REDACTED]` を含む固定文字列を書く（AC-18）。`%q` では引用符付きの表現を書く。
  - `String() string`・`GoString() string`: 直接呼び出された場合も固定文字列を返す（AC-24）。
  - `LogValue() slog.Value`: 属性として `[REDACTED]` を返す（AC-19）。
  - `MarshalJSON() ([]byte, error)`: `"[REDACTED]"` を返す（AC-20）。
  - パッケージコメントとすべての公開識別子に英語のドキュメントコメントを付ける（revive の `package-comments` と `exported` が `make lint` で検査する）。`[REDACTED]` の繰り返しは goconst を避けるためパッケージ定数にまとめる。
- [x] **ステップ 1-2**: `internal/secret/secret_test.go` を作成し、AC-18〜AC-26 のテストを置く（テスト関数名は §5）。`fmt` の直接書式化は固定文字列との完全一致で検証する（`[REDACTED]` の部分一致だけでは、長さやハッシュを付け足した出力を見逃す）。非公開フィールド経路は元の値が現れないことを検証し、出力される関数アドレスはゴールデンファイル化しない（architecture §7.1）。
- [x] **ステップ 1-3**: `docs/dev/developer_guide/package_reference.md` の冒頭を更新し（「No packages exist yet」の記述と「Move each entry here」の案内）、`internal/secret` の行を追加する。プレースホルダの `| _(none yet)_ | |` 行（`package_reference.md:14`）は削除する。
- [x] **ステップ 1-4**: 主要な対策を実装時に壊してテストが失敗することを確認し、コミットメッセージに記録する（例: `Format` を元の値を出力する実装に変えると `TestSecretFmtRedaction` が失敗する、`String()`・`GoString()` を元の値に変えると `TestSecretStringGoString` が失敗する、`LogValue` を元の値を返す実装に変えると `TestSecretSlogRedaction` が失敗する（`LogValue` を外すだけでは、slog が `fmt` 経由で `Format` を呼ぶため `[REDACTED]` のままになり失敗しない）、`MarshalJSON` を外すと `TestSecretJSONRedaction` が失敗する、クロージャ保持を素の `string` フィールドに変えると `TestSecretUnexportedFieldNoLeak` が失敗する、`New` の空文字列チェックを外すと `TestSecretNewEmpty` が失敗する）。
- [x] **ステップ 1-5**: `make fmt` → `make test` → `make lint` を通す。

### PR-1 作成ポイント: secret type and redaction guarantees

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `feat(0001): add Secret type with output-redaction guarantees`

**レビュー観点**: 元の値が fmt（委譲・非委譲・非公開フィールド）・`String()`/`GoString()`・slog・JSON の全経路で現れないこと（AC-18・AC-19・AC-20・AC-24・AC-25・AC-26） / `func() string` クロージャ保持と構築経路の 2 経路限定が設計どおりであること / 空文字列とゼロ値を拒否すること（AC-22・AC-23） / 元の値を取り出す経路が `Reveal()` のみであること（AC-21。メソッド集合の guard `TestSecretRevealExclusive` は architecture §7.1 により PR-4 で入るため、本 PR では目視で確認する） / 各テストが対応する対策を壊すと失敗することをコミットで記録していること

**実装モデル要件**: frontier-recommended

**判定理由**: 秘密情報の非開示という、誤ると漏洩に直結する高リスクな単独ステップ（ステップ 1-1・1-2）を隔離した PR。競合する実装方針の併記はないが、「リスク隔離」の対象となる high-risk なステップのため。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 構成要素パッケージのデータ型と interface

**対象ファイル**
- 新設: `internal/transcript/transcript.go`・`internal/llm/llm.go`・`internal/writer/writer.go`・`internal/publisher/publisher.go`
- 変更: `docs/dev/developer_guide/package_reference.md`

**タスク**
- [x] **ステップ 2-1**: `internal/transcript/transcript.go` に `Segment`・`Transcript` を architecture §3.1 のとおり定義する。
- [x] **ステップ 2-2**: `internal/llm/llm.go` に `GenerateRequest`・`GenerateResponse` を architecture §3.1 のとおり定義する。プロバイダ固有の項目（`thinking`・`reasoning_content` など）や SDK の型は含めない（AC-05）。
- [x] **ステップ 2-3**: `internal/writer/writer.go` に `Article` を architecture §3.1 のとおり定義する。
- [x] **ステップ 2-4**: 4 つの interface（`TranscriptSource`・`LLMClient`・`ArticleWriter`・`Publisher`）を architecture §3.2 の Go 定義どおりに定義する。すべてのメソッドの第 1 引数は `context.Context`。契約（失敗時はエラーを返す・空の結果を正常な結果として返さない・`Publisher` は不完全な記事を投稿しない）を英語のドキュメントコメントとして書く（AC-06〜AC-08）。コメントの文言は architecture §3.2 に従う。4 パッケージそれぞれに英語のパッケージコメントを付ける（revive の `package-comments`）。
- [x] **ステップ 2-5**: この 4 パッケージにパッケージ本体の `_test.go` は置かない（architecture §7.1）。
- [x] **ステップ 2-6**: `docs/dev/developer_guide/package_reference.md` に `internal/transcript`・`internal/llm`・`internal/writer`・`internal/publisher` の 4 行を追加する。
- [x] **ステップ 2-7**: `make fmt` → `make test` → `make lint` を通す。

### PR-2 作成ポイント: stage interfaces and common data types

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7

**推奨タイトル**: `feat(0001): add pipeline stage interfaces and common data types`

**レビュー観点**: データ型と interface のシグネチャが architecture §3.1・§3.2 の Go 定義と一致すること / プロバイダ固有の項目や SDK の型が混入していないこと（AC-05） / interface の契約コメントが英語で「空の結果を正常としない」条項を含むこと（AC-08） / `package_reference.md` に 4 パッケージが登録されていること / 型・interface の guard（`TestCommonTypesFieldSets`・`TestInterfaceContracts`・`TestInterfaceDocComments`）は architecture §7.1 により PR-4 で入るため、本 PR のレビューで一致を目視で確認すること

**実装モデル要件**: standard

**判定理由**: 型宣言と interface 定義のみで、競合する実装方針の併記も高リスクな制御（リカバリ・並行・状態機械）もなく、どのトリガーにも該当しない。下流タスクが依存する契約を凍結するが、競合方針や高リスクな制御を含まないため frontier の要件には達しない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: テスト用の fake

**対象ファイル**
- 新設: `internal/transcript/testutil/mocks.go`・`internal/transcript/testutil/mocks_test.go`
- 新設: `internal/llm/testutil/mocks.go`・`internal/llm/testutil/mocks_test.go`
- 新設: `internal/writer/testutil/mocks.go`・`internal/writer/testutil/mocks_test.go`
- 新設: `internal/publisher/testutil/mocks.go`・`internal/publisher/testutil/mocks_test.go`
- 変更: `docs/dev/developer_guide/package_reference.md`

**共通パターン（4 つの fake すべてに適用）**
- `mocks.go`・`mocks_test.go` の両方の先頭に `//go:build test` を付ける（AC-17。`docs/dev/developer_guide/test_organization.md:91-93`）。
- パッケージ名は `<domain>testutil`（`transcripttestutil`・`llmtestutil`・`writertestutil`・`publishertestutil`）とする（同ガイドの分類 A）。
- 設定可能な `Result` と `Err`、および呼び出し記録用の `Calls []<Fake>Call`（各要素は `Ctx` と入力引数を持つ）をフィールドに持つ（`FakePublisher` は `Publish` が `error` のみを返すため `Result` を持たず `Err` のみを持つ）。メソッドは**ポインタレシーバ**で定義し、呼び出しごとに入力引数を `Calls` に追記する（AC-15・AC-16）。
- コンパイル時アサーション `var _ <interface> = (*<Fake>)(nil)` を置く（AC-06）。
- パッケージコメントとすべての公開識別子に英語のドキュメントコメントを付ける。

**タスク**
- [x] **ステップ 3-1**: `internal/transcript/testutil/mocks.go` に `FakeTranscriptSource` を、`mocks_test.go` にその振る舞いのテストを作成する（記録する入力は `VideoURL`）。
- [x] **ステップ 3-2**: `internal/llm/testutil/mocks.go` に `FakeLLMClient` を、`mocks_test.go` にその振る舞いのテストを作成する（記録する入力は `GenerateRequest`）。
- [x] **ステップ 3-3**: `internal/writer/testutil/mocks.go` に `FakeArticleWriter` を、`mocks_test.go` にその振る舞いのテストを作成する（記録する入力は `Transcript`）。
- [x] **ステップ 3-4**: `internal/publisher/testutil/mocks.go` に `FakePublisher` を、`mocks_test.go` にその振る舞いのテストを作成する（記録する入力は `Article`）。
- [x] **ステップ 3-5**: fake の振る舞い（戻り値・エラー・呼び出し記録）を実装時に壊し、各 `mocks_test.go` が失敗することを確認して、コミットメッセージに記録する（例: メソッドを値レシーバに変えると `Calls` が記録されず失敗する）。
- [x] **ステップ 3-6**: `docs/dev/developer_guide/package_reference.md` に `testutil` 4 パッケージの行を追加する。
- [x] **ステップ 3-7**: タグなしのビルドに fake が含まれないことを `go list -e -f '{{.ImportPath}} {{len .GoFiles}} {{.Error}}' ./internal/transcript/testutil ./internal/llm/testutil ./internal/writer/testutil ./internal/publisher/testutil` で確認する。4 つの `testutil` パッケージそれぞれについて、`GoFiles` が 0 件であること、および `.Error` が「すべての Go ファイルがビルド制約で除外された」ことを示す（`build constraints exclude all Go files`）ことを出力とともに記録する（architecture §7.2 が本計画に委ねた `go list` のコマンド。AC-17）。`-e` はエラーを標準エラーではなくパッケージの `Error` フィールドに入れるため、テンプレートで `.Error` を描画しないと、存在しない import path の誤りも `0` とだけ表示されて期待結果と区別できない。`./internal/...` のようなパターンは、すべてのファイルがビルド制約で除外されたディレクトリを列挙しない（`go help packages` はパターンが「パッケージのディレクトリ」に展開されるとし、`go help list` は名前付きパッケージを 1 行ずつ列挙するとする）ため、`testutil` パッケージは import path を明示して `go list` に渡す。
- [x] **ステップ 3-8**: `make fmt` → `make test` → `make lint` を通す。`make test` は `-tags test` で実行されるため、fake が同じタグでコンパイルされることをこのフェーズのゲートで確認する（`Makefile:63-64`）。

### PR-3 作成ポイント: test fakes for stage interfaces

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8

**推奨タイトル**: `feat(0001): add test fakes for the pipeline stage interfaces`

**レビュー観点**: `testutil/` 配下の全ファイルに `//go:build test` が付き、本番バイナリに含まれないこと（AC-17） / ポインタレシーバで呼び出しが `Calls` に記録されること（AC-15・AC-16） / `var _ <interface> = (*<Fake>)(nil)` で interface を満たすこと（AC-06） / タグなし `go list` で `testutil` の `GoFiles` が 0 件であることを記録していること / タグの guard（`TestFakesCarryBuildTag`）は architecture §7.1 により PR-4 で入るため、本 PR の緑ゲートはタグなし `go list` の記録で担保すること

**実装モデル要件**: standard

**判定理由**: 定型的なテストダブルで、競合する実装方針の併記も高リスクな制御もなく、どのトリガーにも該当しない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: `internal/pipeline`

**対象ファイル**
- 新設: `internal/pipeline/pipeline.go`・`internal/pipeline/pipeline_test.go`
- 変更: `docs/dev/developer_guide/package_reference.md`

**タスク**
- [x] **ステップ 4-1**: `internal/pipeline/pipeline.go` を作成し、architecture §3.4 の定義どおりに実装する。
  - `Stage` 列挙型（ゼロ値 `StageUnknown`）と `String()`。ゼロ値と未知の値には `"unknown"` を返す（fail-secure）。`StageError.Error()` は必ず `String()` を使う。
  - `StageError`（`Stage`・`Err`）と `Error()`・`Unwrap()`。エラーメッセージは `"<段階名>: <元のエラー>"` の形式とし、元のエラー以外の情報を付け加えない。
  - センチネル `ErrNilStage`。
  - `Pipeline` 構造体と `New`・`Run`。
  - `New` は interface 値の `== nil` に加え、動的値が nil の interface（typed-nil）を拒否し、`ErrNilStage` をラップしてどの段階が未設定かを含むエラーを返す（AC-14）。検出は `reflect.ValueOf(v)` の `Kind` が nil になりうる種別（`reflect.Ptr`・`reflect.Func`・`reflect.Map`・`reflect.Slice`・`reflect.Chan`）である場合に `IsNil()` で判定し、それ以外の Kind は nil 判定の対象外とする（architecture §3.4 が本計画に委ねた実装方法）。
  - `Run` は冒頭で 3 つの段階の nil を再確認し、nil なら段階を呼び出さずに `ErrNilStage` をラップしたエラーを返す（ゼロ値の fail-closed、AC-14a）。各段階の呼び出し前に `ctx.Err()` を確認し、キャンセル済みなら `ctx.Err()` をそのまま返して以降の段階を呼ばない（AC-13）。段階の失敗は対応する `Stage` を付けた `StageError` で包む（AC-11・AC-12）。すべて成功した場合は投稿した `Article` を返す（AC-09・AC-10）。
  - パッケージコメントとすべての公開識別子に英語のドキュメントコメントを付ける。
- [x] **ステップ 4-2**: `internal/pipeline/pipeline_test.go` を作成し、AC-09〜AC-14・AC-14a の振る舞いテストと、§5 に記載する guard テストを置く。テストはフェーズ 3 の fake を注入して行う。`TestPipelineSuccess` では、source fake が返す `Transcript` に順序つきの複数セグメントとメタ情報を持たせ、writer fake が記録した値が並び順・内容ともに一致することを検証する（AC-01・AC-02）。`pipeline_test.go` は `//go:build test` のタグ付き `testutil` パッケージを import するため、ファイル先頭に `//go:build test` を付ける（付けないと、タグを渡さない `go vet ./...` や IDE が `build constraints exclude all Go files` で失敗する。architecture §3.5 の AC-17 の注意と同じ理由）。AC-13 の段階間キャンセルは、`Fetch` の呼び出しを観測してから `ctx.Err()` が `context.Canceled` を返すよう切り替えるカスタム `context.Context`（`pipeline_test.go` 内で定義）という決定的な同期手段で実現し、タイマー待ちやスリープは使わない。`Stage` のゼロ値・既知の値・未知の値を検証する補助テスト（`TestStageString`。`StageError.Error()` が `Stage.String()` を使うことを含む）も置く。ファイルを読む guard（`TestInterfaceDocComments`・`TestFakesCarryBuildTag`）は、対象ファイルの集合（件数）が期待どおりであることを先に確認し、空集合で素通りしないようにする。`TestInterfaceDocComments` の英語判定（CJK 文字を含まない、など）は実装時に定義する。
- [x] **ステップ 4-3**: `docs/dev/developer_guide/package_reference.md` に `internal/pipeline` の行を追加する。
- [x] **ステップ 4-4**: 主要な分岐と guard を実装時に壊してテストが失敗することを確認し、コミットメッセージに記録する（例: `Run` の nil 検査を外すと `TestPipelineZeroValueRun` が失敗する、段階の失敗を包まずに返すと `TestPipelineStageError` が失敗する、guard 対象のフィールドを追加すると `TestCommonTypesFieldSets` が失敗する、`//go:build test` を外すと `TestFakesCarryBuildTag` が失敗する、interface の第 1 引数を `context.Context` 以外に変えると `TestInterfaceContracts` が失敗する、interface の契約コメントから条項を削ると `TestInterfaceDocComments` が失敗する、`Secret` に平文を返す公開メソッドを追加すると `TestSecretRevealExclusive` が失敗する、キャンセルの検出点を変えると `TestPipelineCanceled` が失敗する）。
- [x] **ステップ 4-5**: `make fmt` → `make test` → `make lint` を通す。

### PR-4 作成ポイント: pipeline orchestration

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5

**推奨タイトル**: `feat(0001): add pipeline orchestration with stage errors and cancellation`

**レビュー観点**: 段階の実行順・失敗時の打ち切り・キャンセル時の挙動（AC-09〜AC-13） / typed-nil とゼロ値の fail-closed（AC-14） / `Stage` のゼロ値が fail-secure に `"unknown"` を返すこと / guard テストが設計どおりの集合を検証し、空集合で素通りしないこと / 横断 guard（`TestCommonTypesFieldSets`・`TestInterfaceContracts`・`TestInterfaceDocComments`・`TestSecretRevealExclusive`・`TestFakesCarryBuildTag`）がマージ済みの PR-1〜PR-3 の成果物に対して通ること

**実装モデル要件**: frontier-recommended

**判定理由**: キャンセル同期・typed-nil 検出・ゼロ値 fail-closed を含むリスクの高い制御フローの単独ステップ（ステップ 4-1・4-2）を隔離した PR。競合する実装方針の併記はないが、「リスク隔離」の対象となる complex なステップのため。あわせて architecture §7.1 により PR-1〜PR-3 の成果物を検証する横断 guard も本 PR に含む。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: ドキュメント更新

**対象ファイル**
- 変更: `docs/dev/project_overview.md`

**タスク**
- [ ] **ステップ 5-1**: `project_overview.md:33` の `LLMClient` の責務の記述を更新する。
  - 変更前: `- \`LLMClient\`: プロバイダごとの薄いアダプタ。責務は「system プロンプトと user プロンプトを受け取り、テキストを返す」ことだけ。`
  - 変更後: `- \`LLMClient\`: プロバイダごとの薄いアダプタ。責務は「system プロンプトと user プロンプトを受け取り、生成テキストとモデル名を返す」ことだけ。`
- [ ] **ステップ 5-2**: `project_overview.md:35-40` のコード例を更新する。
  - `Generate(ctx context.Context, req GenerateRequest) (string, error)` を `Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)` に変更する。
  - コメント `// GenerateRequest: System, User, MaxOutputTokens, Temperature など、プロバイダ共通の最小限の項目のみ` を `// GenerateRequest: SystemPrompt, UserPrompt, MaxOutputTokens など、プロバイダ共通の最小限の項目のみ` に変更する。
- [ ] **ステップ 5-3**: `project_overview.md:68-83` の想定ディレクトリ構成に `internal/secret/` の行を追加する（例: `internal/secret/          # 秘密情報（API キー・Webhook URL）を保持する型`）。`internal/llm/claude/` の行（`project_overview.md:78`）の直後、`internal/publisher/` の行（`project_overview.md:79`）の直前に追加する。
- [ ] **ステップ 5-4**: 正の確認と旧表記の残骸の確認を行う。
  - 正の確認: `rg -n -e 'GenerateResponse' -e 'SystemPrompt' -e 'UserPrompt' -e 'internal/secret/' docs/dev/project_overview.md` が該当行を返すことを確認し、出力を記録する。
  - 残骸の確認: `rg -n -e '\(string, error\)' -e '\bSystem\b' -e '\bTemperature\b' docs/dev/project_overview.md` が一致なし（exit 1）になることを確認し、出力を記録する（計画作成時の更新前の出力は `:37`・`:39` の 2 件、HEAD `4a2fde5` で確認済み）。`docs/tasks/0001_pipeline_skeleton/` 配下の履歴記述（`01_requirements.md` §5.1、architecture 付録A）は意図的に旧表記のまま残す。
- [ ] **ステップ 5-5**: `make fmt` → `make test` → `make lint` を通す。

### PR-5 作成ポイント: project overview alignment

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5

**推奨タイトル**: `docs(0001): align project overview with the pipeline skeleton design`

**レビュー観点**: `LLMClient` の責務と戻り値の記述が本設計（`GenerateResponse`）と一致すること / 想定ディレクトリ構成に `internal/secret/` が追加されていること / 旧表記（`(string, error)`・`System`・`Temperature`）の残骸がないことを `rg` の出力で確認していること

**実装モデル要件**: standard

**判定理由**: ドキュメントの表記更新のみで、競合する実装方針も高リスクな制御もなく、どのトリガーにも該当しない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 内容 | 成果物 | 完了条件 |
|---|---|---|---|
| M1 | フェーズ 1 | `internal/secret`（`Secret` 型と AC-18〜AC-26 のテスト） | `make test` / `make lint` が通る |
| M2 | フェーズ 2 | 4 つの構成要素パッケージ（`transcript`・`llm`・`writer`・`publisher`）のデータ型と interface | 同上 |
| M3 | フェーズ 3 | 4 つの fake（`testutil/mocks.go`・`mocks_test.go`） | 同上（`-tags test` でコンパイルされる） |
| M4 | フェーズ 4 | `internal/pipeline`（`Stage`・`StageError`・`New`・`Run` と各テスト・guard） | 同上 |
| M5 | フェーズ 5 | `docs/dev/project_overview.md` の更新 | 同上・正の確認と旧表記の残骸なし |

### 3.2. PR 構成

PR はフェーズと 1 対 1 に対応させる。各 PR は主たる関心事（秘密情報型 / 型と interface / fake / パイプライン本体 / ドキュメント）を持ち、単独でグリーンゲートを通せる単位とする。

guard テスト（型・interface・fake の設計適合を検証するテスト）は architecture §7.1 により `internal/pipeline/pipeline_test.go` に集約されるため、PR-1〜PR-3 の成果物に対する guard 検証は PR-4 で入る。したがって、PR-1〜PR-3 のレビューでは guard が後続 PR で入ることを踏まえて設計への適合を目視で確認し、PR-4 ではマージ済みの PR-1〜PR-3 の成果物に対して guard が通ることを確認する。PR-4 はこの横断 guard を併せ持つため、`internal/pipeline` 本体だけでなく他パッケージの契約検証もレビュー対象になる。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | `internal/secret`（`Secret` 型と AC-18〜AC-26 のテスト）、`package_reference.md` 登録 | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 | 4 つの構成要素パッケージのデータ型と interface、`package_reference.md` 登録 | standard |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 | 4 つの fake（`testutil/mocks.go`・`mocks_test.go`）、`package_reference.md` 登録、タグなし `go list` の確認 | standard |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 | `internal/pipeline`（`Stage`（`String()`）・`StageError`・`ErrNilStage`・`New`・`Run`）と振る舞いテスト、PR-1〜PR-3 を検証する横断 guard テスト、`package_reference.md` 登録 | frontier-recommended |
| PR-5 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 | `docs/dev/project_overview.md` の `LLMClient` 周りの更新 | standard |

### 3.3. 実装順序の根拠

architecture §8 の依存の向き（secret → 構成要素パッケージ（transcript・llm → writer → publisher）→ fake → pipeline → ドキュメント）に従う。各フェーズは独立してグリーンゲートを通せる単位とし、package_reference.md の登録は各パッケージを新設するフェーズのコミットに含める（フェーズ 5 にまとめない）。

## 4. テスト戦略 (Test Strategy)

architecture §7 のテスト戦略に従う。

- **ユニットテスト**: 外部コマンド・LLM API・Webhook を一切呼ばない。パイプラインのテストは 4 つの fake（フェーズ 3）を注入して行う。
- **`internal/secret`**: `fmt`・`slog`・`encoding/json`・構造体への埋め込み（公開/非公開フィールド）・ゼロ値・空文字列の各経路を検証する。`fmt` の直接書式化は固定文字列との完全一致で検証し、非公開フィールド経路は元の値が現れないことを検証する。`String()`・`GoString()` の直接呼び出しも個別に検証する。
- **型・interface の検証**: fake のコンパイル時アサーション（AC-06）に加え、`internal/pipeline/pipeline_test.go` に guard テストを置く。検証対象は次の 5 点である。共通データ型（`Segment`・`Transcript`・`GenerateRequest`・`GenerateResponse`・`Article`）の公開フィールドの名前と型の集合（AC-05 と、設計上のフィールド配置（architecture §3.1）の固定。AC-01・AC-02 の検証はこの guard ではなく、下記の `TestPipelineSuccess` で行う）、4 つの interface の第 1 引数（AC-07）、interface のドキュメントコメントの契約条項（AC-08）、`Secret` の公開メソッド集合が設計どおりで平文を返すのが `Reveal` のみであること（AC-21）、`testutil/` 配下の全ファイルの `//go:build test`（AC-17）。`internal/transcript`・`internal/llm`・`internal/writer`・`internal/publisher` にはパッケージ本体の `_test.go` を置かない（architecture §7.1）ため、guard テストを `internal/pipeline/pipeline_test.go` に集約する。
- **統合テスト**: 該当なし。本タスクの成果物は外部と接続する実装を持たない（architecture §7.1）。外部接続を伴う統合は #3・#4・#7 で検証する。
- **セキュリティテスト**: `internal/secret` の AC-18〜AC-26 のテストが、秘密情報が fmt・slog・JSON の各出力経路に現れないことを検証する。パイプラインがエラーに秘密情報を付け加えないことは、`StageError` が元のエラーをそのまま包むことのテストで確認する。
- **テストの実装詳細**（アサーションの書き方、テストデータ）は実装時に決定する。各テストは対応する対策・分岐を壊したときに失敗することを実装時に確認し、コミットメッセージに記録する。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

AC ごとの検証は次のとおり。`test` は実行可能なテスト、`static` は guard テスト・`make` ターゲット・コミット済みスクリプトを指す。

| AC | 内容 | 種別 | 検証の実行場所 |
|---|---|---|---|
| AC-01 | `Transcript` のセグメントが並び順・内容を変えずに次段へ渡る | test | `internal/pipeline/pipeline_test.go::TestPipelineSuccess`（source fake が複数のセグメントを順序つきで返し、writer fake が記録した `Transcript` のセグメントが同じ並び・内容であることを検証する） |
| AC-02 | `Transcript` のメタ情報が次段へ保たれる | test | `internal/pipeline/pipeline_test.go::TestPipelineSuccess`（writer fake が記録したメタ情報 5 項目が一致することを検証する） |
| AC-03 | LLM の応答がテキストとモデル名を呼び出し元へ返す | test | #4 のアダプタテスト（実際に使われたモデル名の記録）と #5 の `Article.Model` への引き継ぎ。本タスクには `GenerateResponse` を生成・消費するコードがないため、ここでは検証しない |
| AC-04 | `Article` がタイトル・本文・出典リンク・モデル名を保持 | test / static | `internal/pipeline/pipeline_test.go::TestPipelineSuccess` ＋ `TestCommonTypesFieldSets`（`Article` の公開フィールドの名前と型の集合） |
| AC-05 | 共通型にプロバイダ固有の項目・SDK 型を含めない | static | `internal/pipeline/pipeline_test.go::TestCommonTypesFieldSets` ＋ `make lint`（depguard の SDK 閉じ込めルール、`.golangci.yml:63-86`） |
| AC-06 | 4 つの interface が定義され、入力・出力・エラーを返す | static | 各 `testutil/mocks.go` の `var _ <interface> = (*<Fake>)(nil)` コンパイル時アサーション（`make test` が評価） |
| AC-07 | 全メソッドの第 1 引数が `context.Context` | static | `internal/pipeline/pipeline_test.go::TestInterfaceContracts`（リフレクションで検証） |
| AC-08 | interface に英語の契約ドキュメントコメント | static | `internal/pipeline/pipeline_test.go::TestInterfaceDocComments`（4 つの interface のソースファイルを読み、契約条項と英語であることを検証） |
| AC-09 | 段階が順に 1 回ずつ呼ばれ出力が伝播 | test | `internal/pipeline/pipeline_test.go::TestPipelineSuccess` |
| AC-10 | 成功時に投稿した `Article` を返す | test | `internal/pipeline/pipeline_test.go::TestPipelineSuccess` |
| AC-11 | 失敗時に以降の段階を呼ばない | test | `internal/pipeline/pipeline_test.go::TestPipelineStageFailure` |
| AC-12 | `StageError` で段階を判別し元のエラーを辿る | test | `internal/pipeline/pipeline_test.go::TestPipelineStageError` |
| AC-13 | キャンセル時に次の段階を呼ばず `context.Canceled` を返す | test | `internal/pipeline/pipeline_test.go::TestPipelineCanceled`（段階間キャンセルは決定的な同期手段で検証） |
| AC-14 | nil 段階（typed-nil を含む）の構築を拒否 | test | `internal/pipeline/pipeline_test.go::TestPipelineNewNilStage` |
| AC-14a | ゼロ値の `Pipeline` の `Run` を拒否（段階を呼ばない） | test | `internal/pipeline/pipeline_test.go::TestPipelineZeroValueRun` |
| AC-15 | fake の戻り値・エラー指定 | test | 各 `internal/<pkg>/testutil/mocks_test.go` |
| AC-16 | fake の呼び出し記録 | test | 各 `internal/<pkg>/testutil/mocks_test.go` |
| AC-17 | fake が `//go:build test` でのみビルドされる | static | `internal/pipeline/pipeline_test.go::TestFakesCarryBuildTag`（`testutil/` 配下の全 `.go` ファイルを検証）＋ステップ 3-7 の `go list`（タグなしで `testutil` の `GoFiles` が 0 件、かつ `.Error` がビルド制約による除外を示す） |
| AC-18 | `fmt` の委譲される指定子で `[REDACTED]` | test | `internal/secret/secret_test.go::TestSecretFmtRedaction` |
| AC-19 | slog 属性で元の値が出ない | test | `internal/secret/secret_test.go::TestSecretSlogRedaction` |
| AC-20 | JSON エンコードで元の値が出ない | test | `internal/secret/secret_test.go::TestSecretJSONRedaction` |
| AC-21 | `Reveal()` のみが元の値を返す | test / static | `internal/secret/secret_test.go::TestSecretReveal` ＋ `internal/pipeline/pipeline_test.go::TestSecretRevealExclusive`（公開メソッド集合の許可リスト照合） |
| AC-22 | `New("")` がエラー | test | `internal/secret/secret_test.go::TestSecretNewEmpty` |
| AC-23 | ゼロ値の `Reveal()` がエラー | test | `internal/secret/secret_test.go::TestSecretZeroValueReveal` |
| AC-24 | `String()`・`GoString()` の直接呼び出し | test | `internal/secret/secret_test.go::TestSecretStringGoString` |
| AC-25 | 委譲されない書式指定子でも元の値が出ない | test | `internal/secret/secret_test.go::TestSecretFmtNoDelegateVerbs` |
| AC-26 | 非公開フィールド経由の `fmt` 出力 | test | `internal/secret/secret_test.go::TestSecretUnexportedFieldNoLeak` |

**AC-01〜AC-03 の検証方法**: AC-01（セグメントの並び順）と AC-02（メタ情報）は、段階間の受け渡しで値が保たれるという観測可能な振る舞いであり、`TestPipelineSuccess` で検証する。AC-03（LLM 応答のテキストとモデル名）を生成・消費するコードは本タスクにないため、`requirements_process.md` の方針に従い挙動検証を #4（実際に使われたモデル名の記録）・#5（`Article.Model` への引き継ぎ）に引き継ぎ、本タスクでは `GenerateResponse` のフィールド配置（architecture §3.1）を設計制約として固定するだけにする。

**AC に対応しない補助テスト**: `internal/pipeline/pipeline_test.go::TestStageString` が `Stage.String()` のゼロ値・既知の値・未知の値と、`StageError.Error()` が `Stage.String()` を使うことを検証する（architecture §3.4 の fail-secure な既定値。AC には直接対応しない）。

**横断検索項目**（`make test` / `make lint` では検出できないもの）: `docs/dev/project_overview.md` の旧表記（`(string, error)`・`System`・`Temperature`）の残骸確認をフェーズ 5 で行う。`docs/tasks/0001_pipeline_skeleton/` 配下の履歴記述は意図的に残す。

## 6. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| `fmt`・slog・`encoding/json` の書式指定子や埋め込みパターンで挙動が想定と異なる | AC-18〜AC-20 の不成立 | 計画作成時にローカル Go 1.27.1 で挙動を確認済み（§1.3）。テストは委譲される指定子・委譲されない指定子・公開/非公開フィールドの各経路をカバーする |
| typed-nil の interface が `Run` 実行時にパニックを起こす | AC-14・AC-14a の不成立 | `New` と `Run` の両方で nil を検出し、構築時・使用時に `ErrNilStage` を返す。テストで 3 引数すべての typed-nil とゼロ値を検証する |
| `Run` を `New` を通さないゼロ値で呼ぶ | パニック | `Run` の冒頭で段階の nil を再確認する（architecture §3.4 のゼロ値の fail-closed） |
| lint（revive の `exported`・`package-comments`、goconst、err113、mnd）が新設コードで指摘を出す | `make lint` が通らない | パッケージコメント・公開識別子への英語ドキュメントコメント、`[REDACTED]` のパッケージ定数化、静的センチネルの使用をタスクに含めた。テスト内の数値リテラルに mnd が出た場合は定数化または必要最小限の `//nolint:mnd` で対応する |
| 4 つの `testutil/mocks.go` は非テストファイルのため `.golangci.yml` の `_test.go` 除外が効かず、`dupl`（近似コード）が発火しうる | PR-3 の `make lint` が通らない | 4 つの fake は意図的に同型である。`dupl` が出た場合は理由付きの `//nolint:dupl` を付ける。`_test.go` 除外の対象外であることを前提に PR-3 のゲートで確認する |
| AC-13 の段階間キャンセルのテストが racy になる | テストが間欠的に失敗する | 決定的な同期手段（カスタム `context.Context`）を実装時に使い、タイマー待ち・スリープを使わない |
| guard テストが部分一致のような弱い検証で素通りする | AC の検証が成立しない | guard は「設計どおりの集合・完全契約」を検証する内容とし、実装時に壊して失敗することを確認してコミットメッセージに記録する |
| 将来の interface シグネチャ変更が fake を壊す | コンパイルエラー | fake のコンパイル時アサーションが `make test` で検出する |
| `project_overview.md` の旧表記が残る | ドキュメントの不整合 | フェーズ 5 で正の確認と残骸確認を `rg` で行い、出力を記録する |

## 7. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
- [x] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7）
- [x] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8）
- [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5）
- [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5）
- [ ] 各 PR で `make fmt` → `make test` → `make lint` が通る
- [ ] 各テストについて、対応する対策・分岐を実装時に壊し、テストが失敗することを確認済み（コミットメッセージに記録）
- [ ] `docs/dev/developer_guide/package_reference.md` に 10 パッケージが登録されている
- [ ] `cmd/yt2column/main.go` が変更されていない

## 8. 成功基準 (Success Criteria)

- 全 AC（AC-01〜AC-26 と AC-14a）に、§5 のとおり `test` または `static` の検証がある（AC-03 は挙動検証を #4 に引き継ぐ）。
- `make test` と `make lint` が通る（green gate）。
- 本番バイナリに fake が含まれない（`TestFakesCarryBuildTag` と `make build` の対象が `./cmd/yt2column` のみであることで確認）。
- `docs/dev/project_overview.md` の `LLMClient` の記述が本設計に一致し、旧表記の残骸がない。`docs/dev/developer_guide/package_reference.md` に全新設パッケージが登録されている。
- `cmd/yt2column/main.go` が変更されていない（CLI 配線は #6）。

## 9. 次のステップ (Next Steps)

- 本計画は `approved`。PR 境界は §2 の `PR-N 作成ポイント` と §3.2 に埋め込み済み。
- `/runplan 0001` で PR の順に実装する（各 PR は独立してグリーンゲートを通す）。
- 本タスク完了後、後続タスク（#3・#4・#5・#6・#7）が、本タスクで定めた型・interface・fake に依存して並行開発できる。
