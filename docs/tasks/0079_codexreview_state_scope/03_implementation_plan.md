# 実装計画書：codexreview の状態の比較を、この worktree の状態と HEAD のブランチに絞る

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-10 |
| Review date | 2026-11-10 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装概要 (Implementation Overview)

### 1.1 目的と実装方針

[要件定義書](01_requirements.md)（以下、要件書）の AC-01〜AC-12 を、[設計書](02_architecture.md)（アーキテクチャ設計書。以下、設計書）のとおりに実装する。`codexreview` が Codex CLI の実行の前後で比べるものを、この worktree の `git status`、HEAD のブランチの名前、HEAD が指す commit、HEAD のブランチの ref に絞る。あわせて、共通手順 `.claude/commands/_lib/local-cli-review.md` の手順 3 の文を改める。

-   設計書 8 章のとおり、実装は 1 本の PR にする。PR の中は、設計書 8 章の順序の目安に合わせて 3 つのフェーズに分け、フェーズごとにコミットする。各フェーズの終わりに、そのフェーズの完了の条件（`make fmt`・`make test`・`make lint`、フェーズ 2 では `make deadcode` も）が通る状態にする。
-   設計の内容（比べるものの型、HEAD の読み取りの出力の形、比較の規則、消すもの、テストの入力）は設計書の節を参照し、本書では繰り返さない。本書は、どのファイルで何をし、何を実行して確かめるかを定める。
-   F-NNN・AC-NN は要件書の項番、H-NN は [design_handoff.md](design_handoff.md) の項番を指す。

### 1.2 計画の際に決めたこと（2026-11-10）

設計書が計画に任せた点のうち、結果が大きく変わる選択肢はなかったので、人には尋ねていない。次のことは、設計書と今のコードから導いた（PR のレビューで異論があれば改める）。

-   **コミットの分け方は、フェーズごとにする（フェーズ 1：HEAD の読み取り、フェーズ 2：比べるものの絞り込み、フェーズ 3：文書と確かめのスクリプト）。** フェーズの順は設計書 8 章の順序の目安に従い、次の 2 点を目安と違う組に入れる。理由も添える。このほか、設計書 8 章にない確かめのスクリプト（`_checks/check.sh`）は、フェーズ 3 に置く。
    -   HEAD の読み取りの失敗の `Run` を通したテスト（設計書 8 章では 2 つ目の組）は、フェーズ 1 に入れる。これらのテストは、フェーズ 1 で加える `parseHead` を通る経路を確かめるもので、比べるものの範囲とは独立に確かめられる。`parseHead` を通さないと失敗することも、フェーズ 1 のコミットで記録できる。
    -   比べるものを述べるコードのコメントと `package_reference.md`（設計書 3.5 の表のうち、`state.go`・`types.go`・`package_reference.md` の行。設計書 8 章では 3 つ目の組）は、そのコメントが述べる振る舞いを変えるフェーズ（フェーズ 1 か 2）で改める。コードとコメントが食い違うコミットを作らないためである。フェーズ 3 に残すのは、共通手順の文、ADR-0013、確かめのスクリプトである。

    ADR の判断：ADR は作らない。本タスクの中で閉じる、作業の順序の決定である。
-   **HEAD の読み取りの失敗の `Run` を通したテスト（設計書 7.1）は、既存のテストに行として加える。** 実行の前の行は `TestRunPreconditions` に、実行の後の 2 行は `TestRunAfterStateUnreadable` に加える。後者は今は 1 つの場合だけを確かめる関数なので、表の形にする。理由：どちらの関数も、すでに確かめている結果の種類（`precondition` と、結果なし）が加える行の期待と同じで、準備の手順も同じである。ADR の判断：ADR は作らない。テストの置き場所の決定である。
-   **比べないもののテスト（設計書 7.1）は、`internal/codexreview/run_test.go` に新しい関数 `TestRunIgnoresSharedRefs` として置く。** 理由：期待する結果（`no_findings`）が `TestRunStateChanged`（`state_changed`）と逆で、表の項目（`changed`・`changed_refs`）を持たないため、同じ表に混ぜない。ADR の判断：ADR は作らない。テストの名前と置き場所の決定である。
-   **変更の前後の関係を述べる部分は、`_checks/check.sh` の `diff` の確かめにする。** 対象は、AC-11 のプロトコルの番号と結果のメンバの名前、AC-12 の「`stop` のときに報告するものは変えない」、AC-04 のサブモジュールを含む `status` の読み方（`statusArgs`）、消した識別子が残っていないこと、ADR-0013 のずれの記述の除去である。理由：どれも「変えない」「除いた」という変更の前後の関係であり、恒久のテストにすると、古い値の写しを持つことになる（`requirements_process.md` §4）。設計書 7.3 が読んで確かめるとした、`statusArgs` と消える定数・変数・フィールドの 2 点も、このスクリプトで確かめる。ADR の判断：ADR は作らない。確かめ方の決定である。
-   **設計書 7.1 のテストの行に、次の行を足す。** `TestRunStateChanged` に 2 行（同じ commit の別のブランチへ switch する行と、ほかの worktree が中身の違う commit で HEAD のブランチを動かす行）、`TestRunIgnoresSharedRefs` に 1 行（stash の項目を入れ替える行）、`state_test.go` に `describeRefs` の単体テストを足す。理由：設計書 7.1 の行だけでは、AC-05 の「ブランチの名前だけが変わる」場合（commit は同じ）、AC-09 の `status`・`head`・`refs` の 3 つがそろうときの順序、AC-03 の「入れ替わった」場合を確かめる行がない。また、`TestRunStateChanged` から 5 行を移すと、`describeRefs` が ref だけを示す分岐（`state.go:159-160`。worktree の一覧を読めないか、どの worktree も checkout していないとき）を通るテストがなくなる。設計書 1.2 は `describeRefs` が ref だけを示す扱いを残すと決めているので、単体テストで確かめ続ける。ADR の判断：ADR は作らない。テストを足すだけの決定である。

`implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-04 は、設計書の付録 B に記録されている。本書は ADR を新しく作らず、ADR の決定も変えない。ADR-0013 のずれの記述を除くのは、実装の PR の編集上の訂正である（ステップ 3-1）。

### 1.3 既存コード調査結果

調べた基準のコミットは `9f57e8c3` である。設計書は、その前のコミット `76e63cb8` で既存のファイルを確かめた。`git diff --stat 76e63cb8 9f57e8c3 -- internal/codexreview cmd/codexreview .claude/commands/_lib/local-cli-review.md docs/dev/conventions/package_reference.md docs/dev/adr/0013-review-setup.md` で確かめたところ、この間に変わった関係するファイルは `docs/dev/conventions/package_reference.md` だけで、変わったのは `internal/config` などの行であり、`internal/codexreview` の行は変わっていない。そのため、設計書の `file:line` は `9f57e8c3` でもそのまま当てはまる。下表の行番号も `9f57e8c3` での行番号である。

| 対象 | 今あるもの | 変えること |
|---|---|---|
| 状態の読み取り | `internal/codexreview/state.go:55-77` の `readState` が、`status`・HEAD・すべての参照・stash を読む。`readHead`（`state.go:81-95`）は 2 つの出力を確かめずにつなぐ | 設計書 3.1〜3.3 のとおり、`status` と HEAD だけを読み、HEAD の読み取りの出力の形を確かめる |
| 比較 | `changedItems`（`state.go:119-144`）。呼び出しは `run.go:422-425` だけである（`rg -n "changedItems\|describeRefs" --glob '*.go'` で確かめた） | 形を変えずに、設計書 3.3 の表の規則にする。呼び出し側は変えない |
| 残すもの | `describeRefs`・`parseWorktrees`（`state.go:150-182`） | 変えない（設計書 3.3）。ref だけを示す分岐（`state.go:159-160`）を今通るテストは、`TestRunStateChanged` の `branch`・`deleted_branch`・`tag` の行だけである（`rg -n "describeRefs\|parseWorktrees\|ChangedRefs" --glob '*_test.go'` で確かめた）。これらの行を移すので、単体テストを足す（本書 1.2）。`internal/cmdrunner/testutil` の `Fake`（`internal/cmdrunner/testutil/mocks.go:34-58`）を `git` の `runner` に使える |
| テストの偽の `codex` | `Behavior.Git`・`Behavior.GitDir`（`internal/codexreview/testutil/helpers.go:197-199`）は、実行の間に `GitDir` で git のコマンドを順に実行する（`helpers.go:301-306`）。`Behavior.Removes`（`helpers.go:196`）はファイルを消す | そのまま使う。新しいテスト用の仕組みは作らない（設計書 7.1） |
| テストの準備 | `newFixture`（`run_test.go:48-57`）、`Repo.Git`（`helpers.go:548-551`）、`mustRun`・`assertKind`（`run_test.go:70-86`）、`shortenTimeouts`（`internal/codexreview/test_helpers.go`） | 再利用する。新しい補助のファイルは作らない |
| `TestRunStateChanged` | `run_test.go:456-594`。行 `branch`・`deleted_branch`・`tag`・`stash`・`branch_in_another_worktree`（`run_test.go:501-550`）が比べなくなるものを確かめている | 設計書 7.1 のとおり、5 行を `TestRunIgnoresSharedRefs` に移して期待を逆にし、設計書 7.1 の 4 行と本書 1.2 の 2 行を加える |
| `TestRunPreconditions` | `run_test.go:158-299`。実行の前の状態の読み取り（`run.go:307-309`）は `codex --version`（`run.go:295`）の後に行うので、そこで止まる行は `versionStarted: true` になる | HEAD が `refs/heads/` の外を指す行を加える |
| `TestRunAfterStateUnreadable` | `run_test.go:688-697`。git を消す 1 つの場合だけを確かめる | 表の形にし、2 行を加える |
| `cmd/codexreview` | `TestOutputIsOneJSONObject`（`cmd/codexreview/main_test.go:180-205`）の `state_changed` の行は、ブランチ `made-by-codex` を作る（`main_test.go:188`） | 入力を、この worktree で空の commit を作る操作に替える |
| 文書のガード | `TestDocumentedProtocolMatches`（`cmd/codexreview/docs_test.go:158`）が共通手順の `--protocol` を `codexreview.Protocol` と照合する。`TestProcedureNamesKinds`（`docs_test.go:311`）が、共通手順の表の `state_changed` の行の終了コード `3` と扱い `stop` を確かめる。`TestADRRecords`（`cmd/yt2column/adr_test.go:434`）が ADR と索引を確かめる | 変えない。AC-11・AC-12 の検証に使う |
| lint | `.golangci.yml` は `unused`（使われない定数・変数・関数・型）を有効にしている（`.golangci.yml:13`） | 消し忘れの一部は `make lint` で見つかる。消したことは `_checks/check.sh` でも確かめる |

**消すものと置き換えるものの台帳。** 下表を正とし、本書のほかの箇所はこれに従う。消すものは設計書 3.3 の表のとおりである。`rg -n "itemStash|refsArgs|stashArgs|errRefsUnreadable|parseRefs" --glob '*.go'` を `9f57e8c3` で実行し、使われているのは `state.go` の中と、`run_test.go:536`（`stash` の行の `itemStash`）だけであることを確かめた。

| 今の名前 | 本タスクの後 |
|---|---|
| `gitState.head`（`state.go:49`） | `gitState.branchRef` と `gitState.commit` に分ける（設計書 3.1） |
| `readHead(ctx, g) (string, error)`（`state.go:81`） | `readHead(ctx, g) (branchRef, commit string, err error)`。形の確かめは新しい `parseHead` が行う（設計書 3.2） |
| `errRefsUnreadable`（`state.go:99`） | 消す。代わりに `errHeadUnreadable` を加える（設計書 4 章） |
| `parseRefs`（`state.go:102-115`） | 消す |
| `itemStash`（`state.go:19`） | 消す |
| `refsArgs`・`stashArgs`（`state.go:42-43`） | 消す |
| `gitState.refs`・`gitState.stash`（`state.go:50-51`） | 消す |

**比べるものを述べる記述。** `rg -n "stash|branches and tags|for-each-ref"` と `rg -ln "state_changed|changed_refs" --glob '!docs/tasks/**'` を `9f57e8c3` で実行した。`docs/tasks/` の外で改める記述は、設計書 3.5 の表と 3.4 の文だけである。`run.go:207-220` の `Run` のコメントは変えない（設計書 3.5）。`cmd/codexreview/main.go` は種類と終了コードを述べるだけで、比べるものを述べていない。

**外部の前提の確かめ（git 2.50.1、scratchpad の一時リポジトリで実行）。**

-   HEAD を `git symbolic-ref HEAD refs/tags/x` でタグに向けた後も、`git status --porcelain=v1`、`git rev-parse --show-toplevel`、`git rev-parse --verify HEAD`、`git merge-base --is-ancestor HEAD HEAD`、`git diff --name-only -z --no-renames <BASE> HEAD --` は終了コード 0 で、`git symbolic-ref -q HEAD` は終了コード 0 で `refs/tags/x` を出した。そのため、`TestRunPreconditions` に加える行は、worktree の根の確かめ（`run.go:268`）、`BASE` の確かめ、`git status` の確かめ、`readChangedFiles`（`run.go:299`）を通り、実行の前の状態の読み取りで止まる。タグは HEAD の commit を指す軽量タグにする。
-   HEAD のブランチの ref を `git update-ref -d` で消した後、`git rev-parse --verify HEAD` は `fatal: Needed a single revision` で終了コード 128 になり、`git status` は終了コード 0 だった。そのため、実行の間に ref を消す行は、実行の後の状態の読み取りの失敗になる（設計書 1.2 と同じ結果）。

## 2. 実装手順 (Implementation Steps)

### フェーズ1：HEAD の読み取りと形の確かめ

#### ステップ 1-1：HEAD の読み取りに形の確かめを加える

**変更するファイル:** `internal/codexreview/state.go`、`internal/codexreview/state_test.go`（新規）、`internal/codexreview/run_test.go`

-   [ ] `errHeadUnreadable` と `parseHead` を加え、`readHead` が `parseHead` を通して `branchRef` と `commit` を返すようにする（設計書 3.2、4 章）。エラーの文には、コマンド名（`rev-parse` か `symbolic-ref`）を前に付け、元のエラーとして `errHeadUnreadable` を含めて返す。git の出力の中身はエラーの文に入れない。
-   [ ] `gitState.head` を `branchRef` と `commit` に分ける（台帳）。このフェーズの `changedItems` は、`branchRef` か `commit` が違えば `head` とする。すべての参照と stash の比較は、このフェーズでは変えない。`readHead` のコメントを、形の確かめを述べる形に改める（設計書 3.5）。
-   [ ] `state_test.go` に `describeRefs` の単体テスト `TestDescribeRefs` を加える。`internal/cmdrunner/testutil` の `Fake` を `runner` にした `git` を使い、`Fake` の手順には `git.run` が前に付ける `cmdrunner.GitHardening()` の引数も含める。確かめるのは、`worktree list` が失敗するときは ref だけが返ること、どの worktree も checkout していない ref は ref だけで返ること、checkout している ref は worktree を併記して返ることである（本書 1.2）。
-   [ ] `state_test.go` に `TestParseHead` を加え、設計書 3.2 の表の受け入れる形と受け入れない形を表で確かめる。受け入れない形には、設計書 7.1 の形に加えて、64 桁の 16 進数、16 進数でない小文字（`g` など）、空の commit の出力、detached でないのに空のブランチの出力を入れる。規則ごとに、その規則が適用される出力（commit の出力とブランチの出力）のそれぞれで、規則に反する形を 1 行以上置く。受け入れない形では `errors.Is(err, errHeadUnreadable)` と、受け入れない出力に入れた目印の文字列がエラーの文にないことを確かめる。ファイルの先頭は、同じパッケージのほかのテストと同じく `//go:build test` にする。
-   [ ] `TestRunPreconditions` に、`git symbolic-ref` で HEAD を、あらかじめ HEAD の commit を指すタグへ向けておく行を加える。期待は `precondition`/`git_failed` で、偽の `codex` の記録は `version` だけ（レビューの実行がない）である。
-   [ ] `TestRunAfterStateUnreadable` を表の形にし、今の場合（git を消す）のほかに、実行の間に HEAD をタグへ向ける行と、実行の間に `git update-ref -d` で HEAD のブランチの ref を消す行を加える。どの行も、`errAfterStateUnreadable` を含むエラーと空の `Result` を確かめる。HEAD をタグへ向ける行は、`errHeadUnreadable` を含むことも確かめる。
-   [ ] 失敗することを確かめ、コミットメッセージに書く：`readHead` が `parseHead` を通さないようにすると、HEAD をタグへ向ける 2 つの行が失敗すること。`parseHead` の規則を 1 つずつ外すと、`TestParseHead` の対応する行が失敗すること。`readHead` が `rev-parse` の失敗を握りつぶし、`parseHead` を通さずに空の commit を返すようにすると、HEAD のブランチの ref を消す行が失敗すること。

