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

用語は設計書の冒頭と要件書 6 節に従う。特に「内容のガード」（`cmd/yt2column/fixpr_docs_test.go`）と「呼び出しのガード」（`cmd/fixprguard/docs_test.go` の `TestDocumentedInvocationsParse`）は、設計書の冒頭の定義で使う。

### 1.2. 実装原則

-   各ステップは設計書の節を参照し、設計の内容を本書で言い換えない。
-   Go のコメント・識別子・文字列リテラルは英語で書く。例外は、用語集の日本語のセルと一致させる `TestGlossaryTerms` のリテラルだけである（`CLAUDE.md`「Go Idioms」）。コマンドの文書（`.claude/commands/`）に書く文も英語で書く（`CLAUDE.md`「Language」）。
-   テストは、対象の実装を壊して失敗することを確かめてからコミットし、壊し方と結果をコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。各ステップの「壊して確かめる」は、その対象を挙げる。壊し方の候補は設計書 7.2 にある。
-   Go のファイルを変えたら `make fmt` を実行する。各フェーズの最後に `make test && make lint` を実行する。
-   ソースのコメントに `AC-NN`・`F-NNN` を書かない（要件書との対応は本書の 5 章に書く。`docs/dev/developer_guide/requirements_process.md` 4 節）。

### 1.3. 既存コード調査結果

調査はコミット `819011a` で行った。設計書が確かめたコミット `e5db595` から、`internal/`・`cmd/`・`.claude/`・`docs/dev/`・`docs/translation_glossary.md`・`CLAUDE.md` に変更はない（`git diff --stat e5db595 819011a -- internal cmd .claude docs/dev docs/translation_glossary.md CLAUDE.md` の出力が空）。そのため、設計書の `file:line` は `819011a` でもそのまま使える。

#### 1.3.1. 変える箇所の一覧

数え上げは、`rg -n --hidden` で `Automated reviewer`・`automated reviewer`・`<reviewer>`・`reviewer bot`・`coderabbitai[bot]`・`<reviewer>[bot]` を、また `rg -n --type go` で `NewGitHub(`・`assessThread(`・`processThread(`・`requireReviewer`・`.reviewer` を探して行った（`docs/tasks/` を除く。`819011a`）。

| 箇所 | 今 | 変え方 | ステップ |
|---|---|---|---|
| `internal/fixprguard/types.go` | `Login` と `ParseLogin` だけがある | `Reviewers` と `ParseReviewers` を加える（設計書 3.2） | 1-1 |
| `internal/fixprguard/construction_test.go:17-21` | `restrictedConstructors` は `Login`・`PRNumber`・`Baseline` | `Reviewers`→`ParseReviewers` を加える。`zeroLiteralAllowed` には入れない。`findRestrictedConstructions` のコメント（`:28-32`）の「`NewGitHub` rejects a zero Login」を改める | 1-1 |
| `internal/fixprguard/github.go:30-63` | `GitHub.reviewer Login`、`NewGitHub(run, reviewer Login)`、`requireReviewer` が `Login` を確かめる | `Reviewers` に替える（設計書 2.2）。`NewGitHubWithoutReviewer`（`:47-56`）は変えない | 1-2 |
| `internal/fixprguard/github.go:328`・`:466`・`:519`、`write.go:126` | `assessThread(..., g.reviewer, ...)` | `g.reviewers` を渡す | 1-2 |
| `internal/fixprguard/github.go:380`・`:386-394` | `processThread(t, isNamed, g.reviewer)`。`ParseLogin` を通さない `first.loginRaw` を文字列で比べる | `Reviewers` を渡し、`ParseLogin` を通した名前で比べる（設計書 2.4・3.3）。コメントを改める | 1-2 |
| `internal/fixprguard/trust.go:62-66`・`:86` | `assessThread(..., reviewer Login, ...)`、`a.login != reviewer` | `Reviewers` を受け取り、一覧のどれかと一致するかで判定する（設計書 3.3）。コメントを改める | 1-2 |
| `internal/fixprguard/test_helpers.go:19`・`:56-59`・`:205-213` | `testReviewer` は 1 つ。`newTestGitHub` は `mustLogin(t, testReviewer)` で作る | 2 つめのテスト用の名前を加え、`newTestGitHub` を `Reviewers` で作る。名前を指定して作る手段を加える（1.3.5） | 1-2 |
| `internal/fixprguard/github_test.go:38-46`・`:415` | `NewGitHub(fake, Login{})`、`NewGitHub(runner, mustLogin(...))` | `Reviewers` に替える | 1-2 |
| `cmd/fixprguard/main.go:175-195` | `flagValue` のコメントと `multi` の判定（`--accept`・`--thread` だけ） | `--reviewer` を値を集めるフラグに加える。コメントを改める | 1-3 |
| `cmd/fixprguard/main.go:213`・`:218-221` | 値を集めるフラグの指定、必須のフラグの確認は `inv.values` だけを見る | 値を集めるフラグにも必須の確認が働くようにする（設計書 2.4） | 1-3 |
| `cmd/fixprguard/main.go:375-383` | `githubFor` が `ParseLogin(inv.values[flagReviewer])` を呼び、エラーにフラグの名前を付けない | `ParseReviewers(inv.multi[flagReviewer])` を呼び、エラーに `--reviewer` を付ける（設計書 4 章） | 1-3 |
| `cmd/fixprguard/main_test.go:95-99` | 「`--reviewer` twice」の行（値を 1 つだけ持つフラグの 2 回の拒否の唯一のテスト） | 別のフラグを 2 回渡す行に置き換える（設計書 1.4 の表） | 1-3 |
| `cmd/fixprguard/test_helpers.go:22` | `testReviewer` は 1 つ | 2 つめのテスト用の名前を加える | 1-3 |
| `cmd/fixprguard/docs_test.go:57-59`・`:86-109` | `reviewerPlaceholder` のコメント。`inv.values[flagReviewer]` を確かめる | `inv.multi` を読み、値がちょうど 1 つの `<reviewer>` であることを確かめる（設計書 3.4） | 1-4 |
| `docs/dev/developer_guide/fixprguard_guide.ja.md`・`fixprguard_guide.md` の `:11`・`:58`・`:60`・`:70`・`:87`・`:100`・`:106`・`:141`・`:175`・`:246`・`:276` | 1 つの `--reviewer` の記述 | 設計書 3.5 のガイドの PR-1 の項目（`:70` は終了コード `2` の行） | 1-5 |
| `docs/dev/developer_guide/package_reference.md:21` | 「the reviewer bot's `Login`」「by the reviewer bot」 | 設計書 3.5 の `package_reference.md` の項目 | 1-5 |
| `.claude/commands/_context.md:185` | `Automated reviewer` の行 | `Automated reviewer list` と `Awaited PR reviewer` の 2 行に置き換える（設計書 1.2・3.4） | 2-1 |
| `cmd/yt2column/fixpr_docs_test.go:41-44`・`:75-86`・`:336-363` | `reviewerSentence`、最初に一致した行を返す `contextRow`、`TestContextAndCommandsAgree` | 設計書 3.4 の検査に改める（1.4 の決定） | 2-1・2-2 |
| `.claude/commands/fixpr.md:44-47`・`:94-95`・`:142-143` | `<reviewer>` の定義、ラウンド上限、「Untrusted threads」 | 設計書 3.5 | 2-2 |
| `.claude/commands/runpr.md:18-19`・`:216-224` | `<reviewer>` の定義、step 3 の `<reviewer>[bot]` | 設計書 3.5（`<awaited-reviewer>`） | 2-2 |
| `docs/translation_glossary.md:68` | 「信頼する著者」の定義が `_context.md` の 1 つの名前を指す | 設計書 2.2（AC-13） | 2-3 |
| `cmd/yt2column/fixpr_docs_test.go:419-450` | `TestGlossaryTerms` | 3 行を加え、「信頼する著者」の定義を確かめる | 2-3 |
| ガイドの日本語版と英語版 | `_context.md` の一覧への言及がない。一覧から外す手順がない | 設計書 3.5 のガイドの PR-2 の項目、9 章 | 2-4 |

