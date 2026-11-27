# 実装計画書：review_records・high_review_decision・mergemain のテストの高速化

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-26 |
| Review date | 2026-11-26 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を指し、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。用語は、要件書 6 章と設計書 1.3 のもの（フィクスチャのリポジトリ、末端のサブテスト、実時間、ケースの前提の検査、計測の条件、共有の状態、共通の初期状態、共有する準備、共有する実行の結果、ストア、種別、鍵、作り手、複製、整え直し）を同じ意味で使う。共有の状態の個々の種類の名前（グループのブランチ、設定の前の実行、空の `takeIn`、競合のマージの途中、マージのコミットの後など）は、設計書 3.3・3.4・3.6 のものである。`design_handoff.md` の H-01〜H-05 は、設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。

既存のコードについての記述は、コミット `51f82a71` のファイルで確かめた。設計書が確かめたコミット `95793184` から、`cmd/yt2column`・`internal/mergepr`・`internal/pipeline`・2 つのスクリプト・`package_reference.md` に変更はない（`git diff --stat 95793184 51f82a71` が空）。そのため、設計書の `file:line` はこのコミットでも同じ行を指す。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

本計画は、3 つのファイル（`cmd/yt2column/review_records_test.go`・`cmd/yt2column/high_review_decision_test.go`・`internal/mergepr/mergemain_test.go`）のテストの準備を共有の状態の複製に替え、`cmd/yt2column` と `internal/mergepr` の実時間を、変更の前の 80% 以下にする（F-001）。その際、ケースの集まり、bash 3 での実行、ケースの前提の検査、各ケースの隔離は減らさない（F-002・F-003）。ただし PR-3（フェーズ 8）は、検証で冗長と確かめたケースだけを AC-13 の検証つきで削る。

### 1.2. 実装の原則

-   変えるのはテストのコードだけにする（設計書 1.1、AC-09）。スクリプトと `internal/mergepr` の製品コードは変えない。
-   ケースの表の行・`setup`・`want` は今の場所に残す（要件書 4.5）。変えるのは、表を走らせる関数とフィクスチャを作る関数である。例外は、準備を bash ごとに行う 6 件の行に足す印（ステップ 4-1）である。印は準備の共有の仕組みの一部（設計書 3.4）なので、共有と同じコミットに入れ、共有と一緒に revert される。ほかの行は変えない。
-   共通の初期状態の複製、実行の結果の共有、準備の共有、並列化は、それぞれ revert で切り離せる別のコミットにする（要件書 5 章、設計書 8 章）。複製を入れるコミットでは、「複製した状態は作り直した状態と同じである」という義務をメッセージに書き、それを固定するテストも入れる（CLAUDE.md「Performance」）。
-   ケースの前提の検査（AC-07）は、それが確かめるフィクスチャや実行を変えたコミットで、前提を一時的に崩してケースが失敗することを確かめ、そのコミットのメッセージに書く。

### 1.3. 計画の際に決めたこと（2026-11-26）

人間のレビュアーが決めたもの:

-   **実装の PR は 2 本にする（2026-11-27 に、冗長テストの削減の PR-3 を足して 3 本になった。下の 1.5 節と 1.6 節、フェーズ 8）:** PR-1 は `internal/snapshottest`、`cmd/yt2column` の変更（設計書 3.2〜3.5 と 3.8 の `cmd/yt2column` の分）、`_checks/` の 2 つの一時スクリプトである。PR-2 は `internal/mergepr` の変更（設計書 3.6・3.8 の `internal/mergepr` の分と、届かないときの 3.7）である。理由: 1 本の PR ごとに、計測（BASE と AFTER の 3 回ずつ）、AC-04・AC-11 の比較、`make coverage-compare` で、機械を約 1 時間使う。PR ごとの割合を掛け合わせる（F-001）ので、PR を増やすと計測の誤差も重なる。パッケージの境目で切れば、PR-1 で `cmd/yt2column` の割合がほぼ決まり、PR-2 で `internal/mergepr` の割合が決まる。採らなかった案: 3 本（共通の初期状態だけでは `cmd/yt2column` が 0.75 に届かず、届くかどうかが 2 本目まで分からない）と、1 本（レビューの量が大きい）。ADR: なし（このタスクの中で閉じる進め方の選択である）。
-   **計測は `/runplan` が流す:** 計測と比較の前に、ほかの重い処理を走らせていないことを人に問い（select-question）、答えを得てからバックグラウンドで流す。理由: F-001 は、ほかの重い処理を走らせずに測ることを求める。一方、`chief` は、ほかの workspace にコマンドを並行して送ることがある。人が手で流す案は、PR の前に手作業が入り、自動送信が `hold` になるので採らない。ADR: なし（同上）。
-   **ステップ 7-5 の `Setup` の手段は候補 (a)（`git fast-import`）にする（2026-11-27）:** ステップ 7-4 の 1 回目の計測で `internal/mergepr` の割合が PR-2 単独 0.809（PR-1 の 1.076 との積 0.870 > 0.75）となったため、人が候補 (a) を選んだ。理由: 候補 (b) は同じ入力の `Setup` にしか効かない一方、候補 (a) はユニークな入力を持つすべての `Setup` 呼び出しの git 起動（15 回 → 3 回）を削減でき、短縮効果が大きい。ADR: なし（このタスクのテスト補助関数の実装の選択である）。

計画で決めたもの（人間のレビュアーに諮らず、理由とともに記す）:

-   **`internal/mergepr` の `TestMain` は PR-2 で足す:** 設計書 8 章の 1 は、2 つのパッケージの `TestMain` のストアを最初に挙げる。しかし、PR-1 では `internal/mergepr` のストアを使うコードがないので、`internal/mergepr/main_test.go` は、それを使うフェーズ 7 の最初のステップで、そのフェーズの前提として足す（8 章の 1 を 2 つのパッケージに分けただけで、順序は変えない）。PR-1 の差分が `internal/mergepr` に触れず、PR-1 の計測で `internal/mergepr` の時間が変わる原因が、もう一方のパッケージの影響だけになる（設計書 2.1「2 つのパッケージの相互の影響」）。
-   **一時スクリプトはフェーズ 6 に置く:** 設計書 8 章の 7 は「最初の実装の PR の計測の前に置く」と定める。そこで、PR-1 の計測の直前のフェーズ 6 に置き、`internal/mergepr` のフェーズ（設計書 8 章の 6）を、PR-2 のフェーズ 7 にする。
-   **AC-09 の範囲を、比較のスクリプトの判定で確かめる:** 設計書 7.4 は、`git diff --name-only` の結果を読むとする。要件書 AC-09 は変更の前と後の関係なので、`diff` の確かめにする。手で読む代わりに、比較のスクリプト（`_checks/compare.sh`）の判定の 1 つにする。変えたファイルの数が多く、記録した差分を読む方法（文書の小さな差分のためのもの）は当たらないためである。ファイルの一覧は、`requirements_process.md` §4「Fixed commands」の File list のコマンド（rename の検出を切る）で取る。`internal/pipeline/pipeline_test.go` と `package_reference.md` は、ファイルの単位では範囲に入るかどうかしか判定できない。そのため、AC-09 が認める箇所だけが変わったことは、PR のレビューで 2 つの差分を読んで確かめる。
-   **AC-11 の「最初でも最後でもない 1 つ」:** 共有の状態の種別ごとに、次の末端のサブテストを選ぶ。`compare.sh` には、これらを引数で渡す。ステップ 7-5 で新しい種別ができたときは、そのステップで 1 行を足す。どれも、コミット `51f82a71` で `go test -tags test -count=1 -json -run <各階層を ^...$ で囲んだ式>` を流し、その末端の名前の `pass` の事象があることを確かめた（出力は表の後に置く）。

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

    コミット `51f82a71` で、表の 8 つの名前を実行した。次は、その出力の末端の `pass` の事象のうち、表の名前の行である:

    ```text
    pass TestReviewRecordsCrossCheck/path-bash/fail-closed_context_validation/valid_mapping_table_control
    pass TestHighReviewDecision/values/old_sonnet
    pass TestReviewRecordsCoverage/path-bash/merge_commit_inside_a_seen_range
    pass TestReviewRecordsCoverage/path-bash/git_config/global/core.abbrev
    pass TestReviewRecordsCrossCheck/path-bash/git_config_with_a_merge/repository/diff.relative
    pass TestMergemainProtocolUpdateAndRuleFacts/protocol_line_conflict_resolved_to_main_plus_one
    pass TestMergemainRestoreSafeAndRefusesOtherChanges/other_branch
    pass TestMergemainPushChecksParentsAndMain/merge_in_progress
    ```

    `bin-bash-3` の名前を選ばないのは、`/bin/bash` が 3.x でない機械には、その名前がないためである。競合のマージの途中の 12 件では、名前の順で `MERGE_HEAD_is_another_commit` が最初、`untracked_file` が最後になるので、そのどちらでもない `other_branch` を選んだ。

### 1.6. PR-3 の削減候補表（2026-11-27）

出典: `HEAD 6c60f9f8` の木で `go test -tags test -race -count=1 -json ./...` を macOS で流した、読み取りだけの調査（issue #1168 に付けたメモ）。候補は、スクリプトとコードを読んで得た仮説であり、ミューテーションで確かめていない。ステップ 8-1 で、ブランチのコードに照らして 1 件ずつ検証し、成り立たないものを除く。略称: RR = `.claude/commands/_lib/review-records.sh`、HRD = `.claude/commands/_lib/high-review-decision.sh`、rrt = `cmd/yt2column/review_records_test.go`、hrdt = `cmd/yt2column/high_review_decision_test.go`、hmc = `cmd/yt2column/harness_mapping_cases_test.go`。行番号は調査時のものである。

**Part 1（bash スクリプトのテスト）**

| id | 場所 | 削るもの | 理由 |
|---|---|---|---|
| C1 | `test_helpers_bash.go` の `scriptBashes` | bin-bash-3 の末端のうち、構文網羅の 40 末端以外（既定。全件はオプトイン環境変数で残す） | HRD は bash 4 が要るので RR だけが 2 回走る。残す部分集合が bash 版に依存する構文をすべて踏めば、残りは awk と git で、どちらの bash でも同じ実行ファイルである（macOS のみ、約 685 末端秒） |
| C2（縮小版） | rrt の `checkConfigScenarios`（Unreviewed・CrossCheck の呼び出し） | Unreviewed・CrossCheck の git config シナリオ。Coverage の repository 8 件と global の全シナリオは残す | repository の設定は GIT_OVERRIDES が全 mode の git 呼び出しを守る（RR の `g()`）。ステップ 8-2 で、その mode の `-c` の上書きを外す壊し確認が、Coverage の残す件で失敗することを確かめられたときだけ削る。global は `GIT_CONFIG_GLOBAL` の削除でも出力が変わらず検出できないので、削らない |
| C3 | rrt の Unreviewed が使う Coverage ケースの再利用 | `widen` に届かない 56 件（計画のチェックボックス 29、seen-range の形 15、環境・読み込み 12） | `widen` が効くのは、START が merge-base でも枝のコミットでもないとき（RR 761-769）とマージのコミット（RR 853）だけである |
| C4 | hmc の「in high row」の文法ケース | RR の 10 件。HRD は 1 件を残して 9 件 | パターンの検証は Role で分岐しない（RR 398-418、HRD 223-245） |
| C5 | hrdt の `values/old_*` | `frontier-required`・`frontier-recommended`・`sonnet+opus-review`・`standard`（`opus` と `sonnet` は残す） | 同じ検索ループのデータ行である（HRD 426-431）。`internal/chief` の `markers_parity_test.go` が全値を走らせる |
| C6 | rrt の CrossCheck の path-bash の環境ケース（GIT_ 変数、export した関数、BASH_ENV、origin/main なし、HEAD がコミットを指さない） | 削除（件数は現行コードで数える） | 環境の掃除は mode の分岐の前にある（RR 226-240）。Coverage 側で網羅する。rrt の注記は繰り返しを意図的と書くが、人が削除を決めた |
| C7 | rrt の Unreviewed の、準備だけして `t.Skip` する 6 末端 | 何も検査しないので、`t.Run` の前に除く（`unreviewedOverrides` と同様に、使わない名前の一覧と照らす） | |
| C8 | hmc の `underscore_suffix_rejected`・`space_in_model_id_rejected` | RR と HRD の両方 | 大文字と同じ `SUFFIX_RE` の拒否（RR 503） |
| C9 | hmc の `regex_grouping_metacharacter`・`whitespace_in_pattern` | RR と HRD の両方 | 文字クラスのケースと同じ、リテラルの正規表現の awk の分岐（RR 413） |
| C10 | rrt の FableAlias の path-bash | `MISSING_without_record`・`MISSING_when_rule_2_fix_uses_codex_seen_range`（`VALID` と fail-closed のケースは残す） | 別名は case の 1 つの選択肢である（RR 1062、262） |
| C11 | rrt の Coverage の「another reviewer」 | 削除 | 「codex reviewer」と同じ `SEEN_RE` の不一致 |

C1 の 40 末端の部分集合（63 秒、全件は 748 秒）: Coverage 13（coderabbit が見た、見た範囲の後のコミット、PR 作成済み、PR 1 本のマージ済み、CRLF の計画、略称が枝の名前、LAST_REVIEWED の上限、HEAD と PLAN の別の形、GIT_ 変数、クリーン環境の事前設定、export した関数、BASH_ENV）、Unreviewed 3（見た範囲の中のマージ、マージの前の古い merge-base、README.en.md と CHECKS の修正）、CrossCheck 23（旧 Fable の実行、費用あり・なしの not run、記録の後の README.en.md、記録の後の計画のチェックボックス、not run の行の中の修正、checkpoint の Cross-check の印の後の作業、マージの後のマージの確認、wrong-model と not-opus の not run のマージの確認、CLI が見たマージの確認の修正、merge-base より前の親、ENTRY の別の形、正常な対照、ヘッダの誤り、`_context.md` なし、`_context.md` がシンボリックリンク、high パターンが 0 件、パターンの 6 件）、alias 1（VALID）。これが踏むべき構文は、`env -i` の再実行と `$$` の印、export した関数と `unset -f`、BASH_ENV、`${!GIT_@}`、`set -u` 下の配列、`[[ =~ ]]` と BASH_REMATCH（正規表現ごと）、プロセス置換、`LC_ALL=C` 下の `${x:0:n}` と `${#x}`、case の `$'\n'`、引用したパターンの接頭辞の削り、関数の中の here document、各 mode の `fail` の出力である。ステップ 8-1 で現行の RR から導き直す。

**Part 2（`internal/mergepr/mergemain_test.go`）**

