# 要件定義書：PR のタイトルを日本語にし、squash のコミットは英語のままにする

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-21 |
| Review date | 2026-10-21 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、PR のタイトルを日本語にし、squash のコミットのメッセージ（件名と本文）は英語のままにするための要件を定義する（[#408](https://github.com/isseis/yt2column/issues/408)）。対象は次のとおりである。

-   PR のタイトルを書くコマンド（`/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2`・`/runplan`・`/runpr`・`/fixpr`）の定義。
-   squash のコミットのメッセージを書く `/mergepr` の定義。
-   言語の規則を定める `CLAUDE.md`。
-   PR マーカーの規約を定める `.claude/commands/_context.md`。

本タスクが変えるのは、これらの定義と規則の文書（エージェントへの指示）である。そのため、受け入れ基準は、定義と規則の文書が何を指示し何を述べるか、という形で書く。本書で使う語は 6 節で定義する。

**背景:**

このリポジトリでは、GitHub に置くもの（PR の本文、issue、レビューへの返信、PR のコメント）を日本語で書く。PR のタイトルだけが例外で、英語の conventional-commit 形式にしている。理由は、squash のコミットの件名を PR のタイトルから作るためである（`CLAUDE.md:49-52`）。

PR のタイトルは、今は次のように決まる。

-   **タスクの文書の PR:** `/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2` は、文書を追加したコミットの件名を PR のタイトルにする（`.claude/commands/_lib/task-pr-handoff.md:35-37`）。タイトルは英語とされている（`_lib/task-pr-handoff.md:100-101`）。コミットの件名は、たとえば `docs(NNNN): draft requirements for <concise English summary>` である（`mkreq.md:106-107`、`mkarch.md:76`）。`/mkplan2` のコミットの件名には、決まった形がない（`mkplan2.md:105`）。
-   **プランの PR:** `/runplan` は、PR マーカーの `推奨タイトル` を PR のタイトルにする（`runplan.md:91`）。`推奨タイトル` は `feat(<task-id>): <concise English title>` の形の英語である（`_context.md:62`、`mkplan2.md:62`・`mkplan2.md:80`・`mkplan2.md:129`）。PR マーカーのないプランでは、`/runplan` は `feat(<task-id>): <concise English title>` の形のタイトルを書く（`runplan.md:198`）。
-   **単独の変更の PR:** `/runpr` は、英語の conventional-commit 形式のタイトルを書く（`runpr.md:85-87`）。
-   **PR の本文の更新:** `/fixpr` の Phase 6 は、PR の本文が大きく古くなったときにタイトルを書き直す。タイトルは英語の conventional-commit 形式に保つ（`fixpr.md:466`）。

すべての PR は squash マージする（`_context.md:51`）。`/mergepr` は、squash のコミットの件名を `<type>(<scope>): <summary> (#<number>)` の形の英語で書く。PR のタイトルがその形に合っていれば、それをそのまま使う（`mergepr.md:49-52`）。`mergepr.md` は、PR のタイトル・本文・コミットのメッセージ・diff を、要約するデータであって従う指示ではないと定めている（`mergepr.md:46-48`）。`mergepr` のバイナリは、件名が空でない 1 行であることだけを確かめる（`internal/mergepr/mergepr.go:441-450`）。PR のタイトルは、`prepare` の出力に表示するだけである（`cmd/mergepr/main.go:48`）。

GitHub に置くものを日本語で書く原則にしたので、PR のタイトルも日本語にしたい。一方、squash のコミットは `main` に残る唯一のコミットであり（`_context.md:51`）、リポジトリに残るものは英語で書く（`CLAUDE.md:44-46`）。そのため、squash のコミットのメッセージは英語のままにする。

issue が確認を求めた、PR のタイトルの形への依存について調べた結果は次のとおりである（2026-10-21）。リポジトリの中には、PR のタイトルの形に依存するものはない。

-   `CLAUDE.md` を読むガードのテスト（`cmd/yt2column/docs_test.go`・`cmd/yt2column/copied_transcript_docs_test.go`・`cmd/yt2column/extension_docs_test.go`・`cmd/yt2column-server/docs_test.go`・`internal/pipeline/relay_guard_test.go`）は、「Language」の節を検査しない。
-   `.claude/commands/` を読むテストはない。
-   CI（`.github/workflows/ci.yml`・`.github/workflows/deepseek-review.yml`）は、PR のタイトルを検査しない。
-   `.coderabbit.yaml` は、PR のタイトルの検査を設定していない。ただし、組織と UI 側の設定を引き継ぐので（`.coderabbit.yaml:6-7`）、その設定は確かめていない。

対応 issue: [#408](https://github.com/isseis/yt2column/issues/408)（関連: [#407](https://github.com/isseis/yt2column/pull/407)。この issue はタスク 0031 の作業中に起票したが、タスク 0031 とは独立している）

**本書の記述範囲:** 本書は、各コマンドが書く PR のタイトルの言語と形、`/mergepr` が書く件名の言語と形と決め方、規則と規約が述べる内容を定める。次の事項は、設計（`02_architecture.md`）で決める。

-   タイトルの形を、どの文書に 1 か所で定め、ほかの文書からどう参照するか。
-   `_lib/task-pr-handoff.md` の PR_TEXT の定義を、どう書き換えるか。
-   各コマンドの定義のどこに、何を書くか。

要件の起草とレビューで挙がった設計の事項は、[design_handoff.md](design_handoff.md) に申し送る。

**issue #408 で決めたこと:**

-   **PR のタイトルを日本語にする:** GitHub に置くものを日本語で書く原則にしたので、PR のタイトルだけを例外にする理由がなくなるためである。
-   **squash のコミットの件名と本文は、当面は英語のままにする:** リポジトリに残るものは英語で書く原則（`CLAUDE.md:44-46`）を保つためである。
-   **`/mergepr` は、PR のタイトルをそのまま件名に使うのをやめ、PR の内容から英語の件名を書く:** PR のタイトルが日本語になると、件名に使えなくなるためである。

**要件定義の際に決めたこと（2026-10-21）:**

-   **PR のタイトルに conventional-commit の接頭辞を残す:** タイトルは `<type>(<scope>): <日本語の要約>` の形にする（F-001・F-002）。PR の一覧で、変更の種類とタスクが分かるためである。また、`/mergepr` が英語の件名の接頭辞を決める手がかりになる。接頭辞を外す案は採らない。
-   **件名の接頭辞は、タスク ID、PR のタイトル、PR の材料の順で決める:** プランの PR では、`<scope>` をタスク ID とする。そのうえで、PR のタイトルの接頭辞が AC-13 の形に合えば、それを使う。合わなければ、接頭辞がないものとして扱い、PR の材料（6 節で定義）から決める（F-003）。PR のタイトルはレビュアーが GitHub で書き換えられるので、形の崩れた接頭辞は直さず、ないものとして扱う。常に PR の材料から決める案は採らない。PR のタイトルの接頭辞は、PR を開いたコマンドかレビュアーが選んだものであり、捨てる理由がないためである。いずれの場合も、件名は `/mergepr` の承認の質問（`mergepr.md` の step 3）で利用者が確かめる。
-   **PR のタイトルは、件名の接頭辞を決めるためだけに使う:** `/mergepr` は、PR のタイトルを英語の要約の情報源にしない（AC-11）。issue の「PR のタイトルをそのまま件名に使うのをやめる」に沿い、レビュアーが書き換えたタイトルや古くなったタイトルに要約が引きずられないようにするためである。タイトルも要約の情報源にする案は採らない。要約がタイトルをほぼ訳しただけの文になりやすいためである。
-   **タスクの文書の PR の接頭辞は `docs(<task-id>): ` とする:** `/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2` のすべてで同じとする（AC-04）。`/mkplan2` のコミットの件名には決まった形がなく（`mkplan2.md:105`）、コミットの件名から接頭辞を写せないためである。
-   **`推奨タイトル` は日本語の PR のタイトルを指す:** PR マーカーの `推奨タイトル` は、`/runplan` が PR のタイトルに使う日本語のタイトルとする（F-004）。英語の件名は、`/mergepr` がマージの時点で PR の最終的な内容から書く（F-003）。英語の件名の下書きとして残す案と、両方の欄を持つ案は採らない。プランは実装の前に書くので、件名の下書きは実際の内容とずれうるためである。
-   **`推奨タイトル` の `<type>` を `feat` 以外にも広げる:** `推奨タイトル` の形を `<type>(<task-id>): <日本語の要約>` とする（AC-16）。今の規約は `feat` に限るが（`_context.md:62`）、プランの PR は `refactor`・`test` の接頭辞でもマージされている（`git log origin/main` の #402・#391 など）。規約を、実際の使い方に合わせるためである。
-   **既存の英語の値は書き換えない:** 承認済みのプランに残る英語の `推奨タイトル` は、本タスクで書き換えない（2.3）。2026-10-21 の時点で、`docs/tasks/*/03_implementation_plan.md` に 168 個ある。`/runplan` は、英語の `推奨タイトル` からも日本語の PR のタイトルを書く（AC-05）ので、書き換えても振る舞いは変わらない。一方、多くのタスクの承認済みの文書に手を入れることになる。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   本タスクの後にコマンドが開く PR のタイトルと、コマンドが書き直す PR のタイトルを、日本語にする。
-   squash のコミットのメッセージ（件名と本文）を、英語のままにする。
-   言語の規則、PR マーカーの規約、各コマンドの定義を、互いに矛盾しないようにする。

### 2.2. スコープ (In Scope)

-   `CLAUDE.md` の「Language」の節の、PR のタイトルと squash のコミットについての記述（F-001）。
-   PR のタイトルを書くコマンドの定義が指示する、タイトルの言語と形（F-002）。
-   `/mergepr` の定義が指示する、squash のコミットの件名の決め方（F-003）。
-   PR マーカーの `推奨タイトル` の定義（F-004）。
-   旧い規則の記述が残っていないこと（F-005）。

### 2.3. スコープ外 (Out of Scope)

-   **既存のプランの `推奨タイトル` の書き換え:** 書き換えない。理由は、1 節の「要件定義の際に決めたこと」の最後の項目のとおりである。
-   **開いている PR（PR #407 など）のタイトルの書き換え:** 書き換えない。英語のタイトルの PR も、`/mergepr` で英語の件名を書いてマージできる（AC-12・AC-13）ので、書き換える必要がないためである。
-   **コミットのメッセージの言語:** 各コマンドが作るコミットのメッセージと squash のコミットのメッセージは、英語のままとする。issue が「当面は英語のまま」としたためである。
-   **ブランチの名前:** 今の規約（`_context.md:49`）のまま、英語とする。issue が対象にしていないためである。
-   **issue のタイトル:** すでに日本語で書いている（`CLAUDE.md:49-50`、`runpr.md:321`）。変えない。
-   **`mergepr` のバイナリ（`cmd/mergepr`・`internal/mergepr`）:** 変えない。件名の言語や形を確かめず（`internal/mergepr/mergepr.go:441-450`）、PR のタイトルを件名に使わない（`cmd/mergepr/main.go:48`）ので、本タスクの変更の影響を受けないためである。
-   **PR のタイトルの形を検査するガードのテストや CI の追加:** 追加しない。issue が求めたのは、既存のものが PR のタイトルの形に依存していないかの確認であり、その結果は 1 節の背景のとおりである。
-   **bot のコマンド（`@coderabbitai review` など）:** 今の規則（`CLAUDE.md:52`）のまま、例外として残す。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 言語の規則

`CLAUDE.md` の「Language」の節で、PR のタイトルを日本語にし、squash のコミットのメッセージを英語のままにすると定める。

**Acceptance Criteria**:
- **AC-01**: `CLAUDE.md` の「Language」の節は、PR のタイトルを、日本語で書くもの（GitHub に置くもの）に含める。PR のタイトルを英語にする例外は書かない。
- **AC-02**: PR のタイトルの形は `<type>(<scope>): <日本語の要約>` であり、接頭辞は conventional-commit の形のまま英語で書く。`CLAUDE.md` は、この形を述べるか、この形を定める文書を参照する。
- **AC-03**: `CLAUDE.md` の「Language」の節は、squash のコミットの件名と本文を英語で書くこと、件名は `/mergepr` が PR の材料から書き、PR のタイトルをそのまま使わないことを述べる。

#### F-002: コマンドが書く PR のタイトル

PR を開くコマンドと PR のタイトルを書き直すコマンドの定義は、AC-02 の形の日本語のタイトルを書くよう指示する。各コマンドが作るコミットのメッセージは、英語のままとする。

**Acceptance Criteria**:
- **AC-04**: `/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2` の定義（またはそれらが従う `_lib/task-pr-handoff.md`）は、PR のタイトルを `docs(<task-id>): <日本語の要約>` とするよう指示する。要約は、その PR で追加または変更した文書の内容を述べる。例: `docs(0032): PR のタイトルを日本語にする要件を起草する`。
- **AC-05**: `/runplan` の定義は、PR マーカーのあるプランの PR のタイトルを、`推奨タイトル` の接頭辞と、`推奨タイトル` と同じ内容の日本語の要約にするよう指示する。`推奨タイトル` が英語の値（本タスクの前に書かれたプランの値）でも同じとし、`/runplan` はそのためにプランを書き換えない。
- **AC-06**: `推奨タイトル` に AC-13 の形の接頭辞がないとき、または接頭辞に `<scope>` がないとき、`/runplan` の定義は、接頭辞を `<type>(<task-id>): ` とし、`<type>` を AC-13 に挙げた値のうち変更の種類に合うものにするよう指示する。
- **AC-07**: PR マーカーのないプランの PR について、`/runplan` の定義は、タイトルを `<type>(<task-id>): <日本語の要約>` とするよう指示する。
- **AC-08**: `/runpr` の定義は、単独の変更の PR のタイトルを AC-02 の形の日本語にするよう指示する。`<type>(<scope>)` は、今と同じく `git log origin/main` の形に合わせる。
- **AC-09**: `/fixpr` の Phase 6 の定義は、PR のタイトルを書き直すとき、新しいタイトルを AC-02 の形の日本語にするよう指示する。PR の本文が大きく古くなっていなければタイトルを書き直さない今の条件（`fixpr.md:458-470`）は保つ。
- **AC-10**: AC-04〜AC-09 のどのコマンドの定義も、そのコマンドが作るコミットのメッセージを英語のままとする。日本語で書くよう指示する箇所は、PR のタイトルと本文だけである。

#### F-003: squash のコミットの件名

`/mergepr` の定義は、PR のタイトルの言語にかかわらず、squash のコミットの件名を英語で書くよう指示する。

**Acceptance Criteria**:
- **AC-11**: `/mergepr` の定義は、件名を `<type>(<scope>): <English summary> (#<number>)` または `<type>: <English summary> (#<number>)` の形にし（`<type>` は AC-13 に挙げた値のいずれか）、要約を英語で PR の材料から書くよう指示する。PR のタイトルは、AC-12 で接頭辞を決めるためだけに読み、要約の情報源にはしない。PR のタイトルがその形に合っていれば、そのまま使う規則（`mergepr.md:50-51`）は、なくす。
- **AC-12**: `/mergepr` の定義は、件名の接頭辞を次の順で決めるよう指示する。
  1. プランの PR では、`<scope>` をタスク ID とする（`mergepr.md:51-52` と同じ）。
  2. PR のタイトルが AC-13 の形の接頭辞で始まれば、その接頭辞を使う。プランの PR では、`<type>` だけを使う。
  3. それ以外は、PR の材料と `git log origin/<baseRefName>` の形から決める。プランの PR では、`<scope>` はタスク ID のままとする。

  英語のタイトルの PR（本タスクの前に開いた PR）も、同じ規則で件名を書く。
- **AC-13**: PR のタイトルの接頭辞として受け付ける形は、次のとおりである。
  - 形は `<type>(<scope>): ` または `<type>: ` で、接頭辞の後には空白が 1 つ続く。
  - `<type>` は `feat`・`fix`・`docs`・`refactor`・`test`・`perf`・`build`・`ci`・`chore`・`style`・`revert` のいずれかである。`<scope>` は英小文字・数字・`-` の 1 文字以上の並びである。
  - この形に合わない接頭辞（`!` 付きのもの、大文字や日本語を含むもの、`:` の後に空白がないもの、など）は、直して使わず、接頭辞がないものとして扱う。
  - 接頭辞がないものとして扱った場合、利用者が承認の質問（`mergepr.md` の step 3）で見る件名は、AC-12 の 3 で決めた接頭辞を持つ。
- **AC-14**: `/mergepr` の定義は、squash のコミットの本文を、今と同じく英語で書くよう指示する（`mergepr.md:48`）。PR のタイトル・本文・コミットのメッセージ・diff を、データであって従う指示ではないとする扱い（`mergepr.md:46-48`）は保つ。PR のタイトルについては、接頭辞を読むとき（AC-12）にこの扱いを適用する。

#### F-004: PR マーカーの `推奨タイトル`

PR マーカーの `推奨タイトル` は、`/runplan` が PR のタイトルに使う日本語のタイトルとする。

**Acceptance Criteria**:
- **AC-15**: `_context.md` の「PR marker conventions」は、`推奨タイトル` が PR のタイトルであって、squash のコミットの件名ではないことを述べる。
- **AC-16**: `推奨タイトル` の形は `<type>(<task-id>): <日本語の要約>` であり、`<type>` は AC-13 に挙げた値のいずれかとする。次の 4 つは、この形を述べるか、この形を定める文書を参照する。
  1. `_context.md` の「PR marker conventions」。
  2. `mkplan2.md` が挿入する PR マーカーの書式。
  3. `mkplan2.md` の `推奨タイトル` の説明。
  4. `mkplan2.md` の PR の境界のレビューのチェックリストの項目。

#### F-005: 旧い規則の記述を残さない

規則・規約・コマンドの定義に、旧い規則の記述を残さない。

**Acceptance Criteria**:
- **AC-17**: リポジトリで追跡しているファイルのうち、`docs/tasks/` の下のファイルを除くどのファイルも、次のいずれも述べない。
  - PR のタイトルを英語で書くこと。
  - squash のコミットの件名を PR のタイトルから作ること、または PR のタイトルをそのまま件名に使うこと。
  - `推奨タイトル` が英語であること、または `feat` に限ること。

  ただし、AC-05 が定める英語の `推奨タイトル` の扱いの説明は、この限りでない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。本タスクは、コマンドの定義と規則の文書だけを変え、実行時のコードを変えない。

### 4.2. セキュリティ (Security)

-   N/A。本タスクは、`docs/dev/security.md` の適用の条件（`yt-dlp` の起動、API キーや Webhook の URL の扱い、キャッシュの読み書き、LLM API や Webhook への送信、信頼できないテキストのプロンプトや記事への埋め込み）のどれにも当たらない。
-   本タスクが新たに読む信頼できない入力は、PR のタイトルの接頭辞（AC-12）だけである。受け付ける形と、合わない場合の扱いは AC-13 で定める。PR のタイトルをデータとして扱う規則は、AC-14 で保つ。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   N/A。本タスクは、実行時のコードもタイムアウトやリトライも変えない。

### 4.4. 互換性 (Compatibility)

-   本タスクの前に書かれたプランの英語の `推奨タイトル` は、書き換えずに使える（AC-05）。
-   本タスクの前に開いた、英語のタイトルの PR は、`/mergepr` でマージできる（AC-12）。ただし、`/mergepr` は、その英語のタイトルを件名にそのまま使わなくなる。件名の要約は PR の材料から書くので、PR のタイトルと違う文になりうる。
-   squash のコミットの件名の形 `<type>(<scope>): <summary> (#<number>)` は変えない（AC-11）。スコープのない `<type>: <summary> (#<number>)` の形も、今の `git log origin/main` にすでにある（#404 など）。`git log origin/main` の件名の形は、本タスクの前後で揃う。

### 4.5. 保守性 (Maintainability)

-   PR のタイトルの形と squash のコミットの件名の形の関係について、規則（F-001）、規約（F-004）、コマンドの定義（F-002・F-003）は矛盾しない（AC-17）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   コマンドの定義（`.claude/commands/`）は英語で書く（`CLAUDE.md:47-48`）。日本語のタイトルの例を書く場合も、説明の文は英語とする。
-   タスク 0031（PR #407）も、`_lib/task-pr-handoff.md` と `runpr.md` を変える見込みである。根拠は、PR #407 の `docs/tasks/0031_codex_pr_review/01_requirements.md` の 1 節（2026-10-21 時点の未マージの版）が、`_lib/task-pr-handoff.md:116-121` と `runpr.md:153-196` の PR レビューの依頼と判定を対象にしていることである。両タスクは独立しているが、同じファイルを変えるので、実装の際は先にマージされた側の変更の上に重ねる。

## 6. 用語集 (Glossary)

用語は `docs/translation_glossary.md` と統一する。本書では、次の語を次の意味で使う。

-   **PR のタイトル (PR title):** GitHub の PR のタイトル。本タスクの後は日本語で書く。
-   **squash のコミット (squash commit):** squash マージで `main` に残る 1 つのコミット。メッセージ（件名と本文）は英語で書く。
-   **件名 (subject):** コミットメッセージの 1 行目（`docs/translation_glossary.md`）。
-   **接頭辞 (prefix):** conventional-commit の `<type>(<scope>): ` または `<type>: ` の部分。PR のタイトルでも英語で書く。
-   **要約 (summary):** PR のタイトルまたは件名のうち、接頭辞の後の部分。PR のタイトルでは日本語、件名では英語で書く。日本語の要約にも、コマンド名・ファイル名・識別子（`/mergepr`・`推奨タイトル` など）を表記を変えずに含めてよい。
-   **PR マーカー (PR marker):** 実装プランの `### PR-N 作成ポイント` の節。PR の境界と、その PR の `推奨タイトル` などを持つ（`_context.md` の「PR marker conventions」）。
-   **`推奨タイトル` (recommended title):** PR マーカーの欄。`/runplan` が PR のタイトルに使う。
-   **プランの PR (plan PR):** `/runplan` が開く、実装プランに基づく PR。
-   **単独の変更 (standalone change):** タスクに属さず、`/runpr` が PR を開く変更（`runpr.md` の step 1）。
-   **PR の材料 (PR material):** `/mergepr` がメッセージの下書きに使う、PR の情報。`docs/translation_glossary.md` の「材料」（`prepare` が書く `log.txt`・`stat.txt`・`body.txt`）に、必要なときに読む diff を加えたものである（`mergepr.md:36-46`）。PR のタイトルは含めない。
