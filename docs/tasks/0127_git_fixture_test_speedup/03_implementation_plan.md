# 実装計画書：review_records・high_review_decision・mergemain のテストの高速化

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-26 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を指し、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。用語は、要件書 6 章と設計書 1.3 のもの（フィクスチャのリポジトリ、末端のサブテスト、実時間、ケースの前提の検査、計測の条件、共有の状態、共通の初期状態、共有する準備、共有する実行の結果、ストア、種別、鍵、作り手、複製、整え直し）を同じ意味で使う。`design_handoff.md` の H-01〜H-05 は、設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。

既存のコードについての記述は、コミット `51f82a71` のファイルで確かめた。設計書が確かめたコミット `95793184` から、`cmd/yt2column`・`internal/mergepr`・`internal/pipeline`・2 つのスクリプト・`package_reference.md` に変更はない（`git diff --stat 95793184 51f82a71` が空）。そのため、設計書の `file:line` はこのコミットでも同じ行を指す。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

本計画は、3 つのファイル（`cmd/yt2column/review_records_test.go`・`cmd/yt2column/high_review_decision_test.go`・`internal/mergepr/mergemain_test.go`）のテストの準備を共有の状態の複製に替え、`cmd/yt2column` と `internal/mergepr` の実時間を、変更の前の 75% 以下にする（F-001）。その際、ケースの集まり、bash 3 での実行、ケースの前提の検査、各ケースの隔離は減らさない（F-002・F-003）。

### 1.2. 実装の原則

-   変えるのはテストのコードだけにする（設計書 1.1、AC-09）。スクリプトと `internal/mergepr` の製品コードは変えない。
-   ケースの表の行・`setup`・`want` は今の場所に残す（要件書 4.5）。変えるのは、表を走らせる関数とフィクスチャを作る関数である。
-   共通の初期状態の複製、実行の結果の共有、準備の共有、並列化は、それぞれ revert で切り離せる別のコミットにする（要件書 5 章、設計書 8 章）。複製を入れるコミットには、「複製した状態は作り直した状態と同じである」という義務と、それを固定するテストを書く（CLAUDE.md「Performance」）。
-   ケースの前提の検査（AC-07）は、それが確かめるフィクスチャや実行を変えたコミットで、前提を一時的に崩してケースが失敗することを確かめ、そのコミットのメッセージに書く。

### 1.3. 計画の際に決めたこと（2026-11-26）

人間のレビュアーが決めたもの:

-   **実装の PR は 2 本にする:** PR-1 は `internal/snapshottest`、`cmd/yt2column` の変更（設計書 3.2〜3.5 と 3.8 の `cmd/yt2column` の分）、`_checks/` の 2 つの一時スクリプトである。PR-2 は `internal/mergepr` の変更（設計書 3.6・3.8 の `internal/mergepr` の分と、届かないときの 3.7）である。理由: 1 本の PR ごとに、計測（BASE と AFTER の 3 回ずつ）、AC-04・AC-11 の比較、`make coverage-compare` で、機械を約 1 時間使う。PR ごとの割合を掛け合わせる（F-001）ので、PR を増やすと計測の誤差も重なる。パッケージの境目で切れば、PR-1 で `cmd/yt2column` の割合がほぼ決まり、PR-2 で `internal/mergepr` の割合が決まる。採らなかった案: 3 本（共通の初期状態だけでは `cmd/yt2column` が 0.75 に届かず、届くかどうかが 2 本目まで分からない）と、1 本（レビューの量が大きい）。ADR: なし（このタスクの中で閉じる進め方の選択である）。
-   **計測は `/runplan` が流す:** 計測と比較の前に、ほかの重い処理を走らせていないことを人に問い（select-question）、答えを得てからバックグラウンドで流す。理由: F-001 は、ほかの重い処理を走らせずに測ることを求める。一方、`chief` は、ほかの workspace にコマンドを並行して送ることがある。人が手で流す案は、PR の前に手作業が入り、自動送信が `hold` になるので採らない。ADR: なし（同上）。

計画で決めたもの（人間のレビュアーに諮らず、理由とともに記す）:

-   **`internal/mergepr` の `TestMain` は PR-2 で足す:** 設計書 8 章の 1 は、2 つのパッケージの `TestMain` のストアを最初に挙げる。しかし、PR-1 では `internal/mergepr` のストアを使うコードがないので、`internal/mergepr/main_test.go` は、それを使うフェーズ 7 で足す。PR-1 の差分が `internal/mergepr` に触れず、PR-1 の計測で `internal/mergepr` の時間が変わる原因が、もう一方のパッケージの影響だけになる（設計書 2.1「2 つのパッケージの相互の影響」）。
-   **一時スクリプトはフェーズ 6 に置く:** 設計書 8 章の 7 は「最初の実装の PR の計測の前に置く」と定める。そこで、PR-1 の計測の直前のフェーズ 6 に置き、`internal/mergepr` のフェーズ（設計書 8 章の 6）を、PR-2 のフェーズ 7 にする。
-   **AC-09 の範囲を、比較のスクリプトの判定で確かめる:** 設計書 7.4 は、`git diff --name-only` の結果を読むとする。要件書 AC-09 は変更の前と後の関係なので、`diff` の確かめにする。手で読む代わりに、比較のスクリプト（`_checks/compare.sh`）の判定の 1 つにする。変えたファイルの数が多く、記録した差分を読む方法（文書の小さな差分のためのもの）は当たらないためである。
-   **AC-11 の「最初でも最後でもない 1 つ」:** 共有の状態の種別ごとに、次の末端のサブテストを選ぶ。どれも、コミット `51f82a71` で `go test -tags test -count=1 -json -run <各階層を ^...$ で囲んだ式>` を流し、`pass` の事象を確かめた。`compare.sh` には、これらを引数で渡す。

    | 種別 | 末端のサブテスト |
    |---|---|
    | `rrFixture` の共通の初期状態 | `TestReviewRecordsCrossCheck/path-bash/fail-closed_context_validation/valid_mapping_table_control` |
    | `hrdFixture` の共通の初期状態 | `TestHighReviewDecision/values/old_sonnet` |
    | git の設定のシナリオのグループのブランチ（共有する準備） | `TestReviewRecordsCoverage/path-bash/git_config/global/core.abbrev` |
    | 設定の前の実行（共有する実行の結果） | `TestReviewRecordsCrossCheck/path-bash/git_config_with_a_merge/repository/diff.relative` |
    | `runRRCases` のケースの準備（共有する準備） | `TestReviewRecordsCoverage/path-bash/merge_commit_inside_a_seen_range` |
    | 空の `takeIn`（共通の初期状態） | `TestMergemainProtocolUpdateAndRuleFacts/protocol_line_conflict_resolved_to_main_plus_one` |
    | 競合のマージの途中（共有する準備） | `TestMergemainRestoreSafeAndRefusesOtherChanges/other_branch` |
    | マージのコミットの後（共有する準備） | `TestMergemainPushChecksParentsAndMain/merge_in_progress` |

    `bin-bash-3` の名前を選ばないのは、`/bin/bash` が 3.x でない機械には、その名前がないためである。競合のマージの途中の 12 件では、名前の順で `MERGE_HEAD_is_another_commit` が最初、`untracked_file` が最後になるので、そのどちらでもない `other_branch` を選んだ。