**完了の条件:** `make fmt`・`make test`・`make lint` が通る。

### フェーズ2：比べるものを絞る

#### ステップ 2-1：比べるものと比較を変え、使われなくなるコードを消す

**変更するファイル:** `internal/codexreview/state.go`、`internal/codexreview/types.go`、`internal/codexreview/run_test.go`、`cmd/codexreview/main_test.go`、`docs/dev/conventions/package_reference.md`

-   [ ] `readState` が `status` と HEAD だけを読むようにし、`changedItems` を設計書 3.3 の表の規則にする。違いのあった ref は 0 個か 1 個で、並べ替えはしない。
-   [ ] 比べるものを述べるコメントと文書を、設計書 3.5 の表のとおりに改める：`state.go` の `gitState`・`changedItems`・読む git のコマンド（`state.go:35-39`）のコメント、`types.go` のパッケージのコメントと `Changed`・`ChangedRefs` のコメント（型と JSON の名前は変えない）、`package_reference.md` の `internal/codexreview` の行。
-   [ ] 定数 `itemStash` を消す。
-   [ ] 変数 `refsArgs` を消す。
-   [ ] 変数 `stashArgs` を消す。
-   [ ] `gitState` のフィールド `refs` と `stash` を消す。
-   [ ] `errRefsUnreadable` を消す。
-   [ ] `parseRefs` を消す。
-   [ ] `TestRunStateChanged` の行 `branch`・`deleted_branch`・`tag`・`stash`・`branch_in_another_worktree` を、`TestRunIgnoresSharedRefs`（下の項目）に移す。
-   [ ] `TestRunStateChanged` に、設計書 7.1 の 4 行（HEAD のブランチをほかの worktree が動かす、detached のまま commit、detached からブランチへ、別の commit のブランチへ checkout）を加える。
-   [ ] `TestRunStateChanged` に、本書 1.2 の 2 行を加える。
    -   あらかじめ HEAD と同じ commit に作ったブランチへ、実行の間に switch する行。期待は `head` だけで、`changed_refs` は空である。
    -   ほかの worktree が、追跡しているファイルを変えて commit し、HEAD のブランチをその commit に向け、同時にタグを 1 つ作る行。期待は `status`・`head`・`refs` の順で、`changed_refs` はこの worktree を併記した HEAD のブランチだけである。
