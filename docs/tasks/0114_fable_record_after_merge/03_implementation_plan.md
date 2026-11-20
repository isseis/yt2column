# 実装計画書：Fable の部分レビューの記録の後、push の前にできた commit を、PR を開く前に確かめる

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-20 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を指す。節番号と「決定 N」は [02_architecture.md](02_architecture.md)（以下、設計書）の節と 1.2 の決定を、H-NN は [design_handoff.md](design_handoff.md) の項番を指す。H-NN の扱いは、どれも設計書 10 章が記録している。本タスクに `implementation_handoff.md` はない。

既存のファイルの内容と `file:line` の行番号は、コミット `4d889096`（`origin/main`）で確かめた。設計書が確かめたコミットは `0051d3f4` である。本計画が変えるファイルのうち、その後に変わったのは次の 2 つで、設計書が引く行番号は今も合う（`git diff --stat 0051d3f4 4d889096 -- .claude/commands/ cmd/yt2column/review_records_test.go docs/dev/adr/0050-fable-partial-review.md`）。

-   `_lib/local-cli-review.md`: 93 行目の 1 文が延びただけである。
-   ADR-0050: 設計の PR（#1049）が更新した。

用語は、要件書 3.1・6 章と設計書の冒頭（判定のスクリプト・ブランチ・END・救済・判定した commit（D）・確認できる merge commit）と同じ意味で使う。大文字の名前（FABLE・CHECKPOINT・AFTER_REMEDY・ENTRY・PLAN・RECORD など）は、設計書 3.3.1・3.4.1 と共通の手順が定める入力の名前である。本計画は、次の用語を加える。

-   **スクリプトのテスト:** 判定のスクリプトを一時的な git のリポジトリの上で走らせる Go のテスト。`cmd/yt2column/review_records_test.go` にある。
-   **実地の確かめ:** PR-2 のマージの後、ほかのタスクのタスク文書の PR で、merge の後の確認を本物のセッションで走らせて確かめること（設計書 7.3）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

判定のスクリプトに、merge の後の確認の記録の読み方と `unreviewed` のモードを足す。あわせて、`_lib/fable-review.md`・`_lib/task-pr-handoff.md`・`runplan.md`・`_lib/local-cli-review.md` の手順に、push の直前の判定と merge の後の確認を足す。これで、Fable の記録の後、push の前にできた commit を、PR を開く前に確かめるようにする（要件書 2.1）。何をどこに書くかは設計書 3 章が定める。本計画は、作業の順序と確かめ方を定める。

### 1.2. 実装の原則

-   **文面は設計書 3 章に従い、英語で書く。** コマンドの定義、`_lib/`、スクリプト、Go のテストは英語で書く。判定のスクリプトの日本語のリテラルは、今のもの（計画書のチェックボックスの文）だけで、本タスクでは足さない。各ステップは、書く内容を設計書の節で指す。文面は実装の PR で書く。
-   **判定のスクリプトの規則は、スクリプトのテストで確かめる**（設計書 7.1）。テストケースごとに、実装者がスクリプトの該当する規則を実際に壊して失敗することを確かめ、commit のメッセージに書く。有効と判定する側を広げる 3 つの変更には、受け付けない側のテストケースをそれぞれ置く（設計書 7.1 の最後の段落）。3 つの変更とは、規則 4、確認の記録の名指し、範囲の確かめである。
-   **コマンドの定義の手順の文だけを定める AC は、PR のレビューで読んで確かめる**（設計書 7.2 の「文の内容」）。値と構造（スクリプトの呼び方、記録の行の形と前置き、メンションの確かめの置き場所）は、guard のテストで確かめる。
-   **シェルスクリプトは bash 3.2 でも動く形のままにする。** 判定のスクリプトの冒頭の「Inputs」「Failure」の契約（`review-records.sh:53-71`）を、新しいモードにも当てる。

### 1.3. 計画の際に決めたこと（2026-11-20）

1.  **PR は 3 本にする。PR-3 の自動送信は `hold` とする。** 各 PR の受け持ちは次のとおりである。
    -   **PR-1（フェーズ 1）:** 判定のスクリプトとそのテスト。
    -   **PR-2（フェーズ 2）:** 手順、guard のテスト、ADR-0050 の食い違いの文の削除。
    -   **PR-3（フェーズ 3）:** 実地の確かめの記録。

    理由は 2 つある。スクリプトは `origin/main` から取り出して走るので、手順が新しい出力を読む前に、`main` にスクリプトがある必要がある（設計書 8 章）。また、実地の確かめは、PR-2 が `main` に入った後でしかできない。人の答え：3 本（推奨）。
    -   採らなかった案：(A) PR-1 を `fable` のモードと `unreviewed` のモードの 2 本に分け、4 本にする。範囲の確かめ（設計書 3.1.4）の共有の部分が 2 つの PR にまたがる。(B) 2 本にし、実地の確かめを PR-2 の後の作業にする。確かめの記録を残す PR がなくなる。
    -   ADR：なし。PR の分け方は本タスクの中で閉じる。