### 1.4. 既存コード調査結果

`file:line` はコミット `51f82a71` の行番号である。

-   **`cmd/yt2column` のフィクスチャ:**
    -   `newGitFixture`（`test_helpers_gitfixture.go:30-36`）は、`HOME`・`TMPDIR`・リポジトリを、どれも `t.TempDir()` に作る。`cross_check_state_docs_test.go` と `adr_candidates_docs_test.go` も、これを 1 か所ずつ呼ぶ。本タスクの範囲外なので、今の形は残し、作る場所を引数で受け取る形を足す（設計書 3.1「`build` の中で作るもの」）。
    -   `newRRFixtureWithContext`（`review_records_test.go:136-152`）と `newHRDFixture`（`high_review_decision_test.go:32-50`）が、共通の初期状態を作る。どちらも、パッケージの中のすべてのケースがこの 2 つの関数を通るので、呼び出し元を変えずに中身を替えられる。
    -   `rrFixture`（`review_records_test.go:91-99`）は、Go の側の値 `form`・`forms`・`bash`・`contextDoc`・`m0`・`mb` を持つ。`sibling`（`:102-107`）と `probe`（`:111-118`）は、`setup` の中で 2 つ目のフィクスチャを作り、スクリプトを走らせる。
    -   `env` の中で 2 つ目のフィクスチャを作るケース（`:729`、`:1855`）と、`f.home` に書くケース（`:747`、`:1866`）がある。どちらも `env` の中なので、共有の後もケースが自分のフィクスチャで作る（設計書 3.4）。
-   **ケースの表と走らせる関数:**
    -   `rrCase`（`review_records_test.go:291-297`）は `name`・`context`・`setup`・`env`・`want` を持つ。準備の仕方の欄はない。
    -   `runRRCases`（`:839-869`）は、bash ごと、ケースごとにフィクスチャを作り、`setup` を走らせる。ケースの名前の重なりは検査していない。
    -   `requireProbe` を `setup` の中で呼ぶのは、`crossCheckCases`（`:1174` から）の次の 6 件である（`:1646`・`:1665`・`:1673`・`:1693`・`:1732`・`:1756`）: `work then merge after a record from the merge-base`、`work then merge after a record from a branch commit`、`record from the old merge-base ending after the merge`、`merge check naming a commit that is not a merge`、`merge check naming a merge outside the branch`、`merge check naming a merge whose first parent is behind the merge-base`。
    -   `checkConfigScenarios`（`:805-834`）は、4 か所（`:902`、`:1027`、`:2058`、`:2060`）から呼ばれる。
    -   `TestReviewRecordsUnreviewed` は、`setup` の中で `LAST_REVIEWED..HEAD` のケースを skip する（`:1003-1009`）。
    -   最上位で `t.Parallel()` を呼ばないのは、`TestReviewRecordsCoverage`（`:891`）・`TestReviewRecordsUnreviewed`（`:992`）・`TestReviewRecordsMode`（`:1036`）・`TestReviewRecordsCrossCheck`（`:1943`）・`TestReviewRecordsFableAlias`（`:2070`）である。
-   **`cmd/yt2column` の `TestMain`:** `main_test.go:22-50` にある。子のプロセスとして走るときは、`:23-25` で最初に抜ける。
-   **`internal/mergepr`:**
    -   `newTakeIn`（`test_helpers_git.go:245-270`）は、`root` の下に `home`・`origin.git`・`work` を作り、4 つの設定を書く。`env` は `HOME`・`XDG_CONFIG_HOME` を `root/home` に向ける。`newTakeIn` は `mergemain_test.go` の 40 か所から呼ばれる。
    -   `takeIn`（`:232-241`）は、`Origin`・`Work`・`PR`・`Env`・`GHCalls` と、内部の `env` を持つ。
    -   `TestMain` はない（`grep -rn "func TestMain" internal/mergepr` が空）。
    -   `conflictSetup`（`mergemain_test.go:613-616`）の後に begin・merge を呼ぶ 12 件は `:642`・`:686`・`:747`（10 件のループ）、`pushFlow`（`:767-776`）を呼ぶ 22 件は `:800`・`:813`・`:824`・`:841`・`:874`（6 件）・`:924`（6 件）・`:967`（4 件）・`:1254`・`:1267` にある。共有しないのは `:1071`（`TestMergemainBeginMergeRestoreFlow`）と `:1240`（`TestMergemainMergeIgnoresConfiguredMergeOptions`）である（設計書 3.6）。
    -   `GHCalls` を確かめるのは `:159` と `:1093` の 2 つのテストだけで、どちらも共有する準備を使わない。
