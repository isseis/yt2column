# 実装計画書：文書のテストから、古い仕様・一度きりの確かめ・過剰な確認を取り除く

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-31 |
| Review date | 2026-10-31 |
| Reviewer | isseis |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN・K1〜K5・D1〜D7 は要件書の項番と分類を、J-NN は設計書 3.2 の判断の一覧の行を、「設計書 3.1.1」などは設計書の節を指す。用語は設計書の冒頭の定義（判定、判定の一覧、判断の一覧、補助、補助の置き換え、製品のコード）と同じ意味で使う。

既存のコードについての記述は、コミット `db9986c6`（2026-10-31 の `main`）で確かめた。設計書が確かめた `bf314092` から `db9986c6` までに変わったのは設計書だけである（`git diff --stat bf314092 db9986c6` は `docs/tasks/0052_doc_test_pruning/02_architecture.md` の 1 ファイルだけを示した）。そのため、設計書の `file:line` はそのまま使える。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書 3.1 の判定の一覧のとおり、文書のテストから D1〜D7 に当たる確認を取り除き、使い手を失う補助を消し、3 つの補助を `internal/doctestutil` の関数に置き換える。あわせて、`CLAUDE.md` に方針を加え、文書のガードのコマンドの一覧と、文書のテストを述べる記述を結果に合わせる。何を残し何を取り除くかは設計書 3〜5 章で決まっている。本書は、それをどの PR・どのコミット・どの確かめで行うかを定める。

### 1.2. 実装の原則

-   設計書 8 章の PR の順（PR-1 → PR-2 → PR-3）で実装する。本書のフェーズ 1〜3 は、その PR-1〜PR-3 にそのまま対応する。前の PR のマージの後に次の PR を始める。
-   各 PR の始めに、`main` で判定の一覧と実装中の計画を調べ直す（設計書 3.5、6.1）。判定の一覧の対象のテストが設計書の時点から変わっていれば、変わった行に基準を当てはめ直す。新しい判断の候補は、取り除く前にレビュアーに諮る（設計書 3.2 の末尾）。
-   取り除くコミットと補助を置き換えるコミットを分ける（設計書 1.1）。取り除きは PR ごとに 1 つのコミットにまとめる（1.3）。
-   Go のテストや補助を取り除く、または置き換えるコミットごとに、製品のコードの関数のカバレッジを前後で比べ、その結果をコミットメッセージと PR の説明に記す（設計書 7.1、AC-11）。
-   一部を削除するテストと、補助の置き換えを受けるテストでは、残す確認の値を一度壊して落ちることを確かめ、コミットメッセージに記す（設計書 7.2）。中身を変えないテストは、壊して確かめなくてよい。
-   一度きりの確かめは、本タスクのディレクトリの `_checks/` に置いたスクリプトで行う（1.6）。PR ごとに実行し、出力と、比べた 2 つのコミットを、本書 7 章の該当の行に記録する。
-   製品のコードと文書の本文は変えない（設計書 1.1、AC-15）。
-   Go のコメント・識別子・文字列リテラルは英語で書く（`_checks/` のスクリプトも同じ）。例外は、日本語の文書の見出しと一致させるガードテストのリテラルだけである（`CLAUDE.md`「Go Idioms」）。

### 1.3. 計画の際に決めたこと（2026-10-31）

人間のレビュアーが決めたもの:

-   **AC-19 は `_checks/` の一度きりのスクリプトで確かめる:** 文書のガードのコマンドを書く 4 か所（設計書 2.2）のパッケージの一覧を互いに照らし合わせるテストは、`db9986c6` の時点でない（`git grep` で `_test.go` と `.ts` を探して確かめた）。ただし `task-pr-handoff.md` は、`cmd/codexreview` の K5 の走査が読む（次の項）。恒久的なガードテストを加える案もあったが、採らない。理由: 本タスクは文書のテストを減らすタスクで、PR-1 で加える方針（D7 (c)、防ぐ誤りに見合わない文書の確認を足さない）とも噛み合わない。スクリプトは PR-1 と PR-3 で実行する（1.6 の `guards`）。
-   **取り除きのコミットは PR ごとに 1 つにまとめる:** テストのファイルごとに分ける案は、設計書 3.1 の表と対応して読みやすいが、カバレッジの比較（`-coverpkg=./...` で全体を走らせる）の回数が増える（PR-2 で 5 回）。PR ごとに 1 つにすれば、取り除きの比較は PR ごとに 1 回で済む。レビューは、PR の説明に載せる判定の一覧の行（AC-04）と照らして読む。補助の置き換えは、設計書 1.1 のとおり別のコミットにする。
-   **`task-pr-handoff.md` には `./cmd/codexreview/...` と書き、走査の誤検出は別の issue にする:** `cmd/codexreview::TestDocumentedInvocationsParse`（K5）の走査 `invocations`（`cmd/codexreview/docs_test.go:192-210`）は、コマンドの文書のコードスパンとコードブロックで、`path.Base` が `codexreview` になる語をツールの呼び出しとみなす。`timeout 600 codexreview` のような前置きのある呼び出しも捉えるため、わざと広く取っている（`TestInvocations` の `after_prefix` の行、`:227`）。そのため、`task-pr-handoff.md:40` に `./cmd/codexreview` を書くと、`go test` の行が呼び出しとして拒否される（レビューのサブエージェントが、リポジトリの外の写しで再現した）。`./cmd/codexreview/...` と書けば `path.Base` は `...` になり、`go list ./cmd/codexreview/...` は `cmd/codexreview` だけを返す（`db9986c6`）ので、対象のパッケージは変わらない。走査を直すことは K5 の確認を変えることになり、要件書の「K4・K5 の確認は減らさない」に反し、要件書 2.3（スコープ外）の範囲も超えるので、別の issue で扱う。副作用は 2 つで、どちらも実害がない。`cmd/codexreview` の下にパッケージが増えると、`task-pr-handoff.md` の CHECKS（タスクの文書を作るコマンドが push の前に走らせる文書のガード）が走らせるテストが増えること、綴りを `./cmd/codexreview` に戻すと K5 のテストがすぐ落ちることである。設計書 2.2・3.9 は、この綴りを編集上の訂正として加えた（設計書の `Comments`）。`fixprguard_guide.md` はコマンドの文書でないので、`./cmd/codexreview` と書く。
-   **`TestInstallTargetDocumented` の名前を変える:** `CLAUDE.md` の部分を取り除くと、このテストは `Makefile` だけを読む（設計書 3.1.4）。名前と中身を合わせるため、`TestInstallTargetInMakefile` に変える（1.5 の台帳）。

計画で決めたもの（理由とともに記す）:

-   **`_checks/` のスクリプトは 4 つにする:** 変えたファイルの範囲（`scope`）、ガードのコマンドの一致（`guards`）、カバレッジの比較（`coverage`）、テストの宣言の変化と取り除いた名前（`tests`）である（1.6）。PR ごとに手で `git grep` や比較をすると、手順が記録に残らず、誤りを後から確かめられない。スクリプトにすれば、同じ確かめを 3 つの PR で繰り返せる。
-   **`scope` と `guards` は PR-1 で、`coverage` と `tests` は PR-2 で加える:** それぞれを最初に使う PR で加える。

### 1.4. 既存コード調査結果

調べた時点は `db9986c6` である。判定ごとのテスト・行番号・補助の使い手の数は設計書 3.1・3.4 のとおりで、`db9986c6` で次のことを確かめた。

