# 実装計画書：/fixpr Phase 4.5 の git の起動と push の前に、ガードツールが git の状態を基準値と比べる

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-10 |
| Review date | 2026-11-10 |
| Reviewer | isseis |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番を、「設計書 3.6」などは設計書の節を指す。用語は要件書 3 章・6 章と設計書の用語（基準値、基準値の commit、Phase 4 の commit、1 本につながる、比べる、HEAD の決まり、GUARD、ROOT、対象のファイル）に従う。「ガイド」は `docs/dev/guides/fixprguard_guide.md` を、「手順」と「手順 N」は設計書 1 章と同じく `.claude/commands/_lib/local-cli-review.md` の手順の全体とその step N を指す。

既存のコードと文書の記述は、commit `85feed05` で確かめた。設計書が読んだ commit `39c0c4aa` から `85feed05` までの間に、`internal/fixprguard` では `write.go` だけが 2 行変わった（本書は `write.go` を引かない）。`cmd/fixprguard`、`docs/dev/guides/fixprguard_guide.md`、タスク 0030 の文書は変わっていない。変わったのは `.claude/commands/fixpr.md`・`_lib/local-cli-review.md`・`_context.md`・`_lib/task-pr-handoff.md`・`runplan.md` と ADR-0015 である（`git diff --stat 39c0c4aa 85feed05 -- <これらのパス>` で確かめた）。本書は、これらの行番号を `85feed05` の値で示す。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`/fixpr` の Phase 4.5 で手順が定める git の起動の前に、ガードツールの新しいサブコマンド `check` が git の状態を基準値と比べ、Phase 4.5 の commit は新しいサブコマンド `push-review` が push するようにする（F-001〜F-003）。`--protocol` を `6` に上げて、古いツールが検査を飛ばさないようにし（F-004）、ほかの呼び出し元の振る舞いは変えない（F-005）。文書を合わせる（F-006）。設計は設計書 1〜6 章のとおりである。

### 1.2. 実装の原則

-   設計書 8 章の手順 1〜8 を、本書のフェーズ 1〜8 として同じ順序で実装する。
-   型・関数・比べる順序・時点・報告の内容は、設計書 3〜4 章で決まっている。本書は、どのファイルを変え、どのテストで確かめ、どの順に作業するかを定める。設計の内容は繰り返さず、節を参照する。
-   加えたテスト（表のテストの行を含む）は、どれも対象を壊して失敗することを確かめ、確かめたことをコミットメッセージに書く（`CLAUDE.md`「Every test must be able to fail for its stated reason」）。壊すものは、設計書 7.2 と、各ステップの「壊して確かめる」の項目に挙げる。
-   Go のコメント・識別子・文字列リテラル（`check` の `head` の規則の名前を含む）は英語で書く（`CLAUDE.md`「Go Idioms」）。文書のガードのテストの日本語のリテラルは、ガイドの見出しなど、ガードのリテラルの例外に当たるものに限る。
-   `fixpr.md` の Phase 4.5 の節には、禁止の文の中でも `git push` と書かない（設計書 7.4）。`fixpr.md` の全体に `git add -A` と `git commit -m` も書かない（`TestRemovedCommandLiterals` が拒否する）。
-   `fixpr.md` が止まったときの報告を分ける根拠になる `git-failed` の規則の名前（`push` そのものの失敗とそれ以外）は、出力の契約として扱う。テストで固定し、ガイド 8 章に書く（設計書 3.8）。

### 1.3. 計画の際に決めたこと（2026-11-10）

-   **PR は 2 つに分ける（人間のレビュアーが決めた）:** PR-1 はフェーズ 1〜7（設計書 8 章の手順 1〜7）、PR-2 はフェーズ 8（マージと再インストールの後の確かめと、その記録）である。理由: 手順 2〜5 と、`--protocol` の番号とガイドの変更（手順 6 の一部）は、1 つの PR に入れる必要がある。`PartBranch` は既存の `inspect`・`finish`・`push` の出力の契約も変えるので、ガイド 9 章の規則により `Protocol` を上げる変更と分けられない。`Protocol` を上げると、`TestDocumentedProtocolMatches`（`cmd/fixprguard/docs_test.go:350-373`）と `TestDocumentedInvocationsParse`（同 `:212-242`）が失敗するので、文書の番号の変更も同じ PR に入れる必要がある（設計書 8 章）。内部だけの PR を先に出すには、`PartBranch` を後ろに回して設計書の順序を変えることになるので、採らない。`fixpr.md`・`local-cli-review.md` の書き換え、ADR-0015、0030 の注記と `_checks/` のスクリプトは、別の PR に分けることもできる（ADR-0015 のずれの記述は、ずれを直し終える PR が取り除く。`docs/dev/conventions/adr.md`「Discrepancy statements」）。しかし分けると、その間の `main` で、ガイドの 7.1 が Phase 4.5 の `check`・`push-review` による保証を述べながら、`fixpr.md` はまだそれらを呼ばない状態になる。本タスクが閉じようとしている、文書と実際のずれである。そのため、まとめて PR-1 にする。手順 8 の観察は PR-1 のマージの後にしか行えないので、その記録を PR-2 にする。設計書 7.6 が、確かめの結果を実装計画に記録するとしているためである。ADR の判断: ADR にしない。本タスクの中で閉じる PR の切り方だからである。
-   **`check`・`push-review` が「作業ツリーの最上位でない」として終了コード `2` にするのは、今の場所に `.git` がないときだけにする（人間のレビュアーが決めた）:** `.git` ファイルの形の誤りなどは、今までどおり `CheckState` の `config` の違い（`internal/fixprguard/gitstate.go:561-564`）にする。理由: 設計書 3.1 の「最上位でない場所」に厳密に合わせるためである。`.git` ファイルの書き換えは、基準値と比べるべき状態の変化である。使い方の誤りとして報告すると、書き換えが場所の誤りに見えてしまう。`begin` は `locateGitDirs`（`gitstate.go:175-212`）の失敗をすべて `ErrInvalidInput` にするが、`begin` は基準値を作る側で、比べる相手がない。ADR の判断: ADR にしない。1 つの関数の中で閉じる判定だからである。
-   **`head` の規則の名前の一覧を関数で公開し、ガイド 8 章との一致を文書のガードで確かめる:** 設計書 4 章は、規則の名前の一覧をガイドの 8 章に書くとする。これは文書が書く値なので、`IgnoredFailureRules()`（`internal/fixprguard/ignored.go:216`）と同じ形で一覧を公開する。ガイドの表との照合は、表がこの一覧のためだけにあるので、`TestGuideListsWatchList` と同じく集合の一致で確かめる（`CLAUDE.md`「a change to a documented value or structure still needs the matching test」）。設計書 7.4 の文書のガードに、この 1 つを足す。ADR の判断: ADR にしない。テストの追加で、本タスクの中で閉じるためである。
-   **コマンドの文書の出力の例の `"protocol":N` も、文書のガードで照合する:** 今の `TestDocumentedProtocolMatches` は、`"protocol":N` をガイドでだけ照合し、コマンドの文書では `--protocol` だけを照合する（`cmd/fixprguard/docs_test.go:350-373`）。`runpr.md:195` の `{"protocol":5,...}` は、どのガードにも照合されずに古いまま残りうる。文書が書く値なので、照合をコマンドの文書にも広げる。ADR の判断: ADR にしない。テストの範囲の拡張で、本タスクの中で閉じるためである。
-   **`push-review` は、比較（push 先の読み取りを含む）の後に、送るものがないことを拒否する:** `compareState` を 1 回で終える形を保つためである。送るものがないときも `git ls-remote` は起動するが、push はしない。出力の `Protocol` は、既存の `Push` と同じく `internal/fixprguard` の中で入れる。ADR の判断: ADR にしない。1 つの関数の中の順序だからである。
-   **ガイド 7.2 の新しい残余のリスクの置き場:** Phase 4.5 の修正が範囲の検査を受けないことは 7.2.3（変更範囲）に、比較が時点だけの確認であること・手順が定めない git の起動・1 本につながる commit の出所を区別しないことは 7.2.4（git の状態の比較）に書く。既存の小節の見出しと番号を変えないので、既存のガードの節の切り出し（`subsection(guide, "### 7.2")` など）が変わらない。ADR の判断: ADR にしない。文書の中の置き場の選択だからである。
-   **`push-review` の送り元の確かめに、`gitRouter` に差し込みの口を足す:** 設計書 7.1 の「refspec の送り元が、比べた HEAD の OID であること」を、push 先の確認（`ls-remote`）の後に HEAD を動かして確かめる。今の `gitRouter`（`internal/fixprguard/test_helpers_git.go:60-71`）は、git のサブコマンドの名前だけを記録し、引数を残さない。そのため、指定したサブコマンドの前に処理を差し込む口を足し、push の後のリモートのブランチの OID で確かめる。ADR の判断: ADR にしない。テストの補助の形で、本タスクの中で閉じるためである。

### 1.4. 既存コード調査結果

**git の状態の比較（`internal/fixprguard/gitstate.go`）**

