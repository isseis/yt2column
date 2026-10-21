# 実装計画書：fixpr でレビュースレッドの本文を信頼できないデータとして扱う

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-21 |
| Review date | 2026-10-21 |
| Reviewer | isseis |
| Comments | 2026-10-21 (editorial): ステップ 2-2 の `hasNextPage` の扱いを理由 (e) に写す記述に直した。ステップ 2-4 の `TestFenceNonceAbsent` を `TestFenceBlockShape` に改め（乱数の nonce では nonce の確認を壊しても失敗しないため）、JSON のエスケープの層のテストをステップ 5-1 の `TestBeginOutputFenced` に移した。AC-18 の検証の行を合わせた。AC は変わらない。2026-10-21 (decision): 設計書 3.7 に加えた `GH_REPO`・`GH_HOST` の規則（設定されていれば `gh` を起動せずに止まる）を、ステップ 5-1 の作業とテスト `TestGhTargetEnvRejected` に加えた（PR #406 のレビューの指摘。このセッションでレビュアーが承認した）。AC は変わらない。2026-10-21 (editorial): 1.3.4 でステップ 3-2 に委ねた決定（作業ディレクトリは現在のディレクトリで作業ツリーの最上位、環境は `GitEnv` を使う runner で渡す、システムの設定は残す）を実装し、コミットメッセージに記録した。ステップ 3-5 の停止の種類は `StopKind` として公開した。設計書 3.2・3.6・3.7 を実装に合わせた。AC は変わらない。2026-10-21 (decision): フェーズ 3 のレビューの指摘から、設計書の Comments に記録した 3 つの決定（config の比較をすべての設定の値に広げる、入れ子の `.gitignore` と index の stat の偽造を残余のリスクとする、push 先を同じ名前のブランチとする）を、ステップ 3-2 の基準値の記述に反映した（このセッションでレビュアーが承認した）。extension の TypeScript を実行に関わるファイルに加えるかは、別の issue で扱う。AC は変わらない。2026-10-21 (editorial): PR #409 のレビューの指摘で、index だけにある変更を数え上げに加えた（設計書 3.6）。これにより、判定の後の確認を通った作業ツリーでは `git add -A` と判定したパスの stage が同じ集合を stage するので、ステップ 3-7 の完了条件の「`finish` が `git add -A` を使う」は失敗するテストを持たない。パスを文字どおりに解釈する stage は、`TestFinishStagesJudgedPathsOnly` が引き続き確かめる。AC は変わらない。2026-10-21 (editorial): ステップ 4-1 の本文のファイルの種類の確認を、開く前の `lstat` から、リンクをたどらずに（`O_NOFOLLOW`）開いたファイルの stat に改めた（確認と読み取りの間にファイルを差し替えられないため。設計書 3.8 も合わせた）。空のファイルも拒否する。AC は変わらない。2026-10-21 (editorial): ステップ 4-2 で、`edit-pr` の本文を、検査した内容を標準入力から `--body-file -` で渡す形にした（`gh` がファイルを読み直すと、検査の後の差し替えを防げないため）。センチネルエラーに `ErrCommentNotInThread`（`ErrInvalidInput` を包む）と `ErrReplyNotResolved`（返信の後に解決できなかった。呼び出し元が返信を繰り返さないため）を、`stop` の種類に `thread-not-trusted` と `reply-not-resolved` を加えた（設計書 4 章も合わせた）。AC は変わらない。2026-10-21 (decision): ステップ 4-2 のレビューの指摘から、設計書の Comments に記録した 3 つの決定（返信するコメントを最初のコメントに限る、返信の後・解決の前にスレッドを取り直し、返信のほかに変化があれば解決しない（`TestReplyRechecksBeforeResolve`）、返信が投稿された可能性のある結果を取り消しも含めて `ErrReplyNotResolved` とする（`TestReplyPostFailureSkipsResolve`））をステップ 4-2 に反映した（このセッションでレビュアーが改めて承認した）。AC は変わらない。2026-10-21 (editorial): 同じ指摘で、本文のファイルの空白だけ・NUL・タイトルの制御文字の拒否を加えた（設計書 3.8 も合わせた）。AC は変わらない |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）の F-001〜F-007 を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。判定と書き込みを担う開発用ツール `fixprguard`（`cmd/fixprguard` + `internal/fixprguard` + `internal/cmdrunner`）を作り、`/fixpr` と `/runpr` の文書がそれを呼ぶように切り替える。設計書 8 章の順序の制約をそのまま 6 つのフェーズにする。

-   **フェーズ 1（`internal/cmdrunner`）:** 外部コマンドの実行の境界を `internal/mergepr` から移して共有する。挙動は変えない。
-   **フェーズ 2（読み取り）:** 信頼の判定、スレッドの 2 段階の取得、区切りのブロック、`head`・`list`・`begin` の中身。
-   **フェーズ 3（変更範囲と git の状態）:** `inspect`・`finish`・`push` の中身。
-   **フェーズ 4（書き込み）:** `reply`・`edit-pr` の中身。
-   **フェーズ 5（コマンドと文書）:** `cmd/fixprguard`、`Makefile`、ツールの利用ガイド。
-   **フェーズ 6（コマンド文書の切り替え）:** `fixpr.md`・`runpr.md`・`_context.md`・用語集・文書のガード。**フェーズ 5 の PR をマージして `make install-fixprguard` を実行した後に**始める（設計書 8 章の 6）。

### 1.2. 実装原則

-   判定の分岐は Go のコードに置き、設計書の記述を計画で言い換えない。各ステップは設計書の節を参照する。
-   `CLAUDE.md`「Testing Strategy」に従い、各テストは対象を壊して失敗することを確かめてからコミットし、コミットメッセージに書く。壊す対象は設計書 7.2 の一覧から、そのフェーズで作ったものを選ぶ。
-   既存のテスト補助を再利用する。再利用するのは、`cmd/yt2column` の文書のテストの補助（`readDoc`・`findTable`・`tableRow`・`sectionByHeading`。`cmd/yt2column/docs_test.go:313,377,393,464`）と、`WaitDelay` のテストで子孫プロセスを作る方法（テストのバイナリを再実行する形。`internal/transcript/exec_test.go:462`）である。
-   外部コマンドは、Go のテストでは `internal/cmdrunner/testutil` の fake で置き換える。git は、一時ディレクトリに作った実際のリポジトリで試す。`yt-dlp`・LLM API・Webhook・実際の GitHub は呼ばない。
-   依存パッケージを加えない（`CLAUDE.md`「Dependencies」）。標準ライブラリと `internal/strictjson` だけを使う。
-   Go のコメント・識別子・文字列リテラルは英語で書く。コマンドの定義（`.claude/commands/`）も英語で書く。
-   新しい `*_test.go` とテストの補助は、すべて先頭に `//go:build test` を付ける（`make lint` は `go vet -tags integration ./...` も実行するので、タグのないテストが補助を参照するとコンパイルに失敗する）。`integration` タグは使わない。
-   Go のテストの文字列リテラルは英語で書く。例外は、日本語の見出しや用語集の語に一致させる文書のガードのリテラルだけである（`CLAUDE.md`「Go Idioms」）。
-   `gosec` の指摘（`exec.CommandContext` の引数、ファイルの読み取り）を抑止するときは、`internal/mergepr/runner.go:19` と同じく、該当する最小の範囲に理由付きの `//nolint:gosec` を付ける。同じ種類の箇所すべてに同じ扱いを適用する。`_test.go` の免除が効かない非テストのファイル（`testutil/` と `test_helpers*.go`）の `exec`・`os.WriteFile` にも、同じ範囲の抑止を付ける。マジックナンバー（`mnd`）になる上限の値は、名前付きの定数にする。

### 1.3. 既存コード調査結果

調査は、コミット `bf1d576` で行った（設計書が確かめた `911b535` の後、`.claude/commands/` と `internal/` に変更はない）。

#### 1.3.1. 名前の変更の台帳

| 古い名前 | 新しい名前 | 場所と扱い |
|---|---|---|
| `mergepr.Runner`（インターフェース。`internal/mergepr/mergepr.go:46`） | `cmdrunner.Runner` | `mergepr.Runner` は `cmdrunner.Runner` の型の別名として残す。`New(runner Runner)`（`mergepr.go:57`）の呼び出し側は変わらない |
| `mergepr.NewOSRunner`（`internal/mergepr/runner.go:12`）と `osRunner`（`runner.go:14`） | `cmdrunner.NewOS(Options{})` | `NewOSRunner` は残し、`cmdrunner.NewOS` に委ねる。`osRunner` は取り除く。`cmd/mergepr/main.go:29` は変えない |

台帳にない名前の変更はない。以降の節は、この表から導かれる。

#### 1.3.2. すでにあるもの・足りないもの・変えるもの

