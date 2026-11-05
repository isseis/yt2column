# 実装計画書：単純な変更のための軽量プロセスと、/mkreq での振り分け

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-04 |
| Review date | 2026-11-04 |
| Reviewer | isseis |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番を、「設計書 3.5.6」などは設計書の節を指す。用語（軽量プロセス、通常のプロセス、類型、仕様のコメント、条件の表、確かめ直しのスクリプト、仕様の手順書、戻しのコメント、道具のコミット、確かめた結果の記録、Part B など）は、要件書 6 章と設計書の冒頭に従う。本タスクに `implementation_handoff.md` はない。[design_handoff.md](design_handoff.md) の H-01〜H-11 は、設計書の付録 A がすべて記録している。

設計書は、本書を書く途中で決定の変更があり、`draft` に戻っている（1.3 節）。本書と設計書は、同じ PR の `/runpr` で承認を受ける。

既存のファイルについての記述は、コミット `68d328ac`（2026-11-04 の `main`）で確かめた。設計書が行番号を引いたコミット `74b9b49` から `68d328ac` までに、本タスクが変えるファイルのうち変わったのは `CLAUDE.md` だけである（`git diff --stat 74b9b49 68d328ac -- .claude CLAUDE.md AGENTS.md docs/dev cmd/yt2column internal/doctestutil .github Makefile` の出力）。その変更で、`CLAUDE.md` の 282 行より後の行は 1 行ずつ前に移った。本書の行番号は、すべて `68d328ac` のものである。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

単純な変更を、タスク文書を作らずに 1 本の PR で進める軽量プロセスを作る。作るものは、条件の定義、`/mkreq` の振り分け、仕様の手順書、`/runissue`、確かめ直しのスクリプト、`/runpr` の変更、周辺の文書とラベルである。何を作るかは設計書 1〜6 章で決まっている。本書は、それをどの順序で、どの PR に分け、どう確かめるかを定める。

### 1.2. 実装の原則

-   フェーズは設計書 8 章の 4 つの段階（1〜4）に合わせる。フェーズ 1 だけを 2 本の PR に分け、全体を 5 本の PR にする（1.3 節）。
-   文書とコマンドの定義に書く内容は設計書の節を参照し、本書では繰り返さない。文の最終的な文言、テストの表の行、スクリプトの出力の細部は、書くときに決める。
-   条件の表の `exclude` の行は、そのパスを作る PR で足す（設計書 8 章）。除外するパスの実在のテスト（ステップ 1-4）が、まだないパスで失敗しないためである。
-   加えたテストは、対象を壊すと失敗することを実装のときに確かめ、コミットメッセージにそう書く（CLAUDE.md「Testing Strategy」、設計書 7.3）。
-   Go と shell のソースのコメント・識別子・文字列リテラル（スクリプトの出力の行を含む）は英語で書く。例外は 2 つだけである。1 つは guard-test literal exception、もう 1 つは確かめ直しのスクリプトとそのテストのリテラルの例外（設計書 3.1.5。ステップ 1-5 で CLAUDE.md に足す）で、範囲は仕様の見出し、fixture の仕様の本文、日本語のパスの fixture のファイル名である。日本語のリテラルは `lightweight_docs_test.go` とスクリプトだけに置き、`internal/doctestutil` には置かない。
-   文書の言語は CLAUDE.md の「Document language」に従う。`.claude/commands/` と `docs/dev/conventions/` は英語で書く。仕様のコメントの雛形・戻しのコメント・issue テンプレート・PR の本文は日本語で書く。
-   各 PR の終わりに、グリーンゲート（`make test && make lint`）を通す。Go のファイルを変える PR では、`make fmt` と `make deadcode` も通す。本タスクは拡張・README・`docs/dev/guides/development.md`・`.gitignore` を変えないので、`make ext-test` は要らない（CLAUDE.md「Development Notes」）。
-   GitHub への書き込み（ラベルの作成）は、そのときに利用者の承認を得る（CLAUDE.md「Tool Execution Safety」）。

### 1.3. 計画の際に決めたこと（2026-11-04）

レビュアーと決めたこと:

-   **PR は 5 本にする。設計書 8 章の段階 1 を、「規則と値」（PR-1）と「確かめ直しのスクリプトとテスト」（PR-2）に分ける:** 確かめ直しのスクリプトは信頼境界そのもので、壊して確かめる項目も多い（設計書 7.3）。規則の文と混ぜるとレビューが散るので、単独でレビューする。段階 2〜4 は、それぞれ 1 本の PR にする。
-   **条件の表の `tool` の行は、最初は `gofumpt -w` と `git mv` の 2 行にする:** 設計書 3.1.3 は、この行を「...」のまま残している。`golangci-lint` はローカルでは `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run --build-tags test,integration` で動く（`Makefile:9-10`）。`go run` はモジュールを取りに行くので、オフラインでは再現の確認が `STOP` になる。道具の版が違う場合も `STOP` になる（安全な側に倒れる）が、これは版を固定しない `gofumpt`（`Makefile:11`）も同じである。したがって、`golangci-lint` を表に入れない理由は、版ではなくネットワークである。`golangci-lint run --fix` の変更は、手で書いた行に数える（類型 M の上限の中なら軽量プロセスで進められる）。後から行を足すのは、通常のプロセスの PR で行う。
-   **確かめ直しのスクリプトとそのテストに、日本語のリテラルを許す例外を CLAUDE.md の「Go Idioms」に足す:** スクリプトは、仕様の本文の `### 類型` の下の 1 文字を読む（設計書 3.5.6）。そのため、ソースに日本語の見出しが要る。CLAUDE.md の「Language」の例外には、shell のスクリプトが入っていない。本書の作成のときにこの食い違いを見つけ、利用者と決めて設計書を直した（設計書 1.2・3.1.5）。

本書のレビューを受けて、設計書に記した上の決定の変更へ、次の点を加えた（設計書の Comments と変更履歴にも記した）。

-   スクリプトがグローバルとシステムの git の設定を読まないこと。あわせて、数える diff だけでなく、コミットの読み取り・作業ツリーの確認・一時的な worktree の作成でも設定を無効にすること（設計書 3.1.2）。
-   リテラルの例外の範囲に、日本語のパスの fixture のファイル名を足したこと（設計書 3.1.5）。
-   文書のテストを 3 つ足したこと（設計書 3.9）：確かめ直しのスクリプトの呼び出し方、`runissue.md` の読み取り、雛形の目印。
-   一覧のフィルタのテストを、フィルタを入れる段階 1 に移したこと（設計書 8 章）。信頼を決めるフィルタを、テストなしに `main` に入れないためである。

上の決定と設計書から導いたこと（PR のレビューで異論があれば改める）:

