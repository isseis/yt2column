# 実装計画書：タスク文書の Comments を今の状態の理由に絞り、編集上の訂正を PR 本文へ、状態の遷移を変更履歴へ移す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-01 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番、H-NN は [design_handoff.md](design_handoff.md) の項目、「設計書 3.4.1 節」などは設計書の節を指す。「規則」「対象のコマンドの定義」「状態の遷移」「変更履歴」は設計書の冒頭の定義と同じ意味で使う。本タスクに `implementation_handoff.md` はない。

既存のコードについての記述は、コミット `0755d504`（2026-11-01 の `main`）で確かめた。設計書が行番号を引用したコミット `8bcfd272` から `0755d504` までに、本タスクが変えるファイルのうち変わったものはない（`git diff --stat 8bcfd272 0755d504 -- .claude docs/dev cmd/fixprguard internal/fixprguard cmd/yt2column` の出力は `cmd/yt2column/extension_docs_test.go` の 1 ファイルだけで、このファイルは本タスクでは変えない）。そのため、設計書の `file:line` はそのまま使える。本書の行番号も `0755d504` のものである。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

規則（`docs/dev/conventions/requirements_process.md`）を、`Comments` には今の状態の理由だけを書き、状態の遷移は変更履歴に、編集上の訂正はコミットメッセージと PR 本文に書く形に改める。あわせて、規則に従って書き込むコマンドの定義と、`/fixpr` が訂正を記録するための `fixprguard` の 2 つの変更（`finish --body-file`・`append-pr`）を実装する。何をどこに書くかは設計書 2〜6 章で決まっている。本書は、それをどの PR・どのステップ・どの確かめで行うかを定める。

### 1.2. 実装の原則

-   設計書 8 章のフェーズの順（1 → 5）で実装する。本書のフェーズ 1〜5 は、設計書 8 章のフェーズ 1〜5 にそのまま対応する。PR の分け方は 3.2 節のとおりで、PR-1 のマージの後に PR-2 を始める。
-   書式は規則の 1 か所だけに書き、コマンドの定義は規則の節を名指しして参照する（設計書 1.1 節、AC-20）。コマンドの定義に、`Comments` の値、変更履歴の行の形、訂正の行と訂正の段落の形を書き写さない。
-   規則とコマンドの定義の文言は、PR のレビューで読んで確かめる（`manual`）。文言を照合する恒久のテストは加えない（設計書 7.1 節「ほかの文書のテスト」、`CLAUDE.md`「Document tests check values, structure, and executable examples, never prose」）。テストを加えるのは、`fixprguard` の振る舞いと、信頼の境界のコマンド（`fixprguard` の呼び出しの場所、本文を出力しない `--jq` の式、投稿のコマンドの一覧）だけである。
-   テストを書いたら、確かめる対象を壊して失敗することを確かめ、そのことをコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。
-   Go のコメント・識別子・文字列リテラルは英語で書く。規則とコマンドの定義の本文も英語で書く。変更履歴の見出しと行の例、訂正の段落の例は、タスク文書に書く日本語の値をそのままコードブロックで示す（設計書 3.2 節）。

### 1.3. 計画の際に決めたこと（2026-11-01）

-   **PR を 2 つに分ける:** PR-1 はフェーズ 1（`fixprguard` とその手引き）、PR-2 はフェーズ 2〜5（規則、コマンドの定義、文書のガード、AC-19 の確かめ）とする。1 つの PR にまとめると、その PR を `/runpr` で進める間、`/fixpr` はブランチの新しい `fixpr.md` に従って `finish --body-file` や `append-pr` を呼ぶ。しかし、インストール済みのツールは `main` から作った古いものである（エージェントはツールをインストールしない。`CLAUDE.md`「Tool Execution Safety」）。そのため、訂正のある回は終了コード `2` で止まる。分ければ、PR-1 のマージの後に利用者が `make install-fixprguard` で再インストールし、PR-2 を新しいツールで進められる。フェーズ 2〜4 を 1 つの PR にまとめることは、設計書 8 章が決めている。
-   **AC-19 の確かめのスクリプトはフェーズ 1 で作る:** 設計書 7.3 節は、本タスクの実装の PR ごとに、そのブランチでマージベースと head を比べるスクリプトを一度ずつ実行するとしている。設計書 8 章の順序ではスクリプトはフェーズ 5 で作るが、それでは PR-1 の時点でスクリプトがない。そこで、フェーズ 1 の最後のステップ（1-5）でスクリプトを作って PR-1 のブランチで実行し、フェーズ 5（ステップ 5-1）では PR-2 のブランチで実行し直す。設計書 7.3 節の「各 PR のブランチで実行する」をそのまま守れ、設計書を直さずに済む。

### 1.4. 既存コード調査結果

**`internal/fixprguard`**

-   **`finish` の commit:** `Finish`（`internal/fixprguard/worktree.go:219`）は、`w.git(ctx, "commit", "commit", "-q", "-m", CommitSubject(req.PR))`（`worktree.go:272`）で件名だけの commit を作る。`FinishRequest`（`worktree.go:198-200`）は `PR` だけを持つ。git は `cmdrunner.Runner` 越しに標準入力なしで起動される（`w.git`、`internal/fixprguard/gitstate.go:426`）。
    -   **採る方法:** 本文を渡したときは、検査した件名・空行・本文をメモリ上の 1 つの文字列にし、`-m` の 1 つの引数として `--cleanup=verbatim` とともに渡す。設計書 3.4.1 節の「git にはメモリ上の文を渡し、ファイルを git に読み直させない」を、標準入力を足さずに満たせる。本文の上限は 64 KiB（`maxTextFileBytes`、`internal/fixprguard/write.go:18`）なので、1 つの引数の長さは OS の上限に収まる。本文を渡さないときの呼び出しは変えない（設計書 3.4.3 節の条件 2）。
