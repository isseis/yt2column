# 実装計画書：package_reference.md の行を責務の要約に絞る

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-11 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

パッケージの参照表（`docs/dev/conventions/package_reference.md`）の冒頭に粒度の規則を書き、表を 6 つに分け、全行を責務の要約に絞る。行から外した詳細は、リンク・doc コメント・削除のどれかで扱う。ADR の参照と、関係する 2 つの文書の 1 行を直す。ADR-0044 は 3 文書の PR で作るので、本計画の対象ではない。

### 1.2. 実装の原則

-   **コードの振る舞いを変えない。** Go のファイルで変えるのは doc コメントだけである（AC-11）。
-   **ADR の参照は、その参照先の行を絞る PR で直す**（設計書 1 章）。
-   **新しい恒久のテストは足さない**（設計書 7 章）。

### 1.3. 計画の際に決めたこと（2026-11-11）

-   **実装の PR を 2 本に分ける。PR-1 は冒頭の規則・表の構成と、パイプラインの構成要素・LLM・共有部品の行を扱う。PR-2 は、サーバとホスト・開発ツール・テスト専用の行と、仕上げを扱う。** 行から外す文は合わせて約 60KB ある。外した詳細がほかの場所にあるか、消してよいかをレビューで確かめるので、1 本あたりの量を減らすためである。
    -   採らなかった案：1 本の PR にする案。途中の状態はなくなるが、1 本のレビューの量が大きい。
    -   ADR：なし。本タスクの中で閉じる計画上の選択である。
-   **各実装の PR の説明に、絞った行ごとに、外した詳細の行き先の一覧を載せる（AC-12）。** レビュアーが、外した詳細が失われていないかを確かめやすくするためである。一覧は PR の説明に置き、タスク文書には残さない。
    -   採らなかった案：一覧を作らず、レビュアーが差分を読んで判断する案。
    -   ADR：なし。本タスクの中で閉じる。
-   **ADR の参照の訂正と `pricing_update.md` の直しは、参照先の行を絞る PR で行う。** 理由は設計書 1 章の「設計の際に決めたこと」にある。
    -   ADR：なし。本タスクの中で閉じる。
-   **確認のスクリプトは 2 つとも、シェルで書く。** どちらも `git` と `grep`・`awk` で足りる。

### 1.4. 既存コード調査結果

コミット `04dc4824145e2243bbf8dcfa82c5ff853c43eb11` で確かめた。

-   パッケージの参照表は 75,604 バイトで、表の行は 56 行ある。表は 1 つで、`## ` の見出しはない。
-   `TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:629`）は、`packageReferenceRows`（同 `:672`）で、`|` で始まるすべての行の最初のコードスパンを集める。表を分けても、見出しの行と表の見出しの行（コードスパンがない）は数えない。
-   `TestPackageReferenceStrictjsonUsers`（`internal/pipeline/strictjson_users_test.go:609`）は、`internal/strictjson` の行の `Shared by `（同 `:168`）に続く一覧を読む。
-   ADR がパッケージの参照表を参照する文は 19 行ある（`git grep -n package_reference -- docs/dev/adr/`）。直すのは 7 か所で、内訳は設計書 3.3 節の表にある。
-   `internal/strictjson` の `Value.AsNonNegativeInt64` の doc コメント（`internal/strictjson/strictjson.go:252-256`）は、受理する構文と、エラーが値を含まないことをすでに述べている。
-   `docs/dev/guides/pricing_update.md:39` と `CLAUDE.md:18` が、直す 1 行である。
-   タスク 0083（Slack Webhook の削除）が、`internal/slackwebhook` の行とパッケージの参照表を変える予定である（`docs/tasks/0083_remove_slack_webhook/01_requirements.md:83`）。どちらが先に `main` に入っても、後の PR は、そのときの `main` の表から作業を始める。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 冒頭の規則・表の構成と、前半の 3 つの表（設計書 3 章、AC-01〜AC-07・AC-09〜AC-14・AC-16・AC-18・AC-19）

**変更するファイル:** `docs/dev/conventions/package_reference.md`、`cmd/`・`internal/`・`prompts/` の下の Go のファイル（doc コメントだけ）、`docs/dev/adr/0001-llm-adapter-stdlib.md`、`docs/dev/adr/0020-strict-json.md`、`docs/dev/adr/0023-artifact-resume.md`、`docs/dev/adr/0044-package-reference-row-granularity.md`、`docs/dev/guides/pricing_update.md`、`docs/tasks/0085_package_reference_summary/_checks/`（新規）、本計画

