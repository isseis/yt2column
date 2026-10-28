# 実装計画書：文書の言語を読み手で決める方針への整理と、README の日英二言語化

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-28 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番、「設計書 3.6」などは設計書の節を指す。既存のコードと文書についての記述は、コミット `c76eec26`（本書の作成時の `main`）で確かめた。`file:line` は、特に断らない限り、このコミットの行番号を指す。

設計書が引く行番号はコミット `364c95b8` のものである。2 つのコミットの差は、拡張の 8 ファイル（`extension/src/ui/`・`extension/static/style.css`・`extension/test/` の描き方のテスト）と、タスク 0046・0048・0049 の文書だけである（`git diff --stat 364c95b8 c76eec26`）。設計書が引く Go のコード、README、CLAUDE.md、コマンドの文書、`extension/src/core/i18n/` の行番号は、そのまま使える。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、次を行う。

-   `docs/dev/developer_guide/` を `docs/dev/conventions/`（英語）と `docs/dev/guides/`（日本語）に分け、旧パスをすべて置き換える（F-002、設計書 2.2・6.2）。
-   文書の言語の原則と例外を CLAUDE.md に書く（F-001、設計書 1.2）。
-   `/mktrans` を `README.md` → `README.en.md` の組に改め、`/runplan`・`/fixpr`・`/runpr` に同期の手順を加える（F-006、設計書 2.4・3.8・4.3）。
-   ガードテストの Markdown の補助関数を共有パッケージ `internal/doctestutil` に移す（設計書 3.3）。
-   README から開発者向けの内容を `docs/dev/guides/cli.md` と `docs/dev/guides/development.md` に移す（F-003、設計書 3.6）。
-   `README.md` を日本語にし、`README.en.md` を作り、ガードテストを両版に広げ、構造の一致のテストを加える（F-004・F-005、設計書 3.3〜3.5・3.7・4.1）。

yt2column の実行時の振る舞いは変えない（要件書 4.1・4.3）。Go の本番のコードで変えるのは、利用者に参照先を示す 3 つの文字列（`cmd/fixprguard/main.go:132`、`cmd/yt2column/run.go:225`・`:259`、`cmd/yt2column-server/run.go:145`）と、コメントのパスだけである。

### 1.2. 実装の方針

-   **設計は設計書に従い、本書は繰り返さない。** 置き場所は設計書 2.2、ファイルごとの変更は 3.1、README の振り分けは 3.6、ガードテストの振り分けは 3.7、`/mktrans` の振る舞いは 3.8、同期の手順は 4.3、旧パスの対応表は 6.2 が定める。本書のステップは、どの節に従って何を変え、どう確かめるかだけを示す。
-   **移動と内容の変更を別のコミットにする。** `git mv` だけのコミット、パスの置換のコミット、翻訳のコミットを分け、レビューで差分を読み分けられるようにする。README の振り分けも、英語のまま移すコミットと翻訳のコミットを分ける（1.3）。
-   **テストは壊して確かめる。** 新しいテストと、検査の対象を変えたテストは、守る対象を壊して失敗することを確かめてからコミットし、そのことをコミットのメッセージに書く（CLAUDE.md、Testing Strategy）。
-   **消すテストは、関数ごとの網羅を比べる。** テストや表の行を消すときは、`go tool cover -func` の関数ごとの結果が前後で変わらないことを確かめ、コミットのメッセージに書く。

### 1.3. 計画の際に決めたこと（2026-10-28）

-   **フェーズ 2 とフェーズ 3 を 1 つの PR にする（レビュアーが選んだ）。** どちらも指示の文書だけを変え、言語の方針と翻訳の流れという 1 つの関心にまとまるためである。PR は全部で 6 つになる。
-   **フェーズ 6 の構造の抽出（`ParseStructure`）を、先に別の PR にする（レビュアーが選んだ）。** README の翻訳のレビューと、コードのレビューを分けるためである。`ParseStructure` は、README の版やロケータに依存せず、単独で表のテストを持てる。
-   **フェーズ 5 では、英語のまま移してから翻訳する（レビュアーが選んだ）。** 英語のまま移したコミットで、今の README のどの文も README・`cli.md`・`development.md` のどれかに残っていること（AC-22）を `_checks/` のスクリプトで機械的に確かめ、その後のコミットで `cli.md` と `development.md` を日本語にする。AC-22 の読み合わせを、機械の確認で補うためである。
-   **英語から日本語への翻訳の手段（H-13）。** `pricing_update.md`・`claude_output_limit.md`・`cli.md`・`development.md`・`README.md` の日本語は、実装するセッションが直接書く。書いた後に `/japrose` で整え、翻訳の正確さは `.claude/commands/_lib/review-subagent-pattern.md` のレビューで、`mktrans.md` の Accuracy checklist を基準として確かめる。翻訳の元は、そのコミットの前の英語の文書である。改訂前の `/mktrans` は使わない（`README.ja.md` を出力するため。設計書 8 章）。
-   **差分の検査のスクリプトは Go で書き、PR ごとにディレクトリを分ける。** `_checks/move/`（PR-1）、`_checks/split/`（PR-4）、`_checks/readme/`（PR-6）である。Go は開発の環境と CI で必ず使え、0040 の `_checks/main.go` に前例がある。1 つのディレクトリには 1 つの `main` パッケージしか置けないので、分ける。
-   **`has-extension-changes.sh` に `docs/dev/guides/development.md` を加えるのは、フェーズ 5 で行う。** `serverReasons.test.ts` がフェーズ 5 から `development.md` を読むので、同じ PR で判定にも加えないと、その間に `development.md` だけを変えた PR で拡張のジョブが走らない。`README.en.md` の行は、そのファイルを作るフェーズ 6 で加える。設計書 8 章のフェーズ 6 の「CI の判定」のうち、`development.md` の行だけを前に出す。
-   **Browser extension の節を最上位（`##`）にするのは、フェーズ 6（PR-6）で行う。** 節の中の `###` の小節で節が切れないよう、`markdownSection` を見出しのレベルに合わせて節を終える形に改める（設計書 3.5）。この改修は PR-5 に入れ、PR-6 の前に済ませる。今の節は `###` なので、改修の前後で読む範囲は変わらない。
-   **AC-26 の参照先の付け替えは、節が移る PR で行う。** `pricing_update.md`・`claude_output_limit.md` が参照する README の節のうち、`cli.md` に移るもの（Token usage、Estimated cost の CLI の行、Output format の `--out` の小節）は PR-4 で `cli.md` の見出しに、README に残るもの（Configuration など）は PR-6 で日本語の見出しにする。`project_overview.md:157`・`production_profile.md:5`・`translation_glossary.md:53` は README に残る節を指すので、PR-6 で日本語の見出しにする。PR-1 の翻訳では、README の今の英語の節の名前をそのまま参照する。
-   **CLAUDE.md で、まだないファイルにリンクを張らない。** PR-2 で CLAUDE.md に書く `README.en.md` などは、バッククォートで名前を書くだけにする。PR-1 で加えるパスの検査（ステップ 1-5）が、存在しないファイルへのリンクで失敗するためである。`cli.md`・`development.md`・`README.en.md` へのリンクと、AC-07・AC-41 の記述は PR-6 で書く（設計書 8 章のフェーズ 2 の注）。
-   **`doctestutil` の補助関数の最終の一覧** は、1.4 の呼び出し元の数で決めた。

### 1.4. 既存コード調査結果

**旧パスの参照（設計書 6.2）。** `c76eec26` で `git grep -c developer_guide`（本タスクのディレクトリを除く）は 117 ファイル・343 行である。タスク文書（本タスクを除く）は 93 ファイルで、設計書の 92 ファイルに `docs/tasks/0049_codex_local_review/observations.md` が加わった。タスク文書以外は、`.claude/commands/` の 11 ファイル（`_context.md`・`fixpr.md`・`mergepr.md`・`mkarch.md`・`mkplan.md`・`mkplan2.md`・`mkreq.md`・`mktrans.md`・`runplan.md`・`runpr.md`・`weakreview.md`）、`AGENTS.md`、`CLAUDE.md`、`Makefile`、`README.md`、`cmd/fixprguard/docs_test.go`、`cmd/fixprguard/main.go`、`docs/dev/project_overview.md`、`internal/config/config.go`、`internal/llm/claude/request.go`、`internal/llm/provider/provider.go`、`internal/llm/provider/provider_test.go`、`internal/pipeline/pipeline_test.go`、`internal/pricing/rates.go` と、移動する 11 ファイルどうしの参照である。

