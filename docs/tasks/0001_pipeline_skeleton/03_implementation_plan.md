# 実装計画書：パイプラインの骨格

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-09-28 |
| Review date | `-` |
| Reviewer | `-` |
| Comments | `-` |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`01_requirements.md` で定義したパイプラインの骨格を実装する。段階間で受け渡す共通データ型、4 つの interface、オーケストレーション（`internal/pipeline`）、テスト用 fake、秘密情報を保持する `Secret` 型を、`02_architecture.md`（以下「architecture」）の設計どおりに新設する。これにより後続タスク（#3・#4・#5・#6・#7）が fake を相手に単体で開発・テストできる状態を作る。

### 1.2. 実装原則

- architecture §1.1 の設計原則 5 項目に従う。
- 段階の責務は各パッケージに閉じ込め、パイプラインは interface だけに依存する（architecture §1.1-1）。
- プロバイダ固有の知識を共通型・interface・パイプラインに持ち込まない（AC-05、architecture §1.1-3）。
- ユニットテストは外部コマンド・LLM API・Webhook を一切呼ばない。テストは fake と標準ライブラリのみで行う。
- 各フェーズの完了時に `make fmt` → `make test` → `make lint` を実行し、通した状態で進める（フェーズ 1 の前提タスクでローカル環境の lint 実行を可能にする）。
- 各テストは「対応する対策や分岐を壊すと失敗すること」を実装時に壊して確認し、その旨をコミットメッセージに記録する（CLAUDE.md「Testing Strategy」）。

### 1.3. 既存コード調査結果

リポジトリの現状を HEAD `73e8b77` で確認した。

- **Go ソース**: `cmd/yt2column/main.go` のみ（空の `main()`、`cmd/yt2column/main.go:4`）。`internal/` 以下のパッケージ・テスト・テストヘルパーは存在せず、再利用できる既存コンポーネントはない。本タスクで変更する既存テストもない。
- **package_reference.md**: `docs/dev/developer_guide/package_reference.md:7` が「No packages exist yet」のまま。本タスクで新設する 10 パッケージ（`internal/secret`・`internal/transcript`・`internal/transcript/testutil`・`internal/llm`・`internal/llm/testutil`・`internal/writer`・`internal/writer/testutil`・`internal/publisher`・`internal/publisher/testutil`・`internal/pipeline`）を登録する。登録は同ファイル冒頭の「パッケージを追加するコミットと同じコミットで更新する」規則に従い、各パッケージを新設するフェーズで行う（フェーズ 5 にまとめない）。最初の行を追加するフェーズ 1 では、冒頭の「No packages exist yet」の記述と「Move each entry here」の案内も更新する。
- **project_overview.md**: `LLMClient` の例（`docs/dev/project_overview.md:35-40`）が、戻り値 `(string, error)` とフィールド名 `System`・`User`・`MaxOutputTokens`・`Temperature` の旧表記のまま。architecture 付録A のとおり `(GenerateResponse, error)` と `SystemPrompt`・`UserPrompt`・`MaxOutputTokens` に更新する（フェーズ 5）。あわせて責務の記述（`project_overview.md:33`）と、想定ディレクトリ構成（`project_overview.md:68-83`、`internal/secret/` が未記載）を更新する。
- **`Secret` の構築経路**: 新設の `internal/secret` は葉パッケージで `value func() string` を非公開フィールドに持ち、公開コンストラクタは `New` のみ。構築経路は `New` とゼロ値の 2 経路に限定され、ゼロ値は `Reveal()` がエラーを返して拒否する（AC-23）。`Secret` 型と `New` を同一ファイルに置くことで、同一パッケージ内での composite literal による直接構築を、目視で確認できる範囲に限定する（実装方針。architecture §3.3 の設計に追加の構造は生じない）。等値判定は要件がないため追加しない（YAGNI、architecture §9）。
- **外部ツール・環境の前提**:
  - 新規の外部依存はない。`make test` は `go test -tags test -race`、`make lint` は `golangci-lint --build-tags test` で実行され、`//go:build test` の fake は両方のゲートでコンパイルされる。本番バイナリの `make build` は `./cmd/yt2column` のみをビルドする。本番バイナリに fake が含まれないことは、フェーズ 4 の guard テスト（設計どおりの内容を守る回帰防止テスト。§4 を参照）で検証する（AC-17）。
  - **ローカル環境の lint 非互換（要修正）**: ローカルの Go は 1.27.1（`/opt/homebrew/bin/go`）だが、`go.mod` が宣言する最低バージョンは `go 1.26.5`。リポジトリがピンする golangci-lint v2.11.4（`Makefile:9`・`.pre-commit-config.yaml`・`.github/workflows/ci.yml` の 3 箇所）は go1.27 の export data を読めず、`fmt` を import するパッケージを解析すると「cannot decode, export data version 4 is greater than maximum supported version 2」で失敗する（2026-09-28 にローカルで再現確認。`.golangci.yml` の設定はそのまま）。CI は `setup-go` の `go-version-file` で go 1.26.5 になるため影響しないが、ローカルの `make lint` は新設パッケージ（stdlib を import する）で必ず失敗する。このままでは各フェーズの完了条件「`make lint` を通す」を満たせないため、フェーズ 1 の前提タスクで golangci-lint を go1.27 対応版へ更新する。
