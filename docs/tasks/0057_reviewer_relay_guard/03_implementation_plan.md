# 実装計画書：外部の人が自動レビュアーを介して /fixpr のエージェントに指示を届ける経路を塞ぐ

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-03 |
| Review date | 2026-11-03 |
| Reviewer | isseis |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番を、「設計書 3.4」などは設計書の節を指す。用語は要件書 6 章と設計書の冒頭の定義（名指しのスレッド、ガードツール、制限の状態、設定の時刻、`--include` の出力、GitHub の時刻）に従う。「ガイド」は `docs/dev/guides/fixprguard_guide.md` を指す。

既存のコードについての記述は、コミット `f0cb3668` で確かめた。設計書が行番号を引いたコミット `5f9cdeb` から `f0cb3668` までの間に、`internal/fixprguard`・`cmd/fixprguard`・`cmd/yt2column`・`docs/dev/guides/`・`docs/translation_glossary.md`・`.claude/commands/_context.md`・0030 と 0036 のタスクの文書は変わっていない（`git diff --stat 5f9cdeb f0cb3668 -- <これらのパス>` の出力が空）。変わったのは `.claude/commands/runpr.md`（220 行目より後ろが 2 行ずれた）と `.claude/commands/fixpr.md` である。本書は、`runpr.md` の行番号を `f0cb3668` の値で示す。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

書き込み権限のない第三者が、自動レビュアーを介して `/fixpr` のエージェントに指示を届け、`make` の実行に至る経路を狭める。具体的には、自動レビュアーの一覧を `chatgpt-codex-connector` だけにし（F-002）、範囲の規則で実行関連ファイルを常に範囲外にし（F-001）、ガードツールに `interaction-limits` を加えて `/runpr` が interaction limits の状態を確かめて止まれるようにし（F-004）、`protocol` を `5` に上げ（F-005）、文書を合わせる（F-006）。F-003 の GitHub の設定は、設計の間に利用者が済ませている（設計書 1.3）。設計は設計書 1〜6 章のとおりである。

### 1.2. 実装の原則

-   設計書 8 章のフェーズ 1〜4 の順に実装する。フェーズの名前と順序は設計書と同じにする。
-   型・関数・受け入れる形・分類の順序・`/runpr` の振る舞いは、設計書 3〜4 章で決まっている。本書は、どのファイルを変え、どのテストで確かめ、どの順に作業するかを定める。設計の内容は繰り返さず、節を参照する。
-   加えたテスト（表のテストの行を含む）は、どれも対象を壊して失敗することを確かめ、確かめたことをコミットメッセージに書く（`CLAUDE.md`「Every test must be able to fail for its stated reason」）。壊すものは、設計書 7.2 の表と、各ステップの「壊して確かめる」の項目に挙げる。
-   Go のコメント・識別子・文字列リテラル（ガードツールの出力の値 `unverifiable`・`http-status` などを含む）は英語で書く（`CLAUDE.md`「Go Idioms」）。文書のガードのテストの日本語のリテラルは、ガイドの見出しなど、ガードのリテラルの例外に当たるものに限る。
-   ガードツールの出力にも `/runpr` の報告にも、応答の本文、ヘッダの値、`gh` の引数と標準エラーを入れない（設計書 3.3、AC-16）。

### 1.3. 計画の際に決めたこと（2026-11-03）

-   **PR は 3 つに分ける（人間のレビュアーが決めた）:** PR-1 はフェーズ 1（自動レビュアーの一覧）、PR-2 はフェーズ 2・3（ガードツールと、コマンドの文書・ガイドの規範の部分）、PR-3 はフェーズ 4（残りの文書）である。理由: フェーズ 2 と 3 は、`TestDocumentedProtocolMatches` と `TestDocumentedInvocationsParse` が文書の `--protocol` とサブコマンドをバイナリと照合するので、同じ PR に入れる必要がある（設計書 8 章の制約）。フェーズ 1 はガードツールを変えず、入口の守りのうち最も早く効くので単独で先に入れる。フェーズ 4 を PR-2 に含めると、すでに最大の PR がさらに大きくなる。PR-3 の `/runpr` は、新しい `runpr.md` で interaction limits の状態を読むので、AC-10 の「最後の PR の `/runpr` が『有効』と報告する」を確かめられる。`internal/fixprguard/limits.go` だけを先に別の PR にする案は、設計書 8 章の「フェーズ 2 と 3 は同じ PR」の制約に反し、設計書の修正が先に要るので採らない。
-   **ガイド 7.2 の新しい残余のリスクは、新しい小節 7.2.6 に置く（人間のレビュアーが決めた。設計書 3.8 が計画に委ねた）:** 7.2.6 は、GitHub の設定と自動レビュアーを介した経路についての残余のリスクをまとめる。7.2.3 の既存の 3 項目（同じパッケージの `.go` のファイル、Unicode の畳み込みによる別名、確認への慣れ）は、その場で、第三者が Codex を介してその状況に至りうることも書き加える。理由: 新しい項目の多くは、ツールの判定の不足ではなく、GitHub の設定とコマンドの手順に関わる。7.2.1〜7.2.5 の見出しと番号を変えないので、既存の参照（`TestGuideDescribesMentionCheck` の `### 7.1`・`### 7.2` など）が変わらない。
-   **AC-08 は、PR のレビューで読んで確かめる（`manual`）:** ガイドの `--reviewer` の例の名前を `_context.md` の一覧と照合するガードは加えない。設計書 3.2・7.3 のとおりである。例の名前が一覧とずれても判定は変わらず（ツールは `_context.md` を読まない）、害が小さいので、`CLAUDE.md`「Do not add a document check whose mistake is unlikely, whose harm is small」に当たる。
-   **AC-12 の呼ぶ位置を、`TestRunprDocContract` の既存の部分テスト `head_before_checkout` を広げて確かめる:** `runpr.md` のステップ 1 で、`fixprguard interaction-limits` が `fixprguard head` の後、`gh pr checkout` の前にあることを確かめる。設計書 7.3 は `TestDocumentedInvocationsParse` だけを挙げるが、`fixprguard` の呼び出しの位置はエージェントの信頼の境界の手順であり、`CLAUDE.md` が文書のテストの例外として残すもの（the `fixprguard` call sites）に当たる。今ある部分テストの比較に 1 つの位置を加えるだけである。
-   **AC-22 の「状態は `approved` のまま」は、`_checks/` のスクリプトで確かめる（`diff`）:** 0030・0036 の 4 つの文書について、merge-base と PR の head の間で、`## Document Status` の表と（あれば）`## 変更履歴` の節がバイト単位で同じで、差分が引用ブロックの注記の行の追加だけであることを確かめる。前後の関係を問う AC なので、恒久のテストにしない（`requirements_process.md` §4）。注記の文面は PR のレビューで読む（`manual`）。
-   **AC-10・AC-11 は、PR での観察を本書に記録して確かめる（`manual`）:** どちらも GitHub の設定の効き目であり、`make test` と CI で動くテストにできない（ユニットテストは GitHub に接続しない。`CLAUDE.md`「Testing Strategy」）。PR の `/runpr` の間に本書へ書き込むと head が変わるので、観察は次の PR のブランチの最初のステップで記録する。本書の PR とその `/runpr` の観察（AC-11）は、PR-1 のブランチの最初のステップ（ステップ 1-0）で記録する。PR-1・PR-2 の観察と、PR-2 の `/runpr` が新しい `runpr.md` で読んだ状態（AC-10 の事前の確認）は、ステップ 4-0 で記録する。本タスクの最後の PR（PR-3）の `/runpr` が「有効」と報告することは、その実行の報告で観察する（本書には記録できない。マージの後に記録する PR を作らない）。AC-10 の「有効」の分類そのものは、`limits_test.go` の `active` の行が確かめる。
-   **フェーズ 2・3 の PR（PR-2）のマージの後に `main` を取り込むブランチは、マージの時点で列挙する:** 計画の時点では、どのブランチが作業中かは決まらない。PR-3 のブランチの最初のステップ（ステップ 4-0）で、`git worktree list` で作業中のブランチを列挙し、それぞれに `main` を取り込み、取り込んだブランチをそのステップに書く（設計書 3.6・8 章）。PR の作成ポイントのチェックの項目は、`mkplan2.md` の形の 4 つのままにする（`runplan.md`・`runpr.md` がその順で印を付けるため）。
-   **`gh pr view` の 1 つの文字列の項目の読み取りを共通にする:** `prBody`（`write.go:431-455`）と `createdAt` の読み取りは、どちらも `gh pr view <PR> --json <1 つの項目>` を `ParseObject`・`Collect` で読み、1 つの必須の文字列を取り出す同じ形である。その部分を 1 つの関数にし、`prBody` もそれを使う。`prBody` の振る舞い（空の文字列を受け入れる、失敗は `ErrResponseRejected`）は変えない。RFC 3339 の検査は `createdAt` の側だけで行う。

