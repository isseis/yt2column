# 実装計画書：Fable の部分レビューの記録の後、push の前にできた commit を、PR を開く前に確かめる

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-20 |
| Review date | 2026-11-20 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を指す。節番号と「決定 N」は [02_architecture.md](02_architecture.md)（以下、設計書）の節と 1.2 の決定を、H-NN は [design_handoff.md](design_handoff.md) の項番を指す。H-NN の扱いは、どれも設計書 10 章が記録している。本タスクに `implementation_handoff.md` はない。

既存のファイルの内容と `file:line` の行番号は、コミット `4d889096`（`origin/main`）で確かめた。設計書が確かめたコミットは `0051d3f4` である。本計画が変えるファイルのうち、その後に変わったのは次の 2 つで、設計書が引く行番号は今も合う（`git diff --stat 0051d3f4 4d889096 -- .claude/commands/ cmd/yt2column/review_records_test.go docs/dev/adr/0050-fable-partial-review.md`）。

-   `_lib/local-cli-review.md`: 93 行目の 1 文が延びただけである。
-   ADR-0050: 設計の PR（#1049）が更新した。

用語は、要件書 3.1・6 章と設計書の冒頭（判定のスクリプト・ブランチ・END・救済・判定した commit（D）・確認できる merge commit）と同じ意味で使う。大文字の名前（FABLE・CHECKPOINT・AFTER_REMEDY・ENTRY・PLAN・RECORD など）は、設計書 3.3.1・3.4.1 と、共通の手順（`.claude/commands/_lib/` の文書）が定める入力の名前である。本計画は、次の用語を加える。

-   **スクリプトのテスト:** 判定のスクリプトを一時的な git のリポジトリの上で走らせる Go のテスト。`cmd/yt2column/review_records_test.go` にある。
-   **確認できる merge の fixture:** スクリプトのテストで、`origin/main` を進めて（`commit-tree` と `update-ref`）その commit をブランチに merge し、できた merge commit が確認できる merge commit であることを確かめてから返す fixture の関数（ステップ 1-4）。今の `rrFixture.merge` は、`origin/main` から届かないブランチを merge するので、確認できない merge commit を作る。
-   **実地の確かめ:** PR-2 のマージの後、ほかのタスクのタスク文書の PR で、merge の後の確認を本物のセッションで走らせて確かめること（設計書 7.3）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

判定のスクリプトに、merge の後の確認の記録の読み方と `unreviewed` のモードを足す。あわせて、`_lib/fable-review.md`・`_lib/task-pr-handoff.md`・`runplan.md`・`_lib/local-cli-review.md` の手順に、push の直前の判定と merge の後の確認を足す。これで、Fable の記録の後、push の前にできた commit を、PR を開く前に確かめるようにする（要件書 2.1）。何をどこに書くかは設計書 3 章が定める。本計画は、作業の順序と確かめ方を定める。

### 1.2. 実装の原則

-   **文面は設計書 3 章に従い、英語で書く。** コマンドの定義、`_lib/`、スクリプト、Go のテストは英語で書く。判定のスクリプトの日本語のリテラルは、今のもの（計画書のチェックボックスの文）だけで、本タスクでは足さない。各ステップは、書く内容を設計書の節で指す。文面は実装の PR で書く。
-   **判定のスクリプトの規則は、スクリプトのテストで確かめる**（設計書 7.1）。テストケースごとに、実装者がスクリプトの該当する規則を実際に壊して失敗することを確かめ、commit のメッセージに書く。ほかの確かめが裏打ちしていて、1 つだけ壊しても失敗する件を作れない条件は、その条件と裏打ちする確かめをテストのコメントと commit のメッセージに書く（今の `TestReviewRecordsCoverage` のコメントと同じ扱い）。
-   **コマンドの定義の手順の文だけを定める AC は、PR のレビューで読んで確かめる**（設計書 7.2 の「文の内容」）。設計書 7.2 は、入口の順序、救済、push の形「など」の文を、文書のテストで確かめないと決めている。確認のモデルとその確かめ、範囲を求める git の命令も、手順の文の中の値で、本計画はこの「など」に含めて読む。モデルの確かめは実地の確かめ（ステップ 3-1）でも確かめる。値と構造のうち guard のテストで確かめるのは、スクリプトの呼び方、記録の行の形と接頭辞、メンションの確かめの置き場所である（設計書 7.2）。
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
    -   PR-1 は、ADR-0050 の食い違いの文も、残る食い違いだけを述べる形に書き直す（ステップ 1-12）。食い違いの一部を直す PR は、その文を残る部分に書き直すと `docs/dev/conventions/adr.md`「Discrepancy statements」が定めているためで、設計書 8 章の順序を変えるものではない。
    -   採らなかった案：PR-1 はスクリプトとテストだけにし、テストは確認の記録の形をいったん Go のリテラルで持ち、PR-2 で表から読む形に移す。テストを 2 度書くことになり、PR-1 の間は正本の規則とスクリプトが食い違う。
    -   ADR：なし。PR の受け持ちの分け方で、本タスクの中で閉じる。
3.  **実地の確かめは、設計書 7.3 のとおり、ほかのタスクのタスク文書の PR で行う。** PR-2 のマージの後、ほかのタスクのタスク文書のコマンドの実行で、人が次のように指示する。Part B step 1 の記録の commit の後、step 1.5 の前に `origin/main` を merge させる。そのうえで、step 1.5 が merge の後の確認を 1 回走らせ、Fable が走らないことを確かめる。結果は PR-3 で本計画の §7 に記録する。`origin/main` が、そのブランチを作った後に進んでいる必要があるので、機会が来るまで PR-3 の `/runplan` を送らない（`hold`）。人の答え：別タスクの文書 PR（推奨）。
    -   採らなかった案：PR-3 の `/runplan` 自身で、最後の phase group の記録の後に `origin/main` を merge し、チェックポイントの入口で確かめる。`/runplan` の経路の確かめになり、設計書 7.3 の「タスク文書の PR」と食い違うので、設計書の直しが要る。
    -   ADR：なし。確かめ方の選択である。
4.  **`unreviewed` のモードの引数は `<HEAD> [<PLAN>]` とし、`coverage none` から LAST_REVIEWED を除いた形にする。** `<PLAN>` は `coverage` と同じく省ける。これで、`unreviewed` と `coverage none` の違いは、設計書 3.2 の 2 点だけになる。Part B step 3 の呼び出しは、設計書 3.4.4 のとおり `<PLAN>` をいつも渡す。タスク文書の PR の時点で計画書がまだなくても、パスは決まる（`fable-review.md:41-42` と同じ扱い）。
    -   経緯：最初の版は `<PLAN>` を必ず渡す形にしていた。計画のレビューで、3 つ目の違いになり、設計書 3.2 の「2 点だけが違う」と食い違うと分かったので、この形にした。
    -   ADR：なし。スクリプトの引数の形で、本タスクの中で閉じる。
5.  **merge の後の確認の指摘の場所の規則を、Fable の規則に範囲の一覧のパスを加えたものにする（設計書の決定 13、要件書 4.2）。** merge-base から HEAD までに変わったファイル（Fable の規則）か、範囲の一覧にあるパスなら、有効な場所とする。計画のレビューで、Fable の規則のままでは、衝突を `main` 側に寄せて解いたファイルへの指摘が INVALID になると分かった。最初は「範囲の一覧のパスか、merge commit が親のどちらかに対して変えたパス」としたが、Fable の部分レビューで、親に対する差分は PR が触れていないファイルまで加え、Fable の規則も置き換えてしまうと分かったので、狭めた。要件書 4.2 の「受け入れる形は Fable と揃える」を変えるので、要件書と設計書を `draft` に戻した。人の答え：規則を変える（推奨）、続けて、狭めて要件も draft に戻す（推奨）。
    -   ADR：なし（設計書の決定 13 の判断のとおり）。

### 1.3.1. 実装の際に決めたこと（2026-11-23）

本計画承認後にタスク 0118（`#1077`）が `_lib/task-pr-handoff.md` Part B にローカル CLI 未レビュー分の補完（`step 2.5`）を追加したため、フェーズ 2 の実装前に人と確認して次の 2 点を決めた（セッション中の再承認。詳細は設計書 1.2.1）。

1.  **Part B の順序とステップ番号:** 既存のタスク 0118 の手順を `step 2.5` のまま残し、本タスクの push の直前の判定（Entry 4）を **`step 2.6`** として `step 2.5` の直後・`step 3` の直前に置く。これに合わせて、`TestReviewRecordsCallSite` の `handoff step 2.5` の終端を `"\n2.6. "` に直し、`handoff step 3` に `unreviewed` の 3 行の並びを足す。人の答え：既存を `step 2.5` のまま残し Entry 4 を `step 2.6` とする（推奨）。
    -   ADR：なし（設計書 1.2.1 の判断のとおり）。