-   設計書 3.1 の取り除く確認を持つ 13 のテストと、置き換えを受けるテストは、すべて設計書の挙げる `file:line` に `func` の宣言がある（`git grep -n "func <name>("`）。
-   設計書 3.4.1 の補助の使い手は、表のとおりである（`git grep -n -w`）。`beforeFirstHeading` は `docs/tasks/0048_doc_language_policy/_checks/readme/main.go:197` にも同名の関数があるが、別のパッケージ（`go list ./...` に含まれない `_checks`）の関数で、本タスクは変えない。`numbered` の語は `internal/codexreview/message.go:38` と `internal/publisher/testutil/integration.go:175` のコメントにも現れるが、識別子 `numbered`（`fixpr_docs_test.go:1075`）とは関係しない。
-   取り除くテストの名前と置き換える補助の名前（1.5 の台帳と設計書 3.4 の表）は、`docs/tasks/` の外の文書に現れない（`git grep -n -w` を、`docs/tasks` と `*.go` を除いて実行し、出力が空だった）。AC-20 で文書を直す箇所は、`CLAUDE.md` の 2 つの句（設計書 3.7）だけである。

**一部を削除するテスト**

| テスト | 今の形（`db9986c6`） | 変えること |
|---|---|---|
| `TestSlackDocsContract`（`cmd/yt2column/docs_test.go:171`） | 1 つの値の一覧（`:178-184`）を、`development.md` と `security.md` の両方に `requireMentions` で当てる | `development.md` に当てる一覧から `CLISlackOptInEnv` を外す。`security.md` にはすべての値を当て続ける。`16383` の確認（`:175`・`:193-200`）は変えない（設計書 3.1.1） |
| `TestProductionProfileDocsContract`（`:661`） | README の版ごとに、推奨構成の小節が「設定」の節の中にあることを確かめる（`:675`・`:680-689`） | その部分（`configHeading`・`inConfiguration`）を消し、doc comment の該当の句も消す（J-06） |
| `TestCostDocsContract`（`:1058`） | 末尾で `rates.go` の冒頭のコメントを読む（`:1123-1127`） | その部分と `pricingRatesPath`（`:38`）を消す |
| `TestCodeRabbitConfigStopsReview`（`codex_review_docs_test.go:354`） | `requirePhrases` で英文 3 句を確かめる（`:356-360`） | その呼び出しを消し、doc comment（`:349-353`）の英文を確かめる旨を消す |
| `TestGuideNamesGitStateParts`（`cmd/fixprguard/docs_test.go:461`） | 8 章の行の確認（`:462-474`）の後に、`fixpr.md` の一覧を読む（`:475-481`） | 後半と doc comment の `fixpr.md` の文を消す（J-07） |
| `TestInstallTargetDocumented`（`cmd/codexreview/docs_test.go:656`） | `Makefile` の確認（`:657-662`）の後に、`CLAUDE.md` の `## Commands` を読む（`:664-669`） | 後半を消し、名前を変え（1.5）、doc comment を `Makefile` の確認だけを述べる形にする（J-10） |

**補助の置き換え（設計書 3.4.2）**

| 補助 | 今の形 | 置き換えの注意 |
|---|---|---|
| `commandDocs`（`codex_review_docs_test.go:291`） | `commandsDir`（`:18`。`repositoryRoot + "/.claude/commands"`）の下の `.md` のパスを、`repositoryRoot` を基点にした形で返す | `doctestutil.CommandDocs(t, root)`（`internal/doctestutil/protocol.go:85`）は `root` からの相対のパスを返す。呼び出し側（`:94`・`:217`）で `repositoryRoot` と連結して読み、`contextPath`（`fixpr_docs_test.go:25`）・`postingSections` のパスとの比較を `filepath.Clean` の後の同じ形で行う |
| `readRepoFile`（`cmd/fixprguard/docs_test.go:25`） | `os.ReadFile` を直接呼ぶ | `cmd/codexreview/docs_test.go:35` と同じく `doctestutil.ReadDoc` を包む形にする。呼び出しは変えない（`db9986c6` で 11 か所。ステップ 3-2 が `:477` の 1 か所を消す） |
| 文書を直接読む `os.ReadFile`（`internal/pipeline/pipeline_test.go:708`・`:758`） | `os.ReadFile` とエラーの分岐 | `doctestutil.ReadDoc` にする。`:660` の `firstLineOf` は Go のソースを読むので変えない。`internal/pipeline` のテストは今 `doctestutil` を import していない。`doctestutil` は本モジュールのほかのパッケージに依存しない（`go list -tags test -deps ./internal/doctestutil` の本モジュールの行は自身だけ）ので、import の循環は起きない。`.golangci.yml` の `depguard` に `internal/pipeline` の import を制限する規則はない |
| `markdownSections`（`pipeline_test.go:1107`） | `## ` の見出しの題をキーにしたマップ。呼び出し側は `sections[...]` で 4 つの節を引く（`:773`・`:793`・`:808`・`:819`） | `doctestutil.SectionByHeading`（`markdown.go:90`）は見出しの行の全体（`"## " + 題`）を受け取り、見出しの行を含まない節を返す。見つからないときは空の文字列を返し、マップにないキーを引いたときと同じになる |

**使い手を失う補助（設計書 3.4.1）**

設計書の表のとおりである。消した後に残る import（`internal/pipeline` の `unicode`、拡張の `metadataDocs.test.ts` の `existsSync`・`path`・`repoRoot` など）は、使われなければコンパイラ・`make lint`・`make ext-check` が示すので、そのとおりに消す。

**文書のガードのコマンド**

`.claude/commands/_lib/task-pr-handoff.md:40` と `docs/dev/guides/fixprguard_guide.md:454` は `./cmd/codexreview` を欠き、`.github/workflows/ci.yml:187` と `CLAUDE.md:540` は 5 つのパッケージを挙げる（設計書 2.2）。

**既存の `_checks/` の例**

`docs/tasks/0048_doc_language_policy/_checks/split/main.go` は、冒頭に `//go:build test` を置き、2 つのコミットを引数に取り、`go run -tags test ./docs/tasks/.../_checks/split/ <merge-base> <commit>` で実行する。`go list ./...` は `_` で始まるディレクトリを含まないので、`make test`・`make lint`・`make deadcode` はこれをビルドしない（設計書 1.4）。本書のスクリプトも同じ形にする。

### 1.5. 名前の変更の台帳

| 種類 | 今の名前 | 新しい名前 | 決めた節 |
|---|---|---|---|
| テスト | `cmd/codexreview/docs_test.go::TestInstallTargetDocumented` | `cmd/codexreview/docs_test.go::TestInstallTargetInMakefile` | 1.3 |

設計書は、判定の対象として今の名前（`TestInstallTargetDocumented`）を使う。設計書 3.1.4 が名前の変更を本書に委ねているので、設計書は直さない。本書のほかの箇所は、取り除く前の確認を指すときは今の名前、取り除いた後のテストを指すときは新しい名前を使う。

### 1.6. 一度きりの確かめのスクリプト（`_checks/`）

置き場所は `docs/tasks/0052_doc_test_pruning/_checks/<name>/main.go` で、リポジトリのルートで `go run` で実行する。`make test` と CI は実行せず、マージの後は保守しない（`requirements_process.md` §4）。引数はスクリプトごとに違う（表の「引数」の列）。

