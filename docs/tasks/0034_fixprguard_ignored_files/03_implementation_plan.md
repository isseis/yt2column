# 実装計画書：git が無視するファイルのうち、後の実行に関わるものへの書き込みを fixprguard が検出する

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-22 |
| Review date | 2026-10-22 |
| Reviewer | isseis |
| Comments | 2026-10-22 (decision): `/runplan 0034` のフェーズ 1 のレビューで、APFS が完全な大文字小文字の変換（`ﬁ` → `fi`、`ß` → `ss`）で名前を解決すると分かった。照合は単純な変換のままとし、展開する変換で同じになる名前を要件書 4.2・設計書 5.3 の残余のリスクに加えた。ステップ 7-1 のガイド 7.2 は、この項目も含めて要件書 4.2 と突き合わせる。レビュアーがこのセッションで選んだので、`Status` は `approved` のままとする。 |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）の F-001〜F-004 を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。作業は設計書 8 章の 8 つの項目を 1 つずつフェーズにし、3 つの PR にまとめる（1.4）。

-   **フェーズ 1〜4（設計書 8 章の 1〜4）:** `internal/fixprguard/ignored.go` に、監視の一覧、列挙と受理する形、ディレクトリの項目のたどり、上限、各ファイルのエントリ、digest、記録の内容を置く。`cmdrunner` に標準エラーを失敗とする設定を加え、列挙の runner を `Worktree` に渡す。まだ `Baseline` と `CheckState` から呼ばないので、ツールの振る舞いは変わらない。
-   **フェーズ 5〜7（設計書 8 章の 5〜7）:** 基準値を `v3` にし、`CheckState` と `begin` に組み込む。`Protocol` を `4` にし、`fixpr.md`・`runpr.md`・ガイド・`check_rules.sh` の番号を揃える。ガイド（日英）を改め、文書のガードを加える。
-   **フェーズ 8（設計書 8 章の 8）:** 0030 の要件書と設計書の残余のリスクを改め、そのガードを加える。

用語は、要件書 6 章と設計書 1.4 の定義に従う（監視の一覧、対象のファイル、状態、項目、ディレクトリの項目、走査したエントリ、エントリ、記録のファイル）。本書の「フェーズ」は実装の区切りを指し、`/fixpr` の「修正のフェーズ」とは別である。「ガイド」は `docs/dev/developer_guide/fixprguard_guide.ja.md`（日本語版）と `fixprguard_guide.md`（英語版）を指す。「規則の名前」は設計書 4 章の表の `git-failed` の診断の名前を指す。

### 1.2. 実装原則

-   各ステップは設計書の節を参照し、設計の内容を本書で言い換えない。
-   Go のコメント・識別子・文字列リテラルは英語で書く。コマンドの文書（`.claude/commands/`）の文も英語で書く（`CLAUDE.md`「Language」）。例外は、日本語の文書の文字列と一致させる文書のガードのリテラルだけである（先例: `cmd/fixprguard/docs_test.go:535-550`）。
-   **`git-failed` のエラーは規則の名前だけから作る。** `ignored.go` と記録のファイルを書く関数が返すエラーは、OS のエラー（`*os.PathError` など）、runner のエラー、git の標準エラーを包まず、書式にも入れない。`stop` は標準エラーにエラーの連鎖の全体を書く（`cmd/fixprguard/main.go:288`）ので、包むとパスが出力に現れる（AC-15）。既存の `fileDigests`（`gitstate.go:492-501`）と `gitFailure`（`:468-476`）の形にならう。
-   **規則ごとに非公開の sentinel を置く。** 各規則の名前に、`ErrGitFailed` を包む非公開の sentinel を置き、テストは `errors.Is` で原因を区別する（`CLAUDE.md`「Error Testing」。先例: `errors.go` の `errConfigList`）。エラーの文言で区別しない。
-   **1 つの上限だけが働く入力を使う。** 1 つの上限を確かめる行は、ほかの上限と時間の上限を十分に大きくし、入力がほかの上限の内にあることを先に確かめる（`CLAUDE.md`「A layered path needs inputs only one layer can handle」）。
-   **時間の上限と、`lstat` と開く間の置き換えは、決まった結果が出るテストで確かめる。** そのための差し込みは、呼び出しごとに渡す値で行う。パッケージの変数を書き換えない（`-race` の下で、取り残した goroutine と競合しないように）。本番のコードにテストのための分岐を置かない。差し込みの具体的な形は実装時に決める（1.3.4）。
-   テストは対象の実装を壊して失敗することを確かめてからコミットし、壊し方と結果をコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」、設計書 7.3）。各ステップの「壊して確かめる」は、壊す対象と、失敗するはずのテストを挙げる。
-   テストは一時ディレクトリの実際のリポジトリで git を動かす（設計書 7 章）。テスト用のリポジトリでは、`core.excludesFile` を `HOME` の下に置いた空のファイルに固定し、`core.ignorecase` と `core.precomposeunicode` を `false` に固定する（ステップ 3-1）。macOS の一時ディレクトリが大文字小文字を区別しないボリュームにあっても、Linux の CI と同じ判定にするためである。
-   **権限のテストは root では失敗させ、モードを必ず戻す。** モードを変えるテストは、root で実行されたら黙って飛ばさずに失敗させる `requireNonRoot` と、`t.TempDir` の後に戻す `t.Cleanup` を登録する `chmodForTest` を使う（先例: `internal/transcript/test_helpers.go:310-333`）。`internal/fixprguard` には同じ補助がないので、`test_helpers_git.go` に置き、gosec の指摘には最小の範囲の `//nolint:gosec` と理由を付ける（`_test.go` でないので除外が効かない）。
-   **不正な UTF-8 の名前のファイルはディスクに作らない。** macOS の APFS は不正な UTF-8 の名前を拒否する（`Illegal byte sequence`。レビューで確かめた）。不正な UTF-8 のパスの行は、照合と記録の行の組み立てを、バイト列で直接確かめる。ディスクに作るのは改行と制御文字を含む名前までにする。
-   監視の一覧の照合のテストの行は、`ignored.go` のデータからではなく、要件書 3 章の表と例から 1 つずつ作る。
-   ガイドは、日本語版をコミットしてから英語版を `.claude/commands/mktrans.md` の原則で翻訳し、翻訳の後に日本語版と節ごとに突き合わせる（`CLAUDE.md`「Translation Guidelines」）。
-   Go のファイルを変えたら `make fmt` を実行する。各ステップの完了条件は、とくに書かない限り `make test && make lint` が通ることである。
-   ソースのコメントに `AC-NN`・`F-NNN` を書かない（要件書との対応は本書 5 章に書く）。

### 1.3. 既存コード調査結果

調査はコミット `c077836`（`origin/main`）で行った。設計書が確かめたコミット `9f83469` から `c077836` までの `git diff --stat 9f83469 c077836 -- internal/ cmd/ .claude/ docs/dev/ docs/tasks/0030_fixpr_untrusted_threads docs/tasks/0032_japanese_pr_title CLAUDE.md Makefile .github` に現れたのは、タスク 0036 の PR-1（#449）の 11 個のファイル（`fixpr.md`・`runpr.md`・`cmd/fixprguard/docs_test.go`・`main_test.go`・ガイドの日英・`check_rules.sh`・`scope.go`・`scope_test.go`・`types.go`・`worktree_test.go`）だけである。`internal/fixprguard/gitstate.go`・`worktree.go`・`cmd/fixprguard/main.go`・`internal/cmdrunner/` は変わっていないので、設計書のこれらの `file:line` はそのまま使える。以下の `file:line` は `c077836` のものである。

#### 1.3.1. 変える箇所の一覧

