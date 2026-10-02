# 実装計画書：DeepSeek アダプタ（LLMClient 実装）

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-02 |
| Review date | 2026-10-02 |
| Reviewer | isseis |
| Comments | PR の区切りの訂正（editorial）。ステップ 6-3・6-4（`security.md` §2 の実キーの例外と `http2debug` のポリシー）を PR-4 に再配置し、ステップ 1-6 の条件付きの件数上限付き分割の最適化を PR-1 とは別の独立した PR とした。何をなぜ作るかは変わらず、決定の変更はない。続く PR の区切りの訂正（editorial）。§3.2 に PR の区切りの不変条件と条件付きの PR の扱いを明記し、フェーズ 6 でステップ 6-3・6-4 を PR-4 作成ポイントの前、6-1・6-2 をその後に並べ替え（番号は維持）、決定済みの独立した最適化の PR を条件付きの PR-1a（ステップ 1-10・1-11、測定が 256 MiB 以下なら `[-]`）として作成ポイントつきで §2・§3.2・§7・§9 に加えた。最適化の条件と独立した PR とする方針は前回の訂正のままで、決定の変更はない。 |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`01_requirements.md`（以下「requirements」）で定義した DeepSeek アダプタを、`02_architecture.md`（以下「architecture」）の設計どおりに実装する。具体的には、厳格な JSON の部品を `internal/transcript` から新設の `internal/strictjson` へ移し、`internal/llm` にプロバイダ共通の番兵・`GenerateRequest.Validate`・`GenerateResponse.ModelVersion` を加え、`internal/llm/deepseek` に `llm.LLMClient` の実装を新設する。実 API を使う統合テストは `make test-integration-deepseek` に分離する。

本計画は architecture §8 の実装優先順位（厳格な JSON の部品の移動 → `internal/llm` の追加 → アダプタの構築と送信 → 応答の検証 → 統合テスト → ドキュメント）をそのままフェーズ 1〜6 とする。§8 の 0（事前調査）は実施済みであり、フィクスチャ 2 件と `testdata/README.md` の出典はコミット `c287b7b` で追加済みである（§1.3）。

### 1.2. 実装原則

- architecture §1.1 の設計原則 8 項目に従う。特に、応答本文は補正せず拒否し（原則 1）、`Reveal()` を呼ぶのは `request.go` だけとし（原則 3）、送信先は `New` が常に本番の値に設定する（原則 5）。
- 本番コードで新設・変更するファイルは architecture §3.8 の一覧に限る。テストファイルは同表の新設のテストファイル 5 件に、本計画で `makefile_test.go` と `integration_env_test.go` の 2 件を加える（§4.4 で理由を示す）。統合テストの環境変数の判定のテストと build tag の guard は、architecture §3.8 の割り当て（AC-23 は `integration_test.go`）と異なり `deepseek_test.go` に置く（`-tags integration` のビルドのテスト関数を `TestIntegrationGenerate` だけにするため。§1.5 の I-02）。ドキュメントは同表の一覧に `README.md` を加える（ステップ 6-5）。
- `internal/llm/deepseek` のユニットテストは同じパッケージ（`package deepseek`）に置き、すべて `test_helpers.go` の非公開のヘルパーで作った、送信先がループバックのアダプタを使う（architecture §7.1）。
- `test_helpers.go` を使う `_test.go` と、`internal/llm/llm_test.go` の先頭には `//go:build test` を付ける（`internal/llm/testutil/mocks_test.go:1`・`internal/transcript/json3_test.go:1` と同じ）。`internal/strictjson/strictjson_test.go` も同様とする。
- Go のコメント・識別子・文字列リテラルは英語で書く。`AC-NN`・`F-NNN`・`H-NN`・`I-NN` は Go ソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
- 上限値・猶予・送信先・ステータスごとの案内などは名前付き定数にする（`mnd`・`goconst`。`.golangci.yml` の `mnd.ignored-numbers` はパーミッションだけ）。
- 各フェーズの完了条件は `make fmt` → `make test` → `make lint` が通ること。各テストは、対象の分岐を実際に壊して失敗することを確認し、そのことをコミットメッセージに書く（CLAUDE.md「Testing Strategy」）。

### 1.3. 既存コード調査結果

HEAD `00573f2`（ブランチ `task/0003-deepseek-llm-client-02`）で確認した。追跡対象のファイルに未コミットの変更はない。ベースラインの `go test -tags test ./...` は全パッケージが `ok` または `[no test files]` で成功した（2026-10-02、HEAD `00573f2`）。

- **厳格な JSON の部品（移動対象）。** `internal/transcript/json3.go:21-48`（静的エラーと `\uXXXX` 走査用の定数）と `:203-406`（`jsonMember`・`decodeJSONObject`・`collectMembers`・`newStrictDecoder`・`newRawDecoder`・`ensureNoTrailingJSON`・`decodeString`・`decodeInt64`・`validateJSONEncoding`・`surrogate*` 定数・`validateSurrogates`・`surrogateKind`・`isHexDigit`）、`internal/transcript/info.go:16-17`（`errMissingField`・`errEmptyField`）と `:88-103`（`requiredString`）。architecture §1.3 が HEAD `628b4ea` で記した行番号は、HEAD `00573f2` でも同じである。
- **移動対象の参照箇所はすべて `json3.go` と `info.go` の中にある。** 上記の識別子を `rg` で `-g '*.go'` 全体から検索した結果、出現するファイルは `internal/transcript/json3.go`（70 件）と `internal/transcript/info.go`（15 件）だけで、テストファイル・`test_helpers.go` は参照しない（2026-10-02、HEAD `00573f2`）。テストが参照する `json3.go`・`info.go` の非公開の識別子（上限の定数 `maxSubtitlesBytes`・`maxInfoBytes`・`maxSubtitleEvents`、`errInputTooLarge`、`videoInfo`、`parseSubtitles`・`parseInfo` など）はいずれも移動の対象外であり、`internal/transcript` に残す。
- **`decodeEvent` は `segs` と `tStartMs` を別々に取り出す**（`json3.go:125` と `:140` の 2 回の `collectMembers`）。本文を持たないイベントの `tStartMs` は検証しない（重複も拒否しない）ためであり、書き換え後も 2 回の `Collect` を保つ（ステップ 1-3）。
- **`info.go` の `description` は空文字列を受理する**（`info.go:78-84`。存在すれば `decodeString` だけを通す）。architecture §3.5 の `strictjson.OptionalString` は空文字列を拒否するため、`description` には使わない（使うと振る舞いが変わる）。同様に `id` の欠落は transcript 固有の `errMissingID`（`info.go:57-60`）で報告しているため、`strictjson.Required` ではなくメンバーの有無の確認と `AsString` で書き換える。
- **`json3.go:21-24` のコメントは移動後に古くなる。** 「parsers が共有する厳格なデコードの静的エラー」と書いているため、残るエラーに合わせて書き換える（ステップ 1-3）。lint では検出されない。
- **`internal/llm/llm.go:1-26`** は `GenerateRequest`・`GenerateResponse`（`Text`・`Model`）・`LLMClient` だけを持ち、番兵もメソッドもない。`internal/llm` にテストファイルはない（ベースラインで `[no test files]`）。
- **`internal/pipeline/pipeline_test.go` の guard。** `TestCommonTypesFieldSets`（`:375`。`GenerateResponse` の期待値は `:398-401` で `Text`・`Model` の 2 つ）と `TestInterfaceDocComments`（`:448`。`LLMClient` の条項は `:458-460` の 1 つ）を更新する。`TestSecretRevealExclusive`（`:486`）は `secret.Secret` のメソッド集合だけを固定しており、どのファイルが `Reveal()` を呼ぶかは検査しない。そのため、architecture の原則 3 を固定する guard はステップ 3-7 で新設する。`TestFakesCarryBuildTag`（`:509`）は `../*/testutil/*.go` を 8 件に固定している（`:514-516`）。本計画は `testutil/` にファイルを足さないため、この guard は変わらない。
- **`internal/llm/testutil/mocks.go`・`mocks_test.go`** は変更しない。`mocks_test.go:15` は `GenerateResponse` を `Text`・`Model` だけで組み立てており、`ModelVersion` の追加後もそのまま通る。
- **`internal/secret/secret.go:22-45`。** `Secret` は値をクロージャに閉じ込め、ゼロ値の `Reveal()` は `errZeroValue` を返す。そのまま使う。
- **統合テストの build tag の guard は `internal/transcript` にだけある。** `internal/transcript/ytdlp_test.go:1676-1709` の `firstLineIs` と `TestIntegrationTestBuildTag` は、同じパッケージの `integration_test.go` だけを検査する。パッケージをまたいで共有できるテストヘルパーの置き場は `testutil/`（`test_organization.md` 分類 A）だが、置くと `TestFakesCarryBuildTag` の件数が変わり、`integration_test.go` のパスを検査するだけの関数のために公開 API を設けることになる。本計画は `internal/llm/deepseek` に同じ形の guard を 1 つ置く（ステップ 5-5）。ファイルの先頭行を比べるだけの数行の処理であり、DRY の対象にしない。
- **lint・vet は既に統合テストを解析する。** `Makefile:10` の `GOLINT` は `--build-tags test,integration`、`Makefile` の `lint` は `go vet -tags integration ./...` も実行する。CI（`.github/workflows/ci.yml:88` の `args: --build-tags test,integration --timeout=5m` と `:93` の `go vet -tags integration ./...`）も同じである。`internal/transcript/ytdlp_test.go:1720` の `TestLintTagsIncludeIntegration` がこれを固定している。`Makefile`・`.pre-commit-config.yaml`・CI の lint 経路は変更しない。
- **Makefile。** `test`（`go test -tags test -race -v ./...`）・`test-ci`（`-tags test`）は `integration` タグのファイルをビルドしない。`test-integration` は `./internal/transcript` だけを対象とする。`test-integration-deepseek` は存在しない。
- **フィクスチャ。** `testdata/deepseek_chat_completion_stop.json`（971 バイト、`finish_reason` `stop`、`system_fingerprint` あり）と `testdata/deepseek_chat_completion_length.json`（619 バイト、`finish_reason` `length`、`content` `""`）と、`testdata/README.md` の出典の節は、コミット `c287b7b` で追加済みである。architecture §3.8 の `testdata/README.md`「変更」は完了しており、本計画では変更しない。
- **`deepseek` パッケージの利用者はいない。** `rg -n "deepseek" -g '*.go' .` の結果は `internal/llm/testutil/mocks_test.go:15` のモデル名の文字列 1 件だけ（2026-10-02、HEAD `00573f2`）。`cmd/yt2column/main.go` は配線を持たず、`deepseek.New` の呼び出し元は #6 で加わる（§9 を参照）。
- **`README.md` は統合テストの実行方法を説明している**（`README.md:72` と `:81-90` の `make test-integration`、`:60` の `DEEPSEEK_API_KEY`）。`make test-integration-deepseek` とその環境変数は載っていない。
- **ドキュメントの更新対象。** `docs/dev/developer_guide/package_reference.md:16-17`（`internal/transcript`・`internal/llm` の行）、`docs/dev/project_overview.md:40`（`GenerateResponse: Text, Model`）・`:70-84`（想定ディレクトリ構成。`testdata/` の行は「json3・info.json のサンプル」のままで、DeepSeek の実応答を含まない）、`CLAUDE.md:78-85`（Architecture Overview のパッケージの説明）、`docs/dev/security.md:16-20`（§2。「テストでは実在のキーや URL を使わない」）。英語版の文書（`*.en.md`）は存在せず、翻訳の更新は不要である。
- **外部前提の確認。**
  - `go.mod:3` は `go 1.26.5`、手元のツールチェーンは go1.27.1。`context.WithTimeoutCause`（Go 1.21）と `errors.AsType`（Go 1.26）が使える。依存の追加は不要である。
  - `http.DefaultTransport` の proxy は `httpproxy.FromEnvironment` が `HTTPS_PROXY`→`https_proxy`、`NO_PROXY`→`no_proxy` の順に最初の空でない値を採り（go1.27.1 `src/vendor/golang.org/x/net/http/httpproxy/proxy.go:90-106`）、ループバックと `localhost` は proxy を通さない（同 `:116`・`:178-185`）。環境変数はプロセス内で最初の 1 回だけ読まれる（`src/net/http/transport.go:1032-1047` の `envProxyOnce`）。ステップ 3-6 の `TestMain` はこれに依拠する。
  - `GODEBUG` に `http2debug=1` または `http2debug=2` を含めると `VerboseLogs` が真になり（go1.27.1 `src/net/http/internal/http2/http2.go:50-56`）、HTTP/2 の Transport が送るヘッダーを値ごと `log.Printf` で出力する（同 `transport.go:1849-1851`）。architecture §5.4 は `=2` だけを挙げているが、`=1` でも同じである。ステップ 6-4 の security.md の記述はこの両方を書く（architecture の決定は変えない）。
  - GNU Make 3.81（手元の `make --version`）で、次を一時ディレクトリの Makefile で確認した（2026-10-02）。`VAR ?= default` は環境で空文字列に定義された変数を上書きしない。ターゲット固有の `export VAR := …` はそのターゲットのレシピにだけエクスポートされ、他のターゲットからは見えない。コマンドラインで与えた `GOTEST=…` は Makefile 内の代入より優先される。ステップ 5-3・5-4 はこれに依拠する。CI の `ubuntu-latest` は GNU Make 4 系である。これらの意味は GNU Make のマニュアルの "Setting Variables"（`?=` は未定義の変数にだけ代入する）・"Variables from the Environment"・"Target-specific Variable Values"（`export` 付きの指定）・"Overriding Variables"（コマンドラインの代入が優先する）に記された仕様であり、版で変わらない。なお、外側の `make` に渡したコマンドラインの変数は `MAKEFLAGS` を通じて内側の `make` に伝わる（レビュー時に 3.81 で確認した）。ステップ 5-4 は `MAKEFLAGS` などを子の環境から除く。