-   **一覧のフィルタのテストと除外するパスの実在のテストは、条件の表と仕様の手順書と同じ PR-1 に入れる:** 表と手順書の誤りを、それらが入る PR で見つけるためである。テストのファイル `cmd/yt2column/lightweight_docs_test.go` も PR-1 で作り、`exclude` の行も PR-1 で足す。
-   **まだないコマンドの名前は、文書に書いてよい。ただし、Markdown のリンクは張らない:** `requirements_process.md` の軽量プロセスの節（PR-1）は、`/runissue`（PR-3）と `/mkreq` の振り分け（PR-5）を名前で挙げる。PR-5 までは、ラベルも振り分けもないので、軽量プロセスに入る issue は生まれない（3.1 節）。まだないファイルへのリンクは、`TestInstructionDocPaths` で失敗する。
-   **`_context.md` の「Reviewer ping」の行への `runissue.md` の追加は、PR-3 で行う:** この行は、各コマンドが PR を作った後に行うことを述べる。そのため、その振る舞いを持つ `runissue.md` と同じ PR に置く。
-   **`mkreq.md` の投稿の区画のテストは、フェーズ 4 に置く:** 設計書 8 章は `postingSections` の追加を段階 2 に挙げるが、`mkreq.md` の区画はステップ 4-3 で初めてできる。段階 2 で足すのは、`runissue.md` の区画だけである。
-   **`mkreq.md` の振り分けは、手順 1 の最後の項目として書き、番号を付け直さない:** 設計書 3.4.1 の「手順 1 の後、手順 2 の前」に当たる。番号を付け直すと、`mkreq.md` の中の手順の参照と、設計書の「手順 9」が変わってしまう。
-   **条件の表の上限の値（AC-02・AC-03）は、テストに写して固定しない:** CLAUDE.md の「Document tests ... never prose」は、テストに写したリテラルで文書の値を固定することを禁じる。スクリプトのテストは、上限を実際の条件の表から読み、上限ちょうどの値と 1 つ超えた値で確かめる。表の値が要件書の値と同じであることは、PR-1 のレビューで読む（7 章の AC-02・AC-03）。
-   **確かめ直しのスクリプトは bash 3.2 でも動く書き方にし、テストは `/bin/bash` が 3.x の環境ではそれでも走らせる:** macOS の `/bin/bash` は 3.2.57 である（`/bin/bash --version`、2026-11-04）。`/runissue` は `PATH` の `bash` で実行するので、利用者の環境によっては 3.2 で動く。CI は ubuntu-latest の bash 5 である。
-   **スクリプトのテストは、本番と同じく `origin/main` から書き出したスクリプトを、リポジトリの外のファイルとして実行する:** fixture の `origin/main` に、実際のスクリプトと `requirements_process.md` を置く（設計書 3.9）。スクリプトは除外するパスの実在を確かめない。実在を確かめるのは `TestLightweightExcludedPaths` だけなので、fixture に除外するパスをそろえる必要はない。
-   **テストは、git の環境を許可リストで作る:** 既存の `cmd/yt2column/makefile_test.go:637-643` と `internal/coveragecompare/test_helpers.go:498-516` に倣う。`PATH`・`HOME`・`TMPDIR` だけを渡し、`GIT_CONFIG_NOSYSTEM=1`・`GIT_TERMINAL_PROMPT=0`・`LC_ALL=C` を足す。`HOME` と `TMPDIR` は場合ごとの空の `t.TempDir()` にし、利用者の `~/.gitconfig` を読ませない。利用者の git の設定の場合（ステップ 1-6）に置く敵対的なグローバルの設定も、その `HOME` の中に置く。pre-commit の `go-test` の hook（`.pre-commit-config.yaml:4-6`）が `git commit` の中で `go test` を実行するので、引き継いだ `GIT_*` を渡さないことが特に必要である（前例は `cmd/fixprguard/main_test.go:266`）。
-   **一覧のフィルタのテストと雛形の目印のテストは、目印の値を `_context.md` の行から読む:** テストに目印の文字列を写さず、ステップ 1-2 で足す行のコードスパンを使う（既存の `contextRow`、`cmd/yt2column/fixpr_docs_test.go:77`）。
-   **`lightweight` ラベルの有無は、件数の上限に左右されない形で確かめる:** `gh label list` の既定の件数の上限は 30 である（`gh label list --help`）。`/mkreq`（設計書 3.4.4 の 1）とステップ 4-5 では、件数の上限を外すか、名前で探して完全に一致するものを見る。
-   **AC-10・AC-33・AC-36 は `diff` で確かめない:** これらは「今のまま」を述べるが、対象はコマンドの定義の文（`mkreq.md`・`runpr.md` の手順）である。手順 5 の項目は書き直して広げるので、文の前後の比較では振る舞いが保たれたかを示せない。PR のレビューで差分を読む（`manual`）。

### 1.4. 既存コード調査結果

**メンションの確認の guard（`cmd/yt2column/codex_review_docs_test.go`）**

-   `postingSections`（`:60-79`）は、投稿する区画の名前・ファイル・区画の探し方・投稿のコマンドの並びである。区画の探し方は 2 つある。見出しで探す `headingSection`（`:33`）と、項目の書き出しから次の書き出しまでを取る `itemSection`（`:42`）である。
-   `TestPostingCommandsInventory`（`:200`）は、`doctestutil.CommandDocs`（`internal/doctestutil/protocol.go:85`。`.claude/commands` の下の `.md` だけを返す）のすべての投稿のコマンドを確かめる。`-F`・`-f` を渡す `gh api graphql` も、投稿に数える（`textFlag`、`:139`）。例外の `readOnlyInvocations`（`:148-150`）は前方一致で照らし、今は `gh api graphql -F owner='{owner}' -F repo='{repo}' -F n=<issue> -f query='` の 1 つだけである。`isReadOnly`（`:172`）は、続くコードブロックに `mutation` がないことも求める。
-   `gh issue edit --add-label`・`--remove-label`、`gh issue develop`、`git push` は、`postingCommand`（`:136`）に当たらないか、本文のフラグを持たない。そのため、投稿に数えない（設計書 2.2）。
-   `.claude/commands/_lib/lightweight-check.sh` は `.md` でない。そのため、`CommandDocs` を使うテストと `TestInstructionDocPaths`（`cmd/yt2column/docpaths_test.go:140`。`instructionDocs`、`:106` も `.md` だけを読む）の対象にならない。

**一覧のフィルタと読み取りの guard（`cmd/yt2column/fixpr_docs_test.go`）**

-   `checkListings`（`:431`）は、区画の中の `gh api --paginate` の一覧を、正規表現 `paginatedListing`（`:214`）で取り出し、fake の応答に jq で実行する。フィルタは単引用符を含められない。置き換えの印は `<awaited-reviewer>`・`<user-login>`・`<review-request>`・`<review-mention>` の 4 つ（`:443-448`）である。置き換えられずに残った印（`leftPlaceholder`、`:216`）は、失敗にする。
-   fake の応答（`listingFixture.response`、`:404`）は、`user`・`body`・`html_url` と、`timeFieldSeconds`（`:397`）にある時刻の項目だけを持ち、`id` を持たない。`checkListingOutput`（`:622`）は、出力の値と応答の項目を `!=` で比べる。JSON から読んだ数は `float64` になるので、`id` を足すときは、応答の側も同じ型で持つ。
-   `checkGhAPICalls`（`:488`）は、区画の中のすべての `gh api ` の呼び出しが、許す一覧の前方一致に当たることを求める。一覧の呼び出しも対象なので、既存の一覧（`step3ReadCalls`・`handoffReadCalls`）は `"gh api --paginate "` を含む。
-   `TestContextAndCommandsAgree`（`:975`）は、`fixpr.md`・`runpr.md`・`_lib/task-pr-handoff.md` がレビュアーの名前を書かないことを確かめる。
-   `requireSection`（`:92`）、`flat`（`:63`）、`contextRow`（`:77`）は、そのまま使える。

**`internal/doctestutil`**

-   コードブロックの外の見出しで節を取る `SectionOutsideBlocks`（`markdown.go:44`）と、コードブロックを順に返す `FencedBlocks`（`markdown.go:165`。言語名の `Info` を持つ）がある。条件の表・雛形・一覧の節は、これらを組み合わせて取り出し、新しい取り出しの関数を書かない。`SectionByHeading`（`markdown.go:90`）はコードブロックを飛ばさないので使わない。
-   コマンドの定義の見出しの定数は、今は `cmd/yt2column/fixpr_docs_test.go:29-41` にある。設計書 2.2・3.9 は新しい見出しの定数を `internal/doctestutil` に置くとしているので、新しいファイル `internal/doctestutil/lightweight.go`（`//go:build test`）に置く。日本語の手引きの見出しを置く `guides.go` とは分ける。

**git を使うテストの前例**

-   `internal/coveragecompare` のテストは、`t.TempDir()` に fixture のリポジトリを作り、許可リストの環境で git を走らせる（`test_helpers.go:498-516`）。`TestDiffIgnoresUserGitConfig`（`run_test.go:712`）は、利用者の git の設定で結果が変わらないことを確かめる。どちらもパッケージの中の非公開の関数で、`cmd/yt2column` からは使えない。確かめ直しのスクリプトのテストの fixture は、`lightweight_docs_test.go` の中に置く。ほかのパッケージから使わないので、`testutil/` は作らない。
-   `_test.go` には gosec が効かない（`.golangci.yml:193-199`）ので、テストの `exec.Command` に `//nolint` は要らない。

**外部の前提の確かめ（2026-11-04、この機械で実行）**

-   `git diff --no-ext-diff --no-textconv --no-color --diff-algorithm=myers --no-renames --numstat -z` は、git 2.50.1 で受け付けられ、NUL で区切った行を出す（`68d328ac` で `HEAD~1 HEAD` に実行した）。
-   `git diff --raw -z` は、追加したシンボリックリンクを `120000`、サブモジュールを `160000`、モードの変更を `:100644 100755 ... M` で出す。`numstat` はこれらを区別しない。`-c diff.ignoreSubmodules=all` を付けると、`numstat` はサブモジュールの行を出さない。その設定があっても、`--ignore-submodules=none` を付ければ出す。スクラッチパッドのリポジトリで確かめた。
-   `git push --force-with-lease=refs/heads/<branch>:<SHA> origin :refs/heads/<branch>` は、`origin` のブランチが `<SHA>` のときだけ消す。ほかの値なら `(stale info)` で拒み、終了コード 1 を返す。スクラッチパッドの bare リポジトリで、古い SHA と正しい SHA の両方を試した（設計書 3.5.7）。
-   GraphQL の `Issue.closedByPullRequestsReferences(first: 20, includeClosedPrs: true)` は、本文に `Closes` を書かずにブランチで issue にリンクした PR も返す。#619 で、#660・#664（どちらも `MERGED`）と `hasNextPage: false` が返った（設計書 3.5.1 の入口の確認）。
-   SHA-256 のコマンドは、Linux の `sha256sum`、macOS 26 の `/sbin/sha256sum`、どちらにもある `shasum -a 256` が使える（`which`）。
-   CI の `doc-guard` ジョブは、`go test -tags test ./cmd/yt2column ...` をすべての PR で実行する（`.github/workflows/ci.yml:175-187`、ubuntu-latest）。新しいテストは bash・git・jq を使う。
-   リポジトリに `lightweight` ラベルはない（`gh label list`、2026-11-04。ラベルは 21 個）。

