# 実装計画書：internal/chief のテストを並列に実行して速くする

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-26 |
| Review date | 2026-11-26 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`internal/chief` の独立したテストとサブテストに `t.Parallel()` を入れ、パッケージの経過時間を変更前の 50% 以下にする（F-001）。テストの判定は変えない（F-002）。変更前と変更後の比較は、`_checks/compare.sh` で行う（設計書 3.5 節）。

### 1.2. 実装の原則

-   テストの判定と入力の値は変えない（設計書 1 章の設計原則）。変えてよいのは、`t.Parallel()` の呼び出し、親テストの共有の値をサブテストごとに作るよう移すこと、親の `defer` を `t.Cleanup` に移すこと、umask に結果が左右されるテストの冒頭で umask を決まった値にすることだけである。
-   製品のコードは変えない（AC-05）。`test_helpers.go` には、umask を決まった値にするヘルパーを 1 つ足すだけにする（設計書 3.1 節・3.2 節）。
-   `t.Setenv` を呼ぶテストと、umask に結果が左右されるテストは直列に残す（設計書 3.2 節）。直列に残すとは、そのテストも祖先も `t.Parallel()` を呼ばないことである。
-   Go のコメント、識別子、文字列リテラルは英語で書く。

### 1.3. 計画の際に決めたこと（2026-11-26）

-   変更前と変更後の比較は、`_checks/compare.sh` が両方の木で `go test` を続けて走らせて行う。変更前の木は、マージベースを `git archive` で一時ディレクトリに展開して作る。
    -   理由: 同じ機械で続けて測るので、機械の負荷の違いが比に入りにくい。実装の前に手で保存した出力と比べると、保存した時点と比べる時点が離れる。
    -   採らなかった案: 実装の前に `go test -json` の出力を手で保存し、後でスクリプトで比べる案。
    -   2026-11-26 に人が選んだ。
    -   ADR: なし。本タスクの中で閉じる。
-   フェーズは 1 つ、実装の PR は 1 本とする。
    -   理由: 変更の中心は `t.Parallel()` の機械的な追加で、レビューの観点は「共有の状態を持つテストを並列にしていないか」の 1 つに絞れる。issue の 6 つのファイルを先に分けても、50% の判定はパッケージ全体を変えた後にしかできない。
    -   採らなかった案: PR-1 で issue の 6 つのファイル、PR-2 で残りを並列にする案。
    -   2026-11-26 に人が選んだ。
    -   ADR: なし。本タスクの中で閉じる。
-   本計画書のステップは、書き方 A（`- [ ] **ステップ N-M**`）で書く。chief が確実に読める書き方だからである（タスク 0113 の要件書 1 章）。
    -   ADR: なし。本タスクの中で閉じる。

### 1.4. 既存コード調査結果

コミット `6c60f9f8`（`origin/main`）で確かめた。

-   `internal/chief` の `_test.go` に `t.Parallel()` の呼び出しはない（`grep`）。リポジトリ全体で `t.Parallel()` を呼ぶテストのファイルは 9 つあり、`cmd/yt2column`・`internal/mergepr`・`internal/coveragecompare` にある。
-   `t.Setenv` を呼ぶのは、次の箇所である（`grep`）。
    -   `test_helpers.go:136`（`RealCommands`）
    -   `autosend_test.go:1084`（`TestGitClean`）
    -   `network_test.go` の 4 つのテスト（`:20`・`:66`・`:141`・`:194`）
-   `RealCommands` を呼ぶテストは、次の 5 つのファイルにある（`grep`）。
    -   `create_test.go:1048`
    -   `removal_test.go:846`
    -   `workspace_test.go:248`・`:276`・`:324`
    -   `task_test.go:463`・`:480`・`:526`
    -   `taskspace_test.go:228`
    -   呼ぶのは、どれもトップレベルのテストである。`TestWorktreeAddDetached`・`TestWorktreeRemoveKeepsBranch`・`TestGitWorktreesOfARealRepository`・`TestGitIsAncestor`・`TestGitCleanCountsStagedAndNestedFiles`・`TestIssueIndexIgnoresGrepPatternType`・`TestIssueTasksReadsTheRealGitGrep`・`TestIssueTasksWithNoLine`・`TestTaskSpaceWorktreeMatchesCanonicalWorktree` の 9 つである。行番号は、実装の時点で変わっていることがある。