- **architecture との整合。** 本計画は architecture §8 のフェーズの構成と順序をそのまま採る。調査の範囲で、architecture の修正が必要な不整合は見つかっていない。architecture の記述にない計画上の判断は、フェーズ 3 の暫定の検証関数（ステップ 3-4）、`TestMain` による proxy の固定（ステップ 3-6）、`Reveal()` の呼び出し箇所の guard（ステップ 3-7）、統合テストの環境変数の判定を純粋な関数にすること（ステップ 5-1）、Makefile のターゲットの実行テスト（ステップ 5-4）、`project_overview.md` の `testdata/` の行の更新（ステップ 6-1）、security.md に `http2debug=1` も書くこと（ステップ 6-4）、`README.md` の更新（ステップ 6-5）である。また、architecture §3.8 は `package_reference.md` で `internal/transcript` の責務を更新するとするが、同パッケージの行の記述（strict json3 and info.json parsers）は移動の後も正しいため、本計画は変更しない（ステップ 1-7）。いずれも architecture の決定を変えない。

### 1.4. 名前の変更・移動の一覧

フェーズ 1 で `internal/transcript` から `internal/strictjson` へ移す要素の一覧である。公開 API の名前は architecture §3.5 のとおりで、本計画の他の箇所はこの表に従う。

| 移動前（`internal/transcript`） | 移動後（`internal/strictjson`） |
|---|---|
| `jsonMember` | 非公開の型（`Object` の内部表現） |
| `decodeJSONObject`・`newStrictDecoder`・`ensureNoTrailingJSON`（トップレベルの組み合わせ） | `ParseObject` |
| `decodeJSONObject`（入れ子のオブジェクト） | `Value.AsObject` |
| `collectMembers` | `Object.Collect` |
| `decodeString` | `Value.AsString` |
| `decodeInt64` | `Value.AsInt64` |
| `requiredString`（`info.go`） | `RequiredString` |
| `json3.go` の配列の走査（`decodeEvents`・`decodeSegs` の `[` の確認と要素の読み取り） | `Value.AsArray`（`decodeEvents`・`decodeSegs` 自体は `internal/transcript` に残す） |
| `newRawDecoder`・`validateJSONEncoding`・`validateSurrogates`・`surrogateKind`・`isHexDigit`・`surrogate*` 定数・`hexDigitsOffset` などの走査用の定数 | 同じ名前の非公開の要素 |
| `errInvalidEncoding`・`errUnpairedSurrogate`・`errTrailingJSON`・`errNotJSONObject`・`errNonStringKey`・`errDuplicateMember`・`errValueNotString`・`errValueNotNumber`・`errMissingField`・`errEmptyField` | 同じ名前の非公開の静的エラー |
| — | 新設: `Object.Has`・`Required`・`OptionalString`、`AsArray` の種類の不一致とゼロ値の `Value` を表す非公開の静的エラー |

`internal/transcript` に残すもの: `errInputTooLarge`・`errMissingEvents`・`errEventsNotArray`・`errTooManyEvents`・`errMissingTimestamp`・`errNegativeTimestamp`・`errSegsNotArray`・`errMissingID`・`errIDMismatch`、上限の定数、`parseSubtitles`・`decodeSubtitles`・`decodeEvents`・`decodeEvent`・`decodeSegs`・`decodeTimestamp`・`parseInfo`・`decodeInfo`。

### 1.5. implementation_handoff.md の項目への対応

| ID | 対応 |
|---|---|
| I-01 | 統合テストの 1 回の `Generate` のタイムアウトを **15 分**、`make test-integration-deepseek` の `-timeout` を **40 分** とする（ステップ 5-2・5-3）。15 分は API の待ちの上限 10 分（architecture §1.4）に生成の時間の余裕 5 分を足した値、40 分は `Generate` 2 回 × 15 分に余裕 10 分を足した値であり、architecture §3.6 の 2 つの関係を満たす。`Generate` の回数を変えるときは `-timeout` を見直す旨を `integration_test.go` のコメントに書く。 |
| I-02 | `-run` を使わず、パッケージのパス `./internal/llm/deepseek` だけを渡す。`-tags integration` のビルドに含まれるテスト関数は `TestIntegrationGenerate` だけとする（ステップ 5-1 の純粋な関数は `test || integration` のファイルに置くが、そのテストは `//go:build test` のファイルに置く）。フラグとパッケージのパスが渡される形は `TestMakeTestIntegrationDeepSeek`（ステップ 5-4）が `GOTEST` を差し替えて検証し、実 API での実行は §5.1 に `TestIntegrationGenerate` の 2 つのサブテストの `--- PASS` を記録する（ステップ 5-6）。 |
| I-03 | `ModelVersion` は `%+q`（`strconv.QuoteToASCII` 相当）で引用してから `t.Logf` で出力する（ステップ 5-2）。制御文字と非 ASCII 文字はエスケープされる。 |
| I-04 | 打ち切りの確認のプロンプトは、`MaxOutputTokens` 16 を大きく超える出力（数段落の説明）を求める英語の固定文とする。`stop` で終わってエラーが返らなかった場合の失敗メッセージには、`MaxOutputTokens` の値と「応答が打ち切られずに終わった（アダプタの不具合ではなく前提が崩れた可能性）」ことを示す（ステップ 5-2）。`finish_reason` は `GenerateResponse` に含まれないが、アダプタがエラーを返さないのは `stop` の場合だけである（architecture §3.4）。 |

## 2. 実装ステップ (Implementation Steps)

各フェーズの完了条件は「そのフェーズの `make fmt` → `make test` → `make lint` が通ること」。ステップは `X-Y` 形式で表す（X: フェーズ番号、Y: フェーズ内の連番）。テスト関数名と AC の対応は §5 にまとめ、各ステップでは対象の AC だけを示す。パッケージを追加・変更するフェーズでは、そのフェーズのコミットで `package_reference.md` の該当行を更新する（architecture §8）。

### フェーズ 1: 厳格な JSON の部品の移動（`internal/strictjson`）

**対象ファイル**
- 新設: `internal/strictjson/strictjson.go`・`internal/strictjson/strictjson_test.go`（`//go:build test`）
- 変更: `internal/transcript/json3.go`・`internal/transcript/info.go`・`docs/dev/developer_guide/package_reference.md`

**タスク**
- [x] **ステップ 1-1**: `strictjson.go` を作成し、§1.4 の一覧に従って部品を移し、architecture §3.5 の公開 API（`Object`・`Value`・`ParseObject`・`Object.Collect`・`Object.Has`・`Required`・`RequiredString`・`OptionalString`・`Value.AsString`・`AsInt64`・`AsObject`・`AsArray`）を実装する。`Object`・`Value` のフィールドは非公開とし、デコーダや `json.RawMessage` を受け取る関数は公開しない。ゼロ値の `Value` の `As...` はエラーを返す。パッケージの doc コメントに、検証を通った文書の値だけを扱うことを書く。import は標準ライブラリだけとする。
- [x] **ステップ 1-2**: `strictjson_test.go` を作成し、§5 の表に挙げたテストを実装する。architecture §7.1 のとおり、公開 API ごとに受理と拒否の最小の入力（リテラル）を検証する。`ParseObject` は不正な UTF-8・対になっていないサロゲート・有効なサロゲートの対の受理・後続データ・オブジェクト以外のトップレベル、`Collect` は消費するキーの重複の拒否と消費しないキーの重複の受理を含める。これに加えて、パーサのテストでは表せない入力（ゼロ値の `Value`、`AsArray` の要素の種類、`Has` が拒否しないこと、`RequiredString` の空文字列、`OptionalString` の欠落の受理と `null`・空文字列・文字列以外の拒否、`AsInt64` の範囲外）を含める。`internal/transcript` のテストが使う `testdata/` のサンプルと実データのケースは繰り返さない。
- [x] **ステップ 1-3**: `json3.go` を `strictjson` の API で書き換え、移した部品と静的エラーを削除する。`events`・`segs` は `AsArray` と `AsObject` で読み、`AsArray` の失敗は従来どおり `errEventsNotArray`・`errSegsNotArray` で報告する。件数の上限は `AsArray` の後で判定する（architecture §3.5）。`decodeEvent` は `segs` と `tStartMs` を 2 回の `Collect` で別々に取り出す形を保つ（§1.3。1 回にまとめると、本文を持たないイベントの `tStartMs` の重複を拒否するようになる）。静的エラーの var ブロックのコメントを、次のとおり書き換える。
  - 変更前: `// Static errors for the strict JSON decoding shared by the parsers, including` / `// the sequences encoding/json would silently repair (invalid UTF-8 and unpaired` / `// UTF-16 surrogate escapes) and the structural shapes the parsers reject.`
  - 変更後: `// Static errors for the json3 and info.json parsers. The strict JSON decoding` / `// they rely on, and its errors, live in internal/strictjson.`
