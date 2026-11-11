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

-   [x] **ステップ 1-1**: 確かめるスクリプトを 2 つ、`docs/tasks/0085_package_reference_summary/_checks/check_state.sh` と `_checks/check_diff.sh` に書く（設計書 7 章）。コメントと出力の文字列は英語で書く。
    -   `check_state.sh`：パッケージの参照表を読み、`## ` の見出しが 6 つで各見出しの下に表が 1 つずつあること（AC-05）と、どのパッケージ名もちょうど 1 行に載ること（AC-06）を確かめる。文書のバイト数を出力し、上限の判定（AC-09）は引数で指定したときだけ行う。
    -   `check_diff.sh`：マージベースと PR のコミットを引数で受け取る。変えたファイルが AC-20 の範囲にあり Go のファイルの追加がないことと、Go のファイルの差分がコメントの行に限ること（AC-12）を確かめる。

    どちらも、違反があれば 0 でない終了コードで終わり、違反の内容を出力する。各確認が、少なくとも 1 つの壊し方で失敗することを確かめ、その結果をこのステップのコミットのメッセージに書く。
-   [x] **ステップ 1-2**: `strictjsonRowCell` を、`Package` の列を持つすべての表から行を集める形に直す（設計書 3.5 節、AC-21）。`TestStrictjsonRowCell` に、行が 2 つ目の表にある場合と、別々の表に 1 つずつある場合を足す。直しを一時的に戻すと、足した場合が失敗することを確かめ、その結果をコミットのメッセージに書く。
-   [x] **ステップ 1-3**: 冒頭を書き換える（設計書 3.1 節、AC-01〜AC-04）。行に書くもの・書かないもの、詳細の置き場所、6 つの種類の定義（箇条書き）を書く。今の更新の規則と `project_overview.md` へのリンクは保つ。
-   [x] **ステップ 1-4**: 6 つの節を作り、全行を設計書 3.1 節の割り当てに従って移す（AC-05・AC-06）。この時点では、行の内容は変えない。表の中は名前順に並べる。`make test` が通ることを確かめる。
-   [x] **ステップ 1-5**: パイプラインの構成要素の行を絞る（設計書 3.2・3.4 節、AC-07・AC-10・AC-11）。外した文か節と、その行き先を、行ごとに PR の説明の一覧の材料として控える（AC-13）。
-   [x] **ステップ 1-6**: LLM の行を絞る。設計書 3.2 節の表で「残す内容」とした番兵の一覧・使用量の型・パラメータの型の置き場所と依存は、行に残す。
-   [x] **ステップ 1-7**: 共有部品の行を絞る。`internal/strictjson` の欄は、`Shared by` の一覧で終える（AC-08）。
-   [x] **ステップ 1-8**: 設計書 3.3 節の PR-1 の ADR の参照 5 か所を、編集上の訂正として直す（AC-14・AC-15）。直した後の参照先が doc コメントのものは、その内容が doc コメントにあることを確かめ、なければ doc コメントへ移す。コミットメッセージに、ADR ごとの訂正の行を書く（`docs/dev/conventions/requirements_process.md`「Recording in the PR description」）。
-   [x] **ステップ 1-9**: ADR-0044 のずれの記述を、残るずれ（後半の 3 つの表の行がまだ規則に従っていないこと）だけを述べる形に書き換える（AC-17）。編集上の訂正（ずれの記述の書き換え）として記録する。PR を作る前に、`git grep -nE 'issues/817([^0-9]|$)' origin/main -- docs/dev/adr/` で、ほかに書き換えるずれの記述がないことを確かめる（`docs/dev/conventions/adr.md`「Discrepancy statements」）。
-   [x] **ステップ 1-10**: `CLAUDE.md:18` の説明文（AC-18）と、`docs/dev/guides/pricing_update.md:39` の 1 文（AC-19）を、新しい冒頭と行の中身に合わせて直す。
-   [x] **ステップ 1-11**: 2 つのスクリプトを PR のブランチで走らせ（`check_state.sh` は上限の判定なし）、出力と 2 つのコミットの完全な ID を 7 章 に書く。AC-15・AC-18・AC-19 の diff の記録は、`/runplan` が 7 章 の記録の形で書く。`make test` と `make lint` を通す。

