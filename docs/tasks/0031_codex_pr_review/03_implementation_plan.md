# 実装計画書：PR レビューを CodeRabbit から Codex の GitHub 連携に切り替える

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-22 |
| Review date | 2026-10-22 |
| Reviewer | isseis |
| Comments | 2026-10-22 (editorial): ステップ 1-5 の `TestGuideDescribesMentionCheck` を、定数の値がガイドの 7.1・7.2 節にあることの確認だけに絞った。9 章の但し書きと 7.2 節の古い文の言い回しを固定する確認は、意味の正しさを確かめられないので外し、読み合わせに任せた。検証する AC と方針は変わらないので、承認を保つ（isseis がこのセッションで了承した）。2026-10-22 (editorial): ステップ 1-5 の読み合わせの項目と PR-1 のレビュー観点で、検査の時点を「GitHub に触れる前」から「GitHub への書き込み前」に改めた。PR 番号を省略すると、検査の前に今の PR を GitHub から読むためで、ガイドの記述とも合わせた。実装と方針は変わらず、決定は変わらないので、承認を保つ 2026-10-23 (decision): タスク 0038 の F-005 により、AC-01〜AC-15・AC-17・AC-22・AC-24・AC-26・AC-27・AC-29 の検証方法を改めた（`TestCodexObservationRecord`・`TestHandoffStartMarker`・`TestDocsDescribeCodexReview`・`TestNoCodeRabbitPRReview`・`TestLocalCodeRabbitReviewKept`・`TestGlossaryTerms` と、`TestRunprDocContract`・`TestFixprDocContract`・`TestAgentsCodeReviewRules` の語句の確認を削除し、`TestRunprDocContract/listings_print_no_body` を設けたため）。決定はタスク 0038 の要件の承認で得たので、`Status` は `approved` のままとする。 |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の、H-NN は [design_handoff.md](design_handoff.md) の項番を指す。設計書の「n.n」は設計書の節を指す。既存のファイルに関する記述は、コミット `419dec0` のファイルで確かめた。`file:line` はこのコミットの行番号を指す。設計書が確かめたコミット `3ed049b` から `419dec0` までの差分は `docs/tasks/` の下だけなので（`git diff --stat 3ed049b 419dec0`。2026-10-22 に実行）、設計書の行番号もそのまま使える。

用語は設計書の冒頭の定義と同じ意味で使う（PR を開くコマンド、要約コメント、状態の行、開始の印、依頼の文面とメンション、想定外の投稿、判定できない状態、依頼したコミット、文書のガードなど）。

## 1. 実装の概要

### 1.1 目的

-   `fixprguard` の `reply`・`edit-pr` が、`@codex` のメンションを含む本文とタイトルを拒否するようにする（設計書 3.8 の「ツールの拒否」。PR-1）。
-   コマンドの文書を、Codex の PR レビューを待ってレビュー済みを判定する手順に切り替え、CodeRabbit の PR レビューを止める（設計書 3.1・3.5〜3.11。PR-2、要件書の「切り替えの PR」）。

### 1.2 実装の原則

-   設計書の内容を本書で繰り返さない。各ステップは、設計書のどの節を実装するかを示し、作業と確かめることだけを書く。
-   レビュー済みの判定はコマンドの文書の手順であり、Go のテストで動かせない（設計書 7 章）。手順の記述は文書のガードで固定し、振る舞いは切り替えの PR 自身の `/runpr` と、その後の PR で確かめる。
-   文書のガードとテストは、実装者が対象を壊し、そのガードやテストが失敗することを確かめて、コミットメッセージに書く（`CLAUDE.md` の「Every test must be able to fail for its stated reason」、設計書 7.2）。
-   **PR とコミットの文面にメンションを書かない。** 本タスクの PR のタイトル・説明、`/fixpr` が書き換える PR の説明、PR の説明の元になるコミットメッセージは、メンションを `@codex` と書かず、「Codex へのメンション」や「`_context.md` の `PR review request`」と書く。PR の作成と説明の書き換えは利用者のアカウントの投稿であり（設計書 3.8）、PR-2 では、本計画のフェーズ 3 が加える投稿の前の確認が PR の作成や説明の書き換えを止めるためである。Go のソースとテストの中の値は対象外である。
-   Go のコメント・識別子・文字列リテラルは英語で書く。例外は、日本語の文書の見出しや表のセルと一致させるガードのテストのリテラルだけである（`CLAUDE.md` の「Go Idioms」）。
-   `fixprguard_guide` は日本語版を先に書き、英語版はその翻訳として書く（`CLAUDE.md` の「Translation Guidelines」）。

### 1.3 計画の際に決めたこと（2026-10-22）

計画の範囲で、設計書が決めていない選択はなかった。PR の分け方（PR-1 と PR-2）と順序は設計書 8 章が決めているので、利用者への質問はしていない。次は、本書を書く際に決めたことである。

-   **PR-2 を 3 つのフェーズに分ける（フェーズ 2〜4）。** PR は 1 つのままとし、各フェーズの終わりで `make test`・`make lint` が通る単位に分ける。理由: `TestContextAndCommandsAgree`（`cmd/yt2column/fixpr_docs_test.go:527-570`）は、`Awaited PR reviewer` の値と、`runpr.md` が名前を書いてよい唯一の文字列 `"@" + 名前 + " review"`（同 `549`）を結びつけている。そのため、`_context.md` の値の変更と `runpr.md` step 3 の書き換えは、同じフェーズに入れないと途中でガードが失敗する。投稿の前の確認（設計書 3.8）と、PR レビューを名指しするその他の文書（設計書 3.9・3.10）は、判定の切り替えと独立に書けるので、別のフェーズにしてレビューしやすくする。
-   **設計書の取りこぼしを、編集上の訂正で直した。** `_lib/coderabbit-local-review.md:83-84` は「the PR-side CodeRabbit review」と、PR レビューを CodeRabbit として述べる（AC-24 の対象）。設計書 2.2 はこのファイルを変えないものに挙げていた。ローカルの CLI レビューを変えないという決定は変わらないので、設計書の 2.2・3.10 にこの文だけを改めることを書き加え、承認を保った（設計書の Comments）。
-   **AC-01〜AC-04 は観測の記録の内容を確かめるガードで検証し、AC-05 は手続きの記録で検証する。** 観測は設計の前に済み、[observations.md](observations.md) に記録されている。新しい観測はしない。ただし、PR での観測のうち依頼したコミットの SHA（AC-02）が書かれていないものは、ステップ 4-5 で GitHub の記録から補う。AC-01〜AC-04 には、記録が AC の求める値を持つことを確かめる確認を置く（ステップ 4-5）。AC-05 は「設計を始めない」という手続きの条件なので、記録の「AC-05 に当たる点」の節と、要件書・設計書の承認の順序（各文書の Document Status）を根拠とする。
-   **設計書 3.11 の確認を、次のとおり強める。** どれも設計書の確認と同じファイルに置き、設計書と矛盾しない。
    -   CodeRabbit の PR レビューの語の確認は、設計書 3.11 の語に加えて、今の文書で PR レビューを CodeRabbit として述べている文（1.4 の一覧）がすべて消えたことを確かめる。あわせて、ローカルの CLI レビューを CodeRabbit と呼ぶ記述が残ることを確かめる（AC-24 の後半）。
    -   投稿の前の確認は、1.4 の表の 7 か所の節のそれぞれに、決まった文と、設計書 3.8 の表の「含まれていたときの扱い」があることを確かめる。さらに、`.claude/commands/` の下の投稿のコマンドが、確認のある節の外に加えられたら失敗する確認を置く。
    -   `runpr.md` の判定は、規則 1〜6 の順序、祖先の判定の向き、依頼のコメントの絞り込み、Part B step 5 の開始の印の絞り込みまで固定する。
    -   `fixprguard` のガイド（日英）が、メンションの定数の値、9 章の但し書きを述べ、古い「内容は検査しない」の文が消えたことを確かめる。
    -   `CLAUDE.md` が Codex の設定を書いていることを確かめる（AC-07）。
-   **振る舞いの AC を文書のガードで検証することを、例外として記録する。** AC-06・AC-08〜AC-15・AC-17 は手順の振る舞いだが、手順はコマンドの文書にあり、Go のテストで動かせない（設計書 7 章）。そのため `static` の検証は文書の記述の固定であり、振る舞いは `manual` の確かめで補う（7 章の注）。

### 1.4 既存コード調査結果