| 領域 | すでにあるもの | 足りないもの・変えるもの |
|---|---|---|
| 外部コマンドの実行 | `internal/mergepr/runner.go` の `osRunner`。標準出力を上限なしで読み、失敗時に引数と標準エラーを含む文面を返す（`runner.go:18-26`）。fake は `internal/mergepr/test_helpers.go:48` の `fakeRunner`（`Run` だけを実装） | 出力の上限、`WaitDelay`、標準入力、型付きの失敗 `ExitError`、作業ディレクトリ、子プロセスの環境変数。`Options.Dir` と `Options.Env` は設計書 3.2 の `Options` にないが、設計書 3.7 の「継承した `GIT_*` を取り除く」起動と、一時リポジトリの git と `make` を試すテストに要るので加える（ゼロ値は現在のディレクトリと継承した環境で、現行の挙動）。設計書 3.2 は「フィールドの並び」を実装で決めるとしており、設計書と食い違わない |
| strict な JSON | `internal/strictjson`（公開関数は `ParseObject`・`Required`・`RequiredString`・`OptionalString`、メソッドは `Object.Collect`・`CollectOnly`、`Value.IsNull`・`As*`。`strictjson.go:72,90,109,135,145,163,180`） | 追加なし。`internal/fixprguard` が使う。UTF-8・サロゲート・末尾のデータは `ParseObject`、重複キーは `Collect`・`CollectOnly` が拒否する。GraphQL の `author: null` は正当な入力で、理由 (c) に写す（拒否しない）。これらの挙動は `internal/fixprguard` のテストで確かめる |
| 開発用ツールの先例 | `cmd/mergepr/main.go`（サブコマンドと `flag`）、`Makefile` の `install-mergepr`（`Makefile:68`）、`.PHONY` の行（`Makefile:53`） | `cmd/fixprguard`、`install-fixprguard`、`.PHONY` への追加 |
| パッケージの一覧のガード | `TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:671`）は、非テストのソースを持つ `cmd/`・`internal/` のパッケージと、`testutil` のパッケージの行が `package_reference.md` にないと失敗する | `internal/cmdrunner`・`internal/cmdrunner/testutil`（フェーズ 1）、`internal/fixprguard`（フェーズ 2）、`cmd/fixprguard`（フェーズ 5）の行を、パッケージを作るフェーズで加える。加えないとそのフェーズの `make test` が通らない |
| `testutil` の数のガード | `TestFakesCarryBuildTag` は `testutil` のファイルの数を `21` と固定している（`pipeline_test.go:619`） | `internal/cmdrunner/testutil/mocks.go` と `mocks_test.go` を加えるフェーズ 1 で `23` にする。既存の `testutil` は `mocks.go` と `mocks_test.go` を持ち（`internal/llm/testutil` など）、このガードは `_test.go` も数える。新しい `mocks_test.go` にも `//go:build test` を付ける |
| 文書のガード | `doc-guard` ジョブが `./cmd/yt2column ./cmd/yt2column-server ./internal/pipeline` を毎回実行する（`.github/workflows/ci.yml:170-184`） | 新しい文書のガード `cmd/yt2column/fixpr_docs_test.go` は `cmd/yt2column` に置くので、それだけなら CI の設定は変えない。ただし、引数の解釈を検査する `cmd/fixprguard/docs_test.go` は `package main` のテストで、`./cmd/fixprguard` が対象に入っていないため、ステップ 6-5 で `doc-guard` の対象に加える（設計書 2.2 の「CI の設定を変えない」との差）。`.claude/commands/` を読む既存のテストはない（`rg -n "claude/commands|fixpr|runpr" -g '*_test.go' .` が何も返さないことを `bf1d576` で確かめた） |
| 削除済みの語のガード | `TestSharedTokenRemoved`（`internal/pipeline/relay_guard_test.go:397`）が `docs/dev/` の下の `.md` と `CLAUDE.md` を読み、削除した語を探す | 新しい文書（`fixprguard_guide`）に、その語を書かない。実行は `make test` で確かめる |
| コマンドの文書 | `fixpr.md`（489 行。Phase 1〜6 は 141・198・258・333・374・429 行、Final report は 454 行）、`runpr.md`（333 行。ステップ 1・3・4 は 48・140・232 行、Authorization は 32 行） | 設計書 3.10 の表のとおり。`git add -A`（`fixpr.md:347`）、`git commit -m`（`fixpr.md:349`）、`git push`（`fixpr.md:352`）、`REPLYBODY_EOF`（`fixpr.md:400,402,410,413`）、Phase 6 の `gh pr edit` の heredoc `<<'EOF'` と `--title "` の直書き（`fixpr.md:442-444`）、`comments(first:10)`（`fixpr.md:164`、`runpr.md:197`。`fixpr.md:177` の `first:10` は同じ取得の説明）、`coderabbitai[bot]` の直書き（`runpr.md:169,171`）を取り除く。`runpr.md:181` の `@coderabbitai review`（レビューの依頼のコメント）は、信頼の判定でなく依頼の文面なので残す |
| ツールの文書 | `mergepr_guide.ja.md`（日本語の原本）と `mergepr_guide.md`（英訳）。両方が `Runner` インターフェースに触れる（`.ja.md:235`、`.md:236`） | `fixprguard_guide.ja.md` と `fixprguard_guide.md` を新設する。`mergepr_guide` の 2 つは、`Runner` の所在（`cmdrunner`）の記述を直す |

#### 1.3.3. 外部の前提の確かめ

| 前提 | 確かめた根拠 |
|---|---|
| `gh api` は標準入力から本文を読める | `gh api --help`（`gh` 2.96.0）に `--input file` と「`-` で標準入力」とある |
| `gh pr view --json isCrossRepository,headRepository,headRepositoryOwner`、権限の照会、GraphQL の `comments` の項目 | 設計書 3.3 の観測の表（2026-10-21、`gh` 2.96.0）。計画では再観測しない。観測できなかった値は設計書が安全側に倒している |
| `go install ./cmd/fixprguard` が `$(go env GOPATH)/bin` に入れる | `Makefile:66-69` の `install-mergepr` と同じ形。外部モジュールを使わないので、ネットワークは要らない |
| git の機能（名前の変更の検出の無効化、リテラルの pathspec、`--numstat`、`git ls-remote`、`git rev-parse --git-common-dir`） | 手元の `git` 2.50.1 の `git diff -h`・`git --literal-pathspecs`・`git ls-remote -h`・`git rev-parse --git-common-dir` で存在を確かめた（2026-10-21）。CI（`ubuntu-latest`）の git の版は計画では確かめられないので、ステップ 3-2 で、使う機能が導入された版（実装時に `git help` と git のリリースノートで確かめる）をコメントに記録し、CI のログで `git --version` を確かめる |
| 利用者の git の設定がテストを乱さない | 手元のグローバルの設定に署名の要求がありうる（コミットの署名は `git log` で確認済み）。ツールは継承した `GIT_*` を取り除く（設計書 3.7）ので、`GIT_CONFIG_GLOBAL` では隔離できない。テストは `HOME` と `XDG_CONFIG_HOME` を一時ディレクトリにし（`t.Setenv`。並列にしない）、リポジトリ内の設定で署名とフックを無効にする。システムの設定の扱いは、ステップ 3-2 で決める |

#### 1.3.4. 実装時に決めること

-   `Worktree` の git の呼び出しが対象のディレクトリと環境をどう渡すか（`Options.Dir`・`Options.Env`、`git -C`、`git -c`）。設計書 3.7 の「起動する git の制限」と両立する方を、ステップ 3-2 で選び、理由をコミットメッセージに書く。公開の型（設計書 3.2）は、引数が増えても要件書の AC を変えない。
-   `Login`・`PRNumber`・`Baseline` を、コンストラクタだけで作れる形にする方法は、**同じパッケージ内の複合リテラルを検出する構造のガードのテスト**（`internal/fixprguard/construction_test.go`。ステップ 2-1 で作り、3-2 で `Baseline` を加える）とする。フィールドは非公開にし、ゼロ値は使えない値にする。リーフのパッケージには分けない（設計書 2.2 のファイル構成を変えないため）。
-   `fixpr_docs_test.go` と `cmd/fixprguard/docs_test.go` が参照するツール側の値（commit の件名、`make` のターゲット、`protocol` の番号、`stop` の種類、実行に関わるファイルの一覧）は、`internal/fixprguard` から公開する（`cmd/fixprguard` は `package main` で、他のパッケージのテストが取り込めないため）。公開の形はステップ 3-5 で決める。

### 1.4. 計画の際に決めたこと（2026-10-21）

利用者に確かめて決めたこと:

-   **ツール側の PR は、設計書 8 章の順に分ける（PR-1〜PR-5）。コマンド文書の切り替えは PR-6 とする。** `Worktree` の `inspect` と `finish`・`push` は、基準値の比較と判定した集合を共有するので、同じ PR に入れる。理由: 分けて `inspect` だけを先に入れると、その PR 単体では `finish` が使われず、基準値の比較を使う側が後の PR になる。レビューの大きさより、不変条件を 1 つの PR で読めることを優先した。
-   **`fixprguard_guide` は日本語版を先に作り、英語版を翻訳する。** 設計書 2.2 は英語のみとしていたが、`mergepr_guide.ja.md` と `mergepr_guide.md` の構成に揃える。理由: `CLAUDE.md` の翻訳手順は日本語版を先にコミットするとしており、`mergepr_guide` に先例がある。設計書 2.2 の表の「新設」の 1 行が、2 行（`.ja.md` と `.md`）に増える。AC は変わらない。

計画で設計書 2.2・8 章の記述と異なる扱いにしたもの（AC を変えない）: (1) `package_reference.md` の行を、パッケージを作るフェーズごとに加える（設計書 8 章はフェーズ 5。早いフェーズでは `TestPackageReferenceListsPackages` が失敗する）。(2) `mergepr_guide` の 2 ファイルの `Runner` の記述を直す（ステップ 1-2）。(3) `Options.Env` を加える（1.3.2）。(4) `Login`・`PRNumber`・`Baseline` の構造のガードのテスト（1.3.4）。(5) `Worktree.Begin` を `CheckClean` と `Baseline` に分けて公開し、起動前の比較を `Worktree` の公開のメソッドにする（ステップ 3-2・3-4）。(6) `doc-guard` の対象に `./cmd/fixprguard` を加える（ステップ 6-5）。

