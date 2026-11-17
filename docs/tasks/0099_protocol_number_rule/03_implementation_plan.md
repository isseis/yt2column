# 実装計画書：プロトコル番号は文書に書かず「main の値 + 1」とし、マージの時点で確かめる

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-17 |
| Review date | 2026-11-17 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を指す。[02_architecture.md](02_architecture.md) を以下、設計書と呼ぶ。「設計書 3.1」のように「設計書」を付けた節番号は設計書の節を、「§1.3」のように § を付けた節番号は本計画の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `160435ca` で確かめた。設計書が確かめたコミットは `e638505d` である。設計書が引く `internal/mergepr`・`cmd/mergepr`・`internal/cmdrunner`・`internal/doctestutil` の行番号は、`160435ca` でも同じ行を指す（§1.4 で引いた行を開いて確かめた）。

用語は、要件書 3 章（プロトコル番号・道具・PR のコミット・番号を変えた PR・期待する値）と、設計書の冒頭（番号の確かめ・読む版・宣言の行・受け付ける行・版ごとの値・判定）と同じ意味で使う。本計画は、次の用語を加える。

-   **確かめの本体:** 設計書 3.1・3.2・4.1 の、番号の読み取りと番号の確かめ。`internal/mergepr/protocolcheck.go` に置き、PR-1 では `Prepare`・`Merge` からまだ呼ばない。
-   **入れ替えの手順:** PR-2 の `/runpr` の前後に人が行う、`mergepr` の入れ直しの手順（設計書 8 章の「実装の PR の扱い」、§5 の R-03）。
-   **示すコマンド:** `mergepr` が誤りの文で利用者に示す `mergepr` のコマンド（設計書 3.5 の最後の段落）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

プロトコル番号を上げる PR が、マージの時点の `origin/main` の値 + 1 でしかマージされないようにする。確かめを `mergepr` の `prepare` と `merge` に入れ、`mergepr` にもプロトコル番号を持たせる。あわせて、番号の決め方の規則を CLAUDE.md に置き、`fixprguard_guide.md` からはそれを参照する（要件書 2.1）。

### 1.2. 実装の原則

-   確かめの判定は Go のコードで行い、実際の git のリポジトリでテストする（設計書 1.1、H-05）。
-   読めないもの、git の失敗は「ない」や「変えていない」に変えず、判定を「読めない」として止める（設計書 1.1・3.1 の「git の失敗」）。
-   道具（`mergepr`）、それを呼ぶ文書（`mergepr.md`・`runpr.md`・`mergepr_guide.md`）、ガード（`cmd/mergepr/docs_test.go`）は、同じ PR で変える（ADR-0014 の「道具の引数・出力・終了コードは…契約にする」）。
-   テストは、誤りの型を `errors.Is`・`errors.AsType` で確かめ、誤りの文の中身に頼らない（設計書 4.1）。

### 1.3. 計画の際に決めたこと（2026-11-17）

1.  **PR は 2 本に分ける。PR-1 は確かめの本体とそのテストだけで、`Prepare`・`Merge` からはまだ呼ばない。PR-2 は、組み込み・`cmd/mergepr` の `--protocol`・コマンドの定義・手引き・CLAUDE.md・ガード・CI をまとめる。** 確かめの本体は、マージのコミットの扱い、読めない値、shallow、base、消した道具の判定と、実際の git の履歴を組むテストを持ち、このタスクで最も重いので、単独でレビューできるように分ける。確かめの本体は、呼ばれない限り `mergepr` の振る舞いも引数も変えないので、ADR-0014 の「道具と、それを呼ぶ文書と、ガードは同じ PR で変える」に触れない。`make deadcode` は、テストから届く関数を到達できると数える（ADR-0038）。設計書 8 章は 1 本を勧めていたが、分け方は計画で決めるとしていた。PR-2 を、組み込みと `--protocol` の 2 本にさらに分ける案は採らない。組み込みだけを先に入れると、確かめを持つのに版を照合しない `mergepr` が `main` に入る。これは、古い定義と組み合わせても確かめが黙って抜けないことを `--protocol` で保証するという要件書の決定（AC-16a）と、PR-2 のマージまで食い違うためである。ADR: なし（PR の分け方はこのタスクの中で閉じるため）。
2.  **確かめの本体の git の呼び出しは、`Tool` の `run` とは別の `Runner` で行う。`runner.go` に、作業ディレクトリを引数に取り、`cmdrunner.GitEnv(os.Environ())` を環境とする本番の `Runner` を作る関数を足す。環境は、その関数を呼んだ時点で取り込む。`New` は、作業ディレクトリを空（今のディレクトリ）としてこの `Runner` から確かめの本体を組み立て、`cmd/mergepr` からの呼び方（`mergepr.New(mergepr.NewOSRunner())`、`cmd/mergepr/main.go:28`）は変えない。確かめの本体のテストは、同じ関数に一時のリポジトリのディレクトリを渡して組み立てる。** 設計書 3.1 は「この環境で git を動かす別の `Runner` を持たせる」とし、3.7 は差し替え口を定めるが、組み立てる場所は定めていない。テストが本番の関数を通るので、PR-1 でもこの関数はテストから届く。`Prepare`・`Merge` が新しく行う `origin` の `main` の取得は、今の `git fetch origin`（`internal/mergepr/mergepr.go:134`）と同じく `run` で行う。設計書 3.7 が、その取得を `fakeRunner` の台本に書くとしているためである。ADR: なし（関数の形のため）。
3.  **AC-19 のテストの 3 つの集合のために、次の名前を公開する。(a) `mergepr.WorktreeProtocolTools(root string) (tools, unreadable []string, err error)`: 作業ツリーの `internal/*/` の直下の `.go` ファイル（`_test.go` を除く）に設計書 3.1 の読み方を当て、値か読めない値になった道具の名前と、そのうち読めない値の道具の名前を返す。(b) `doctestutil.ProtocolCallers(doc string, names []string)`: `doc` の各 `--protocol` について、`lastTool` と同じ読み方で、`names` のうち最後に書かれた名前を返す。どの名前も書かれていない `--protocol`（`fixpr.md` の文中の `--protocol 6` など）は、何も返さない。(c) `doctestutil.ProtocolTools()`: 文書のガードの道具の一覧の写しを返す。** 設計書 7.2 が「公開の名前は計画で決める」としたためである。(a) は製品のコードに置き、行を分類する非公開の関数を確かめの本体と共有する。`testutil/` の下（`docs/dev/conventions/test_organization.md` の分類 A）は公開された名前しか使えず、その非公開の関数に届かない。テスト用のビルドタグのファイル（分類 B）は、パッケージの中だけで使う補助のための場所である。そのため、どちらにも置かない。ADR: なし（テストの手段のため）。
4.  **確かめの本体のテストが使う git のリポジトリは、`internal/mergepr/test_helpers_git.go`（`//go:build test`）に新しく作る補助で組む。** 既存の補助（`internal/codexreview/testutil/helpers.go:533` の `NewRepo`、`internal/fixprguard/test_helpers_git.go:129`・`internal/chief/test_helpers.go:106`・`internal/coveragecompare/test_helpers.go:575`・`cmd/fixprguard/test_helpers.go:50` の `newTestRepo`）は、どれもほかのパッケージの非公開の補助であるか、ブランチ・複数の親を持つコミット・`refs/remotes/origin/main` の書き換え・shallow の複製を組む操作を持たないかの、どちらかである。空の `HOME` の用意と署名の無効化は、`internal/fixprguard/test_helpers_git.go:129` の `newTestRepo` をなぞる。ADR: なし（テストの補助の置き場所のため）。
5.  **AC-18 の `diff` の確かめは、記録した `diff` を読む形にする。比べるパスは `docs/dev/guides/fixprguard_guide.md` だけとする。** 差分は `fixprguard_guide.md` の §9 の数文の置き換えと §3 の 1 文の直しで小さく、読む量はスクリプトより少ないためである。ADR: なし（確かめの手段のため）。
6.  **`cmd/mergepr` の `run` を、引数を読むだけの関数と、サブコマンドを実行する部分に分ける。示すコマンドは、`internal/mergepr` が公開する組み立ての関数（`mergepr.ShownCommands(number int, statePath string) []string`。示しうるすべてのコマンドを返し、誤りの文は同じ組み立て方の一部を使う）で作り、誤りの文とテストはその関数を共有する。引数を読むだけの関数は `cmd/mergepr` の非公開の `parse(args []string)` とする。`base` が変わったときの「re-run prepare」（`internal/mergepr/mergepr.go:249`）も、`--protocol` を付けた `mergepr prepare` のコマンドとして示す。** 今の `run`（`cmd/mergepr/main.go:25-82`）は、引数を読んだ後すぐにサブコマンドを実行するので、テストが引数の読み取りだけを確かめられない。`cmd/fixprguard` は、読み取りと実行を分けている（`cmd/fixprguard/main.go:227` の `parse` と `:363` の `execute`）。ADR: なし（関数の形のため）。
7.  **PR-2 の入れ替えの手順の記録は、計画書ではなく、PR-2 の PR のコメントに残す。** 記録する事柄の多くは `/runpr` の途中か、マージの後に起きる。`/runpr` の途中で計画書を書き換えると、head が動き、PRE_MERGE の後の「PRE_MERGE が変えるファイルだけ」の確かめ（`.claude/commands/runpr.md` の step 4 の項 4）にも引っかかるためである。§7 は、記録の項目と置き場所だけを示す。ADR: なし（記録の置き場所のため）。