**本タスクが変える箇所（`68d328ac` の行番号）**

-   `docs/dev/conventions/requirements_process.md`：「LLM Constraints」（7〜14 行）、「Review Flow」（63〜78 行）。新しい小節「### Lightweight Process」は「## 0. Review Workflow」の中、「## 1.」（388 行）の前に置く（設計書 3.1.1）。
-   `.claude/commands/_context.md`：「Process convention」の表（35〜54 行）、「Reviewer ping」の行（50 行）、「PR title and squash subject」の表（75〜82 行）。
-   `.claude/commands/_lib/task-pr-handoff.md`：呼び出し元を挙げる冒頭の段落（7〜13 行）、入力（TASK_DIR 36 行、PR_TEXT 44 行、ISSUE 48 行）、「Authorization」（57〜59 行）、Part B の手順 3 の `Refs #<issue>` の項目（243〜246 行）。
-   `.claude/commands/_lib/local-cli-review.md`：呼び出し元の一覧（「Five callers run it」、11〜20 行）。
-   `.claude/commands/mktrans.md`：README の同期を手順として行う呼び出し元の一覧（33〜35 行）。
-   `.claude/commands/runpr.md`：「Authorization」（65〜90 行）、手順 1 の `MERGED`（109〜117 行）と単独の変更の PR の作成（117〜171 行）、手順 2 の「Kind」（262〜271 行）、手順 4 の 1（645〜651 行）、手順 5 の issue を閉じる項目（809〜827 行）。
-   `.claude/commands/mkreq.md`：front matter の `description`（2 行）、目的の段落（12〜17 行）、「Authorization」（26〜30 行）、手順 1（34〜49 行）、手順 9（151 行）。
-   `CLAUDE.md`：33 行（Each task lives in）、99〜100 行（`/mktrans` を実行するコマンド）、154 行（`/mkreq`）、166 行（Each task PR is handled by two commands）、591 行（jq の項目）、621 行（「## Go Idioms」。guard-test literal exception は 645 行）、668 行（「## Requirements and Acceptance Criteria」）。
-   `AGENTS.md`：11 行（実装のコミットについての指摘の規則）。
-   `.github/ISSUE_TEMPLATE/`：今は `accessibility.md` だけがある。テンプレートを読むテストはない（`grep -rn ISSUE_TEMPLATE` を `cmd`・`internal`・`extension/test` で実行した結果が 0 件）。

**除外するパスの実在（設計書 3.1.3 の表）:** 本タスクが作る 4 つのパス（`.claude/commands/runissue.md`・`.claude/commands/_lib/lightweight-spec.md`・`.claude/commands/_lib/lightweight-check.sh`・`cmd/yt2column/lightweight_docs_test.go`）を除き、表のパスはすべて `68d328ac` にある。`exclude-absent` の `go.sum`・`go.work`・`.gitmodules` はない（`git cat-file -e` で 1 つずつ確かめた）。

**型による保証:** 本タスクは、製品のコードに Go の型を加えない（設計書 3 章の冒頭）。具体的な型で振る舞いを分ける設計はないので、型の作り方を列挙して確かめる必要もない。

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: 条件と値（設計書 8 章の 1）

#### ステップ 1-1: 軽量プロセスの節と条件の表

**変更するファイル:** `docs/dev/conventions/requirements_process.md`

-   [x] 「## 0. Review Workflow」の中に小節「### Lightweight Process」を足し、設計書 3.1.1 の項目を英語で書く。類型・上限・除外の文言は、要件書の AC-01〜AC-05 をそのまま英語にする。数え方は設計書 3.1.2、機械的に確かめる条件とレビューで確かめる条件の区別は設計書 3.1.4 のとおりにする。
-   [x] 見出し「#### Mechanically checked conditions」の直後に、条件の表を置く（設計書 3.1.3 の書式）。表の行は次のとおりにする。
    -   `limit` の行：AC-02・AC-03 の値
    -   `tool` の行：`gofumpt -w` と `git mv`（1.3 節）
    -   `exclude`・`exclude-absent` の行：設計書 3.1.3 の表のパスのうち、この PR の時点で実在するものと、この PR で作る `lightweight-spec.md`・`lightweight_docs_test.go`。`lightweight-check.sh` は PR-2、`runissue.md` は PR-3 で足す。
-   [x] 同じ小節に、「信頼境界に当たるパッケージやファイルを新しく足す PR は、同じ PR で条件の表に `exclude` を足す」という規則を書く（設計書 3.1.3）。
-   [x] 「LLM Constraints」に、軽量プロセスの例外と範囲を足す（AC-38、設計書 3.1.5）。「Review Flow」に、軽量プロセスの流れを足す（AC-07、設計書 2.1）。
-   [x] まだないファイル（`runissue.md` など）は、Markdown のリンクを張らず、コードスパンで名前を書く（1.3 節）。ステップ 1-2・1-3 も同じにする。

#### ステップ 1-2: `_context.md` の値

**変更するファイル:** `.claude/commands/_context.md`

-   [x] 「Process convention」の表に、設計書 3.2 の 4 行（Lightweight label・Lightweight spec marker・Lightweight revert marker・Lightweight branch name）を足す。目印の値は、ステップ 1-4 のテストが読めるように、セルの中の 1 つのコードスパンに書く。
-   [x] 「PR title and squash subject」の表に、Lightweight PR title と Lightweight commit subject の 2 行を足す（設計書 3.2）。squash の件名の正規表現は変えない。

#### ステップ 1-3: 仕様の手順書

**変更するファイル:** `.claude/commands/_lib/lightweight-spec.md`（新規。英語）

-   [x] 設計書 3.3 のとおりに書く。仕様のコメントの書式（日本語の雛形を 1 つのコードブロックで置く）、形の確かめ方（AC-19）、自分のコメントの一覧（設計書 3.3.2）、仕様とみなすコメントの選び方と戻した issue の判定（AC-17・AC-32）である。値は `_context.md` の名前で参照する。
-   [x] 見出し「## Listing own marker comments」の下に、設計書 3.3.2 の一覧のコマンドを 1 つだけ書く。形は、`checkListings` が読める形にする（1.4 節）。置き換えの印は `<user-login>`・`<spec-marker>`・`<revert-marker>` を使い、フィルタに単引用符を入れない。この節のコードブロックには、`#` で始まる行を置かない。

#### ステップ 1-4: 条件の表・一覧のフィルタ・雛形の目印のテスト

**変更するファイル:** `internal/doctestutil/lightweight.go`（新規。`//go:build test`）、`cmd/yt2column/lightweight_docs_test.go`（新規。`//go:build test`）、`cmd/yt2column/fixpr_docs_test.go`、`CLAUDE.md`（591 行の jq の項目）