### 1.4. 既存コード調査結果

**範囲の規則（`internal/fixprguard/scope.go`）**

| 項目 | 今あるもの | 変えること |
|---|---|---|
| 規則 4 の分岐 | `inScopeOf`（`scope.go:231-254`）の最初の分岐 `if isExecutionRelated(changed) { return !named && changed == p }`（`:232-234`） | 実行関連ファイルなら常に `false` を返す形にする（設計書 3.1）。`inScopeOf` の引数の形は変えない（H-04） |
| コメント | 冒頭のコメント（`:23-26` の「The deny list ... applies everywhere: an execution-related file is in scope only when it is p and the thread is not a named untrusted one, whose author chose p.」）と `inScopeOf` のコメント（`:227-230`） | 実行関連ファイルがスレッドの著者と `path` にかかわらず範囲外であることに改める。`named` を使うのは `.go` の規則だけであることを `inScopeOf` のコメントに書く |
| テスト | `scope_test.go:60` の `makefile_is_path`（範囲内を期待）、`:23-36` の `denyListAtPathItself`（信頼する著者の側で範囲内を期待）と、その呼び出し 5 か所（`:182-214`） | 期待を改める（ステップ 2-1） |

-   `worktree_test.go`・`worktree_flow_test.go`・`cmd/fixprguard/main_test.go` を検索した（設計書 3.1 が計画に委ねた確認）。スレッドの `path` を実行関連ファイルにして範囲内を期待するテストはない。`threadAt` の呼び出し（`test_helpers_git.go:311`）は、`testFile`（`docs/notes/a.md`。`test_helpers_git.go:21`）か `internal/x/x.go`（`worktree_flow_test.go:110`）だけを渡す。`worktree_flow_test.go:689-703` の `replaced_base_commit` は `Makefile` を範囲外と期待しており、変わらない。
-   AC-04 の「範囲外の変更を、利用者が続けると答えた後に `finish` が commit する」は、既存の `worktree_flow_test.go::TestFinishAfterOutOfScope`（`:524-539`）が確かめている。`finish` は範囲を見ないので、変えない（設計書 3.1）。

**`--include` の出力の分け方（`internal/fixprguard/trust.go`）**

-   `splitIncluded`（`trust.go:197-223`）は `(status int, body []byte, ok bool)` を返し、HTTP の状態コードの 3 文字を `strconv.Atoi` で読む（`:207`）。呼び出し元は `queryPermission`（`:148`）と `failureCause`（`:181`）の 2 か所だけである（`git grep -n splitIncluded` の結果。テストを除く）。戻り値にヘッダの行を加え、2 か所を機械的に合わせる（ステップ 2-2）。

**GitHub の呼び出し（`internal/fixprguard/github.go`）**

| 項目 | 今あるもの | 本書での使い方 |
|---|---|---|
| `gh` の起動 | `runGH`（`github.go:67-75`）。呼び出しごとに `githubTimeout`（`:19`、2 分）の上限 | `InteractionLimits` は、全体の上限の context を作ってから 3 つの呼び出しに渡す（設計書 3.3）。`runGH` の上限は内側で働く |
| 失敗の写し | `ghFailure`（`:80-85`）は、元のエラーを捨て、時間切れかどうかを文面にだけ残す | `timeout` の判定は、文面ではなく、`InteractionLimits` が作った全体の context の `Err()` を `errors.Is(..., context.DeadlineExceeded)` で調べて決める（設計書 4 章） |
| リポジトリの名前 | `resolveRepository`（`:96-122`） | そのまま使う。失敗は `repository`、時間切れは `timeout` |
| PR の `gh pr view` | `Head`（`:146-180`）と `prBody`（`write.go:431-455`）が、`gh pr view <PR> --json <fields>` を `strictjson.ParseObject`・`Collect` で読む | `prBody` の 1 つの項目の読み取りを共通の関数にし、`createdAt` の読み取りにも使う（1.3） |
| スレッドを判定しない `GitHub` | `NewGitHubWithoutReviewer`（`:52-57`）と、そのコメントの一覧（`:47-51`） | `InteractionLimits` をコメントの一覧に加える |

-   `resolveRepository` はエラーだけを返す。`InteractionLimits` は、呼び出した側で、その失敗を `repository`（時間切れなら `timeout`）に写す。`gh` の呼び出しの失敗の写し方（`timeout`・`http-status`・`gh-failed`）も、`InteractionLimits` が全体の context と `cmdrunner.ExitError` から決める。
-   `github_test.go:407-423` の `deadlineRunner` は、期限の時刻ではなく残り時間（`time.Until(deadline)`）を記録する。偽物はすぐに返るので、呼び出しごとの上限でも共有の上限でも残り時間はほぼ 2 分になり、3 つの呼び出しが 1 つの期限を共有することを確かめられない。そのため、`ctx.Deadline()` の時刻も記録するように広げ（`TestGitHubCallsHaveTimeout` の確かめは変えない）、3 つの時刻が等しいことを確かめる。
-   `cmdrunner/testutil` の `Fake`（`internal/cmdrunner/testutil/mocks.go:61`）は context を見ず、あらかじめ設定した結果をそのまま返す。そのため、時間切れの行は、期限を過ぎた context を渡したうえで、その段の `Step.Err` にエラーを設定して返させる（実際の起動と同じく、期限切れの後の呼び出しは失敗する）。
-   `cmd/fixprguard/main_test.go` の `validArgs`（`:26-46`）は `TestProtocolMismatch`（`:48-`）と `TestReviewerFlag`（`:72-`）の表の元で、`TestGhTargetEnvRejected`（`:216-`）は `usesGH` のサブコマンドのすべてに行があることを `specs` と照合する（`:233-237`）。新しいサブコマンドは、これらの表に加える。`cmd/yt2column/fixpr_docs_test.go::TestFixprguardCallSites`（`:647-`）は、コマンドの文書の各呼び出しの節を登録している。

**JSON の検査（`internal/strictjson`）**

-   `ParseObject`・`Object.Collect`・`RequiredString` で、要件書 4.2 の拒否の規則をすべて満たせる（設計書 3.4 の表）。新しい関数は要らない。

**列挙型の形**

-   既存の `ChangeKind`・`Reason`・`Cause` は、ゼロ値を名前のない値にし、`MarshalText` がエラーを返すようにする（`changes.go:40-64`、`types.go:211-241`）。`InteractionState` だけは、ゼロ値を `InteractionUnverifiable` にする（設計書 3.3）。`StopKinds()`（`errors.go`）と `TestGuideCoversStopKindsAndScope`（`cmd/fixprguard/docs_test.go:52-75`）の組が、`InteractionStates()`・`UnverifiableReasons()` とガイドの照合の手本になる。
-   設計書は、具体的な型ごとに例外的な振る舞いを持たせていない。`InteractionLimitsInfo` は出力のための値で、`cmd/fixprguard` とテストが組み立てるので、フィールドは公開のままにする。構築の経路をリーフパッケージに閉じ込める必要はない。

**サブコマンドの登録（`cmd/fixprguard/main.go`）**

