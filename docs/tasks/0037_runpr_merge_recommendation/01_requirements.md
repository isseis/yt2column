# 要件定義書：/runpr のマージの質問で、次のコマンドを続けて実行する選択肢を推奨にしない

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-22 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`/runpr` のマージの質問の推奨を、「次のコマンドを続けて実行する」から「マージだけ（マージ前の記録を含む）」に改めるための要件を定義する。あわせて、マージの後に `/clear` してから次のコマンドを実行するよう案内することも定義する（[#401](https://github.com/isseis/yt2column/issues/401)）。

**背景:**

`/runpr` は、PR のマージの前に 1 回だけ質問する（マージの質問）。この質問で、マージ前の記録（PRE_MERGE。文書の承認や計画書のチェックポイント）、マージ、次のコマンド（NEXT。`/mkarch`・`/mkplan`・`/runplan`）の扱いをまとめて決める（`.claude/commands/runpr.md:328-367`）。マージの選択肢のうち、「PRE_MERGE を記録し、マージし、NEXT を実行する」が推奨と定められている。この選択肢は NEXT があるときだけ示される（`runpr.md:359-360`）。推奨にする理由は書かれていない。NEXT がないときは、どの選択肢も推奨と示されない（`runpr.md:357-367`）。この選択肢を選ぶと、`/runpr` は step 5 で NEXT を同じセッションのまま実行する（`runpr.md:472-474`）。

次のコマンドに必要な文脈は、引き継ぎの文書（要件定義書・設計書・計画書と PR のマーカー）に載っている。そのため、`/clear` してから次のコマンドを実行しても、失うものはない。逆に続けて実行すると、レビューと修正を重ねた長い会話を次のコマンドに持ち越す。PR #400 のマージの際に、この点を指摘された（issue #401 の「背景」）。

NEXT が `/runplan` のときは、次の PR マーカーの `実装モデル要件` を質問のメッセージに示す。今のセッションのモデルが合うかを、利用者が判断できるようにするためである（`runpr.md:345-347`）。この値は最終報告には載らない（`runpr.md:472-474`）。

対応 issue: [#401](https://github.com/isseis/yt2column/issues/401)

**本書の記述範囲:** 本書は、マージの質問の選択肢と推奨、マージの後の報告、それらを説明する文書の記述を定める。次の事項は設計（`02_architecture.md`）で決める。

-   `runpr.md` の step 4・step 5 の文の書き方と、選択肢の並べ方の記述。
-   文書のガードのテストを加えるかどうかと、その形。

要件の起草で挙がった設計の事項は、[design_handoff.md](design_handoff.md) に申し送る。

**issue #401 で決めたこと:**

-   **マージだけを推奨にし、先頭に置く:** step 4 の質問では、「マージだけ（承認の記録を含む）」を推奨にし、選択肢の先頭に置く。次のコマンドに必要な文脈は引き継ぎの文書に載っているので、続けて実行する利点がないためである。
-   **続けて実行する選択肢は残す:** 「続けて NEXT を実行」は推奨にしないが、選べる選択肢として残す。短い PR のように持ち越す会話が短いときは、続けて実行したい利用者もいるためである。
-   **最終報告で `/clear` を案内する:** step 5 の最終報告に、「`/clear` してから `<NEXT>` を実行する」という案内を載せる。マージだけを選んだ利用者が、次に何をするかを報告から読み取れるようにするためである。
-   **文書の記述を合わせる:** `runpr.md` の冒頭など、NEXT を自動で続けるように読める記述を、同じ方針に合わせる。`CLAUDE.md` の「Claude Commands」と `_lib/task-pr-handoff.md` の説明が、新しい既定と食い違わないようにする。
-   **コマンドの定義は英語で書く:** `CLAUDE.md` の「Language」に従う。

**要件定義の際に決めたこと（2026-10-22）:**

-   **マージだけの選択肢は、NEXT の有無によらず推奨にする:** NEXT がないとき（standalone change、タスクの最後の plan PR など）も、マージだけの選択肢を推奨と示し、先頭に置く（AC-01）。`/runpr` を実行するのはレビューを終えた後で、マージはこのコマンドの目的である。また、NEXT の有無で推奨の付き方が変わらないほうが、利用者は質問を読み違えにくい。
-   **NEXT が `/runplan` のときは、案内に `実装モデル要件` を添える:** 最終報告の案内に、次の PR マーカーの `実装モデル要件` も示す（AC-06）。`/clear` で新しいセッションにすれば、`/runplan` の前にモデルを選び直せる。ところが今は、判断に使うこの値が質問のメッセージにしか出ない。issue #401 にない追加なので、ここに決定として記録する。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   マージの質問の既定（推奨）を、長い会話を次のコマンドに持ち越さないものにする。
-   マージだけを選んだ利用者が、次に何を実行すればよいかを最終報告から読み取れるようにする。
-   `/runpr` を説明する文書が、新しい既定と食い違わないようにする。

### 2.2. スコープ (In Scope)

-   `runpr.md` step 4 のマージの質問の、マージの選択肢の推奨と順序（F-001）。
-   `runpr.md` step 5 の最終報告の案内（F-002）。
-   `/runpr` が次のコマンドを実行すると述べる文書の記述（`runpr.md` の冒頭と見出し、`CLAUDE.md`、`_lib/task-pr-handoff.md`、`requirements_process.md`、`mergepr.md`）（F-003）。

### 2.3. スコープ外 (Out of Scope)

-   `/runpr` が自分で `/clear` を実行すること。コマンドの定義から会話を消す手段はなく、`/clear` は利用者が実行する。
-   NEXT の決め方（`runpr.md` step 2、`runpr.md:177-187`）を変えること。issue #401 は推奨と案内だけを扱う。
-   マージの質問のうち、マージの選択肢以外の質問を変えること。deferred のスレッドの質問と、PR レビューの質問（Escalation が `ask` のときに「request a PR review first」を推奨にする規則、`runpr.md:352-356`）は変えない。
-   ほかのコマンド（`/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2`・`/runplan`）が PR を作った後に止まる振る舞いを変えること。
-   完了したタスクの文書（`docs/tasks/` の過去のタスク）にある、`/runpr` が次のコマンドを実行するという記述を直すこと。過去の時点の記録だからである。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: マージの質問の推奨

`runpr.md` step 4 のマージの質問で、マージだけの選択肢を推奨にし、先頭に置く。続けて NEXT を実行する選択肢は、推奨にせずに残す。

**Acceptance Criteria**:
- **AC-01**: `runpr.md` step 4 は、マージの質問の「Merge」の選択肢のうち、「PRE_MERGE を記録してマージし、NEXT を実行しない」選択肢（PRE_MERGE がないときは「マージ」）を、先頭に置いて推奨と示すと定める。この定めは、NEXT の有無と PR の種類（plan PR、タスク文書の PR、standalone change）によらない。
- **AC-02**: NEXT があるとき、`runpr.md` step 4 は、「PRE_MERGE を記録してマージし、NEXT を実行する」選択肢を示すと定める。この選択肢は推奨と示さない。NEXT がないときは、今と同じく示さない。
- **AC-03**: `runpr.md` step 4 の「Merge」の選択肢で推奨と示すのは、AC-01 の選択肢だけである。「承認なしでマージ」と「マージしない」は、今と同じ条件で示し（`runpr.md:363-367`）、推奨と示さない。
- **AC-04**: マージの質問のメッセージは、今と同じく NEXT を示す。NEXT が `/runplan` のときは、次の PR マーカーの `実装モデル要件` も示す（`runpr.md:345-347`）。

#### F-002: マージの後の案内

マージだけを選んだとき、step 5 の最終報告で、`/clear` してから NEXT を実行するよう案内する。

**Acceptance Criteria**:
- **AC-05**: マージの質問の答えが NEXT を実行しないマージで、NEXT があるとき、`runpr.md` step 5 の最終報告は、`/clear` してから NEXT を実行するよう案内すると定める。案内の NEXT は、引数を含めて、そのまま実行できる形で示す（例: `/runplan 0037`）。
- **AC-06**: AC-05 の案内で、NEXT が `/runplan` のとき、案内は次の PR マーカーの `実装モデル要件` の値も示す。計画書に PR マーカーがなく、値がないときは、値がないことを示す。
- **AC-07**: NEXT がないとき（「承認なしでマージ」を選んだときを含む。`runpr.md:363-365`）、最終報告に AC-05 の案内を載せない。マージしなかったときも載せない。
- **AC-08**: マージの質問の答えが「PRE_MERGE を記録してマージし、NEXT を実行する」のときは、今と同じく step 5 で NEXT を実行する（`runpr.md:472-473`）。
- **AC-09**: NEXT があるときの次のブランチの作成（`runpr.md:451-454`）と、plan PR のチェックポイントの「次のブランチへ切り替えた」の記録（`runpr.md:174-176`）は、マージの選択肢によらず、今と同じである。そのため、`/clear` の後に実行した NEXT は、そのブランチで作業を始める（`_lib/task-pr-handoff.md` Part A step 3）。

#### F-003: 文書の記述

`/runpr` を説明する文書が、マージの後に NEXT を必ず、または既定で実行すると読める書き方をしないようにする。

**Acceptance Criteria**:
- **AC-10**: `runpr.md` の frontmatter の `description`（`runpr.md:2`）、冒頭の説明（`runpr.md:30-31`）、step 5 の見出し（`runpr.md:443`）は、NEXT を示し、利用者が選んだときに実行すると述べる。マージの後に NEXT を必ず実行するとは述べない。
- **AC-11**: 次の記述は、`/runpr` がマージの後に NEXT を示し、利用者が選んだときに実行すると述べる。マージの後に NEXT を必ず実行する、または既定で実行するとは述べない。
    -   `CLAUDE.md` の「Claude Commands」の `/runpr` の行（`CLAUDE.md:86`）と、タスクの PR の流れを説明する段落（`CLAUDE.md:98-102`）。
    -   `_lib/task-pr-handoff.md` の冒頭の説明（`.claude/commands/_lib/task-pr-handoff.md:18-22`）。
    -   `docs/dev/developer_guide/requirements_process.md` の「Review Flow」の説明（`requirements_process.md:55`）。
    -   `mergepr.md` step 3 の、`runpr.md` の中で実行するときの説明（`.claude/commands/mergepr.md:122-125`）。
- **AC-12**: 変更した文書は英語で書く（`CLAUDE.md` の「Language」と「Documents」。`requirements_process.md` は `docs/dev/developer_guide/*.md` の例外に当たる）。
- **AC-13**: `make test`・`make lint` と、CI の `doc-guard` のジョブが通る。

### 3.2. 信頼できない入力の境界 (Untrusted Input Boundaries)

N/A。本タスクはコマンドの文書の、選択肢の推奨と報告の文を変えるだけで、新しく受け取る入力はない。マージの質問の答えは利用者の入力だが、受け取り方（`AskUserQuestion` の選択肢と自由記述）は変えない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。

### 4.2. セキュリティ (Security)

-   N/A。本タスクは外部コマンドの実行、秘密、キャッシュ、ネットワークへの送信、信頼できないテキストの埋め込みのいずれも変えない（`.claude/commands/_context.md` の「Conditional-guide trigger」に当たらない）。マージには、今と同じく利用者の明示の答えが要る（`runpr.md:370-371`）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   N/A。

### 4.4. 互換性 (Compatibility)

-   マージの質問の選択肢の種類と、それぞれを選んだときの振る舞い（記録、マージ、ブランチの作成、NEXT の実行）は変えない。変えるのは、推奨の付け方、選択肢の順序、マージの後の報告である（AC-01〜AC-03、AC-05〜AC-09）。
-   1 つの質問の選択肢は、今と同じく 4 つ以下である（`runpr.md:347`）。

### 4.5. 保守性 (Maintainability)

-   マージの選択肢の推奨は、`runpr.md` step 4 の 1 か所で定める。ほかの文書は、`/runpr` が NEXT を示すことだけを述べ、どの選択肢が推奨かは書かない（AC-11）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   コマンドの定義（`.claude/commands/`）は英語で書く（`CLAUDE.md` の「Language」）。
-   マージは、マージの質問への利用者の答えがあるときだけ行う（`CLAUDE.md` の「Tool Execution Safety」、`runpr.md:67-68`）。本タスクはこの条件を変えない。

## 6. 用語集 (Glossary)

-   **マージの質問**: `/runpr` の step 4 が、マージの前に 1 回だけ利用者に尋ねる質問（`runpr.md:328-367`）。deferred のスレッド、PR レビュー、マージの選択肢をまとめて尋ねる。英語では merge question。
-   **マージ前の記録（PRE_MERGE）**: マージの直前に 1 つの commit で記録する、文書の承認と計画書のチェックポイント（`runpr.md:162-176`）。
-   **次のコマンド（NEXT）**: マージの後に示すコマンド。`/mkarch`・`/mkplan`・`/runplan` のいずれかか、なし（`runpr.md:177-187`）。
-   **実装モデル要件**: 計画書の PR マーカーの項目。PR の実装に要るモデルの段階（`frontier-required`・`frontier-recommended`・`standard`）を示す（`_context.md` の「PR marker conventions」）。
-   用語は `docs/translation_glossary.md` と統一する。
