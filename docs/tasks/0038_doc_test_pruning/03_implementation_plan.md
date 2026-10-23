# 実装計画書：自然言語の意味や不要な文言を固定する文書のテストを削除する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-23 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）の F-001〜F-009 を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。設計書 8 章の PR-1〜PR-5 を、順にフェーズ 1〜5 とし、1 つのフェーズを 1 つの PR にする。

用語は、要件書 6 章と設計書の冒頭の定義に従う（文書のテスト、確認、判定、判定の一覧、補助、補助の単体テスト、削除の PR、AC の検証の行、カバレッジの比較、読み合わせ）。K1〜K5・D1〜D4 は要件書 F-001 の分類を指す。本書では、さらに次の語を使う。

-   **検査のスクリプト:** フェーズ 1 で加える `docs/tasks/0038_doc_test_pruning/check_pruning.sh`。次の 5 種類の確認を、サブコマンドとして持つ。名前の一覧の照合（`removed`・`kept`）、変えないファイルの確認（`unchanged`）、タスクの文書の記録の確認（`records`）、文書を読むテストの洗い出し（`survey`）、カバレッジの比較（`coverage`）である（1.4、ステップ 1-1）。
-   **名前の一覧:** 検査のスクリプトが読む `docs/tasks/0038_doc_test_pruning/pruned_names.txt`。削除する対象（テスト、補助、node のテストの題）と、残すテストを、1 行に 1 つずつ挙げる。各行は、名前、種類（Go の関数・変数・定数・型、node のテストの題）、置き場所（ファイル）、`docs/tasks/` 以外の文書でも探すか、担当の PR、削除か残すか、を持つ。PR-1 で、設計書 3.1・3.2 と本書 1.3.2 から全体を書き、各削除の PR は、作業の始めに数え直して見つかったテストだけを書き足す。

### 1.2. 実装原則

-   **判定は設計書 3.1 に従う。** 本書は判定を言い換えない。設計書の後に `main` に入ったテストの判定だけを 1.3.2 に記す。
-   **各削除の PR の始めと、PR を作る直前に、前提とテストの数を確かめ直す**（設計書 3.4 の「確かめる時点」）。本書の `file:line` は `28cb00c` のものである。行は、テスト名と見出しで探し直す。
-   **削除とその影響は同じ PR で直す**（設計書 1.1）。テストを削除する PR が、そのテストを挙げるタスクの文書と、そのテストを述べる文書も改める。
-   **コミットの単位は、テストのファイル（またはファイルの群）ごとにする。** Go のテストを削除する、または一部を削除するコミットごとに、検査のスクリプトでカバレッジを比べ、その結果をコミットメッセージに記す（AC-10）。中身を変えたテストは、残す確認を壊して落ちることを確かめ、そのこともコミットメッセージに記す（設計書 7.2）。
-   **Go と TypeScript のソースのコメント・識別子・文字列は英語で書く**（`CLAUDE.md`「Go Idioms」）。検査のスクリプトのコメントとメッセージも英語で書く。
-   **検査のスクリプトに対するレビューの指摘は、`/fixpr` に任せず、人が直す。** `/fixpr` の範囲の規則が、`docs/tasks/` の下の `.sh` を許さないからである（`internal/fixprguard/scope_test.go:146`）。名前の一覧は `.txt` にして、`/fixpr` でも直せるようにする。

### 1.3. 既存コード調査結果

調査はコミット `28cb00c`（本書を書き始めたときの `origin/main`）で行った。設計書が確かめた `80c17a8` の後に、次の 3 つのコミットが入った。

-   `f326ce6`（タスク 0037 の PR-1、#459）: `runpr.md`・`CLAUDE.md`・`requirements_process.md`・`codex_review_docs_test.go` を変えた。`runpr.md` の step 3 は `:211` から始まる。
-   `6022d4b`（タスク 0034 の PR-2、#461）: `cmd/fixprguard/docs_test.go` に 4 つのテストと補助を加え、`cmd/yt2column/fixpr_docs_test.go` の `TestFixprDocContract` にサブテスト `ignored_file_stops`（`:337`）を加え、用語集に 3 行を加えた。そのため、`fixpr_docs_test.go` の `:337` より後の行は 27 行ずれた（`TestRunprDocContract` は `:422`、`TestContextAndCommandsAgree` は `:823`、`TestContextReviewerRowsMalformed` は `:871`、`TestAuthorizationSections` は `:953`、`TestRemovedCommandLiterals` は `:975`、`TestGlossaryTerms` は `:1011`）。`cmd/fixprguard/docs_test.go` の既存のテストは 1 行ずれた（`TestGuideCoversStopKindsAndScope` は `:49`、`TestGuideDescribesConfigScope` は `:527`）。`fixprguard_guide`（日英）の 9 章は `:442` から始まり、自動レビュアーの一覧を変える手順は `:454-459`、`docs_test.go` を述べる行は `:461` になった。
-   `28cb00c`: 本タスクの設計書だけである。

ほかのファイル（`codex_review_docs_test.go`・`docs_test.go`・`output_layout_docs_test.go`・`copied_transcript_docs_test.go`・`extension_docs_test.go`・`cmd/yt2column-server/docs_test.go`・`relay_guard_test.go`）のテストの行は、設計書の行と同じである（`grep -n '^func '` で確かめた）。

#### 1.3.1. タスク 0034・0037 の進み具合

-   **タスク 0037:** 実装の PR（PR-1）はマージ済みで、残る作業はない。
-   **タスク 0034:** PR-1・PR-2 はマージ済みである。PR-3（フェーズ 8、0030 の文書）は未実装で、ステップ 8-4 は `TestTask0030DescribesIgnoredFiles`（D4）を加えると定めている（`docs/tasks/0034_fixprguard_ignored_files/03_implementation_plan.md:437`）。PR-3 は実装が進んでいるので、このテストを加えないよう、PR-3 の実装の側に伝える（1.4 の 1 つめ）。

#### 1.3.2. タスク 0034 が加えたテストの判定

設計書 3.4 の見込みを、`6022d4b` のコードで確かめ直した結果である。

| テスト（`28cb00c`） | 判定 | 残す確認 | 削除する確認 | 設計書 3.4 の見込みとの違い |
|---|---|---|---|---|
| `TestGuideListsWatchList`（`cmd/fixprguard/docs_test.go:718`） | 一部を削除する | 7.1 の表の code span が `fixprguard.WatchListPatterns` と等しいこと（K1。`**監視の一覧。**`・`**Watch list.**` は手がかりの語） | 7.1 の保証の行があること（`singleLine` の `tc.row`。D1） | なし |
| `TestGuideNamesGitStateParts`（`:744`） | 残す | 日英の 8 章の `git-state-changed` の行の `parts` の一覧と、`fixpr.md` の部分の一覧を、`fixprguard.GitStateParts`・`PartIgnored` と照合すること（K1。`` `parts`（ ``・`name the `parts` that differ from the baseline (` は手がかりの語） | - | 設計書 3.4 は、`fixpr.md` の部分の一覧の確認（計画 `:360`）を PR-3 の担当とした。実際にはこのテストの中にあり、`PartIgnored` から導いているので、PR-2 で残す |
| `TestGuideDescribesIgnoredFiles`（`:794`） | 一部を削除する | `fixprguard.IgnoredFailureRules` の各規則が `ignored` を含むか `.git name below .claude` であること（文書を読まない確認。変えない）、8.2 の表が `IgnoredFailureRules` のすべてを code span で挙げること（K1。表の列の見出しは表を探すための K2） | 7.2.3・7.2.5・8.1 の語句、`git-failed` の行の語句（D1）、8.2 の表の各行のセルが 3 つであること（1.4 の 6 つめ） | 見込みは「削除する（D1・D2）」だったが、K1 の確認と文書を読まない確認がある |
| `TestGuideIgnoredCommands`（`:923`） | 一部を削除する | 日本語のガイドの 8.1 のコードブロックを、`begin`・`inspect` が書いた記録に対して実行すること（K3） | 日英のコードブロックが同じであること（文書どうしの一致で、値の出どころを持たない） | なし。英語のガイドのブロックは実行されなくなる（7.1） |
| `TestFixprDocContract` の `ignored_file_stops`（`cmd/yt2column/fixpr_docs_test.go:337`） | サブテストごと削除する | - | 語句（D1）、Phase 4 が規則を 2 回参照すること（D1・D2） | なし。守っていた手順の文は読み合わせに任せる（7.1） |
| `TestGlossaryTerms` に加えた 3 つの組（`:1011`） | テストごと削除する（設計書 3.1.2） | - | 用語集の行（D2） | なし |
| `TestTask0030DescribesIgnoredFiles`（0034 の計画 `:437`） | 加えられない見込み。加えられていれば削除する（D4） | - | - | 1.4 の 1 つめ |

