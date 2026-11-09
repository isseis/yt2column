# 実装計画書：使われていない軽量プロセスの削除

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-09 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、H-NN は [design_handoff.md](design_handoff.md) の項番を指す。節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指し、本計画自身の節は「本計画 §N」と書く。用語は、要件書 6 章の用語（軽量プロセス・通常の進め方・軽量プロセスへの参照・簡易な進め方）と、設計書の冒頭の用語（専用ファイル・実装の PR・削除のコミット・書き換えのコミット・確認のスクリプト・マージの後の手順）を、同じ意味で使う。

既存のファイルの行番号は、コミット `ecefa1bdfa3958edf4a171ae7e0fae4d7474fb2d`（`main`）で確かめた。設計書が行番号を確かめたコミット `ccb06be5` からこのコミットまでに、`docs/tasks/` の外で変わったファイルは `docs/dev/adr/0040-simple-task-documents.md` だけである（`git log --oneline ccb06be5..ecefa1bd -- . ':!docs/tasks'` の出力は `489ababd` の 1 件）。ADR-0040 は本タスクで変えないので、設計書の行番号はそのまま使える。本タスクに `implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-06 は、どれも設計書の付録 B に扱いが記録されている。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

6 つの専用ファイルを削除し（F-001）、軽量プロセスへの参照を、今効いている指示・規約・コード・テストから取り除く（F-002）。通常の進め方のコマンドの振る舞いは変えない（F-003）。削除が済んだことは、タスクの `_checks/` の確認のスクリプトで一度だけ確かめる（F-006）。GitHub の `lightweight` ラベルは、実装の PR のマージの後に、マージの後の手順で確かめてから削除する（F-005）。ADR の変更（F-004）は要件書の PR（#753）で済んでいる。

### 1.2. 実装の原則

-   **変えるファイルは設計書 2.2 の表のとおりにする。** Go の製品のコード、拡張、`Makefile`、CI の設定、既存のタスク文書、ADR-0012 の 2 つのリンクのほかの ADR は変えない。
-   **文面は実装で決める。** 本計画は、除く箇所と除いた後に満たすことを設計書 3.2〜3.5 に委ね、新しい文面を書かない。例外は、テストのコメントの言い直し（ステップ 3-5）で、変更の前後の全文を書く。
-   **新しい恒久のテストは足さない**（AC-12、設計書 7.1）。
-   **削除のコミットには削除だけを入れる**（設計書 3.6）。本計画の進み具合の更新も、削除のコミットには入れない。
-   **どのコミットでも `make test` と `make lint` が通る**（設計書 1.1）。

### 1.3. 計画の際に決めたこと（2026-11-09）

次の 2 点は、計画を書く前に利用者に尋ねて決めた。どちらも本タスクの中で閉じる決定なので、ADR は作らない。

-   **設計書 8 章の優先順位の 1〜4 を、それぞれ 1 つのフェーズにし、最後のフェーズの後に PR の作成ポイントを 1 つだけ置く。** PR は設計書 1.2 のとおり 1 本である。理由：`/runplan` はフェーズのまとまりごとに、サブエージェントのレビューとローカルの CLI レビューを行ってから次へ進む（`.claude/commands/runplan.md:111`）。削除のコミットは、メッセージの amend とカバレッジの比較の記録が絡み、誤ると作り直しになる。そのため、削除のコミットだけを 1 つのフェーズにして、書き換えより前にレビューを通す。採らなかった案：フェーズを 1 つにする案。レビューは 1 回で済むが、削除と書き換えを 1 回のレビューでまとめて読むことになる。
-   **確認のスクリプトの語の一覧と除外の一覧は、`_checks/data.json` に置く。** 語の一覧には日本語の語（`軽量`・`仕様のコメント`）が含まれる。Go の文字列リテラルは英語で書く規則で、`_checks/` は guard-test literal exception（`CLAUDE.md`「Go Idioms」）の対象ではない。タスク 0071 の `_checks/main.go` と `_checks/data.json` と同じ形にする。設計書 7.3 の「スクリプト自身の blob の ID を出力する」は、`main.go` と `data.json` の両方の blob の ID を出力することとして実装する。採らなかった案：Go のソースに `\u` のエスケープで書く案。ファイルは 1 つで済むが、語が読めなくなる。

次の点は、設計書と既存の仕組みから導いた。PR のレビューで異論があれば改める。どれも本タスクの中で閉じるので、ADR は作らない。

-   **書き換えのコミットは、設計書 3.2〜3.5 の節と、3.6 のコメントの言い直しごとに 1 つずつ、計 5 つにする**（ステップ 3-1〜3-5）。設計書 6.1 の 3 が、数を実装計画書に委ねている。節ごとに分ければ、レビューで各コミットを設計書の 1 つの節と並べて読める。
-   **確認のスクリプトは、AC-11 の一部も確かめる。** `Makefile` と `.github/` が、そのコミットで本タスクの `_checks/` を参照しないことを確かめる項目を足す。設計書付録 A は AC-11 を `static` とするが、その確かめを実行する場所を定めていない。確認のスクリプトに入れれば、PR の head とマージされた `main` の両方で実行される。`_checks/` が `go list ./...` の対象の外であることは、ディレクトリ名が `_` で始まることで決まる（設計書 7.3）。
-   **`data.json` は、確かめるコミットからではなく作業ツリーから読む。** 確認のスクリプトは、そのディレクトリがまだない `main`（`main` の取り込みを確かめる前のマージベースなど）でも使えるようにする。読んだ `data.json` の blob の ID を出力して、結果をその版に結び付ける。作業ツリーから読むのは `main.go` と `data.json` だけで、確かめる対象のファイルはすべて引数のコミットから読む（設計書 7.3）。タスク 0071 と同じ扱いである。

### 1.4. 既存コード調査結果

製品のコードの変更はない。変えるファイルと、変更に関わる既存の仕組みは次のとおりである（行番号は `ecefa1bd`）。

| 対象 | 今あるもの | 本タスクですること |
|---|---|---|
| 専用ファイル（6 つ） | 設計書の冒頭の一覧のとおり。下の検索の一致は計 243 行 | ステップ 2-1 で削除する |
| `docs/tasks/` と `docs/dev/adr/` の外の参照 | 下の検索が一致したファイルは、専用ファイルのほかに、`_context.md`（7 行）、`_lib/local-cli-review.md`（1）、`_lib/task-pr-handoff.md`（17）、`mkreq.md`（42）、`mktrans.md`（1）、`runpr.md`（31）、`AGENTS.md`（1）、`CLAUDE.md`（25）、`codex_review_docs_test.go`（5）、`fixpr_docs_test.go`（3）、`requirements_process.md`（25）、`test_organization.md`（1）である。`docs/tasks/0000_template/` には一致がない。設計書 2.2 の表と同じ集合である | ステップ 2-1・3-1〜3-5 で除く。`test_organization.md:14` の「Lightweight mocks」は軽量プロセスと関係がないので、変えずに除外の一覧に入れる |
| `cmd/yt2column/codex_review_docs_test.go` | `postingSections`（`:56-87`、10 個。軽量プロセスの 2 行は `:79-87`）、`readOnlyInvocations`（`:154-163`、軽量プロセスの 2 項目は `:161`・`:162`）。`TestPostingPathsCheckMention`（`:128`）と `TestPostingCommandsInventory`（`:213`）がこれらを使う | ステップ 2-1 で 2 行と 2 項目を消し、ステップ 3-5 でコメントを直す |
| `cmd/yt2column/fixpr_docs_test.go` | 定数 `runissuePath`（`:27`）、`TestContextAndCommandsAgree`（`:988`）の表の行（`:1004`）とコメント（`:983-987`） | ステップ 2-1 で定数と行を消し、ステップ 3-5 でコメントを直す |
| `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（`:140`） | ADR を含む指示の文書の相対リンクの先があることを確かめる。ADR-0012 の `:26`・`:43` のリンクが専用ファイルを指す | ステップ 1-1 でリンクを permalink に直す（設計書 3.7） |
| `cmd/yt2column/adr_test.go::TestADRRecords`（`:434`） | ADR の構造と索引を確かめる | 変えない。AC-08 の確かめに使う |
| `cmd/codexreview/docs_test.go` | `doctestutil.CommandDocs` で列挙したコマンドの文書の `--protocol` を確かめる。`runissue.md` は列挙から消えるだけ | 変えない |
| `make coverage-compare` | `Makefile:199`。BASE のコミットと HEAD でテストを走らせて比べる | ステップ 2-2 で使う。BASE では `TestLightweightCheck` が走るので、`bash` と `--attr-source` を受け付ける `git` が要る（手元の `git version 2.50.1` は受け付ける） |
| `_checks/` の先例 | `docs/tasks/0071_security_source_link/_checks/main.go` と `data.json`（`//go:build test`、`go build -tags test -o <パス> ./docs/tasks/.../_checks/` でビルドして実行、終了コード 0・1・2） | ステップ 4-1 で同じ形にならう。git の読み取りの関数は写して使ってよい |
| `git grep` の振る舞い | `git grep -h` で、`-i`・`-F`・`--no-color`・`--no-textconv` があることを確かめた。`ecefa1bd` で `git -c core.quotePath=false grep -n -i -F --no-color --no-textconv -e 'lightweight mocks' HEAD -- . ':(exclude)docs/tasks' ':(exclude)docs/dev/adr'` の出力は `HEAD:docs/dev/conventions/test_organization.md:14:    ├── mocks.go              # Lightweight mocks (no external dependencies)` の 1 行で、終了コードは 0 だった | 出力の形（`<コミット>:<パス>:<行番号>:<内容>`）は設計書 7.3 の前提どおりである |
| `.github/ISSUE_TEMPLATE/` | `accessibility.md` と `lightweight.md`。`config.yml` はない。このディレクトリを読む Go と TypeScript のコードはない（`git grep -n ISSUE_TEMPLATE -- '*.go' '*.ts'` の出力が空） | `lightweight.md` を削除するだけ |

