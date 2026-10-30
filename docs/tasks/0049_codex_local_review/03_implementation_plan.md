# 実装計画書：CodeRabbit CLI が失敗したときに Codex CLI でローカルの CLI レビューを行う

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-29 |
| Review date | 2026-10-29 |
| Reviewer | isseis |
| Comments | 2026-10-29（PR-1 の実装の際の編集上の訂正。決定の変更はない）: 1.2 の `internal/transcript` の形にならう項目と、ステップ 1-2 の記述を、実装に合わせた。`Setpgid` で子をグループの長にする点はならい、グループへの停止は `cmd.Cancel` ではなく、自分で持つ出力のパイプとシグナルの送信で行う。`exec` のコピーの goroutine では、直接の子の終了とパイプの読み終わりの間にグループを止められないためである。猶予は直接の子が終わった時点で終わり、残った子孫はすぐに強制の停止を受ける。(a) の時間切れは、同じ `ctx.Done` の経路を通るコンテキストの取り消しで確かめた。2026-10-29（PR-2 の実装の際の編集上の訂正。決定の変更はない）: ステップ 2-3 の、git-ignore されたファイルを比べないことの確かめを、`TestRunStateChanged` の行から独立のテスト `TestRunIgnoresIgnoredFiles` に分けた（結果の種類が `state_changed` でないため）。`Run` はプロセスの環境を読まず、呼び出し元から受け取る（`internal/config` の `TestEnvAccessConfined` が、環境の読み取りを `main` に限るため。`fixprguard` と同じ形）。ステップ 2-2 の入口の見分けのテストの名前を `TestRoleOf` とし、9 章にテストのファイル名を書き戻した。1.2 の、`Detail` の値と種類を列挙する一覧の関数は、それを使うガード（ステップ 5-2）と同じ PR で加える（`make deadcode` が、どのテストからも届かない関数を拒むため）。2026-10-29（決定の変更。isseis がこのセッションで承認した）: Codex のシェルに環境変数を引き継がせない対策を採ることにした。環境変数を引き継ぐと、Codex が実行するどのコマンドも秘密の値を持ち、`env` の実行や環境を含むエラーの出力から、ファイルを読まなくても暗黙のうちに OpenAI に送られうる。`.envrc` を読む経路とは別の経路であり、「採っても残るリスクは変わらない」という設計書 1.2 の判断は誤りだった（PR #573 の CodeRabbit の指摘と利用者の指摘による）。ステップ 2-3 で `codex` に渡す環境を決めた名前の変数だけにし、固定の引数に `shell_environment_policy.inherit="core"` を加えた（`TestRunPassesOnlyAllowedEnvironment`、`TestRunPassesFixedArguments`）。ステップ 8-2 に、絞った環境での認証と、`env` による確かめを加えた。偽の `codex` は環境ではなく、リンクの隣のファイルと引数で振る舞いを受け取る形に改めた。2026-10-30（PR-3 の実装の際の編集上の訂正。決定の変更はない）: ステップ 2-4 で、`usage` の結果を作る `BadArgumentsResult` と `ProtocolMismatchResult` を `internal/codexreview` に加え、`internal/config` の `TestEnvAccessConfined` の許可に `cmd/codexreview/main.go` の `os.Environ` を加えた。`main` は、`codex` のプロセスグループを止め終えるまで 2 回目のシグナルでも終わらないよう、シグナルを受け続ける（`TestSecondSignalStillStopsCodex`）。ツールへの SIGKILL は防げない。引数は 4 つの形だけを受け付け、`--protocol` を先に照合する。ステップ 3-2 で、`--protocol` をどのツールの呼び出しのものとするかの決め方と、コマンドの文書の一覧は `internal/doctestutil`（`ProtocolFlags`・`CommandBoundary`・`CommandDocs`）に置いて 2 つのガードで共有した。`TestSettingsAllowsTool` は、Bash の許可のコマンドの語を `coderabbit` と `codexreview` に限り、`defaultMode` を確認を求める値に限る形にした（`codex` の語の許可を拒むことを含み、それより広い）。「コマンドの位置」には、予約語（`if`・`then`・`!` など）、リダイレクト、`(`・`{` の後、ラッパーのオプションの値の後、`-lc` のようなオプションの組の `-c` の後も含め、`timeout`・`nice`・`sudo` などのラッパーも加えた。ラッパーとオプションの一覧は網羅でなく、見逃した形は権限の確認で止まる。テストの名前に `TestMayBypassTool`・`TestRunsCodex`・`TestCodexreviewProtocolMismatches`・`TestFixprguardProtocolMismatches`・`TestProtocolFlags` を加えた。2026-10-30（PR-4 の実装の際の編集上の訂正。決定の変更はない）: ステップ 5-2 の一覧の関数を `internal/codexreview` の `Kinds`（種類ごとに、その種類が持ちうる `Detail` を返す）とし、宣言した種類と `Detail` をすべて載せていることを `TestKindsListsEveryConstant` で確かめた。`TestDocumentedInvocationsParse` が `--base` の形を確かめるのに使う `ValidBase` を公開し、`Run` もそれを使う。呼び出しの取り出しを `TestInvocations` で確かめた。共通手順の種類の表には、終了コードに加えて扱い（`triage`・`record`・`stop`）の列を置き、`TestProcedureNamesKinds` が終了コードと `dirty_tree` から決まる扱いと比べる。設計書 1.2 は今のファイル名を参照するテストはないとしたが、その後に加わった `cmd/yt2column/fixpr_docs_test.go` の `TestReadmeSyncStepPlacement` が Part B 手順 1 をファイル名で探していたので、ステップ 5-1 で新しい名前に付け替えた。`_lib/task-pr-handoff.md` の「`_lib/coderabbit-local-review.md` step 5」は、記録の行を書く手順の番号に合わせて step 7 とした。設計書 3.6 の `Codex local review:` の行に、`InRange` が偽の指摘の位置を写さない形を加えた（AC-12 の 3 の適用。設計書の Comments）。`TestDocumentedInvocationsParse` は、呼び出しがツールの名前で始まり前に何もない形（権限の確認で止まらない形）であることも確かめる。共通手順に呼び出しがあることの確かめは、`TestDocumentedProtocolMatches` ではなく `TestDocumentedInvocationsParse` に置いた。共通手順が振る舞いを選ぶ `dirty_tree` は、`internal/codexreview` の `DetailDirtyTree` として公開し、ガードはそれを使う。ステップ 6-1 の手順 7.7 の記録の行の写し先を、設計書 3.7 の編集上の訂正（設計書の Comments）に合わせた。2026-10-30（PR-5 の実装の際の編集上の訂正。決定の変更はない）: ステップ 4-1・8-1 の、実行を確かめる形の記述を直した。共通手順が書く形の `--protocol` は 1 だが、計画は `--protocol 0` を「共通手順が書く形」としていた。0 はツールの値と違うので `usage` が返り、`codex` は起動されない（設計書 7.2 の「不正な引数」）。記述を「共通手順が書く形の `--protocol` の値を 1 つ違えた形」と改め、9 章の記録の文も同じにした。 |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的と原則

[要件定義書](01_requirements.md)（以下、要件書）の AC-03〜AC-18 を、[アーキテクチャ設計書](02_architecture.md)（以下、設計書）に従って実装する。AC-01・AC-02 は、観測の記録（[observations.md](observations.md)、PR #555）と設計書 1.2 で満たした。本書の用語（共通手順、呼び出し元、`codexreview`、ツールの結果、最終メッセージ、試行の結果、記録の行）は設計書冒頭の定義に、要件書 6 節の用語（試行、代替、異系統のレビュー、範囲など）は要件書に従う。

-   **ツールを先に、文書を後に入れる（設計書 8 章）。** `codexreview` は `main` からインストールして使うので、ツールの PR がマージされ、利用者が `make install-codexreview` で入れてから、共通手順がツールを参照する。コマンドの文書もエージェントも、`make install-codexreview` と `go build` を実行しない（設計書 3.8）。
-   **単体テストは `codex` も CodeRabbit CLI も実行しない（`CLAUDE.md`「Testing Strategy」）。** 本物の `codex` は割り当てを使い、差分を OpenAI に送る。そのため、偽の `codex` を使うだけでなく、テストのパッケージ全体で本物の `codex` が見つからないようにする（ステップ 2-2）。
-   Go のコメント・識別子・文字列は英語とする。受け入れ基準との対応は本書 9 章に記録し、ソースのコメントに `AC-NN` を書かない（`requirements_process.md` §4）。
-   テストを加えたら、そのテストが確かめる仕組みを壊して失敗することを確かめ、そのことをコミットメッセージに書く（`CLAUDE.md`「Every test must be able to fail for its stated reason」）。テストを移すか消すときは、`go tool cover -func` の結果が関数ごとに変わらないことを確かめ、そのことを書く。
-   Go のコードを加えるか変えるステップの完了の条件には、`make test` と `make lint` に加えて `make deadcode` を含める（`make deadcode` は CI にもグリーンゲートにもない）。
-   PR-4 か PR-5 のレビューの修正で `internal/codexreview`・`cmd/codexreview`・`internal/cmdrunner` のテスト以外のコードを変えたときは、マージの後に利用者が入れ直し、ステップ 4-1 の確かめをやり直す。PR-1 は `fixprguard` の振る舞いを変えないが、コードは変わるので、マージの後に利用者が `make install-fixprguard` で入れ直す（`CLAUDE.md` の Commands の節）。
-   `.pre-commit-config.yaml` のフックは、Go のファイルを含むコミットのたびに `go test -tags test ./...` を実行する。テストと実装は、通る状態で 1 つのコミットにする。

### 1.2. 計画の際に決めたこと（2026-10-29）

利用者に確かめて決めたこと:

-   **`GitEnv` と git の設定の上書き（`gitHardening`）は、`internal/fixprguard` から `internal/cmdrunner` に移して共有する。** 設計書 3.3 は共有する場所を実装計画に委ねている。設計書 2.2 は「`codexreview` は `internal/cmdrunner` を共有するだけで、`fixprguard` と `mergepr` に依存しない」としている。`internal/cmdrunner` に置けば、この記述とそのまま合う。新しいパッケージ（`internal/gitenv` など）を作る案は、設計書 2.2 の表にないパッケージが増えるので採らなかった。
-   **偽の `codex` は、テストのバイナリそのものとする。** テストのバイナリを `codex` という名前の symlink にして、テストが作る `PATH` に置く。`codex` という名前で起動されたテストのバイナリは、偽の `codex` として動く（Go の helper process の形）。こうすれば、`PATH` の探索、実際のプロセスグループの停止、終了コードを、本物と同じ経路で確かめられる。`sh` のスクリプトは場面ごとにテストの中に散らばるので採らなかった。`internal/cmdrunner/testutil` の `Fake` は、`codexreview` の層でのプロセスの停止を確かめられないので採らなかった。
-   **PR は 5 つに分ける。** 文法の判定と状態の比較を、権限と文書のガードと分けて読めるようにする。ツールと文書の間には、利用者のインストール（フェーズ 4）が入る。PR の区切りは 2 章の `PR-N 作成ポイント` と 3.2 に置いた。