-   サブコマンドの名前の定数（`main.go:57-68`）、`specs`（`:105-121`）、`execute` の `switch`（`:349-384`）に加える。`head`（`:427-441`）が、`validatePR` と `NewGitHubWithoutReviewer` を使う手本である。`interaction-limits` は PR 番号を省略できないので、`spec.arg` は `argRequiredPR`（`:88-93`）にする。`usesGH: true` で `GH_REPO`・`GH_HOST` の検査（`:350-356`）が当たる。
-   `--protocol` の照合は `parse`（`:213-`、照合は `:238-240`）と `TestProtocolMismatch`（`main_test.go:48-`）が確かめている。拒否する値は `Protocol + 1`・`"0" + Protocol`・空の値（`:55`）で、1 つ前の番号はない。AC-19 の後半（新しいツールが `--protocol 4` を拒否する）のため、`Protocol - 1` をこの値に加える。

**`protocol` の番号**

-   `internal/fixprguard/types.go:21` の `Protocol = 4`。`--protocol 4` は、`fixpr.md`・`runpr.md`・ガイドに書かれている（ガイドでは例の呼び出し（`:89`・`:99`・`:120`・`:143`・`:153`・`:171-172`・`:184`・`:196`・`:206`・`:220`）、出力の `"protocol":4`（`:92`・`:187`・`:199`・`:209`・`:223`）、4 章の「現在の番号は `4`」（`:64`））。`TestDocumentedProtocolMatches`（`cmd/fixprguard/docs_test.go:322-345`）が、`.claude/commands` のすべてのコマンドの文書の `--protocol` と、ガイドの例・出力・「現在の番号」を `Protocol` と照合する。`git grep -n -- '--protocol 4'` と `git grep -n '"protocol":4'` で漏れを探せる。

**自動レビュアーの一覧と、そのガード**

-   `coderabbitai` は、`docs/tasks/` とテストを除くと、`_context.md:185` と、ガイドの `:65`（4 章の例）・`:99`・`:120`・`:153`・`:196`（5 章の例）だけにある（`git grep -n coderabbitai -- ':!docs/tasks' ':!*_test.go' ':!*test_helpers*'` の結果）。
-   `cmd/yt2column/fixpr_docs_test.go:58` の `wantReviewers` を、`TestContextAndCommandsAgree`（`:925-`）が `checkList`（`:867-`）で `_context.md` の行と照合する。`TestContextReviewerRowsMalformed`（`:956-1020`）の行ごとの影響は設計書 3.2 のとおりである。
-   `cmd/fixprguard/test_helpers.go` と `internal/fixprguard/test_helpers.go:19` の `testReviewer = "coderabbitai"` は変えない（設計書 2.2）。
-   AC-06 は、一覧の値と、一覧にない Bot を `other-bot` とする既存のテスト `internal/fixprguard/trust_test.go::TestAssessAuthors` の `second_reviewer_not_passed` の行（`:46-53`）と、想定外の著者のスレッドの本文を取得しない `internal/fixprguard/github_test.go::TestBeginNeverFetchesUntrustedBody`（`:199-`）で確かめる。AC-07 は `github_test.go::TestBeginAcceptsNamedThread`（`:311-`）と、ステップ 2-1 の名指しの行で確かめる。

**`runpr.md` と、そのガード**

| 項目 | 今あるもの（`f0cb3668`） | 変えること |
|---|---|---|
| `--protocol` と `--reviewer` の説明 | `runpr.md:13-31` | `--protocol 5`。`interaction-limits` は `--reviewer` を取らないことを、`--reviewer` を取らない呼び出しの一覧に加える |
| 「Authorization」 | `:62-89`。`fixprguard` が `head`・`list`・`reply` で行う操作の一覧は `:68-72` | `interaction-limits` の `gh repo view`・`gh pr view`・`gh api --include` を加える（設計書 3.5） |
| ステップ 1 の `head` | `:169-180`（呼び出しは `:172`） | その直後、`gh pr checkout`（`:181-`）の前に `interaction-limits` の呼び出しと結果の扱いを置く |
| メンションの一覧 | 会話のコメントの `--jq`（`:282-305`）の `mention` の種類（`:301-302`）、行コメントと PR レビューの `--jq`（`:331-338`） | 書いた人が利用者かどうか、Bot かどうかの真偽値を加え、止まる条件を書く（設計書 3.5） |
| Codex の断りのコメントの例外 | `:383-388` | 変えない |
| 想定外の著者のスレッド | `:523-540` | 変えない |
| ガード | `TestRunprDocContract`（`cmd/yt2column/fixpr_docs_test.go:153-187`）の `head_before_checkout`（`:157-164`）と `listings_print_no_body`（`:181-186`）、`runprListingFixtures`（`:272-`）、`checkListingOutput`（`:591-`）。`TestDocumentedInvocationsParse`（`cmd/fixprguard/docs_test.go:184-210`）の `runpr.md` の期待（`:190`） | ステップ 3-2 のとおり |

**ガイドを読むガード（`cmd/fixprguard/docs_test.go`）**

-   `section(guide, "## 6.")`・`subsection(guide, "### 7.1")` などで節を切り出す。新しい 10 章と 7.2.6 は末尾か小節の最後に加え、既存の見出しを変えないので、これらの切り出しは変わらない。`TestGuideCoversStopKindsAndScope` は、6 章の表を行頭の `**実行関連ファイル。**` で探す（`docs_test.go:60`）。ステップ 3-4 でこの段落を改めるときも、この印はそのまま残す。
-   `` `body` `` はガイドにすでに 2 回現れる。状態と理由の値のガードは、ガイド全体ではなく、5 章の `interaction-limits` の節の表を `subsection` と `guideTable` で切り出して照合する。
-   `TestGuideIgnoredCommands`（`:512-`）は 8.1 の最初の `sh` のブロックを実行する。本タスクは 8.1 を変えない。

**`implementation_handoff.md` と `design_handoff.md`**

-   `implementation_handoff.md` は、このタスクにはない。`design_handoff.md` の H-01〜H-07 は、すべて設計書の付録 A に記録されている。

## 2. 実装手順 (Implementation Steps)

テストの関数名は予定の名前で、実装で変えたときは本書 7 章を同じ PR で直す。各ステップの「壊して確かめる」は、そのステップで加えるテストが失敗することを確かめるために壊すものである（1.2）。

### フェーズ 1: 自動レビュアーの一覧（設計書 8 章の手順 1）

#### ステップ 1-0: 本書の PR の観察の記録

**変更するファイル:** `docs/tasks/0057_reviewer_relay_guard/03_implementation_plan.md`

-   [x] 本書の PR とその `/runpr` で観察したこと（Codex の要約のコメントの完了の状態、利用者の `git push` による PR のブランチの更新、`/mergepr` の squash merge とブランチの削除、`/runpr` ステップ 5 の `gh issue develop` による PR-1 のブランチの作成（PR-1 のブランチで作業している時点で済んでいる））を、7 章の AC-11 の行に記録する（1.3）。どれかが ruleset で止められていたら、要件書 1 章の決定に従い作業を止め、利用者に報告する。

**成功の条件:** AC-11 の行に、4 つの観察の結果と PR の番号がある。

#### ステップ 1-1: 一覧の値とそのガード

**変更するファイル:** `.claude/commands/_context.md`、`cmd/yt2column/fixpr_docs_test.go`

-   [x] `_context.md` の「Automated reviewer list」（`:185`）の値を `` `chatgpt-codex-connector` `` だけにする。説明の文は変えない（設計書 3.2）。
-   [x] 同じコミットで、`wantReviewers`（`fixpr_docs_test.go:58`）を `chatgpt-codex-connector` だけにする。
-   [x] 同じコミットで、`TestContextReviewerRowsMalformed` を設計書 3.2 の行ごとの影響のとおりに改める。受け入れる行（`:975`）を 1 つの名前の行にし、テストのコメントの「2 つの名前」の記述を改める。ほかの行の期待は変えない。
-   [x] 壊して確かめる: `_context.md` を `coderabbitai` を含む元の行に戻すと `TestContextAndCommandsAgree` が失敗する。