-   **テストだけのパッケージの登録:** `internal/pipeline/pipeline_test.go:540-547` の `testOnlyPackageDirs` と、`docs/dev/conventions/package_reference.md` の行の 2 か所である（`internal/udstest` で確かめた）。
-   **再利用するもの:** スクリプトを走らせる `runScript`（`test_helpers_gitfixture.go`）、`scriptBashes`・`requireBash4`（`test_helpers_bash.go`）、`takeIn` の `gitIn`、`make coverage-compare` は、そのまま使う。複製は標準ライブラリの `os.CopyFS`（設計書 1.2）で行い、自前の複製は書かない。
-   **競合する手段のある箇所:** 届かないときに足す `Setup` の作り（設計書 3.7）は、手段が決まっていない。候補は、(a) `Setup` の履歴を `git fast-import` などのより少ないプロセスで作る、(b) `Setup` の入力ごとに状態をストアに置いて複製する、の 2 つで、足すことになったときにステップ 7-5 で選ぶ。ほかのステップは、設計書が手段を決めている。
-   **具体的な型の作り方の経路:** `Snapshot` と `Kind` は、新しいパッケージ `internal/snapshottest` に置き、欄を公開しない。外のパッケージは `Load` と `NewKind` からしか作れない（設計書 3.1）。パッケージの中で作る経路は、`Load` と `NewKind` だけにする。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 複製の仕組みとストア（設計書 3.1・3.9、8 章の 1）

- [ ] **ステップ 1-1**: `internal/snapshottest` を作る
    - 追加するファイル: `internal/snapshottest/snapshot.go`（`//go:build test`）
    - 設計書 3.1 の `Store`・`NewStore`・`Close`・`Kind`・`NewKind`・`Snapshot`・`Load`・`CopyTo`・`Value` を作る。作った結果の記録（作った・失敗・skip、記録の前の終わりは失敗）、鍵の長さを付けた連結、作った直後のリンクの検査、`os.CopyFS` での複製は、設計書 3.1 のとおりにする
    - 失敗と skip のメッセージは、設計書 4 章の表の形にする
    - 共有の状態を求めるテストは `t.Setenv` を使わない、という規則を `Load` の doc コメントに書く（設計書 3.1「環境」）
- [ ] **ステップ 1-2**: `internal/snapshottest` のテストを書く
    - 追加するファイル: `internal/snapshottest/snapshot_test.go`
    - 設計書 7.1 の 5 つを確かめる: 並列のテストから同じ種別と鍵を求めても作る関数が 1 回だけ走ること（`TestLoadBuildsOnce`）、違う鍵の列が同じ鍵にならないこと（`TestLoadKeyParts`）、作り手の失敗・`t.Errorf` の後の正常な戻り・skip が後から来たケースを失敗・失敗・skip にすること（`TestLoadPropagatesFailureAndSkip`、テストのバイナリを子のプロセスで走らせ直す）、`Value` の複製（`TestValueIsCloned`）、`CopyTo` の実行の権限とリンク、外を指すリンクの失敗（`TestCopyTo`）
    - 記録の分岐（作った・失敗・skip）をそれぞれ一時的に壊し、`TestLoadPropagatesFailureAndSkip` が失敗することを確かめ、コミットのメッセージに書く
- [ ] **ステップ 1-3**: 新しいパッケージを登録する
    - 変えるファイル: `internal/pipeline/pipeline_test.go`（`testOnlyPackageDirs` に `internal/snapshottest` を足す）、`docs/dev/conventions/package_reference.md`（`internal/snapshottest` の行）
    - `TestFakesCarryBuildTag` の `testutil` のファイルの数（22）は変えない（設計書 3.9）
- [ ] **ステップ 1-4**: `cmd/yt2column` のストアを作る
    - 変えるファイル: `cmd/yt2column/main_test.go`、`cmd/yt2column/test_helpers_gitfixture.go`（ストアの変数）
    - 子のプロセスの分岐（`main_test.go:23-25`）の後で `os.MkdirTemp` の場所に `NewStore` を作り、`m.Run` の後、`os.Exit` の前に `Close` を呼ぶ（設計書 3.1「寿命」）
- 完了の条件: `go test -tags test -race ./internal/snapshottest ./internal/pipeline ./cmd/yt2column`、`make lint` が通る。ステップ 1-1〜1-3 と 1-4 を別のコミットにする

### フェーズ 2: `cmd/yt2column` の共通の初期状態（設計書 3.2・3.8、8 章の 2）

- [ ] **ステップ 2-1**: 2 つのフィクスチャの共通の初期状態を複製にする
    - 変えるファイル: `cmd/yt2column/test_helpers_gitfixture.go`、`review_records_test.go`、`high_review_decision_test.go`
    - `newGitFixture` に、リポジトリ・`HOME`・`TMPDIR` を作る場所を受け取る形を足す。今の形はそれを `t.TempDir()` で呼ぶ
    - `newRRFixtureWithContext` と `newHRDFixture` を、設計書 3.2 の種別と鍵で `Load` し、ケースの一時ディレクトリに複製する形にする。作る関数は今の手順をそのまま使う。`m0`・`mb` は値で受け取る。ケースの `HOME`・`TMPDIR` は今と同じく `t.TempDir()` で作る（AC-10）
    - 複製の後に index を読み直さない（設計書 3.2）
- [ ] **ステップ 2-2**: 複製と作り直しを比べるテストを書く
    - 追加するファイル: `cmd/yt2column/fixture_snapshot_test.go`（`TestFixtureSnapshotsMatchRebuilt`）
    - `rrFixture`（`_context.md` があるものとないもの）と `hrdFixture` の共通の初期状態について、設計書 3.8「比べるもの」を、複製と今の手順で作り直した状態とで比べる。フェーズ 3・4 で、比べる対象を足す
    - 複製の側に作り手のパスを書き足す、ref を 1 つ消す、の 2 つの壊し方で、このテストが失敗することを確かめ、コミットのメッセージに書く