-   [x] `internal/doctestutil/lightweight.go` に、見出し「#### Mechanically checked conditions」「## Spec comment format」「## Listing own marker comments」の定数を置く（1.4 節）。「## Spec comment format」は、`TestLightweightSpecTemplateMarker` が雛形を探す見出しである。
-   [x] `lightweight_docs_test.go` に、条件の表を読む関数を置く。`SectionOutsideBlocks` と `FencedBlocks` を組み合わせて使う。見出しやコードブロックがないとき、2 つ以上あるとき、知らない行の種類があるとき、`exclude` の行が 1 つもないときは、失敗にする（ステップ 1-6 もこの関数を使う）。
-   [x] `TestLightweightExcludedPaths` を書く。`exclude` の行のパスがリポジトリに実在し、`exclude-absent` の行のパスが実在しないことを確かめる（設計書 3.9）。`/` で終わるものはディレクトリ、それ以外はファイルとし、大文字と小文字を区別して照らす。macOS の APFS は大文字と小文字を区別しないので、`os.Stat` だけでは照らさない。
-   [x] `checkListings` の置き換えの表に `<spec-marker>`・`<revert-marker>` を足し、fake の応答の項目に `id`・`created_at`・`updated_at` を出せるようにする（設計書 3.9）。時刻の項目は、既存の `timeFieldSeconds` のしくみで出せるので、足すのは `id` である（1.4 節）。既存の `TestRunprDocContract`・`TestHandoffListingPrintsNoBody` が通り続けることを確かめる。
-   [x] `TestLightweightSpecListing` を書き、仕様の手順書の一覧の節を `checkListings` で確かめる。許す呼び出しの一覧には、`"gh api --paginate "`・`gh api user --jq .login`・1 件の本文の読み取りを置く（1.4 節）。目印の値は `_context.md` の行から読む（1.3 節）。fake の項目は、設計書 3.9 の観点（書き手、書き手の種類、目印、CRLF、本文を出さないこと）を分けて確かめられるものにする。目印の後ろに文字が続く 1 行目と、先頭に空白がある 1 行目も含める。
-   [x] `TestLightweightSpecTemplateMarker` を書く。仕様の手順書の雛形のコードブロックの 1 行目が、`_context.md` の仕様の目印の値と同じであることを確かめる（設計書 3.9「雛形の目印」）。
-   [x] `CLAUDE.md` の 591 行の jq の項目に `TestLightweightSpecListing` を足す。bash と git も要ることは、それらを使う `TestLightweightCheck` を足すステップ 1-6 で書く（1.3 節）。
-   [x] 新しいテストは、bash・git・jq がないときに飛ばさずに失敗にする（既存の `checkListings` と同じ）。
-   [x] 壊して確かめる。次の壊し方のそれぞれで、名前を挙げたテストが失敗することを確かめ、コミットメッセージに書く。
    -   `exclude` の 1 行のパスを、実在しない名前・大文字と小文字だけが違う名前に変える。`/` を付け外しする。実在するパスを `exclude-absent` にする（`TestLightweightExcludedPaths`）。
    -   フィルタから、書き手の条件を外す。`.user.type` の条件を外す。目印を前方一致にする。本文を出す（`TestLightweightSpecListing`）。
    -   雛形の 1 行目を変える（`TestLightweightSpecTemplateMarker`）。

**完了の条件（ステップ 1-1〜1-4）:** `make fmt`・`make test`・`make lint`・`make deadcode` が通る。

### PR-1 作成ポイント: lightweight process rules, values, and listing filter

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4

**推奨タイトル**: `docs(0061): 軽量プロセスの条件と条件の表、仕様の手順書、_context.md の値とそのテストを加える`

**レビュー観点**: 類型・上限・除外の文言が要件書 AC-01〜AC-05 と同じ意味で、条件の表の上限の値が AC-02・AC-03 と一致すること / 条件の表の除外するパスが設計書 3.1.3 の表と一致し、まだ作らないパスが入っていないこと / 一覧のフィルタが書き手で絞り、本文を出さないこと、そのテストの fake の項目が各条件を分けて確かめていること / `checkListings` に `id` と置き換えの印を足しても、既存の 2 つの一覧のテストの意味が変わらないこと

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-3 の一覧のフィルタは、どのコメントを仕様として信頼するかを決める（mkplan.md の panel mode の security-gate の条件）。ステップ 1-1 の条件の表も、信頼境界の除外を定める

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/675
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 1-5: 確かめ直しのスクリプト

**変更するファイル:** `.claude/commands/_lib/lightweight-check.sh`（新規）、`docs/dev/conventions/requirements_process.md`（条件の表の `exclude` の 1 行）、`CLAUDE.md`（「Go Idioms」）

-   [x] 設計書 3.5.6 のとおりに書く。引数は、仕様の本文のファイルのパスだけにする。BASE（`git merge-base origin/main HEAD`）・類型・道具のコミットの範囲・条件の表は、スクリプトが自分で決める。条件の表は `git show origin/main:docs/dev/conventions/requirements_process.md` から読み、書式の規則（設計書 3.1.3）に合わなければ `STOP` にする。
-   [x] 数え方は設計書 3.1.2 に従う。行は `--numstat -z`、シンボリックリンク・サブモジュール・モードの変更は `--raw -z` で読む（1.4 節）。
-   [x] git の設定の無効化（設計書 3.1.2）を、すべての git の呼び出しに当てはめる。
    -   グローバルとシステムの設定は読まない。
    -   リポジトリの設定が効く項目は、明示したオプションで無効にする。少なくとも、diff のアルゴリズム、外部の diff、textconv、色、`--ignore-submodules=none`、`diff.relative`、ログの署名の表示を無効にする。
    -   作業ツリーの確認は、未追跡のファイルをすべて数える形で行う。
    -   一時的な worktree の作成とその中の道具の実行では、hook を動かさない。
    -   リポジトリのルートで git を呼ぶ。
-   [x] 道具のコミットの再現は、親のコミットの一時的な worktree で行う。trailer のコマンドは shell を通さずに実行し、実行後の作業ツリーと index から得た tree の ID を、コミットの tree の ID と比べる。一時的なファイルと worktree は `TMPDIR` の下に作り、終わるときに必ず消す（`trap`）。
-   [x] 出力と終了コードは、設計書 3.5.6 の表のとおりにする（`PASS` は 0、`FAIL` は 1、`STOP` は 2）。`FAIL` のときは、その前の行に、理由ごとの実際の値を出す。
-   [x] bash 3.2 でも動く書き方にし、ネットワークを使わない（1.3 節）。SHA-256 は `sha256sum` を使い、ないときは `shasum -a 256` を使う。
-   [x] 条件の表に `exclude .claude/commands/_lib/lightweight-check.sh` を足す。
-   [x] `CLAUDE.md` の「Go Idioms」に、リテラルの例外を足す（設計書 3.1.5・3.8）。書き方は既存の例外と同じ形（範囲・理由・境界）にする。

#### ステップ 1-6: 確かめ直しのスクリプトのテスト

**変更するファイル:** `cmd/yt2column/lightweight_docs_test.go`、`CLAUDE.md`（jq の項目）

-   [x] `CLAUDE.md` の jq の項目に、`make test` が bash と git も要ることを書く（1.3 節）。
-   [x] `TestLightweightCheck` を書く。fixture は、`t.TempDir()` に作るリポジトリである。`origin` の URL は存在しないローカルのパスにし、`refs/remotes/origin/main` に実際の `requirements_process.md` とスクリプトを置く。道具のコミットの場合だけは、`tool` の行を、テストの中で用意した、出力が決まっているコマンドにした条件の表を置く。
-   [x] スクリプトは、本番と同じく、`origin/main` から書き出したファイルとして実行する（1.3 節）。仕様の本文のファイルは、fixture のリポジトリの外に置く。仕様の本文は、仕様の手順書の雛形のコードブロックから作り、類型の行は位置で埋める。
-   [x] git とスクリプトは、許可リストの環境で走らせる（1.3 節）。`TMPDIR` は場合ごとの `t.TempDir()` にする。各場合のコマンドには期限を付ける。
-   [x] 設計書 7.1 の表のすべての場合を確かめる。最後の行と終了コードを確かめ、`FAIL` の場合は超えた値の行も確かめる。上限の場合は、上限を実際の条件の表から読み、上限ちょうどの値（`PASS`）と 1 つ超えた値（`FAIL`）の両方を置く（1.3 節）。
-   [x] 設計書 7.1 の表にない場合も確かめる。どれも設計書 3.1.2・3.3.1・3.5.2・3.5.6 と AC-03・AC-21・AC-29 が定めることである。
    -   BASE：分かれた後に `origin/main` が進む（BASE が分かれた点であること）。
    -   数え方：名前の変更を、削除と追加の行、および 2 つのファイルとして数える。空白だけの変更を数える。類型 M で、道具が生成した変更だけのファイルをファイル数に数えない。
    -   `docs/tasks/` の下のファイルを足す（`FAIL excluded`）。
    -   1 つのコミットに trailer が 2 行ある（`STOP`）。
    -   類型の読み取り：CRLF の仕様、類型の下に 2 行がある、小文字、見出しが 2 つある、見出しがコードブロックの中にある。
    -   `sha256sum` のない `PATH` でも動く。
