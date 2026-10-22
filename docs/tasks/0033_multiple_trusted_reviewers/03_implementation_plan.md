# 実装計画書：fixprguard で複数の自動レビュアー（CodeRabbit と Codex）を信頼できるようにする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-22 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）の F-001〜F-003 を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。設計書 8 章の順序に従い、作業を 2 つのフェーズに分け、フェーズごとに 1 つの PR にする。

-   **フェーズ 1（ツール）:** `fixprguard` が `--reviewer` を繰り返し受け付け、`Reviewers` のどれかと一致する `Bot` を信頼するようにする。あわせて、呼び出しのガード、ガイドの PR-1 の項目、`package_reference.md` を変える。コマンドの文書と `_context.md` は変えない（設計書 8 章の 1）。
-   **フェーズ 2（文書）:** `_context.md` の 2 行、`fixpr.md`・`runpr.md`、ガイドの PR-2 の項目、用語集、内容のガードを変える。PR-1 をマージし、利用者が `make install-fixprguard` を実行した後に始める（設計書 8 章の 2・3）。

用語は設計書の冒頭と要件書 6 節に従う。特に「内容のガード」（`cmd/yt2column/fixpr_docs_test.go`）と「呼び出しのガード」（`cmd/fixprguard/docs_test.go` の `TestDocumentedInvocationsParse`）は、設計書の冒頭の定義に従う。本書の「フェーズ」は実装の区切りを指し、`/fixpr` の「修正のフェーズ」とは別である。

### 1.2. 実装原則

-   各ステップは設計書の節を参照し、設計の内容を本書で言い換えない。
-   Go のコメント・識別子・文字列リテラルは英語で書く。コマンドの文書（`.claude/commands/`）に書く文も英語で書く（`CLAUDE.md`「Language」「Go Idioms」）。例外は、日本語の文書の文字列と一致させる文書のガードのリテラルだけである。本タスクでは、`TestGlossaryTerms` と、ガイドの日本語版を読むテスト（ステップ 1-5・2-3）がこれに当たる。先例は `cmd/yt2column/docs_test.go:738`・`:747` である。
-   テストは、対象の実装を壊して失敗することを確かめてからコミットし、壊し方と結果をコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。各ステップの「壊して確かめる」は、その対象と、失敗するはずのテストを挙げる。壊し方は、実行のたびに同じ結果になる形にする。たとえば、集合の「最初の要素」には頼らず、テスト用の名前を直接書いて比べる。
-   Go のファイルを変えたら `make fmt` を実行する。各フェーズの最後に `make test && make lint` を実行する。
-   ソースのコメントに `AC-NN`・`F-NNN` を書かない（要件書との対応は本書の 5 章に書く。`docs/dev/developer_guide/requirements_process.md` 4 節）。

### 1.3. 既存コード調査結果

調査はコミット `819011a` で行った。設計書が確かめたコミット `e5db595` から、`internal/`・`cmd/`・`.claude/`・`docs/dev/`・`docs/translation_glossary.md`・`CLAUDE.md` に変更はない（`git diff --stat e5db595 819011a -- internal cmd .claude docs/dev docs/translation_glossary.md CLAUDE.md` の出力が空）。そのため、設計書の `file:line` は `819011a` でもそのまま使える。

#### 1.3.1. 変える箇所の一覧

洗い出しは、`rg -n --hidden` で `Automated reviewer`・`automated reviewer`・`<reviewer>`・`reviewer bot`・`coderabbitai[bot]`・`<reviewer>[bot]` を、また `rg -n --type go` で `NewGitHub(`・`assessThread(`・`processThread(`・`requireReviewer`・`.reviewer` を探して行った（`docs/tasks/` を除く。`819011a`）。

| 箇所 | 今 | 変え方 | ステップ |
|---|---|---|---|
| `internal/fixprguard/types.go` | `Login` と `ParseLogin` だけがある | `Reviewers` と `ParseReviewers` を加える（設計書 3.2） | 1-1 |
| `internal/fixprguard/construction_test.go:17-21`・`:28-71` | `restrictedConstructors` は `Login`・`PRNumber`・`Baseline`。検査はリテラル・`new`・`var` 宣言だけを報告する | `Reviewers`→`ParseReviewers` を加え、`ParseReviewers` と `Reviewers` のメソッドの外での `logins` のフィールドの参照も報告する（1.3.3）。コメント（`:28-32`）を改める | 1-1 |
| `internal/fixprguard/github.go:30-63` | `GitHub.reviewer Login`、`NewGitHub(run, reviewer Login)`、`requireReviewer` が `Login` を確かめる。`NewGitHubWithoutReviewer` のコメント（`:47-50`）が「the zero Login」と書く。エラーの文字列 `reviewer login`（`:42`・`:61`） | `Reviewers` に替える（設計書 2.2）。`NewGitHubWithoutReviewer` の処理は変えず、コメントとエラーの文字列だけを `Reviewers` に合わせる | 1-2 |
| `internal/fixprguard/github.go:328`・`:466`・`:519`、`write.go:126` | `assessThread(..., g.reviewer, ...)` | `g.reviewers` を渡す | 1-2 |
| `internal/fixprguard/github.go:380`・`:386-394` | `processThread(t, isNamed, g.reviewer)`。`ParseLogin` を通さない `first.loginRaw` を文字列で比べる | `Reviewers` を渡し、`ParseLogin` を通した名前で比べる（設計書 2.4・3.3）。コメントを改める | 1-2 |
| `internal/fixprguard/trust.go:62-66`・`:86` | `assessThread(..., reviewer Login, ...)`、`a.login != reviewer` | `Reviewers` を受け取り、一覧のどれかと一致するかで判定する（設計書 3.3）。コメントを改める | 1-2 |
| `internal/fixprguard/test_helpers.go:19`・`:205-213` | `testReviewer` は 1 つ。`newTestGitHub` は `mustLogin(t, testReviewer)` で作る | `newTestGitHub` は 1 つの名前のまま `Reviewers` で作る。2 つめのテスト用の名前と、名前を指定して作るヘルパーを加える | 1-2 |
| `internal/fixprguard/github_test.go:38-46`・`:415` | `NewGitHub(fake, Login{})`、`NewGitHub(runner, mustLogin(...))` | `Reviewers` に替える | 1-2 |
| `cmd/fixprguard/main.go:175-195` | `flagValue` のコメントと `multi` の判定（`--accept`・`--thread` だけ） | `--reviewer` を値を集めるフラグに加える。コメントを改める | 1-3 |
| `cmd/fixprguard/main.go:213`・`:218-221` | 値を集めるフラグの指定。必須のフラグの確認は `inv.values` だけを見る | 値を集めるフラグにも必須の確認が働くようにする（設計書 2.4） | 1-3 |
| `cmd/fixprguard/main.go:375-383` | `githubFor` が `ParseLogin(inv.values[flagReviewer])` を呼び、エラーにフラグの名前を付けない | `ParseReviewers(inv.multi[flagReviewer])` を呼び、エラーに `--reviewer` を付ける（設計書 4 章） | 1-3 |
| `cmd/fixprguard/main_test.go:95-99` | 「`--reviewer` twice」の行（値を 1 つだけ持つフラグを 2 回渡すと拒否されることを確かめる、唯一のテスト） | 別のフラグを 2 回渡す行に置き換える（設計書 1.4 の表） | 1-3 |
| `cmd/fixprguard/test_helpers.go:22`・`:246-248`・`:322-339`、`main_test.go:504-512` | `testReviewer` は 1 つ。`reviewerThread`・`fingerprintOf`・`replyArgs` は `--reviewer testReviewer` を書き込んでいる | 2 つめのテスト用の名前を加え、`fingerprintOf`・`accept`・`replyArgs` が渡す `--reviewer` の並びを指定できるようにする | 1-3 |
| `cmd/fixprguard/docs_test.go:57-59`・`:84-116` | `reviewerPlaceholder` のコメント。`inv.values[flagReviewer]` を確かめる | 確かめる処理を関数に切り出し、`inv.multi` の値がちょうど 1 つの `<reviewer>` であることを確かめる（設計書 3.4） | 1-4 |
| `cmd/fixprguard/docs_test.go` | ガイドは、止まる理由の種類と範囲のファイルの定義だけを確かめる（`:38-55`） | ガイドの `--reviewer` の記述を確かめるテストを加える | 1-5・2-3 |
| `docs/dev/developer_guide/fixprguard_guide.ja.md`・`fixprguard_guide.md` の `:11`・`:58`・`:60`・`:70`・`:87`・`:100`・`:106`・`:141`・`:175`・`:246`・`:276` | 1 つの `--reviewer` の記述 | 設計書 3.5 のガイドの PR-1 の項目（`:70` は終了コード `2` の行） | 1-5 |
| `docs/dev/developer_guide/package_reference.md:21` | 「the reviewer bot's `Login`」「by the reviewer bot」 | 設計書 3.5 の `package_reference.md` の項目 | 1-5 |
| `cmd/yt2column/docs_test.go` | `package_reference.md` の行を読むテストの先例がある（`:754-781`） | `internal/fixprguard` の行を確かめるテストを加える | 1-5 |
| `.claude/commands/_context.md:185` | `Automated reviewer` の行 | `Automated reviewer list` と `Awaited PR reviewer` の 2 行に置き換える（設計書 1.2・3.4） | 2-1 |
| `cmd/yt2column/fixpr_docs_test.go:41-44`・`:75-86`・`:336-363` | `reviewerSentence`、最初に一致した行を返す `contextRow`、`TestContextAndCommandsAgree` | 設計書 3.4 の検査に改める（1.4 の決定） | 2-1 |
| `.claude/commands/fixpr.md:44-47`・`:94-95`・`:142-143` | `<reviewer>` の定義、ラウンド上限、「Untrusted threads」 | 設計書 3.5 | 2-1 |
| `.claude/commands/runpr.md:18-19`・`:216-224` | `<reviewer>` の定義、step 3 の `<reviewer>[bot]` と `"Automated reviewer"` への参照 | 設計書 3.5（`<awaited-reviewer>`） | 2-1 |
| `docs/translation_glossary.md:68` | 「信頼する著者」の定義が `_context.md` の 1 つの名前を指す | 設計書 2.2（AC-13） | 2-2 |
| `cmd/yt2column/fixpr_docs_test.go:419-450` | `TestGlossaryTerms` | 3 行を加え、「信頼する著者」の定義を確かめる | 2-2 |
| ガイドの日本語版と英語版 | `_context.md` の一覧への言及がない。一覧から外す手順がない | 設計書 3.5 のガイドの PR-2 の項目、9 章 | 2-3 |