### 1.4. 既存コード調査結果

-   **`internal/mergepr`.** `Tool` は `run` だけを持つ（`internal/mergepr/mergepr.go:52-59`）。`Prepare` は、PR を読み（`:118`）、`git fetch origin`（`:134`）の後に CI を待つ（`:137`）。`Merge` は、PR が開いていることと base の名前を確かめた（`:241-250`）後に `gh pr merge --match-head-commit` を呼ぶ（`:254`）。示すコマンドは `mergepr cleanup --state %s` の 2 か所（`:246`・`:255`）と、`re-run prepare`（`:249`）である。番兵の誤りは `internal/mergepr/errors.go` にある。パッケージの説明（`:1-7`）は、安全上の約束を「head・base・CI・ローカルのブランチ」の 4 つとしている。番号の確かめを足すので書き直す。`Protocol` の定数はまだない。
-   **`internal/mergepr` のテスト.** `fakeRunner` は、台本と違う呼び出しや余分な呼び出しで、テストを失敗させ、何番目の呼び出しかを `index` に持つ（`internal/mergepr/test_helpers.go:46-68`）。`Prepare`・`Merge` を最後まで通すテストは、取得が増えると台本の書き換えが要る。`newTool`（`:78`）が、確かめの本体の差し替え口になる。
-   **`cmd/mergepr`.** `flag.ContinueOnError` の 1 つの `FlagSet` で、4 つのサブコマンドの引数を読む（`cmd/mergepr/main.go:31-35`）。使い方の文は `errUsage`（`:16`）にある。`--protocol` の照合の形は、`cmd/fixprguard/main.go:246-253` の「必須にし、文字列として比べ、違えば `errUsage` を元のエラーとして含めて返し、`--protocol does not match this binary's <N>` の形で示す」をなぞる。文字列として比べるので、`01`・`+1`・`0x1` は受け付けない。テストは `TestRunRejectsExtraArguments` だけである（`cmd/mergepr/main_test.go`）。
-   **`internal/cmdrunner`.** `GitHardening()` と `GitEnv()` がある（`internal/cmdrunner/git.go:19-45`）。`GitHardening()` は git 2.12 以上を前提とする（`:13`）。`Options.Env` と `Options.Dir` を指定した `NewOS` で、`GitEnv` の環境で、決まったディレクトリで git を動かす `Runner` を作れる（`internal/cmdrunner/runner.go:52-60`）。
-   **今ある 3 つの定数.** 受け付ける行の形で書かれている（`internal/fixprguard/types.go:21`、`internal/codexreview/types.go:15`、`internal/chief/records.go:28`）。`internal/*/` の製品のファイルで、`Protocol` を含む行のうち、空白と `const` を除いて `Protocol` で始まるものは、この 3 行だけだった。`internal/chief/output.go:342` などの複合リテラルと、`internal/codexreview/run.go:67` の `detailProtocolMismatch` は、宣言の行にならない。
-   **git の振る舞い（作業用の一時リポジトリで確かめた。git 2.50.1、2026-11-17）.**
    -   既定の pathspec では `*` が `/` を越え、`:(glob)` を付けると越えない。`git grep -n --text -F Protocol HEAD -- 'internal/*/*.go'` は `internal/a/sub/s.go` と `internal/a/a_test.go` も拾い、`-- ':(glob)internal/*/*.go' ':(glob,exclude)internal/*/*_test.go'` は `internal/a/a.go` だけを拾った。
    -   一致のない `git grep` は終了コード `1` を返した。版を渡すと、各行の前に `<版>:` が付く。
    -   サブディレクトリで実行すると、`:(glob)` のパス指定はそのディレクトリからの相対になる。`--full-name` と `:(top,glob)` を付けると、サブディレクトリからでも `HEAD:internal/a/a.go:2:const Protocol = 1` のように最上位からのパスで読めた。
    -   `git clone --depth 1 <ローカルのパス>` は `--depth is ignored in local clones; use file:// instead` と警告して shallow にならず（`git rev-parse --is-shallow-repository` は `false`）、`file://` の URL では `true` になった。その shallow なリポジトリから作った worktree でも `true` だった。worktree では `.git` がファイルなので、`.git/shallow` の有無では見分けられない。
