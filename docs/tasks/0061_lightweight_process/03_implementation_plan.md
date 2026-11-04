# 実装計画書：単純な変更のための軽量プロセスと、/mkreq での振り分け

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-04 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番を、「設計書 3.5.6」などは設計書の節を指す。用語（軽量プロセス、通常のプロセス、類型、仕様のコメント、条件の表、確かめ直しのスクリプト、仕様の手順書、戻しのコメント、道具のコミット、確かめた結果の記録、Part B など）は、要件書 6 章と設計書の冒頭に従う。本タスクに `implementation_handoff.md` はない。[design_handoff.md](design_handoff.md) の H-01〜H-11 は、設計書の付録 A がすべて記録している。

既存のファイルについての記述は、コミット `68d328ac`（2026-11-04 の `main`）で確かめた。設計書が行番号を引いたコミット `74b9b49` から `68d328ac` までに、本タスクが変えるファイルのうち変わったのは `CLAUDE.md` の 270〜282 行（サーバの説明の段落）だけである（`git diff --stat 74b9b49 68d328ac -- .claude CLAUDE.md AGENTS.md docs/dev cmd/yt2column internal/doctestutil .github Makefile` の出力）。本タスクが変える箇所の行番号は変わっていない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

単純な変更を、タスク文書を作らずに 1 本の PR で進める軽量プロセスを作る。条件の定義、`/mkreq` の振り分け、仕様の手順書、`/runissue`、確かめ直しのスクリプト、`/runpr` の変更、周辺の文書とラベルである。何を作るかは設計書 1〜6 章で決まっている。本書は、それをどの順序で、どの PR に分け、どう確かめるかを定める。

### 1.2. 実装の原則

-   フェーズは設計書 8 章の 4 つの段階（1〜4）に合わせる。フェーズ 1 だけを 2 本の PR に分け、全体を 5 本の PR にする（1.3 節）。
-   文書とコマンドの定義に書く内容は設計書の節を参照し、本書では繰り返さない。文の最終的な文言、テストの表の行、スクリプトの出力の細部は、書くときに決める。
-   条件の表の `exclude` の行は、そのパスを作る PR で足す（設計書 8 章）。除外するパスの実在のテスト（ステップ 1-4）が、まだないパスで失敗しないためである。
-   加えたテストは、対象を壊すと失敗することを実装のときに確かめ、コミットメッセージにそう書く（CLAUDE.md「Testing Strategy」、設計書 7.3）。
-   Go のコメント・識別子・文字列リテラルは英語で書く。例外は、guard-test literal exception と、確かめ直しのスクリプトとそのテストの仕様の見出しのリテラル（設計書 3.1.5。ステップ 1-5 で CLAUDE.md に足す）だけである。`.claude/commands/` と `docs/dev/conventions/` の文書は英語、仕様のコメントの雛形・戻しのコメント・issue テンプレート・PR の本文は日本語で書く（CLAUDE.md「Document language」）。
-   各 PR の終わりに、グリーンゲート（`make test && make lint`）を通す。Go のファイルを変える PR では `make fmt` と `make deadcode` も通す。本タスクは拡張・README・`docs/dev/guides/development.md`・`.gitignore` を変えないので、`make ext-test` は要らない（CLAUDE.md「Development Notes」）。
-   GitHub への書き込み（ラベルの作成）は、そのときに利用者の承認を得る（CLAUDE.md「Tool Execution Safety」）。

### 1.3. 計画の際に決めたこと（2026-11-04）

レビュアーと決めたこと:

-   **PR は 5 本にする。設計書 8 章の段階 1 を、「規則と値」（PR-1）と「確かめ直しのスクリプトとテスト」（PR-2）に分ける:** 確かめ直しのスクリプトは信頼境界そのもので、壊して確かめる項目も多い（設計書 7.3）。規則の文と混ぜるとレビューが散るので、単独でレビューする。段階 2〜4 は、それぞれ 1 本の PR にする。
-   **条件の表の `tool` の行は、最初は `gofumpt -w` と `git mv` の 2 行にする:** 設計書 3.1.3 は「...」と残している。`golangci-lint` はローカルでは `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run --build-tags test,integration` で動く（`Makefile:9-10`）。`go run` はモジュールを取りに行き、`--fix` の結果は linter の版にも依存するので、再現の確認が `STOP` になりやすい。`golangci-lint run --fix` の変更は手で書いた行に数える（類型 M の上限の中なら軽量プロセスで進められる）。後から行を足すのは、通常のプロセスの PR で行う。
-   **確かめ直しのスクリプトとそのテストに、仕様の見出し `### 類型` の日本語のリテラルを許す例外を CLAUDE.md の「Go Idioms」に足す:** スクリプトは仕様の本文の `### 類型` の下の 1 文字を読む（設計書 3.5.6）ので、ソースに日本語の見出しが要る。CLAUDE.md の「Language」の例外には shell のスクリプトが入っていない。計画の作成のときにこの食い違いを見つけ、利用者と決めて、設計書を直した（設計書 1.2・3.1.5。設計書を `draft` に戻し、本 PR で再び承認を受ける）。

上の決定と設計書から導いたこと（PR のレビューで異論があれば改める）:

-   **除外するパスの実在のテストは、条件の表と同じ PR-1 に入れる:** 設計書 3.9 のテストのうち、実在のテストは条件の表だけを読む。表の誤りをその PR で見つけるため、スクリプトのテスト（PR-2）を待たない。テストのファイル `cmd/yt2column/lightweight_docs_test.go` も PR-1 で作り、`exclude` の行も PR-1 で足す。
-   **一覧のフィルタのテストは、設計書 8 章のとおり段階 2（PR-3）に置く:** 仕様の手順書（フィルタを含む）は PR-1 で入るが、それを実行するコマンド（`/runissue`・`/mkreq` の振り分け）は PR-3 より前にはない。
-   **`_context.md` の「Reviewer ping」の行への `runissue.md` の追加は、`runissue.md` を作る PR-3 で行う:** PR-1 で足すと、まだないコマンドを名前で挙げることになる。PR-1 の `_context.md` の変更は、設計書 3.2 の値だけにする。
-   **条件の表の上限の値（AC-02・AC-03）は、テストに写して固定しない:** CLAUDE.md の「Document tests ... never prose」は、テストに写したリテラルで文書の値を固定することを禁じる。スクリプトのテストは、上限を実際の条件の表から読み、上限ちょうどと 1 つ超えた値で確かめる。表の値が要件書の値と同じであることは、PR-1 のレビューで読む（7 章の AC-02・AC-03）。
-   **確かめ直しのスクリプトは bash 3.2 でも動く書き方にする:** macOS の `/bin/bash` は 3.2.57 である（`/bin/bash --version`、2026-11-04）。`/runissue` は `PATH` の `bash` で実行するので、利用者の環境によっては 3.2 で動く。連想配列・`mapfile`・`${var,,}` など 4.0 以降の機能を使わない。テストは `PATH` の `bash` で走らせる（CI は ubuntu-latest の bash 5）。
-   **スクリプトのテストは、継いだ `GIT_*` の環境変数と利用者の git の設定を外して git とスクリプトを走らせる:** pre-commit の `go-test` の hook（`.pre-commit-config.yaml:4-6`）は `git commit` の中で `go test -tags test ./...` を実行するので、`GIT_INDEX_FILE` などが継がれうる。`cmd/fixprguard/main_test.go:266`（`TestGhRunsWithoutGitVariables`）が同じ問題を扱った前例である。
-   **一覧のフィルタのテストは、目印の値を `_context.md` の行から読む:** テストに目印の文字列を写さず、ステップ 1-2 で足す「Lightweight spec marker」「Lightweight revert marker」の行のコードスパンを使う（既存の `contextRow`、`cmd/yt2column/fixpr_docs_test.go:77`）。
-   **`CLAUDE.md` の「Development Notes」の jq の項目（591 行）に、一覧のフィルタのテストを足す:** この項目は、jq を使うテストを名前で挙げている。新しいテストも `checkListings` で jq を使うので、挙げないと項目が古くなる。