設計書が決めたこと（再び開かない）: ガードツールを Go で作ること、判定したパスだけを stage すること、2 段階の取得、`internal/cmdrunner` に移すこと、文書の切り替えをツールの導入の後にすること。

### 1.5. 申し送りの項目への対応

`implementation_handoff.md` は、このタスクにない。`design_handoff.md` の H-01〜H-10 は、設計書の付録 A がすべて記録している（`911b535` で確かめた）。計画で新しく扱うものはない。

## 2. 実装ステップ (Implementation Steps)

各フェーズの終わりに、グリーンゲート（`make test && make lint`）を通す。ステップに付けた設計書の節は、実装の仕様の出所である。各ステップの「テスト」の項は、そのステップが作るテストの名前（5 章の表と同じ名前）と、確かめる境界・失敗の経路を挙げる。表の行や期待する文面は計画に書かず、コードを書くときに決める。

### フェーズ 1: `internal/cmdrunner`（設計書 8 章の 1）

#### ステップ 1-1: `internal/cmdrunner` を作る

**変更するファイル**: `internal/cmdrunner/runner.go`、`internal/cmdrunner/runner_test.go`

-   [x] `Runner`・`InputRunner`・`Options`（`MaxOutput`・`WaitDelay`・`Dir`・`Env`）・`ExitError`・`ErrOutputTooLarge`・`NewOS` を作る（設計書 3.2、1.3.2 の `Dir`・`Env`）。
-   [x] `MaxOutput` を超えた出力は、切り詰めた出力を返さず `ErrOutputTooLarge` で失敗する。上限を超えた後も、子プロセスがブロックしないように読み捨てて待つ。
-   [x] `WaitDelay` を、キャンセルの後の待ちの上限に使う（子孫のプロセスがパイプを持ち続けても待ちが終わる）。
-   [x] `Options` がゼロ値のとき、エラーの文面は `internal/mergepr/runner.go:24` と同じ形（コマンド・引数・終了の状態・標準エラー）で、かつ `errors.AsType[*ExitError]` で取り出せる。
-   [x] `exec.CommandContext` に、`runner.go:19` と同じ理由の `//nolint:gosec` を付ける。
-   [x] テスト（`runner_test.go`）: `MaxOutput` がちょうどの大きさと 1 バイト超過、ゼロ値が上限なしであること（`mergepr` の既存のテストは短い出力しか試さない）、上限を超えた後の読み捨て、`WaitDelay`（子孫のプロセスは `internal/transcript/exec_test.go:462` と同じくテストのバイナリを再実行して作る）、標準入力、`ExitError`、`Dir`、`Env`。

**完了条件**: `go test -tags test ./internal/cmdrunner` が通る。

#### ステップ 1-2: `internal/mergepr` を移す

**変更するファイル**: `internal/mergepr/mergepr.go`、`internal/mergepr/runner.go`、`docs/dev/developer_guide/mergepr_guide.ja.md`、`docs/dev/developer_guide/mergepr_guide.md`

-   [x] `Runner` を `cmdrunner.Runner` の別名にし、`NewOSRunner` が `cmdrunner.NewOS(cmdrunner.Options{})` を返すようにする（1.3.1 の台帳）。`osRunner` を取り除く。
-   [x] `internal/mergepr` の既存のテスト（`runner_test.go` の `TestOSRunnerReturnsOutput`・`TestOSRunnerReportsFailure` を含む）を変えずに通す。これらが、既定の挙動（短い出力の成功と、終了の状態と標準エラーを含む失敗の文面）が変わらないことの確認になる。cmdrunner 側に同じ内容のテストを重ねない（上限なしの大きな出力はステップ 1-1 が試す）。
-   [x] `mergepr_guide.ja.md:235` と `mergepr_guide.md:236` の `Runner` の説明に、所在が `internal/cmdrunner` であることを加える。

**完了条件**: `go test -tags test ./internal/mergepr ./cmd/mergepr` が、テストを変えずに通る。

#### ステップ 1-3: fake と文書のガード

**変更するファイル**: `internal/cmdrunner/testutil/mocks.go`、`internal/cmdrunner/testutil/mocks_test.go`（ともに `package cmdrunnertestutil`、`//go:build test`）、`internal/pipeline/pipeline_test.go`、`docs/dev/developer_guide/package_reference.md`

-   [x] 期待するコマンドの列を順に再生し、順序の違いと余分な呼び出しで失敗する fake を作る。`InputRunner` も実装し、標準入力の内容を記録する。各手順に、実行時の副作用を行う任意の関数を持てるようにする（`make` の fake が範囲内のファイルや git の設定を書き換える場面を、ステップ 3-5・3-7 のテストが作るため）。`internal/mergepr` の `fakeRunner`（`test_helpers.go:48`）は置き換えない。`Run` だけを実装する別の型で、移すと `mergepr` のテストの書き換えが増えるだけである。
-   [x] `mocks_test.go` に、fake 自身のテスト（順序の違い、余分な呼び出し、標準入力の記録、副作用の関数）を書く（`test_organization.md` の分類 A）。
-   [x] `TestFakesCarryBuildTag` の `21` を `23` にする（`pipeline_test.go:619`。1.3.2）。
-   [x] `package_reference.md` に `internal/cmdrunner` と `internal/cmdrunner/testutil` の行を加える。

**完了条件**: `make test && make lint`。壊して確かめる: ステップ 1-1 の上限の確認を外すと、上限のテストが失敗する。

### PR-1 作成ポイント: shared command runner

**対象ステップ**: 1-1 / 1-2 / 1-3

**推奨タイトル**: `refactor(0030): move the command runner to internal/cmdrunner`

**レビュー観点**: `internal/mergepr` の挙動が変わらないこと / 出力の上限と待ちの上限の扱い / `Options.Dir`・`Options.Env` の追加

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-1 が、出力の上限を超えた後の読み捨てと `WaitDelay` という、パイプの並行処理を伴う単独の複雑な処理を含む。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/402
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 読み取り（設計書 8 章の 2）

#### ステップ 2-1: 型・センチネルエラー・引数の文法

**変更するファイル**: `internal/fixprguard/errors.go`、`internal/fixprguard/types.go`（名前は実装で決めてよい）、`internal/fixprguard/types_test.go`、`internal/fixprguard/construction_test.go`

-   [x] 設計書 3.2・4 章の型（`Login`・`PRNumber`・`AuthorKind`・`Reason`・`Cause`・`Finding`・`Assessment`・`Fingerprint`・`Accept`）とセンチネルエラーを作る。ゼロ値は最も仮定の少ない解釈にし、`switch` の `default` は想定外（信頼しない）に倒す。
-   [x] `ParseLogin`（文法は設計書 3.3）、`ParsePRNumber`（10 進数だけ。設計書 3.1）、スレッド ID・コメント ID の文法の検査（設計書 3.8。フェーズ 2 の `--accept` と、フェーズ 3 の `--thread` が使うので、ここで作る）を、補正せずに拒否する形で作る。`--accept`・`--thread` の `<ID>=<指紋>` の解析も、ここで作る。
-   [x] `owner/repo` をローカルのリポジトリ（`gh repo view`）から求める関数を作る（設計書 3.1。権限の照会の URL とフェーズ 4 の書き込みが使う）。
-   [x] `Login`・`PRNumber` はフィールドを非公開にし、ゼロ値を使えない値にする。構造のガードのテストを作る（1.3.4）: パッケージの非テストのソースを `go/parser` で読み、これらの型の複合リテラル（`T{}`・`&T{}`）・`new(T)`・`var x T` が、コンストラクタの中以外にないことを調べる。
-   [x] エラーの文面に、GitHub から来た値と外部コマンドの引数・標準エラーを含めない（設計書 4 章）。
-   [x] 外部コマンドの失敗は、`ExitError` を読んでセンチネルエラーか `Cause` に写し、`ExitError` のテキスト（引数・標準エラー）を `fixprguard` のエラーの文面と出力に残さない。テスト: `TestErrorsOmitCommandDetails`（失敗する fake の標準エラーと引数の文字列が、返ったエラーの文面にも出力にも現れない）。
-   [x] 構造のガードは、構造体のフィールドやスライスの要素としてのゼロ値を検出しない。その場合の守りは、ゼロ値を使えない値にすること（`NewGitHub` がゼロ値の `Login` を拒否する、など）とする。
-   [x] テスト: `TestParseLogin`・`TestParsePRNumber`・`TestIDGrammar`（URL やパスの形、`..` の拒否）・`TestParseAccept`、`construction_test.go::TestRestrictedTypesBuiltByConstructors`（検出する形ごとに、その形を 1 つ足した入力で失敗すること）。

#### ステップ 2-2: スレッドの取得と指紋

**変更するファイル**: `internal/fixprguard/threads.go`、`internal/fixprguard/threads_test.go`

