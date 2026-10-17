# 実装計画書：LLM の料金の概算の算出と、利用者への提示

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-16 |
| Review date | 2026-10-16 |
| Reviewer | isseis |
| Comments | レビューでの修正: フェーズ 3 の対象ファイルと §3.2 の PR-5 の変更内容に、手動の確認（M-1〜M-3）の日時と結果を記録する `docs/tasks/0022_cost_estimate/03_implementation_plan.md` を加えた（AC-32・AC-34 の記録先を PR-5 の対象範囲と一致させた）。決定の変更はない。実装時の editorial correction（決定の変更はない）: ステップ 3-7 のテストの補助を `saveArtifactsOnce` から同形の `saveCostArtifacts` に改めた（共有の補助は setup の後に検証・推敲の client を上書きするため、レビューの段階に表にあるモデルと既知の使用量を与えられない）。`TestRunCostLinesOmitModel` の秘密情報の確認を、標準出力と標準エラーを見る形から標準エラーに限った（公開する記事の `- Model:` の行は本タスクの前から応答のモデル名をそのまま載せ、`requireSafeOutput` はその行を含む `--out` を見るため通らない。`--out` の行を赤くする変更は本タスクの対象外）。再利用した段階だけが不明な場合の確認を `TestRunCostLinesResumeUnknownStage` として分けた。 |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下「requirements」）の項番、H-NN は [design_handoff.md](design_handoff.md) の項目、節番号は [02_architecture.md](02_architecture.md)（以下「architecture」）の節を指す。既存のファイルの行番号は、断りがない限りコミット `c4eeaa6` で確かめた（その後のコミットは文書だけを変えている）。architecture が確かめたコミット `15d9b72` から `c4eeaa6` までの Go のファイルの差分は `cmd/yt2column-server` の `main.go` とそのテストだけで、本計画が変えるパッケージには差分がない（`git diff --stat 15d9b72 c4eeaa6 -- '*.go'` で確認）。Go のソースのコメント・識別子・文字列リテラルは英語で書き（`_context.md`）、`AC-NN`・`F-NNN`・`H-NN` は Go のソースに書かず本計画にだけ記録する（`requirements_process.md` §4）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

requirements の機能要件（F-001〜F-010）を、architecture の設計どおりに実装する。単価の表と算出を新しいパッケージ `internal/pricing` に置く。あわせて、`writer` が各呼び出しの時刻を記録して中間生成物の来歴に残し、段階ごとの料金の概算と 2 つの合計を、CLI の標準エラーの要約・`--out`・Slack の投稿・統合テストのログに出す。

用語（段階の使用量・実行の使用量・金額・段階の料金・合計・単価の組・適用した単価）は architecture の冒頭の用語の表に従う。実装は architecture §8 の実装優先順位どおり 4 つのフェーズに分ける。

### 1.2. 実装原則

- architecture §1.1 の設計原則 8 項目に従う。`internal/llm`（本番のコード）・`internal/pipeline`・`internal/job`・`internal/server`・`cmd/yt2column-server` は変更しない（architecture §3.11）。
- 単価の選び方・不明の理由・適用した単価は列挙型で宣言し、`switch` の `default` は不明か `invalid` にする（CLAUDE.md「Declare, don't infer」）。モデル名の照合は完全一致だけにする。
- 単価の表・算出・合計の規則は `internal/pricing`、標準エラーの料金の行の書式は `cmd/yt2column`、`--out`・Slack の行は `internal/publisher` の `renderArticle` の、それぞれ 1 か所に置く（requirements §4.5）。
- 各 PR の完了時に、`_context.md` の "Build checks"（`make fmt` → `make test` → `make lint`）を通す。グリーンゲートは `make test && make lint`。`make test` は `-tags test` で走る（`Makefile:71-72`）ので、新しいテストのファイルには既存と同じく `//go:build test` を付ける。`make lint` は `go vet -tags integration ./...` も実行する（`Makefile:196-198`）ので、`integration` のタグ付きのファイルの変更もグリーンゲートでコンパイルされる。
- 外部ライブラリは追加しない（requirements §5）。`internal/pricing` は標準ライブラリ（`math/big`・`time` など）と本リポジトリのパッケージだけを使う（architecture §2.1）。
- 各テストは、対象の仕組みを壊して失敗することを実装時に確かめ、そのことをコミットメッセージに書く（CLAUDE.md「Every test must be able to fail for its stated reason」）。本計画の「壊して確かめる」タスクは、この義務を具体化したものである。

### 1.3. 既存コード調査結果

**`internal/writer`（フェーズ 2 で変える）。** `StageUsage`（`writer.go:44-51`）は `Origin`・`Model`・`Usage` だけを持ち、`Options`（`writer.go:79-85`）に時計はない。`Generate` の呼び出しは 3 か所で、どれも直後に `stageUsage(resp)` を呼ぶ（`writer.go:415-416`・`shorten.go:28-29`・`review.go:256-257`）。3 か所とも `templateWriter` のメソッドの中なので、時計は `templateWriter` のフィールドとして渡せる。`reusedUsage`（`writer.go:403-405`）は `Write` の 3 か所（`:342`・`:358`・`:379`）で呼ばれる。保存する `StageRecord` は `generateStage`（`:424-434`）・`shortenStage`（`:461-471`）・`reviewStage`（`:491-502`）の 3 つのリテラルで作られる。`Write` の再開の分岐（`:322-331`）は `Load` の直後に連鎖を受け取るので、時刻のない記録の拒否はここで LLM の呼び出しより前に置ける。`ArtifactStore`（`artifact.go:80-89`）は `TestInterfaceDocComments`（`internal/pipeline/pipeline_test.go`）の対象ではないので、契約の文を加えても guard は変わらない。

**`internal/writer` のテスト（フェーズ 2 で変える）。** `RunUsage` を `!=` で比べるのは `usage_test.go:79`（`requireFailureUsage`）・`:98`・`:220`・`:245`・`:274`・`:290`・`:316` と `output_test.go:85`（`TestWriteArticle`）である（`rg -n 'RunUsage !=|a != want' internal/writer/*_test.go` の結果。コミット `c4eeaa6`）。fake の `ArtifactStore` に連鎖を渡すのは `artifact_test.go` の `resumeChainGeneration`・`resumeChainShorten`・`resumeChainReview`（`:62-89`。`overLimitChain`（`:485-489`）はこれを使う）と `usage_test.go:193-205`・`:254`・`:297`・`:495-496` だけである（`rg -n 'loadResult' internal/writer` で確認）。保存した記録を比べる `sameArtifact`（`artifact_test.go:34-40`）は `Usage` を比べず、`CallTime` も比べないので、`TestWriteSavesAllArtifacts` などはフィールドを加えても変わらない（architecture §1.3 は「レビューでの了承が要る」変更の候補として挙げるが、変更は要らない）。保存した記録の時刻は新しいテスト（ステップ 2-6）で確かめる。

**`internal/artifact`（フェーズ 2 で変える）。** `artifactVersion = 2`（`artifact.go:21`）、`artifactKeys`（`format.go:39-42`）、`recordJSON`（`:53-66`）、`Marshal`（`:102-145`。最後に `Parse` で往復を確かめる）、`Parse`（`:150-219`。`CollectOnly` の後に `parseStage` で版を確かめる）、`parseStage` の版の拒否のメッセージ（`:232-236`。版の数字は `artifactVersion` から作る）、`recordID`（`:437-464`。`usageIDParts` の直後に部分を加える）を変える。時刻の要素の文字列は `strictjson.RequiredString`（`strictjson.go:145`。既存の `parseModel` が使う）で読める。テストの補助は `validGeneration`（`format_test.go:38-48`）・`validReview`（`:55-64`）・`sameRecord`（`:69-87`。フィールドを 1 つずつ比べるので `CallTime` の比較を加えないと往復のテストが時刻の欠落を検出しない）・`fieldsFrom`（`:125-154`）であり、`store_test.go` の `shortenOf`・`reviewOf`（`:33-49`）と `chain_test.go` は `validGeneration` から値を作る。`TestParseRejectsVersion1`（`format_test.go:695-707`）は `"version 2"` をメッセージに求める。`TestParseRecordRejects` の版の行（`:284`）は `artifactVersion+1` で書かれており、版を上げても変更は要らない。版を述べるコメントは、本番のコードの `artifact.go:20`（「Version 2 adds the usage element to version 1.」）とテストの `chain_test.go:129-130`（「valid version 2 generation and review chain」）にある。`Parse` の `id` の照合は最後の検査である（`format.go:215-217`）ので、`fieldsFrom` を変えただけの入力は、時刻の検査を外しても `id` の照合で拒否される。

**中間生成物を作るほかのテスト（フェーズ 2 で変える）。** `internal/artifact` の外で `artifact.Marshal`・`artifact.Parse` を呼ぶのは `cmd/yt2column/run_test.go:2015`（`requireArtifactStages`。段階の名前だけを確かめる）と `:2361`（`TestRunArtifactExitCodes` の `validGeneration` が `writer.StageRecord` のリテラルから作る）だけである（`rg -n 'artifact\.Marshal|artifact\.Parse' -g '*.go'` から `internal/artifact` の中を除いた結果）。CLI のほかの中間生成物のテスト（`saveArtifactsOnce`、`run_test.go:2056-2068`）は CLI を実行して保存するので、`Clock` が nil（`time.Now`）のまま時刻が入る。

**`internal/publisher`（フェーズ 3 で変える）。** `renderArticle`（`file.go:203-214`）は `FilePublisher`（`file.go:111`）と Slack の両方が使う。見出し全体を比べるのは `file_test.go:64` と `TestRenderArticleSteps`（`:94-125`）と、`integrationHeader`（`testutil/integration.go:168-170`）である。Slack のテスト（`slack_test.go`・`slack_message_test.go`）は期待値を `renderArticle` から作る。メンションの検査は `findMention`（`slack_message.go:159`）、分割は `splitSlackText`（`:75`）である。

**`docs/tasks/0008_column_prompt/check_article.py`（フェーズ 3 で変える）。** `HEADER_RE`（`:29-33`）はモデル名の行の直後に空行を求めるので、料金の行を加えた `--out` のファイルは `unrecognized --out format`（`:95-96`）になる。0018 の評価が 0008 の手順でこのスクリプトを使うので、architecture の変更の対象に加えた（editorial correction。§2.0）。