変えない箇所は次のとおりである。

-   `@coderabbitai review` を含む `.claude/commands/_lib/task-pr-handoff.md:190`・`_lib/pr-review-escalation.md:9`・`CLAUDE.md:56`・`.claude/commands/runpr.md:238`（設計書 2.2 の末尾。0031 の範囲）。
-   ガイドの各節の呼び出しの例（`fixprguard_guide.ja.md:92`・`:113`・`:146`・`:182`、英語版の同じ箇所）。`--reviewer` を 1 つ書いた形のままでよい（設計書 3.5）。
-   `internal/fixprguard/types_test.go:30` と `trust_test.go:116` の `coderabbitai[bot]` は、文法に合わない名前の入力の例なので残す。
-   `cmd/fixprguard/test_helpers.go:246` の `reviewerThread` のコメント「opened by the reviewer bot」は、テスト用の 1 つめの名前の Bot を指すので、意味は変わらない。

#### 1.3.2. 名前を変えるものの一覧

| 今の名前 | 新しい名前 | 場所 | ステップ |
|---|---|---|---|
| `_context.md` の行 `Automated reviewer` | `Automated reviewer list` と `Awaited PR reviewer` の 2 行 | `.claude/commands/_context.md` | 2-1 |
| プレースホルダー `<reviewer>[bot]` | `<awaited-reviewer>[bot]` | `.claude/commands/runpr.md` step 3 | 2-2 |
| フィールド `GitHub.reviewer`（`Login`） | `GitHub.reviewers`（`Reviewers`） | `internal/fixprguard/github.go` | 1-2 |

本書の各ステップ、5 章の表、6 章のチェックリストは、この表の名前で書く。古い名前が残っていないことは、ステップ 1-2 ではビルドで、ステップ 2-1・2-2 では内容のガード（`TestRemovedCommandLiterals` に加える行。ステップ 2-2）で確かめる。

#### 1.3.3. 型の構築の経路

`Reviewers` は、使えない値の扱いを型で保証する型である（設計書 3.2）。ゼロ値は「名前なし」で、`NewGitHub` が拒否し、判定する呼び出しは `requireReviewer` で拒否する。使える値の構築を `ParseReviewers` だけに限る方法として、本書は、既存の `TestRestrictedTypesBuiltByConstructors`（`internal/fixprguard/construction_test.go:84-113`）に登録する方を採る。`Reviewers` を葉のパッケージに移す案は採らない。理由は次のとおりである。

-   `Login`・`PRNumber`・`Baseline` が同じ仕組みで守られており、検査はパッケージの中の、テストでないすべてのファイルのリテラル・`new`・`var` 宣言を報告する（`construction_test.go:33-71`・`:84-113`、`819011a`）。
-   この検査が報告しない形は、構造体のフィールド、関数の引数、名前付きの戻り値である（`construction_test.go:28-32`）。`GitHub.reviewers` のフィールドと `assessThread`・`processThread` の引数はこれに当たるが、どれもゼロ値になるだけで、`requireReviewer` が拒否する。
-   `NewGitHubWithoutReviewer`（`github.go:51-56`）は `GitHub` のリテラルで `Reviewers` のフィールドを省くので、検査に触れない（設計書 3.2）。