- **検証済みの Go の挙動**: 本計画のテスト設計が依存する Go 標準ライブラリの挙動を、ローカルの Go 1.27.1 で動作確認した（2026-09-28。これらの挙動は go 1.26〜1.27 で長く安定している）。
  - `fmt` は `Formatter` を実装した値について、`%s`・`%v`・`%+v`・`%#v`・`%q`・`%d`・`%x`・`%c`・`%U`・`%b`・`%e` などの指定子で `Format` を呼ぶ。`%p`（非ポインタ値）・非 error への `%w`・`%T` は `Format` を呼ばず、それぞれ `%!p(...)`・`%!w(...)`・型名の形になる。
  - 非公開フィールドに `Secret` を持つ構造体を `%+v`・`%#v` で表示すると `{k:{value:0x...}}` の形になり、クロージャの関数アドレスのみが出力され元の値は現れない。
  - slog の TextHandler は、属性として渡した `Secret` を `LogValuer` で解決して `[REDACTED]` を出力し、非公開フィールド経由の構造体では `fmt` と同様にリフレクション表示（関数アドレスのみ）になる。
  - `encoding/json` は `MarshalJSON` により `"[REDACTED]"` を出力し、非公開フィールドは出力に含めない。
  - `//go:build test` のみを持つパッケージは、タグなしの `go build <パッケージパス>` で「build constraints exclude all Go files」エラーになる（`./...` は黙ってスキップするため、明示パスでの確認が必要）。

## 2. 実装ステップ (Implementation Steps)

各フェーズの完了条件は「そのフェーズの `make fmt` → `make test` → `make lint` が通ること」。architecture §8 の実装優先順位に沿い、依存の向きの下から順に実装する。フェーズ 1 の冒頭の前提タスク（golangci-lint の更新）はローカル環境の前提を整えるためのもので、architecture §8 の優先順位には含まれない（§1.3 参照）。

### フェーズ 1: `internal/secret`（秘密情報型）

**前提タスク（ローカル環境の lint 実行基盤の整備）**

後続フェーズ（2〜5）の完了条件「`make lint` を通す」をローカルで満たすための前提。前提は、その前提が必要になるフェーズの冒頭に記載する（本計画のレビュー規則。後続フェーズの前提はそのフェーズ内に記載する）。この前提タスクはツールチェーンの更新のみを 1 コミットに収め、本フェーズの `internal/secret` 以降のタスクとは分離する。

**対象ファイル**
- 変更: `Makefile`・`.pre-commit-config.yaml`・`.github/workflows/ci.yml`

**タスク**
- [ ] golangci-lint のピンを v2.11.4 から v2.13.0 に更新する。`Makefile:9` の `GOLANGCI_VERSION`・`.pre-commit-config.yaml` の golangci-lint エントリ・`.github/workflows/ci.yml` の version ピンの 3 箇所を、`Makefile:5-9` のコメント「Bump all three pins together」どおり同時に更新する。
  - 根拠: golangci-lint の CHANGELOG で v2.13.0（2026-08-19 リリース）が「🎉 go1.27 support」を追加したことを確認した（github.com/golangci/golangci-lint CHANGELOG.md）。go1.26 対応は v2.9.0 で追加されており、v2.11.4 は go1.26 まで対応するが、go1.27 には未対応。ローカルの Go 1.27.1 と v2.11.4 の組み合わせで解析が失敗することは §1.3 のとおり確認済み。
- [ ] `make lint` を実行し、`fmt` を import する新設パッケージを解析しても失敗しないことを確認する。
- [ ] `make test` が通ることを確認する。
- [ ] 前提タスクのコミット（golangci-lint 更新のみ）をタスク本体のコミットから分離する。

**対象ファイル**
- 新設: `internal/secret/secret.go`・`internal/secret/secret_test.go`
- 変更: `docs/dev/developer_guide/package_reference.md`

**タスク**
- [ ] `internal/secret/secret.go` を作成し、architecture §3.3 の型定義どおりに実装する。
  - 元の値を `func() string` クロージャで保持する（要件 §5 の制約。非公開フィールド経由のリフレクション表示を防ぐ）。
  - `New(value string) (Secret, error)`: 空文字列はエラー（AC-22）。拒否のエラーは静的（定数文字列）な `errors.New` によるパッケージレベルのセンチネルとして返す（err113 は `%w` なしの `fmt.Errorf` と動的エラーを拒否する）。
  - `Reveal() (string, error)`: ゼロ値（nil クロージャ）はエラー（AC-23）。元の値を返す経路はこのメソッドのみ（AC-21）。
  - `Format(f fmt.State, verb rune)`: すべての書式指定子で `[REDACTED]` を含む固定文字列を書き、元の値を決して出力しない（AC-18）。`%q` は引用符付きの表現にする。
  - `LogValue() slog.Value`: 属性として `[REDACTED]` を返す（AC-19）。
  - `MarshalJSON() ([]byte, error)`: `"[REDACTED]"` を返す（AC-20）。
  - パッケージコメントとすべての公開識別子に英語のドキュメントコメントを付ける（revive の `package-comments` と `exported` ルールが `make lint` で検査）。`[REDACTED]` の繰り返しは goconst の指摘を避けるためパッケージ定数にまとめる。