-   **文書のガード.** `internal/doctestutil/protocol.go:23` の `protocolTools` は 3 つの道具だけを持つ。`ProtocolFlags` は、どの道具の呼び出しでもない `--protocol` を `Tool: ""` とする。`fixprguard` のガードは、`Tool` が `fixprguard` か `""` の値を、`fixprguard.Protocol`（今は `6`）と比べる（`cmd/fixprguard/docs_test.go:380-388`）。そのため、`mergepr` を一覧に足さずに `mergepr.md`・`runpr.md` に `--protocol 1` を書くと、`fixprguard` のガードが失敗する。文書の呼び出しを道具の引数の解析に通す読み取りは `doctestutil.Invocations`（`internal/doctestutil/commands.go:32`）にあり、ほかの 3 つの道具の `TestDocumentedInvocationsParse` が使っている（`cmd/fixprguard/docs_test.go:212`、`cmd/codexreview/docs_test.go:214`、`cmd/chief/docs_test.go:257`）。ほかの道具の `--protocol` を読み飛ばすことは、`TestFixprguardProtocolMismatches`（`cmd/fixprguard/docs_test.go:394`）と `TestCodexreviewProtocolMismatches`（`cmd/codexreview/docs_test.go:335`）が確かめている。
-   **`mergepr` の呼び出しの文書.** `git grep -n -E 'mergepr (prepare|merge|cleanup|discard)' -- ':!docs/tasks'` は、`.claude/commands/mergepr.md` に 5 行、`.claude/commands/runpr.md` に 12 行、`docs/dev/guides/mergepr_guide.md` に 8 行、`cmd/mergepr/main.go` に 1 行、`internal/mergepr/mergepr.go` に 2 行を挙げた（`160435ca`）。`runpr.md` の 12 行には、文中の言及（`:780` の「comes before `mergepr prepare`」、`:1212` の「after `mergepr discard`」）も含まれる。`internal/chief/testdata/screens/opencode-runpr-report.txt` の 2 行は、実際に採った画面のサンプルなので変えない。`mergepr.md` の手順 1・4（`.claude/commands/mergepr.md:34`・`:129`）と、`runpr.md` の step 4 の項 2・4・5（`.claude/commands/runpr.md:910`・`:1226`・`:1240`）が、確かめで止まったときの報告の書き換え先である。
-   **CI と CLAUDE.md の `doc-guard`.** `doc-guard` の job の対象は `.github/workflows/ci.yml:215` に、同じ一覧の文は `CLAUDE.md:707` に、文書を読むガードの一覧は `CLAUDE.md:690-698` にある。どれにも `cmd/mergepr` はない。`.claude/commands/_lib/task-pr-handoff.md:40` の CHECKS の一覧は、`doc-guard` の写しではなく、タスク文書のコマンドの確かめなので変えない（設計書 2.2 も挙げていない）。
-   **ADR-0014.** 決定の後に、まだ反映していないことを述べる段落がある（`docs/dev/adr/0014-deterministic-checks-in-go.md:27`）。設計書 3.8 のとおり、PR-2 で消す。
-   **`Makefile`.** `install-mergepr`・`install-fixprguard`・`install-codexreview`・`install-chief` の対象がある（`Makefile:82`・`:88`・`:94`・`:100`）。AC-12 の直し方が示す `make install-<道具>` の値になる。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 確かめの本体（設計書 8 章の 1、3.1・3.2・4.1）

#### ステップ 1-1: 行の読み取りと誤りの型

**変更するファイル:** `internal/mergepr/protocolcheck.go`（新規）、`internal/mergepr/errors.go`、`internal/mergepr/protocolcheck_test.go`（新規）

- [x] 設計書 4.1 の型（`protocolChecker` を除く）を `protocolcheck.go` に、番兵の誤りを `errors.go` に足す。`unreadableKind` は、零値を不正な値とし、AC-13 の (1)〜(7) と「`origin/main` に道具がない」を別の値にする（1 つの `iota` の並びに置く）。(1) の値を作るのはステップ 2-2 で、PR-1 では使われない。
- [x] 1 つの版について、パッケージごとの行の一覧から道具ごとの版ごとの値を求める非公開の関数を書く（設計書 3.1 の宣言の行・受け付ける行・9 桁の上限）。git にも作業ツリーにも当てられるように、行の取り出し方とは分ける。
- [x] 誤りの文を書く（設計書 4.2）。AC-12 の手順は、食い違った道具ごとの `make install-<道具>` を含める。AC-15 の直し方を加えるかどうかは、食い違いごとに判断する非公開の関数で決め、誤りの文はその結果で分ける。
- [x] `protocolcheck_test.go::TestClassifyProtocolLines` で、設計書 7.1 の「読み取り」の場合を、行を直接渡して表で確かめる。9 桁の上限は両側（`999999999` は値、`1000000000` は読めない値）を確かめる。1 つのファイルと 2 つのファイルにまたがる 2 行の宣言を、どちらも読めない値とすることも確かめる。
- [x] `protocolcheck_test.go::TestMismatchRemedy` で、`fixprguard`・`codexreview`・`chief`・`mergepr` のそれぞれについて、食い違いの誤りの文が示す `make install-<道具>` が `Makefile` の対象にあること（`../../Makefile` を読む）と、AC-15 の直し方を加えるかの判断の結果を確かめる。

#### ステップ 1-2: 版の木から番号を読む

**変更するファイル:** `internal/mergepr/runner.go`、`internal/mergepr/protocolcheck.go`、`internal/mergepr/test_helpers_git.go`（新規、`//go:build test`）、`internal/mergepr/protocolcheck_test.go`、`internal/config/envaccess_test.go`

- [x] `runner.go` に、§1.3 の決定 2 の `Runner` を作る関数を足す。`internal/config/envaccess_test.go` の `TestEnvAccessConfined` の許可の一覧に、この関数の `os.Environ` の参照を足す。
- [x] 確かめの本体の git の呼び出しの前に、毎回 `cmdrunner.GitHardening()` を付ける。
- [x] 1 つの版の、すべての道具の版ごとの値を読む関数を書く。`git grep` に固定の文字列 `Protocol` と、`--text`・`--full-name`・出力の形を固定する引数を渡す。パス指定は `:(top,glob)` で `internal/*/*.go` に限り、`_test.go` を除く（§1.4 の「git の振る舞い」）。終了コード `1` だけを「宣言の行がない」と読み、それ以外の失敗は読めないとする。
- [x] `test_helpers_git.go` に、一時ディレクトリの git のリポジトリを組む補助を書く（§1.3 の決定 4）。複数の親を持つコミットは、マージの仕組みに頼らずに、木と親を指定して作る。shallow の複製は `file://` の URL で作る。`exec.Command` とファイルの書き込みには、`internal/fixprguard/test_helpers_git.go` と同じく、その行だけに理由付きの `//nolint:gosec` を付ける。
- [x] 宣言の行を持つ Go のソースは、`_test.go` のファイルの中だけで組み立てる。`test_helpers_git.go` は確かめの本体が読むファイルなので、そこに書かない（設計書 3.1 の末尾）。
- [x] `protocolcheck_test.go::TestReadRevisionProtocols` で、実際の git の木について、次を確かめる。下の階層のパッケージと `_test.go` を読まない。ビルドの制約のあるファイルを読む。宣言の行のない版を「ない」とする。存在しない版を読めないとする。
- [x] 同じテストで、利用者の設定と環境に左右されないことを、基準の結果と同じ値になることで確かめる。対象は、`HOME` の `.gitconfig` の `grep.*`・`color.*`・`core.quotePath` の設定、`refs/replace` の置き換え、`GIT_DIR` の環境変数、サブディレクトリを作業ディレクトリにした実行である。テストは、確かめの本体を組み立てる前に `HOME` と環境変数を設定する。

