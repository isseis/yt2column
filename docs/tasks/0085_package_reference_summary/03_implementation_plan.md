# 実装計画書：package_reference.md の行を責務の要約に絞る

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-11 |
| Review date | 2026-11-11 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

パッケージの参照表（`docs/dev/conventions/package_reference.md`）の冒頭に粒度の規則を書き、表を 6 つに分け、全行を責務の要約に絞る。行から外した詳細は、リンク・doc コメント・削除のどれかで扱う。ADR の参照と、関係する 2 つの文書の 1 行と、strictjson の利用者の一覧を読むテストを直す。ADR-0044 の作成と issue #817 の本文の書き足しは、3 文書の PR で行うので、本計画の対象ではない。

### 1.2. 実装の原則

-   **コードの振る舞いを変えない。** Go のファイルで変えるのは、既存のファイルの doc コメントと、`internal/pipeline/strictjson_users_test.go` だけである（AC-12）。
-   **文書の参照は、それを古くする PR で直す**（設計書 1 章）。
-   **新しい恒久のテストは足さない**（設計書 7 章）。

### 1.3. 計画の際に決めたこと（2026-11-11）

-   **実装の PR を 2 本に分ける。PR-1 は、テストの直し・冒頭の規則・表の構成と、パイプラインの構成要素・LLM・共有部品の行を扱う。PR-2 は、サーバとホスト・開発ツール・テスト専用の行と、仕上げを扱う。** 行から外す文は合わせて約 60KB ある。外した詳細がほかの場所にあるか、消してよいかをレビューで確かめる必要があり、1 本あたりの量を減らすためである。
    -   採らなかった案：1 本の PR にする案。途中の状態はなくなるが、1 本のレビューの量が大きい。
    -   ADR：なし。本タスクの中で閉じる計画上の選択である。
-   **各実装の PR の説明に、絞った行ごとに、外した文か節のすべてと、その行き先の一覧を載せる（AC-13）。** レビュアーが、外した詳細が失われていないかを、行の削除と照らして確かめられるようにするためである。一覧は PR の説明に置き、タスク文書には残さない。
    -   採らなかった案：一覧を作らず、レビュアーが差分を読んで判断する案。
    -   ADR：なし。本タスクの中で閉じる。
-   **確かめるスクリプトは 2 つとも、シェルで書く。** どちらも `git` と `grep`・`awk` で足りるためである。
    -   ADR：なし。本タスクの中で閉じる。

### 1.4. 既存コード調査結果

コミット `04dc4824145e2243bbf8dcfa82c5ff853c43eb11` で確かめた。

-   パッケージの参照表は 75,604 バイトで、表の行は 56 行ある。表は 1 つで、`## ` の見出しはない。
-   `TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:629`）は、`packageReferenceRows`（同 `:672`）で、`|` で始まるすべての行の最初のコードスパンを集める。表を分けても、`## ` の見出しの行（`|` で始まらない）と、コードスパンを持たない表のヘッダー行は数えない。
-   `TestPackageReferenceStrictjsonUsers`（`internal/pipeline/strictjson_users_test.go:609`）は、`strictjsonRowCell`（同 `:196`）で、`doctestutil.FindTable`（`internal/doctestutil/tables.go:61-75`）が返す最初の `Package` の表だけを読む。表を分けると、共有部品の表にある `internal/strictjson` の行を見つけられない。`strictjsonRowCell` の単体テストは `TestStrictjsonRowCell`（`internal/pipeline/strictjson_users_test.go:417`）にある。一覧の後ろの文は `parseUserList`（同 `:230`）が拒否する。
-   ADR がパッケージの参照表を参照する文は 19 行ある（`git grep -n package_reference -- docs/dev/adr/`）。直すのは 7 か所で、内訳は設計書 3.3 節の表にある。
-   `Value.AsNonNegativeInt64` の doc コメント（`internal/strictjson/strictjson.go:262-266`）は、受理する構文と、エラーが値を含まないことをすでに述べている。`internal/artifact` の doc コメント（`internal/artifact/artifact.go:19-22`）は、版 1〜4 の形式の違いを述べている。
-   `docs/dev/guides/pricing_update.md:39` と `CLAUDE.md:18` が、直す 1 行である。
-   タスク 0083（Slack Webhook の削除）が、`internal/slackwebhook` の行とパッケージの参照表を変える予定である（`docs/tasks/0083_remove_slack_webhook/01_requirements.md:83`）。どちらが先に `main` に入っても、後の PR は、そのときの `main` の表から作業を始める。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: テストの直し・冒頭の規則・表の構成と、前半の 3 つの表（設計書 3 章、AC-01〜AC-08・AC-10〜AC-15・AC-17〜AC-21）

