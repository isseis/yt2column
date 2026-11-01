# 実装計画書：タスク文書の Comments を今の状態の理由に絞り、編集上の訂正を PR 本文へ、状態の遷移を変更履歴へ移す

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-01 |
| Review date | 2026-11-01 |
| Reviewer | isseis |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番、H-NN は [design_handoff.md](design_handoff.md) の項目、「設計書 3.4.1 節」などは設計書の節を指す。「規則」「対象のコマンドの定義」「状態の遷移」「変更履歴」は、設計書の冒頭の定義と同じ意味で使う。本タスクに `implementation_handoff.md` はない。

既存のコードについての記述は、コミット `0755d504`（2026-11-01 の `main`）で確かめた。設計書が行番号を引用したコミット `8bcfd272` から `0755d504` までに、本タスクが変えるファイルは変わっていない。`git diff --stat 8bcfd272 0755d504 -- .claude docs/dev cmd/fixprguard internal/fixprguard cmd/yt2column` が示したのは `cmd/yt2column/extension_docs_test.go` の 1 ファイルだけで、このファイルは本タスクでは変えない。そのため、設計書の `file:line` はそのまま使える。本書の行番号も `0755d504` のものである。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

規則（`docs/dev/conventions/requirements_process.md`）を次の形に改める。`Comments` には今の状態の理由だけを書く。状態の遷移は変更履歴に、編集上の訂正はコミットメッセージと PR 本文に書く。あわせて、規則に従って書き込むコマンドの定義を改め、`/fixpr` が訂正を記録するための `fixprguard` の 2 つの変更（`finish --body-file`・`append-pr`）を実装する。何をどこに書くかは設計書 2〜6 章で決まっている。本書は、それをどの PR・どのステップ・どの確かめで行うかを定める。

### 1.2. 実装の原則

-   設計書 8 章のフェーズの順（1 → 5）で実装する。本書のフェーズ 1〜5 は、設計書 8 章のフェーズ 1〜5 に対応する。ただし、フェーズ 5 で実行する AC-19 のスクリプトは、フェーズ 1 のステップ 1-5 で作る（1.3 節）。PR の分け方は 3.2 節のとおりで、PR-1 のマージの後に PR-2 を始める。
-   書式は規則の 1 か所だけに書き、コマンドの定義は規則の節を名指しして参照する（設計書 1.1 節、AC-20）。`Comments` の値、変更履歴の行の形、訂正の行と訂正の段落の形を、コマンドの定義に書き写さない。
-   規則とコマンドの定義の文言は、PR のレビューで読んで確かめる（`manual`）。文言を照合する恒久のテストは加えない（設計書 7.1 節「ほかの文書のテスト」、`CLAUDE.md`「Document tests check values, structure, and executable examples, never prose」）。テストを加えるのは、`fixprguard` の振る舞いと、信頼の境界のコマンド（`fixprguard` の呼び出しの場所、メンションの確認、本文を出力しない `--jq` の式）だけである。
-   テストを書いたら、確かめる対象を壊して失敗することを確かめ、そのことをコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。壊す対象は、4.1 節の表の「壊して確かめる」の列に挙げる。
-   Go のコメント・識別子・文字列リテラルは英語で書く。規則とコマンドの定義の本文も英語で書く。変更履歴の見出しと行の例、訂正の段落の例は、タスク文書に書く日本語の値をそのままコードブロックで示す（設計書 3.2 節）。

### 1.3. 計画の際に決めたこと（2026-11-01）

-   **PR を 2 つに分ける:** PR-1 はフェーズ 1（`fixprguard` とその手引き）、PR-2 はフェーズ 2〜5（規則、コマンドの定義、文書のガード、AC-19 の確かめ）とする。1 つの PR にまとめると、その PR を `/runpr` で進める間、`/fixpr` はブランチの新しい `fixpr.md` に従って `finish --body-file` や `append-pr` を呼ぶ。一方、インストール済みのツールは `main` から作った古いものである（エージェントはツールをインストールしない。`CLAUDE.md`「Tool Execution Safety」）。そのため、訂正のある回は終了コード `2` で止まる。分ければ、PR-1 のマージの後に利用者が `make install-fixprguard` で再インストールし、PR-2 を新しいツールで進められる。フェーズ 2〜4 を 1 つの PR にまとめることは、設計書 8 章が決めている。
-   **AC-19 の確かめのスクリプトはフェーズ 1 で作る:** 設計書 7.3 節は、本タスクの実装の PR ごとに、そのブランチでマージベースと head を比べるスクリプトを一度ずつ実行するとしている。設計書 8 章の順序ではスクリプトはフェーズ 5 で作るので、そのままでは PR-1 の時点でスクリプトがない。そこで、フェーズ 1 の最後のステップ（1-5）でスクリプトを作って PR-1 のブランチで実行し、フェーズ 5（ステップ 5-1）では PR-2 のブランチで実行し直す。設計書 7.3 節の「各 PR のブランチで実行する」をそのまま守れ、設計書を直さずに済む。

### 1.4. 既存コード調査結果

**`internal/fixprguard` の `finish`**

-   **commit の作り方:** `Finish`（`internal/fixprguard/worktree.go:219`）は、`w.git(ctx, "commit", "commit", "-q", "-m", CommitSubject(req.PR))`（`worktree.go:272`）で件名だけの commit を作る。`FinishRequest`（`worktree.go:198-200`）は `PR` だけを持つ。git は `cmdrunner.Runner` 越しに、標準入力なしで起動される（`w.git`、`internal/fixprguard/gitstate.go:426`）。
    -   **採る方法:** 本文を渡したときは、ツールが件名・空行・本文を 1 つの文字列に組み立て、`-m` の 1 つの引数として `--cleanup=verbatim` とともに渡す。git にファイルを読み直させない、という設計書 3.4.1 節の求めを、標準入力を足さずに満たせる。組み立てた文字列は、本文が LF で終わるかどうかによらず、ちょうど 1 つの LF で終わるようにする。`git commit -m` は、最後の行が LF で終わらないときに LF を 1 つ足す。そのため、ツールが LF で終える形に組み立てておけば、記録されるメッセージは組み立てた文字列と同じになる。これはツールが作るメッセージの形であり、入力の修正ではない。`--cleanup=verbatim` が末尾の空白を残すことと、`git commit -m` が LF を 1 つ足すことは、レビューの際に git 2.50.1 で確かめた。
    -   本文の上限は 64 KiB（`maxTextFileBytes`、`internal/fixprguard/write.go:18`）なので、1 つの引数の長さは Linux の 1 引数の上限（128 KiB）に収まる。本文を渡さないときの呼び出しは変えない（設計書 3.4.3 節の条件 2）。
-   **commit の後の確認:** `checkCommit`（`worktree.go:395-436`）は HEAD・親・tree を確かめるが、メッセージは確かめない。`commitObject`（`gitstate.go:637-662`）は commit オブジェクトを `cat-file commit` で読み、最初の `\n\n` より前のヘッダーから `tree` と `parent` だけを取り出す。呼び出し元は `worktree.go:160`・`403`・`410`・`519` の 4 か所である。
    -   **採る方法:** `commitObject` がメッセージ（最初の `\n\n` の後）も返すように戻り値を増やし、4 か所の呼び出し元を合わせる。署名の `gpgsig` ヘッダーは、続きの行を空白で始めるので空行を含まない。この前提はステップ 1-1 のテストで確かめる。
    -   メッセージの一致は、本文を渡したときだけ確かめる。本文を渡さない `finish` にこの確認を加えると、commit-msg の hook が trailer を足す環境では、今まで通っていた呼び出しが止まるようになる。これは設計書 3.4.3 節の条件 2（新しいツールは古い呼び出しを今までと同じ規則で扱う）に反する。
    -   一致しないときは、`checkCommit` が親や branch の食い違いに返すものと同じ `&GitStateError{Parts: []string{PartHead}}` を返す。HEAD の commit が `finish` の作るべき commit でない点で、親の食い違いと同じ扱いになるためである。新しい番兵エラーも新しい `Part*` も加えない（設計書 4.1 節）。このとき commit はローカルに残る。これは今の `checkCommit` の失敗と同じで、`fixpr.md` は件名でこの場合を扱っている（`fixpr.md:572-574`）。手引き 8 章の `git-state-changed` の行の原因か対処の欄に、hook がメッセージを変えた場合を加える（ステップ 1-4）。`TestGuideNamesGitStateParts`（`cmd/fixprguard/docs_test.go:454`）が読む `parts（…）` の中は変えない。