**成功の条件:** 文書のガード（下記）が通る。

#### ステップ 1-2: ガイドの例

**変更するファイル:** `docs/dev/guides/fixprguard_guide.md`

-   [x] 4 章の `--reviewer LOGIN` の行の `（例: coderabbitai）`（`:65`）を `（例: chatgpt-codex-connector）` にする。
-   [x] 5 章の例の `--reviewer coderabbitai`（`:99`・`:120`・`:153`・`:196`）を `--reviewer chatgpt-codex-connector` にする。
-   [x] ガイドのほかの箇所に、`coderabbitai` を信頼する自動レビュアーとして挙げる記述が残っていないことを読んで確かめる（AC-08。`manual`）。

**成功の条件:** `git grep -n coderabbitai -- docs/dev/guides` の出力が空である。

**フェーズ 1 の完了の条件:** `make test`・`make lint` と、文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview ./internal/pipeline`。CI の `doc-guard` と同じ形）が通る。

### PR-1 作成ポイント: automated reviewer list

**対象ステップ**: 1-0 / 1-1 / 1-2

**推奨タイトル**: `feat(0057): 自動レビュアーの一覧から CodeRabbit を外す`

**レビュー観点**: `_context.md` の一覧と `wantReviewers`・`TestContextReviewerRowsMalformed` が同じコミットで変わっていること / 受け入れる行が 1 つの名前になっても、形の誤りの行が区切りの解析を引き続き確かめていること / ガイドの例が一覧の名前を使うこと

**実装モデル要件**: standard

**判定理由**: パネルモードの条件にも条件付きの確認の 2 つ以上にも当たらず、既存コード調査結果に競合する案もない。値と文書のガードの機械的な変更である

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: ガードツール（設計書 8 章の手順 2）

#### ステップ 2-1: 範囲の規則

**変更するファイル:** `internal/fixprguard/scope.go`、`internal/fixprguard/scope_test.go`、`internal/fixprguard/worktree_flow_test.go`

-   [ ] 規則を変える前のコミットで、AC-02 の 6 行（名指しのスレッドの `path` そのもの）と AC-03 の 5 行を `TestScopeRules` に加え、今のコードで通ることを確かめる（「今と同じ」をコードで示す）。既存の行や `denyListAtPathItself` の組と入力が同じものは、重ねて書かない。
-   [ ] 次のコミットで、`inScopeOf` の規則 4 の分岐を、実行関連ファイルなら常に範囲外にする形にする（設計書 3.1）。冒頭のコメントと `inScopeOf` のコメントを 1.4 の表のとおりに改める。
-   [ ] 同じコミットで、`makefile_is_path`（`scope_test.go:60`）の期待を範囲外にし、`denyListAtPathItself` の信頼する著者の側の期待を範囲外にしてコメントを改める。拒否リストだけを確かめていることは、拒否リストにないパスの `path` そのものの行（`path_itself` など。範囲内）と対にして示す（設計書 7.1）。AC-01 の 6 行（信頼する著者のスレッドの `path` そのもの）を加える。
-   [ ] `TestFinishAfterOutOfScope` の形で、信頼する著者のスレッドの `path` が `Makefile` で、`Makefile` の変更が `OutOfScope` に入り、利用者が続けると答えた後に `finish` がそれを commit する行を加える（AC-04、設計書 6.2）。
-   [ ] 壊して確かめる: 規則 4 の分岐を元の形に戻すと、AC-01 の信頼する著者の行と、上の `finish` の行（`OutOfScope` が空になる）が失敗する。

**成功の条件:** `go test -tags test ./internal/fixprguard/...` が通る。

#### ステップ 2-2: `--include` の出力の分け方

**変更するファイル:** `internal/fixprguard/trust.go`、`internal/fixprguard/trust_test.go`

-   [ ] `splitIncluded` を、HTTP の状態コードの行と空の行の間の行（末尾の `\r` を 1 つ除いたもの）も返す形に広げる（設計書 3.4）。HTTP の状態コードを数字 3 桁だけに限る。
-   [ ] 呼び出し元の `queryPermission` と `failureCause` を、戻り値の数に合わせる。振る舞いは変えない。
-   [ ] テストを加える。ヘッダの行を返すこと、末尾の `\r` を除くこと、符号付きの HTTP の状態コード（`+20`・`-20`）を拒否すること。権限の照会の既存のテスト（`TestPermissionCause` など）が期待を変えずに通ることを確かめる。
-   [ ] 壊して確かめる: 数字の検査を外すと符号付きの行が、ヘッダの行の `\r` を除く処理を外すとヘッダの行が失敗する。

**成功の条件:** `go test -tags test ./internal/fixprguard/...` が通る。

#### ステップ 2-3: 制限の状態の読み取りと分類

**変更するファイル:** `internal/fixprguard/limits.go`（新規）、`internal/fixprguard/limits_test.go`（新規）、`internal/fixprguard/github.go`、`internal/fixprguard/write.go`、`internal/fixprguard/github_test.go`、`internal/fixprguard/test_helpers.go`

-   [ ] 設計書 3.3 の型と関数（`InteractionState` とその `MarshalText`、`InteractionStates`、`UnverifiableReason` とその値、`UnverifiableReasons`、`InteractionLimitsInfo`、`GitHub.InteractionLimits`）を書く。検査と分類は設計書 3.4 のとおりである。全体の上限は 1 つの context で 3 つの呼び出しに当てる。
-   [ ] `prBody` の 1 つの文字列の項目の読み取りを共通の関数にし、`prBody` と `createdAt` の読み取りの両方で使う（1.3）。
-   [ ] 失敗の写し方を 1.4 のとおりにする。`reason` は、段ごとの型の値として決め、エラーの文面から求めない（設計書 4 章）。時間切れは全体の context で決める。どの段の失敗でも、結果を `unverifiable` にし、部分的な結果を捨てる。
-   [ ] `NewGitHubWithoutReviewer` のコメント（`github.go:47-51`）の、スレッドを判定しない呼び出しの一覧に `InteractionLimits` を加える。
-   [ ] `--include` の出力を組み立てる補助（HTTP の状態コード、ヘッダ、本文から設計書 1.3 の形のバイト列を作る）を `internal/fixprguard/test_helpers.go`（`//go:build test`、英語）に置く。
-   [ ] テストを書く。項目は設計書 7.1 の「検査と分類」のとおりで、次を加える。
    -   分類の順序: 2 つの状態に当たる入力（期限切れかつ `weak` → `none`、`weak` かつ設定より前の PR → `weak`、設定より前の PR かつ 14 日以内 → `predates`、期限切れかつ設定より前の PR → `none`）。
    -   境界: ちょうど 64 KiB の本文（受け入れる）、月末の日がない月への見積もり（`expires_at` が 2027-08-30T00:00:00Z の場合。8 月 30 日は月末ではないが、6 か月前の 2 月には 30 日がない）、ANSI C の形の `Date`、名前が小文字の `date:` のヘッダ（受け入れる）、`cmdrunner.ErrOutputTooLarge`（`output-form`）。
    -   時間切れ: `repository`・`pr`・`api` の 3 つの段のそれぞれで、期限を過ぎた context とその段の `Step.Err` を与えて `timeout` になる行（1.4）。全体の context が期限切れになっていないときに、`context.DeadlineExceeded` を包んだエラーを段が返す行は `timeout` にならない。
    -   上限の共有: `deadlineRunner` を、`ctx.Deadline()` の時刻も記録するように広げ（`github_test.go`）、3 つの呼び出しの期限の時刻が等しく、始めてから `githubTimeout` 以内であることを確かめる。
    -   どの行でも、`State`・`ExpiresOn`・`Reason`・`HTTPStatus` の 4 つの値を確かめる（`none` で `ExpiresOn` が空であることを含む）。拒否の行の `Reason` が `UnverifiableReasons()` にあること、表に現れる状態と理由の集合が `InteractionStates()`・`UnverifiableReasons()` と等しいこと、一覧の外の `InteractionState` の `MarshalText` が誤りを返すこと、ゼロ値の `InteractionLimitsInfo` を JSON にすると `state` が `unverifiable` になることも確かめる。
