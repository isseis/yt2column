# 要件定義書：開発の道具の規則を ADR-0014 から規約 dev_tools.md に移す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `simple` |
| Created | 2026-11-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、開発の道具を足すときや変えるときにエージェントが従う規則を、英語の規約 `docs/dev/conventions/dev_tools.md` に移す要件を定める。

**背景:**

-   開発の道具の規則は、今は [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)（決定的な判定はエージェントに任せず、テストのある Go の開発ツールに置く）の「決定」に、規則の本文のまま書かれている（`docs/dev/adr/0014-deterministic-checks-in-go.md:17-25`）。`CLAUDE.md` にも `docs/dev/conventions/` にも、この規則はない。
-   ADR の規約は、正本の文書がある主題では、ADR は決定を要約して正本へリンクすると定めている（`docs/dev/conventions/adr.md`「Writing an ADR」の「決定」）。エージェントが従う規則は英語で書く（`CLAUDE.md`「Document language」の原則 (b)）。今の形では、道具を足すエージェントが従う規約がなく、日本語の ADR を指示として読むことになる。
-   ADR-0014 を規則の置き場所として指している文書は、次の 3 つである。
    -   `docs/dev/guides/chief_guide.md:22`（「ADR-0014 の『道具は `main` からインストールして使う』」）
    -   `docs/dev/guides/numreserve_guide.md:11`（同じ）
    -   `.claude/commands/_lib/adr-correction-candidates.sh:62-64` のコメント（Go に置く道具の規則の置き場所として ADR-0014 を名指す）

    ほかの ADR（ADR-0037・ADR-0046・ADR-0051・ADR-0053）は、ADR-0014 の決定とその理由を指している。規則の置き場所として指しているわけではない。`docs/dev/conventions/package_reference.md` は ADR-0014 を指していない。
-   「エージェントは道具をインストールもビルドもしない」と明記している道具は、今は 3 つだけである。`CLAUDE.md`「Tool Execution Safety」が `codexreview`（`CLAUDE.md:1006-1007`）・`numreserve`（`CLAUDE.md:1015-1016`）・`chief`（`CLAUDE.md:1032`）について書いている。ADR-0014 の「道具は `main` からインストールして使う」（`docs/dev/adr/0014-deterministic-checks-in-go.md:22`）は、エージェントについては何も書いていない。`fixprguard` と `mergepr` については、コマンドの定義が利用者に入れ直しを案内している（`.claude/commands/fixpr.md:42`、`.claude/commands/mergepr.md:19`）が、エージェントが入れないことは規則になっていない。
-   `coveragecompare` は、インストールせずに使う。`make coverage-compare` と `make ext-coverage-compare` が、そのつど一時ディレクトリにビルドして走らせる（`docs/dev/adr/0014-deterministic-checks-in-go.md:22`）。この 2 つは、エージェントがテストの削除を確かめるときに走らせる（`CLAUDE.md`「Testing Strategy」）。

以上の行番号は、コミット `a73b2c4e`（`origin/main`）で確かめた。