| 項目 | 今あるもの（`85feed05`） | 変えること |
|---|---|---|
| `parts` の値 | 定数 `PartHead` などと `GitStateParts()`（`:50-64`） | `PartBranch` を加え、`GitStateParts()` に含める（設計書 3.2） |
| 誤りの型 | `GitStateError{Parts}` と `Error()`（`:149-161`） | `Rule` の欄を加え、`head` の規則の名前を 1 行の診断に出す（設計書 3.2・4 章） |
| 比較 | `compareState(ctx, base, wantHead, check)`（`:808-839`）。HEAD とブランチの違いは、どちらも `PartHead`（`:822-824`）。push 先の OID は常に `base.remoteOID` と比べる（`:836-838`） | HEAD の決まり（`headRule`）を受け取り、`stateResult` を返す形にする。ブランチの違いを `PartBranch` にし、`headFollowsPhase4` で系列の判定（設計書 3.3）をして、push 先の OID を `--commit` と比べる |
| 状態の比較の本体 | `CheckState`（`:556-606`）。`locateGitDirs` の失敗を `config` の違いにする（`:561-564`） | 変えない。`check`・`push-review` もこれを使う |
| commit の読み取り | `commitObject`（`:639-664`）。replace refs を無効にした `git cat-file` で親の一覧を返す | 変えない。系列の判定で使う |
| HEAD の読み取り | `headState`（`:610-632`） | 変えない。系列をたどり始める OID と push の送り元は、これが返す OID である |
| 最上位の判定 | `locateGitDirs`（`:175-212`）。`.git` がないときを含め、失敗はすべて `ErrInvalidInput` | 変えない。`check`・`push-review` の最上位の判定は、`.git` の有無だけを、git を起動せずに確かめる（1.3） |

-   `compareState` の呼び出しは、製品のコードに 3 か所（`internal/fixprguard/worktree.go:144` の `Inspect`、`:343` の `checkedChanges`、`:575` の `Push`）、テストに 4 か所（`gitstate_test.go:268`・`:273`・`:319`・`:533`）ある（`git grep -n "compareState(" -- '*.go'` の結果）。どれも HEAD を基準値の commit か `Push` の `--commit` と比べるので、`headIsCommit` に機械的に合わせる。
-   ブランチの違いを `head` と期待するテストは、`gitstate_test.go::TestBaselineDetectsChange` の `branch_switch_same_commit`（`:101-103`）と `detached_head_same_commit`（`:104-106`）の 2 行だけである（`internal/fixprguard` と `cmd/fixprguard` のテストで、`PartHead`・`"head"` の期待の前に `switch`・`--detach`・`checkout` を含む行を探した結果）。どちらも `compareState` を直接呼ぶ。
-   `checkCommit`（`worktree.go:454-`）は、ブランチの違いと親の違いとメッセージの違いを、まとめて `PartHead` にする（`:467-469`）。ブランチの違いだけを、`compareState` と共有する振り分けの関数で `PartBranch` にする（設計書 3.2）。`checkCommit` のブランチの違いを確かめる既存のテストはない（設計書 2.2）。

**push（`internal/fixprguard/worktree.go`）**

-   `Push`（`:571-589`）は、`compareState` の後に、親が基準値の commit だけであることを `commitObject` で確かめ（`:579-583`）、`<commit>:refs/heads/<branch>` の refspec を `-q --no-follow-tags --recurse-submodules=no` で push する（`:584-586`）。送り元は引数の SHA で、記号の `HEAD` ではない。push の失敗は、`w.git` の規則の名前 `push` を通じて、`git failed: push failed` などの 1 行の診断になる（`gitFailure`、`gitstate.go:450-458`）。`ls-remote` の失敗は規則の名前 `ls-remote` になる（`gitstate.go:703`）。`PushReviewed` も同じ形で push するので、refspec を組み立てて push する部分を 1 つの関数にし、両方で使う。`Push` の振る舞いは変えない。
-   `PushResult`（`:558-562`）を `push-review` の出力にも使う（設計書 3.2）。

**サブコマンドの登録（`cmd/fixprguard/main.go`）**

-   サブコマンドの名前の定数（`:57-70`）、`specs`（`:107-126`）、`execute` の `switch`（`:354-389`）に加える。`push`（`specs` の `:114`、処理は `:653-660`）が、`gh` を起動せず、PR を取らず、`--baseline` と `--commit` を必須にする手本である。`usesGH` は `false` なので、`GH_REPO`・`GH_HOST` の検査（`:355-361`）の対象外になる。
-   出力の `protocol` は、`interactionLimits`（`:454-466`）と同じく、`fixprguard.Protocol` を入れる。終了コード `2` は、`stop`（`:299-311`）が `ErrInvalidInput` から決める。

**`protocol` の番号**

-   `internal/fixprguard/types.go:21` の `Protocol = 5`。`git grep -n -e '--protocol[ =]5' -e '"protocol":5' -e '"protocol": 5'` の結果から `codexreview` の行を除くと、`fixpr.md` に 10 か所、`runpr.md` に 6 か所（`--protocol 5` が `:23`・`:176`・`:188`・`:567`・`:758` の 5 か所と、出力の例 `"protocol":5` が `:195` の 1 か所）、ガイドに 19 か所ある。過去のタスクの文書は変えない。ガイド 4 章の「現在の番号は `5`」は `:64` にある。

**テスト**

-   `internal/fixprguard` のテストは、git を一時ディレクトリの実際のリポジトリで試す。`prepareRound`・`prepared.mustFinish`（`worktree_flow_test.go:25`・`:60`）で Phase 4 の commit を作り、`newTestWorktree`（`test_helpers_git.go:250`）の `gitRouter` で起動した git を記録し、`wantStateParts`（`gitstate_test.go:22`）で `parts` を確かめる。設定を書き換えたときに `git config --list` 以外の git を起動しないことは、`TestPushRechecks` の `listOnly`（`worktree_flow_test.go:588-636`）と同じ確かめ方で確かめる。
-   `cmd/fixprguard/main_test.go` の `validArgs`（`:28-48`）は `TestProtocolMismatch`（`:50-75`。`Protocol + 1`・`Protocol - 1` を拒否する）と `TestReviewerFlag` の表の元である。`TestGhTargetEnvRejected`（`:223-261`）は、`usesGH` が `true` のサブコマンドにだけ行を求める。`TestPreviousBaselineRejected`（`:357-389`）は `--baseline` を取るサブコマンドの表を持つ。`newTestRun(t, nil)` の `gh` の fake は、呼ばれると失敗する（`internal/cmdrunner/testutil/mocks.go:64`）ので、`gh` を起動しないことを確かめられる。
-   文書のガードは、`cmd/fixprguard/docs_test.go` の `TestDocumentedInvocationsParse`（`:212-242`。`fixpr.md` の期待は `:217`）、`TestDocumentedProtocolMatches`（`:350-373`）、`TestGuideNamesGitStateParts`（`:482-496`）と、`cmd/yt2column/fixpr_docs_test.go` の `TestFixprguardCallSites`（`:708-731`）、`TestRemovedCommandLiterals`（`:1103-1122`。Phase 4.5 の例外とその注記は `:1115-1116`）である。`fixpr_docs_test.go` の節の見出しの定数（`:33-39`）には Phase 4.5 がないので、加える。

**コマンドの文書（`85feed05`）**

| 文書 | 今あるもの | 変えること |
|---|---|---|
| `fixpr.md` | 「The guard tool」（`:28-87`。`--protocol 5` は `:44`、「Stops on the ignored files」は `:65-78`）、「Authorization」（`:89-106`。`git push` は `:99-100`・`:105-106`）、Phase 1 の `begin`（`:268-`）、Phase 4.5（`:778-804`。BASE は `:787-788`、素の `git push` は `:801`）、Final report の 7・9・10（`:1012-1052`） | ステップ 6-3 のとおり |
| `_lib/local-cli-review.md` | 入力（`:30-37`）、手順 1（`:69-75`）、手順 3 の `cd`（`:101-104`）、手順 5 の build checks と commit と README の同期（`:203-217`）、手順 6 の `coderabbit review findings --clear`（`:226-228`） | ステップ 6-2 のとおり |
| `runpr.md` | `--protocol 5` の説明（`:22-23`）、step 1 の `head`（`:173-182`） | ステップ 6-4 のとおり |
| `_context.md` | 「Guard tool」の行（`:189`） | ステップ 6-5 のとおり |

-   ガイドで、基準値と比べるサブコマンドを列挙する箇所は、`:23`・`:127`・`:343`・`:348`・`:379`・`:456`（8 章の `git-state-changed` の行）である（`` git grep -n '`inspect`・`finish`・`push`' `` などで探した）。これらには `check`・`push-review` を加える。次の箇所は、名前を足すのではなく、それぞれ書き直す。
    -   `:31`（2 章の「作業ディレクトリ」）: 最上位でないとき、`check`・`push-review` は、`begin` と同じく終了コード `2` になる（`.git` がないときだけ。1.3）。`inspect`・`finish`・`push` の列に加えない。
    -   7.2.4 の「基準値を持たない…`push` の後の書き換えは検出しない」の項目と、7.2.5 の「`push` の後に変えること」（`:417`）: 最後の push は `push-review` になる。
    -   7.1 の「範囲外の変更を、利用者の確認なしに commit しない」の行: Phase 4.5 の commit には当てはまらないことを添える（範囲の検査がないことは 7.2.3 に書く。1.3）。
-   `fixpr.md` の Phase 3.5 の止まったときの記述（`:680-686`）は、`parts` の値を文で列挙している。`branch` を加える。
-   `docs/dev/conventions/package_reference.md` の `cmd/fixprguard` の行（`:16`）がサブコマンドを列挙している。設計書 2.2 の表にないが、古い記述を残さないため、ステップ 6-7 で `check`・`push-review` を加える。`internal/fixprguard` の行（`:26`）には、比較の関数の一覧がないので変えない。

