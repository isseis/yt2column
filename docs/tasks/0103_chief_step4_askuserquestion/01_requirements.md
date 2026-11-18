# 要件定義書：/chief の step 4 の質問を AskUserQuestion で聞くことを明示する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `simple` |
| Created | 2026-11-18 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`/chief` の定義（`.claude/commands/chief.md`）で、人に問いを出す方法が step 3 と step 4 で違うことを、読み違えない形で書き直す要件を定める。

**背景:**

-   `chief.md` の step 3「The choices」は、待ち受けの間は質問でセッションを止めないと定める。ラウンドの最後に、番号つきの一覧（`[<number>] <description>`）をテキストで書いて、ターンを終える（`.claude/commands/chief.md:259-274`）。`once` のときは `AskUserQuestion` で聞く（`chief.md:275-278`）。
-   step 4「Acting on the answer」は、人が選んだ直後の問いを、待ち受けの間も `once` でも `AskUserQuestion` で聞くと冒頭の 1 文で定める（`chief.md:296-297`）。step 4 で人に聞くのは次の 4 つである。
    -   中継する選択肢の選択（`chief.md:312-320`）
    -   PR の詳細を示した後の問い。選択肢は `/runpr` を送る・送らない・ほかの PR を見る（`chief.md:321-325`）
    -   `停止を解く` の確認（`chief.md:347-350`）
    -   `/mkreq` の提案での issue 番号（`chief.md:341-343`）。今の文は「ask the human for the issue number first」で、聞き方を定めていない

    リセットの項目は、選んだらそれ以上は聞かずに送る（`chief.md:332-337`）。`/mkreq` 以外の提案も聞かずに送る（`chief.md:344-346`）。
-   step 4 の冒頭の 1 文は、step 3 の「待ち受けの間は質問で止めない」という原則と読み違えやすい。2026-11-18 に、chief セッションが PR #979 の詳細を示した後、step 4 の問いを `AskUserQuestion` で聞かず、選択肢をテキストで書いた。人は自由記述で答えることになり、続けて `/runpr 979` を chief セッションに直接打ち込んだ。本来は `chief send` で OpenCode の pane へ送る経路だった（issue #983「起きたこと」）。
-   モデルの切り替えの後に送り直すかどうかの問いは、未回答の問いとして表の下に残し（`chief.md:352-357`）、step 3 の一覧に `送り直す: <label>`・`送り直さない: <label>` として番号なしで示す（`chief.md:252-255`）。この問いはラウンドをまたいで待つ。

