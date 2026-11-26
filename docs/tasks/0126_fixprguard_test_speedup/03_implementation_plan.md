# 実装計画書：fixprguard のテストの並列化と git フィクスチャの軽量化

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-26 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装の概要

### 1.1. 目的

[01_requirements.md](01_requirements.md) の F-001〜F-005 を、[02_architecture.md](02_architecture.md) の設計どおりに実装する。`internal/fixprguard` と `cmd/fixprguard` のテストを `t.Parallel()` で動く形にし、テストのリポジトリをひな形のコピーで作る。CLI の振る舞いは変えない。設計の中身は設計書を参照し、本書は作業の順序・置き場所・確かめ方を定める。

### 1.2. 実装の原則

-   **製品を変えるコミットを先に置く（AC-14）:** PR-1 では、製品を変えるコミットを、テストだけを変えるコミットより前に置く。PR-2 と PR-3 は製品を変えない。各 PR の `make coverage-compare` の `BASE` は、PR-1 では製品を最後に変えたコミット、PR-2 と PR-3 ではマージベースである。
-   **期待値を変えない（AC-16）:** テストの差分は、製品の呼び出しの引数と、テストの準備の変更に限る。期待値を変えざるを得ないテストが見つかったら、設計の決定の変更なので、止めて人に尋ねる。
-   **設計書と違うことが分かったら止まる:** 設計書 4 章の「実装の際に確かめること」で、並列にできないテストが見つかったときは、止めて人に尋ねる（[requirements_process.md](../../dev/conventions/requirements_process.md)「Editing an approved document」）。
-   **最適化は別のコミット（`CLAUDE.md`「Performance」）:** フィクスチャのひな形のコピー（ステップ 3-3）は、それだけのコミットにする。正しさの義務（設計書 1.3 節の「フィクスチャを速くする手段」）をコミットメッセージに書く。
-   **壊して失敗を確かめる:** 新しく足すテストは、確かめる対象を壊すと失敗することを確かめ、そのことをコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。`_checks/` のスクリプトの破壊の確かめは、一時スクリプトの記録（5 章）に書く。

### 1.3. 計画の際に決めたこと（2026-11-26）

-   **PR の分け方:** 3 つに分ける。PR-1 は見込みの確かめと製品の変更、PR-2 は作業ディレクトリの隔離・テストの環境・直列に残す新しいテスト、PR-3 は並列化・フィクスチャの軽量化・計測である（3.2 節）。
    -   理由: 信頼の境界を守るツールの製品の変更（PR-1）を、大きなテストの書き直しと分けてレビューできる。AC-12 の計測は、タスクを終える PR（PR-3）で行うと要件が定めている。
    -   採らなかった案: 4 つに分けて、並列化とフィクスチャを別の PR にする案。PR ごとの名前の比較と `coverage-compare` が 1 回増える割に、レビューのしやすさはあまり変わらない。2 つに分ける案は、製品の変更が大きなテストの書き直しと同じ PR に入る。
    -   2026-11-26 に人が選んだ。ADR: なし（このタスクの進め方で、ほかのタスクを縛らない）。
-   **AC-01 と AC-04 の確かめ:** `_checks/testshape/main.go` に、`go/ast` で 2 つのパッケージのテストを読む Go のプログラムを置く（5 章）。
    -   理由: 要件 2.3 節は、`t.Parallel()` を義務づける恒久的な規則や linter を置かないと決めている。そのため、確かめはタスクの一時スクリプトになる。Go の関数の本体と `t.Run` に渡す関数は、shell では正しく読めないので、shell ではなく Go を使う。
    -   採らなかった案: golangci-lint の `paralleltest` を一度だけ動かす案。`t.Setenv` を呼ぶテストの扱いなど、振る舞いがまだ確かめられていない。
    -   2026-11-26 に人が選んだ。ADR: なし（このタスクの確かめ方である）。
-   **AC-02 に足す実行:** PR-3 で、`-race -count=3` に加えて `-race -count=1 -cpu 1,4` を 1 回実行して記録する（H-06）。
    -   理由: スケジュールが違う実行を足すと、順序に頼るテストや競合が見つかりやすい。費用は 2 つのパッケージを 2 回分実行する時間だけである。
    -   2026-11-26 に人が選んだ。ADR: なし。
-   **変更の前の木の作り方:** 計測（AC-12）と名前の比較（AC-15）の前の木は、`git archive` で作業ツリーの外の一時ディレクトリに展開する（設計書 8.2 節が実装計画に委ねた選択）。
    -   理由: エージェントは `git worktree` の追加と削除を実行しない（[chief_guide.md](../../dev/guides/chief_guide.md) 6 章、`.claude/commands/_lib/harness-claude.md` の shell-run）。`git archive` は、呼び出し元のリポジトリの状態を変えない。前の木のテストは作業ディレクトリをテストのリポジトリに変えるので、展開した場所が git のリポジトリでなくても動く。`cmd/fixprguard/docs_test.go` が読む文書も、展開した木に含まれる。
    -   作成者が決めた（規則から一つに決まる）。PR のレビューで確かめてもらう。ADR: なし。
-   **計測と名前の比較の手順の形:** `_checks/gotest.sh` に、`jq` を使う shell の一時スクリプトとして置く（設計書 8.2 節の「手順の形式」）。
    -   理由: 実装計画に書いたコマンドは、計画を書く時点では実行できない（後の木がまだない）。スクリプトにすれば、計画には性質だけを書き、コマンドはスクリプトに置ける。`jq` は `make test` がすでに求めている（`CLAUDE.md`「Development Notes」）。
    -   作成者が決めた。PR のレビューで確かめてもらう。ADR: なし。
-   **一時スクリプトを置くコミット:** `_checks/` の 2 つのスクリプトは、PR-1 の最初のコミットに置く。製品を変えるコミットより前である。
    -   理由: ステップ 1-1 の見込みの確かめにスクリプトが要る。また、`_checks/testshape/main.go` は Go のファイルなので、`coverage-compare` の比較の範囲（`BASE` から先頭まで）に入れないためである。
    -   作成者が決めた。ADR: なし。