2.  **PR-1 は、`fable-review.md` の「The validity decision」の規則の箇条と「Record lines」の表の 4 行も変える。** 「The validity decision」は、有効かの規則の正本である（`fable-review.md:107-109`、要件書 4.5）。スクリプトだけを変えると、PR-1 のマージから PR-2 のマージまで、正本とスクリプトが食い違う。また、スクリプトのテストの `readFableForms`（`review_records_test.go:735-749`）は、記録の行の形を「Record lines」の表から読む。表に `Merge check:` の行がなければ、テストが確認の記録を作れない。出力 `MERGE` の読み方、入口、「The merge check」の節は PR-2 で書く。それまでの間、今の手順は `MERGE` を「形の違う 1 行目」として `MISSING <merge-base>` と読み、Fable が PR の全体を見る（設計書 8 章）。人の答え：入れる（推奨）。
    -   採らなかった案：PR-1 はスクリプトとテストだけにし、テストは確認の記録の形をいったん Go のリテラルに持って、PR-2 で表から読む形に移す。テストを 2 度書くことになり、PR-1 の間は正本の規則とスクリプトが食い違う。
    -   ADR：なし。PR の受け持ちの分け方で、本タスクの中で閉じる。
3.  **実地の確かめは、設計書 7.3 のとおり、ほかのタスクのタスク文書の PR で行う。** PR-2 のマージの後、ほかのタスクのタスク文書のコマンドの実行で、人が次のように指示する。Part B step 1 の記録の commit の後、step 1.5 の前に `origin/main` を merge させる。そのうえで、step 1.5 が merge の後の確認を 1 回走らせ、Fable が走らないことを確かめる。結果は PR-3 で本計画の §7 に記録する。`origin/main` が、そのブランチを作った後に進んでいる必要があるので、機会が来るまで PR-3 の `/runplan` を送らない（`hold`）。人の答え：別タスクの文書 PR（推奨）。
    -   採らなかった案：PR-3 の `/runplan` 自身で、最後の phase group の記録の後に `origin/main` を merge し、チェックポイントの入口で確かめる。`/runplan` の経路の確かめになり、設計書 7.3 の「タスク文書の PR」と食い違うので、設計書の直しが要る。
    -   ADR：なし。確かめ方の選択である。
4.  **`unreviewed` のモードの `<PLAN>` は、`fable` のモードと同じく必ず渡す。ファイルはなくてもよい。** 設計書 3.4.4 が、Part B step 3 の呼び出しの行を `bash <records-script> unreviewed <HEAD> <PLAN>` と文字どおりに定めているためである。タスク文書の PR の時点で計画書がまだなくても、パスは決まる（`fable-review.md:41-42` と同じ扱い）。
    -   ADR：なし。スクリプトの引数の形で、本タスクの中で閉じる。

### 1.4. 既存コード調査結果

**判定のスクリプト（`.claude/commands/_lib/review-records.sh`）。**

-   **あるもの:**
    -   モードの振り分け（`:584-596`）と、失敗のときの出力（`fail`、`:179-186`）。
    -   範囲の行の確かめ `checked_range`（`:416-425`）。START の条件は `:423` にある。
    -   中身のない commit の見分け `contentless`（`:369-389`）、空の commit の見分け `empty_commit`（`:392-396`）、チェックボックスの commit の見分け `checkbox_only`（`:295-302`）。
    -   `fable` のモードの本体（`:505-582`）。記録を探す処理（`:517-531`）、同じ実行の直し（`:533-542`）、印の後の commit（`:544-562`）、判定と START（`:564-581`）からなる。merge commit は、`parent_count` が 1 でないので、どの規則にも当たらない（`:568-573`）。
    -   `coverage` のモードの本体（`:427-489`）。
-   **足りないもの:**
    -   merge の後の確認の記録の正規表現と、その読み方。
    -   確認できる merge commit の見分け。
    -   規則 4。
    -   出力 `MERGE <n>`。
    -   `unreviewed` のモード。
-   **変えるもの:**
    -   `checked_range` の START の条件。`fable` と `unreviewed` のモードだけで広げ、`coverage` では今のままにする（設計書 3.1.4）。
    -   START の決め方（`:574-577`、設計書 3.1.5）。
    -   ヘッダーの `ENTRY` の説明（`:76`）、`fable` の契約（`:86-120`）、使い方の行（`:5-6`）。
    -   `fail` が `unreviewed` で `PARTIAL` を出すこと。
-   **再利用する:** `coverage` と `unreviewed` は、判定の関数を 1 つ共有し、設計書 3.2 の 2 点だけを引数で選ぶ。`resolve12`・`in_list`・`message`・`parent_count`・`load_branch`・`has_line` は、そのまま使う。

**スクリプトのテスト（`cmd/yt2column/review_records_test.go`）。**

-   **あるもの:**
    -   fixture `rrFixture`（`:91-196`）。merge のための `merge`（`:156-160`）と `side`（`:164-174`）を持つ。`origin/main` を進める例は、`commit-tree` と `update-ref` を使う件（`:930-956`）にある。
    -   `fableCases`（`:802-`）と `TestReviewRecordsFable`（`:1146-1200`）。各件を `scriptBashes` の各 bash で走らせ、git の設定を変えた走らせ方（`rrConfigScenarios`）でも同じ出力になることを確かめる。
    -   `coverageCases`（`:231-`）と `TestReviewRecordsCoverage`（`:652`）。
    -   `readFableForms`（`:735-749`）、`TestReviewRecordsMode`（`:715-725`）、`TestReviewRecordsCallSite`（`:1234-1260`）、`TestRecordLinePrefixesDistinct`（`:1318-1360`）、`TestRecordCommitsCheckMention`（`:1365-1389`）。
-   **足りないもの:**
    -   確認の記録の行を作る fixture の関数。
    -   確認できない merge commit の 3 つの形を作る関数（親が 3 つ、第 1 親が `main` 側、第 2 親が `origin/main` にない）。
    -   `unreviewed` のテスト。
