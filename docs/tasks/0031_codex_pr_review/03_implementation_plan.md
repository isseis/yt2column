# 実装計画書：PR レビューを CodeRabbit から Codex の GitHub 連携に切り替える

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-22 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の、H-NN は [design_handoff.md](design_handoff.md) の項番を指す。設計書の「n.n」は設計書の節を指す。既存のファイルに関する記述は、コミット `419dec0` のファイルで確かめた。`file:line` はこのコミットの行番号を指す。設計書が確かめたコミット `3ed049b` から `419dec0` までの差分は `docs/tasks/` の下だけなので（`git diff --stat 3ed049b 419dec0`。2026-10-22 に実行）、設計書の行番号もそのまま使える。

用語は設計書の冒頭の定義と同じ意味で使う（PR を開くコマンド、要約コメント、状態の行、開始の印、依頼の文面とメンション、想定外の投稿、判定できない状態、依頼したコミット、文書のガードなど）。

## 1. 実装の概要

### 1.1 目的

-   `fixprguard` の `reply`・`edit-pr` が、`@codex` のメンションを含む本文とタイトルを拒否するようにする（設計書 3.8 の「ツールの拒否」。PR-1）。
-   コマンドの文書を、Codex の PR レビューを待ってレビュー済みを判定する手順に切り替え、CodeRabbit の PR レビューを止める（設計書 3.1・3.5〜3.11。PR-2、要件書の「切り替えの PR」）。

### 1.2 実装の原則

-   設計書の内容を本書で繰り返さない。各ステップは、設計書のどの節を実装するかを示し、作業と確かめることだけを書く。
-   レビュー済みの判定はコマンドの文書の手順であり、Go のテストで動かせない（設計書 7 章）。手順の記述は文書のガードで固定し、振る舞いは切り替えの PR 自身の `/runpr` で確かめる。
-   文書のガードは、本物の文書の写しを 1 か所壊すと失敗することを、実装者が壊して確かめ、コミットメッセージに書く（`CLAUDE.md` の「Every test must be able to fail for its stated reason」、設計書 7.2）。
-   Go のコメント・識別子・文字列リテラルは英語で書く。例外は、日本語の文書の見出しや表のセルと一致させるガードのテストのリテラルだけである（`CLAUDE.md` の「Go Idioms」）。
-   `fixprguard_guide` は日本語版を先に書き、英語版はその翻訳として書く（`CLAUDE.md` の「Translation Guidelines」）。

### 1.3 計画の際に決めたこと（2026-10-22）

設計書が決めていない、計画の範囲の選択はなかった。PR の分け方（PR-1 と PR-2）と順序は設計書 8 章が決めている。そのため、利用者への質問はしていない。次は、本書を書く際に決めたことである。

-   **PR-2 を 3 つのフェーズに分ける（フェーズ 2〜4）。** PR は 1 つのままとし、各フェーズの終わりで `make test`・`make lint` が通る単位に分ける。理由: `TestContextAndCommandsAgree`（`cmd/yt2column/fixpr_docs_test.go:527-570`）は、`Awaited PR reviewer` の値と、`runpr.md` が名前を書いてよい唯一の文字列 `"@" + 名前 + " review"`（同 `549`）を結びつけている。そのため、`_context.md` の値の変更と `runpr.md` step 3 の書き換えは、同じフェーズに入れないと途中でガードが落ちる。投稿の前の確認（設計書 3.8）と、PR レビューを名指しするその他の文書（設計書 3.9・3.10）は、判定の切り替えと独立に書けるので、別のフェーズにしてレビューしやすくする。
-   **AC-01〜AC-05 の検証は、観測の記録の構造を確かめるガードにする。** 観測は設計の前に済み、[observations.md](observations.md) に記録されている。実装の作業はないが、各 AC に実行される検証を置くため、記録が AC-01 の 1〜9 の項目と「AC-05 に当たる点」の節を持つことを確かめる確認を、新しいガードのファイルに加える（ステップ 4-4）。
-   **設計書 3.11 の確認に、次の 3 つを加える。** どれも設計書の確認と同じファイルに置き、設計書と矛盾しない。
    -   `fixprguard` のガイド（日英）の 7.1・7.2 節が、メンションの定数の値を名指しすること（ステップ 1-5）。ガイドの記述をツールの実装に結びつけるためである。
    -   投稿の前の確認（設計書 3.8 の表の 7 つの経路）が、それぞれの節に書かれていること（ステップ 3-3）。設計書 3.11 は文書の `@codex` の場所を確かめるが、確認の手順そのものは固定しないためである。
    -   `CLAUDE.md` が Codex の設定（レビューのタイミングは PR 作成時のみ、徹底的なコードレビューは無効）を書いていること（ステップ 4-4）。AC-07 の文書の側に、実行される検証を置くためである。

### 1.4 既存コード調査結果

**`internal/fixprguard`（PR-1）**