- [ ] **ステップ 2-3**: refs/replace の前提の検査（`high_review_decision_test.go:546`）を確かめる
    - 前提を一時的に崩し、そのケースが失敗することを見て、コミットのメッセージに書く（AC-07）
- 完了の条件: `go test -tags test -race ./cmd/yt2column`、`make lint` が通る。ステップ 2-1〜2-3 を 1 つのコミットにし、そのメッセージに複製の義務（複製した状態は作り直した状態と同じである）と、それを固定するテストを書く

### フェーズ 3: git の設定のシナリオの共有（設計書 3.3、8 章の 3）

- [ ] **ステップ 3-1**: グループのブランチと設定の前の実行を共有する
    - 変えるファイル: `cmd/yt2column/review_records_test.go`（`checkConfigScenarios` とその 4 つの呼び出し）
    - 設計書 3.3 のとおり、グループのブランチ（鍵: 最上位のテスト関数の名前・グループの名前・モード）と、設定の前の実行（鍵: グループの鍵と bash の名前）を種別にする。設定の前の実行を作る関数が、終了コードと出力の先頭（`base`）を検査する
    - グループのブランチは、bash ごとの親のサブテストが、シナリオのサブテストを始める前に求める。各シナリオは、ブランチを複製し、plain git の出力が設定で変わることを確かめ、自分の bash で走らせた結果を、その bash の設定の前の実行と比べる
    - 末端の名前（`<bash>/git config/<scope>/<設定の名前>`）は変えない（AC-04）
- [ ] **ステップ 3-2**: `TestFixtureSnapshotsMatchRebuilt` に、グループのブランチの 1 つ（`rrConfigBranch`）を足す
- [ ] **ステップ 3-3**: 2 つの前提の検査（設計書 7.4 の「設定の前の出力」と「設定による plain git の変化」）と AC-08 を確かめる
    - 設定の前の出力の前提を崩すと、その bash のそのグループのシナリオがすべて失敗し、`go test` が失敗することを見る（AC-07・AC-08）
    - 設定による plain git の変化の前提を崩すと、そのシナリオが失敗することを見る
    - 結果をコミットのメッセージに書く
- 完了の条件: `go test -tags test -race ./cmd/yt2column`、`make lint` が通る。フェーズ 3 を 1 つのコミットにし、そのメッセージに共有の義務（設定の前の実行と各シナリオは同じ共有の状態の複製の上で走る）を書く

### フェーズ 4: ケースの準備の、2 本の bash の間での共有（設計書 3.4、8 章の 4）

- [ ] **ステップ 4-1**: `rrCase` に準備の仕方の欄を足す
    - 変えるファイル: `cmd/yt2column/review_records_test.go`
    - 値は「共有する」（0 値）と「bash ごとに行う」の 2 つにする（設計書 3.4）。1.4 節の 6 件にだけ「bash ごとに行う」を書く
- [ ] **ステップ 4-2**: `runRRCases` で準備を共有する
    - 変えるファイル: `cmd/yt2column/review_records_test.go`
    - ケースを走らせる前に、ケースの名前の重なりを検査する
    - 「共有する」のケースは、設計書 3.4 の鍵（最上位のテスト関数の名前・モード・ケースの名前・`_context.md` の中身）で準備を `Load` し、複製した自分のリポジトリで、自分の bash でスクリプトを走らせる。`env` と `want` は自分のフィクスチャで作る。「bash ごとに行う」のケースは、今の手順のままにする
- [ ] **ステップ 4-3**: 共有する準備の中でスクリプトを走らせない検査を足す
    - 変えるファイル: `cmd/yt2column/test_helpers_gitfixture.go`、`review_records_test.go`
    - 共有する準備を作るフィクスチャ（`sibling` が作るものを含む）に印を付け、印のあるフィクスチャの `runScript` でケースを失敗させる（設計書 3.4、4 章）
- [ ] **ステップ 4-4**: `TestFixtureSnapshotsMatchRebuilt` に、共有する準備の代表の 1 件を足す
- [ ] **ステップ 4-5**: 前提の検査と、共有の失敗を確かめる
    - 次の 10 か所のそれぞれで前提を崩し、それに依るケースが `path-bash` と `bin-bash-3` の両方で失敗することを見る: `requireProbe`（`review_records_test.go:125`）、`mainMerge`（`:235`）、`rrSubmodule`（`:324`）、`setup` の中の 7 か所（`:379`・`:447`・`:690`・`:1356`・`:1439`・`:1817`・`:1832`）（AC-07）
    - 1.4 節の 6 件の 1 つから「bash ごとに行う」を外すとケースが失敗すること、ケースの名前を 1 つ重ねるとテスト関数が失敗すること、共有する準備を作る関数を失敗させると `go test` が失敗することを見る（AC-08）
    - 結果をコミットのメッセージに書く
- 完了の条件: `go test -tags test -race ./cmd/yt2column`、`make lint` が通る。フェーズ 4 を 1 つのコミットにし、そのメッセージに共有の義務（共有する準備は git だけで作り、どちらの bash のケースが作り手になっても同じ状態になる）を書く

### フェーズ 5: 最上位のテスト関数の並列化（設計書 3.5、8 章の 5）

- [ ] **ステップ 5-1**: 5 つの最上位のテスト関数で `t.Parallel()` を呼ぶ
    - 変えるファイル: `cmd/yt2column/review_records_test.go`（1.4 節の 5 つの関数）
    - どれも `t.Setenv` と作業ディレクトリの変更を使わないことを、変える前に読んで確かめる（設計書 3.5）
