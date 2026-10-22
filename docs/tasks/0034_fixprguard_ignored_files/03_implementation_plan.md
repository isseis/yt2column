# 実装計画書：git が無視するファイルのうち、後の実行に関わるものへの書き込みを fixprguard が検出する

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

[01_requirements.md](01_requirements.md)（以下、要件書）の F-001〜F-004 を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。作業は設計書 8 章の 8 つの項目を 1 つずつフェーズにし、3 つの PR にまとめる（1.4）。

-   **フェーズ 1〜4（設計書 8 章の 1〜4）:** `internal/fixprguard/ignored.go` に、監視の一覧、列挙と受理する形、ディレクトリの項目のたどり、上限、各ファイルのエントリと digest と記録の内容を置く。`cmdrunner` に標準エラーを失敗とする設定を加える。まだ `CheckState` には組み込まないので、ツールの振る舞いは変わらない。
-   **フェーズ 5〜7（設計書 8 章の 5〜7）:** 基準値を `v3` にし、`CheckState` と `begin` に組み込む。`Protocol` を `4` にし、`fixpr.md`・`runpr.md`・ガイド・`check_rules.sh` の番号を揃える。ガイド（日英）を改め、文書のガードを加える。
-   **フェーズ 8（設計書 8 章の 8）:** 0030 の要件書と設計書の残余のリスクを改め、そのガードを加える。

用語は、要件書 6 章と設計書 1.4 の定義に従う（監視の一覧、対象のファイル、状態、項目、ディレクトリの項目、走査したエントリ、エントリ、記録のファイル）。本書の「フェーズ」は実装の区切りを指し、`/fixpr` の「修正のフェーズ」とは別である。「ガイド」は `docs/dev/developer_guide/fixprguard_guide.ja.md`（日本語版）と `fixprguard_guide.md`（英語版）を指す。

### 1.2. 実装原則

-   各ステップは設計書の節を参照し、設計の内容を本書で言い換えない。
-   Go のコメント・識別子・文字列リテラルは英語で書く。コマンドの文書（`.claude/commands/`）の文も英語で書く（`CLAUDE.md`「Language」）。例外は、日本語の文書の文字列と一致させる文書のガードのリテラルだけである（先例: `cmd/fixprguard/docs_test.go:535-550`）。
-   テストは、対象の実装を壊して失敗することを確かめてからコミットし、壊し方と結果をコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」、設計書 7.3）。各ステップの「壊して確かめる」は、壊す対象と、失敗するはずのテストを挙げる。
-   テストは一時ディレクトリの実際のリポジトリで git を動かす（設計書 7 章）。対象のファイルの判定がテストの環境に左右されないよう、テスト用のリポジトリで `core.excludesFile`・`core.ignorecase`・`core.precomposeunicode` を固定する（ステップ 3-1・5-5）。
-   監視の一覧の照合のテストの行は、`ignored.go` のデータからではなく、要件書 3 章の表と例から 1 つずつ作る。データから項目を消すと行も消える、ということを避けるためである。
-   ガイドは、日本語版をコミットしてから英語版を `.claude/commands/mktrans.md` の原則で翻訳し、翻訳の後に日本語版と節ごとに突き合わせる（`CLAUDE.md`「Translation Guidelines」）。
-   Go のファイルを変えたら `make fmt` を実行する。各フェーズの最後に `make test && make lint` を実行する。
-   ソースのコメントに `AC-NN`・`F-NNN` を書かない（要件書との対応は本書 5 章に書く）。

### 1.3. 既存コード調査結果

調査はコミット `c077836`（`origin/main`）で行った。設計書が確かめたコミット `9f83469` から `c077836` までに変わったのは、タスク 0036 の PR-1（#449）のファイルだけである（`git diff --stat 9f83469 c077836 -- internal/ cmd/ .claude/ docs/dev/ docs/tasks/0030_fixpr_untrusted_threads docs/tasks/0032_japanese_pr_title CLAUDE.md Makefile .github`）。`internal/fixprguard/gitstate.go`・`worktree.go`・`cmd/fixprguard/main.go`・`internal/cmdrunner/` は変わっていないので、設計書のこれらの `file:line` はそのまま使える。以下の `file:line` は `c077836` のものである。

#### 1.3.1. 変える箇所の一覧

| 箇所 | 今 | 変え方 | ステップ |
|---|---|---|---|
| `internal/fixprguard/scope.go:57-60` | 0036 が `goBuildInputSuffixes` に Go のビルドの入力の 17 種類を置いた | 変えない。監視の一覧の照合が参照する（設計書 3.2） | 1-1 |
| `internal/cmdrunner/runner.go:30-39`・`:78-110` | `Options` は `MaxOutput`・`WaitDelay`・`Dir`・`Env`。成功したら標準エラーを捨てる | `StderrIsFailure` と `ErrUnexpectedStderr` を加える（設計書 3.3・4 章） | 2-1 |
| `internal/fixprguard/gitstate.go:28-32`・`:112-153` | `baselineVersion = "v2"`、`baselineFields = 7`、`Baseline` の 7 つの欄、`newBaseline`・`ParseBaseline`・`String` | `v3`・9 つの欄にする（設計書 3.6） | 5-1 |
| `gitstate.go:50-56` | `Part*` の 5 つの定数 | `PartIgnored` と `GitStateParts` を加える（設計書 3.1） | 5-1 |
| `gitstate.go:568-607`（`CheckState`） | hooks の比較で終わる | 最後に対象のファイルの状態を比べ、違えば止まった時点の記録のファイルを書く（設計書 3.7・3.8） | 5-3 |
| `gitstate.go:742-787`（`Baseline`） | 7 つの欄を求める | 対象のファイルの状態を求め、2 つの欄を加える（設計書 3.6） | 5-3 |
| `internal/fixprguard/worktree.go:51-61`（`Worktree`・`NewWorktree`） | git と make の 2 つの runner | 列挙のための runner を加える（設計書 3.3） | 5-2 |
| `worktree.go:80-85`（`Begin`） | `CheckClean` の後に `Baseline` | `Baseline` の後に `WriteBeginRecord` を呼ぶ（設計書 3.1・3.8） | 5-4 |
| `cmd/fixprguard/main.go:149-165`（`osEnv`）・`:350` | `NewWorktree(e.git, e.make)` | 列挙の runner を `StderrIsFailure` の設定で作って渡す | 5-2 |
| `main.go:470-520`（`begin`） | `w.Baseline` の後に `g.Begin` | `w.Baseline` の後に `w.WriteBeginRecord` を呼ぶ（設計書 3.8「書き方」の最後） | 5-4 |
| `NewWorktree` の呼び出し: `internal/fixprguard/test_helpers_git.go:221`、`cmd/fixprguard/test_helpers.go:114`・`:125` | 2 つの引数 | 列挙の runner を加える。`worktree_flow_test.go` には呼び出しはない（設計書 2.2 の表を editorial correction で改める。1.4） | 5-2 |
| `internal/fixprguard/test_helpers_git.go:104-127`（`newTestRepo`）、`cmd/fixprguard/test_helpers.go:42-60` | `HOME`・`XDG_CONFIG_HOME` は固定済み。`core.excludesFile`・`core.ignorecase`・`core.precomposeunicode` は固定していない | 3 つの設定を固定する（設計書 7 章）。cmd 側も同じ形のリポジトリを作るので、両方に当てる | 3-1・5-5 |
| `internal/fixprguard/gitstate_test.go:324-355`（`TestBaselineMalformedRejected`） | `with(5, ...)`・`with(6, ...)` と、前の版を `v1` にする行（`:340`） | 前の版を `v2` と 7 つの欄の形にし、新しい欄の誤りの行を加える（設計書 2.2） | 5-5 |
| `cmd/fixprguard/test_helpers.go:381-390`（`validBaseline`） | `v2` と 7 つの欄 | `v3` と 9 つの欄 | 5-5 |
| `cmd/fixprguard/main_test.go:26`（`validArgs` の `"v2.x"`） | 引数の解析だけを試す値 | `"v3.x"` にする | 5-5 |
| `cmd/fixprguard/main_test.go:309-345`（`TestPreviousBaselineRejected`） | `v2.` を `v1.` に替えて前の版を作る | 前の版を `v2` と 7 つの欄の形にする（AC-03） | 5-5 |
| `cmd/fixprguard/main_test.go:270`（`TestBeginOrder`） | `begin` の呼び出しの順序 | `WriteBeginRecord` が加える `git ls-files` を順序に加える | 5-5 |
| `internal/fixprguard/gitstate_test.go:275`・`:375`・`:394`・`:551`・`:587`、`worktree_flow_test.go:311`・`:443` | git の呼び出しの列（`router.Calls`）を比べる | config・hooks が違って止まる場合は列挙の前に止まるので変わらない。一致する場合の列が変わるテストだけを直す | 5-5 |
| `internal/fixprguard/types.go:21` | `Protocol = 3` | `4`（設計書 3.9） | 6-1 |
| `.claude/commands/fixpr.md`（`--protocol 3` が 8 か所。`:44` は文）、`runpr.md`（4 か所。`:18` は文） | `--protocol 3` | `--protocol 4` | 6-1 |
| ガイド 4 章（各版 `:64`）・5 章（`--protocol 3` と `"protocol":3`、`'v2....'` が `:153`・`:165`・`:177`） | `3` と `'v2....'` | `4` と `'v3....'`（設計書 3.9） | 6-1 |
| `docs/tasks/0032_japanese_pr_title/check_rules.sh:524` | `--protocol 3` | `--protocol 4` | 6-1 |
| `.claude/commands/fixpr.md:492-493`・`:524-525`・`:663-664` | `parts` を `HEAD`・`the remote branch` などと言い換えて列挙する | `ignored` を加え、止まったときの扱いを書く（設計書 3.10） | 6-2 |
| ガイド 1 章（各版 `:13`）・5 章（`begin` の副作用 `:136`、`inspect`・`finish`・`push` の節）・7.1（`:258-268`）・7.2（`:281`）・8 章（`:295`・`:304`）・9 章（`:313`・`:326`） | 対象のファイルの記述がない | 設計書 3.10 の表のとおり改める | 7-1・7-2 |
| `cmd/fixprguard/docs_test.go:91`（`guideTable`） | 太字の見出しの後の最初の表を切り出す（0036） | 監視の一覧の表のガードで再利用する | 7-4 |
| `cmd/fixprguard/docs_test.go:526-576`（`TestGuideDescribesConfigScope`） | 1 章の層 (3)（`"3. **"` で始まる行がちょうど 1 つ）、5 章の `inspect`・`push`、7.1 の行、7.2、8 章の `git-state-changed` の行の語を確かめる | 変えない。改める段落でもこの語と行の形を保つ（設計書 2.2） | 7-1・7-2 |
| `docs/dev/developer_guide/package_reference.md:19`（`internal/cmdrunner`）・`:21`（`internal/fixprguard`） | `Options` の 4 つの欄を挙げる | `StderrIsFailure`・`ErrUnexpectedStderr` と、対象のファイルの比較を加える（設計書 2.2 の表に editorial correction で加える。1.4） | 7-5 |
| `docs/translation_glossary.md:78` の近く | `基準値` はあるが、本タスクの用語はない | ガイドの翻訳で使った用語（監視の一覧、対象のファイル、記録のファイル）を加える（`mktrans.md`「Update Glossary」） | 7-3 |
| 0030 `01_requirements.md:201`・`02_architecture.md:847` | 無視されるファイルを残余のリスクとする | 設計書 3.10 のとおり改め、`Comments` に記録する | 8-2・8-3 |

