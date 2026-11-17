# 実装計画書：プロトコル番号は文書に書かず「main の値 + 1」とし、マージの時点で確かめる

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-17 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `160435ca` で確かめた。設計書が確かめたコミットは `e638505d` である。設計書が引く `internal/mergepr`・`cmd/mergepr`・`internal/cmdrunner`・`internal/doctestutil` の行番号は、`160435ca` でも同じ行を指す（§1.4 で引いた行を開いて確かめた）。

用語は、要件書 3 章（プロトコル番号・道具・PR のコミット・番号を変えた PR・期待する値）と、設計書の冒頭（番号の確かめ・読む版・宣言の行・受け付ける行・版ごとの値・判定）と同じ意味で使う。本計画は、次の用語を加える。

-   **確かめの本体:** 設計書 3.1・3.2・4.1 の、番号の読み取りと番号の確かめ。`internal/mergepr/protocolcheck.go` に置き、PR-1 では `Prepare`・`Merge` からまだ呼ばない。
-   **入れ替えの手順:** PR-2 の `/runpr` の前後に人が行う、`mergepr` の入れ直しの手順（設計書 8 章の「実装の PR の扱い」）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

プロトコル番号を上げる PR が、マージの時点の `origin/main` の値 + 1 でしかマージされないようにする。確かめを `mergepr` の `prepare` と `merge` に入れ、`mergepr` にもプロトコル番号を持たせる。あわせて、番号の決め方の規則を CLAUDE.md に置き、`fixprguard_guide.md` からはそれを参照する（要件書 2.1）。

### 1.2. 実装の原則

-   確かめの判定は Go のコードで行い、実際の git のリポジトリでテストする（設計書 1.1、H-05）。
-   読めないもの、git の失敗は「ない」や「変えていない」に変えず、判定を「読めない」として止める（設計書 1.1・3.1 の「git の失敗」）。
-   道具（`mergepr`）、それを呼ぶ文書（`mergepr.md`・`runpr.md`・`mergepr_guide.md`）、ガード（`cmd/mergepr/docs_test.go`）は、同じ PR で変える（ADR-0014 の「道具の引数・出力・終了コードは…契約にする」）。
-   テストは、誤りの型を `errors.Is`・`errors.AsType` で確かめ、誤りの文の中身に頼らない（設計書 4.1）。

### 1.3. 計画の際に決めたこと（2026-11-17）

1.  **PR は 2 本に分ける。PR-1 は確かめの本体とそのテストだけで、`Prepare`・`Merge` からはまだ呼ばない。PR-2 は、組み込み・`cmd/mergepr` の `--protocol`・コマンドの定義・手引き・CLAUDE.md・ガード・CI をまとめる。** 確かめの本体は、マージのコミットの扱い、読めない値、shallow、base、消した道具の判定と、実際の git の履歴を組むテストを持ち、このタスクで最も重い。単独でレビューできるようにするためである。確かめの本体は、呼ばれない限り `mergepr` の振る舞いも引数も変えないので、ADR-0014 の「道具と、それを呼ぶ文書と、ガードは同じ PR で変える」に触れない。`make deadcode` は、テストから届く関数を到達できると数える（ADR-0038）ので、PR-1 でも通る。設計書 8 章は 1 本を勧めていたが、分け方は計画で決めるとしていた。ADR: なし（PR の分け方はこのタスクの中で閉じるため）。
2.  **確かめの本体の git の呼び出しは、`Tool` の `run` とは別の `Runner` で行う。`New` は、`cmdrunner.GitEnv` の環境で動く本番の `Runner` から確かめの本体を組み立て、`cmd/mergepr` からの呼び方（`mergepr.New(mergepr.NewOSRunner())`、`cmd/mergepr/main.go:28`）は変えない。テストは、`test_helpers.go` の `newTool` で確かめの本体を差し替える。** 設計書 3.1 は「この環境で git を動かす別の `Runner` を持たせる」とし、3.7 は差し替え口を定めるが、組み立てる場所は定めていない。`New` の引数を変えないのは、`cmd/mergepr` の変更を `--protocol` の照合に限るためである。`Prepare`・`Merge` が新しく行う `origin` の `main` の取得は、今の `git fetch origin`（`internal/mergepr/mergepr.go:134`）と同じく `run` で行う。設計書 3.7 が、その取得を `fakeRunner` の台本に書くとしているためである。ADR: なし（関数の形のため）。
3.  **AC-19 のテストの 3 つの集合のために、次の名前を公開する。(a) `mergepr.WorktreeProtocolTools(root string)`: 作業ツリーの `internal/*/` の直下の `.go` ファイル（`_test.go` を除く）に 3.1 の読み方を当て、値か読めない値になった道具の名前を返す。(b) `doctestutil.ProtocolCallers(doc string, names []string)`: `doc` の各 `--protocol` について、`lastTool` と同じ読み方で、`names` のうち最後に書かれた名前を返す。(c) `doctestutil.ProtocolTools()`: 文書のガードの道具の一覧の写しを返す。** 設計書 7.2 が「公開の名前は計画で決める」としたためである。(a) は製品のコードに置き、行を分類する部分を確かめの本体と共有する。テスト用のビルドタグのファイルに置くと、`internal/mergepr` の外のテストから使う補助になり、`docs/dev/conventions/test_organization.md` の分類 B（パッケージの中だけで使う補助）に合わないためである。ADR: なし（テストの手段のため）。
4.  **確かめの本体のテストが使う git のリポジトリは、`internal/mergepr/test_helpers_git.go`（`//go:build test`）に新しく作る補助で組む。** `internal/codexreview/testutil` の `NewRepo`（`internal/codexreview/testutil/helpers.go:533`）は、ブランチ・マージのコミット・親のないコミット・`refs/remotes/origin/main` の書き換え・shallow の複製を組む操作を持たない。空の `HOME` は、`codexreview` の偽物（`NewFake`）を先に作ることに頼っている。そのため、そのままでは使えない。補助は `internal/fixprguard/test_helpers_git.go:129` の `newTestRepo` と同じく、`HOME` と `XDG_CONFIG_HOME` を空のディレクトリに向け、署名を切る。ADR: なし（テストの補助の置き場所のため）。
5.  **AC-18 の `diff` の確かめは、記録した `diff` を読む形にする。比べるパスは `docs/dev/guides/fixprguard_guide.md` だけとする。** 差分は §9 の数文の置き換えと §3 の 1 文の直しで小さく、読む量はスクリプトより少ないためである。ADR: なし（確かめの手段のため）。