-   **`construction_test.go` の変更を置く PR:** `WorktreeDir` を構築の規則のテストに加える変更（設計書 3.2 節）は、PR-1 の、製品を変えるコミットの後のテストだけを変えるコミットに置く。設計書 9 章は、この変更を「隔離と環境」の段に挙げているが、PR の区切りは実装計画に委ねている。型を足す PR で、その型の構築の規則も確かめるためである。AC-14 の「製品を変えるコミットが先」は保たれる。
    -   作成者が決めた。PR のレビューで確かめてもらう。ADR: なし。
-   **設計書の行番号のずれ:** 設計書の行番号はコミット `98456465` のものである。その後、0122 の変更で `cmd/fixprguard/main.go` と `cmd/fixprguard/main_test.go` の行番号がずれた（1.4 節）。本書は、コミット `52ca51eb` の行番号か、`grep` のパターンで場所を示す。記号の名前と構造は設計書のとおりで、設計の決定は変わらない。

### 1.4. 既存コード調査結果

コミット `52ca51eb`（`origin/main`）で確かめた。

-   **製品の関数（`internal/fixprguard`）:** 設計書 3.2 節の表の関数は、すべて同じ場所にある（`locateGitDirs` `gitstate.go:187`、`hooksDigest` `:311`、`hooksState` `:535`、`blobOf` `worktree.go:521`、`checkReviewInput` `:577`、`trackedChange`・`untrackedChange`・`examine`・`readChanged` `changes.go:289`・`:318`・`:339`・`:382`、`checkTopDirGitNames` `ignored.go:410`、`walkIgnoredDir` `:453`、`targetStateOf` `:482`、`targetEntry` `:516`、`linkTarget` `:554`、`readTarget` `:582`、`lstatFunc` `:256`、`NewWorktree` `worktree.go:66`）。`grep -nE 'os\.(Lstat|Stat|ReadFile|Open|OpenFile|Readlink|Remove|CreateTemp|Rename)|filepath\.(Abs|WalkDir|EvalSymlinks)'` は、製品のファイルで 26 か所に当たった。`openRegular`（`gitstate.go:230`）は、`readChanged`・`blobOf`・`readTarget`・`hooksDigest` の下の読み出しと、`OpenRegularFile`（`:251`。呼び出し元が名前を指定するテキストのファイル用、設計書 3.2 節の末尾）から呼ばれる。
-   **`NewWorktree` の呼び出し元:** `cmd/fixprguard/main.go:408`（`execute`）、`cmd/fixprguard/test_helpers.go:140`・`:150`・`:169`（`Baseline`・`Begin`・`JudgedSet`）、`internal/fixprguard/test_helpers_git.go:265`（`newTestWorktree`）の 5 か所である。
-   **CLI の `env`:** `type env struct`（`cmd/fixprguard/main.go:181`）、`osEnv`（`:202-215`）、`execute`（`:400`）。設計書の `:178-190`・`:378` から行がずれた。
-   **`t.Chdir`・`t.Setenv` を呼ぶ箇所:** 補助は `internal/fixprguard/test_helpers_git.go:136-140` と `cmd/fixprguard/test_helpers.go:61-67`。テストは、`internal/fixprguard` では設計書 4 章の行のとおりである。`cmd/fixprguard/main_test.go` では、`t.Chdir` が `:183`・`:386`・`:1207`・`:1365`・`:1785`、`t.Setenv` が `:259`（`TestGhTargetEnvRejected`）・`:281-282`（`TestGhRunsWithoutGitVariables`）・`:303-304`（`TestListRunnerRejectsStderr`）にある。設計書の `:180`・`:381`・`:971`・`:1129`・`:1549`・`:254`・`:276-277`・`:298-299` から行がずれた。`os.Chdir`・`os.Setenv`・`os.Unsetenv` を呼ぶ箇所はない。
-   **パッケージのディレクトリから読むテスト:** `cmd/fixprguard/docs_test.go:25` の `repoRoot = "../.."` と、それを使う `readDoc`（`:29`）・`:43`・`doctestutil.CommandDocs`（`:506`）がある。ほかに、`internal/fixprguard/review_state_test.go:926` の `testdata/review_state`（設計書の `:839` から行がずれた）、`internal/fixprguard/construction_test.go:153`・`:163` がある。`internal/fixprguard/scope_test.go` の `testdata` は文字列の値で、ファイルは読まない。
-   **テストの数:** 2 つのパッケージの `_test.go` には、`func Test` が 218、`t.Run(` が 264 あり、`t.Parallel()` は 0 である。`TestMain` はどちらのパッケージにもない。
-   **フィクスチャ:** `internal/fixprguard` の `newTestRepo`（`test_helpers_git.go:127-169`）は git を 13 回、`cmd/fixprguard` の `newTestRepo`（`test_helpers.go:54-82`）は 12 回起動する。設計書 3.6 節の数と同じである。
-   **自分で runner を作るテスト:** `internal/fixprguard/ignored_test.go:330`（`router.List` の差し替え）。`lstatFunc` の fake は `ignored_test.go:760-770` にある。
-   **一覧に加えるテスト:** `restrictedConstructors`（`internal/fixprguard/construction_test.go:17-22`）は、型の名前から 1 つの関数の名前への `map[string]string` である。`testOnlyPackageDirs`（`internal/pipeline/pipeline_test.go:540`）と `TestPackageReferenceListsPackages`（`:630`）は、テスト専用のパッケージを足すと、同じコミットで一覧の項目と `package_reference.md` の行を求める。
-   **`coverage-compare` の分類:** `//go:build test` のファイルは製品のコードに数えない（`internal/coveragecompare/product.go` の `isGoProduct`）。`fixprguardtest` とテストの補助の変更は、テストだけの変更になる。`coverage-compare` は、子の `go test` の `TMPDIR` と `GOTMPDIR` を、作業ディレクトリの下の `tmp` にする（`internal/coveragecompare/childenv.go:42-43`、`target.go:40`）。このディレクトリは、展開した BASE と HEAD のリポジトリと並ぶ場所にあり、git のリポジトリの中ではない（`checkout.go:22-44`）。そのため、`IsolateWorkingDir` の確かめは `coverage-compare` の中でも通る。
-   **外の前提の確かめ:** `t.TempDir()` が `GOTMPDIR` の下に作ることは、Go 1.27.1 の `src/testing/testing.go:1613` で確かめた。`docs/tasks/*/_checks/` の Go のファイルが `go run ./<path>/main.go` で動き、`./...` には入らないことは、この機械の Go 1.27.1 で確かめた（`go list ./...` は `_` で始まるディレクトリを無視する）。git は 2.50.1 である。
-   **再利用するもの:** `cmdrunner.GitEnv`・`cmdrunner.NewOS`・`cmdrunner.Options.Dir`（`internal/cmdrunner`）、`gitRouter`・`cmdrunnertestutil.Fake`、今の `newTestRepo` の手順（ひな形を作る `build` にする）を、そのまま使う。新しく書くのは、設計書 3.4 節の `fixprguardtest` と 3.2 節の `WorktreeDir` だけである。
-   **型の構築の経路:** `WorktreeDir` は `internal/fixprguard` に置く（設計書 3.2 節）。そのため、同じパッケージの製品のコードが合成リテラルで作れる。葉のパッケージに移す案は、設計書が非公開のメソッドを `Worktree` から呼ぶ形に決めているので採らない。代わりに、`construction_test.go` の `findRestrictedConstructions` が見る形（合成リテラル、`new`、`var` の宣言、別名の型）に `WorktreeDir` を加え、`CurrentDir` と `NewWorktreeDir` の外で作る形のそれぞれについて、壊して失敗することを確かめる（ステップ 1-6）。