**指示と規約の文書の今のパス。** CLAUDE.md・AGENTS.md・`.claude/commands/`・`docs/dev/` の 35 の `.md` ファイルについて、バッククォートの中の `docs/dev/…md` と相対リンクの先を `c76eec26` で走査した。存在しない先は、コードブロックの中の `CLAUDE.md:501`（Go のコード例の `](err)`）だけだった。設計書 4.2 の検査はコードブロックの中を対象にしないので、今の文書は検査に通る見込みである（走査は本書の作成のための一時的なスクリプトで行った。検査そのものはステップ 1-5 のテストである）。

**補助関数の重複と呼び出し元。** `cmd/yt2column-server/docs_test.go` の `readDoc`・`markdownTables`・`splitTableRow`・`isTableSeparator`・`findTable`・`tableRow`・`stripBackticks`・`headingLevel`・`sectionByHeading`（`:370-522`）は、`cmd/yt2column/docs_test.go` の同名の関数と本体が一字一句同じである（`awk` で関数を切り出して比べた）。呼び出しの数（定義を除く、`*_test.go` の中）は次のとおりである。

| 関数 | `cmd/yt2column` | `cmd/yt2column-server` | 移す先 |
|---|---|---|---|
| `readDoc` | 45 | 3 | `ReadDoc` |
| `sectionByHeading` | 8 | 6 | `SectionByHeading` |
| `markdownTables`・`findTable` | 3・3 | 2・2 | `MarkdownTables`・`FindTable` |
| `tableRow` | 1 | 1 | `TableRow`（設計書 3.3 の一覧にないが、両方にあるので移す） |
| `stripBackticks`・`headingLevel` | 5・5 | 6・1 | `StripBackticks`・`HeadingLevel`（両方が直接呼ぶので公開する） |
| `anchorFromHeading`・`headingsOutsideBlocks`・`sectionOutsideBlocks` | 3・2・9 | 0 | `AnchorFromHeading`・`HeadingsOutsideBlocks`・`SectionOutsideBlocks`（設計書 3.3） |
| `scanDocLines`（型 `docLine`） | 10 | 0 | `ScanDocLines`・`DocLine`（残る関数 `markdownLinkTargets`・`numberedItems`・`TestReadmeLinks`・`output_layout_docs_test.go` が直接使い、`ParseStructure` も使うので公開する） |
| `fencedBlocks`（型 `fencedBlock`） | 3 | 0 | `FencedBlocks`・`FencedBlock`（設計書 3.3 のとおり文書の文字列を受け取る形にする。今の 3 つの呼び出し元はどれも `slices.Collect(strings.Lines(…))` を渡している） |
| `splitTableRow`・`isTableSeparator`・`isClosingFence` | 1・1・1 | 1・1・0 | 公開しない（移した関数だけが使う） |
| `markdownLinkTargets`・`headingAnchors`・`numberedItems`・`firstCodeSpan`・`tableRowsWithFirstCell`・`tableColumn`・`containsToken` | 4・2・3・2・3・1・4 | 0 | 移さない（`cmd/yt2column` だけが使う） |
| `tablesByFirstHeader`・`hasLineContainingAll`・`hasCurlCommand` | 0 | 1・-・- | 移さない（サーバのパッケージだけが使う） |

`cmd/yt2column-nmhost/main_test.go:436` の `TestReadmeHostExitCodes` は補助関数を使わず、`bufio.Scanner` で README を読む。フェーズ 6 で両版を読むときに `doctestutil` を使う形にする。

**補助関数のテスト。** `cmd/yt2column/docs_test.go` の `TestSectionByHeading`（`:347`）・`TestScanDocLines`（`:594`）・`TestFencedBlocks`（`:615`）・`TestAnchorFromHeading`（`:1154`）・`TestSectionOutsideBlocks`（`:1190`）を `doctestutil` に移す。`TestNumberedItems`（`:1175`）と `output_layout_docs_test.go:137` の `TestFirstCodeBlockAfter` は、残る関数のテストなので残す。`cmd/yt2column-server/docs_test.go:213` の `TestSectionByHeading` は、コピーとの一致を固定するテストなので消す（設計書 3.3）。

**テスト専用のパッケージ。** `internal/pipeline/pipeline_test.go:583` の `testOnlyPackageDirs` に 4 つのパッケージがある。`TestFakesCarryBuildTag` は、そこに挙げたディレクトリの `_test.go` でないファイルに `//go:build test` を求める。`TestPackageReferenceListsPackages`（`:671`）は、`_test.go` でない Go のファイルを持つパッケージの行を `package_reference.md` に求め、文書を `:707` のパスで読む。

**UI が示すパス。** `cmd/fixprguard/main.go:132` の `guidePath` は `:268` の診断で使う。CLI のヘルプは `cmd/yt2column/run.go:225`（`--slack`、「see README」）と `:259`（「see README.md」）で、`run_test.go:1261` の `TestRunHelp` がヘルプの出力を検査している。サーバの診断は `cmd/yt2column-server/run.go:145` で、`run_test.go:39` の `TestStartupFailures` が `takes no arguments` を検査している（`:60`）。拡張の英語のメッセージは `extension/src/core/i18n/en.ts:83`、日本語は `ja.ts:81` である。

**拡張のテストの補助。** `extension/test/helpers/repoFile.ts:15` の `markdownSection` は、次のレベル 1〜3 の見出しで節を終える。`extension/test/markdownSection.test.ts` がその表のテストを持つ。`metadataDocs.test.ts:25` は `"### Browser extension"` で節を探し、英語の促しのメッセージだけを比べる。`serverReasons.test.ts:15-17` は README の `reason | status code | extension shows` の表を探す。`ciChanges.test.ts:21-30` の走らせる表には `README.md` があり、走らせない表には `docs/README.md`・`docs/dev/security.md` などがある。

**`fixprguard` の範囲の判定（設計書 2.5）。** 設計書のとおり、変更は要らない。`.claude/commands/fixpr.md` の同期の手順は `git restore` を使いうるが、`cmd/yt2column/fixpr_docs_test.go:971` が禁じるのは `git add -A`・`git commit -m` などで、`git restore` は含まない。同期の手順は Phase 4.5 の外で `git push` を書かない（同 `:984`）。

**同時に進むタスク。** タスク 0046 は PR-3 と PR-4（README のポップアップの説明を変える）が未マージである（`docs/tasks/0046_popup_redesign/03_implementation_plan.md:329`・`:390`）。タスク 0049（Codex CLI によるローカルのレビュー）は、要件書が `_lib/task-pr-handoff.md`・`runplan.md`・`fixpr.md`・`runpr.md` を変える対象に挙げている（`docs/tasks/0049_codex_local_review/01_requirements.md`）。

**本番のコードの型の構築の経路。** 本タスクは具体的な型で振る舞いを分けない。`doctestutil.Lang` の零値の扱いは `Locator.In` が失敗させる（設計書 3.3）。テスト専用のパッケージなので、本番のコードからの構築の経路はない。

**`implementation_handoff.md`。** ない。`design_handoff.md` の H-01〜H-13 は、どれも設計書 10 章に記録がある。

### 1.5. 名前の対応表

本タスクで名前が変わるものは次のとおりである。ほかの節は、この表の新しい名前を使う。

| 旧 | 新 |
|---|---|
| `docs/dev/developer_guide/` の 11 ファイル | 設計書 6.2 の対応表のとおり |
| `cmd/yt2column/docs_test.go` の `readDoc`・`anchorFromHeading`・`headingsOutsideBlocks`・`sectionByHeading`・`sectionOutsideBlocks`・`markdownTables`・`findTable`・`tableRow`・`stripBackticks`・`headingLevel`・`scanDocLines`・`docLine`・`fencedBlocks`・`fencedBlock` | `internal/doctestutil` の `ReadDoc`・`AnchorFromHeading`・`HeadingsOutsideBlocks`・`SectionByHeading`・`SectionOutsideBlocks`・`MarkdownTables`・`FindTable`・`TableRow`・`StripBackticks`・`HeadingLevel`・`ScanDocLines`・`DocLine`・`FencedBlocks`・`FencedBlock` |
| `cmd/yt2column-server/docs_test.go` の上の 9 つのコピー | なくす |
| `cmd/fixprguard/docs_test.go` の `guideFiles`（2 ファイル） | 1 ファイル（`docs/dev/guides/fixprguard_guide.md`）。変数の名前は実装で決める |

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: developer_guide の分割

#### ステップ 1-1: 英語版の手引きと日本語版の手引きの読み比べ（AC-11）

**変更するファイル:** `docs/dev/developer_guide/mergepr_guide.ja.md`・`fixprguard_guide.ja.md`（英語版にだけある記述があれば）、本書

