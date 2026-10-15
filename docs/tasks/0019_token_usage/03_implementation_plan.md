# 実装計画書：LLM のトークン数の段階ごとの記録と出力

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-15 |
| Review date | 2026-10-15 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下「requirements」）の項番、H-NN は [design_handoff.md](design_handoff.md) の項目、節番号は [02_architecture.md](02_architecture.md)（以下「architecture」）の節を指す。既存のファイルの行番号は、断りがない限り HEAD `6a92cb4` で確かめた。architecture の行番号はコミット `593c696` のものだが、`593c696..6a92cb4` の Go のファイルの差分はテストの補助（`internal/config/test_helpers.go`・`internal/server/test_helpers.go`・`cmd/yt2column-server/docs_test.go`）だけで、本計画が変える Go のソースは同じである（`git diff --stat 593c696 6a92cb4 -- '*.go'` で確認）。Go のソースのコメント・識別子・文字列リテラルは英語で書き（`_context.md`）、`AC-NN`・`F-NNN`・`H-NN` は Go のソースに書かず本計画にだけ記録する（`requirements_process.md` §4）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

requirements の機能要件（F-001〜F-008）を、architecture の設計どおりに実装する。LLM の応答が報告するトークン数を、プロバイダに依存しない型で段階（生成、字数の調整、検証・推敲）ごとに記録し、CLI の標準エラーの要約（成功と失敗の両方）と中間生成物の来歴に出力する。

用語（トークン数・使用量・段階の使用量・実行の使用量・不明・再利用した値・段階・パイプラインの段階）は architecture の冒頭の用語の表に従う。実装は architecture §8 の実装優先順位どおり 5 つのフェーズに分ける。

### 1.2. 実装原則

- architecture §1.1 の設計原則 9 項目に従う。`internal/job`・`internal/server`・`cmd/yt2column-server`・`internal/publisher` は変更しない（architecture §3.10）。
- 使用量の由来と状態は型（`writer.UsageOrigin`・`llm.Usage`）で宣言し、CLI は値の内容から推測しない（CLAUDE.md「Declare, don't infer」）。
- 数値の検証は `strictjson.Value.AsNonNegativeInt64` の 1 か所、不明の理由の文字列は `llm.Usage.UnknownReason` の 1 か所、トークン数の行の書式は `cmd/yt2column` の 1 か所に置く（architecture §1.1 の 5・2・8）。
- 各フェーズの完了時に、`_context.md` の "Build checks"（`make fmt` → `make test` → `make lint`）を通す。グリーンゲートは `make test && make lint`。`make test` は `-tags test` で走る（`Makefile:71-72`）ので、新しいテストのファイルには既存と同じく `//go:build test` を付ける。
- 外部ライブラリは追加しない（requirements §5）。
- 各テストは、対象の仕組みを壊して失敗することを実装時に確かめ、そのことをコミットメッセージに書く（CLAUDE.md「Every test must be able to fail for its stated reason」）。本計画の「壊して確かめる」タスクは、この義務を具体化したものである。

### 1.3. 既存コード調査結果

**`internal/llm/llm.go`（フェーズ 1・2 で変える）。** `GenerateResponse`（`:57-61`）は `Text`・`Model`・`ModelVersion` だけを持つ。`LLMClient` の文書（`:63-68`）は、エラーのときに何を返すかを定めていない。フィールド名を付けない形の `llm.GenerateResponse{...}` のリテラルはリポジトリにない（`rg -n 'GenerateResponse\{[^}]' -g '*.go'` の 1 行の該当と、`rg -n -U -A1 'GenerateResponse\{\n' -g '*.go'` の複数行の該当を 1 件ずつ見て、すべてフィールド名付きであることを確かめた）ので、フィールドを加えても既存のリテラルはコンパイルできる。

**`internal/strictjson/strictjson.go`（フェーズ 1 で変える）。** `Collect`（`:85-97`）が返すエラーは、消費する要素の重複（`errDuplicateMember`）だけである。アダプタは `Collect` のエラーを `RuleDuplicate` に対応させるので、この前提をステップ 1-4 のテストで固定する。`AsInt64`（`:207-224`）は負数と `-0` を受理し、`strconv` のエラーをそのまま返す（architecture §1.3）。`null` を判定する関数はない。

**DeepSeek のアダプタ（フェーズ 2 で変える）。** `parseResponse`（`internal/llm/deepseek/response.go:42-99`）は、`Collect(keyModel, keyChoices, keySystemFingerprint)`（`:52`）、`parseChoice`（`:79`）、終了理由の `switch`（`:84-90`）、空の本文の判定（`:91-93`）の順に読み、どの拒否でもゼロ値を返す。`usage` は、`parseChoice` の後で `switch` の前に読む（architecture §3.4「`usage` を読む時点」）。`Generate`（`internal/llm/deepseek/deepseek.go:88-94`）は、エラーのときに応答を捨ててゼロ値を返す。`parseResponse` の文書の「Any rejection returns the zero GenerateResponse.」（`response.go:41`）はフェーズ 2 で事実でなくなる。

**Claude のアダプタ（フェーズ 2 で変える）。** `parseResponse`（`internal/llm/claude/response.go:68-111`）は、`Collect(keyModel, keyStopReason, keyContent)`（`:78`）、`parseContent`（`:90`）、終了理由の `switch`（`:95-101`）、空のテキストの判定（`:102-105`）の順である。`usage` は、`parseContent` の後で `switch` の前に読む。`Generate`（`internal/llm/claude/claude.go:93-99`）は DeepSeek と同じ形である。`parseResponse` の文書（`response.go:67`）にも同じ文がある。`TestGenerateDuplicateMembers` の重複を許す行（`response_test.go:333` 付近）と `TestGenerateUnconsumedMembers`（`:413-427`）のコメントは `usage` を消費しない要素として扱っており、フェーズ 2 の後もテストは通るが、コメントは事実でなくなる。

**アダプタのテスト（フェーズ 2 で変える）。** `assertRejected`（`internal/llm/deepseek/test_helpers.go:207-215`、`internal/llm/claude/test_helpers.go:227-236`）は、すべての失敗でゼロ値の応答を求める。これを呼ぶ `assertBodyRejected`（DeepSeek の `response_test.go:33-42`、Claude の `response_test.go:34-43`）は、`ErrInvalidResponse` の拒否のテストでも使われる。HEAD で、`assertRejected`・`assertBodyRejected` を 3 つのエラー（`ErrTruncated`・`ErrUnexpectedFinishReason`・`ErrEmptyResponse`）で呼ぶ箇所を関数ごとに数えた結果は、architecture §1.3 の例外 1 の一覧（コミット `6a92cb4` で Claude の `TestGenerateResponseFixtures` の行を加えた）と一致する。DeepSeek は `TestGenerateResponseFixtures`（`response_test.go:66`）・`TestGenerateFinishReason`（`:118`・`:124`・`:139`・`:151`）・`TestGenerateValidationOrder`（`:170`）・`TestGenerateEmptyContent`（`:186`・`:193`）・`TestGenerateSentinelsDistinct`（`deepseek_test.go:769`）、Claude は `TestGenerateResponseFixtures`（`response_test.go:66`）・`TestGenerateStopReason`（`:96`・`:105`・`:117`・`:129`）・`TestGenerateValidationOrder`（`:153`）・`TestGenerateEmptyText`（`:178`・`:192`）・`TestGenerateSentinelsDistinct`（`claude_test.go:858`）である。ほかの `assertRejected` の呼び出し（HTTP の状態・タイムアウト・取り消し・リクエストの不正・リダイレクト・本番の宛先）はゼロ値の確認のまま変えない。記録済みの応答（`testdata/deepseek_chat_completion_stop.json`・`_length.json`・`claude_messages_end_turn.json`・`_max_tokens.json`）は、どれも `usage` を持ち、消費しない要素も含む（architecture §3.4。HEAD で値を確かめた）。重複した要素を持つ応答は、既存の `TestGenerateDuplicateMembers`（両アダプタの `response_test.go`）と同じく、`replaceOnce` でバイト列として組み立てる（`map[string]any` を経由する `encodeDocument` では重複を表せない）。