**`cmd/yt2column`（フェーズ 3 で変える）。** `tokensLine`（`run.go:637-666`）・`reportTokens`（`:670-684`）、`reportSuccess` の最後の呼び出し（`:714`）、`reportRunError` の呼び出し（`:795-797`。`*job.ReviewLogError` の分岐の後）がある。トークン数の行の接頭辞は `tokensPrefix = programName + ": tokens: "`（`run.go:48`）で、料金の行の接頭辞（`yt2column: cost: `・`yt2column: cost-total: `）とは前方一致しない。既存の `token_test.go` は `tokenLines`（`:59-67`）で `tokens:` の行だけを取り出して比べ、`requireSafeOutput`（`run_test.go`）は各行が `yt2column: ` で始まることを求める。CLI の `writer.Options` は `deps.newWriter`（`run.go:76`・`:547`）で渡るので、テストは `e.d.newWriter` を包んで `Options.Clock` を設定できる（前例は `run_test.go:1687`）。`run.go` はすでに `internal/artifact` を import している。

**統合テスト（フェーズ 3 で変える）。** `internal/llm/deepseek/integration_test.go` と `internal/llm/claude/integration_test.go` は `//go:build integration` で、`generate`（`:45`・`:59`）と `truncated`（`:80`・`:90`）のサブテストを持つ。`generate` はモデル名をログに出す前に、API キーを含まないことを確かめる（DeepSeek `:62-75`、Claude `:79-87`）。`truncated` は応答を捨てており（`_, err :=`）、この確認を持たない。CLI の統合テストは、成功した実行の標準エラーをログに出さない。`runIntegrationCLI`（`cmd/yt2column/integration_test.go:46-65`）は標準エラーを `requireNoSecrets` にだけ渡し、`integration_slack_test.go:58-66` は失敗のときだけ標準エラー全体を、成功では投稿の要約の行だけを出す。そのため architecture §3.8 の「コードの変更は要らない」は事実でなく、§2.0 のとおり architecture を直した。

**文書（フェーズ 1・2・4 で変える）。** `README.md` の `## Token usage`（`:122-185`）の最後の段落が「The artifact format is version 2」（`:183-185`）と書く。`docs/dev/developer_guide/package_reference.md` の `internal/artifact` の行（`:33`）は版 2 を書き、`cmd/yt2column/docs_test.go:751` がその行に `"version 2"` を求める。`docs/dev/project_overview.md` は中間生成物の形式の版を書いていない（`--artifacts` の項は `:34`、トークン数の行の項は `:35`、モデル名の制約は `:66`）ので、AC-42 の「形式の版」は改めるのではなく書き加える。`docs/dev/security.md` の `## 6.` の `--artifacts` の項は `:78` である。`TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:669`）は `cmd/` と `internal/` のすべてのパッケージの行を `package_reference.md` に求めるので、`internal/pricing` の行はパッケージを作るフェーズ 1 で要る。`docs/dev/developer_guide/` の英語の文書に日本語訳を求める guard はない（`rg -n 'ja\.md' -g '*_test.go' cmd internal` で該当なし）。

**外部の前提の確認。** モジュールは `go 1.26.5`、ツールチェーンは `go1.27.1`（`go.mod:3`・`go version`）。architecture §3.5 の時刻の受理の規則の前提を、`go1.27.1` で書式 `2006-01-02T15:04:05Z` に対して、`time.Parse` と書き直しの比較（`time.Parse` が返した時刻を同じ書式で書き戻し、元の文字列と比べること）を実行して確かめた（scratchpad の一回限りのプログラム。2026-10-16）。結果は次のとおりである。`2026-02-30T00:00:00Z`・`24:00:00`・`23:59:60`・小文字の `z`・小文字の `t`・`+00:00` は、`time.Parse` が拒否した。`.5Z`（小数の秒）と `T1:00:00Z`（桁の欠け）は、`time.Parse` は受理したが、書き直しの比較で不一致になった。`0001-01-01T00:00:00Z` は往復も一致するので、ゼロの時刻は別に拒否する必要がある。`Asia/Tokyo` 相当の `time.FixedZone` の時刻をそのまま書くと、9 時間ずれた値に `Z` が付いた。architecture §3.4 の「`UTC()` と `Truncate` はモノトニック時計の読みを除く」は `go doc time` の "Monotonic Clocks" の節（"t.In, t.Local, and t.UTC ... strip any monotonic clock reading"、"t.Round(d) and t.Truncate(d) ... strip"）で確かめた。

**具体型での分岐。** 本タスクは、具体型で振る舞いを分ける設計を加えない。`StageCost`・`Total`・`Amount` はフィールドを非公開にし、`internal/pricing` の外では `Estimate`・`Totals` とゼロ値でしか作れない（architecture §3.1）。パッケージの中でリテラルを作る箇所は `Estimate`・`Totals` だけにし、ゼロ値が不明であることはステップ 1-5 のテストで確かめる。

### 1.4. 新しい名前の一覧

本タスクで加える名前である。既存の名前の変更・削除はない。以降の本文と AC の表はこの表の名前を使う。

| 置き場所 | 名前 | 出典 |
|---|---|---|
| `internal/pricing`（新規） | `Amount`・`Amount.Format`、`RateBasis`（`BasisUnset`・`BasisFlat`・`BasisOffPeak`・`BasisPeak`・`BasisShortPrompt`・`BasisLongPrompt`）、`UnknownReason`（`ReasonUnset`・`ReasonUsageUnknown`・`ReasonNoModel`・`ReasonUnpricedModel`・`ReasonInvalid`）、`StageCost`・`StageCost.Known`・`StageCost.Unknown`、`Total`・`Total.Amount`、`Estimate`・`Totals`。非公開の `microUSDPerMTok`・`rates`・`rateScheme`・`modelRates`・`peakRule`・`minuteRange`・`validate` と、§3.2 の表の番兵 | architecture §3.1・§3.2 |
| `internal/writer` | `StageUsage.CallTime`（フェーズ 1）・`StageRecord.CallTime`・`Options.Clock`・`ErrMissingCallTime` | architecture §3.4・§4.1・§8 |
| `internal/pricing/testutil`（新規、`package pricingtestutil`、`//go:build test \|\| integration`） | 段階の料金を、適用した単価の種類か不明の理由とともに 1 行のログの文字列にする関数 | architecture §3.8（2 つのアダプタの統合テストが同じ語でログを出すため、本計画で置き場所を決める。CLI の語の対応は `package main` にあり、ほかのパッケージから使えない） |
| `internal/artifact` | JSON のキー `call_time`。CLI の `at=` と同じ書式を使うための公開の定数 `CallTimeLayout`（値は `2006-01-02T15:04:05Z`） | architecture §3.5・§3.6（「時刻の書式の文字列は、`internal/artifact` の `call_time` の書式と同じ値を使う」の実現の仕方として、本計画で名前を決める） |
| `cmd/yt2column` | `reportUsage` と、料金の行・合計の行の書式の定数（2 つの接頭辞・状態の語・合計の種類の語・`usd=`・`reason=`・`at=`・`none`）、列挙値を語に対応させる関数 | architecture §3.6 |
| 文書 | `docs/dev/developer_guide/pricing_update.md`（新規・英語）、README の料金の節 | architecture §3.9 |

### 1.5. 申し送りへの対応

`design_handoff.md` の H-01〜H-05 は architecture §3.10 が対応を記録しており、本計画はそのとおりに実装する。`implementation_handoff.md` は存在しない。

| 項目 | 本計画での扱い |
|---|---|
| H-01 単価の表の置き場所と形 | ステップ 1-1（表・出典のコメント・`validate`）、ステップ 1-5（表の形のテスト） |
| H-02 算出の方法と桁あふれ | ステップ 1-2・1-3（`math/big` の整数と四捨五入）、ステップ 1-5（手で計算した期待値と巨大なトークン数） |
| H-03 呼び出しの時刻の取得と受け渡し | ステップ 2-1・2-2（`Options.Clock` と `CallTime`）、ステップ 2-5（`writer` のテスト） |
| H-04 料金の `Publisher` までの経路 | ステップ 3-1（`renderArticle` が `pricing.Totals` を呼ぶ）、ステップ 3-3 |
| H-05 中間生成物の時刻の形 | ステップ 2-3（版 3 と `call_time`）、ステップ 2-6（拒否のテスト） |

### 1.6. 対象外（requirements §2.3）

日本円などへの換算、単価の取得のためのネットワーク、中国の祝日の判定、表にないモデルの単価、プロバイダの割引と特別な料金体系、Claude のキャッシュの書き込みの TTL ごとの単価、失敗の通知の投稿への料金の記載、localhost サーバのログと HTTP の応答への料金の記載、機械可読な出力のための新しいフラグ、料金による実行の制限は、本タスクの対象外である。

### 1.7. 横断検索チェックリスト

本タスクは既存の名前を変更・削除しない。中間生成物の形式の版を述べる文書は、ステップ 2-8 で版 3 に改め、README の記述と `package_reference.md` の記述を guard（`TestTokenUsageDocsContract`）で確かめる。`reportTokens` の呼び出しを置き換えたときに事実でなくなるコメント（`reportSuccess` の文書の「one token-usage line per stage」、`run.go:685-690`）はステップ 3-6、版 2 を述べる Go のコメント（`artifact.go:20`・`chain_test.go:129-130`）はステップ 2-3・2-4、`integrationHeader` の文書（`testutil/integration.go:165-167`）はステップ 3-2 のタスクにした。これらのほかに、`make lint`・`make test` で検出できない残存参照として、次の 1 つを手で確かめる。

- [x] `docs/` の `docs/tasks/` を除く文書と `README.md` が、中間生成物の形式の版を 2 と書いていないこと（ステップ 2-8 の完了時）。`docs/tasks/` の過去のタスクの記録は、当時の事実として残す。

## 2. 実装ステップ (Implementation Steps)

### 2.0. 前提（解消済み）

計画の作成時に、architecture の記述の漏れと古くなった行番号を見つけ、editorial correction として直した（status は `approved` のまま。`Comments` に記録。コミット `c4eeaa6`・`33780ab`）。設計の決定は変えていない。

- §1.3 の例外 1 と §3.11 に、`docs/tasks/0008_column_prompt/check_article.py` を加えた（§1.3 の調査結果）。
- §1.3 の `cmd/yt2column-server/run_test.go` の行番号を、0021 の変更の後の `:481-503` に改めた。
- 計画のレビューで、§3.8 の「CLI の統合テストは標準エラーをログに出している」と §8 の「`Totals` は既存の型だけを使う」が事実でないことが分かり、§3.8・§3.11（CLI の統合テストに料金の行のログを加える）と §8（フェーズ 1 で `StageUsage.CallTime` のフィールドだけを加える）を直した（コミット `33780ab`）。2 つのアダプタの統合テストが共有するログの補助 `internal/pricing/testutil`（ステップ 3-9）はテストの補助の置き場所であり、architecture の設計の要素を変えないので、本計画で決める（§1.4）。