変えない箇所は次のとおりである。

-   `@coderabbitai review` を含む `.claude/commands/_lib/task-pr-handoff.md:190`・`_lib/pr-review-escalation.md:9`・`CLAUDE.md:56`・`.claude/commands/runpr.md:238`（設計書 2.2 の末尾。0031 の範囲）。
-   ガイドの各節の呼び出しの例（`fixprguard_guide.ja.md:92`・`:113`・`:146`・`:182`、英語版の同じ箇所）。`--reviewer` を 1 つ書いた形のままでよい（設計書 3.5）。
-   `internal/fixprguard/types_test.go:30` と `trust_test.go:116` の `coderabbitai[bot]` は、文法に合わない名前の入力の例なので残す。

#### 1.3.2. 名前を変えるものの一覧

| 今の名前 | 新しい名前 | 場所 | ステップ |
|---|---|---|---|
| `_context.md` の行 `Automated reviewer` | `Automated reviewer list` と `Awaited PR reviewer` の 2 行 | `.claude/commands/_context.md` | 2-1 |
| プレースホルダー `<reviewer>[bot]` | `<awaited-reviewer>[bot]` | `.claude/commands/runpr.md` step 3 | 2-1 |
| フィールド `GitHub.reviewer`（`Login`） | `GitHub.reviewers`（`Reviewers`） | `internal/fixprguard/github.go` | 1-2 |

本書の各ステップ、5 章の表、6 章のチェックリストは、この表の名前で書く。古い名前が残っていないことは次で確かめる。

-   フィールド: ビルドで確かめる。
-   `_context.md` の行: 内容のガードの行の数の検査で確かめる（ステップ 2-1）。
-   `fixpr.md`・`runpr.md` の参照: `TestRemovedCommandLiterals` に加える行で確かめる（ステップ 2-1）。古い参照は `fixpr.md:45`・`runpr.md:19`・`runpr.md:217` の 3 か所にある。前後の引用符を含めた `"Automated reviewer"` は、この 3 か所すべてに一致し、新しい `"Automated reviewer list"` には一致しない。

#### 1.3.3. 型の構築の経路

`Reviewers` は、使えない値を型で防ぐ型である（設計書 3.2）。ゼロ値は「名前なし」で、`NewGitHub` が拒否し、判定する呼び出しは `requireReviewer` で拒否する。使える値の構築を `ParseReviewers` だけに限る方法として、本書は既存の `TestRestrictedTypesBuiltByConstructors`（`internal/fixprguard/construction_test.go:84-113`）に登録する方を採る。`Reviewers` を葉のパッケージに移す案は採らない。理由は次のとおりである。

-   `Login`・`PRNumber`・`Baseline` が同じ仕組みで守られている。この検査は、パッケージの中のテストでないすべてのファイルのリテラル・`new`・`var` 宣言を報告する（`construction_test.go:33-71`・`:84-113`、`819011a`）。
-   この検査が報告しない形は、構造体のフィールド、スライスの要素、関数の引数、名前付きの戻り値である（`construction_test.go:28-32`）。これらは宣言だけならゼロ値になり、`requireReviewer` が拒否する。

ただし、宣言の後の書き込みは、今の検査には見えない。`Reviewers` は map を持つので、同じパッケージのコードは `ParseReviewers` を通さずに使える値を作れる。名前付きの戻り値やフィールドの `logins` に map を代入する形と、既存の値の `logins` に要素を加えるか消す形である。map は値のコピー同士で共有されるので、後者はすべてのコピーに影響する。`m := r.logins` のように別名を取ってから書き換えると、書き込みの形だけを調べる検査では見つからない。これは信頼を広げる経路なので、ステップ 1-1 で検査を広げ、`logins` のフィールドを参照できるのは `ParseReviewers` と `Reviewers` のメソッド（名前を含むかを調べるメソッドと使えるかを調べるメソッド。どちらも読むだけ）だけとし、それ以外の参照をすべて報告させる。参照を禁じれば、代入・要素の書き換え・`delete`・別名の取得のどれも書けない。`logins` という名前のフィールドは、`internal/fixprguard` の中でほかに使われていない（`819011a`）。設計書 3.2 の表現（非公開の map）は変えない。

`NewGitHubWithoutReviewer`（`github.go:51-56`）は、`GitHub` のリテラルで `Reviewers` のフィールドを省くので、検査に触れない（設計書 3.2）。