-   **commit の後の確認:** `checkCommit`（`worktree.go:395`）は HEAD・親・tree を確かめるが、メッセージは確かめない。`commitObject`（`gitstate.go:637-662`）は commit オブジェクトを `cat-file commit` で読み、最初の `\n\n` より前のヘッダーから `tree` と `parent` だけを取り出す。
    -   **採る方法:** `commitObject` が、最初の `\n\n` の後のメッセージも返すようにする。署名の `gpgsig` ヘッダーは、続きの行を空白で始めるので空行を含まない。そのため、最初の `\n\n` の後がメッセージである。メッセージの一致を確かめるのは、本文を渡したときだけとする。本文を渡さない `finish` にこの確認を加えると、commit-msg の hook がメッセージに trailer を足す環境で、今まで通っていた呼び出しが止まるようになる。これは設計書 3.4.3 節の条件 2（新しいツールは古い呼び出しを今までと同じ規則で扱う）に反する。
-   **本文のファイルの検査:** `readTextFile`（`write.go:28-50`）と `checkPostText`（`write.go:73-84`）をそのまま使う。訂正の行の形の検査は新しい関数として加える。`ErrInvalidInput` の説明のコメント（`internal/fixprguard/errors.go:12-15`）は `reply` と `edit-pr` だけを挙げているので、`finish` と `append-pr` も加える。
-   **PR 本文の書き込み:** `EditPR`（`write.go:297-319`）は、本文を `gh pr edit <n> --title=... --body-file -` の標準入力で渡す。`append-pr` の書き込みも同じ形にし、`--title` は渡さない。今の本文の取得には `gh pr view <n> --json body` を使う。その応答は、`Head` が `gh pr view <n> --json <fields>` の応答を読む方法（`internal/fixprguard/github.go:149` 以降）と同じく `strictjson` で検査して読む。失敗は `ghFailure`（`github.go:79`）で、本文も引数も含まない誤りにする。
-   **既存のテスト:** `TestFinishCommitSubject`（`internal/fixprguard/worktree_flow_test.go:332`）は件名と親を確かめる。本文がないときの確認には、このテストをそのまま使う。`TestBodyFileChecks`（`write_test.go:30`）は `readTextFile` の規則を網羅している。そのため、`finish` と `append-pr` のテストでは、`readTextFile` の規則を行ごとに繰り返さない。新しい規則の行と、`readTextFile` を通すことを示す 1 行だけを置く（`TestEditPRVerbatim` の `rejects_before_any_call` と同じ考え方。`write_test.go:369-370`）。hook がコミットを変える場合のテストは `TestFinishPostCommitTreeCheck`（`worktree_flow_test.go:239`）と `TestFinishStopsWhenHooksChangeConfig`（同 285）にあり、hook を置く補助はこれらを参考にする。

**`cmd/fixprguard`**

-   `specs`（`cmd/fixprguard/main.go:104-121`）の `cmdFinish` は `--baseline`・`--judged-set` だけを受け付ける。`--body-file` の名前（`flagBodyFile`）は `reply`・`edit-pr` が既に使っている。`editPR`（`main.go:666-682`）の形（`NewGitHubWithoutReviewer`、`prNumber`、`writeOutput`）を `append-pr` に使う。
-   `main_test.go:225-235` の表 `lines` は、`usesGH` のサブコマンドごとに 1 行を持つことを確かめる。`append-pr` を加えると、この表にも行が要る。
-   `TestDocumentedInvocationsParse`（`cmd/fixprguard/docs_test.go:184-213`）の期待するサブコマンドの一覧（同 189 行）に `append-pr` を加えるのは PR-2 である。`fixpr.md` が `append-pr` を呼ぶのは PR-2 からで、PR-1 で一覧に加えると、この時点の `fixpr.md` に呼び出しがないためテストが落ちる。
-   パッケージのコメント（`main.go:1-11`）はサブコマンドの名前を挙げていないので変えない。`stop` のコメント（`main.go:287-290`）と `writeOutput` のコメント（`main.go:627`）は `reply` と `edit-pr` を挙げているので、`finish` と `append-pr` も加える。

**文書のガード（`cmd/yt2column`）**

-   `TestFixprguardCallSites`（`cmd/yt2column/fixpr_docs_test.go:647-670`）の表に、Phase 6 の `fixprguard append-pr ` の行を加える。Phase 4 の `fixprguard finish ` の行は既にある。
-   本文を出力しない `--jq` の式は、`checkListings`（`fixpr_docs_test.go:400`）で確かめている（`TestRunprDocContract` は同 153 行、`TestHandoffListingPrintsNoBody` は同 189 行）。Phase 6 で訂正の段落の数を取得する式も、同じ関数で確かめる。
-   `postingCommand`（`cmd/yt2column/codex_review_docs_test.go:132`）は `fixprguard (reply|edit-pr)` だけを投稿のコマンドとみなす。`postingSections` の Phase 6 の行（同 59 行）と、ファイルの冒頭のコメント（同 19 行）も `edit-pr` だけを挙げている。どれにも `append-pr` を加える。
-   `postingSections` は `_lib/task-pr-handoff.md` Part B 手順 3 も投稿の節として読む（同 71 行）。`cmd/codexreview/docs_test.go:167` は `_lib/local-cli-review.md` を読む。本タスクはこの 2 つの節にも書き足すので、PR-2 の文書のガードが通ることを確かめる。