-   `Reply` は、引数を検査した後、`readTextFile` で本文を読み（`internal/fixprguard/write.go:108`）、次の `resolveRepository` で初めて GitHub に触れる（同 `112`）。`EditPR` は、`readTitleFile`（同 `275`）と `readTextFile`（同 `279`）の後に `gh pr edit` を呼ぶ（同 `283`）。`checkPostText` はこの読み取りの直後に置ける。`readTextFile` の契約のコメント（同 `20-27`）は変えない（設計書 3.8）。
-   `ErrInvalidInput` のコメントは「a malformed flag value」だけを挙げ（`internal/fixprguard/errors.go:12-13`）、すでに `readTextFile` の拒否（同 `write.go:31-46`）を含んでいない。ステップ 1-1 で直す。
-   `ErrInvalidInput` を包んだエラーは、`cmd/fixprguard/main.go:287-292` の `stop` が終了コード `2` にし、標準出力に何も書かない。新しいコードは要らない。
-   テストで再利用するもの: 本文のファイルを作る `writeTextFile`（`internal/fixprguard/write_test.go:21`）、手順を持たない fake で「どの `gh` の呼び出しもテストを落とす」形の `TestReplyRejectsInputBeforeAnyCall`（同 `271`）と `TestEditPRVerbatim` の `rejects_before_any_call`（同 `371-386`）、受け付ける場合の手順を作る `replyLookupSteps`・`writeSteps`・`replyRequest`（同 `106`・`130`・`141`）と `editStep`（同 `348`）。`cmd/fixprguard` の側は `newTestRun`（`cmd/fixprguard/test_helpers.go:151`）と `validArgs`（`cmd/fixprguard/main_test.go:24`）。新しいテストの補助のファイルは要らない。
-   既存のテストの `trickyTexts`（`write_test.go:306`）を含め、テストに `@codex` を含む文面はない（`git grep -n -i "@codex" -- ':!docs/tasks'` は 0 件。2026-10-22、`419dec0` で実行）。既存のテストは、拒否の追加で落ちない。

**`fixprguard` のガイド（PR-1）**

-   日英のガイドの 7.2 節の最後の項目は「本文の内容。ツールはファイルの種類・大きさ・文字コードだけを検査する」と述べる（`docs/dev/developer_guide/fixprguard_guide.ja.md:259`、英語版の同じ行）。この項目を固定するガードはない（`cmd/fixprguard/docs_test.go` に該当する文字列はない）。
-   9 章は、`cmd/fixprguard/docs_test.go:305-308` が `Protocol` の但し書き（`tc.proviso`）を含むことを確かめている。但し書きを加えても、既存の文を残せば落ちない。

**コマンドの文書が GitHub に投稿する経路（PR-2）**

`git grep -n -E "gh (issue (create|comment|close|edit)|pr (comment|create|edit|review|merge))|-X POST|--method (POST|PATCH)|fixprguard (reply|edit-pr)" -- .claude/commands` を `419dec0` で実行した結果（2026-10-22）、投稿の経路は次のとおりで、設計書 3.8 の表の 7 つと一致した。`mergepr.md:147` は squash のマージで、設計書 3.8 が対象外とする。`_context.md:98` は説明の文である。

| 経路 | 場所 |
|---|---|
| スレッドへの返信 | `fixpr.md:596`（フェーズ 5） |
| PR のタイトルと本文の書き換え | `fixpr.md:643`（フェーズ 6） |
| PR の作成 | `runpr.md:111`（step 1）、`_lib/task-pr-handoff.md:166`（Part B step 3） |
| deferred のスレッドを移す issue・コメント、スレッドへの返信 | `runpr.md:394-395`・`404`（step 4） |
| issue を閉じるコメント | `runpr.md:470`（step 5） |
| link sub-issue の本文 | `_lib/task-pr-handoff.md:142-143`（Part A step 4） |
| 依頼のコメント（確認の対象外） | `runpr.md:244`、`_lib/task-pr-handoff.md:190` |

**CodeRabbit の PR レビューを名指しする記述（PR-2）**

`git grep -n -i "coderabbit" -- .claude/commands CLAUDE.md AGENTS.md .coderabbit.yaml` を `419dec0` で実行し（2026-10-22）、ローカルの CLI レビューを指すもの（`_lib/coderabbit-local-review.md` 全体、`_lib/triage-pattern.md:7`、`runplan.md` の step 7.7 とその周辺、`task-pr-handoff.md:17`・`157-158`・`178`、`runpr.md:9-10`・`102`、`fixpr.md:553`・`560`、`CLAUDE.md:96`・`394`）を除くと、次が残った。どれも設計書 3.5〜3.10 が変更の対象に挙げている。

-   `_context.md:186`（`Awaited PR reviewer` の値と説明）
-   `runpr.md:2`・`30`・`53-54`・`190-249`・`264`・`270`・`320-322`・`353`・`378`
-   `_lib/task-pr-handoff.md:20`・`45`・`55`・`186-191`・`197`
-   `_lib/pr-review-escalation.md:2`・`7-12`・`22`（`9` に `@coderabbitai review` がある）
-   `fixpr.md:86-89`・`557-558`
-   `mkreq.md:16`
-   `CLAUDE.md:56`・`86`・`99`・`104-106`
-   `.coderabbit.yaml:9-14`
-   `AGENTS.md:5`（見出し `## Review guidelines`）

`.claude/commands`・`CLAUDE.md`・`AGENTS.md` の外で CodeRabbit を名指しするのは、`.claude/settings.json:4`（`coderabbit review` の許可。ローカルの CLI レビュー）、`fixprguard_guide` の使用例の `--reviewer coderabbitai`（設計書 2.2 が変えないとする）、Go のテストの値（信頼の判定のテストの名前。変えない）だけである。`@codex` は、`docs/tasks/` の外のどこにも現れない（同じ日に `git grep -n -i "@codex" -- ':!docs/tasks'` で 0 件）。

**文書のガード（PR-2）**