-   **本文のファイルの検査:** `readTextFile`（`write.go:28-49`）と `checkPostText`（`write.go:73-84`）をそのまま使う。訂正の行の形の検査は、新しい関数として `write.go` に、この 2 つと並べて置く（設計書 2.2 節）。`ErrInvalidInput` の説明のコメント（`internal/fixprguard/errors.go:12-16`）は `reply` と `edit-pr` だけを挙げているので、`finish` と `append-pr` も加える。
-   **ファイルの置き場所:** 本文のファイルを作業ツリーの中に置くと、`finish` の照合は、そのファイルを判定の後に現れた変更とみなして止まる（設計書 3.4.1 節）。手引きにこの振る舞いを書く前に、テストで確かめる（ステップ 1-1）。

**`internal/fixprguard` の `append-pr`**

-   `EditPR`（`write.go:297-319`）は、本文を `gh pr edit <n> --title=... --body-file -` の標準入力で渡す。`append-pr` の書き込みも同じ形にし、`--title` は渡さない。
-   今の本文は `gh pr view <n> --json body` で取得し、`Head` が `gh pr view <n> --json <fields>` の応答を読む方法（`internal/fixprguard/github.go:149-178`。`maxResponseBytes` の上限を含む）と同じく、`strictjson` で検査して読む。ただし、`strictjson.RequiredString`（`internal/strictjson/strictjson.go:144-146`。`resolveRepository` が使う。`github.go:111`）は空の文字列を拒否する。空の PR 本文を受け付けるため、`Required` と `AsString` で読む。`gh` の失敗と応答の拒否は `ErrResponseRejected`（`ghFailure`、`github.go:79`）とし、本文も引数も含めない。
-   足した後の本文が上限を超える場合と、メンションを含む場合（今の本文が既にメンションを含む場合を含む）は、`ErrInvalidInput` を返す。これは設計書 4.1 節の終了コード `2` に当たる（`cmd/fixprguard/main.go:291-295` は `ErrInvalidInput` を終了コード `2` にする）。
-   区切りは、設計書 3.4.2 節のとおり空行とする。今の本文が空のとき、LF で終わるとき、CRLF の改行を含むとき（GitHub の画面で編集した本文）の扱いは、実装のときに決めてテストに書く。どの場合も、今の本文のバイト列は変えない。

**`cmd/fixprguard`**

-   `specs`（`cmd/fixprguard/main.go:104-119`）の `cmdFinish` は `--baseline`・`--judged-set` だけを受け付ける。`--body-file` の名前（`flagBodyFile`）は `reply`・`edit-pr` が既に使っている。`append-pr` は `editPR`（`main.go:666-682`）と同じ形（`NewGitHubWithoutReviewer`、`prNumber`、`writeOutput`）で加える。
-   古いツールの診断は次の 2 つで、どちらも終了コード `2` になる。
    -   `finish --body-file` は、`flag.Parse` の誤り（`main.go:226-228`）になり、診断が知らないフラグ `-body-file` を示す。
    -   `append-pr` は、`specs` にないサブコマンドとして、メッセージを持たない `errUsage` になる（`main.go:215-218`）。診断は `usage` とガイドへの案内だけで、サブコマンドの名前を示さない。
-   この 2 つの診断について、設計書 4.1 節は、「診断が知らないフラグかサブコマンドを示すときだけ」再インストールを案内するとしている。`append-pr` の診断はサブコマンドを示さないので、`/fixpr` は `append-pr` の失敗では再インストールを案内しない。追記しなかった訂正として最終報告に挙げる（設計書 4.1 節の表の 2 行目）。手引き 4 章の終了コード `2` の説明には、両方の診断の文と、`go version -m` の `vcs.revision` でインストール済みのツールの作られたコミットを確かめる方法（手引き 3 章）を書く。なお、訂正のある回は Phase 4 の `finish --body-file` が先に止まるので、古いツールで `append-pr` まで進むのは、Phase 4.5 のコミットだけに訂正の行がある回に限られる。
-   テストの表のうち、新しいサブコマンドに行が要るものは 2 つある。`main_test.go:222-230` の表 `lines` は、`usesGH` のサブコマンドごとに 1 行を持つことを確かめる（同 231-235）。`validArgs`（`main_test.go:27-44`）は `TestProtocolMismatch`（同 47）と `TestReviewerFlag`（同 71）が使う。メンションの拒否の表 `TestPostTextMentionExitsUsage`（同 717-781）には、新しい経路の行を加える。止まった種類の表 `TestStopKinds`（同 570）は、止まった種類ごとに 1 つの場面を持ち、メッセージの食い違いは既にある `git-state-changed` なので、行を加えない。ファイルを使う実行のテストには、`newTestRepo`（`cmd/fixprguard/test_helpers.go:50`）と `newTestRun`（同 191）を使う。
-   `TestDocumentedInvocationsParse`（`cmd/fixprguard/docs_test.go:184-213`）の期待するサブコマンドの一覧（同 189 行）に `append-pr` を加えるのは PR-2 である。`fixpr.md` が `append-pr` を呼ぶのは PR-2 からで、PR-1 で一覧に加えると、この時点の `fixpr.md` に呼び出しがないためテストが落ちる。手引きの見出しを読む `TestGuideDescribesReviewerList`（同 285-307）は、`append-pr` が `--reviewer` を取らないので影響を受けない。
-   パッケージのコメント（`main.go:1-11`）はサブコマンドの名前を挙げていないので変えない。`stop` のコメント（`main.go:287-290`）と `writeOutput` のコメント（`main.go:627`）は `reply` と `edit-pr` だけを挙げているので、`finish` と `append-pr` も加える。

**文書のガード（`cmd/yt2column`）**

-   `TestFixprguardCallSites`（`cmd/yt2column/fixpr_docs_test.go:647-670`）の表に、Phase 6 の `fixprguard append-pr ` の行を加える。Phase 4 の `fixprguard finish ` の行は既にある。
-   **本文を出力しない `--jq` の式:** 既存の `checkListings`（`fixpr_docs_test.go:400-455`）は、`gh api --paginate repos/<owner>/<repo>/... --jq '...'` の一覧（同 211 行の正規表現）だけを対象にし、出力を `kind` などを持つ JSON のオブジェクトの列として読む。Phase 6 で訂正の段落の数を取得する式は `gh pr view <n> --json body --jq '...'` で、数を 1 つ出力するので、`checkListings` では確かめられない。そこで、`runJQ` と `bodyMarker` を使う新しいテストを加える（ステップ 4-4）。Phase 6 のエージェントへの指示には `gh pr view NUMBER --json title,body` が既にある（`fixpr.md:687`）。新しい確かめは、このセッションが実行する数の取得のコマンドだけを取り出す。
-   **投稿のコマンドとメンションの確認:** `postingCommand`（`cmd/yt2column/codex_review_docs_test.go:132`）は `fixprguard (reply|edit-pr)` だけを投稿のコマンドとみなす。`postingSections`（同 57-75）の Phase 6 の行（同 59）とファイルの冒頭のコメント（同 16-19）も同じである。`TestPostingCommandsInventory`（同 196-251）は見つけた投稿が一覧にあることを、`TestIsPost`（同 257）は分類の関数を確かめるが、正規表現が一覧のコマンドに当たることはどちらも確かめない。
    -   **採る方法:** `append-pr` を投稿のコマンドに加える。あわせて、`finish --body-file` も投稿のコマンドに加え、`fixpr.md` Phase 4 を投稿の節に加える。`finish` の本文は push で GitHub に届くので、設計書 5 章の 2 段の守り（セッションの確認が主、ツールの拒否が裏）が適用され、`CLAUDE.md` が恒久のテストを残すとする「メンションの確認」に該当するためである。設計書 7.1 節はこの行を挙げていないが、テストの行を加えるだけで、設計の決定は変えない。正規表現は `fixprguard (reply|edit-pr|append-pr|finish)\b` とする。`textFlag`（同 135 行）は `--protocol` を含むので、`--body-file` のない `finish` の呼び出し（`fixpr.md:556`）も投稿とみなされる。そのため、Phase 4 の `**Mention check.**` の段落（`TestPostingPathsCheckMention` が求める見出しの形）を、`finish` の呼び出しより前に置く。訂正がない回は、確かめるファイルがないので確認を飛ばすことを、その段落に書く。`postingSections` の冒頭のコメント（同 55 行）の節の数も 7 から 8 に直す。
    -   `postingSections` の一覧のコマンドが、`postingCommand` に当たることを確かめる行を加える（`TestIsPost` の表に、`append-pr`、`--body-file` のない `finish`、`--body-file` のある `finish` の行を加える）。
