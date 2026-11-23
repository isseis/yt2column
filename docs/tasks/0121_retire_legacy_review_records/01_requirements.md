# 要件定義書：古い記録行の読み方規則を、対象ブランチがなくなった後に外す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-23 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、タスク 0118（Codex の PR レビューと Codex CLI の撤去）で移行期の後方互換として残した古い記録行（後述の 3 形式。6 節で定義）の読み方規則を、該当する開いたブランチ（`origin` に現存する `main` 以外のリモートブランチ。6 節で定義）がなくなった後に撤去するための要件を定義する。

対応 issue: [#1087](https://github.com/isseis/yt2column/issues/1087)（前提: [#1059](https://github.com/isseis/yt2column/issues/1059)、関連: [#1084](https://github.com/isseis/yt2column/pull/1084)・[#1066](https://github.com/isseis/yt2column/pull/1066)・[#1051](https://github.com/isseis/yt2column/pull/1051)・[#1072](https://github.com/isseis/yt2column/issues/1072)）

**背景:**

タスク 0118（`docs/tasks/0118_codex_removal/01_requirements.md` F-006、AC-28）では、ローカルの CLI レビューを CodeRabbit CLI のみに切り替え、新しく書く記録行を `(coderabbit)`・`(coderabbit: <kind>)` のみに改めた（`.claude/commands/_lib/local-cli-review.md:186,188-189`）。一方で、0118 の変更より前に作られたブランチを壊さないため、次の 3 形式の古い記録行を同じ意味で読み続ける後方互換の規則を残した（`docs/tasks/0118_codex_removal/02_architecture.md` §3.6）。

-   `Codex local review:` で始まる指摘の行（`.claude/commands/_lib/local-cli-review.md:300-301`、`.claude/commands/_lib/task-pr-handoff.md:256-263`、`.claude/commands/_lib/fable-review.md:298-300`）。
-   `(codex)` を持つ見た範囲の行 `Local CLI reviewed: <START12>..<END12> (codex)`（`.claude/commands/_lib/local-cli-review.md:305,314-317`、`.claude/commands/_lib/review-records.sh:192`）。
-   `(coderabbit: <kind>; codex: <kind>)` を持つ未レビュー範囲の行（`.claude/commands/_lib/local-cli-review.md:314-317`、`cmd/yt2column/review_records_test.go:322,328`）。

0118 の設計書（`docs/tasks/0118_codex_removal/02_architecture.md` §3.6・§5.1）は、この読み方規則では記録行が書かれた日時を区別できず、新しく書かれた `(codex)` の行も見た範囲として受理してしまう危険を一時的に受け入れた。そのうえで、古い記録行を持つ開いたブランチがなくなったときに規則を外す方針を定め、フォローアップ issue（#1087）を作成することとした（`docs/tasks/0118_codex_removal/03_implementation_plan.md` ステップ 3-3）。

なお、2026-11-23 時点で `origin` の全ブランチを調査したところ、開いている PR のうち `origin/isseis/0114-fable-record-after-merge-04`（PR #1066）と `origin/isseis/0115-chief-conflict-resolution-02`（PR #1051）の 2 本に、`Local CLI reviewed: ... (codex)` と `Codex local review:` の行が残っている。その他の `origin` ブランチには残っていない。`/runpr`（`.claude/commands/runpr.md:608-618`）、`_lib/task-pr-handoff.md` Part B step 2.5（`.claude/commands/_lib/task-pr-handoff.md:205-209`）、および `_lib/fable-review.md`（`.claude/commands/_lib/fable-review.md:82-92`）は `origin/main` の `review-records.sh` を実行し、正規表現 `SEEN_RE` で見た範囲を判定する（`.claude/commands/_lib/review-records.sh:465,546`）。そのため、該当ブランチが残っている間に本タスクの実装を `main` にマージすると、それらの PR で `(codex)` の行により見た範囲に含まれていたコミットが未見（`review-records.sh` が `PARTIAL` または `MISSING` と判定する状態。6 節で定義）に変わる。

また、関連する並行変更として、PR #1066（タスク 0114、`docs/tasks/0114_fable_record_after_merge/02_architecture.md` §3.1・§3.2）は `review-records.sh` に `unreviewed` モードを追加している。さらに、承認済みのタスク 0120（`docs/tasks/0120_harness_independent_commands/02_architecture.md` §3.5）は `_lib/fable-review.md` を `_lib/cross-check-review.md` へ改名し、`Cross-check finding:` と `review-records.sh` の `cross-check` モードを追加する計画である。

**本書の記述範囲:** 本書は、撤去の前提条件（開いたブランチの確認）、コマンド定義および `review-records.sh` が記録行を受理・拒否する観測可能な振る舞い、ならびにテスト・関連文書に求める状態を定める。次の事項はアーキテクチャ設計書（`02_architecture.md`）で決める。

-   `cmd/yt2column/review_records_test.go` の既存 `(codex)` テストケース（`review_records_test.go:252-256`）を削除して `make coverage-compare` で確かめるか、拒否ケースへ書き換えるかのテスト構成。
-   F-001 の確認手順の具体的なコマンド構成と、PR #1066（タスク 0114）・タスク 0120 の先行マージに伴う `review-records.sh`・コマンド定義の整合方針。
-   既存の文書ガードテストの更新範囲。

設計へ引き継ぐ論点は [design_handoff.md](design_handoff.md) に申し送る。

**issue #1087 で決めたこと:**

-   **撤去の条件:** 古い記録行（`Codex local review:`、`(codex)`、`(coderabbit: <kind>; codex: <kind>)`）を持つ開いたブランチがなくなってから、読み方規則を外す。
-   **対象箇所:** `.claude/commands/_lib/local-cli-review.md` の「Reading the record lines」、`.claude/commands/_lib/task-pr-handoff.md` の Part B step 3、`.claude/commands/_lib/review-records.sh` の正規表現、`cmd/yt2column/review_records_test.go`、および関連するガイドの記述を残す形に合わせる。
-   **テスト削除の扱い:** `cmd/yt2column/review_records_test.go` の `(codex)` 後方互換テストを削除する場合は、削除のみのコミットを分けて `make coverage-compare` で確かめる（[ADR-0037](../../dev/adr/0037-test-deletion-coverage.md)）。

**要件定義の際に決めたこと（2026-11-23）:**

以下の 4 点をレビュアーと確認して決めた。

-   **進め方の選択（通常の進め方）:**
    -   **除外判定:** 簡易な進め方の除外に該当すると判定した。`fixprguard`・`mergepr` 自体は変えないが、[ADR-0013](../../dev/adr/0013-review-setup.md) および `.claude/commands/_lib/pr-review-escalation.md:53-62` が開発ツールの信頼の境界（レビューの関門。6 節で定義）に含める `.claude/commands/_lib/review-records.sh` と記録行の読み方規則を変更するため、`docs/dev/conventions/requirements_process.md`（「Simple Process — Excluded changes」）の「迷う場合は除外とする」規則を適用した。
    -   **決定と理由:** 推奨どおり通常の進め方（`full`）を採る（除外判定は覆さない）。
    -   **ADR の判断:** ADR なし（本タスクの進め方の選択であり、タスク内で閉じるため）。
-   **開いたブランチの確認範囲と実施時点（F-001）:**
    -   **決定と理由:** 開いている PR（現在 PR #1066・PR #1051 に残存）だけでなく、PR を開いていないリモートブランチも含め、`origin` に現存する `main` 以外の全ブランチに古い記録行がないことを実装開始前および実装 PR マージ前の必須条件とする。`review-records.sh` は `origin/main` のものが実行されるため（`.claude/commands/runpr.md:608-618`、`.claude/commands/_lib/task-pr-handoff.md:205-209`、`.claude/commands/_lib/fable-review.md:82-92`）、対象ブランチが残っている間に本タスクをマージすると既存ブランチの判定が変わってしまう。加えて、PR #1066 自体も `review-records.sh` を変更している（`docs/tasks/0114_fable_record_after_merge/02_architecture.md` §3.1・§3.2）。なお、マージ済み PR の古いローカル専用ブランチ（例: PR #1040 マージ後のローカル `0108` ブランチ）は `origin` に存在せず、確認対象に含めない。
    -   **ADR の判断:** ADR なし（本タスクの移行タイミング・完了条件であり、タスク内で閉じるため）。
-   **`fable-review.md` の対象への追加と範囲なし `not run` の除外（F-002）:**
    -   **決定と理由:** `.claude/commands/_lib/fable-review.md:298-300`（タスク 0120 による改名後は `.claude/commands/_lib/cross-check-review.md`）の `Codex local review: (legacy lines from older branches)` は issue #1087 にファイル名の明記がないものの、撤去対象の `Codex local review:` の読み方規則そのものであるためスコープに含める。一方、タスク 0049 以前の形式である範囲なしの `CodeRabbit local review: not run`（`.claude/commands/_lib/local-cli-review.md:297-299`、`.claude/commands/_lib/task-pr-handoff.md:269-271`、`.claude/commands/runplan.md:283`）はタスク 0118 の Codex 関連フォローアップではないためスコープ外に残す。
    -   **ADR の判断:** ADR なし（タスク 0118 の方針の適用であり、既存 ADR の決定を変えないため）。
-   **`SEEN_RE` のみの厳格化（F-003）:**
    -   **決定と理由:** `.claude/commands/_lib/review-records.sh:194` の `NOT_RUN_RE`（`^Local CLI review: not run ([0-9a-f]{12})\.\.([0-9a-f]{12}) \(.*\)$`）は `codex` を明示しておらず、`.claude/commands/_lib/local-cli-review.md:295-296` の記述とも整合しているため変更しない。`review-records.sh:192` の `SEEN_RE` のみを `(coderabbit)` に絞る。
    -   **ADR の判断:** ADR なし（既存スクリプトの 1 つの正規表現の扱いであり、タスク内で閉じるため）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   古い記録行を持つ開いたブランチが `origin` に残っていないことを確認したうえで、移行期の後方互換規則を安全に撤去する。
-   コマンド定義と `review-records.sh` が、ローカルの CLI レビュー記録行として現行の CodeRabbit CLI の形式のみを受理する状態にする。
-   0118 で一時的に受け入れていた「新しく書かれた `(codex)` の行が見た範囲として受理される」状態を解消する。

### 2.2. スコープ (In Scope)

-   `origin` に現存する `main` 以外の全ブランチに古い記録行が残っていないことの確認と記録（F-001）。
-   コマンド定義（`.claude/commands/_lib/local-cli-review.md`・`.claude/commands/_lib/task-pr-handoff.md`・`.claude/commands/_lib/fable-review.md` [タスク 0120 による改名後は `_lib/cross-check-review.md`]）からの古い記録行の読み方規則の撤去（F-002）。
-   `.claude/commands/_lib/review-records.sh` の `SEEN_RE` から `codex` を外し、`(coderabbit)` のみを受理するようにすること（F-003）。
-   `cmd/yt2column/review_records_test.go` のテスト・コメント・テスト入力と、関連するガイド文書・規約文書（該当箇所がある場合）の整合（F-004）。

### 2.3. スコープ外 (Out of Scope)

-   **範囲なしの `CodeRabbit local review: not run`（タスク 0049 以前の旧形式）:** `.claude/commands/_lib/local-cli-review.md:297-299`、`.claude/commands/_lib/task-pr-handoff.md:269-271`、`.claude/commands/runplan.md:283` の記述は変更しない（要件定義の際に決めたこと）。
-   **`review-records.sh` の `NOT_RUN_RE` の厳格化:** `.claude/commands/_lib/review-records.sh:194` の `\(.*\)` は変更しない（要件定義の際に決めたこと）。
-   **`docs/tasks/` および `docs/research/` の過去の文書:** 過去のタスクの決定・調査の記録として変更しない。
-   **`internal/chief/testdata/` などのテスト用サンプルおよび `fixprguard` の `@codex` メンション拒否:** 0118 で残すと決めた仕様（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)、0118 F-009・F-012）であり、本タスクの対象外とする。
-   **既存 ADR の変更:** いずれの有効な ADR の決定も変えないため、ADR は作成・更新しない。
-   **PR #1066（タスク 0114）・PR #1051（タスク 0115）自体のマージまたはクローズ作業:** それぞれのタスクで進める。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