| id | 場所 | 削るもの | 理由 |
|---|---|---|---|
| M1 | `ConflictClassification/markdown_content` | 削除 | `MergeIgnoresConfiguredMergeOptions` と同じ入力と検査 |
| M2 | `ConflictClassification/go_file` | 削除（`test_file` は残す） | `ReadsNoAttributesOutsideTheBaseHead/ignored_.gitattributes_at_merge` と同じ |
| M3 | `ConflictClassification/markdown_with_merge_attribute_unset` | 削除 | `RefusesOtherMergeDrivers/committed_union_driver` と同じ分岐（`mergemain.go:987`） |
| M4 | `ConflictClassification/protocol_line_only` | 先に `ProtocolUpdateAndRuleFacts/protocol_line_conflict_resolved_to_main_plus_one` に `ProtocolOnly` の検査を足してから削除 | 同じ入力 |
| M5 | `markdown_renamed_to_one_path_on_both_sides`・`markdown_renamed_onto_an_addition`（`renamed_and_changed` は残す） | 削除。削除枠の代わりに、main 側だけがファイルをリネームする 1 ケースを足す | `classifyPath` は `renamed[p]` で、ステージの検査より前に返る（`mergemain.go:910`）。main 側だけのリネームを検出するケースが元からなかった |
| M6 | `RestoreSafeAndRefusesOtherChanges/commit_after_the_merge_commit` | 削除 | 同じ検査（`mergemain_restore.go:97-104`）。`swapped_parents` の方が強い |
| M7 | `PushSuccessRejectedAndUnknown/pushed` | 先に 2 つの検査（`NewHead == head`、origin == head）を `PushRejectsNonHeadOrForce/push_command` に移してから削除 | |
| M8 | `PushRejectsNonHeadOrForce/GH_REPO_set`・`second_remote`（`PR_head_branch_is_main` は残す） | 削除 | 同じ `readPR` の分岐を begin から検査している |
| M9 | `RefusesOtherMergeDrivers/push_of_a_union_merge` | 削除 | `push_of_a_union_merge_after_main_drops_the_rule` の方が強い |
| M10 | `ExtCheckHint/development_guide`・`unrelated_file` | 削除 | `README.en.md`・`extensions.md` と同じ `extCheckPaths` の分岐 |
| M11 | `PushChecksProtocolValues/value_changed_to_another_integer` | 削除 | 同じ最終の値のループ（`mergemain_push.go:181-185`） |
| M12 | `BeginPremisesAndNothingToTakeIn/closed_PR`・`tracked_change` | 削除 | 構文の解析は単体で検査済み。`tracked_change` は `untracked_file` と同じ `!clean` の分岐 |
| M13 | `TestMergemainPRViewReadsNarrowFields`・`TestMergemainBeginAcceptsGHDefaultOfOrigin` | begin の ok 副テストへ統合（フィクスチャの再利用が済んでいなければ） | 確信が低いので、8-1 で成り立つか見る |

削らないもの: RR と HRD の両方にある文法ケース（別の解析器の写しである）、Coverage の計画のチェックボックスのケース、reviewer の形の確認、`codex alongside, same commit` と `same branch`、CrossCheck の記録・マージ・checkpoint・not-run の理由のケース、replace ref の各形、HRD のほかの群、`markdown_added_on_both_sides`・`protocol_and_another_line`・`test_file` ほか `mergemain_test.go` の調査の「Not redundant」の欄のもの。

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
-   **手段が決まっていない箇所:** 届かないときに足す `Setup` の作り（設計書 3.7）は、手段が決まっていない。候補は、(a) `Setup` の履歴を `git fast-import` などのより少ないプロセスで作る、(b) `Setup` の入力ごとに状態をストアに置いて複製する、の 2 つで、足すことになったときに、ステップ 7-5 で人に尋ねて選ぶ。ほかのステップは、設計書が手段を決めている。
-   **`Snapshot` と `Kind` を作れる経路:** `Snapshot` と `Kind` は、新しいパッケージ `internal/snapshottest` に置き、欄を公開しない。外のパッケージは `Load` と `NewKind` からしか作れない（設計書 3.1）。パッケージの中で作る経路は、`Load` と `NewKind` だけにする。

### 1.5. 実装の際に決めたこと（2026-11-27）

PR-1・PR-2 の計測（7.1 節）の結果を受けて、人が決めたもの:

-   **`cmd/yt2column` の目標を 80% 以下にする:** 計測の割合は 0.777 で、75% に届かなかった。要件書の「実装の際に決めたこと（2026-11-27）」のとおり、約 0.80 に下げる。`internal/mergepr` は、このときは 75% 以下のままとした（その後、下のとおり 80% 以下に改めた）。
-   **`internal/mergepr` の PR-1 の割合 1.076 は、ぶれとして受け入れる:** PR-1 は `internal/mergepr` を変えておらず、6 回の値が 40.4〜45.9 秒に散らばったため。
-   **比較のスクリプトは、名前の中のコミットの短い ID を置き換えて比べる:** `TestBaseCommitReadsTheRefItself` の末端の名前（`feat-1-g<12 桁の 16 進数>`）が実行ごとに変わり、名前の集まりが同じになりえないため。比べるときだけ置き換え、単独の選択では元の名前を使う。スクリプトを変えるので、7.4 節の壊し方の確認と、PR-1 の比較の実行を、やり直す（計測のスクリプト、テストのコード、製品コードは変わらないので、計測、`make coverage-compare`、`make test`、`make lint` の PR-1 の記録は、AFTER `8bb968e5` のものが、タスクのディレクトリの下だけが異なる今の head でも有効である）。
-   **`internal/mergepr` の目標を 80% 以下にする:** PR-2 のステップ 7-5 後の計測で、`internal/mergepr` は PR-2 単独で 0.737（40.886 秒 → 30.140 秒）と 75% 以下に達したが、PR-1 でぶれとして受け入れた 1.076 との積が 0.793 となり 75% を超えた。要件書の「実装の際に決めたこと（2026-11-27）」のとおり、人が F-001 の積の判定式を維持したまま `internal/mergepr` の目標（AC-02）を 80% 以下に改めることを決めた。
-   **PR-3（冗長テストの削減）を追加する:** PR-2 マージ後の PR-3 として、冗長テスト候補（Part 1: C1〜C11、Part 2: M1〜M13）の検証と削減をフェーズ 8 に追加する。人が決めた方針: C1 は既定を構文・行網羅の代表サブセットとしオプトイン環境変数で全件実行を残す、C6 は削除する、6 件の git config シナリオと global シナリオは維持する、`mergemain_test.go` に main 側のみリネームする 1 ケースを追加する。ADR: なし（このタスクのテスト構成と進め方である）。
-   **PR-3 の bash 行カバレッジ方式と `compare.sh` の PR-3 対応（2026-11-27）:** ステップ 8-1 の現行コード検証を受け、人が次を決めた。(1) `_checks/verify_bash_cut.sh` の行カバレッジ一致は、各 bash でのスクリプト全体の実行行集合（`line`）一致＋`review-records.sh` の全 mode 分岐と `fail()` 分岐への到達とし、C3 は `unreviewed()` 自身の引数検証 3 件（`HEAD of another form`・`PLAN of another form`・`too many arguments`）を残して 53 件削減とし、C1〜C11 を計画どおり実施する。(2) 行記録は bash 3.2 に `BASH_XTRACEFD` がなく `set -x` が `stderr` を汚すため、両スクリプトとも `env -i` 後の `set -T`＋`trap DEBUG`（`TMPDIR` 外のトレースファイルへ追記）とする。(3) PR-3 の AC-09・AC-11 検証は `_checks/compare.sh` に PR-3 用オプション（`CHECK_SKIP_AC04=1`）を足して行い、変更後に `compare.sh` の壊し確認をやり直す。ADR: なし（タスク 0127 の一時スクリプトの検証方式である）。

ADR: なし（このタスクの目標と手順である）。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 複製の仕組みとストア（設計書 3.1・3.9、8 章の 1）

- [x] **ステップ 1-1**: `internal/snapshottest` を作る
    - 追加するファイル: `internal/snapshottest/snapshot.go`（`//go:build test`）
    - 設計書 3.1 の `Store`・`NewStore`・`Close`・`Kind`・`NewKind`・`Snapshot`・`Load`・`CopyTo`・`Value` を作る。作った結果の記録（作った・失敗・skip、記録の前の終わりは失敗）、鍵の長さを付けた連結、作った直後のリンクの検査、`os.CopyFS` での複製は、設計書 3.1 のとおりにする
    - 失敗と skip のメッセージは、設計書 4 章の表の形にする
    - 共有の状態を求めるテストは `t.Setenv` を使わないこと、`-count=N` では共有の状態を作り直さないこと（設計書 3.1「環境」「`-count` との関係」）を、`Load` の doc コメントに書く
    - `gosec` が指摘する行（変数のパスのファイル操作）には、その行だけに理由を付けた `//nolint:gosec` を付ける。ファイル全体には付けない（`.golangci.yml` の `_test.go` の除外は、このファイルに当たらない）
- [x] **ステップ 1-2**: `internal/snapshottest` のテストを書く
    - 追加するファイル: `internal/snapshottest/snapshot_test.go`
    - 設計書 7.1 の項目を、次のテストで確かめる
        - `TestLoadBuildsOnce`: 並列のテストから同じ種別と鍵を求めても、作る関数が 1 回だけ走る
        - `TestLoadKeyParts`: 違う鍵の列が同じ鍵にならない
        - `TestLoadPropagatesFailureAndSkip`: テストのバイナリを子のプロセスで走らせ直し、作り手の 5 つの終わり方（`t.Fatal`、`t.Errorf` の後の正常な戻り、`t.Skip`、`t.Errorf` の後の `t.Skip`、結果の記録の前の終わり）で、後から来たケースが失敗・失敗・skip・失敗・失敗になることを確かめる。skip のほかは、子のプロセスが 0 でない終了コードで終わることも確かめる
        - `TestValueIsCloned`: `Value` を書き換えても、後から来たケースの値は変わらない
        - `TestCopyTo`: 実行の権限とリンクが写る。1 つの複製を書き換えてから複製し直しても、もとの中身が得られる。複製先がすでにあれば失敗する
        - `TestLoadPropagatesFailureAndSkip` の行（`TestChildScenario` の子のプロセスの中）: 絶対パスを指すリンクと、`../` で外を指す相対パスのリンクの、どちらも「失敗」になる。別のテストにはしていない
        - `TestCloseRemovesBuiltDirectories`: ストアを閉じると、作った後のディレクトリが消える。`TestLoadNilInterfaceValue`: 値が nil のインターフェースでも `Load` が働く
    - 上のテストのそれぞれについて、確かめる仕組みを 1 つ一時的に壊し、そのテストが失敗することを確かめ、コミットのメッセージに書く。`TestLoadPropagatesFailureAndSkip` では、失敗と skip を見る順序の入れ替えも壊し方に含める
- [x] **ステップ 1-3**: 新しいパッケージを登録する
    - 変えるファイル: `internal/pipeline/pipeline_test.go`（`testOnlyPackageDirs` に `internal/snapshottest` を足す）、`docs/dev/conventions/package_reference.md`（`internal/snapshottest` の行）
    - `TestFakesCarryBuildTag` の `testutil` のファイルの数（22）は変えない（設計書 3.9）
- [x] **ステップ 1-4**: `cmd/yt2column` のストアを作る
    - 変えるファイル: `cmd/yt2column/main_test.go`、`cmd/yt2column/test_helpers_gitfixture.go`（ストアの変数）
    - 子のプロセスの分岐（`main_test.go:23-25`）の後で `os.MkdirTemp` の場所に `NewStore` を作る。`os.Exit(m.Run())` を、`m.Run` の終了コードを取り、`Close` を呼び、その後で `os.Exit` する形にする（設計書 3.1「寿命」）。`MkdirTemp` か `Close` が失敗したときは、標準エラー出力に書き、0 でない終了コードで終わる
- 完了の条件: `go test -tags test -race ./internal/snapshottest ./internal/pipeline ./cmd/yt2column`、`make lint` が通る。ステップ 1-1〜1-3 と 1-4 を別のコミットにする

### フェーズ 2: `cmd/yt2column` の共通の初期状態（設計書 3.2・3.8、8 章の 2）

- [x] **ステップ 2-1**: 2 つのフィクスチャの共通の初期状態を複製にする
    - 変えるファイル: `cmd/yt2column/test_helpers_gitfixture.go`、`review_records_test.go`、`high_review_decision_test.go`
    - `newGitFixture` に、リポジトリ・`HOME`・`TMPDIR` を作る場所を受け取る形を足す。今の形はそれを `t.TempDir()` で呼ぶ
    - `newRRFixtureWithContext` と `newHRDFixture` を、設計書 3.2 の種別と鍵で `Load` し、ケースの一時ディレクトリに複製する形にする。作る関数は今の手順をそのまま使う。`m0`・`mb` は値で受け取る。ケースの `HOME`・`TMPDIR` は今と同じく `t.TempDir()` で作る（AC-10）
    - 作る関数は、設計書 3.2 のとおり、`dir` の下に自分の `HOME`・`TMPDIR` とリポジトリを作る。`CopyTo` はディレクトリ全体を写すので、ケースの側には空の `HOME`・`TMPDIR` も写るが、ケースはそれを使わず、写したリポジトリだけを使う。作り手のケースも、共有の状態のディレクトリでは作業せず、ほかのケースと同じく複製してから使う
    - 複製の後に index を読み直さない（設計書 3.2）
- [x] **ステップ 2-2**: 複製と作り直しを比べるテストを書く
    - 追加するファイル: `cmd/yt2column/fixture_snapshot_test.go`（`TestFixtureSnapshotsMatchRebuilt`）
    - `rrFixture`（`_context.md` があるものとないもの）と `hrdFixture` の共通の初期状態について、設計書 3.8「比べるもの」を、複製と今の手順で作り直した状態とで比べる。フェーズ 3・4 で、比べる対象を足す
    - 複製の側に作り手のパスを書き足す、ref を 1 つ消す、の 2 つの壊し方で、このテストが失敗することを確かめ、コミットのメッセージに書く
- [x] **ステップ 2-3**: refs/replace の前提の検査（`high_review_decision_test.go:546`）と、共通の初期状態の失敗を確かめる
    - 前提を一時的に崩し、そのケースが失敗することを見る（AC-07）
    - `rrFixture` と `hrdFixture` の共通の初期状態を作る関数をそれぞれ一時的に失敗させ、それに依るケースが合格や skip にならず、`go test` が失敗することを見る（AC-08）
    - 結果をコミットのメッセージに書く