-   [ ] `mergepr_guide` と `fixprguard_guide` の英語版と日本語版を、節ごとに読み比べる（設計書 7.3 の事実を出発点にする）。
-   [ ] 英語版にだけある記述を、日本語版に加える。なければ加えない。
-   [ ] 結果（節ごとの対応、加えた記述とその場所、`mergepr_guide` の 1 行の差の理由）を 7 章の「読み比べの記録」に書く。

**完了の条件:** 記録が 7 章にある。加えた記述は、ステップ 1-6 の AC-09 の検査が差分の行として一覧に出す。

#### ステップ 1-2: ファイルの移動

**変更するファイル:** `docs/dev/developer_guide/` の 11 ファイル

-   [ ] 設計書 2.2・6.2 のとおり、`git mv` で移す。`mergepr_guide.ja.md`・`fixprguard_guide.ja.md` は `docs/dev/guides/` の `.ja` を外した名前にする。英語版の `mergepr_guide.md`・`fixprguard_guide.md` は `git rm` で消す。`pricing_update.md`・`claude_output_limit.md` は英語のまま `docs/dev/guides/` に移す（翻訳はステップ 1-4）。
-   [ ] 内容を変えずに、このステップだけでコミットする（このコミットではテストが失敗してよい。次のステップで直す）。

#### ステップ 1-3: 旧パスの置換

**変更するファイル:** 1.4 の「旧パスの参照」のすべてのファイル、`docs/tasks/0032_japanese_pr_title/check_rules.sh`

-   [ ] 設計書 6.2 の対応表で、ファイル名を含む旧パスを新しいパスに置き換える。相対リンクはディレクトリの部分だけを置き換える。ディレクトリだけを指す記述（設計書 6.2 の 6 か所と、`CLAUDE.md:25`・`mktrans.md:21`）は置き換えない。`CLAUDE.md:25`・`mktrans.md:21` は PR-2 で書き直す。
-   [ ] `cmd/fixprguard/main.go:132` の `guidePath` を `"docs/dev/guides/fixprguard_guide.md"` にする（AC-16）。
-   [ ] `cmd/fixprguard/docs_test.go:27-30` の `guideFiles` を 1 つのファイルにし、英語の手引きの印を使う行（`"**Execution-related files.**"` など）を消す（AC-17、設計書 3.7）。消した行について、`go tool cover -func` の関数ごとの結果が変わらないことを確かめる。
-   [ ] `internal/pipeline/pipeline_test.go:707` のパスを `docs/dev/conventions/package_reference.md` にする（AC-17）。
-   [ ] 置換の後、`git grep -n developer_guide` の残りが、本タスクの文書、ディレクトリだけを指す記述、`CLAUDE.md:25`・`mktrans.md:21` だけであることを確かめる（AC-13 の検査そのものはステップ 1-6）。

#### ステップ 1-4: `pricing_update.md`・`claude_output_limit.md` の翻訳（AC-10）

**変更するファイル:** `docs/dev/guides/pricing_update.md`・`claude_output_limit.md`

-   [ ] 1.3 の手段で、全文を日本語に翻訳する。節の構成、手順、値、コマンドの例は変えない。README の節の参照は、今の英語の節の名前のまま訳す（付け替えは PR-4・PR-6。1.3）。
-   [ ] 新しい用語があれば、用語集（`docs/translation_glossary.md`）に加える。
-   [ ] `/japrose` で整え、翻訳のレビューで指摘を直す。翻訳のコミットは、移動と置換のコミットと分ける。

#### ステップ 1-5: パスの検査のテスト（AC-15・AC-16）

**変更するファイル:** `cmd/yt2column/docpaths_test.go`（新規）、`cmd/fixprguard/docs_test.go`

-   [ ] 設計書 4.2 の「指示と規約の文書が示すパス」のテスト `TestInstructionDocPaths` を加える。対象は CLAUDE.md・AGENTS.md・`.claude/commands/` 配下・`docs/dev/` 配下の `.md` で、相対リンクの先とバッククォートの中の `docs/dev/…md` が存在するファイルであることを確かめる。コードブロックの中と、`<…>` の形の仮の値のリンクは対象にしない。補助関数は `cmd/yt2column` の今の関数（`markdownLinkTargets`・`scanDocLines`）を使う（`doctestutil` はフェーズ 4）。
-   [ ] `guidePath` のファイルがリポジトリに存在することを確かめるテスト `TestGuidePathExists` を `cmd/fixprguard/docs_test.go` に加える（AC-16）。
-   [ ] `TestInstructionDocPaths` が、存在しない相対リンク、存在しないバッククォートのパス、の 2 つでそれぞれ失敗し、コードブロックの中の同じリンクでは失敗しないことを、一時的に文書を変えて確かめる。`TestGuidePathExists` が、`guidePath` を旧パスに戻すと失敗することを確かめる。

**完了の条件:** `go test -tags test ./cmd/yt2column ./cmd/fixprguard` が通る。

#### ステップ 1-6: 移動の差分の検査（AC-08・AC-09・AC-12〜AC-15）

**変更するファイル:** `docs/tasks/0048_doc_language_policy/_checks/move/main.go`（新規）、本書

-   [ ] merge-base と PR の先頭のコミットを比べ、設計書 7.3 の表の AC-08・AC-09（の一部）・AC-12・AC-13・AC-14・AC-15 を確かめるプログラムを書く。AC-09 では、`docs/dev/guides/` に `.ja.md` で終わるファイルがないことも確かめ、`.ja.md` の版との差分の行を一覧で出力する。AC-13 は、ファイル名を含む旧パス（設計書 6.2 の対応表の 11 ファイル）だけを探し、本タスクのディレクトリを除く。
-   [ ] 各検査が違反を見つけることを、一時的に 1 か所ずつ変えて確かめ、元に戻す（過去のタスク文書の Status の行を変える、旧パスを 1 つ残す、相対リンクの深さを変える、など）。結果をコミットのメッセージに書く。
-   [ ] リポジトリのルートで `go run ./docs/tasks/0048_doc_language_policy/_checks/move/` を実行し、終了コード、出力、2 つのコミットの ID を 7 章の「差分の検査の実行の記録」に書く。AC-09 の一覧に出た差分の行を、PR のレビューで読む行として PR の本文に挙げる。

**完了の条件:** PR-1 のブランチで、プログラムが終了コード 0 で終わる。`make test` と CI では実行しない（`requirements_process.md` §4。PR-1 のマージの後は `docs/dev/conventions/requirements_process.md` §4）。

### PR-1 作成ポイント: developer_guide split and path replacement

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6

**推奨タイトル**: `docs(0048): developer_guide を規約と手引きに分け、旧パスを置き換える`

**レビュー観点**: 英語版の手引きの読み比べの記録と、日本語版に加えた記述（AC-11） / `_checks/move` の出力（特に AC-09 の差分の行と AC-14 の結果） / `pricing_update.md`・`claude_output_limit.md` の翻訳の正確さ（AC-10） / `TestInstructionDocPaths` の対象と除外

**実装モデル要件**: standard

**判定理由**: 機械的な移動と置換が中心で、設計書が対応表と検査の内容を決めており、`mkplan.md` の Conditional checks の 2 つ以上には当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

PR-1 の本文には、次の 2 つを書く。マージの後に `make install-fixprguard` で入れ直す必要があること（AC-18。入れ直すまでは、インストール済みの `fixprguard` が旧パスを示す）。開いている PR（0046 の PR-3・PR-4、0049 の PR など）は、`main` を取り込んで旧パスを直す必要があること（設計書 8 章）。

### フェーズ 2: 言語の原則

#### ステップ 2-1: CLAUDE.md の改訂（AC-01〜AC-05・AC-40）

**変更するファイル:** `CLAUDE.md`

-   [ ] "Documents" 節に、3 つの原則（AC-01）、ディレクトリごとの言語（AC-02）、例外（AC-04・AC-04a。範囲と理由を含む）を書き、`CLAUDE.md:25` の例外の列挙を消す（設計書 1.2）。`README.en.md` は名前だけを書き、リンクにしない（1.3）。
-   [ ] "Language" 節を、原則と矛盾しない形にする（AC-03）。
-   [ ] Go Idioms の例外（`CLAUDE.md:507-516`）が Go のテストにも適用されると明記する（AC-05）。
-   [ ] "Translation Guidelines" 節を、`README.md` を先に変えて `/mktrans` で `README.en.md` に反映する流れに改める（AC-40）。

#### ステップ 2-2: `_context.md` の改訂（AC-40）

**変更するファイル:** `.claude/commands/_context.md`

-   [ ] 「Translation language pair」の行（`:54`）を、`README.md` → `README.en.md` の組だけを扱う `/mktrans` に合わせる。