**`internal/fixprguard`（PR-1）**

-   `Reply` は、引数を検査した後、`readTextFile` で本文を読み（`internal/fixprguard/write.go:108`）、次の `resolveRepository` で初めて GitHub に触れる（同 `112`）。`EditPR` は、`readTitleFile`（同 `275`）と `readTextFile`（同 `279`）の後に `gh pr edit` を呼ぶ（同 `283`）。`checkPostText` はこの読み取りの直後に置ける。`readTextFile` の契約のコメント（同 `20-27`）は変えない（設計書 3.8）。
-   `ErrInvalidInput` のコメントは「a malformed flag value」だけを挙げ（`internal/fixprguard/errors.go:12-13`）、今でも `readTextFile` の拒否（同 `write.go:31-46`）を説明に含んでいない。ステップ 1-1 で直す。
-   `ErrInvalidInput` を包んだエラーは、`cmd/fixprguard/main.go:287-292` の `stop` が終了コード `2` にし、標準出力に何も書かない。新しいコードは要らない。
-   テストで再利用するもの:
    -   `internal/fixprguard` の側: 本文のファイルを作る `writeTextFile`（`internal/fixprguard/write_test.go:21`）。手順を持たない fake で「どの `gh` の呼び出しもテストを落とす」形の `TestReplyRejectsInputBeforeAnyCall`（同 `271`）と `TestEditPRVerbatim` の `rejects_before_any_call`（同 `371-386`）。受け付ける場合の手順を作る `replyLookupSteps`・`writeSteps`・`replyRequest`（同 `106`・`130`・`141`）と `editStep`（同 `348`）。
    -   `cmd/fixprguard` の側: `newTestRun`（`cmd/fixprguard/test_helpers.go:151`）。引数の検査を通る `reply` の行を作る `replyArgs`（`cmd/fixprguard/main_test.go:660`）。診断が値を写さないことを確かめる `hidden` の形（同 `113-187`）。
    -   `validArgs`（同 `24`）は使わない。指紋 `f` が `Reply` の引数の検査（`write.go:105`）で先に拒否され、本文まで届かないためである。
    -   新しいテストの補助のファイルは要らない。
-   テストに `@codex` を含む文面はない（`git grep -n -i "@codex" -- ':!docs/tasks'` は 0 件。2026-10-22、`419dec0` で実行）。既存のテストは、拒否を加えても失敗しない。

**`fixprguard` のガイド（PR-1）**

-   日英のガイドの 7.2 節の最後の項目は「本文の内容。ツールはファイルの種類・大きさ・文字コードだけを検査する」と述べる（`docs/dev/developer_guide/fixprguard_guide.ja.md:259`、英語版の同じ行）。この項目を固定するガードはない。
-   `cmd/fixprguard/docs_test.go` には、日英のガイドの一覧 `guideFiles`（`:25`）、`readRepoFile`（`:31`）、`singleLine`（`:180`）、`section`・`subsection`（`:197`・`:201`）と、古い文が消えたことを確かめる `stale` の形（`TestGuideDescribesConfigScope`、`:454-503`）がある。9 章については、`:305-308` が既存の `Protocol` の但し書き（`tc.proviso`）を含むことを確かめている。新しい但し書きを加えても、既存の文を残せばこの確認は失敗しない。

**コマンドの文書が GitHub に投稿する経路（PR-2）**

`git grep -n -E "gh (issue (create|comment|close|edit)|pr (comment|create|edit|review|merge))|-X POST|--method (POST|PATCH)|fixprguard (reply|edit-pr)" -- .claude/commands` を `419dec0` で実行した結果（2026-10-22）、投稿の経路は次のとおりで、設計書 3.8 の表の 6 種類の投稿（7 か所の節）と一致した。ほかの一致は、`mergepr.md:147`（squash のマージ。設計書 3.8 が対象外とする）と、コマンドを呼ばずに名前だけを挙げる文（`_context.md:98` の説明、Authorization の節や許可の一覧の `_lib/task-pr-handoff.md:54-55`・`fixpr.md:75`・`runpr.md:53`・`61`・`63`、エージェントへの禁止の `fixpr.md:618`）である。

| 節（7 か所） | 投稿 | 場所 |
|---|---|---|
| `fixpr.md` フェーズ 5 | スレッドへの返信 | `fixpr.md:596` |
| `fixpr.md` フェーズ 6 | PR のタイトルと本文の書き換え | `fixpr.md:643` |
| `runpr.md` step 1 | PR の作成 | `runpr.md:111` |
| `_lib/task-pr-handoff.md` Part B step 3 | PR の作成 | `_lib/task-pr-handoff.md:166` |
| `runpr.md` step 4 | deferred のスレッドを移す issue・コメント、スレッドへの返信 | `runpr.md:394-395`・`404` |
| `runpr.md` step 5 | issue を閉じるコメント | `runpr.md:470` |
| `_lib/task-pr-handoff.md` Part A step 4 | link sub-issue の本文 | `_lib/task-pr-handoff.md:142-143` |
| （確認の対象外） | 依頼のコメント | `runpr.md:244`、`_lib/task-pr-handoff.md:190` |

`runpr.md:470` の `gh issue close` は、コメントを `--comment "<comment>"` の引数で渡す（`gh issue close` にファイルから読むフラグはない。`gh issue close --help`、2.96.0）。確認はコメントの文面を書いた時点で行う。

**CodeRabbit を名指しする記述（PR-2）**

`git grep -n -i "coderabbit" -- .claude/commands CLAUDE.md AGENTS.md .coderabbit.yaml` を `419dec0` で実行した結果（2026-10-22、80 行）を、次のとおり分けた。

-   **残すもの（ローカルの CLI レビュー）:** `_lib/coderabbit-local-review.md`（`:83-84` を除く全体）、`runplan.md:50`・`85-86`・`88`・`208`・`211`、`_lib/triage-pattern.md:7`、`_lib/task-pr-handoff.md:17`・`157-158`・`178`、`runpr.md:9-10`・`68`・`102`、`fixpr.md:78`・`553`・`560`、`CLAUDE.md:96`（「local CodeRabbit」）・`392-394`。
-   **残すもの（そのほか）:** `_context.md:185`（自動レビュアーの一覧の `coderabbitai`。設計書 1.2）、`.coderabbit.yaml:1`（スキーマの行）。
-   **改めるもの（PR レビューを CodeRabbit として述べる）:** `_context.md:186`、`runpr.md:2`・`30`・`53-54`・`190-249`・`264`・`270`・`320-322`・`353`・`378`、`_lib/task-pr-handoff.md:20`・`45`・`55`・`186-191`・`197`、`_lib/pr-review-escalation.md:2`・`7-12`・`22`、`_lib/coderabbit-local-review.md:83-84`、`fixpr.md:86-89`・`557-558`、`mkreq.md:16`、`CLAUDE.md:56`・`86`・`99`・`104-106`、`.coderabbit.yaml:9-14`。どれも設計書 3.5〜3.10 が対象に挙げている（`:83-84` は 1.3 の訂正の後）。

`AGENTS.md` には CodeRabbit の語はなく、見出し `## Review guidelines`（`AGENTS.md:5`）を改める。`AGENTS.md:3` は節の外ですでに `CLAUDE.md` を参照している。`.claude/commands`・`CLAUDE.md`・`AGENTS.md` の外で CodeRabbit を名指しするのは、`.claude/settings.json:4`（`coderabbit review` の許可）、`fixprguard_guide` の使用例の `--reviewer coderabbitai`（設計書 2.2 が変えないとする）、Go のテストの値（信頼の判定のテストの名前。変えない）だけである。

**文書のガード（PR-2）**

