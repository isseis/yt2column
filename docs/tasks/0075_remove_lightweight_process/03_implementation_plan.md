# 実装計画書：使われていない軽量プロセスの削除

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-09 |
| Review date | 2026-11-09 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、H-NN は [design_handoff.md](design_handoff.md) の項番を指す。節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指し、本計画自身の節は「本計画 §N」と書く。用語は、要件書 6 章の用語（軽量プロセス・通常の進め方・軽量プロセスへの参照・簡易な進め方）と、設計書の冒頭の用語（専用ファイル・実装の PR・削除のコミット・書き換えのコミット・確認のスクリプト・マージの後の手順）で定めた意味のまま使う。

既存のファイルの行番号は、コミット `ecefa1bdfa3958edf4a171ae7e0fae4d7474fb2d`（`main`）で確かめた。設計書が行番号を確かめたコミット `ccb06be5` からこのコミットまでに、`docs/tasks/` の外で変わったファイルは `docs/dev/adr/0040-simple-task-documents.md` だけである（`git log --oneline ccb06be5..ecefa1bd -- . ':!docs/tasks'` の出力は `489ababd` の 1 件）。ADR-0040 は本タスクで変えないので、設計書の行番号はそのまま使える。本タスクに `implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-06 は、どれも設計書の付録 B に扱いが記録されている。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

6 つの専用ファイルを削除し（F-001）、軽量プロセスへの参照を、今効いている指示・規約・コード・テストから取り除く（F-002）。通常の進め方のコマンドの振る舞いは変えない（F-003）。削除が済んだことは、恒久のテストにせず、タスクの `_checks/` の確認のスクリプトで確かめる（F-006）。GitHub の `lightweight` ラベルは、実装の PR のマージの後に、マージの後の手順で確かめてから削除する（F-005）。ADR の変更（F-004）は要件書の PR（#753）で済んでいる。

### 1.2. 実装の原則

-   **変えるファイルは設計書 2.2 の表のとおりにする。** Go の製品のコード、拡張、`Makefile`、CI の設定、既存のタスク文書、ADR-0012 の 2 つのリンクのほかの ADR は変えない。
-   **文面は実装で決める。** 本計画は、除く箇所と除いた後に満たすことを設計書 3.2〜3.5 に委ね、新しい文面を書かない。例外は、テストのコメントの言い直し（ステップ 3-5）で、変更の前後の全文を書く。
-   **新しい恒久のテストは足さない**（AC-12、設計書 7.1）。
-   **削除のコミットには削除だけを入れる**（設計書 3.6）。本計画の進み具合の更新も、削除のコミットには入れない。
-   **どのコミットでも `make test` と `make lint` が通る**（設計書 1.1）。

### 1.3. 計画の際に決めたこと（2026-11-09）

次の 2 点は、計画を書く前に利用者に尋ねて決めた。どちらも本タスクの中で閉じる決定なので、ADR は作らない。

-   **設計書 8 章の優先順位の 1〜4 を、それぞれ 1 つのフェーズにし、最後のフェーズの後に PR の作成ポイントを 1 つだけ置く。** PR は設計書 1.2 のとおり 1 本である。理由：`/runplan` はフェーズのまとまりごとに、サブエージェントのレビューとローカルの CLI レビューを行ってから次へ進む（`.claude/commands/runplan.md:162-226` の手順 7〜7.7）。削除のコミットは、メッセージの amend とカバレッジの比較の記録が絡み、誤ると作り直しになる。そのため、削除のコミットだけを 1 つのフェーズにして、書き換えより前にレビューを通す。採らなかった案：フェーズを 1 つにする案。レビューは 1 回で済むが、削除と書き換えを 1 回のレビューでまとめて読むことになる。
-   **確認のスクリプトの語の一覧と除外の一覧は、`_checks/data.json` に置く。** 語の一覧には日本語の語（`軽量`・`仕様のコメント`）が含まれる。Go の文字列リテラルは英語で書く規則で、`_checks/` は guard-test literal exception（`CLAUDE.md`「Go Idioms」）の対象ではない。タスク 0071 の `_checks/main.go` と `_checks/data.json` と同じ形にする。設計書 7.3 の「スクリプト自身の blob の ID を出力する」は、`main.go` と `data.json` の両方の blob の ID を出力することとして実装する。採らなかった案：Go のソースに `\u` のエスケープで書く案。ファイルは 1 つで済むが、語が読めなくなる。

次の点は、設計書と既存の仕組みから導いた。PR のレビューで異論があれば改める。どれも本タスクの中で閉じるので、ADR は作らない。

-   **書き換えのコミットは、設計書 3.2〜3.5 の節と、3.6 のコメントの言い直しごとに 1 つずつ、計 5 つにする**（ステップ 3-1〜3-5）。設計書 6.1 の 3 が、数を実装計画書に委ねている。節ごとに分ければ、レビューで各コミットを設計書の 1 つの節と並べて読める。
-   **確認のスクリプトの AC-11 の項目は、設計書付録 A の AC-11 の行のとおり、`Makefile` と `.github/workflows/` が、そのコミットで本タスクの `_checks/` を参照しないことを確かめるものにする。** 設計書 7.3 は見出しに AC-11 を挙げるが、表に項目の行がない。付録 A の確かめを確認のスクリプトに入れれば、PR の head とマージされた `main` の両方で実行される。`_checks/` が `go list ./...` の対象の外であることは、ディレクトリ名が `_` で始まることで決まる（設計書 7.3）。
-   **`data.json` も、引数のコミットから読む。** 設計書 7.3 の「作業ツリーは読まない」「結果は引数のコミットだけで決まる」を、語の一覧にも当てはめる。出力する blob の ID は、ビルドに使った `main.go` の blob の ID と、引数のコミットの `data.json` の blob の ID である。このため、壊して確かめる一時的なコミット（ステップ 4-2）は、`_checks/` をコミットした後のコミットから作る。

### 1.4. 既存コード調査結果

製品のコードの変更はない。変えるファイルと、変更に関わる既存の仕組みは次のとおりである（行番号は `ecefa1bd`）。

| 対象 | 今あるもの | 本タスクですること |
|---|---|---|
| 専用ファイル（6 つ） | 設計書の冒頭の一覧のとおり。下の検索の一致は計 243 行 | ステップ 2-1 で削除する |
| `docs/tasks/` と `docs/dev/adr/` の外の参照 | 下の検索が一致したファイルは、専用ファイルのほかに、次のとおりである（括弧内は一致した行数）。`_context.md`（7）、`_lib/local-cli-review.md`（1）、`_lib/task-pr-handoff.md`（17）、`mkreq.md`（42）、`mktrans.md`（1）、`runpr.md`（31）、`AGENTS.md`（1）、`CLAUDE.md`（25）、`codex_review_docs_test.go`（5）、`fixpr_docs_test.go`（3）、`requirements_process.md`（25）、`test_organization.md`（1）。`docs/tasks/0000_template/` には一致がない。設計書 2.2 の表と同じ集合である | ステップ 2-1・3-1〜3-5 で除く。`test_organization.md:14` の「Lightweight mocks」は軽量プロセスと関係がないので、変えずに除外の一覧に入れる |
| `cmd/yt2column/codex_review_docs_test.go` | `postingSections`（`:56-87`、10 個。軽量プロセスの 2 行は `:79-87`）、`readOnlyInvocations`（`:154-163`、軽量プロセスの 2 項目は `:161`・`:162`）。`TestPostingPathsCheckMention`（`:128`）と `TestPostingCommandsInventory`（`:213`）がこれらを使う | ステップ 2-1 で 2 行と 2 項目を消し、ステップ 3-5 でコメントを直す |
| `cmd/yt2column/fixpr_docs_test.go` | 定数 `runissuePath`（`:27`）、`TestContextAndCommandsAgree`（`:988`）の表の行（`:1004`）とコメント（`:983-987`） | ステップ 2-1 で定数と行を消し、ステップ 3-5 でコメントを直す |
| `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（`:140`） | ADR を含む指示の文書の相対リンクの先があることを確かめる。ADR-0012 の `:26`・`:43` のリンクが専用ファイルを指す | ステップ 1-1 でリンクを permalink に直す（設計書 3.7） |
| `cmd/yt2column/adr_test.go::TestADRRecords`（`:434`） | ADR の構造と索引を確かめる | 変えない。AC-08 の確かめに使う |
| `cmd/codexreview/docs_test.go` | `doctestutil.CommandDocs` で列挙したコマンドの文書の `--protocol` を確かめる。`runissue.md` は列挙から消えるだけ | 変えない |
| `make coverage-compare` | `Makefile:199`。BASE のコミットと HEAD でテストを走らせて比べる | ステップ 2-2 で使う。BASE では `TestLightweightCheck` が走るので、`bash` と `--attr-source` を受け付ける `git` が要る（手元の `git version 2.50.1` は受け付ける） |
| `_checks/` の先例 | `docs/tasks/0071_security_source_link/_checks/main.go` と `data.json`（`//go:build test`、`go build -tags test -o <パス> ./docs/tasks/.../_checks/` でビルドして実行、終了コード 0・1・2） | ステップ 4-1 で同じ形にならう。git の読み取りの関数は写して使ってよい |
| `git grep` の振る舞い | `git grep -h` で、`-i`・`-F`・`--no-color`・`--no-textconv` があることを確かめた。`ecefa1bd` で `git -c core.quotePath=false grep -n -i -F --no-color --no-textconv -e 'lightweight mocks' HEAD -- . ':(exclude)docs/tasks' ':(exclude)docs/dev/adr'` の出力は `HEAD:docs/dev/conventions/test_organization.md:14:    ├── mocks.go              # Lightweight mocks (no external dependencies)` の 1 行で、終了コードは 0 だった | 出力の形（`<コミット>:<パス>:<行番号>:<内容>`）は設計書 7.3 の前提どおりである |
| `.github/ISSUE_TEMPLATE/` | `accessibility.md` と `lightweight.md`。`config.yml` はない。このディレクトリを読む Go と TypeScript のコードはない（`git grep -n ISSUE_TEMPLATE -- '*.go' '*.ts'` の出力が空） | `lightweight.md` を削除するだけ |

