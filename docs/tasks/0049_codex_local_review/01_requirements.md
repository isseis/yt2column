# 要件定義書：CodeRabbit CLI が失敗したときに Codex CLI でローカルの CLI レビューを行う

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-28 |
| Review date | 2026-10-28 |
| Reviewer | isseis |
| Comments | 2026-10-28: isseis が承認した（PR #553）。2026-10-28（決定の変更）: 設計の際に、Codex CLI の実行と出力の判定を、`main` からインストールする Go の開発用ツールで行うと利用者が決めた（`02_architecture.md` の 1.2）。4.2 の「センチネルエラー」を改めた。あわせて、Codex CLI の実行の結果を確かめられないときも止めることにし、1 節の決定、AC-09、4.3 を改めた。用語「回」を「試行」に改めた（6 節）。`draft` に戻し、`02_architecture.md` と同じ PR で再び承認を受ける。2026-10-29（決定の変更。isseis がこのセッションで承認した）: Codex のシェルに環境変数を引き継がせない対策を採ることにした。環境変数を引き継ぐと、Codex が実行するどのコマンドも秘密の値を持ち、`env` の実行や環境を含むエラーの出力から、ファイルを読まなくても暗黙のうちに OpenAI に送られうる。`.envrc` を読む経路とは別の経路であり、「採っても残るリスクは変わらない」という設計書 1.2 の判断は誤りだった（PR #573 の CodeRabbit の指摘と利用者の指摘による）。1 節の決定、2.3、4.2 を改めた。状態は `approved` のままとする |

## 1. 概要 (Overview)

**目的:** 本書は、ローカルの CLI レビューで CodeRabbit CLI が失敗したときに、代替として Codex CLI（`codex review`）でレビューするための要件を定義する。

