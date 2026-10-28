# 実装計画書：文書の言語を読み手で決める方針への整理と、README の日英二言語化

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-28 |
| Review date | 2026-10-28 |
| Reviewer | isseis |
| Comments | 2026-10-29: ステップ 3-4 の記述を、`runpr.md` に `main` を取り込む手順がないことに合わせて直した。衝突の規則は、`runpr.md` の手順 1 に加えた段落から `mktrans.md` の規則を参照する形にした。ステップ 3-1 で、`mktrans.md` の説明（frontmatter の `description`）と CLAUDE.md のコマンドの一覧の `/mktrans` の行も、README の組だけを扱う形に改めた。止まったときに `/mktrans` が書きかけた `README.en.md` を戻す手段を、呼び出し元の `git restore` から、`/mktrans` が書く前に取った写しで自分で戻す形にした（設計書 4.3 も合わせた。呼び出し元の作業ツリーの `README.en.md` の変更を消さないため）（決定の変更なし。編集上の修正） |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番、「設計書 3.6」などは設計書の節を指す。既存のコードと文書についての記述は、コミット `c76eec26`（本書の作成時の `main`）で確かめた。`file:line` は、特に断らない限り、このコミットの行番号を指す。

設計書が引く行番号はコミット `364c95b8` のものである。2 つのコミットの差は、拡張の 7 ファイル（`extension/src/ui/`・`extension/static/style.css`・`extension/test/` の描画のテスト）と、タスク 0046・0048・0049 の文書だけである（`git diff --stat 364c95b8 c76eec26`）。設計書が引く Go のコード、README、CLAUDE.md、コマンドの文書、`extension/src/core/i18n/` の行番号は、そのまま使える。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、次を行う。

-   `docs/dev/developer_guide/` を `docs/dev/conventions/`（英語）と `docs/dev/guides/`（日本語）に分け、旧パスをすべて置き換える（F-002、設計書 2.2・6.2）。
-   文書の言語の原則と例外を CLAUDE.md に書く（F-001、設計書 1.2）。
-   `/mktrans` を `README.md` → `README.en.md` の組に改め、`/runplan`・`/fixpr`・`/runpr` に同期の手順を加える（F-006、設計書 2.4・3.8・4.3）。
-   ガードテストの Markdown の補助関数を共有パッケージ `internal/doctestutil` に移す（設計書 3.3）。
-   README から開発者向けの内容を `docs/dev/guides/cli.md` と `docs/dev/guides/development.md` に移す（F-003、設計書 3.6）。
-   `README.md` を日本語にし、`README.en.md` を作り、ガードテストを両版に広げ、構造の一致のテストを加える（F-004・F-005、設計書 3.3〜3.5・3.7・4.1）。

yt2column の実行時の振る舞いは変えない（要件書 4.1・4.3）。Go の本番のコードで変えるのは、利用者に参照先を示す 4 つの文字列（`cmd/fixprguard/main.go:132`、`cmd/yt2column/run.go:225`・`:259`、`cmd/yt2column-server/run.go:145`）と、コメントのパスだけである。

### 1.2. 実装の方針

-   **設計は設計書に従い、本書は繰り返さない。** 置き場所は設計書 2.2、ファイルごとの変更は 3.1、README の振り分けは 3.6、ガードテストの振り分けは 3.7、`/mktrans` の振る舞いは 3.8、同期の手順は 4.3、旧パスの対応表は 6.2 が定める。本書のステップは、どの節に従って何を変え、どう確かめるかだけを示す。
-   **移動と内容の変更を別のコミットにする。** `git mv` だけのコミット、パスの置換のコミット、翻訳のコミットを分け、レビューで差分を読み分けられるようにする。README の振り分けも、英語のまま移すコミットと翻訳のコミットを分ける（1.3）。途中のコミットでテストが失敗してよいのは、本書がそう書くコミット（ステップ 1-2、6-2〜6-4）だけである。PR は squash でマージされ、グリーンゲートは PR の先頭で確かめる。
-   **テストは壊して確かめる。** 新しいテストと、検査の対象を変えたテストは、守る対象を壊して失敗することを確かめてからコミットし、そのことをコミットのメッセージに書く（CLAUDE.md、Testing Strategy）。
-   **消すテストは、関数ごとの網羅を比べる。** テストや表の行を消すときは、`go tool cover -func` の関数ごとの結果が前後で変わらないことを確かめ、コミットのメッセージに書く。関数を移すときは、1.5 の対応表で古い名前と新しい名前を対応させて比べる。
-   **新しい Go のテストのファイルと `_checks/` のプログラム。** 新しい `_test.go` には、ほかのテストと同じく `//go:build test` を付ける。`_checks/` のプログラムは環境変数を読まない（リポジトリ全体の環境変数の読み取りの検査は、`_` で始まるディレクトリにも及ぶ。`docs/tasks/0040_production_profile/_checks/main.go` の先頭のコメント）。コメントと文字列は英語にする。

### 1.3. 計画の際に決めたこと（2026-10-28）

-   **フェーズ 2 とフェーズ 3 を 1 つの PR にする（レビュアーが選んだ）。** どちらも指示の文書だけを変え、言語の方針と翻訳の流れという 1 つの関心にまとまるためである。PR は全部で 6 つになる。
-   **フェーズ 6 の構造の抽出（`ParseStructure`）を、先に別の PR にする（レビュアーが選んだ）。** README の翻訳のレビューと、コードのレビューを分けるためである。`ParseStructure` は、README の版やロケータ（ガードテストが節や表を探す見出しと列名。設計書 1.1・3.3）に依存せず、単独で表のテストを持てる。
-   **フェーズ 5 では、英語のまま移してから翻訳する（レビュアーが選んだ）。** 英語のまま移したコミットで、今の README のどの文も README・`cli.md`・`development.md` のどれかに残っていること（AC-22）を `_checks/` のプログラムで機械的に確かめ、その後のコミットで `cli.md` と `development.md` を日本語にする。AC-22 の読み合わせを、機械的な確認で補うためである。
-   **英語から日本語への翻訳の手段（設計への申し送りの H-13。設計書 10 章）。** `pricing_update.md`・`claude_output_limit.md`・`cli.md`・`development.md`・`README.md` の日本語は、実装するセッションが直接書く。書いた後に `/japrose` で整え、翻訳の正確さは `.claude/commands/_lib/review-subagent-pattern.md` のレビューで、`mktrans.md` の Accuracy checklist を基準として確かめる。翻訳の元は、そのコミットの前の英語の文書である。改訂前の `/mktrans` は使わない（`README.ja.md` を出力するため。設計書 8 章）。翻訳の前後で、見出しのレベルの並び、コードブロック、インラインのコードが変わらないことは、`_checks/` のプログラムで確かめる（4.3）。
-   **差分の検査のスクリプトは Go で書き、PR ごとにディレクトリを分ける。** `_checks/move/`（PR-1）、`_checks/split/`（PR-4）、`_checks/readme/`（PR-6）である。Go は開発の環境と CI で必ず使え、0040 の `_checks/main.go` に前例がある。1 つのディレクトリには 1 つの `main` パッケージしか置けないので、分ける。Markdown の走査は、PR-3 の後は `doctestutil` を `go run -tags test` で使い、作り直さない。
-   **フェーズ 5 に、設計書 8 章のフェーズ 6 から 2 つを前に出す（設計書 8 章を編集上の修正で合わせた）。** (1) `has-extension-changes.sh` と `ciChanges.test.ts` の `docs/dev/guides/development.md` の行。フェーズ 5 から `serverReasons.test.ts` が `development.md` を読む。同じ PR で CI の判定にも加えないと、`development.md` だけを変える PR では拡張のジョブが走らなくなる。(2) Browser extension の節の最上位（`##`）化。`## Development`（`README.md:1115`）を移すと、`### Browser extension`（`:1179`）が前の `##` の節の下に入ってしまうためである。今の節は `###` の小節を持たない（`README.md:1179-1305` の見出しは節の見出しだけ）ので、`markdownSection` を改めずに `## Browser extension` を読める。`markdownSection` の改修（設計書 3.5）は、日本語の README で節の中に小節を置くとき（ステップ 6-2）だけ行う。
-   **AC-26 の参照先の付け替えは、節が移る PR で行う。** `pricing_update.md`・`claude_output_limit.md` が参照する README の節のうち、`cli.md` に移るもの（Token usage、Estimated cost の CLI の行、Output format の `--out` の小節）は PR-4 で `cli.md` の見出しに、README に残るもの（Configuration など）は PR-6 で日本語の見出しにする。`project_overview.md:157`・`production_profile.md:5`・`translation_glossary.md:53` は README に残る節を指すので、PR-6 で日本語の見出しにする。PR-1 の翻訳では、README の今の英語の節の名前をそのまま参照する。
-   **まだないファイルを、指示と規約の文書に書かない。** PR-1 で加えるパスの検査（ステップ 1-5）は、相対リンクと、バッククォートの中の `docs/dev/…md` を確かめる。そのため PR-2 で CLAUDE.md とコマンドの文書に `docs/dev/guides/cli.md`・`development.md` のパスを書くと失敗する。PR-2 では、`README.en.md` だけを名前（バッククォート、リンクにしない）で書く。`cli.md`・`development.md`・`README.en.md` へのリンクと、AC-07・AC-41 の記述は PR-6 で書く（設計書 8 章のフェーズ 2 の注）。
-   **日本語のリテラルの例外の範囲（AC-05）。** CLAUDE.md の Go Idioms の例外を、Go のガードテストと、そのロケータを持つテスト専用のパッケージ（`internal/doctestutil`）に及ぶ形で書く。リテラルの種類は、見出し、表のセル、項目の札（例：`TestServerDocsContract` が探す `macOS:` などの行頭の語）、太字の印（例：`cmd/fixprguard/docs_test.go` の `**実行関連ファイル。**`）である。ロケータは `_test.go` でないファイルに置くので、「ガードテストのリテラル」だけでは範囲が足りないためである。
-   **同期の印の ID の長さ。** このリポジトリのオブジェクトの形式は SHA-1 である（`git rev-parse --show-object-format` の出力が `sha1`、`c76eec26`）。印を検査するテストは 40 文字の 16 進の小文字だけを受け入れる。形式が変わったら、テストを改める。
-   **`doctestutil` の補助関数の最終の一覧**は、1.4 の呼び出し元の数で決めた。

### 1.4. 既存コード調査結果