- 完了の条件: `go test -tags test -race ./cmd/yt2column`、`make lint` が通る。ステップ 2-1〜2-3 を 1 つのコミットにし、そのメッセージに複製の義務（複製した状態は作り直した状態と同じである）と、それを固定するテストを書く

### フェーズ 3: git の設定のシナリオの共有（設計書 3.3、8 章の 3）

- [x] **ステップ 3-1**: グループのブランチと設定の前の実行を共有する
    - 変えるファイル: `cmd/yt2column/review_records_test.go`（`checkConfigScenarios` とその 4 つの呼び出し）
    - 設計書 3.3 のとおり、グループのブランチ（鍵: 最上位のテスト関数の名前・グループの名前・モード）と、設定の前の実行（鍵: グループの鍵と bash の名前）を種別にする。設定の前の実行を作る関数が、終了コードと出力の先頭（`base`）を検査する
    - 設定の前の実行の値に、走らせた bash のパスを持たせる。各シナリオは、それが自分の `bash.path` と同じでなければ失敗する（AC-05）。設定の前の実行の鍵から bash を外す壊し方で、`/bin/bash` 3.x の機械でシナリオが失敗することを確かめ、コミットのメッセージに書く
    - グループのブランチは、bash ごとの親のサブテストが、シナリオのサブテストを始める前に求める。各シナリオは、ブランチを複製し、plain git の出力が設定で変わることを確かめ、自分の bash で走らせた結果を、その bash の設定の前の実行と比べる
    - 末端の名前（`<bash>/git config/<scope>/<設定の名前>`）は変えない（AC-04）
- [x] **ステップ 3-2**: `TestFixtureSnapshotsMatchRebuilt` に、グループのブランチの 1 つ（`rrConfigBranch`）を足す
- [x] **ステップ 3-3**: 2 つの前提の検査（設計書 7.4 の「設定の前の出力」と「設定による plain git の変化」）と AC-08 を確かめる
    - 設定の前の出力の前提を崩すと、その bash のそのグループのシナリオがすべて失敗し、`go test` が失敗することを見る（AC-07・AC-08）
    - 設定による plain git の変化の前提を崩すと、そのシナリオが失敗することを見る
    - グループのブランチを作る関数を失敗させると、それを求めた親のサブテストが両方の bash で失敗し、`go test` が失敗することを見る（AC-08）。グループのブランチの中で呼ぶ `mainMerge`（`rrUnreviewedConfigBranch`・`rrFableMergeConfigBranch`）の前提の検査も、ここで親の失敗として確かめる（AC-07）
    - 結果をコミットのメッセージに書く
- 完了の条件: `go test -tags test -race ./cmd/yt2column`、`make lint` が通る。フェーズ 3 を 1 つのコミットにし、そのメッセージに共有の義務（設定の前の実行と各シナリオは同じ共有の状態の複製の上で走る）を書く

### フェーズ 4: ケースの準備の、2 本の bash の間での共有（設計書 3.4、8 章の 4）

- [x] **ステップ 4-1**: `rrCase` に準備の仕方の欄を足す
    - 変えるファイル: `cmd/yt2column/review_records_test.go`
    - 値は「共有する」（0 値）と「bash ごとに行う」の 2 つにする（設計書 3.4）。1.4 節の 6 件にだけ「bash ごとに行う」を書く
- [x] **ステップ 4-2**: `runRRCases` で準備を共有する
    - 変えるファイル: `cmd/yt2column/review_records_test.go`
    - ケースを走らせる前に、ケースの名前の重なりを検査する
    - 「共有する」のケースは、設計書 3.4 の鍵（最上位のテスト関数の名前・モード・ケースの名前・`_context.md` の中身）で準備を `Load` し、複製した自分のリポジトリで、自分の bash でスクリプトを走らせる。`env` と `want` は自分のフィクスチャで作る。「bash ごとに行う」のケースは、今の手順のままにする
    - 準備を作るフィクスチャには、ケースと同じ `form`・`forms` を渡し、`bash` は 0 値のままにする
- [x] **ステップ 4-3**: 共有する準備の中でスクリプトを走らせない検査を足す
    - 変えるファイル: `cmd/yt2column/test_helpers_gitfixture.go`、`review_records_test.go`
    - 共有する準備（`runRRCases` のケースの準備と、git の設定のシナリオのグループのブランチ）を作るフィクスチャ（`sibling` が作るものを含む）に印を付け、印のあるフィクスチャの `runScript` でケースを失敗させる（設計書 3.4、4 章）。共通の初期状態と、設定の前の実行（スクリプトを走らせるのが目的である）を作るフィクスチャには付けない
- [x] **ステップ 4-4**: `TestFixtureSnapshotsMatchRebuilt` に、共有する準備の代表の 1 件を足す
- [x] **ステップ 4-5**: 前提の検査と、共有の失敗を確かめる
    - 次の 10 か所のそれぞれで前提を崩し、それに依るケースが `path-bash` と `bin-bash-3` の両方で失敗することを見る: `requireProbe`（`review_records_test.go:125`）、`mainMerge`（`:235`。ケースの `setup` での呼び出し）、`rrSubmodule`（`:324`）、`setup` の中の 7 か所（`:379`・`:447`・`:690`・`:1356`・`:1439`・`:1817`・`:1832`）（AC-07）
    - 1.4 節の 6 件の 1 つから「bash ごとに行う」を外すとケースが失敗すること、ケースの名前を 1 つ重ねるとテスト関数が失敗すること、共有する準備を作る関数を失敗させると `go test` が失敗することを見る（AC-08）
    - 結果をコミットのメッセージに書く
- 完了の条件: `go test -tags test -race ./cmd/yt2column`、`make lint` が通る。フェーズ 4 を 1 つのコミットにし、そのメッセージに共有の義務（共有する準備は git だけで作り、どちらの bash のケースが作り手になっても同じ状態になる）を書く

### フェーズ 5: 最上位のテスト関数の並列化（設計書 3.5、8 章の 5）

- [x] **ステップ 5-1**: 5 つの最上位のテスト関数で `t.Parallel()` を呼ぶ
    - 変えるファイル: `cmd/yt2column/review_records_test.go`（1.4 節の 5 つの関数）
    - どれも `t.Setenv` と作業ディレクトリの変更を使わないことを、変える前に読んで確かめる（設計書 3.5）
- 完了の条件: `go test -tags test -race ./cmd/yt2column`、`make lint` が通る。フェーズ 5 を 1 つのコミットにする

### フェーズ 6: 計測と比較の一時スクリプトと PR-1 の記録（設計書 7.5、8 章の 7）

- [x] **ステップ 6-1**: 計測のスクリプトを書く
    - 追加するファイル: `docs/tasks/0127_git_fixture_test_speedup/_checks/measure.sh`
    - 設計書 7.5「計測のスクリプト」のとおりにする（BASE・AFTER の交互の 6 回、`ok` の行の読み取り、パッケージごとの中央値と割合、機械の情報、元のブランチへの戻し）。言語は bash で、`jq` を使ってよい
- [x] **ステップ 6-2**: 比較のスクリプトを書く
    - 追加するファイル: `docs/tasks/0127_git_fixture_test_speedup/_checks/compare.sh`
    - 設計書 7.5「比較のスクリプト」のとおりにする（AC-04 の名前・`fail`・`skip` の集まり、AC-11 の単独の選択と `-shuffle=on` の 2 回）。どの `go test` の実行も、終了コードが 0 で、2 つのパッケージの `pass` の事象があるときだけ通ったとする
    - 単独の選択の最上位のテスト関数は、AFTER の 3 つのファイルの `func Test` から取る。中間の名前は、1.3 節の表の名前を引数で受け取り、BASE の名前の集まりにあり、それを使う末端の中で最初でも最後でもないことを確かめる。選択は 1 つずつ別の `go test -count=1` で流し、選んだ末端の名前の `pass` の事象がなければ失敗とする（何も選ばれずに終了コードが 0 になる実行を、合格としない）
    - `/bin/bash` が 3.x の機械で、BASE の 4 つの関数（要件書 AC-05）の下に `bin-bash-3` の末端がなければ失敗とする
    - AC-04 の名前の比べ方: 末端の名前の中の実行ごとに変わるコミットの短い ID（`-g` と 12 桁の 16 進数）を、比べるときだけ同じ記号に置き換える。単独の選択では元の名前を使う（1.5 節）
    - AC-09 の判定を足す: `requirements_process.md` §4「Fixed commands」の File list のコマンドで BASE と AFTER の間のファイルの一覧を取り、全体を出力する。各ファイルが、要件書 AC-09 の範囲（`cmd/yt2column/`・`internal/mergepr/` の `_test.go` のファイルと最初の行が `//go:build test` のファイル、`_test.go` でないファイルの最初の行がすべて `//go:build test` である `internal/snapshottest/`、`internal/pipeline/pipeline_test.go`、`docs/dev/conventions/package_reference.md`、`docs/tasks/0127_git_fixture_test_speedup/` の下）に入ること。消したファイルは BASE の内容で読み、読めないときは通ったとしない
- [x] **ステップ 6-3**: 2 つのスクリプトの壊し方を確かめる
    - スクリプトをコミットしてから、各判定が少なくとも 1 つの壊し方で失敗することを確かめ、7.3 節・7.4 節に書く。作業ツリーの検査に先に当たらないように、壊し方は、作業ツリーの外に写したスクリプトに入れるか、比べるコミットを替えて与える。失敗したのがその判定であることを、出力で確かめる
    - 壊し方の確認の時間を抑えるため、2 つのスクリプトは、対象のパッケージ・`-run`・回数を狭める引数を受け取ってよい。狭めた実行は計測として記録しない
- [x] **ステップ 6-4**: PR-1 の計測と記録
    - `git fetch origin main` の後、BASE を `git merge-base origin/main HEAD`、AFTER を `HEAD` にする
    - ほかの重い処理を走らせておらず、計測の間 `chief` の見張り（ほかの workspace への自動送信）を止めていることを人に問い（select-question）、答えを 7.1 節に書く
    - `measure.sh`、`compare.sh`、`make coverage-compare BASE=<BASE>`、`make test`、`make lint` を、この順に 1 つずつバックグラウンドで流す。前の実行が終わったという通知が来るまで、次の実行を始めず、ほかの重い処理も走らせず、作業ツリーも変えない（2 つのスクリプトは作業ツリーで BASE と AFTER を取り出し、計測は重い処理がないことを前提とするため）。記録は、すべての実行が終わってから 7.1・7.2・7.6・7.7 節に書く。`coverage-compare` が時間で揺れる関数で `FAIL` になったときは、きれいな作業ツリーで流し直し、すべての実行を書く（AC-06）
    - `cmd/yt2column` の割合が 0.80 を超えたときは、作業を止め、要件書の決定の変更をレビューに諮る（設計書 2.1「届かないとき」）
    - `internal/mergepr` の PR-1 の割合も 7.1 節に書く。1.0 を超えたときは（PR-1 の計測では 1.076 だったが、1.5 節のとおり、人がぶれとして受け入れた）、PR-2 の見込み（0.74〜0.76）を掛けると AC-02 に届かないおそれがあるので、PR を作る前に作業を止めて人に知らせる
- 完了の条件: 7.1〜7.4・7.6・7.7 節の PR-1 の記録がある。記録のコミットは、AFTER とはタスクのディレクトリの下だけが異なる

記録の後に、PR を開く前のレビュー（ローカルの CLI レビュー、上位モデルのレビュー）の直しで、タスクのディレクトリの外のファイルか 2 つのスクリプトが変わったときは、その直しのコミットを AFTER にして、ステップ 6-4 をやり直す。7 章の記録は、PR を作る前の最後の実行のものにする。上位モデルのレビューが直しを入れるのはよくあることなので、PR ごとに 2 回の計測（約 2 時間）を見込む（5 章）。

### PR-1 作成ポイント: shared fixture snapshots for the review records and high-review decision tests

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 5-1 / 6-1 / 6-2 / 6-3 / 6-4

**推奨タイトル**: `test(0127): review_records と high_review_decision のテストの準備を複製で共有して速くする`

**レビュー観点**: フェーズごとのコミットを単位に読む。`Load` が作り手の失敗と skip を後から来たケースに伝え、作りかけの状態を渡さないこと（設計書 3.1） / 各ケースのスクリプトの実行がそのケースの bash で行われ（ステップ 3-1 の bash のパスの検査、ステップ 4-2 の準備のフィクスチャの `bash` が 0 値）、共有する準備の中でスクリプトが走ればケースが失敗すること（ステップ 4-3） / ケースの表の行・`setup`・`want` が変わらず（要件書 4.5）、並列にした 5 つの関数が `t.Setenv` と作業ディレクトリの変更を使わないこと（ステップ 5-1） / 7.1 節の `cmd/yt2column` の割合と `internal/mergepr` の PR-1 の割合、7.2 節の名前の集まりと差分の範囲

**実装モデル要件**: standard+high-review

**判定理由**: ステップ 1-1 の `Load` は、作り手の goroutine での作成と、失敗・skip の伝達を扱う並行の処理であり、PR は mkplan の条件つきの確認のうち 3 つ（`t.TempDir` の外に置く共有の状態、`test` のビルドタグだけでコンパイルする新しいソース、`//nolint:gosec` の範囲）に当たる。手段は設計書 3.1〜3.5 がすべて決めており、ケースの抜けは `compare.sh` が、設定の前の実行の bash の取り違えはステップ 3-1 の検査が見つけるので、`high`（前例のない設計の判断や、mkplan の panel のレビューの条件に当たる手順）にはしない。`high` 役割より軽いモデルが実装すると、`bin-bash-3` の名前のケースを `PATH` の bash の結果で合格させるか（AC-05）、共有する準備の中で probe を 1 本の bash でだけ走らせるか（ステップ 4-3）、共有の側に移した前提の検査を働かなくしてケースに何も確かめさせないおそれがある（AC-07）

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1193
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 7: `internal/mergepr` の共通の初期状態と準備の共有（設計書 3.6〜3.8、8 章の 6）

- [x] **ステップ 7-1**: `internal/mergepr` のストアを作る
    - 追加・変えるファイル: `internal/mergepr/main_test.go`（新規、`//go:build test`、`TestMain`）、`internal/mergepr/test_helpers_git.go`（ストアの変数）
    - ストアの作り方と消し方は、ステップ 1-4 と同じにする。このステップを 1 つのコミットにする