**コマンドの定義と規則**

-   書き換える箇所は、設計書 3.6 節の表のとおりである（`runplan.md:63-64`、`mkarch.md:72`、`mkplan2.md:107`、`fixpr.md:456-461`・`465-476`・Phase 4 の `545-600`・Phase 6 の `680-726`・Final report の `727` 以降、`runpr.md:149-162`・`204-217`・`564-573`・`631-655`、`task-pr-handoff.md` Part B 手順 3、`local-cli-review.md` 手順 7）。行番号は `0755d504` で確かめ、設計書の引用と同じであることを確かめた。
-   `runpr.md` 手順 4 の問いは PRE_MERGE の変更を示す（`runpr.md:537`）。PR 番号の記入も PRE_MERGE の変更として、ここに示される。
-   `AGENTS.md:11` は、`/runpr` が記録する `docs(<task-id>): record the review of PR #N` の commit を、承認の前の実装の commit の例外としている。PR 番号の記入だけの commit（件名 `docs(<task-id>): fill PR numbers for PR #<number>`。設計書 3.5 節）は文書を承認しない。そのため、この例外とは関係がなく、`AGENTS.md` は変えない。
-   `_lib/settle-decisions.md` は変えない（設計書 2.2 節）。`mkplan.md:82` と `_context.md:48` も変えない（要件書 2.2）。

**`_checks/`**

-   本タスクのディレクトリに `_checks/` はまだない。タスク 0044 の `docs/tasks/0044_article_job_api/_checks/extension_unchanged.sh` は、`sh` と `git` だけで、変わったパスを調べて終了コードで結果を返す。AC-19 のスクリプトも同じく `sh` と `git` で書き、新しい依存を加えない。

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: `fixprguard` の `finish --body-file` と `append-pr`（設計書 8 章のフェーズ 1）

#### ステップ 1-1: `finish` に本文のファイルを加える

**変更するファイル:** `internal/fixprguard/worktree.go`、`internal/fixprguard/gitstate.go`、`internal/fixprguard/errors.go`、`internal/fixprguard/worktree_flow_test.go`

-   [ ] `FinishRequest` に `BodyFile` を加える（設計書 3.4.1 節の型）。
-   [ ] `Finish` が、作業ツリーの照合より前に本文のファイルを読んで検査するようにする。`readTextFile`・`checkPostText` に加えて、訂正の行の形の検査を新しい関数で行う（受け付ける内容は設計書 3.4.1 節）。誤りがあれば `ErrInvalidInput` を返し、`make` も stage も commit もしない。
-   [ ] 本文があるときは、件名・空行・本文を 1 つの `-m` の引数にし、`--cleanup=verbatim` で commit する（1.4 節「採る方法」）。本文がないときの commit の呼び出しは変えない。
-   [ ] `commitObject` が commit のメッセージも返すようにする。本文があるときだけ、`checkCommit` の確認に、記録されたメッセージが渡した文と一致することを加える（1.4 節）。
-   [ ] `ErrInvalidInput` のコメントに `finish` の本文のファイルを加える。
-   [ ] テストを加える（4 章の 1-1 の行）。

**完了の条件:** 4 章の 1-1 のテストが通り、本文の検査、メッセージの一致の確認、`--cleanup=verbatim` のそれぞれを外すと該当のテストが落ちることを確かめて、コミットメッセージに書いた。

#### ステップ 1-2: `append-pr` を `internal/fixprguard` に加える

**変更するファイル:** `internal/fixprguard/write.go`、`internal/fixprguard/errors.go`、`internal/fixprguard/write_test.go`

-   [ ] `AppendPRRequest` と `(*GitHub).AppendPR` を加える（設計書 3.4.2 節の型）。
-   [ ] `AppendPR` は、ファイルを `readTextFile`・`checkPostText` と、`@`・`<` の拒否で検査してから `gh` を呼ぶ。今の本文を `gh pr view <n> --json body` で取得し、`strictjson` で読む（1.4 節）。今の本文のバイト列を変えずに、その後ろに区切りとファイルの内容を足す。足した後の本文全体を `checkPostText` と `maxTextFileBytes` の上限で検査する。通ったら、`gh pr edit <n> --body-file -` の標準入力で書き込み、`--title` は渡さない。区切りの形（今の本文が空のとき、改行で終わるときの扱いを含む）は、実装のときに決めてテストに書く。
-   [ ] 取得した本文を、戻り値の誤りにも診断にも含めない（`ghFailure` の形にそろえる）。
-   [ ] `ErrInvalidInput` のコメントに `append-pr` を加える。
-   [ ] テストを加える（4 章の 1-2 の行）。

**完了の条件:** 4 章の 1-2 のテストが通り、`@`・`<` の拒否、足した後の検査、`--title` を渡さないことのそれぞれを壊すと該当のテストが落ちることを確かめて、コミットメッセージに書いた。