上の表の「下の検索」は、`ecefa1bd` で実行した次の 2 つのコマンドである。1 つ目の出力を、ファイルごとの行数に集計した。2 つ目の出力は空だった。語の一覧は設計書 7.3 の一覧である。

```bash
git grep -n -i -E 'lightweight|runissue|軽量|LW_ISSUE|spec comment|仕様のコメント|spec-marker|spec marker|revert marker|issue-<|ISSUE_KEYWORD|closingIssuesReferences|attr-source' HEAD -- . ':!docs/tasks' ':!docs/dev/adr'
git grep -n -i -E 'lightweight|runissue|軽量' HEAD -- docs/tasks/0000_template
```

具体的な型で振る舞いを分ける設計はないので、構築の経路の列挙は要らない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: ADR-0012 のリンクの訂正（設計書 3.7）

**変更するファイル:** `docs/dev/adr/0012-lightweight-check-in-bash.md`、本計画（§7 の基点の記録）

-   [x] **ステップ 1-1**: 実装のブランチの基点（`git merge-base origin/main HEAD`）の完全な ID を求め、その ID のコミットに `.claude/commands/_lib/lightweight-check.sh` と `cmd/yt2column/lightweight_docs_test.go` があることを確かめる。ADR-0012 の `:26` と `:43` のリンクの先を、その ID を使った permalink（`https://github.com/isseis/yt2column/blob/<ID>/<パス>`）に直す。リンクの文、状態、経緯、索引は変えない。コミットのメッセージに、設計書 3.7 の形の訂正の行を、行頭から 1 行書く。基点の ID を本計画 §7 の冒頭の「実装のブランチの基点」の行に記録する。