-   **期待する出力が変わる既存の件:** 設計書 3.7 の 3 件である。そのうち「merge after the record」（`:1007-1014`）は、`side` のブランチ（`origin/main` から届かない）を merge する。そのため、決定 12 の確認できない merge commit に当たり、`MISSING <merge-base>` になる。設計書 3.7 はこの件の見込みを `MERGE 1` と書いていた。決定 12 と食い違う表のセルなので、本 PR で設計書を編集上の訂正として直した（決定の変更なし）。merge を作るほかの件（`f.merge` と `git merge` を呼ぶ件）も、実装の PR で走らせ直して確かめる（設計書 3.7）。

**手順の文書。**

-   **`_lib/fable-review.md`:** 入口の表（`:13-19`）は 3 つで、「three entry points」とある。FABLE の段落は、チェックポイントの入口を KIND `implementation` に固定している（`:45-47`）。「Two task PRs pass FABLE」の段落（`:21-25`）もある。「The validity decision」（`:79-135`）は、規則の箇条（`:111-128`）、読み方（`:94-106`）、「A merge commit is never covered」（`:134-135`）を持つ。このほか、Entry 1（`:137-163`）、Entry 3（`:234-248`）、単独の経路（`:208-232`）、「Record lines」（`:437-525`）、「Reading the record lines」（`:527-546`）がある。「The merge check」の節と Entry 4 はない。
-   **`_lib/task-pr-handoff.md`:**
    -   Part B は step 1〜7 からなる。step 1.5 と step 2.5 はない。
    -   step 1 は FABLE を自分で組み立てる（`:184-197`）。
    -   step 3 は、ブランチ名で push する（`:218`）。どちらの CLI もレビューしなかった commit の文（`:250-260`）は、セッションの SUMMARY を使う。
    -   Authorization の段落（`:61-71`）は、`git push`（`-u` を含む）を挙げる。
    -   `postingSections` の表（`codex_review_docs_test.go:74-78`。`TestPostingPathsCheckMention` が読む）は、step 3 の書き出し `3. Push and create the PR for the current branch.` と step 4 の書き出し `4. Run AFTER_CREATE` で節を探す。`TestHandoffListingPrintsNoBody`（`fixpr_docs_test.go:197`）は、step 5 と step 6 の書き出しで節を探す。どちらの書き出しも変えない（設計書 3.4.4 の「step 番号」）。
-   **`runplan.md`:** step 5a のチェックポイントの段落（`:98`）、Part B の呼び出し（`:104-149`）、step 8 のチェックポイント（`:263`）、B1 の文（`:279-280`）、step 5a の LOCAL_REVIEW の文（`:120-123`）がある。
-   **`_lib/local-cli-review.md`:** 呼び出し元の一覧（`:10-23`、「Seven callers」）と、RECORD の表（`:70-79`）がある。
-   **ADR-0050:** 決定の節 5 の後の食い違いの文（`docs/dev/adr/0050-fable-partial-review.md:84`）が、#988 にリンクしている。
-   **変えない文書:** `runpr.md`・`opus-review.md`・`mk*.md` は変えない（設計書 2.2）。`CLAUDE.md:229-235` と `_lib/simple-process.md:232` は、Fable の部分レビューを要約するだけで、入口の数や push の形を述べていない。それでも PR-2 で読み直し、食い違いが出ないことを確かめる（ステップ 2-7）。

**型の生成の経路:** 本タスクに、具体的な型で振り分ける設計はない（N/A）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 判定のスクリプトとそのテスト（設計書 8 章の 1）

**変更するファイル:** `.claude/commands/_lib/review-records.sh`、`cmd/yt2column/review_records_test.go`、`.claude/commands/_lib/fable-review.md`（「The validity decision」の規則の箇条と「Record lines」の表だけ）、`docs/dev/adr/0050-fable-partial-review.md`（食い違いの文だけ）、本計画

-   [ ] **ステップ 1-1**: `fable` のモードを変える（設計書 3.1）。
    -   範囲の確かめで、START の条件を広げる（3.1.4）。Fable の記録、同じ実行の直しの範囲の行、規則 4 の見た範囲の行に当てる。`coverage` の確かめは変えない。
    -   merge の後の確認の記録を読む（3.1.3）。`Merge check: run` と `Merge check: not run` の形の正規表現を足す。名指しした ID の 4 つの条件を確かめる。
    -   確認できる merge commit を見分ける（用語、決定 12）。
    -   規則 4 を足す（3.1.2）。
    -   結果を `MISSING`・`MERGE <n>`・`VALID` の順で決める（3.1.2、3.1.1）。
    -   START を決め直す（3.1.5）。
    -   ヘッダーの使い方、`ENTRY` の説明（決定 5）、`fable` の契約を合わせる。
-   [ ] **ステップ 1-2**: `unreviewed` のモードを足す（設計書 3.2、§1.3 の決定 4）。`coverage` と判定の関数を共有し、3.2 の 2 点（範囲の確かめと merge commit の扱い）だけを引数で選ぶ。ヘッダーは `unreviewed` を「`coverage none` に 2 点を当てたもの」と定義し、規則を 2 度書かない。失敗のときは `PARTIAL` だけを出して、終了コード 1 で終わる。
-   [ ] **ステップ 1-3**: `_lib/fable-review.md` を変える（§1.3 の決定 2）。
    -   「The validity decision」の規則の箇条に、規則 4、確認できる merge commit と確認済みの merge commit の扱い、広げた範囲の確かめ、START の決め方を足す（設計書 3.1.2〜3.1.5）。「A merge commit is never covered」（`:134-135`）を、それに合わせて直す。出力の読み方（`:94-106`）は PR-2 で変える。
    -   「Record lines」の表に、`Merge check:` の 4 行を足す。値（`<reason>`・`<criterion>`）の説明も足す（設計書 3.3.6）。
