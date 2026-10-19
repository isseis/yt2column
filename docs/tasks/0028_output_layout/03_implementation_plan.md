# 実装計画書：出力の構成の見直し

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-19 |
| Review date | 2026-10-19 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）が定める出力の構成の見直しを、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。具体的には次のことを行う。

-   チャンネル名を `Transcript` から `Article` まで運び、入口・LLM を呼ぶ前・公開の直前の 3 か所で同じ関数 `CheckChannelName` により検査する（F-001、設計書 3.1）。
-   `--out` のファイルを、YAML のフロントマターで始まる形にする（F-003、設計書 3.4）。
-   Slack の投稿を、見出し行・本文・脚注の並びにし、分割とメンションの扱いを改める（F-002、設計書 3.3・3.5・3.7）。
-   出力の形を固定しているテストと文書を新しい構成に合わせる（F-004、設計書 3.6）。

本計画が使う記号は次のとおりである。M1〜M4 は `rejectMention` の拒否規則、V1〜V3 は検査用の文字列を作る変換、R-N は設計書 10 章と付録 B の項目を指す（設計書 3.7）。AC-NN・F-NNN は要件書、H-NN は [design_handoff.md](design_handoff.md)、I-NN は [implementation_handoff.md](implementation_handoff.md) の項目を指す。「設計書 N」は設計書の節を、「§N」は本計画の節を指す。

フェーズは設計書 8 章の実装優先順位 1〜5 と 1 対 1 に対応させる（フェーズ N = 優先順位 N）。PR の境界は §3.2 と各 `### PR-N 作成ポイント` に定めた。

### 1.2. 実装原則

-   設計書 1.1 の設計原則に従う。各 PR は、終わりで `make fmt` → `make test` → `make lint` が通る状態にする。旧い形と新しい形が混ざった状態は、PR-3〜PR-6 の途中のマージ（各ステップの「完了」に記した状態）を除き、フェーズの終わりに残さない（設計書 8 章）。
-   新しい依存パッケージを加えない（YAML のライブラリを含む。要件書 §5）。
-   加える・改めるテストは、対象を無効にして（呼び出しを外す、規則を緩める、削除した分岐を戻す）失敗することを確かめ、その結果をコミットメッセージに記す（CLAUDE.md「Every test must be able to fail for its stated reason」）。各フェーズの最後のステップがこれを行う。
-   Go のコメント・識別子・文字列リテラルは、テストのコードとテストの補助ファイルを含めて英語で書く。文書のテストが日本語の見出しや語句と照合するリテラルだけは日本語を使う（CLAUDE.md「Go Idioms」）。
-   テストの表の行、期待する文字列、エラーの文言は、コードを書くときに決める。本計画は、何を確かめるかとどこに置くかだけを定める。
-   Go のファイルを編集したら `make fmt` を実行する。最適化は入れない（CLAUDE.md「Performance」）。

### 1.3. 既存コード調査結果

本節の `file:line` は HEAD `6506807` で読んだ。設計書が引く行番号とずれていたものは、本節の値を使う。

**チャンネル名の経路（フェーズ 1）**

-   `Article`（`internal/writer/writer.go:18`）は `ChannelName` を持たない。`newArticle`（`internal/writer/output.go:181`）の呼び出し元は `writer.go:405` の 1 か所だけである。`Write`（`writer.go:324`）は生成の実行でも `--after` の再開でも `t transcript.Transcript` を受け取る。
-   `displayStringProblem`（`output.go:166`）の呼び出し元は `checkModelString`（`output.go:147`）、`checkGeneratedText`（`output.go:44`）、`Article.CheckPublishable`（`article.go:15`）、`checkStepModel`（`article.go:58`）で、`ReviewResult.Check`（`revision.go:27`）も使う。`checkModelString` は先に `utf8.ValidString` を呼ぶ（`output.go:150`）。
-   `validateTranscript`（`prompt.go:50`）は `ChannelName` を、`Title`・`Description` と一緒に UTF-8 の検査の表に入れている（`prompt.go:66-70`）。`newTemplateData` は `ChannelName` を `templateData` に渡す（`prompt.go:94`）。
-   `server.parseRequest`（`internal/server/request.go`）は `optionalMember(members, "channelName")` を `request.go:76` で読み、検査せずに `CopiedInput` へ渡す（`request.go:93`）。拒否は `reasonInvalidBody` である。
-   `internal/server/job.go:143`（`classifyWrite`、`:133` から）は `ErrInvalidTranscript` を `KindInternal` に写す。このため、入口の検査を先に置く必要がある（設計書 3.1）。
-   `writer/test_helpers.go:57` は、すでに `ChannelName: markChannel` を持つ。`prompt_test.go:38` の `TestWriteRejectsInvalidTranscript` は、`validateTranscript` の拒否の表を持つ。

**ファイルの出力（フェーズ 2・3）**

-   `renderArticle`（`internal/publisher/file.go:211`）は、`file.go:117`（`FilePublisher`）と `slack_message.go:42`（`prepareSlackMessages`）の本番の呼び出し元 2 つを持つ。したがって、フェーズ 3 で `FilePublisher` を `renderFileArticle` に切り替えても、`renderArticle` はフェーズ 4 の終わりまで Slack が使い続ける。削除はフェーズ 4 で行う。
-   `articleCostLine`（`file.go:229`）・`modelVersionOrNone`（`file.go:241`）・`articleCostDecimals`（`file.go:39`）の参照元は、`file.go` のほかに `reviewlog.go:77`（`modelVersionOrNone`）だけである。
-   `internal/publisher` で `pricing.Totals`（`internal/pricing`）を呼ぶのは `articleCostLine` だけである（`file.go:230`）。`cmd/yt2column/run.go:752` も呼ぶが、本タスクは変えない。

**Slack の投稿（フェーズ 4）**

-   `prepareSlackMessages`（`slack_message.go:25`）は、`CheckPublishable` の後に、`Title`・`Body`・モデル名・モデルの版の UTF-8 を検査し（`:29-41`）、`renderArticle` → `splitSlackText` → `rejectMention` の順に進める。`SlackMessageCount`（`:59`）は `prepareSlackMessages` を呼ぶ。`splitSlackText`（`:75`）の呼び出し元は `prepareSlackMessages` だけである。
-   `findInspectionMention`（`:186`）の M2 の判定（`ins[i] == '@' && isMentionNameRune(ins[i+1])`）は、`@` の直前を見ない。`isMentionNameRune`（`:306`）は `_`・文字・数字だけを名前の文字とする。設計書 3.7 は `.`・`-` も名前の文字とするので、`mentionAts` 用の判定を別に持つ（`isMentionNameRune` の本番の呼び出し元は `slack_message.go:191` の M2 だけである。置き換えの後は不要になる）。
-   `inspectMentionText`（`:265`）は V2 → V3 → V1 の順で検査用の文字列を作る。I-05 は M2 には V3 だけを使うと定めるので、`mentionAts` は `inspectMentionText` の結果ではなく、`replaceLookAlike`（`:293`）を使う。
-   `markdown.go` に `proseLines`（`:74`）・`codeSpans`（`:198`）・`codeSpansExempt`（`:157`）・`bodyLine`（`:31`）・`span`（`:43`）がある。`ProseRanges` と `ByteRange` はない。
-   `failure.Kind` は `KindPublish` の次に `KindInternal` を持つ（`failure.go:25-26`）。`classify`（`job.go:109`）は `StagePublish` を `KindPublish` に写す（`job.go:122`）。`slack_failure.go:113` が `KindPublish` の文面を持つ。
-   `cmd/yt2column/run.go:1031` が `ErrSlackMention` の案内を出す。`run_test.go:1489` がその文面を確かめる。
-   `publishertestutil.IntegrationArticles`（`internal/publisher/testutil/integration.go:145`）は、`integrationHeader`（`:169`）で旧い見出しの長さを数え、投稿全体がちょうど 16,383 コードポイントになるように本文を切る。新しい投稿の見出し行と脚注を数えるように改めないと、`TestIntegrationArticles`（`slack_articles_test.go`）が失敗する。

**出力の形を固定しているテストと文書（設計書 3.6 の表を HEAD で確かめた）**

-   `renderArticle` を使うテスト: `file_test.go`（`TestRenderArticleSteps:141`・`TestRenderArticleCost:179`・`TestFilePublisherWritesArticle:76`・`TestFilePublisherWritesCostLine:108`・`TestFilePublisherEmptyModelVersion:127`）、`slack_test.go`（`TestSlackPublishPayload:456`・`TestSlackPublishCostLine:513`・`TestSlackPublishSplit:559`・`TestSlackMessagesForTest:642`）、`slack_message_test.go`（`TestSlackSplit:81`・`TestCostLineHasNoMention:300`・`TestSlackMessageCount:398` ほか。ファイル内の `renderArticle(` の全件）。
-   メンションの旧い期待値: `slack_message_test.go` の `TestSlackMentionRejected:235`・`TestSlackMentionAccepted:273`・`TestSlackMentionPosition:428`・`TestSlackMentionMessagesFollowPosted:460`。
-   失敗の種類: `failure_test.go` の `TestKindConstants:65`、`job_test.go` の `TestFailureClassification:160`、`slack_failure_test.go` の `TestSlackFailureNotifyText:115`。
-   `cmd/yt2column`: `run_test.go:1489`（`TestRunSlackFailureHints:1449` の中の `mention` の行）、`run_test.go:1622`・`:2150`、`cost_test.go:165`、`integration_test.go`（`checkIntegrationArticle:236`。ビルドタグ `integration`）。
-   文書: `README.md`（Estimated cost の節 `:189`・`:241-245`、M2 の説明 `:317-319`）、`docs/dev/project_overview.md:36`、`docs/dev/security.md:74`、`docs/dev/developer_guide/package_reference.md:37`（`renderArticle` の説明）、`cmd/yt2column/docs_test.go:795`（`TestCostDocsContract`）。`docs/tasks/0008_column_prompt/check_article.py:31`（`HEADER_RE`）。
-   `internal/publisher/slack_articles_test.go` と `makefile_test.go` は `IntegrationArticles` の出力の大きさに依存する。