変えない箇所は次のとおりである。

-   `internal/cmdrunner/testutil/mocks.go`。`Fake` は `Options` を持たず、`Step.Err` に `ErrUnexpectedStderr` を入れれば失敗を再現できる。本タスクのテストは列挙に実際の git を使う（設計書 7 章）。設計書 2.2 の表はこのファイルを「変更」とするので、editorial correction で改める（1.4）。
-   範囲の規則（`scope.go` の判定）と、`changes.go` の `splitNUL`・`readChanged`、`gitstate.go` の `hooksDigest`・`fileSHA256`（設計書 1.3 の「使わない」）。
-   `runpr.md` の `git-state-changed` の扱い（`:273-278` 付近）。番号だけを変える（設計書 2.2）。
-   0034 の要件書。設計書 3.10 の表の 0034 の行の変更（照合の記述、AC-19 と 4.2 の記録の記述）は、設計書の PR（#447）で済んでいる。要件書 3 章 `:116`・4.4 `:269` は Unicode の単純な大文字小文字の変換を、AC-19 `:214` は「書き換えれば分かる」記録を、4.2 `:233` は記録のファイルの digest を述べている。ステップ 8-1 で確かめ直す。

#### 1.3.2. 外部の前提の確かめ

-   **タスク 0036 との順序:** 0036 の PR-1（#449）は `c077836` で `main` に入り、`Protocol` は `3`、Go のビルドの入力の拡張子は `scope.go` の `goBuildInputSuffixes` にある。そのため、設計書 3.9 の「0036 が先」の場合に当たり、番号は `3` から `4` にする。設計書 3.9 の「本タスクが先に入る場合に 0036 に伝える手順」は当てはまらない。0036 の PR-2（ガイド 7.2・8 章、用語集、0030 の文書の注記）はまだ入っていない（7.1 のリスク）。
-   **タスク 0031 の開いている PR #448:** ガイド 4・5・7・9 章、`cmd/fixprguard/docs_test.go`・`main.go` を変える。`Protocol` は上げない（0031 `03_implementation_plan.md`「フェーズ 1 の完了の条件」）。番号の衝突はないが、同じ箇所を変えるので、後に入る PR が `main` を取り込んで競合を解く（7.1 のリスク）。
-   **git の振る舞い:** 列挙の出力の形、入れ子のリポジトリの項目、読めないディレクトリの警告と終了コード `0` は、設計書 2.4 が git 2.50.1 で確かめた。本書の時点の手元の git も 2.50.1 である。CI の git の版での振る舞いは、ステップ 3-1 のテストが CI で確かめる。
-   **標準ライブラリ:** `os.CreateTemp` はモード `0o600` で、同時に呼んでも同じ名前を選ばない（`go doc os.CreateTemp`、Go 1.27.1）。`unicode.SimpleFold` は単純な大文字小文字の変換で同値な文字を巡回する（`go doc unicode.SimpleFold`）。`rename(2)` は置き換える先のシンボリックリンクをたどらない（設計書 3.8）。
-   **SHA-256 のコマンド:** macOS に `/usr/bin/shasum` と `/sbin/sha256sum` がある（2026-10-22）。ガイドには設計書 3.8 のとおり `shasum -a 256` と `sha256sum` の両方を書き、ステップ 7-1 で実際に実行して確かめる。
-   **AC-01a の作業ツリー:** 本 worktree には `extension/node_modules/` がない。ステップ 7-6 で、利用者の承認を得て `make ext-install`（ネットワークに接続する）を実行するか、`node_modules` がある main のチェックアウトで、PR-2 のブランチからビルドしたバイナリの `begin` を実行する（`CLAUDE.md`「Tool Execution Safety」）。

#### 1.3.3. 型の構築の経路

`Baseline` は、今までどおり非公開の欄と唯一の構築子 `newBaseline` を持ち、`construction_test.go:20` のガードが構築の経路を確かめる（設計書 3.1）。欄を加えても構築子は 1 つのままなので、ガードは変えない。新しい型 `targetState`・`targetLimits` は非公開で、型で振る舞いを選ばない。上限の値は `defaultTargetLimits` だけから本番のコードに渡る（設計書 3.5）。そのため、構築の経路のガードは加えない。

#### 1.3.4. 実装時に決めること