#### ステップ 1-3: `cmd/fixprguard` の引数の解析と呼び出し

**変更するファイル:** `cmd/fixprguard/main.go`、`cmd/fixprguard/main_test.go`

-   [ ] サブコマンドの名前 `append-pr` の定数を加え、`specs` に `append-pr`（`--body-file` は必須、`[PR]` は省略できる、`usesGH`）を加える。`cmdFinish` の `flags` に `flagBodyFile` を加え、`required` には加えない。
-   [ ] `finish` の処理が `--body-file` の値を `FinishRequest.BodyFile` に渡すようにする。`append-pr` の処理を `editPR` と同じ形で加え、出力は `writeOutput`（設計書 3.4.2 節「出力」）とする。
-   [ ] `stop` と `writeOutput` のコメントに `finish` と `append-pr` を加える（1.4 節）。
-   [ ] テストを加える（4 章の 1-3 の行）。`main_test.go` の表 `lines` に `append-pr` の行を加える。

**完了の条件:** `make test` と `make lint` が通る。

#### ステップ 1-4: 手引きとパッケージの説明

**変更するファイル:** `docs/dev/guides/fixprguard_guide.md`、`docs/dev/conventions/package_reference.md`

-   [ ] 手引き 5 章の `finish` の見出しと説明に `[--body-file FILE]` を加え、受け付ける内容と、ファイルを作業ツリーの外に置くこと（設計書 3.4.1 節）を書く。`edit-pr` の後に `append-pr --body-file FILE [PR]` の節を加える。
-   [ ] 手引き 4 章の終了コード `2` の行に、`finish` の本文のファイルと `append-pr` のファイルの拒否を加える。古いツールが `--body-file`（`finish`）や `append-pr` を知らずに止まったときの診断と、`make install-fixprguard` での再インストールの案内も書く（設計書 3.4.3 節）。診断の文は、PR-1 の前の `main` から作ったツールで実際に 2 つの呼び出しを実行し、その出力から書く。
-   [ ] 手引き 7.1 節の保証の表に、`finish` の本文のファイルと `append-pr` を加える（設計書 5 章の末尾）。
-   [ ] `package_reference.md` の `cmd/fixprguard` の行（サブコマンドの一覧）と、`internal/fixprguard` の行（`NewGitHubWithoutReviewer` で作る `GitHub` が使える呼び出しに `AppendPR` を加える）を直す。

**完了の条件:** 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview/... ./internal/pipeline`）が通る。手引きの記述を、1-1〜1-3 の実装と、古いツールの実際の出力と突き合わせた。

#### ステップ 1-5: AC-19 の差分の確かめのスクリプトを作り、PR-1 のブランチで実行する

**変更するファイル:** `docs/tasks/0053_status_comments_policy/_checks/task_docs_unchanged.sh`（新規）、本書 7.1 節

-   [ ] マージベース（`git merge-base origin/main HEAD`）と head を比べ、`docs/tasks/` 以下で変わったパスが `docs/tasks/0053_status_comments_policy/` の下だけであることを確かめるスクリプトを作る（設計書 7.3 節）。`sh` と `git` だけを使い、終了コードの意味と使い方を冒頭のコメントに書く（タスク 0044 の `extension_unchanged.sh` と同じ形）。
-   [ ] スクリプトが落ちることを確かめる。一時的に `docs/tasks/` 以下のほかのタスクの文書を変えた commit を作って実行し、終了コードが 0 以外になることを見てから、その commit を捨てる。
-   [ ] PR-1 のブランチで、ほかの作業がすべて終わった後に実行し、出力・終了コード・マージベースと head のコミット ID を本書 7.1 節に記録する。

**完了の条件:** スクリプトの終了コードが 0 で、その記録が本書 7.1 節にある。

### PR-1 作成ポイント: fixprguard finish body file and append-pr

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `feat(0053): fixprguard の finish に本文のファイルを、append-pr サブコマンドを加える`

**レビュー観点**: `finish` の本文の行の検査とメッセージの一致の確認 / `append-pr` が今の本文を変えずに足し、本文をツールの外に出さないこと / `--body-file` のない `finish` の振る舞いが変わらないこと / PR 本文に再インストールの案内があること

**実装モデル要件**: frontier-required

**判定理由**: 1-1・1-2 は、GitHub に届く新しい 2 つの経路（コミットメッセージの本文、PR 本文への追記）の守りを加えるセキュリティの関門の変更で、`mkplan.md` 手順 8 の panel mode の条件に当たる。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

PR-1 の本文には、マージの直後に `make install-fixprguard` で再インストールすることを書く（設計書 8 章「移行の注意」）。

### フェーズ 2: 規則（設計書 8 章のフェーズ 2）

#### ステップ 2-1: `requirements_process.md` を新しい規則に改める

**変更するファイル:** `docs/dev/conventions/requirements_process.md`

-   [ ] 「Document Status Format」の状態表の `Comments` の説明を「今の状態の理由（なければ `-`）」に改め、状態ごとの値、書き換えること、`<br>` で区切ること、`draft` の期間に決定の変更が重なったときの変更履歴の扱いを書く（設計書 3.1 節）。
-   [ ] 変更履歴の形（見出し、行の形、セクションの見分け方、置き場所、作る時期、承認者、未記入の印 `#未記入`、連鎖した戻し、移した項目の接頭辞）を書く（設計書 3.2 節）。
-   [ ] 旧規則で書かれた文書の定義と移し方（写しをバイト列として比べてから commit すること）を書く（設計書 3.3 節）。
-   [ ] 訂正の行（`Editorial correction: ` で始まるコミットメッセージの行）と訂正の段落（PR 本文）の形を書く（設計書 3.6.1 節）。
-   [ ] 「Editing an approved document」を、決定の変更・セッション中の再承認・編集上の訂正の記録先が上の節を参照する形に書き換える（設計書 2.3 節の表）。過去の承認を `Comments` に残す記述と、訂正を `Comments` に書く記述を除く。
-   [ ] `withdrawn` の記録（`Comments` に日付と理由、変更履歴に `withdrawn` の行）を書く（AC-16）。