**旧パスの参照（設計書 6.2）。** `c76eec26` で `git grep -c developer_guide`（本タスクのディレクトリを除く）は 117 ファイル・343 行である。内訳は、タスク文書（本タスクを除く）の 93 ファイルと、それ以外の 24 ファイルである。タスク文書は、設計書の 92 ファイルに `docs/tasks/0049_codex_local_review/observations.md` が加わった。それ以外は、`.claude/commands/` の 11 ファイル（`_context.md`・`fixpr.md`・`mergepr.md`・`mkarch.md`・`mkplan.md`・`mkplan2.md`・`mkreq.md`・`mktrans.md`・`runplan.md`・`runpr.md`・`weakreview.md`）、`AGENTS.md`、`CLAUDE.md`、`Makefile`、`README.md`、`cmd/fixprguard/docs_test.go`、`cmd/fixprguard/main.go`、`docs/dev/project_overview.md`、`internal/config/config.go`、`internal/llm/claude/request.go`、`internal/llm/provider/provider.go`、`internal/llm/provider/provider_test.go`、`internal/pipeline/pipeline_test.go`、`internal/pricing/rates.go` である。移動する 11 ファイルの本文には、`developer_guide` を含む参照はない。11 ファイルどうしの参照は、同じディレクトリに移る `requirements_process.md` → `mermaid_reference.md` の相対リンクと、`pricing_update.md:70` の `package_reference.md` の名前だけで、どちらも書き換えは要らない（`pricing_update.md:70` は、移動の後に `package_reference.md` と別のディレクトリになるので、名前だけの参照のまま読めるかを、ステップ 1-4 の翻訳で確かめる）。

**指示と規約の文書の今のパス。** CLAUDE.md・AGENTS.md・`.claude/commands/`・`docs/dev/` の 35 の `.md` ファイルについて、バッククォートの中の `docs/dev/…md` と相対リンクの先を `c76eec26` で走査した。存在しない先は、コードブロックの中の `CLAUDE.md:501`（Go のコード例の `](err)`）だけだった（走査は本書の作成のための一時的なスクリプトで行った。検査そのものはステップ 1-5 のテストである）。

**補助関数の重複と呼び出し元。** `cmd/yt2column-server/docs_test.go` の `readDoc`・`markdownTables`・`splitTableRow`・`isTableSeparator`・`findTable`・`tableRow`・`stripBackticks`・`headingLevel`・`sectionByHeading` は、`cmd/yt2column/docs_test.go` の同名の関数と本体が一字一句同じである（`awk` で関数を切り出して比べた）。`:363-368` は重複を説明するコメントで、`:450-461` の `tablesByFirstHeader` はサーバのパッケージだけにある。呼び出しの数（定義を除く、`*_test.go` の中）は次のとおりである。

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

**lint の除外。** `.golangci.yml:181-188` は、`gosec`・`goconst`・`err113`・`dupl`・`gocyclo`・`errcheck` を `_test.go` だけで外す。`internal/doctestutil` の `_test.go` でないファイルにはこれらが及ぶ。`ReadDoc` の `os.ReadFile` は変数のパスを読む。前例は `cmd/yt2column/test_helpers.go:378` の行の単位の `//nolint:gosec` である。

**補助関数のテスト。** `cmd/yt2column/docs_test.go` の `TestSectionByHeading`（`:347`）・`TestScanDocLines`（`:594`）・`TestFencedBlocks`（`:615`）・`TestAnchorFromHeading`（`:1154`）・`TestSectionOutsideBlocks`（`:1190`）を `doctestutil` に移す。`TestNumberedItems`（`:1175`）と `output_layout_docs_test.go:137` の `TestFirstCodeBlockAfter` は、残る関数のテストなので残す。`cmd/yt2column-server/docs_test.go:213` の `TestSectionByHeading` は、コピーとの一致を固定するテストなので消す（設計書 3.3）。`TestReadmeLinks` は、コードブロックの外のリンクの開き（`](`）の数と抜き出したリンクの数が等しいことも確かめる（`docs_test.go:1123-1131`）。

**テスト専用のパッケージ。** `internal/pipeline/pipeline_test.go:583` の `testOnlyPackageDirs` に 4 つのパッケージがある。`TestFakesCarryBuildTag` は、そこに挙げたディレクトリの `_test.go` でないファイルに `//go:build test` を求める。`TestPackageReferenceListsPackages`（`:671`）は、`_test.go` でない Go のファイルを持つパッケージの行を `package_reference.md` に求め、文書を `:707` のパスで読む。

**UI が示すパス。** `cmd/fixprguard/main.go:132` の `guidePath` は `:268` の診断で使う。CLI のヘルプは `cmd/yt2column/run.go:225`（`--slack` のフラグの説明、「see README」）と `:259`（使い方の文、「see README.md」）で、`run_test.go:1261` の `TestRunHelp` がヘルプの出力を検査している。サーバの診断は `cmd/yt2column-server/run.go:145` で、`run_test.go:39` の `TestStartupFailures` が `takes no arguments` を検査している（`:60`）。拡張の英語のメッセージは `extension/src/core/i18n/en.ts:83`、日本語は `ja.ts:81` である。

**拡張のテストの補助。** `extension/test/helpers/repoFile.ts:15` の `markdownSection` は、次のレベル 1〜3 の見出しで節を終える。`extension/test/markdownSection.test.ts` がその表のテストを持つ。`metadataDocs.test.ts:25` は `"### Browser extension"` で節を探し、英語の促しのメッセージだけを比べる。`serverReasons.test.ts:15-17` は README の `reason | status code | extension shows` の表を探す。`ciChanges.test.ts:21-30` の走らせる表には `README.md` があり、走らせない表（`:69-77`）には、文書のパターンごとに `docs/README.md`・`README.md.bak` のような、似ているが違うパスの行がある。

**コマンドの文書を読むテスト。** `cmd/yt2column/fixpr_docs_test.go:30-40` は `fixpr.md` の Phase の見出しと `runpr.md` の手順の見出しを定数に持つ。`:189` は `_lib/task-pr-handoff.md` の Part B の「5. Make sure Codex starts reviewing the PR.」と「\n6. 」で節を切り、`codex_review_docs_test.go:74` は Part B の手順 3 を読む。`fixpr_docs_test.go:971`（`TestRemovedCommandLiterals`）は `fixpr.md` などで `git add -A`・`git commit -m` などを禁じ、`:984` は Phase 4.5 の外の `git push` を禁じる。`git restore` は禁じていない。`fixpr.md` の Phase 4.5（`:581-597`）は、`_lib/coderabbit-local-review.md` の手順で修正をコミットし、その後に `git push` を 1 度だけ実行する。

**`fixprguard` の範囲の判定（設計書 2.5）。** 設計書のとおり、変更は要らない。

**同時に進むタスク。** タスク 0046 は PR-3 と PR-4（README のポップアップの説明を変える）が未マージである（`docs/tasks/0046_popup_redesign/03_implementation_plan.md:329`・`:390`）。タスク 0049（Codex CLI によるローカルのレビュー）は、要件書が `_lib/task-pr-handoff.md`・`runplan.md`・`fixpr.md`・`runpr.md` を変える対象に挙げている（`docs/tasks/0049_codex_local_review/01_requirements.md`）。

**型の構築の経路。** 本タスクは本番のコードで具体的な型によって振る舞いを分けない。`doctestutil.Lang` の零値と範囲の外の値は `Locator.In` が失敗させる（設計書 3.3）。テスト専用のパッケージなので、本番のコードからの構築の経路はない。

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

-   [x] `mergepr_guide` と `fixprguard_guide` の英語版と日本語版を、節ごとに読み比べる（設計書 7.3 の事実を出発点にする）。
-   [x] 英語版にだけある記述を、日本語版に加える。なければ加えない。
-   [x] 結果（節ごとの対応、加えた記述とその場所、`mergepr_guide` の 1 行の差の理由）を 7 章の「読み比べの記録」に書く。

**完了の条件:** 記録が 7 章にある。加えた記述が、ステップ 1-6 の実行結果の AC-09 の差分の行の一覧に出ることを確かめる。

#### ステップ 1-2: ファイルの移動

**変更するファイル:** `docs/dev/developer_guide/` の 11 ファイル

-   [x] 設計書 2.2・6.2 のとおり、`git mv` で移す。`mergepr_guide.ja.md`・`fixprguard_guide.ja.md` は `docs/dev/guides/` の `.ja` を外した名前にする。英語版の `mergepr_guide.md`・`fixprguard_guide.md` は `git rm` で消す。`pricing_update.md`・`claude_output_limit.md` は英語のまま `docs/dev/guides/` に移す（翻訳はステップ 1-4）。
-   [x] 内容を変えずに、このステップだけでコミットする（このコミットではテストが失敗してよい。次のステップで直す）。

**完了の条件:** `docs/dev/developer_guide/` がなく、コミットの差分が改名と削除だけである。

#### ステップ 1-3: 旧パスの置換

**変更するファイル:** 1.4 の「旧パスの参照」のすべてのファイル、`docs/tasks/0032_japanese_pr_title/check_rules.sh`

-   [x] 設計書 6.2 の対応表で、ファイル名を含む旧パスを新しいパスに置き換える。相対リンクはディレクトリの部分だけを置き換える。ディレクトリだけを指す記述（設計書 6.2 の 6 か所と、`CLAUDE.md:25`・`mktrans.md:21`）は置き換えない。`CLAUDE.md:25`・`mktrans.md:21` は PR-2 で書き直す。
-   [x] `cmd/fixprguard/main.go:132` の `guidePath` を `"docs/dev/guides/fixprguard_guide.md"` にする（AC-16）。
-   [x] `cmd/fixprguard/docs_test.go:27-30` の `guideFiles` を 1 つのファイルにし、英語の手引きの印を使う行（`"**Execution-related files.**"` など）を消す（AC-17、設計書 3.7）。消した行について、`go tool cover -func` の関数ごとの結果が変わらないことを確かめる。
-   [x] `internal/pipeline/pipeline_test.go:707` のパスを `docs/dev/conventions/package_reference.md` にする（AC-17）。

**完了の条件:** `make test` が通る。`git grep -n developer_guide` の残りが、本タスクの文書と、ディレクトリだけを指す記述だけである（AC-13 の検査そのものはステップ 1-6）。

#### ステップ 1-4: `pricing_update.md`・`claude_output_limit.md` の翻訳（AC-10）

**変更するファイル:** `docs/dev/guides/pricing_update.md`・`claude_output_limit.md`、`docs/translation_glossary.md`

-   [x] 1.3 の手段で、全文を日本語に翻訳する。節の構成、手順、値、コマンドの例は変えない。README の節の参照は、今の英語の節の名前のまま訳す（付け替えは PR-4・PR-6。1.3）。
-   [x] 新しい用語があれば、用語集に加える。
-   [x] `/japrose` で整え、翻訳のレビューで指摘を直す。翻訳のコミットは、移動と置換のコミットと分ける。