- 完了の条件: `go test -tags test -race ./cmd/yt2column`、`make lint` が通る。フェーズ 5 を 1 つのコミットにする

### フェーズ 6: 計測と比較の一時スクリプトと PR-1 の記録（設計書 7.5、8 章の 7）

- [ ] **ステップ 6-1**: 計測のスクリプトを書く
    - 追加するファイル: `docs/tasks/0127_git_fixture_test_speedup/_checks/measure.sh`
    - 設計書 7.5「計測のスクリプト」のとおりにする（BASE・AFTER の交互の 6 回、`ok` の行の読み取り、パッケージごとの中央値と割合、機械の情報、元のブランチへの戻し）。言語は bash で、`jq` を使ってよい
- [ ] **ステップ 6-2**: 比較のスクリプトを書く
    - 追加するファイル: `docs/tasks/0127_git_fixture_test_speedup/_checks/compare.sh`
    - 設計書 7.5「比較のスクリプト」のとおりにする（AC-04 の名前・`fail`・`skip` の集まり、AC-11 の単独の選択と `-shuffle=on` の 2 回）。単独の選択の最上位のテスト関数は、AFTER の 3 つのファイルの `func Test` から取る。中間の名前は、1.3 節の表の 8 つを引数で受け取る
    - AC-09 の判定を足す: `git diff --name-only <BASE> <AFTER>` の各ファイルが、要件書 AC-09 の範囲（`cmd/yt2column/`・`internal/mergepr/` の `_test.go` のファイルと最初の行が `//go:build test` のファイル、`internal/snapshottest/` のファイル、`internal/pipeline/pipeline_test.go`、`docs/dev/conventions/package_reference.md`、`docs/tasks/0127_git_fixture_test_speedup/` の下）に入ること。消したファイルは BASE の内容で読み、読めないときは通ったとしない
- [ ] **ステップ 6-3**: 2 つのスクリプトの壊し方を確かめる
    - スクリプトをコミットしてから、各判定が少なくとも 1 つの壊し方で失敗することを確かめ、7.3 節・7.4 節に書く。壊し方は、コミットした内容の上で作業ツリーに入れ、確かめた後に戻す
- [ ] **ステップ 6-4**: PR-1 の計測と記録
    - `git fetch origin main` の後、BASE を `git merge-base origin/main HEAD`、AFTER を `HEAD` にする
    - ほかの重い処理を走らせていないことを人に問い（select-question）、答えを 7.1 節に書く
    - `measure.sh` と `compare.sh` をバックグラウンドで流し、`make coverage-compare BASE=<BASE>`、`make test`、`make lint` を流す。結果を 7.1・7.2・7.6・7.7 節に書く。`coverage-compare` が時間で揺れる関数で `FAIL` になったときは、きれいな作業ツリーで流し直し、すべての実行を書く（AC-06）
    - `cmd/yt2column` の割合が 0.75 を超えたときは、作業を止め、要件書の決定の変更をレビューに諮る（設計書 2.1「届かないとき」）
- 完了の条件: 7.1〜7.4・7.6・7.7 節の PR-1 の記録がある。記録のコミットは、AFTER とはタスクのディレクトリの下だけが異なる

記録の後に、PR を開く前のレビュー（ローカルの CLI レビュー、上位モデルのレビュー）の直しで、タスクのディレクトリの外のファイルが変わったときは、その直しのコミットを AFTER にして、ステップ 6-4 をやり直す。

### PR-1 作成ポイント: shared fixture snapshots for the review records and high-review decision tests

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 5-1 / 6-1 / 6-2 / 6-3 / 6-4

**推奨タイトル**: `test(0127): review_records と high_review_decision のテストの準備を複製で共有して速くする`

**レビュー観点**: `Load` が作り手の失敗と skip を後から来たケースに伝え、作りかけの状態を渡さないこと（設計書 3.1） / 各ケースのスクリプトの実行がそのケースの bash で行われ、設定の前の実行の鍵が bash を含むこと（AC-05） / ケースの表の行・`setup`・`want` が変わっていないこと（要件書 4.5） / 7 章の計測と比較の記録が AC-01〜AC-12 の PR-1 の分を満たすこと

**実装モデル要件**: standard+high-review

**判定理由**: ステップ 1-1 の `Load` は、作り手の goroutine での作成と、失敗・skip の伝達を扱う並行の処理であり、PR は mkplan の条件つきの確認のうち 2 つ（`t.TempDir` の外に置く共有の状態、`test` のビルドタグだけでコンパイルする新しいソース）に当たる。`high` 役割より軽いモデルが実装すると、作り手の skip や失敗を伝え損ねてケースを黙って合格させるか、ステップ 6-1 の計測が途中で止まったときに作業ツリーを BASE に残すおそれがある

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 7: `internal/mergepr` の共通の初期状態と準備の共有（設計書 3.6〜3.8、8 章の 6）

- [ ] **ステップ 7-1**: `internal/mergepr` のストアを作る
    - 追加・変えるファイル: `internal/mergepr/main_test.go`（新規、`TestMain`）、`internal/mergepr/test_helpers_git.go`（ストアの変数）
    - ストアの作り方と消し方は、ステップ 1-4 と同じにする
- [ ] **ステップ 7-2**: 空の `takeIn` を共通の初期状態にする
    - 変えるファイル: `internal/mergepr/test_helpers_git.go`
    - `newTakeIn` の今の手順を作る関数にし、`origin.git` と `work` を 1 つのディレクトリの下に作る。ケースは、それを自分の一時ディレクトリに複製し、自分の `home` と `env` を作り、`remote.origin.url` を複製した origin に書き換える（設計書 3.6「整え直し」）。`PR`・`Env`・`GHCalls` は今と同じ既定の値から始める
    - 追加するファイル: `internal/mergepr/takein_snapshot_test.go`（`TestTakeInSnapshotsMatchRebuilt`）。空の `takeIn` について、設計書 3.8「比べるもの」を比べる
    - `remote.origin.url` の書き換えを外すと `TestTakeInSnapshotsMatchRebuilt` が失敗することを確かめ、コミットのメッセージに書く
    - このステップを 1 つのコミットにし、そのメッセージに複製の義務を書く
