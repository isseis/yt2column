# ADR-0036: タスク文書の Comments には今の状態の理由だけを書き、状態の遷移は変更履歴に、編集上の訂正は PR の説明とコミットに記録する

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

タスク文書は、`approved` になるまで次の段階に進めないが、承認した後にも編集が入る。そのため、承認した後の編集が承認を取り消すかどうかと、状態とその理由・経緯をどこに記録するかを決める必要がある。編集を分けないと、言い回しを直すたびに承認をやり直すことになる（[requirements_process.md](../conventions/requirements_process.md)「Editing an approved document」）。記録の置き場所が曖昧だと、今の状態の理由と過去の経緯が混ざり、レビュアーが今読むべきことが埋もれる。

実際に、状態表の `Comments` に訂正と過去の承認を積み重ねた結果、1 つのセルが数 KB に膨らんだ（[0053 要件定義書](../../tasks/0053_status_comments_policy/01_requirements.md) §1）。また、変更履歴のない古い文書を `draft` に戻すと、過去の承認が文書から消えた（[0056 要件定義書](../../tasks/0056_legacy_approval_history/01_requirements.md) §1）。

## 決定

-   **記録の置き場所は、読む人と読む時期で決める。** タスク文書についての記録の置き場所は、状態表の `Comments`、変更履歴、本文の「…で決めたこと（YYYY-MM-DD）」の節、PR の説明、コミットメッセージ、ADR の 6 つである。それぞれに何を置くかは、[requirements_process.md](../conventions/requirements_process.md) の「Document Status Format」の段落に従う。ADR は、タスク文書の形の状態表と変更履歴を持たない。ADR の状態は ADR の「状態」の節に、決定の変遷は「経緯」に書き、`/runpr` は ADR の承認を記録しない（[adr.md](../conventions/adr.md)「Status」「History entries」）。ADR の編集上の訂正をどこに記録するかは、[ADR-0039](0039-adr-writing-criteria.md) が定める。
-   **承認済みの文書への編集を、決定の変更・セッション中の再承認・編集上の訂正の 3 つに分ける。** 決定の変更は、文書を `draft` に戻して承認し直す。`/runplan` が実装中に見つけた決定の変更は、セッションでのレビュアーの答えを再承認とする。編集上の訂正は、状態も `Comments` も変更履歴も変えない。分け方と、それぞれで何を書き換えるかは、[requirements_process.md](../conventions/requirements_process.md)「Editing an approved document」に従う。
-   **`Comments` には、今の状態についてレビュアーが今知るべき理由だけを書く。** 状態が変わるたびに書き換え、前の値を残さない。状態ごとの値は、[requirements_process.md](../conventions/requirements_process.md)「Comments」に従う。
-   **状態の遷移は、文書の末尾の変更履歴に 1 件 1 行で追記する。** 変更履歴には編集上の訂正を書かない。各行は、原則としてその行を加えた PR の番号を持ち、PR を作る前に書いた行の番号は `/runpr` がマージの直前に埋める（[runpr.md](../../../.claude/commands/runpr.md)）。行の形と PR 番号の例外は、[requirements_process.md](../conventions/requirements_process.md)「Change history」に従う。
-   **変更履歴のない文書は、最初の状態の遷移を書くときに移す。** 旧 `Comments` を変更履歴の先頭の項目にし、承認済みの文書なら状態表から承認の行を写す。写せないときは、遷移を書かずに止まる。一括では移さない。対象と手順は、[requirements_process.md](../conventions/requirements_process.md)「Documents without a change history」に従う。
-   **編集上の訂正と決定の変更は、PR の説明にも書く。** 編集上の訂正はコミットのメッセージに訂正の行として書き、PR の説明の訂正の段落はその行から作る（[task-pr-handoff.md](../../../.claude/commands/_lib/task-pr-handoff.md) の Part B、[fixpr.md](../../../.claude/commands/fixpr.md)）。形は、[requirements_process.md](../conventions/requirements_process.md)「Recording in the PR description」に従う。`/fixpr` が PR の説明に追記する道具と、その文面を信頼しないデータとして扱うことは、[ADR-0015](0015-untrusted-review-threads.md) が定める。
-   **書式は規則の 1 か所で定め、コマンドの定義は規則の節を名指しして参照する。** `Comments` の値、変更履歴の行の形、訂正の行と訂正の段落の形は、[requirements_process.md](../conventions/requirements_process.md) だけに書く。[runpr.md](../../../.claude/commands/runpr.md)・[task-pr-handoff.md](../../../.claude/commands/_lib/task-pr-handoff.md)・[runplan.md](../../../.claude/commands/runplan.md)・[fixpr.md](../../../.claude/commands/fixpr.md) などのコマンドの定義には、いつどの文書に書くかだけを書く。

## 採らなかった案