**完了の条件:** 規則の各節が設計書 2.3 節・3.1〜3.3 節・3.6.1 節と食い違わないことを、読んで確かめた。

### フェーズ 3: `runpr.md` と `task-pr-handoff.md`（設計書 8 章のフェーズ 3）

#### ステップ 3-1: `runpr.md` の PRE_MERGE と手順 4

**変更するファイル:** `.claude/commands/runpr.md`

-   [ ] PRE_MERGE の承認の項目（`runpr.md:207-213`）を、設計書 3.5 節「承認」のとおりに改める。「`Comments` を保つ」を除き、規則の移し方の節と変更履歴の節を参照する。
-   [ ] PRE_MERGE に PR 番号の記入の項目を加える（設計書 3.5 節「PR 番号の記入」）。PR 番号の記入だけの commit の件名を定める（設計書 3.5 節「承認せずにマージする選択肢」）。
-   [ ] 手順 4 の選択肢「merge without the approvals」（`runpr.md:569-573`）を、PR 番号を記録してマージする選択肢に改め、手順 4.3（`runpr.md:631-655`）をこの選択肢でも通すようにする（設計書 3.5 節）。
-   [ ] 手順 4 の問いに、`/fixpr` が PR 本文に追記できなかった訂正と、`main` に既にある `#未記入` の行を示すことを加える（設計書 3.5 節「手順 4 の問い」、4.3 節「検出」）。

**完了の条件:** 文書のガードが通る。PRE_MERGE の再実行で記録が二重にならないこと（設計書 3.5 節「二重の記録の防止」）が、改めた記述から読み取れることを確かめた。

#### ステップ 3-2: PR 本文に訂正の段落と決定の変更を入れる

**変更するファイル:** `.claude/commands/_lib/task-pr-handoff.md`、`.claude/commands/runpr.md`

-   [ ] `task-pr-handoff.md` Part B 手順 3 の PR 本文の一覧に、訂正の段落と決定の変更の 2 つを加える（設計書 3.6 節の表の該当の行）。形は規則の節を参照する。
-   [ ] `runpr.md` 手順 1 の単独の変更の PR の本文（`runpr.md:149-162`）にも、同じ 2 つを入れることを加える。

**完了の条件:** 文書のガード（`TestPostingPathsCheckMention` を含む）が通る。

### フェーズ 4: 書き込む側のコマンドの定義（設計書 8 章のフェーズ 4）

#### ステップ 4-1: `fixpr.md` の Phase 3

**変更するファイル:** `.claude/commands/fixpr.md`

-   [ ] Phase 3 の指示の 4（`fixpr.md:456-461`）を、設計書 3.6 節の表の該当の行のとおりに改める（重なった決定の変更と、R3 の連鎖した戻しを含む）。
-   [ ] Phase 3 の応答の形（`fixpr.md:465-476`）に `editorial` を加え、何を挙げるか（承認済みの文書の訂正だけ）を書く（設計書 6.2 節「訂正の一覧」）。
-   [ ] 応答を受けた後の段落に、訂正の一覧をデータとして扱い、項目ごとに確かめて通らないものを落とすことを加える（設計書 5 章「セッションの確認」「書かれた指示に従わない」）。

#### ステップ 4-2: `fixpr.md` の Phase 4

**変更するファイル:** `.claude/commands/fixpr.md`

-   [ ] 残った訂正の項目から訂正の行のファイルを作業ツリーの外に作り、メンションの確認を通して `finish` に `--body-file` で渡す手順を加える。訂正がないときは今の呼び出しのままにする（設計書 6.2 節「Phase 4」）。
-   [ ] `finish` が終了コード `2` で止まったときの扱い（診断の報告、古いツールの案内）を書く（設計書 4.1 節の表）。

#### ステップ 4-3: `fixpr.md` の Phase 6 と最終報告

**変更するファイル:** `.claude/commands/fixpr.md`

-   [ ] Phase 6 のエージェントへの指示に、既にある訂正の段落を残すこと、この回の訂正を書かないことを加える（設計書 6.2 節「Phase 6 の書き直し」）。
-   [ ] 書き直しを送る前に、今の本文の訂正の段落の数を本文を出力しない `--jq` の式で取得し、新しい本文の数と比べる手順を加える。
-   [ ] この回のコミットの訂正の行から訂正の段落のファイルを作り、メンションの確認を通して `fixprguard append-pr` で追記する手順を加える（設計書 6.2 節「Phase 6 の追記」）。本文が古くない場合でも追記する。
-   [ ] 最終報告に、PR 本文に入らなかった訂正とその理由、追記した訂正の件数を加える（設計書 4.2 節、6.2 節「最終報告」）。