-   `ignored.go` の非公開の関数の名前と分け方（列挙の検査、たどり、エントリの組み立て、記録の書き込み）。公開するものは設計書 3.1 のとおりである。
-   テストの関数の名前。5 章の表に書いた名前と変えたときは、同じ PR で本書の表を改める。
-   列挙の runner をテストの `gitRouter` に通すかどうか。通すと `router.Calls` に `git ls-files` が現れるので、1.3.1 の呼び出しの列を比べるテストを直す。
-   文書のガードが固定する語と、箇所を取り出す鍵。語は書いた文から選ぶ。

### 1.4. 計画の際に決めたこと（2026-10-22）

-   **PR は 3 つにする（PR-1 = フェーズ 1〜4、PR-2 = フェーズ 5〜7、PR-3 = フェーズ 8）:** レビュアーが選んだ。理由: PR-1 は振る舞いを変えない部品だけなので、照合・形の検査・上限・読み取りを、それだけでレビューできる。PR-2 は、ツールが `ignored` で止まるようになる変更と、その対処の手順（ガイド 8 章）を同じ PR に入れる。そのため、`main` に止まり方だけが入り、手順がない期間ができない。`Protocol` の引き上げは、振る舞いを変える PR-2 に入れる（設計書 3.9）。PR-3 は過去のタスクの文書だけの変更である。受け入れる影響: PR-2 は、Go の組み込みとガイドの日英の改訂を含むので大きい。
-   **設計書の 3 か所を editorial correction として改める（この計画の PR）:** 設計書 2.2 の表で、(1) `internal/cmdrunner/testutil/mocks.go` を変えないことにする（1.3.1 の「変えない箇所」）、(2) `NewWorktree` の呼び出しの行を、実際の呼び出しのあるファイル（`test_helpers_git.go`・`cmd/fixprguard/test_helpers.go`）にし、cmd 側のテスト用のリポジトリにも 3 つの設定を固定する、(3) `package_reference.md` と `docs/translation_glossary.md` の行を加える。どれも作るものと理由を変えないので、`Status` は `approved` のままにし、設計書の `Comments` に記録する。
-   **AC-01a は、`test` と `manual` で確かめる:** 既定の上限で、依存のパッケージに似た無視されるディレクトリ（多数の監視の一覧に当たらないファイルと、C のヘッダ・`Makefile` を含む）の `begin` が止まらないことを単体テストで確かめる。実際の `extension/node_modules/` での確認は、ステップ 7-6 で手で行う（設計書 7.4）。CI には `node_modules` がない場合がある。
-   **AC-18〜AC-20a は、文書のガードと PR のレビューで確かめる:** 設計書 7.4 は、監視の一覧の表、`git-state-changed` の行、`fixpr.md` の `parts` の 3 つのガードを挙げる。AC-18（7.2 の分けた項目）、AC-19 の (a)〜(e) の手順、AC-19a（`git-failed` の行）、AC-20・AC-20a（0030 の文書）にも、語の有無を確かめるガードを加える。0036 の `passage`・`checkPassages`（`docs_test.go:467-524`）の形を使う。ガードは語の有無だけを確かめるので、中身は PR のレビューで読み合わせる（`manual`）。
-   **ガイド 8 章のコマンドは、実際の記録のファイルで実行して確かめる:** ステップ 7-1 で、テスト用のリポジトリで `begin` と `inspect` を実行して 2 つの記録のファイルを作り、ガイドの (a)・(b) のコマンドをそのまま実行して、照合の一致と `diff` の出力を確かめる。結果はコミットメッセージに書く。

### 1.5. 申し送りの項目への対応

-   `design_handoff.md` の H-01〜H-08 は、すべて設計書の付録 A に記録されている。
-   `implementation_handoff.md` はない。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: 監視の一覧（設計書 8 章の 1）

#### ステップ 1-1: 監視の一覧のデータと照合

**変更するファイル**: `internal/fixprguard/ignored.go`（新規）、`internal/fixprguard/ignored_test.go`（新規）

-   [ ] 監視の一覧のデータを設計書 3.2 の表のとおり置く。Go のビルドの入力の拡張子は `scope.go` の `goBuildInputSuffixes` を参照する。
-   [ ] `isWatched` を、設計書 3.2「同一視の仕方」のとおり、正しい UTF-8 の文字は `unicode.SimpleFold` で、不正なバイトは完全な一致で比べる。`strings.EqualFold` と `strings.ToLower` は使わない。
-   [ ] `WatchListPatterns` を設計書 3.2「ガイドの表記」の形で返す。判定と同じデータから組み立てる。
-   [ ] `ignored_test.go` に照合のテストを加える。行は設計書 7.1「監視の一覧の照合」の項目で、要件書 3 章の当たる例と当たらない例を 1 つずつ行にする（1.2）。

**壊して確かめる**: 次で、それぞれ照合のテストが失敗すること。

-   データから 1 つの項目を消す（名前・拡張子・Go のビルドの入力の拡張子・最上位のファイルのそれぞれで 1 つずつ）。
-   同一視を ASCII だけにする（KELVIN SIGN と LONG S の行）。
-   不正なバイトを U+FFFD とみなす（不正なバイトだけが違う 2 つのパスの行）。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

### フェーズ 2: `cmdrunner` の標準エラーの設定（設計書 8 章の 2）

#### ステップ 2-1: `StderrIsFailure` と `ErrUnexpectedStderr`

**変更するファイル**: `internal/cmdrunner/runner.go`、`internal/cmdrunner/runner_test.go`

-   [ ] `Options` に `StderrIsFailure` を加え、成功したコマンドが標準エラーに書いたら `ErrUnexpectedStderr` を返す（設計書 3.3・4 章）。ゼロ値の振る舞い（今の振る舞い）は変えない。標準出力は返さない。
-   [ ] `runner_test.go` に、設定ありで標準エラーに書いて成功するコマンドが `ErrUnexpectedStderr` になること、標準エラーが空なら標準出力を返すこと、設定なしなら今までどおり成功することを確かめるテストを加える。上限を超えた場合は今までどおり `ErrOutputTooLarge` になることも確かめる。

**壊して確かめる**: 標準エラーの検査を外すと、加えたテストが失敗すること。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

### フェーズ 3: 列挙・受理する形・たどり・上限（設計書 8 章の 3）

#### ステップ 3-1: 列挙と受理する形

**変更するファイル**: `internal/fixprguard/ignored.go`、`ignored_test.go`、`internal/fixprguard/test_helpers_git.go`

-   [ ] `newTestRepo` で `core.excludesFile`・`core.ignorecase`・`core.precomposeunicode` を固定する（設計書 7 章）。既存のテストが通ることを確かめる。
-   [ ] 列挙を設計書 3.3「列挙の手段」「標準エラー」のとおり、`gitHardening` を付けて、`StderrIsFailure` の runner で起動する関数を加える。この時点では、関数が runner を引数で受け取る（`Worktree` の欄はステップ 5-2 で加える）。
-   [ ] 受理する形の検査を、設計書 3.3「受理する形」のとおり、出力のバイト列に対して行う。`splitNUL` は使わない（設計書 1.3）。合わなければ、設計書 4 章の規則の名前で `ErrGitFailed` を包むエラーを返す。
-   [ ] テストを加える。形の検査は設計書 7.1「列挙の形の検査」の各例と、正しい出力・ディレクトリの項目の受け付けを、git を起動せずにバイト列で確かめる。git の警告は、テスト用のリポジトリの `build/` の下と `.claude/` の下にモード `0311` のディレクトリを置き、`git-failed` で止まることを確かめる（設計書 7.1「git の警告」）。テストの後にモードを戻す `t.Cleanup` を置く（`t.TempDir` の削除が失敗しないように）。

**壊して確かめる**: 形の検査の各条件を 1 つずつ外すと、対応する行が失敗すること。列挙の runner を `StderrIsFailure` なしにすると、git の警告のテストが失敗すること。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

#### ステップ 3-2: ディレクトリの項目のたどりと `.claude/settings.local.json`

**変更するファイル**: `internal/fixprguard/ignored.go`、`ignored_test.go`

