# 要件定義書：fixprguard で複数の自動レビュアー（CodeRabbit と Codex）を信頼できるようにする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-21 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`fixprguard` が信頼する自動レビュアーを、1 つの名前から名前の一覧に広げるための要件を定義する（[#405](https://github.com/isseis/yt2column/issues/405)）。CodeRabbit（`coderabbitai`）と Codex（`chatgpt-codex-connector`）の両方のスレッドを、名指しなしで信頼する著者のスレッドとして処理できるようにする。あわせて、`/runpr` がレビュー済みの判定に使う Bot（待つ PR レビュアー）を、信頼の一覧とは別の項目に分ける。

**背景:**

タスク 0030（[#395](https://github.com/isseis/yt2column/issues/395)）は、信頼する自動レビュアーを `_context.md` の名前 1 つで定めた（`docs/tasks/0030_fixpr_untrusted_threads/01_requirements.md` の AC-01・AC-03・AC-35）。今の値は `coderabbitai` である（`.claude/commands/_context.md:185`）。`fixprguard` はこの名前を `--reviewer` で 1 つだけ受け取る。同じフラグを 2 回指定すると、使い方のエラーになる（`cmd/fixprguard/main_test.go:97`）。信頼の判定は、著者の種類が `Bot` で、`login` がその 1 つの名前と一致するときだけ信頼する（`internal/fixprguard/trust.go:84-88`）。名前の違う `Bot` は、理由 (b)（`other-bot`）で信頼しない。

タスク 0031（[#403](https://github.com/isseis/yt2column/issues/403)）では、PR レビューを Codex の GitHub 連携に切り替える。Codex のスレッドの最初のコメントの著者は、GraphQL では `__typename: Bot`・`login: chatgpt-codex-connector` だった。REST では `type: Bot`・`login: chatgpt-codex-connector[bot]` だった（`docs/tasks/0031_codex_pr_review/observations.md` の項目 3。2026-10-21、PR #407）。今の設計のままでは、Codex のスレッドはすべて理由 (b) で信頼されず、名指しでしか処理できない。

一方、`/runpr` の step 3 は、自動レビュアーの名前に `[bot]` を付けて REST の名前を作る。その名前で CodeRabbit の PR レビューと要約コメントを絞り込み、レビュー済みかを判定する（`.claude/commands/runpr.md:216-224`）。名前を一覧にすると、どの名前に `[bot]` を付けるかが決まらなくなる。そのため、信頼の一覧と、レビュー済みの判定に使う名前を分ける必要がある（issue #405 の「やること」の 4 つめ。0031 の要件定義書の AC-25）。

前提の #395 は完了している。PR-6（[#421](https://github.com/isseis/yt2column/pull/421)）はマージ済みで、`fixpr.md` と `runpr.md` は `fixprguard` を呼ぶ（`docs/tasks/0030_fixpr_untrusted_threads/03_implementation_plan.md:460`）。Codex の著者の値の観測（#403 の未確認項目 3）も記録済みである（上記）。

対応 issue: [#405](https://github.com/isseis/yt2column/issues/405)（前提: [#395](https://github.com/isseis/yt2column/issues/395)（タスク `0030_fixpr_untrusted_threads`）。関連: [#403](https://github.com/isseis/yt2column/issues/403)（タスク `0031_codex_pr_review`）。依存関係は #395 → #405 → #403 である）

**本書の記述範囲:** 本書は、観測できる振る舞いを定める。具体的には、`_context.md` に何を置くか、`fixprguard` が何を受け付けて、どう判定するか、コマンドの文書が何を渡すか、である。次の事項は設計（`02_architecture.md`）で決める。

-   `_context.md` の一覧の書き方（表の行の形、名前の区切り）。
-   名前の一覧を持つ Go の型、`NewGitHub` の引数の形、`protocol` の番号を上げるかどうか。
-   ツールと文書の変更を PR にどう分けるか。

要件の起草で挙がった設計の事項は、[design_handoff.md](design_handoff.md) に申し送る。

**issue #405 で決めたこと:**

-   **一覧の名前は GraphQL の形にする:** `_context.md` の自動レビュアーを名前の一覧にし、各名前は GraphQL の `author.login` の形で書く。`fixprguard` は GraphQL の `author` で判定するためである（0030 の `02_architecture.md` 3.3）。
-   **`--reviewer` を繰り返し指定できる形にする:** 判定の表の「`Bot` で `login` が自動レビュアーの名前と一致」を、「一覧のどれかと一致」に変える。名前の一覧を 1 つの値に詰めず、フラグを繰り返す形である。
-   **一覧が空なら止める:** `--reviewer` が 1 つもないときは、今の「`--reviewer` がない」と同じく、起動時に止める。
-   **REST の形の規則を確かめる:** REST の名前を `<名前>[bot]` で作る規則（0030 の design_handoff.md の H-02）が、Codex でも成り立つかを確かめる。確かめた結果、成り立つ（`observations.md` の項目 3）。そのため、規則を変えずに拒否する形は要らない。
-   **信頼の一覧と待つ PR レビュアーを分ける:** `runpr.md` step 3 のレビュー済みの判定は、どの Bot のレビューを待つかを 1 つに決める必要があり、信頼の一覧とは役割が違う。判定方法の変更そのものは #403（0031）の範囲である。
-   **判定の表に行を加える:** 名前が 2 つあるとき、片方だけに一致する `Bot`、どちらにも一致しない `Bot`、名前が一致する `User` を判定の表のテストに加え、文書のガードのテスト（0030 の `02_architecture.md` 7.4 の (3)・(7)）を、複数の名前に合わせる。
-   **完了条件:** CodeRabbit と Codex の両方のスレッドが、名指しなしで信頼する著者のスレッドと判定される。一覧にない `Bot` と、名前が一致する `User`（権限の照会で判定する）は、#395 と同じく信頼されない。`make test`・`make lint` と `doc-guard` が通る。

**要件定義の際に決めたこと（2026-10-21）:**

-   **待つ PR レビュアーの項目は本タスクで作る:** `_context.md` に、信頼の一覧とは別の「待つ PR レビュアー」の項目を作る。値は今と同じ `coderabbitai` にする。`runpr.md` step 3 のレビュー済みの判定は、この項目から REST の名前を作る（F-003）。名前を一覧にすると、step 3 が使う 1 つの名前が一覧から決まらなくなるためである。一覧の先頭の名前を使うなどの暫定の扱いは、一覧の順序に意味を持たせることになるので採らない。0031 は、この項目の値と判定の方法を変えるだけで済む（0031 の AC-25 に合う）。
-   **同じ名前の重複は拒否する:** `--reviewer` に同じ名前を 2 回以上渡すと、使い方のエラーで止める（AC-07）。`_context.md` の一覧に同じ名前が 2 回以上あると、文書のガードが失敗する（AC-02）。重複を 1 つにまとめて受け付けると、書き間違い（たとえば、別の名前を書くつもりで同じ名前を写したもの）が表に出ないためである（`CLAUDE.md` の「Reject, don't normalize」）。
-   **外部の人が Codex の指摘を誘導できる点は、残るリスクとして記録する:** Codex のアカウントを接続している外部のアカウントは、`@codex review` で、その人の割り当てで Codex を動かせると考えられる（`observations.md` の項目 8。確かめてはいない）。依頼のコメントでレビューの観点を指示すれば、指摘の内容を誘導できうる。その指摘は信頼する Bot のスレッドとして届く。このリスクを 4.2 に記録して受け入れる。スレッドの本文は区切りのブロックでデータとして渡され、変更の範囲の確認と、マージ前の人のレビューが残るためである。CodeRabbit を信頼するときも、同じ構造である。確かめるには Codex の契約を持つ別のアカウントが要り、費用に見合わない（0031 の要件定義書の 1 節と同じ判断）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   CodeRabbit と Codex の両方のスレッドを、名指しなしで信頼する著者のスレッドとして処理できるようにする。
-   一覧にない `Bot` と、名前が一致する `User` は、#395 と同じく信頼しないままにする。
-   `/runpr` のレビュー済みの判定に使う名前を、信頼の一覧から切り離す。

### 2.2. スコープ (In Scope)

-   `_context.md` の信頼する自動レビュアーの一覧と、待つ PR レビュアーの項目（F-001）。
-   `fixprguard` の `--reviewer` を繰り返し指定できる形にすることと、信頼の判定（F-002）。
-   コマンドの文書（`fixpr.md`・`runpr.md`）、`fixprguard_guide.md`、`docs/translation_glossary.md`、文書のガードのテスト（F-003）。

### 2.3. スコープ外 (Out of Scope)

-   `runpr.md` step 3 のレビュー済みの判定の方法を Codex 用に変えること、`@coderabbitai review` の依頼を `@codex review` に変えること、待つ PR レビュアーの値を Codex に変えること。#403（0031）で行う（issue #405 の「やること」の 4 つめ）。
-   一覧から `coderabbitai` を外すこと。issue #405 の完了条件が両方の信頼を求めているためである。CodeRabbit の PR レビューを止めた後（0031 の F-004）に外すかは、本タスクでは決めない。
-   `--reviewer` の値が `_context.md` の一覧と一致することを、ツールが確かめること。0030 の設計は、ツールに `_context.md` を読ませないと決めている（`docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md` 5.3 の「`--reviewer` の値」）。本タスクでも変えない。
-   Bot をノード ID で特定すること。0030 の設計が、`_context.md` の値を増やすので採らなかった（同 5.3 の「自動レビュアーの同一性」）。本タスクでも変えない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 信頼の一覧と待つ PR レビュアー

`_context.md` に、信頼する自動レビュアーの名前の一覧と、待つ PR レビュアーを、別々の項目として定める。

**Acceptance Criteria**:
- **AC-01**: `.claude/commands/_context.md` に、信頼する自動レビュアーの名前の一覧が 1 か所で定められている。各名前は GraphQL の `author.login` の形（`[bot]` を付けない形）である。一覧は `coderabbitai` と `chatgpt-codex-connector` の 2 つの名前を持つ。
- **AC-02**: 次のいずれかに当たるとき、文書のガードのテストが失敗する。一覧が空である。一覧の名前が `login` の文法（`docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md` 3.3。英数字とハイフンの並びで、先頭は英数字）に合わない。一覧に同じ名前が 2 回以上ある。
- **AC-03**: `.claude/commands/_context.md` に、待つ PR レビュアーが、信頼の一覧とは別の項目として 1 か所で定められている。値は 1 つの名前で、GraphQL の `author.login` の形の `coderabbitai` である。その項目は、REST の `user.login` の形がその名前に `[bot]` を付けたものであることを述べる。待つ PR レビュアーの名前が `login` の文法に合わないか、信頼の一覧にないとき、文書のガードのテストが失敗する。

#### F-002: `fixprguard` の複数の `--reviewer`

`fixprguard` の `list`・`begin`・`inspect`・`reply` が、`--reviewer` を 1 回以上受け取り、そのどれかと一致する `Bot` を信頼する。

**Acceptance Criteria**:
- **AC-04**: `list`・`begin`・`inspect`・`reply` は、`--reviewer` を 1 回以上受け付ける。各値が 1 つの名前である。`--reviewer` を 1 回だけ渡す呼び出しは、今と同じ結果になる。
- **AC-05**: スレッドのコメントの著者の種類が `Bot` で、`login` が渡された名前のどれかと一致するとき、その著者は信頼する著者であり、権限の照会をしない。一致は、大文字と小文字を区別した文字列の完全な一致で判定する。`[bot]` の付け外しなどの変換はしない。名前が 2 つのとき、片方の名前の `Bot` も、もう片方の名前の `Bot` も、それぞれ信頼する著者になる。両方の名前の `Bot` が混ざったスレッドも、ほかの条件（0030 の AC-04〜AC-06）を満たせば、信頼する著者のスレッドになる。
- **AC-06**: 著者の種類が `Bot` で、`login` が渡されたどの名前とも一致しないとき、その著者は信頼する著者ではない。権限の照会はせず、理由は (b)（`other-bot`。アカウントの名前を付ける）である（0030 の AC-03・AC-11 と同じ）。著者の種類が `User` で、`login` が渡された名前のどれかと一致するときは、自動レビュアーとして扱わず、権限の照会で判定する（0030 の AC-02・AC-03 と同じ）。
- **AC-07**: 次のとき、`fixprguard` は GitHub にも git にも触れずに、使い方のエラー（終了コード `2`、標準エラーに 1 行の診断）で止まる。`--reviewer` を 1 つも渡さない（今と同じ）。同じ名前を 2 回以上渡す。値が `login` の文法に合わない（今と同じ）。
- **AC-08**: `begin` が出力する、処理するスレッドの `authorType` は、最初のコメントの著者が `Bot` で、`login` が渡された名前のどれかと一致するときだけ `Bot` である。それ以外（名指しされた、一覧にない `Bot` のスレッドを含む）は `User` である。そのため、`/fixpr` のラウンド上限（`fixpr.md:94-101`）は、一覧のどの自動レビュアーが開いたスレッドにも同じく働く。
- **AC-09**: `--reviewer` を受け付けないサブコマンド（`head`・`fence`・`finish`・`push`・`edit-pr`）は、今と同じく `--reviewer` を使い方のエラーで拒否する。

#### F-003: 文書

コマンドの文書が一覧のすべての名前を渡し、待つ PR レビュアーを参照するようにする。ガイドと用語集を、複数の名前に合わせる。

**Acceptance Criteria**:
- **AC-10**: `fixpr.md` と `runpr.md` は、`--reviewer` を取る `fixprguard` のすべての呼び出しで、`_context.md` の信頼の一覧のすべての名前を、1 つずつ `--reviewer` で渡すと定める。名前そのものは書かず、`_context.md` の項目を参照する。
- **AC-11**: `runpr.md` step 3 のレビュー済みの判定は、`_context.md` の待つ PR レビュアーの REST の形で、PR レビューと PR のコメントを絞り込む。判定の結果は今と同じである（CodeRabbit のレビューを待つ）。`runpr.md` と `fixpr.md` は、信頼の一覧の名前と待つ PR レビュアーの名前を、レビューの依頼のコメント（`@<待つ PR レビュアー> review`）の中でしか書かない。
- **AC-12**: `fixpr.md` の信頼する著者の説明（「Untrusted threads」の節）とラウンド上限の説明は、一覧のどれかの自動レビュアーを指す書き方になっている。`fixprguard_guide.md` は、`--reviewer` を繰り返し指定できること、1 つもないときと同じ名前を 2 回以上渡したときに使い方のエラーになること、`other-bot` が一覧にない `Bot` であることを述べる。
- **AC-13**: `docs/translation_glossary.md` の「信頼する著者」の定義が、自動レビュアーの一覧を指す。「自動レビュアーの一覧」と「待つ PR レビュアー」が載っている。
- **AC-14**: `make test`・`make lint` と、CI の `doc-guard` のジョブが通る。

### 3.2. 信頼できない入力の境界 (Untrusted Input Boundaries)

本タスクが変える境界は、GitHub API が返すスレッドのコメントの著者である。応答の受け入れの形は 0030 の要件のまま変えない（`docs/tasks/0030_fixpr_untrusted_threads/01_requirements.md`。同 `02_architecture.md` 3.3・3.4）。

-   **受け入れる形:** 著者の種類（`__typename`）と名前（`login`）は、0030 と同じ形で受け入れる。名前は、渡された一覧と完全に一致したときだけ自動レビュアーとする（AC-05）。大文字と小文字の違い、前後の空白、`[bot]` の有無は、補って一致とせず、一致しないものとして扱う。
-   **拒否の扱い:** 自動レビュアーと一致しない `Bot` は、エラーではなく、信頼しない理由 (b) になる（AC-06）。センチネルエラーは使わない。0030 と同じく、信頼の判定は理由の一覧で表すためである。
-   **使うメンバー:** 0030 と同じく、著者の `__typename` と `login` である。本タスクで使うメンバーは増えない。
-   **上限:** 本タスクで新しい上限は設けない。照会の上限（1 回の実行で 100 回）は 0030 のまま変えない。

`--reviewer` の値は、GitHub の応答ではなく、コマンドの文書から呼び出し元が渡す値である。値の検査（文法、重複、1 つ以上）は AC-07 で定める。拒否は使い方のエラー（終了コード `2`）で表す。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。比べる名前が 1 つから 2 つになるだけで、GitHub への問い合わせの回数は変わらない。

### 4.2. セキュリティ (Security)

本タスクは、信頼できないテキスト（スレッドの本文）のうち、どれを修正のフェーズのプロンプトに渡すかを決める判定を変える。`docs/dev/security.md` の観点から、次を要件とする。

-   **信頼を広げるのは一覧の名前だけ:** 信頼する著者に加わるのは、一覧の名前と一致する `Bot` だけである（AC-05）。一覧にない `Bot`、名前が一致する `User`、著者を確かめられないコメントの扱いは、0030 と同じである（AC-06）。
-   **本文の扱いは変えない:** 信頼する著者のスレッドの本文も、0030 と同じく区切りのブロックでデータとして渡す。変更の範囲の確認も同じく働く。Codex のスレッドの本文も、信頼できない入力として扱う（0031 の要件定義書の 4.2）。
-   **残るリスク（外部の人による誘導）:** Codex のアカウントを接続している外部のアカウントが `@codex review` で Codex を動かし、依頼のコメントで指摘の内容を誘導できうる（1 節の決定）。その指摘は、信頼する Bot のスレッドとして処理される。修正は変更の範囲の確認の中に限られ、マージの前に人がレビューする。
-   **残るリスク（`--reviewer` の値）:** ツールは `--reviewer` の値が `_context.md` の一覧と一致するかを確かめない（2.3）。セッションが一覧にない名前を渡すと、その Bot が信頼される。値が 1 つから一覧に変わっても、このリスクの性質は変わらない。文書のガード（AC-10）が、文書が `_context.md` の値を渡す記述を持つことを確かめるにとどまる。
-   **秘密:** 本タスクは秘密を扱わない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   ツールが古いまま（`make install-fixprguard` を実行していない）文書が 2 つの `--reviewer` を渡すと、古いツールは使い方のエラーで止まる（`cmd/fixprguard/main_test.go:97`）。誤って判定せずに止まるので安全側だが、再インストールまで `/fixpr` と `/runpr` が使えない。ツールと文書の変更をどの順で入れるかは、設計と実装計画で決める（design_handoff.md の H-01）。

### 4.4. 互換性 (Compatibility)

-   `--reviewer` を 1 回だけ渡す呼び出しは、今と同じ結果になる（AC-04）。
-   `fixprguard` の出力の形（`list` の `findings`、`begin` の `authorType` など）は変えない。
-   `runpr.md` step 3 のレビュー済みの判定の結果は変えない（AC-11）。

### 4.5. 保守性 (Maintainability)

-   信頼の一覧と待つ PR レビュアーは、それぞれ `_context.md` の 1 か所で定める（AC-01・AC-03）。自動レビュアーを加えたり外したりするときは、一覧を変える。PR レビューを行う Bot を替えるときは、待つ PR レビュアーを変える。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   #395（タスク 0030）の完了を前提にする。完了している（1 節）。
-   ツールは `_context.md` を読まない（0030 の `02_architecture.md` 1.2・5.3）。
-   ツールを変える PR がマージされた後は、`make install-fixprguard` で再インストールが要る（`CLAUDE.md` の「Build Commands」）。

## 6. 用語集 (Glossary)

-   **自動レビュアー**: PR のスレッドを開く Bot のうち、信頼する著者として扱うもの。
-   **自動レビュアーの一覧**: `_context.md` に定める、自動レビュアーの名前（GraphQL の `author.login` の形）の一覧（AC-01）。`fixprguard` には `--reviewer` で 1 つずつ渡す。英語では list of automated reviewers。
-   **待つ PR レビュアー**: `/runpr` がレビュー済みの判定に使う Bot（AC-03）。自動レビュアーの一覧とは別に定める（0031 の要件定義書の 6 節と同じ用語）。英語では awaited PR reviewer。
-   **信頼する著者**: リポジトリへの書き込み権限を持つ、Bot でないアカウントと、自動レビュアーの一覧のどれかの名前の Bot のアカウント（`docs/translation_glossary.md` の定義を、本タスクで一覧に合わせる。AC-13）。
-   用語は `docs/translation_glossary.md` と統一する。
