# 要件定義書：自然言語の意味や不要な文言を固定する文書のテストを削除する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-23 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、文書を読むテストのうち、自然言語の文の意味を確かめようとするものと、固定しても防げる誤りのない文言を固定するものを削除するための要件を定義する（[#455](https://github.com/isseis/yt2column/issues/455)）。あわせて、文書のガードを述べる記述を削除の結果に合わせ、同じ種類のテストを今後足さないための方針を定める。

**背景:**

このプロジェクトには、文書（`README.md`、`CLAUDE.md`、`docs/`、`.claude/commands/`、`prompts/README.md` など）を読んで、その内容を確かめるテストが多くある。その一部は、自然言語の文に特定の文字列が含まれる、または含まれないことを確かめている。

この形の確かめには、二つの問題がある（issue #455）。

-   **意味を保証できない。** 必須の語が含まれることしか確かめないので、否定形にして意味を反転させた文（「比べない」→「比べる」）でも通る。
-   **言い換えで壊れる。** 意味が同じでも、文言を直すとテストが落ちる。文書を直すたびに、テストのリテラルも直すことになる。

タスク 0035 の PR #438 のレビューで、同じ理由から、AC-17 の確かめをガードのテストから PR のレビューでの読み合わせ（`manual`）に改めた（`docs/tasks/0035_fixprguard_branch_config/03_implementation_plan.md` の `Comments`、2026-10-22 (decision)）。同じ種類のガード `TestGuideDescribesConfigScope`（`cmd/fixprguard/docs_test.go:526`、0035 AC-18）は残っており、[#440](https://github.com/isseis/yt2column/issues/440) がその削除を求めている。本タスクは、この判断を文書のテスト全体に広げる。

現在の手順は、自然言語の記述にもガードのテストを求めている。`.claude/commands/mkplan.md:74` は、AC ごとに `test` か `static` の確かめを 1 つ以上求め、`manual` は「補うだけで代わりにならない」としている。`docs/dev/developer_guide/requirements_process.md:239` も同じ規則を述べる。そのため、文書の記述を定める AC にも、語の有無を確かめる `static` のガードが書かれてきた。

要件定義の際の調査で見つかった、文書を読むテストのあるファイルは次のとおりである。issue #455 の一覧にないものには「（追加）」と記す。

-   Go: `cmd/fixprguard/docs_test.go`、`cmd/yt2column/docs_test.go`、`cmd/yt2column/copied_transcript_docs_test.go`、`cmd/yt2column/extension_docs_test.go`、`cmd/yt2column/fixpr_docs_test.go`、`cmd/yt2column/output_layout_docs_test.go`、`cmd/yt2column/codex_review_docs_test.go`（追加）、`cmd/yt2column/run_test.go` の `TestRunEvaluationProcedureFlags`（`:2614`。追加）、`cmd/yt2column-server/docs_test.go`、`cmd/yt2column-nmhost/main_test.go` の `TestReadmeHostExitCodes`（`:408`）、`internal/pipeline/pipeline_test.go` の `TestPackageReferenceListsPackages`（`:671`）・`TestPromptsREADMEMatchesContract`（`:755`）、`internal/pipeline/relay_guard_test.go` の `TestSharedTokenRemoved`（`:397`）
-   拡張機能（node）: `extension/test/metadataDocs.test.ts`、`extension/test/serverReasons.test.ts`。`extension/test/ciChanges.test.ts` は文書を読まないが、拡張のジョブを起動する条件として `README.md`・`docs/dev/security.md` を挙げている（`:27-28`）。この条件は、node のテストがこの 2 つの文書を読むことに対応する（`extension/scripts/has-extension-changes.sh:8-10`・`:17`）。

これらのテストを AC の確かめとして挙げている行は、`docs/tasks/*/02_architecture.md`・`03_implementation_plan.md` の AC の表に 120 行ある（24 の承認済みのタスクと、取り下げたタスク 0029）。

文書のガードに触れている記述には、次のものがある。

-   `CLAUDE.md`「Development Notes」の、node のテストが読む文書と text-sync guards の一覧（`CLAUDE.md:302-315`）
-   `doc-guard` CI ジョブの対象パッケージ（`.github/workflows/ci.yml:183-184`、`CLAUDE.md:311-315`）
-   `fixprguard_guide.md`・`fixprguard_guide.ja.md` の 10 章で、ガードのテストを名指す箇所（`fixprguard_guide.ja.md:350-357`）