-   [ ] ディレクトリの項目の下を、設計書 3.3「ディレクトリの項目」のとおりたどる。シンボリックリンクはたどらず、通常のファイルとシンボリックリンクだけを対象のファイルにする。
-   [ ] 監視の一覧で選び、`.claude/settings.local.json` を列挙の結果にかかわらず加える（設計書 3.3）。対象のファイルをパスのバイト列の順に並べる。
-   [ ] テストを加える。入れ子のリポジトリの中の `.go` が対象になること、入れ子のリポジトリの中の FIFO と監視の一覧に当たらないファイルが対象にならないこと、`.claude/settings.local.json` が無視されていなくても（存在しなくても）必ず含まれ、列挙にも現れたときに 1 つになること、読めないディレクトリで止まること。

**壊して確かめる**: たどりを外すと入れ子のリポジトリの `.go` の行が、`.claude/settings.local.json` を必ず加える処理を外すと存在しない場合の行が失敗すること。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

#### ステップ 3-3: 上限と時間の上限

**変更するファイル**: `internal/fixprguard/ignored.go`、`ignored_test.go`

-   [ ] `targetLimits` と `defaultTargetLimits` を設計書 3.1・3.5 の表の値で置く。対象のファイルの数と、ディレクトリの項目の下の走査したエントリの数の上限を、列挙とたどりに当てはめる。列挙の出力の上限は、既存の git の出力の上限（`cmd/fixprguard/main.go:45`）を列挙の runner にも当てる（設計書 3.5）。
-   [ ] 状態を求める処理の全体を、時間の上限以内で待つ（設計書 3.5「時間の上限」）。超えたら結果を待たずに `read ignored files timed out` で止まる。
-   [ ] テストを加える。小さな `targetLimits` で各上限を超えさせ、設計書 4 章の規則の名前で止まること。時間の上限は、テストの中で処理を待たせる仕掛け（たとえば FIFO を開く代わりに待つ関数を差し込む）で確かめる。テストのための分岐を本番のコードに置かない。差し込みは `targetLimits` か非公開の関数の値で行う。

**壊して確かめる**: 各上限の検査を 1 つずつ外すと、対応する行が失敗すること。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

### フェーズ 4: エントリ・digest・記録の内容（設計書 8 章の 4）

#### ステップ 4-1: 各ファイルのエントリと読み取り

**変更するファイル**: `internal/fixprguard/ignored.go`、`ignored_test.go`

-   [ ] 設計書 3.4 の 2 つの表のとおり、対象のファイルとリンクのたどる先のエントリを作る。値は `writeField`（`threads.go`）で区切る。内容は設計書 3.4「内容の読み方」のとおり、`openRegular` と同じく待たずに開き、開いたファイルで種類と大きさを確かめ、上限に 1 を足した量までしか読まない。
-   [ ] 1 つのファイルの大きさと、読み取る内容の合計の上限を、読み取りに当てはめる（設計書 3.5）。合計は、読んだバイト数をそのまま足す。
-   [ ] テストを加える（設計書 7.1「上限と読み取り」「エントリの境目」）。読めないファイル、ディレクトリを指すリンク、`EACCES` で調べられないたどる先、`ENOENT`・`ELOOP`・`ENOTDIR` のたどれないリンク（`unresolved` として記録し、止まらない）、FIFO に置き換えたファイル（待たずに止まる）、大きさの上限、同じ疎ファイルを指す多数のリンクで合計の上限を超えること、エントリの境目。

**壊して確かめる**: 合計の上限の検査を外すと多数のリンクの行が、`O_NONBLOCK` を外すと FIFO の行が（時間の上限で止まるので、テストは時間切れの規則の名前でないことを確かめる）、`unresolved` を止まる扱いにするとたどれないリンクの行が失敗すること。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

#### ステップ 4-2: まとめた digest と記録の内容

**変更するファイル**: `internal/fixprguard/ignored.go`、`ignored_test.go`

-   [ ] `collectTargets` を、設計書 6.2 の流れのとおり、`targetState`（まとめた digest と記録の内容）を返す関数にまとめる（設計書 3.1・3.4）。
-   [ ] 記録の内容を設計書 3.8「形」のとおり作る。パスは `strconv.QuoteToASCII` で引用する。
-   [ ] テストを加える。状態の変化（設計書 7.1「状態の比較」の各変化）で digest が変わること、更新時刻だけの変化と監視の一覧に当たらないファイルの変化で変わらないこと。記録の各行がエントリの digest と ASCII に引用したパスであり、改行・制御文字・不正な UTF-8 を含むパスも 1 行になること。既定の上限で、依存のパッケージに似たディレクトリ（監視の一覧に当たらないファイル数千個と、C のヘッダ・`Makefile`）が止まらないこと（AC-01a。1.4）。
-   [ ] 本書 5 章の表の、フェーズ 1〜4 のテストの名前を、実際の名前に合わせる。

**壊して確かめる**: エントリから実行の許可・リンクの文字列・たどる先の値を 1 つずつ外すと、対応する変化の行が失敗すること。更新時刻を書くと更新時刻の行が失敗すること。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。`make deadcode` が、テストからだけ使う関数を報告しないこと（`deadcode -test`）を確かめる。

### PR-1 作成ポイント: ignored-file state machinery

**対象ステップ**: 1-1 / 2-1 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2

**推奨タイトル**: `feat(0034): fixprguard に、git が無視するファイルのうち監視の一覧に当たるものの状態を求める処理を加える`

**レビュー観点**: 監視の一覧の照合が Unicode の単純な大文字小文字の変換で同一視し、不正なバイトを区別すること / 列挙の受理する形の検査と git の警告で、補正も読み飛ばしもせずに止まること / 各上限と時間の上限が、内容を読み続けずに止まり、FIFO で待たないこと / エントリの境目が一意で、更新時刻を含まないこと

**実装モデル要件**: frontier-required

**判定理由**: ステップ 3-1〜4-1 は、信頼できない入力（git の出力、攻撃者が選べるファイルの名前・種類・大きさ）の境界と、0030 の守りの層 (3) に加える検出の部品である（`mkplan.md` step 8 の panel mode の契機のセキュリティのゲート）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 基準値と比較への組み込み（設計書 8 章の 5）

フェーズ 5 と 6 は、同じ PR で `main` に入れる（設計書 3.9）。フェーズ 5 の途中では、文書のガードとバイナリの番号は一致したままである（番号はステップ 6-1 で上げる）。

#### ステップ 5-1: 基準値の形と `parts`

**変更するファイル**: `internal/fixprguard/gitstate.go`

-   [ ] `baselineVersion` を `v3`、`baselineFields` を 9 にし、`Baseline`・`newBaseline`・`ParseBaseline`・`String` に `ignored`・`ignoredRecord` を加える（設計書 3.6）。`ParseBaseline` は 2 つの新しい欄を 64 文字の小文字の 16 進数として検査する。
-   [ ] `PartIgnored` と `GitStateParts` を加える（設計書 3.1）。

**完了条件**: `go build ./...` が通る（既存のテストの期待はステップ 5-5 で改める）。

#### ステップ 5-2: 列挙の runner

**変更するファイル**: `internal/fixprguard/worktree.go`、`cmd/fixprguard/main.go`、`internal/fixprguard/test_helpers_git.go`、`cmd/fixprguard/test_helpers.go`

-   [ ] `Worktree` に列挙の runner を加え、`NewWorktree` の引数にする（設計書 3.3）。フェーズ 3 の列挙の関数は、この欄を使う。
-   [ ] `osEnv` で、列挙の runner を `StderrIsFailure`・`MaxOutput: gitMaxOutput`・`WaitDelay`・`Env: gitEnv` で作り、`main.go:350` で渡す。ほかの git の runner には `StderrIsFailure` を付けない（設計書 3.3）。
-   [ ] テストの `NewWorktree` の呼び出し（1.3.1）に、`StderrIsFailure` の実際の git の runner を渡す。

**完了条件**: `go build ./...` と `go vet -tags test ./...` が通る。

#### ステップ 5-3: `Baseline` と `CheckState` での比較

**変更するファイル**: `internal/fixprguard/gitstate.go`、`internal/fixprguard/ignored.go`