| 名前 | 加える PR | 引数 | 確かめること | 失敗とする条件 |
|---|---|---|---|---|
| `scope` | PR-1 | 本計画の PR の番号（PR-1〜PR-3 の `1`〜`3`。`-pr 1` など）、merge-base、head | 2 つのコミットの間で変わったファイル（AC-03・AC-15〜AC-17・AC-21）と、`CLAUDE.md` の「Document tests check values, structure, and executable examples, never prose」の項の既存の文（AC-24） | 変わったファイルが、その PR に許すファイルの外にある。許すファイルは、設計書 2.3 の表のその PR の行に挙がるファイル、本書と `docs/tasks/0052_doc_test_pruning/_checks/` の下のファイル、および PR-3 に限り `docs/tasks/0050_viewer_tab/` の `02_architecture.md`・`03_implementation_plan.md` である（0050 の文書を変えるのは、ステップ 3-1 で変えると決めた場合だけ）。0050 の変更は、許したうえで一覧として出力する。0052 のほかの文書（要件書・設計書・`design_handoff.md`）の変更は失敗とする。`CLAUDE.md` が変わったとき、merge-base のその項の文が、head の同じ項にない |
| `guards` | PR-1 | 1 つのコミット（省略すると作業ツリー）と、PR-3 では `-final` | 4 か所の文書のガードのコマンド（設計書 2.2）が挙げるパッケージの集合（AC-19）。`./cmd/codexreview/...` と `./cmd/codexreview` は同じパッケージとして扱う（1.3） | 4 か所の集合が一致しない。`-final` を付けたとき、その集合が、テストのファイルが `internal/doctestutil` を import するパッケージ（`cmd/yt2column-nmhost` と `docs/tasks/` の下を除く）の集合と一致しない |
| `coverage` | PR-2 | `go tool cover -func` の 2 つの出力のファイル（前と後） | 製品のコードの関数ごとの結果（AC-11・AC-12） | 製品のコードの関数で、結果が違うものがある。製品のコードへの絞り込みは設計書 7.1 のとおり（`//go:build test` で始まる制約を持つファイルと `total:` の行を除き、モジュールのパスの接頭辞を外す） |
| `tests` | PR-2 | merge-base、head、PR-3 では `-final` | 要件書 2.2 の対象のファイルのうち Go のもの（`internal/doctestutil/` と `internal/config/test_helpers.go` を含む）の、トップレベルの宣言の変化（AC-01〜AC-03・AC-07・AC-10）と、比べる 2 つのコミットの間で取り除いた名前の残り（AC-09・AC-20。後の PR で取り除く名前は、その PR の head に残っているのが正しいので対象にしない）。`-final` を付けたときは、設計書 3.4.1・3.4.2 の全削除対象の不在も、比べる範囲で消えたかによらず確かめる。宣言は、関数・メソッド・型と、`var`・`const` の 1 つずつの名前を単位とし、import は含めない。比べるのは、gofmt で整えた後の宣言の本文（doc comment を含む）である | (1) 「変える宣言の一覧」にない宣言が、変わった、消えた、または加わった。<br>一覧は、設計書 3.1 の「削除する」「一部を削除する」「統合する」の行のテスト、3.4.1・3.4.2 の補助、置き換えを受けるテスト（`TestCodexMentionOnlyInRequestRow`・`TestPostingCommandsInventory`・`TestPackageReferenceListsPackages`・`TestPromptsREADMEMatchesContract`）、`cmd/fixprguard` の `readRepoFile`、1.5 の台帳の両方の名前から作る。<br>(2) 比べる merge-base と head の間で取り除いた名前（設計書 3.4.1・3.4.2 の補助、取り除くテスト、1.5 の今の名前のうち、その範囲で消えたもの）が、`docs/tasks/` の外の Go のファイルに識別子として残る。後の PR で取り除く名前が head に残っていることは、失敗としない。<br>(3) `-final` を付けたとき、設計書 3.4.1・3.4.2 の全削除対象が、比べる範囲で消えたかによらず、`docs/tasks/` の外の Go のファイルに識別子として残る。全削除対象の不在は、この `-final` の実行（ステップ 3-4）で確かめる。<br>コメントと文書（`docs/tasks/` の外の `.md` と `.claude/commands/`）に現れる名前は、失敗とせず一覧として出力する。`numbered` のような普通の英単語がコメントに現れることがあるため、読んで判断する（1.4）。<br>出力には、消えた宣言と変わった宣言の一覧も含める |

`metadataDocs.test.ts` は `tests` の対象にしない。その変更は、`make ext-check` と PR のレビューでの読み合わせで確かめる（ステップ 3-2）。

`coverage` に渡す出力は、設計書 7.1 のコマンド（`go test -tags test -count=1 -coverpkg=./... -coverprofile=<file> ./...` の後の `go tool cover -func=<file>`）で作る。結果が違ったときの扱い（取り除くテストだけで測り、揺れか変化かを見分ける）も、設計書 7.1 のとおりである。

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: 方針とガードのコマンド（設計書 8 章の PR-1）

#### ステップ 1-1: 判定の一覧と実装中の計画を調べ直す

**変更するファイル:** `docs/tasks/0052_doc_test_pruning/03_implementation_plan.md`（このステップの記録）

-   [x] `main` の HEAD で、要件書 2.2 と設計書 3.1 の対象のファイルが `bf314092` から変わったかを `git diff --stat` で調べる。変わったテストがあれば、その行に設計書 3.1 の基準を当てはめ直し、新しい判断の候補はレビュアーに諮る（設計書 3.2 の末尾）。
-   [x] 設計書 3.5 の 3 つの方法で、実装中の計画を探す。開いている PR の一覧（`gh pr list`）はネットワークを使うので、レビュアーの承認を得てから実行する。タスク 0050 の PR #595 の状態（マージされたか。取り除く確認を保つ作業が 0050 の計画に入っていないか）を記録する。
-   [x] 調べたコミットと結果を、このステップの下に記録する（AC-05）。

**完了の条件:** 調べたコミット、変わったテストの有無、実装中の計画の有無が本書に記録されている。

**調べ直しの記録（2026-11-01）:**

-   調べたコミット: `b467905e`（`main` の HEAD。本計画の PR #597 のマージ後）。要件書 2.2 と設計書 3.1 の対象のファイルは、`git diff --stat bf314092 b467905e` が `docs/tasks/0050_viewer_tab/02_architecture.md`・`03_implementation_plan.md` と、本タスクの `02_architecture.md`・`03_implementation_plan.md` の 4 ファイルだけを示したので、`bf314092` から変わっていない。設計書 3.1 の判定の一覧を当てはめ直す必要はない。
-   実装中の計画（設計書 3.5 の 3 つの方法）:
    -   `main` の承認済みの計画で未チェックの項目が残るものは、0002・0009・0011・0012・0014・0017・0019〜0023・0026・0027・0030・0031・0038・0047・0050・0052 である。このうち 0050 と 0052 を除くものの未チェックの項目は、PR のマージの記録・ブランチの切り替え・issue を閉じることと、完了の条件の再掲だけで、文書のテストを加える作業を含まない（設計書 3.5 の対象外）。0052 は本タスクである。
    -   計画のない承認済みの設計書はない（`docs/tasks/*/02_architecture.md` の Status と、同じディレクトリの `03_implementation_plan.md` の有無を調べた）。
    -   開いている PR はない（`gh pr list --state open`。`/runplan` の実行の承認に含まれるネットワークの操作）。`docs/tasks/*/02_architecture.md`・`03_implementation_plan.md` を `main` と変えるブランチは `isseis/0043-extension-i18n-03` だけで、それはタスク 0043 の計画のブランチであり、`main` の計画は PR #517 でマージ済みで未チェックの項目がないので、実装中の計画ではない。`isseis/0050-viewer-tab-04` と `origin/claude/festive-bardeen-cw14ci` は `origin/main` より先のコミットを持たない。