| 箇所 | 現状 | 変え方 | ステップ |
|---|---|---|---|
| `internal/fixprguard/scope.go:56-60` | 0036 が `goBuildInputSuffixes` に Go のビルドの入力の 17 種類を置いた | 変えない。監視の一覧の照合が参照する（設計書 3.2） | 1-1 |
| `internal/cmdrunner/runner.go:30-39`・`:78-110` | `Options` は `MaxOutput`・`WaitDelay`・`Dir`・`Env`。成功したら標準エラーを捨てる | `StderrIsFailure` と `ErrUnexpectedStderr` を加える（設計書 3.3・4 章） | 2-1 |
| `internal/fixprguard/worktree.go:51-61`（`Worktree`・`NewWorktree`） | git と make の 2 つの runner | 列挙の runner の欄と引数を加える（設計書 3.3） | 3-1 |
| `cmd/fixprguard/main.go:135-143`（`env`）・`:149-165`（`osEnv`）・`:350` | `env` に `git`・`make`。`NewWorktree(e.git, e.make)` | `env` に列挙の runner の欄を加え、`osEnv` で `StderrIsFailure` の設定で作り、`:350` で渡す | 3-1 |
| `NewWorktree` の呼び出し: `internal/fixprguard/test_helpers_git.go:223`、`cmd/fixprguard/test_helpers.go:114`・`:125` | 2 つの引数 | 列挙の runner を加える。`worktree_flow_test.go` には呼び出しはない（設計書 2.2 を editorial correction で改める。1.4） | 3-1 |
| `cmd/fixprguard/test_helpers.go:151-163`（`newTestRun` の `env{...}`） | `git: realGit()` | 列挙の runner の欄にも実際の git（`StderrIsFailure`）を入れる。入れないと欄が nil のまま、コンパイルは通り実行時に止まる | 3-1 |
| `cmd/fixprguard/main_test.go:172`・`:237`・`:334` | `r.git = cmdrunnertestutil.New(t)` で、使い方の誤りで止まる場合に、コマンドが実行されないことを確かめる | 列挙の runner も手順のない `Fake` に置き換える | 3-1 |
| `internal/fixprguard/test_helpers_git.go:104-127`（`newTestRepo`）、`cmd/fixprguard/test_helpers.go:42-60` | `HOME`・`XDG_CONFIG_HOME` は固定済み | `core.excludesFile`・`core.ignorecase`・`core.precomposeunicode` を固定する（1.2）。cmd 側も同じ形のリポジトリを作るので、両方に当てる | 3-1 |
| `internal/fixprguard/test_helpers_git.go:34-55`（`gitRouter`） | git を実際に動かし、`Calls` に記録する | 列挙の runner もこの router を通し、`Calls` に `git ls-files` を記録する（1.4） | 3-1 |
| `internal/fixprguard/gitstate.go:28-32`・`:112-153` | `baselineVersion = "v2"`、`baselineFields = 7`、`Baseline` の 7 つの欄 | `v3`・9 つの欄にする（設計書 3.6） | 5-1 |
| `gitstate.go:50-56` | `Part*` の 5 つの定数 | `PartIgnored` と `GitStateParts` を加える（設計書 3.1） | 5-1 |
| `gitstate.go:568-607`（`CheckState`）・`:742-787`（`Baseline`） | hooks の比較で終わる。7 つの欄を求める | 対象のファイルの状態を求めて比べる（設計書 3.6・3.7・3.8） | 5-1 |
| `internal/fixprguard/gitstate_test.go:324-355`（`TestBaselineMalformedRejected`） | `with(5, ...)`・`with(6, ...)` と、前の版を `v1` にする行（`:340`） | 前の版を `v2` と 7 つの欄の形にし、新しい欄の誤りの行を加える（設計書 2.2） | 5-1 |
| `cmd/fixprguard/test_helpers.go:381-390`（`validBaseline`）、`main_test.go:26`（`validArgs` の `"v2.x"`）、`main_test.go:309-345`（`TestPreviousBaselineRejected`） | `v2` と 7 つの欄 | `v3` と 9 つの欄。前の版は `v2` と 7 つの欄の形にする（AC-03） | 5-1 |
| `internal/fixprguard/gitstate_test.go:275`・`:375`・`:394`・`:551`・`:587`、`worktree_flow_test.go:311`・`:443` | git の呼び出しの列（`router.Calls`）を比べる | config か hooks の違いで止まる場合は、列挙より前に止まるので、期待は変わらない。一致する場合の列が変わるテストだけを直す | 5-1 |
| `worktree.go:80-85`（`Begin`）、`main.go:470-520`（`begin`） | `Baseline` の後に何もしない | `Baseline` の後に `WriteBeginRecord` を呼ぶ（設計書 3.8） | 5-2 |
| `internal/fixprguard/test_helpers_git.go:237-246`（`mustBaseline`） | `w.Begin` を呼ぶ | 変えない。ステップ 5-2 の後は、多くの既存のテストが `begin` の記録のファイルを書く。設計書 3.1 の「基準値を求めるだけのテストは git のディレクトリに書かない」は、`cmd` 側の `testRepo.Baseline`（`cmd/fixprguard/test_helpers.go:111-119`）にだけ当てはまるので、editorial correction で改める（1.4） | 5-2 |
| `internal/fixprguard/types.go:21` | `Protocol = 3` | `4`（設計書 3.9） | 6-1 |
| `.claude/commands/fixpr.md`（`--protocol 3` が 8 か所。`:44` は文）、`runpr.md`（4 か所。`:18` は文） | `--protocol 3` | `--protocol 4` | 6-1 |
| ガイド 4 章（各版 `:64`）・5 章（`--protocol 3` と `"protocol":3`、`'v2....'` が `:153`・`:165`・`:177`） | `3` と `'v2....'` | `4` と `'v3....'`（設計書 3.9） | 6-1 |
| `docs/tasks/0032_japanese_pr_title/check_rules.sh:524` | `--protocol 3` | `--protocol 4` | 6-1 |
| `.claude/commands/fixpr.md:255-262`（`begin` の止まり方）・`:492-493`・`:524-525`・`:663-664` | `parts` の値を `HEAD`・`the remote branch` などの言葉で列挙している。無視されるファイルの扱いがない | `ignored` を加え、無視されるファイルにかかわるすべての止まり方での扱いを書く（1.4） | 6-2 |
| `cmd/yt2column/fixpr_docs_test.go:180`（`TestFixprDocContract`）・`:151`（`requirePhrases`） | ツールが強制できない `fixpr.md` の文を確かめる | 無視されるファイルを一覧にしたり読んだりしない指示の語を加える | 6-2 |
| ガイド 1 章（各版 `:13`）・5 章（`begin` の副作用 `:136`、`inspect`・`finish`・`push` の節）・7.1（`:258-268`）・7.2（`:281`）・8 章（`:295`・`:304`）・9 章（`:313`・`:326`） | 対象のファイルの記述がない | 設計書 3.10 の表のとおり改める。8 章の (a)〜(e) は表の後の小見出しに置く（1.4） | 7-1・7-2 |
| `cmd/fixprguard/docs_test.go:91`（`guideTable`）・`:252`（`singleLine`）・`:269-292`（`section`・`subsection`・`cutSection`）・`:467-524`（`configScope`・`passage`・`checkPassages`） | 0036 と 0035 が置いたガードの部品 | 新しいガードで再利用する | 7-4・8-4 |
| `cmd/fixprguard/docs_test.go:526-576`（`TestGuideDescribesConfigScope`） | 1 章の層 (3)（`"3. **"` で始まる行がちょうど 1 つ）、5 章の `inspect`・`push`、7.1 の行、7.2、8 章の `git-state-changed` の行の語を確かめる | 変えない。改める段落でもこの語と行の形を保つ（設計書 2.2） | 7-1・7-2 |
| `cmd/yt2column/fixpr_docs_test.go:702`（`TestGlossaryTerms`） | fixprguard の用語の日英の組を確かめる | 加えた用語の組を加える | 7-3 |
| `docs/dev/developer_guide/package_reference.md:19`（`internal/cmdrunner`）・`:21`（`internal/fixprguard`） | `Options` の 4 つの欄を挙げる | `StderrIsFailure`・`ErrUnexpectedStderr` と、対象のファイルの比較と記録のファイルを加える（設計書 2.2 に editorial correction で加える。1.4） | 7-5 |
| `docs/translation_glossary.md:78` の近く | `基準値` はあるが、本タスクの用語はない | ガイドの翻訳で使った用語（監視の一覧、対象のファイル、記録のファイル）を加える（`mktrans.md`「Update Glossary」） | 7-3 |
| 0030 `01_requirements.md:201`・`02_architecture.md:847` | 無視されるファイルを残余のリスクとする | 設計書 3.10 のとおり改め、`Comments` に記録する | 8-2・8-3 |

変えない箇所は次のとおりである。

-   `internal/cmdrunner/testutil/mocks.go`。`Fake` は `Options` を持たず、`Step.Err` に `ErrUnexpectedStderr` を入れれば失敗を再現できる。本タスクのテストは列挙に実際の git を使う（設計書 7 章）。設計書 2.2 の表はこのファイルを「変更」とするので、editorial correction で改める（1.4）。
-   範囲の規則（`scope.go` の判定）と、`changes.go` の `splitNUL`・`readChanged`、`gitstate.go` の `hooksDigest`・`fileSHA256`（設計書 1.3 の「使わない」）。`gitstate.go:246-261` の `openRegular` は使う（ステップ 4-1）。
-   `cmd/fixprguard/main_test.go:270`（`TestBeginOrder`）。`begin` が `dirty-work-tree` で止まり、`gh` を起動しないことを確かめるテストで、`Baseline` に届かない。
-   `runpr.md` の `git-state-changed` の扱い（`:273-278` 付近）。番号だけを変える（設計書 2.2）。`runpr` は `fixpr` の報告を示して止まるだけで、無視されるファイルを調べないので、`fixpr.md` の禁止（1.4）は書き足さない。
-   0034 の要件書。設計書 3.10 の表の 0034 の行の変更（照合の記述、AC-19 と 4.2 の記録の記述）は、設計書の PR（#447）で済んでいる。要件書 3 章 `:116`・4.4 `:269` は Unicode の単純な大文字小文字の変換を、AC-19 `:214` は「書き換えれば分かる」記録を、4.2 `:233` は記録のファイルの digest を述べている。ステップ 8-1 で確かめ直す。

#### 1.3.2. 外部の前提の確かめ

-   **タスク 0036 との順序:** 0036 の PR-1（#449）は `c077836` で `main` に入り、`Protocol` は `3`、Go のビルドの入力の拡張子は `scope.go` の `goBuildInputSuffixes` にある。そのため、設計書 3.9 の「0036 が先」の場合に当たり、番号は `3` から `4` にする。設計書 3.9 の「本タスクが先に入る場合に 0036 に伝える手順」は当てはまらない。0036 の PR-2（ガイド 7.2・8 章、用語集、0030 の文書の注記）はまだ入っていない（7.1 のリスク）。
-   **タスク 0031 の開いている PR #448:** ガイド 4・5・7・9 章、`cmd/fixprguard/docs_test.go`・`main.go` を変える。`Protocol` は上げない（0031 `03_implementation_plan.md`「フェーズ 1 の完了の条件」）。番号の衝突はないが、同じ箇所を変えるので、後に入る PR が `main` を取り込んで競合を解く（7.1 のリスク）。
-   **git の振る舞い:** 列挙の出力の形、入れ子のリポジトリの項目、読めないディレクトリの警告と終了コード `0` は、設計書 2.4 が git 2.50.1 で確かめた。本書の時点の手元の git も 2.50.1 である。`core.untrackedCache` を有効にして `git status` でキャッシュを作った後でも、読めないディレクトリで `ls-files --others --ignored --exclude-standard -z` は `warning: could not open directory 'build/locked/': Permission denied` を書き、終了コード `0` を返した（2026-10-22、一時リポジトリで確かめた）。CI の git の版での振る舞いは、ステップ 3-2 のテストが CI で確かめる。
-   **標準ライブラリ:** `os.CreateTemp` はモード `0o600` で、同時に呼んでも同じ名前を選ばない（`go doc os.CreateTemp`）。`unicode.SimpleFold` は単純な大文字小文字の変換で同値な文字を巡回する（`go doc unicode.SimpleFold`）。手元の Go は 1.27.1、`go.mod` は `go 1.26.5` で、どちらの API も 1.26 にある。`rename(2)` は置き換える先のシンボリックリンクをたどらない（設計書 3.8）。
-   **SHA-256 のコマンド:** macOS に `/usr/bin/shasum` と `/sbin/sha256sum` がある（2026-10-22）。ガイドには設計書 3.8 のとおり両方を書き、ステップ 7-4 のテストで実行する。
-   **今の番号の固定箇所:** `c077836` で `git grep -nE -e '--protocol[ =]3' -e '"protocol":\s*3' -- .claude docs/dev docs/tasks/0032_japanese_pr_title` は 39 行を出力し、`sh docs/tasks/0032_japanese_pr_title/check_rules.sh fixpr_title` は `ok   fixpr_title` を出力して終了コード `0` で終わった（2026-10-22）。ステップ 6-1 の後は、前者が 0 行になる。
-   **`make deadcode` の範囲:** `Makefile` の `deadcode` は `deadcode -test -tags test ./cmd/yt2column` で、`cmd/yt2column` から届く関数だけを数える。そのため `internal/fixprguard` の関数はすべて報告される（`c077836` で 202 行。レビューで確かめた）。`make deadcode` は、グリーンゲート（`make test && make lint`）にも CI にも含まれない。PR-1 の `ignored.go` は PR-2 で使う部品なので、`runplan.md` step 6 の「意図した足場」として残す。`make lint` の `unused` は、テストからの使用を数える（`--build-tags test,integration`）。
-   **AC-01a の作業ツリー:** この作業ツリーには `extension/node_modules/` がない。ステップ 7-6 で、利用者の承認を得て `make ext-install`（ネットワークに接続する）を実行するか、`node_modules` がある main のチェックアウトで、PR-2 のブランチからビルドしたバイナリの `begin` を実行する（`CLAUDE.md`「Tool Execution Safety」）。