本節は、前提条件の確認（F-001）、コマンド定義の更新（F-002）、判定スクリプトの厳格化（F-003）、およびテスト・関連文書の整合（F-004）の 4 つの機能要件を定める。

#### F-001: 古い記録行を持つ開いたブランチがないことの確認

本タスクの実装を開始する前、および実装 PR をマージする前に、最新の `origin` に現存する `main` 以外のすべてのリモートブランチ（削除済みブランチは除く）について、`origin` の `main` から到達できないコミットに古い記録行が存在しないことを確かめる。対象の 3 形式は、コミットメッセージの 1 列目から始まる次の行とする。

1. `Codex local review:` で始まる行。
2. `Local CLI reviewed: <12 桁の小文字 16 進>..<12 桁の小文字 16 進> (codex)` の行。
3. `Local CLI review: not run <12 桁の小文字 16 進>..<12 桁の小文字 16 進> (` で始まり、括弧内に `codex:` を含む行（`(coderabbit: <kind>; codex: <kind>)` の旧形式）。

**Acceptance Criteria**:
- **AC-01**: 実装開始前と実装 PR のマージ前の**それぞれの時点**で、最新の `origin` に現存する `main` 以外のすべてのリモートブランチについて `main` から到達できないコミットのメッセージを検査し、上記 3 形式に一致する行を持つブランチが 0 件であることを確認する。2 つの時点の**それぞれ**について、確認日・検査したブランチ一覧・0 件である結果が記録されている（実装開始前の確認は実装計画書または PR の説明に、マージ前の確認は PR の説明またはマージの記録に残す）。
- **AC-02**: 上記 3 形式のいずれかを含むコミットが `origin` の対象ブランチに 1 件以上残っている間は、本タスクの実装を開始せず、実装 PR もマージしない（該当するブランチ名と、一致した記録行を報告して作業を止める）。また、`origin` の取得またはブランチ・コミットメッセージの読み取りに失敗した場合は、0 件とみなさず安全側に倒して停止し、失敗した操作と診断内容を報告する。