-   `cmd/yt2column/fixpr_docs_test.go` で変えるもの: `runprStep3`（`:37`）、`awaitedReviewerSentence`（`:79-80`。文の場所は変わるが文は保てる）、`reviewedCheck`（`:85-90`）、`lastReviewedRule`（`:92-94`）、`autoFixprCondition`（`:98-106`）、`TestRunprDocContract` の `no_body_fetch`（`:323-333`）、`TestContextAndCommandsAgree`（`:527-570`）、`TestContextReviewerRowsMalformed`（`:572-628`）、`TestGlossaryTerms`（`:702-734`）。`TestFixprguardCallSites`（`:346-370`）は `runprStep3` を使うので、見出しの定数を変えればそのまま通る。
-   再利用する補助: `readDoc`・`markdownTables`・`findTable`・`tableRow`・`tableRowsWithFirstCell`・`sectionByHeading`・`containsToken`（`cmd/yt2column/docs_test.go:313`・`325`・`377`・`393`・`405`・`464`・`623`）、`flat`・`requireSection`・`requirePhrases`・`contextLine`（`cmd/yt2column/fixpr_docs_test.go:111`・`140`・`151`・`630`）、`repositoryRoot`（`cmd/yt2column/makefile_test.go:23`）。新しいガードのファイルもこれらを使い、補助のファイルは作らない。
-   `CLAUDE.md` を読むほかのガード（`cmd/fixprguard/docs_test.go:59`、`cmd/yt2column-server/docs_test.go:253-276`、`cmd/yt2column/extension_docs_test.go:91`、`cmd/yt2column/copied_transcript_docs_test.go:189`・`228`、`internal/pipeline/relay_guard_test.go:435`）が固定する文は、本タスクで変える行（`CLAUDE.md:56`・`86`・`96-106`）に含まれない。

**外部の前提**

-   `gh api` の `-i, --include`（応答のステータスの行とヘッダーを出す）、`--paginate`、`--jq` は、`gh api --help`（`gh` 2.96.0）で確かめた。`Date` ヘッダーが応答に含まれることは、ステップ 2-2 の実装の際に読み取りだけの呼び出しで確かめる（ネットワークの操作なので、本書を書く際には実行していない）。
-   `git merge-base --is-ancestor` は、祖先なら `0`、祖先でなければ `1`、知らないオブジェクトなら `128` を返した（`git` 2.50.1。2026-10-22 に `419dec0` と `3ed049b` と存在しない SHA で実行）。設計書 3.3 の「`0`・`1` 以外は依頼したコミットでない」と合う。
-   `go version -m <binary>` で、`go install` したツールの `vcs.revision` を読める（`fixprguard_guide` の 4 章の終了コード `2` の行がこの手順を案内している）。

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

- [ ] 設計書 4 章のとおり、公開の定数 `CodexMention`（値 `@codex`）と非公開のエラー値 `errCodexMention`（`ErrInvalidInput` を包む）を加える。コメントは設計書 4 章のコード片のとおり英語で書く。
- [ ] `ErrInvalidInput` のコメントを、変更前「`ErrInvalidInput rejects a malformed flag value: a login, a PR number, a thread or comment ID, a fingerprint, or an accept pair.`」から、変更後「`ErrInvalidInput rejects a malformed flag value (a login, a PR number, a thread or comment ID, a fingerprint, or an accept pair) or a text file reply or edit-pr refuses (its type, size, encoding, or content).`」に改める。

**完了の条件**: `make lint` が通る（`errCodexMention` はステップ 1-2 で使う。このステップだけで `unused` が出るなら、1-2 と同じコミットにする）。

#### ステップ 1-2: `checkPostText` と適用

**変更するファイル**: `internal/fixprguard/write.go`

- [ ] `checkPostText` を加える。拒否する条件と拒否しないものは設計書 3.8 の箇条のとおりで、比べ方は ASCII の範囲で大文字と小文字を区別しない（Unicode の正規化はしない）。
- [ ] `Reply` で、本文を `readTextFile` で読んだ直後、`resolveRepository` より前に、本文に適用する。
- [ ] `EditPR` で、タイトルと本文を読んだ後、`runGH` より前に、両方に適用する。
- [ ] `readTextFile`・`readTitleFile` には加えない（設計書 3.8）。

**完了の条件**: ステップ 1-3・1-4 のテストが通る。

#### ステップ 1-3: ユニットテスト

**変更するファイル**: `internal/fixprguard/write_test.go`

- [ ] `Reply` の本文、`EditPR` のタイトル、`EditPR` の本文のそれぞれについて、設計書 7.1 の拒否する入力（大文字と小文字の違い、行頭と文の途中、`@codex review`）が `errCodexMention` で拒否され（`errors.Is`）、手順を持たない fake で `gh` が一度も呼ばれないことを確かめるテストを加える。既存の `TestReplyRejectsInputBeforeAnyCall` と `rejects_before_any_call` の形を再利用する。
- [ ] 設計書 7.1 の受け付ける入力（`@` のない `Codex`・`codex`、`@coderabbitai`、全角の `＠codex`）を、`Reply` と `EditPR` が投稿まで進めることを確かめる。範囲の境界を `checkPostText` の単体の表で固定し、`Reply`・`EditPR` を通す受け付けの行は各 1 つに絞ってよい（`CLAUDE.md` の「Check for duplication before adding a table test for a helper」）。
- [ ] 拒否の値を、ほかの理由の拒否（空、大きさ、文字コード）と `errors.Is` で区別できることを確かめる。

#### ステップ 1-4: コマンドのテスト

**変更するファイル**: `cmd/fixprguard/main_test.go`

- [ ] メンションを含む本文で `reply` を、メンションを含むタイトルと本文で `edit-pr` を実行すると、終了コード `2`（`exitUsage`）で、標準出力に何も出ず、標準エラーの診断にファイルの内容が出ないことを確かめるテストを加える。`gh` の fake は手順を持たない。