**完了の条件:** 翻訳のレビューに Critical と Major の指摘が残っていない。ステップ 1-6 の AC-10 の検査が通る。

#### ステップ 1-5: パスの検査のテスト（AC-15・AC-16）

**変更するファイル:** `cmd/yt2column/docpaths_test.go`（新規）、`cmd/fixprguard/docs_test.go`

-   [x] 設計書 4.2 の「指示と規約の文書が示すパス」のテスト `TestInstructionDocPaths` を加える。対象は CLAUDE.md・AGENTS.md・`.claude/commands/` 配下・`docs/dev/` 配下の `.md` で、相対リンクの先とバッククォートの中の `docs/dev/…md`（`:NN` のような行番号の付いた形を含む）が存在するファイルであることを確かめる。コードブロックの中と、`<…>` の形の仮の値のリンクは対象にしない。
-   [x] 1 つの文書から確かめる参照を抜き出す部分を、文書の文字列だけを受け取る関数にし、表のテストを持たせる。行は、相対リンク、`#` 付きのリンク、`<…>` のリンク、コードブロックの中のリンク、バッククォートのパス（行番号付きを含む）、外部の URL、`mailto:`、文書の中の `#` のリンクである。
-   [x] 文書ごとに、`TestReadmeLinks` と同じく、コードブロックの外のリンクの開きの数と抜き出したリンクの数が等しいことを確かめる。
-   [x] `guidePath` のファイルがリポジトリに存在することを確かめるテスト `TestGuidePathExists` を `cmd/fixprguard/docs_test.go` に加える（AC-16）。
-   [x] `TestInstructionDocPaths` が、入れ子のディレクトリの文書（`.claude/commands/_lib/task-pr-handoff.md` と `docs/dev/conventions/` の 1 つ）の、存在しない相対リンクと存在しないバッククォートのパスで、それぞれ失敗することを、一時的に文書を変えて確かめる。`TestGuidePathExists` が、`guidePath` を旧パスに戻すと失敗することを確かめる。

**完了の条件:** `go test -tags test ./cmd/yt2column ./cmd/fixprguard` が通る。

#### ステップ 1-6: 移動の差分の検査（AC-08〜AC-10・AC-12〜AC-15）

**変更するファイル:** `docs/tasks/0048_doc_language_policy/_checks/move/main.go`（新規）、本書

-   [x] merge-base と PR の先頭のコミットを比べ、設計書 7.3 の表の AC-08・AC-09（の一部）・AC-12・AC-13・AC-14・AC-15 を確かめるプログラムを書く。AC-09 では、`docs/dev/guides/` に `.ja.md` で終わるファイルがないことも確かめ、`.ja.md` の版との差分の行を一覧で出力する。AC-13 は、ファイル名を含む旧パス（設計書 6.2 の対応表の 11 ファイル）だけを探し、本タスクのディレクトリを除く。
-   [x] AC-10 の構造と値の部分を確かめる。`pricing_update.md`・`claude_output_limit.md` の翻訳の前後で、見出しのレベルの並びが同じで、コードブロックの内容がリンクのパスの違いを除いて同じで、インラインのコードの多重集合が同じであること。
-   [x] 各検査が違反を見つけることを、一時的に 1 か所ずつ変えて確かめ、元に戻す（AC-08 の規約の 1 行を変える、`docs/dev/guides/` に `.ja.md` を置く、`docs/dev/developer_guide/` に空のファイルを置く、旧パスを 1 つ残す、過去のタスク文書の Status の行を変える、相対リンクの深さを変える、翻訳のコードブロックの 1 文字を変える）。結果をコミットのメッセージに書く。
-   [x] リポジトリのルートで `go run ./docs/tasks/0048_doc_language_policy/_checks/move/` を実行し、終了コード、出力、2 つのコミットの ID を 7 章の「差分の検査の実行の記録」に書く。

**完了の条件:** PR-1 のブランチで、プログラムが終了コード 0 で終わる。`make test` と CI では実行しない（`requirements_process.md` §4）。

#### ステップ 1-7: PR-1 の本文

**変更するファイル:** なし（PR の本文）

-   [x] PR-1 の本文に、マージの後に `make install-fixprguard` で入れ直す必要があること（AC-18。入れ直すまでは、インストール済みの `fixprguard` が旧パスを示す）を書く。
-   [x] 開いている PR（0046 の PR-3・PR-4、0049 の PR など）は、`main` を取り込んで旧パスを直す必要があることを書く（設計書 8 章）。
-   [x] ステップ 1-6 の AC-09 の一覧に出た差分の行を、PR のレビューで読む行として挙げる。

**完了の条件:** PR-1 の本文に 3 つの記述がある。

### PR-1 作成ポイント: developer_guide split and path replacement

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7

**推奨タイトル**: `docs(0048): developer_guide を規約と手引きに分け、旧パスを置き換える`

**レビュー観点**: 英語版の手引きの読み比べの記録と、日本語版に加えた記述（AC-11） / `_checks/move` の出力（特に AC-09 の差分の行と AC-14 の結果） / `pricing_update.md`・`claude_output_limit.md` の翻訳の正確さ（AC-10） / `TestInstructionDocPaths` の対象と除外

**実装モデル要件**: standard

**判定理由**: 機械的な移動と置換が中心で、設計書が対応表と検査の内容を決めており、`mkplan.md` の Conditional checks の 2 つ以上には当たらない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/561
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 言語の原則

AGENTS.md の変更（リンクのパス）は、ステップ 1-3 の置換に含まれる（設計書 8 章）。このフェーズでは AGENTS.md を変えない。

#### ステップ 2-1: CLAUDE.md の改訂（AC-01〜AC-05・AC-40）

**変更するファイル:** `CLAUDE.md`

-   [x] "Documents" 節に、3 つの原則（AC-01）、ディレクトリごとの言語（AC-02）、例外（AC-04・AC-04a。範囲と理由を含む）を書き、`CLAUDE.md:25` の例外の列挙を消す（設計書 1.2）。`README.en.md` は名前だけを書き、リンクにしない。`docs/dev/guides/cli.md`・`development.md` のパスは書かない（1.3）。
-   [x] "Language" 節を、原則と矛盾しない形にする（AC-03）。
-   [x] Go Idioms の例外（`CLAUDE.md:507-516`）を、1.3 の範囲（Go のガードテストと `internal/doctestutil`、4 種類のリテラル）で明記する（AC-05）。
-   [x] "Translation Guidelines" 節を、`README.md` を先に変えて `/mktrans` で `README.en.md` に反映する流れに改める（AC-40）。

**完了の条件:** `make test` が通る（`TestInstructionDocPaths` を含む）。

#### ステップ 2-2: `_context.md` の改訂（AC-40）

**変更するファイル:** `.claude/commands/_context.md`

-   [x] 「Translation language pair」の行（`:54`）を、`README.md` → `README.en.md` の組だけを扱う `/mktrans` に合わせる。

**完了の条件:** `make test` が通る（`_context.md` を読む `fixpr_docs_test.go` のテストを含む）。

### フェーズ 3: `/mktrans` の改訂と同期の手順

#### ステップ 3-1: `/mktrans` の改訂（AC-38・AC-39）

**変更するファイル:** `.claude/commands/mktrans.md`

-   [x] 設計書 3.8 の表と 6.1 の判定の流れのとおりに改める。翻訳の組は `README.md` → `README.en.md` だけにし、`.ja.md` の方向の規則（`:27-35`）と developer_guide への言及（`:20-22`）を消す。同期点は同期の印のブロブの ID にする（設計書 2.4 の印の形と求め方）。
-   [x] 単独と下位の手順の違い（用語集の扱い、コミット、結果の報告の 4 つの値）を書く。コミットの指示は単独で呼んだときの節にだけ置く。
-   [x] 止まったときの復旧の手順（設計書 4.3 の表）と、`main` を取り込むときの衝突の規則（設計書 4.3）を書く。

**完了の条件:** 設計書 3.8 の表の 5 行と 6.1 の判定のそれぞれが、`mktrans.md` のどこに書かれているかを、PR の本文に挙げる。

#### ステップ 3-2: `_lib/task-pr-handoff.md` と `/runplan` の同期の手順

**変更するファイル:** `.claude/commands/_lib/task-pr-handoff.md`・`.claude/commands/runplan.md`

-   [x] Part B の手順 1 の後、手順 2 の前に、同期の手順を加える（設計書 4.3 の `/runplan` の項と結果の表）。手順 1 の中の番号のない段落として加え、Part B のどの項目の番号も変えない（`fixpr_docs_test.go:189`・`codex_review_docs_test.go:74`、CLAUDE.md の Development Notes が番号で参照する）。「README.md を変えた」の判定は、`$(git merge-base origin/main HEAD)..HEAD` のコミットと作業ツリーで行う。
-   [x] `runplan.md` の手順 5a の CHECKS の説明（`:82`）を、同期のコミットがあれば green gate を走らせ直すと改める。

**完了の条件:** `make test` が通る。

#### ステップ 3-3: `/fixpr` の同期の手順

**変更するファイル:** `.claude/commands/fixpr.md`

-   [x] Phase 3 の後、Phase 3.5 の前に、このセッションが同期の手順を実行すると書く（設計書 4.3 の `/fixpr` の項）。`README.en.md` の変更は、Phase 4 のコミットに入る。
-   [x] Phase 4.5 の修正のコミットが `README.md` を変えたときは、その `git push` の前に同期の手順をもう一度実行し、`README.en.md` の変更をコミットすると書く。コミットは `_lib/coderabbit-local-review.md` の修正のコミットと同じく文で指示し、`TestRemovedCommandLiterals` が禁じるコマンドの字句（`git add -A`・`git commit -m` など）を書かない。Authorization の段落（`fixpr.md:88-101`）に、このコミットを加える。
-   [x] 止まったときに作業ツリーに残る変更の扱いを、最終の報告の項に加える。
-   [x] 見出し（`fixpr_docs_test.go:30-37` の定数）は変えない。

**完了の条件:** `make test` が通る。

#### ステップ 3-4: `/runpr` の同期の手順と印の検査

**変更するファイル:** `.claude/commands/runpr.md`