-   `postingSections` は `_lib/task-pr-handoff.md` Part B 手順 3 も投稿の節として読む（同 71 行）。`cmd/codexreview/docs_test.go:167` は `_lib/local-cli-review.md` を読む。本タスクはこの 2 つの節にも書き足すので、PR-2 で文書のガードが通ることを確かめる。

**コマンドの定義と規則**

-   書き換える箇所は、設計書 3.6 節の表のとおりである。`runplan.md:63-64`、`mkarch.md:72`、`mkplan2.md:107`、`fixpr.md` の Phase 3 の指示の 4（`454-462`）と応答の形（`464-477`）、Phase 4（`545-600`）、Phase 6（`680-726`）、Final report（`727` 以降）、`runpr.md:149-162`・`204-217`・`564-573`・`631-655`、`task-pr-handoff.md` Part B 手順 3 が該当する。
-   設計書 3.6 節が `local-cli-review.md`「手順 7 の commit」と書いている commit は、手順 5 で作られる（`.claude/commands/_lib/local-cli-review.md:203-207`）。手順 7 は commit メッセージの記録の行を定める（同 221-256）。訂正の行も commit メッセージの行なので、手順 7 の記録の行の一覧に加える。手順 7 のメンションの確認（同 237-245）は `Codex local review:` の行を含むメッセージだけを対象にするので、訂正の行を含むメッセージも対象に加える。設計書 5 章の、GitHub に届く文面をセッションが確かめる方針を、`finish` を通らない commit にも当てはめるためである。
-   `fixpr.md` のうち、セッションの作業を挙げるほかの箇所も合わせて直す。「The guard tool」の、ツールに渡すファイルの置き場所の項目（`fixpr.md:80-84`）に、訂正の行のファイルと訂正の段落のファイルを加える。「Authorization」（`fixpr.md:90-98`）には、このセッションが行う `gh pr view --json body --jq` の数の取得を加える。Phase 6 の見出し `(model: haiku)` は変えない。見出しは文書のガードの `fixprPhase6`（`fixpr_docs_test.go:38`）が読んでいる。Phase 6 の `edit-pr` の呼び出しは今もこのセッションが行っており、新しい数の取得と `append-pr` もこのセッションが行うので、見出しの示すエージェントの役割は変わらない。
-   `runpr.md` 手順 4 の問いは PRE_MERGE の変更を示す（`runpr.md:537`）。PR 番号の記入も PRE_MERGE の変更なので、この問いに含まれる。
-   `AGENTS.md:11` は、`/runpr` が記録する `docs(<task-id>): record the review of PR #N` の commit を、承認の前の実装の commit の例外としている。PR 番号の記入だけの commit（件名 `docs(<task-id>): fill PR numbers for PR #<number>`。設計書 3.5 節）は文書を承認しないので、この例外とは関係がない。`AGENTS.md` は変えない。
-   `_lib/settle-decisions.md` は変えない（設計書 2.2 節）。`mkplan.md:82` と `_context.md:48` も変えない（要件書 2.2）。

**`_checks/`**

-   本タスクのディレクトリに `_checks/` はまだない。タスク 0044 の `docs/tasks/0044_article_job_api/_checks/extension_unchanged.sh` は、`sh` と `git` だけを使い、`:(top)` の pathspec で変わったパスを調べ、結果を終了コードで返す。AC-19 のスクリプトも同じく `sh` と `git` で書き、新しい依存を加えない。

## 2. 実装手順 (Implementation Steps)

テストの名前と、壊して確かめる対象は 4.1 節の表にある。各ステップのテストの作業は、その表の行を指す。

### フェーズ 1: `fixprguard` の `finish --body-file` と `append-pr`（設計書 8 章のフェーズ 1）

#### ステップ 1-1: `finish` に本文のファイルを加える

**変更するファイル:** `internal/fixprguard/worktree.go`、`internal/fixprguard/gitstate.go`、`internal/fixprguard/write.go`、`internal/fixprguard/errors.go`、`internal/fixprguard/worktree_flow_test.go`、`internal/fixprguard/write_test.go`、`internal/fixprguard/gitstate_test.go`

-   [x] `FinishRequest` に `BodyFile` を加える（設計書 3.4.1 節の型）。
-   [x] 訂正の行の形の検査を、`write.go` に新しい関数として加える（受け付ける内容は設計書 3.4.1 節）。
-   [x] `Finish` が、作業ツリーの照合より前に本文のファイルを読んで検査するようにする（`readTextFile`・`checkPostText`・上の関数）。誤りがあれば `ErrInvalidInput` を返し、`make` も stage も commit もしない。
-   [x] 本文があるときは、1.4 節の「採る方法」の形でメッセージを組み立てて commit する。本文がないときの commit の呼び出しは変えない。
-   [x] `commitObject` がメッセージも返すようにし、4 か所の呼び出し元を合わせる。この変更は振る舞いを変えない準備として、`--body-file` の振る舞いとは別のコミットにする（`Push` と `changesAgainst` の呼び出し元を、振る舞いが変わらないことだけを見て読めるようにするため）。本文があるときだけ、`checkCommit` で、記録されたメッセージが組み立てた文字列と一致することを確かめ、一致しなければ `PartHead` の `GitStateError` を返す（1.4 節）。
-   [x] `ErrInvalidInput` のコメントに `finish` の本文のファイルを加える。
-   [x] 4.1 節の 1-1 の行のテストを加え、`TestFinishCommitSubject` が件名だけのメッセージ全体を確かめるようにする。

**完了の条件:** 4.1 節の 1-1 の行のテストが通る。同じ行の「壊して確かめる」の各項目で該当のテストが落ちることを確かめ、コミットメッセージに書いた。

#### ステップ 1-2: `append-pr` を `internal/fixprguard` に加える

**変更するファイル:** `internal/fixprguard/write.go`、`internal/fixprguard/errors.go`、`internal/fixprguard/write_test.go`

-   [x] `AppendPRRequest` と `(*GitHub).AppendPR` を加える（設計書 3.4.2 節の型）。
-   [x] `AppendPR` は、`gh` を呼ぶ前に、ファイルを `readTextFile`・`checkPostText` と `@`・`<` の拒否で検査する。今の本文の取得と読み方、空行での区切り、足した後の検査、書き込みは 1.4 節のとおりにする。足した後の検査に通らなければ `ErrInvalidInput` を返し、`pr edit` を呼ばない。
-   [x] 取得した本文を、戻り値の誤りにも診断にも含めない。
-   [x] `ErrInvalidInput` のコメントに `append-pr` を加える。
-   [x] 4.1 節の 1-2 の行のテストを加える。