-   タスク 0050: PR #595 は 2026-10-31 にマージされた。その計画のステップ 7-2（`03_implementation_plan.md:576`）は `metadataDocs.test.ts` の socket directory の確認（`:64-78`）を消すとしているので、設計書 3.5 の「PR #595 がこの形でマージされれば、本タスクは 0050 の文書を変えない」に当たる。0050 の実装のブランチ `isseis/0050-viewer-tab-04` はまだコミットを持たない。PR-3 の始め（ステップ 3-1）に状態を調べ直す。

#### ステップ 1-2: `scope` と `guards` のスクリプトを加える

**変更するファイル:** `docs/tasks/0052_doc_test_pruning/_checks/scope/main.go`（新規）、`docs/tasks/0052_doc_test_pruning/_checks/guards/main.go`（新規）

-   [x] 1.6 の表のとおりに 2 つのスクリプトを書く。冒頭のコメントに、実行の仕方と、確かめることを英語で書く（`0048` の `split/main.go` と同じ形）。
-   [x] `scope` について、失敗とする条件を一度ずつ作って（許さないファイルを変えた一時的なコミット、PR-1 での 0050 の文書の変更、`CLAUDE.md` の項の既存の文を消した変更）、失敗することを確かめる。一時的な変更はコミットしない。
-   [x] `guards` を `main` の HEAD に対して実行し、2 か所が `./cmd/codexreview` を欠くので失敗することを確かめる（ステップ 1-4 の前の状態が、この失敗の例になる）。
-   [x] 確かめたことをコミットメッセージに記す。

**完了の条件:** `scope` が `main` の HEAD を merge-base と head の両方にした実行で通り、上の条件で失敗する。`guards` が `main` の HEAD で失敗する。

#### ステップ 1-3: `CLAUDE.md` に方針を加える

**変更するファイル:** `CLAUDE.md`

-   [x] 「Testing Strategy」の「Document tests check values, structure, and executable examples, never prose」の項（`CLAUDE.md:492-511`）に、設計書 3.6 の 3 つの規則（D5・D7 (a)・D7 (c)）を、それぞれの理由とともに加える。既存の文は変えない（AC-24）。
-   [x] 加えた文が、`TestCodexMentionOnlyInRequestRow` の確かめる言及を含まないことを、`make test` で確かめる。

**完了の条件:** 文が加わり、`make test` が通る。

#### ステップ 1-4: 文書のガードのコマンドに `cmd/codexreview` を加える

**変更するファイル:** `.claude/commands/_lib/task-pr-handoff.md`、`docs/dev/guides/fixprguard_guide.md`

-   [x] `task-pr-handoff.md:40` の CHECKS のコマンドを、`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline` から `go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview/... ./internal/pipeline` に変える。`/...` を付ける理由は 1.3 のとおりである。
-   [x] `fixprguard_guide.md:454` のコマンドを `go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./cmd/codexreview ./internal/pipeline` に変える。同じ箇所のほかの文は変えない。`:461` の古い記述（「本書（日英）」）は、設計書 4.3 のとおり変えない。
-   [x] 変えた後の 2 つのコマンドを、リポジトリのルートで一度ずつ実行して通ることを確かめる。`go test -tags test ./cmd/codexreview` で、`TestDocumentedInvocationsParse` が `task-pr-handoff.md` の行を呼び出しとみなさないことも確かめる。

**完了の条件:** `go run -tags test ./docs/tasks/0052_doc_test_pruning/_checks/guards/` が通る。

**フェーズ 1 の完了の条件:** `make test`、`make lint`、`make deadcode`、`make ext-check` が通る（AC-14）。`scope -pr 1` を PR の merge-base と head で実行して通り、その出力と 2 つのコミットを本書の 7 章の、`scope` を挙げる行（AC-03・AC-15・AC-17・AC-21・AC-24）に記録した。`guards` の出力も AC-19 の行に記録した。

### PR-1 作成ポイント: testing policy and documentation guard commands

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4

**推奨タイトル**: `docs(0052): 文書のテストの方針に古い仕様と過剰な確認の規則を加え、文書のガードのコマンドを揃える`

**レビュー観点**: 加えた方針の文が D5・D7 (a)・D7 (c) をその理由とともに述べ、既存の規則と K4・K5 の例外を変えていないこと / `task-pr-handoff.md` と `fixprguard_guide.md` のコマンドが `ci.yml`・`CLAUDE.md` と同じパッケージを挙げること / `scope`・`guards` のスクリプトの失敗の条件が 1.6 の表のとおりであること / ステップ 1-1 の調べ直しの記録

**実装モデル要件**: standard

**判定理由**: 文書と一度きりのスクリプトだけの変更で、`mkplan` のパネルモードの条件・既存コード調査結果の競合する案・Conditional checks のどれにも当たらない

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `cmd/yt2column` の取り除き（設計書 8 章の PR-2）

フェーズ 2 のコミットは、`_checks` のスクリプトの追加、取り除き、補助の置き換え、記述の直しの 4 つに分ける。

#### ステップ 2-1: 調べ直しと、`coverage`・`tests` のスクリプト

**変更するファイル:** `docs/tasks/0052_doc_test_pruning/_checks/coverage/main.go`（新規）、`docs/tasks/0052_doc_test_pruning/_checks/tests/main.go`（新規）、本書（記録）

-   [ ] ステップ 1-1 と同じ調べ直しを、`main` の HEAD で行い、記録する。
-   [ ] 1.6 の表のとおりに `coverage` と `tests` を書く。`tests` の残存の検査は、比べる merge-base と head の間で取り除いた名前を対象にし、PR-3 で取り除く名前がこの時点の head に残っていることを失敗としない（1.6）。
-   [ ] それぞれ、失敗とする条件を一度ずつ作って失敗することを確かめ、コミットメッセージに記す（`coverage` は製品のコードの関数の結果を 1 つ書き換えた出力、`tests` は一覧にない宣言を変えた一時的なコミット）。

**完了の条件:** `tests` は `main` の HEAD を merge-base と head の両方にした実行で、`coverage` は同じ出力を前と後の両方にした実行で通り、どちらも上の失敗の条件で失敗する。

#### ステップ 2-2: `cmd/yt2column` の確認・テスト・補助を取り除く

**変更するファイル:** `cmd/yt2column/docs_test.go`、`readme_structure_test.go`、`run_test.go`、`fixpr_docs_test.go`、`codex_review_docs_test.go`、`internal/doctestutil/readme.go`、`internal/config/test_helpers.go`