ステップ 1-1 は、`Reviewers` のリテラル・`new`・`var` 宣言の各形が報告されることを、自己テストの行で確かめる。

#### 1.3.4. 外部の前提の確かめ

-   **古いツールの診断:** インストール済みのツール（`go version -m "$(command -v fixprguard)"` の `vcs.revision` は `aedefac`、#416 のマージコミット）に、作業ツリーの外で `fixprguard list --protocol 1 --reviewer coderabbitai --reviewer chatgpt-codex-connector 1` を渡した。結果は終了コード `2` で、標準エラーは次の 1 行だった（2026-10-22）。

    ```
    fixprguard: usage: invalid value "chatgpt-codex-connector" for flag -reviewer: flag given twice: --reviewer (see docs/dev/developer_guide/fixprguard_guide.md)
    ```

    設計書 1.2 の 3 つめの項目が使う文字列 `flag given twice: --reviewer` が含まれる。ガイドの案内（ステップ 1-5）は、この文字列で古いツールを見分ける。
-   **ツールの版の確かめ:** `go version -m` が `vcs.revision` を表示することを、上のとおり確かめた。フェーズ 2 の前提の確かめ（2 章のフェーズ 2 の冒頭）に使う。
-   **fake が呼ばれたときの失敗:** `cmdrunnertestutil.Fake` は、手順にない呼び出しで `Fatalf` する（`internal/cmdrunner/testutil/mocks.go:64`、`819011a`）。AC-07 の「GitHub にも git にも触れない」は、手順を持たない fake の `gh` と git を渡して確かめる。既存の表（`cmd/fixprguard/main_test.go:100-128`）と同じ方法である。
-   **`go tool cover -func`:** `go test -tags test -coverprofile=<file> ./cmd/fixprguard` の後に `go tool cover -func=<file>` で関数ごとのカバレッジを表示する。ステップ 1-3 で、行の置き換えの前後で `flagValue.Set` のカバレッジを比べるのに使う。

#### 1.3.5. 実装時に決めること

-   `ParseReviewers` のエラーの規則の名前と、`githubFor` の診断の文言（設計書 4 章が実装に委ねた範囲）。
-   テスト用の 2 つめの名前（実在の `chatgpt-codex-connector` を使うかを含む）と、名前を指定して `GitHub` を作るテストのヘルパーの形。
-   値を集めるフラグの指定の書き方（`parse` の式に `--reviewer` を足すか、`spec` に持たせるか）。
-   コマンドの文書の英語の文面と、内容のガードが固定する文。設計書 3.5 の趣旨を変えない範囲で決める。
-   新しいテストの関数の名前。5 章の表に書いた名前と変えたときは、同じ PR で本書の表を改める。

### 1.4. 計画の際に決めたこと（2026-10-22）

-   **内容のガードの `_context.md` の検査は、本文を受け取って問題を返す関数に切り出し、壊れた写しの表をテストとしてコミットする:** 設計書 7.3 は「`_context.md` の写しを壊して確かめる」と書くが、その確かめを残すテストにするかは決めていない。表にすれば、ガードの検査が後で緩められたときに CI で検出できる。実装のときに一時的に壊すだけでは、その時点の確かめにとどまる。
-   **`cmd/fixprguard` の結合テストは、新しいテストとして加え、設計書 7.4 を本 PR で直す:** 設計書 7.4 は「既存の流れのテスト（`begin` から `reply` まで）」に場合を加えるとするが、`begin` と `reply` を続けて実行する既存のテストはない（`cmd/fixprguard/main_test.go`。2 回以上 `run` を呼ぶテストは `TestFenceSubcommand` と `TestStopKinds` だけで、どちらも別々の状況を 1 回ずつ実行する。`819011a`）。新しいテストを加えることは設計書の意図どおりなので、設計書の誤りは editorial correction として直し、承認を保つ（`docs/dev/developer_guide/requirements_process.md`「Editing an approved document」）。
-   **PR は設計書 8 章のとおり 2 つにする:** `NewGitHub` の引数の型が変わるので、`internal/fixprguard` と `cmd/fixprguard` は同じ PR でなければビルドが通らない。ガイドの PR-1 の項目はツールの振る舞いの記述なので、コードと同じ PR でレビューするほうが食い違いを見つけやすい。

### 1.5. 申し送りの項目への対応

-   `design_handoff.md` の H-01〜H-04 は、すべて設計書の付録 A に記録されている。H-01 の順序はフェーズの分け方（1.1）とフェーズ 2 の前提の確かめに、H-04 の壊し方は各ステップの「壊して確かめる」に落とした。
-   `implementation_handoff.md` はない。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: ツール（設計書 8 章の 1）

#### ステップ 1-1: `Reviewers` と `ParseReviewers`

**変更するファイル**: `internal/fixprguard/types.go`、`types_test.go`、`construction_test.go`

-   [ ] `Reviewers` と `ParseReviewers` を、設計書 3.2 の型とコメントのとおりに加える。名前を含むかを調べるメソッドと使えるかを調べるメソッドは、パッケージの外に出さない。
-   [ ] `types_test.go` に `TestParseReviewers` を加える。設計書 7.1 の `ParseReviewers` の項目の場合を確かめる。エラーは `errors.Is(err, ErrInvalidInput)` で確かめ、エラーの文字列が入力の値を含まないことを確かめる。
-   [ ] `restrictedConstructors` に `Reviewers`→`ParseReviewers` を加え、`findRestrictedConstructions` のコメントを `Reviewers` に合わせる。`TestRestrictedTypesBuiltByConstructors` の自己テストの表に、`ParseReviewers` の中のリテラルは報告されないこと、外のリテラル・`&` 付きのリテラル・`new`・`var` 宣言は報告されることの行を加える。