-   [ ] `Baseline` の最後で `collectTargets` を呼び、`ignored` と `ignoredRecord`（記録の内容の SHA-256）を基準値に入れる（設計書 3.6・3.8）。
-   [ ] `CheckState` の hooks の比較の後で `collectTargets` を呼び、`ignored` と比べる（設計書 3.7）。違えば止まった時点の記録のファイルを書き、`GitStateError{Parts: []string{PartIgnored}}` を返す。記録を書けなくても `stop` は変えない（設計書 3.8「止まった時点の記録」）。状態を求められなければ、`collectTargets` の `git-failed` のエラーを返す。
-   [ ] 記録のファイルの書き込みを、設計書 3.8「書き方」のとおり実装する（同じディレクトリの `os.CreateTemp`、書いてから `rename`、失敗したら一時ファイルを消す）。置き場所は `locateGitDirs` の作業ツリーごとの git のディレクトリ（設計書 1.2）。
-   [ ] `CheckState` と `Baseline` のコメントに、対象のファイルを最後に比べることと、その理由（設計書 3.7「順序の理由」）を英語で書く。

**完了条件**: `go build ./...` が通る。

#### ステップ 5-4: `begin` の記録のファイル

**変更するファイル**: `internal/fixprguard/ignored.go`、`internal/fixprguard/worktree.go`、`cmd/fixprguard/main.go`

-   [ ] `WriteBeginRecord` を、設計書 3.8「`begin` の記録と基準値の結び付け」のとおり実装する。状態を求め直し、記録の内容の digest が基準値の `ignoredRecord` と一致するときだけ書く。一致しなければ書かずに `git-state-changed`（`ignored`）を返す。書けなければ `write ignored begin record` の `git-failed` を返す。
-   [ ] `Worktree.Begin` と `main.go` の `begin` で、`Baseline` の後に `WriteBeginRecord` を呼ぶ。`begin` は記録を書き終えてから基準値を出力する（設計書 3.8「書き方」）。

**完了条件**: `go build ./...` が通る。

#### ステップ 5-5: 既存のテストを直す

**変更するファイル**: `internal/fixprguard/gitstate_test.go`、`internal/fixprguard/worktree_flow_test.go`、`cmd/fixprguard/test_helpers.go`、`cmd/fixprguard/main_test.go`

-   [ ] 1.3.1 の表の既存のテストを直す（`TestBaselineMalformedRejected`、`validBaseline`、`validArgs`、`TestPreviousBaselineRejected`、`TestBeginOrder`、呼び出しの列を比べるテスト）。期待の変え方は 1.3.1 の表のとおりで、ほかの期待は変えない。
-   [ ] `cmd/fixprguard/test_helpers.go` のテスト用のリポジトリでも、`core.excludesFile`・`core.ignorecase`・`core.precomposeunicode` を固定する（1.4）。
-   [ ] `make test` の全体が通ることを確かめる。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

#### ステップ 5-6: 組み込みのテスト

**変更するファイル**: `internal/fixprguard/ignored_test.go`、`internal/fixprguard/worktree_flow_test.go`、`cmd/fixprguard/main_test.go`

-   [ ] 状態の比較のテストを、`CheckState` を通して加える（設計書 7.1「状態の比較」の各変化で `parts` が `ignored` だけであること、変化しない 2 つの場合で止まらないこと）。AC-13 の行は、`begin` の後に新しい `.gitignore` を置いてから、そこで無視される `.go` を作る。
-   [ ] `begin` が止まらないことのテストを加える（設計書 7.1「`begin` が止まらないこと」）。
-   [ ] 記録のファイルのテストを加える（設計書 7.1「記録のファイル」）。`begin` の記録の digest が基準値の `ignoredRecord` と一致すること、`Baseline` の後に対象のファイルが変わると `WriteBeginRecord` が書かずに止まること、止まった時点の記録が書かれること、記録の名前にあるシンボリックリンクの先に書かないこと、一時ファイルが残らないこと。
-   [ ] `finish` と `push` のテストを `worktree_flow_test.go` に加える（設計書 7.2）。fake の make の `Effect` で `make test` の中で対象のファイルを書くと、`finish` が `git-state-changed` で止まり commit を作らないこと（AC-10）。`finish` の前に書くと build checks を 1 つも実行しないこと（AC-09）。`finish` の後に書くと `push` が push しないこと（AC-11）。
-   [ ] `cmd/fixprguard/main_test.go` に、出力のテストを加える。対象のファイルがあり変わらないとき、`inspect`・`finish`・`push` の出力が今までと同じであること（AC-02）。`ignored` で止まったときの出力に `parts` が `ignored` だけで、標準出力・標準エラー・基準値に、識別できる文字列を名前と内容に入れた対象のファイルのパス・名前・内容が現れないこと（AC-14・AC-15。`TestConfigStopOmitsNames`（`main_test.go:347`）の形にならう）。

**壊して確かめる**: 次で、それぞれ括弧の中のテストが失敗すること。

-   `CheckState` から対象のファイルの比較を外す（状態の比較のテスト、`finish`・`push` のテスト）。
-   `finish` の `make test` の後の `CheckState` を外す（AC-10 のテスト）。
-   `WriteBeginRecord` の一致の検査を外す（記録のファイルのテスト）。
-   止まったときの診断に記録のパスを入れる（AC-15 のテスト）。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

### フェーズ 6: `Protocol` と `fixpr.md`（設計書 8 章の 6）

#### ステップ 6-1: `Protocol` を `4` に上げる

ガードが `Protocol` と文書の番号の一致を求めるので、このステップは 1 つのコミットにする（設計書 3.9「同じ commit に入れる」）。

**変更するファイル**: `internal/fixprguard/types.go`、`.claude/commands/fixpr.md`、`.claude/commands/runpr.md`、`docs/dev/developer_guide/fixprguard_guide.ja.md`、`fixprguard_guide.md`、`docs/tasks/0032_japanese_pr_title/check_rules.sh`

-   [ ] `Protocol` を `4` にする。コメントは変えない（`types.go:17-20` は契約と範囲の規則の版と述べており、基準値の形と `parts` は契約に含まれる）。
-   [ ] 1.3.1 の表の箇所の番号を、すべて `4` にする。ガイドの基準値の例 `'v2....'` を `'v3....'` にする。そのうえで `git grep -nE -e '--protocol[ =]3' -e '"protocol":\s*3' -- .claude docs/dev docs/tasks/0032_japanese_pr_title` を実行し、`3` が残っていないことを確かめる。結果をコミットメッセージに書く。
-   [ ] `sh docs/tasks/0032_japanese_pr_title/check_rules.sh fixpr_title` が通ることを確かめる。

**壊して確かめる**: ガイドの `--protocol 4` の 1 か所を一時的に `3` に戻して、`TestDocumentedProtocolMatches` が失敗すること。

**完了条件**: `make fmt`、`make test`、`make lint` が通る。

#### ステップ 6-2: `fixpr.md` の `ignored` の扱い

**変更するファイル**: `.claude/commands/fixpr.md`

-   [ ] `parts` を列挙する 3 か所（1.3.1）に `ignored` を加える。`ignored` で止まったときに、セッションが無視されるファイルを一覧にしたり読んだりせず（`git status --ignored`・`git ls-files -o -i`・`ls .claude` などを実行しない）、利用者にガイド 8 章の手順を案内するよう書く（設計書 3.10）。文は英語で書く。

**完了条件**: `cmd/yt2column/fixpr_docs_test.go` と `cmd/fixprguard/docs_test.go` が通る（`fixpr.md` の `ignored` のガードはステップ 7-4 で加える）。

### フェーズ 7: ガイドと文書のガード（設計書 8 章の 7）