**完了の条件:** 2 つのスクリプトが終了コード 0 で終わり、7 章 にその記録と diff の記録がある。`make test` と `make lint` が通る。

### PR-1 作成ポイント: test fix, front matter, table split, and pipeline, LLM, and shared rows

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11

**推奨タイトル**: `docs(0085): package_reference.md の規則と表の構成を変え、前半の 3 つの表の行を絞る`

**レビュー観点**: 外した文か節のすべてが、PR の説明の一覧でリンク・doc コメント・削除のどれかに当たり、リンク先が今の挙動を述べているか / ADR が行に残すとした内容が行に残っているか / ADR の参照の訂正が決定を変えていないか / strictjson のテストの直しが、表をまたいだ重複と欠けを見つけるか

**実装モデル要件**: standard

**判定理由**: 文書の書き直しと doc コメントの追加、テストの探し方の小さな直しだけで、前例のない設計判断・panel の条件・競合する実装の方法のどれにも当たらない

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/828
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 後半の 3 つの表と仕上げ（設計書 3 章、AC-05〜AC-07・AC-09〜AC-15・AC-17・AC-20）

**変更するファイル:** 設計書 3 章の表のうち PR-2 の行と、本計画

-   [x] **ステップ 2-1**: サーバとホストの行を絞る（設計書 3.2・3.4 節、AC-07・AC-10・AC-11）。外した文か節と、その行き先を、PR の説明の一覧の材料として控える（AC-13）。
-   [x] **ステップ 2-2**: 開発ツールの行を絞る。秘密の扱いと子プロセスの環境変数に関わる詳細は、設計書 3.4 節のとおり、削るだけにしない。
-   [x] **ステップ 2-3**: テスト専用の行を絞る。
-   [ ] **ステップ 2-4**: 設計書 3.3 節の PR-2 の ADR の参照 2 か所（ADR-0027）を、編集上の訂正として直す（AC-14・AC-15）。コミットメッセージに訂正の行を書く。
-   [ ] **ステップ 2-5**: ADR-0044 のずれの記述を消す（AC-17）。編集上の訂正（ずれの記述の削除）として記録する。PR を作る前に、`git grep -nE 'issues/817([^0-9]|$)' origin/main -- docs/dev/adr/` で、ほかに消すずれの記述がないことを確かめる。
-   [ ] **ステップ 2-6**: `check_state.sh` を上限の判定つき（15,000 バイト）で、`check_diff.sh` を PR-2 のマージベースと PR のコミットで走らせる。上限を超えるときは、前半の 3 つの表の行も含めて絞り直す（5 章のリスクの対策）。出力と 2 つのコミットの完全な ID を 7 章 に書く。AC-15 の diff の記録は、`/runplan` が 7 章 の記録の形で書く。`make test` と `make lint` を通す。

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

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11）
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

実行の記録（ステップ 1-11・2-6）：

PR-1（2026-11-11）: merge-base `cecc4c7b4822a3983ad707a41736a58ea1522864`、記録したコミット `4639b802d8f4426d98d4214230bb248664e07520`。この記録のコミットと、PR チェックポイントのチェックボックスのコミットは、本計画だけを変える。

`_checks/check_state.sh`（上限の判定なし）:

```text
sections: 6
rows: 57
bytes: 57078
OK: six kind headings, one table each, no duplicate package row
```

`_checks/check_diff.sh <merge-base> <commit>`:

```text
base: cecc4c7b4822a3983ad707a41736a58ea1522864
head: 4639b802d8f4426d98d4214230bb248664e07520
changed files: 12
Go files checked: 2 (comment-only except internal/pipeline/strictjson_users_test.go)
OK: the PR stays in scope and its Go diffs are comment-only
```

