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

-   **製品を変えるコミットを先に置く（AC-14）:** PR-1 では、製品を変えるコミット（ステップ 1-2〜1-5）を、テストだけを変えるコミット（ステップ 1-6、1-7）より前に置く。PR-2 と PR-3 は製品を変えない。`make coverage-compare` の `BASE` は、PR-1 では PR-1 で製品を最後に変えたコミット、PR-2 と PR-3 ではマージベースである。`BASE` は PR を作る前とマージの前に決め直す。
-   **テストだけを変えるコミットの後に製品を直すとき:** レビューの指摘などで、テストだけを変えるコミットの後に製品のコードを直す必要が出たら、AC-14 の順序が崩れる。その場合は、止めて人に尋ねる。対話的な rebase はしない。
-   **期待値を変えない（AC-16）:** テストの差分は、製品の呼び出しの引数と、テストの準備の変更に限る。期待値を変えざるを得ないテストが見つかったら、設計の決定の変更なので、止めて人に尋ねる。
-   **設計書と違うことが分かったら止まる:** 設計書 4 章の「実装の際に確かめること」で、並列にできないテストが見つかったときは、止めて人に尋ねる（[requirements_process.md](../../dev/conventions/requirements_process.md)「Editing an approved document」）。
-   **最適化は別のコミット（`CLAUDE.md`「Performance」）:** フィクスチャのひな形のコピー（ステップ 3-3）は、それだけのコミットにする。正しさの義務（設計書 1.3 節の「フィクスチャを速くする手段」）をコミットメッセージに書く。
-   **壊して失敗を確かめる:** 新しく足すテストは、壊し確認（確かめる対象を壊すと失敗することの確認）をして、そのことをコミットメッセージに書く（`CLAUDE.md`「Testing Strategy」）。`_checks/` のスクリプトの破壊の確かめは、一時スクリプトの記録（5.1 節）に書く。
-   **lint の抑制は行ごと:** `fixprguardtest` は `_test.go` でないので、gosec などが当たる。git の起動やファイルの読み書きで抑制が要るときは、今の `internal/fixprguard/test_helpers_git.go` と同じく、その行だけに `//nolint:<linter>` と、テストの準備に限った理由を書く。ファイルやパッケージの全体には書かない。
-   **長い実行:** 変更の前の木（1.3 節）の `internal/fixprguard` の `-race` の実行は 1 回で約 500 秒かかる（issue #1167 の計測）。`gotest.sh` の実行と `make coverage-compare` は、10 分以上の時間の上限を付けるか、バックグラウンドで実行する。

### 1.3. 計画の際に決めたこと（2026-11-26）

-   **PR の分け方:** 3 つに分ける。PR-1 は見込みの確かめ、製品の変更、直列に残す新しいテスト。PR-2 は作業ディレクトリの隔離とテストの環境。PR-3 は並列化、フィクスチャの軽量化、計測である（3.2 節）。
    -   理由: 信頼の境界を守るツールの製品の変更（PR-1）を、大きなテストの書き直しと分けてレビューできる。AC-12 の計測は、タスクを終える PR（PR-3）で行うと要件が定めている。
    -   採らなかった案: 4 つに分けて、並列化とフィクスチャを別の PR にする案。PR ごとの名前の比較と `coverage-compare` が 1 回増える割に、レビューのしやすさはあまり変わらない。2 つに分ける案は、製品の変更が大きなテストの書き直しと同じ PR に入る。
    -   2026-11-26 に人が選んだ。ADR: なし（このタスクの進め方で、ほかのタスクを縛らない）。
-   **AC-01 などの確かめ:** `_checks/testshape/main.go` に、`go/ast` で 2 つのパッケージのテストを読む Go のプログラムを置く（5.1 節）。
    -   理由: 要件 2.3 節は、`t.Parallel()` を義務づける恒久的な規則や linter を置かないと決めている。そのため、確かめはタスクの一時スクリプトになる。Go の関数の本体と `t.Run` に渡す関数は、shell では正しく読めないので、shell ではなく Go を使う。
    -   採らなかった案: golangci-lint の `paralleltest` を一度だけ動かす案。`t.Setenv` を呼ぶテストの扱いなど、振る舞いがまだ確かめられていない。
    -   2026-11-26 に人が選んだ。ADR: なし（このタスクの確かめ方である）。
-   **AC-02 に足す実行:** PR-3 で、`-race -count=3` に加えて `-race -count=1 -cpu 1,4` を実行して記録する（H-06）。
    -   理由: スケジュールが違う実行を足すと、順序に頼るテストや競合が見つかりやすい。費用は 2 つのパッケージを 2 回分実行する時間だけである。
    -   2026-11-26 に人が選んだ。ADR: なし。
-   **直列に残す新しいテストを置く PR:** `TestWorktreeIgnoresProcessDir` と `TestRunInProcessDir`（設計書 8.1 節）は、PR-1 の、製品を変えるコミットの後のテストだけを変えるコミットに置く（ステップ 1-7）。設計書 9 章は、この 2 つを「隔離と環境」の段に挙げているが、PR の区切りは実装計画に委ねている。
    -   理由: 製品の変更の後、既存のテストはすべて絶対パスの `WorktreeDir` を渡す。CLI が通る `CurrentDir()` の側を CLI の入口で確かめるテストがないまま、PR-1 をマージすることになる。利用者は `main` から fixprguard を入れ直すので、PR-2 のマージまでの間に誤りが届きうる。2 つのテストは直列で `t.Chdir`・`t.Setenv` を使い、`fixprguardtest` に頼らないので、PR-1 で書ける。AC-14 の順序は保たれる。
    -   作成者が決めた（計画のレビューの指摘への対策）。PR のレビューで確かめてもらう。ADR: なし。
-   **`construction_test.go` の変更を置く PR:** `WorktreeDir` を構築の規則のテストに加える変更（設計書 3.2 節）も、PR-1 のテストだけを変えるコミットに置く（ステップ 1-6）。設計書 9 章では「隔離と環境」の段にあるが、型を足す PR で、その型の構築の規則も確かめるためである。
    -   作成者が決めた。PR のレビューで確かめてもらう。ADR: なし。
-   **`WorktreeDir` の構築の守り:** `WorktreeDir` は `internal/fixprguard` に置く（設計書 3.2 節）。そのため、同じパッケージの製品のコードが作ったり書き換えたりできる。リーフパッケージに移す案は、設計書が非公開のメソッドを `Worktree` から呼ぶ形に決めているので採らない。代わりに、`construction_test.go` で次のように守る（ステップ 1-6）。
    -   フィールドには、パッケージのほかの名前と重ならない名前を付ける（`path` は `change.path` などと重なる）。
    -   `Reviewers` と同じく、フィールドへの参照を、許した関数とメソッド（`CurrentDir`、`NewWorktreeDir`、3 つの操作、ゼロ値の確かめ）に限る。
    -   `NewWorktree` のゼロ値の確かめは、空の合成リテラルとの比較ではなく、フィールドを読むメソッドで書く。`zeroLiteralAllowed` には加えない。
    -   作成者が決めた（計画のレビューの指摘への対策）。ADR: なし。