#### 1.3.3. 型の構築の経路

`Baseline` は、今までどおり非公開の欄と唯一の構築子 `newBaseline` を持ち、`construction_test.go:20` のガードが構築の経路を確かめる（設計書 3.1）。欄を加えても構築子は 1 つのままなので、ガードは変えない。新しい型 `targetState`・`targetLimits` は非公開で、型によって振る舞いを分ける設計ではない。上限の値は `defaultTargetLimits` だけから本番のコードに渡る（設計書 3.5）。そのため、構築の経路のガードは加えない。

#### 1.3.4. 実装時に決めること

-   `ignored.go` の非公開の関数の名前と分け方（列挙の検査、たどり、エントリの組み立て、記録の書き込み）。公開するものは設計書 3.1 のとおりである。
-   規則ごとの sentinel の名前（1.2）。
-   テストの関数の名前。5 章の表に書いた名前と変えたときは、同じ PR で本書の表を改める。
-   文書のガードが固定する語（書いた文から選ぶ）と、箇所を取り出すときの目印。ガイド 8 章の (a)〜(e) の小見出しの名前。
-   時間の上限と、`lstat` と開く間の置き換えを確かめるための差し込みの形（1.2 の条件を満たす、呼び出しごとの値の渡し方）。

### 1.4. 計画の際に決めたこと（2026-10-22）

-   **PR は 3 つにする（PR-1 = フェーズ 1〜4、PR-2 = フェーズ 5〜7、PR-3 = フェーズ 8）:** レビュアーが選んだ。理由: PR-1 は振る舞いを変えない部品だけなので、照合・形の検査・上限・読み取りを、それだけでレビューできる。PR-2 は、ツールが `ignored` で止まるようになる変更と、その対処の手順（ガイド 8 章）を同じ PR に入れる。そのため、`main` に止まり方だけが入り、手順がない期間ができない。`Protocol` の引き上げは、振る舞いを変える PR-2 に入れる（設計書 3.9）。PR-3 は過去のタスクの文書だけの変更である。受け入れる影響: PR-2 は、Go の組み込みとガイドの日英の改訂を含むので大きい。
-   **`fixpr.md` の禁止を、無視されるファイルにかかわるすべての止まり方に当てはめる（決定の変更）:** レビュアーが選んだ。設計書 3.10 の `fixpr.md` の行は、`ignored` で止まったときだけ、セッションが無視されるファイルを一覧にしたり読んだりすることを禁じる。本タスクの後は、`begin` も `WriteBeginRecord` の不一致で `ignored` で止まり、`begin`・`inspect`・`finish`・`push` は無視されるファイルの規則（設計書 4 章の表の `list ignored files …` 以下）で `git-failed` で止まる。そのどれでも、セッションが `git ls-files -o -i` などで原因を調べると、攻撃者が選んだファイル名がセッションに届く（AC-15 の目的）。そのため、これらのすべての止まり方で禁止し、ガイド 8 章の手順を案内するよう書く。設計書 3.10 の `fixpr.md` の行と 2.2 の表の行を、この計画の PR で改め、設計書の `Comments` に決定として記録する。決定はこのセッションでレビュアーが選んだので、設計書の `Status` は `approved` のままとする。
-   **ガイド 8 章の (a)〜(e) は、表の後の小見出しに置く（editorial correction）:** レビュアーが選んだ。8 章の表の `git-state-changed` の行は 1 行のセルで、`TestGuideDescribesConfigScope` もこの行を 1 行として読む（`singleLine`）。そこに複数行のコマンドは入らない。そこで、行には `parts` の `ignored` の意味と小見出しへの参照だけを書き、(a)〜(e) とコマンドは表の後の小見出しに置く。手順の中身は設計書 3.10 のとおりで、置き場所だけが変わるので、設計書 3.10 を editorial correction で改める。
-   **設計書のほかの箇所を editorial correction として改める（この計画の PR）:** 設計書 2.2 の表で、(1) `internal/cmdrunner/testutil/mocks.go` を変えないことにする、(2) `NewWorktree` の呼び出しの行を実際の呼び出しのあるファイルにし、`env`・`newTestRun` と cmd 側のテスト用のリポジトリにも当てる、(3) `check_rules.sh` の行の今の番号を `3` にする、(4) `package_reference.md` と `docs/translation_glossary.md` の行を加える。設計書 3.1 の「基準値を求めるだけのテスト」の例を `cmd` 側の `testRepo.Baseline` に改める（1.3.1）。どれも作るものと理由を変えないので、`Status` は `approved` のままにし、設計書の `Comments` に記録する。
-   **列挙の runner の配線は PR-1 に入れる:** 設計書 3.1 は `collectTargets` を `Worktree` のメソッドとする。そのため、列挙の runner の欄と `NewWorktree` の引数、`env` と `osEnv` の配線をステップ 3-1 で加える。`Baseline` と `CheckState` から呼ぶのは PR-2 なので、PR-1 の振る舞いは変わらない。本番の列挙の runner の設定（`StderrIsFailure`・`MaxOutput`・`Env`）は、PR-1 のテストで確かめる。
-   **列挙の runner をテストの `gitRouter` に通す:** `router.Calls` に `git ls-files` が現れるので、config・hooks が違うときに列挙を起動しないこと（設計書 3.7 の順序）を、既存の `router.Calls` の比較で確かめられる。
-   **AC-01a は、`test` と `manual` で確かめる:** 既定の上限で、依存のパッケージに似た無視されるディレクトリの `begin` が止まらないことを単体テストで確かめる。実際の `extension/node_modules/` での確認は、ステップ 7-6 で手で行う（設計書 7.4）。CI には `node_modules` がない場合がある。
-   **ガイド 8 章のコマンドを、テストで実行する:** 設計書 3.8・3.10 の (a)・(b) のコマンドは、ガイドの中のコードブロックとして書く。`cmd/fixprguard` のテストが、日英のガイドからそのブロックを取り出し、両版で同じであることを確かめ、`gh` を fake にして `begin` と `inspect` を実行して作った記録のファイルに対して、ブロックのコマンドを `sh` で実行する。こうすると、ガイドの後の編集もテストが確かめる。
-   **AC-17〜AC-20a は、文書のガードと PR のレビューで確かめる:** 設計書 7.4 の 3 つのガードに加え、AC-18（7.2 の分けた項目）、AC-19 の (a)〜(e)、AC-19a（`git-failed` の行）、AC-20・AC-20a（0030 の文書）にも、語の有無を確かめるガードを加える。0035 の `passage`・`checkPassages` の形を使う。ガードは語の有無だけを確かめるので、中身は PR のレビューで読み合わせる（`manual`）。

### 1.5. 申し送りの項目への対応

-   `design_handoff.md` の H-01〜H-08 は、すべて設計書の付録 A に記録されている。
-   `implementation_handoff.md` はない。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 監視の一覧（設計書 8 章の 1）

#### ステップ 1-1: 監視の一覧のデータと照合

**変更するファイル**: `internal/fixprguard/ignored.go`（新規）、`internal/fixprguard/ignored_test.go`（新規）

-   [x] 監視の一覧のデータを設計書 3.2 の表のとおり置く。Go のビルドの入力の拡張子は `scope.go` の `goBuildInputSuffixes` を参照する。
-   [x] `isWatched` を、設計書 3.2「同一視の仕方」のとおり実装する。`strings.EqualFold` と `strings.ToLower` は使わない。
-   [x] `WatchListPatterns` を設計書 3.2「ガイドの表記」の形で返す。判定と同じデータから組み立てる。
-   [x] `TestIsWatched` を加える。行は設計書 7.1「監視の一覧の照合」の項目に従い、要件書 3 章の当たる例と当たらない例を 1 つずつ行にする（1.2）。不正な UTF-8 の行はバイト列で書く。
-   [x] `TestWatchListPatternsMatch` を加える。`WatchListPatterns` の各項目から作ったパスが `isWatched` に当たること（先例: `scope_test.go:229` の `samplePath`）と、項目の数が要件書 3 章の表の数（1 + 3 + 5 + 2 + 17 = 28）であることを確かめる。

**壊して確かめる**: 次の変更を 1 つずつ加えると、括弧の中のテストが失敗すること。

-   データから 1 つの項目を消す。`.claude`・最上位のファイル・名前・拡張子・Go のビルドの入力の拡張子のそれぞれで 1 つずつ（`TestIsWatched`）。
-   `WatchListPatterns` から 1 つの項目を落とす（`TestWatchListPatternsMatch`）。
-   同一視を ASCII だけにする（`TestIsWatched` の KELVIN SIGN と LONG S の行）。
-   不正なバイトを U+FFFD とみなす（`TestIsWatched` の不正なバイトだけが違う 2 つのパスの行）。

### フェーズ 2: `cmdrunner` の標準エラーの設定（設計書 8 章の 2）