- [ ] **ステップ 7-3**: 競合のマージの途中とマージのコミットの後を共有する
    - 変えるファイル: `internal/mergepr/mergemain_test.go`、`test_helpers_git.go`
    - 設計書 3.6 の表の 2 つの共有する準備を作る。値（base head、main、begin の結果、merge の結果）は種別の `clone` で写す。作る関数は、準備の merge のエラーが nil であることを確かめる。1.4 節の 12 件と 22 件を、それを複製する形に替え、2 件（`:1071`・`:1240`）は今のままにする
    - 整え直しに、index の stat の情報の読み直し（unmerged の項目を残す）を足す
    - `TestTakeInSnapshotsMatchRebuilt` に、2 つの共有する準備を足す（`git diff-files` の比較を含む）
    - index の読み直しを外すと `TestTakeInSnapshotsMatchRebuilt` が失敗すること、共有する準備を作る関数を失敗させると `go test` が失敗すること（AC-08）を確かめ、コミットのメッセージに書く
    - このステップを 1 つのコミットにし、そのメッセージに整え直しの義務を書く
- [ ] **ステップ 7-4**: PR-2 の計測と記録
    - ステップ 6-4 と同じ手順で流し、7.1・7.2・7.6・7.7 節に PR-2 の記録を書く
    - このタスクの割合（PR-1 と PR-2 の割合の積）を、パッケージごとに 7.1 節に書く
    - `internal/mergepr` のこのタスクの割合が 0.75 を超えたときは、ステップ 7-5 に進む。超えないときは、ステップ 7-5 に「足さなかった」と書いて印を付ける
- [ ] **ステップ 7-5（条件つき）**: `Setup` の git の起動を減らす（設計書 3.7）
    - 変えるファイル: `internal/mergepr/test_helpers_git.go`、`takein_snapshot_test.go`
    - 1.4 節の 2 つの候補から手段を選び、選んだ理由をコミットのメッセージに書く。`Setup` の後の状態とコミットの順序と親子を今と同じにし、`TestTakeInSnapshotsMatchRebuilt` に `Setup` の後の状態の比較を足す
    - このステップを 1 つのコミットにし、ステップ 7-4 の計測と記録をやり直す。足したことと計測の値を 7.1 節に書く（設計書 3.7「記録」）
    - それでも 0.75 を超えるときは、作業を止め、要件書の決定の変更をレビューに諮る（設計書 2.1）
- 完了の条件: `go test -tags test -race ./internal/mergepr`、`make test`、`make lint` が通り、7 章の PR-2 の記録がある

ステップ 6-4 の後の段落と同じく、記録の後にタスクのディレクトリの外のファイルが変わったときは、ステップ 7-4 をやり直す。

### PR-2 作成ポイント: shared take-in snapshots for the mergemain tests

**対象ステップ**: 7-1 / 7-2 / 7-3 / 7-4 / 7-5

**推奨タイトル**: `test(0127): mergemain のテストの準備を複製で共有して速くする`

**レビュー観点**: 複製の後の整え直し（`remote.origin.url` と index）で、ケースが作り手の origin や古い stat の情報を読まないこと（設計書 3.6） / 共有しない 2 件と、`GHCalls` を確かめるテストが今のままであること / ステップ 7-5 を足したかどうかと、その理由と計測の値 / 7 章の記録と、2 つの PR の割合の積が AC-01・AC-02 を満たすこと

**実装モデル要件**: standard+high-review

**判定理由**: ステップ 7-5 は計測の結果しだいで足す手段で、1.4 節に 2 つの候補があり、ステップ 7-2・7-3 は複製した git の状態の整え直しを扱う。`high` 役割より軽いモデルが実装すると、整え直しが欠けたまま復元のケースを通すか、ケースに作り手の origin へ push させるか、`Setup` を作り直して状態を変え、比較のテストを弱めるおそれがある

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | `internal/snapshottest` とそのテスト、`cmd/yt2column` のストア |
| M2（フェーズ 2〜5） | `cmd/yt2column` の共通の初期状態の複製、設定のシナリオの共有、準備の共有、並列化、比較のテスト |
| M3（フェーズ 6） | 2 つの一時スクリプトと、PR-1 の計測・比較の記録 |
| M4（フェーズ 7） | `internal/mergepr` の共通の初期状態と準備の共有、比較のテスト、PR-2 の記録と、このタスクの割合 |

フェーズの順は、設計書 8 章の優先順位のとおりである。ただし、1.3 節のとおり、8 章の 7（一時スクリプト）は PR-1 の計測の前のフェーズ 6 に置き、8 章の 1 のうち `internal/mergepr` の `TestMain` は、フェーズ 7 に置く。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 5-1 / 6-1 / 6-2 / 6-3 / 6-4 | `internal/snapshottest`、`cmd/yt2column` の共通の初期状態・設定のシナリオ・準備の共有と並列化、比較のテスト、2 つの一時スクリプトと PR-1 の記録 | standard+high-review | auto |
| PR-2 | 7-1 / 7-2 / 7-3 / 7-4 / 7-5 | `internal/mergepr` の共通の初期状態と 2 つの共有する準備、整え直し、比較のテスト、届かないときの `Setup`、PR-2 の記録とこのタスクの割合 | standard+high-review | auto |

## 4. テスト戦略 (Test Strategy)