#### ステップ 1-3: 番号の確かめ

**変更するファイル:** `internal/mergepr/protocolcheck.go`、`internal/mergepr/protocolcheck_test.go`

- [x] 確かめの本体の型と、`check(ctx, headOID, baseRefName)` の方法を書く（設計書 3.7 の形）。設計書 3.2 の規則 1〜6 を、6.1 の順に当てる。shallow は `git rev-parse --is-shallow-repository` で確かめ、出力が `false` でなければ読めないとする（§1.4）。PR のコミットと親は `git rev-list` で列挙し、`git log` は使わない。親のないコミットの親の値は「ない」とし、途中の読めない値はほかのどの値とも違う値として扱う。通ったときは `protocolSummary` を返す。
- [x] `protocolcheck_test.go::TestCheckProtocolHistories` で、設計書 7.1 の「番号の確かめ」に挙げた履歴のうち、通すものと食い違うものを、実際の git のリポジトリで組んで確かめる。食い違いは `errors.AsType` で、道具・`origin/main` の値・head の値・番号を変えたコミットを確かめる。次の場合を含める。
    - AC-10 の新しい道具と足し直す道具は、通す場合（head が `1`）に加え、食い違う場合（`origin/main` の値が「ない」で、head が `1` でない）も組む。消した道具が食い違う道具と並ぶ PR では、消した道具が食い違いに入らないことを確かめる。
    - AC-13(3) のただし書きの 2 つ（消した道具、道具より古いブランチ）は、通す場合として組む。
    - `origin/main` という名前のローカルのブランチを、`refs/remotes/origin/main` とは判定が変わる値のコミットに向けて組む。
    - 通す場合の 1 つで、`protocolSummary` の `MainTools` と `Changed` を確かめる。この場合は、同じファイルの `TestCheckProtocolSummary` に置く。
- [x] `protocolcheck_test.go::TestCheckProtocolUnreadable` で、AC-13 の (2)〜(7) と、`origin/main` に道具がない場合を確かめる。読めないものの種類は `unreadableKind` の値で確かめる。(2) は存在しない head のコミット ID で、(3) は道具を足した後に `main` を取り込み、その衝突を「ない」で解いた履歴で、(7) は `file://` の shallow な複製と、親のオブジェクトを消したリポジトリで作る。
- [x] 規則 3 のマージのコミットの扱い、規則 4 の base、規則 5 の消した道具の判定、`refs/remotes/origin/main` の完全な名前、`GitHardening()` と `--full-name` を、それぞれ一時的に壊し、上の 3 つのテストのどれかが失敗することを確かめる。結果はコミットのメッセージに書く（CLAUDE.md の「Testing Strategy」）。

**完了の条件:** グリーンゲート（`_context.md` の "Green gate"）と `make deadcode` が通る。

### PR-1 作成ポイント: protocol number check core in internal/mergepr

**対象ステップ**: 1-1 / 1-2 / 1-3

**推奨タイトル**: `feat(0099): プロトコル番号の確かめの本体を mergepr に足す`

**レビュー観点**: マージのコミットを、どの親とも違うときだけ番号を変えたものとして数えているか / git の失敗・読めない値・途中の読めない値を「ない」や「変えていない」に変えていないか / `git grep` が下の階層と `_test.go` を読まず、作業ディレクトリと利用者の設定に左右されず、終了コード `1` だけを「ない」と読んでいるか / 履歴のテストが、設計書 7.1 の場合をすべて実際の git で組んでいるか

**実装モデル要件**: opus

**判定理由**: ステップ 1-3 は、すべてのマージが通る経路に後から置くことになる、セキュリティゲート型の判定の本体で、`mkplan.md` の panel mode の契機に当たる。Sonnet で進めると、マージのコミットを最初の親とだけ比べるか、git の失敗を「宣言の行がない」と読み、食い違う PR を黙って通しやすい。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/959
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 組み込みと `--protocol`（設計書 8 章の 2、3.3〜3.5・3.7）

#### ステップ 2-1: `mergepr` の番号と差し替え口

**変更するファイル:** `internal/mergepr/mergepr.go`、`internal/mergepr/protocolcheck.go`、`internal/mergepr/test_helpers.go`

- [x] 設計書 3.5 の `Protocol` の定数を、受け付ける行の形で `mergepr.go` に足す。値は `origin/main` の値 + 1 で、今の `origin/main` に定数がないので `1` になる（AC-10）。
- [x] 設計書 3.7 の `protocolChecker` を足し、`Tool` に持たせる。`New` で本番の確かめの本体を組み立てる（§1.3 の決定 2）。
- [x] `test_helpers.go` に、確かめの本体の差し替えを足し、`newTool` で使う。差し替えは、決めた結果（通す・食い違い・読めない）を返し、受け取った head と base と、呼ばれた時点の `fakeRunner` の `index` を記録する。
- [x] パッケージの説明（`mergepr.go:1-7`）の安全上の約束に、番号の確かめを足す。

#### ステップ 2-2: `Prepare` と `Merge` への組み込み

**変更するファイル:** `internal/mergepr/mergepr.go`、`internal/mergepr/mergepr_test.go`