-   `cmd/yt2column/fixpr_docs_test.go` で変えるもの: `runprStep3`（`:37`）、`awaitedReviewerSentence`（`:79-80`）、`reviewedCheck`（`:85-90`）、`lastReviewedRule`（`:92-94`）、`autoFixprCondition`（`:98-106`）、`TestRunprDocContract` の `no_body_fetch`（`:323-333`）、`TestContextAndCommandsAgree`（`:527-570`）、`TestContextReviewerRowsMalformed`（`:572-626`）、`TestGlossaryTerms`（`:702-734`）。
-   書き換える文書の中で、変えずに保つ文を固定しているガード: `TestContextAndCommandsAgree` の `Awaited PR reviewer` の説明の「REST `user.login`」「followed by `[bot]`」（`:535`）、`TestRunprDocContract` の `untrusted_question`・`stop_without_merge`（`:298-321`）、`TestFixprguardCallSites` の step 3 の `fixprguard list `（`:346-367`）、`TestAuthorizationSections` の `runpr.md` の Authorization の節（`:644-664`）、`cmd/fixprguard/docs_test.go` の `TestDocumentedInvocationsParse`（`:122`、文書が書く `fixprguard` の呼び出し）。
-   再利用する補助: `readDoc`・`markdownTables`・`findTable`・`tableRow`・`tableRowsWithFirstCell`・`sectionByHeading`・`containsToken`（`cmd/yt2column/docs_test.go:313`・`325`・`377`・`393`・`405`・`464`・`623`）、`flat`・`requireSection`・`requirePhrases`・`contextLine`（`cmd/yt2column/fixpr_docs_test.go:111`・`140`・`151`・`630`）、`repositoryRoot`（`cmd/yt2column/makefile_test.go:23`）、文書の順序を確かめる `head_before_checkout` の形（`fixpr_docs_test.go:286-296`）、壊した写しの表の `checkContextReviewers`・`TestContextReviewerRowsMalformed` の形（`:452`・`:572`）。補助のファイルは作らない。
-   `CLAUDE.md` を読むほかのガード（`cmd/fixprguard/docs_test.go:59`、`cmd/yt2column-server/docs_test.go:253-276`、`cmd/yt2column/extension_docs_test.go:91`、`cmd/yt2column/copied_transcript_docs_test.go:189`・`228`、`internal/pipeline/relay_guard_test.go:435`）が固定する文は、本タスクで変える行に含まれない。

**外部の前提**

-   `gh api` の `-i, --include`（応答のステータスの行とヘッダーを出す）、`--paginate`、`--jq` は、`gh api --help`（`gh` 2.96.0）で確かめた。`Date` ヘッダーが応答に含まれることは、ステップ 2-3 で読み取りだけの呼び出しで確かめる（ネットワークの操作なので、本書を書く際には実行していない）。
-   `git merge-base --is-ancestor` は、祖先なら `0`、祖先でなければ `1`、存在しないオブジェクトなら `128` を返した（`git` 2.50.1。2026-10-22 に `419dec0` と `3ed049b` と存在しない SHA で実行）。設計書 3.3 の「`0`・`1` 以外は依頼したコミットでない」と合う。
-   `go version -m <binary>` で、`go install` したツールの `vcs.revision` と `vcs.modified` を読める（`fixprguard_guide` の 4 章の終了コード `2` の行がこの手順を案内している）。
-   `.coderabbit.yaml` の `reviews.auto_review.enabled` のキーは、ファイルの 1 行目が示すスキーマ（`https://coderabbit.ai/integrations/schema.v2.json`）で、ステップ 4-1 で確かめる（ネットワークの操作なので、本書を書く際には読んでいない）。CodeRabbit が設定を base と head のどちらから読むかは確かめていない（設計書 3.9）。

**`implementation_handoff.md`** はない。`design_handoff.md` の H-01〜H-09 は、すべて設計書の付録 A に記録されている。

### 1.5 名前の変更の一覧

本書が変える名前・見出しは、次の表だけである。ほかの節の記述は、この表に従う。

| 種類 | 変更前 | 変更後 | ステップ |
|---|---|---|---|
| `runpr.md` の見出し（`runprStep3` の値） | `## 3. Wait for CodeRabbit's first review and CI, running fixpr as comments appear` | `## 3. Wait for Codex's first review and CI, running fixpr as comments appear` | 2-2 |
| `AGENTS.md` の見出し | `## Review guidelines` | `## Code Review Rules` | 4-2 |
| `_context.md` の行の値（`Awaited PR reviewer`） | `coderabbitai` | `chatgpt-codex-connector` | 2-1 |

## 2. 実装ステップ

### フェーズ 1: `fixprguard` のメンションの拒否（設計書 3.8 の「ツールの拒否」、4 章）

#### ステップ 1-1: 定数とエラー値

**変更するファイル**: `internal/fixprguard/errors.go`

- [x] 設計書 4 章のとおり、公開の定数 `CodexMention` と非公開のエラー値 `errCodexMention`（`ErrInvalidInput` を包む）を加える。
- [x] `ErrInvalidInput` のコメントを、フラグの値の誤りに加えて、`reply`・`edit-pr` が拒否するテキストのファイル（種類・大きさ・文字コード・内容）も含む説明に改める。

**完了の条件**: ステップ 1-2 と同じコミットで `make lint` が通る（`errCodexMention` は 1-2 で使う）。

#### ステップ 1-2: `checkPostText` と適用

**変更するファイル**: `internal/fixprguard/write.go`

- [x] `checkPostText` を加える。拒否する条件と拒否しないものは設計書 3.8 の箇条のとおりで、比べ方は ASCII の範囲で大文字と小文字を区別しない部分文字列の一致である（語の境界を求めない。Unicode の正規化はしない）。
- [x] `Reply` で、本文を `readTextFile` で読んだ直後、`resolveRepository` より前に、本文に適用する。
- [x] `EditPR` で、タイトルと本文を読んだ後、`runGH` より前に、両方に適用する。
- [x] `readTextFile`・`readTitleFile` には加えない（設計書 3.8）。

**完了の条件**: ステップ 1-3・1-4 のテストが通る。

#### ステップ 1-3: ユニットテスト

**変更するファイル**: `internal/fixprguard/write_test.go`

- [x] `TestCheckPostText`: `checkPostText` の範囲の境界を 1 つの表で固定する。拒否する行は、設計書 7.1 の入力（大文字と小文字の違い、行頭と文の途中、`@codex review`）と、設計書 3.8 が拒否するとした語の一部（`@codexbot`、`a@codex.example`）。受け付ける行は、`@` のない `Codex`・`codex`、`@coderabbitai`、全角の `＠codex`、`@` と `codex` の間に幅のない文字を挟んだもの。
- [x] `TestReplyRejectsCodexMention`・`TestEditPRRejectsCodexMention`: `Reply` の本文、`EditPR` のタイトル、`EditPR` の本文のそれぞれについて、メンションを含む文面が `errCodexMention` で拒否され（`errors.Is`）、手順を持たない fake で `gh` が一度も呼ばれないことを確かめる。`EditPR` の行は、メンションをタイトルと本文の一方だけに置き、他方はメンションのない文面にする。受け付けの側は、メンションのない文面で `Reply`・`EditPR` が投稿まで進む行を各 1 つ置く（範囲の境界は `TestCheckPostText` が持つ）。
- [x] 検査の順序を固定する行を置く: メンションを含み、かつ `readTextFile` で拒否される本文（大きさの上限を超える、または UTF-8 でない）は、`errCodexMention` ではなくファイルの拒否になる。

#### ステップ 1-4: コマンドのテスト

**変更するファイル**: `cmd/fixprguard/main_test.go`

- [x] `TestPostTextMentionExitsUsage`: `replyArgs` で作った引数の検査を通る `reply` の行（実在する本文のファイル）と、実在するタイトル・本文のファイルを渡す `edit-pr` の行について、メンションを含む文面で終了コード `2`（`exitUsage`）、標準出力が空、標準エラーの診断がメンションの拒否を示すこと、診断に文面の中の目印の文字列が出ないこと（`hidden` の形）を確かめる。`edit-pr` は、タイトルだけと本文だけの 2 行にする。
- [x] 対照の行として、メンションのない文面で `gh` の fake に届くことを確かめる行を置く。