### 1.4. 既存コード調査結果

-   **`internal/mergepr`.** `Tool` は `run` だけを持つ（`internal/mergepr/mergepr.go:52-59`）。`Prepare` は、PR を読み（`:116`）、`git fetch origin`（`:134`）の後に CI を待つ（`:137`）。`Merge` は、PR が開いていることと base の名前を確かめた（`:241-250`）後に `gh pr merge --match-head-commit` を呼ぶ（`:254`）。誤りの文が示すコマンドは `mergepr cleanup --state %s` の 2 か所である（`:246`・`:255`）。番兵の誤りは `internal/mergepr/errors.go` にある。パッケージの説明（`:1-7`）は、安全上の約束を「head・base・CI・ローカルのブランチ」の 4 つとしている。番号の確かめを足すので書き直す。`Protocol` の定数はまだない。
-   **`internal/mergepr` のテスト.** `fakeRunner` は、台本と違う呼び出しや余分な呼び出しで、テストを失敗させる（`internal/mergepr/test_helpers.go:46-68`）。`Prepare`・`Merge` を最後まで通すテストは、取得が増えると台本の書き換えが要る。`newTool`（`:78`）が、確かめの本体の差し替え口になる。
-   **`cmd/mergepr`.** `flag.ContinueOnError` の 1 つの `FlagSet` で、4 つのサブコマンドの引数を読む（`cmd/mergepr/main.go:31-35`）。使い方の文は `errUsage`（`:16`）にある。`--protocol` の照合の形は、`cmd/fixprguard/main.go:246-253` の「必須にし、違えば `errUsage` を包んで `--protocol does not match this binary's <N>`」をなぞる。テストは `TestRunRejectsExtraArguments` だけである（`cmd/mergepr/main_test.go`）。
-   **`internal/cmdrunner`.** `GitHardening()` と `GitEnv()` がある（`internal/cmdrunner/git.go:19-45`）。`Options.Env` を指定した `NewOS` で、`GitEnv` の環境で git を動かす `Runner` を作れる（`internal/cmdrunner/runner.go:52-60`）。`GitHardening()` の引数は、呼び出しごとに前に付ける。
-   **今ある 3 つの定数.** 受け付ける行の形で書かれている（`internal/fixprguard/types.go:21`、`internal/codexreview/types.go:15`、`internal/chief/records.go:28`）。`internal/*/` の製品のファイルで、`Protocol` を含む行のうち、空白と `const` を除いて `Protocol` で始まるものは、この 3 行だけだった。`internal/chief/output.go:342` などの複合リテラルと、`internal/codexreview/run.go:67` の `detailProtocolMismatch` は、宣言の行にならない。
-   **git の pathspec.** 既定の pathspec では `*` が `/` を越え、`:(glob)` を付けると越えない。`git grep` は一致がないとき終了コード `1` を返す。スクラッチの一時リポジトリで、`git grep -n --text -F Protocol HEAD -- 'internal/*/*.go'` が `internal/a/sub/s.go` と `internal/a/a_test.go` も拾い、`-- ':(glob)internal/*/*.go' ':(glob,exclude)internal/*/*_test.go'` が `internal/a/a.go` だけを拾うこと、一致のない `git grep` が `exit=1` を返すことを確かめた（git 2.50.1、2026-11-17）。`git grep` に版を渡すと、各行の前に `<版>:` が付く。
-   **文書のガード.** `internal/doctestutil/protocol.go:23` の `protocolTools` は 3 つの道具だけを持つ。`ProtocolFlags` は、どの道具の呼び出しでもない `--protocol` を `Tool: ""` とする。`fixprguard` のガードは、`Tool` が `fixprguard` か `""` の値を、`fixprguard.Protocol`（今は `6`）と比べる（`cmd/fixprguard/docs_test.go:380-388`）。そのため、`mergepr` を一覧に足さずに `mergepr.md`・`runpr.md` に `--protocol 1` を書くと、`fixprguard` のガードが失敗する。一覧への追加と文書の変更は、同じコミットで行う。
-   **`mergepr` の呼び出しの文書.** `git grep -n -E 'mergepr (prepare|merge|cleanup|discard)' -- ':!docs/tasks'` は、`.claude/commands/mergepr.md` に 5 行、`.claude/commands/runpr.md` に 12 行、`docs/dev/guides/mergepr_guide.md` に 8 行、`cmd/mergepr/main.go` に 1 行、`internal/mergepr/mergepr.go` に 2 行を挙げた（`160435ca`）。`internal/chief/testdata/screens/opencode-runpr-report.txt` の 2 行は、実際に採った画面のサンプルなので変えない。`mergepr.md` の手順 1・4（`.claude/commands/mergepr.md:34`・`:129`）と、`runpr.md` の step 4 の項 2・4・5（`.claude/commands/runpr.md:910`・`:1226`・`:1240`）が、確かめで止まったときの報告の書き換え先である。
-   **CI と CLAUDE.md の `doc-guard`.** `doc-guard` の job の対象は `.github/workflows/ci.yml:215` に、同じ一覧の文は `CLAUDE.md:707` にある。どちらにも `./cmd/mergepr` はない。`.claude/commands/_lib/task-pr-handoff.md:40` の CHECKS の一覧は、`doc-guard` の写しではなく、タスク文書のコマンドの確かめなので変えない（設計書 2.2 も挙げていない）。
-   **ADR-0014.** 決定の後に、まだ反映していないことを述べる段落がある（`docs/dev/adr/0014-deterministic-checks-in-go.md:27`）。設計書 3.8 のとおり、PR-2 で消す。
-   **`Makefile`.** `install-mergepr`・`install-fixprguard`・`install-codexreview`・`install-chief` の対象がある（`Makefile:59`）。AC-12 の直し方が示す `make install-<道具>` の値になる。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 確かめの本体（設計書 8 章の 1、3.1・3.2・4.1）