**完了の条件（フェーズ 2）:** `make test` が通る（`TestInstructionDocPaths` を含む）。

### フェーズ 3: `/mktrans` の改訂と同期の手順

#### ステップ 3-1: `/mktrans` の改訂（AC-38・AC-39）

**変更するファイル:** `.claude/commands/mktrans.md`

-   [ ] 設計書 3.8 の表と 6.1 の判定の流れのとおりに改める。翻訳の組は `README.md` → `README.en.md` だけにし、`.ja.md` の方向の規則（`:27-35`）と developer_guide への言及（`:20-22`）を消す。同期点は同期の印のブロブの ID にする（設計書 2.4 の印の形と求め方）。
-   [ ] 単独と下位の手順の違い（用語集の扱い、コミット、結果の報告の 4 つの値）を書く。
-   [ ] 止まったときの復旧の手順（設計書 4.3 の表）と、`main` を取り込むときの衝突の規則（設計書 4.3）を書く。

#### ステップ 3-2: `_lib/task-pr-handoff.md` と `/runplan` の同期の手順

**変更するファイル:** `.claude/commands/_lib/task-pr-handoff.md`・`.claude/commands/runplan.md`

-   [ ] Part B の手順 1 の後、手順 2 の前に、同期の手順を加える（設計書 4.3 の `/runplan` の項と結果の表）。「README.md を変えた」の判定は、`$(git merge-base origin/main HEAD)..HEAD` のコミットと作業ツリーで行う。今の「3. Push and create the PR for the current branch.」と「4. Run AFTER_CREATE」の番号と文は変えない（`cmd/yt2column/codex_review_docs_test.go`・`fixpr_docs_test.go` が区切りに使う。設計書 3.7）。
-   [ ] `runplan.md` の手順 5a の CHECKS の説明（`:82`）を、同期のコミットがあれば green gate を走らせ直すと改める。

#### ステップ 3-3: `/fixpr` の同期の手順

**変更するファイル:** `.claude/commands/fixpr.md`

-   [ ] Phase 3 の後、Phase 3.5 の前に、このセッションが同期の手順を実行すると書く。Phase 4.5 が `README.md` を変えるコミットをしたときは、その push の前にもう一度実行すると書く（設計書 4.3 の `/fixpr` の項）。止まったときに作業ツリーに残る変更の扱いを、最終の報告の項に加える。
-   [ ] 見出し（`## Phase 3 — Fix (model: default)` など、`fixpr_docs_test.go:30-37` の定数）は変えない。

#### ステップ 3-4: `/runpr` の同期の手順と印の検査

**変更するファイル:** `.claude/commands/runpr.md`

-   [ ] 手順 1 の単独の変更の PR を作る項で、ローカルのレビューの後、push の前に同期の手順を加える。チェックアウトしていないブランチで `README.md` の変更があれば止まると書く（設計書 4.3 の `/runpr` の項）。
-   [ ] 手順 4 のマージの質問の前に、印の検査を加える（設計書 2.4。検査だけで反映しない。`README.en.md` がなければ検査しない）。
-   [ ] `main` を取り込む手順に、`README.en.md` の衝突の規則（設計書 4.3）を加える。
-   [ ] 手順の見出し（`fixpr_docs_test.go:38-40`・`codex_review_docs_test.go:83` の定数）は変えない。

**完了の条件（フェーズ 3）:** `make test` が通る（コマンドの文書を読む `cmd/yt2column/fixpr_docs_test.go`・`codex_review_docs_test.go` のテストを含む）。同期の手順は、`fixprguard` の新しい呼び出しも、Codex のメンションも書かない（設計書 5 章）。

### PR-2 作成ポイント: language principles and README sync procedure

**対象ステップ**: 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4

**推奨タイトル**: `docs(0048): 文書の言語の原則を定め、README の同期の手順を加える`

**レビュー観点**: CLAUDE.md の原則・例外と、AC-06 の表の各文書の言語の一致 / `/mktrans` の振る舞いが設計書 3.8 の表と 6.1 の流れに一致すること / 3 つのコマンドの同期の手順の位置と、止まったときの振る舞い（設計書 4.3） / `main` を取り込むときの衝突の規則

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-2〜3-4 は、`/fixpr` の Phase 3.5 と `fixprguard`、`/runpr` の手順 4 の PRE_MERGE という、ほかの手順の不変条件と交わる位置に手順を加える。読み違えると push の前の検査を飛ばしうる。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 共有パッケージ `internal/doctestutil`

#### ステップ 4-1: パッケージの新設と補助関数の移動

**変更するファイル:** `internal/doctestutil/`（新規。ファイルの分け方は実装で決める）、`cmd/yt2column/docs_test.go`

-   [ ] すべてのファイルに `//go:build test` を付けた `package doctestutil` を作る。
-   [ ] 1.5 の対応表の関数と型を移し、公開する。`splitTableRow`・`isTableSeparator`・`isClosingFence` は公開せずに移す。本体は変えない。例外は、`ReadDoc` が `testing.TB` を受け取ること、`FencedBlocks` が文書の文字列を受け取ること、`DocLine` のフィールドを公開することの 3 つである（設計書 3.3）。
-   [ ] `TestSectionByHeading`（`cmd/yt2column/docs_test.go:347`）・`TestScanDocLines`・`TestFencedBlocks`・`TestAnchorFromHeading`・`TestSectionOutsideBlocks` を移す。
-   [ ] `TestAnchorFromHeading` の表に、設計書 3.4 で確かめた日本語の見出しの行を加える（設計書 3.4 の「確かめた文字」だけを使う）。

#### ステップ 4-2: 呼び出し元の付け替えとコピーの削除

**変更するファイル:** `cmd/yt2column/*_test.go`（1.4 の呼び出し元のあるもの）、`cmd/yt2column-server/docs_test.go`

-   [ ] `cmd/yt2column` と `cmd/yt2column-server` の呼び出しを `doctestutil` の関数にする。
-   [ ] `cmd/yt2column-server/docs_test.go` の 9 つのコピー（`:370-522`）を消す。
-   [ ] `cmd/yt2column-server/docs_test.go:213` の `TestSectionByHeading` を消す。
-   [ ] 消したテストと移したテストについて、`go tool cover -func`（`-coverpkg` に `internal/doctestutil` を含める）の関数ごとの結果が、移動の前の 2 つのパッケージの結果と変わらないことを確かめ、コミットのメッセージに書く。

#### ステップ 4-3: テスト専用のパッケージとしての登録

**変更するファイル:** `internal/pipeline/pipeline_test.go`、`docs/dev/conventions/package_reference.md`

-   [ ] `testOnlyPackageDirs`（`:583`）に `internal/doctestutil` を加える。
-   [ ] `package_reference.md` に `internal/doctestutil` の行を加える（`TestPackageReferenceListsPackages` が求める）。
-   [ ] `internal/doctestutil` の 1 つのファイルから `//go:build test` を一時的に外し、`TestFakesCarryBuildTag` が失敗することを確かめる。

**完了の条件（フェーズ 4）:** `make test`・`make lint`・`make deadcode` が通る。

### PR-3 作成ポイント: shared doc test helpers

**対象ステップ**: 4-1 / 4-2 / 4-3

**推奨タイトル**: `test(0048): ガードテストの Markdown の補助関数を internal/doctestutil に移す`

**レビュー観点**: 移した関数の本体が変わっていないこと（3 つの例外を除く） / 公開する関数の一覧と呼び出し元の数（1.4） / 消したテストの網羅の比較の結果 / 日本語の見出しのアンカーの行

**実装モデル要件**: standard

**判定理由**: 本体を変えない移動で、関数の一覧と形は 1.4 と設計書 3.3 が決めており、Conditional checks のうち当たるのは build tag の 1 つだけである。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: README の振り分け

#### ステップ 5-1: 始める条件の確認

**変更するファイル:** 本書、必要なら設計書

-   [ ] 0046 の PR-4 がマージされていることを確かめる。マージされていなければ始めない（要件書 §1「進行中のタスクとの順序」）。
-   [ ] PR-4 が README のどこを変えたかを確かめる。設計書 3.6 が想定する範囲（Browser extension の節のポップアップの説明）の外を変えていれば、設計書 3.6・3.7 を作り直してから進む（設計書 3.6）。範囲の中だけなら、PR-4 の後の README の行番号と設計書 3.6 の行番号の対応を、本書の冒頭の表に加える。

#### ステップ 5-2: 英語のまま移す

**変更するファイル:** `README.md`、`docs/dev/guides/cli.md`・`development.md`（新規）、README を読むガードテスト（4.2 の表で PR-4 の行）、`extension/test/serverReasons.test.ts`、`extension/scripts/has-extension-changes.sh`、`extension/test/ciChanges.test.ts`