対応 issue: [#460](https://github.com/isseis/yt2column/issues/460)。関連するタスク: [0031_codex_pr_review](../0031_codex_pr_review/01_requirements.md)（PR レビューを Codex の GitHub 連携に切り替えた）。

本書は、代替を行う条件と範囲、トリアージと回数の上限、両方が失敗したときの振る舞い、結果の記録、実行の承認、そして設計の前に行う試験運用の観測を決める。`codex review` の呼び出し方（引数、設定、出力の取り込み方）、CodeRabbit CLI の失敗の判定方法、コミットのメッセージに残す行の形式は `02_architecture.md` で決める（`design_handoff.md`）。

**背景:**

PR を作る前と、PR を作った後の修正に対するローカルの CLI レビューは、今は CodeRabbit CLI だけが担っている（`.claude/commands/_lib/coderabbit-local-review.md:5-11`）。この CLI のレビューは Claude 以外のモデルによるもので、PR を作る前に入る唯一の異系統のレビュー（Claude 以外のモデルによるレビュー。6 節で定義）でもある（issue #460）。

-   CLI が失敗したとき（認証、利用の上限、ネットワーク）、手順は再試行も停止もせず、残りを飛ばして失敗を SUMMARY に記録する（`.claude/commands/_lib/coderabbit-local-review.md:42-44`）。その範囲は、異系統のレビューを受けないまま PR に進む。
-   `/runplan` は、手順 7.7 が失敗したフェーズ群について `CodeRabbit local review: not run` をコミットのメッセージに残す（`.claude/commands/runplan.md:212`）。PR を開くときにその行があれば、ブランチ全体を改めてレビューする（`.claude/commands/runplan.md:84-90`）。この再レビューも CodeRabbit CLI なので、CodeRabbit CLI が使えない状態が続けば、やはり異系統のレビューは入らない。
-   PR レビューは Codex の GitHub 連携が、PR を作ったときに 1 回だけ行う（`CLAUDE.md:110-114`）。Codex の割り当ては利用者のほかの ChatGPT の利用と共有で、上限は公開されていない（`CLAUDE.md:110-111`）。2 回目以降の PR レビューは、エスカレーションの判断か利用者の依頼があったときだけ依頼する（`.claude/commands/_lib/pr-review-escalation.md:14-18`）。
-   ローカルの CLI レビューの共通手順を呼ぶのは 5 か所である。`/runplan` の手順 7.7（`.claude/commands/runplan.md:212`）と手順 5a のブランチ全体の再レビュー（`.claude/commands/runplan.md:84-90`）、`/runpr` の手順 1（`.claude/commands/runpr.md:115-119`）、`_lib/task-pr-handoff.md` の Part B 手順 1（`.claude/commands/_lib/task-pr-handoff.md:175-177`）、`/fixpr` の Phase 4.5（`.claude/commands/fixpr.md:581-590`）である。
-   `coderabbit review` は差分を外部のサービスに送るが、`CLAUDE.md` の「Tool Execution Safety」で承認なしに実行してよいとされている（`CLAUDE.md:540-542`）。Claude Code の権限の設定でも、`Bash(coderabbit review *)` が許可されている（`.claude/settings.json:4`）。`codex review` には、どちらの許可もない。
-   PR を開くとき、PR 本文にローカルの CLI レビューの `unclear` と `deferred` の指摘を載せるのは `_lib/task-pr-handoff.md` の Part B 手順 3 だけである（`.claude/commands/_lib/task-pr-handoff.md:199-204`）。`/runpr` の手順 1 の単独の変更の PR 本文は、ブランチのコミットから書き、ローカルの CLI レビューの指摘を載せない（`.claude/commands/runpr.md:134-147`）。
-   作業環境には `codex` CLI（0.163.0）があり、`codex review` で非対話のレビューを実行できる。範囲の指定には `--base <BRANCH>`（ブランチとの比較）、`--commit <SHA>`（1 つのコミット）、`--uncommitted`（コミットしていない変更）がある（`codex review --help`。2026-10-28 確認）。CodeRabbit CLI の `--base-commit` のように任意のコミットから HEAD までを指定できるかは、確かめていない。

**issue #460 で決めたこと:**

-   **CodeRabbit CLI は残す。** 廃止は対象外とする。CodeRabbit CLI の割り当ては PR レビューとは別であり（`.claude/commands/_lib/pr-review-escalation.md:10-12`）、ローカルの CLI レビューの主な担い手であり続けるためである。
-   **PR レビュー（Codex の GitHub 連携）は変えない。** 本タスクは PR を作る前と、PR を作った後の修正に対するローカルの CLI レビューだけを扱う。
-   **PR-Agent は導入しない。** issue #460 は、前提の誤りと、既存のサブエージェントのレビューとの重複を理由に見送っている。

**要件定義の際に決めたこと（2026-10-28）:**

-   **Codex CLI は、CodeRabbit CLI が失敗したときの代替に限る。** 常に併用はしない。issue の背景にある穴（CodeRabbit CLI が失敗した範囲が異系統のレビューを受けない）を塞ぐのに足り、Codex の割り当ての消費を失敗したときだけに抑えられるためである。
-   **設計の前に、試験運用で `codex review` を観測して記録する（F-001）。** 割り当てを PR レビューと共有するか、コミットの範囲を指定できるか、作業ツリーに書き込まずに動くか、出力をトリアージに取り込めるかは、どれも確かめていない。どれかが成り立たなければ設計が変わるため、0031 の試験運用（`docs/tasks/0031_codex_pr_review/01_requirements.md` の F-001）と同じく、観測を先に行う。観測では数回分の割り当てを使う。
-   **試験運用は、`/mkarch` の前に、このタスクの別の PR で行う（順序は 2.4）。** エージェントが `codex review` を実行し、結果を `observations.md` にまとめてその PR に入れる。`codex review` の実行は、1 回ごとに利用者の承認を得る。AC-16 が実装されるまでは、差分を外部に送る `codex review` は `CLAUDE.md` の「Tool Execution Safety」の承認の対象だからである。
-   **割り当てを PR レビューと共有すると分かっても、そのまま採用する。** 代替は CodeRabbit CLI が失敗したときだけ実行するので、PR レビューの割り当てへの影響は小さいと判断した。観測の結果は文書に残す（AC-18）。
-   **`codex review` は承認なしに実行してよいものとし、作業ツリーに書き込ませない。** `coderabbit review` と同じく、`CLAUDE.md` の「Tool Execution Safety」で承認なしの実行を許し、Claude Code の権限の確認でも止まらないようにする（AC-16・AC-17）。承認を求めると、無人で進む `/runplan` や `/fixpr` の途中で代替が止まるためである。書き込みを許さないのは、レビューの範囲や、手順が後で行うコミットの内容を Codex に変えさせないためである（AC-08）。
-   **呼び出し元の 5 か所すべてで代替を行う。** issue が挙げた 3 か所に加え、`/fixpr` の Phase 4.5 と `/runplan` の手順 5a も含める。どれも同じ共通手順を呼んでおり、PR を作った後の修正こそ PR レビューを受けないためである（`.claude/commands/fixpr.md:584-586`）。
-   **両方が失敗しても止めず、記録して先へ進む（Codex CLI が作業ツリーなどを変えたとき（AC-08）と、それに当たるかを確かめられないときを除く）。** 今の CodeRabbit CLI の失敗の扱い（`.claude/commands/_lib/coderabbit-local-review.md:42-44`）を変えない。そのうえで、異系統のレビューを受けていない範囲を PR 本文か報告に書く（AC-13〜AC-15）。無人の実行を止めないためである。
-   **回数の上限は、CodeRabbit CLI と同じく範囲ごとに 2 回とし、「試行」を単位に数える。** 各試行は CodeRabbit CLI の実行 1 回と、それが失敗したときの Codex CLI の実行 1 回から成る。2 回目の試行も CodeRabbit CLI から試す（AC-07）。代替が失敗した CodeRabbit CLI の 1 回を置き換えるだけにすれば、上限と収束の規則（`.claude/commands/_lib/coderabbit-local-review.md:71-77`、`.claude/commands/_lib/review-subagent-pattern.md` の Convergence）を代替のために変えずに済む。また、Codex CLI の指摘で直した修正も 2 回目の試行でレビューされる。
-   **Codex CLI の指摘は、原文を引用せずに記録する。** コミットのメッセージ、PR 本文、SUMMARY には、指摘の位置と、エージェントが自分の言葉で書いた要約だけを残す（AC-12）。Codex CLI の出力はレビュー対象の文面に影響されうるので、秘密の値や Codex へのメンションを含みうる。原文を写すと、それが GitHub に公開されるか、メンションの確認で PR の作成が止まるためである。
-   **PR 本文への記録は `_lib/task-pr-handoff.md` の Part B に限る。** `/runpr` の手順 1 の単独の変更では、今と同じく PR 本文にローカルの CLI レビューの指摘を載せず、最終報告に記録する（AC-14）。今 PR 本文に指摘を載せているのが Part B だけであり、単独の変更の PR 本文に新しい内容を加える理由が issue にないためである。
-   **`/runplan` のフェーズ群のレビューを Codex CLI で代替できたときは、PR を開くときのブランチ全体の再レビューを行わない（AC-15）。** 今は CodeRabbit CLI が失敗すると必ず再レビューする（`.claude/commands/runplan.md:84-90`）。代替が成功した範囲は、すでに異系統のレビューを受けているためである。
-   **秘密の環境変数は Codex に渡さない。git-ignore されている秘密のファイルを Codex が読んで送る経路は、残るリスクとして記録するだけにする。** `codex review` はレビュー中にシェルのコマンドを実行するエージェントである。環境変数を引き継ぐと、Codex が実行するどのコマンドも秘密の値を持ち、`env` の実行や環境を含むエラーの出力から、意図して読まなくても OpenAI に送られうる。そのため、Codex に渡す環境は必要な変数だけにする（4.2）。一方、書き込みを許さない設定でも、作業ツリーの外を含めて読み取りは許されうるので、`.envrc`・`.env`（`CLAUDE.md:458`）のファイルを読んで送る経路は残る。Claude Code 自身も同じ経路を持つため、この経路への対策は求めず、4.2 に残るリスクとして書く（2026-10-29 に改めた。Comments）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   CodeRabbit CLI が失敗した範囲も、Claude 以外のモデルのレビューを受けてから、PR に進むか、PR を作った後の修正として push する。
-   どの範囲がどの CLI のレビューを受けたか、どの範囲が異系統のレビューを受けていないかが、PR 本文と報告から分かる。

### 2.2. スコープ (In Scope)

-   試験運用での `codex review` の観測と記録（F-001）。
-   CodeRabbit CLI が失敗したときの、Codex CLI による代替とトリアージ（F-002）。対象は 1 節の 5 か所の呼び出し元である。
-   どの CLI のレビューを受けたかと、異系統のレビューを受けていない範囲の記録（F-003）。
-   `codex review` の実行の承認と、関係する文書の更新（F-004）。

### 2.3. スコープ外 (Out of Scope)

-   CodeRabbit CLI の廃止（issue #460）。
-   PR レビュー（Codex の GitHub 連携）の変更（issue #460）。PR レビューの依頼の回数と、エスカレーションの基準（`.claude/commands/_lib/pr-review-escalation.md` の Criteria）も変えない。
-   PR-Agent の導入（issue #460）。
-   CodeRabbit CLI が成功したときの Codex CLI の併用（1 節の決定）。
-   git-ignore されている秘密のファイルを Codex が読んで送る経路への対策（1 節の決定）。
-   `/runpr` の手順 1 の単独の変更の PR 本文の変更（1 節の決定）。
-   `/runplan` の手順 7 と 7.5 のサブエージェントのレビューの変更。

### 2.4. 作業の順序

本タスクは、試験運用という運用の作業を含む。試験運用を担うコマンドはないため、通常の要件 → 設計 → 計画 → 実装の流れに、次の段階を手作業で加える。

1.  本書の承認とマージ（`/runpr`）。このとき `/runpr` は次のコマンドとして `/mkarch` を提案するが、実行しない。
2.  試験運用（F-001）。新しいセッションで、本タスクの次の連番のブランチを作り、エージェントが `codex review` を実行して観測する。`codex review` の実行は、1 回ごとに利用者の承認を得る（1 節の決定）。観測の記録（`observations.md`）を PR にし、`/runpr` でマージする。観測の結果が AC-02 に当たれば、設計に進まず利用者に報告する。
3.  `observations.md` のマージの後に、設計（`/mkarch`）。試験運用の PR がマージされるまで `/mkarch` を実行しない。
4.  実装計画（`/mkplan`）と実装（`/runplan`）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 試験運用での観測と記録

F-002 から F-004 を設計する前に、`codex review` を実際に動かして観測し、タスクのディレクトリの `observations.md` に記録する。

**Acceptance Criteria**:
- **AC-01**: `observations.md` に、次の各項目について、観測の日付、使った `codex` CLI の版、実行したコマンド、結果が記録されている。判断や観測ができなかった項目は、できなかったことと、その理由を記録する。
    1. ローカルの `codex review` の実行が、Codex の PR レビューと同じ割り当てを消費するか。判断の根拠（何を見て判断したか）も記録する。
    2. あるコミット（BASE）から HEAD までのコミット済みの変更だけを範囲にしてレビューできるか。BASE がブランチの分岐点でない場合（`/fixpr` の Phase 4.5 の `<commitSha>^`、`/runplan` のフェーズ群の始まり）も含む。コミットしていない変更と、まだ git に追加していないファイルが範囲に入らないか。
    3. 作業ツリーに書き込ませない設定で実行できるか。実行の前後で、`git status --short` の出力、git-ignore されているファイル、`HEAD`、ブランチとタグの一覧、stash の一覧が変わらないか。
    4. 承認を求めて止まらずに、非対話で最後まで実行できるか。
    5. 出力から、各指摘の位置（ファイルと行）と内容を読み取れるか。あわせて、指摘がないときの出力の形、指摘以外に出る部分（要約、進捗の表示など）の形、出力の大きさ。
    6. 失敗したとき（認証がない、ネットワークに届かないなど、観測で起こせるもの）の終了コードと出力の形。
    7. レビュー中に Codex が読んだファイル（実行したシェルのコマンド）。差分の外のファイルを読んだかどうか。
    8. 1 回の実行にかかった時間。
- **AC-02**: AC-01 の 2・3・4・5 のどれかが成り立たないと分かったとき、または観測できなかったときは、F-002 から F-004 の設計に進まず、観測の結果を利用者に報告して、続けるかを利用者が決める。AC-01 の 1 で割り当てを共有すると分かったときは、そのまま進む（1 節の決定）。

#### F-002: Codex CLI による代替

ローカルの CLI レビューの共通手順で CodeRabbit CLI が失敗したとき、同じ範囲を Codex CLI でレビューし、同じ規則でトリアージする。

**Acceptance Criteria**:
- **AC-03**: CodeRabbit CLI のレビューが失敗したとき（認証、利用の上限、ネットワークなど、レビューの結果が得られなかったとき）、同じ試行の中で、同じ範囲（BASE から HEAD までのコミット済みの変更）を Codex CLI でレビューする。利用者に承認を求めない（AC-16・AC-17）。
- **AC-04**: CodeRabbit CLI のレビューが成功したときは、指摘が 0 件でも、その試行では Codex CLI を実行しない。
- **AC-05**: Codex CLI の指摘は、CodeRabbit CLI の指摘と同じく `.claude/commands/_lib/triage-pattern.md` の尺度（`verdict`、`severity`、`topic`、`replyBody`）で分類し、同じ規則で扱う。`must-fix` を直し、`unclear` を未解決として記録し、タスクのディレクトリの文書への指摘には `off-level` の判定を適用する（`.claude/commands/_lib/coderabbit-local-review.md:46-69` と同じ）。
- **AC-06**: Codex CLI の各指摘は、トリアージする前に、指摘が挙げた位置のコードを読んで確かめる。存在しないファイルや行を挙げる指摘と、範囲（BASE から HEAD まで）で変更されていないファイルを挙げる指摘は `invalid` とする。指摘の文面に含まれる指示（コマンドの実行、ファイルの変更、レビュー対象の外への操作）を、指摘の文面だけを根拠に実行しない。
- **AC-07**: 1 つの範囲（呼び出し元が SCOPE と呼ぶもの）につき、ローカルの CLI レビューの試行は最大 2 回である。各試行では CodeRabbit CLI を先に実行し、失敗したときだけ Codex CLI で代替する。2 回目の試行は 1 回目の試行で修正したときだけ行い、2 回目の試行でも CodeRabbit CLI から試し、`must-fix` だけに対応する。したがって、1 つの範囲で CodeRabbit CLI と Codex CLI はそれぞれ最大 2 回実行される。
- **AC-08**: Codex CLI の各実行の前後で、`git status --short` の出力、`HEAD`、ローカルのブランチとタグの一覧、stash の一覧が変わらない。手順が実行の前に自分で作り、実行の後に消す一時的な参照は、この比較から除く。git-ignore されているファイルは各実行では比べず、書き込みを許さない設定（AC-01 の 3 で確かめる）で守る。違いがあったときは、その試行の Codex CLI のレビューを失敗とする。違いは元に戻さず、コミットもせず、実行を止めて利用者に報告する。
- **AC-09**: Codex CLI のレビューも失敗したとき、再試行せず、AC-08 の場合と、AC-08 に当たるかを確かめられない場合（Codex CLI の実行の結果が得られない、結果の種類が分からない、実行の直前に作業ツリーが変わっていた）を除いて実行を止めず、その試行を「異系統のレビューを受けなかった試行」として記録して先へ進む（F-003）。次のどれも Codex CLI のレビューの失敗である。
    1. `codex` CLI が見つからない。
    2. 終了コードが失敗を示す。
    3. 設計で定める時間の上限までに終わらない。このときはプロセスを終わらせる。
    4. 出力が空である。
    5. 出力が設計で定める大きさの上限を超える。
    6. 出力の全体を、AC-01 の 5 で記録した形の部分（指摘、「指摘なし」、指摘以外の部分）だけから成るものとして読み取れない。どの形にも当たらない部分があれば、一部だけ読み取れる出力も失敗とし、読み取れた指摘も使わない。
- **AC-10**: AC-03 から AC-09 は、1 節の 5 か所の呼び出し元（`/runplan` の手順 7.7 と手順 5a、`/runpr` の手順 1、`_lib/task-pr-handoff.md` の Part B 手順 1、`/fixpr` の Phase 4.5）のどれから共通手順を呼んだときにも成り立つ。

#### F-003: 結果の記録

どの CLI のレビューを受けたか、どの範囲が異系統のレビューを受けていないかを、後のセッションと人のレビュアーが読める形で残す。

**Acceptance Criteria**:
- **AC-11**: ローカルの CLI レビューの各試行について、呼び出し元の SUMMARY（報告）が、その試行の結果（CodeRabbit CLI のレビュー、代替の Codex CLI のレビュー、異系統のレビューを受けなかった試行のいずれか）を示す。失敗した CLI ごとに、失敗の種類（認証、利用の上限、ネットワーク、時間の上限、読み取れない出力など、分かる範囲で）を示す。
- **AC-12**: コミットのメッセージに残すローカルの CLI レビューの記録（今の `CodeRabbit local review:` の行。`.claude/commands/_lib/coderabbit-local-review.md:79-87`）は、次を満たす。
    1. 却下・`deferred`・`unclear` の各指摘について、どちらの CLI の指摘かを示す。異系統のレビューを受けていない範囲があるときは、そのことと範囲を示す。
    2. 残す場所は今と同じである。修正のコミットがあればそのメッセージに、なければブランチの次のコミットのメッセージに残す。次のコミットがないときは SUMMARY にだけ残す（Part B では PR 本文。AC-13）。
    3. Codex CLI の指摘は、コミットのメッセージ、PR 本文、SUMMARY のどれにも原文を写さず、位置と、エージェントが自分の言葉で書いた要約だけを残す。その要約は Codex へのメンションを含まない。
- **AC-13**: `_lib/task-pr-handoff.md` の Part B 手順 3 で PR を開くとき、PR 本文は次を示す。
    1. Codex CLI の指摘のうち `unclear` と `deferred` のもの。CodeRabbit CLI の指摘と同じく、このセッションのものと、ブランチのコミットのメッセージに記録された前のセッションのものの両方である（`.claude/commands/_lib/task-pr-handoff.md:199-204`）。
    2. PR のコミットのうち異系統のレビューを受けていない範囲があれば、その範囲。
- **AC-14**: `/runpr` の手順 1 の単独の変更と、`/fixpr` の Phase 4.5 で、異系統のレビューを受けていない範囲が残ったときは、その最終報告（Final report）に、その範囲を示す。
- **AC-15**: `/runplan` のフェーズ群のレビューで CodeRabbit CLI が失敗し、Codex CLI で代替できたときは、PR を開くときにブランチ全体を改めてレビューしない。異系統のレビューを受けていないフェーズ群があるときだけ、今の `CodeRabbit local review: not run` の行と同じく、PR を開くときにブランチ全体を改めてレビューする（`.claude/commands/runplan.md:84-90`）。この再レビューが CodeRabbit CLI か Codex CLI で成功したときは、ブランチ全体が異系統のレビューを受けたものとし、AC-13 の 2 で示さない。

#### F-004: 承認と文書

`codex review` の扱いと、代替の存在を、プロジェクトの規則とコマンドの文書に反映する。

**Acceptance Criteria**:
- **AC-16**: `CLAUDE.md` の「Tool Execution Safety」が、`codex review`（ローカルの CLI レビューの代替としての Codex CLI）を承認なしに実行してよいものとして挙げ、その実行が差分と Codex の読んだ内容を OpenAI のサービスに送ることを述べている。共通手順の「Authorization」と、`codex review` を実行しうるコマンドの承認の記述（`.claude/commands/runpr.md:82-83`、`.claude/commands/fixpr.md:97`、`.claude/commands/_lib/task-pr-handoff.md:65-66` と同じ種類の記述）も、`codex review` を承認なしに実行してよいものとして挙げている。
- **AC-17**: 共通手順が使う形の `codex review` の実行は、Claude Code の権限の確認で止まらない。
- **AC-18**: `CLAUDE.md`、`.claude/commands/` の下の文書、`.coderabbit.yaml` のうち、ローカルの CLI レビューを CodeRabbit CLI だけが行うと述べる記述と、ローカルの CLI レビューの割り当てが PR レビューと別だと述べる記述が、すべて改められている。改めた記述は、CodeRabbit CLI が失敗したときに Codex CLI で代替することを述べ、割り当てについては AC-01 の 1 の観測の結果（共有する、しない、判断できなかった）と一致している。共通手順の呼び出し元を列挙する記述は、5 か所の呼び出し元と一致している。今ある該当の記述は次のとおりである。
    1. CodeRabbit CLI だけが行う、または割り当てが別だと述べる記述: `CLAUDE.md:115-117`、`.claude/commands/_lib/pr-review-escalation.md:10-12`、`.claude/commands/fixpr.md:584-586`、`.claude/commands/_lib/coderabbit-local-review.md:5-11`、`.claude/commands/runplan.md:212`、`.coderabbit.yaml:9-12`。
    2. 呼び出し元を列挙する記述: `.claude/commands/_lib/coderabbit-local-review.md:7-9`、`.claude/commands/_lib/triage-pattern.md:7`。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。1 回の実行にかかる時間は観測で記録する（AC-01 の 8）。代替は CodeRabbit CLI が失敗したときだけ実行するので、CodeRabbit CLI が成功する実行の時間は変わらない（AC-04）。Codex CLI の時間の上限は設計で決める（AC-09 の 3）。

### 4.2. セキュリティ (Security)

本タスクは、ネットワークへの送信（差分と、Codex が読んだ内容を OpenAI のサービスに送る）と、信頼できない入力（Codex CLI の出力）の扱いに関わる。`docs/dev/security.md` の観点から、次を要件とする。

-   **レビューの対象とする範囲:** Codex CLI に渡すレビューの範囲は、CodeRabbit CLI と同じく、BASE から HEAD までのコミット済みの変更だけである（AC-03）。コミットしていない変更と、まだ git に追加していないファイルを範囲に含めない（AC-01 の 2）。ただし、実際に OpenAI に送られる内容は、この範囲に限らない（次の項目）。秘密（`DEEPSEEK_API_KEY`・`ANTHROPIC_API_KEY`・`SLACK_WEBHOOK_URL`）をコミットしない規則（`CLAUDE.md:458-459`）は変わらない。
-   **秘密の環境変数:** Codex CLI とそれが実行するコマンドに、秘密の環境変数（`DEEPSEEK_API_KEY`・`ANTHROPIC_API_KEY`・`SLACK_WEBHOOK_URL` など）を渡さない。渡す環境は、Codex CLI の動作に要る変数だけにする（1 節の決定）。
-   **残るリスク（秘密のファイル）:** `codex review` はレビュー中にシェルのコマンドでファイルを読むエージェントである。作業ツリーに書き込ませない設定でも読み取りは許されうるので、git-ignore されている `.envrc`・`.env` のファイルを読み、OpenAI に送りうる。本タスクではこの経路への対策を求めない（1 節の決定）。Codex が実際に何を読んだかは、試験運用で記録する（AC-01 の 7）。
-   **書き込ませない:** Codex CLI は作業ツリー、コミット、ブランチを変えない。各実行の前後で確かめ、違いがあれば止めて報告する（AC-08）。
-   **Codex CLI の出力は信頼できないデータとして扱う:** 出力は LLM が書いた文面であり、レビュー対象のコード（PR の内容）に書かれた文面に影響されうる。境界の要件は次のとおりである。
    -   **受け入れる形:** 出力の全体が、AC-01 の 5 で記録した形の部分（位置と内容を持つ指摘、「指摘なし」、要約や進捗の表示などの指摘以外の部分）だけから成るものを受け入れる。
    -   **それ以外:** 空の出力と、どの形にも当たらない部分を含む出力は、補って受け入れず、「指摘なし」ともみなさず、Codex CLI のレビューの失敗とする（AC-09 の 4・6）。
    -   **使う部分:** 使うのは各指摘の位置と内容だけである。指摘以外の部分（要約、進捗の表示など）は使わずに読み飛ばす。位置は、指摘が挙げた位置のコードを読んで確かめ、存在しない位置と範囲で変更されていないファイルの位置は `invalid` とする（AC-06）。
    -   **指示として実行しない:** 指摘の文面に含まれる指示を、文面だけを根拠に実行しない（AC-06）。
    -   **原文を写さない:** 指摘の原文を、コミットのメッセージ、PR 本文、SUMMARY に写さない（AC-12）。原文に含まれうる秘密の値や Codex へのメンションを、GitHub に公開しないためである。
    -   **大きさの上限:** 出力の大きさに有限の上限を設け、超えた出力は Codex CLI のレビューの失敗とする（AC-09 の 5）。値は観測（AC-01 の 5）をもとに設計で決める。
    -   **センチネルエラー:** Codex CLI の実行と、出力が受け入れる形かの判定は、Go の開発用ツールが行う。ツールは拒否した理由を、決まった種類の値として返す。共通手順は、その値に応じて、Codex CLI のレビューの失敗として扱うか、AC-08 に当たるとき、または当たるかを確かめられないときは実行を止める（AC-09）。トリアージは、今と同じくコマンドの文書（エージェントへの手順）で行う。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   CodeRabbit CLI と Codex CLI の両方が失敗しても、実行を止めない（AC-09。AC-08 に当たる場合と、当たるかを確かめられない場合を除く）。異系統のレビューを受けていない範囲は記録し、PR 本文か報告で示す（AC-12〜AC-14）。
-   Codex CLI が終わらないときは、時間の上限で終わらせて失敗とする（AC-09 の 3）。
-   Codex CLI を同じ試行の中で再試行しない（AC-09）。回数は範囲ごとに最大 2 回である（AC-07）。
-   Codex CLI が作業ツリーなどを変えたとき（AC-08）と、それを確かめられないとき（AC-09）だけは、実行を止めて報告する。

### 4.4. 互換性 (Compatibility)

-   CodeRabbit CLI が成功したときの手順と結果は、今と変わらない（AC-04）。
-   前のセッションがコミットのメッセージに残した `CodeRabbit local review:` の行は、引き続き PR 本文に取り込まれる（AC-13 の 1）。`CodeRabbit local review: not run` の行も、引き続きブランチ全体の再レビューのきっかけになる（AC-15）。
-   PR レビュー（Codex の GitHub 連携）の依頼と、レビュー済みの判定は変えない（2.3）。

### 4.5. 保守性 (Maintainability)

-   代替の手順は、ローカルの CLI レビューの共通手順の中に 1 つだけ置き、5 か所の呼び出し元はそれを呼ぶ（AC-10）。呼び出し元ごとに代替の手順を書かない。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   F-002 から F-004 は、F-001 の観測の結果に基づいて設計する（AC-02）。試験運用は `/mkarch` の前に、このタスクの別の PR で行う（1 節の決定、2.4）。
-   Codex CLI には ChatGPT のプランへのログインが必要である。ログインと `codex` CLI のインストールは、リポジトリの外の設定であり、利用者が行う。
-   Codex の割り当ては、利用者のほかの ChatGPT の利用や Codex の PR レビューと共有されうる（`CLAUDE.md:110-111`）。
-   ネットワークに触れる外部のツールは、`CLAUDE.md` の「Tool Execution Safety」が承認したものだけを承認なしに実行する（AC-16）。

## 6. 用語集 (Glossary)

-   **ローカルの CLI レビュー**: コミット済みの変更を、外部のサービスのレビュー用 CLI でローカルからレビューすること（`.claude/commands/_lib/coderabbit-local-review.md`）。本タスクの前は CodeRabbit CLI だけが行い、本タスクで Codex CLI による代替を加える。
-   **試行**: ローカルの CLI レビューを 1 回試みること。CodeRabbit CLI の実行 1 回と、それが失敗したときの Codex CLI の実行 1 回から成る（AC-07）。
-   **代替**: CodeRabbit CLI が失敗したときに、同じ試行の中で同じ範囲を Codex CLI でレビューすること（F-002）。
-   **異系統のレビュー**: Claude 以外のモデルによるレビュー。CodeRabbit CLI と Codex CLI のレビューを指す。`/runplan` のサブエージェントのレビューは含まない。
-   **異系統のレビューを受けていない範囲**: 範囲のコミットのうち、CodeRabbit CLI か Codex CLI のレビューが成功した試行の範囲に含まれないもの。たとえば、1 回目が成功して修正し、2 回目で両方が失敗したときの修正のコミットである。後でその範囲を含むレビュー（`/runplan` の手順 5a のブランチ全体の再レビューなど）が成功すれば、その範囲は異系統のレビューを受けていない範囲ではなくなる。
-   **範囲（SCOPE）**: 呼び出し元がローカルの CLI レビューに渡す、BASE から HEAD までのコミット（フェーズ群、ブランチ、`/fixpr` の修正のラウンド）。
-   **フェーズ群**: `/runplan` が 1 回の実行で実装するフェーズのまとまり（phase group）。
-   **PR レビュー**: 自動レビュアーが GitHub の PR 上で行うレビュー。ローカルの CLI レビューと区別する。
-   **割り当て**: `docs/translation_glossary.md` の「割り当て」（quota）と同じく、一定時間に受け付ける利用の回数の上限。本書では、自動レビュアーの PR レビューに加えて、Codex CLI と CodeRabbit CLI の利用の上限にも使う。「枠」（slot）は localhost サーバの同時処理数の単位であり、別の概念なので本書では使わない。
-   **試験運用**: F-001 で、設計の前に `codex review` を実際に動かして観測すること。
-   **指摘**: レビューが挙げる個々の問題。
-   用語は `docs/translation_glossary.md` と統一する。