-   **ひな形を作る手順（`build`）の形:** `build` は、`*testing.T` を受け取らず、失敗を `error` で返す関数にする。git の起動とファイルの書き込みは、`build` に渡す関数（ディレクトリと引数を受け取り、`Env` の環境で git を起動して `error` を返すもの）を通す。`TestMain`（`BuildTemplate`）と同等性のテストの両方が、この同じ `build` を呼ぶ（設計書 3.4 節が実装計画に委ねた形）。
    -   前の手順は `t.Fatalf` で失敗を報告するので、この形にするには、`t.Setenv`・`t.Chdir` の削除と環境の受け渡しのほかに、失敗の伝え方も変わる。そのため、設計書 3.6 節の「前の手順との一致」の確かめでは、差分を読んで、git の引数・順序・書くファイルが変わらないことを確かめる。この読み合わせは設計書のとおり PR のレビューで行い、PR の説明に記録する。計画は、それに加えて、git の引数と順序、テストがリポジトリを受け取るまでの git の起動の数を `testshape fixture-git` で機械的に比べる（5.1 節）。書くファイルはレビューの読み合わせだけで確かめる。`fixture-git` が実際のコードを判定できないときは、AC-10 の確かめがなくなるので、検査を外さずに止めて人に尋ねる（確かめを変えることは決定の変更である）。
    -   作成者が決めた（計画のレビューの指摘への対策）。ADR: なし。
-   **補助が受け取る `HOME`:** PR-2 で、`newTestWorktree`・`newTestRun`・`realGit`・`realGitList` などの補助は、対象のディレクトリに加えて `HOME` のディレクトリを必須の引数として受け取る。ディレクトリから `HOME` を推し量らない。リポジトリを作ったテストはそのリポジトリの `home` を渡す（`cmd/fixprguard` の `testRepo` に `Home` を足す）。リポジトリのないテストは、補助が別の `t.TempDir()` で作る空のディレクトリを渡す（設計書 3.3 節）。
    -   作成者が決めた。ADR: なし。
-   **一時スクリプトの置き方:** `_checks/` の 2 つのスクリプトは、PR-1 の最初のコミットに置く。`_checks/testshape/main.go` には `//go:build test` を付け、`go run -tags test` で動かす。
    -   理由: ステップ 1-1 の見込みの確かめにスクリプトが要る。`coverage-compare` は、`test` の build constraint のない Go のファイルを製品のコードに数える（`internal/coveragecompare/product.go` の `isGoProduct`）。build constraint を付ければ、PR-2 と PR-3 でこのファイルを直しても AC-14 の比較は止まらない。
    -   作成者が決めた。ADR: なし。
-   **変更の前の木の作り方:** 計測（AC-12）、名前の比較（AC-15）、フィクスチャの git の比較（AC-10、AC-11）の前の木は、`git archive` で作業ツリーの外の一時ディレクトリに展開する（設計書 8.2 節が実装計画に委ねた選択）。
    -   理由: エージェントは `git worktree` の追加と削除を実行しない（[chief_guide.md](../../dev/guides/chief_guide.md) 6 章、`.claude/commands/_lib/harness-claude.md` の shell-run）。`git archive` は、呼び出し元のリポジトリの状態を変えない。計画のレビューで、コミット `52ca51eb` を展開した木で 2 つのパッケージのテストが通ることを確かめた（`-race` なし）。`-race` の実行は、ステップ 1-1 の `estimate` で確かめる。
    -   作成者が決めた（規則から一つに決まる）。PR のレビューで確かめてもらう。ADR: なし。
-   **計測と名前の比較の手順の形:** `_checks/gotest.sh` に、`jq` を使う shell の一時スクリプトとして置く（設計書 8.2 節の「手順の形式」）。
    -   理由: 実装計画に書いたコマンドは、計画を書く時点では実行できない（後の木がまだない）。スクリプトにすれば、計画には性質だけを書き、コマンドはスクリプトに置ける。`jq` は `make test` がすでに求めている（`CLAUDE.md`「Development Notes」）。
    -   作成者が決めた。PR のレビューで確かめてもらう。ADR: なし。
-   **AC-12 の「前」のコミット:** PR-1 を作るときのマージベースを使う。ステップ 1-8 でそのコミット ID を 5.2 節の PR-1 の欄と PR-1 の説明に書き、ステップ 3-3 の `fixture-git` とステップ 3-5 の `measure` はそれを読む。後で PR-1 に `main` を取り込んでも、記録した ID は変えない。
    -   理由: 前の木を 1 つに決めておけば、どの PR で計っても同じ木と比べられる。前の木と後の木の間には、このタスクのほかに `main` に入った変更も含まれる。これは AC-12 の定め方から避けられないので、ステップ 3-5 で、前と後の間に 2 つのパッケージを変えたこのタスク以外のコミットを PR-3 の説明に書き、計測の結果を読む人が判断できるようにする。
    -   作成者が決めた。PR のレビューで確かめてもらう。ADR: なし。
    -   作成者が決めた。ADR: なし。
-   **`fixprguardtest` の置き場所:** 設計書のとおり `internal/fixprguard/fixprguardtest` に置く。[test_organization.md](../../dev/conventions/test_organization.md) の `testutil/` の形ではないが、`testOnlyPackageDirs` に同じ形の前例（`internal/loopbacktest`、`internal/udstest`、`internal/llm/llmhttp/llmhttptest`）がある。
-   **設計書の行番号のずれ:** 設計書の行番号はコミット `98456465` のものである。その後、0122 の変更で `cmd/fixprguard/main.go`・`main_test.go` と `internal/fixprguard/review_state_test.go` の行番号がずれた（1.4 節）。本書は、コミット `52ca51eb` の行番号か、`grep` のパターンで場所を示す。記号の名前と構造は設計書のとおりで、設計の決定は変わらない。

### 1.4. 既存コード調査結果

コミット `52ca51eb`（`origin/main`）で確かめた。