2.  **`step 2.6` の救済後の再チェックで `step 2.5` を走らせ直さない:** `step 2.6` の再チェックは `AFTER_REMEDY` → README の同期 → CHECKS → 最終段のみとし、`step 2.5` は走らせ直さない。人の答え：走らせ直さない（推奨）。
    -   ADR：なし（設計書 1.2.1 の判断のとおり）。（この判断は 1.3.2 で改めた。）

### 1.3.2. 実装の際に決めたこと（2026-11-24）

フェーズ 3 の実地の確かめで、merge の後の確認が、ADR-0013 の決定と step 2.6 の振る舞いの食い違いを見つけた。これを受けて、次の 2 点を人と決めた（セッション中の再承認。詳細は設計書 1.2.2）。

1.  **ADR-0013 と ADR-0042 の例外:** Part B step 2.5 の後にできる commit（step 2.6 の救済の後の再チェックと、step 3 の CHECKS の直し）をローカルの CLI レビューにかけ直さない例外を、ADR-0013 と ADR-0042 の決定に書き足す。1.3.1 の item 2 の「ADR：なし」は、この点で正しくない。2 つの ADR の決定・採らなかった案・経緯（ADR-0013 は受け入れた危険も）は、PR-3 で訂正する。最初はフォローアップの issue にすると答えたが、レビューで、決定を変える PR は同じ PR で ADR を更新するという `adr.md`「Approval and the PR」の規則が分かったため、PR-3 で訂正する。ADR-0042 も同じ例外で狭まることは、Fable の部分レビューの指摘で分かった。人の答え：PR-3 で訂正し、ADR-0042 にも同じ例外を足す（推奨）。
2.  **AC-05 の確かめ:** 実地の確かめでは Part B step 3 の push を走らせず、Part B step 1.5 からの Entry 3 の呼び出しも観測しなかった（§7 の実地の確かめの記録）。AC-05 の確かめから実地の確かめを外し、PR-2 のレビューでの手順の読みだけで確かめたものとする。step 1.5 の呼び出しも同じである。実地で確かめ直す issue も作らない。人の答え：読みだけで受け入れる（推奨）。

### 1.4. 既存コード調査結果

**判定のスクリプト（`.claude/commands/_lib/review-records.sh`）。**

-   **あるもの:**
    -   モードの振り分け（`:584-596`）と、失敗のときの出力（`fail`、`:179-186`）。
    -   範囲の行の確かめ `checked_range`（`:416-425`）。START の条件は `:423` にある。
    -   中身のない commit の見分け `contentless`（`:369-389`）、空の commit の見分け `empty_commit`（`:392-396`）、チェックボックスの commit の見分け `checkbox_only`（`:295-302`）。
    -   `fable` のモードの本体（`:505-582`）。記録を探す処理（`:517-531`）、同じ実行の直し（`:533-542`）、印の後の commit（`:544-562`）、判定と START（`:564-581`）からなる。merge commit は、`parent_count` が 1 でないので、どの規則にも当たらない（`:568-573`）。
    -   `coverage` のモードの本体（`:427-489`）。
    -   ヘッダーの「Where the rules are defined」の段落（`:122-137`）。記録の行の形の定義の置き場所と、`coverage` の規則の根拠（タスク 0098 の設計書）を述べる。
-   **足りないもの:**
    -   merge の後の確認の記録の正規表現と、その読み方。
    -   確認できる merge commit の見分け。
    -   規則 4。
    -   出力 `MERGE <n>`。
    -   `unreviewed` のモード。
-   **変えるもの:**
    -   `checked_range` の START の条件。`fable` と `unreviewed` のモードだけで広げ、`coverage` では今のままにする（設計書 3.1.4）。
    -   START の決め方（`:574-577`、設計書 3.1.5）。
    -   ヘッダーの `ENTRY` の説明（`:76`）、`fable` の契約（`:86-120`）、使い方の行（`:5-6`）、「Where the rules are defined」の段落。
    -   `fail` が `unreviewed` で `PARTIAL` を出すこと。
-   **再利用する:** `coverage` と `unreviewed` は、判定の関数を 1 つ共有し、設計書 3.2 の 2 点だけを引数で選ぶ。`resolve12`・`in_list`・`message`・`parent_count`・`load_branch`・`has_line` は、そのまま使う。

**スクリプトのテスト（`cmd/yt2column/review_records_test.go`）。**

-   **あるもの:**
    -   fixture `rrFixture`（`:91-196`）。merge のための `merge`（`:156-160`）と `side`（`:164-174`）を持つ。どちらも `origin/main` から届かないブランチを作って merge するので、できる merge commit は、決定 12 の確認できない merge commit である。`origin/main` を進める例は、`commit-tree` と `update-ref` を使う件（`:930-956`）にある。
    -   `fableCases`（`:802-`）と `TestReviewRecordsFable`（`:1146-1200`）。各件を `scriptBashes` の各 bash で走らせる。git の設定を変えた走らせ方（`rrConfigScenarios`）は、merge のない `rrFableConfigBranch`（`:1122-1132`）の上でだけ走る。
    -   `coverageCases`（`:231-`）と `TestReviewRecordsCoverage`（`:652`）。約 25 件が `rrNone(f, "")` で `["none", HEAD]` を返す。LAST_REVIEWED に固有の件（「LAST_REVIEWED limits the commits judged」など）と、引数の形の失敗の件がある。`coverage` には、START が古い merge-base の見た範囲の行を、`origin/main` が進んだ後に読む件がない。
    -   `readFableForms`（`:735-749`）、`TestReviewRecordsMode`（`:715-725`、コメントの「a mode other than coverage and fable」を含む）、`TestReviewRecordsCallSite`（`:1234-1260`。走らせる節を見出しで探す）、`TestRecordLinePrefixesDistinct`（`:1318-1360`。`fable-review.md` の接頭辞の数の下限が 3、`:1347`）、`TestRecordCommitsCheckMention`（`:1365-1389`）。
    -   `fable_state_docs_test.go` の `TestFableStateCheckBlock` は、`fable-review.md`「The state check」のコードブロックを確かめる。「The merge check」が状態の確かめをこの節への参照で使えば、このテストのままで足りる。
-   **足りないもの:**
    -   確認の記録の行を作る fixture の関数。
    -   確認できる merge の fixture。
    -   確認できない merge commit の 3 つの形を作る関数（親が 3 つ、第 1 親が `main` 側、第 2 親が `origin/main` にない）。
    -   `unreviewed` のテスト。
-   **期待する出力が変わる既存の件:** 設計書 3.7 の 3 件である。
    -   「merge after the record」（`:1007-1014`）は、`side` のブランチ（`origin/main` から届かない）を merge するので、決定 12 の確認できない merge commit に当たり、出力は `MISSING <merge-base>` になる。START が merge-base になる理由は、3.1.5 の「記録の後に merge commit があれば merge-base」である。設計書 3.7 はこの件の見込みを `MERGE 1` と書いていた。決定 12 と食い違う表のセルなので、本計画の PR で、設計書を編集上の訂正として直した（決定の変更なし）。
    -   「range of the record's commit beyond it」（`:918-928`）は、範囲の行の確かめを通らない side の中身のある commit s1 が記録を無効にするので、もとから `MISSING` である。START が `HEAD~2` から merge-base に変わる理由は、3.1.5（AC-14）である（設計書 3.7）。
    -   merge を作るほかの件（`f.merge` と `git merge` を呼ぶ件）も、実装の PR で走らせ直して確かめる（設計書 3.7）。

**手順の文書。**

-   **`_lib/fable-review.md`:**
    -   入口の表（`:13-19`）は 3 つで、「three entry points」とある。「Two task PRs pass FABLE」の段落（`:21-26`）と、チェックポイントの入口を KIND `implementation` に固定する FABLE の段落（`:45-47`）がある。
    -   有効な記録を見つけたときだけ起動しない、と述べる文が 2 か所ある（`:62-64`、`:167-168`）。
    -   「Authorization and constraints」（`:66-77`）は、Fable についてだけ述べる。
    -   「The validity decision」（`:79-135`）は、規則の箇条（`:111-128`）、読み方（`:94-106`）、「A merge commit is never covered」（`:134-135`）を持つ。
    -   単独の経路（`:208-232`）は、呼ぶのがチェックポイントの入口だけで（`:210`）、RANGE は有効かの判定から取る（`:213-214`）と述べる。
    -   このほか、Entry 1（`:137-163`）、Entry 3（`:234-248`）、「Checking the output」（`:330-357`。step 3 が指摘の場所を merge-base から HEAD までに変わったファイルに限る）、「Record lines」（`:437-525`）、「Reading the record lines」（`:527-546`。呼ぶ側が `origin/main..HEAD` から読むと述べる）がある。「The merge check」の節と Entry 4 はない。
