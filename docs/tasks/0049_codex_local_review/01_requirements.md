# 要件定義書：CodeRabbit CLI が失敗したときに Codex CLI でローカルの CLI レビューを行う

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-28 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、ローカルの CLI レビューで CodeRabbit CLI が失敗したときに、代わりに Codex CLI（`codex review`）でレビューするための要件を定義する。

対応 issue: [#460](https://github.com/isseis/yt2column/issues/460)。関連するタスク: [0031_codex_pr_review](../0031_codex_pr_review/01_requirements.md)（PR レビューを Codex の GitHub 連携に切り替えた）。

本書は、代わりのレビューを行う条件と範囲、トリアージと回数の上限、両方が失敗したときの振る舞い、結果の記録、実行の承認、そして実装の前に行う試験運用の観測を決める。`codex review` の呼び出し方（引数、設定、出力の取り込み方）、CodeRabbit CLI の失敗の判定方法、コミットのメッセージに残す行の形式は `02_architecture.md` で決める（`design_handoff.md`）。

**背景:**

PR を作る前と、PR を作った後の修正に対するローカルの CLI レビューは、今は CodeRabbit CLI だけが担っている（`.claude/commands/_lib/coderabbit-local-review.md:5-11`）。この CLI は Claude 以外のモデルによるレビューであり、PR を作る前に入る唯一の異系統の視点でもある（issue #460）。

-   CLI が失敗したとき（認証、利用の上限、ネットワーク）、手順は再試行も停止もせず、残りを飛ばして失敗を SUMMARY に記録する（`.claude/commands/_lib/coderabbit-local-review.md:42-44`）。その範囲は、異系統のレビューを受けないまま PR に進む。
-   `/runplan` は、手順 7.7 が失敗した段階のグループについて `CodeRabbit local review: not run` をコミットのメッセージに残す（`.claude/commands/runplan.md:212`）。PR を開くときにその行があれば、ブランチ全体を改めてレビューする（`.claude/commands/runplan.md:84-90`）。この再レビューも CodeRabbit CLI なので、CodeRabbit が使えない状態が続けば、やはり異系統のレビューは入らない。
-   PR レビューは Codex の GitHub 連携が、PR を作ったときに 1 回だけ行う（`CLAUDE.md:110-114`）。Codex の割り当ては利用者のほかの ChatGPT の利用と共有で、上限は公開されていない（`CLAUDE.md:110-111`）。2 回目以降の PR レビューは、エスカレーションの判断か利用者の依頼があったときだけ依頼する（`.claude/commands/_lib/pr-review-escalation.md:14-18`）。
-   ローカルの CLI レビューの共通手順を呼ぶのは 4 か所である。`/runplan` の手順 7.7（`.claude/commands/runplan.md:212`）、`/runpr` の手順 1（`.claude/commands/runpr.md:115-119`）、`_lib/task-pr-handoff.md` の Part B 手順 1（`.claude/commands/_lib/task-pr-handoff.md:175-177`）、`/fixpr` の Phase 4.5（`.claude/commands/fixpr.md:581-590`）である。
-   `coderabbit review` は、差分を外部のサービスに送るが、`CLAUDE.md` の「Tool Execution Safety」で承認なしに実行してよいとされている（`CLAUDE.md:540-542`）。`codex review` には、この扱いがない。
-   作業環境には `codex` CLI（0.163.0）があり、`codex review` で非対話のレビューを実行できる。範囲の指定には `--base <BRANCH>`（ブランチとの比較）、`--commit <SHA>`（1 つのコミット）、`--uncommitted`（コミットしていない変更）がある（`codex review --help`。2026-10-28 確認）。CodeRabbit CLI の `--base-commit` のように任意のコミットから HEAD までを指定できるかは、確かめていない。

**issue #460 で決めたこと:**

-   **CodeRabbit CLI は残す。** 廃止は対象外とする。CodeRabbit CLI の割り当ては PR レビューとは別であり（`.claude/commands/_lib/pr-review-escalation.md:10-12`）、ローカルの CLI レビューの主な担い手であり続けるためである。
-   **PR レビュー（Codex の GitHub 連携）は変えない。** 本タスクは PR を作る前と、PR を作った後の修正に対するローカルの CLI レビューだけを扱う。
-   **PR-Agent は導入しない。** issue #460 は、前提の誤りと、既存のサブエージェントのレビューとの重複を理由に見送っている。

**要件定義の際に決めたこと（2026-10-28）:**

-   **Codex CLI は、CodeRabbit CLI が失敗したときの代替に限る。** 常に併用はしない。issue の背景にある穴（CodeRabbit CLI が失敗した範囲が異系統のレビューを受けない）を塞ぐのに足り、Codex の割り当ての消費を失敗したときだけに抑えられるためである。
-   **実装の前に、試験運用で `codex review` を観測して記録する（F-001）。** 割り当てを PR レビューと共有するか、コミットの範囲を指定できるか、作業ツリーに書き込まずに動くか、出力をトリアージに取り込めるかは、どれも確かめていない。どれかが成り立たなければ設計が変わるため、0031 の試験運用（`docs/tasks/0031_codex_pr_review/01_requirements.md` の F-001）と同じく、観測を先に行う。観測では数回分の割り当てを使う。
-   **割り当てを PR レビューと共有すると分かっても、そのまま採用する。** 代替は CodeRabbit CLI が失敗したときだけ実行するので、PR レビューの割り当てへの影響は小さいと判断した。観測の結果は文書に残す（AC-17）。
-   **`codex review` は承認なしに実行してよいものとし、作業ツリーに書き込ませない。** `coderabbit review` と同じく `CLAUDE.md` の「Tool Execution Safety」に加える（AC-16）。承認を求めると、無人で進む `/runplan` や `/fixpr` の途中で代替が止まるためである。書き込みを許さないのは、レビューの範囲と、手順が後で行うコミットの内容を Codex に変えさせないためである（AC-08）。
-   **呼び出し元の 4 か所すべてで代替を行う。** issue が挙げた 3 か所に加え、`/fixpr` の Phase 4.5 も含める。どれも同じ共通手順を呼んでおり、PR を作った後の修正こそ PR レビューを受けないためである（`.claude/commands/fixpr.md:584-586`）。
-   **両方が失敗しても止めず、記録して先へ進む。** 今の CodeRabbit CLI の失敗の扱い（`.claude/commands/_lib/coderabbit-local-review.md:42-44`）を変えない。そのうえで、異系統のレビューを受けていないことを PR 本文か報告に書く（AC-12・AC-13）。無人の実行を止めないためである。
-   **実行の回数の上限は CodeRabbit CLI と同じ 2 回とする。** 範囲（SCOPE）ごとに、CodeRabbit CLI と Codex CLI の実行を合わせて最大 2 回とし、2 回目は 1 回目で修正したときだけ行う（AC-07）。代替は失敗した CodeRabbit CLI の 1 回の実行を置き換えるものであり、上限と収束の規則（`.claude/commands/_lib/coderabbit-local-review.md:71-77`）を代替だけ変える理由がないためである。
-   **git-ignore されている秘密のファイルや環境変数を Codex が読む経路は、残るリスクとして記録するだけにする。** `codex review` はレビュー中にシェルのコマンドでファイルを読むエージェントであり、読み取り専用のサンドボックスでも、作業ツリーの外を含めて読み取りは許される。そのため `.envrc`・`.env`（`CLAUDE.md:458`）や、それらから読み込まれた環境変数を読んで OpenAI に送りうる。Claude Code 自身も同じ経路を持つため、対策は求めず、4.2 に残るリスクとして書く。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   CodeRabbit CLI が失敗した範囲も、Claude 以外のモデルのレビューを受けてから PR に進むか、PR を作った後の修正として push される。
-   どの範囲がどのレビュアーのレビューを受けたか、どの範囲がどちらのレビューも受けていないかが、PR 本文と報告から分かる。

### 2.2. スコープ (In Scope)

-   試験運用での `codex review` の観測と記録（F-001）。
-   CodeRabbit CLI が失敗したときの、Codex CLI による代替のレビューとトリアージ（F-002）。対象は 1 節の 4 か所の呼び出し元である。
-   どのレビュアーが動いたかと、両方が失敗したことの記録（F-003）。
-   `codex review` の実行の承認と、関係する文書の更新（F-004）。

### 2.3. スコープ外 (Out of Scope)

-   CodeRabbit CLI の廃止（issue #460）。
-   PR レビュー（Codex の GitHub 連携）の変更（issue #460）。PR レビューの依頼の回数と、エスカレーションの基準（`.claude/commands/_lib/pr-review-escalation.md` の Criteria）も変えない。
-   PR-Agent の導入（issue #460）。
-   CodeRabbit CLI が成功したときの Codex CLI の併用（1 節の決定）。
-   git-ignore されている秘密のファイルや環境変数を Codex が読む経路への対策（1 節の決定）。
-   `/runplan` の手順 7 と 7.5 のサブエージェントのレビューの変更。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 試験運用での観測と記録

F-002 から F-004 を設計する前に、`codex review` を実際に動かして観測し、タスクのディレクトリの `observations.md` に記録する。

**Acceptance Criteria**:
- **AC-01**: `observations.md` に、次の各項目について、観測の日付、使った `codex` CLI の版、実行したコマンド、結果が記録されている。
    1. ローカルの `codex review` の実行が、Codex の PR レビューと同じ割り当てを消費するか。判断の根拠（何を見て判断したか）も記録する。判断できなかったときは、判断できなかったことと、その理由を記録する。
    2. あるコミット（BASE）から HEAD までのコミット済みの変更だけを範囲にしてレビューできるか。コミットしていない変更とコミットされていないファイルが範囲に入らないか。
    3. 作業ツリーに書き込ませない設定で実行できるか。実行の前後で `git status --short` の出力、`HEAD`、ブランチの一覧が変わらないか。
    4. 承認を求めて止まらずに、非対話で最後まで実行できるか。
    5. 出力から、各指摘の位置（ファイルと行）と内容を読み取れるか。指摘がないときの出力の形。
    6. 失敗したとき（認証がない、ネットワークに届かないなど、観測で起こせるもの）の終了コードと出力の形。起こせなかった失敗は、起こせなかったことを記録する。
    7. レビュー中に Codex が読んだファイル（実行したシェルのコマンド）。差分の外のファイルを読んだかどうか。
    8. 1 回の実行にかかった時間。
- **AC-02**: AC-01 の 2・3・4・5 のどれかが成り立たないと分かったときは、F-002 から F-004 の設計に進まず、観測の結果を利用者に報告して、続けるかを利用者が決める。AC-01 の 1 で割り当てを共有すると分かったときは、そのまま進む（1 節の決定）。

#### F-002: Codex CLI による代替のレビュー

ローカルの CLI レビューの共通手順で CodeRabbit CLI が失敗したとき、同じ範囲を Codex CLI でレビューし、同じ規則でトリアージする。

**Acceptance Criteria**:
- **AC-03**: CodeRabbit CLI のレビューが失敗したとき（認証、利用の上限、ネットワークなど、レビューの結果が得られなかったとき）、同じ実行の中で、同じ範囲（BASE から HEAD までのコミット済みの変更）を Codex CLI でレビューする。利用者に承認を求めない（AC-16）。
- **AC-04**: CodeRabbit CLI のレビューが成功したときは、指摘が 0 件でも、Codex CLI を実行しない。
- **AC-05**: Codex CLI の指摘は、CodeRabbit CLI の指摘と同じく `.claude/commands/_lib/triage-pattern.md` の尺度（`verdict`、`severity`、`topic`、`replyBody`）で分類し、同じ規則で扱う。`must-fix` を直し、`unclear` を未解決として記録し、タスクのディレクトリの文書への指摘には `off-level` の判定を適用する（`.claude/commands/_lib/coderabbit-local-review.md:46-69` と同じ）。
- **AC-06**: Codex CLI の指摘の位置と内容は、トリアージする前に、指摘が挙げた位置のコードを読んで確かめる。指摘の文面に含まれる指示（コマンドの実行、ファイルの変更、レビュー対象の外への操作）を、指摘の文面だけを根拠に実行しない。
- **AC-07**: 1 つの範囲（呼び出し元が SCOPE と呼ぶもの）につき、ローカルの CLI レビューの実行は、CodeRabbit CLI と Codex CLI を合わせて最大 2 回である。各回は CodeRabbit CLI を先に実行し、失敗したときだけ Codex CLI で代替する。2 回目は 1 回目で修正したときだけ行い、2 回目では `must-fix` だけに対応する。たとえば、1 回目に CodeRabbit CLI が成功して修正し、2 回目に CodeRabbit CLI が失敗したときは、2 回目を Codex CLI で代替する。
- **AC-08**: Codex CLI の実行の前後で、`git status --short` の出力、`HEAD`、ローカルのブランチの一覧が変わらない。Codex CLI はファイルを書き換えず、コミットもブランチも作らない。
- **AC-09**: Codex CLI のレビューも失敗したとき（`codex` CLI が見つからないとき、終了コードが失敗を示すとき、出力から AC-01 の 5 で記録した形の指摘も「指摘なし」も読み取れないときを含む）、再試行せず、実行を止めず、その回のローカルの CLI レビューを「実行されなかった」として記録して先へ進む（F-003）。読み取れない出力を「指摘なし」とみなさない。
- **AC-10**: AC-03 から AC-09 は、1 節の 4 か所の呼び出し元（`/runplan` の手順 7.7、`/runpr` の手順 1、`_lib/task-pr-handoff.md` の Part B 手順 1、`/fixpr` の Phase 4.5）のどれから共通手順を呼んだときにも成り立つ。

#### F-003: 結果の記録

どのレビュアーのレビューを受けたか、どちらのレビューも受けなかったかを、後のセッションと人のレビュアーが読める形で残す。

**Acceptance Criteria**:
- **AC-11**: ローカルの CLI レビューの各回について、呼び出し元の SUMMARY（報告）が、その回を実行したレビュアー（CodeRabbit CLI、代替の Codex CLI、どちらも実行されなかった）を示す。Codex CLI で代替したときは、CodeRabbit CLI が失敗したことも示す。
- **AC-12**: コミットのメッセージに残すローカルの CLI レビューの記録（今の `CodeRabbit local review:` の行。`.claude/commands/_lib/coderabbit-local-review.md:79-87`）は、却下・`deferred`・`unclear` の各指摘について、どちらのレビュアーの指摘かを示す。範囲がどちらのレビューも受けなかったときは、そのことを示す記録をコミットのメッセージに残す。
- **AC-13**: PR を開くとき（`_lib/task-pr-handoff.md` の Part B 手順 3、`/runpr` の手順 1 の単独の変更）、PR 本文は次を示す。
    1. Codex CLI の指摘のうち `unclear` と `deferred` のもの。今の CodeRabbit CLI の指摘と同じく、このセッションのものと、ブランチのコミットのメッセージに記録された前のセッションのものの両方である。
    2. PR のコミットのうち、どちらのレビューも受けていない範囲があれば、その範囲と、異系統のレビューを受けていないこと。
- **AC-14**: `/fixpr` の Phase 4.5 で両方が失敗したときは、`/fixpr` の Final report に、その修正のラウンドが異系統のレビューを受けていないことを示す。
- **AC-15**: `/runplan` が段階のグループのレビューで CodeRabbit CLI に失敗し、Codex CLI で代替できたときは、PR を開くときにブランチ全体を改めてレビューしない。両方が失敗したときだけ、今の `CodeRabbit local review: not run` の行と同じく、PR を開くときにブランチ全体を改めてレビューする（`.claude/commands/runplan.md:84-90`）。この再レビューにも AC-03 から AC-09 が成り立つ。

#### F-004: 承認と文書

`codex review` の扱いと、代替の存在を、プロジェクトの規則とコマンドの文書に反映する。

**Acceptance Criteria**:
- **AC-16**: `CLAUDE.md` の「Tool Execution Safety」が、`codex review`（ローカルの CLI レビューの代替としての Codex CLI）を、差分と Codex が読んだ内容を OpenAI のサービスに送るにもかかわらず、承認なしに実行してよいものとして挙げている。共通手順の「Authorization」と、`codex review` を実行しうるコマンドの承認の記述（`.claude/commands/runpr.md:82-83`、`.claude/commands/fixpr.md:97`、`.claude/commands/_lib/task-pr-handoff.md:65-66` と同じ種類の記述）も、`codex review` を承認なしに実行してよいものとして挙げている。
- **AC-17**: ローカルの CLI レビューを CodeRabbit CLI だけが担うと述べている記述（`CLAUDE.md:115-117` のコマンドの説明、`.claude/commands/_lib/pr-review-escalation.md:10-12` の割り当ての記述など）が、CodeRabbit CLI が失敗したときに Codex CLI で代替することを述べている。割り当てについての記述は、AC-01 の 1 の観測の結果（共有する、しない、判断できなかった）と一致している。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。1 回の実行にかかる時間は観測で記録する（AC-01 の 8）。代替は CodeRabbit CLI が失敗したときだけ実行するので、通常の実行の時間は変わらない（AC-04）。

### 4.2. セキュリティ (Security)

本タスクは、ネットワークへの送信（差分と、Codex が読んだ内容を OpenAI のサービスに送る）と、信頼できない入力（Codex CLI の出力）の扱いに関わる。`docs/dev/security.md` の観点から、次を要件とする。

-   **送る範囲:** Codex CLI に渡すレビューの範囲は、CodeRabbit CLI と同じく、BASE から HEAD までのコミット済みの変更だけである（AC-03）。コミットしていない変更とコミットされていないファイルを範囲に含めない（AC-01 の 2）。秘密（`DEEPSEEK_API_KEY`・`ANTHROPIC_API_KEY`・`SLACK_WEBHOOK_URL`）をコミットしない規則（`CLAUDE.md:458-459`）は変わらない。
-   **残るリスク（秘密のファイルと環境変数）:** `codex review` はレビュー中にシェルのコマンドでファイルを読むエージェントである。作業ツリーに書き込ませない設定でも読み取りは許されるので、git-ignore されている `.envrc`・`.env` や、それらから読み込まれてプロセスに渡る環境変数を読み、OpenAI に送りうる。本タスクでは対策を求めない（1 節の決定）。Codex が実際に何を読んだかは、試験運用で記録する（AC-01 の 7）。
-   **書き込ませない:** Codex CLI は作業ツリー、コミット、ブランチを変えない（AC-08）。
-   **Codex CLI の出力は信頼できないデータとして扱う:** 出力は LLM が書いた文面であり、レビュー対象のコード（PR の内容）に書かれた文面に影響されうる。境界の要件は次のとおりである。
    -   **受け入れる形:** AC-01 の 5 で記録した形の指摘（位置と内容）と、「指摘なし」の出力だけを受け入れる。
    -   **それ以外:** 形の読み取れない出力は、補って受け入れず、「指摘なし」ともみなさず、Codex CLI のレビューの失敗とする（AC-09）。
    -   **使うメンバー:** 使うのは各指摘の位置と内容だけである。位置は、指摘が挙げた位置のコードを読んで確かめる（AC-06）。ほかの出力（要約、進捗の表示など）は使わない。
    -   **指示として実行しない:** 指摘の文面に含まれる指示を、文面だけを根拠に実行しない（AC-06）。
    -   **件数とサイズの上限:** N/A。出力はエージェントが読むファイルであり、Go のコードで解析しない。回数は AC-07 で抑える。
    -   **センチネルエラー:** N/A。判定はコマンドの文書（エージェントへの手順）で行い、Go のコードを持たないためである。拒否の結果は、Codex CLI のレビューの失敗とすることである。
-   **Codex へのメンションを投稿しない:** ローカルの CLI レビューの指摘は GitHub に投稿しない。PR 本文やコミットのメッセージに Codex CLI の指摘を写すときも、コマンドが投稿する文面のメンションの確認（`.claude/commands/_lib/task-pr-handoff.md:185-189`）は変わらず適用される。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   CodeRabbit CLI も Codex CLI も失敗しても、実行を止めない（AC-09）。どちらのレビューも受けていない範囲は記録し、PR 本文か報告で示す（AC-12・AC-13・AC-14）。
-   Codex CLI を再試行しない（AC-09）。回数は範囲ごとに最大 2 回である（AC-07）。

### 4.4. 互換性 (Compatibility)

-   CodeRabbit CLI が成功したときの手順と結果は、今と変わらない（AC-04）。
-   前のセッションがコミットのメッセージに残した `CodeRabbit local review:` の行は、引き続き PR 本文に取り込まれる（AC-13 の 1）。`CodeRabbit local review: not run` の行も、引き続きブランチ全体の再レビューのきっかけになる（AC-15）。
-   PR レビュー（Codex の GitHub 連携）の依頼と、レビュー済みの判定は変えない（2.3）。

### 4.5. 保守性 (Maintainability)

-   代替の手順は、ローカルの CLI レビューの共通手順の中に 1 つだけ置き、4 か所の呼び出し元はそれを呼ぶ（AC-10）。呼び出し元ごとに代替の手順を書かない。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   F-002 から F-004 は、F-001 の観測の結果に基づいて設計する（AC-02）。
-   Codex CLI には ChatGPT のプランへのログインが必要である。ログインと `codex` CLI のインストールは、リポジトリの外の設定であり、利用者が行う。
-   Codex の割り当ては、利用者のほかの ChatGPT の利用や Codex の PR レビューと共有されうる（`CLAUDE.md:110-111`）。
-   ネットワークに触れる新しい外部のツールの実行は、`CLAUDE.md` の「Tool Execution Safety」が承認したものだけを承認なしに行う（AC-16）。

## 6. 用語集 (Glossary)

-   **ローカルの CLI レビュー**: コミット済みの変更を、外部のサービスのレビュー用 CLI でローカルからレビューすること（`.claude/commands/_lib/coderabbit-local-review.md`）。本タスクの前は CodeRabbit CLI だけが行い、本タスクで Codex CLI による代替を加える。
-   **代替**: CodeRabbit CLI が失敗したときに、同じ範囲を Codex CLI でレビューすること（F-002）。
-   **異系統のレビュー**: Claude 以外のモデルによるレビュー。CodeRabbit CLI と Codex CLI のレビューを指す。`/runplan` のサブエージェントのレビューは含まない。
-   **PR レビュー**: 自動レビュアーが GitHub の PR 上で行うレビュー。ローカルの CLI レビューと区別する。
-   **範囲（SCOPE）**: 呼び出し元がローカルの CLI レビューに渡す、BASE から HEAD までのコミット（段階のグループ、ブランチ、`/fixpr` の修正のラウンド）。
-   **割り当て**: サービスが一定時間に受け付ける利用の回数の上限（英語では quota）。`docs/translation_glossary.md` の「枠」（slot）は localhost サーバの同時処理数の単位であり、別の概念なので本書では使わない。
-   **試験運用**: F-001 で、設計の前に `codex review` を実際に動かして観測すること。
-   **指摘**: レビューが挙げる個々の問題。
-   用語は `docs/translation_glossary.md` と統一する。