- [ ] `internal/secret/secret_test.go` を作成し、AC-18〜AC-23 のテストを置く。
  - `TestSecretFmtRedaction`（AC-18）: 主要な書式指定子（`%s`・`%v`・`%+v`・`%#v`・`%q`・`%d`・`%x`）・あまり使われない指定子（`%c`・`%U`・`%b`・`%e` など）・未知の指定子で、`Secret` 単体と公開フィールドとして含む構造体の両方について、出力が**固定文字列と完全一致**することを検証（例: `%s`・`%v` は `[REDACTED]`、`%q` は引用符付きの `"[REDACTED]"`）。負の性質: 固定文字列以外の何かを出力する指定子（`[REDACTED]-<長さ>`・`[REDACTED]-<ハッシュ>`・引用符の欠落など）は失敗する。
  - `TestSecretFmtNoDelegateVerbs`（AC-18）: `fmt.Formatter` に委譲されない指定子（非ポインタ値への `%p`、非 error への `%w`）で元の値が出力に現れないことを検証。
  - `TestSecretUnexportedFieldNoLeak`（AC-18）: 非公開フィールドとして含む構造体を `%v`・`%+v`・`%#v` で出力し、元の値が現れないことを検証（出力される関数アドレス `0x...` 自体は検証しない。architecture §7.1）。
  - `TestSecretSlogRedaction`（AC-19）: slog 属性の直接出力と公開フィールドの構造体で `[REDACTED]` が現れ、非公開フィールドの構造体では元の値が現れないことを検証。
  - `TestSecretJSONRedaction`（AC-20）: `json.Marshal` で `"[REDACTED]"` が出力され、公開フィールドの構造体でも同様、非公開フィールドの構造体ではフィールドが省略されることを検証。
  - `TestSecretReveal`（AC-21）・`TestSecretNewEmpty`（AC-22）・`TestSecretZeroValueReveal`（AC-23）。
  - 各テストが、対応する対策（`Format` 実装・クロージャ保持・空値チェック・ゼロ値チェック）を壊すと失敗することを実装時に確認し、コミットメッセージに記録する。
- [ ] `docs/dev/developer_guide/package_reference.md` に `internal/secret` の行を追加し、冒頭の「No packages exist yet」の記述と「Move each entry here」の案内を更新する。追加後、`rg -n 'internal/secret' docs/dev/developer_guide/package_reference.md` が一致することを確認する。
- [ ] `make fmt` → `make test` → `make lint` を通す。

### フェーズ 2: 葉パッケージのデータ型と interface

**対象ファイル**
- 新設: `internal/transcript/transcript.go`・`internal/llm/llm.go`・`internal/writer/writer.go`・`internal/publisher/publisher.go`
- 変更: `docs/dev/developer_guide/package_reference.md`

**タスク**
- [ ] `internal/transcript/transcript.go` を作成し、`Segment`・`Transcript` を architecture §3.1 のとおり定義する（AC-01・AC-02）。`Segments` は出現順を保って保持する。
- [ ] `internal/llm/llm.go` を作成し、`GenerateRequest`・`GenerateResponse` を architecture §3.1 のとおり定義する（AC-03・AC-05）。プロバイダ固有の項目（`thinking`・`reasoning_content` など）や SDK の型は含めない。
- [ ] `internal/writer/writer.go` を作成し、`Article` を architecture §3.1 のとおり定義する（AC-04）。
- [ ] 4 つの interface（`TranscriptSource`・`LLMClient`・`ArticleWriter`・`Publisher`）を architecture §3.2 の Go 定義どおりに定義する（AC-06・AC-07）。すべてのメソッドの第 1 引数は `context.Context`。
  - 契約（失敗時にエラーを返す・空の結果を正常な結果として返さない）を、英語のドキュメントコメントとして各 interface に書く（AC-08）。コメントの文言は architecture §3.2 の例に従う。
  - 4 パッケージそれぞれに英語のパッケージコメントを付ける（revive の `package-comments`）。
- [ ] この 4 パッケージにパッケージ本体の `_test.go` は置かない（architecture §7.1。型の検証はフェーズ 3・4 の fake とフローテスト・guard テストで行う）。
- [ ] `docs/dev/developer_guide/package_reference.md` に `internal/transcript`・`internal/llm`・`internal/writer`・`internal/publisher` の 4 行を追加する。追加後、`rg` で各パッケージパスが一致することを確認する。
- [ ] `make fmt` → `make test` → `make lint` を通す。

### フェーズ 3: テスト用 fake

**対象ファイル**
- 新設: `internal/transcript/testutil/mocks.go`・`internal/transcript/testutil/mocks_test.go`
- 新設: `internal/llm/testutil/mocks.go`・`internal/llm/testutil/mocks_test.go`
- 新設: `internal/writer/testutil/mocks.go`・`internal/writer/testutil/mocks_test.go`
- 新設: `internal/publisher/testutil/mocks.go`・`internal/publisher/testutil/mocks_test.go`
- 変更: `docs/dev/developer_guide/package_reference.md`

**タスク**