`docs/dev/developer_guide/test_organization.md` には、文書のテストについての記述はない。

対応 issue: [#455](https://github.com/isseis/yt2column/issues/455)（関連: [#440](https://github.com/isseis/yt2column/issues/440)。タスク 0035・[#431](https://github.com/isseis/yt2column/issues/431)）

**本書の記述範囲:** 本書は、どの確かめを残し、どれを削除するかの基準と、削除に伴って変わる文書・手順・CI の振る舞いを定める。基準をテストごとに当てはめた結果（残す・削除する・一部を削除する、とその理由の一覧）、補助の型・関数の整理の手段、PR の分け方は、設計（`02_architecture.md`）と実装計画で決める。要件の段階で挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #455 で決めたこと:**

-   **削除するもの:** 自然言語の文の意味を確かめようとするテスト（否定や言い換えを見分けられない）と、文言を固定しているが、固定しても防げる誤りのないテスト。
-   **残すもの:** 文書の中の値をコードや設定の値と照合するテスト（フラグの一覧、終了コード、プロトコル番号、拡張機能の ID、権限、`make` のターゲット、拒否理由の表など）、見出しの構造を確かめるテスト、文書から実行可能な例を取り出して実際に解析・実行するテスト。自然言語の意味ではなく、機械的に照合できる値を確かめるからである。
-   **補助の削除:** 削除したテストだけが使う補助の型・関数も取り除く。ほかのテストが使うものは残す。
-   **削除の確かめ:** 削除のたびに `go tool cover -func` の関数ごとの結果が変わらないことを確かめ、記録する（`CLAUDE.md`「Deleting a test is a claim that must be checked」）。
-   **タスクの文書:** 削除したテストを AC の確かめとして挙げているタスクの文書では、その AC の確かめを `manual`（PR のレビューでの読み合わせ）に改め、記録する。
-   **一覧:** テストごとに「残す／削除する／一部を削除する」と理由を一覧にし、PR の説明に載せる。
-   **方針:** 同じ種類のテストを今後足さないよう、`CLAUDE.md` の「Testing Strategy」に方針を一項加える。

**要件定義の際に決めたこと（2026-10-23）:**

-   **#440 を取り込む:** `TestGuideDescribesConfigScope` の削除と、タスク 0035 の文書の AC-18 の確かめの改訂を、本タスクで行う。本タスクの基準にそのまま当たり、分けると同じ判断を 2 回レビューすることになるためである。#440 は本タスクの完了時に閉じる（F-008）。
-   **直書きの期待値は削除する:** 文書に識別子・パス・値・エラーの文言・コマンドの断片が含まれることを確かめるもののうち、期待値をテストの中に直書きしていて、コードや設定から導いていないものは削除する。直書きのリテラルの存在の確かめは、文言の固定と変わらないためである。期待値をコードや設定から導いているものだけを、値の照合として残す（F-001）。
-   **終わったタスクの記録の確かめは削除する:** 手動の確認の表が埋まっている、記入待ちの印が残っていない、評価の記録の行が揃っている、`Comments` に記録がある、といったタスクの記録の確かめは削除する。記録はタスクの完了後に変わらず、確かめに意味があるのは実施の間だけだからである（F-001）。
-   **文書の側の不在の確かめは削除する:** 削除した識別子や古い語が文書に含まれないことを確かめるものは削除する。「〜は使わない」という禁止の文と、本当に使っている箇所を区別できないためである。Go のコードや拡張のソースに識別子が含まれないことを確かめる、コードのガードは残す（F-001）。
-   **信頼する自動レビュアーの一覧の確かめは例外として残す:** `.claude/commands/_context.md` の自動レビュアーの一覧と待つ PR レビュアーの名前を、テストに直書きした期待値（`cmd/yt2column/fixpr_docs_test.go` の `wantReviewers` など）と照合する確かめは、直書きの期待値でも残す。この行は、`/fixpr`・`/runpr` がどの著者のスレッドを信頼するかを決める設定に当たる値である。一覧を変えるには、`_context.md` とテストの両方を同じコミットで変える必要がある（`fixprguard_guide.ja.md:350-355`）。この確かめは文書の正確さではなく、信頼の境界を守る二重の鍵として働いている（F-001）。
-   **手順の規則を改める:** `mkplan.md:74` と `requirements_process.md:239` を改め、文書の自然言語の記述を定める AC は、`manual` だけで確かめてよいとする。今の規則のままでは、これからの計画でも同じ種類のガードが書かれ、過去のタスクの文書を `manual` に改めることとも矛盾するためである（F-007）。
-   **承認済みの文書は `approved` のまま記録する:** タスクの文書で AC の確かめを `manual` に改めるときは、Status を `approved` のまま変えず、`Comments` に `(decision)` として本タスクでの変更を記録する。確かめの方法を変える決定は本タスクの要件で定め、その承認が再承認に当たるためである。0035 の 2026-10-22 の記録が先例である。取り下げたタスク 0029 の文書は書き換えない（F-005）。
-   **拡張のジョブの起動条件を合わせる:** 削除の結果、node のテストが読まなくなった文書は、拡張の CI ジョブを起動する条件から外す。`ciChanges.test.ts` の行と `CLAUDE.md` の説明も合わせる。読まない文書の変更で拡張のジョブを走らせる理由がないためである（F-006）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   文書を読むテストに、自然言語の意味を確かめようとするものと、固定しても防げる誤りのない文言を固定するものを残さない。
-   コードや設定から導いた値との照合、見出しの構造、実行可能な例の解析・実行、という機械的に確かめられる文書のテストは保つ。
-   文書を直すときに、意味の変わらない言い換えでテストを直す必要をなくす。
-   削除したテストを挙げているタスクの文書、文書のガードを述べる記述、拡張の CI ジョブの起動条件を、削除の結果と一致させる。
-   同じ種類のテストを今後足さないよう、方針と手順の規則を改める。

### 2.2. スコープ (In Scope)

-   1 章の背景に挙げたファイルの、文書を読むすべてのテストへの基準の当てはめと、削除・一部の削除（F-001・F-002）。
-   削除したテストだけが使っていた補助の型・関数と、その補助の単体テストの削除（F-003）。
-   削除の確かめの記録と、テストごとの一覧（F-004）。
-   削除したテストを AC の確かめとして挙げている、承認済みのタスクの `02_architecture.md`・`03_implementation_plan.md` の改訂（F-005）。
-   文書のガードを述べる記述と、拡張の CI ジョブの起動条件の更新（F-006）。
-   `CLAUDE.md`「Testing Strategy」の方針の追加と、`mkplan.md`・`requirements_process.md` の規則の改訂（F-007）。
-   #440 を閉じること（F-008）。

### 2.3. スコープ外 (Out of Scope)

-   **直書きの期待値をコードから導く形に書き換えること。** 新しいテストを書く作業になり、issue #455 の「洗い出して削除する」を超える。削除した確かめを書き直したい場合は、別の issue で扱う。
-   **プログラムの出力の文言を固定するテスト。** 文書を読まないテストは対象外とする。たとえば `TestArtifactDocsContract`（`cmd/yt2column/docs_test.go:527`）のうち、`run -h` の出力を確かめる部分は文書を読まないので、本タスクでは変えない。
-   **文書を読まないコードのガード。** 文書を読むテストの中にあっても、Go のコードや拡張のソース、設定ファイル（`.coderabbit.yaml` の値など）、`manifest.json` を確かめる部分は対象外とし、残す。
-   **`doc-guard` CI ジョブの対象パッケージを増やすこと。** `cmd/yt2column-nmhost` の `TestReadmeHostExitCodes`（値の照合で、残す）は `doc-guard` ジョブの対象外である（`.github/workflows/ci.yml:184`）。この欠けは本タスクの削除とは関わりがないので、本タスクでは扱わない。
-   **取り下げたタスク 0029 の文書の書き換え。** 取り下げた文書は記録として残す。
-   **用語集（`docs/translation_glossary.md`）の行の追加・削除。** 用語集を確かめるテストを削除しても、用語集そのものは変えない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 残す確かめと削除する確かめの基準

文書を読むテストの各確かめを、次の基準で「残す」か「削除する」かに分ける。1 つのテストが両方の確かめを含む場合は、削除する確かめだけを取り除く（一部の削除）。

**残す確かめ:**

-   (K1) **値の照合:** 文書の中の値を、テストがコードまたは設定から導いた値と照合する確かめ。期待値をテストの中に直書きしていないもの。例: フラグの一覧、終了コード、`--protocol N`、拡張機能の ID、`manifest.json` の権限、`make` のターゲット、拒否理由の表、パッケージの一覧。照合の向きは問わない（文書にあるべき値がある、文書の値がコードにある、の両方を含む）。
-   (K2) **見出しの構造:** 必要な見出しがあること、見出しのレベル、表の列の見出しを確かめる確かめ。
-   (K3) **実行可能な例:** 文書からコマンドの呼び出し例・出力の例・コードの断片を取り出し、実際のパーサやプログラムで解析・実行・生成して確かめる確かめ。
-   (K4) **信頼する自動レビュアーの一覧:** 1 章の「要件定義の際に決めたこと」の例外。`_context.md` の自動レビュアーの一覧と待つ PR レビュアーの名前を、テストに直書きした期待値と照合する確かめ。

**削除する確かめ:**

-   (D1) **自然言語の意味:** 文書の節・行・表のセルが、特定の語句を含むことで、記述の意味を確かめようとする確かめ。
-   (D2) **直書きのリテラルの存在:** 文書が、テストに直書きした識別子・パス・値・エラーの文言・コマンドや jq の断片・テスト名・リンクを含むことを確かめる確かめ（K4 を除く）。期待値がコードや設定から導かれていないもの。手順の中の語の順序を、直書きの語で確かめるものを含む。
-   (D3) **文書の側の不在:** 文書に、削除した識別子や古い語、禁止したコマンドが含まれないことを確かめる確かめ。
-   (D4) **タスクの記録:** タスクの文書の記録（手動の確認の表、記入待ちの印、評価の記録の行、`Status`・`Comments` の記録、ほかの文書へのリンク）が揃っていることを確かめる確かめ。

一つの確かめが、コードから導いた値と自然言語の語句の両方を確かめる場合（たとえば、コードの定数の値と「読まない」の語が同じ行にあること）は、値の照合（K1）の部分を残し、語句の部分（D1）を削除する。

**Acceptance Criteria**:

-   **AC-01**: 本タスクの完了後、1 章の背景に挙げたファイルの、文書を読むテストの各確かめは、K1〜K4 のいずれかに当たる。D1〜D4 に当たる確かめは残っていない。
-   **AC-02**: K1〜K4 に当たる確かめは、本タスクの前と同じ文書の同じ値・構造・例を確かめている。本タスクは、残す確かめの対象を狭めない。
-   **AC-03**: `TestGuideDescribesConfigScope`（`cmd/fixprguard/docs_test.go:526`）は、本タスクの完了後に存在しない（D1・D3。#440）。
-   **AC-04**: 残る確かめのうち、文書を読まない部分（Go のコード・拡張のソース・設定ファイル・`manifest.json`・プログラムの出力の確かめ）は、本タスクの前と同じものを確かめている。

#### F-002: 対象のファイル

1 章の背景に挙げたすべてのファイルを、F-001 の基準の対象とする。issue #455 の一覧にないもの（`cmd/yt2column/codex_review_docs_test.go`、`cmd/yt2column/run_test.go` の `TestRunEvaluationProcedureFlags`、`internal/pipeline/pipeline_test.go` の該当する 2 つのテスト、`internal/pipeline/relay_guard_test.go` の `TestSharedTokenRemoved`）も含む。

**Acceptance Criteria**:

-   **AC-05**: F-004 の一覧は、1 章の背景に挙げたファイルの、文書を読むすべてのテストを挙げている。
-   **AC-06**: F-004 の一覧を作る時点で、1 章の背景に挙げていないファイルに文書を読むテストが見つかった場合は、そのテストも一覧に加え、F-001 の基準を当てはめている。

#### F-003: 補助の型・関数の削除

削除した確かめだけが使っていた補助の型・関数・定数・正規表現と、それらの単体テストを取り除く。残す確かめが使うものは残す。

**Acceptance Criteria**:

-   **AC-07**: 本タスクの完了後、テストのコード（`*_test.go`、`test_helpers*.go`、`extension/test/`）に、どのテストからも使われない補助の型・関数・定数・正規表現は残っていない。`make lint`（`unused` を含む）と `make ext-lint` が通る。
-   **AC-08**: 補助の単体テスト（たとえば `TestClaudeBullet`、`TestWithoutSection`）は、その補助が残る場合に限って残っている。
-   **AC-09**: `make test`、`make lint`、`make ext-check` が通る。

#### F-004: 削除の確かめの記録とテストごとの一覧

**Acceptance Criteria**:

-   **AC-10**: 本タスクの PR の説明に、F-002 の対象のテストごとに、「残す」「削除する」「一部を削除する」のいずれかと、その理由（F-001 の K1〜K4・D1〜D4 のどれに当たるか）を記した一覧がある。「一部を削除する」では、残す確かめと削除する確かめをそれぞれ挙げている。複数の PR に分ける場合は、各 PR の説明に、その PR が扱うテストの一覧がある。
-   **AC-11**: テストを削除するコミットごとに、`go tool cover -func` の関数ごとの結果が、削除の前後で変わらないことを確かめ、その旨をコミットメッセージに記している（`CLAUDE.md`「Deleting a test is a claim that must be checked」）。
-   **AC-12**: 削除によって `go tool cover -func` のいずれかの関数の結果が下がる場合は、その削除を行っていない。下がる原因の確かめが K1〜K3 の確かめなら、F-001 の基準どおり残す。D1〜D4 の確かめなら、その関数を別のテストが通るようにしてから削除するか、削除しない理由を AC-10 の一覧に記している。

#### F-005: タスクの文書の AC の確かめの改訂

削除した確かめ（一部の削除を含む）を AC の確かめとして挙げている、承認済みのタスクの `02_architecture.md`・`03_implementation_plan.md` を改める。

**Acceptance Criteria**:

-   **AC-13**: 本タスクの完了後、承認済みのタスクの `02_architecture.md`・`03_implementation_plan.md` に、AC の確かめとして、存在しないテスト、または削除した確かめを挙げている行はない。削除した確かめだけで確かめていた AC は `manual`（PR のレビューでの読み合わせ）に改めている。残す確かめもあわせて挙げていた AC は、残す確かめだけを挙げている。
-   **AC-14**: AC-13 で改めたそれぞれの文書は、Status が `approved` のままで、`Comments` に、日付、`(decision)`、本タスク（0038）で AC の確かめを改めたこと、改めた AC の ID が記録されている。
-   **AC-15**: タスク 0035 の `02_architecture.md`（7.5）と `03_implementation_plan.md`（1.4、5 章）の AC-18 は、`manual` で確かめると記されている。
-   **AC-16**: 取り下げたタスク 0029 の文書は、本タスクの前と変わっていない。

#### F-006: 文書のガードを述べる記述と CI の起動条件の更新

**Acceptance Criteria**:

-   **AC-17**: `CLAUDE.md`「Development Notes」の、node のテストが読む文書と text-sync guards の一覧は、本タスクの完了後に残っている文書のテストと一致している。削除したテストだけが確かめていた文書を、ガードが読む文書として挙げていない。
-   **AC-18**: `doc-guard` CI ジョブの対象パッケージ（`.github/workflows/ci.yml` と `CLAUDE.md` の記述）は、文書を読むテストが残っているパッケージを、本タスクの前と同じだけ含む。削除によって文書を読むテストがなくなったパッケージがあれば、その記述を改めている。
-   **AC-19**: 本タスクの完了後、node のテストが読まない文書の変更だけでは、拡張の CI ジョブは起動しない。node のテストが読む文書の変更では、今までどおり起動する。`extension/scripts/has-extension-changes.sh` の冒頭の説明、`extension/test/ciChanges.test.ts` の行、`CLAUDE.md` の説明は、この起動条件と一致している。
-   **AC-20**: 本タスクの完了後、`docs/tasks/` 以外の文書（`CLAUDE.md`、`README.md`、`docs/dev/`、`.claude/commands/`）に、削除したテスト・補助の名前や、削除した確かめが行う確かめを、現にあるものとして述べる記述はない（たとえば `fixprguard_guide.md`・`fixprguard_guide.ja.md` 10 章の、ガードのテストを名指す箇所）。
-   **AC-21**: `docs/dev/developer_guide/test_organization.md` に文書のテストを述べる記述がある場合は、AC-17 と同じく本タスクの結果と一致している。

#### F-007: 方針と手順の規則

**Acceptance Criteria**:

-   **AC-22**: `CLAUDE.md` の「Testing Strategy」に、文書のテストについての一項がある。その項は、文書のテストで確かめてよいもの（F-001 の K1〜K3）と、確かめないもの（D1〜D4）を述べ、自然言語の記述は PR のレビューでの読み合わせで確かめる、と述べている。K4 の例外とその理由も述べている。
-   **AC-23**: `.claude/commands/mkplan.md` の AC の確かめの規則は、文書の自然言語の記述を定める AC について、`manual` だけでの確かめを認めている。そのような AC に、語の有無を確かめるガードのテストや `rg` の確かめを、`static` として求めていない。
-   **AC-24**: `docs/dev/developer_guide/requirements_process.md` の AC の確かめの規則（4 章）は、AC-23 と同じ規則を述べている。
-   **AC-25**: AC-23・AC-24 の改訂の後も、文書の自然言語の記述以外を定める AC（振る舞い、値、構造）には、今までどおり `test` か `static` の確かめを 1 つ以上求めている。

#### F-008: #440 の扱い

**Acceptance Criteria**:

-   **AC-26**: 本タスクの PR のうち `TestGuideDescribesConfigScope` を削除するものの説明は、#440 を `Refs #440` で参照している。
-   **AC-27**: 本タスクの最後の PR のマージの後、#440 は閉じている。閉じるときのコメントは、本タスクで対応したことを述べている。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。テストの削除で `make test` の時間は短くなるが、時間の目標は設けない。

### 4.2. セキュリティ (Security)

-   N/A（`docs/dev/security.md` の条件付きガイドの対象外）。本タスクは外部コマンドの起動、API キーや Webhook URL、キャッシュ、ネットワーク、信頼できないテキストを扱う製品のコードを変えない。
-   セキュリティに関わる文書のガードのうち、コードの定数との照合（たとえば `fixprguard.CodexMention` が `_context.md` の PR レビュー依頼の行にだけ現れることの確かめ、`TestCodexMentionOnlyInRequestRow`）は K1 として残る。信頼する自動レビュアーの一覧の確かめは K4 として残る。
-   削除する確かめが述べていた安全の性質（たとえばコマンド文書の、投稿の前に `@codex` の言及の確かめを行う、という手順の文）は、PR のレビューでの読み合わせで確かめる。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   N/A。

### 4.4. 互換性 (Compatibility)

-   製品のコード（`cmd/`・`internal/` の `_test.go` 以外、`extension/src/`）は変えない。拡張のジョブの起動条件（F-006）だけが CI の振る舞いとして変わる。

### 4.5. 保守性 (Maintainability)

-   文書の意味を変えない言い換えでは、テストが落ちない（F-001 の D1・D2 を削除した結果）。
-   文書の値・構造・実行可能な例が、コードや設定と食い違えば、今までどおりテストが落ちる（F-001 の K1〜K3）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   `CLAUDE.md`「Deleting a test is a claim that must be checked」に従い、削除ごとに `go tool cover -func` の結果を確かめる（F-004）。
-   `CLAUDE.md` の方針に対する例外は、F-007 で手順の規則（`mkplan.md:74`、`requirements_process.md:239`）を改めることだけである。改めた後の規則は、本タスクの変更そのものを許す。
-   拡張の node のテストを変えるので、`make ext-check` を通す（`CLAUDE.md`「Development Notes」）。

## 6. 用語集 (Glossary)

-   **文書のテスト:** 文書（`README.md`、`CLAUDE.md`、`docs/`、`.claude/commands/`、`prompts/README.md`、`AGENTS.md` など）を読み、その内容を確かめるテスト。`CLAUDE.md` の text-sync guards と、`doc-guard` CI ジョブが走らせるガードを含む。
-   **確かめ:** 1 つのテストの中の、一つの条件の検査。1 つのテストは複数の確かめを含みうる。
-   **値の照合:** 文書の中の値を、テストがコードまたは設定から導いた値と比べること（F-001 の K1）。
-   **一部の削除:** 1 つのテストから、削除する確かめだけを取り除き、残す確かめを保つこと。
-   **読み合わせ（`manual`）:** PR のレビューで、レビュアーが文書の記述を読んで確かめること。
-   その他の用語は `docs/translation_glossary.md` と統一する（受け入れ基準 = acceptance criteria、ガードツール = guard tool）。