#### ステップ 1-1: 行の読み取りと誤りの型

**変更するファイル:** `internal/mergepr/protocolcheck.go`（新規）、`internal/mergepr/errors.go`、`internal/mergepr/protocolcheck_test.go`（新規）

- [ ] 設計書 4.1 の型（`protocolState`・`protocolValue`・`protocolChange`・`protocolMismatch`・`protocolMismatchError`・`unreadableKind`・`protocolUnreadableError`・`protocolSummary`）を `protocolcheck.go` に、番兵の誤り（`errProtocolMismatch`・`errProtocolUnreadable`）を `errors.go` に足す。`unreadableKind` は、零値を不正な値とし、AC-13 の (1)〜(7) と「`origin/main` に道具がない」を別の値にする。
- [ ] 1 つの版の、パッケージごとの行の一覧から版ごとの値を求める関数を書く（設計書 3.1 の宣言の行・受け付ける行・9 桁の上限）。git にも作業ツリーにも当てられるように、行の取り出し方とは分ける。
- [ ] 誤りの文を書く。書き出しは `protocol check: ` とし、設計書 4.2 の値・コミット・直し方を示す。AC-12 の手順は、食い違った道具ごとの `make install-<道具>` を含める。head の値が `origin/main` の値に等しい道具があれば、AC-15 の直し方と条件を加える。
- [ ] `protocolcheck_test.go::TestClassifyProtocolLines` で、設計書 7.1 の「読み取り」の場合を表で確かめる（git を使わずに行を渡す。下の階層・`_test.go`・ビルドの制約は、ステップ 1-2 の実際の git のテストで確かめる）。
- [ ] `protocolcheck_test.go::TestMismatchRemedy` で、食い違いの誤りの文が、道具ごとの `make install-<道具>` を示し、その対象が `Makefile` にあること（`../../Makefile` を読む）を確かめる。AC-15 の直し方を加えるかどうかは、型の値で確かめる。

**完了の条件:** `go test -tags test ./internal/mergepr` が通る。

#### ステップ 1-2: 版の木から番号を読む

**変更するファイル:** `internal/mergepr/runner.go`、`internal/mergepr/protocolcheck.go`、`internal/mergepr/test_helpers_git.go`（新規、`//go:build test`）、`internal/mergepr/protocolcheck_test.go`