**壊して確かめる**: 重複の確認を外す、空の一覧の確認を外す、比べる前に大文字と小文字をそろえる、の 3 つで、それぞれ `TestParseReviewers` の該当の行が失敗すること。`restrictedConstructors` から `Reviewers` を外すと、自己テストの行が失敗すること。

**完了条件**: `go test -tags test ./internal/fixprguard` が通る。

#### ステップ 1-2: 判定に `Reviewers` を使う

**変更するファイル**: `internal/fixprguard/github.go`、`trust.go`、`write.go`、`test_helpers.go`、`trust_test.go`、`github_test.go`、`write_test.go`

-   [ ] `GitHub`・`NewGitHub`・`requireReviewer`・`assessThread`・`processThread`・`Reply` の判定を、設計書 2.2 の表と 3.3 のとおりに `Reviewers` に替える。コメント（`github.go:35-36`・`:386-389`、`trust.go:62-65`）を改める。
-   [ ] `test_helpers.go` に 2 つめのテスト用の名前を加え、`newTestGitHub` を 2 つの名前の `Reviewers` で作る。既存のテストのうち 1 つの名前を前提にするものは、1 つの名前で作る手段を使う（AC-04 の「1 回だけ渡す呼び出しは今と同じ」を、既存の行がそのまま通ることで確かめる）。
-   [ ] `TestNewGitHub` を、ゼロ値の `Reviewers` を拒否する形に改める。
-   [ ] `TestAssessAuthors` に、設計書 7.1 の判定の表の 6 つの場合を別々の行として加える。`TestAssessUnverifiableAuthor` の `bot_login_outside_grammar` を、2 つの名前の `Reviewers` で理由 (c) になることを確かめる形にする。
-   [ ] `TestBeginAuthorType` を、設計書 7.1 の `authorType` の 3 つの場合を確かめる形にする。
-   [ ] `TestThreadPathsRequiresTrustedOrNamed` に、2 つめの名前の `Bot` のスレッドが `Named` でない場合を加える。
-   [ ] `write_test.go` に、2 つめの名前の `Bot` のスレッドに名指しなしで返信できることを確かめるテストを加える。一覧にない `Bot` のスレッドに書き込まないことは、既存の `TestReplyRefusesUntrustedThread`（`write_test.go:152-161`、`819011a`。`other-bot` の Bot で `ErrThreadNotTrusted`）が、2 つの名前の `newTestGitHub` のまま確かめる。

**壊して確かめる**: `assessThread` が一覧の名前のうち 1 つとだけ比べる（`TestAssessAuthors` の 2 つめの名前の行）。`processThread` が 1 つめの名前とだけ比べる（`TestBeginAuthorType` の 2 つめの名前の行）。`Reply` が `assessThread` に 1 つめの名前だけの `Reviewers` を渡す（返信のテスト）。`ThreadPaths` が同じく 1 つめの名前だけを渡す（`TestThreadPathsRequiresTrustedOrNamed` の新しい行）。

**完了条件**: `go test -tags test ./internal/fixprguard` が通る。`cmd/fixprguard` はステップ 1-3 までビルドが通らないので、`make test` はステップ 1-3 の後に実行する。

#### ステップ 1-3: `cmd/fixprguard` のフラグ

**変更するファイル**: `cmd/fixprguard/main.go`、`main_test.go`、`test_helpers.go`

-   [ ] `--reviewer` を値を集めるフラグにし、値を集めるフラグにも必須の確認が働くようにする。`flagValue` のコメントを改める（設計書 2.2 の `main.go` の行）。
-   [ ] `githubFor` が `ParseReviewers` を呼び、エラーに `--reviewer` を付けるようにする（設計書 4 章）。`githubFor` のコメントを改める。
-   [ ] `test_helpers.go` に 2 つめのテスト用の名前を加える。
-   [ ] 違う名前の `--reviewer` を 2 回渡すと `parse` が受け付け、2 つの値を集めることを確かめるテストを加える。
-   [ ] `TestAcceptThreadFlagParsing` の「`--reviewer` twice」の行を、値を 1 つだけ持つフラグ（`inspect` の `--baseline`）を 2 回渡す行に置き換える。置き換えの前後で `go tool cover -func` の `flagValue.Set` のカバレッジが変わらないことを確かめ、コミットメッセージに書く（1.3.4）。
-   [ ] 同じテストの「コマンドを起動しない」表（`main_test.go:100-128`）に、`list`・`begin`・`inspect`・`reply` のそれぞれで、同じ名前を 2 回渡す行と、文法に合わない値を渡す行を加える。各行のほかの引数は正しい値にする。この 8 行では、終了コード `2` に加えて、標準エラーが 1 行で `--reviewer` を含むことも確かめる。既存の `begin` の `bad name` の行は、加える行と重なるので、加える行に含める。
-   [ ] `list` と `begin` について、`--reviewer` の順序を逆にしても標準出力が同じであることを確かめるテストを加える（設計書 7.1 の「順序によらないこと」）。
-   [ ] 2 つの名前を渡し、2 つめの名前の `Bot` のスレッドを `begin` で処理し、`begin` の出力の指紋で `reply` するテストを加える（設計書 7.4 の結合テスト。1.4）。
-   [ ] `TestReviewerFlag` と、`--reviewer` を 1 回だけ渡す既存のテストが、変えずに通ることを確かめる（AC-04・AC-09）。