-   [x] 1 段目（本文なし）と 2 段目（処理するスレッドの本文）の問い合わせと応答の検査を作る（設計書 3.4）。検査は `internal/strictjson` を使い、`errors` を含む応答と 16 MiB を超える応答を拒否する。`hasNextPage` が真のコメント（上限 100 件）は、理由 (e) に写す。
-   [x] 指紋を作る（コメントの追加・編集・削除のそれぞれで変わる）。
-   [x] `begin` 用に 1 ページ（100 スレッド）と `capHit`、`list` 用に全ページ（上限 2000 スレッド、超えたら `thread-cap`）を作る。
-   [x] 外部コマンドの呼び出しに、時間の上限（GitHub は 2 分）を設ける。
-   [x] テスト: `TestCommentCap`（100 件と 101 件、`hasNextPage`）、`TestThreadCap`（2000 件と 2001 件）、`TestResponseSizeCap`（16 MiB と 16 MiB + 1）、`TestResponseRejected`（`errors`、重複キー、必須の項目の `null`、応答全体の `null`。コメントの `author: null` は拒否せず、理由 (c) に写る側のテストとして `TestAssessUnverifiableAuthor` が受け持つ）、`TestFingerprintChanges`（追加・編集・削除）、`TestStageOneQueryHasNoBody`（1 段目の問い合わせに本文の項目がない。AC-08）、`gh` の失敗。

#### ステップ 2-3: 信頼の判定

**変更するファイル**: `internal/fixprguard/trust.go`、`internal/fixprguard/trust_test.go`

-   [x] 設計書 3.3 の判定の表を実装する。権限の照会は、設計書 3.3 の予算（`login` ごとに 1 回、全体で 100 回）と順序の規則に従う。失敗は `Cause` に写す。応答は `internal/strictjson` で検査し、`role_name` だけを読む。
-   [x] 理由は該当するものをすべて集める（AC-11）。判定は著者と権限の情報だけを入力にし、本文を受け取らない形にする（AC-08）。文法に合わない名前は出力に書かず、コメントの URL を理由 (c) に付ける（設計書 3.3）。
-   [x] テスト: `TestAssessAuthors`（著者の種類・権限・混在・名前の違う Bot・名前が同じ User）、`TestAssessUnverifiableAuthor`（著者なし・名前の不備・種類の不明・`author: null`）、`TestPermissionBudget`（100 回と 101 回、同じ `login` は 1 回）、`TestPermissionCause`（`Cause` ごと）、`TestUnusableLoginNotPrinted`。

#### ステップ 2-4: 区切りのブロック

**変更するファイル**: `internal/fixprguard/fence.go`、`internal/fixprguard/fence_test.go`

-   [x] `Fence(data, rand)` を作る（設計書 3.5）。nonce がデータに含まれていたら選び直し、使い果たしたら失敗する。
-   [x] テスト: 設計書 7.2 の最後の段落に従い、nonce の確認だけが防げる入力（nonce と同じ文字列を含む本文）と、JSON のエスケープだけが防げる入力（改行と引用符で行頭の区切りを作ろうとする本文）を、別々のテストにする。nonce の確認は `TestFenceRetriesOnCollision`・`TestFenceGivesUp` が試し、ブロックの形は `TestFenceBlockShape` が試す。JSON のエスケープの層は、`Round` を JSON にして `Fence` で囲む処理を持つステップ 5-1 の `TestBeginOutputFenced` が試す（このステップには、その層を壊して失敗させられるコードがないため）。

#### ステップ 2-5: `GitHub` 型

**変更するファイル**: `internal/fixprguard/github.go`、`internal/fixprguard/github_test.go`

-   [x] `NewGitHub`（`--reviewer` が使えない名前、またはゼロ値の `Login` なら失敗）、`Head`（設計書 3.9）、`List`、`Begin`（`accept` の指紋の照合を含む）、`ThreadPaths`（設計書 1.2・3.6。そのスレッドが信頼する著者のスレッドか名指しされたスレッドであることも確かめる）を作る。`Begin` と `List` は最初に fork の判定をする。
-   [x] `begin` の出力の形（設計書 3.4）と、`list` が本文を含まないことを、型の上で表す。出力は設計書 5.4 の項目（`protocol`、権限の照会の使用回数と残り）を含む。`protocol` の番号は、ここで `internal/fixprguard` の定数として定め、公開する（1.3.4）。
-   [x] テスト: `TestHeadRequiresSameRepository`（`true`・`null`・欠け・文字列）、`TestBeginAndListCheckForkFirst`、`TestBeginAssessesEveryCall`、`TestBeginNeverFetchesUntrustedBody`（想定外の著者のスレッドの本文を取得する問い合わせが fake に届かない）、`TestBeginStageTwoFingerprintMismatch`、`TestBeginReportsAllReasons`、`TestBeginAcceptsNamedThread`、`TestBeginAcceptChangedFingerprint`（追加・編集・削除）、`TestListOmitsBodies`、`TestThreadPathsRequiresTrustedOrNamed`、`gh` の失敗。

#### ステップ 2-6: テストの補助と文書のガード（作業は 2-2 の前に始める。2-2〜2-5 のテストが使う）

**変更するファイル**: `internal/fixprguard/test_helpers.go`（`//go:build test`）、`docs/dev/developer_guide/package_reference.md`

-   [x] GitHub の応答の組み立て（スレッド・コメント・権限）をまとめた補助を、`internal/cmdrunner/testutil` の fake の上に作る。
-   [x] `package_reference.md` に `internal/fixprguard` の行を加え、`internal/strictjson` の行の共有先に `internal/fixprguard` を加える。

**完了条件**: `make test && make lint`。壊して確かめる（設計書 7.2 から）: 権限の判定を常に信頼にする、`role_name` の許可の一覧に `read` を加える、`Fence` の nonce の確認を外す、2 段目の指紋の確認を外す。それぞれ、名指しした 2-1〜2-5 のテストのうち、その経路を通るものが失敗することを確かめる。

### PR-2 作成ポイント: read-side trust judgment and thread fetch

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6

**推奨タイトル**: `feat(0030): add the fixprguard trust judgment and thread fetch`

**レビュー観点**: 信頼の判定の表と理由の収集（AC-01〜AC-08）/ 想定外の著者の本文を取得しないこと（AC-09）/ 区切りの nonce の確認（AC-18）/ 応答の厳格な検査

**実装モデル要件**: frontier-required

**判定理由**: 本文を取得しないという安全の保証を担う判定のコードを含み、セキュリティの門にあたる（`mkplan.md` の panel mode の trigger）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/406
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 変更範囲と git の状態（設計書 8 章の 3）

#### ステップ 3-1: 範囲の規則

**変更するファイル**: `internal/fixprguard/scope.go`、`internal/fixprguard/scope_test.go`

-   [x] 設計書 3.6 の規則 1〜6、パスの検査、コマンドの実行に関わるファイルの定義（H-10）を、1 か所（`scope.go`）に実装する。一覧は、ガイドの検査（ステップ 5-3）が読めるように公開する。名指しされた想定外の著者のスレッドでは、`path` そのものの例外を適用しない。
-   [x] 照合は、ファイルシステムを使わない文字列だけの関数にする（大文字小文字を区別しない macOS と区別する Linux の CI で結果が変わらないため）。
-   [x] テスト: `TestScopeRules`（設計書 7.1 の「範囲」の項目）。

#### ステップ 3-2: 一時リポジトリの補助、git の基準値、起動する git の制限

**変更するファイル**: `internal/fixprguard/test_helpers_git.go`（`//go:build test`）、`internal/fixprguard/gitstate.go`、`internal/fixprguard/gitstate_test.go`

-   [x] 先に、一時ディレクトリに実際の git のリポジトリと、`git ls-remote` と push の相手になるローカルの bare リポジトリを作る補助を作る。`HOME` と `XDG_CONFIG_HOME` を一時ディレクトリにし（`t.Setenv`。並列にしない）、リポジトリ内の設定で署名を無効にする。hooks は、テストが明示的に置くものだけが存在する状態にする。git のコマンドだけを実際の実行に回し、`make` と `gh` を fake に回す実行の振り分けも、ここに置く。この補助を使うテストは、この後のすべてのステップで同じ補助を使う。
-   [x] `Baseline` を作る。値は設計書 3.7 の表（head とブランチ、remote の OID、hooks、config（ファイルとすべての設定の値）、info とフラグ）。形式はバージョンの接頭辞付き。形が合わない値は `ErrInvalidInput` で拒否する。`Baseline` も、2-1 の構造のガードの対象に加える。
-   [x] hooks・config・info のダイジェストと、skip-worktree・assume-unchanged のフラグの確認を作る。`core.hooksPath` と worktree の共有の git のディレクトリに対応する。
-   [x] git の起動の制限（設定が起動する外部コマンドの無効化、`GIT_*` の除去、ロックと端末の無効化）と、git・`gh` を起動する前のダイジェストの比較を作る（設計書 3.7）。比較は、`main` が `GitHub.ThreadPaths` の前にも呼べるように、`Worktree` の公開のメソッドにする。
-   [x] 1.3.4 の作業ディレクトリと環境の渡し方、システムの設定の扱いを決める。
-   [x] テスト: `TestBaselineDetectsChange`（head・remote の移動・hooks の追加と削除と実行ビットとリンク先・config・worktree の config・`core.hooksPath` の先・`info/exclude`・skip-worktree・assume-unchanged）、`TestBaselineIgnoresIndexAndFetchHead`、`TestBaselineMalformedRejected`、`TestGitNotLaunchedAfterConfigChange`。

#### ステップ 3-3: 変更の数え上げ

**変更するファイル**: `internal/fixprguard/changes.go`、`internal/fixprguard/changes_test.go`