- [ ] `runner.go` に、`cmdrunner.GitEnv(os.Environ())` を環境とする本番の `Runner` を作る関数を足す（§1.3 の決定 2）。
- [ ] 確かめの本体の git の呼び出しの前に、毎回 `cmdrunner.GitHardening()` を付ける。
- [ ] 1 つの版の、すべての道具の版ごとの値を読む関数を書く。`git grep` に固定の文字列 `Protocol` を渡し、`--text` と、出力の形を固定する引数を付ける。パス指定は `:(glob)` で `internal/*/*.go` に限り、`_test.go` を除く（§1.4 の「git の pathspec」）。終了コード `1` だけを「宣言の行がない」と読み、それ以外の失敗は読めないとする。
- [ ] `test_helpers_git.go` に、一時ディレクトリの git のリポジトリを組む補助を書く（§1.3 の決定 4）。ファイルの書き込み、コミット、ブランチ、マージのコミット、親のないコミット、`refs/remotes/origin/main` の書き換え、shallow の複製を扱う。宣言の行を持つ Go のソースは、テストの中で文字列として組み立てる（設計書 3.1 の末尾）。
- [ ] `protocolcheck_test.go::TestReadRevisionProtocols` で、実際の git の木について、下の階層のパッケージと `_test.go` を読まないこと、ビルドの制約のあるファイルを読むこと、宣言の行のない版を「ない」とすることを確かめる。
- [ ] git の失敗（存在しない版）を読めないとすることを、同じテストで確かめる。

**完了の条件:** `go test -tags test ./internal/mergepr` が通る。

#### ステップ 1-3: 番号の確かめ

**変更するファイル:** `internal/mergepr/protocolcheck.go`、`internal/mergepr/protocolcheck_test.go`

- [ ] 設計書 3.7 の `protocolChecker` の型と、その本番の実装を書く。設計書 3.2 の規則 1〜6 を、6.1 の順に当てる。shallow の確かめ、`refs/remotes/origin/main` の完全な名前、PR のコミットと親の列挙（`git rev-list`。`git log` は使わない）、親のないコミットの親の値を「ない」とすること、途中の読めない値をほかのどの値とも違う値として扱うことを含める。
- [ ] 通ったときは `protocolSummary` を返す。
- [ ] `protocolcheck_test.go::TestCheckProtocolHistories` で、設計書 7.1 の「番号の確かめ」に挙げた履歴のうち、通すものと食い違うものを、それぞれ実際の git のリポジトリで組んで確かめる。食い違いは、`errors.AsType` で、道具・`origin/main` の値・head の値・番号を変えたコミットを確かめる。
- [ ] `protocolcheck_test.go::TestCheckProtocolUnreadable` で、AC-13 の (2)〜(7) と、`origin/main` に道具がない場合、`origin/main` という名前のローカルのブランチがある場合を確かめる。読めないものの種類は、`unreadableKind` の値で確かめる。
- [ ] 規則 3 のマージのコミットの扱い、規則 4 の base、規則 5 の消した道具の判定を、それぞれ一時的に壊し、上の 2 つのテストのどれかが失敗することを確かめる。結果はコミットのメッセージに書く（CLAUDE.md の「Testing Strategy」）。

**完了の条件:** グリーンゲート（`_context.md` の "Green gate"）と `make deadcode` が通る。`internal/mergepr` の製品のファイルに、宣言の行がない（PR-1 は `mergepr` の番号を足さない）。

### PR-1 作成ポイント: protocol number check core in internal/mergepr

**対象ステップ**: 1-1 / 1-2 / 1-3

**推奨タイトル**: `feat(0099): プロトコル番号の確かめの本体を mergepr に足す`

**レビュー観点**: マージのコミットを、どの親とも違うときだけ番号を変えたものとして数えているか / git の失敗・読めない値・途中の読めない値を「ない」や「変えていない」に変えていないか / `git grep` の pathspec が下の階層と `_test.go` を読まず、終了コード `1` だけを「ない」と読んでいるか / 履歴のテストが、設計書 7.1 の場合をすべて実際の git で組んでいるか

**実装モデル要件**: opus

**判定理由**: ステップ 1-3 は、マージが必ず通る経路に置くセキュリティゲート型の判定で、`mkplan.md` の panel mode の契機に当たる。Sonnet で進めると、マージのコミットを最初の親とだけ比べるか、git の失敗を「宣言の行がない」と読み、食い違う PR を黙って通しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 組み込みと `--protocol`（設計書 8 章の 2、3.3〜3.5・3.7）

#### ステップ 2-1: `mergepr` の番号と差し替え口

**変更するファイル:** `internal/mergepr/mergepr.go`、`internal/mergepr/test_helpers.go`

- [ ] 設計書 3.5 の `Protocol` の定数を、受け付ける行の形で `mergepr.go` に足す。値は `origin/main` の値 + 1 で、今の `origin/main` に定数がないので `1` になる（AC-10）。
- [ ] `Tool` に確かめの本体を持たせ、`New` で本番の確かめの本体を組み立てる（§1.3 の決定 2）。
- [ ] `test_helpers.go` に、結果（通す・食い違い・読めない）を返す確かめの本体の差し替えを足し、`newTool` で使う。
- [ ] パッケージの説明（`mergepr.go:1-7`）の安全上の約束に、番号の確かめを足す。

#### ステップ 2-2: `Prepare` と `Merge` への組み込み