本書で決めたこと:

-   **`codexreview` の終了コードは、`findings`・`no_findings` で 0、`state_changed` で 3、ほかの種類で 1 とする（設計書 3.3「具体的な数値は実装計画で決める」）。** 2 は、Go の実行時のパニックの終了コードであり、`flag` パッケージの既定の終了コードでもあるので使わない。この 2 つのときは標準出力に結果がないので、共通手順は「結果がない」として止まる（設計書 4 章）。126・127 はシェルが使うので使わない。
-   **`codexreview` 自身の失敗の扱い（設計書 4 章「ツールの中のエラーの型は実装計画で決める」）。** `codex` を起動する前の失敗（git の読み取り、`<RUN_DIR>` の作成、`codex --version` の失敗と時間切れ）は、種類 `precondition` で返す。`codex` が動いていないので、記録して先へ進んでよい。`codex` を起動した後に、後の状態を読めないときは、結果を書かずに 1 で終わる。共通手順は「結果がない」として止まる。作業ツリーが変わったかどうかを確かめられないからである（要件書 AC-09）。
-   **時間の配分と、種類の優先の順。** `codex` の時間の上限は 480 秒（設計書 1.2）、終了を求めてから強制的に止めるまでの猶予は 5 秒、`codex --version` の上限は 10 秒とする。実行の前と後の git の読み取りは、それぞれ合わせて 30 秒までとし、`codex` の実行のコンテキストとは別の、取り消されないコンテキストの下で行う。合計は 555 秒で、Bash ツールの 600 秒（設計書 3.1）より短い。時間切れでも、後の状態を比べてから結果を返す。種類の優先の順は、`state_changed`、`timeout`、ほかの種類とする（時間切れはツール自身の時計で判定し、出力の上限より先に見る）。
-   **シグナルを受けたときは、`codex` のプロセスグループを止め、後の状態を比べる。違いがあれば `state_changed` を返し、なければ結果を書かずに 1 で終わる。** シグナルは利用者の中断か Bash ツールの打ち切りなので、共通手順は「結果がない」として止まる（設計書 4 章）。
-   **`Protocol` は 1 から始め、`internal/codexreview` の定数とする。** 設計書 3.3 の `Detail` の値（`bad_arguments`・`protocol_mismatch`・`not_worktree_root`・`base_not_ancestor`・`dirty_tree`・`project_codex_config`・`exit_status_<n>`）はそのまま使う。ほかの値は実装で決め、`internal/codexreview` の一覧の関数で列挙する。共通手順が振る舞いを選ぶのに使う `Detail` は、`dirty_tree` だけである。
-   **設計書 3.9 の記述のうち、テストや規則が前提にするものは、それを必要とするフェーズで書く。** この決定に合わせて、設計書 8 章の 1 と 7 を編集上の訂正として改めた（本 PR）。
    -   `package_reference.md` の行は、パッケージを加えるか変えるステップで書く（フェーズ 1・2）。`TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:671`）が、パッケージと同じコミットで行を求めるためである。
    -   `CLAUDE.md`「Declare, don't infer」の例外は、最終メッセージを解析するステップ 2-1 で書く。`CLAUDE.md`「Testing Strategy」の例外と Commands の節の `make install-codexreview` の行は、文書のガードを加えるステップ 3-2 で書く。`ci.yml` の `doc-guard` と、`CLAUDE.md` の Development Notes のその対象の列挙は、同じステップ 3-3 で直す。規則とそれに頼るコードを同じ PR で読めるようにするためである。
    -   `CLAUDE.md`「Tool Execution Safety」は、設計書 3.8 の項目なので、承認の記述と同じフェーズ 6 で書く。
-   **共通手順が `codexreview` を呼ぶことを求めるガードと、文書の呼び出しを解析するガードは、フェーズ 5 で加える。** フェーズ 3 の時点では、共通手順はまだツールを呼ばない。フェーズ 3 のガードは、組み立てた文書で確かめる。
-   **文書の `codexreview` の呼び出しでは、`--base` に 40 桁の SHA をそのまま書く。** 共通手順は、BASE を先に `git rev-parse` で 40 桁の SHA にし、その値を呼び出しに書き入れる。`$(...)`、変数、`cd ... &&` のような組み合わせは使わない。これらは権限の確認で止まるからである（ステップ 8-1 で確かめる）。
-   **`_lib/task-pr-handoff.md:40` の CHECKS と、`docs/dev/guides/fixprguard_guide.md` の 454 行目にある文書のガードのコマンドには、`./cmd/codexreview` を加えない。** CHECKS は、タスクの文書（`docs/tasks/` の下）だけを変えるコマンドが push の前に実行するものである。`cmd/codexreview` のガードが読むファイル（コマンドの文書、`.claude/settings.json`、`Makefile`、`CLAUDE.md`）を、それらのコマンドは変えない。`fixprguard_guide` の行は、自動レビュアーの一覧を変えるときのガードを指す。CI の `doc-guard` の job には加える（ステップ 3-3）。
-   **`internal/transcript/exec.go:94-116` の、プロセスグループごと止める実装は変えない。** `internal/transcript` は、受け継ぐファイルや標準エラーの扱いが違う独自の実行の経路を持つ。統合は本タスクの範囲の外である。`internal/cmdrunner` の新しいオプションは、この実装の形のうち `Setpgid` で子をグループの長にする点にならう。グループへの停止は、`cmd.Cancel` ではなく、自分で持つ出力のパイプとシグナルの送信で行う。直接の子が終わった後、パイプを読み終える前にグループを止めるためである（ステップ 1-2）。
-   **設計書を、編集上の訂正として補った（本 PR）。** 2.2 の表、3.3、7.1 の改めるテスト、8 章の 1 と 7 を、上の決定に合わせた。決定は変えていないので、設計書の状態は `approved` のままとし、Comments に記録した。`cmd/fixprguard/docs_test.go` の `TestDocumentedProtocolMatches` の変更（1.3）は、`fixprguard` の文書のガードの対象を狭めるものなので、レビュアーはこれを編集上の訂正とみなせるかを確かめる。

### 1.3. 既存コード調査結果

PR-1・PR-2 のマージで `.claude/commands/` と `CLAUDE.md` の行番号が動いた。以下の行番号は `0d7283e` 時点のものである（`cmd/fixprguard/docs_test.go` の行番号も同様に動いた）。ただし、文書のガードのコマンドの列挙と `package_reference.md` のパスは、マージ後のものに直した。2 章以降が引く行番号は、マージ後のものである。

コミット `0d7283e` で調べた。設計書が根拠にしたコミット `c76eec2` から、下に挙げるファイルは変わっていない（`git diff --stat c76eec2 0d7283e` に、これらのファイルはない）。