**壊して確かめる**: `inspect` で `githubFor` の呼び出しを `CheckState` の後に動かすと、`inspect` の重複の行が失敗すること（fake の git が呼ばれる）。`githubFor` がエラーに `--reviewer` を付けないと、8 行の標準エラーの確かめが失敗すること。必須の確認を `inv.values` だけに戻すと、`TestReviewerFlag` の `--reviewer` を消した行が失敗すること。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

#### ステップ 1-4: 呼び出しのガード

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [ ] `TestDocumentedInvocationsParse` が、`--reviewer` を取る各呼び出しで、`inv.multi` の `--reviewer` の値がちょうど 1 つで、それが `<reviewer>` であることを確かめるようにする（設計書 3.4 の呼び出しのガード）。テストのコメントと `reviewerPlaceholder` のコメントを改める。

**壊して確かめる**: `fixpr.md` の `fixprguard begin` の呼び出しの写しに `--reviewer <reviewer>` をもう 1 つ足した入力と、`--reviewer coderabbitai` に替えた入力で、ガードが失敗すること（文書は変えずに、一時的に `fixpr.md` を書き換えて確かめ、戻す）。

**完了条件**: `go test -tags test ./cmd/fixprguard` が通る。

#### ステップ 1-5: ガイドと `package_reference.md`

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`、`fixprguard_guide.md`、`package_reference.md`、`cmd/fixprguard/docs_test.go`

-   [ ] 日本語版のガイドを、設計書 3.5 のガイドの PR-1 の項目のとおりに改める。`_context.md` の行の名前には触れない。終了コード `2` の説明には、1.3.4 で確かめた古いツールの診断の文字列 `flag given twice: --reviewer` と、`go version -m "$(command -v fixprguard)"` の `vcs.revision` での確かめ方を書く。
-   [ ] 日本語版をコミットしてから、英語版を `.claude/commands/mktrans.md` の原則で翻訳する（`CLAUDE.md`「Translation Guidelines」）。
-   [ ] `package_reference.md` の `internal/fixprguard` の行を、設計書 3.5 の `package_reference.md` の項目のとおりに改める。
-   [ ] `cmd/fixprguard/docs_test.go` に、両方のガイドが次を述べていることを確かめるテストを加える。`--reviewer` が繰り返せること、1 つもないときと同じ名前を 2 回以上渡したときは使い方のエラーになること、`other-bot` が渡したどの名前とも一致しない Bot であること、古いツールの診断の文字列。あわせて、`package_reference.md` の `internal/fixprguard` の行が `ParseReviewers` と `Reviewers` を名指し、「the reviewer bot」を含まないことを確かめる。固定する語は、書いた文から選ぶ。
-   [ ] ガイドの記述をツールの振る舞いと突き合わせる。終了コード `2` の説明は、ステップ 1-3 の後のバイナリ（`go build -o <scratch>/fixprguard ./cmd/fixprguard`）に、作業ツリーの外で同じ名前を 2 回渡したときの診断に `flag given twice` が含まれないことを確かめる（設計書 1.2 の 3 つめの項目の「取り違えない」）。その出力をコミットメッセージに書く。

**壊して確かめる**: ガイドの日本語版と英語版のそれぞれで、固定した文を 1 つずつ一時的に消し、テストが失敗すること。

**完了条件**: `go test -tags test ./cmd/fixprguard` が通る。

#### ステップ 1-6: フェーズ 1 の確かめ

**変更するファイル**: なし

-   [ ] `.claude/commands/`・`_context.md` を変えていないことを、`git diff --stat "$(git merge-base origin/main HEAD)" -- .claude` の出力が空であることで確かめる（設計書 8 章の 1。PR-1 の `/runpr`・`/fixpr` が今のツールで動くため）。
-   [ ] `make test && make lint` と `make deadcode` を実行する。`make deadcode` が新しく報告する関数がないことを確かめる。
-   [ ] 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）を実行する。

**完了条件**: 上の 3 つ。

### PR-1 作成ポイント: fixprguard accepts repeated --reviewer

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6

**推奨タイトル**: `feat(0033): fixprguard で --reviewer を繰り返し受け付け、どれかと一致する Bot を信頼する`

**レビュー観点**: 判定の 4 か所（`list`・`begin`・`inspect`・`reply`）と `processThread` が同じ `Reviewers` を同じ比べ方で使うこと / `ParseReviewers` が唯一の構築関数で、重複・空・文法違反を拒否し、`githubFor` が GitHub と git に触れる前に呼ばれること / `--reviewer` を 1 回だけ渡す呼び出しの結果が変わらないこと（AC-04）と、`--protocol` を上げない例外のガイドの但し書き / ガイドの日本語版と英語版の一致

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-2 は、どのスレッドを名指しなしで修正のフェーズに渡すかを決める信頼の判定を広げる、セキュリティのゲートの変更である（`mkplan.md` step 8 の panel mode の契機）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

**マージの後**: 利用者が `main` を更新して `make install-fixprguard` を実行してから、`/runplan 0033` でフェーズ 2 に進む（設計書 8 章の 2）。今の `/runpr` と `/mergepr` は再インストールを案内しないので、ここに書く。

### フェーズ 2: 文書（設計書 8 章の 3）

**前提（このフェーズに入る前に）**: PR-1 がマージされ、利用者が `main` で `make install-fixprguard` を実行している（設計書 8 章の 2）。最初に、`go version -m "$(command -v fixprguard)"` の `vcs.revision` が PR-1 のマージコミットか、それより後の `main` のコミットであることを `git merge-base --is-ancestor <PR-1 のマージコミット> <vcs.revision>` で確かめる。`protocol` の番号は変わらないので、番号ではなく版で確かめる。満たされていなければ、このフェーズを始めず、利用者に `make install-fixprguard` の実行を求める。

#### ステップ 2-1: `_context.md` の 2 行と内容のガードの行の検査

**変更するファイル**: `.claude/commands/_context.md`、`cmd/yt2column/fixpr_docs_test.go`

-   [ ] `_context.md` の `Automated reviewer` の行を、設計書 1.2 と 3.4 の形の `Automated reviewer list` と `Awaited PR reviewer` の 2 行に置き換える。説明の文は、AC-01・AC-03 が求めることを述べ、1 つの Bot だけを信頼するとは述べない。
-   [ ] 内容のガードの `_context.md` の検査（設計書 3.4 の行の数の検査と 1〜4）を、`_context.md` の本文を受け取って問題の一覧を返す関数に切り出す（1.4）。`TestContextAndCommandsAgree` は、本物の `_context.md` でこの関数が問題を返さないことを確かめ、取り出した一覧と待つ PR レビュアーを、ステップ 2-2 の検査に使う。
-   [ ] 同じファイルに、本物の `_context.md` から作った壊れた写しの表で、この関数が問題を返すことを確かめるテストを加える。写しの壊し方は設計書 7.3 の列挙（空、文法違反、重複、区切りの違い、3 つめの名前、どちらかの行が 2 つ、今の `Automated reviewer` の行が残る）に、待つ PR レビュアーの名前が 2 つ・空・文法違反の場合と、` — ` がない場合を加える。

**壊して確かめる**: 関数から、行の数の確認、名前の並びの形の確認、ちょうど 2 つの名前の確認を 1 つずつ外し、表の該当の行が失敗すること（設計書 7.2）。

**完了条件**: `go test -tags test ./cmd/yt2column` が通る。

#### ステップ 2-2: `fixpr.md`・`runpr.md` と内容のガードの文の検査

**変更するファイル**: `.claude/commands/fixpr.md`、`runpr.md`、`cmd/yt2column/fixpr_docs_test.go`

-   [ ] 両方の文書の `<reviewer>` の定義を、設計書 3.5 の 2 つの趣旨（一覧の名前ごとに 1 回ずつの `--reviewer`、実行の最初に 1 度だけ作った並びを使い続けること）の文にする。名前そのものは書かない。
-   [ ] `fixpr.md` の「Untrusted threads」とラウンド上限を、設計書 3.5 のとおりに改める。
-   [ ] `runpr.md` step 3 の `<reviewer>[bot]` を `<awaited-reviewer>[bot]` に替え、`<awaited-reviewer>` の定義の文（`_context.md` の `Awaited PR reviewer`、REST の形は `[bot]` を付けたもの）を書く。照合の行、LAST_REVIEWED の決め方、`@coderabbitai review` の依頼のコメントは変えない（AC-11）。
-   [ ] `TestContextAndCommandsAgree` を、設計書 3.4 の 5・6 の検査に改める。`reviewerSentence` を新しい定義の文に替え、`<awaited-reviewer>` の定義の文の定数を加える。
-   [ ] `TestRunprDocContract` に、step 3 の 2 つの `jq` の絞り込みが `<awaited-reviewer>[bot]` を使い、照合の `grep` の行が今のままであることを確かめる行を加える。今のガードは、この 3 行を固定していない（`fixpr_docs_test.go` に `between` と `[bot]` の語はない。`819011a`）。
-   [ ] `TestFixprDocContract` に、「Untrusted threads」の信頼する著者の説明とラウンド上限の説明が一覧のどれかの自動レビュアーを指すことを確かめる行を加える（AC-12）。
-   [ ] `TestRemovedCommandLiterals` に、`fixpr.md`・`runpr.md` に古い参照（`("Automated reviewer"` の形の行の名前の参照と `<reviewer>[bot]`）が残らないことの行を加える（1.3.2）。

**壊して確かめる**: 文書に `chatgpt-codex-connector` を 1 回書く、`coderabbitai[bot]` を書く、`<reviewer>` の定義の文から「1 度だけ作る」の部分を消す、step 3 の片方の `jq` を `<reviewer>[bot]` に戻す、のそれぞれで、内容のガードが失敗すること（一時的に書き換えて確かめ、戻す）。

**完了条件**: `go test -tags test ./cmd/yt2column ./cmd/fixprguard` が通る。

#### ステップ 2-3: 用語集

**変更するファイル**: `docs/translation_glossary.md`、`cmd/yt2column/fixpr_docs_test.go`

-   [ ] 「信頼する著者」の定義を自動レビュアーの一覧を指す形に改め、「自動レビュアー」（automated reviewer）、「自動レビュアーの一覧」（automated reviewer list）、「待つ PR レビュアー」（awaited PR reviewer）の 3 行を加える。定義は要件書 6 節に合わせる。
-   [ ] `TestGlossaryTerms` に 3 行を加え、「信頼する著者」の定義のセルが「自動レビュアーの一覧」を含むことを確かめる。

**壊して確かめる**: 用語集の 3 行のどれかを消す、「信頼する著者」の定義を元に戻す、のそれぞれでテストが失敗すること。

**完了条件**: 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）が通る。用語集は `cmd/yt2column-server` と `internal/pipeline` のガードも表として読むので、表の形を保つ。

#### ステップ 2-4: ガイドの PR-2 の項目

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`、`fixprguard_guide.md`、`cmd/fixprguard/docs_test.go`

-   [ ] 日本語版のガイドの信頼する著者の説明と `--reviewer` の行に、渡す名前は `_context.md` の `Automated reviewer list` のすべての名前であることを加える。§9 に、自動レビュアーを一覧に加えるときと外すときの手順と、急いで信頼をやめるときの手順を加える（設計書 3.5 のガイドの PR-2 の項目、9 章の 1・2 つめの項目）。
-   [ ] 日本語版をコミットしてから、英語版を翻訳する。
-   [ ] ステップ 1-5 のガイドのテストに、`Automated reviewer list` への言及を加える（AC-12 の「一覧の名前を 1 つずつ `--reviewer` で渡す」）。

**壊して確かめる**: 両方のガイドのそれぞれで、`Automated reviewer list` への言及を消すとテストが失敗すること。

**完了条件**: `go test -tags test ./cmd/fixprguard` が通る。

#### ステップ 2-5: フェーズ 2 の確かめ

**変更するファイル**: なし

-   [ ] `make test && make lint` を実行する。
-   [ ] 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）を実行する。
-   [ ] インストール済みのツールで、PR-2 のブランチの `fixpr.md` と `runpr.md` の `--reviewer` の書き方が受け付けられることを、このブランチの PR の `/runpr` で確かめる（設計書 6.2 の表の右下の組み合わせ）。PR の本文に、マージの前に `make install-fixprguard` が要ることを書く（設計書 5.3 の「切り替えの間の worktree」）。

**完了条件**: 上の 3 つ。

### PR-2 作成ポイント: command documents pass the reviewer list

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5

**推奨タイトル**: `docs(0033): _context.md に自動レビュアーの一覧と待つ PR レビュアーを置き、コマンドの文書から一覧を渡す`

**レビュー観点**: `_context.md` の 2 行の形と、内容のガードがその形を厳密に読み、壊れた写しの表で失敗すること / `fixpr.md`・`runpr.md` が名前を書かずに一覧を参照し、実行の最初に 1 度だけ並びを作ると定めること / `runpr.md` step 3 が `<awaited-reviewer>[bot]` だけを変え、照合の行と依頼のコメントを変えないこと / 用語集とガイドの用語の一致

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 2-2 は、すべての `/fixpr`・`/runpr` のセッションが従う、信頼する Bot の渡し方の記述を置き換える高リスクのステップである。ステップ 2-1 のガードの関数は、一覧の書き換えを検出する守り（設計書 5.2）を担う。

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
| M3 | 利用者の `make install-fixprguard` | フェーズ 2 の前提の確かめ |
| M4 | `_context.md`、コマンドの文書、用語集、ガイドの PR-2 の項目、内容のガード | PR-2 のマージ |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 | `Reviewers`、信頼の判定、`--reviewer` の繰り返し、呼び出しのガード、ガイドの PR-1 の項目、`package_reference.md` | frontier-required |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 | `_context.md` の 2 行、`fixpr.md`・`runpr.md`、用語集、ガイドの PR-2 の項目、内容のガード | frontier-recommended |

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。PR-1 はコマンドの文書を変えないので、PR-1 自体の `/runpr`・`/fixpr` は今のツールで動く。PR-2 の内容のガードは PR-1 の `ParseReviewers` を使うので、PR-1 より先にはマージできない。フェーズ 1 の中では、型（1-1）→ 判定（1-2）→ フラグ（1-3）の順に置く。`internal/fixprguard` の変更を `cmd/fixprguard` より先に置くためである。ステップ 1-2 の後は `cmd/fixprguard` のビルドが通らないので、ステップ 1-2 と 1-3 は同じ PR の中で続けて行う。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

設計書 7.1 の項目を、ステップ 1-1〜1-3 に割り当てた。GitHub は `cmdrunner.Runner` の fake で置き換え、`gh` を呼ばない（設計書 7 章）。既存のテストのうち、`--reviewer` を 1 回だけ渡すものと、1 つの名前の `Reviewers` で判定するものは、変えずに通ることで AC-04 を確かめる。

### 4.2. 文書のガード

-   呼び出しのガード（ステップ 1-4）と、ガイドと `package_reference.md` のテスト（ステップ 1-5・2-4）は PR-1 から、内容のガード（ステップ 2-1〜2-3）は PR-2 で入る。どれも CI の `doc-guard` のジョブが実行する（AC-14）。
-   内容のガードの `_context.md` の検査は、壊れた写しの表で失敗を確かめる（1.4）。文書の文の検査は、ステップ 2-2 の「壊して確かめる」で一時的に書き換えて確かめる。

### 4.3. 結合テスト

-   ステップ 1-3 の `begin` → `reply` のテスト（設計書 7.4。1.4）。
-   PR-2 の `/runpr` で、インストール済みの新しいツールと PR-2 のブランチの文書の組み合わせが動くことを観察する（ステップ 2-5。`manual`）。

### 4.4. セキュリティテスト

設計書 7.4 のセキュリティテストの 3 つを、次のステップで確かめる。

-   判定の表の「一致しない `Bot`」「名前が一致する `User`」「大文字と小文字の違い」と `[bot]` 付きの行（ステップ 1-2）。
-   重複・文法違反で GitHub にも git にも触れないこと（ステップ 1-3）。
-   内容のガードが 3 つめの名前を加えた一覧で失敗すること（ステップ 2-1）。一覧とガードの期待値を同時に変えた場合は検出しない（設計書 5.3）。

### 4.5. 後方互換性

-   `--reviewer` を 1 回だけ渡す呼び出しは、今と同じ結果になる（AC-04）。4.1 の既存のテストで確かめる。
-   PR-1 のマージから PR-2 のマージまでの間、今の文書は新しいツールで今までどおり動く（設計書 6.2 の表）。PR-1 の `/runpr` は今のツールで動き、PR-1 のマージの後に `main` で別の作業の `/runpr` を実行すれば、今の文書と新しいツールの組み合わせを観察できる。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は振る舞いを確かめる Go のテスト、`static` は文書を読む Go のガードのテスト、`manual` は PR の上での観察を表す。新しいテストの名前は仮の名前で、実装で変えたときは本書の表を同じ PR で改める（1.3.5）。

| AC | 検証 | 場所 |
|---|---|---|
| AC-01 | static | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`（行の数、ちょうど 2 つの名前）、`::TestContextReviewerRowsMalformed`（壊れた写しの表。ステップ 2-1） |
| AC-02 | static | `cmd/yt2column/fixpr_docs_test.go::TestContextReviewerRowsMalformed`（空、文法違反、重複の行）、`::TestContextAndCommandsAgree` |
| AC-03 | static | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`、`::TestContextReviewerRowsMalformed`（待つ PR レビュアーの行） |
| AC-04 | test | `cmd/fixprguard/main_test.go::TestReviewerFlagRepeats`（違う名前の 2 回。ステップ 1-3）、`::TestReviewerOrderIrrelevant`（順序）、`::TestReviewerFlag` と `--reviewer` を 1 回だけ渡す既存のテスト、`internal/fixprguard/trust_test.go::TestAssessAuthors` の既存の行 |
| AC-05 | test | `internal/fixprguard/trust_test.go::TestAssessAuthors`（1 つめ・2 つめ・混ざったスレッド・大文字と小文字）、`internal/fixprguard/github_test.go::TestThreadPathsRequiresTrustedOrNamed`、`internal/fixprguard/write_test.go::TestReplyTrustsEveryReviewer`、`cmd/fixprguard/main_test.go::TestSecondReviewerBeginReply` |
| AC-06 | test | `internal/fixprguard/trust_test.go::TestAssessAuthors`（一致しない `Bot`、名前が一致する `User`）、`::TestAssessUnverifiableAuthor`（`bot_login_outside_grammar`） |
| AC-07 | test | `internal/fixprguard/types_test.go::TestParseReviewers`、`cmd/fixprguard/main_test.go::TestAcceptThreadFlagParsing`（4 つのサブコマンドの重複と文法違反の行。標準エラー）、`::TestReviewerFlag`（`--reviewer` がない場合） |
| AC-08 | test | `internal/fixprguard/github_test.go::TestBeginAuthorType` |
| AC-09 | test | `cmd/fixprguard/main_test.go::TestReviewerFlag` |
| AC-10 | static | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`（定義の文）、`cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`（ちょうど 1 つの `<reviewer>`） |
| AC-11 | static | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`（名前が依頼のコメントの外に現れない。`<awaited-reviewer>` の定義の文）、`::TestRunprDocContract`（step 3 の絞り込みと照合の行）、`::TestRemovedCommandLiterals`（`<reviewer>[bot]`） |
| AC-12 | static | `cmd/yt2column/fixpr_docs_test.go::TestFixprDocContract`（`fixpr.md`）、`cmd/fixprguard/docs_test.go::TestGuideDescribesReviewerList`（ガイドの両方の版と `package_reference.md`。ステップ 1-5・2-4） |
| AC-13 | static | `cmd/yt2column/fixpr_docs_test.go::TestGlossaryTerms` |
| AC-14 | static + manual | `make test`・`make lint`、文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）。PR-1 と PR-2 の CI の `doc-guard` のジョブの結果 |

AC-01〜AC-03・AC-10〜AC-13 は、`_context.md`・コマンドの文書・ガイド・用語集が何を述べるかを定めるので、`static` で確かめる。

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6）
- [ ] 利用者が `main` で `make install-fixprguard` を実行した（フェーズ 2 の前提）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5）

## 7. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| PR-1 のマージの後、再インストールを忘れて PR-2 に進む | フェーズ 2 の前提の確かめで止まる。忘れても、PR-2 の `/runpr` が使い方のエラーで止まるだけで、誤った判定はしない（設計書 6.2） |
| `--protocol` を上げないので、古いツールが番号の食い違いで止まらない | 古いツールは `flag given twice: --reviewer` で止まる（1.3.4）。ガイドの終了コード `2` の説明が再インストールを案内する（ステップ 1-5） |
| 判定の 4 か所と `processThread` のどれかが 1 つの名前の比べ方のまま残る | `GitHub.reviewer` を消すのでビルドで残りが見つかる（1.3.2）。各箇所の 2 つめの名前のテストを、壊して確かめる（ステップ 1-2） |
| 修正のフェーズが一覧と内容のガードの期待値を同時に書き換える | 設計書 5.3 のとおり受け入れ、マージの前の人のレビューで見つける |
| 開いているブランチの文書が PR-2 の前のまま、Codex のスレッドを想定外の著者のスレッドとして報告する | 安全側の振る舞いである。ブランチに `main` を取り込めば解消する（設計書 5.3）。PR-2 の本文に書く（ステップ 2-5） |

## 8. 成功基準 (Success Criteria)

-   [ ] 5 章のすべての `test` と `static` の検証が通る。
-   [ ] `make test && make lint` と、PR-1・PR-2 の CI の `doc-guard` のジョブが通る。
-   [ ] 各ステップの「壊して確かめる」の結果が、コミットメッセージに書かれている。
-   [ ] ガイドの日本語版と英語版の記述が一致する。

## 9. 次のステップ (Next Steps)

-   承認の後、`/runplan 0033` で PR-1 から実装する。PR-1 のマージの後は、利用者が `make install-fixprguard` を実行してから PR-2 に進む。
-   PR-2 のマージで、タスク 0031（#403）が `Awaited PR reviewer` の値と `runpr.md` step 3 を Codex 用に変えられるようになる（設計書 9 章）。
-   PR-2 は本タスクの最後のプランの PR なので、マージの後に issue #405 を閉じる。