-   **単体テスト:** `internal/snapshottest` の 5 つのテスト（ステップ 1-2、設計書 7.1）。失敗と skip の伝達は、子のプロセスの出力で確かめる。
-   **複製の義務:** `TestFixtureSnapshotsMatchRebuilt` と `TestTakeInSnapshotsMatchRebuilt` が、複製（`takeIn` では整え直した状態）と作り直した状態を比べる（設計書 3.8）。比べる対象は、それを共有にするフェーズで足す。
-   **結合テスト:** 3 つのファイルのテストそのものが、共有の状態とスクリプト・`mergemain` の結合テストである。ケースの期待する出力は変えない。ケースが減っていないことは、`compare.sh` の名前の集まりの比較で確かめる（AC-04）。
-   **前提の検査と失敗の伝達:** 13 か所の前提の検査（AC-07）と、共有の状態を作れなかったときの失敗（AC-08）は、前提や作る関数を一時的に壊して確かめ、コミットのメッセージに書く（ステップ 2-3・3-3・4-5・7-3）。
-   **順序と選び方:** 単独の選択と `-shuffle=on` を `compare.sh` で確かめる（AC-11）。
-   **後方互換:** 製品コードとスクリプトは変えない（AC-09）。Linux（CI）では `/bin/bash` の名前のケースがないことは今と同じである（要件書 4.4）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| `cmd/yt2column` の割合が 0.75 に届かない（設計書 2.1 の見込みは 0.65〜0.75） | ステップ 6-4 で測り、届かなければ止めて、要件書の決定の変更をレビューに諮る |
| `internal/mergepr` の割合が 0.75 に届かない（見込みは 0.74〜0.76） | ステップ 7-5 で `Setup` の作りを足す。それでも届かなければ止めて諮る |
| 作り手の skip や失敗が伝わらず、ケースが確かめずに合格する | `TestLoadPropagatesFailureAndSkip` と、ステップ 3-3・4-5・7-3 の壊し方。PR を開く前に上位モデルのレビューを行う（3.2 節） |
| 複製に作り手のパスが残り、ケースが作り手の状態を読み書きする | 比較のテストが、作り手のパスを含むファイルを調べる（設計書 3.8） |
| 共有する準備の中でスクリプトが走り、probe が 1 本の bash でしか走らない | ステップ 4-3 の検査が、そのケースを失敗させる |
| 計測の途中で止まり、作業ツリーが BASE のまま残る | `measure.sh` と `compare.sh` は、変更のない作業ツリーでだけ始め、どの終わり方でも元のブランチに戻す。壊し方の確認で、戻すことも確かめる |
| ほかの workspace の処理が計測と重なり、割合がぶれる | 計測の前に人に問う（1.3 節）。BASE と AFTER を交互に流す（設計書 7.5） |
| PR を開く前のレビューの直し、PR のレビューの直し、`origin/main` の取り込みで、記録が古くなる | 変えた人が、影響を受ける記録を取り直す（フェーズ 6・7 の終わりの段落、7.3 節・7.4 節の予定の実行） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 5-1 / 6-1 / 6-2 / 6-3 / 6-4）
- [ ] PR-2 マージ済み（対象ステップ: 7-1 / 7-2 / 7-3 / 7-4 / 7-5）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `test` | `_checks/measure.sh` の PR ごとの計測と、7.1 節のこのタスクの割合（PR-1 と PR-2 の積）。スクリプトの記録は 7.3 節 |
| AC-02 | `test` | 同上（`internal/mergepr`） |
| AC-03 | `test` | `_checks/measure.sh` の出力（機械の情報、BASE と AFTER の ID、6 回の時間、中央値、割合）を 7.1 節に書く |
| AC-03 | `manual` | PR のレビューで、7.1 節が AC-03 の項目をすべて持つことを読む |
| AC-04 | `diff` | `_checks/compare.sh` の「名前の集まり」と「`fail` と `skip`」の判定。結果は 7.2 節 |
| AC-05 | `test` | `_checks/compare.sh` の「名前の集まり」の判定で、`path-bash` と `bin-bash-3` の名前の下の末端が BASE と同じく合格すること。機械が `/bin/bash` 3.x の macOS であることは、7.1 節の機械の情報で確かめる |
| AC-05 | `manual` | PR のレビューで、設定の前の実行の鍵が bash の名前を含み（ステップ 3-1）、共有する準備のケースが自分の bash でスクリプトを走らせる（ステップ 4-2）ことを読む |
| AC-06 | `test` | PR ごとの `make coverage-compare BASE=<BASE>`。出力は 7.6 節 |
| AC-07 | `test` | 13 か所の前提の検査のそれぞれを崩す確かめ。崩した場所と記録するコミットは 7.5 節 |
| AC-08 | `test` | `internal/snapshottest/snapshot_test.go::TestLoadPropagatesFailureAndSkip`。共有の状態を作る関数を壊して `go test` が失敗することを、ステップ 3-3・4-5・7-3 のコミットのメッセージに書く |
| AC-09 | `diff` | `_checks/compare.sh` の「差分の範囲」の判定。結果は 7.2 節 |
| AC-10 | `test` | `cmd/yt2column/fixture_snapshot_test.go::TestFixtureSnapshotsMatchRebuilt`、`internal/mergepr/takein_snapshot_test.go::TestTakeInSnapshotsMatchRebuilt`（複製の設定に作り手のパスがないこと） |
| AC-10 | `manual` | PR のレビューで、ケースが自分の `HOME`・`TMPDIR`（`internal/mergepr` では `HOME`・`XDG_CONFIG_HOME`）を作ることを読む（ステップ 2-1・7-2） |
| AC-11 | `test` | `_checks/compare.sh` の「単独の選択」と「shuffle」の判定。結果は 7.2 節 |
| AC-12 | `test` | PR ごとの `make test` と `make lint`。結果は 7.7 節 |

### 7.1. 計測の記録（AC-01〜AC-03、AC-05）

（`/runplan` が書く。PR ごとに、計測の前の人の答え、BASE と AFTER の完全な ID、`measure.sh` の出力の全体（機械の情報、6 回のパッケージごとの時間、中央値、割合）。PR-2 の後に、パッケージごとのこのタスクの割合と、ステップ 7-5 を足したかどうか）