-   [ ] 壊して確かめる: 設計書 7.2 の表の、このステップに当たる 11 行（空の本文の照合から `InteractionState` のゼロ値まで）。加えて、全体の context をやめて呼び出しごとの上限だけにする（上限の共有の行）、時間切れを全体の context でなくエラーで決める（期限切れでない context の行）、分類の 2 つの順を入れ替える（順序の行）、ヘッダの名前を大文字小文字を区別して照合する（小文字の `date:` の行）。

**成功の条件:** `go test -tags test ./internal/fixprguard/...` が通る。

#### ステップ 2-4: サブコマンドと `protocol` の番号

**変更するファイル:** `cmd/fixprguard/main.go`、`cmd/fixprguard/main_test.go`、`cmd/fixprguard/test_helpers.go`、`internal/fixprguard/types.go`

-   [ ] `interaction-limits` を、サブコマンドの名前の定数（`cmdInteractionLimits`）、`specs`（`flags` なし、`arg: argRequiredPR`、`usesGH: true`）、`execute` の `switch` に加える。処理は `head` と同じく `NewGitHubWithoutReviewer` で `GitHub` を作り、`InteractionLimits` の結果の `Protocol` を `fixprguard.Protocol` にそろえて出力し、終了コード `0` を返す（設計書 3.3）。
-   [ ] `Protocol` を `4` から `5` にする（`types.go:21`）。
-   [ ] `cmdInteractionLimits` を、`validArgs`、`TestGhTargetEnvRejected` の `lines` に加える（`--protocol` の不一致、`--reviewer` の拒否、`GH_REPO`・`GH_HOST` の拒否は、これらの表が確かめる）。`TestProtocolMismatch` の拒否する値に `Protocol - 1` を加える（AC-19）。
-   [ ] 新しいテストを書く。6 つの状態の出力と終了コード `0`、PR 番号がないときの終了コード `2`、確かめられないときに標準出力が目印の文字列を含まず、標準エラーが空であること（目印は、本文、ヘッダの値、`ExitError` の標準出力と標準エラーに入れる。AC-16）。`--include` の出力の補助は `cmd/fixprguard/test_helpers.go` に置く（既存の `roleStep` と同じく、パッケージごとに持つ）。
-   [ ] 壊して確かめる: `Protocol` のそろえを外す（出力の `protocol` が違う行）、`InteractionLimits` の失敗のエラーを標準エラーに書く（標準エラーが空の行）、`specs` の `interaction-limits` の `flags` に `flagReviewer` を加える（`TestReviewerFlag` の `--reviewer` の拒否の行）。

**成功の条件（フェーズ 2 の完了の条件）:** このフェーズだけでは、文書が `--protocol 4` のままなので、`TestDocumentedProtocolMatches` と `TestDocumentedInvocationsParse` の 2 つだけが失敗する。`make fmt`、`make lint`、`go test -tags test ./... -skip '^(TestDocumentedProtocolMatches|TestDocumentedInvocationsParse)$'` が通ることを確かめ、フェーズ 3 に進む。`/runplan` は、フェーズ 2 と 3 を 1 つの実装のまとまりとして進める（PR-2）。

### フェーズ 3: コマンドの文書とガイドの規範の部分（設計書 8 章の手順 3）

#### ステップ 3-1: `--protocol 5`

**変更するファイル:** `.claude/commands/fixpr.md`、`.claude/commands/runpr.md`、`docs/dev/guides/fixprguard_guide.md`

-   [ ] `fixpr.md`・`runpr.md` のすべての `--protocol 4` を `--protocol 5` にする。
-   [ ] ガイドの例の `--protocol 4`、出力の `"protocol":4`、4 章の「現在の番号は `4`」を `5` にする（1.4 の行の一覧）。

**成功の条件:** `git grep -n -e '--protocol 4' -e '"protocol":4' -- .claude/commands docs/dev/guides` の出力が空で、`TestDocumentedProtocolMatches` が通る。

#### ステップ 3-2: `runpr.md` の状態の確認とメンションの検出

**変更するファイル:** `.claude/commands/runpr.md`、`cmd/yt2column/fixpr_docs_test.go`、`cmd/fixprguard/docs_test.go`

-   [ ] ステップ 1 の `fixprguard head` の直後、`gh pr checkout` の前に、`fixprguard interaction-limits --protocol 5 <number>` を 1 回だけ呼ぶことを書く。呼ぶ位置、結果の扱いの表、状態ごとに示す内容と利用者が行うこと、LIMITS_NOTE の保持と報告への載せ方は、設計書 3.5 のとおりである。`MERGED`・`CLOSED` では呼ばないこと（ステップ 1 がその前に止まる）と、standalone の変更では PR を作った後の `head` の後に呼ぶことを、位置の説明から読み取れるようにする。
-   [ ] 冒頭の段落（`runpr.md:13-31`）を合わせる。`fixprguard` の用途の一覧に `interaction-limits`（ステップ 1）を加え、メンションの一覧の `--jq` が URL と時刻のほかには、書いた人が利用者か、Bot かを示す真偽値だけを出力することを書き、`--reviewer` を取らないサブコマンドの記述に `interaction-limits` が含まれるようにする。
-   [ ] 「Authorization」に、`interaction-limits` の `gh repo view`・`gh pr view`・`gh api --include` を加える（設計書 3.5）。
-   [ ] ステップ 3 の 3 つのメンションの一覧の `--jq` に、書いた人が利用者（`<user-login>` の `User`）かどうかと、Bot かどうかの真偽値を加える。本文は今までどおり出力しない。人が書いた利用者以外のメンションで止まること、Bot だけなら URL を示して続けるかを問うこと、調べる時点（ステップ 3 の各回の読み取りで、Codex の依頼の状態の規則の前と、ステップ 4 のマージの問いの前）を書く（設計書 3.5）。自動レビュアーの名前を `_context.md` から読む形のままにし、`runpr.md` に `chatgpt-codex-connector` を書かない（`TestContextAndCommandsAgree` が拒否する）。
-   [ ] `runpr.md` に、interaction limits・作成の制限・ruleset を変える呼び出しと、PR を閉じる・作り直す呼び出しを書かない（AC-17）。止まったときの報告は、ガイド 10 章の手順を示すだけにする。
-   [ ] `TestDocumentedInvocationsParse` の `runpr.md` の期待（`cmd/fixprguard/docs_test.go:190`）に `cmdInteractionLimits` を加える。`TestFixprguardCallSites` に、`runpr.md` のステップ 1 の `fixprguard interaction-limits ` の行を加える。
-   [ ] `TestRunprDocContract` の `head_before_checkout` を広げ、ステップ 1 で `fixprguard interaction-limits ` が `fixprguard head ` の後、`gh pr checkout` の前にあることを確かめる（1.3）。
-   [ ] `runprListingFixtures` と、一覧の項目の名前の表（`mention` の種類）を、設計書 7.3 のとおりに改める。2 つの真偽値は `mention` の項目の必須の項目にし、項目があることと fixture の期待の値に等しいことを確かめる。
-   [ ] fixture には、書いた人を 5 通り（利用者の `User`、利用者と同じ名前の `Bot`、Codex の名前の `User`、ほかの `User`、ほかの `Bot`）用意し、3 つの一覧（`issues/<number>/comments`・`pulls/<number>/comments`・`pulls/<number>/reviews`）のそれぞれに入れる。
-   [ ] 壊して確かめる: 呼び出しの行を `gh pr checkout` の後ろに動かす（`head_before_checkout`）、1 つの一覧の `--jq` から真偽値の項目を外す、真偽値の計算を逆にする（`listings_print_no_body`）。

**成功の条件:** `go test -tags test ./cmd/yt2column ./cmd/fixprguard` が通る。