- [x] `Prepare` で、`git fetch origin` の後に、`main` を `refs/remotes/origin/main` へ取り込む refspec を明示した取得を足し、その後、CI を待つ前に、PR の head と base で確かめる（設計書 3.3）。止まれば作業ディレクトリを作らない。通れば、要約を `Prepared` に入れて返す。
- [x] `Merge` で、base の名前を確かめた後、`gh pr merge` の前に、同じ取得と、`state.json` の head と base での確かめを行う（設計書 3.4）。通れば要約を `Report` に入れる。
- [x] `main` の取得の失敗は、AC-13(1) の種類の `protocolUnreadableError` として返す。
- [x] 示すコマンドを組み立てる公開の関数を足し（§1.3 の決定 6）、`:246`・`:249`・`:255` と、`Merge` の確かめで止まったときの `discard` に使う。どれも `--protocol <Protocol>` を付ける（設計書 3.4・3.5）。
- [x] `Prepare` と `Merge` を最後まで通す既存のテストの台本に、`main` の取得を足す。
- [x] `mergepr_test.go::TestPrepareRunsProtocolCheck` で、確かめの本体が `main` の取得の後に、`gh pr view` の head と base で呼ばれることを、差し替えの記録で確かめる。食い違いと読めないでは、`gh pr checks` を呼ばずに止まり、作業ディレクトリを作らないことも確かめる（AC-14）。
- [x] `mergepr_test.go::TestMergeRunsProtocolCheck` で、確かめの本体が `main` の取得の後に、`state.json` の head と base で呼ばれることを確かめる。食い違いと読めないの両方で、`gh pr merge` を呼ばずに止まることも確かめる（AC-16）。
- [x] `mergepr_test.go::TestProtocolFetchFailureStops` で、`Prepare` と `Merge` の `main` の取得が失敗すると、確かめの本体もその後のコマンドも呼ばずに、AC-13(1) の種類の誤りで止まることを確かめる。
- [x] 確かめの本体を呼ぶ位置を取得の前に移すことと、渡す head を変えることで、`TestPrepareRunsProtocolCheck` と `TestMergeRunsProtocolCheck` が失敗することを確かめ、コミットのメッセージに書く。

#### ステップ 2-3: `cmd/mergepr` の `--protocol`

**変更するファイル:** `cmd/mergepr/main.go`、`cmd/mergepr/main_test.go`

- [x] `run` から、引数を読むだけの関数を分ける（§1.3 の決定 6）。`run` は、その関数を `mergepr.New` の前に呼ぶ。
- [x] 4 つのサブコマンドで `--protocol` を必須にし、`mergepr.Protocol` と文字列として照合する（§1.4 の `cmd/mergepr`）。違うか、渡されないときは、何も読まず、何も書かずに、`errUsage` を元のエラーとして含めた誤りで止まる。誤りの文は、道具が古ければ `main` から入れ直すこと、定義が古ければ `main` を取り込むことの 2 つを示す（設計書 3.5）。
- [x] `errUsage` の使い方の文の 4 つの形に `--protocol N` を足す。
- [x] `prepare` と `merge` が、通ったときの要約を 1 行で示す（設計書 3.3）。
- [x] `main_test.go::TestParseRequiresProtocol` で、4 つのサブコマンドについて、ほかの引数が正しい呼び出しを基準の呼び出しとする。基準の呼び出しが通ることと、`--protocol` を違う値（`01`・`+1`・`0x1`・`1junk` を含む）にした場合と外した場合に `errUsage` で拒むことを確かめる（AC-16a）。`TestRunRejectsExtraArguments` の引数に `--protocol` を足す。
- [x] `main_test.go::TestShownCommandsParse` で、§1.3 の決定 6 の関数が組み立てるすべての示すコマンドが、引数を読むだけの関数を通ることを確かめる。

**完了の条件:** `go test -tags test ./internal/mergepr ./cmd/mergepr` が通る。このフェーズだけでは `mergepr.md`・`runpr.md` がまだ `--protocol` を渡さないので、フェーズ 3 を同じ PR に入れる。

### フェーズ 3: 文書・ガード・CI（設計書 8 章の 3、3.6・3.8・7.2）

ステップ 3-1〜3-4 は、ガードどうしが互いの変更を前提とする（§1.4 の「文書のガード」）ので、1 つのコミットにまとめる。

#### ステップ 3-1: 文書のガードの道具の一覧

**変更するファイル:** `internal/doctestutil/protocol.go`、`internal/doctestutil/protocol_test.go`、`cmd/fixprguard/docs_test.go`、`cmd/codexreview/docs_test.go`

- [x] `ToolMergepr` を足し、`protocolTools` に加える。`ProtocolFlag.Tool` の説明を合わせる。
- [x] `ProtocolTools()` と `ProtocolCallers(doc, names)` を公開する（§1.3 の決定 3）。今の 3 つのガードが `--protocol` を割り当てる道具は変えない。
- [x] `protocol_test.go::TestProtocolFlags` に `mergepr` の呼び出しの行を、`protocol_test.go::TestProtocolCallers` に、`cmd/` の下の名前を渡したときと、どの名前もない `--protocol` の行を足す。
- [x] `TestFixprguardProtocolMismatches` と `TestCodexreviewProtocolMismatches` に、`mergepr` の呼び出しの `--protocol` を読み飛ばす行を足す。

#### ステップ 3-2: コマンドの定義

**変更するファイル:** `.claude/commands/mergepr.md`、`.claude/commands/runpr.md`

- [x] `mergepr` のすべての呼び出しに、位置引数より前に `--protocol 1` を書く（§1.4 の呼び出しの一覧、設計書 3.6）。文中のコードスパンの言及（`runpr.md:780`・`:1212`）も呼び出しとして読まれる（ステップ 3-4 のガード）ので、`--protocol` を付けるか、コマンドの形でない書き方に直す。`` `/mergepr <PR>` ``（`runpr.md:263`・`:621`）は、スラッシュコマンドで `mergepr` の呼び出しではないが、同じガードに読まれるので、`--protocol` を付けずに、空白を含むコードスパンにならない書き方に直す。
- [x] `mergepr.md` の手順 1・4 に、番号の確かめで止まったときの報告を書く（設計書 3.6）。
- [x] `runpr.md` の step 4 の項 2・4・5 に、止まったときの扱いを書く（設計書 3.6、1.2 の承認の commit の扱い、AC-14）。項 4 の今の「report and stop」は、番号の確かめの停止も含むように書き直す。`internal/chief` の画面の読み取りが対象にする、`/runpr` の質問と報告の決まった文は変えない。

#### ステップ 3-3: 手引き

**変更するファイル:** `docs/dev/guides/mergepr_guide.md`、`docs/dev/guides/fixprguard_guide.md`、`docs/dev/conventions/package_reference.md`

- [x] `mergepr_guide.md` に、設計書 3.6 の「`mergepr_guide.md`」に挙げた項目を書く。コマンドの例は、ステップ 3-4 のガードが引数の解析に通す。
- [x] `fixprguard_guide.md` の §9 と §3 を、AC-18 の 2 点だけ直す。直したコミットの後、§7 の「Diff record (AC-18)」に、`requirements_process.md` の「`diff` verification」の固定のコマンドで記録する。
- [x] `package_reference.md` の `internal/mergepr` の行に、番号の確かめを足す。

#### ステップ 3-4: 作業ツリーの道具と、文書のガード

**変更するファイル:** `internal/mergepr/protocolcheck.go`、`internal/mergepr/protocolcheck_test.go`、`cmd/mergepr/docs_test.go`（新規）