**変更するファイル:** 設計書 3 章の表のうち PR-1 の行と、本計画

-   [ ] **ステップ 1-1**: 確かめるスクリプトを 2 つ、`docs/tasks/0085_package_reference_summary/_checks/check_state.sh` と `_checks/check_diff.sh` に書く（設計書 7 章）。コメントと出力の文字列は英語で書く。
    -   `check_state.sh`：パッケージの参照表を読み、`## ` の見出しが 6 つで各見出しの下に表が 1 つずつあること（AC-05）と、どのパッケージ名もちょうど 1 行に載ること（AC-06）を確かめる。文書のバイト数を出力し、上限の判定（AC-09）は引数で指定したときだけ行う。
    -   `check_diff.sh`：マージベースと PR のコミットを引数で受け取る。変えたファイルが AC-20 の範囲にあり Go のファイルの追加がないことと、Go のファイルの差分がコメントの行に限ること（AC-12）を確かめる。

    どちらも、違反があれば 0 でない終了コードで終わり、違反の内容を出力する。各確認が、少なくとも 1 つの壊し方で失敗することを確かめ、その結果をこのステップのコミットのメッセージに書く。
-   [ ] **ステップ 1-2**: `strictjsonRowCell` を、`Package` の列を持つすべての表から行を集める形に直す（設計書 3.5 節、AC-21）。`TestStrictjsonRowCell` に、行が 2 つ目の表にある場合と、別々の表に 1 つずつある場合を足す。直しを一時的に戻すと、足した場合が失敗することを確かめ、その結果をコミットのメッセージに書く。
-   [ ] **ステップ 1-3**: 冒頭を書き換える（設計書 3.1 節、AC-01〜AC-04）。行に書くもの・書かないもの、詳細の置き場所、6 つの種類の定義（箇条書き）を書く。今の更新の規則と `project_overview.md` へのリンクは保つ。
-   [ ] **ステップ 1-4**: 6 つの節を作り、全行を設計書 3.1 節の割り当てに従って移す（AC-05・AC-06）。この時点では、行の内容は変えない。表の中は名前順に並べる。`make test` が通ることを確かめる。
-   [ ] **ステップ 1-5**: パイプラインの構成要素の行を絞る（設計書 3.2・3.4 節、AC-07・AC-10・AC-11）。外した文か節と、その行き先を、行ごとに PR の説明の一覧の材料として控える（AC-13）。
-   [ ] **ステップ 1-6**: LLM の行を絞る。設計書 3.2 節の表で「残す内容」とした番兵の一覧・使用量の型・パラメータの型の置き場所と依存は、行に残す。
-   [ ] **ステップ 1-7**: 共有部品の行を絞る。`internal/strictjson` の欄は、`Shared by` の一覧で終える（AC-08）。
-   [ ] **ステップ 1-8**: 設計書 3.3 節の PR-1 の ADR の参照 5 か所を、編集上の訂正として直す（AC-14・AC-15）。直した後の参照先が doc コメントのものは、その内容が doc コメントにあることを確かめ、なければ doc コメントへ移す。コミットメッセージに、ADR ごとの訂正の行を書く（`docs/dev/conventions/requirements_process.md`「Recording in the PR description」）。
-   [ ] **ステップ 1-9**: ADR-0044 のずれの記述を、残るずれ（後半の 3 つの表の行がまだ規則に従っていないこと）だけを述べる形に書き換える（AC-17）。編集上の訂正（ずれの記述の書き換え）として記録する。PR を作る前に、`git grep -nE 'issues/817([^0-9]|$)' origin/main -- docs/dev/adr/` で、ほかに書き換えるずれの記述がないことを確かめる（`docs/dev/conventions/adr.md`「Discrepancy statements」）。
-   [ ] **ステップ 1-10**: `CLAUDE.md:18` の説明文（AC-18）と、`docs/dev/guides/pricing_update.md:39` の 1 文（AC-19）を、新しい冒頭と行の中身に合わせて直す。
-   [ ] **ステップ 1-11**: 2 つのスクリプトを PR のブランチで走らせ（`check_state.sh` は上限の判定なし）、出力と 2 つのコミットの完全な ID を 7 章 に書く。AC-15・AC-18・AC-19 の diff の記録は、`/runplan` が 7 章 の記録の形で書く。`make test` と `make lint` を通す。