### フェーズ 1: `internal/pricing`

**対象ファイル:** `internal/pricing/rates.go`・`amount.go`・`pricing.go`（新規）、`internal/pricing/rates_test.go`・`amount_test.go`・`pricing_test.go`（新規）、`internal/writer/writer.go`、`docs/dev/developer_guide/package_reference.md`

- [x] **ステップ 1-1**: `internal/pricing/rates.go` に、architecture §3.2 の非公開の型・単価の表・ピーク時間帯の規則・`validate` と §3.2 の表の番兵を置く。表に写す直前に、出典の 2 つのページ（requirements F-001）で単価とピーク時間帯をもう一度確かめ、各エントリとピーク時間帯の規則の直前のコメントに出典の URL と確認日（確かめた日）を書く。architecture §3.2 の表の値とページが食い違えば、実装を止めて architecture を直す（§9）。DeepSeek のキャッシュに書き込んだ入力の単価を 0 とすることは、アダプタの宣言に対応している。この対応関係をコメントに書く。
- [x] **ステップ 1-2**: `internal/pricing/amount.go` に、architecture §3.1 の `Amount` と `Format`（§3.3 の四捨五入）を置く。
- [x] **ステップ 1-3**: `internal/writer/writer.go` の `StageUsage` に、architecture §3.4 の `CallTime` のフィールドと文書だけを加える（`Totals` が時間帯の判定に読むため。architecture §8）。時刻を入れる処理はフェーズ 2 で加えるので、このフェーズでは常にゼロ値であり、既存のテストは変わらない。`internal/pricing/pricing.go` に、パッケージの文書と、architecture §3.1 の `RateBasis`・`UnknownReason`・`StageCost`・`Total`・`Estimate`・`Totals` を置く。`Estimate` の判定の順序と単価の組の選び方は §3.3 と図5、`Totals` の規則は §3.1 のとおりとする。プロンプトの長さの和の判定は桁あふれさせない（§3.3）。
- [x] **ステップ 1-4**: `docs/dev/developer_guide/package_reference.md` に `internal/pricing` の行を、architecture §2.2 の責務と §3.1 の公開の型で書く（`TestPackageReferenceListsPackages` が新しいパッケージの行を求めるため、このフェーズで要る）。この行はフェーズ 4 で書き直さない。
- [x] **ステップ 1-5**: テストを書く。期待値は出典のページの単価から手で計算した文字列とし、計算式をテストのコメントに書く。表の定数や算出のコードから期待値を導かない（AC-35）。
  - `rates_test.go`: `TestRateTableValid`（実際の表に `validate` を通す）、`TestValidateRejects`（§3.2 の規則ごとに、その規則だけを壊した表で、その番兵が `errors.Is` で返る）、`TestRateTableModels`（4 つのモデル名ごとの単価の選び方と組の数。閾値の値は `TestEstimatePromptTier` が振る舞いで固定するので、ここでは定数と同じ値のリテラルと比べない。AC-01）、`TestRateFieldsUnsigned`（`rates` の 4 つのフィールドと `microUSDPerMTok` の型の種類が符号なしの整数であることを `reflect` で確かめる。AC-03 の負の単価）、`TestRateTableSourceComments`（`go/parser` で `rates.go` を読み、表の各エントリとピーク時間帯の規則の直前のコメントが、出典の URL と `checked YYYY-MM-DD` の形の確認日を持つ。AC-02）。
  - `amount_test.go`: `TestAmountFormat`（0、ちょうど 0.5 の桁の切り上げ、0.5 未満の切り捨て、6 桁と 4 桁、整数部が 0 と 1 以上、指数を使わないこと）。
  - `pricing_test.go`: `TestEstimateCategories`（4 つのモデルで 4 区分が互いに異なる 0 でない値。AC-05）、`TestEstimatePeakWindow`（architecture §7.1 の境界の 8 つの時刻、土曜・日曜の 01:00:00、UTC 以外のタイムゾーンの時刻。UTC 以外の行は、その地方時の曜日と時刻で読むと判定が変わる値にし、曜日がまたがる値（例: UTC の金曜 00:00 が地方時の土曜になる値）を含める。AC-06）、`TestEstimateIgnoresCallTime`（Claude の 3 モデルでピーク・オフピーク・ゼロの時刻が同じ金額と同じ適用した単価。AC-07）、`TestEstimatePromptTier`（和が 100,000 と 100,001、区分ごとに 1 つだけで閾値を超える入力、`uint64` で素朴に足すと桁あふれして閾値以下になる入力（例: 入力が `uint64` の上限、キャッシュの読みが 2）が閾値超になること、ほかの 3 モデルで 100,000 を超えても単価が変わらない。AC-07a・AC-08）、`TestEstimateLargeCounts`（各区分が `int64` の上限と `uint64` の上限。手で計算した金額。AC-08）、`TestEstimateUnknown`（architecture §7.1 の不明の各入力と理由、時間帯のモデルのゼロの時刻の `ReasonInvalid`。AC-04・AC-09・AC-10）、`TestEstimateUnknownScheme`（実際の表では通らない `default` の分岐を、単価の組を選ぶ非公開の関数に列挙の外の `rateScheme` のエントリを渡して、`ReasonInvalid` の不明になることで確かめる）、`TestZeroValuesAreUnknown`（`StageCost{}` が `ReasonUnset` の不明、`Total{}` が不明）、`TestTotals`（architecture §7.1 の由来の組み合わせ。AC-11〜AC-14）。
- [x] **ステップ 1-6**: 壊して確かめる。次の操作で、それぞれ対応するテストが失敗することを確かめる。
  - `validate` の規則を 1 つずつ外す: `TestValidateRejects` の該当する行。
  - ピーク時間帯の判定を `UTC()` を通さない形にする: `TestEstimatePeakWindow` の UTC 以外の行。
  - 範囲の終了を含む比較にする: 同テストの境界の行。
  - プロンプトの長さの和から区分を 1 つ外す: `TestEstimatePromptTier` のその区分の行。
  - 和の判定を素朴な `+` にする: 同テストの桁あふれの行。
  - `Totals` が `OriginReused` を「この実行の料金」に含める: `TestTotals`。
  - `Format` を切り捨てにする: `TestAmountFormat`。
  - 表のエントリのコメントから確認日を消す: `TestRateTableSourceComments`。

**完了条件:** グリーンゲートが通る。`internal/pricing` はまだどこからも import されない（`make deadcode` の報告に新しい関数が出ることは、フェーズ 3 で解消する）。

### PR-1 作成ポイント: pricing package

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6

**推奨タイトル**: `feat(0022): add the pricing package with the rate table and cost estimation`

**レビュー観点**: 単価の表の値が出典のページと一致し、各エントリに出典の URL と確認日のコメントがあり、`validate` の規則ごとの番兵がその規則だけで返ること / `Estimate` の不明の理由の優先順位、ピーク時間帯の UTC と半開区間の判定、プロンプトの長さの和の桁あふれのない判定 / 期待値が出典の単価から手で計算され、計算式がコメントにあること（表の定数から導いていないこと）/ `StageCost`・`Total` のゼロ値が不明で、`Totals` が由来で集合を選ぶこと / ステップ 1-3 の `internal/writer` の差分が `StageUsage.CallTime` のフィールドと文書だけで、既存のテストを変えていないこと

**実装モデル要件**: standard

**判定理由**: 算出の方法（`math/big` の整数・四捨五入・単価の選び方）は architecture §3.1〜§3.3 に確定しており、競合する実装案はない。新しいパッケージの追加で、移行・外部リソース・前例のない設計判断のいずれにも当たらない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 呼び出しの時刻と中間生成物

**対象ファイル:** `internal/writer/writer.go`・`artifact.go`・`errors.go`・`shorten.go`・`review.go`、`internal/writer/usage_test.go`・`output_test.go`・`artifact_test.go`、`internal/writer/calltime_test.go`（新規）、`internal/artifact/artifact.go`・`format.go`・`format_test.go`・`chain_test.go`、`cmd/yt2column/run_test.go`・`docs_test.go`、`README.md`、`docs/dev/developer_guide/package_reference.md`

- [x] **ステップ 2-1**: architecture §3.4 の残りの型を置く（`StageUsage.CallTime` はステップ 1-3 で加えた）。`StageRecord.CallTime`、`Options.Clock`、`errors.go` の `ErrMissingCallTime`。`ArtifactStore` の文書に「`Load` が返す各記録の `CallTime` はゼロでない」を加える。
- [x] **ステップ 2-2**: `Write` に時刻の取得と受け渡しを加える。`New` が `Options.Clock`（nil は `time.Now`）を `templateWriter` に持たせ、`Generate` が戻った直後（成否にかかわらず）に時刻を取って `stageUsage` に渡す。`stageUsage` は UTC・秒に切り捨てて `CallTime` に入れ、保存する 3 つの `StageRecord` に同じ値を入れる。`reusedUsage` は記録の時刻を写す。`Write` は、読み込んだ連鎖の記録のどれかの `CallTime` がゼロなら、LLM を呼ばずに `ErrMissingCallTime` を包んだ `*ArtifactError` で失敗する（architecture §3.4）。この失敗も、ほかの `Write` の失敗と同じく `*TokenUsageError` で包む（`ArticleWriter` の契約）。
- [x] **ステップ 2-3**: `internal/artifact` を architecture §3.5 の版 3 にする。`artifactVersion` を 3、`call_time` を `usage` の直後に置き、`CallTimeLayout` を公開の定数にする。`Parse` は `call_time` を `parseStage` の後で読む（版 2 のファイルを、時刻の欠けでなく版の違いで拒否するため）。`Marshal` は書く前に時刻の 3 つの条件を検査する。`recordID` の `usageIDParts` の直後に時刻の部分を加える。拒否のメッセージは architecture §4.2 の形で、ファイルの値を含めない。`artifactVersion` の文書（`artifact.go:20`）を版 3 の内容に改める。
- [x] **ステップ 2-4**: 既存のテストを更新する（architecture §1.3「要件 AC-39 との関係」）。トークン数・モデル名・由来の確認は変えない。
  - `internal/writer`: `RunUsage` を `!=` で比べるテスト（§1.3 の 8 か所）が呼ぶ `Write` の、すべての構築（`TestWriteFailureRunUsage` の各サブテストの `mustNew`・`withStepsEnabled` を含む）に、段階ごとに異なる時刻を返す固定の時計を入れ、期待する値に `CallTime` を加える。これで、失敗した呼び出しの段階の時刻（AC-19）と再利用した段階の時刻（AC-28）を、既存の `TestWriteFailureRunUsage`・`TestWriteRunUsageResume` が確かめる。再開の連鎖の値（`resumeChain*`、`usage_test.go` の 4 か所）に、固定の時計のどの時刻とも異なる `CallTime` を加える。`requireRunUsageInvariants`（`usage_test.go:46`）に architecture §3.4 の 2 つの不変条件を加える。固定の時計の補助は `calltime_test.go` に置き、同じパッケージのほかの `_test.go` から使う。
  - `internal/artifact`: `validGeneration` に `CallTime` を、`fieldsFrom` に `call_time` を、`sameRecord` に `CallTime` の比較を加える。`TestParseRejectsVersion1` が求めるメッセージの `"version 2"` を `"version 3"` にし、`chain_test.go:129-130` のコメントの版を改める。
  - `cmd/yt2column/run_test.go`: `TestRunArtifactExitCodes` の `validGeneration` の記録に `CallTime` を加える。