-   **製品の関数（`internal/fixprguard`）:** 設計書 3.2 節の表の関数は、すべて同じ場所にある（`locateGitDirs` `gitstate.go:187`、`hooksDigest` `:311`、`hooksState` `:535`、`blobOf` `worktree.go:521`、`checkReviewInput` `:577`、`trackedChange`・`untrackedChange`・`examine`・`readChanged` `changes.go:289`・`:318`・`:339`・`:382`、`checkTopDirGitNames` `ignored.go:410`、`walkIgnoredDir` `:453`、`targetStateOf` `:482`、`targetEntry` `:516`、`linkTarget` `:554`、`readTarget` `:582`、`lstatFunc` `:256`、`NewWorktree` `worktree.go:66`）。
-   **システムコールの箇所:** `grep -nE 'os\.(Lstat|Stat|ReadFile|Open|OpenFile|Readlink|Remove|CreateTemp|Rename)|filepath\.(Abs|WalkDir|EvalSymlinks)'` は、製品のファイルで 26 か所に当たった。`openRegular`（`gitstate.go:230`、`:235` の `os.OpenFile`）を呼ぶのは、`readChanged`（`changes.go:383`）、`blobOf`（`worktree.go:538`）、`readTarget`（`ignored.go:583`）、`readSmallFile`（`gitstate.go:254`。`locateGitDirs` の `:196` の `readSmallFile(".git")` などから）、`fileSHA256`（`:415`。`hooksDigest` の下）、`OpenRegularFile`（`:251`）、`write.go:29` である。最後の 2 つは呼び出し元が名前を指定するテキストのファイル用である（設計書 3.2 節の末尾）。
-   **`NewWorktree` の呼び出し元:** `cmd/fixprguard/main.go:408`（`execute`）、`cmd/fixprguard/test_helpers.go:140`・`:150`・`:169`（`Baseline`・`Begin`・`JudgedSet`）、`internal/fixprguard/test_helpers_git.go:265`（`newTestWorktree`）の 5 か所である。`execute` はすべてのサブコマンドの前に `NewWorktree` を呼ぶので、リポジトリを使わないサブコマンド（`head`・`fence`・`ci-checks` など）を動かすテストも、`newTestRun` に `WorktreeDir` を渡す必要がある。
-   **CLI の `env`:** `type env struct`（`cmd/fixprguard/main.go:181`）、`osEnv`（`:202-215`）、`execute`（`:400`）、GH_REPO と GH_HOST を拒む確かめ（`:403`）。設計書の `:178-190`・`:378` から行がずれた。
-   **`t.Chdir`・`t.Setenv` を呼ぶ箇所:** 補助は `internal/fixprguard/test_helpers_git.go:136-140` と `cmd/fixprguard/test_helpers.go:61-67`。テストは、`internal/fixprguard` では設計書 4 章の行のとおりである。`cmd/fixprguard/main_test.go` では、`t.Chdir` が `:183`・`:386`・`:1207`・`:1365`・`:1785`、`t.Setenv` が `:259`（`TestGhTargetEnvRejected`）・`:281-282`（`TestGhRunsWithoutGitVariables`）・`:303-304`（`TestListRunnerRejectsStderr`）にある。`os.Chdir`・`os.Setenv`・`os.Unsetenv` を呼ぶ箇所はない。
-   **`os.Environ()` から環境を作る箇所:** `internal/fixprguard/test_helpers_git.go` に 4 か所、`internal/fixprguard/ignored_test.go:330`、`cmd/fixprguard/test_helpers.go` に 3 か所、`cmd/fixprguard/docs_test.go:756` 付近の `sh` の実行（文書の例を動かす。git ではない）に 1 か所ある。
-   **作業ディレクトリからの相対の名前を使うテスト:** `newTestRepo` の `t.Chdir` に頼って相対の名前で読み書きするテストは、自分で `t.Chdir` を呼ばないテストにもある。例は、`cmd/fixprguard/main_test.go:1543` の `os.ReadFile(beginRecordName)`（`TestBeginWritesRecord`）、`:1681` の `os.Symlink`（`TestIgnoredOutputOmitsNames`）、`cmd/fixprguard/docs_test.go:779` の `os.OpenFile(beginRecordName, ...)`、`internal/fixprguard/ignored_test.go` の `collect`・`stateOf`・`stateWithin` などの補助である。
-   **パッケージのディレクトリから読むテスト:** `cmd/fixprguard/docs_test.go:25` の `repoRoot = "../.."` と、それを使う `readDoc`（`:29`）・`:43`・`doctestutil.CommandDocs`（`:506`）がある。ほかに、`internal/fixprguard/review_state_test.go:926` の `testdata/review_state`（設計書の `:839` から行がずれた）、`internal/fixprguard/construction_test.go:153`・`:163` がある。
-   **テストの数:** 2 つのパッケージの `_test.go` には、`func Test` が 218、`t.Run(` が 262 ある（`grep -o 't\.Run(' | wc -l`）。`t.Parallel()` は 0 である。`TestMain` はどちらのパッケージにもない。計画のレビューで、すべての `t.Run` が関数のリテラルを受け取ることを `go/ast` で確かめた。
-   **フィクスチャ:** `internal/fixprguard` の `newTestRepo`（`test_helpers_git.go:127-169`）は git を 13 回、`cmd/fixprguard` の `newTestRepo`（`test_helpers.go:54-82`）は 12 回起動する。設計書 3.6 節の数と同じである。前の手順の git の起動は、`gitIn`・`Git` を通り、失敗を `t.Fatalf` で報告する（`test_helpers_git.go:178-187`、`cmd/fixprguard/test_helpers.go:97-111`）。
-   **自分で runner を作るテスト:** `internal/fixprguard/ignored_test.go:330`（`router.List` の差し替え）。`lstatFunc` の fake は `ignored_test.go:760-770` にある。
-   **一覧に加えるテスト:** `restrictedConstructors`（`internal/fixprguard/construction_test.go:17-22`）は、型の名前から 1 つの関数の名前への `map[string]string` である。フィールドへの参照の規則（`reviewersFieldName`・`reviewersFieldAccessors`、`:33-41`）は `Reviewers` だけにある。`findRestrictedConstructions`（`:52-127`）は、合成リテラル、`new`、`var` の宣言、別名の型を報告する。`testOnlyPackageDirs`（`internal/pipeline/pipeline_test.go:540`）と `TestPackageReferenceListsPackages`（`:630`）は、テスト専用のパッケージを足すと、同じコミットで一覧の項目と `package_reference.md` の行を求める。
-   **`Protocol`:** `internal/fixprguard/types.go:21` にある。
-   **`coverage-compare` の分類:** `//go:build test` のファイルは製品のコードに数えない（`internal/coveragecompare/product.go` の `isGoProduct`）。比べる範囲は、深さによらずすべての `*.go` である（`target_go.go:12`）。子の `go test` の `TMPDIR` と `GOTMPDIR` は、作業ディレクトリの下の `tmp` である（`childenv.go:42-43`、`target.go:40`）。このディレクトリは、展開した BASE と HEAD のリポジトリと並ぶ場所にあり、git のリポジトリの中ではない（`checkout.go:22-44`）。そのため、`IsolateWorkingDir` の確かめは `coverage-compare` の中でも通る。
-   **lint:** gosec などを `_test.go` だけで外している（`.golangci.yml:116-125`）。`//go:build test` の補助の git の起動とファイルの読み書きは、行ごとに `//nolint:gosec` を付けている（`internal/fixprguard/test_helpers_git.go:180`・`:194` など）。
-   **外の前提の確かめ:** `t.TempDir()` が `GOTMPDIR` の下に作ることは、Go 1.27.1 の `src/testing/testing.go:1613` で確かめた。`docs/tasks/*/_checks/` の Go のファイルが `go run` で動き、`./...` には入らないことは、この機械の Go 1.27.1 で確かめた（`go list ./...` は `_` で始まるディレクトリを無視する）。git 2.50.1 では、`cp -Rp` で写したリポジトリで `git status --porcelain` は空のまま、`git diff-files` は stat の違う項目を報告する（計画のレビューで確かめた）。
-   **再利用するもの:** `cmdrunner.GitEnv`・`cmdrunner.NewOS`・`cmdrunner.Options.Dir`（`internal/cmdrunner`）、`gitRouter`・`cmdrunnertestutil.Fake`、前の `newTestRepo` の手順（`build` にする）を使う。新しく書くのは、設計書 3.4 節の `fixprguardtest` と 3.2 節の `WorktreeDir` だけである。

## 2. 実装のステップ

### フェーズ 1: 見込みの確かめと製品の変更

#### ステップ 1-1: 一時スクリプトと見込みの確かめ

-   **ファイル:** `docs/tasks/0126_fixprguard_test_speedup/_checks/gotest.sh`、`docs/tasks/0126_fixprguard_test_speedup/_checks/testshape/main.go`
-   **作業:**
    -   [ ] `_checks/gotest.sh` を書く。5.1 節の記録の枠の 5 つのモードを持つ。
    -   [ ] `_checks/testshape/main.go`（`//go:build test`）を書く。5.1 節の記録の枠の 4 つの検査を持つ。直列に残すテストの一覧（設計書 4 章の 6 項目）と、それらが使う補助の名前はプログラムの中に持つ。
    -   [ ] 2 つのスクリプトをコミットし、この時点で成功しうる検査（`estimate`・`names`・`unchanged`・`race`）の破壊の確かめをして、記録を書く。ほかの検査の破壊の確かめは、その検査が初めて成功しうる木で行う（5.1 節の「破壊の確かめの時期」）。
    -   [ ] `gotest.sh estimate` を `origin/main` の木で実行し、設計書 8.2 節の見込みを出す。見込みが変更の前の実行時間の 50% を超えたら、止めて人に尋ねる。結果は 5.1 節の実行の記録に書く。