#### ステップ 2-1: `StderrIsFailure` と `ErrUnexpectedStderr`

**変更するファイル**: `internal/cmdrunner/runner.go`、`internal/cmdrunner/runner_test.go`

-   [ ] `Options` に `StderrIsFailure` を加え、成功したコマンドが標準エラーに書いたら `ErrUnexpectedStderr` を返す（設計書 3.3・4 章）。エラーの文言に標準エラーの内容を入れない。ゼロ値の振る舞い（今の振る舞い）は変えない。
-   [ ] `TestStderrIsFailure` を加える。設定ありで標準エラーに書いて成功するコマンドが `ErrUnexpectedStderr` になり、エラーの文言に標準エラーのバイト列がないこと。標準エラーが空なら標準出力を返すこと。設定ありで終了コードが 0 でなければ、今までどおり `*ExitError` になること。上限を超えれば `ErrOutputTooLarge` になること。設定なしなら今までどおり成功すること。

**壊して確かめる**: 標準エラーの検査を外すと、`TestStderrIsFailure` の該当の行が失敗すること。

### フェーズ 3: 列挙・受理する形・たどり・上限（設計書 8 章の 3）

#### ステップ 3-1: 列挙の runner の配線とテスト用のリポジトリ

**変更するファイル**: `internal/fixprguard/worktree.go`、`cmd/fixprguard/main.go`、`internal/fixprguard/test_helpers_git.go`、`cmd/fixprguard/test_helpers.go`、`cmd/fixprguard/main_test.go`

-   [ ] `Worktree` に列挙の runner の欄を加え、`NewWorktree` の引数にする（設計書 3.3）。
-   [ ] `env` に列挙の runner の欄を加える。`osEnv` で `StderrIsFailure`・`MaxOutput: gitMaxOutput`・`WaitDelay`・`Env: gitEnv` で作り、`NewWorktree` に渡す。ほかの git の runner には `StderrIsFailure` を付けない（設計書 3.3）。
-   [ ] 1.3.1 の表のテストの呼び出し（`NewWorktree`・`newTestRun`・`main_test.go` の 3 か所）を直す。列挙の runner は `gitRouter` を通す（1.4）。
-   [ ] 1.2 の 3 つの git の設定を `newTestRepo`（両パッケージ）で固定し、権限のテストの補助（`requireNonRoot`・`chmodForTest`）を `test_helpers_git.go` に加える（1.2）。
-   [ ] `TestListRunnerRejectsStderr` を `cmd/fixprguard/main_test.go` に加える。`TestGhRunsWithoutGitVariables`（`main_test.go:249`）にならい、`PATH` に置いた代わりの `git` で 2 つの場合を確かめる。(1) 標準エラーに 1 バイト書いて `0` で終わると、`osEnv()` の列挙の runner が `ErrUnexpectedStderr` を返すこと。(2) 代わりの `git` が環境変数を標準出力にだけ書いたとき、`osEnv()` の列挙の runner の出力に `GIT_DIR=` がないこと。
-   [ ] `gitRouter` に、列挙のための 2 つめの実際の runner（`StderrIsFailure` 付き）を持たせ、`git ls-files` はそれで動かす。テストはこの runner を差し替えられる（ステップ 3-2 の上限の行）。

**壊して確かめる**: `osEnv` の列挙の runner から `StderrIsFailure` を外すと (1) が、`Env: gitEnv` を外すと (2) が、`TestListRunnerRejectsStderr` で失敗すること。

#### ステップ 3-2: 列挙と受理する形

**変更するファイル**: `internal/fixprguard/ignored.go`、`ignored_test.go`

-   [ ] 列挙を設計書 3.3「列挙の手段」のとおり、`gitHardening` を付けて列挙の runner で起動する。失敗と出力の上限は `gitFailure` の分け方にならうが、エラーは規則ごとの sentinel にする（1.2）。時間切れは `read ignored files timed out` にする（設計書 4 章。`gitRaw` は git の runner と `gitTimeout` を使うので使わない）。
-   [ ] 受理する形の検査を、設計書 3.3「受理する形」のとおり、出力のバイト列に対して行う。`splitNUL` は使わない（設計書 1.3）。
-   [ ] `TestParseIgnoredListing` を加える。設計書 7.1「列挙の形の検査」の各例と、正しい出力・ディレクトリの項目の受け付けを、git を起動せずにバイト列で確かめる。
-   [ ] `TestCollectTargetsGitFailures` を加える。`build/` の下と `.claude/` の下にモード `0311` のディレクトリを置くと、警告の規則の sentinel で止まること（設計書 7.1「git の警告」。`.claude/` はテスト用のリポジトリの `.gitignore` で無視させてから置く）。列挙の git が 0 でない終了コードで終わる場合、起動できない場合、出力が runner の上限を超える場合（小さな `MaxOutput` の runner を渡す）も、それぞれ対応する規則の sentinel で止まること。

**壊して確かめる**: 形の検査の各条件を 1 つずつ外すと、`TestParseIgnoredListing` の対応する行が失敗すること。テストの列挙の runner から `StderrIsFailure` を外すと、`TestCollectTargetsGitFailures` の警告の行が失敗すること。

#### ステップ 3-3: ディレクトリの項目のたどりと `.claude/settings.local.json`

**変更するファイル**: `internal/fixprguard/ignored.go`、`ignored_test.go`

-   [ ] ディレクトリの項目の下を、設計書 3.3「ディレクトリの項目」のとおりたどる。
-   [ ] 列挙の結果を監視の一覧で選び、それとは別に `.claude/settings.local.json` を必ず加える（設計書 3.3）。対象のファイルをパスのバイト列の順に並べる。
-   [ ] `TestCollectTargetsWalk` を加える。入れ子のリポジトリの中の `.go` が対象になること、入れ子のリポジトリの中の FIFO と監視の一覧に当たらないファイルが対象にならないこと、`.claude/settings.local.json` が無視されていなくても（存在しなくても）必ず含まれ、列挙にも現れたときに 1 つになること、入れ子のリポジトリの中の読めないディレクトリで止まること。

**壊して確かめる**: たどりを外すと入れ子のリポジトリの `.go` の行が、`.claude/settings.local.json` を必ず加える処理を外すと存在しない場合の行が、`TestCollectTargetsWalk` で失敗すること。

#### ステップ 3-4: 上限と時間の上限

**変更するファイル**: `internal/fixprguard/ignored.go`、`ignored_test.go`

-   [ ] `targetLimits` と `defaultTargetLimits` を設計書 3.1・3.5 の表の値で置く。対象のファイルの数と、ディレクトリの項目の下の走査したエントリの数の上限を、列挙とたどりに当てはめる。列挙の出力の上限は、ステップ 3-1 の runner の `MaxOutput` である。
-   [ ] 状態を求める処理の全体が終わるのを、時間の上限まで待つ（設計書 3.5「時間の上限」）。結果を受け取るチャネルはバッファ付きにし、待ちを打ち切った後に goroutine が詰まらないようにする。
-   [ ] `TestCollectTargetsLimits` を加える。各上限の行は、ほかの上限を十分に大きくし、入力がほかの上限の内にあることを先に確かめる（1.2）。時間の上限の行は、処理の中で待つ関数を差し込む（差し込み方は 1.2）。待つ関数は `t.Cleanup` で閉じるチャネルを待ち、自身でも数秒で戻る。時間の上限の sentinel が返ることを確かめ、経過時間は緩く確かめる。

**壊して確かめる**: 各上限の検査を 1 つずつ外すと、`TestCollectTargetsLimits` の対応する行が失敗すること。時間の上限の待ちを外すと、時間の上限の行が（差し込んだ関数が戻るまで数秒待って）失敗すること。

### フェーズ 4: エントリ・digest・記録の内容（設計書 8 章の 4）

#### ステップ 4-1: 各ファイルのエントリと読み取り

**変更するファイル**: `internal/fixprguard/ignored.go`、`ignored_test.go`

-   [ ] 設計書 3.4 の 2 つの表のとおり、対象のファイルとリンクのたどる先のエントリを作る。値は `writeField`（`threads.go`）で区切る。内容は `openRegular`（`gitstate.go:246`）で開き（対象のファイルは `follow` なし、たどる先は `follow` あり）、返るエラーは規則の sentinel に置き換える（1.2）。上限に 1 を足した量までしか読まない（設計書 3.4「内容の読み方」）。
-   [ ] 1 つのファイルの大きさと、読み取る内容の合計の上限を、読み取りに当てはめる（設計書 3.5）。合計は、読んだバイト数をそのまま足す。
-   [ ] `TestTargetEntries` を加える（設計書 7.1「上限と読み取り」）。読めないファイル、ディレクトリを指すリンク、`EACCES` で調べられないたどる先（親のディレクトリのモードを 0 にする）、大きさの上限、同じ疎ファイルを指す多数のリンクで合計の上限を超える場合。いずれも、対応する規則の sentinel が返ることを確かめる。`ENOENT`・`ELOOP`・`ENOTDIR` のたどれないリンクは、エラーにならず `unresolved` として記録されることを確かめる。
-   [ ] `lstat` と開く間の置き換えの行を加える。`lstat` の後、開く前に呼ぶ関数を差し込み（差し込み方は 1.2）、通常のファイルを FIFO に置き換える。時間の上限を短く設定して実行し、`read ignored file` の sentinel が返ること（時間の上限の sentinel でないこと）を確かめる。`t.Cleanup` で FIFO の書く側を開き、壊したときに読み取りが戻るようにする。

**壊して確かめる**: 次の変更を 1 つずつ加えると、`TestTargetEntries` の括弧の中の行が失敗すること。

-   合計の上限の検査を外す（多数のリンクの行）。
-   `openRegular` を `os.Open` に替える（置き換えの行。時間の上限の sentinel が返る）。
-   `unresolved` を止まる扱いにする（たどれないリンクの行）。

#### ステップ 4-2: まとめた digest と記録の内容

**変更するファイル**: `internal/fixprguard/ignored.go`、`ignored_test.go`