**完了の条件（4-1〜4-3）:** 4-4 の文書のガードが通る。

#### ステップ 4-4: 文書のガードを新しい呼び出しに合わせる

**変更するファイル:** `cmd/yt2column/fixpr_docs_test.go`、`cmd/yt2column/codex_review_docs_test.go`、`cmd/fixprguard/docs_test.go`

-   [ ] `TestFixprguardCallSites` の表に、Phase 6 の `fixprguard append-pr ` の行を加える。
-   [ ] Phase 6 の訂正の段落の数を取得する `--jq` の式が本文を出力しないことを、`checkListings` を使うテストで確かめる（設計書 7.1 節）。
-   [ ] `postingCommand` の正規表現に `append-pr` を加え、`postingSections` の Phase 6 の行に `fixprguard append-pr ` を加え、冒頭のコメントも直す。
-   [ ] `TestDocumentedInvocationsParse` の `fixpr.md` の期待するサブコマンドの一覧に `append-pr` を加える。

**完了の条件:** 文書のガードが通る。加えた行と式の確かめのそれぞれを壊すと落ちることを確かめ、コミットメッセージに書いた。

#### ステップ 4-5: `local-cli-review.md`・`runplan.md`・`mkarch.md`・`mkplan2.md`

**変更するファイル:** `.claude/commands/_lib/local-cli-review.md`、`.claude/commands/runplan.md`、`.claude/commands/mkarch.md`、`.claude/commands/mkplan2.md`

-   [ ] `local-cli-review.md` 手順 7 の commit に、修正が承認済みのタスク文書の編集上の訂正であれば訂正の行も書くことを加える（設計書 3.6 節）。
-   [ ] `runplan.md:63`（編集上の訂正）と `runplan.md:64`（決定の変更）を、設計書 3.6 節の表と 3.7 節のとおりに改める。RECORD は「実装の際に決めたこと（YYYY-MM-DD）」の節とし、PR のあるブランチで作業しないことを根拠とともに一文で書く。
-   [ ] `mkarch.md:72` を、規則の「Editing an approved document」を参照する形に改める（設計書 3.6 節）。
-   [ ] `mkplan2.md:107`（手順 8.5）を、設計書 3.6 節の表のとおりに改める。
-   [ ] 遷移を書く 5 つのコマンドの定義（`runpr.md` の PRE_MERGE、`fixpr.md` の Phase 3、`runplan.md`、`mkarch.md`、`mkplan2.md`）が、旧規則で書かれた文書の移し方の節を参照していることを確かめる（設計書 3.3 節の末尾）。

**完了の条件:** 文書のガード（`cmd/codexreview` を含む）が通る。対象のコマンドの定義のどれも、編集上の訂正を `Comments` または変更履歴に書かせていないこと（AC-06）を、`Comments` と `editorial` を `.claude/commands/` で検索して読んで確かめた。

### フェーズ 5: AC-19 の差分の確かめ（設計書 8 章のフェーズ 5）

#### ステップ 5-1: PR-2 のブランチで確かめを実行する

**変更するファイル:** 本書 7.2 節

-   [ ] PR-2 のブランチで、ほかの作業がすべて終わった後に、ステップ 1-5 のスクリプトを実行する。出力・終了コード・マージベースと head のコミット ID を本書 7.2 節に記録する。

**完了の条件:** スクリプトの終了コードが 0 で、その記録が本書 7.2 節にある。

### PR-2 作成ポイント: status comments policy and command definitions

**対象ステップ**: 2-1 / 3-1 / 3-2 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 5-1

**推奨タイトル**: `docs(0053): Comments を今の状態の理由に絞り、訂正を PR 本文へ、遷移を変更履歴へ移す規則とコマンドの定義`

**レビュー観点**: 規則とコマンドの定義が食い違わず、書式が規則の 1 か所だけにあること（AC-20） / PRE_MERGE の PR 番号の記入と、承認しない選択肢の扱い / `/fixpr` のセッションの確認と、PR 本文をセッションに入れないこと / AC-03 の `runplan.md` の読み替え（設計書 3.7 節）

**実装モデル要件**: frontier-required

**判定理由**: 4-1〜4-3 は GitHub に届く文面のセッションの確認（セキュリティの関門）を、2-1・3-1 は旧規則の文書の移し方（移行）を定め、どちらも `mkplan.md` 手順 8 の panel mode の条件に当たる。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: ツール | `finish --body-file` と `append-pr`、手引き、パッケージの説明、AC-19 のスクリプト | PR-1 のマージ。利用者が `make install-fixprguard` で再インストールした |
| M2: 規則とコマンドの定義 | 新しい規則、`runpr.md`・`task-pr-handoff.md`・`fixpr.md`・`local-cli-review.md`・`runplan.md`・`mkarch.md`・`mkplan2.md`、文書のガード | PR-2 のマージ |

PR-2 は M1 の後に始める。PR-2 の `/fixpr` が新しいツールを使うためである（1.3 節）。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | `fixprguard` の `finish --body-file` と `append-pr`、手引き、`package_reference.md`、AC-19 のスクリプトと PR-1 での実行 | frontier-required |
| PR-2 | 2-1 / 3-1 / 3-2 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 5-1 | 規則、PRE_MERGE と PR 本文、書き込む側のコマンドの定義、文書のガード、PR-2 での AC-19 の実行 | frontier-required |