**`implementation_handoff.md` と `design_handoff.md`**

-   `implementation_handoff.md` は、このタスクにはない。`design_handoff.md` の H-01〜H-05 は、すべて設計書の付録 A に記録されている。

**具体的な型ごとの振る舞い**

-   設計書は、具体的な型で振る舞いを選ばない。HEAD の決まりは `headRuleKind` の値で宣言し、`switch` で選ぶ。ゼロ値は厳しい方の `headIsCommit` で、`default` は誤りを返す（設計書 3.2、`CLAUDE.md`「Declare, don't infer」）。`headRule` は `internal/fixprguard` の中だけで作るので、構築の経路を閉じ込める必要はない。

## 2. 実装手順 (Implementation Steps)

テストの関数名は予定の名前である。実装で変えたときは、本書 7 章を同じ PR で直す。各ステップの「壊して確かめる」は、そのステップで加えるテストが失敗することを確かめるために壊すものである（1.2）。

### フェーズ 1: 実際のツールが比べる状態を書き換えないことの確かめ（設計書 8 章の手順 1）

#### ステップ 1-1: 基準値を取ってからツールを実行し、比べる

**変更するファイル:** `docs/tasks/0076_fixpr_local_review_guard/03_implementation_plan.md`（記録）

-   [x] 利用者に、確かめに使う開いている PR（このタスクのものでなくてよい。設計書 7.6）を選んでもらい、次の実行の許可を求める: その PR のブランチの取得（`git fetch`）、`fixprguard begin`・`inspect` の `gh` の読み取り、`coderabbit review`、`codexreview`（Codex の割り当てを 1 回使う）。この確かめの `codexreview` は手順の外での実行なので、`CLAUDE.md`「Tool Execution Safety」の常設の許可に当たらない。
-   [x] この worktree（作業ツリーが clean）で、選んだ PR のブランチ（ほかの worktree で checkout 中のときは、その head のコミットに `--no-track` で作った別名のローカルブランチ。push 先の OID は比べる対象だが、push はしない）に `git switch` し、今インストールされているツールで `fixprguard begin --protocol 5 --reviewer <reviewer> <PR>` を実行して基準値を取る。`GH_REPO`・`GH_HOST` は、この後のどの呼び出しでも設定しない。権限の確認には「今後は聞かない」と答えない（`.claude/settings.local.json` が書き換わり、`ignored` で止まるため。設計書 5.3）。
-   [x] 次の 6 つを、この順に 1 つずつ実行する。`codexreview` は作業ツリーが clean であることを求めるので、`make` より前に置く。レビューの範囲は、HEAD の親から HEAD までにする。
    -   CodeRabbit CLI（手順 2 の形）
    -   `coderabbit review findings --clear`
    -   `codexreview --protocol 1 --base <HEAD の親の 40 桁の SHA>`（手順 3 の形）
    -   `make fmt`、`make test`、`make lint`
-   [x] それぞれの直後に、`fixprguard inspect --protocol 5 --reviewer <reviewer> --baseline <baseline> <PR>`（`--thread` は渡さない）で比べる。6 回とも同じ形で呼ぶ。
-   [x] 各 `inspect` の終了コードと、止まったときの `stop`・`parts` を、選んだ PR と実行した commit とともに控える。終了コード `3`（範囲外の変更）は、比較に通ったものとして扱う（設計書 7.6）。終わったら実装のブランチに戻り、控えた結果をこのステップに記録する。
-   [x] 1 つでも `git-state-changed` で止まったら、フェーズ 2 に進まない。止まったツールと `parts` を利用者に報告し、設計の変更として扱う（設計書 7.6 の前提が崩れるため）。


**記録（2026-11-10）:**

-   選んだ PR: #783（`isseis/0077-openai-llm-client-02`）。このブランチはほかの worktree で checkout 中だったので、その head `ed4e3035eac2992fa7c9da319d9f704c3fd59222` に `--no-track` で別名のローカルブランチ `isseis/0076-observe-783` を作って実行した（確かめの後に削除した）。レビューの範囲は `d93b24b1651cfba75c2463b0034fe56dd749ad46`（HEAD の親）から HEAD まで。`fixprguard` は `vcs.revision=0fa614feb75b44119e224770fd958a43e800beb9`（`vcs.modified=false`）。
-   1 回目の基準値（`begin` 終了コード `0`）の後の結果:

    | 実行したもの | 実行の結果 | 直後の `inspect` |
    |---|---|---|
    | CodeRabbit CLI | 終了コード `1`、`errorType` `rate_limit`（レビューは行われなかった） | `0` |
    | `coderabbit review findings --clear` | 終了コード `0` | `0` |
    | `codexreview --protocol 1 --base d93b24b1…` | `kind` `no_findings` | `0` |
    | `make fmt` | 終了コード `0` | `0` |
    | `make test` | 終了コード `0` | `1`（`git-state-changed`、`parts` `config`） |
    | `make lint` | 終了コード `0` | `1`（`git-state-changed`、`parts` `config`） |

-   `config` の違いの原因: 共有の config に、VS Code が新しく作ったブランチについて `branch.isseis/0076-observe-783.vscode-merge-base` を書き込んでいた（config の更新時刻は `make fmt` の後・`make test` の最中）。今のブランチの設定なので比べる対象であり、比べる対象から外さないのは設計どおりである。ほかの既存のブランチには、同じ値がすでにある。
-   確かめ直し: 基準値を取り直し（`begin` 終了コード `0`）、`make test`・`make lint` の後にそれぞれ `inspect` を実行すると、どちらも終了コード `0` だった。前後の `git config --list` に差分はなかった。
-   判断: 止まった原因は、観察のために新しいブランチを作ったことと VS Code の書き込みで、確かめる対象のツールではない。下の成功の条件は文字どおりには満たしていない（6 つのうち 2 つが終了コード `1`）。設計の変更として止まる代わりに、人間のレビュアー（isseis）が 2026-11-10 にこの節の停止の規則を外し、比較に通ったものとして扱ってフェーズ 2 に進むと決めた。確かめ直しは `make test`・`make lint` だけで、6 つを 1 つの基準値でやり直してはいない。
-   残るリスク: 新しく作ったブランチでは、VS Code が少し後に `branch.<今のブランチ>.vscode-merge-base` を書き込むので、その前に取った基準値との比較は `config` で止まりうる。実際の `/fixpr` でも、`gh pr checkout` の直後に始めると起こりうる。ステップ 6-6 でガイドに書き、ステップ 8-2 で観察する。

**成功の条件:** 6 つの `inspect` が、どれも終了コード `0` か `3` で終わったことが記録されている。

### フェーズ 2: `PartBranch` と、`compareState` の HEAD の決まり（設計書 8 章の手順 2）

#### ステップ 2-1: ブランチの違いを `branch` で示し、`compareState` に HEAD の決まりを渡す

**変更するファイル:** `internal/fixprguard/gitstate.go`、`internal/fixprguard/worktree.go`、`internal/fixprguard/gitstate_test.go`、`internal/fixprguard/worktree_flow_test.go`、`docs/dev/guides/fixprguard_guide.md`（8 章の `parts` の一覧だけ）

-   [x] `PartBranch` を加え、`GitStateParts()` に含める（設計書 3.2）。
-   [x] `headRuleKind`・`headRule`・`stateResult` を設計書 3.2 のとおりに加え、`compareState` の引数と戻り値を変える（`stateResult` の `commits` の欄は、使い始めるステップ 3-1 で加える）。このステップでは `headIsCommit` の分岐だけを書き、`headFollowsPhase4` の分岐は誤りを返す（ステップ 3-1 で書く）。
-   [x] ブランチの比較と `branch`・`head` の振り分けを 1 つの関数にし、`compareState` と `checkCommit` の両方で使う（設計書 3.2）。`checkCommit` の親とメッセージの違いは、今までどおり `head` にする。
-   [x] `GitStateError` に `Rule` を加える。`Error()` は、`Rule` があるときだけ、規則の名前を添える（設計書 3.2・4 章）。
-   [x] `compareState` の呼び出し 7 か所（1.4）を `headIsCommit` に合わせる。振る舞いは変えない。
-   [x] `branch_switch_same_commit`・`detached_head_same_commit` の期待を `PartBranch` にする。
-   [x] `finish`・`push` でブランチの違いが `branch` になることを確かめる行を加える。`push` は `TestPushRechecks` の表にブランチを切り替える行を加える。`finish` は、`checkCommit` に届くように、post-commit の hook がブランチを切り替える行を加える（hook は基準値を取る前に置くので、hooks の比較は通る）。`inspect` は、更新した 2 つの行（`Inspect` と同じ形で `compareState` を直接呼ぶ）で確かめる。
-   [x] ブランチと HEAD の両方が違う行を加え、`parts` がちょうど `branch` だけであることを確かめる（設計書 4 章。ブランチが違えば HEAD は比べない）。
-   [x] `headRuleKind` の一覧の外の値を渡したときに、誤りが返り、`git ls-remote` を起動しないことを確かめる行を加える。
-   [x] ガイド 8 章の `git-state-changed` の行の `parts` の一覧に `branch` を加える（`TestGuideNamesGitStateParts` が `GitStateParts()` と照合するため）。8 章のほかの記述は、ステップ 6-6 で書く。
-   [x] 壊して確かめる: 振り分けの関数がブランチの違いを `head` にする形に戻す、ブランチが違うときも HEAD を比べる、`switch` の `default` が誤りを返さない。加えた行と更新した 2 つの行のうち、どれが失敗したかをコミットメッセージに書く。