-   `privatefile.MkdirBelow` は、ディレクトリを作る間だけプロセスの umask を変える（`internal/privatefile/privatefile.go:399-401`・`:434-436`）。chief の製品のコードでは `records.go:132`・`:579`・`:1627` が呼ぶ。`OpenStore` を呼ぶテストのヘルパーは、`newStore`（`records_test.go:38`）、`newWorld` の中（`world_test.go:166`）、`newSendWorld` の中（`send_test.go:455`）にある。扱いは設計書 1 章と 3.2 節のとおりである。
-   広い権限と権限の確かめを `grep -n '0o644\|0o755\|0o666\|Perm()'` で探すと、当たるのは`records_test.go` の 4 行（`:285`・`:427`・`:945`・`:1107`）である。umask に結果が左右されるテストは、直列に残し、テストの冒頭で umask を `0o022` にし、終わりに元へ戻す（設計書 1 章・3.2 節）。どれが当たるかは、実装の際に確かめる。
-   テストの中で `os.Setenv`・`os.Chdir`・`t.Chdir` を呼ぶ箇所はない（`grep`。`TestMain` の前の `isolateTestPath` を除く。設計書 3.2 節）。
-   `newTestRepo`（`test_helpers.go:107-126`）は、テストごとの `t.TempDir()` の下に作る（設計書 3.4 節）。
-   最も長い `TestReadClaudeReport`（`transcript_test.go:23`）は、親テストで、上限（`transcript.go:40` の `sessionTranscriptLimit`、64 MiB）を 1 バイト超える入力を作って読む（`transcript_test.go:49-55`）。22.3 秒のうち、サブテストの合計は約 0.5 秒である（要件書 1 章）。サブテストは親の `config`（`transcript_test.go:25`）を共有し、ループの後に親が `config` に書いて判定する（`transcript_test.go:115-128`）。並列にする前に設計書 3.3 節を確かめる。
-   実際の時間で待つテストは、`TestTranscriptFIFORefused` の `time.After(5 * time.Second)`（`send_test.go:974`）である。そのほかに `deadline`・`Deadline` に当たるのは、コメントとテストの名前と `os.ErrDeadlineExceeded` だけである（`grep`）。
-   親の本体で `defer` を使うテストがある（`records_test.go:446`・`autosend_test.go:1289`・`create_test.go:625` など）。サブテストを並列にするときは、設計書 3.3 節の「親の本体の順序」を確かめる。
-   新しいテストのヘルパーは要らない（`docs/dev/conventions/test_organization.md`）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 比較のスクリプト、並列化、計測（設計書 3 章、AC-01〜AC-06）

- [x] **ステップ 1-1**: 比較のスクリプト
    - 追加するファイル: `docs/tasks/0128_chief_test_speedup/_checks/compare.sh`
    - 設計書 3.5 節のとおりに書く。判定は、時間の比（AC-01）、テストの名前の集合と `fail`・`skip` の有無（AC-03）、変わったファイルの一覧（AC-05）の 3 つである。判定できないときは終了コード 2 を返す
    - スクリプトをコミットしてから、テストのファイルに変更のない `HEAD` で実行し、AC-01 が通らず、AC-03・AC-05 が通ることを確かめる（設計書 8 章）
    - 各判定が少なくとも 1 つの壊し方で失敗することを確かめる。結果は 7.1 節の一時スクリプトの記録に書く
    - 完了の条件: スクリプトがコミットされ、7.1 節に壊し方の記録がある
- [x] **ステップ 1-2**: テストの並列化
    - 変えるファイル: `internal/chief/*_test.go`・`internal/chief/test_helpers.go`
    - ファイルごとに設計書 3.3 節を確かめ、独立したテストとサブテストに `t.Parallel()` を足す（設計書 3.2 節）。issue の 6 つのファイルから始め、残りのファイルに広げる
    - 設計書 3.2 節の直列に残すテスト（`t.Setenv`、umask、そのほかのプロセス全体の状態）は、そのテストも祖先も `t.Parallel()` を呼ばないままにする。umask に左右されるテストは、テストの冒頭で `test_helpers.go` に足すヘルパーを呼び、umask を `0o022` にして `t.Cleanup` で元に戻す。ヘルパーは `t.Setenv` も呼ぶ（設計書 1 章・3.2 節）。ヘルパーを呼んだテストも 7.3 節に書く
    - ヘルパーを足すコミットのメッセージに、正しさの義務（umask に結果が左右されるテストは直列に残し、ヘルパーを呼ぶこと）を書く（`CLAUDE.md`「Performance」、設計書 1 章）
    - 直列に残したテストとその理由（`t.Setenv`、umask、共有の値、親の本体の順序、時間に依存する判定のどれか）を、7.3 節に書く
    - 共有の値をサブテストの中に移したテストと、`defer` を `t.Cleanup` に移したテストを、変えた内容とともに 7.3 節に書く。PR のレビューで、判定と入力が変わっていないことを読む
    - ファイルの組ごとに `go test -tags test -race ./internal/chief` を通してからコミットする
    - 完了の条件: パッケージのテストが通り、7.3 節に記録がある