**ビルドタグを要するファイルのコンパイルの確認。** 次のコマンドを、HEAD `6506807` で実行した。終了コードは 0 で、出力の全行が `ok` または `no test files` だった。`integration_test.go` と `slack_integration_test.go` は、タグ `integration` を付けたときだけコンパイルされる。このコマンドは `test` と `integration` を同時に付けるので、`integration` だけのビルドは `make lint`（`go vet -tags integration ./...`）が確かめる。`testutil/integration.go` は両方のタグでコンパイルされるので、`test` だけのシンボルに依存しない。

```text
go test -run '^$' -tags test,integration ./...
```

**`Article` を作る経路の調査。** 設計書は具象型による分岐を持たない（`stageModel`・`slackPost` は整形の内部の値で、公開の入口を持たない）ので、作る経路を 1 つに絞る追加の措置は要らない。

### 1.4. 申し送りの項目への対応

**設計への申し送り（H-01〜H-06）。** すべて設計書 6.1 に記録されている。設計書が記録していない項目はない。

**実装への申し送り（I-01〜I-05）。**

| 項目 | 対応するステップ | 方針 |
|---|---|---|
| I-01 `ProseRanges` の実装 | 4-1 | `markdown.go` の `proseLines`・`codeSpans`・`codeSpansExempt` から範囲を組み立てる。判定できない 5 つの場合（閉じていないフェンス、フェンスらしい行、対にならないバッククォート、フェンスの外にバッククォートがあるときの `codeSpansExempt` が偽、複数の行にまたがるコードスパン）は false を返す。フェンスの外にバッククォートがない本文は、`[`・`]`・`\` があっても散文として true を返す |
| I-02 rune の添字とバイトの位置の変換 | 4-2 | `neutralizeProse` が `mentionAts` の添字を元の本文のバイトの位置に直す。`＠` がコードの中と外にある本文をテストに入れる |
| I-03 分割の切る位置の移動と確認の層 | 4-4 | 切る位置を 1 文字前に動かす規則と、分割後の確認の層の両方を実装する |
| I-04 切る位置の探索の範囲 | 4-4 | 探索の上限を容量と `keepTogetherFrom` の小さいほうにする。`isWhitespaceOnly` を `keepTogetherFrom` で切った断片にも適用する |
| I-05 M2 の検査用の文字列 | 4-2 / 4-5 | M2 は V3 だけを行う。M1・M3・M4 の検査用の文字列は変えない |

## 2. 実装ステップ (Implementation Steps)

### 2.1. 名前を変えるものの一覧

名前の変更・移動・置き換えは、この表を唯一の記録とする。本計画のほかの節と設計書の参照は、この表に従う。

| 旧 | 新 | フェーズ |
|---|---|---|
| `renderArticle`（`file.go`） | ファイル: `renderFileArticle`（`file_render.go`、フェーズ 3）。Slack: `renderSlackPost`（`slack_render.go`、フェーズ 4）。`renderArticle` はフェーズ 4 で削除する | 3・4 |
| `articleCostLine`（`file.go`） | `articleCost`（`format.go`。金額と既知かどうかを返す。行の文字列は各整形が組み立てる）。`articleCostLine` はフェーズ 2 で `articleCost` を使う形に直し、ステップ 4-11 で `renderArticle` と一緒に削除する | 2・4 |
| `modelVersionOrNone`・`articleCostDecimals`（`file.go`） | 同名で `format.go` へ移す | 2 |
| `splitSlackText(posted string)` | `splitSlackText(post slackPost)` | 4 |
| `integrationHeader`（`testutil/integration.go`） | 新しい投稿の見出し行と脚注の長さを数える関数に置き換える（名前はコードで決める） | 4 |
| `findInspectionMention` の M2 の分岐 | `mentionAts` の結果を使う分岐（`findMention` に置く。`findInspectionMention` は M1 だけを判定する） | 4 |
| `TestRenderArticleSteps`・`TestRenderArticleCost`（`file_test.go`） | `renderFileArticle` を確かめるテストに改める（名前はコードで決める） | 3 |

### フェーズ 1: チャンネル名の受け渡しと検査（F-001）

**前提（手動）。** 設計書 7.2 の実機の確認を、フェーズ 1 の実装の前に行う。実際の YouTube の動画ページで、拡張が読む `channelName` の `textContent`（`extension/src/browser/chromeDeps.ts:123`）に制御文字や前後の空白が混ざらないことを確かめ、結果を §5.1 に記録する。混ざる場合は、拡張側の不具合として別に直す（サーバの拒否は変えない）。

#### ステップ 1-1: 規則と番兵エラー

**対象:** `internal/writer/errors.go`・`internal/writer/output.go`

-   [x] `ErrInvalidChannelName` を加える（設計書 3.1）。
-   [x] `CheckChannelName` と `maxChannelNameRunes`（200）を `output.go` の `displayStringProblem` の隣に置く。規則は設計書 3.1 のとおり（有効な UTF-8、空白だけでない、制御文字なし、双方向の制御文字 U+202A〜U+202E・U+2066〜U+2069 なし、200 コードポイント以下）。空文字は受理する。エラーは規則の名前だけを示し、値を含めない。
-   [x] `displayStringProblem` に有効な UTF-8 の検査を加える。

**完了:** `go test -tags test ./internal/writer` が通る。

#### ステップ 1-2: `Article` への受け渡し

**対象:** `internal/writer/writer.go`・`internal/writer/output.go`

-   [x] `Article` に `ChannelName string` を加える。`==` で比較できる構造を保つ。
-   [x] `newArticle` に `channelName` を加え、`writer.go:405` の呼び出しで `t.ChannelName` を渡す。

**完了:** ステップ 1-6 の `Article.ChannelName` の受け渡しのテスト（`--after` なしと 3 つの再開）が通る。
#### ステップ 1-3: 公開の直前の検査

**対象:** `internal/writer/article.go`

-   [x] `CheckPublishable` に `CheckChannelName` を加え、違反を `ErrInvalidArticle` と `ErrInvalidChannelName` の両方で包む（複数の `%w` は `article.go:37` に前例がある）。
-   [x] `CheckPublishable` に `SourceURL` の UTF-8 の検査を加える（設計書 3.5）。

**完了:** ステップ 1-6 の `CheckPublishable` のテスト（チャンネル名と各値の UTF-8 の拒否）が通る。
#### ステップ 1-4: LLM を呼ぶ前の検査

**対象:** `internal/writer/prompt.go`

-   [x] `validateTranscript` が `CheckChannelName` を呼び、違反を `ErrInvalidTranscript` と `ErrInvalidChannelName` の両方で包んで返す。UTF-8 の検査の表（`prompt.go:66-70`）から `ChannelName` の項目を外し、`Title`・`Description` の検査は残す。

**完了:** ステップ 1-6 の `validateTranscript` のテスト（`TestWriteRejectsInvalidTranscript` の行）が通る。

#### ステップ 1-5: 要求の入口の検査

**対象:** `internal/server/request.go`

-   [x] `parseRequest` が `channelName` を `CheckChannelName` で検査し、違反を `reasonInvalidBody`（HTTP 400）で拒否する。README の拒否の理由の表は変えない。

**完了:** ステップ 1-6 の `parseRequest` のテストと、3 つの呼び出し元の一致を確かめるテストが通る。

#### ステップ 1-6: テスト

-   [x] `internal/writer/output_test.go` に `CheckChannelName` の表のテストを加える（空文字、通常の名前、空白だけ、制御文字、不正な UTF-8、双方向の制御文字、ZWJ を含む名前、200 と 201 コードポイントの境界）。
-   [x] `internal/writer/article_test.go` に `CheckPublishable` のチャンネル名の拒否と、各値（`SourceURL` を含む）の不正な UTF-8 の拒否のテストを加える。エラーが値を含まないことも確かめる（既存の `TestArticleCheckPublishableOmitsValues:145` を拡張してよい）。
-   [x] `internal/writer/steps_test.go` に、`Article.ChannelName` が、`--after` なしの実行と `--after generation`・`shorten`・`review` の再開のどれでも入力と同じであることを確かめるテストを加える。
-   [x] `internal/writer/prompt_test.go` の `TestWriteRejectsInvalidTranscript:38` に、チャンネル名の拒否の行を加える。
-   [x] `internal/server/request_test.go` に `parseRequest` のチャンネル名の拒否のテストを加える。
-   [x] 3 つの呼び出し元の一致を確かめるテストを加える。同じ入力の表を `CheckPublishable`・`validateTranscript`・`parseRequest` に通し、受理と拒否が一致することを確かめる。表は 1 つにし、複数のパッケージのテストが使うので、既存の `internal/writer/testutil/` に置く（`docs/dev/developer_guide/test_organization.md` の分類 A。公開の API だけを使う）。`CheckPublishable` は直接、`validateTranscript` は `writer.Write`（fake の `LLMClient`）を通して、`parseRequest` は `server` のパッケージの中から、それぞれ呼ぶ。
    -   実装では、表を `internal/writer/testutil/helpers.go` の `ChannelNameCases` に置き、3 つの呼び出しをまとめて 1 つのテスト（`internal/server/request_test.go` の `TestChannelNameAgreesWithWriter`）で確かめた。`writer` の内部のテストは `writertestutil` を import できない（循環する）ので、`CheckChannelName` の個別の規則と境界（200・201 コードポイント）は `output_test.go` の `TestCheckChannelName` が別に持つ。
-   [x] 違反したチャンネル名の記事を書き出し・通信の前に拒否することを、既存の拒否の表（`file_test.go::TestFilePublisherRejectsArticle:320`、`slack_message_test.go::TestSlackPrepareRejectsInvalidArticle:327`、`slack_test.go` の送信 0 件の表）に行として加える。一時ファイルが残らないことと、リクエストが 0 件であることをその行で確かめる。
    -   `displayStringProblem` が UTF-8 を検査するようになったので、`TestSlackPrepareRejectsInvalidArticle` の `Body` 以外の不正な UTF-8 の行は `CheckPublishable` の段階で拒否される。行の `checkPublishableOK` を false に改めた。`prepareSlackMessages` 側の重複した検査は、ステップ 4-4 が `Body` だけに絞る。

#### ステップ 1-7: フェーズ 1 の確認

-   [x] `make fmt` → `make test` → `make lint` が通る。
-   [x] 次の変更を 1 つずつ行い、ステップ 1-6 の対応するテストが失敗することを確かめ、コミットメッセージに記す: `newArticle` がチャンネル名を写さない（`steps_test.go` の 4 つの実行）、`CheckPublishable` の `CheckChannelName` の呼び出しを外す、`CheckPublishable` の `SourceURL` の UTF-8 の検査を外す、`displayStringProblem` の UTF-8 の検査を外す、`validateTranscript` の `CheckChannelName` の呼び出しを外して `ChannelName` を UTF-8 の表に戻す（制御文字のチャンネル名の拒否のテスト）、`parseRequest` の検査を外す、`CheckChannelName` の双方向の制御文字・長さの上限の規則を外す。

### PR-1 作成ポイント: channel name carrying and validation

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7

**推奨タイトル**: `feat(0028): carry and validate the channel name`

**レビュー観点**: 3 か所（入口・LLM を呼ぶ前・公開の直前）が同じ `CheckChannelName` で一致すること / 拒否のエラーが値を含まないこと / `Article` が `==` で比較できる構造を保つこと / 各検査を外したときにテストが失敗すること

**実装モデル要件**: standard

**判定理由**: 該当するトリガはない。新しい規則は設計書 3.1 が定めており、3 か所の呼び出しと一致を確かめるテストを加える変更である。

-   [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
-   [x] PR を作成した
-   [x] PR がマージされた
-   [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 共有の値の書式（出力は変えない）

#### ステップ 2-1: `format.go`

**対象:** `internal/publisher/format.go`（新規）・`internal/publisher/file.go`

-   [x] `articleCost(runUsage) (pricing.Amount, bool)`、`modelVersionOrNone`、`articleCostDecimals` を `format.go` に置く。`modelVersionOrNone`・`articleCostDecimals` は `file.go` から移す。`articleCost` は、`articleCostLine`（`file.go:229`）が呼ぶ `pricing.Totals` と `Amount()` の組み立てをそのまま使う。
-   [x] `articleCostLine` を `articleCost` を使う形に直し、`renderArticle` の出力を変えない。
-   [x] `reviewlog.go` が `modelVersionOrNone` の移動後もそのままコンパイルされ、出力が変わらないことを確かめる。

**完了:** 既存のテストが、変更なしで通る。`format.go` の新しい関数は、フェーズ 3 とフェーズ 4 の整形が使う。

#### ステップ 2-2: フェーズ 2 の確認

-   [x] `make fmt` → `make test` → `make lint` → `make deadcode` が通る。`articleCost` が到達不能として報告される場合は、フェーズ 3 の前に `articleCostLine` から呼ばれていることを確かめる。

### フェーズ 3: ファイルの出力（F-003）

#### ステップ 3-1: 整形

**対象:** `internal/publisher/file_render.go`（新規）

-   [x] `renderFileArticle` を設計書 3.4 のとおりに実装する。キーの名前と順序、条件付きのキー、`(none)`、`estimated_cost_usd` の 2 つの形、フロントマターの後の空行・見出し・チャンネル名の行・本文の並びは、要件書 F-003 に従う。
-   [x] 値の書き出しは `strconv.Quote` を使い、独自のエスケープ関数は作らない（H-04、設計書 6.3）。
-   [x] `escapeLeadingMarkdown` を実装する。行頭の 1 文字だけをエスケープし、描画した結果がチャンネル名と一致するようにする。行頭がブロックの記法になる場合の文字ごとの規則は、ここで決め、コードのコメントに理由を書く（設計書 3.4）。

#### ステップ 3-2: 切り替え

**対象:** `internal/publisher/file.go`

-   [x] `FilePublisher` が `renderFileArticle` を呼ぶ（`file.go:117`）。`renderArticle` は Slack が使うので、削除しない（§1.3）。
-   [x] 書く前の `CheckPublishable`、一時ファイル、`link(2)`、上書きしない動作を変えないことを確かめる（AC-21）。

#### ステップ 3-3: テストの更新と追加

-   [x] `file_test.go` の次のテストを新しい形に改める: `TestFilePublisherWritesArticle:76`・`TestFilePublisherWritesCostLine:108`（実装では `TestFilePublisherWritesCost` に改名）・`TestFilePublisherEmptyModelVersion:127`。`TestRenderArticleSteps:141`・`TestRenderArticleCost:179` は削除し、同じ不変条件を新しい `file_render_test.go`（`TestRenderFileArticleLayout` の表の行）が引き継ぐ。
-   [x] `internal/publisher/file_render_test.go`（新規）に `renderFileArticle` の新しいテストを加える: フロントマターの先頭と区切り、キーの名前と順序、条件付きのキーの有無、`(none)`、`estimated_cost_usd` の 2 つの形、見出しの下にモデル・コストの行がないこと、チャンネル名の行の有無、フロントマターの前に BOM や空行がないこと。
-   [x] フロントマターの値のテストを加える: 引用符・バックスラッシュ・`:`・`#`・`---`・日本語を含む値で、行数とキーの並びが変わらず、`strconv.Unquote` で元の値に戻ること。出力に出るエスケープシーケンスの種類が、設計書 6.3 の集合（`\a \b \f \n \r \t \v \\ \"` と `\xNN`・`\uNNNN`・`\UNNNNNNNN`）に含まれること。後者の検査に使う値には、`\a \b \f \v`・DEL・U+0085・U+2028・U+2029・U+FEFF・非 BMP の非印字文字を入れる。
-   [x] `escapeLeadingMarkdown` の表のテストを加える（ブロックの記法になる各場合と、ならない場合）。
-   [x] `cmd/yt2column/run_test.go:1622`・`:2150`、`cost_test.go:165` を新しい形に直す。
-   [x] `cmd/yt2column/integration_test.go` の `checkIntegrationArticle`（`:236`）が、フロントマターで始まる `--out` のファイルを読むように直す。タイトルの見出し、`model` の値、末尾の出典のリンクを確かめる内容は保つ。
-   [x] `docs/tasks/0008_column_prompt/check_article.py` の `HEADER_RE`（`:31`）を、フロントマターの形に合わせる（0022 のステップ 3-2 の前例）。チャンネル名の行（ある場合）を本文から除くよう、`check()` の `body` の切り出し（`header.end()` から出典の前まで）と、先頭の見出しの検査を直す。