**成功の条件:** `make fmt` と `make lint` を実行し、`go test -tags test ./internal/fixprguard/... ./cmd/fixprguard/...` が通る。

### フェーズ 3: 系列の判定と `CheckReviewState`（設計書 8 章の手順 3）

#### ステップ 3-1: 系列の判定と `check` の比較

**変更するファイル:** `internal/fixprguard/gitstate.go`、`internal/fixprguard/worktree.go`、`internal/fixprguard/review_test.go`（新規）

-   [x] `headFollowsPhase4` の分岐に、系列の判定を設計書 3.3 のとおりに書く。たどる数の上限は、Phase 4 の commit より後の commit で 32 個である。止まる 5 つの規則（マージの commit、親のない commit、Phase 4 の commit がない、`--commit` との不一致、上限）には、それぞれ固定の英語の名前を付け、`GitStateError.Rule` に入れる。名前は型 `lineRule` の定数にし、`Rule` を持つ誤りは関数 `lineError` だけが作る。
-   [x] 規則の名前の一覧を返す公開の関数 `HeadLineRules()` を加える（1.3。`IgnoredFailureRules()` と同じ形）。`head` の各行の確かめは、期待の規則がこの一覧にあることも確かめる。
-   [x] `GitStateError.Error()` の規則の名前の添え方を、`TestGitStateErrorText` で確かめる（文面が出力の契約であるため）。
-   [x] `ReviewState` と `CheckReviewState` を設計書 3.2 のとおりに加える。最初に `--commit` の形（`validOID`）と、今の場所に `.git` があること（git を起動せずに確かめる。1.3）を確かめ、どちらも違えば `ErrInvalidInput` を返す。その後に `compareState` を `headFollowsPhase4` と `checkLocal` で呼ぶ（設計書 3.4・3.10。`check` は push 先を読まない）。
-   [x] `review_test.go` に、設計書 7.1 の「系列の判定」と「比較の順序」の各場合を書く。加えて、次の行を書く。
    -   上限の境界: Phase 4 の commit より後の commit が 32 個（通る）と 33 個（`head`）。
    -   系列の途中にマージの commit がある場合（HEAD 自身は親が 1 つ）。
    -   `.git` がない場所（`ErrInvalidInput` で、git を 1 回も起動しない）と、形の誤った `.git` ファイル（`config`）。
    -   `--commit` の形の誤り（大文字、短い値、空）。
    -   `ignored` で止まったとき、止まった時点の記録のファイルが書かれること（設計書 3.8 の最後の段落）。
    -   push 先が基準値の OID のまま（Phase 4 の push が届いていない）でも `check` が通り、どの行でも `git ls-remote` を起動しないこと（設計書 3.10）。
-   [x] 行の確かめ方: 通る行では、`Base`・`Head`・`Commits` の 3 つの値を確かめる。`head` の各行では、その行が当たるべき規則の名前を期待の値として持ち、`GitStateError.Rule` と等しいことを確かめる（`wantStateParts` の隣に補助を置く）。`head` 以外で止まる行では、`Rule` が空であることを確かめる。基準値の commit から続かない列の行は、基準値の commit がリポジトリの最初の commit にならないように作る（`newTestRepo` の最初の commit の上に commit を足してから基準値を取る）。親のない commit の行と区別するためである。
-   [x] 壊して確かめる: 親が 2 つの commit を通す、`--commit` との照合を外す、HEAD が基準値の commit であることの確かめを外す、ブランチの比較を外す、`CheckState` の呼び出しを外す、上限の比較を 1 つずらす、`checkLocal` を `checkWithRemote` にする（設計書 7.2）。どの行が失敗したかをコミットメッセージに書く。

**成功の条件:** `make fmt` と `make lint` を実行し、`go test -tags test ./internal/fixprguard/...` が通る。

### フェーズ 4: `PushReviewed`（設計書 8 章の手順 4）

#### ステップ 4-1: Phase 4.5 の commit の push

**変更するファイル:** `internal/fixprguard/worktree.go`、`internal/fixprguard/review_test.go`、`internal/fixprguard/test_helpers_git.go`

-   [x] `Push` の、refspec を組み立てて push する部分（`worktree.go:584-586`）を 1 つの関数にし、`Push` と `PushReviewed` の両方で使う（1.4）。`w.git` に渡す規則の名前は `push` のままにする（1.2）。
-   [x] `PushReviewed` を設計書 3.2・3.5 のとおりに加える。`CheckReviewState` と同じ入力の確かめの後に、`compareState` を `headFollowsPhase4` と `checkWithRemote` で呼び、push 先の OID が `--commit` であることを確かめる。Phase 4 の commit より後の commit がなければ、比較の後に、push せずに `ErrInvalidInput` を返す（設計書 3.1、1.3）。送り元は、系列をたどった OID である。
-   [x] `gitRouter` に、指定した git のサブコマンドの前に処理を差し込む口（`Before`）を加える（1.3。`test_helpers_git.go` は `//go:build test`）。`ls-remote` の失敗の行も、この口で `ls-remote` の前にリモートを動かして作る。
-   [x] `review_test.go` に、設計書 7.1 の「`push-review`」の各場合を書く。加えて、次の行を書く。
    -   push 先が基準値の OID のまま（Phase 4 の push が届いていない）: `remote` で、push しない。
    -   `ls-remote` の後に HEAD を進めても、push 先のブランチが比べた OID になる（差し込みの口を使う）。差し込みは `push` のサブコマンドの前に置き、`PushReviewed` の呼び出しだけに使う新しい router に付ける。HEAD は router を通さずに `testRepo.Git` で進める。行の初めに、差し込みが働いてローカルの HEAD が比べた OID より先にあることを確かめる。
    -   設定された push の refspec と `push.followTags` は基準値を取る前に設定し（`TestPushExplicitRefspec` と同じ）、注釈付きのタグは Phase 4.5 の commit に付ける。
    -   止まるどの行でも、`git push` を起動せず（`router.Calls`）、リモートのブランチが動かないこと。送るものがない行も同じである（`cmd` の層の終了コード `2` だけでは、push しないことを確かめられないため）。
    -   push そのものの失敗が `ErrGitFailed` になり、診断が規則の名前 `push` を含むこと（`TestPushFailure` と同じく、リモートの pre-receive の hook で失敗させる）。`ls-remote` の失敗の診断は `push` を含まないこと。
-   [x] 壊して確かめる: push 先の比較の相手を基準値の OID に戻す、送り元を記号の `HEAD` にする、送るものがないときの拒否を外す、`--no-follow-tags` を外す、push の規則の名前を変える（設計書 7.2）。どの行が失敗したかをコミットメッセージに書く。

**成功の条件:** `make fmt` と `make lint` を実行し、`go test -tags test ./internal/fixprguard/...` が通る。

### フェーズ 5: `cmd/fixprguard` の `check`・`push-review` と、`Protocol` の `6`（設計書 8 章の手順 5）

#### ステップ 5-1: サブコマンドと番号

**変更するファイル:** `cmd/fixprguard/main.go`、`cmd/fixprguard/main_test.go`、`internal/fixprguard/types.go`

-   [x] `check` と `push-review` を、サブコマンドの名前の定数、`specs`（`flags` は `--baseline` と `--commit`、どちらも必須、`arg: argNone`、`usesGH: false`）、`execute` の `switch` に加える。処理は `push`（1.4）と同じ形で、`ParseBaseline` の後に `CheckReviewState`・`PushReviewed` を呼ぶ。出力の `protocol` は、`internal/fixprguard` が入れる（1.3）。
-   [x] `Protocol` を `5` から `6` にする（`types.go:21`）。
-   [x] 2 つのサブコマンドを `validArgs` に加える（`--protocol` の不一致は `TestProtocolMismatch` が確かめる）。`TestPreviousBaselineRejected` には加えない。このテストは `.git` のない場所で実行するので、新しいサブコマンドでは `.git` の確かめも同じ終了コード `2` になり、基準値の拒否と区別できないためである。基準値の解析は `push` と共有の `ParseBaseline` である。`TestGhTargetEnvRejected` は、`usesGH` が `false` なので行を求めない。
-   [x] 新しいテストを書く（設計書 7.3）。どちらも、実際のリポジトリで実行し、`gh` の fake に呼び出しがないこと（`newTestRun(t, nil)`）を確かめる。Phase 4 の commit は、`repo.Begin()` で基準値を取った後に `repo.Git` で commit し、`origin` の同じ名前のブランチに push して作る（`-u` は付けない。設定を変えないため）。新しい補助が要れば `cmd/fixprguard/test_helpers.go`（`//go:build test`）に置く。
    -   `check`: 終了コード `0` で、出力の JSON の項目が `protocol`・`base`・`head`・`commits` のちょうど 4 つであり、`protocol` が `fixprguard.Protocol`、ほかの 3 つがリポジトリの値（基準値の commit、HEAD、Phase 4.5 の commit の数）と等しいこと。ブランチを切り替えたときに終了コード `1` で `parts` が `branch` であること。
    -   `push-review`: 終了コード `0` で、出力が `protocol`・`pushed`・`commit` であり、`commit` が HEAD と等しいこと。Phase 4.5 の commit がないとき、終了コード `2` になること（push しないことはステップ 4-1 の行が確かめる）。リモートの pre-receive の hook で push を失敗させたとき、終了コード `1` で `stop` が `git-failed`、標準エラーの 1 行が規則の名前 `push` を含むこと（1.2）。
    -   どちらも: `--commit` の形の誤りで、終了コード `2` になること。`head` で止まったときに、標準エラーの 1 行が規則の名前を含み、SHA を含まないこと（1 行全体を照合する）。`.git` のない場所は、層を分けて確かめられるステップ 3-1・4-1 のテストに任せる（この層では、もう 1 つの層も同じ終了コード `2` にするため）。