-   [x] 手順 1 の単独の変更の PR を作る項で、ローカルのレビューの後、push の前に同期の手順を加える。チェックアウトしていないブランチで `README.md` の変更があれば止まると書く（設計書 4.3 の `/runpr` の項）。
-   [x] 手順 4 のマージの質問の前に、印の検査を加える（設計書 2.4。検査だけで反映しない。`README.en.md` がなければ検査しない）。
-   [x] `runpr.md` には `main` を取り込む手順がないので、手順 1 に、`main` を取り込むときに従う段落（「Taking in `main`」）を加え、`README.en.md` の衝突の規則（設計書 4.3）を `mktrans.md` の同名の段落に書いて参照する。
-   [x] 手順の見出し（`fixpr_docs_test.go:38-40`・`codex_review_docs_test.go:83` の定数）は変えない。

**完了の条件:** `make test` が通る。

#### ステップ 3-5: 同期の手順の位置のテスト

**変更するファイル:** `cmd/yt2column/fixpr_docs_test.go`

-   [x] 同期の手順は `fixprguard` の境界の内側で `/mktrans` を呼ぶので、CLAUDE.md の Testing Strategy が文書のテストを残すとする「信頼の境界でエージェントが実行するコマンド」に当たる。次を確かめるテスト `TestReadmeSyncStepPlacement` を加える。(1) `fixpr.md` の同期の手順が Phase 3 と Phase 3.5 の見出しの間にあり、`git add`・`git commit`・`git push` を含まない。(2) `mktrans.md` のコミットの指示が、単独で呼んだときの節の中にだけある。(3) `_lib/task-pr-handoff.md` の同期の手順が、Part B の項目 1 と項目 2 の間にある。同期の手順の見つけ方（見出しか、決まった語か）は実装で決め、決めたものを 3 つの文書で同じにする。
-   [x] 3 つの検査が、手順を別の Phase に移す、`mktrans.md` のコミットの指示を下位の手順の節に写す、同期の手順を Part B の項目 2 の後に移す、でそれぞれ失敗することを確かめる。

**完了の条件（フェーズ 3）:** `make test` が通る（コマンドの文書を読む `cmd/yt2column/fixpr_docs_test.go`・`codex_review_docs_test.go` のテストを含む）。同期の手順は、`fixprguard` の新しい呼び出しも、Codex のメンションも書かない（設計書 5 章）。

### PR-2 作成ポイント: language principles and README sync procedure

**対象ステップ**: 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `docs(0048): 文書の言語の原則を定め、README の同期の手順を加える`

**レビュー観点**: CLAUDE.md の原則・例外と、AC-06 の表の各文書の言語の一致 / `/mktrans` の振る舞いが設計書 3.8 の表と 6.1 の流れに一致すること / 3 つのコマンドの同期の手順の位置と、止まったときの振る舞い（設計書 4.3）、Phase 4.5 の後のコミット / `main` を取り込むときの衝突の規則

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-2〜3-4 は、`/fixpr` の Phase 3.5 と `fixprguard`、`/runpr` の手順 4 の PRE_MERGE という、ほかの手順の不変条件と交わる位置に手順を加える。読み違えると push の前の検査を飛ばしうる。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 共有パッケージ `internal/doctestutil`

このフェーズは PR-1 のマージの後に始める（ステップ 4-3 が `docs/dev/conventions/package_reference.md` を変えるため）。

#### ステップ 4-1: パッケージの新設と補助関数の移動

**変更するファイル:** `internal/doctestutil/`（新規。ファイルの分け方は実装で決める）、`cmd/yt2column/docs_test.go`

-   [ ] すべてのファイルに `//go:build test` を付けた `package doctestutil` を作る。
-   [ ] 1.5 の対応表の関数と型を移し、公開する。`splitTableRow`・`isTableSeparator`・`isClosingFence` は公開せずに移す。本体は変えない。例外は、`ReadDoc` が `testing.TB` を受け取ること、`FencedBlocks` が文書の文字列を受け取ること、`DocLine` のフィールドを公開することの 3 つである（設計書 3.3）。
-   [ ] `ReadDoc` の `os.ReadFile` に、行の単位の `//nolint:gosec` を付ける。理由として、読むパスはガードテストが指定するリポジトリ内の文書であることを、テストの文脈に限って書く（1.4 の前例）。ほかの移した関数で、`_test.go` の除外がなくなって出る lint の指摘も直す。
-   [ ] `TestSectionByHeading`（`cmd/yt2column/docs_test.go:347`）・`TestScanDocLines`・`TestFencedBlocks`・`TestAnchorFromHeading`・`TestSectionOutsideBlocks` を移す。
-   [ ] `TestAnchorFromHeading` の表に、設計書 3.4 で確かめた日本語の見出しの行を加える（設計書 3.4 の「確かめた文字」だけを使う）。

**完了の条件:** `go test -tags test ./internal/doctestutil` が通る。

#### ステップ 4-2: 呼び出し元の付け替えとコピーの削除

**変更するファイル:** `cmd/yt2column/*_test.go`（1.4 の呼び出し元のあるもの）、`cmd/yt2column-server/docs_test.go`

-   [ ] `cmd/yt2column` と `cmd/yt2column-server` の呼び出しを `doctestutil` の関数にする。
-   [ ] `cmd/yt2column-server/docs_test.go` の 9 つのコピー（1.5 の表の関数。`tablesByFirstHeader` は残す）と、重複を説明するコメント（`:363-368`）を消す。
-   [ ] `cmd/yt2column-server/docs_test.go:213` の `TestSectionByHeading` を消す。
-   [ ] 網羅を比べる。移す前に `cmd/yt2column` のテストで `cmd/yt2column` の関数の `go tool cover -func` を取り、移した後に 3 つのパッケージ（`internal/doctestutil`・`cmd/yt2column`・`cmd/yt2column-server`）のテストで `doctestutil` の関数について同じものを取る。両者を 1.5 の対応表で同じ関数どうしに対応させて比べる。消したサーバのコピーは比べない（関数ごとなくなるため）。使った `-coverpkg` と結果をコミットのメッセージに書く。

**完了の条件:** `make test` が通る。

#### ステップ 4-3: テスト専用のパッケージとしての登録

**変更するファイル:** `internal/pipeline/pipeline_test.go`、`docs/dev/conventions/package_reference.md`

-   [ ] `testOnlyPackageDirs`（`:583`）に `internal/doctestutil` を加える。
-   [ ] `package_reference.md` に `internal/doctestutil` の行を加える（`TestPackageReferenceListsPackages` が求める）。
-   [ ] `internal/doctestutil` の 1 つのファイルから `//go:build test` を一時的に外し、`TestFakesCarryBuildTag` が失敗することを確かめる。

**完了の条件（フェーズ 4）:** `make test`・`make lint`・`make deadcode` が通る。

### PR-3 作成ポイント: shared doc test helpers

**対象ステップ**: 4-1 / 4-2 / 4-3

**推奨タイトル**: `test(0048): ガードテストの Markdown の補助関数を internal/doctestutil に移す`

**レビュー観点**: 移した関数の本体が変わっていないこと（3 つの例外と lint の対処を除く） / 公開する関数の一覧と呼び出し元の数（1.4） / 網羅の比較の結果 / 日本語の見出しのアンカーの行

**実装モデル要件**: standard

**判定理由**: 本体を変えない移動で、関数の一覧と形は 1.4 と設計書 3.3 が決めている。Conditional checks のうち当たるのは build tag と lint の抑止の 2 つだが、どちらも前例のある機械的な対処である。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: README の振り分け

#### ステップ 5-1: 始める条件の確認

**変更するファイル:** 本書、必要なら設計書

-   [ ] 0046 の PR-4 がマージされていることを確かめる。マージされていなければ始めない（要件書 §1「進行中のタスクとの順序」）。
-   [ ] PR-4 が README のどこを変えたかを確かめる。設計書 3.6 が想定する範囲（Browser extension の節のポップアップの説明）の外を変えていれば、設計書 3.6・3.7 と本書の 4.2 を作り直してから進む（設計書 3.6）。範囲の中だけなら、PR-4 の後の README の行番号と設計書 3.6 の行番号の対応を、本書の冒頭に表で加える。

**完了の条件:** 確かめた結果が本書にある。

#### ステップ 5-2: 英語のまま移す

**変更するファイル:** `README.md`、`docs/dev/guides/cli.md`・`development.md`（新規）、README を読むガードテスト（4.2 の表で PR-4 の行）、`extension/test/metadataDocs.test.ts`・`serverReasons.test.ts`・`ciChanges.test.ts`、`extension/scripts/has-extension-changes.sh`

-   [ ] 設計書 3.6 の表と表 3-2 のとおり、移す文・句・節を、英語のまま、文言を変えずに `cli.md` と `development.md` に移す。句だけを移した文は、残りの部分を README に残す。README には、移った先への短い案内を残す。移る文の中のリンクは、設計書 3.6 の「リンクの付け替え」のとおりにする（AC-25）。
-   [ ] README の Checking the server with curl の後に、3 列の表の `reason` と `extension shows` の 2 列から、2 列の表（13 行）を作る（設計書 3.6 の表 3-2、AC-19）。
-   [ ] `cli.md` に、README の Configuration の節へのリンクを置く（AC-36）。
-   [ ] Browser extension の節を `## Browser extension` にし（1.3）、`metadataDocs.test.ts:25` の見出しを同じにする。
-   [ ] 4.2 の表のうち PR-4 の行のテストを、移った先のファイルを読むようにする。この時点では見出しと列名は英語のままである。
-   [ ] `TestReadmeLinks` を、README・`cli.md`・`development.md` の 3 つに広げ、文書をまたぐアンカーの検査を加える（設計書 4.2）。`TestInstructionDocPaths` の対象から `docs/dev/guides/cli.md`・`development.md` を外す（`TestReadmeLinks` が確かめるため）。
-   [ ] `serverReasons.test.ts` を、`development.md` の表を読むようにする。
-   [ ] `has-extension-changes.sh` に `docs/dev/guides/development.md` を加え、先頭のコメントを、拡張のテストが読む文書の今の一覧に合わせる（1.3）。`ciChanges.test.ts` の走らせる表に `docs/dev/guides/development.md` を、走らせない表に `docs/dev/guides/cli.md` と、似ているが違うパス（例：`x/docs/dev/guides/development.md`・`docs/dev/guides/development.md.orig`）を加える。
-   [ ] このステップだけでコミットする（以下「英語の移動のコミット」）。

**完了の条件:** `make test`・`make ext-check` が通る。

#### ステップ 5-3: 振り分けの差分の検査（AC-19〜AC-22・AC-24）

**変更するファイル:** `docs/tasks/0048_doc_language_policy/_checks/split/main.go`・同ディレクトリの分けた文の一覧（新規）、本書