#### ステップ 1-5: ガイド（日英）

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`、`docs/dev/developer_guide/fixprguard_guide.md`、`cmd/fixprguard/docs_test.go`

- [x] 日本語版を設計書 3.8 の「ガイド」のとおりに改める: 5 章の `reply`・`edit-pr` の本文とタイトルの条件、4 章の終了コード `2` の説明、7.1 の保証の表に「メンションを含む文面を GitHub に送らない」行、7.2 の「本文の内容」の項目（内容はメンションだけを検査する。全角や幅のない文字を挟んだ形は検査しない）、9 章の `Protocol` の但し書き（設計書 1.4 の 2 行目）。9 章の既存の但し書きの文は残す。
- [x] 英語版を、日本語版の翻訳として同じ構造で改める（`.claude/commands/mktrans.md` の原則と `docs/translation_glossary.md` に従う）。日英の見出しと表の行が対応していることを読み合わせる。
- [x] `TestGuideDescribesMentionCheck`（`cmd/fixprguard/docs_test.go`）: `guideFiles` の日英のそれぞれで、7.1 節と 7.2 節が `fixprguard.CodexMention` の値を含むことを確かめる。コードの定数とガイドの食い違いを検出するためである。9 章の但し書きと 7.2 節の古い文は、言い回しを固定しても意味の正しさを確かめられないので固定せず、読み合わせ（次の項目）で確かめる。
- [x] 書いた記述を実装と照らし合わせる: 拒否の条件、終了コード、拒否しないもの、検査の順序（GitHub への書き込み前）が、ステップ 1-2 のコードと一致することを読み合わせ、結果をコミットメッセージに書く。

#### ステップ 1-6: 壊して失敗を確かめる

- [x] 次のそれぞれで、対応するテストが失敗することを確かめ、コミットメッセージに書く。
    -   `Reply` から `checkPostText` の呼び出しを外す（1-3 の `Reply` の行、1-4 の `reply` の行）。
    -   `EditPR` のタイトルへの適用だけを外す、本文への適用だけを外す（1-3・1-4 の該当する行）。
    -   比べ方を大文字と小文字を区別するものにする（大文字を含む行）。
    -   `@codex` の後に語の境界を求める（`@codexbot` の行）。
    -   `checkPostText` を `readTextFile` より前に動かす（順序の行）。
    -   診断に文面を含める（1-4 の目印の確認）。
    -   ガイドの片方の 7.2 節から定数の値を消す（1-5 の確認）。

**フェーズ 1 の完了の条件**: `make fmt`・`make test`・`make lint` が通る。`Protocol` は上げない（設計書 1.4）。

### PR-1 作成ポイント: fixprguard mention rejection

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6

**推奨タイトル**: `feat(0031): fixprguard の返信と PR の説明で Codex へのメンションを拒否する`

**レビュー観点**: 拒否が GitHub への書き込み前に起こり、`reply` と `edit-pr` の 3 つの文面すべてに適用されていること / 拒否の範囲の境界（大文字と小文字、語の一部、全角、幅のない文字、`@` なし）がテストで固定されていること / `Protocol` を上げない判断と、ガイドの 9 章の但し書きが設計書 1.4 と一致すること / PR の文面とコミットメッセージにメンションを書いていないこと（1.2）

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-2 は、利用者のアカウントの投稿が Codex を動かすのを防ぐセキュリティの守り（mkplan の panel mode の「security-gate」）に当たる。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/448
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 完了を待つ PR レビュアーと、レビュー済みの判定の切り替え（設計書 3.1〜3.7）

前提: PR-1 がマージされ、利用者が `make install-fixprguard` で再インストールしていること（ステップ 2-0 で確かめる）。

#### ステップ 2-0: 再インストールの確認

- [x] PR-1 のマージのコミットを `gh pr view <PR-1> --json mergeCommit` で求め、`go version -m "$(command -v fixprguard)"` の `vcs.revision` がそのコミットまたはその子孫であること（`git merge-base --is-ancestor`）と、`vcs.modified` が `false` であることを確かめる。違えば、利用者に再インストールを頼み、先に進まない（設計書 1.2・5.3）。

**完了の条件**: 両方の確認が通る。

#### ステップ 2-1: `_context.md` の値

**変更するファイル**: `.claude/commands/_context.md`

- [x] 設計書 3.1 の表のとおり、`Awaited PR reviewer` の値を `chatgpt-codex-connector` にし、説明の文を改める。説明には、`TestContextAndCommandsAgree` が求める「REST `user.login`」と「followed by `[bot]`」を残す（1.4）。
- [x] `PR review request` の行を、設計書 3.1 の表の説明の文とともに加える。行の形は `Awaited PR reviewer` の行と同じにする。

**完了の条件**: ステップ 2-5 の `_context.md` の確認が通る。

#### ステップ 2-2: `runpr.md` の枠組みと step 4

**変更するファイル**: `.claude/commands/runpr.md`

- [x] 設計書 3.5 の次の箇条のとおりに改める: `description` と冒頭、Authorization、冒頭のスレッドの読み方の文（設計書 1.4 の 3 行目の例外）、プレースホルダーの定義（設計書 3.1 の 3 つ）、step 3 の見出し（1.5 の一覧）と冒頭、`fixpr` を再び実行する条件の語、CodeRabbit のステータスによる停止の削除、step 4 の PR レビューの質問と依頼。
- [x] 1.4 の「変えずに保つ文を固定しているガード」の文を残す。

**完了の条件**: 1.4 に挙げた既存のガードが通る。

#### ステップ 2-3: `runpr.md` step 3 の判定と依頼

**変更するファイル**: `.claude/commands/runpr.md`

- [x] 「Reviewed」を、設計書 3.2（読む値と受け入れる形、状態の行の受け入れる形、読み方の制約、時点と時間の測り方、前提とする運用）と 3.3（依頼の一覧、依頼したコミット、最新の依頼の状態の規則 1〜6 を上からこの順で当てはめること、使わない投稿の表、断りのコメント、LAST_REVIEWED）のとおりに、エージェントが従える手順として書く。要約コメントは 1 行目と 2 行目だけを取り出す `--jq` の絞り込みで読む。
- [x] 「Requesting the review」を、設計書 3.4（依頼を投稿する条件と、待っていた依頼が完了した後の確かめ直し）、4 章（停止の表、止まり続けるとき）、6.1・6.2 の流れのとおりに書く。依頼は `gh pr comment <number> --body "<review-request>"` で、初回・エスカレーション・step 4 のどれも同じ書き方にする。
- [x] `Date` ヘッダーを読む呼び出し（設計書 3.2）を、読み取りだけの `gh api -i` で一度実行し、`Date` の行が出ることを確かめる（ネットワークの操作なので、実行の前に利用者の承認を得る）。

**完了の条件**: 書いた手順を、設計書 3.2〜3.4・4 章・6 章と 1 項目ずつ読み合わせ、落ちた規則がないことを確かめる。ステップ 2-5 の `TestRunprDocContract` が通る。

#### ステップ 2-4: PR を開くコマンドとエスカレーション

**変更するファイル**: `.claude/commands/_lib/task-pr-handoff.md`、`.claude/commands/_lib/pr-review-escalation.md`

- [x] `_lib/task-pr-handoff.md` を、設計書 3.6 の箇条のうち「投稿の前の確認」を除くすべてに従って改める（冒頭・AFTER_CREATE・Part B step 7 の語、Authorization、プレースホルダーの定義、Part B step 5）。プレースホルダーの定義は `runpr.md` と同じ文にする。
- [x] `_lib/pr-review-escalation.md` を設計書 3.7 のとおりに改める。判断の基準と手順は変えない。`_lib/pr-review-escalation.md:9` の `@coderabbitai review` を消す。

**完了の条件**: ステップ 2-5 の確認が通る。

#### ステップ 2-5: ガードの更新

**変更するファイル**: `cmd/yt2column/fixpr_docs_test.go`

- [x] `TestContextAndCommandsAgree` を設計書 3.11 の 1 つめの箇条のとおりに改める: 期待値を `chatgpt-codex-connector` にする、確かめる文書に `_lib/task-pr-handoff.md` を加える、許す文字列をなくす、`runpr.md` と `_lib/task-pr-handoff.md` が `<awaited-reviewer>`・`<review-request>`・`<review-mention>` の定義の文を持つことを確かめる。定義の文は、`awaitedReviewerSentence` と同じく定数で持ち、2 つの文書で同じ文を求める。
- [x] `PR review request` の行の検査を、設計書 3.11 の 2 つめの箇条のとおりに加える。値の最初の語が `fixprguard.CodexMention` と一致することも確かめる。`TestContextReviewerRowsMalformed` に、この行が欠ける・2 行ある・値の形が違う・最初の語が違う、の行を加える。
- [x] `TestRunprDocContract` を設計書 3.11 の 3 つめの箇条のとおりに改める（`no_body_fetch`、`runprStep3`、`reviewedCheck`、`lastReviewedRule`、`autoFixprCondition`）。`autoFixprCondition` のコメントの「the switch must leave it unchanged」は、条件を変えず語だけを改めたことが分かる文にする。
- [x] `TestRunprDocContract` に次の文を固定するサブテストを加える（設計書 7.3 の「停止と再実行」）。どの文を固定するかは書いた文書から選ぶ。
    -   規則 1〜6 と、断りのコメントの但し書き。規則の順序は `head_before_checkout` と同じく位置の比較で固定する。
    -   依頼したコミットの 2 つの祖先の判定（向きが依頼の種類で逆になる）、`0`・`1` 以外の終了コードの扱い、判定の前の `git fetch origin`。
    -   依頼のコメントの絞り込み（`user.type` が `User`、`login` が `gh api user` の値、本文が `<review-request>` と一致）。
    -   依頼の書き方（`gh pr comment <number> --body "<review-request>"`）が step 3 と step 4 にあること、最新の依頼が「待っている」か「判定できない」の間は投稿しないこと、待っていた依頼が完了した後に確かめ直して 1 回投稿すること。
    -   時間切れと想定外の投稿と判定できない状態で止まって step 4 に進まないこと、再実行で終わった依頼を依頼し直すこと。
    -   step 4 の PR レビューの質問が Codex を名指しすること。
- [x] `TestHandoffStartMarker`（同じファイル、新規）: `_lib/task-pr-handoff.md` Part B step 5 が、約 2 分間、約 30 秒おきに読むこと、要約コメントを `Bot` の種類と `<awaited-reviewer>[bot]` と 1 行目で絞ること、PR を開いた時点より後の書き換えを開始の印とすること、ないときに `<review-request>` で 1 回依頼すること、完了を待たないことを述べていることを確かめる（設計書 3.6・5.2）。範囲は step 5 の中に限る。

#### ステップ 2-6: 壊して失敗を確かめる

- [x] 次のそれぞれで、該当するガードが失敗することを確かめ、コミットメッセージに書く。
    -   設計書 7.2 の 2 つめの箇条のうち、このフェーズの確認に当たるもの（`runpr.md` に `@coderabbitai review` を戻す、`task-pr-handoff.md` に `chatgpt-codex-connector` を書く、`PR review request` の行を消す・2 行にする・最初の語を変える）。
    -   規則 1 と規則 2 の順序を入れ替える。
    -   祖先の判定の引数の順序を入れ替える。
    -   依頼のコメントの絞り込みから `User` の条件を消す。
    -   Part B step 5 から `Bot` の絞り込みを消す、「PR を開いた時点より後」を消す。
    -   2 つの文書の片方のプレースホルダーの定義の文を変える。

**フェーズ 2 の完了の条件**: `make test`・`make lint` が通る。

### フェーズ 3: 投稿の前のメンションの確認（設計書 3.8 の「投稿の前の確認」、AC-29）

#### ステップ 3-1: `fixpr.md`

**変更するファイル**: `.claude/commands/fixpr.md`

- [x] プレースホルダー `<review-mention>` の定義の文を、`runpr.md` と同じ文で加える（設計書 3.1）。
- [x] `/fixpr` のフェーズ 2・3 でエージェントに渡す返信の文面の規則と、`/fixpr` のフェーズ 6 でエージェントに渡す PR の説明の規則に、メンションを書かないこと（スレッドの本文を引用するときも写さないこと）を加える。指示はプレースホルダーで書く（設計書 3.8）。
- [x] `/fixpr` のフェーズ 5 とフェーズ 6 に、投稿の前の確認と、含まれていたときの扱いを、設計書 3.8 の表のとおりに加える。
- [x] `/fixpr` のフェーズ 5 の「ほかの止まり方」に終了コード `2` を含め、停止の種類の代わりに診断を挙げることを明記する（設計書 3.8 の箇条の `fixpr.md` フェーズ 5）。

**完了の条件**: ステップ 3-3 の確認が通る。

#### ステップ 3-2: `runpr.md` と `_lib/task-pr-handoff.md`

**変更するファイル**: `.claude/commands/runpr.md`、`.claude/commands/_lib/task-pr-handoff.md`

- [x] 1.4 の表の残りの 5 か所の節（`runpr.md` の step 1・step 4・step 5、`_lib/task-pr-handoff.md` の Part A step 4・Part B step 3）に、投稿の前の確認と、含まれていたときの扱いを設計書 3.8 の表のとおりに加える。`runpr.md` step 4 では、issue・コメントと返信の両方を対象にする。step 5 では、`--comment` に渡すコメントの文面を書いた時点で確かめる（1.4）。依頼のコメントは対象外であることを書く。

**完了の条件**: ステップ 3-3 の確認が通る。

#### ステップ 3-3: ガード

**変更するファイル**: `cmd/yt2column/fixpr_docs_test.go`、`cmd/yt2column/codex_review_docs_test.go`（新規）

- [x] `TestContextAndCommandsAgree` に、`fixpr.md` が `<review-mention>` の定義の文（`runpr.md` と同じ定数）を持つことを加える。
- [x] `TestCodexMentionOnlyInRequestRow`（新しいファイル）: 設計書 3.11 の新しいファイルの 1 つめの確認。`.claude/commands/` の下（`_lib/` を含む）のすべての文書と `CLAUDE.md` で、`@codex` が大文字と小文字を区別せずに現れるのは `_context.md` の `PR review request` の行だけであることを確かめる。
- [x] `TestPostingPathsCheckMention`（新しいファイル）: 1.4 の表の 7 か所の節のそれぞれが、投稿の前の確認の決まった文（定数で持つ）と、その節の「含まれていたときの扱い」（設計書 3.8 の表）を述べることを確かめる。各節で、確認の文がその節の投稿のコマンドより前にあることも、`head_before_checkout` と同じく位置の比較で確かめる。`runpr.md` step 4 は issue・コメントと返信の両方について確かめる。
- [x] `TestPostingCommandsInventory`（新しいファイル）: 新しい投稿の経路が確認なしに加わったら失敗する、棚卸しのテストである。`.claude/commands/` の下で GitHub に投稿するコマンドの呼び出し（コードのブロックの中の行と、バッククォートで囲んだ呼び出しのうち `--body`・`--body-file`・`--title`・`--comment` か `fixprguard reply`・`edit-pr` の引数を伴うもの）が、7 か所の節の中にあるか、依頼のコメントか、`mergepr.md` の squash のマージであることを確かめる。名前だけを挙げる文（1.4 の Authorization の節などの一致）は呼び出しとして数えない。どちらの形で数えるかは、今の文書で 1.4 の一致がすべて分類されること（呼び出しは 7 か所と例外、名前だけの文は数えない）を最初に確かめて決める。
- [x] `/fixpr` のフェーズ 2・3・6 のエージェントへの規則（メンションを書かない、引用でも写さない）と、`/fixpr` のフェーズ 5 の終了コード `2` の扱いの文を、`TestFixprDocContract` に固定する。

#### ステップ 3-4: 壊して失敗を確かめる

- [x] 次のそれぞれで、該当するガードが失敗することを確かめ、コミットメッセージに書く。
    -   `fixpr.md` に `@codex` を書く、`_lib/` の下の文書に `@Codex` を書く、`_context.md` のほかの行に書く、`CLAUDE.md` に書く。
    -   7 か所の節のそれぞれについて、確認の文を消す（7 回）。
    -   1 か所の節で、確認の文を投稿のコマンドの後に移す。
    -   `runpr.md` step 5 の扱い（コメントを付けずに閉じない）を消す。
    -   7 か所の外の節に `gh issue comment` の行を加える。
    -   `fixpr.md` の `<review-mention>` の定義を消す、`/fixpr` のフェーズ 2 の規則を消す。

**フェーズ 3 の完了の条件**: `make test`・`make lint` が通る。

### フェーズ 4: CodeRabbit の PR レビューの停止と、PR レビューを説明する文書（設計書 3.9・3.10）

#### ステップ 4-1: `.coderabbit.yaml`

**変更するファイル**: `.coderabbit.yaml`

- [x] 1 行目のスキーマで `reviews.auto_review.enabled` のキーと意味を確かめる（ネットワークの操作なので、実行の前に利用者の承認を得る）。
- [x] 設計書 3.9 のとおり、`reviews.auto_review` に `enabled: false` を加え、`auto_incremental_review: false` を消し、コメントを PR レビューを行わない理由の説明に改める。`pre_merge_checks.docstrings` とその説明は変えない。
- [x] 変更の後に `coderabbit review` を実行し、指摘か「指摘なし」の結果が返ってローカルの CLI レビューが動くことを確かめる（設計書 3.9・7.3）。動かなければ、実装を止めて利用者に報告する。

**完了の条件**: ローカルの CLI レビューが動く。ステップ 4-5 の `.coderabbit.yaml` の確認が通る。

#### ステップ 4-2: `AGENTS.md`・`CLAUDE.md`・コマンドの文書・用語集

**変更するファイル**: `AGENTS.md`、`CLAUDE.md`、`.claude/commands/mkreq.md`、`.claude/commands/fixpr.md`、`.claude/commands/_lib/coderabbit-local-review.md`、`docs/translation_glossary.md`

- [x] `AGENTS.md` の見出しを 1.5 の一覧のとおりに改め、節の最初に `CLAUDE.md` を参照する文を置く。今の箇条と `### Static guards …` の節は変えない（設計書 3.10）。
- [x] `CLAUDE.md` を設計書 3.10 の 1 つめの箇条のとおりに改める。ローカルの CLI レビューの記述（`:96` の「local CodeRabbit」、`:392-394`）は残す。
- [x] `mkreq.md:16`、`fixpr.md:86-89`・`557-558`、`_lib/coderabbit-local-review.md:83-84` を設計書 3.10 の箇条のとおりに改める。
- [x] `docs/translation_glossary.md` に、割り当て / quota、指摘 / finding、スレッド / thread の 3 行を設計書 3.10 のとおりに加える。