**`internal/writer`（フェーズ 3 で変える）。** `Article`（`writer.go:17-25`）、`Write`（`:273-341`）、`generateStage`（`:345-377`）、`shortenStage`（`:382-407`）、`reviewStage`（`:410-435`）、`runShorten`（`shorten.go:14-38`）、`runReview`（`review.go:248-275`）、`newArticle`（`output.go:181-191`）、`checkModelString`（`output.go:147-158`）、`StageRecord`（`artifact.go:56-67`）、`StepError`（`errors.go:113-122`）を変える・使う。`Write` の再開の分岐（`:284-333`）は、再利用した段階の `StageRecord` を `chain` から読むので、`OriginReused` の記録はこの分岐に置ける。`Write` の失敗の戻り値を `Article{}` と比べるテスト（`writer_test.go`・`steps_test.go`・`artifact_test.go`・`prompt_test.go`）は、`Write` がゼロ値を返し続けるので変わらない。`errors.AsType[*StepError]`（`steps_test.go:27`）などの判定は、外側の `*TokenUsageError` を通り抜ける。成功した `Write` の `Article` 全体を値で比べるのは `TestWriteArticle`（`output_test.go:75-82`）だけである（`rg -n 'Article\{' -g '*_test.go' internal/writer` で確認。ほかの `Article{...}` のリテラルは fake の `ArticleWriter` や `Publisher` の入力で、`Write` を通らない）。fake の `ArtifactStore`（`test_helpers.go:276-332`）と `FakeLLMSequence`（`internal/llm/testutil/mocks.go`）は、そのまま段階ごとに異なる使用量を返すテストに使える。

**`internal/artifact`（フェーズ 3 で変える）。** `artifactVersion`（`artifact.go:20`）、`artifactKeys`（`format.go:34-37`）、`recordJSON`（`format.go:45-57`）、`Marshal`（`:82-117`）、`Parse`（`:122-186`）、`parseStage`（`:190-211`）、`recordID`（`:357-381`）、`loadRecord`（`store.go:212-260`）を変える。`Parse` は `CollectOnly` の後に `parseStage` で版を確かめるので、`usage` の読み取りを `parseStage` の後に置けば、版 1 のファイルは `usage` がないことではなく版の違いで拒否される（architecture §3.7）。テストは `artifactVersion` の定数で版を書く（`format_test.go:107`）が、`TestParseRecordRejects` の「unsupported version」の行（`format_test.go:223-226`）は版を `2` と書いており、版を 2 に上げると受理される値になる。往復のテストが使う比較の補助 `sameRecord`（`format_test.go:66-82`。`TestMarshalParseRoundTrip` の `:189` と `store_test.go::TestSaveLoadRoundTrip` の `:153`・`:166`・`:179` が使う）は `StageRecord` のフィールドを 1 つずつ比べ、新しい `Usage` を比べない。ゼロ値の `Usage`（`not-read`）は `null` として保存され、`recorded-unknown` として読み戻るので、単純な `!=` を加えると既存の行が失敗する。CLI のテストは `artifact.Marshal` を通して中間生成物を作る（`cmd/yt2column/run_test.go` の `saveArtifactsOnce`）ので、版の変更で書き直す必要はない。

**`internal/pipeline`（フェーズ 1・3・4 で変える）。** `Run`（`pipeline.go:85-116`）は、公開の失敗を `&StageError{Stage: StagePublish}`（`:112-114`）で、`Write` の後の中断を `ctx.Err()` のまま（`:108-110`）返す。`TestCommonTypesFieldSets`（`pipeline_test.go:378-427`）は `GenerateResponse`（`:401-405`）と `Article`（`:406-414`）のフィールドを固定し、`TestInterfaceDocComments`（`:455-492`）は契約の文言を固定する。`TestPipelineCanceled` の「canceled after write」（`:256-279`）は `err != context.Canceled`（`:267`）で同一性を比べる（architecture §1.3、コミット `15a5b84` で一覧に加えた）。`TestPipelineStageError` の「publish」の行（`:166-168`）は `err.Error() == "publish: boom"` を求めるが、`TokenUsageError.Error()` は中のエラーのメッセージをそのまま返すので、変えずに通る。`Run` の文書（`pipeline.go:85-87`）の「the context error if the context is canceled between stages」は、`Write` の後の中断では包んだエラーになる。`internal/server/job.go:112` のコメント（「pipeline.Run returns the context error between stages」）は、包んだエラーも `StageError` ではなく `classify` の結果も変わらないので、内容として正しいまま残る。`TestWriterImports`（`:771-809`）は `internal/writer` が `internal/llm/` の下のパッケージを import しないことを求める。`internal/llm` 自体（末尾の `/` がない）は対象外で、`writer` はすでに import している。

**`cmd/yt2column/run.go`（フェーズ 4 で変える）。** `stderrWriter.line`（`:210-212`）がすべての行を `redact.Line` に通す。`job.Run` の後の分岐（`:307-333`）は、`*job.ReviewLogError` で `reportSuccess` を呼んでから `reportRunError` を呼ぶ。`reportSuccess`（`:597-620`）の最後の行は中間生成物の保存の行である。`reportRunError`（`:695-723`）は、`*job.ReviewLogError` の分岐（`:696-699`）で戻ってから失敗の行を書くので、`reportTokens` はこの分岐の後で失敗の行の前に置く（architecture §3.8）。既存の CLI のテストは標準エラーを部分文字列で照合し、行の形の検査（`requireSafeOutput`、`run_test.go:311-330`）は各行が `yt2column: ` で始まることだけを求める。HEAD の `cmd/yt2column` と `internal/server` のテストにトークン数の行の接頭辞（`tokens:`）を照合するものはない（`rg -n 'tokens:|"tokens' cmd/yt2column/*_test.go internal/server/*_test.go` で該当なし）。

**CLI の統合テスト（変更しない）。** `cmd/yt2column/integration_test.go` は実際の DeepSeek の API を呼ぶ（`make test-integration-cli`、`Makefile:150-155`）。ループバックの `httptest` のサーバに実際のアダプタを向ける経路は、`deepseek.NewForLoopbackTest`（`internal/llm/deepseek/test_helpers_endpoint.go:17`、`//go:build test`）で、`internal/llm/provider/provider_test.go:184` が前例である。`cmd/yt2column` には、まだこれを使うテストがない。この事実に合わせて architecture §7.2 を直した（コミット `15a5b84`。§2.0）。

**サーバ（変更しない）。** `classify`（`internal/server/job.go:109-150`）は `errors.AsType[*pipeline.StageError]` と `errors.Is` だけで判定するので、`*TokenUsageError` で包んでも分類は変わらない。HEAD の Go のファイルで、`context.Canceled`・`context.DeadlineExceeded` をエラーの `==`・`!=` で比べるのは、テストの `pipeline_test.go:213`・`:239`・`:267` だけである（`rg -n '(!=|==) context\.(Canceled|DeadlineExceeded)' -g '*.go'` で確認。`:213`・`:239` は `Write` の前の中断で、包まないので変わらない）。

**文書（フェーズ 5 で変える）。** `README.md` には、トークン数の行の説明がない（`## Post-generation steps` は `:72-121`）。`docs/dev/project_overview.md` の `## パイプライン`（`:18`）の `LLMClient` の項は、`GenerateResponse` を `Text`・`Model`・`ModelVersion` として説明する（`:42`）。`docs/dev/security.md` の `## 6. 信頼できないテキストの取り扱い`（`:68`）に、LLM の応答の検証と `--artifacts` の項目（`:77`。中間生成物に入る来歴を列挙し、トークン数を含まない）がある。`docs/dev/developer_guide/package_reference.md` の `internal/llm`（`:26`）・`internal/strictjson`（`:19`）・`internal/artifact`（`:33`。依存先を `internal/writer`・`internal/strictjson`・`internal/transcript` と書く）・`internal/writer`（`:34`）・`internal/llm/deepseek`（`:29`）・`internal/llm/claude`（`:28`）の行を更新する。翻訳（`*.en.md`）はない（`find docs -name '*.en.md'` で該当なし）。

**外部の前提の確認。** モジュールは `go 1.26.5`、ツールチェーンは `go1.27.1`（`go.mod`・`go version`）。`errors.AsType` は既存コードが使う（`cmd/yt2column/run.go:324`）。`http.ProxyFromEnvironment` は `localhost` とループバックの IP アドレスの宛先にプロキシを使わない（`go1.27.1` の `$(go env GOROOT)/src/vendor/golang.org/x/net/http/httpproxy/proxy.go:178-187` の `useProxy`。`go doc net/http.ProxyFromEnvironment` は `localhost` だけを述べる）。そのため、`cmd/yt2column` の `TestMain`（`main_test.go:22`）がプロキシの環境変数を閉じたアドレスに向けていても、`127.0.0.1` の `httptest` のサーバには届く。

**具体型での分岐。** 本タスクは具体型で振る舞いを分ける設計を加えない。`llm.Usage` はフィールドを非公開にし、`KnownUsage`・`UnknownUsage`・ゼロ値の 3 つでだけ作れる（architecture §3.1）。`writer.StageUsage`・`RunUsage` は公開のフィールドの単純な値で、組み合わせの正しさは不変条件のテスト（ステップ 3-5）で確かめる（architecture §3.5「`StageUsage` を公開のフィールドの構造体にする理由」）。