補助のうち、`codeSpans`（`:705`）・`guideBlock`（`:906`）・`singleLine` は残す確認が使うので残す。`requireAll`（`:773`）は `TestGuideDescribesIgnoredFiles` の削除する確認だけが使うので削除する。

#### 1.3.3. 変える箇所のうち、設計書の後に分かったもの

| 箇所（`28cb00c`） | 今 | 変え方 | ステップ |
|---|---|---|---|
| `docs/dev/developer_guide/requirements_process.md:278` | 5 章の確認項目「At least one test per acceptance criterion」 | `:239` と同じ例外を反映する（1.4 の 7 つめ） | 1-4 |
| `.claude/commands/weakreview.md:219-222` | AC の検証の節が、範囲が閉じるすべての AC に「real test locations (`path::TestName`) or real static commands」を挙げていることを確かめる項目 | 文書の自然言語の記述を定める AC は `manual` でよい、という例外を反映する（1.4 の 7 つめ） | 1-4 |
| `.claude/commands/mkplan.md:71`・`:106`、`requirements_process.md:227` | AC ごとに実行される検証（テスト、`make` のターゲット、スクリプト）を挙げることを求め、「document review」のような札を禁じる項目（`mkplan.md:71`）。panel mode の Reviewer B の役目の「every AC has at least one concrete test/static verification task」（`:106`）。AC を「the test that proves it」に対応させる項目（`requirements_process.md:227`） | 文書の自然言語の記述を定める AC は `manual`（読み合わせ）でよい、という例外を反映する（1.4 の 7 つめ） | 1-3・1-4 |
| `.claude/commands/mkplan.md:79` | 横断の検索の項目。「doc/glossary consistency」を含む項目を「Prefer converting such an item into a guard test」とする | 文書の自然言語の記述と用語集の一致は、文書のテストにせず読み合わせで確かめる、とする（1.4 の 7 つめ） | 1-3 |
| `.claude/commands/runpr.md:238`・`:287`・`:291` | step 3 が毎回実行する 3 つの一覧（`gh api --paginate ... --jq '`）。`TestRunprDocContract` がフィルタの全文を直書きで固定している（`fixpr_docs_test.go:99-137` の `reviewedCheck`・`mentionListings`、`:142` の `requestCommentFilter`） | 3 つのフィルタを `jq` で実行し、本文を出力しないことを確かめる形にする（1.4 の 2 つめ）。`:397`（`--jq .body`）と `:410`（先頭 500 文字）は、止まったときの報告のための読み込みなので対象にしない（設計書 1.2）。`:302` の `Date` の見出しを読むコマンドは `--jq` の一覧でないので、設計書 3.1.8 の対象にしない | 3-2 |
| `cmd/yt2column/docs_test.go:527` の `TestArtifactDocsContract` | 削除の後に残るのは `run -h` の出力の確認（`:555-564`）だけ。`run_test.go:1229` の `TestRunHelp` は、`-h`・`--help` の終了コード、空の標準エラー、使い方の文字列の一覧を確かめている | 下の名前の対応表のとおり、残る 2 つの文字列を `TestRunHelp` の一覧に移す（1.4 の 3 つめ） | 4-3 |
| `CLAUDE.md:302-316`（「Development Notes」） | node のテストが「the video-metadata statements in the README "Browser extension" section and security.md 8.1」を読むと述べ、`make test` に要るものとして `jq` を挙げていない | 設計書 3.6 の改め方に加え、PR-5 の後に README で残る確認（展開の案内の文）に合わせる。PR-3 で、`make test` が `jq` を要することを記す | 3-6・4-5・5-4 |

**名前の対応表**（本書のほかの箇所の名前は、この表に従う）:

| 今の名前と場所 | 移す先 | 名前を挙げる箇所と改め方 |
|---|---|---|
| `cmd/yt2column/docs_test.go::TestArtifactDocsContract`（`run -h` の部分） | `cmd/yt2column/run_test.go::TestRunHelp`（使い方の文字列の一覧に 2 つを加える） | タスク 0017 の計画 `:378`（AC-28）は、`TestRunHelp` をすでに挙げているので、`TestArtifactDocsContract` を取り除く（設計書 3.3.2 の規則 3）。`:383`（AC-33）・`:384`（AC-34）は、`TestArtifactDocsContract` の削除する確認を挙げているので、同じく規則 2・3 で改める。0017 の計画 `:258`・`:261`・`:269`、0019 の計画 `:288` は本文の作業の記録なので変えない（要件書 2.3）。本タスクの要件書・設計書は、本タスクの前の状態を述べるので変えない |

#### 1.3.4. 外部の前提の確かめ

-   **`jq`:** この環境の `/usr/bin/jq` は `jq-1.7.1-apple` である（2026-10-23）。今は、リポジトリのどのテストも `jq` のコマンドを実行しない。CI の `test` ジョブ（`make test-ci`）と `doc-guard` ジョブはどちらも `ubuntu-latest` で走るので、両方で `jq` があることを PR-3 の CI の実行で確かめる（ステップ 3-7）。
-   **`gh --jq` と `jq` の違い:** `gh` の `--jq` は、`gh` に組み込まれた gojq で評価する（`go version -m "$(command -v gh)"` に `github.com/itchyny/gojq v0.12.17` がある。2026-10-23）。テストは `jq` 1.7 で実行するので、2 つの実装で結果が違う構文（たとえば正規表現の方言）がフィルタに入れば、テストはその違いを検出しない（7.1 のリスク）。
-   **`go test -coverpkg`:** `go help testflag`（go1.27.1）は、`-coverpkg` を「Apply coverage analysis in each test to packages whose import paths match the patterns」と説明する。複数のパッケージで 1 つの `-coverprofile` を書く形は、`Makefile:88` の `test-ci` がすでに使っている。

#### 1.3.5. 変えない箇所

-   `docs/tasks/0032_japanese_pr_title/check_rules.sh`・`docs/tasks/0037_runpr_merge_recommendation/check_wording.sh` は、文書を読むが、`make test`・`make ext-test`・CI のどれも実行しない（`git grep -n "check_rules.sh\|check_wording.sh" -- '*.go' '*.ts' Makefile '.github/*'` の一致は、パスを文字列として使う `internal/fixprguard/scope_test.go:146` だけである）。要件書 6 章の「文書のテスト」はテストを指すので、これらは対象にしない。PR-5 の AC-05 の調べ（ステップ 5-5）で、この扱いを判定の一覧に記す。
-   `.github/workflows/ci.yml`、`cmd/yt2column-nmhost/main_test.go`、`internal/pipeline/pipeline_test.go`、`extension/test/serverReasons.test.ts`、取り下げたタスク 0029 の文書（設計書 2.3）。
-   `fixprguard_guide`（日英）9 章の `:454-459`（K4 の手順。`wantReviewers` と `TestContextReviewerRowsMalformed` は残る）と `:461`（残る K1 の確認を述べる）。PR-2 で、ほかに削除の対象を述べる箇所がないことを確かめる（設計書 3.6）。

### 1.4. 計画の際に決めたこと（2026-10-23）

レビュアーが決めたもの:

1.  **タスク 0034 の PR-3 には `TestTask0030DescribesIgnoredFiles` を加えさせない。** 作ってすぐ削除する手間を避けるためである（要件書 1 章）。PR-3 の実装が進んでいるので、実装の側には、理由（D4、要件書 F-006・AC-17）を添えて次の 5 点を伝える（伝えるプロンプトはレビュアーに渡した）。(a) このテストを加えない（加えていれば取り除く）。(b) 0034 の計画のステップ 8-4 を取り下げる。(c) AC-20・AC-20a の検証方法を `manual` に改める。(d) これと矛盾する `:123` の注記、ステップ 8-5 の確かめ、`:607` の成功基準を改める。(e) `Comments` に `(decision)` を記録する。本タスクは、PR-2 の始めに、これらが `main` に入ったことを確かめる（ステップ 2-1）。
2.  **step 3 の一覧のフィルタが本文を出力しないことは、`jq` で実行して確かめる**（設計書 3.1.8 で計画に委ねられた点）。フィルタを文字列として調べる方法では、要約のコメントの状態を本文の 2 行め（`$l[1]`）から導く部分を例外として扱う規則が要り、壊れやすい。`jq` がない環境では、`TestGuideIgnoredCommands` と同じく、飛ばさずに失敗させる。
3.  **`TestArtifactDocsContract` の残る部分は、`run_test.go` に移す**（設計書 3.1.4 で計画に委ねられた点）。削除の後は文書を読まないので、`docs_test.go` に置く理由がない。移す先は、同じ `run -h` を実行して使い方の文字列を確かめる `TestRunHelp` の一覧とする。別のテストにすると、同じ実行と確認を繰り返すことになるからである（1.3.3 の対応表）。
4.  **削除した名前の不在は、検査のスクリプトで確かめる**（AC-03・AC-06・AC-07・AC-22）。タスク 0037 の `check_wording.sh` にならって本タスクのディレクトリに置き、各削除の PR の終わりに実行して結果を記録する。`make test` と CI では実行しない。不在の確認を恒久のテストにすると、それ自体が D3 になるためである。