#### F-002: コマンド定義からの古い記録行の読み方規則の撤去

ローカルの CLI レビュー記録行を読むコマンド定義から、古い記録行の 3 形式を読み取る規則と退役条件の記述を外す。対象ファイルは `.claude/commands/_lib/local-cli-review.md`、`.claude/commands/_lib/task-pr-handoff.md`、および `.claude/commands/_lib/fable-review.md`（タスク 0120 `docs/tasks/0120_harness_independent_commands/02_architecture.md` §3.5 により実装前に改名されていた場合は `.claude/commands/_lib/cross-check-review.md`）の 3 ファイルとする。

**Acceptance Criteria**:
- **AC-03**: `.claude/commands/_lib/local-cli-review.md` の「Reading the record lines」節（現状 `:286-317`）が、指摘の行の接頭辞を `CodeRabbit local review:` のみ、見た範囲の行の形を `Local CLI reviewed: <START12>..<END12> (coderabbit)` のみと定めており、`Codex local review:`・`(codex)`・`(coderabbit: <kind>; codex: <kind>)` を読み取る記述と、古い記録行の退役条件を述べる段落を持たない。
- **AC-04**: `.claude/commands/_lib/task-pr-handoff.md` の Part B step 3 の指摘の転記に関する項目（現状 `:252-263`）から、`Codex local review:` および Codex CLI の指摘に関する記述（場所と自分の要約のみで記し本文を転記しない旨の言及を含む）が外れており、Codex 以外の指摘の行（現状の `CodeRabbit local review:`・`Fable finding:` およびタスク 0120 で追加されるクロスチェックの指摘の行を含む）の読み取りと本文転記禁止の規則が維持されている。
- **AC-05**: `.claude/commands/_lib/fable-review.md`（タスク 0120 による改名後は `.claude/commands/_lib/cross-check-review.md`）の「The prompt」節の「Settled issues」（現状 `:298-300`）から `Codex local review:` の記述が外れており、Codex 以外の却下済み指摘（現状の `CodeRabbit local review:`・`Fable finding:` およびタスク 0120 で追加されるクロスチェックの指摘の行を含む）が読み取り対象として維持されている。
- **AC-06**: `.claude/commands/_lib/local-cli-review.md`・`.claude/commands/_lib/task-pr-handoff.md`・`.claude/commands/_lib/fable-review.md`（タスク 0120 による改名後は `.claude/commands/_lib/cross-check-review.md`）の変更差分は、AC-03〜AC-05 の古い記録行の読み方規則の撤去とそれに伴う文言調整に限られる。