上の表の「下の検索」は、`ecefa1bd` で実行した次の 2 つのコマンドである。1 つ目の出力を、ファイルごとの行数に数えた。2 つ目の出力は空だった。語の一覧は設計書 7.3 の一覧である。

```bash
git grep -n -i -E 'lightweight|runissue|軽量|LW_ISSUE|spec comment|仕様のコメント|spec-marker|spec marker|revert marker|issue-<|ISSUE_KEYWORD|closingIssuesReferences|attr-source' HEAD -- . ':!docs/tasks' ':!docs/dev/adr'
git grep -n -i -E 'lightweight|runissue|軽量' HEAD -- docs/tasks/0000_template
```

具体的な型でふるまいを分ける設計はないので、構築の経路の列挙は要らない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: ADR-0012 のリンクの訂正（設計書 3.7）

**変更するファイル:** `docs/dev/adr/0012-lightweight-check-in-bash.md`、本計画（§6 の記録）

-   [ ] **ステップ 1-1**: 実装のブランチの基点（`git merge-base origin/main HEAD`）の完全な ID を求め、その木に `.claude/commands/_lib/lightweight-check.sh` と `cmd/yt2column/lightweight_docs_test.go` があることを確かめる。ADR-0012 の `:26` と `:43` のリンクの先を、その ID を使った permalink（`https://github.com/isseis/yt2column/blob/<ID>/<パス>`）に直す。リンクの文、状態、経緯、索引は変えない。コミットのメッセージに、設計書 3.7 の形の訂正の行を 1 行、行頭から書く。基点の ID を本計画 §6 に記録する。