**完了の条件:** 4.1 節の 1-2 の行のテストが通る。同じ行の「壊して確かめる」の各項目で該当のテストが落ちることを確かめ、コミットメッセージに書いた。

#### ステップ 1-3: `cmd/fixprguard` の引数の解析と呼び出し

**変更するファイル:** `cmd/fixprguard/main.go`、`cmd/fixprguard/main_test.go`

-   [x] サブコマンドの名前 `append-pr` の定数を加え、`specs` に `append-pr` を加える（`--body-file` は必須、`[PR]` は省略できる、`usesGH`）。`cmdFinish` の `flags` に `flagBodyFile` を加え、`required` には加えない。
-   [x] `finish` の処理が、`--body-file` の値を `FinishRequest.BodyFile` に渡すようにする。`append-pr` の処理を `editPR` と同じ形で加え、出力は `writeOutput`（設計書 3.4.2 節「出力」）とする。
-   [x] `stop` と `writeOutput` のコメントに `finish` と `append-pr` を加える（1.4 節）。
-   [x] 4.1 節の 1-3 の行のテストを加える。`lines`・`validArgs` に `append-pr` の行を、`TestPostTextMentionExitsUsage` に新しい経路の行を加える。

**完了の条件:** `make test` と `make lint` が通る。4.1 節の 1-3 の行の「壊して確かめる」の各項目で該当のテストが落ちることを確かめ、コミットメッセージに書いた。

#### ステップ 1-4: 手引きとパッケージの説明

**変更するファイル:** `docs/dev/guides/fixprguard_guide.md`、`docs/dev/conventions/package_reference.md`

-   [x] 手引き 5 章の `finish` の見出しと説明に `[--body-file FILE]` を加え、受け付ける内容と、ファイルを作業ツリーの外に置くこと（1-1 のテストで確かめた振る舞い）を書く。`edit-pr` の後に `append-pr --body-file FILE [PR]` の節を加える。
-   [x] 手引き 4 章の終了コード `2` の行に、`finish` の本文のファイルの拒否と、`append-pr` のファイルと足した後の本文の拒否を加える。古いツールの 2 つの診断と再インストールの案内も書く（1.4 節「`cmd/fixprguard`」、設計書 3.4.3 節）。診断の文は、マージベースの `main` のツールを `go run ./cmd/fixprguard` で実行して得る。呼び出しは `fixpr.md` に書く形（`--protocol 4` と必須のフラグと PR 番号をすべて付けた形）にする。マージベースのツリーは、スクラッチパッドに作った `git worktree` に取り出して使う。2 つの標準エラーの行と、実行したコミットを本書 7.1 節に記録する。
-   [x] 手引き 7.1 節の保証の表に、`finish` の本文のファイルと `append-pr` を加える（設計書 5 章の末尾）。手引き 8 章の `git-state-changed` の行に、hook がメッセージを変えた場合を加える（1.4 節）。
-   [x] `package_reference.md` の `cmd/fixprguard` の行（サブコマンドの一覧）と `internal/fixprguard` の行（`NewGitHubWithoutReviewer` で作った `GitHub` が使える呼び出しに、`AppendPR` を加える）を直す。

**完了の条件:** 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview ./internal/pipeline`）が通る。手引きの記述を、1-1〜1-3 の実装とテスト、および古いツールの実際の出力と突き合わせた。

#### ステップ 1-5: AC-19 の差分の確かめのスクリプトを作り、PR-1 のブランチで実行する

**変更するファイル:** `docs/tasks/0053_status_comments_policy/_checks/task_docs_unchanged.sh`（新規）、本書 7.1 節

-   [x] マージベースと head を比べ、`docs/tasks/` 以下で変わったパスが、どれも `docs/tasks/0053_status_comments_policy/` の下にあることを確かめるスクリプトを作る（設計書 7.3 節）。マージベースは `/runplan` が `task-pr-handoff.md` Part A で取得した `origin/main` から求める。名前の変更は、元の場所の削除と新しい場所の追加として数える（`--no-renames`）。パスは `:(top)` を付けて最上位から比べ、本タスクのディレクトリの名前で始まるだけの別のディレクトリを本タスクのものとみなさない。`sh` と `git` だけを使い、終了コードの意味と使い方を冒頭のコメントに書く（タスク 0044 の `extension_unchanged.sh` と同じ形）。
-   [x] スクリプトが期待どおりに失敗することを、スクラッチパッドに作った使い捨ての clone で確かめる（PR のブランチは変えない）。ほかのタスクの文書の変更・削除、ほかのタスクから本タスクのディレクトリへの名前の変更、本タスクの名前で始まる別のディレクトリの変更、引数やリポジトリの誤りのそれぞれで、期待する終了コードになることを見る。各場合の終了コードを本書 7.1 節に記録する。
-   [ ] PR-1 のブランチで、ほかの作業がすべて終わった後に実行する。出力・終了コード・マージベースと、実行した時点の head のコミット ID を本書 7.1 節に記録する。記録を書くコミットが本書だけを変えることも、あわせて記録する。

**完了の条件:** スクリプトの終了コードが 0 で、その記録が本書 7.1 節にある。

### PR-1 作成ポイント: fixprguard finish body file and append-pr

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `feat(0053): fixprguard の finish に本文のファイルの指定を、新しく append-pr サブコマンドを加える`

**レビュー観点**: `finish` の本文の行の検査とメッセージの一致の確認、`--body-file` のない `finish` が変わらないこと / `append-pr` が今の本文を変えずに足し、本文をツールの外に出さないこと / 手引きの古いツールの診断が実際の出力と合うこと / AC-19 のスクリプトが名前の変更と似た名前のディレクトリを見逃さないこと

**実装モデル要件**: frontier-required

**判定理由**: 1-1・1-2 は、GitHub に届く新しい 2 つの経路（コミットメッセージの本文と PR 本文への追記）の守りを加える、セキュリティの関門の変更である。1-1 は `push` も使う `commitObject` と `checkCommit` も変える。これは `mkplan.md` 手順 8 の panel mode の条件に当たる。1-3（`cmd/`）が 1-1・1-2（`internal/`）に依存するので、1-1・1-2 を先にし、1-3 を後にする。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

PR-1 の本文には、マージの直後に `make install-fixprguard` で再インストールすることを書く（設計書 8 章「移行の注意」）。

### フェーズ 2: 規則（設計書 8 章のフェーズ 2）

#### ステップ 2-1: `requirements_process.md` を新しい規則に改める

**変更するファイル:** `docs/dev/conventions/requirements_process.md`

-   [ ] 「Document Status Format」の状態表の `Comments` の説明を「今の状態の理由（なければ `-`）」に改める。状態ごとの値、状態が変わるたびに書き換えること、`<br>` で区切ること、`draft` の期間に決定の変更が重なったときの変更履歴の扱いを書く（設計書 3.1 節）。
-   [ ] 変更履歴の形を書く（設計書 3.2 節）。見出し、行の形、セクションの見分け方、置き場所、作る時期、承認者、未記入の印 `#未記入`、連鎖した戻し、移した項目の接頭辞を含む。
-   [ ] 旧規則で書かれた文書の定義と移し方を書く。写しをバイト列として比べてから commit することを含む（設計書 3.3 節）。
-   [ ] 訂正の行（`Editorial correction: ` で始まるコミットメッセージの行）と、訂正の段落（PR 本文）の形を書く（設計書 3.6.1 節）。
-   [ ] 「Editing an approved document」を書き換え、決定の変更・セッション中の再承認・編集上の訂正の記録先は、上の各節を参照する形にする（設計書 2.3 節の表）。過去の承認を `Comments` に残す記述と、訂正を `Comments` に書く記述を除く。
-   [ ] `withdrawn` の記録（`Comments` に日付と理由、変更履歴に `withdrawn` の行）を書く（AC-16）。