-   **`_lib/task-pr-handoff.md`:**
    -   Part B は step 1〜7 からなる。step 1.5 と step 2.5 はない。
    -   step 1 は FABLE を自分で組み立てる（`:184-197`）。止まったときは「stop before step 2」とある（`:196`・`:205`）。
    -   step 3 は、ブランチ名で push する（`:218`）。どちらの CLI もレビューしなかった commit の文（`:250-260`）は、セッションの SUMMARY を使う。
    -   Authorization の段落（`:61-71`）は、`git push`（`-u` を含む）を挙げる。
    -   `postingSections` の表（`codex_review_docs_test.go:74-78`。`TestPostingPathsCheckMention` が読む）は、`itemSection` で、step 3 の書き出し `3. Push and create the PR for the current branch.` と step 4 の書き出し `4. Run AFTER_CREATE` の間を節とする。`TestHandoffListingPrintsNoBody`（`fixpr_docs_test.go:197`）は、step 5 と step 6 の書き出しで節を探す。どちらの書き出しも変えない（設計書 3.4.4 の「step 番号」）。
-   **`runplan.md`:** step 5a のチェックポイントの段落（`:98`）、Part B の呼び出し（`:104-149`）、step 8 のチェックポイントと、計画書に PR マーカーがないときの Part B の呼び出し（`:263`。「as step 5a does (with its CHECKS, LOCAL_REVIEW, and Opus review result of PR_TEXT)」と、受け継ぐ入力を挙げる）、B1 の文（`:279-280`）、step 5a の LOCAL_REVIEW の文（`:120-123`）がある。
-   **`_lib/local-cli-review.md`:** 呼び出し元の一覧（`:10-23`、「Seven callers」）、RECORD の表（`:70-79`）、ローカルの CLI レビューのものでない記録の行の一覧（`:424-426`）がある。
-   **ADR-0050:** 決定の節 5 の後の食い違いの文（`docs/dev/adr/0050-fable-partial-review.md:84`）が、#988 にリンクしている。
-   **変えない文書:** `runpr.md`・`opus-review.md`・`mk*.md` は変えない（設計書 2.2）。`CLAUDE.md:224-235` と `_lib/simple-process.md:232` は、Fable の部分レビューを要約するだけで、入口の数や push の形を述べていない。それでも PR-2 で読み直し、食い違いが出ないことを確かめる（ステップ 2-7）。

**型の生成の経路:** 本タスクに、具体的な型で振り分ける設計はない（N/A）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 判定のスクリプトとそのテスト（設計書 8 章の 1）

**変更するファイル:** `.claude/commands/_lib/review-records.sh`、`cmd/yt2column/review_records_test.go`、`.claude/commands/_lib/fable-review.md`（「The validity decision」の規則の箇条と「Record lines」の表だけ）、`docs/dev/adr/0050-fable-partial-review.md`（食い違いの文だけ）、本計画

-   [x] **ステップ 1-1**: `fable` のモードを変える（設計書 3.1）。
    -   範囲の確かめで、START の条件を広げる（3.1.4）。Fable の記録、同じ実行の直しの範囲の行、規則 4 の見た範囲の行に当てる。`coverage` の確かめは変えない。
    -   merge の後の確認の記録を読む（3.1.3）。`Merge check: run` と `Merge check: not run` の形の正規表現を足す。名指しした ID の 4 つの条件を確かめる。ID が何のオブジェクトも指さないときと、commit でないオブジェクトを指すときは、その行を数えずに読み進め、失敗にしない。
    -   確認できる merge commit を見分ける（用語、決定 12）。
    -   規則 4 を足す（3.1.2）。
    -   結果を `MISSING`・`MERGE <n>`・`VALID` の順で決める（3.1.2、3.1.1）。
    -   START を決め直す（3.1.5）。
    -   ヘッダーの使い方、`ENTRY` の説明（決定 5）、`fable` の契約、「Where the rules are defined」の段落（`Merge check:` の行の形の定義が `fable-review.md` にあること）を合わせる。
-   [x] **ステップ 1-2**: `unreviewed` のモードを足す（設計書 3.2、§1.3 の決定 4）。引数は `<HEAD> [<PLAN>]` とする。`coverage` と判定の関数を共有し、3.2 の 2 点（範囲の確かめと merge commit の扱い）だけを引数で選ぶ。ヘッダーは `unreviewed` を「`coverage none` に 2 点を当てたもの」と定義し、規則を 2 度書かない。「Where the rules are defined」の段落に、`unreviewed` の 2 点の根拠が本タスクの設計書 3.2 にあることを足す。失敗のときは `PARTIAL` だけを出して、終了コード 1 で終わる。
-   [x] **ステップ 1-3**: `_lib/fable-review.md` を変える（§1.3 の決定 2）。
    -   「The validity decision」の規則の箇条に、規則 4、確認できる merge commit と確認済みの merge commit の扱い、広げた範囲の確かめ、START の決め方を足す（設計書 3.1.2〜3.1.5）。「A merge commit is never covered」（`:134-135`）を、それに合わせて直す。出力の読み方（`:94-106`）は PR-2 で変える。
    -   「Record lines」の表に、`Merge check:` の 4 行を足す。値（`<reason>`・`<criterion>`）の説明も足す（設計書 3.3.6）。
-   [x] **ステップ 1-4**: スクリプトのテストの fixture を足す。
    -   `readFableForms` が `Merge check: run` と `Merge check: not run` の形を表から読む。
    -   確認の記録の行を作る関数を足す。
    -   確認できる merge の fixture を足す。merge commit の親がちょうど 2 つで、第 1 親がブランチの commit か merge-base、第 2 親が `origin/main` から届くことを確かめ、違えば `t.Fatal`（「so the case checks nothing」の形）で止める。
    -   確認できない merge commit の 3 つの形を作る関数を足す（§1.4）。
-   [x] **ステップ 1-5**: `fableCases` に、設計書 7.1 の `fable` のモードの表の件を足す。足し方には、次の 6 つの決まりがある。
    -   確認できる merge commit を使う件は、確認できる merge の fixture で作る。
    -   受け付けない側の件（名指しを数えない、範囲の行を読まない、規則 4 に当てない）は、数えれば出力が変わる配置で作り、期待する出力を 1 行として書く。可能なら、正しい形の行で同じ配置を作ると出力が変わることを先に確かめ、変わらなければ `t.Fatal` で止める。たとえば、merge commit でない commit N の名指しは、記録 → 中身のない commit N（空の commit）→ 中身のある commit X → N を名指しする確認の記録（見た範囲が X を含む）の配置で、`MISSING` を期待する。名指しを除いた同じ配置が `MISSING` を返し、名指しを数えれば X が規則 4 で除かれる配置であることを、先に確かめる。
    -   名指しの件に、何のオブジェクトも指さない 40 桁の ID と、tree の ID を名指しする件を足す。確認できる merge commit の横に置き、名指しを数えない出力（`MERGE 1`）と終了コード 0 を期待する。
    -   AC-14 は、merge の前の記録の START が古い merge-base の件と、ブランチの commit の件の 2 つを置く。どちらも、記録 → 中身のある commit X → 確認できる merge commit の順にし、`MISSING <新しい merge-base>` を期待する。それぞれ、X を除いた同じ配置が `MERGE 1` を返すこと（記録が読まれていること）を先に確かめ、違えば `t.Fatal` で止める。
    -   設計書 7.1 の件が既存の件と同じ配置と出力になるとき（「README.en.md after the record」、ステップ 1-6 で直す「record from a merge-base origin/main moved past」）は、新しい件を足さず、既存の件をその行の確かめとして使う。
    -   決定 12 の「第 2 親が `origin/main` にない merge commit」の件は、ステップ 1-6 で直す「merge after the record」と同じ形になる。違いが確認の記録の有無だけなら 1 つにまとめ、確認の記録を持つ形を残す（確認できない merge commit を名指しする記録を数えないことを確かめるため）。残す件が何を確かめるかをコメントに書く。