**完了の条件:** ADR-0012 のほかの行が変わっていない。`make test` と `make lint` が通る。

### フェーズ 2: 削除のコミットとカバレッジの比較（設計書 3.6・7.2、AC-13）

**変更するファイル:** 設計書 3.6 の表と、その下の箇条に挙げたファイル。本計画（§7 の記録。削除のコミットとは別のコミット）

-   [ ] **ステップ 2-1（削除のコミット）**: 設計書 3.6 の削除を、1 つのコミットで行う。行や項目は、読んだ文面と同じ文字列を 1 つずつ消す（`.claude/commands/runplan.md` step 5）。
    -   [ ] `cmd/yt2column/lightweight_docs_test.go` を削除する。
    -   [ ] `internal/doctestutil/lightweight.go` を削除する。
    -   [ ] `.claude/commands/runissue.md` を削除する。
    -   [ ] `.claude/commands/_lib/lightweight-spec.md` を削除する。
    -   [ ] `.claude/commands/_lib/lightweight-check.sh` を削除する。
    -   [ ] `.github/ISSUE_TEMPLATE/lightweight.md` を削除する。
    -   [ ] `codex_review_docs_test.go` の `postingSections` から「runissue.md revert」の行を消す。
    -   [ ] `codex_review_docs_test.go` の `postingSections` から「mkreq.md lightweight triage」の行を消す。
    -   [ ] `codex_review_docs_test.go` の `readOnlyInvocations` から `runissueLinkedPRsQuery` を消す。
    -   [ ] `codex_review_docs_test.go` の `readOnlyInvocations` から `runpr.md` の step 2 の GraphQL の読み取り（`-F n=<number>` の項目）を消す。
    -   [ ] `fixpr_docs_test.go` の定数 `runissuePath` を消す。
    -   [ ] `fixpr_docs_test.go` の `TestContextAndCommandsAgree` の表から `runissue.md` の行を消す。
    -   [ ] `.claude/commands/mkreq.md` の step 1 の「Lightweight triage」の項目（`:70-191`）を消す。
    -   [ ] `.claude/commands/runpr.md` の分類「Lightweight PR」の項目（`:298-338`）を消す。

    `make fmt` が `fixpr_docs_test.go` の定数の桁をそろえ直す変更は、このコミットに含めてよい（設計書 3.6）。ほかの変更（コメントの言い直し、本計画の進み具合）は含めない。`make test` と `make lint` が通ることを確かめてからコミットする。