計画で決めたもの（レビュアーに諮らず、理由とともに記す）:

5.  **カバレッジの比較とタスクの文書の記録の確認も、検査のスクリプトで行う**（AC-10・AC-11・AC-14・AC-15）。比較の前後のどちらかの実行が失敗した結果を基準にしない、`-coverpkg=./...` を付ける、といった設計書 7.1 の条件を、コミットごとに手で守るのは誤りやすい。改めたタスクの文書の `Status` と `Comments` も、値なので機械で確かめられる。
6.  **`TestGuideDescribesIgnoredFiles` の 8.2 の表の各行のセルが 3 つであることの確認は削除する。** この確認は、セルの中の `|` でコマンドが切れて表示されることを防ぐためのものである（`cmd/fixprguard/docs_test.go` のコメント）。K2（見出しのレベル、見出し、表の列の見出し）には当たらず、K1 の確認（規則の名前を code span から読む）の前提でもない。
7.  **要件書と設計書が挙げなかった、同じ規則の箇所も PR-1 で改める。** `requirements_process.md:227`・`:278`、`weakreview.md:219-222`、`mkplan.md:71`・`:79`・`:106` は、要件書 1 章が挙げた箇所と同じく、自然言語の記述にも `test` か `static` の検証や文書のテストを求める（1.3.3）。残すと、AC-24 の改訂と矛盾する。決定を変えず、同じ決定の及ぶ箇所を足すだけなので、設計書は改めない。
8.  **1 つのフェーズを 1 つの PR にし、設計書 8 章の順に作る。** PR-2〜PR-5 の間に順序の依存はない（設計書 3.8 の H-03）が、各 PR が `CLAUDE.md` やタスクの文書の同じ箇所を順に改めるので、本書の番号の順に作る。検査のスクリプトは、PR-2〜PR-5 が使うので PR-1 に入れる。PR-3 は、`jq` の確認を加えるコミットを、フィルタの定数を削除するコミットより先に置き、`TestRunprDocContract` の作り直しを 1 つのコミットで読めるようにする。

### 1.5. 申し送りの項目への対応

`implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-08 は、設計書 3.8 にすべて記録されている。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 規則と方針（設計書 8 章の PR-1）

#### ステップ 1-1: 検査のスクリプトと名前の一覧

**変更するファイル**: `docs/tasks/0038_doc_test_pruning/check_pruning.sh`（新規）、`docs/tasks/0038_doc_test_pruning/pruned_names.txt`（新規）

-   [ ] `pruned_names.txt` を、設計書 3.1・3.2 と本書 1.3.2 の全体から書く（1.1 の名前の一覧の各欄）。`docs/tasks/` 以外の文書でも探すのは、テストの名前と、ほかの語と紛れない補助の名前だけにする。node のテストの確認は、題（`it("...")` の文字列）で挙げる。
-   [ ] POSIX `sh` で、次のサブコマンドを持つスクリプトを書く。終了コードは、確認が通れば `0`、通らなければ `1`、使い方の誤り・知らないリビジョン・`git` の失敗は `2` とする。既定は作業ツリーを読み、`CHECK_REV` でリビジョンを選べる形は `check_wording.sh` にならう。
    -   `removed <PR>`: その PR までに削除する行の名前が、置き場所のファイルに残っていないこと（Go は宣言、node は題。AC-03・AC-06・AC-07）。また、`docs/tasks/` 以外の文書でも探す行の名前が、その文書（`CLAUDE.md`・`README.md`・`AGENTS.md`・`docs/dev/`・`.claude/commands/`）に現れないこと（AC-22）。
    -   `kept`: 残す行のテストが、置き場所のファイルに宣言されていること（AC-01・AC-19）。
    -   `unchanged <base>`: 取り下げたタスク 0029 の文書と、`.github/workflows/ci.yml` の `doc-guard` ジョブの `go test` の行が、`<base>`（その PR の前の `origin/main`）から変わっていないこと（AC-16・AC-21）。ほかのタスクが `ci.yml` のほかの行を変えても失敗しない。
    -   `records <base>`: `<base>` から変わった `docs/tasks/` のほかのタスクの `02_architecture.md`・`03_implementation_plan.md` が、`Status` を `approved` のままとし、`Comments` に `(decision)` と `0038` を含むこと（AC-14）。タスク 0035 の計画 5 章と設計書 7.5 の AC-18 の行が `manual` であること（AC-15）。
    -   `survey`: `_test.go`、`//go:build test` のファイル、`extension/test/` のうち、文書のパスや名前（`.md`、`docs/`、`.claude/`、`README`、`CLAUDE`、`AGENTS`、`prompts/`）を含むファイルを挙げること（AC-05 の調べの元）。
    -   `coverage <before> <after>`: 2 つのリビジョンを SHA に解決し、一時ディレクトリの `--detach` の作業ツリーで設計書 7.1 のコマンドで測り、`go tool cover -func` の差分を出すこと。どちらかの `go test` が失敗したら、差分を出さずに `2` で終える。成功しても失敗しても、作った作業ツリーを取り除く（AC-10・AC-11）。
-   [ ] `28cb00c` に対して、`removed 1`・`kept`・`unchanged 28cb00c` が通り、`coverage 28cb00c 28cb00c` の差分が空であることを確かめる。`coverage` の自己比較は 2 回行う。実行ごとに揺れる関数があれば、比較から外さずに止まり、7.1 のリスクの対策をとる。
-   [ ] 壊して確かめる（結果をコミットメッセージに記す）:
    -   `removed`: 削除の行に、今あるテストの名前を書いた写しで `1`。
    -   `kept`: 残す行に、ない名前を書いた写しで `1`。
    -   `unchanged`: `doc-guard` の `go test` の行を変えた作業ツリーで `1`、`ci.yml` のほかの行を変えた作業ツリーで `0`。
    -   `records`: `Comments` に `0038` のない文書の変更で `1`。
    -   `coverage`: ほかのテストが通らない関数を通る唯一のテストを消したコミットで差分が出ること。失敗するテストを入れたコミットで、差分を出さずに `2`。知らないリビジョンで `2`。各実行の後に `git worktree list` に作業ツリーが残らないこと。

**完了条件**: 上の確かめが済み、`survey` の出力が要件書 1 章のファイルの一覧を含む。

#### ステップ 1-2: `CLAUDE.md`「Testing Strategy」

**変更するファイル**: `CLAUDE.md`

-   [ ] 「Testing Strategy」（`CLAUDE.md:267-`）に、文書のテストについての一項を英語で加える（設計書 3.5、AC-23）。K1〜K3 は確かめてよいもの、D1〜D4 は確かめないもの、自然言語の記述は PR のレビューでの読み合わせで確かめること、K4・K5 の例外とその理由（`/fixpr`・`/runpr` の信頼の境界）を述べる。

**完了条件**: 項が加わり、`make test` が通る。

#### ステップ 1-3: `mkplan.md`

**変更するファイル**: `.claude/commands/mkplan.md`

-   [ ] `:74` の「Every AC must have at least one `test` or `static` entry」に、文書の自然言語の記述を定める AC は `manual` だけでよい、という例外を加える。それ以外の AC には、今までどおり `test` か `static` を求める（AC-24・AC-25）。
-   [ ] `:71` の、AC ごとに実行される検証を挙げる項目と、`:106` の Reviewer B の役目に、同じ例外を反映する（1.3.3）。
-   [ ] `:79` の横断の検索の項目の「doc/glossary consistency」を、文書のテストにせず読み合わせで確かめるものにする（1.3.3）。
-   [ ] `:88` の「Documentation-only ACs」の項目を、自然言語の記述は読み合わせ（`manual`）、値と構造は K1〜K3 の文書のテストで確かめる、という形に改める（AC-24）。`rg` で語の有無を確かめる作業を求める例を除く。
-   [ ] `:122` の確認項目に、同じ例外を反映する。