-   [ ] merge-base の `README.md` の文・表の行・コードブロックの行のそれぞれについて、英語の移動のコミットの README・`cli.md`・`development.md` のどれにあるかを確かめるプログラムを書く。文の単位で比べ（段落の折り返しは比較の前に戻す）、リンクの先は比較から外す。
-   [ ] 句を移した文（設計書 3.6 の表 3-2 の「前半」「後半」「句」の行）は、同じディレクトリに置く一覧で扱う。一覧の各行は、元の文、README に残る部分、移った部分とその先の文書である。プログラムは、一覧の各行の部分が、示した文書にそれぞれあり、合わせると元の文になることを確かめる。一覧は PR のレビューで設計書の表 3-2 と照らす。
-   [ ] 出力は、文ごとの行き先（R・C・D）、2 つ以上の文書にある文の一覧（AC-22 の重複の読み合わせに使う）、見つからない文の一覧である。見つからない文があるときだけ失敗する。新しく加えた文（案内と 2 列の表）は失敗の理由にしない。
-   [ ] `docs/dev/guides/` に `cli.md`・`development.md` のほかの版（例：`cli.en.md`）がないことも確かめる（AC-24）。
-   [ ] 1 つの文を消す、表の 1 行を消す、一覧の 1 行の移った部分を消す、`cli.en.md` を置く、でそれぞれ失敗することを確かめる。
-   [ ] `go run -tags test ./docs/tasks/0048_doc_language_policy/_checks/split/ <merge-base> <英語の移動のコミット>` の形で実行し、出力と 2 つのコミットの ID を 7 章の記録に書く。

**完了の条件:** 終了コードが 0 である。

#### ステップ 5-4: `cli.md`・`development.md` の翻訳

**変更するファイル:** `docs/dev/guides/cli.md`・`development.md`、`README.md`、`internal/doctestutil/`、ステップ 5-2 で変えたガードテスト、`extension/test/helpers/`・`serverReasons.test.ts`、`docs/dev/guides/pricing_update.md`・`claude_output_limit.md`、`docs/translation_glossary.md`、`_checks/split/`

-   [ ] 1.3 の手段で、`cli.md` と `development.md` を日本語にする。翻訳の元は英語の移動のコミットの 2 つのファイルである（AC-24）。
-   [ ] README の中の、2 つの文書の見出しへのリンクのアンカーを、日本語の見出しのアンカーにする（AC-25）。
-   [ ] 2 つの文書の見出しと列名を、日本語の文字列の定数として `doctestutil` に置く（設計書 3.3）。拒否の理由の表示の区分の表（区分ごとの日本語と英語の書き方。設計書 3.7 の末尾）と、セルを区分に分類する関数も置く。この関数は、知らない書き方をどの区分にも当てはめずに失敗させる。ガードテストは、これらの定数で節と表を探す。
-   [ ] 区分に分類する関数の表のテストを加える。行は、各区分の各言語の書き方、条件付きの形、知らない書き方である。
-   [ ] `rejection_reasons` のサブテストを設計書 3.7 の行のとおりにする。この時点の README は英語の 1 版なので、README の 2 列の表は英語の書き方の区分で読む。README の 2 列の表の 1 つのセルを別の区分にすると失敗することを確かめる。
-   [ ] `extension/test/helpers/` に `development.md` の拒否の理由の表の列名と区分の書き方を置き、`serverReasons.test.ts` がそれを使う（設計書 3.5）。
-   [ ] `pricing_update.md`・`claude_output_limit.md` の参照のうち、`cli.md` に移った節を指すものを、`cli.md` の見出しにする（AC-26、1.3）。
-   [ ] `_checks/split` に、翻訳の前後（英語の移動のコミットと PR の先頭）の `cli.md`・`development.md` で、見出しのレベルの並び、コードブロックの内容、表の行の数が同じであることの検査を加え、実行して 7 章に記録する。
-   [ ] 4.2 の表の PR-4 の行の各テストが、移った先の値を壊すと失敗することを確かめる（例：`cli.md` のフラグの表から 1 行消す、`development.md` の Job API の表の 1 つのコードを変える）。

**完了の条件（フェーズ 5）:** `make test`・`make lint`・`make deadcode`・`make ext-check` が通る。

### PR-4 作成ポイント: README split into developer documents

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4

**推奨タイトル**: `docs(0048): README の開発者向けの内容を cli.md と development.md に移す`

**レビュー観点**: 振り分けと分けた文の一覧が設計書 3.6 の表と表 3-2 に一致すること（AC-19〜AC-21） / `_checks/split` の出力（行き先と重複の一覧、AC-22） / `cli.md`・`development.md` の翻訳の正確さ（AC-24） / ガードテストの付け替えが 4.2 の表の PR-4 の行に一致すること（AC-32）

**実装モデル要件**: frontier-recommended

**判定理由**: 文の単位の振り分けは判断が要り（設計書 3.6）、ガードテストの付け替えと CI の判定の変更（Conditional checks の CI の判定）が同時に入る。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: README の日英二言語化

#### ステップ 6-1: 構造の抽出と比較

**変更するファイル:** `internal/doctestutil/`

-   [ ] 設計書 3.3 の `HeadingPos`・`Structure`・`ParseStructure` と、4.1 の 3 つの番兵のエラーを加える。走査は `ScanDocLines` を使い、規則を変えない。拒否する形は設計書 4.1 のとおりである。
-   [ ] 2 つの `Structure` を比べ、食い違いを型で返す関数を加える。結果は、食い違った項目（見出しの並びかコードブロックの数か）、最初に食い違った見出しの位置、両版のその見出しの `HeadingPos` を持つ。README のテスト（ステップ 6-7）は、この結果をメッセージにするだけにする（設計書 4.1 の失敗のメッセージ）。両方が食い違うときに何を返すかは実装で決め、テストで固定する。
-   [ ] `ParseStructure` の表のテストを加える。行は設計書 7.1 の「`ParseStructure` 自体のテスト」に加えて、境界（`#` が 6 個と 7 個、空白のない `#######x`、空行の後の `---`、フェンスの中の Setext の下線、情報の文字列付きのフェンス、3 個のバッククォートで開いたブロックを 4 個で閉じる行）を持つ。拒否する形の行は `errors.Is` で番兵を確かめる。
-   [ ] 比較の関数の表のテストを加える。行は、一致、途中のレベルの違い、一方が他方の先頭の部分である並び、コードブロックの数だけの違い、両方の違い、文言だけが違う見出し（一致する）である。
-   [ ] `ParseStructure` から拒否の 1 つを外すと、その行が失敗することを確かめる。

**完了の条件:** `make test`・`make lint`・`make deadcode` が通る。

### PR-5 作成ポイント: README structure parser

**対象ステップ**: 6-1

**推奨タイトル**: `test(0048): README の構造の抽出と比較を加える`

**レビュー観点**: 拒否する形が設計書 4.1 に一致し、どれも番兵で返ること / 走査の規則を変えていないこと / 比較の結果の型と、両方が食い違うときの扱い / 1〜3 個の空白で字下げした ATX の見出し（GitHub は見出しとして表示し、`HeadingLevel` は見出しとしない。設計書 4.1 に扱いがない）を拒否するかどうか

**実装モデル要件**: standard

**判定理由**: 型とエラーと拒否する形は設計書 4.1 が決めている。Conditional checks のうち当たるのは build tag の 1 つだけで、PR-3 と同じ対処である。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

ステップ 6-2〜6-4 のコミットでは、README を読むテストが失敗してよい（1.2）。ステップ 6-5・6-6 で直す。

#### ステップ 6-2: `README.md` の日本語化

**変更するファイル:** `README.md`、`docs/dev/guides/cli.md`・`development.md`、必要なら `extension/test/helpers/repoFile.ts`・`markdownSection.test.ts`

-   [ ] 1.3 の手段で、`README.md` を日本語にする。翻訳の元はこの PR の merge-base の `README.md` である。内容を加えも削りもしない（AC-27 の例外を除く）。AC-27a の書き換えは、設計書 3.6 の「CLI を主語や比較の対象にした文」の一覧のとおりにする。
-   [ ] ポップアップの文言は、日本語のカタログ（`ja.ts`）の文言を引用し、引用の言語の説明を日本語の版に合わせる（AC-31a）。
-   [ ] 先頭（最初の見出しより前）に、`README.en.md` へのリンク（AC-28）と、`cli.md`・`development.md` へのリンク（AC-23）を置く。
-   [ ] `cli.md`・`development.md` の中の、README の見出しへのリンクのアンカーを、日本語の見出しのアンカーにする（AC-25・AC-36）。
-   [ ] 見出しには、設計書 3.4 で確かめた文字だけを使う。使えないときは、設計書 3.4 の方法で確かめ、結果を 7 章に記録する。
-   [ ] Browser extension の節の中に `###` の小節を置くときは、`markdownSection` を、見出しと同じか高いレベルの次の見出しで節を終える形に改め（設計書 3.5）、`markdownSection.test.ts` に、`##` の節の中の `###` で切れない行と、次の `##`・`#` で終わる行を加える。今の行は変えずに通す。

**完了の条件:** 翻訳のレビューに Critical と Major の指摘が残っていない。

#### ステップ 6-3: 用語集（AC-34）

**変更するファイル:** `docs/translation_glossary.md`

-   [ ] AC-34 の (1)〜(3) のうち、用語集にないものを加える。(3) の候補は、`README.md` の 2 つ以上の節の地の文に現れる、概念を表す語である。加えた語と、候補から外した語とその理由を、PR の本文に挙げる。

**完了の条件:** PR の本文に一覧がある。(1) はステップ 6-10 の検査が確かめる。

#### ステップ 6-4: `README.en.md` の作成

**変更するファイル:** `README.en.md`（新規）、`docs/translation_glossary.md`

-   [ ] 改訂した `/mktrans` を単独で `README.md` に対して呼び、全文を翻訳させる。1 行目に同期の印、先頭に `README.md` へのリンクがあることを確かめる（AC-27・AC-28）。`/mktrans` のコミットの手順（翻訳と用語集を別のコミット）に従う。
-   [ ] ポップアップの文言が英語のカタログ（`en.ts`）の文言であることを確かめる（AC-31a）。

**完了の条件:** `/mktrans` の翻訳のレビューに Critical と Major の指摘が残っていない。

#### ステップ 6-5: README の版とロケータ

**変更するファイル:** `internal/doctestutil/`、`extension/test/helpers/`

