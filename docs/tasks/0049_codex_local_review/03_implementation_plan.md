# 実装計画書：CodeRabbit CLI が失敗したときに Codex CLI でローカルの CLI レビューを行う

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-29 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的と原則

[要件定義書](01_requirements.md)（以下、要件書）の AC-03〜AC-18 を、[アーキテクチャ設計書](02_architecture.md)（以下、設計書）に従って実装する。AC-01・AC-02 は、観測の記録（[observations.md](observations.md)、PR #555）と設計書 1.2 で満たした。本書の用語（共通手順、呼び出し元、`codexreview`、ツールの結果、最終メッセージ、試行の結果、記録の行）は設計書冒頭の定義に、要件書 6 節の用語（試行、代替、異系統のレビュー、範囲など）は要件書に従う。

-   **ツールを先に、文書を後に入れる（設計書 8 章）。** `codexreview` は `main` からインストールして使うので、ツールの PR がマージされ、利用者が `make install-codexreview` で入れてから、共通手順がツールを参照する。コマンドの文書もエージェントも、`make install-codexreview` と `go build` を実行しない（設計書 3.8）。
-   **単体テストは `codex` も CodeRabbit CLI も実行しない（`CLAUDE.md`「Testing Strategy」）。** 偽の `codex` を使う。本物の `codex` は割り当てを使うので、テストは、本物の `codex` が `PATH` から見つかる環境を作らない（ステップ 2-2）。
-   Go のコメント・識別子・文字列は英語とする。受け入れ基準との対応は本書 9 章に記録し、ソースのコメントに `AC-NN` を書かない（`requirements_process.md` §4）。
-   テストを加えたら、そのテストが確かめる仕組みを壊して失敗することを確かめ、そのことをコミットメッセージに書く（`CLAUDE.md`「Every test must be able to fail for its stated reason」）。
-   `.pre-commit-config.yaml` のフックは、Go のファイルを含むコミットのたびに `go test -tags test ./...` を実行する。テストと実装は、通る状態で 1 つのコミットにする。

### 1.2. 計画の際に決めたこと（2026-10-29）

利用者に確かめて決めたこと:

-   **`GitEnv` と git の設定の上書き（`gitHardening`）は、`internal/fixprguard` から `internal/cmdrunner` に移して共有する。** 設計書 3.3 は共有する場所を実装計画に委ねている。設計書 2.2 は「`codexreview` は `internal/cmdrunner` を共有するだけで、`fixprguard` と `mergepr` に依存しない」としている。`internal/cmdrunner` に置けば、この記述とそのまま合う。新しいパッケージ（`internal/gitenv` など）を作る案は、設計書 2.2 の表にないパッケージが増えるので採らなかった。
-   **偽の `codex` は、テストのバイナリそのものとする。** テストのバイナリを `codex` という名前の symlink にして、テストが作る `PATH` に置く。起動されたテストのバイナリは、環境変数で偽の `codex` として動く（Go の helper process の形）。こうすれば、`PATH` の探索、実際のプロセスグループの停止、終了コードを、本物と同じ経路で確かめられる。`sh` のスクリプトは場面ごとにテストの中に散らばるので採らなかった。`internal/cmdrunner/testutil` の `Fake` は、`codexreview` の段階でのプロセスの停止を確かめられないので採らなかった。
-   **PR は 5 つに分ける（3.2）。** 文法の判定と状態の比較（PR-2）を、権限と文書のガード（PR-3）と分けて読めるようにする。ツールと文書の間には、利用者のインストール（フェーズ 4）が入る。

本書で決めたこと:

-   **`codexreview` の終了コードは、`findings`・`no_findings` で 0、`state_changed` で 3、ほかの種類で 1 とする（設計書 3.3「具体的な数値は実装計画で決める」）。** 2 は、Go の実行時のパニックと `flag` パッケージの既定の終了コードなので使わない。どちらのときも標準出力に結果がないので、共通手順は「結果がない」として止まる（設計書 4 章）。126・127 はシェルが使うので使わない。
-   **`codexreview` 自身の失敗の扱い（設計書 4 章「ツールの中のエラーの型は実装計画で決める」）。** `codex` を起動する前の失敗（git の読み取り、`<RUN_DIR>` の作成）は、種類 `precondition` で返す。`codex` が動いていないので、記録して先へ進んでよい。`codex` を起動した後に、実行の後の状態を読めないときは、結果を書かずに 1 で終わる。共通手順は「結果がない」として止まる。作業ツリーが変わったかどうかを確かめられないからである（要件書 AC-09）。
-   **`Protocol` は 1 から始め、`internal/codexreview` の定数とする。** 設計書 3.3 の `Detail` の値（`bad_arguments`・`protocol_mismatch`・`not_worktree_root`・`base_not_ancestor`・`dirty_tree`・`project_codex_config`・`exit_status_<n>`）はそのまま使う。ほかの値（`incomplete` と `output_too_large` と `unreadable` の原因など）は実装で決め、`internal/codexreview` の一覧の関数で列挙する。共通手順が振る舞いを選ぶのに使う `Detail` は、`dirty_tree` だけである。
-   **設計書 3.9 の記述のうち、テストや方針の上で前提になるものは、それを必要とするフェーズで書く。** フェーズの順は設計書 8 章のままとし、項目だけを前のフェーズに移す。
    -   `package_reference.md` の行は、パッケージを加えるか変えるステップで書く（フェーズ 1・2）。`TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:671`）が、パッケージと同じコミットで行を求めるためである。
    -   `CLAUDE.md`「Declare, don't infer」の例外は、最終メッセージを解析するステップ 2-1 で書く。`CLAUDE.md`「Testing Strategy」の例外と Commands の節の `make install-codexreview` の行は、文書のガードを加えるステップ 3-2 で書く。`ci.yml` の `doc-guard` と、`CLAUDE.md` の Development Notes のその対象の列挙は、同じステップ 3-3 で直す。規則とそれに頼るコードを同じ PR で読めるようにするためである。
    -   `CLAUDE.md`「Tool Execution Safety」は、設計書 3.8 の項目なので、承認の記述と同じフェーズ 6 で書く。