**完了の条件:** 2 つのスクリプトが終了コード 0 で終わり、7 章 にその記録と diff の記録がある。`make test` と `make lint` が通る。

### PR-1 作成ポイント: test fix, front matter, table split, and pipeline, LLM, and shared rows

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11

**推奨タイトル**: `docs(0085): package_reference.md の規則と表の構成を変え、前半の 3 つの表の行を絞る`

**レビュー観点**: 外した文か節のすべてが、PR の説明の一覧でリンク・doc コメント・削除のどれかに当たり、リンク先が今の挙動を述べているか / ADR が行に残すとした内容が行に残っているか / ADR の参照の訂正が決定を変えていないか / strictjson のテストの直しが、表をまたいだ重複と欠けを見つけるか

**実装モデル要件**: standard

**判定理由**: 文書の書き直しと doc コメントの追加、テストの探し方の小さな直しだけで、前例のない設計判断・panel の条件・競合する実装の方法のどれにも当たらない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 後半の 3 つの表と仕上げ（設計書 3 章、AC-05〜AC-07・AC-09〜AC-15・AC-17・AC-20）

**変更するファイル:** 設計書 3 章の表のうち PR-2 の行と、本計画

-   [ ] **ステップ 2-1**: サーバとホストの行を絞る（設計書 3.2・3.4 節、AC-07・AC-10・AC-11）。外した文か節と、その行き先を、PR の説明の一覧の材料として控える（AC-13）。
-   [ ] **ステップ 2-2**: 開発ツールの行を絞る。秘密の扱いと子プロセスの環境変数に関わる詳細は、設計書 3.4 節のとおり、削るだけにしない。
-   [ ] **ステップ 2-3**: テスト専用の行を絞る。
-   [ ] **ステップ 2-4**: 設計書 3.3 節の PR-2 の ADR の参照 2 か所（ADR-0027）を、編集上の訂正として直す（AC-14・AC-15）。コミットメッセージに訂正の行を書く。
-   [ ] **ステップ 2-5**: ADR-0044 のずれの記述を消す（AC-17）。編集上の訂正（ずれの記述の削除）として記録する。PR を作る前に、`git grep -nE 'issues/817([^0-9]|$)' origin/main -- docs/dev/adr/` で、ほかに消すずれの記述がないことを確かめる。
-   [ ] **ステップ 2-6**: `check_state.sh` を上限の判定つき（15,000 バイト）で、`check_diff.sh` を PR-2 のマージベースと PR のコミットで走らせる。出力と 2 つのコミットの完全な ID を 7 章 に書く。AC-15 の diff の記録は、`/runplan` が 7 章 の記録の形で書く。`make test` と `make lint` を通す。

**完了の条件:** 2 つのスクリプトが終了コード 0 で終わり、7 章 にその記録と diff の記録がある。`make test` と `make lint` が通る。

### PR-2 作成ポイント: server, host, dev tool, and test-only rows and wrap-up

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6