#### 1.3.4. 外部の前提の確かめ

-   **古いツールの診断:** インストール済みのツールの `vcs.revision` は `aedefac`（#416 のマージコミット）である（`go version -m "$(command -v fixprguard)"`）。これに、作業ツリーの外で `fixprguard list --protocol 1 --reviewer coderabbitai --reviewer chatgpt-codex-connector 1` を渡した。結果は終了コード `2` で、標準エラーは次の 1 行だった（2026-10-22）。

    ```
    fixprguard: usage: invalid value "chatgpt-codex-connector" for flag -reviewer: flag given twice: --reviewer (see docs/dev/developer_guide/fixprguard_guide.md)
    ```

    設計書 1.2 の 3 つめの項目が使う文字列 `flag given twice: --reviewer` が含まれる。ガイドの案内（ステップ 1-5）は、この文字列で古いツールを見分ける。新しいツールの診断がこの文字列を含まないことは、ステップ 1-3 のテストで確かめる。
-   **ツールの版の確かめ:** `go version -m` は `vcs.revision` と `vcs.modified` を表示する。上のとおり確かめた。ガイドの案内（ステップ 1-5）で使う。
-   **インストール済みのツールの振る舞いの確かめ:** `list` は、`--reviewer` を読む前に PR 番号の文法を確かめる（`cmd/fixprguard/main.go:423-426` の `validatePR`、`5b54919`）。文法に合わない PR 番号を渡すと、`gh` も git も呼ばずに止まる。`5b54919` のソースから作ったツールに、作業ツリーの外で次の 2 つを渡した（2026-10-22）。どちらも終了コード `2` である。
    -   `list --protocol 1 --reviewer coderabbitai x` の標準エラーは `fixprguard: invalid input: PR number grammar` だった。
    -   `list --protocol 1 --reviewer coderabbitai --reviewer chatgpt-codex-connector x` の標準エラーは、上の古いツールの診断と同じ 1 行だった。

    PR-1 の後のツールは、2 つめの呼び出しでもフラグを受け付け、PR 番号の診断を返さなければならない。ステップ 2-0 はこの違いで版を確かめ、ステップ 1-3 の表の行がこの振る舞いを固定する。コミットの祖先では確かめない。PR-1 が revert されても、revert のコミットは PR-1 のコミットの子孫なので、祖先の判定は通ってしまうからである。
-   **fake が呼ばれたときの失敗:** `cmdrunnertestutil.Fake` は、手順にない呼び出しで `Fatalf` する（`internal/cmdrunner/testutil/mocks.go:64`、`819011a`）。AC-07 の「GitHub にも git にも触れない」は、手順を持たない fake の `gh` と git を渡して確かめる。既存の表（`cmd/fixprguard/main_test.go:100-128`）と同じ方法である。
-   **`go tool cover -func`:** `go test -tags test -coverprofile=<file> ./cmd/fixprguard` の後に `go tool cover -func=<file>` で関数ごとのカバレッジを表示する。ステップ 1-3 で、行の置き換えの前後を比べるのに使う。
-   **`make deadcode`:** `deadcode -test -tags test ./cmd/yt2column` だけを実行する（`Makefile:228-229`）。`internal/fixprguard` の関数はもともと到達できないものとして報告されるので、フェーズ 1 の確かめには使わない。

#### 1.3.5. 実装時に決めること

-   `ParseReviewers` のエラーの規則の名前と、`githubFor` の診断の文言（設計書 4 章が実装に委ねた範囲）。
-   テスト用の 2 つめの名前（実在の `chatgpt-codex-connector` を使うかを含む）と、名前を指定して `GitHub` を作るテストのヘルパーの形。
-   値を集めるフラグの指定の書き方（`parse` の式に `--reviewer` を足すか、`spec` に持たせるか）。
-   内容のガードの関数が返す問題の型。検査ごとに区別できる種類を持たせる（ステップ 2-1）。
-   コマンドの文書の英語の文面と、内容のガードが固定する文。設計書 3.5 の趣旨を変えない範囲で決める。`/runpr` の中で実行する `/fixpr` も、その実行の最初に `_context.md` から名前の並びを自分で作る。`/runpr` が作った並びと食い違うのは、`_context.md` が実行の途中で書き換えられた場合に限られる。これは、設計書 5.3 が受け入れた残るリスク「一覧の書き換え」に含まれる。
-   新しいテストの関数の名前。5 章の表に書いた名前と変えたときは、同じ PR で本書の表を改める。

### 1.4. 計画の際に決めたこと（2026-10-22）

-   **内容のガードの `_context.md` の検査は、本文を受け取って問題を返す関数に切り出し、壊れた写しの表をテストとしてコミットする:** 設計書 7.3 は「`_context.md` の写しを壊して確かめる」と書くが、その確かめを残すテストにするかは決めていない。表にすれば、ガードの検査が後で緩められたときに CI で検出できる。実装のときに一時的に壊すだけでは、その時点の確かめにとどまる。同じ理由で、呼び出しのガードの検査も関数に切り出し、合成した呼び出しの表で確かめる（ステップ 1-4）。
-   **`cmd/fixprguard` の結合テストは、新しいテストとして加え、設計書 7.4 を本 PR で直す:** 設計書 7.4 は「既存の流れのテスト（`begin` から `reply` まで）」に場合を加えるとする。しかし、`begin` の後に同じテストで `reply` を実行する既存のテストはない（`cmd/fixprguard/main_test.go`、`819011a`）。続けて実行するテストには、`list` の出力の指紋で `inspect` を実行する `TestInspectExitCode`（`:340-371`）があるが、`begin` と `reply` は含まない。新しいテストを加えることは設計書の意図どおりなので、設計書の誤りは editorial correction として直し、承認を保つ（`docs/dev/developer_guide/requirements_process.md`「Editing an approved document」）。
-   **PR は設計書 8 章のとおり 2 つにする:** `NewGitHub` の引数の型が変わるので、`internal/fixprguard` と `cmd/fixprguard` は同じ PR でなければビルドが通らない。ガイドの PR-1 の項目はツールの振る舞いの記述なので、コードと同じ PR でレビューするほうが食い違いを見つけやすい。そのため、高リスクのステップ 1-2 は PR-1 の中で独立させず、ガイドと同じ PR に置く。ガイドを別の PR にして 3 つに分ける案は採らない。

### 1.5. 申し送りの項目への対応

-   `design_handoff.md` の H-01〜H-04 は、すべて設計書の付録 A に記録されている。H-01 の順序は、フェーズの分け方（1.1）とステップ 2-0 に落とした。H-04 の壊し方は、各ステップの「壊して確かめる」に落とした。
-   `implementation_handoff.md` はない。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: ツール（設計書 8 章の 1）

#### ステップ 1-1: `Reviewers` と `ParseReviewers`

**変更するファイル**: `internal/fixprguard/types.go`、`types_test.go`、`construction_test.go`

-   [ ] `Reviewers` と `ParseReviewers` を、設計書 3.2 の型とコメントのとおりに加える。名前を含むかを調べるメソッドと使えるかを調べるメソッドは、パッケージの外に出さない。
-   [ ] `types_test.go` に `TestParseReviewers` を加える。設計書 7.1 の `ParseReviewers` の項目の場合に加え、次を確かめる。
    -   正しい名前の後に正しくない名前が続く並び。
    -   隣り合わない重複。
    -   `nil`・空のスライス・空の文字列 1 つ。
    -   多くの名前の並び（要件書 3.2 の「数の上限を設けない」）。

    エラーは `errors.Is(err, ErrInvalidInput)` で確かめる。エラーの文字列が入力の値を含まないことは、`TestErrorsOmitCommandDetails`（`github_test.go:517`）と同じく文字列で確かめる。