**変更するファイル:** `internal/mergepr/mergepr.go`、`internal/mergepr/mergepr_test.go`

- [ ] `Prepare` で、`git fetch origin` の後に、`main` を `refs/remotes/origin/main` へ取り込む refspec を明示した取得を足し、その後、CI を待つ前に確かめる（設計書 3.3）。止まれば作業ディレクトリを作らない。通れば、要約を `Prepared` に入れて返す。
- [ ] `Merge` で、base の名前を確かめた後、`gh pr merge` の前に、同じ取得と、`state.json` の head と base での確かめを行う（設計書 3.4）。止まったときの誤りの文は、`mergepr discard --protocol <Protocol> --state <state-file>` で準備を捨ててから直すことを示す。通れば要約を `Report` に入れる。
- [ ] 誤りの文が示すコマンド（`mergepr.go:246`・`:255` の `mergepr cleanup --state %s` と、上の `discard`）に、すべて `--protocol <Protocol>` を付ける（設計書 3.5）。
- [ ] `Prepare` と `Merge` を最後まで通す既存のテストの台本に、`main` の取得を足す。
- [ ] `mergepr_test.go::TestPrepareStopsOnProtocolCheck` で、確かめが食い違いか読めないを返すと、`Prepare` が `gh pr checks` を呼ばずに止まり、作業ディレクトリを作らないことを確かめる（AC-14）。
- [ ] `mergepr_test.go::TestMergeStopsOnProtocolCheck` で、`Merge` が `gh pr merge` を呼ばずに止まることを確かめる（AC-16）。
- [ ] `mergepr_test.go::TestProtocolFetchFailureStops` で、`Prepare` と `Merge` の `main` の取得が失敗すると、確かめもその後のコマンドも呼ばずに止まることを確かめる（AC-13(1)）。

#### ステップ 2-3: `cmd/mergepr` の `--protocol`

**変更するファイル:** `cmd/mergepr/main.go`、`cmd/mergepr/main_test.go`

- [ ] 4 つのサブコマンドで `--protocol` を必須にし、`mergepr.Protocol` と照合する。違うか、渡されないときは、何も読まず、何も書かずに `errUsage` を包んだ誤りで止まる。誤りの文は、道具が古ければ `main` から入れ直すこと、定義が古ければ `main` を取り込むことの 2 つを示す（設計書 3.5）。
- [ ] `errUsage` の使い方の文の 4 つの形に `--protocol N` を足す。
- [ ] `prepare` と `merge` が、通ったときの要約を 1 行で示す（設計書 3.3）。
- [ ] `main_test.go::TestRunRequiresProtocol` で、4 つのサブコマンドについて、違う値と、渡されない場合を `errUsage` で拒むことを確かめる（AC-16a）。`TestRunRejectsExtraArguments` の引数に `--protocol` を足す。
- [ ] `main_test.go::TestShownCommandsParse` で、`internal/mergepr` が誤りの文で示すコマンドが `--protocol` を含み、`run` の引数の解析を通ることを確かめる（設計書 7.1）。示すコマンドを取り出すために `internal/mergepr` に要る公開の口は、実装の際に決める。

**完了の条件:** `go test -tags test ./internal/mergepr ./cmd/mergepr` が通る。このフェーズだけでは `mergepr.md`・`runpr.md` がまだ `--protocol` を渡さないので、フェーズ 3 を同じ PR に入れる。

### フェーズ 3: 文書・ガード・CI（設計書 8 章の 3、3.6・3.8・7.2）

#### ステップ 3-1: 文書のガードの道具の一覧

**変更するファイル:** `internal/doctestutil/protocol.go`、`internal/doctestutil/protocol_test.go`

- [ ] `ToolMergepr` を足し、`protocolTools` に加える。`ProtocolFlag.Tool` の説明を合わせる。
- [ ] `ProtocolTools()` と `ProtocolCallers(doc, names)` を公開する（§1.3 の決定 3）。`ProtocolCallers` は `lastTool` と同じ語の読み方を使い、今の 3 つのガードが `--protocol` を割り当てる道具は変えない。
- [ ] `protocol_test.go::TestProtocolFlags` に `mergepr` の呼び出しの行を足す。`protocol_test.go::TestProtocolCallers` で、`names` に `cmd/` の下の名前を渡したときの読み方を確かめる。

#### ステップ 3-2: 作業ツリーの道具と、AC-19・文書のガードのテスト

**変更するファイル:** `internal/mergepr/protocolcheck.go`、`internal/mergepr/protocolcheck_test.go`、`cmd/mergepr/docs_test.go`（新規）