-   [ ] 設計書 3.3 の `Lang`・`ReadmeVersion`・`ReadmeVersions`・`Locator`・`In` と、README のロケータの表を加える。ロケータは、4.2 の表で「両版」を読むテストが使う見出し、列名、項目の札（`TestServerDocsContract` のソケットのパスの札など）である。`ReadmeVersion.Path` はリポジトリのルートからのパスなので、各テストのパッケージのディレクトリから読めるよう、ルートのパスと結合する関数も用意する。
-   [ ] `Locator.In` の表のテストを加える。行は、日本語と英語の綴り、`LangUnknown`、範囲の外の値、空の綴りである。失敗させる行は、`testing.TB` の偽物（`internal/llm/claude/claude_test.go:196-207` の形）で確かめる。偽物の `Fatalf` は処理を止めないので、`In` は失敗を報告した後に戻ることを確かめる。
-   [ ] `extension/test/helpers/` に、README の版（パスと `Locale`）と、Browser extension の節の見出しの英日の組を置く（設計書 3.5）。

**完了の条件:** `go test -tags test ./internal/doctestutil` が通る。

#### ステップ 6-6: ガードテストを両版に広げる（AC-31・AC-32）

**変更するファイル:** 4.2 の表で PR-6 の行のテストのファイル、`cmd/yt2column-nmhost/main_test.go`、`extension/test/metadataDocs.test.ts`

-   [ ] 4.2 の表の PR-6 の行のテストを、`ReadmeVersions()` で回す版ごとのサブテスト（`t.Run` の名前に版のパス）にする。`TestReadmeHostExitCodes` は `doctestutil` を使う形にする。
-   [ ] `TestReadmeLinks` に `README.en.md` を加える。
-   [ ] `metadataDocs.test.ts` を、`README.md` は日本語のカタログ、`README.en.md` は英語のカタログと比べるようにする（AC-31a、設計書 3.5）。0046 の PR-4 が加えた確認も同じにする。
-   [ ] 各テストが、片方の版だけで値を壊したときに、その版の名前で失敗することを確かめる（例：`README.en.md` の拡張の ID を変える、`README.md` のホストの終了コードの表から 1 行消す）。`TestReadmeLinks` が、`docs/dev/guides/cli.md` から見て存在しない相対パスと、存在しないアンカー（`../../../README.md#x` の形）で失敗することを確かめる。

**完了の条件:** `make test`・`make ext-check` が通る。

#### ステップ 6-7: 構造の一致・同期の印・先頭のリンクのテスト（AC-23・AC-28・AC-29・AC-36）

**変更するファイル:** `cmd/yt2column/readme_structure_test.go`（新規）

-   [ ] `TestReadmeVersionsShareStructure` を加える。両版を `ParseStructure` で読み、ステップ 6-1 の比較の結果をメッセージにして失敗する。ファイルがないとき（スキップしない）と、`ParseStructure` がエラーを返すときは、版のパスを示して失敗する。
-   [ ] `TestReadmeSyncMarker` を加える。`README.en.md` の 1 行目が同期の印の形で、印が 1 つだけであることを確かめる（設計書 2.4・4.1 の末尾。ID の長さは 1.3）。印を読む部分を文字列だけを受け取る関数にし、表のテストを持たせる。行は、正しい印、短い ID、41 文字の ID、大文字の 16 進、`README.md` の語がない、2 行目の印、コードブロックの中の 2 つ目の印、余分な空白である。
-   [ ] `TestReadmeVersionLinks` を加える。両版の最初の見出しより前に、もう一方の版へのリンクがあり、両版に `cli.md`・`development.md` へのリンクがあることを確かめる（設計書 7.1）。`cli.md` に、README の設定の節の見出し（ロケータの日本語の綴り）のアンカーへのリンクがあることも確かめる（AC-36）。
-   [ ] 設計書 7.1 の例（`README.en.md` の見出しを 1 つ消す、`README.md` だけにコードブロックを加える、`README.en.md` を消す）と、印を消す、印を 2 つにする、先頭のリンクを消す、`cli.md` の設定の節へのリンクを消す、でそれぞれ失敗することを確かめる。

**完了の条件:** `go test -tags test ./cmd/yt2column` が通る。

#### ステップ 6-8: CI の判定と UI の参照先（AC-33・AC-35・AC-35a・AC-36）

**変更するファイル:** `extension/scripts/has-extension-changes.sh`、`extension/test/ciChanges.test.ts`、`cmd/yt2column/run.go`・`run_test.go`、`cmd/yt2column-server/run.go`・`run_test.go`、`extension/src/core/i18n/en.ts`、`extension/test/metadataDocs.test.ts`

-   [ ] `has-extension-changes.sh` に `README.en.md` を加え、先頭のコメントを合わせる。`ciChanges.test.ts` の走らせる表に `README.en.md` を、走らせない表に似ているが違うパス（例：`docs/README.en.md`・`README.en.md.bak`）を加える。
-   [ ] `run.go:225` を、`(see README)` が `(see docs/dev/guides/cli.md)` になる形に、`:259` を、`see README.md.` が `see docs/dev/guides/cli.md.` になる形にする。`TestRunHelp` に、`--slack` のフラグの行と使い方の文のそれぞれが `docs/dev/guides/cli.md` を示し、そのファイルが存在することの検査を加える（AC-36）。2 か所を 1 つずつ元に戻し、それぞれで失敗することを確かめる。
-   [ ] `cmd/yt2column-server/run.go:145` を、`(see README.md)` が `(see README.en.md)` になる形にする。`TestStartupFailures` の引数の行で、診断が `README.en.md` を示し、そのファイルが存在することを確かめる（AC-35a）。
-   [ ] `en.ts:83` を、`the permissions the README describes.` が `the permissions README.en.md describes.` になる形にする（文全体の最終の形は、英語の文として実装で整える）。`ja.ts:81` は変えない。英語のカタログのこのメッセージが `README.en.md` を含み、そのファイルが存在することを確かめるテストを `metadataDocs.test.ts` に加える（AC-35）。
-   [ ] サーバの診断と拡張のメッセージのテストが、文字列を元に戻すと失敗することを確かめる。

**完了の条件:** `make test`・`make ext-check` が通る。

#### ステップ 6-9: 文書の参照と CLAUDE.md（AC-07・AC-26・AC-41）

**変更するファイル:** `CLAUDE.md`、`docs/dev/project_overview.md`、`docs/dev/production_profile.md`、`docs/translation_glossary.md`、`docs/dev/guides/pricing_update.md`・`claude_output_limit.md`

-   [ ] `CLAUDE.md:212-213` を、サーバの API は `development.md`、起動は README に書く形にする（AC-07）。
-   [ ] Development Notes の文書を読むガードテストの説明（`CLAUDE.md:431-448`）を、4.2 の表の後の状態に合わせる。`TestReadmeHostExitCodes` が両版を読み、`doc-guard` では走らないことを書く（AC-41）。
-   [ ] `project_overview.md:157`、`production_profile.md:5`、`translation_glossary.md:53`、`pricing_update.md`・`claude_output_limit.md` の README に残る節への参照を、日本語の見出しにする（AC-26）。過去のタスク文書以外で README の節を名前で参照する箇所をほかにも探し（`git grep -n README` を `docs/tasks/` を除いて読む）、同じにする。

**完了の条件:** `make test` が通る。

#### ステップ 6-10: README の差分の検査（AC-27・AC-34・AC-35）

**変更するファイル:** `docs/tasks/0048_doc_language_policy/_checks/readme/main.go`（新規）、本書

-   [ ] merge-base と PR の先頭のコミットを比べるプログラムを書く。(1) `extension/src/core/i18n/ja.ts` が同じであること（AC-35）。(2) `README.md` の翻訳の前後で、見出しのレベルの並び、コードブロックの内容、表の行の数が同じであること。例外は、AC-31a の引用、リンクの先、先頭の 2 つのリンク、AC-27a の書き換えで、例外の箇所は一覧で出力する（AC-27）。(3) `README.md` の各見出しが用語集の日本語の欄にあり、同じ位置の `README.en.md` の見出しが、その行の英語の欄と大文字と小文字を除いて同じであること（AC-34 の (1)）。
-   [ ] 3 つの検査が、`ja.ts` を変える、`README.md` のコードブロックの 1 文字を変える、用語集から見出しの 1 行を消す、でそれぞれ失敗することを確かめる。
-   [ ] 実行し、出力と 2 つのコミットの ID を 7 章の記録に書く。

**完了の条件（フェーズ 6）:** プログラムが終了コード 0 で終わる。`make test`・`make test-parse-norace`・`make lint`・`make deadcode`・`make ext-check` が通る。7 章のすべての行が確かめられている。PR-6 の `doc-guard` のジョブのログで、`TestReadmeVersionsShareStructure` が走ったことを確かめ、7 章に記録する（AC-29 の CI の部分）。

### PR-6 作成ポイント: bilingual README and guard tests

**対象ステップ**: 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 6-9 / 6-10

**推奨タイトル**: `docs(0048): README を日本語の正本と英語版の二言語にする`

**レビュー観点**: `README.md` の翻訳の正確さと AC-27a の書き換え（AC-27） / `README.en.md` と用語集の一致（AC-34） / ガードテストが 4.2 の表のとおり両版を検査すること（AC-31・AC-32） / 構造の一致のテストと同期の印のテスト（AC-29）

**実装モデル要件**: frontier-recommended

**判定理由**: 2 つの翻訳と十数のガードテストの変更が 1 つの PR に入り、Conditional checks のうち CI の判定とテストの build tag の 2 つに当たる。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序とマイルストーン

フェーズは設計書 8 章の順序に従う。フェーズ 2 はフェーズ 1 の新しいパスを書く。フェーズ 4 は PR-1 のマージの後に始める（ステップ 4-3）。フェーズ 5 は 0046 の PR-4 のマージとフェーズ 4 の後に始める。フェーズ 6 はフェーズ 3（改訂した `/mktrans`）とフェーズ 5 の後に始める。

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1（PR-1） | `docs/dev/conventions/`・`docs/dev/guides/`、旧パスの置換、パスの検査のテスト | PR-1 のマージ。`docs/dev/developer_guide/` がない |
| M2（PR-2） | CLAUDE.md の原則、改訂した `/mktrans`、3 つのコマンドの同期の手順 | PR-2 のマージ。`README.en.md` はまだないので、同期の手順は何もしない |
| M3（PR-3） | `internal/doctestutil` | PR-3 のマージ |
| M4（PR-4） | `cli.md`・`development.md`、README は英語の 1 版 | PR-4 のマージ |
| M5（PR-5） | `ParseStructure` と構造の比較 | PR-5 のマージ |
| M6（PR-6） | 日本語の `README.md`、`README.en.md`、両版のガードテスト | PR-6 のマージ。7 章のすべての行が確かめられている |