-   **完了の条件:** 2 つのスクリプトがコミットされ、破壊の記録と見込みの実行の記録がある。

#### ステップ 1-2: `WorktreeDir` を足す

-   **ファイル:** `internal/fixprguard/workdir.go`（追加）
-   **作業:**
    -   [ ] 設計書 3.2 節の `WorktreeDir`・`CurrentDir`・`NewWorktreeDir` と、名前を結ぶ操作・場所を解く操作・逆の操作の 3 つの非公開のメソッドを書く。フィールドの名前と、ゼロ値を確かめるメソッドは 1.3 節のとおりにする。
    -   [ ] `NewWorktreeDir` は、設計書 5 章のとおり `ErrInvalidInput` で拒む。メッセージにパスを含めない。
-   **完了の条件:** `go build ./internal/fixprguard` が通る。

#### ステップ 1-3: 製品の関数が `WorktreeDir` を受け取るようにする

-   **ファイル:** `internal/fixprguard/worktree.go`、`gitstate.go`、`ignored.go`、`changes.go`
-   **作業:**
    -   [ ] `Worktree` に `WorktreeDir` を持たせ、`NewWorktree(dir, gitRun, listRun, makeRun)` にする。ゼロ値の `dir` で panic する（設計書 3.2 節、5 章）。
    -   [ ] 設計書 3.2 節の「`WorktreeDir` を受け取る非公開の関数」の表の関数を、表のとおりに変える。
    -   [ ] 1.4 節の `grep` のパターンを製品のファイルでもう一度実行する。当たった 26 か所のそれぞれについて、呼び出し元をさかのぼって、作業ツリーの中の名前か git が返す場所が届くかを読む。`openRegular` は 1.4 節の呼び出し元のそれぞれを読む。届く箇所は、結ぶ操作か解く操作を通す。届かない箇所（`OpenRegularFile` と `write.go:29`）は変えない。読んだ結果は PR の説明に表で書く。
    -   [ ] `Worktree` のメソッドから上の関数に、`Worktree` が持つ `WorktreeDir` を渡す。
-   **完了の条件:** `go build ./internal/fixprguard` が通る（`cmd/fixprguard` はステップ 1-4 で合わせる）。

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
    -   [ ] これらを呼ぶ途中の補助（`internal/fixprguard` の `mustBaseline`・`mustInspect`・`mustInspectPaths`・`collect`・`prepared.finishBody` など、`cmd/fixprguard` の `testRun` を作る補助）にも、同じディレクトリの引数を足す。
    -   [ ] 呼び出し元のテストが渡すディレクトリは、次のとおりにする。
        -   リポジトリを作るテストは `r.Dir`。
        -   `t.Chdir` で別の場所に移っているテストは、その場所。
        -   リポジトリを使わないテスト（`head`・`fence`・`ci-checks`・`mentions`・`review-state`・`interaction-limits` のテストなど）は、新しい `t.TempDir()`。パッケージのディレクトリ（`os.Getwd()`）は渡さない。
    -   [ ] `ignored_test.go:330` の runner と、`ignored_test.go:760-770` の fake の比べる名前を、設計書 3.2 節のとおりに変える。fake の判定（`errReadIgnoredFile` を期待すること）は変えない。
    -   [ ] 期待値を変えていないことを、このコミットの差分を読んで確かめる（AC-16）。
-   **完了の条件:** `make test` と `make lint` が通る。2 つのパッケージのテストは直列のままである。

#### ステップ 1-6: `WorktreeDir` のテストと構築の規則（テストだけを変えるコミット）

-   **ファイル:** `internal/fixprguard/workdir_test.go`（追加）、`internal/fixprguard/construction_test.go`、`internal/fixprguard/worktree_test.go`
-   **作業:**
    -   [ ] 設計書 8.3 節の `WorktreeDir` の単体テストを書く（受け入れと拒否、3 つの操作の `CurrentDir()` と絶対パスの両方、`./` を足さないこと、末尾の `/` を保つこと、接頭辞で始まらない名前を拒むこと）。
    -   [ ] ゼロ値の `WorktreeDir` で `NewWorktree` が panic するテストを書く。
    -   [ ] `restrictedConstructors` の値を関数の名前の集合にし、`WorktreeDir` に `CurrentDir` と `NewWorktreeDir` を許す。今の 4 つの型の判定は変えない。
    -   [ ] `WorktreeDir` のフィールドへの参照の規則を、1.3 節のとおりに足す。
    -   [ ] `CurrentDir`・`NewWorktreeDir` の外で `WorktreeDir` を作るか書き換える形（合成リテラル、`new`、`var` の宣言、別名の型、フィールドへの代入、同じ形の構造体からの変換）を製品のファイルに一時的に入れ、それぞれで `TestRestrictedTypesBuiltByConstructors` が失敗することを確かめる。
    -   [ ] 新しいテストのそれぞれについて、確かめる対象を壊して失敗することを確かめ、コミットメッセージに書く。
-   **完了の条件:** `make test` と `make lint` が通る。

#### ステップ 1-7: 直列に残す新しいテスト（テストだけを変えるコミット）

-   **ファイル:** `internal/fixprguard/worktree_flow_test.go`、`cmd/fixprguard/main_test.go`
-   **作業:**
    -   [ ] `TestWorktreeIgnoresProcessDir` を書く（設計書 8.1 節の AC-06 の行）。
    -   [ ] `TestRunInProcessDir` と 6 つのサブテストを書く（設計書 8.1 節の AC-08 の行）。`run` のすべての呼び出しで、開発者の環境の `GH_REPO`・`GH_HOST` が届かないようにする（`execute` は、空の値でも設定されていれば拒む。`main.go:403`）。`t.Setenv` は空の値を設定するだけで、拒まれるので使わない。`osEnv()` が返す `env` の `lookupEnv` を、`GH_REPO`・`GH_HOST` を未設定と答える関数に差し替える。
    -   [ ] それぞれについて、確かめる対象を壊して失敗することを確かめ、コミットメッセージに書く。`TestRunInProcessDir` では、少なくとも、`osEnv` の `dir` を `CurrentDir()` 以外にしたときと、`GitEnv` で `GIT_DIR` を除かなくしたときに失敗することを確かめる。
-   **完了の条件:** `make test` と `make lint` が通る。

#### ステップ 1-8: PR-1 の確かめと記録

-   **作業:**
    -   [ ] `make coverage-compare BASE=<PR-1 で製品を最後に変えたコミット>` を実行する（AC-14）。`FAIL` のときは、設計書 8.2 節の「coverage-compare の揺れ」のとおり、きれいな作業ツリーで比べ直す。すべての回の出力を、最後の行まで PR の説明に書く。
    -   [ ] `gotest.sh names <マージベース>` と `gotest.sh unchanged <マージベース>` を実行し、5.2 節の PR-1 の欄に記録する（AC-15、AC-09）。新しく現れた組と、その理由を PR の説明に書く。
    -   [ ] 期待値を変えたテスト（ない見込み）と、準備の変更（`ignored_test.go` の fake）を、PR の説明に書く（AC-16）。
    -   [ ] PR-1 のマージベースのコミット ID を、AC-12 の「前」のコミットとして 5.2 節の PR-1 の欄と PR の説明に書く（1.3 節）。