-   [ ] `restrictedConstructors` に `Reviewers`→`ParseReviewers` を加える。`findRestrictedConstructions` を広げ、`ParseReviewers` と `Reviewers` のメソッドの外での `logins` のフィールドの参照を報告させる（1.3.3）。コメントを改める。
-   [ ] `TestRestrictedTypesBuiltByConstructors` の自己テストの表に、次の行を加える。
    -   `ParseReviewers` の中のリテラルは報告されない。
    -   外のゼロ値のリテラル、値を持つリテラル、`&` 付きのリテラル、`new`、`var` 宣言は報告される。
    -   別の構築関数（たとえば `ParseLogin`）の中の `Reviewers{}` は報告される。
    -   `ParseReviewers` と `Reviewers` のメソッドの中の `logins` の参照は報告されない。
    -   それ以外の関数・メソッドでの `logins` の参照（名前付きの戻り値への代入、要素への代入、`delete`、`m := r.logins` の別名の取得、読むだけの参照）は報告される。

**壊して確かめる**: 次を確かめる。

-   重複の確認を外す、空の一覧の確認を外す、2 つめ以降の名前の文法の確認を外す、比べる前に大文字と小文字をそろえる。それぞれで `TestParseReviewers` の該当の行が失敗すること。
-   `restrictedConstructors` から `Reviewers` を外すと、自己テストのリテラルの行が失敗すること。
-   広げた検査を外すと、`logins` の参照の自己テストの行が失敗すること。

**完了条件**: `go test -tags test ./internal/fixprguard` が通る。

#### ステップ 1-2: 判定に `Reviewers` を使う

**変更するファイル**: `internal/fixprguard/github.go`、`trust.go`、`write.go`、`test_helpers.go`、`trust_test.go`、`github_test.go`、`write_test.go`

-   [ ] `GitHub`・`NewGitHub`・`requireReviewer`・`assessThread`・`processThread`・`Reply` の判定を、設計書 2.2 の表と 3.3 のとおりに `Reviewers` に替える。コメント（`github.go:35-36`・`:47-50`・`:386-389`、`trust.go:62-65`）とエラーの文字列（`github.go:42`・`:61`）を改める。
-   [ ] `test_helpers.go` の `newTestGitHub` は、今と同じ 1 つの名前の `Reviewers` で作る。既存のテストが変更なしで通ることで、1 つの名前の判定が今と同じであることを確かめる（AC-04）。2 つめのテスト用の名前と、名前の並びを指定して作るヘルパーを加える。
-   [ ] `TestNewGitHub` を、ゼロ値の `Reviewers` を拒否する形に改める。
-   [ ] `TestAssessAuthors` に、2 つの名前の `Reviewers` で、設計書 7.1 の判定の表の 6 つの場合を確かめる行を加える。既存の `reviewer_bot_without_lookup`・`user_named_like_reviewer` の行と重なる場合は、新しい行を足さず、既存の行を 2 つの名前で確かめる形に改める。あわせて、1 つの名前の `Reviewers` で、渡さなかった 2 つめの名前の `Bot` が理由 (b) になり `account` にその名前が入ることを確かめる（設計書 5.2 の「渡し忘れは安全側」）。
-   [ ] `TestAssessUnverifiableAuthor` の `bot_login_outside_grammar` を、2 つの名前の `Reviewers` で確かめる形にする。`chatgpt-codex-connector[bot]` の行を加える。
-   [ ] `TestBeginAuthorType` を、2 つの名前の `Reviewers` で、設計書 7.1 の `authorType` の 3 つの場合と、一覧の名前と一致する `User`（書き込み権限で信頼される）のスレッドが `User` になる場合を確かめる形にする（AC-08）。
-   [ ] `TestThreadPathsRequiresTrustedOrNamed` に、2 つめの名前の `Bot` のスレッドが `Named` でない場合を加える。
-   [ ] `write_test.go` に、2 つの名前の `Reviewers` で、2 つめの名前の `Bot` のスレッドに名指しなしで返信できることを確かめるテストを加える。一覧にない `Bot` のスレッドに書き込まないことは、既存の `TestReplyRefusesUntrustedThread`（`write_test.go:152-161`、`819011a`。`other-bot` の Bot で `ErrThreadNotTrusted`）が確かめる。

**壊して確かめる**: 次の 4 つで、それぞれ括弧の中のテストが失敗すること。どの壊し方も、比べる名前をテスト用の 1 つめの名前に固定して書く（1.2）。

-   `assessThread` が 1 つめの名前とだけ比べる（`TestAssessAuthors` の 2 つめの名前の行）。
-   `processThread` が 1 つめの名前とだけ比べる（`TestBeginAuthorType` の 2 つめの名前の行）。
-   `Reply` が `assessThread` に 1 つめの名前だけの `Reviewers` を渡す（返信のテスト）。
-   `ThreadPaths` が同じく 1 つめの名前だけを渡す（`TestThreadPathsRequiresTrustedOrNamed` の新しい行）。

**完了条件**: `go test -tags test ./internal/fixprguard` が通る。`cmd/fixprguard` はステップ 1-3 までビルドが通らないので、`make test` はステップ 1-3 の後に実行する。

#### ステップ 1-3: `cmd/fixprguard` のフラグ

**変更するファイル**: `cmd/fixprguard/main.go`、`main_test.go`、`test_helpers.go`

-   [ ] 最初に、`TestAcceptThreadFlagParsing` の「`--reviewer` twice」の行を、値を 1 つだけ持つフラグを 2 回渡す行に置き換える。たとえば `inspect` の `--baseline` を、ほかの引数を正しい値にして 2 回渡す。この置き換えは、`main.go` を変える前に、それだけのコミットにする。置き換えの前後で、`go tool cover -func` のパッケージ全体の関数ごとのカバレッジが変わらないことを確かめ、コミットメッセージに書く（`CLAUDE.md`「Deleting a test is a claim that must be checked」。1.3.4）。
-   [ ] `--reviewer` を値を集めるフラグにし、値を集めるフラグにも必須の確認が働くようにする。`flagValue` のコメントを改める（設計書 2.2 の `main.go` の行）。
-   [ ] `githubFor` が `ParseReviewers` にすべての `--reviewer` の値を渡し、エラーに `--reviewer` を付けるようにする（設計書 4 章）。`githubFor` のコメントを改める。
-   [ ] `test_helpers.go` に 2 つめのテスト用の名前を加え、`fingerprintOf`・`accept`（`test_helpers.go:342`）・`replyArgs` が渡す `--reviewer` の並びを指定できるようにする。
-   [ ] 違う名前の `--reviewer` を 2 回渡すと `parse` が受け付け、2 つの値を集めることを確かめるテストを加える。
-   [ ] 「コマンドを起動しない」表（`main_test.go:100-128`）を、行ごとの `t.Run` にする。`list`・`begin`・`inspect`・`reply` のそれぞれについて、次の 4 行を加える。各行のほかの引数は正しい値にする。
    -   `--reviewer` がない。
    -   同じ名前を 2 回渡す。
    -   正しくない値を渡す。
    -   正しい値と正しくない値を渡す。

    あわせて、`list` に違う 2 つの正しい名前と文法に合わない PR 番号 `x` を渡す行を加える。この行は、ステップ 2-0 の確かめの振る舞いを固定する。終了コードが `2` で、標準エラーが PR 番号の文法の診断であり、`flag given twice` を含まないことを確かめる。

    上の 16 行では、終了コード `2` に加えて、標準エラーが 1 行で、`--reviewer` を含み、渡した値を含まないことも確かめる。`--reviewer` を 2 回以上渡す行では、`parse` が成功すること（拒否するのは `ParseReviewers` だけであること。設計書 7.2）と、標準エラーが `flag given twice` を含まないことも確かめる。既存の `begin` の `bad name` の行は、加える行に含める。表のほかの既存の行（`--accept`・`--baseline`・`finish`・`head` など）は残す。