**完了条件**: 6 か所が改まり、`make test` が通る。

#### ステップ 1-4: `runplan.md`・`weakreview.md`・`requirements_process.md`

**変更するファイル**: `.claude/commands/runplan.md`、`.claude/commands/weakreview.md`、`docs/dev/developer_guide/requirements_process.md`

-   [ ] `runplan.md` の `:78`・`:158`・`:213` に、ステップ 1-3 と同じ例外を反映する（AC-24・AC-25）。
-   [ ] `weakreview.md` の `:219-222` に、同じ例外を反映する（1.3.3）。
-   [ ] `requirements_process.md` の `:227`・`:239`・`:278` に、同じ例外を反映する（AC-24・AC-25）。

**完了条件**: 7 か所が改まり、`make test` が通る。

#### ステップ 1-5: PR-1 の確かめ

**変更するファイル**: なし

-   [ ] `make test && make lint` を実行する。`.claude/commands/` と `CLAUDE.md` を読むテスト（`TestCodexMentionOnlyInRequestRow`・`TestPostingCommandsInventory` など）が、加えた文を受け入れることも、これで確かめる。
-   [ ] `.claude/commands/`・`docs/dev/`・`CLAUDE.md` を、AC ごとに `test` か `static` の検証を求める言い回しで検索し、ステップ 1-3・1-4 で改めていない箇所がないことを確かめる。
-   [ ] `pruned_names.txt` の行を、設計書 3.1・3.2 と本書 1.3.2 の表と 1 行ずつ突き合わせ、漏れと余りがないことを確かめる。
-   [ ] ステップ 1-2〜1-4 の差分に、要件書 F-001 の K1〜K5・D1〜D4 と同じ区別が書かれていること、文書の自然言語の記述以外の AC に `test` か `static` を求め続けていることを、読み合わせで確かめる（AC-23〜AC-25）。

**完了条件**: 上の 4 つが済んでいる。

### PR-1 作成ポイント: rules for document tests and the pruning check script

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `docs(0038): 文書の自然言語の記述を定める AC に manual だけの検証を認め、文書のテストの方針と削除の検査のスクリプトを加える`

**レビュー観点**: 例外が文書の自然言語の記述を定める AC だけに当たり、振る舞い・値・構造の AC に `test` か `static` を求め続けること / `CLAUDE.md` の方針の K1〜K3・D1〜D4・K4・K5 が要件書 F-001 と一致すること / 名前の一覧が設計書 3.1・3.2 と本書 1.3.2 に一致し、検査のスクリプトの各サブコマンドが壊したときに `1` か `2` で終わること

**実装モデル要件**: standard

**判定理由**: 規則の文書と、タスクの中で使う検査のスクリプトだけの変更で、`mkplan2.md` step 4 の契機のどれにも当たらない。スクリプトは後の PR のカバレッジの比較に使うが、作り方は本書で決まっている。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `cmd/fixprguard`（設計書 8 章の PR-2）

#### ステップ 2-1: 始めの確かめ

**変更するファイル**: なし（足りないものがあれば、0034 の計画）

-   [ ] タスク 0034・0037 の実装の PR がすべてマージされたことを、両タスクの計画の PR マーカーで確かめる。マージされていなければ、本フェーズを始めない（設計書 3.4）。
-   [ ] 1.4 の 1 つめの 0034 の計画の改訂（ステップ 8-4 の取り下げ、AC-20・AC-20a の `manual`、`:123`・ステップ 8-5・`:607` の改訂、`Comments` の `(decision)`）が `main` にあることを確かめる。足りなければ、本 PR で改める（AC-17）。
-   [ ] `TestTask0030DescribesIgnoredFiles` が `main` にないことを確かめる。あれば、`pruned_names.txt` に加え、ステップ 2-2 で削除する（AC-18）。
-   [ ] 今の `main` で、`cmd/fixprguard` の文書のテストを数え直し、`pruned_names.txt` にないテストがあれば、判定して書き足す。

**完了条件**: 上の 4 つが済み、本フェーズを始められる。

#### ステップ 2-2: テストごと削除する

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [ ] `TestGuideDescribesConfigScope` を削除する（AC-03、#440）。
-   [ ] `TestTask0030NotesScopeChange` を削除する。

**完了条件**: 2 つがなく、`make test` が通る。

#### ステップ 2-3: 一部を削除する

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [ ] `TestGuideCoversStopKindsAndScope`: 設計書 3.1.1 の削除する確認を取り除く。
-   [ ] `TestGuideDescribesReviewerList`: 同上。
-   [ ] `TestGuideListsWatchList`: 1.3.2 の削除する確認を取り除く。
-   [ ] `TestGuideDescribesIgnoredFiles`: 同上。
-   [ ] `TestGuideIgnoredCommands`: 同上。

**完了条件**: 残す確認だけが残り、`make test` が通る。

#### ステップ 2-4: 補助と確かめ

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [ ] 設計書 3.2 の `cmd/fixprguard` の行の補助（`configScope`・`passage`・`checkPassages`・`containsAll`・`reviewerListName`・`contentGuard`・`task0036Docs`・`commentRecord`・`scopeRequirement`）と、1.3.2 の `requireAll` を、使い手がなくなったものから削除する。単体テストを持つ補助（`cutSection`・`guideTable`・`checkDocumentedReviewer`）は、単体テスト以外の使い手を `git grep` で数え、設計書 3.2 と違えば PR の説明に記す。
-   [ ] 中身を変えたテストの残す確認を、1 つずつ壊して落ちることを確かめる（設計書 7.2）。
-   [ ] 削除の各コミットで、検査のスクリプトの `coverage` を実行し、差分が空であることをコミットメッセージに記す（AC-10）。差分が `//go:build test` の補助のファイルだけなら、使われなくなった補助として AC-06 で扱う。製品のコードに差分があれば、設計書 6.1 のとおり製品のコードを通る部分を残し、例外として判定の一覧に記す（AC-11）。

**完了条件**: `check_pruning.sh removed 2` と `make lint` が通る。

#### ステップ 2-5: 文書

**変更するファイル**: 設計書 3.3.2 の表で PR-2 が担当するタスクの文書（0030 の注記の `cmd/fixprguard` の分、0033、0034、0035、0036）、`fixprguard_guide.md`・`.ja.md`（必要な場合）

-   [ ] 各文書の AC の検証の行を、設計書 3.3.2 の規則で改め、`Comments` に記録する（AC-13・AC-14）。
-   [ ] タスク 0035 の AC-18 を、計画の 5 章の表と注記、設計書の 7.5 の両方で `manual` にする（AC-15）。
-   [ ] `fixprguard_guide`（日英）9 章で、削除したテストを述べる箇所がないことを確かめる（1.3.5）。

**完了条件**: `check_pruning.sh records <PR-2 の前の origin/main>` が通る。

#### ステップ 2-6: PR-2 の確かめ

**変更するファイル**: なし

-   [ ] `main` を取り込み、`cmd/fixprguard` の文書のテストを数え直す。増えていれば、ステップ 2-1 の 4 つめと同じく扱う（設計書 3.4）。
-   [ ] `check_pruning.sh` の `removed 2`・`kept`・`unchanged <PR の前の origin/main>`・`records <PR の前の origin/main>` を実行し、結果を PR の説明に記す。
-   [ ] `make test && make lint` を実行する。
-   [ ] PR の説明に、判定の一覧（AC-09）と `Refs #440`（AC-26）を書く。判定の一覧には、英語のガイドの 8.1 のブロックが実行されなくなることを、残余のリスクとして挙げる（7.1）。

**完了条件**: 上の 4 つが済んでいる。

### PR-2 作成ポイント: prune fixprguard document tests

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6

**推奨タイトル**: `test(0038): cmd/fixprguard の文書のテストから、自然言語の意味と不要な文言を固定する確認を削除する`

**レビュー観点**: 判定の一覧の各確認の分類が要件書 F-001 と設計書 3.1.1・本書 1.3.2 に合うこと / 残す確認が、読む文書・位置と値の出どころを変えていないこと / 改めたタスクの文書の AC の検証の行と `Comments` / カバレッジの比較と壊した確かめの記録

**実装モデル要件**: standard