## 2. 実装のステップ

### フェーズ 1: 見込みの確かめと製品の変更

#### ステップ 1-1: 一時スクリプトと見込みの確かめ

-   **ファイル:** `docs/tasks/0126_fixprguard_test_speedup/_checks/gotest.sh`、`docs/tasks/0126_fixprguard_test_speedup/_checks/testshape/main.go`
-   **作業:**
    -   [ ] `_checks/gotest.sh` を書く。5 章の記録の枠の 5 つのモードを持つ。
    -   [ ] `_checks/testshape/main.go` を書く。5 章の記録の枠の 2 つの検査を持つ。直列に残すテストの一覧（設計書 4 章の 6 項目）はプログラムの中に持つ。
    -   [ ] 2 つのスクリプトをコミットし、5 章の記録の破壊の確かめをして、記録を書く。
    -   [ ] `gotest.sh estimate` を `origin/main` の木で実行し、設計書 8.2 節の見込みを出す。見込みが前の時間の 50% を超えたら、止めて人に尋ねる。結果は 5 章の実行の記録に書く。
-   **完了の条件:** 2 つのスクリプトがコミットされ、破壊の記録と見込みの実行の記録がある。

#### ステップ 1-2: `WorktreeDir` を足す

-   **ファイル:** `internal/fixprguard/workdir.go`（追加）
-   **作業:**
    -   [ ] 設計書 3.2 節の `WorktreeDir`・`CurrentDir`・`NewWorktreeDir` と、名前を結ぶ操作・場所を解く操作・逆の操作の 3 つの非公開のメソッドを書く。
    -   [ ] `NewWorktreeDir` は、設計書 5 章のとおり `ErrInvalidInput` で拒む。メッセージにパスを含めない。
-   **完了の条件:** `go build ./...` が通る。

#### ステップ 1-3: 製品の関数が `WorktreeDir` を受け取るようにする

-   **ファイル:** `internal/fixprguard/worktree.go`、`gitstate.go`、`ignored.go`、`changes.go`
-   **作業:**
    -   [ ] `Worktree` に `dir WorktreeDir` を持たせ、`NewWorktree(dir, gitRun, listRun, makeRun)` にする。ゼロ値の `dir` で panic する（設計書 3.2 節、5 章）。
    -   [ ] 設計書 3.2 節の「`WorktreeDir` を受け取る非公開の関数」の表の関数を、表のとおりに変える。
    -   [ ] 1.4 節の `grep` のパターンを製品のファイルでもう一度実行し、当たった 26 か所のそれぞれについて、作業ツリーの中の名前か git が返す場所が届くかを読む。届く箇所は、結ぶ操作か解く操作を通す。届かない箇所（`OpenRegularFile` など、呼び出し元が名前を指定するファイル）は変えない。読んだ結果は PR の説明に表で書く。
    -   [ ] `Worktree` のメソッドから上の関数に `w.dir` を渡す。
-   **完了の条件:** `go build ./...` が通る。

#### ステップ 1-4: CLI に `env.dir` を足す

-   **ファイル:** `cmd/fixprguard/main.go`
-   **作業:**
    -   [ ] `env` に `dir fixprguard.WorktreeDir` を足し、`osEnv` で `fixprguard.CurrentDir()` を設定する。runner の `Options` は変えない（設計書 3.3 節）。
    -   [ ] `execute` の `NewWorktree` の呼び出し（`main.go:408`）を `NewWorktree(e.dir, e.git, e.gitList, e.make)` にする。
-   **完了の条件:** `go build ./...` が通る。

#### ステップ 1-5: テストの呼び出しの引数と補助の準備を合わせる

ステップ 1-2〜1-5 で、製品を変える 1 つのコミットにする（AC-14）。