-   [ ] `collectTargets` を、設計書 6.2 の流れのとおり、`targetState`（まとめた digest と記録の内容）を返すメソッドにまとめる（設計書 3.1・3.4）。
-   [ ] 記録の内容を設計書 3.8「形」のとおり作る。パスは `strconv.QuoteToASCII` で引用する。
-   [ ] `TestTargetStateDigest` を加える。エントリの境目（リンクの文字列やパスに、ほかのエントリの値に見える文字列を入れても digest が衝突しない）と、同じ状態からは同じ digest になること。状態の変化ごとの行はステップ 5-3 の `TestCheckStateIgnoredChanges` に置き、ここでは繰り返さない。
-   [ ] `TestTargetRecordLines` を加える。各行が、64 文字の 16 進数、空白、引用したパス（ASCII の印字できる文字だけ）からなり、パスのバイト列の順に並ぶこと。改行・制御文字を含むパス（ディスクに作る）と、不正な UTF-8 のパス（行の組み立てをバイト列で直接確かめる。1.2）も 1 行になること。1 つの対象のファイルの内容を変えると、その行の digest だけが変わること。
-   [ ] `TestDefaultLimitsAcceptDependencyTree` を加える。既定の上限で、依存のパッケージに似た無視されるディレクトリ（監視の一覧に当たらないファイル数千個と、C のヘッダ・`Makefile`）が止まらないこと（AC-01a。1.4）。
-   [ ] 本書 5 章の表の、フェーズ 1〜4 のテストの名前を、実際の名前に合わせる。

**壊して確かめる**: エントリから種類の印か値の数を外し、その印だけが区別する入力の行で、`TestTargetStateDigest` が失敗すること。記録のパスを `strconv.Quote` に替えると、`TestTargetRecordLines` の不正な UTF-8 と制御文字の行が失敗すること。

### PR-1 作成ポイント: ignored-file state machinery

**対象ステップ**: 1-1 / 2-1 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2

**推奨タイトル**: `feat(0034): fixprguard に、git が無視するファイルのうち監視の一覧に当たるものの状態を求める処理を加える`

**レビュー観点**: 監視の一覧の照合が Unicode の単純な大文字小文字の変換で同一視し、不正なバイトを区別すること / 列挙の受理する形の検査と git の警告で、補正も読み飛ばしもせずに止まり、エラーにパスや git の標準エラーを含めないこと / 各上限と時間の上限が、内容を読み続けずに止まり、FIFO で待たないこと / 本番の列挙の runner の設定と、まだ `Baseline`・`CheckState` から呼ばないこと

**実装モデル要件**: frontier-required

**判定理由**: ステップ 3-2〜4-1 は、信頼できない入力（git の出力、攻撃者が選べるファイルの名前・種類・大きさ）の境界と、0030 の守りの層 (3) に加える検出の部品である（`mkplan.md` step 8 の panel mode の契機のセキュリティのゲート）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 基準値と比較への組み込み（設計書 8 章の 5）

フェーズ 5 と 6 は、同じ PR で `main` に入れる（設計書 3.9）。

#### ステップ 5-1: 基準値の形と `CheckState` での比較

基準値の形を変えると、`Baseline` が新しい欄を求めない限り往復のテストが壊れるので、形と比較と既存のテストの修正を 1 つのステップにする。

**変更するファイル**: `internal/fixprguard/gitstate.go`、`internal/fixprguard/ignored.go`、`internal/fixprguard/gitstate_test.go`、`internal/fixprguard/worktree_flow_test.go`、`cmd/fixprguard/test_helpers.go`、`cmd/fixprguard/main_test.go`

-   [ ] `baselineVersion` を `v3`、`baselineFields` を 9 にし、`Baseline`・`newBaseline`・`ParseBaseline`・`String` に `ignored`・`ignoredRecord` を加える（設計書 3.6）。`PartIgnored` と `GitStateParts` を加える（設計書 3.1）。
-   [ ] `Baseline` の最後で `collectTargets` を呼び、`ignored` と `ignoredRecord`（記録の内容の SHA-256）を基準値に入れる（設計書 3.6・3.8）。
-   [ ] `CheckState` の hooks の比較の後で `collectTargets` を呼び、`ignored` と比べる（設計書 3.7）。違えば止まった時点の記録のファイルを書き、`GitStateError{Parts: []string{PartIgnored}}` を返す。記録を書けなくても `stop` は変えない（設計書 3.8）。状態を求められなければ、規則の sentinel を返す。
-   [ ] 記録のファイルの書き込みを、設計書 3.8「書き方」のとおり実装する。置き場所は `locateGitDirs` の作業ツリーごとの git のディレクトリ（設計書 1.2）。
-   [ ] `CheckState` と `Baseline` のコメントに、対象のファイルを最後に比べることと、その理由（設計書 3.7「順序の理由」）を英語で書く。
-   [ ] 1.3.1 の表のステップ 5-1 の既存のテストを直す。期待の変え方は表のとおりで、ほかの期待は変えない。

**壊して確かめる**: `TestBaselineMalformedRejected` の新しい欄の行が、`ParseBaseline` の新しい欄の検査を外すと失敗すること。

#### ステップ 5-2: `begin` の記録のファイル

**変更するファイル**: `internal/fixprguard/ignored.go`、`internal/fixprguard/worktree.go`、`cmd/fixprguard/main.go`

-   [ ] `WriteBeginRecord` を、設計書 3.8「`begin` の記録と基準値の結び付け」のとおり実装する。一致しなければ書かずに `git-state-changed`（`ignored`）を、書けなければ `write ignored begin record` の sentinel を返す。
-   [ ] `Worktree.Begin` と `main.go` の `begin` で、`Baseline` の後に `WriteBeginRecord` を呼ぶ。`begin` は記録を書き終えてから基準値を出力する（設計書 3.8「書き方」）。

#### ステップ 5-3: 組み込みのテスト

**変更するファイル**: `internal/fixprguard/ignored_test.go`、`internal/fixprguard/worktree_flow_test.go`、`cmd/fixprguard/main_test.go`

-   [ ] `TestCheckStateIgnoredChanges` を加える。設計書 7.1「状態の比較」の各変化で `parts` が `ignored` だけであること、変化しない 2 つの場合で止まらないこと。AC-13 は 3 つの行にする: 新しい最上位の `.gitignore`、下の階層の `.gitignore`（自分自身を無視するものを含む）、`core.excludesFile` のファイルへの追記。いずれもそこで無視される `.go` を作る。
-   [ ] 同じテストに、hooks（または config）と対象のファイルが同時に変わる行を加え、`parts` が `hooks` だけで、止まった時点の記録が書かれず、`router.Calls` に `git ls-files` がないことを確かめる（設計書 3.7）。
-   [ ] `TestBaselineIgnoredFailures` を加える。`Baseline` の中で状態を求められないとき（読めない対象のファイルなど）、規則の sentinel を返し、基準値を返さないこと。
-   [ ] `TestBeginWithTargets` を加える（設計書 7.1「`begin` が止まらないこと」）。
-   [ ] `TestIgnoredRecordFiles` を加える（設計書 7.1「記録のファイル」）。`Baseline` の後に対象のファイルが変わると `WriteBeginRecord` が書かずに止まること。止まった時点の記録が書かれること。記録の名前にディレクトリを置いて `rename` を失敗させると、`WriteBeginRecord` は `write ignored begin record` の sentinel を返し、`CheckState` は `parts` が `ignored` のままで、どちらも一時ファイルを残さないこと。記録の名前にシンボリックリンクを置くと、リンクの先が変わらないこと。linked worktree（`r.AddWorktree`、`test_helpers_git.go:167`）では記録が `.git/worktrees/<name>` に書かれること。
-   [ ] `worktree_flow_test.go` に `TestFinishStopsOnIgnoredChange` と `TestPushStopsOnIgnoredChange` を加える（設計書 7.2）。`finish` の前に書くと build checks を 1 つも実行しないこと（AC-09）。make の fake に `fmt` と `test` だけを手順として置き、`test` の `Effect` で対象のファイルを書くと、`make lint` を呼ばずに止まり、commit を作らないこと（AC-10）。`finish` の後に書くと `push` が push しないこと（AC-11）。どれも `parts` が `ignored` だけであること。
-   [ ] `cmd/fixprguard/main_test.go` に次のテストを加える。
    -   `TestBeginWritesRecord`: `gh` を fake にした `begin` の後、`.git/fixprguard-ignored-begin` があり、その SHA-256 が出力の基準値を `.` で分けた 10 番目と一致すること。記録を書けないとき（記録の名前にディレクトリ）、`begin` が終了コード `1` で止まり、基準値を出力しないこと。
    -   `TestRoundWithUnchangedTargets`: 対象のファイルがあり変わらないとき、`inspect`・`finish`・`push` の出力の JSON が、対象のファイルがないときの同じ実行と、基準値・commit の SHA を除いて同じ欄と値を持つこと（AC-02）。
    -   `TestIgnoredOutputOmitsNames`: 名前と内容に識別できる文字列を入れた対象のファイルで、`ignored` で止まる場合と、`git-failed` で止まる各場合（読めないファイル、読めないディレクトリ、ディレクトリを指すリンク、大きさの上限、git の警告）に、標準出力・標準エラー・基準値に、その文字列も、対象のファイルごとのエントリの digest（記録のファイルに書く値をテストで求めたもの）も現れないこと（AC-15。`TestConfigStopOmitsNames`（`main_test.go:347`）にならう）。

**壊して確かめる**: 次の変更を 1 つずつ加えると、括弧の中のテストが失敗すること。

-   `CheckState` から対象のファイルの比較を外す（`TestCheckStateIgnoredChanges`、`TestFinishStopsOnIgnoredChange`、`TestPushStopsOnIgnoredChange`）。
-   対象のファイルの比較を config の一覧の比較の前に移す（`TestGitNotLaunchedAfterConfigChange`（`gitstate_test.go:367`）と、同時に変わる行）。
-   `finish` の `make test` の後の `CheckState` を外す（`TestFinishStopsOnIgnoredChange` の AC-10 の行。`make lint` の呼び出しで fake が失敗する）。
-   `main.go` の `begin` から `WriteBeginRecord` の呼び出しを外す。`String` の 9 番目と 10 番目の欄を入れ替える（`TestBeginWritesRecord`）。
-   `WriteBeginRecord` の一致の検査を外す（`TestIgnoredRecordFiles`）。
-   記録を `rename` でなく `os.WriteFile` で書く（`TestIgnoredRecordFiles` のシンボリックリンクの行）。
-   規則の sentinel の代わりに OS のエラーを `%w` で包む（`TestIgnoredOutputOmitsNames`）。