-   [x] 利用者の git の設定の場合を確かめる。fixture のリポジトリの設定とグローバルの設定に、`diff.algorithm`・textconv・`diff.ignoreSubmodules`・`diff.relative`・`log.showSignature`・`status.showUntrackedFiles`・`core.hooksPath` の hook を置く。判定が、設定のないときと同じであることを確かめる。設定ごとに、まずその設定が素の git の出力を変えることを fixture で確かめる（hook は、素の git で実行したときに目印のファイルを残すことで確かめる）。変えない設定は、この場合に入れない。
-   [x] 「ブランチが条件の表を緩める」場合は、ブランチの条件の表から `requirements_process.md` 自身の `exclude` の行を外し、上限も緩める。こうすると、作業ツリーの表で判定すれば `PASS` になる。この場合の期待は `FAIL` とする。
-   [x] どの場合も、実行の後に `TMPDIR` が空で、`git worktree list` が fixture の作業ツリーだけを示すことを確かめる。
-   [x] `/bin/bash` が 3.x のときは、同じ場合を `/bin/bash` でも走らせる（1.3 節）。
-   [x] 壊して確かめる。次の 11 個を 1 つずつ壊し（外す、変える、切り替える）、どの場合が失敗するかをコミットメッセージに書く。
    -   設計書 7.3 の 7 つ。それぞれ、次の場合で失敗することを確かめる。
        -   上限の比較：上限の場合
        -   除外するパスの照合：除外の場合
        -   `--no-renames`：名前の変更の数え方の場合
        -   数えられない変更の判定：数えられない変更の場合
        -   `origin/main` の版の条件の表を読むこと：条件の表を緩める場合
        -   再現の確認：再実行で tree が一致しない場合
        -   許可リストとの語の単位の照合と、残りの語のパスの確かめ：許可リストの場合
    -   BASE を `git merge-base` から `origin/main` に変える：BASE の場合。
    -   git の設定の無効化：利用者の git の設定の場合。
    -   `trap`：後片付けを確かめる場合。
    -   `shasum` への切り替え：`sha256sum` のない場合。

**完了の条件（ステップ 1-5・1-6）:** `make fmt`・`make test`・`make lint`・`make deadcode` が通る。

### PR-2 作成ポイント: lightweight check script

**対象ステップ**: 1-5 / 1-6

**推奨タイトル**: `feat(0061): 軽量プロセスの条件を実際の diff で確かめ直すスクリプトとテストを加える`

**レビュー観点**: スクリプトが判定に使う値をエージェントから受け取らず、`origin/main` の版の条件の表を読み、利用者の git の設定に左右されないこと / `FAIL` を diff についての事実だけに使い、手順の誤りや道具の失敗を `STOP` にしていること / 道具のコミットの再現が shell を通さず、一時的な worktree が `trap` で必ず消え、bash 3.2 で動く書き方であること / テストが設計書 7.1 のすべての場合と壊し方を確かめ、CLAUDE.md の例外の範囲が狭いこと

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-5 のスクリプトは、軽量プロセスの条件を機械的に守る信頼境界である（mkplan.md の panel mode の security-gate の条件）。ステップ 1-6 は、git の fixture を多く使う

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/683
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `/runissue` と Part B（設計書 8 章の 2）

#### ステップ 2-1: Part B の変更

**変更するファイル:** `.claude/commands/_lib/task-pr-handoff.md`

-   [x] 設計書 3.6 のとおりに変える。冒頭の段落に、`/runissue` が Part B の手順 3〜7 を使うことを足す。入力 ISSUE_KEYWORD を足す（値は `refs` か `closes`。省けば `refs`、ほかの値なら止まる）。各入力の説明に `/runissue` の場合を足し、「Authorization」の範囲を issue にも広げる。
-   [x] 手順 3 の `Refs #<issue>` の項目（243〜246 行）を、ISSUE_KEYWORD で `Refs` か `Closes` を選ぶ形にする。ISSUE_KEYWORD を省いたとき（今の呼び出し元）の本文は、今と変えない。番号付きの手順と、手順 3・5 の書き出しの文も変えない（`itemSection` が探す。1.4 節）。

#### ステップ 2-2: 呼び出し元の一覧

**変更するファイル:** `.claude/commands/_lib/local-cli-review.md`、`.claude/commands/mktrans.md`、`CLAUDE.md`（99〜100 行）

-   [x] 次の 3 つの一覧に `runissue.md` を足す。`/runissue` は、ローカルの CLI レビューと README の同期を自分で行うためである（設計書 3.5.5）。
    -   `local-cli-review.md` の呼び出し元の一覧（11〜20 行）。冒頭の数（「Five callers」）も一覧の数に合わせる。
    -   `mktrans.md` の、README の同期を手順として行う呼び出し元の一覧（33〜35 行）。
    -   `CLAUDE.md` の、`/mktrans` を実行するコマンドの一覧（99〜100 行）。

#### ステップ 2-3: `/runissue`

**変更するファイル:** `.claude/commands/runissue.md`（新規。英語）、`.claude/commands/_context.md`（「Reviewer ping」の行）、`docs/dev/conventions/requirements_process.md`（条件の表の `exclude` の 1 行）

-   [x] 設計書 3.5 の各節のとおりに書く。全体の流れは、設計書 6.1 に合わせる。Part B の手順 3〜7 は、ISSUE_KEYWORD を `closes` にして呼ぶ。サブエージェントによるレビューは、`review-subagent-pattern.md` との違いを明記する（設計書 3.5.4）。
-   [x] 確かめ直しのスクリプトは、`git fetch origin main` の後に、`git show origin/main:.claude/commands/_lib/lightweight-check.sh` で書き出したファイルを `bash` で実行する（設計書 3.5.6）。作業ツリーの版は実行しない。実行するのは、最後のコミットの後、push の前である。修正のコミットが増えたら、実行し直す。
-   [x] 値（ラベル名、目印、ブランチ名、タイトルと件名の形）は `_context.md` の名前で参照し、仕様の読み書きは仕様の手順書を名前で参照する（AC-39）。CHECKS（実装の後に行う確認の一覧。設計書 3.5.3）は CLAUDE.md を参照し、既存の要件書の検索は `mkreq.md` の手順 1 を参照する。どちらも書き写さない。issue の本文とタイトルは取らない（設計書 1.2）。
-   [x] 戻しのコメントの雛形は、日本語で 1 つのコードブロックに置く。投稿の前に「**Mention check.**」の段（`_lib/task-pr-handoff.md` と同じ文）と、1 行目が仕様の目印でないことの確かめを置く（AC-20・AC-30）。投稿のコマンドは `gh issue comment <N> --body-file <file>` の形にする。
-   [x] 「Authorization」に、設計書 3.5 が使う操作を、その issue とその PR に限って書く（AC-42）。
-   [x] 条件の表に `exclude .claude/commands/runissue.md` を足す。`_context.md` の「Reviewer ping」の行に `runissue.md` を足す（1.3 節）。

#### ステップ 2-4: `/runissue` の guard

**変更するファイル:** `cmd/yt2column/codex_review_docs_test.go`、`cmd/yt2column/fixpr_docs_test.go`、`cmd/yt2column/lightweight_docs_test.go`

-   [x] `postingSections` に、`runissue.md` の戻しのコメントの区画を足す。`runissue.md` の GraphQL の読み取りが `readOnlyInvocations` の前方一致に当たらなければ、その書き出しを足す。
-   [x] `TestContextAndCommandsAgree` の対象に `runissue.md` を足す。
-   [x] `TestLightweightCheckCallSite` を書く（設計書 3.9「確かめ直しのスクリプトの呼び出し方」）。コマンドの定義で確かめ直しのスクリプトを実行する箇所が、すべて `git show origin/main:` で書き出す形であり、作業ツリーのパスを実行する形がないことを確かめる。
-   [x] `TestRunissueReads` を書く（設計書 3.9「`runissue.md` の読み取り」）。次の 2 つを確かめる。
    -   `runissue.md` の `gh api` の呼び出しが、仕様の手順書の形か、名前を挙げた GraphQL の読み取りに限られる。
    -   `gh issue view` が `--json` を付け、本文・タイトル・コメントの項目を取らない。
-   [x] `TestRevertTemplateMarker` を書く。`runissue.md` の戻しのコメントの雛形の 1 行目が、`_context.md` の戻しの目印の値と同じであることを確かめる（設計書 3.9「雛形の目印」）。
-   [x] 壊して確かめる。次の壊し方のそれぞれで、名前を挙げたテストが失敗することを確かめ、コミットメッセージに書く。
    -   メンションの確認を投稿の後に移す（`TestPostingCommandsInventory`）。
    -   足した `readOnlyInvocations` のコードブロックに `mutation` を入れる（`TestPostingCommandsInventory`）。
    -   スクリプトを作業ツリーのパスで実行する（`TestLightweightCheckCallSite`）。
    -   `gh issue view <N> --comments` を足す（`TestRunissueReads`）。
    -   戻しの雛形の 1 行目を変える（`TestRevertTemplateMarker`）。

**完了の条件（ステップ 2-1〜2-4）:** `make fmt`・`make test`・`make lint`・`make deadcode` が通る。

### PR-3 作成ポイント: runissue command

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4

**推奨タイトル**: `feat(0061): 軽量プロセスの仕様を実装して PR を作る /runissue を加える`