共通パターン（4 つの fake すべてに適用）:
- `mocks.go`・`mocks_test.go` の両方の先頭に `//go:build test` を付ける（AC-17。test_organization.md の「すべてのテストヘルパーファイルに build tag」規則）。
- パッケージ名は `docs/dev/developer_guide/test_organization.md` の分類 A に従い `<domain>testutil`（`transcripttestutil`・`llmtestutil`・`writertestutil`・`publishertestutil`）とする。
- fake は設定可能な `Result` と `Err`、呼び出し記録の `Calls []<Fake>Call`（`Ctx` と引数を持つ）を持つ。メソッドは**ポインタレシーバ**で定義し、呼び出しごとに引数を `Calls` に追記する（AC-15・AC-16）。
- コンパイル時アサーション `var _ <interface> = (*<Fake>)(nil)` を置く（AC-06 の検証）。
- パッケージコメントとすべての公開識別子に英語のドキュメントコメントを付ける（revive の `package-comments` と `exported` ルール）。

- [ ] `internal/transcript/testutil/mocks.go` に `FakeTranscriptSource` を作成する（記録する引数は `VideoURL`）。
- [ ] `internal/transcript/testutil/mocks_test.go` に fake の振る舞いテスト（指定した `Transcript` とエラーを返す・呼び出しを記録する）を作成する。
- [ ] `internal/llm/testutil/mocks.go` に `FakeLLMClient` を作成する（記録する引数は `GenerateRequest`）。
- [ ] `internal/llm/testutil/mocks_test.go` に fake の振る舞いテスト（指定した `GenerateResponse` とエラーを返す・呼び出しを記録する）を作成する。
- [ ] `internal/writer/testutil/mocks.go` に `FakeArticleWriter` を作成する（記録する引数は `Transcript`）。
- [ ] `internal/publisher/testutil/mocks.go` に `FakePublisher` を作成する（記録する引数は `Article`）。
- [ ] 各 `mocks_test.go` が、fake の振る舞い（戻り値・エラー・呼び出し記録）を壊すと失敗することを実装時に確認し、コミットメッセージに記録する。
- [ ] `docs/dev/developer_guide/package_reference.md` に `testutil` 4 パッケージの行を追加する。追加後、`rg` で各パッケージパスが一致することを確認する。
- [ ] `make fmt` → `make test` → `make lint` を通す（`make test` が `-tags test` で fake をコンパイルすることを確認する）。

### フェーズ 4: `internal/pipeline`

**対象ファイル**
- 新設: `internal/pipeline/pipeline.go`・`internal/pipeline/pipeline_test.go`
- 変更: `docs/dev/developer_guide/package_reference.md`

**タスク**
- [ ] `internal/pipeline/pipeline.go` を作成し、architecture §3.4 の定義どおりに実装する。
  - `Stage` 列挙型（ゼロ値 `StageUnknown`）と `String()`。未知の値とゼロ値には `"unknown"` を返す。`Error()` は必ず `String()` を使う。
  - `StageError`（`Stage`・`Err`）と `Error()`・`Unwrap()`。エラーメッセージは `"<段階名>: <元のエラー>"` の形式とし、元のエラー以外の情報を付け加えない（AC-12、要件 §4.2）。
  - センチネルエラー `ErrNilStage`。
  - `Pipeline` 構造体と `New`・`Run`。
  - `New` は各段階の nil（interface 値の `== nil`）と typed-nil（リフレクションで格納値が nil）を検出し、`ErrNilStage` を返す（AC-14）。typed-nil の判定は nil を許容するすべての kind（ptr・func・map・slice・chan）に `reflect.Value.IsNil` を適用して行い、ポインタに限定しない。nil を含むパイプラインを構築できない。
  - `Run` は各段階の呼び出し前に `ctx.Err()` を確認し、キャンセル済みなら `ctx.Err()` をそのまま返して以降の段階を呼ばない（AC-13）。段階の失敗は対応する `Stage` を付けた `StageError` で包む（AC-11・AC-12）。すべて成功した場合は投稿した `Article` を返す（AC-09・AC-10）。
  - パッケージコメントとすべての公開識別子に英語のドキュメントコメントを付ける。
