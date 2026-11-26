# 要件定義書：review_records・high_review_decision・mergemain のテストの高速化

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-26 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、Go のテストのうち、時間の大半を占める 3 つのファイル（`cmd/yt2column/review_records_test.go`・`cmd/yt2column/high_review_decision_test.go`・`internal/mergepr/mergemain_test.go`）のテストを、検証の質を保ったまま速くするための要件を定義する（[#1168](https://github.com/isseis/yt2column/issues/1168)）。

**背景:**

issue #1168 の計測（`go test -tags test -race -count=1 ./...`）では、次の 3 つのファイルが、それぞれのパッケージの壁時計の時間の主な原因になっている。

| ファイル | 末端のサブテストの時間の合計 | サブテスト数（最上位 / 末端） | パッケージの壁時計の時間 |
|---|---:|---:|---:|
| `cmd/yt2column/review_records_test.go` | 2659.6 秒 | 8 / 734 | 387.5 秒 |
| `internal/mergepr/mergemain_test.go` | 881.9 秒 | 28 / 112 | 128.7 秒 |
| `cmd/yt2column/high_review_decision_test.go` | 170.9 秒 | 2 / 102 | 387.5 秒 |

今のテストの作りは次のとおりである。

-   **フィクスチャのリポジトリを、ケースごとに作り直す。** `review_records_test.go` は、ケースごとに `newRRFixtureWithContext`（`cmd/yt2column/review_records_test.go:136-152`）で `git init` から始め、初期のコミット 2 つ、`origin/main`、`feature` の枝を作る。`internal/mergepr` は、テストごとに `newTakeIn`（`internal/mergepr/test_helpers_git.go:245-270`）で bare の origin を作って clone する。
-   **git の設定のシナリオは、シナリオごとにスクリプトを 2 回走らせる。** `checkConfigScenarios`（`cmd/yt2column/review_records_test.go:805-834`）は、2 つのスコープ（リポジトリ・グローバル）と 8 つの設定の組ごとに、フィクスチャと枝を作り、設定の前と後でスクリプトを 1 回ずつ走らせる。設定の前の実行は、同じ枝なら同じ出力になるはずのものである。
-   **bash を 2 本走らせるのは `review_records_test.go` だけである。** `scriptBashes`（`cmd/yt2column/test_helpers_bash.go:47-68`）は、`/bin/bash` が `PATH` の bash と異なる 3.x のとき（macOS）に、両方を返す。`review-records.sh` は bash 3.2 より新しい機能を使わないと約束している（`.claude/commands/_lib/review-records.sh:76-77`）。`high_review_decision_test.go` は、bash 4 以上の 1 本だけで走る（`requireBash4`、`cmd/yt2column/high_review_decision_test.go:221`）。CI は ubuntu だけで走る（`.github/workflows/ci.yml`）ので、bash 3.2 での確認は macOS の手元の `make test` にしかない。

本書を書く際に、手元（12 コアの macOS、`PATH` の bash は 5.3、`/bin/bash` は 3.2）で次を計った。

-   git の設定のシナリオ 1 件（`TestReviewRecordsCoverage/path-bash/git_config/repository/diff.noprefix`）を単独で走らせると 2.14 秒だった。その内訳は、フィクスチャの作成が 約 0.24 秒、枝の組み立て（`rrConfigBranch`）が 約 0.44 秒、`review-records.sh` の 1 回の実行が 約 0.93 秒（`PATH` の bash でも `/bin/bash` でも同じ）である。並列で 15〜16 秒かかるのは、CPU の取り合いのためである。
-   したがって、時間の大半はスクリプトの実行であり、issue が挙げる原因（フィクスチャの作り直し）は一部にとどまる。この計測は、issue の改善方針 1・2（テンプレートのリポジトリの複製）だけでは目標に届かないおそれを示す。
-   `go test -tags test -race -count=1 ./cmd/yt2column ./internal/mergepr`（2 つのパッケージだけ）を 1 回流すと、`cmd/yt2column` が 261.4 秒、`internal/mergepr` が 105.0 秒だった。issue の `./...` での値より短いのは、ほかのパッケージと CPU を取り合わないためである。

対応 issue: [#1168](https://github.com/isseis/yt2column/issues/1168)

**本書の記述範囲:** 本書は、短縮の目標と測り方、保つべき検証の質、変えてよいファイルの範囲を定める。短縮の手段（テンプレートのリポジトリの複製、実行の結果の共有、並列の度合いなど）は、設計（`02_architecture.md`）で選ぶ。要件の起草で挙がった設計の事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #1168 で決めたこと:**

-   **対象:** 上の 3 つのファイルのテストと、その補助の関数を対象にする。
-   **方針の候補:** 共通の初期状態をテンプレートのリポジトリとして 1 回だけ作り、ケースはその複製から始めること。git の設定のシナリオの準備を最適化すること。bash 3 の二重の実行の要否を整理すること。
-   **完了の条件:** `review-records.sh`・`high-review-decision.sh`・`mergemain` の検証の質（bash 3 での互換性と、git の設定に左右されないことの確認を含む）を保つ。`make test` と `make lint` が通る。

**要件定義の際に決めたこと（2026-11-26）:**

-   **進め方は通常の手順にする:** 変わるのはテストだけだが、`internal/mergepr` のテストは、開発の道具の信頼境界を守る `mergepr`（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）の検証である。簡易な手順から外す条件（`docs/dev/conventions/requirements_process.md`、「Simple Process」の「Excluded changes」）に当たるかは迷う程度に当たるので、迷うときの規則に従って外れる変更と判断した。さらに、新しいテストの仕組みを作り、決めることも多い。この判断のとおり、人は通常の手順を選んだ。ADR はなし（手順の選択は、このタスクの中で閉じる）。
-   **短縮の目標は、同じ機械で基準の半分以下にする:** 同じ機械と条件で変更の前と後を測り、`cmd/yt2column` と `internal/mergepr` の壁時計の時間を、どちらも前の 50% 以下にする（F-001）。理由: issue の「大幅に短縮」は測れないので、数値にする。壁時計の時間はほかのパッケージの負荷で変わるので、機械ごとの絶対の秒数ではなく、同じ条件で比べた割合で決める。末端のサブテストの時間の合計は、並列の取り合いで膨らむ値なので、目標にしない。ADR はなし（このタスクの目標で、ほかのタスクを縛らない）。
-   **計測は 2 つのパッケージを 3 回ずつ流し、中央値で比べる:** 同じ機械で `go test -tags test -race -count=1 ./cmd/yt2column ./internal/mergepr` を変更の前と後で 3 回ずつ流し、パッケージごとの中央値を比べる（AC-03）。理由: 2 つのパッケージだけを流せば、ほかのパッケージの負荷によるぶれが減る。1 回だけでは負荷のぶれを受けやすいので、3 回の中央値にする。`make test` と同じ `-race` を付ける。ADR はなし（このタスクの測り方である）。
-   **bash 3 での全件の二重の実行を残す:** `review_records_test.go` は、今と同じく、`scriptBashes` が返すすべての bash で全件を走らせる（AC-05）。理由: bash 3.2 での確認は macOS の手元にしかなく、Go のカバレッジは bash の行を測れないので、代表のケースに絞ったときに互換性の確認が減っていないことを確かめる手段がない。短縮は、実行の結果の共有とフィクスチャの再利用で得る。ADR はなし（今のテストの作りを保つだけである）。
-   **短縮の手段は設計に任せる:** 要件は、目標と、検証の質を保つ条件と、変えてよいファイルの範囲だけを定める。手段は、issue が挙げるテンプレートのリポジトリの複製に限らず、設計で選ぶ（[design_handoff.md](design_handoff.md) の H-01）。理由: 上の計測では時間の大半がスクリプトの実行なので、issue が挙げる手段だけでは目標に届かないおそれがある。ADR はなし（手段の選択はこのタスクの中で閉じる。設計で主要な決定が出れば、`02_architecture.md` で判断する）。
-   **検証の質は、末端のサブテストの名前の集まり、Go のカバレッジ、壊したときの失敗、レビューで確かめる:** 末端のサブテストの名前の集まりが、変更の前と後で同じであること（AC-04）。モジュールの製品コードのカバレッジが下がらないこと（AC-06）。各ケースの前提の確認（「so the case checks nothing」で失敗させる確認）が、前提が崩れたときに今と同じく失敗させること（AC-07）。各ケースの期待する出力と失敗の条件が変わっていないことは、PR のレビューで読む（4.5）。理由: 手段を設計に任せるので、テストの形が変わっても、ケースが減っていないことを機械的に確かめられるようにする。名前の集まりを縛ると、設計はケースをまとめたり名前を変えたりできないが、ケースが減ったことを検出できる利点のほうが大きいと人が判断した。ADR はなし（このタスクの受け入れ基準である）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `make test` で最も時間のかかる 2 つのパッケージ（`cmd/yt2column`・`internal/mergepr`）の壁時計の時間を、半分以下にする。
-   その際、3 つのファイルのテストが確かめていること（ケースの集まり、bash 3 での実行、git の設定に左右されないことの確認、前提の確認）を減らさない。

### 2.2. スコープ (In Scope)

-   `cmd/yt2column/review_records_test.go`・`cmd/yt2column/high_review_decision_test.go`・`internal/mergepr/mergemain_test.go` のテストと、それらが使う補助の関数（`cmd/yt2column/test_helpers_*.go`・`internal/mergepr/test_helpers*.go` など、`test` のビルドタグか `_test.go` のファイルにあるもの）の変更。
-   短縮の前と後の計測と、その記録。

### 2.3. スコープ外 (Out of Scope)

-   **製品コードとスクリプトの変更:** `.claude/commands/_lib/review-records.sh`・`high-review-decision.sh`、`internal/mergepr` の製品コード、ほかの製品コードは変えない（AC-09）。理由: issue はテストの高速化を求めており、検証の対象を変えると「検証の質を保つ」の比較が成り立たない。
-   **bash 3 での実行の削減:** 全件の二重の実行を残す（§1 の決定）。
-   **ほかのテストファイル:** `scriptBashes` を使う `cmd/yt2column/adr_candidates_docs_test.go:373` や、ほかのパッケージのテストは対象にしない。理由: issue は 3 つのファイルを挙げている。補助の関数を変えた結果としてほかのテストが速くなることは妨げない。
-   **`make test`・CI の設定の変更:** `Makefile` のテストのターゲットと `.github/workflows/` は変えない（AC-09）。理由: issue はテストのコードの変更で短縮することを求めており、テストの選び方を変えると、確かめる範囲が変わる。
-   **末端のサブテストの時間の合計を目標にすること:** 並列の取り合いで膨らむ値なので、目標にしない（§1 の決定）。計測の記録には参考として含めてよい。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 2 つのパッケージの壁時計の時間を半分以下にする

同じ機械と条件で測った `cmd/yt2column` と `internal/mergepr` の壁時計の時間を、変更の前の半分以下にする。

**計測の条件:** 同じ機械で、ほかの重い処理を走らせずに、`go test -tags test -race -count=1 ./cmd/yt2column ./internal/mergepr` を流す。変更の前（このタスクの最初の実装の PR の、`origin/main` との merge-base）と変更の後（その PR の head）で 3 回ずつ流し、`go test` が出すパッケージごとの時間の中央値を比べる。

**Acceptance Criteria**:
- **AC-01**: 計測の条件で、変更の後の `cmd/yt2column` の時間の中央値が、変更の前の中央値の 50% 以下である。
- **AC-02**: 計測の条件で、変更の後の `internal/mergepr` の時間の中央値が、変更の前の中央値の 50% 以下である。
- **AC-03**: 計測の記録が、実装計画（`03_implementation_plan.md`）の受け入れ基準の検証の節にある。記録は、計った機械（OS・CPU のコア数・`PATH` の bash と `/bin/bash` の版）、前と後のコミットの ID、6 回すべてのパッケージごとの時間、中央値、割合を持つ。

#### F-002: 検証の質を保つ

3 つのファイルのテストが確かめていることを、変更の後も減らさない。

**Acceptance Criteria**:
- **AC-04**: `/bin/bash` が `PATH` の bash と異なる 3.x である macOS の機械で、3 つのファイルにあるテスト関数とその下の末端のサブテストのうち、合格したものの名前の集まり（`go test -json` の `pass` の事象の `Test` の値）を変更の前と後で比べると、差が空である。
- **AC-05**: AC-04 の機械で、`review_records_test.go` の各末端のケースが、`path-bash` と `bin-bash-3` の両方の名前の下で走る。`/bin/bash` が 3.x でないか `PATH` の bash と同じ機械では、今と同じく `path-bash` だけで走る（`cmd/yt2column/test_helpers_bash.go:47-68`）。`high_review_decision_test.go` は、今と同じく bash 4 以上の 1 本だけで走る。
- **AC-06**: `make coverage-compare BASE=<変更の前のコミット>` を変更の後の head で流すと、最後の行が `coverage-compare: PASS` である（モジュールの製品コードのどの関数の文のカバレッジも下がらない）。
- **AC-07**: 各ケースの前提の確認（今のコードで、前提が崩れたときに「so the case checks nothing」などの理由でケースを失敗させる確認。例: `requireProbe`、`checkConfigScenarios` の設定の前の出力と plain git の出力の確認、`mainMerge` の親の確認）は、変更の後も、前提が崩れたときにケースを失敗させる。確かめは、各種類の確認について、前提を崩す変更を一時的に入れてケースが失敗することによる。
- **AC-08**: 共通の初期状態や共有する実行の結果を作れなかったとき（git や bash の失敗）、それに依る各ケースは失敗として報告され、合格や skip としては報告されない。

#### F-003: 振る舞いと範囲を変えない

テストの結果がテストの順序や選び方に依らないことと、変えるファイルの範囲を定める。

**Acceptance Criteria**:
- **AC-09**: このタスクの実装の PR の差分（`origin/main` との merge-base から head まで）は、`test` のビルドタグを持つファイルと `_test.go` のファイル（`cmd/yt2column/`・`internal/mergepr/` と、共有の補助を置く場合は `testutil/` の下）と、`docs/tasks/0127_git_fixture_test_speedup/` の下のファイルに限られる。
- **AC-10**: 3 つのファイルの任意の末端のサブテストを `-run` で 1 つだけ選んで走らせても合格する。`-shuffle=on` を付けて 2 つのパッケージを走らせても合格する。
- **AC-11**: 変更の後の head で、`make test` と `make lint` が通る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   F-001 のとおり。`CLAUDE.md`「Performance」の「Profile before optimizing」に従い、設計は手段を選ぶ前に、各テストの時間の内訳（フィクスチャの作成・枝の組み立て・スクリプトの実行・Go の処理）を測る（[design_handoff.md](design_handoff.md) の H-01）。

### 4.2. セキュリティ (Security)

-   N/A。変わるのはテストのコードだけで、`yt-dlp` を呼ばず、API キーを扱わず、キャッシュのディレクトリを読み書きせず、ネットワークに送らず、信頼できない文字列をプロンプトや記事に埋め込まない（`docs/dev/security.md` を当てはめる条件に当たらない）。テストは今と同じく、一時ディレクトリの中で git と bash を走らせる。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   テストは、順序や並列の度合いに依らずに同じ結果を出す（AC-10）。共有する状態を作れなかったときは失敗させ、確かめずに合格させない（AC-08）。

### 4.4. 互換性 (Compatibility)

-   macOS（`/bin/bash` が 3.2）と Linux（CI の ubuntu）の両方で、今と同じ bash の組でテストが走る（AC-05）。

### 4.5. 保守性 (Maintainability)

-   各ケースの期待する出力と失敗の条件が変わっていないことは、PR のレビューで読む。設計と実装は、レビューで読める形（ケースの表の行と期待する出力を、今と同じく 1 か所に保つなど）を保つ。
-   共有の初期状態を複製する仕組みは、`CLAUDE.md`「Performance」の「An optimization that adds a correctness obligation」に当たる（複製した状態が、作り直した状態と同じであること）。その義務をコミットのメッセージに書き、テストで固定する。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   `CLAUDE.md`「Testing Strategy」に従う。各テストは、述べた理由で失敗できる。テストを削除する場合は、削除だけのコミットとカバレッジの比較が要る（本書はケースを減らさないので、削除は想定しない）。
-   `CLAUDE.md`「Performance」の「Optimizations go in their own commit」に従い、高速化の変更は、振る舞いを変える変更と同じコミットに入れない。

## 6. 用語集 (Glossary)

-   **フィクスチャのリポジトリ:** テストが一時ディレクトリに作る git のリポジトリ（`newGitFixture`・`newRRFixtureWithContext`・`newTakeIn`）。
-   **末端のサブテスト:** その下にサブテストを持たないテスト。`go test -json` の事象の `Test` の値で名前を表す。
-   **壁時計の時間:** `go test` がパッケージごとに出す、パッケージのテストの開始から終わりまでの時間。
-   **計測の条件:** F-001 に定める、変更の前と後の時間を比べる条件。
-   用語は `docs/translation_glossary.md` と統一する。