-   [ ] **ステップ 2-2**: 削除のコミットを HEAD にしたまま、`make coverage-compare BASE=<削除のコミットの親>` を、コマンドの時間の上限を 10 分以上にするか、バックグラウンドで実行する。最後の行が `coverage-compare: PASS` であることを確かめる。`FAIL` か `STOP <kind>` のときは、先へ進まずに止まり、出力を示して利用者に尋ねる（設計書 7.2）。`PASS` のときは、出力を最後の行まで、削除のコミットのメッセージに足す。amend はメッセージだけを変え、tree の ID が変わらないことを amend の前後の `git rev-parse HEAD^{tree}` で確かめる。
-   [ ] **ステップ 2-3**: 比較の出力、amend の後の削除のコミットの ID、その tree の ID を本計画 §7 の AC-13 の行に書き、本計画の進み具合とともにコミットする（削除のコミットとは別のコミット）。

**完了の条件:** 削除のコミットの差分が、ファイルの削除と行の削除（と `make fmt` の桁のそろえ直し）だけである。比較が `PASS` で、その出力が削除のコミットのメッセージと本計画 §7 にある。

### フェーズ 3: 書き換えのコミット（設計書 3.2〜3.6）

**変更するファイル:** `.claude/commands/mkreq.md`、`.claude/commands/runpr.md`、`.claude/commands/_lib/task-pr-handoff.md`、`.claude/commands/_context.md`、`CLAUDE.md`、`AGENTS.md`、`docs/dev/conventions/requirements_process.md`、`.claude/commands/mktrans.md`、`.claude/commands/_lib/local-cli-review.md`、`cmd/yt2column/codex_review_docs_test.go`、`cmd/yt2column/fixpr_docs_test.go`、本計画（進み具合）

各ステップを 1 つのコミットにする。どのステップも、テストの関数・表の行・サブテストを消さない（設計書 6.1 の 3）。各コミットの前に `make test` と `make lint` を実行する。テストが見出しや項目の書き出しで節を探す文書（`runpr.md`・`_lib/task-pr-handoff.md`・`mkreq.md`・`fixpr.md`）では、その目印の文字列を変えない。