- [x] **ステップ 1-3**: 計測と記録
    - 計測の前に、同じ機械でほかのテストやビルドが走っていないかを人に尋ね、走っていないという答えを得てから計測する（要件書 AC-01）。`/runplan` のセッションからは、ほかの workspace の負荷を確かめられないためである。人の答えを記録に書く
    - `git fetch origin main` の後に、`compare.sh` を実行する（コマンドの時間制限を 10 分以上にして実行する）
    - 出力と 2 つのコミットを 7.0 節に書き、7.1 節の実行の記録からその場所を指す（`docs/dev/conventions/requirements_process.md`「Temporary script record」）
    - AC-01 が通らないときは、作業を止めて人に知らせる。フィクスチャの高速化を足すことは、設計書 1 章のとおり決定の変更になる
    - `go test -tags test -race -count=3 ./internal/chief` を実行し、終了コードと実行したコミットを 7.2 節に書く（AC-04）。この後に `internal/chief` のテストが変わったとき（レビューの直し、`origin/main` の取り込み）は、変えた人が実行し直して 7.2 節を書き直す
    - `make fmt`・`make test`・`make lint` を通す（AC-06）
    - PR の説明に、`compare.sh` の出力（2 つの経過時間、その比、2 つのコミット）を写す（AC-02）
    - 完了の条件: 7 章の記録があり、`make fmt`・`make test`・`make lint` が通る

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | 比較のスクリプトとその記録、並列化したテスト、直列のまま残したテストの一覧、計測の記録 |

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-3 を含む。

**実装モデル要件**: standard+high-review

**判定理由**: ステップ 1-2 が、並行性に関わる危険度の高い単独のステップに当たる。高位のロールより軽いモデルが親テストの共有の値を見落として並列にすると、データ競合や実行の重なり方で結果の変わる失敗を持ち込む。こうした失敗は `-count=3` の実行でも見つからないことがある

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   新しいテストは足さない。既存のテストの判定と入力は変えない（1.2 節）。共有の値やその `defer` を移したテストは 7.3 節に記録し、判定と入力が変わっていないことを PR のレビューで読む（設計書 7 章）。
-   AC-01・AC-03・AC-05 は、`compare.sh` の実行で確かめる。スクリプトの各判定が失敗できることは、ステップ 1-1 の壊し方の確認で確かめる。
-   AC-04 は、`-race -count=3` の実行で確かめる。
-   AC-06 は、`make test` と `make lint` で確かめる。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 並列のテストの間で `privatefile.MkdirBelow` の umask の変更が重なり、ほかのテストが作るファイルの権限が狭くなる。この問題は `-race` では見つからない | umask に結果が左右されるテストを直列に残し、そのテストの冒頭で umask を決まった値にする。`-count=3` で前の回の umask が残っても受け継がない（設計書 1 章・3.2 節）。権限は狭くなる方向にしか変わらないので、ほかのテストの判定は変わらない |
| 共有の値を持つテストを並列にし、まれにしか出ない失敗を持ち込む | 設計書 3.3 節をファイルごとに確かめる。`-race -count=3` で実行する（AC-04）。実装の後に、高位のモデルのレビューを行う（3.2 節） |
| 並列の負荷で、実際の時間で待つテストが遅れて失敗する | 設計書 3.3 節のとおり、そのテストは直列のまま残す。判定の時間は変えない |
| 並列化だけでは AC-01 に届かない | ステップ 1-3 で止めて人に知らせる。フィクスチャの高速化は決定の変更として扱う（設計書 1 章） |
| 並行して、chief を変えるほかのタスクの PR が同じテストのファイルを変える | 衝突は、`origin/main` を取り込んで解く。取り込みで `internal/chief` のテストが変わったら、`compare.sh` と `-race -count=3` を実行し直す（7.1 節・7.2 節） |
| `make test` の全体の実行で、ほかのパッケージと CPU を取り合い、時間の比が単独の計測と変わる | AC-01 は、このパッケージだけを続けて実行して比べる（要件書 AC-01）。`make test` の全体の時間は範囲の外である（要件書 2.3 節） |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] フェーズ 1（ステップ 1-1〜1-3）
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1184
- [x] PR がマージされた
- [-] 次のブランチへ切り替えた（最終の PR のため）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `diff` | `_checks/compare.sh` の時間の比の判定。出力と 2 つのコミットは 7.0 節に書く |
| AC-02 | `manual` | PR のレビューで、7.0 節と PR の説明に、2 つの経過時間、その比、2 つのコミットがあることを読む |
| AC-03 | `diff` | `_checks/compare.sh` の名前の集合の判定。出力は 7.0 節に書く |
| AC-04 | `test` | `go test -tags test -race -count=3 ./internal/chief` の実行。記録は 7.2 節に書く |
| AC-05 | `diff` | `_checks/compare.sh` の変わったファイルの一覧の判定。出力は 7.0 節に書く |
| AC-06 | `test` | `make test`・`make lint` |