### 1.4. 新しい名前の一覧

本タスクで加える名前である。既存の名前の変更・削除はない。以降の本文と AC の表はこの表の名前を使う。

| 置き場所 | 名前 | 出典 |
|---|---|---|
| `internal/llm`（`usage.go`） | `TokenCounts`（`Input`・`CacheRead`・`CacheWrite`・`Output`）・`UnknownRule`（`RuleNotRead`・`RuleMissing`・`RuleDuplicate`・`RuleNotObject`・`RuleNotNonNegativeInteger`・`RuleRecordedUnknown`）・`Usage`・`KnownUsage`・`UnknownUsage`・`Usage.Counts`・`Usage.UnknownReason`、`GenerateResponse.Usage` | architecture §3.1・§3.2 |
| `internal/strictjson` | `Value.IsNull`・`Value.AsNonNegativeInt64` と、その拒否の非公開の静的なエラー | architecture §3.3 |
| `internal/writer` | `UsageOrigin`（`OriginNone`・`OriginRun`・`OriginReused`）・`StageUsage`・`RunUsage`・`Article.RunUsage`・`StageRecord.Usage`・`TokenUsageError`（`Error`・`Unwrap`） | architecture §3.5・§3.6 |
| `internal/artifact` | JSON のキー `usage` と、その中の `input`・`cache_read`・`cache_write`・`output` | architecture §3.7 |
| `cmd/yt2column` | `reportTokens` と、1 行を作る関数・書式の定数（接頭辞・3 つの語・区分の名前・`model=`・`nomodel`） | architecture §3.8 |

### 1.5. 申し送りへの対応

`design_handoff.md` の H-01〜H-06 は architecture §3.9 が対応を記録しており、本計画はそのとおりに実装する。`implementation_handoff.md` は存在しない。

| 項目 | 本計画での扱い |
|---|---|
| H-01 アダプタのテストの応答の形 | ステップ 2-4・2-5（記録済みの応答と、`usage` だけを置き換える補助） |
| H-02 失敗した呼び出しのトークン数の受け渡し | ステップ 2-1・2-2（3 つのエラーで `Model`・`Usage` を返す）、ステップ 3-2（`TokenUsageError`）、ステップ 4-1（`pipeline.Run` の包み） |
| H-03 `Article` と結果の型の形 | ステップ 3-1・3-2・3-5、ステップ 1-5 と 3-6（`TestCommonTypesFieldSets`） |
| H-04 要約の行の書式 | ステップ 4-2・4-3、ステップ 5-1・5-2 |
| H-05 中間生成物の形式の変更 | ステップ 3-3・3-4・3-7 |
| H-06 数値の検証の方法 | ステップ 1-3・1-4 |

### 1.6. 対象外（requirements §2.3）