#### ステップ 7-1: ガイドの日本語版

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`

-   [ ] 設計書 3.10 の表の、ガイドの 1 章・5 章・7.1・7.2・8 章・9 章の各行のとおり改める。7.1 の「監視の一覧」の表は `WatchListPatterns` と同じ表記にする。8 章の `git-state-changed` の行は設計書 3.10 の (a)〜(e) を、`git-failed` の行は設計書 4 章の規則の名前ごとの原因と対処を書く。
-   [ ] `TestGuideDescribesConfigScope` が確かめる語と、`3. **` で始まる行が 1 つであることを保つ（1.3.1）。
-   [ ] 8 章の (a)・(b) のコマンドを、テスト用のリポジトリで実際に実行して確かめる（1.4）。PR-2 のブランチのバイナリで `begin` を実行し、対象のファイルを変えて `inspect` を `ignored` で止め、ガイドのコマンドで照合が一致することと、`diff` が変えたパスを示すことを確かめる。記録を書き換えた場合に照合が一致しないことも確かめる。`shasum -a 256` と `sha256sum` の両方で行う。結果をコミットメッセージに書く。
-   [ ] 8 章の `git-failed` の行の、上限を超えた対象のファイルを探すコマンド（設計書 3.10）も、同じリポジトリで実行し、出力のパスがエスケープされることを確かめる。
-   [ ] 書いた 7.2 の項目を、要件書 4.2 の残余のリスクと設計書 5.3 と突き合わせる。

**完了条件**: 日本語版が設計書 3.10 のとおりになっている（ステップ 7-4 のガードで確かめる）。

#### ステップ 7-2: 9 章の `Protocol` の規則

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.ja.md`

-   [ ] 9 章に、`Protocol` の番号は上げるだけで、使った番号を再び使ったり下げたりしないことを書く（設計書 3.9・3.10）。監視の一覧を変えたら 7 章の表も変えることを、一覧の更新の項目（`:326`）に加える。`TestGuideDescribesReviewerList` が確かめる 9 章の但し書きは変えない。

**完了条件**: `cmd/fixprguard/docs_test.go` が通る。

#### ステップ 7-3: ガイドの英語版と用語集

**変更するファイル**: `docs/dev/developer_guide/fixprguard_guide.md`、`docs/translation_glossary.md`

-   [ ] ステップ 7-1・7-2 の日本語版をコミットしてから、英語版の同じ節を翻訳する（1.2）。翻訳の後に、日本語版と節ごとに突き合わせる。
-   [ ] 翻訳で使った用語のうち、用語集にないもの（監視の一覧、対象のファイル、記録のファイル）を用語集に加える（`mktrans.md`「Update Glossary」）。

**完了条件**: 英語版が日本語版と節ごとに一致する。

#### ステップ 7-4: 文書のガード

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [ ] 設計書 3.10「文書のガード」の 3 つを加える。監視の一覧の表は、日英の 7.1 の「監視の一覧」の太字の見出しの後の表を `guideTable` で切り出し、`WatchListPatterns` のすべての項目がコードスパンであることを確かめる。`git-state-changed` の行は `singleLine` で取り出し、`GitStateParts` のすべての値がコードスパンであることを確かめる。`fixpr.md` は `parts` を列挙する文に `` `ignored` `` があることを確かめる。
-   [ ] 1.4 の追加のガードを、`passage`・`checkPassages` の形で加える（AC-17 の 7.1 の行、AC-18 の 7.2 の分けた項目、AC-19 の (a)〜(e) の語と 2 つの SHA-256 のコマンド、AC-19a の `git-failed` の行の原因の語）。`checkPassages` は `configScope` の 3 つの語を必ず求めるので、0036 の PR-2 が同じ関数を改めていればその形を使い、そうでなければ確かめる語の一覧を渡せる形に改めて使う（`TestGuideDescribesConfigScope` の語は変えない）。

**壊して確かめる**: 次で、それぞれのガードが失敗すること。一時的に書き換えて確かめ、戻す。

-   どちらかの版の監視の一覧の表から 1 つの項目を消し、同じコードスパンを表の外の文に残す。
-   どちらかの版の `git-state-changed` の行から `` `ignored` `` を消す。
-   `fixpr.md` の `parts` の文から `` `ignored` `` を消す。
-   どちらかの版の 7.2・8 章の各ガードの語を 1 つずつ消す。

**完了条件**: 文書のガード（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）が通る。

#### ステップ 7-5: パッケージの説明と設計書

**変更するファイル**: `docs/dev/developer_guide/package_reference.md`

-   [ ] `internal/cmdrunner` の行に `StderrIsFailure` と `ErrUnexpectedStderr` を、`internal/fixprguard` の行に、基準値と `CheckState` が監視の一覧に当たる無視されるファイルの状態を比べることと、記録のファイルを加える。書いた内容を `runner.go`・`ignored.go` のコードと突き合わせる。

**完了条件**: 上の行がコードと一致している。

#### ステップ 7-6: PR-2 の確かめ

**変更するファイル**: なし

-   [ ] `make test && make lint` を実行する。
-   [ ] 文書のガード（ステップ 7-4 の完了条件のコマンド）を実行する。
-   [ ] AC-01a を手で確かめる（1.3.2）。`extension/node_modules/` がある作業ツリーで、PR-2 のブランチからビルドしたバイナリの `begin` が止まらないことを確かめる。`begin` は GitHub を読むので、利用者の承認を得て実行する（`CLAUDE.md`「Tool Execution Safety」）。かかった時間と対象のファイルの数を本書 1.3.2 に記録する。
-   [ ] 本タスクの前のツール（`Protocol` 3）で、PR-2 のブランチの `fixpr.md` の形の `--protocol 4` の呼び出しが終了コード `2` で止まることを確かめる。`go version -m "$(command -v fixprguard)"` の `vcs.revision` が PR-2 の前の `origin/main` のコミットでなければ、そのコミットの `cmd/fixprguard` をスクラッチのディレクトリにビルドして使う。

**完了条件**: 上の 4 つが済んでいる。

### PR-2 作成ポイント: compare ignored-file state with the baseline

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 6-1 / 6-2 / 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6

**推奨タイトル**: `feat(0034): fixprguard が、git が無視するファイルのうち監視の一覧に当たるものの変化を git-state-changed で止める`

**レビュー観点**: `CheckState` を通るすべての時点（`finish` の build checks の後を含む）で比べ、`parts` が `ignored` だけになること / 標準出力・標準エラー・基準値にパスや内容が現れず、記録のファイルにだけ書くこと / `begin` の記録と基準値の `ignoredRecord` の結び付けと、ガイド 8 章のコマンドが実際の記録で動くこと / `Protocol` 4 とコマンドの文書・ガイド・`check_rules.sh` の番号、`/runpr` の前の再インストール

**実装モデル要件**: frontier-required

**判定理由**: ステップ 5-3・5-4 は、0030 の守りの層 (3) の比較と基準値の形を変える、セキュリティのゲートの変更である（`mkplan.md` step 8 の panel mode の契機）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

**`/runpr` の前**: 利用者は、PR-2 のブランチの作業ツリーで `make install-fixprguard` を実行してから `/runpr` を実行する（ガイド 3 章）。マージまでの間、`main` を取り込んでいないほかの worktree の `/fixpr`・`/runpr` は終了コード `2` で止まる。

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

**完了条件**: 5.3 の項目が要件書 4.2 の残余のリスクと同じ範囲を示している。

#### ステップ 8-3: 0030 の要件書 4.2

**変更するファイル**: `docs/tasks/0030_fixpr_untrusted_threads/01_requirements.md`

-   [ ] 4.2 の残余のリスク（`:201`）のうち、無視されるファイルの部分だけを AC-20 と同じ内容にする。ネットワークに接続することは残す（AC-20a）。
-   [ ] `Comments` をステップ 8-2 と同じ形で記録する。

**完了条件**: 4.2 の項目が AC-20 と同じことを示している。

#### ステップ 8-4: 0030 の文書のガード

**変更するファイル**: `cmd/fixprguard/docs_test.go`

-   [ ] 0030 の設計書 5.3「無視されるファイル」の項目と、要件書 4.2 の該当の項目に、本タスクへの参照と残る範囲の語があり、両文書の `Status` が `approved` で、`Comments` に本タスクの記録があることを確かめるガードを加える。0036 の PR-2 が 0030 の文書のガードを置いていれば、同じ部品を使う。