-   [ ] **ステップ 1-4**: スクリプトのテストの fixture を足す。
    -   `readFableForms` が `Merge check: run` と `Merge check: not run` の形を表から読む。
    -   確認の記録の行を作る関数を足す。
    -   確認できない merge commit の 3 つの形を作る関数を足す（§1.4）。
-   [ ] **ステップ 1-5**: `fableCases` に、設計書 7.1 の `fable` のモードの表の件をすべて足す。
-   [ ] **ステップ 1-6**: 期待する出力が変わる既存の件を、それぞれ直す（設計書 3.7）。
    -   [ ] 「record from a merge-base origin/main moved past」: `MERGE 1` と M にする。
    -   [ ] 「merge after the record」: `MISSING <merge-base>` にする。件の前のコメントも、merge の規則ではなく確認できない merge commit であることを述べる形に直す。
    -   [ ] 「range of the record's commit beyond it」: `MISSING <merge-base>` にする。
    -   [ ] merge を作るほかの件を走らせ、期待する出力が変わらないことを確かめる。
-   [ ] **ステップ 1-7**: `unreviewed` のテスト `TestReviewRecordsUnreviewed` を足す。
    -   `coverageCases` のうち `LAST_REVIEWED` が `none` の件を、`unreviewed` の引数に移して再利用する。3.2 の 2 点で結果が変わる件（たとえば「merge commit inside a seen range」）は、期待する出力を上書きする。どの件が変わるかは、実装の際に走らせて決める。
    -   設計書 7.1 の `unreviewed` の表の件を足す。
    -   `coverage` と同じく、各 bash、git の設定と環境を変えた走らせ方、失敗の件（引数の形、`origin/main` がない）でも確かめる。
-   [ ] **ステップ 1-8**: guard のテストを直す（設計書 3.7）。
    -   `TestReviewRecordsMode` のモードの一覧に `Unreviewed` を足す。
    -   `TestRecordLinePrefixesDistinct` が、足す接頭辞どうし（`Fable ...` と `Merge check ...`）も比べるようにする。
-   [ ] **ステップ 1-9**: ステップ 1-5〜1-8 の各件について、スクリプトの該当する規則か、文書の行の形を壊すと失敗することを確かめ、commit のメッセージに書く。有効と判定する側を広げる 3 つの変更には、受け付けない側の件がそれぞれ失敗することを含める。3 つの変更とは、規則 4 の各条件、名指しの 4 つの条件、範囲の確かめの 2 つの条件である（設計書 7.1）。
-   [ ] **ステップ 1-10**: ADR-0050 の食い違いの文（`:84`）を、残る食い違いだけを述べる形に書き直す。判定のスクリプトの部分は PR-1 で解消し、手順の部分（タスク文書の PR の印、push の直前の判定、merge の後の確認）が残る。編集上の訂正の 2 つ目の種類として、commit のメッセージに訂正の行を書く（`docs/dev/conventions/adr.md`「Discrepancy statements」）。
-   [ ] **ステップ 1-11**: AC-23 の差分の記録を書く（§7 の「差分の記録」の PR-1 の欄）。

**完了の条件:**

-   設計書 3.1・3.2 の項目と、ステップ 1-3 の `fable-review.md` の変更が、スクリプトと文書にある。
-   `bash -n` が判定のスクリプトに通る。
-   macOS の手元で `go test -tags test -v -run 'TestReviewRecords' ./cmd/yt2column` を走らせ、`/bin/bash` 3.2 の subtest が走って通ったことを、PR の説明に書く。
-   `TestReviewRecordsCoverage` の既存の件が、期待する出力を変えずに通る（`coverage` を変えないこと、要件書 2.3）。
-   §7 に PR-1 の差分の記録がある。
-   `make test` と `make lint` が通る。

### PR-1 作成ポイント: validity decision for merges after the Fable record

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11

**推奨タイトル**: `feat(0114): 判定のスクリプトに merge の後の確認の記録と unreviewed のモードを足す`

**レビュー観点**: 有効と判定する側を広げる 3 つの変更（規則 4・名指し・範囲の確かめ）が受け付けない件を持つか / 確認できない merge commit が `MISSING <merge-base>` に倒れるか / `coverage` の判定が変わらないか / `fable-review.md` の規則の箇条とスクリプトのヘッダーが同じ規則を述べるか

**実装モデル要件**: opus

**判定理由**: ステップ 1-1 は、Fable の記録を有効とみなして PR を開いてよいかを決める判定の関門を、有効の側へ広げる変更である（`mkplan.md` step 8 のパネルの条件の security-gate）。Sonnet が名指しの条件や範囲の確かめを取り違えると、確認していない merge commit や見ていない commit を `VALID` と判定し、Fable を飛ばして PR を開かせうる。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 手順の変更（設計書 8 章の 2）

**変更するファイル:** `.claude/commands/_lib/fable-review.md`、`.claude/commands/_lib/task-pr-handoff.md`、`.claude/commands/runplan.md`、`.claude/commands/_lib/local-cli-review.md`、`cmd/yt2column/review_records_test.go`、`docs/dev/adr/0050-fable-partial-review.md`、本計画