- [x] **ステップ 1-4**: `info.go` を書き換える。`title`・`channel` は `strictjson.RequiredString` を使う。`id` は従来どおり欠落を `errMissingID` で報告し、値は `AsString` で読む。`description` は存在すれば `AsString` だけを通し、空文字列を受理する（`OptionalString` は使わない。§1.3）。`requiredString`・`errMissingField`・`errEmptyField` を削除する。
- [x] **ステップ 1-5**: `internal/transcript` の既存テスト（`json3_test.go`・`info_test.go`・`ytdlp_test.go`・`cache_test.go` など）を変更せずに `make test` が通ることを確認する。このフェーズの差分に `internal/transcript/*_test.go` と `internal/transcript/test_helpers.go` が含まれないことを、コミット前に差分の一覧で確認する。
- [x] **ステップ 1-6**: architecture §3.5 の測定を行う。入力は、`{"events":[0,0,…]}` の形で 1 バイトの要素を並べて 8 MiB ちょうどにした文書（`AsArray` が作る要素の数が最大になる形。件数の上限を超えるため拒否されるが、分割は上限の判定より前に行われる）とする。これを `parseSubtitles` に渡したときの割り当ての総量（`runtime.MemStats.TotalAlloc` の差）とヒープの最大使用量（`HeapInuse` の最大値）を、移動前（HEAD `00573f2`）と移動後で測り、§5.2 に記録する。測定のコードはコミットしない。移動後のヒープの最大使用量は測定して §5.2 に記録し、それが **256 MiB** を超える場合にだけ、件数の上限付きの分割の最適化を実装する（CLAUDE.md「Performance」）。この最適化は PR-1 には含めず、条件付きの PR-1a（ステップ 1-10・1-11）で実装する。256 MiB 以下の場合は、§3.2 の「条件付きの PR」の規則に従い、ステップ 1-10・1-11 と PR-1a 作成ポイントのチェックボックスをすべて `[-]` にし、理由（§5.2 の測定値）を添えて、このステップと同じコミットに含める。
- [x] **ステップ 1-7**: `package_reference.md` に `internal/strictjson` の行を追加する（検証を通った JSON 文書から値を取り出す厳格な部品。`internal/transcript` と `internal/llm/deepseek` が使う）。`internal/transcript` の行の「strict json3 and info.json parsers」は変わらないため変更しない。
- [x] **ステップ 1-8**: 主要な分岐を壊して、対応するテストが失敗することを確認し、コミットメッセージに記録する。対象は次のとおり。最初の 3 つは `strictjson` のテストと `internal/transcript` の既存テスト（`TestParseSubtitlesUTF8`・`TestParseSubtitlesDuplicateMembers`・`TestParseSubtitlesRejectsSimple` など）の両方が失敗することを確かめる。
  - `validateSurrogates` を呼ばない（`TestParseObject`）。
  - `Collect` の重複の検出を外す（`TestObjectCollect`）。
  - `ParseObject` の後続データの確認を外す（`TestParseObject`）。
  - `AsString` が `null` を受理する、`AsInt64` が範囲外を受理する、`AsArray` がオブジェクトを受理する、ゼロ値の `Value` を受理する（`TestValueAccessors`）。
  - `Has` が存在しないキーに真を返す（`TestObjectHas`）。
  - `RequiredString` が空文字列を受理する、`Required` が欠落を受理する（`TestRequiredAndRequiredString`）。
  - `OptionalString` が空文字列を受理する（`TestOptionalString`）。
- [x] **ステップ 1-9**: `make fmt` → `make test` → `make lint` を通す。このフェーズを 1 つのリファクタリングのコミットにし、DeepSeek アダプタのコミットと分ける（architecture §3.5）。

### PR-1 作成ポイント: strict JSON extraction (internal/strictjson)

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9

**推奨タイトル**: `refactor(0003): move the strict JSON helpers to internal/strictjson`

**レビュー観点**: 部品が architecture §3.5 の公開 API に切り出され、`internal/transcript` の返す番兵と `*ParseError` が無変更の既存テストで保たれていること（ステップ 1-5） / `decodeEvent` の 2 回の `Collect` と `description` の空文字列の受理という移動前の振る舞いが保たれていること（§1.3、ステップ 1-3・1-4） / 未消費メンバーの重複を受理し、消費するメンバーの重複だけを拒否する境界が公開 API とテストで一致していること（ステップ 1-1・1-2） / 移動後のメモリ使用量の測定（ステップ 1-6）と、256 MiB を超える場合の扱いが §5.2 に記録されていること

**実装モデル要件**: standard

**判定理由**: 純粋なリファクタリングとそのテストに限られ、競合する実装方針の併記・リカバリや状態機械などの高リスクな制御・パネルモードのトリガーに該当せず、Conditional checks も該当しないため。

ステップ 1-6 の測定が **256 MiB** を超えた場合の件数の上限付きの分割の最適化は、PR-1 には含めず、次の条件付きの PR-1a で実装する（§3.2）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 1a（条件付き）: 件数の上限付きの分割の最適化

ステップ 1-6 の移動後の測定で、ヒープの最大使用量が **256 MiB** を超えた場合にだけ実施する。256 MiB 以下の場合は、ステップ 1-6 でこのフェーズのステップと PR-1a 作成ポイントのチェックボックスをすべて `[-]` にしてある（§3.2 の「条件付きの PR」）。

**対象ファイル**
- 変更: `internal/strictjson/strictjson.go`・`internal/strictjson/strictjson_test.go`

**タスク**
- [ ] **ステップ 1-10**: `AsArray` の分割に件数の上限を加え、上限を超える配列を全要素の分割の前に拒否する。最適化は振る舞いを変えないこと（拒否される入力と返る番兵が最適化の前後で同じであること）を正しさの義務とし、それをコミットメッセージに書き、最適化を外すか変えると失敗するテストで固定する（CLAUDE.md「Performance」）。最適化だけを 1 つのコミットにする。ステップ 1-6 と同じ入力で再測定し、§5.2 に記録する。上限は `internal/strictjson` の非公開の定数 `maxArrayElements` とし、値は `1 << 17`（131072）とする。`internal/transcript` の `maxSubtitleEvents`（65536）より大きいため、同パッケージの件数の判定（`errTooManyEvents`）は上限内の範囲で従来どおり働き、上限を超える配列だけが `AsArray` によって早期に拒否される。どちらの経路でも `internal/transcript` の返る番兵は `ErrParseSubtitles` のままである。境界のテスト（`maxArrayElements-1`・`maxArrayElements`・`maxArrayElements+1`）を `strictjson_test.go` に置き、上限を外すと `maxArrayElements+1` が成功して失敗するようにする。DeepSeek の `choices`（ちょうど 1 要素）は上限の影響を受けない。
- [ ] **ステップ 1-11**: `make fmt` → `make test` → `make lint` を通す。

### PR-1a 作成ポイント: bounded array split in internal/strictjson (conditional)

**対象ステップ**: 1-10 / 1-11

**推奨タイトル**: `perf(0003): bound the array split in internal/strictjson`

**レビュー観点**: ステップ 1-6 の測定が 256 MiB を超えたこと、および最適化後の再測定が §5.2 に記録されていること（ステップ 1-6・1-10） / 最適化の前後で拒否される入力と返る番兵が変わらず、その義務がコミットメッセージに書かれ、テストで固定されていること（ステップ 1-10） / 最適化が独立したコミットで、単独で revert できること（CLAUDE.md「Performance」）

**実装モデル要件**: standard

**判定理由**: 1 つの部品の局所的な最適化とその固定のテストに限られ、競合する実装方針の併記・リカバリや状態機械などの高リスクな制御・パネルモードのトリガーに該当せず、Conditional checks も該当しないため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `internal/llm` の追加

**対象ファイル**
- 変更: `internal/llm/llm.go`・`internal/pipeline/pipeline_test.go`・`docs/dev/developer_guide/package_reference.md`
- 新設: `internal/llm/llm_test.go`（`//go:build test`）

**タスク**
- [ ] **ステップ 2-1**: `llm.go` に architecture §3.2 の 4 つの番兵（`ErrInvalidRequest`・`ErrTruncated`・`ErrUnexpectedFinishReason`・`ErrEmptyResponse`）、`GenerateResponse.ModelVersion`、`GenerateRequest.Validate` を追加する。`Validate` は、両プロンプトが空でなく正しい UTF-8 であることと `MaxOutputTokens` が負でないことを検査し、`ErrInvalidRequest` をラップしたエラーを返す。import は標準ライブラリだけとし、`internal/llm` をリーフのままにする。
- [ ] **ステップ 2-2**: `llm.go` の doc コメントを更新する。`GenerateRequest` に `MaxOutputTokens` の 0（プロバイダの既定）と負（不正）の意味を、`GenerateResponse` に `ModelVersion` の意味（プロバイダ共通、返さないプロバイダでは空文字列）を書く。`LLMClient` には、既存の条項（`must not return an empty response without an error`）を残したうえで、4 つの番兵で報告する条項と、タイムアウト・キャンセルを `context.DeadlineExceeded`・`context.Canceled` で判別できる条項を加える（architecture §3.2）。
- [ ] **ステップ 2-3**: `llm_test.go` に `TestGenerateRequestValidate` を作成する（AC-07 の検証規則。空の各プロンプト・不正な UTF-8 のバイト列を含む各プロンプト・負の `MaxOutputTokens` の拒否と、空白だけのプロンプト・`MaxOutputTokens` 0・正の値の受理）。
- [ ] **ステップ 2-4**: `pipeline_test.go` を更新する。`TestCommonTypesFieldSets` の `GenerateResponse` の期待値に `"ModelVersion": "string"` を加える。`TestInterfaceDocComments` の `LLMClient` のケースに、ステップ 2-2 で加えた 2 つの条項を、doc コメントと同じ文言で加える。
- [ ] **ステップ 2-5**: `package_reference.md` の `internal/llm` の行に、プロバイダ共通の番兵と `GenerateRequest.Validate` を加える。
- [ ] **ステップ 2-6**: 壊して失敗することを確認し、コミットメッセージに記録する。対象: `Validate` の負の値の検査を外す（`TestGenerateRequestValidate`）、`utf8.ValidString` の検査を外す（同）、`LLMClient` の doc コメントから加えた条項を消す（`TestInterfaceDocComments`）。
- [ ] **ステップ 2-7**: `make fmt` → `make test` → `make lint` を通す。

### PR-2 作成ポイント: provider-common LLM API (internal/llm)

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7

**推奨タイトル**: `feat(0003): add the provider-common LLM sentinels and request validation`

**レビュー観点**: 4 つの番兵と `GenerateRequest.Validate` が architecture §3.2 の検査（空のプロンプト・不正な UTF-8・負の `MaxOutputTokens`）を満たし、`internal/llm` が標準ライブラリだけのリーフのままであること（ステップ 2-1） / `GenerateResponse.ModelVersion` の追加に合わせて `TestCommonTypesFieldSets` と `TestInterfaceDocComments` の期待値が更新されていること（ステップ 2-4） / doc コメントに加えた条項が guard の期待値と一致していること

**実装モデル要件**: standard

**判定理由**: 番兵・検査・型の追加と guard の更新に限られ、競合する実装方針の併記・高リスクな制御・パネルモードのトリガー・Conditional checks のいずれにも該当しないため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: アダプタの構築と送信（`internal/llm/deepseek`）

**対象ファイル**
- 新設: `internal/llm/deepseek/errors.go`・`deepseek.go`・`request.go`・`response.go`（応答本文の読み取りまで）
- 新設: `internal/llm/deepseek/test_helpers.go`（`//go:build test`）・`internal/llm/deepseek/deepseek_test.go`（`//go:build test`）
- 変更: `docs/dev/developer_guide/package_reference.md`