-   [ ] `TestRunStateChanged` の行を書くときの注意を守る。偽の `codex` は決まった引数で git を実行するので、HEAD のブランチを動かすには、ほかの worktree で `update-ref refs/heads/main HEAD` を実行し、新しい値を commit ID ではなく、その worktree の `HEAD` で渡す（`NewRepo` はブランチ `main` で作る。`helpers.go:536`）。`changed_refs` が空になる行では、空の配列（nil でない）であることも確かめる。関数のコメントを、比べるものの変化を確かめる表だと読める形に改める。
-   [ ] `TestRunIgnoresSharedRefs` を加え、設計書 7.1 の「比べないもののテスト」の 9 行と、stash の項目を入れ替える行（あらかじめ作った stash を消し、別の変更を stash する）で、結果の種類が `no_findings` であることを確かめる。
-   [ ] `TestOutputIsOneJSONObject` の `state_changed` の行の入力を、この worktree で空の commit を作る操作に替える（設計書 7.1）。
-   [ ] 失敗することを確かめ、コミットメッセージに書く（設計書 7.1「失敗することの確かめ」の 1 つ目と 2 つ目に、本書で足した行の分を加えたもの）。
    -   比べるものを元の広さに戻すと、`TestRunIgnoresSharedRefs` の各行が失敗する。
    -   `refs` の条件から「`branchRef` が前後で同じ」を外すと「別の commit のブランチへ checkout」の行が、「空でない」を外すと「detached のまま commit」の行が失敗する。`refs` を導くこと自体をやめると、`commit` の行と「HEAD のブランチをほかの worktree が動かす」の行が失敗する。
    -   `head` の条件を `commit` の比較だけにすると、「同じ commit の別のブランチへ switch」と「detached からブランチへ」の行が失敗する。`branchRef` が空かどうかだけの比較にすると、「同じ commit の別のブランチへ switch」の行が失敗する。
    -   `status` と `head` を並べる順を入れ替えると、「ほかの worktree が中身の違う commit で動かす」の行が失敗する。
    -   `TestOutputIsOneJSONObject` の `state_changed` の行は、偽の `codex` の空の commit を外すと失敗する。