**完了の条件:** ADR-0012 のほかの行が変わっていない。`make test` と `make lint` が通る。

### フェーズ 2: 削除のコミットとカバレッジの比較（設計書 3.6・7.2、AC-13）

**変更するファイル:** 設計書 3.6 の表と、その下の箇条に挙げたファイル。本計画（§7 の記録。削除のコミットとは別のコミット）

-   [-] **ステップ 2-0**: フェーズ 1 の `/runplan` の手順 7.7 が返した記録の行を、中身のないコミット（`git commit --allow-empty -F <file>`）のメッセージに書く。`Codex local review:` の行があれば、書く前に mention check を行う（`.claude/commands/runplan.md:226`）。これにより、削除のコミットのメッセージには、削除の説明と比較の出力だけを書けばよくなる。スキップした：フェーズ 1 の手順 7.7 は CodeRabbit CLI のレビューが指摘 0 件で、記録の行がなかったので、空のコミットは作らなかった。
-   [x] **ステップ 2-1（削除のコミット）**: 設計書 3.6 の削除を、1 つのコミットで行う。行や項目は、読んで確かめた文面のとおりに 1 つずつ消す（`.claude/commands/runplan.md` step 5）。
    -   [x] `cmd/yt2column/lightweight_docs_test.go` を削除する。
    -   [x] `internal/doctestutil/lightweight.go` を削除する。
    -   [x] `.claude/commands/runissue.md` を削除する。
    -   [x] `.claude/commands/_lib/lightweight-spec.md` を削除する。
    -   [x] `.claude/commands/_lib/lightweight-check.sh` を削除する。
    -   [x] `.github/ISSUE_TEMPLATE/lightweight.md` を削除する。
    -   [x] `codex_review_docs_test.go` の `postingSections` から「runissue.md revert」の行を消す。
    -   [x] `codex_review_docs_test.go` の `postingSections` から「mkreq.md lightweight triage」の行を消す。
    -   [x] `codex_review_docs_test.go` の `readOnlyInvocations` から `runissueLinkedPRsQuery` を消す。
    -   [x] `codex_review_docs_test.go` の `readOnlyInvocations` から `runpr.md` の step 2 の GraphQL の読み取り（`-F n=<number>` の項目）を消す。
    -   [x] `fixpr_docs_test.go` の定数 `runissuePath` を消す。
    -   [x] `fixpr_docs_test.go` の `TestContextAndCommandsAgree` の表から `runissue.md` の行を消す。
    -   [x] `.claude/commands/mkreq.md` の step 1 の「Lightweight triage」の項目（`:70-191`）を消す。
    -   [x] `.claude/commands/runpr.md` の分類「Lightweight PR」の項目（`:298-338`）を消す。

    `make fmt` が `fixpr_docs_test.go` の定数の桁をそろえ直す変更は、このコミットに含めてよい（設計書 3.6）。ほかの変更（コメントの言い直し、本計画の進み具合）は含めない。`make test` と `make lint` が通ることを確かめてからコミットする。
