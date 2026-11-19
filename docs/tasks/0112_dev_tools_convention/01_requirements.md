# 要件定義書：開発の道具の規則を ADR-0014 から規約 dev_tools.md に移す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、開発の道具を足すときや変えるときにエージェントが従う規則を、英語の規約 `docs/dev/conventions/dev_tools.md` に移す要件を定める。あわせて、エージェントが道具をインストールもビルドもしない規則を、利用者が `main` から入れる道具のすべてに広げる要件を定める。

**背景:**

-   開発の道具の規則は、今は [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)（決定的な判定はエージェントに任せず、テストのある Go の開発ツールに置く）の「決定」に、規則の本文のまま書かれている（`docs/dev/adr/0014-deterministic-checks-in-go.md:17-25`）。`CLAUDE.md` にも `docs/dev/conventions/` にも、この規則はない。
-   ADR の規約 `docs/dev/conventions/adr.md` は、正本の文書がある主題では、ADR は決定を要約して正本へリンクすると定めている（同「Writing an ADR」の「決定」）。エージェントが従う規則は英語で書く（`CLAUDE.md`「Document language」の原則 (b)）。今の形では、道具を足すエージェントが従う規約がなく、日本語の ADR を指示として読むことになる。
-   ADR-0014 を規則の置き場所として指している文書は、次の 3 つである。
    -   `docs/dev/guides/chief_guide.md:22`（「ADR-0014 の『道具は `main` からインストールして使う』」）
    -   `docs/dev/guides/numreserve_guide.md:11`（同じ）
    -   `.claude/commands/_lib/adr-correction-candidates.sh:62-64` のコメント（Go に置く道具の規則の置き場所として ADR-0014 を名指す）

    ほかに ADR-0014 を指している文書は、ADR-0014 の決定とその理由を指していて、規則の置き場所を指してはいない。ADR-0013（`:36`）、ADR-0015（`:21`）、ADR-0037（`:21`・`:44`）、ADR-0046（`:70`・`:133`）、ADR-0051（`:18`）、ADR-0053（`:18`）、`docs/dev/guides/chief_guide.md:3` である。ADR-0051 の「ADR-0014 の開発の道具として `main` から入れる」も、道具が ADR-0014 の決定に従うことを述べている。`docs/dev/conventions/package_reference.md` は ADR-0014 を指していない。
-   「エージェントは道具をインストールもビルドもしない」と明記している道具は、今は 3 つだけである。`CLAUDE.md`「Tool Execution Safety」が `codexreview`（`CLAUDE.md:1006-1007`）・`numreserve`（`CLAUDE.md:1015-1016`）・`chief`（`CLAUDE.md:1032`）について書いている。ADR-0014 の「道具は `main` からインストールして使う」（`docs/dev/adr/0014-deterministic-checks-in-go.md:22`）は、エージェントについては何も書いていない。
-   `fixprguard` と `mergepr` を入れる手順は、誰が入れるかが揃っていない。
    -   利用者に案内している箇所：`.claude/commands/fixpr.md:42`・`:72`、`.claude/commands/runpr.md:199-202`、`.claude/commands/chief.md:99-100`。
    -   主語のない箇所：`.claude/commands/mergepr.md:18-19`（「Install it with `make install-mergepr` …」）。エージェントへの指示とも読める。
    -   プロトコル番号を上げる PR で、その PR のブランチから入れ直す手順にも主語がない。`CLAUDE.md:816-819`（「Protocol numbers」の「reinstall the tool from the branch」）、`docs/dev/guides/mergepr_guide.md:48`、同 7 章の手順 5（`:286`）、`docs/dev/guides/fixprguard_guide.md:54`、`docs/dev/guides/chief_guide.md:30` である。`mergepr.md` はエージェントに `mergepr_guide.md` を読ませる（`CLAUDE.md`「Document language」の例外）。
-   `coveragecompare` は、インストールせずに使う。`make coverage-compare` と `make ext-coverage-compare` が、そのつど一時ディレクトリにビルドして走らせる（`docs/dev/adr/0014-deterministic-checks-in-go.md:22`）。この 2 つは、エージェントがテストの削除を確かめるときに走らせる（`CLAUDE.md`「Testing Strategy」）。
-   エージェントが道具を入れることを技術的に止める仕組みはない。`.claude/settings.json` の許可の規則には、`go install`・`make install-*`・`go run ./cmd/*` を拒む項目がない。