-   **ファイル:** `internal/fixprguard/test_helpers_git.go`、`cmd/fixprguard/test_helpers.go`、2 つのパッケージの `*_test.go`
-   **作業:**
    -   [ ] `newTestWorktree`（`internal/fixprguard`）と、`newTestRun`・`realGit`・`realGitList` と `Baseline`・`Begin`・`JudgedSet`（`cmd/fixprguard`）が、対象のディレクトリを必須の引数として受け取り、そこから `WorktreeDir` と runner の `Options.Dir` を作るようにする（設計書 3.3 節）。この時点では、補助はまだ `t.Chdir`・`t.Setenv` を呼び、`os.Environ()` から環境を作る（設計書 9 章の 2）。
    -   [ ] 呼び出し元のテストを、ふつうは `r.Dir` を、`t.Chdir` で別の場所に移っているテストはその場所を渡すように変える。
    -   [ ] `ignored_test.go:330` の runner と、`ignored_test.go:760-770` の fake の比べる名前を、設計書 3.2 節のとおりに変える。fake の判定（`errReadIgnoredFile` を期待すること）は変えない。
    -   [ ] 期待値を変えていないことを、このコミットの差分を読んで確かめる（AC-16）。
-   **完了の条件:** `make test` と `make lint` が通る。2 つのパッケージのテストは直列のままである。

#### ステップ 1-6: `WorktreeDir` のテストと構築の規則（テストだけを変えるコミット）

-   **ファイル:** `internal/fixprguard/workdir_test.go`（追加）、`internal/fixprguard/construction_test.go`、`internal/fixprguard/worktree_test.go`
-   **作業:**
    -   [ ] 設計書 8.3 節の `WorktreeDir` の単体テストを書く（受け入れと拒否、3 つの操作の `CurrentDir()` と絶対パスの両方、`./` を足さないこと、末尾の `/` を保つこと、接頭辞で始まらない名前を拒むこと）。
    -   [ ] ゼロ値の `WorktreeDir` で `NewWorktree` が panic するテストを書く。
    -   [ ] `restrictedConstructors` の値を関数の名前の集合にし、`WorktreeDir` に `CurrentDir` と `NewWorktreeDir` を許す。今の 4 つの型の判定は変えない。
    -   [ ] `CurrentDir`・`NewWorktreeDir` の外で `WorktreeDir` を作る形（合成リテラル、`new`、`var` の宣言、別名の型）を製品のファイルに一時的に入れ、それぞれで `TestRestrictedTypesBuiltByConstructors` が失敗することを確かめる。
    -   [ ] 新しいテストのそれぞれについて、確かめる対象を壊して失敗することを確かめ、コミットメッセージに書く。
-   **完了の条件:** `make test` と `make lint` が通る。

#### ステップ 1-7: PR-1 の確かめと記録

-   **作業:**
    -   [ ] `make coverage-compare BASE=<ステップ 1-5 のコミット>` を実行する（AC-14）。`FAIL` のときは、設計書 8.2 節の「coverage-compare の揺れ」のとおり、きれいな作業ツリーで比べ直す。すべての回の出力を、最後の行まで PR の説明に書く。
    -   [ ] `gotest.sh names <マージベース>` を実行する（AC-15）。結果と、新しく現れた組とその理由を PR の説明に書く。
    -   [ ] `gotest.sh unchanged <マージベース>` を実行する（AC-09）。
    -   [ ] 期待値を変えたテスト（ない見込み）と、準備の変更（`ignored_test.go` の fake）を、PR の説明に書く（AC-16）。
-   **完了の条件:** 上の結果が PR の説明にある。

### PR-1 作成ポイント: worktree directory API in fixprguard

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7

**推奨タイトル**: `refactor(0126): fixprguard が作業ツリーのディレクトリを受け取るようにする`

**レビュー観点**: CLI が渡す `CurrentDir()` で OS に渡すパスが変わらないこと / 26 か所のシステムコールのうち名前が届く箇所がすべて結ぶ操作を通ること / 製品を変えるコミットのテストの差分が引数と準備に限られること / 見込みの確かめの結果

**実装モデル要件**: high

**判定理由**: ステップ 1-3 は信頼の境界を守るツールのすべてのファイルの操作を変える security-gate の変更（panel mode の trigger）で、軽いモデルでは相対の名前が届く箇所の見落としや、名前と場所の取り違えで、確かめが緩むおそれがある

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 作業ディレクトリの隔離とテストの環境

PR-2 は製品を変えない。すべてのコミットがテストだけを変える。

#### ステップ 2-1: `fixprguardtest` の隔離と環境

-   **ファイル:** `internal/fixprguard/fixprguardtest/isolate.go`、`env.go`（追加、`//go:build test`）、`internal/pipeline/pipeline_test.go`、`docs/dev/conventions/package_reference.md`
-   **作業:**
    -   [ ] 設計書 3.4 節の `Isolation`・`IsolateWorkingDir`・`Env` を書く。隔離の確かめと後の確かめは設計書 3.5 節、`Env` の値は 3.3 節のとおりである。`Template` はステップ 3-3 で足す。
    -   [ ] `testOnlyPackageDirs` に `internal/fixprguard/fixprguardtest` を加え、`package_reference.md` の「Test-Only Packages」の表に行を足す（同じコミットで。1.4 節）。
-   **完了の条件:** `go test -tags test ./internal/pipeline` が通る。

#### ステップ 2-2: `fixprguardtest` のテスト

-   **ファイル:** `internal/fixprguard/fixprguardtest/*_test.go`（追加）
-   **作業:**
    -   [ ] このパッケージの `TestMain` で `IsolateWorkingDir` を呼ぶ（設計書 3.1 節）。
    -   [ ] 設計書 8.3 節のテストを書く。`IsolateWorkingDir` が git のリポジトリの中の `TMPDIR` で失敗すること（子のプロセスで確かめる）、後の確かめが空でない作業ディレクトリで失敗すること、`Env` の 3 つの値である。
    -   [ ] 新しいテストのそれぞれについて、確かめる対象を壊して失敗することを確かめ、コミットメッセージに書く。
-   **完了の条件:** `go test -tags test -race ./internal/fixprguard/fixprguardtest` が通る。