料金の計算と単価、トークン数の合計、`--out` と Webhook の投稿へのトークン数の記載、localhost サーバのログと応答、トークン数による実行の制限、推論の出力の内訳、Claude のキャッシュの書き込みの TTL ごとの内訳、機械可読な出力のための新しいフラグ、トークン数を得られない失敗での推定は、本タスクの対象外である。字幕の `decodeTimestamp` を `AsNonNegativeInt64` に移すことは [#259](https://github.com/isseis/yt2column/issues/259) で扱う（architecture §3.3）。

### 1.7. 横断検索チェックリスト

本タスクは既存の名前を変更・削除しない追加であり、新しい名前は §1.4 のパッケージの中に閉じる。変える契約（アダプタがエラーのときに返す値、`Write`・`pipeline.Run` が返すエラーの形、中間生成物の版）を前提とする既存のテストは §1.3 で関数ごとに数え、各ステップのタスクにした。変える契約を述べるソースのコメントと文書（§1.3 で挙げた `response.go` の文書、Claude の `response_test.go` のコメント、`pipeline.Run` の文書、`project_overview.md:42`、`security.md:77`、`package_reference.md` の各行）は、ステップ 2-1・2-2・2-5・4-1・5-3〜5-5 のタスクにした。これらのほかに、`make lint`・`make test` で検出できない残存参照は想定されないので、横断検索チェックリストは設けない。

## 2. 実装ステップ (Implementation Steps)

### 2.0. 前提（解消済み）

計画の作成時に、architecture が既存のコードと食い違う箇所を 3 つ見つけ、architecture を editorial correction として直した（status は `approved` のまま。`Comments` に記録。コミット `15a5b84`・`6a92cb4`）。設計の決定は変えていない。

- §1.3「要件 AC-27 との関係」と §3.10 に、`TestPipelineCanceled` の「canceled after write」を加えた（§1.3 の `internal/pipeline`）。
- §7.2 の最初の項目を、実際の API を呼ぶ既存の統合テストではなく、`make test` で走る新しいループバックのテスト（ステップ 4-4）に改めた。
- §1.3 の例外 1 と §3.10 の Claude の一覧に、`TestGenerateResponseFixtures` の打ち切りの行を加えた。

### フェーズ 1: 型と数値の検証

**対象ファイル:** `internal/llm/usage.go`（新規）、`internal/llm/usage_test.go`（新規）、`internal/llm/llm.go`、`internal/strictjson/strictjson.go`、`internal/strictjson/strictjson_test.go`、`internal/pipeline/pipeline_test.go`

- [ ] **ステップ 1-1**: `internal/llm/usage.go` に、architecture §3.1 の `TokenCounts`・`UnknownRule`・`Usage`・`KnownUsage`・`UnknownUsage`・`Counts`・`UnknownReason` を置く。理由の語は architecture §3.1 の `UnknownReason` の文書と §3.4 の表の例のとおりとし、その綴りは `UnknownReason` の中だけに書く。
- [ ] **ステップ 1-2**: `internal/llm/llm.go` の `GenerateResponse` に `Usage Usage` を加え、文書の 1 行目を architecture §3.2 の形にする。`LLMClient` の文書の変更はフェーズ 2（ステップ 2-3）で行う。
- [ ] **ステップ 1-3**: `internal/strictjson/strictjson.go` に `Value.IsNull` と `Value.AsNonNegativeInt64` を architecture §3.3 のとおりに加える。
- [ ] **ステップ 1-4**: テストを書く。
  - `internal/strictjson/strictjson_test.go` の `TestAsNonNegativeInt64`（architecture §7.1 の受理と拒否の値。拒否のエラーのメッセージが入力の値の文字列を含まないこと）と `TestIsNull`。
  - `internal/llm/usage_test.go` の `TestUsageZeroValue`（ゼロ値が不明で `not-read`、`KnownUsage(TokenCounts{})` が 0 の分かった値でゼロ値と `!=`）と `TestUnknownReason`（規則と要素名の組み合わせごとの語、空白を含まないこと、列挙の外の規則の `invalid-rule`）。
  - AC-03 の guard の `TestNoProviderMemberNames`。`internal/llm` のディレクトリの直下の、`_test.go` でない `.go` ファイル（サブディレクトリのアダプタを含めない）が、architecture §7.1 の 7 つの要素名を含まないことを確かめる。
  - `strictjson_test.go` の `TestCollectErrorIsDuplicate`: `Collect` が返すエラーが消費する要素の重複（`errDuplicateMember`）だけであることを固定する（アダプタが `Collect` のエラーを `RuleDuplicate` に対応させる前提。§1.3）。
  - `AsNonNegativeInt64` を `AsInt64` に差し戻すと `-1`・`-0` の行が失敗することを確かめる。`TestNoProviderMemberNames` は、`usage.go` に要素名の 1 つを一時的に書くと失敗することを確かめる。
- [ ] **ステップ 1-5**: `internal/pipeline/pipeline_test.go` の `TestCommonTypesFieldSets` の `GenerateResponse` の行に `"Usage": "llm.Usage"` を加える（architecture §3.10）。

**完了条件:** グリーンゲートが通る。アダプタはまだ `Usage` を設定しないので、既存のアダプタのテストは変えずに通る（応答の `Usage` はゼロ値で `!=` の比較に影響しない）。

### フェーズ 2: アダプタ

**対象ファイル:** `internal/llm/deepseek/response.go`・`deepseek.go`・`test_helpers.go`・`response_test.go`・`deepseek_test.go`、`internal/llm/claude/response.go`・`claude.go`・`test_helpers.go`・`response_test.go`・`claude_test.go`、`internal/llm/llm.go`、`internal/pipeline/pipeline_test.go`

- [ ] **ステップ 2-1**: DeepSeek の `parseResponse` に、architecture §3.4 の `usage` の読み取り（読む時点、4 つの区分への対応、不明の規則と要素名）を加える。要素名はこのパッケージの定数とする。3 つのエラーで返す応答と `Generate` の扱いは architecture §3.2 のとおりとする。`parseResponse` の文書の「Any rejection returns the zero GenerateResponse.」を、3 つのエラーで `Model` と `Usage` を返すことを述べる文に改める。
- [ ] **ステップ 2-2**: Claude の `parseResponse` に、同じく architecture §3.4 の読み取り（キャッシュの 2 つの要素の `null` を 0 とする扱いを含む。requirements AC-02a）を加える。3 つのエラーと `Generate`、`parseResponse` の文書の扱いは DeepSeek と同じである。
- [ ] **ステップ 2-3**: `internal/llm/llm.go` の `LLMClient` の文書に、architecture §3.2 の 2 文（3 つのエラーで `Model` と `Usage` を返し、`Text` と `ModelVersion` は空。ほかのエラーではゼロ値）を加える。`TestInterfaceDocComments` の `LLMClient` の行に、その 2 文を固定する句を加える。
- [ ] **ステップ 2-4**: 各アダプタの `test_helpers.go`（`//go:build test`。`test_organization.md` の分類 B）に、記録済みの応答の `usage` だけを置き換えた本文を組み立てる補助を加える（区分ごとに異なる値、要素の削除・値の置き換え・`null`、最上位と中の重複）。重複は `replaceOnce` でバイト列として作る。`assertRejected` はゼロ値の確認のまま残し、3 つのエラーには、`Model` と `Usage` が期待どおりであることを確かめる別の補助を加える。architecture §1.3 の例外 1 は「`assertRejected` を改める」と書くが、どちらの期待を確かめるかを呼び出し側が補助の選択で宣言すれば、エラーの番兵から期待を推測せずに済む（CLAUDE.md「Declare, don't infer」）。そこで、補助を分けることを実装の選択とする。`assertBodyRejected` にも、3 つのエラー用の版を加える。新しい補助は、既存の補助（`readFixture`・`documentOf`・`encodeDocument`・`replaceOnce`・`newResponseServer`・`generateAgainst`）の上に書く。
- [ ] **ステップ 2-5**: 3 つのエラーを確かめる既存のテストを、ステップ 2-4 の補助に切り替える。対象は §1.3 で数えた DeepSeek の `TestGenerateResponseFixtures`（打ち切りの行）・`TestGenerateFinishReason`・`TestGenerateValidationOrder`・`TestGenerateEmptyContent`、Claude の `TestGenerateResponseFixtures`（打ち切りの行）・`TestGenerateStopReason`・`TestGenerateValidationOrder`・`TestGenerateEmptyText` である。テキスト・終了理由・番兵の確認は変えない。Claude の `TestGenerateDuplicateMembers`・`TestGenerateUnconsumedMembers` のコメントのうち `usage` を消費しない要素と述べる部分を、`usage` が不明になる経路を通ることを述べる形に改める（行の期待は変えない）。両アダプタの `TestGenerateSentinelsDistinct` は、3 つのエラーの行で `Model` と `Usage` を、ほかのすべての行（リクエストの不正・HTTP の状態・`ErrInvalidResponse`・通信の失敗・期限切れ・取り消し）でゼロ値を確かめるように広げる（architecture §3.2）。
- [ ] **ステップ 2-6**: 新しいテストを、各アダプタの `response_test.go` に書く。どれも公開の `Generate` を `httptest` のサーバに向けて呼ぶ（requirements AC-25）。
  - `TestGenerateUsageFixtures`: 記録済みの応答（正常終了）の受理と、使用量が応答の値であること。
  - `TestGenerateUsageCategories`: 区分ごとに異なる値での対応（AC-01・AC-02）。
  - `TestGenerateUsageNullCache`（Claude だけ）: 2 つの要素のそれぞれについて、その要素だけが `null` の応答（AC-02a）。
  - `TestGenerateUsageUnknown`: architecture §3.4 の不明の表の各行と、AC-05 の不正な形（要素ごと）の表。各行は、応答が受理され `Text`・`Model`・`ModelVersion` が置き換え前と同じであること、`Counts` の第 2 戻り値が偽であること、`UnknownReason` が期待する語と等しく、注入した値（巨大な数の文字列など）を含まないことを確かめる（AC-04〜AC-06・AC-08）。
  - `TestGenerateUsageIgnoresUnconsumed`: 消費しない要素に、値の種類の違う値と重複を入れても、使用量が分かった値のままであること（AC-07）。
  - `TestGenerateUsageKeepsVerdict`: `ErrInvalidResponse`・3 つのエラー・受理の各応答で、`usage` が正しい場合と不明になる場合のエラーが同じであること（AC-09）。
  - `TestGenerateRejectedResponseUsage`: ステップ 2-5 で更新した行（記録済みの打ち切りの応答）が確かめない入力だけを対象にする。区分ごとに異なる値を持つ想定外の終了理由と空の応答で、`Model` と使用量が返ること、`usage` が不正な打ち切りの応答では不明が返ること（AC-17）。
- [ ] **ステップ 2-7**: 壊して確かめる。`usage` を既存の最上位の `Collect` に加えると、`TestGenerateUsageUnknown` の最上位の重複の行（応答が受理されることを確かめる）が失敗することを確かめる。`Generate` がエラーのときにゼロ値を返す形に戻すと、`TestGenerateRejectedResponseUsage` が失敗することを確かめる。

**完了条件:** グリーンゲートが通る。`internal/llm` を除く他のパッケージは、アダプタの `Usage` をまだ読まない。

### フェーズ 3: `writer` と中間生成物

**対象ファイル:** `internal/writer/writer.go`・`artifact.go`・`errors.go`・`shorten.go`・`review.go`・`output.go`・`output_test.go`、`internal/writer/usage_test.go`（新規）、`internal/artifact/artifact.go`・`format.go`・`store.go`・`format_test.go`・`chain_test.go`、`internal/pipeline/pipeline_test.go`

- [ ] **ステップ 3-1**: architecture §3.5・§3.6 の型を置く。`UsageOrigin`・`StageUsage`・`RunUsage` と `Article.RunUsage` は `writer.go`、`StageRecord.Usage` は `artifact.go`、`TokenUsageError` は `errors.go` に置く。`ArticleWriter` の文書に「失敗では `*TokenUsageError` で包んだエラーを返す」を加える。
- [ ] **ステップ 3-2**: `Write` に使用量の記録を加える。各段階の関数が返す `StageUsage`、`StageUsage.Model` に入れる値、`StageRecord.Usage`、再利用した段階の `OriginReused` は architecture §3.5（「段階の関数の戻り値」「`StageUsage.Model` に入れる値」）、失敗での `*TokenUsageError` の包み方は §3.6 と図6 のとおりとする。成功では `newArticle` に `RunUsage` を渡す。
- [ ] **ステップ 3-3**: `internal/artifact` を、architecture §3.7 の形式の版 2（`usage` 要素の形、読み込みの規則、読み込んだ不明の値、`recordID` に加える部分）にする。`Parse` は `usage` を `parseStage` の後で読む（版 1 のファイルを版の違いで拒否するため。ステップ 3-8 で固定する）。拒否は `malformed`・`malformedf` で、ファイルの値を含めずに返す。
- [ ] **ステップ 3-4**: `internal/artifact` のメッセージを 2 か所変える。
  - `parseStage` の版の拒否（`format.go:200`）を、`"unsupported artifact version %d"` から `"unsupported artifact version %d; this CLI reads version %d only; move away the artifacts an older CLI saved, or use a new --artifacts directory, then rerun"`（2 つ目の `%d` は `artifactVersion`）に変える。版 1 では architecture §3.7 の文になる。
  - `loadRecord` が `Parse` から受け取ったエラー（`store.go:252-254`）を `fmt.Errorf("%s artifact: %w", stage, err)` で包む。大きさの上限の 2 つのメッセージ（`store.go:243`・`:250`）を、`"artifact exceeds the limit of %d bytes"` から `"artifact %s exceeds the limit of %d bytes"`（`%s` は段階の名前）に変える。番兵（`ErrMalformed`・`writer.ErrMalformedOutput`）の包みは変えない。
- [ ] **ステップ 3-5**: `internal/writer/usage_test.go` に、fake の `LLMClient`（`FakeLLMSequence`・`FakeLLMClient`）と fake の `ArtifactStore` で architecture §7.1 の `internal/writer` のテストを書く。
  - `TestWriteRunUsagePerStage`: 3 つの段階に異なる使用量を返し、各段階の値とモデル名が対応する呼び出しのものであること（AC-10）。1 つの段階が `KnownUsage(TokenCounts{})` を返す行で、その段階が `OriginRun` で `Counts` の第 2 戻り値が真であること（AC-11 の「0 を含む」）。
  - `TestWriteRunUsageUnknownStage`: 1 つの段階だけが不明を返す成功した実行で、記事が返り、ほかの段階が影響を受けないこと（AC-11a）。
  - `TestWriteRunUsageInvariants`: architecture §1.3 の例外 3 の不変条件を、字数の調整と検証・推敲の有効・無効、字数の調整が LLM を呼ぶかどうか、`--after` の 3 つの値と再開しない場合の組み合わせのすべてで確かめる（AC-11・AC-14）。
  - `TestWriteRunUsageResume`: 再開で、再利用した段階が `OriginReused` で中間生成物の使用量とモデル名を持ち、呼んだ段階が `OriginRun` であること（AC-14）。中間生成物の値と、再開の実行の fake が返す値は、トークン数もモデル名も異ならせる。
  - `TestWriteFailureRunUsage`: architecture §7.1 の失敗の経路の表（字幕の検証、`--after` の読み込み、展開の失敗、呼び出しの前の中断、字数の調整の呼び出しの失敗（使用量を返す打ち切り）、検証・推敲の呼び出しの失敗、生成の使用量を返す打ち切り、期限切れ、検証・推敲の応答の形の不正、保存の失敗、最後の字数の検査）で、`errors.AsType[*TokenUsageError]` が取り出せ、各段階の `Origin` と `Usage` が期待どおりであること、`Article` がゼロ値であること（AC-16〜AC-20）。失敗した呼び出しのモデル名が `checkModelString` を通らない行では、`StageUsage.Model` が空であることを含める。
  - `TestWriteSavesUsage`: 保存した各段階の `StageRecord.Usage` が、その段階の呼び出しの使用量であること（AC-21）。
  - `TestTokenUsageErrorTransparent`: `Error()` が中のエラーのメッセージと等しく、`errors.Is`・`errors.AsType` が中の `*StepError`・`*ArtifactError`・番兵に届くこと（AC-28 の前提）。
- [ ] **ステップ 3-6**: 既存のテストを更新する。`TestWriteArticle`（`output_test.go:75-82`）の期待する `Article` に `RunUsage` を加える。`TestCommonTypesFieldSets` の `Article` の行に `"RunUsage": "writer.RunUsage"` を、`TestInterfaceDocComments` の `ArticleWriter` の行に、ステップ 3-1 の文を固定する句を加える。`TestParseRecordRejects` の「unsupported version」の行（`format_test.go:223-226`）の版を、`2` から `artifactVersion+1` に変える。`fieldsFrom`（`format_test.go:105`）に `usage` を加える。`sameRecord`（`format_test.go:66-82`）に使用量の比較を加える。分かった値はトークン数が等しいこと、不明は両方が不明であることを等しいとし、理由は比べない（理由は保存しない。architecture §3.7）。
- [ ] **ステップ 3-7**: `internal/artifact` のテストを書く。
  - `format_test.go` の `TestMarshalParseRoundTrip` に、分かった値と不明の行を加える（AC-21）。
  - `TestParseRecordRejectsUsage`: `usage` がない・重複・オブジェクトでも `null` でもない値（文字列・数・真偽値・配列）、中の宣言していない要素・要素の欠け・重複、値の負数・`-0`・小数・指数・`int64` を超える数・文字列・真偽値・配列・オブジェクト・`null` を、それぞれその入力だけで `ErrMalformed` で拒否し、メッセージにファイルの値を含めないこと（AC-22）。
  - `TestParseRejectsVersion1`: `usage` のない版 1 の中間生成物を `ErrMalformed` で拒否し、メッセージが版の違いと回復の手順を示すこと（AC-23）。
  - `TestRecordIDCoversUsage`: トークン数だけが異なる 2 つの記録の `id` が異なること、`id` を変えずにトークン数だけを書き換えたファイルを `ErrMalformed` で拒否すること、理由だけが異なる不明の 2 つの記録の `id` が同じであること（AC-24、architecture §3.7）。
  - `chain_test.go` の `TestLoadNamesBrokenStage`: 版 2 の生成の中間生成物と検証・推敲の中間生成物、および版 1 の字数の調整の中間生成物で `--after review` の `Load` を呼び、エラーが `ErrMalformed` で、メッセージが字数の調整の段階の名前を含むこと（architecture §7.1）。
- [ ] **ステップ 3-8**: 壊して確かめる。`Origin` を設定する箇所を 1 つずつ消し、そのたびに次のテストが失敗することを確かめる。生成、字数の調整、検証・推敲の `OriginRun`（成功した呼び出し）は `TestWriteRunUsageInvariants` の該当する組み合わせの行、同じ 3 つの `OriginRun`（失敗した呼び出し）は `TestWriteFailureRunUsage` の生成の使用量を返す打ち切りの行、字数の調整の呼び出しの失敗の行、検証・推敲の呼び出しの失敗の行、生成、字数の調整、検証・推敲の `OriginReused` は `TestWriteRunUsageResume` の `--after generation`・`shorten`・`review` の行である。あわせて、`Marshal` から `usage` を除くと `TestMarshalParseRoundTrip` の分かった値の行が失敗すること、`recordID` から使用量の部分を除くと `TestRecordIDCoversUsage` が失敗すること、`Parse` で `usage` を `parseStage` の前に読むと `TestParseRejectsVersion1` が失敗することを確かめる。

**完了条件:** グリーンゲートが通る。CLI はまだ `RunUsage` を出さない。中間生成物は版 2 で保存され、既存の CLI の中間生成物のテスト（`artifact.Marshal` で作る）は変えずに通る。

### フェーズ 4: パイプラインと CLI

**対象ファイル:** `internal/pipeline/pipeline.go`・`pipeline_test.go`、`cmd/yt2column/run.go`・`run_test.go`

- [ ] **ステップ 4-1**: `pipeline.Run` が、`Write` の成功後の公開の失敗を `&StageError{Stage: StagePublish, Err: &writer.TokenUsageError{RunUsage: article.RunUsage, Err: err}}` で、`Write` と公開の間の中断を `&writer.TokenUsageError{RunUsage: article.RunUsage, Err: ctxErr}` で返すようにし、`Run` の文書に「`Write` の成功後の失敗を `*writer.TokenUsageError` で包む」を加え、「the context error if the context is canceled between stages」を、`Write` の後の中断では包んだエラーになることが分かる文に改める（architecture §3.6）。ゼロ値の `Article` を返す規則は変えない。
- [ ] **ステップ 4-2**: `pipeline_test.go` を更新する。`TestPipelineCanceled` の「canceled after write」（`:267`）の `err != context.Canceled` を `errors.Is` での比較に改め、`StageError` で包まないことの確認は残す。そのうえで、`*writer.TokenUsageError` を取り出せ、fake の `ArticleWriter` が返した `Article.RunUsage` を持つことを加える。新しい `TestPipelinePublishFailureCarriesRunUsage` で、公開の失敗のエラーから `*writer.TokenUsageError` が取り出せ、`RunUsage` が `Write` の値で、`StageError.Stage` が `StagePublish` であることを確かめる（AC-19 の公開の失敗）。
- [ ] **ステップ 4-3**: `cmd/yt2column/run.go` に、architecture §3.8 の書式の定数、1 行を作る関数、`RunUsage` から段階の順に行を書く `reportTokens` を置く。`OriginNone` と列挙の外の `UsageOrigin` は行を書かない。`reportSuccess` の最後で `reportTokens(errOut, article.RunUsage)` を呼ぶ。`reportRunError` では、`*job.ReviewLogError` の分岐の後、失敗の行の前に、`errors.AsType[*writer.TokenUsageError]` で取り出せたときだけ `reportTokens` を呼ぶ。行はすべて `errOut.line` で書く。
- [ ] **ステップ 4-4**: `cmd/yt2column/run_test.go` に、fake の `LLMClient`・`TranscriptSource`・`Publisher` と `t.TempDir`（`newRunEnv`・`funcLLM`・`reviewLLM`・`useReviewClient`・`saveArtifactsOnce`・`firstLongThenShort` を再利用する）で CLI のテストを書く。行は `redact.Line` を通った後の標準エラーの行のバイト列として比べる。
  - `TestRunTokenLinesAllStages`: 3 つの段階が走る成功した実行で、段階の順に 3 行が、段階ごとに異なる値とモデル名で出ること、行が既存の要約の行の後にあること（AC-12・AC-15）。
  - `TestRunTokenLinesSkippedStages`: 字数の調整が LLM を呼ばない実行と `--no-review` の実行で、その段階の行が出ないこと（AC-13）。
  - `TestRunTokenLinesResume`: `--after generation`・`shorten`・`review` の各再開で、再利用した段階の行が `reused` で保存時の値とモデル名を、呼んだ段階の行が `run` を示すこと。保存の実行と再開の実行の fake は、トークン数もモデル名も異ならせる。不明のまま保存した段階を再利用した行が `reused unknown reason=recorded-unknown` であること。`--after` の読み込みが成功した後に失敗した実行で、再利用した段階の行が出ること（AC-14・AC-20 の後半・AC-21）。
  - `TestRunTokenLinesUnknown`: 1 つの段階が不明を返す成功した実行で、その行が `unknown reason=` と理由を示して数値を出さず、記事が公開され、終了コードが 0 であること（AC-15a）。
  - `TestRunTokenLinesReviewFailure`: 検証・推敲の呼び出しが失敗した実行で、生成と字数の調整の行が失敗の行の前に出て、`--out` のファイルがなく、終了コードが 0 でないこと（AC-16）。
  - `TestRunTokenLinesRejectedResponse`: 生成の fake が `ErrTruncated`・`ErrUnexpectedFinishReason`・`ErrEmptyResponse` とともに使用量を返す実行で、その値の行が出ること、不明を返す場合は不明の行が出ること（AC-17）。
  - `TestRunTokenLinesNoUsage`: fake が `context.DeadlineExceeded` や `ErrInvalidResponse` をゼロ値の応答とともに返す実行で、その段階の行が `unknown reason=not-read nomodel` であること（AC-18）。
  - `TestRunTokenLinesAfterAllCalls`: すべての呼び出しが成功した後の字数の上限・中間生成物の保存・公開の失敗で、呼んだ段階すべての行が出ること（AC-19）。
  - `TestRunTokenLinesBeforeLLM`: 使い方の誤り・設定の誤り・字幕の取得の失敗・`--after` の読み込みの失敗で、トークン数の行が出ないこと（AC-20）。
  - `TestRunTokenLineModelName`: モデル名が空白・`"`・`\`・U+202E を含む行で、`model=` から行の終わりまでがモデル名であり、README に書く手順（`model=` の後の文字列を `"` で囲み `strconv.Unquote` に渡す）で元のモデル名に戻ること（architecture §3.8）。
  - `TestRunTokenLinesReviewLogFailure`: 検証・推敲の一覧のファイルの書き込みが失敗した実行で、行が 1 回だけ出ること。
  - 既存の `TestRunArtifactExitCodes` に、`usage` が不正な中間生成物で `--after` を実行する行を加え、既存の行と同じく `requireUntouched` で LLM を呼ばず公開しないことを確かめる（AC-22 の CLI の部分）。
- [ ] **ステップ 4-5**: architecture §7.2 のループバックのテスト `TestRunTokenLinesLoopbackAdapter` を `run_test.go` に書く。`deps.newLLMClient` を、`deepseek.NewForLoopbackTest` で `httptest` のサーバに向けたアダプタを返す関数に差し替え、サーバは記録済みの応答（`testdata/deepseek_chat_completion_stop.json`）の `message.content` だけを記事の形の文字列に置き換えた本文を返す。字数の調整と検証・推敲を無効にして実行し、生成の行が記録済みの応答の `usage` の値とモデル名で出ることを確かめる。記録済みの応答は `../../testdata/` の定数のパスで読み、gosec が読み込みを指摘する場合は、その 1 行に限った `//nolint:gosec` を、根拠（コミット済みの testdata の記録）のコメント付きで置く。
- [ ] **ステップ 4-6**: 壊して確かめる。`reportRunError` の `reportTokens` の呼び出しを消すと `TestRunTokenLinesReviewFailure` が失敗すること、`reportTokens` の `OriginNone` の判定を外すと `TestRunTokenLinesSkippedStages` が失敗することを確かめる。

**完了条件:** グリーンゲートが通る。既存の `cmd/yt2column` と `internal/server` のテストが変えずに通る（AC-27・AC-28）。

### フェーズ 5: 文書

**対象ファイル:** `README.md`、`cmd/yt2column/docs_test.go`、`docs/dev/project_overview.md`、`docs/dev/security.md`、`docs/dev/developer_guide/package_reference.md`

- [ ] **ステップ 5-1**: `README.md` に、トークン数の行の節（例: `## Token usage`。`## Post-generation steps` の後）を加える。書く内容は architecture §3.10 の `README.md` の行のとおりで、行の書式（2 つの形と位置ごとの語）、4 つの区分の意味、再利用の印、不明の表し方と理由の語、`nomodel`、モデル名のエスケープの戻し方、行の順序（成功では要約の最後、失敗では失敗の行の前）、行が出ない終わり方（architecture §3.6）、更新後は新しい `--artifacts` のディレクトリを使うこと、である。例の行は、ステップ 4-4 のテストの出力と同じ形にする。
- [ ] **ステップ 5-2**: `cmd/yt2column/docs_test.go` に、既存の `TestArtifactDocsContract` と同じ形の `TestTokenUsageDocsContract` を加える。`readDoc`・`sectionByHeading`・`bulletContaining` を使い、次を確かめる。
  - README の節が、`run.go` の書式の定数（接頭辞・3 つの語・区分の名前・`model=`・`nomodel`）をすべて含むこと。
  - README の節が、`llm.UnknownUsage` で作った各規則の `UnknownReason` の語と `not-read`・`recorded-unknown` を含むこと。語はコードから得て、テストに綴りを書かない。
  - README の例の行が、接頭辞の後の第 1〜3 の語に定数の語だけを持つこと。
  - `project_overview.md` の `## パイプライン` が `Usage` を、`security.md` の §6 が応答の `usage` の項目（新しい項目）と、`--artifacts` の項目（`bulletContaining` で選ぶ）の `usage` の記述の両方を、`package_reference.md` の `internal/llm` の行が `TokenCounts` と `Usage` を含むこと（AC-29）。
  - README から定数の語を 1 つ消すと失敗することを確かめる。
- [ ] **ステップ 5-3**: `docs/dev/project_overview.md` の `## パイプライン` の `ArticleWriter` の項に段階ごとの使用量と CLI の標準エラーの行を、`LLMClient` の項（`:35`）と `GenerateResponse` の説明（`:42`）に `Usage` と 3 つのエラーで返す値を加える。
- [ ] **ステップ 5-4**: `docs/dev/security.md` の `## 6. 信頼できないテキストの取り扱い` に、応答の `usage` を信頼できない入力として扱う規則（受理する形、不明にする場合、応答を拒否しないこと、理由に値を含めないこと）の項目を加える。また、`--artifacts` の項目（`:77`）の来歴の列挙にトークン数を加え、`usage` 要素の拒否の規則を加える（requirements §3.2、architecture §3.4・§3.7）。
- [ ] **ステップ 5-5**: `docs/dev/developer_guide/package_reference.md` の `internal/llm` の行に `TokenCounts`・`UnknownRule`・`Usage` を、`internal/strictjson` の行に `IsNull`・`AsNonNegativeInt64` を、`internal/writer` の行に `RunUsage`・`StageUsage`・`UsageOrigin`・`TokenUsageError` を加える。`internal/llm/deepseek`・`internal/llm/claude` の行に、応答の `usage` を読むことと、3 つのエラーで `Model` と `Usage` を返すことを加える。`internal/artifact` の行に形式の版 2 と `usage` を加え、依存先に `internal/llm` を加える。
- [ ] **ステップ 5-6**: 書いた文書を、正とするもの（コードとテストの出力）と照らす。README の書式・語・順序は `TestTokenUsageDocsContract` とステップ 4-4 のテストの出力で、`security.md`・`project_overview.md` の規則は、それぞれが述べる振る舞いを固定するテスト（ステップ 2-6・3-7 の名前）と、PR の差分で突き合わせる。

**完了条件:** グリーンゲートが通る。`TestTokenUsageDocsContract`・`TestPackageReferenceListsPackages` が通る。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

| マイルストーン | このフェーズで作るもの | フェーズ |
|---|---|---|
| M1: 使用量の型と数値の検証がある | `internal/llm` の使用量の型、`GenerateResponse.Usage`、`strictjson` の `IsNull`・`AsNonNegativeInt64` | フェーズ 1 |
| M2: アダプタが使用量を返す | 両アダプタの `usage` の読み取り、3 つのエラーでの `Model`・`Usage`、`LLMClient` の契約 | フェーズ 2 |
| M3: `Write` が段階ごとの使用量を返す | `RunUsage`・`TokenUsageError`、`Write` の記録、中間生成物の版 2 | フェーズ 3 |
| M4: CLI がトークン数の行を出す | `pipeline.Run` の包み、`reportTokens`、CLI のテスト | フェーズ 4 |
| M5: 文書が揃う | README と契約のテスト、`project_overview.md`・`security.md`・`package_reference.md` | フェーズ 5 |

- フェーズの名前と順序は architecture §8 の手順 1〜5 と同じである。
- 各フェーズは前のフェーズに依存する。フェーズ 2 は M1 の型を、フェーズ 3 はアダプタが返す `Usage` の契約（fake で代用できるが、契約の文書はフェーズ 2 で入る）を、フェーズ 4 は `RunUsage` と `TokenUsageError` を、フェーズ 5 は `run.go` の書式の定数を使う。
- 各フェーズは単独でグリーンゲートを通せる単位とする。PR の境界と実装モデル要件は `/mkplan2` で本書に加える。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

ユニットテストは実際の LLM の API・`yt-dlp`・Webhook を呼ばない（CLAUDE.md「Testing Strategy」）。アダプタのテストは `httptest` のサーバと記録済みの応答を使う（AC-25）。`internal/writer` のテストは `FakeLLMSequence`・`FakeLLMClient` と fake の `ArtifactStore` を、`internal/artifact` のテストは `t.TempDir` の実ファイルを、`internal/pipeline` のテストは fake の `TranscriptSource`・`ArticleWriter`・`Publisher` を、`cmd/yt2column` のテストは fake の `LLMClient`・`TranscriptSource`・`Publisher` と `t.TempDir` を使う（AC-26）。対象は architecture §7.1 のとおりで、AC との対応は §5 の表に記す。目標は、§5 の各 AC の行のテストが通ることと、`make test` が通ることである。

### 4.2. 統合テスト

- `cmd/yt2column/run_test.go::TestRunTokenLinesLoopbackAdapter`（ステップ 4-5）が、記録済みの応答の形の `usage` から、実際のアダプタと CLI を通って行が出ることを、実際の API を呼ばずに確かめる（architecture §7.2）。
- 実際の API を呼ぶ統合テスト（`make test-integration-cli` など）は変更せず、本タスクでは実行しない（実行には人の承認が要る。CLAUDE.md「Tool Execution Safety」）。

### 4.3. セキュリティテスト

architecture §7.3 の検査を、ステップ 1-4（`AsNonNegativeInt64` のエラーに値を含めない）、ステップ 2-6（不明の理由が応答の値を含まない、`int64` を超える数・`-0`・重複が不明になる）、ステップ 3-7（中間生成物では `ErrMalformed` になり、メッセージにファイルの値を含めない）、ステップ 4-4（行が `redact.Line` を通り、`requireSafeOutput` が秘密情報と生の制御文字がないことを確かめる）に含める。

### 4.4. テストヘルパー

新しい `testutil/` のファイルは加えない。アダプタの `usage` を置き換える補助は各アダプタの既存の `test_helpers.go`（`//go:build test`、`test_organization.md` の分類 B）に加える。`internal/writer`・`cmd/yt2column` は既存の補助（§1.3）を使い、補助が要る場合は既存の `test_helpers.go` に加える。`TestFakesCarryBuildTag` が数える `testutil/` のファイルは増えない。

### 4.5. 既存のテストとの互換

requirements AC-27 が認める変更は、architecture §1.3「要件 AC-27 との関係」の一覧（アダプタの失敗の契約のテスト、`TestWriteArticle`、`TestCommonTypesFieldSets`・`TestInterfaceDocComments`、`TestPipelineCanceled` の「canceled after write」）と、版の値を書いた `TestParseRecordRejects` の行・`fieldsFrom`・`sameRecord`（architecture §3.10 の `format_test.go` の行）、コメントだけを直す Claude の `TestGenerateDuplicateMembers`・`TestGenerateUnconsumedMembers` だけである。ほかの既存のテストは内容を変えずに `make test` で通す。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

「検証の種類」の `test` は、実行され、誤った振る舞いで失敗するテストを指す。`static` は、実行される guard・`make` ターゲットを指す。`manual` は、PR での確認を指す。テストのパスは `path::TestName` の形で記す。表の新しいテストは、§2 の各ステップで加える。

| AC | 検証の種類 | 検証 |
|---|---|---|
| AC-01 | test | `internal/llm/deepseek/response_test.go::TestGenerateUsageCategories` |
| AC-02 | test | `internal/llm/claude/response_test.go::TestGenerateUsageCategories` |
| AC-02a | test | `internal/llm/claude/response_test.go::TestGenerateUsageNullCache`（`cache_read_input_tokens`・`cache_creation_input_tokens` のそれぞれ）と `::TestGenerateUsageUnknown`（`input_tokens`・`output_tokens` の `null` の行） |
| AC-03 | static | `internal/llm/usage_test.go::TestNoProviderMemberNames` |
| AC-04 | test | `internal/llm/deepseek/response_test.go::TestGenerateUsageUnknown`・`internal/llm/claude/response_test.go::TestGenerateUsageUnknown`（`usage` の不在・オブジェクトでない値・`null` の行）と `internal/llm/usage_test.go::TestUsageZeroValue`（不明が 0 の分かった値と区別できる） |
| AC-05 | test | 両アダプタの `::TestGenerateUsageUnknown`（要素ごと・不正な形ごとの行）と `internal/strictjson/strictjson_test.go::TestAsNonNegativeInt64` |
| AC-06 | test | 両アダプタの `::TestGenerateUsageUnknown`（`usage` の中の消費する要素の重複の行） |
| AC-07 | test | 両アダプタの `::TestGenerateUsageFixtures`（記録済みの応答）と `::TestGenerateUsageIgnoresUnconsumed` |
| AC-08 | test | 両アダプタの `::TestGenerateUsageUnknown`（理由が期待する語と等しく、注入した値を含まない）、`internal/llm/usage_test.go::TestUnknownReason`、`internal/strictjson/strictjson_test.go::TestAsNonNegativeInt64`（エラーに値を含まない） |
| AC-09 | test | 両アダプタの `::TestGenerateUsageKeepsVerdict` と `::TestGenerateUsageUnknown`（最上位の `usage` の重複でも応答が受理される） |
| AC-10 | test | `internal/writer/usage_test.go::TestWriteRunUsagePerStage` |
| AC-11 | test | `internal/writer/usage_test.go::TestWriteRunUsageInvariants`（呼ばなかった段階が `OriginNone`。分かった値・0・不明の区別は `::TestWriteRunUsagePerStage`・`::TestWriteRunUsageUnknownStage`） |
| AC-11a | test | `internal/writer/usage_test.go::TestWriteRunUsageUnknownStage` |
| AC-12 | test | `cmd/yt2column/run_test.go::TestRunTokenLinesAllStages` |
| AC-13 | test | `cmd/yt2column/run_test.go::TestRunTokenLinesSkippedStages` |
| AC-14 | test | `internal/writer/usage_test.go::TestWriteRunUsageResume`・`::TestWriteRunUsageInvariants` と `cmd/yt2column/run_test.go::TestRunTokenLinesResume` |
| AC-15 | static / test | `cmd/yt2column/docs_test.go::TestTokenUsageDocsContract`（README の書式と定数の一致）と `cmd/yt2column/run_test.go::TestRunTokenLinesAllStages`（固定の位置の固定の語） |
| AC-15a | test | `cmd/yt2column/run_test.go::TestRunTokenLinesUnknown` |
| AC-16 | test | `internal/writer/usage_test.go::TestWriteFailureRunUsage`（検証・推敲の呼び出しの失敗の行）と `cmd/yt2column/run_test.go::TestRunTokenLinesReviewFailure` |
| AC-17 | test | 両アダプタの `::TestGenerateRejectedResponseUsage`・`::TestGenerateSentinelsDistinct`、`internal/writer/usage_test.go::TestWriteFailureRunUsage`（生成と字数の調整の打ち切りの行）、`cmd/yt2column/run_test.go::TestRunTokenLinesRejectedResponse` |
| AC-18 | test | `internal/writer/usage_test.go::TestWriteFailureRunUsage`（期限切れ・応答の形の不正の行）と `cmd/yt2column/run_test.go::TestRunTokenLinesNoUsage` |
| AC-19 | test | `internal/writer/usage_test.go::TestWriteFailureRunUsage`（最後の字数の検査・保存の失敗の行）、`internal/pipeline/pipeline_test.go::TestPipelinePublishFailureCarriesRunUsage`、`cmd/yt2column/run_test.go::TestRunTokenLinesAfterAllCalls` |
| AC-20 | test | `internal/writer/usage_test.go::TestWriteFailureRunUsage`（字幕の検証・`--after` の読み込みの行がすべて `OriginNone`）、`cmd/yt2column/run_test.go::TestRunTokenLinesBeforeLLM`・`::TestRunTokenLinesResume`（`--after` を受理した後の失敗） |
| AC-21 | test | `internal/writer/usage_test.go::TestWriteSavesUsage`、`internal/artifact/format_test.go::TestMarshalParseRoundTrip`（分かった値と不明の行）、`cmd/yt2column/run_test.go::TestRunTokenLinesResume`（不明のまま保存した段階の再利用） |
| AC-22 | test | `internal/artifact/format_test.go::TestParseRecordRejectsUsage` と `cmd/yt2column/run_test.go::TestRunArtifactExitCodes`（`usage` が不正な中間生成物で LLM を呼ばず公開しない行） |
| AC-23 | test | `internal/artifact/format_test.go::TestParseRejectsVersion1` と `internal/artifact/chain_test.go::TestLoadNamesBrokenStage` |
| AC-24 | test | `internal/artifact/format_test.go::TestRecordIDCoversUsage` |
| AC-25 | test | AC-01〜AC-09・AC-17 のアダプタのテスト。どれも `httptest` のサーバに対して公開の `Generate` を呼ぶ。`make test` で実行する |
| AC-26 | test | AC-10〜AC-21 の `internal/writer`・`internal/pipeline`・`cmd/yt2column` のテスト。fake の `LLMClient`・`TranscriptSource`・`Publisher` と `t.TempDir` を使う。`make test` で実行する |
| AC-27 | static / manual | `make test` が既存のテストを通す（static）。§4.5 の一覧の外の既存のテストを変えていないことを、各 PR の差分で確認する（manual） |
| AC-28 | static / test | `make test` が `internal/server` の既存のテスト（`internal/server/job_test.go::TestFailureClassification` など）を変えずに通す（static）。`internal/writer/usage_test.go::TestTokenUsageErrorTransparent`（test） |
| AC-29 | static / manual | `cmd/yt2column/docs_test.go::TestTokenUsageDocsContract`（README の書式・区分・再利用の印・不明の表し方と、`project_overview.md`・`security.md`・`package_reference.md` の記述の存在）。記述の内容が architecture と一致することは PR で確認する |

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| `usage` を既存の最上位の `Collect` に加え、`usage` の重複で応答全体を拒否する | AC-09 が満たされず、プロバイダの変化で記事が生成できなくなる | 別の `Collect` で読む（ステップ 2-1・2-2）。最上位の重複で応答が受理されることをテストし、壊して確かめる（ステップ 2-6・2-7） |
| `Write` が段階の `Origin` の設定を忘れ、正しい形の誤った値（`OriginNone`）を返す | 呼んだ段階の行が出ず、料金が記録されない | architecture §1.3 の例外 3 の不変条件を組み合わせのすべてで確かめ、失敗の経路ごとに `Origin` を確かめる（ステップ 3-5）。設定を 1 つ消して失敗することを確かめる（ステップ 3-8） |
| `Parse` が `usage` を版の検査より前に読み、版 1 のファイルを `usage` の欠けとして拒否する | AC-23 の回復の手順のメッセージが出ない | `usage` を `parseStage` の後で読む（ステップ 3-3）。順序を入れ替えて `TestParseRejectsVersion1` が失敗することを確かめる（ステップ 3-8） |
| 版を 2 に上げたとき、版 `2` を「対応しない版」として書いた既存の行が受理される値になる | `TestParseRecordRejects` が失敗する | 行の版を `artifactVersion+1` に変える（ステップ 3-6） |
| `pipeline.Run` の包みで、`context.Canceled` を同一性で比べる既存のテストが失敗する | `TestPipelineCanceled` が失敗する | `errors.Is` での比較に改める（ステップ 4-2。architecture §1.3 に記録済み） |
| `reportRunError` の `reportTokens` を `*job.ReviewLogError` の分岐の前に置き、一覧のファイルの失敗で行が 2 回出る | 後続タスク 0018 が使用量を 2 回数える | 分岐の後に置き（ステップ 4-3）、行が 1 回だけ出ることをテストする（ステップ 4-4 の `TestRunTokenLinesReviewLogFailure`） |
| 行の値を引用符で囲み、`redact.Line` のエスケープで README の書式と出力が食い違う | 0018 が行を読めない | 値を引用符で囲まず、`redact.Line` の 1 層だけにする（architecture §3.8）。行を `redact.Line` の後のバイト列で比べ、README の手順でモデル名を戻せることをテストする（ステップ 4-4） |
| アダプタのテストの入力が実際の応答と異なる形になる | 受理のテストが実際の応答の受理を保証しない | 記録済みの応答をそのまま使い、`usage` だけを置き換える（ステップ 2-4、H-01） |
| `Write` の分岐と、意図して並べた 2 つのアダプタの `usage` の読み取りが、`gocyclo`（閾値 20）・`dupl`（`.golangci.yml`）に指摘される | `make lint` が通らない | 指摘されたら、段階ごとの記録や要素の読み取りを小さな非公開の関数に分ける。アダプタの間で共有の関数にはしない（architecture §3.4「重複を共有の関数にしない理由」） |

### 6.2. スケジュールのリスク

- 外部の待ち（実際の API の呼び出しや承認）を要する作業はない。各フェーズは単独でグリーンゲートを通せるので、遅れたフェーズはそのフェーズの中で吸収し、後のフェーズの範囲は変えない。

### 6.3. セキュリティリスク

- 不明の理由とトークン数の行に、応答の値・API キー・Webhook URL を含めない。理由は規則の列挙値とアダプタの定数だけから作り（ステップ 1-1・2-1・2-2）、`strictjson` のエラーのメッセージを理由に入れない。行は `stderrWriter.line` を通す（ステップ 4-3）。
- 中間生成物の `usage` は信頼できない入力として境界で検査し、補正せず、部分受理しない（ステップ 3-3・3-7）。トークン数の書き換えは `id` の照合で拒否する（AC-24）。`id` を計算し直したファイルを検出できないことは、architecture §5.1 の T4 の残余のリスクのとおりである。

## 7. 実装チェックリスト (Implementation Checklist)

- [x] architecture の 3 つの食い違いを editorial correction として直した（§2.0）
- [ ] フェーズ 1 の完了条件を満たす（ステップ 1-1〜1-5）
- [ ] フェーズ 2 の完了条件を満たす（ステップ 2-1〜2-7）
- [ ] フェーズ 3 の完了条件を満たす（ステップ 3-1〜3-8）
- [ ] フェーズ 4 の完了条件を満たす（ステップ 4-1〜4-6）
- [ ] フェーズ 5 の完了条件を満たす（ステップ 5-1〜5-6）
- [ ] §5 のすべての AC の `test`・`static` の検証が実行され、通る
- [ ] `make fmt`・`make test`・`make lint` がすべてのフェーズの完了時に通る

## 8. 成功基準 (Success Criteria)

- **機能:** 両アダプタが応答の `usage` を 4 つの区分で返し、不正な `usage` を不明として応答を拒否しない（AC-01〜AC-09）。`Write` が段階ごとの使用量と由来を成功と失敗の両方で返す（AC-10・AC-11・AC-11a・AC-16〜AC-20）。CLI が成功と失敗の要約に固定の書式の行を出す（AC-12〜AC-15a・AC-16〜AC-20）。中間生成物が使用量を来歴に持ち、版 1 と不正な `usage` を拒否する（AC-21〜AC-24）。
- **品質:** `make test`・`make lint` が通る。§4.5 の一覧の外の既存のテストを変えていない（AC-27）。サーバの振る舞いを変えていない（AC-28）。各テストが、対象の仕組みを壊して失敗することを確かめ、コミットメッセージに書いた。
- **セキュリティ:** 不明の理由・トークン数の行・中間生成物の拒否のメッセージが、応答とファイルの値・秘密情報を含まない（AC-08、requirements §4.2）。
- **文書:** README・`project_overview.md`・`security.md`・`package_reference.md` を更新し、`TestTokenUsageDocsContract` で確かめる（AC-15・AC-29）。

## 9. 次のステップ (Next Steps)

- 本書の承認後に、`/mkplan2 0019` で PR の境界と実装モデル要件を加え、`/runplan 0019` でフェーズ 1 から実装する。
- 実装中に architecture の記述が実際のコードと合わない箇所が見つかった場合は、本計画を独自に変えず、`requirements_process.md` の「Editing an approved document」に従って architecture を直す。
- 本タスクの完了後に、0018（#248）が基準の集合を作る（requirements §1）。