#### ステップ 3-3: `_context.md` の「Guard tool」

**変更するファイル:** `.claude/commands/_context.md`

-   [ ] 「Guard tool」の行（`:188`）の、`fixpr.md`・`runpr.md` がガードツールを呼ぶ用途の一覧に、`runpr.md` が interaction limits の状態を読むことを加える（設計書 3.8）。

**成功の条件:** PR-2 のレビューで読む。

#### ステップ 3-4: ガイドの 4 章・5 章・6 章・10 章と、その値のガード

**変更するファイル:** `docs/dev/guides/fixprguard_guide.md`、`cmd/fixprguard/docs_test.go`

-   [ ] 4 章・5 章・6 章・10 章を、設計書 3.8（4 章・5 章・6 章）と 3.7（10 章）のとおりに改める。5 章の節の見出しは、既存の節と同じ ``### `interaction-limits PR` `` の形にする。6 章の `**実行関連ファイル。**` の印はそのまま残す（1.4）。
-   [ ] 新しい内容を、もとになる事実と照らす。10 章の設定の場所・ruleset の項目・更新と作り直しの手順は設計書 1.3 の観測と ruleset の読み取りの結果と、読むだけの `gh api --include` の例の引数は `limits.go` の引数と、5 章の出力の例の項目と値の形は `InteractionLimitsInfo` の JSON のタグとステップ 2-4 のテストの出力と照らす。照らしたものをコミットメッセージに書く。
-   [ ] `cmd/fixprguard/docs_test.go` に、5 章の `interaction-limits` の節の表が `InteractionStates()` のすべての値（`MarshalText` の名前）と `UnverifiableReasons()` のすべての値をコードの区切りで挙げることを確かめるテストを加える（設計書 7.3、1.4）。
-   [ ] 壊して確かめる: 5 章の節の表から値を 1 つ消す（ガイドのほかの場所に同じ値が残っていても失敗する）。

**成功の条件:** `make fmt`、`make test`、`make lint` が通る（フェーズ 3 の完了の条件）。

#### ステップ 3-5: PR-2 を作る前の確認

-   [ ] ほかの worktree で `/fixpr`・`/runpr` が動いていないことを利用者に確かめる（設計書 8 章）。
-   [ ] このブランチで `make install-fixprguard` を実行する（ガイド 3 章。PR-2 の `/runpr` は新しいツールを使う）。

**成功の条件:** 作業ツリーが clean な状態で入れ、`go version -m "$(command -v fixprguard)"` の `vcs.revision` がこのブランチの head で、`vcs.modified` が `false` である。この後に commit を加えたら（`/fixpr` の修正を含む）、`/runpr` を続ける前に入れ直す。

### PR-2 作成ポイント: scope rule, interaction-limits, and protocol 5

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `feat(0057): 実行関連ファイルを常に範囲外にし、/runpr が interaction limits を確かめて止まるようにする`

**レビュー観点**: 応答の検査が要件書 4.2 の拒否の規則をすべて満たし、どの拒否でも部分的な結果を使わず `unverifiable` になること / 分類の順序、AC-18・AC-26 の境界と、月末の設定の時刻の見積もり / `runpr.md` が状態を読む位置と、止まる状態ごとの報告、利用者以外のメンションで止まる時点 / 出力と報告が応答の本文を含まないこと

**実装モデル要件**: frontier-required

**判定理由**: 信頼できない GitHub の応答を受け入れる形に写す境界と、`protocol` を上げて新しい規則を入れるセキュリティの関門（パネルモードの条件）を含む

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 残りの文書（設計書 8 章の手順 4）

#### ステップ 4-0: PR-2 のマージの後の作業と、観察の記録

**変更するファイル:** `docs/tasks/0057_reviewer_relay_guard/03_implementation_plan.md`

-   [ ] `main` で `make install-fixprguard` を再実行する（ガイド 3 章）。
-   [ ] `git worktree list` で作業中のブランチを列挙し、それぞれに `main` を取り込む。取り込んだブランチの名前を、このステップに書く（設計書 8 章）。
-   [ ] PR-1・PR-2 とその `/runpr` で観察したこと（AC-11 の 3 つの観察）と、PR-2 の `/runpr` が報告した interaction limits の状態と期限の日付を、7 章の AC-10・AC-11 の行に記録する（1.3）。

**成功の条件:** このステップに取り込んだブランチの一覧（なければ「なし」）があり、AC-10・AC-11 の行に PR-1・PR-2 の観察がある。

#### ステップ 4-1: ガイドの 1 章と 7.2

**変更するファイル:** `docs/dev/guides/fixprguard_guide.md`

-   [ ] 1 章の役割分担の表の `fixprguard` の行と、7.2.4 の一覧（ガイド:350）を、設計書 3.8 のとおりに改める。
-   [ ] 7.2.3 の「同じパッケージの `.go` のファイル」「ファイルシステムの Unicode の畳み込みによる別名」「確認への慣れ」に、第三者が Codex を介してその状況に至りうることも書き加える（1.3）。
-   [ ] 7.2 の最後（7.2.5 の後、8 章の前）に「#### 7.2.6」を加え、要件書 4.2 の 11 項目の残余のリスクのうち 7.2.3 にないものと、設計書 5.3 の分かっていることを書く。
-   [ ] 要件書 4.2 の 11 項目が、7.2.3 と 7.2.6 のどこかにすべて書かれていることを、項目ごとに確かめる（AC-21。`manual`）。

**成功の条件:** 文書のガードが通る（`subsection(guide, "### 7.2")` が 7.2.6 を含む）。

#### ステップ 4-2: 用語集と `package_reference.md`

**変更するファイル:** `docs/translation_glossary.md`、`docs/dev/conventions/package_reference.md`

-   [ ] 用語集の「実行関連ファイル」の説明（`:113`）を、設計書 3.8 のとおりに改める。「interaction limits」「作成の制限」「push の制限」を、要件書 6 章の定義で加える（AC-23）。
-   [ ] `package_reference.md` の `cmd/fixprguard` の行（`:15`）と `internal/fixprguard` の行（`:25`）を、設計書 3.8 のとおりに英語で改める。`NewGitHubWithoutReviewer` で呼べる呼び出しの一覧に `InteractionLimits` を加える。

**成功の条件:** PR-3 のレビューで読む。

#### ステップ 4-3: 0030 と 0036 の文書の注記

**変更するファイル:** `docs/tasks/0030_fixpr_untrusted_threads/01_requirements.md`、`docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md`、`docs/tasks/0036_fixprguard_exec_file_scope/01_requirements.md`、`docs/tasks/0036_fixprguard_exec_file_scope/02_architecture.md`、`docs/tasks/0057_reviewer_relay_guard/_checks/notes_only.sh`（新規）、`docs/tasks/0057_reviewer_relay_guard/03_implementation_plan.md`（AC-22 の記録）

-   [ ] 設計書 3.8 の表の節ごとに、節の初めに `> **注記（タスク 0057）:** ...` の引用ブロックを 1 つ加える。注記には設計書 3.8 の 4 点のうち節の内容に合うものと、本タスクの要件書と設計書への参照を書く（AC-22）。本文の規則、状態の表、`Comments` は変えない。
-   [ ] コミットメッセージに、文書ごとに 1 行の訂正の行（`Editorial correction: <document path>: ...; no decision changed`）を書く（`requirements_process.md`「Recording in the PR description」）。
-   [ ] `_checks/notes_only.sh <upstream> <head>` を書く。merge-base と head の間で、4 つの文書のそれぞれについて次を確かめる。(1) 文書が変わっている。(2) 差分に消した行がなく、加えた行はどれも `>` で始まるか空である。(3) `## Document Status` の表と、`## 変更履歴` の節（ある場合）がバイト単位で同じである。(4) 設計書 3.8 の表の節ごとに、`> **注記（タスク 0057）:**` で始まる引用ブロックがちょうど 1 つ加わっている。引数・出力・終了コード・使い方の書き方は、`docs/tasks/0056_legacy_approval_history/_checks/task_docs_unchanged.sh` の形に合わせる（POSIX の `sh` と `git`）。
-   [ ] `_checks/notes_only.sh origin/main HEAD` を PR のブランチで 1 回実行し、出力と 2 つのコミット ID を 7 章の AC-22 の行に記録する。
-   [ ] 壊して確かめる: 次のそれぞれを一時的なコミットで作り、それに対して実行して終了コード `1` になることを確かめ、一時的なコミットを捨てる。状態の表の 1 文字の変更、本文の 1 行の書き換え、`>` で始まらない行の追加、ある節の注記の削除、ある節への 2 つ目の注記、1 つの文書の変更の取り消し。結果を AC-22 の行に書く。