- [x] **ステップ 2-5**: `internal/writer/calltime_test.go` に、fake の `LLMClient`（`FakeLLMSequence`・`FakeLLMClient`）と fake の `ArtifactStore` で、architecture §7.1 の `internal/writer` のテストを書く。
  - `TestWriteCallTimePerStage`: 呼び出しごとに異なる時刻（UTC 以外のタイムゾーン、ナノ秒を持つ値を含む）を返す時計で、各段階の `StageUsage.CallTime` と保存した `StageRecord.CallTime` が、その呼び出しの時刻を UTC・秒に切り捨てた値であること（AC-27・AC-36）。
  - `TestWriteRejectsMissingCallTime`: 連鎖の生成・字数の調整・検証・推敲の記録のそれぞれだけが時刻を持たない場合に、`Write` が `errors.Is(err, ErrMissingCallTime)`・`*ArtifactError`・ゼロ値の `RunUsage` を持つ `*TokenUsageError` で失敗し、fake の `LLMClient` が呼ばれないこと。
  - `TestWriteDefaultClock`: `Options.Clock` が nil のとき、`CallTime` が呼び出しの前後に `time.Now` で取った時刻の範囲（秒に切り捨て）に入ること。
- [x] **ステップ 2-6**: `internal/artifact/format_test.go` にテストを書く。
  - `TestMarshalParseRoundTrip` に、異なる時刻の行を加える。
  - `TestParseRecordRejectsCallTime`: architecture §3.5 の拒否の一覧の各入力（欠け・重複・文字列でない値・UTF-8 の不正・オフセット付き・小文字の `z`・小数の秒・秒の欠け・空白・区切りの違い・桁数の違い・範囲外の日付と時刻・ゼロの時刻）を `ErrMalformed` で拒否し、メッセージに入力の値を含めないこと（AC-29）。`id` の照合は `Parse` の最後の検査なので（§1.3）、`time.Parse` が値を返す行（小数の秒・桁の欠け・ゼロの時刻など）は、その値で `recordID` を計算し直した `id` を入れ、時刻の規則だけが拒否できる入力にする（CLAUDE.md「A layered path needs inputs only one layer can handle」）。重複は `fieldsFrom` を経由せずバイト列で作る。
  - `TestParseRejectsVersion2`: `call_time` のない版 2 の中間生成物を、版の違いのメッセージと `ErrMalformed` で拒否すること（AC-30）。
  - `TestRecordIDCoversCallTime`: 時刻だけが異なる 2 つの記録の `id` が異なること、`id` を変えずに時刻だけを正しい形の別の時刻に書き換えたファイル（時刻の規則では拒否されない入力）を `ErrMalformed` で拒否すること（AC-31）。
  - `TestMarshalRejectsNonCanonicalCallTime`: ゼロの時刻・UTC でないタイムゾーンの時刻・ナノ秒を持つ時刻の記録を、`Marshal` がそれぞれ `ErrMalformed` で拒否すること（architecture §3.5）。
- [x] **ステップ 2-7**: `cmd/yt2column/run_test.go` の `TestRunArtifactExitCodes` に、`call_time` が不正な中間生成物で `--after` を実行する行を加え、既存の行と同じく `requireUntouched` で LLM を呼ばず公開しないことを確かめる（AC-29 の CLI の部分、architecture §7.3 の T1）。行は既存の「a malformed usage is a run failure」の行（`run_test.go:2405-2425`）と同じく、`saveArtifactsOnce` で保存した中間生成物の `call_time` の値だけを、`time.Parse` が拒否する形（例: オフセット付き）に書き換えて作る。`time.Parse` が拒否する値は `id` の照合より前に時刻の検査で拒否されるので、`id` を計算し直す必要はない。
- [x] **ステップ 2-8**: 版の数字を述べる文書を同じフェーズで改める（版を上げたフェーズで文書が事実でなくなるのを避けるため。ほかの文書の更新はフェーズ 4）。`README.md:183-185` の版の記述を版 3（版 2 を再開に使えない）に、`package_reference.md` の `internal/artifact` の行の版を 3 と `call_time` に改める。`cmd/yt2column/docs_test.go` の `TestTokenUsageDocsContract` の `internal/artifact` の行の `"version 2"` を `"version 3"` にして `call_time` を加え、README の `## Token usage` の節が `"version 3"` を含むことの確認を加える。§1.7 の横断検索を行う。
- [x] **ステップ 2-9**: 壊して確かめる。次の操作で、それぞれ対応するテストが失敗することを確かめる。
  - `stageUsage` で秒への切り捨てを外す: `TestWriteCallTimePerStage`。
  - `generateStage`・`runShorten`・`runReview` のそれぞれで、時計の代わりにゼロの時刻を渡す: `TestWriteCallTimePerStage` のその段階の確認と、`TestWriteFailureRunUsage` のその段階の呼び出しの失敗の行。
  - `reusedUsage` が時刻を写さないようにする: `TestWriteRunUsageResume`。
  - 時刻のない再利用の記録の拒否を外す: `TestWriteRejectsMissingCallTime`。
  - `Parse` で書き直しの比較を外す: `TestParseRecordRejectsCallTime` の小数の秒と桁の欠けの行。
  - `Parse` でゼロの時刻の拒否を外す: 同テストのゼロの時刻の行。
  - `recordID` から時刻を外す: `TestRecordIDCoversCallTime`。
  - `Parse` で `call_time` を `parseStage` の前に読む: `TestParseRejectsVersion2`。
  - `Marshal` の検査を外す: `TestMarshalRejectsNonCanonicalCallTime` の UTC でない行とナノ秒の行。

**完了条件:** グリーンゲートが通る。中間生成物は版 3 で保存され、版 2 を拒否する。CLI と `Publisher` はまだ料金を出さない。

### PR-2 作成ポイント: call time and artifact format version 3

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9

**推奨タイトル**: `feat(0022): record call times and raise the artifact format to version 3`

**レビュー観点**: `Generate` の戻りで取った時刻が UTC・秒に切り捨てられて `StageUsage` と保存する `StageRecord` の両方に入り、再利用した段階が記録の時刻を使うこと / `call_time` の受理の形が 1 つで、書き直しの比較・ゼロの時刻の拒否・`Marshal` の事前の検査・`recordID` への追加があり、版 2 を版の違いとして拒否すること / 時刻のない再利用の記録を LLM を呼ばずに `ErrMissingCallTime` で拒否すること / 既存のテストの更新が §4.5 の一覧の範囲に収まり、版の数字を述べる文書が同じ PR で改められていること

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-3・2-8 が中間生成物の形式の版を 2 から 3 に上げて古い版を拒否する移行であるため（`mkplan.md` ステップ 8 のパネルモードのトリガーの、移行に該当）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 出力

**対象ファイル:** `internal/publisher/file.go`・`file_test.go`・`slack_test.go`・`slack_message_test.go`、`internal/publisher/testutil/integration.go`、`docs/tasks/0008_column_prompt/check_article.py`、`internal/pipeline/pipeline_test.go`、`cmd/yt2column/run.go`、`cmd/yt2column/cost_test.go`（新規。CLI の料金のテストは `token_test.go` と同じパッケージの新しいファイルに置く）、`cmd/yt2column/integration_test.go`・`integration_slack_test.go`、`internal/pricing/testutil/helpers.go`・`helpers_test.go`（新規）、`internal/llm/deepseek/integration_test.go`、`internal/llm/claude/integration_test.go`、`docs/dev/developer_guide/package_reference.md`、`docs/tasks/0022_cost_estimate/03_implementation_plan.md`（手動の確認 M-1〜M-3 の日時と結果）

- [x] **ステップ 3-1**: `renderArticle` に、architecture §3.7 の料金の行を 1 行加える。金額は `pricing.Totals` の記事 1 本の料金の `Format(4)` と `USD`、不明は `unknown` だけとする。`renderArticle` の文書に行の位置と内容を加える。
- [x] **ステップ 3-2**: 見出しの形を固定する既存のものを更新する（architecture §1.3 の例外 1）。`file_test.go:64` と `TestRenderArticleSteps` の期待値に不明の行を加え、`integrationHeader` に同じ行を加え、その文書（`testutil/integration.go:165-167`）に料金の行を写していることを加える。`check_article.py` の `HEADER_RE` を、モデル名の行の後に料金の行（分かった場合と不明の場合）を任意で受け付ける形にする。スクリプトの先頭のコメント（`:26-28`）にも料金の行を加える。
- [x] **ステップ 3-3**: `internal/publisher` のテストを書く。
  - `file_test.go` の `TestRenderArticleCost`: 料金が分かった `Article`（手で計算した 4 桁の金額）と、不明になる `Article`（`RunUsage` のゼロ値、表にないモデル）で、見出しの語 `Estimated cost` を含む行の文字列を綴りで固定し、行が 1 回だけ現れること（AC-23・AC-25）。`TestFilePublisherWritesCostLine`: 料金が分かった `Article` で、`FilePublisher` が書いたファイルに同じ行があること（AC-23。不明の行のファイルは、ステップ 3-2 で更新する `file_test.go:64` のファイル全体の比較が確かめる）。
  - `slack_test.go` の `TestSlackPublishCostLine`: `httptest` の Webhook への投稿で、料金が分かった場合と不明の場合に行が 1 回だけ現れ、分割が起きる長さの記事でも最初のメッセージにだけ現れること（AC-24・AC-25）。
  - `slack_message_test.go` の `TestCostLineHasNoMention`: 分かった場合と不明の場合の `renderArticle` の結果から取り出した料金の行を `findMention` に渡して当たらないこと（AC-26）。手で書いた文字列は使わない。