-   [ ] `make deadcode` を実行し、何も報告されないことを確かめる。

**完了の条件:** `make fmt`・`make test`・`make lint`・`make deadcode` が通る。

### フェーズ3：共通手順の文、ADR-0013、確かめのスクリプト

#### ステップ 3-1：共通手順の文と ADR-0013 を改め、変更の前後を確かめる

**変更するファイル:** `.claude/commands/_lib/local-cli-review.md`、`docs/dev/adr/0013-review-setup.md`、`docs/tasks/0079_codexreview_state_scope/_checks/check.sh`（新規）、本書（7 章の記録）

-   [ ] `local-cli-review.md` の手順 3、`stop` の説明の最後の文（「A `state_changed` caused by another session's commit or stash in another worktree ... is still a stop: the user decides.」）を、設計書 3.4 の 2 点（ほかの worktree の操作でも HEAD のブランチが動けば `state_changed` になって止めること、ほかのブランチ・タグ・stash の変化では `state_changed` にならないこと）を述べる文に書き換える。`_checks/check.sh` がこの文を切り取れるよう、書き換えた文も「A `state_changed` caused by」で始める。
-   [ ] ADR-0013 の決定の 4 つ目の項目から、ずれの記述の 3 文（「この決定は、まだ…」から「…#778 で扱う。」まで）を除く。PR を作る前に、`docs/dev/conventions/adr.md`「Discrepancy statements」のとおり `git grep -nE 'issues/778([^0-9]|$)' origin/main -- docs/dev/adr/` でほかにずれの記述がないかを確かめる。編集上の訂正なので、コミットメッセージに correction line（`Editorial correction: docs/dev/adr/0013-review-setup.md: ...; no decision changed`）を書く。PR の説明の correction paragraph は、`/runplan` が PR を作るときに書く。
-   [ ] `_checks/check.sh` を書く。引数に変更の前と後の 2 つのコミットを受け取り、次を確かめて、項目ごとに結果を 1 行出す（手本はタスク 0078 の `docs/tasks/0078_adr0024_server_env_result/_checks/check.sh`）。
    -   `internal/codexreview/types.go` の `Protocol` の定義の行が前後で同じであること（AC-11）。
    -   `types.go` の `Result` の型の各フィールドの JSON の名前の並びが、前後で同じであること（AC-11）。
    -   `internal/codexreview/state.go` の `statusArgs` の定義の行が前後で同じであること（AC-04）。
    -   `local-cli-review.md` で、手順 3 の `stop` の箇条を前後とも 1 つの段落につなぎ、「A `state_changed` caused by」から箇条の終わりまでを切り取ったとき、残りのファイル全体が前後で同じであり、切り取った部分は前後で違うこと（AC-12）。行の折り返しが変わっても比べられるようにするためである。
    -   ADR-0013 で、決定の 4 つ目の項目からずれの記述の 3 文を除いた行が後の行と同じで、ほかの行は前後で同じであること。
    -   台帳の消す名前（`itemStash`・`refsArgs`・`stashArgs`・`errRefsUnreadable`・`parseRefs`）が、後のコミットの `internal/codexreview` に単語として現れないこと。フィールド `refs`・`stash` は、`refs` がほかの識別子の引数名にも使われるので、後のコミットの `state.go` の `type gitState struct { … }` の本体だけを見て、ないことを確かめる。