-   [ ] **ステップ 2-1**: `_lib/fable-review.md` の入口を変える（設計書 3.3.1〜3.3.4）。
    -   冒頭の入口の数と入口の表を 4 つにする（「three entry points」を直す）。「Two task PRs pass FABLE」の段落と、FABLE の段落（`:45-47`）を合わせる。
    -   「The validity decision」の読み方に `MERGE` と、その形の確かめを足す（3.1.1）。判定を走らせる入口の文（`:81`）と `<ENTRY>` の説明（`:96-98`）を、決定 5 に合わせる。
    -   Entry 1 の `MERGE`（3.3.2）、Entry 3 の変更（3.3.3。印の後に作る commit の一覧を含む）、Entry 4（3.3.4）を書く。
-   [ ] **ステップ 2-2**: `_lib/fable-review.md` に節「The merge check」を足す（設計書 3.3.5、決定 3・7・8・9・12）。
    -   範囲の一覧、範囲を求められないときの扱い、指示、出力の確かめ、記録の commit を書く。
    -   待つ時間と呼び直しは、Entry 1・Entry 2 の該当する step を引数を替えて参照し、文を写さない。
    -   記録の commit の前に、`_lib/local-cli-review.md` step 7 の「Mention check」を走らせることを書く。
    -   「Reading the record lines」に、merge の後の確認の行の読み方を足す（3.3.6）。
-   [ ] **ステップ 2-3**: `_lib/local-cli-review.md` の呼び出し元の一覧（`:10-23`）と RECORD の表（`:70-79`）に、「The merge check」（RECORD `return`）を足す（設計書 2.1）。
-   [ ] **ステップ 2-4**: `_lib/task-pr-handoff.md` を変える（設計書 3.4）。
    -   「Inputs」に FABLE・CHECKPOINT・AFTER_REMEDY を足す（3.4.1）。step 1 は、FABLE を受け取って渡す形にする。
    -   step 1.5（3.4.2）と step 2.5（3.4.3）を足す。
    -   step 3 を変える（3.4.4）。本文を読む範囲を `origin/main..<D>` にする。「どちらの CLI もレビューしなかった commit」の文を、`unreviewed` のモードの 3 行のコードブロックに置き換える。merge の後の確認の節と、push の直前の判定の行を足す。push は D を名指しし、上流を設定し直す。HEAD が D と違うときの扱いと、D が決まっていない呼び出しの扱いを書く。step 3 と step 4 の書き出しの文は変えない。
    -   AFTER_CREATE の前に、HEAD が D であることを確かめる（決定 10）。
    -   Authorization の段落に、D を名指しした push と `git branch --set-upstream-to` を足す。
-   [ ] **ステップ 2-5**: `runplan.md` を変える（設計書 3.5）。
    -   step 5a（`:98`）と step 8（`:263`）のチェックポイントの段落に、Entry 3 の `MERGE` と FABLE を足す（3.5.1）。
    -   Part B の呼び出しに、FABLE・CHECKPOINT `done`・AFTER_REMEDY を足す（3.5.2）。
    -   B1 の文（`:279-280`）と、step 5a の LOCAL_REVIEW の文（`:120-123`）を直す（3.5.3）。
-   [ ] **ステップ 2-6**: guard のテストを直す（設計書 3.7、7.2）。
    -   `TestReviewRecordsCallSite` について、2 つを変える。許す呼び出しの行に `bash <records-script> unreviewed <HEAD> <PLAN>` を足す。走らせる節に、`task-pr-handoff.md` Part B step 3 の 3 行の並びを足す。
    -   `TestRecordCommitsCheckMention` の確かめる節に、「The merge check」を足す。
    -   どちらも、足した確かめを壊すと失敗することを確かめ、commit のメッセージに書く。壊し方は 2 つある。Part B step 3 から実行の行を消すこと、「The merge check」からメンションの確かめを消すことである。
-   [ ] **ステップ 2-7**: 手順の変更の後、次の箇所を読み、新しい手順と食い違う文がないことを確かめる。食い違えば直し、PR の説明に書く。
    -   `CLAUDE.md:224-235`、`_lib/simple-process.md:232`。
    -   `runpr.md` のうち、`task-pr-handoff.md` Part B step 3 を指す箇所（`:164`・`:177`・`:1555`）。
    -   `local-cli-review.md:337`・`:411`。
-   [ ] **ステップ 2-8**: ADR-0050 の食い違いの文を消す。
    -   PR を作る前に、`git grep -nE 'issues/988([^0-9]|$)' origin/main -- docs/dev/adr/` で、`origin/main` の文を探す。
    -   決定の節にあり、この食い違いを述べる文だけを消す。commit のメッセージに訂正の行を書く（`docs/dev/conventions/adr.md`「Discrepancy statements」）。
-   [ ] **ステップ 2-9**: AC-23 の差分の記録を書く（§7 の「差分の記録」の PR-2 の欄）。

**完了の条件:**

-   設計書 3.3〜3.5 の項目が、文書にある。ADR-0050 に、#988 にリンクする食い違いの文がない。
-   §7 に PR-2 の差分の記録がある。
-   `make test` と `make lint` が通る。

### PR-2 作成ポイント: pre-push re-check and merge check procedures

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9

**推奨タイトル**: `feat(0114): push の直前に Fable の記録を確かめ直し、記録の後の merge を merge の後の確認で扱う`

**レビュー観点**: push するのが判定した commit D だけで、確かめの後に足された commit が送られないか / 救済の後の再チェックの順序（AFTER_REMEDY・README の同期・CHECKS）と、最後の判定で止まる条件 / merge の後の確認の範囲の一覧と、範囲を求める git が失敗したときに Fable へ回すこと / 「The merge check」がサブエージェントの出力の文を記録や PR の説明に写さないこと