Diff record (AC-15):

PR-1:

- Compared paths: `docs/dev/adr/0001-llm-adapter-stdlib.md` `docs/dev/adr/0020-strict-json.md` `docs/dev/adr/0023-artifact-resume.md` `docs/dev/adr/0044-package-reference-row-granularity.md`
- Merge-base: `cecc4c7b4822a3983ad707a41736a58ea1522864`
- Compared commit: `81aa1d966c9fd1d2cd65089a97c189b98dd08423`

Path diff:

```text
diff --git a/docs/dev/adr/0001-llm-adapter-stdlib.md b/docs/dev/adr/0001-llm-adapter-stdlib.md
index ec9716d40e7859d655caf102cc8fef1fcdc9eb58..76506e6f85562311fda4f6346c01636fc5acd750 100644
--- a/docs/dev/adr/0001-llm-adapter-stdlib.md
+++ b/docs/dev/adr/0001-llm-adapter-stdlib.md
@@ -17,7 +17,7 @@
 ## 決定

 -   **SDK を使わず、標準ライブラリで API を呼ぶ。** Claude は Anthropic Messages API を、DeepSeek は OpenAI 互換の Chat Completions API を、OpenAI は Responses API を、`net/http` と `encoding/json` で直接呼ぶ。OpenAI の追加は [0077 要件定義書](../../tasks/0077_openai_llm_client/01_requirements.md) が定め、API 固有の方針は [ADR-0041](0041-openai-api.md) に従う。提供元ごとの詳細を閉じ込め、OpenAI の要求・応答の処理は DeepSeek と共通化しない。SDK の JSON のデコードは寛容で、応答を補正せずに拒否する検証と両立しにくく、外部モジュールを増やさない方針にも合うためである（[0009 要件定義書](../../tasks/0009_claude_llm_client/01_requirements.md) §2.3）。規則は [project_overview.md](../project_overview.md) の「決定済みの方針」にある。
--   **HTTP の送信の経路は `internal/llm/llmhttp` の 1 か所に置き、Claude・DeepSeek・OpenAI のアダプタが使う。** 秘密情報の扱いに関わるコードを 2 か所に持たないためである（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md) §1.1）。`llmhttp` が受け持つ送信の手順は、[package_reference.md](../conventions/package_reference.md) のパッケージの表の `internal/llm/llmhttp` の行にある。
+-   **HTTP の送信の経路は `internal/llm/llmhttp` の 1 か所に置き、Claude・DeepSeek・OpenAI のアダプタが使う。** 秘密情報の扱いに関わるコードを 2 か所に持たないためである（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md) §1.1）。`llmhttp` が受け持つ送信の手順は、`internal/llm/llmhttp` のパッケージの doc コメントにある。
 -   **`llmhttp` はプロバイダ固有の番兵を持たず、アダプタが渡した番兵で失敗を報告する。** 各アダプタが自分の番兵を保つためである（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md) §3.4）。規則は [package_reference.md](../conventions/package_reference.md) のパッケージの表の `internal/llm/llmhttp` の行にある。
 -   **プロバイダ固有のパラメータの型は、アダプタの外の小さなパッケージに置く。** Claude は `internal/llm/claudeparam`、DeepSeek は `internal/llm/deepseekparam`、OpenAI は `internal/llm/openaiparam` である。どれも標準ライブラリだけに依存し、設定の読み込みとアダプタの両方が使う。`internal/config` はアダプタのパッケージを import できないので、両方が参照する型はアダプタの外に置く必要があるためである（[0051 設計書](../../tasks/0051_llm_effort/02_architecture.md) §1.2）。規則は [package_reference.md](../conventions/package_reference.md) のパッケージの表のそれぞれのパッケージの行にある。
 -   **アダプタは、応答の厳格な検証に、[ADR-0020](0020-strict-json.md) が定める共有の部品を使う。** 部品を複製すると、サロゲートの走査などの細かいコードが 2 か所になるためである（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。部品の規則は [ADR-0020](0020-strict-json.md) が定める。
diff --git a/docs/dev/adr/0020-strict-json.md b/docs/dev/adr/0020-strict-json.md
index 10456f83658c798821a82d10a7989bb1b6072e6a..460d8942f5c8355386fc3d7ca92a2f97e9b9ee9f 100644
--- a/docs/dev/adr/0020-strict-json.md
+++ b/docs/dev/adr/0020-strict-json.md
@@ -14,10 +14,10 @@ LLM の API の応答、字幕の json3 と info.json、中間生成物、サー

 ## 決定

--   **検証済みの JSON から値を取り出す部品は `internal/strictjson` にだけ置き、複数のパッケージで共有する。部品は、`encoding/json` が黙って補正する入力を拒否する。** 部品を複製するとサロゲートの走査などの細かいコードが 2 か所になり、`internal/transcript` に残すと LLM のアダプタ（構成要素）が字幕取得の構成要素に依存するためである。拒否する入力は [package_reference.md](../conventions/package_reference.md) の `internal/strictjson` の行と、[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) の「3.5. `internal/strictjson`（厳格な JSON の部品）」にある。
+-   **検証済みの JSON から値を取り出す部品は `internal/strictjson` にだけ置き、複数のパッケージで共有する。部品は、`encoding/json` が黙って補正する入力を拒否する。** 部品を複製するとサロゲートの走査などの細かいコードが 2 か所になり、`internal/transcript` に残すと LLM のアダプタ（構成要素）が字幕取得の構成要素に依存するためである。拒否する入力は [0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) の「3.5. `internal/strictjson`（厳格な JSON の部品）」と、`internal/strictjson` のパッケージと各メソッドの doc コメントにある。
 -   **`Object` と `Value` は不透明な型にし、中身を持つ値は `ParseObject` からしか得られないようにする。** フィールドは非公開にする。ゼロ値の `Value` は中身を持たず、その読み取りはエラーを返す。ゼロ値の `Object` はメンバーを持たない。検証を経ていないバイト列を読み取りの関数に渡して U+FFFD への置き換えを受ける呼び出しを、規約でなく型で防ぎ、公開範囲を広げても検証済みの値だけを扱うという不変条件を保つためである。型の形は [0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) の「3.5. `internal/strictjson`（厳格な JSON の部品）」にある。
--   **読み取りの手段は、利用者の必要に合わせて `internal/strictjson` の 1 か所に持つ。** 検査を呼び出し元ごとに書くと書き忘れる余地が残り、部品の中で完結させればその余地がないためである（[0010 設計書](../../tasks/0010_post_generation_review/02_architecture.md) の「3.5.3. 応答の形と受理の規則（H-01）」）。各手段の契約は [package_reference.md](../conventions/package_reference.md) の `internal/strictjson` の行にある。`Object.Collect` の契約は [0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) の「3.5. `internal/strictjson`（厳格な JSON の部品）」にある。
--   **負でない整数の規則は `Value.AsNonNegativeInt64` の 1 か所にする。** 負でない整数の検査を各利用者が書くと、`-0` や小数の扱いが箇所ごとにずれるためである。受理する構文と、拒否のエラーに入力の値を含めないことは、[package_reference.md](../conventions/package_reference.md) の `internal/strictjson` の行にある。
+-   **読み取りの手段は、利用者の必要に合わせて `internal/strictjson` の 1 か所に持つ。** 検査を呼び出し元ごとに書くと書き忘れる余地が残り、部品の中で完結させればその余地がないためである（[0010 設計書](../../tasks/0010_post_generation_review/02_architecture.md) の「3.5.3. 応答の形と受理の規則（H-01）」）。各手段の契約は `internal/strictjson` の各メソッドの doc コメントにある。`Object.Collect` の契約は [0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) の「3.5. `internal/strictjson`（厳格な JSON の部品）」にある。
+-   **負でない整数の規則は `Value.AsNonNegativeInt64` の 1 か所にする。** 負でない整数の検査を各利用者が書くと、`-0` や小数の扱いが箇所ごとにずれるためである。受理する構文と、拒否のエラーに入力の値を含めないことは、`Value.AsNonNegativeInt64` の doc コメントにある。
 -   **負でない整数が要る箇所では `Value.AsInt64` を使わない。** `AsInt64` は符号と `-0` を受理するためである。規則は [0023 設計書](../../tasks/0023_non_negative_int_unification/02_architecture.md) の「1. 設計の全体像 (Design Overview)」にある。
 -   **この部品を使うパッケージの一覧は [package_reference.md](../conventions/package_reference.md) が持つ。** 同じ一覧をほかの文書にも書くと、利用者が増えるたびに何か所も直すことになるためである（[0068 要件定義書](../../tasks/0068_strictjson_package_reference/01_requirements.md) の「1. 概要 (Overview)」）。一覧は [package_reference.md](../conventions/package_reference.md) の `internal/strictjson` の行にある。

diff --git a/docs/dev/adr/0023-artifact-resume.md b/docs/dev/adr/0023-artifact-resume.md
index 8cdfa118a1d0d3468419f341a184e043752c802c..049ea00a683c6657e76bc34ef9b4df9efa7d814f 100644
--- a/docs/dev/adr/0023-artifact-resume.md
+++ b/docs/dev/adr/0023-artifact-resume.md
@@ -17,7 +17,7 @@
 -   **中間生成物を残すかどうかは、`--artifacts <dir>` で明示する。** ディレクトリを渡さない実行は、保存も再開もしない。中間生成物には記事の本文が入り、既定で保存すると本番の実行のたびにディスクに残るうえ、保存先を実行ごとに選べる必要があるためである（[0017 要件定義書](../../tasks/0017_intermediate_artifacts/01_requirements.md) F-002）。規則は [cli.md](../guides/cli.md) の「使い方」のフラグの表にある。
 -   **ディレクトリは利用者が用意し、CLI は作らない。キャッシュの削除で消えないよう、キャッシュディレクトリの中は拒否する。** キャッシュディレクトリの字幕と info.json は投稿の成功後に既定で削除されるので、その中に置くと中間生成物が消えるためである（0017 要件定義書 F-002）。規則は cli.md の「使い方」と、[security.md](../security.md) §5「キャッシュディレクトリ」にある。
 -   **再開する工程は `--after <stage>` で指定する。** `generation`・`shorten`・`review` のいずれかで、指定した工程の出力を再利用し、それより後の工程だけを実行する。工程の実行の有無は、この値と `--no-shorten`・`--no-review` から導き、保存された値から推測しない。呼び出し元が渡した値の中身から振る舞いを選ばないためである（[CLAUDE.md](../../../CLAUDE.md)「Declare, don't infer」）。規則は cli.md の「使い方」と「生成の後の工程」にある。
--   **保存の単位は、動画 ID と工程の組ごとの 1 つの JSON ファイルである。新しく書く形式は版4で、工程の名前・モデル名とモデルの版・テンプレートの識別子・入力の識別子・文面・使用量・呼び出しの時刻・料金の適用条件を持つ。** 各記録は内容から計算した識別子を持ち、生成以外の記録の入力は前の工程の記録の識別子を、生成の入力は素材の識別子を指す。別の実行の中間生成物を、別の素材や別の前段の出力に当てないためである（0017 要件定義書 F-004）。形式は [package_reference.md](../conventions/package_reference.md) の `internal/artifact` の行と、[0017 設計書](../../tasks/0017_intermediate_artifacts/02_architecture.md) §3.2「成果物の形式と識別子（H-01）」にある。
+-   **保存の単位は、動画 ID と工程の組ごとの 1 つの JSON ファイルである。新しく書く形式は版4で、工程の名前・モデル名とモデルの版・テンプレートの識別子・入力の識別子・文面・使用量・呼び出しの時刻・料金の適用条件を持つ。** 各記録は内容から計算した識別子を持ち、生成以外の記録の入力は前の工程の記録の識別子を、生成の入力は素材の識別子を指す。別の実行の中間生成物を、別の素材や別の前段の出力に当てないためである（0017 要件定義書 F-004）。形式は `internal/artifact` のパッケージの doc コメントと、[0077 設計書](../../tasks/0077_openai_llm_client/02_architecture.md) の「3.4.2. 中間生成物と既存の再開の互換性」にある。
 -   **読み込みは信頼しない入力として検証する。** 構文は `internal/strictjson` で厳密に検査し、規則に合わない入力を補正せずに拒否する。再開する工程から生成の出力まで入力をたどり、生成の入力が今の素材と一致することを確かめる。不在・破損・食い違いは、部分的に受理せずに実行の失敗にする。再開の入力は、検証・推敲の応答から作られ、ディスクを経由して戻ってくるためである（0017 要件定義書 F-004）。拒否するものは 0017 要件定義書の F-004 と §3.2「信頼できない入力の境界 (Untrusted Input Boundaries)」、[0022 設計書](../../tasks/0022_cost_estimate/02_architecture.md) の §3.5「中間生成物の形式の変更（H-05）」に、たどり方は 0017 設計書の §3.3「連鎖の検証（H-11・F-004）」にある。
 -   **工程が完了した時点で保存する。** 後の工程が失敗しても、それより前の工程の出力は残る。保存の失敗は、その工程の失敗として扱い、記事を公開しない。再開の実行でも、実行した工程は保存する。後の工程の失敗で前の工程の出力を失わず、保存した中間生成物と公開した記事が食い違う状態を残さないためである（0017 要件定義書 §4.3「信頼性・可用性 (Reliability/Availability)」）。規則は cli.md の「生成の後の工程」にある。
 -   **書き込みは一時ファイルと `rename(2)` で行う。** 中断の後、ファイルは置き換えの前か後のどちらかであり、書きかけの内容が次の再開に受理されない。手順は 0017 設計書の §3.4「保存の手順と中断への強さ（H-03）」にある。
diff --git a/docs/dev/adr/0044-package-reference-row-granularity.md b/docs/dev/adr/0044-package-reference-row-granularity.md
index 747d925f5212de49bf17e60c557697dbfb784727..91197105704cf563b468ee40122a73ab0149bebd 100644
--- a/docs/dev/adr/0044-package-reference-row-granularity.md
+++ b/docs/dev/adr/0044-package-reference-row-granularity.md
@@ -19,7 +19,7 @@
 -   **表は、パッケージの種類ごとに分ける。** 1 つの表に全パッケージが並ぶと、目的のパッケージを探しにくいためである。種類とその定義は [package_reference.md](../conventions/package_reference.md) の冒頭にある。
 -   **行の粒度の上限は、行ごとのバイト数やテストでは決めず、PR のレビューで読んで確かめる。** テストで止めると、正当に長い行があるたびに規則と衝突するためである。維持の費用が防ぐ誤りを上回る文書の確認は足さない（[ADR-0016](0016-document-test-policy.md)）。規則は [package_reference.md](../conventions/package_reference.md) の冒頭にある。

-パッケージの参照表の冒頭は、まだこの規則を述べていない。行もまだ規則に従っておらず、表も種類ごとに分かれていない。この差分は [#817](https://github.com/isseis/yt2column/issues/817) の実装で解消する。
+パッケージの参照表の冒頭はこの規則を述べており、表も種類ごとに分かれている。サーバとホスト・開発ツール・テスト専用の 3 つの表の行は、まだこの規則に従っていない。この差分は [#817](https://github.com/isseis/yt2column/issues/817) の実装で解消する。

 ## 採らなかった案

```