**レビュー観点**: 入口の確認と Part B の直前の確認が、仕様・issue の状態・ラベルの変化を見逃さないこと / 「条件を外れた」（戻す）と「確かめられない」（issue を変えずに止まる）の分け方が設計書 4 章のとおりで、戻しの順序（コメント→ラベル→リース付きのブランチの削除）と途中の失敗の報告が設計書 3.5.7 のとおりであること / 確かめ直しのスクリプトを `origin/main` の版で、最後のコミットの後に実行すること / Part B で ISSUE_KEYWORD を省いたときに、既存のタスクの PR の本文が変わらないこと

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-3 は、GitHub への書き込みと取り消せない戻しを持つ新しいコマンドで、他人の書いた文字列を扱う信頼境界である（mkplan.md の panel mode の security-gate の条件）

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/688
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: `/runpr`（設計書 8 章の 3）

#### ステップ 3-1: 軽量プロセスの PR の判定と issue を閉じること

**変更するファイル:** `.claude/commands/runpr.md`

-   [ ] 手順 2 の「Kind」に Lightweight PR を足し、Standalone change を「それ以外」にする（設計書 3.7.1）。判定に使う GraphQL の読み取りと、止まる場合を書く。PRE_MERGE と NEXT はない（AC-33）。
-   [ ] 手順 1 の単独の変更の PR の作成で、対象のブランチの名前が軽量プロセスのブランチ名の形なら、PR を作らずに止まる（設計書 3.7.2）。
-   [ ] 手順 1 の `MERGED` の場合に、軽量プロセスの PR なら手順 5 の issue を閉じる項目を実行する、と書き足す（AC-35）。手順 1 では、その項目を参照し、`gh issue close` を書かない（書くと、手順 1 の区画で `TestPostingCommandsInventory` が失敗する）。
-   [ ] 手順 4 の 1 に、軽量プロセスの PR の squash のメッセージは本文の最後を `Closes #<LW_ISSUE>` とする、と書き足す（設計書 3.7.3）。
-   [ ] 手順 5 の issue を閉じる項目（809〜827 行）を、「閉じる issue」を持つ形に広げる（設計書 3.7.4）。既存の「**Mention check.**」の段と `gh issue close` のコマンドは、この項目の中に 1 つずつのまま残す。軽量プロセスの PR では、コメントの 1 行目が仕様の目印でないことも確かめる。タスクの最後のプランの PR の条件は変えない（AC-36）。
-   [ ] 手順の見出し（`runprStep1`・`runprStep3`・`runprStep4`・`runprStep5` が探すもの）は変えない。
-   [ ] 「Authorization」に、設計書 3.7.4 の操作を足す（AC-44）。

#### ステップ 3-2: `/runpr` の guard

**変更するファイル:** `cmd/yt2column/codex_review_docs_test.go`

-   [ ] `runpr.md` の GraphQL の読み取りの書き出しを `readOnlyInvocations` に足す（設計書 2.2）。そのコードブロックに `mutation` を書かない。
-   [ ] 壊して確かめる。次の壊し方のそれぞれで、名前を挙げたテストが失敗することを確かめ、コミットメッセージに書く。
    -   足したコードブロックに `mutation` を入れる（`TestPostingCommandsInventory`）。
    -   手順 5 のメンションの確認を `gh issue close` の後に移す（`TestPostingCommandsInventory`）。

**完了の条件（ステップ 3-1・3-2）:** `make fmt`・`make test`・`make lint`・`make deadcode` が通る。

### PR-4 作成ポイント: runpr lightweight PR handling

**対象ステップ**: 3-1 / 3-2

**推奨タイトル**: `feat(0061): /runpr で軽量プロセスの PR を見分けてマージの後に issue を閉じる`

**レビュー観点**: 軽量プロセスの PR の判定が本文を読まず、GitHub の構造化された値だけで決まり、決められないときに止まること / 軽量プロセスのブランチから単独の PR を作らないこと / `MERGED` の場合の実行し直しで issue が閉じること / タスクの PR と単独の変更の扱いが変わっていないこと（手順 2・5 の差分）

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-1 は、手順 2 の種類の分岐と手順 5 の issue を閉じる処理を変え、既存のタスクの PR の振る舞いを保つ必要がある（隔離すべき分岐の変更）

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 入口と周辺（設計書 8 章の 4）

#### ステップ 4-1: CLAUDE.md と AGENTS.md

**変更するファイル:** `CLAUDE.md`、`AGENTS.md`

-   [ ] `CLAUDE.md` を設計書 3.8 のとおりに変える（AC-37・AC-38）。
    -   33 行に、例外と範囲を足す。
    -   「Claude Commands」の一覧に `/runissue <N>` を足し、`/mkreq` の行に振り分けを足す。
    -   166 行の段落を改める。
    -   「Requirements and Acceptance Criteria」に、概要と `requirements_process.md` の節へのリンクを足す。
-   [ ] `AGENTS.md` の 11 行の規則に、軽量プロセスの例外を足す（設計書 3.1.5・3.8）。

#### ステップ 4-2: issue テンプレート

**変更するファイル:** `.github/ISSUE_TEMPLATE/lightweight.md`（新規。日本語）

-   [ ] 設計書 3.8 のとおりに書く。front matter に `labels` を書かない。本文には、冒頭の `/mkreq <N>` の 1 行と、背景・やること・やらないこと・確かめ方の見出しを置く（AC-40）。

#### ステップ 4-3: `/mkreq` の振り分け

**変更するファイル:** `.claude/commands/mkreq.md`

-   [ ] 手順 1 の最後の項目として、太字の名前を付けた振り分けの項目を足す（1.3 節）。中身は設計書 3.4.1〜3.4.2 の順のとおりで、戻した issue の判定、AC-15 の 3 つの停止、`[task]` の扱い、条件に当たるかの判断である。仕様のコメントの読み書きは、仕様の手順書を参照する。
-   [ ] 問いの形（設計書 3.4.3）と、投稿とラベルの付与（設計書 3.4.4）を書く。ラベルの有無は、件数の上限に左右されない形で確かめる（1.3 節）。投稿の前に「**Mention check.**」の段を置き、投稿のコマンドは `gh issue comment <N> --body-file <file>` の形にする。
-   [ ] front matter の `description`、目的の段落（12〜17 行）、手順 9 の報告、「Authorization」（設計書 3.4.4 の操作、AC-43）を改める。

#### ステップ 4-4: `/mkreq` の guard

**変更するファイル:** `cmd/yt2column/codex_review_docs_test.go`、`cmd/yt2column/fixpr_docs_test.go`

-   [ ] `postingSections` に、`mkreq.md` の振り分けの区画を足す。区画は `itemSection` で探し、振り分けの項目の書き出しから `"\n2. Resolve the task directory."` までとする。
-   [ ] `TestContextAndCommandsAgree` の対象に `mkreq.md` を足す。
-   [ ] 壊して確かめる：振り分けの区画で、メンションの確認を投稿の後に移す。`TestPostingCommandsInventory` が失敗することを確かめ、コミットメッセージに書く。

#### ステップ 4-5: `lightweight` ラベル

**変更するファイル:** なし（GitHub のラベル）。記録は本書 7.1 節。

-   [ ] `/runplan` が、PR-5 を開く前（ステップ 4-1〜4-4 の後）に、ラベルを作る（AC-41）。
    -   まず、ラベルがないことを、件数の上限に左右されない形で確かめる。
    -   利用者の承認を得てから、`gh label create` で、色 `c2e0c6` と設計書 1.2 の説明で作る。
    -   確かめた出力を 7.1 節に書き、PR を開く前にコミットする。
    -   PR-5 のレビューで色や説明を変えることになったときは、利用者がラベルを手で直す。7.1 節の記録は、PR の修正として直す。
    -   PR-5 がマージされる前にラベルがあっても、`main` の `/mkreq` はまだ振り分けないので、仕様を投稿した issue は生まれない（3.1 節）。

**完了の条件（ステップ 4-1〜4-5）:** `make fmt`・`make test`・`make lint`・`make deadcode` が通る。ラベルがリポジトリにある。

### PR-5 作成ポイント: documents and mkreq triage

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5

**推奨タイトル**: `feat(0061): /mkreq で軽量プロセスへ振り分け、CLAUDE.md・AGENTS.md・issue テンプレートを改める`

**レビュー観点**: 振り分けの判定の順序と、AC-15 の 3 つの停止と戻した issue の扱い / 作ったラベルの色と説明が設計書 1.2 のとおりで、7.1 節に記録があること / 承認が、示した全文についての選択に限られ、投稿する文面が示した全文とバイト単位で同じであること / 投稿・ラベルの付与の失敗の報告 / CLAUDE.md・AGENTS.md の例外の範囲が、F-001 の条件と承認された仕様に限られていること

**実装モデル要件**: frontier-required