-   [ ] **ステップ 1-1**: 確認のスクリプトを 2 つ、`_checks/` に書く（設計書 7 章）。コメントと出力の文字列は英語で書く。
    -   状態のスクリプト：パッケージの参照表を読み、`## ` の見出しが 6 つで各見出しの下に表が 1 つずつあること（AC-04）と、どのパッケージ名もちょうど 1 行に載ること（AC-05）を確かめる。文書のバイト数を出力し、上限の判定（AC-08）は引数で指定したときだけ行う。
    -   差分のスクリプト：マージベースと PR のコミットを引数で受け取る。変えたファイルが AC-19 の範囲にあることと、Go のファイルの差分がコメントの行に限ること（AC-11）を確かめる。

    どちらも、違反があれば 0 でない終了コードで終わり、違反の内容を出力する。各確認が、少なくとも 1 つの壊し方で失敗することを確かめ、その結果をこのステップのコミットのメッセージに書く。
-   [ ] **ステップ 1-2**: 冒頭を書き換える（設計書 3.1 節、AC-01〜AC-03）。行に書くもの・書かないもの、詳細の置き場所、6 つの種類の定義を書く。今の更新の規則と `project_overview.md` へのリンクは保つ。
-   [ ] **ステップ 1-3**: 6 つの節を作り、全行を設計書 3.1 節の割り当てに従って移す（AC-04・AC-05）。この時点では、行の内容は変えない。表の中は名前順に並べる。
-   [ ] **ステップ 1-4**: パイプラインの構成要素の行を絞る（設計書 3.2・3.4 節、AC-06・AC-09・AC-10）。外した詳細の扱いを、行ごとに PR の説明の一覧の材料として控える（AC-12）。
-   [ ] **ステップ 1-5**: LLM の行を絞る。設計書 3.2 節の表で「残す内容」とした番兵の一覧・使用量の型・パラメータの型の置き場所は、行に残す。
-   [ ] **ステップ 1-6**: 共有部品の行を絞る。`internal/strictjson` の行は `Shared by` の一覧を保つ（AC-07）。
-   [ ] **ステップ 1-7**: 設計書 3.3 節の PR-1 の ADR の参照 5 か所を、編集上の訂正として直す（AC-13・AC-14）。直した後の参照先が doc コメントのものは、その内容が doc コメントにあることを確かめ、なければ doc コメントへ移す。コミットメッセージに、ADR ごとの訂正の行を書く（`docs/dev/conventions/requirements_process.md`「Recording in the PR description」）。
-   [ ] **ステップ 1-8**: ADR-0044 のずれの記述を、残るずれ（サーバとホスト・開発ツール・テスト専用の行がまだ規則に従っていないこと）だけを述べる形に書き換える（AC-16）。編集上の訂正（ずれの記述の書き換え）として記録する。
-   [ ] **ステップ 1-9**: `docs/dev/guides/pricing_update.md:39` の 1 文を、新しい `internal/pricing` の行の中身に合わせて直す（AC-18）。
-   [ ] **ステップ 1-10**: 2 つのスクリプトを PR のブランチで走らせ（状態のスクリプトは上限の判定なし）、出力と 2 つのコミットの完全な ID を §7 に書く。AC-14・AC-18 の diff の記録は、`/runplan` が §7 の記録の形で書く。`make test` と `make lint` を通す。

**完了の条件:** 2 つのスクリプトが終了コード 0 で終わり、§7 にその記録と diff の記録がある。`make test` と `make lint` が通る。

### PR-1 作成ポイント: front matter, table split, and pipeline, LLM, and shared rows

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10

**推奨タイトル**: `docs(0085): package_reference.md の規則と表の構成を変え、前半の 3 つの表の行を絞る`

**レビュー観点**: 外した詳細の行き先の一覧（PR の説明）が、リンク・doc コメント・削除のどれかに正しく当たるか / ADR が行に残すとした内容が行に残っているか / ADR の参照の訂正が決定を変えていないか / doc コメントの差分がコメントだけか

**実装モデル要件**: standard

**判定理由**: 文書の書き直しと doc コメントの追加だけで、前例のない設計判断・panel の条件・競合する実装の方法のどれにも当たらない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 後半の 3 つの表と仕上げ（設計書 3 章、AC-04〜AC-06・AC-08〜AC-14・AC-16・AC-17・AC-19）