-   [ ] 取り除く前に、`coverage` に渡す出力を取る（1.6）。
-   [ ] `TestMigrationDocsContract` と `checkMigrationSection` を消す（J-01）。
-   [ ] `TestQuickStartLinks` を消す（J-03）。
-   [ ] `TestNumberedItems`・`numberedItems`・`numberedItemRE` を消す（AC-10）。
-   [ ] `TestProductionProfileLinks` と `promptsReadmePath` を消す（J-04）。
-   [ ] `TestReadmeVersionLinks`・`beforeFirstHeading`・`linksTo` を消す（J-05）。
-   [ ] `TestRunEvaluationProcedureFlags`・`evaluationProcedureCommands`・`fencedCode`・`procedureArgs` と `reviewEvaluationPath` を消す（J-02）。
-   [ ] `TestReadmeSyncStepPlacement`・`readmeSyncIn`・`readmeSyncMarker`・`mktransPath`・`numbered` を消す（J-08）。
-   [ ] 1.4 の「一部を削除するテスト」の表の、`TestSlackDocsContract`・`TestProductionProfileDocsContract`・`TestCostDocsContract`・`TestCodeRabbitConfigStopsReview` の 4 行のとおりに変える。`pricingRatesPath` を消す。各テストの doc comment から、取り除いた確認を述べる句を消す（AC-20）。
-   [ ] `internal/doctestutil/readme.go` から `ReadmeMigrationHeading`・`ReadmeQuickStartHeading`・`ReadmeRequirementsHeading`・`ReadmeConfigurationHeading` を消す。
-   [ ] `internal/config/test_helpers.go` から `DeepSeekAPIKeyEnvForTest` を消す（設計書 3.8 の H-05）。
-   [ ] 一部を削除した 4 つのテストと統合先について、残す確認の値を一度壊して落ちることを確かめる（設計書 7.2）。
    -   `TestSlackDocsContract` は、`security.md` の `CLISlackOptInEnv` を消した写しで落ちること。
    -   統合先の `TestDeepSeekDocsContract` の `cli_slack` は、`development.md` の節の `CLISlackOptInEnv` を消した写しで落ちること。
    -   `TestProductionProfileDocsContract` は、README の推奨構成のブロックの値を 1 つ変えた写しで落ちること。
    -   `TestCostDocsContract` は、`project_overview.md` の `costPrefix` を変えた写しで落ちること。
    -   `TestCodeRabbitConfigStopsReview` は、`.coderabbit.yaml` の `enabled: false` を `true` にした写しで落ちること。
-   [ ] 取り除いた後の出力を取り、`coverage` で比べる。結果が違えば設計書 6.1 の流れ（AC-12）に従う。
-   [ ] 1 つのコミットにし、コミットメッセージに、取り除いたもの（判定の一覧の行）、カバレッジの比較のコマンドと結果、壊して確かめたことを記す。

**完了の条件:** `make test`、`make lint`、`make deadcode` が通り、`coverage` が通る。

#### ステップ 2-3: `commandDocs` を `doctestutil.CommandDocs` に置き換える

**変更するファイル:** `cmd/yt2column/codex_review_docs_test.go`

-   [ ] 1.4 の「補助の置き換え」の表の `commandDocs` の行のとおりに置き換え、`commandDocs` と `commandsDir` を消す。
-   [ ] 置き換えの前後で、2 つの呼び出しが読むファイルの集合が同じであることを、一時的な比較で一度確かめる（コミットしない。設計書 3.4.2）。
-   [ ] 設計書 7.2 の 2 つの壊し方で、置き換えた後のテストが落ちることを確かめる。`TestPostingCommandsInventory` は `_lib/` の下の文書で、言及の確認の前に投稿のコマンドを置いた写しで落ちること。`TestCodexMentionOnlyInRequestRow` は、`_context.md` でないコマンドの文書に `| PR review request | ` で始まり言及を含む行を書き加えた写しで落ちること。
-   [ ] 置き換えの前後の出力を `coverage` で比べる。
-   [ ] コミットメッセージに、集合の一致、壊して確かめたこと、カバレッジの比較の結果を記す。

**完了の条件:** `make test`、`make lint`、`make deadcode` が通り、`coverage` が通る。

#### ステップ 2-4: 記述を直し、PR-2 の確かめを実行する

**変更するファイル:** `CLAUDE.md`、本書（記録）

-   [ ] `CLAUDE.md`「Development Notes」の text-sync guards の一覧の「the configuration block and Quick start links of both READMEs」から「Quick start links」を外す（設計書 3.7。`db9986c6` の `:531-532`。PR-1 で行がずれるので、文で探す）。「the configuration block」は残す。
-   [ ] `scope -pr 2`・`tests` を PR の merge-base と head で実行して通ることを確かめ、出力と 2 つのコミットを 7 章の該当の行に記録する。PR-3 で取り除く名前は PR-2 の head に残っているのが正しいので、`tests` の残存の検査の対象にしない（1.6）。`tests` が一覧として出すコメントと文書の中の名前は、PR のレビューで読むものとして PR の説明に載せる（AC-20）。
-   [ ] PR の説明に、設計書 3.1.1〜3.1.3 の判定の一覧の行（AC-04）、3.3 の K4・K5 の対応表のうち PR-2 が触れる行（AC-02）、ステップ 2-2・2-3 のカバレッジの比較の結果（AC-11）を載せる。

**フェーズ 2 の完了の条件:** `make test`、`make lint`、`make deadcode`、`make ext-check` が通る（AC-14）。`scope`・`tests` の実行の記録が本書にある。

### PR-2 作成ポイント: prune cmd/yt2column document tests

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4

**推奨タイトル**: `test(0052): cmd/yt2column の文書のテストから古い仕様・一度きり・過剰な確認を取り除く`

**レビュー観点**: PR の説明の判定の一覧の行と、取り除いた確認が一致すること / 一部を削除したテストで、残す確認の範囲と値の出どころが変わっていないこと / `commandDocs` の置き換えでパスの比較が同じ文書を指し、K5 の `TestPostingCommandsInventory` が壊した写しで落ちること / カバレッジの比較の記録と `tests` の出力

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 2-3 は、K5（エージェントの信頼の境界）のテストの入力の作り方を変える、独立した高リスクのステップである

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: ほかのパッケージと拡張の取り除き（設計書 8 章の PR-3）

フェーズ 3 のコミットは、取り除き、補助の置き換え、記述の直しの 3 つに分ける。

#### ステップ 3-1: 最後の調べ直しと、タスク 0050 の扱い

**変更するファイル:** 本書（記録）。0050 の文書は、下の条件に当たる場合だけ変える。

-   [ ] `main` の HEAD のすべての文書のテストについて、AC-05 の調べをやり直す（設計書 8 章の PR-3）。設計書 3.1 の冒頭の方法（`doctestutil` を使う、または `.md` のパスを持つテストのファイルを `git grep` で挙げ、一覧の外のものを読む）で、一覧の外に文書を読むテストが加わっていないかを調べる。加わっていれば基準を当てはめ、新しい判断の候補はレビュアーに諮り、決定を日付とともに本書の `Comments` に記録する（AC-07）。
-   [ ] 設計書 3.5 の 3 つの方法で、実装中の計画を調べる。
-   [ ] タスク 0050 の状態を調べ、設計書 3.5 の「0050 の扱い」のどれに当たるかを決めて記録する。0050 の文書を変える場合は、先にその担当（レビュアー）に確かめる。

**完了の条件:** 調べたコミット、結果、0050 の扱いが本書に記録されている。

#### ステップ 3-2: ほかのパッケージと拡張の確認・テスト・補助を取り除く

**変更するファイル:** `cmd/fixprguard/docs_test.go`、`cmd/codexreview/docs_test.go`、`internal/pipeline/pipeline_test.go`、`extension/test/metadataDocs.test.ts`