-   [ ] **ステップ 3-1**: `mkreq.md` から、設計書 3.2 の「除くもの」のうち、ステップ 2-1 で消した step 1 の項目を除く残りを除く（`description`、冒頭の引用の文、紹介の段落、「Placeholders」の段落、「Authorization」の承認の文、`gh issue view` の `--json` の `labels`、step 9 の報告の 2 つの文）。step 1 の残りの 3 つの確認と、step 2 へ進む流れは変えない（AC-05）。
-   [ ] **ステップ 3-2**: `runpr.md` から、設計書 3.3 の表の箇所のうち、ステップ 2-1 で消した分類の項目を除く残りを除き、文の組み立てを直す。設計書 3.3 の箇条（Standalone change の定義は「anything else」のまま、`fixprguard` のための `gh repo view` と `gh api graphql` の承認と `gh api user` は残す、step 5 の `gh issue close` の前の mention check は残す）を守る（AC-06・AC-07）。
-   [ ] **ステップ 3-3**: `_lib/task-pr-handoff.md` と `_context.md` から、設計書 3.4 の箇所を除く。Part B 手順 3 の ISSUE の行は、`refs` の場合の文だけにする（H-01）。
-   [ ] **ステップ 3-4**: 設計書 3.5 の表のファイル（`CLAUDE.md`、`AGENTS.md`、`requirements_process.md`、`mktrans.md`、`_lib/local-cli-review.md`）から、表の箇所を除く。`CLAUDE.md` の `jq` の項目は、残る 3 つのテストと 3 つのフィルタを述べる文にする。`requirements_process.md` の「Lightweight Process」の節の後の区切りの線 `---` は残す。
-   [ ] **ステップ 3-5**: テストのコメントを、次のとおりに直す（設計書 3.6）。コードは変えない。
    -   `codex_review_docs_test.go` の `postingSections` のコメントの 1 行目を、`// postingSections are the ten sections that post text to GitHub. fixpr.md` から `// postingSections are the eight sections that post text to GitHub. fixpr.md` に変える。続く 2 行は変えない。
    -   `codex_review_docs_test.go` の `readOnlyInvocations` のコメントを、変更前の全文

        ```text
        // readOnlyInvocations are the gh api invocations that pass a text flag but
        // write nothing: the link count query of _lib/task-pr-handoff.md,
        // runissue.md's read of the pull requests linked to its issue, and runpr.md's
        // read of a PR's head branch and linked issues. Each query follows on later
        // lines, so the code block it opens must hold no mutation.
        ```

        から、変更後の全文

        ```text
        // readOnlyInvocations are the gh api invocations that pass a text flag but
        // write nothing: the link count query of _lib/task-pr-handoff.md. Each query
        // follows on later lines, so the code block it opens must hold no mutation.
        ```

        に変える。
    -   `fixpr_docs_test.go` の `TestContextAndCommandsAgree` のコメントの 4〜5 行目を、`// mention fixprguard rejects, and fixpr.md, runpr.md, runissue.md, mkreq.md,` と `// and _lib/task-pr-handoff.md never write a reviewer's name.` から、`// mention fixprguard rejects, and fixpr.md, runpr.md, mkreq.md, and` と `// _lib/task-pr-handoff.md never write a reviewer's name.` に変える。

    `make fmt` を実行する。

**完了の条件:** 5 つの書き換えのコミットがこの順にあり、どれもテストを消していない。`make test` と `make lint` が通る。

### フェーズ 4: 確認のスクリプトと最後の確かめ（設計書 7.1・7.3、AC-01〜AC-04・AC-11）

**変更するファイル:** `docs/tasks/0075_remove_lightweight_process/_checks/main.go`（新規）、`docs/tasks/0075_remove_lightweight_process/_checks/data.json`（新規）、本計画（§7 の記録）