-   **完了の条件:** 上の結果が PR の説明と 5.2 節にある。

### PR-1 作成ポイント: worktree directory API in fixprguard

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8

**推奨タイトル**: `refactor(0126): fixprguard が作業ツリーのディレクトリを受け取るようにする`

**レビュー観点**: CLI が渡す `CurrentDir()` で OS に渡すパスが変わらないこと（`TestRunInProcessDir`） / 26 か所のシステムコールのうち名前が届く箇所がすべて結ぶ操作を通ること / `WorktreeDir` の構築の規則が作る形と書き換える形のすべてを報告すること / 製品を変えるコミットのテストの差分が引数と準備に限られること

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
    -   [ ] 設計書 3.4 節の `Isolation`・`IsolateWorkingDir`・`Env` を書く。隔離の確かめと後の確かめは設計書 3.5 節、`Env` の値は 3.3 節のとおりである。後の確かめ（作業ディレクトリが空か）は、ディレクトリを受け取る非公開の関数に分け、単体で確かめられるようにする。`Template` はステップ 3-3 で足す。
    -   [ ] `testOnlyPackageDirs` に `internal/fixprguard/fixprguardtest` を加え、`package_reference.md` の「Test-Only Packages」の表に行を足す（同じコミットで。1.4 節）。
-   **完了の条件:** `go vet -tags test ./internal/fixprguard/fixprguardtest` と `go test -tags test ./internal/pipeline` が通る。

#### ステップ 2-2: `fixprguardtest` のテスト

-   **ファイル:** `internal/fixprguard/fixprguardtest/*_test.go`（追加）
-   **作業:**
    -   [ ] このパッケージの `TestMain` で `IsolateWorkingDir` を呼ぶ（設計書 3.1 節）。
    -   [ ] 設計書 8.3 節のテストを書く。
        -   `IsolateWorkingDir` が git のリポジトリの中の `TMPDIR` で失敗すること。子のプロセスで確かめる。
        -   後の確かめが空でないディレクトリで失敗すること。ステップ 2-1 の関数を `t.TempDir()` に対して呼んで確かめ、テストのバイナリの作業ディレクトリには書かない。
        -   `Env` の 3 つの値。
    -   [ ] 新しいテストのそれぞれについて、確かめる対象を壊して失敗することを確かめ、コミットメッセージに書く。
-   **完了の条件:** `go test -tags test -race ./internal/fixprguard/fixprguardtest` と `make lint` が通る。

#### ステップ 2-3: `TestMain` と、補助から `t.Chdir`・`t.Setenv` を外す

-   **ファイル:** `internal/fixprguard/testmain_test.go`、`cmd/fixprguard/testmain_test.go`（追加）、`internal/fixprguard/test_helpers_git.go`、`cmd/fixprguard/test_helpers.go`
-   **作業:**
    -   [ ] 2 つの `TestMain` を書く。`IsolateWorkingDir` を呼び、`Isolation` をパッケージの変数に書いてから `m.Run` を呼び、後の確かめをする。失敗したときは設計書 5 章のとおり 1 行の診断を出して終了コード `1` で終わる。`TestMain` がない状態で、`t.Chdir` を外したパッケージのテストを動かさない（漏れた操作がこのリポジトリに届くため）。
    -   [ ] 2 つの `newTestRepo` から `t.Setenv`・`t.Chdir` を外し、準備の git に `fixprguardtest.Env(home, dir)` の環境を渡す。
    -   [ ] 製品の runner を作る補助（ステップ 1-5 の補助）が、`HOME` の引数を受け取り（1.3 節）、`Env` の環境と `Dir` を渡すようにする。`ConfigList` と `ignored_test.go:330` の runner も同じにする（設計書 3.3 節）。1.4 節の `os.Environ()` の箇所のうち、`docs_test.go` の `sh` の実行と直列に残すテストを除いて、すべてを `Env` に置き換える。
-   **完了の条件:** `go vet -tags test ./internal/fixprguard/... ./cmd/fixprguard` が通る（テストはステップ 2-4 の後に通る）。

#### ステップ 2-4: 作業ディレクトリに頼っていたテストを書き直す

ステップ 2-3 と 2-4 で、テストだけを変える 1 つのコミットにする。

-   **ファイル:** 2 つのパッケージの `*_test.go`
-   **作業:**
    -   [ ] 1.4 節の `t.Chdir` を呼ぶテスト（設計書 4 章の「並列にできると判断したテスト」）から `t.Chdir` を外し、同じディレクトリを補助に渡す。
    -   [ ] 2 つのパッケージのすべてのテストのファイルで、相対の名前を渡すファイルの操作（`os.` と `filepath.` の呼び出し、`writeFile`・`mkdir`・`symlink`・`stateOf`・`stateWithin`・`walkedEntries`・`collect` などの補助）を `grep` で探し、結んだパスにする（設計書 3.5 節）。`newTestRepo` の `t.Chdir` に頼っていたテストも対象である（1.4 節の `main_test.go:1543`・`:1681`、`docs_test.go:779` など）。
    -   [ ] ステップ 1-7 の `TestWorktreeIgnoresProcessDir` と `TestRunInProcessDir` を、自分で作業ディレクトリと `HOME` を設定し、補助に `HOME` を明示して渡す形にする。
    -   [ ] 1.4 節のパッケージのディレクトリから読むテストを、`Isolation.PackageDir` を起点に読むようにする。
    -   [ ] 期待値を変えていないことを、このコミットの差分を読んで確かめる（AC-16）。
-   **完了の条件:** `make test` と `make lint` が通る。`TestMain` の後の確かめが通る。

#### ステップ 2-5: AC-07 の確かめ

-   **作業:**
    -   [ ] 設計書 6.1 節の「確かめ（AC-07）」の 2 種類（ファイルの操作の漏れと git の漏れ）を、それぞれコミットせずに入れて、パッケージのテストが失敗することを確かめる。壊した変更は戻す。
    -   [ ] それぞれの前後で、このリポジトリの `git status --porcelain`、ローカルの ref（`refs/heads/*` と `refs/remotes/origin/*`）と reflog が変わらないことを確かめる。`origin` そのものの ref を `git ls-remote` で読むのはネットワークの操作なので、人の承認を得たときだけ行う。
    -   [ ] 入れた変更、失敗したテスト、上の確かめの結果を、PR の説明に書く。
-   **完了の条件:** 記録が PR の説明にある。

#### ステップ 2-6: PR-2 の確かめと記録

-   **作業:**
    -   [ ] `make coverage-compare BASE=<マージベース>` を実行する（AC-14。PR-2 は製品を変えない）。記録はステップ 1-8 と同じである。
    -   [ ] `gotest.sh names <マージベース>` と `gotest.sh unchanged <マージベース>` を実行し、5.2 節の PR-2 の欄に記録する（AC-15、AC-09）。新しく現れた組には、ステップ 2-2 のテストがある。
    -   [ ] `testshape process-state` と `testshape env-source` を実行して、5.1 節の実行の記録に書く（AC-04、AC-05）。
    -   [ ] 期待値を変えたテストと準備の変更を記録する（AC-16）。
-   **完了の条件:** 上の結果が PR の説明と 5 章にある。