- [x] `mergepr.WorktreeProtocolTools(root)` を書く（§1.3 の決定 3）。ファイルを読む行には、ガードのテストから呼ぶ用途に限ることを理由に書いた `//nolint:gosec` を付ける。`protocolcheck_test.go::TestWorktreeProtocolTools` で、一時ディレクトリの作業ツリーについて、下の階層と `_test.go` を読まないことと、読めない値の道具を `unreadable` に返すことを確かめる。
- [x] `cmd/mergepr/docs_test.go::TestDocumentedProtocolMatches` で、コマンドの定義の `mergepr` の呼び出しの `--protocol` と、`mergepr_guide.md` の `--protocol` が `mergepr.Protocol` に等しいことを確かめる（`cmd/chief/docs_test.go:205` と同じ形）。`mergepr_guide.md` には、少なくとも 1 つあることを求める。
- [x] `cmd/mergepr/docs_test.go::TestDocumentedInvocationsParse` で、コマンドの定義と `mergepr_guide.md` の `mergepr` のすべての呼び出しを `doctestutil.Invocations` で取り出し、`<state-file>` などのプレースホルダに値を入れて、ステップ 2-3 の引数を読むだけの関数に通す（`cmd/chief/docs_test.go:257` と同じ形）。`mergepr.md` が 4 つのサブコマンドをどれも呼ぶことと、`runpr.md` が自分で呼ぶ `prepare`・`cleanup`・`discard` を呼ぶことも求める（`runpr.md` は `merge` を `mergepr.md` の手順 4 で呼ぶ）。Mermaid の図と `text` のブロック（出力の例と `/mergepr` の例）は呼び出しとして読まない。
- [x] `cmd/mergepr/docs_test.go::TestProtocolToolsAgree` で、設計書 1.2 の 3 つの集合（`WorktreeProtocolTools("../..")` の道具、コマンドの定義に `ProtocolCallers` を `cmd/` の下のディレクトリの名前で当てた結果、`ProtocolTools()`）が等しいことと、`unreadable` が空であることを確かめる（AC-19）。
- [x] 次の 2 つを確かめ、結果をコミットのメッセージに書く。`runpr.md` の呼び出しの 1 つから `--protocol` を消すと、`TestDocumentedInvocationsParse` が失敗する。`internal/mergepr` の定数を `var` に変えたときと、`ProtocolTools()` から `mergepr` を抜いたときは、`TestProtocolToolsAgree` が失敗する。

#### ステップ 3-5: CLAUDE.md・CI・ADR-0014

**変更するファイル:** `CLAUDE.md`、`.github/workflows/ci.yml`、`docs/dev/adr/0014-deterministic-checks-in-go.md`

- [x] CLAUDE.md の「Development Notes」の下に「Protocol numbers」の節を立て、設計書 3.8 の 5 項目を英語で書く（AC-01〜AC-05・AC-17・AC-20）。止まったときの直し方は `mergepr_guide.md` を参照する。
- [x] `ci.yml:215` の `doc-guard` の `go test` の対象と、`CLAUDE.md:707` の一覧の文に `./cmd/mergepr` を足す。`CLAUDE.md:690-698` の、文書を読むガードの一覧に `cmd/mergepr` のガードを足す。
- [x] ADR-0014 の、まだ反映していないことを述べる段落（`0014-deterministic-checks-in-go.md:27`）を消す（設計書 3.8）。
- [ ] PR の説明に、入れ替えの手順（§5 の R-03）と、§7 の「PR-2 の実行の記録」の項目を書く。

**完了の条件:** グリーンゲートと `make deadcode` が通る。`go test -tags test ./cmd/mergepr` が `doc-guard` の対象として通る。

### PR-2 作成ポイント: wire the protocol check into mergepr and document the rule

**対象ステップ**: 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `feat(0099): mergepr のマージでプロトコル番号を確かめ、番号の規則を CLAUDE.md に置く`

**レビュー観点**: `Prepare` が CI を待つ前に、`Merge` が `gh pr merge` の前に、`main` を取得し直してから確かめているか / すべての `mergepr` の呼び出しと示すコマンドに `--protocol` があり、`runpr.md` がマージの質問の前の停止と承認の commit の後の停止を書き分けているか / `cmd/mergepr` が `--protocol` を文字列として比べて照合の前に何も読まず、`internal/mergepr`・`internal/doctestutil` の製品の行に宣言の行がないか / CLAUDE.md の「Protocol numbers」が AC-01〜AC-05・AC-20 を満たし、`fixprguard_guide.md` の差分が AC-18 の 2 点に限り、PR の説明に入れ替えの手順があるか

**実装モデル要件**: opus

**判定理由**: ステップ 2-3 とフェーズ 3 は、古い道具と新しい定義、新しい道具と古い定義のどちらも止まる契約の変更で、ほかの worktree を止める入れ替えの手順を伴う移行であり、ステップ 2-2 はセキュリティゲートをマージの経路に置く。どちらも `mkplan.md` の panel mode の契機に当たる。Sonnet で進めると、呼び出しのどれかに `--protocol` を書き落とすか、確かめを取得の前や CI を待った後に置くか、`runpr.md` の、マージの質問の前の停止と承認の commit の後の停止を混ぜやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 対応する PR |
|---|---|---|
| M1: 確かめの本体 | `internal/mergepr/protocolcheck.go` と、実際の git の履歴でのテスト。`mergepr` の振る舞いはまだ変わらない | PR-1 |
| M2: マージの時点の確かめ | `prepare`・`merge` の確かめ、`mergepr` の `--protocol`、コマンドの定義・手引き・CLAUDE.md・ガード・CI | PR-2 |

フェーズの順序は、設計書 8 章の優先順位のとおりである。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 | 確かめの本体とそのテスト（まだ呼ばない） | opus | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | 組み込み・`--protocol`・コマンドの定義・手引き・CLAUDE.md・ガード・CI・ADR-0014 の段落の削除 | opus | auto |

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト（実際の git）.** 確かめの本体は、一時ディレクトリの git のリポジトリで履歴を組んで確かめる（ステップ 1-2・1-3、H-05）。リポジトリは利用者の git の設定に左右されないように組み、確かめの本体が設定と環境に左右されないことも確かめる（ステップ 1-2）。テストは `HOME` と環境変数を変え、作業ディレクトリを使うので、並行には走らせない。CI の `actions/checkout` は shallow だが、テストは自分のリポジトリを組むので影響を受けない。
-   **ユニットテスト（差し替え）.** `Prepare`・`Merge` への組み込みは、確かめの本体の差し替えと `fakeRunner` の台本で、呼ぶ位置と渡す値まで確かめる（ステップ 2-2）。
-   **文書のガード.** `--protocol` の値、文書の呼び出しが引数の解析を通ること、AC-19 の 3 つの集合の一致を `cmd/mergepr/docs_test.go` で確かめ、CI の `doc-guard` でも走らせる（ステップ 3-4・3-5）。CLAUDE.md と手引きの文の意味は、PR のレビューで読む（ADR-0016）。
-   **結合テスト.** `gh` を相手にした結合テストは置かない（設計書 7.3）。PR-2 自身のマージで、入れ替えの手順の中で確かめる（§7 の「PR-2 の実行の記録」）。
-   **互換性.** 古い `mergepr` と新しい定義、新しい `mergepr` と古い定義の組み合わせは、どちらも止まる（設計書 3.5）。新しい側は `TestParseRequiresProtocol` で、古い側は PR-2 の入れ替えの手順で確かめる。
-   **壊して失敗することの確かめ.** 各テストは、確かめる対象を壊して失敗することを、コミットの前に確かめ、コミットのメッセージに書く（CLAUDE.md の「Testing Strategy」）。