### 1.4. 既存コード調査結果

**メンションの確認の guard（`cmd/yt2column/codex_review_docs_test.go`）**

-   `postingSections`（`:60-79`）は、投稿する区画の名前・ファイル・区画の探し方・投稿のコマンドの並びである。区画の探し方は、見出しで探す `headingSection`（`:33`）と、番号付きの項目の書き出しから次の項目の書き出しまでを取る `itemSection`（`:42`）の 2 つがある。`mkreq.md` の手順は番号付きの項目（`.claude/commands/mkreq.md:34` の `1. Check the starting state.` など）なので、振り分けの区画は `itemSection` で探す。
-   `TestPostingCommandsInventory`（`:200`）は、`doctestutil.CommandDocs`（`internal/doctestutil/protocol.go:85`、`.claude/commands` の下の `.md` だけ）のすべての投稿のコマンドを確かめる。`-F`・`-f` を渡す `gh api graphql` も投稿に数える（`textFlag`、`:139`）。例外の `readOnlyInvocations`（`:148-150`）は前方一致で、今は `gh api graphql -F owner='{owner}' -F repo='{repo}' -F n=<issue> -f query='` の 1 つである。`isReadOnly`（`:172`）は、続くコードブロックに `mutation` がないことも求める。
-   `gh issue edit --add-label`・`--remove-label`、`gh issue develop`、`git push` は `postingCommand`（`:136`）に当たらないか、本文のフラグを持たないので、投稿に数えない（設計書 2.2「既存のテストへの影響」のとおり）。
-   `.claude/commands/_lib/lightweight-check.sh` は `.md` でないので、`CommandDocs` を使うテスト（メンションの検索・投稿の一覧）と `TestInstructionDocPaths`（`cmd/yt2column/docpaths_test.go:140`。`instructionDocs`、`:106` も `.md` だけ）の対象にならない。

**一覧のフィルタの guard（`cmd/yt2column/fixpr_docs_test.go`）**

-   `checkListings`（`:431`）は、区画の中の `gh api --paginate` の一覧を、正規表現 `paginatedListing`（`:214`、`gh api --paginate repos/<owner>/<repo>/(\S+) --jq '([^']*)'`）で取り出して、fake の応答に jq で実行する。フィルタは単引用符を含められない。置き換えの印は `<awaited-reviewer>`・`<user-login>`・`<review-request>`・`<review-mention>` の 4 つ（`:443-448`）で、残った印（`leftPlaceholder`、`:216`、`<[a-z][a-z-]*>`）は失敗にする。
-   fake の応答（`listingFixture.response`、`:404`）は `user`・`body`・`html_url` と、`timeFieldSeconds`（`:397`）にある時刻の項目だけを持つ。`id` はない。`checkListingOutput`（`:622`）は、出力の値を `source` の対応で応答の項目と `!=` で比べる。JSON から読んだ数は `float64` になるので、`id` を足すときは応答の側も同じ型で持つ。
-   `checkGhAPICalls`（`:488`）は、区画の中の `gh api` の呼び出しが許す一覧（`readCalls`）の前方一致に当たることを求める。仕様の手順書の区画には、`gh api user --jq .login` と 1 件の本文の読み取り（`gh api repos/<owner>/<repo>/issues/comments/<id> --jq .body`）がある。
-   区画を探す `requireSection`（`:92`）、`flat`（`:63`）、`_context.md` の行を読む `contextRow`（`:77`）は、そのまま使える。

**`internal/doctestutil`**

-   コマンドの定義の見出しの定数は、今は `cmd/yt2column/fixpr_docs_test.go:29-41` にある。設計書 2.2・3.9 は、新しいテストの見出しの定数を `internal/doctestutil` に置くとしているので、新しいファイル（`internal/doctestutil/lightweight.go`、`//go:build test`）に置く。日本語の手引きの見出しを置く `guides.go` とは分ける。

**git を使うテストの前例**

-   `internal/coveragecompare/run_test.go` と `test_helpers.go` は、`t.TempDir()` に fixture のリポジトリを作り、HOME と TMPDIR をテストのものにして git を走らせる（`test_helpers.go:501-516`）。`TestDiffIgnoresUserGitConfig`（`run_test.go:712`）は、利用者の git の設定（diff のアルゴリズム、textconv など）で結果が変わらないことを確かめる。どちらも `coveragecompare` のパッケージの中の非公開の関数で、`cmd/yt2column` から使えない。確かめ直しのスクリプトのテストの fixture は、`lightweight_docs_test.go` の中に置く（ほかのパッケージから使わないので、`testutil/` は作らない）。

**外部の前提の確かめ（2026-11-04、この機械で実行）**

-   `git diff --no-ext-diff --no-textconv --no-color --diff-algorithm=myers --no-renames --numstat -z` は git 2.50.1 で受け付けられ、NUL で区切った行を出す（`git diff ... HEAD~1 HEAD` を `68d328ac` で実行）。
-   `git push --force-with-lease=refs/heads/<branch>:<SHA> origin :refs/heads/<branch>` は、`origin` のブランチが `<SHA>` のときだけ消し、ほかの値なら `(stale info)` で拒み、終了コード 1 を返す。スクラッチパッドの bare リポジトリで、古い SHA と正しい SHA の両方を試した（設計書 3.5.7 の消し方）。
-   SHA-256 のコマンドは、Linux の `sha256sum`、macOS 26 の `/sbin/sha256sum`、どちらにもある `shasum -a 256` が使える（`which`）。スクリプトはどれかを決め打ちせず、使えるものを選ぶ。
-   CI の `doc-guard` ジョブは `go test -tags test ./cmd/yt2column ...` をすべての PR で実行する（`.github/workflows/ci.yml:175-187`、ubuntu-latest）。新しいテストは bash・git・jq を使う。jq は既存のテストがすでに要する（CLAUDE.md:591）。
-   リポジトリに `lightweight` ラベルはない（`gh label list`、2026-11-04。ラベルは 21 個）。ラベルはステップ 4-5 で作る。