#### ステップ 2-3: `TestMain` と、補助から `t.Chdir`・`t.Setenv` を外す

-   **ファイル:** `internal/fixprguard/testmain_test.go`、`cmd/fixprguard/testmain_test.go`（追加）、`internal/fixprguard/test_helpers_git.go`、`cmd/fixprguard/test_helpers.go`
-   **作業:**
    -   [ ] 2 つの `TestMain` を書く。`IsolateWorkingDir` を呼び、`Isolation` をパッケージの変数に書いてから `m.Run` を呼び、後の確かめをする。失敗したときは設計書 5 章のとおり 1 行の診断を出して終了コード `1` で終わる。
    -   [ ] 2 つの `newTestRepo` から `t.Setenv`・`t.Chdir` を外し、準備の git に `fixprguardtest.Env(home, dir)` の環境を渡す。
    -   [ ] 製品の runner を作る補助（ステップ 1-5 の補助）と `ConfigList` が、`Env` の環境と `Dir` を渡すようにする。`ignored_test.go:330` の runner も同じにする（設計書 3.3 節）。
    -   [ ] リポジトリのないテストの `HOME` は、補助が対象のディレクトリとは別の `t.TempDir()` で作る（設計書 3.3 節）。
-   **完了の条件:** `go build -tags test ./...` が通る（テストはステップ 2-4 の後に通る）。

#### ステップ 2-4: 作業ディレクトリに頼っていたテストを書き直す

ステップ 2-3 と 2-4 で、テストだけを変える 1 つのコミットにする。

-   **ファイル:** 2 つのパッケージの `*_test.go`
-   **作業:**
    -   [ ] 1.4 節の `t.Chdir` を呼ぶテスト（設計書 4 章の「並列にできると判断したテスト」）から `t.Chdir` を外し、同じディレクトリを補助に渡す。
    -   [ ] `t.Chdir` を外すテストごとに、相対の名前を渡すファイルの操作（`os.` と `filepath.` の呼び出し、`writeFile`・`mkdir`・`symlink`・`stateOf`・`stateWithin`・`walkedEntries` などの補助）を `grep` で探し、結んだパスにする（設計書 3.5 節）。
    -   [ ] 1.4 節のパッケージのディレクトリから読むテストを、`Isolation.PackageDir` を起点に読むようにする。
    -   [ ] 期待値を変えていないことを、このコミットの差分を読んで確かめる（AC-16）。
-   **完了の条件:** `make test` と `make lint` が通る。`TestMain` の後の確かめが通る。

#### ステップ 2-5: 直列に残す新しいテスト

-   **ファイル:** `internal/fixprguard/worktree_flow_test.go`、`cmd/fixprguard/main_test.go`
-   **作業:**
    -   [ ] `TestWorktreeIgnoresProcessDir` を書く（設計書 8.1 節の AC-06 の行）。
    -   [ ] `TestRunInProcessDir` と 6 つのサブテストを書く（設計書 8.1 節の AC-08 の行）。
    -   [ ] それぞれについて、確かめる対象を壊して失敗することを確かめ、コミットメッセージに書く。`TestRunInProcessDir` では、少なくとも、`osEnv` の `dir` を `CurrentDir()` 以外にしたときと、`GitEnv` で `GIT_DIR` を除かなくしたときに失敗することを確かめる。
-   **完了の条件:** `make test` が通る。

#### ステップ 2-6: AC-07 の確かめ

-   **作業:**
    -   [ ] 設計書 6.1 節の「確かめ（AC-07）」の 2 種類（ファイルの操作の漏れと git の漏れ）を、それぞれコミットせずに入れて、パッケージのテストが失敗し、このリポジトリとその `origin` に変化がないことを確かめる。壊した変更は戻す。
    -   [ ] 入れた変更、失敗したテスト、このリポジトリの `git status` と `origin` の ref に変化がないことを、PR の説明に書く。
-   **完了の条件:** 記録が PR の説明にある。

#### ステップ 2-7: PR-2 の確かめと記録

-   **作業:**
    -   [ ] `make coverage-compare BASE=<マージベース>` を実行する（AC-14。PR-2 は製品を変えない）。記録はステップ 1-7 と同じである。
    -   [ ] `gotest.sh names <マージベース>` と `gotest.sh unchanged <マージベース>` を実行して記録する（AC-15、AC-09）。新しく現れた組には、ステップ 2-2 と 2-5 のテストがある。
    -   [ ] `testshape process-state` を実行して記録する（AC-04）。
    -   [ ] 期待値を変えたテストと準備の変更を記録する（AC-16）。
-   **完了の条件:** 上の結果が PR の説明にある。

### PR-2 作成ポイント: working directory isolation for fixprguard tests

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7

**推奨タイトル**: `test(0126): fixprguard のテストを作業ディレクトリと環境変数から切り離す`

**レビュー観点**: `TestMain` の隔離がこのリポジトリの中の一時ディレクトリで止まること / 相対の名前で読み書きしていたテストが残らず結んだパスになったこと / `TestRunInProcessDir` が AC-08 の条件をすべて確かめること / AC-07 の確かめの記録

**実装モデル要件**: standard+high-review

**判定理由**: ステップ 2-3 と 2-6 はプロセス全体の作業ディレクトリを変える隔離と漏れの確かめという独立した高リスクの段階で、軽いモデルでは相対の名前で読み書きするテストの見落としが、このリポジトリへの書き込みや誤った理由で通るテストになりうる

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 並列化、フィクスチャの軽量化、計測

PR-3 は製品を変えない。

#### ステップ 3-1: `t.Parallel()` を足す

-   **ファイル:** 2 つのパッケージの `*_test.go`
-   **作業:**
    -   [ ] 設計書 4 章の一覧にないテストとサブテストに `t.Parallel()` を足す。
    -   [ ] 設計書 4 章の「実装の際に確かめること」の 4 つを、テストごとに確かめる。並列にできないテストが見つかったら、止めて人に尋ねる。
    -   [ ] `testshape parallel` を実行し、`t.Parallel()` を呼ばないテストが一覧のものだけであることを確かめる（AC-01、AC-03）。