## 5. リスク管理 (Risk Management)

| ID | リスク | 対策 |
|---|---|---|
| R-01 | git の pathspec・作業ディレクトリ・終了コードの読み違いで、道具を 1 つも見つけずにすべての PR を通す | 設計書 3.2 の規則 1（`origin/main` に道具がなければ読めない）。`TestReadRevisionProtocols` と `TestCheckProtocolUnreadable` |
| R-02 | `internal/mergepr` の製品のファイルやテストの補助に `const Protocol = …` の行を書き、確かめが自分自身を読み違える | 宣言の行を持つ Go のソースは `_test.go` の中だけで組み立てる（ステップ 1-2）。PR-2 の `TestProtocolToolsAgree` が、作業ツリーの読めない値で失敗する |
| R-03 | PR-2 のブランチから新しい `mergepr` を入れると、マージまで、ほかのすべての worktree の `/mergepr`・`/runpr` が止まる | 入れ替えの手順を PR-2 の説明に書く（ステップ 3-5）。人が、(1) ほかの workspace の `/runpr` がマージに進まない間を選び、どの worktree にも準備したままの作業ディレクトリ（`mergepr-*`。主のチェックアウトでは `.git` の下、worktree ではその最上位）と終わっていない `cleanup` がないことを確かめ、(2) PR-2 の `/runpr` が最初に `mergepr` を呼ぶ前に、PR-2 のブランチから `make install-mergepr` で入れ、(3) マージの直後に `main` から入れ直す。途中でほかの PR をマージする必要が生じたら、`main` から入れ直す（設計書 8 章） |
| R-04 | `mergepr` を一覧に足す前に文書に `--protocol 1` を書き、`fixprguard` のガードが失敗する | ステップ 3-1〜3-4 を 1 つのコミットにまとめ、グリーンゲートで確かめる（§1.4 の「文書のガード」） |
| R-05 | PR-1 のマージから PR-2 のマージまでの間、確かめの本体は呼ばれない | 振る舞いは今と同じで、新しい危険はない。PR-2 を続けて進める |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

テストのパスはリポジトリの最上位からの相対である。

| AC | 確かめ | 種類 | ステップ |
|---|---|---|---|
| AC-01 | CLAUDE.md の「Protocol numbers」を PR のレビューで読む。対象の道具の集合そのものは `cmd/mergepr/docs_test.go::TestProtocolToolsAgree` | manual、test | 3-4 / 3-5 |
| AC-02 | CLAUDE.md の「Protocol numbers」を PR のレビューで読む | manual | 3-5 |
| AC-03 | CLAUDE.md の「Protocol numbers」を PR のレビューで読む | manual | 3-5 |
| AC-04 | CLAUDE.md の「Protocol numbers」を PR のレビューで読む | manual | 3-5 |
| AC-05 | CLAUDE.md の「Protocol numbers」と `mergepr_guide.md` の 7 章を PR のレビューで読む | manual | 3-3 / 3-5 |
| AC-06 | `internal/mergepr/protocolcheck_test.go::TestCheckProtocolHistories`（同じ値・2 以上大きい・小さい） | test | 1-3 |
| AC-07 | `internal/mergepr/protocolcheck_test.go::TestCheckProtocolHistories`（`protocolMismatchError` の道具・値・コミット）。誤りの文の形は PR のレビューで読む | test、manual | 1-1 / 1-3 |
| AC-08 | `internal/mergepr/protocolcheck_test.go::TestCheckProtocolHistories`（2 本目のブランチが `main` を取り込んだ場合） | test | 1-3 |
| AC-09 | `internal/mergepr/protocolcheck_test.go::TestCheckProtocolHistories`（番号に触れない PR、`main` を取り込んだだけの PR） | test | 1-3 |
| AC-10 | `internal/mergepr/protocolcheck_test.go::TestCheckProtocolHistories`（新しい道具・`--protocol` を足す道具・足し直す道具の通す場合と食い違う場合、消す道具） | test | 1-3 |
| AC-11 | `internal/mergepr/protocolcheck_test.go::TestCheckProtocolHistories`（2 つの道具が食い違う場合に、両方を返す） | test | 1-3 |
| AC-12 | `internal/mergepr/protocolcheck_test.go::TestMismatchRemedy`（示す `make install-<道具>` が `Makefile` の対象にある）、`cmd/mergepr/main_test.go::TestShownCommandsParse`。手順の文は、誤りの文と `mergepr_guide.md` を PR のレビューで読む | test、manual | 1-1 / 2-3 / 3-3 |
| AC-13 | (1) `internal/mergepr/mergepr_test.go::TestProtocolFetchFailureStops`。(2)〜(7) `internal/mergepr/protocolcheck_test.go::TestCheckProtocolUnreadable`、`TestReadRevisionProtocols`（git の失敗）、`TestClassifyProtocolLines`（(4)・(5) の行）。(3) のただし書きは `TestCheckProtocolHistories` | test | 1-1 / 1-2 / 1-3 / 2-2 |
| AC-14 | `internal/mergepr/mergepr_test.go::TestPrepareRunsProtocolCheck`。`runpr.md` の step 4 の項 2 は PR のレビューで読む | test、manual | 2-2 / 3-2 |
| AC-15 | `internal/mergepr/protocolcheck_test.go::TestCheckProtocolHistories`（上げてから元に戻した PR）と `TestMismatchRemedy`（AC-15 の直し方を加えるかの判断）。直し方と条件の文は PR のレビューで読む | test、manual | 1-1 / 1-3 |
| AC-16 | `internal/mergepr/mergepr_test.go::TestMergeRunsProtocolCheck`、`TestProtocolFetchFailureStops` | test | 2-2 |
| AC-16a | `cmd/mergepr/main_test.go::TestParseRequiresProtocol`、`TestShownCommandsParse`、`cmd/mergepr/docs_test.go::TestDocumentedProtocolMatches`、`TestDocumentedInvocationsParse`。古い `mergepr` が `--protocol` を拒むことは、PR-2 の入れ替えの手順で確かめる（下の記録） | test、manual | 2-3 / 3-2 / 3-4 |
| AC-17 | CLAUDE.md の「Protocol numbers」と `fixprguard_guide.md` §9 の参照を PR のレビューで読む | manual | 3-3 / 3-5 |
| AC-18 | 記録した `diff` を読む（下の「Diff record (AC-18)」）。直した文は PR のレビューで読む | diff、manual | 3-3 |
| AC-19 | `cmd/mergepr/docs_test.go::TestProtocolToolsAgree` | test | 3-4 |
| AC-20 | CLAUDE.md の「Protocol numbers」を PR のレビューで読む | manual | 3-5 |