以上の行番号は、コミット `a73b2c4e`（`origin/main`）で確かめた。

対応 issue: [#1014](https://github.com/isseis/yt2column/issues/1014)

**本書の記述範囲:** 本書は、規約が持つ規則、ADR-0014 の書き直しの範囲、`CLAUDE.md` とガイドとコマンドの定義で変えるもの、参照の付け替え、変えないものを定める。規約の節の構成、ADR-0014 から外す本文の行き先、各文書の書き方は `02_architecture.md` で、確かめ方は `03_implementation_plan.md` で定める。`design_handoff.md` と `implementation_handoff.md` に、要件定義の際に人が確かめた設計と計画の決定と、レビューで出た設計と計画の懸念を置く。

**issue #1014 で決めたこと:**

-   **規約に移す規則は、次の 6 つとする。規則の内容は変えない（ただし、下の「要件定義の際に決めたこと」が規則 5 を広げる）。** 規則の番号は、本書でもこの番号を使う。各規則は ADR-0014 の項目の条件をそのまま持つ。
    1.  判定と副作用を持つ開発の道具は Go で書き、テストを持つ。形は `cmd/<名前>` と `internal/<名前>` の組とし、製品のバイナリには入れない（ADR-0014 `:17`）。
    2.  道具は、判定に使う基準値を状態のファイルから読み返さない。基準値と判定した集合はサブコマンドの出力として返し、セッションが次の呼び出しの引数で渡す。chief の例外は範囲を ADR-0014 と ADR-0046 に残し、規約は例外があることだけをリンクで示す（ADR-0014 `:18`・`:19`）。
    3.  処理の順序は、コマンドの定義ではなく道具のコードで強制する。コマンドの定義には、サブコマンドを呼ぶ順序と利用者への確認だけを書く（ADR-0014 `:20`）。
    4.  道具の引数・出力・終了コードは、コマンドの定義と文書のガードが読む契約である。変えるときは、道具と、それを呼ぶ文書と、ガードを同じ PR で変える（ADR-0014 `:24`）。
    5.  道具は `main` からインストールして使い、道具を変えたマージの後は入れ直す。エージェントは道具をインストールもビルドもしない（`CLAUDE.md`「Tool Execution Safety」と重なる部分は、そちらへリンクする）（ADR-0014 `:22`）。
    6.  例外として、記録の行を読んで判定するだけで副作用を持たない判定は、コマンドの定義から独立したシェルスクリプトにしてよい。条件は、`.claude/commands/_lib/` に置くこと、`origin/main` の commit から作業ツリーの外に取り出して走らせること、失敗は安全な側の判定とすること、規則の全体を常設の Go のテストで確かめ、規則を壊して失敗することを確かめることの 4 つである（ADR-0014 `:25`）。

    理由：エージェントが従う規則の正本を、英語の規約に置くためである（背景）。
    -   ADR: ADR-0014 の「決定」を、要約と規約へのリンクに書き直す。規則の置き場所を正本の文書に移すことは決定を変えないので、編集上の訂正（`docs/dev/conventions/adr.md`「Editorial corrections」の 1 つ目の種類）として記録する。ADR の規約は「既存の決定を正本の文書に書き移すことは決定を変えない」と定め、正本の文書の例に `CLAUDE.md` と `.claude/commands/_context.md` を挙げている（同「Writing an ADR」）。新しい ADR は作らない。
-   **ADR-0014 には、理由・採らなかった案・経緯を残す。** ADR が持つものは理由・採らなかった案・経緯であり、仕様は正本の文書が持つためである（`docs/dev/conventions/adr.md`「Writing an ADR」）。
    -   ADR: 上と同じ（ADR-0014 の編集上の訂正）。
-   **`CLAUDE.md` の「Quick Links」の「Development Guides」に、規約へのリンクを足す。** 規約へたどれるようにするためである。
    -   ADR: なし。リンクの追加で、決定を変えない。
-   **ADR-0014 を規則の置き場所として指している文書を、規約を指すように直す。** 規則の置き場所を 1 つにするためである。
    -   ADR: なし。参照の付け替えで、決定を変えない。
-   **プロトコル番号の規則は対象外とする。** 正本が `CLAUDE.md` の「Protocol numbers」にすでにあるためである。ただし、その手順の主語は本タスクで直す（下の「要件定義の際に決めたこと」）。
    -   ADR: なし。
-   **ADR-0024（環境変数を読む場所）と ADR-0019（`context.Context` を第 1 引数にする規則など）の規則を規約に移すかは、別の issue で扱う。**
    -   ADR: なし。

**要件定義の際に決めたこと（2026-11-19）:**

-   **進め方は通常の進め方（`full`）とする。最初は簡易な進め方を選んだが、3 文書を書いたあとで通常の進め方に切り替えた。** 2026-11-19 に人が選んだ。経緯は次のとおりである。
    -   最初の判断：変更は文書だけで、新しい仕組みを作らないので、簡易な進め方を推奨し、人が選んだ。
    -   除外の判断：規則 5 を `fixprguard` と `mergepr` に広げることに決まった（下の項目）。この 2 つは開発の道具の信頼境界の道具である（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）。エージェントが道具を入れないという規則はその境界に影響するので、除外に当たると判断し直した（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。人は、コマンドの定義がすでに利用者に入れ直しを案内している運用を規則として書くだけだとして、判断を上書きした。
    -   切り替え：3 文書のレビューで、上書きの前提の一部が誤っていると分かった。`mergepr.md:18-19` の指示には主語がなく、利用者への案内になっていない（背景）。また、プロトコル番号を上げる PR の入れ直しの手順を直すことになり、範囲が信頼境界の道具のコマンドの定義とガイドに広がった（下の項目）。迷う場合は通常の進め方を推奨するという規約に従って切り替えを推奨し、人が選んだ。要件定義の際に人が確かめた設計と計画の決定は、`design_handoff.md` と `implementation_handoff.md` の確かめ済みの項目に置いた。
    -   ADR: なし。本タスクの中で閉じる、進め方の選択である。
-   **規則 5 の「エージェントは道具をインストールもビルドもしない」は、利用者が `make install-<名前>` で `main` から入れる道具（以下、入れる道具）のすべてに当てはめる。今は `mergepr`・`fixprguard`・`codexreview`・`chief`・`numreserve` の 5 つである。** 2026-11-19 に人が選んだ。理由は 2 つある。1 つ目は、エージェントが PR のブランチから `fixprguard` や `mergepr` を入れ直すか走らせると、そのブランチが変えた判定でガードが働き、ガードを迂回できることである。2 つ目は、入れ直しの多くの手順がすでに利用者に案内しており（背景）、案内していない箇所も同じ形に揃えられることである。make のターゲットがそのつどビルドして走らせる道具（以下、そのつどビルドする道具。今は `coveragecompare`）は、インストールしないので、この規則の対象ではない。
    -   採らなかった案：`CLAUDE.md` がすでに明記している 3 つ（`codexreview`・`numreserve`・`chief`）だけに当てはめる案。規則の内容は変わらないが、`fixprguard` と `mergepr` について規則がないままになる。
    -   ADR: ADR-0014 を更新する。決定の変更なので、「決定」に項目を足し、「経緯」に項目を足す（本 PR の [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)）。
-   **規則 5 の「ビルドする」は、道具を走らせるためのバイナリを作ること、またはソースから走らせることを指す。** 例は `make install-<名前>`、`go install`、`go build ./cmd/<名前>`、`go run ./cmd/<名前>` である。`make test`・`make lint`・`go test`・`go vet` のように、テストや lint のために道具のパッケージをコンパイルすることは含まない。2026-11-19 に人が選んだ。言葉どおりに読むと、エージェントが日々行うテストと lint まで禁じ、逆にインストールだけを禁じると、`go run` でブランチの判定を走らせる抜け道が残るためである。
    -   採らなかった案：インストール（`make install-*` と `go install`）だけを禁じる案。`go run` や `go build` で作ったバイナリでブランチの判定を走らせられる。
    -   ADR: ADR-0014 の更新（上の項目）に含める。
-   **プロトコル番号を上げる PR のブランチで道具を入れ直すのも、利用者とする。エージェントは、道具を入れるかビルドする手順に来たとき、または道具が見つからないか番号が合わずに止まったとき、止まって利用者に頼む。主語のない手順（背景の `CLAUDE.md:816-819`、`mergepr_guide.md:48`・`:286`、`fixprguard_guide.md:54`、`chief_guide.md:30`、`mergepr.md:18-19`）を、利用者が入れる形に直す。** 2026-11-19 に人が選んだ。主語がないままだと、「エージェントは入れない」とぶつかり、どちらが優先するかが分からないためである。
    -   受け入れる危険：プロトコル番号を上げる PR では、その PR の `/runpr` の間、ガードはブランチの判定で動く。入れ直すのは利用者で、利用者はその PR の変更を見たうえで入れ直すので、この危険は受け入れる。
    -   採らなかった案：主語の直しを範囲外にし、別の issue で扱う案。規則と手順がぶつかったまま `main` に入る。
    -   ADR: ADR-0014 の更新（上の項目）に含める。
-   **`CLAUDE.md` の「Tool Execution Safety」に、入れる道具のすべてについて「エージェントはインストールもビルドもしない」という 1 項目を足し、詳細は規約へリンクする。今の 3 つの道具の記述は残す。** 2026-11-19 に人が選んだ。この節は毎回読み込まれるので、`fixprguard` と `mergepr` を扱うときにも、エージェントの目に確実に入るためである。規則の正本は規約で、この項目と今の 3 つの記述は、それを毎回読み込まれる場所に置く写しである。
    -   採らなかった案：規約だけを正本にし、`CLAUDE.md` は「Quick Links」だけを変える案。規約を読まないエージェントには届かない。
    -   ADR: ADR-0014 の更新（上の項目）に含める。新しい ADR は作らない。
-   **規則を技術的に強制する仕組み（`.claude/settings.json` の拒否の規則など）は、本タスクでは足さない。強制は指示の文だけで、残る危険はセキュリティの節に書く。** 拒否の規則は、本タスクの issue の範囲を超えるためである。コマンドの前方一致で拒む規則は迂回できるので、足すとしても多層の守りの 1 つになる。
    -   ADR: なし。決定を変えない。
-   **ADR-0014 の「決定」のうち、移す 6 つの規則以外の項目は変えない。対象は、chief の例外（`docs/dev/adr/0014-deterministic-checks-in-go.md:19`）、拡張の関数の範囲を TypeScript で求める規則（`:21`）、プロトコル番号（`:23`）の 3 つである。** 2026-11-19 に人が選んだ。chief の例外は、規則 2 のとおり ADR に範囲を残す。TypeScript の補助は `coveragecompare` だけの決まりである。プロトコル番号は issue の対象外である。
    -   採らなかった案：プロトコル番号の項目も、要約と `CLAUDE.md` へのリンクに縮める案。issue の対象外の項目に手を広げることになる。
    -   ADR: なし。ADR-0014 のこの 3 項目は変えない。
-   **規約には規則だけを書き、理由は書かずに ADR-0014 へリンクする。** 2026-11-19 に人が選んだ。理由の正本を ADR-0014 の 1 か所に保ち、同じ内容を 2 か所で手入れしないためである。
    -   採らなかった案：規則ごとに 1 文の理由を添える案。エージェントが境界の場合を判断しやすくなるが、ADR と同じ内容を 2 か所に持つことになる。
    -   ADR: なし。規約の書き方の選択で、本タスクの中で閉じる。
-   **規則 1 と規則 6 の範囲の食い違いは、文言のまま規約に移し、直すかは別の issue で扱う。** 2026-11-19 に人が選んだ。規則 1 は「判定と副作用を持つ」道具を対象とし、規則 6 は「副作用を持たない」判定を例外とする。副作用を持たない判定はもともと規則 1 の外なので、規則 6 が何の例外かが文言からは決まらない。`adr-correction-candidates.sh:62-66` のコメントは、この食い違いを根拠に、自分は Go に置く道具ではないと述べている。直すと規則の範囲が変わり、決定の変更になるので、規則の内容を変えない本タスクでは扱わない。別の issue は、人が作る。
    -   ADR: なし。本タスクでは決定を変えない。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   道具を足すときや変えるときにエージェントが従う規則を、英語の規約 1 か所から読めるようにする。
-   ADR-0014 を、ADR の規約が定める形（要約と正本へのリンク）にする。
-   エージェントが、入れる道具をどれもインストールもビルドもしないことを規則にし、道具を入れる手順の主語を利用者に揃える。

### 2.2. スコープ (In Scope)

-   `docs/dev/conventions/dev_tools.md`（英語）を新しく作り、6 つの規則を書く（F-001）。
-   ADR-0014 の「決定」と「経緯」に、規則 5 を広げる決定を入れる（F-002。本 PR で行う）。
-   ADR-0014 の「決定」のうち、6 つの規則の項目を書き直す（F-002）。
-   `CLAUDE.md` の「Quick Links」と「Tool Execution Safety」に 1 項目ずつ足す（F-003）。
-   道具を入れる手順の主語を利用者に揃える。`CLAUDE.md`「Protocol numbers」、`mergepr.md`、`mergepr_guide.md`、`fixprguard_guide.md`、`chief_guide.md` が対象である（F-004）。
-   ADR-0014 を規則の置き場所として指している 3 か所を、規約を指すように直す（F-005）。

### 2.3. スコープ外 (Out of Scope)

-   規則 1〜4 と 6 の内容の変更。規則 5 は、1 章で決めた範囲だけを広げる。
-   規則 1 と規則 6 の範囲の食い違いの直し（1 章の「要件定義の際に決めたこと」、別の issue）。
-   ADR-0014 の「決定」のうち、chief の例外・TypeScript の補助・プロトコル番号の 3 項目と、「背景」「採らなかった案」「結果」の変更。
-   ADR-0014 の決定とその理由を指しているほかの文書（1 章の背景に挙げた ADR と `chief_guide.md:3`）の変更。規則の置き場所として指しているわけではないためである。
-   `CLAUDE.md`「Tool Execution Safety」にある、`codexreview`・`numreserve`・`chief` についての今の記述の変更。
-   すでに利用者に案内している手順（`fixpr.md:42`・`:72`、`runpr.md:199-202`、`chief.md:99-100`）の変更。
-   規則を技術的に強制する仕組み（`.claude/settings.json` の拒否の規則など）。
-   ADR-0024 と ADR-0019 の規則を規約に移すこと（issue #1014 の対象外）。
-   プロトコル番号の規則の内容（正本は `CLAUDE.md`「Protocol numbers」）。
-   道具のコードと Makefile の変更。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 規約 dev_tools.md

`docs/dev/conventions/dev_tools.md` を新しく作る。開発の道具を足すときや変えるときにエージェントが従う規則の正本とする。

**Acceptance Criteria**:
- **AC-01**: `docs/dev/conventions/dev_tools.md` があり、英語で書かれている。規則 1〜6（1 章の「issue #1014 で決めたこと」）が、それぞれ見出しのある節として書かれている。
- **AC-02**: 各規則の節は、ADR-0014 の対応する項目の条件を落とさず、広げない。規則 5 だけは、1 章の「要件定義の際に決めたこと」で広げた分を持つ。
- **AC-03**: 規約は、対象の道具を名前の一覧で示さない。入れる道具とそのつどビルドする道具の 2 つの種類を定め、それぞれの一覧が `CLAUDE.md` のどこにあるかを示す。
- **AC-04**: 規則 2 の節は、chief の例外の範囲を書かない。例外があることだけを書き、ADR-0014 と ADR-0046 へリンクする。
- **AC-05**: 規則 5 の節は、次を書く。
    -   入れる道具は利用者が `main` から入れ、道具を変えたマージの後に入れ直すこと。プロトコル番号を上げる PR のブランチで入れ直すのも利用者であること。
    -   エージェントは入れる道具をインストールもビルドもしないこと。「ビルドする」の意味（1 章の「要件定義の際に決めたこと」）と、テストや lint のためのコンパイルは含まないこと。
    -   エージェントは、道具を入れるかビルドする手順に来たとき、または道具が見つからないか番号が合わずに止まったとき、止まって利用者に頼むこと。
    -   そのつどビルドする道具は、この節の対象でないこと。
    -   `CLAUDE.md`「Tool Execution Safety」と「Protocol numbers」へのリンク。
- **AC-06**: 規約は、規則の理由を書かない。理由は ADR-0014 にあることを、リンクで示す。
- **AC-07**: 規約は、プロトコル番号の規則を書かない。`CLAUDE.md` の「Protocol numbers」へのリンクで示す。

#### F-002: ADR-0014 の更新と書き直し

**Acceptance Criteria**:
- **AC-08**（本 PR）: ADR-0014 の「決定」に、エージェントは入れる道具をどれもインストールもビルドもしないこと、「ビルドする」の意味、プロトコル番号を上げる PR のブランチで入れ直すのも利用者であることが、1 つの項目として、太字の要約と理由とともに書かれている。「経緯」の最後に、この決定の項目（2026-11-19、本書へのリンク）がある。
- **AC-09**: ADR-0014 の「決定」のうち、規則 1〜6 の 6 つの項目と AC-08 の項目は、規則の本文（道具の名前の一覧、`make` のターゲットの一覧、手順の詳細）を持たない。各項目は、規則の要約と短い理由を持ち、規約の該当する節の見出しの名前を本文で示してリンクする。
- **AC-10**: 実装の PR での ADR-0014 の差分は、AC-09 の 7 つの項目の書き換えに限る。chief の例外・TypeScript の補助・プロトコル番号の 3 項目と、「背景」「採らなかった案」「結果」「経緯」は変わらない。
- **AC-11**: 実装の PR は、AC-10 の書き換えを編集上の訂正として記録する。コミットメッセージには訂正行を書き、PR の説明には訂正の段落を書く（`docs/dev/conventions/requirements_process.md`「Recording in the PR description」）。
- **AC-12**: `TestADRRecords` を含む `make test` が通る。

#### F-003: CLAUDE.md の 2 項目

**Acceptance Criteria**:
- **AC-13**: `CLAUDE.md` の「Quick Links」の「Development Guides」に、`docs/dev/conventions/dev_tools.md` へのリンクと、その要旨がある。
- **AC-14**: `CLAUDE.md` の「Tool Execution Safety」に、次の 1 項目がある。エージェントは入れる道具を、インストールもビルドもしない（「ビルドする」の意味を含む）。項目は規約へリンクする。

#### F-004: 道具を入れる手順の主語

**Acceptance Criteria**:
- **AC-15**: `CLAUDE.md` の「Protocol numbers」の直し方の手順（今の `CLAUDE.md:816-819`）は、PR のブランチで道具を入れ直すのが利用者であることを述べる。
- **AC-16**: `docs/dev/guides/mergepr_guide.md` の `:48` と 7 章の手順 5、`docs/dev/guides/fixprguard_guide.md:54`、`docs/dev/guides/chief_guide.md:30` は、PR のブランチで道具を入れ直すのが利用者であることを述べる。
- **AC-17**: `.claude/commands/mergepr.md` の `mergepr` を入れる指示（今の `:18-19`）は、利用者が `main` から入れることを述べ、エージェントへの命令の形でない。
- **AC-18**: 実装の PR での `CLAUDE.md` の差分は、AC-13・AC-14・AC-15 の 3 か所に限る。

#### F-005: 参照の付け替え

**Acceptance Criteria**:
- **AC-19**: `docs/dev/guides/chief_guide.md:22` と `docs/dev/guides/numreserve_guide.md:11` の「道具は `main` から入れる」の規則の参照が、ADR-0014 ではなく規約の規則 5 の節を指す。
- **AC-20**: `.claude/commands/_lib/adr-correction-candidates.sh` のコメントで、Go に置く道具の規則の置き場所として ADR-0014 を名指している箇所が、規約を名指す。スクリプトのコードは変わらず、`TestADRCorrectionCandidates` が通る。
- **AC-21**: 実装の PR で変わるファイルは、次に限る。
    -   `docs/dev/conventions/dev_tools.md`（新規）
    -   `docs/dev/adr/0014-deterministic-checks-in-go.md`
    -   `CLAUDE.md`
    -   `.claude/commands/mergepr.md`
    -   `docs/dev/guides/chief_guide.md`
    -   `docs/dev/guides/numreserve_guide.md`
    -   `docs/dev/guides/mergepr_guide.md`
    -   `docs/dev/guides/fixprguard_guide.md`
    -   `.claude/commands/_lib/adr-correction-candidates.sh`
    -   本タスクのディレクトリ（`docs/tasks/0112_dev_tools_convention/`）
- **AC-22**: 実装の PR でのガイドとコマンドの定義の差分は、AC-16・AC-17・AC-19 の箇所に限る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A。文書だけの変更である。

### 4.2. セキュリティ (Security)

`docs/dev/security.md` の条件（外部コマンドの起動、API キー、キャッシュ、ネットワーク、信頼できない文字列の埋め込み）には当たらない。ただし、規則 5 を広げることは、開発の道具の信頼境界（ADR-0015）に関わるので、次を定める。

-   **守るもの:** `/fixpr`・`/runpr`・`/mergepr` のガードが、`main` の判定で動くこと。
-   **脅威:** 信頼できないレビューの本文を読んだエージェントが、PR のブランチから `fixprguard` や `mergepr` を入れるか走らせ、そのブランチが変えた判定でガードを働かせる。
-   **影響の範囲:** `$(go env GOPATH)/bin` の道具は、同じ利用者のすべての worktree とその後のセッションで共有される（`docs/dev/guides/fixprguard_guide.md` 3 章）。置き換えは、利用者が `main` から入れ直すまで続く。
-   **守り:** 規約と、毎回読み込まれる `CLAUDE.md`「Tool Execution Safety」の指示の文だけである。技術的に強制する仕組みはない（1 章の「要件定義の際に決めたこと」）。
-   **受け入れる危険:** エージェントが指示に従わない危険は残る。プロトコル番号を上げる PR では、利用者が入れ直したあと、ガードはそのブランチの判定で動く（1 章）。
-   **気づき方と戻し方:** 入れた道具が作られたコミットは `go version -m "$(command -v <道具>)"` の `vcs.revision` で確かめられる（`fixprguard_guide.md` 3 章、`chief_guide.md:22`）。違えば、利用者が `main` から入れ直す。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A。道具とコマンドの振る舞いを変えない。

### 4.4. 互換性 (Compatibility)
-   どの道具のプロトコル番号も変えない。道具を入れ直す必要はない。
-   本 PR のマージから実装の PR のマージまでの間、ADR-0014 は広げた規則を持つが、`CLAUDE.md` はまだ 3 つの道具についてしか書いていない。その間にエージェントの振る舞いが今より緩むことはない。

### 4.5. 保守性 (Maintainability)
-   規約は道具の名前の一覧を持たない（AC-03）。道具を足しても、規約を直す必要はない。
-   規約は理由を持たない（AC-06）。理由の正本は ADR-0014 の 1 か所である。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   規約とコマンドの定義は英語で、ADR、ガイド、タスク文書は日本語で書く（`CLAUDE.md`「Document language」）。
-   ADR-0014 の書き直しは、ADR の規約 `docs/dev/conventions/adr.md`（「Writing an ADR」「Editorial corrections」）に従う。ADR から規約の節を指すときは、アンカーを使わず、見出しの名前を本文で示す（同「Links」）。
-   文書のテストは文の意味を確かめない（`CLAUDE.md`「Testing Strategy」）。規約の文は PR のレビューで読む。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **開発の道具:** `cmd/<名前>` と `internal/<名前>` の組で作る、開発の手順のための Go の道具。製品のバイナリには入らない（規則 1）。
-   **規約:** 本書では `docs/dev/conventions/dev_tools.md` を指す。ADR の書き方の規約は「ADR の規約 `docs/dev/conventions/adr.md`」と呼んで区別する。
-   **規則 1〜6:** 1 章の「issue #1014 で決めたこと」に挙げた 6 つの規則。
-   **入れる道具:** 利用者が `make install-<名前>` で `main` から入れる開発の道具。
-   **そのつどビルドする道具:** make のターゲットが、走らせるたびに一時ディレクトリにビルドする開発の道具。
-   **入れる・入れ直す:** インストールする・再インストールする。本書ではこの 2 語を使う。
-   **ビルドする:** 規則 5 では、道具を走らせるためのバイナリを作ること、またはソースから走らせること（1 章の「要件定義の際に決めたこと」）。