**完了の条件:** 規則の各節が、設計書 2.3 節・3.1〜3.3 節・3.6.1 節と食い違わないことを、読んで確かめた。

### フェーズ 3: `runpr.md` と `task-pr-handoff.md`（設計書 8 章のフェーズ 3）

#### ステップ 3-1: `runpr.md` の PRE_MERGE と手順 4

**変更するファイル:** `.claude/commands/runpr.md`

-   [ ] PRE_MERGE の承認の項目（`runpr.md:207-213`）を、設計書 3.5 節「承認」のとおりに改める。「`Comments` を保つ」を除き、規則の移し方の節と変更履歴の節を参照する。
-   [ ] PRE_MERGE に PR 番号の記入の項目を加える（設計書 3.5 節「PR 番号の記入」）。PR 番号の記入だけの commit の件名を定める（設計書 3.5 節「承認せずにマージする選択肢」）。
-   [ ] 手順 4 の選択肢「merge without the approvals」（`runpr.md:569-573`）を、PR 番号を記録してマージする選択肢に改め、手順 4.3（`runpr.md:631-655`）をこの選択肢でも通すようにする（設計書 3.5 節）。
-   [ ] 手順 4 の問いに、`/fixpr` が PR 本文に追記できなかった訂正と、`main` に既にある `#未記入` の行を示すことを加える（設計書 3.5 節「手順 4 の問い」、4.3 節「検出」）。

**完了の条件:** 文書のガードが通る。PRE_MERGE を再実行しても記録が二重にならないこと（設計書 3.5 節「二重の記録の防止」）を、改めた記述を読んで確かめた。

#### ステップ 3-2: PR 本文に訂正の段落と決定の変更を入れる

**変更するファイル:** `.claude/commands/_lib/task-pr-handoff.md`、`.claude/commands/runpr.md`

-   [ ] `task-pr-handoff.md` Part B 手順 3 の PR 本文の一覧に、訂正の段落と決定の変更の 2 つを加える（設計書 3.6 節の表の該当の行）。形は規則の節を参照する。
-   [ ] `runpr.md` 手順 1 の単独の変更の PR の本文（`runpr.md:149-162`）にも、同じ 2 つを入れることを加える。

**完了の条件:** 文書のガード（`TestPostingPathsCheckMention` を含む）が通る。

### フェーズ 4: 書き込む側のコマンドの定義（設計書 8 章のフェーズ 4）

#### ステップ 4-1: `fixpr.md` の Phase 3

**変更するファイル:** `.claude/commands/fixpr.md`

-   [ ] Phase 3 の指示の 4（`fixpr.md:454-462`）を、設計書 3.6 節の表の該当の行のとおりに改める（決定の変更が重なる場合と、連鎖した戻しを含む。連鎖した戻しは、`fixpr.md` の R3 が申し送りの項目の変更に伴って後の文書を戻すことである）。
-   [ ] Phase 3 の応答の形（`fixpr.md:464-477`）に `editorial` を加え、承認済みの文書の訂正だけを挙げることを書く（設計書 6.2 節「訂正の一覧」）。
-   [ ] 応答を受けた後の段落に、次のことを加える。訂正の一覧をデータとして扱う。項目ごとに確かめ、通らないものは一覧から落とす（設計書 5 章「セッションの確認」「書かれた指示に従わない」）。

#### ステップ 4-2: `fixpr.md` の Phase 4

**変更するファイル:** `.claude/commands/fixpr.md`

-   [ ] 残った訂正の項目から、訂正の行のファイルを作業ツリーの外に作る手順を加える。ファイルは、`finish` の呼び出しより前に置いた `**Mention check.**` の段落（1.4 節「投稿のコマンドとメンションの確認」）の確認を通してから、`finish` に `--body-file` で渡す。訂正がないときは、今の呼び出しのままにする（設計書 6.2 節「Phase 4」）。
-   [ ] `finish` が終了コード `2` で止まったときの扱いを書く。診断を報告し、診断が知らないフラグ `-body-file` を示すときは再インストールを案内する（設計書 4.1 節の表、本書 1.4 節「`cmd/fixprguard`」）。
-   [ ] 「The guard tool」のファイルの置き場所の項目（`fixpr.md:80-84`）に、訂正の行のファイルと訂正の段落のファイルを加える。

#### ステップ 4-3: `fixpr.md` の Phase 6 と最終報告

**変更するファイル:** `.claude/commands/fixpr.md`

-   [ ] Phase 6 のエージェントへの指示に、2 つのことを加える。既にある訂正の段落を残すこと、この回の訂正を書かないこと（設計書 6.2 節「Phase 6 の書き直し」）。
-   [ ] 書き直しを送る前に、今の本文の訂正の段落の数を、本文を出力しない `--jq` の式で取得し、新しい本文の数と比べる手順を加える。
-   [ ] 追記の手順を加える。この回のコミットの訂正の行から訂正の段落のファイルを作り、メンションの確認を通して `fixprguard append-pr` で追記する（設計書 6.2 節「Phase 6 の追記」）。本文が古くない場合でも追記する。
-   [ ] `append-pr` が終了コード `2` で止まったときは、診断を報告し、訂正を追記できなかったものとして最終報告に挙げる（設計書 4.1 節の表の 2 行目、AC-05）。
-   [ ] 「Authorization」（`fixpr.md:90-98`）に、このセッションが行う数の取得を加える。
-   [ ] 最終報告に、PR 本文に入らなかった訂正とその理由、追記した訂正の件数を加える（設計書 4.2 節、6.2 節「最終報告」）。

**完了の条件（4-1〜4-3）:** 4-4 の文書のガードが通る。

#### ステップ 4-4: 文書のガードを新しい呼び出しに合わせる

**変更するファイル:** `cmd/yt2column/fixpr_docs_test.go`、`cmd/yt2column/codex_review_docs_test.go`、`cmd/fixprguard/docs_test.go`

-   [ ] `TestFixprguardCallSites` の表に、Phase 6 の `fixprguard append-pr ` の行を加える。
-   [ ] Phase 6 の訂正の段落の数を取得する式を確かめるテストを、`runJQ` と `bodyMarker` を使って加える（4.1 節の 4-4 の行）。
-   [ ] `postingCommand` に `append-pr` と `finish` を加え、`postingSections` の Phase 6 の行に `fixprguard append-pr ` を、新しい Phase 4 の行に `fixprguard finish ` を加え、冒頭のコメントを直す（1.4 節「投稿のコマンドとメンションの確認」）。`TestIsPost` の表に、1.4 節の 3 つの形の行を加える。
-   [ ] `TestDocumentedInvocationsParse` の、`fixpr.md` の期待するサブコマンドの一覧に `append-pr` を加える。

**完了の条件:** 文書のガードが通る。4.1 節の 4-4 の行の「壊して確かめる」の各項目で該当のテストが落ちることを確かめ、コミットメッセージに書いた。

#### ステップ 4-5: `local-cli-review.md`・`runplan.md`・`mkarch.md`・`mkplan2.md`

**変更するファイル:** `.claude/commands/_lib/local-cli-review.md`、`.claude/commands/runplan.md`、`.claude/commands/mkarch.md`、`.claude/commands/mkplan2.md`

-   [ ] `local-cli-review.md` 手順 7 の記録の行の一覧に、訂正の行を加える。修正が承認済みのタスク文書への編集上の訂正であれば、手順 5 の commit のメッセージに書くことにする。手順 7 のメンションの確認の対象に、訂正の行を含むメッセージを加え、この確認が手順 5 の commit の前に行われることを書く（1.4 節「コマンドの定義と規則」）。
-   [ ] `runplan.md:63`（編集上の訂正）と `runplan.md:64`（決定の変更）を、設計書 3.6 節の表と 3.7 節のとおりに改める。RECORD（`settle-decisions.md` に渡す記録先）は「実装の際に決めたこと（YYYY-MM-DD）」の節とし、PR のあるブランチで作業しないことを、根拠とともに一文で書く。
-   [ ] `mkarch.md:72` を、規則の「Editing an approved document」を参照する形に改める（設計書 3.6 節）。
-   [ ] `mkplan2.md:107`（手順 8.5）を、設計書 3.6 節の表のとおりに改める。
-   [ ] 遷移を書く 5 つのコマンドの定義が、旧規則で書かれた文書の移し方の節を参照していることを確かめる（`runpr.md` の PRE_MERGE、`fixpr.md` の Phase 3、`runplan.md`、`mkarch.md`、`mkplan2.md`。設計書 3.3 節の末尾）。