#### ステップ 1-5: ガイド（日英）

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`、`docs/dev/developer_guide/fixprguard_guide.md`、`cmd/fixprguard/docs_test.go`

- [ ] 日本語版を設計書 3.8 の「ガイド」のとおりに改める: 5 章の `reply`・`edit-pr` の本文とタイトルの条件、4 章の終了コード `2` の説明、7.1 の保証の表に「メンションを含む文面を GitHub に送らない」行、7.2 の「本文の内容」の項目（内容はメンションだけを検査する。全角や幅のない文字を挟んだ形は検査しない）、9 章の `Protocol` の但し書き（設計書 1.4 の 2 行目）。9 章の既存の但し書きの文は残す。
- [ ] 英語版を、日本語版の翻訳として同じ構造で改める（`.claude/commands/mktrans.md` の原則と `docs/translation_glossary.md` に従う）。
- [ ] `cmd/fixprguard/docs_test.go` に、日英のガイドの 7.1 節と 7.2 節が `fixprguard.CodexMention` の値を含むことを確かめる確認を加える（1.3）。
- [ ] 書いた記述を実装と照らし合わせる: 拒否の条件、終了コード、拒否しないもの、検査の順序（GitHub に触れる前）が、ステップ 1-2 のコードと一致することを読み合わせる。

#### ステップ 1-6: 壊して失敗を確かめる

- [ ] `Reply` から `checkPostText` の呼び出しを外すと、ステップ 1-3 の `Reply` の拒否の行と、ステップ 1-4 の `reply` の行が失敗することを確かめる。
- [ ] `EditPR` のタイトルへの適用と本文への適用を、それぞれ外すと、該当する行が失敗することを確かめる。
- [ ] 比べ方を大文字と小文字を区別するものに変えると、大文字を含む行が失敗することを確かめる。
- [ ] ガイドの片方の 7.2 節から定数の値を消すと、ステップ 1-5 の確認が失敗することを確かめる。
- [ ] 以上をコミットメッセージに書く。

**フェーズ 1 の完了の条件**: `make fmt`・`make test`・`make lint` が通る。`Protocol` は上げない（設計書 1.4）。

### PR-1 作成ポイント: fixprguard mention rejection

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6

**推奨タイトル**: `feat(0031): fixprguard の返信と PR の説明で @codex のメンションを拒否する`

**レビュー観点**: 拒否が GitHub に触れる前に起こり、`reply` と `edit-pr` の 3 つの文面すべてに適用されていること / 拒否の範囲の境界（大文字と小文字、全角、`@` なし）がテストで固定されていること / `Protocol` を上げない判断と、ガイドの 9 章の但し書きが設計書 1.4 と一致すること / 日英のガイドの対応

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-2 は、利用者のアカウントの投稿が Codex を動かすのを防ぐセキュリティの守り（mkplan の panel mode の「security-gate」）に当たる。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 完了を待つ PR レビュアーと、レビュー済みの判定の切り替え（設計書 3.1〜3.7）

前提: PR-1 がマージされ、利用者が `make install-fixprguard` で再インストールしていること。

#### ステップ 2-0: 再インストールの確認

- [ ] `go version -m "$(command -v fixprguard)"` の `vcs.revision` が、PR-1 のマージのコミットか、その子孫であることを確かめる。違えば、利用者に再インストールを頼み、先に進まない（設計書 1.2・5.3 の「ツールの拒否は抜け道のない境界ではない」）。

#### ステップ 2-1: `_context.md` の値

**変更するファイル**: `.claude/commands/_context.md`

- [ ] 設計書 3.1 の表のとおり、`Awaited PR reviewer` の値を `chatgpt-codex-connector` にし、説明の文を改める（REST の名前の作り方、要約コメントの絞り込み、`_lib/task-pr-handoff.md` Part B step 5 も待つこと）。
- [ ] `PR review request` の行（値 `@codex review`）を、設計書 3.1 の表の説明の文とともに加える。行の形は `Awaited PR reviewer` の行と同じにする。

#### ステップ 2-2: `runpr.md` の判定の切り替え

**変更するファイル**: `.claude/commands/runpr.md`

- [ ] 設計書 3.5 の箇条のうち、「投稿の前の確認」を除くすべてを行う: `description` と冒頭、Authorization、冒頭のスレッドの読み方の文（設計書 1.4 の 3 行目の例外）、プレースホルダーの定義（設計書 3.1 の 3 つ）、step 3 の見出し（1.5 の一覧）と冒頭、「Reviewed」、「Requesting the review」、`fixpr` を再び実行する条件の語、CodeRabbit のステータスによる停止の削除、step 4 の PR レビューの質問と依頼。
- [ ] 「Reviewed」と「Requesting the review」は、設計書 3.2（読む値と受け入れる形、時点と時間の測り方、前提とする運用）、3.3（依頼の一覧、依頼したコミット、最新の依頼の状態の規則 1〜6、使わない投稿の表、断りのコメント、LAST_REVIEWED）、3.4（依頼を投稿する条件）、4 章（停止の表、止まり続けるとき）、6.1・6.2 の流れを、エージェントが従える手順として書く。値の読み方は設計書 3.2 の「読み方の制約」を守り、要約コメントは 1 行目と 2 行目だけを取り出す `--jq` の絞り込みで読む。
- [ ] `Date` ヘッダーを読む呼び出し（設計書 3.2）を、この PR の作業の中で、読み取りだけの `gh api -i` で一度実行し、`Date` の行が出ることを確かめる（ネットワークの操作なので、実行の前に利用者の承認を得る）。

#### ステップ 2-3: PR を開くコマンドとエスカレーション

**変更するファイル**: `.claude/commands/_lib/task-pr-handoff.md`、`.claude/commands/_lib/pr-review-escalation.md`

- [ ] `_lib/task-pr-handoff.md` を設計書 3.6 の箇条のうち、「投稿の前の確認」を除くすべてのとおりに改める（冒頭・AFTER_CREATE・Part B step 7 の語、Authorization、プレースホルダーの定義、Part B step 5）。
- [ ] `_lib/pr-review-escalation.md` を設計書 3.7 のとおりに改める。判断の基準と手順は変えない。`:9` の `@coderabbitai review` を消す。

#### ステップ 2-4: ガードの更新

**変更するファイル**: `cmd/yt2column/fixpr_docs_test.go`

- [ ] `TestContextAndCommandsAgree` を設計書 3.11 の 1 つめの箇条のとおりに改める: 期待値を `chatgpt-codex-connector` にする、確かめる文書に `_lib/task-pr-handoff.md` を加える、許す文字列をなくす、`runpr.md` と `_lib/task-pr-handoff.md` が `<awaited-reviewer>`・`<review-request>`・`<review-mention>` の定義の文を持つことを確かめる（`fixpr.md` の `<review-mention>` の定義はステップ 3-4 で加える）。
- [ ] `PR review request` の行の検査を、設計書 3.11 の 2 つめの箇条のとおりに加える。値の最初の語が `fixprguard.CodexMention` と一致することも確かめる。`TestContextReviewerRowsMalformed` に、この行が欠ける・2 行ある・値の形が違う・最初の語が違う、の行を加える。
- [ ] `TestRunprDocContract` を設計書 3.11 の 3 つめの箇条のとおりに改める（`no_body_fetch`、`runprStep3`、`reviewedCheck`、`lastReviewedRule`、`autoFixprCondition`）。加えて、最新の依頼の状態の規則（設計書 3.3 の規則 1〜6 と断りのコメントの但し書き）、依頼を投稿しない条件（最新の依頼が「待っている」か「判定できない」のとき）、時間切れと想定外の投稿で止まって step 4 に進まないこと、再実行で終わった依頼を依頼し直すこと、の文を固定する（設計書 7.3 の「停止と再実行」）。どの文を固定するかは書いた文書から選ぶ。
- [ ] `autoFixprCondition` のコメントの「the switch must leave it unchanged」を、条件は変えず CodeRabbit の語だけを Codex に改めたことが分かる文に改める（設計書 1.4 の 4 行目）。

#### ステップ 2-5: 壊して失敗を確かめる

- [ ] 設計書 7.2 の 2 つめの箇条のうち、このフェーズの確認に当たるもの（`runpr.md` に `@coderabbitai review` を戻す、`task-pr-handoff.md` に `chatgpt-codex-connector` を書く、`PR review request` の行を消す・2 行にする・最初の語を変える）と、ステップ 2-4 で固定した各文を 1 か所ずつ壊し、該当するガードが失敗することを確かめて、コミットメッセージに書く。

**フェーズ 2 の完了の条件**: `make test`・`make lint` が通る。

### フェーズ 3: 投稿の前のメンションの確認（設計書 3.8 の「投稿の前の確認」、AC-29）

#### ステップ 3-1: `fixpr.md`

**変更するファイル**: `.claude/commands/fixpr.md`

- [ ] プレースホルダー `<review-mention>` の定義の文を、`runpr.md` と同じ文で加える（設計書 3.1）。
- [ ] フェーズ 2・3 のエージェントに渡す返信の文面の規則と、フェーズ 6 のエージェントに渡す PR の説明の規則に、メンションを書かないこと（スレッドの本文を引用するときも写さないこと）を加える。指示はプレースホルダーで書く（設計書 3.8）。
- [ ] フェーズ 5 とフェーズ 6 に、投稿の前の確認と、含まれていたときの扱いを、設計書 3.8 の表のとおりに加える。
- [ ] フェーズ 5 の「ほかの止まり方」に終了コード `2` を含め、停止の種類の代わりに診断を挙げることを明記する（設計書 3.8 の箇条の `fixpr.md` フェーズ 5）。

#### ステップ 3-2: `runpr.md` と `_lib/task-pr-handoff.md`

**変更するファイル**: `.claude/commands/runpr.md`、`.claude/commands/_lib/task-pr-handoff.md`

- [ ] 設計書 3.8 の表の残りの経路（`runpr.md` の step 1・step 4・step 5、`_lib/task-pr-handoff.md` の Part A step 4・Part B step 3）に、投稿の前の確認と、含まれていたときの扱いを加える。依頼のコメントは対象外であることを書く。

#### ステップ 3-3: ガード

**変更するファイル**: `cmd/yt2column/fixpr_docs_test.go`、`cmd/yt2column/codex_review_docs_test.go`（新規）

- [ ] `TestContextAndCommandsAgree` に、`fixpr.md` が `<review-mention>` の定義の文を持つことを加える。
- [ ] 新しいファイルに、設計書 3.11 の新しいファイルの 1 つめの確認（`.claude/commands/` の下のすべての文書と `CLAUDE.md` で、`@codex` が大文字と小文字を区別せずに現れるのは `_context.md` の `PR review request` の行だけであること）を加える。下位のディレクトリ（`_lib/`）も読む。
- [ ] 同じファイルに、設計書 3.8 の表の 7 つの経路の節（`fixpr.md` のフェーズ 5・6、`runpr.md` の step 1・4・5、`_lib/task-pr-handoff.md` の Part A・Part B）が、それぞれ `<review-mention>` の確認を書いていることを確かめる確認を加える（1.3）。

#### ステップ 3-4: 壊して失敗を確かめる

- [ ] `fixpr.md` に `@codex` を書く、`_lib/` の下の文書に `@Codex` を書く、7 つの経路のうち 1 つの確認の文を消す、`fixpr.md` の `<review-mention>` の定義を消す、のそれぞれでガードが失敗することを確かめて、コミットメッセージに書く。

**フェーズ 3 の完了の条件**: `make test`・`make lint` が通る。

### フェーズ 4: CodeRabbit の PR レビューの停止と、PR レビューを説明する文書（設計書 3.9・3.10）

#### ステップ 4-1: `.coderabbit.yaml`

**変更するファイル**: `.coderabbit.yaml`

- [ ] 設計書 3.9 のとおり、`reviews.auto_review` に `enabled: false` を加え、`auto_incremental_review: false` を消し、コメントを PR レビューを行わない理由の説明に改める。`pre_merge_checks.docstrings` とその説明は変えない。
- [ ] 変更の後に `coderabbit review` を実行し、指摘か「指摘なし」の結果が返ってローカルの CLI レビューが動くことを確かめる（設計書 3.9・7.3）。設定のエラーで動かなければ、実装を止めて利用者に報告する。

#### ステップ 4-2: `AGENTS.md`・`CLAUDE.md`・`mkreq.md`・`fixpr.md`・用語集

**変更するファイル**: `AGENTS.md`、`CLAUDE.md`、`.claude/commands/mkreq.md`、`.claude/commands/fixpr.md`、`docs/translation_glossary.md`

- [ ] `AGENTS.md` の見出しを 1.5 の一覧のとおりに改め、節の最初に `CLAUDE.md` を参照する文を置く。今の箇条と `### Static guards …` の節は変えない（設計書 3.10）。
- [ ] `CLAUDE.md` を設計書 3.10 の 1 つめの箇条のとおりに改める（`:56` の Bot のコマンドの例を `_context.md` の `PR review request` を指す書き方に、`:86` の `/runpr` の説明と `:96-106` の PR の流れと割り当ての説明を Codex に、Codex の設定を利用者が行うリポジトリの外の設定として書く）。ローカルの CLI レビューの記述（`:96` の「local CodeRabbit」、`:392-394`）は残す。
- [ ] `mkreq.md:16` と `fixpr.md:86-89`・`557-558` を設計書 3.10 の箇条のとおりに改める。
- [ ] `docs/translation_glossary.md` に、割り当て / quota、指摘 / finding、スレッド / thread の 3 行を設計書 3.10 のとおりに加える。