**推奨タイトル**: `docs(0085): package_reference.md の後半の 3 つの表の行を絞り、ADR-0044 のずれの記述を消す`

**レビュー観点**: 外した文か節のすべてが、PR の説明の一覧でリンク・doc コメント・削除のどれかに当たり、リンク先が今の挙動を述べているか / 秘密と子プロセスの環境変数に関わる詳細が、doc コメントか、今の挙動を述べる文書に残っているか / ADR-0027 の訂正が決定を変えていないか / 文書が 15,000 バイト以下か

**実装モデル要件**: standard

**判定理由**: 文書の書き直しと doc コメントの追加だけで、前例のない設計判断・panel の条件・競合する実装の方法のどれにも当たらない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | `_checks/` の 2 つのスクリプト、strictjson のテストの直し、冒頭の規則、6 つの表、前半の 3 つの表の行、ADR の訂正 5 か所、`CLAUDE.md`、`pricing_update.md` |
| M2（フェーズ 2） | 後半の 3 つの表の行、ADR-0027 の訂正、ADR-0044 のずれの記述の削除 |

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 | テストの直し、冒頭の規則、表の分割、パイプラインの構成要素・LLM・共有部品の行、ADR の訂正 5 か所、`CLAUDE.md`、`pricing_update.md` | standard |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 | サーバとホスト・開発ツール・テスト専用の行、ADR-0027 の訂正、ずれの記述の削除、大きさの確認 | standard |

## 4. テスト戦略 (Test Strategy)