-   [ ] `_checks/check.sh` の各項目について、確かめる対象を壊したコミット（一時的なもの）を後のコミットとして渡すと失敗することを、1 回ずつ確かめる。確かめたことをコミットメッセージに書く。
-   [ ] PR のブランチで `sh docs/tasks/0079_codexreview_state_scope/_checks/check.sh $(git merge-base origin/main HEAD) HEAD` を実行し、出力と 2 つのコミット ID を 7 章の記録に書く。

PR の説明には、入れ直しの前後の食い違いの見分け方（設計書 6.2：`changed` に `stash` が入るか、`changed_refs` に HEAD のブランチ以外の ref が入れば、入れ直す前のツールである）と、マージの後に `make install-codexreview` で入れ直すことを書く。`/runplan` が PR を作るときに書くので、チェックボックスにはしない。

**PR を作る直前の確かめ直し。** ステップ 3-1 の実行の記録の後に、`/runplan` のレビューや `.claude/commands/_lib/task-pr-handoff.md` Part B の修正で、`_checks/check.sh` が確かめるファイルを変えるコミットが入ったときは、スクリプトを実行し直して 7 章の記録を改める。

**完了の条件:** `make fmt`・`make test`・`make lint` が通り（`TestDocumentedProtocolMatches`・`TestProcedureNamesKinds`・`TestADRRecords` を含む）、`_checks/check.sh` が `OK` で終わる。

### PR-1 作成ポイント: narrow codexreview state comparison

**対象ステップ**: 1-1 / 2-1 / 3-1

**推奨タイトル**: `fix(0079): codexreviewの状態の比較をこのworktreeとHEADのブランチに絞る`

**レビュー観点**: 比べるものの範囲と受け入れる見逃しの範囲 / HEAD の読み取りの形の確かめ / テストの行と失敗することの確かめ / 共通手順の文と ADR-0013 の訂正

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-1 が、Codex に sandbox を破られたときの検出の範囲という開発用ツールの守りを狭める。これは mkplan2 の手順 4 が挙げる security-gate の条件に当たり、正しさは AC-05・AC-09 の条件の組み立てに懸かる。

- [ ] グリーンゲート（`make test`・`make lint`）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1 順序

| マイルストーン | フェーズ | 成果物 |
|---|---|---|
| M1 | フェーズ 1 | `parseHead`・`errHeadUnreadable`、`TestParseHead`、`describeRefs` の単体テスト、HEAD の読み取りの失敗の 3 行 |
| M2 | フェーズ 2 | 比べるものの絞り込み、使われなくなるコードの削除、コメントと `package_reference.md`、`TestRunStateChanged` の変更、`TestRunIgnoresSharedRefs`、`TestOutputIsOneJSONObject` の変更 |
| M3 | フェーズ 3 | 共通手順の文、ADR-0013 のずれの記述の除去、`_checks/check.sh` とその実行の記録 |