- [ ] `internal/pipeline/pipeline_test.go` を作成し、AC-09〜AC-14・AC-17 のテストと、guard テスト（AC-03・AC-05・AC-07・AC-08・AC-21・`Stage.String()`）を置く。テストはフェーズ 3 の fake を注入して行う。guard テストは、対応する AC が守っていることを弱い代理（`[REDACTED]` の部分一致・フィールド名のみの比較・ドキュメントコメントの存在確認など）ではなく**完全契約**で検証し、検査が強制する負の性質（AC の違反がビルドを失敗させること）を明記する。
  - `TestPipelineSuccess`（AC-01・AC-02・AC-04・AC-09・AC-10）: 3 段階が順に 1 回ずつ呼ばれ、前段の出力が次段に同じ値で渡り、成功時に `Run` が投稿した `Article` を返すことを、fake の `Calls` と戻り値で検証。
  - `TestPipelineStageFailure`（AC-11）: 各段階が失敗した場合に以降の段階が呼ばれないことを、失敗する段階を切り替えたテーブルで検証。
  - `TestPipelineStageError`（AC-12）: `errors.AsType[*pipeline.StageError]` で失敗した段階を判別でき、`errors.Is` で段階が返した元のエラーを辿れることを検証。
  - `TestPipelineCanceled`（AC-13）: Run 開始前にキャンセル済みの場合にどの段階も呼ばれず `context.Canceled` を返すことと、段階間でキャンセルされた場合に以降の段階を呼ばず `context.Canceled` を返すことの両方を検証。段階間のキャンセルは、Fetch の呼び出しを観測してから `ctx.Err()` が `context.Canceled` を返すよう切り替えるカスタム `context.Context`（`pipeline_test.go` 内で定義）など、**決定的な同期手段**で実現する。タイマー待ちやスリープによる racy な方法は使わない。fake に共通のフィールド構成（`Result`・`Err`・`Calls`）は変更しない。
  - `TestPipelineNewNilStage`（AC-14）: 通常の nil（interface 値の nil）と typed-nil（例: 値が nil の `*publishertestutil.FakePublisher`、値が nil の名前付き関数型）を、3 つの引数（source・writer・publisher）それぞれについてテーブルで検証し、`New` が `ErrNilStage` で拒否することを確認する。typed-nil のテーブルにはポインタに加えて、いずれかの段階 interface を実装する名前付き関数型の nil 値（kind が func の typed-nil）を少なくとも 1 件含める。
  - `TestStageString`: `Stage` のゼロ値（`StageUnknown`）・既知の値（`StageTranscript`・`StageWrite`・`StagePublish`）・範囲外の値それぞれで `String()` が正しい文字列を返し、`StageError.Error()` が段階名を含むことを検証（architecture §3.4 の fail-secure な既定値）。
  - `TestCommonTypesFieldSets`（AC-03・AC-05、guard）: リフレクションで共通データ型の公開フィールドの（名前, 型）ペアの集合が、設計どおりの正確な集合（`Transcript` は `VideoID string`・`VideoURL string`・`Title string`・`ChannelName string`・`Description string`・`Segments []Segment`、`Segment` は `StartMs int64`・`Text string`、`GenerateRequest` は `SystemPrompt string`・`UserPrompt string`・`MaxOutputTokens int`、`GenerateResponse` は `Text string`・`Model string`、`Article` は `Title string`・`Body string`・`SourceURL string`・`Model string`）であることを検証。フィールド名だけでなく各フィールドの正確な Go 型（`reflect.Type`）も名前と一緒に比較する。負の性質: `GenerateRequest` に余分なフィールドを足すこと、既存フィールドの型を変えること（例: `GenerateResponse.Model` を `string` 以外の型にする）で失敗する。この guard が実際に失敗することを実装時に確認し、コミットメッセージに記録する。
  - `TestInterfaceContracts`（AC-07、guard）: リフレクションで 4 つの interface の各メソッドの第 1 引数の型が `context.Context` であることを検証。この guard が、第 1 引数から `context.Context` を外すと失敗することを実装時に確認し、コミットメッセージに記録する。
  - `TestInterfaceDocComments`（AC-08、guard）: 4 つの interface のソースファイル（`internal/transcript/transcript.go`・`internal/llm/llm.go`・`internal/writer/writer.go`・`internal/publisher/publisher.go`）を読み、各 interface のドキュメントコメントが、その interface に要求される契約条項（失敗時にエラーを返す条項と、値返しの interface では空の結果を正常な結果として返さない条項。architecture §3.2 の文言）を含み、英語である（CJK 文字を含まない）ことを検証。負の性質: コメントの削除・日本語化・契約条項の欠落で失敗する。この guard が、`make lint`（revive の `exported`）では検出できない契約内容の欠落を検出することを実装時に確認し、コミットメッセージに記録する。
  - `TestSecretRevealExclusive`（AC-21、guard）: リフレクションで `secret.Secret` と `*secret.Secret` の両方の公開メソッド集合を調べ、設計が実際に定義するメソッド（`New`・`Reveal`・`Format`・`LogValue`・`MarshalJSON`。平文文字列を返すのは `Reveal` のみ）の明示的な許可リストと照合し、許可リストにない公開メソッドが存在しないことを検証する。戻り値のシグネチャ（`(string, error)`）だけからの推測はしない。負の性質: 許可リストにない公開アクセサを追加すると失敗する（`Value() string`・`Bytes() []byte` のような値レシーバのアクセサに限らず、ポインタレシーバのアクセサも含む）。この guard が、`Reveal` 以外の平文取得経路を追加すると失敗することを実装時に確認し、コミットメッセージに記録する。
  - `TestFakesCarryBuildTag`（AC-17、guard）: 4 つの `testutil/mocks.go` を、`go test` のワーキングディレクトリ（パッケージディレクトリ）からの相対パスで読み、先頭に `//go:build test` があることを検証する。
  - 各テストが、対応する分岐（段階停止・キャンセル検査・nil 判定・guard の対象）を壊すと失敗することを実装時に確認し、コミットメッセージに記録する。
- [ ] `docs/dev/developer_guide/package_reference.md` に `internal/pipeline` の行を追加する。追加後、`rg` でパッケージパスが一致することを確認する。
- [ ] `make fmt` → `make test` → `make lint` を通す。

### フェーズ 5: ドキュメント更新

**対象ファイル**
- 変更: `docs/dev/project_overview.md`

**タスク**
- [ ] `project_overview.md:33` の `LLMClient` の責務の記述を更新する。
  - 変更前: `- \`LLMClient\`: プロバイダごとの薄いアダプタ。責務は「system プロンプトと user プロンプトを受け取り、テキストを返す」ことだけ。`
  - 変更後: `- \`LLMClient\`: プロバイダごとの薄いアダプタ。責務は「system プロンプトと user プロンプトを受け取り、生成テキストとモデル名を返す」ことだけ。`