-   [ ] 設計書 3.6 の表と表 3-2 のとおり、移す文と節を、英語のまま、文言を変えずに `cli.md` と `development.md` に移す。README には、移った先への短い案内を残す。移る文の中のリンクは、設計書 3.6 の「リンクの付け替え」のとおりにする（AC-25）。
-   [ ] 4.2 の表のうち PR-4 の行のテストを、移った先のファイルを読むようにする。この時点では見出しと列名は英語のままである。
-   [ ] `TestReadmeLinks` を、README・`cli.md`・`development.md` の 3 つに広げ、文書をまたぐアンカーの検査を加える（設計書 4.2）。`README.en.md` はステップ 6-6 で加える。
-   [ ] `serverReasons.test.ts` を、`development.md` の表を読むようにする。
-   [ ] `has-extension-changes.sh` と `ciChanges.test.ts` に `docs/dev/guides/development.md` を加える（1.3）。走らせない表に `docs/dev/guides/cli.md` を加える（設計書 7.1）。
-   [ ] このステップだけでコミットする（以下「英語の移動のコミット」）。

#### ステップ 5-3: 振り分けの差分の検査（AC-22）

**変更するファイル:** `docs/tasks/0048_doc_language_policy/_checks/split/main.go`（新規）、本書

-   [ ] merge-base の `README.md` の文・表の行・コードブロックの行が、英語の移動のコミットの README・`cli.md`・`development.md` のどれかにあることを確かめるプログラムを書く。文の単位で比べる（段落の折り返しは比較の前に戻す）。リンクの先は比較から外す（ステップ 5-2 で付け替えるため）。見つからない文を一覧で出力する。
-   [ ] プログラムが、1 つの文を消したとき、表の 1 行を消したときに、それぞれ見つけることを一時的に変えて確かめる。
-   [ ] `go run ./docs/tasks/0048_doc_language_policy/_checks/split/ <merge-base> <英語の移動のコミット>` の形で実行し、出力と 2 つのコミットの ID を 7 章の記録に書く。

**完了の条件:** 終了コードが 0 である。

#### ステップ 5-4: `cli.md`・`development.md` の翻訳

**変更するファイル:** `docs/dev/guides/cli.md`・`development.md`、`internal/doctestutil/`、ステップ 5-2 で変えたガードテスト、`extension/test/helpers/`・`serverReasons.test.ts`、`docs/dev/guides/pricing_update.md`・`claude_output_limit.md`、`docs/translation_glossary.md`

-   [ ] 1.3 の手段で、`cli.md` と `development.md` を日本語にする。翻訳の元は英語の移動のコミットの 2 つのファイルである（AC-24）。
-   [ ] 2 つの文書の見出しと列名を、日本語の文字列の定数として `doctestutil` に置く（設計書 3.3）。拒否の理由の表示の区分の表（区分ごとの日本語と英語の書き方。設計書 3.7 の末尾）も置く。ガードテストは、これらの定数で節と表を探す。
-   [ ] `rejection_reasons` のサブテストを設計書 3.7 の行のとおりにする。この時点の README は英語の 1 版なので、README の 2 列の表は英語の書き方の区分で読む。
-   [ ] `extension/test/helpers/` に `development.md` の拒否の理由の表の列名と区分の書き方を置き、`serverReasons.test.ts` がそれを使う（設計書 3.5）。
-   [ ] `pricing_update.md`・`claude_output_limit.md` の参照のうち、`cli.md` に移った節を指すものを、`cli.md` の見出しにする（AC-26、1.3）。
-   [ ] 4.2 の表の PR-4 の行の各テストが、移った先の値を壊すと失敗することを確かめる（例：`cli.md` のフラグの表から 1 行消す、`development.md` の Job API の表の 1 つのコードを変える）。

**完了の条件（フェーズ 5）:** `make test`・`make lint`・`make ext-check` が通る。

### PR-4 作成ポイント: README split into developer documents

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4

**推奨タイトル**: `docs(0048): README の開発者向けの内容を cli.md と development.md に移す`

**レビュー観点**: 振り分けが設計書 3.6 の表と表 3-2 に一致すること（AC-19〜AC-22） / `_checks/split` の出力 / `cli.md`・`development.md` の翻訳の正確さ（AC-24） / ガードテストの付け替えが 4.2 の表の PR-4 の行に一致すること（AC-32）

**実装モデル要件**: frontier-recommended

**判定理由**: 文の単位の振り分けは判断が要り（設計書 3.6）、ガードテストの付け替えと CI の判定の変更（Conditional checks の CI の判定）が同時に入る。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: README の日英二言語化

#### ステップ 6-1: 構造の抽出と、`markdownSection` の改修

**変更するファイル:** `internal/doctestutil/`、`extension/test/helpers/repoFile.ts`、`extension/test/markdownSection.test.ts`

-   [ ] 設計書 3.3 の `HeadingPos`・`Structure`・`ParseStructure` と、4.1 の 3 つの番兵のエラーを加える。走査は `ScanDocLines` を使い、規則を変えない。拒否する形は設計書 4.1 のとおりである。
-   [ ] `ParseStructure` の表のテストを加える。行は設計書 7.1 の「`ParseStructure` 自体のテスト」のとおりで、拒否する形の行は `errors.Is` で番兵を確かめる。
-   [ ] `markdownSection` を、見出しのレベルと同じか高いレベルの次の見出しで節を終える形に改める（1.3）。`markdownSection.test.ts` に、`##` の節の中の `###` で切れない行と、次の `##` で終わる行を加える。今の `###` の行は変えずに通す。

**完了の条件:** `make test`・`make lint`・`make deadcode`・`make ext-check` が通る。

### PR-5 作成ポイント: README structure parser

**対象ステップ**: 6-1

**推奨タイトル**: `test(0048): README の構造の抽出と markdownSection の節の終わり方を加える`

**レビュー観点**: 拒否する形が設計書 4.1 に一致し、どれも番兵で返ること / 走査の規則を変えていないこと / `markdownSection` の改修の前後で今の `###` の節の範囲が変わらないこと

**実装モデル要件**: standard

**判定理由**: 型とエラーと拒否する形は設計書 4.1 が決めており、新しい判断は要らない。Conditional checks にも当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 6-2: `README.md` の日本語化

**変更するファイル:** `README.md`

-   [ ] 1.3 の手段で、`README.md` を日本語にする。翻訳の元はこの PR の merge-base の `README.md` である。内容を加えも削りもしない（AC-27 の例外を除く）。AC-27a の書き換えは、設計書 3.6 の「CLI を主語や比較の対象にした文」の一覧のとおりにする。
-   [ ] ポップアップの文言は、日本語のカタログ（`ja.ts`）の文言を引用し、引用の言語の説明を日本語の版に合わせる（AC-31a）。
-   [ ] Browser extension の節を最上位（`##`）にする（AC-19）。
-   [ ] 先頭（最初の見出しより前）に、`README.en.md` へのリンク（AC-28）と、`cli.md`・`development.md` へのリンク（AC-23）を置く。
-   [ ] 見出しには、設計書 3.4 で確かめた文字だけを使う。使えないときは、設計書 3.4 の方法で確かめ、結果を 7 章に記録する。

#### ステップ 6-3: 用語集（AC-34）

**変更するファイル:** `docs/translation_glossary.md`

-   [ ] AC-34 の (1)〜(3) のうち、用語集にないものを加える。(3) の候補は、`README.md` の 2 つ以上の節の地の文に現れる、概念を表す語である。加えた語と、候補から外した語とその理由を、PR の本文に挙げる。

#### ステップ 6-4: `README.en.md` の作成

**変更するファイル:** `README.en.md`（新規）、`docs/translation_glossary.md`

-   [ ] 改訂した `/mktrans` を単独で `README.md` に対して呼び、全文を翻訳させる。1 行目に同期の印、先頭に `README.md` へのリンクがあることを確かめる（AC-27・AC-28）。`/mktrans` のコミットの手順（翻訳と用語集を別のコミット）に従う。
-   [ ] ポップアップの文言が英語のカタログ（`en.ts`）の文言であることを確かめる（AC-31a）。

#### ステップ 6-5: README の版とロケータ

**変更するファイル:** `internal/doctestutil/`、`extension/test/helpers/`