### PR-2 作成ポイント: working directory isolation for fixprguard tests

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6

**推奨タイトル**: `test(0126): fixprguard のテストを作業ディレクトリと環境変数から切り離す`

**レビュー観点**: `TestMain` の隔離がこのリポジトリの中の一時ディレクトリで止まり、終了コード `128` だけを受け入れること / すべての runner と準備の git が `Env` の環境を受け取ること / 相対の名前で読み書きしていたテストが残らず結んだパスになったこと / AC-07 の確かめの記録

**実装モデル要件**: standard+high-review

**判定理由**: ステップ 2-3 はプロセス全体の作業ディレクトリを変える隔離という独立した高リスクの段階で、軽いモデルでは、隔離の確かめが受け入れる終了コードの誤り、確かめの git と漏れた git の環境の食い違い、後の確かめの抜けのために、テストがこのリポジトリに届くか、誤った理由で通るおそれがある

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

-   **ファイル:** `internal/fixprguard/fixprguardtest/template.go`（追加）、2 つの `testmain_test.go`、`internal/fixprguard/test_helpers_git.go`、`cmd/fixprguard/test_helpers.go`、`docs/dev/conventions/package_reference.md`
-   **作業:**
    -   [ ] 設計書 3.4 節の `Template`・`BuildTemplate`・`Copy` を書く。コピーと設定の書き直しは設計書 3.6 節のとおりである。
    -   [ ] 2 つのパッケージで、前の `newTestRepo` の手順を、1.3 節の形の `build` にする。`TestMain` は、隔離の後に `BuildTemplate` で 1 回だけひな形を作る。
    -   [ ] `newTestRepo` を、ひな形のコピーと `git update-index -q --refresh` の 1 回にする。
    -   [ ] `build` の差分を前の `newTestRepo` と読み合わせ、git の引数・順序・書くファイルが変わらないことを PR の説明に書く（設計書 3.6 節の「前の手順との一致」）。
    -   [ ] `testshape fixture-git <AC-12 の前のコミット>` を実行し、`build` の git の引数と順序が前の `newTestRepo` と同じであることと、テストがリポジトリを受け取るまでの git の起動の数を確かめる（AC-10、AC-11）。判定できないときは、止めて人に尋ねる（1.3 節）。結果は 5.1 節の実行の記録に書く。
    -   [ ] `package_reference.md` の `fixprguardtest` の行に、足した公開の名前を加える。
    -   [ ] 正しさの義務をコミットメッセージに書く（設計書 1.3 節）。このコミットはステップ 3-4 のテストと別にする。
-   **完了の条件:** `make test` と `make lint` が通る。

#### ステップ 3-4: ひな形のテスト

-   **ファイル:** `internal/fixprguard/fixprguardtest/template_test.go`、`internal/fixprguard/fixture_test.go`、`cmd/fixprguard/fixture_test.go`（追加）
-   **作業:**
    -   [ ] `Copy` が設定の行の形の不一致と特別なファイルで失敗するテストを書く（設計書 8.3 節）。
    -   [ ] 各パッケージに、並列の同等性のテストと分離のテストを書く（設計書 3.6 節）。同等性のテストには、設計書 3.6 節の値に加えて、index の stat の情報がコピー先のファイルと合っていることの確かめ（`GIT_OPTIONAL_LOCKS=0` の下で `git diff-files --quiet` が成功すること）を入れる。その確かめより前に、index を書き直しうる git（`git status` など）を動かさない。`git status --porcelain` は stat の違いを報告しないので、この確かめの代わりにならない（1.4 節）。
    -   [ ] それぞれについて、確かめる対象を壊して失敗することを確かめ、コミットメッセージに書く。同等性のテストでは、少なくとも、`Copy` が 2 つの設定の値を書き直さないときと、`update-index --refresh` を外したときに失敗することを確かめる。
-   **完了の条件:** `make test` と `make lint` が通る。`testshape parallel` が成功する。

#### ステップ 3-5: 計測と PR-3 の確かめ

-   **作業:**
    -   [ ] `gotest.sh race` を、ステップ 3-3 と 3-4 の後の木でもう一度実行し、実行したコミットとともに記録する（AC-02）。
    -   [ ] `gotest.sh measure <前のコミット>` を実行する（AC-12、AC-13）。前のコミットは、ステップ 1-8 で 5.2 節に書いたマージベースである。目標に届かないときは、PR を開く前に止め、結果と原因の見立てを人に示す。
    -   [ ] 前のコミットと今の木の間で、2 つのパッケージを変えたこのタスク以外のコミットを、PR の説明に書く（1.3 節）。
    -   [ ] 機械（OS と CPU のコア数、`GOMAXPROCS`）、Go の版、前後のコミット、パッケージごと・回ごとの時間を PR の説明に書く。3 回を超えて繰り返したときは、すべての回と判定に使った 3 回を書く。
    -   [ ] AC-10 の前後の数（前は 13 回と 12 回、後は 1 回。ひな形は別に 13 回と 12 回）と、`testshape fixture-git` の出力を PR の説明に書く（設計書 3.6 節）。
    -   [ ] `make coverage-compare BASE=<マージベース>`、`gotest.sh names <マージベース>`、`gotest.sh unchanged <マージベース>` を実行し、5.2 節の PR-3 の欄に記録する（AC-14、AC-15、AC-09）。
    -   [ ] `testshape parallel`、`testshape process-state`、`testshape env-source` を実行して記録する（AC-01、AC-04、AC-05）。
    -   [ ] 期待値を変えたテストと準備の変更を記録する（AC-16）。
-   **完了の条件:** 上の結果が PR の説明と 5 章にある。

### PR-3 作成ポイント: parallel fixprguard tests with template fixtures

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5

**推奨タイトル**: `test(0126): fixprguard のテストを並列にし、フィクスチャをひな形のコピーにする`

**レビュー観点**: `t.Parallel()` を呼ばないテストが設計書 4 章の一覧と一致すること / 同等性のテストが index の stat の情報まで確かめること / ひな形を作る手順が前の手順と同じ git を同じ順に起動すること / ひな形の後の `-race` の実行と AC-12 の計測の記録

**実装モデル要件**: standard+high-review

**判定理由**: ステップ 3-1 は 2 つのパッケージのテストを並列にする並行性の変更で、ステップ 3-3 は正しさの義務を持つ最適化である。軽いモデルでは、親の値を書き換えるサブテストの見落としや、コピーしたリポジトリと前の手順との差の見落としが、実行の順序によってだけ失敗するテストになりうる

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の条件 |
|---|---|---|
| M1（PR-1） | `WorktreeDir` と、それを受け取る製品の関数・CLI。AC-06・AC-08 の直列のテスト。見込みの確かめの結果 | `make test`・`make lint` が通り、AC-14 の比較が `PASS` |
| M2（PR-2） | `fixprguardtest` の隔離と環境、2 つの `TestMain`、作業ディレクトリと環境変数に頼らないテスト | 同上。AC-07 の確かめの記録 |
| M3（PR-3） | 並列のテスト、ひな形のコピー、同等性と分離のテスト | 同上。AC-02 と AC-12 の記録 |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 | 一時スクリプト、見込みの確かめ、`WorktreeDir` と製品の関数・CLI の変更、呼び出しの引数の変更、構築の規則、直列に残す新しいテスト | high | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 | `fixprguardtest` の隔離と環境、`TestMain`、`t.Chdir`・`t.Setenv` の除去、AC-07 の確かめ | standard+high-review | auto |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 | `t.Parallel()`、競合の確かめ、ひな形のコピーとそのテスト、計測 | standard+high-review | auto |