- [ ] `project_overview.md:35-40` のコード例を更新する。
  - `Generate(ctx context.Context, req GenerateRequest) (string, error)` を `Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)` に変更する。
  - コメント `// GenerateRequest: System, User, MaxOutputTokens, Temperature など、プロバイダ共通の最小限の項目のみ` を `// GenerateRequest: SystemPrompt, UserPrompt, MaxOutputTokens など、プロバイダ共通の最小限の項目のみ` に変更する。
- [ ] `project_overview.md:68-83` の想定ディレクトリ構成に `internal/secret/` の行を追加する（例: `internal/secret/          # 秘密情報（API キー・Webhook URL）を保持する型`）。`internal/llm/` 系の行と `internal/publisher/` の間に挿入する。
- [ ] 更新内容の**正の確認**と旧表記の**残骸の確認**の両方を行う。
  - 正の確認: `project_overview.md` に `GenerateResponse`・`SystemPrompt`・`UserPrompt`・`MaxOutputTokens`・`internal/secret/` が現れることを `rg` で確認し、出力を記録する。
  - 残骸の確認: 次のコマンドが一致なし（exit 1）になることを確認し、出力を記録する。`rg -n -e '\(string, error\)' -e '\bSystem\b' -e '\bTemperature\b' docs/dev/project_overview.md`（`\bSystem\b` は `SystemPrompt` に一致しない。計画作成時の更新前の出力は 2 件: `project_overview.md:37`・`project_overview.md:39`、HEAD `73e8b77` で確認）。`docs/tasks/0001_pipeline_skeleton/` 配下の履歴記述（要件 §5.1、architecture 付録A）は意図的に旧表記のまま残す。
- [ ] `make fmt` → `make test` → `make lint` を通す。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

| マイルストーン | 内容 | 成果物 | 完了条件 |
|---|---|---|---|
| M1 | フェーズ 1 | `internal/secret`（`Secret` 型と AC-18〜23 のテスト）。前提タスクとして golangci-lint の v2.13.0 への更新（3 箇所のピン）を含む | `make test` / `make lint` が通る。前提タスク完了時点でローカル `make lint` が stdlib を import するパッケージを解析できる |
| M2 | フェーズ 2 | 4 つの葉パッケージのデータ型と interface | 同上 |
| M3 | フェーズ 3 | 4 つの fake（`testutil/mocks.go`・`mocks_test.go`） | 同上（`-tags test` でコンパイルされる） |
| M4 | フェーズ 4 | `internal/pipeline`（`Stage`・`StageError`・`New`・`Run` と各テスト・guard） | 同上 |
| M5 | フェーズ 5 | `docs/dev/project_overview.md` の更新 | 同上・正の確認と旧表記の残骸なし |

各フェーズを 1 コミット単位で進め、package_reference.md の登録は各パッケージを新設するフェーズのコミットに含める（フェーズ 5 にまとめない）。ただし、フェーズ 1 の前提タスク（golangci-lint の更新）は、フェーズ 1 内で `internal/secret` の作業とは分離した独自のコミットとして進める例外を適用する（前提タスクのタスク文のとおり）。

## 4. テスト戦略 (Test Strategy)

architecture §7 のテスト戦略に従う。

- **ユニットテスト**: 外部コマンド・LLM API・Webhook を一切呼ばない。パイプラインのテストは 4 つの fake（フェーズ 3）を注入して行う。
- **`internal/secret`**: `fmt.Sprintf`・`slog`・`encoding/json`・構造体への埋め込み（公開/非公開フィールド）・ゼロ値・空文字列の各経路を検証する。直接の書式化経路（`Secret` 単体と公開フィールドとして含む構造体）は出力が固定文字列と**完全一致**することを検証し（例: `%s`・`%v` は `[REDACTED]`、`%q` は `"[REDACTED]"`。`[REDACTED]` が現れることだけの検証では、`[REDACTED]-<長さ>`・`[REDACTED]-<ハッシュ>`・引用符の欠落が通ってしまうため）、非公開フィールド経路は元の値が**現れないこと**を検証する。非公開フィールド経路の出力（関数アドレス）はゴールデンファイル化しない。
- **型・interface の検証**: fake のコンパイル時アサーション（AC-06）に加え、リフレクションによる guard テストで、interface の第 1 引数が `context.Context` であること（AC-07）、interface のドキュメントコメントが契約条項を含み英語であること（AC-08）、`secret.Secret` と `*secret.Secret` の両方の公開メソッド集合が、設計が定義するメソッド（`New`・`Reveal`・`Format`・`LogValue`・`MarshalJSON`）の明示的な許可リストと一致し、平文文字列を返すのが `Reveal` のみであること（AC-21）、共通データ型のフィールド集合が（名前, 型）ペアで設計どおりであること（AC-03・AC-05）を検証する。葉パッケージにはパッケージ本体の `_test.go` を置かない（architecture §7.1）ため、guard テストは `internal/pipeline/pipeline_test.go` に置く。
- **build tag（AC-17）**: `TestFakesCarryBuildTag` が 4 つの `mocks.go` の `//go:build test` を検証する。`make test` が `-tags test` で fake をコンパイルすることを確認する。
- **テストの実装詳細**（テスト関数名の付け方・配置、アサーションの書き方）は実装時に決定し、本計画では細部を固定しない。各テストは対応する対策・分岐を壊したときに失敗することを実装時に確認する。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