PR-2:

- Compared paths: `docs/dev/adr/0027-article-job-api.md` `docs/dev/adr/0044-package-reference-row-granularity.md`

Diff record (AC-18):

PR-1:

- Compared paths: `CLAUDE.md`
- Merge-base: `cecc4c7b4822a3983ad707a41736a58ea1522864`
- Compared commit: `ea776ecdc86ab7e4622be873fd6f296dff79352a`

Path diff:

```text
diff --git a/CLAUDE.md b/CLAUDE.md
index 60ca7974a8e6a45d3f6b411de83fb8a6fc8263b7..e9aaff0dd1e30358d5733b527650ce0d625dbc67 100644
--- a/CLAUDE.md
+++ b/CLAUDE.md
@@ -15,7 +15,7 @@ This file provides guidance to Claude Code (claude.ai/code) when working with co
 **Development Guides:**
 - Requirements and Acceptance Criteria Process: [requirements_process.md](docs/dev/conventions/requirements_process.md) - Process for implementing new features
 - Test Organization Guide: [test_organization.md](docs/dev/conventions/test_organization.md) - Test helper file organization
-- [Package Reference](docs/dev/conventions/package_reference.md) - Detailed package structure
+- [Package Reference](docs/dev/conventions/package_reference.md) - A one-row summary of each package's responsibility, its main exported names, and where its detailed specification lives
 - [Updating the Pricing Table](docs/dev/guides/pricing_update.md) - How to update the LLM rate table in `internal/pricing`
 - [Claude Output Limit](docs/dev/guides/claude_output_limit.md) - When and how to re-measure the basis of the Claude adapter's default output limit (`max_tokens`)
 - [OpenAI Output Limit](docs/dev/guides/openai_output_limit.md) - When and how to re-measure the basis of the OpenAI adapter's default output limit (`max_output_tokens`)
```