-   **編集上の訂正を `Comments` に記録し、決定の変更でないことをそこに明記する（旧規則）。** 訂正の詳細が PR のたびに追記され、レビューが終わった後も残り続けた。決定の変更でないことへの反論の機会は、その PR のレビュアーが一度読めば確保でき、何を直したかは PR に残る（[0053 要件定義書](../../tasks/0053_status_comments_policy/01_requirements.md) §1 と「issue #596 で決めたこと」）。
-   **状態の遷移を `Comments` に 1 件 1 行で残す（issue #596 の表の案）。** 遷移のたびに `Comments` が伸び、今の状態の理由が過去の行に埋もれる（[0053 要件定義書](../../tasks/0053_status_comments_policy/01_requirements.md)「要件定義の際に決めたこと（2026-11-01）」）。
-   **編集上の訂正を変更履歴に記録する。** `Comments` と同じ肥大化が、変更履歴でも起きる。訂正の記録は PR の説明で足りる（[0053 要件定義書](../../tasks/0053_status_comments_policy/01_requirements.md)「要件定義の際に決めたこと（2026-11-01）」）。
-   **`/fixpr` が入れた編集上の訂正を、スレッドへの返信やコミットメッセージだけに残す。** 訂正の説明が PR の説明にあることを、PR を作った後の訂正でも満たすため、PR の説明に追記する（[0053 要件定義書](../../tasks/0053_status_comments_policy/01_requirements.md)「要件定義の際に決めたこと（2026-11-01）」）。
-   **変更履歴を表にする。** `Comments` と同じく、1 つのセルに長い要旨が入り、読みにくくなる（[0053 設計書](../../tasks/0053_status_comments_policy/02_architecture.md)「1.3. 設計の際に決めたこと（2026-11-01）」）。
-   **PR 番号を、PR を作った直後に書き足す。** PR を作った後の push が失敗すると印が残るので、結局 PRE_MERGE にも同じ置き換えが要り、置き換える場所が 2 か所になる（[0053 設計書](../../tasks/0053_status_comments_policy/02_architecture.md)「1.3. 設計の際に決めたこと（2026-11-01）」）。
-   **`/fixpr` のコミットには訂正の行を書かない例外を、規則に設ける。** 要件は、訂正の説明を PR の説明とコミットメッセージの両方に書くことを求める。そこで、例外を設けず、`fixprguard finish` に `--body-file` を加えて、`/fixpr` のコミットにも訂正の行を書けるようにした（[0053 設計書](../../tasks/0053_status_comments_policy/02_architecture.md)「1.3. 設計の際に決めたこと（2026-11-01）」）。
-   **`/fixpr` の訂正の段落を、既存の `fixprguard edit-pr` で追記する。** セッションが今のタイトルと本文を取得してファイルに書く必要があり、信頼しない本文がセッションに渡る。また、セッションは `edit-pr` が求める改行のない 1 行のタイトルのファイルを作れない（[0053 設計書](../../tasks/0053_status_comments_policy/02_architecture.md)「1.3. 設計の際に決めたこと（2026-11-01）」）。
-   **既存のタスク文書を一括で新しい規則に移す。** 理由は記録されていない（[0053 要件定義書](../../tasks/0053_status_comments_policy/01_requirements.md)「issue #596 で決めたこと」は、新しい規則をこれ以降の編集から適用するとだけ述べる。[0053 実装計画書](../../tasks/0053_status_comments_policy/03_implementation_plan.md) §9 と [0056 設計書](../../tasks/0056_legacy_approval_history/02_architecture.md) §9 は、必要になったら別のタスクにするとする）。
-   **0053 の要件定義書の AC-17・AC-18 を書き換えて、移し替えの対象を広げる。** それ自体が決定の変更になってその文書が `draft` に戻り、新たに決める内容のない再承認が必要になる（[0056 要件定義書](../../tasks/0056_legacy_approval_history/01_requirements.md)「要件定義の際に決めたこと（2026-11-02）」）。
-   **写した承認の行の PR 番号を、git の履歴から探して書く。** 移し替えのたびに履歴の検索と GitHub への問い合わせが要るうえ、状態表から機械的に写せるという issue の前提が崩れる（[0056 要件定義書](../../tasks/0056_legacy_approval_history/01_requirements.md) §2.3）。
-   **誤記と思われる値を直して写す、または値がないときに推測で埋めるか行を省く。** 写す側で値を推測して直さない（`CLAUDE.md` の「Reject, don't normalize」）。推測で写した承認の行を作ったり、黙って過去の承認を失ったりしない（[0056 要件定義書](../../tasks/0056_legacy_approval_history/01_requirements.md)「要件定義の際に決めたこと（2026-11-02）」）。
-   **写した承認の行の印を、issue #621 の例の `旧規則` にする。** 短いが、何が旧規則なのかが読む人に伝わりにくい（[0056 設計書](../../tasks/0056_legacy_approval_history/02_architecture.md)「1.3. 設計の際に決めたこと（2026-11-02）」）。
-   **写した値を、`git diff` で消えた行と比べる。** セッション中の再承認と取り下げでは `Review date` と `Reviewer` の行が変わらず、diff に現れない（[0056 設計書](../../tasks/0056_legacy_approval_history/02_architecture.md)「1.3. 設計の際に決めたこと（2026-11-02）」）。
-   **移し替えの確かめを、`fixprguard` のようなツールに移して強制する。** 今は、写す値が短く、止まる文書もないので、手順で足りる。移し替えの誤りが実際に起きるようなら移す（[0056 設計書](../../tasks/0056_legacy_approval_history/02_architecture.md) §9）。
-   **ADR にも、タスク文書の状態表と `/runpr` での承認の記録を持ち込む。** `/runpr` の承認の記録を ADR に広げる必要があり、変更が大きくなる（[0060 要件定義書](../../tasks/0060_adr_records/01_requirements.md)「要件定義の際に決めたこと（2026-11-04）」）。