**完了の条件**: ステップ 4-3〜4-5 の確認が通る。

#### ステップ 4-3: 用語集のガード

**変更するファイル**: `cmd/yt2column/fixpr_docs_test.go`

- [x] `TestGlossaryTerms` に 3 つの用語を加え、割り当ての行が「枠」と区別していることを確かめる（設計書 3.10）。

#### ステップ 4-4: CodeRabbit の PR レビューが残らないことのガード

**変更するファイル**: `cmd/yt2column/codex_review_docs_test.go`

- [x] `TestNoCodeRabbitPRReview`: 設計書 3.11 の新しいファイルの 2 つめの確認（`@coderabbitai review`、`runpr.md` の `StatusContext` と `between [0-9a-f]{40}`、`CodeRabbit PR review`・`CodeRabbit's PR review` がないこと）に加えて、1.4 の「改めるもの」の各行で PR レビューを CodeRabbit として述べている今の文（「waits for CodeRabbit and CI」「the PR-side CodeRabbit review」「CodeRabbit has not reviewed」など）が残らないことを、`flat` で読んで確かめる。
- [x] `TestLocalCodeRabbitReviewKept`: 1.4 の「残すもの（ローカルの CLI レビュー）」のうち、`fixpr.md` フェーズ 4.5 の見出し、`_lib/task-pr-handoff.md` Part B step 1、`CLAUDE.md` の `coderabbit review` の項目が、CodeRabbit のローカルの CLI レビューとして残ることを確かめる（AC-24 の後半）。
- [x] `TestDocsDescribeCodexReview`: `runpr.md` と `_lib/pr-review-escalation.md` の `description`、`_lib/task-pr-handoff.md` の冒頭と Part B step 7、`CLAUDE.md` の `/runpr` の説明が PR レビューを Codex として説明すること（AC-24）、`CLAUDE.md` が Codex の設定（レビューのタイミングは PR 作成時のみ、徹底的なコードレビューは無効）を書いていること（AC-07）を確かめる。