**変更するファイル:** `docs/dev/conventions/package_reference.md`、`cmd/`・`internal/` の下の Go のファイル（doc コメントだけ）、`docs/dev/adr/0027-article-job-api.md`、`docs/dev/adr/0044-package-reference-row-granularity.md`、`CLAUDE.md`、本計画

-   [ ] **ステップ 2-1**: サーバとホストの行を絞る（設計書 3.2・3.4 節、AC-06・AC-09・AC-10）。外した詳細の扱いを、PR の説明の一覧の材料として控える（AC-12）。
-   [ ] **ステップ 2-2**: 開発ツールの行を絞る。秘密の扱いと子プロセスの環境変数に関わる詳細は、削るだけにしない（設計書 5 章）。
-   [ ] **ステップ 2-3**: テスト専用の行を絞る。
-   [ ] **ステップ 2-4**: 設計書 3.3 節の PR-2 の ADR の参照 2 か所（ADR-0027）を、編集上の訂正として直す（AC-13・AC-14）。コミットメッセージに訂正の行を書く。
-   [ ] **ステップ 2-5**: ADR-0044 のずれの記述を消す（AC-16）。編集上の訂正（ずれの記述の削除）として記録する。PR を作る前に、`git grep -nE 'issues/817([^0-9]|$)' origin/main -- docs/dev/adr/` で、ほかに消すずれの記述がないことを確かめる。
-   [ ] **ステップ 2-6**: `CLAUDE.md:18` の説明文を、文書の新しい役割に合わせて直す（AC-17）。
-   [ ] **ステップ 2-7**: 状態のスクリプトを上限の判定つき（15,000 バイト）で、差分のスクリプトを PR-2 のマージベースと PR のコミットで走らせる。出力と 2 つのコミットの完全な ID を §7 に書く。AC-14・AC-17 の diff の記録は、`/runplan` が §7 の記録の形で書く。`make test` と `make lint` を通す。

**完了の条件:** 2 つのスクリプトが終了コード 0 で終わり、§7 にその記録と diff の記録がある。`make test` と `make lint` が通る。

### PR-2 作成ポイント: server, host, dev tool, and test-only rows and wrap-up

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7

**推奨タイトル**: `docs(0085): package_reference.md の後半の 3 つの表の行を絞り、ADR-0044 のずれの記述を消す`

**レビュー観点**: 外した詳細の行き先の一覧（PR の説明）が、リンク・doc コメント・削除のどれかに正しく当たるか / 秘密と子プロセスの環境変数に関わる詳細が、security.md か doc コメントに残っているか / ADR-0027 の訂正が決定を変えていないか / 文書が 15,000 バイト以下か

**実装モデル要件**: standard

**判定理由**: 文書の書き直しと doc コメントの追加だけで、前例のない設計判断・panel の条件・競合する実装の方法のどれにも当たらない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | 冒頭の規則、6 つの表、前半の 3 つの表の行、ADR の訂正 5 か所、`pricing_update.md`、`_checks/` の 2 つのスクリプト |
| M2（フェーズ 2） | 後半の 3 つの表の行、ADR-0027 の訂正、ADR-0044 のずれの記述の削除、`CLAUDE.md` |

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 | 冒頭の規則、表の分割、パイプラインの構成要素・LLM・共有部品の行、ADR の訂正 5 か所、`pricing_update.md` | standard |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 | サーバとホスト・開発ツール・テスト専用の行、ADR-0027 の訂正、ずれの記述の削除、`CLAUDE.md`、大きさの確認 | standard |

## 4. テスト戦略 (Test Strategy)