- [x] **ステップ 7-2**: 空の `takeIn` を共通の初期状態にする
    - 変えるファイル: `internal/mergepr/test_helpers_git.go`
    - `newTakeIn` の今の手順を作る関数にする。共有の状態のディレクトリには `origin.git` と `work` だけを置き、作る関数の `home` は作り手の `t.TempDir()` に作る。ケース（作り手を含む）は、それを自分の一時ディレクトリに複製し、自分の `home` と `env` を作り、`remote.origin.url` を複製した origin に書き換える（設計書 3.6「整え直し」）。`PR`・`Env`・`GHCalls` は今と同じ既定の値から始める
    - 追加するファイル: `internal/mergepr/takein_snapshot_test.go`（`TestTakeInSnapshotsMatchRebuilt`）。空の `takeIn` について、設計書 3.8「比べるもの」を比べる。複製の側は、ケースが使うのと同じ複製と整え直しの関数で作る。origin のパスは、比べるそれぞれの状態の自分の origin のパスで置き換える
    - `remote.origin.url` の書き換えを外すと `TestTakeInSnapshotsMatchRebuilt` が失敗すること、空の `takeIn` を作る関数を失敗させると `go test` が失敗すること（AC-08）を確かめ、コミットのメッセージに書く
    - このステップを 1 つのコミットにし、そのメッセージに複製の義務を書く
- [x] **ステップ 7-3**: 競合のマージの途中とマージのコミットの後を共有する
    - 変えるファイル: `internal/mergepr/mergemain_test.go`、`test_helpers_git.go`
    - 設計書 3.6 の表の 2 つの共有する準備を作る。値（base head、main、begin の結果、merge の結果）は種別の `clone` で写す。作る関数は、準備の merge のエラーが nil であることを確かめる。1.4 節の 12 件と 22 件を、それを複製する形に替え、2 件（`:1071`・`:1240`）は今のままにする
    - 整え直しに、index の stat の情報の読み直し（unmerged の項目を残す）を足す
    - `TestTakeInSnapshotsMatchRebuilt` に、2 つの共有する準備を足す（`git diff-files` の比較を含む）。複製の側では、`git diff-files` より前に index を書く git（`GIT_OPTIONAL_LOCKS=0` のない `git status` など）を走らせない
    - index の読み直しを外すと `TestTakeInSnapshotsMatchRebuilt` が失敗すること、共有する準備を作る関数を失敗させると `go test` が失敗すること（AC-08）を確かめ、コミットのメッセージに書く
    - このステップを 1 つのコミットにし、そのメッセージに整え直しの義務を書く
- [x] **ステップ 7-4**: PR-2 の計測と記録
    - ステップ 6-4 と同じ手順で流し、7.1・7.2・7.6・7.7 節に PR-2 の記録を書く
    - このタスクの割合（PR-1 と PR-2 の割合の積）を、パッケージごとに 7.1 節に書く
    - `cmd/yt2column` のこのタスクの割合が 0.80 を超えたときは、作業を止め、要件書の決定の変更をレビューに諮る（設計書 2.1）
    - `internal/mergepr` のこのタスクの割合が 0.75 を超えたときは、ステップ 7-5 に進む。超えないときは、ステップ 7-5 に「足さなかった」と書いて印を付ける
- [x] **ステップ 7-5（条件つき）**: `Setup` の git の起動を減らす（設計書 3.7）
    - 変えるファイル: `internal/mergepr/test_helpers_git.go`、`takein_snapshot_test.go`
    - 1.4 節の 2 つの候補は、守る義務が違う（(a) はすべての `Setup` の呼び出しで同じ状態・コミットの順序・親子を保つ。(b) は新しい種別と AC-11 の行を足す）。そのため、手段は `/runplan` が決めず、計測の値と 2 つの候補を示して人に問い（select-question）、答えを 1.3 節に書く。`Setup` の後の状態とコミットの順序と親子を今と同じにする
    - 今の `Setup` の手順を、比べるもとの手順として `takein_snapshot_test.go` に残し、`TestTakeInSnapshotsMatchRebuilt` で、新しい `Setup` の後の状態をそれと比べる。新しい `Setup` のコミットの順序か親を変える壊し方で、このテストが失敗することを確かめ、コミットのメッセージに書く
    - 候補 (b) を選んで新しい種別ができたときは、それを使う末端のうち最初でも最後でもない 1 つを 1.3 節の表に足し（その時点で `-run` で `pass` を確かめる）、`compare.sh` に渡す
    - このステップを 1 つのコミットにし、ステップ 7-4 の計測と記録を、その止まる条件も含めてやり直す。足したことと計測の値を 7.1 節に書く（設計書 3.7「記録」）
    - それでも `cmd/yt2column` のこのタスクの割合が 0.80 を、`internal/mergepr` の割合が 0.80 を（この行を書いたときは 0.75。要件書の再承認で 80% に改めた）超えるときは、作業を止め、要件書の決定の変更をレビューに諮る（設計書 2.1）
- 完了の条件: `go test -tags test -race ./internal/mergepr`、`make test`、`make lint` が通り、7 章の PR-2 の記録がある

ステップ 6-4 の後の段落と同じく、記録の後にタスクのディレクトリの外のファイルか 2 つのスクリプトが変わったときは、ステップ 7-4 をやり直す。

### PR-2 作成ポイント: shared take-in snapshots for the mergemain tests

**対象ステップ**: 7-1 / 7-2 / 7-3 / 7-4 / 7-5

**推奨タイトル**: `test(0127): mergemain のテストの準備を複製で共有して速くする`

**レビュー観点**: 複製の後の整え直し（`remote.origin.url` と index）で、ケースが作り手の origin や古い stat の情報を読まないこと（設計書 3.6） / 共有しない 2 件と、`GHCalls` を確かめるテストが今のままであること / ステップ 7-5 を足したかどうかと、その理由と計測の値 / 7 章の記録と、2 つの PR の割合の積が AC-01・AC-02 を満たすこと

**実装モデル要件**: standard+high-review

**判定理由**: ステップ 7-5 は計測の結果しだいで足す手段で、1.4 節に 2 つの候補があり、ステップ 7-2・7-3 は複製した git の状態の整え直しを扱う。`high` 役割より軽いモデルが実装すると、整え直しが欠けたまま復元のケースを通すか、ケースに作り手の origin へ push させるか、`Setup` を作り直して状態を変え、比較のテストを弱めるおそれがある

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1197
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 8: 冗長テストの検証と削減（Part 1・Part 2）

- [x] **ステップ 8-1**: 削減候補（1.6 節の C1〜C11・M1〜M13）の仮説検証と bash 3 サブセットの再導出
    - 現在のブランチ上の `review-records.sh`・`high-review-decision.sh`・`internal/mergepr` とテストコードに照らし、各候補が成り立つかを検証して成り立たない候補を除外する（C3 は `unreviewed()` 自身の引数検証 3 件を残して 53 件削減、C6 は現行 5 件、M13 は `TestMergemainBeginPremisesAndNothingToTakeIn/ok` への統合で成立）
    - `review-records.sh` の現在のコードから、bash 3.2 依存構文と実行可能 bash 行をすべて踏む代表サブセットを再導出する
- [x] **ステップ 8-2**: bash テスト削減の検証用一時スクリプトを作成し壊し確認する
    - 追加・変更するファイル: `docs/tasks/0127_git_fixture_test_speedup/_checks/verify_bash_cut.sh`（新規）、`docs/tasks/0127_git_fixture_test_speedup/_checks/compare.sh`（PR-3 用オプション `CHECK_SKIP_AC04=1` 追加）
    - `verify_bash_cut.sh` の機能: (1) 一時コピーした `review-records.sh` と `high-review-decision.sh`（`env -i` 後の `unset *_CLEAN_ENV` 行に `set -T`＋`trap DEBUG` を 1 行で追記し、`TMPDIR` 外のトレースファイルへ記録して行番号と `stderr` を変えない）で、各 bash（`high-review-decision.sh` は bash 4 以上だけ）における削減前後のスクリプト全体の実行行集合（`line`）が一致し、`review-records.sh` の全 mode 分岐と `fail()` 分岐に到達することを確認、(2) 残す代表ケースに対するミューテーション（`-c i18n.logOutputEncoding` 削除、`widen` 変更、`fable` 削除）が失敗することを確認、(3) `YT2COLUMN_TEST_BASH3_ALL=1` で `bin-bash-3` の末端集合が `path-bash` と一致すること（AC-13 (3)）を確認する。`GIT_CONFIG_GLOBAL` の削除は、`-c i18n.logOutputEncoding=UTF-8` が global より優先して出力が変わらず検出できないので、ミューテーションに含めない。行カバレッジは設定シナリオを見分けられないので、設定シナリオの削除は (2) の壊し確認でだけ確かめる
    - 7.8 節にこの一時スクリプトの記録を置き、壊し確認を行って記録する。`compare.sh` の変更後、7.4 節の壊し確認もやり直す
- [x] **ステップ 8-3**: Part 1（`review_records_test.go`・`high_review_decision_test.go`・`harness_mapping_cases_test.go`・`test_helpers_bash.go`）の検証済み冗長ケースを削減する
    - C1: 既定は構文・行網羅の代表サブセットとし、オプトイン環境変数 `YT2COLUMN_TEST_BASH3_ALL=1` で bash 3 全件実行を残す
    - C2（縮小版）: Coverage の repository シナリオと global の全シナリオ（および将来の porcelain 移行に備える 6 件の git config シナリオ）を残し、Unreviewed・CrossCheck の重複シナリオだけを、ステップ 8-2 の壊し確認で成り立つと確かめられたときに削除する（成り立たなければ削らない）
    - C6: 削除する（現行 5 件: `HEAD names no commit`・`no origin/main`・`GIT_ variables`・`exported functions`・`BASH_ENV`。env クリーンアップと `load_branch` は Coverage 側で網羅）
    - C3（53 件、引数検証 3 件は維持）・C4・C5・C7〜C11: ステップ 8-1 で検証が成り立ったものを削除・整理する
    - 削除のまとまり（C1〜C11 の候補ごと）を、削除だけのコミットにし、各コミットの後に `make coverage-compare BASE=<そのコミットの親>`（10 分以上のタイムアウト）を流して、最後の行 `coverage-compare: PASS` までをコミットメッセージと PR の説明に書く（Go の製品コードは変わらないので、これは削除が Go のカバレッジを下げないことの確認にとどまる）
    - bash スクリプトの行の網羅は `coverage-compare` で見えないので、`verify_bash_cut.sh` を実行して結果を 7.8 節に記録する（`high_review_decision_test.go` と `harness_mapping_cases_test.go` の削除は、`high-review-decision.sh` の実行行集合の一致で確かめる）
- [x] **ステップ 8-4**: Part 2（`internal/mergepr/mergemain_test.go`）の検証済み冗長ケースを削減し、main 側リネームの 1 ケースを追加する
    - M4・M7・M13 はアサーション移動を別コミットにした上で、各テスト削除は削除のみのコミットにし、`make coverage-compare BASE=<parent>`（10 分以上のタイムアウト）の通過出力と指定の壊し確認（M5/M6/M9/M10）をコミットメッセージと PR 説明に記録する
    - M5 の削除枠の代わりに、main 側のみファイルリネームする 1 ケースを追加する
- [x] **ステップ 8-5**: PR-3 の計測とグリーンゲート確認・記録
    - `measure.sh` で PR-3 の BASE と AFTER を測り、PR-3 の割合と、PR-1・PR-2・PR-3 の積（このタスクの割合）を 7.1 節に記録する。判定は AC-01・AC-02（`cmd/yt2column` は `0.758 × r ≤ 0.80`、すなわち PR-3 の割合 r が約 1.055 以下、`internal/mergepr` は `0.793 × r ≤ 0.80`、すなわち r が約 1.008 以下）で行う。超えたときは、きれいな作業ツリーで測り直し、それでも超えるときは作業を止め、要件書の決定の変更をレビューに諮る（PR-2 のノイズ 1.076 と 0.826 / 0.976 のように、変えていないパッケージの割合が 1 を超えうるので、すべての測定値を記録する）。PR-1・PR-2 の時点の「AC-01・AC-02 達成」は、PR-3 の後の再判定まで暫定とする
    - AC-06（`make coverage-compare BASE=<PR-3 の BASE>`）、AC-09（変更範囲）、AC-11（単独選択とシャッフル。代表は、削除の後に残るケースから選び直し、`CHECK_SKIP_AC04=1` を付けて `compare.sh` に渡す）、`make test`・`make lint`・`make deadcode` を PR-3 にも実行し、結果を 7 章に記録する。AC-04・AC-05 は、PR-3 では AC-13 に置き換える

### PR-3 作成ポイント: remove verified redundant test cases in review_records, high_review_decision, and mergemain

**対象ステップ**: 8-1 / 8-2 / 8-3 / 8-4 / 8-5

**推奨タイトル**: `test(0127): review_records・high_review_decision・mergemain の冗長なテストケースを検証の上で削減する`

**レビュー観点**: 各削除候補（1.6 節の C1〜C11・M1〜M13）の仮説検証の妥当性と、bash 3.2 の構文・行網羅サブセット＋オプトイン全件実行の網羅性 / `_checks/verify_bash_cut.sh` の実行行集合一致・全 mode 分岐到達（`review-records.sh` と `high-review-decision.sh`）とミューテーション検出および壊し確認（`GIT_CONFIG_GLOBAL` 削除は検出できないので含めない） / `mergemain_test.go` の各削除コミットが削除のみで `make coverage-compare BASE=<parent>` の通過出力と M5/M6/M9/M10 の壊し確認を持つこと / main 側のみファイルリネームする新規ケースの妥当性 / 7 章の計測・検証記録

**実装モデル要件**: high

**判定理由**: テストの削除は検証の抜けを招くリスクが高く、現行の `review-records.sh`・`high-review-decision.sh`・`internal/mergepr` を精読して各削減候補の仮説と bash 3.2 依存構文の網羅性を自力で再検証する判断、`CLAUDE.md` Testing Strategy に従った削除単位コミットと `coverage-compare`・壊し確認、および `_checks/` の行カバレッジ・ミューテーション検証スクリプトの設計を要するため

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
| PR-3 | 8-1 / 8-2 / 8-3 / 8-4 / 8-5 | `review_records`・`high_review_decision`・`harness_mapping_cases`・`mergemain` の検証済み冗長ケースの削減（C1 は既定 40 末端＋オプトイン全件、main 側リネーム 1 ケース追加）と行カバレッジ・ミューテーション・カバレッジ比較検証 | high | auto |

## 4. テスト戦略 (Test Strategy)