-   **`internal/cmdrunner`:** `osRunner.RunInput` は `exec.CommandContext` で直接の子だけを止め、プロセスグループは扱わない（`internal/cmdrunner/runner.go:89-93`）。`WaitDelay` の後に Go が送る強制の停止も、直接の子だけに届く。時間切れのときは、コンテキストのエラーと `*ExitError` を `errors.Join` で返す（同 `:115-119`）。出力の上限は、コンテキストのエラーより先に見る（同 `:102-105`）。パッケージのコメント（同 `:1-3`）は、使う側のツールとして `mergepr` と `fixprguard` を挙げている。既存のテストは `sh -c` で子のプロセスを作る（`internal/cmdrunner/runner_test.go:17`）。**足りないもの:** プロセスグループごと止めるオプション。**変えること:** オプションを加え、零値の振る舞いは変えない。`GitEnv` と git の設定の上書きを、ここに移す。
-   **プロセスグループごと止める既存の実装:** `internal/transcript/exec.go:94-116`（`Setpgid` と `cmd.Cancel`、`killProcessGroup`）。取り消しのときだけグループに SIGKILL を送り、正常に終わったときは送らない（同 `:97-100` のコメント）。
-   **`GitEnv` と `gitHardening` の使用箇所:** 定義は `internal/fixprguard/gitstate.go:66-110`。本番のコードの使用は `cmd/fixprguard/main.go:157`、`internal/fixprguard/gitstate.go:482`、`internal/fixprguard/ignored.go:272`。テストと補助の使用は `cmd/fixprguard/test_helpers.go:102,124,130`、`internal/fixprguard/test_helpers_git.go:173,185,187,251,252`、`internal/fixprguard/ignored_test.go:287,320`、`internal/fixprguard/gitstate_test.go:430-450`（`TestGitEnvDropsInheritedGitVariables`。`newTestRepo` と `Worktree.Baseline` を使い、`GIT_DIR` を受け継いでも正しいリポジトリを読むことも確かめる）。名前を挙げるコメントは `internal/fixprguard/gitstate.go:468,669`、`internal/fixprguard/test_helpers_git.go:77,246`、`internal/fixprguard/worktree.go:48`。文書では `docs/dev/conventions/package_reference.md:14`（`fixprguard.GitEnv`）だけである。`gitHardening` のコメント（`gitstate.go:66-84`）は、ほかの設定は `CheckState` が確かめること、diff のコマンドには `--no-ext-diff` と `--no-textconv` を付けることを述べている。`codexreview` には `CheckState` がないので、利用者の git の設定（`status.showUntrackedFiles` など）が比較を変えないようにする（ステップ 2-3）。
-   **テストの git の隔離:** `internal/fixprguard/test_helpers_git.go:104-128` と `cmd/fixprguard/test_helpers.go:42-58` は、`HOME` と `XDG_CONFIG_HOME` を空のディレクトリに向け、利用者の git の設定（コミットの署名、フック）がテストに届かないようにする。どちらもパッケージの中の非公開の補助である。
-   **テストのバイナリを子として起動し直す前例:** `cmd/yt2column/main_test.go:22-45` の `TestMain` は、子のモードを最初に見分け、そうでなければパッケージ全体のプロキシの変数を閉じたアドレスに向けてから `m.Run` を呼ぶ。本物の外部のサービスに届かないようにするためである。`cmd/yt2column-server/main_test.go:12-17` も子のモードを持つ。孫の生死の確かめは、`internal/transcript/exec_test.go:437-530`（`detachedHelperLifetime`、`processExists`・`waitProcessGone`。ゾンビを止まったものとみなす）と `cmd/yt2column/test_helpers.go:392`（`waitForProcessGone`）にある。
-   **`internal/signalrun`:** `Run` は、最初のシグナルでコンテキストを取り消す（`internal/signalrun/signalrun.go:44-66`）。`cmd/codexreview` の `main` はこれを使う。
-   **`fixprguard` の版の照合と文書のガード:** `Protocol` は `internal/fixprguard/types.go:21`（値 4）。`cmd/fixprguard/docs_test.go` の `TestDocumentedProtocolMatches`（`:328`）は、`.claude/commands` の下のすべての文書の `--protocol N`（`protocolFlag`、`:317`）を、本文も含めて `fixprguard` の `Protocol` と比べる。共通手順が `codexreview --protocol 1` を書くと、このテストが失敗する。**変えること:** `codexreview` の呼び出しの `--protocol` を、この照合から除く（ステップ 3-2）。`TestDocumentedInvocationsParse`（`:187`）は、文書の呼び出しをツール自身の引数の解析に通す。`cmd/codexreview` のガードは、この形にならう。
-   **`cmd/yt2column` の文書のガード:** `TestCodeRabbitConfigStopsReview`（`cmd/yt2column/codex_review_docs_test.go:351-356`）は、`.coderabbit.yaml` の文 ``The local CLI review (`coderabbit review`) is still used`` を求める。注釈を改めるので、この文も改める（ステップ 7-1）。`TestCodexMentionOnlyInRequestRow`（同 `:91`）は、`_context.md` の 1 行を除いて、コマンドの文書と `CLAUDE.md` にメンションがないことを確かめる。共通手順は、メンションの確認でもメンションそのものを書かず、`<review-mention>` と書く。`TestPostingCommandsInventory`（同 `:195`）の `postingCommand`（同 `:131`）は `gh` と `fixprguard reply`・`edit-pr` だけに当たり、`codexreview` と `git commit` には当たらない。変える必要はない。`cmd/yt2column/fixpr_docs_test.go` の見出しの定数（`:31-37`）に Phase 4.5 の見出しはないので、見出しを変えても影響はない。
-   **`internal/pipeline` のガード:** `TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:671`）は、`cmd/` と `internal/` の下の、テスト以外のソースを持つすべてのパッケージと、すべての `testutil` に、`package_reference.md` の行を求める。`TestFakesCarryBuildTag` は、`testutil/` の下のファイルの数を 23 に固定し（同 `:619`）、`testutil/` と `test_helpers*.go` の 1 行目を `//go:build test` か `//go:build test || integration` に限る（同 `:628-634`）。
-   **`gosec` の除外:** `.golangci.yml:180-188` は、`gosec` を `_test.go` のファイルだけで除く。`testutil/` と `test_helpers.go` には効かない。前例は `internal/transcript/exec_test.go:461` の、文ごとの `//nolint:gosec`。
-   **`Makefile`:** `install-mergepr`・`install-fixprguard`（`Makefile:72-79`）にならう。`.PHONY`（`Makefile:57`）にも加える。`TestMakeBuildBuildsEveryBinary`（`cmd/yt2column/makefile_test.go:479`）は `build` と `all` だけを見るので、影響はない。
-   **文書のガードのコマンドの列挙:** `.github/workflows/ci.yml:187`、`CLAUDE.md:512`、`.claude/commands/_lib/task-pr-handoff.md:40`、`docs/dev/guides/fixprguard_guide.md:454` の 4 か所にある。これらが一致することを確かめるテストはない。直すのは前の 2 か所である（1.2）。
-   **コマンドの文書にある `codex` の語:** `.claude/commands/runpr.md:240-278`（jq の文字列。`:255` の `kind: "codex"` を含む）、`:278`・`:339`・`:341`・`:404`（コメントの種類の名前としてのコードスパン `codex`）、`.claude/commands/_lib/task-pr-handoff.md:221,228`（HTML の印）、`.claude/commands/_context.md:185-187`（ログイン名と、`@codex review` のメンション）。どれも、`codex` をコマンドとして実行する形ではない。ステップ 3-2 のガードは、これらを拒まない。
-   **共通手順の今のファイル名と、CodeRabbit CLI だけを述べる記述:** `git grep -l coderabbit-local-review -- ':!docs/tasks'` の出力は、`.claude/commands/_lib/coderabbit-local-review.md`、`_lib/pr-review-escalation.md`、`_lib/task-pr-handoff.md`、`_lib/triage-pattern.md`、`fixpr.md`、`runplan.md`、`runpr.md` の 7 つだった（`0d7283e`）。テストはない。設計書 3.9 の表の箇所に加えて、`runpr.md:9-10`（「The local CodeRabbit review is」）と `runplan.md:217`（PR の区切りのない計画で PR を開く経路）も改める。
-   **最終メッセージの実物:** 観測の記録には、形（観測の記録の 5、O1・O2）だけがあり、文面そのものはコミットされていない。テストの入力は、同じ形の架空の文面で作る。
-   **型による振り分けの有無:** `Kind` と `Result` は JSON で出す値で、型による振り分けはない。`Result` を作るのは `internal/codexreview` だけである。`cmd/codexreview` が種類を終了コードに写す `switch` は、知らない種類を `default` で 1 にする。

### 1.4. 申し送りの扱い

`design_handoff.md` の H-01〜H-08 は、設計書の付録 A に記録されている。`implementation_handoff.md` はない。

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: `internal/cmdrunner` の変更（設計書 8 章の 1）

#### ステップ 1-1: `GitEnv` と git の設定の上書きを `internal/cmdrunner` に移す

**変えるファイル**: `internal/cmdrunner/`（新しいファイル `git.go` を想定）とそのテスト、`internal/fixprguard/gitstate.go`・`gitstate_test.go`・`ignored.go`・`ignored_test.go`・`test_helpers_git.go`・`worktree.go`、`cmd/fixprguard/main.go`・`test_helpers.go`、`docs/dev/conventions/package_reference.md`

-   [x] `GitEnv` と git の設定の上書きを `internal/cmdrunner` に移す。設定の上書きは、Go の呼び出し側が書き換えられないように、呼ぶたびに新しいスライスを返す関数として公開する。
-   [x] コメントは、git に共通の理由（別のリポジトリを見ない、コマンドを起動する設定を止める、git 2.12 以上が要る）を移した先に置く。`CheckState` が設定を確かめるという `fixprguard` に固有の理由は、`fixprguard` の使う場所に残す。パッケージのコメント（`runner.go:1-3`）に、git の環境と設定の上書きと、新しく使う側になる `codexreview` を加える。
-   [x] 1.3 の使用箇所と、名前を挙げるコメントを、すべて付け替える。`fixprguard` の振る舞いは変えない。
-   [x] `TestGitEnvDropsInheritedGitVariables` を分ける。環境の変数を除くことと加えることの確かめは `internal/cmdrunner` に移し、`GIT_DIR` を受け継いでも `Worktree.Baseline` が正しいリポジトリを読むことの確かめは `internal/fixprguard` に残す。`go tool cover -func` の結果が、関数の名前ごとに（移した関数は移した先のパッケージで）変わらないことを確かめ、コミットメッセージに書く。
-   [x] 返したスライスを書き換えても、次に呼んだときの結果が変わらないことを確かめるテストを加える。

**完了の条件**: `make test`・`make lint`・`make deadcode` が通る。`internal/fixprguard` と `cmd/fixprguard` の既存のテストが、参照の付け替えのほかは変わらずに通る。

#### ステップ 1-2: プロセスグループごと止めるオプションを加える

**変えるファイル**: `internal/cmdrunner/runner.go`、`internal/cmdrunner/runner_test.go`、`docs/dev/conventions/package_reference.md`

-   [x] `Options` に、子を自分のプロセスグループで起動するオプションを加える（設計書 3.3、1.3 の `internal/transcript` の形）。コンテキストが取り消されたら、グループに終了を求め、猶予の後も直接の子が終わっていなければ、グループに強制の停止を送る（直接の子が猶予の中で終われば、残った子孫には次の文の強制の停止がすぐに届く）。コンテキストが既に取り消されていれば、子を起動しない。直接の子が終わったら、出力のパイプが閉じるのを待つ前にグループに強制の停止を送り、子の終了コードで結果を決める。残った孫が出力のパイプを持ち続けても `exec.ErrWaitDelay` の失敗にはせず、孫が後の状態の比較と同時に動き続けることもないようにするためである（1.3 の `runner.go:89-93` は出力を `os.File` でない書き込み先で受けるので、`Wait` はパイプが閉じるまで待つ）。猶予は `WaitDelay` とは別に持つ。`WaitDelay` は、グループを止めた後にパイプを待つ上限として残す。零値のときは今と同じ振る舞いにする。
-   [x] テストを加える。オプションを付けると、次の 3 つの場合に孫のプロセスが止まる。(a) 時間切れ（時間切れと同じ `ctx.Done` の経路を通る、コンテキストの取り消しで確かめる）、(b) 終了の求めを無視する孫（猶予の後に止まる）、(c) 直接の子は 0 で終わり、孫だけが残って出力のパイプを持ち続ける場合（結果は成功で、標準出力も返る）。直接の子はグループの ID として自分のプロセス ID を、孫は自分のプロセス ID をファイルに書いてから待つので、テストはその書き込みを待ってから取り消す。停止は、期限まで繰り返し確かめ、ゾンビを止まったものとみなす（1.3 の既存の補助にならう）。直接の子のプロセス ID が分かったらすぐに、そのグループに強制の停止を送る `t.Cleanup` を登録し、孫自身にも生きる時間の上限を持たせる。オプションがないときの振る舞いは、既存のテストのまま確かめる。
-   [x] 次の 4 つの壊し方のそれぞれで、新しいテストのどれかが失敗することを確かめる。オプションを外す、グループに送らずに直接の子だけに送る、強制の停止の段を外す、正常に終わったときのグループの停止を外す。壊した形で止まったままにならないように、テストには `WaitDelay` を付ける。
-   [x] `package_reference.md` の `internal/cmdrunner` の行にオプションを加える。

**完了の条件**: `make test`・`make lint`・`make deadcode` が通る。

### PR-1 作成ポイント: cmdrunner process group and shared git environment

**対象ステップ**: 1-1 / 1-2

**推奨タイトル**: `feat(0049): cmdrunner にプロセスグループの停止を加え、git の環境を共有する`