-   [ ] 取り除く前に、`coverage` に渡す出力を取る。
-   [ ] 1.4 の「一部を削除するテスト」の表の、`TestGuideNamesGitStateParts`・`TestInstallTargetDocumented` の 2 行のとおりに変える。`TestInstallTargetDocumented` の名前を `TestInstallTargetInMakefile` に変える（1.5）。
-   [ ] `TestInterfaceDocComments`・`interfaceDoc`・`hasCJK` を消す（J-11）。
-   [ ] `metadataDocs.test.ts` の「socket directory documentation reference」の `describe`（`:64-78`）を消す。ステップ 3-1 で、0050 がすでに消していた場合は変えず、その旨を記録する（設計書 3.5）。
-   [ ] 一部を削除した 2 つのテストについて、残す確認の値を一度壊して落ちることを確かめる（設計書 7.2）。`TestGuideNamesGitStateParts` は 8 章の行から `GitStateParts` の値を 1 つ消した写しで、`TestInstallTargetInMakefile` は `Makefile` の `install-codexreview:` の次の行を変えた写しで落ちること。
-   [ ] 取り除いた後の出力を取り、`coverage` で比べる。
-   [ ] `make ext-check` を通す。取り除いた node の確認が `extension/src/` の関数を通らないこと（設計書 3.1.7）を、PR の説明に記す（AC-13）。
-   [ ] 1 つのコミットにし、コミットメッセージに、取り除いたもの、名前の変更、カバレッジの比較の結果、壊して確かめたことを記す。

**完了の条件:** `make test`、`make lint`、`make deadcode`、`make ext-check` が通り、`coverage` が通る。

#### ステップ 3-3: `readRepoFile`・文書の読み込み・`markdownSections` を置き換える

**変更するファイル:** `cmd/fixprguard/docs_test.go`、`internal/pipeline/pipeline_test.go`

-   [ ] 1.4 の「補助の置き換え」の表の、`readRepoFile`・`os.ReadFile`・`markdownSections` の 3 行のとおりに置き換え、`markdownSections` を消す。
-   [ ] 置き換えた各呼び出しが、置き換えの前後で同じ文字列を返すことを、一時的な比較で一度確かめる（コミットしない。設計書 3.4.2）。
-   [ ] 置き換えを受けるテストについて、確かめる値を一度壊して落ちることを確かめる（設計書 7.2）。対象は、`TestPackageReferenceListsPackages`・`TestPromptsREADMEMatchesContract` と、`readRepoFile` を呼ぶ `cmd/fixprguard` のすべての文書のテストである。後者は `db9986c6` で次の 9 つである。`TestGuideCoversStopKindsAndScope`・`TestDocumentedInvocationsParse`・`TestGuideDescribesReviewerList`・`TestDocumentedProtocolMatches`・`TestGuideDescribesMentionCheck`・`TestGuideListsWatchList`・`TestGuideNamesGitStateParts`・`TestGuideDescribesIgnoredFiles`、および `guideBlock`（`:509`）を通る `TestGuideIgnoredCommands`。
-   [ ] 置き換えの前後の出力を `coverage` で比べる。
-   [ ] コミットメッセージに、文字列の一致、壊して確かめたこと、カバレッジの比較の結果を記す。

**完了の条件:** `make test`、`make lint`、`make deadcode` が通り、`coverage` が通る。

#### ステップ 3-4: 記述を直し、PR-3 の確かめを実行する

**変更するファイル:** `CLAUDE.md`、本書（記録）。ステップ 3-1 で決めた場合は 0050 の文書。

-   [ ] `CLAUDE.md`「Development Notes」の「the `codexreview` permission, install target, and command-document checks」から「install target」を外す（設計書 3.7。`db9986c6` の `:533-534`。文で探す）。
-   [ ] ステップ 3-1 で 0050 の文書を変えると決めた場合は、設計書 3.5 のとおりに変え、`Comments` に日付・`(decision)`・本タスクで改めたこと・改めた節を記録する（AC-16）。
-   [ ] `extension/scripts/has-extension-changes.sh`（`:2-11`・`:19`）、`extension/test/ciChanges.test.ts`（`:21-32`・`:62-89`）、`CLAUDE.md`「Development Notes」の node のテストが読む文書の説明（`db9986c6` の `:521-526`）が、取り除いた後の node のテストが読む文書と一致することを読んで確かめ、PR の説明に記す（設計書 2.2、AC-21）。
-   [ ] `scope -pr 3`・`tests -final` を PR の merge-base と head で、`guards` を `-final` を付けて実行し、通ることを確かめ、出力と 2 つのコミットを 7 章の該当の行に記録する。`tests -final` は、設計書 3.4.1・3.4.2 の全削除対象が残っていないことを確かめる最後の実行でもある（AC-09・AC-20。1.6）。
-   [ ] PR の説明に、設計書 3.1.4〜3.1.7 の判定の一覧の行、3.3 の K4・K5 の対応表のうち PR-3 が触れる行、ステップ 3-2・3-3 のカバレッジの比較の結果を載せる。

**フェーズ 3 の完了の条件:** `make test`、`make lint`、`make deadcode`、`make ext-check` が通る（AC-14）。`scope`・`tests`・`guards` の実行の記録が本書にある。PR-3 をマージする直前に、設計書 3.5 の実装中の計画の調べをもう一度行い、結果を記録した。

### PR-3 作成ポイント: prune other packages and extension document tests

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4

**推奨タイトル**: `test(0052): fixprguard・codexreview・pipeline・拡張の文書のテストから過剰な確認を取り除く`

**レビュー観点**: 最後の調べ直しと 0050 の扱いの記録 / `readRepoFile`・`markdownSections` の置き換えで、K1・K3・K5 の確認が読む範囲が変わらないこと / `TestInstallTargetInMakefile` の名前と中身、`CLAUDE.md` の「install target」の削除 / 拡張の CI ジョブの起動条件と `guards` の `-final` の結果

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-3 は `cmd/fixprguard` の K5 のテストの入力の作り方を変える、独立した高リスクのステップであり、ステップ 3-1 は判断の候補と別のタスクの文書の扱いを決める

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: 方針とガードのコマンド | `CLAUDE.md` の方針、2 か所のコマンド、`scope`・`guards` | PR-1 がマージされた |
| M2: `cmd/yt2column` | 取り除き、`commandDocs` の置き換え、`coverage`・`tests` | PR-2 がマージされた |
| M3: ほかのパッケージと拡張 | 取り除き、`readRepoFile`・`markdownSections` の置き換え、最後の調べ直し | PR-3 がマージされた |

順序を固定する理由は設計書 8 章のとおりである。

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 | 調べ直し、`scope`・`guards`、`CLAUDE.md` の方針、2 か所のガードのコマンド | standard |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 | `coverage`・`tests`、`cmd/yt2column` の取り除きと補助の削除、`commandDocs` の置き換え、「Quick start links」 | frontier-recommended |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 | 最後の調べ直しと 0050 の扱い、ほかのパッケージと拡張の取り除き、2 つの置き換え、「install target」 | frontier-recommended |

## 4. テスト戦略 (Test Strategy)

設計書 7 章のとおりである。本タスクは新しいテストを加えない。確かめは次の 3 つで行う。

-   **カバレッジの比較:** 取り除きと置き換えのコミットごとに、製品のコードの関数の結果が変わらないことを `coverage` で確かめる（設計書 7.1）。
-   **残す確認が落ちうること:** 一部を削除するテストと置き換えを受けるテストで、値を壊して落ちることを確かめる（設計書 7.2。ステップ 2-2・2-3・3-2・3-3）。
-   **変えないものが変わっていないこと:** `tests` で本書で変えると決めていない宣言が同じであることを、`scope` で許したファイルの外が変わっていないことを確かめる。