**本タスクが変える箇所（`68d328ac` の行番号）**

-   `docs/dev/conventions/requirements_process.md`：「LLM Constraints」（7〜14 行）、「Review Flow」（63〜78 行）、新しい小節「### Lightweight Process」は「## 0. Review Workflow」の中、「## 1.」（388 行）の前に置く（設計書 3.1.1）。
-   `.claude/commands/_context.md`：「Process convention」の表（35〜54 行）、「Reviewer ping」の行（50 行）、「PR title and squash subject」の表（75〜82 行）。
-   `.claude/commands/_lib/task-pr-handoff.md`：冒頭の説明（13〜19 行）、入力（34〜62 行。TASK_DIR 36 行、PR_TEXT 44 行、ISSUE 48 行）、「Authorization」（57〜59 行）、Part B の手順 3 の `Refs #<issue>` の項目（243〜246 行）。
-   `.claude/commands/_lib/local-cli-review.md`：呼び出し元の一覧（「Five callers run it」、11〜20 行）。
-   `.claude/commands/runpr.md`：「Authorization」（65〜90 行）、手順 1 の `MERGED`（109〜117 行）と単独の変更の PR の作成（117〜171 行）、手順 2 の「Kind」（262〜271 行）、手順 4 の 1（645〜651 行）、手順 5 の issue を閉じる項目（809〜827 行）。
-   `.claude/commands/mkreq.md`：front matter の `description`（2 行）、目的の段落（12〜17 行）、「Authorization」（26〜30 行）、手順 1（34〜49 行）と手順 2（51 行）の間、手順 9（151 行）。
-   `CLAUDE.md`：33 行（Each task lives in）、154 行（`/mkreq`）、166 行（Each task PR is handled by two commands）、591 行（jq の項目）、621 行（「## Go Idioms」。guard-test literal exception は 645 行）、668 行（「## Requirements and Acceptance Criteria」）。
-   `AGENTS.md`：11 行（実装のコミットについての指摘の規則）。
-   `.github/ISSUE_TEMPLATE/`：今は `accessibility.md` だけがある。テンプレートを読むテストはない（`grep -rn ISSUE_TEMPLATE` を `cmd`・`internal`・`extension/test` で実行した結果が 0 件）。

**除外するパスの実在（設計書 3.1.3 の表）:** 表のパスのうち、本タスクが作る 4 つ（`.claude/commands/runissue.md`・`.claude/commands/_lib/lightweight-spec.md`・`.claude/commands/_lib/lightweight-check.sh`・`cmd/yt2column/lightweight_docs_test.go`）を除くすべてが `68d328ac` にある。`exclude-absent` の `go.sum`・`go.work`・`.gitmodules` はない（`git cat-file -e` で 1 つずつ確かめた）。

**型による保証:** 本タスクは製品のコードに Go の型を加えない（設計書 3 章の冒頭）。具体的な型で振る舞いを分ける設計はないので、型の作り方を列挙して確かめる必要もない。

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: 条件と値（設計書 8 章の 1）

#### ステップ 1-1: 軽量プロセスの節と条件の表

**変更するファイル:** `docs/dev/conventions/requirements_process.md`

-   [ ] 「## 0. Review Workflow」の中に小節「### Lightweight Process」を足し、設計書 3.1.1 の項目を英語で書く。類型・上限・除外の文言は要件書の AC-01〜AC-05 をそのまま英語にする。数え方は設計書 3.1.2、機械的に確かめる条件とレビューで確かめる条件は設計書 3.1.4 のとおりにする。
-   [ ] 見出し「#### Mechanically checked conditions」の直後に、`text` の言語名を付けたコードブロックで条件の表を置く（設計書 3.1.3 の書式）。`limit` の行は AC-02・AC-03 の値、`tool` の行は `gofumpt -w` と `git mv`（1.3 節）、`exclude`・`exclude-absent` の行は設計書 3.1.3 の表のパスのうち、この PR の時点で実在するものと `lightweight-spec.md`・`lightweight_docs_test.go`（この PR で作る）にする。`runissue.md` は PR-3、`lightweight-check.sh` は PR-2 で足す。
-   [ ] 同じ小節に、「信頼境界に当たるパッケージやファイルを新しく足す PR は、同じ PR で条件の表に `exclude` を足す」という規則を書く（設計書 3.1.3）。
-   [ ] 「LLM Constraints」に、軽量プロセスの例外と範囲を足す（AC-38、設計書 3.1.5）。「Review Flow」に、軽量プロセスの流れを足す（AC-07、設計書 2.1）。
-   [ ] まだないファイル（`runissue.md` など）へは Markdown のリンクを張らず、コードスパンで名前を書く（`TestInstructionDocPaths` は相対リンクの実在を確かめる）。

#### ステップ 1-2: `_context.md` の値

**変更するファイル:** `.claude/commands/_context.md`

-   [ ] 「Process convention」の表に、設計書 3.2 の 4 行（Lightweight label・Lightweight spec marker・Lightweight revert marker・Lightweight branch name）を足す。目印の値は、ステップ 2-4 のテストが読めるように、セルの中の 1 つのコードスパンに書く。
-   [ ] 「PR title and squash subject」の表に、Lightweight PR title と Lightweight commit subject の 2 行を足す（設計書 3.2）。squash の件名の正規表現は変えない。

#### ステップ 1-3: 仕様の手順書

**変更するファイル:** `.claude/commands/_lib/lightweight-spec.md`（新規。英語）

-   [ ] 設計書 3.3 のとおりに書く。仕様のコメントの書式（日本語の雛形をコードブロックで）、形の確かめ方（AC-19）、自分のコメントの一覧、仕様とみなすコメントの選び方と戻した issue の判定（AC-17・AC-32）。値は `_context.md` の名前で参照する。
-   [ ] 見出し「## Listing own marker comments」の下に、一覧のコマンドを 1 つだけ書く。形は `checkListings` が読める形にする（1.4 節：`gh api --paginate repos/<owner>/<repo>/issues/<N>/comments --jq '...'`、フィルタに単引用符を入れない、置き換えの印は `<user-login>`・`<spec-marker>`・`<revert-marker>`）。出すキーは `kind`・`id`・`url`・`created_at`・`updated_at` だけにする。同じ節に、`gh api user --jq .login` と、選んだ 1 件の本文の読み取り（`gh api repos/<owner>/<repo>/issues/comments/<id> --jq .body`）を書く。

#### ステップ 1-4: 除外するパスの実在のテスト

**変更するファイル:** `internal/doctestutil/lightweight.go`（新規。`//go:build test`）、`cmd/yt2column/lightweight_docs_test.go`（新規。`//go:build test`）