**判定理由**: ステップ 4-3 は、`/runissue` が後で実装の指示として信頼するコメントを、利用者の承認に基づいて投稿する承認の関門である（mkplan.md の panel mode の security-gate の条件）。そのため、文書の変更（4-1・4-2）の後に置いた

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: 規則と値 | 軽量プロセスの節と条件の表、`_context.md` の値、仕様の手順書、条件の表・一覧のフィルタ・雛形の目印のテスト | PR-1 がマージされた |
| M2: 確かめ直し | 確かめ直しのスクリプトとテスト、CLAUDE.md のリテラルの例外 | PR-2 がマージされた |
| M3: `/runissue` | `runissue.md`、Part B と呼び出し元の一覧の変更、`/runissue` の guard | PR-3 がマージされた |
| M4: `/runpr` | 軽量プロセスの PR の判定、issue を閉じること | PR-4 がマージされた |
| M5: 入口と周辺 | CLAUDE.md・AGENTS.md、issue テンプレート、`/mkreq` の振り分けと guard、ラベル | PR-5 がマージされた |

PR は 1→5 の順にマージする。軽量プロセスを使えるようになるのは M5 の後である。ラベルは、PR-5 の実装の中（ステップ 4-5）で作るまで手で作らない。PR-3 と PR-4 の間に手でラベルを付けて `/runissue` を実行すると、その PR を `/runpr` がまだ軽量プロセスの PR として扱えず、マージの後に issue が開いたまま残るからである。M5 より前に `/mkreq` は振り分けないので、仕様を投稿した issue は生まれない（設計書 8 章）。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 | 軽量プロセスの節と条件の表、`_context.md` の値、仕様の手順書、条件の表・一覧のフィルタ・雛形の目印のテスト | frontier-required |
| PR-2 | 1-5 / 1-6 | 確かめ直しのスクリプトとテスト、CLAUDE.md のリテラルの例外 | frontier-required |
| PR-3 | 2-1 / 2-2 / 2-3 / 2-4 | `/runissue`、Part B と呼び出し元の一覧、`/runissue` の guard | frontier-required |
| PR-4 | 3-1 / 3-2 | `/runpr` の判定と issue を閉じること | frontier-recommended |
| PR-5 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 | CLAUDE.md・AGENTS.md、issue テンプレート、`/mkreq` の振り分けと guard、ラベル | frontier-required |

## 4. テスト戦略 (Test Strategy)

設計書 7 章のとおりである。どの AC をどの確かめで確かめるかは、本書 7 章に示す。

-   **単体テスト:** 確かめ直しのスクリプトを fixture のリポジトリで実行する `TestLightweightCheck`（ステップ 1-6）。設計書 7.1 のすべての場合と、ステップ 1-6 に挙げた追加の場合を確かめる。
-   **文書の値と構造のテスト:**
    -   除外するパスの実在：`TestLightweightExcludedPaths`（ステップ 1-4）
    -   一覧のフィルタ：`TestLightweightSpecListing`（ステップ 1-4）
    -   雛形の目印：`TestLightweightSpecTemplateMarker`（ステップ 1-4）、`TestRevertTemplateMarker`（ステップ 2-4）
    -   確かめ直しのスクリプトの呼び出し方：`TestLightweightCheckCallSite`（ステップ 2-4）
    -   `runissue.md` の読み取り：`TestRunissueReads`（ステップ 2-4）
    -   投稿の区画のメンションの確認：既存の `TestPostingPathsCheckMention`・`TestPostingCommandsInventory`。区画を足すのはステップ 2-4・4-4、`readOnlyInvocations` を足すのはステップ 2-4・3-2 である。
-   **テストを足さないもの:** ラベル名、CLAUDE.md のコマンドの一覧、テンプレートの見出し、仕様のコメントの形の確かめ方、`_context.md` の新しい値の参照（設計書 3.9 の理由のとおり）。条件の表の上限の値も、テストに写さない（1.3 節）。
-   **結合テスト:** コマンドの流れ（`/mkreq` から `/runpr` まで）は、自動のテストにしない。PR-5 のマージの後、最初の実際の軽量プロセスの issue で、利用者と一緒に流れを確かめる（設計書 7.2、本書 9 章）。
-   **後方互換:** 既存のテスト（`TestRunprDocContract`・`TestHandoffListingPrintsNoBody`・`TestPostingCommandsInventory`・`TestContextAndCommandsAgree` など）を、変えた文書で通す。各 PR のグリーンゲートで確かめる。
-   **壊して確かめる:** ステップ 1-4・1-6・2-4・3-2・4-4 の壊し方を実装のときに行い、コミットメッセージに書く（設計書 7.3）。
-   **読んで確かめる:** 文書の文言だけを定める AC は、PR のレビューで読む（設計書 7.4）。
-   **カバレッジ:** 数値の目標は置かない。本タスクはテストを削除しないので、`make coverage-compare` は使わない。
-   **セキュリティ:** 設計書 7.5 のとおりである。メンションの確認は投稿の区画のテストで確かめる。書き手で絞ることと本文を出さないことは、一覧のフィルタのテストと `runissue.md` の読み取りのテストで確かめる。除外・数えられない変更・条件の表の独立性・道具のコミット・git の設定の無効化は、確かめ直しのスクリプトのテストで確かめ、スクリプトの呼び出し方は呼び出しのテストで確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| スクリプトが macOS の bash 3.2 で動かない | 利用者の環境で `/runissue` が `STOP` する | bash 4.0 以降の機能を使わない。`/bin/bash` が 3.x の環境では、テストをそれでも走らせる（ステップ 1-6） |
| 利用者の git の設定（署名の表示、サブモジュールを無視する設定など）で、スクリプトの読み取りが変わる | 誤った判定、または理由のない `STOP` | グローバルとシステムの設定を読まず、リポジトリの設定を明示したオプションで無効にする（ステップ 1-5）。テストで設定ごとに確かめる（ステップ 1-6） |
| スクリプトのテストが、pre-commit の hook の中で引き継いだ `GIT_*` の環境変数によって、別のリポジトリを操作する | テストが誤って通るか、作業中のリポジトリを壊す | 許可リストの環境で走らせる（1.3 節） |
| 再現の確認の一時的な worktree が残る | 次の実行や `git worktree list` が汚れる | `trap` で必ず消す（ステップ 1-5）。テストで、どの場合の後にも残らないことを確かめる（ステップ 1-6） |
| `checkListings` の拡張が、既存の一覧のテストを壊す | `TestRunprDocContract` などが失敗する | 置き換えの印と `id` を足すだけにし、ステップ 1-4 で既存のテストが通ることを確かめる |
| 条件の表の除外するパスに漏れがある | 信頼境界の変更が、軽量プロセスで進む | 設計書 3.1.3 の表と照らして、PR-1 でレビューする。新しい信頼境界を足す PR が `exclude` を足す規則を書く（ステップ 1-1） |
| PR-5 の実装より前に手でラベルを作り、仕様のコメントを付ける | `/runpr` が扱えない PR ができ、issue が開いたまま残る | ラベルは PR-5 のステップ 4-5 まで作らない（3.1 節） |
| 設計書を `draft` に戻した決定の変更が、本 PR のレビューで認められない | 計画が設計書と食い違う | 本 PR の `/runpr` の問いで、設計書の再承認を受ける。認められないときは、1.3 節の決定と、それに関わるステップを改める。本書は設計書の再承認の前に承認しない |
| 実装が予定より長くかかる | 途中の PR がほかのタスクの `main` の変更と衝突する | PR を小さく保ち（3.2 節）、各 PR の前に `main` を取り込む |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4）
- [x] PR-2 マージ済み（対象ステップ: 1-5 / 1-6）
- [ ] PR-3 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4）
- [ ] PR-4 マージ済み（対象ステップ: 3-1 / 3-2）
- [ ] PR-5 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別は 2 つである。`test` は `make test` で実行するテスト、`manual` は PR のレビューで読むこと、または GitHub の状態を確かめて 7.1 節に記録することである。前後の関係を述べる AC（AC-10・AC-33・AC-36）を `diff` で確かめない理由は、1.3 節に書いた。テストの場所は、すべて `cmd/yt2column/` の下である。