統合テストと実際の外部サービスは使わない。後方互換は、製品のコードを変えないこと（`scope`）と、カバレッジの比較で確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| 置き換えの誤りで、残す確認の読む範囲が変わる（設計書 4.3） | 確認が誤って通る、または関係ない変更で落ちる | 置き換えを別のコミットにし、前後で同じ文字列・ファイルの集合を返すことを一時的な比較で確かめ、値を壊して落ちることを確かめる（ステップ 2-3・3-3） |
| 一部の削除で、残す確認まで消える | 守っていた値の食い違いに気づけなくなる | 残す確認を壊して落ちることを確かめる（ステップ 2-2・3-2）。`tests` が、変えると決めていない宣言の変化を止める |
| 取り除いたテストが製品のコードの唯一の通り道だった | カバレッジが狭まる | `coverage` で比べ、違えば設計書 6.1 の流れで製品のコードを通る部分を残す（AC-12） |
| 設計の後に、文書を読むテストや実装中の計画が加わる | 一覧から漏れる、または別のタスクと同じ確認を変える | 各 PR の始めに調べ直す（ステップ 1-1・2-1・3-1）。PR-3 で全体をやり直し、マージの直前にも実装中の計画を調べる |
| タスク 0050 の PR #595 が、取り除く確認を保つ形でマージされる | 0050 の文書と本タスクの判定が食い違う | ステップ 3-1 で状態を調べ、設計書 3.5 の扱いに従う。0050 の文書を変える前に担当に確かめる |
| PR-2 と PR-3 が `CLAUDE.md` の同じ文を変える | 衝突 | PR の順を固定する（設計書 8 章）。前の PR のマージの後に次の PR を始める |
| カバレッジの比較が時間に依存して揺れる | 誤って変化とみなす、または見逃す | 設計書 7.1 の見分け方に従い、揺れとみなした関数の名前を記録に挙げる |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4）

**横断の確認（`make lint` と `make test` では検出できないもの）:**

-   取り除いた名前の残りは、Go のファイルについては `tests` が確かめる（7 章の AC-09・AC-20）。`tests` の残存の検査は、比べる merge-base と head の間で取り除いた名前を対象とし、後の PR で取り除く名前が head に残っていることは失敗としない。全削除対象の不在は、`-final` を付けた PR-3 の実行で確かめる。文書については、`tests` が出す一覧を PR のレビューで読む。
-   公開されたロケータ（`doctestutil.Readme*Heading`）と、単体テストからだけ呼ばれる補助は、`make deadcode` と `make lint` が報告しない（設計書 3.4.1 の「検出の限り」）。そのため、`tests` が残存を検査する名前の一覧に、設計書 3.4.1・3.4.2 の表で取り除くすべての補助（`commandDocs`・`commandsDir`・`markdownSections` を含む）を入れ、PR-3 の `tests -final` でそのすべての不在を確かめる。

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

`_checks/` のスクリプトは `docs/tasks/0052_doc_test_pruning/_checks/<name>/` を指す。実行の記録（出力と 2 つのコミット）は、実装の際に各行に加える。

| AC | 種類 | 確かめ方 | 実装 |
|---|---|---|---|
| AC-01 | diff・manual | diff: `tests`（変えると決めていない宣言が同じ）。manual: PR の説明の判定の一覧と、変えたテストの差分を読む | ステップ 2-2・2-3・3-2・3-3 |
| AC-02 | diff・manual | diff: `tests`（K4・K5 の確認を持つテストとその補助は、置き換えを受けるもの以外が同じ）。置き換えを受けるテストは、壊して確かめた記録で確かめる。`TestCodexMentionOnlyInRequestRow`・`TestPostingCommandsInventory` はステップ 2-3 の記録、`readRepoFile` の置き換えを受ける `cmd/fixprguard` の K5 のテストはステップ 3-3 の記録である。manual: PR の説明の設計書 3.3 の対応表 | ステップ 2-3・2-4・3-3・3-4 |
| AC-03 | diff | `scope`（`cmd/yt2column/readme_sync_test.go` が変わったファイルにない） | 全 PR（PR-1 の実行: 7.1） |
| AC-04 | manual | 設計書 3.1 の一覧と、各 PR の説明の該当の行を読む | ステップ 2-4・3-4 |
| AC-05 | manual | ステップ 1-1・2-1・3-1 の調べ直しの記録（調べたコミットを含む）を読む | ステップ 1-1・2-1・3-1 |
| AC-06 | manual | 設計書 3.2 の判断の一覧を読む | -（設計書） |
| AC-07 | diff・manual | diff: `tests`（取り除いた宣言の一覧を出力する）。manual: その一覧を設計書 3.2 の「取り除く」の行と、本書の `Comments` の新しい決定と照らして読む | ステップ 2-2・3-1・3-2 |
| AC-08 | manual | 設計書 3.1 の D5・D6 の行の仕様とタスクの番号を読む | -（設計書） |
| AC-09 | static・diff | static: `make deadcode`・`make lint`。diff: `tests`（各 PR は、その PR が取り除いた補助の名前が `docs/tasks/` の外の Go のファイルに識別子として残らないことを確かめ、PR-3 の `tests -final` で設計書 3.4.1・3.4.2 の全削除対象の不在を確かめる） | ステップ 2-2・3-2・3-4 |
| AC-10 | diff | `tests`（`TestNumberedItems` が補助と一緒に消え、残る補助の単体テストは変わらない） | ステップ 2-2 |
| AC-11 | diff | `coverage`（取り除きと置き換えのコミットごと）。結果をコミットメッセージと PR の説明に記す | ステップ 2-2・2-3・3-2・3-3 |
| AC-12 | diff・manual | diff: `coverage`。結果が違ったときは、設計書 6.1 の流れで残した部分と例外を、PR の説明に載せた判定の一覧の行（例外として記したもの）で読む | ステップ 2-2・3-2 |
| AC-13 | static・manual | static: `make ext-check`。manual: PR-3 の説明の、取り除いた確認が `extension/src/` の関数を通らないことの記述（設計書 3.1.7） | ステップ 3-2 |
| AC-14 | static | `make test`・`make lint`・`make deadcode`・`make ext-check`（各 PR） | 全 PR |
| AC-15 | diff | `scope`（各 PR） | 全 PR（PR-1 の実行: 7.1） |
| AC-16 | diff・manual | diff: `scope`（0050 の文書の変更を一覧として出力する）。manual: 0050 の文書を変えた場合に、その `Comments` を読む | ステップ 3-1・3-4 |
| AC-17 | diff | `scope`（`docs/tasks/` の下で 0052 と 0050 の外が変わらない） | 全 PR（PR-1 の実行: 7.1） |
| AC-18 | manual | `CLAUDE.md`「Development Notes」の一覧を、取り除いた後のテストと照らして読む | ステップ 2-4・3-4 |
| AC-19 | static | `guards`（PR-1 は 4 か所の一致、PR-3 は `-final` で文書のテストが残るパッケージとの一致） | ステップ 1-4・3-4（PR-1 の実行: 7.1） |
| AC-20 | diff・manual | diff: `tests`（各 PR は、その PR が取り除いた名前が `docs/tasks/` の外の Go のファイルに識別子として残らないことを確かめ、PR-3 の `tests -final` で全削除対象の不在を確かめる）。manual: `tests` が出すコメントと文書の中の名前の一覧と、一部を削除したテストの doc comment を読む | ステップ 2-2・2-4・3-2・3-4 |
| AC-21 | diff・static・manual | diff: `scope`（`extension/scripts/has-extension-changes.sh`・`extension/test/ciChanges.test.ts`・`.github/workflows/`・`Makefile`・`go.mod` が変わらない）。static: `make ext-check`（`ciChanges.test.ts`）。manual: ステップ 3-4 の読み合わせの記述 | ステップ 3-4（`scope` の PR-1 の実行: 7.1） |
| AC-22 | manual | `CLAUDE.md` の加えた文を読む | ステップ 1-3 |
| AC-23 | manual | 同上 | ステップ 1-3 |
| AC-24 | diff・manual | diff: `scope`（項の既存の文が残る）。manual: K4・K5 の例外の理由が残ることを読む | ステップ 1-3（PR-1 の実行: 7.1） |