-   [ ] `internal/doctestutil/lightweight.go` に、条件の表の見出し「#### Mechanically checked conditions」の定数を置く（1.4 節）。
-   [ ] `lightweight_docs_test.go` に、`requirements_process.md` からその見出しの直後のコードブロックを取り出して行に分ける関数を置く（ステップ 1-6 も使う）。見出しやコードブロックがない、または 2 つ以上あるときは失敗にする。
-   [ ] `TestLightweightExcludedPaths` を書く。`exclude` の行のパスがリポジトリに実在し（`/` で終わるものはディレクトリ、それ以外はファイル）、`exclude-absent` の行のパスが実在しないことを確かめる（設計書 3.9）。
-   [ ] 壊して確かめる：`exclude` の 1 行のパスを実在しない名前に変える、`/` を付け外しする、実在するパスを `exclude-absent` にする。それぞれでテストが失敗することを確かめ、コミットメッセージに書く。

**完了の条件（ステップ 1-1〜1-4）:** `make fmt`・`make test`・`make lint`・`make deadcode` が通る。

### PR-1 作成ポイント: lightweight process rules and values

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4

**推奨タイトル**: `docs(0061): 軽量プロセスの条件と条件の表、仕様の手順書、_context.md の値を加える`

**レビュー観点**: 類型・上限・除外の文言が要件書 AC-01〜AC-05 と同じ意味で、条件の表の上限の値が AC-02・AC-03 と一致すること / 条件の表の除外するパスが設計書 3.1.3 の表と一致し、まだ作らないパスが入っていないこと / 仕様の手順書の一覧のフィルタが本文を出さず、書き手で絞る形であること / 「LLM Constraints」の例外の範囲が F-001 の条件と承認された仕様に限られていること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-1 の条件の表が信頼境界の除外を定め、ステップ 1-3 の一覧のフィルタが他人のコメントの本文を会話に入れない防御になる、隔離して読むべき箇所である

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 1-5: 確かめ直しのスクリプト

**変更するファイル:** `.claude/commands/_lib/lightweight-check.sh`（新規）、`docs/dev/conventions/requirements_process.md`（条件の表の `exclude` の 1 行）、`CLAUDE.md`（「Go Idioms」）

-   [ ] 設計書 3.5.6 のとおりに書く。引数は仕様の本文のファイルのパスだけで、BASE・類型・道具のコミットの範囲・条件の表は自分で決める。条件の表は `git show origin/main:docs/dev/conventions/requirements_process.md` から読み、書式の規則（設計書 3.1.3）に合わなければ `STOP` にする。数え方は設計書 3.1.2（git の設定を無効にするオプション、`--no-renames`、NUL 区切り、数えられない変更）に従う。
-   [ ] 道具のコミットの再現は、親のコミットの一時的な worktree で、trailer のコマンドを shell を通さずに実行し、作業ツリーと index の結果の tree の ID をコミットの tree の ID と比べる。worktree は終了のときに必ず消す（`trap`）。
-   [ ] 出力と終了コードは設計書 3.5.6 の表のとおりにする（`PASS` 0・`FAIL` 1・`STOP` 2）。`FAIL` のときは、その前の行に理由ごとの実際の値を出す。
-   [ ] bash 3.2 でも動く書き方にし、ネットワークを使わない（1.3 節・1.4 節）。SHA-256 は使えるコマンドを選んで求める。
-   [ ] 条件の表に `exclude .claude/commands/_lib/lightweight-check.sh` を足す。
-   [ ] `CLAUDE.md` の「Go Idioms」に、確かめ直しのスクリプトとそのテスト（`cmd/yt2column/lightweight_docs_test.go`）のリテラルの例外を、既存の例外と同じ形（範囲・理由・境界）で足す（設計書 3.1.5・3.8）。

#### ステップ 1-6: 確かめ直しのスクリプトのテスト

**変更するファイル:** `cmd/yt2column/lightweight_docs_test.go`

-   [ ] `TestLightweightCheck` を書く。`t.TempDir()` に fixture のリポジトリを作り、`origin/main` に当たる参照（`refs/remotes/origin/main`）に、実際の `requirements_process.md` を置く。道具のコミットの場合だけ、`tool` の行をテストの中で用意した決まった出力のコマンドにした条件の表を置く。スクリプトはリポジトリの作業ツリーの版を `PATH` の `bash` で実行する。
-   [ ] git とスクリプトは、継いだ `GIT_*` の環境変数を外し、HOME と git のグローバルの設定をテストのものにして走らせる。ネットワークは使わない（1.3 節）。
-   [ ] 設計書 7.1 の表のすべての場合を確かめる。最後の行と終了コードを確かめ、`FAIL` の場合は超えた値の行も確かめる。上限の場合は、上限を実際の条件の表から読み、上限ちょうど（`PASS`）と 1 つ超えた値（`FAIL`）の両方を置く（1.3 節）。
-   [ ] 設計書 7.1 の表にないが、設計書 3.1.2・3.3.1・3.5.2 が定める次の場合も確かめる：利用者の git の設定（diff のアルゴリズム、外部の diff、textconv、色、`core.quotePath`）を敵対的にしても判定が変わらない、1 つのコミットに trailer が 2 行ある（`STOP`）、仕様の本文が CRLF でも類型を読める、`docs/tasks/` の下のファイルを足す（`FAIL excluded`、AC-21）。
-   [ ] 壊して確かめる：設計書 7.3 の 7 つ（上限の比較、除外するパスの照合、`--no-renames`、数えられない変更の判定、`origin/main` の版の条件の表を読むこと、再現の確認、許可リストとの語の単位の照合と残りの語のパスの確かめ）を 1 つずつ外し、対応する行が失敗することを確かめ、コミットメッセージに書く。

**完了の条件（ステップ 1-5・1-6）:** `make fmt`・`make test`・`make lint`・`make deadcode` が通る。

### PR-2 作成ポイント: lightweight check script

**対象ステップ**: 1-5 / 1-6

**推奨タイトル**: `feat(0061): 軽量プロセスの条件を実際の diff で確かめ直すスクリプトとテストを加える`

**レビュー観点**: スクリプトが判定に使う値（BASE・類型・道具のコミットの範囲・条件の表）をエージェントから受け取らず、`origin/main` の版の条件の表を読むこと / `FAIL` を diff についての事実だけに使い、手順の誤りや道具の失敗を `STOP` にしていること / 道具のコミットの再現が shell を通さず、許可リストと語の単位で照合していること / テストが設計書 7.1 のすべての場合と 7.3 の壊し方を確かめていること、CLAUDE.md の例外の範囲が狭いこと

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-5 のスクリプトは、軽量プロセスの条件を機械的に守る信頼境界（mkplan.md の panel mode の security-gate の条件）である

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `/runissue` と Part B（設計書 8 章の 2）

#### ステップ 2-1: Part B の変更

**変更するファイル:** `.claude/commands/_lib/task-pr-handoff.md`