-   [ ] **ステップ 4-1**: 確認のスクリプトを `_checks/main.go` と `_checks/data.json` に書く（本計画 §1.3 の形）。確かめる項目は、設計書 7.3 の表の 2 行（AC-01、AC-02・AC-03）と、本計画 §1.3 の AC-11 の項目である。設計書 7.3 の性質（コミットを 1 つ引数で受け取って完全な ID に解決する、作業ツリーの対象のファイルを読まない、git の設定の影響を除く、出力の行を読めなければ失敗する、`git grep` の終了コードを区別する、`docs/tasks/0000_template/` を範囲に戻す、除外はパスと行の内容の全体の組で各々ちょうど 1 回当たる、違反と読み取りの失敗で 0 でない終了コード、違反のファイルと行番号と内容を出力する、確かめたコミットの完全な ID と `main.go`・`data.json` の blob の ID を出力する）を持たせる。コメント・識別子・文字列リテラルは英語で書く。`make fmt` と `go vet -tags test ./docs/tasks/0075_remove_lightweight_process/_checks/` を通す。
-   [ ] **ステップ 4-2**: 確認のスクリプトが、狙った理由で失敗できることを確かめる。PR のブランチから `git worktree` で使い捨てのブランチを作り、次の壊し方を 1 件ずつ別の一時的なコミットとして作る。それぞれのコミットを引数にして、ビルドした実行ファイルを PR のブランチの作業ツリーから走らせ、狙った項目が失敗することと、終了コードを確かめる。
    -   AC-01：専用ファイルの 1 つを戻す。
    -   AC-02：残すファイルの 1 行に語の一覧の語を足す。
    -   AC-02（範囲）：`docs/tasks/0000_template/` のファイルに語の一覧の語を足す。
    -   除外：`test_organization.md` の「Lightweight mocks」の行を変える（除外が当たらなくなる）。
    -   AC-11：`Makefile` に本タスクの `_checks/` のパスを足す。
    -   読み取りの失敗：存在しないコミットを引数にする。

    加えて、`docs/tasks/` の別のタスク文書と `docs/dev/adr/` の ADR に語を足しても合格することを 1 件ずつ確かめる。使い捨てのブランチは push せず、確かめた後に worktree とともに消す。壊し方ごとの結果をコミットのメッセージに書いて、`_checks/` の 2 つのファイルをコミットする。
-   [ ] **ステップ 4-3**: PR の head で、確認のスクリプト、`make test`、`make lint`、`make deadcode` を実行する（AC-01〜AC-04・AC-11）。確認のスクリプトの出力と終了コード、確かめたコミットの完全な ID、3 つの `make` の結果を本計画 §7 に書き、本計画の進み具合とともにコミットする。このコミットは本タスクのディレクトリの下しか変えないので、記録した結果はそのまま head に当てはまる（設計書 6.1 の 5）。
-   [ ] **ステップ 4-4**: PR を作る直前に、`git fetch origin main` の後で `origin/main` を取り込む必要があるかを確かめる。取り込むときは、設計書 6.2 のとおり merge で行い（rebase しない）、確認のスクリプトと 3 つの `make` を実行し直して、本計画 §7 の記録を改める。取り込みでテストの削除が新たに要るときは、設計書 6.2 の 3 つ目の箇条のとおり、もう 1 つの削除のコミットを作り、フェーズ 2 と同じく比較して記録する。時点は、`/runplan` の手順 7.5・7.7 のレビューの修正を入れた後で、手順 5a でグリーンゲートを確かめる前である。その後に本タスクのディレクトリの外を変えるコミット（ローカルの CLI レビューの修正など）が入ったときも、このステップをやり直す。

**完了の条件:** 確認のスクリプトが終了コード 0 で終わり、その記録が本計画 §7 にある。壊し方ごとの結果が `_checks/` のコミットのメッセージにある。`make test`・`make lint`・`make deadcode` が通る。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | ステップ |
|---|---|---|
| M1: リンクの訂正 | 訂正の行つきの ADR-0012 のコミット | 1-1 |
| M2: 削除 | 削除のコミット（比較の出力つき）と本計画 §7 の記録 | 2-1〜2-3 |
| M3: 書き換え | 5 つの書き換えのコミット | 3-1〜3-5 |
| M4: 確認 | 確認のスクリプトと、PR の head での実行の記録 | 4-1〜4-4 |
| M5: マージの後の手順 | #744 への結果のコメント | 本計画 §10 |

各マイルストーンは前のマイルストーンを前提とする（設計書 6.1）。M4 の後に `/runplan` が PR を作る。M5 は `/runpr` が実装の PR をマージした後に、手で行う。

## 4. テスト戦略 (Test Strategy)

### 4.1. 単体テスト

新しいテストは足さない（AC-12）。各コミットで `make test` と `make lint` を、PR の head で `make deadcode` も実行する（AC-04）。残るテストが確かめることは設計書 7.1 のとおりである。削除のコミットで消すテストは、ステップ 2-2 の比較で確かめる（AC-13）。

### 4.2. 確認のスクリプト

AC-01〜AC-03 と AC-11 の一部を、確認のスクリプトで、PR の head（ステップ 4-3・4-4）とマージされた `main`（本計画 §10）で確かめる。確認のスクリプトが違反と読み取りの失敗を検出できることは、ステップ 4-2 で項目ごとに壊して確かめる。