-   [x] **ステップ 1-6**: 期待する出力が変わる既存の件を、それぞれ直す（設計書 3.7）。
    -   [x] 「record from a merge-base origin/main moved past」: `MERGE 1` と、その merge commit（設計書 3.1.2 の M）の ID にする。
    -   [x] 「merge after the record」: `MISSING <merge-base>` にする。件の前のコメントも、`MISSING` の理由が確認できない merge commit であること（決定 12）と、START が merge-base になる理由（3.1.5）を述べる形に直す。
    -   [x] 「range of the record's commit beyond it」: `MISSING <merge-base>` にする。コメントに、START が merge-base になる理由が 3.1.5 であることを書く。
    -   [x] merge を作るほかの件を走らせ、期待する出力が変わらないことを確かめる。出力が変わらない件ごとに、新しい規則のどれがその出力を作るかをコメントに書く。「parent of the first missing commit behind the merge-base」（`:930-946`）は、START を merge-base に戻す分岐（`review-records.sh:574-577`）ではなく決定 12 か 3.1.5 の規則で merge-base になる。その分岐に届く件を残せるならそれを足し、届かなくなったなら、その分岐が守りのためだけに残ることをスクリプトのコメントと commit のメッセージに書く。
-   [x] **ステップ 1-7**: `unreviewed` のテスト `TestReviewRecordsUnreviewed` を足す。
    -   `coverageCases` を再利用する。setup が返す引数の先頭が `none` の件だけを取り、`none` を除いて `unreviewed` の引数にする関数を通す。先頭が `none` でない件（LAST_REVIEWED に固有の件と、LAST_REVIEWED の形の失敗の件）は、対象にしない。
    -   3.2 の 2 点で期待する出力が変わる件は、走らせる前に設計書 3.2 から期待する出力を決めて上書きする。上書きした件と、それを変えた 3.2 の点を、commit のメッセージに書く（たとえば「merge commit inside a seen range」は `ALL` になる）。
    -   設計書 7.1 の `unreviewed` の表の件を足す。merge を使う件は、確認できる merge の fixture で作る。「見た範囲の行の範囲に入る merge commit」は、上書きした「merge commit inside a seen range」で兼ねる。
    -   `unreviewed` の引数の失敗の件（引数がない、引数が多すぎる、HEAD の形が違う、PLAN の形が違う）は、再利用した `coverage` の件から `none` を除いた形（「too few arguments」「too many arguments」「HEAD of another form」「PLAN of another form」など）で確かめ、同じ件を別に足さない。
    -   `coverage` と同じく、各 bash と、環境を変えた走らせ方で確かめる。
-   [x] **ステップ 1-8**: `coverage` が広げた範囲の確かめを使わないことを固定する件を、`coverageCases` に足す（要件書 2.3）。START が古い merge-base の見た範囲の行の後に、確認できる merge commit を置く（#987 の形）。範囲の commit を `PARTIAL` に挙げることを期待する。
-   [x] **ステップ 1-9**: git の設定と、置き換えの ref の件を、新しい読み方にも当てる。
    -   確認できる merge commit と確認の記録を持つ分岐の上で、`rrConfigScenarios` を走らせる。基準の出力は `MERGE 1` か、確認の記録のある `VALID` とする。
    -   置き換えの ref（`refs/replace/`）で、確認できない merge commit を確認できるように見せる件を足し、名指しを数えないことを確かめる（`coverage` の「replace ref」の件と同じ扱い）。
-   [x] **ステップ 1-10**: guard のテストを直す（設計書 3.7）。
    -   `TestReviewRecordsMode` のモードの一覧に `Unreviewed` を足し、コメントを 3 つのモードに合わせる。
    -   `TestRecordLinePrefixesDistinct` が、`Fable ...` の接頭辞と `Merge check ...` の接頭辞も互いに比べるようにする。同じ族の中の同じ接頭辞（`Fable review:` が 3 行にある）は比べない。読んだ接頭辞の数の下限を、足した行に合わせて上げる。
-   [x] **ステップ 1-11**: ステップ 1-5〜1-10 の各件について、スクリプトの該当する規則か、文書の行の形を壊すと失敗することを確かめ、commit のメッセージに書く。§1.2 のとおり、ほかの確かめが裏打ちする条件は、その旨と裏打ちする確かめを書く。たとえば、広げた範囲の確かめの「START が `origin/main` から届く」は、今の「START が END の祖先」と「END がブランチの commit」が裏打ちしうる。規則 4 の「確認の記録の commit（設計書 3.1.2 の Q）の祖先」は、`checked_range` の「END が Q の真の祖先」が裏打ちしうる。
-   [x] **ステップ 1-12**: ADR-0050 の食い違いの文（`:84`）を、残る食い違いだけを述べる形に書き直す。
    -   PR を作る前に、`git grep -nE 'issues/988([^0-9]|$)' origin/main -- docs/dev/adr/` で、`origin/main` の文を探す（`docs/dev/conventions/adr.md`「Discrepancy statements」）。
    -   判定のスクリプトの読み方（merge の後の確認の記録、広げた範囲の確かめ、START の決め方）は PR-1 で解消する。残るのは手順の部分で、次の 3 つを述べる。
        -   タスク文書の PR はまだ印を残さないので、CHECKS の直しが記録を無効にすること。
        -   push の直前の判定と merge の後の確認がないこと。
        -   今の手順が `MERGE` を `MISSING <merge-base>` と読み、Fable が PR の全体を見ること。
    -   書き直した文も、#988 へのリンクを保つ（ステップ 2-8 がそのリンクで文を探す）。
    -   編集上の訂正の 2 つ目の種類（`docs/dev/conventions/adr.md`「Editorial corrections」）として、commit のメッセージに訂正の行を書く。
-   [x] **ステップ 1-13**: AC-23 の差分の記録を書く（§7 の「差分の記録」の PR-1 の欄）。

**完了の条件:**

-   設計書 3.1・3.2 の項目と、ステップ 1-3 の `fable-review.md` の変更が、スクリプトと文書にある。
-   `bash -n` が判定のスクリプトに通る。
-   macOS の手元で `go test -tags test -v -run 'TestReviewRecords' ./cmd/yt2column` を走らせ、`/bin/bash` 3.2 の subtest が走って通ったことを、PR の説明に書く。
-   `TestReviewRecordsCoverage` の既存の件が、期待する出力を変えずに通る（`coverage` を変えないこと、要件書 2.3）。
-   §7 に PR-1 の差分の記録がある。
-   `make test` と `make lint` が通る。

### PR-1 作成ポイント: validity decision for merges after the Fable record

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 / 1-12 / 1-13

**推奨タイトル**: `feat(0114): 判定のスクリプトに merge の後の確認の記録と unreviewed のモードを足す`

**レビュー観点**: 有効と判定する側を広げる 3 つの変更（規則 4・名指し・範囲の確かめ）が、数えれば出力が変わる配置の受け付けない件を持つか / 確認できない merge commit が `MISSING <merge-base>` に倒れ、確認できる merge commit の件が確認できる merge の fixture で作られているか / `coverage` の判定が変わらないか / `fable-review.md` の規則の箇条とスクリプトのヘッダーが同じ規則を述べるか

**実装モデル要件**: opus

**判定理由**: ステップ 1-1 は、Fable の記録を有効とみなして PR を開いてよいかを決める判定の関門を、有効の側へ広げる変更である（`mkplan.md` step 8 のパネルの条件の security-gate）。Sonnet が名指しの条件や範囲の確かめを取り違えると、確認していない merge commit や見ていない commit を `VALID` と判定し、Fable を飛ばして PR を開かせうる。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1066
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 手順の変更（設計書 8 章の 2）

**変更するファイル:** `.claude/commands/_lib/fable-review.md`、`.claude/commands/_lib/task-pr-handoff.md`、`.claude/commands/runplan.md`、`.claude/commands/_lib/local-cli-review.md`、`cmd/yt2column/review_records_test.go`、`docs/dev/adr/0050-fable-partial-review.md`、本計画

-   [x] **ステップ 2-1**: `_lib/fable-review.md` の入口を変える（設計書 3.3.1〜3.3.4）。
    -   冒頭の入口の数と入口の表を 4 つにする（「three entry points」を直す）。「Two task PRs pass FABLE」の段落と、FABLE の段落（`:45-47`）を合わせる。
    -   起動しないのは有効な記録を見つけたときだけ、と述べる 2 か所（`:62-64`、`:167-168`）に、`MERGE` も足す。
    -   「The validity decision」の読み方に `MERGE` と、その形の確かめを足す（3.1.1）。判定を走らせる入口の文（`:81`）と `<ENTRY>` の説明（`:96-98`）を、決定 5 に合わせる。
    -   Entry 1 の `MERGE`（3.3.2）、Entry 3 の変更（3.3.3。印の後に作る commit の一覧を含む）、Entry 4（3.3.4）を書く。
    -   単独の経路の呼ぶ側の文（`:210`）を、Entry 3・Entry 4 に加えて、範囲を求められないときの「The merge check」も呼ぶ形に直す。RANGE の出どころの文（`:213-214`）を、呼ぶ側が渡す RANGE に直す（有効かの判定の `<START>..HEAD`、または merge-base..HEAD）。
    -   「Reading the record lines」の読む範囲（`:529-530`）を、Part B step 3 では `origin/main..<D>` に合わせる。