### 7.0. `compare.sh` の出力（AC-01・AC-03・AC-05）

-   **比べたコミット:** マージベース `98456465ab0ee32e35941a6a406b07cf04462d36`（変更前）と、`HEAD` の `2e796dbe64d1fd8b0efb1788a5e4008c2e0b9320`（変更後。作業ツリーはきれい）。
-   **計測の間の負荷:** 計測の前に、同じ機械でほかのテストやビルドが走っていないかを人に尋ね、「走っていない」という答えを得た（2026-11-26）。
-   **出力（`git fetch origin main` の後に実行）:**

```text
before (merge-base): 98456465ab0ee32e35941a6a406b07cf04462d36
after  (HEAD):       2e796dbe64d1fd8b0efb1788a5e4008c2e0b9320 (work tree clean: yes)
elapsed before: 44.548s, after: 20.133s, ratio: 0.452
passing tests before: 2259, after: 2259
fail/skip before: 0, after: 0
changed files: 43
AC-01 (ratio <= 0.5): PASS
AC-03 (same passing names, no fail/skip): PASS
AC-05 (only allowed files changed): PASS
exit 0
```

-   **変わったファイルの一覧（AC-05。マージベースと `HEAD` の `git diff --name-only`）:** `internal/chief/` 直下の `_test.go` 40 ファイル、`internal/chief/test_helpers.go`、本タスクのディレクトリの下の 2 ファイル（`03_implementation_plan.md`、`_checks/compare.sh`）。`_test.go` でないのは次の 3 つである。

```text
docs/tasks/0128_chief_test_speedup/03_implementation_plan.md
docs/tasks/0128_chief_test_speedup/_checks/compare.sh
internal/chief/test_helpers.go
```

-   **ばらつきの記録:** 変更前の経過時間は、同じマージベースの木でも実行ごとに変わった。本タスクの途中の実行では、変更前が 51.0 秒（変更後の側は、テストを変えていない `HEAD` で 94.6 秒）・48.8 秒、並列化の途中の実行では 115.4 秒（変更後 28.7 秒、比 0.249）で、上の最終の実行は 44.5 秒（変更後 20.1 秒、比 0.452）だった。変更後の経過時間は、単独の実行で 20〜29 秒だった。比の判定は最終の実行で通ったが、変更前が短く出る実行では、比が 0.5 に近づく。

### 7.1. 一時スクリプトの記録（`_checks/compare.sh`）

-   **スクリプト:** `docs/tasks/0128_chief_test_speedup/_checks/compare.sh`。変更前（マージベース）と変更後（`HEAD`）の `internal/chief` のテストを続けて実行して比べる。AC-01・AC-03・AC-05 を確かめる。
-   **判定:**
    -   時間の比（AC-01）
    -   テストの名前の集合と、`fail`・`skip` の有無（AC-03）
    -   変わったファイルの一覧（AC-05）
-   **壊し方の確認:** 各判定が少なくとも 1 つの壊し方で失敗することを確かめる。壊し方は、コミットした内容の上で作業ツリーに入れ、確かめた後に戻す。
-   **予定の実行:**
    -   PR を作る前に、`/runplan` が 1 回実行する（2 章の計測と記録の手順）。
    -   その後にスクリプトか `internal/chief` のテストが変わったとき（レビューの直し、`origin/main` の取り込み）は、変えた人が実行し直す（`docs/dev/conventions/requirements_process.md`「Temporary script record」の「Rewriting」）。