**実装モデル要件**: opus

**判定理由**: ステップ 2-1・2-4 は、PR を開いてよいかを決める判定の関門と push の形を変える（`mkplan.md` step 8 のパネルの条件の security-gate）。Sonnet が D の名指しや HEAD の確かめ、救済の後の再チェックを取り違えると、判定されていない commit を push したり、Fable も確認もないまま PR を開いたりしうる。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 実地の確かめ（設計書 8 章の 3）

**変更するファイル:** 本計画

-   [ ] **ステップ 3-1**: 実地の確かめの結果を、§7 の「実地の確かめの記録」に書く（設計書 7.3、§1.3 の決定 3）。確かめは、PR-2 のマージの後に、人がほかのタスクのタスク文書の PR で行う。書くのは次のとおりである。
    -   その PR の URL。
    -   記録の commit、merge commit、merge の後の確認の記録の commit の ID。
    -   step 1.5 の判定の出力の 1 行目（2 回分）。
    -   `Merge check:` の行が `run` か `not run` か。`run` なら、モデル ID が `claude-opus-` で始まること（AC-25）。
    -   新しい `Fable review:` の記録が足されていないこと（AC-24）。
    -   設計書と違う振る舞いがあれば、その内容。違いがあれば、決定の変更として人と決める。
-   [ ] **ステップ 3-2**: AC-23 の差分の記録を書く（§7 の「差分の記録」の PR-3 の欄）。

**完了の条件:** §7 に実地の確かめの記録と PR-3 の差分の記録がある。`make test` と `make lint` が通る。

### PR-3 作成ポイント: live check of the merge check

**対象ステップ**: 3-1 / 3-2

**推奨タイトル**: `docs(0114): merge の後の確認を実地で確かめた結果を記録する`

**レビュー観点**: 実地の確かめで、step 1.5 が merge の後の確認を 1 回だけ走らせ、Fable が走らなかったか / `Model:` の行が Opus を示したか / 記録が、確認のサブエージェントの出力の文を写していないか

**実装モデル要件**: sonnet

**判定理由**: どの条件にも当たらない。実地の確かめは PR-2 のマージの後に人がほかのタスクで行い、この PR はその結果を計画書に書くだけである。

**自動送信**: hold

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 前提 |
|---|---|---|
| M1（PR-1） | 判定のスクリプトの `fable` のモードの変更と `unreviewed` のモード、そのテスト、`fable-review.md` の規則の箇条と記録の行の表 | 本計画の承認 |
| M2（PR-2） | `fable-review.md` の入口と「The merge check」、`task-pr-handoff.md` の step 1.5・2.5・3、`runplan.md`、`local-cli-review.md`、guard のテスト、ADR-0050 の食い違いの文の削除 | PR-1 のマージ（手順が読む `MERGE` と `unreviewed` が、`origin/main` のスクリプトにある） |
| M3（PR-3） | 実地の確かめの記録 | PR-2 のマージと、人が行うほかのタスクのタスク文書の PR での確かめ |

PR-1 のマージから PR-2 のマージまでの間は、今の手順が `MERGE` を形の違う 1 行目と読み、Fable が PR の全体を見る（設計書 8 章）。#987 で起きた形と同じ、安全な側の動きである。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 | 判定のスクリプトの `fable` のモードの変更と `unreviewed` のモード、スクリプトのテスト、`fable-review.md` の規則の箇条と記録の行の表、ADR-0050 の食い違いの文の書き直し | opus | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9 | `fable-review.md` の入口と「The merge check」、`task-pr-handoff.md`・`runplan.md`・`local-cli-review.md` の手順、guard のテスト、ADR-0050 の食い違いの文の削除 | opus | auto |
| PR-3 | 3-1 / 3-2 | 実地の確かめの記録 | sonnet | hold |

## 4. テスト戦略 (Test Strategy)

-   **スクリプトのテスト:** 判定のスクリプトの規則は、`TestReviewRecordsFable` と `TestReviewRecordsUnreviewed` が、一時的なリポジトリの上で、設計書 7.1 の件ごとに確かめる。どちらも、PATH の bash と `/bin/bash` 3.2 で走らせ、git の設定と環境を変えても出力が変わらないことを確かめる。fixture の記録の行は、手順の文書の表から読んだ形で作る（`readFableForms`）。表とスクリプトの正規表現が食い違えば、確認の記録で `VALID` になる件が失敗する。
-   **境界と失敗:** 確認の記録の名指しの 4 つの条件、規則 4 の 3 つの条件、広げた範囲の確かめの 2 つの条件、確認できない merge commit の 3 つの形には、それぞれ受け付けない側の件を置く（設計書 7.1）。`unreviewed` の失敗（引数の形、`origin/main` がない）は、`coverage` の失敗の件を移して確かめる。
-   **guard のテスト:** スクリプトの呼び方（`TestReviewRecordsCallSite`）、記録の行の接頭辞（`TestRecordLinePrefixesDistinct`）、記録の commit の前のメンションの確かめ（`TestRecordCommitsCheckMention`）、モードの名前（`TestReviewRecordsMode`）。
-   **後方互換:** `coverage` のモードは変えない。`TestReviewRecordsCoverage` の既存の件が、期待する出力を変えずに通ることで確かめる（要件書 2.3、4.4）。既存の記録の行の形は変えない（要件書 4.4）。
-   **読んで確かめる:** 入口の順序、救済、push の形などの手順の文は、PR のレビューで読む（設計書 7.2）。
-   **差分の記録:** AC-23 は、各 PR で `docs/tasks/0098_fable_partial_review/` の差分が空であることを記録する（§7）。
-   **実地の確かめ:** Opus のサブエージェントの起動とモデル ID の確かめは、手順の文だけでは確かめられないので、PR-2 のマージの後に実地で確かめる（設計書 7.3）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 判定のスクリプトの誤りで、確認していない merge commit や見ていない commit を `VALID` とする | 有効の側へ広げる変更ごとに、受け付けない側の件を置き、規則を壊して失敗することを確かめる（ステップ 1-9）。失敗は `MISSING <merge-base>` に倒す（設計書 1.1）。PR-1 と PR-2 は `opus` で実装する |
| PR-1 と PR-2 の間に、記録の後の merge で Fable が PR の全体を見直す | 意図した途中の状態で、今と同じ動きである（§3.1） |
| PR-2 のチェックポイントと Part B は、PR-2 自身が書き換えた途中の手順の文を読んで走る | 手順の文に従って止まったときは、報告して人に問う。止まった後の再開の手段は範囲外である（要件書 2.3） |
| 実地の確かめに要る、`origin/main` が進んだタスク文書の PR が、すぐには来ない | PR-3 を `hold` にし、人が確かめてから `/runplan` を送る（§1.3 の決定 3） |
| `git show --remerge-diff` の振る舞いが、利用者の git で設計書の実験と違う | 範囲を求める git の失敗は、確認せずに Fable へ回す（決定 12）。実地の確かめで違いを見つけたら、決定の変更として人と決める（ステップ 3-1） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