-   **共通手順が `codexreview` を呼ぶことを求めるガードは、フェーズ 5 で加える。** フェーズ 3 のガードは、文書にある呼び出しの `--protocol` を確かめるだけにする。フェーズ 3 の時点では、共通手順はまだツールを呼ばないからである。
-   **`_lib/task-pr-handoff.md:40` の CHECKS と、`fixprguard_guide.md`・`fixprguard_guide.ja.md` の 454 行目にある文書のガードのコマンドには、`./cmd/codexreview` を加えない。** CHECKS は、タスクの文書（`docs/tasks/` の下）だけを変えるコマンドが push の前に実行するものである。`cmd/codexreview` のガードが読むファイル（コマンドの文書、`.claude/settings.json`、`Makefile`、`CLAUDE.md`）を、それらのコマンドは変えない。`fixprguard_guide` の行は、自動レビュアーの一覧を変えるときのガードを指す。CI の `doc-guard` の job には加える（ステップ 3-3）。
-   **設計書 2.2 の表を、編集上の訂正として補った（本 PR）。** 上の共有の場所の決定に伴う `fixprguard` の参照の付け替えと、`cmd/fixprguard/docs_test.go` の `TestDocumentedProtocolMatches` の変更（1.3）を加えた。決定は変えていないので、設計書の状態は `approved` のままとし、Comments に記録した。

### 1.3. 既存コード調査結果

コミット `0d7283e` で調べた。設計書が根拠にしたコミット `c76eec2` から、下に挙げるファイルは変わっていない（`git diff --stat c76eec2 0d7283e` に、これらのファイルはない）。

-   **`internal/cmdrunner`:** `osRunner.RunInput` は `exec.CommandContext` で直接の子だけを止め、プロセスグループは扱わない（`internal/cmdrunner/runner.go:89-93`）。時間切れのときは、コンテキストのエラーと `*ExitError` を `errors.Join` で返す（同 `:115-119`）。`MaxOutput` は標準出力と標準エラーの両方に効き、超えた分も読み捨てる（同 `:97-105`、`:130-150`）。既存のテストは `sh -c` で子のプロセスを作る（`internal/cmdrunner/runner_test.go:17`）。**足りないもの:** プロセスグループごと止めるオプション。**変えること:** オプションを加え、零値の振る舞いは変えない。`GitEnv` と git の設定の上書きを、ここに移す。
-   **`GitEnv` と `gitHardening` の使用箇所:** 定義は `internal/fixprguard/gitstate.go:66-110`。本番のコードの使用は `cmd/fixprguard/main.go:157`、`internal/fixprguard/gitstate.go:482`、`internal/fixprguard/ignored.go:272`。テストと補助の使用は `cmd/fixprguard/test_helpers.go:102,124,130`、`internal/fixprguard/test_helpers_git.go:173,185,187,251,252`、`internal/fixprguard/ignored_test.go:287,320`、`internal/fixprguard/gitstate_test.go:430-445`（`TestGitEnvDropsInheritedGitVariables`）。名前を挙げるコメントは `internal/fixprguard/gitstate.go:468,669`、`internal/fixprguard/test_helpers_git.go:77,246`、`internal/fixprguard/worktree.go:48`。文書では `docs/dev/developer_guide/package_reference.md:14`（`fixprguard.GitEnv`）だけである。`gitHardening` のコメント（`gitstate.go:66-84`）には、`CheckState` が設定を確かめるという `fixprguard` に固有の理由が含まれる。
-   **`internal/signalrun`:** `Run` は、最初のシグナルでコンテキストを取り消す（`internal/signalrun/signalrun.go:44-66`）。`cmd/codexreview` の `main` はこれを使う。コンテキストが取り消されたときに `codex` のプロセスグループを止めるのは、`internal/cmdrunner` の新しいオプションである。
-   **`fixprguard` の版の照合と文書のガード:** `Protocol` は `internal/fixprguard/types.go:21`（値 4）。`cmd/fixprguard/docs_test.go` の `TestDocumentedProtocolMatches`（`:328`）は、`.claude/commands` の下のすべての文書の `--protocol N`（`protocolFlag`、`:317`）を、`fixprguard` の `Protocol` と比べる。共通手順が `codexreview --protocol 1` を書くと、このテストが失敗する。**変えること:** `codexreview` の呼び出しの `--protocol` を、この照合から除く（ステップ 3-2）。`cmd/codexreview` のガードは、`TestDocumentedInvocationsParse`（`:187`）と同じく、文書の呼び出しを読む形にならう。
-   **`cmd/yt2column` の文書のガード:** `TestCodeRabbitConfigStopsReview`（`cmd/yt2column/codex_review_docs_test.go:351-356`）は、`.coderabbit.yaml` の文 ``The local CLI review (`coderabbit review`) is still used`` を求める。注釈を改めるので、この文も改める（ステップ 7-1）。`TestCodexMentionOnlyInRequestRow`（同 `:91`）は、`_context.md` の 1 行を除いて、コマンドの文書と `CLAUDE.md` にメンションがないことを確かめる。共通手順は、メンションの確認でもメンションそのものを書かず、`<review-mention>` と書く。`TestPostingCommandsInventory`（同 `:195`）の `postingCommand`（同 `:135`）は `gh` と `fixprguard reply`・`edit-pr` だけに当たり、`codexreview` と `git commit` には当たらない。変える必要はない。`cmd/yt2column/fixpr_docs_test.go` の見出しの定数（`:31-37`）に Phase 4.5 の見出しはないので、見出しを変えても影響はない。
-   **`TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:671`）:** `cmd/` と `internal/` の下の、テスト以外のソースを持つすべてのパッケージと、すべての `testutil` に、`package_reference.md` の行を求める。
-   **`Makefile`:** `install-mergepr`・`install-fixprguard`（`Makefile:72-79`）にならう。`.PHONY`（`Makefile:57`）にも加える。`TestMakeBuildBuildsEveryBinary`（`cmd/yt2column/makefile_test.go:479`）は `build` と `all` だけを見るので、影響はない。
-   **文書のガードのコマンドの列挙:** `.github/workflows/ci.yml:187`、`CLAUDE.md:446`、`.claude/commands/_lib/task-pr-handoff.md:40`、`docs/dev/developer_guide/fixprguard_guide.md:454`・`fixprguard_guide.ja.md:454` の 5 か所にある。これらが一致することを確かめるテストはない。直すのは前の 2 か所である（1.2）。
-   **コマンドの文書にある `codex` の語:** `.claude/commands/runpr.md:240-278`（jq の文字列と、コメントの種類の名前としてのコードスパン `codex`）、`:339`・`:341`・`:404`（同じ種類の名前）、`.claude/commands/_lib/task-pr-handoff.md:221,228`（HTML の印）、`.claude/commands/_context.md:185-187`（ログイン名とメンション）。どれも、`codex` をコマンドとして実行する形ではない。ステップ 3-2 のガードは、これらを拒まない。
-   **共通手順の今のファイル名を参照するもの:** `git grep -l coderabbit-local-review -- ':!docs/tasks'` の出力は、`.claude/commands/_lib/coderabbit-local-review.md`、`_lib/pr-review-escalation.md`、`_lib/task-pr-handoff.md`、`_lib/triage-pattern.md`、`fixpr.md`、`runplan.md`、`runpr.md` の 7 つだった（`0d7283e`）。テストはない。
-   **最終メッセージの実物:** 観測の記録には、形（観測の記録の 5、O1・O2）だけがあり、文面そのものはコミットされていない。テストの入力は、同じ形の架空の文面で作る。
-   **具体的な型による振り分け:** `Kind` と `Result` は JSON で出す値で、型による振り分けはない。`Result` を作るのは `internal/codexreview` だけである。`cmd/codexreview` が種類を終了コードに写す `switch` は、知らない種類を `default` で 1 にする。