**タスク**
- [ ] **ステップ 3-1**: `errors.go` に architecture §4.1 の 3 つの番兵（`ErrHTTPStatus`・`ErrInvalidResponse`・`ErrTransport`）と `HTTPStatusError`（`StatusCode` だけを持ち、`Unwrap` は `ErrHTTPStatus` を返す）を定義する。`Error()` はステータスコードと、architecture §4.2 のステータスごとの固定の案内を返す。構築のエラーとタイムアウトの cause に使う非公開の静的エラーも置く。
- [ ] **ステップ 3-2**: `request.go` に、リクエスト本文の非公開の構造体（`model`・`messages`・`stream`、正の値のときだけの `max_tokens`。`thinking` などのフィールドは持たない）、その組み立て（`json.Marshal`）、`Content-Type` と `Authorization` ヘッダーの設定、構築時の API キーの形の検査（ゼロ値でないこと、表示可能な ASCII `0x21`〜`0x7E` だけであること）を実装する（architecture §3.7）。`Reveal()` を呼ぶのはこのファイルの 2 か所だけとし、取り出した文字列をフィールドにもエラーにも残さない。ヘッダーの設定時の `Reveal()` のエラーは無視せず、HTTP リクエストを送らずにエラーを返す。
- [ ] **ステップ 3-3**: `deepseek.go` に `Options`・`New`・非公開の具体型・`Generate` と、送信先の定数を実装する（architecture §3.1・§3.3・§6.1）。`New` は API キー・`Model`（空・前後の空白・不正な UTF-8）・`Timeout`（0 以下）を検査し、`CheckRedirect` が `http.ErrUseLastResponse` を返す `http.Client`（`Timeout` は設定しない）を作る。`Transport` は設定せず（`http.DefaultTransport` を使い、環境変数の proxy に従う）、ステップ 3-6 の `TestMain` の proxy がアダプタの送信に効くようにする。`Generate` は `Validate` → 呼び出し元の `ctx` の終了の確認 → `context.WithTimeoutCause` で呼び出しの `ctx` を作る → 送信 → ステータスの確認（`200` 以外は応答本文を読まずに閉じる）→ 応答本文の読み取り → 応答の検証、の順に進む。送信と読み取りの失敗は、先に呼び出しの `ctx` の `Err()` を確認して `context` のエラーと `ErrTransport` に分け、`ErrTransport` には下位のエラーを `%w` でつながず文言だけを添える（architecture §4.1）。すべてのエラーのメッセージは `deepseek: ` で始め、architecture §4.2 の内容（`HTTPStatusError` にはモデル名と送った `max_tokens`、タイムアウトには期限の出所）を含める。
- [ ] **ステップ 3-4**: `response.go` に、応答本文を上限 + 1 バイトまで読み、上限（8 MiB、定数）を超えたら `ErrInvalidResponse` とする読み取りを実装する（architecture §3.4 の手順 2）。`Generate` が読み取った応答本文を渡す検証関数は、同じ PR-3 に含まれるステップ 4-1 で実装する。PR をマージする時点で読み取りと検証の両方が揃い、常にエラーを返す実装が main に入ることはない。ステップ 4-1 より先にフェーズ 3 のコミットを作る必要がある場合に限り、検証関数を常に `ErrInvalidResponse` を返す暫定の実装にしてコンパイルを通してよいが、その実装は同じ PR-3 の中でステップ 4-1 に置き換え、PR の最終状態には残さない。暫定の実装を置く場合は引数を使わず結果も一定であるため、`unparam`・`revive` の `unused-parameter` が指摘しうる。指摘された場合は、その関数に限った `//nolint:unparam,revive` と「ステップ 4-1 で置き換える暫定の実装」である旨の英語のコメントを付ける。
- [ ] **ステップ 3-5**: `test_helpers.go` を作成する。内容は次のとおり。
  - `New` と同じ検証を通したアダプタの送信先を差し替える非公開のヘルパー（ループバックアドレス `127.0.0.1`・`::1` 以外の URL を拒否する。architecture §3.1）。
  - `testdata/` のフィクスチャのパス定数。
  - テストが共有するアサーション: 返るエラーが 7 つの番兵と 2 つの `context` のエラーのうちちょうど 1 つに該当すること、`Error()`・`%v`・`%+v`・`%#v` のどれにも API キーが現れないこと、メッセージが `deepseek: ` で始まり、`deepseek: ` が 2 回現れないこと。
- [ ] **ステップ 3-6**: `deepseek_test.go` を作成し、§5 の表に挙げたテストを実装する（AC-01〜AC-08・AC-10・AC-15 の後半・AC-17〜AC-21・AC-25）。各失敗のケースでは、ステップ 3-5 の共有のアサーションを使う。あわせて次を置く。
  - `TestMain` で、受け付けた接続をすぐ閉じるループバックのリスナーを自分で開き、`HTTPS_PROXY`・`https_proxy`・`HTTP_PROXY`・`http_proxy` をそのアドレスに設定し、`NO_PROXY`・`no_proxy` を空にしてから、テストを実行する（固定のポート番号は使わない。他のプロセスが使っている可能性があるため）。誤って本番の送信先へ送るテストを書いた場合、外部ホストへ届かずに失敗する。ループバックの `httptest` サーバーは proxy を通らない（§1.3）。開いたリスナーは `TestMain` の終了時に閉じる。
  - `TestUnitTestsCannotReachProductionEndpoint`: 本番の送信先の URL に対して `http.ProxyFromEnvironment` が上記のリスナーのアドレスを返すことを確かめる。あわせて、`New` で作った（送信先が本番の）アダプタの `http.Client` の `Transport` が nil（`http.DefaultTransport` を使う）であることを先に確かめ、そうでなければ `Generate` を呼ばずに `t.Fatal` で止める（ステップ 3-9 の破壊の確認で、本番の送信先へ実際に送らないようにするため）。そのうえで `Generate` を呼び、`ErrTransport` が返ることと、上記のリスナーが接続を受け付けたこと（受け付けた回数を数える）を確かめる。接続先はループバックのリスナーだけで、外部ホストには届かない。`http.ProxyFromEnvironment` だけの確認では、アダプタが proxy に従わない `Transport` を使っても通ってしまうためである。
  - フェーズ 3 のコミットの時点では応答の検証が暫定の実装（ステップ 3-4）でありうるため、`TestGenerateSendsRequest`・`TestGenerateMaxTokens`・`TestGenerateNoRedirect` などは、サーバーが受け取ったリクエストと、`200` 以外・`context`・通信の失敗のエラーだけを確かめる。`200` の応答に対する成功の確認は、同じ PR-3 の `response_test.go`（フェーズ 4）で行う。
  - `TestGenerateTransportFailure` は、返るエラーが下位のエラーを連鎖に含まないこと（`errors.AsType[*url.Error]` が偽、`errors.Is(err, io.ErrUnexpectedEOF)` が偽）も確かめる（architecture §4.1 の「`%w` でつながない」）。
  - タイムアウトとキャンセルのテストは、待ち続けるハンドラをテストの終了時に解放する（`t.Cleanup`）。猶予 2 秒（architecture §3.3）は名前付き定数にする。
  - `TestGenerateCanceled` の「キャンセル済みの `ctx` ではサーバーへのリクエストが 0 回」は、アダプタの事前の確認を外しても `net/http` の Transport が送信前に `ctx` を確認するため、テストが通ってしまいかねない。アダプタの事前の確認を壊す対象は、`ctx` の確認と `Validate` の順序（`TestGenerateInvalidRequest` の、終了済みの `ctx` と不正なリクエストが同時の場合に `ErrInvalidRequest` を返すケース）とする。
- [ ] **ステップ 3-7**: `deepseek_test.go` に `TestRevealOnlyInRequestFile` を作成する。`internal/llm/deepseek` の `_test.go` 以外の Go ファイル（`test_helpers.go` を含む）を `go/parser` で解析し、名前が `Reveal` のセレクタ（呼び出しだけでなく、`f := s.Reveal` のようなメソッド値も含む）が `request.go` の中にだけ、2 か所あることを確かめる（architecture 原則 3）。
- [ ] **ステップ 3-8**: `package_reference.md` に `internal/llm/deepseek` の行を追加する（DeepSeek の Chat Completions API を呼ぶ `llm.LLMClient` の実装。送信先は固定、リダイレクトに従わない、応答本文を厳格に検証する）。
- [ ] **ステップ 3-9**: 壊して失敗することを確認し、コミットメッセージに記録する。対象は次のとおり。
  - `New` の API キーの形の検査を外す、`Model` の前後の空白の検査を外す（`TestNew`）。
  - `max_tokens` の省略を外す（`TestGenerateMaxTokens`）。
  - リクエスト本文に API キーを入れる、system と user のメッセージの順を入れ替える、`stream` を送らない（`TestGenerateSendsRequest`）。
  - `Validate` を呼ばない、`ctx` の確認を `Validate` より前に置く（`TestGenerateInvalidRequest`）。
  - `CheckRedirect` を外す（`TestGenerateNoRedirect`）。
  - `200` 以外の応答本文を読んでメッセージに含める（`TestGenerateHTTPStatus`）。
  - `http.Client.Timeout` だけで期限を設定し `context.WithTimeoutCause` を外す（`TestGenerateTimeout`）。
  - 失敗時に `ctx.Err()` を確認せず常に `ErrTransport` にする（`TestGenerateTimeout`・`TestGenerateCanceled`）。
  - `ErrTransport` に下位のエラーを `%w` でつなぐ（`TestGenerateTransportFailure`）。
  - 具体型に API キーを `string` のフィールドとして持たせる（`TestClientOutputDoesNotLeakAPIKey`）。
  - `deepseek.go` に `Reveal()` の呼び出しを足す（`TestRevealOnlyInRequestFile`）。
  - `TestMain` の proxy の設定を外す、`New` で `Proxy` を持たない `http.Transport` を設定する（`TestUnitTestsCannotReachProductionEndpoint`）。
  - ヘルパーのループバックの判定を外す（`TestNewTestClientRejectsNonLoopback`）。
- [ ] **ステップ 3-10**: `make fmt` → `make test` → `make lint` を通す。`test_helpers.go` が `make test`（`-tags test`）でコンパイルされ、`go vet -tags integration ./...`（`make lint`）でビルドに含まれないことを確認する。

### フェーズ 4: 応答の検証

**対象ファイル**
- 変更: `internal/llm/deepseek/response.go`・`internal/llm/deepseek/deepseek_test.go`（`TestGenerateSentinelsDistinct` の追加）
- 新設: `internal/llm/deepseek/response_test.go`（`//go:build test`）

**タスク**
- [ ] **ステップ 4-1**: ステップ 3-4 の検証関数を、architecture §3.4 の手順 3〜6 の実装にする（暫定の実装を置いた場合は置き換える）。`strictjson.ParseObject` で応答本文全体を検査し、トップレベル（`model`・`choices`・`system_fingerprint`）、`choices[0]`（`finish_reason` を先に、次に `message`）、`message`（`content`）の順に `Collect` で取り出す。`choices` はちょうど 1 要素で要素がオブジェクトであること、`model` は `RequiredString`、`system_fingerprint` は `OptionalString` で検査する。続いて終了理由（`length` → `llm.ErrTruncated`、`stop` でも `length` でもない値 → `llm.ErrUnexpectedFinishReason`）、`content` の空・空白だけ（`strings.TrimSpace`。→ `llm.ErrEmptyResponse`）を判定し、`Text`・`Model`・`ModelVersion` を加工せずに組み立てる。どの拒否でも `GenerateResponse` のゼロ値を返す。
- [ ] **ステップ 4-2**: `ErrInvalidResponse` と `ErrUnexpectedFinishReason` のメッセージに、architecture §4.2 が定める情報を含める。`error` メンバーの存在の確認には `Object.Has` を使う。
- [ ] **ステップ 4-3**: `response_test.go` を作成し、§5 の表に挙げたテストを実装する（AC-09・AC-11〜AC-14・AC-15 の前半・AC-26〜AC-34、keep-alive の空行、`error` メンバーの診断）。入力は `testdata/` の実応答（ステップ 3-5 のパス定数）を基に 1 か所だけを変えて作り、`httptest` サーバーから返す。各拒否のケースでは、ステップ 3-5 の共有のアサーション（番兵がちょうど 1 つ、API キーが現れない、接頭辞が 1 回）と、`GenerateResponse` がゼロ値であることを確かめる。AC-30 の不正なバイト列のケースは、入力自身が不正な UTF-8 を含むこと（`utf8.Valid` が偽）を先に確かめる。サロゲートのケースは、入力が `utf8.Valid` と `json.Valid` の両方で真であることを先に確かめる（サロゲートの検査だけが拒否できる入力であることを示す。`internal/transcript/json3_test.go` の `TestParseSubtitlesUTF8` と同じ）。AC-09 には、実応答の `content` の前後に空白と改行を足した応答を含め、`Text` が完全に一致することを確かめる（実応答の `content` は前後に空白を持たないため）。
- [ ] **ステップ 4-4**: `deepseek_test.go` に `TestGenerateSentinelsDistinct` を作成する。7 つの番兵と 2 つの `context` のエラーのそれぞれを 1 回ずつ実際の `Generate` の経路で起こし、共有のアサーションで、それぞれが自分の番兵だけに該当することを確かめる（AC-16）。
- [ ] **ステップ 4-5**: 壊して失敗することを確認し、コミットメッセージに記録する。対象は次のとおり。
  - `Text` に `strings.TrimSpace` を適用する（`TestGenerateResponseFixtures` の前後に空白を足したケース）。
  - 各レベルで `Collect` に挙げていないメンバーを拒否する（`TestGenerateResponseFixtures` の未知のメンバーを足したケース）。
  - `Model` に構築時のモデル名を入れる（`TestGenerateModelFromResponse`）。
  - `length` を `stop` と同じに扱う、未知の値を `ErrTruncated` にする（`TestGenerateFinishReason`）。
  - 終了理由の判定を `content` の判定の後へ移す（`TestGenerateValidationOrder`）。
  - `content` の判定に `strings.TrimSpace` を使わない、`reasoning_content` を `Text` に連結する（`TestGenerateEmptyContent`）。
  - `ParseObject` の代わりに `json.Unmarshal` で後続データを読み残す（`TestGenerateInvalidBody`）。
  - `content` の `null` を空文字列として受理する（`TestGenerateConsumedMembers`）。
  - `choices` の要素数を確かめず先頭だけを読む（`TestGenerateChoicesShape`）。
  - `Collect` が重複を拒否せず後の値を採る（`TestGenerateDuplicateMembers`）。
  - 応答本文の UTF-8・サロゲートの検査を外す（`TestGenerateEncoding`）。
  - 上限の比較を `>=` にする（`TestGenerateSizeLimit`）。
  - 空白だけの応答本文の診断を外す（`TestGenerateKeepAliveBlankLines`）。
  - `error` メンバーの値をメッセージに含める、存在を示さない（`TestGenerateErrorMemberDiagnostics`）。
  - `system_fingerprint` の空文字列を受理する、欠落を拒否する（`TestGenerateSystemFingerprint`）。
  - `ErrTruncated` を `ErrUnexpectedFinishReason` にもラップする（`TestGenerateSentinelsDistinct`）。