AC ごとの検証は次のとおり。`test` は実行可能なテスト、`static` は guard テスト・`make` ターゲット・コミット済みスクリプトを指す。

| AC | 内容 | 種別 | 検証の実行場所 |
|---|---|---|---|
| AC-01 | `Transcript.Segments` が開始時刻・文字列・出現順を保持 | test | `internal/pipeline/pipeline_test.go::TestPipelineSuccess` |
| AC-02 | `Transcript` がメタ情報 5 項目を保持 | test | 同上 |
| AC-03 | LLM の応答がテキストとモデル名を返す | test | `internal/pipeline/pipeline_test.go::TestCommonTypesFieldSets`（`GenerateResponse` のフィールド集合が（名前, 型）ペアで `Text string`・`Model string` とちょうど一致することを検証。fake が指定値をそのまま返すだけの確認は、代入値を読み返すだけのパターンになるため、AC-03 の検証には用いない） |
| AC-04 | `Article` がタイトル・本文・出典 URL・モデル名を保持 | test | `internal/pipeline/pipeline_test.go::TestPipelineSuccess` |
| AC-05 | 共通型にプロバイダ固有の項目・SDK 型を含めない | test / static | `TestCommonTypesFieldSets`（`Transcript`・`Segment`・`GenerateRequest`・`GenerateResponse`・`Article` の公開フィールドを（名前, 型）ペアで固定し、余分なフィールド追加とフィールド型の変更の両方を検出）＋ `make lint`（depguard の SDK 閉じ込めルール、`.golangci.yml:63-86`） |
| AC-06 | 4 つの interface が定義され、入力・出力・エラーを返す | test | 各 `testutil/mocks.go` のコンパイル時アサーション（`make test` で検証） |
| AC-07 | 全メソッドの第 1 引数が `context.Context` | test | `internal/pipeline/pipeline_test.go::TestInterfaceContracts`（コンパイル時アサーションでは第 1 引数の位置を検証できないため、リフレクションで検証） |
| AC-08 | interface に英語のドキュメントコメント（失敗時エラー・空の結果を返さない契約を含む） | static | `internal/pipeline/pipeline_test.go::TestInterfaceDocComments`（4 つの interface のソースファイルを読み、各ドキュメントコメントが契約条項を含み英語であることを検証）＋ `make lint`（revive の `exported` ルール） |
| AC-09 | 段階が順に 1 回ずつ呼ばれ出力が伝播 | test | `internal/pipeline/pipeline_test.go::TestPipelineSuccess` |
| AC-10 | 成功時に投稿した `Article` を返す | test | 同上 |
| AC-11 | 失敗時に以降の段階を呼ばない | test | `internal/pipeline/pipeline_test.go::TestPipelineStageFailure` |
| AC-12 | `StageError` で段階を判別し元のエラーを辿る | test | `internal/pipeline/pipeline_test.go::TestPipelineStageError` |
| AC-13 | キャンセル時に次の段階を呼ばず `context.Canceled` を返す | test | `internal/pipeline/pipeline_test.go::TestPipelineCanceled`（段階間キャンセルは決定的な同期手段で検証） |
| AC-14 | nil 段階の構築を拒否（typed-nil を含む） | test | `internal/pipeline/pipeline_test.go::TestPipelineNewNilStage`（3 引数すべてでテーブル検証。typed-nil は nil を許容するすべての kind（ptr・func など）に `reflect.Value.IsNil` を適用し、名前付き関数型の nil 値も含む） |
| AC-15 | fake の戻り値・エラー指定 | test | 各 `testutil/mocks_test.go` |
| AC-16 | fake の呼び出し記録 | test | 各 `testutil/mocks_test.go` |
| AC-17 | fake が `//go:build test` でのみビルド | static | `internal/pipeline/pipeline_test.go::TestFakesCarryBuildTag`（4 つの `mocks.go` の build tag を検証） |
| AC-18 | fmt の全経路で元の値が出ない | test | `internal/secret/secret_test.go::TestSecretFmtRedaction`（直接書式化は固定文字列との**完全一致**を検証。例: `%s`・`%v` → `[REDACTED]`、`%q` → `"[REDACTED]"`。負の性質: 固定文字列以外を出力する指定子は失敗）・`TestSecretFmtNoDelegateVerbs`・`TestSecretUnexportedFieldNoLeak`（後 2 つは元の値が現れないことを検証） |
| AC-19 | slog 属性で元の値が出ない | test | `internal/secret/secret_test.go::TestSecretSlogRedaction` |
| AC-20 | JSON エンコードで元の値が出ない | test | `internal/secret/secret_test.go::TestSecretJSONRedaction` |
| AC-21 | `Reveal()` のみが元の値を返す | test | `internal/secret/secret_test.go::TestSecretReveal`（振る舞い）＋ `internal/pipeline/pipeline_test.go::TestSecretRevealExclusive`（guard。`Secret` と `*Secret` の両方の公開メソッド集合を、設計が定義するメソッド（`New`・`Reveal`・`Format`・`LogValue`・`MarshalJSON`）の明示的な許可リストと照合し、平文文字列を返すのが `Reveal` のみであることを検証） |
| AC-22 | `New("")` がエラー | test | `internal/secret/secret_test.go::TestSecretNewEmpty` |
| AC-23 | ゼロ値の `Reveal()` がエラー | test | `internal/secret/secret_test.go::TestSecretZeroValueReveal` |