**レビュー観点**: 直接の子が終わった後も、取り消しのときも、孫まで止まるか / `fixprguard` の振る舞いが参照の付け替えのほかに変わっていないか（分けたテストと `go tool cover -func` の記録） / 孫のプロセスがテストの後に残らないか

**実装モデル要件**: frontier-recommended

**判定理由**: 1-2 はプロセスグループとシグナルの扱いという、並行の処理の、切り離した高リスクのステップである

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/570
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `internal/codexreview` と `cmd/codexreview`（設計書 8 章の 2）

#### ステップ 2-1: 最終メッセージの文法と、指摘の位置の確かめ

**変えるファイル**: `internal/codexreview/`（新設。最終メッセージの解析のファイル）とそのテスト、`internal/codexreview/testdata/`、`CLAUDE.md`、`docs/dev/conventions/package_reference.md`

-   [x] 最終メッセージを、設計書 3.4 の形と照らす関数を書く。結果は「指摘あり」「指摘なし」「読み取れない」のどれかで、「読み取れない」のときは指摘を 1 つも返さない。
-   [x] 指摘の位置を確かめる関数を書く（設計書 3.4「位置の確かめ」）。シンボリックリンクを解いた形でルートと比べる。ルートの外のパスを指す指摘が 1 つでもあれば、最終メッセージ全体を「読み取れない」とする。`BASE..HEAD` で変わったファイルにあり、ファイルがあり、行の範囲がファイルの中にあるときだけ `InRange` を真にする。
-   [x] テスト `TestParseFinalMessage` を書く。入力は、観測の記録の 5 と O1・O2 の形（見出しが `Full review comments:` で指摘 2 件、`Review comment:` で指摘 1 件、指摘なしの 1 段落）を架空の文面で作り、`testdata/` に置く。題名に ` — ` を含む指摘（最後の区切りより後ろが位置になる）も受け入れる。設計書 3.4 の「読み取れない」の各例、開始行が終了行より大きい指摘の行、「指摘なし」から除く 3 つの行の形も、それぞれ「読み取れない」になることを確かめる。
-   [x] テスト `TestLocateFindings` を書く。ルートの外のパス、シンボリックリンクを経たルート、範囲で変わっていないファイル、範囲で消されたファイル、ファイルの外の行を扱う。
-   [x] `CLAUDE.md`「Declare, don't infer」に、設計書 1.4 の例外（範囲、理由、境界）を、既存の例外と同じ形で加える（1.2）。
-   [x] `package_reference.md` に `internal/codexreview` の行を加える。

**完了の条件**: `make test`・`make lint`・`make deadcode` が通る。文法の判定と位置の確かめの各分岐を 1 つずつ外し、2 つのテストのどれかの行が失敗することを確かめる。

#### ステップ 2-2: 偽の `codex` と、本物の `codex` を見つけない環境

**変えるファイル**: `internal/codexreview/testutil/helpers.go`（新設、`package codexreviewtestutil`、1 行目は `//go:build test`）、`internal/pipeline/pipeline_test.go`（`TestFakesCarryBuildTag` の数）、`docs/dev/conventions/package_reference.md`

-   [x] テストのバイナリを偽の `codex` にする補助を書く（1.2）。`internal/codexreview` と `cmd/codexreview` の両方のテストから使うので、`testutil` に置く（`test_organization.md` の分類 A）。
-   [x] `TestMain` から最初に呼ぶ入口を用意する。`os.Args[0]` の名前が `codex` なら、フラグを読む前に偽の `codex` として動いて終わる。`codex` のリンクの隣に振る舞いのファイルがなければ、決まった 0 以外の終了コードで終わり、テストを走らせ直さない。そうでなければ、プロセスの `PATH` を `git` だけを置いたディレクトリにし、`exec.LookPath("codex")` が失敗することを確かめてから `m.Run` を呼ぶ（1.3 の `cmd/yt2column/main_test.go` の前例）。テストが作るどの子のプロセスの環境も、この環境から作る。
-   [x] テストが使う準備用の関数を用意する。`codex` という名前の symlink（テストのバイナリへ）と `git` への symlink を置いたディレクトリを `PATH` にする。`git` は本物の実行ファイルを指すようにし、macOS の `/usr/bin/git` で動くことを実装の際に確かめる。`HOME` と `XDG_CONFIG_HOME` を空のディレクトリに向け、`TMPDIR` を `t.TempDir()` に向ける（1.3 の `fixprguard` の補助と同じ形。それらはパッケージの中の非公開の補助なので、共有せずに同じ形で書く）。一時的な git のリポジトリを作る関数も置き、リポジトリの設定で署名を止め、`user.name` と `user.email` を設定する。テストが自分で実行する git には `GIT_CONFIG_NOSYSTEM=1` も付ける。
-   [x] 偽の `codex` の振る舞いを、設計書 7.1 の一覧のとおりに選べるようにする（引数の記録、終了コード、`turn.completed` を出さない、2 つ出す、`error` か `turn.failed` の行を出す、`-o` のファイルを書かない、上限を超えて書く、作業ツリーのファイルを変える、追跡していないファイルを作る、git-ignore されたファイルを変える、ブランチを作る、タグを作る、stash を作る、孫のプロセスを起動して終わらない）。`--version` にも答える。記録やプロセス ID のファイルは、作業ツリーの外に書く。起動されたことは、作業ツリーの外の印のファイルで分かるようにする（`--version` の起動も数える）。孫には生きる時間の上限を持たせる。
-   [x] `os.Executable()` の実行、symlink の作成、変数のパスの読み書きで `gosec` が当たる文には、文ごとに `//nolint:gosec` と、テストのためだけの理由を書く（1.3）。
-   [x] `TestFakesCarryBuildTag` の数を、加えたファイルの数だけ増やす。
-   [x] `package_reference.md` に `internal/codexreview/testutil` の行を加える。

**完了の条件**: ステップ 2-3 のテストで使われ、`make test`・`make lint`・`make deadcode` が通る。`TestMain` の入口の見分け（`codex` の名前のとき、振る舞いのファイルがないとき）を、組み立てた引数で確かめるテスト（`internal/codexreview/testutil/helpers_test.go` の `TestRoleOf`）がある。

#### ステップ 2-3: `codex` の実行と、前後の比較

**変えるファイル**: `internal/codexreview/`（実行のファイル、`test_helpers.go`（1 行目は `//go:build test`））とそのテスト

-   [x] 設計書 3.3 のとおりに、実行の前の確かめ、前の状態の記録、固定した引数での `codex` の実行、完了の判定、最終メッセージの読み取りと判定、後の状態との比較、指示ファイルの変更の確かめ、ツールの結果の組み立てを書く。`Kind`・`Result`・`Finding`・`Protocol` は設計書 3.3 の形とする。時間の配分、優先の順、シグナルと自身の失敗の扱いは 1.2 のとおりにする。
-   [x] `codex` の実行には、ステップ 1-2 のオプションを付けた `internal/cmdrunner` を使う。環境は、呼び出し元（`cmd/codexreview` の `main`）が渡すプロセスの環境から、決めた名前の変数だけを残したもの（設計書 1.2・3.3。`TestRunPassesOnlyAllowedEnvironment`）、標準入力は空、標準出力の上限は 16 MiB とする。git の実行には、ステップ 1-1 の設定の上書きを付ける。
-   [x] 比べる git の出力は、利用者の git の設定に左右されない形で読む（1.3）。使うのは次の 3 つである。追跡していないファイルをすべて出し、サブモジュールの変更を無視しない `status`。`--no-ext-diff`・`--no-textconv`・`--no-renames` を付けた `diff --name-only`。書式を指定した参照と stash の一覧。
-   [x] 時間の上限を短くする関数を `test_helpers.go` に置く。この差し替えは、テストのときだけビルドされ、引数や環境変数からは使えない（設計書 7.1）。`t.Cleanup` で元に戻す。`t.Setenv` とこの差し替えを使うテストは並行に走らせない。
-   [x] テストを書く。どれも、ステップ 2-2 の準備用の関数を使う。各テストは、設計書 3.3・3.4 の分岐ごとに 1 つ以上の行を持つ。
    -   `TestRunPassesFixedArguments`: 偽の `codex` に渡る引数が、設計書 3.3 の固定した形と一致する。
    -   `TestRunPreconditions`: 設計書 3.3 の「実行の前の確かめ」の各行（`codex_not_found` を含む）と、`codex --version` の失敗と時間切れで、偽の `codex` のレビューが一度も起動されず、決めた種類と `Detail` が返る。
    -   `TestRunKinds`: `findings`・`no_findings`・`timeout`・`codex_failed`（`Detail` の `exit_status_<n>`）・`incomplete`（完了の行がない、2 つある、`error` の行、`turn.failed` の行）・`output_too_large`（標準出力、標準エラー、最終メッセージ。最終メッセージは 64 KiB ちょうどと 1 バイト超え）・`output_empty`（空のファイル、ファイルがない）・`unreadable` が、それぞれの条件で返る。
    -   `TestRunStateChanged`: 作業ツリーのファイル、追跡していないファイル（リポジトリに `status.showUntrackedFiles=no` を設定しても）、`HEAD`、ブランチ、タグ、stash のそれぞれを偽の `codex` が変えると、`state_changed` になり、`Changed` が違った項目を示す。2 つ目の worktree を作り、そこで checkout しているブランチが変わると、`ChangedRefs` がその worktree を示す（パスはシンボリックリンクを解いて比べる）。状態の変化と、0 以外の終了、時間切れが重なると、`state_changed` が優先する。
    -   `TestRunIgnoresIgnoredFiles`: git-ignore されたファイルの変更だけでは `state_changed` にならない。
    -   `TestRunTimeoutStopsProcessGroup`・`TestRunCancelStopsProcessGroup`: 時間切れでは種類が `timeout` になり、偽の `codex` が起動した孫のプロセスも止まる。取り消しでも孫が止まり、状態が変わっていれば `state_changed` を返し、変わっていなければ結果を返さない（1.2）。
    -   `TestRunAfterStateUnreadable`: `codex` を起動した後に後の状態を読めないとき、結果を返さずに失敗する（1.2）。
    -   `TestRunInstructionsChanged`: 範囲が下のディレクトリの `AGENTS.md` などを変えたときだけ、`InstructionsChanged` が真になる。
    -   `TestRunDirKeepsOnlyLastMessage`: `<RUN_DIR>` は所有者だけが読み書きでき、最終メッセージのほかに何も残らない。