#### F-003: `review-records.sh` の見た範囲の行の厳格化

`.claude/commands/_lib/review-records.sh` で見た範囲の行（`Local CLI reviewed:`）を読むすべてのモード（現行の `coverage`・`fable` に加え、実装時点で存在するタスク 0114 の `unreviewed` モードやタスク 0120 の `cross-check` モードを含む）について、受理するレビュアー名を `coderabbit` のみに限定し、`(codex)` を含むそれ以外の形を受理しないようにする。`Local CLI reviewed:` の行が無視されてもスクリプトは失敗しない。また、無視された行があっても、同じコミットまたは同じブランチ上の有効な `(coderabbit)` の行や `Local CLI review: not run` の行の判定には影響しない。`NOT_RUN_RE`（`Local CLI review: not run <START12>..<END12> (...)`）は変更しない。

**Acceptance Criteria**:
- **AC-07**: `.claude/commands/_lib/review-records.sh` の `coverage` モード（および実装時点で `Local CLI reviewed:` を読む他の `ALL`/`PARTIAL` モード）で、1 列目から始まる `Local CLI reviewed: <12 桁の小文字 16 進>..<12 桁の小文字 16 進> (coderabbit)` の行が既存の `START`/`END` 検証を満たすとき、その範囲のコミットを見たものとして扱い、他に未見のコミットがなければ `ALL` を出力し終了コード 0 で終わる。
- **AC-08**: `.claude/commands/_lib/review-records.sh` の `coverage` モード（および実装時点で `Local CLI reviewed:` を読む他の `ALL`/`PARTIAL` モード）で、`Local CLI reviewed: <12 桁の小文字 16 進>..<12 桁の小文字 16 進> (codex)` の行、および `(coderabbit)` に完全一致しないレビュアー表記（大文字小文字の違い、余分な空白、他のレビュアー名を含む）を持つ行は、補正も読み替えもされずに無視され、スクリプトは終了コード 0 で終わる。無視された行の範囲にしか含まれないコミットは未見として `PARTIAL <n>` に列挙される。また、無視された行があっても、同じコミットまたは同じブランチ上の有効な `(coderabbit)` の行の判定には影響しない。
- **AC-09**: `.claude/commands/_lib/review-records.sh` の `fable` モード（タスク 0120 による追加後は `cross-check` モードを含む）で、部分レビューを記録したコミット上の `Local CLI reviewed: <12 桁の小文字 16 進>..<12 桁の小文字 16 進> (codex)` の行は、同じ実行の修正コミットの範囲として受理されない。その結果、他の規則に該当しない修正コミットがあれば、スクリプトは `MISSING <START>` を出力して終了コード 0 で終わる。一方、無視された行があっても、同じコミット上の有効な `(coderabbit)` の行や `Local CLI review: not run <12 桁の小文字 16 進>..<12 桁の小文字 16 進> (...)` の行による同じ実行の修正コミット判定には影響しない。