-   **完了の条件:** `make test` が通る。

#### ステップ 3-2: 競合の確かめ

-   **作業:**
    -   [ ] `gotest.sh race` を実行する（AC-02。`-race -count=3` と `-race -count=1 -cpu 1,4`）。失敗したら原因を直す。実時間の待ちで失敗するときは、判定の時間を変えずに、止めて人に尋ねる（設計書 4 章）。
-   **完了の条件:** `gotest.sh race` が成功する。

#### ステップ 3-3: ひな形のコピー（最適化のコミット）

-   **ファイル:** `internal/fixprguard/fixprguardtest/template.go`（追加）、2 つの `testmain_test.go`、`internal/fixprguard/test_helpers_git.go`、`cmd/fixprguard/test_helpers.go`
-   **作業:**
    -   [ ] 設計書 3.4 節の `Template`・`BuildTemplate`・`Copy` を書く。コピーと設定の書き直しは設計書 3.6 節のとおりである。
    -   [ ] 2 つのパッケージで、今の `newTestRepo` の手順を `build` 関数として残し（`t.Setenv`・`t.Chdir` はすでにない）、`TestMain` が隔離の後に `BuildTemplate` で 1 回だけひな形を作る。
    -   [ ] `newTestRepo` を、ひな形のコピーと `git update-index -q --refresh` の 1 回にする。
    -   [ ] `build` の差分が `t.Setenv`・`t.Chdir` の削除と、git に環境とディレクトリを渡す変更だけで、git の引数と順序が変わらないことを、マージベースとの差分を読んで確かめ、結果を PR の説明に書く（設計書 3.6 節の「前の手順との一致」）。
    -   [ ] 正しさの義務をコミットメッセージに書く（設計書 1.3 節）。このコミットはステップ 3-4 のテストと別にする。
-   **完了の条件:** `make test` が通る。

#### ステップ 3-4: ひな形のテスト

-   **ファイル:** `internal/fixprguard/fixprguardtest/template_test.go`、`internal/fixprguard/fixture_test.go`、`cmd/fixprguard/fixture_test.go`（追加）
-   **作業:**
    -   [ ] `Copy` が設定の行の形の不一致と特別なファイルで失敗するテストを書く（設計書 8.3 節）。
    -   [ ] 各パッケージに、並列の同等性のテストと分離のテストを書く（設計書 3.6 節）。
    -   [ ] それぞれについて、確かめる対象を壊して失敗することを確かめ、コミットメッセージに書く。同等性のテストでは、少なくとも、`Copy` が 2 つの設定の値を書き直さないときと、`update-index --refresh` を外したときに失敗することを確かめる。
-   **完了の条件:** `make test` と `make lint` が通る。`testshape parallel` が成功する。

#### ステップ 3-5: 計測と PR-3 の確かめ

-   **作業:**
    -   [ ] `gotest.sh measure <前のコミット>` を実行する（AC-12、AC-13）。前のコミットは、PR-1 のスカッシュコミットの親（最初の実装 PR のマージベース）である。目標に届かないときは、PR を開く前に止め、結果と原因の見立てを人に示す。
    -   [ ] 機械（OS と CPU のコア数、`GOMAXPROCS`）、Go の版、前後のコミット、パッケージごと・回ごとの時間を PR の説明に書く。3 回を超えて繰り返したときは、すべての回と判定に使った 3 回を書く。
    -   [ ] AC-10 の前後の数（前は 13 回と 12 回、後は 1 回。ひな形は別に 13 回と 12 回）と、数えた根拠の行を PR の説明に書く（設計書 3.6 節）。
    -   [ ] `make coverage-compare BASE=<マージベース>`、`gotest.sh names <マージベース>`、`gotest.sh unchanged <マージベース>`、`testshape parallel`、`testshape process-state` を実行して記録する（AC-14、AC-15、AC-09、AC-01、AC-04）。
    -   [ ] 期待値を変えたテストと準備の変更を記録する（AC-16）。
-   **完了の条件:** 上の結果が PR の説明にある。

### PR-3 作成ポイント: parallel fixprguard tests with template fixtures

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `test(0126): fixprguard のテストを並列にし、フィクスチャをひな形のコピーにする`

**レビュー観点**: `t.Parallel()` を呼ばないテストが設計書 4 章の一覧と一致すること / 同等性のテストが比べる値が設計書 3.6 節を満たすこと / ひな形を作る手順が今の手順と同じ git を同じ順に起動すること / AC-12 の計測の記録と判定

**実装モデル要件**: standard+high-review

**判定理由**: ステップ 3-1 は 2 つのパッケージのテストを並列にする並行性の変更で、ステップ 3-3 は正しさの義務を持つ最適化である。軽いモデルでは、親の値を書き換えるサブテストの見落としや、コピーしたリポジトリと今の手順との差の見落としが、実行の順序によってだけ失敗するテストになりうる

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の条件 |
|---|---|---|
| M1（PR-1） | `WorktreeDir` と、それを受け取る製品の関数・CLI。見込みの確かめの結果 | `make test`・`make lint` が通り、AC-14 の比較が `PASS` |
| M2（PR-2） | `fixprguardtest` の隔離と環境、2 つの `TestMain`、作業ディレクトリと環境変数に頼らないテスト、AC-06・AC-08 の直列のテスト | 同上。AC-07 の確かめの記録 |
| M3（PR-3） | 並列のテスト、ひな形のコピー、同等性と分離のテスト | 同上。AC-02 と AC-12 の記録 |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 | 一時スクリプト、見込みの確かめ、`WorktreeDir` と製品の関数・CLI の変更、呼び出しの引数の変更、構築の規則 | high | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 | `fixprguardtest` の隔離と環境、`TestMain`、`t.Chdir`・`t.Setenv` の除去、直列の新しいテスト、AC-07 の確かめ | standard+high-review | auto |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | `t.Parallel()`、競合の確かめ、ひな形のコピーとそのテスト、計測 | standard+high-review | auto |