### 7.1. PR-1 の一度きりの確かめの実行の記録（2026-11-01）

-   **`scope`:** `go run -tags test ./docs/tasks/0052_doc_test_pruning/_checks/scope/ -pr 1 b467905e 1eac7553`。比べたコミットは merge-base `b467905e`（`origin/main`）と head `1eac7553`。出力:

    ```
    pull request 1
    merge-base   b467905e8c87cf46a97d42bf884f6e0e18719c26
    head         1eac7553b64249befc5a397ef4783bfa9f821974
    ---- changed files
    OK   .claude/commands/_lib/task-pr-handoff.md
    OK   CLAUDE.md
    OK   docs/dev/guides/fixprguard_guide.md
    OK   docs/tasks/0052_doc_test_pruning/03_implementation_plan.md
    OK   docs/tasks/0052_doc_test_pruning/_checks/guards/main.go
    OK   docs/tasks/0052_doc_test_pruning/_checks/scope/main.go
    OK   every changed file is in the pull request's change set
    OK   every sentence of the CLAUDE.md policy section is still there
    ```

    この記録を加えるコミットと、その後のレビューの修正のコミットは、`scope` が許すファイルだけを変える。そのため PR の head を比べても上の一覧は同じになる（PR の説明に、head での実行結果を記す）。

    失敗とする条件は、それぞれ一時的なコミットを作って確かめた（確かめた後に `git reset --hard` で戻し、ブランチには残っていない）。

    -   許さないファイル（`README.md`）を変えた実行と、PR-1 で 0050 の文書を変えた実行は、どちらも `NG files outside the pull request's change set` で終了コード 1 になった。同じ 0050 の変更は PR-3 では `LIST` として通り、終了コード 0 になった。
    -   `CLAUDE.md` の項の既存の文 3 つを消した実行は `NG CLAUDE.md policy section: 3 sentence(s) lost` で、項の見出しの文を書き換えて項自体をなくした実行は同じ検査の `the section itself is gone` で、どちらも終了コード 1 になった。既存の文に `Usually, ` を前置した実行も、文を単位に比べるので `NG CLAUDE.md policy section: 1 sentence(s) lost` で終了コード 1 になった。
    -   `README.md` を `docs/tasks/0052_doc_test_pruning/_checks/moved/README.md` へ `git mv` した実行は、`--no-renames` により移した先（許すパス）と元のパスの両方を一覧に出し、`NG README.md` で終了コード 1 になった。

-   **`guards`:** `go run -tags test ./docs/tasks/0052_doc_test_pruning/_checks/guards/ 1eac7553`。出力:

    ```
    commit 1eac7553b64249befc5a397ef4783bfa9f821974
    ---- commands
    OK   .github/workflows/ci.yml                 doc-guard job: ./cmd/codexreview ./cmd/fixprguard ./cmd/yt2column ./cmd/yt2column-server ./internal/pipeline
    OK   CLAUDE.md                                Development Notes: ./cmd/codexreview ./cmd/fixprguard ./cmd/yt2column ./cmd/yt2column-server ./internal/pipeline
    OK   .claude/commands/_lib/task-pr-handoff.md CHECKS: ./cmd/codexreview ./cmd/fixprguard ./cmd/yt2column ./cmd/yt2column-server ./internal/pipeline
    OK   docs/dev/guides/fixprguard_guide.md      automated reviewer list procedure: ./cmd/codexreview ./cmd/fixprguard ./cmd/yt2column ./cmd/yt2column-server ./internal/pipeline
    ---- comparison
    OK   the four places name the same 5 package(s): ./cmd/codexreview ./cmd/fixprguard ./cmd/yt2column ./cmd/yt2column-server ./internal/pipeline
    ```

    失敗とする条件も、一時的なコミットで確かめた。

    -   ステップ 1-4 の前の `b467905e` に対しては、`task-pr-handoff.md` と `fixprguard_guide.md` が `./cmd/codexreview` を欠くので、`NG ... differs from .github/workflows/ci.yml` で終了コード 1 になった。
    -   `task-pr-handoff.md` に 2 つ目のコマンドを書き加えた実行は `NG ... CHECKS: 2 documentation guard commands` で、コマンドを `./cmd/yt2column` を挙げない形に変えた実行は `NG ... CHECKS: no documentation guard command` で、どちらも終了コード 1 になった。

-   **`guards -final`:** PR-3 で使う。`1eac7553` に対しては、テストが `internal/doctestutil` を import するパッケージを `./cmd/codexreview`・`./cmd/fixprguard`・`./cmd/yt2column`・`./cmd/yt2column-server` と数え、`./internal/pipeline` を欠くので、`NG the guard commands do not name the packages whose tests read documents through doctestutil` で終了コード 1 になった。`internal/pipeline` は PR-3 の置き換え（設計書 3.4.2）で `doctestutil` を import するので、`-final` は PR-3 で通る。

-   **AC-14（`1eac7553`）:** `make test`、`make lint`、`make deadcode`、`make ext-check` が通った。`make ext-check` は `extension/.node-version` のとおり Node.js 24.21.0 で実行した（手元の既定の Node.js 26.12.0 は `Makefile` の版の確認で止まる）。node のテストは 975 件が通り、失敗はない。

## 8. 成功基準 (Success Criteria)

-   **機能:** 設計書 3.1 の判定の一覧のとおりに取り除かれ、AC-01〜AC-24 が 7 章の確かめ方で満たされている。
-   **品質:** 各 PR で `make test`・`make lint`・`make deadcode`・`make ext-check` が通る。取り除きと置き換えのコミットごとに、製品のコードのカバレッジが変わらないことを記録している。一部を削除したテストと置き換えを受けたテストは、壊して落ちることを確かめ、コミットメッセージに記録している。
-   **セキュリティ:** K4・K5 の確認を 1 つも取り除いていない（`tests`、設計書 3.3）。置き換えを受けた K5 のテストは、壊した写しで落ちることを確かめている。
-   **文書:** 7 章の `manual` の行を、該当する PR のレビューで読んでいる。`_checks/` の実行の記録が本書にある。

## 9. 次のステップ (Next Steps)

-   本書の PR をレビューし、`/runpr` で承認とマージを行う。
-   承認の後、PR-1 を `/runplan 0052` で実装する。
-   PR-1 のマージの後に PR-2 を、PR-2 のマージの後に PR-3 を、それぞれ `/runplan 0052` で実装する。
-   PR-3 のマージの後、issue #591 を閉じる（`/runpr` が提案する）。
-   設計書 4.3 の範囲外の古い記述（`docs/dev/guides/fixprguard_guide.md:461` の「本書（日英）」）と、README の移行の節を残すかどうか（設計書 9 章）は、別の issue で扱う。`cmd/codexreview` の K5 の走査が `go test` の引数を呼び出しとみなす誤検出（1.3）も、別の issue で扱う。