### 7.2. 比較の記録（AC-04・AC-05・AC-09・AC-11）

（`/runplan` が書く。PR ごとに、BASE と AFTER の完全な ID と、`compare.sh` の出力の全体（判定ごとの結果、`-shuffle` の 2 回の実行の各パッケージのシード））

### 7.3. 一時スクリプトの記録（`_checks/measure.sh`）

-   **スクリプト:** `docs/tasks/0127_git_fixture_test_speedup/_checks/measure.sh`。BASE と AFTER を交互に取り出して計測の条件のコマンドを 3 回ずつ流し、パッケージごとの時間、中央値、割合と、機械の情報を出す（設計書 7.5）。AC-01・AC-02・AC-03 を確かめる。
-   **判定:**
    -   作業ツリーに変更がないこと
    -   6 回のどの実行も通り、2 つのパッケージの時間を読めること
    -   機械の情報を読めること
    -   終わった後に、元のブランチに戻っていること
-   **壊し方の確認:** 各判定が少なくとも 1 つの壊し方で失敗することを確かめる。
-   **予定の実行:**
    -   PR-1 と PR-2 のそれぞれで、PR を作る前に、`/runplan` が人に問ってから 1 回実行する（ステップ 6-4・7-4）。ステップ 7-5 を足したときは、その後にもう 1 回実行する。
    -   記録の後に、タスクのディレクトリの外のファイルが変わったとき（PR を開く前のレビューの直し、PR のレビューの直し、`origin/main` の取り込み）は、変えた人が、人への問いからやり直して実行し直す。
-   **テスト:** コミットしない。
-   **壊し方の記録:** （`/runplan` が書く）
-   **実行の記録:** （`/runplan` が書く。出力と 2 つのコミットは 7.1 節に置き、ここからはその場所を指す）

### 7.4. 一時スクリプトの記録（`_checks/compare.sh`）

-   **スクリプト:** `docs/tasks/0127_git_fixture_test_speedup/_checks/compare.sh`。BASE と AFTER で `-json` の実行を流して比べ、AFTER で単独の選択と `-shuffle=on` の 2 回を流し、BASE と AFTER の間の変えたファイルを調べる（設計書 7.5、1.3 節）。AC-04・AC-05・AC-09・AC-11 を確かめる。
-   **判定:**
    -   **名前の集まり（AC-04・AC-05）:** BASE の最上位のテスト関数ごとに、末端の名前の集まりが BASE と AFTER で同じであること
    -   **`fail` と `skip`（AC-04）:** 比べる最上位のテスト関数の下に `fail` の事象がなく、`skip` の事象の `Test` の値の集まりが BASE と AFTER で同じであること
    -   **単独の選択（AC-11）:** 3 つのファイルの最上位のテスト関数ごとの最初と最後の末端と、1.3 節の 8 つが、それぞれ単独で合格すること
    -   **shuffle（AC-11）:** `-shuffle=on` の 2 回が合格し、各パッケージのシードを読めること
    -   **差分の範囲（AC-09）:** 変えたファイルのすべてが、ステップ 6-2 の範囲に入ること
    -   終わった後に、元のブランチに戻っていること
-   **壊し方の確認:** 各判定が少なくとも 1 つの壊し方で失敗することを確かめる。
-   **予定の実行:** 7.3 節と同じ時点と人で実行する。
-   **テスト:** コミットしない。
-   **壊し方の記録:** （`/runplan` が書く）
-   **実行の記録:** （`/runplan` が書く。`diff` の確かめを含むスクリプトなので、出力と 2 つのコミットは 7.2 節に置き、ここからはその場所を指す）

### 7.5. 前提の検査の確かめ（AC-07）

前提の検査の場所は、BASE（コミット `51f82a71`）の行番号で示す。AFTER での場所は設計書 7.4 の表のとおりである。

| 前提の検査 | 崩して確かめるステップ（結果を書くコミット） |
|---|---|
| refs/replace（`high_review_decision_test.go:546`） | ステップ 2-3 |
| 設定の前の出力（`review_records_test.go:817`） | ステップ 3-3 |
| 設定による plain git の変化（`review_records_test.go:826`） | ステップ 3-3 |
| `requireProbe`（`review_records_test.go:125`） | ステップ 4-5 |
| `mainMerge`（`:235`）、`rrSubmodule`（`:324`） | ステップ 4-5 |
| `setup` の中の 7 か所（`:379`・`:447`・`:690`・`:1356`・`:1439`・`:1817`・`:1832`） | ステップ 4-5 |

### 7.6. カバレッジの比較（AC-06）

（`/runplan` が書く。PR ごとに、`make coverage-compare BASE=<BASE>` のすべての実行の出力の最後の部分と、実行したコミット）

### 7.7. テストと lint（AC-12）

（`/runplan` が書く。PR ごとに、実行したコミットと、`make test`・`make lint` の終了コード）

## 8. 成功基準 (Success Criteria)

-   **機能:** このタスクの `cmd/yt2column` と `internal/mergepr` の割合が、どちらも 0.75 以下である（AC-01・AC-02）。
-   **品質:** ケースの集まりが減らず（AC-04）、製品コードのカバレッジが下がらず（AC-06）、13 か所の前提の検査が働き（AC-07）、`make test` と `make lint` が通る（AC-12）。
-   **セキュリティ:** N/A（要件書 4.2）。
-   **文書:** 7 章の記録がそろい、`package_reference.md` に `internal/snapshottest` の行がある。

## 9. 次のステップ (Next Steps)

-   PR-2 のマージの後、issue #1168 を閉じる（`/runpr` が、タスクの最後の計画の PR の後に提案する）。
-   `scriptBashes` を使うほかのテストへの適用は、範囲外として残す（設計書 9 章）。