- [ ] `mergepr.WorktreeProtocolTools(root)` を書く（§1.3 の決定 3）。行を分類する部分は、ステップ 1-1 の関数を使う。`protocolcheck_test.go::TestWorktreeProtocolTools` で、一時ディレクトリの作業ツリーについて、下の階層と `_test.go` を読まないことを確かめる。
- [ ] `cmd/mergepr/docs_test.go::TestDocumentedProtocolMatches` で、コマンドの定義の `mergepr` の呼び出しの `--protocol` と、`mergepr_guide.md` の `--protocol` が `mergepr.Protocol` に等しいことを確かめる（`cmd/chief/docs_test.go:205`・`cmd/fixprguard/docs_test.go:351` と同じ形）。`mergepr_guide.md` には、少なくとも 1 つあることを求める。
- [ ] `cmd/mergepr/docs_test.go::TestProtocolToolsAgree` で、設計書 1.2 の 3 つの集合（`WorktreeProtocolTools("../..")`、コマンドの定義に `ProtocolCallers` を `cmd/` の下のディレクトリの名前で当てた結果、`ProtocolTools()`）が等しいことを確かめる（AC-19）。作業ツリーで読めない値になった道具があれば失敗させる。
- [ ] `TestProtocolToolsAgree` が、`internal/mergepr` の定数を `var` に変えたときと、`ProtocolTools()` から `mergepr` を抜いたときに失敗することを確かめ、コミットのメッセージに書く。

#### ステップ 3-3: コマンドの定義

**変更するファイル:** `.claude/commands/mergepr.md`、`.claude/commands/runpr.md`

- [ ] 両方の、`mergepr` のすべての呼び出しに、位置引数より前に `--protocol 1` を書く（§1.4 の呼び出しの一覧、設計書 3.6）。
- [ ] `mergepr.md` の手順 1・4 に、番号の確かめで止まったときの報告を書く（設計書 3.6）。
- [ ] `runpr.md` の step 4 の項 2・4・5 に、止まったときの扱いを書く（設計書 3.6、1.2 の承認の commit の扱い、AC-14）。項 4 の今の「report and stop」は、番号の確かめの停止も含むように書き直す。

#### ステップ 3-4: 手引き

**変更するファイル:** `docs/dev/guides/mergepr_guide.md`、`docs/dev/guides/fixprguard_guide.md`、`docs/dev/conventions/package_reference.md`

- [ ] `mergepr_guide.md` に、設計書 3.6 の「`mergepr_guide.md`」に挙げた項目を書く。書いたコマンドの例は、`cmd/mergepr` の引数の解析を通る形にする（ステップ 3-2 のガードが `--protocol` の値を確かめる）。
- [ ] `fixprguard_guide.md` の §9 と §3 を、AC-18 の 2 点だけ直す。直した後、§7 の「PR-2 の `diff` の記録」の比べるパスについて、`requirements_process.md` の「`diff` verification」の固定のコマンドで記録する。
- [ ] `package_reference.md` の `internal/mergepr` の行に、番号の確かめを足す。

#### ステップ 3-5: CLAUDE.md・CI・ADR-0014

**変更するファイル:** `CLAUDE.md`、`.github/workflows/ci.yml`、`docs/dev/adr/0014-deterministic-checks-in-go.md`

- [ ] CLAUDE.md の「Development Notes」の下に「Protocol numbers」の節を立て、設計書 3.8 の 5 項目を英語で書く（AC-01〜AC-05・AC-17・AC-20）。止まったときの直し方は `mergepr_guide.md` を参照する。
- [ ] `ci.yml:215` の `doc-guard` の `go test` の対象と、`CLAUDE.md:707` の一覧の文に `./cmd/mergepr` を足す。
- [ ] ADR-0014 の、まだ反映していないことを述べる段落（`0014-deterministic-checks-in-go.md:27`）を消す（設計書 3.8）。
- [ ] PR の説明に、入れ替えの手順（§5 の R-03）を書く。

**完了の条件:** グリーンゲートと `make deadcode` が通る。`go test -tags test ./cmd/mergepr` が `doc-guard` の対象として通る。

### PR-2 作成ポイント: wire the protocol check into mergepr and document the rule

**対象ステップ**: 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `feat(0099): mergepr のマージでプロトコル番号を確かめ、番号の規則を CLAUDE.md に置く`

**レビュー観点**: `Prepare` が CI を待つ前に、`Merge` が `gh pr merge` の前に、`main` を取得し直してから確かめているか / `mergepr.md`・`runpr.md`・手引き・誤りの文のすべての `mergepr` の呼び出しに `--protocol` があるか / `runpr.md` が、マージの質問の前の停止と承認の commit の後の停止を書き分けているか / CLAUDE.md の「Protocol numbers」が AC-01〜AC-05・AC-20 を満たし、`fixprguard_guide.md` の差分が AC-18 の 2 点に限るか

**実装モデル要件**: sonnet+opus-review