**壊して確かめる**: 注記の語を 1 つずつ消す。参照を消す。`Status` を `draft` にする。`Comments` の本タスクの記録を消す。それぞれでガードが失敗すること。

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
| M1 | 監視の一覧と `cmdrunner` の設定 | ステップ 2-1 の完了条件 |
| M2 | 対象のファイルの状態を求める処理 | PR-1 のマージ |
| M3 | 基準値 `v3` と `CheckState`・`begin` への組み込み | ステップ 5-6 の完了条件 |
| M4 | `Protocol` 4、ガイド（日英）、文書のガード | PR-2 のマージ |
| M5 | 0030 の文書とガード | PR-3 のマージ |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 2-1 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2 | `ignored.go` の監視の一覧・列挙・たどり・上限・エントリ・digest・記録の内容、`cmdrunner` の `StderrIsFailure` | frontier-required |
| PR-2 | 5-1〜5-6 / 6-1 / 6-2 / 7-1〜7-6 | 基準値 `v3`、`CheckState` と `begin` への組み込み、記録のファイル、`Protocol` 4、`fixpr.md`、ガイド（日英）、用語集、文書のガード、`package_reference.md` | frontier-required |
| PR-3 | 8-1 / 8-2 / 8-3 / 8-4 / 8-5 | 0030 の要件書と設計書、そのガード | standard |

### 3.3. 実装順序の根拠

設計書 8 章の順序に従う。

-   監視の一覧（フェーズ 1）と `cmdrunner` の設定（フェーズ 2）は、列挙（フェーズ 3）の前提である。
-   エントリと digest（フェーズ 4）は、列挙とたどりで得た対象のファイルを入力にする。
-   組み込み（フェーズ 5）は、フェーズ 1〜4 の部品を使う。`Protocol` の引き上げ（フェーズ 6）は、組み込みと同じ PR に入れる。別の PR にすると、振る舞いが変わってから番号が上がるまで、入れ直していないツールが古い規則で黙って判定する（設計書 3.9）。
-   ガイドとガード（フェーズ 7）は、`ignored` で止まる振る舞いと同じ PR に入れる（1.4）。ガードは `WatchListPatterns` と `GitStateParts` を読むので、その後に置く。
-   0030 の文書（フェーズ 8）は、ガイド 7.2 の残余のリスクの記述を下敷きにするので、最後に置く。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

設計書 7.1 の各項目を、フェーズ 1〜5 の `ignored_test.go`・`runner_test.go`・`gitstate_test.go` のテストに割り当てた（各ステップのテストの項目）。テストは一時ディレクトリの実際のリポジトリで git を動かし、ネットワークに接続しない。設計書 7.3 の壊し方は、各ステップの「壊して確かめる」に割り当てた。

### 4.2. 結合テスト

設計書 7.2 の `finish`・`push`・AC-02 は、ステップ 5-6 で `worktree_flow_test.go` と `cmd/fixprguard/main_test.go` に置く。make と gh は `cmdrunnertestutil.Fake` で置き換え、`make test` の中の書き込みは `Step.Effect` で表す（`test_helpers_git.go` の `makeStep`）。

### 4.3. セキュリティテスト

設計書 7.3 の、パス・名前・内容・ファイルごとの digest が出力と基準値に現れないことは、ステップ 5-6 の `cmd/fixprguard/main_test.go` のテストで確かめる。設計書 5.2 の表の各経路は、次のテストで確かめる。

| 経路 | テスト |
|---|---|
| `.claude/settings.local.json`・`.env`・`.envrc`・無視される `.go`・`Makefile`・`*.mk` | ステップ 5-6 の状態の比較のテスト |
| 新しい `.gitignore` で隠す | 同上（AC-13 の行） |
| ディレクトリの読み取りの許可を外す | ステップ 3-1 の git の警告のテスト |
| Unicode の大文字小文字の別名 | ステップ 1-1 の照合のテスト |
| リンクの先の書き換え、ディレクトリを指すリンク | ステップ 4-1・4-2・5-6 のテスト |
| 巨大な疎ファイル、同じ内容を指す多数のリンク | ステップ 4-1 のテスト |
| FIFO、応答しない読み取り | ステップ 3-3・4-1 のテスト |
| 出力にパスを載せる | ステップ 5-6 の AC-15 のテスト |
| 記録の名前のリンク | ステップ 5-6 の記録のファイルのテスト |

### 4.4. 文書のガード

ステップ 7-4・8-4 のガードは `cmd/fixprguard/docs_test.go` に置き、CI の `doc-guard` のジョブが実行する。ガードが確かめるのは記述の有無と構造だけなので、中身が正しいことは PR のレビューで読み合わせる。ガイド 8 章のコマンドは、ステップ 7-1 で実際の記録のファイルで実行して確かめる。

### 4.5. 後方互換性

前の版の基準値（`v2` と 7 つの欄）は受け付けない（AC-03。ステップ 5-5 の `TestPreviousBaselineRejected`）。本タスクの前のツールと新しい文書の組み合わせは、`--protocol` が一致せず終了コード `2` で止まる（既存の `TestProtocolMismatch`（`cmd/fixprguard/main_test.go:43`）と、ステップ 7-6 の手での確認）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

`test` は振る舞いを確かめる Go のテスト、`static` は文書を読む Go のガードのテスト、`manual` は PR のレビューか手での確認を表す。新しいテストの名前は仮の名前で、実装で変えたときは本書の表を同じ PR で改める（1.3.4）。