-   [ ] 設計書 3.6 のとおりに変える。冒頭の説明に `/runissue` が Part B の手順 3〜7 を使うことを足し、入力 ISSUE_KEYWORD（`refs` か `closes`、省けば `refs`、ほかの値なら止まる）を足す。TASK_DIR・PR_TEXT・ISSUE・REPORT・AFTER_CREATE・CHECKS・LOCAL_REVIEW の説明に `/runissue` の場合を足し、「Authorization」の「for its task and its PR only」を「for its task or issue, and its PR only」にする。
-   [ ] 手順 3 の `Refs #<issue>` の項目（243〜246 行）を、ISSUE_KEYWORD で `Refs` か `Closes` を選ぶ形にする。番号付きの手順と、手順 3・5 の書き出しの文は変えない（1.4 節の `itemSection` が探す）。

#### ステップ 2-2: ローカルの CLI レビューの呼び出し元

**変更するファイル:** `.claude/commands/_lib/local-cli-review.md`

-   [ ] 呼び出し元の一覧（11〜20 行）に `runissue.md` を足し、冒頭の数（「Five callers」）を一覧の数に合わせる。

#### ステップ 2-3: `/runissue`

**変更するファイル:** `.claude/commands/runissue.md`（新規。英語）、`.claude/commands/_context.md`（「Reviewer ping」の行）、`docs/dev/conventions/requirements_process.md`（条件の表の `exclude` の 1 行）

-   [ ] 設計書 3.5 のとおりに書く。手順は、入口の確認（3.5.1）、ブランチの作成とコミットの並び（3.5.2）、CHECKS（3.5.3。CLAUDE.md を参照し、書き写さない）、サブエージェントによるレビュー（3.5.4。`review-subagent-pattern.md` との違いを明記する）、ローカルの CLI レビューと README の同期（3.5.5）、確かめ直しと Part B の直前の確認（3.5.6）、確かめた結果の記録（3.5.8）、通常のプロセスへの戻し（3.5.7）、Part B の手順 3〜7 の呼び出し（ISSUE_KEYWORD は `closes`）である。全体の流れは設計書 6.1 に合わせる。
-   [ ] 値（ラベル名、目印、ブランチ名、タイトルと件名の形）は `_context.md` の名前で、仕様の読み書きは仕様の手順書を名前で参照する（AC-39）。要件定義書の検索は `mkreq.md` の手順 1 を参照する。issue の本文とタイトルは取らない（設計書 1.2）。
-   [ ] 戻しのコメントの投稿の前に、「**Mention check.**」の段（`_lib/task-pr-handoff.md` と同じ文）と、1 行目が仕様の目印でないことの確かめを置く（AC-20・AC-30）。投稿のコマンドは `gh issue comment <N> --body-file <file>` の形にする。
-   [ ] 「Authorization」に、設計書が挙げる操作（`gh issue view`、`gh repo view`、`gh api user`、コメントの一覧と 1 件の読み取り、GraphQL の `closedByPullRequestsReferences` の読み取り、`git fetch origin`、`git ls-remote`、`gh issue develop`、`gh issue comment`、`gh issue edit --remove-label`、リース付きのブランチの削除、Part B の操作）を、その issue と PR に限って書く（AC-42）。
-   [ ] 条件の表に `exclude .claude/commands/runissue.md` を足す。`_context.md` の「Reviewer ping」の行に `runissue.md` を足す（1.3 節）。

#### ステップ 2-4: `/runissue` と仕様の手順書の guard

**変更するファイル:** `cmd/yt2column/codex_review_docs_test.go`、`cmd/yt2column/fixpr_docs_test.go`、`cmd/yt2column/lightweight_docs_test.go`、`internal/doctestutil/lightweight.go`、`CLAUDE.md`（591 行の jq の項目）

-   [ ] `postingSections` に `runissue.md` の戻しのコメントの区画を足す。区画は、戻しの手順の見出しか項目で探す。`runissue.md` の GraphQL の読み取りが `readOnlyInvocations` の前方一致に当たらなければ、その書き出しを足す。
-   [ ] `checkListings` の置き換えの表に `<spec-marker>`・`<revert-marker>` を足し、fake の応答の項目に `id` を足す（設計書 3.9、1.4 節）。既存の `TestRunprDocContract`・`TestHandoffListingPrintsNoBody` が通り続けることを確かめる。
-   [ ] `internal/doctestutil/lightweight.go` に見出し「## Listing own marker comments」の定数を足す。`TestLightweightSpecListing` を書き、仕様の手順書のその節を `checkListings` で確かめる。fake の項目は、利用者の仕様と戻しのコメント、他人の `User`・`Bot`・利用者と同じ名前の `Bot` の目印のコメント、目印が 2 行目にあるもの、1 行目の行末が CR のもの、本文が `null` のものを含める。目印の値は `_context.md` の行から読む（1.3 節）。`readCalls` には `gh api user --jq .login` と 1 件の本文の読み取りを置く。
-   [ ] `CLAUDE.md` の 591 行の jq の項目に、`TestLightweightSpecListing` を足す（1.3 節）。
-   [ ] 壊して確かめる：仕様の手順書のフィルタから書き手の条件を外す、`.user.type` の条件を外す、本文を出す。`runissue.md` のメンションの確認を投稿の後に移す。それぞれで対応するテストが失敗することを確かめ、コミットメッセージに書く（設計書 7.3）。

**完了の条件（ステップ 2-1〜2-4）:** `make fmt`・`make test`・`make lint`・`make deadcode` が通る。

### PR-3 作成ポイント: runissue command

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4

**推奨タイトル**: `feat(0061): 軽量プロセスの仕様を実装して PR を作る /runissue を加える`

**レビュー観点**: 入口の確認と Part B の直前の確認が、仕様・issue の状態・ラベルの変化を見逃さないこと / 「条件を外れた」（戻す）と「確かめられない」（issue を変えずに止まる）の分け方が設計書 4 章のとおりであること / 戻しの手順の順序（コメント→ラベル→リース付きのブランチの削除）と、途中の失敗の報告 / 一覧のフィルタのテストが書き手・種類・目印・CR の扱いと本文を出さないことを確かめていること

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-3 は GitHub への書き込みと取り消せない戻しを持つ新しいコマンドで、他人の書いた文字列を扱う信頼境界（mkplan.md の panel mode の security-gate の条件）である

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: `/runpr`（設計書 8 章の 3）

#### ステップ 3-1: 軽量プロセスの PR の判定と issue を閉じること

**変更するファイル:** `.claude/commands/runpr.md`