対応 issue: [#983](https://github.com/isseis/yt2column/issues/983)

**本書の記述範囲:** 本書は、`chief.md` の step 3 と step 4 で問いの出し方を述べる文の要件を定める。どの文をどう書き換えるかは `02_architecture.md` で、確かめ方は `03_implementation_plan.md` で定める。

**issue #983 で決めたこと:**

-   **step 4 の冒頭に、この節の問いは必ず `AskUserQuestion` で聞くことを明示する。** 今の 1 文は、step 3 の原則と読み違えやすいためである。
    -   ADR：なし。コマンドの定義の文を明確にするだけで、決定は変わらない。[ADR-0046](../../dev/adr/0046-chief-of-staff.md) は問いの出し方を定めていない。
-   **step 4 の冒頭に、step 3 の番号つきの一覧はラウンドの最後に出す `pending` の一覧にだけ使い、step 4 の問いには使わないことを明示する。** 同じ理由である。
    -   ADR：なし。同上。
-   **step 3 の「待ち受けの間」の説明に、step 4 の問いはこの例外であることを 1 行足す。** step 3 だけを読んでも、step 4 の問いを取り違えないためである。
    -   ADR：なし。同上。
-   **`chief` の手順（呼び出すサブコマンドと引数）は変えない。** issue の補足のとおり、変更は文だけである。
    -   ADR：なし。同上。

**要件定義の際に決めたこと（2026-11-18）:**

-   **進め方は簡易な進め方（`simple`）とする。この変更は、除外される変更に当たらない。** 変わるのは 1 つのコマンドの定義の文だけである。新しい仕組みは作らず、どの ADR の決定も変えない。未決の判断も少ない。除外の判断：変更は問いの出し方の文を明確にするだけで、次のどれも変えない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。
    -   `docs/dev/security.md` の規則と前提
    -   開発の道具（`fixprguard`・`codexreview`・`mergepr`）とその信頼境界。chief が送る入力と、人が選ぶまで送らない規則（`chief.md:34-37`、Rule 3）も変わらない
    -   外部依存
    -   ADR：なし。本タスクの中で閉じる、進め方の選択である。
-   **`/mkreq` の提案で issue 番号を聞く問いも、`AskUserQuestion` で聞く。人は issue 番号を自由記述の答えに書く（F-001）。** テキストで聞くと、待ち受けの間は、人が打った数字が step 3 の一覧の項目の番号として読まれうるためである（`chief.md:285-288`）。
    -   採らなかった案：自由記述の答えなので、この問いだけはテキストで聞く例外として残す案。上の取り違えが残る。
    -   ADR：なし。本タスクの中で閉じる。
-   **モデルの切り替えの後に送り直すかどうかの問いは、今のまま step 3 の一覧に示し、step 4 の問いの規則の対象にしない。対象にしないことを step 4 に書く（F-001）。** この問いはラウンドをまたいで待つ問いで、人が選んだ直後の確認ではないためである。`AskUserQuestion` で聞くと、答えるまでラウンドと自動送信が止まる。
    -   採らなかった案：`model_mismatch` が返った直後に `AskUserQuestion` で聞く案。人が pane のモデルを切り替えるまで待ち受けが止まる。
    -   ADR：なし。本タスクの中で閉じる。
-   **変更は `chief.md` だけにし、`docs/dev/guides/chief_guide.md` は変えない（F-002）。** issue の補足のとおりである。利用ガイドは、chief セッションが step 4 の問いをどう出すかを述べていない（`docs/dev/guides/chief_guide.md:50` は、`/chief` が番号つきの一覧を示すことだけを述べる）。
    -   採らなかった案：利用ガイドにも、step 4 の問いは `AskUserQuestion` で出ると書き足す案。人が読む手順は変わらず、同じ内容を 2 か所に持つことになる。
    -   ADR：なし。本タスクの中で閉じる。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   chief セッションが、step 4 の問いをテキストの一覧で書かず、`AskUserQuestion` で聞くようにする。
-   step 3 と step 4 の、問いの出し方の境目を、どちらの節から読んでも分かるようにする。

### 2.2. スコープ (In Scope)

-   `chief.md` の step 4 の冒頭の文の書き換え（F-001）。
-   `chief.md` の step 4 の提案の項目で、issue 番号の聞き方を定める文の書き換え（F-001）。
-   `chief.md` の step 3 の「待ち受けの間」の項目への 1 行の追加（F-001）。

### 2.3. スコープ外 (Out of Scope)

-   `chief` の道具（`cmd/chief`・`internal/chief`）の変更。呼び出すサブコマンドと引数も変えない（issue の補足）。
-   `docs/dev/guides/chief_guide.md` の変更。理由は 1 章の「要件定義の際に決めたこと」にある。
-   モデルの切り替えの後に送り直すかどうかの問いの出し方の変更。理由は 1 章の「要件定義の際に決めたこと」にある。
-   step 4 の問いが開いている間にラウンドが止まることへの対処。今の定義（`chief.md:296-297`）のままで、人が選んだ直後に答える問いなので、止まるのは短い間である。
-   ADR-0046 の変更。問いの出し方は ADR の決定に含まれない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: step 4 の問いを AskUserQuestion で聞くことを明示する

**Acceptance Criteria**:
- **AC-01**: `chief.md` の step 4 の冒頭は、この節で人に聞く問いを、待ち受けの間も `once` でも `AskUserQuestion` で聞くと述べる。対象の問いとして、中継する選択肢の選択、PR の詳細を示した後の問い、`停止を解く` の確認、`/mkreq` の提案での issue 番号の 4 つを挙げる。
- **AC-02**: `chief.md` の step 4 の冒頭は、step 3 の番号つきの一覧（`[<number>] <description>` の行）をラウンドの最後に出す選択肢にだけ使い、step 4 の問いをその形でもほかのテキストでも書かないと述べる。
- **AC-03**: `chief.md` の step 4 の冒頭は、モデルの切り替えの後に送り直すかどうかの問いが AC-01 の対象でなく、step 3 の一覧に示すものだと述べる。
- **AC-04**: `chief.md` の step 4 の提案の項目は、`/mkreq` の issue 番号を `AskUserQuestion` で聞き、人が番号を自由記述の答えに書くと述べる。選択肢は「送らない」と「ほかの項目を見る」である。自由記述の答えが issue 番号でなければ、選択として扱わず、何も送らない。
- **AC-05**: `chief.md` の step 3 の「待ち受けの間」の項目は、step 4 の問いはこの項目の例外で、待ち受けの間も `AskUserQuestion` で聞くと述べ、step 4 を参照する。

#### F-002: 変更の範囲を文に限る

**Acceptance Criteria**:
- **AC-06**: 実装の PR の差分のうち、`.claude/commands/chief.md` 以外のファイルの変更は、本タスクのディレクトリ（`docs/tasks/0103_chief_step4_askuserquestion/`）の中に限る。
- **AC-07**: `chief.md` の差分は、step 3 の「待ち受けの間」の項目（`chief.md:259-274`）、step 4 の冒頭の段落（`chief.md:296-297`）、step 4 の提案の項目のうち `/mkreq` の issue 番号を聞く文（`chief.md:341-343`）の追加・書き換えに限る。`chief` を呼び出す行（`chief <subcommand> ...` のコードスパン）は変わらない。
- **AC-08**: `go test -tags test ./cmd/chief` が通る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A。文の変更だけである。

### 4.2. セキュリティ (Security)
-   N/A。`docs/dev/security.md` の条件（外部コマンドの起動、API キー、キャッシュ、ネットワーク、信頼できない文字列の埋め込み）のどれにも当たらない。chief が送る入力も、人が選ぶまで送らない規則（`chief.md:34-37`）も変えない。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A。道具の振る舞いを変えない。

### 4.4. 互換性 (Compatibility)
-   `chief` の protocol 番号（`chief.md` の `--protocol 3`）を変えない。道具を入れ直す必要はない。

### 4.5. 保守性 (Maintainability)
-   step 3 と step 4 の問いの出し方は、それぞれの節に書き、互いに参照する（AC-02・AC-05）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   コマンドの定義は英語で書く（`CLAUDE.md`「Document language」の原則 (b)）。
-   文書のテストは文の意味を確かめない（`CLAUDE.md`「Testing Strategy」）。文の中身はレビューで読んで確かめる。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **ラウンド:** `/chief` が状況を読み取って示す 1 回分（用語集の「ラウンド」）。
-   **待ち受け:** 引数なしの `/chief` が、ラウンドを繰り返しながら人の答えを待つこと（`chief.md` の「watching」。`chief_guide.md:50`）。
-   **step 4 の問い:** `chief.md` の step 4 で、人が項目を選んだ直後に聞く問い（AC-01 の 4 つ）。