**成功の条件:** スクリプトが終了コード `0` で終わり、6 つの壊した場合のどれもが終了コード `1` になる。

**フェーズ 4 の完了の条件:** `make test`、`make lint`、文書のガードが通る。

### PR-3 作成ポイント: remaining documents

**対象ステップ**: 4-0 / 4-1 / 4-2 / 4-3

**推奨タイトル**: `docs(0057): ガイドの残余のリスク・用語集・過去のタスクの文書を新しい規則に合わせる`

**レビュー観点**: ガイド 7.2.3・7.2.6 に要件書 4.2 の 11 項目がすべてあること / 0030・0036 の注記が節の内容に合い、本文と状態を変えていないこと / 用語集の「実行関連ファイル」が新しい規則に合うこと

**実装モデル要件**: standard

**判定理由**: パネルモードの条件にも条件付きの確認の 2 つ以上にも当たらず、既存コード調査結果に競合する案もない。文書と一度だけ実行するスクリプトだけである

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 内容 | 成果物 |
|---|---|---|
| M1 | CodeRabbit のスレッドを信頼しない | PR-1（`_context.md`、`fixpr_docs_test.go`、ガイドの例） |
| M2 | 実行関連ファイルが常に範囲外になり、`/runpr` が interaction limits を確かめて止まる | PR-2（ガードツール、`runpr.md`・`fixpr.md`、ガイドの 4・5・6・10 章） |
| M3 | 文書が新しい規則と残余のリスクを示す | PR-3（ガイドの 1 章・7.2、用語集、`package_reference.md`、0030・0036 の注記） |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-0 / 1-1 / 1-2 | 本書の PR の観察の記録、自動レビュアーの一覧を `chatgpt-codex-connector` だけにし、そのガードとガイドの例を合わせる | standard |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | 範囲の規則、`splitIncluded` の拡張、`interaction-limits`、`Protocol` の `5`、`runpr.md` の状態の確認とメンションの検出、ガイドの規範の部分 | frontier-required |
| PR-3 | 4-0 / 4-1 / 4-2 / 4-3 | マージの後の作業と観察の記録、ガイドの 1 章・7.2、用語集、`package_reference.md`、0030・0036 の注記と確認のスクリプト | standard |

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** 範囲の規則は `scope_test.go::TestScopeRules` の表に行を加え、`finish` の行を `worktree_flow_test.go` に加える。`--include` の出力の分け方は `trust_test.go`、制限の状態の読み取りと分類は新しい `limits_test.go`、サブコマンドは `cmd/fixprguard/main_test.go`（既存の表への追加と新しいテスト）で確かめる（設計書 7.1）。`gh` は `cmdrunner/testutil` の `Fake` と、期限の時刻も記録するように広げた `deadlineRunner` に置き換え、GitHub に接続しない。
-   **壊して失敗を確かめる:** 設計書 7.2 の表の 13 行と、各ステップの「壊して確かめる」の項目を確かめ、コミットメッセージに書く。
-   **文書のガード:** 自動レビュアーの一覧（`TestContextAndCommandsAgree`・`TestContextReviewerRowsMalformed`）、`--protocol` の値（`TestDocumentedProtocolMatches`）、`runpr.md` の呼び出し（`TestDocumentedInvocationsParse`・`TestFixprguardCallSites`・`TestRunprDocContract`）、ガイドの状態と理由の値（ステップ 3-4 の新しいテスト）。文面は PR のレビューで読む（設計書 7.3）。
-   **前後の関係:** 0030・0036 の文書の状態を変えず注記だけを加えることは、`_checks/notes_only.sh` を一度実行して確かめる（ステップ 4-3）。
-   **結合と手動の確認:** AC-10・AC-11 は PR での観察を記録する（設計書 7.4、1.3）。
-   **後方互換:** 古いツールと新しい文書、新しいツールと古い文書の組み合わせが終了コード `2` で止まることを、`TestProtocolMismatch`（`Protocol + 1` と `Protocol - 1`）で確かめる（AC-19）。すでに PR にある CodeRabbit のスレッドが想定外の著者のスレッドになることは、AC-06 の確かめに含まれる。
-   **カバレッジ:** `limits.go` の関数は、受け入れる形・拒否の各段・分類の各分岐をテストで通す。テストを削除しないので、`go tool cover -func` の比較は要らない。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| PR-2 のブランチでツールを入れると、ほかの worktree の `/fixpr`・`/runpr` が終了コード `2` で止まる | 並行する作業が止まる | ステップ 3-5 で、ほかの worktree で動いていないことを確かめてから入れる。マージの後に作業中のブランチに `main` を取り込む（ステップ 4-0）。PR-2 がマージの前に長く止まるときは、`main` で `make install-fixprguard` を実行して元のツールに戻す |
| PR-2 のマージの後、2026-11-02T21:30:10Z より前に作られた開いている PR が `predates` で止まる | それらの PR を作り直す手間 | 正しい判定として受け入れ、マージの後に作り直す（設計書 8 章）。PR-3 は PR-2 のマージの後に作るので当たらない |
| `gh` の出力の形が変わり、すべての実行が「確かめられない」になる | `/runpr` で PR を進められない | `reason` で原因の見当をつけ、ガイド 10 章の手順で調べる。`/mergepr` で直接マージする手段が残る（設計書 5.3） |
| CodeRabbit が設定に反してスレッドを作り、想定外の著者のスレッドとしてマージを止める | 本タスクの PR の `/runpr` が止まる | 利用者が名指しするか解決する（設計書 5.3）。PR-1 のマージの後から当たる |
| ruleset が `/mergepr` か Codex のレビューを止める | AC-11 が成り立たない | 要件書 1 章の決定に従い作業を止め、決定の変更として要件書に戻す（設計書 3.7） |
| PR-2 が大きく、レビューに時間がかかる | 日程の遅れ | PR-2 は frontier の実装モデルで実装し、`レビュー観点` を検査の境界と `runpr.md` の振る舞いに絞る |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-0 / 1-1 / 1-2）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5）
- [ ] PR-3 マージ済み（対象ステップ: 4-0 / 4-1 / 4-2 / 4-3）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種類の列は、`test`（実行して誤った振る舞いで失敗するテスト）、`static`（ガードのテスト・`make` の対象）、`diff`（`_checks/` のスクリプトを PR のブランチで一度実行し、出力と 2 つのコミット ID を記録する）、`manual`（PR のレビューでの読み取り、PR での観察）である。