| AC | 検証 | 場所 |
|---|---|---|
| AC-01 | test | `internal/fixprguard/ignored_test.go::TestBeginWithTargets`（利用者が置いた `.claude/settings.local.json`・`.envrc`、監視の一覧に当たるパスの FIFO。ステップ 5-6） |
| AC-01a | test + manual | `ignored_test.go::TestDefaultLimitsAcceptDependencyTree`（ステップ 4-2）。`extension/node_modules/` がある作業ツリーでの `begin`（ステップ 7-6） |
| AC-02 | test | `cmd/fixprguard/main_test.go::TestRoundWithUnchangedTargets`（ステップ 5-6）、既存の `cmd/fixprguard` と `internal/fixprguard` のテストの全体（ステップ 5-5） |
| AC-03 | test | `cmd/fixprguard/main_test.go::TestPreviousBaselineRejected`、`internal/fixprguard/gitstate_test.go::TestBaselineMalformedRejected`（ステップ 5-5） |
| AC-04 | test | `ignored_test.go::TestCheckStateIgnoredChanges`（`.claude/settings.local.json` の作成の行。ステップ 5-6） |
| AC-05 | test | `ignored_test.go::TestCheckStateIgnoredChanges`（書き換えの行） |
| AC-06 | test | `ignored_test.go::TestCheckStateIgnoredChanges`（削除の行） |
| AC-07 | test | `ignored_test.go::TestCheckStateIgnoredChanges`（`.envrc`・`.env`、`build/` の `.go`・`Makefile`・`*.mk`、入れ子のリポジトリの `.go` の行）、`ignored_test.go::TestIsWatched`（ステップ 1-1）、`ignored_test.go::TestCollectTargetsWalk`（ステップ 3-2） |
| AC-08 | test | `ignored_test.go::TestCheckStateIgnoredChanges`（種類・実行の許可・リンクの文字列の行）、`ignored_test.go::TestTargetStateDigest`（ステップ 4-2） |
| AC-08a | test | `ignored_test.go::TestCheckStateIgnoredChanges`（更新時刻だけの行）、`ignored_test.go::TestTargetStateDigest` |
| AC-08b | test | `ignored_test.go::TestCheckStateIgnoredChanges`（たどる先の書き換え、たどれないリンクがたどれるようになる行）、`ignored_test.go::TestTargetEntries`（ディレクトリを指すリンクで止まる。ステップ 4-1） |
| AC-09 | test | `internal/fixprguard/worktree_flow_test.go::TestFinishStopsOnIgnoredChange`（`finish` の前に書く行。ステップ 5-6） |
| AC-10 | test | `worktree_flow_test.go::TestFinishStopsOnIgnoredChange`（`make test` の中で書く行） |
| AC-11 | test | `worktree_flow_test.go::TestPushStopsOnIgnoredChange` |
| AC-12 | test | `ignored_test.go::TestCheckStateIgnoredChanges`（監視の一覧に当たらない無視されるファイルの行）、`ignored_test.go::TestIsWatched`（当たらない例の行） |
| AC-13 | test | `ignored_test.go::TestCheckStateIgnoredChanges`（新しい `.gitignore` で隠した `.go` の行） |
| AC-14 | test | `ignored_test.go::TestCheckStateIgnoredChanges`（`parts` が `ignored` だけ）、`cmd/fixprguard/main_test.go::TestIgnoredStopOmitsNames`（出力の `parts`） |
| AC-15 | test + static | `main_test.go::TestIgnoredStopOmitsNames`（ステップ 5-6）、`ignored_test.go::TestIgnoredRecordFiles`（パスを記録のファイルにだけ書く）。`cmd/fixprguard/docs_test.go::TestFixprNamesIgnoredPart`（`fixpr.md` の `ignored`。ステップ 7-4） |
| AC-16 | test | `ignored_test.go::TestParseIgnoredListing`（形の検査。ステップ 3-1）、`::TestCollectTargetsGitWarning`（ステップ 3-1）、`::TestCollectTargetsLimits`（ステップ 3-3）、`::TestTargetEntries`（読めない・大きさ・合計・ディレクトリを指すリンク・FIFO。ステップ 4-1）、`internal/cmdrunner/runner_test.go::TestStderrIsFailure`（ステップ 2-1） |
| AC-17 | static + manual | `docs_test.go::TestGuideListsWatchList`（7.1 の表と行。ステップ 7-4）。説明の文は PR-2 のレビューで読み合わせる |
| AC-18 | static + manual | `docs_test.go::TestGuideDescribesIgnoredFiles`（7.2 の分けた項目の語）。各項目の範囲は PR-2 のレビューで要件書 4.2 と読み合わせる |
| AC-19 | static + manual | `docs_test.go::TestGuideNamesGitStateParts`（`git-state-changed` の行の `parts`）、`docs_test.go::TestGuideDescribesIgnoredFiles`（(a)〜(e) の語と 2 つの SHA-256 のコマンド）、`ignored_test.go::TestIgnoredRecordFiles`（記録と `ignoredRecord` の一致）。コマンドの実行（ステップ 7-1） |
| AC-19a | static + manual | `docs_test.go::TestGuideDescribesIgnoredFiles`（`git-failed` の行の原因の語）。対処の中身は PR-2 のレビューで読み合わせる |
| AC-20 | static + manual | `docs_test.go::TestTask0030DescribesIgnoredFiles`（ステップ 8-4）。範囲は PR-3 のレビューで要件書 4.2 と読み合わせる |
| AC-20a | static + manual | `docs_test.go::TestTask0030DescribesIgnoredFiles`。同上 |

-   AC-02 の「出力が `protocol` を除いて本タスクの前と同じ」は、既存のテストが期待を変えずに通ること（ステップ 5-5 で、期待を変えるのは 1.3.1 の表の行だけ）と、対象のファイルがあるときの出力のテストで確かめる。
-   `Protocol` が `4` に上がることは、PR-2 の差分で確かめる（`manual`）。定数をそれ自身のリテラルと比べるテストは書かない（`CLAUDE.md`「Testing Strategy」）。

### 5.1. 横断の検索

`make test`・`make lint` とガードが検出しないものだけを挙げる。どれも `manual` で、実行した検索とその結果を、そのステップのコミットメッセージに書く。

-   [ ] ステップ 6-1 の `git grep` で、`.claude/`・`docs/dev/`・`check_rules.sh` に `--protocol 3`・`"protocol":3` が残っていない。`docs/tasks/` の下の `.md` の文書の番号は、当時の記録として変えない。
-   [ ] ガイドの日英で、基準値の例が `'v3....'` になっている。
-   [ ] ガイド 7.2 の、今の「`.gitignore` が無視するファイル、…」の項目（`:281`）の古い文が、分けた項目の後に残っていない。

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 2-1 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2）
- [ ] PR-2 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 6-1 / 6-2 / 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6）
- [ ] PR-3 マージ済み（対象ステップ: 8-1 / 8-2 / 8-3 / 8-4 / 8-5）

## 7. リスク管理 (Risk Management)

### 7.1. 技術的リスク

| リスク | 対策 |
|---|---|
| 照合のテストの行を `ignored.go` のデータから作り、データから項目を消すと行も消える | 行を要件書 3 章の表と例から作る（1.2）。ステップ 1-1 の壊す確認で確かめる |
| macOS の大文字小文字を区別しないボリュームと Linux の CI で、テストの結果が違う | テスト用のリポジトリで `core.ignorecase` などを固定する（ステップ 3-1・5-5）。照合はパスの文字列だけで行う（設計書 1.1） |
| CI の git の版で、列挙の出力の形や警告の出し方が違う | ステップ 3-1 のテストが CI で git を実際に動かして確かめる。違えば止まって設計を見直す |
| モード `0311` のディレクトリや FIFO が `t.TempDir` の削除を妨げる | テストの中で `t.Cleanup` でモードを戻す（ステップ 3-1） |
| 時間の上限のテストが遅い、または不安定になる | `targetLimits` に小さな時間を渡し、待つ処理を差し込んで確かめる（ステップ 3-3） |
| `CheckState` の呼び出しが増え、テストの呼び出しの列の期待が崩れる | 1.3.1 の表のテストだけを直し、ほかの期待は変えない（ステップ 5-5） |
| タスク 0036 の PR-2 とタスク 0031 の PR #448 が、ガイド 7.2・8 章・9 章、`docs_test.go`、0030 の文書を同時に変える | 後に入る PR が `main` を取り込んで競合を解く。ステップ 7-4・8-4 で、先に入った PR のガードの部品があれば使う |
| `Protocol` の番号の固定箇所の漏れ | ステップ 6-1 の `git grep` と `check_rules.sh` の実行、`TestDocumentedProtocolMatches` |
| PR-2 の `/runpr` が、インストール済みのツールの番号の不一致で止まる | PR-2 の作成ポイントの「`/runpr` の前」のとおり再インストールする。忘れても終了コード `2` で止まるだけで、古い規則では判定しない |
| 正当な書き込み（許可の確認への回答、Claude Code の実行時のファイル）で止まり、利用者が確かめずにやり直すことに慣れる | 要件書 4.2・設計書 5.3 の残余のリスクとしてガイド 7.2 に書き、8 章の (e) で案内する（ステップ 7-1） |

### 7.2. スケジュールのリスク

-   ステップ 7-6 の AC-01a の確認は、ネットワークに接続する操作（`make ext-install` か `begin` の GitHub の読み取り）に利用者の承認が要る。承認を待つ間も、ほかの確かめは進められる。
-   PR-2 は大きい（1.4）。レビューが長引いたら、ガイドの英語版の確かめ（ステップ 7-3）を先に済ませ、レビュアーが日本語版と Go の変更に集中できるようにする。

## 8. 成功基準 (Success Criteria)

-   **機能:** 要件書の AC-01〜AC-20a のすべてに、5 章の `test` か `static` の検証があり、通る。
-   **品質:** `make test`・`make lint`・文書のガードが通る。各テストが目的の理由で失敗できることを、壊して確かめ、コミットメッセージに書いた（`CLAUDE.md`「Testing Strategy」）。
-   **セキュリティ:** 4.3 の表の各経路のテストが通る。出力と基準値にパスや内容が現れない（AC-15）。
-   **文書:** ガイド（日英）、`fixpr.md`、`package_reference.md`、用語集、0030 の 2 つの文書が、新しい保証と残余のリスクを示している。

## 9. 次のステップ (Next Steps)

-   PR-3 のマージの後、issue #424 を閉じる（`/runpr` が最後の計画の PR の後に提案する）。
-   #446（既存の clean の検査と変更の集合に、標準エラーの検査を広げる）で、`cmdrunner` の `StderrIsFailure` を使う（設計書 9 章）。