#### ステップ 4-3: 既存のガードの更新

**変更するファイル**: `cmd/yt2column/fixpr_docs_test.go`

- [ ] `TestGlossaryTerms` に 3 つの用語を加え、割り当ての行が「枠」と区別していることを確かめる（設計書 3.10）。

#### ステップ 4-4: 新しいガードの残り

**変更するファイル**: `cmd/yt2column/codex_review_docs_test.go`

- [ ] 設計書 3.11 の新しいファイルの 2〜4 つめの確認を加える: CodeRabbit の PR レビューの依頼・判定・語が残らないこと（`@coderabbitai review`、`runpr.md` の `StatusContext` と `between [0-9a-f]{40}`、`CodeRabbit PR review`・`CodeRabbit's PR review`。文書の折り返しに左右されないよう `flat` で読む）、`.coderabbit.yaml` の `reviews.auto_review` に `enabled: false` があること、`AGENTS.md` の見出しと `CLAUDE.md` の参照。`.coderabbit.yaml` は標準ライブラリに YAML の解析がないので、字下げで `reviews` → `auto_review` → `enabled: false` の並びを確かめる（`CLAUDE.md` の「Dependencies」）。
- [ ] `runpr.md` と `_lib/pr-review-escalation.md` の `description`、`_lib/task-pr-handoff.md` の冒頭と Part B step 7、`CLAUDE.md` の `/runpr` の説明が、PR レビューを Codex として説明していることを確かめる（AC-24）。
- [ ] `CLAUDE.md` が Codex の設定（レビューのタイミングは PR 作成時のみ、徹底的なコードレビューは無効）を書いていることを確かめる（1.3、AC-07）。
- [ ] 観測の記録（`observations.md`）が、「AC-01 の各項目」の節に項目 1〜9 を、「AC-05 に当たる点」の節を持つことを確かめる（1.3、AC-01〜AC-05）。