**完了の条件:** 文書のガード（`cmd/codexreview` を含む）が通る。AC-06 のために読む箇所を、`.claude/commands/` で `Comments` と `editorial` を検索して洗い出した。対象のコマンドの定義のどれも、編集上の訂正を `Comments` や変更履歴に書かせていないことは、洗い出した箇所を読んで確かめる（`manual`）。洗い出した箇所の一覧は PR の本文に書く。

### フェーズ 5: AC-19 の差分の確かめ（設計書 8 章のフェーズ 5）

#### ステップ 5-1: PR-2 のブランチで確かめを実行する

**変更するファイル:** 本書 7.2 節

-   [ ] PR-2 のブランチで、ほかの作業がすべて終わった後に、ステップ 1-5 のスクリプトを実行する。出力・終了コード・マージベースと、実行した時点の head のコミット ID を本書 7.2 節に記録する。記録を書くコミットが本書だけを変えることも、あわせて記録する。

**完了の条件:** スクリプトの終了コードが 0 で、その記録が本書 7.2 節にある。

### PR-2 作成ポイント: status comments policy and command definitions

**対象ステップ**: 2-1 / 3-1 / 3-2 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 5-1

**推奨タイトル**: `docs(0053): Comments を今の状態の理由に絞り、訂正を PR 本文へ、遷移を変更履歴へ移すよう規則とコマンドの定義を改める`

**レビュー観点**: 規則とコマンドの定義が食い違わず、書式が規則の 1 か所だけにあること（AC-20。訂正の行の接頭辞と訂正の段落の書き出しを含む） / PRE_MERGE の PR 番号の記入と、承認しない選択肢の扱い / `/fixpr` のセッションの確認と、PR 本文をセッションに入れないこと / 手順 4.3 を「PR 番号を記録してマージ」の選択肢からも通すことと、その失敗からの戻り方 / 本文を出力しない数の取得の式（`TestFixprPhase6CountFilter`）と、旧 `Comments` の写しをバイト列で比べる手順（設計書 3.3 節） / AC-03 の `runplan.md` の読み替え（設計書 3.7 節）と、`finish` を投稿のコマンドに加えたこと（1.4 節） / PR-2 自身の `/fixpr`・`/runpr` が新しい手順の最初の実行になること（5 章）

**実装モデル要件**: frontier-required

**判定理由**: 4-2・4-3 は GitHub に届く文面のセッションの確認（セキュリティの関門）を定める。2-1〜4-5 は、旧規則とそれに従うコマンドの定義を同時に置き換える移行である。どちらも `mkplan.md` 手順 8 の panel mode の条件に当たる。ステップの順は設計書 8 章のフェーズの順（2 → 3 → 4）による。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: ツール | `finish --body-file` と `append-pr`、手引き、パッケージの説明、AC-19 のスクリプト | PR-1 がマージされ、利用者が `make install-fixprguard` で再インストールした |
| M2: 規則とコマンドの定義 | 新しい規則、`runpr.md`・`task-pr-handoff.md`・`fixpr.md`・`local-cli-review.md`・`runplan.md`・`mkarch.md`・`mkplan2.md`、文書のガード | PR-2 のマージ |

PR-2 は M1 の後に始める。PR-2 の `/fixpr` が新しいツールを使うためである（1.3 節）。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | `fixprguard` の `finish --body-file` と `append-pr`、手引き、`package_reference.md`、AC-19 のスクリプトと PR-1 での実行 | frontier-required |
| PR-2 | 2-1 / 3-1 / 3-2 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 5-1 | 規則、PRE_MERGE と PR 本文、書き込む側のコマンドの定義、文書のガード、PR-2 での AC-19 の実行 | frontier-required |

## 4. テスト戦略 (Test Strategy)

### 4.1. 単体テスト

テストの名前は予定の名前である。実装のときに変えたら、本書 7 章を同じコミットで直す。どのテストも、`readTextFile` の規則（`TestBodyFileChecks`、`write_test.go:30`）の検査項目を 1 つずつ繰り返さない。`readTextFile` を通すことを示す 1 行だけを置く（`TestEditPRVerbatim` の `rejects_before_any_call` と同じ考え方。`write_test.go:369-370`）。補助は既存の `WriteHook`（`internal/fixprguard/test_helpers_git.go:217`）、`makeStep`（同 290）、`writeTextFile`、`trickyTexts`（`write_test.go`）、`runJQ`、`bodyMarker`（`cmd/yt2column/fixpr_docs_test.go`）を使い、新しい補助のファイルは作らない。

| ステップ | テスト（置き場所） | 確かめること | 壊して確かめる |
|---|---|---|---|
| 1-1 | `TestFinishCommitBody`（`internal/fixprguard/worktree_flow_test.go`） | 本文を渡すと、記録されたメッセージが組み立てた文字列のとおりになる。本文が LF で終わる場合と終わらない場合、末尾に空白がある場合を含む。`make fmt` の fake が build checks の間に本文のファイルを書き換えても、記録されるのは検査した本文である | `--cleanup=verbatim` を外す。commit でファイルを git に渡す |
| 1-1 | `TestFinishCommitSubject`（同。既存のテストを広げる） | 本文がないと、メッセージ全体が件名だけになる | 本文がないときも空の本文を付ける |
| 1-1 | `TestFinishRejectsBodyFile`（同） | 次の入力では、どれも `ErrInvalidInput` を返し、`make` の fake が一度も呼ばれず、HEAD が変わらない。入力は、メンション、`Editorial correction: ` で始まらない行、説明のない訂正の行、`@`、制御文字（CR を含む）、空の行、2 つ以上の末尾の LF、`readTextFile` が拒否する 1 例である。作業ツリーの中の本文のファイルでは、判定の後に現れた変更として止まる（今の `judgedSetChanged` の誤り） | 行の形の検査を外す。検査を照合の後に移す |
| 1-1 | `TestFinishDetectsRewrittenMessage`（同） | hook は baseline を取る前に置く（`CheckState` が hook を見ているため）。本文があり、commit-msg の hook がメッセージを変えると、`PartHead` の `GitStateError` を返し、SHA を返さない。本文がなく、hook が trailer を足しても、今までどおり成功して SHA を返す | メッセージの確認を外す。本文がないときにも確認する |
| 1-1 | `TestCommitObjectMessage`（`internal/fixprguard/gitstate_test.go`） | 複数行の `gpgsig` ヘッダーを持つ commit オブジェクト（`git hash-object -t commit -w` で作る）から、メッセージをそのまま読む | メッセージの読み出しを最初の LF で切る |
| 1-1 | `TestCommitBodyChecks`（`internal/fixprguard/write_test.go`） | 訂正の行の形の検査の関数の、受け付ける形と拒否する形の境界（1 行、複数行、末尾の LF が 0 個と 1 個） | 末尾の LF の数の確認を外す |
| 1-2 | `TestAppendPR`（`write_test.go`） | 今の本文（`trickyTexts`、空、LF で終わる、CRLF を含む）の後ろに、空行を挟んでファイルの内容を足した本文を、標準入力で、`--title` なしで書き込む。今の本文のバイト列は変わらない。足した後がちょうど上限なら書き込む | `--title` を付ける。今の本文を整える |
| 1-2 | `TestAppendPRRejects`（同） | ファイルの `@`・`<`・メンションと `readTextFile` が拒否する 1 例では、`gh` を一度も呼ばない。足した後が上限を 1 バイト超える場合と、今の本文がメンションを含む場合は `ErrInvalidInput` を返し、`pr edit` を呼ばない。`pr view` の失敗、上限を超える応答、`body` がない・`null`・文字列でない応答は `ErrResponseRejected` を返す。どの誤りにも、今の本文に置いた目印が含まれない | `@`・`<` の拒否を外す。足した後の検査を外す |
| 1-3 | `TestParseAppendPR`・`TestParseFinishBodyFile`（`cmd/fixprguard/main_test.go`） | `append-pr` は `--body-file` を必須とし、`[PR]` を省略できる。`finish` は `--body-file` を受け付け、省略もできる | `specs` から `append-pr` を外す |
| 1-3 | `TestFinishBodyFileRun`（同） | 一時ディレクトリのリポジトリで `finish --body-file` を実行すると、commit のメッセージに本文が入る。`append-pr` の出力が `{"protocol":4,"written":true}` である | `main.go` で `BodyFile` を渡さない |
| 1-3 | 既存の表に行を加える（同） | `lines`・`validArgs` の `append-pr` の行（`GH_REPO`・`GH_HOST`、`--protocol`、`--reviewer` の拒否）。`TestPostTextMentionExitsUsage` の `finish` の本文、`append-pr` のファイル、メンションを含む今の本文の行（終了コード `2`、診断が文を含まない）。メッセージが変わったときの止まった種類（`git-state-changed`、終了コード `1`）は、`TestFinishBodyFileRun` の部分テストで確かめる（`TestStopKinds` は止まった種類ごとに 1 つの場面を持つ表で、`git-state-changed` の場面は既にある。`main_test.go:570`） | `append-pr` の処理を外す |
| 4-4 | `TestFixprPhase6CountFilter`（`cmd/yt2column/fixpr_docs_test.go`） | Phase 6 のうち、このセッションが実行する部分（エージェントへの指示の引用の外）に、数を取得するコマンドがちょうど 1 つある。その `--jq` の式を、`bodyMarker` と N 個の訂正の段落、似た形（字下げした段落、行の途中の書き出し、CRLF）を含む `{"body": ...}` に当てると、出力がちょうど整数 N になり、目印を含まない。`jq` がなければ、読み飛ばさずに落ちる | 式が本文を出力する形にする。式が常に `0` を出力する形にする |
| 4-4 | `TestFixprguardCallSites`・`TestIsPost`・`TestPostingPathsCheckMention`・`TestPostingCommandsInventory`・`TestDocumentedInvocationsParse`（既存の表に行を加える） | 1.4 節「文書のガード」の変更 | `postingCommand` から `append-pr` を外す。Phase 4 のメンションの確認を消す |