-   [ ] 設計書 3.3 の `Lang`・`ReadmeVersion`・`ReadmeVersions`・`Locator`・`In` と、README のロケータの表を加える。ロケータは、4.2 の表で「両版」を読むテストが使う見出しと列名である。
-   [ ] `Locator.In` のテストを加える（`LangUnknown` と空の綴りで失敗すること）。失敗させるテストは、`testing.TB` の偽物で確かめる。
-   [ ] `extension/test/helpers/` に、README の版（パスと `Locale`）と、Browser extension の節の見出しの英日の組を置く（設計書 3.5）。

#### ステップ 6-6: ガードテストを両版に広げる（AC-31・AC-32）

**変更するファイル:** 4.2 の表で PR-6 の行のテストのファイル、`cmd/yt2column-nmhost/main_test.go`、`extension/test/metadataDocs.test.ts`

-   [ ] 4.2 の表の PR-6 の行のテストを、`ReadmeVersions()` で回す版ごとのサブテスト（`t.Run` の名前に版のパス）にする。`TestReadmeHostExitCodes` は `doctestutil` を使う形にする。
-   [ ] `TestReadmeLinks` に `README.en.md` を加える。
-   [ ] `metadataDocs.test.ts` を、`README.md` は日本語のカタログ、`README.en.md` は英語のカタログと比べるようにする（AC-31a、設計書 3.5）。0046 の PR-4 が加えた確認も同じにする。
-   [ ] 各テストが、片方の版だけで値を壊したときに、その版の名前で失敗することを確かめる（例：`README.en.md` の拡張の ID を変える、`README.md` のホストの終了コードの表から 1 行消す）。

#### ステップ 6-7: 構造の一致・同期の印・先頭のリンクのテスト（AC-23・AC-28・AC-29）

**変更するファイル:** `cmd/yt2column/readme_structure_test.go`（新規）

-   [ ] `TestReadmeVersionsShareStructure` を加える。両版の `ParseStructure` を比べ、設計書 4.1 の 3 つの場合に、そこに書いた内容を示して失敗する。ファイルがないときはスキップしない。
-   [ ] `TestReadmeSyncMarker` を加える。`README.en.md` の 1 行目が同期の印の形で、印が 1 つだけであることを確かめる（設計書 4.1 の末尾）。
-   [ ] `TestReadmeVersionLinks` を加える。両版の最初の見出しより前に、もう一方の版へのリンクがあり、両版に `cli.md`・`development.md` へのリンクがあることを確かめる（設計書 7.1）。
-   [ ] 設計書 7.1 の例（`README.en.md` の見出しを 1 つ消す、`README.md` だけにコードブロックを加える、`README.en.md` を消す）と、印を消す、印を 2 つにする、先頭のリンクを消す、でそれぞれ失敗することを確かめる。

#### ステップ 6-8: CI の判定と UI の参照先（AC-33・AC-35・AC-35a・AC-36）

**変更するファイル:** `extension/scripts/has-extension-changes.sh`、`extension/test/ciChanges.test.ts`、`cmd/yt2column/run.go`・`run_test.go`、`cmd/yt2column-server/run.go`・`run_test.go`、`extension/src/core/i18n/en.ts`、`extension/test/metadataDocs.test.ts`

-   [ ] `has-extension-changes.sh` に `README.en.md` を加え、先頭のコメントを読む文書の今の一覧に合わせる。`ciChanges.test.ts` の走らせる表に `README.en.md` を加える。
-   [ ] `run.go:225` を、`(see README)` が `(see docs/dev/guides/cli.md)` になる形に、`:259` を、`see README.md.` が `see docs/dev/guides/cli.md.` になる形にする。`TestRunHelp` に、ヘルプが `docs/dev/guides/cli.md` を示し、そのファイルが存在することの検査を加える（AC-36）。
-   [ ] `cmd/yt2column-server/run.go:145` を、`(see README.md)` が `(see README.en.md)` になる形にする。`TestStartupFailures` の引数の行で、診断が `README.en.md` を示し、そのファイルが存在することを確かめる（AC-35a）。
-   [ ] `en.ts:83` を、`the permissions the README describes.` が `the permissions README.en.md describes.` になる形にする（文全体の最終の形は、英語の文として実装で整える）。`ja.ts:81` は変えない。英語のカタログのこのメッセージが `README.en.md` を含み、そのファイルが存在することを確かめるテストを `metadataDocs.test.ts` に加える（AC-35）。
-   [ ] 各テストが、文字列を元に戻すと失敗することを確かめる。

#### ステップ 6-9: 文書の参照と CLAUDE.md（AC-07・AC-26・AC-41）

**変更するファイル:** `CLAUDE.md`、`docs/dev/project_overview.md`、`docs/dev/production_profile.md`、`docs/translation_glossary.md`、`docs/dev/guides/pricing_update.md`・`claude_output_limit.md`

-   [ ] `CLAUDE.md:212-213` を、サーバの API は `development.md`、起動は README に書く形にする（AC-07）。
-   [ ] Development Notes の文書を読むガードテストの説明（`CLAUDE.md:431-448`）を、4.2 の表の後の状態に合わせる。`TestReadmeHostExitCodes` が両版を読み、`doc-guard` では走らないことを書く（AC-41）。
-   [ ] `project_overview.md:157`、`production_profile.md:5`、`translation_glossary.md:53`、`pricing_update.md`・`claude_output_limit.md` の README に残る節への参照を、日本語の見出しにする（AC-26）。過去のタスク文書以外で README の節を名前で参照する箇所をほかにも探し（`git grep -n README` を `docs/tasks/` を除いて読む）、同じにする。

#### ステップ 6-10: 日本語のカタログの差分の検査（AC-35）

**変更するファイル:** `docs/tasks/0048_doc_language_policy/_checks/readme/main.go`（新規）、本書

-   [ ] merge-base と PR の先頭のコミットで、`extension/src/core/i18n/ja.ts` が同じであることを確かめるプログラムを書く。違うときは差分の行を出力して失敗する。一時的に `ja.ts` を変えて失敗することを確かめる。
-   [ ] 実行し、出力と 2 つのコミットの ID を 7 章の記録に書く。

**完了の条件（フェーズ 6）:** `make test`・`make test-parse-norace`・`make lint`・`make deadcode`・`make ext-check` が通る。7 章のすべての行が確かめられている。

### PR-6 作成ポイント: bilingual README and guard tests

**対象ステップ**: 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 6-9 / 6-10

**推奨タイトル**: `docs(0048): README を日本語の正本と英語版の二言語にする`

**レビュー観点**: `README.md` の翻訳の正確さと AC-27a の書き換え（AC-27） / `README.en.md` と用語集の一致（AC-34） / ガードテストが 4.2 の表のとおり両版を検査すること（AC-31・AC-32） / 構造の一致のテストの失敗のメッセージ（AC-29）

**実装モデル要件**: frontier-recommended

**判定理由**: 2 つの翻訳と十数のガードテストの変更が 1 つの PR に入り、Conditional checks のうち CI の判定とテストの build tag の 2 つに当たる。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序とマイルストーン

フェーズは設計書 8 章の順序に従う。フェーズ 2 はフェーズ 1 の新しいパスを書く。フェーズ 5 は 0046 の PR-4 のマージとフェーズ 4 の後に始める。フェーズ 6 はフェーズ 3（改訂した `/mktrans`）とフェーズ 5 の後に始める。

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1（PR-1） | `docs/dev/conventions/`・`docs/dev/guides/`、旧パスの置換、パスの検査のテスト | PR-1 のマージ。`docs/dev/developer_guide/` がない |
| M2（PR-2） | CLAUDE.md の原則、改訂した `/mktrans`、3 つのコマンドの同期の手順 | PR-2 のマージ。`README.en.md` はまだないので、同期の手順は何もしない |
| M3（PR-3） | `internal/doctestutil` | PR-3 のマージ |
| M4（PR-4） | `cli.md`・`development.md`、README は英語の 1 版 | PR-4 のマージ |
| M5（PR-5） | `ParseStructure`、`markdownSection` の改修 | PR-5 のマージ |
| M6（PR-6） | 日本語の `README.md`、`README.en.md`、両版のガードテスト | PR-6 のマージ。7 章のすべての行が確かめられている |

PR-1・PR-2・PR-3 は 0046 の PR-4 を待たずに進めてよい（要件書 §1）。PR-3 はフェーズ 1〜3 に依存しないが、PR-1 の後に始めると、パスの置換と衝突しない。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 | developer_guide の分割、旧パスの置換、2 つの手引きの翻訳、パスの検査のテスト、移動の差分の検査 | standard |
| PR-2 | 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4 | CLAUDE.md と `_context.md` の言語の原則、`/mktrans` の改訂、同期の手順 | frontier-recommended |
| PR-3 | 4-1 / 4-2 / 4-3 | `internal/doctestutil` の新設と補助関数の移動 | standard |
| PR-4 | 5-1 / 5-2 / 5-3 / 5-4 | README の振り分け、`cli.md`・`development.md`、移った値を読むガードテスト | frontier-recommended |
| PR-5 | 6-1 | `ParseStructure`、`markdownSection` の改修 | standard |
| PR-6 | 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 6-9 / 6-10 | `README.md` の日本語化、`README.en.md`、両版のガードテスト、構造の一致のテスト、UI の参照先 | frontier-recommended |