### PR-2 の `diff` の記録

Diff record (AC-18):

- Compared paths: `docs/dev/guides/fixprguard_guide.md`
- Merge-base: `f73fc2c364a8c75fb67a833ec935818cba488605`
- Compared commit: `12efbb990bfdc8630df6cba488260c65cf524001`

Path diff:

````text
diff --git a/docs/dev/guides/fixprguard_guide.md b/docs/dev/guides/fixprguard_guide.md
index 939d5653cdc28fc5777f148a79281b2cc502f3b1..99690e4dd863093beb963551c032ad42dc4e8220 100644
--- a/docs/dev/guides/fixprguard_guide.md
+++ b/docs/dev/guides/fixprguard_guide.md
@@ -55,7 +55,7 @@ go version -m "$(command -v fixprguard)"
 - マージの直後に、マージの後の `main` で再インストールする。再インストールの前に新しい番号の文書から実行すると、終了コード `2` で止まる。
 - 作業中のほかのブランチには、次の `/fixpr`・`/runpr` の前に `main` を取り込む。取り込む前は、そのブランチの文書が古い番号を渡すので、終了コード `2` で止まる。

-どちらも、古い規則で判定することはない（fail-closed）。
+どちらも、古い規則で判定することはない（fail-closed）。ただし、同じ番号に上げる並行するブランチがある間は、この限りでない。マージの前は両方のブランチが同じ番号を共有するので、片方のブランチで入れたバイナリが、もう片方の文書の `--protocol` を受け付け、違う規則で判定しうる（CLAUDE.md の「Protocol numbers」の受け入れた危険）。

 ## 4. 共通の約束

@@ -584,7 +584,7 @@ find . -path ./.git -prune -o -type l -print | LC_ALL=C sed 's/\^/^^/g' | LC_ALL

 - 判定は `internal/fixprguard`、引数の解釈と呼び出しの順序は `cmd/fixprguard` にある。外部コマンドは `internal/cmdrunner` の `Runner`・`InputRunner` 越しに呼ぶ。テストでは、`gh` と `make` を `internal/cmdrunner/testutil` の fake に置き換え、git は一時ディレクトリの実際のリポジトリで試す。
 - 7.1 節の保証を変更する場合は、対応する実装を壊してテストが失敗することを確かめ、そのことをコミットメッセージに書く（CLAUDE.md の「Testing Strategy」）。
-- 出力や引数の契約、または範囲の規則（6 章）を変える場合は、`internal/fixprguard` の `Protocol` を上げ、コマンドの文書が渡す `--protocol` も合わせて更新する。番号は上げるだけで、一度使った番号を再び使ったり、下げたりしない。並行する 2 つのブランチが同じ番号を使うと、片方のブランチで作ったバイナリが、もう片方の文書の `--protocol` を受け付け、違う規則で黙って判定するためである（タスク 0036 が `3` を使ったので、タスク 0034 は `4` を使った。タスク 0076 は `check`・`push-review` と `parts` の `branch` を加えて `6` を使った）。ただし、次の 3 つの条件がすべて成り立つ変更に限り、`Protocol` を上げなくてよい。`--reviewer` を繰り返し受け付ける変更（タスク 0033）はこれに当たる。
+- 出力や引数の契約、または範囲の規則（6 章）を変える場合は、`internal/fixprguard` の `Protocol` を上げ、コマンドの文書が渡す `--protocol` も合わせて更新する。番号の決め方は、CLAUDE.md の「Protocol numbers」に従う。ただし、次の 3 つの条件がすべて成り立つ変更に限り、`Protocol` を上げなくてよい。`--reviewer` を繰り返し受け付ける変更（タスク 0033）はこれに当たる。
     1. 古いツールは、新しい文書の呼び出しを引数の解析で拒否し（終了コード `2`）、古い規則で判定しない。
     2. 新しいツールは、古い文書の呼び出しを今までと同じ規則で判定する。
     3. 出力の形は変えない。
````

### PR-2 の実行の記録

入れ替えの手順（§5 の R-03）の中で、人が PR-2 の PR のコメントに、実行したコマンド・その出力・そのときの head のコミット ID を残す（§1.3 の決定 7）。残す項目は次のとおりである。

- 新しい `mergepr` を入れる前に、どの worktree にも準備したままの作業ディレクトリと終わっていない `cleanup` がないことを確かめた結果。
- 入れ替える前の `mergepr` で `mergepr discard --protocol 1 --state <存在しないパス>` を実行し、`flag provided but not defined: -protocol` で拒んだこと（AC-16a）。
- PR-2 の `/runpr` の `prepare` と `merge` が示した要約。`origin/main` の道具が `fixprguard`・`codexreview`・`chief` で、番号を変えた道具が `mergepr` であること（期待する値 `1`、設計書 7.3）。

## 8. 成功の基準 (Success Criteria)

-   **機能.** 番号を変えた PR は、`origin/main` の値 + 1 でしか `mergepr` でマージされない。番号に触れない PR は、追加の操作なしにマージされる。
-   **品質.** グリーンゲートと `make deadcode` が通る。§7 のすべての `test` の行が、確かめる対象を壊すと失敗することを、実装の際に確かめている。
-   **セキュリティ.** N/A（設計書 5 章）。読めない値で止まることは、`TestClassifyProtocolLines` と `TestCheckProtocolUnreadable` が確かめる。
-   **文書.** CLAUDE.md の「Protocol numbers」、`mergepr_guide.md`、`fixprguard_guide.md` §3・§9、`package_reference.md`、`mergepr.md`、`runpr.md` が、PR-2 の実装と合っている。

## 9. 次のステップ (Next Steps)

-   PR-2 のマージの直後に、`main` から `make install-mergepr` で入れ直す（§5 の R-03）。
-   進行中のタスク（0092・0093・0095・0096）の、番号を決め打ちした文書は、それぞれのタスクが実装の PR の前に直す（要件書 2.3・4.4）。
-   「番号を変えた PR」の定義の見直し（設計書 3.2 の末尾の既知の限界）は、[#947](https://github.com/isseis/yt2column/issues/947) で扱う。

## 変更履歴

- 2026-11-17 承認（isseis、#951）