- [x] **ステップ 3-4**: 壊して確かめる。`renderArticle` の料金の行を消すと `TestRenderArticleCost`・`TestSlackPublishCostLine` が失敗することを確かめる。`renderArticle` の行に、名前の文字が続く `@`（例: `@all`。`findMention` の M2 は `@` の後に名前の文字を求める。`slack_message.go:187-191`）を加えると `TestCostLineHasNoMention` が失敗することを確かめる。
- [x] **ステップ 3-5**: `check_article.py` を、料金の行が分かった場合と不明の場合の `--out` のファイル（`FilePublisher` が実際に書いたもの。例えばステップ 3-3 のテストの一時のファイルを写す）と、料金の行のない古い形のファイルで、一時のキャッシュのディレクトリ（`load_info` が読む `<id>.current`（`a` か `b`）と `<id>.<slot>/<id>.info.json`。`check_article.py:44-50`）に対して `python3 -I` で実行し、どれも `unrecognized --out format` にならないことを確かめる。実行したコマンドと出力を PR の説明に書く（manual）。

### PR-3 作成ポイント: publisher cost line

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `feat(0022): add the estimated cost line to the --out and Slack article header`

**レビュー観点**: `renderArticle` の料金の行が `pricing.Totals` の記事 1 本の料金の 4 桁と `USD`、不明では `unknown` だけで、値（0 を含む）を推測しないこと / 行が見出しのメタデータに 1 回だけ現れ、Slack の分割でも最初のメッセージにだけ出て、メンションの規則に当たらないこと / `file_test.go:64`・`TestRenderArticleSteps`・`integrationHeader`・`check_article.py` の追従が見出しの形と一致すること

**実装モデル要件**: standard

**判定理由**: `renderArticle` への 1 行の追加と既存の見出しの固定の追従であり、書式は architecture §3.7 に確定している。前例のない設計判断・panel-mode のトリガー・競合する実装案・分離された高リスクのステップのいずれにも当たらない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

- [x] **ステップ 3-6**: `cmd/yt2column/run.go` に、architecture §3.6 の書式の定数、列挙値を語に対応させる関数（`default` は `invalid`）、段階の行と合計の行を作る関数、`reportUsage` を置く。`reportUsage` は既存の `reportTokens` を呼んでから料金の行と合計の行を書き、`reportSuccess` と `reportRunError` の `reportTokens` の呼び出しを `reportUsage` の呼び出しに置き換える。`at=` は `artifact.CallTimeLayout` で書く。`reportSuccess` の文書（`:685-690`）に料金の行と合計の行を加える。
- [x] **ステップ 3-7**: `cmd/yt2column/cost_test.go` に、fake の `LLMClient`・`TranscriptSource`・`Publisher` と `t.TempDir`（`newRunEnv`・`usageSequence`・`usageResponse`・`useReviewClient` を再利用する。保存してから再開するテストは、`saveArtifactsOnce` が setup の後に検証・推敲の client を上書きするため、レビューの段階に表にあるモデルと既知の使用量を与えられるよう、同じ形の `saveCostArtifacts` をこのファイルに置く）と、`e.d.newWriter` を包んで `Options.Clock` を設定する固定の時計で CLI のテストを書く。段階ごとに、表にあるモデル名と異なるトークン数を返す。行は `redact.Line` を通った後の標準エラーの行のバイト列として比べる（AC-36・AC-37）。
  - `TestRunCostLinesAllStages`: 3 つの段階が走る成功した実行で、段階の行が 3 行、合計の行が 2 行、トークン数の行の直後にこの順に出て、金額が手で計算した 6 桁の値で、2 つの合計が等しいこと。同じ実行の `--out` のファイルの料金の行が、記事 1 本の料金を 4 桁に四捨五入した値であること（AC-12・AC-15・AC-23）。
  - `TestRunCostLinesRateBasis`: `deepseek-flash` のピーク・オフピークの時刻、Claude のモデルの `flat`・`short-prompt`・`long-prompt` が、適用した単価の位置の語に出ること（AC-16）。
  - `TestRunCostLinesUnknown`: 3 つの理由（不明の使用量・モデル名なし・表にないモデル名）の段階の行が `unknown reason=` と理由の語を出して数値を出さず、2 つの合計が不明で、記事が公開されること（AC-09・AC-11・AC-17）。
  - `TestRunCostLinesResume`: 既存の `TestRunTokenLinesResume`（`token_test.go:360`）と同じく、`saveCostArtifacts` で保存してから再開する 2 回の実行のどちらにも、固定の時計・`withSteps`・検証・推敲の client を設定する。`--after generation`・`shorten`・`review` の各再開で、再利用した段階の行が `reused` で、「この実行の料金」と「記事 1 本の料金」が手で計算した値であること。`--after review` の「この実行の料金」が `usd=0.000000` であること（AC-11・AC-13・AC-18）。再利用した段階だけが不明のとき「この実行の料金」は分かり「記事 1 本の料金」は不明であることは、`TestRunCostLinesResumeUnknownStage` で確かめる。
  - `TestRunCostLinesResumeUsesSavedTime`: `TestRunCostLinesResume` と同じ 2 回の実行の形で、ピーク時間帯の時計で保存した中間生成物を、オフピーク時間帯の時計で `--after` で再開し、再利用した `deepseek-flash` の段階が `peak` と保存時の `at=` で算出されること。逆も確かめる（AC-27・AC-28）。
  - `TestRunCostLinesSkippedStages`: 字数の調整が LLM を呼ばない実行と `--no-review` の実行で、その段階の行が出ず、合計がほかの段階の和であること（AC-14）。
  - `TestRunCostLinesFailure`: 次の失敗で、料金の行と合計の行が失敗の行の前に出て、終了コードが本タスクの前と同じであること（AC-19）。(1) 0019 の AC-16〜AC-19 の各場合に当たる、`token_test.go` の `TestRunTokenLinesReviewFailure`・`RejectedResponse`・`NoUsage`・`AfterAllCalls` と同じ失敗の経路（検証・推敲の呼び出しの失敗、生成の打ち切り・想定外の終了理由・空の応答で使用量を返す場合、使用量のない失敗（`usage-unknown` の行）、すべての呼び出しの後の字数の上限・中間生成物の保存・公開の失敗）。(2) `--after` を受理した後の失敗（再利用した段階の行が出る）。
  - `TestRunCostLinesReviewLogFailure`: 検証・推敲の一覧のファイルの書き込みが失敗した実行で、行が 1 回だけ出ること。
  - `TestRunCostLinesBeforeLLM`: トークン数の行を出さない実行（`TestRunTokenLinesBeforeLLM` と同じ、使い方の誤り・設定の誤り・字幕の取得の失敗・`--after` の読み込みの失敗）で、料金の行も合計の行も出ないこと（AC-20）。
  - `TestRunCostLinesGoldenFormat`: 分かった段階・不明の段階・2 つの合計の行の綴りを、定数を使わずに文字列で固定する（AC-21）。
  - `TestCostWords`: `RateBasis` と `UnknownReason` のゼロ値と列挙の外の値が `invalid` の語になること（architecture §3.6）。
  - `TestRunCostLinesOmitModel`: モデル名の規則を通る、表にない目印のモデル名（`Estimate` に届く）を返す実行で、目印がトークン数の行には現れ、料金の行と合計の行には現れないこと。秘密情報の値を含むモデル名の実行で、標準エラーに秘密情報が現れないこと（architecture §7.3、requirements §4.2。公開する記事の `- Model:` の行は本タスクの前から応答のモデル名をそのまま載せる既存の出力で、確認は CLI 自身の行に限る）。
- [x] **ステップ 3-8**: 壊して確かめる。`reportRunError` の `reportUsage` の呼び出しを `reportTokens` に戻すと `TestRunCostLinesFailure` が、「記事 1 本の料金」の行に「この実行の料金」の値を出すと `TestRunCostLinesResume` の `--after` の行が、語の対応の `default` を `flat` にすると `TestCostWords` が失敗することを確かめる。

### PR-4 作成ポイント: CLI cost lines

**対象ステップ**: 3-6 / 3-7 / 3-8

**推奨タイトル**: `feat(0022): print per-stage cost and total lines in the CLI summary`

**レビュー観点**: 料金の行と合計の行が、トークン数の行の直後に固定の位置の固定の語で出て、モデル名と応答の値を含まないこと / 成功・失敗・検証・推敲の一覧のファイルの失敗・`--after` の各経路で、行の有無・順序・1 回だけの出力が 0019 のトークン数の行と同じ規則に従うこと / `--after` の再利用した段階が中間生成物の時刻で算出され、`--after review` の「この実行の料金」が `usd=0.000000` であること / 列挙の外の値とゼロ値が `invalid` になること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-6 が、成功・失敗・一覧のファイルの失敗・再開の各経路での行の順序と有無を決める、分離された込み入ったステップであるため（ステップ 3-7 は Conditional checks の「CLI を複数回起動する」にも当たる。移行や外部リソースの面は持たないので `frontier-required` ではない）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

- [x] **ステップ 3-9**: `internal/pricing/testutil/helpers.go`（`package pricingtestutil`、`//go:build test || integration`）に、段階の料金を、金額（`Format(6)`）と適用した単価の種類、または不明の理由を architecture §3.6 の語で 1 行にする関数を置き、`helpers_test.go` の `TestCostSummary` で各状態の文字列を確かめる。語は CLI の行と同じ綴りにするが、ログは機械可読の契約にしない（architecture §3.8）ので、2 つを結ぶ guard は置かない。統合テストのログの語がずれても、利用者の出力には影響しない。
  - `package_reference.md` にこのパッケージの行を加える（`TestPackageReferenceListsPackages`）。`TestFakesCarryBuildTag` が固定する `testutil` のファイルの数（`internal/pipeline/pipeline_test.go:617` の 19）を、加えた 2 つのファイルの分だけ 21 に改める。
  - 両アダプタの `integration_test.go` の `generate` と `truncated` のサブテストで、`Generate` が戻った直後の時刻と応答から `pricing.Estimate` を求め、4 区分のトークン数とこの関数の結果を `t.Logf` で出す。`truncated` は応答を受け取る形にし、モデル名をログに出す前に、`generate` と同じく応答のモデル名（DeepSeek ではモデルの版も、Claude では設定したワークスペース ID の値も）が API キー（とワークスペース ID）を含まないことを確かめる。`at=` に使う時刻は `Generate` が戻った直後に取り、その値を `Estimate` に渡す。DeepSeek では、その確認が使うキーの取り出しを両方のサブテストから使える位置に移す。モデル名は既存の `t.Logf` と同じく `%+q` で出す。
