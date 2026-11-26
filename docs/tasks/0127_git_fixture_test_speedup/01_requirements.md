# 要件定義書：review_records・high_review_decision・mergemain のテストの高速化

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Process | `full` |
| Created | 2026-11-26 |
| Review date | 2026-11-26 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、Go のテストのうち、時間の大半を占める 3 つのファイル（`cmd/yt2column/review_records_test.go`・`cmd/yt2column/high_review_decision_test.go`・`internal/mergepr/mergemain_test.go`）のテストを、検証の質を保ったまま速くするための要件を定義する（[#1168](https://github.com/isseis/yt2column/issues/1168)）。

**背景:**

issue #1168 の計測（`go test -tags test -race -count=1 ./...`）では、次の 3 つのファイルが、それぞれのパッケージの実時間の主な原因になっている。

| ファイル | 末端のサブテスト（§6 で定義）の時間の合計 | サブテスト数（最上位 / 末端） | パッケージの実時間 |
|---|---:|---:|---:|
| `cmd/yt2column/review_records_test.go` | 2659.6 秒 | 8 / 734 | 387.5 秒 |
| `internal/mergepr/mergemain_test.go` | 881.9 秒 | 28 / 112 | 128.7 秒 |
| `cmd/yt2column/high_review_decision_test.go` | 170.9 秒 | 2 / 102 | 387.5 秒 |

今のテストの作りは次のとおりである。

-   **フィクスチャのリポジトリの作り直し:** `review_records_test.go` は、ケースごとに `newRRFixtureWithContext`（`cmd/yt2column/review_records_test.go:136-152`）で `git init` から始め、初期のコミット 2 つ、`origin/main`、`feature` のブランチを作る。`internal/mergepr` は、テストごとに `newTakeIn`（`internal/mergepr/test_helpers_git.go:245-270`）で bare の origin を作って clone する。
-   **git の設定のシナリオでの 2 回の実行:** `checkConfigScenarios`（`cmd/yt2column/review_records_test.go:805-834`）は、2 つのスコープ（リポジトリ・グローバル）と 8 つの設定のすべての組み合わせについて、フィクスチャとブランチを作り、設定の前と後でスクリプトを 1 回ずつ走らせる。フィクスチャの git は、コミットの作者とコミッターの名前は固定するが、日時は固定しない（`cmd/yt2column/test_helpers_gitfixture.go:56-58`）。そのため、別々に作ったフィクスチャのコミットの ID は一致しない。スクリプトの引数と出力にはコミットの ID が含まれる（例: `rrUnreviewedConfigBranch`、`cmd/yt2column/review_records_test.go:972-981`）ので、設定の前の実行の出力が同じになるのは、同じリポジトリの中だけである。
-   **2 本の bash での実行:** bash を 2 本走らせるのは `review_records_test.go` だけである。`scriptBashes`（`cmd/yt2column/test_helpers_bash.go:47-68`）は、`/bin/bash` が `PATH` の bash と異なる 3.x のとき（macOS）に、両方を返す。`review-records.sh` は bash 3.2 より新しい機能を使わないと明記している（`.claude/commands/_lib/review-records.sh:76-77`）。`high_review_decision_test.go` は、bash 4 以上の 1 本だけで走る（`requireBash4`、`cmd/yt2column/high_review_decision_test.go:221`）。CI のジョブは ubuntu だけで走る（`.github/workflows/ci.yml:14,152,176,219,279,297` の `runs-on: ubuntu-latest`）ので、bash 3.2 での確認は macOS の手元の `make test` にしかない。

本書を書く際に、手元（12 コアの macOS、`PATH` の bash は 5.3、`/bin/bash` は 3.2）で、コミット `6c60f9f8` の木の上で次を測った。内訳は、一時的に置いた計測用のテスト（コミットしていない）で、`newRRFixture`・`rrConfigBranch`・`runScript` の前後の時刻を記録して求めた。

-   git の設定のシナリオ 1 件（`TestReviewRecordsCoverage/path-bash/git_config/repository/diff.noprefix`）を単独で走らせると 2.14 秒だった。その内訳は、フィクスチャの作成が約 0.24 秒、ブランチの組み立て（`rrConfigBranch`）が約 0.44 秒、`review-records.sh` の 1 回の実行が約 0.93 秒（`PATH` の bash でも `/bin/bash` でも同じ）である。issue の計測で同じ種類のシナリオが 15〜16 秒かかったのは、並列実行での CPU の取り合いのためと推測する（確かめていない）。
-   したがって、時間の大半はスクリプトの実行であり、issue が挙げる原因（フィクスチャの作り直し）は一部にとどまる。この計測から、issue の改善方針 1・2（テンプレートのリポジトリの複製）だけでは、後述の目標（F-001）に届かないおそれがあることが分かる。
-   `go test -tags test -race -count=1 ./cmd/yt2column ./internal/mergepr`（2 つのパッケージだけ）を 1 回流すと、`cmd/yt2column` が 261.4 秒、`internal/mergepr` が 105.0 秒だった。issue の `./...` での値より短いのは、ほかのパッケージと CPU を取り合わないためと推測する。`cmd/yt2column` には、3 つのファイルのほかにも git のフィクスチャや bash を使うテスト（`cross_check_state_docs_test.go`・`adr_candidates_docs_test.go`）があり、それらは本タスクの短縮の対象にしない。その時間はまだ測っておらず、目標に届くかどうかの見込みは、設計で見積もる（[design_handoff.md](design_handoff.md) の H-02）。

対応 issue: [#1168](https://github.com/isseis/yt2column/issues/1168)

**本書の記述範囲:** 本書は、短縮の目標と測り方、保つべき検証の質、変えてよいファイルの範囲を定める。短縮の手段（テンプレートのリポジトリの複製、実行の結果の共有、並列の度合いなど）は、設計（`02_architecture.md`）で選ぶ。要件の起草で挙がった設計の事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #1168 で決めたこと:**

-   **対象:** 上の 3 つのファイルのテストと、その補助の関数を対象にする。
-   **方針の候補:** 共通の初期状態をテンプレートのリポジトリとして 1 回だけ作り、ケースはその複製から始めること。git の設定のシナリオの準備を最適化すること。bash 3 の二重の実行の要否を整理すること。
-   **完了の条件:** `review-records.sh`・`high-review-decision.sh`・`mergemain` の検証の質（bash 3 での互換性と、git の設定に左右されないことの確認を含む）を保つ。`make test` と `make lint` が通る。

**要件定義の際に決めたこと（2026-11-26）:**

-   **進め方:** 通常の進め方にする。変わるのはテストだけだが、`internal/mergepr` のテストは、開発ツールの信頼境界を守る `mergepr`（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）の検証である。簡易な進め方から外す条件（`docs/dev/conventions/requirements_process.md`、「Simple Process」の「Excluded changes」）に当たるかどうかは判断が分かれる程度なので、迷うときの規則に従って、外れる変更と判断した。さらに、新しいテストの仕組みを作るので、決めることも多い。この判断のとおり、人は通常の進め方を選んだ。ADR はなし（進め方の選択は、このタスクの中で閉じる）。
-   **短縮の目標:** 同じ機械で、基準の半分以下にする。同じ機械と条件で変更の前と後を測り、`cmd/yt2column` と `internal/mergepr` の実時間を、どちらも前の 50% 以下にする（F-001）。理由: issue の「大幅に短縮」は測れないので、数値にする。実時間はほかのパッケージの負荷で変わるので、機械ごとの絶対の秒数ではなく、同じ条件で比べた割合で決める。末端のサブテストの時間の合計は、並列実行での CPU の取り合いで膨らむ値なので、目標にしない。ADR はなし（このタスクの目標で、ほかのタスクを縛らない）。
-   **計測の方法:** 2 つのパッケージを 3 回ずつ流し、中央値で比べる。同じ機械で `go test -tags test -race -count=1 ./cmd/yt2column ./internal/mergepr` を変更の前と後で 3 回ずつ流し、パッケージごとの中央値を比べる（AC-03）。理由: 2 つのパッケージだけを流せば、ほかのパッケージの負荷によるぶれが減る。1 回だけでは負荷のぶれを受けやすいので、3 回の中央値にする。`make test` と同じ `-race` を付ける。実装の PR が 2 つ以上になるときは、PR ごとにその merge-base と比べ、PR ごとの割合を掛け合わせる（F-001）。これは、起草の後のレビューで、ほかのタスクが同じ期間に足したテストや製品コードの変更が比較に入る問題が見つかったために、作成者が足した決め方で、人の答えはまだない（PR のレビューで確かめる）。ADR はなし（このタスクの測り方である）。
-   **bash 3 での二重の実行:** 全件で残す。`review_records_test.go` は、今と同じく、`scriptBashes` が返すすべての bash で全件を走らせる（AC-05）。理由: bash 3.2 での確認は macOS の手元にしかなく、Go のカバレッジは bash の行を測れないので、代表のケースに絞ったときに互換性の確認が減っていないことを確かめる手段がない。短縮は、実行の結果の共有とフィクスチャの再利用で得る。ADR はなし（今のテストの作りを保つだけである）。
-   **短縮の手段:** 設計に任せる。要件は、目標と、検証の質を保つ条件と、変えてよいファイルの範囲だけを定める。手段は、issue が挙げるテンプレートのリポジトリの複製に限らず、設計で選ぶ（[design_handoff.md](design_handoff.md) の H-01）。理由: 上の計測では時間の大半がスクリプトの実行なので、issue が挙げる手段だけでは目標に届かないおそれがある。ADR はなし（手段の選択はこのタスクの中で閉じる。設計で主要な決定が出れば、`02_architecture.md` で判断する）。
-   **検証の質の確かめ方:** 次の 4 つで確かめる。(1) 末端のサブテストの名前の集まりが、変更の前と後で同じであること（AC-04）。名前の集まりは、変更の前からあった最上位のテスト関数ごとに比べ、新しく足す最上位のテスト関数（複製の義務を固定するテストなど）は比べる対象に含めない。(2) Go モジュールの製品コードのカバレッジが下がらないこと（AC-06）。(3) 各ケースの前提の検査（§6 で定義。「so the case checks nothing」で失敗させる検査）が、前提が崩れたときに今と同じくケースを失敗させること。これは、前提を壊してケースが失敗するのを見て確かめる（AC-07）。(4) 各ケースの期待する出力と失敗の条件が変わっていないことは、PR のレビューで読む（4.5）。理由: 手段を設計に任せるので、テストの形が変わっても、ケースが減っていないことを機械的に確かめられるようにする。名前の集まりを縛ると、設計はケースをまとめたり名前を変えたりできないが、ケースが減ったことを検出できる利点のほうが大きいと人が判断した。ADR はなし（このタスクの受け入れ基準である）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `make test` で最も時間のかかる 2 つのパッケージ（`cmd/yt2column`・`internal/mergepr`）の実時間を、半分以下にする。
-   その際、3 つのファイルのテストが確かめていること（ケースの集まり、bash 3 での実行、git の設定に左右されないことの確認、ケースの前提の検査、各ケースの隔離）を減らさない。

### 2.2. スコープ (In Scope)

-   `cmd/yt2column/review_records_test.go`・`cmd/yt2column/high_review_decision_test.go`・`internal/mergepr/mergemain_test.go` のテストと、それらが使う補助の関数（`cmd/yt2column/test_helpers_*.go`・`internal/mergepr/test_helpers*.go` など、`test` のビルドタグか `_test.go` のファイルにあるもの）の変更。
-   短縮の前と後の計測と、その記録。

### 2.3. スコープ外 (Out of Scope)

-   **製品コードとスクリプトの変更:** `.claude/commands/_lib/review-records.sh`・`high-review-decision.sh`、`internal/mergepr` の製品コード、ほかの製品コードは変えない（AC-09）。理由: issue はテストの高速化を求めており、検証の対象を変えると「検証の質を保つ」の比較が成り立たない。
-   **bash 3 での実行の削減:** 全件の二重の実行を残す（§1 の決定）。
-   **ほかのテストファイル:** `scriptBashes` を使う `cmd/yt2column/adr_candidates_docs_test.go:373` や、ほかのパッケージのテストは対象にしない。理由: issue は 3 つのファイルを挙げている。補助の関数を変えた結果としてほかのテストが速くなることは妨げない。補助の関数を共有するほかのテストの末端の名前の集まりも、AC-04 で比べる。
-   **`make test`・CI の設定の変更:** `Makefile` のテストのターゲットと `.github/workflows/` は変えない（AC-09）。理由: issue はテストのコードの変更で短縮することを求めており、テストの選び方を変えると、確かめる範囲が変わる。
-   **末端のサブテストの時間の合計を目標にすること:** 並列実行での CPU の取り合いで膨らむ値なので、目標にしない（§1 の決定）。計測の記録には参考として含めてよい。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 2 つのパッケージの実時間を半分以下にする

計測の条件で測った `cmd/yt2column` と `internal/mergepr` の実時間を、変更の前の半分以下にする。

**計測の条件:**

-   **機械:** `/bin/bash` が `PATH` の bash と異なる 3.x である macOS の機械（今の確認の範囲で bash の実行が最も多い環境）。前と後を同じ機械で、ほかの重い処理を走らせずに測る。
-   **コマンド:** `go test -tags test -race -count=1 ./cmd/yt2column ./internal/mergepr`。`go test` がパッケージごとに出す時間を読む。
-   **比べる組:** このタスクの実装の PR ごとに、その PR の `origin/main` との merge-base（BASE）と、その PR のテストの変更をすべて含む head 側のコミット（AFTER）を比べる。BASE は計測の時点で決める（`origin/main` を取り込んだ後は、取り込んだ後の merge-base）。計測の記録を書くコミットは、AFTER とはタスクのディレクトリの下だけが異なる。
-   **回数:** PR ごとに、BASE と AFTER で 3 回ずつ流し、パッケージごとの中央値を比べる。PR ごとの割合（AFTER の中央値 ÷ BASE の中央値）を、このタスクのすべての実装の PR について掛け合わせた値を、このタスクの割合とする（実装の PR が 1 つなら、その PR の割合）。PR ごとに比べるのは、ほかのタスクが同じ期間に足したテストや製品コードの変更を、比較に入れないためである。

**Acceptance Criteria**:
- **AC-01**: 計測の条件で、このタスクの `cmd/yt2column` の割合が 50% 以下である。
- **AC-02**: 計測の条件で、このタスクの `internal/mergepr` の割合が 50% 以下である。
- **AC-03**: 計測の記録が、実装計画（`03_implementation_plan.md`）の受け入れ基準の検証の節にある。記録は、測った機械（OS・CPU のコア数・`PATH` の bash と `/bin/bash` の版・Go の版・`GOMAXPROCS` を設定したか）、実装の PR ごとの BASE と AFTER のコミットの ID、BASE と AFTER の 6 回すべての、パッケージごとの時間、中央値、PR ごとの割合、このタスクの割合を持つ。

#### F-002: 検証の質を保つ

3 つのファイルのテストが確かめていることを、変更の後も減らさない。

**Acceptance Criteria**:
- **AC-04**: 計測の条件の機械で、実装の PR ごとに、`go test -tags test -race -count=1 -json ./cmd/yt2column ./internal/mergepr` をその PR の BASE と AFTER で流す。`Action` が `pass` の事象の `Test` の値から、末端の名前（ほかのどの名前も、その名前に `/` を足したもので始まらない名前）を取り出す。BASE にあった最上位のテスト関数のそれぞれについて、その関数自身の名前、またはその名前に `/` を足したもので始まる末端の名前の集まりが、BASE と AFTER で同じである。AFTER で新しく足した最上位のテスト関数は、比べる対象に含めない。比べる最上位のテスト関数の下では、BASE と AFTER のどちらにも `fail` の事象がなく、`skip` の事象の `Test` の値の集まりも BASE と AFTER で同じである（失敗や skip のケースが名前の集まりから黙って抜けないようにするため）。
- **AC-05**: 計測の条件の機械で、bash ごとのサブテストを作る `TestReviewRecordsCoverage`・`TestReviewRecordsUnreviewed`・`TestReviewRecordsCrossCheck`・`TestReviewRecordsFableAlias` の各末端のケースが、`path-bash` と `bin-bash-3` の両方の名前の下で合格し、それぞれの名前の下のケースが確かめるスクリプトの出力は、その名前の bash（`PATH` の bash か `/bin/bash`）で実際に走らせた実行の出力である。ある bash の実行の結果を、ほかの bash の名前の下のケースに使わない。`TestReviewRecordsMode` は、今と同じく `scriptBashes` が返す各 bash でスクリプトを走らせる（`cmd/yt2column/review_records_test.go:1036-1046`）。`/bin/bash` が 3.x でないか `PATH` の bash と同じ機械では、今と同じく `PATH` の bash だけで走る（`cmd/yt2column/test_helpers_bash.go:47-68`）。`high_review_decision_test.go` は、今と同じく bash 4 以上の 1 本だけで走る（`cmd/yt2column/high_review_decision_test.go:221`）。
- **AC-06**: 実装の PR ごとに、`make coverage-compare BASE=<その PR の BASE>` をその PR の AFTER で流すと、最後の行が `coverage-compare: PASS` である。この AC が確かめるのは Go の製品コード（主に `internal/mergepr`）のカバレッジだけで、bash のスクリプトの行は確かめない。`cmdrunner` の signalGroup のように時間で揺れる関数で `FAIL` になったときは、きれいな作業ツリーで流し直し、すべての実行の結果を記録に挙げる。
- **AC-07**: BASE の 3 つのファイルには、メッセージに `checks nothing` を含む `t.Fatal`・`t.Fatalf`・`t.Error`・`t.Errorf` の呼び出しがある。これがケースの前提の検査である。`6c60f9f8` の時点で、`review_records_test.go` と `high_review_decision_test.go` に計 13 か所（コメントを除く）あり、`mergemain_test.go` にはない。そのそれぞれについて、AFTER でも同じ前提を検査する。前提が崩れたときは、その前提に依るケースが失敗する。確かめは、各検査について前提を崩す変更を一時的に入れ、ケースが失敗することを見て行う。結果は、`CLAUDE.md`「Testing Strategy」のとおり、その検査か、その検査が確かめるフィクスチャや実行を変えたコミットのメッセージに書く。
- **AC-08**: 共通の初期状態や共有する実行の結果を作れなかったとき（git や bash の失敗）、それに依るケースは合格や skip として報告されず、そのパッケージの `go test` は失敗する（失敗は、ケースそのものでなく、共有の状態を作る親のテストの失敗として報告されてもよい）。

#### F-003: 隔離と範囲を変えない

各ケースの隔離、テストの結果が順序や選び方に依らないこと、変えるファイルの範囲を定める。

**Acceptance Criteria**:
- **AC-09**: このタスクの実装の PR の差分（それぞれの PR の、`origin/main` との merge-base から head まで）は、次のファイルに限られる。
    -   `cmd/yt2column/`・`internal/mergepr/` の、`_test.go` のファイルと `//go:build test` のファイル。
    -   `test` のビルドタグのファイルだけを持つ、テストの補助のパッケージを新しく作るときは、そのファイルと、`docs/dev/conventions/package_reference.md` のそのパッケージの行と、`internal/pipeline/pipeline_test.go` の、テストだけのパッケージを数える箇所（`testOnlyPackageDirs` と `TestFakesCarryBuildTag` の `testutil` のファイルの数、`internal/pipeline/pipeline_test.go:535-585`）。
    -   `docs/tasks/0127_git_fixture_test_speedup/` の下のファイル。
- **AC-10**: 各ケースは、今と同じ隔離を保つ。`cmd/yt2column` のフィクスチャのケースは自分の `HOME`・`TMPDIR`（`cmd/yt2column/test_helpers_gitfixture.go:30-46`）を、`internal/mergepr` のケースは自分の `HOME`・`XDG_CONFIG_HOME`（`internal/mergepr/test_helpers_git.go:247-255`）を持ち、どちらも自分のリポジトリを持つ。あるケースが書いた git の設定（リポジトリ・グローバル）やファイルは、ほかのケースから読めない。
- **AC-11**: AFTER で、3 つのファイルの最上位のテスト関数のそれぞれについて、末端のサブテストの名前（AC-04 の BASE の名前の集まり）を名前の順に並べ、最初のものと最後のものを、それぞれ単独で選んで走らせると、どちらも合格する。さらに、共有する状態（共通の初期状態や共有する実行の結果）の種類ごとに、それを使う末端のサブテストのうち、名前の順で最初でも最後でもないものを 1 つ単独で選んで走らせても合格する。選ぶには、各階層を `^...$` で囲み、正規表現の記号を逃がした `-run` を使う。`-shuffle=on` と `-json` を付けて AC-04 の 2 つのパッケージを 2 回走らせると合格し、記録に、各実行の各パッケージのシード（`-test.shuffle` の行）を挙げる。
- **AC-12**: AFTER で、`make test` と `make lint` が通る。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   F-001 のとおり。`CLAUDE.md`「Performance」の「Profile before optimizing」に従い、設計は手段を選ぶ前に、各テストの時間の内訳（フィクスチャの作成・ブランチの組み立て・スクリプトの実行・Go の処理）を測る（[design_handoff.md](design_handoff.md) の H-01）。

### 4.2. セキュリティ (Security)

-   N/A。変わるのはテストのコードだけで、`yt-dlp` を呼ばず、API キーを扱わず、キャッシュのディレクトリを読み書きせず、ネットワークに送らず、信頼できない文字列をプロンプトや記事に埋め込まない（`docs/dev/security.md` の規則を適用する場面にならない）。`docs/dev/security.md` の規則も変えない。テストは今と同じく、一時ディレクトリの中で git と bash を走らせる。`mergepr` のテストが弱まって信頼境界の守りの検証が減ることは、セキュリティの規則ではなく、検証の質の要件（F-002）と各ケースの隔離（AC-10）で防ぐ。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   テストは、順序やケースの選び方に依らずに同じ結果を出す（AC-11）。各ケースは自分の環境を持ち、ほかのケースの設定を読まない（AC-10）。共通の初期状態や共有する実行の結果を作れなかったときは失敗させ、確かめずに合格させない（AC-08）。

### 4.4. 互換性 (Compatibility)

-   macOS（`/bin/bash` が 3.2）と Linux（CI の ubuntu）の両方で、今と同じ bash の組でテストが走る（AC-05）。

### 4.5. 保守性 (Maintainability)

-   各ケースの期待する出力と失敗の条件が変わっていないことは、PR のレビューで読む。設計と実装は、レビューで読める形（ケースの表の行と期待する出力を、今と同じく 1 か所に保つなど）を保つ。
-   共通の初期状態を複製する仕組みは、`CLAUDE.md`「Performance」の「An optimization that adds a correctness obligation」に当たる（複製した状態が、作り直した状態と同じであること）。その義務をコミットのメッセージに書き、テストで固定する。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   `CLAUDE.md`「Testing Strategy」に従う。各テストは、そのテストが確かめると述べたことが崩れたときに失敗するものにする。テストを削除する場合は、削除だけのコミットとカバレッジの比較が要る（本書はケースを減らさないので、削除は想定しない）。
-   `CLAUDE.md`「Performance」の「Optimizations go in their own commit」に従い、共通の初期状態を複製する仕組みと、実行の結果を共有する仕組みは、それぞれ revert で切り離せるコミットに入れ、ケースの表の整理などと同じコミットにしない。

## 6. 用語集 (Glossary)

-   **フィクスチャのリポジトリ:** テストが一時ディレクトリに作る git のリポジトリ（`newGitFixture`・`newRRFixtureWithContext`・`newTakeIn`）。
-   **末端のサブテスト:** その下にサブテストを持たないテスト。`go test -json` の事象の `Test` の値で名前を表す。
-   **実時間:** `go test` がパッケージごとに出す、パッケージのテストの開始から終わりまでの時間（`docs/translation_glossary.md` の「実時間」、wall-clock time）。
-   **ケースの前提の検査:** ケースの組み立てが、確かめたい規則を実際に働かせる形になっていることを検査し、なっていなければ「so the case checks nothing」などの理由でケースを失敗させるテストの中の検査（AC-07）。一時スクリプトの「前提の確認」とは別のものである。
-   **計測の条件:** F-001 に定める、変更の前（BASE）と後（AFTER）の時間を比べる条件。
-   用語は `docs/translation_glossary.md` と統一する。

## 変更履歴

- 2026-11-26 承認（isseis、#1174）