Diff record (AC-19):

PR-1:

- Compared paths: `docs/dev/guides/pricing_update.md`
- Merge-base: `cecc4c7b4822a3983ad707a41736a58ea1522864`
- Compared commit: `ea776ecdc86ab7e4622be873fd6f296dff79352a`

Path diff:

```text
diff --git a/docs/dev/guides/pricing_update.md b/docs/dev/guides/pricing_update.md
index 5c5b2f76523d201b58dfa6a99f24ac8844eb4b8f..be8f33a841bce48f8f1fc8e79736a37c789e02f0 100644
--- a/docs/dev/guides/pricing_update.md
+++ b/docs/dev/guides/pricing_update.md
@@ -39,4 +39,4 @@
 - README の「出力の書式」の Slack への投稿の例と、[cli.md の「`--out` のファイル」](cli.md#--out-のファイル)の例は、料金表から求めた推定額（`0.0094`）を示している。`TestOutputLayoutDocsContract` が実際の出力と比べるので、料金が変われば、2 つの例の金額を更新するまでそのテストは失敗する。
 - README の「設定」の節と `project_overview.md` はモデルの名前を挙げているので、対応するモデルが増減したときだけ更新が必要になる。

-`package_reference.md` の `internal/pricing` の行は、パッケージが公開する型を列挙している。料金の変更では変えない。この行が古くなっていないかは、PR のレビューで読んで確かめる。
+`package_reference.md` の `internal/pricing` の行は、パッケージの責務と、主な公開の名前（`Estimate`・`Totals`・`Amount`・`RateBasis`・`UnknownReason`）を挙げている。料金の変更では変えない。この行が古くなっていないかは、PR のレビューで読んで確かめる。
```

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-21 の検証がすべて済み、7 章 に記録がある（AC-16 は 3 文書の PR で済む）。
-   `make test` と `make lint` が通る。

## 9. 次のステップ (Next Steps)

-   PR-2 のマージの後、`/runpr` が issue #817 を閉じる。

## 変更履歴

- 2026-11-11 承認（isseis、#821）