-   **テスト:** コミットしない。
-   **壊し方の記録:** 確かめた内容のコミットは `c5eb58126a69`（スクリプトを追加したコミット）である。壊し方は作業ツリーに入れ、確かめた後に戻した（`git checkout`）。
    -   AC-01（時間の比）: テストのファイルに変更のない `HEAD` で実行した。時間の比は 1.854 で、判定は `FAIL`、終了コードは 1 だった。壊し方は「`t.Parallel()` を足していない木」で、同じ木の 2 回の実行が、比 1.854 と 0.905 で、どちらも `FAIL` だった（2 回目は AC-03・AC-05 の壊し方を入れた実行）。並列化していない木では、この判定は通らない。比は実行ごとに大きく変わる（7.0 節）。比が 0.5 の近くに出たときは、スクリプトを変えず、機械が空いている状態で実行し直す。
    -   AC-03（名前の集合と `fail`・`skip`）: `internal/chief/workspace_test.go` の `TestGitIsAncestor` の冒頭に `t.Skip("breakage")` を足して実行した。判定は `FAIL`、`pass` のテストは変更前の 2259 から 2251 に減り、`skip` が 1 つあった。終了コードは 1 だった。
    -   AC-05（変わったファイル）: `internal/chief/records.go` の末尾に行を足して実行した（AC-03 の壊し方と同じ実行）。判定は `FAIL`、許す範囲の外のファイルとして `internal/chief/records.go` が表示された。終了コードは 1 だった。
    -   同じ状態で、AC-03・AC-05 の壊し方なしの実行（`HEAD`、変更なし）は、両方 `PASS` だった（変更のないテストの名前の集合は同じで、変わったファイルは `_checks/compare.sh` だけである）。
-   **実行の記録:** PR を作る前の 1 回の実行は、7.0 節に置いた（`diff` 検証のスクリプトなので、出力と 2 つのコミットは 7.0 節に置く）。確かめた内容のコミットは `2e796dbe64d1`、結果は `PASS`（終了コード 0）である。

### 7.2. `-race -count=3` の実行の記録

-   `go test -tags test -race -count=3 ./internal/chief` を、コミット `2e796dbe64d1` で実行した。終了コードは 0、出力は `ok  github.com/isseis/yt2column/internal/chief 60.238s` だった。
-   親テストを並列にしていた実装（`48fabf346af2`）の `-count=3` では 1 回失敗した。原因と対処は 7.3 節に書いた。対処の後（`265c199ef89c`）の `-count=10` は通った（191.7 秒）。
-   `make test`（終了コード 0）と `make lint`（`0 issues.`）を、コミット `2e796dbe64d1` で実行した。`make test` を最初に実行したとき、`internal/fixprguard` が既定の 10 分の時間切れで失敗した。同じパッケージだけを実行すると通り（386 秒）、`make test` をもう一度実行すると通った（そのパッケージは 288.9 秒）。本タスクが変えたファイルではなく、ほかのパッケージとの負荷の重なりによる時間切れだった。

### 7.3. 直列に残したテストと、`t.Parallel()` を足す以上に変えたテスト