フェーズ 1 は、比べるものの範囲を変えずに HEAD の読み取りだけを変える。フェーズ 2 で振る舞いが変わる。フェーズ 3 は文書と確かめのスクリプトだけで、ツールの振る舞いを変えない。

### 3.2 PR 構成

設計書 8 章のとおり、1 本の PR にする。`codexreview` の変更と共通手順の文のどちらか一方だけが `main` にあると、文書とツールの振る舞いが食い違うためである。3 つのフェーズを合わせても変更は `internal/codexreview` の 3 ファイルとそのテスト、文書 3 つ、確かめのスクリプトに収まり、1 本でレビューできる大きさである。テストの行の移動と追加、失敗することの確かめが多いので、PR はフェーズごとのコミットの単位で読めるようにする。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 2-1 / 3-1 | codexreviewの状態の比較をこのworktreeとHEADのブランチに絞る | frontier-required |

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** `parseHead` は、出力を直接与える `TestParseHead` で確かめる（設計書 7.1）。実際の git では作れない出力（改行がない、大文字を含む、NUL を含むなど）は、ここでだけ確かめる。
-   **`Run` を通したテスト:** 比べるものと比べないものは、実際の git と偽の `codex` で、Codex CLI の実行の間に git の操作を起こして確かめる（`TestRunStateChanged`・`TestRunIgnoresSharedRefs`・`TestRunPreconditions`・`TestRunAfterStateUnreadable`）。各テストは `t.TempDir` の中のリポジトリを使い、ほかの worktree も `t.TempDir` の中に作るので、テストどうしで状態が残らない。
-   **`cmd/codexreview` のテスト:** `TestOutputIsOneJSONObject` で、`state_changed` の終了コード `3` と、`--protocol 1` の結果の 1 つの JSON を確かめ続ける。
-   **結合テスト:** なし。実際の Codex CLI は実行しない（設計書 7.2）。
-   **互換性:** 結果の形とプロトコルの番号を変えないことは、`_checks/check.sh` で変更の前後を比べて確かめる。入れ直す前のツールとの組み合わせはテストしない（設計書 6.2）。
-   **失敗することの確かめ:** 各フェーズの最後の項目のとおり、壊して失敗することを確かめ、コミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。
-   **テストの削除:** `TestRunStateChanged` から移す 5 行は、同じ入力の期待を逆にして移すもので、テストの削除ではない（設計書 7.1「テストの削除の扱い」）。移した行だけが通っていた `describeRefs` の ref だけを示す分岐は、フェーズ 1 で加える単体テストが確かめる。ほかにテストは消さない。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 比べるものを絞りすぎ、この worktree の変化を見逃す | `TestRunStateChanged` の今の行を残し、HEAD のブランチをほかの worktree が動かす行などを加える。`refs` の各条件を外すと失敗することを確かめる（ステップ 2-1） |
| HEAD の読み取りの出力を補って読む | `parseHead` で形を確かめ、受け入れない形を `TestParseHead` で確かめる。HEAD が `refs/heads/` の外を指すときは、実行の前なら Codex CLI を実行しない（ステップ 1-1） |
| 共通手順の文とツールの振る舞いが食い違う | 両方を同じ PR に入れる（設計書 8 章）。入れ直すまでの食い違いの見分け方は PR の説明に書く（ステップ 3-1） |
| 一部だけ戻すと、文書とツールが食い違う | 戻すときは PR のスカッシュコミットを丸ごと戻す。文書だけ、またはツールだけを戻さない |
| ADR-0013 のずれの記述を残す、または除きすぎる | `_checks/check.sh` で、除いたのが 3 文だけであることを確かめる（ステップ 3-1） |

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 2-1 / 3-1）
-   [ ] 7 章のすべての AC の検証が通り、`_checks/check.sh` の実行を記録した

## 7. 受け入れ条件の検証 (Acceptance Criteria Verification)

下表の「新規」は実装で加えるテストや行である。型は、`test` が振る舞いを確かめるテスト、`static` がガードのテスト、`diff` が `_checks/check.sh` による変更の前後の比較、`manual` が PR のレビューで読んで確かめることを表す。テストのパスはすべて `internal/codexreview/` か `cmd/codexreview/` の下である。