#### ステップ 4-5: 設定・規則・観測の記録のガード

**変更するファイル**: `cmd/yt2column/codex_review_docs_test.go`、`docs/tasks/0031_codex_pr_review/observations.md`

- [x] `TestCodeRabbitConfigStopsReview`: `.coderabbit.yaml` の `reviews` → `auto_review` → `enabled: false` の並びを、テキストを受け取る純粋な関数で確かめる。標準ライブラリに YAML の解析はないので、字下げとキーで読み、コメントの行は飛ばす（`CLAUDE.md` の「Dependencies」）。本物のファイルの写しを 1 か所ずつ壊した表（`TestContextReviewerRowsMalformed` の形。壊さない対照の行を含む）でこの関数を確かめる。壊し方は、`enabled: false` をコメントにする、`true` にする、`reviews` の直下に移す、`pre_merge_checks` の下に移す、`auto_review` を 2 つにする、を含める。あわせて、コメントが PR レビューを行わない理由（Codex が行う、ローカルの CLI レビューは続ける）を述べることを確かめる（AC-22 の 3 つめの文）。
- [x] `TestAgentsCodeReviewRules`: `AGENTS.md` に `## Code Review Rules` の見出しがちょうど 1 つあり、`## Review guidelines` がなく、`sectionByHeading` で取り出したその節の中で `CLAUDE.md` を参照することを確かめる（AC-26。`AGENTS.md:3` の参照は節の外なので数えない）。
- [x] `observations.md` の PR での観測のうち、依頼したコミットの SHA がないもの（たとえば項目 3・5・6・8 の PR での観測）を洗い出し、GitHub の記録から SHA を補う。記録は読み取りだけの `gh api` で取り出す（ネットワークの操作なので、実行の前に利用者の承認を得る）。SHA を決められない観測には、その理由を項目に書く。観測の内容と結論は変えない。
- [x] `TestCodexObservationRecord`: `observations.md` の「AC-01 の各項目」の項目 1〜9 がそれぞれ、日付と根拠（PR の番号、`gh api`、URL のいずれか）と、観測の結果を述べる文を持つこと（AC-01・AC-02）、PR での観測のそれぞれが PR の番号と、依頼したコミットの SHA（または SHA を決められない理由）を持つこと（AC-02）、項目 3 が GraphQL の `__typename`・`login` と REST の `type`・`login` の値を持つこと（AC-03。今の記録は表の形で書いている）、項目 1 が指摘 0 件と 1 件以上の両方の観測を持つこと（AC-04）、「AC-05 に当たる点」の節があること（AC-05）を確かめる。

#### ステップ 4-6: 壊して失敗を確かめる

- [x] 次のそれぞれで、該当するガードが失敗することを確かめ、コミットメッセージに書く。
    -   設計書 7.2 の 2 つめの箇条の残り（`AGENTS.md` の見出しを戻す、`.coderabbit.yaml` の `enabled: false` を消す、用語集の行を消す）。
    -   「waits for CodeRabbit and CI」を `CLAUDE.md` に戻す、「the PR-side CodeRabbit review」を戻す。
    -   `fixpr.md` フェーズ 4.5 の見出しの CodeRabbit を Codex に変える。
    -   `AGENTS.md` の節の中の `CLAUDE.md` の参照を消す（`:3` は残す）。
    -   `CLAUDE.md` から Codex の設定の文を消す。
    -   観測の記録の項目 3 の表から REST の `type` の値を消す。
    -   観測の記録の項目 8 の結論の文を消す。PR での観測の 1 つから依頼したコミットの SHA を消す。

**フェーズ 4 の完了の条件**: `make test`・`make lint` と、文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）が通る。

### PR-2 作成ポイント: switch PR review to Codex

**対象ステップ**: 2-0 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6

**推奨タイトル**: `feat(0031): PR レビューを CodeRabbit から Codex に切り替える`

**レビュー観点**: `runpr.md` step 3 の依頼の状態の規則（設計書 3.3）と停止・再実行が、設計書のとおりに手順として書かれていること / 完了の印の受け入れる形（著者の絞り込み、1 行目の一致、状態の行の形、`has_any_findings`）と祖先の判定の向きが正しいこと / 7 か所の投稿の節すべてに投稿の前の確認があること / CodeRabbit の PR レビューの依頼と待ちが残らず、ローカルの CLI レビューの記述は残っていること / PR の文面とコミットメッセージにメンションを書いていないこと（1.2）。`/runpr` は PR-2 のブランチを checkout してから実行する（4.2）

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-3 は依頼の状態の規則（設計書 3.3）とレビュー済みの判定というセキュリティの境界（要件書 4.2）を含み、PR-2 全体が PR レビューの移行（mkplan の panel mode の「security-gate / migration」）に当たる。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/454
- [x] PR がマージされた
- [-] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン

### 3.1 マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: ツールの拒否 | PR-1（フェーズ 1） | PR-1 のマージと、利用者の `make install-fixprguard`（ステップ 2-0 で確かめる） |
| M2: 判定の切り替え | フェーズ 2 | フェーズ 2 の完了の条件 |
| M3: 投稿の前の確認 | フェーズ 3 | フェーズ 3 の完了の条件 |
| M4: 切り替えの PR | PR-2（フェーズ 2〜4） | PR-2 の `/runpr` で 4.2 の確かめが済み、マージされ、#403 が閉じられる |