-   **単体テスト:** 足さない。コードの振る舞いを変えない。
-   **既存のテスト:** `TestPackageReferenceListsPackages`・`TestPackageReferenceStrictjsonUsers`・`TestADRRecords`・`TestInstructionDocPaths` が、各 PR の `make test` と CI で走る。
-   **確認のスクリプト:** `_checks/` の 2 つのスクリプトを、各 PR で 1 回実行する。`make test` にも CI にも組み込まない。
-   **記録した diff の読み取り:** ADR・`pricing_update.md`・`CLAUDE.md` の差分を、PR のレビューで読む。
-   **読んで確かめる:** 規則と行の内容、詳細の行き先、PR の説明の一覧は、各 PR のレビューで読む。
-   **後方互換のテスト:** N/A。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 呼び出し側が頼る契約を、行から外したときに失う | 外した詳細の行き先の一覧を PR の説明に載せ、レビューで確かめる（AC-12）。秘密と子プロセスの環境変数に関わる詳細は、削るだけにしない（設計書 5 章） |
| ADR が残すとした内容を、行から誤って外す | 設計書 3.2 節の表を、ステップ 1-5・1-6 で照らし合わせる。ADR の参照は §7 の AC-13 の読み取りで確かめる |
| タスク 0083 と、パッケージの参照表の変更がぶつかる | どちらの PR も、そのときの `main` から作業を始める。衝突は、後からマージする PR が `main` を取り込んで解く |
| 文書が 15,000 バイトに収まらない | PR-2 の状態のスクリプトが判定する。収まらないときは、詳細が残る行を絞り直す。上限を変えるのは決定の変更なので、人に確かめる |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `manual` | 各 PR のレビューで、冒頭が行に書くもの・書かないものを述べることを読んで確かめる |
| AC-02 | `manual` | 同じく、詳細の置き場所と、既にある詳細はリンクすることを述べることを読んで確かめる |
| AC-03 | `manual` | 同じく、今の更新の規則が保たれていることを読んで確かめる |
| AC-04 | `static` | `_checks/` の状態のスクリプト（ステップ 1-1）を各 PR で実行し、結果を下の実行の記録に書く |
| AC-05 | `test`・`static` | `internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`（パッケージの集合）と、状態のスクリプト（重複がないこと） |
| AC-06 | `manual` | 各 PR のレビューで、その PR で絞った行が AC-01 の規則に従うことを読んで確かめる |
| AC-07 | `test` | `internal/pipeline/strictjson_users_test.go::TestPackageReferenceStrictjsonUsers` |
| AC-08 | `static` | 状態のスクリプトを PR-2 で上限の判定つきで実行し（ステップ 2-7）、結果を下の実行の記録に書く |
| AC-09 | `manual` | 各 PR のレビューで、PR の説明の一覧の「既存の文書へのリンク」の項目が、行からたどれることを確かめる |
| AC-10 | `manual` | 同じく、「doc コメントへ移した」の項目が、doc コメントにあることを確かめる |
| AC-11 | `diff` | `_checks/` の差分のスクリプト（ステップ 1-1）を各 PR で実行し、結果を下の実行の記録に書く |
| AC-12 | `manual` | 各 PR のレビューで、PR の説明に一覧があることを確かめる |
| AC-13 | `manual` | PR-2 のレビューで、`git grep -n package_reference -- docs/dev/adr/` の各行が、参照先の行が持つ内容だけを指すことを読んで確かめる |
| AC-14 | `diff`・`manual` | 各 PR の ADR の差分を記録した diff の読み取りで確かめる（下の記録）。訂正の行と訂正の段落は、各 PR のレビューで読む |
| AC-15 | `test`・`manual` | `cmd/yt2column/adr_test.go::TestADRRecords`（索引の行と ADR の構造）を、3 文書の PR の CI で走らせる。「決定」の内容は、3 文書の PR のレビューで読む |
| AC-16 | `manual` | PR-1 のレビューでずれの記述の書き換えを、PR-2 のレビューでその削除を読んで確かめる |
| AC-17 | `diff`・`manual` | `CLAUDE.md` の差分を、PR-2 の記録した diff の読み取りで確かめる（下の記録）。説明文の内容は、レビューで読む |
| AC-18 | `diff`・`manual` | `docs/dev/guides/pricing_update.md` の差分を、PR-1 の記録した diff の読み取りで確かめる（下の記録）。文の内容は、レビューで読む |
| AC-19 | `diff` | 差分のスクリプトを各 PR で実行し、結果を下の実行の記録に書く |

実行の記録（ステップ 1-10・2-7）：未記入。

Diff record (AC-14):

PR-1:

- Compared paths: `docs/dev/adr/0001-llm-adapter-stdlib.md` `docs/dev/adr/0020-strict-json.md` `docs/dev/adr/0023-artifact-resume.md` `docs/dev/adr/0044-package-reference-row-granularity.md`

PR-2:

- Compared paths: `docs/dev/adr/0027-article-job-api.md` `docs/dev/adr/0044-package-reference-row-granularity.md`

Diff record (AC-17):

PR-2:

- Compared paths: `CLAUDE.md`

Diff record (AC-18):

PR-1:

- Compared paths: `docs/dev/guides/pricing_update.md`

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-19 の検証がすべて済み、§7 に記録がある（AC-15 は 3 文書の PR で済む）。
-   `make test` と `make lint` が通る。

## 9. 次のステップ (Next Steps)

-   PR-2 のマージの後、`/runpr` が issue #817 を閉じる。