-   [x] **ステップ 2-2**: 削除のコミットを HEAD にしたまま、`make coverage-compare BASE=<削除のコミットの親>` を、コマンドの時間の上限を 10 分以上にするか、バックグラウンドで実行する。最後の行が `coverage-compare: PASS` であることを確かめる。`FAIL` か `STOP <kind>` のときは、先へ進まずに止まり、出力を示して利用者に尋ねる（設計書 7.2）。`PASS` のときは、出力を最後の行まで、削除のコミットのメッセージに足す。amend はメッセージだけを変え、tree の ID が変わらないことを amend の前後の `git rev-parse HEAD^{tree}` で確かめる。
-   [x] **ステップ 2-3**: 比較の出力、amend の後の削除のコミットの ID、その tree の ID を本計画 §7 の AC-13 の行に書き、本計画の進み具合とともにコミットする（削除のコミットとは別のコミット）。

**削除のコミットが誤っていたとき。** フェーズ 2 のレビュー（`/runplan` の手順 7〜7.7）が、削除のコミットの中身の誤り（消し漏れ以外の誤り。削除でない変更や消しすぎ）を見つけたときは、まだ push していないブランチを削除のコミットの親まで戻し（`git reset --hard <親>`。戻す前に、捨てるコミットの ID を控える）、ステップ 2-1〜2-3 をやり直す。

消し漏れだけのときは、削除のコミットを作り直さず、そのテストと文書の部分だけを消す 2 つ目の削除のコミットを作り、ステップ 2-2・2-3 と同じく比較して記録する（設計書 6.2 の 3 つ目の箇条と同じ形）。レビューの修正で、削除のコミットの中身をほかの方法で変えない。

**完了の条件:** 削除のコミットの差分が、ファイルの削除と行の削除（と `make fmt` の桁のそろえ直し）だけである。次の 3 点で確かめ、レビューでも差分を読む。(1) `git show --numstat --format= <削除のコミット>` で、`fixpr_docs_test.go` 以外のファイルに足した行が 0 である。(2) `fixpr_docs_test.go` に足した行が定数の並びのそろえ直しだけである（`git show -U0` で読む）。(3) `git show --name-status --format= <削除のコミット>` の状態が `D` か `M` だけである。比較が `PASS` で、その出力が削除のコミットのメッセージと本計画 §7 にある。

### フェーズ 3: 書き換えのコミット（設計書 3.2〜3.6）

**変更するファイル:** `.claude/commands/mkreq.md`、`.claude/commands/runpr.md`、`.claude/commands/_lib/task-pr-handoff.md`、`.claude/commands/_context.md`、`CLAUDE.md`、`AGENTS.md`、`docs/dev/conventions/requirements_process.md`、`.claude/commands/mktrans.md`、`.claude/commands/_lib/local-cli-review.md`、`cmd/yt2column/codex_review_docs_test.go`、`cmd/yt2column/fixpr_docs_test.go`、本計画（進み具合）

各ステップを 1 つのコミットにする。どのステップも、テストの関数・表の行・サブテストを消さない（設計書 6.1 の 3）。各コミットの前に `make test` と `make lint` を実行する。本フェーズで変える文書のうち、テストが見出し・項目の書き出し・表の行で読む文書（`runpr.md`・`_lib/task-pr-handoff.md`・`mkreq.md`・`_context.md`・`AGENTS.md`・`CLAUDE.md`）では、テストが読む目印と値を変えない。

-   [ ] **ステップ 3-1**: `mkreq.md` から、設計書 3.2 の「除くもの」のうち、ステップ 2-1 で消した step 1 の項目以外を、すべて取り除く（`description`、冒頭の引用の文、紹介の段落、「Placeholders」の段落、「Authorization」の承認の文、`gh issue view` の `--json` の `labels`、step 9 の報告の 2 つの文）。step 1 の残りの 3 つの確認と、step 2 へ進む流れは変えない（AC-05）。
-   [ ] **ステップ 3-2**: `runpr.md` から、設計書 3.3 の表の箇所のうち、ステップ 2-1 で消した分類の項目以外を取り除き、文の組み立てを直す。設計書 3.3 の箇条（Standalone change の定義は「anything else」のままにする；`fixprguard` のための `gh repo view` と `gh api graphql` の承認と `gh api user` は残す；step 5 の `gh issue close` の前の mention check は残す）を守る（AC-06・AC-07）。
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
        // write nothing: the link count query of _lib/task-pr-handoff.md. The query
        // follows on later lines, so the code block it opens must hold no mutation.
        ```

        に変える。
    -   `fixpr_docs_test.go` の `TestContextAndCommandsAgree` のコメントの 4〜5 行目を、`// mention fixprguard rejects, and fixpr.md, runpr.md, runissue.md, mkreq.md,` と `// and _lib/task-pr-handoff.md never write a reviewer's name.` から、`// mention fixprguard rejects, and fixpr.md, runpr.md, mkreq.md, and` と `// _lib/task-pr-handoff.md never write a reviewer's name.` に変える。

    `make fmt` を実行する。

**完了の条件:** 5 つの書き換えのコミットがこの順にあり、どれもテストを消していない。`make test` と `make lint` が通る。

### フェーズ 4: 確認のスクリプトと最後の確かめ（設計書 7.1・7.3、AC-01〜AC-04・AC-11）