### 4.3. 読んで確かめる

AC-02・AC-05・AC-06・AC-12 は、PR のレビューで読む（設計書 7.4）。

### 4.4. 結合テスト・後方互換のテスト

N/A。実行時の振る舞いを変えない（設計書 7.5）。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 削除のコミットに削除でない変更が混ざる | AC-13 の比較が削除だけを確かめたことにならない | ステップ 2-1 で項目ごとに消し、本計画の進み具合は別のコミットにする。フェーズ 2 の後のレビューで差分を読む |
| 削除のコミットの後に別のコミットを作ってから amend しようとする | メッセージだけの amend ができず、比較の記録の ID がずれる | ステップ 2-2 を、削除のコミットの直後に行う。tree の ID を amend の前後で比べる |
| 親のコミットで `TestLightweightCheck` が `bash` や `git` の不足で失敗する | 比較が `STOP` で終わる | 手元の `git` が `--attr-source` を受け付けることを本計画 §1.4 で確かめた。`STOP` のときは止まって利用者に尋ねる（ステップ 2-2） |
| 書き換えで、テストが節を探す目印の文字列を変える | `make test` が失敗する | フェーズ 3 の冒頭の注意を守り、各コミットの前に `make test` を実行する |
| タスク 0072・0073・0074 の実装が先にマージされ、衝突や新しい参照が入る | AC-03 を満たさない | ステップ 4-4 で取り込み、確認のスクリプトで新しい参照を見つける（設計書 6.2）。ADR-0012 の衝突は `docs/dev/conventions/adr.md`「Taking in `main`」に従って解き、直すのはリンクの先だけとする（設計書 3.7） |
| 最後の確認とマージの間に、ほかの PR が参照を `main` に入れる | マージされた `main` で AC-02 を満たさない | 本計画 §10 で、マージされた `main` を確かめ直す |
| 語の一覧に当たらない言い換えの参照が残る | AC-02 を満たさない | PR のレビューで読む（設計書 7.3 の「語の判断の限界」） |
| ラベルを誤って消す、または消すべきでないときに消す | 元に戻せない | 本計画 §10 の手順（設計書 6.3）で、件数の確かめ、承認、削除の直前の読み直しを行う |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] ステップ 1-1 の記録（実装のブランチの基点：（未記録））
- [ ] フェーズ 1 完了（ステップ 1-1）
- [ ] フェーズ 2 完了（ステップ 2-1〜2-3）
- [ ] フェーズ 3 完了（ステップ 3-1〜3-5）
- [ ] フェーズ 4 完了（ステップ 4-1〜4-4）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

確認のスクリプトは `docs/tasks/0075_remove_lightweight_process/_checks/`（`main.go` と `data.json`。以下、`_checks`）である。実行の記録（日付、確かめたコミットの完全な ID、`main.go` と `data.json` の blob の ID、出力、終了コード）は、ステップ 4-3・4-4 で各行に書く。