#### F-004: テストと関連文書の整合

`cmd/yt2column/review_records_test.go` および関連するガイド文書・規約文書から古い記録行の前提を取り除き、残す形式（CodeRabbit CLI のみ）に合わせる。

なお、2026-11-23 時点の横断検索では、`docs/dev/guides/`・`docs/dev/conventions/`・`CLAUDE.md` に撤去対象の 3 形式を現行の読み方規則として記述した箇所は見つかっていない。実装開始時点で再度確認し、その間に追加された箇所があれば合わせて更新する。

**Acceptance Criteria**:
- **AC-10**: `cmd/yt2column/review_records_test.go` に、`(codex)` を見た範囲として受理する期待値（現状 `:252-256`）、旧形式の `not run` 入力文字列 `(coderabbit: other; codex: timeout)`（現状 `:322,328`）、および Codex の PR レビューを前提とするコメント（現状 `:279`）が残っておらず、`not run` のテスト入力は現行の CodeRabbit 単体の形式にそろっている。
- **AC-11**: `docs/dev/guides/`・`docs/dev/conventions/`・`CLAUDE.md` に、撤去対象の 3 形式（`Codex local review:`、`(codex)`、`(coderabbit: <kind>; codex: <kind>)`）を現行の読み方規則として記述した箇所が存在しない。
- **AC-12**: 実装後に `make test`・`make lint`・`make deadcode` および CI の `doc-guard` ジョブがすべて成功する。また、Go のテストを削除する場合は、削除のみのコミットを分けたうえで、`make coverage-compare BASE=<親>` が `PASS` となり、最終行（`coverage-compare: PASS`）までの出力がコミットメッセージと PR の説明に記録されている。