**変更するファイル:** `docs/tasks/0075_remove_lightweight_process/_checks/main.go`（新規）、`docs/tasks/0075_remove_lightweight_process/_checks/data.json`（新規）、本計画（§7 の記録）

-   [ ] **ステップ 4-1**: 確認のスクリプトを `_checks/main.go` と `_checks/data.json` に書く（本計画 §1.3 の形）。確かめる項目は、設計書 7.3 の表の 2 行（AC-01、AC-02・AC-03）と、本計画 §1.3 の AC-11 の項目である。スクリプトには、設計書 7.3 の次の性質を持たせる。
    -   コミットを 1 つ引数で受け取り、完全な ID に解決する。
    -   作業ツリーの対象のファイルを読まない。
    -   git の設定の影響を除く。
    -   出力の行を読めなければ失敗する。
    -   `git grep` の終了コードを区別する。
    -   `docs/tasks/0000_template/` を範囲に戻す。
    -   除外の各項目は、パスと行の内容の全体の組で書き、ちょうど 1 回当たることを求める。
    -   違反と読み取りの失敗では、0 でない終了コードで終わる。
    -   違反のファイルと行番号と内容を出力する。
    -   確かめたコミットの完全な ID と、`main.go`・`data.json` の blob の ID を出力する。

    コメント・識別子・文字列リテラルは英語で書く。`make fmt` と `go vet -tags test ./docs/tasks/0075_remove_lightweight_process/_checks/` を通してから、`_checks/` の 2 つのファイルをコミットする。ビルドは、リポジトリの根で `go build -tags test -o <スクラッチパッドの中のパス> ./docs/tasks/0075_remove_lightweight_process/_checks/` とする（タスク 0071 と同じ）。出力する `main.go` の blob の ID は、ビルドに使った作業ツリーの `main.go` から求める。
-   [ ] **ステップ 4-2**: 確認のスクリプトが、狙った理由で失敗できることを確かめる。ステップ 4-1 のコミットから `git worktree` で使い捨てのブランチを作り、次の壊し方を 1 件ずつ別の一時的なコミットとして作る。それぞれのコミットを引数にして、ビルドした実行ファイルを走らせ、狙った項目が失敗することと、終了コード（違反は 1、読み取りの失敗は 2。タスク 0071 の `_checks/` と同じ）を確かめる。
    -   AC-01（違反）：専用ファイルの 1 つを戻す。
    -   AC-02（違反）：残すファイルの 1 行に、大文字の英語の語（`RUNISSUE`）を足す。別の一時的なコミットで、日本語の語（`軽量`）を足す。
    -   AC-02 の範囲（違反）：`docs/tasks/0000_template/` のファイルに語の一覧の語を足す。
    -   除外（違反）：`test_organization.md` の「Lightweight mocks」の行を変える（除外が 0 回当たる）。別の一時的なコミットで、その行を同じファイルに複製する（除外が 2 回当たる）。
    -   AC-11（違反）：`Makefile` に本タスクの `_checks/` のパスを足す。別の一時的なコミットで、`.github/workflows/` のファイルに同じパスを足す。
    -   出力の行を読めない（読み取りの失敗）：語の一覧の語を含むバイナリのファイルを、残すファイルの場所に足す（`Binary file … matches` の行になる）。
    -   読み取りの失敗：存在しないコミットを引数にする。

    加えて、`docs/tasks/` の別のタスク文書と `docs/dev/adr/` の ADR に語を足しても合格すること（終了コード 0）を、タスク文書と ADR のそれぞれについて確かめる。`git grep` が 2 以上の終了コードで終わる場合は、壊して再現できないので、PR のレビューでコードを読む。狙った項目以外が失敗するときは、壊し方を直す。使い捨てのブランチは push せず、確かめた後に worktree とともに消す。壊し方ごとの結果は、ステップ 4-3 のコミットのメッセージに書く。
-   [ ] **ステップ 4-3**: PR の head で、確認のスクリプト、`make test`、`make lint`、`make deadcode` を実行する（AC-01〜AC-04・AC-11）。確認のスクリプトの出力と終了コード、確かめたコミットの完全な ID、3 つの `make` の結果を本計画 §7 に書き、本計画の進み具合とともにコミットする。このコミットは本タスクのディレクトリの下しか変えないので、記録した結果はそのまま head に当てはまる（設計書 6.1 の 5）。

**PR の作成ポイントでの確かめ直し（以下、作成ポイントの確かめ直し）。** フェーズ 4 の後の `/runplan` のレビュー（手順 6〜7.7）の修正が入った後、手順 5a でグリーンゲートを確かめる前に、次を行う。`/runplan` がこれを実装のまとまりとして拾わないよう、チェックボックスにはしない。