- [ ] **ステップ 4-6**: `make fmt` → `make test` → `make lint` を通す。

### PR-3 作成ポイント: DeepSeek adapter (construction, sending, and response validation)

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6

**推奨タイトル**: `feat(0003): add the DeepSeek adapter`

**レビュー観点**: `Reveal()` の呼び出しが `request.go` の 2 か所だけに固定され、エラー・ログ・`fmt` の出力のいずれにも API キーが現れないこと（AC-20・AC-21、ステップ 3-2・3-7） / 送信と読み取りの失敗が `ctx.Err()` の確認で `context` のエラーと `ErrTransport` に正しく分かれ、`ErrTransport` が下位のエラーを `%w` でつながないこと（architecture §4.1、AC-17〜AC-19） / `200` 以外の応答本文を読まず、リダイレクトに従わないこと（AC-08・AC-10） / 応答本文を `strictjson` で厳格に検証し、検証の順序と番兵の相互判別（AC-15・AC-16）、消費するメンバーの欠落・`null`・種類の不一致・重複の拒否（AC-27・AC-29）、8 MiB の上限（AC-31）が成立すること / PR の最終状態に、常にエラーを返す暫定の検証関数が残っていないこと（ステップ 3-4・4-1）

**実装モデル要件**: frontier-recommended

**判定理由**: 秘密情報の非開示（`Reveal()` の隔離）と、信頼できない API 応答を厳格に検証する境界（architecture §5.1）という高リスクなセキュリティのステップを含み、gosec の最小の抑制と build tag 下でのコンパイル確認の 2 つの Conditional checks に該当するため。送信の `200` の経路は応答の検証が揃わないと成功できず、検証を別の PR に分けると常にエラーを返す暫定の実装を main に置くことになるため、構築・送信・検証を 1 つの PR にまとめる。競合する実装方針の併記とパネルモードのトリガーには該当しない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 統合テスト

**対象ファイル**
- 新設: `internal/llm/deepseek/integration_env_test.go`（`//go:build test || integration`）・`internal/llm/deepseek/integration_test.go`（`//go:build integration`）・`internal/llm/deepseek/makefile_test.go`（`//go:build test`）
- 変更: `Makefile`・`internal/llm/deepseek/deepseek_test.go`（`TestIntegrationSettings`・`TestIntegrationTestBuildTag` の追加）

**タスク**
- [ ] **ステップ 5-1**: `integration_env_test.go` に、環境変数の読み取り関数（`getenv func(string) string`）を受け取る純粋な関数を作る。判定の順序は architecture §7.2 のとおり、オプトイン `YT2COLUMN_DEEPSEEK_INTEGRATION` が `1` でなければスキップ、`YT2COLUMN_TEST_DEEPSEEK_API_KEY` が未設定・空ならスキップ、`YT2COLUMN_MODEL` が未設定・空なら失敗とする。スキップと失敗の理由には変数名を含める（オプトインがないことによるスキップの理由には `make test-integration-deepseek` も含める）。すべて設定されていれば、`secret.New` で包んだ API キーとモデル名を返す。`DEEPSEEK_API_KEY` は読まない。このファイルは `test` と `integration` のどちらのビルドにも含まれ、テスト関数を持たない。この関数のテスト `TestIntegrationSettings` は `deepseek_test.go`（`//go:build test`）に置き、オプトインの欠如と `1` 以外の値（`0`・`true`・` 1`）、API キーの未設定・空（`DEEPSEEK_API_KEY` だけが設定されている場合を含む）、モデル名の未設定・空、すべて設定された場合に API キーとモデル名がそのまま返ることを確かめる。
- [ ] **ステップ 5-2**: `integration_test.go` に `TestIntegrationGenerate` を作成する。ステップ 5-1 の関数に `os.Getenv` を渡し、スキップなら `t.Skip`、失敗なら `t.Fatal` とする。`test_helpers.go` に依存せず、`New` で作った、送信先が本番のアダプタを使う。1 回の `Generate` のタイムアウトは 15 分（§1.5 の I-01）とし、`Options.Timeout` に渡す。サブテストは 2 つとする。
  - 正常な生成: 短い固定の英語のプロンプト（字幕・API キー・パス・個人情報を含まない）で `Generate` を呼び、エラーがなく、`Text` が空白文字以外を含み、`Model` が空でないことを確かめる。`ModelVersion` は `%+q` で `t.Logf` に出力する（I-03）。
  - 打ち切り: `MaxOutputTokens` 16（architecture §3.6）と長い出力を求めるプロンプトで `llm.ErrTruncated` を確かめる。エラーが返らなかった場合のメッセージは I-04 のとおりとする。
  - テストの出力に API キーもその一部も書かない。
  - `Generate` を呼ぶ前に `GODEBUG` を確認し、`http2debug=1`・`http2debug=2` が含まれる場合はそれらを取り除くか、検出して失敗する。継承された設定で実キーの `Authorization` ヘッダーと API キーが標準エラー出力に漏れないようにする（architecture §5.4）。この方針は `make test-integration-deepseek` から実行した場合と、ステップ 5-6 の直接実行の両方に効かせる。
- [ ] **ステップ 5-3**: `Makefile` に `test-integration-deepseek` を追加し、`.PHONY` に加える。レシピは、実 API を使い料金が発生することを表示してから、`$(GOTEST) -tags integration -count=1 -timeout $(DEEPSEEK_INTEGRATION_TIMEOUT) -v ./internal/llm/deepseek` を実行する（`-run` は使わない。I-02）。`DEEPSEEK_INTEGRATION_TIMEOUT ?= 40m`（I-01）、`YT2COLUMN_MODEL ?= deepseek-flash` とし、`YT2COLUMN_MODEL` と `YT2COLUMN_DEEPSEEK_INTEGRATION=1` は、ターゲット固有の `export` でこのターゲットにだけエクスポートする。値をシェルのテキストに埋め込まない（既存の `test-integration` の方針）。既存の `test-integration` は変更しない。
- [ ] **ステップ 5-4**: `makefile_test.go` に `TestMakeTestIntegrationDeepSeek` を作成する。`t.TempDir` に、受け取った引数と関係する環境変数を書き出すだけのスタブを置き、リポジトリのルートで `make -s test-integration-deepseek GOTEST=<スタブ>` を実行して、次を確かめる。実 API もネットワークも使わない。
  - 料金の発生を示す表示があること。
  - 引数に `-tags integration`・`-count=1`・`-timeout`（値つき）・`-v` があり、`./internal/llm/deepseek` がフラグの値ではなくパッケージの引数として渡ること。
  - `YT2COLUMN_DEEPSEEK_INTEGRATION` が `1` で渡ること。
  - `YT2COLUMN_MODEL` が、環境で未定義なら `deepseek-flash`、空文字列に定義されていれば空文字列、値があればその値で渡ること。
  - スタブが実際に呼ばれたこと（スタブの出力ファイルが存在すること）。レシピが `$(GOTEST)` を経由しない形に変わった場合に、この確認が失敗する。
  - `make` が `PATH` にない場合はスキップせず失敗させる（CI の `ubuntu-latest` とローカル開発には `make` がある）。
  - `make` のすべての呼び出しで、子の環境をテストが明示的に組み立てる。外側の `make test` から伝わる `MAKEFLAGS`・`MFLAGS`・`MAKELEVEL`（コマンドラインの変数が `MAKEFLAGS` を通じて伝わるため。§1.3）と、`YT2COLUMN_TEST_DEEPSEEK_API_KEY`・`DEEPSEEK_API_KEY` は子の環境から除く。ステップ 3-6 の proxy の変数は残す。`YT2COLUMN_MODEL` は各ケースの値だけを与える。
- [ ] **ステップ 5-5**: `deepseek_test.go` に `TestIntegrationTestBuildTag` を作成する。`integration_test.go` の先頭行が `//go:build integration`、`integration_env_test.go` の先頭行が `//go:build test || integration` であることを確かめる（AC-22。`internal/transcript/ytdlp_test.go:1692` と同じ形）。
- [ ] **ステップ 5-6**: 人間の明示的な承認を得て（CLAUDE.md の Tool Execution Safety。料金が発生する）、`YT2COLUMN_TEST_DEEPSEEK_API_KEY` を設定した環境で `make test-integration-deepseek` を実行する。実行日時・HEAD・`-v` 出力の `TestIntegrationGenerate` とその 2 つのサブテストの `=== RUN`・`--- PASS` の行（`--- SKIP` がないこと）・所要時間・`ModelVersion` のログ行を §5.1 に記録する。API キーは記録しない。あわせて、オプトインの変数を設定せずに `go test -tags integration -count=1 -v ./internal/llm/deepseek` を実行し、`--- SKIP` と変数名を含むメッセージが出て API を呼ばないことを §5.1 に記録する。
- [ ] **ステップ 5-7**: 壊して失敗することを確認し、コミットメッセージに記録する。対象は次のとおり。
  - レシピに `-run` を足してパッケージのパスを正規表現として渡す、`YT2COLUMN_MODEL ?=` を `:=` にする、オプトインのエクスポートを外す（`TestMakeTestIntegrationDeepSeek`）。
  - オプトインの判定を外す、`DEEPSEEK_API_KEY` を代わりに読む（`TestIntegrationSettings`）。
  - `integration_test.go` と `integration_env_test.go` の build tag を変える（`TestIntegrationTestBuildTag`）。
- [ ] **ステップ 5-8**: `make fmt` → `make test` → `make lint` を通す。`make lint` の `go vet -tags integration ./...` が `integration_test.go` と `integration_env_test.go` を `test_helpers.go` なしでコンパイルすることを確認する。`-tags integration` のビルドのテスト関数が `TestIntegrationGenerate` だけであることを `go test -tags integration -list . ./internal/llm/deepseek` で確かめる（`-list` はテストを実行しない）。

### フェーズ 6: ドキュメント