-   [x] 名前の変更の検出を無効にした数え上げ、変更の種類と行数（バイナリは行数の代わりにバイナリであること）、`lstat` による種類の判定、`JudgedSetID` を作る（設計書 3.6）。
-   [x] 上限（1000 ファイル、8 MiB）と、超えたときの `change-cap` の停止を作る。上限の値は定数にする。
-   [x] テスト: `TestEnumerateChanges`（追加・削除・名前の変更・未追跡・モード）、`TestChangeCaps`（1000 と 1001 ファイル、8 MiB と 8 MiB + 1）。

#### ステップ 3-4: `Worktree` の `Begin` と `Inspect`

**変更するファイル**: `internal/fixprguard/worktree.go`、`internal/fixprguard/worktree_test.go`

-   [x] `Begin`（作業ツリーの確認 AC-20、基準値の採取）と `Inspect`（基準値との比較、数え上げ、範囲との照合）を、設計書 3.2 の署名で作る。`Begin` は、`CheckClean`（作業ツリーの確認）と `Baseline`（基準値の採取）を順に呼ぶだけの形にし、この 2 つも公開する。設計書 3.1 の `begin` の順序（作業ツリー → fork → 基準値 → 取得）の間に fork の確認を挟めるようにするためである。`Inspect` の `paths` は、`main` が `GitHub.ThreadPaths` から取り直して渡す。
-   [x] 範囲外の一覧に、ファイルごとの変更の種類と行数を含める（AC-23）。
-   [x] テスト: `TestBeginRejectsDirtyWorkTree`（未追跡を含め `ErrDirtyWorkTree`。GitHub を呼ばないことは、`main` の層のテスト 5-1 `TestBeginOrder` が確かめる）、`TestInspectInScope`、`TestInspectListsOutOfScope`。

#### ステップ 3-5: `Finish`

**変更するファイル**: `internal/fixprguard/worktree.go`、`internal/fixprguard/worktree_flow_test.go`

-   [x] 設計書 3.6 の `finish` の順序と不変条件を、1 回の呼び出しの中で強制する（厳密な確認 → `make fmt` → 整形後の集合の記録 → `make test`・`make lint` → 厳密な比較 → stage → stage の確認と取り消し → commit → commit の後のツリーの確認）。`make` のターゲットは固定値で、引数から受け取らない。`make` の時間の上限は 30 分、`committed: false`、build checks の失敗の出力（失敗した段階と末尾 8 KiB を区切りのブロックで返す）も、設計書 3.6 のとおりに作る。
-   [x] stage は、判定したパスだけを、パスを文字どおりに解釈する指定で行う。`git add -A` を使わない。
-   [x] commit の件名の組み立て、`make` のターゲットの一覧、`protocol` の番号、`stop` の種類を、`internal/fixprguard` から公開する（1.3.4）。
-   [x] テスト: `TestFinishOrder`（fake の手順の順序）、`TestFinishAbsorbsFormatting`、`TestFinishStopsOnJudgedSetMismatch`、`TestFinishStopsWhenBuildChecksRewrite`、`TestFinishStagesJudgedPathsOnly`（stage の層だけが防げる入力: 判定した集合にはグロブや pathspec の特殊な記法に見える名前のファイルを含め、先に、厳密な確認だけでは止まらないことを確かめる）、`TestFinishStageRollback`、`TestFinishPostCommitTreeCheck`（commit の hook が index を書き換える）、`TestFinishCommitSubject`、`TestFinishEmptyJudgedSet`、`TestFinishBuildCheckFailureOutput`、`TestFinishAfterOutOfScope`（範囲外を判定した集合でも `finish` が進む）、`TestInspectLeavesWorkTree`。

#### ステップ 3-6: `Push`

**変更するファイル**: `internal/fixprguard/worktree.go`、`internal/fixprguard/worktree_flow_test.go`

-   [x] 設計書 3.6 の `push` の不変条件 (1)〜(4) を、push の直前の再確認と明示した refspec で実装する。
-   [x] テスト: `TestPushRechecks`（HEAD が `finish` の SHA でない、hooks が変わった、config が変わった、push 先が変わった、リモートの OID が変わった。それぞれ別の入力で、push が実行されないこと）、`TestPushExplicitRefspec`、`TestPushFailure`。

#### ステップ 3-7: 通しの結合テスト

**変更するファイル**: `internal/fixprguard/worktree_flow_test.go`

-   [x] 設計書 7.3 のとおり、ステップ 3-2 の補助で `Begin` から `Push` の直前までを通して試す。判定の後にファイルを足す、hooks を足す、範囲外のファイルを変える、build checks が範囲内のファイルを書き換える、のそれぞれで、正しい場所で止まること。テスト名: `TestRoundStopsAtRightPlace`（4 つの場面）。

**完了条件**: `make test && make lint`。壊して確かめる（設計書 7.2 から）: 範囲の規則で実行に関わるファイルの除外を外す（`TestScopeRules`）、`finish` が build checks の後の確認を省く（`TestFinishStopsWhenBuildChecksRewrite`）、`finish` が `git add -A` を使う（`TestFinishStagesJudgedPathsOnly`）、基準値の比較から hooks を外す（`TestBaselineDetectsChange`）、`push` が再確認を省く（`TestPushRechecks`）。

### PR-3 作成ポイント: change scope and git state guard

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7

**推奨タイトル**: `feat(0030): add the fixprguard scope check, finish and push`

**レビュー観点**: 範囲の規則と実行に関わるファイルの定義（AC-21・AC-23）/ `finish` の順序と判定した集合の一致（AC-21a）/ 基準値と `push` の再確認（AC-26）/ 起動する git の制限

**実装モデル要件**: frontier-required

**判定理由**: 範囲の確認と git の状態の確認という安全の門を、順序の強制と一時リポジトリを使う結合テストとともに実装する（`mkplan.md` の panel mode の trigger）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/409
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 書き込み（設計書 8 章の 4）

#### ステップ 4-1: 本文のファイルの検査

**変更するファイル**: `internal/fixprguard/write.go`、`internal/fixprguard/write_test.go`

-   [x] 本文のファイルの検査を作る（リンクをたどらずに開いたファイルの stat で通常のファイル、64 KiB 以下、空でない有効な UTF-8。タイトルは 1 行も確かめる。設計書 3.8）。ID の文法と `owner/repo` の取得は、ステップ 2-1 のものを使う。
-   [x] テスト: `TestBodyFileChecks`（シンボリックリンク、64 KiB と 64 KiB + 1、不正な UTF-8、タイトルの複数行）。

#### ステップ 4-2: `Reply` と `EditPR`

**変更するファイル**: `internal/fixprguard/github.go`、`internal/fixprguard/write.go`、各テスト

-   [x] `Reply` を作る。現在の状態でスレッドを判定し直し（AC-07）、信頼する著者のスレッドか、名指しで指紋が一致するスレッドで、指紋が現在と一致し、コメント ID がそのスレッドの最初のコメントであるときだけ書き込む。JSON の本文を組み立てて標準入力で渡し、成功した場合だけ、スレッドを取り直して判定したコメントと返信だけがあることを確かめてから解決する。
-   [x] `EditPR` を作る。タイトルは `--title=<値>` の形で、本文は検査した内容を標準入力から `--body-file -` で渡す（`gh` がファイルを読み直さない）。シェルを介さない。
-   [x] テスト: `TestReplyRefusesUntrustedThread`、`TestReplyReassesses`、`TestReplyRefusesChangedFingerprint`、`TestReplyRefusesForeignComment`（拒否の 4 つの条件は、それぞれ、その条件だけが満たされない入力にする）、`TestReplyBodyVerbatim`（終端の文字列だけの行、`$(...)`、バッククォート、引用符、`@file` の形、`true`・`null` の文字列を入れても、標準入力に渡る JSON の `body` が文面と一致すること）、`TestEditPRVerbatim`、`TestReplyPostFailureSkipsResolve`、`TestReplyRechecksBeforeResolve`、`TestReplyResolveFailure`。

**完了条件**: `make test && make lint`。壊して確かめる（設計書 7.2 から）: `reply` が信頼の判定を省く（`TestReplyRefusesUntrustedThread`）、`reply` が指紋の確認を省く（`TestReplyRefusesChangedFingerprint`）。

### PR-4 作成ポイント: guarded GitHub writes

**対象ステップ**: 4-1 / 4-2

**推奨タイトル**: `feat(0030): add the fixprguard reply and edit-pr writes`

**レビュー観点**: 本文を JSON として標準入力で渡すこと（AC-19）/ 想定外の著者のスレッドへの書き込みの拒否（AC-10）/ 本文のファイルの検査

**実装モデル要件**: frontier-required

**判定理由**: 書き込みの側の安全の門（信頼の判定の再確認、指紋の確認、他のスレッドのコメントの拒否）を担い、PR-2・PR-3 と同じセキュリティの門にあたる（`mkplan.md` の panel mode の trigger）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/414
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: コマンドと文書（設計書 8 章の 5）

#### ステップ 5-1: `cmd/fixprguard`

**変更するファイル**: `cmd/fixprguard/main.go`、`cmd/fixprguard/main_test.go`