**判定理由**: テストと過去のタスクの文書の削除・改訂で、判定は設計書と本書で決まっており、`mkplan2.md` step 4 の契機のどれにも当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: `cmd/yt2column` の `/fixpr`・`/runpr`・Codex のテスト（設計書 8 章の PR-3）

#### ステップ 3-1: 始めの確かめ

**変更するファイル**: なし

-   [ ] ステップ 2-1 の 1 つめと 4 つめを、`fixpr_docs_test.go`・`codex_review_docs_test.go` について行う。

**完了条件**: 本フェーズを始められる。

#### ステップ 3-2: `TestRunprDocContract` の作り直し

**変更するファイル**: `cmd/yt2column/fixpr_docs_test.go`

1.4 の 8 つめのとおり、このステップのコミットを、ほかの削除より先に置く。

-   [ ] step 3 の `gh api --paginate ... --jq '...'` のすべての一覧のフィルタを取り出し、`jq` で実行する確認を加える（1.4 の 2 つめ）。確かめる性質は次のとおりである。
    -   一覧がちょうど 3 つ（issue のコメント、行のコメント、レビュー）であること。
    -   偽の JSON の各項目について、出力の有無と `kind` が期待どおりであること。偽の JSON は、各フィルタのすべての分かれ道を通る内容にする。要約のコメントは、状態の出力のすべての分かれ道（`completed`・`running`・状態の行の形の誤り・JSON として読めない・欄の型の誤り・知らない状態）を通る。行のコメントとレビューの一覧は、Codex の投稿、言及を含まない投稿、言及を含む投稿を含む。
    -   出力の各オブジェクトの鍵が、その `kind` に許した鍵（`kind`・`url`・`created_at`・`updated_at`・`state`）だけで、値が、偽の JSON の同じ項目の本文以外の欄の値と等しく、`state` が状態の形（`completed <sha>`・`running <sha>`・`indeterminate`）であること。
    -   本文に目印を入れられる箇所（各行の先頭と末尾、状態の行の JSON の余分な欄と知らない状態の値）すべてに、英小文字と数字だけの目印を入れ、出力に大文字小文字を区別せずに目印が現れないこと。依頼のコメントの本文と要約のコメントの 1 行めは、決まった値のときだけフィルタを通るので、目印を入れられない。この 2 つが本文を出力したときは、上の値の比較で検出する。
    -   `jq` が終了コード `0` で終わり、標準エラーが空であること。

    なお、プレースホルダの置き換えの値と、フィルタの取り出しの方法は、実装で決める。
-   [ ] 設計書 3.1.2 の残す確認（`fixprguard head` の順序、`reviewThreads`・`comments(first:` の不在、`StopJudgedSetChanged`・`StopGitStateChanged`）を残し、フィルタの全文の確認とほかを取り除く。
-   [ ] このコミットで、検査のスクリプトの `coverage` を実行し、結果をコミットメッセージに記す（AC-10）。
-   [ ] 壊して確かめる（結果をコミットメッセージに記す）。次のそれぞれで確認が失敗すること。(a) issue のコメントの一覧のうち、Codex のほかのコメント・依頼・言及の各分かれ道の出力に `body` を加えた写し。(b) 行のコメントの一覧の出力に `body` を加えた写し。(c) レビューの一覧の出力に `body` を加えた写し。(d) 要約のコメントの `running` の分かれ道の出力に `$l[1]` を加えた写し。(e) step 3 に 4 つめの一覧を加えた写し。(f) `jq` を `PATH` から外した環境。

**完了条件**: 上の確かめが済み、`make test` が通る。

#### ステップ 3-3: テストごと削除する

**変更するファイル**: `cmd/yt2column/fixpr_docs_test.go`、`cmd/yt2column/codex_review_docs_test.go`

-   [ ] `TestHandoffStartMarker` を削除する。
-   [ ] `TestAuthorizationSections` を削除する。
-   [ ] `TestGlossaryTerms` を削除する。
-   [ ] `TestRequestExemptFromMentionCheck` を削除する。
-   [ ] `TestNoCodeRabbitPRReview` を削除する。
-   [ ] `TestLocalCodeRabbitReviewKept` を削除する。
-   [ ] `TestDocsDescribeCodexReview` を削除する。
-   [ ] `TestCodexObservationRecord` を削除する。

**完了条件**: 8 つがなく、`make test` が通る。

#### ステップ 3-4: 一部を削除する

**変更するファイル**: 同上

-   [ ] `TestFixprDocContract`: 設計書 3.1.2 の削除する確認と、サブテスト `ignored_file_stops`（1.3.2）を取り除く。round_counting と Phase 3.5 の確認は、設計書 3.1.8 の残す部分にする。
-   [ ] `TestContextAndCommandsAgree`: 設計書 3.1.2 の削除する確認を取り除き、PR レビュー依頼の行の説明の確認を、設計書 3.1.8 の残す部分にする。
-   [ ] `TestRemovedCommandLiterals`: 設計書 3.1.2 の D3 の語を取り除く。
-   [ ] `TestPostingPathsCheckMention`: 設計書 3.1.8 の残す部分（各節の `**Mention check.**` の行が言及の確認のコマンドを含むこと）にする。
-   [ ] `TestAgentsCodeReviewRules`: 設計書 3.1.3 の削除する確認を取り除く。

**完了条件**: 残す確認だけが残り、`make test` が通る。

#### ステップ 3-5: 補助と確かめ

**変更するファイル**: `cmd/yt2column/fixpr_docs_test.go`、`cmd/yt2column/codex_review_docs_test.go`

-   [ ] 設計書 3.2 の `handoffStep5` などの行の補助・定数・フィールドを、使い手がなくなったものから削除する。ステップ 3-2 で使わなくなったフィルタの定数（`reviewedCheck`・`reviewedRules`・`mentionListings`・`requestCommentFilter`・`requestCommand`・`lastReviewedRule`・`requestRules`・`arrivalRules`・`autoFixprCondition`）も含む。`runprStep5` は `TestPostingCommandsInventory` が読むので残す（設計書 3.4）。単体テストを持つ補助（`isPost` など）は、ステップ 2-4 と同じく使い手を数える。
-   [ ] ステップ 2-4 の 2 つめと 3 つめ（壊した確かめ、カバレッジの比較）を行う。K4・K5 の確認は、設計書 7.2 の例（`fixpr.md` にレビュアーの名前を書く、`git commit -m` を加える、`fixprguard head` の呼び出しを消す、言及の確認のコマンドを消す）で壊して確かめる。

**完了条件**: `check_pruning.sh removed 3` と `make lint` が通る。

#### ステップ 3-6: 文書

**変更するファイル**: 設計書 3.3.2 の表で PR-3 が担当するタスクの文書（0030、0031、0033、0034、0036）、`CLAUDE.md`

-   [ ] 各文書の AC の検証の行を、設計書 3.3.2 の規則で改め、`Comments` に記録する（AC-13・AC-14）。タスク 0034 の行は、1.3.2 の判定に従う。
-   [ ] `CLAUDE.md`「Development Notes」の、`make test` に要らないもの（`yt-dlp`）を述べる項の近くに、`make test` が `jq` を要することを記す（1.3.3）。

**完了条件**: `check_pruning.sh records <PR-3 の前の origin/main>` が通る。

#### ステップ 3-7: PR-3 の確かめ

**変更するファイル**: なし

-   [ ] ステップ 2-6 の 1〜3 つめを、`fixpr_docs_test.go`・`codex_review_docs_test.go` と `removed 3` について行う。
-   [ ] PR の説明に判定の一覧を書く（AC-09）。`ignored_file_stops` と `TestRequestExemptFromMentionCheck` などで読み合わせに任せる手順の文を、残余のリスクとして挙げる（7.1、設計書 5.1）。
-   [ ] CI の `test` ジョブと `doc-guard` ジョブの両方で、ステップ 3-2 の確認が `jq` を見つけて通ったことを確かめ、`jq` の版を記す（1.3.4）。見つからずに失敗したら、止まって利用者と扱いを決める。

**完了条件**: 上の 3 つが済んでいる。

### PR-3 作成ポイント: prune fixpr, runpr, and Codex document tests

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7

**推奨タイトル**: `test(0038): /fixpr・/runpr・Codex の文書のテストを削り、step 3 のフィルタが本文を出力しないことを jq で確かめる`

**レビュー観点**: K4・K5 の確認（自動レビュアーの一覧、ガードツールの呼び出し、迂回するコマンド、言及の確認のコマンド）が残り、壊すと落ちること / `jq` の確認が 3 つの一覧のすべての分かれ道を通り、本文やその一部の出力を検出すること / 削除した確認の分類が設計書 3.1.2・3.1.3 と本書 1.3.2 に合うこと / 改めたタスクの文書の AC の検証の行と `Comments`