-   [ ] `list` と `begin` を、`--reviewer` の 2 つの順序で同じリポジトリに対して実行するテストを加える。スレッドは、2 つの名前の `Bot` のスレッドと一覧にない `Bot` のスレッドを用意する。2 つの順序で標準出力が同じであることに加え、2 つの名前のスレッドが処理され、一覧にない `Bot` のスレッドが処理されないことを確かめる（設計書 7.1 の「順序によらないこと」）。
-   [ ] 2 つの名前を渡し、2 つめの名前の `Bot` のスレッドを `begin` で処理し、`begin` の出力の指紋で `reply` するテストを加える（設計書 7.4 の結合テスト。1.4）。`begin` と `reply` の両方に、同じ 2 つの `--reviewer` を渡す。
-   [ ] `TestReviewerFlag` と、`--reviewer` を 1 回だけ渡す既存のテストが、変更なしで通ることを確かめる（AC-04・AC-09）。

**壊して確かめる**: 次の 5 つで、それぞれ括弧の中のテストが失敗すること。

-   必須の確認が値を集めるフラグを確かめない（`TestReviewerFlag` の `--reviewer` を消した行と、表の `--reviewer` がない行）。
-   `githubFor` が最初の `--reviewer` の値だけを `ParseReviewers` に渡す（結合テストと順序のテスト）。
-   `githubFor` がエラーに `--reviewer` を付けない、または渡した値をエラーに含める（表の標準エラーの確かめ）。
-   `inspect` では `githubFor` の呼び出しを `CheckState` の後ろへ、`begin` では `CheckClean` の後ろへ移す（それぞれのサブコマンドの重複の行。fake の git が呼ばれる）。`list` と `reply` は、`githubFor` が作る `GitHub` を使ってから GitHub と git に触れるので、この壊し方の対象にならない。
-   `flagValue.Set` が `--reviewer` の 2 回目を拒否する（表の `parse` の成功の確かめ）。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

#### ステップ 1-4: 呼び出しのガード

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [ ] `TestDocumentedInvocationsParse` の `--reviewer` の確かめを、呼び出しの引数を受け取って問題を返す関数に切り出す。関数は、`--reviewer` を取る各呼び出しで、`inv.multi` の `--reviewer` の値がちょうど 1 つで、それが `<reviewer>` であることを確かめる（設計書 3.4 の呼び出しのガード）。テストのコメントと `reviewerPlaceholder` のコメントを改める。
-   [ ] この関数を、合成した呼び出しの表で確かめるテストを加える（1.4）。表は、`<reviewer>` が 2 つ、名前そのもの、`<reviewer>` 1 つ（問題なし）の場合を含む。

**壊して確かめる**: 「ちょうど 1 つ」の確かめと「`<reviewer>` である」の確かめを 1 つずつ外し、表の該当の行が失敗すること。

**完了条件**: `go test -tags test ./cmd/fixprguard` が通る。