確かめの種類は次のとおりである。

-   `test`: Go のテストが、判定のスクリプトの振る舞いを確かめる。テストの場所は `cmd/yt2column/review_records_test.go`（以下、`rr_test`）である。
-   `static`: guard のテストが、コマンドの文書の値と構造を確かめる。
-   `diff`: 差分の記録を PR のレビューで読む（`docs/dev/conventions/requirements_process.md` §4「`diff` verification」）。
-   `manual`: PR のレビューで、その文書の該当箇所を読んで確かめる。コマンドの定義の手順の文だけを定める AC は、`manual` だけで確かめる（§1.2）。実地の確かめの結果は、PR-3 のレビューで読む。

| AC | 確かめ | 種類 | 実装 | PR |
|---|---|---|---|---|
| AC-01 | `rr_test::TestReviewRecordsCallSite`（「The validity decision」の 3 行の並び）。`task-pr-handoff.md` step 2.5 と `fable-review.md` Entry 4 を読む | static、manual | ステップ 2-1・2-4 | PR-2 |
| AC-02 | `rr_test::TestReviewRecordsFable`（印の後に CHECKS の直しだけがあるときの `VALID`）。Entry 4 の最初の段が commit を作らないことを読む | test、manual | ステップ 1-1・1-5・2-1 | PR-1、PR-2 |
| AC-03 | `rr_test::TestReviewRecordsFable`（`MISSING <START>` の件と失敗の件）。Entry 4 の救済と、FABLE の渡し方を読む | test、manual | ステップ 1-1・1-5・2-1・2-4・2-5 | PR-1、PR-2 |
| AC-04 | `task-pr-handoff.md` step 2.5 の再チェックの順序を読む | manual | ステップ 2-4 | PR-2 |
| AC-05 | `task-pr-handoff.md` step 2.5・step 3（D の名指しの push、HEAD の確かめ、判定のやり直し）と AFTER_CREATE の前の確かめを読む | manual | ステップ 2-4 | PR-2 |
| AC-06 | Entry 4 の最後の段と `task-pr-handoff.md` step 2.5 の止まり方と報告の内容を読む | manual | ステップ 2-1・2-4 | PR-2 |
| AC-07 | Entry 4 と単独の経路の `not run` の扱いを読む | manual | ステップ 2-1 | PR-2 |
| AC-08 | Entry 4 の、救済の途中で止まったときの扱いを読む | manual | ステップ 2-1 | PR-2 |
| AC-09 | `runplan.md` の AFTER_REMEDY を読む | manual | ステップ 2-5 | PR-2 |
| AC-10 | `rr_test::TestReviewRecordsFable`（「README.en.md after the record」と、設計書 7.1 の `README.en.md` だけの commit の件） | test | ステップ 1-1・1-5 | PR-1 |
| AC-11 | `rr_test::TestReviewRecordsFable`（記録 → 印 → CHECKS の直し、`checkpoint`）。`task-pr-handoff.md` step 1.5 を読む | test、manual | ステップ 1-5・2-4 | PR-1、PR-2 |
| AC-12 | `rr_test::TestReviewRecordsFable`（記録 → 中身のある commit → 印 → CHECKS の直し） | test | ステップ 1-5 | PR-1 |
| AC-13 | `rr_test::TestReviewRecordsFable`（記録 → merge commit、記録 → 印 → merge commit、確認できない merge commit の 3 つの形）。Entry 3 の `MERGE` を読む | test、manual | ステップ 1-1・1-5・2-1 | PR-1、PR-2 |
| AC-14 | `rr_test::TestReviewRecordsFable`（中身のある commit X の後の merge、START がブランチの commit の記録の後の merge、「range of the record's commit beyond it」） | test | ステップ 1-1・1-5・1-6 | PR-1 |
| AC-15 | `rr_test::TestReviewRecordsFable`（起動の後、記録の commit の前の merge commit）。Entry 4 の最後の段で止まることを読む | test、manual | ステップ 1-5・2-1 | PR-1、PR-2 |
| AC-16 | `rr_test::TestReviewRecordsUnreviewed`（`README.en.md` だけの commit・CHECKS の直しを挙げ、見た範囲に入る merge commit を挙げない件）、`rr_test::TestReviewRecordsCallSite`（Part B step 3 の 3 行の並び） | test、static | ステップ 1-2・1-7・2-4・2-6 | PR-1、PR-2 |
| AC-17 | `rr_test::TestReviewRecordsUnreviewed`（#987 の形、START が古い merge-base で END が merge commit の後の見た範囲の行） | test | ステップ 1-2・1-7 | PR-1 |
| AC-18 | `rr_test::TestReviewRecordsUnreviewed`（チェックポイントの印 → merge commit） | test | ステップ 1-2・1-7 | PR-1 |
| AC-19 | `rr_test::TestReviewRecordsUnreviewed`（単独の経路の直しで、見た範囲の行が直しを含む件） | test | ステップ 1-2・1-7 | PR-1 |
| AC-20 | `task-pr-handoff.md` step 3 の Fable の部分レビューの節を読む | manual | ステップ 2-4 | PR-2 |
| AC-21 | ADR-0050 の「決定」を読み、食い違いの文が残っていないことを確かめる。形は `cmd/yt2column/adr_test.go::TestADRRecords` が確かめる | manual | ステップ 1-10・2-8 | PR-1、PR-2 |
| AC-22 | ADR-0050 の「採らなかった案」と「経緯」を読む（設計の PR #1049 が更新済み） | manual | なし（設計の PR で済み） | PR-2 |
| AC-23 | 差分の記録（AC-23）。比べるパス: `docs/tasks/0098_fable_partial_review/`。各 PR で記録する | diff | ステップ 1-11・2-9・3-2 | PR-1、PR-2、PR-3 |
| AC-24 | `rr_test::TestReviewRecordsFable`（`MERGE <n>` の件）。Entry 3・Entry 4 の救済と、「The merge check」の範囲を求められないときの扱いを読む。実地の確かめ（ステップ 3-1） | test、manual | ステップ 1-1・1-5・2-1・2-2・3-1 | PR-1、PR-2、PR-3 |
| AC-25 | 「The merge check」のモデルとモデルの確かめを読む。実地の確かめ（ステップ 3-1） | manual | ステップ 2-2・3-1 | PR-2、PR-3 |
| AC-26 | 「The merge check」の範囲の一覧を読む | manual | ステップ 2-2 | PR-2 |
| AC-27 | `rr_test::TestReviewRecordsFable`（名指しの 4 つの条件、形の違う行、12 桁の名指し。行は `readFableForms` が表から読んだ形で作る） | test | ステップ 1-1・1-3・1-4・1-5 | PR-1 |
| AC-28 | `rr_test::TestReviewRecordsFable`（`not run` の確認の記録で `VALID`）。「The merge check」の `not run` の扱いを読む | test、manual | ステップ 1-5・2-2 | PR-1、PR-2 |
| AC-29 | 「The merge check」の制約と状態の確かめを読む | manual | ステップ 2-2 | PR-2 |
| AC-30 | `rr_test::TestReviewRecordsFable`（規則 4 の件。M より前の commit、確認の記録の commit の祖先でない commit、`not run` の範囲だけの直し）。「The merge check」のトリアージと直しの確かめを読む | test、manual | ステップ 1-1・1-5・2-2 | PR-1、PR-2 |
| AC-31 | `rr_test::TestRecordCommitsCheckMention`（「The merge check」）。`task-pr-handoff.md` step 3 の merge の後の確認の節を読む | static、manual | ステップ 2-2・2-4・2-6 | PR-2 |
| AC-32 | `rr_test::TestReviewRecordsFable`（START が古い merge-base の記録の後の merge、範囲が `main` 側の commit を含む記録を読まない件） | test | ステップ 1-1・1-5・1-6 | PR-1 |