-   [x] 壊して確かめる: `specs` の `usesGH` を `true` にする（`TestGhTargetEnvRejected`）、出力の `Protocol` を入れない、`push-review` の送るものがないときの拒否を外す、push の規則の名前を変える。どの行が失敗したかをコミットメッセージに書く。

**成功の条件（フェーズ 5 の完了の条件）:** このフェーズだけでは、文書が `--protocol 5` のままなので、`TestDocumentedProtocolMatches` と `TestDocumentedInvocationsParse` の 2 つだけが失敗する。`make fmt`、`make lint`、`go test -tags test ./... -skip '^(TestDocumentedProtocolMatches|TestDocumentedInvocationsParse)$'` が通ることを確かめ、フェーズ 6 に進む。`/runplan` は、フェーズ 5 と 6 を 1 つの実装のまとまりとして進める。

### フェーズ 6: コマンドの文書・ガイドと、文書のガード（設計書 8 章の手順 6）

#### ステップ 6-1: `--protocol 6`

**変更するファイル:** `.claude/commands/fixpr.md`、`.claude/commands/runpr.md`、`docs/dev/guides/fixprguard_guide.md`

-   [x] `fixpr.md`・`runpr.md` のすべての `--protocol 5` を `--protocol 6` にし、`runpr.md:195` の出力の例の `"protocol":5` を `"protocol":6` にする（1.4）。
-   [x] ガイドの例の `--protocol 5`、出力の `"protocol":5`、4 章の「現在の番号は `5`」を `6` にする（1.4 の数）。

**成功の条件:** `TestDocumentedProtocolMatches` が通る。

#### ステップ 6-2: `local-cli-review.md` の GUARD と ROOT

**変更するファイル:** `.claude/commands/_lib/local-cli-review.md`

-   [x] 入力に、任意の入力 **GUARD** と **ROOT** を、設計書 3.6 のとおりに加える。GUARD の項目は、太字のラベル `**GUARD**` で始める（ステップ 6-8 のガードが探す）。
-   [x] 設計書 3.6 の表の `local-cli-review.md` の時点（手順 1 の最初、手順 3 の最初、手順 5 の build checks の前・README の同期の検査の前・stage の前、手順 6 の `coderabbit review findings --clear` の前）に、GUARD を実行する文を書く。手順 3 では、GUARD を渡されたとき、`git rev-parse --show-toplevel` を起動せず、ROOT への単独の `cd` の後に GUARD を実行する。
-   [x] 加える文は、どれも GUARD を渡されたときだけに働く形にする。渡されないときの文と振る舞いは変えない（AC-11）。手順 7（記録の行）は変えない（AC-03。ステップ 7-3 の確かめの対象）。

**成功の条件:** ステップ 6-8 のガードが通る。GUARD を渡されたときだけに働く形かは、PR-1 のレビューで読む。

#### ステップ 6-3: `fixpr.md`

**変更するファイル:** `.claude/commands/fixpr.md`

-   [x] Phase 1: `begin` が通った直後に、今の場所を `pwd` で記録することを書く（設計書 3.6「実行する場所」）。
-   [x] Phase 4.5 を、設計書 3.6〜3.8 のとおりに書き直す。内容は、次の順に書く。最初の `check`（BASE は出力の `base`）、手順に渡す BASE・SCOPE・SUMMARY・GUARD・ROOT、手順が戻った直後の `check`（`commits` とセッションが数えた commit の数の照合、HEAD3）、README の同期の検査、`README.en.md` の stage の前の `check`、`push-review`（`commits` が 1 以上のときだけ）。`git push`・`git commit -m` と書かない（1.2）。`check` と `push-review` の呼び出しの形は、コードブロックの行としても書く（`TestDocumentedInvocationsParse` はコードブロックの `fixprguard ` で始まる行だけを解析するため。`cmd/fixprguard/docs_test.go:187-204`）。
-   [x] Phase 3.5 の止まったときの記述（`:680-686`）の `parts` の値の列挙に `branch` を加える（1.4）。
-   [x] 止まったときの振る舞いと、報告に使う値（設計書 3.8）を Phase 4.5 に書く。止まった後に git を起動しないこと、`push-review` の結果が不明な場合、最初の `check` で止まった場合を含める。
-   [x] 「The guard tool」: 番号の合わないツールの案内（設計書 3.7。診断が `--protocol does not match` を含むときの 2 つの直し方）を書く。「Stops on the ignored files」に、`check`・`push-review` と、GUARD として実行した `check` の停止を加える（設計書 3.7）。
-   [x] 「Authorization」: `git push` を Phase 4（`push`）と Phase 4.5（`push-review`）でガードツールが行うと書き直し、`README.en.md` の commit が `push-review` で push されると直す（設計書 3.7）。
-   [x] Final report: 設計書 3.8 の表のとおりに、「Stops to act on」（7）、「Editorial corrections」（9）、「Local CLI review」（10）を直す。1 の要約の行に、Phase 4.5 の比較で止まったことを示す語を加える。
-   [x] 書いた内容を設計書と照合する。設計書 3.6 の表の 10 の時点、3.7 の各項目、3.8 の各項目（報告に使う値、「少なくとも」の数、push の結果が不明な場合、最初の `check` で止まった場合）と Final report の表のそれぞれについて、それを書いた `fixpr.md`・`local-cli-review.md`・`runpr.md` の箇所を対応させ、その対応をコミットメッセージに書く。PR-1 のレビューは、この対応に沿って読む。

**成功の条件:** ステップ 6-8 のガードが通る。内容は PR-1 のレビューで読む。

#### ステップ 6-4: `runpr.md`

**変更するファイル:** `.claude/commands/runpr.md`

-   [x] step 1 の `head` の呼び出し（`:173-182`）に、番号の合わないツールの案内（設計書 3.7）を足す。この診断を、fork の PR の停止として報告しないことを書く。
-   [x] step 1 の standalone の変更の項目（`:117-171`。ローカルの CLI レビュー、README の同期、push を含む）は変えない（AC-11。ステップ 7-3 の確かめの対象）。

**成功の条件:** `TestDocumentedInvocationsParse` と `TestRunprDocContract` が通る。内容は PR-1 のレビューで読む。

#### ステップ 6-5: `_context.md` の「Guard tool」

**変更するファイル:** `.claude/commands/_context.md`

-   [x] 「Guard tool」の行（`:189`）に、Phase 4.5 で `check` が git の起動の前に比べ、`push-review` が push すること、Phase 4.5 の commit が範囲の検査を受けないことを書く（設計書 3.9、AC-12）。

**成功の条件:** PR-1 のレビューで読む。

#### ステップ 6-6: ガイド

**変更するファイル:** `docs/dev/guides/fixprguard_guide.md`

-   [x] 設計書 3.9 の「`fixprguard_guide.md`」の各項目（2 章、4 章、5 章、7.1、7.2、8 章、9 章）を書く。5 章の節の見出しは、既存の節と同じく ``### `check --baseline TOKEN --commit SHA` `` と ``### `push-review --baseline TOKEN --commit SHA` `` の形にする。
-   [x] 基準値と比べるサブコマンドを列挙する箇所に `check`・`push-review` を加え、書き直す箇所（`:31`、7.2.4 と 7.2.5 の `push` の後の書き換え、7.1 の範囲外の変更の行）を書き直す（どちらも 1.4 の一覧）。
-   [x] 7.2 の新しい項目は、1.3 の置き場のとおり、7.2.3 と 7.2.4 に書く。
-   [x] ステップ 1-1 で見つけた VS Code の書き込み（新しく作ったブランチの `branch.<今のブランチ>.vscode-merge-base`）を、7.2.5 の正当な書き込みと 8 章の `config` の対処に書く（書き込みが済んでからやり直す）。
-   [x] 8 章の `git-state-changed` の行に、`finish` が commit の後に `branch` で止まったとき、commit が基準値のブランチに残ることと、取り消す前に基準値のブランチへ戻ることを書く。同じ行の「HEAD と push 先をその後に比べる」に、ブランチを加える。
-   [x] 8 章に、`head` の規則の名前の表を置く（ステップ 6-8 のガードが探す）。`line-too-long` は、基準値の commit に着かない長い列（新しい `main` に rebase したブランチなど）も含むことを書く。`git-failed` の行に、`push-review` の push そのものの失敗が規則の名前 `push` で示され、そのときは push が届いたかが分からないことを書く（1.2、設計書 3.8）。
-   [x] 新しい内容を、もとになる事実と照合する。5 章の出力の例は `ReviewState`・`PushResult` の JSON のタグとステップ 5-1 のテストの出力と、規則の名前の表はステップ 3-1 の一覧と、2 章・4 章の終了コード `2` の条件はステップ 3-1・4-1・5-1 のテストと、それぞれ照合する。照合したものをコミットメッセージに書く。

**成功の条件:** `go test -tags test ./cmd/fixprguard` が通る。内容は PR-1 のレビューで読む。

#### ステップ 6-7: `package_reference.md`