#### ステップ 4-5: 壊して失敗を確かめる

- [ ] 設計書 7.2 の 2 つめの箇条の残り（`AGENTS.md` の見出しを戻す、`.coderabbit.yaml` の `enabled: false` を消す、用語集の行を消す）と、ステップ 4-4 の各確認を、本物の文書の写しを 1 か所壊して失敗することを確かめて、コミットメッセージに書く。`.coderabbit.yaml` の確認は、`enabled: false` を別の節（`pre_merge_checks` の下など）に移した写しでも失敗することを確かめる。

**フェーズ 4 の完了の条件**: `make test`・`make lint` と、文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）が通る。

### PR-2 作成ポイント: switch PR review to Codex

**対象ステップ**: 2-0 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5

**推奨タイトル**: `feat(0031): PR レビューを CodeRabbit から Codex に切り替える`

**レビュー観点**: `runpr.md` step 3 の依頼の状態の規則（設計書 3.3）と停止・再実行が、設計書のとおりに手順として書かれていること / 完了の印の受け入れる形（著者の絞り込み、1 行目の一致、状態の行の形、`has_any_findings`）が漏れなく書かれていること / 投稿の 7 つの経路すべてに投稿の前の確認があること / CodeRabbit の PR レビューの依頼と待ちが残らず、ローカルの CLI レビューの記述は残っていること

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-2 は依頼の状態の機械（設計書 3.3）とレビュー済みの判定というセキュリティの境界（要件書 4.2）を含み、PR-2 全体が PR レビューの移行（mkplan の panel mode の「security-gate / migration」）に当たる。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン

### 3.1 マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: ツールの拒否 | PR-1（フェーズ 1） | PR-1 のマージと、利用者の `make install-fixprguard`（ステップ 2-0 で確かめる） |
| M2: 判定の切り替え | フェーズ 2 | フェーズ 2 の完了の条件 |
| M3: 投稿の前の確認 | フェーズ 3 | フェーズ 3 の完了の条件 |
| M4: 切り替えの PR | PR-2（フェーズ 2〜4） | PR-2 の `/runpr` で 4.2 の結合の確かめが済み、マージされ、#403 が閉じられる |