**対象ファイル**
- 変更（PR-5）: `docs/dev/project_overview.md`・`CLAUDE.md`・`README.md`
- 変更（`security.md` §2 のステップ 6-3・6-4 は、実キーを使う統合テストと同じ PR-4 で更新する）: `docs/dev/security.md`
- 確認のみ（変更しない。フェーズ 1〜3 のステップ 1-7・2-5・3-8 で更新済み）: `docs/dev/developer_guide/package_reference.md`

**タスク**

PR-4 に属するステップ 6-3・6-4 を PR-4 作成ポイントの前に、PR-5 に属する残りのステップをその後に置く（§3.2 の不変条件）。ステップの番号は他の節からの参照を保つため変えず、文書上の順序だけを入れ替えている。

- [ ] **ステップ 6-3**: `security.md` §2 に、統合テストに限って実在の API キーを使う例外と、その範囲・安全策（`//go:build integration`、オプトインの変数、テスト専用の `YT2COLUMN_TEST_DEEPSEEK_API_KEY`、出力に API キーを書かない）を追記する（architecture §5.2）。
- [ ] **ステップ 6-4**: `security.md` §2 に、`GODEBUG` に `http2debug=1` または `http2debug=2` を含めると `Authorization` ヘッダーが標準エラー出力に出るため、この設定で調査するときは無効な API キーを使うこと、統合テストは `Generate` の前にこれらの設定を取り除くか検出して失敗することを追記する（architecture §5.4、§1.3。ステップ 5-2）。

### PR-4 作成ポイント: integration test and Makefile target

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 / 5-8 / 6-3 / 6-4

**推奨タイトル**: `feat(0003): add the DeepSeek integration test and make target`

**レビュー観点**: 統合テストが `//go:build integration` で既定の `make test`・`make test-ci` から分離され、`-tags integration` のビルドのテスト関数が `TestIntegrationGenerate` だけであること（AC-22、I-02、ステップ 5-1・5-5・5-8） / 環境変数の判定が純粋な関数で、オプトインの欠如・`1` 以外の値・キーの未設定・`DEEPSEEK_API_KEY` だけの設定をスキップし、`YT2COLUMN_MODEL` の未設定・空を失敗とすること（AC-23、ステップ 5-1・5-4） / `make test-integration-deepseek` の引数・表示・環境変数の渡し方と、`GODEBUG` の `http2debug=1`・`http2debug=2` を `Generate` の前に取り除くか検出して失敗させること（AC-23、architecture §5.4、ステップ 5-2・5-3・5-4） / 人間の承認を得て手動実行し、結果を §5.1 に記録していること（AC-24、ステップ 5-6） / `security.md` §2 の実キーの例外と `http2debug` のポリシー更新（ステップ 6-3・6-4）が、実キーを使う統合テストと同じこの PR に含まれ、`main` がマージ直後から文書化されたポリシーと一致すること（AC-23・AC-24、architecture §5.2・§5.4）

**実装モデル要件**: frontier-required

**判定理由**: ステップ 5-2〜5-6 が実 DeepSeek API とネットワーク・料金・手動実行にわたる重い統合テストで、mkplan.md ステップ 8 のパネルモードトリガー（重い統合テスト / 外部リソースの面）に該当するため。`security.md` §2 の更新（ステップ 6-3・6-4）を同じ PR に置くのは、実キーを使うテストの追加と文書化されたポリシーを `main` 上で同時に一致させるためである。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

- [ ] **ステップ 6-1**: `project_overview.md` を更新する。想定ディレクトリ構成に `internal/strictjson/` を加える（検証を通った JSON 文書から値を取り出す厳格な部品）。`:40` の `GenerateResponse` の説明に `ModelVersion`（生成に使ったモデルまたはバックエンドの版の識別子。返さないプロバイダでは空文字列）を加える。`testdata/` の行に DeepSeek の API の実応答を加える。決定済みの方針は変更しない（requirements §5.1）。
- [ ] **ステップ 6-2**: `CLAUDE.md` の Architecture Overview のパッケージの説明に `internal/strictjson`（`internal/transcript` と `internal/llm/deepseek` が共有する厳格な JSON の部品）を加える。
- [ ] **ステップ 6-5**: `README.md` の Development の節に `make test-integration-deepseek` を加え、既存の `make test-integration` の説明（`README.md:81-90`）と同じ形で、実 API を使い料金が発生すること、`YT2COLUMN_TEST_DEEPSEEK_API_KEY`（本番の `DEEPSEEK_API_KEY` とは別）が必要なこと、`YT2COLUMN_MODEL` の既定値、ターゲットがオプトインの変数を設定すること、オプトインがなければ統合テストがスキップされることを説明する。
- [ ] **ステップ 6-6**: 追記した内容を根拠と突き合わせる。6-3 と 6-5 は `integration_env_test.go`・`integration_test.go`・`Makefile` の実装と、6-4 は §1.3 に記した go1.27.1 のソース（`src/net/http/internal/http2/http2.go:50-56`・`transport.go:1849-1851`）と照合する。6-1・6-2 のパッケージの説明は `package_reference.md` の行と照合する。照合した根拠をコミットメッセージに書く。
- [ ] **ステップ 6-7**: `package_reference.md` の `internal/strictjson`・`internal/llm`・`internal/llm/deepseek` の行が、フェーズ 1〜5 の最終的な実装と一致していることを確認する。
- [ ] **ステップ 6-8**: `make test` → `make lint` を通す（Go の変更はない）。

### PR-5 作成ポイント: documentation

**対象ステップ**: 6-1 / 6-2 / 6-5 / 6-6 / 6-7 / 6-8

**推奨タイトル**: `docs(0003): update the DeepSeek adapter documentation`

**レビュー観点**: `project_overview.md`・`CLAUDE.md`・`README.md` の記述が実装と一致し、`package_reference.md` の 3 行がフェーズ 1〜3 の更新どおりであることを確認していること（ステップ 6-6・6-7） / 追記の根拠を go1.27.1 のソースと実装に突き合わせ、その根拠がコミットメッセージに記録されていること（ステップ 6-6）

**実装モデル要件**: standard

**判定理由**: ドキュメントの記述の整合と根拠との照合のみで、競合する実装方針の併記・高リスクな制御・パネルモードのトリガー・Conditional checks のいずれにも該当しないため。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 内容 | 成果物 | 完了条件 |
|---|---|---|---|
| M1 | フェーズ 1 | `internal/strictjson`、`json3.go`・`info.go` の書き換え、§5.2 の測定の記録 | `make test` / `make lint` が通り、`internal/transcript` のテストが無変更で通る |
| M1a | フェーズ 1a（条件付き） | 件数の上限付きの分割の最適化と §5.2 の再測定の記録（ステップ 1-6 の測定が 256 MiB 以下なら `[-]`） | `make test` / `make lint` が通る |
| M2 | フェーズ 2 | `llm.go` の番兵・`Validate`・`ModelVersion`・doc コメント、`llm_test.go`、`pipeline_test.go` の更新 | `make test` / `make lint` が通る |
| M3 | フェーズ 3 | `errors.go`・`deepseek.go`・`request.go`・`response.go`（読み取り）・`test_helpers.go`・`deepseek_test.go` | 同上 |
| M4 | フェーズ 4 | `response.go` の検証、`response_test.go`、`TestGenerateSentinelsDistinct` | 同上 |
| M5 | フェーズ 5 | 統合テスト、`Makefile` の `test-integration-deepseek`、§5.1 の記録 | 同上・`make test-integration-deepseek` が通る |
| M6 | フェーズ 6 | `project_overview.md`・`CLAUDE.md`・`security.md`・`README.md`・`package_reference.md` | 同上・根拠との照合を済ませた |

### 3.2. PR 構成

PR はフェーズを単位とするが、フェーズ 3（アダプタの構築と送信）とフェーズ 4（応答の検証）は 1 つの PR-3 にまとめる。各 PR は主たる関心事（厳格な JSON の切り出し / プロバイダ共通 API / DeepSeek アダプタ / 統合テストと Makefile / ドキュメント）を持ち、単独でグリーンゲートを通せる単位とする。本タスクは `cmd/yt2column/main.go` を変更しないため（配線は #6）、`internal/` の変更が `cmd/` に先行する順序の問題は生じない。

PR-1（`internal/strictjson` の切り出し）と PR-2（`internal/llm` の追加）は、アダプタが依存する共有の部品を先に完成させる。PR-3 はアダプタの構築・送信・失敗の分類（フェーズ 3）と応答の検証（フェーズ 4）をまとめる。送信の `200` の経路は応答の検証が揃わないと成功できず、検証を別の PR に分けると、常にエラーを返す暫定の検証関数を先の PR の時点で main に入れることになる（ステップ 3-4、§6 のリスク）。そのため構築・送信・検証を 1 つの PR にまとめ、秘密情報の非開示と信頼できない応答の検証という高リスクなステップを同じ PR のレビュー観点に集約する。PR-4（統合テストと Makefile）は実 API・ネットワーク・料金・手動実行に触れるため frontier-required とする。`security.md` §2 の実キーの例外と `http2debug` の注意（ステップ 6-3・6-4）は、実キーを使う統合テスト（ステップ 5-1〜5-8）と同じ PR-4 に置き、ポリシーの記述と `main` の状態が PR-4 のマージ直後から一致するようにする。PR-5 は残りのドキュメントのみである。また、ステップ 1-6 の測定が 256 MiB を超えた場合の件数の上限付きの分割の最適化は、PR-1 には含めず、条件付きの PR-1a とする（PR-1 はスカッシュマージされるため、最適化を独立して revert できる必要がある）。`package_reference.md` は PR-1〜PR-3 で更新し、PR-5 では実装との一致を確認するだけである。

**PR の区切りの不変条件。** `/runplan` は §2 を文書の順に走査し、`PR-N 作成ポイント` に達したときにだけ PR を作る。そのため §2 は次を満たす。(1) 文書の順で、すべてのステップは、自分の PR の 1 つ前の作成ポイント（先頭の PR では文書の先頭）と自分の PR の作成ポイントの間にあり、他の PR のステップがその間に入らない。(2) 条件付きの PR を含め、すべての PR が作成ポイントを持ち、§3.2 の表・§7 のチェックリスト・§9 の実行順に現れる。ステップの番号はフェーズに従うため、文書の順と番号の順は一致しないことがある（フェーズ 6 のステップ 6-3・6-4）。

**条件付きの PR。** PR-1a（ステップ 1-10・1-11）は、ステップ 1-6 の測定が 256 MiB を超えた場合にだけ実施する。実施しないと判断したステップ（1-6）で、PR-1a の対象ステップと作成ポイントのチェックボックスをすべて `[-]`（理由つき）にする。`/runplan` は `[-]` を完了とみなして次へ進むため、実施しない PR-1a で止まらない。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 | `internal/strictjson` の新設、`internal/transcript/json3.go`・`info.go` の書き換え、`package_reference.md` の更新 | standard |
| PR-1a（条件付き） | 1-10 / 1-11 | ステップ 1-6 の測定が 256 MiB を超えた場合だけ、`internal/strictjson` の `AsArray` の件数の上限付きの分割の最適化と再測定の記録。超えない場合は `[-]` | standard |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 | `internal/llm` の番兵・`Validate`・`ModelVersion`・doc コメント、`llm_test.go`、`pipeline_test.go` の更新 | standard |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 | `internal/llm/deepseek` の構築・送信・失敗の分類・応答の検証、`test_helpers.go`・`deepseek_test.go`・`response_test.go`、`package_reference.md` の更新 | frontier-recommended |
| PR-4 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 / 5-8 / 6-3 / 6-4 | 統合テスト、`integration_env_test.go`・`makefile_test.go`、`Makefile` の `test-integration-deepseek`、§5.1 の記録、`security.md` §2 の実キーの例外と `http2debug` のポリシー更新 | frontier-required |
| PR-5 | 6-1 / 6-2 / 6-5 / 6-6 / 6-7 / 6-8 | `project_overview.md`・`CLAUDE.md`・`README.md` の更新と根拠との照合（`security.md` §2 は PR-4、`package_reference.md` は確認のみ） | standard |