-   [ ] 手順 2 の「Kind」に Lightweight PR を足し、Standalone change を「それ以外」にする（設計書 3.7.1）。判定に使う GraphQL の読み取り（`headRefName`・`closingIssuesReferences`・各 `pageInfo.hasNextPage`）と、止まる場合を書く。PRE_MERGE と NEXT はない（AC-33）。
-   [ ] 手順 1 の単独の変更の PR の作成で、対象のブランチの名前が軽量プロセスのブランチ名の形なら、PR を作らずに止まる（設計書 3.7.2）。手順 1 の `MERGED` の場合に、軽量プロセスの PR なら手順 5 の issue を閉じる項目を行うことを足す（AC-35）。
-   [ ] 手順 4 の 1 に、軽量プロセスの PR の squash のメッセージの本文の最後を `Closes #<LW_ISSUE>` とすることを足す（設計書 3.7.3）。
-   [ ] 手順 5 の issue を閉じる項目（809〜827 行）を、「閉じる issue」を持つ形に広げる（設計書 3.7.4）。既存の「**Mention check.**」の段と `gh issue close` のコマンドは、この項目の中に 1 つずつのまま残す。軽量プロセスの PR では、コメントの 1 行目が仕様の目印でないことも確かめる。タスクの最後のプランの PR の条件は変えない（AC-36）。
-   [ ] 「Authorization」に、設計書 3.7.4 の操作を足す（AC-44）。

#### ステップ 3-2: `/runpr` の guard

**変更するファイル:** `cmd/yt2column/codex_review_docs_test.go`

-   [ ] `runpr.md` の GraphQL の読み取りの書き出しを `readOnlyInvocations` に足す（設計書 2.2）。そのコードブロックに `mutation` を書かない。
-   [ ] 既存の `TestPostingPathsCheckMention`・`TestPostingCommandsInventory`・`TestRunprDocContract` が、変えた `runpr.md` で通ることを確かめる。

**完了の条件（ステップ 3-1・3-2）:** `make fmt`・`make test`・`make lint`・`make deadcode` が通る。

### PR-4 作成ポイント: runpr lightweight PR handling

**対象ステップ**: 3-1 / 3-2

**推奨タイトル**: `feat(0061): /runpr で軽量プロセスの PR を見分けてマージの後に issue を閉じる`

**レビュー観点**: 軽量プロセスの PR の判定が本文を読まず、GitHub の構造化された値だけで決まり、決められないときに止まること / 軽量プロセスのブランチから単独の PR を作らないこと / `MERGED` の場合の実行し直しで issue が閉じること / タスクの PR と単独の変更の扱いが変わっていないこと

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-1 は手順 2 の種類の分岐と手順 5 の issue を閉じる処理を変え、既存のタスクの PR の振る舞いを保つ必要がある（隔離すべき分岐の変更）

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 入口と周辺（設計書 8 章の 4）

#### ステップ 4-1: `/mkreq` の振り分け

**変更するファイル:** `.claude/commands/mkreq.md`

-   [ ] 手順 1 と手順 2 の間に、振り分けの手順（たとえば「1.5.」）を足す（設計書 3.4.1・3.4.2）。戻した issue の判定、AC-15 の 3 つの停止、`[task]` の扱い、条件に当たるかの判断を、設計書 3.4.2 の順に書く。仕様のコメントの読み書きは仕様の手順書を参照する。
-   [ ] 問いの形（設計書 3.4.3）と、投稿とラベルの付与（設計書 3.4.4）を書く。投稿の前に「**Mention check.**」の段を置き、投稿のコマンドは `gh issue comment <N> --body-file <file>` の形にする。
-   [ ] front matter の `description`、目的の段落（12〜17 行）、手順 9 の報告、「Authorization」（設計書 3.4.4 の操作、AC-43）を改める。

#### ステップ 4-2: `/mkreq` の guard

**変更するファイル:** `cmd/yt2column/codex_review_docs_test.go`

-   [ ] `postingSections` に `mkreq.md` の振り分けの区画を足す。区画は `itemSection` で、振り分けの項目の書き出しから手順 2 の書き出しまでとする（1.4 節）。
-   [ ] 壊して確かめる：振り分けの区画で、メンションの確認を投稿の後に移す。`TestPostingCommandsInventory` が失敗することを確かめ、コミットメッセージに書く。

#### ステップ 4-3: CLAUDE.md と AGENTS.md

**変更するファイル:** `CLAUDE.md`、`AGENTS.md`

-   [ ] `CLAUDE.md` を設計書 3.8 のとおりに変える：33 行の例外と範囲、「Claude Commands」の一覧への `/runissue <N>` の追加と `/mkreq` の行の振り分け、166 行の段落、「Requirements and Acceptance Criteria」の概要と `requirements_process.md` の節へのリンク（AC-37・AC-38）。
-   [ ] `AGENTS.md` の 11 行の規則に、軽量プロセスの例外を足す（設計書 3.1.5・3.8）。

#### ステップ 4-4: issue テンプレート

**変更するファイル:** `.github/ISSUE_TEMPLATE/lightweight.md`（新規。日本語）

-   [ ] 設計書 3.8 のとおりに書く。front matter に `labels` を書かない。本文は背景・やること・やらないこと・確かめ方の見出しと、冒頭の `/mkreq <N>` の 1 行である（AC-40）。

#### ステップ 4-5: `lightweight` ラベル

**変更するファイル:** なし（GitHub のラベル）。記録は本書 7.1 節。

-   [ ] `gh label list` でラベルがないことを確かめ、利用者の承認を得てから、`gh label create` で色 `c2e0c6` と設計書 1.2 の説明で作る（AC-41）。`gh label list` の出力を 7.1 節に書く。

**完了の条件（ステップ 4-1〜4-5）:** `make fmt`・`make test`・`make lint`・`make deadcode` が通る。ラベルがリポジトリにある。

### PR-5 作成ポイント: mkreq triage and documents

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5

**推奨タイトル**: `feat(0061): /mkreq で軽量プロセスへ振り分け、CLAUDE.md・AGENTS.md・issue テンプレートを改める`

**レビュー観点**: 振り分けの判定の順序と、AC-15 の 3 つの停止と戻した issue の扱い / 承認が示した全文についての選択に限られ、投稿する文面が示した全文とバイト単位で同じであること / 投稿・ラベルの付与の失敗の報告 / CLAUDE.md・AGENTS.md の例外の範囲が F-001 の条件と承認された仕様に限られていること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 4-1 は利用者の承認と GitHub への投稿を持ち、ほかの 4 つのステップと分けて読むべき分岐を持つ（隔離すべき手順の変更）

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: 規則と値 | 軽量プロセスの節と条件の表、`_context.md` の値、仕様の手順書、除外するパスの実在のテスト | PR-1 がマージされた |
| M2: 確かめ直し | 確かめ直しのスクリプトとテスト、CLAUDE.md のリテラルの例外 | PR-2 がマージされた |
| M3: `/runissue` | `runissue.md`、Part B と `local-cli-review.md` の変更、投稿の区画と一覧のフィルタのテスト | PR-3 がマージされた |
| M4: `/runpr` | 軽量プロセスの PR の判定、issue を閉じること | PR-4 がマージされた |
| M5: 入口と周辺 | `/mkreq` の振り分け、CLAUDE.md・AGENTS.md、issue テンプレート、ラベル | PR-5 がマージされた |