PR-1 を先にするのは、PR-2 の `/fixpr` を、メンションを拒否するツールで動かすためである（設計書 8 章）。CodeRabbit の停止と判定の切り替えは、同じ PR-2 に入れる（同）。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 | `fixprguard` の `reply`・`edit-pr` がメンションを含む文面を拒否する。テストとガイド（日英） | frontier-required |
| PR-2 | 2-0 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 | `_context.md` の値、`runpr.md`・`_lib/task-pr-handoff.md`・`_lib/pr-review-escalation.md` の判定と依頼、投稿の前の確認、`.coderabbit.yaml`・`AGENTS.md`・`CLAUDE.md`・用語集、文書のガード | frontier-required |

## 4. テスト戦略

### 4.1 ユニットテストと文書のガード

| 対象 | テスト | フェーズ |
|---|---|---|
| メンションの拒否（`reply`・`edit-pr` の 3 つの文面、範囲の境界、GitHub に触れないこと） | `internal/fixprguard/write_test.go` の新しいテスト | 1 |
| 終了コード `2`、標準出力なし、診断にファイルの内容なし | `cmd/fixprguard/main_test.go` の新しいテスト | 1 |
| ガイドがメンションの定数を名指しすること | `cmd/fixprguard/docs_test.go` の新しい確認 | 1 |
| `_context.md` の 2 つの行、プレースホルダーの定義、名前を書かないこと | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`・`TestContextReviewerRowsMalformed` | 2・3 |
| `runpr.md` step 3・4 の判定・依頼・停止の文 | `cmd/yt2column/fixpr_docs_test.go::TestRunprDocContract` | 2 |
| `@codex` の場所、投稿の前の確認の経路 | `cmd/yt2column/codex_review_docs_test.go` | 3 |
| CodeRabbit の PR レビューが残らないこと、`.coderabbit.yaml`、`AGENTS.md`、`CLAUDE.md`、観測の記録 | `cmd/yt2column/codex_review_docs_test.go` | 4 |
| 用語集 | `cmd/yt2column/fixpr_docs_test.go::TestGlossaryTerms` | 4 |

新しいテストの関数の名前は、実装の際に決める。決めた名前は、7 章の表の「検証」の欄に書き入れる。

### 4.2 結合テストと手動の確かめ

設計書 7.3 のとおりである。自動のテストは作らない。

- **切り替えの PR 自身（PR-2）:** PR-2 のブランチの新しい文書で `/runpr` を実行し、PR を開いたときの Codex の自動レビュー、完了の印、LAST_REVIEWED を確かめる（AC-09・AC-10・AC-17）。CodeRabbit のステータスコンテキストとチェックランを読み、`gh pr checks --watch` が終わることを確かめる（設計書 3.9・5.3）。結果を PR の説明に書く。
- **ローカルの CLI レビュー:** ステップ 4-1 の 2 つめの作業。
- **新しい Part B step 5:** 切り替えの後に最初に開く PR で、開始の印を見て依頼しないこと（または依頼すること）を確かめる（AC-06。9 章）。
- **停止と再実行（AC-12・AC-13・AC-15）:** 文書のガードで記述を固定し（ステップ 2-4）、利用者が PR-2 を確認するときに設計書 3.3・4 章と読み合わせる。

### 4.3 後方互換性

-   PR-1 だけをマージした時点では、文書は今のままで、振る舞いの違いはメンションを含む返信と PR の説明が拒否されることだけである（設計書 8 章）。`--protocol 2` は変わらない。
-   ローカルの CLI レビューの手順は変えない（要件書 4.4）。ステップ 4-1 で動くことを確かめる。

## 5. リスク管理

| リスク | 影響 | 対策 |
|---|---|---|
| 利用者が PR-1 の後に再インストールを忘れる | PR-2 の `/fixpr` が、裏の守りのないツールで動く | ステップ 2-0 で `vcs.revision` を確かめる。主な守り（投稿の前の確認）はフェーズ 3 で入る |
| `auto_review.enabled: false` がローカルの CLI レビューを止める | ローカルの CLI レビューが使えなくなる | ステップ 4-1 で `coderabbit review` を実行し、動かなければ止めて報告する |
| CodeRabbit が `PENDING` のステータスを残し、`gh pr checks --watch` が終わらない | `/runpr` が待ち続ける | PR-2 の `/runpr` で確かめる（4.2）。起きたら利用者が止め、設計書 5.3 のとおり扱いを決める |
| 切り替えの PR 自身の `/runpr` が、新しい判定の誤りで止まり続ける | PR-2 をマージできない | 設計書 4 章の「止まり続けるとき」のとおり、GitHub の画面で Codex のレビューを確かめ、`/mergepr` で直接マージする |
| 並行するタスク（0034・0036）が `fixprguard` のガイドやガードを変える | PR-1・PR-2 のリベースで衝突する | 各 PR の作業の前に `origin/main` を取り込む。衝突したガイドの節は、両方の変更を残して日英をそろえる |
| `main` を取り込んでいないブランチの開いた PR | その `/runpr` が CodeRabbit のレビューを待って止まる | PR-2 のマージの後、開いている PR に `main` を取り込む（9 章、設計書 8 章） |
| ステップ 2-2 の文書が長くなり、設計書の規則の一部が落ちる | 判定が設計書と食い違う | ステップ 2-4 のガードで規則 1〜6 と停止の文を固定し、PR-2 の `レビュー観点` に挙げる |

## 6. 実装チェックリスト

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6）
- [ ] 利用者が `make install-fixprguard` を実行した（ステップ 2-0 で確かめた）
- [ ] PR-2 マージ済み（対象ステップ: 2-0 / 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5）
- [ ] PR-2 の `/runpr` で 4.2 の切り替えの PR 自身の確かめを済ませた
- [ ] #403 を閉じた

## 7. 受け入れ基準の検証

種別は、`test`（実行され、誤った振る舞いで失敗する）、`static`（ガードのテストか `make` の対象）、`manual`（PR での観察）である。テストの関数の名前が「（新規）」のものは、実装の際に決めた名前を書き入れる。

| AC | 実装のステップ | 検証 | 種別 |
|---|---|---|---|
| AC-01 | なし（観測は実施済み。`observations.md`） | `cmd/yt2column/codex_review_docs_test.go`（新規）の観測の記録の確認（ステップ 4-4） | static |
| AC-02 | なし（同上） | 同上 | static |
| AC-03 | なし（同上） | 同上 | static |
| AC-04 | なし（同上） | 同上 | static |
| AC-05 | なし（同上） | 同上（「AC-05 に当たる点」の節） | static |
| AC-06 | 2-3、2-2（step 3 の「始まらなかった」） | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`（`_lib/task-pr-handoff.md` の定義の文）、`codex_review_docs_test.go`（新規）の `@coderabbitai review` の不在。切り替えの後の最初の PR での観察（4.2） | static、manual |
| AC-07 | 4-2 | `codex_review_docs_test.go`（新規）の `CLAUDE.md` の Codex の設定の確認。設定そのものは利用者が行い、観測の記録の項目 2・9 で確かめた | static、manual |
| AC-08 | 2-2 | `fixpr_docs_test.go::TestRunprDocContract`（依頼の文）、`codex_review_docs_test.go`（新規）の `@coderabbitai review` の不在 | static |
| AC-09 | 2-2 | `fixpr_docs_test.go::TestRunprDocContract` の `reviewed_check`。PR-2 の `/runpr`（4.2） | static、manual |
| AC-10 | 2-2 | `fixpr_docs_test.go::TestRunprDocContract` の `reviewed_check`・LAST_REVIEWED の文。PR-2 の `/runpr`（4.2） | static、manual |
| AC-11 | 2-2 | `fixpr_docs_test.go::TestRunprDocContract` の `reviewed_check`（`Bot` と `<awaited-reviewer>[bot]` の絞り込み、1 行目の一致） | static |
| AC-12 | 2-2 | `fixpr_docs_test.go::TestRunprDocContract`（規則 3 と断りのコメントの但し書き、停止の文） | static |
| AC-13 | 2-2 | `fixpr_docs_test.go::TestRunprDocContract`（規則 4、停止の文） | static |
| AC-14 | 2-2 | `fixpr_docs_test.go::TestRunprDocContract`（止まって step 4 に進まない文） | static |
| AC-15 | 2-2 | `fixpr_docs_test.go::TestRunprDocContract`（「待っている」の間は依頼しない文、終わった依頼を依頼し直す文） | static |
| AC-17 | 2-2、2-3 | `fixpr_docs_test.go::TestRunprDocContract` の `lastReviewedRule`、`codex_review_docs_test.go`（新規）の `_lib/pr-review-escalation.md` の説明の確認。PR-2 の `/runpr`（4.2） | static、manual |
| AC-22 | 2-2、2-3、4-1 | `codex_review_docs_test.go`（新規）の CodeRabbit の PR レビューが残らない確認と `.coderabbit.yaml` の確認。PR-2 の `/runpr` で CodeRabbit の PR レビューが走らないこと（4.2） | static、manual |
| AC-24 | 2-2、2-3、4-2 | `codex_review_docs_test.go`（新規）の語の確認と `description` の確認 | static |
| AC-25 | 2-1、2-2、2-3 | `fixpr_docs_test.go::TestContextAndCommandsAgree`・`TestContextReviewerRowsMalformed` | static |
| AC-26 | 4-2 | `codex_review_docs_test.go`（新規）の `AGENTS.md` の確認 | static |
| AC-27 | 4-2 | `fixpr_docs_test.go::TestGlossaryTerms` | static |
| AC-28 | すべて | `make test`、`make lint`、CI の `doc-guard` のジョブ（PR-1・PR-2 の CI） | static、manual |
| AC-29 | 1-1〜1-5、3-1〜3-3 | `internal/fixprguard/write_test.go`（新規）・`cmd/fixprguard/main_test.go`（新規）のメンションの拒否、`codex_review_docs_test.go`（新規）の `@codex` の場所と 7 つの経路の確認、`fixpr_docs_test.go::TestContextAndCommandsAgree`（`PR review request` の最初の語と `fixprguard.CodexMention` の一致） | test、static |

