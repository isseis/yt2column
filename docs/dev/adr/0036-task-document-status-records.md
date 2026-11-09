# ADR-0036: タスク文書の Comments には今の状態の理由だけを書き、状態の遷移は変更履歴に、編集上の訂正は PR の説明とコミットに記録する

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

タスク文書（`01_requirements.md`・`02_architecture.md`・`03_implementation_plan.md`）は、状態表の `Status` が `approved` になるまで次の段階に進めない。承認済みの文書にも、後から編集が入る。そのため、承認した後の編集が承認を取り消すものかどうかと、その記録をどこに残すかを決める必要がある。

承認済みの文書への編集を、決定の変更と編集上の訂正に分ける規則は、リポジトリを作ったときに go-safe-cmd-runner から移した開発の手順にあった。タスクで決めたものではない。分けないと、言い回しを直すたびに `draft` に戻り、何も決めない再承認が繰り返される（[requirements_process.md](../conventions/requirements_process.md)「Editing an approved document」）。

この規則のもとで、状態表の `Comments` が肥大化した。旧規則は、編集上の訂正を `Comments` に記録し、`draft` に戻すときは過去の承認も `Comments` に残すとしていた。issue #596 の調べでは、150 文書のうち 77 文書で `Comments` が 500 バイトを超え、最大は約 8.9KB だった。今の状態の理由と過去の経緯が 1 つのセルに混ざり、レビュアーが今読むべきことが埋もれていた（[0053 要件定義書](../../tasks/0053_status_comments_policy/01_requirements.md) §1）。

また、状態の遷移を変更履歴に書く規則より前に承認された文書には、変更履歴がない。そうした文書を決定の変更で `draft` に戻すと、`Review date` と `Reviewer` が `-` になり、過去の承認が文書から消える。2026-11-02 の時点で、変更履歴のない承認済みの文書は 155 件あった（[0056 要件定義書](../../tasks/0056_legacy_approval_history/01_requirements.md) §1）。

## 決定

-   **記録の置き場所は、読む人と読む時期で決める。** タスク文書についての記録の置き場所は 6 つである。状態表の `Comments`、変更履歴、本文の「…で決めたこと（YYYY-MM-DD）」の節、PR の説明、コミットメッセージ、ADR である。それぞれに何を置くかは、[requirements_process.md](../conventions/requirements_process.md) の「Document Status Format」の段落に従う。ADR は状態表と変更履歴を持たず、`/runpr` は ADR の承認を記録しない（[adr.md](../conventions/adr.md)「Status」）。
-   **承認済みの文書への編集を、決定の変更・セッション中の再承認・編集上の訂正の 3 つに分ける。** 決定の変更は、本文に決定を書き、文書を `draft` に戻して `/runpr` で承認し直す。`/runplan` が実装中に見つけた決定の変更は、セッションでのレビュアーの答えを再承認とし、文書は `approved` のままにする。決定と理由は、変えた文書の「実装の際に決めたこと（YYYY-MM-DD）」の節に書く。編集上の訂正は、状態も `Comments` も変更履歴も変えない。分け方と、それぞれで何を書き換えるかは、[requirements_process.md](../conventions/requirements_process.md)「Editing an approved document」に従う。
-   **`Comments` には、今の状態についてレビュアーが今知るべき理由だけを書く。** 状態が変わるたびに書き換え、前の値を残さない。状態ごとの値は、[requirements_process.md](../conventions/requirements_process.md)「Comments」に従う。
-   **状態の遷移は、文書の最後の変更履歴に 1 件 1 行で追記する。** 遷移は、承認・`draft` への戻し・セッション中の再承認・取り下げの 4 種類である。変更履歴には編集上の訂正を書かない。節は最初の 1 行を書くときに作る。`main` に入った行は、移した項目と状態表から写した承認の行を除き、その行を加えた PR の番号を持つ。PR を作る前に書いた行は未記入の印 `#未記入` を持ち、マージの直前に `/runpr` の PRE_MERGE がこの PR の番号に置き換える（[runpr.md](../../../.claude/commands/runpr.md)）。承認の行も PRE_MERGE が書く。行の形と PR 番号の例外は、[requirements_process.md](../conventions/requirements_process.md)「Change history」に従う。
-   **変更履歴のない文書は、最初の状態の遷移を書くときに移す。** 旧 `Comments` を、内容を変えずに変更履歴の先頭の項目にする。承認済みの文書なら、状態表の `Review date` と `Reviewer` から写した承認の行を作る。写した値は、`HEAD` の状態表から組み立てた項目と機械的に比べる。写せないときは、遷移を書かずに止まる。編集上の訂正だけでは移さない。対象と手順は、[requirements_process.md](../conventions/requirements_process.md)「Documents without a change history」に従う。
-   **編集上の訂正と決定の変更は、PR の説明にも書く。** 編集上の訂正は、それを加えたコミットのメッセージに訂正の行として書き、PR の説明の訂正の段落はその行から作る。PR を作るときは、ブランチの訂正の行から訂正の段落を、ブランチが加えた変更履歴の行から決定の変更を書く（[task-pr-handoff.md](../../../.claude/commands/_lib/task-pr-handoff.md) の Part B）。PR を作った後は、`/fixpr` がその回の訂正の段落を追記する。`/fixpr` が PR を作った後に `draft` に戻した決定の変更は、PR の説明に足さない（[fixpr.md](../../../.claude/commands/fixpr.md)）。訂正の行と訂正の段落の形は、[requirements_process.md](../conventions/requirements_process.md)「Recording in the PR description」に従う。`/fixpr` が PR の説明に追記する道具と、その文面を信頼しないデータとして扱うことは、[ADR-0015](0015-untrusted-review-threads.md) が定める。
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
- 2026-11-04 タスク文書についての記録の置き場所に ADR を加えた。ADR は状態表を持たず、`/runpr` の承認の記録の対象にしない（[0060 要件定義書](../../tasks/0060_adr_records/01_requirements.md)「要件定義の際に決めたこと（2026-11-04）」）