| AC | 種別 | 確かめ | ステップ |
|---|---|---|---|
| AC-01 | manual | PR-1 のレビューで、3 つの類型と確かめ方の文を読む | 1-1 |
| AC-02 | test・manual | test: `lightweight_docs_test.go::TestLightweightCheck`（類型ごとの上限ちょうどと 1 つ超えた値、類型 M の削除行、道具のコミットの行を数えないこと、名前の変更と空白だけの変更の数え方）。manual: 条件の表の値が AC-02 と一致すること | 1-1・1-6 |
| AC-03 | test・manual | test: `TestLightweightCheck`（ファイル数の上限、名前の変更を 2 ファイルに数えること、道具が生成した変更だけのファイルを数えないこと）。manual: 条件の表の値と、`README.en.md` を数える文 | 1-1・1-6 |
| AC-04 | manual | PR-1 のレビューで、ちょうど 1 つの類型に収まる規則の文を読む | 1-1 |
| AC-05 | test・manual | test: `lightweight_docs_test.go::TestLightweightExcludedPaths`、`TestLightweightCheck`（除外の各場合）。manual: 除外する変更の文と、条件の表のパスが設計書 3.1.3 の表と一致すること | 1-1・1-4・1-6 |
| AC-06 | test・manual | test: `TestLightweightExcludedPaths` と `TestLightweightCheck` は、見出し「#### Mechanically checked conditions」の直後のコードブロックを読む。manual: 機械的に確かめる条件とレビューで確かめる条件の区別の文 | 1-1・1-4・1-6 |
| AC-07 | manual | PR-1 のレビューで、「Review Flow」の軽量プロセスの流れを読む | 1-1 |
| AC-08 | manual | PR-5 のレビューで、`mkreq.md` の振り分けの位置と条件を読む | 4-3 |
| AC-09 | manual | 同上（問いの前の説明と選択肢、問い直し） | 4-3 |
| AC-10 | manual | 同上（通常のプロセスで進む場合。手順 2 以降の差分がないこと） | 4-3 |
| AC-11 | test・manual | test: `lightweight_docs_test.go::TestLightweightSpecTemplateMarker`（雛形の 1 行目が仕様の目印）。manual: `--body-file` で示したファイルを投稿し、その後にラベルを付ける文 | 1-4・4-3 |
| AC-12 | test・manual | test: `codex_review_docs_test.go::TestPostingPathsCheckMention`、`::TestPostingCommandsInventory`（`mkreq.md` の振り分けの区画）。manual: メンションを含むときにラベルも付けずに止まる文 | 4-3・4-4 |
| AC-13 | manual | PR-5 のレビューで、手順 9 の報告を読む | 4-3 |
| AC-14 | manual | 同上（投稿とラベルの付与の失敗の報告） | 4-3 |
| AC-15 | manual | 同上（3 つの停止と案内） | 4-3 |
| AC-16 | manual | PR-3 のレビューで、`runissue.md` の入口の確認を読む | 2-3 |
| AC-17 | test・manual | test: `lightweight_docs_test.go::TestLightweightSpecListing`（書き手・書き手の種類・目印・CR の扱い）。manual: 最新の 1 件を選ぶ規則と戻した issue の扱いの文 | 1-3・1-4 |
| AC-18 | test・manual | test: `TestLightweightSpecListing`（本文を出さないこと）、`lightweight_docs_test.go::TestRunissueReads`（本文・タイトル・コメントを取らないこと）。manual: 仕様のコメントだけを指示とする文 | 1-4・2-3・2-4 |
| AC-19 | test・manual | test: `TestLightweightCheck`（スクリプトの類型の読み取りの場合。AC-19 の 4 つの見出しの形の確かめそのものではない）。manual: 仕様の手順書の形の確かめ方の文 | 1-3・1-6 |
| AC-20 | test・manual | test: `TestRevertTemplateMarker`（戻しの雛形の 1 行目が戻しの目印）。manual: 投稿の前に 1 行目が仕様の目印でないことを確かめる文（`runissue.md`・`runpr.md`） | 2-3・2-4・3-1 |
| AC-21 | test・manual | test: `TestLightweightCheck`（`docs/tasks/` の下のファイルで `FAIL excluded`）。manual: `runissue.md` のブランチの作成の文 | 1-6・2-3 |
| AC-22 | manual | PR-3 のレビューで、CHECKS と README の同期の文を読む | 2-2・2-3 |
| AC-23 | manual | 同上（テストの削除のコミットと、カバレッジの比較の扱い） | 2-3 |
| AC-24 | manual | 同上（サブエージェントによるレビュー） | 2-3 |
| AC-25 | manual | PR-3 のレビューで、`runissue.md` と `local-cli-review.md` の呼び出し元の一覧を読む | 2-2・2-3 |
| AC-26 | test・manual | test: `TestLightweightCheck`（出力の `spec-sha256` が仕様のファイルの SHA-256 と一致すること、行数とファイル数の行）。manual: 確かめた結果の記録の書式の文 | 1-6・2-3 |
| AC-27 | test・manual | test: `TestPostingPathsCheckMention`（既存の Part B 手順 3 の区画）。manual: PR の本文の項目と ISSUE_KEYWORD の文 | 2-1・2-3 |
| AC-28 | manual | PR-3 のレビューで、Part B の手順 5〜7 を使うことを読む | 2-1・2-3 |
| AC-29 | test・manual | test: `TestLightweightCheck`（設計書 7.1 のすべての場合と、BASE・利用者の git の設定の場合）、`lightweight_docs_test.go::TestLightweightCheckCallSite`（`origin/main` の版を実行すること）。manual: 実行の時点（最後のコミットの後、push の前、修正のコミットの後に実行し直すこと） | 1-6・2-3・2-4 |
| AC-30 | test・manual | test: `TestPostingPathsCheckMention`、`TestPostingCommandsInventory`（`runissue.md` の戻しの区画）。manual: 戻しの手順の順序と失敗の扱い | 2-3・2-4 |
| AC-31 | manual | PR-3 のレビューで、判断が要ったときの戻しの文を読む | 2-3 |
| AC-32 | test・manual | test: `TestLightweightSpecListing`（戻しのコメントを `revert` の種類で出すこと）。manual: 戻した issue で振り分けない文（`mkreq.md`）と、仕様とみなさない文（仕様の手順書） | 1-3・1-4・4-3 |
| AC-33 | manual | PR-4 のレビューで、「Kind」と NEXT の文を読む | 3-1 |
| AC-34 | test・manual | test: `TestPostingPathsCheckMention`、`TestPostingCommandsInventory`（既存の `runpr.md` 手順 5 の区画）。manual: 閉じる issue の文 | 3-1・3-2 |
| AC-35 | manual | PR-4 のレビューで、手順 1 の `MERGED` の文を読む | 3-1 |
| AC-36 | manual | PR-4 のレビューで手順 5 の差分を読み、タスクの最後のプランの PR の条件が変わっていないことを確かめる（1.3 節） | 3-1 |
| AC-37 | manual | PR-5 のレビューで、CLAUDE.md の概要・リンク・コマンドの一覧・段落を読む。リンクの実在は `docpaths_test.go::TestInstructionDocPaths`（既存） | 4-1 |
| AC-38 | manual | PR-1 のレビューで「LLM Constraints」を、PR-5 のレビューで CLAUDE.md の 33 行を読む | 1-1・4-1 |
| AC-39 | manual | PR-1 のレビューで `_context.md` の値を、PR-3〜PR-5 のレビューでコマンドの定義が値を名前で参照することを読む（テストを足さない理由は設計書 3.9） | 1-2・2-3・3-1・4-3 |
| AC-40 | manual | PR-5 のレビューで、テンプレートの見出しと front matter を読む（同上） | 4-2 |
| AC-41 | manual | ステップ 4-5 でラベルを確かめ、出力を 7.1 節に書く | 4-5 |
| AC-42 | manual | PR-3 のレビューで、`runissue.md` が英語で書かれ、手順と「Authorization」を持つことを読む | 2-3 |
| AC-43 | manual | PR-5 のレビューで、`mkreq.md` の手順と「Authorization」を読む | 4-3 |
| AC-44 | manual | PR-4 のレビューで、`runpr.md` の「Authorization」を読む | 3-1 |

### 7.1. 実行の記録

（ステップ 4-5 で、ラベルを確かめた出力と実行した日を書く。）

## 8. 成功基準 (Success Criteria)

-   **機能の完成度:** 7 章のすべての行の確かめが済み、7.1 節にラベルの記録がある。
-   **品質:** 各 PR で、グリーンゲート（`make test && make lint`）・`make fmt`・`make deadcode` が通る。加えたテストは、対象を壊すと失敗することをコミットメッセージに記録している。
-   **セキュリティ:** 4 章の「セキュリティ」に挙げたテストが、すべてある。
-   **文書:** `requirements_process.md`・`_context.md`・CLAUDE.md・AGENTS.md・コマンドの定義が軽量プロセスを定め、issue テンプレートがある。

## 9. 次のステップ (Next Steps)

-   PR-5 のマージの後、`/runpr` が issue #619 を閉じる（本タスクの最後の PR のため）。
-   最初の実際の軽量プロセスの issue で、`/mkreq` の振り分けから `/runpr` のマージまでを、利用者と一緒に確かめる（設計書 7.2）。

## 変更履歴

- 2026-11-04 承認（isseis、#670）