PR-1 を先にするのは、PR-2 の `/fixpr` を、メンションを拒否するツールで動かすためである（設計書 8 章）。CodeRabbit の停止と判定の切り替えは、同じ PR-2 に入れる（設計書 8 章）。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 | `fixprguard` の `reply`・`edit-pr` がメンションを含む文面を拒否する。テストとガイド（日英） | frontier-required |
| PR-2 | 2-0 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 | `_context.md` の値、`runpr.md`・`_lib/task-pr-handoff.md`・`_lib/pr-review-escalation.md` の判定と依頼、投稿の前の確認、`.coderabbit.yaml`・`AGENTS.md`・`CLAUDE.md`・用語集、文書のガード | frontier-required |

## 4. テスト戦略

### 4.1 ユニットテストと文書のガード

| 対象 | テスト | フェーズ |
|---|---|---|
| メンションの範囲の境界 | `internal/fixprguard/write_test.go::TestCheckPostText` | 1 |
| `reply`・`edit-pr` の 3 つの文面、GitHub に触れないこと、検査の順序 | `internal/fixprguard/write_test.go::TestReplyRejectsCodexMention`・`TestEditPRRejectsCodexMention` | 1 |
| 終了コード `2`、標準出力なし、診断に文面なし | `cmd/fixprguard/main_test.go::TestPostTextMentionExitsUsage` | 1 |
| ガイドの記述 | `cmd/fixprguard/docs_test.go::TestGuideDescribesMentionCheck` | 1 |
| `_context.md` の 2 つの行、プレースホルダーの定義、名前を書かないこと | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`・`TestContextReviewerRowsMalformed` | 2・3 |
| `runpr.md` step 3・4 の判定・依頼・停止の文 | `cmd/yt2column/fixpr_docs_test.go::TestRunprDocContract` | 2 |
| Part B step 5 の開始の印 | `cmd/yt2column/fixpr_docs_test.go::TestHandoffStartMarker` | 2 |
| `@codex` の場所、投稿の前の確認、投稿のコマンドの棚卸し | `cmd/yt2column/codex_review_docs_test.go::TestCodexMentionOnlyInRequestRow`・`TestPostingPathsCheckMention`・`TestPostingCommandsInventory`、`fixpr_docs_test.go::TestFixprDocContract` | 3 |
| CodeRabbit の PR レビューが残らないこと、ローカルの CLI レビューが残ること、Codex の説明 | `codex_review_docs_test.go::TestNoCodeRabbitPRReview`・`TestLocalCodeRabbitReviewKept`・`TestDocsDescribeCodexReview` | 4 |
| `.coderabbit.yaml`、`AGENTS.md`、観測の記録 | `codex_review_docs_test.go::TestCodeRabbitConfigStopsReview`・`TestAgentsCodeReviewRules`・`TestCodexObservationRecord` | 4 |
| 用語集 | `fixpr_docs_test.go::TestGlossaryTerms` | 4 |

テストの名前は実装の際に変えてよい。変えたときは、同じ PR で本表と 7 章の表を改める。

### 4.2 結合テストと手動の確かめ

設計書 7.3 のとおりである。自動のテストは作らない。結果は、PR-2 の確かめは PR-2 の説明に、切り替えの後の確かめは #403 のコメントに書く。

-   **切り替えの PR 自身（PR-2）:** PR-2 のブランチを checkout し、その新しい文書で `/runpr` を実行する（`main` の古い `runpr.md` は CodeRabbit を待つ）。PR を開いたときの Codex の自動レビュー、完了の印、LAST_REVIEWED を確かめる（AC-09・AC-10・AC-17）。`/fixpr` の push の後に Codex のレビューが走らないことを見る（AC-07）。CodeRabbit のステータスコンテキストとチェックランを読み、`gh pr checks --watch` が終わることを確かめる（設計書 3.9・5.3）。CodeRabbit が PR-2 をレビューしたかと、それがどちらのブランチの設定によるかを記録する（PR-2 の base の `main` は自動レビューが有効なので、レビューされても AC-22 の反証にはならない）。
-   **停止と再実行（AC-12〜AC-15）:** 文書のガードで記述を固定し（ステップ 2-5）、利用者が PR-2 を確認するときに設計書 3.3・4 章と読み合わせる。あわせて、利用者の承認を得て、PR-2 で利用者が依頼のコメントを手で投稿してから `/runpr` を再実行し、重ねて依頼しないことを確かめる（AC-15。割り当てを 1 回使う）。
-   **ローカルの CLI レビュー:** ステップ 4-1 の 3 つめの作業。
-   **切り替えの後の最初の PR:** Part B step 5 が開始の印を見て依頼しないこと（または依頼すること）（AC-06）と、CodeRabbit の PR レビューが走らないこと（AC-22）を確かめる（9 章）。

### 4.3 後方互換性

-   PR-1 だけをマージした時点では、文書は今のままで、振る舞いの違いはメンションを含む返信と PR の説明が拒否されることだけである（設計書 8 章）。`--protocol 2` は変わらない。
-   ローカルの CLI レビューの手順は変えない（要件書 4.4）。ステップ 4-1 で動くことを確かめ、`TestLocalCodeRabbitReviewKept` で記述を固定する。

## 5. リスク管理

| リスク | 影響 | 対策 |
|---|---|---|
| 利用者が PR-1 の後に再インストールを忘れる | PR-2 の `/fixpr` が、裏の守りのないツールで動く | ステップ 2-0 で `vcs.revision` と `vcs.modified` を確かめる。主な守り（投稿の前の確認）はフェーズ 3 で入る |
| PR の文面やコミットメッセージにメンションが入る | PR-1 では Codex を動かしうる。PR-2 では、フェーズ 3 の投稿の前の確認が PR の作成や説明の書き換えを止める | 1.2 の規則。PR-1・PR-2 の `レビュー観点` に挙げる |
| `auto_review.enabled: false` がローカルの CLI レビューを止める | ローカルの CLI レビューが使えなくなる | ステップ 4-1 で `coderabbit review` を実行し、動かなければ止めて報告する |
| CodeRabbit が `PENDING` のステータスを残し、`gh pr checks --watch` が終わらない | `/runpr` が待ち続ける | PR-2 の `/runpr` で確かめる（4.2）。起きたら利用者が止め、設計書 5.3 のとおり扱いを決める |
| 切り替えの PR 自身の `/runpr` が、新しい判定の誤りで止まり続ける | PR-2 をマージできない | 設計書 4 章の「止まり続けるとき」のとおり、GitHub の画面で Codex のレビューを確かめ、`/mergepr` で直接マージする |
| 並行するタスク（0034・0036）が `fixprguard` のガイドやガードを変える | PR-1・PR-2 のリベースで衝突する | 各 PR の作業の前に `origin/main` を取り込む。衝突したガイドの節は、両方の変更を残して日英をそろえる |
| `main` を取り込んでいないブランチの、開いている PR | その `/runpr` が CodeRabbit のレビューを待って止まる | PR-2 のマージの後、開いている PR に `main` を取り込む（9 章、設計書 8 章） |
| ステップ 2-3 の文書が長くなり、設計書の規則の一部が落ちる | 判定が設計書と食い違う | ステップ 2-3 の完了の条件で読み合わせ、ステップ 2-5 のガードで規則と順序を固定し、PR-2 の `レビュー観点` に挙げる |
| 日程: PR-2 が大きく、レビューと `/runpr` の確かめに時間がかかる | #403 を閉じるのが遅れる | フェーズごとにコミットを分け、レビューはフェーズの単位で読めるようにする。PR-1 は先にマージでき、PR-2 を待たずに守りが働く |

## 6. 実装チェックリスト

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6）
- [x] 利用者が `make install-fixprguard` を実行した（ステップ 2-0 で確かめた）
- [x] PR-2 マージ済み（対象ステップ: 2-0 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6）
- [ ] PR-2 のブランチで `/runpr` を実行し、4.2 の切り替えの PR 自身の確かめを済ませた
- [ ] 切り替えの後の最初の PR で、4.2 の確かめを済ませた
- [ ] #403 を閉じた

## 7. 受け入れ基準の検証

種別は、`test`（実行され、誤った振る舞いで失敗する）、`static`（ガードのテストか `make` の対象）、`manual`（PR での観察、読み合わせ）である。

**例外の記録。** AC-06・AC-08〜AC-15・AC-17 は手順の振る舞いだが、手順はコマンドの文書（エージェントへの指示）にあり、Go のテストで動かせない（設計書 7 章・要件書 4.2 の「センチネルエラー」）。そのため、これらの `static` は手順の記述の固定であり、振る舞いは `manual` の確かめで補う。タスク 0038 で手順の記述を固定する確認を削除したので、AC-09・AC-11・AC-15 の `static` は `jq` で実行する一覧のフィルタの確認だけになり、ほかの手順の記述は PR のレビューで読み合わせる。AC-05 は手続きの条件（設計を始めない）なので、手続きの記録を根拠とする。

| AC | 実装のステップ | 検証 | 種別 |
|---|---|---|---|
| AC-01 | なし（観測は実施済み） | 観測の記録の項目 1〜9 と各項目の日付・根拠・観測の結果を、PR のレビューで読み合わせる（タスク 0038 で `TestCodexObservationRecord` を削除した） | manual |
| AC-02 | 4-5（依頼したコミットの SHA を補う） | 同上（日付と根拠。PR での観測の PR の番号と、依頼したコミットの SHA または決められない理由） | manual |
| AC-03 | なし（同上） | 同上（項目 3 の 4 つの値） | manual |
| AC-04 | なし（同上） | 同上（指摘 0 件と 1 件以上の観測） | manual |
| AC-05 | なし（同上） | 同上（「AC-05 に当たる点」の節）。要件書の承認（PR #407 と Comments の再承認）の後に設計書が書かれ承認されたこと（各文書の Document Status） | manual |
| AC-06 | 2-4、2-3 | `_lib/task-pr-handoff.md` の開始の印と `runpr.md` の「始まらなかった」の規則を、PR のレビューで読み合わせる（タスク 0038 で `TestHandoffStartMarker` と `TestRunprDocContract` のその確認を削除した）。切り替えの後の最初の PR（4.2） | manual |
| AC-07 | 4-2 | `CLAUDE.md` の Codex の設定の記述を、PR のレビューで読み合わせる（タスク 0038 で `TestDocsDescribeCodexReview` を削除した）。PR-2 の `/fixpr` の push の後に Codex のレビューが走らないこと（4.2） | manual |
| AC-08 | 2-2、2-3 | step 3 と step 4 の依頼の書き方を、PR のレビューで読み合わせる（タスク 0038 で `TestRunprDocContract` のその確認と `TestNoCodeRabbitPRReview` を削除した） | manual |
| AC-09 | 2-3 | `fixpr_docs_test.go::TestRunprDocContract/listings_print_no_body`（要約のコメントの一覧を `jq` で実行し、`has_any_findings` が真偽値でない `completed` を `indeterminate` とすることを含めて、状態の導き方を確かめる）。完了の印の規則の文は PR のレビューで読み合わせる（タスク 0038 で `reviewed_check` を削除した）。PR-2 の `/runpr`（4.2） | static、manual |
| AC-10 | 2-3 | 祖先の判定の向きと LAST_REVIEWED の記述を、PR のレビューで読み合わせる（タスク 0038 で `TestRunprDocContract` のその確認を削除した）。PR-2 の `/runpr`（4.2） | manual |
| AC-11 | 2-3 | `fixpr_docs_test.go::TestRunprDocContract/listings_print_no_body`（`Bot` と `<awaited-reviewer>[bot]` の絞り込み、1 行目の一致を、一覧を `jq` で実行して確かめる）。PR-2 の確認の読み合わせ（4.2） | static、manual |
| AC-12 | 2-3 | 規則 3 と断りのコメントの但し書き、規則の順序、停止の文を、PR のレビューで読み合わせる（4.2。タスク 0038 で `TestRunprDocContract` のその確認を削除した） | manual |
| AC-13 | 2-3 | 規則 4 と停止の文を、PR のレビューで読み合わせる（4.2。タスク 0038 で `TestRunprDocContract` のその確認を削除した） | manual |
| AC-14 | 2-3 | 止まって step 4 に進まない文を、PR のレビューで読み合わせる（4.2。タスク 0038 で `TestRunprDocContract` のその確認を削除した） | manual |
| AC-15 | 2-3 | `fixpr_docs_test.go::TestRunprDocContract/listings_print_no_body`（依頼のコメントの絞り込みを、一覧を `jq` で実行して確かめる）。「待っている」の間は依頼しない文と、終わった依頼を依頼し直す文は、PR のレビューで読み合わせる（タスク 0038 でその確認を削除した）。PR-2 での手の依頼の後の再実行（4.2） | static、manual |
| AC-17 | 2-2、2-3、2-4 | LAST_REVIEWED の定め、step 4 の質問、エスカレーションの説明を、PR のレビューで読み合わせる（タスク 0038 で `TestRunprDocContract` のその確認と `TestDocsDescribeCodexReview` を削除した）。PR-2 の `/runpr`（4.2） | manual |
| AC-22 | 2-3、2-4、4-1 | `codex_review_docs_test.go::TestCodeRabbitConfigStopsReview`。コマンドの文書に CodeRabbit の PR レビューの手順が残らないことは、PR のレビューで読み合わせる（タスク 0038 で `TestNoCodeRabbitPRReview` を削除した）。切り替えの後の最初の PR で CodeRabbit の PR レビューが走らないこと（4.2） | static、manual |
| AC-24 | 2-2、2-4、4-2 | コマンドの文書と `CLAUDE.md` の PR レビューとローカルの CLI レビューの説明を、PR のレビューで読み合わせる（タスク 0038 で `TestNoCodeRabbitPRReview`・`TestDocsDescribeCodexReview`・`TestLocalCodeRabbitReviewKept` を削除した） | manual |
| AC-25 | 2-1、2-2、2-4 | `fixpr_docs_test.go::TestContextAndCommandsAgree`・`TestContextReviewerRowsMalformed` | static |
| AC-26 | 4-2 | `codex_review_docs_test.go::TestAgentsCodeReviewRules`（見出しがちょうど 1 つあること）。節が `CLAUDE.md` を写さずに参照することは、PR のレビューで読み合わせる（タスク 0038 でその確認を削除した） | static、manual |
| AC-27 | 4-2 | 用語集の 3 つの用語の行を、PR のレビューで読み合わせる（タスク 0038 で `TestGlossaryTerms` を削除した） | manual |
| AC-28 | すべて | `make test`、`make lint`、CI の `doc-guard` のジョブ（PR-1・PR-2 の CI） | static、manual |
| AC-29 | 1-1〜1-5、3-1〜3-3 | `internal/fixprguard/write_test.go::TestCheckPostText`・`TestReplyRejectsCodexMention`・`TestEditPRRejectsCodexMention`、`cmd/fixprguard/main_test.go::TestPostTextMentionExitsUsage`、`codex_review_docs_test.go::TestCodexMentionOnlyInRequestRow`・`TestPostingPathsCheckMention`・`TestPostingCommandsInventory`、`fixpr_docs_test.go::TestContextAndCommandsAgree`（`PR review request` の最初の語と `fixprguard.CodexMention` の一致）。エージェントへの規則は、PR のレビューで読み合わせる（タスク 0038 で `TestFixprDocContract` のその確認を削除した） | test、static、manual |

AC-16・AC-18〜AC-21・AC-23 は要件書で削除された欠番である。

## 8. 成功基準

-   **機能:** 7 章のすべての AC の `test`・`static` の検証が通り、`manual` の確かめを 4.2 の記録の場所に書いている。
-   **品質:** `make test`・`make lint` と CI の `doc-guard` のジョブが通る。各ガードと各テストは、壊して失敗することを確かめ、コミットメッセージに書いている（ステップ 1-6・2-6・3-4・4-6）。
-   **セキュリティ:** メンションを含む文面が、ツールの拒否（PR-1）と投稿の前の確認（PR-2）の両方で止まる。完了の印は、設計書 3.2 の受け入れる形のものだけを使う。
-   **文書:** 日英の `fixprguard` のガイドが同じ内容を述べ、用語集に 3 つの用語がある。

## 9. 次のステップ

-   切り替えの後に最初に開く PR で、4.2 の「切り替えの後の最初の PR」の確かめを行い、#403 のコメントに書く。
-   開いている PR のうち `main` を取り込んでいないものに、`main` を取り込む（5 章）。
-   最後の PR のマージの後、`/runpr` が #403 を閉じる（要件書 2.4）。
-   設計書 9 章の将来の拡張（`coderabbitai` を自動レビュアーの一覧から外す、エスカレーションの方針の見直しなど）は、別の issue で扱う。#394 をどう閉じるかも、本タスクでは決めない（要件書 2.3）。