1.  `git fetch origin main` の後で、`origin/main` を取り込む必要があるかを確かめる。取り込むときは、設計書 6.2 のとおり merge で行う（rebase しない）。取り込みでテストの削除が新たに要るときは、設計書 6.2 の 3 つ目の箇条のとおり、もう 1 つの削除のコミットを作り、ステップ 2-2・2-3 と同じく比較して記録する。
2.  ステップ 4-3 の後に、本タスクのディレクトリの外を変えるコミット（レビューの修正や取り込み）が入ったときは、確認のスクリプトと 3 つの `make` を実行し直し、本計画 §7 の記録を改める。`_checks/` を変えるコミットが入ったときは、ステップ 4-2 もやり直す。
3.  記録の変更は、手順 5a のグリーンゲートのチェックボックスのコミットに含める。このコミットは、手順 7.7 の記録の行も運ぶ（`.claude/commands/runplan.md:226`）。
4.  `_lib/task-pr-handoff.md` Part B の手順 1・2 の修正でコミットが増え、本タスクのディレクトリの外が変わったときは、上の 2 をやり直してから PR を作る。

**完了の条件:** 確認のスクリプトが終了コード 0 で終わり、その記録が本計画 §7 にある。壊し方ごとの結果が、ステップ 4-3 のコミットのメッセージにある。`make test`・`make lint`・`make deadcode` が通る。

### PR-1 作成ポイント: remove the lightweight process

**対象ステップ**: 1-1 / 2-0 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 4-1 / 4-2 / 4-3

**推奨タイトル**: `chore(0075): 使われていない軽量プロセスを削除する`

**レビュー観点**: 削除のコミットが削除だけで、`make coverage-compare` が `PASS`、amend の前後で tree の ID が同じ / 書き換えた `mkreq.md` と `runpr.md` で通常の進め方の振る舞いが変わらず、残る投稿の箇所を `TestPostingPathsCheckMention` と `TestPostingCommandsInventory` が守る（AC-05〜AC-07） / 確認のスクリプトが違反と読み取りの失敗を区別し、除外がちょうど 1 回当たることを確かめる / ADR-0012 の変更がリンクの先だけで、訂正の行がある

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 4-1 が、除外がちょうど 1 回当たることの確かめと終了コード 1・2 の区別を持つ新しい確認のスクリプトを作り、ステップ 3-2・3-3 が投稿の守りを確かめるテスト（AC-07）が読むコマンドの文書を書き換える。削除のコミット（ステップ 2-1〜2-2）は、PR ではなくフェーズとして切り離している。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | ステップ |
|---|---|---|
| M1: リンクの訂正 | 訂正の行つきの ADR-0012 のコミット | 1-1 |
| M2: 削除 | 削除のコミット（比較の出力つき）と本計画 §7 の記録 | 2-0〜2-3 |
| M3: 書き換え | 5 つの書き換えのコミット | 3-1〜3-5 |
| M4: 確認 | 確認のスクリプトと、PR の head での実行の記録 | 4-1〜4-3、作成ポイントの確かめ直し |
| M5: マージの後の手順 | #744 への結果のコメント | 本計画 §10 |

各マイルストーンは前のマイルストーンを前提とする（設計書 6.1）。M4 の後に `/runplan` が PR を作る。M5 は `/runpr` が実装の PR をマージした後に、手で行う。

### 3.2 PR 構成

PR は 1 本である（設計書 1.2）。フェーズ 1〜4 の後に、PR-1 の作成ポイントを置く。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 2-0〜2-3 / 3-1〜3-5 / 4-1〜4-3 | ADR-0012 のリンクの訂正、専用ファイルとテストの削除、参照の書き換え、確認のスクリプト | frontier-recommended |

## 4. テスト戦略 (Test Strategy)

### 4.1. 単体テスト

新しいテストは足さない（AC-12）。各コミットで `make test` と `make lint` を、PR の head で `make deadcode` も実行する（AC-04）。残るテストが確かめることは設計書 7.1 のとおりである。削除のコミットで消すテストは、ステップ 2-2 の比較で確かめる（AC-13）。

### 4.2. 確認のスクリプト

AC-01〜AC-03 と AC-11 の一部を、確認のスクリプトで、PR の head（ステップ 4-3 と作成ポイントの確かめ直し）とマージされた `main`（本計画 §10）で確かめる。確認のスクリプトが違反と読み取りの失敗を検出できることは、ステップ 4-2 で項目ごとに壊して確かめる。

### 4.3. 読んで確かめる

AC-02・AC-05・AC-06・AC-12 は、PR のレビューで読む（設計書 7.4）。

### 4.4. 結合テスト・後方互換のテスト