PR-1・PR-2・PR-3 は 0046 の PR-4 を待たずに進めてよい（要件書 §1）。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 | developer_guide の分割、旧パスの置換、2 つの手引きの翻訳、パスの検査のテスト、移動の差分の検査 | standard |
| PR-2 | 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | CLAUDE.md と `_context.md` の言語の原則、`/mktrans` の改訂、同期の手順とその位置のテスト | frontier-recommended |
| PR-3 | 4-1 / 4-2 / 4-3 | `internal/doctestutil` の新設と補助関数の移動 | standard |
| PR-4 | 5-1 / 5-2 / 5-3 / 5-4 | README の振り分け、`cli.md`・`development.md`、移った値を読むガードテスト、Browser extension の最上位化 | frontier-recommended |
| PR-5 | 6-1 | `ParseStructure` と構造の比較 | standard |
| PR-6 | 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 6-9 / 6-10 | `README.md` の日本語化、`README.en.md`、両版のガードテスト、構造の一致のテスト、UI の参照先 | frontier-recommended |

## 4. テスト戦略 (Test Strategy)

### 4.1. 加える・変えるテスト

| テスト | ステップ |
|---|---|
| `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths` と、参照の抜き出しの表のテスト（新規） | 1-5・5-2 |
| `cmd/fixprguard/docs_test.go::TestGuidePathExists`（新規）、同ファイルの手引きを読む各テスト（1 ファイルにする） | 1-3・1-5 |
| `cmd/yt2column/fixpr_docs_test.go::TestReadmeSyncStepPlacement`（新規） | 3-5 |
| `internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`（パス）・`testOnlyPackageDirs` | 1-3・4-3 |
| `internal/doctestutil` の移したテスト（`TestSectionByHeading`・`TestScanDocLines`・`TestFencedBlocks`・`TestAnchorFromHeading`・`TestSectionOutsideBlocks`） | 4-1 |
| `internal/doctestutil` の区分に分類する関数・`ParseStructure`・構造の比較・`Locator.In` の表のテスト（新規） | 5-4・6-1・6-5 |
| 4.2 の表のガードテスト | 5-2・5-4・6-6 |
| `cmd/yt2column/readme_structure_test.go` の `TestReadmeVersionsShareStructure`・`TestReadmeSyncMarker`（印を読む関数の表のテストを含む）・`TestReadmeVersionLinks`（新規） | 6-7 |
| `cmd/yt2column/run_test.go::TestRunHelp`、`cmd/yt2column-server/run_test.go::TestStartupFailures` | 6-8 |
| `extension/test/markdownSection.test.ts`（改修するときだけ） | 6-2 |
| `extension/test/serverReasons.test.ts` | 5-2・5-4 |
| `extension/test/metadataDocs.test.ts` | 5-2・6-6・6-8 |
| `extension/test/ciChanges.test.ts` | 5-2・6-8 |

テストの名前は、新しく加えるものを本書で決めた。既存のテストの名前は変えない。

### 4.2. ガードテストの前後の表（AC-32）

本書の作成時に README を読むガードテストが検査する値と、本タスクの後にその値を検査する文書である（設計書 3.7 をもとに、値を `c76eec26` のテストで確かめて加えた）。「PR」の列は、読む文書を変える PR である。

| テスト | 今 README で検査する値 | 本タスクの後に検査する文書 | PR |
|---|---|---|---|
| `TestREADMEDocumentsFlags`（`cmd/yt2column/docs_test.go:173`） | フラグの表の、`newFlagSet` のすべてのフラグ | `cli.md` | PR-4 |
| `TestSlackDocsContract`（`:191`） | `16,383` と、Webhook の統合テストの変数・`make` のターゲット | `16,383` は両版、統合テストの値は `development.md` | PR-4・PR-6 |
| `TestClaudeDocsContract`（`:239`） | Claude の統合テストの変数 | `development.md` | PR-4 |
| `TestDeepSeekDocsContract`（`:261`） | 3 つの統合テストの節の変数（API キー・明示の同意・モデル） | `development.md` | PR-4 |
| `TestLLMDefaultsDocsContract`（`:302`） | 設定の表の `Variable`・`When unset` の列の値 | 両版（日本語の列名は `変数`・`未設定のとき`） | PR-6 |
| `TestProductionProfileDocsContract`（`:934`） | Recommended configuration の設定のブロックの値と、それが Configuration の節の中にあること（表は `production_profile.md` で検査する） | 両版 | PR-6 |
| `TestMigrationDocsContract`（`:976`） | Migrating の旧変数と新しい変数の対応 | 両版 | PR-6 |
| `TestQuickStartLinks`（`:1036`） | Quick start の順序、各項目の README の見出しへのリンク、外部の URL がないこと | 両版 | PR-6 |
| `TestProductionProfileLinks`（`:1079`） | Recommended configuration の `production_profile.md` へのリンク | 両版（README 以外は今のまま） | PR-6 |
| `TestReadmeLinks`（`:1119`） | すべてのリンクの先とアンカー、リンクの開きと抜き出しの数の一致 | README・`cli.md`・`development.md`（PR-4）、`README.en.md`（PR-6） | PR-4・PR-6 |
| `TestTokenUsageDocsContract`（`:1255`） | Token usage の `tokens:` の行の例 | `cli.md` | PR-4 |
| `TestCostDocsContract`（`:1313`） | Estimated cost の `cost:`・`cost-total:` の行の語、料金の根拠の語、理由の語（`project_overview.md` と `rates.go` のコメントも読む） | `cli.md`（README 以外は今のまま） | PR-4 |
| `TestOutputLayoutDocsContract`（`output_layout_docs_test.go:92`） | `--out` のファイルの例と Slack の投稿の例 | `--out` の例は `cli.md`（PR-4）、Slack の投稿の例は両版（PR-6） | PR-4・PR-6 |
| `TestExtensionDocumentation`（`extension_docs_test.go:52`） | 拡張の ID | 両版（CLAUDE.md と `security.md` は今のまま） | PR-6 |
| `TestServerDocsContract` の `README.md`（`cmd/yt2column-server/docs_test.go:42`） | Upgrading の小節を除いた Localhost server の節の、`SLACK_WEBHOOK_URL`、ソケットのパス（札 `macOS:` などの行）、終了コードの表、curl の例（`POST /articles` と Job API の 3 つ） | 前の 3 つは両版（PR-6）、curl の例は `development.md`（PR-4） | PR-4・PR-6 |
| 同 `rejection_reasons`（`:96`） | 3 列の表の `reason` とステータスコード、行の数 | 3 列の表は `development.md`（PR-4）。2 列の表の `reason` の集合と表示の区分は README（PR-4 は英語の 1 版、PR-6 は両版） | PR-4・PR-6 |
| 同 `api_codes`（`:124`） | Job API codes の 6 つの表 | `development.md` | PR-4 |
| `TestReadmeHostExitCodes`（`cmd/yt2column-nmhost/main_test.go:436`） | ホストの終了コードの表 | 両版 | PR-6 |
| `serverReasons.test.ts:44` | 3 列の表の「fixed message」の行 | `development.md` の 3 列の表 | PR-4 |
| `metadataDocs.test.ts:24-35` | Browser extension の節の英語の促しのメッセージ（0046 の PR-4 の後は主ボタンの名前も） | 節の見出しは PR-4 で `##`。`README.md` は `ja.ts`、`README.en.md` は `en.ts` の文言（PR-6） | PR-4・PR-6 |

ステップ 5-1 で設計書 3.7 を作り直したときは、この表も作り直す。

### 4.3. 差分の検査

設計書 7.3 の AC、AC-10・AC-19〜AC-22・AC-24・AC-27・AC-34 の構造と値の部分、AC-35 の変えない部分は、`_checks/` のプログラムで確かめる。`_checks/move/` は PR-1（ステップ 1-6）、`_checks/split/` は PR-4（ステップ 5-3・5-4）、`_checks/readme/` は PR-6（ステップ 6-10）で実行し、結果を 7 章に記録する。`make test` と CI では実行せず、マージの後は保守しない。

### 4.4. 統合テスト

N/A（設計書 7.2）。

### 4.5. 既存のテスト

コマンドの文書を読むテスト（`cmd/yt2column/fixpr_docs_test.go`・`codex_review_docs_test.go` の各テスト）と `cmd/yt2column/copied_transcript_docs_test.go` は、検査する見出しと行を変えずに通す（設計書 3.7、H-08）。変わるのは、`doctestutil` に移した補助関数の呼び出しだけである（PR-3）。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| PR-1 のマージの後に、開いているブランチが旧パスを持ち込む | エージェントが規約を読めないまま進む | 指示と規約の文書の相対リンクとバッククォートの `docs/dev/…md` は、`TestInstructionDocPaths` が CI で確かめる。それ以外の書き方と過去のタスク文書は検査しないので、PR-1 の本文で知らせる（ステップ 1-7） |
| タスク 0049 が同じコマンドの文書を変える | PR-2 と 0049 の PR が衝突する | 先にマージされた方に、後の PR が `main` を取り込んで合わせる。取り込みの後、同期の手順の位置を `TestReadmeSyncStepPlacement` で確かめる |
| 0046 の PR-4 が想定の外の README を変える | 設計書 3.6・3.7 が古くなる | ステップ 5-1 で確かめ、外なら設計書を作り直してから進む |
| PR-4 から PR-6 までの間、CLAUDE.md の Development Notes が古い README の表を述べる | 読み手が古い説明を読む | PR-4・PR-5 の 2 つの PR の間である。`has-extension-changes.sh` のコメントは PR-4 で直す。CLAUDE.md は PR-6 で直す（ステップ 6-9） |
| PR-6 が大きく、レビューに時間がかかる | マージが遅れ、README に触れるほかの PR と衝突する | 構造の抽出を PR-5 に分けた（1.3）。PR-6 の間は README に触れる別の変更を入れない |
| 翻訳で内容が落ちる・増える | AC-10・AC-24・AC-27 を満たさない | 1.3 の翻訳のレビューと、`_checks/` の構造と値の検査（4.3）で確かめる |
| 日本語の見出しのアンカーが GitHub と違う | リンクの検査が通っても、GitHub でリンクが切れる | 設計書 3.4 で確かめた文字だけを見出しに使う（ステップ 6-2） |

## 6. 実装チェックリスト (Implementation Checklist)