-   **単体テスト:** `internal/snapshottest` の単体テスト（ステップ 1-2、設計書 7.1）。失敗と skip の伝達は、子のプロセスの出力で確かめる。
-   **複製の義務:** `TestFixtureSnapshotsMatchRebuilt` と `TestTakeInSnapshotsMatchRebuilt` が、複製（`takeIn` では整え直した状態）と作り直した状態を比べる（設計書 3.8）。比べる対象は、それを共有にするフェーズで足す。
-   **結合テスト:** 3 つのファイルのテストそのものが、共有の状態とスクリプト・`mergemain` の結合テストである。ケースの期待する出力は変えない。ケースが減っていないことは、`compare.sh` の名前の集まりの比較で確かめる（AC-04）。
-   **前提の検査と失敗の伝達:** 13 か所の前提の検査（AC-07）と、共有の状態を作れなかったときの失敗（AC-08）は、前提や作る関数を一時的に壊して確かめ、コミットのメッセージに書く（ステップ 2-3・3-3・4-5・7-2・7-3）。
-   **順序と選び方:** 単独の選択と `-shuffle=on` を `compare.sh` で確かめる（AC-11）。
-   **後方互換:** 製品コードとスクリプトは変えない（AC-09）。Linux（CI）では `/bin/bash` の名前のケースがないことは今と同じである（要件書 4.4）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| `cmd/yt2column` の割合が 0.80 に届かない（設計書 2.1 の見込みは 0.65〜0.75。PR-1 の計測は 0.777 だった） | ステップ 6-4 で測り、届かなければ止めて、要件書の決定の変更をレビューに諮る。PR-2 の計測で 2 つの PR の積が 0.80 を超えたときも止める（ステップ 7-4） |
| `internal/mergepr` の割合が 0.80 に届かない（見込みは 0.74〜0.76。PR-2 ステップ 7-5 後は単独 0.737、PR-1 との積 0.793 だった） | ステップ 7-5 で `Setup` の作りを足す。それでも届かなければ止めて諮る |
| 作り手の skip や失敗が伝わらず、ケースが確かめずに合格する | `TestLoadPropagatesFailureAndSkip` と、ステップ 2-3・3-3・4-5・7-2・7-3 の壊し方。PR を開く前に上位モデルのレビューを行う（3.2 節） |
| 複製に作り手のパスが残り、ケースが作り手の状態を読み書きする | 比較のテストが、作り手のパスを含むファイルを調べる（設計書 3.8） |
| 共有する準備の中でスクリプトが走り、probe が 1 本の bash でしか走らない | ステップ 4-3 の検査が、そのケースを失敗させる |
| 計測の途中で止まり、作業ツリーが BASE のまま残る | `measure.sh` と `compare.sh` は、変更のない作業ツリーでだけ始め、どの終わり方でも元のブランチに戻す。壊し方の確認で、戻すことも確かめる |
| ほかの workspace の処理が計測と重なり、割合がぶれる | 計測の前に、重い処理がないことと `chief` の見張りを止めたことを人に問う（ステップ 6-4）。BASE と AFTER を交互に流す（設計書 7.5） |
| PR-1 の並列化で `internal/mergepr` が遅くなり、2 つの PR の積が AC-02 に届かない | ステップ 6-4 で `internal/mergepr` の PR-1 の割合を書き、1.0 を超えたら PR を作る前に止める |
| PR を開く前のレビューの直しで計測をやり直し、PR ごとの時間が延びる | PR ごとに 2 回の計測を見込む。7 章の記録は最後の実行のものにする（フェーズ 6 の終わりの段落） |
| PR を開く前のレビューの直し、PR のレビューの直し、`origin/main` の取り込みで、記録が古くなる | 変えた人が、影響を受ける記録を取り直す（フェーズ 6・7 の終わりの段落、7.3 節・7.4 節の予定の実行） |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 2-1 / 2-2 / 2-3 / 3-1 / 3-2 / 3-3 / 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 5-1 / 6-1 / 6-2 / 6-3 / 6-4）
- [x] PR-2 マージ済み（対象ステップ: 7-1 / 7-2 / 7-3 / 7-4 / 7-5）
- [ ] PR-3 マージ済み（対象ステップ: 8-1 / 8-2 / 8-3 / 8-4 / 8-5）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `test` | `_checks/measure.sh` の PR ごとの計測と、7.1 節のこのタスクの割合（PR-1 と PR-2 の積）。スクリプトの記録は 7.3 節 |
| AC-02 | `test` | 同上（`internal/mergepr`） |
| AC-03 | `test` | `_checks/measure.sh` の出力（機械の情報、BASE と AFTER の ID、6 回の時間、中央値、割合）を 7.1 節に書く |
| AC-03 | `manual` | PR のレビューで、7.1 節が AC-03 の項目をすべて持つことを読む |
| AC-04 | `diff` | `_checks/compare.sh` の「名前の集まり」と「`fail` と `skip`」の判定。出力と 2 つのコミットの ID は 7.2 節に置く |
| AC-05 | `test` | 各シナリオが、設定の前の実行の bash のパスと自分の bash のパスが同じであることを確かめる検査（ステップ 3-1。壊し方はそのコミットのメッセージ） |
| AC-05 | `test` | `_checks/compare.sh` の「名前の集まり」と「bash 3 の末端」の判定で、`path-bash` と `bin-bash-3` の名前の下の末端が BASE と同じく合格すること。機械が `/bin/bash` 3.x の macOS であることは、7.1 節の機械の情報で確かめる |
| AC-05 | `manual` | PR のレビューで、共有する準備のケースが自分の bash でスクリプトを走らせる（ステップ 4-2）ことを読む |
| AC-06 | `test` | PR ごとの `make coverage-compare BASE=<BASE>`。出力は 7.6 節 |
| AC-07 | `test` | 13 か所の前提の検査のそれぞれを崩す確かめ。崩した場所と記録するコミットは 7.5 節 |
| AC-08 | `test` | `internal/snapshottest/snapshot_test.go::TestLoadPropagatesFailureAndSkip`。共有の状態を作る関数を壊して `go test` が失敗することを、ステップ 2-3・3-3・4-5・7-2・7-3 のコミットのメッセージに書く |
| AC-09 | `diff` | `_checks/compare.sh` の「差分の範囲」の判定。出力（ファイルの一覧の全体を含む）と 2 つのコミットの ID は 7.2 節に置く |
| AC-09 | `manual` | PR のレビューで、`internal/pipeline/pipeline_test.go` と `package_reference.md` の差分が AC-09 の認める箇所だけであることを読む（1.3 節） |
| AC-10 | `test` | `cmd/yt2column/fixture_snapshot_test.go::TestFixtureSnapshotsMatchRebuilt`、`internal/mergepr/takein_snapshot_test.go::TestTakeInSnapshotsMatchRebuilt`（複製の設定に作り手のパスがないこと）、`internal/snapshottest/snapshot_test.go::TestCopyTo`（1 つの複製への書き込みが、後の複製に現れないこと） |
| AC-10 | `manual` | PR のレビューで、ケースが自分の `HOME`・`TMPDIR`（`internal/mergepr` では `HOME`・`XDG_CONFIG_HOME`）を作ることを読む（ステップ 2-1・7-2） |
| AC-11 | `test` | `_checks/compare.sh` の「単独の選択」と「shuffle」の判定。結果は 7.2 節 |
| AC-12 | `test` | PR ごとの `make test` と `make lint`。結果は 7.7 節 |
| AC-13 | `test` | PR-3 の削除コミットごとの `make coverage-compare BASE=<parent>`（7.6 節・各コミットメッセージ）、`_checks/verify_bash_cut.sh` の実行行集合一致・mode 到達・ミューテーション検出・`YT2COLUMN_TEST_BASH3_ALL=1` 末端一致（7.8 節）、および `_checks/measure.sh`・`_checks/compare.sh`（7.1・7.2 節） |

### 7.1. 計測の記録（AC-01〜AC-03、AC-05）

（`/runplan` が書く。PR ごとに、計測の前の人の答え、BASE と AFTER の完全な ID、`measure.sh` の出力の全体（機械の情報、6 回のパッケージごとの時間、中央値、割合）。PR-2 の後に、パッケージごとのこのタスクの割合と、ステップ 7-5 を足したかどうか）

**PR-1（2026-11-27、結果: 目標を 80% に改めて通った）**

-   人の答え: 「止めた。始めてよい」（ほかの重い処理を走らせておらず、`chief` の見張りを止めている、という問いへの答え）
-   BASE: `fb71447bb8bd1baa47e113573db52939b41146cd`、AFTER: `8bb968e5c28f44bafd2a3811d190e2fdac2a72ac`（AFTER と、この記録のコミットは、タスクのディレクトリの下だけが異なる）
-   `cmd/yt2column` の割合は 0.777 で、0.75 を超えた。`internal/mergepr` の PR-1 の割合は 1.076 で、1.0 を超えた（PR-1 は `internal/mergepr` を変えていない。BASE と AFTER の中央値は 41.388 秒と 44.518 秒で、6 回の値は 40.4〜45.9 秒に散らばった）。ステップ 6-4 の定めにより、作業を止めて人に知らせた。人は、`cmd/yt2column` の目標を 80% に下げること（0.777 は届く）と、`internal/mergepr` の 1.076 をぶれとして受け入れることを決めた（1.5 節）

`````text
== machine ==
os: Darwin 25.6.0 arm64
cores: 12
path-bash: /opt/homebrew/bin/bash 5.3.15(1)-release
bin-bash: /bin/bash 3.2.57(1)-release
go: go version go1.27.1 darwin/arm64
GOMAXPROCS: unset
BASE: fb71447bb8bd1baa47e113573db52939b41146cd
AFTER: 8bb968e5c28f44bafd2a3811d190e2fdac2a72ac
packages: ./cmd/yt2column ./internal/mergepr
rounds: 3
== runs ==
BASE round 1 ./cmd/yt2column 178.275s
BASE round 1 ./internal/mergepr 41.158s
AFTER round 1 ./cmd/yt2column 138.453s
AFTER round 1 ./internal/mergepr 44.518s
BASE round 2 ./cmd/yt2column 184.690s
BASE round 2 ./internal/mergepr 45.900s
AFTER round 2 ./cmd/yt2column 138.962s
AFTER round 2 ./internal/mergepr 44.706s
BASE round 3 ./cmd/yt2column 165.649s
BASE round 3 ./internal/mergepr 41.388s
AFTER round 3 ./cmd/yt2column 128.060s
AFTER round 3 ./internal/mergepr 40.359s
== medians ==
./cmd/yt2column BASE 178.275s AFTER 138.453s ratio 0.777
./internal/mergepr BASE 41.388s AFTER 44.518s ratio 1.076
measure: DONE
`````

**PR-2（2026-11-27、ステップ 7-5 を足した。結果: 目標を 80% に改めて通った）**

-   **1 回目の計測（ステップ 7-5 の前、BASE `ff1973333e0d08c12108b384a226c110467049cd`、AFTER `ddd9e6034e00873ca4058d5ea06e231195580a3b`）:**
    -   人の答え: 「止めた。始めてよい」
    -   `./cmd/yt2column`: BASE 142.939s → AFTER 118.069s、PR-2 割合 `0.826`（PR-1 との積 `0.777 × 0.826 = 0.642`）
    -   `./internal/mergepr`: BASE 44.343s → AFTER 35.868s、PR-2 割合 `0.809`（PR-1 との積 `1.076 × 0.809 = 0.870 > 0.75`）
    -   `internal/mergepr` の積が 0.75 を超えたため、ステップ 7-5 に進み、人が候補 (a)（`Setup` の `git fast-import` 化）を選んだ（1.3 節）。
-   **2 回目の計測（ステップ 7-5 の後、PR-2 の最終記録）:**
    -   人の答え: 「止めた。始めてよい」
    -   BASE: `ff1973333e0d08c12108b384a226c110467049cd`、AFTER: `cdd92b731ea27f7a3de6e211acade5f9486e1e47`（AFTER と、この記録のコミットは、タスクのディレクトリの下だけが異なる）
    -   ステップ 7-5（設計書 3.7）を足した（候補 (a): `takeIn.Setup` の `git fast-import` 化）。
    -   **このタスクの割合（PR-1 と PR-2 の割合の積）:**
        -   `cmd/yt2column`: `0.777 × 0.976 = 0.758`（≤ 0.80、AC-01 達成）
        -   `internal/mergepr`: `1.076 × 0.737 = 0.793`（PR-2 単独は 0.737 ≤ 0.75 だが PR-1 のぶれ 1.076 との積が 0.793 となったため、人が AC-02 の目標を 80% 以下に改めた。≤ 0.80、AC-02 達成）

`````text
== machine ==
os: Darwin 25.6.0 arm64
cores: 12
path-bash: /opt/homebrew/bin/bash 5.3.15(1)-release
bin-bash: /bin/bash 3.2.57(1)-release
go: go version go1.27.1 darwin/arm64
GOMAXPROCS: unset
BASE: ff1973333e0d08c12108b384a226c110467049cd
AFTER: cdd92b731ea27f7a3de6e211acade5f9486e1e47
packages: ./cmd/yt2column ./internal/mergepr
rounds: 3
== runs ==
BASE round 1 ./cmd/yt2column 135.932s
BASE round 1 ./internal/mergepr 59.452s
AFTER round 1 ./cmd/yt2column 114.164s
AFTER round 1 ./internal/mergepr 32.860s
BASE round 2 ./cmd/yt2column 115.056s
BASE round 2 ./internal/mergepr 40.478s
AFTER round 2 ./cmd/yt2column 113.007s
AFTER round 2 ./internal/mergepr 29.988s
BASE round 3 ./cmd/yt2column 115.761s
BASE round 3 ./internal/mergepr 40.886s
AFTER round 3 ./cmd/yt2column 112.609s
AFTER round 3 ./internal/mergepr 30.140s
== medians ==
./cmd/yt2column BASE 115.761s AFTER 113.007s ratio 0.976
./internal/mergepr BASE 40.886s AFTER 30.140s ratio 0.737
measure: DONE
`````

**PR-3（2026-11-28、結果: 通った）**

-   人の答え: 「他の重いプロセスはなく計測を開始してよい」（ほかの重い処理がなく、`chief` の自動送信も停止した状態であることへの確認）
-   BASE: `0dcdfc0cbeefa68495349c12628a7c6a57232cf5`、AFTER: `abf0ebc69a3b3eb9715d041c0f87b2c1b9332bb1`（AFTER と、この記録のコミットは、タスクのディレクトリの下だけが異なる）
-   **このタスクの割合（PR-1・PR-2・PR-3 の割合の積）:**
    -   `cmd/yt2column`: `0.758 × 0.625 = 0.474`（≤ 0.80、AC-01 達成）
    -   `internal/mergepr`: `0.793 × 0.948 = 0.752`（≤ 0.80、AC-02 達成）