## 結果

-   LLM が自分の編集を編集上の訂正と判断したことは、コミットメッセージと PR の説明に残り、その PR のレビュアーが反論できる。訂正の記録は文書には残らず、PR とコミットにだけ残る。
-   `Comments` は短く保たれる。過去の承認と `draft` への戻しは、変更履歴を読めば分かる。
-   変更履歴の PR 番号は、タスク文書を変える PR が `/runpr` を通ってマージされることを前提にする。`/runpr` を通らずにマージされると、`#未記入` が `main` に残りうる。`/runpr` はそれを自動では埋めず、別の PR で直す（[requirements_process.md](../conventions/requirements_process.md)「Change history」）。
-   移し替えの確かめは規則の手順であり、ツールは強制しない。人が手で取り下げるときに確かめを省くと、写しの誤りは PR のレビューでしか見つからない。確かめは写した承認の行を作るコミットでだけ働き、後のコミットが行を書き換えても機械的には検出しない（[0056 設計書](../../tasks/0056_legacy_approval_history/02_architecture.md) 4 章）。
-   既存の文書は一括で移さないので、変更履歴のない文書が残る。その文書は今のままで有効で、最初の状態の遷移を書くときに移す。
-   書式を変えるときは、規則を 1 か所だけ直し、規則の節を参照するコマンドの定義の文言を確かめる。

## 経緯

- 2026-11-01 `Comments` を今の状態の理由だけにし、状態の遷移を文書の末尾の変更履歴に 1 件 1 行で書くことにした。編集上の訂正は PR の説明とコミットメッセージに書き、`/runplan` のセッション中の決定の変更は「実装の際に決めたこと」の節に書く。旧規則の `Comments` は、最初の遷移のときに変更履歴の先頭へ移す（[0053 要件定義書](../../tasks/0053_status_comments_policy/01_requirements.md)「issue #596 で決めたこと」「要件定義の際に決めたこと（2026-11-01）」）
- 2026-11-01 変更履歴を箇条書きにし、PR を作る前に書く行の PR 番号を `#未記入` として `/runpr` の PRE_MERGE で埋めることにした。PR の説明に書く訂正は、コミットメッセージの `Editorial correction: ` の行だけから集め、`/fixpr` は訂正の段落を PR の説明の末尾に追記する（[0053 設計書](../../tasks/0053_status_comments_policy/02_architecture.md)「1.3. 設計の際に決めたこと（2026-11-01）」）
- 2026-11-02 移し替えの対象を、変更履歴のないすべての文書に広げた。承認済みの文書には、状態表の `Review date` と `Reviewer` から写した承認の行を PR 番号なしで作り、値を直さずに写し、写せないときは止まる（[0056 要件定義書](../../tasks/0056_legacy_approval_history/01_requirements.md)「issue #621 で決めたこと」「要件定義の際に決めたこと（2026-11-02）」）
- 2026-11-02 写した承認の行の印を `状態表から写した` にし、規則の節を「Documents without a change history」に改めた。写した値は `HEAD` の状態表から組み立てた項目と `cmp` で比べ、写せないときは何もコミットせずに作業ツリーに残す（[0056 設計書](../../tasks/0056_legacy_approval_history/02_architecture.md)「1.3. 設計の際に決めたこと（2026-11-02）」）
- 2026-11-02 移し替えの手順 1 で `Status` と `Comments` の行も確かめ、旧 `Comments` に `#未記入` があれば止まることにした。`/runpr` の PRE_MERGE も止まりうるとした（[0056 設計書](../../tasks/0056_legacy_approval_history/02_architecture.md)「1.4. 実装の際に決めたこと（2026-11-02）」）
- 2026-11-04 タスク文書についての記録の置き場所に ADR を加えた。ADR の状態は `有効` と `廃止` の 2 つだけとし、タスク文書の状態表（`draft` → `approved`）と `/runpr` の承認の記録を持ち込まない（[0060 要件定義書](../../tasks/0060_adr_records/01_requirements.md)「要件定義の際に決めたこと（2026-11-04）」）
- 2026-11-09 ADR の決定を変えない編集上の訂正は、タスク文書の編集上の訂正と同じく、コミットメッセージと PR の説明に記録することにした。その規則は ADR-0039 で扱う（[0070 要件定義書](../../tasks/0070_adr_writing_criteria/01_requirements.md)「要件定義の際に決めたこと（2026-11-09）」）