-   **並列にしたテスト:** `internal/chief` の 40 のファイルで、トップレベルのテスト 394 に `t.Parallel()` を足した。`TestMarkerReadingMatchesDecisionScript` の 23 のサブテストにも足した（親は直列、下の「直列に残したテスト」）。各サブテストは自分のリポジトリを作り、親の `contextDoc` と `old` を読むだけである。
-   **直列に残したテスト:** 次の 16 のテストは、そのテストも祖先も `t.Parallel()` を呼ばない。
    -   `t.Setenv` を呼ぶ（`RealCommands` を通す場合を含む）: `TestWorktreeAddDetached`（`create_test.go`）、`TestWorktreeRemoveKeepsBranch`（`removal_test.go`）、`TestIssueIndexIgnoresGrepPatternType`・`TestIssueTasksReadsTheRealGitGrep`・`TestIssueTasksWithNoLine`（`task_test.go`）、`TestTaskSpaceWorktreeMatchesCanonicalWorktree`（`taskspace_test.go`）、`TestGitWorktreesOfARealRepository`・`TestGitIsAncestor`・`TestGitCleanCountsStagedAndNestedFiles`（`workspace_test.go`）の 9 つ。`TestGitClean`（`autosend_test.go`）。`network_test.go` の 4 つ（`TestSnapshotRunsOnlyReadsInTheRightPlaces`・`TestDetailAndSendRunOnlyReadsInTheRightPlaces`・`TestTickRunsOnlyReadsInTheRightPlaces`・`TestCreateRunsOnlyInTheRightPlaces`）。
    -   umask に結果が左右される: `TestClearRecordsKeepsTheCreatedRecordWhenAnotherRemovalFails`（`records_test.go`）。`os.WriteFile` で `0o644` のファイルを作り、`privatefile` が拒むことを確かめる。冒頭で `fixUmask` を呼ぶ。`records_test.go` の `:285`・`:945`・`:967` は `os.Chmod` で権限を決めるので、umask に左右されない。`:427` は `MkdirBelow` が作るディレクトリが `0o700` であることを確かめる。umask が `077` で残っても、`0o700` は変わらない。
    -   子プロセスを起こす並列のサブテストの親: `TestMarkerReadingMatchesDecisionScript`（`markers_parity_test.go`）。親は直列に残し、23 のサブテストは並列にした。親が直列なので、サブテストはほかのトップレベルのテストと同時には走らず、サブテストの間だけで並列に走る。理由: このサブテストは git と bash の子プロセスを起こす。子プロセスは、fork から exec までの間、親が開いている fd の複製を持つ。そのため、並列のほかのテストが `cachelock` で取った flock は、そのテストが閉じた後もしばらく保たれ、次の `Acquire` が busy になりうる。親も並列にした最初の実装では、`go test -tags test -race -count=3` の 1 回で `TestAutoSendReportMismatch/Claude_Code:_the_session's_file_is_missing` が `another send to the workspace is under way`（`not_sent busy`）で失敗した。原因の仮説であり、再現させて確かめてはいない。親を直列にした後は、`-count=10` が通った（191.7 秒、コミット `265c199ef89c`）。
    -   子プロセスを起こすテストは、ほかに `newTestRepo` を使うテストと `RealCommands` を使うテストだが、どれも直列である。並列のテストは子プロセスを起こさない。
    -   この決まり（子プロセスを起こすテストは直列に残すか、直列の親のサブテストにする）は、正しさの義務である（`CLAUDE.md`「Performance」）。`newTestRepo` の説明に書いた（`test_helpers.go`）。後でテストを足す人が `t.Parallel()` を呼んで `newTestRepo` を使っても、コンパイルも `-race` も通り、まれに `busy` で失敗するだけなので、機械的には守れていない。直列に残す仕組み（`t.Setenv` による panic など）は、並列のサブテストが使う `newTestRepo` を直列の目的に使えなくするので、足さなかった。
    -   サブテストを並列にしたのは、この 23 だけである。ほかのテストのサブテストは、判断を 3.3 節のとおりに行ったのではなく、トップレベルの並列化だけで AC-01 に届いたので、足さなかった（YAGNI）。
    -   共有の値、親の本体の順序、時間に依存する判定が理由で直列に残したテストはない。実際の時間で待つ `TestTranscriptFIFORefused` も並列にしたが、`-race -count=3` で失敗しなかった。
-   **共有の値を移したテストと、`defer` を `t.Cleanup` に移したテスト:** なし。並列のサブテストを持つのは `TestMarkerReadingMatchesDecisionScript` だけで、親の値は読むだけであり、親に `defer` もない。そのほかのテストのサブテストは直列に実行されるので、親の本体の順序は変わらない。
-   **`test_helpers.go` の変更:** `fixUmask` を 1 つ足した（`syscall` の import と定数 `fixedUmask` を含む）。umask を `0o022` にし、`t.Cleanup` で元に戻し、`t.Setenv` を呼ぶ。理由は設計書 1 章と 3.2 節のとおり。`records_test.go` の 1 つのテストだけが呼ぶ。

## 8. 成功基準 (Success Criteria)

-   **機能:** AC-01〜AC-06 の検証が、7 章のとおりに済んでいる。
-   **品質:** `make test`・`make lint` が通る。テストの名前の集合が変更前と同じである（AC-03）。
-   **セキュリティ:** N/A（要件書 4.2 節）。
-   **文書:** 7 章の記録と PR の説明に、計測の結果がある（AC-02）。

## 9. 次のステップ (Next Steps)

-   PR のマージの後、issue #1169 を閉じる（`/runpr` が、タスクの最後の計画の PR の後に提案する）。

## 変更履歴

- 2026-11-26 承認（isseis、#1176）