`````text
== machine ==
os: Darwin 25.6.0 arm64
cores: 12
path-bash: /opt/homebrew/bin/bash 5.3.15(1)-release
bin-bash: /bin/bash 3.2.57(1)-release
go: go version go1.27.1 darwin/arm64
GOMAXPROCS: unset
BASE: 0dcdfc0cbeefa68495349c12628a7c6a57232cf5
AFTER: abf0ebc69a3b3eb9715d041c0f87b2c1b9332bb1
packages: ./cmd/yt2column ./internal/mergepr
rounds: 3
== runs ==
BASE round 1 ./cmd/yt2column 125.617s
BASE round 1 ./internal/mergepr 47.029s
AFTER round 1 ./cmd/yt2column 77.984s
AFTER round 1 ./internal/mergepr 43.133s
BASE round 2 ./cmd/yt2column 119.531s
BASE round 2 ./internal/mergepr 43.230s
AFTER round 2 ./cmd/yt2column 74.727s
AFTER round 2 ./internal/mergepr 40.979s
BASE round 3 ./cmd/yt2column 114.496s
BASE round 3 ./internal/mergepr 39.492s
AFTER round 3 ./cmd/yt2column 72.792s
AFTER round 3 ./internal/mergepr 38.710s
== medians ==
./cmd/yt2column BASE 119.531s AFTER 74.727s ratio 0.625
./internal/mergepr BASE 43.230s AFTER 40.979s ratio 0.948
measure: DONE
`````

### 7.2. 比較の記録（AC-04・AC-05・AC-09・AC-11）

（`/runplan` が書く。PR ごとに、BASE と AFTER の完全な ID と、`compare.sh` の出力の全体（判定ごとの結果、`-shuffle` の 2 回の実行の各パッケージのシード））

**PR-1（2026-11-27、結果: 通った）**

-   BASE: `fb71447bb8bd1baa47e113573db52939b41146cd`、AFTER: `89151749534247cac4e2faa858aec890384cb462`（`compare.sh` は、1.5 節の名前の比べ方を足した後のもの）
-   最初の実行（AFTER `8bb968e5c28f44bafd2a3811d190e2fdac2a72ac`、名前の比べ方を足す前）は、`internal/mergepr` の `TestBaseCommitReadsTheRefItself` の末端の名前に実行ごとに変わるコミットの短い ID（`feat-1-g<hash>`）が入るため、`names` の判定だけが失敗した。そのほかの判定は通った。1.5 節の決定でスクリプトを直し、compare.sh を 2 回直したので（1.5 節の名前の比べ方と、クロスチェックの指摘による中間の名前の判定）、下の出力は、最後の直しの後にやり直した実行のものである

`````text
BASE: fb71447bb8bd1baa47e113573db52939b41146cd
AFTER: 89151749534247cac4e2faa858aec890384cb462
packages: ./cmd/yt2column ./internal/mergepr
== full runs ==
full runs: both passed
BASE leaves:     1994, AFTER leaves:     1994, BASE tops:      267
== names, fail and skip (compare) ==
check names: PASS
check fail: PASS
check skip: PASS
== bash 3 leaves ==
check bin-bash-3: PASS
== single selections (AFTER) ==
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCoverage/bin-bash-3/BASH_ENV
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCoverage/path-bash/without__context.md
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsUnreviewed/bin-bash-3/BASH_ENV
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsUnreviewed/path-bash/without__context.md
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsMode
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCrossCheck/bin-bash-3/BASH_ENV
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCrossCheck/path-bash/work_then_merge_after_a_record_from_the_merge-base
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsFableAlias/bin-bash-3/MISSING_when_rule_2_fix_uses_codex_seen_range
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsFableAlias/path-bash/fail-closed_on_missing__context.md
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCallSite
select ok: github.com/isseis/yt2column/cmd/yt2column TestRecordLinePrefixesDistinct
select ok: github.com/isseis/yt2column/cmd/yt2column TestRecordCommitsCheckMention
select ok: github.com/isseis/yt2column/cmd/yt2column TestHighReviewDecision/all-high_agent_coverage/commit_covered_by_both_high_and_non-high_records_is_tainted
select ok: github.com/isseis/yt2column/cmd/yt2column TestHighReviewDecision/values/unreadable
select ok: github.com/isseis/yt2column/cmd/yt2column TestHighReviewDecisionCallSite
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginRejectsInvalidPRArg/""
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginRejectsInvalidPRArg/"１"
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainZeroPRStopsBeforeAnything
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPRViewReadsNarrowFields
select ok: github.com/isseis/yt2column/internal/mergepr TestParseMergemainPRRejectsMalformedValues
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginPremisesAndNothingToTakeIn/GH_REPO_set
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginPremisesAndNothingToTakeIn/untracked_file
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainMergeNoFastForward
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainMergeRefusesIgnoredOverwrite
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainMergeChecksArguments
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainConflictClassification/binary_markdown
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainConflictClassification/tool_file_renamed_with_only_its_declaration_line_in_conflict
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainProtocolUpdateAndRuleFacts/protocol_line_conflict_resolved_to_main_plus_one
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainProtocolUpdateAndRuleFacts/unchanged_tool_not_set
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainReportsADRConflictFacts/branch_number_free
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainReportsADRConflictFacts/same_number_on_both_sides
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainExtCheckHint/English_README
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainExtCheckHint/unrelated_file
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainRestoreSafeAndRefusesOtherChanges/MERGE_HEAD_is_another_commit
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainRestoreSafeAndRefusesOtherChanges/untracked_file
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushSuccessRejectedAndUnknown/pushed
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushSuccessRejectedAndUnknown/unknown
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushRejectsNonHeadOrForce/GH_REPO_set
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushRejectsNonHeadOrForce/second_remote
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushChecksParentsAndMain/PR_head_is_not_the_base_head
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushChecksParentsAndMain/tracked_change
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushRejectsOutOfScope/go_file_changed
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushRejectsOutOfScope/protocol_line_of_a_tool_the_branch_did_not_change
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushChecksProtocolValues/body_of_a_changed_tool_edited
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushChecksProtocolValues/value_the_merge_set
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginMergeRestoreFlow
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginRefusesSubdirectory
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainRefusesOtherMergeDrivers/committed_union_driver
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainRefusesOtherMergeDrivers/push_of_a_union_merge_main's_rule_allows
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginAcceptsGHDefaultOfOrigin
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBlockingPathIgnoresLetterCase
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainReadsNoAttributesOutsideTheBaseHead/ignored_.gitattributes_at_merge
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainReadsNoAttributesOutsideTheBaseHead/untracked_.gitattributes_at_push
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainMergeIgnoresConfiguredMergeOptions
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushAfterOriginHasItAsksNoRestore
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushAfterItLandedAsksNoRestore
select ok: github.com/isseis/yt2column/internal/mergepr TestNewMergemainIgnoresTheCallersGitEnvironment
check single-first-last: PASS
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCrossCheck/path-bash/fail-closed_context_validation/valid_mapping_table_control
select ok: github.com/isseis/yt2column/cmd/yt2column TestHighReviewDecision/values/old_sonnet
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCoverage/path-bash/git_config/global/core.abbrev
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCrossCheck/path-bash/git_config_with_a_merge/repository/diff.relative
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCoverage/path-bash/merge_commit_inside_a_seen_range
check single-middle: PASS
== shuffle (AFTER) ==
shuffle run 1 seeds:
github.com/isseis/yt2column/cmd/yt2column -test.shuffle 1795738317091842000
github.com/isseis/yt2column/internal/mergepr -test.shuffle 1795738316871630000
shuffle run 2 seeds:
github.com/isseis/yt2column/cmd/yt2column -test.shuffle 1795738446459733000
github.com/isseis/yt2column/internal/mergepr -test.shuffle 1795738446658981000
check shuffle: PASS
== changed files (BASE..AFTER) ==
cmd/yt2column/fixture_snapshot_test.go
cmd/yt2column/high_review_decision_test.go
cmd/yt2column/main_test.go
cmd/yt2column/review_records_test.go
cmd/yt2column/test_helpers_gitfixture.go
docs/dev/conventions/package_reference.md
docs/tasks/0127_git_fixture_test_speedup/01_requirements.md
docs/tasks/0127_git_fixture_test_speedup/02_architecture.md
docs/tasks/0127_git_fixture_test_speedup/03_implementation_plan.md
docs/tasks/0127_git_fixture_test_speedup/_checks/compare.sh
docs/tasks/0127_git_fixture_test_speedup/_checks/measure.sh
internal/pipeline/pipeline_test.go
internal/snapshottest/snapshot.go
internal/snapshottest/snapshot_test.go
check range: PASS
compare: PASS
`````

**PR-2（2026-11-27、結果: 通った）**

-   BASE: `ff1973333e0d08c12108b384a226c110467049cd`、AFTER: `cdd92b731ea27f7a3de6e211acade5f9486e1e47`

`````text
BASE: ff1973333e0d08c12108b384a226c110467049cd
AFTER: cdd92b731ea27f7a3de6e211acade5f9486e1e47
packages: ./cmd/yt2column ./internal/mergepr
== full runs ==
full runs: both passed
BASE leaves:     1999, AFTER leaves:     1999, BASE tops:      268
== names, fail and skip (compare) ==
check names: PASS
check fail: PASS
check skip: PASS
== bash 3 leaves ==
check bin-bash-3: PASS
== single selections (AFTER) ==
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCoverage/bin-bash-3/BASH_ENV
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCoverage/path-bash/without__context.md
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsUnreviewed/bin-bash-3/BASH_ENV
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsUnreviewed/path-bash/without__context.md
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsMode
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCrossCheck/bin-bash-3/BASH_ENV
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCrossCheck/path-bash/work_then_merge_after_a_record_from_the_merge-base
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsFableAlias/bin-bash-3/MISSING_when_rule_2_fix_uses_codex_seen_range
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsFableAlias/path-bash/fail-closed_on_missing__context.md
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCallSite
select ok: github.com/isseis/yt2column/cmd/yt2column TestRecordLinePrefixesDistinct
select ok: github.com/isseis/yt2column/cmd/yt2column TestRecordCommitsCheckMention
select ok: github.com/isseis/yt2column/cmd/yt2column TestHighReviewDecision/all-high_agent_coverage/commit_covered_by_both_high_and_non-high_records_is_tainted
select ok: github.com/isseis/yt2column/cmd/yt2column TestHighReviewDecision/values/unreadable
select ok: github.com/isseis/yt2column/cmd/yt2column TestHighReviewDecisionCallSite
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginRejectsInvalidPRArg/""
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginRejectsInvalidPRArg/"１"
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainZeroPRStopsBeforeAnything
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPRViewReadsNarrowFields
select ok: github.com/isseis/yt2column/internal/mergepr TestParseMergemainPRRejectsMalformedValues
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginPremisesAndNothingToTakeIn/GH_REPO_set
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginPremisesAndNothingToTakeIn/untracked_file
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainMergeNoFastForward
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainMergeRefusesIgnoredOverwrite
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainMergeChecksArguments
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainConflictClassification/binary_markdown
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainConflictClassification/tool_file_renamed_with_only_its_declaration_line_in_conflict
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainProtocolUpdateAndRuleFacts/protocol_line_conflict_resolved_to_main_plus_one
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainProtocolUpdateAndRuleFacts/unchanged_tool_not_set
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainReportsADRConflictFacts/branch_number_free
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainReportsADRConflictFacts/same_number_on_both_sides
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainExtCheckHint/English_README
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainExtCheckHint/unrelated_file
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainRestoreSafeAndRefusesOtherChanges/MERGE_HEAD_is_another_commit
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainRestoreSafeAndRefusesOtherChanges/untracked_file
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushSuccessRejectedAndUnknown/pushed
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushSuccessRejectedAndUnknown/unknown
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushRejectsNonHeadOrForce/GH_REPO_set
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushRejectsNonHeadOrForce/second_remote
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushChecksParentsAndMain/PR_head_is_not_the_base_head
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushChecksParentsAndMain/tracked_change
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushRejectsOutOfScope/go_file_changed
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushRejectsOutOfScope/protocol_line_of_a_tool_the_branch_did_not_change
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushChecksProtocolValues/body_of_a_changed_tool_edited
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushChecksProtocolValues/value_the_merge_set
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginMergeRestoreFlow
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginRefusesSubdirectory
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainRefusesOtherMergeDrivers/committed_union_driver
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainRefusesOtherMergeDrivers/push_of_a_union_merge_main's_rule_allows
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginAcceptsGHDefaultOfOrigin
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBlockingPathIgnoresLetterCase
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainReadsNoAttributesOutsideTheBaseHead/ignored_.gitattributes_at_merge
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainReadsNoAttributesOutsideTheBaseHead/untracked_.gitattributes_at_push
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainMergeIgnoresConfiguredMergeOptions
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushAfterOriginHasItAsksNoRestore
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushAfterItLandedAsksNoRestore
select ok: github.com/isseis/yt2column/internal/mergepr TestNewMergemainIgnoresTheCallersGitEnvironment
check single-first-last: PASS
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainProtocolUpdateAndRuleFacts/protocol_line_conflict_resolved_to_main_plus_one
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainRestoreSafeAndRefusesOtherChanges/other_branch
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushChecksParentsAndMain/merge_in_progress
check single-middle: PASS
== shuffle (AFTER) ==
shuffle run 1 seeds:
github.com/isseis/yt2column/cmd/yt2column -test.shuffle 1795773553894447000
github.com/isseis/yt2column/internal/mergepr -test.shuffle 1795773554059666000
shuffle run 2 seeds:
github.com/isseis/yt2column/cmd/yt2column -test.shuffle 1795773683702969000
github.com/isseis/yt2column/internal/mergepr -test.shuffle 1795773683465352000
check shuffle: PASS
== changed files (BASE..AFTER) ==
internal/mergepr/main_test.go
internal/mergepr/mergemain_test.go
internal/mergepr/takein_snapshot_test.go
internal/mergepr/test_helpers_git.go
check range: PASS
compare: PASS
`````

**PR-3（2026-11-28、結果: `CHECK_SKIP_AC04=1` で通った）**

-   BASE: `0dcdfc0cbeefa68495349c12628a7c6a57232cf5`、AFTER: `abf0ebc69a3b3eb9715d041c0f87b2c1b9332bb1`
-   中間の名前は 1.3 節の表のうち、C2 で削除した `TestReviewRecordsCrossCheck/path-bash/git_config_with_a_merge/repository/diff.relative` を、残る `TestReviewRecordsCoverage/path-bash/git_config/repository/diff.relative` に替えた 8 つを渡した。