**判定理由**: `mkplan.md` の Conditional checks のうち、繰り返し呼ぶ CLI のすべての呼び出しに同じ前提（`--protocol`）を当てる項と、複数の成果物が固定する 1 つの値（番号・文書・ガードの一覧）を同じコミットで変える項の 2 つに当たる。Sonnet で進めると、17 か所の呼び出しのどれかに `--protocol` を書き落とすか、確かめを CI を待った後に置きやすい。

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
| PR-2 | 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | 組み込み・`--protocol`・コマンドの定義・手引き・CLAUDE.md・ガード・CI・ADR-0014 の段落の削除 | sonnet+opus-review | auto |

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト（実際の git）.** 確かめの本体は、一時ディレクトリの git のリポジトリで履歴を組んで確かめる（ステップ 1-2・1-3、H-05）。リポジトリは利用者の git の設定に左右されないように組む（§1.3 の決定 4）。CI の `actions/checkout` は shallow だが、テストは自分のリポジトリを組むので影響を受けない。
-   **ユニットテスト（差し替え）.** `Prepare`・`Merge` への組み込みは、確かめの本体の差し替えと `fakeRunner` の台本で確かめる（ステップ 2-2）。
-   **文書のガード.** `--protocol` の値と、AC-19 の 3 つの集合の一致を `cmd/mergepr/docs_test.go` で確かめ、CI の `doc-guard` でも走らせる（ステップ 3-2・3-5）。CLAUDE.md と手引きの文の意味は、PR のレビューで読む（ADR-0016）。
-   **結合テスト.** `gh` を相手にした結合テストは置かない（設計書 7.3）。PR-2 自身のマージで、入れ替えの手順の中で確かめる（§7 の「PR-2 の実行の記録」）。
-   **互換性.** 古い `mergepr` と新しい定義、新しい `mergepr` と古い定義の組み合わせは、どちらも止まる（設計書 3.5）。新しい側は `TestRunRequiresProtocol` で、古い側は PR-2 の入れ替えの手順で確かめる。
-   **壊して失敗することの確かめ.** 各テストは、確かめる対象を壊して失敗することを、コミットの前に確かめ、コミットのメッセージに書く（CLAUDE.md の「Testing Strategy」）。

## 5. リスク管理 (Risk Management)

| ID | リスク | 対策 |
|---|---|---|
| R-01 | git の pathspec や終了コードの読み違いで、道具を 1 つも見つけずにすべての PR を通す | 設計書 3.2 の規則 1（`origin/main` に道具がなければ読めない）。`TestReadRevisionProtocols` と `TestCheckProtocolUnreadable` |
| R-02 | `internal/mergepr` の製品のファイルやテストの補助に `const Protocol = …` の行を書き、確かめが自分自身を読み違える | 宣言の行を持つ Go のソースは、`_test.go` の中で組み立てる（ステップ 1-2）。PR-1 の完了の条件で、製品のファイルに宣言の行がないことを確かめる |
| R-03 | PR-2 のブランチから新しい `mergepr` を入れると、マージまで、ほかのすべての worktree の `/mergepr`・`/runpr` が止まる | 入れ替えの手順を PR-2 の説明に書く（ステップ 3-5）。人が、(1) どの worktree にも準備したままの `mergepr-*` と終わっていない `cleanup` がないことを確かめ、(2) PR-2 の `/runpr` が最初に `mergepr` を呼ぶ前に、PR-2 のブランチから `make install-mergepr` で入れ、(3) マージの直後に `main` から入れ直す。途中でほかの PR をマージする必要が生じたら、`main` から入れ直す（設計書 8 章） |
| R-04 | `mergepr` を一覧に足す前に文書に `--protocol 1` を書き、`fixprguard` のガードが失敗する | ステップ 3-1 と 3-3 を同じ PR に置き、グリーンゲートで確かめる（§1.4 の「文書のガード」） |
| R-05 | PR-1 のマージから PR-2 のマージまでの間、確かめの本体は呼ばれない | 振る舞いは今と同じで、新しい危険はない。PR-2 を続けて進める |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

テストのパスはリポジトリの最上位からの相対である。

