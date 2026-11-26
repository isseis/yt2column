# 要件定義書：クロスチェックの部分レビューを重い側と軽い側の役割に分け、強めたプロンプトと単独実行のコマンドを足す

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Process | `full` |
| Created | 2026-11-26 |
| Review date | 2026-11-27 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、タスクの PR を開く前のクロスチェックの部分レビュー（[ADR-0050](../../dev/adr/0050-fable-partial-review.md)）について、次の 4 点の要件を定める。

-   PR の種類によって、重い側と軽い側の 2 つのクロスチェック役割を使い分ける。
-   両方の役割の共通のプロンプトを強める。
-   Antigravity では、重い側を Claude Code に引き継ぐ。
-   部分レビューを単独で行うコマンド `/crosscheck` を足す。

本書の新しい用語（重い側・軽い側の役割、クロスチェックの記録、引き継ぎ、再実行したコマンド、単独実行のコマンド）は 6 節で定義する。

対応 issue: [#1057](https://github.com/isseis/yt2column/issues/1057)（関連: [#1104](https://github.com/isseis/yt2column/issues/1104)・[#1105](https://github.com/isseis/yt2column/issues/1105)・[#1106](https://github.com/isseis/yt2column/issues/1106)・[#1082](https://github.com/isseis/yt2column/issues/1082)・[#1008](https://github.com/isseis/yt2column/issues/1008)）

**背景:**

クロスチェックの部分レビューは、タスクの PR を開くすべてのコマンド（`/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2`・`/runplan`）で、同じ 1 つの役割 `cross-check` のモデルを使っている（`.claude/commands/_context.md:213,217`）。担当は、Claude Code では `fable`、Antigravity では `claude-opus-5-5-medium` である。呼び出し元が渡す入力 CROSS_CHECK には、PR の種類として KIND（`documents` か `implementation`）しかない（`.claude/commands/_lib/cross-check-review.md:44-50`）。そのため、要件定義書の PR と計画書だけの PR を区別できない。

issue #1057 は、#985〜#1048 で Fable が走った 36 回を集計した。1 回あたりの Major の数は、要件だけの PR で 2.4、3 文書の PR で 2.2、設計の PR で 1.6、計画書だけの PR で 0.4、実装の PR で 0.62 だった。1 回の費用は、段階によらず約 11 分・約 26 万トークンだった。また、Fable が走った 13 回（実装の PR 8 回、計画書だけの PR 5 回）を、プロンプトを強めた Opus で再現した。強めた Opus は Fable の Major 10 件のうち 2 件を Major として、3 件を Minor として見つけ、費用はおよそ Fable の 35% だった。

今の仕組みには、次の性質がある。

-   **`not run` の記録も有効になる:** 部分レビューが走らなかったときの `not run` の行もクロスチェックの記録であり、その後に中身のある commit がなければ有効（`VALID`）と判定される（`.claude/commands/_lib/cross-check-review.md:132-174`、`.claude/commands/_lib/review-records.sh:273`）。
-   **有効かは判定のスクリプトだけが決める:** 判定のスクリプト `review-records.sh` は、レビューされるブランチの版ではなく `origin/main` の版を実行する。クロスチェックの記録が有効かは、記録の行をセッションが読んで決めず、このスクリプトだけが決める（`.claude/commands/_lib/cross-check-review.md:94-111,864-865`）。今の cross-check のモードは、`<ENTRY> <HEAD> <PLAN>` の 3 つの引数だけを受け付ける（`.claude/commands/_lib/review-records.sh:888`）。
-   **判定のスクリプトは役割の一覧を固定で持つ:** `review-records.sh` と `high-review-decision.sh` は、どちらも `Role` 列の値を `high`・`standard`・`low`・`cross-check` の 4 つに限り、ほかの値の表を拒否する（`.claude/commands/_lib/review-records.sh:379-380`、`.claude/commands/_lib/high-review-decision.sh:206`）。両スクリプトの共通のテスト入力は `cmd/yt2column/harness_mapping_cases_test.go` にある。`review-records.sh` は、どのハーネスの行の検証パターンもまとめて読む（`.claude/commands/_lib/review-records.sh:419-423`）。
-   **Antigravity からは Fable を呼べない:** Antigravity のモデル固定ワーカーは、`swarm` で選べるモデルに限られる（`.claude/commands/_lib/harness-antigravity.md` の「8. pinned-worker」）。Antigravity から Claude Code の CLI を呼ぶ案は [#1082](https://github.com/isseis/yt2column/issues/1082) に分けてある（ADR-0050「採らなかった案」）。
-   **再実行は止まる:** ブランチに要件定義書があると、`/mkreq` は再実行しても step 1 で止まる（`.claude/commands/mkreq.md:84-95`。例外は簡易プロセスの再開だけ）。`02_architecture.md` があると、`/mkarch` も止まる（`.claude/commands/mkarch.md:51`）。
-   **書き手のモデル:** Claude Code では、`/mkreq`・`/mkarch`・`/mkplan` と、実装モデル要件が `high` の `/runplan` は `opus` で動く（`.claude/commands/_context.md:174-175`）。

**本書の記述範囲:** 本書は、役割とモデルの対応、PR の種類ごとの役割、クロスチェックの記録が有効かの判定、Antigravity での引き継ぎと再実行、`/crosscheck` の振る舞い、共通のプロンプトに足す義務と制約を定める。次の事項は `02_architecture.md` で決める。

-   役割の名前と `_context.md` の「Harness model mapping」の表の形（effort の表し方、Antigravity の重い側の行の形）。
-   PR の種類を判定のスクリプトと `/crosscheck` に渡す方法と、本タスク自身の PR を開く方法。
-   再実行の入口と `/crosscheck` の手順の組み立て。

設計へ引き継ぐ論点は [design_handoff.md](design_handoff.md) に申し送る。

**issue #1057 で決めたこと:**

-   **PR の種類ごとの役割:** 要件・設計・3 文書の PR（`/mkreq`・`/mkarch`）は重い側の役割（今の `cross-check`）を使う。計画書だけの PR（`/mkplan`・`/mkplan2`）と実装の PR（`/runplan`）は、新しく設ける軽い側の役割を使う。理由は、1 回あたりの Major の数が計画書と実装の PR で少なく、1 回の費用は段階によらず一定だからである。
-   **Claude Code のモデル:** 重い側は `fable` を続け、軽い側は `opus`（effort `high`）にする。理由は、再現実験で、強めた Opus が Fable のおよそ 35% の費用で Fable の Major の一部と Opus にしかない本物の指摘を見つけたからである。
-   **並走期間を設けない:** 切り替え前の並走期間（5〜6 件）は設けず、本タスクで直接切り替える。切り替え後の効果と課題は実運用の記録で追い、#1105・#1106 の材料にする。
-   **強めたプロンプトを共通の手順に入れる:** 次の 6 つの義務と、範囲の終点より後の情報（`main` など）を読まない制約を、クロスチェックの部分レビューの共通のプロンプト（`.claude/commands/_lib/cross-check-review.md`）に入れる。重い側にも効かせ、プロンプトを 2 つに分けて管理しないためである。
    -   スコープ一覧の除外を判定し直す。
    -   決定や値を変えたときは、古い記述が残っていないか全体を探す。
    -   外部の挙動についての主張は、実験で確かめる。
    -   検査が誤って通る入力を作ってみる。
    -   文書にない選択ごとに、人の答えが記録されているかをたどる。
    -   重大度を結果の重さで決める。
-   **単独実行のコマンドを足す:** 既存のブランチの上で部分レビューを行い、トリアージ・修正・記録の commit まで行い、push はしないコマンドを足す。
-   **記録の形は変えない:** 範囲の決め方・記録の形・有効性の判定の枠組みは保ち、`review-records.sh` のモデルの検証を新しい役割に対応させる。
-   **保留にする案:** 実装のレビューをタスクの最後に 1 回だけ行う案は、本タスクでは扱わない。追加の節約が小さく、見つかるのが遅れ、レビューの関門の作り直しが要るためである。

**要件定義の際に決めたこと（2026-11-26）:**

以下をレビュアーと確認して決めた。issue の「改訂方針（2026-11-26）」の Antigravity の割り当て（重い側・軽い側とも `claude-opus-5-5-high`）は、2 つ目の項目で改めた。

-   **進め方の選択:**
    -   **除外判定:** 簡易プロセスの除外に当たると判断した。レビューのスレッドのテキストの扱い（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）・`docs/dev/security.md` の規則・外部のサービス（プロバイダは Anthropic のまま）は変えない。しかし、クロスチェックの記録が有効かの判定に PR の種類という入力を足し、レビューのモデルを変える。これはレビューの関門の変更（`.claude/commands/_lib/pr-review-escalation.md:57` の基準 4）であり、開発ツールの信頼の境界に関わる。`docs/dev/conventions/requirements_process.md`（「Simple Process」の「Excluded changes」）の「迷う場合は除外とする」規則を当てはめた。最初の質問では「除外に当たらない」と示したが、自己レビューの指摘で改めた。
    -   **決定と理由:** 完全プロセス（`full`）を採る（除外判定は覆さない）。変更が `_context.md`・2 つの判定のスクリプト・共有の手順・5 つのコマンド・2 つのアダプタ・新しいコマンドとその入口・Go のテストに及ぶためである。ADR-0050 の決定も変え、新しい仕組み（役割とコマンド）も作る。
    -   **ADR の判断:** ADR なし（本タスクの進め方の選択であり、タスク内で閉じるため）。
-   **Antigravity の重い側のモデルと走らせ方（F-001・F-003）:**
    -   **決定と理由:** Antigravity の重い側の役割も `fable` にする。Antigravity から `fable` は呼べないので、Antigravity のセッションは重い側の部分レビューに達したら止まり、利用者が Claude Code で `/crosscheck` を走らせる。その後、利用者は Antigravity に戻って残りの作業を再開する。重い側は 1 回あたりの Major が多い PR を見るので、ハーネスによらず同じモデルにそろえる。引き継ぎを将来は `chief` が制御する。
    -   **ADR の判断:** ADR-0050 を更新する（決定 4 のハーネスごとのモデル）。
-   **Antigravity の止まり方と再実行（F-003）:**
    -   **決定と理由:** Antigravity は、重い側の部分レビューに達したら push の前に止まる。`/crosscheck` の後に同じコマンドを再実行すると、PR を開く手前から続ける。PR を開いてから見る案は採らない。人のレビューを直した後の PR から始めるためで、ADR-0050 の決定 1 の理由と同じである。
    -   **ADR の判断:** ADR-0050 を更新する（決定 4 の走らないときの扱い）。
-   **Antigravity の軽い側のモデル（F-001）:**
    -   **決定と理由:** Antigravity の軽い側は、Claude Code の軽い側（`opus`、effort `high`）と effort をそろえて `claude-opus-5-5-high` にする。利用者の環境に残っていた 2026-11-21 の `swarm(action: "models", all: true)` の出力には、`claude-opus-5-5-high` の名前と割り当ての残りがある。しかし、実際に走らせてはいない。タスク 0120 で実測したのは `claude-opus-5-5-medium` だけである（`docs/tasks/0120_harness_independent_commands/02_architecture.md` 1.4）。そのため、走らせたときの時間・出力の形・割り当ての消費を `/mkarch` の段階で確かめる（[design_handoff.md](design_handoff.md) の H-01）。使えなければ、要件の決定の変更として本書に戻す。
    -   **ADR の判断:** ADR-0050 を更新する（決定 4 のハーネスごとのモデル）。
-   **`/crosscheck` を走らせる条件（F-004）:**
    -   **決定と理由:** `/crosscheck` は、ブランチの最後のクロスチェックの記録が `not run`（理由を問わない）のとき、記録がないとき、または記録が有効でないときに部分レビューを行う。最後の記録が有効な `run` の記録なら、何もせずに報告する。どのときも commit は push せず、push は利用者が行う。記録を問わずに PR の全体をいつも見直す案は採らない。有効な `run` の記録があっても費用が 1 回分かかるためである。
    -   **ADR の判断:** ADR-0050 を更新する（単独実行のコマンドを足す）。
-   **`/crosscheck` の判定の失敗（F-004）:**
    -   **決定と理由:** 記録が有効かの判定そのものが失敗したとき（`git fetch origin main` の失敗、`origin/main` の表が読めないなど）、`/crosscheck` は部分レビューも commit もせずに止まり、失敗の理由を報告する。`/crosscheck` は push しないので、PR を開くコマンドのように安全側として走らせる必要がない。判定が失敗している間は、作った記録が数えられるかも分からない。PR の全体をレビューする案は採らない。記録が数えられるか分からないまま、費用を 1 回分使うためである。
    -   **ADR の判断:** ADR-0050 を更新する（単独実行のコマンドの範囲）。
-   **`/crosscheck` と merge の後の確認（F-004）:**
    -   **決定と理由:** 最後の記録が有効な `run` の記録で、その後に merge の後の確認を待つ merge commit だけがある（判定が `MERGE`）とき、`/crosscheck` は部分レビューも merge の後の確認も行わずに報告する。AC-14 で止まったブランチでは、再実行したコマンドのチェックポイントの入口と push の直前の入口が確認する。PR を開いた後のブランチの merge は、今も確認しない。`/crosscheck` に merge の後の確認もさせる案は採らない。コマンドの役目が広がるためである。
    -   **ADR の判断:** ADR-0050 を更新する（単独実行のコマンドの範囲）。
-   **記録の役割と PR の種類の照合（F-002）:**
    -   **決定と理由:** 重い側の役割を使う PR（重い側の PR）では、重い側の役割のモデルで走った `run` の記録だけを有効とする。軽い側の役割を使う PR（軽い側の PR）では、どちらの役割の `run` の記録も有効とする。重い側の記録は、重い側のモデルが同じ範囲を見たものなので、軽い側の PR で見直す必要がない。切り替えの前から作業中のブランチにある Fable の記録も、軽い側の PR ではそのまま有効になる。軽い側の PR でも、重い側の PR と同じく記録の役割を PR の役割に完全に一致させる案は採らない。作業中のブランチの Fable の記録が無効になり、もう一度走るためである。
    -   **ADR の判断:** ADR-0050 を更新する（決定 5 の有効かの判定）。
-   **PR の種類を渡さない呼び出し（F-002）:**
    -   **決定と理由:** 判定のスクリプトを PR の種類を渡さずに呼んだときは、どちらの役割の `run` の記録も数える（軽い側の PR と同じ判定）。本タスクを `main` に入れる前から作業中のブランチは、古い手順のまま新しいスクリプトを呼ぶ。そうしたブランチを、ハーネスによらず止めないためである。代わりに、移行期に古い手順のままの Antigravity の重い側の PR が、`claude-opus-5-5-medium` の記録で開きうる。この危険を受け入れる。
    -   **採らなかった案:** 重い側の PR とみなす案は採らない。古い手順のままの Antigravity のブランチは、`claude-opus-5-5-medium` の記録が数えられず、手順がもう一度同じモデルで走らせてもチェックポイントで止まる。`main` を取り込むまで PR を開けず、レビュー 2 回分の割り当ても使う（クロスチェックの部分レビューの指摘で分かった。最初に人に示した「もう一度求められうる」という説明は誤りだった）。失敗として扱う案も採らない。古い手順のブランチが、ハーネスによらず push の直前で止まるためである。
    -   **ADR の判断:** ADR-0050 を更新する（決定 5 の有効かの判定）。
-   **誤った PR の種類を渡される危険（F-002）:**
    -   **決定と理由:** PR の種類は、レビューされるブランチにあるコマンドの定義が宣言する。そのため、ブランチが誤った種類を宣言すると、重い側の PR が軽い側の記録で開きうる。この危険を受け入れる。今もブランチはコマンドの定義を変えて判定の呼び出しそのものを飛ばせるので、新たに広がる危険ではないためである。種類をブランチが選べない事実（`origin/main` の側）から求める案は採らない。設計と実装が大きくなるためである。読めない値は、重い側の PR とみなさず、有効でないと判定する（AC-11）。
    -   **ADR の判断:** ADR-0050 を更新する（結果に受け入れる危険を書く）。
-   **Claude Code の軽い側と書き手の同一モデル（F-001）:**
    -   **決定と理由:** Claude Code では、軽い側のレビュー（`opus`）が、`/mkplan` と実装モデル要件が `high` の `/runplan` の書き手と同じモデルになる。別系統のモデルによる確かめではなくなるが、受け入れる。issue #1057 の再現実験で強めた Opus が効いたことと、書き手とは別の文脈のワーカーが見ることを根拠とする。
    -   **ADR の判断:** ADR-0050 を更新する（結果に受け入れる危険を書く）。
-   **`not run` の記録と再実行（F-003）:**
    -   **決定と理由:** `/crosscheck` でも重い側が走らず `not run` と記録されたときは、再実行したコマンドはその記録で PR を開く。部分レビューは追加のレビューで PR を止めないという、ADR-0050 の決定 4 の方針どおりである。`run` の記録ができるまで止める案は採らない。Fable の枠が戻るまでタスクが止まるためである。この決定から、`not run` の記録は、PR の種類によらず今と同じく有効と判定される（AC-10）。`not run` の記録にはモデルがないので、役割を照らし合わせられないためでもある。
    -   **ADR の判断:** ADR-0050 を更新する（決定 4 の走らないときの扱い）。
-   **単独実行のコマンドの名前（F-004）:**
    -   **決定と理由:** ほかのコマンド（`/weakreview`・`/japrose`）と同じく、区切りのない小文字にする。[ADR-0055](../../dev/adr/0055-harness-independent-commands.md) に従い、`.agents/skills/crosscheck/SKILL.md` の薄い入口も足す。
    -   **ADR の判断:** ADR なし（コマンドの名前はタスク内で閉じるため）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   品質をさほど落とさずに、計画書だけの PR と実装の PR の部分レビューの費用を下げる。
-   共通のプロンプトを強め、両方の役割の見落としを減らす。
-   Antigravity でも重い側の PR を Fable に見せられるようにする。
-   部分レビューが走らなかったブランチを、後から単独で見直せるようにする。

### 2.2. スコープ (In Scope)

-   重い側と軽い側の 2 つのクロスチェック役割と、ハーネスごとのモデルの割り当て（F-001）。
-   PR を開くコマンドごとの役割の使い分け（F-001）。
-   クロスチェックの記録が有効かの判定での、記録の役割と PR の種類の照合（F-002）。
-   Antigravity で重い側に達したときの停止と、同じコマンドの再実行（F-003）。
-   単独実行のコマンド `/crosscheck` と、その Antigravity の入口（F-004）。
-   共通のプロンプトに足す 6 つの義務と、範囲の終点より後の情報を読まない制約（F-005）。
-   ADR-0050・`CLAUDE.md`・用語集など、関係する文書の更新（F-006）。

### 2.3. スコープ外 (Out of Scope)

-   **引き継ぎの `chief` による制御:** Antigravity と Claude Code の間の引き継ぎを `chief` が制御することは、将来の別タスクとする（要件定義の際に決めたこと）。`_context.md` の「Command routing」の表には `/crosscheck` の行を足さない。
-   **Antigravity から Claude Code の CLI を呼ぶこと:** [#1082](https://github.com/isseis/yt2column/issues/1082) で扱う。
-   **実装のレビューをタスク単位で 1 回にする案:** 保留とする（issue #1057 で決めたこと）。
-   **異系統の LLM のローカルレビュー:** [#1106](https://github.com/isseis/yt2column/issues/1106) で扱う。
-   **記録の分類と集計:** [#1105](https://github.com/isseis/yt2column/issues/1105) と [#936](https://github.com/isseis/yt2column/issues/936) で扱う。
-   **merge の後の確認と上位モデルのレビュー:** モデル（`high` 役割）・手順・記録を変えない。`/crosscheck` も merge の後の確認を行わない（要件定義の際に決めたこと）。
-   **記録の行の形:** 「Record lines」の各行の形（`.claude/commands/_lib/cross-check-review.md:719-730`）と `not run` の理由の値は変えない（issue #1057 で決めたこと）。役割は記録の `model <model-id>` から分かる。重い側と軽い側の検証パターンは重ならないためである。
-   **OpenCode:** OpenCode で動く `/runplan` の部分レビューは、今と同じく `not run agent-cannot-call` とする（`.claude/commands/_lib/cross-check-review.md:193-198`）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 重い側と軽い側の役割とモデルの割り当て

クロスチェックの部分レビューの役割を、重い側と軽い側の 2 つに分ける。PR を開くコマンドごとに、どちらの役割を使うかを決める。ハーネスごとのモデルは `_context.md` の「Harness model mapping」に置く。

**Acceptance Criteria**:
- **AC-01**: 次のコマンドが起動する部分レビューは、重い側の役割で走る。ローカルの CLI レビューと並列に起動するもの（`_lib/task-pr-handoff.md` Part B step 1）も、チェックポイントの入口と push の直前の入口が起動するものも同じである。
    -   `/mkreq`（完全プロセスと簡易プロセスの両方）
    -   `/mkarch`
- **AC-02**: 次のコマンドが起動する部分レビューは、軽い側の役割で走る。起動する箇所は AC-01 と同じである（`/runplan` は step 7.7 とチェックポイントの入口）。
    -   `/mkplan`
    -   `/mkplan2`
    -   `/runplan`
- **AC-03**: Claude Code では、重い側の部分レビューは `fable` のモデルで走り、記録の `model` は `claude-fable-` で始まる ID になる。軽い側の部分レビューは `opus` のモデルで走り、記録の `model` は `claude-opus-` で始まる ID になる。Claude Code のアダプタの手順は、軽い側のワーカーを effort `high` で起動すると書いている。
- **AC-04**: Antigravity では、軽い側の部分レビューは `claude-opus-5-5-high` のモデルで走り、記録の `model` は `claude-opus-5-5-high` になる。重い側は Antigravity では起動せず、F-003 のとおり止まる。
- **AC-05**: 重い側と軽い側の割り当ては、ハーネスごとの表 1 か所（`_context.md` の「Harness model mapping」）にある。`.claude/commands/_lib/cross-check-review.md` と、それが組み立てるプロンプトは、モデルの版を含む具体的なモデルの ID を書かない（今の規則、`.claude/commands/_lib/cross-check-review.md:31-33`）。
- **AC-06**: 2 つの判定のスクリプト（`review-records.sh`・`high-review-decision.sh`）は、新しい役割を含む `origin/main` の表を受理する。表の検証と照合の規則は、今の規則（フェイルクローズの表の検証、正規表現でない照合）を保つ。表に重い側か軽い側のどちらかの検証パターンがないときは、`review-records.sh` はフェイルクローズで失敗する。
- **AC-07**: merge の後の確認は、今と同じく `high` 役割のモデルで走る。OpenCode で動く `/runplan` の部分レビューは、今と同じく `not run agent-cannot-call` と記録される。

#### F-002: クロスチェックの記録が有効かの判定での役割の照合

クロスチェックの記録が有効かを判定するとき、`run` の記録のモデルが、PR の種類の求める役割に合うかを照らし合わせる。検証パターンは、今と同じく、どのハーネスの行のものもまとめて使う。

**Acceptance Criteria**:
- **AC-08**: 重い側の PR では、`model` がどの重い側の検証パターンにも合わない `run` の記録を、クロスチェックの記録として数えない。合わない例は、軽い側のパターンにだけ合う `model` である。数えない記録しかなければ、記録がないときと同じく、判定は merge-base からの `MISSING` になる。
- **AC-09**: 軽い側の PR では、`model` が重い側か軽い側のどちらかの検証パターンに合う `run` の記録が、今の規則のとおり有効になりうる。本タスクを `main` に入れる前に、作業中のブランチに書かれた `model claude-fable-…` と `model claude-opus-5-5-medium` の記録も含む。
- **AC-10**: `not run` の記録は、PR の種類によらず、今の規則のとおり有効になりうる。
- **AC-11**: 判定のスクリプトが受け付ける PR の種類の値は、重い側と軽い側を表す 2 つだけである。ほかの値、空の値、または読めない値を渡されたときは、今の規則のとおり失敗する。その失敗は、呼び出し元が `MISSING <merge-base>` と読むものである（`.claude/commands/_lib/cross-check-review.md:125-130`）。誤って `VALID` にはならない。
- **AC-12**: PR の種類を渡さない呼び出し（本タスクより前の手順の呼び方）では、`model` が重い側か軽い側のどちらかの検証パターンに合う `run` の記録が、今の規則のとおり有効になりうる。空の値を渡すことは、渡さないことに当たらない（AC-11）。
- **AC-13**: 本タスクの実装の PR は、判定のスクリプトが PR の種類を受け付ける前の `origin/main` の上でも、記録が有効と判定されて PR を開ける。確かめ方は設計で決める（[design_handoff.md](design_handoff.md) の H-03）。

#### F-003: Antigravity での重い側の停止と再実行

Antigravity のセッションは、重い側の部分レビューに達したら、push の前に止まる。利用者が Claude Code で `/crosscheck` を走らせた後に同じコマンドを Antigravity で再実行すると、PR を開く手前から続ける。

**Acceptance Criteria**:
- **AC-14**: Antigravity の `/mkreq` か `/mkarch` で、重い側の部分レビューを起動する箇所に達し、判定が `MISSING` のとき、次のとおり止まる。AC-01 のどの起動箇所でも同じである。
    -   部分レビューのワーカーを起動しない。
    -   クロスチェックの記録（`run`・`not run`）を書かない。
    -   push せず、PR を開かない。
    -   ブランチの commit を消さない。
- **AC-15**: AC-14 で止まったときの報告は、次の 4 つを含む。
    -   重い側の部分レビューを Antigravity で走らせられないこと。
    -   同じ worktree で Claude Code を開き、同じブランチのまま `/crosscheck` を走らせること。
    -   その後に Antigravity で、同じ worktree で同じコマンド（`/mkreq <N>`、`/mkarch <task>`）を再実行すること。
    -   止まったブランチの名前。
- **AC-16**: AC-14 で止まったブランチで `/crosscheck` がクロスチェックの記録を作った後、Antigravity で同じコマンドを再実行すると、PR を開く手前から続ける。再実行したコマンドは、次のことをしない。
    -   新しいタスク番号を決めない。
    -   新しいブランチを作らない。
    -   決定を人にもう一度問わない。
    -   書いた文書を書き直さない。

    再実行したコマンドは、記録が有効なら部分レビューを起動せずに、`_lib/task-pr-handoff.md` Part B の残りの手順を行って PR を開く。残りの手順で作る commit（CHECKS の直しなど）は、`/crosscheck` の記録を無効にしない。そのため、再実行したコマンドが AC-14 でもう一度止まることはない。
- **AC-17**: `/crosscheck` を走らせずに再実行したとき（判定が `MISSING` のまま）は、AC-14・AC-15 と同じく止まる。
- **AC-18**: `/crosscheck` の記録が `not run` のとき、再実行したコマンドは止まらずに PR を開く。PR の説明の「クロスチェックの部分レビュー」の節には、その `not run` の記録が理由とともに出る。
- **AC-19**: 同じコマンドの再実行で続けられるのは、AC-14 で止まったブランチで、まだ PR のないものだけである。それ以外の状態では、今の規則のとおり止まる。たとえば、ブランチに PR がある、作業ツリーに変更がある、要件定義書がほかのブランチにもある、といった状態である（`.claude/commands/mkreq.md:84-95`、`.claude/commands/mkarch.md:51`）。
- **AC-20**: Claude Code で動く `/mkreq`・`/mkarch` は、F-003 の停止をしない。今と同じく、重い側の部分レビューを自分で起動する。

#### F-004: 単独実行のコマンド `/crosscheck`

今のブランチの上で部分レビューを単独で行い、トリアージ・修正・修正のローカルの CLI レビュー・記録の commit まで行う。push はしない。

**Acceptance Criteria**:
- **AC-21**: `.claude/commands/crosscheck.md` と `.agents/skills/crosscheck/SKILL.md` がある。後者は、ほかのコマンドの入口と同じく薄い入口である。`CLAUDE.md` の「Claude Commands」の一覧に `/crosscheck` がある。
- **AC-22**: `/crosscheck` は、今のブランチの PR の種類に応じて、重い側か軽い側の役割で走る。`/mkreq`・`/mkarch` のブランチなら重い側、`/mkplan`・`/mkplan2`・`/runplan` のブランチなら軽い側である。種類を決める方法は設計で決める（[design_handoff.md](design_handoff.md) の H-03）。種類を決められないときは、commit を作らずに止まり、理由を報告する。
- **AC-23**: `/crosscheck` の振る舞いは、ブランチの最後のクロスチェックの記録（F-002 の照合で数えるもの）と判定の結果によって次のとおり決まる。最後の記録が `run` か `not run` かは、判定のスクリプトの出力から得る。セッションが記録の行を読んで決めることはしない（今の規則、`.claude/commands/_lib/cross-check-review.md:864-865`）。

    | 最後のクロスチェックの記録 | 判定の結果 | 振る舞い |
    |---|---|---|
    | `run` | `VALID` | 部分レビューを行わず、commit を作らず、行う必要がないことを報告する |
    | `run` | `MERGE` | 同上。merge の後の確認も行わない |
    | `not run`（理由を問わない） | `VALID` または `MERGE` | 部分レビューを行う |
    | どれか、またはなし | `MISSING` | 部分レビューを行う |
    | どれか、またはなし | 判定の失敗 | 部分レビューも commit もせずに止まり、失敗の理由を報告する |

    数える記録がないときに判定が `VALID` か `MERGE` になる組み合わせは起こらない。部分レビューの範囲は、最後に数える `run` の記録より後のすべての commit を含む（数える `run` の記録がなければ merge-base から HEAD まで）。`not run` の記録の範囲は、続けて 2 つ以上あっても、すべて範囲に入る。範囲に merge commit があれば、範囲は merge-base から HEAD までとする。範囲の求め方は設計で決める（[design_handoff.md](design_handoff.md) の H-05）。

- **AC-24**: AC-23 で部分レビューを行うときは、指摘を今の手順（`.claude/commands/_lib/cross-check-review.md` の「The standalone path」）と同じく扱う。
    -   トリアージし、直す。
    -   直しをローカルの CLI レビューで確かめる。
    -   記録の行を記録の commit に書く。

    記録の行の形は、今の「Record lines」の形である。
- **AC-25**: 実行中のハーネスの表の行で、その役割のワーカーを起動できないとき、`/crosscheck` はクロスチェックの記録を書かず、commit を作らずに止まる。報告には、起動できない役割と、起動できるハーネスを含める。たとえば、Antigravity で重い側を求められたとき（その行は起動できない形）や、OpenCode で動いたときである。
- **AC-26**: 起動はできるが部分レビューが走らなかったとき（モデルの一覧にない、割り当ての残りがない、利用上限、時間切れ、モデルの不一致など）、`/crosscheck` は `not run <reason>` の記録の commit を作り、その理由を報告する。今の理由の値を使う。
- **AC-27**: `/crosscheck` は、次のことをしない。
    -   push しない。
    -   GitHub に書き込まない（PR・issue・コメント・レビューのスレッド）。
    -   PR を作らない。

    最後の報告には、作った commit の一覧と、次の手順を含める。次の手順は、AC-14 で止まったブランチなら、Antigravity で同じ worktree のまま元のコマンドを再実行することである。それ以外のブランチなら、利用者が push することである。あわせて、ブランチに PR があれば、その説明の「クロスチェックの部分レビュー」の節は `/crosscheck` の後も古いままであり、記録は commit にあることも報告する。この一文は、PR があるかを調べずに、いつも報告に含める。
- **AC-28**: 作業ツリーに変更があるとき、`/crosscheck` は何も commit も破棄もせずに止まり、変更のあるファイルを報告する。
- **AC-29**: `/crosscheck` は、ワーカーについて、今の部分レビューと同じ状態の比較を行う（`.claude/commands/_lib/cross-check-review.md` の「The state check」）。ワーカーに課す制約（作業ツリーとリポジトリの状態を変えない、ネットワークを使わない、秘密のファイルを読まない）も同じである。状態が変わっていたら、結果を使わず、commit を作らずに止まる。
- **AC-30**: `.claude/commands/crosscheck.md` は、起動で承認するネットワークの操作を書いている。それは、判定に要る `git fetch origin main` と、ローカルの CLI レビュー（`coderabbit review`）だけである。書き方は、`_lib/task-pr-handoff.md` の「Authorization」と同じ形である。

#### F-005: 共通のプロンプトの強化

クロスチェックの部分レビューの共通のプロンプト（`.claude/commands/_lib/cross-check-review.md` の「The prompt」）に、6 つの義務と 1 つの制約を足す。両方の役割の部分レビューと `/crosscheck` に、同じプロンプトが効く。

**Acceptance Criteria**:
- **AC-31**: 共通のプロンプトは、ワーカーに次の 6 つの義務を課す。
    -   スコープ一覧で除外された箇所が基準に当たらないかを判定し直す。
    -   決定や値を変えた箇所について、古い記述が範囲の全体に残っていないかを探す。
    -   外部の挙動についての主張を、作業ツリーの外でのローカルな実験で確かめる。ネットワークと本物のサービスは使わない（今の制約のとおり）。ローカルで確かめられない主張は、確かめられないことを指摘に書く。
    -   検査やテストが誤って通る入力を作ってみる。
    -   文書にない選択ごとに、人の答えが記録されているかをたどる。
    -   重大度を、起こりうる結果の重さで決める。
- **AC-32**: 共通のプロンプトは、範囲の終点（RANGE の END）より後の情報を読むことを禁じる。`origin/main`・範囲の後の commit・ほかのブランチの内容が、その情報に当たる。ワーカーは、先行文書と ADR を、範囲の終点の内容で読む。
- **AC-33**: `.claude/commands/_lib/cross-check-review.md` の「The prompt」の節の、merge-base からの差分は、AC-31 の 6 つの義務と AC-32 の制約の追加に限る。

#### F-006: 文書の更新

関係する文書を、本書の決定に合わせる。

**Acceptance Criteria**:
- **AC-34**: ADR-0050 の「背景」「決定」「採らなかった案」「結果」「経緯」は、本書の決定を反映する。反映するのは、次の 4 つである。
    -   重い側と軽い側の役割と、ハーネスごとのモデル。
    -   Antigravity での重い側の引き継ぎ。
    -   記録の役割の照合。
    -   `/crosscheck` と共通のプロンプトの強化。

    「結果」には、本書で受け入れる危険を書く。
- **AC-35**: `CLAUDE.md` の部分レビューの説明（「Claude Commands」のうち、クロスチェックの部分レビューを述べる段落）は、2 つの役割の使い分けと Antigravity での引き継ぎを述べる。`docs/translation_glossary.md` は、次の 2 点を満たす。
    -   「クロスチェックの部分レビュー」の行が 2 つの役割を述べる。
    -   重い側の役割、軽い側の役割、引き継ぎの用語がある。
- **AC-36**: 2 つのハーネスのアダプタ（`.claude/commands/_lib/harness-claude.md`・`harness-antigravity.md`）の `pinned-worker` の節は、2 つの役割を扱う。Antigravity のアダプタには、重い側を起動しないことが書かれている。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   軽い側の PR の部分レビューの 1 回の費用（トークン数と時間）は、記録の行の `<seconds>s, tokens <tokens>` で追う。本タスクでは目標の値を設けない。切り替え後の実運用の記録を #1105・#1106 の材料にする（issue #1057 で決めたこと）。

### 4.2. セキュリティ (Security)

-   **`docs/dev/security.md`:** `N/A`。本タスクは `yt-dlp` の呼び出し・API キー・キャッシュ・LLM API への送信・信頼しないテキストの埋め込みを変えない（`.claude/commands/_context.md` の「Conditional-guide trigger」に当たらない）。
-   **レビューの関門:** 本タスクは、クロスチェックの記録が有効かの判定に PR の種類という入力を足す。種類は、レビューされるブランチにあるコマンドの定義が宣言する。誤った種類を宣言する危険は、受け入れる（要件定義の際に決めたこと）。読めない値は有効でないと判定する（AC-11）。種類を渡さない呼び出しはどちらの役割の記録も数える（AC-12）ので、移行期に古い手順のままの重い側の PR が軽い側のモデルの記録で開く危険も受け入れる（要件定義の際に決めたこと）。
-   **軽い側の effort:** クロスチェックの記録の `model` には effort が出ないので、軽い側が effort `high` で走ったかを記録からは確かめられない。この危険は受け入れる。手順の書き方で確かめる（AC-03）。
-   **同じモデルによるレビュー:** Claude Code の軽い側のレビューは、`/mkplan` と実装モデル要件が `high` の `/runplan` の書き手と同じモデルになる。この危険は、受け入れる（要件定義の際に決めたこと）。
-   **ワーカーの制約と状態の比較:** 部分レビューのワーカーに課す制約と状態の比較（ADR-0050 の決定 6）は、両方の役割と `/crosscheck` に同じく効く（AC-29）。
-   **ワーカーの出力:** 出力はデータとして扱い、その文をコミットメッセージにも PR の説明にも写さない（`.claude/commands/_lib/cross-check-review.md` の「Checking the output」）。この規則は `/crosscheck` にも効く。
-   **`/crosscheck` のネットワーク操作:** 起動で承認するのは、`git fetch origin main` と `coderabbit review` だけである（AC-30）。push と GitHub への書き込みはしない（AC-27）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   **判定の失敗:** PR を開くコマンドでは、判定のスクリプトの失敗と読めない出力は、今と同じく安全側（有効でない）に倒す（AC-11）。`/crosscheck` では、判定が失敗したら止まる（AC-23）。
-   **重い側を走らせられないとき:** Antigravity の重い側の PR は、利用者が Claude Code で `/crosscheck` を走らせるまで PR を開けない（AC-14・AC-17）。Claude Code でも Fable が走らないときは `not run` と記録して進む（AC-18・AC-26）ので、Fable の枠が尽きてもタスクは止まらない。
-   **`/crosscheck` の文脈:** `/crosscheck` は、PR を開くコマンドとは別のセッションで動く。そのため、PR を開くコマンドのセッションが却下した自己レビューの指摘（SELF_REJECTED）を渡せず、同じ指摘がもう一度出ることがある。この重複は、トリアージで扱う。

### 4.4. 互換性 (Compatibility)

-   **作業中のブランチの記録（軽い側の PR）:** 切り替えの前から作業中の軽い側の PR のブランチにある Fable と `claude-opus-5-5-medium` の記録は、そのまま有効になる（AC-09・AC-12）。検証パターンは、どのハーネスの行のものもまとめて使うためである。
-   **作業中のブランチの記録（重い側の PR）:** 切り替えの前から作業中の、Antigravity の重い側の PR のブランチにある `claude-opus-5-5-medium` の `run` の記録は、ブランチが `main` を取り込んで手順が PR の種類を渡すようになると、数えなくなる（AC-08）。そのブランチは、F-003 のとおり Claude Code での `/crosscheck` を求められる。この扱いを受け入れる。重い側の PR は重い側のモデルで見るという決定に従うためである。
-   **古い手順の呼び出し:** 古い手順のまま作業中のブランチは、PR の種類を渡さずに `origin/main` の新しいスクリプトを呼ぶ。この呼び出しは、どちらの役割の記録も数え、失敗しない（AC-12）。そのため、古い手順のままのブランチは、ハーネスによらず止まらない。古い手順のままの Antigravity の重い側の PR は、`claude-opus-5-5-medium` の記録で開きうる。この危険は受け入れる（要件定義の際に決めたこと）。
-   **本タスク自身の PR:** 本タスクの PR は、新しいスクリプトがまだ `origin/main` にない間も開ける（AC-13）。
-   **記録の行:** 記録の行の形は変えないので、PR の説明と `/runpr` の読み取りは変わらない（2.3 節）。

### 4.5. 保守性 (Maintainability)

-   ハーネスごとのモデルの割り当ては、`_context.md` の表 1 か所に置く（AC-05）。モデルを変えるときは、その表と、両スクリプトの共通のテスト入力（`cmd/yt2column/harness_mapping_cases_test.go`）を直す。
-   強めたプロンプトは共通のプロンプト 1 つに置き、役割ごとに分けない（issue #1057 で決めたこと）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   ADR-0050 のうち本書が変えない決定（PR を開く前に 1 回行う、4 つの基準に絞る、まとめたトリアージ、記録を無効にしない commit、状態の比較、記録の行）は保つ。
-   ADR-0055 の、手順の正本を `.claude/commands/` に置き、ハーネスに固有の操作をアダプタに分ける決定に従う。
-   エージェントは、インストールする開発ツールをインストールもビルドもしない（`CLAUDE.md` の「Tool Execution Safety」）。本タスクはプロトコル番号のあるツールを変えない見込みである。変えるときは「Protocol numbers」の規則に従う。

## 6. 用語集 (Glossary)

-   **クロスチェックの部分レビュー:** `docs/translation_glossary.md` の定義のとおり。
-   **重い側の役割（重い側）:** 要件・設計・3 文書の PR のクロスチェックの部分レビューに使う役割。Claude Code・Antigravity とも `fable` を割り当てる。Antigravity からは起動せず、Claude Code の `/crosscheck` で走らせる。
-   **軽い側の役割（軽い側）:** 計画書だけの PR と実装の PR のクロスチェックの部分レビューに使う役割。Claude Code では `opus`（effort `high`）、Antigravity では `claude-opus-5-5-high` を割り当てる。
-   **重い側の PR:** `/mkreq`（完全プロセス・簡易プロセス）と `/mkarch` が開く PR。
-   **軽い側の PR:** `/mkplan`・`/mkplan2`・`/runplan` が開く PR。
-   **重い側のモデル・軽い側のモデル:** それぞれの役割に割り当てたモデル。「上位モデルのレビュー」の `high` 役割のモデルとは別である。
-   **クロスチェックの記録:** `Cross-check review: run` か `Cross-check review: not run` の行（古い `Fable review:` の行を含む）。`.claude/commands/_lib/cross-check-review.md` の「Reading the record lines」のとおり。用語集の「記録」（拡張のジョブの記録）とは別である。
-   **有効な記録:** 判定のスクリプトが `VALID` と判定するクロスチェックの記録（`.claude/commands/_lib/cross-check-review.md` の「The validity decision」）。本書の F-002 の照合を含む。
-   **引き継ぎ:** Antigravity のセッションが重い側の部分レビューに達して止まり、利用者が Claude Code で `/crosscheck` を走らせ、Antigravity で同じコマンドを再実行して続ける一連の流れ（F-003）。
-   **再実行したコマンド:** 引き継ぎの後に Antigravity で再実行した `/mkreq` か `/mkarch`。用語集の「再開した実行」（CLI の `--after`）とは別である。
-   **単独実行のコマンド:** `/crosscheck`。`.claude/commands/_lib/cross-check-review.md` の「The standalone path」（チェックポイントの入口と push の直前の入口が使う単独の経路）とは別である。`/crosscheck` は、その経路を再利用しうる。

## 変更履歴

- 2026-11-27 承認（isseis、#1190）