| AC | 確かめ | 種類 | 実装 |
|---|---|---|---|
| AC-01 | `internal/fixprguard/scope_test.go::TestScopeRules`（AC-01 の信頼する著者の行、`denyListAtPathItself`、`makefile_is_path`） | test | ステップ 2-1 |
| AC-02 | `internal/fixprguard/scope_test.go::TestScopeRules`（AC-01 の各パスの名指しの行。規則を変える前のコミットで通ることを確かめる） | test | ステップ 2-1 |
| AC-03 | `internal/fixprguard/scope_test.go::TestScopeRules`（AC-03 の 5 行。規則を変える前のコミットで通ることを確かめる） | test | ステップ 2-1 |
| AC-04 | `internal/fixprguard/worktree_flow_test.go` の、`Makefile` をスレッドの `path` にした `finish` の行（ステップ 2-1）と、既存の `TestFinishAfterOutOfScope` | test | ステップ 2-1 |
| AC-05 | `cmd/yt2column/fixpr_docs_test.go::TestContextAndCommandsAgree`（`wantReviewers`）、`TestContextReviewerRowsMalformed`、`cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`（`checkDocumentedReviewer`。`--reviewer` が一覧からだけ来ること） | static | ステップ 1-1 |
| AC-06 | AC-05 の一覧の値と、`internal/fixprguard/trust_test.go::TestAssessAuthors`（`second_reviewer_not_passed`。既存）、`internal/fixprguard/github_test.go::TestBeginNeverFetchesUntrustedBody`（既存） | test・static | ステップ 1-1 |
| AC-07 | `internal/fixprguard/github_test.go::TestBeginAcceptsNamedThread`（既存）、`scope_test.go::TestScopeRules`（名指しの行、`named_go_same_package` など） | test | ステップ 2-1 |
| AC-08 | ガイドの説明と例を PR-1 のレビューで読む（1.3） | manual | ステップ 1-2 |
| AC-09 | ガイド 10 章を、もとになる事実と照らし（ステップ 3-4）、PR-2 のレビューで読む | manual | ステップ 3-4 |
| AC-10 | PR-2 の `/runpr` が報告した状態を記録する（ステップ 4-0。記録: 未）。PR-3 の `/runpr` が「有効」と期限の日付を報告することを、その報告で観察する（1.3）。分類は `internal/fixprguard/limits_test.go` の `active` の行が確かめる | manual・test | ステップ 2-3・4-0 |
| AC-11 | 本書の PR（ステップ 1-0）と PR-1・PR-2（ステップ 4-0）で、Codex の要約のコメントの完了の状態、利用者の `git push` によるブランチの作成と更新、`/mergepr` の squash merge とブランチの削除を観察し、記録する。記録（本書の作成の時点、`f0cb3668`）: 設計書の PR #634 は、ruleset の設定の後に squash merge された（squash commit `f0cb3668`）。次のブランチ `isseis/0057-reviewer-relay-guard-03` は、そのマージの直後に `origin` に作られ、upstream を持つ（`git rev-parse --abbrev-ref @{u}` の出力が `origin/isseis/0057-reviewer-relay-guard-03`）。本書の PR #637 は、PR を開いた後にも commit を push して head を `05c81d1f` に更新でき、Codex の要約のコメントが、その head に含まれる commit `8be7d34f` について `completed`（`has_any_findings` は `false`）に達した。`/mergepr` の squash merge は squash commit `34c85bb1` を作り、ブランチ `isseis/0057-reviewer-relay-guard-03` は `origin` から消えた。`/runpr` ステップ 5 は `gh issue develop` で次のブランチ `isseis/0057-reviewer-relay-guard-04` を `origin` に作り、そのブランチは upstream を持つ（`git rev-parse --abbrev-ref @{u}` の出力が `origin/isseis/0057-reviewer-relay-guard-04`） | manual | ステップ 1-0・4-0 |
| AC-12 | `cmd/fixprguard/docs_test.go::TestDocumentedInvocationsParse`（`runpr.md` の `interaction-limits`）、`cmd/yt2column/fixpr_docs_test.go::TestFixprguardCallSites`、`TestRunprDocContract`（`head_before_checkout`）。1 回だけ読むこと、マージ済みで読まないこと、「Authorization」は PR-2 のレビューで読む | static・manual | ステップ 3-2 |
| AC-13 | `internal/fixprguard/limits_test.go`（`active` の分類と `expiresOn`）、`cmd/fixprguard/main_test.go`（`active` の出力）。最終の報告の 1 行は PR-2 のレビューで読む | test・manual | ステップ 2-3・2-4・3-2 |
| AC-14 | `internal/fixprguard/limits_test.go`（`expiring` の分類）。警告の内容と、示す 2 つの時点は PR-2 のレビューで読む | test・manual | ステップ 2-3・3-2 |
| AC-15 | `internal/fixprguard/limits_test.go`（`existing_users`・`contributors_only` が `weak`）。警告の内容は PR-2 のレビューで読む | test・manual | ステップ 2-3・3-2 |
| AC-16 | `internal/fixprguard/limits_test.go`（拒否の各行が `unverifiable`）、`cmd/fixprguard/main_test.go`（出力が目印を含まず、標準エラーが空）。報告の内容は PR-2 のレビューで読む | test・manual | ステップ 2-3・2-4・3-2 |
| AC-17 | `runpr.md` に設定を変える呼び出しと、PR を閉じる・作る呼び出しがないことを、PR-2 のレビューで読む（standalone の変更では `gh pr create` を使うので、呼び出しの有無を単純に検査するガードにできない） | manual | ステップ 3-2 |
| AC-18 | `internal/fixprguard/limits_test.go`（`Date` と同じ時刻、14 日後、14 日と 1 秒後の 3 行） | test | ステップ 2-3 |
| AC-19 | `cmd/fixprguard/main_test.go::TestProtocolMismatch`（`Protocol + 1` と `Protocol - 1`、`interaction-limits` を含むすべてのサブコマンド）、`cmd/fixprguard/docs_test.go::TestDocumentedProtocolMatches` | test・static | ステップ 2-4・3-1 |
| AC-20 | ガイド 6 章の規則 4 と判定の順序を PR-2 のレビューで読む | manual | ステップ 3-4 |
| AC-21 | ガイド 7.2.3・7.2.6 を PR-3 のレビューで読み、要件書 4.2 の 11 項目と照らす | manual | ステップ 4-1 |
| AC-22 | `docs/tasks/0057_reviewer_relay_guard/_checks/notes_only.sh origin/main HEAD`（記録: 未。merge-base と head のコミット ID、出力、6 つの壊した場合の終了コードを書く）。注記の文面は PR-3 のレビューで読む | diff・manual | ステップ 4-3 |
| AC-23 | 用語集の項目を PR-3 のレビューで読む | manual | ステップ 4-2 |
| AC-24 | 各 PR の `make test`・`make lint` と CI の `doc-guard` | static | すべてのステップ |
| AC-25 | `internal/fixprguard/limits_test.go`（`none`・`weak`・`predates`・`unverifiable` の分類と、分類の順序の行）。`runpr.md` がその場で止まることと報告の内容は PR-2 のレビューで読む | test・manual | ステップ 2-3・3-2 |
| AC-26 | `internal/fixprguard/limits_test.go`（設定の時刻と同じ時刻・1 秒前の行、月末の行、月にない日の行） | test | ステップ 2-3 |
| AC-27 | `cmd/yt2column/fixpr_docs_test.go::TestRunprDocContract`（`listings_print_no_body` の 3 つの一覧の真偽値の fixture）。止まる時点と報告は PR-2 のレビューで読む | static・manual | ステップ 3-2 |

## 8. 成功基準 (Success Criteria)

-   **機能:** 7 章のすべての AC の確かめが済んでいる。
-   **品質:** 各 PR で `make test`・`make lint` が通り、壊して失敗することをコミットメッセージに記録している。
-   **セキュリティ:** 要件書 4.2 の拒否の規則が 1 つずつテストされ、ガードツールの出力と `/runpr` の報告が応答の本文を含まない。実行関連ファイルは、どの著者のスレッドからの変更でも範囲外になる。
-   **文書:** ガイド・用語集・`package_reference.md`・0030 と 0036 の注記が新しい規則に合い、PR のレビューで読まれている。

## 9. 次のステップ (Next Steps)

-   PR-3 のマージの後、タスクの issue（#629）を閉じる（`/runpr` ステップ 5）。
-   CodeRabbit が設定に反して自動レビューを行う件の issue がまだなければ、利用者に確かめてから作る（要件書 1 章の決定。要件書の PR の後に作る予定だった）。
-   #405 で自動レビュアーを加えるときは、その Bot が外部の人の指示に応じるかを評価する（設計書 9 章）。

## 変更履歴

- 2026-11-03 承認（isseis、#637）