**横断検索項目**: `docs/dev/project_overview.md` の旧表記（`(string, error)`・`System`・`Temperature`）の残骸確認はフェーズ 5 のタスクで行う（`make test` / `make lint` では検出できないため）。`docs/tasks/0001_pipeline_skeleton/` 配下の履歴記述（要件 §5.1、architecture 付録A）は意図的に旧表記のまま残す。

## 6. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| ローカルツールチェーン（go 1.27.1）と golangci-lint v2.11.4 の非互換で `make lint` が失敗する | 各フェーズの完了条件を満たせない | フェーズ 1 の前提タスクで golangci-lint を v2.13.0（go1.27 対応）に更新する（§1.3・§2 フェーズ 1） |
| `fmt`・slog・`encoding/json` の特定の書式指定子や埋め込みパターンで挙動が想定と異なる | AC-18〜20 の不成立 | 計画作成時にローカル Go で挙動を検証済み（§1.3）。テストは全指定子のテーブルと公開/非公開フィールドの両経路をカバーする |
| typed-nil の interface が `Run` 実行時にパニックを起こす | AC-14 の不成立 | `New` でリフレクションによる nil 検出を行い、構築時に `ErrNilStage` を返す。テストで 3 引数すべての typed-nil をテーブル検証する |
| lint（revive の `exported`・`package-comments`、goconst、err113 など）が新設コードで指摘を出す | `make lint` が通らない | パッケージコメント・公開識別子への英語ドキュメントコメント、`[REDACTED]` のパッケージ定数化、静的 `errors.New` の使用を各フェーズのタスクに含めた |
| テスト内の数値リテラル（`StartMs` の値、`len(Calls)` など）が mnd に指摘される（`_test.go` は mnd の除外対象ではない） | `make lint` が通らない | 実装時に指摘された場合、定数化または必要最小限の `//nolint:mnd` で対応する（フェーズ 1・4 の実装時に確認） |
| AC-13 の段階間キャンセルのテストが racy になる | テストが間欠的に失敗する | フェーズ 4 で決定的な同期手段（カスタム `context.Context`）を指定し、fake に共通のフィールド構成は変更しない |
| 将来の interface シグネチャ変更が fake を壊す | コンパイルエラー | fake のコンパイル時アサーションが `make test` で検出する（テスト戦略 §4） |

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 前提タスク: golangci-lint の v2.13.0 への更新（`Makefile`・`.pre-commit-config.yaml`・`.github/workflows/ci.yml` の 3 箇所を同時に。ツールチェーン更新のみを 1 コミットに）
- [ ] フェーズ 1: `internal/secret`（`Secret` 型・AC-18〜23 のテスト・package_reference 更新）
- [ ] フェーズ 2: 4 つの葉パッケージ（データ型・interface・package_reference 更新）
- [ ] フェーズ 3: 4 つの fake（`testutil/mocks.go`・`mocks_test.go`・package_reference 更新）
- [ ] フェーズ 4: `internal/pipeline`（`Stage`・`StageError`・`New`・`Run`・AC-09〜14 と AC-17 のテスト・guard テスト・package_reference 更新）
- [ ] フェーズ 5: `docs/dev/project_overview.md` の更新（`LLMClient` の例・責務・ディレクトリ構成・正の確認と残骸の確認）
- [ ] 全フェーズで `make fmt` → `make test` → `make lint` が通る
- [ ] 各テストが「対応する対策・分岐を壊すと失敗する」ことを実装時に確認済み（コミットメッセージに記録）

## 8. 成功基準 (Success Criteria)

- 全 AC（AC-01〜AC-23）に、§5 のとおり `test` または `static` の検証がある。
- `make test` と `make lint` が通る（green gate）。ローカルの開発環境でも `make lint` が stdlib を import するパッケージを解析できる（フェーズ 1 の前提タスク）。
- 本番バイナリに fake が含まれない（`TestFakesCarryBuildTag` が 4 つの `mocks.go` の `//go:build test` を検証）。
- `docs/dev/project_overview.md` の `LLMClient` の記述が本設計に一致し、`docs/dev/developer_guide/package_reference.md` に全新設パッケージが登録されている（各フェーズのタスクで `rg` による正の確認を行う）。
- `cmd/yt2column/main.go` が変更されていない（CLI 配線は #6）。

## 9. 次のステップ (Next Steps)

- 本計画のレビューと承認（`draft` → `approved`）。
- 承認後、`/mkplan2` を実行して PR 境界（`### PR-` マーカー）を本計画に埋め込む。`/runplan` は、フェーズ 2 以上を含む計画で `### PR-` マーカーがない場合にステップ 2.5 で停止して「PR boundary design is missing」を報告するため、この設計ステップは実装開始の前に必須である。PR 境界設計の成果物は実装前に人間のレビューを受ける。
- 承認後、フェーズ 1 から順に実装し、各フェーズで `make fmt` → `make test` → `make lint` を通す。
- 本タスク完了後、後続タスク（#3 字幕取得・#4 DeepSeek アダプタ・#5 記事生成・#6 FilePublisher と CLI・#7 Slack 投稿）が、本タスクで定めた型と interface・fake に依存して並行開発できる。