### 1.4. 申し送りの扱い

`design_handoff.md` の H-01〜H-08 は、設計書の付録 A に記録されている。`implementation_handoff.md` はない。

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: `internal/cmdrunner` の変更（設計書 8 章の 1）

#### ステップ 1-1: `GitEnv` と git の設定の上書きを `internal/cmdrunner` に移す

**変えるファイル**: `internal/cmdrunner/`（新しいファイル `git.go` を想定）、`internal/cmdrunner/runner_test.go` か新しいテストのファイル、`internal/fixprguard/gitstate.go`・`gitstate_test.go`・`ignored.go`・`ignored_test.go`・`test_helpers_git.go`・`worktree.go`、`cmd/fixprguard/main.go`・`test_helpers.go`、`docs/dev/developer_guide/package_reference.md`

-   [ ] `GitEnv` と git の設定の上書きを `internal/cmdrunner` に移す。設定の上書きは、呼び出し元が書き換えられないように、新しいスライスを返す関数として公開する。
-   [ ] コメントは、git に共通の理由（別のリポジトリを見ない、コマンドを起動する設定を止める、git 2.12 以上が要る）を移した先に置く。`CheckState` が設定を確かめるという `fixprguard` に固有の理由は、`fixprguard` の使う場所に残す。
-   [ ] 1.3 の使用箇所と、名前を挙げるコメントを、すべて付け替える。`fixprguard` の振る舞いは変えない。
-   [ ] `TestGitEnvDropsInheritedGitVariables` を `internal/cmdrunner` に移す。
-   [ ] `package_reference.md` の `internal/cmdrunner`・`internal/fixprguard`・`cmd/fixprguard` の行を、移した後の名前に合わせる。

**完了の条件**: `make test` と `make lint` が通る。`internal/fixprguard` と `cmd/fixprguard` の既存のテストが、参照の付け替えのほかは変わらずに通る。

#### ステップ 1-2: プロセスグループごと止めるオプションを加える

**変えるファイル**: `internal/cmdrunner/runner.go`、`internal/cmdrunner/runner_test.go`、`docs/dev/developer_guide/package_reference.md`

-   [ ] `Options` に、子を自分のプロセスグループで起動し、コンテキストが取り消されたらグループごと止めるオプションを加える（設計書 3.3）。止め方は、まずグループに終了を求め、待っても残っていれば強制的に止める。待つ時間の上限をオプションで決められるようにする。零値のときは今と同じ振る舞いにする。
-   [ ] テストを加える。オプションを付けると、時間切れのときに孫のプロセスも止まる。終了の求めを無視する孫も、待つ時間の後に止まる。どちらも、孫のプロセス ID をファイルに書かせ、止まったことを確かめる。オプションがないときの振る舞いは、既存のテストのまま確かめる。
-   [ ] オプションを外す、グループに送らずに直接の子だけに送る、強制的に止める段を外す、の 3 つで、それぞれ新しいテストが失敗することを確かめる。
-   [ ] `package_reference.md` の `internal/cmdrunner` の行にオプションを加える。

**完了の条件**: `make test` と `make lint` が通る。

### フェーズ 2: `internal/codexreview` と `cmd/codexreview`（設計書 8 章の 2）

#### ステップ 2-1: 最終メッセージの文法と、指摘の位置の確かめ

**変えるファイル**: `internal/codexreview/`（新設。最終メッセージの解析のファイル）、そのテスト、`internal/codexreview/testdata/`、`CLAUDE.md`、`docs/dev/developer_guide/package_reference.md`

-   [ ] 最終メッセージを、設計書 3.4 の形と照らす関数を書く。結果は「指摘あり」「指摘なし」「読み取れない」のどれかで、「読み取れない」のときは指摘を 1 つも返さない。
-   [ ] 指摘の位置を確かめる関数を書く（設計書 3.4「位置の確かめ」）。シンボリックリンクを解いた形でルートと比べる。ルートの外を挙げる指摘があれば、最終メッセージ全体を「読み取れない」とする。`BASE..HEAD` で変わったファイルにあり、行の範囲がファイルの中にあるときだけ `InRange` を真にする。
-   [ ] テスト `TestParseFinalMessage` を書く。入力は、観測の記録の 5 と O1・O2 の形（見出しが `Full review comments:` で指摘 2 件、`Review comment:` で指摘 1 件、指摘なしの 1 段落）を架空の文面で作り、`testdata/` に置く。設計書 3.4 の「読み取れない」の各例と、「指摘なし」から除く 3 つの行の形も、それぞれ「読み取れない」になることを確かめる。
-   [ ] テスト `TestLocateFindings` を書く。ルートの外のパス、シンボリックリンクを経たルート、範囲で変わっていないファイル、ファイルの外の行、開始行が終了行より大きい行を扱う。
-   [ ] `CLAUDE.md`「Declare, don't infer」に、設計書 1.4 の例外（範囲、理由、境界）を、既存の例外と同じ形で加える（1.2）。
-   [ ] `package_reference.md` に `internal/codexreview` の行を加える。