-   [x] **ステップ 2-2**: `_lib/fable-review.md` に節「The merge check」を足す（設計書 3.3.5、決定 3・7・8・9・12・13）。
    -   範囲の一覧、範囲を求められないときの扱い、指示、出力の確かめ、記録の commit を書く。出力の確かめでは、指摘の場所の規則を決定 13 のものにする。記録の行の `<location>` も、その規則で確かめる（3.3.6）。
    -   待つ時間と呼び直しは、Entry 1・Entry 2 の該当する step を、引数を替えて参照する。状態の確かめは「The state check」を参照する。どちらも文を写さない。
    -   「Authorization and constraints」（`:66-77`）を、確認のサブエージェントにも当てる形に直す（AC-29）。
    -   記録の commit の前に、`_lib/local-cli-review.md` step 7 の「Mention check」を走らせることを書く。
    -   「Reading the record lines」に、merge の後の確認の行の読み方を足す（3.3.6）。
-   [x] **ステップ 2-3**: `_lib/local-cli-review.md` を変える（設計書 2.1）。
    -   呼び出し元の一覧（`:10-23`）に「The merge check」を足す（見出し文はタスク 0118 で数を含まない形に更新済み）。
    -   RECORD の表（`:70-79`）に、「The merge check」（RECORD `return`）を足す。
-   [x] **ステップ 2-4**: `_lib/task-pr-handoff.md` を変える（設計書 3.4、§1.3.1）。
    -   「Inputs」に FABLE・CHECKPOINT・AFTER_REMEDY を足す（3.4.1）。step 1 は、FABLE を受け取って渡す形にする。step 1 の「stop before step 2」（`:196`・`:205`）を、step 1.5 より前で止まることが分かる形に直す。
    -   step 1.5（3.4.2）と step 2.6（3.4.3、既存のタスク 0118 の step 2.5 の直後）を足す。
    -   step 3 を変える（3.4.4）。本文を読む範囲を `origin/main..<D>` にする。「どちらの CLI もレビューしなかった commit」の文を、`unreviewed` のモードの 3 行のコードブロックに置き換える。merge の後の確認の節と、push の直前の判定の行を足す。push は D を名指しし、上流を設定し直す。HEAD が D と違うときの扱いと、D が決まっていない呼び出しの扱いを書く。step 3 と step 4 の書き出しの文は変えない。
    -   AFTER_CREATE の前に、HEAD が D であることを確かめる（決定 10）。
    -   Authorization の段落に、D を名指しした push と `git branch --set-upstream-to` を足す。
-   [x] **ステップ 2-5**: `runplan.md` を変える（設計書 3.5）。
    -   step 5a（`:98`）と step 8（`:263`）のチェックポイントの段落に、Entry 3 の `MERGE` と FABLE を足す（3.5.1）。
    -   step 5a の Part B の呼び出しに、FABLE・CHECKPOINT `done`・AFTER_REMEDY を足す（3.5.2）。
    -   step 8 の、計画書に PR マーカーがないときの Part B の呼び出し（`:263`）が受け継ぐ入力にも、FABLE・CHECKPOINT・AFTER_REMEDY を入れる。
    -   B1 の文（`:279-280`）と、step 5a の LOCAL_REVIEW の文（`:120-123`）を直す（3.5.3）。
-   [x] **ステップ 2-6**: guard のテストを直す（設計書 3.7、7.2、§1.3.1）。
    -   `TestReviewRecordsCallSite` について、許す呼び出しの行に `bash <records-script> unreviewed <HEAD> <PLAN>` を足し、既存の `handoff step 2.5` の終端を `"\n2.6. "` に合わせたうえで、走らせる節に `task-pr-handoff.md` Part B step 3 の 3 行の並びを足す。Part B step 3 は番号付きの項目なので、節は `postingSections` と同じ `itemSection` で探す。
    -   `TestRecordCommitsCheckMention` の確かめる節に、「The merge check」を足す。
    -   どちらも、足した確かめを壊すと失敗することを確かめ、commit のメッセージに書く。壊し方は 2 つある。Part B step 3 から実行の行を消すこと、「The merge check」からメンションの確かめを消すことである。
-   [x] **ステップ 2-7**: 手順の変更の後、次の箇所を読み、新しい手順と食い違う文がないことを確かめる。食い違えば直し、PR の説明に書く。
    -   `CLAUDE.md:224-235`、`_lib/simple-process.md:232`。
    -   `runpr.md` のうち、`task-pr-handoff.md` Part B step 3 を指す箇所（`:164`・`:177`・`:1555`）。
    -   `local-cli-review.md` の該当箇所（何も起動しなかったときの SUMMARY に `MERGE` を含める、「Triage rules」に merge の後の確認を足す、ローカルの CLI レビューのものでない記録の行の一覧に `Merge check:` 系を足す）。
-   [x] **ステップ 2-8**: ADR-0050 の食い違いの文を消す。
    -   PR を作る前に、`git grep -nE 'issues/988([^0-9]|$)' origin/main -- docs/dev/adr/` で、`origin/main` の文を探す。
    -   決定の節にあり、この食い違いを述べる文だけを消す。commit のメッセージに訂正の行を書く（`docs/dev/conventions/adr.md`「Discrepancy statements」）。
-   [x] **ステップ 2-9**: AC-23 の差分の記録を書く（§7 の「差分の記録」の PR-2 の欄）。

**完了の条件:**

-   設計書 3.3〜3.5 の項目が、文書にある。ADR-0050 に、#988 にリンクする食い違いの文がない。
-   §7 に PR-2 の差分の記録がある。
-   `make test` と `make lint` が通る。

### PR-2 作成ポイント: pre-push re-check and merge check procedures

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9

**推奨タイトル**: `feat(0114): push の直前に Fable の記録を確かめ直し、記録の後の merge を merge の後の確認で扱う`

**レビュー観点**: push するのが判定した commit D だけで、確かめの後に足された commit が送られないか / 救済の後の再チェックの順序（AFTER_REMEDY・README の同期・CHECKS）と、最後の判定で止まる条件。計画書に PR マーカーがない `/runplan` の経路も含む / merge の後の確認の範囲の一覧と、指摘の場所の規則（決定 13）、範囲を求める git が失敗したときに Fable へ回すこと / 「The merge check」がサブエージェントの出力の文を記録や PR の説明に写さないこと

**実装モデル要件**: opus

**判定理由**: ステップ 2-1・2-4 は、PR を開いてよいかを決める判定の関門と push の形を変える（`mkplan.md` step 8 のパネルの条件の security-gate）。Sonnet が D の名指しや HEAD の確かめ、救済の後の再チェックを取り違えると、判定されていない commit を push したり、Fable も確認もないまま PR を開いたりしうる。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1097
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 実地の確かめ（設計書 8 章の 3）

**変更するファイル:** 本計画、`docs/dev/adr/0013-review-setup.md`、`docs/dev/adr/0042-implementation-model-tiers.md`、要件書、設計書（後の 3 つは §1.3.2 の決定の変更による）