`TestFixprPhase6CountFilter` の入力の訂正の段落の書き出し `**編集上の訂正（` は、`CLAUDE.md`「Go Idioms」のガードのテストのリテラルの例外（太字の目印）に当たる。

訂正の行の接頭辞 `Editorial correction: ` と訂正の段落の書き出し `**編集上の訂正（` は、規則・`fixpr.md`・ツールの 3 か所に書かれる。この値の照合のテストは、規則とコマンドの定義の間には加えない（設計書 7.1 節「ほかの文書のテスト」）。`fixpr.md` の数の取得の式と段落の書き出しは、上の `TestFixprPhase6CountFilter` が確かめる。規則と `fixpr.md` の書き出しが合っていることは、PR-2 のレビューで読んで確かめる（PR-2 のレビュー観点）。

### 4.2. 統合テスト

N/A（設計書 7.2 節）。コマンドの定義を実行した結果は、本タスクの後の PR で分かる（要件書 2.3）。

### 4.3. 後方互換のテスト

-   `--body-file` のない `finish` の振る舞いが変わらないことは、既存の `TestFinish*` のテスト（`internal/fixprguard/worktree_flow_test.go:72-390` と `625`）がそのまま通ること、および `TestFinishDetectsRewrittenMessage` の hook が trailer を足す行で確かめる。
-   既存のサブコマンドの出力の形が変わらないことは、既存の `cmd/fixprguard/main_test.go` のテストがそのまま通ることで確かめる。
-   旧規則で書かれた既存の文書を変えないこと（AC-19）は、`_checks/` のスクリプトで確かめる（7 章）。

### 4.4. セキュリティのテスト

4.1 節の 1-1・1-2・1-3 の拒否の行（メンション、`@`、`<`、行の形、制御文字、診断に文を含めないこと）と、4-4 の本文を出力しない式とメンションの確認の行が当たる（設計書 7.4 節）。`/fixpr` のセッションの確認の文言は、PR のレビューで読んで確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| PR-1 のマージの後に再インストールせず、PR-2 の `/fixpr` が古いツールで止まる | PR-1 の本文に再インストールの案内を書く（PR-1 作成ポイント）。止まっても安全側で、手引き 4 章の終了コード `2` の説明が、2 つの診断と再インストールを案内する（ステップ 1-4） |
| 記録されるメッセージが、組み立てた文字列と末尾の改行などで食い違う | ツールがちょうど 1 つの LF で終わる形に組み立てる（1.4 節）。`TestFinishCommitBody` が、LF で終わる本文と終わらない本文の両方で、記録されたメッセージを読み出して比べる |
| commit-msg の hook や `commit.gpgsign` の設定が、本文のある commit のメッセージや commit オブジェクトの形を変える | メッセージの一致の確認が hook による変更を止める（`TestFinishDetectsRewrittenMessage`）。署名のヘッダーの読み方は `TestCommitObjectMessage` が確かめる |
| 今の PR 本文が既にメンションを含み、`append-pr` が毎回止まる | 止まった訂正は最終報告に挙がり、レビュアーが PR 本文を直す（設計書 4.2 節）。セッションは、これを本文のファイル自体の誤りと区別できないが、どちらも終了コード `2` の診断の 1 行が、当たった検査の名前を示す |
| PR-2 が大きく、規則とコマンドの定義の食い違いを見落とす | PR-2 のレビュー観点に AC-20 を挙げる。書式を規則の 1 か所にだけ書き、コマンドの定義は節を参照する（1.2 節）。PR-2 はステップごとに 1 つのコミットにし、差分をステップごとに読めるようにする。PR の本文には、変えたコマンドの定義の節と、それが参照する規則の節の対応を挙げる |
| PR-2 の作成（`task-pr-handoff.md` Part B）、PR-2 の `/fixpr`、`/runpr` の PRE_MERGE と手順 4.3 は、ブランチの新しい定義に従うので、PR-2 自身が新しい手順の最初の実行になる | PR-2 の PRE_MERGE の後に、レビュアーが記録の commit の差分と、この PR が加えた行に `#未記入` が残っていないことを確かめる。手順 4.3 が途中で止まったときは、記録を手で commit して push し、`/mergepr` でマージする |
| `append-pr` の取得と書き込みの間に、レビュアーが PR 本文を編集する | 防げない残余のリスクとして受け入れる（設計書 5 章「残余のリスク」） |
| `/runpr` を通らずにマージして `#未記入` が `main` に残る | 次の `/runpr` が手順 4 で示す（ステップ 3-1）。直し方は設計書 4.3 節 |
| AC-19 の確かめの後に、`/runpr` の中の `/fixpr` の commit が加わる | 記録は、実行した時点の head までを確かめたことを示す。それより後の commit は PR のレビューで見る |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 3-1 / 3-2 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 5-1）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別は `test`（実行するテスト）、`static`（ガードのテスト・`make` の対象）、`diff`（`_checks/` のスクリプト）、`manual`（PR のレビューで読む）である。AC-01〜AC-18 と AC-20 は規則とコマンドの定義の文言を定めるので、`manual` で確かめる（設計書 7.5 節）。そのうち、`fixprguard` の振る舞いや信頼の境界のコマンドを含む AC には、`test` と `static` も挙げる。