-   [ ] 設計書 3.1 のサブコマンド（`head`・`list`・`begin`・`fence`・`inspect`・`finish`・`push`・`reply`・`edit-pr`）の引数の解釈、JSON の出力、`protocol` の照合、終了コード（0・1・2・3）を `cmd/mergepr/main.go` と同じ構成で作る。
-   [ ] 順序の組み立てを `main` に置く（設計書 3.1・3.5・3.7）: `begin` は `Worktree.CheckClean` → `GitHub.Head`（fork の確認）→ `Worktree.Baseline` → `GitHub.Begin`（取得。ここでも fork を確かめる）の順に呼び、出力を `Fence` で囲む。`list`・`head`・`reply`・`edit-pr` は基準値の入力を持たないので、起動前の比較は `inspect` と `finish`・`push` だけが行う（設計書 3.7 が全サブコマンドに求める比較との差。基準値を渡さないサブコマンドは比べる相手がない）。`inspect` は `Worktree` の起動前の比較 → `GitHub.ThreadPaths` → `Inspect`。`--accept`・`--thread`・基準値・`JudgedSetID` の形が合わなければ、`ErrInvalidInput`（終了コード 2）で止まる。
-   [ ] 環境変数 `GH_REPO` または `GH_HOST` が設定されていれば、`gh` を呼ぶサブコマンドは `gh` を起動せずに `ErrInvalidInput`（終了コード 2）で止まる（設計書 3.7）。値を取り除いて進めることはしない。
-   [ ] `--protocol` が合わなければ終了コード 2 で止まる。`--reviewer` は `list`・`begin`・`inspect`・`reply` だけが受け取り、なければ止まる。
-   [ ] テスト（毎回の呼び出しに、必要な `--protocol` と `--reviewer` を渡す）: `TestBeginOrder`（作業ツリーに未コミットの変更があるとき、`gh` を呼んだら失敗する fake で、`gh` が呼ばれないこと。AC-20）、`TestInspectChecksGitStateBeforeGh`、`TestBeginOutputFenced`（改行と引用符で行頭の区切りを作ろうとする本文が、区切りの外に出ないこと。nonce と異なる区切りを使い、nonce の確認が働かない入力にする。ステップ 2-4 から移した）、`TestFenceSubcommand`、`TestAcceptThreadFlagParsing`、`TestProtocolMismatch`、`TestReviewerFlag`、`TestGhTargetEnvRejected`（`GH_REPO` と `GH_HOST` のそれぞれを `t.Setenv` で設定し、`gh` を呼んだら失敗する fake で、`gh` が呼ばれずに終了コード 2 になること）、`TestInspectExitCode`（範囲外で 3）、`TestBeginNothingToProcess`、`TestStopKinds`（`stop` の種類ごとに、その種類を起こす実際の場面から出力を得る。種類の文字列を表に書き写さない）。

#### ステップ 5-2: `Makefile` と案内

**変更するファイル**: `Makefile`、`CLAUDE.md`、`docs/dev/developer_guide/package_reference.md`

-   [ ] `install-fixprguard` を、`install-mergepr`（`Makefile:68`）と同じ形で加え、`.PHONY` の行（`Makefile:53`）にも加える。
-   [ ] `CLAUDE.md` の「Quick Links」と「Build Commands」に案内を加える。
-   [ ] `package_reference.md` に `cmd/fixprguard` の行を加える。

#### ステップ 5-3: 利用ガイド

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`、`docs/dev/developer_guide/fixprguard_guide.md`、`cmd/fixprguard/docs_test.go`

-   [ ] 日本語版を先に書き、コミットする。内容は、ツールの使い方、保証すること・しないこと（設計書 5.2・5.3）、止まったときの回復の手順、範囲の規則の一覧（設計書 3.6）、各サブコマンドを単独で実行する手順（設計書 5.4。毎回 `--protocol` と、必要な `--reviewer` を付ける）。
-   [ ] 英語版を `/mktrans` で翻訳する。
-   [ ] ガイドの検査を作る（`cmd/fixprguard/docs_test.go::TestGuideCoversStopKindsAndScope`）: 日英の両方が、公開した `stop` の種類のすべてと、実行に関わるファイルの一覧のすべての項目を載せ、`CLAUDE.md` がガードの文書へのリンクを持つこと。
-   [ ] ガイドのコマンドのうちネットワークを使わないもの（`fence`、`--protocol` の不一致）を実際に実行し、出力と終了コードがガイドの記述と一致することを確かめる。`inspect`・`finish` は、ステップ 3-7 の結合テストが試すので、手では実行しない。結果は PR の説明に書く。

#### ステップ 5-4: 手動の確認

-   [ ] 利用者の承認を得て、実際の PR に対し `fixprguard head`・`list`・`begin`（読み取りだけ。`begin` は作業ツリーに未コミットの変更がないときだけ進む）を実行し、判定と理由が GitHub 上の実際のコメントと合うことを確かめる（`CLAUDE.md`「Tool Execution Safety」。ネットワークを使うので、承認なしに実行しない）。結果を PR の説明に書く。

**完了条件**: `make test && make lint`、`make build` が成功する。壊して確かめる: `begin` の順序で fork の確認を作業ツリーの確認の前に入れ替える（`TestBeginOrder`）、`inspect` の起動前の比較を省く（`TestInspectChecksGitStateBeforeGh`）、`--protocol` の照合を省く（`TestProtocolMismatch`）、`GH_REPO`・`GH_HOST` の確認を省く（`TestGhTargetEnvRejected`）、ガイドから `stop` の種類を 1 つ消す（`TestGuideCoversStopKindsAndScope`）。

### PR-5 作成ポイント: fixprguard command, Makefile and guide

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4

**推奨タイトル**: `feat(0030): add the fixprguard command and its guide`

**レビュー観点**: サブコマンドと終了コードの契約 / `begin`・`inspect` の順序の組み立て / ガイドの記述が実際の挙動と一致すること / `Makefile`・`CLAUDE.md`・`package_reference.md` の整合

**実装モデル要件**: frontier-recommended

**判定理由**: `begin`・`inspect` の順序の組み立て（作業ツリー → fork → 基準値 → 取得、起動前の比較 → `gh`）という、安全の保証を担う処理を `main` に置く。それ以外は `cmd/mergepr` と同じ配線である。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: コマンド文書の切り替え（設計書 8 章の 6）

**前提（このフェーズに入る前に）**: PR-5 がマージされ、利用者が `main` で `make install-fixprguard` を実行している。`command -v fixprguard` が通り、`fixprguard` の `protocol` の番号がソースの値と一致し、`go version -m "$(command -v fixprguard)"` の `vcs.revision` が PR-5 のマージコミット以降であることを、最初に確かめる（`protocol` の番号が同じまま規則が変わっても、古いバイナリを見つけるため。インストール済みの `mergepr` で `vcs.revision` が出ることを 2026-10-21 に確かめた）。満たされていなければ、このフェーズを始めず、利用者に実行を求める。

#### ステップ 6-1: `_context.md`

**変更するファイル**: `.claude/commands/_context.md`

-   [ ] 自動レビュアーの名前（`coderabbitai`。GraphQL の `author.login` の形。REST の形は `coderabbitai[bot]`）とガードツール（`fixprguard`、`make install-fixprguard`）の 2 行を「Domain-specific」に加える（設計書 3.10、AC-35）。

#### ステップ 6-2: `fixpr.md`

**変更するファイル**: `.claude/commands/fixpr.md`

-   [ ] 設計書 3.10 の `fixpr.md` の表の各行を反映する（冒頭の説明、Authorization の新設、Convergence policy、Caller override、Shared rules の R6、Phase 1〜6、Final report）。
-   [ ] 1.3.2 の「コマンドの文書」の行に挙げた直書きを取り除く（`fixpr.md` の `git add -A`・`git commit -m`・Phase 4 の手順の `git push`・`REPLYBODY_EOF`・Phase 6 の `<<'EOF'` と `--title "`・`comments(first:10)`）。`runpr.md` の `git push -u` と `--title "`（自分の PR の作成）、`fixpr.md` の Authorization の `git push`・`gh pr edit` は、正当な記述なので残す。Phase 4 の commit の件名（`fix: address PR #NUMBER review comments`）は、ツールが組み立てる件名と一致させる。
-   [ ] コマンドの定義は英語で書く。

#### ステップ 6-3: `runpr.md`

**変更するファイル**: `.claude/commands/runpr.md`

-   [ ] 設計書 3.10 の `runpr.md` の表の各行を反映する（冒頭、Authorization、ステップ 1 の `head`、ステップ 3 の `list` と質問、ステップ 4）。
-   [ ] `coderabbitai[bot]` の直書き（`runpr.md:169,171`）を、`_context.md` の名前から作る形に替える。
-   [ ] `/fixpr` を自動で再実行する条件（`runpr.md:212-217`）の文面は変えない（AC-33）。

#### ステップ 6-4: 用語集

**変更するファイル**: `docs/translation_glossary.md`

-   [ ] AC-37 の語（フェーズ・信頼する著者・想定外の著者のスレッド・範囲外の変更）と、設計書 3.10 の語（ガードツール・区切りのブロック・指紋・基準値・名指し）を、英訳とともに加える。「フェーズ」の備考に、「段階」（`stage`）との区別を書く。英訳は `/mktrans` の訳語の原則に従い、レビューで確かめる。

#### ステップ 6-5: 文書のガード

**変更するファイル**: `cmd/yt2column/fixpr_docs_test.go`、`cmd/fixprguard/docs_test.go`、`.github/workflows/ci.yml`、`CLAUDE.md`、`.claude/commands/_lib/task-pr-handoff.md`

