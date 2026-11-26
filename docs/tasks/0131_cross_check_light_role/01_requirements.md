# 要件定義書：クロスチェックの部分レビューを重い側と軽い側の役割に分け、強めたプロンプトと単独実行のコマンドを足す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-26 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、タスクの PR を開く前のクロスチェックの部分レビュー（[ADR-0050](../../dev/adr/0050-fable-partial-review.md)）について、次の 4 点の要件を定める。

-   PR の種類によって、重い側と軽い側の 2 つのクロスチェック役割を使い分ける。
-   両方の役割の共通のプロンプトを強める。
-   Antigravity では、重い側を Claude Code に引き継ぐ。
-   部分レビューを単独で行うコマンド `/crosscheck` を足す。

対応 issue: [#1057](https://github.com/isseis/yt2column/issues/1057)（関連: [#1104](https://github.com/isseis/yt2column/issues/1104)・[#1105](https://github.com/isseis/yt2column/issues/1105)・[#1106](https://github.com/isseis/yt2column/issues/1106)・[#1082](https://github.com/isseis/yt2column/issues/1082)・[#1008](https://github.com/isseis/yt2column/issues/1008)）

**背景:**

クロスチェックの部分レビューは、タスクの PR を開くすべてのコマンド（`/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2`・`/runplan`）で、同じ 1 つの役割 `cross-check` のモデルを使っている（`.claude/commands/_context.md:213,217`）。担当は、Claude Code では `fable`、Antigravity では `claude-opus-5-5-medium` である。呼び出し元が渡す入力 CROSS_CHECK には、PR の種類として KIND（`documents` か `implementation`）しかない（`.claude/commands/_lib/cross-check-review.md:44-50`）。そのため、要件定義書の PR と計画書だけの PR を区別できない。

issue #1057 は、#985〜#1048 で Fable が走った 36 回を集計した。1 回あたりの Major の数は、要件だけの PR で 2.4、3 文書の PR で 2.2、設計の PR で 1.6、計画書だけの PR で 0.4、実装の PR で 0.62 だった。1 回の費用は、段階によらず約 11 分・約 26 万トークンだった。また、Fable が走った 13 回（実装の PR 8 回、計画書だけの PR 5 回）を、プロンプトを強めた Opus で再現した。強めた Opus は Fable の Major 10 件のうち 2 件を Major として、3 件を Minor として見つけ、費用はおよそ Fable の 35% だった。

今の仕組みには、次の性質がある。

-   **`not run` の記録も記録として有効になる:** 部分レビューが走らなかったときの `not run` の記録も「クロスチェックの記録」であり、その後に中身のある commit がなければ有効（`VALID`）と判定される（`.claude/commands/_lib/cross-check-review.md:132-174`、`.claude/commands/_lib/review-records.sh:273`）。
-   **判定のスクリプトは役割の一覧を固定で持つ:** 判定のスクリプト `review-records.sh` と `high-review-decision.sh` は、どちらも `Role` 列の値を `high`・`standard`・`low`・`cross-check` の 4 つに限り、ほかの値の表を拒否する（`.claude/commands/_lib/review-records.sh:379-380`、`.claude/commands/_lib/high-review-decision.sh:206`）。両スクリプトの共通のテスト入力は `cmd/yt2column/harness_mapping_cases_test.go` にある。
-   **Antigravity からは Fable を呼べない:** Antigravity のモデル固定ワーカーは、`swarm` で選べるモデルに限られる（`.claude/commands/_lib/harness-antigravity.md` の「8. pinned-worker」）。Antigravity から Claude Code の CLI を呼ぶ案は [#1082](https://github.com/isseis/yt2column/issues/1082) に分けてある（ADR-0050「採らなかった案」）。
-   **再実行は止まる:** ブランチに要件定義書があると、`/mkreq` は再実行しても step 1 で止まる（`.claude/commands/mkreq.md:84-95`。例外は簡易プロセスの再開だけ）。`02_architecture.md` があると、`/mkarch` も止まる（`.claude/commands/mkarch.md:51`）。

**本書の記述範囲:** 本書は、役割とモデルの対応、PR の種類ごとの役割、記録が有効かの判定、Antigravity での引き継ぎと再開、`/crosscheck` の振る舞い、共通のプロンプトに足す義務と制約を定める。次の事項は `02_architecture.md` で決める。

-   役割の名前と `_context.md` の「Harness model mapping」の表の形（effort の表し方、Antigravity の重い側の行の形）。
-   判定のスクリプトが PR の種類を知る方法。
-   再開の入口と `/crosscheck` の手順の組み立て。

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
-   **保留にする案:** 実装のレビューをタスクの最後に 1 回だけ行う案は、本タスクでは扱わない。追加の節約が小さく、見つかるのが遅れ、レビューの関門の作り直しが要るためである。

**要件定義の際に決めたこと（2026-11-26）:**

以下をレビュアーと確認して決めた。issue の「改訂方針（2026-11-26）」の Antigravity の割り当て（重い側・軽い側とも `claude-opus-5-5-high`）は、2 つ目の項目で改めた。

-   **進め方の選択（完全プロセス）:**
    -   **除外判定:** 簡易プロセスの除外には当たらないと判断した。部分レビューのモデルは変わるが、レビューのスレッドのテキストの扱い（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）・`docs/dev/security.md` の規則・外部のサービス（プロバイダは Anthropic のまま）は変えないためである（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。
    -   **決定と理由:** 推奨どおり完全プロセス（`full`）を採る。変更が `_context.md`・2 つの判定のスクリプト・共有の手順・5 つのコマンド・2 つのアダプタ・新しいコマンドとその入口・Go のテストに及ぶためである。ADR-0050 の決定も変え、新しい仕組み（役割とコマンド）も作る。
    -   **ADR の判断:** ADR なし（本タスクの進め方の選択であり、タスク内で閉じるため）。
-   **Antigravity の重い側は `fable` にし、利用者が Claude Code で走らせる（F-001・F-003）:**
    -   **決定と理由:** Antigravity の重い側の役割も `fable` にする。Antigravity から `fable` は呼べないので、Antigravity のセッションは重い側の部分レビューに達したら止まり、利用者が Claude Code で `/crosscheck` を走らせる。その後、利用者は Antigravity に戻って残りの作業を再開する。重い側は 1 回あたりの Major が多い PR を見るので、ハーネスによらず同じモデルにそろえる。引き継ぎを将来は `chief` が制御する。
    -   **ADR の判断:** ADR-0050 を更新する（決定 4 のハーネスごとのモデル）。
-   **Antigravity は PR を開く前に止まり、同じコマンドの再実行で再開する（F-003）:**
    -   **決定と理由:** Antigravity は、重い側の部分レビューに達したら push の前に止まる。`/crosscheck` の後に同じコマンドを再実行すると、PR を開く手前から再開する。PR を開いてから見る案は採らない。人のレビューを直した後の PR から始めるためで、ADR-0050 の決定 1 の理由と同じである。
    -   **ADR の判断:** ADR-0050 を更新する（決定 4 の走らないときの扱い）。
-   **Antigravity の軽い側は `claude-opus-5-5-high` にし、設計で使えるかを確かめる（F-001）:**
    -   **決定と理由:** Antigravity の軽い側は、Claude Code の軽い側（`opus`、effort `high`）と effort をそろえて `claude-opus-5-5-high` にする。`swarm` で使えるかはまだ確かめていない（タスク 0120 で実測したのは `claude-opus-5-5-medium` だけ）。そのため、`/mkarch` の段階で確かめる（[design_handoff.md](design_handoff.md) の H-01）。使えなければ、要件の決定の変更として本書に戻す。
    -   **ADR の判断:** ADR-0050 を更新する（決定 4 のハーネスごとのモデル）。
-   **`/crosscheck` は、最後の記録が有効な `run` でないときに走らせる（F-004）:**
    -   **決定と理由:** `/crosscheck` は、ブランチの最後のクロスチェックの記録が `not run`（理由を問わない）のとき、記録がないとき、または記録が有効でないときに部分レビューを行う。最後の記録が有効な `run` の記録なら、何もせずに報告する。どのときも commit は push せず、push は利用者が行う。記録を問わずに PR の全体をいつも見直す案は採らない。有効な `run` の記録があっても費用が 1 回分かかるためである。
    -   **ADR の判断:** ADR-0050 を更新する（単独実行のコマンドを足す）。
-   **記録が有効かの判定で、記録の役割と PR の種類を照らし合わせる（F-002）:**
    -   **決定と理由:** 重い側の役割を使う PR（重い側の PR）では、重い側の役割のモデルで走った `run` の記録だけを有効とする。軽い側の役割を使う PR（軽い側の PR）では、どちらの役割の `run` の記録も有効とする。重い側の記録は、上位のモデルが同じ範囲を見たものなので、軽い側の PR で見直す必要がない。切り替えの前から作業中のブランチにある Fable の記録も、軽い側の PR ではそのまま有効になる。軽い側の PR でも重い側と完全に一致させる案は採らない。作業中のブランチの Fable の記録が無効になり、もう一度走るためである。
    -   **ADR の判断:** ADR-0050 を更新する（決定 5 の有効かの判定）。
-   **再開したとき、`not run` の記録ならそのまま進む（F-003）:**
    -   **決定と理由:** `/crosscheck` でも重い側が走らず `not run` と記録されたときは、Antigravity で再開したコマンドはその記録で PR を開く。部分レビューは追加のレビューで PR を止めないという、ADR-0050 の決定 4 の方針どおりである。`run` の記録ができるまで止める案は採らない。Fable の枠が戻るまでタスクが止まるためである。この決定から、`not run` の記録は、PR の種類によらず今と同じく有効と判定される（AC-10）。`not run` の記録にはモデルがないので、役割を照らし合わせられないためでもある。
    -   **ADR の判断:** ADR-0050 を更新する（決定 4 の走らないときの扱い）。
-   **単独実行のコマンドの名前は `/crosscheck`（F-004）:**
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
-   記録が有効かの判定での、記録の役割と PR の種類の照合（F-002）。
-   Antigravity で重い側に達したときの停止と、同じコマンドの再実行による再開（F-003）。
-   単独実行のコマンド `/crosscheck` と、その Antigravity の入口（F-004）。
-   共通のプロンプトに足す 6 つの義務と、範囲の終点より後の情報を読まない制約（F-005）。
-   ADR-0050・`CLAUDE.md`・用語集など、関係する文書の更新（F-006）。

### 2.3. スコープ外 (Out of Scope)

-   **引き継ぎの `chief` による制御:** Antigravity と Claude Code の間の引き継ぎを `chief` が制御することは、将来の別タスクとする（要件定義の際に決めたこと）。`_context.md` の「Command routing」の表には `/crosscheck` の行を足さない。
-   **Antigravity から Claude Code の CLI を呼ぶこと:** [#1082](https://github.com/isseis/yt2column/issues/1082) で扱う。
-   **実装のレビューをタスク単位で 1 回にする案:** 保留とする（issue #1057 で決めたこと）。
-   **異系統の LLM のローカルレビュー:** [#1106](https://github.com/isseis/yt2column/issues/1106) で扱う。
-   **記録の分類と集計:** [#1105](https://github.com/isseis/yt2column/issues/1105) と [#936](https://github.com/isseis/yt2column/issues/936) で扱う。
-   **merge の後の確認と上位モデルのレビュー:** モデル（`high` 役割）・手順・記録を変えない。
-   **記録の行の形:** 「Record lines」の各行の形（`.claude/commands/_lib/cross-check-review.md:719-730`）と `not run` の理由の値は変えない。役割は記録の `model <model-id>` から分かるためである。
-   **OpenCode:** OpenCode で動く `/runplan` の部分レビューは、今と同じく `not run agent-cannot-call` とする（`.claude/commands/_lib/cross-check-review.md:193-198`）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 重い側と軽い側の役割とモデルの割り当て

クロスチェックの部分レビューの役割を、重い側と軽い側の 2 つに分ける。PR を開くコマンドごとに、どちらの役割を使うかを決める。ハーネスごとのモデルは `_context.md` の「Harness model mapping」に置く。

**Acceptance Criteria**:
- **AC-01**: 次のコマンドがローカルの CLI レビューと並列に起動する部分レビュー（`_lib/task-pr-handoff.md` Part B step 1、`runplan.md` step 7.7）は、重い側の役割で走る。チェックポイントの入口と push の直前の入口が起動する部分レビューも同じである。
    -   `/mkreq`（完全プロセスと簡易プロセスの両方）
    -   `/mkarch`
- **AC-02**: 次のコマンドが起動する部分レビューは、軽い側の役割で走る。起動する箇所は AC-01 と同じである。
    -   `/mkplan`
    -   `/mkplan2`
    -   `/runplan`
- **AC-03**: Claude Code では、重い側の部分レビューは `fable` のモデルで走り、記録の `model` は `claude-fable-` で始まる ID になる。軽い側の部分レビューは `opus` のモデルを effort `high` で起動し、記録の `model` は `claude-opus-` で始まる ID になる。
- **AC-04**: Antigravity では、軽い側の部分レビューは `claude-opus-5-5-high` のモデルで走り、記録の `model` は `claude-opus-5-5-high` になる。重い側は Antigravity では起動せず、F-003 のとおり止まる。
- **AC-05**: 重い側と軽い側の割り当ては、ハーネスごとの表 1 か所（`_context.md` の「Harness model mapping」）にある。`.claude/commands/_lib/cross-check-review.md` と、それが組み立てるプロンプトは、モデルの版を含む具体的なモデルの ID を書かない（今の規則、`.claude/commands/_lib/cross-check-review.md:31-33`）。
- **AC-06**: 2 つの判定のスクリプト（`review-records.sh`・`high-review-decision.sh`）は、新しい役割を含む `origin/main` の表を受理する。表の検証と照合の規則は、今の規則（フェイルクローズの表の検証、正規表現でない照合）を保つ。表に重い側か軽い側のどちらかの検証パターンがないときは、`review-records.sh` はフェイルクローズで失敗する。
- **AC-07**: merge の後の確認は、今と同じく `high` 役割のモデルで走る。OpenCode で動く `/runplan` の部分レビューは、今と同じく `not run agent-cannot-call` と記録される。

#### F-002: 記録が有効かの判定での役割の照合

記録が有効かを判定するとき、記録の `run` の行のモデルが、PR の種類の求める役割に合うかを照らし合わせる。

**Acceptance Criteria**:
- **AC-08**: 重い側の PR で、最後のクロスチェックの記録が `run` の記録であり、その `model` がどの重い側の検証パターンにも合わないとき、判定は `MISSING` になる。合わない例は、軽い側のパターンにだけ合う `model` である。重い側のパターンに合う `run` の記録は、今の規則のとおり有効になりうる。
- **AC-09**: 軽い側の PR では、`model` が重い側か軽い側のどちらかの検証パターンに合う `run` の記録が、今の規則のとおり有効になりうる。本タスクを `main` に入れる前に、作業中のブランチに書かれた `model claude-fable-…` の記録も含む。
- **AC-10**: `not run` の記録は、PR の種類によらず、今の規則のとおり有効になりうる。
- **AC-11**: 判定のスクリプトが PR の種類を決められないとき（入力の誤り、読めない入力など）は、判定は `MISSING` か、今の規則のとおりの失敗（呼び出し元が `MISSING <merge-base>` と読むもの、`.claude/commands/_lib/cross-check-review.md:125-130`）になる。誤って `VALID` にはならない。

#### F-003: Antigravity での重い側の停止と再開

Antigravity のセッションは、重い側の部分レビューに達したら、push の前に止まる。利用者が Claude Code で `/crosscheck` を走らせた後に同じコマンドを Antigravity で再実行すると、PR を開く手前から再開する。

**Acceptance Criteria**:
- **AC-12**: Antigravity の `/mkreq` か `/mkarch` で、重い側の部分レビューを起動する箇所に達し、判定が `MISSING` のとき、次のとおり止まる。AC-01 のどの起動箇所でも同じである。
    -   部分レビューのワーカーを起動しない。
    -   クロスチェックの記録（`run`・`not run`）を書かない。
    -   push せず、PR を開かない。
    -   ブランチの commit を消さない。
- **AC-13**: AC-12 で止まったときの報告は、次の 4 つを含む。
    -   重い側の部分レビューを Antigravity で走らせられないこと。
    -   この worktree で Claude Code を開いて `/crosscheck` を走らせること。
    -   その後に Antigravity で同じコマンド（`/mkreq <N>`、`/mkarch <task>`）を再実行すること。
    -   止まったブランチの名前。
- **AC-14**: AC-12 で止まったブランチで `/crosscheck` が記録を作った後、Antigravity で同じコマンドを再実行すると、PR を開く手前から再開する。再開した実行は、次のことをしない。
    -   新しいタスク番号を決めない。
    -   新しいブランチを作らない。
    -   決定を人にもう一度問わない。
    -   書いた文書を書き直さない。

    再開した実行は、記録が有効なら部分レビューを起動しない。そのうえで、`_lib/task-pr-handoff.md` Part B の残り（CHECKS・push の直前の判定・push・PR の作成・レビュアーへの知らせ）を行う。
- **AC-15**: `/crosscheck` を走らせずに再実行したとき（判定が `MISSING` のまま）は、AC-12・AC-13 と同じく止まる。
- **AC-16**: `/crosscheck` の記録が `not run` のとき、再開した実行は止まらずに PR を開く。PR の説明の「クロスチェックの部分レビュー」の節には、その `not run` の記録が理由とともに出る。
- **AC-17**: 再開できるのは、AC-12 で止まった実行が残したブランチで、まだ PR のないものだけである。それ以外の状態では、今の規則のとおり止まる。たとえば、ブランチに PR がある、作業ツリーに変更がある、要件定義書がほかのブランチにもある、といった状態である（`.claude/commands/mkreq.md:84-95`、`.claude/commands/mkarch.md:51`）。
- **AC-18**: Claude Code で動く `/mkreq`・`/mkarch` は、F-003 の停止をしない。今と同じく、重い側の部分レビューを自分で起動する。

#### F-004: 単独実行のコマンド `/crosscheck`

今のブランチの上で部分レビューを単独で行い、トリアージ・修正・修正のローカルの CLI レビュー・記録の commit まで行う。push はしない。

**Acceptance Criteria**:
- **AC-19**: `.claude/commands/crosscheck.md` と `.agents/skills/crosscheck/SKILL.md` がある。後者は、ほかのコマンドの入口と同じく薄い入口である。`CLAUDE.md` の「Claude Commands」の一覧に `/crosscheck` がある。
- **AC-20**: `/crosscheck` は、今のブランチの PR の種類から、重い側と軽い側のどちらの役割で走るかを決める。`/mkreq`・`/mkarch` が作るブランチなら重い側、`/mkplan`・`/mkplan2`・`/runplan` が作るブランチなら軽い側である。種類を決められないときは、commit を作らずに止まり、理由を報告する。
- **AC-21**: ブランチの最後のクロスチェックの記録が有効な `run` の記録のとき、`/crosscheck` は部分レビューを行わず、commit を作らず、行う必要がないことを報告する。F-002 の照合に合わない `run` の記録は、有効な `run` の記録に当たらない。最後の記録が有効な `run` の記録で、そのあとに merge の後の確認を待つ merge commit だけがある（判定が `MERGE`）ときも同じである。merge の後の確認は、PR を開くコマンドのチェックポイントの入口と push の直前の入口が、どのハーネスでも行うためである。
- **AC-22**: 次のどれかのとき、`/crosscheck` は F-001 の役割で部分レビューを行う。
    -   最後のクロスチェックの記録が `not run` の記録（理由を問わない）のとき。
    -   クロスチェックの記録がないとき。
    -   判定が `MISSING` のとき。

    レビューの範囲は、`not run` の記録の範囲と、その後の commit のうち有効な `run` の記録が見ていないものを含む。指摘は、今の手順（`.claude/commands/_lib/cross-check-review.md` の「The standalone path」）と同じく次のように扱う。
    -   トリアージし、直す。
    -   直しをローカルの CLI レビューで確かめる。
    -   記録の行を記録の commit に書く。

    記録の行の形は、今の「Record lines」の形である。
- **AC-23**: AC-22 の部分レビューが走らなかったとき（利用上限、時間切れ、モデルの不一致など）、`/crosscheck` は `not run <reason>` の記録の commit を作り、その理由を報告する。今の理由の値を使う。
- **AC-24**: このハーネスがその役割のワーカーを起動できないとき、`/crosscheck` は記録を書かずに、commit を作らずに止まる。報告には、起動できない役割と、起動できるハーネスを含める。たとえば、Antigravity で重い側を求められたときや、OpenCode で動いたときである。
- **AC-25**: `/crosscheck` は、次のことをしない。
    -   push しない。
    -   GitHub に書き込まない（PR・issue・コメント・レビューのスレッド）。
    -   PR を作らない。

    最後の報告には、作った commit の一覧と、次の手順を含める。次の手順は、AC-12 で止まったブランチなら Antigravity で元のコマンドを再実行すること、PR のあるブランチなら利用者が push することである。
- **AC-26**: 作業ツリーに変更があるとき、`/crosscheck` は何も commit も破棄もせずに止まり、変更のあるファイルを報告する。
- **AC-27**: `/crosscheck` のワーカーには、今の部分レビューと同じ状態の比較を行う（`.claude/commands/_lib/cross-check-review.md` の「The state check」）。ワーカーに課す制約（作業ツリーとリポジトリの状態を変えない、ネットワークを使わない、秘密のファイルを読まない）も同じである。状態が変わっていたら、結果を使わず、commit を作らずに止まる。

#### F-005: 共通のプロンプトの強化

クロスチェックの部分レビューの共通のプロンプト（`.claude/commands/_lib/cross-check-review.md` の「The prompt」）に、6 つの義務と 1 つの制約を足す。両方の役割の部分レビューと `/crosscheck` に、同じプロンプトが効く。

**Acceptance Criteria**:
- **AC-28**: 共通のプロンプトは、ワーカーに次の 6 つの義務を課す。
    -   スコープ一覧で除外された箇所が基準に当たらないかを判定し直す。
    -   決定や値を変えた箇所について、古い記述が範囲の全体に残っていないかを探す。
    -   外部の挙動についての主張を、作業ツリーの外での実験で確かめる。
    -   検査やテストが誤って通る入力を作ってみる。
    -   文書にない選択ごとに、人の答えが記録されているかをたどる。
    -   重大度を、起こりうる結果の重さで決める。
- **AC-29**: 共通のプロンプトは、範囲の終点（RANGE の END）より後の情報を読むことを禁じる。`origin/main`・範囲の後の commit・ほかのブランチの内容が、その情報に当たる。ワーカーは、先行文書と ADR を、範囲の終点の内容で読む。
- **AC-30**: 共通のプロンプトの役割・範囲・除外した箇所・決着済みの論点・制約・出力の形の各部分は、AC-28・AC-29 の追加と、AC-05 の役割の区別に要る変更のほかは変わらない。この差分は、`main` との `diff` で確かめる。

#### F-006: 文書の更新

関係する文書を、本書の決定に合わせる。

**Acceptance Criteria**:
- **AC-31**: ADR-0050 の「決定」と「経緯」は、本書の決定を反映する。反映するのは、次の 4 つである。
    -   重い側と軽い側の役割と、ハーネスごとのモデル。
    -   Antigravity での重い側の引き継ぎ。
    -   記録の役割の照合。
    -   `/crosscheck` と共通のプロンプトの強化。

    「採らなかった案」には、本書で採らなかった案を加える。
- **AC-32**: `CLAUDE.md` の部分レビューの説明（`CLAUDE.md` の「Claude Commands」のうち、クロスチェックの部分レビューを述べる段落）は、2 つの役割の使い分けと Antigravity での引き継ぎを述べる。`docs/translation_glossary.md` には、重い側と軽い側の役割の用語がある。
- **AC-33**: 2 つのハーネスのアダプタ（`.claude/commands/_lib/harness-claude.md`・`harness-antigravity.md`）の `pinned-worker` の節は、2 つの役割を扱う。Antigravity のアダプタには、重い側を起動しないことが書かれている。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   軽い側の PR の部分レビューの 1 回の費用（トークン数と時間）は、記録の行の `<seconds>s, tokens <tokens>` で追う。本タスクでは目標の値を設けない。切り替え後の実運用の記録を #1105・#1106 の材料にする（issue #1057 で決めたこと）。

### 4.2. セキュリティ (Security)

-   **`docs/dev/security.md`:** `N/A`。本タスクは `yt-dlp` の呼び出し・API キー・キャッシュ・LLM API への送信・信頼しないテキストの埋め込みを変えない。
-   **開発ツールの信頼の境界:** 部分レビューのワーカーに課す制約と状態の比較（ADR-0050 の決定 6）は、両方の役割と `/crosscheck` に同じく効く（AC-27）。
-   **ワーカーの出力:** 出力はデータとして扱い、その文をコミットメッセージにも PR の説明にも写さない（`.claude/commands/_lib/cross-check-review.md` の「Checking the output」）。この規則は `/crosscheck` にも効く。
-   **`/crosscheck` のネットワーク操作:** `/crosscheck` を起動すると、判定に要る `git fetch origin main` と、ローカルの CLI レビュー（`coderabbit review`、`CLAUDE.md` の「Tool Execution Safety」で常に許可）を承認したことになる。それ以外のネットワーク操作（push、GitHub への書き込み）はしない（AC-25）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   **判定の失敗:** 判定のスクリプトの失敗と読めない出力は、今と同じく安全側（有効でない）に倒す（AC-11）。
-   **重い側を走らせられないとき:** Antigravity の重い側の PR は、利用者が Claude Code で `/crosscheck` を走らせるまで PR を開けない（AC-12・AC-15）。Claude Code でも Fable が走らないときは `not run` と記録して進む（AC-16・AC-23）ので、Fable の枠が尽きてもタスクは止まらない。

### 4.4. 互換性 (Compatibility)

-   **作業中のブランチの記録（軽い側の PR）:** 切り替えの前から作業中の軽い側の PR のブランチにある Fable の記録は、そのまま有効になる（AC-09）。
-   **作業中のブランチの記録（重い側の PR）:** 切り替えの前から作業中の、Antigravity の重い側の PR のブランチにある `claude-opus-5-5-medium` の `run` の記録は、本タスクを `main` に入れた後は無効になる（AC-08）。そのブランチは、F-003 のとおり Claude Code での `/crosscheck` を求められる。この扱いを受け入れる。重い側の PR は重い側のモデルで見るという決定に従うためである。
-   **判定のスクリプトの版:** 判定のスクリプトは `origin/main` の版を実行し、`origin/main` の `_context.md` を読む（`.claude/commands/_lib/cross-check-review.md:94-111`）。古い手順のまま作業中のブランチでも、本タスクを `main` に入れた後は、新しい表とスクリプトで判定される。
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
-   **記録:** クロスチェックの記録。`Cross-check review: run` か `Cross-check review: not run` の行（古い `Fable review:` の行を含む）。`.claude/commands/_lib/cross-check-review.md` の「Reading the record lines」のとおり。
-   **有効な記録:** 判定のスクリプトが `VALID` と判定する記録（`.claude/commands/_lib/cross-check-review.md` の「The validity decision」）。本書の F-002 の照合を含む。
-   **引き継ぎ:** Antigravity のセッションが重い側の部分レビューに達して止まり、利用者が Claude Code で `/crosscheck` を走らせ、Antigravity で同じコマンドを再実行して再開する一連の流れ（F-003）。