| AC | 種類 | 検証 | ステップ |
|---|---|---|---|
| AC-01 | static | `_checks`（6 つの専用ファイルが、確かめたコミットにない）。PR の head で実行する。マージされた `main` での実行は本計画 §10 で行い、#744 に記録する | 2-1・4-1〜4-4 |
| AC-02 | static | `_checks`（`docs/tasks/`（`0000_template/` を除く）と `docs/dev/adr/` の外で、語の一覧に当たる行が除外のほかにない）。実行の時点は AC-01 と同じ | 2-1・3-1〜3-5・4-1〜4-4 |
| AC-02 | manual | 書き換えた文書が軽量プロセスへの参照を持たず、文として成り立っていること、除外の行が軽量プロセスと関係がないことを、PR のレビューで読む（設計書 7.4） | 3-1〜3-4 |
| AC-03 | static | `_checks` を、`origin/main` を取り込んだ後の head と、マージされた `main` で実行する | 4-4、本計画 §10 |
| AC-04 | test | `make test`・`make lint`・`make deadcode`（PR の head）。`TestLightweightCheck` が残らないことは、AC-02 の `_checks` が `lightweight` の語で確かめる | 4-3・4-4 |
| AC-05 | manual | `mkreq.md` の step 1 の残りの確認と、step 2 へ進む流れを、PR のレビューで読む（設計書 7.4） | 2-1・3-1 |
| AC-06 | manual | `runpr.md` の分類と、Standalone change の PR を作る手順を、PR のレビューで読む（設計書 7.4） | 2-1・3-2 |
| AC-07 | test | `cmd/yt2column/codex_review_docs_test.go::TestPostingPathsCheckMention`、`cmd/yt2column/codex_review_docs_test.go::TestPostingCommandsInventory`（`make test` で実行） | 2-1・3-1〜3-3 |
| AC-08 | test | `cmd/yt2column/adr_test.go::TestADRRecords`（`make test` で実行）。ADR の変更は #753 で済んでいる | 1-1・4-3 |
| AC-09 | （マージの後） | 本計画 §10 の手順で確かめ、#744 のコメントに記録する。PR を作る時点では確かめられない既知の不足である（設計書付録 A） | 本計画 §10 |
| AC-10 | （マージの後） | 同上 | 本計画 §10 |
| AC-11 | static | `_checks` の AC-11 の項目（`Makefile` と `.github/` が本タスクの `_checks/` を参照しない）。`go list ./...` の対象の外であることは、ディレクトリ名による | 4-1・4-3 |
| AC-12 | manual | 実装の PR の差分が、テストを足していないことを、PR のレビューで読む（設計書 7.4） | 全ステップ |
| AC-13 | test | `make coverage-compare BASE=<削除のコミットの親>` の出力（最後の行が `coverage-compare: PASS`）。削除のコミットのメッセージ、PR の説明、この行に記録する。記録：（未記録） | 2-2・2-3 |

## 8. 成功の基準 (Success Criteria)

-   **機能の完全性:** 専用ファイルがなく、`docs/tasks/` と `docs/dev/adr/` の外に軽量プロセスへの参照がない。本計画 §7 の AC-09・AC-10 のほかの検証が済んでいる。
-   **品質:** `make test`・`make lint`・`make deadcode` が通る。`_checks` が終了コード 0 で終わり、項目ごとに壊して失敗することを確かめた記録がコミットのメッセージにある。削除のコミットの比較が `PASS` である。
-   **セキュリティ:** 残るコマンドの投稿の箇所の守り（mention check、`fixprguard`）が今と同じである（AC-07）。
-   **文書:** 変えたファイルが設計書 2.2 の表のとおりである。

## 9. 次のステップ (Next Steps)

-   本計画の承認の後、`/runplan 0075` でフェーズ 1〜4 を行い、実装の PR を作る。
-   実装の PR の説明には、AC-13 の比較の出力、ADR-0012 の訂正の段落、AC-09・AC-10 がマージの後の手順で確かめられることを書く。
-   実装の PR のマージの後、`/runpr` が #744 を閉じる。その後、本計画 §10 のマージの後の手順を、利用者が頼んだセッションで行う。

## 10. マージの後の手順（AC-02・AC-03・AC-09・AC-10）

この節は、どのコマンドも実行しない。チェックボックスにせず、PR の作成ポイントの印にもしない（設計書 6.3）。実装の PR がマージされ、`/runpr` が #744 を閉じた後に、利用者が頼んだセッションで、設計書 6.3 の流れ（図 4）と箇条のとおりに手で行う。要点は次のとおりである。

1.  `git fetch origin main` の後、確認のスクリプトを、実装の PR の squash のコミットで実行する。参照が残っているか、失敗したときは、ラベルに触れずに止まる。
2.  リポジトリを `isseis/yt2column` に固定し、1 回の GraphQL の問い合わせで `lightweight` ラベルと、それが付いた issue と PR を読む。止まる条件と「ラベルがない」の扱いは設計書 6.3 のとおりである。
3.  件数が 0 のときだけ、`AskUserQuestion` で削除と #744 へのコメントの承認を求める。
4.  承認の後、削除の直前に同じ問い合わせを読み直し、条件を満たすときだけ `gh label delete lightweight -R isseis/yt2column --yes` で削除し、読み直して `label` が `null` であることを確かめる。
5.  どの場合も、結果を日本語のコメントにしてファイルに書き、mention check を通してから、利用者に示して承認を得たうえで #744 に投稿する。