### 3.2. 信頼できない入力の境界 (Untrusted Input Boundaries)

`.claude/commands/_lib/review-records.sh` は、レビュー対象のブランチ上のコミットメッセージを入力として読む bash スクリプトである（Go の番兵エラーは該当せず、下の表の終了コード・標準出力・標準エラーの契約で拒否と失敗を区別する）。本タスクで変更する見た範囲の行の境界は次のとおりとする。

| 境界 | 受理する形 | 上限 | 消費する要素・拡張性 | 失敗・拒否時の扱い | 主な AC |
|---|---|---|---|---|---|
| `Local CLI reviewed:` 行（`review-records.sh` で見た範囲を読む全モード: `coverage`・`fable` および実装時点で存在する `unreviewed`・`cross-check` 等） | 1 列目から始まる 1 行全体が `Local CLI reviewed: <START12>..<END12> (coderabbit)` に完全一致すること（`<START12>`・`<END12>` は 12 桁の小文字 16 進数で、既存の `START`/`END` 解決・祖先関係・ブランチ所属・ref 名非重複の検査を満たすこと。`.claude/commands/_lib/review-records.sh:48-55,424-433`） | 新たな上限は設けない（`origin/main..HEAD` のコミット列を対象とする既存の挙動を維持する） | 消費する要素は `START12`・`END12` と固定トークン `(coderabbit)` のみ。拡張可能な要素は宣言せず、他のレビュアー名（`(codex)` を含む）や末尾の追加文字列は受理しない | **行単位の拒否:** 形式に合わない行は補正せず無視し（前後の空白除去・大文字小文字の正規化・`(codex)` からの読み替えを行わない）、同じコミット・ブランチ上の有効な行を無効化せずに終了コード 0 で進む。無視された行の範囲にしか含まれないコミットは `PARTIAL <n>`（未見一覧）または `MISSING <START>` に現れる。<br>**呼び出し・git の失敗:** 引数不正・`origin/main` 不在・`git` 失敗時は既存どおり終了コード 1 で `PARTIAL` または `MISSING` のみと標準エラー診断を出力する（`.claude/commands/_lib/review-records.sh:69-72,119-123,182-189`） | AC-07・AC-08・AC-09 |
| `Local CLI review: not run` 行（`review-records.sh` の `fable` / `cross-check` モード） | 1 列目から始まる 1 行全体が `Local CLI review: not run <START12>..<END12> (...)` に一致し、`START12`・`END12` が上記と同じ検証を満たすこと（`.claude/commands/_lib/review-records.sh:102-106,194`） | 同上 | 消費する要素は `START12`・`END12`。括弧内 `(...)` は拡張可能な自由文字列として既存どおり受け入れる（要件定義の際に決めたこと） | 形式に合わない行は無視する（既存どおり） | AC-09 |

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。正規表現の分岐を 1 つ減らし文書記述を削る変更であり、新たな性能要件はない。

### 4.2. セキュリティ (Security)

-   条件付きのガイド（`docs/dev/security.md`）の適用条件（外部コマンド `yt-dlp` の起動、API キーの扱い、キャッシュの読み書き、LLM API への送信、信頼しないテキストのプロンプトまたは公開記事への埋め込み）に該当する機能はない。N/A。
-   **開発ツールのレビューの関門の厳格化:** 本タスクにより、`review-records.sh` が `(codex)` の見た範囲の行を受理しなくなるため、タスク 0118 の設計書（`docs/tasks/0118_codex_removal/02_architecture.md` §3.6・§5.1）で一時的に受け入れていた「新しく書かれた `(codex)` の行が見た範囲として受理される」状態が解消される。既存の保護機構（`origin/main` 上の `review-records.sh` の実行: `.claude/commands/runpr.md:608-618`・`.claude/commands/_lib/task-pr-handoff.md:205-209`・`.claude/commands/_lib/fable-review.md:82-92`、クリーン環境での再実行および `git` の plumbing コマンドによる読み取り: `.claude/commands/_lib/review-records.sh:56-68,150-178`）は維持する。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   F-001 の確認により、古い記録行を持つ開いたブランチが残っている間に判定規則が切り替わって既存 PR の `/runpr` 判定が突然 `PARTIAL` や `MISSING` になる事態を防ぐ（AC-01・AC-02）。
-   `review-records.sh` の異常系（引数不正・`git` 失敗・`origin/main` 不在）で安全側（`PARTIAL` / `MISSING`、終了コード 1）に倒す既存の振る舞い（`.claude/commands/_lib/review-records.sh:69-72,119-123,182-189`）を保つ。