PR は 1→5 の順にマージする。軽量プロセスを使えるようになるのは M5 の後である。M5 より前は、`/mkreq` が振り分けず、ラベルもないので、仕様を投稿した issue は生まれない（設計書 8 章）。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 | 軽量プロセスの節と条件の表、`_context.md` の値、仕様の手順書、除外するパスの実在のテスト | frontier-recommended |
| PR-2 | 1-5 / 1-6 | 確かめ直しのスクリプトとテスト、CLAUDE.md のリテラルの例外 | frontier-required |
| PR-3 | 2-1 / 2-2 / 2-3 / 2-4 | `/runissue`、Part B と `local-cli-review.md`、投稿の区画と一覧のフィルタのテスト | frontier-required |
| PR-4 | 3-1 / 3-2 | `/runpr` の判定と issue を閉じること | frontier-recommended |
| PR-5 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 | `/mkreq` の振り分け、CLAUDE.md・AGENTS.md、issue テンプレート、ラベル | frontier-recommended |

## 4. テスト戦略 (Test Strategy)

設計書 7 章のとおりである。どの AC をどの確かめで確かめるかは本書 7 章に示す。

-   **単体テスト:** 確かめ直しのスクリプトを fixture のリポジトリで実行する `TestLightweightCheck`（ステップ 1-6）。設計書 7.1 のすべての場合と、ステップ 1-6 に挙げた追加の場合を確かめる。
-   **文書の値と構造のテスト:** 除外するパスの実在（`TestLightweightExcludedPaths`、ステップ 1-4）、一覧のフィルタ（`TestLightweightSpecListing`、ステップ 2-4）、投稿の区画のメンションの確認（既存の `TestPostingPathsCheckMention`・`TestPostingCommandsInventory` に区画を足す。ステップ 2-4・3-2・4-2）。
-   **テストを足さないもの:** ラベル名、CLAUDE.md のコマンドの一覧、テンプレートの見出し、仕様のコメントの形の確かめ方、`_context.md` の新しい値（設計書 3.9 の理由のとおり）。条件の表の上限の値も、テストに写さない（1.3 節）。
-   **結合テスト:** コマンドの流れ（`/mkreq` から `/runpr` まで）は自動のテストにしない。PR-5 のマージの後、最初の実際の軽量プロセスの issue で、利用者と一緒に流れを確かめる（設計書 7.2、9 章）。
-   **後方互換:** 既存のテスト（`TestRunprDocContract`・`TestHandoffListingPrintsNoBody`・`TestPostingCommandsInventory` など）を変えた文書で通すことを、各 PR のグリーンゲートで確かめる。
-   **壊して確かめる:** ステップ 1-4・1-6・2-4・4-2 の壊し方を実装のときに行い、コミットメッセージに書く（設計書 7.3）。
-   **読んで確かめる:** 文書の文言だけを定める AC は、PR のレビューで読む（設計書 7.4）。
-   **カバレッジ:** 数値の目標は置かない。本タスクはテストを削除しないので、`make coverage-compare` は使わない。
-   **セキュリティ:** 設計書 7.5 のとおり、メンションの確認は投稿の区画のテスト、書き手で絞ることと本文を出さないことは一覧のフィルタのテスト、除外・数えられない変更・条件の表の独立性・道具のコミットは確かめ直しのスクリプトのテストで確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| スクリプトが macOS の bash 3.2 で動かない | 利用者の環境で `/runissue` が `STOP` する | bash 4.0 以降の機能を使わない（1.3 節）。PR-2 のレビュー観点で読む |
| スクリプトのテストが、pre-commit の hook の中で継いだ `GIT_*` の環境変数で別のリポジトリを操作する | テストが誤って通るか、作業中のリポジトリを壊す | 継いだ `GIT_*` を外して走らせる（ステップ 1-6） |
| 再現の確認の一時的な worktree が残る | 次の実行や `git worktree list` が汚れる | `trap` で必ず消す（ステップ 1-5）。テストでも、実行の後に worktree が残らないことを確かめる |
| `checkListings` の拡張が既存の一覧のテストを壊す | `TestRunprDocContract` などが失敗する | 置き換えの印と `id` を足すだけにし、ステップ 2-4 で既存のテストが通ることを確かめる |
| 条件の表の除外するパスに漏れがある | 信頼境界の変更が軽量プロセスで進む | 設計書 3.1.3 の表と照らして PR-1 でレビューする。新しい信頼境界を足す PR が `exclude` を足す規則を書く（ステップ 1-1） |
| PR-1〜PR-4 のマージの後、PR-5 の前に、利用者が手でラベルと仕様のコメントを付ける | 途中の状態で `/runissue` が動く | ラベルは PR-5 のステップ 4-5 で作るので、それまでは付けられない |
| 設計書を `draft` に戻した決定の変更が、本 PR のレビューで認められない | 計画のリテラルの例外が設計書と食い違う | 本 PR の `/runpr` の問いで設計書の再承認を受ける。認められないときは、1.3 節の決定とステップ 1-5 の例外を改める |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4）
- [ ] PR-2 マージ済み（対象ステップ: 1-5 / 1-6）
- [ ] PR-3 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4）
- [ ] PR-4 マージ済み（対象ステップ: 3-1 / 3-2）
- [ ] PR-5 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別は `test`（`make test` で実行するテスト）、`manual`（PR のレビューで読む、または GitHub の状態を確かめて 7.1 節に記録する）である。本タスクの AC に、前後の関係を述べるものはないので、`diff` は使わない。テストの場所はすべて `cmd/yt2column/` の下である。