**完了の条件**: `make test` と `make lint` が通る。文法の判定の各分岐を 1 つずつ外し、`TestParseFinalMessage` のどれかの行が失敗することを確かめる。

#### ステップ 2-2: 偽の `codex`

**変えるファイル**: `internal/codexreview/testutil/helpers.go`（新設、`package codexreviewtestutil`、`//go:build test`）、`docs/dev/developer_guide/package_reference.md`

-   [ ] テストのバイナリを偽の `codex` にする補助を書く（1.2）。テストの `TestMain` から呼ぶ入口と、テストが使う準備の関数を用意する。準備の関数は、`codex` という名前の symlink と、`git` への symlink だけを置いたディレクトリを作り、それだけを `PATH` にする。元の `PATH` を引き継がないので、本物の `codex` は見つからない。
-   [ ] 偽の `codex` の振る舞いを、設計書 7.1 の一覧のとおりに選べるようにする（引数の記録、終了コード、`turn.completed` を出さない、`error` の行を出す、何も書かない、上限を超えて書く、作業ツリーのファイルを変える、ブランチを作る、stash を作る、孫のプロセスを起動して終わらない）。`--version` にも答える。
-   [ ] `internal/codexreview` と `cmd/codexreview` の両方のテストから使うので、`testutil` に置く（`test_organization.md` の分類 A）。
-   [ ] `package_reference.md` に `internal/codexreview/testutil` の行を加える。

**完了の条件**: ステップ 2-3 のテストで使われ、`make test` が通る。

#### ステップ 2-3: `codex` の実行と、前後の比較

**変えるファイル**: `internal/codexreview/`（実行のファイル、`test_helpers.go`（`//go:build test`））、そのテスト

-   [ ] 設計書 3.3 のとおりに、実行の前の確かめ、前の状態の記録、固定した引数での `codex` の実行、完了の判定、最終メッセージの読み取りと判定、後の状態との比較、指示ファイルの変更の確かめ、ツールの結果の組み立てを書く。`Kind`・`Result`・`Finding`・`Protocol` は設計書 3.3 の形とする。
-   [ ] `codex` の実行には、ステップ 1-2 のオプションを付けた `internal/cmdrunner` を使う。環境は `GitEnv`、標準入力は空、標準出力の上限は 16 MiB、時間の上限は 480 秒とする。git の実行には、ステップ 1-1 の設定の上書きを付ける。`codex --version` にも短い時間の上限を付ける。
-   [ ] 時間の上限を短くする関数を `test_helpers.go` に置く。この差し替えは、テストのときだけビルドされ、引数や環境変数からは使えない（設計書 7.1）。
-   [ ] 1.2 の「`codexreview` 自身の失敗の扱い」のとおりに、`codex` の起動の前と後の失敗を分ける。
-   [ ] テストを書く。どれも、一時的な git のリポジトリと、ステップ 2-2 の偽の `codex` を使う。
    -   `TestRunPassesFixedArguments`: 偽の `codex` に渡る引数が、設計書 3.3 の固定した形と一致する。
    -   `TestRunPreconditions`: 設計書 3.3 の「実行の前の確かめ」の各行で、偽の `codex` が一度も起動されず、表の種類と `Detail` が返る。
    -   `TestRunKinds`: `findings`・`no_findings`・`codex_not_found`・`codex_failed`・`incomplete`・`output_too_large`・`output_empty`・`unreadable` が、それぞれの条件で返る。
    -   `TestRunStateChanged`: 作業ツリーのファイル、`HEAD`、ブランチ、タグ、stash のそれぞれを偽の `codex` が変えると、ほかの結果より優先して `state_changed` になり、`Changed` と `ChangedRefs`（worktree を含む）が違った項目を示す。
    -   `TestRunTimeoutStopsProcessGroup`・`TestRunCancelStopsProcessGroup`: 時間切れと、コンテキストの取り消しで、偽の `codex` が起動した孫のプロセスも止まる。
    -   `TestRunInstructionsChanged`: 範囲が下のディレクトリの `AGENTS.md` などを変えたときだけ、`InstructionsChanged` が真になる。
    -   `TestRunDirKeepsOnlyLastMessage`: `<RUN_DIR>` は所有者だけが読み書きでき、最終メッセージのほかに何も残らない。
    -   後の状態を読めないときに、結果を返さずに失敗する。
-   [ ] 各テストの対象を 1 つずつ外して（たとえば、比較の項目を 1 つ外す、引数を 1 つ外す、優先の順を変える）、対応するテストが失敗することを確かめる。

**完了の条件**: `make test` と `make lint` が通る。

#### ステップ 2-4: `cmd/codexreview` と `install-codexreview`

**変えるファイル**: `cmd/codexreview/main.go`（新設）、`cmd/codexreview/main_test.go`（新設）、`Makefile`、`docs/dev/developer_guide/package_reference.md`

-   [ ] `main` を `internal/signalrun` の `Run` の下で動かす。引数は `--protocol` と `--base` を 1 回ずつだけ受け付け、ほかのフラグと位置引数を拒む（設計書 3.3）。`--protocol` の不一致と `--base` の形の誤りも `usage` として返す。結果の JSON を標準出力に 1 つだけ書き、1.2 の終了コードで終わる。
-   [ ] テストを書く。
    -   `TestRunRejectsArguments`: 不正な引数、重なったフラグ、位置引数、`--protocol` の不一致、40 桁の 16 進数でない `--base` で `usage` になり、偽の `codex` が起動されない。
    -   `TestExitCodes`: すべての種類と、知らない種類について、終了コードが 1.2 のとおりになる。
    -   `TestOutputIsOneJSONObject`: 標準出力が 1 つの JSON のオブジェクトで、`protocol` が `Protocol` と一致する。
    -   `TestSignalStopsCodexProcessGroup`: テストの中でビルドした `codexreview` に SIGTERM を送ると、偽の `codex` の孫のプロセスも止まり、`codexreview` が終わる。ビルドの形は `cmd/yt2column-server/relay_e2e_test.go:190` にならう。