**変更するファイル:** `docs/dev/conventions/package_reference.md`

-   [x] `cmd/fixprguard` の行（`:16`）のサブコマンドの一覧に `check`・`push-review` を英語で加える（1.4）。

**成功の条件:** PR-1 のレビューで読む。

#### ステップ 6-8: 文書のガード

**変更するファイル:** `cmd/fixprguard/docs_test.go`、`cmd/yt2column/fixpr_docs_test.go`

-   [x] `TestDocumentedInvocationsParse` の `fixpr.md` の期待（`docs_test.go:217`）に `cmdCheck`・`cmdPushReview` を加える。
-   [x] `cmd/fixprguard/docs_test.go` に、ガイド 8 章の `head` の規則の名前の表の 1 列目がコードの区切りで挙げる名前の集合が、ステップ 3-1 の一覧と等しいことを確かめるテスト（`TestGuideListsHeadLineRules`）を加える（1.3）。
-   [x] `TestDocumentedProtocolMatches` の `"protocol":N` の照合を、`fixprguard` の出力を示すコマンドの文書（`fixpr.md`・`runpr.md`）にも広げる（1.3）。
-   [x] `fixpr_docs_test.go` に Phase 4.5 の見出しの定数を加え、`TestFixprguardCallSites` に、Phase 4.5 が `fixprguard check ` と `fixprguard push-review ` を呼ぶ 2 行を加える。
-   [x] `TestRemovedCommandLiterals` の Phase 4.5 の例外とその注記（`:1115-1116`）を外し、見出しの一覧に Phase 4.5 を加える。
-   [x] 設計書 7.4 の GUARD の構造の検査（`TestLocalReviewGuard`）を加える。`local-cli-review.md` の入力の節に `**GUARD**` で始まる項目があること、手順 1・3・5・6 の各 step の本文が `GUARD` を含むこと、`fixpr.md` の Phase 4.5 が `**GUARD**` に `fixprguard check` を渡すこと。節は、見出し・太字のラベル・step の番号で見つける。この検査は、`CLAUDE.md`「Document tests」の例外のうち、エージェントが信頼の境界で実行するコマンド（`fixprguard` の呼び出し箇所）に当たる（設計書 7.4）。1 つの step の中の時点の数は確かめず、PR のレビューで読む（設計書 5.3）。
-   [x] 壊して確かめる: ガイドの規則の名前の表から 1 つ消す・1 つ足す、`runpr.md` の出力の例の番号を変える、Phase 4.5 の `push-review` の呼び出しを消す、Phase 4.5 に `git push` の語を戻す、手順 5 の GUARD の文をすべて消す、入力の `**GUARD**` の項目を消す、`fixpr.md` の GUARD に別のコマンドを渡す。

**成功の条件（フェーズ 6 の完了の条件）:** `make fmt`、`make test`、`make lint` が通る。

### フェーズ 7: ADR-0015 のずれの記述と、タスク 0030 の設計書の注記（設計書 8 章の手順 7）

#### ステップ 7-1: ADR-0015 のずれの記述を取り除く

**変更するファイル:** `docs/dev/adr/0015-untrusted-review-threads.md`

-   [x] PR を作る前に、`git grep -nE 'issues/426([^0-9]|$)' origin/main -- docs/dev/adr/` で、ずれの記述を探す（`adr.md`「Discrepancy statements」）。決定の節の「この決定は、まだ…このずれは #426 で扱う。」の 3 文（`85feed05` で `:24` の末尾）を取り除く。
-   [x] 決定の節の残りが、フェーズ 2〜6 の実装と合うことを読んで確かめる。合わなければ、決定の変更として利用者に確かめる。
-   [x] コミットメッセージに、訂正の行（`Editorial correction: docs/dev/adr/0015-untrusted-review-threads.md: ...; no decision changed` の形。`requirements_process.md`「Recording in the PR description」）を書く。経緯の節には足さない（`adr.md`「History entries」）。

**成功の条件:** `make test`（`TestADRRecords` を含む）が通る。

#### ステップ 7-2: タスク 0030 の設計書の注記と、その確かめ

**変更するファイル:** `docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md`、`docs/tasks/0076_fixpr_local_review_guard/_checks/notes_only.sh`（新規）、`docs/tasks/0076_fixpr_local_review_guard/03_implementation_plan.md`（AC-13 の記録）

-   [x] 3.7・5.2・5.3 の節の初めに、`> **注記（タスク 0076）:** ...` の引用ブロックを 1 つずつ加える。内容は設計書 3.9 の「タスク 0030 の設計書」のとおりで、本タスクの要件書と設計書へのリンクを持つ。同じ節に 0057 の注記がある場合は、その後に置く。本文は変えない（AC-13）。
-   [x] コミットメッセージに、節ごとに 1 行の訂正の行を書く。
-   [x] `_checks/notes_only.sh <upstream> <head>` を書く。merge-base と head の間で、0030 の設計書について次を確かめる。(1) 文書が変わり、ファイルのモードは変わっていない。(2) 差分に消した行がなく、加えた行はどれも `>` で始まるか空である。(3) `## Document Status` の表がバイト単位で同じである（0030 の設計書には `## 変更履歴` の節がない）。(4) 3 つの節のそれぞれに、`> **注記（タスク 0076）:**` で始まる引用ブロックがちょうど 1 つ加わり、ほかの節には加わっていない。(5) その注記が、節の本文より前（0057 の注記があればその後）にある。(6) 各注記が、本タスクの文書への相対リンク（`../0076_fixpr_local_review_guard/`）を持ち、リンク先のファイルが head にある。引数・出力・終了コード・書き方は、`docs/tasks/0057_reviewer_relay_guard/_checks/notes_only.sh` の形に合わせる（POSIX の `sh` と `git`）。
-   [x] `_checks/notes_only.sh origin/main HEAD` を PR のブランチで 1 回実行し、出力と 2 つのコミット ID を 7 章の AC-13 の行に記録する。記録するのは実行した時点の head である。その後の commit が 0030 の設計書を変えたときだけ、実行し直して記録を改める。
-   [x] 壊して確かめる: 次のそれぞれを一時的なコミットで作り、それに対して実行して終了コード `1` になることと、出た理由の行が壊した性質のものであることを確かめ、一時的なコミットを捨てる。本文の 1 行の書き換え、`>` で始まらない行の追加、`## Document Status` の節の中への `>` で始まる行の追加（(2) を通り (3) だけに当たる）、ある節の注記の削除、ある節への 2 つ目の注記、対象の外の節への注記、注記を節の本文の後ろへ移す、リンクのない注記、ファイルのモードの変更。結果を AC-13 の行に書く。

**成功の条件:** スクリプトが終了コード `0` で終わり、9 つの壊した場合のどれもが、その性質の理由で終了コード `1` になる。

#### ステップ 7-3: ほかの呼び出し元と記録の行が変わらないことの確かめ

**変更するファイル:** `docs/tasks/0076_fixpr_local_review_guard/_checks/unchanged_callers.sh`（新規）、`docs/tasks/0076_fixpr_local_review_guard/03_implementation_plan.md`（AC-03・AC-11 の記録）

-   [x] `_checks/unchanged_callers.sh <upstream> <head>` を書く。merge-base と head の間で、次がバイト単位で同じであることを確かめる（設計書 7.4 の最後の段落）。
    -   `.claude/commands/runplan.md` と `.claude/commands/_lib/task-pr-handoff.md` の全体。
    -   `runpr.md` の step 1 の standalone の変更の項目（`85feed05` で `:117-171`。ローカルの CLI レビュー、README の同期、push を含む）。項目は、`- If none exists, it is a standalone change:` の行から、同じ字下げの次の `-` の行の前までとする。
    -   `local-cli-review.md` の手順 7（記録の行。AC-03）。`7. **Record lines.**` の行から `8. **SUMMARY.**` の行の前までとする。
-   [x] 項目と手順 7 は、どちらの版でもちょうど 1 つ見つからなければ、終了コード `1` で止める（見つける文が変わって、両方の版で見つからないまま通るのを防ぐ）。
-   [x] PR のブランチで 1 回実行し、出力と 2 つのコミット ID を 7 章の AC-03・AC-11 の行に記録する。記録するのは実行した時点の head である。その後の commit が対象のファイルを変えたときだけ、実行し直して記録を改める。
-   [x] 壊して確かめる: 4 つの対象のそれぞれに 1 文字の変更を一時的なコミットで作り、終了コード `1` になることを確かめる。2 つの見つける行（standalone の変更の項目と手順 7）のそれぞれの文を変えたとき（見つからない）も終了コード `1` になることを確かめる。`runpr.md` の step 1 の `head` の項目だけを変えたときに終了コード `0` のままであることも確かめる。結果を記録する。

**成功の条件:** スクリプトが終了コード `0` で終わり、6 つの壊した場合のどれもが終了コード `1` になる。

#### ステップ 7-4: PR-1 を作る前の確認

**変更するファイル:** なし

-   [x] PR-1 を作る直前（このブランチの最後の commit の後）に、ほかの worktree で `/fixpr`・`/runpr` が動いていないことを利用者に確かめる（ツールを入れ直すと、ほかの worktree の文書の `--protocol 5` が終了コード `2` で止まるため）。
-   [x] このブランチで、作業ツリーが clean な状態で `make install-fixprguard` を実行する（ガイド 3 章。PR-1 の `/runpr` は新しい `runpr.md` と `fixpr.md` を新しいツールで実行する）。