AC-16・AC-18〜AC-21・AC-23 は要件書で削除された欠番である。

## 8. 成功基準

-   **機能:** 7 章のすべての AC の `test`・`static` の検証が通り、`manual` の確かめを PR-2 の説明か本書に記録している。
-   **品質:** `make test`・`make lint` と CI の `doc-guard` のジョブが通る。各ガードと各テストは、壊して失敗することを確かめ、コミットメッセージに書いている（ステップ 1-6・2-5・3-4・4-5）。
-   **セキュリティ:** メンションを含む文面が、ツールの拒否（PR-1）と投稿の前の確認（PR-2）の両方で止まる。完了の印は、設計書 3.2 の受け入れる形のものだけを使う。
-   **文書:** 日英の `fixprguard` のガイドが同じ内容を述べ、用語集に 3 つの用語がある。

## 9. 次のステップ

-   切り替えの後に最初に開く PR で、Part B step 5 が開始の印を見て依頼しないこと（または依頼すること）を確かめる（4.2）。
-   開いている PR のうち `main` を取り込んでいないものに、`main` を取り込む（5 章）。
-   最後の PR のマージの後、`/runpr` が #403 を閉じる（要件書 2.4）。
-   設計書 9 章の将来の拡張（`coderabbitai` を自動レビュアーの一覧から外す、エスカレーションの方針の見直しなど）は、別の issue で扱う。#394 をどう閉じるかも、本タスクでは決めない（要件書 2.3）。