N/A。実行時の振る舞いを変えない（設計書 7.5）。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 削除のコミットに削除でない変更が混ざる | AC-13 の比較が削除だけを確かめたことにならない | ステップ 2-1 で項目ごとに消し、本計画の進み具合は別のコミットにする。フェーズ 2 の後のレビューで差分を読む |
| 削除のコミットの後に別のコミットを作ってから amend しようとする | メッセージだけの amend ができず、比較の記録の ID がずれる | ステップ 2-2 を、削除のコミットの直後に行う。tree の ID を amend の前後で比べる |
| フェーズ 2 のレビューが、削除のコミットの中身の誤りを見つける | 修正のコミットでは削除のコミットを直せず、記録した比較が誤った中身を指す | フェーズ 2 の「削除のコミットが誤っていたとき」に従い、作り直すか 2 つ目の削除のコミットにする |
| 親のコミットで `TestLightweightCheck` が `bash` や `git` の不足で失敗する | 比較が `STOP` で終わる | 手元の `git` が `--attr-source` を受け付けることを本計画 §1.4 で確かめた。`STOP` のときは止まって利用者に尋ねる（ステップ 2-2） |
| 書き換えで、テストが節を探す目印の文字列を変える | `make test` が失敗する | フェーズ 3 の冒頭の注意を守り、各コミットの前に `make test` を実行する |
| タスク 0072・0073・0074 の実装が先にマージされ、衝突や新しい参照が入る | AC-03 を満たさない | 作成ポイントの確かめ直しで取り込み、確認のスクリプトで新しい参照を見つける（設計書 6.2）。ADR-0012 の衝突は `docs/dev/conventions/adr.md`「Taking in `main`」に従って解き、直すのはリンクの先だけとする（設計書 3.7）。0074 が先にマージされたときは、取り込みの後に `adr.md`「Editorial corrections」を読み直し、ステップ 1-1 の訂正の記録の形が変わっていれば、それに合わせる |
| 最後の確認とマージの間に、ほかの PR が参照を `main` に入れる | マージされた `main` で AC-02 を満たさない | 本計画 §10 で、マージされた `main` を確かめ直す |
| 語の一覧に当たらない言い換えの参照が残る | AC-02 を満たさない | PR のレビューで読む（設計書 7.3 の「語の判断の限界」） |
| 条件を満たさないのにラベルを消す | 元に戻せない | 本計画 §10 の手順（設計書 6.3）で、件数の確かめ、承認、削除の直前の読み直しを行う |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 2-0 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 4-1 / 4-2 / 4-3）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

確認のスクリプトは `docs/tasks/0075_remove_lightweight_process/_checks/`（`main.go` と `data.json`。以下、`_checks`）である。実行の記録（日付、確かめたコミットの完全な ID、`main.go` と `data.json` の blob の ID、出力、終了コード）は、ステップ 4-3 と作成ポイントの確かめ直しで各行に書く。

実装のブランチの基点：`dcc4edf34c99d8aecd47c81710420fb54b0a1b29`（`git merge-base origin/main HEAD`。このコミットに `.claude/commands/_lib/lightweight-check.sh` と `cmd/yt2column/lightweight_docs_test.go` があることを `git ls-tree` で確かめた）

AC-01〜AC-03 は、文書に語がないことを `static`（確認のスクリプトの `git grep`）で確かめる。これは `requirements_process.md`「4. Acceptance Tests」の例外で、要件書 1 章（issue #744 の決定）と設計書付録 A が定める。語が軽量プロセスを指すかの判断は、AC-02 の `manual` で読む。

| AC | 種類 | 検証 | ステップ |
|---|---|---|---|
| AC-01 | static | `_checks`（6 つの専用ファイルが、確かめたコミットにない）。PR の head で実行する。マージされた `main` での実行は本計画 §10 で行い、#744 に記録する | 2-1・4-1〜4-3、作成ポイントの確かめ直し |
| AC-02 | static | `_checks`（`docs/tasks/`（`0000_template/` を除く）と `docs/dev/adr/` の外で、語の一覧に当たる行が除外のほかにない）。実行の時点は AC-01 と同じ | 2-1・3-1〜3-5・4-1〜4-3、作成ポイントの確かめ直し |
| AC-02 | manual | 書き換えた文書が軽量プロセスへの参照を持たず、文として成り立っていること、除外の行が軽量プロセスと関係がないことを、PR のレビューで読む（設計書 7.4） | 3-1〜3-4 |
| AC-03 | static | `_checks` を、`origin/main` を取り込んだ後の head と、マージされた `main` で実行する | 作成ポイントの確かめ直し、本計画 §10 |
| AC-04 | test | `make test`・`make lint`・`make deadcode`（PR の head）。`TestLightweightCheck` が残らないことは、AC-02 の `_checks` が `lightweight` の語で確かめる | 4-3、作成ポイントの確かめ直し |
| AC-05 | manual | `mkreq.md` の step 1 の残りの確認と、step 2 へ進む流れを、PR のレビューで読む（設計書 7.4） | 2-1・3-1 |
| AC-06 | manual | `runpr.md` の分類と、Standalone change の PR を作る手順を、PR のレビューで読む（設計書 7.4） | 2-1・3-2 |
| AC-07 | test | `cmd/yt2column/codex_review_docs_test.go::TestPostingPathsCheckMention`、`cmd/yt2column/codex_review_docs_test.go::TestPostingCommandsInventory`（`make test` で実行） | 2-1・3-1〜3-3 |
| AC-08 | test | `cmd/yt2column/adr_test.go::TestADRRecords`（`make test` で実行）。ADR の変更は #753 で済んでいる | 1-1・4-3 |
| AC-09 | （マージの後） | 本計画 §10 の手順で確かめ、#744 のコメントに記録する。PR を作る時点では確かめられない既知の不足である（設計書付録 A） | 本計画 §10 |
| AC-10 | （マージの後） | 同上 | 本計画 §10 |
| AC-11 | static | `_checks` の AC-11 の項目（`Makefile` と `.github/workflows/` が本タスクの `_checks/` を参照しない）。`go list ./...` の対象の外であることは、ディレクトリ名による | 4-1・4-3 |
| AC-12 | manual | 実装の PR の差分が、テストを足していないことを、PR のレビューで読む（設計書 7.4） | 全ステップ |
| AC-13 | test | `make coverage-compare BASE=<削除のコミットの親>` の出力（最後の行が `coverage-compare: PASS`）。削除のコミットのメッセージ、PR の説明、この行に記録する。記録（2026-11-09）：削除のコミット `d661acdfbf946b8183141b1236e348797d2860e7`（amend の前は `d8b2bc557732bdd43534f2251c1a900aea92ea4c`。tree `23a69b547facdf9bf2450a98c067f765723078b6` は amend の前後で同じ）、親 `c9565dc5314d4d5d8a37a2ca290ee417159113f8`。出力：`target make coverage-compare BASE="c9565dc5314d4d5d8a37a2ca290ee417159113f8"` / `base c9565dc5314d4d5d8a37a2ca290ee417159113f8` / `head d8b2bc557732bdd43534f2251c1a900aea92ea4c tree 23a69b547facdf9bf2450a98c067f765723078b6` / `toolchain go1.27.1` / `tests base 1294 head 1286` / `files base 139 head 139` / `functions base 1312 head 1312` / `deleted 0` / `no compared function lost coverage` / `coverage-compare: PASS` | 2-2・2-3 |