#### ステップ 3-4: フェーズ 3 の確認

-   [x] `make fmt` → `make test` → `make lint` が通る。
-   [x] `go test -run '^$' -tags test,integration ./...` が通る（`integration_test.go` のコンパイルの確認。§1.3 に出力を記録した同じコマンド）。
-   [x] 固定の入力から `renderFileArticle` の出力を `testdata/` の下のファイルに書き出す手順を用意する（`file_render_test.go` の `TestRenderFileArticleGolden` が同じ内容を期待値として持つ。`-update` を付けて実行すると書き直す）。そのファイルに `check_article.py` を実行して受理されることを、チャンネル名ありとなしの 2 つで確かめ、コマンドと出力を §5.3 に記録する。
    -   ステップ 3-5 で、`TestRenderFileArticleGolden` を `golden_test.go` の `TestRenderGolden` に置き換え、2 つのファイルを `testdata/golden/` に移した（`same_model.md`・`without_channel.md`）。
-   [x] 次の変更を 1 つずつ行い、ステップ 3-3 の対応するテストが失敗することを確かめ、コミットメッセージに記す: `FilePublisher` の切り替えを戻す、キーの順序の入れ替え、条件付きのキー（`channel`・`shorten_*`・`review_*`）を常に出す、`(none)` の置き換えを外す、コストの 2 つの形のそれぞれの出力を壊す、`strconv.Quote` を値をそのまま引用符で囲む書き出しに変える、`escapeLeadingMarkdown` の呼び出しを外す、チャンネル名の行の有無の分岐を外す、フロントマターの後の空行を 1 行減らす。

#### ステップ 3-5: golden file のテスト（追加）

PR-6 のマージの後に加えたステップである。§3.2 の PR 構成の外の PR で、ステップ 4-10a と一緒に入れる。設計は変えない（設計書 3.3・3.4 のとおり Go で整形し、テンプレートエンジンは使わない）。

**対象:** `internal/publisher/golden_test.go`（新規）、`internal/publisher/testdata/golden/`（新規）、`internal/publisher/file_render_test.go`

-   [x] 代表的な記事について、`renderFileArticle` の出力の全文を `testdata/golden/<case>.md` に置き、バイト単位で比べる（`TestRenderGolden`）。`go test -tags test ./internal/publisher -run Golden -update` で書き直せるようにする。`TestRenderFileArticleGolden` はこのテストに置き換え、その 2 つのファイルを移す。置き換えの前後で `go tool cover -func` の関数ごとの値が変わらないことを確かめる。
-   [x] 場合は次のとおり（入力は架空の値。`docs/dev/security.md` §7）: チャンネル名ありで 3 つの段階が同じモデル・コストが既知（`same_model`）、チャンネル名なし（`without_channel`）、段階ごとにモデルが違う（`mixed_models`）、字数の調整を飛ばした（`shorten_skipped`）、コストが `unknown` で版が `(none)`（`cost_unknown`）、チャンネル名の先頭がブロックの記法（`channel_heading`・`channel_list`）、末尾が `#`（`channel_trailing_hash`）、タイトル・チャンネル名・本文のコードの外の対象の `@`（`mentions`）、語の途中の `@`（`mid_word_at`）。各場合の入力が `CheckPublishable` を通ることも確かめる。
-   [x] golden file をツールに書き換えさせない: pre-commit の `trailing-whitespace`・`end-of-file-fixer` の対象から `internal/publisher/testdata/golden/` を外し、`.gitattributes` で `-text` にする。
-   [x] `docs/dev/developer_guide/test_organization.md` に、golden file の場所と再生成の手順を加える。
-   [x] golden file は人が読むための補助であり、ステップ 3-3 の要素ごとのテストは削らない。
-   [x] フロントマターの後の空行を 1 行増やす、`source` と `model` のキーの順序を入れ替える、の 2 つの変更で `TestRenderGolden` が失敗することを確かめ、コミットメッセージに記す。

### PR-2 作成ポイント: shared formatting and front-matter file output

**対象ステップ**: 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4

**推奨タイトル**: `feat(0028): write --out files with YAML front matter`