-   [x] 各分岐を 1 つずつ外して（比較の項目を 1 つ外す、git の出力の書式の指定を外す、引数を 1 つ外す、優先の順を変える、後の状態の読み取りを取り消されたコンテキストで行う）、対応するテストが失敗することを確かめる。

**完了の条件**: `make test`・`make lint`・`make deadcode` が通る。

### PR-2 作成ポイント: codexreview core: grammar, run and state comparison

**対象ステップ**: 2-1 / 2-2 / 2-3

**推奨タイトル**: `feat(0049): Codex CLI のレビューを実行して判定する internal/codexreview を加える`

**レビュー観点**: 最終メッセージの文法が設計書 3.4 のとおりで、読み取れないときに指摘を返さないか / 前後の比較が利用者の git の設定に左右されず、`state_changed` が優先するか / テストのパッケージで本物の `codex` が見つからないことを、`TestMain` が確かめているか / 時間切れ、取り消し、後の状態を読めないときの種類と結果

**実装モデル要件**: frontier-required

**判定理由**: 信頼できない出力の判定と作業ツリーの変化の検出という、安全のための守り（mkplan の panel mode の契機）を実装し、外部のプロセスを扱うテストの基盤も作る

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/573
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2（続き）: `cmd/codexreview`

#### ステップ 2-4: `cmd/codexreview` と `install-codexreview`

**変えるファイル**: `cmd/codexreview/main.go`（新設）、`cmd/codexreview/main_test.go`（新設）、`Makefile`、`docs/dev/conventions/package_reference.md`

-   [x] `main` を `internal/signalrun` の `Run` の下で動かす。引数は `--protocol` と `--base` を 1 回ずつだけ受け付け、ほかのフラグと位置引数を拒む（設計書 3.3）。`--protocol` の不一致と `--base` の形の誤りも `usage` として返す。結果の JSON を標準出力に 1 つだけ書き、1.2 の終了コードで終わる。
-   [x] テストを書く。`TestMain` は、まず子のモード（下の `TestSignalStopsCodexProcessGroup` が使う）を見分け、子は親が渡した環境のまま動く。そうでなければステップ 2-2 の入口を呼ぶ（1.3 の `cmd/yt2column/main_test.go` の順）。
    -   `TestRunRejectsArguments`: 不正な引数、重なったフラグ、位置引数、`--protocol` の不一致、40 桁の 16 進数でない `--base` で `usage` になり、偽の `codex` が起動されない。
    -   `TestExitCodes`: すべての種類と、知らない種類について、終了コードが 1.2 のとおりになる。
    -   `TestOutputIsOneJSONObject`: 標準出力が 1 つの JSON のオブジェクトで、`protocol` が `Protocol` と一致する。
    -   `TestSignalStopsCodexProcessGroup`: テストのバイナリを子のモードで起動し直して `codexreview` として動かし（1.3 の `cmd/yt2column` の前例）、偽の `codex` の孫が起動したのを待ってから SIGTERM を送ると、孫も止まり、`codexreview` が結果を書かずに 1 で終わる。
-   [x] 次の 4 つの壊し方のそれぞれで、対応するテストが失敗することを確かめる。`signalrun` を使わない、重なったフラグを受け付ける、知らない種類を 0 にする、JSON のオブジェクトを 2 つ書く。
-   [x] `Makefile` に `install-codexreview`（`go install ./cmd/codexreview`）を、`install-fixprguard` と同じ形のコメントとともに加え、`.PHONY` にも加える。
-   [x] `package_reference.md` に `cmd/codexreview` の行を加える。

**完了の条件**: `make test`・`make lint`・`make deadcode` が通る。

### フェーズ 3: 権限、文書のガード、CI（設計書 8 章の 3）

#### ステップ 3-1: `.claude/settings.json` の許可

**変えるファイル**: `.claude/settings.json`

-   [x] `permissions.allow` に `Bash(codexreview *)` を加える。`codex` そのものの許可は加えない（設計書 3.8）。

**完了の条件**: ステップ 3-2 の `TestSettingsAllowsTool` が通る。

#### ステップ 3-2: 文書のガード

**変えるファイル**: `cmd/codexreview/docs_test.go`（新設、`//go:build test`）、`cmd/fixprguard/docs_test.go`、`CLAUDE.md`

-   [x] 設計書 7.1 の文書のガードを、`cmd/fixprguard/docs_test.go` の書き方にならって書く。
    -   `TestSettingsAllowsTool`: `.claude/settings.json` の `permissions.allow` に `Bash(<名前> *)` の項目があり、その名前が `cmd/` の下のこのツールのディレクトリの名前と一致する。コマンドの語が `codex` の項目（`Bash(codex *)`、`Bash(codex:*)` など）はない。
    -   `TestDocumentedProtocolMatches`: コマンドの文書の `codexreview` の呼び出しの `--protocol` の値が、`codexreview.Protocol` と一致する。`--protocol` がどのツールの呼び出しのものかは、同じコマンドの区切り（`|`・`;`・`&&` などで分けた部分）の中で、その前にある最も近いツールの語で決める。
    -   `TestNoDirectCodexInvocation`: コマンドの文書のコードブロックと、コマンドとして書いたコードスパンに、コマンドの位置にある `codex` がない。
    -   `TestInstallTargetDocumented`: `Makefile` に `install-codexreview` のターゲットがあり、`CLAUDE.md` の Commands の節に同じ名前の項目がある。
-   [x] 「コマンドとして書いたコードスパン」は、空白で区切った引数を含むコードスパンとする。「コマンドの位置」は、行や文の初め、`|`・`||`・`;`・`&`・`&&`・`$(`・バッククォートの後、`env`・`exec`・`command`・`xargs`・`time`・`nohup`・`sh -c`／`bash -c` の引数の初めとし、パスを付けた形（`/opt/homebrew/bin/codex`、`./codex`）と引用符で囲んだ形を含む。`npx @openai/codex` も拒む。`:` や `==` の後の引用符で囲んだ `"codex"` は、コマンドの位置ではない。
-   [x] 判定の関数を、組み立てた文書で確かめる。上の拒む形をそれぞれ 1 行にし、1.3 の既存の形（`runpr.md:270` の `kind: "codex"`、`runpr.md:293` のコードスパン `codex`、HTML の印、ログイン名、`_context.md:187` のメンション）を拒まないことを、それぞれ 1 行で確かめる。`TestDocumentedProtocolMatches` も、組み立てた文書で、値の違いを見つけることを確かめる。この時点では、共通手順はまだ `codexreview` を呼ばない（1.2）。
-   [x] `cmd/fixprguard/docs_test.go` の `TestDocumentedProtocolMatches` が、上と同じ決め方で `codexreview` の呼び出しのものとした `--protocol` だけを除いて、今と同じくすべての `--protocol N` を確かめるようにする（1.3）。決め方は 2 つのガードで共有する。どの `--protocol N` も 2 つのガードのちょうど 1 つが確かめることを、組み立てた文書で確かめる（両方のツールの呼び出しを 1 行に含むもの、`--protocol` の前に別のフラグがあるもの、パイプを含むもの）。
-   [x] `CLAUDE.md` の Commands の節に `make install-codexreview` の項目を加える（設計書 3.9。1.2）。
-   [x] `CLAUDE.md`「Testing Strategy」の文書のテストの例外に、`codexreview` の `--protocol` の値と、`codex` を直接実行する形がないことの 2 つを加える（設計書 1.4）。

**完了の条件**: `make test`・`make lint`・`make deadcode` が通る。各ガードの判定を 1 つずつ外し、失敗することを確かめる。`TestDocumentedProtocolMatches`・`TestNoDirectCodexInvocation`・`cmd/fixprguard` の除外は組み立てた文書で、`TestSettingsAllowsTool`（許可の名前の比較、`codex` の許可の拒否）と `TestInstallTargetDocumented` は実物のファイルで確かめる。

#### ステップ 3-3: CI の `doc-guard` の job

**変えるファイル**: `.github/workflows/ci.yml`、`CLAUDE.md`

-   [x] `doc-guard` の job の `go test` の対象に `./cmd/codexreview` を加える（設計書 7.1）。このパッケージのほかのテスト（ステップ 2-4）も、この job で走る。
-   [x] `CLAUDE.md` の Development Notes の、`doc-guard` の対象の列挙を同じに直す。文書を読むガードの列挙にも、`cmd/codexreview` のガードを加える。

**完了の条件**: ローカルで `go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview ./internal/pipeline` が通る。PR の CI で `doc-guard` の job が通る。

### PR-3 作成ポイント: codexreview command, permission and document guards

**対象ステップ**: 2-4 / 3-1 / 3-2 / 3-3

**推奨タイトル**: `feat(0049): codexreview コマンドと、その許可と文書のガードを加える`

**レビュー観点**: 引数を `--protocol` と `--base` のほかに受け付けず、シグナルで `codex` の孫まで止まるか / `.claude/settings.json` が `codexreview` だけを許し、`codex` そのものを許さないか / 「コマンドの位置」の判定が、既存の `codex` の語を拒まず、直接の実行の各形を拒むか / `fixprguard` の `--protocol` の照合を狭めた範囲

**実装モデル要件**: frontier-required

**判定理由**: 3-1 の権限の許可と 3-2 の信頼の境界のガード（コマンドの位置の判定、`fixprguard` のガードの対象の変更）は安全のための守りであり、3-3 は CI を変える（mkplan の panel mode の契機）。2-4 はシグナルとプロセスグループという並行の処理を含む

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/581
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: インストール（設計書 8 章の 4）

#### ステップ 4-1: 利用者が `codexreview` をインストールする

**変えるファイル**: なし

-   [x] PR-3 のマージの後、利用者が最新の `main` で `make install-codexreview` を実行する。エージェントはインストールを実行しない（設計書 3.8）。
-   [x] エージェントは、共通手順が書く形の `--protocol` の値を 1 つ違えた形（`--protocol 0 --base <40 桁の SHA>`）で、インストールした `codexreview` を Bash ツールで 1 回実行し、権限の確認で止まらずに `usage` が返ることを確かめる（`codex` は起動されない）。止まるなら、フェーズ 5 に進まずに利用者に報告する。結果はステップ 8-1 で記録する。
-   [x] エージェントは、`command -v codexreview` でツールがあることと、`go version -m "$(command -v codexreview)"` の `vcs.modified` が `false` で、`vcs.revision` が PR-3 のマージのコミットを祖先に持つこと（`git merge-base --is-ancestor`）を確かめる。マージのコミットが手元にないときの `git fetch` は、利用者の承認を得て行う。どれかが成り立たなければ、利用者にインストールを頼んで止まる。