- [x] **ステップ 3-10**: `cmd/yt2column/integration_test.go` の `runIntegrationCLI` と `integration_slack_test.go` の成功の経路で、既存の秘密情報の検査（`requireNoSecrets`）の後、実行の成否を判定する `checkRun` の前に（失敗した実行でも行が残るように）、標準エラーのトークン数の行・料金の行・合計の行を `t.Log` で出す（architecture §3.8。M-1・M-3 で照らすため）。

**完了条件:** 各 PR（PR-3・PR-4・PR-5）でグリーンゲートが通る（PR-5 では `make lint` の `go vet -tags integration` が統合テストの変更と `pricingtestutil` をコンパイルする）。既存の `token_test.go` と `internal/server`・`cmd/yt2column-server` のテストが変えずに通る（AC-22・AC-38）。

### 手動の確認（人の承認を得て行う）

実際の API と Webhook を呼ぶ確認であり、実行には人の承認が要る（CLAUDE.md「Tool Execution Safety」、requirements §5）。PR-5 の変更（統合テストのログ）を実際に動かす唯一の確認なので、PR-5 のマージの前に行い、結果を本節の表に記録して PR-5 に含める（AC-32・AC-34）。

- [ ] **ステップ M-1**: `make test-integration-cli` を実行し、ログの料金の行と合計の行の値を、DeepSeek のコンソールの利用履歴の同じ実行の料金と照らす（祝日に行った呼び出しは除く。AC-32）。
- [ ] **ステップ M-2**: `make test-integration-deepseek` と `make test-integration-claude` を実行し、ログに各呼び出しのトークン数と料金の概算が出ることを確かめる（AC-33）。
- [ ] **ステップ M-3**: `make test-integration-cli-slack` を実行し、Mattermost の投稿の見出しのメタデータに料金の行が表示されることを確かめる（AC-34）。

| 確認 | 日時 | 結果 |
|---|---|---|
| M-1（AC-32） | - | - |
| M-2（AC-33） | - | - |
| M-3（AC-34） | - | - |

### PR-5 作成ポイント: integration test cost logs

**対象ステップ**: 3-9 / 3-10 / M-1 / M-2 / M-3

**推奨タイトル**: `test(0022): log token counts and cost estimates in the billed integration tests`

**レビュー観点**: `truncated` のサブテストでも、モデル名をログに出す前に API キーを含まないことを確かめ、`errors.Is(err, llm.ErrTruncated)` の確認を保ったまま、エラーとともに返る応答の使用量で料金を出すこと / CLI の統合テストが `requireNoSecrets` の後、`checkRun` の前にだけ行をログに出すこと / `pricingtestutil` が `//go:build test || integration` を持ち、`TestFakesCarryBuildTag` の数と `package_reference.md` の行が更新されていること / M-1〜M-3 の日時と結果が §2 の表に記録されていること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-9 が、課金を伴う統合テストに API キーの漏れを防ぐ検査を加えて `truncated` のサブテストの形を変える、分離された高リスクのステップであり、グリーンゲートはそのコードをコンパイルするだけで実行しない（実行は人の承認を得た M-1〜M-3 だけ）ため。実際の API を呼ぶテストの数や CI の面は増やさないので `frontier-required` ではない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 文書

**対象ファイル:** `README.md`、`docs/dev/developer_guide/pricing_update.md`（新規）、`internal/pricing/rates.go`（先頭のコメント）、`CLAUDE.md`、`docs/dev/project_overview.md`、`docs/dev/security.md`、`docs/dev/developer_guide/package_reference.md`、`cmd/yt2column/docs_test.go`

- [ ] **ステップ 4-1**: `README.md` の `## Token usage` の後に、料金の概算の節（例: `## Estimated cost`）を加える。内容は architecture §3.9 の README の項目のとおりで、版 3 の記述はステップ 2-8 で済んでいる。例の行は、ステップ 3-7 の `TestRunCostLinesGoldenFormat` が固定する綴りと同じ形にする。
- [ ] **ステップ 4-2**: `docs/dev/developer_guide/pricing_update.md`（英語）に、architecture §3.9 の単価の更新の手順を書く。`internal/pricing/rates.go` の先頭のコメントからこの文書を指し、`CLAUDE.md` の Development Guides の一覧に 1 行加える（AC-41）。
- [ ] **ステップ 4-3**: `docs/dev/project_overview.md` の `--artifacts` の項（`:34`）に形式の版 3 と呼び出しの時刻を、トークン数の行の項（`:35`）の後に料金の行と `--out`・Slack の行を加え、「前提・制約」のモデル名の項（`:66`）に、単価の表がこの方針の対象外であることの注記を加える（architecture §1.3 の例外 3）。
- [ ] **ステップ 4-4**: `docs/dev/security.md` の `## 6.` の `--artifacts` の項（`:78`）に、来歴の列挙への呼び出しの時刻と、`call_time` の受理の形と拒否の規則を加える（architecture §3.9）。
- [ ] **ステップ 4-5**: `package_reference.md` の `internal/writer` の行に `CallTime`・`Clock`・`ErrMissingCallTime` を、`internal/publisher` の行に料金の行を、`cmd/yt2column` の行に料金の行を加える。
- [ ] **ステップ 4-6**: `cmd/yt2column/docs_test.go` に、既存の `TestTokenUsageDocsContract` と同じ形の `TestCostDocsContract` を加え、`readDoc`・`sectionByHeading`・`bulletContaining`・`containsToken` を使って次を確かめる。`TestTokenUsageDocsContract` の `package_reference.md` の表に `internal/pricing` の行（公開の型）を加える（architecture §3.11）。
  - README の料金の節が、`run.go` の料金の書式の定数と、語の対応の関数が返すすべての語（適用した単価・不明の理由・`invalid`）を含むこと。語はコードから得て、テストに綴りを書かない。README の例の行の固定の位置の語が、定数と語の対応の関数の語だけであること。`--out`・Slack の行の見出しの語 `Estimated cost` を含むこと（この語は `internal/publisher` の非公開の定数なので、guard は綴りを書き、同じ綴りを `TestRenderArticleCost` が固定する）（AC-21・AC-40）。
  - `pricing_update.md` が `rates.go` と、手で計算した期待値のテストの名前を含み、`CLAUDE.md` がこの文書へのリンクを持ち、`rates.go` の先頭のコメントがこの文書を指すこと（AC-41）。
  - `project_overview.md` が料金の行の接頭辞（`run.go` の定数）と `call_time` を、`security.md` の `--artifacts` の項が `call_time` を含むこと（AC-42。日本語の文に当たる文字列は Go のリテラルに書かない）。
- [ ] **ステップ 4-7**: 書いた文書を、その根拠となるコード・テスト・architecture と照らす。README の行の書式と例は `TestCostDocsContract` と `TestRunCostLinesGoldenFormat` の出力で、README の DeepSeek の判定・合計の丸め・再利用した段階の単価の説明は architecture §3.3・§3.6・§5.2 とステップ 1-5 のテストで、`pricing_update.md` の手順は `rates.go` とステップ 1-5 のテストのファイルの名前で、`security.md` の規則は `TestParseRecordRejectsCallTime` の行と、PR の差分で突き合わせる（manual）。README の DeepSeek の判定の記述（祝日、境界、時計）が AC-40 の項目を満たすことも確かめる。

**完了条件:** グリーンゲートが通る。`TestCostDocsContract`・`TestTokenUsageDocsContract`・`TestPackageReferenceListsPackages` が通る。

### PR-6 作成ポイント: documentation

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7

**推奨タイトル**: `docs(0022): document the cost estimate and the rate update procedure`

**レビュー観点**: README の料金の節が `run.go` の書式の定数と語の対応の関数のすべての語を含み（`TestCostDocsContract`）、例の行が `TestRunCostLinesGoldenFormat` の綴りと一致すること / DeepSeek のピーク時間帯の判定の限界（祝日・境界・時計）と、合計の丸め・再利用した段階の単価・請求額はコンソールで確かめることが README にあること / `pricing_update.md` の手順が `rates.go` と手で計算した期待値のテストを指し、`CLAUDE.md` からリンクされていること / `project_overview.md`・`security.md`・`package_reference.md` の記述が、それぞれが述べるコードとテストに一致すること

**実装モデル要件**: standard

**判定理由**: 文書の追加と、その内容を確かめる guard のテストである。前例のない設計判断・panel-mode のトリガー・競合する実装案・分離された高リスクのステップのいずれにも当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | このフェーズで作るもの | フェーズ |
|---|---|---|
| M1: 料金を算出できる | `internal/pricing`（単価の表・`validate`・`Amount`・`Estimate`・`Totals`）と、`package_reference.md` の行 | フェーズ 1 |
| M2: 時刻が段階と中間生成物に載る | `writer` の `CallTime`・`Options.Clock`・`ErrMissingCallTime`、中間生成物の版 3 と `call_time`、版の記述 | フェーズ 2 |
| M3: 利用者に料金が見える | `renderArticle` の料金の行、CLI の料金の行と合計の行、`pricingtestutil` と統合テストのログ、`check_article.py` | フェーズ 3 |
| M4: 文書が揃う | README・`pricing_update.md`・`CLAUDE.md`・`project_overview.md`・`security.md`・`package_reference.md` と `TestCostDocsContract` | フェーズ 4 |

- フェーズの名前と順序は architecture §8 の手順 1〜4 と同じである。フェーズ 1 の `StageUsage.CallTime` のフィールド（ステップ 1-3）と `package_reference.md` の行（ステップ 1-4）、フェーズ 2 の版の記述（ステップ 2-8）は、それを含む PR のコンパイル・グリーンゲート・文書の正しさのために、その PR の中で行う前提の作業である。
- フェーズ 3 はフェーズ 1 の `pricing` とフェーズ 2 の `CallTime` を、フェーズ 4 はフェーズ 3 の書式の定数と語の対応の関数を使う。フェーズ 2 は、フェーズ 1 のステップ 1-3 で加えた `StageUsage.CallTime` のフィールドに値を入れる（architecture §8）。
- 各 PR は単独でグリーンゲートを通せる単位とする。手動の確認（M-1〜M-3）は PR-5 のマージの前に、人の承認を得て行う。PR の境界と実装モデル要件は §3.2 と各 `### PR-N 作成ポイント` のとおりである。

### 3.2 PR 構成