各 PR は、単独でグリーンゲートを通る。PR-1 の後のテストは直列で `t.Chdir` を使い、PR-2 の後のテストは直列のまま作業ディレクトリに頼らない。PR-3 は、PR-2 の `TestMain` と `fixprguardtest` に `Template` を足す。

## 4. テスト戦略

-   **単体テスト:** `WorktreeDir`（ステップ 1-6）、`fixprguardtest` の隔離・環境・ひな形（ステップ 2-2、3-4）。設計書 8.3 節の項目を確かめる。
-   **統合テスト:** 既存の 2 つのパッケージのテストは、本物の git でテストのリポジトリを読み書きする。期待値を変えずに、ディレクトリを補助に渡す形にする（ステップ 1-5、2-4）。
-   **後方互換（CLI の振る舞い）:** `TestRunInProcessDir`（ステップ 2-5）が、本物のプロセスの作業ディレクトリで CLI の入口を動かす。既存の `TestGhRunsWithoutGitVariables`・`TestListRunnerRejectsStderr`・`TestGhTargetEnvRejected` は、期待値を変えずに直列のまま残す。
-   **フィクスチャの同等性:** 各パッケージの同等性のテストと分離のテスト（ステップ 3-4）。
-   **検証範囲の維持:** PR ごとの `make coverage-compare`（AC-14）と、テストの名前の比較（AC-15）。
-   **並列の安全:** `-race -count=3` と `-race -count=1 -cpu 1,4`（AC-02）。
-   **カバレッジの目標:** 新しく足す関数（`WorktreeDir` の 3 つの操作、`fixprguardtest` の公開の関数）は、成功の経路と失敗の経路の両方をテストが通る。

## 5. 受け入れ基準の検証

| AC | 種類 | 確かめ方 | ステップ |
|---|---|---|---|
| AC-01 | test | `_checks/testshape/main.go` の `parallel` 検査（下の記録）。PR-3 で実行する | 1-1、3-1、3-5 |
| AC-02 | test | `_checks/gotest.sh` の `race` モード（下の記録） | 3-2 |
| AC-03 | test、manual | `testshape parallel` が、プログラムに持つ一覧（設計書 4 章の 6 項目）の外で `t.Parallel()` を呼ばないテストを報告しないこと。一覧と設計書 4 章が同じことは、PR-3 のレビューで読む | 3-1、3-5 |
| AC-04 | test | `_checks/testshape/main.go` の `process-state` 検査（下の記録）。PR-2 と PR-3 で実行する | 2-7、3-5 |
| AC-05 | test、manual | `internal/fixprguard/fixprguardtest` の `Env` のテスト（ステップ 2-2）。同等性のテストの `home` の比較（ステップ 3-4）。開発者の設定が届かないことは、PR-2 のレビューで、すべての runner と準備の git に `Env` を渡したことを読む | 2-2、2-3、3-4 |
| AC-06 | test | `internal/fixprguard/worktree_flow_test.go::TestWorktreeIgnoresProcessDir` | 2-5 |
| AC-07 | test、manual | `internal/fixprguard/fixprguardtest` の `IsolateWorkingDir` のテスト（リポジトリの中で失敗すること、後の確かめ）。2 種類の漏れをわざと入れて失敗することの一度の確かめを、PR-2 の説明に記録する | 2-2、2-6 |
| AC-08 | test、manual | `cmd/fixprguard/main_test.go::TestRunInProcessDir`、`::TestGhRunsWithoutGitVariables`、`::TestListRunnerRejectsStderr`。マージベースにあるテストの期待値の差分が空であることは、AC-16 と同じく PR のレビューで差分を読んで確かめる（要件の AC-16 が定める確かめ方。期待値の行を機械的に取り出す手段はない） | 2-5、各 PR |
| AC-09 | diff | `_checks/gotest.sh` の `unchanged` モード（下の記録）。PR ごとに実行する | 1-7、2-7、3-5 |
| AC-10 | manual | 設計書 3.6 節と H-04 の数え方（直線の手順の呼び出しを読む）で、前後の数と根拠の行を PR-3 の説明に記録する。設計書は、起動の回数を数える wrapper を使わないと決めているので、実行して数えるテストは置かない | 3-5 |
| AC-11 | test、manual | `internal/fixprguard/fixture_test.go` と `cmd/fixprguard/fixture_test.go` の同等性のテストと分離のテスト、`fixprguardtest` の `Copy` のテスト。`build` が今の手順と同じ git を同じ順に起動することは、PR-3 のレビューで差分を読む | 3-3、3-4 |
| AC-12 | test | `_checks/gotest.sh` の `measure` モード（下の記録） | 3-5 |
| AC-13 | test、manual | `measure` モードが機械・Go の版・前後のコミット・回ごとの時間を出す。その出力を PR-3 の説明に書く | 3-5 |
| AC-14 | test | `make coverage-compare BASE=<コミット>`。`BASE` は 1.2 節のとおり。PR ごとに実行し、すべての回の出力を PR の説明に書く | 1-7、2-7、3-5 |
| AC-15 | test | `_checks/gotest.sh` の `names` モード（下の記録）。PR ごとに実行する | 1-7、2-7、3-5 |
| AC-16 | manual | 期待値を変えたテストと準備の変更を PR の説明に記録し、PR のレビューで差分を読む（要件の AC-16 が定める確かめ方） | 1-5、2-4、各 PR |
| AC-17 | test | `make test`、`make lint` | 各 PR |

### 5.1. 一時スクリプトの記録

#### `_checks/gotest.sh`