**成功の条件:** `go version -m "$(command -v fixprguard)"` の `vcs.revision` がこのブランチの head で、`vcs.modified` が `false` である。この後の commit（`/fixpr` の修正を含む）が `cmd/fixprguard` か `internal/fixprguard` を変えたら、その `/fixpr` の実行が終わった後、`/runpr` を続ける前に入れ直す。1 回の `/fixpr` の実行の中では、Phase 4 が変えたツールのコードではなく、入れてあるツールで Phase 4.5 を行う（番号は同じ `6` である）。

**フェーズ 7 の完了の条件:** `make test`、`make lint`、文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview ./internal/pipeline`。CI の `doc-guard` と同じ形）が通る。

### PR-1 作成ポイント: check, push-review, protocol 6, and the Phase 4.5 documents

**対象ステップ**: 1-1 / 2-1 / 3-1 / 4-1 / 5-1 / 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 7-1 / 7-2 / 7-3 / 7-4

**推奨タイトル**: `feat(0076): /fixpr の Phase 4.5 で git の起動と push の前にガードツールが git の状態を比べるようにする`

**レビュー観点**: 系列の判定の 5 つの規則と上限、`--commit` が検査を緩めないこと、`push-review` の push 先の確認と送り元、push の失敗の規則の名前 / `fixpr.md` と `local-cli-review.md` の時点が設計書 3.6 の表と一致し、GUARD を渡さない呼び出し元が変わらないこと / 止まったときの報告が git を起動せずに設計書 3.8 の値を示すこと / ADR-0015 の決定が実装と合い、0030 の設計書が注記だけを加え、`_checks/` の 2 つのスクリプトの記録がそろっていること

**実装モデル要件**: frontier-required

**判定理由**: `protocol` を上げ、既存のサブコマンドの出力の契約と新しい検査を同時に変えるセキュリティの関門（パネルモードの条件）を含み、ステップ 1-1 は設計の前提を確かめる観察である

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 8: マージと再インストールの後の確かめ（設計書 8 章の手順 8）

#### ステップ 8-1: 再インストールと、作業中のブランチの確認

**変更するファイル:** `docs/tasks/0076_fixpr_local_review_guard/03_implementation_plan.md`（記録）

-   [ ] 利用者の許可を得て `main` を最新にし（`git pull` はネットワークの操作である）、`main` で `make install-fixprguard` を再実行する（`main` がほかの worktree で使われていれば、その worktree で行う）。`go version -m "$(command -v fixprguard)"` の `vcs.revision` が PR-1 の squash commit で、`vcs.modified` が `false` であることを確かめて記録する。
-   [ ] `git worktree list` で作業中のブランチを列挙し、文書がまだ `--protocol 5` を渡すブランチ（`main` を取り込んでいないもの）を利用者に示す。古い文書の `--protocol 5` は、新しいツールで終了コード `2` になる。取り込みは、そのブランチを使うセッションで行う（ほかのセッションの作業と衝突しないため）。示したブランチを、このステップに書く。

**成功の条件:** 再インストールの commit と、示したブランチの一覧（なければ「なし」）が記録されている。

#### ステップ 8-2: `/fixpr` の Phase 4.5 を通す観察

**変更するファイル:** `docs/tasks/0076_fixpr_local_review_guard/03_implementation_plan.md`（記録）

-   [ ] 再インストールの後に、`/fixpr` の Phase 4.5 が動いた実行を観察する（設計書 7.6 の 2 つ目。文書だけの小さな PR が望ましい）。Phase 4.5 は、Phase 4 が commit を作ったときにだけ動く。PR-1 の `/runpr` で、ステップ 7-4 で入れたツールの Phase 4.5 が動いていれば、その結果もあわせて記録する。
-   [ ] 観察したことを記録する。PR の番号、そのブランチを `/fixpr` の直前に作ったか（ステップ 1-1 の VS Code の書き込みのため）、各時点の `check` が止まらなかったか、`push-review` が push したか。止まったときは、`stop`・`parts`・規則の名前と、ガイド 7.2.5 の正当な書き込みに当たるかの判断。
-   [ ] PR-2 のブランチで作業を始める時点で、Phase 4.5 が動いた実行がまだなければ、利用者に進め方を確かめる（観察を待つか、観察していないことを記録して issue を開いたままにするか）。

**成功の条件:** 観察の結果、または利用者が決めた進め方が記録されている。

**フェーズ 8 の完了の条件:** 文書のガードが通る。

### PR-2 作成ポイント: post-merge observations

**対象ステップ**: 8-1 / 8-2

**推奨タイトル**: `docs(0076): マージの後の再インストールと /fixpr の Phase 4.5 の観察を記録する`

**レビュー観点**: 再インストールの commit と、`main` を取り込んでいないブランチの一覧 / Phase 4.5 の観察の結果と、止まった場合の原因の判断

**実装モデル要件**: standard

**判定理由**: パネルモードの条件にも条件付きの確認の 2 つ以上にも当たらず、既存コード調査結果に競合する案もない。観察の記録だけである

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 内容 | 成果物 |
|---|---|---|
| M1 | 実際のツールが比べる状態を書き換えないことを確かめる | フェーズ 1 の記録 |
| M2 | ガードツールが Phase 4.5 の比較と push を行える | フェーズ 2〜5（`check`・`push-review`、`Protocol` の `6`） |
| M3 | `/fixpr` の Phase 4.5 が比較と `push-review` を使い、文書が実際と一致する | フェーズ 6・7（PR-1） |
| M4 | マージの後の実際の `/fixpr` で、Phase 4.5 が止まらずに push まで進むことを確かめる | フェーズ 8（PR-2） |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 2-1 / 3-1 / 4-1 / 5-1 / 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 7-1 / 7-2 / 7-3 / 7-4 | 実際のツールでの確かめ、`PartBranch` と HEAD の決まり、系列の判定、`check`・`push-review`、`Protocol` の `6`、コマンドの文書・ガイド・文書のガード、ADR-0015 と 0030 の注記、`_checks/` のスクリプト | frontier-required |
| PR-2 | 8-1 / 8-2 | マージの後の再インストールと、Phase 4.5 の観察の記録 | standard |

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** `PartBranch` と HEAD の決まりは `gitstate_test.go`・`worktree_flow_test.go` の既存の表に行を加え、系列の判定・`CheckReviewState`・`PushReviewed` は新しい `review_test.go` で確かめる（設計書 7.1）。git は一時ディレクトリの実際のリポジトリで試し、`gh` と `make` は起動しない。
-   **結合テスト:** `cmd/fixprguard/main_test.go` で `check`・`push-review` を実行し、出力の JSON の形、終了コード、`gh` を起動しないことを確かめる（設計書 7.3）。
-   **壊して失敗を確かめる:** 設計書 7.2 の項目と、各ステップの「壊して確かめる」の項目を確かめ、コミットメッセージに書く。
-   **セキュリティテスト:** 書き換えられた config で `git config --list` 以外の git を起動しないこと、マージの commit と基準値の commit から続かない列を押し通せないこと、確かめていない commit を送らないことを、ステップ 3-1・4-1 のテストで確かめる（設計書 7.5）。
-   **文書のガード:** `--protocol` の番号、`fixpr.md` の呼び出し、Phase 4.5 に `git push` がないこと、GUARD の構造、ガイドの `parts` と規則の名前（ステップ 6-8）。文面は PR のレビューで読む（設計書 7.4）。
-   **前後の関係:** タスク 0030 の設計書に注記だけを加えること、ほかの呼び出し元と記録の行が変わらないことは、`_checks/` のスクリプトを一度実行して確かめる（ステップ 7-2・7-3）。
-   **実際のツールでの確かめ:** フェーズ 1 とステップ 8-2 で、観察を記録する（設計書 7.6）。
-   **後方互換:** 古いツールと新しい文書、新しいツールと古い文書の組み合わせが終了コード `2` で止まることを、`TestProtocolMismatch`（`Protocol + 1` と `Protocol - 1`、`check`・`push-review` を含むすべてのサブコマンド）で確かめる（AC-10）。
-   **カバレッジ:** テストを削除しないので、`make coverage-compare` は要らない。既存の 2 行は期待を変えるだけである。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| CodeRabbit CLI などのツールが、比べる状態を書き換える | Phase 4.5 が毎回止まる | フェーズ 1 で、コードを書く前に確かめる。書き換えれば、設計の変更として止まる（ステップ 1-1） |
| PR-1 のブランチでツールを入れると、ほかの worktree の `/fixpr`・`/runpr` が終了コード `2` で止まる | 並行する作業が止まる | ステップ 7-4 で、ほかの worktree で動いていないことを確かめてから入れる。マージの後に、`main` を取り込んでいない作業中のブランチを利用者に示す（ステップ 8-1）。PR-1 がマージの前に長く止まるときは、`main` で `make install-fixprguard` を実行して元のツールに戻す |
| 権限の確認への答えで `.claude/settings.local.json` が書き換わり、Phase 4.5 が `ignored` で止まる | 返信と解決がされない | 要件書 4.3 が受け入れた停止である。ガイド 8.1 に従って確かめてからやり直す。フェーズ 1 では「今後は聞かない」と答えない |
| PR-1 が大きく、レビューに時間がかかる | 日程の遅れ | 1.3 の理由で分けない。`レビュー観点` を、系列の判定、`push-review` の送り元と push 先の確認、Phase 4.5 の時点と報告に絞る |
| マージの後、Phase 4.5 が動く実行がしばらくない | ステップ 8-2 を記録できない | 利用者に進め方を確かめる（ステップ 8-2） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 2-1 / 3-1 / 4-1 / 5-1 / 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 7-1 / 7-2 / 7-3 / 7-4）
- [ ] PR-2 マージ済み（対象ステップ: 8-1 / 8-2）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種類の列は、`test`（実行して誤った振る舞いで失敗するテスト）、`static`（ガードのテスト・`make` の対象）、`diff`（`_checks/` のスクリプトを PR のブランチで一度実行し、出力と 2 つのコミット ID を記録する）、`manual`（PR のレビューでの読み取り、観察）である。

| AC | 確かめ | 種類 | 実装 |
|---|---|---|---|
| AC-01 | `cmd/yt2column/fixpr_docs_test.go` の GUARD の構造の検査（手順 1・3・5・6 と入力の `**GUARD**`、`fixpr.md` の Phase 4.5 が渡す `fixprguard check`）、`TestFixprguardCallSites`（Phase 4.5 の `check`）。1 つの step の中の時点（手順 5 の 3 つ）と `fixpr.md` の 3 つの時点は PR-1 のレビューで読む（設計書 5.3） | static・manual | ステップ 6-2・6-3・6-8 |
| AC-02 | `internal/fixprguard/review_test.go`（系列の判定の各行、`--commit` との不一致、ブランチの切り替え、config・hooks・info・対象のファイルの各行、比較の順序）、`internal/fixprguard/gitstate_test.go::TestBaselineDetectsChange` | test | ステップ 2-1・3-1 |
| AC-03 | `_checks/unchanged_callers.sh`（`local-cli-review.md` の手順 7 の本文）。`fixpr.md` の README の同期の commit のメッセージは PR-1 のレビューで読む。手順 5 の修正の commit のメッセージの規則（記録の行を入れる）と、ROOT の入力に加えた「GUARD を渡されたとき HEAD3 の行は呼び出し元に任せる」文は、スクリプトの対象の外なので、PR-1 のレビューで読む。記録（2026-11-10）: `_checks/unchanged_callers.sh origin/main HEAD` を merge-base `9f57e8c3086a607a8cf36f51d70a934a9970f673`、head `800e19413d2b9066e1e4716e04e9e5f95412bbc2` で実行し、終了コード `0`、出力の最後の行「runplan.md, task-pr-handoff.md, runpr.md's standalone-change item, and local-cli-review.md step 7 are unchanged」。壊した 6 つの場合（`runplan.md`・`task-pr-handoff.md`・standalone の変更の項目・手順 7 のそれぞれの 1 文字の変更と、2 つの見つける行の変更）は、どれも終了コード `1` だった。`runpr.md` の step 1 の `head` の項目だけを変えた場合は、終了コード `0` のままだった | diff・manual | ステップ 6-3・7-3 |
| AC-04 | `internal/fixprguard/review_test.go`（止まるどの行でも `git push` を起動せず、リモートが動かない）。`fixpr.md` の止まったときの振る舞いは PR-1 のレビューで読む | test・manual | ステップ 4-1・6-3 |
| AC-05 | `internal/fixprguard/review_test.go`（`branch`・`head`・`remote` の区別、規則の名前、止まった時点の記録のファイル）、`cmd/fixprguard/main_test.go`（標準エラーの規則の名前）、`cmd/fixprguard/docs_test.go::TestGuideNamesGitStateParts` とステップ 6-8 の規則の名前のガード。Final report の項目は PR-1 のレビューで読む | test・static・manual | ステップ 2-1・3-1・5-1・6-3・6-6・6-8 |
| AC-06 | `cmd/yt2column/fixpr_docs_test.go::TestRemovedCommandLiterals`（Phase 4.5 を含む）、`TestFixprguardCallSites`（Phase 4.5 の `push-review`）。ツールが push することは `internal/fixprguard/review_test.go` と `cmd/fixprguard/main_test.go` の `push-review` の成功の行。`commits` が 1 以上のときだけ呼ぶことは PR-1 のレビューで読む | static・test・manual | ステップ 4-1・5-1・6-3・6-8 |
| AC-07 | `internal/fixprguard/review_test.go`（push 先が `--commit` でない・基準値の OID のままの行、比較の各行で push しないこと） | test | ステップ 4-1 |
| AC-08 | `internal/fixprguard/review_test.go`（設定された push の refspec とタグ、`ls-remote` の後に HEAD を進める行）、既存の `worktree_flow_test.go::TestPushExplicitRefspec` | test | ステップ 4-1 |
| AC-09 | `internal/fixprguard/review_test.go` と `cmd/fixprguard/main_test.go`（Phase 4.5 の commit がないときの `push-review` の拒否）。`fixpr.md` が呼ばないことは PR-1 のレビューで読む | test・manual | ステップ 4-1・5-1・6-3 |
| AC-10 | `cmd/fixprguard/main_test.go::TestProtocolMismatch`（`check`・`push-review` を含む）、`cmd/fixprguard/docs_test.go::TestDocumentedProtocolMatches`。案内の内容は PR-1 のレビューで読む | test・static・manual | ステップ 5-1・6-1・6-3・6-4 |
| AC-11 | `_checks/unchanged_callers.sh`（`runplan.md`・`task-pr-handoff.md` の全体と、`runpr.md` の step 1 の standalone の変更の項目）。`local-cli-review.md` に加えた文が GUARD を渡されたときだけに働くことは PR-1 のレビューで読む。記録（2026-11-10）: `_checks/unchanged_callers.sh origin/main HEAD` を merge-base `9f57e8c3086a607a8cf36f51d70a934a9970f673`、head `800e19413d2b9066e1e4716e04e9e5f95412bbc2` で実行し、終了コード `0`、出力の最後の行「runplan.md, task-pr-handoff.md, runpr.md's standalone-change item, and local-cli-review.md step 7 are unchanged」。壊した 6 つの場合（`runplan.md`・`task-pr-handoff.md`・standalone の変更の項目・手順 7 のそれぞれの 1 文字の変更と、2 つの見つける行の変更）は、どれも終了コード `1` だった。`runpr.md` の step 1 の `head` の項目だけを変えた場合は、終了コード `0` のままだった | diff・manual | ステップ 6-2・7-3 |
| AC-12 | `TestDocumentedInvocationsParse`・`TestDocumentedProtocolMatches`・`TestGuideNamesGitStateParts`・ステップ 6-8 の規則の名前のガード（`cmd/fixprguard/docs_test.go`）、`TestADRRecords`。各文書の内容は PR-1 のレビューで読む | static・manual | ステップ 6-1〜6-7・6-8・7-1 |
| AC-13 | `_checks/notes_only.sh origin/main HEAD`。注記の文面は PR-1 のレビューで読む。記録（2026-11-10）: merge-base `9f57e8c3086a607a8cf36f51d70a934a9970f673`、head `95292a57ecf6e381b501ae34e0c0435976e2f256`、終了コード `0`、出力の最後の行「docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md adds one task 0076 note per listed section, before the body, and changes nothing else」。壊した場合は、計画の 9 つに 1 つを加えた 10 で、どれも一時的なコミットに対して終了コード `1` になり、その性質の理由の行を出した。本文の 1 行の書き換え（removed と added without >）、`>` で始まらない行の追加（added block does not start with the note）、`## Document Status` の節の中への空行の追加（changed its Document Status section だけ。(2) は空行を飛ばすので通る）、ある節の注記の削除（holds 0 task 0076 notes）、ある節への 2 つ目の注記（holds 2）、対象の外の節への注記（not one per listed section）、注記を節の本文の後ろへ移す（comes after the body）、リンクのない注記（has no link）、ファイルのモードの変更（changed its file mode）、5.3 で 0076 の注記を 0057 の注記より前に移す（before a task 0057 note）。このスクリプトの (2) は、加えた空行を許すので、本文の表の中への空行の追加と、注記と同じ hunk の後ろの別の `>` の引用は通す（0057 の形に合わせた限界）。注記の 0076 の文書でないリンク（ガイドへのリンク）は確かめない。どちらも PR-1 のレビューで読む | diff・manual | ステップ 7-2 |
| AC-14 | 各 PR の `make test`・`make lint` と CI の `doc-guard` | static | すべてのステップ |

## 8. 成功基準 (Success Criteria)

-   **機能:** 7 章のすべての AC の確かめが済んでいる。
-   **品質:** 各 PR で `make test`・`make lint` が通り、壊して失敗することをコミットメッセージに記録している。
-   **セキュリティ:** 系列の判定の各規則、`push-review` の push 先の確認と送り元、比較の順序が、1 つずつテストされている。`fixpr.md` の Phase 4.5 は `git push` を含まない。
-   **文書:** `fixpr.md`・`local-cli-review.md`・`runpr.md`・`_context.md`・ガイド・ADR-0015・0030 の注記・`package_reference.md` が実際と一致し、PR のレビューで読まれている。
-   **実際の動作:** フェーズ 1 とステップ 8-2 の観察が記録されている。

## 9. 次のステップ (Next Steps)

-   PR-2 のマージの後、タスクの issue（#426）を閉じる（`/runpr` ステップ 5）。ステップ 8-2 で観察していないことを記録した場合は、利用者が決めたとおりにする。
-   Phase 5・6 の `reply`・`edit-pr`・`append-pr` の前の比較は、要件書 2.3 のスコープ外である（設計書 9 章）。

## 変更履歴

- 2026-11-10 承認（isseis、#782）