-   [x] **ステップ 3-1**: 実地の確かめの結果を、§7 の「実地の確かめの記録」に書く（設計書 7.3、§1.3 の決定 3）。
    -   確かめは、PR-2 のマージの後に、人がほかのタスクのタスク文書の PR で行う。
    -   確かめる人は、ほかのタスクのタスク文書のコマンドのセッションに指示して実地の確かめを行わせ、結果の値を、本タスクの `/runplan` のセッションに渡す。セッションは、渡されなかった値を人に尋ねる。
        -   その PR の URL。
        -   その実行の判定の出力の 1 行目：step 1.5 の 2 回と step 2.6 の最初の段の 1 回。
        -   step 3 で走った push の命令と、上流を設定したか（実地の確かめでは走らなかった。§1.3.2 の 2）。
        -   merge の後の確認を起動した Agent ツールの呼び出しの `model` の値。
    -   セッションが自分で読む値もある。記録の commit、merge commit、merge の後の確認の記録の commit の ID と、`Merge check:` の行（`run` なら、確かめたモデル ID を持つ）である。サブエージェントの出力の `Model:` の行は commit に残らないので、読まない。セッションはこれらを、その PR のブランチの commit から読む。ブランチを取ってくる `git fetch` はネットワークの操作なので、人に許可を求める。
    -   書くのは次の 4 つである。
        -   上の値。
        -   `Merge check:` の行が `run` か `not run` か。`run` なら、その行のモデル ID が `claude-opus-` で始まること（AC-25）。`not run` なら AC-25 は確かめられていないので、そう書き、次の機会に確かめ直すかを人と決める。
        -   新しい `Fable review:` の記録が足されていないこと（AC-24）。
        -   push したのが step 2.6 の判定した commit であること（AC-05）。push が走らなかったときは、そう書く（§1.3.2 の 2 で、AC-05 は手順の読みだけで確かめると決めた）。
    -   設計書と違う振る舞いがあれば、その内容を書き、決定の変更として人と決める。実地の確かめで見つかった ADR-0013 の食い違いは、§1.3.2 の 1 のとおり、ADR-0013 と ADR-0042 に例外を書き足して扱う。
-   [x] **ステップ 3-2**: AC-23 の差分の記録を書く（§7 の「差分の記録」の PR-3 の欄）。

**完了の条件:** §7 に実地の確かめの記録と PR-3 の差分の記録がある。`make test` と `make lint` が通る。

### PR-3 作成ポイント: live check of the merge check

**対象ステップ**: 3-1 / 3-2

**推奨タイトル**: `docs(0114): merge の後の確認を実地で確かめた結果を記録する`

**レビュー観点**: 実地の確かめで、step 1.5 が merge の後の確認を 1 回だけ走らせ、Fable が走らなかったか / `Merge check:` の行のモデル ID が `claude-opus-` で始まるか / 記録が、確認のサブエージェントの出力の文を写していないか / push を観測できなかったことと、ADR-0013 の食い違いの扱いが記録にあるか

**実装モデル要件**: sonnet

**判定理由**: どの条件にも当たらない。実地の確かめは PR-2 のマージの後に人がほかのタスクで行い、この PR はその結果を計画書に書く。§1.3.2 で、ADR-0013 と ADR-0042 に例外も書き足すが、PR-2 で人と決めて実装した手順の振る舞いを ADR の文に写すだけで、手順もコードも変えない。

**自動送信**: hold

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1118
- [x] PR がマージされた
- [-] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 前提 |
|---|---|---|
| M1（PR-1） | 判定のスクリプトの `fable` のモードの変更と `unreviewed` のモード、そのテスト、`fable-review.md` の規則の箇条と記録の行の表 | 本計画の承認 |
| M2（PR-2） | `fable-review.md` の入口と「The merge check」、`task-pr-handoff.md` の step 1.5・2.6・3、`runplan.md`、`local-cli-review.md`、guard のテスト、ADR-0050 の食い違いの文の削除 | PR-1 のマージ（手順が読む `MERGE` と `unreviewed` が、`origin/main` のスクリプトにある） |
| M3（PR-3） | 実地の確かめの記録 | PR-2 のマージと、人が行うほかのタスクのタスク文書の PR での確かめ |

PR-1 のマージから PR-2 のマージまでの間は、今の手順が `MERGE` を形の違う 1 行目と読み、Fable が PR の全体を見る（設計書 8 章）。#987 で起きた形と同じ、安全な側の動きである。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 / 1-12 / 1-13 | 判定のスクリプトの `fable` のモードの変更と `unreviewed` のモード、スクリプトのテスト、`fable-review.md` の規則の箇条と記録の行の表、ADR-0050 の食い違いの文の書き直し | opus | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9 | `fable-review.md` の入口と「The merge check」、`task-pr-handoff.md`・`runplan.md`・`local-cli-review.md` の手順、guard のテスト、ADR-0050 の食い違いの文の削除 | opus | auto |
| PR-3 | 3-1 / 3-2 | 実地の確かめの記録 | sonnet | hold |

## 4. テスト戦略 (Test Strategy)

-   **スクリプトのテスト:** 判定のスクリプトの規則は、`TestReviewRecordsCrossCheck` と `TestReviewRecordsUnreviewed` が、一時的なリポジトリの上で、設計書 7.1 の件ごとに確かめる。どちらも、PATH の bash と `/bin/bash` 3.2 で走らせる。git の設定を変えても出力が変わらないことは、merge と確認の記録を持つ分岐でも確かめる（ステップ 1-9）。fixture の記録の行は、手順の文書の表から読んだ形で作る（`readFableForms`）。
-   **境界と失敗:** 確認の記録の名指しの条件、規則 4 の条件、広げた範囲の確かめの条件、確認できない merge commit の 3 つの形には、数えれば出力が変わる配置で、受け付けない側の件を置く（ステップ 1-5）。ほかの確かめが裏打ちする条件は、その旨を書く（§1.2）。名指しした ID が何も指さないときや commit でないときは、失敗ではなく、数えない（ステップ 1-1）。`unreviewed` の失敗は、`coverage` の失敗の件のうち LAST_REVIEWED に固有でないものと、`unreviewed` に固有の引数の件で確かめる。
-   **guard のテスト:** スクリプトの呼び方（`TestReviewRecordsCallSite`）、記録の行の接頭辞（`TestRecordLinePrefixesDistinct`）、記録の commit の前のメンションの確かめ（`TestRecordCommitsCheckMention`）、モードの名前（`TestReviewRecordsMode`）。状態の確かめのコードブロックは、今の `TestFableStateCheckBlock` が確かめる。
-   **後方互換:** `coverage` のモードは変えない。`TestReviewRecordsCoverage` の既存の件が期待する出力を変えずに通ることと、ステップ 1-8 の件で確かめる（要件書 2.3、4.4）。既存の記録の行の形は変えない（要件書 4.4）。
-   **読んで確かめる:** 入口の順序、救済、push の形、確認のモデルと範囲の求め方などの手順の文は、PR のレビューで読む（設計書 7.2）。
-   **差分の記録:** AC-23 は、各 PR で `docs/tasks/0098_fable_partial_review/` の差分が空であることを記録する（§7）。
-   **実地の確かめ:** Opus のサブエージェントの起動とモデル ID の確かめは、手順の文だけでは確かめられないので、PR-2 のマージの後に実地で確かめる（設計書 7.3）。実地で通るのは、Entry 3（タスク文書の PR の Part B を手で続けたので、step 1.5 からの呼び出しは通らない）と、記録の後の commit がないときの step 2.6 である。Part B step 3 の push は、実地の確かめでは走らせられなかった（§7 の実地の確かめの記録）。step 1.5 の呼び出しと push（AC-05）は、手順の読みだけで確かめる（§1.3.2 の 2）。Entry 4 の救済と、HEAD が D と違うときの扱いは、読んで確かめるだけになる（設計書 7.3 が受け入れた範囲）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 判定のスクリプトの誤りで、確認していない merge commit や見ていない commit を `VALID` とする | 有効の側へ広げる変更ごとに、数えれば出力が変わる配置で受け付けない件を置き、規則を壊して失敗することを確かめる（ステップ 1-5・1-11）。失敗は `MISSING <merge-base>` に倒す（設計書 1.1）。PR-1 と PR-2 は `opus` で実装する |
| テストの fixture が確認できない merge commit を作り、確認できる merge commit の件が別の理由で通る | 確認できる merge の fixture が、merge commit の形を確かめてから返す（ステップ 1-4） |
| PR-1 と PR-2 の間に、記録の後の merge で Fable が PR の全体を見直す | 意図した途中の状態で、今と同じ動きである（§3.1） |
| PR-2 のチェックポイントと Part B は、PR-2 自身が書き換えた途中の手順の文を読んで走る | 手順の文に従って止まったときは、報告して人に問う。止まった後の再開の手段は範囲外である（要件書 2.3） |
| 実地の確かめに要る、`origin/main` が進んだタスク文書の PR が、すぐには来ない | PR-3 を `hold` にし、人が確かめてから `/runplan` を送る（§1.3 の決定 3） |
| `git show --remerge-diff` の振る舞いが、利用者の git で設計書の実験と違う | 範囲を求める git の失敗は、確認せずに Fable へ回す（決定 12）。実地の確かめで違いを見つけたら、決定の変更として人と決める（ステップ 3-1） |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 / 1-12 / 1-13）
- [x] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 / 2-9）
- [x] PR-3 マージ済み（対象ステップ: 3-1 / 3-2）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

確かめの種類は次のとおりである。