対応 issue: [#1014](https://github.com/isseis/yt2column/issues/1014)

**本書の記述範囲:** 本書は、規約が持つ規則、ADR-0014 の書き直しの範囲、`CLAUDE.md` に足すもの、参照の付け替え、変えないものを定める。規約の節の構成と ADR-0014 の各項目の書き方は `02_architecture.md` で、確かめ方は `03_implementation_plan.md` で定める。

**issue #1014 で決めたこと:**

-   **規約に移す規則は、次の 6 つとする。規則の内容は変えない（ただし、下の「要件定義の際に決めたこと」が規則 5 を広げる）。** 規則の番号は、本書でもこの番号を使う。
    1.  判定と副作用を持つ開発の道具は Go で書き、テストを持つ。形は `cmd/<名前>` と `internal/<名前>` の組とし、製品のバイナリには入れない。
    2.  道具は、判定に使う基準値を状態のファイルから読み返さない。基準値はサブコマンドの出力として返し、セッションが次の呼び出しの引数で渡す。chief の例外は範囲を ADR-0014 と ADR-0046 に残し、規約は例外があることだけをリンクで示す。
    3.  処理の順序は、コマンドの定義ではなく道具のコードで強制する。コマンドの定義には、サブコマンドを呼ぶ順序と利用者への確認だけを書く。
    4.  道具の引数・出力・終了コードは、コマンドの定義と文書のガードが読む契約である。変えるときは、道具と、それを呼ぶ文書と、ガードを同じ PR で変える。
    5.  道具は `main` からインストールして使い、道具を変えたマージの後は入れ直す。エージェントは道具をインストールもビルドもしない（`CLAUDE.md`「Tool Execution Safety」と重なる部分は、そちらへリンクする）。
    6.  例外として、記録の行を読んで判定するだけで副作用を持たない判定は、シェルスクリプトにしてよい。条件は、`.claude/commands/_lib/` に置くこと、`origin/main` の commit から作業ツリーの外に取り出して走らせること、失敗は安全な側の判定とすること、規則の全体を常設の Go のテストで確かめ、規則を壊して失敗することを確かめることの 4 つである。

    理由：エージェントが従う規則の正本を、英語の規約に置くためである（背景）。
    -   ADR: ADR-0014 の「決定」を、要約と規約へのリンクに書き直す。決定は変えないので、編集上の訂正（`docs/dev/conventions/adr.md`「Editorial corrections」の 1 つ目の種類）として記録する。同じ規約は「既存の決定を正本の文書に書き移すことは決定を変えない」とも定めている。新しい ADR は作らない。
-   **ADR-0014 には、理由・採らなかった案・経緯を残す。** ADR が持つものは理由・採らなかった案・経緯であり、仕様は正本の文書が持つためである（`docs/dev/conventions/adr.md`「Writing an ADR」）。
    -   ADR: 上と同じ（ADR-0014 の編集上の訂正）。
-   **`CLAUDE.md` の「Quick Links」の「Development Guides」に、規約へのリンクを足す。** 規約へたどれるようにするためである。
    -   ADR: なし。リンクの追加で、決定を変えない。
-   **ADR-0014 を規則の置き場所として指している文書を、規約を指すように直す。** 規則の置き場所を 1 つにするためである。
    -   ADR: なし。参照の付け替えで、決定を変えない。
-   **プロトコル番号の規則は対象外とする。** 正本が `CLAUDE.md` の「Protocol numbers」にすでにあるためである。
    -   ADR: なし。
-   **ADR-0024（環境変数を読む場所）と ADR-0019（`context.Context` を第 1 引数にする規則など）の規則を規約に移すかは、別の issue で扱う。**
    -   ADR: なし。

**要件定義の際に決めたこと（2026-11-19）:**

-   **進め方は簡易な進め方（`simple`）とする。この変更は除外される変更に当たると判断したが、人がその判断を上書きした。** 2026-11-19 に人が選んだ。
    -   簡易を推奨した理由：変更は文書だけで（規約 1 つ、ADR 1 つ、`CLAUDE.md`、参照 3 か所）、新しい仕組みを作らず、未決の判断も少ない。
    -   除外の判断：最初は除外に当たらないと判断した。`docs/dev/security.md` の規則と前提も外部依存も変えず、道具のコードも変えないためである。そのあと、規則 5 を `fixprguard` と `mergepr` に広げることに決まった（次の項目）。この 2 つは開発の道具の信頼境界の道具である（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）。エージェントが道具を入れないという規則は、その境界に影響する。境界を強める向きの変更ではあるが、迷う場合に当たるので、除外に当たると判断し直した（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。
    -   上書き：人は通常の進め方への切り替えを選ばず、判断を上書きして簡易な進め方のまま進めることを選んだ。道具のコードを変えず、コマンドの定義がすでに利用者に入れ直しを案内している運用を、規則として書くだけだからである。
    -   ADR: なし。本タスクの中で閉じる、進め方の選択である。
-   **規則 5 の「エージェントは道具をインストールもビルドもしない」は、利用者が `main` から入れる道具のすべてに当てはめる。今は `mergepr`・`fixprguard`・`codexreview`・`chief`・`numreserve` の 5 つである。** 2026-11-19 に人が選んだ。理由は 2 つある。1 つ目は、エージェントが PR のブランチから `fixprguard` や `mergepr` を入れ直せると、そのブランチが変えた判定でガードが働き、ガードを迂回できることである。2 つ目は、コマンドの定義がすでに利用者に入れ直しを案内しており（背景）、運用と規則を揃えられることである。make のターゲットがそのつどビルドして走らせる道具（今は `coveragecompare`）は、インストールしないので、この規則の対象ではない。
    -   採らなかった案：`CLAUDE.md` がすでに明記している 3 つ（`codexreview`・`numreserve`・`chief`）だけに当てはめる案。規則の内容は変わらないが、`fixprguard` と `mergepr` について規則がないままになる。
    -   ADR: ADR-0014 を更新する。決定の変更なので、「決定」の「道具は `main` からインストールして使う」の項目にこの規則を足し、「経緯」に項目を足す（本 PR の [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)）。
-   **`CLAUDE.md` の「Tool Execution Safety」に、利用者が `main` から入れる道具のすべてについて「エージェントはインストールもビルドもしない」という 1 項目を足し、詳細は規約へリンクする。今の 3 つの道具の記述は残す。** 2026-11-19 に人が選んだ。この節は毎回読み込まれるので、`fixprguard` と `mergepr` を扱うときにも、エージェントの目に確実に入るためである。
    -   採らなかった案：規約だけを正本にし、`CLAUDE.md` は「Quick Links」だけを変える案。規約を読まないエージェントには届かない。
    -   ADR: ADR-0014 の更新（上の項目）に含める。新しい ADR は作らない。
-   **ADR-0014 の「決定」のうち、移す 6 つの規則以外の項目は変えない。対象は、chief の例外（`docs/dev/adr/0014-deterministic-checks-in-go.md:19`）、拡張の関数の範囲を TypeScript で求める規則（`:21`）、プロトコル番号（`:23`）の 3 つである。** 2026-11-19 に人が選んだ。chief の例外は、規則 2 のとおり ADR に範囲を残す。TypeScript の補助は `coveragecompare` だけの決まりである。プロトコル番号は issue の対象外である。
    -   採らなかった案：プロトコル番号の項目も、要約と `CLAUDE.md` へのリンクに縮める案。issue の対象外の項目に手を広げることになる。
    -   ADR: なし。ADR-0014 のこの 3 項目は変えない。
-   **規約には規則だけを書き、理由は書かずに ADR-0014 へリンクする。** 2026-11-19 に人が選んだ。理由の正本を ADR-0014 の 1 か所に保ち、同じ内容を 2 か所で手入れしないためである。
    -   採らなかった案：規則ごとに 1 文の理由を添える案。エージェントが境界の場合を判断しやすくなるが、ADR と同じ内容を 2 か所に持つことになる。
    -   ADR: なし。規約の書き方の選択で、本タスクの中で閉じる。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   道具を足すときや変えるときにエージェントが従う規則を、英語の規約 1 か所から読めるようにする。
-   ADR-0014 を、ADR の規約が定める形（要約と正本へのリンク）にする。
-   エージェントが、利用者が `main` から入れる道具をどれもインストールもビルドもしないことを規則にする。

### 2.2. スコープ (In Scope)

-   `docs/dev/conventions/dev_tools.md`（英語）を新しく作り、6 つの規則を書く（F-001）。
-   ADR-0014 の「決定」のうち、6 つの規則の項目を書き直す（F-002）。
-   ADR-0014 の「決定」と「経緯」に、規則 5 を広げる決定を入れる（F-002。本 PR で行う）。
-   `CLAUDE.md` の「Quick Links」と「Tool Execution Safety」に 1 項目ずつ足す（F-003）。
-   ADR-0014 を規則の置き場所として指している 3 か所を、規約を指すように直す（F-004）。

### 2.3. スコープ外 (Out of Scope)

-   規則 1〜4 と 6 の内容の変更。規則 5 は、上で決めた範囲だけを広げる。
-   ADR-0014 の「決定」のうち、chief の例外・TypeScript の補助・プロトコル番号の 3 項目と、「背景」「採らなかった案」「結果」の変更。理由は 1 章の「要件定義の際に決めたこと」にある。
-   ADR-0014 の決定とその理由を指しているほかの ADR（ADR-0037・ADR-0046・ADR-0051・ADR-0053）の変更。規則の置き場所として指しているわけではないためである（1 章の背景）。
-   `CLAUDE.md`「Tool Execution Safety」にある、`codexreview`・`numreserve`・`chief` についての今の記述の変更。
-   ADR-0024 と ADR-0019 の規則を規約に移すこと（issue #1014 の対象外）。
-   プロトコル番号の規則（正本は `CLAUDE.md`「Protocol numbers」）。
-   道具のコード、コマンドの定義、Makefile の変更。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 規約 dev_tools.md

`docs/dev/conventions/dev_tools.md` を新しく作る。開発の道具を足すときや変えるときにエージェントが従う規則の正本とする。

**Acceptance Criteria**:
- **AC-01**: `docs/dev/conventions/dev_tools.md` があり、英語で書かれている。規則 1〜6（1 章の「issue #1014 で決めたこと」）が、それぞれ見出しのある節として書かれている。
- **AC-02**: 規約は、対象の道具を名前の一覧で示さず、種類で示す。種類は「`make install-<名前>` で `main` から入れる道具」と「make のターゲットがそのつどビルドして走らせる道具」で、道具の一覧は `CLAUDE.md` の「Build Commands」へのリンクで示す。
- **AC-03**: 規則 2 の節は、chief の例外の範囲を書かない。例外があることだけを書き、ADR-0014 と ADR-0046 へリンクする。
- **AC-04**: 規則 5 の節は、次の 3 つを書く。利用者が `make install-<名前>` で `main` から入れ、道具を変えたマージの後に入れ直すこと。エージェントはそれらの道具をインストールもビルドもしないこと。make のターゲットがそのつどビルドする道具はこの規則の対象でないこと。`CLAUDE.md`「Tool Execution Safety」と重なる部分は、そちらへリンクする。
- **AC-05**: 規約は、規則の理由を書かない。理由は ADR-0014 にあることを、リンクで示す。
- **AC-06**: 規約は、プロトコル番号の規則を書かない。`CLAUDE.md` の「Protocol numbers」へのリンクで示す。

#### F-002: ADR-0014 の書き直し

**Acceptance Criteria**:
- **AC-07**（本 PR）: ADR-0014 の「決定」の「道具は `main` からインストールして使う」の項目に、エージェントは利用者が `main` から入れる道具をどれもインストールもビルドもしないことが、理由とともに書かれている。「経緯」の最後に、この決定の項目（2026-11-19、本書へのリンク）がある。
- **AC-08**: ADR-0014 の「決定」のうち、規則 1〜6 の 6 つの項目は、規則の本文（道具の名前の一覧、`make` のターゲットの一覧、手順の詳細）を持たない。各項目は、規則の要約と短い理由を持ち、規約の該当する節の見出しの名前を本文で示してリンクする。
- **AC-09**: 実装の PR での ADR-0014 の差分は、規則 1〜6 の 6 つの項目の書き換えに限る。chief の例外・TypeScript の補助・プロトコル番号の 3 項目と、「背景」「採らなかった案」「結果」「経緯」は変わらない。
- **AC-10**: 実装の PR は、AC-09 の書き換えを編集上の訂正として記録する。コミットメッセージには訂正行を書き、PR の説明には訂正の段落を書く（`docs/dev/conventions/requirements_process.md`「Recording in the PR description」）。
- **AC-11**: `TestADRRecords` を含む `make test` が通る。

#### F-003: CLAUDE.md

**Acceptance Criteria**:
- **AC-12**: `CLAUDE.md` の「Quick Links」の「Development Guides」に、`docs/dev/conventions/dev_tools.md` へのリンクと、その要旨がある。
- **AC-13**: `CLAUDE.md` の「Tool Execution Safety」に、次の 1 項目がある。エージェントは、利用者が `make install-<名前>` で `main` から入れる道具を、インストールもビルドもしない。項目は規約へリンクする。
- **AC-14**: 実装の PR での `CLAUDE.md` の差分は、AC-12 と AC-13 の 2 項目の追加に限る。

#### F-004: 参照の付け替え

**Acceptance Criteria**:
- **AC-15**: `docs/dev/guides/chief_guide.md` と `docs/dev/guides/numreserve_guide.md` の「道具は `main` から入れる」の規則の参照が、ADR-0014 ではなく規約の規則 5 の節を指す。
- **AC-16**: `.claude/commands/_lib/adr-correction-candidates.sh` のコメントで、Go に置く道具の規則の置き場所として ADR-0014 を名指している箇所が、規約を名指す。スクリプトの動作は変わらない（`TestADRCorrectionCandidates` が通る）。
- **AC-17**: 実装の PR で変わるファイルは、次に限る。
    -   `docs/dev/conventions/dev_tools.md`（新規）
    -   `docs/dev/adr/0014-deterministic-checks-in-go.md`
    -   `CLAUDE.md`
    -   `docs/dev/guides/chief_guide.md`
    -   `docs/dev/guides/numreserve_guide.md`
    -   `.claude/commands/_lib/adr-correction-candidates.sh`
    -   本タスクのディレクトリ（`docs/tasks/0112_dev_tools_convention/`）
- **AC-18**: 実装の PR での `chief_guide.md` と `numreserve_guide.md` の差分は、それぞれ AC-15 の参照の 1 か所の書き換えに限る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A。文書だけの変更である。

### 4.2. セキュリティ (Security)
-   N/A。`docs/dev/security.md` の条件（外部コマンドの起動、API キー、キャッシュ、ネットワーク、信頼できない文字列の埋め込み）のどれにも当たらない。規則 5 を広げることは開発の道具の信頼境界（ADR-0015）に関わるが、エージェントができることを減らす向きの変更で、道具のコードと `docs/dev/security.md` は変えない。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A。道具とコマンドの振る舞いを変えない。

### 4.4. 互換性 (Compatibility)
-   どの道具のプロトコル番号も変えない。道具を入れ直す必要はない。

### 4.5. 保守性 (Maintainability)
-   規約は道具の名前の一覧を持たない（AC-02）。道具を足しても、規約を直す必要はない。
-   規約は理由を持たない（AC-05）。理由の正本は ADR-0014 の 1 か所である。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   規約は英語で、ADR とタスク文書は日本語で書く（`CLAUDE.md`「Document language」）。
-   ADR-0014 の書き直しは、ADR の規約（`docs/dev/conventions/adr.md`「Writing an ADR」「Editorial corrections」）に従う。ADR から規約の節を指すときは、アンカーを使わず、見出しの名前を本文で示す（同じ規約の「Links」）。
-   文書のテストは文の意味を確かめない（`CLAUDE.md`「Testing Strategy」）。規約の文は PR のレビューで読む。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **開発の道具:** `cmd/<名前>` と `internal/<名前>` の組で作る、開発の手順のための Go の道具。製品のバイナリには入らない（規則 1）。
-   **規約:** 本書では `docs/dev/conventions/dev_tools.md` を指す。
-   **規則 1〜6:** 1 章の「issue #1014 で決めたこと」に挙げた 6 つの規則。