各 PR は、単独でグリーンゲートを通る。PR-1 の後のテストは直列で `t.Chdir` を使う。PR-2 の後のテストは直列のまま作業ディレクトリに頼らない。PR-3 は、PR-2 の `TestMain` と `fixprguardtest` に `Template` を足す。

## 4. テスト戦略

-   **単体テスト:** `WorktreeDir`（ステップ 1-6）、`fixprguardtest` の隔離・環境・ひな形（ステップ 2-2、3-4）。設計書 8.3 節の項目を確かめる。
-   **統合テスト:** 既存の 2 つのパッケージのテストは、本物の git でテストのリポジトリを読み書きする。期待値を変えずに、ディレクトリを補助に渡す形にする（ステップ 1-5、2-4）。
-   **後方互換（CLI の振る舞い）:** `TestRunInProcessDir`（ステップ 1-7）が、本物のプロセスの作業ディレクトリで CLI の入口を動かし、`CurrentDir()` の側を確かめる。既存の `TestGhRunsWithoutGitVariables`・`TestListRunnerRejectsStderr`・`TestGhTargetEnvRejected` は、期待値を変えずに直列のまま残す。
-   **フィクスチャの同等性:** 各パッケージの同等性のテストと分離のテスト（ステップ 3-4）。分離のテストは設計書 3.6 節のとおり各パッケージに置く。
-   **検証範囲の維持:** PR ごとの `make coverage-compare`（AC-14）と、テストの名前の比較（AC-15）。
-   **並列の安全:** `-race -count=3` と `-race -count=1 -cpu 1,4`（AC-02）。ひな形の後にもう一度実行する。
-   **カバレッジの目標:** 新しく足す関数（`WorktreeDir` の 3 つの操作、`fixprguardtest` の公開の関数）は、成功の経路と失敗の経路の両方をテストが通る。

## 5. 受け入れ基準の検証

| AC | 種類 | 確かめ方 | ステップ |
|---|---|---|---|
| AC-01 | test | `_checks/testshape/main.go` の `parallel` 検査（5.1 節）。PR-3 で実行する | 3-1、3-5 |
| AC-02 | test | `_checks/gotest.sh` の `race` モード（5.1 節） | 3-2、3-5 |
| AC-03 | test、manual | `testshape parallel` が、プログラムが持つ一覧（設計書 4 章の 6 項目）の外で `t.Parallel()` を呼ばないテストを報告しないこと。一覧と設計書 4 章が同じことは、PR-3 のレビューで読む | 3-1、3-5 |
| AC-04 | test | `testshape process-state` 検査（5.1 節）。PR-2 と PR-3 で実行する | 2-6、3-5 |
| AC-05 | test | `fixprguardtest` の `Env` のテスト（ステップ 2-2）。`testshape env-source` 検査（5.1 節）。同等性のテストの `home` の比較（ステップ 3-4） | 2-2、2-6、3-4、3-5 |
| AC-06 | test | 作業ディレクトリが別のリポジトリのとき: `internal/fixprguard/worktree_flow_test.go::TestWorktreeIgnoresProcessDir`（PR-1）。作業ディレクトリに `.git` がないとき: PR-2 からの、`TestMain` の空のディレクトリの下での 2 つのパッケージのテスト全体（設計書 8.1 節）。PR-2 の `make test` で確かめる | 1-7、2-3、2-4 |
| AC-07 | test、manual | `fixprguardtest` の `IsolateWorkingDir` と後の確かめのテスト（ステップ 2-2）。2 種類の漏れをわざと入れて失敗することの一度の確かめを、PR-2 の説明に記録する | 2-2、2-5 |
| AC-08 | test、manual | `cmd/fixprguard/main_test.go::TestRunInProcessDir`、`::TestGhRunsWithoutGitVariables`、`::TestListRunnerRejectsStderr`。マージベースにあるテストの期待値の差分が空であることは、AC-16 と同じく PR のレビューで差分を読んで確かめる。要件の AC-16 がこの確かめ方を定めており、期待値の行だけを機械的に取り出す手段はない | 1-7、各 PR |
| AC-09 | diff | `_checks/gotest.sh` の `unchanged` モード（5.1 節）。PR ごとに実行し、5.2 節に記録する | 1-8、2-6、3-5 |
| AC-10 | static | `testshape fixture-git` 検査（5.1 節）が、前の `newTestRepo` と後の `newTestRepo` の git の起動の数を出す。前後の数を PR-3 の説明に書く。判定できないときは止めて人に尋ねる | 3-3、3-5 |
| AC-11 | test、static、manual | 同等性のテストと分離のテスト（`internal/fixprguard/fixture_test.go`、`cmd/fixprguard/fixture_test.go`）、`fixprguardtest` の `Copy` のテスト。`build` が前の手順と同じ git を同じ順に起動し、同じファイルを書くことは、`testshape fixture-git` 検査と、PR-3 のレビューでの差分の読み合わせ（設計書 3.6 節） | 3-3、3-4 |
| AC-12 | diff | `_checks/gotest.sh` の `measure` モード（5.1 節）。5.2 節に記録する | 3-5 |
| AC-13 | test、manual | `measure` モードが機械・Go の版・前後のコミット・回ごとの時間を出す。その出力を PR-3 の説明に書く | 3-5 |
| AC-14 | test | `make coverage-compare BASE=<コミット>`。`BASE` は 1.2 節のとおり。PR ごとに実行し、すべての回の出力を PR の説明に書く | 1-8、2-6、3-5 |
| AC-15 | diff | `_checks/gotest.sh` の `names` モード（5.1 節）。PR ごとに実行し、5.2 節に記録する | 1-8、2-6、3-5 |
| AC-16 | manual | 期待値を変えたテストと準備の変更を PR の説明に記録し、PR のレビューで差分を読む（要件の AC-16 が定める確かめ方） | 1-5、2-4、各 PR |
| AC-17 | test | `make test`、`make lint` | 各 PR |

### 5.1. 一時スクリプトの記録

#### `_checks/gotest.sh`

-   **スクリプト:** `docs/tasks/0126_fixprguard_test_speedup/_checks/gotest.sh`（shell、`jq` を使う）。目的は、2 つのパッケージの `go test` の実行と、その `-json` の出力の比較、変えないファイルの比較である。確かめる AC は AC-02、AC-09、AC-12、AC-13、AC-15 である。前の木は `git archive` で作業ツリーの外の一時ディレクトリに展開し、終わりに消す（1.3 節）。`estimate`・`names`・`measure` の `go test` には `-timeout 0` を渡す（前の木の `-race` の実行が既定の 10 分に近いため。計る時間は変わらない）。`race` は AC-02 のコマンドのとおりに動かし、`-timeout` を渡さない。どの実行でも、パッケージの `pass` の行がないとき、`fail` の行があるとき、前の木の `pass` の組が空のときは、判定せずに失敗する。
-   **検査:**
    -   `estimate`: `origin/main` の木で 2 つのパッケージを 1 回ずつ実行する。並列にした後の時間の下限の見込みを、(直列に残さないトップレベルのテストのうち最も長いもの + 設計書 4 章の直列のテストのうち今あるものの合計) と (テストの時間の合計 ÷ `GOMAXPROCS`) の大きい方として出し、変更の前の実行時間の 50% と比べる。
    -   `race`: 今の木で、`-race -count=3` と `-race -count=1 -cpu 1,4` を実行する。
    -   `names <base>`: `<base>` の木と今の木で、テストの名前の組を比べる（設計書 8.2 節の「テストの名前」）。
    -   `measure <before>`: `<before>` の木と今の木で、2 つのパッケージを別々に 3 回ずつ実行し、中央値と比を出す（設計書 8.2 節の「計測」）。機械と Go の版も出す。
    -   `unchanged <base>`: `docs/dev/guides/fixprguard_guide.md` と、`internal/fixprguard/types.go` の `const Protocol` の行が、`<base>` と同じであることを確かめる。
    -   それぞれの検査は、少なくとも 1 つの破壊で失敗することを確かめる。