**レビュー観点**: `format.go` への移動で出力が変わらないこと / フロントマターのキーの順序・条件付きのキー・`strconv.Quote` の往復 / `escapeLeadingMarkdown` の文字ごとの規則 / `check_article.py` と統合テストの追従

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-1 の `escapeLeadingMarkdown` は、行頭がブロックの記法になる場合の文字ごとの規則をコードで決める孤立した複雑なステップである。そのため frontier-recommended とした。

-   [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
-   [x] PR を作成した
-   [x] PR がマージされた
-   [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: Slack の投稿（F-002）とメンションの扱い（AC-14〜AC-14f）

フェーズ 4 は、ステップ 4-1〜4-14 の順に進め、PR-3〜PR-7 の 5 つに分ける（§3.2）。各 PR は、コードとそのコードのテストを同じ PR に入れる。ステップ 4-5 で M2 の判定が変わるので、`slack_message_test.go` のメンションのテストをそのステップで直す。

#### ステップ 4-1: `ProseRanges`（I-01）

**対象:** `internal/writer/markdown.go`

-   [x] `ByteRange` と `ProseRanges(body string) ([]ByteRange, bool)` を、`proseLines`・`codeSpans`・`codeSpansExempt` を使って実装する。行の先頭の位置を累積して本文の中のバイト範囲に直す。判定できない本文（I-01 に列挙した 5 つの場合）は false を返す。全体がコードの本文は、空の範囲と true を返す。
-   [x] `internal/writer/markdown_test.go` にテストを加える（フェンス付きコードブロック、複数のバッククォートのコードスパン、判定できない 5 つの場合、インデントのコードブロックを散文とする扱い、全体がコードの本文）。

#### ステップ 4-2: メンションの判定と無効化の部品（I-02）

**対象:** `internal/publisher/slack_message.go`（`mentionAts`。M2 の判定と一緒に置く。テストは `slack_message_test.go`）、`internal/publisher/slack_render.go`（新規。`neutralizeMentions`・`neutralizeProse`。設計書 3.6。テストは `slack_render_test.go`）

-   [x] `mentionAts(text []rune) []int` を設計書 3.7(1) のとおりに実装する。名前の文字は文字・数字・`_`・`.`・`-`。`@` の直前が文字でも数字でもない（または先頭）ことを条件とする。V3（`replaceLookAlike`）だけを適用し、V2・V1 は適用しない（I-05）。
-   [x] `neutralizeMentions(value string) string` を実装する。対象の `@` の直後に U+200B を入れる。直前が `/` の `@` は無効化しない。
-   [x] `neutralizeProse(body string, prose []writer.ByteRange) string` を実装する。`mentionAts` の rune の添字を元の本文のバイトの位置に直して、範囲と比べる（I-02）。
-   [x] テストを加える: `mentionAts` の対象になる場合とならない場合（設計書 7.1 の `mentionAts` の行）、`neutralizeMentions` が入れた後の値に対象が残らないこと（例外を除く）と、対象のない値が変わらないこと、`neutralizeProse` で `＠` がコードスパンの中と外にある本文の位置の変換。

#### ステップ 4-3: PR-3 の確認

-   [x] `make fmt` → `make test` → `make lint` → `make deadcode` が通る。この時点で `mentionAts`・`neutralizeMentions`・`neutralizeProse`・`ProseRanges` は本番のコードから呼ばれず、テストだけが呼ぶ。`make deadcode` は `-test` を付けて実行するので、報告されない。報告された場合は、テストが呼んでいることを確かめる。
    -   実装では、`make deadcode` がこれらを到達不能として報告した（ベースラインの報告に 8 行加わる）。ステップ 4-5 以降（PR-4〜PR-7）が呼ぶまでの意図した足場なので、削除しない。
-   [x] 次の変更を 1 つずつ行い、ステップ 4-1・4-2 の対応するテストが失敗することを確かめ、コミットメッセージに記す: `ProseRanges` の範囲の判定を外す; `ProseRanges` が false を返す 5 つの場合のそれぞれを、true を返す形にする; `neutralizeProse` の rune からバイトへの変換を外し、rune の添字のまま比べる（`＠` の入力）; `neutralizeMentions` の直前が `/` の例外を外す; `mentionAts` の名前の文字から `.`・`-` を外す; `mentionAts` の直前の文字の条件を外す。

### PR-3 作成ポイント: mention parts (ProseRanges and neutralization)

**対象ステップ**: 4-1 / 4-2 / 4-3

**推奨タイトル**: `feat(0028): add ProseRanges and mention neutralization helpers`

**レビュー観点**: `ProseRanges` が判定できない 5 つの場合で false を返すこと / rune の添字からバイトの位置への変換（`＠` を含む本文） / `mentionAts` の名前の文字と直前の文字の条件 / 本番のコードから未使用でも `make lint` が通ること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 4-1 は Markdown のコード範囲を組み立てる小さな解析器で、判定できない場合の扱い（I-01）と rune・バイトの変換（I-02）を誤るとメンションの無効化が漏れる。孤立した複雑なステップなので frontier-recommended とした。

-   [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
-   [x] PR を作成した
-   [x] PR がマージされた
-   [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 4-4: 分割（I-03・I-04）

**対象:** `internal/publisher/slack_message.go`・`internal/publisher/slack_render.go`

-   [x] `slackPost`（`text`・`keepTogetherFrom`）の型を `slack_render.go` に加える。整形の関数はステップ 4-9 で加える。それまでの間、`prepareSlackMessages` は `renderArticle` の出力に `keepTogetherFrom` を投稿の長さとして渡す（最後の領域（`keepTogetherFrom` 以降）がなく、切る位置に制限がない）。
-   [x] `splitSlackText` が `slackPost` を受け取る形にする。最後以外の断片は `keepTogetherFrom` 以前で切る。探索の上限を容量と `keepTogetherFrom` の小さいほうにし、最後の領域が容量を超えるときは `ErrSlackUnsplittable` で拒否する（I-04、設計書 3.5）。`isWhitespaceOnly` を `keepTogetherFrom` で切った断片にも適用する。
    -   実装では、切る位置を選ぶ層を `cutSlackFragments` に分け、`splitSlackText` はその結果に確認の層をかけてから位置の記号を付ける。
-   [x] 改行の後以外の位置で切るとき、切る位置の直後が対象の `@` で、直前が文字または数字なら、切る位置を 1 文字前に動かす。動かすと断片が空になる場合は `ErrSlackUnsplittable` で拒否する（I-03）。
    -   実装では、動かすと断片が空になる場合は動かさず、確認の層が `ErrSlackUnsplittable` で拒否する（結果は I-03 と同じ）。これにより、確認の層を通しで確かめる入力ができる。移動の規則と確認の層は、同じ判定 `cutStartsMention` を使う。
-   [x] 分割の後に確認の層を加える。位置の記号 `(n/m)` を含む各メッセージの先頭の `@` について、投稿の全体では語の途中だったのにメッセージの中で対象になったものがあれば、`ErrSlackUnsplittable` で拒否する（I-03）。確認の層は独立した関数にし、手で作った断片の並びを直接渡してテストする。テストの入力には、切る位置の移動の規則を通った後にも残る形を選ぶ。先に、移動の規則だけではこの入力が変わらず拒否もされないことを確かめる。
    -   実装では、確認の層は `checkFragmentStarts`（`TestCheckFragmentStarts` が断片の並びを直接渡す）。通しのテスト `TestSlackSplitRefusesUnmovableCut` は、先に `cutSlackFragments` だけでは断片の先頭に `@here` が残り拒否もされないことを確かめ、次に `splitSlackText` が拒否することを確かめる。
-   [x] `prepareSlackMessages` の UTF-8 の検査（`:29-41`）を、`Body` だけを対象にする形に絞る（設計書 3.5）。
-   [x] `slack_message.go:47-49` のコメント（M1〜M4 の前提）を、M2 だけが成り立たなくなり、分割の規則と確認の層がそれを補うこと（設計書 3.7(3)）を述べる形に書き換える。
-   [x] `splitSlackText` を呼ぶ既存のテストを、`slackPost` を渡す形に直す。
-   [x] 分割と `@` のテストを `slack_message_test.go` に加える: 改行のない長い本文で、切る位置が `x` と `@` の間に来る入力で `@` が断片の先頭にならないこと。確認の層が `ErrSlackUnsplittable` を返す入力は、切る位置の移動の規則を通らない形（移動すると断片が空になる）で作る。最後の領域が容量を超える入力と、`keepTogetherFrom` の手前が容量にちょうど収まる入力（`keepTogetherFrom` を直接指定する）も加える。
    -   実装では、`TestSlackSplitAvoidsMidWordAt`（`x@here`・`1@here`・`x＠here`）、`TestSlackSplitRefusesUnmovableCut`、`TestSlackSplitKeepTogether` に置いた。既存のテストは `prepareSlackMessages` を通して呼ぶので、直す行はなかった。

**完了:** この時点では M2 も整形も旧いままで、`make test` が通る。**ステップ 4-4 と 4-5 は同じ PR-4 に入れ、4-5 だけを先にマージしない。** 4-4 の分割の規則なしに M2 を緩めると、分割が語の途中の `@` を断片の先頭に置いた投稿が、全体の検査をすり抜けるためである。

#### ステップ 4-5: M2 の置き換え（I-05）

このステップは 4-4 と同じ PR-4 に入れる。

**対象:** `internal/publisher/slack_message.go`

-   [x] `findInspectionMention` の M2 の分岐を `mentionAts` の結果に置き換える。M1 の判定（`<!`・`<@`）と、`inspectMentionText` の V2 → V3 → V1 は変えない。M3・M4 は変えない。
    -   実装では、M2 は元の文字列に V3 だけを当てる（`mentionAtsOf`）ので、検査用の文字列を受け取る `findInspectionMention` から外し、`findMention` で判定する。`mentionAtsOf` は `slack_render.go` から `slack_message.go` に移した。
-   [x] `isMentionNameRune` を削除する（呼び出し元は M2 だけ。§1.3）。
-   [x] `TestSlackMentionRejected:235`・`TestSlackMentionAccepted:273`・`TestSlackMentionPosition:428`・`TestSlackMentionMessagesFollowPosted:460` を、新しい M2 に合わせる。語の途中の `@`（`Tech@Work`、`user@example.com`、`さん@xxx`）が受理され、`foo_@bar`・`x@@y` のように直前が `_` や `@` の `@` が対象になることを確かめる行を加える。
    -   実装では、`TestSlackMentionRejected` の「at a fragment boundary」の行（`あ…@here`）は語の途中の `@` になるので、空白を挟んだ「in a later fragment」の行に改めた。`TestSlackMentionPosition` の `x\n\\@here` は、M2 がバックスラッシュを戻さないので、位置を `@` 自身（2 行目 2 列）に改めた。`TestSlackMentionAccepted` はタイトルと本文の両方で確かめる。

**完了:** 整形はまだ旧い `renderArticle` で、分割と M2 が新しい。`make test` が通る。**PR-4 のマージから PR-7 のマージまでの間、`main` は緩めた M2 を持ち、実機の確認（R-4）は済んでいない。** この間に `main` から Slack へ公開することを避ける。R-4 の確認は、ステップ 4-14 で、語の途中の `@`（`Tech@Work`・`user@example.com`）が通知にならないことも対象にする。

#### ステップ 4-6: PR-4 の確認

-   [x] `make fmt` → `make test` → `make lint` が通る。
-   [x] 次の変更を 1 つずつ行い、ステップ 4-4・4-5 の対応するテストが失敗することを確かめ、コミットメッセージに記す: `keepTogetherFrom` の上限を外す; `isWhitespaceOnly` を `keepTogetherFrom` で切った断片に適用しない; 切る位置の移動の規則を外す; 確認の層を外す; `prepareSlackMessages` の UTF-8 の検査を `Body` 以外にも広げる、または `Body` からも外す; `findInspectionMention` の M2 の分岐を旧い判定（直前を見ない）に戻す。切る位置の移動の規則と確認の層は、別々の入力のテストで確かめる。
    -   実装では、`Body` 以外へ広げる変更は、`CheckPublishable` が先に同じ値を拒否するので、どのテストも失敗しない（観測できない）。最後の領域の開始位置の拒否（`start >= keepTogetherFrom`）を外しても、空の断片がメッセージ数の上限に達して同じ `ErrSlackUnsplittable` になるので、テストでは区別できない。

### PR-4 作成ポイント: slack split rules and M2 replacement

**対象ステップ**: 4-4 / 4-5 / 4-6

**推奨タイトル**: `feat(0028): split Slack posts at safe positions and relax M2`

**レビュー観点**: 切る位置の移動の規則と確認の層が、M2 の緩和で通るようになる投稿を補っていること / `keepTogetherFrom` の探索の上限と `ErrSlackUnsplittable` の条件 / M2 の新旧の判定の差（語の途中の `@`、直前が `_`・`@`） / 各変更を外したときにテストが失敗すること

**実装モデル要件**: frontier-required

**判定理由**: ステップ 4-5 は M2 を緩め、ステップ 4-4 の分割の規則と確認の層が同時にそれを補う。拒否を下げる変更と上げる変更が同時に入るセキュリティの関門なので、frontier-required とした。

-   [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
-   [x] PR を作成した
-   [x] PR がマージされた
-   [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 4-7: 失敗の理由（AC-14e・AC-14f）

**対象:** `internal/failure/failure.go`・`internal/server/job.go`・`internal/publisher/slack_failure.go`・`cmd/yt2column/run.go`

-   [x] `failure.KindPublishMention` を `KindPublish` の次に加える。
-   [x] `classify`（`job.go:109`）が、公開段階の失敗が `ErrSlackMention` を包むとき `KindPublishMention` を返す。
-   [x] `slack_failure.go:113` の写像に、`KindPublishMention` の固定の文面を加える（要件書 AC-14f の内容を英語で）。通知はサーバが選んだ固定の値だけで組み立てる。
-   [x] `run.go:1031` の案内を、原因（`@` の直後に名前の文字が続く並び）、エラーが示す位置（行と列。1 行目は見出し行。見出し行が入るのは PR-7 からで、それまでは旧い整形での位置である）、再実行すると別の記事が生成されること、`--out` で記事を確かめられることを示す文面にする。`@` の後の文字列は示さない。
-   [x] 失敗の理由のテストを、`failure_test.go` の `TestKindConstants:65`、`job_test.go` の `TestFailureClassification:160`、`slack_failure_test.go` の `TestSlackFailureNotifyText:115`、`run_test.go:1489` の行に加える。CLI の案内が `@` の後の文字列を含まないことも確かめる。

#### ステップ 4-8: PR-5 の確認

-   [x] `make fmt` → `make test` → `make lint` が通る。
-   [x] `classify` の `ErrSlackMention` の分岐を外し、ステップ 4-7 の対応するテストが失敗することを確かめ、コミットメッセージに記す。

### PR-5 作成ポイント: publish-mention failure reason

**対象ステップ**: 4-7 / 4-8

**推奨タイトル**: `feat(0028): report Slack mention rejection as its own failure kind`

**レビュー観点**: `classify` が `ErrSlackMention` だけを `KindPublishMention` に写すこと / 通知の文面がサーバの選んだ固定の値だけで組み立てられること / CLI の案内が `@` の後の文字列を含まないこと

**実装モデル要件**: standard

**判定理由**: 該当するトリガはない。既存の `Kind` の写像に 1 つの種類を加える変更で、方式は一つに定まっている。

-   [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
-   [x] PR を作成した
-   [x] PR がマージされた
-   [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 4-9: 整形

**対象:** `internal/publisher/slack_render.go`（ステップ 4-2・4-4 で作成済み）

-   [x] `renderSlackPost`、`stageModel`、`stageModels` を設計書 3.3 のとおりに実装する。見出し行は `# <title>（<channel>）`（チャンネル名がないときは `# <title>`）。脚注は英語の平文 1 行で、版を出さず、同じモデル名の段階を 1 つにまとめる。コストは `articleCost` を使う。
-   [x] 見出し行のタイトルとチャンネル名、脚注のモデル名は `neutralizeMentions` を、本文は `ProseRanges` と `neutralizeProse` を通してから並べる。`ProseRanges` が false のときは、本文の `@` を無効化しない（このとき本文の対象の `@` が拒否される経路を、`renderSlackPost` から `prepareSlackMessages` まで通すテストをステップ 4-13 に加える）。`Article` の値は変えない。
-   [x] `keepTogetherFrom` は、`Body` の末尾の改行を除いた最後の行の先頭の rune の位置とする（設計書 3.3）。
-   [x] `renderSlackPost` のテストを `slack_render_test.go` に加える: 先頭行（チャンネル名の有無）、本文の前が見出し行と空行だけであること、最終行が脚注で 1 行であること、版が出ないこと（版が空の場合と空でない場合で同じ）、同一モデルの統合、異なるモデルの併記と段階の順序、字数の調整・推敲が走らない場合、コストの 2 つの形、`keepTogetherFrom` が本文の最後の行を指すこと。見出し行・脚注の対象の `@name` と `＠everyone` が無効化されること、本文のコードの外の対象の `@name` が無効化されコードの中のものは無効化されないこと、語の途中の `@` を含むタイトル・本文・チャンネル名が変わらないこと、`ProseRanges` が false のとき本文が変わらないこと。

**完了:** `renderSlackPost` は本番のコードからまだ呼ばれない。`make test` が通る。

#### ステップ 4-10: PR-6 の確認

-   [x] `make fmt` → `make test` → `make lint` → `make deadcode` が通る。`renderSlackPost` が本番のコードから呼ばれるのは PR-7 からで、それまでは `make deadcode` が `stageModels`・`renderSlackPost` などを到達不能として報告する。報告された項目は、テストが呼んでいることを確かめ、PR-7 が呼ぶまでの意図した足場として削除しない。
-   [x] 次の変更を 1 つずつ行い、ステップ 4-9 の対応するテストが失敗することを確かめ、コミットメッセージに記す: `renderSlackPost` の `neutralizeMentions` の呼び出しを外す; `neutralizeProse` の呼び出しを外す; `stageModels` の同一モデルの統合を外す、または段階の順序を入れ替える; `keepTogetherFrom` の指す位置を本文の末尾にする。

### PR-6 作成ポイント: slack post renderer

**対象ステップ**: 4-9 / 4-10

**推奨タイトル**: `feat(0028): add the Slack post renderer with header and footnote`

**レビュー観点**: 見出し行・本文・脚注の並びと `keepTogetherFrom` の位置 / 見出し行・脚注・本文の `@` の無効化（コードの内外） / `stageModels` の統合と順序 / 本番のコードから未使用でも既存の動作が変わらないこと

**実装モデル要件**: standard

**判定理由**: 該当するトリガはない。設計書 3.3 が整形の形を定めており、新しい関数は PR-3 の部品の組み合わせである。

-   [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
-   [x] PR を作成した
-   [x] PR がマージされた
-   [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 4-10a: golden file のテスト（追加）

PR-6 のマージの後に加えたステップである。§3.2 の PR 構成の外の PR で、ステップ 3-5 と一緒に入れる。

**対象:** `internal/publisher/golden_test.go`（ステップ 3-5 と同じファイル）、`internal/publisher/testdata/golden/`

-   [x] ステップ 3-5 の各場合について、`renderSlackPost` の `text` を `testdata/golden/<case>.slack.txt` に置き、バイト単位で比べる（`TestRenderGolden`）。16,383 文字を超えて複数のメッセージに分かれる投稿は、人が読める大きさにならないので golden file にせず、分割のテスト（ステップ 4-4・4-13）に任せる。
-   [x] U+200B は見えるように置き換えず、実際の出力のまま golden file に入れる。`mentions` の golden file に U+200B が入っていることをテストのコメントに書き、入るべき位置にあることと、`mid_word_at` に入っていないことを `TestRenderGoldenNeutralized` で確かめる。
-   [x] golden file は人が読むための補助であり、ステップ 4-9 の要素ごとのテストは削らない。
-   [x] 見出し行の `neutralizeMentions` の呼び出しを外す、脚注の前の空行を 1 行増やす、の 2 つの変更で `TestRenderGolden` が失敗することを確かめる。`mentions.slack.txt` から U+200B を 1 つ除くと `TestRenderGoldenNeutralized` が失敗することを確かめる。コミットメッセージに記す。

#### ステップ 4-11: 切り替え

**対象:** `internal/publisher/slack_message.go`・`internal/publisher/file.go`

-   [x] `prepareSlackMessages` が `renderSlackPost` を呼ぶ。分割の後にメンションを検査する順序を保つ。
-   [x] `rejectMention` が示す位置（行と列）が、Slack に送る形（無効化の後）の投稿の文字列で数えたものになるようにする（設計書 3.7(2) の最後）。
    -   実装では、`prepareSlackMessages` が `renderSlackPost` の `text`（無効化の後）をそのまま `rejectMention` に渡すので、コードの変更は要らない。`TestSlackMentionPositionAfterNeutralization` が、無効化で入った U+200B を数えた列を確かめる。
-   [x] 旧い `renderArticle`・`articleCostLine` を削除する（§2.1）。削除の前後で `go tool cover -func` の関数ごとの値を比べる（§5.2・§5.3）。
    -   `go tool cover -func` の関数ごとの値は、削除した 2 関数を除いて下がっていない（§5.3）。`cutSlackFragments` は、メッセージ数の上限の分岐を通す入力（1 行の長い本文）が最後の領域の拒否に変わったので、行ごとの入力に改めて 100% を保った。

#### ステップ 4-12: 統合テストの補助

**対象:** `internal/publisher/testutil/integration.go`

-   [x] `integrationHeader` を、新しい投稿の見出し行（空行を含む）と脚注の長さを数える関数に置き換え、`IntegrationArticles` の本文の切り方を、投稿全体が `single` ではちょうど 16,383 コードポイントになる形に改める（§1.3）。`split` が複数のメッセージになり、10 通の上限に遠いことを保つ。
    -   実装では、`integrationFraming` に置き換えた。`integrationBody` は、末尾が改行で終わらないようにする（投稿が末尾の改行を落とすため）。
-   [x] このファイルの変更は、タグ `test` と `integration` の両方でコンパイルされる（ファイル先頭の `//go:build test || integration`）。

#### ステップ 4-13: テストの更新と追加

-   [x] `file_test.go` に残る `renderArticle` を呼ぶ行はない（ステップ 3-3 で改めた）ことを確かめる。`slack_test.go`・`slack_message_test.go` の `renderArticle(article)` を、`renderSlackPost(article).text` を使う形に直す。期待値の中の `- Estimated cost: …` の行と、その行が 1 回だけ現れることを確かめるテスト（`TestSlackPublishCostLine:513`）は、脚注の形に直す。
-   [x] 分割のテストを `slack_message_test.go` に加える: 16,383 文字を超える記事で、見出し行が先頭のメッセージにだけ、脚注が最後のメッセージにだけ 1 回現れ、脚注だけのメッセージがないこと。入力は、本文の最後の改行の直後で切れる場合、10 通に脚注が加わる場合、容量を超える見出し行を含む場合。`SlackMessageCount` が送る数と一致すること（`TestSlackMessageCount:398` を拡張してよい）。見出し行が分割されるほど長い `Tech@Work` のタイトルでも、拒否されないこと。
    -   実装では、`TestSlackSplitLayout`（境界付近の長さの掃引、10 通、容量を超える見出し行）に置いた。メッセージ数の上限のテストは、1 行の長い本文では最後の領域の拒否になるので、行ごとの入力に改めた。
-   [x] メンションの拒否と無効化のテストを、設計書 7.1 の該当行に従って加える。計画として足すのは次のとおり: 本文のコードの外の対象の `@name` は無効化されて拒否されない。コードスパン・コードブロックの中と、直前が `/` の対象の `@` は `ErrSlackMention` で拒否される。見出し行と脚注の対象の `@name` と `＠everyone` は無効化される。語の途中の `@` を含むタイトル・本文・チャンネル名は変更も拒否もされない。`<!`・`<@`、文字参照、`<a|b>` は見出し行・脚注・本文のどこにあっても拒否される。`ProseRanges` が false の本文で、対象の `@` が拒否される経路を `prepareSlackMessages` まで通すテストを加える（ステップ 4-9）。位置のテストも加える: 見出し行か本文に、無効化で U+200B が入る対象の `@` があり、その後ろに拒否される `@`（コードの中）がある投稿で、`rejectMention` が示す行と列が、Slack に送る形（無効化の後）の文字列で数えた値になること。
    -   実装では、`TestSlackMentionRejected`（M1・M3・M4）、`TestSlackMentionNeutralized`、`TestSlackMentionTargetsOutsideNeutralization`、`TestSlackMentionPositionAfterNeutralization` に置いた。ほかの `@channel` を本文の散文に置いていたテスト（`cmd/yt2column/run_test.go`、`slack_test.go`）は、コードスパンの中に置く形に改めた。
-   [x] `internal/publisher/slack_integration_test.go`（`integration` タグ）が、更新した `IntegrationArticles` と矛盾しないことを確かめる。
-   [x] `go test -tags test ./internal/publisher -run Golden -update` を実行し、`testdata/golden/` に差分が出ないことを確かめる（切り替えは `renderSlackPost` の出力を変えない）。出力の形を変える PR では、golden file の差分をレビューで確かめるよう PR の本文に書く。

#### ステップ 4-14: PR-7 の確認

-   [x] `make fmt` → `make test` → `make lint` → `make deadcode` が通る。
-   [x] `go test -run '^$' -tags test,integration ./...` が通る。
-   [x] 次の変更を 1 つずつ行い、ステップ 4-11〜4-13 の対応するテストが失敗することを確かめ、コミットメッセージに記す: `prepareSlackMessages` の `renderSlackPost` の呼び出しを旧い整形に戻す; 脚注を最後以外のメッセージにも出す; `rejectMention` の位置を無効化の前の文字列で数える。
    -   実装では、旧い整形へ戻す変更は `renderArticle` の削除で不可能なので、割り当てた他の 3 つで確かめた: 脚注を最後以外のメッセージにも出す（`TestSlackSplit`・`TestSlackSplitLayout`・`TestSlackPublishCostLine` が失敗）、位置を U+200B なしで数える（`TestSlackMentionPositionAfterNeutralization` が失敗）、分割に整形前の本文を渡す（分割・メンション・ペイロードのテストが失敗）。
-   [x] **手動（実機）。** `@`・U+200B・`name` の投稿が、通知にならないことを実際の Mattermost で確かめる（設計書 10 章 R-4）。確かめた Mattermost の版と結果を §5.1 に記録する。確認が済むまでは、どの版でも通知を防ぐとは書かない。通知を受けないテスト用のアカウントか、`silent` に対応した版を使う。あわせて、語の途中の `@`（`Tech@Work`・`user@example.com`）を含む投稿が通知にならないことも確かめる（PR-4 が受理するようになった形）。
-   [x] **手動（実機）。** 見出し行と脚注の見え方を、テスト用の Webhook に `make test-integration-slack` で投稿して確かめる（AC-05〜AC-07）。実行は利用者の承認を得てから行う（CLAUDE.md「Tool Execution Safety」）。

### PR-7 作成ポイント: switch Slack to the new post

**対象ステップ**: 4-11 / 4-12 / 4-13 / 4-14

**推奨タイトル**: `feat(0028): switch Slack publishing to the new post layout`

**レビュー観点**: `renderArticle`・`articleCostLine` の削除の前後で `go tool cover -func` が下がっていないこと / `IntegrationArticles` が投稿全体でちょうど 16,383 コードポイントになること / `rejectMention` の位置が無効化の後の文字列で数えられていること / 実機の確認（Mattermost の R-4 と `make test-integration-slack`）の記録

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 4-12 は投稿全体をちょうど 16,383 コードポイントにする計算をタグ付きのファイルで行い（ビルドタグのコンパイル確認に該当）、ステップ 4-11・4-13 は旧い整形の削除と多数のテストの移行を伴う。方式は設計書で定まっており、実機の確認は利用者が行うので、孤立した複雑なステップとして frontier-recommended とした。

-   [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
-   [x] PR を作成した
-   [ ] PR がマージされた
-   [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 文書（F-004）

#### ステップ 5-1: 文書の更新

-   [ ] `README.md`（英語で書く）: Estimated cost の節（`:241-245`）の「heading metadata の 1 行」を、フロントマターの `estimated_cost_usd` と Slack の脚注に書き直す。`--out` のファイルのフロントマターの例と、Slack の投稿の構成の例を、実際の出力と合う形で加える。メンションの拒否の説明（`:317-319`）を、AC-14〜AC-14f に合わせる（語の途中の `@` は拒否しないこと、見出し行・脚注・本文のコードの外の対象の `@` は U+200B で無効化すること、コードの中と直前が `/` の `@` は拒否すること）。保存済みの `info.json` に、制御文字を含むチャンネル名や空白だけのチャンネル名があると、LLM を呼ぶ前に拒否されるようになる。この互換性の変化を書く（設計書 4 章）。
-   [ ] `docs/dev/project_overview.md:36`: `Article` が持つ値と、料金の行の置き場所を新しい構成に合わせる。
-   [ ] `docs/dev/security.md`: チャンネル名が外部由来の表示用の文字列であること、検査の 3 か所、フロントマターの引用符、メンションの規則（`:74`）を、設計書 5 章と 3.7 に合わせる。
-   [ ] `docs/dev/developer_guide/package_reference.md:37`: `renderArticle` の説明を、`renderFileArticle`・`renderSlackPost` と共有の `format.go` の説明に直す。
-   [ ] `cmd/yt2column/docs_test.go` の `TestCostDocsContract:795` が、更新した README の Estimated cost の節に合うことを確かめる。合わない語があれば、文書を直す（テストの期待値を文書に合わせて緩めない）。
-   [ ] 出力の構成を確かめる文書のガードを `cmd/yt2column/docs_test.go` に加える（既存の `readDoc` を使う）。確かめる内容は次の 3 つである。(1) README のフロントマターの例と Slack の投稿の例の各コードブロックが、固定の `Article` を `FilePublisher`（一時ディレクトリ）と `publisher.SlackMessagesForTest`（既存の公開の補助）に通した実際の出力と一致すること。ガードが各ブロックを見つけられるように、README の例のコードブロックには見出しを付ける。(2) `security.md` がチャンネル名・U+200B による無効化・フロントマターの引用符を述べていること、`project_overview.md` が `Article` のチャンネル名と、脚注・フロントマターの料金を述べていること。(3) `README.md` と `project_overview.md` が旧い記述（`- Estimated cost:` の行、`heading metadata`、`見出しのメタデータ`）を含まないこと。(2)(3) で照合する語句は、コードを書くときに決める。
-   [ ] `docs/dev/developer_guide/package_reference.md` の `internal/writer` の行（`CheckPublishable` の規則）と `internal/failure` の行に、チャンネル名の検査・`ProseRanges`・`KindPublishMention` を加える。

#### ステップ 5-2: フェーズ 5 の確認

-   [ ] `make test` → `make lint` が通る。`README.md` と `docs/dev/security.md` を変えたので、`make ext-test` も通る（Node のバージョンが `extension/.node-version` と一致していること）。
-   [ ] `security.md`・`project_overview.md` の新しい記述を、設計書 5 章・3.7 と実装の `file:line` に照らして確かめ、§5.4 に記録する。README の例はステップ 5-1 のガードが確かめる。

### PR-8 作成ポイント: documentation update

**対象ステップ**: 5-1 / 5-2

**推奨タイトル**: `docs(0028): document the new output layout`

**レビュー観点**: README の例が実際の出力と一致すること（文書のガード） / `security.md` の記述が設計書 5 章・3.7 と実装に合うこと / 旧い記述が残っていないこと / `make ext-test` が通ること

**実装モデル要件**: standard

**判定理由**: 該当するトリガはない。文書とその文書のガードの更新で、設計上の判断を含まない。

-   [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
-   [ ] PR を作成した
-   [ ] PR がマージされた
-   [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の条件 |
|---|---|---|
| M1 | フェーズ 1 | チャンネル名が `Article` に届き、3 か所で同じ規則により検査される。出力は変わらない |
| M2 | フェーズ 2 | `format.go` ができ、出力は変わらない |
| M3 | フェーズ 3 | `--out` のファイルがフロントマター付きの形になる |
| M4 | フェーズ 4 | Slack の投稿が新しい構成になり、メンションの扱いが変わる |
| M5 | フェーズ 5 | 文書が新しい構成に合い、`make ext-test` が通る |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 | チャンネル名を `Article` へ運び、入口・LLM を呼ぶ前・公開の直前の 3 か所で `CheckChannelName` により検査する | standard |
| PR-2 | 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4 | 共有の値の書式を `format.go` に切り出し、`--out` のファイルをフロントマター付きの形にする | frontier-recommended |
| PR-3 | 4-1 / 4-2 / 4-3 | `ProseRanges` と、メンションの判定・無効化の部品（本番のコードからはまだ呼ばない） | frontier-recommended |
| PR-4 | 4-4 / 4-5 / 4-6 | 分割の規則と確認の層を加え、同時に M2 を `mentionAts` に置き換える | frontier-required |
| PR-5 | 4-7 / 4-8 | `KindPublishMention` を加え、メンションの拒否を専用の失敗の理由として報告する | standard |
| PR-6 | 4-9 / 4-10 | `renderSlackPost`・`stageModels` を加える（本番のコードからはまだ呼ばない） | standard |
| PR-7 | 4-11 / 4-12 / 4-13 / 4-14 | Slack の公開を新しい整形に切り替え、`renderArticle` を削除し、統合テストの補助とテストを改める | frontier-recommended |
| PR-8 | 5-1 / 5-2 | 文書とその文書のガードを新しい構成に合わせる | standard |

ステップ 3-5・4-10a（golden file のテスト）は計画の承認の後に加えたもので、PR-6 と PR-7 の間に、この表の外の PR で入れる。

PR は番号順にマージする。PR-1 のチャンネル名が PR-2 と PR-6 の整形の前提になり、PR-3 の部品を PR-4 と PR-6 が使い、PR-2 の `articleCost` を PR-6 が使う。PR-4 の分割の規則は、PR-4 の M2 の置き換えと同じ PR に入れる（ステップ 4-4 の完了の注記）。

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。フェーズ 1 のチャンネル名の受け渡しが、フェーズ 3・4 の整形の前提になる。フェーズ 2 は、2 つの整形が共有する値の書式を、出力を変えずに先に切り出す。`renderArticle` は、Slack が使うフェーズ 4 の終わりまで残す（§1.3）。メンションの扱いを Slack の整形と同じフェーズに入れるのは、見出し行と脚注の `@` の扱いが整形と一体だからである（設計書 8 章）。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

設計書 7.1 の表のとおりである。テストは `yt-dlp`・LLM の API・Webhook を呼ばない。`LLMClient` は fake を、HTTP は `net/http/httptest` を使う。新しい補助の関数が要る場合は、同じパッケージの内部だけが使うなら `test_helpers.go`（`//go:build test`）に、複数のパッケージが使うなら `testutil/` に置く（`docs/dev/developer_guide/test_organization.md`）。チャンネル名の表は `internal/writer/testutil/` に置く（ステップ 1-6）。補助のファイルは、必要になったときだけ加える。

### 4.2. 統合テスト（手動）

-   `make test-integration-slack`（実際のテスト用 Webhook に投稿する）は、利用者の承認を得て実行する。
-   LLM の API を呼ぶ `cmd/yt2column/integration_test.go` は、ステップ 3-3 の変更をコンパイルで確かめる。実行は課金を伴うので、本計画では実行しない。

### 4.3. 後方互換性

要件書 §4.4 のとおり、ファイルの形式は破壊的に変わり、旧形式は残さない。0006 の M2 と 0022 の AC-23・AC-24 の置き換えの関係は、設計書 6.2 に記録されている。0006・0022 の文書は書き換えない。

### 4.4. 横断検索

`make lint` と `make test` が見つけられない残りだけを、フェーズ 5 の終わりに確かめる。

-   `renderArticle`、`articleCostLine`、`integrationHeader` が、コメント・文書・テストの名前に残っていない。
-   `- Estimated cost`、`- Model:`、`heading metadata`、`見出しのメタデータ` が、完了したタスクの文書（`docs/tasks/00*`）を除く文書に残っていない。
-   M2 を「すべての `@name`」「`@` の後に名前の文字が続く語」とする記述が、`README.md`・`docs/dev/security.md`・`docs/dev/project_overview.md` に残っていない。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は実行でき、誤った動作では失敗するテスト、`static` はガードのテスト・`make` のターゲット・コミットされたスクリプト、`manual` は PR や実機での観察を指す。「新規」は新しいテストまたはファイル、「追加」は既存のファイルに加えるテスト、「拡張」（ブラウザ拡張とは別）は既存のテストに行を加えるものである。名前はコードを書くときに調整してよい。調整したら、この表も同じコミットで直す。

| AC | 区分 | 検証 |
|---|---|---|
| AC-01 | test | `internal/writer/steps_test.go`（追加。`--after` なしと 3 つの再開の `Article.ChannelName`） |
| AC-02 | test | `internal/writer/article_test.go::TestArticleCheckPublishable`（空のチャンネル名の行を加える） |
| AC-03 | test | `internal/writer/output_test.go`（追加。`CheckChannelName` の表）、`internal/writer/article_test.go::TestArticleCheckPublishableOmitsValues`、`internal/writer/prompt_test.go::TestWriteRejectsInvalidTranscript`、`internal/server/request_test.go`（追加。`parseRequest` の拒否）、3 つの呼び出し元の一致を確かめるテスト（新規。ステップ 1-6） |
| AC-04 | test | `internal/publisher/file_test.go::TestFilePublisherRejectsArticle`・`internal/publisher/slack_message_test.go::TestSlackPrepareRejectsInvalidArticle`（どちらもチャンネル名の行を拡張） |
| AC-05〜AC-08 | test | `internal/publisher/slack_render_test.go`（新規。`renderSlackPost`）、`internal/publisher/golden_test.go::TestRenderGolden`（`.slack.txt`。人が読むための補助）。AC-07 の複数メッセージの場合は AC-12 の行のテスト |
| AC-09・AC-10 | test | `internal/publisher/slack_render_test.go`（新規。`stageModels`） |
| AC-11 | test | `internal/publisher/slack_render_test.go`（新規。コストの 2 つの形）、`internal/publisher/slack_test.go::TestSlackPublishCostLine` |
| AC-12・AC-13 | test | `internal/publisher/slack_message_test.go::TestSlackSplit`・`TestSlackSplitRejects`・`TestSlackMessageCount`（拡張）、`internal/publisher/slack_test.go::TestSlackPublishSplit`（拡張） |
| AC-14 | test | `internal/publisher/slack_message_test.go`（追加。`mentionAts`）、`internal/publisher/slack_render_test.go`（新規。`neutralizeMentions`・`neutralizeProse`）、`internal/writer/markdown_test.go`（追加。`ProseRanges`） |
| AC-14a | test | `internal/publisher/slack_message_test.go::TestSlackMentionAccepted`（拡張） |
| AC-14b | test | `internal/publisher/slack_message_test.go::TestSlackMentionRejected`（拡張） |
| AC-14c | test | `internal/publisher/slack_message_test.go::TestSlackMentionRejected`・`TestSlackMentionPosition`（拡張） |
| AC-14d | test | `internal/publisher/slack_message_test.go`（追加。分割と `@`） |
| AC-14e | test | `cmd/yt2column/run_test.go::TestRunSlackFailureHints` |
| AC-14f | test | `internal/failure/failure_test.go::TestKindConstants`、`internal/server/job_test.go::TestFailureClassification`、`internal/publisher/slack_failure_test.go::TestSlackFailureNotifyText` |
| AC-15・AC-16・AC-20 | test | `internal/publisher/file_test.go::TestFilePublisherWritesArticle`（改めた）、`internal/publisher/file_render_test.go::TestRenderFileArticleLayout`・`TestRenderFileArticleChannelLine`、`internal/publisher/golden_test.go::TestRenderGolden`（`.md`。ステップ 3-5 で `TestRenderFileArticleGolden` を置き換えた）（先頭・区切り・キーの順序・条件付きのキー・チャンネル名の行の有無・見出しの下にメタデータがないこと） |
| AC-18 | test | `internal/publisher/file_test.go::TestFilePublisherEmptyModelVersion`（改めた）、`file_render_test.go::TestRenderFileArticleLayout`（版が空の `review_model_version`） |
| AC-19 | test | `internal/publisher/file_test.go::TestFilePublisherWritesCost`（改めた）、`file_render_test.go::TestRenderFileArticleLayout`（コストの 3 つの形） |
| AC-17 | test | `internal/publisher/file_render_test.go::TestRenderFileArticleQuoting`（フロントマターの値の往復とエスケープの種類）、`TestEscapeLeadingMarkdown`（AC-15 も） |
| AC-21・AC-22 | test | `internal/publisher/file_test.go::TestFilePublisherExistingPath`・`TestFilePublisherRejectsArticle`・`TestFilePublisherLinkFailure`（変更なしで通る）、AC-22 はチャンネル名の拒否のテスト（AC-04 の行） |
| AC-23 | static | `cmd/yt2column/docs_test.go::TestCostDocsContract` と、ステップ 5-1 で加える出力の構成の文書のガード（追加。README の例を実際の出力と比べる）、`make ext-test` |
| AC-24 | static + manual | ステップ 5-1 で加える出力の構成の文書のガード（追加）。記述を実装と照合した記録は §5.4（manual） |
| AC-25 | static + manual | ステップ 5-1 で加える出力の構成の文書のガード（追加）、`make test`（`doc-guard` の文書のガードを含む）。記述を設計書 5 章・3.7 と照合した記録は §5.4（manual） |
| AC-26 | static | `make test`、`make lint`、`make ext-test` |

AC-23〜AC-25 のガードは語句の有無だけを確かめ、記述が実装と合うことは確かめない。後者は §5.4 の照合の記録で補う。

### 5.1. 手動の確認と記録

実施するときに記録する。

| 確認 | 時期 | 結果 |
|---|---|---|
| 拡張が読む `channelName` に制御文字・前後の空白が混ざらないこと（実際の YouTube の動画ページ） | フェーズ 1 の前 | - |
| `@`・U+200B・`name` が通知にならないこと（実際の Mattermost。版を記録する） | フェーズ 4 | 2026-10-20、Mattermost クライアントから投稿して確認。`@`+U+200B+`here`/`channel`/`all`/`everyone` は通知にならない。`Tech@Work` は通知にならず、`user@example.com` は mailto のリンクになるが Mattermost のユーザー名としては扱われない。Mattermost サーバ 11.11.2、デスクトップ 6.3.1 |
| 見出し行と脚注の見え方（`make test-integration-slack`、利用者の承認の後） | フェーズ 4 | 2026-10-20 に実行して通った（single 1 通・split 3 通、マーカー `20261019T233134.424Z`）。投稿の見え方の目視は未記録 |

### 5.2. 削除・改めるテストの対応

削除するテストはない。改めるテストは、ステップ 3-3・4-5・4-13 に名前を挙げた。改めた各テストが確かめていた不変条件（上書きしない、モデルの行の順序、コストの値と丸め、メッセージの分割と位置の記号、旧い M2 の拒否）は、次のとおり引き継がれる。

-   モデルの行の順序: `stageModels` のテストとフロントマターのキーの順序のテスト。
-   コストの値と丸め: `articleCost` を使う 2 つの整形のテスト（`0.0023` と `unknown`、未登録のモデルの `unknown`）。
-   旧い M2（直前を見ずに拒否）: 置き換える方針なので、引き継ぐ不変条件はない。直前が `/` の `@`、コードの中の `@` の拒否が、新しい拒否のテストになる。
-   `renderArticle` の削除（ステップ 4-11）の前後で、`go tool cover -func` の関数ごとの値を比べ、削除した関数を除いて下がっていないことを確かめて、§5.4 に記録する（CLAUDE.md「Deleting a test is a claim that must be checked」。テストの削除はないが、関数の削除に伴う確認として行う）。

### 5.3. 確認のコマンドと出力

実施するときに、コマンド・実行したコミット・出力を記録する。

-   ステップ 3-4・4-14 の `go test -run '^$' -tags test,integration ./...`。
-   ステップ 3-4 の `check_article.py` の実行。

**ステップ 3-4 の記録（フェーズ 2・3 のコミット `5aa1903` の上で実行）。**

```text
$ go test -run '^$' -tags test,integration ./...      # 終了コード 0、全行が ok または no test files
$ python3 -I docs/tasks/0008_column_prompt/check_article.py <cache> abcdefghijk \
    internal/publisher/testdata/file_article_with_channel.md \
    internal/publisher/testdata/file_article_without_channel.md
| file | model | model version | title | sections | length | mechanical checks |
| file_article_with_channel.md | model-x | 2026-01-01 | A title | 3 | 50 | pass |
| file_article_without_channel.md | model-x | 2026-01-01 | A title | 3 | 50 | pass |
```

`<cache>` は、`abcdefghijk.current`（`a`）と `abcdefghijk.a/abcdefghijk.info.json`（`{"title":"Video title","duration":300}`）だけを置いた作業用のディレクトリである。旧い `HEADER_RE` は同じファイルを `unrecognized --out format` として拒否した（比較のための実行）。

**ステップ 3-5 の記録。** golden file を `testdata/golden/` に移して場合を増やした後、同じ `<cache>` で `.md` のすべてに実行した。

```text
$ python3 -I docs/tasks/0008_column_prompt/check_article.py <cache> abcdefghijk internal/publisher/testdata/golden/*.md
| file | model | model version | title | sections | length | mechanical checks |
| channel_heading.md | claude-haiku-5-5 | 2026-01-01 | A title | 3 | 50 | pass |
| channel_list.md | claude-haiku-5-5 | 2026-01-01 | A title | 3 | 50 | pass |
| channel_trailing_hash.md | claude-haiku-5-5 | 2026-01-01 | A title | 3 | 50 | pass |
| cost_unknown.md | model-x | (none) | A title | 3 | 50 | pass |
| mentions.md | claude-haiku-5-5 | 2026-01-01 | Notes for @alice | 3 | 83 | pass |
| mid_word_at.md | claude-haiku-5-5 | 2026-01-01 | Inside Tech@Work | 3 | 77 | pass |
| mixed_models.md | deepseek-flash | v4 | A title | 3 | 50 | pass |
| same_model.md | claude-haiku-5-5 | 2026-01-01 | A title | 3 | 50 | pass |
| shorten_skipped.md | deepseek-flash | v4 | A title | 3 | 50 | pass |
| without_channel.md | claude-haiku-5-5 | 2026-01-01 | A title | 3 | 50 | pass |
```

-   ステップ 4-11 の `renderArticle` の削除の前後の `go tool cover -func` の比較。

**ステップ 4-11 の記録（PR-7 の作業ブランチ。削除の前は HEAD `e4c1b7b`）。** `go test -tags test -coverprofile … ./internal/publisher` の `go tool cover -func` を、削除の前後で行番号を除いて比べた。

```text
< file.go:  renderArticle      77.8%
< file.go:  articleCostLine   100.0%
  （上の 2 行が削除された関数。ほかの関数の値は同じ。cutSlackFragments は、
   メッセージ数の上限のテストを行ごとの入力に改めて 100.0% を保った）
< total: 95.9%    > total: 96.1%
```

### 5.4. 文書の照合の記録

ステップ 5-1・5-2 で、文書の記述と実装の照合を記録する。

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 対策 |
|---|---|
| フェーズ 4 が大きく、レビューしにくい | ステップ 4-1〜4-14 の順に進め、PR-3〜PR-7 に分けた（§3.2）。メンションの部品（4-1・4-2）は PR-3 として独立して通る。分割（4-4）と M2 の置き換え（4-5）は同じ PR-4 に入れる |
| 無効化が実際の Mattermost の版で効かない | 設計書 5 章のとおり、R-4 の確認が済むまで保証しない。`silent` と拒否の層は残る |
| `IntegrationArticles` の大きさの計算が合わず、統合テストが 16,383 コードポイントを超える | ステップ 4-12 で、`TestIntegrationArticles` が通ることを確かめる |
| 本文が極端に長いとき、従来は収まった記事が 11 通になり拒否される（設計書 3.5） | 受け入れる。字数の調整が有効な既定の実行では起こらない |
| `ProseRanges` が判定できない本文で、対象の `@` が拒否される頻度が高い | 設計書 R-6 のとおり、まず測らず、プロンプトの変更はこのタスクの外とする |
| 文書のガードのテストが README の語句を読むので、README の更新でテストが壊れることがある | ステップ 5-1 で `TestCostDocsContract` を確かめ、`make ext-test` を実行する |

### 6.2. スケジュールのリスク

R-4 の実機の確認は、通知を受けないテスト用のアカウントか `silent` に対応した版が要る。用意できない場合、フェーズ 4 のコードの完了と、保証する版の決定を分けて記録する。

## 7. 実装チェックリスト (Implementation Checklist)

-   [ ] フェーズ 1 の前の手動の確認（§5.1）
-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7）
-   [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4）
-   [ ] PR-3 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3）
-   [ ] PR-4 マージ済み（対象ステップ: 4-4 / 4-5 / 4-6）
-   [ ] PR-5 マージ済み（対象ステップ: 4-7 / 4-8）
-   [ ] PR-6 マージ済み（対象ステップ: 4-9 / 4-10）
-   [ ] PR-7 マージ済み（対象ステップ: 4-11 / 4-12 / 4-13 / 4-14）
-   [ ] PR-8 マージ済み（対象ステップ: 5-1 / 5-2）
-   [ ] §4.4 の横断検索
-   [ ] すべての AC の検証が §5 の表のとおり実行され、手動の確認が §5.1・§5.4 に記録されている
-   [ ] `make test`・`make lint`・`make ext-test` が通る

## 8. 成功基準 (Success Criteria)

-   要件書の AC-01〜AC-26（AC-14a〜AC-14f を含む）の検証が、§5 の表のとおり実行されている。
-   `--out` のファイルがフロントマターで始まり、Slack の投稿が見出し行・本文・脚注の並びになっている。
-   旧い `renderArticle` が残っていない。
-   メンション・構造の破壊のテスト（AC-03・AC-14〜AC-14d・AC-17）が通り、R-4 の実機の確認の結果が §5.1 に記録されている。
-   文書の更新がステップ 5-1 のガードと §5.4 の照合の記録で確かめられている。
-   新しい依存パッケージが加わっていない。

## 9. 次のステップ (Next Steps)

1.  利用者が本計画をレビューし、`approved` にする。
2.  `/runplan 0028` でフェーズ 1 から実装する。