### フェーズ 6: `Protocol` と `fixpr.md`（設計書 8 章の 6）

#### ステップ 6-1: `Protocol` を `4` に上げる

ガードが `Protocol` と文書の番号の一致を求めるので、このステップは 1 つのコミットにする（設計書 3.9「同じ commit に入れる」）。

**変更するファイル**: `internal/fixprguard/types.go`、`.claude/commands/fixpr.md`、`.claude/commands/runpr.md`、`docs/dev/developer_guide/fixprguard_guide.ja.md`、`fixprguard_guide.md`、`docs/tasks/0032_japanese_pr_title/check_rules.sh`

-   [ ] `Protocol` を `4` にする。コメントは変えない（`types.go:17-20` は契約と範囲の規則の版と述べており、基準値の形と `parts` は契約に含まれる）。
-   [ ] 1.3.1 の表の箇所の番号を、すべて `4` にする。ガイドの基準値の例 `'v2....'` を `'v3....'` にする。1.3.2 の `git grep` が 0 行になることと、`check_rules.sh fixpr_title` が通ることを確かめ、結果をコミットメッセージに書く。

**壊して確かめる**: ガイドの `--protocol 4` の 1 か所を一時的に `3` に戻して、`TestDocumentedProtocolMatches` が失敗すること。

#### ステップ 6-2: `fixpr.md` の無視されるファイルの扱い

**変更するファイル**: `.claude/commands/fixpr.md`、`cmd/yt2column/fixpr_docs_test.go`

-   [ ] `parts` を列挙する 3 か所（1.3.1）に `ignored` を加える。
-   [ ] `begin`・`inspect`・`finish`・`push` が `git-state-changed`（`parts` に `ignored`）か、無視されるファイルの規則の `git-failed` で止まったとき、セッションは無視されるファイルを一覧にしたり読んだりしない（`git status --ignored`・`git ls-files -o -i`・`ls .claude` などを実行しない）と書く。代わりに、利用者にガイド 8 章の手順を案内するよう書く（1.4）。`begin` の止まり方（`:258-262`）にも同じ内容を書く。`git-failed` のうち無視されるファイルにかかわるものは、標準エラーの診断の規則の名前（設計書 4 章の `ignored` を含む名前と `list ignored files` で始まる名前）で見分けると書く。文は英語で書く。
-   [ ] `TestFixprDocContract`（`cmd/yt2column/fixpr_docs_test.go:180`）に、この指示の語を `requirePhrases` で確かめる行を加える。

**壊して確かめる**: `fixpr.md` から禁止の文を消すと、`TestFixprDocContract` が失敗すること。

### フェーズ 7: ガイドと文書のガード（設計書 8 章の 7）

#### ステップ 7-1: ガイドの日本語版

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`

-   [ ] 設計書 3.10 の表の、ガイドの 1 章・5 章・7.1・7.2・8 章の各行のとおり改める。7.1 の「監視の一覧」の小見出しの下の表は、`WatchListPatterns` と同じ表記にする。8 章の `git-state-changed` の行には `ignored` の意味と小見出しへの参照を書き、(a)〜(e) とコマンドは表の後の小見出しに置く（1.4）。(a)・(b) のコマンドは 1 つのコードブロックにまとめる（ステップ 7-4 のテストが取り出す）。`git-failed` の行は、設計書 4 章の規則の名前ごとの原因と対処を書く。
-   [ ] `TestGuideDescribesConfigScope` が確かめる語と、`3. **` で始まる行が 1 つであることを保つ（1.3.1）。
-   [ ] 書いた 7.2 の項目を、要件書 4.2 の残余のリスクと設計書 5.3 と突き合わせる。

#### ステップ 7-2: 9 章の `Protocol` の規則

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`

-   [ ] 9 章に、`Protocol` の番号は上げるだけで、使った番号を再び使ったり下げたりしないことを書く（設計書 3.9・3.10）。監視の一覧を変えたら 7 章の表も変えることを、一覧の更新の項目（`:326`）に加える。`TestGuideDescribesReviewerList` が確かめる 9 章の但し書きは変えない。