**完了の条件**: 上の確かめが通った。

### フェーズ 5: 共通手順の改名と書き換え（設計書 8 章の 5）

#### ステップ 5-1: 共通手順を `_lib/local-cli-review.md` にする

**変えるファイル**: `.claude/commands/_lib/coderabbit-local-review.md` → `.claude/commands/_lib/local-cli-review.md`、参照を持つ 6 つの文書（1.3）、`cmd/yt2column/fixpr_docs_test.go`

-   [x] `git mv` で名前を変え、設計書 3.1〜3.6 のとおりに書き換える。frontmatter の `description`、見出し、呼び出し元の列挙（5 か所）、入力、Authorization（設計書 3.8）、手順を含む。
-   [x] 手順には次を書く。
    -   CodeRabbit CLI の成否の判定（設計書 3.2）。
    -   失敗したときの `codexreview` の実行（設計書 3.1）。作業ツリーのルートで実行し、HEAD は実行の直前に控える。Bash ツールの時間の上限を 600 秒に指定し、フォアグラウンドで実行する。BASE は 1.2 のとおり 40 桁の SHA で書き入れる。
    -   設計書 4 章の 3 つの扱い。種類（`precondition` の `dirty_tree` を含む）と、それぞれの終了コードと扱いを表で書く（ステップ 5-2 のガードが読む）。
    -   両方の CLI の指摘のトリアージと回数（設計書 3.5）。
    -   記録の行と、それを入れるコミットのメッセージのメンションの確認（設計書 3.6）。前のセッションの `CodeRabbit local review: not run` の読み方。
    -   SUMMARY に書くこと（設計書 3.1）。
-   [x] 文書には、`codex` を引数とともにコマンドの形で書かない（ステップ 3-2 のガードが拒む）。Codex CLI は名前で書く。メンションは `<review-mention>` と書き、メンションそのものを書かない。
-   [x] 参照を持つ 6 つの文書と、Part B 手順 1 をファイル名で探す `cmd/yt2column/fixpr_docs_test.go` の `TestReadmeSyncStepPlacement` のファイル名の参照を、新しい名前に付け替える（設計書 3.9。記述の書き換えはフェーズ 6・7）。

**完了の条件**: `make test` が通る。5 章の横断検索の 1 が空になる。

#### ステップ 5-2: 共通手順の呼び出しと値を確かめるガード

**変えるファイル**: `cmd/codexreview/docs_test.go`、`internal/codexreview/run.go`（`Kinds`・`ValidBase`）、`internal/codexreview/kinds_test.go`

-   [x] `_lib/local-cli-review.md` に `codexreview` の呼び出しが 1 つ以上あることを求める（1.2）。呼び出しの確かめが呼び出しのないときに素通りするのを防ぐ確かめなので、`TestDocumentedProtocolMatches` ではなく、下の `TestDocumentedInvocationsParse` に置いた。
-   [x] `TestDocumentedInvocationsParse` を加える。コマンドの文書のすべての `codexreview` の呼び出しを、ツール自身の引数の解析に通す。`<BASE>` などの置き換える印は、40 桁の SHA に置き換えてから通す。
-   [x] `TestProcedureNamesKinds` を加える。共通手順は、設計書 4 章の扱いを、種類（`precondition` の `dirty_tree` を含む）と終了コードと扱いの表で書く（ステップ 5-1）。テストは、その表の行を、`internal/codexreview` の一覧の関数 `Kinds` が返すすべての種類と `Detail`、`dirty_tree`、1.2 の終了コード、終了コードと `dirty_tree` から決まる扱いと比べる。`Kinds` が宣言したすべての種類と `Detail` を載せていることは `internal/codexreview/kinds_test.go::TestKindsListsEveryConstant` で確かめる（表の値と構造の比較なので、`CLAUDE.md` の文書のテストの規則の範囲に入る）。

**完了の条件**: `make test`・`make lint`・`make deadcode` が通る。実物の文書で、共通手順の呼び出しを消す、`--protocol` の値を変える、フラグを 1 つ加える、種類の 1 つを書き換える、のそれぞれでガードが失敗することを確かめる。

### フェーズ 6: 呼び出し元と承認の記述（設計書 8 章の 6）

各ステップで、コマンドの文書には `codex` を引数とともにコマンドの形で書かない（ステップ 5-1 と同じ。`codexreview` が起動するものは「Codex CLI」と書く）。

#### ステップ 6-1: `runplan.md`

**変えるファイル**: `.claude/commands/runplan.md`

-   [x] 手順 4.5（`coderabbit` が `--base-commit` として受け取るという記述）、手順 7.7、手順 5a、手順 8（要約と、PR の区切りのない計画で PR を開く経路 `:220`）を、設計書 3.7 の表と 3.9 の表のとおりに改める。手順 5a と `:220` の経路は、`Local CLI review: not run` か `CodeRabbit local review: not run` の行があるときだけ、ブランチ全体を改めてレビューする（AC-15）。手順 8 の要約は、試行ごとの結果と Codex CLI の実行回数を示す（AC-11）。

**完了の条件**: `make test` が通る。

#### ステップ 6-2: `_lib/task-pr-handoff.md`

**変えるファイル**: `.claude/commands/_lib/task-pr-handoff.md`

-   [x] Part B 手順 1 の「local CodeRabbit CLI」の記述を改める。Part B 手順 3 が PR 本文に載せるものを、設計書 3.7「Part B 手順 3 が PR 本文に載せるもの」のとおりにする（AC-13・AC-15）。Authorization の `coderabbit review` の記述（`:65-67`）に `codexreview` を加える（AC-16）。

**完了の条件**: `make test` が通る。

#### ステップ 6-3: `runpr.md` と `fixpr.md`

**変えるファイル**: `.claude/commands/runpr.md`、`.claude/commands/fixpr.md`

-   [x] `runpr.md` の冒頭の注記（`:9-10`）を、ローカルの CLI レビューの記述に改める。手順 1 で、異系統のレビューを受けていない範囲を最終報告に示す（AC-14）。PR 本文は変えない。承認の記述（`:82-83`）に `codexreview` を加える（AC-16）。
-   [x] `fixpr.md` の Phase 4.5 の見出しと本文（割り当ての記述 `:603-605` を含む）を、ローカルの CLI レビューに改める。異系統のレビューを受けていない範囲を Final report に示す（AC-14）。承認の記述（`:97-98`）に `codexreview` を加える（AC-16）。

**完了の条件**: `make test` が通る。

#### ステップ 6-4: `CLAUDE.md`「Tool Execution Safety」

**変えるファイル**: `CLAUDE.md`

-   [x] `coderabbit review` の項目の後に、`codexreview` を承認なしに実行してよい項目を加える。その実行が Codex CLI（`codex exec review`）を起動し、差分と Codex が読んだ内容を OpenAI のサービスに送ること、CodeRabbit CLI が失敗したときの代替であることを述べる（設計書 3.8、AC-16）。`CLAUDE.md` はコマンドの文書ではないので、ステップ 3-2 のガードの対象の外である。

**完了の条件**: `make test` と `make lint` が通る。

### PR-4 作成ポイント: shared procedure with the Codex CLI fallback and its callers

**対象ステップ**: 4-1 / 5-1 / 5-2 / 6-1 / 6-2 / 6-3 / 6-4

**推奨タイトル**: `feat(0049): ローカルの CLI レビューに Codex CLI の代替を加える`

**レビュー観点**: 共通手順の、設計書 4 章の 3 つの扱い（止める場合を含む）と記録の行が設計書のとおりか / 前のセッションの `CodeRabbit local review: not run` を引き続き読むか / 5 か所の呼び出し元が代替を自分で書かず、範囲を PR 本文か報告に示すか / 承認の記述と `CLAUDE.md`「Tool Execution Safety」

**実装モデル要件**: frontier-required

**判定理由**: 5 か所の呼び出し元の振る舞いを同時に変え、記録の行の形を新旧の両方で読ませる移行（mkplan の panel mode の契機）である

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/584
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 7: 残りの記述（設計書 8 章の 7）

`_lib/` の文書はコマンドの文書なので、フェーズ 6 と同じく、`codex` を引数とともにコマンドの形で書かない。

#### ステップ 7-1: `CLAUDE.md`、`.coderabbit.yaml`、`_lib/` の記述

**変えるファイル**: `CLAUDE.md`、`.coderabbit.yaml`、`.claude/commands/_lib/pr-review-escalation.md`、`.claude/commands/_lib/triage-pattern.md`、`cmd/yt2column/codex_review_docs_test.go`

-   [x] 設計書 3.9 の表の残りの行を改める。`CLAUDE.md:167` と `:181-183`、`_lib/pr-review-escalation.md:10-12`、`_lib/triage-pattern.md:7`（呼び出し元の 5 か所の列挙）、`.coderabbit.yaml:9-12` である。割り当ての記述は、観測の記録の 1 の結論（PR レビューと同じ割り当てを消費する）と一致させる。
-   [x] `TestCodeRabbitConfigStopsReview` が求める文を、改めた `.coderabbit.yaml` の注釈の文に合わせる（設計書 7.1）。

**完了の条件**: `make test` と `make lint` が通る。`.coderabbit.yaml` の新しい文を消すと `TestCodeRabbitConfigStopsReview` が失敗することを確かめる。

### フェーズ 8: 手動の確かめ（設計書 8 章の 8）

#### ステップ 8-1: 権限の確認で止まらないこと

**変えるファイル**: `docs/tasks/0049_codex_local_review/03_implementation_plan.md`（9 章の記録）

-   [x] ステップ 4-1 で確かめた、共通手順が書く形の `--protocol` の値を 1 つ違えた形（`--protocol 0`）の実行の結果を記録する（設計書 7.2）。
-   [ ] `$(...)` やリダイレクトを含む形で実行しようとすると、権限の確認で止まることを確かめる。
-   [ ] 結果（実行した形、止まったかどうか、返った種類）を 9 章に記録する。

**完了の条件**: 9 章に記録がある。

#### ステップ 8-2: 本物の `codex` での 1 回の実行

**変えるファイル**: `docs/tasks/0049_codex_local_review/03_implementation_plan.md`（9 章の記録）