-   [ ] `cmd/yt2column/fixpr_docs_test.go` に、設計書 7.4 の (1)〜(7) と、5 章の表の `static` の名前（`TestFixprDocContract`・`TestRunprDocContract` の部分テスト、`TestContextAndCommandsAgree`・`TestAuthorizationSections`・`TestGlossaryTerms`）を書く。ツール側の値は `internal/fixprguard` の公開した値から読み、文書に値を写さない。
    -   `static` の部分テストの名前: `TestFixprDocContract` は `final_report`・`accept_only_reported`・`nothing_to_process`・`round_counting`・`fenced_prompts`・`out_of_scope_stop`・`people_answered_scope`、`TestRunprDocContract` は `head_before_checkout`・`untrusted_question`・`stop_without_merge`・`no_body_fetch`・`auto_fixpr_condition`。
    -   `TestAuthorizationSections` は、`fixpr.md` と `runpr.md` の両方の Authorization が、新しいネットワークの操作を挙げていることを調べる（AC-36）。
    -   `TestContextAndCommandsAgree` は、`coderabbitai` の文字列が `_context.md` の外に現れる箇所を、レビュー済みの許可の一覧（`@coderabbitai review` の依頼など）だけにする（AC-35）。
    -   取り除いた直書きが残っていないことも調べる。調べる対象は、ファイルと形を限る。`fixpr.md` は `git add -A`・`git commit -m`・`REPLYBODY_EOF`・`<<'EOF'`・`gh pr edit` の行の `--title "`・`comments(first:10)`、Phase 4 の本文の `git push`（Authorization の節は除く）。`runpr.md` は `coderabbitai[bot]`・`comments(first:10)`。
    -   `TestGlossaryTerms` の日本語のリテラルは、用語集の語に一致させるための例外である（1.2）。
-   [ ] `cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse` を書く。`fixpr.md` と `runpr.md` に書かれた `fixprguard` の呼び出しのすべてを、実際の引数の解釈に通し、サブコマンド、フラグの名前、`--protocol` と `--reviewer` の有無が受け付けられることを調べる。
-   [ ] `doc-guard` ジョブの `go test` の対象に `./cmd/fixprguard` を加える。文書だけの PR は `check-changes` が非コードと分類してテストを飛ばすので、この対象に入れないと文書の変更でガードが働かない。`CLAUDE.md`（「Development Notes」の `doc-guard` の説明）と `_lib/task-pr-handoff.md` の CHECKS の記述の対象の一覧も、同じ値に揃える。

#### ステップ 6-6: 手動の確認

-   [ ] 切り替えの後、実際の `/fixpr` と `/runpr` の実行を観察する（フェーズ 1 の `begin` から最終報告までが文書のとおりに進むこと）。想定外の著者のスレッドの実例は、リポジトリの外の人が書かないと作れないので、実例がなければ、`fixprguard begin` の出力を使い、文書の流れを読み合わせる。結果を PR の説明に書く。

**完了条件**: `make test && make lint`。壊して確かめる: 文書のガードの検査対象を 1 つずつ元に戻し（`runpr.md` に `coderabbitai[bot]` を戻す、`fixpr.md` に `git add -A` を戻す、`_context.md` の名前を消す、`fixpr.md` の呼び出しのフラグ名を変える、用語集の行を 1 つ消す）、対応するテスト（`TestContextAndCommandsAgree`・`TestFixprDocContract`・`TestDocumentedInvocationsParse`・`TestGlossaryTerms`）が失敗すること。

### PR-6 作成ポイント: command document switch

**対象ステップ**: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6

**推奨タイトル**: `feat(0030): switch fixpr and runpr to the fixprguard tool`

**レビュー観点**: 文書の流れが設計書 6.1・6.2 と一致すること / 取り除くべき直書きが残っていないこと / 名指しの経路が直前の報告に限られること（AC-12）/ Authorization の網羅

**実装モデル要件**: frontier-recommended

**判定理由**: `/fixpr` と `/runpr` の実際の動作を切り替える、隔離された高リスクの変更である。PR を戻すときは、文書・ガード・用語集・CI の設定を 1 つの単位として戻す（6.1）。文書の変更は、挙動に関わる検査をガードのテストが担い、残りは文章なので、`required` にはしない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の目安 |
|---|---|---|
| M1 | `internal/cmdrunner`、`internal/mergepr` の移行 | PR-1 のマージ。`mergepr` のテストが変更なしで通る |
| M2 | 信頼の判定とスレッドの取得 | PR-2 のマージ |
| M3 | 範囲・git の状態の確認、`finish`・`push` | PR-3 のマージ |
| M4 | `reply`・`edit-pr` | PR-4 のマージ |
| M5 | `fixprguard` のインストールと利用ガイド | PR-5 のマージ、利用者の `make install-fixprguard` |
| M6 | コマンド文書の切り替え | PR-6 のマージ |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 | `internal/cmdrunner` の新設、`internal/mergepr` の移行、fake | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 | 型・取得・信頼の判定・区切り・`GitHub` 型 | frontier-required |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 | 範囲・基準値・数え上げ・`Inspect`・`Finish`・`Push`・結合テスト | frontier-required |
| PR-4 | 4-1 / 4-2 | `Reply`・`EditPR`・本文のファイルの検査 | frontier-required |
| PR-5 | 5-1 / 5-2 / 5-3 / 5-4 | `cmd/fixprguard`（順序の組み立て）・`Makefile`・利用ガイド（日英）・手動の確認 | frontier-recommended |
| PR-6 | 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 | コマンド文書・`_context.md`・用語集・文書のガード | frontier-recommended |

### 3.3. 実装順序の根拠

設計書 8 章の順序の制約に従う。フェーズ 1 は挙動を変えないので、先に入れて後の PR の土台にする。フェーズ 2〜4 は、`internal/fixprguard` の中で、読み取り → 変更範囲と git → 書き込みの順に依存する（`Finish` が基準値と範囲を、`Reply` が信頼の判定を使う）。フェーズ 5 は、`cmd/` を `internal/` の後に置く。フェーズ 6 は、ツールがマージされて手元にインストールされるまで始められない（ツールがないと `/fixpr` と `/runpr` が止まる）。この前提は、フェーズ 6 の先頭に書いた。この PR 自体（フェーズ 1〜5）の `/fixpr` と `/runpr` は、従来の手順で動く。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

設計書 7.1 の分類に従い、各ステップがテストを作る（名前と境界は 2 章のステップの「テスト」の項）。置き場所は、`internal/cmdrunner/runner_test.go`、`internal/fixprguard/*_test.go`、`cmd/fixprguard/main_test.go`・`docs_test.go`、`cmd/yt2column/fixpr_docs_test.go`。fake は `internal/cmdrunner/testutil/mocks.go`（`package cmdrunnertestutil`、分類 A）、`internal/fixprguard` の補助は `test_helpers.go` / `test_helpers_git.go`（`//go:build test`、分類 B。`docs/dev/developer_guide/test_organization.md`）。表のテストの行や期待値は、コードを書くときに決める。

層が重なる経路では、片方の層だけが防げる入力を作り、先に、もう片方の層だけでは止まらないことを確かめる（`CLAUDE.md`「Testing Strategy」）。

| 重なる層 | 場所 | 層だけが防げる入力 |
|---|---|---|
| nonce の確認と JSON のエスケープ | ステップ 2-4（nonce）・ステップ 5-1（JSON のエスケープ） | 前者: nonce と同じ文字列を含む本文。後者: 改行と引用符で行頭の区切りを作ろうとする本文 |
| `finish` の厳密な確認と stage の指定 | ステップ 3-5 | stage だけが防げる: グロブや pathspec の特殊な記法に見える名前のファイルを含む判定した集合 |
| `reply` の拒否の 4 つの条件 | ステップ 4-2 | 条件ごとに、その条件だけが満たされない入力 |
| 基準値との比較と起動前のダイジェストの比較 | ステップ 3-2・5-1 | 前者だけが防げる: git を起動した後にだけ現れる違い（head の移動）。後者だけが防げる: config の書き換え（git を起動しない） |
| 1 段目の指紋と 2 段目の指紋 | ステップ 2-5 | 1 段目の後で、2 段目の前にだけ変わるコメント |

### 4.2. 結合テスト

ステップ 3-7 の、一時リポジトリでの `Begin` から `Push` の直前までの通しのテスト（設計書 7.3）。ローカルの bare リポジトリを `git ls-remote` と push の相手にする。実際の GitHub と `make` は使わない。

### 4.3. 後方互換性

フェーズ 1 は `internal/mergepr` の既存のテストを変えずに通す。信頼する著者のスレッドの処理（要件書 4.4）は、フェーズ 6 の文書の切り替えまで変わらない。

### 4.4. 壊して失敗を確かめる

各フェーズの完了条件に、設計書 7.2 の一覧から選んだ対象と、それを検出するテストを書いた。フェーズ 5 とフェーズ 6 も、対象を書いた。実装者は、対象を実際に壊してテストが失敗することを確かめ、コミットメッセージに書く。壊し方が、名指しのテストの通る経路を実際に壊していることも確かめる（例えば `Fence` の nonce の確認を外すときは、nonce と同じ文字列を含む本文のテストが失敗すること）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は実行できて誤った挙動で失敗するテスト、`static` は文書のガードのテストなど実行される検査、`manual` は観察を表す。テストの名前は、2 章の対応するステップが作る。コマンドの手順（`fixpr.md`・`runpr.md`）は LLM が実行する文章なので、手順の分岐そのものはテストできない（設計書 5.2）。そのため、手順が依存するツール側の出力と停止の契約を `test` で確かめ、手順の記述を `static` で確かめ、実際の実行の観察を `manual` で補う。