## 8. 成功の基準 (Success Criteria)

-   **機能の完全性:** 専用ファイルがなく、`docs/tasks/` と `docs/dev/adr/` の外に軽量プロセスへの参照がない。本計画 §7 の AC-09・AC-10 以外の検証が済んでいる。
-   **品質:** `make test`・`make lint`・`make deadcode` が通る。`_checks` が終了コード 0 で終わり、項目ごとに壊して失敗することを確かめた記録が、ステップ 4-3 のコミットのメッセージにある。削除のコミットの比較が `PASS` である。
-   **セキュリティ:** 残る投稿の箇所の守り（mention check、`fixprguard`）が今と同じである（AC-07）。
-   **文書:** 変えたファイルが設計書 2.2 の表のとおりである。

## 9. 次のステップ (Next Steps)

-   本計画の承認の後、`/runplan 0075` でフェーズ 1〜4 を行い、PR-1 の作成ポイントで実装の PR を作る。
-   実装の PR の説明には、AC-13 の比較の出力、ADR-0012 の訂正の段落、AC-09・AC-10 がマージの後の手順で確かめられることを書く。
-   実装の PR のマージの後、`/runpr` が #744 を閉じる。その後、本計画 §10 のマージの後の手順を、利用者が頼んだセッションで行う。

## 10. マージの後の手順（AC-02・AC-03・AC-09・AC-10）

この節の手順は、`/runplan` を含め、どのコマンドも自動では実行しない。チェックボックスにせず、PR の作成ポイントの印にもしない（設計書 6.3）。実装の PR がマージされ、`/runpr` が #744 を閉じた後に、利用者が頼んだセッションで、設計書 6.3 の流れ（図 4）と箇条のとおりに手で行う。要点は次のとおりである。

1.  `git fetch origin main` の後、実装の PR の squash のコミット（`gh pr view <PR> --json mergeCommit` の ID）を、使い捨ての worktree に取り出す（`git worktree add --detach <スクラッチパッドの中のパス> <ID>`）。その worktree の根で、ステップ 4-1 のコマンドで確認のスクリプトをビルドし、その ID を引数にして実行する。出力の `main.go` と `data.json` の blob の ID が本計画 §7 の記録と異なるとき、参照が残っているとき、またはスクリプトが失敗したときは、ラベルに触れずに止まる。確かめた後に worktree を消す。
2.  リポジトリを `isseis/yt2column` に固定し、1 回の GraphQL の問い合わせで `lightweight` ラベルと、それが付いた issue と PR を読む。止まる条件と「ラベルがない」の扱いは設計書 6.3 のとおりである。
3.  件数が 0 のときだけ、`AskUserQuestion` で削除と #744 へのコメントの承認を求める。
4.  承認の後、削除の直前に同じ問い合わせを読み直し、条件を満たすときだけ `gh label delete lightweight -R isseis/yt2column --yes` で削除し、読み直して `label` が `null` であることを確かめる。
5.  どの場合も、結果を日本語のコメントにしてファイルに書き、mention check を通してから #744 に投稿する。削除の場合のコメントは、3 の承認に含まれている。止まった場合と「ラベルがない」場合のコメントは、投稿の前に利用者に示して承認を得る（設計書 6.3 の「承認の範囲」）。

## 変更履歴

- 2026-11-09 承認（isseis、#762）