PR はフェーズ 1・2・4 をそれぞれ 1 つとし、フェーズ 3 を `internal/publisher`（PR-3）・`cmd/yt2column` の要約（PR-4）・統合テストのログ（PR-5）の 3 つに分ける（PR は 6 つ）。フェーズ 2 は 1 つの PR に残す。`writer` の時刻の記録を先に、`internal/artifact` の版 3 と時刻のない記録の拒否を後にすれば 2 つの PR に分けられるが、そのためにはステップ 2-1・2-2・2-5・2-9 を分割する必要があり、architecture §8 も 2 つを同じフェーズに置いている。フェーズ 3 は、`internal/` の出力（PR-3）を `cmd/` の要約（PR-4）より先に着地させ、課金を伴う統合テストのログ（PR-5）を本番の出力から切り離してレビューできるようにする。そのため、旧ステップ 3-8（壊して確かめる）を `internal/publisher` の部分（ステップ 3-4）と CLI の部分（ステップ 3-8）に分け、`check_article.py` の確認（ステップ 3-5）を `renderArticle` の変更と同じ PR に移し、フェーズ 3 のステップを番号順に振り直した。手動の確認（M-1〜M-3）は PR-5 の変更を実際に動かす唯一の確認なので、PR-5 のマージの前に人の承認を得て行い、結果を PR-5 で記録する（手動の確認の節は、そのためにフェーズ 3 の最後へ移した）。

PR は PR-1 → PR-2 → PR-3 → PR-4 → PR-5 → PR-6 の順とし、各 PR は前の PR に依存する（PR-1 と PR-2 は `StageUsage.CallTime` のフィールドで、PR-3 は `pricing`、PR-4 は `artifact.CallTimeLayout` と PR-3 の `--out` の行、PR-5 は PR-4 の行の接頭辞、PR-6 は PR-4 の書式の定数と語の対応の関数を使う）。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 | `internal/pricing`（単価の表・`validate`・`Amount`・`Estimate`・`Totals`）とテスト、`StageUsage.CallTime` のフィールド、`package_reference.md` の `internal/pricing` の行 | standard |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9 | `writer` の `Options.Clock`・時刻の記録・`ErrMissingCallTime`、`internal/artifact` の版 3 と `call_time`、既存のテストの更新、版の数字を述べる文書 | frontier-required |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | `renderArticle` の料金の行、見出しの固定の追従（`file_test.go`・`integrationHeader`・`check_article.py`）、`internal/publisher` のテスト | standard |
| PR-4 | 3-6 / 3-7 / 3-8 | `cmd/yt2column/run.go` の料金の行・合計の行・`reportUsage`、`cost_test.go` | frontier-recommended |
| PR-5 | 3-9 / 3-10 / M-1 / M-2 / M-3 | `pricingtestutil`、両アダプタと CLI の統合テストのログ、`TestFakesCarryBuildTag` の数、`docs/tasks/0022_cost_estimate/03_implementation_plan.md` の手動の確認の記録 | frontier-recommended |
| PR-6 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 | README・`pricing_update.md`・`CLAUDE.md`・`project_overview.md`・`security.md`・`package_reference.md` と `TestCostDocsContract` | standard |

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

ユニットテストは実際の LLM の API・`yt-dlp`・Webhook を呼ばない（CLAUDE.md「Testing Strategy」）。`internal/pricing` のテストは、テストが与えた時刻と使用量だけを使い、実際の時計に依存しない（AC-36）。`internal/writer` のテストは `FakeLLMSequence`・`FakeLLMClient`・fake の `ArtifactStore` と固定の時計を、`internal/artifact` のテストはバイト列と `t.TempDir` の実ファイルを、`internal/publisher` のテストは `httptest` の Webhook と `t.TempDir` を、`cmd/yt2column` のテストは fake の `LLMClient`・`TranscriptSource`・`Publisher`・`t.TempDir` と固定の時計を使う（AC-37）。対象は architecture §7.1 のとおりで、AC との対応は §5 の表に記す。目標は、§5 の各 AC の行のテストが通ることと、`make test` が通ることである。

### 4.2. 統合テスト

- 実際の API を呼ぶ統合テストは、ステップ 3-9・3-10 でログを加えるだけで、本タスクの自動のテストとしては実行しない。手動の確認（M-1〜M-3）で、人の承認を得て実行する。
- localhost サーバの経路で料金の行が載ることは、同じ `renderArticle` を通る `internal/publisher` のテスト（ステップ 3-3）で確かめる。`internal/server`・`cmd/yt2column-server` の既存のテストは変えずに通す（AC-38）。

### 4.3. セキュリティテスト

architecture §7.3 の検査を、ステップ 1-5（`TestEstimateLargeCounts`。T3）、ステップ 2-6（`TestParseRecordRejectsCallTime` のメッセージに入力の値を含めない。T1）、ステップ 2-6（`TestRecordIDCoversCallTime`。T2）、ステップ 2-7（`call_time` が不正な中間生成物で LLM を呼ばず公開しない。T1）、ステップ 3-3（`TestCostLineHasNoMention`）、ステップ 3-7（`TestRunCostLinesOmitModel`。T4）、ステップ 3-9・3-10（統合テストのログは、API キーの検査の後でだけ出す。T4）に含める。

### 4.4. テストヘルパー

CLI の固定の時計の補助は `cost_test.go` の中に、`internal/writer` の固定の時計の補助は `calltime_test.go` の中に置く（どちらも同じパッケージの `_test.go` だけが使う）。新しい `testutil/` は、2 つのアダプタの統合テストが共有する `internal/pricing/testutil`（ステップ 3-9。`test_organization.md` の分類 A の `helpers.go` と `helpers_test.go`、`//go:build test || integration`）だけで、`TestFakesCarryBuildTag` の数える `testutil` のファイルが 2 つ増える（ステップ 3-9 で期待する数を改める）。`internal/pricing` の壊した表の補助は `rates_test.go` の中に置く（非公開の型を使うが、使うのは同じパッケージのテストだけなので `test_helpers.go` は要らない）。`internal/writer`・`internal/artifact` は既存の補助（§1.3）を更新して使う。

### 4.5. 既存のテストとの互換

requirements AC-39 が認める変更は、architecture §1.3「要件 AC-39 との関係」の一覧と、本計画のステップ 2-4・2-6・2-7・2-8・3-2・3-9・3-10・4-6 の更新（両アダプタと CLI の統合テストへのログの追加と `truncated` のサブテストの応答の受け取り、`sameRecord` への `CallTime` の比較の追加、`TestMarshalParseRoundTrip` と `TestRunArtifactExitCodes` への行の追記、`TestFakesCarryBuildTag` の `testutil` のファイルの数、`TestTokenUsageDocsContract` の版の文字列と `call_time` と `internal/pricing` の行、`check_article.py`）だけである。ほかの既存のテストは内容を変えずに `make test` で通す。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

「検証の種類」の `test` は、実行され、誤った振る舞いで失敗するテストを指す。`static` は、実行される guard・`make` ターゲットを指す。`manual` は、人が行う確認を指す。テストのパスは `path::TestName` の形で記す。表の新しいテストは、§2 の各ステップで加える。