## 4. テスト戦略 (Test Strategy)

### 4.1. 加える・変えるテスト

| テスト | ステップ |
|---|---|
| `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（新規） | 1-5 |
| `cmd/fixprguard/docs_test.go::TestGuidePathExists`（新規）、同ファイルの手引きを読む各テスト（1 ファイルにする） | 1-3・1-5 |
| `internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`（パス）・`testOnlyPackageDirs` | 1-3・4-3 |
| `internal/doctestutil` の移したテスト（`TestSectionByHeading`・`TestScanDocLines`・`TestFencedBlocks`・`TestAnchorFromHeading`・`TestSectionOutsideBlocks`） | 4-1 |
| `internal/doctestutil` の `TestParseStructure`・`TestLocatorIn`（新規） | 6-1・6-5 |
| 4.2 の表のガードテスト | 5-2・5-4・6-6 |
| `cmd/yt2column/readme_structure_test.go` の `TestReadmeVersionsShareStructure`・`TestReadmeSyncMarker`・`TestReadmeVersionLinks`（新規） | 6-7 |
| `cmd/yt2column/run_test.go::TestRunHelp`、`cmd/yt2column-server/run_test.go::TestStartupFailures` | 6-8 |
| `extension/test/markdownSection.test.ts` | 6-1 |
| `extension/test/serverReasons.test.ts` | 5-2・5-4 |
| `extension/test/metadataDocs.test.ts` | 6-6・6-8 |
| `extension/test/ciChanges.test.ts` | 5-2・6-8 |

テストの名前は、新しく加えるものを本書で決めた。既存のテストの名前は変えない。

### 4.2. ガードテストの前後の表（AC-32）

本書の作成時に README を読むガードテストが検査する値と、本タスクの後にその値を検査する文書である（設計書 3.7 をもとに、値を加えた）。「PR」の列は、読む文書を変える PR である。PR-4 の行は、PR-4 の時点では英語の 1 版の README を読み、PR-6 で両版に広げるものを含む。

| テスト | 今 README で検査する値 | 本タスクの後に検査する文書 | PR |
|---|---|---|---|
| `TestREADMEDocumentsFlags`（`cmd/yt2column/docs_test.go:173`） | フラグの表の、`newFlagSet` のすべてのフラグ | `cli.md` | PR-4 |
| `TestSlackDocsContract`（`:191`） | `16,383` と、Webhook の統合テストの変数・`make` のターゲット | `16,383` は両版、統合テストの値は `development.md` | PR-4・PR-6 |
| `TestClaudeDocsContract`（`:239`） | Claude の統合テストの変数 | `development.md` | PR-4 |
| `TestDeepSeekDocsContract`（`:261`） | 3 つの統合テストの節の変数とターゲット | `development.md` | PR-4 |
| `TestLLMDefaultsDocsContract`（`:302`） | 設定の表の `Variable`・`When unset` の列の値 | 両版（日本語の列名は `変数`・`未設定のとき`） | PR-6 |
| `TestProductionProfileDocsContract`（`:934`） | Recommended configuration の設定のブロックと表 | 両版 | PR-6 |
| `TestMigrationDocsContract`（`:976`） | Migrating の旧変数と新しい変数の対応 | 両版 | PR-6 |
| `TestQuickStartLinks`（`:1036`） | Quick start の順序と、各項目の README の見出しへのリンク | 両版 | PR-6 |
| `TestProductionProfileLinks`（`:1079`） | Recommended configuration の `production_profile.md` へのリンク | 両版（README 以外は今のまま） | PR-6 |
| `TestReadmeLinks`（`:1119`） | すべてのリンクの先とアンカー | README・`cli.md`・`development.md`（PR-4）、`README.en.md`（PR-6） | PR-4・PR-6 |
| `TestTokenUsageDocsContract`（`:1255`） | Token usage の `tokens:` の行の例 | `cli.md` | PR-4 |
| `TestCostDocsContract`（`:1313`） | Estimated cost の `cost:`・`cost-total:` の行の例と料金の表 | `cli.md` | PR-4 |
| `TestOutputLayoutDocsContract`（`output_layout_docs_test.go:92`） | `--out` のファイルの例と Slack の投稿の例 | `--out` の例は `cli.md`（PR-4）、Slack の投稿の例は両版（PR-6） | PR-4・PR-6 |
| `TestExtensionDocumentation`（`extension_docs_test.go:52`） | 拡張の ID | 両版（CLAUDE.md と `security.md` は今のまま） | PR-6 |
| `TestServerDocsContract` の `README.md`（`cmd/yt2column-server/docs_test.go:42`） | `SLACK_WEBHOOK_URL`、ソケットのパス、終了コードの表、curl の例（`POST /articles` と Job API の 3 つ） | 前の 3 つは両版（PR-6）、curl の例は `development.md`（PR-4） | PR-4・PR-6 |
| 同 `rejection_reasons`（`:96`） | 3 列の表の `reason` とステータスコード | 3 列の表は `development.md`（PR-4）。2 列の表の `reason` の集合と表示の区分は README（PR-4 は英語の 1 版、PR-6 は両版） | PR-4・PR-6 |
| 同 `api_codes`（`:124`） | Job API codes の 6 つの表 | `development.md` | PR-4 |
| `TestReadmeHostExitCodes`（`cmd/yt2column-nmhost/main_test.go:436`） | ホストの終了コードの表 | 両版 | PR-6 |
| `serverReasons.test.ts:44` | 3 列の表の「fixed message」の行 | `development.md` の 3 列の表 | PR-4 |
| `metadataDocs.test.ts:24-35` | Browser extension の節の英語の促しのメッセージ（0046 の PR-4 の後は主ボタンの名前も） | `README.md` は `ja.ts`、`README.en.md` は `en.ts` の文言 | PR-6 |

ステップ 5-1 で設計書 3.7 を作り直したときは、この表も作り直す。

### 4.3. 差分の検査

設計書 7.3 の AC と AC-22・AC-35 の変えない部分は、`_checks/` のプログラムで確かめる。`_checks/move/` は PR-1（ステップ 1-6）、`_checks/split/` は PR-4（ステップ 5-3）、`_checks/readme/` は PR-6（ステップ 6-10）で、それぞれ 1 度実行し、結果を 7 章に記録する。`make test` と CI では実行せず、マージの後は保守しない。

### 4.4. 統合テスト

N/A（設計書 7.2）。

### 4.5. 既存のテスト

コマンドの文書を読むテスト（`cmd/yt2column/fixpr_docs_test.go`・`codex_review_docs_test.go` の各テスト）と `cmd/yt2column/copied_transcript_docs_test.go` は、検査する見出しと行を変えずに通す（設計書 3.7、H-08）。変わるのは、`doctestutil` に移した補助関数の呼び出しだけである（PR-3）。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| PR-1 のマージの後に、開いているブランチが旧パスを持ち込む | エージェントが規約を読めないまま進む | 指示と規約の文書は `TestInstructionDocPaths` が CI で失敗させる。過去のタスク文書は検査しないので、PR-1 の本文で知らせる（設計書 8 章） |
| タスク 0049 が同じコマンドの文書を変える | PR-2 と 0049 の PR が衝突する | 先にマージされた方に、後の PR が `main` を取り込んで合わせる。PR-2 の同期の手順は、0049 が変える手順（ローカルのレビュー）の後に置くので、取り込みで位置を確かめる |
| 0046 の PR-4 が想定の外の README を変える | 設計書 3.6・3.7 が古くなる | ステップ 5-1 で確かめ、外なら設計書を作り直してから進む |
| PR-4 と PR-6 の間、CLAUDE.md の Development Notes が古い README の表を述べる | 読み手が古い説明を読む | 1 つの PR の間だけである。PR-6 で直す（ステップ 6-9） |
| PR-6 が大きく、レビューに時間がかかる | マージが遅れ、README に触れるほかの PR と衝突する | 構造の抽出を PR-5 に分けた（1.3）。PR-6 の間は README に触れる別の変更を入れない |
| 翻訳で内容が落ちる・増える | AC-10・AC-27 を満たさない | 1.3 の翻訳のレビュー、`_checks/split`、構造の一致のテストで確かめる |
| 日本語の見出しのアンカーが GitHub と違う | リンクの検査が通っても、GitHub でリンクが切れる | 設計書 3.4 で確かめた文字だけを見出しに使う（ステップ 6-2） |

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6）
-   [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4）
-   [ ] PR-3 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3）
-   [ ] PR-4 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4）
-   [ ] PR-5 マージ済み（対象ステップ: 6-1）
-   [ ] PR-6 マージ済み（対象ステップ: 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 6-9 / 6-10）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` はテスト、`static` は `make` のターゲット、`diff` は `_checks/` のプログラム、`manual` は PR のレビューでの読み合わせを指す。