### 3.3. 実装順序の根拠

architecture §8 の順序に従う。`internal/strictjson` はフェーズ 4 の応答の検証が使うため最初に置き、既存テストで振る舞いの保存を確かめられる独立したリファクタリングとする。`internal/llm` の番兵と `Validate` はアダプタが使うため次に置く。アダプタは構築・送信・失敗の分類（フェーズ 3）と応答の検証（フェーズ 4）からなるが、送信の成功の経路は検証と不可分であるため 1 つの PR にまとめる（§3.2）。応答の検証が揃ってから統合テストを置き、ドキュメントは実装の確定後に更新する。

## 4. テスト戦略 (Test Strategy)

architecture §7 のテスト戦略に従う。テスト関数名と AC の対応は §5 に示す。

### 4.1. ユニットテスト

- DeepSeek の API もネットワーク上の外部ホストも呼ばない（AC-25）。`internal/llm/deepseek` のユニットテストは `test_helpers.go` のヘルパーで送信先をループバックの `httptest` サーバーに差し替え、`TestMain` が proxy をループバックに固定する（ステップ 3-6）。テストの API キーは実在しない固定の文字列とする。
- 応答の検証のテストは `testdata/` の実応答を基に 1 か所だけを変えた入力を使う（AC-32）。
- タイムアウト・キャンセルのテストは時間の厳密な比較をせず、「構築時のタイムアウト + 猶予 2 秒」以内に戻ることで判定する（architecture §3.3）。
- `internal/strictjson` のテストは、公開 API ごとの最小の入力に限り、`internal/transcript` のテストが使う `testdata/` のサンプルと実データのケースを繰り返さない（architecture §7.1）。網羅率の目標は、`internal/strictjson` と `internal/llm/deepseek` の本番コードの、到達できるすべての文をそれぞれのパッケージのテストが通ること（文の網羅率）とする。到達できない文は次の 3 つである。`request.go` のヘッダー設定時の `Reveal()` のエラーの分岐（architecture §3.7）、固定の構造体の `json.Marshal` のエラーの分岐、定数の URL での `http.NewRequestWithContext` のエラーの分岐。`internal/strictjson` では、`ParseObject` が検証済みの値に対する `As...`・`Collect` の中のデコーダの読み取り（`Token`・`Decode`）のエラーの分岐が、公開 API からは到達できない（`Value`・`Object` は検証を通った文書からしか作れないため）。これらを到達できない文として扱い、非公開のフィールドに不正なバイト列を入れて分岐を通すテストは書かない（起こりえない状態を固定するだけになるため）。`go test -tags test -coverprofile` の出力を `go tool cover -html` で開き、網羅されていない文が上に挙げたものだけであることを確認する。
- 各テストは、対象の仕組みを実際に壊して失敗することを確認してからコミットする。壊す対象と対応するテスト名は各フェーズのタスクに示す。
- **後方互換性:** `internal/transcript` の振る舞い（返る番兵とその条件）は変えず、既存テストを変更せずに通す（ステップ 1-5）。`GenerateResponse` に `ModelVersion` を加えるため、`TestCommonTypesFieldSets` の期待値を更新する（ステップ 2-4）。`FakeLLMClient` とそのテストは変わらない。

### 4.2. 統合テスト

architecture §7.2 と F-006 に従う。正常な生成と打ち切りを 1 つのテスト関数のサブテストとして順に実行する（`Generate` 2 回。I-01 の `-timeout` の前提）。環境変数の判定はステップ 5-1 の純粋な関数で行い、`make test` で単体テストする。実 API での実行は人間の承認を得てから行い、結果を §5.1 に記録する。

### 4.3. セキュリティテスト

architecture §7.3 に従う。計画固有の事項は次のとおり。

- API キーの非漏洩（AC-20）は、失敗のケースを持つすべてのテストで共有のアサーションを使って確かめる。
- `Reveal()` の呼び出し箇所を `TestRevealOnlyInRequestFile` で固定する（ステップ 3-7）。
- 本番の送信先へのリクエストが外部へ届かないことを、`TestMain` の proxy と `TestUnitTestsCannotReachProductionEndpoint` で確かめる（ステップ 3-6）。

### 4.4. テストヘルパー

- `internal/llm/deepseek/test_helpers.go`（`//go:build test`、`test_organization.md` 分類 B）に、送信先の差し替えのヘルパー、フィクスチャのパス定数、共有のアサーションを置く。非公開の API を使うため分類 B である。
- `integration_env_test.go` は、`test` と `integration` の両方のビルドに含める必要がある純粋な関数のために設ける（`integration_test.go` は `test_helpers.go` に依存できず、`test_helpers.go` は `integration` のビルドに含まれないため）。`_test.go` であるため本番のビルドには含まれない。テスト関数は置かず、`-tags integration` のビルドのテスト関数を `TestIntegrationGenerate` だけにする（I-02）。
- `makefile_test.go` は、パッケージの振る舞いではなく `Makefile` のターゲットを検証するため、`deepseek_test.go` と分ける。
- `testutil/` は追加しない（新しい公開 interface の fake は不要。`TestFakesCarryBuildTag` の固定件数を変えない）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は実行可能なテスト、`static` は guard テスト・`make` ターゲットを指す。`manual` は補助的な確認であり、`test` または `static` を置き換えない。表の「失敗ケースの共有アサーション」は、ステップ 3-5 の共有のアサーションを、そのテストの各失敗ケースで使うことを指す。

| AC | 内容 | 種別 | 検証の実行場所 |
|---|---|---|---|
| AC-01 | 有効な構築で `llm.LLMClient` が返る | test | `internal/llm/deepseek/deepseek_test.go::TestNew` |
| AC-02 | 不正な API キー・モデル名・タイムアウトの構築を拒否 | test | `internal/llm/deepseek/deepseek_test.go::TestNew`（ゼロ値の `Secret` と、API キーの文字の契約、すなわち表示可能な ASCII `0x21`〜`0x7E` の範囲外のバイトをすべて拒否すること（内部の制御文字・境界・非 ASCII を代表的な入力として含める）。空・前後に空白・不正な UTF-8 のモデル名、0 と負のタイムアウトでもエラーと nil を返す。メッセージにモデル名も API キーも現れないこと） |
| AC-03 | `POST` 1 回、`Content-Type`・`Authorization`、本文と URL に API キーなし | test | `internal/llm/deepseek/deepseek_test.go::TestGenerateSendsRequest` |
| AC-04 | モデル名・2 件のメッセージ列・非ストリーミング、プロンプトの同一性 | test | `internal/llm/deepseek/deepseek_test.go::TestGenerateSendsRequest`（前後の空白・改行・`<`・`&` を含むプロンプトを、デコードした文字列で比較） |
| AC-05 | `max_tokens` の有無 | test | `internal/llm/deepseek/deepseek_test.go::TestGenerateMaxTokens` |
| AC-06 | `thinking` を含まない | test | `internal/llm/deepseek/deepseek_test.go::TestGenerateSendsRequest` |
| AC-07 | 不正なリクエストは `ErrInvalidRequest` で送信 0 回 | test | `internal/llm/llm_test.go::TestGenerateRequestValidate`・`internal/llm/deepseek/deepseek_test.go::TestGenerateInvalidRequest`（サーバーへのリクエスト 0 回。既に終了した `ctx` と同時の場合も `ErrInvalidRequest`） |
| AC-08 | リダイレクトに従わない | test | `internal/llm/deepseek/deepseek_test.go::TestGenerateNoRedirect`（`307` と同じサーバーの別パスの `Location`、別パスへのリクエスト 0 回、`StatusCode` 307） |
| AC-09 | 成功時の `Text`・`Model` | test | `internal/llm/deepseek/response_test.go::TestGenerateResponseFixtures`（実応答の `content`、および `content` の前後に空白と改行を足した応答で、完全一致）・`TestGenerateModelFromResponse`（応答の `model` を構築時と異なる値に変えた応答） |
| AC-10 | `200` 以外は `ErrHTTPStatus`、ステータスコード、本文の目印が現れない | test | `internal/llm/deepseek/deepseek_test.go::TestGenerateHTTPStatus`（`400`・`401`・`402`・`429`・`500`・`503`・`307`） |
| AC-11 | `length` は `ErrTruncated`、ゼロ値 | test | `internal/llm/deepseek/response_test.go::TestGenerateFinishReason`（`content` が空でない `length`）・`TestGenerateResponseFixtures`（実応答の `length`） |
| AC-12 | `stop`・`length` 以外は `ErrUnexpectedFinishReason` | test | `internal/llm/deepseek/response_test.go::TestGenerateFinishReason`（`content_filter`・`insufficient_system_resource`・`tool_calls`・未知の値） |
| AC-13 | 空・空白だけの `content` は `ErrEmptyResponse` | test | `internal/llm/deepseek/response_test.go::TestGenerateEmptyContent`（空文字列、半角空白・改行・タブ、全角空白） |
| AC-14 | `reasoning_content` を含めない | test | `internal/llm/deepseek/response_test.go::TestGenerateEmptyContent`（`reasoning_content` の目印が `Text` に現れない、`reasoning_content` だけで `content` が空なら `ErrEmptyResponse`） |
| AC-15 | 検証の順序 | test | `internal/llm/deepseek/response_test.go::TestGenerateValidationOrder`（`length` かつ空の `content` は `ErrTruncated`）・`internal/llm/deepseek/deepseek_test.go::TestGenerateHTTPStatus`（`200` 以外かつ不正な JSON は `ErrHTTPStatus` で `ErrInvalidResponse` でない） |
| AC-16 | 番兵の相互の区別 | test | `internal/llm/deepseek/deepseek_test.go::TestGenerateSentinelsDistinct`・失敗ケースの共有アサーション（`deepseek_test.go`・`response_test.go` の全失敗ケース） |
| AC-17 | タイムアウトは猶予 2 秒以内に `context.DeadlineExceeded` | test | `internal/llm/deepseek/deepseek_test.go::TestGenerateTimeout`（ヘッダーを返さないサーバー、応答本文の途中で止まるサーバー、呼び出し元の期限が短い場合のメッセージの出所） |
| AC-18 | キャンセルは `context.Canceled`、キャンセル済みなら送信 0 回 | test | `internal/llm/deepseek/deepseek_test.go::TestGenerateCanceled` |
| AC-19 | 通信の失敗は `ErrTransport` | test | `internal/llm/deepseek/deepseek_test.go::TestGenerateTransportFailure`（接続をすぐ閉じるリスナー、`Content-Length` より前に切断するサーバー。`context` のエラーにも `ErrInvalidResponse` にも該当せず、ゼロ値） |
| AC-20 | エラーの各書式に API キーが現れない | test | 失敗ケースの共有アサーション（`TestNew`・`TestGenerateInvalidRequest`・`TestGenerateNoRedirect`・`TestGenerateHTTPStatus`・`TestGenerateFinishReason`・`TestGenerateEmptyContent`・`TestGenerateTimeout`・`TestGenerateCanceled`・`TestGenerateTransportFailure` と、`response_test.go` の 3.2 の各拒否のテスト）・`internal/llm/deepseek/deepseek_test.go::TestRevealOnlyInRequestFile` |
| AC-21 | アダプタの値の出力に API キーが現れない | test | `internal/llm/deepseek/deepseek_test.go::TestClientOutputDoesNotLeakAPIKey`（`fmt` の `%v`・`%+v`・`%#v`、`slog` の TextHandler・JSONHandler、`encoding/json`） |
| AC-22 | 統合テストは `make test`・`make test-ci` に含まれない | static | `internal/llm/deepseek/deepseek_test.go::TestIntegrationTestBuildTag`（`integration_test.go` と `integration_env_test.go` の先頭行）・`make test`・`make test-ci`（`-tags test` だけでビルドする） |
| AC-23 | `make test-integration-deepseek` のフラグ・表示・スキップ | test / manual | `internal/llm/deepseek/makefile_test.go::TestMakeTestIntegrationDeepSeek`（表示、`-count=1`・`-timeout`・`-v`・パッケージの引数、オプトインと `YT2COLUMN_MODEL` の渡し方）・`internal/llm/deepseek/deepseek_test.go::TestIntegrationSettings`（オプトインの欠如・`1` 以外の値と `YT2COLUMN_TEST_DEEPSEEK_API_KEY` の未設定・空でスキップし理由に変数名を含む、`DEEPSEEK_API_KEY` だけではスキップ、`YT2COLUMN_MODEL` の未設定・空で失敗、すべて設定されていれば API キーとモデル名を返す）・§5.1 の手動実行の記録（`TestIntegrationGenerate` の 2 つのサブテストの `--- PASS`） |
| AC-24 | 実 API での正常な生成と打ち切り | test / manual | `internal/llm/deepseek/integration_test.go::TestIntegrationGenerate`（`make test-integration-deepseek` で実行）・§5.1 の記録 |
| AC-25 | ユニットテストは外部ホストを呼ばない | test | `internal/llm/deepseek/deepseek_test.go::TestUnitTestsCannotReachProductionEndpoint`（`TestMain` の proxy の固定）・`TestNewTestClientRejectsNonLoopback`（ヘルパーがループバック以外を拒否） |
| AC-26 | 不正な JSON・空・オブジェクト以外・後続データ | test | `internal/llm/deepseek/response_test.go::TestGenerateInvalidBody` |
| AC-27 | 消費するメンバーの欠落・`null`・種類の不一致、空の `model` | test | `internal/llm/deepseek/response_test.go::TestGenerateConsumedMembers`（`ErrEmptyResponse` でないことも確かめる） |
| AC-28 | `choices` の要素数と要素の種類 | test | `internal/llm/deepseek/response_test.go::TestGenerateChoicesShape` |
| AC-29 | 消費するメンバーの重複 | test | `internal/llm/deepseek/response_test.go::TestGenerateDuplicateMembers`（トップレベル・`choices[0]`・`message` の各レベル、値が同じ場合も異なる場合も。消費しないメンバーの重複と種類の違いは受理） |
| AC-30 | 不正な UTF-8・対になっていないサロゲート | test | `internal/llm/deepseek/response_test.go::TestGenerateEncoding`（`content` と `reasoning_content` のそれぞれ） |
| AC-31 | 8 MiB ちょうどは受理、超過は拒否 | test | `internal/llm/deepseek/response_test.go::TestGenerateSizeLimit`（後ろに空白を足した 8 MiB ちょうど、先頭の空行を含めて 8 MiB ちょうど、8 MiB + 1 バイト） |
| AC-32 | 消費しないメンバーを無視して受理、実応答のフィクスチャ | test | `internal/llm/deepseek/response_test.go::TestGenerateResponseFixtures`（実応答と、各レベルに未知のメンバーを足した応答） |
| AC-33 | `ModelVersion` の組み立て | test | `internal/llm/deepseek/response_test.go::TestGenerateSystemFingerprint`（実応答の `system_fingerprint` と一致、メンバーを除いた応答で空文字列） |
| AC-34 | `system_fingerprint` の `null`・数値・空・重複 | test | `internal/llm/deepseek/response_test.go::TestGenerateSystemFingerprint` |