-   `test`: Go のテストが、判定のスクリプトの振る舞いを確かめる。テストの場所は `cmd/yt2column/review_records_test.go`（以下、`rr_test`）である。
-   `static`: guard のテストが、コマンドの文書の値と構造を確かめる。
-   `diff`: 差分の記録を PR のレビューで読む（`docs/dev/conventions/requirements_process.md` §4「`diff` verification」）。
-   `manual`: PR のレビューで、その文書の該当箇所を読んで確かめる。コマンドの定義の手順の文だけを定める AC と、設計書 7.2 が文書のテストで確かめないとした手順の文の中の値（push の形と、§1.2 がその「など」に含めて読む確認のモデルとその確かめ、範囲を求める命令）は、`manual` で確かめる（§1.2）。実地の確かめの結果は、PR-3 のレビューで読む。

| AC | 確かめ | 種類 | 実装 | PR |
|---|---|---|---|---|
| AC-01 | 取り出し方と走らせ方を共有することは、既存の `rr_test::TestReviewRecordsCallSite`（「The validity decision」の 3 行の並び）が確かめる。push の直前に判定を走らせることは、`task-pr-handoff.md` step 2.6 と `fable-review.md` Entry 4 を読む | static、manual | ステップ 2-1・2-4 | PR-2 |
| AC-02 | `rr_test::TestReviewRecordsCrossCheck`（印の後に CHECKS の直しだけがあるときの `VALID`）。Entry 4 の最初の段が commit を作らないことを読む | test、manual | ステップ 1-1・1-5・2-1 | PR-1、PR-2 |
| AC-03 | `rr_test::TestReviewRecordsCrossCheck`（`MISSING <START>` の件と失敗の件）。Entry 4 の救済と、FABLE の渡し方を読む | test、manual | ステップ 1-1・1-5・2-1・2-4・2-5 | PR-1、PR-2 |
| AC-04 | `task-pr-handoff.md` step 2.6 の再チェックの順序を読む | manual | ステップ 2-4 | PR-2 |
| AC-05 | `task-pr-handoff.md` step 2.6・step 3（D の名指しの push、HEAD の確かめ、判定のやり直し）と AFTER_CREATE の前の確かめを読む。実地の確かめでは Part B step 3 の push を走らせなかったため、push したのが判定した commit であることは確かめられていない（§7 の実地の確かめの記録） | manual | ステップ 2-4・3-1 | PR-2、PR-3 |
| AC-06 | Entry 4 の最後の段と `task-pr-handoff.md` step 2.6 の止まり方と報告の内容を読む | manual | ステップ 2-1・2-4 | PR-2 |
| AC-07 | Entry 4 と単独の経路の `not run` の扱いを読む | manual | ステップ 2-1 | PR-2 |
| AC-08 | Entry 4 の、救済の途中で止まったときの扱いを読む | manual | ステップ 2-1 | PR-2 |
| AC-09 | `runplan.md` の AFTER_REMEDY を、PR マーカーのある経路とない経路の両方で読む | manual | ステップ 2-5 | PR-2 |
| AC-10 | `rr_test::TestReviewRecordsCrossCheck`（「README.en.md after the record」） | test | ステップ 1-1・1-5 | PR-1 |
| AC-11 | `rr_test::TestReviewRecordsCrossCheck`（記録 → 印 → CHECKS の直し、`checkpoint`）。`task-pr-handoff.md` step 1.5 を読む | test、manual | ステップ 1-5・2-4 | PR-1、PR-2 |
| AC-12 | `rr_test::TestReviewRecordsCrossCheck`（記録 → 中身のある commit → 印 → CHECKS の直し） | test | ステップ 1-5 | PR-1 |
| AC-13 | `rr_test::TestReviewRecordsCrossCheck`（記録 → 確認できる merge commit、記録 → 印 → 確認できる merge commit、確認できない merge commit の 3 つの形）。Entry 3 の `MERGE` を読む | test、manual | ステップ 1-1・1-5・2-1 | PR-1、PR-2 |
| AC-14 | `rr_test::TestReviewRecordsCrossCheck`（記録の START が古い merge-base の件と、ブランチの commit の件。どちらも中身のある commit X の後に確認できる merge commit） | test | ステップ 1-1・1-5 | PR-1 |
| AC-15 | `rr_test::TestReviewRecordsCrossCheck`（起動の後、記録の commit の前の merge commit）。Entry 4 の最後の段で止まることを読む | test、manual | ステップ 1-5・2-1 | PR-1、PR-2 |
| AC-16 | `rr_test::TestReviewRecordsUnreviewed`（`README.en.md` だけの commit・CHECKS の直しを挙げる件と、上書きした「merge commit inside a seen range」）、`rr_test::TestReviewRecordsCallSite`（Part B step 3 の 3 行の並び） | test、static | ステップ 1-2・1-7・2-4・2-6 | PR-1、PR-2 |
| AC-17 | `rr_test::TestReviewRecordsUnreviewed`（#987 の形、START が古い merge-base で END が merge commit の後の見た範囲の行） | test | ステップ 1-2・1-7 | PR-1 |
| AC-18 | `rr_test::TestReviewRecordsUnreviewed`（チェックポイントの印 → 確認できる merge commit） | test | ステップ 1-2・1-7 | PR-1 |
| AC-19 | `rr_test::TestReviewRecordsUnreviewed`（単独の経路の直しで、見た範囲の行が直しを含む件） | test | ステップ 1-2・1-7 | PR-1 |
| AC-20 | `task-pr-handoff.md` step 3 の Fable の部分レビューの節を読む | manual | ステップ 2-4 | PR-2 |
| AC-21 | ADR-0050 の「決定」を読み、食い違いの文が残っていないことを確かめる。形は `cmd/yt2column/adr_test.go::TestADRRecords` が確かめる | manual | ステップ 1-12・2-8 | PR-1、PR-2 |
| AC-22 | ADR-0050 の「採らなかった案」と「経緯」は、設計の PR（#1049）が更新し、そのレビューで読んで確かめた。PR-2 のレビューでは、食い違いの文を消した後も変わっていないことを読む | manual | なし（#1049 で済み） | PR-2 |
| AC-23 | 差分の記録（AC-23）。比べるパス: `docs/tasks/0098_fable_partial_review/`。各 PR で記録する | diff | ステップ 1-13・2-9・3-2 | PR-1、PR-2、PR-3 |
| AC-24 | `rr_test::TestReviewRecordsCrossCheck`（`MERGE <n>` の件）。Entry 3・Entry 4 の救済と、「The merge check」の範囲を求められないときの扱いを読む。実地の確かめ（ステップ 3-1） | test、manual | ステップ 1-1・1-5・2-1・2-2・3-1 | PR-1、PR-2、PR-3 |
| AC-25 | 「The merge check」のモデルとモデルの確かめを読む。実地の確かめ（ステップ 3-1） | manual | ステップ 2-2・3-1 | PR-2、PR-3 |
| AC-26 | 「The merge check」の範囲の一覧を読む | manual | ステップ 2-2 | PR-2 |
| AC-27 | `rr_test::TestReviewRecordsCrossCheck`（名指しの条件、何も指さない ID と tree の ID、形の違う行、12 桁の名指し。行は `readFableForms` が表から読んだ形で作る）。「The merge check」が merge commit を名指しした行を記録の commit に残すことを読む | test、manual | ステップ 1-1・1-3・1-4・1-5・2-2 | PR-1、PR-2 |
| AC-28 | `rr_test::TestReviewRecordsCrossCheck`（`not run` の確認の記録で `VALID`）。「The merge check」の `not run` の扱いを読む | test、manual | ステップ 1-5・2-2 | PR-1、PR-2 |
| AC-29 | 「The merge check」の制約と状態の確かめを読む（状態の確かめのコードブロックは `cmd/yt2column/fable_state_docs_test.go::TestFableStateCheckBlock` が確かめる） | manual | ステップ 2-2 | PR-2 |
| AC-30 | `rr_test::TestReviewRecordsCrossCheck`（規則 4 の件。M より前の commit、`not run` の範囲だけの直し）。「The merge check」のトリアージと直しの確かめを読む | test、manual | ステップ 1-1・1-5・2-2 | PR-1、PR-2 |
| AC-31 | `task-pr-handoff.md` step 3 の merge の後の確認の節を読む。出力の文を写さないための記録の commit の前のメンションの確かめは、`rr_test::TestRecordCommitsCheckMention`（「The merge check」）が確かめる（要件書 4.2） | static、manual | ステップ 2-2・2-4・2-6 | PR-2 |
| AC-32 | `rr_test::TestReviewRecordsCrossCheck`（START が古い merge-base の記録の後の確認できる merge commit、範囲が `main` 側の commit を含む記録を読まない件） | test | ステップ 1-1・1-5・1-6 | PR-1 |