#### ステップ 7-3: ガイドの英語版と用語集

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.md`、`docs/translation_glossary.md`、`cmd/yt2column/fixpr_docs_test.go`

-   [ ] ステップ 7-1・7-2 の日本語版をコミットしてから、英語版の同じ節を翻訳する（1.2）。翻訳の後に、日本語版と節ごとに突き合わせる。8 章のコードブロックは日本語版と同じバイト列にする。
-   [ ] 翻訳で使った用語のうち、用語集にないもの（監視の一覧、対象のファイル、記録のファイル）を用語集に加え、`TestGlossaryTerms`（`cmd/yt2column/fixpr_docs_test.go:702`）に日英の組を加える。

#### ステップ 7-4: 文書のガード

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [ ] `TestGuideListsWatchList` を加える。日英の 7.1 の「監視の一覧」の小見出しの後の表を `guideTable` で切り出し、表のコードスパンの集合が `WatchListPatterns` の集合と一致することを確かめる（足りない項目も、余分な項目も失敗にする）。7.1 の保証の行があることも確かめる（AC-17）。
-   [ ] `TestGuideNamesGitStateParts` を加える。日英の 8 章の `git-state-changed` の行を `singleLine` で取り出し、`GitStateParts` のすべての値がコードスパンであることを確かめる。
-   [ ] `TestGuideDescribesIgnoredFiles` を加える。`passage`・`checkPassages` の形で、日英の 7.2 の分けた項目（AC-18）、8 章の小見出しの (a)〜(e) の語と 2 つの SHA-256 のコマンド（AC-19）、`git-failed` の行の原因の語（AC-19a）を確かめる。`checkPassages` は `configScope` の 3 つの語を必ず求めるので、0036 の PR-2 が同じ関数を改めていればその形を使い、そうでなければ確かめる語の一覧を渡せる形に改めて使う（`TestGuideDescribesConfigScope` の語は変えない）。
-   [ ] `TestGuideIgnoredCommands` を加える（1.4）。日英の 8 章の小見出しのコードブロックを取り出し、両版で同じであることを確かめる。`gh` を fake にした `begin` と、対象のファイルを変えた後の `inspect` で 2 つの記録のファイルを作り、ブロックのコマンドを `sh` で実行する。照合が一致し、`diff` が変えたパスを示すこと、`begin` の記録を書き換えると照合が一致しないことを確かめる。`shasum` と `sha256sum` のうち使える方で実行し、どちらも使えなければ失敗させる。
-   [ ] `fixpr.md` の `parts` を列挙する文に `` `ignored` `` があることを、`TestGuideNamesGitStateParts` か別の小さなテストで確かめる（設計書 3.10）。

**壊して確かめる**: 次の変更を 1 つずつ加えると、それぞれのガードが失敗すること。一時的に書き換えて確かめ、戻す。

-   どちらかの版の監視の一覧の表から 1 つの項目を消し、同じコードスパンを表の外の文に残す。表に余分な項目を加える。
-   どちらかの版の `git-state-changed` の行から `` `ignored` `` を消す。`fixpr.md` の `parts` の文から `` `ignored` `` を消す。
-   どちらかの版の 7.2・8 章・`git-failed` の行の、各ガードの語を 1 つずつ消す。
-   8 章のコードブロックの照合のコマンドで、比べる欄を 10 番目から 9 番目にする。英語版のブロックだけを変える。

**完了条件**: 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）が通る。

#### ステップ 7-5: パッケージの説明

**変更するファイル**: `docs/dev/developer_guide/package_reference.md`

-   [ ] `internal/cmdrunner` の行に `StderrIsFailure` と `ErrUnexpectedStderr` を、`internal/fixprguard` の行に、基準値が監視の一覧に当たる無視されるファイルの状態を含むこと、`CheckState` がそれを比べること、記録のファイルを書くことを加える。書いた内容を `runner.go`・`ignored.go` のコードと突き合わせる。

#### ステップ 7-6: PR-2 の確かめ

**変更するファイル**: なし

-   [ ] `make test && make lint` と、文書のガード（ステップ 7-4 の完了条件のコマンド）を実行する。
-   [ ] AC-01a を手で確かめる（1.3.2）。`extension/node_modules/` がある作業ツリーで、PR-2 のブランチからビルドしたバイナリの `begin` が止まらないことを確かめる。`begin` は GitHub を読むので、利用者の承認を得て実行する（`CLAUDE.md`「Tool Execution Safety」）。かかった時間と対象のファイルの数を本書 1.3.2 に記録する。
-   [ ] 本タスクの前のツール（`Protocol` 3）で、PR-2 のブランチの `fixpr.md` に書いてある形の `--protocol 4` の呼び出しが終了コード `2` で止まることを確かめる。`go version -m "$(command -v fixprguard)"` の `vcs.revision` が PR-2 の前の `origin/main` のコミットでなければ、そのコミットの `cmd/fixprguard` をスクラッチのディレクトリにビルドして使う。

**完了条件**: 上の 3 つが済んでいる。

### PR-2 作成ポイント: compare ignored-file state with the baseline

**対象ステップ**: 5-1 / 5-2 / 5-3 / 6-1 / 6-2 / 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6

**推奨タイトル**: `feat(0034): fixprguard が、git が無視するファイルのうち監視の一覧に当たるものの変化を git-state-changed で止める`

**レビュー観点**: `CheckState` を通るすべての時点（`finish` の build checks の後を含む）で、config と hooks の後に比べ、`parts` が `ignored` だけになること / 標準出力・標準エラー・基準値にパスや内容が現れず、記録のファイルにだけ書くこと、`fixpr.md` が無視されるファイルにかかわるすべての止まり方で調べることを禁じること / `begin` の記録と基準値の 10 番目の欄の結び付けと、ガイド 8 章のコマンドがテストで実際の記録に対して動くこと / `Protocol` 4 とコマンドの文書・ガイド・`check_rules.sh` の番号、`/runpr` の前の再インストール

**実装モデル要件**: frontier-required

**判定理由**: ステップ 5-1・5-2 は、0030 の守りの層 (3) の比較と基準値の形を変える、セキュリティのゲートの変更である（`mkplan.md` step 8 の panel mode の契機）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

**`/runpr` の前**: 利用者は、PR-2 のブランチの作業ツリーで `make install-fixprguard` を実行してから `/runpr` を実行する（ガイド 3 章）。マージまでの間、`main` を取り込んでいないほかの作業ツリーの `/fixpr`・`/runpr` は終了コード `2` で止まる。

**マージの後**: 利用者は、マージの後の `origin/main` を含む作業ツリーで `make install-fixprguard` を実行し直す。作業中のほかのブランチには、次の `/fixpr`・`/runpr` の前に `main` を取り込む。

### フェーズ 8: 0030 の文書（設計書 8 章の 8）

#### ステップ 8-1: 0034 の要件書の確かめ

**変更するファイル**: なし

-   [ ] 0034 の要件書の 3 章・4.4・AC-19・4.2 が、設計書 3.10 の表の 0034 の行のとおりになっていることを確かめる（1.3.1 の「変えない箇所」）。違っていたら、止まって利用者と扱いを決める。

**完了条件**: 確かめた結果をステップ 8-2 のコミットメッセージに書いた。

#### ステップ 8-2: 0030 の設計書 5.3

**変更するファイル**: `docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md`

-   [ ] 5.3「無視されるファイル」（`:847`）を、設計書 3.10 の表の行のとおり、本タスクで検出するようになった範囲と、なお残る範囲（要件書 4.2 の残余のリスクと同じ）にする（AC-20）。
-   [ ] `Status` を `approved` のままにし、`Comments` に、決定は本タスクの要件書の承認で得たので `approved` のままとすることを記録する（設計書 1.2）。

#### ステップ 8-3: 0030 の要件書 4.2

**変更するファイル**: `docs/tasks/0030_fixpr_untrusted_threads/01_requirements.md`

-   [ ] 4.2 の残余のリスク（`:201`）のうち、無視されるファイルの部分だけを AC-20 と同じ内容にする。ネットワークに接続することに関する残余のリスクは残す（AC-20a）。
-   [ ] `Comments` をステップ 8-2 と同じ形で記録する。

#### ステップ 8-4: 0030 の文書のガード

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [ ] `TestTask0030DescribesIgnoredFiles` を加える。0030 の設計書 5.3「無視されるファイル」の項目と、要件書 4.2 の該当の項目に、本タスクへの参照と残る範囲の語があり、両文書の `Status` が `approved` で、`Comments` に本タスクの記録があることを確かめる。0036 の PR-2 が 0030 の文書のガードを置いていれば、同じ部品を使う。

**壊して確かめる**: 注記の語を 1 つずつ消す。参照を消す。`Status` を `draft` にする。`Comments` の本タスクの記録を消す。それぞれで `TestTask0030DescribesIgnoredFiles` が失敗すること。

**完了条件**: 文書のガードが通る。

#### ステップ 8-5: PR-3 の確かめ

**変更するファイル**: なし

-   [ ] `make test && make lint` を実行する。
-   [ ] 本書 5 章の AC の表のすべての行に、`test` か `static` の検証があり、その名前が実際のテストと一致することを確かめる（最後の PR の確かめ）。

**完了条件**: 上の 2 つが済んでいる。

### PR-3 作成ポイント: task 0030 residual-risk documents

**対象ステップ**: 8-1 / 8-2 / 8-3 / 8-4 / 8-5

**推奨タイトル**: `docs(0034): 0030 の要件書と設計書の、無視されるファイルの残余のリスクを改める`

**レビュー観点**: 0030 の 2 か所が、要件書 4.2 の残余のリスクと同じ範囲を示し、ネットワークの残余のリスクを残していること / `Status` を保ち、`Comments` に決定の出所を記録したこと / ガードが文書の語と状態を確かめること

**実装モデル要件**: standard

**判定理由**: 過去のタスクの文書と、その記述を確かめるガードだけの変更で、`mkplan2.md` step 4 の契機のどれにも当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の目安 |
|---|---|---|
| M1 | 監視の一覧、`cmdrunner` の設定、列挙の runner の配線 | ステップ 3-1 が済む |
| M2 | 対象のファイルの状態を求める処理 | PR-1 のマージ |
| M3 | 基準値 `v3` と `CheckState`・`begin` への組み込み | ステップ 5-3 が済む |
| M4 | `Protocol` 4、`fixpr.md`、ガイド（日英）、文書のガード | PR-2 のマージ |
| M5 | 0030 の文書とガード | PR-3 のマージ |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 2-1 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2 | `ignored.go` の監視の一覧・列挙・たどり・上限・エントリ・digest・記録の内容、`cmdrunner` の `StderrIsFailure`、列挙の runner の配線 | frontier-required |
| PR-2 | 5-1 / 5-2 / 5-3 / 6-1 / 6-2 / 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6 | 基準値 `v3`、`CheckState` と `begin` への組み込み、記録のファイル、`Protocol` 4、`fixpr.md`、ガイド（日英）、用語集、文書のガード、`package_reference.md` | frontier-required |
| PR-3 | 8-1 / 8-2 / 8-3 / 8-4 / 8-5 | 0030 の要件書と設計書、そのガード | standard |

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。

-   監視の一覧（フェーズ 1）と `cmdrunner` の設定（フェーズ 2）は、列挙（フェーズ 3）の前提である。列挙の runner の配線は、列挙のメソッドの前提なので、フェーズ 3 の最初に置く。
-   エントリと digest（フェーズ 4）は、列挙とたどりで得た対象のファイルを入力にする。
-   組み込み（フェーズ 5）は、フェーズ 1〜4 の部品を使う。`Protocol` の引き上げ（フェーズ 6）は、組み込みと同じ PR に入れる。別の PR にすると、振る舞いが変わってから番号が上がるまで、入れ直していないツールが古い規則で黙って判定する（設計書 3.9）。
-   ガイドとガード（フェーズ 7）は、`ignored` で止まる振る舞いと同じ PR に入れる（1.4）。ガードは `WatchListPatterns` と `GitStateParts` を読むので、その後に置く。
-   0030 の文書（フェーズ 8）は、ガイド 7.2 の残余のリスクの記述を下敷きにするので、最後に置く。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

設計書 7.1 の各項目を、フェーズ 1〜5 の `ignored_test.go`・`runner_test.go`・`gitstate_test.go` のテストに割り当てた（各ステップのテストの項目）。テストは一時ディレクトリの実際のリポジトリで git を動かし、ネットワークに接続しない。原因は規則ごとの sentinel を `errors.Is` で区別する（1.2）。設計書 7.3 の壊し方は、各ステップの「壊して確かめる」に割り当てた。

### 4.2. 結合テスト

設計書 7.2 の `finish`・`push`・AC-02 は、ステップ 5-3 で `worktree_flow_test.go` と `cmd/fixprguard/main_test.go` に置く。make と gh は `cmdrunnertestutil.Fake` で置き換え、`make test` の中の書き込みは `Step.Effect` で表す（`test_helpers_git.go` の `makeStep`）。ガイド 8 章のコマンドも、ステップ 7-4 のテストで実際の記録のファイルに対して実行する。

### 4.3. セキュリティテスト

設計書 7.3 の、パス・名前・内容・ファイルごとの digest が出力と基準値に現れないことは、ステップ 5-3 の `TestIgnoredOutputOmitsNames` で、`ignored` と `git-failed` の両方の止まり方で確かめる。設計書 5.2 の表の各経路は、次のテストで確かめる。

| 経路 | テスト |
|---|---|
| `.claude/settings.local.json`・`.env`・`.envrc`・無視される `.go`・`Makefile`・`*.mk` | `TestCheckStateIgnoredChanges` |
| 新しい `.gitignore`・グローバルの除外のファイルで隠す | 同上（AC-13 の 3 つの行） |
| ディレクトリの読み取りの許可を外す | `TestCollectTargetsGitFailures`、本番の runner は `TestListRunnerRejectsStderr` |
| Unicode の大文字小文字の別名 | `TestIsWatched` |
| リンクの先の書き換え、ディレクトリを指すリンク | `TestTargetEntries`・`TestCheckStateIgnoredChanges` |
| 巨大な疎ファイル、同じ内容を指す多数のリンク | `TestTargetEntries` |
| FIFO への置き換え、応答しない読み取り | `TestTargetEntries`（置き換えの行）・`TestCollectTargetsLimits`（時間の上限の行） |
| 出力にパスを載せる | `TestIgnoredOutputOmitsNames`、`fixpr.md` の禁止は `TestFixprDocContract` |
| 削除を見落とさせる | `TestBeginWritesRecord`・`TestIgnoredRecordFiles`・`TestGuideIgnoredCommands` |
| 記録の名前のリンク | `TestIgnoredRecordFiles` |

### 4.4. 文書のガード

ステップ 6-2・7-3・7-4・8-4 のガードは `cmd/fixprguard/docs_test.go` と `cmd/yt2column/fixpr_docs_test.go` に置き、CI の `doc-guard` のジョブが実行する。ガードが確かめるのは記述の有無と構造だけなので、中身が正しいことは PR のレビューで読み合わせる。ガイド 8 章のコマンドは、`TestGuideIgnoredCommands` が実行する。

### 4.5. 後方互換性

前の版の基準値（`v2` と 7 つの欄）は受け付けない（AC-03。ステップ 5-1 の `TestPreviousBaselineRejected`）。本タスクの前のツールと新しい文書の組み合わせは、`--protocol` が一致せず終了コード `2` で止まる（既存の `TestProtocolMismatch`（`cmd/fixprguard/main_test.go:43`）と、ステップ 7-6 の手での確認）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は振る舞いを確かめる Go のテスト、`static` は文書を読む Go のガードのテスト、`manual` は PR のレビューか手での確認を表す。新しいテストの名前は仮の名前で、実装で変えたときは本書の表を同じ PR で改める（1.3.4）。場所にファイル名を書いていないテストは、`internal/fixprguard/ignored_test.go` のテストである。

| AC | 検証 | 場所 |
|---|---|---|
| AC-01 | test | `ignored_test.go::TestBeginWithTargets`（ステップ 5-3）、`cmd/fixprguard/main_test.go::TestBeginWritesRecord`（終了コード `0`） |
| AC-01a | test + manual | `ignored_test.go::TestDefaultLimitsAcceptDependencyTree`（ステップ 4-2）。`extension/node_modules/` がある作業ツリーでの `begin`（ステップ 7-6） |
| AC-02 | test | `main_test.go::TestRoundWithUnchangedTargets`（ステップ 5-3）、既存の `cmd/fixprguard` と `internal/fixprguard` のテスト（ステップ 5-1 で期待を変えるのは 1.3.1 の表の行だけ） |
| AC-03 | test | `main_test.go::TestPreviousBaselineRejected`、`internal/fixprguard/gitstate_test.go::TestBaselineMalformedRejected`（ステップ 5-1） |
| AC-04 | test | `ignored_test.go::TestCheckStateIgnoredChanges`（`.claude/settings.local.json` の作成の行） |
| AC-05 | test | 同上（書き換えの行） |
| AC-06 | test | 同上（削除の行） |
| AC-07 | test | 同上（`.envrc`・`.env`、`build/` の `.go`・`Makefile`・`*.mk`、入れ子のリポジトリの `.go` の行）、`ignored_test.go::TestIsWatched`、`::TestCollectTargetsWalk` |
| AC-08 | test | `ignored_test.go::TestCheckStateIgnoredChanges`（種類・実行の許可・リンクの文字列の行） |
| AC-08a | test | 同上（更新時刻だけの行） |
| AC-08b | test | 同上（たどる先の書き換え、たどれないリンクがたどれるようになる行）。たどる先がディレクトリの場合は、設計書 1.2 の決定により `git-failed` で止まる（`ignored_test.go::TestTargetEntries`） |
| AC-09 | test | `internal/fixprguard/worktree_flow_test.go::TestFinishStopsOnIgnoredChange`（`finish` の前に書く行） |
| AC-10 | test | 同上（`make test` の中で書く行。`make lint` を呼ばない） |
| AC-11 | test | `worktree_flow_test.go::TestPushStopsOnIgnoredChange` |
| AC-12 | test | `ignored_test.go::TestCheckStateIgnoredChanges`（監視の一覧に当たらない無視されるファイルの行）、`::TestIsWatched`（当たらない例の行） |
| AC-13 | test | `ignored_test.go::TestCheckStateIgnoredChanges`（最上位と下の階層の新しい `.gitignore`、`core.excludesFile` のファイルへの追記の 3 つの行） |
| AC-14 | test | `ignored_test.go::TestCheckStateIgnoredChanges`（`parts` が `ignored` だけ。同時に変わる行では、先に比べる config か hooks だけ）、`worktree_flow_test.go` の 2 つのテスト、`main_test.go::TestIgnoredOutputOmitsNames`（出力の `parts`） |
| AC-15 | test + static | `main_test.go::TestIgnoredOutputOmitsNames`（`ignored` と `git-failed`。名前・内容とファイルごとのエントリの digest）、`ignored_test.go::TestIgnoredRecordFiles`、`internal/cmdrunner/runner_test.go::TestStderrIsFailure`（エラーに標準エラーを含めない）。`cmd/yt2column/fixpr_docs_test.go::TestFixprDocContract`（`fixpr.md` の禁止。ステップ 6-2） |
| AC-16 | test | `ignored_test.go::TestParseIgnoredListing`・`::TestCollectTargetsGitFailures`（ステップ 3-2）、`::TestCollectTargetsWalk`（ステップ 3-3）、`::TestCollectTargetsLimits`（ステップ 3-4）、`::TestTargetEntries`（ステップ 4-1）、`::TestBaselineIgnoredFailures`（ステップ 5-3）、`runner_test.go::TestStderrIsFailure`（ステップ 2-1）、`main_test.go::TestListRunnerRejectsStderr`（ステップ 3-1） |
| AC-17 | static + manual | `cmd/fixprguard/docs_test.go::TestGuideListsWatchList`（ステップ 7-4）、`ignored_test.go::TestWatchListPatternsMatch`（ステップ 1-1）。説明の文は PR-2 のレビューで読み合わせる |
| AC-18 | static + manual | `docs_test.go::TestGuideDescribesIgnoredFiles`（7.2 の分けた項目の語）。各項目の範囲は PR-2 のレビューで要件書 4.2 と読み合わせる |
| AC-19 | test + static + manual | `docs_test.go::TestGuideIgnoredCommands`（コマンドの実行）、`::TestGuideNamesGitStateParts`、`::TestGuideDescribesIgnoredFiles`（(a)〜(e) の語）、`main_test.go::TestBeginWritesRecord`（記録と 10 番目の欄）。手順の中身は PR-2 のレビューで読み合わせる |
| AC-19a | static + manual | `docs_test.go::TestGuideDescribesIgnoredFiles`（`git-failed` の行の原因の語）。対処の中身は PR-2 のレビューで読み合わせる |
| AC-20 | static + manual | `docs_test.go::TestTask0030DescribesIgnoredFiles`（ステップ 8-4）。範囲は PR-3 のレビューで要件書 4.2 と読み合わせる |
| AC-20a | static + manual | 同上 |

-   `Protocol` が `4` に上がることは、PR-2 の差分で確かめる（`manual`）。定数をそれ自身のリテラルと比べるテストは書かない（`CLAUDE.md`「Testing Strategy」）。

### 5.1. 横断の検索

`make test`・`make lint` とガードが検出しないものだけを挙げる。どれも `manual` で、実行した検索とその結果を、そのステップのコミットメッセージに書く。

-   [ ] ステップ 6-1 の `git grep` で、`.claude/`・`docs/dev/`・`check_rules.sh` に `--protocol 3`・`"protocol":3` が残っていない。`docs/tasks/` の下の `.md` の文書の番号は、当時の記録として変えない。
-   [ ] ガイドの日英で、基準値の例が `'v3....'` になっている。
-   [ ] ガイド 7.2 の、今の「`.gitignore` が無視するファイル、…」の項目（`:281`）の古い文が、分けた項目の後に残っていない。

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 2-1 / 3-1 / 3-2 / 3-3 / 3-4 / 4-1 / 4-2）
- [ ] PR-2 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 6-1 / 6-2 / 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6）
- [ ] PR-3 マージ済み（対象ステップ: 8-1 / 8-2 / 8-3 / 8-4 / 8-5）

## 7. リスク管理 (Risk Management)

### 7.1. 技術的リスク

| リスク | 対策 |
|---|---|
| 照合のテストの行を `ignored.go` のデータから作り、データから項目を消すと行も消える | 行を要件書 3 章の表と例から作る（1.2）。ステップ 1-1 の壊す確認で確かめる |
| macOS と Linux の CI で、テストの結果が違う | テスト用のリポジトリで 3 つの git の設定を固定する。不正な UTF-8 の名前をディスクに作らない（1.2） |
| CI の git の版で、列挙の出力の形や警告の出し方が違う | ステップ 3-2 のテストが CI で git を実際に動かして確かめる。違えば止まって設計を見直す |
| 権限のテストが root では権限の失敗を作れない、モードの残ったディレクトリが `t.TempDir` の削除を妨げる | `requireNonRoot` と `chmodForTest` を使う（1.2） |
| 時間の上限と FIFO のテストが遅い、止まる、または `-race` で競合する | 差し込みを呼び出しごとの値にし、差し込んだ関数を `t.Cleanup` と自身の期限で戻す（1.2、ステップ 3-4・4-1） |
| エラーの連鎖に OS のエラーや git の標準エラーが入り、パスが標準エラーに出る | 規則の名前だけからエラーを作り（1.2）、`TestIgnoredOutputOmitsNames` で確かめる |
| `CheckState` の呼び出しが増え、テストの呼び出しの列の期待が崩れる | 1.3.1 の表のテストだけを直し、ほかの期待は変えない（ステップ 5-1） |
| タスク 0036 の PR-2 とタスク 0031 の PR #448 が、ガイド 7.2・8 章・9 章、`docs_test.go`、0030 の文書を同時に変える | 後に入る PR が `main` を取り込んで競合を解く。ステップ 7-4・8-4 で、先に入った PR のガードの部品があれば使う |
| `Protocol` の番号の固定箇所の漏れ | ステップ 6-1 の `git grep` と `check_rules.sh` の実行、`TestDocumentedProtocolMatches` |
| PR-2 の `/runpr` が、インストール済みのツールの番号の不一致で止まる | PR-2 の作成ポイントの「`/runpr` の前」のとおり再インストールする。忘れても終了コード `2` で止まるだけで、古い規則では判定しない |
| 正当な書き込み（許可の確認への回答、Claude Code の実行時のファイル）で止まり、利用者が確かめずにやり直すことに慣れる | 要件書 4.2・設計書 5.3 の残余のリスクとしてガイド 7.2 に書き、8 章の (e) で案内する（ステップ 7-1） |

### 7.2. スケジュールのリスク

-   ステップ 7-6 の AC-01a の確認は、ネットワークに接続する操作（`make ext-install` か `begin` の GitHub の読み取り）に利用者の承認が要る。承認を待つ間も、ほかの確かめは進められる。
-   PR-2 は大きい（1.4）。Go の組み込み（フェーズ 5・6）と文書（フェーズ 7）をコミットで分け、PR の説明でレビューの順を示す。

## 8. 成功基準 (Success Criteria)

-   **機能:** 要件書の AC-01〜AC-20a のすべてに、5 章の `test` か `static` の検証があり、通る。
-   **品質:** `make test`・`make lint`・文書のガードが通る。各テストが目的の理由で失敗できることを、壊して確かめ、コミットメッセージに書いた（`CLAUDE.md`「Testing Strategy」）。
-   **セキュリティ:** 4.3 の表の各経路のテストが通る。出力と基準値にパスや内容が現れない（AC-15）。
-   **文書:** ガイド（日英）、`fixpr.md`、`package_reference.md`、用語集、0030 の 2 つの文書が、新しい保証と残余のリスクを示している。

## 9. 次のステップ (Next Steps)

-   PR-3 のマージの後、issue #424 を閉じる（`/runpr` が最後の計画の PR の後に提案する）。
-   #446（既存の clean の検査と変更の集合に、標準エラーの検査を広げる）で、`cmdrunner` の `StderrIsFailure` を使う（設計書 9 章）。