-   [ ] `Makefile` に `install-codexreview`（`go install ./cmd/codexreview`）を、`install-fixprguard` と同じ形のコメントとともに加え、`.PHONY` にも加える。
-   [ ] `package_reference.md` に `cmd/codexreview` の行を加える。

**完了の条件**: `make test` と `make lint` と `make deadcode` が通る。

### フェーズ 3: 権限、文書のガード、CI（設計書 8 章の 3）

#### ステップ 3-1: `.claude/settings.json` の許可

**変えるファイル**: `.claude/settings.json`

-   [ ] `permissions.allow` に `Bash(codexreview *)` を加える。`codex` そのものの許可は加えない（設計書 3.8）。

**完了の条件**: ステップ 3-2 の `TestSettingsAllowsTool` が通る。

#### ステップ 3-2: 文書のガード

**変えるファイル**: `cmd/codexreview/docs_test.go`（新設、`//go:build test`）、`cmd/fixprguard/docs_test.go`、`CLAUDE.md`

-   [ ] 設計書 7.1 の文書のガードを、`cmd/fixprguard/docs_test.go` の書き方にならって書く。
    -   `TestSettingsAllowsTool`: `.claude/settings.json` の `permissions.allow` に `Bash(<名前> *)` の項目があり、その名前が `cmd/` の下のこのツールのディレクトリの名前と一致する。
    -   `TestDocumentedProtocolMatches`: コマンドの文書の `codexreview` の呼び出しの `--protocol` が、`codexreview.Protocol` と一致する。
    -   `TestNoDirectCodexInvocation`: コマンドの文書のコードブロックと、コマンドとして書いたコードスパンに、コマンドの位置にある `codex` の語がない（`codexreview` を除く。`env codex` と引用符で囲んだ `"codex"` も対象とする）。1.3 の既存の `codex` の語は拒まない。
    -   `TestInstallTargetDocumented`: `Makefile` に `install-codexreview` のターゲットがあり、`CLAUDE.md` の Commands の節に同じ名前の項目がある。
-   [ ] 判定の関数を、組み立てた文書でも確かめる。コマンドの位置の `codex`（行頭、`|`・`;`・`&&`・`$(` の後、`env` の後、引用符で囲んだもの）を拒み、1.3 の形（jq の文字列、コメントの種類の名前、HTML の印、ログイン名）を拒まないことを、それぞれの行で確かめる。
-   [ ] `cmd/fixprguard/docs_test.go` の `TestDocumentedProtocolMatches` が、`codexreview` の呼び出しの `--protocol` を除いて、今と同じくすべての `--protocol N` を確かめるようにする（1.3）。`codexreview` の呼び出しを含む組み立てた文書で、除かれることと、`fixprguard` の呼び出しは今までどおり確かめられることを確かめる。
-   [ ] `CLAUDE.md` の Commands の節に `make install-codexreview` の項目を加える（設計書 3.9。1.2）。
-   [ ] `CLAUDE.md`「Testing Strategy」の文書のテストの例外に、`codexreview` の `--protocol` の値と、`codex` を直接実行する形がないことの 2 つを加える（設計書 1.4）。

**完了の条件**: `make test` と `make lint` が通る。各ガードを、文書の側（許可の名前を変える、`--protocol` の値を変える、コードブロックに `codex exec review` を書く、`CLAUDE.md` の項目を消す）で 1 つずつ壊して失敗することを確かめる。

#### ステップ 3-3: CI の `doc-guard` の job

**変えるファイル**: `.github/workflows/ci.yml`、`CLAUDE.md`

-   [ ] `doc-guard` の job の `go test` の対象に `./cmd/codexreview` を加える（設計書 7.1）。
-   [ ] `CLAUDE.md` の Development Notes の、`doc-guard` の対象の列挙を同じに直す。文書を読むガードの列挙にも、`cmd/codexreview` のガードを加える。

**完了の条件**: ローカルで `go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview ./internal/pipeline` が通る。PR の CI で `doc-guard` の job が通る。

### フェーズ 4: インストール（設計書 8 章の 4）

#### ステップ 4-1: 利用者が `codexreview` をインストールする

**変えるファイル**: なし

-   [ ] PR-3 のマージの後、利用者が最新の `main` で `make install-codexreview` を実行する。エージェントはインストールを実行しない（設計書 3.8）。
-   [ ] エージェントは、`command -v codexreview` でツールがあることと、`go version -m "$(command -v codexreview)"` の `vcs.revision` が PR-3 のマージのコミットか、それより後の `main` のコミットであることを確かめる。どちらかが成り立たなければ、利用者にインストールを頼んで止まる。

**完了の条件**: 上の 2 つが確かめられた。

### フェーズ 5: 共通手順の改名と書き換え（設計書 8 章の 5）

#### ステップ 5-1: 共通手順を `_lib/local-cli-review.md` にする

**変えるファイル**: `.claude/commands/_lib/coderabbit-local-review.md` → `.claude/commands/_lib/local-cli-review.md`、参照を持つ 6 つの文書（1.3）

-   [ ] `git mv` で名前を変え、設計書 3.1〜3.6 のとおりに書き換える。frontmatter の `description`、見出し、呼び出し元の列挙（5 か所）、入力、Authorization（設計書 3.8）、手順を含む。
-   [ ] 手順には次を書く。CodeRabbit CLI の成否の判定（設計書 3.2）。失敗したときの `codexreview --protocol 1 --base <BASE>` の実行（作業ツリーのルートで、`<BASE>` は `git rev-parse` で得た 40 桁の SHA、HEAD は実行の直前に控え、Bash ツールの時間の上限を 600 秒に指定してフォアグラウンドで実行する。設計書 3.1）。設計書 4 章の 3 つの扱い。両方の CLI の指摘のトリアージと回数（設計書 3.5）。記録の行と、それを入れるコミットのメッセージのメンションの確認（設計書 3.6）。前のセッションの `CodeRabbit local review: not run` の読み方。SUMMARY に書くこと（設計書 3.1）。
-   [ ] 文書には、`codex` を引数とともにコマンドの形で書かない（ステップ 3-2 のガードが拒む）。Codex CLI は名前で書く。メンションは `<review-mention>` と書き、メンションそのものを書かない。
-   [ ] 参照を持つ 6 つの文書のファイル名の参照を、新しい名前に付け替える（設計書 3.9。記述の書き換えはフェーズ 6・7）。