`````text
BASE: 0dcdfc0cbeefa68495349c12628a7c6a57232cf5
AFTER: abf0ebc69a3b3eb9715d041c0f87b2c1b9332bb1
packages: ./cmd/yt2column ./internal/mergepr
== full runs ==
full runs: AFTER passed (BASE skipped under CHECK_SKIP_AC04=1)
AFTER leaves:     1529, AFTER tops:      267
== names, fail and skip (compare) ==
names/skip: skipped (CHECK_SKIP_AC04=1)
check fail: PASS
== bash 3 leaves ==
bin-bash-3: skipped (CHECK_SKIP_AC04=1)
== single selections (AFTER) ==
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCoverage/bin-bash-3/BASH_ENV
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCoverage/path-bash/without__context.md
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsUnreviewed/bin-bash-3/HEAD_of_another_form
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsUnreviewed/path-bash/too_many_arguments
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsMode
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCrossCheck/bin-bash-3/ENTRY_of_another_form
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCrossCheck/path-bash/work_then_merge_after_a_record_from_the_merge-base
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsFableAlias/bin-bash-3/VALID_with_Cross-check_record
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsFableAlias/path-bash/fail-closed_on_missing__context.md
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCallSite
select ok: github.com/isseis/yt2column/cmd/yt2column TestRecordLinePrefixesDistinct
select ok: github.com/isseis/yt2column/cmd/yt2column TestRecordCommitsCheckMention
select ok: github.com/isseis/yt2column/cmd/yt2column TestHighReviewDecision/all-high_agent_coverage/commit_covered_by_both_high_and_non-high_records_is_tainted
select ok: github.com/isseis/yt2column/cmd/yt2column TestHighReviewDecision/values/unreadable
select ok: github.com/isseis/yt2column/cmd/yt2column TestHighReviewDecisionCallSite
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginRejectsInvalidPRArg/""
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginRejectsInvalidPRArg/"１"
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainZeroPRStopsBeforeAnything
select ok: github.com/isseis/yt2column/internal/mergepr TestParseMergemainPRRejectsMalformedValues
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginPremisesAndNothingToTakeIn/GH_REPO_set
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginPremisesAndNothingToTakeIn/untracked_file
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainMergeNoFastForward
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainMergeRefusesIgnoredOverwrite
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainMergeChecksArguments
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainConflictClassification/binary_markdown
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainConflictClassification/tool_file_renamed_with_only_its_declaration_line_in_conflict
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainProtocolUpdateAndRuleFacts/protocol_line_conflict_resolved_to_main_plus_one
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainProtocolUpdateAndRuleFacts/unchanged_tool_not_set
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainReportsADRConflictFacts/branch_number_free
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainReportsADRConflictFacts/same_number_on_both_sides
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainExtCheckHint/English_README
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainExtCheckHint/extension_file
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainRestoreSafeAndRefusesOtherChanges/MERGE_HEAD_is_another_commit
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainRestoreSafeAndRefusesOtherChanges/untracked_file
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushSuccessRejectedAndUnknown/rejected
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushSuccessRejectedAndUnknown/unknown
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushRejectsNonHeadOrForce/PR_head_branch_is_main
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushRejectsNonHeadOrForce/push_command
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushChecksParentsAndMain/PR_head_is_not_the_base_head
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushChecksParentsAndMain/tracked_change
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushRejectsOutOfScope/go_file_changed
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushRejectsOutOfScope/protocol_line_of_a_tool_the_branch_did_not_change
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushChecksProtocolValues/body_of_a_changed_tool_edited
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushChecksProtocolValues/value_the_merge_set
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginMergeRestoreFlow
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBeginRefusesSubdirectory
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainRefusesOtherMergeDrivers/committed_union_driver
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainRefusesOtherMergeDrivers/push_of_a_union_merge_main's_rule_allows
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainBlockingPathIgnoresLetterCase
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainReadsNoAttributesOutsideTheBaseHead/ignored_.gitattributes_at_merge
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainReadsNoAttributesOutsideTheBaseHead/untracked_.gitattributes_at_push
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainMergeIgnoresConfiguredMergeOptions
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushAfterOriginHasItAsksNoRestore
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushAfterItLandedAsksNoRestore
select ok: github.com/isseis/yt2column/internal/mergepr TestNewMergemainIgnoresTheCallersGitEnvironment
check single-first-last: PASS
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCrossCheck/path-bash/fail-closed_context_validation/valid_mapping_table_control
select ok: github.com/isseis/yt2column/cmd/yt2column TestHighReviewDecision/values/old_sonnet
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCoverage/path-bash/merge_commit_inside_a_seen_range
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCoverage/path-bash/git_config/global/core.abbrev
select ok: github.com/isseis/yt2column/cmd/yt2column TestReviewRecordsCoverage/path-bash/git_config/repository/diff.relative
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainProtocolUpdateAndRuleFacts/protocol_line_conflict_resolved_to_main_plus_one
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainRestoreSafeAndRefusesOtherChanges/other_branch
select ok: github.com/isseis/yt2column/internal/mergepr TestMergemainPushChecksParentsAndMain/merge_in_progress
check single-middle: PASS
== shuffle (AFTER) ==
shuffle run 1 seeds:
github.com/isseis/yt2column/cmd/yt2column -test.shuffle 1795793555093563000
github.com/isseis/yt2column/internal/mergepr -test.shuffle 1795793554830544000
shuffle run 2 seeds:
github.com/isseis/yt2column/cmd/yt2column -test.shuffle 1795793632949902000
github.com/isseis/yt2column/internal/mergepr -test.shuffle 1795793632711856000
check shuffle: PASS
== changed files (BASE..AFTER) ==
cmd/yt2column/harness_mapping_cases_test.go
cmd/yt2column/high_review_decision_test.go
cmd/yt2column/review_records_test.go
cmd/yt2column/test_helpers_bash.go
docs/tasks/0127_git_fixture_test_speedup/01_requirements.md
docs/tasks/0127_git_fixture_test_speedup/02_architecture.md
docs/tasks/0127_git_fixture_test_speedup/03_implementation_plan.md
docs/tasks/0127_git_fixture_test_speedup/_checks/compare.sh
docs/tasks/0127_git_fixture_test_speedup/_checks/verify_bash_cut.sh
internal/mergepr/mergemain_test.go
check range: PASS
compare: PASS
`````

### 7.3. 一時スクリプトの記録（`_checks/measure.sh`）

-   **スクリプト:** `docs/tasks/0127_git_fixture_test_speedup/_checks/measure.sh`。BASE と AFTER を交互に取り出して計測の条件のコマンドを 3 回ずつ流し、パッケージごとの時間、中央値、割合と、機械の情報を出す（設計書 7.5）。AC-01・AC-02・AC-03 を確かめる。
-   **判定:**
    -   作業ツリーに変更がないこと
    -   6 回のどの実行も通り、2 つのパッケージの時間を読めること
    -   機械の情報を読めること
    -   終わった後に、元のブランチに戻っていること
-   **壊し方の確認:** 各判定が少なくとも 1 つの壊し方で失敗することを確かめる（ステップ 6-3）。
-   **予定の実行:**
    -   PR-1 と PR-2 のそれぞれで、PR を作る前に、`/runplan` が人に尋ねてから 1 回実行する（ステップ 6-4・7-4）。ステップ 7-5 を足したときは、その後にもう 1 回実行する。
    -   記録の後に、タスクのディレクトリの外のファイルかスクリプトが変わったとき（PR を開く前のレビューの直し、PR のレビューの直し、`origin/main` の取り込み）は、変えた人が、人への問いからやり直して実行し直す。スクリプトが変わったときは、壊し方の確認もやり直す（`requirements_process.md`「Temporary script record」の「Rewriting」）。
-   **テスト:** コミットしない。
-   **壊し方の記録:** 確かめたコミット: `cc946ac57728ce55b6676f6c68b6fe5371a825a7`。`CHECK_PKGS=./internal/strictjson`、`CHECK_ROUNDS=1` に狭めて流した（狭めた実行は計測として記録しない）。壊さない実行は `measure: DONE`（0）で、`BASE 3.051s AFTER 2.858s ratio 0.937` を出した。
    -   作業ツリー: 追跡されていないファイルを置いた → `FAIL: the work tree is not clean`（1）
    -   実行の合否: `CHECK_PKGS=./nonexistent` → `FAIL: go test failed at BASE round 1`（1）。`CHECK_RUN` が何も選ばない → `FAIL: cannot read the time of ./internal/strictjson at BASE round 1`（1）
    -   機械の情報: コアの数の読み取りを空にした（写したスクリプトで） → `FAIL: cannot read the core count`（1）
    -   元のブランチへの復帰: ブランチへの切り替えを外した（写したスクリプトで） → `STOP: not on the original branch`（1）。その後、手でブランチに戻した
-   **実行の記録:** PR-1・PR-2・PR-3 の実行は 7.1 節に置いた（出力の全体と 2 つのコミットの ID）。

### 7.4. 一時スクリプトの記録（`_checks/compare.sh`）

-   **スクリプト:** `docs/tasks/0127_git_fixture_test_speedup/_checks/compare.sh`。BASE と AFTER で `-json` の実行を流して比べ、AFTER で単独の選択と `-shuffle=on` の 2 回を流し、BASE と AFTER の間の変えたファイルを調べる（設計書 7.5、1.3 節）。AC-04・AC-05・AC-09・AC-11 を確かめる。
-   **判定:**
    -   **名前の集まり（AC-04・AC-05）:** BASE の最上位のテスト関数ごとに、末端の名前の集まりが BASE と AFTER で同じであること（実行ごとに変わるコミットの短い ID は置き換えて比べる）
    -   **`fail` と `skip`（AC-04）:** 比べる最上位のテスト関数の下に `fail` の事象がなく、`skip` の事象の `Test` の値の集まりが BASE と AFTER で同じであること
    -   **bash 3 の末端（AC-05）:** `/bin/bash` が 3.x の機械で、BASE の 4 つの関数の下に `bin-bash-3` の末端があること
    -   **単独の選択（AC-11）:** 3 つのファイルの最上位のテスト関数ごとの最初と最後の末端と、1.3 節の表の名前が、それぞれ単独の実行で、その名前の `pass` の事象とともに合格すること。表の名前が BASE にあり、最初でも最後でもないこと
    -   **shuffle（AC-11）:** `-shuffle=on` の 2 回が合格し、各パッケージのシードを読めること
    -   **差分の範囲（AC-09）:** 変えたファイル（rename の検出を切った一覧）のすべてが、ステップ 6-2 の範囲に入ること
    -   終わった後に、元のブランチに戻っていること
-   **壊し方の確認:** 各判定が少なくとも 1 つの壊し方で失敗することを確かめる（ステップ 6-3）。
-   **予定の実行:** 7.3 節と同じ時点と人で実行する。
-   **テスト:** コミットしない。
-   **壊し方の記録:** 確かめたコミット: `6649ffc4649bb94a689ed3ad30daea7db57bbc62`（PR-3 用オプション `CHECK_SKIP_AC04=1` を足した後のスクリプトで、全判定の壊し確認をやり直した。壊さない実行は通常モード・`CHECK_SKIP_AC04=1` モードとも `compare: PASS`）。作業ツリーの外に写したスクリプトに壊し方を入れるか、引数・環境変数で入力を替えて流した（`CHECK_PKGS=./cmd/yt2column`、`CHECK_RUN` と `CHECK_TOPS` で `TestReviewRecordsFableAlias` と `TestHighReviewDecisionCallSite` に狭めた。狭めた実行は記録しない）。どの壊し方も作業ツリーに残していない。
    -   名前の集まり: AFTER の末端の名前を 1 行消した → `check names: FAIL (1 differing names)`（終了コード 1）
    -   `fail`: `CHECK_SKIP_AC04=1` で AFTER の `fail` の事象を 1 件足した → `check fail: FAIL (fail events present)`（1）
    -   `skip`: AFTER の `skip` の集まりに 1 件足した → `check skip: FAIL (skip sets differ)`（1）
    -   bash 3 の末端: 写したスクリプトの対象の関数の並びを `TestReviewRecordsFableAlias` 1 つに替えて、壊さない実行は `check bin-bash-3: PASS`。名前の `bin-bash-3` を `bin-bash-9` に替えた → `check bin-bash-9: FAIL`（1）
    -   単独の選択: `CHECK_SKIP_AC04=1` で AFTER にない最上位の名前 `TestNoSuchTop` を渡した → `select FAILED: TestNoSuchTop has no leaf in AFTER`、`check single-first-last: FAIL`（1）
    -   中間の名前: `CHECK_SKIP_AC04=1` でパッケージの最初の末端（`TestHighReviewDecisionCallSite`）を渡した → `middle FAILED: TestHighReviewDecisionCallSite is the first or the last leaf of github.com/isseis/yt2column/cmd/yt2column`、`check single-middle: FAIL`（1）。名前を渡さない → `middle FAILED: no middle name given`、`check single-middle: FAIL`（1）
    -   shuffle: `-shuffle=on` を `-count=1` に替えた → `shuffle run 1: expected 1 seed lines, read 0`、`check shuffle: FAIL`（1）
    -   差分の範囲: 一覧に `README.md` を足した → `range FAILED: README.md`、`check range: FAIL`（1）
    -   `go test` の失敗: `CHECK_PKGS=./nonexistent` → `compare: STOP: go test failed at AFTER`（1）
    -   作業ツリー: 追跡されていないファイルを置いた → `compare: STOP: the work tree is not clean`（1）
    -   元のブランチへの復帰: ブランチへの切り替えを外した → `compare: STOP: not on the original branch`（1）。その後、手でブランチに戻した
-   **実行の記録:** PR-1・PR-2・PR-3 の実行は 7.2 節に置いた（出力の全体と 2 つのコミットの ID）。PR-1 の中間の名前は 1.3 節の表の `cmd/yt2column` の 5 つ、PR-2 の中間の名前は `internal/mergepr` の 3 つ（`TestMergemainProtocolUpdateAndRuleFacts/protocol_line_conflict_resolved_to_main_plus_one`、`TestMergemainRestoreSafeAndRefusesOtherChanges/other_branch`、`TestMergemainPushChecksParentsAndMain/merge_in_progress`）、PR-3 の中間の名前は `CHECK_SKIP_AC04=1` で渡した 8 つ（C2 で削除した 1 つを `TestReviewRecordsCoverage/path-bash/git_config/repository/diff.relative` に差し替え）である。