**実装モデル要件**: frontier-required

**判定理由**: ステップ 3-2・3-5 は、`/fixpr`・`/runpr` の信頼の境界を守る確認（K4・K5）を作り直し、ほかを削除するセキュリティのゲートの変更で、`mkplan.md` step 8 の panel mode の契機に当たる。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: `cmd/yt2column` のほかのテスト（設計書 8 章の PR-4）

#### ステップ 4-1: 始めの確かめ

**変更するファイル**: なし

-   [ ] ステップ 2-1 の 1 つめと 4 つめを、`docs_test.go`・`output_layout_docs_test.go`・`run_test.go`・`copied_transcript_docs_test.go`・`extension_docs_test.go` について行う。

**完了条件**: 本フェーズを始められる。

#### ステップ 4-2: テストごと削除する

**変更するファイル**: `cmd/yt2column/docs_test.go`、`copied_transcript_docs_test.go`、`extension_docs_test.go`

-   [ ] `TestReviewEvaluationRecords` を削除する。
-   [ ] `TestPackageReferenceFixprguard` を削除する。
-   [ ] `TestClaudeMDCopiedTranscriptException` を削除する。
-   [ ] `TestCopiedTranscriptDocsUpdated` を削除する。
-   [ ] `TestClaudeBullet` を削除する。
-   [ ] `TestExtensionSendDocumentation` を削除する。
-   [ ] `TestContextMenuRemovalRecords` を削除する。
-   [ ] `TestNativeMessagingRelayDocumentation` を削除する。
-   [ ] `TestSectionByHeadingPrefix` を削除する。

**完了条件**: 9 つがなく、`make test` が通る。

#### ステップ 4-3: 一部を削除する

**変更するファイル**: `cmd/yt2column/docs_test.go`、`output_layout_docs_test.go`、`run_test.go`、`copied_transcript_docs_test.go`、`extension_docs_test.go`

-   [ ] `TestREADMEDocumentsFlags`: 設計書 3.1.4 の削除する確認を取り除く。
-   [ ] `TestClaudeDocsContract`: 同上。
-   [ ] `TestArtifactDocsContract`: 文書の確認をすべて取り除き、`run -h` の 2 つの文字列（`requires --artifacts`・`--after review does not accept --review-log`）を、1.3.3 の対応表のとおり `TestRunHelp` の一覧に加えて、テストを削除する。確かめる文字列は変えない（AC-04）。
-   [ ] `TestTokenUsageDocsContract`: 設計書 3.1.4 の削除する確認を取り除く。
-   [ ] `TestCostDocsContract`: 同上。
-   [ ] `TestOutputLayoutDocsContract`: 同上。
-   [ ] `TestRunEvaluationProcedureFlags`: 同上。
-   [ ] `TestCopiedTranscriptSurveyRecords`: 設計書 3.1.5 の削除する確認を取り除く。
-   [ ] `TestExtensionDocumentation`: 同上。

**完了条件**: 残す確認だけが残り、`make test` が通る。

#### ステップ 4-4: 補助と確かめ

**変更するファイル**: 同上

-   [ ] 設計書 3.2 の `markdownBullet`・`sectionByHeadingPrefix`・`claudeBullet`・`stepBlock` などの行、`sampleRecordedSaved` などの行、`checkManualCheckTable` などの行の補助と定数を、使い手がなくなったものから削除する。単体テストを持つ補助（`sectionByHeading`・`firstCodeBlockAfter` など）は、ステップ 2-4 と同じく使い手を数える。
-   [ ] ステップ 2-4 の 2 つめと 3 つめを行う。`TestRunHelp` は、`run.go` の `--after` の説明から 2 つの文字列をそれぞれ消すと落ちることを確かめる。

**完了条件**: `check_pruning.sh removed 4` と `make lint` が通る。

#### ステップ 4-5: 文書

**変更するファイル**: 設計書 3.3.2 の表で PR-4 が担当するタスクの文書（0005、0006、0010、0011 の分、0012、0015、0016、0017、0019、0020 の分、0022、0024、0026 の分、0027 の分、0028、0033 の分）、`docs/dev/developer_guide/pricing_update.md`、`CLAUDE.md`

-   [ ] 各文書の AC の検証の行を、設計書 3.3.2 の規則で改め、`Comments` に記録する（AC-13・AC-14）。タスク 0017 の行は、1.3.3 の対応表のとおりに改め、設計書 `:862`（AC-33）も規則で改める。
-   [ ] `pricing_update.md:69-71` の、`package_reference.md` の `internal/pricing` の行を `TestTokenUsageDocsContract` が確かめる、という記述を改める（設計書 3.6）。
-   [ ] `CLAUDE.md`「Development Notes」の text-sync guards の一覧から「the task records」を外し、「the `project_overview.md` notes」を残る確認（環境変数の名前、`costPrefix`、`16,383`）に合わせる（設計書 3.6、AC-20）。

**完了条件**: `check_pruning.sh records <PR-4 の前の origin/main>` が通る。

#### ステップ 4-6: PR-4 の確かめ

**変更するファイル**: なし

-   [ ] ステップ 2-6 の 1〜3 つめを、本フェーズのファイルと `removed 4` について行う。
-   [ ] PR の説明に判定の一覧を書く（AC-09）。

**完了条件**: 上の 2 つが済んでいる。

### PR-4 作成ポイント: prune the other cmd/yt2column document tests

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6

**推奨タイトル**: `test(0038): cmd/yt2column のほかの文書のテストから、自然言語の意味・不要な文言・タスクの記録を固定する確認を削除する`

**レビュー観点**: K1・K3 の確認（README のフラグ・費用・出力の例、`testdata/README.md`、拡張機能の ID と権限）が、読む位置と値の出どころを変えずに残ること / `TestRunHelp` が `run -h` の同じ 2 つの文字列を確かめること / 多くのタスクの文書の AC の検証の行の改め方が設計書 3.3.2 の規則に合うこと

**実装モデル要件**: standard

**判定理由**: 変更は多いが、判定と改め方は設計書と本書で決まっており、`mkplan2.md` step 4 の契機のどれにも当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: サーバ・`internal/pipeline`・拡張（設計書 8 章の PR-5）

#### ステップ 5-1: 始めの確かめ

**変更するファイル**: なし

-   [ ] ステップ 2-1 の 1 つめと 4 つめを、`cmd/yt2column-server/docs_test.go`・`relay_guard_test.go`・`extension/test/` について行う。

**完了条件**: 本フェーズを始められる。

#### ステップ 5-2: Go のテスト

**変更するファイル**: `cmd/yt2column-server/docs_test.go`、`internal/pipeline/relay_guard_test.go`

-   [ ] `TestServerDocsContract`: 設計書 3.1.6 の削除する確認を取り除き、3.1.8 の 4 つの行の残す部分にする。README の残す確認は、更新の節を除いた節で確かめ続ける（設計書 3.1.6）。
-   [ ] `TestSharedTokenRemoved`: 文書を読む部分を取り除き、Go のソースと `extension/src`・`extension/static` を調べる部分を残す（設計書 3.1.6）。
-   [ ] `TestWithoutSection` を削除する。
-   [ ] 補助 `withoutSection`・`splitSection`・`upgradeHeading` と、使われなくなるパスの定数（`cmd/yt2column-server/docs_test.go` の `claudePath`・`glossaryPath` など）を削除する（設計書 3.2）。
-   [ ] ステップ 2-4 の 2 つめと 3 つめを行う。

**完了条件**: `make test` と `make lint` が通る。

#### ステップ 5-3: 拡張のテストと起動条件

**変更するファイル**: `extension/test/metadataDocs.test.ts`、`extension/scripts/has-extension-changes.sh`、`extension/test/ciChanges.test.ts`

-   [ ] `metadataDocs.test.ts` の「README says the extension sends the video metadata」と「security.md 8.1 records ...」の 2 つの確認を削除する（設計書 3.1.7）。
-   [ ] 同じコミットで、`has-extension-changes.sh` の冒頭の説明とパターンから `docs/dev/security.md` を外し、`ciChanges.test.ts` の `docs/dev/security.md` を `quiet` に移し、`x/docs/dev/security.md`・`docs/dev/security.md.orig` の行を取り除く（設計書 2.2、AC-28〜AC-30）。
-   [ ] `make ext-check` を通し、削除した確認が `extension/src` を読み込まないことを判定の一覧に記す（AC-12）。