**完了の条件**: `make test` が通る。5 章の横断検索の 1 が空になる。

#### ステップ 5-2: 共通手順が `codexreview` を呼ぶことを求める

**変えるファイル**: `cmd/codexreview/docs_test.go`

-   [ ] `TestDocumentedProtocolMatches` が、`_lib/local-cli-review.md` に `codexreview` の呼び出しが 1 つ以上あることも求めるようにする（1.2）。

**完了の条件**: `make test` が通る。共通手順の呼び出しを消すと失敗することを確かめる。

### フェーズ 6: 呼び出し元と承認の記述（設計書 8 章の 6）

#### ステップ 6-1: `runplan.md`

**変えるファイル**: `.claude/commands/runplan.md`

-   [ ] 手順 4.5（`coderabbit` が `--base-commit` として受け取るという記述）、手順 7.7、手順 5a、手順 8 を、設計書 3.7 の表と 3.9 の表のとおりに改める。手順 5a は、`Local CLI review: not run` か `CodeRabbit local review: not run` の行があるときだけ、ブランチ全体を改めてレビューする（AC-15）。手順 8 の要約は、試行ごとの結果と Codex CLI の実行回数を示す（AC-11）。

#### ステップ 6-2: `_lib/task-pr-handoff.md`

**変えるファイル**: `.claude/commands/_lib/task-pr-handoff.md`

-   [ ] Part B 手順 1 の「local CodeRabbit CLI」の記述を改める。Part B 手順 3 が PR 本文に載せるものを、設計書 3.7「Part B 手順 3 が PR 本文に載せるもの」のとおりにする（AC-13・AC-15）。Authorization の `coderabbit review` の記述（`:65-67`）に `codexreview` を加える（AC-16）。

#### ステップ 6-3: `runpr.md` と `fixpr.md`

**変えるファイル**: `.claude/commands/runpr.md`、`.claude/commands/fixpr.md`

-   [ ] `runpr.md` の手順 1 で、異系統のレビューを受けていない範囲を最終報告に示す（AC-14）。PR 本文は変えない。承認の記述（`:82-83`）に `codexreview` を加える（AC-16）。
-   [ ] `fixpr.md` の Phase 4.5 の見出しと本文（割り当ての記述 `:584-586` を含む）を、ローカルの CLI レビューに改める。異系統のレビューを受けていない範囲を Final report に示す（AC-14）。承認の記述（`:97-98`）に `codexreview` を加える（AC-16）。

#### ステップ 6-4: `CLAUDE.md`「Tool Execution Safety」

**変えるファイル**: `CLAUDE.md`

-   [ ] `coderabbit review` の項目の後に、`codexreview` を承認なしに実行してよい項目を加える。その実行が `codex exec review` を起動し、差分と Codex が読んだ内容を OpenAI のサービスに送ること、CodeRabbit CLI が失敗したときの代替であることを述べる（設計書 3.8、AC-16）。

**フェーズ 6 の完了の条件**: `make test` と `make lint` が通る。

### フェーズ 7: 残りの記述（設計書 8 章の 7）

#### ステップ 7-1: `CLAUDE.md`、`.coderabbit.yaml`、`_lib/` の記述

**変えるファイル**: `CLAUDE.md`、`.coderabbit.yaml`、`.claude/commands/_lib/pr-review-escalation.md`、`.claude/commands/_lib/triage-pattern.md`、`cmd/yt2column/codex_review_docs_test.go`

-   [ ] 設計書 3.9 の表の残りの行を改める。`CLAUDE.md:101` と `:115-117`、`_lib/pr-review-escalation.md:10-12`、`_lib/triage-pattern.md:7`（呼び出し元の 5 か所の列挙）、`.coderabbit.yaml:9-12` である。割り当ての記述は、観測の記録の 1 の結論（PR レビューと同じ割り当てを消費する）と一致させる。
-   [ ] `TestCodeRabbitConfigStopsReview` が求める文を、改めた `.coderabbit.yaml` の注釈の文に合わせる（設計書 7.1）。

**完了の条件**: `make test` と `make lint` が通る。`.coderabbit.yaml` の新しい文を消すと `TestCodeRabbitConfigStopsReview` が失敗することを確かめる。

### フェーズ 8: 手動の確かめ（設計書 8 章の 8）

#### ステップ 8-1: 権限の確認で止まらないこと

**変えるファイル**: `docs/tasks/0049_codex_local_review/03_implementation_plan.md`（9 章の記録）

-   [ ] インストールした `codexreview` を、不正な引数を付けて Bash ツールで 1 回実行する。権限の確認で止まらず、`usage` が返ることを確かめる。`codex` は起動されない（設計書 7.2）。
-   [ ] `$(...)` やリダイレクトを含む形で実行しようとすると、権限の確認で止まることを確かめる。
-   [ ] 結果（実行した形、止まったかどうか、返った種類）を 9 章に記録する。

#### ステップ 8-2: 本物の `codex` での 1 回の実行

**変えるファイル**: `docs/tasks/0049_codex_local_review/03_implementation_plan.md`（9 章の記録）

-   [ ] 利用者の承認を得てから、このブランチで `codexreview` を本物の `codex` で 1 回実行し、`findings` か `no_findings` が返ることを確かめる。割り当てを 1 回分使う（設計書 7.2）。
-   [ ] 結果（日付、`codex` の版、種類、かかった時間、指摘の数）を 9 章に記録する。Codex の文面は写さない。

#### ステップ 8-3: 横断検索と完了の確認

**変えるファイル**: なし

-   [ ] 5 章の横断検索を行い、結果を確かめる。
-   [ ] グリーンゲート（`make test && make lint`）が通る。

**フェーズ 8 の完了の条件**: 9 章に 8-1 と 8-2 の記録がある。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序と前提