### 4.4. 互換性 (Compatibility)

-   **既存ブランチとの互換性:** 本タスクの実装は、古い記録行を持つ `origin` のブランチがすべてなくなった後に開始・マージする（F-001）。マージ後に過去のブランチから分岐し、古い記録行を持つブランチが現れた場合、その `(codex)` 行は無視され、該当コミットは未見として安全側（`PARTIAL` / `MISSING`）に判定される。
-   **シェル互換性:** `.claude/commands/_lib/review-records.sh` は既存契約（`.claude/commands/_lib/review-records.sh:74`）のとおり bash 3.2 以降で動作すること。

### 4.5. 保守性 (Maintainability)

-   記録行の形式の正本は `.claude/commands/_lib/local-cli-review.md` とし、`.claude/commands/_lib/review-records.sh` および `cmd/yt2column/review_records_test.go` が正本と一致する状態を保つ（`.claude/commands/_lib/local-cli-review.md:310-312`、`.claude/commands/_lib/review-records.sh:125-134`）。
-   コマンド定義の文書・スクリプトのコメント・Go テストのコードとコメントは英語で書く（`CLAUDE.md`「Language」）。

## 5. 制約条件 (Constraints)

-   [project_overview.md](../../dev/project_overview.md) の「決定済みの方針」「前提・制約」および `CLAUDE.md` の規則に従う。
-   **先行タスクの要件の退役（方針の変更）:** 本タスクは、タスク 0118 の要件定義書（`docs/tasks/0118_codex_removal/01_requirements.md:194`）の AC-28（「このタスクの変更がマージされる前の commit にある `Codex local review:` の行と、`(codex)` を持つ `Local CLI reviewed:` の行は、それを持つブランチで今と同じ意味に読まれる」）を退役させる。この退役は、同タスクの設計書（`docs/tasks/0118_codex_removal/02_architecture.md:616`）で古い記録行を持つ開いたブランチがなくなったときに行うとあらかじめ定めた移行完了手順であり、F-001 でその条件の成立を確認してから実施する。なお、`docs/tasks/0118_codex_removal/` 配下の過去文書自体は変更しない（§2.3）。
-   **テスト削除の規則:** `cmd/yt2column/review_records_test.go` から Go のテストを削除する場合は、`CLAUDE.md`「Testing Strategy」および [ADR-0037](../../dev/adr/0037-test-deletion-coverage.md) に従い、削除のみのコミットを分けて `make coverage-compare BASE=<親>` を実行し、結果をコミットメッセージと PR の説明に残す。
-   外部依存は追加しない。

## 6. 用語集 (Glossary)

-   **古い記録行:** タスク 0118 で撤去した Codex CLI に由来する、移行期の後方互換として読み続けていた 3 形式のコミットメッセージ行（`Codex local review:` で始まる指摘の行、`Local CLI reviewed: <START12>..<END12> (codex)` の見た範囲の行、`Local CLI review: not run <START12>..<END12> (coderabbit: <kind>; codex: <kind>)` の未レビュー範囲の行）。
-   **開いたブランチ:** 本書では、`origin` に現存する `main` 以外のすべてのリモートブランチを指す。開いている PR の head ブランチに加え、PR を開いていないリモートブランチも含む。
-   **見た範囲の行:** ローカルの CLI レビューがレビューしたコミット範囲を記録する `Local CLI reviewed: <START12>..<END12> (...)` の行（`.claude/commands/_lib/local-cli-review.md` step 7）。
-   **未見:** `review-records.sh` が、ローカルの CLI レビューが見た範囲に入っていないと判定したコミットの状態（`coverage` 等のモードの `PARTIAL`、または `fable` / `cross-check` モードの `MISSING` に現れる）。
-   **レビューの関門:** `/runpr` が PR レビューを推奨するかどうかや、PR 作成前に追加のローカル CLI レビューを走らせるかを決める仕組み一式（[ADR-0013](../../dev/adr/0013-review-setup.md)、`.claude/commands/_lib/pr-review-escalation.md` 基準 4）。
-   その他の用語は [docs/translation_glossary.md](../../translation_glossary.md) と統一する。