| AC | ステップ | 型と実行する検証 |
|---|---|---|
| AC-01 | 2-1 | test: `internal/codexreview/run_test.go::TestRunIgnoresSharedRefs`（新規、ブランチを作る・消す・動かす・ほかの worktree が checkout しているブランチを動かす行） |
| AC-02 | 2-1 | test: `run_test.go::TestRunIgnoresSharedRefs`（新規、タグを作る・動かす・消す行） |
| AC-03 | 2-1 | test: `run_test.go::TestRunIgnoresSharedRefs`（新規、stash を加える・消す・入れ替える行） |
| AC-04 | 2-1 / 3-1 | test: `run_test.go::TestRunStateChanged`（`tracked_file`・`untracked_file_hidden_by_configuration`）。diff: `_checks/check.sh`（`statusArgs` の行が前後で同じ。サブモジュールの変更を含む読み方を変えていない） |
| AC-05 | 2-1 | test: `run_test.go::TestRunStateChanged`（`detach_at_same_commit`、新規の detached のまま commit・detached からブランチへ・同じ commit の別のブランチへ switch・別の commit のブランチへ checkout の行） |
| AC-06 | 2-1 | test: `run_test.go::TestRunStateChanged`（`commit`、新規の HEAD のブランチをほかの worktree が動かす 2 行）、`cmd/codexreview/main_test.go::TestOutputIsOneJSONObject`（`state_changed` の行） |
| AC-07 | 2-1 | test: `run_test.go::TestRunStateChanged`（`with_failed_exit`・`with_timeout`） |
| AC-08 | 1-1 | test: AC-08 の 1 は `run_test.go::TestRunPreconditions`（新規の、HEAD がタグを指す行）。AC-08 の 2 は `run_test.go::TestRunAfterStateUnreadable`（今の git を消す行、新規の HEAD をタグへ向ける行と HEAD のブランチの ref を消す行）。AC-08 の 3 は `internal/codexreview/state_test.go::TestParseHead`（新規）と、上の HEAD がタグを指す 2 つの行 |
| AC-09 | 2-1 | test: `run_test.go::TestRunStateChanged`（各行の `Changed`。3 つがそろう順序は、ほかの worktree が中身の違う commit で動かす行） |
| AC-10 | 1-1 / 2-1 | test: `run_test.go::TestRunStateChanged`（各行の `ChangedRefs`。空のときは nil でない空の配列。同時に作ったタグが現れないことは、ほかの worktree が中身の違う commit で動かす行）、`state_test.go::TestDescribeRefs`（新規、worktree の併記） |
| AC-11 | 2-1 / 3-1 | test: `cmd/codexreview/main_test.go::TestOutputIsOneJSONObject`（終了コード `3` と `--protocol 1` の結果）。static: `cmd/codexreview/docs_test.go::TestDocumentedProtocolMatches`（共通手順の `--protocol` が `codexreview.Protocol` と一致）。diff: `_checks/check.sh`（`Protocol` の定義の行と、`Result` の JSON の名前の並びが前後で同じ） |
| AC-12 | 3-1 | manual: 書き換えた文が設計書 3.4 の 2 点を述べることを、PR のレビューで読んで確かめる。static: `cmd/codexreview/docs_test.go::TestProcedureNamesKinds`（`state_changed` の行の終了コード `3` と扱い `stop`）。diff: `_checks/check.sh`（書き換えた文（「A `state_changed` caused by」で始める）のほかは、`stop` のときに報告するものを含めて前後で同じ） |

**manual の読み取り（AC 以外）:** 設計書 3.5 の表のコメントと文書（`types.go`、`state.go`、`package_reference.md`）が、AC-09〜AC-11 と矛盾しないことを、PR のレビューで読んで確かめる（H-04）。

**`_checks/check.sh` の実行の記録:** ステップ 3-1 の最後の項目で書く。

| 項目 | 値 |
|---|---|
| 変更の前（merge-base） | （実装の PR で書く） |
| 変更の後（PR の head） | （実装の PR で書く） |
| 出力 | （実装の PR で書く） |

## 8. 完了条件 (Success Criteria)

-   [ ] AC-01〜AC-12 の検証が 7 章のとおり通る。
-   [ ] ほかのブランチ・タグ・stash の変化では止まらず、この worktree の `git status`・HEAD・HEAD のブランチの ref の変化では今と同じく止まる。
-   [ ] HEAD の読み取りの想定外の出力を補わず、読めないものとして扱う（要件書 4.2 の「信頼できない入力」）。
-   [ ] 共通手順の文、`types.go` のコメント、`package_reference.md`、ADR-0013 が、新しい比べるものと食い違わない。
-   [ ] `make test`・`make lint`・`make deadcode` が通る。

## 9. 次のステップ (Next Steps)

-   本書の PR のマージの後、`/runplan 0079` で実装の PR を作る。
-   実装の PR のマージの後、利用者が `make install-codexreview` で `codexreview` を入れ直す（要件書 4.4、設計書 6.2）。

## 変更履歴

- 2026-11-10 承認（isseis、#未記入）