| 本書のフェーズ | 設計書 8 章の項目 |
|---|---|
| フェーズ 1 | 1. `internal/cmdrunner` のプロセスグループのオプション（あわせて、`GitEnv` と git の設定の上書きの移動） |
| フェーズ 2 | 2. `internal/codexreview` と `cmd/codexreview`、`Makefile` の `install-codexreview` |
| フェーズ 3 | 3. `.claude/settings.json` の許可と、文書のガード、`doc-guard` の job の対象 |
| フェーズ 4 | 4. ツールのマージの後、利用者が `make install-codexreview` で入れる |
| フェーズ 5 | 5. 共通手順の改名と書き換え、参照の付け替え |
| フェーズ 6 | 6. 呼び出し元の変更と承認の記述 |
| フェーズ 7 | 7. `CLAUDE.md`、`.coderabbit.yaml`、`_lib/pr-review-escalation.md`、`_lib/triage-pattern.md` の記述と `TestCodeRabbitConfigStopsReview` の文の更新 |
| フェーズ 8 | 8. 手動の確かめ |

-   1.2 のとおり、設計書 3.9 の項目のうち `package_reference.md` の行、`CLAUDE.md` の 2 つの例外、Commands の節、Development Notes の `doc-guard` の列挙は、前のフェーズで書く。
-   ステップ 2-3 は 1-1・1-2 を、2-4 は 2-3 を、3-2 は 2-4（`Protocol` と `Makefile` のターゲット）を前提にする。
-   フェーズ 5 は、フェーズ 4 のインストールを前提にする。ステップ 4-1 が確かめられなければ、フェーズ 5 に進まない。
-   ステップ 8-1・8-2 は、フェーズ 4 のインストールと、ステップ 3-1 の許可を前提にする。

### 3.2. マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（ツールのマージ） | プロセスグループのオプション、`codexreview`、`install-codexreview`、許可、文書のガード |
| M2（インストール） | 利用者の環境の `codexreview` |
| M3（文書のマージ） | 共通手順と 5 か所の呼び出し元の代替、承認と方針の記述、手動の確かめの記録。本タスクの完了 |

## 4. テスト戦略 (Test Strategy)

### 4.1. 検証の分担

| 層 | 確かめること | ステップ |
|---|---|---|
| `internal/cmdrunner` のテスト | プロセスグループごと止めること、零値の振る舞い、`GitEnv` | 1-1・1-2 |
| `internal/codexreview` の文法と位置のテスト | 受け入れる形と読み取れない形、位置の確かめ | 2-1 |
| `internal/codexreview` の実行のテスト | 固定した引数、実行の前の確かめ、各種類、前後の比較、時間切れと取り消しでの停止、指示ファイル、`<RUN_DIR>` | 2-3 |
| `cmd/codexreview` のテスト | 引数の拒否、終了コード、出力の形、シグナルでの停止 | 2-4 |
| 文書のガード（`cmd/codexreview`、`cmd/fixprguard`、`cmd/yt2column`） | 許可の名前、`--protocol` の値、`codex` を直接実行する形がないこと、インストールのターゲット、`.coderabbit.yaml` の文 | 3-2・5-2・7-1 |
| PR のレビュー（`manual`） | コマンドの文書の手順と記述 | 5-1・6-1〜6-4・7-1 |
| 手動の確かめ | 権限の確認、本物の `codex` での 1 回の実行 | 8-1・8-2 |

-   設計書 7.3 のセキュリティテストは、上の 2-1・2-3・2-4 のテストに含まれる（引数の拒否と固定、clean でないときと `.codex/` があるときの拒否、前後の比較、プロセスグループの停止、大きさの上限、文法の拒否、位置の確かめ）。
-   互換性（要件書 4.4）は、CodeRabbit CLI が成功したときの手順を変えないことと、前のセッションの `CodeRabbit local review: not run` を読むことで保つ。どちらも文書の記述なので、PR のレビューで確かめる。

### 4.2. 実行範囲と外部の前提

-   新しいテストのファイルと補助は `//go:build test` とする。`make lint` は `--build-tags test,integration` で解析する（`Makefile:10`）。
-   テストが使う外部のコマンドは `git` と `go`（ステップ 2-4 のビルドだけ）である。どちらも開発環境と CI の `ubuntu-latest` にある。偽の `codex` はテストのバイナリそのものなので、新しいツールは加えない。
-   プロセスグループとシグナルは、macOS（手元）と Linux（CI）の両方で動く `syscall` の範囲で扱う。Windows は対象外である（既存の開発用ツールと同じ）。

## 5. 横断検索 (Cross-Search Checklist)

`make lint` と `make test` では見つけられないものだけを挙げる。

1.  [ ] 共通手順の古いファイル名の参照: `git grep -l coderabbit-local-review -- ':!docs/tasks'` の出力が空である。`0d7283e` では 1.3 の 7 つのファイルが出た。
2.  [ ] `GitEnv` と `gitHardening` の古い参照: コメントと `package_reference.md` に、`fixprguard.GitEnv` と `gitHardening` の古い名前が残っていない。`0d7283e` では、`git grep -n "gitHardening\|GitEnv" -- ':!docs/tasks'` が 1.3 の箇所を出した。
3.  [ ] 文書のガードのコマンドの列挙: `.github/workflows/ci.yml` と `CLAUDE.md` の Development Notes の対象が一致する（読んで確かめる）。

## 6. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| テストが本物の `codex` を起動する | 割り当てを使い、差分を OpenAI に送る | テストの `PATH` を、偽の `codex` と `git` の symlink だけから作り、元の `PATH` を引き継がない（2-2） |
| プロセスグループの扱いが macOS と Linux で違う | 一方の環境だけでテストが失敗するか、孫が残る | 手元（macOS）と CI（Linux）の両方でテストを走らせる。孫が止まったことを、プロセス ID で確かめる（1-2・2-3） |
| `codex` の版が変わり、引数や出力の形が変わる | 代替が働かない | 設計書 5.2 のとおり、失敗に倒れて記録される。ステップ 8-2 で今の版の振る舞いを確かめる |
| 文書が `codexreview` を参照した後に、ツールがインストールされていない | 代替が働かない | ステップ 4-1 で確かめてからフェーズ 5 に進む。インストールされていないときは、共通手順が記録して先へ進む（設計書 4 章） |
| 共有の参照（ブランチ、stash）が、ほかの worktree のセッションで変わる | 無人の実行が `state_changed` で止まる | 設計書 5.2 で受け入れたリスクである。`ChangedRefs` で原因を確かめられる |
| 文書のガードの「コマンドの位置」の判定が広すぎるか狭すぎる | 既存の文書が拒まれるか、直接の実行を見逃す | 1.3 の既存の語と、拒むべき形の両方を、組み立てた文書で確かめる（3-2） |
| PR-4 と PR-5 の間で、`CLAUDE.md` の割り当ての記述が古いまま残る | 読み手が割り当てを取り違える | PR-5 を続けて実装する。承認の記述（AC-16）は PR-4 に入れる（1.2） |