### 差分の記録

AC-23 は、比べるパスの差分が空であることを確かめる。差分が小さい文書の変更なので、スクリプトではなく、記録した差分を読む形にする（`requirements_process.md` §4「Recorded diff reading」）。比べるパスに本計画は入らない。

Diff record (AC-23):

PR-1:

- Compared paths: `docs/tasks/0098_fable_partial_review/`

PR-2:

- Compared paths: `docs/tasks/0098_fable_partial_review/`

PR-3:

- Compared paths: `docs/tasks/0098_fable_partial_review/`

### 実地の確かめの記録

（PR-3 で書く）

## 8. 成功の基準 (Success Criteria)

-   **機能:** 要件書の AC-01〜AC-32 が、§7 の確かめで満たされる。
-   **品質:** 有効の側へ広げる変更のそれぞれに、受け付けない側の件があり、規則を壊すと失敗することを確かめてある（ステップ 1-9）。`make test` と `make lint` が通る。
-   **セキュリティ:** 判定のスクリプトを `origin/main` のコピーでだけ走らせることが、`TestReviewRecordsCallSite` で確かめられる。確認のサブエージェントの出力の文を、記録にも PR の説明にも写さないことが、PR-2 のレビューで読まれる（設計書 5.2）。
-   **文書:** ADR-0050 に、#988 にリンクする食い違いの文が残らない。

## 9. 次のステップ (Next Steps)

-   PR-3 のマージの後、issue #988 を閉じる（`/runpr` が、タスクの最後の計画の PR のマージの後に扱う）。
-   止まった後の再開の手段と、PR を開いた後の merge の扱いは、範囲外として残る（設計書 9 章）。必要になったら、別の issue で扱う。