| AC | 確かめ方 | ステップ | 内容 |
|---|---|---|---|
| AC-01〜AC-04a | manual | 2-1 | CLAUDE.md の原則・ディレクトリの言語・例外の記述（PR-2 のレビュー） |
| AC-05 | manual | 2-1 | Go Idioms の例外の記述（PR-2） |
| AC-06 | manual | 1-4・2-1・5-4・6-2・6-4 | 表の各文書の言語（PR-6 のレビューで全体を読む） |
| AC-07 | manual | 6-9 | CLAUDE.md のサーバの API と起動の記述（PR-6） |
| AC-08 | diff: `_checks/move/` | 1-6 | 5 ファイルがリンクのパスの違いを除いて同じ |
| AC-09 | diff: `_checks/move/` ＋ manual | 1-1・1-6 | 2 つの手引きの差分の行が許された 3 種類だけで、`.ja.md` がない（diff）。4 ファイルが日本語であること（manual） |
| AC-10 | manual | 1-4 | 翻訳の内容（PR-1。1.3 の翻訳のレビューを経る） |
| AC-11 | manual | 1-1 | 7 章の読み比べの記録（PR-1） |
| AC-12・AC-13 | diff: `_checks/move/` | 1-6 | `docs/dev/developer_guide/` がなく、旧パスが残っていない |
| AC-14 | diff: `_checks/move/` | 1-6 | 過去のタスク文書の変更がパスだけ |
| AC-15 | diff: `_checks/move/` ＋ test: `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths` | 1-5・1-6 | 置き換えた相対リンクの先がある（diff）。指示と規約の文書のパスがある（test） |
| AC-16 | test: `cmd/fixprguard/docs_test.go::TestGuidePathExists` | 1-3・1-5 | `guidePath` の値のファイルがある |
| AC-17 | test: `cmd/fixprguard/docs_test.go` の手引きを読む各テスト・`internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages` ＋ static: `make test` | 1-3 | 移動後の 1 つのファイルを検査して通る |
| AC-18 | manual | PR-1 の本文 | 入れ直しの記述 |
| AC-19〜AC-21 | manual ＋ diff: `_checks/split/` | 5-2・5-3・6-2 | 振り分けが設計書 3.6 のとおり（manual）。文の行き先の一覧は diff の出力で補う |
| AC-22 | diff: `_checks/split/` ＋ manual | 5-3 | 今の README の文がどれかの文書にある（diff）。重複が許された種類だけであること（manual） |
| AC-23 | test: `cmd/yt2column/readme_structure_test.go::TestReadmeVersionLinks` | 6-2・6-7 | 両版に `cli.md`・`development.md` へのリンクがある |
| AC-24 | manual | 5-4 | 2 つの文書が日本語で、英語版のファイルがない（PR-4） |
| AC-25 | test: `cmd/yt2column/docs_test.go::TestReadmeLinks` | 5-2・6-6 | 4 つの文書の相対リンクとアンカー |
| AC-26 | manual | 5-4・6-9 | 参照先の付け替え（PR-4・PR-6） |
| AC-27・AC-27a | manual | 6-2・6-4 | 翻訳の内容と書き換え（PR-6。1.3 の翻訳のレビューを経る） |
| AC-28 | test: `cmd/yt2column/readme_structure_test.go::TestReadmeVersionLinks` | 6-2・6-4・6-7 | 両版の先頭に、もう一方の版へのリンクがある |
| AC-29 | test: `cmd/yt2column/readme_structure_test.go::TestReadmeVersionsShareStructure`・`internal/doctestutil` の `TestParseStructure` | 6-1・6-7 | 構造の一致と失敗のメッセージ、拒否する形 |
| AC-30 | static: `make test` ＋ test: 同上 | 6-7 | テストが `cmd/yt2column` にあり、`make test` と `doc-guard` で走る |
| AC-31 | test: 4.2 の表で「両版」の各テスト | 6-6 | 版ごとのサブテストで、欠けた版の名前で失敗する |
| AC-31a | test: `extension/test/metadataDocs.test.ts::video metadata documentation` | 6-2・6-4・6-6 | 版ごとのカタログとの一致 |
| AC-32 | manual | 4.2・5-2・5-4・6-6 | 4.2 の表と実装の一致（PR-4・PR-6） |
| AC-33 | test: `extension/test/ciChanges.test.ts::has-extension-changes` | 5-2・6-8 | `README.en.md`・`development.md` で走り、`cli.md` で走らない |
| AC-34 | manual | 6-3・6-4 | 用語集の語と、`README.en.md` での使い方（PR-6） |
| AC-35 | test: `extension/test/metadataDocs.test.ts`（英語のカタログの参照先） ＋ diff: `_checks/readme/` | 6-8・6-10 | 英語のメッセージが `README.en.md` を示す（test）。`ja.ts` が変わらない（diff） |
| AC-35a | test: `cmd/yt2column-server/run_test.go::TestStartupFailures` | 6-8 | 診断が `README.en.md` を示し、ファイルがある |
| AC-36 | test: `cmd/yt2column/run_test.go::TestRunHelp` ＋ test: `TestReadmeLinks`（`cli.md` の README の設定の節へのリンク） ＋ manual | 6-8・5-4 | ヘルプが `cli.md` を示し、ファイルがある。`cli.md` に設定の節へのリンクがあることは PR-4・PR-6 で読む |
| AC-37 | manual | 3-2・3-3・3-4 | 3 つのコマンドの同期の手順（PR-2） |
| AC-38・AC-39 | manual | 3-1 | `/mktrans` の振る舞い（PR-2）。ステップ 6-4 で全文の翻訳を実際に使う |
| AC-40 | manual | 2-1・2-2 | CLAUDE.md と `_context.md` の記述（PR-2） |
| AC-41 | manual | 6-9 | Development Notes の記述（PR-6） |

**読み比べの記録（AC-11）:** （ステップ 1-1 で記録する）

**差分の検査の実行の記録:** （ステップ 1-6・5-3・6-10 で記録する）

## 8. 横断の確認 (Cross-Search Checklist)

`make lint`・`make test` では見つからない残りを確かめる。

-   [ ] 1.5 の旧い補助関数の名前（小文字の `readDoc` など）が、`cmd/yt2column`・`cmd/yt2column-server` のコメントに残っていない（PR-3）。
-   [ ] `docs/translation_glossary.md` の訳語と、`cli.md`・`development.md`・`README.md`・`README.en.md` の用語が一致する（PR-4・PR-6 のレビューで読む）。
-   [ ] `has-extension-changes.sh` の先頭のコメントと、CLAUDE.md の Development Notes が、拡張のテストが読む文書の同じ一覧を述べる（PR-6 のレビューで読む）。

## 9. 成功基準 (Success Criteria)

-   **機能の完全性:** 7 章のすべての AC の確かめ方が実行され、`test`・`static`・`diff` が合格し、`manual` の行が PR のレビューで確かめられている。
-   **品質:** 各 PR で `make test`・`make lint` が通り、拡張を変える PR で `make ext-check`、Go のテスト専用のパッケージを変える PR で `make deadcode` が通る。新しいテストと、検査の対象を変えたテストは、守る対象を壊して失敗することが確かめられ、コミットのメッセージに書かれている。消したテストと表の行について、網羅の比較の結果がコミットのメッセージに書かれている。
-   **セキュリティ:** `fixprguard` の範囲の判定とメンションの検査は変わらない（設計書 5 章）。
-   **文書:** 新しい文書を追加するとき、CLAUDE.md の原則とディレクトリの規則から言語が一つに決まる（要件書 4.5）。

## 10. 次のステップ

-   本書の承認の後、`/runplan 0048` で PR-1 を実装して開く。PR-2・PR-3 も、前の PR のマージの後に `/runplan 0048` で進める。
-   PR-4 は、0046 の PR-4 のマージの後に始める（ステップ 5-1）。
-   PR-1 のマージの後、`make install-fixprguard` で `fixprguard` を入れ直す。
-   PR-6 のマージの後、issue #537 を閉じる（`/runpr` が最後の PR の後に案内する）。