-   [x] 利用者の承認を得てから、このブランチで `codexreview` を本物の `codex` で 1 回実行し、`findings` か `no_findings` が返ることを確かめる。割り当てを 1 回分使う（設計書 7.2）。
-   [x] この実行で、`codexreview` が絞った環境でも `codex` の認証と接続が通ることを確かめる（設計書 3.3）。
-   [x] 利用者の承認を得てから、`codexreview` と同じ環境と `-c 'shell_environment_policy.inherit="core"'` の下で `codex exec` に `env` を実行させ、`codexreview` が渡す変数のほかと秘密の変数が出ないことを確かめる。割り当てを 1 回分使う。記録には変数の名前だけを書き、値は写さない（設計書 1.2）。
-   [x] 結果（日付、`codex` の版、種類、かかった時間、指摘の数、`env` の確かめの結果）を 9 章に記録する。Codex の文面は写さない。

**完了の条件**: 9 章に記録がある。

#### ステップ 8-3: 横断検索と完了の確認

**変えるファイル**: なし

-   [x] 5 章の横断検索を行い、結果を確かめる。
-   [x] グリーンゲート（`make test && make lint`）が通る。

**完了の条件**: 上の 2 つが済んだ。

### PR-5 作成ポイント: remaining policy text and manual checks

**対象ステップ**: 7-1 / 8-1 / 8-2 / 8-3

**推奨タイトル**: `docs(0049): 割り当てと CodeRabbit CLI の記述を改め、手動の確かめを記録する`

**レビュー観点**: 割り当ての記述が観測の記録の 1 の結論と一致するか / 呼び出し元の列挙が 5 か所と一致するか / 8-1・8-2 の記録が 9 章にあるか

**実装モデル要件**: standard

**判定理由**: 7-1 は記述の書き換えである。8-2 は外部のサービス（Codex）を使うが、利用者の承認を得て、できあがったツールを決まった形で 1 回実行するだけで、設計の判断を含まない。どの契機にも当たらない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. 順序と前提

| 本書のフェーズ | 設計書 8 章の項目 |
|---|---|
| フェーズ 1 | 1. `internal/cmdrunner` のプロセスグループのオプションと、`GitEnv` と git の設定の上書きの移動 |
| フェーズ 2 | 2. `internal/codexreview` と `cmd/codexreview`、`Makefile` の `install-codexreview` |
| フェーズ 3 | 3. `.claude/settings.json` の許可と、文書のガード、`doc-guard` の job の対象 |
| フェーズ 4 | 4. ツールのマージの後、利用者が `make install-codexreview` で入れる |
| フェーズ 5 | 5. 共通手順の改名と書き換え、参照の付け替え |
| フェーズ 6 | 6. 呼び出し元の変更と承認の記述 |
| フェーズ 7 | 7. `CLAUDE.md`、`.coderabbit.yaml`、`_lib/pr-review-escalation.md`、`_lib/triage-pattern.md` の記述と `TestCodeRabbitConfigStopsReview` の文の更新 |
| フェーズ 8 | 8. 手動の確かめ |

-   1.2 のとおり、設計書 3.9 の項目のうち `package_reference.md` の行、`CLAUDE.md` の 2 つの例外、Commands の節、Development Notes の `doc-guard` の列挙は、前のフェーズで書く。
-   ステップ 2-3 は 1-1・1-2・2-2 を、2-4 は 2-3 を、3-2 は 2-3（`Protocol`）と 2-4（`Makefile` のターゲット）を前提にする。
-   フェーズ 5 は、フェーズ 4 のインストールを前提にする。ステップ 4-1 が確かめられなければ、フェーズ 5 に進まない。
-   ステップ 8-1・8-2 は、フェーズ 4 のインストールと、ステップ 3-1 の許可を前提にする。
-   共通手順が書く形の実行が権限の確認で止まらないことは、文書がツールを参照する前に、ステップ 4-1 で確かめる。ステップ 8-1 は、その結果と、止まるべき形の確かめを記録する。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 | `GitEnv` と git の設定の上書きの `internal/cmdrunner` への移動、プロセスグループごと止めるオプション | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 | `internal/codexreview`（文法、位置、実行、前後の比較）、偽の `codex`、`CLAUDE.md`「Declare, don't infer」の例外 | frontier-required |
| PR-3 | 2-4 / 3-1 / 3-2 / 3-3 | `cmd/codexreview`、`install-codexreview`、`.claude/settings.json` の許可、文書のガードと `cmd/fixprguard` の `TestDocumentedProtocolMatches` の除外、`doc-guard` の job、`CLAUDE.md` の Commands の節・「Testing Strategy」の例外・`doc-guard` の列挙 | frontier-required |
| PR-4 | 4-1 / 5-1 / 5-2 / 6-1 / 6-2 / 6-3 / 6-4 | インストールの確かめ、共通手順の改名と書き換え、呼び出し元、承認の記述 | frontier-required |
| PR-5 | 7-1 / 8-1 / 8-2 / 8-3 | `CLAUDE.md`・`.coderabbit.yaml`・`_lib/` の残りの記述、手動の確かめ | standard |

-   PR-1〜PR-3 はツールの PR で、`internal/` を `cmd/` より先にマージする。PR-3 のマージの後に、利用者がインストールする（ステップ 4-1 は PR-4 の最初に確かめる）。
-   PR-2 は、文法の判定と状態の比較という安全のための守りを、権限と文書のガード（PR-3）と分けて読めるようにする。
-   PR-4 は、共通手順と、それを呼ぶ文書と承認の記述を、同じ PR にする。共通手順が承認なしに `codexreview` を実行する時点で、承認の記述がそろっているようにするためである。

### 3.3. マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（PR-3 のマージ） | プロセスグループのオプション、`codexreview`、`install-codexreview`、許可、文書のガード |
| M2（インストール） | 利用者の環境の `codexreview` |
| M3（PR-5 のマージ） | 共通手順と 5 か所の呼び出し元の代替、承認と方針の記述、手動の確かめの記録。本タスクの完了 |

## 4. テスト戦略 (Test Strategy)

### 4.1. 検証の分担

| 層 | 確かめること | ステップ |
|---|---|---|
| `internal/cmdrunner` のテスト | プロセスグループごと止めること（時間切れ、終了の求めの無視、正常な終了の後）、零値の振る舞い、`GitEnv` と設定の上書き | 1-1・1-2 |
| `internal/codexreview` の文法と位置のテスト | 受け入れる形と読み取れない形、位置の確かめ | 2-1 |
| 偽の `codex` の入口のテスト | `codex` の名前のときと、変数がないときの見分け | 2-2 |
| `internal/codexreview` の実行のテスト | 固定した引数、実行の前の確かめ、各種類と境界、前後の比較と優先の順、時間切れと取り消しでの停止、後の状態を読めないとき、指示ファイル、`<RUN_DIR>` | 2-3 |
| `cmd/codexreview` のテスト | 引数の拒否、終了コード、出力の形、シグナルでの停止 | 2-4 |
| 文書のガード（`cmd/codexreview`、`cmd/fixprguard`、`cmd/yt2column`） | 許可の名前と `codex` の許可がないこと、`--protocol` の値、文書の呼び出しの解析、種類と終了コードの値、`codex` を直接実行する形がないこと、インストールのターゲット、`.coderabbit.yaml` の文 | 3-2・5-2・7-1 |
| PR のレビュー（`manual`） | コマンドの文書の手順と記述 | 5-1・6-1〜6-4・7-1 |
| 手動の確かめ | 権限の確認、本物の `codex` での 1 回の実行 | 8-1・8-2 |

-   **網羅の目標:** 設計書 3.3・3.4 の各分岐（実行の前の確かめの各行、各種類、比べる各項目、文法の各規則、位置の確かめの各規則）に、1 つ以上のテストケース（表の行）と、1 つの壊し方の確かめがある。コードの行の網羅率の数値の目標は置かない。
-   設計書 7.3 のセキュリティテストは、上の 2-1・2-3・2-4 のテストに含まれる（引数の拒否と固定、clean でないときと `.codex/` があるときの拒否、前後の比較、プロセスグループの停止、大きさの上限、文法の拒否、位置の確かめ）。
-   互換性（要件書 4.4）は、CodeRabbit CLI が成功したときの手順を変えないことと、前のセッションの `CodeRabbit local review: not run` を読むことで保つ。どちらも文書の記述なので、PR のレビューで確かめる。

### 4.2. 実行範囲と外部の前提

-   新しいテストのファイルと補助の 1 行目は `//go:build test` とする。`make lint` は `--build-tags test,integration` で解析する（`Makefile:10`）。
-   新しいテストが起動する外部のコマンドは `git` だけである。偽の `codex` と孫のプロセスはテストのバイナリそのものを使う。既存の `internal/cmdrunner` のテストにならって、ステップ 1-2 のテストで `sh` を使ってもよい。どれも開発環境と CI の `ubuntu-latest` にある。新しいツールは加えない。
-   `t.Setenv` を使うテストと、時間の上限の差し替えを使うテストは、並行に走らせない。
-   プロセスグループとシグナルは、macOS（手元）と Linux（CI）の両方で動く `syscall` の範囲で扱う。Windows は対象外である（既存の開発用ツールと同じ）。

## 5. 横断検索 (Cross-Search Checklist)

`make lint` と `make test` では見つけられないものだけを挙げる。

1.  [x] 共通手順の古いファイル名の参照: `git grep -l coderabbit-local-review -- ':!docs/tasks'` の出力が空である。`0d7283e` では 1.3 の 7 つのファイルが出た。
2.  [x] `GitEnv` と `gitHardening` の古い参照: コメントと `package_reference.md` に、`fixprguard.GitEnv` と `gitHardening` の古い名前が残っていない。`0d7283e` では、`git grep -n "gitHardening\|GitEnv" -- ':!docs/tasks'` が 1.3 の箇所を出した。
3.  [x] 文書のガードのコマンドの列挙: `.github/workflows/ci.yml` と `CLAUDE.md` の Development Notes の対象が一致する（読んで確かめる）。