-   **単体テスト:** `TestStrictjsonRowCell` に、表をまたぐ場合を足す（ステップ 1-2）。ほかには足さない。
-   **既存のテスト:** `TestPackageReferenceListsPackages`・`TestPackageReferenceStrictjsonUsers`・`TestADRRecords`・`TestInstructionDocPaths` が、各 PR の `make test` と CI で走る。
-   **確かめるスクリプト:** `_checks/` の 2 つのスクリプトを、各 PR で 1 回実行する。`make test` にも CI にも組み込まない。
-   **記録した diff の読み取り:** ADR・`CLAUDE.md`・`pricing_update.md` の差分を、PR のレビューで読む。
-   **読んで確かめる:** 規則と行の内容、詳細の行き先、PR の説明の一覧は、各 PR のレビューで読む。
-   **後方互換のテスト:** N/A。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 呼び出し側が頼る契約を、行から外したときに失う | 外した文か節のすべてと行き先を PR の説明に載せ、レビューで確かめる（AC-13）。秘密と子プロセスの環境変数に関わる詳細は、削るだけにしない（設計書 3.4 節） |
| 古い設計書の節へリンクする | リンクする前に、その節が今の挙動を述べていることをコードと照らして確かめる（設計書 3.4 節、AC-10） |
| ADR が残すとした内容を、行から誤って外す | 設計書 3.2 節の表を、ステップ 1-6・1-7 で照らし合わせる。ADR の参照は AC-14 の読み取りで確かめる |
| タスク 0083 と、パッケージの参照表の変更がぶつかる | どちらの PR も、そのときの `main` から作業を始める。衝突は、後からマージする PR が `main` を取り込んで解く |
| 文書が 15,000 バイトに収まらない | PR-2 の `check_state.sh` が判定する。収まらないときは、上限を変えずに行を絞り直す。上限を変えるのは決定の変更なので、人に確かめる（AC-09） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `manual` | PR-1 のレビューで、冒頭が行に書くもの・書かないものを述べることを読んで確かめる |
| AC-02 | `manual` | 同じく、詳細の置き場所と、既にある詳細はリンクすることを述べることを読んで確かめる |
| AC-03 | `manual` | 同じく、今の更新の規則が保たれていることを読んで確かめる |
| AC-04 | `manual` | 同じく、6 つの種類の定義と、どれにも当たらないパッケージの扱いを述べることを読んで確かめる |
| AC-05 | `static`・`manual` | `_checks/check_state.sh`（ステップ 1-1）を各 PR で実行し、結果を下の実行の記録に書く。見出しが 6 つの種類に当たり、割り当てが設計書 3.1 節に合うことは、PR-1 のレビューで読む |
| AC-06 | `test`・`static` | `internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`（パッケージの集合）と、`_checks/check_state.sh`（重複がないこと） |
| AC-07 | `manual` | 各 PR のレビューで、その PR で絞った行が AC-01 の規則に従うことを読んで確かめる |
| AC-08 | `test` | `internal/pipeline/strictjson_users_test.go::TestPackageReferenceStrictjsonUsers` |
| AC-09 | `static` | `_checks/check_state.sh` を PR-2 で上限の判定つきで実行し（ステップ 2-6）、結果を下の実行の記録に書く |
| AC-10 | `manual` | 各 PR のレビューで、PR の説明の一覧の「既存の文書へのリンク」の項目が、行からたどれ、リンク先が今の挙動を述べることを確かめる |
| AC-11 | `manual` | 同じく、「doc コメントへ移した」の項目が、doc コメントにあることを確かめる |
| AC-12 | `diff` | `_checks/check_diff.sh`（ステップ 1-1）を各 PR で実行し、結果を下の実行の記録に書く |
| AC-13 | `manual` | 各 PR のレビューで、PR の説明の一覧が、その PR で絞った行から外した文か節のすべてを挙げることを、行の削除と照らして確かめる |
| AC-14 | `manual` | 各 PR のレビューで、その PR で絞った行を参照する ADR の文（`git grep -n package_reference -- docs/dev/adr/`）が、参照先の行が持つ内容だけを指すことを読んで確かめる |
| AC-15 | `diff`・`manual` | 各 PR の ADR の差分を記録した diff の読み取りで確かめる（下の記録）。訂正の行と訂正の段落は、各 PR のレビューで読む |
| AC-16 | `test`・`manual` | `cmd/yt2column/adr_test.go::TestADRRecords`（索引の行と ADR の構造）を、3 文書の PR の CI で走らせる。「決定」の内容と issue #817 の本文は、3 文書の PR のレビューで読む |
| AC-17 | `manual` | PR-1 のレビューでずれの記述の書き換えを、PR-2 のレビューでその削除を読んで確かめる |
| AC-18 | `diff`・`manual` | `CLAUDE.md` の差分を、PR-1 の記録した diff の読み取りで確かめる（下の記録）。説明文の内容は、レビューで読む |
| AC-19 | `diff`・`manual` | `docs/dev/guides/pricing_update.md` の差分を、PR-1 の記録した diff の読み取りで確かめる（下の記録）。文の内容は、レビューで読む |
| AC-20 | `diff` | `_checks/check_diff.sh` を各 PR で実行し、結果を下の実行の記録に書く |
| AC-21 | `test` | `internal/pipeline/strictjson_users_test.go::TestStrictjsonRowCell`（ステップ 1-2 で足す表をまたぐ場合） |

実行の記録（ステップ 1-11・2-6）：未記入。

Diff record (AC-15):

PR-1:

- Compared paths: `docs/dev/adr/0001-llm-adapter-stdlib.md` `docs/dev/adr/0020-strict-json.md` `docs/dev/adr/0023-artifact-resume.md` `docs/dev/adr/0044-package-reference-row-granularity.md`

PR-2:

- Compared paths: `docs/dev/adr/0027-article-job-api.md` `docs/dev/adr/0044-package-reference-row-granularity.md`

Diff record (AC-18):

PR-1:

- Compared paths: `CLAUDE.md`

Diff record (AC-19):

PR-1:

- Compared paths: `docs/dev/guides/pricing_update.md`

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-21 の検証がすべて済み、7 章 に記録がある（AC-16 は 3 文書の PR で済む）。
-   `make test` と `make lint` が通る。

## 9. 次のステップ (Next Steps)

-   PR-2 のマージの後、`/runpr` が issue #817 を閉じる。

## 変更履歴

- 2026-11-11 承認（isseis、#821）