#### ステップ 1-5: ガイドと `package_reference.md`

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`、`fixprguard_guide.md`、`package_reference.md`、`cmd/fixprguard/docs_test.go`、`cmd/yt2column/docs_test.go`

-   [ ] 日本語版のガイドを、設計書 3.5 のガイドの PR-1 の項目のとおりに改める。`_context.md` の行の名前には触れない。終了コード `2` の説明には、1.3.4 で確かめた古いツールの診断の文字列 `flag given twice: --reviewer` と、`go version -m "$(command -v fixprguard)"` の `vcs.revision` での確かめ方を書く。
-   [ ] 日本語版をコミットしてから、英語版を `.claude/commands/mktrans.md` の原則で翻訳する（`CLAUDE.md`「Translation Guidelines」）。
-   [ ] `package_reference.md` の `internal/fixprguard` の行を、設計書 3.5 の `package_reference.md` の項目のとおりに改める。
-   [ ] `cmd/fixprguard/docs_test.go` に、両方のガイドについて次を確かめるテストを加える。
    -   サブコマンドの見出しのうち、`--reviewer LOGIN ...` を含むものが、`specs` で `--reviewer` を取るサブコマンドとちょうど一致すること。
    -   フラグの表の `--reviewer` の行が、繰り返せること、1 つもないときと同じ名前を 2 回以上渡したときは使い方のエラーになることを述べていること。
    -   `other-bot` の行が、渡したどの名前とも一致しない Bot を述べていること。
    -   終了コード `2` の説明が、古いツールの診断の文字列と `go version -m` を含むこと。
    -   §9 が `Protocol` の規則の但し書きを含むこと。

    行の中の確かめは、表の行を取り出して行う。日本語版を確かめるリテラルは日本語で書く（1.2）。固定する語は、書いた文から選ぶ。
-   [ ] `cmd/yt2column/docs_test.go` に、`package_reference.md` の `internal/fixprguard` の行が、語として `ParseReviewers` と `Reviewers` を含み、`reviewer bot` を含まないことを確かめるテストを加える。既存の `findTable`・`tableRow`・`containsToken`（`cmd/yt2column/docs_test.go:377-400`・`:623`）を使う。
-   [ ] ガイドの記述をツールの振る舞いと突き合わせる。サブコマンドの見出しは `specs` と、終了コード `2` と重複の説明はステップ 1-3 のテストと、§9 の但し書きは設計書 1.4 の 3 つの条件と比べる。

**壊して確かめる**: ガイドの日本語版と英語版のそれぞれで、固定した文を 1 つずつ一時的に消すと、テストが失敗すること。`reply` の見出しから `...` を消すと、見出しの確かめが失敗すること。

**完了条件**: `go test -tags test ./cmd/fixprguard ./cmd/yt2column` が通る。

#### ステップ 1-6: フェーズ 1 の確かめ

**変更するファイル**: なし

-   [ ] `.claude/` を変えていないことを、`git diff --stat "$(git merge-base origin/main HEAD)" -- .claude` の出力が空であることで確かめる（設計書 8 章の 1。PR-1 の `/runpr`・`/fixpr` が今のツールで動くため）。`origin/main` は、`/runplan` が作業の初めに取得したものを使う（`_lib/task-pr-handoff.md` の Part A の 2）。
-   [ ] `make test && make lint` を実行する。
-   [ ] 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）を実行する。

**完了条件**: 上の 3 つ。

### PR-1 作成ポイント: fixprguard accepts repeated --reviewer

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6

**推奨タイトル**: `feat(0033): fixprguard で --reviewer を繰り返し受け付け、どれかと一致する Bot を信頼する`

**レビュー観点**: 判定の 4 か所（`list`・`begin`・`inspect`・`reply`）と `processThread` が同じ `Reviewers` を同じ比べ方で使うこと / `ParseReviewers` が唯一の構築関数で（`logins` の参照の検査を含む）、重複・空・文法違反を拒否し、`githubFor` が GitHub と git に触れる前に呼ばれること / `--reviewer` を 1 回だけ渡す呼び出しの結果が変わらないこと（AC-04）と、`--protocol` を上げない例外についてのガイドの但し書き / マージの後、PR-2 に進む前に `make install-fixprguard` が要ること

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-2 は、信頼の判定を広げるセキュリティのゲートの変更である。この判定は、どのスレッドを名指しなしで修正のフェーズに渡すかを決める（`mkplan.md` step 8 の panel mode の契機）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

**マージの後**: 利用者は、PR-1 のマージの後の `origin/main` を含む作業ツリーで `make install-fixprguard` を実行する。`/runpr` がマージの後に作るブランチでもよい。設計書 8 章の 2 がこの申し送りを本書に求めている。今の `/runpr` と `/mergepr` はこの案内を示さない。そのため、PR-1 の `/runpr` で「マージして次を実行する」を選ぶと、再インストールの前に `/runplan 0033` が始まる。その場合はステップ 2-0 で止まる。

### フェーズ 2: 文書（設計書 8 章の 3）

#### ステップ 2-0: インストール済みのツールの版の確かめ（設計書 8 章の 2）

**変更するファイル**: なし

-   [ ] インストール済みの `fixprguard` に `list --protocol 1 --reviewer coderabbitai --reviewer chatgpt-codex-connector x` を渡し、終了コードが `2` で、標準エラーが PR 番号の文法の診断の 1 行であり、`flag given twice` を含まないことを確かめる（1.3.4）。PR 番号 `x` は文法に合わないので、ツールは GitHub にも git にも触れずに止まる。`protocol` の番号は変わらないので、番号ではなく振る舞いで確かめる。コミットの祖先では確かめない（1.3.4）。
-   [ ] 満たされていなければ、このフェーズを始めず、利用者に `make install-fixprguard` の実行を求めて止まる。

**完了条件**: 上の確かめが通る。

#### ステップ 2-1: `_context.md`・`fixpr.md`・`runpr.md` と内容のガード

`_context.md` の行を変えると、今の `TestContextAndCommandsAgree` と `reviewerSentence` が失敗する。そのため、このステップの文書とガードの変更は、1 つのコミットにまとめる。

**変更するファイル**: `.claude/commands/_context.md`、`fixpr.md`、`runpr.md`、`cmd/yt2column/fixpr_docs_test.go`

-   [ ] `_context.md` の `Automated reviewer` の行を、設計書 1.2 と 3.4 の形の `Automated reviewer list` と `Awaited PR reviewer` の 2 行に置き換える。説明の文は AC-01・AC-03 が求めることを述べ、1 つの Bot だけを信頼するとは述べない。
-   [ ] `fixpr.md`・`runpr.md` の `<reviewer>` の定義、`fixpr.md` の「Untrusted threads」とラウンド上限、`runpr.md` step 3 の `<awaited-reviewer>[bot]` と `<awaited-reviewer>` の定義を、設計書 3.5 のとおりに改める。名前そのものは書かない。照合の行、LAST_REVIEWED の決め方、`@coderabbitai review` の依頼のコメントは変えない（AC-11）。
-   [ ] 内容のガードの `_context.md` の検査（設計書 3.4 の行の数の検査と 1〜4）を、`_context.md` の本文を受け取って問題の一覧を返す関数に切り出す（1.4）。問題は、検査ごとに区別できる種類を持たせる（1.3.5）。行の数え上げには既存の `markdownTables`・`tableRowsWithFirstCell`（`cmd/yt2column/docs_test.go:325`・`:405`）を使う。
-   [ ] `TestContextAndCommandsAgree` は、本物の `_context.md` でこの関数が問題を返さないことを確かめる。あわせて、待つ PR レビュアーが `coderabbitai` であること、その説明が REST の `user.login` の形と `[bot]` を述べること、一覧の説明が 1 つの Bot だけを信頼するとは述べないことを確かめる（AC-01・AC-03）。続けて、設計書 3.4 の 5・6 の検査を行う。`reviewerSentence` を新しい定義の文に替え、`<awaited-reviewer>` の定義の文の定数を加える。
-   [ ] 同じファイルに、本物の `_context.md` から作った写しの表で、この関数が期待した種類の問題を返すことを確かめるテストを加える。写しの壊し方は、設計書 7.3 の列挙に次を加える。
    -   どちらかの行がない。
    -   名前の 1 つを別の正しい名前に替える。
    -   区切りの違いの各形（バッククォートがない、`,` の後に空白がない、空白が 2 つ、末尾に区切りがある）。
    -   ` — ` の前が空、または ` — ` がない。
    -   待つ PR レビュアーの名前が 2 つ、空、文法違反。

    問題を返さない行も置く。説明の文に `` `[bot]` `` などのバッククォートの語や 2 つめの ` — ` を含む写しが、ちょうど 2 つの名前に読まれることを確かめる（H-02）。
-   [ ] `TestRunprDocContract` に、step 3 の「Reviewed」の部分（2 つの `jq` の絞り込み、照合の `grep` の行、LAST_REVIEWED の段落）を、`autoFixprCondition`（`fixpr_docs_test.go:46-56`）と同じく定数で固定する確かめを加える。今のガードは、この部分を固定していない（`fixpr_docs_test.go` に `between` と `[bot]` の語はない。`819011a`）。
-   [ ] `TestFixprDocContract` に、「Untrusted threads」の信頼する著者の説明とラウンド上限の説明が、一覧のどれかの自動レビュアーを指すことを確かめる行を加える（AC-12）。
-   [ ] `TestRemovedCommandLiterals` に、`fixpr.md`・`runpr.md` が `"Automated reviewer"`（前後の引用符を含む）と `<reviewer>[bot]` を含まないことの行を加える（1.3.2）。

**壊して確かめる**: 次で、それぞれ括弧の中のテストが失敗すること。

-   関数から、行の数の確認、名前の並びの形の確認、`ParseReviewers` の呼び出し、ちょうど 2 つの名前の確認を 1 つずつ外す（写しの表の、その種類を期待する行。設計書 7.2）。
-   文書に `chatgpt-codex-connector` を 1 回書く、`coderabbitai[bot]` を書く、`<reviewer>` の定義の文から「1 度だけ作る」の部分を消す、step 3 の片方の `jq` を `<reviewer>[bot]` に戻す（`TestContextAndCommandsAgree`・`TestRunprDocContract`。一時的に書き換えて確かめ、戻す）。

**完了条件**: `go test -tags test ./cmd/yt2column ./cmd/fixprguard` が通る。

#### ステップ 2-2: 用語集

**変更するファイル**: `docs/translation_glossary.md`、`cmd/yt2column/fixpr_docs_test.go`

-   [ ] 「信頼する著者」の定義を、自動レビュアーの一覧を指す形に改める。「自動レビュアー」（automated reviewer）、「自動レビュアーの一覧」（automated reviewer list）、「待つ PR レビュアー」（awaited PR reviewer）の 3 行を加える。定義は要件書 6 節に合わせる。
-   [ ] `TestGlossaryTerms` に 3 行を加え、「信頼する著者」の定義のセルが「自動レビュアーの一覧」を含むことを確かめる。

**壊して確かめる**: 用語集の 3 行のどれかを消す、「信頼する著者」の定義を元に戻す、のそれぞれでテストが失敗すること。

**完了条件**: 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）が通る。用語集は `cmd/yt2column-server` と `internal/pipeline` のガードも表として読むので、表の形を保つ。

#### ステップ 2-3: ガイドの PR-2 の項目

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`、`fixprguard_guide.md`、`cmd/fixprguard/docs_test.go`