| AC | 種別 | 確かめ | ステップ |
|---|---|---|---|
| AC-01 | manual・test・static | PR-2 のレビューで、規則の「Editing an approved document」と訂正の行の節を読む。`/fixpr` のコミットメッセージの部分は `internal/fixprguard/worktree_flow_test.go::TestFinishCommitBody`・`TestFinishRejectsBodyFile`・`TestFinishDetectsRewrittenMessage`、`internal/fixprguard/write_test.go::TestCommitBodyChecks`、`cmd/fixprguard/main_test.go::TestFinishBodyFileRun`、`cmd/yt2column/codex_review_docs_test.go::TestPostingPathsCheckMention`（Phase 4 の行） | 1-1・1-3・2-1・4-2・4-4・4-5 |
| AC-02 | manual | PR-2 のレビューで、`task-pr-handoff.md` Part B 手順 3 と `runpr.md` 手順 1 を読む | 3-2 |
| AC-03 | manual・test・static | PR-2 のレビューで、`fixpr.md` Phase 6 と `runplan.md:63` の書き換え（設計書 3.7 節の読み替え）を読む。`internal/fixprguard/write_test.go::TestAppendPR`・`TestAppendPRRejects`、`cmd/fixprguard/main_test.go::TestParseAppendPR`・`TestFinishBodyFileRun`、`cmd/yt2column/fixpr_docs_test.go::TestFixprguardCallSites`、`cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`、`cmd/yt2column/codex_review_docs_test.go::TestPostingCommandsInventory`・`TestPostingPathsCheckMention`・`TestIsPost` | 1-2・1-3・4-3・4-4・4-5 |
| AC-04 | manual・static | PR-2 のレビューで、`fixpr.md` Phase 6 の書き直しの指示と段落の数の確認を読む。段落の数を取得する式は `cmd/yt2column/fixpr_docs_test.go::TestFixprPhase6CountFilter` | 4-3・4-4 |
| AC-05 | manual | PR-2 のレビューで、`fixpr.md` の Phase 6 と最終報告、`runpr.md` 手順 4 の問いを読む | 3-1・4-3 |
| AC-06 | manual | PR-2 のレビューで、対象のコマンドの定義を読む（読む箇所はステップ 4-5 で洗い出し、PR の本文に挙げる） | 3-1・4-1・4-5 |
| AC-07 | manual | PR-2 のレビューで、規則の「Document Status Format」を読む | 2-1 |
| AC-08 | manual | 同上 | 2-1 |
| AC-09 | manual | 同上 | 2-1 |
| AC-10 | manual | PR-2 のレビューで、規則の変更履歴の節を読む | 2-1 |
| AC-11 | manual | 同上 | 2-1 |
| AC-12 | manual | PR-2 のレビューで、規則の未記入の印の記述と、`runpr.md` の PRE_MERGE・手順 4・手順 4.3、`runplan.md`・`mkplan2.md`・`mkarch.md` の記述を読む | 2-1・3-1・4-5 |
| AC-13 | manual | PR-2 のレビューで、`runpr.md` の PRE_MERGE を読む | 3-1 |
| AC-14 | manual | PR-2 のレビューで、規則と `fixpr.md` Phase 3・`mkarch.md`・`mkplan2.md` を読む | 2-1・4-1・4-5 |
| AC-15 | manual | PR-2 のレビューで、規則と `runplan.md:64` を読む | 2-1・4-5 |
| AC-16 | manual | PR-2 のレビューで、規則の `withdrawn` の記述を読む | 2-1 |
| AC-17 | manual | PR-2 のレビューで、規則の旧規則の文書の定義を読む | 2-1 |
| AC-18 | manual | PR-2 のレビューで、規則の移し方の節を読む | 2-1 |
| AC-19 | diff | `docs/tasks/0053_status_comments_policy/_checks/task_docs_unchanged.sh`（PR-1 は 7.1 節、PR-2 は 7.2 節に実行の記録） | 1-5・5-1 |
| AC-20 | manual | PR-2 のレビューで、規則と対象のコマンドの定義を読み比べる | 2-1・3-1・3-2・4-1〜4-5 |

### 7.1. PR-1 の実行の記録

（ステップ 1-4 の古いツールの診断と、ステップ 1-5 のスクリプトの負の場合と実行の結果を記録する。）

**古いツールの診断（ステップ 1-4）:** マージベース `0aef3ed8045a3508b85e8cf88778c3cb006b336f`（`origin/main`）のツリーをスクラッチパッドの `git worktree` に取り出し、そのディレクトリで `go run ./cmd/fixprguard` を実行した（2026-11-01）。`go run` 自身の `exit status 2` の行は、ツールの終了コード `2` を示す。

| 呼び出し | 標準エラー | 終了コード |
|---|---|---|
| `finish --protocol 4 --baseline v3.x --judged-set j --body-file /tmp/x.txt 42` | `fixprguard: usage: flag provided but not defined: -body-file (see docs/dev/guides/fixprguard_guide.md)` | `2` |
| `append-pr --protocol 4 --body-file /tmp/x.txt 42` | `fixprguard: usage (see docs/dev/guides/fixprguard_guide.md)` | `2` |

どちらも引数の解析で止まるので、値の文法（`--baseline`・`--judged-set`）を確かめる前の診断である。

**AC-19 のスクリプトの負の場合（ステップ 1-5）:** `git clone` で作った使い捨ての clone をスクラッチパッドに置き、マージベース `0aef3ed8045a3508b85e8cf88778c3cb006b336f` から 1 つの commit を作って、`task_docs_unchanged.sh 0aef3ed8045a3508b85e8cf88778c3cb006b336f HEAD` を実行した（2026-11-01）。PR のブランチは変えていない。

| 場合 | 終了コード |
|---|---|
| 本タスクのディレクトリの文書だけを変更 | `0` |
| ほかのタスクの文書（`docs/tasks/0044_article_job_api/01_requirements.md`）を変更 | `1` |
| ほかのタスクの文書を削除 | `1` |
| ほかのタスクの文書を本タスクのディレクトリへ名前を変更（`git mv`） | `1`（元の場所の削除として示す） |
| 本タスクの名前で始まる別のディレクトリ（`docs/tasks/0053_status_comments_policy_x/`）にファイルを追加 | `1` |
| `docs/tasks/` の外（`README.md`）だけを変更 | `0` |
| ほかのタスクの文書を変更し、`internal/` から実行 | `1` |
| 引数なし、引数 1 つ | `2` |
| コミットでない値（`nosuchref`） | `2` |
| リポジトリの外で実行 | `2` |
| `docs/tasks/` のない作業ツリーで実行 | `2` |

### 7.2. PR-2 の差分の確かめの実行の記録

（ステップ 5-1 で記録する。）

## 8. 成功基準 (Success Criteria)

-   **機能の完成度:** 6 章の 2 つの PR がマージされ、7 章のすべての行の確かめが済んでいる。
-   **品質:** 各 PR で、グリーンゲート（`make test && make lint`）と `make deadcode` が通る。4.1 節の「壊して確かめる」の各項目について、該当のテストが落ちることを確かめ、コミットメッセージに書いた。
-   **セキュリティ:** 4.4 節のテストが通る。
-   **文書:** 手引き・`package_reference.md`・規則・対象のコマンドの定義が、実装と設計書に合っている（PR のレビューで読む）。

## 9. 次のステップ (Next Steps)

-   PR-1 のマージの直後に、利用者が `make install-fixprguard` で再インストールする。
-   本タスクのマージの後、新しい規則に従った最初の PR（編集上の訂正だけの PR、決定の変更の PR）で、`Comments`・変更履歴・PR 本文の記録が規則どおりになるかを見る（要件書 2.3）。
-   既存の文書の一括の移し替えと、`/mergepr` での `#未記入` の確認は、必要になったら別のタスクにする（設計書 9 章）。