**完了条件**: `check_pruning.sh removed 5` と `make ext-check` が通る。

#### ステップ 5-4: 文書

**変更するファイル**: `CLAUDE.md`、設計書 3.3.2 の表で PR-5 が担当するタスクの文書（0011 の分、0021、0025、0026 の分、0027 の分）

-   [ ] `CLAUDE.md`「Development Notes」の、node のテストが読む文書を README.md だけにし、README で読むものを、残る確認（拒否理由の表と、展開の案内の文）に合わせる。`cmd/yt2column-nmhost` の `TestReadmeHostExitCodes` は `doc-guard` ジョブでなく、ローカルの `make test` と CI の `test` ジョブが走らせる、と記す（設計書 3.6、AC-20・AC-30）。`doc-guard` のコマンドは変えない（AC-21）。
-   [ ] 各文書の AC の検証の行を、設計書 3.3.2 の規則で改め、`Comments` に記録する（AC-13・AC-14）。

**完了条件**: `check_pruning.sh records <PR-5 の前の origin/main>` が通る。

#### ステップ 5-5: AC-05 の調べと承認済みの計画の見直し

**変更するファイル**: なし（見つかったものに応じて、テストやタスクの文書）

-   [ ] `check_pruning.sh survey` の出力を、PR-2〜PR-5 の判定の一覧と比べ、一覧にない文書のテストがあれば判定して扱う（AC-05、設計書 3.4 の 3 つめ）。1.3.5 の 2 つのスクリプトの扱いを判定の一覧に記す。
-   [ ] 承認済みの計画のうち実装の作業が残るものを見直し、D1〜D4 だけに当たる文書のテストを加えるものがないことを確かめる（AC-17、設計書 3.4）。

**完了条件**: 調べと見直しの結果を PR の説明に記した。

#### ステップ 5-6: PR-5 の確かめ

**変更するファイル**: なし

-   [ ] ステップ 2-6 の 1〜3 つめを、本フェーズのファイルと `removed 5` について行い、`make ext-check` を実行する（AC-08）。
-   [ ] PR の説明に判定の一覧を書く（AC-09）。
-   [ ] 本書 5 章の表のすべての行の検証を実行した、または記録したことを確かめる（最後の PR の確かめ）。

**完了条件**: 上の 3 つが済んでいる。

### PR-5 作成ポイント: prune server, pipeline, and extension document tests

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6

**推奨タイトル**: `test(0038): サーバ・internal/pipeline・拡張の文書のテストを削り、拡張の CI ジョブの起動条件から security.md を外す`

**レビュー観点**: `TestServerDocsContract` の残す確認が、更新の節を除いた節で同じ値を確かめること / `docs/dev/security.md` を外す変更と `metadataDocs.test.ts` の削除が同じコミットにあり、`ciChanges.test.ts` が新しい起動条件を確かめること / `CLAUDE.md`「Development Notes」の説明が残る文書のテストと一致すること / AC-05 の調べのやり直しの結果

**実装モデル要件**: standard

**判定理由**: 変更検出のスクリプトの変更（`mkplan.md` の Conditional checks の 1 つ）を含むが、既存の `ciChanges.test.ts` が確かめる 1 つのパターンの削除で、`mkplan2.md` step 4 のほかの契機に当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

**マージの後**: #440 を、本タスクで対応したことを述べるコメントを付けて閉じる（AC-27、設計書 3.7）。#455 は `/runpr` が閉じる（`runpr.md` step 5）。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の時点 |
|---|---|---|
| M1 | 改めた規則と方針、検査のスクリプトと名前の一覧 | PR-1 のマージ |
| M2 | `cmd/fixprguard` の文書のテストの削除、#440 への参照 | PR-2 のマージ |
| M3 | `/fixpr`・`/runpr`・Codex の文書のテストの削除と、`jq` の確認 | PR-3 のマージ |
| M4 | `cmd/yt2column` のほかの文書のテストの削除 | PR-4 のマージ |
| M5 | サーバ・`internal/pipeline`・拡張の削除と起動条件、AC-05 の調べ | PR-5 のマージ |
| M6 | #440 を閉じる | PR-5 のマージの後 |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | 検査のスクリプトと名前の一覧、`CLAUDE.md`「Testing Strategy」、`mkplan.md`・`runplan.md`・`weakreview.md`・`requirements_process.md` の規則 | standard |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 | `cmd/fixprguard/docs_test.go`、タスク 0030・0033〜0036 の文書 | standard |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 | `TestRunprDocContract` の `jq` の確認、`fixpr_docs_test.go`・`codex_review_docs_test.go`、タスク 0030・0031・0033・0034・0036 の文書、`CLAUDE.md` の `jq` | frontier-required |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 | `cmd/yt2column` のほかの文書のテスト、`pricing_update.md`、`CLAUDE.md`、多くのタスクの文書 | standard |
| PR-5 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 | サーバ・`internal/pipeline`・拡張のテスト、起動条件、`CLAUDE.md`、AC-05 の調べ | standard |

### 3.3. 実装順序の根拠

-   設計書 8 章のとおり、規則を先に改め（PR-1）、続いてパッケージ（またはテストの群）ごとに削除する（PR-2〜PR-5）。PR-1 は、タスク 0037 のマージの後という前提を満たしている（1.3.1）。
-   PR-2〜PR-5 は、タスク 0034 の PR-3 のマージの後に始める（設計書 3.4、1.4 の 1 つめ）。
-   PR-3 は、`jq` の確認の作り直しを最初のコミットに置き、大量の削除と分けて読めるようにする（1.4 の 8 つめ）。PR-4 は大きいが、判定は設計書で決まっており、ファイルごとのコミットで読めるので、分けない。
-   PR-5 を最後に置くのは、AC-05 の調べと承認済みの計画の見直しを、ほかの削除がすべて入った `main` でやり直すためである（設計書 3.4）。

## 4. テスト戦略 (Test Strategy)

### 4.1. カバレッジの比較

設計書 7.1 のとおりに、検査のスクリプトの `coverage` で、Go のテストを削除する、または一部を削除するコミットごとに比べる。比べたリビジョンと、差分が空であったことをコミットメッセージに記す（AC-10）。`//go:build test` の補助のファイルだけの差分は、製品のコードの網羅ではないので、AC-11 でなく AC-06（使われなくなった補助）で扱う。

### 4.2. 残す確認が落ちうること

設計書 7.2 のとおり、中身を変えたテストの残す確認を、1 つずつ壊して落ちることを確かめ、コミットメッセージに記す。設計書 3.1.8 の、手を入れる残す確認と、K4・K5 の確認と、`TestAgentsCodeReviewRules` の見出しの確認（K2）は必ず確かめる。本書で加える確認（ステップ 3-2 の `jq` の確認、`TestRunHelp` に加える 2 つの文字列）も同じく確かめる。

### 4.3. 新しく書く確認

本タスクで新しく書く確認は、ステップ 3-2 の `jq` の確認だけである。今の `TestRunprDocContract` がフィルタの全文の固定で守っていた性質のうち、K5 の性質（本文を出力しないこと）を、文言に依らずに確かめる形にする（設計書 3.1.8）。`TestRunHelp` には、`TestArtifactDocsContract` の 2 つの文字列を移すだけである。

### 4.4. 拡張の確認

設計書 7.3 のとおり、PR-5 で `make ext-check` を通す。

### 4.5. 検査のスクリプト

ステップ 1-1 で、`28cb00c` に対して通ることと、壊したときに `1` か `2` で終わることを確かめる。スクリプトそのものの恒久のテストは書かない。タスクの中だけで使い、`make test` と CI では実行しないためである。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`check_pruning.sh` は検査のスクリプト（`docs/tasks/0038_doc_test_pruning/check_pruning.sh`）を指す。