| AC | 確かめ | 種類 | ステップ |
|---|---|---|---|
| AC-01 | CLAUDE.md の「Protocol numbers」を PR のレビューで読む。対象の道具の集合そのものは `cmd/mergepr/docs_test.go::TestProtocolToolsAgree` | manual、test | 3-2 / 3-5 |
| AC-02 | CLAUDE.md の「Protocol numbers」を PR のレビューで読む | manual | 3-5 |
| AC-03 | CLAUDE.md の「Protocol numbers」を PR のレビューで読む | manual | 3-5 |
| AC-04 | CLAUDE.md の「Protocol numbers」を PR のレビューで読む | manual | 3-5 |
| AC-05 | CLAUDE.md の「Protocol numbers」と `mergepr_guide.md` の 7 章を PR のレビューで読む | manual | 3-4 / 3-5 |
| AC-06 | `internal/mergepr/protocolcheck_test.go::TestCheckProtocolHistories`（同じ値・2 以上大きい・小さい） | test | 1-3 |
| AC-07 | `internal/mergepr/protocolcheck_test.go::TestCheckProtocolHistories`（`protocolMismatchError` の道具・値・コミット）。誤りの文の形は PR のレビューで読む | test、manual | 1-1 / 1-3 |
| AC-08 | `internal/mergepr/protocolcheck_test.go::TestCheckProtocolHistories`（2 本目のブランチが `main` を取り込んだ場合） | test | 1-3 |
| AC-09 | `internal/mergepr/protocolcheck_test.go::TestCheckProtocolHistories`（番号に触れない PR、`main` を取り込んだだけの PR） | test | 1-3 |
| AC-10 | `internal/mergepr/protocolcheck_test.go::TestCheckProtocolHistories`（新しい道具・`--protocol` を足す道具・消す道具・足し直す道具） | test | 1-3 |
| AC-11 | `internal/mergepr/protocolcheck_test.go::TestCheckProtocolHistories`（2 つの道具が食い違う場合に、両方を返す） | test | 1-3 |
| AC-12 | `internal/mergepr/protocolcheck_test.go::TestMismatchRemedy`（示す `make install-<道具>` が `Makefile` の対象にある）。手順の文は、誤りの文と `mergepr_guide.md` を PR のレビューで読む | test、manual | 1-1 / 3-4 |
| AC-13 | (1) `internal/mergepr/mergepr_test.go::TestProtocolFetchFailureStops`。(2)〜(7) `internal/mergepr/protocolcheck_test.go::TestCheckProtocolUnreadable`、`TestReadRevisionProtocols`（git の失敗）、`TestClassifyProtocolLines`（(4)・(5) の行） | test | 1-1 / 1-2 / 1-3 / 2-2 |
| AC-14 | `internal/mergepr/mergepr_test.go::TestPrepareStopsOnProtocolCheck`。`runpr.md` の step 4 の項 2 は PR のレビューで読む | test、manual | 2-2 / 3-3 |
| AC-15 | `internal/mergepr/protocolcheck_test.go::TestCheckProtocolHistories`（上げてから元に戻した PR）と `TestMismatchRemedy`（AC-15 の直し方を加えるかの値） | test | 1-1 / 1-3 |
| AC-16 | `internal/mergepr/mergepr_test.go::TestMergeStopsOnProtocolCheck`、`TestProtocolFetchFailureStops` | test | 2-2 |
| AC-16a | `cmd/mergepr/main_test.go::TestRunRequiresProtocol`、`TestShownCommandsParse`、`cmd/mergepr/docs_test.go::TestDocumentedProtocolMatches`。古い `mergepr` が `--protocol` を拒むことは、PR-2 の入れ替えの手順で確かめる（下の記録） | test、manual | 2-3 / 3-2 / 3-3 |
| AC-17 | CLAUDE.md の「Protocol numbers」と `fixprguard_guide.md` §9 の参照を PR のレビューで読む | manual | 3-4 / 3-5 |
| AC-18 | 記録した `diff` を読む（下の「Diff record (AC-18)」）。直した文は PR のレビューで読む | diff、manual | 3-4 |
| AC-19 | `cmd/mergepr/docs_test.go::TestProtocolToolsAgree` | test | 3-2 |
| AC-20 | CLAUDE.md の「Protocol numbers」を PR のレビューで読む | manual | 3-5 |

### PR-2 の `diff` の記録

Diff record (AC-18):

- Compared paths: `docs/dev/guides/fixprguard_guide.md`

### PR-2 の実行の記録

入れ替えの手順（§5 の R-03）の中で、次を記録する。

- [ ] 新しい `mergepr` を入れる前に、どの worktree にも準備したままの `mergepr-*` と終わっていない `cleanup` がないことを確かめた
- [ ] 入れ替える前の `mergepr` が、`--protocol` を渡す呼び出しを `flag provided but not defined: -protocol` で拒んだ（AC-16a）
- [ ] PR-2 の `/runpr` の `prepare` が、`mergepr` を `origin/main` にない道具として期待する値 `1` で比べて通し、要約を示した（設計書 7.3）
- [ ] マージの直後に、`main` から `make install-mergepr` で入れ直した

## 8. 成功の基準 (Success Criteria)

-   **機能.** 番号を変えた PR は、`origin/main` の値 + 1 でしか `mergepr` でマージされない。番号に触れない PR は、追加の操作なしにマージされる。
-   **品質.** グリーンゲートと `make deadcode` が通る。§7 のすべての `test` の行が、確かめる対象を壊すと失敗することを、実装の際に確かめている。
-   **セキュリティ.** N/A（設計書 5 章）。読めない値で止まることは、`TestClassifyProtocolLines` と `TestCheckProtocolUnreadable` が確かめる。
-   **文書.** CLAUDE.md の「Protocol numbers」、`mergepr_guide.md`、`fixprguard_guide.md` §3・§9、`package_reference.md`、`mergepr.md`、`runpr.md` が、PR-2 の実装と合っている。

## 9. 次のステップ (Next Steps)

-   PR-2 のマージの後、`main` から `make install-mergepr` で入れ直す（§5 の R-03）。
-   進行中のタスク（0092・0093・0095・0096）の、番号を決め打ちした文書は、それぞれのタスクが実装の PR の前に直す（要件書 2.3・4.4）。
-   「番号を変えた PR」の定義の見直し（設計書 3.2 の末尾の既知の限界）は、[#947](https://github.com/isseis/yt2column/issues/947) で扱う。