## 6. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| テストが本物の `codex` を起動する | 割り当てを使い、差分を OpenAI に送る | テストのパッケージの `TestMain` が、`codex` の見つからない `PATH` にしたことを確かめてから `m.Run` を呼ぶ。テストが作る子のプロセスの環境は、この環境から作る（2-2） |
| 偽の `codex` がテストを走らせ直す | テストが自分を起動し続ける | `os.Args[0]` の名前で見分け、振る舞いのファイルがなければ 0 以外で終わる（2-2） |
| 利用者の git の設定（署名、`status.showUntrackedFiles`）がテストや比較に効く | テストが手元だけで失敗するか、比較が変化を見逃す | テストでは `HOME` と `XDG_CONFIG_HOME` を空にする。本番の比較は、設定に左右されない引数で読む（2-2・2-3） |
| 孫のプロセスがテストの後に残る | 手元と CI に、止まらないプロセスが残る | 起動したらすぐにグループを止める `t.Cleanup` を登録し、孫に生きる時間の上限を持たせる（1-2・2-2） |
| プロセスの停止の確かめが、時間やゾンビで不安定になる | テストがまれに失敗するか、何も確かめずに通る | 孫がプロセス ID を書くのを待ってから止め、停止は期限まで繰り返し確かめる（1-2・2-3） |
| `codex` の版が変わり、引数や出力の形が変わる | 代替が働かない | 設計書 5.2 のとおり、失敗に倒れて記録される。ステップ 8-2 で今の版の振る舞いを確かめる |
| 文書が `codexreview` を参照した後に、ツールがインストールされていない | 代替が働かない | ステップ 4-1 で確かめてからフェーズ 5 に進む。インストールされていないときは、共通手順が記録して先へ進む（設計書 4 章） |
| 文書の呼び出しの書き誤りで、ツールが毎回 `usage` を返す | 代替が気づかれずに働かない | 文書の呼び出しをツールの引数の解析に通すガードを置く（5-2） |
| 共有の参照（ブランチ、stash）が、ほかの worktree のセッションで変わる | 無人の実行が `state_changed` で止まる | 設計書 5.2 で受け入れたリスクである。`ChangedRefs` で原因を確かめられる |
| 文書のガードの「コマンドの位置」の判定が広すぎるか狭すぎる | 既存の文書が拒まれるか、直接の実行を見逃す | 1.3 の既存の語と、拒むべき形の両方を、組み立てた文書で確かめる（3-2） |
| PR-4 と PR-5 の間で、`CLAUDE.md` の割り当ての記述が古いまま残る | 読み手が割り当てを取り違える | PR-5 を続けて実装する。承認の記述（AC-16）は、共通手順と同じ PR-4 に入れる（3.2） |

## 7. 実装チェックリスト (Implementation Checklist)

-   [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2）
-   [x] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3）
-   [x] PR-3 マージ済み（対象ステップ: 2-4 / 3-1 / 3-2 / 3-3）
-   [x] PR-4 マージ済み（対象ステップ: 4-1 / 5-1 / 5-2 / 6-1 / 6-2 / 6-3 / 6-4）
-   [ ] PR-5 マージ済み（対象ステップ: 7-1 / 8-1 / 8-2 / 8-3）

## 8. 成功基準 (Success Criteria)

-   機能: 要件書の AC-03〜AC-18 が、9 章の検証で確かめられている。
-   品質: グリーンゲートと `make deadcode` が通る。追加したテストの壊し方の確かめが、コミットメッセージに記録されている。
-   セキュリティ: 設計書 5.1 の守りが、4.1 のテストで確かめられている。単体テストが本物の `codex` を起動しない。
-   文書: 共通手順、5 か所の呼び出し元、`CLAUDE.md`、`.coderabbit.yaml`、`_lib/` の記述が改められ、5 章の横断検索が済んでいる。

## 9. 受け入れ基準の検証 (Acceptance Criteria Verification)

テストの名前は本書で決めたものである。実装で変えたときは、この表に書き戻す。`manual` は PR のレビューで読んで確かめるものである（設計書 7.2）。`…_test.go` のファイル名は実装で決め、決めたらこの表に書き戻す。

| AC | 種別 | 検証 | 実装 |
|---|---|---|---|
| AC-01 | manual | 観測の記録（PR #555）で満たした | — |
| AC-02 | manual | 設計書 1.2 の記録（PR #555・#558）で満たした | — |
| AC-03 | static | `cmd/codexreview/docs_test.go::TestDocumentedProtocolMatches`、`::TestDocumentedInvocationsParse`（共通手順が `codexreview` を呼び、呼び出しが権限の確認で止まらない形で、ツールの解析を通る）、`::TestInvocations`（呼び出しの取り出し）、`::TestSettingsAllowsTool` | 3-1・3-2・5-1・5-2 |
| AC-03 | manual | 共通手順の、CodeRabbit CLI の成否の判定と、同じ試行の中での代替の記述 | 5-1 |
| AC-04 | manual | 共通手順の、CodeRabbit CLI が成功したときに `codexreview` を実行しない記述 | 5-1 |
| AC-05 | manual | 共通手順の、どちらの CLI の指摘も同じ規則でトリアージする記述 | 5-1 |
| AC-06 | test | `internal/codexreview/locate_test.go::TestLocateFindings` | 2-1 |
| AC-06 | manual | 共通手順の、`InRange` が偽の指摘を `invalid` とし、指摘の文面の指示を実行しない記述 | 5-1 |
| AC-07 | manual | 共通手順の、回数の記述 | 5-1 |
| AC-08 | test | `internal/codexreview/run_test.go::TestRunStateChanged`、`::TestRunIgnoresIgnoredFiles`、`::TestRunPreconditions`（`dirty_tree`）、`internal/cmdrunner/runner_test.go` のプロセスグループのテスト（正常な終了の後の停止） | 1-2・2-3 |
| AC-08 | static | `cmd/codexreview/docs_test.go::TestProcedureNamesKinds`（`state_changed`・`dirty_tree`） | 5-2 |
| AC-08 | manual | 共通手順の、止めて元に戻さない記述 | 5-1 |
| AC-09 | test | `internal/codexreview/run_test.go::TestRunKinds`、`::TestRunPreconditions`、`::TestRunTimeoutStopsProcessGroup`、`::TestRunCancelStopsProcessGroup`、`::TestRunAfterStateUnreadable`、`internal/codexreview/message_test.go::TestParseFinalMessage`（読み取れないときに指摘を返さない）、`internal/cmdrunner/runner_test.go` のプロセスグループのテスト、`cmd/codexreview/main_test.go::TestExitCodes`・`::TestSignalStopsCodexProcessGroup` | 1-2・2-1・2-3・2-4 |
| AC-09 | static | `cmd/codexreview/docs_test.go::TestProcedureNamesKinds`、`internal/codexreview/kinds_test.go::TestKindsListsEveryConstant`（表と比べる一覧が、すべての種類と `Detail` を載せる） | 5-2 |
| AC-09 | manual | 共通手順の、設計書 4 章の 3 つの扱いの記述 | 5-1 |
| AC-10 | manual | 5 か所の呼び出し元が共通手順を呼び、代替を自分で書いていないこと | 5-1・6-1〜6-3 |
| AC-11 | test | `internal/codexreview/run_test.go::TestRunKinds`（失敗の種類と `Detail`） | 2-3 |
| AC-11 | manual | 共通手順の SUMMARY の記述と、`runplan.md` 手順 8 の要約 | 5-1・6-1 |
| AC-12 | static | `cmd/yt2column/codex_review_docs_test.go::TestCodexMentionOnlyInRequestRow`（既存。コマンドの文書にメンションがない。補助の確かめ） | 5-1 |
| AC-12 | manual | 共通手順の記録の行、残す場所、原文を写さないこと、メンションの確認 | 5-1 |
| AC-13 | manual | `_lib/task-pr-handoff.md` の Part B 手順 3 | 6-2 |
| AC-14 | manual | `runpr.md` の手順 1 と `fixpr.md` の Phase 4.5 | 6-3 |
| AC-15 | manual | `runplan.md` の手順 7.7・5a・8 と、Part B 手順 3 | 6-1・6-2 |
| AC-16 | manual | `CLAUDE.md`「Tool Execution Safety」、共通手順の Authorization、`runpr.md`・`fixpr.md`・`_lib/task-pr-handoff.md` の承認の記述 | 5-1・6-2〜6-4 |
| AC-17 | static | `cmd/codexreview/docs_test.go::TestSettingsAllowsTool`（`codex` の許可がないことを含む）、`::TestNoDirectCodexInvocation`、`::TestDocumentedInvocationsParse` | 3-1・3-2・5-2 |
| AC-17 | test | `cmd/codexreview/main_test.go::TestRunRejectsArguments`、`internal/codexreview/run_test.go::TestRunPassesFixedArguments` | 2-3・2-4 |
| AC-17 | manual | ステップ 8-1 の記録（下） | 8-1 |
| AC-18 | manual | 設計書 3.9 の表の記述と、`runpr.md:9-10`、5 章の横断検索 | 5-1・6-1〜6-3・7-1 |
| AC-18 | test | `cmd/yt2column/codex_review_docs_test.go::TestCodeRabbitConfigStopsReview`（改めた文。補助の確かめで、割り当ての記述の中身は `manual` で確かめる） | 7-1 |

**ステップ 8-1 の記録:** （実装の際に書く）ステップ 4-1 の結果（2026-10-30）: `command -v codexreview` は `/Users/<user>/go/bin/codexreview`、`go version -m` は `vcs.revision=81990175a716152fb53a1b039cfcb3ce6d130787`（PR-3 のマージのコミットそのもの）・`vcs.modified=false`。共通手順が書く形の `--protocol` の値を 1 つ違えた `codexreview --protocol 0 --base 81990175a716152fb53a1b039cfcb3ce6d130787` を Bash ツールで 1 回実行し、権限の確認で止まらず、終了コード 1 で `{"kind":"usage","detail":"protocol_mismatch",...}` が返った（`codex` は起動されない）。

**ステップ 8-2 の記録:** 2026-10-30。`codex-cli 0.163.0`。このブランチの `1c8f78f2b8092e332d6a490197713bc9575359f5`（`origin/main`）を BASE として、`codexreview --protocol 1 --base 1c8f78f2b8092e332d6a490197713bc9575359f5` を 1 回実行した。約 18 秒、種類は `no_findings`、指摘は 0 件。絞った環境でも `codex` の認証と接続は通り、`codex_version` は `codex-cli 0.163.0` だった。`instructions_changed` は真（範囲が `CLAUDE.md` を変える）。`env` の確かめは、`codexreview` が渡す環境と同じ変数だけを与え、`-c 'shell_environment_policy.inherit="core"'` の下で `codex exec` に `env` を実行させた。`DEEPSEEK_API_KEY`・`ANTHROPIC_API_KEY`・`SLACK_WEBHOOK_URL` と、与えていない変数 `ENVCHECK_UNLISTED` は出ず、出たのは与えた変数と、`codex` とシェルが自分で加える変数（`CODEX_*`・`PAGER`・`TERM` など）だけだった。値は写さない。

## 10. 次のステップ (Next Steps)

-   本タスクの最後の PR のマージの後、`/runpr` が issue #460 を閉じるかを利用者に尋ねる。
-   `codex` の版が変わったときは、ステップ 8-2 と同じ 1 回の実行で、引数と出力の形を確かめ直す（設計書 5.2・9 章）。