| AC | 検証の種類 | 検証 |
|---|---|---|
| AC-01 | test | `internal/pricing/rates_test.go::TestRateTableModels`。各モデルの単価の値は `internal/pricing/pricing_test.go::TestEstimateCategories`・`::TestEstimatePeakWindow`・`::TestEstimatePromptTier` |
| AC-02 | static | `internal/pricing/rates_test.go::TestRateTableSourceComments` |
| AC-03 | test / static | `internal/pricing/rates_test.go::TestRateTableValid`・`::TestValidateRejects`（test）、`::TestRateFieldsUnsigned`（static。負の単価を書けない型） |
| AC-04 | test | `internal/pricing/pricing_test.go::TestEstimateUnknown`（大文字小文字・接尾辞・空白の行） |
| AC-05 | test | `internal/pricing/pricing_test.go::TestEstimateCategories` |
| AC-06 | test | `internal/pricing/pricing_test.go::TestEstimatePeakWindow` |
| AC-07 | test | `internal/pricing/pricing_test.go::TestEstimateIgnoresCallTime` |
| AC-07a | test | `internal/pricing/pricing_test.go::TestEstimatePromptTier` |
| AC-08 | test | `internal/pricing/pricing_test.go::TestEstimateLargeCounts`・`::TestEstimatePromptTier`（プロンプトの長さの和の桁あふれの行） |
| AC-09 | test | `internal/pricing/pricing_test.go::TestEstimateUnknown`、`cmd/yt2column/cost_test.go::TestRunCostLinesUnknown` |
| AC-10 | test | `internal/pricing/pricing_test.go::TestEstimateUnknown`（表にないモデル名の行） |
| AC-11 | test | `internal/pricing/pricing_test.go::TestTotals`、`cmd/yt2column/cost_test.go::TestRunCostLinesUnknown`・`::TestRunCostLinesResumeUnknownStage`（再利用した段階だけが不明の行） |
| AC-12 | test | `internal/pricing/pricing_test.go::TestTotals`、`cmd/yt2column/cost_test.go::TestRunCostLinesAllStages` |
| AC-13 | test | `internal/pricing/pricing_test.go::TestTotals`、`cmd/yt2column/cost_test.go::TestRunCostLinesResume` |
| AC-14 | test | `internal/pricing/pricing_test.go::TestTotals`（`OriginNone` の行）、`cmd/yt2column/cost_test.go::TestRunCostLinesSkippedStages` |
| AC-15 | test | `cmd/yt2column/cost_test.go::TestRunCostLinesAllStages` |
| AC-16 | test | `cmd/yt2column/cost_test.go::TestRunCostLinesRateBasis` |
| AC-17 | test | `cmd/yt2column/cost_test.go::TestRunCostLinesUnknown` |
| AC-18 | test | `cmd/yt2column/cost_test.go::TestRunCostLinesResume`・`::TestRunCostLinesResumeUsesSavedTime` |
| AC-19 | test | `cmd/yt2column/cost_test.go::TestRunCostLinesFailure`、`internal/writer/usage_test.go::TestWriteFailureRunUsage`（失敗した呼び出しの段階の `CallTime`） |
| AC-20 | test | `cmd/yt2column/cost_test.go::TestRunCostLinesBeforeLLM` |
| AC-21 | static / test | `cmd/yt2column/docs_test.go::TestCostDocsContract`（README と定数の一致）、`cmd/yt2column/cost_test.go::TestRunCostLinesGoldenFormat`（固定の位置の固定の語の綴り） |
| AC-22 | test | `cmd/yt2column/token_test.go` の既存のテスト（変えずに通る。`TestRunTokenLinesGoldenFormat` などがトークン数の行をバイト列で比べる）と `cmd/yt2column/run_test.go` の既存の要約のテスト。`make test` で実行する |
| AC-23 | test | `internal/publisher/file_test.go::TestRenderArticleCost`・`::TestFilePublisherWritesCostLine` |
| AC-24 | test | `internal/publisher/slack_test.go::TestSlackPublishCostLine` |
| AC-25 | test | `internal/publisher/file_test.go::TestRenderArticleCost`・`::TestFilePublisherWritesCostLine`（不明の行）、`internal/publisher/slack_test.go::TestSlackPublishCostLine`（不明の行）、`cmd/yt2column/cost_test.go::TestRunCostLinesUnknown`（公開と終了コード） |
| AC-26 | test | `internal/publisher/slack_message_test.go::TestCostLineHasNoMention`、`internal/publisher/slack_test.go::TestSlackPublishCostLine`（分割）。既存の分割・UTF-8 の検査のテストが変えずに通る |
| AC-27 | test | `internal/writer/calltime_test.go::TestWriteCallTimePerStage`、`cmd/yt2column/cost_test.go::TestRunCostLinesResumeUsesSavedTime`（保存した `at=` と判定の一致） |
| AC-28 | test | `internal/writer/usage_test.go::TestWriteRunUsageResume`（再利用した段階の `CallTime`）、`cmd/yt2column/cost_test.go::TestRunCostLinesResumeUsesSavedTime` |
| AC-29 | test | `internal/artifact/format_test.go::TestParseRecordRejectsCallTime`、`cmd/yt2column/run_test.go::TestRunArtifactExitCodes`（`call_time` が不正な中間生成物の行） |
| AC-30 | test | `internal/artifact/format_test.go::TestParseRejectsVersion2` |
| AC-31 | test | `internal/artifact/format_test.go::TestRecordIDCoversCallTime` |
| AC-32 | static / manual | `make lint`（`go vet -tags integration` がステップ 3-10 のログの変更をコンパイルする。static）、ステップ M-1（§2 の表に日時と結果を記録。manual） |
| AC-33 | test / static / manual | `internal/pricing/testutil/helpers_test.go::TestCostSummary`（ログの行の文字列。test）、`make lint`（`go vet -tags integration` がステップ 3-9 のログの変更をコンパイルする。static）、ステップ M-2（manual） |
| AC-34 | test / manual | `internal/publisher/slack_test.go::TestSlackPublishCostLine`（投稿の本文。test）、ステップ M-3（Mattermost での表示。manual） |
| AC-35 | test / manual | §5 の `internal/pricing` のテスト（test）。期待値が手で計算した値で、計算式がコメントにあることは、ステップ 1-5 の PR で確認する（manual） |
| AC-36 | test | `internal/pricing/pricing_test.go::TestEstimatePeakWindow`、`internal/writer/calltime_test.go::TestWriteCallTimePerStage`、`cmd/yt2column/cost_test.go::TestRunCostLinesResumeUsesSavedTime`（固定の時計） |
| AC-37 | test | `cmd/yt2column/cost_test.go` のすべてのテスト。fake の `LLMClient`・`TranscriptSource`・`Publisher` と `t.TempDir` を使う。`make test` で実行する |
| AC-38 | static | `make test` が `internal/server` と `cmd/yt2column-server` の既存のテストを変えずに通す |
| AC-39 | static / manual | `make test` が既存のテストを通す（static）。§4.5 の一覧の外の既存のテストを変えていないことを、各 PR の差分で確認する（manual） |
| AC-40 | static / manual | `cmd/yt2column/docs_test.go::TestCostDocsContract`（static）。記述の内容がコードと architecture と一致することは、ステップ 4-7 で確認する（manual） |
| AC-41 | static | `cmd/yt2column/docs_test.go::TestCostDocsContract`（`pricing_update.md`・`CLAUDE.md`・`rates.go` のコメント） |
| AC-42 | static | `cmd/yt2column/docs_test.go::TestCostDocsContract`（`project_overview.md`）・`::TestTokenUsageDocsContract`（`package_reference.md` の `internal/pricing`・`internal/artifact` の行）、`internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages` |

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| 出典のページの単価が architecture §3.2 の表と食い違っている | 算出の値が誤る | 表に写す直前にページと照らし、食い違えば architecture を直してから実装する（ステップ 1-1） |
| 期待値を表の定数から導き、表の誤りがテストを通る | AC-05 の誤りを検出できない | 期待値を出典の値から手で計算し、計算式をコメントに書く（ステップ 1-5、AC-35） |
| `Marshal` が UTC でない時刻を書き、9 時間ずれた正しい形の時刻が保存される | 再開した段階の時間帯の判定が誤る | `Marshal` が書く前に UTC・ナノ秒 0・ゼロでないことを検査する（ステップ 2-3）。検査を外して失敗することを確かめる（ステップ 2-9） |
| `Parse` が `call_time` を版の検査より前に読み、版 2 のファイルを時刻の欠けとして拒否する | AC-30 の版の違いと回復の手順のメッセージが出ない | `parseStage` の後で読み（ステップ 2-3）、順序を入れ替えて `TestParseRejectsVersion2` が失敗することを確かめる（ステップ 2-9） |
| `sameRecord` に時刻の比較がなく、往復のテストが時刻の欠落を見逃す | AC-27 の保存の誤りを検出できない | `sameRecord` に `CallTime` を加える（ステップ 2-4） |
| `RunUsage` を `!=` で比べるテストが、nil の時計の `time.Now` の時刻で不安定に失敗する | `make test` が失敗する | 該当するテスト（§1.3 の 8 か所）に固定の時計を入れる（ステップ 2-4） |
| 料金の行で見出しが長くなり、これまで 1 つのメッセージに収まった記事が 2 つに分かれる | Slack の分割のテストの期待が変わる | 分割のテストは長さを `renderArticle` から求めるので追従する（architecture §3.7）。`integrationHeader` を同じ変更で直す（ステップ 3-2） |
| `check_article.py` が料金の行のある `--out` のファイルを読めない | 0018 の評価の機械的な判定ができない | `HEADER_RE` を改め（ステップ 3-2）、古い形と新しい形のファイルで実行して確かめる（ステップ 3-5） |
| 単価の表のリテラルの数値が `mnd`（magic number）や `gosec` に指摘される | `make lint` が通らない | 表は宣言した定数であり、指摘を受けたときは、該当するエントリの行に限って `//nolint:<linter>` を根拠のコメント付きで置く（ファイル全体の抑制はしない） |
| `Estimate` の分岐と CLI の語の対応が `gocyclo`（閾値 20）に指摘される | `make lint` が通らない | 単価の組の選び方や語の対応を、小さな非公開の関数に分ける |
| DeepSeek のコンソールの利用履歴が、1 回の実行の料金を分けて示さない（日ごと・モデルごとの合計だけを示す） | M-1 で 1 回の実行の概算と照らせない | M-1 の前にコンソールの粒度を確かめ、ほかの呼び出しのない時間帯に実行するなど、実行を切り分ける方法を M-1 の記録に書く |

### 6.2. スケジュールのリスク

- 手動の確認（M-1〜M-3）は人の承認と実際の API・Webhook を要する。PR-1〜PR-4 は手動の確認を待たない。PR-5 は、マージの前に手動の確認を行うので、人の承認を得られるまでマージを待つ。その間も PR-6（文書）の作業は進められる。

### 6.3. セキュリティリスク

- 料金の行・合計の行・`--out`・Slack の行は、金額の数字と固定の語だけで作り、モデル名・応答の値・秘密情報を含めない（ステップ 3-1・3-6）。標準エラーの行は `stderrWriter.line` を通す。
- 中間生成物の `call_time` は信頼できない入力として境界で検査し、補正せず、部分受理しない（ステップ 2-3・2-6）。時刻の書き換えは `id` の照合で拒否する（AC-31）。`id` を計算し直したファイルを検出できないことは、architecture §5.1 の T2 の残余のリスクのとおりである。

## 7. 実装チェックリスト (Implementation Checklist)

- [x] architecture の記述の漏れと古い行番号を editorial correction として直した（§2.0）
- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6）
- [x] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9）
- [x] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5）
- [x] PR-4 マージ済み（対象ステップ: 3-6 / 3-7 / 3-8）
- [ ] PR-5 マージ済み（対象ステップ: 3-9 / 3-10 / M-1 / M-2 / M-3）
- [ ] PR-6 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7）
- [ ] §5 のすべての AC の `test`・`static` の検証が実行され、通る
- [ ] `make fmt`・`make test`・`make lint` がすべての PR の完了時に通る

## 8. 成功基準 (Success Criteria)

- **機能:** 4 つのモデルの単価の表があり、段階の料金を手で計算した値どおりに算出し、ピーク時間帯とプロンプトの長さで単価を選ぶ（AC-01〜AC-08）。不明を 3 つの理由で区別し、合計に推測の値を出さない（AC-09〜AC-14）。CLI が成功と失敗の要約に固定の書式の料金の行と合計の行を出す（AC-15〜AC-22）。`--out` と Slack に記事 1 本の料金の行を 1 回だけ出す（AC-23〜AC-26）。中間生成物が呼び出しの時刻を来歴に持ち、版 2 と不正な時刻を拒否する（AC-27〜AC-31）。
- **品質:** `make test`・`make lint` が通る。§4.5 の一覧の外の既存のテストを変えていない（AC-39）。サーバの振る舞いを変えていない（AC-38）。各テストが、対象の仕組みを壊して失敗することを確かめ、コミットメッセージに書いた。
- **セキュリティ:** 料金の行・投稿の行・中間生成物の拒否のメッセージが、モデル名・応答とファイルの値・秘密情報を含まない（requirements §4.2）。
- **手動の確認:** 統合テストのログの料金がコンソールの利用履歴と一致し、Mattermost で料金の行が表示される（AC-32〜AC-34）。
- **文書:** README・`pricing_update.md`・`CLAUDE.md`・`project_overview.md`・`security.md`・`package_reference.md` を更新し、`TestCostDocsContract` で確かめる（AC-40〜AC-42）。

## 9. 次のステップ (Next Steps)

- 本書の PR の境界と実装モデル要件（§3.2 と各 `### PR-N 作成ポイント`）に従い、`/runplan 0022` で PR-1 から順に実装する。
- 実装中に architecture の記述が実際のコードや出典のページと合わない箇所が見つかった場合は、本計画を独自に変えず、`requirements_process.md` の「Editing an approved document」に従って architecture を直す。
- 本タスクの完了後に、0018（#248）が基準の集合を版 3 の CLI で作る（requirements §5）。