## 4. テスト戦略 (Test Strategy)

### 4.1. 単体テスト

テストの名前は予定の名前である。実装のときに変えたら、本書 7 章を同じコミットで直す。

| ステップ | 置き場所 | 確かめること |
|---|---|---|
| 1-1 | `internal/fixprguard/worktree_flow_test.go` | 本文を渡すと、メッセージが件名・空行・本文のとおりになる（末尾に空白のある本文を含む）。本文がないときは、`TestFinishCommitSubject` がそのまま確かめる。本文のファイルの誤り（メンション、`Editorial correction: ` で始まらない行、`@`、制御文字、空の行、2 つ以上の末尾の LF、`readTextFile` が拒否する 1 例）では、`make` の fake が一度も呼ばれず、HEAD が変わらない。commit-msg の hook がメッセージを変えると止まり、SHA を返さない |
| 1-2 | `internal/fixprguard/write_test.go` | 今の本文の後ろにファイルの内容を足した本文を、標準入力で、`--title` なしで書き込む（今の本文が空の場合を含む）。ファイルの `@`・`<`・メンションでは `gh` を一度も呼ばない。足した後の本文が上限を超える、またはメンションを含むと、`pr edit` を呼ばない。`pr view` の失敗と、`strictjson` が拒否する応答では `ErrResponseRejected` になり、誤りに本文を含まない |
| 1-3 | `cmd/fixprguard/main_test.go` | `finish` が `--body-file` を受け付け、省略もできる。`append-pr` が `--body-file` を必須とし、`[PR]` を省略できる。`append-pr` の行を `lines` に加え、`GH_REPO`・`GH_HOST` の拒否を確かめる。ファイルの誤りは終了コード `2` になる |
| 4-4 | `cmd/yt2column/fixpr_docs_test.go`、`cmd/yt2column/codex_review_docs_test.go`、`cmd/fixprguard/docs_test.go` | 1.4 節「文書のガード」の 4 つの変更（呼び出しの場所、本文を出力しない式、投稿のコマンドの一覧、期待するサブコマンドの一覧） |

訂正の行の接頭辞 `Editorial correction: ` と訂正の段落の書き出し `**編集上の訂正（` は、規則・`fixpr.md`・ツールに書かれる。この値を照合するテストは加えない（設計書 7.1 節「ほかの文書のテスト」と同じ理由）。食い違うと、`/fixpr` が初めて訂正を記録する回に `finish` が終了コード `2` で止まり（安全側の失敗）、最終報告にも現れるためである。

### 4.2. 統合テスト

N/A（設計書 7.2 節）。コマンドの定義を実行した結果は、本タスクの後の PR で分かる（要件書 2.3）。

### 4.3. 後方互換のテスト

-   `--body-file` のない `finish` の振る舞いが変わらないことは、既存の `TestFinish*` のテスト（`internal/fixprguard/worktree_flow_test.go:72-375`）がそのまま通ることで確かめる。
-   既存のサブコマンドの出力の形が変わらないことは、既存の `cmd/fixprguard/main_test.go` のテストがそのまま通ることで確かめる。
-   旧規則で書かれた既存の文書を変えないこと（AC-19）は、`_checks/` のスクリプトで確かめる（7 章）。

### 4.4. セキュリティのテスト

1-1・1-2 の拒否の行（メンション、`@`、`<`、行の形、制御文字）が当たる（設計書 7.4 節）。`/fixpr` のセッションの確認は手順の文言なので、PR のレビューで読んで確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| PR-1 のマージの後に再インストールせず、PR-2 の `/fixpr` が古いツールで止まる | PR-1 の本文に再インストールの案内を書く（PR-1 作成ポイント）。止まっても安全側で、手引き 4 章の終了コード `2` の説明が再インストールを案内する（ステップ 1-4） |
| `-m` と `--cleanup=verbatim` で記録されるメッセージが、渡した文と末尾の改行などで食い違う | 1-1 のテストで、記録されたメッセージを読み出して比べる。食い違えば、メッセージの一致の確認が止める |
| commit-msg の hook や `commit.gpgsign` の設定が、本文のある commit のメッセージや commit オブジェクトの形を変える | メッセージの一致の確認が hook による変更を止める（1-1 のテスト）。署名のヘッダーは空行を含まないので、メッセージの読み出しに影響しない（1.4 節） |
| PR-2 が大きく、規則とコマンドの定義の食い違いを見落とす | PR-2 のレビュー観点に AC-20 を挙げる。書式を規則の 1 か所にだけ書き、コマンドの定義は節を参照する（1.2 節） |
| `append-pr` の取得と書き込みの間に、レビュアーが PR 本文を編集する | 防げない残余のリスクとして受け入れる（設計書 5 章「残余のリスク」） |
| `/runpr` を通らずにマージして `#未記入` が `main` に残る | 次の `/runpr` が手順 4 で示す（ステップ 3-1）。直し方は設計書 4.3 節 |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 3-1 / 3-2 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 5-1）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別は `test`（実行するテスト）、`static`（ガードのテスト・`make` の対象）、`diff`（`_checks/` のスクリプト）、`manual`（PR のレビューで読む）である。AC-01〜AC-18 と AC-20 は規則とコマンドの定義の文言を定めるので、`manual` で確かめる（設計書 7.5 節）。そのうち、`fixprguard` の振る舞いや信頼の境界のコマンドを含む AC には、`test` と `static` も挙げる。