-   [ ] 日本語版のガイドの信頼する著者の説明と `--reviewer` の行に、渡す名前は `_context.md` の `Automated reviewer list` のすべての名前であることを加える。
-   [ ] §9 に、自動レビュアーを一覧に加えるときと外すときの手順、急いで信頼をやめるときの手順を加える（設計書 3.5 のガイドの PR-2 の項目、9 章の 1・2 つめの項目）。一覧に加える手順と外す手順には、同じコミットで変えるものをすべて挙げる。`_context.md` の行、内容のガードの期待する名前、写しの表の名前がこれに当たる。そのうえで、文書のガードをそのコミットで実行すると書く。
-   [ ] 日本語版をコミットしてから、英語版を翻訳する。
-   [ ] ステップ 1-5 のガイドのテストに、次の確かめを加える（AC-12 の「一覧の名前を 1 つずつ `--reviewer` で渡す」）。
    -   信頼する著者の説明と、フラグの表の `--reviewer` の行が `Automated reviewer list` を名指すこと。
    -   §9 の手順が `cmd/yt2column/fixpr_docs_test.go` を名指し、そのファイルが実在すること。

**壊して確かめる**: 両方のガイドのそれぞれで、`--reviewer` の行の `Automated reviewer list` への言及を消すと、テストが失敗すること。

**完了条件**: `go test -tags test ./cmd/fixprguard` が通る。

#### ステップ 2-4: フェーズ 2 の確かめ

**変更するファイル**: なし

-   [ ] `make test && make lint` を実行する。
-   [ ] 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）を実行する。

**完了条件**: 上の 2 つ。PR を作った後の観察は 4.3 に書く。

### PR-2 作成ポイント: command documents pass the reviewer list

**対象ステップ**: 2-0 / 2-1 / 2-2 / 2-3 / 2-4

**推奨タイトル**: `feat(0033): _context.md に自動レビュアーの一覧と待つ PR レビュアーを置き、コマンドの文書から一覧を渡す`

**レビュー観点**: `_context.md` の 2 行の形と、内容のガードがその形を厳密に読み、壊れた写しの表で検査ごとに失敗すること / `fixpr.md`・`runpr.md` が名前を書かずに一覧を参照し、実行の最初に 1 度だけ並びを作ると定めること / `runpr.md` step 3 が `<awaited-reviewer>[bot]` だけを変え、照合の行・LAST_REVIEWED・依頼のコメントを変えないこと / このブランチの `/runpr`・`/fixpr` には PR-1 の後のツールが必要なこと（マージの前の `make install-fixprguard`）と、`main` を取り込んでいない開いたブランチでは Codex のスレッドが想定外の著者のスレッドとして報告されること（設計書 5.3）

**実装モデル要件**: frontier-required

**判定理由**: 段階的な切り替え（ツール → 再インストール → 文書。設計書 6.2・8 章）の 2 段目で、すべての `/fixpr`・`/runpr` のセッションが Codex のスレッドを信頼し始める変更である（`mkplan.md` step 8 の panel mode の契機の、セキュリティのゲートと段階的な切り替え）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の目安 |
|---|---|---|
| M1 | `Reviewers` と `ParseReviewers`、判定の変更 | ステップ 1-2 の完了条件 |
| M2 | `cmd/fixprguard` のフラグ、呼び出しのガード、ガイドの PR-1 の項目 | PR-1 のマージ |
| M3 | 利用者の `make install-fixprguard` | ステップ 2-0 の完了条件 |
| M4 | `_context.md`、コマンドの文書、用語集、ガイドの PR-2 の項目、内容のガード | PR-2 のマージ |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 | `Reviewers`、信頼の判定、`--reviewer` の繰り返し、呼び出しのガード、ガイドの PR-1 の項目、`package_reference.md` | frontier-required |
| PR-2 | 2-0 / 2-1 / 2-2 / 2-3 / 2-4 | ツールの版の確かめ、`_context.md` の 2 行、`fixpr.md`・`runpr.md`、用語集、ガイドの PR-2 の項目、内容のガード | frontier-required |

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。

-   PR-1 はコマンドの文書を変えないので、PR-1 自体の `/runpr`・`/fixpr` は今のツールで動く。
-   PR-2 の内容のガードは PR-1 の `ParseReviewers` を使うので、PR-1 より先にはマージできない。
-   フェーズ 1 の中では、型（1-1）→ 判定（1-2）→ フラグ（1-3）の順に置く。`internal/fixprguard` の変更を `cmd/fixprguard` より先に置くためである。ステップ 1-2 の後は `cmd/fixprguard` のビルドが通らないので、ステップ 1-2 と 1-3 は同じ PR の中で続けて行う。
-   フェーズ 2 の中では、版の確かめ（2-0）を最初に置く。`_context.md` とコマンドの文書は、ガードと同じコミットで変える（2-1）。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

設計書 7.1 の項目を、ステップ 1-1〜1-3 に割り当てた。GitHub は `cmdrunner.Runner` の fake で置き換え、`gh` を呼ばない（設計書 7 章）。`newTestGitHub` は 1 つの名前のまま使い、既存のテストが変更なしで通ることで AC-04 を確かめる。2 つの名前の判定は、名前の並びを指定して作るヘルパーで確かめる。

### 4.2. 文書のガード

-   呼び出しのガード（ステップ 1-4）と、ガイドと `package_reference.md` のテスト（ステップ 1-5・2-3）は PR-1 から入る。内容のガード（ステップ 2-1・2-2）は PR-2 で入る。どれも CI の `doc-guard` のジョブが実行する（AC-14）。
-   内容のガードの `_context.md` の検査と、呼び出しのガードの検査は、関数に切り出し、表で失敗を確かめる（1.4）。文書の文の検査は、ステップ 2-1 の「壊して確かめる」で一時的に書き換えて確かめる。

### 4.3. 結合テスト

-   ステップ 1-3 の `begin` → `reply` のテスト（設計書 7.4。1.4）。
-   PR-2 を作った後、PR-2 のブランチで始めた `/runpr` で、インストール済みの新しいツールと PR-2 の文書の組み合わせが動くことを観察する（`manual`。設計書 6.2 の表の右下）。`/runpr` は PR のブランチに切り替える前に `runpr.md` と `_context.md` を読む（設計書 6.2 の末尾）。そのため、別のブランチから始めると、PR-2 の文書を確かめたことにならない。

### 4.4. セキュリティテスト

設計書 7.4 のセキュリティテストと、構築の検査を、次のステップで確かめる。

-   判定の表の「一致しない `Bot`」「名前が一致する `User`」「大文字と小文字の違い」、`[bot]` 付きの行、渡し忘れの行（ステップ 1-2）。
-   `--reviewer` がないとき、重複・文法違反のときに、GitHub にも git にも触れず、診断に値を書かないこと（ステップ 1-3）。
-   `ParseReviewers` の外で `Reviewers` を作るか書き換える形を、構築の検査が報告すること（ステップ 1-1）。
-   内容のガードが、3 つめの名前を加えた一覧と、名前を替えた一覧で失敗すること（ステップ 2-1）。一覧とガードの期待値を同時に変えた場合は検出しない（設計書 5.3）。

### 4.5. 後方互換性