-   **実行の予定:**
    -   `estimate`: ステップ 1-1（実装の前）。`/runplan` の実行者が行う。
    -   `names`・`unchanged`: 各 PR を作る前（ステップ 1-8、2-6、3-5）。`/runplan` の実行者が行う。
    -   `race`: ステップ 3-2 と 3-5。`/runplan` の実行者が行う。
    -   `measure`: PR-3 を作る前（ステップ 3-5）。`/runplan` の実行者が行う。
-   **破壊の確かめの時期:** `estimate`・`names`・`unchanged`・`race` はステップ 1-1、`measure` はステップ 3-5（前の木と後の木が違うとき）。
-   **テスト:** コミットしない。
-   **破壊の記録:** （`/runplan` が書く）
-   **実行の記録:** （実行した者が書く。`names`・`unchanged`・`measure` は 5.2 節を指す）

#### `_checks/testshape/main.go`

-   **スクリプト:** `docs/tasks/0126_fixprguard_test_speedup/_checks/testshape/main.go`（Go、`//go:build test`。`go run -tags test` で動かす）。目的は、2 つのパッケージのテストとテストの補助の形を `go/ast` で確かめることである。Go の関数の本体と `t.Run` に渡す関数は shell では正しく読めないので、Go を使う（1.3 節）。確かめる AC は AC-01、AC-03、AC-04、AC-05、AC-10、AC-11 である。
-   **検査:**
    -   `parallel`: `internal/fixprguard` と `cmd/fixprguard` の `_test.go` のすべての `func TestXxx(t *testing.T)` と `t.Run` に渡す関数のうち、`t.Parallel()` を呼ばないものを、プログラムが持つ一覧（設計書 4 章）と比べる。一覧の外のものがあるとき、一覧のテストが見つからないとき、`t.Run` に関数のリテラルでない値が渡されて判定できないときは、失敗する。
    -   `process-state`: 2 つのパッケージの `_test.go`・`test_helpers*.go` と `internal/fixprguard/fixprguardtest` の Go のファイルで、`t.Chdir`・`t.Setenv`・`os.Chdir`・`os.Setenv`・`os.Unsetenv` の呼び出しが、一覧のテストとその補助（プログラムが名前で持つ）の中と `IsolateWorkingDir` の中にだけあることを確かめる。
    -   `env-source`: 同じファイルで、`os.Environ()` の呼び出しが、`fixprguardtest.Env` の中、一覧のテストとその補助の中、`cmd/fixprguard/docs_test.go` の `sh` の実行の中にだけあることを確かめる。
    -   `fixture-git <before>`: `<before>` の木の 2 つの `newTestRepo` と、今の木の `build` が起動する git の引数の並びを比べ、同じであることを確かめる。また、前の `newTestRepo` と後の `newTestRepo` が起動する git の数を出す。分岐やループのある本体、読めない呼び出しに当たったときは、判定せずに失敗する。
    -   それぞれの検査は、少なくとも 1 つの破壊で失敗することを確かめる。
-   **実行の予定:**
    -   `process-state`・`env-source`: PR-2 と PR-3 を作る前（ステップ 2-6、3-5）。`/runplan` の実行者が行う。
    -   `parallel`: ステップ 3-1、3-4、3-5。`/runplan` の実行者が行う。
    -   `fixture-git`: ステップ 3-3 と、PR-3 を作る前（ステップ 3-5）。`/runplan` の実行者が行う。
-   **破壊の確かめの時期:** 健全な木で成功するようになったときに行う。`process-state`・`env-source` はステップ 2-6、`parallel` はステップ 3-1、`fixture-git` はステップ 3-3。それより前の実行は、記録の対象にしない。
-   **テスト:** コミットしない。
-   **破壊の記録:** （`/runplan` が書く）
-   **実行の記録:** （実行した者が書く）

### 5.2. diff の記録

`diff` の確かめ（AC-09、AC-12、AC-15）の出力と、比べた 2 つのコミット ID を、PR ごとに書く。`/runplan` が書く。

-   **PR-1:** AC-09（未記入）、AC-15（未記入）、AC-12 の前のコミット（未記入）
-   **PR-2:** AC-09（未記入）、AC-15（未記入）
-   **PR-3:** AC-09（未記入）、AC-12（未記入）、AC-15（未記入）

## 6. 実装チェックリスト

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5）

## 7. リスク管理

| リスク | 影響 | 対策 |
|---|---|---|
| 見込みの確かめで、並列にしても 50% に届かない | 実装しても AC-12 を満たせない | ステップ 1-1 で止めて人に尋ねる（設計書 8.2 節） |
| 名前が届くシステムコールの見落とし | テストが作業ディレクトリに頼り、CLI では確かめが緩む | ステップ 1-3 の 26 か所の読み合わせ、`TestRunInProcessDir`（PR-1）、`TestMain` の隔離、AC-07 の確かめ |
| テストだけを変えるコミットの後に製品を直す必要が出る | AC-14 の順序が崩れる | 止めて人に尋ねる（1.2 節） |
| 実時間で待つテストが並列の負荷で失敗する | AC-02 か `make test` が揺れる | 判定の時間は変えず、止めて人に尋ねる（設計書 4 章） |
| `coverage-compare` の `signalGroup` の揺れ | AC-14 が `FAIL` になる | きれいな作業ツリーで比べ直し、すべての回を記録する（設計書 8.2 節） |
| コピーしたリポジトリが前の手順のものと違う | 既存のテストの前提が崩れる | 同等性のテスト（index の stat の情報を含む）と `testshape fixture-git`（ステップ 3-3、3-4） |
| `testshape fixture-git` が実際のコードを判定できない | AC-10 の確かめがなくなる | 検査を外さず、止めて人に尋ねる（1.3 節） |
| 並列の負荷で計測の値が揺れる | AC-12 の判定が回によって変わる | 3 回の中央値で判定し、繰り返したときはすべての回を記録する（AC-13） |

## 8. 成功の基準

-   **機能:** AC-01〜AC-17 がすべて 5 章の確かめ方で満たされる。
-   **品質:** 各 PR で `make test`・`make lint` が通り、`make coverage-compare` が `PASS` で終わる。
-   **セキュリティ:** CLI の振る舞い（OS に渡すパス、子のコマンドの環境、終了コード）が変わらないことを、`TestRunInProcessDir` と既存のテストが確かめる。テストがこのリポジトリに届かないことを、AC-07 の確かめで記録する。
-   **文書:** `package_reference.md` に `fixprguardtest` の行がある。`fixprguard_guide.md` は変えない（AC-09）。

## 9. 次のステップ

-   PR-3 のマージの後、issue #1167 を閉じる（`/runpr` が提案する）。
-   タスク 0127 が同じひな形の補助を要るときは、そのタスクで、`fixprguardtest` をより一般的な場所に移すかを決める（設計書 10 章）。