| AC | 種別 | 確かめ | ステップ |
|---|---|---|---|
| AC-01 | manual・test | PR-2 のレビューで規則の「Editing an approved document」と訂正の行の節を読む。`/fixpr` のコミットメッセージの部分は `internal/fixprguard/worktree_flow_test.go::TestFinishCommitBody`、`TestFinishRejectsBodyFile`、`TestFinishDetectsRewrittenMessage` | 1-1・2-1・4-2 |
| AC-02 | manual | PR-2 のレビューで `task-pr-handoff.md` Part B 手順 3 と `runpr.md` 手順 1 を読む | 3-2 |
| AC-03 | manual・test・static | PR-2 のレビューで `fixpr.md` Phase 6 と `runplan.md:63` の書き換え（設計書 3.7 節の読み替え）を読む。`internal/fixprguard/write_test.go::TestAppendPR`、`TestAppendPRRejects`、`cmd/fixprguard/main_test.go::TestParseAppendPR`、`cmd/yt2column/fixpr_docs_test.go::TestFixprguardCallSites`、`cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`、`cmd/yt2column/codex_review_docs_test.go::TestPostingCommandsInventory`・`TestPostingPathsCheckMention` | 1-2・1-3・4-3・4-4・4-5 |
| AC-04 | manual・static | PR-2 のレビューで `fixpr.md` Phase 6 の書き直しの指示と段落の数の確認を読む。段落の数を取得する式が本文を出力しないことは `cmd/yt2column/fixpr_docs_test.go::TestFixprPhase6CountPrintsNoBody` | 4-3・4-4 |
| AC-05 | manual | PR-2 のレビューで `fixpr.md` の最終報告と `runpr.md` 手順 4 の問いを読む | 3-1・4-3 |
| AC-06 | manual | PR-2 のレビューで対象のコマンドの定義を読む（ステップ 4-5 の完了の条件の検索の結果を PR の本文に書く） | 3-1・4-1・4-5 |
| AC-07 | manual | PR-2 のレビューで規則の「Document Status Format」を読む | 2-1 |
| AC-08 | manual | 同上 | 2-1 |
| AC-09 | manual | 同上 | 2-1 |
| AC-10 | manual | PR-2 のレビューで規則の変更履歴の節を読む | 2-1 |
| AC-11 | manual | 同上 | 2-1 |
| AC-12 | manual | PR-2 のレビューで規則の未記入の印の記述と、`runpr.md` の PRE_MERGE・手順 4・手順 4.3、`runplan.md`・`mkplan2.md`・`mkarch.md` の記述を読む | 2-1・3-1・4-5 |
| AC-13 | manual | PR-2 のレビューで `runpr.md` の PRE_MERGE を読む | 3-1 |
| AC-14 | manual | PR-2 のレビューで規則と `fixpr.md` Phase 3・`mkarch.md`・`mkplan2.md` を読む | 2-1・4-1・4-5 |
| AC-15 | manual | PR-2 のレビューで規則と `runplan.md:64` を読む | 2-1・4-5 |
| AC-16 | manual | PR-2 のレビューで規則の `withdrawn` の記述を読む | 2-1 |
| AC-17 | manual | PR-2 のレビューで規則の旧規則の文書の定義を読む | 2-1 |
| AC-18 | manual | PR-2 のレビューで規則の移し方の節を読む | 2-1 |
| AC-19 | diff | `docs/tasks/0053_status_comments_policy/_checks/task_docs_unchanged.sh`（PR-1 は 7.1 節、PR-2 は 7.2 節に実行の記録） | 1-5・5-1 |
| AC-20 | manual | PR-2 のレビューで規則と対象のコマンドの定義を読み比べる | 2-1・3-1・3-2・4-1〜4-5 |

### 7.1. PR-1 の差分の確かめの実行の記録

（ステップ 1-5 で記録する。）

### 7.2. PR-2 の差分の確かめの実行の記録

（ステップ 5-1 で記録する。）

## 8. 成功基準 (Success Criteria)

-   **機能の完成度:** 6 章の 2 つの PR がマージされ、7 章のすべての行の確かめが済んでいる。
-   **品質:** 各 PR でグリーンゲート（`make test && make lint`）と `make deadcode` が通る。1-1・1-2・4-4 の各テストについて、確かめる対象を壊すと落ちることを確かめ、コミットメッセージに書いた。
-   **セキュリティ:** 4.4 節の拒否のテストが通る。`append-pr` の出力と誤りに PR 本文が含まれない（1-2 のテスト）。
-   **文書:** 手引き・`package_reference.md`・規則・対象のコマンドの定義が、実装と設計書に合っている（PR のレビューで読む）。

## 9. 次のステップ (Next Steps)

-   PR-1 のマージの直後に、利用者が `make install-fixprguard` で再インストールする。
-   本タスクのマージの後に、新しい規則に従った最初の PR（編集上の訂正だけの PR、決定の変更の PR）で、`Comments`・変更履歴・PR 本文の記録が規則どおりになるかを見る（要件書 2.3）。
-   既存の文書の一括の移し替えと、`/mergepr` での `#未記入` の確認は、必要になったら別のタスクにする（設計書 9 章）。