-   `--reviewer` を 1 回だけ渡す呼び出しは、今と同じ結果になる（AC-04）。4.1 の既存のテストで確かめる。
-   PR-1 のマージから PR-2 のマージまでの間、今の文書は新しいツールで今までどおり動く（設計書 6.2 の表）。PR-1 のマージの後に別の作業の `/runpr` を実行すれば、今の文書と新しいツールの組み合わせを観察できる。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は振る舞いを確かめる Go のテスト、`static` は文書を読む Go のガードのテスト、`manual` は PR の上での観察を表す。新しいテストの名前は仮の名前で、実装で変えたときは本書の表を同じ PR で改める（1.3.5）。

| AC | 検証 | 場所 |
|---|---|---|
| AC-01 | static | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`（行の数、ちょうど 2 つの名前、説明の文）、`::TestContextReviewerRowsMalformed`（写しの表。ステップ 2-1） |
| AC-02 | static | `cmd/yt2column/fixpr_docs_test.go::TestContextReviewerRowsMalformed`（空、文法違反、重複の行。`ParseReviewers` の種類の問題） |
| AC-03 | static | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`（値と説明の文）、`::TestContextReviewerRowsMalformed`（待つ PR レビュアーの行） |
| AC-04 | test | `cmd/fixprguard/main_test.go::TestReviewerFlagRepeats`（違う名前の 2 回。ステップ 1-3）、`::TestReviewerOrderIrrelevant`（順序）、`::TestReviewerFlag` と `--reviewer` を 1 回だけ渡す既存のテスト、`internal/fixprguard` の 1 つの名前の `newTestGitHub` を使う既存のテスト |
| AC-05 | test | `internal/fixprguard/trust_test.go::TestAssessAuthors`（1 つめ・2 つめ・混ざったスレッド・大文字と小文字）、`internal/fixprguard/github_test.go::TestThreadPathsRequiresTrustedOrNamed`、`internal/fixprguard/write_test.go::TestReplyTrustsEveryReviewer`、`cmd/fixprguard/main_test.go::TestSecondReviewerBeginReply` |
| AC-06 | test | `internal/fixprguard/trust_test.go::TestAssessAuthors`（一致しない `Bot`、名前が一致する `User`、渡し忘れ）、`::TestAssessUnverifiableAuthor`（`bot_login_outside_grammar`）、`internal/fixprguard/write_test.go::TestReplyRefusesUntrustedThread` |
| AC-07 | test | `internal/fixprguard/types_test.go::TestParseReviewers`、`cmd/fixprguard/main_test.go::TestAcceptThreadFlagParsing`（4 つのサブコマンドそれぞれの、`--reviewer` がない・重複・文法違反の行。標準エラーも確かめる）、`::TestReviewerFlag` |
| AC-08 | test | `internal/fixprguard/github_test.go::TestBeginAuthorType` |
| AC-09 | test | `cmd/fixprguard/main_test.go::TestReviewerFlag` |
| AC-10 | static | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`（定義の文）、`cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`（ちょうど 1 つの `<reviewer>`）、`::TestReviewerInvocationCheck`（合成した呼び出しの表。ステップ 1-4） |
| AC-11 | static | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`（名前が依頼のコメントの外に現れない。`<awaited-reviewer>` の定義の文）、`::TestRunprDocContract`（step 3 の「Reviewed」の部分）、`::TestRemovedCommandLiterals`（`<reviewer>[bot]` と `"Automated reviewer"`） |
| AC-12 | static | `cmd/yt2column/fixpr_docs_test.go::TestFixprDocContract`（`fixpr.md`）、`cmd/fixprguard/docs_test.go::TestGuideDescribesReviewerList`（ガイドの両方の版。ステップ 1-5・2-3）、`cmd/yt2column/docs_test.go::TestPackageReferenceFixprguard`（`package_reference.md`） |
| AC-13 | static | `cmd/yt2column/fixpr_docs_test.go::TestGlossaryTerms` |
| AC-14 | static + manual | `make test`・`make lint`、文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）。PR-1 と PR-2 の CI の `doc-guard` のジョブの結果、4.3 の観察 |

AC-01〜AC-03・AC-10〜AC-13 は、`_context.md`・コマンドの文書・ガイド・用語集が何を述べるかを定めるので、`static` で確かめる。

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6）
- [ ] PR-2 マージ済み（対象ステップ: 2-0 / 2-1 / 2-2 / 2-3 / 2-4）

## 7. リスク管理 (Risk Management)

### 7.1. 技術的リスク

| リスク | 対策 |
|---|---|
| PR-1 のマージの後、再インストールを忘れて PR-2 に進む | ステップ 2-0 で止まる。PR-1 の作成ポイントの「マージの後」と「レビュー観点」は案内にとどまり、順序を守らせるのはステップ 2-0 だけである。忘れても、PR-2 の `/runpr` が使い方のエラーで止まるだけで、誤った判定はしない（設計書 6.2） |
| `--protocol` を上げないので、古いツールが番号の食い違いで止まらない | 古いツールは `flag given twice: --reviewer` で止まる（1.3.4）。ガイドの終了コード `2` の説明が再インストールを案内する（ステップ 1-5） |
| 判定の 4 か所と `processThread` のどれかが 1 つの名前の比べ方のまま残る | `GitHub.reviewer` を消すので、残りはビルドで見つかる（1.3.2）。各箇所の 2 つめの名前のテストを、壊して確かめる（ステップ 1-2） |
| 同じパッケージのコードが `ParseReviewers` を通さずに `Reviewers` を作るか広げる | 構築の検査を広げ、`logins` のフィールドを参照できる関数を限る（1.3.3、ステップ 1-1） |
| 修正のフェーズが一覧と内容のガードの期待値を同時に書き換える | 設計書 5.3 のとおり受け入れ、マージの前の人のレビューで見つける |
| 開いているブランチの文書が PR-2 の前のまま、Codex のスレッドを想定外の著者のスレッドとして報告する | 安全側の振る舞いである。ブランチに `main` を取り込めば解消する（設計書 5.3）。PR-2 の「レビュー観点」に書き、PR の本文に載せる |

### 7.2. スケジュールのリスク

-   PR-2 は、PR-1 のマージと、利用者の `make install-fixprguard` の実行の後でなければ始められない（3.1 の M3）。再インストールが済んでいなければ、ステップ 2-0 で止まり、先に進まない。
-   CodeRabbit の PR レビューは 1 時間あたりの回数が限られるので、PR を開いてからレビューが届くまで待つことがある。その間は、ローカルの CodeRabbit レビューで見つかった指摘の修正を進める。依頼から約 15 分たってもレビューが届かなければ、`/runpr` は止まって報告する（`runpr.md` step 3）。その場合は、回数の制限が戻ってから `/runpr` を実行し直す。

## 8. 成功基準 (Success Criteria)

-   [ ] 5 章のすべての `test` と `static` の検証が通る。
-   [ ] `make test && make lint` と、PR-1・PR-2 の CI の `doc-guard` のジョブが通る。
-   [ ] 各ステップの「壊して確かめる」の結果が、コミットメッセージに書かれている。
-   [ ] ガイドの日本語版と英語版の記述が一致する。

## 9. 次のステップ (Next Steps)

-   承認の後、`/runplan 0033` で PR-1 から実装する。PR-1 のマージの後は、利用者が `make install-fixprguard` を実行してから PR-2 に進む。
-   PR-2 のマージで、タスク 0031（#403）が `Awaited PR reviewer` の値と `runpr.md` step 3 を Codex 用に変えられるようになる（設計書 9 章）。
-   PR-2 は本タスクの最後のプランの PR なので、マージの後に issue #405 を閉じる。