-   [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7）
-   [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5）
-   [ ] PR-3 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3）
-   [ ] PR-4 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4）
-   [ ] PR-5 マージ済み（対象ステップ: 6-1）
-   [ ] PR-6 マージ済み（対象ステップ: 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 6-9 / 6-10）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` はテスト、`static` は `make` のターゲット、`diff` は `_checks/` のプログラム、`manual` は PR のレビューでの読み合わせを指す。

| AC | 確かめ方 | ステップ | 内容 |
|---|---|---|---|
| AC-01〜AC-04a | manual | 2-1 | CLAUDE.md の原則・ディレクトリの言語・例外の記述（PR-2 のレビュー） |
| AC-05 | manual | 2-1 | Go Idioms の例外の記述と範囲（PR-2） |
| AC-06 | manual | 1-4・2-1・5-4・6-2・6-4 | 言語を変える文書は、それを変える PR で読む。変えない分類（`docs/dev/` の直下、`docs/tasks/`、`prompts/README.md`・`testdata/README.md`、指示の文書）は、PR-6 で今の言語のままであることを 1 度確かめる |
| AC-07 | manual | 6-9 | CLAUDE.md のサーバの API と起動の記述（PR-6） |
| AC-08 | diff: `_checks/move/` | 1-6 | 5 ファイルがリンクのパスの違いを除いて同じ |
| AC-09 | diff: `_checks/move/` ＋ manual | 1-1・1-6 | 2 つの手引きの差分の行が許された 3 種類だけで、`.ja.md` がない（diff）。4 ファイルが日本語であること（manual） |
| AC-10 | diff: `_checks/move/` ＋ manual | 1-4・1-6 | 見出しの並び・コードブロック・インラインのコードが同じ（diff）。翻訳の内容（manual） |
| AC-11 | manual | 1-1 | 7 章の読み比べの記録（PR-1） |
| AC-12・AC-13 | diff: `_checks/move/` | 1-6 | `docs/dev/developer_guide/` がなく、旧パスが残っていない |
| AC-14 | diff: `_checks/move/` | 1-6 | 過去のタスク文書の変更がパスだけ |
| AC-15 | diff: `_checks/move/` ＋ test: `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths` | 1-5・1-6 | 置き換えた相対リンクの先がある（diff）。指示と規約の文書のパスがある（test） |
| AC-16 | test: `cmd/fixprguard/docs_test.go::TestGuidePathExists` | 1-3・1-5 | `guidePath` の値のファイルがある |
| AC-17 | test: `cmd/fixprguard/docs_test.go` の手引きを読む各テスト・`internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages` ＋ static: `make test` | 1-3 | 移動後の 1 つのファイルを検査して通る |
| AC-18 | manual | 1-7 | PR-1 の本文の入れ直しの記述 |
| AC-19〜AC-21 | diff: `_checks/split/` ＋ manual | 5-2・5-3・6-2 | 文ごとの行き先と分けた文の一覧（diff の出力）を、設計書 3.6 の表と照らす（manual） |
| AC-22 | diff: `_checks/split/` ＋ manual | 5-3 | 今の README の文がどれかの文書にある（diff）。2 つ以上の文書にある文が許された種類だけであること（diff の重複の一覧を manual で読む） |
| AC-23 | test: `cmd/yt2column/readme_structure_test.go::TestReadmeVersionLinks` | 6-2・6-7 | 両版に `cli.md`・`development.md` へのリンクがある |
| AC-24 | diff: `_checks/split/` ＋ manual | 5-3・5-4 | ほかの版のファイルがなく、翻訳の前後で構造が同じ（diff）。日本語であること（manual） |
| AC-25 | test: `cmd/yt2column/docs_test.go::TestReadmeLinks` | 5-2・5-4・6-2・6-6 | 4 つの文書の相対リンクとアンカー |
| AC-26 | manual | 5-4・6-9 | 参照先の付け替え（PR-4・PR-6） |
| AC-27 | diff: `_checks/readme/` ＋ manual | 6-2・6-4・6-10 | 翻訳の前後で構造とコードブロックが同じ（diff）。翻訳の内容（manual。1.3 の翻訳のレビューを経る） |
| AC-27a | manual | 6-2 | 書き換え（PR-6） |
| AC-28 | test: `cmd/yt2column/readme_structure_test.go::TestReadmeVersionLinks` | 6-2・6-4・6-7 | 両版の先頭に、もう一方の版へのリンクがある |
| AC-29 | test: `cmd/yt2column/readme_structure_test.go::TestReadmeVersionsShareStructure`・`internal/doctestutil` の `ParseStructure` と構造の比較の表のテスト ＋ manual | 6-1・6-7・6-10 | 構造の一致と食い違いの内容、拒否する形（test）。README だけの PR で CI が走ることは、`.github/workflows/ci.yml:187` の `doc-guard` が `./cmd/yt2column` を走らせることと、PR-6 のジョブのログで確かめる（manual） |
| AC-30 | static: `make test` ＋ test: 同上 | 6-7 | テストが `cmd/yt2column` にあり、`make test` で走る |
| AC-31 | test: 4.2 の表で「両版」の各テスト | 6-6 | 版ごとのサブテストで、欠けた版の名前で失敗する |
| AC-31a | test: `extension/test/metadataDocs.test.ts::video metadata documentation` ＋ manual | 6-2・6-4・6-6 | 版ごとのカタログとの一致（test）。各版の引用の言語の説明（manual） |
| AC-32 | manual | 4.2・5-2・5-4・6-6 | 4.2 の表と実装の一致（PR-4・PR-6） |
| AC-33 | test: `extension/test/ciChanges.test.ts::has-extension-changes` | 5-2・6-8 | `README.en.md`・`development.md` で走り、`cli.md` と似たパスで走らない |
| AC-34 | diff: `_checks/readme/` ＋ manual | 6-3・6-4・6-10 | 見出しの訳が用語集にあり、`README.en.md` の見出しと一致する（diff）。(2)・(3) の語と、`README.en.md` での使い方（manual） |
| AC-35 | test: `extension/test/metadataDocs.test.ts`（英語のカタログの参照先） ＋ diff: `_checks/readme/` | 6-8・6-10 | 英語のメッセージが `README.en.md` を示す（test）。`ja.ts` が変わらない（diff） |
| AC-35a | test: `cmd/yt2column-server/run_test.go::TestStartupFailures` | 6-8 | 診断が `README.en.md` を示し、ファイルがある |
| AC-36 | test: `cmd/yt2column/run_test.go::TestRunHelp`・`cmd/yt2column/readme_structure_test.go::TestReadmeVersionLinks` | 5-2・6-7・6-8 | ヘルプの 2 か所が `cli.md` を示し、ファイルがある。`cli.md` に README の設定の節へのリンクがある |
| AC-37 | test: `cmd/yt2column/fixpr_docs_test.go::TestReadmeSyncStepPlacement` ＋ manual | 3-2〜3-5 | 同期の手順の位置とコミットの指示の位置（test）。手順の内容（manual、PR-2） |
| AC-38・AC-39 | manual | 3-1 | `/mktrans` の振る舞い（PR-2）。ステップ 6-4 で全文の翻訳を実際に使う |
| AC-40 | manual | 2-1・2-2 | CLAUDE.md と `_context.md` の記述（PR-2） |
| AC-41 | manual | 6-9 | Development Notes の記述（PR-6） |

**読み比べの記録（AC-11）:** 2026-10-29 に、移動前の `mergepr_guide.md` と `mergepr_guide.ja.md`、`fixprguard_guide.md` と `fixprguard_guide.ja.md` を節ごとに読み比べた。

- 節の対応: どちらの組も見出しが 1 対 1 で対応する（`mergepr_guide` は 18、`fixprguard_guide` は 28）。節ごとの段落、箇条書きの項目、表の行、コードブロックの数も一致した。
- 英語版にだけあった記述: ない。そのため、日本語版に加えた記述はない。
- `mergepr_guide` の 1 行の差: 英語版は `prepare [PR]` の手順 4 の続きの 3 文を別々の段落にしているが、日本語版は字下げした続きの行にしている。内容は同じで、`_checks/move` の差分の一覧にも現れない。
- 残した記述: `fixprguard_guide.md` の最後の箇条書きは、ガードテストが「本書（日英）」を確かめると述べている。英語版の手引きを消した後は古い記述になるが、AC-09 はこの PR で日本語の手引きの文言を変えることを許していないため、そのままにした。この行は `.ja.md` の版と同一なので、`_checks/move` の差分の一覧には現れない。PR の本文でレビュアーに判断を求める。

**差分の検査の実行の記録:** PR-1 で 2026-10-29 に、リポジトリのルートで `go run ./docs/tasks/0048_doc_language_policy/_checks/move/` を実行した。merge-base は `234da9bd3c73a9520f234c8c2784ebe632477034`、実行した時点の HEAD は `7763e4d1d8a7ded13c9c84198bdc7d6293718697`、終了コードは 0 である。

```text
merge-base 234da9bd3c73a9520f234c8c2784ebe632477034
head       7763e4d1d8a7ded13c9c84198bdc7d6293718697
OK   conventions unchanged
OK   guides renamed from the Japanese originals
OK   old directory is gone
OK   no old file path remains
OK   past task documents changed only at paths
OK   relative links resolve
OK   translations keep the structure
```

各検査が違反を見つけることは、PR の先頭とは別の一時的なコミットで 1 か所ずつ変えて確かめ、元に戻した。どれも終了コード 1 になった。

| 変えた 1 か所 | 失敗した検査 |
|---|---|
| `docs/dev/conventions/requirements_process.md` に 1 行を加える | AC-08 |
| `docs/dev/guides/` に `extra.ja.md` を置く | AC-09 |
| `docs/dev/developer_guide/` に `stub.md` を置く | AC-12 |
| `CLAUDE.md` に旧パス `docs/dev/developer_guide/package_reference.md` を残す | AC-13 |
| 過去のタスク文書の Status の行を `draft` に変える | AC-14 |
| 過去のタスク文書の相対リンクの深さを 1 段変える | AC-15（AC-14 も失敗する） |
| `pricing_update.md` のインラインのコード `` `0.0094` `` を `` `0.0095` `` に変える | AC-10 |
| `pricing_update.md` だけにコードブロックを加える | AC-10 |

翻訳のレビューでは、Critical 1 件（出力の上限の条件 (b) が比べる時間を示していない）と Major 3 件、Minor 多数の指摘を受け、すべて直した。直した後の再確認で、残る指摘はなかった。

**CI の記録（AC-29）:** （ステップ 6-10 で記録する）

## 8. 横断の確認 (Cross-Search Checklist)

`make lint`・`make test` では見つからない残りを確かめる。

-   [ ] 1.5 の旧い補助関数の名前（小文字の `readDoc` など）が、`cmd/yt2column`・`cmd/yt2column-server` のコメントに残っていない（PR-3）。
-   [ ] `docs/translation_glossary.md` の訳語と、`cli.md`・`development.md`・`README.md`・`README.en.md`・`pricing_update.md`・`claude_output_limit.md` の用語が一致する（PR-1・PR-4・PR-6 のレビューで読む）。
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