| AC | 検証 | 場所 |
|---|---|---|
| AC-01〜AC-04 | test | `internal/fixprguard/trust_test.go::TestAssessAuthors` |
| AC-05 | test | `internal/fixprguard/threads_test.go::TestCommentCap` |
| AC-06 | test | `internal/fixprguard/trust_test.go::TestAssessUnverifiableAuthor`、`::TestPermissionBudget`、`::TestPermissionCause` |
| AC-07 | test | `internal/fixprguard/github_test.go::TestBeginAssessesEveryCall`、`internal/fixprguard/write_test.go::TestReplyReassesses` |
| AC-08 | test | `internal/fixprguard/threads_test.go::TestStageOneQueryHasNoBody` |
| AC-09 | test | `internal/fixprguard/github_test.go::TestBeginNeverFetchesUntrustedBody`、`::TestBeginStageTwoFingerprintMismatch` |
| AC-10 | test | `internal/fixprguard/write_test.go::TestReplyRefusesUntrustedThread` |
| AC-11 | test + static | `internal/fixprguard/github_test.go::TestBeginReportsAllReasons`。`cmd/yt2column/fixpr_docs_test.go::TestFixprDocContract/final_report` |
| AC-12 | test + static | `internal/fixprguard/github_test.go::TestBeginAcceptsNamedThread`。`TestFixprDocContract/accept_only_reported`。名指しを直前の報告に限ることは文書に依存する（設計書 5.2） |
| AC-13 | test | `internal/fixprguard/github_test.go::TestBeginAcceptChangedFingerprint`（追加・編集・削除） |
| AC-14 | test + static | `cmd/fixprguard/main_test.go::TestBeginNothingToProcess`。`TestFixprDocContract/nothing_to_process` |
| AC-15 | test + static | `internal/fixprguard/worktree_flow_test.go::TestFinishCommitSubject`（件名がラウンドの数え方と同じ形）、`internal/fixprguard/worktree_flow_test.go::TestFinishEmptyJudgedSet`（判定した集合が空のとき commit が作られない）、`cmd/fixprguard/main_test.go::TestBeginNothingToProcess`（処理するスレッドがないことの報告）。`TestFixprDocContract/round_counting` |
| AC-16・AC-17 | static | `cmd/yt2column/fixpr_docs_test.go::TestFixprDocContract/fenced_prompts`（区切りのブロックを埋め込むプロンプトの記述の有無。区切りの出力そのものは `cmd/fixprguard/main_test.go::TestBeginOutputFenced`） |
| AC-18 | test | `internal/fixprguard/fence_test.go::TestFenceRetriesOnCollision`、`::TestFenceGivesUp` |
| AC-19 | test | `internal/fixprguard/write_test.go::TestReplyBodyVerbatim`、`::TestEditPRVerbatim` |
| AC-20 | test | `cmd/fixprguard/main_test.go::TestBeginOrder`（`gh` を呼ばない）、`internal/fixprguard/worktree_test.go::TestBeginRejectsDirtyWorkTree` |
| AC-21 | test | `internal/fixprguard/changes_test.go::TestEnumerateChanges`、`internal/fixprguard/scope_test.go::TestScopeRules`、`internal/fixprguard/worktree_flow_test.go::TestFinishAbsorbsFormatting` |
| AC-21a | test | `internal/fixprguard/worktree_flow_test.go::TestFinishStopsOnJudgedSetMismatch`、`::TestFinishStopsWhenBuildChecksRewrite`、`::TestFinishStagesJudgedPathsOnly`、`::TestFinishStageRollback`、`::TestFinishPostCommitTreeCheck`、`::TestRoundStopsAtRightPlace` |
| AC-22・AC-23 | test | `internal/fixprguard/worktree_test.go::TestInspectInScope`、`::TestInspectListsOutOfScope`、`cmd/fixprguard/main_test.go::TestInspectExitCode` |
| AC-24・AC-25 | test + static | `internal/fixprguard/worktree_flow_test.go::TestFinishAfterOutOfScope`、`::TestInspectLeavesWorkTree`。`TestFixprDocContract/out_of_scope_stop`（「続けない」と答えたときに止まる記述）。回答の分岐はセッションが行う（設計書 5.2） |
| AC-26 | test | `internal/fixprguard/gitstate_test.go::TestBaselineDetectsChange`、`::TestBaselineIgnoresIndexAndFetchHead`、`::TestBaselineMalformedRejected`、`::TestGitNotLaunchedAfterConfigChange`、`internal/fixprguard/worktree_flow_test.go::TestPushRechecks`、`cmd/fixprguard/main_test.go::TestInspectChecksGitStateBeforeGh` |
| AC-27 | test + static | `internal/fixprguard/github_test.go::TestHeadRequiresSameRepository`、`::TestBeginAndListCheckForkFirst`。`TestRunprDocContract/head_before_checkout` |
| AC-28〜AC-31 | test + static + manual | ツール側の停止の契約は `cmd/fixprguard/main_test.go::TestStopKinds`（`runpr.md` がマージせずに止まる条件の入力）。手順の記述は `TestRunprDocContract/untrusted_question`・`/stop_without_merge`。実際の実行の観察はステップ 6-6 |
| AC-32 | test + static | `internal/fixprguard/github_test.go::TestListOmitsBodies`。`TestRunprDocContract/no_body_fetch` |
| AC-33 | static | `TestRunprDocContract/auto_fixpr_condition`（`runpr.md:212-217` の条件の文面と同じであること。ステップ 6-3 の前に写した文面を固定する） |
| AC-34 | static | `TestFixprDocContract/people_answered_scope` |
| AC-35 | static | `TestContextAndCommandsAgree` |
| AC-36 | static | `TestAuthorizationSections`（`fixpr.md` と `runpr.md` の両方） |
| AC-37 | static | `TestGlossaryTerms`（日本語の語と英訳の両方の存在。訳語の妥当性はレビューで確かめる） |

5 章の `static` の名前はすべて `cmd/yt2column/fixpr_docs_test.go`（ステップ 6-5）にある。ほかに、`cmd/fixprguard/docs_test.go` の `TestGuideCoversStopKindsAndScope`（ステップ 5-3）と `TestDocumentedInvocationsParse`（ステップ 6-5）が、ガイドと文書の呼び出しをツールの実際の挙動に照らして補う。`manual` は `test` や `static` の代わりにしない。

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 対策 |
|---|---|
| ツールを導入する PR の間（PR-1〜PR-5）は、この PR 自体の `/fixpr` が従来の手順で動き、防御がない | 設計書 8 章が受け入れている。PR-1〜PR-5 は、コマンド文書を変えないので、既存の挙動に影響しない |
| PR-6 を戻すと、文書とガードと用語集の整合が崩れる | PR-6 は 1 つの単位として戻す（`git revert` で PR のマージコミットごと。文書・ガードのテスト・用語集・`doc-guard` の対象・`CLAUDE.md` の記述が含まれる）。ガードのテストが文書を縛っているので、一部だけ戻すと CI が失敗する |
| 利用者の git のグローバルの設定（署名など）が、一時リポジトリのテストを乱す | ステップ 3-2 の補助で、`HOME` を一時ディレクトリにし、リポジトリ内の設定で無効にする。以降のテストはすべて同じ補助を使う |
| 同じ `protocol` の番号のまま規則が変わり、古いバイナリが残る | 設計書 5.3 が受け入れている。ツールを変える PR のマージの後は、再インストールが要ることを `fixprguard_guide` に書く |
| フェーズ 6 の前提（インストール）が満たされないまま始まる | フェーズ 6 の先頭の前提の確認で止める |

### 6.2. スケジュールのリスク

-   PR-2 と PR-3 は大きい（PR-3 は 7 ステップ）。レビューの量を減らすため、各ステップの完了のたびにコミットを分ける。PR を分けると、基準値の比較を使う側が後になる（1.4）。

## 7. 実装チェックリスト (Implementation Checklist)

PR ごとにまとめる。各ステップの詳細は 2 章のチェックボックスで追う。

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3）
- [x] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6）
- [x] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7）
- [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2）
- [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4）
- [ ] 利用者が `make install-fixprguard` を実行した
- [ ] PR-6 マージ済み（対象ステップ: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6）
- [ ] 要件書の AC-01〜AC-37 のすべてが、5 章の検証でカバーされている

## 8. 成功基準 (Success Criteria)

-   **機能:** 要件書の AC-01〜AC-37 を、5 章の検証が満たす。信頼する著者のスレッドの処理は、今までどおりである（要件書 4.4）。
-   **品質:** `make test && make lint` が各 PR で通る。新しいテストは、壊して失敗することを確かめ、コミットメッセージに書く。
-   **セキュリティ:** 想定外の著者のスレッドの本文が、ツールのプロセス、エージェント、`/fixpr` と `/runpr` のセッションのどれにも入らない（AC-09・AC-32）。返信と PR の説明の文面が、シェルのコマンドにならない（AC-19）。
-   **文書:** `fixprguard_guide`（日英）、`package_reference.md`、`CLAUDE.md`、用語集が実装と一致する。

## 9. 次のステップ (Next Steps)

1.  この計画書を人のレビューにかけ、`approved` にする（`/runpr` が承認を記録する）。
2.  承認の後、`/runplan 0030` で PR-1 から実装する。PR-5 のマージの後は、利用者が `make install-fixprguard` を実行してから PR-6 に進む。