### 差分の記録

AC-23 は、比べるパスの差分が空であることを確かめる。差分が小さい文書の変更なので、スクリプトではなく、記録した差分を読む形にする（`requirements_process.md` §4「Recorded diff reading」）。比べるパスに本計画は入らない。

Diff record (AC-23):

PR-1:

- Compared paths: `docs/tasks/0098_fable_partial_review/`
- Merge-base: `ca11254c451d01ddb3abf8995024f239304d5d8a`
- Compared commit: `bf5308f227f1d980c09e762038b5d616188424e6`

Path diff:

```text
```

PR-2:

- Compared paths: `docs/tasks/0098_fable_partial_review/`
- Merge-base: `d74acaeb62c0aeebc1a85a2593324aac97aa3029`
- Compared commit: `5c71c0881c91953b42099dd7594b304329e6dda5`

Path diff:

```text
```

PR-3:

- Compared paths: `docs/tasks/0098_fable_partial_review/`
- Merge-base: `d29bb3d730960dcf299f5801144c0cb92b888978`
- Compared commit: `03bd990b6f984aba19c7ccf44a538836cc69188f`

Path diff:

```text
```

### 実地の確かめの記録

2026-11-24 に、タスク 0122 のタスク文書の PR（[#1102](https://github.com/isseis/yt2column/pull/1102)）で行った。Part B step 1 の記録の commit の後に `origin/main` を merge し、step 1.5（Entry 3）を走らせた。

-   **判定の出力の 1 行目:** step 1.5 の 1 回目は `MERGE 1`（2 行目は merge commit `c7239121`）、2 回目は `VALID`、step 2.6 の最初の段は `VALID`。判定した commit（D）は `fda2fcc5`。
-   **merge の後の確認:** 1 回走った（`Merge check: run c7239121355b6c694086d4eca167b7811e2ee4ac (model claude-opus-5-5; fixed 0, rejected 0, deferred 1, unclear 0, invalid 0; 59s, tokens 104784)`）。モデル ID が `claude-opus-` で始まるので、AC-25 を満たす。起動した Agent ツールの `model` は `opus`。範囲の箇所は `resolution` 0 件、`main-reference` 8 件で、指摘は 1 件（`deferred`）。
-   **新しい Fable の記録:** step 1.5 の間には足されていない。step 1.5 の間に `Fable review: run` と `Fable review: not run` の行は増えず、印の `Fable review: checkpoint` だけができた。Fable は走らなかった（AC-24 を満たす）。なお、マージの前に、記録を作り直す単独の経路が `9171922a` を作っている（下の「実地の確かめの進め方の、計画との差分」）。
-   **push:** 観測できなかった。人が実地の確かめを step 2.6 の最初の段の後で終え、Part B step 3 の push（D を名指しする push と上流の設定）を走らせていない。その後の `/runpr 1102` が、D と承認の commit `131723e5` を push してマージした（squash `6e13dd74`）。したがって、push したのが D であること（AC-05）は、この実地の確かめでは確かめられていない。AC-05 は §7 の表のとおり `manual`（手順の読み）だけで確かめると、人と決めた（§1.3.2 の 2）。
-   **commit:** Fable の記録 `9171922a`、merge commit `c7239121`、merge の後の確認の記録 `c6d2f6dc`、チェックポイントの印 `6888a50e`、D `fda2fcc5`。
-   **判定に使ったスクリプト:** step 1.5 の 1 回目は `origin/main` の `3597d4fb` のもの（開始時の `origin/main`。マージは 10:10、`2cfeec03` が `origin/main` に届いたのは 10:11。1 回目がその後に走ったなら 1 回目も `2cfeec03` である）。実行の途中で `origin/main` が `2cfeec03`（タスク 0120 の `review-records.sh` の一般化）に進んだので、step 1.5 の 2 回目と step 2.5・2.6 の判定は、その時点の `origin/main` の `2cfeec03` のもの（正本は判定の前に `origin/main` を取り直す）。merge の後の確認はスクリプトを使わない。したがって、2 回目の `VALID` と step 2.6 の `VALID` は、0114 のスクリプトではなく後継のスクリプトで得た結果である。
-   **設計書と違う振る舞い:** Entry 3 の手順についてはない。step 1.5 が `MERGE` で merge の後の確認を 1 回走らせ、Fable が走らないことは、設計書 7.3 のとおりである。ただし、次の 2 つは確かめていない。(1) `_lib/task-pr-handoff.md` Part B step 1.5 が `CHECKPOINT` = `run` で Entry 3 を呼ぶこと（実地の確かめは、既に開いた PR のブランチで Part B を手で続行した）。(2) push（AC-05）。どちらも、手順の読みだけで確かめると人と決めた（§1.3.2 の 2）。
-   **実地の確かめの進め方の、計画との差分:**
    -   タスク文書のコマンドの実行中ではなく、既に開いた PR のブランチで、Part B を step 1.5 から手で続行した。
    -   記録の後に中断した修正の commit が残り、Fable の記録が無効だったため、マージの前に単独の経路で Fable の記録を作り直した（Entry 3 の救済と同じ手順を、チェックポイントの印を残さずに走らせた）。
    -   本文の更新と push は Part B step 3 ではなく `/runpr` が行った（上の「push」）。
    -   step 2.5 では、`origin/main` のスクリプトの `coverage none` が `PARTIAL 5` を返し、ローカルの CLI レビューが利用上限（`rate_limit`）で失敗したので、正本のとおり再試行せず、`Local CLI review: not run 3597d4fb5ba6..6888a50ed80e (coderabbit: rate_limit)` と記録した。
-   **ADR-0013 の食い違い:** merge の後の確認が、ADR-0013 の「PR を開く前に、どのローカルの CLI レビューも見ていない中身のある commit を、まとめてもう 1 回ローカルの CLI レビューにかける」（`docs/dev/adr/0013-review-setup.md` の 42〜43 行）と、step 2.6 が救済の後の CHECKS の直しをかけ直さないことの食い違いを、`deferred` の指摘として挙げた。0114 の設計 1.2.1 が人と決めた振る舞いで実装は設計どおりだが、ADR の文と実装が合っておらず、設計 1.2.1 の item 2 の「ADR の決定を変えない」という判断も正しくない。本タスクは ADR-0013 の決定を変えるので、`docs/dev/conventions/adr.md`「Approval and the PR」に従い、本 PR で ADR-0013 の決定に例外を書き足した。同じ例外で狭まる ADR-0042 にも書き足した（設計書 1.2.2、計画書 1.3.2）。PR #1102 の説明には `deferred` として残した。

## 8. 成功の基準 (Success Criteria)

-   **機能:** 要件書の AC-01〜AC-32 が、§7 の確かめで満たされる。
-   **品質:** 有効の側へ広げる変更のそれぞれに、受け付けない側の件があり、規則を壊すと失敗することを確かめてある。ほかの確かめが裏打ちする条件は、その旨を書いてある（ステップ 1-11）。`make test` と `make lint` が通る。
-   **セキュリティ:** 判定のスクリプトを `origin/main` のコピーでだけ走らせることが、`TestReviewRecordsCallSite` で確かめられる。確認のサブエージェントの出力の文を、記録にも PR の説明にも写さないことを、PR-2 のレビューで読んで確かめる（設計書 5.2）。
-   **文書:** ADR-0050 に、#988 にリンクする食い違いの文が残らない。

## 9. 次のステップ (Next Steps)

-   PR-3 のマージの後、issue #988 を閉じる（`/runpr` が、タスクの最後の計画の PR のマージの後に扱う）。
-   止まった後の再開の手段と、PR を開いた後の merge の扱いは、範囲外として残る（設計書 9 章）。必要になったら、別の issue で扱う。

## 変更履歴

- 2026-11-20 承認（isseis、#1060）
- 2026-11-23 セッション中の再承認：タスク 0118 の Part B step 2.5 を残して本タスクの push 直前判定を step 2.6 とし、step 2.6 の再チェックでは step 2.5 を走らせ直さない（isseis、#1097）
- 2026-11-24 セッション中の再承認：ADR-0013 と ADR-0042 の決定に、Part B step 2.5 の後にできる commit をローカルの CLI レビューにかけ直さない例外を書き足し、2 つの ADR を本 PR で訂正する。AC-05 の確かめから実地の確かめを外し、手順の読みだけにする（isseis、#1118）