| AC | ラベル | 検証 | 担当 |
|---|---|---|---|
| AC-01 | static + manual | `check_pruning.sh kept` と `check_pruning.sh removed <PR>`（名前の一覧の削除と残すテストの有無）。確認ごとの分類は、各 PR の判定の一覧と差分を読み合わせる | PR-2〜PR-5 |
| AC-02 | test + manual | `make test`（残す確認が通ること）。残す確認ごとに、読む文書・位置と値の出どころが同じであることを判定の一覧に記し、壊して落ちることをコミットメッセージに記す（4.2） | PR-2〜PR-5 |
| AC-03 | static | `check_pruning.sh removed 2`（`TestGuideDescribesConfigScope` の行） | PR-2 |
| AC-04 | static + test + manual | `check_pruning.sh coverage`、`make test`。文書を読まない確認の差分を読み合わせる | PR-2〜PR-5 |
| AC-05 | static + manual | `check_pruning.sh survey` の出力と判定の一覧を比べる（ステップ 5-5） | PR-5 |
| AC-06 | static + manual | `make lint`（`unused`）、`check_pruning.sh removed <PR>`。単体テストだけが使う補助とフィールドは `git grep` で数える | PR-2〜PR-5 |
| AC-07 | static | `check_pruning.sh removed <PR>`（削除した補助の単体テスト）と `check_pruning.sh kept`（残す補助の単体テスト） | PR-2〜PR-5 |
| AC-08 | test | `make test`、`make lint`、`make ext-check` | PR-1〜PR-5 |
| AC-09 | manual | 各 PR の説明の判定の一覧 | PR-2〜PR-5 |
| AC-10 | static + manual | `check_pruning.sh coverage`。結果をコミットメッセージに記す | PR-2〜PR-5 |
| AC-11 | static + manual | `check_pruning.sh coverage`。差分が出た確認の扱いを判定の一覧に記す | PR-2〜PR-5 |
| AC-12 | test + manual | `make ext-test`。削除した確認が `extension/src` を読み込まないことを判定の一覧に記す | PR-5 |
| AC-13 | manual | 改めたタスクの文書の差分の読み合わせ | PR-2〜PR-5 |
| AC-14 | static + manual | `check_pruning.sh records <base>`（`Status` と `Comments` の `(decision)`・`0038`）。日付と AC の ID は読み合わせる | PR-2〜PR-5 |
| AC-15 | static | `check_pruning.sh records <base>`（タスク 0035 の AC-18 の 2 か所） | PR-2 |
| AC-16 | static | `check_pruning.sh unchanged <base>`（タスク 0029 の文書） | PR-2〜PR-5 |
| AC-17 | static + manual | `check_pruning.sh records <base>`（0034 の計画を本 PR で改めた場合）。0034 の計画の差分（ステップ 2-1）と、ステップ 5-5 の見直しの結果 | PR-2・PR-5 |
| AC-18 | static + manual | `check_pruning.sh removed 2`（`TestTask0030DescribesIgnoredFiles` が加えられていた場合）。判定の一覧（1.3.2） | PR-2・PR-3 |
| AC-19 | static + manual | `check_pruning.sh kept`（`TestGuideNamesGitStateParts` などの残すテスト）。判定の一覧（1.3.2） | PR-2 |
| AC-20 | manual | `CLAUDE.md`「Development Notes」の差分の読み合わせ | PR-4・PR-5 |
| AC-21 | static + manual | `check_pruning.sh unchanged <base>`（`.github/workflows/ci.yml` の `doc-guard` の行）。`CLAUDE.md`・ガイドのコマンドの差分の読み合わせ | PR-2〜PR-5 |
| AC-22 | static + manual | `check_pruning.sh removed <PR>`（`docs/tasks/` 以外の文書）。K4 の手順の読み合わせ | PR-2〜PR-5 |
| AC-23 | manual | `CLAUDE.md`「Testing Strategy」の差分の読み合わせ | PR-1 |
| AC-24 | manual | 規則の差分の読み合わせ | PR-1 |
| AC-25 | manual | 同上 | PR-1 |
| AC-26 | manual | PR-2 の説明の `Refs #440` | PR-2 |
| AC-27 | manual | PR-5 のマージの後の #440 の状態とコメント | PR-5 の後 |
| AC-28 | test + manual | `extension/test/ciChanges.test.ts`（`make ext-test`）。起動条件の文書が node のテストの読む文書と同じであることは、`check_pruning.sh survey` の出力と読み合わせる | PR-5 |
| AC-29 | test | `extension/test/ciChanges.test.ts`（文書でない条件の行は変えない） | PR-5 |
| AC-30 | test + manual | 同上。`has-extension-changes.sh` の冒頭の説明と `CLAUDE.md` の差分の読み合わせ | PR-5 |

**`manual` だけの AC:**

-   AC-20・AC-23〜AC-25 は、文書の自然言語の記述を定める AC である。PR-1 で改める規則（AC-24）により `manual` で確かめる。要件書 5 章は、本タスクの AC-22〜AC-24 が改めた後の規則に従うと定め、設計書 7.5 は AC-23〜AC-25 を `manual` とする。
-   AC-13 は、タスクの文書の AC の検証の行の記述を定める AC である。削除したテストの名前は本文の作業の記録にも残る（要件書 2.3）ので、名前の検索では確かめられない。
-   AC-09・AC-26・AC-27 は、PR の説明と issue の状態という、リポジトリの外の記録を定める AC である。リポジトリのテストや検査の対象にならない（設計書 7.5）。

### 5.1. 横断の検索

削除・名前の変更の残りは、`check_pruning.sh removed <PR>` が確かめる（AC-03・AC-06・AC-22）。`docs/tasks/` の本文の作業の記録に残る名前は、要件書 2.3 のとおり変えないので、検索の対象にしない。

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7）
- [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6）
- [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6）
- [ ] #440 を閉じた（AC-27）

## 7. リスク管理 (Risk Management)

### 7.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| `gh --jq`（gojq）と `jq` 1.7 の結果が違う構文が、step 3 のフィルタに入る | ステップ 3-2 の確認が、実際に `gh` が出力するものと違う結果を確かめる | 受け入れる。`runpr.md` のフィルタを変える PR のレビューで読み合わせる |
| CI のランナーに `jq` がない | `test`・`doc-guard` ジョブが失敗する | ステップ 3-7 で確かめ、失敗したら止まって利用者と扱いを決める |
| タスク 0034 の PR-3 が、1.4 の 1 つめの改訂なしでマージされる | D4 のテストが入る、または 0034 の計画が矛盾したまま残る | ステップ 2-1 で確かめ、PR-2 でテストを削除し、計画を改める（AC-17・AC-18） |
| 削除の PR の作業の間に、`main` に文書のテストが入る | 判定の一覧から漏れる | 各 PR の始めと PR を作る直前に数え直し（ステップ 2-1・2-6 ほか）、PR-5 で AC-05 の調べをやり直す |
| 確認の分類を誤り、K4・K5 の確認を削除する | `/fixpr`・`/runpr` の信頼の境界が、テストで守られなくなる | PR-3 を `frontier-required` とし、K4・K5 の確認を壊して確かめる（4.2） |
| カバレッジが実行ごとに揺れる関数がある | 削除と関係なく差分が出て、比較が意味を失う | 揺れる関数を比較から外さない（外すと、その関数の網羅を下げる削除を見逃す）。ステップ 1-1 の 2 回の自己比較か、後の比較で揺れる関数が見つかったら、止まって、結果を決定的にする方法（たとえば揺れの原因のテストを `-count=1` で単独に実行して確かめる）を利用者と決める |
| 時間に依存するテストが負荷で落ちる | 失敗した実行を基準にしてしまう | 検査のスクリプトが、失敗した実行の結果を出さない（ステップ 1-1） |
| テストで守られなくなる手順の文（`fixpr.md` の無視されるファイルを調べない指示など、1.3.2 の `ignored_file_stops`）と、実行されなくなる英語のガイドの 8.1 のブロック | 文が書き換えられても、英語のブロックが日本語と違っても、テストでは気づけない | 受け入れる（要件書 4.2、設計書 4.3・5.1 と同じ残余のリスク）。PR-3・PR-2 の判定の一覧に挙げる |
| 行番号が `main` の変化でずれる | 誤った箇所を改める | 各 PR の始めに、テスト名と見出しで探し直す（1.2） |

### 7.2. スケジュールのリスク

-   PR-2〜PR-5 は、タスク 0034 の PR-3 のマージを待つ。待つ間に PR-1 を進める。

## 8. 成功基準 (Success Criteria)

-   残る文書のテストの確認が、すべて K1〜K5 に当たる（AC-01）。
-   `make test`・`make lint`・`make ext-check` が通る（AC-08）。
-   削除のコミットごとに、カバレッジの比較の差分が空である（AC-10）。
-   `check_pruning.sh` の `removed 5`・`kept`・`unchanged <base>` が、PR-5 のブランチで通る。
-   手順の規則と `CLAUDE.md` が、文書の自然言語の記述を定める AC に `manual` だけを認め、ほかの AC には `test` か `static` を求める（AC-24・AC-25）。

## 9. 次のステップ (Next Steps)

-   本書の承認の後、`/runplan 0038` で PR-1 から実装する。
-   PR-5 のマージの後、#440 を閉じる（AC-27）。