-   **スクリプト:** `docs/tasks/0126_fixprguard_test_speedup/_checks/gotest.sh`（shell、`jq` を使う）。目的は、2 つのパッケージの `go test` の実行と、その `-json` の出力の比較である。確かめる AC は AC-02、AC-09、AC-12、AC-13、AC-15 である。前の木は `git archive` で作業ツリーの外の一時ディレクトリに展開し、終わりに消す（1.3 節）。
-   **検査:**
    -   `estimate`: `origin/main` の木で 2 つのパッケージを 1 回ずつ実行し、最も長いトップレベルのテストと、設計書 4 章の直列のテストのうち今あるものの合計から、並列にした後の時間の下限の見込みを出し、前の時間の 50% と比べる。
    -   `race`: 今の木で、`-race -count=3` と `-race -count=1 -cpu 1,4` を実行する。
    -   `names <base>`: `<base>` の木と今の木で、テストの名前の組を比べる（設計書 8.2 節の「テストの名前」）。
    -   `measure <before>`: `<before>` の木と今の木で、2 つのパッケージを別々に 3 回ずつ実行し、中央値と比を出す（設計書 8.2 節の「計測」）。機械と Go の版も出す。
    -   `unchanged <base>`: `docs/dev/guides/fixprguard_guide.md` と、`internal/fixprguard` の `const Protocol` の行が、`<base>` と同じであることを確かめる。
    -   それぞれの検査は、少なくとも 1 つの破壊で失敗することを確かめる。
-   **実行の予定:**
    -   `estimate`: ステップ 1-1（実装の前）。`/runplan` の実行者が行う。
    -   `names`・`unchanged`: 各 PR を作る前（ステップ 1-7、2-7、3-5）。`/runplan` の実行者が行う。
    -   `race`: ステップ 3-2 と、PR-3 を作る前。`/runplan` の実行者が行う。
    -   `measure`: PR-3 を作る前（ステップ 3-5）。`/runplan` の実行者が行う。
-   **テスト:** コミットしない。
-   **破壊の記録:** （`/runplan` が書く）
-   **実行の記録:** （実行した者が書く）

#### `_checks/testshape/main.go`

-   **スクリプト:** `docs/tasks/0126_fixprguard_test_speedup/_checks/testshape/main.go`（Go。`go run` で動かす）。目的は、2 つのパッケージのテストの形を `go/ast` で確かめることである。Go の関数の本体と `t.Run` に渡す関数は shell では正しく読めないので、Go を使う（1.3 節）。確かめる AC は AC-01、AC-03、AC-04 である。
-   **検査:**
    -   `parallel`: `internal/fixprguard` と `cmd/fixprguard` の `_test.go` のすべての `func TestXxx(t *testing.T)` と `t.Run` に渡す関数のうち、`t.Parallel()` を呼ばないものを、プログラムが持つ一覧（設計書 4 章）と比べる。一覧の外のものがあるとき、一覧のテストが見つからないとき、`t.Run` に関数のリテラルでない値が渡されて判定できないときは、失敗する。
    -   `process-state`: 2 つのパッケージの `_test.go`・`test_helpers*.go` と `internal/fixprguard/fixprguardtest` の Go のファイルで、`t.Chdir`・`t.Setenv`・`os.Chdir`・`os.Setenv`・`os.Unsetenv` の呼び出しが、一覧のテストの中と `IsolateWorkingDir` の中にだけあることを確かめる。
    -   それぞれの検査は、少なくとも 1 つの破壊で失敗することを確かめる。
-   **実行の予定:**
    -   `process-state`: PR-2 と PR-3 を作る前（ステップ 2-7、3-5）。`/runplan` の実行者が行う。
    -   `parallel`: ステップ 3-1 と、PR-3 を作る前（ステップ 3-5）。`/runplan` の実行者が行う。
-   **テスト:** コミットしない。
-   **破壊の記録:** （`/runplan` が書く）
-   **実行の記録:** （実行した者が書く）

## 6. 実装チェックリスト

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5）

## 7. リスク管理

| リスク | 影響 | 対策 |
|---|---|---|
| 見込みの確かめで、並列にしても 50% に届かない | 実装しても AC-12 を満たせない | ステップ 1-1 で止めて人に尋ねる（設計書 8.2 節） |
| 名前が届くシステムコールの見落とし | テストが作業ディレクトリに頼り、CLI では確かめが緩む | ステップ 1-3 の 26 か所の読み合わせ、`TestMain` の隔離、AC-07 の確かめ、`TestRunInProcessDir` |
| 実時間で待つテストが並列の負荷で失敗する | AC-02 か `make test` が揺れる | 判定の時間は変えず、止めて人に尋ねる（設計書 4 章） |
| `coverage-compare` の `signalGroup` の揺れ | AC-14 が `FAIL` になる | きれいな作業ツリーで比べ直し、すべての回を記録する（設計書 8.2 節） |
| コピーしたリポジトリが今の手順のものと違う | 既存のテストの前提が崩れる | 同等性のテストと、`build` の差分の読み合わせ（ステップ 3-3、3-4） |
| 並列の負荷で計測の値が揺れる | AC-12 の判定が回によって変わる | 3 回の中央値で判定し、繰り返したときはすべての回を記録する（AC-13） |

## 8. 成功の基準

-   **機能:** AC-01〜AC-17 がすべて 5 章の確かめ方で満たされる。
-   **品質:** 各 PR で `make test`・`make lint` が通り、`make coverage-compare` が `PASS` で終わる。
-   **セキュリティ:** CLI の振る舞い（OS に渡すパス、子のコマンドの環境、終了コード）が変わらないことを、`TestRunInProcessDir` と既存のテストが確かめる。テストがこのリポジトリと `origin` に届かないことを、AC-07 の確かめで記録する。
-   **文書:** `package_reference.md` に `fixprguardtest` の行がある。`fixprguard_guide.md` は変えない（AC-09）。

## 9. 次のステップ

-   PR-3 のマージの後、issue #1167 を閉じる（`/runpr` が提案する）。
-   タスク 0127 が同じひな形の補助を要るときは、そのタスクで、`fixprguardtest` をより一般的な場所に移すかを決める（設計書 10 章）。