## 7. 実装チェックリスト (Implementation Checklist)

-   [ ] フェーズ 1〜3 が完了し、ツールの PR がマージされた
-   [ ] フェーズ 4 のインストールを確かめた
-   [ ] フェーズ 5〜8 が完了し、文書の PR がマージされた
-   [ ] 9 章に 8-1 と 8-2 の記録を書いた

## 8. 成功基準 (Success Criteria)

-   機能: 要件書の AC-03〜AC-18 が、9 章の検証で確かめられている。
-   品質: グリーンゲートと `make deadcode` が通る。追加したテストの壊し方の確認が、コミットメッセージに記録されている。
-   セキュリティ: 設計書 5.1 の守りが、4.1 のテストで確かめられている。単体テストが本物の `codex` を起動しない。
-   文書: 共通手順、5 か所の呼び出し元、`CLAUDE.md`、`.coderabbit.yaml`、`_lib/` の記述が改められ、5 章の横断検索が済んでいる。

## 9. 受け入れ基準の検証 (Acceptance Criteria Verification)

テストの名前は本書で決めたものである。実装で変えたときは、この表に書き戻す。`manual` は PR のレビューで読んで確かめるものである（設計書 7.2）。

| AC | 種別 | 検証 | 実装 |
|---|---|---|---|
| AC-01 | manual | 観測の記録（PR #555）で満たした | — |
| AC-02 | manual | 設計書 1.2 の記録（PR #555・#558）で満たした | — |
| AC-03 | static | `cmd/codexreview/docs_test.go::TestDocumentedProtocolMatches`（共通手順が、合う `--protocol` で `codexreview` を呼ぶ）、`cmd/codexreview/docs_test.go::TestSettingsAllowsTool` | 3-1・3-2・5-1・5-2 |
| AC-03 | manual | 共通手順の、CodeRabbit CLI の成否の判定と、同じ試行の中での代替の記述 | 5-1 |
| AC-04 | manual | 共通手順の、CodeRabbit CLI が成功したときに `codexreview` を実行しない記述 | 5-1 |
| AC-05 | manual | 共通手順の、どちらの CLI の指摘も同じ規則でトリアージする記述 | 5-1 |
| AC-06 | test | `internal/codexreview/…_test.go::TestLocateFindings` | 2-1 |
| AC-06 | manual | 共通手順の、`InRange` が偽の指摘を `invalid` とし、指摘の文面の指示を実行しない記述 | 5-1 |
| AC-07 | manual | 共通手順の、回数の記述 | 5-1 |
| AC-08 | test | `internal/codexreview/…_test.go::TestRunStateChanged`、`::TestRunPreconditions`（`dirty_tree`） | 2-3 |
| AC-08 | manual | 共通手順の、止めて元に戻さない記述 | 5-1 |
| AC-09 | test | `internal/codexreview/…_test.go::TestRunKinds`、`::TestRunTimeoutStopsProcessGroup`、`::TestParseFinalMessage`（読み取れないときに指摘を返さない）、`internal/cmdrunner/runner_test.go` のプロセスグループのテスト、`cmd/codexreview/main_test.go::TestExitCodes` | 1-2・2-1・2-3・2-4 |
| AC-09 | manual | 共通手順の、設計書 4 章の 3 つの扱いの記述 | 5-1 |
| AC-10 | manual | 5 か所の呼び出し元が共通手順を呼び、代替を自分で書いていないこと | 5-1・6-1〜6-3 |
| AC-11 | test | `internal/codexreview/…_test.go::TestRunKinds`（失敗の種類と `Detail`） | 2-3 |
| AC-11 | manual | 共通手順の SUMMARY の記述と、`runplan.md` 手順 8 の要約 | 5-1・6-1 |
| AC-12 | manual | 共通手順の記録の行、残す場所、原文を写さないこと、メンションの確認 | 5-1 |
| AC-13 | manual | `_lib/task-pr-handoff.md` の Part B 手順 3 | 6-2 |
| AC-14 | manual | `runpr.md` の手順 1 と `fixpr.md` の Phase 4.5 | 6-3 |
| AC-15 | manual | `runplan.md` の手順 7.7・5a・8 と、Part B 手順 3 | 6-1・6-2 |
| AC-16 | manual | `CLAUDE.md`「Tool Execution Safety」、共通手順の Authorization、`runpr.md`・`fixpr.md`・`_lib/task-pr-handoff.md` の承認の記述 | 5-1・6-2〜6-4 |
| AC-17 | static | `cmd/codexreview/docs_test.go::TestSettingsAllowsTool`、`::TestNoDirectCodexInvocation` | 3-1・3-2 |
| AC-17 | test | `cmd/codexreview/main_test.go::TestRunRejectsArguments`、`internal/codexreview/…_test.go::TestRunPassesFixedArguments` | 2-3・2-4 |
| AC-17 | manual | ステップ 8-1 の記録（下） | 8-1 |
| AC-18 | test | `cmd/yt2column/codex_review_docs_test.go::TestCodeRabbitConfigStopsReview`（改めた文） | 7-1 |
| AC-18 | manual | 設計書 3.9 の表の記述と、5 章の横断検索 | 5-1・6-1〜6-3・7-1 |

`…_test.go` のファイル名は実装で決め、決めたらこの表に書き戻す。

**ステップ 8-1 の記録:** （実装の際に書く）

**ステップ 8-2 の記録:** （実装の際に書く）

## 10. 次のステップ (Next Steps)

-   本タスクの最後の PR のマージの後、`/runpr` が issue #460 を閉じるかを利用者に尋ねる。
-   `codex` の版が変わったときは、ステップ 8-2 と同じ 1 回の実行で、引数と出力の形を確かめ直す（設計書 5.2・9 章）。