AC に対応しない、architecture が求める検証は次のとおり。

| 対象 | 検証の実行場所 |
|---|---|
| `internal/strictjson` の公開 API（architecture §3.5・§7.1） | `internal/strictjson/strictjson_test.go::TestParseObject`・`TestObjectCollect`・`TestObjectHas`・`TestRequiredAndRequiredString`・`TestOptionalString`・`TestValueAccessors` |
| 部品の移動で `internal/transcript` の振る舞いが変わらない（architecture §3.5） | `internal/transcript` の既存テスト（無変更。ステップ 1-5） |
| keep-alive の空行と空白だけの応答本文（architecture §3.4） | `internal/llm/deepseek/response_test.go::TestGenerateKeepAliveBlankLines` |
| `error` メンバーの診断（architecture §4.2） | `internal/llm/deepseek/response_test.go::TestGenerateErrorMemberDiagnostics`（存在がメッセージに示され、値が現れない） |
| `GenerateResponse` のフィールドと `LLMClient` の doc コメントの条項（architecture §3.2） | `internal/pipeline/pipeline_test.go::TestCommonTypesFieldSets`・`TestInterfaceDocComments` |

### 5.1. 手動実行の記録 (AC-23・AC-24)

ステップ 5-6 で記入する。

| 項目 | 内容 |
|---|---|
| 実行日時 | （未実施） |
| HEAD | （未実施） |
| コマンド | `make test-integration-deepseek` |
| `-v` 出力の `TestIntegrationGenerate` と 2 つのサブテストの `=== RUN`・`--- PASS` | （未実施） |
| 所要時間 | （未実施） |
| `ModelVersion` のログ行 | （未実施） |
| オプトインなしの直接実行の `--- SKIP` とメッセージ | （未実施） |

### 5.2. 測定の記録（`internal/strictjson` のメモリ使用量）

ステップ 1-6 で記入する。GC を無効にして `parseSubtitles` を 1 回呼び、`runtime.MemStats` の `TotalAlloc` の差と、呼び出し後の `HeapInuse`（GC が動かないため呼び出し中の最大値とみなせる）を測った（`-tags test`、2026-10-02、`go1.27.1`）。

| 項目 | 内容 |
|---|---|
| 入力 | `{"events":[0,0,…]}` の形で 1 バイトの要素を並べ、末尾を空白で埋めて 8 MiB（8,388,608 バイト）ちょうどにした文書。要素数は 4,194,298（`AsArray` が作る要素数が最大になる形） |
| 移動前（HEAD `00573f2`）の `TotalAlloc` の差・`HeapInuse` の最大値 | 24 MiB・32 MiB（先頭の要素がオブジェクトでないため、イベントの走査は最初の要素で失敗する） |
| 移動後の `TotalAlloc` の差・`HeapInuse` の最大値 | 738 MiB・750 MiB（`AsArray` が全要素を先に分割するため） |
| 判断 | 750 MiB は 256 MiB を超えるため、条件付きの PR-1a（ステップ 1-10・1-11）で `AsArray` の分割に件数の上限を加える。ステップ 1-10 で同じ入力で再測定する |

## 6. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 部品の移動で `internal/transcript` のエラーの理由が変わる（件数の上限の判定順など。architecture §3.5） | メッセージの文言が変わる | 返る番兵と `*ParseError` は変えない。既存テストは番兵と `*ParseError` だけで判定しており（architecture §1.3）、無変更で通ることを確認する（ステップ 1-5）。 |
| `AsArray` で配列を要素に分けると、小さな要素が大量に並ぶ 8 MiB の入力でメモリ使用量が増える | 字幕の解析でメモリを多く使う | ステップ 1-6 で測定し、256 MiB を超える場合にだけ条件付きの PR-1a（ステップ 1-10）で対策する。 |
| `test_helpers.go` は `_test.go` ではないため、`.golangci.yml` のテスト向けの除外（errcheck・err113・goconst・dupl・gosec など）が効かない | `make lint` が通らない | 未チェックのエラーを残さない、静的エラーを使う、固定の文字列を定数にする。`make lint` は `--build-tags test,integration` で解析する。 |
| `gosec` が可変の URL での HTTP リクエスト（G107 など）を指摘する | `make lint` が通らない | 指摘された場合は、リクエストの作成の 1 行に限った `//nolint:gosec` と、送信先が定数またはテストのヘルパーが検査したループバックの URL だけである理由のコメントを付ける。ファイル・パッケージ単位の抑制はしない。 |
| フェーズ 3 の暫定の検証関数がそのまま残る | 成功の経路が動かない | ステップ 4-1 で置き換える。暫定の実装は常に `ErrInvalidResponse` を返し、検証していない内容を成功として返さない。フェーズ 4 の `TestGenerateResponseFixtures` が成功の経路を確かめる。 |
| タイムアウトのテストが `-race` 付きの CI で不安定になる | CI が断続的に失敗する | 猶予 2 秒で判定し、厳密な時間比較をしない（architecture §3.3）。待ち続けるハンドラは `t.Cleanup` で解放する。 |
| `TestMain` の proxy の設定が、`-tags test,integration` で統合テストを実行したときにも効く | その実行では統合テストが本番の送信先へ届かず失敗する | 統合テストは `make test-integration-deepseek`（`-tags integration` だけ）で実行する。失敗は外部へ送らない側に倒れる。 |
| 統合テストが API の混雑で `context.DeadlineExceeded` になる | 料金のかかる再実行 | 1 回の `Generate` のタイムアウトを API の待ちの上限より長くする（I-01）。結果の読み方は architecture §7.2 のとおり。 |
| 統合テストの打ち切りの確認が、モデルの入れ替えで `stop` になる | アダプタの不具合でない失敗 | 長い出力を求めるプロンプトと、前提が崩れたことを示す失敗メッセージ（I-04）。 |
| `TestMakeTestIntegrationDeepSeek` が `make` のない環境で失敗する | ローカルでテストが通らない | CI とローカル開発には `make` がある（CLAUDE.md の Commands はすべて `make` 経由）。スキップしないことで、検証が黙って抜けることを防ぐ。 |

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9）
- [ ] PR-1a マージ済み、またはステップ 1-6 の測定が 256 MiB 以下のため `[-]`（対象ステップ: 1-10 / 1-11）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6）
- [ ] PR-4 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 / 5-8 / 6-3 / 6-4）
- [ ] PR-5 マージ済み（対象ステップ: 6-1 / 6-2 / 6-5 / 6-6 / 6-7 / 6-8）
- [ ] 各 PR で `make fmt` → `make test` → `make lint` が通る
- [ ] PR-4 で `make test-integration-deepseek` が通り、§5.1 に実行の記録が残っている
- [ ] §5 のすべての AC の検証が通る
- [ ] `implementation_handoff.md` の I-01〜I-04 が §1.5 のとおり反映されている

## 8. 成功基準 (Success Criteria)

- **機能:** AC-01〜AC-34 のすべてが §5 の検証で確認されている。
- **品質:** `make test`・`make lint` が通る。§4.1 の網羅率の目標を満たす。各テストは対象を壊して失敗することを確認済みで、そのことがコミットメッセージに記録されている。
- **セキュリティ:** API キーの非漏洩（AC-03・AC-20・AC-21）、`Reveal()` の呼び出し箇所の固定、リダイレクトに従わないこと（AC-08）、応答本文の上限（AC-31）、`200` 以外の応答本文を読まないこと（AC-10）、ユニットテストが外部ホストへ届かないこと（AC-25）を確認済みである。
- **互換性:** `internal/transcript` の既存テストが無変更で通る。依存モジュールを追加していない（`.golangci.yml` の depguard を変更していない）。
- **ドキュメント:** `package_reference.md`・`project_overview.md`・`CLAUDE.md`・`security.md`・`README.md` が実装と一致し、追記の根拠を照合済みである。

## 9. 次のステップ (Next Steps)

- 本計画は `approved`。PR の区切りは §2 の `PR-N 作成ポイント` と §3.2 に埋め込み済みである。
- `/runplan 0003` で PR の順（PR-1 → PR-1a（条件付き。実施しない場合はステップ 1-6 で `[-]` にしてある）→ PR-2〜PR-5）に実装する（各 PR は独立してグリーンゲートを通す。§3.2 の不変条件）。
- 実装の完了後、architecture §9 の申し送りを #5（`ArticleWriter`。`MaxOutputTokens` が推論過程を含むこと、`ModelVersion` の `writer.Article` への引き継ぎ）と #6（`deepseek.New` への `Options` の受け渡し、タイムアウトの既定値）の作業で参照する。