| AC | 種別 | 確かめ | ステップ |
|---|---|---|---|
| AC-01 | manual | PR-1 のレビューで、3 つの類型と確かめ方の文を読む | 1-1 |
| AC-02 | test・manual | test: `lightweight_docs_test.go::TestLightweightCheck`（類型ごとの上限ちょうどと 1 つ超えた値、類型 M の削除行、道具のコミットの行を数えないこと）。manual: 条件の表の値が AC-02 と一致すること | 1-1・1-6 |
| AC-03 | test・manual | test: `TestLightweightCheck`（ファイル数の上限、道具が生成した変更だけのファイルを数えないこと）。manual: 条件の表の値と、`README.en.md` を数える文 | 1-1・1-6 |
| AC-04 | manual | PR-1 のレビューで、ちょうど 1 つの類型に収まる規則の文を読む | 1-1 |
| AC-05 | test・manual | test: `lightweight_docs_test.go::TestLightweightExcludedPaths`、`TestLightweightCheck`（除外の各行）。manual: 除外する変更の文と、条件の表のパスが設計書 3.1.3 の表と一致すること | 1-1・1-4・1-6 |
| AC-06 | test・manual | test: `TestLightweightExcludedPaths` と `TestLightweightCheck` が、見出し「#### Mechanically checked conditions」の直後のコードブロックを読む。manual: 機械的に確かめる条件とレビューで確かめる条件の区別の文 | 1-1・1-4・1-6 |
| AC-07 | manual | PR-1 のレビューで、「Review Flow」の軽量プロセスの流れを読む | 1-1 |
| AC-08 | manual | PR-5 のレビューで、`mkreq.md` の振り分けの位置と条件を読む | 4-1 |
| AC-09 | manual | 同上（問いの前の説明と選択肢、問い直し） | 4-1 |
| AC-10 | manual | 同上（通常のプロセスで進む場合） | 4-1 |
| AC-11 | manual | 同上（`--body-file` で示したファイルを投稿し、その後にラベルを付けること） | 4-1 |
| AC-12 | test | `codex_review_docs_test.go::TestPostingPathsCheckMention`、`::TestPostingCommandsInventory`（`mkreq.md` の振り分けの区画） | 4-2 |
| AC-13 | manual | PR-5 のレビューで、手順 9 の報告を読む | 4-1 |
| AC-14 | manual | 同上（投稿とラベルの付与の失敗の報告） | 4-1 |
| AC-15 | manual | 同上（3 つの停止と案内） | 4-1 |
| AC-16 | manual | PR-3 のレビューで、`runissue.md` の入口の確認を読む | 2-3 |
| AC-17 | test・manual | test: `lightweight_docs_test.go::TestLightweightSpecListing`（書き手・書き手の種類・目印・CR の扱い）。manual: 最新の 1 件を選ぶ規則と戻した issue の扱いの文 | 1-3・2-4 |
| AC-18 | test・manual | test: `TestLightweightSpecListing`（本文を出さないこと）。manual: `runissue.md` が issue の本文とタイトルを読まないこと | 2-3・2-4 |
| AC-19 | test・manual | test: `TestLightweightCheck`（仕様の本文の類型を読めないときの `STOP`）。manual: 仕様の手順書の形の確かめ方の文 | 1-3・1-6 |
| AC-20 | manual | PR-3・PR-4 のレビューで、投稿の前に 1 行目が仕様の目印でないことを確かめる文を読む | 2-3・3-1 |
| AC-21 | test・manual | test: `TestLightweightCheck`（`docs/tasks/` の下のファイルで `FAIL excluded`）。manual: `runissue.md` のブランチの作成の文 | 1-6・2-3 |
| AC-22 | manual | PR-3 のレビューで、CHECKS と README の同期の文を読む | 2-3 |
| AC-23 | manual | 同上（テストの削除のコミットとカバレッジの比較の扱い） | 2-3 |
| AC-24 | manual | 同上（サブエージェントによるレビュー） | 2-3 |
| AC-25 | manual | PR-3 のレビューで、`runissue.md` と `local-cli-review.md` の呼び出し元の一覧を読む | 2-2・2-3 |
| AC-26 | test・manual | test: `TestLightweightCheck`（出力の `spec-sha256` が仕様のファイルの SHA-256 と一致すること、行数とファイル数の行）。manual: 確かめた結果の記録の書式の文 | 1-6・2-3 |
| AC-27 | test・manual | test: `TestPostingPathsCheckMention`（既存の Part B 手順 3 の区画）。manual: PR の本文の項目と ISSUE_KEYWORD の文 | 2-1・2-3 |
| AC-28 | manual | PR-3 のレビューで、Part B の手順 5〜7 を使うことを読む | 2-1・2-3 |
| AC-29 | test | `TestLightweightCheck`（設計書 7.1 のすべての場合） | 1-6 |
| AC-30 | test・manual | test: `TestPostingPathsCheckMention`、`TestPostingCommandsInventory`（`runissue.md` の戻しの区画）。manual: 戻しの手順の順序と失敗の扱い | 2-3・2-4 |
| AC-31 | manual | PR-3 のレビューで、判断が要ったときの戻しの文を読む | 2-3 |
| AC-32 | test・manual | test: `TestLightweightSpecListing`（戻しのコメントを `revert` の種類で出すこと）。manual: 戻した issue で振り分けない文（`mkreq.md`）と、仕様とみなさない文（仕様の手順書） | 1-3・2-4・4-1 |
| AC-33 | manual | PR-4 のレビューで、「Kind」と NEXT の文を読む | 3-1 |
| AC-34 | test・manual | test: `TestPostingPathsCheckMention`（既存の `runpr.md` 手順 5 の区画）。manual: 閉じる issue の文 | 3-1・3-2 |
| AC-35 | manual | PR-4 のレビューで、手順 1 の `MERGED` の文を読む | 3-1 |
| AC-36 | manual | PR-4 のレビューで、手順 5 の差分を読み、タスクの最後のプランの PR の条件が変わっていないことを確かめる | 3-1 |
| AC-37 | manual | PR-5 のレビューで、CLAUDE.md の概要・リンク・コマンドの一覧・段落を読む。リンクの実在は `docpaths_test.go::TestInstructionDocPaths`（既存） | 4-3 |
| AC-38 | manual | PR-1 のレビューで「LLM Constraints」を、PR-5 のレビューで CLAUDE.md の 33 行を読む | 1-1・4-3 |
| AC-39 | manual | PR-1 のレビューで `_context.md` の値を、PR-3〜PR-5 のレビューでコマンドの定義が名前で参照することを読む（テストを足さない理由は設計書 3.9） | 1-2・2-3・3-1・4-1 |
| AC-40 | manual | PR-5 のレビューで、テンプレートの見出しと front matter を読む（同上） | 4-4 |
| AC-41 | manual | ステップ 4-5 で `gh label list` を実行し、出力を 7.1 節に書く | 4-5 |
| AC-42 | manual | PR-3 のレビューで、`runissue.md` が英語で、手順と「Authorization」を持つことを読む | 2-3 |
| AC-43 | manual | PR-5 のレビューで、`mkreq.md` の手順と「Authorization」を読む | 4-1 |
| AC-44 | manual | PR-4 のレビューで、`runpr.md` の「Authorization」を読む | 3-1 |

### 7.1. 実行の記録

（ステップ 4-5 で、`gh label list` の出力と実行した日を書く。）

## 8. 成功基準 (Success Criteria)

-   **機能の完成度:** 7 章のすべての行の確かめが済み、7.1 節にラベルの記録がある。
-   **品質:** 各 PR でグリーンゲート（`make test && make lint`）・`make fmt`・`make deadcode` が通る。加えたテストは、対象を壊して失敗することをコミットメッセージに記録している。
-   **セキュリティ:** 確かめ直しのスクリプトのテストが、除外・数えられない変更・条件の表の独立性・道具のコミットの場合を確かめ、一覧のフィルタのテストが書き手で絞ることと本文を出さないことを確かめ、投稿の区画のテストがメンションの確認を確かめる。
-   **文書:** `requirements_process.md`・`_context.md`・CLAUDE.md・AGENTS.md・コマンドの定義が軽量プロセスを定め、issue テンプレートがある。

## 9. 次のステップ (Next Steps)

-   PR-5 のマージの後、`/runpr` が issue #619 を閉じる案内をする（本タスクの最後の PR のため）。
-   最初の実際の軽量プロセスの issue で、`/mkreq` の振り分けから `/runpr` のマージまでを利用者と一緒に確かめる（設計書 7.2）。
