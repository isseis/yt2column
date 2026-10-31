# 要件定義書：文書のテストから、古い仕様・一度きりの確かめ・過剰な確認を取り除く

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-31 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、文書のテストのうち、古い仕様を確かめ続けるもの、過去の 1 回の変更の性質を確かめるもの、過剰なものを取り除くための要件を定義する（[#591](https://github.com/isseis/yt2column/issues/591)）。あわせて、タスク 0038 の基準にすでに反しているのに残っている確認を取り除き、同じ種類のテストを今後足さないための方針を定める。

**背景:**

タスク 0038（[#455](https://github.com/isseis/yt2column/issues/455)）は、文書のテストの各確認を、残す確認 K1〜K5 と削除する確認 D1〜D4 に分ける基準を定めた（`docs/tasks/0038_doc_test_pruning/01_requirements.md` F-001）。そして、自然言語の文の意味を確かめるテストや、直書きのリテラルを固定するテストを削除した。この基準は `CLAUDE.md`「Testing Strategy」の「Document tests check values, structure, and executable examples, never prose」の項（`CLAUDE.md:490-509`）に記されている。

0038 の調査（`80c17a8` の時点。0038 の `02_architecture.md`）の後も、タスク 0040・0045・0048・0049 などで文書のテストが加わった。たとえば `TestMigrationDocsContract`（`cmd/yt2column/docs_test.go:619`）、`TestQuickStartLinks`（`:688`）、`TestProductionProfileLinks`（`:733`）、`TestReadmeVersionsShareStructure`・`TestReadmeVersionLinks`（`cmd/yt2column/readme_structure_test.go:21`・`:42`）、`TestReadmeSyncStepPlacement`（`cmd/yt2column/fixpr_docs_test.go:1027`）、`cmd/codexreview/docs_test.go` の全体である。2026-10-31 の `66222946` の時点で、`cmd/yt2column/docs_test.go` は 1030 行、`cmd/yt2column/fixpr_docs_test.go` は 1075 行ある。

issue #591 は、0038 の基準の D1〜D4 とは別の種類の、次の 3 つのテストが残っているとする。

1.  **古い仕様に対するテスト。** 退役した仕様や移行期間の記述、置き換えた手順を確かめ続けるもの。たとえば `TestMigrationDocsContract` は、両方の README に「退役した LLM の変数からの移行」の節があり、退役した変数（`config.RetiredLLMVarsForTest`）の組が新しい値に対応することを求める。節を消すとテストが落ちるので、仕様が変わった後も、文書にその記述を残させてしまう。
2.  **過去の 1 回の変更の性質を確かめるテスト。** ある変更で何かが移った・消えた・変わらなかった・決まった位置に加わった、という性質を確かめるもの。たとえば `TestReadmeSyncStepPlacement` は、タスク 0048 が加えた README の同期の手順が、`fixpr.md` の Phase 3 と `_lib/task-pr-handoff.md` の Part B の決まった位置にあることを確かめる。`CLAUDE.md:506-509` と `docs/dev/conventions/requirements_process.md` §4 は、このような変更の性質を恒久的なテストにせず、タスクの `_checks/` のスクリプトで merge-base に対して 1 回だけ確かめると定めている。
3.  **過剰なテスト。** 防ぐ誤りが起きにくいもの、起きても害が小さいもの、ほかのテストと同じ値を確かめるもの、呼び出し側の表と重なる補助（文書のテストが使うテスト用の型・関数）の単体テスト、文書を直すたびにテストも直す手間が、得られる効果を上回るもの。たとえば `TestSlackDocsContract`（`cmd/yt2column/docs_test.go:170`）は、同じ値を複数の文書で確かめる。`TestReadmeVersionsShareStructure` は、それだけのための 467 行のパーサ（`internal/doctestutil/structure.go`）に支えられている。

このほかに、0038 の基準にすでに反しているのに残っている確認がある。たとえば `TestCodeRabbitConfigStopsReview`（`cmd/yt2column/codex_review_docs_test.go:354`）は、`.coderabbit.yaml` のコメントの英文を語句で固定している。そのため、タスク 0049 でコメントを言い換えたときに、テストも直す必要があった（`d0a82d68`）。`TestCostDocsContract`（`cmd/yt2column/docs_test.go:960`）は、`internal/pricing/rates.go` の冒頭のコメントが直書きのファイル名 `pricing_update.md` を含むことを確かめる（`:1027`）。

文書のテストを述べる記述には、次のものがある。

-   `CLAUDE.md`「Development Notes」の、node のテストが読む文書と text-sync guards（文書のテストのこと。6 章）の一覧（`CLAUDE.md:519-540`）
-   `doc-guard` CI ジョブの対象パッケージ（`.github/workflows/ci.yml:187`、`CLAUDE.md:537-538`）と、タスクの文書を作るコマンドが push の前に走らせる文書のガード（`.claude/commands/_lib/task-pr-handoff.md:40`）
-   `fixprguard_guide.md` の、自動レビュアーの一覧を変えるときの手順（`docs/dev/guides/fixprguard_guide.md:454`）。この手順と `task-pr-handoff.md:40` が挙げる文書のガードのコマンドは、今 `./cmd/codexreview` を含まない
-   `docs/dev/conventions/package_reference.md` の、`internal/doctestutil` の関数の一覧（`:52`）
-   拡張の CI ジョブを起動する条件（`extension/scripts/has-extension-changes.sh:8-10`・`:19`、`extension/test/ciChanges.test.ts`）

実装中のタスク 0051 の計画は、次の PR で `cmd/yt2column/docs_test.go` の文書のガードを変え、`TestMigrationDocsContract` を AC-35 の検証方法に挙げている（`docs/tasks/0051_llm_effort/03_implementation_plan.md:304-309`・`:434`）。本タスクの実装は 0051 の完了の後に始めるので（5 章）、その時点で 0051 は完了したタスクになり、その計画は変えない（AC-17）。

対応 issue: [#591](https://github.com/isseis/yt2column/issues/591)（関連: [#455](https://github.com/isseis/yt2column/issues/455)・タスク 0038、[#180](https://github.com/isseis/yt2column/issues/180)・[#181](https://github.com/isseis/yt2column/issues/181)）

**本書の記述範囲:** 本書は、どの確認を残し、どれを取り除くかの基準、テストごとの判断をレビュアーが確かめる方法、取り除くことに伴って変わる記述・方針・CI の振る舞いを定める。基準をテストごとに当てはめた結果の一覧、補助の整理の手段（統合するか削除するか）、カバレッジの比べ方、PR の分け方は、設計（`02_architecture.md`）と実装計画で決める。要件の段階で挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #591 で決めたこと:**

-   **取り除く 3 つの種類:** 古い仕様に対するテスト、過去の 1 回の変更の性質を確かめるテスト、過剰なテスト（1 章の背景の 1〜3）。
-   **一覧:** テストごとに「残す／削除する／`_checks/` へ移す／縮める」と理由を一覧にする。
-   **判断の基準を明文化する:** 信頼の境界の例外（K4・K5）について、どこまでが「誤ると実行時にガードが働かなくなる」もので、どこからが過剰かの線を引く。
-   **補助の削除:** 削除したテストだけが使っていた補助も取り除く。
-   **カバレッジの比較:** 削除のたびに `go tool cover -func` の関数ごとの結果が変わらないことを確かめ、記録する（`CLAUDE.md`「Deleting a test is a claim that must be checked」）。
-   **記述の更新:** 文書のガードに言及している記述を、結果に合わせて直す。
-   **残すもの:** 値の照合、見出しの構造（README の 2 つの版の構造の一致を含む）、文書のコマンドの例の解析・実行、`TestReadmeSyncMarker`、自動レビュアーの一覧とエージェントの信頼の境界のコマンドのテスト。ただし、精査の結果、過剰と判断したものは除く。

**要件定義の際に決めたこと（2026-10-31）:**

-   **1 回の変更の性質を確かめるテストは削除する:** `_checks/` へは移さない。元のタスクはどれもマージ済みである。`_checks/` は merge-base と PR の head を比べて 1 回だけ実行するものなので、今から元のタスクの `_checks/` に移しても、実行する機会がない。削除の理由は一覧に残す（F-001 の D6）。issue の「`_checks/` へ移す」という選択肢は、これで採らないことになる。
-   **過剰なテストは、機械的な基準と判断の両方で決める:** 機械的に判定できる重複（同じ値を同じ出どころと照合する確認が 2 つ以上ある、補助の単体テストが呼び出し側の表と同じ行を持つ）に加えて、費用と効果を比べた判断による削除も認める。判断による削除は、テストごとに理由を一覧に記し、レビュアーが設計書のレビューで一件ずつ決める（F-001 の D7、F-003）。機械的な基準だけでは、判断なしに決められないものを取りこぼすためである。同じ理由で、K1〜K3 にも当たる確認を D5・D6 として取り除くかどうかと、K4・K5 に当たるかが分かれる確認の扱いも、この一件ずつの決定に含める（F-003）。
-   **完了したタスクの文書は変えない:** 削除したテストを AC の検証方法に挙げている、完了したタスクの `02_architecture.md`・`03_implementation_plan.md` は書き換えない。完了したタスクの検証の記録は経緯の記録であり、書き換えても新しい情報は増えず、手間だけがかかるためである。0038 は検証方法を `manual` に書き換えたが（0038 F-005）、本タスクはそうしない。書き換えるのは、実装中の計画だけである（F-006）。
-   **文書の本文は変えない:** 古い仕様のテストを削除しても、その仕様の記述（README の移行の節など）は消さない。記述を消すかどうかは、記述ごとに別の issue で判断する。本タスクが変える文書は、文書のテストと方針を述べる記述だけである（F-005）。
-   **K4・K5 の確認は減らさない:** 自動レビュアーの一覧（K4）と、エージェントが実行する手順（K5）を確かめる確認は、過剰に見えても 1 つも取り除かない。認めるのは、確認を失わない重複の統合と、補助の単体テストの整理だけである（F-001）。誤ると `/fixpr`・`/runpr`・`codexreview` の信頼の境界が実行時に働かなくなる、という 0038 の理由が変わっていないためである。
-   **D1〜D4 の残りも取り除く:** 0038 の基準の D1〜D4 にすでに当たるのに残っている確認も、本タスクで取り除く（`TestCodeRabbitConfigStopsReview` の英文の固定など）。同じテストを精査するので、別の issue に分けると同じファイルを 2 回レビューすることになるためである（F-001）。
-   **方針に分類 1・3 の規則を加える:** `CLAUDE.md`「Testing Strategy」の文書のテストの項に、古い仕様を確かめるテストと過剰なテストを恒久的に足さない旨の規則を加える。1 回の変更の性質（分類 2）は、`CLAUDE.md:506-509` の既存の規則をそのまま使う（F-008）。
-   **実装は 0051 の完了の後に始める:** 本タスクの実装（`/runplan`）は、タスク 0051 の最後の PR がマージされてから始める。0051 の次の PR は `cmd/yt2column/docs_test.go` の文書のガードを変えるので、並行して進めると同じ確認を両方のタスクで変えることになる。本書の基準は、実装を始める時点のテストに当てはめる。要件と設計の文書は並行して進めてよい（5 章）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   文書のテストに、古い仕様を確かめ続ける確認、過去の 1 回の変更の性質を確かめる確認、過剰な確認、0038 の基準の D1〜D4 に当たる確認を残さない。
-   残す確認（0038 の K1〜K5）が確かめていることを失わない。特に K4・K5 の確認は 1 つも減らさない。
-   古い仕様の記述を文書から消すとき、また文書を直すときに、テストを直す手間を減らす。
-   文書のテストを述べる記述、文書のガードのパッケージの一覧、拡張の CI ジョブの起動条件を、結果と一致させる。
-   同じ種類のテストを今後足さないよう、方針を改める。

### 2.2. スコープ (In Scope)

-   この節の末尾に挙げる対象のファイルにあるすべての文書のテストと、それが使う補助の単体テストに基準を当てはめ、取り除くこと（F-001・F-002）。
-   テストごとの判断の一覧と、判断による削除のレビュアーの決定（F-003）。
-   取り除いた確認だけが使っていた補助と、その単体テストの削除。カバレッジの比較（F-004）。
-   実装中の計画の扱い（F-006）。
-   文書のテストを述べる記述、文書のガードのパッケージの一覧、拡張の CI ジョブの起動条件の更新（F-007）。
-   `CLAUDE.md`「Testing Strategy」の方針の追加（F-008）。

対象のファイル（2026-10-31、`66222946` の時点）:

-   Go: `cmd/yt2column/docs_test.go`、`fixpr_docs_test.go`、`codex_review_docs_test.go`、`extension_docs_test.go`、`output_layout_docs_test.go`、`copied_transcript_docs_test.go`、`docpaths_test.go`、`readme_structure_test.go`、`readme_sync_test.go`、`run_test.go` の `TestRunEvaluationProcedureFlags`（`:2868`）、`cmd/yt2column-server/docs_test.go`、`cmd/yt2column-nmhost/main_test.go` の `TestReadmeHostExitCodes`（`:435`）、`cmd/fixprguard/docs_test.go`、`cmd/codexreview/docs_test.go`、`internal/pipeline/pipeline_test.go` の文書を読むテスト（`:672`・`:756`）と、インターフェースの doc comment を確かめる `TestInterfaceDocComments`（`:512`。issue #591 が挙げる）、`internal/doctestutil/` の補助とその単体テスト
-   拡張（node）: `extension/test/metadataDocs.test.ts`、`serverReasons.test.ts`、`markdownSection.test.ts`、`extension/test/helpers/` のうち文書を読む補助。`extension/test/ciChanges.test.ts` は文書を読まないが、起動条件を確かめるので F-007 の対象にする

### 2.3. スコープ外 (Out of Scope)

-   **文書とタスクの記録の本文。** 古い仕様の記述（README の移行の節、`Upgrading from the shared-token version` の節など）を消すこと、取り除いたテストを AC の検証方法に挙げている完了したタスクの文書を書き換えること。1 章「文書の本文は変えない」「完了したタスクの文書は変えない」のとおりである。
-   **K4・K5 の確認を減らすこと。** 1 章「K4・K5 の確認は減らさない」のとおりである。
-   **取り除いた確認の代わりのテストを書くこと、直書きの期待値をコードから導く形に書き換えること。** 新しいテストを書く作業になり、issue #591 の範囲を超える。必要なら別の issue で扱う。ただし、確認を失わずに重複を統合するための書き換え（D7 の (a)・(b)）は範囲に含む。
-   **文書を読まない確認。** 文書のテストの中にあっても、製品のコード・設定ファイル・プログラムの出力だけを確かめる部分（`TestNativeHostContract` の拡張のソースの確認、`TestSettingsAllowsTool` の `settings.json` の確認など）は、D1〜D7 に当たらない限り変えない。ただし、設定ファイルやコードのコメントの自然言語を固定する部分（`TestCodeRabbitConfigStopsReview` の英文、`TestCostDocsContract` の `rates.go` のコメント）は、D1・D2 として扱う。
-   **`doc-guard` CI ジョブの対象パッケージを増やすこと。** `cmd/yt2column-nmhost` の `TestReadmeHostExitCodes` が `doc-guard` ジョブの対象外であること（`CLAUDE.md:533-537`）は扱わない。
-   **コードだけを確かめる、退役した名前の不在の確認。** `TestSharedTokenRemoved`（`internal/pipeline/relay_guard_test.go:328`）は、0038 以降は文書を読まない（0038 F-001 の D3 は、コードの不在の確認を残すとした）。本タスクの対象外とする。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 残す確認と取り除く確認の基準

文書のテストの各確認を、次の基準で「残す」か「取り除く」かに分ける。1 つのテストが両方を含む場合は、取り除く確認だけを取り除く（一部の削除）。「確認」「値の出どころ」「一部の削除」は 0038 の定義（0038 F-001・6 章）に従う。

**残す確認:** 0038 F-001 の K1（値の照合）、K2（見出しの構造）、K3（実行可能な例）、K4（自動レビュアーの一覧）、K5（エージェントが実行する手順）。K4・K5 の範囲は、0038 F-001 の定義のとおりとする（K5 は、0038 `01_requirements.md` の `Comments` にある 2026-10-23 の改訂で広げた後の範囲）。ただし K1〜K3 に当たる確認でも、D5〜D7 に当たるものは取り除く。

**取り除く確認:**

-   (D1〜D4) 0038 F-001 の D1（自然言語の意味）、D2（直書きのリテラルの存在）、D3（文書の側の不在）、D4（タスクの記録）。設定ファイルの値の部分ではない注釈と、コードのコメントの自然言語を、語句やリテラルで確かめる確認も、D1・D2 に含む。
-   (D5) **古い仕様:** 退役した仕様（退役した環境変数・設定の値、移行期間の手順、置き換えた行や節）の記述が文書にあること、またはその記述の値を確かめる確認。退役した仕様の記述を探して確かめるためだけの手がかり（見出しのロケータなど）も D5 に含む。残す確認の範囲を狭める手がかりは、D5 に含まない（下の「重なるときの扱い」）。
-   (D6) **1 回の変更の性質:** 過去の 1 つのタスクで加えた、移した、消した、変えなかったという性質を確かめる確認。そのタスクの変更が正しく入ったこと以外を守らないもの。例: そのタスクで加えた手順が決まった位置にあること、そのタスクで加えたリンクがあること、そのタスクで移す前のパスが残っていないこと。
-   (D7) **過剰:** 次のどれかに当たる確認。
    -   (a) 同じ文書の同じ値を、同じ出どころと照合する確認が、ほかのテストにもある（重複）。
    -   (b) 補助の単体テストの行が、その補助を呼ぶテストの表の行と同じリテラルで同じ場合を確かめている（`CLAUDE.md`「Check for duplication before adding a table test for a helper」）。
    -   (c) 上の (a)・(b) に当たらないが、防ぐ誤りが起きにくい、起きても害が小さい、または保守の手間が防ぐ誤りに見合わないと判断したもの。(c) で取り除くのは、F-003 でレビュアーが決めたものだけである。

**重なるときの扱い:**

-   K4・K5 に当たる確認は、D1〜D7 のどれに当たる場合でも取り除かない。K4・K5 の確認に認めるのは、D7 (a)・(b) に当たる重複の統合だけであり、統合の後もその確認は少なくとも 1 か所に残る。
-   D7 (a) で重複を取り除くときは、重複する確認のうち 1 つを残す。
-   残す確認が調べる範囲を狭めるための手がかり（ある節を範囲から除く見出しのロケータなど）は、その残す確認の一部であり、D5 に当たらない。たとえば `cmd/yt2column-server/docs_test.go:53-63` の `ReadmeUpgradeHeading` は、README の古い版からの移行の節を、K1 の確認が調べる範囲から除いている。そのような手がかりを取り除くのは、取り除いても残す確認が受け入れるものが広がらない場合に限る。
-   K1〜K3 にも当たる確認を D5・D6 として取り除くかどうかと、K4・K5 に当たるかが分かれる確認の扱いは、F-003 でレビュアーが決める。
-   `TestReadmeSyncMarker`（`cmd/yt2column/readme_sync_test.go`）は、README の同期の仕組みそのもの（`CLAUDE.md`「Translation Guidelines」）なので、D5〜D7 に当たっても取り除かない。

**Acceptance Criteria**:

-   **AC-01**: 本タスクの完了後、2.2 の対象のファイルと、AC-05 で一覧に加えたテストにある文書の確認は、いずれも K1〜K5 のどれかに当たる。D1〜D7 に当たる確認が残るのは、K4・K5 の確認、`TestReadmeSyncMarker` の確認、F-003 でレビュアーが残すと決めた確認、AC-12 の例外に限る。
-   **AC-02**: 本タスクの前に K4・K5 に当たった確認は、本タスクの完了後もすべて、どこかのテストに残っている。AC-04 の一覧が、K4・K5 の確認ごとに、本タスクの後にそれを確かめるテストの名前を挙げている。
-   **AC-03**: `TestReadmeSyncMarker` は、本タスクの前と同じものを確かめている。
-   **AC-04**: 対象のすべての文書のテストと補助の単体テストについて、「残す」「削除する」「一部を削除する」「統合する」のいずれかと、その理由（K1〜K5・D1〜D7 のどれに当たるか）を記した 1 つの一覧が、`02_architecture.md` にある。変えないテストも「残す」として載っている。「一部を削除する」「統合する」では、残す確認と取り除く確認をそれぞれ挙げ、統合では統合した先のテストを挙げている。本タスクの各 PR の説明は、この一覧のうちその PR が扱う行を挙げている。

#### F-002: 対象のファイル

2.2 の対象のファイルのすべてを、F-001 の基準の対象とする。実装を始める時点（5 章）で、ほかのファイルに文書のテストがあれば、それも対象とする。

**Acceptance Criteria**:

-   **AC-05**: AC-04 の一覧は、実装を始める時点で 2.2 の対象のファイルにあるすべての文書のテストと、それが使う補助の単体テストを挙げている。ほかのファイルで見つかった文書のテストも一覧に加え、F-001 の基準を当てはめている。一覧は、どの時点のコミットで調べたかを記している。設計の後、実装を始めるまでにテストが変わった場合は、変わった行を、実装を始める時点のコミットで調べ直している。

#### F-003: 判断による削除の決定

判断を要する確認は、レビュアーが一件ずつ決める。判断を要する確認とは、D7 (c) に当たると判断した確認、K1〜K3 にも当たるのに D5・D6 として取り除く確認、K4・K5 に当たるかが分かれる確認である（F-001）。

**Acceptance Criteria**:

-   **AC-06**: `02_architecture.md` に、判断を要する確認の一覧がある。各行は、テストの名前と確認、どの種類の判断か、防ぐ誤り、取り除く、または残すと判断した理由を記している。D7 (c) の行は、その誤りが起きにくい・害が小さい・保守の手間が見合わない、のどれに当たるかを記している。
-   **AC-07**: 本タスクで取り除いた、判断を要する確認は、AC-06 の一覧にあり、レビュアーが取り除くと決めたものだけである。設計の承認の後に加わった候補は、レビュアーが決めた後に取り除き、その決定を日付とともに本タスクの文書に記録している。
-   **AC-08**: D5・D6 として取り除いた確認ごとに、AC-04 の一覧が、その確認がどの仕様（D5）またはどのタスクの変更（D6）に当たるかを記している。

#### F-004: 補助の削除とカバレッジの比較

取り除いた確認だけが使っていた補助と、その単体テストを取り除く。取り除いても、テストが製品のコードを通る範囲が狭まっていないことを確かめ、記録する。

**Acceptance Criteria**:

-   **AC-09**: 本タスクの削除によってどのテストからも使われなくなった補助の型・関数・定数・正規表現・ロケータ（`internal/doctestutil` のもの、`//go:build test` のファイルにあるテスト用のアクセサ、たとえば `config.RetiredLLMVarsForTest` を含む）は、残っていない。`make deadcode` の出力は空である。
-   **AC-10**: 補助の単体テストは、その補助が残る場合に限って残っている。
-   **AC-11**: Go のテストや補助を取り除く、または一部を取り除くコミットごとに、`go tool cover -func` の関数ごとの結果を前後で比べ、製品のコード（`//go:build test` のファイルと `_test.go` 以外の Go のファイル）の関数で結果が変わらないことを確かめ、その旨をコミットメッセージと、そのコミットを含む PR の説明に記している（`CLAUDE.md`「Deleting a test is a claim that must be checked」。比べる対象を製品のコードに限る理由は 5 章）。
-   **AC-12**: 取り除く対象の確認を取り除くと AC-11 の結果が変わる場合は、製品のコードを通る部分を残し、文書の確認だけを取り除いている。それでも結果が変わる確認は取り除かず、AC-04 の一覧に、残した理由とともに例外として記している。
-   **AC-13**: node のテストを取り除く、または一部を取り除く場合は、AC-04 の一覧に、取り除いた確認が `extension/src/` のコードを通らないことを記している。
-   **AC-14**: 本タスクの各 PR で、`make test`、`make lint`、`make deadcode`、`make ext-check` が通る。

#### F-005: 文書の本文を変えない

本タスクは、文書の本文を変えない（1 章「文書の本文は変えない」）。

**Acceptance Criteria**:

-   **AC-15**: 本タスクの PR が変える、テストのコード以外のファイルは、本タスクのタスクディレクトリ、F-006 の計画、F-007 の記述（`CLAUDE.md`、`.github/workflows/ci.yml`、`.claude/commands/_lib/task-pr-handoff.md`、`docs/dev/guides/fixprguard_guide.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/conventions/test_organization.md`、`extension/scripts/has-extension-changes.sh`）、F-008 の記述（`CLAUDE.md`）、AC-20 の参照を直す箇所だけである。AC-20 のために `docs/tasks/` 以外のほかの文書（2026-10-31 の時点では `docs/dev/guides/pricing_update.md` などが文書のテストの名前を挙げている）を変える場合は、取り除いたテスト・補助の名前やその参照だけを変え、ほかの記述は変えない。それ以外の文書（`README.md`・`README.en.md`、`docs/` の文書、`.claude/commands/`、`prompts/README.md`、`testdata/README.md`）と、設定ファイル（`.coderabbit.yaml` など）は、AC-20 で名前や参照を直す箇所を除き、本タスクの前と同じである。

#### F-006: 実装中の計画の扱い

本タスクの実装を始める時点で、まだ実装していない作業を含む承認済みの計画と、計画のまだない承認済みの `02_architecture.md` が、本タスクの基準で取り除く確認を加える、または保つと定めている場合に、その文書を基準と矛盾しない形にする。完了したタスクの文書は変えない（1 章「完了したタスクの文書は変えない」）。0051 は、実装の開始の条件（5 章）により、その時点で完了したタスクであり、AC-17 に当たる。

**Acceptance Criteria**:

-   **AC-16**: 本タスクの実装を始める時点で、まだ実装していない作業を含む承認済みの計画（または計画のまだない承認済みの設計書）が、D1〜D7 だけに当たる確認を加える、または保つ作業を含む場合、本タスクの完了後のその文書は、その作業を含まない。対応する AC の検証方法は、取り除いた確認だけで確かめていた AC は `manual`、残す確認もあわせて挙げていた AC は残す確認だけを挙げている。その文書の Status は `approved` のままで、`Comments` に、日付、`(decision)`、本タスク（0052）で改めたこと、改めた作業と AC の ID が記録されている（0038 F-006 と同じ扱い）。
-   **AC-17**: 本タスクの実装を始める時点で、承認済みの計画の作業がすべて実装されているタスク（最後の PR がマージされたタスク）と、取り下げたタスクの文書は、本タスクの前と同じである。

#### F-007: 文書のテストを述べる記述と CI の起動条件

文書のテストを述べる記述を、本タスクの結果に合わせる。

**Acceptance Criteria**:

-   **AC-18**: `CLAUDE.md`「Development Notes」の、node のテストが読む文書と text-sync guards の一覧は、本タスクの完了後に残っている文書のテストと一致している。
-   **AC-19**: `.github/workflows/ci.yml` の `doc-guard` ジョブ、`CLAUDE.md`、`.claude/commands/_lib/task-pr-handoff.md` の CHECKS、`docs/dev/guides/fixprguard_guide.md` が挙げる文書のガードのコマンドは、同じパッケージを対象にしている。対象は、本タスクの完了後に文書のテストが残っている Go のパッケージ（`cmd/yt2column-nmhost` を除く。2.3）と一致している。
-   **AC-20**: 本タスクの完了後、`docs/tasks/` 以外の文書、`.claude/commands/`、残るテストと補助のコメント（`//go:build test` のファイルのものを含む）に、本タスクが取り除いたテスト・補助の名前を、今あるものとして述べる記述はない。
-   **AC-21**: 拡張の CI ジョブを起動する文書は、本タスクの完了後に node のテストが読む文書と一致している。起動条件のうち文書でないもの（`extension/` 以下、`Makefile`、`.github/workflows/` 以下、`go.mod`）は変わっていない。`internal/server/response.go` は、node のテストがそれと照合する文書（`docs/dev/guides/development.md` の拒否理由の表）を読む間だけ、起動条件に残る。`extension/scripts/has-extension-changes.sh` の冒頭の説明、`extension/test/ciChanges.test.ts` の行、`CLAUDE.md` の説明は、この起動条件と一致している。

#### F-008: 方針

同じ種類のテストを今後足さないよう、`CLAUDE.md` に方針を記す（1 章「方針に分類 1・3 の規則を加える」）。

**Acceptance Criteria**:

-   **AC-22**: `CLAUDE.md`「Testing Strategy」の文書のテストの項は、古い仕様の記述を確かめる確認（D5）を恒久的なテストにしないことを、その理由とともに述べている。
-   **AC-23**: 同じ項は、同じ値を同じ出どころと照合する確認を複数のテストに置かないこと（D7 (a)）と、防ぐ誤りに見合わない文書の確認を足さないこと（D7 (c)）を述べている。
-   **AC-24**: 同じ項の、1 回の変更の性質は `_checks/` で確かめるという既存の規則（`CLAUDE.md:506-509`）と、K4・K5 の例外とその理由は残っている。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。

### 4.2. セキュリティ (Security)

N/A。`docs/dev/security.md` の条件付きガイドの対象外である。本タスクは、外部コマンドの起動、API キーや Webhook URL、キャッシュ、ネットワーク、信頼できないテキストを扱う製品のコードを変えない。信頼できない入力も新たに受け取らない。CI の設定とエージェントの手順の文書（`.claude/commands/`）で変えるのは、文書のガードのパッケージの一覧と拡張のジョブの起動条件だけである（F-007）。

エージェントが実行する手順（`.claude/commands/`）の安全を守る確認（K4・K5）は、1 つも取り除かない（F-001、AC-02）。K4・K5 に当たらない文書の確認を取り除いた後、手順の安全の性質を述べる文が書き換えられても、テストでは気づけない。この残余のリスクは、0038 と同じく PR のレビューでの読み合わせで扱う（0038 4.2）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   N/A。

### 4.4. 互換性 (Compatibility)

-   製品のコード（`_test.go` と `//go:build test` のファイル以外の `cmd/`・`internal/` の Go のコード、`extension/src/`）は変えない。
-   CI の振る舞いで変わるのは、`doc-guard` ジョブなどの対象パッケージ（AC-19）と拡張のジョブの起動条件（AC-21）だけである。

### 4.5. 保守性 (Maintainability)

-   F-001 の D5・D7 の基準を `CLAUDE.md` に記し（F-008）、これからの文書のテストにも当てはめる。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   **実装の開始の条件:** 本タスクの実装（`/runplan`）は、タスク 0051 の最後の PR がマージされてから始める（1 章「実装は 0051 の完了の後に始める」）。本書が挙げる行番号と対象は `66222946` の時点のものであり、設計は、実装を始める時点のテストに基準を当てはめる。
-   `CLAUDE.md`「Deleting a test is a claim that must be checked」に従い、カバレッジを比べる（F-004）。比べる対象は、下の例外のとおり製品のコードの関数に限る。
-   拡張の node のテストを変えるので、`make ext-check` を通す（`CLAUDE.md`「Development Notes」）。
-   本タスクは、既存の方針に対して次の例外を設ける。
    -   **カバレッジの比較の対象:** `CLAUDE.md`「Deleting a test is a claim that must be checked」は、関数ごとの結果が変わらないことを求め、対象の関数を限っていない。本タスクは比べる対象を製品のコードの関数に限る（AC-11）。本タスクは補助（`internal/doctestutil` など、テストのためのコード）そのものを取り除くので、補助の関数の結果は必ず変わるからである。issue #591 も「テスト対象のコードについて」と限っている。
    -   **承認済みの文書の編集:** `docs/dev/conventions/requirements_process.md`「Editing an approved document」は、承認済みの文書の決定を変える場合、Status を `draft` に戻して再承認を求める。本タスクは、実装中の計画（F-006）を `approved` のまま改め、`Comments` に `(decision)` として記録する。検証方法を変える決定は本書で定め、本書の承認がその再承認に当たるためである。0038 F-006 が先例である。

## 6. 用語集 (Glossary)

-   **文書のテスト**、**確認**、**値の照合**、**一部の削除**、**読み合わせ**、**カバレッジの比較**: 0038 の 6 章の定義に従う。`CLAUDE.md` の「文書のガード」（text-sync guards、`doc-guard`）は、文書のテストと同じものを指す。
-   **取り除く:** 確認を、削除・一部の削除・統合のどれかでなくすことの総称。AC-04 の一覧の「削除する」「一部を削除する」「統合する」が、その 3 つに当たる。issue #591 の「縮める」は「一部を削除する」に当たる。
-   **古い仕様:** 退役した、または置き換えた仕様。退役した環境変数・設定の値、移行期間の手順、置き換えた行や節を含む（F-001 の D5）。
-   **1 回の変更の性質:** 過去の 1 つのタスクの変更で、何かが加わった・移った・消えた・変わらなかったこと（F-001 の D6）。`requirements_process.md` §4 の `diff` の検証が確かめるもの。
-   **統合:** 重複する確認のうち 1 つを残し、ほかを取り除くこと（F-001 の D7 (a)・(b)）。
-   **実装中の計画:** 本タスクの実装を始める時点で、まだ実装していない作業を含む承認済みの `03_implementation_plan.md`（F-006）。
-   その他の用語は `docs/translation_glossary.md` と統一する（確認 = check、受け入れ基準 = acceptance criteria、ガードツール = guard tool、自動レビュアーの一覧 = automated reviewer list）。