### 7.5. 前提の検査の確かめ（AC-07）

前提の検査の場所は、BASE（コミット `51f82a71`）の行番号で示す。AFTER での場所は設計書 7.4 の表のとおりである。

| 前提の検査 | 崩して確かめるステップ（結果を書くコミット） |
|---|---|
| refs/replace（`high_review_decision_test.go:546`） | ステップ 2-3 |
| 設定の前の出力（`review_records_test.go:817`） | ステップ 3-3 |
| 設定による plain git の変化（`review_records_test.go:826`） | ステップ 3-3 |
| `requireProbe`（`review_records_test.go:125`） | ステップ 4-5 |
| `mainMerge`（`:235`）、`rrSubmodule`（`:324`） | ステップ 4-5（ケースの `setup` での呼び出し）、ステップ 3-3（グループのブランチの中の `mainMerge`。親のサブテストの失敗として） |
| `setup` の中の 7 か所（`:379`・`:447`・`:690`・`:1356`・`:1439`・`:1817`・`:1832`） | ステップ 4-5 |

### 7.6. カバレッジの比較（AC-06）

（`/runplan` が書く。PR ごとに、`make coverage-compare BASE=<BASE>` のすべての実行の出力の最後の部分と、実行したコミット）

**PR-1（2026-11-27）** 1 回流して通った。実行したコミット: `8bb968e5c28f44bafd2a3811d190e2fdac2a72ac`

`````text
target    make coverage-compare BASE="fb71447bb8bd1baa47e113573db52939b41146cd"
base      fb71447bb8bd1baa47e113573db52939b41146cd
head      8bb968e5c28f44bafd2a3811d190e2fdac2a72ac tree ebe3271f787588de2f8a9265b2013b90fd5a560e
toolchain go1.27.1
tests     base 1831 head 1840
files     base 195 head 195
functions base 2348 head 2348
deleted   0
no compared function lost coverage
coverage-compare: PASS
`````

**PR-2（2026-11-27）** 1 回流して通った。実行したコミット: `cdd92b731ea27f7a3de6e211acade5f9486e1e47`

`````text
target    make coverage-compare BASE="ff1973333e0d08c12108b384a226c110467049cd"
base      ff1973333e0d08c12108b384a226c110467049cd
head      cdd92b731ea27f7a3de6e211acade5f9486e1e47 tree 588d0b9cfbe45cde42a21802b174dbf447a107e4
toolchain go1.27.1
tests     base 1845 head 1846
files     base 196 head 196
functions base 2354 head 2354
deleted   0
no compared function lost coverage
coverage-compare: PASS
`````

**PR-3（2026-11-28）** 削除専用コミット 3 本（`77b83957`・`882fb0b2`・`abf0ebc6`）の個別比較および PR-3 全体（BASE=`0dcdfc0cbeefa68495349c12628a7c6a57232cf5`、HEAD=`abf0ebc69a3b3eb9715d041c0f87b2c1b9332bb1`）がいずれも 1 回流して通った。

-   Part 1 削除コミット 1（C4・C5・C8・C9、コミット `77b83957`、BASE=`f7ce1729ee0cea9d6b3cd3ad2d45b2ed9f66dc80`、tree `8cc9846b7e0e12191b95fbe114fc3d7a68e27829`）: `tests base 1850 head 1850`、`functions base 2354 head 2354`、`no compared function lost coverage`、`coverage-compare: PASS`
-   Part 1 削除コミット 2（C2・C3・C6・C7・C10・C11、コミット `882fb0b2`、BASE=`77b83957467026a51ef5ae011a5e62a51e9e8aa4`、tree `3bff833d0d38a09f10a11289c06609aea3e8a371`）: `tests base 1850 head 1850`、`functions base 2354 head 2354`、`no compared function lost coverage`、`coverage-compare: PASS`
-   Part 2 削除コミット（M1〜M13、コミット `abf0ebc6`、BASE=`fae73f658ffb2a4e0bc8207fae703e497dcaeda0`、tree `6a71a7a3b1deb1604a168fd7339fc68473967fd3`）: `tests base 1850 head 1848`、`functions base 2354 head 2354`、`no compared function lost coverage`、`coverage-compare: PASS`
-   PR-3 全体（AC-06、実行したコミット `abf0ebc69a3b3eb9715d041c0f87b2c1b9332bb1`）:

`````text
target    make coverage-compare BASE="0dcdfc0cbeefa68495349c12628a7c6a57232cf5"
base      0dcdfc0cbeefa68495349c12628a7c6a57232cf5
head      abf0ebc69a3b3eb9715d041c0f87b2c1b9332bb1 tree 6a71a7a3b1deb1604a168fd7339fc68473967fd3
toolchain go1.27.1
tests     base 1850 head 1848
files     base 196 head 196
functions base 2354 head 2354
deleted   0
no compared function lost coverage
coverage-compare: PASS
`````

### 7.7. テストと lint（AC-12）

（`/runplan` が書く。PR ごとに、実行したコミットと、`make test`・`make lint` の終了コード）

**PR-1（2026-11-27）** 実行したコミット: `8bb968e5c28f44bafd2a3811d190e2fdac2a72ac`。`make test` は終了コード 0、`make lint` は終了コード 0（`0 issues.`）。

**PR-2（2026-11-27）** 実行したコミット: `cdd92b731ea27f7a3de6e211acade5f9486e1e47`。`make test` は終了コード 0、`make lint` は終了コード 0（`0 issues.`）。

**PR-3（2026-11-28）** 実行したコミット: `abf0ebc69a3b3eb9715d041c0f87b2c1b9332bb1`。`make test` は終了コード 0、`make lint` は終了コード 0（`0 issues.`）、`make deadcode` は終了コード 0（出力なし）。

### 7.8. 一時スクリプトの記録（`_checks/verify_bash_cut.sh`）

-   **スクリプト:** `docs/tasks/0127_git_fixture_test_speedup/_checks/verify_bash_cut.sh`。BASE と AFTER で一時的に計装した `review-records.sh` と `high-review-decision.sh`（`env -i` 後の `set -T`＋`trap DEBUG` で `TMPDIR` 外のトレースファイルへ記録）を使ってテストを走らせ、各 bash でのスクリプト全体の実行行集合（`line`）一致・全 mode 分岐と `fail()` 分岐への到達・代表ケースへのミューテーション検出・`YT2COLUMN_TEST_BASH3_ALL=1` 時の `bin-bash-3` 末端集合一致を確かめる。AC-13 (2)(3) を確かめる。
-   **判定:**
    -   作業ツリーに変更がないこと
    -   **実行行集合一致（`review-records.sh`・`path-bash`）:** BASE と AFTER で `path-bash` が実行した `review-records.sh` の行集合が一致し、全 mode 分岐と `fail()` 分岐に到達すること
    -   **実行行集合一致（`review-records.sh`・`bin-bash-3`）:** `/bin/bash` が 3.x の機械で、BASE と AFTER（既定サブセット）で `bin-bash-3` が実行した `review-records.sh` の行集合が一致し、全 mode 分岐と `fail()` 分岐に到達すること
    -   **実行行集合一致（`high-review-decision.sh`・`path-bash`）:** BASE と AFTER で `path-bash` が実行した `high-review-decision.sh` の行集合が一致すること
    -   **ミューテーション検出:** AFTER に対して `-c i18n.logOutputEncoding=UTF-8` 削除、`widen` 分岐の変更、`fable` 分岐の削除のそれぞれを入れたとき、対応するテストが失敗すること
    -   **オプトイン全件一致（AC-13 (3)）:** AFTER で `YT2COLUMN_TEST_BASH3_ALL=1` を付けて走らせた `bin-bash-3` の末端集合が、AFTER の `path-bash` の末端集合（`bin-bash-3` を `path-bash` に読み替えたもの）と一致すること
    -   終わった後に、元のブランチに戻り作業ツリーがきれいであること
-   **壊し方の確認:** 各判定が少なくとも 1 つの壊し方で失敗することを確かめる（ステップ 8-2）。
-   **予定の実行:** PR-3 を作る前に `/runplan` が実行し、結果を本節に書く。記録の後にタスクのディレクトリの外のファイルかスクリプトが変わったときはやり直す（スクリプトが変わったときは壊し確認もやり直す）。
-   **テスト:** コミットしない。
-   **壊し方の記録:** 確かめたコミット: `a579553be4706d346d842d0cbc844d9ade597edb`（既定実行の `env -u "$optin_var"` 隔離と `PARTIAL`/`MISSING` 検索の `|| true` を足した後のスクリプトで、全判定の壊し確認をやり直した）。`CHECK_ONLY`・`CHECK_RR_RUN`・`CHECK_HRD_RUN` で狭め、作業ツリーの外に写したスクリプトに壊し方を入れて確かめた（狭めた実行は記録しない）。壊さない実行（環境変数 `YT2COLUMN_TEST_BASH3_ALL=1` 下での `optin-all` を含む）は `verify_bash_cut: PASS`（0）。
    -   作業ツリー: 追跡されていないファイルを置いた → `verify_bash_cut: STOP: the work tree is not clean`（1）
    -   `review-records.sh` 行集合（`path-bash`・`bin-bash-3`）: AFTER の行一覧から 1 行削った → `check rr-lines-path-bash: FAIL (line sets differ)`、`check rr-lines-bin-bash-3: FAIL (line sets differ)`（1）
    -   全 mode 分岐と `fail()` 分岐: `TestReviewRecordsFableAlias` のみに狭めた → `check rr-modes-path-bash: FAIL (missing modes: coverage unreviewed cross-check, fail_branches_missing=1)`、`check rr-modes-bin-bash-3: FAIL (missing modes: coverage unreviewed cross-check, fail_branches_missing=1)`（1）
    -   `high-review-decision.sh` 行集合: AFTER の行一覧から 1 行削った → `check hrd-lines: FAIL (line sets differ)`（1）
    -   ミューテーション検出: `drop-log-encoding` のミューテーションをテストを落とさないコメント追記（`# noop`）に替えた（`cmp -s` は差分ありと判定し `go test` が rc=0 で通る） → `check mut-drop-log-encoding: FAIL (expected test failure (rc=0))`（終了コード 1）。また `drop-fable` の置換パターンを一致しない文字列に替えた → `check mut-drop-fable: FAIL (mutation did not change .claude/commands/_lib/review-records.sh)`（1）
    -   オプトイン全件一致: C1 適用前の `BASE == AFTER`（既定と全件の末端数が同じ 4 件）で流した → `check optin-all: FAIL (default or opt-in leaf sets do not match expectations)`（1）。なお環境変数 `YT2COLUMN_TEST_BASH3_ALL=1` 下で狭めて流した健常実行は `default leaves: path-bash=2, bin-bash-3=1; opt-in leaves: path-bash=2, bin-bash-3=2` で `check optin-all: PASS`（0）
    -   元のブランチへの復帰: ブランチへの切り替えを外した → `verify_bash_cut: STOP: not on the original branch`（1）。その後、手でブランチに戻した
-   **実行の記録:** PR-3（2026-11-28、結果: 通った。スクリプトのコミット: `a579553be4706d346d842d0cbc844d9ade597edb`）。BASE: `0dcdfc0cbeefa68495349c12628a7c6a57232cf5`、AFTER: `abf0ebc69a3b3eb9715d041c0f87b2c1b9332bb1`。

`````text
BASE: 0dcdfc0cbeefa68495349c12628a7c6a57232cf5
AFTER: abf0ebc69a3b3eb9715d041c0f87b2c1b9332bb1
path-bash major: 5, /bin/bash major: 3
== review-records.sh line coverage ==
path-bash lines: BASE=341 AFTER=341
check rr-lines-path-bash: PASS
check rr-modes-path-bash: PASS
bin-bash-3 lines: BASE=342 AFTER=342
check rr-lines-bin-bash-3: PASS
check rr-modes-bin-bash-3: PASS
== high-review-decision.sh line coverage ==
high-review-decision lines: BASE=184 AFTER=184
check hrd-lines: PASS
== mutations (AFTER) ==
check mut-drop-log-encoding: PASS
check mut-break-widen: PASS
check mut-drop-fable: PASS
== bash 3.2 opt-in all leaves (AFTER) ==
default leaves: path-bash=227, bin-bash-3=51; opt-in leaves: path-bash=227, bin-bash-3=227
check optin-all: PASS
verify_bash_cut: PASS
`````

## 8. 成功基準 (Success Criteria)

-   **機能:** このタスクの `cmd/yt2column` と `internal/mergepr` の割合が 0.80 以下である（AC-01・AC-02）。
-   **品質:** ケースの集まりが減らず（PR-1・PR-2 は AC-04。PR-3 は検証済みの削除だけで AC-13）、製品コードのカバレッジが下がらず（AC-06）、13 か所の前提の検査が働き（AC-07）、`make test` と `make lint` が通る（AC-12）。
-   **セキュリティ:** N/A（要件書 4.2）。
-   **文書:** 7 章の記録がそろい、`package_reference.md` に `internal/snapshottest` の行がある。

## 9. 次のステップ (Next Steps)

-   PR-3 のマージの後、issue #1168 を閉じる（`/runpr` が、タスクの最後の計画の PR の後に提案する）。
-   `scriptBashes` を使うほかのテストへの適用は、範囲外として残す（設計書 9 章）。

## 変更履歴

- 2026-11-26 承認（isseis、#1188）
- 2026-11-27 セッション中の再承認：`cmd/yt2column` の目標を 80% 以下に改め、比較のスクリプトの名前の比べ方を直し、1.5 節に記録した（isseis、#1193）
- 2026-11-27 セッション中の再承認：`internal/mergepr` の目標を 80% 以下に改め、1.5 節に記録した（isseis、#1197）
- 2026-11-27 セッション中の再承認：冗長テストの削減（フェーズ 8・PR-3）を計画に追加した（isseis、#1197）
- 2026-11-27 セッション中の再承認：1.6 節に削減候補表を置き、C2 を縮小版にし、フェーズ 8 の検証（`coverage-compare`、`high-review-decision.sh` への適用、停止条件、AC-06・AC-09・AC-11）と、1.3 節・1.5 節・8 章・9 章の旧い記述を直した（isseis、#1197）
- 2026-11-27 セッション中の再承認：ステップ 8-2 の bash 行カバレッジ方式を `set -T`＋`trap DEBUG` によるスクリプト全体の実行行集合一致＋全 mode 分岐到達とし、7.8 節に `_checks/verify_bash_cut.sh` の記録枠を置き、`_checks/compare.sh` に PR-3 用オプションを足す方針に改めた（isseis、#未記入）
