# 実装計画書：ADR の残りの候補 20 件と、0058・0059 の決定の反映

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-06 |
| Review date | 2026-11-06 |
| Reviewer | isseis |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番を、「設計書 3.1」などは設計書の節を指す。用語（ADR、主題、候補、調査の番号、出典のタスク、仕様の正本の文書、PR の基点、規約、移す、移し元・移し先、移す項目の一覧、経緯に入るタスク、決定を記録する節）は、要件書 6 章と設計書の冒頭に従う。本タスクに `implementation_handoff.md` はない。[design_handoff.md](design_handoff.md) の H-01〜H-05 は、設計書の付録 A がすべて記録している。

既存のファイルについての記述は、コミット `8437995a`（2026-11-06 の `main`）で確かめた。設計書が照合したコミット `622db232` から `8437995a` までに `main` に入ったのは、#701（0063 の記事の Markdown の解析）、#702（0066 の要件定義）、#703（本タスクの設計書）の 3 つである。この間に、設計書が行番号を引いた ADR（ADR-0001・0004・0005・0006・0008・0009・0010・0011・0012・0014）、`docs/dev/project_overview.md`、`docs/dev/security.md`、規約は変わっていない。`git diff --quiet 622db232 8437995a -- <これらのパス>` が終了コード 0 で終わることで確かめた。変わったのは、ADR-0003（0066 の更新。設計書が引く `:30`・`:59` の行の位置は変わらない）、ADR-0018 の新設（0066）、索引、CLAUDE.md（0063 の例外の段落）、`extension/src/core/articleMarkdown.ts` ほか拡張のファイル（0063）である。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

0060 で ADR にしなかった 20 件の主題を新しい ADR にし、0058・0059 の決定を既存の ADR に反映する。何を書くか（候補の振り分け、移す部分、正本の文書、経緯に入るタスク、0058・0059 の照合、確かめの方法）は、設計書 1〜7 章で決まっている。本書は、それをどの順序で、どの PR に分け、どう確かめるかを定める。あわせて、設計書が実装計画に任せた値（ADR の `<kebab>` の名前、経緯に入れるタスクの一覧、移す項目の一覧）を決める。

### 1.2. 実装の原則

-   フェーズは設計書 8 章の 4 つの PR（①〜④）に合わせ、1 つのフェーズを 1 つの PR にする。
-   ADR に書く内容は設計書 3 章・6.2 を参照し、本書では繰り返さない。ADR の文の最終的な言い方は、書くときに決める。
-   新しい ADR は番号の順に 1 本ずつコミットし、各コミットに、その ADR の索引の行と、その ADR への移し（移し元から除くことと、除いた位置のリンク）を入れる。`TestADRRecords` は、どのコミットでも索引と ADR が一対一で番号が抜けなく続くことを求める（`cmd/yt2column/adr_test.go:88`、`8437995a`）。移しを同じコミットに入れるのは、設計書 5.2 のとおり、決定がどの ADR からも抜け落ちたコミットを作らないためである。例外として、B6 と B9 の ADR は 1 つのコミットにする。ADR-0010 の「結果」の 1 行が、前の 2 文を B6 へ、最後の文を B9 へ移すので、別のコミットにすると、その間のコミットで最後の文がどの ADR にもなくなるためである。
-   リンクとリンク先は、同じコミットに入れるか、リンク先を先にコミットする。各コミットで `make test` が通るようにする。
-   `_checks/` のプログラムは、対象を壊すと `NG` になることを実装のときに確かめ、コミットメッセージにそう書く（CLAUDE.md「Testing Strategy」）。
-   Go のコメント・識別子・文字列リテラルは英語で書く（CLAUDE.md「Go Idioms」）。`_checks/` のプログラムは guard-test literal exception の範囲に入らないので、日本語のリテラルを持たない。日本語の値（移す文）は、本書の表から読む（1.3）。
-   各フェーズの終わりに、グリーンゲート（`make test && make lint`）を通す。本タスクは拡張・README・`docs/dev/guides/development.md`・`.gitignore` を変えないので、`make ext-test` は要らない（CLAUDE.md「Development Notes」）。

### 1.3. 計画の際に決めたこと（2026-11-06）

レビュアーと決めたこと:

-   **今書ける一覧は本書に書き、ADR を書くまで決まらない値は、その ADR を書く PR で埋める。** 本書に、ADR の `<kebab>` と PR（1.5）、ADR ごとの経緯に入れるタスクの番号（1.6）、移す文と書き換える行の PR の基点の文面（1.7）を、今の `main` から書く。移す範囲を、この計画の PR でレビューできるようにするためである。経緯の項目の出典の節と日付（1.6）、リンクを置くために書き換えた行の後の文面（1.7）は、ADR を書いてみないと決まらないので、その ADR を書く PR で埋める。ADR の判断：本タスクの進め方の決定で、タスクの中で閉じるので、ADR は作らない。
-   **A8 の ADR と ADR-0018 は、互いにリンクする。** 0066 が `622db232` の後に作った ADR-0018（`0018-server-config-file.md`）は、「サーバが環境変数から読むのは `HOME` と `GODEBUG` だけ」と決めており、環境変数を読む場所という A8 の主題と重なる。設計書 1.2 の「重なるが移さない部分は、互いにリンクする」を当てはめ、② で A8 の ADR から ADR-0018 へ、ADR-0018 の「決定」の 1 つ目の項目から A8 の ADR へリンクする。ADR-0018 は設計書 2.2 の変更ファイルの一覧にないが、それは設計書の照合の時点で ADR-0018 がなかったためである。設計書 6.1 の「`622db232` の後に `main` に入ったタスクの文書とコードを、PR の主題と照合する」の結果として扱う。ADR-0018 の項目は移さない。A8 の主題はコードのどの場所が環境を読んでよいかで、ADR-0018 の項目はサーバが設定をどこから読み、環境のどの変数を読むかという設定ファイルの主題に属するためである（設計書 3.1 の「移さない重なり」の A8 と ADR-0003 の行と同じ理由）。0066 は要件定義・設計・実装計画まで `main` にあり、実装中である。PR の基点のサーバはまだ設定を環境変数から読む（`cmd/yt2column-server/main.go:29`、`internal/config/config.go:362-383`）。A8 の ADR の「決定」は PR の基点のコードを述べ（AC-03）、ADR-0018 が決めてまだ実装されていない変更は、「決定」に含めず、ADR-0018 へのリンクとともに「結果」に 1 文で記す。0066 は実装中なので、ADR-0018 を同時に変える PR と競合しうる。そのときは規約「Taking in `main`」に従う。ADR の判断：同上。

上の決定と設計書から導いたこと（PR のレビューで異論があれば改める）:

-   **新しい ADR の番号は `0019`〜`0038` にする。** 0066 が `0018` を先に使ったので、設計書 3.1 の「ほかのタスクが先に番号を取ったときは、次の空き番号を使う」に従い、設計書の予定の番号を 1 つずつずらす。対応は 1.5 の表に置く。本書のほかの箇所は 1.5 の表の番号で書く。
-   **`_checks/` のプログラムは、本書の表を ASCII の目印で探して読む。** 設計書 7.3 は、確かめが読む値を実装計画の一覧から読むとしている。移す文は日本語なので、Go のコードに書き写せない。プログラムは、PR の head の本書を `git` から読み、各表の直前に置いた HTML のコメント（`<!-- adrchecks:... -->` の形。1.5〜1.9）で表を探す。確かめが読む値はすべて、この目印の付いた表に置き、プログラムに写さない。
-   **`0058`・`0059` の照合を含む F-002 の値も、同じプログラムで確かめる。** 既存の ADR（ADR-0006・0008・0009・0010・0011）も 1.6 の表に載せ、0058 か 0059 の文書へのリンクを経緯に求める（設計書 7.3 の表の 3 行目）。

### 1.4. 既存コード調査結果

**本タスクが変える ADR の箇所（`8437995a` で確かめた）**

-   設計書 3.1 の「移す部分」と「移さない重なり」の表の `file:line` は、ADR-0003 を除き `622db232` から変わっていない。ADR-0003 の `:30`（正本を示す段落）と `:59`（経緯の 0041 の項目）も、0066 の更新の後で同じ行にある（`sed -n '30p;59p'` で確かめた）。
-   移す文と書き換える行の PR の基点の文面は 1.7 の表に引いた。① が ADR-0010 の `:21`（ビューアの URL の項目）と ADR-0011 の `:36`（採らなかった案の「新しい API」）を変えるが、どちらも 1.7 の表の行ではない。
-   ADR-0018 の「決定」の 1 つ目の項目（`docs/dev/adr/0018-server-config-file.md:17`）は、サーバが環境変数から読むものを `HOME` と `GODEBUG` に限る。A8 の ADR との相互のリンクの対象である（1.3）。

**`project_overview.md` の 3 項目（AC-13）**

-   「決定済みの方針」は文書で最初の `##` の節である（`docs/dev/project_overview.md:7`）。AC-13 の 3 項目は、その節の最上位の箇条の 1 番目（言語。`:9`）、2 番目（実行形式。`:10`）、6 番目（LangChain 相当。`:15`）である。`:13` は 4 番目の項目の下の入れ子の箇条で、数えない。
-   位置のずれを見逃さないための目印は、1 番目は `docs/tasks/0007_browser_extension_skeleton/` のコードスパン、2 番目は `cmd/yt2column-nmhost` のコードスパン、6 番目は ASCII の語 `LangChain` である（6 番目にはコードスパンがない）。
-   2 番目の項目は、0060 が足した ADR-0005・ADR-0008 へのリンクを末尾に持つ。0066 の AC-33 は「方針と設定の節」を書き換えるので、本タスクの PR の間に、この節が変わりうる（5 章）。

**`_checks/` の既存の例**

-   `docs/tasks/0060_adr_records/_checks/task_docs_unchanged.sh` は、マージの基点と head の間で、自分のタスクのディレクトリの外の `docs/tasks/` が変わっていないことを確かめる。`task` の変数を変えれば本タスクに使える（AC-12）。
-   `docs/tasks/0060_adr_records/_checks/adrlinks/main.go` は、`//go:build test` を 1 行目に持ち、`go run -tags test` で動き、2 つのコミットのファイルを `git` から読む。`history-sources`（経緯の節を 6 番目の `##` の節として探し、各タスクの文書へ解決される相対リンクを求める）と `overview-items`（項目を位置と目印で探し、末尾の ADR のリンクの並びを除いた文の比較と、リンク先の比較を行う）は、本タスクの確かめにほぼそのまま使える。`quick-links` と `viewer-appendix` は 0060 だけのもので、使わない。ADR を番号ではなく `<kebab>` で探す作りも、そのまま使う。
-   `_checks/` は `./...` の外にあるので、`make lint` と `make deadcode` の対象にならない。`go vet -tags test` を別に実行する（0060 のステップ 1-6 と同じ）。
-   `/runplan` の手順 6.5 の確かめは、変えた Go のファイルに `AC-NN` があると拒む（`.claude/commands/runplan.md` の手順 6.5）。プログラムは、確かめに内容を表す名前を付け、AC の番号を書かない。

**既存のテスト**

-   `TestADRRecords`（`cmd/yt2column/adr_test.go`）は、新しい ADR と索引の行を、追加の設定なしで確かめる（AC-14）。`TestInstructionDocPaths`（`cmd/yt2column/docpaths_test.go`）は、`docs/dev` の文書のリンク先の実在を確かめる。本タスクはどちらも変えない。
-   `project_overview.md` を読むテスト（`cmd/yt2column/docs_test.go`、`cmd/yt2column-server/docs_test.go`）は、項目の末尾にリンクを足しても、読む値が変わらない（0060 のステップ 2-2 と同じ）。

**型による保証:** 本タスクは製品のコードに Go の型を加えない（設計書 3 章の冒頭）。具体的な型で振る舞いを分ける設計はないので、型の作り方を列挙して確かめる必要もない。

### 1.5. ADR の番号と名前

設計書 3.1 の予定の番号を、1.3 のとおり 1 つずつずらす。`<kebab>` は主題を短く表す英語にする。番号はほかのタスクが先に取ればさらに変わりうるが、`<kebab>` は変えない。この表は AC-01 の主題と ADR の対応表を兼ねる。`_checks/adrchecks` は、この表の `<kebab>` と PR を読む。

<!-- adrchecks:adrs -->
| 設計書の予定の番号 | 本書の予定の番号 | 調査の番号 | `<kebab>` | PR |
|---|---|---|---|---|
| 0018 | 0019 | A1 | `pipeline-component-interfaces` | 2 |
| 0019 | 0020 | A2 | `strict-json` | 2 |
| 0020 | 0021 | A4 | `llm-client-contract` | 2 |
| 0021 | 0022 | A5 | `cost-estimate` | 2 |
| 0022 | 0023 | A7 | `artifact-resume` | 2 |
| 0023 | 0024 | A8 | `env-access-in-config` | 2 |
| 0024 | 0025 | A11 | `claude-output-limit` | 2 |
| 0025 | 0026 | B3 | `extension-platform` | 3 |
| 0026 | 0027 | B6 | `article-job-api` | 3 |
| 0027 | 0028 | B9 | `no-automatic-resend` | 3 |
| 0028 | 0029 | B10 | `extension-ui-languages` | 3 |
| 0029 | 0030 | B12 | `minimum-chrome-version` | 3 |
| 0030 | 0031 | C1 | `secret-closure` | 4 |
| 0031 | 0032 | C2 | `testdata-license` | 4 |
| 0032 | 0033 | C3 | `transcript-cache-slots` | 4 |
| 0033 | 0034 | C4 | `template-allowlist` | 4 |
| 0034 | 0035 | D5 | `language-policy` | 4 |
| 0035 | 0036 | D6 | `task-document-status-records` | 4 |
| 0036 | 0037 | D7 | `test-deletion-coverage` | 4 |
| 0037 | 0038 | D8 | `deadcode-and-race-free-timing` | 4 |

### 1.6. 経緯に入れるタスク

設計書 3.3 の表の「出典のタスク」と「ほかに入るタスク」を合わせたもの（括弧の注記は除く）と、F-002 で経緯に項目を足す既存の ADR である。「出典の節・日付」の列は、その ADR を書く PR で埋める（1.3）。日付は規約「History entries」の「Date of an entry copied from an existing decision」に、項目の並びは設計書 1.2（出典の節を `main` に加えた PR のマージの順）に従う。実装で、表にないタスクが設計書 3.3 の規則に当たると分かったときは行に足し、表のタスクが規則に当たらないと分かったときは除き、どちらも「設計書との違い」の列に理由を書く（設計書 3.3・4 章）。除くタスクが要件書 F-001 の出典のタスクなら、設計書を改める（設計書 3.3「入れないもの」）。`_checks/adrchecks` は、この表の `<kebab>` と「タスク」の列を読む。

<!-- adrchecks:history -->
| `<kebab>` | タスク | 出典の節・日付 | 設計書との違い |
|---|---|---|---|
| `threat-model` | 0058 | 0058 要件定義書「要件定義の際に決めたこと（2026-11-03）」・2026-11-03（節の見出しの日付） | - |
| `launch-path` | 0058 | 0058 要件定義書「要件定義の際に決めたこと（2026-11-03）」・2026-11-03（節の見出しの日付） | - |
| `viewer-opening` | 0058 | 0058 要件定義書「要件定義の際に決めたこと（2026-11-03）」・2026-11-03（節の見出しの日付）、0058 設計書「設計の際に決めたこと（2026-11-04）」・2026-11-04（節の見出しの日付） | - |
| `extension-stores-nothing` | 0058 | 0058 要件定義書「issue #593 で決めたこと」・2026-11-04（変更履歴の最初の承認）、0058 設計書「設計の際に決めたこと（2026-11-04）」・2026-11-04（節の見出しの日付） | - |
| `webhook-publishing` | 0059 | 0059 要件定義書「issue #526 で決めたこと」・2026-11-04（変更履歴の承認） | - |
| `pipeline-component-interfaces` | 0001 0003 0009 | 0001 設計書「1.1 設計原則」・2026-09-29（設計書の Review date）、0003 設計書「1.1 設計原則」・2026-10-02（同）、0009 設計書「1.1 設計原則」・2026-10-10（同） | - |
| `strict-json` | 0003 0010 0019 0023 0044 | 0003 設計書「1.1 設計原則」・2026-10-02（設計書の Review date）、0010 設計書「3.5.3」・2026-10-10（同）、0019 設計書「3.3」・2026-10-15（同）、0023 設計書「1. 設計の全体像」・2026-10-16（同）、0044 設計書「3.10」・2026-10-27（同） | - |
| `llm-client-contract` | 0001 0003 0019 0022 | 0001 設計書「付録A: 決定履歴」・2026-09-29（設計書の Review date）、0003 設計書「1.3・3.2」・2026-10-02（同）、0019 設計書「3.2」・2026-10-15（同）、0022 設計書「付録 A」・2026-10-16（同） | - |
| `cost-estimate` | 0022 0042 | 0022 設計書「1.1 設計原則」・2026-10-16（設計書の Review date）、0042 設計書「1.2. 設計の際に決めたこと（2026-10-25）」・2026-10-25 | - |
| `artifact-resume` | 0017 0019 0022 | 0017 設計書「1.1 設計原則」・2026-10-14（設計書の Review date）、0019 設計書「3.7」・2026-10-15（同）、0022 設計書「3.5」・2026-10-16（同） | - |
| `env-access-in-config` | 0005 0009 0011 0026 0055 | 0005 設計書「3.1」・2026-10-05（設計書の Review date）、0009 設計書「5.2」・2026-10-10（同）、0011 設計書「3.14」・2026-10-13（同）、0026 設計書「3.8」・2026-10-19（同）、0055 設計書「3.11」・2026-11-02（同） | 0009 設計書「5.2」で `ANTHROPIC_API_KEY` を秘密情報の変数名の一覧に加えたため、経緯に 0009 を足した |
| `claude-output-limit` | 0009 0041 | 0009 設計書「3.6」・2026-10-10（設計書の Review date）、0041 設計書「1.2. 設計の際に決めたこと（2026-10-25）」・2026-10-25 | - |
| `extension-platform` | 0007 0026 0050 | （③ で埋める） | - |
| `article-job-api` | 0044 0050 0058 0059 | （③ で埋める） | - |
| `no-automatic-resend` | 0011 0020 0026 0044 0050 | （③ で埋める） | - |
| `extension-ui-languages` | 0007 0043 0050 | （③ で埋める） | - |
| `minimum-chrome-version` | 0007 0020 0026 0046 0050 | （③ で埋める） | - |
| `secret-closure` | 0001 | （④ で埋める） | - |
| `testdata-license` | 0002 | （④ で埋める） | - |
| `transcript-cache-slots` | 0002 0005 | （④ で埋める） | - |
| `template-allowlist` | 0004 0010 | （④ で埋める） | - |
| `language-policy` | 0032 0040 0043 0048 0052 0060 0061 | （④ で埋める） | - |
| `task-document-status-records` | 0053 0056 0060 | （④ で埋める） | - |
| `test-deletion-coverage` | 0027 0038 0052 0054 0055 | （④ で埋める） | - |
| `deadcode-and-race-free-timing` | 0015 0039 0047 | （④ で埋める） | - |

`8437995a` までに `main` に入った 0066 の要件定義書は、決定を記録する節で、サーバが設定ファイルから読み、環境からは `HOME` と `GODEBUG` だけを読むと決めた。② の時点では、0066 の設計書と実装計画も `main` にある。設計書までを照合した結果、0066 が決めたのは、サーバが設定をどこから読み、環境のどの変数を読むかという ADR-0018 の主題である。コードのどの場所が環境を読んでよいかという A8 の主題は決めておらず、その制限（`internal/config/envaccess_test.go` の許す場所）との関係は、0066 の [design_handoff.md](../0066_server_config_file/design_handoff.md) が設計に任せている。そのため `env-access-in-config` の行に 0066 を入れない。

### 1.7. 移す項目の一覧

設計書 3.1 の「移す部分」の文と項目と、「移さない重なり」でリンクを足す行を、`8437995a` の文面のまま、行の全体で引く。「種類」は次の 3 つである。

-   `move`：行の全体を移し先へ移し、移し元から行を除く。
-   `split`：行の一部を移し先へ移し、残りの部分と、移した位置のリンクで行を書き換える。
-   `link`：何も移さず、重なる新しい ADR へのリンクを足すために行を書き換える。

`split` と `link` の「書き換えた後の文面」には書き換えた後の行の全体を書き、その行を変える PR で埋める（1.3）。どの文面も、行の前後の空白と改行を除いた行の全体の一致で比べる（表のセルは前後の空白を保てないため。ADR-0014 の行は字下げした入れ子の箇条である）。セルの中の `|` は `\|` と書き、プログラムは `\|` を `|` に戻して比べる。

ADR を書くときに、移し元の「背景」「採らなかった案」「結果」に、移す決定だけを述べる項目がほかにも見つかったら、`move` の行を足す（設計書 1.2）。移す決定と残る決定の両方に触れる項目は、意味を変えずに分けて、`split` の行にする。分けられないときは、止まって利用者に確かめる（設計書 4 章）。新しい ADR へのリンクを既存の ADR に足すときは、どの節の行でも `link` の行を足す。PR の基点の文面が `main` の取り込みで変わったときは、取り込んだ後の文面に直す（設計書 4 章）。`_checks/adrchecks` は、この表のすべての列を読む（7.1 節の `moved-items`・`move-links`）。

<!-- adrchecks:moves -->
| PR | 移し元 | 移し先 | 種類 | PR の基点の文面 | 書き換えた後の文面 |
|---|---|---|---|---|---|
| 2 | `llm-adapter-stdlib` | `pipeline-component-interfaces` | move | -   プロバイダに依存しない型と番兵: `GenerateRequest`・`GenerateResponse`・`LLMClient`、および不正なリクエスト・打ち切り・想定外の終了理由・空の応答。 | - |
| 2 | `llm-adapter-stdlib` | `pipeline-component-interfaces` | move | -   **プロバイダに依存しない型と番兵は `internal/llm` に置く。** `LLMClient` と `GenerateRequest`・`GenerateResponse`、`ErrInvalidRequest`・`ErrTruncated`・`ErrUnexpectedFinishReason`・`ErrEmptyResponse`、リクエストの検証はここにある。 | - |
| 2 | `llm-adapter-stdlib` | `pipeline-component-interfaces` | split | -   **HTTP の送信の経路は `internal/llm/llmhttp` に置き、両方のアダプタが使う。** 期限の適用、リダイレクトの禁止、応答本文の上限、失敗の分類をここで行う。`llmhttp` はプロバイダ固有の番兵を持たず、アダプタが渡した番兵で失敗を報告する。HTTP の状態・応答本文の形・通信の失敗の番兵は、プロバイダの名前を持つので各アダプタに残す。 | -   **HTTP の送信の経路は `internal/llm/llmhttp` に置き、両方のアダプタが使う。** 期限の適用、リダイレクトの禁止、応答本文の上限、失敗の分類をここで行う。`llmhttp` はプロバイダ固有の番兵を持たず、アダプタが渡した番兵で失敗を報告する。番兵の置き場所は [ADR-0019](0019-pipeline-component-interfaces.md) が定める。 |
| 2 | `llm-adapter-stdlib` | `pipeline-component-interfaces` | move | -   **番兵をすべて各アダプタに置く。** `ArticleWriter` が打ち切りを判別するために、プロバイダのパッケージを import することになる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。 | - |
| 2 | `llm-adapter-stdlib` | `pipeline-component-interfaces` | move | -   **番兵をすべて `internal/llm` に置く。** SDK を使うプロバイダが、HTTP ステータスや応答本文の形に固有の番兵を `internal/llm` に合わせる必要が生じる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。 | - |
| 2 | `llm-adapter-stdlib` | `pipeline-component-interfaces` | split | -   プロバイダを足すときは、アダプタのパッケージと、そのプロバイダのパラメータの型のパッケージを足す。`internal/llm` の型と `llmhttp` は変えない。 | -   プロバイダを足すときは、アダプタのパッケージと、そのプロバイダのパラメータの型のパッケージを足す。`llmhttp` は変えない。プロバイダに依存しない型は [ADR-0019](0019-pipeline-component-interfaces.md) が定める。 |
| 2 | `llm-adapter-stdlib` | `pipeline-component-interfaces` | move | -   プロバイダ固有の失敗（HTTP ステータス・応答本文の形・通信の失敗）は、プロバイダごとの番兵で報告される。呼び出し側は、プロバイダに依存しない 4 つの番兵だけで、打ち切り・想定外の終了理由・空の応答・不正なリクエストを判別できる。 | - |
| 2 | `llm-adapter-stdlib` | `strict-json` | split | -   応答の厳格な検証: 標準ライブラリのデコーダが黙って補正する入力を拒否する部品。 | -   応答の厳格な検証: 標準ライブラリのデコーダが黙って補正する入力を拒否する部品（[ADR-0020](0020-strict-json.md) が定める）。 |
| 2 | `llm-adapter-stdlib` | `strict-json` | move | -   **応答の厳格な検証は `internal/strictjson` に置き、複数のパッケージで共有する。** 字幕のパーサ、両方のアダプタ、`ArticleWriter`、`internal/publisher`、`internal/artifact`、`internal/fixprguard`、`internal/server`、`internal/nmhost`、`internal/codexreview`、`internal/coveragecompare` が使う。 | - |
| 2 | `llm-adapter-stdlib` | `strict-json` | move | -   **厳格な JSON の部品を `internal/llm/deepseek` に複製する。** サロゲートの走査などの細かいコードが 2 か所になる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。 | - |
| 2 | `llm-adapter-stdlib` | `strict-json` | move | -   **`internal/transcript` の非公開の関数を公開して LLM の段階から使う。** LLM の段階が字幕取得の段階に依存する（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。 | - |
| 2 | `server-architecture` | `pipeline-component-interfaces` | link | -   **サーバは `cmd/yt2column-server` の別の実行ファイルとし、利用者が手動で起動する。** ログイン時の自動起動はしない。記事を作る処理と Slack への投稿の処理は、CLI と同じパッケージを共有し、サーバ固有のコードは HTTP の入口・検査・応答・ジョブに限る。 | -   **サーバは `cmd/yt2column-server` の別の実行ファイルとし、利用者が手動で起動する。** ログイン時の自動起動はしない。記事を作る処理と Slack への投稿の処理は、CLI と同じパッケージを共有し、サーバ固有のコードは HTTP の入口・検査・応答・ジョブに限る。記事を作る処理と Slack への投稿の処理を共有する構成は [ADR-0019](0019-pipeline-component-interfaces.md) の構成要素の分け方に従う。 |
| 2 | `llm-config-contract` | `env-access-in-config` | link | この決定の仕様の正本は [project_overview.md](../project_overview.md) の設定の節である。値の解析と既定の当てはめは `internal/config` が行い、effort の値の一覧は `internal/llm/claudeparam` と `internal/llm/deepseekparam` が 1 か所ずつ持つ。 | この決定の仕様の正本は [project_overview.md](../project_overview.md) の設定の節である。値の解析と既定の当てはめは `internal/config` が行い、effort の値の一覧は `internal/llm/claudeparam` と `internal/llm/deepseekparam` が 1 か所ずつ持つ。環境変数を読む場所は [ADR-0024](0024-env-access-in-config.md) が定める。 |
| 2 | `server-config-file` | `env-access-in-config` | link | -   **サーバは設定ファイルだけから設定を読み、CLI は今までどおり環境変数だけから読む。** 入口ごとに読む場所を 1 つに決め、どの値が効いているかを曖昧にしない。CLI を環境変数のままにするのは、設定ファイルが解く問題を CLI が持たないからである。CLI は利用者がシェルから実行するので、`.envrc` の値が毎回届く。CLI は開発の経路で、`YT2COLUMN_LLM=… yt2column …` のように実行ごとに値を変えて試す使い方があり、ファイルだけから読むとそれを失う。二系統を保つコストは、両方が同じキーの表と同じ値の検証（[ADR-0003](0003-llm-config-contract.md)）を通ることで小さく保つ。サーバが環境変数から読むのは、パスを導出する `HOME` と、Go のランタイムが出す HTTP/2 のデバッグログについて警告するための `GODEBUG` だけである。 | -   **サーバは設定ファイルだけから設定を読み、CLI は今までどおり環境変数だけから読む。** 入口ごとに読む場所を 1 つに決め、どの値が効いているかを曖昧にしない。CLI を環境変数のままにするのは、設定ファイルが解く問題を CLI が持たないからである。CLI は利用者がシェルから実行するので、`.envrc` の値が毎回届く。CLI は開発の経路で、`YT2COLUMN_LLM=… yt2column …` のように実行ごとに値を変えて試す使い方があり、ファイルだけから読むとそれを失う。二系統を保つコストは、両方が同じキーの表と同じ値の検証（[ADR-0003](0003-llm-config-contract.md)）を通ることで小さく保つ。サーバが環境変数から読むのは、パスを導出する `HOME` と、Go のランタイムが出す HTTP/2 のデバッグログについて警告するための `GODEBUG` だけである。環境変数を読む場所の規則は [ADR-0024](0024-env-access-in-config.md) が定める。 |
| 2 | `production-profile` | `claude-output-limit` | move | -   **Claude アダプタの出力の上限の既定は 40,000 トークンとし、3 つの段階で共有する。** 本番の構成で長い動画の出力が打ち切られないようにするためである。 | - |
| 3 | `extension-stores-nothing` | `article-job-api` | move | -   **サーバの状態はメモリにだけ置く。** 記事のキャッシュはサーバのプロセスの中だけにあり、ディスクに書かない（サーバの構成は [ADR-0005](0005-server-architecture.md)）。サーバを再起動すると、記事と投稿の状態はすべて失われ、ビューアは記事が失われたことを示す。これは受け入れる。 | - |
| 3 | `extension-stores-nothing` | `article-job-api` | move | -   サーバを再起動すると記事と投稿の状態が失われる。利用者は記事を作り直す。投稿済みの記事を送り直すと、同じ内容がもう一度投稿されうる。 | - |
| 3 | `webhook-publishing` | `no-automatic-resend` | split | -   **成功は、HTTP の状態コードが 200 で、応答の本文が `ok` のときとする。** 1 通ごとに 30 秒で打ち切り、応答の本文は上限までしか読まない。リダイレクトには従わない。投稿は冪等ではないので、自動で再送しない。 | （③ で埋める） |
| 3 | `webhook-publishing` | `no-automatic-resend` | move | -   **サーバが記事を保持している間は、投稿を冪等にする。** 投稿中または投稿済みの記事への投稿の操作には、投稿し直さずに受け付けたときと同じ応答を返す。記事が長いときの分割と、分割した投稿の途中で失敗したときの報告は、[security.md](../security.md) §3 と §8.2 に従う。 | - |
| 3 | `webhook-publishing` | `no-automatic-resend` | move | -   **投稿を自動でリトライする（HTTP 429 の `Retry-After` に従う待機を含む）。** Incoming Webhook への投稿は冪等ではなく、応答を受け取れなかったメッセージを再送すると、同じメッセージが重複して投稿されうる（[0006 要件定義書](../../tasks/0006_slack_webhook_publisher/01_requirements.md) §2.3）。 | - |
| 3 | `webhook-publishing` | `article-job-api` | link | -   **投稿は利用者の操作でだけ行う。** Webhook へ投稿するのはサーバだけであり、ビューアからの投稿の操作（`POST /jobs/{key}/publish`）だけが投稿を始める。サーバの投稿のジョブは、`PrepareSlackMessages` で記事を検査・分割し、`PostMessages` で送る。CLI は投稿せず、記事を `--out` のファイルに書くだけである。生成の完了時に自動で投稿する予約は設けない。 | （③ で埋める） |
| 3 | `server-architecture` | `article-job-api` | link | -   **生成の開始は `202` で受け付け、ジョブをサーバのメモリで動かす。** 同時に走る生成の数、本文の大きさ、読み取り・書き込み・待機の時間の上限は [security.md](../security.md) §8.2 に従う。 | （③ で埋める） |
| 3 | `server-architecture` | `extension-platform` | link | -   **拡張は Native Messaging のホストだけに送り、ホストが UDS へ中継する。** 送り先は固定のホスト名 `io.github.isseis.yt2column` で、ホストを起動できる拡張は manifest の `allowed_origins` に載った固定の拡張だけである。ホストは 1 つのメッセージにつき 1 回起動し、1 つのリクエストを送って 1 つの応答を返し、終了する。ホストはサーバを起動せず、ジョブはサーバに残る。ブラウザを閉じてもジョブは続く。 | （③ で埋める） |
| 3 | `threat-model` | `extension-platform` | link | -   **Native Messaging のホストを起動できる拡張を固定し、秘密は `secret.Secret` で持つ。** ホストの `allowed_origins` は固定した拡張 ID のオリジンだけとし、API キーと Webhook URL は、送信先へのリクエストを除き、エラーやログを含むどの出力にも漏らさない。 | （③ で埋める） |
| 4 | `lightweight-check-in-bash` | `language-policy` | link | -   スクリプトとそのテストには、仕様の見出しのための日本語のリテラルの例外がある（`CLAUDE.md` の「Go Idioms」）。 | （④ で埋める） |
| 4 | `deterministic-checks-in-go` | `test-deletion-coverage` | link |     -   `coveragecompare`: テストを削除する前後でのカバレッジの比較。`make coverage-compare` と `make ext-coverage-compare` から使う。 | （④ で埋める） |
| 4 | `threat-model` | `secret-closure` | split | -   **Native Messaging のホストを起動できる拡張を固定し、秘密は `secret.Secret` で持つ。** ホストの `allowed_origins` は固定した拡張 ID のオリジンだけとし、API キーと Webhook URL は、送信先へのリクエストを除き、エラーやログを含むどの出力にも漏らさない。 | （④ で埋める） |
| 4 | `webhook-publishing` | `secret-closure` | split | -   **Webhook URL は `secret.Secret` で持ち、値を出力に流さない。** URL は、送信先への HTTP リクエストを除き、エラー・ログ・標準出力を含むどの出力にも出さない。`*url.Error` をそのまま返すと URL が現れるので、送信の失敗は URL を含まないエラーに直す。 | （④ で埋める） |

ADR-0010 の結果の行（`extension-stores-nothing` → `article-job-api` の 2 行目）は、前の 2 文を B6 の ADR へ、最後の文「投稿済みの記事を送り直すと、同じ内容がもう一度投稿されうる。」を B9 の ADR へ移す（設計書 3.1）。行の全体を除くので `move` とし、移し先は前の 2 文の B6 とする。B6 と B9 の ADR を 1 つのコミットに入れるので（1.2）、最後の文がどの ADR にもないコミットはできない。B9 の ADR がこの文を述べることは、PR のレビューで読む（7 章 AC-02）。

`threat-model` の `allowed_origins` の行は、③ で B3 へのリンクを足し（`link`）、④ で C1 の部分を移す（`split`）。③ の PR は、③ の行の「書き換えた後の文面」を埋めるコミットで、④ の行の「PR の基点の文面」を同じ文面に改める。

### 1.8. `project_overview.md` の項目

AC-13 の 3 項目を、「決定済みの方針」の最上位の箇条の位置と、位置のずれを見逃さないための ASCII の目印（1.4）で示す。「足すリンク先」は設計書 3.6 の表の ADR である。`_checks/adrchecks` は、この表を読む（7.1 節の `overview-items`）。

<!-- adrchecks:overview -->
| 位置 | 目印 | 足すリンク先 |
|---|---|---|
| 1 | `docs/tasks/0007_browser_extension_skeleton/` | `extension-platform` |
| 2 | `cmd/yt2column-nmhost` | `article-job-api` |
| 6 | LangChain | `pipeline-component-interfaces` |

### 1.9. 正本の文書と issue へのリンク

新しい ADR の「決定」がリンクする正本の文書を、設計書 3.2 の表からファイルの単位で写す。節の名前は表に入れず、節を正しく指すかは PR のレビューで読む。正本の文書がない A8（`env-access-in-config`）は行を置かない。`_checks/adrchecks` は、この表を読む（7.1 節の `spec-links`）。

<!-- adrchecks:specs -->
| `<kebab>` | 正本の文書 |
|---|---|
| `pipeline-component-interfaces` | CLAUDE.md docs/dev/project_overview.md docs/dev/conventions/package_reference.md |
| `strict-json` | docs/dev/conventions/package_reference.md |
| `llm-client-contract` | docs/dev/project_overview.md |
| `cost-estimate` | docs/dev/guides/pricing_update.md docs/dev/guides/cli.md |
| `artifact-resume` | docs/dev/guides/cli.md docs/dev/security.md |
| `claude-output-limit` | docs/dev/guides/claude_output_limit.md docs/dev/production_profile.md |
| `extension-platform` | CLAUDE.md docs/dev/security.md internal/nmhost/manifest.go |
| `article-job-api` | docs/dev/guides/development.md docs/dev/security.md docs/dev/conventions/package_reference.md |
| `no-automatic-resend` | docs/dev/security.md docs/dev/guides/development.md |
| `extension-ui-languages` | CLAUDE.md README.md |
| `minimum-chrome-version` | extension/static/manifest.json |
| `secret-closure` | docs/dev/security.md docs/dev/conventions/package_reference.md |
| `testdata-license` | docs/dev/security.md testdata/README.md |
| `transcript-cache-slots` | docs/dev/cache_consistency.md |
| `template-allowlist` | prompts/README.md docs/dev/security.md |
| `language-policy` | CLAUDE.md .claude/commands/_context.md .claude/commands/mktrans.md |
| `task-document-status-records` | docs/dev/conventions/requirements_process.md |
| `test-deletion-coverage` | CLAUDE.md |
| `deadcode-and-race-free-timing` | CLAUDE.md |

AC-03 の食い違い（設計書 3.5）を記録した issue を、その ADR を書く PR で埋める。実装でほかの食い違いが見つかったら行を足す。`_checks/adrchecks` は、この表を読む（7.1 節の `issue-links`）。

<!-- adrchecks:issues -->
| `<kebab>` | issue の番号 |
|---|---|
| `strict-json` | 713 |
| `extension-ui-languages` | （③ で埋める） |
| `template-allowlist` | （④ で埋める） |

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: 0058・0059 の反映と確かめのプログラム（設計書 8 章の ①）

#### ステップ 1-1: 既存の ADR への 0058・0059 の反映

**変更するファイル:** `docs/dev/adr/0006-threat-model.md`、`0008-launch-path.md`、`0009-viewer-opening.md`、`0010-extension-stores-nothing.md`、`0011-webhook-publishing.md`、本書（1.6 の表）

-   [x] `8437995a` の後に `main` に入ったタスクの文書とコードを、3.4 の 5 本の主題と照合する（設計書 6.1）。結果を PR の説明に書く。
-   [x] ADR-0006：経緯に 0058 の T-10 の再評価の項目を足し、「結果」に T-10 を受け入れるリスクとして 1 項目を足す（設計書 3.4、AC-08）。
-   [x] ADR-0008：「決定」と「結果」の、生成の開始を主ボタンだけが送るとする箇所（設計書 3.4 の `:18`・`:20`・`:36`。`:36` は「結果」）を、主ボタンと副次のボタンの 2 つが送る形に改め、経緯に 0058 の項目を足す（設計書 3.4、AC-11）。改める前に、`extension/src/launch.ts` の副次のボタンの処理を PR の基点で読み直す。
-   [x] ADR-0009：「決定」に、ポップアップの選択の状態、照会付きの開始で既存の記事を表示すること、ポップアップが前面のタブを開く操作を書き足し、経緯に 0058 の項目を足す（設計書 3.4、AC-11）。
-   [x] ADR-0010：「決定」のビューアの URL に持つ値に `e` を足し、キーを持つ条件を今のコード（`extension/src/core/viewerUrl.ts`）に合わせ、経緯に 0058 の項目を足す（設計書 3.4、AC-09・AC-11）。
-   [x] ADR-0011：経緯に 0059 の失敗の通知の削除の項目を足し、採らなかった案の「新しい API の失敗を…」を「ジョブの API の失敗を…」に改める（設計書 3.4、AC-10）。
-   [x] 経緯の項目は設計書 1.2 の順の位置に入れる（ADR-0011 の 0059 の項目は、0062 の項目の前）。1.6 の表の 5 行の「出典の節・日付」を埋める。
-   [x] ADR ごとに 1 コミットにする。

**完了の条件:** `go test -tags test -run 'TestADRRecords|TestInstructionDocPaths' ./cmd/yt2column` が通る。

#### ステップ 1-2: 1 回だけの確かめ

**変更するファイル:** `docs/tasks/0065_adr_remaining/_checks/task_docs_unchanged.sh`（新規）、`docs/tasks/0065_adr_remaining/_checks/adrchecks/main.go`（新規。`//go:build test`）

-   [x] 0060 の `task_docs_unchanged.sh` を写し、`task` の変数と冒頭のコメントのタスク名・使い方の例だけを本タスクのものに変える。実行の権限を付ける（AC-12）。
-   [x] 0060 の `_checks/adrlinks` を写して `_checks/adrchecks` を作る。`go run -tags test ./docs/tasks/0065_adr_remaining/_checks/adrchecks/ -pr <N> <merge-base> <head>` で動かし、ファイルは `git` から読む。値は head の本書の 1.5〜1.9 の表を、直前の `<!-- adrchecks:... -->` の目印で探して読む（1.3）。確かめは 7.1 節の 7 つで、0060 の `quick-links` と `viewer-appendix` は除く。終了コードは 0060 と同じ（実行できない確かめがあれば 2、`NG` があれば 1、それ以外は 0）とし、`-final` は持たない。`-pr 4` では、後の PR で作る ADR がないので `PENDING` が出ない。
-   [x] ADR の節は、日本語の見出しではなく `##` の位置で探す（2〜5 番目が「背景」「決定」「採らなかった案」「結果」、6 番目が「経緯」）。
-   [x] `PENDING` は、1.5 の表で `-pr` より後の PR に当たる ADR と行にだけ許す。`-pr` 以前の PR に当たるものが見つからなければ `NG` とする（設計書 7.3「`PENDING` を許す範囲」）。
-   [x] 表の値の食い違いは終了コード 2 で止める。対象は、目印のない表、列の数の合わない行、1.7 の「移し先」の `<kebab>` や 1.8・1.9 の `<kebab>` が 1.5 の表にないこと、1.7 の行の PR が移し先の 1.5 の PR と違うこと、1.6 と 1.7 の「移し元」の `<kebab>` が 1.5 の表になく、マージの基点の ADR にもないこと、`-pr` の値が 1〜4 でないことである。
-   [x] 各確かめが、壊した対象だけで `NG` になることを確かめる。壊した状態は一時的なブランチにコミットし、壊さない状態での出力と比べて、対象とした確かめの行だけが `OK` から `NG` に変わることを確かめる。一時的なブランチでは、本書の表を編集し、壊す行だけを確かめの対象の PR に入れる。① には新しい ADR がないので、一時的な ADR と移しもそのブランチに置く。壊し方は、経緯のリンクを 1 つ消す、`-pr` 以前の PR の ADR の `<kebab>` を変える（`PENDING` ではなく `NG` になる）、`move` の行を移し元に残す、`split` の行の書き換えた後の文面を head に置かない、前の PR の行の文面を移し元に戻す、表にない行を移し元から消す、移し元の「決定」のリンクを消す、正本の文書へのリンクを消す、issue へのリンクを消す、`project_overview.md` の項目の文を 1 文字変える、項目のリンク先を別の ADR に変える、CLAUDE.md を 1 文字変える、である。終了コード 2 の各場合も、1 つずつ起こして確かめる。結果をコミットメッセージに書き、一時的なブランチは消す。
-   [x] （① では、手順 7 のレビューで見つかった誤りを、ADR とは別のコミットで直して確かめ直した。② 以降でも当てはめる）② 以降で `_checks/adrchecks` の誤りが見つかったら、ADR とは別のコミットで直し、その確かめを壊して確かめ直し、コミットメッセージに書く。直した後に、その PR の `-pr` で実行し直す。
-   [x] `task_docs_unchanged.sh` が、一時的なブランチでほかのタスクの文書を 1 つ変えると 1 で終わることを確かめ、結果をコミットメッセージに書く。

**完了の条件:** `make fmt` と `go vet -tags test ./docs/tasks/0065_adr_remaining/_checks/adrchecks/` が通る。

#### ステップ 1-3: 確かめの実行と記録

**変更するファイル:** 本書 7.2 節

-   [x] ほかの作業がすべて終わった後に、PR のブランチで `git fetch origin main` を行い、`task_docs_unchanged.sh origin/main HEAD` と `adrchecks -pr 1 "$(git merge-base origin/main HEAD)" HEAD` を実行する。出力・終了コード・マージの基点と head のコミット ID を 7.2 節に書く。記録する head は、記録を書く前のコミットである。記録を書くコミットは本書の 7.2 節だけを変えるので、確かめる ADR と文書の中身は記録した head と同じであり、記録のコミットの後に実行し直す必要はない（設計書 6.1）。
-   [x] （PR を開く時点の head で実行した。`/fixpr` や `main` の取り込みで head が変わったら当てはめる）head が変わったら、マージの前に最後の head で実行し直し、記録を改める（設計書 6.1）。

**完了の条件:** グリーンゲートが通る。`task_docs_unchanged.sh` が 0 で終わる。`adrchecks -pr 1` が 0 で終わる。確かめの結果は、`claude-md-unchanged` が `OK`、ほかの 6 つが `PENDING` である。`PENDING` の 6 つのうち、`history-sources` の中の既存の 5 本の項目は `ok`、`overview-items` の中の文の比較は `OK` で、`PENDING` になるのは後の PR の ADR と行に当たる項目だけである。

### PR-1 作成ポイント: 0058/0059 reflection and one-off checks

**対象ステップ**: 1-1 / 1-2 / 1-3

**推奨タイトル**: `docs(0065): 0058・0059 の決定を既存の ADR に反映し、1 回だけの確かめを加える`

**レビュー観点**: ADR-0008・0009・0010 の「決定」と ADR-0008 の「結果」が今のコードと食い違わないこと / 経緯の項目の位置と出典 / `_checks/adrchecks` が本書の表を目印で読み、壊した確かめの行だけが `NG` になることの記録 / 終了コード 2 の場合

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 1-2 の `_checks/adrchecks` は、本書の 6 つの表を目印で読み、7 つの確かめで `-pr` ごとに `PENDING` と `NG` を分け、基点と head の行の差分を表の行と照合する。壊して確かめる場合も多く、隔離すべき複雑な処理である

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/710
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: A 系の ADR（設計書 8 章の ②）

#### ステップ 2-1: A 系の 7 本の ADR

**変更するファイル:** `docs/dev/adr/0019-pipeline-component-interfaces.md`〜`0025-claude-output-limit.md`（新規。1.5 の表の PR 2 の 7 本）、`docs/dev/adr/0001-llm-adapter-stdlib.md`、`0003-llm-config-contract.md`、`0004-production-profile.md`、`0005-server-architecture.md`、`0018-server-config-file.md`、`docs/dev/adr/README.md`、本書（1.6・1.7・1.9 の表）

-   [x] 索引を読み直し、A 系の主題の有効な ADR がほかのタスクで作られていないことと、`0019` が空いていることを確かめる。番号が埋まっていれば、規約「Location and naming」に従って次の空き番号を使い、1.5 の表の「本書の予定の番号」を改める。
-   [x] `622db232` から PR の基点までに `main` に入ったタスクの文書とコードを、A 系の 7 つの主題と照合する（設計書 6.1）。0066 の設計書と実装計画は `main` にあり、A8 の経緯に入らないと判断した（1.6）。0009 は秘密情報の変数名の一覧に `ANTHROPIC_API_KEY` を加えたため、1.6 の表に足した。結果を PR の説明に書く。
-   [x] 主題ごとに、1.6 の表のタスクの `docs/tasks/<task>/` の下の文書、設計書 3.2 の正本の文書、PR の基点のコードを読み、設計書 6.2 のとおりに ADR を書く。1.6 の表の「出典の節・日付」を埋める。
-   [x] 1.7 の表の PR 2 の行を移す。移し元から除くことと、除いた位置に移し先の ADR へのリンクを置くことは、移し先の ADR と同じコミットに入れる（1.2）。`split` と `link` の行の「書き換えた後の文面」を埋める。
-   [x] 設計書 3.1 の「移さない重なり」の A1（ADR-0005 の「決定」の 1 つ目の項目）と A8（ADR-0003 の正本を示す段落）、1.3 の ADR-0018 の「決定」の 1 つ目の項目に、新しい ADR へのリンクを足し、新しい ADR からもリンクする。これらの行は 1.7 の表の `link` の行である。表にない行にリンクを足すときは、`link` の行を足す。
-   [x] A2 の ADR の「決定」には、`package_reference.md` の `strictjson` の使い手の一覧とコードのずれ（設計書 3.5）を書く。ADR の文面が固まった後に、利用者の承認を得て、同じずれの issue がないかを探し、なければ作り、ADR からリンクする。issue を作る前に、その本文に対して `_lib/task-pr-handoff.md` の「Mention check」を実行する。実装でほかのずれが見つかったときも同じく扱う（設計書 3.5・4 章）。
-   [x] PR の説明に、設計書 7.4 の 4 つ（「決定」と「採らなかった案」の出典、移し元から除いた項目と移し先の項目の対応、`622db232` の後の照合の結果、作った issue）を挙げる。

#### ステップ 2-2: `project_overview.md` の A1 へのリンク

**変更するファイル:** `docs/dev/project_overview.md`

-   [x] 「LangChain 相当のフレームワークや…」の項目の末尾に、半角の空白 1 つと `[ADR-0019](adr/0019-pipeline-component-interfaces.md)` の形のリンクだけを足す（設計書 3.6）。項目の要約の文は変えない。A1 の ADR と同じコミットか、その後のコミットに入れる。

#### ステップ 2-3: 確かめの実行と記録

**変更するファイル:** 本書 7.2 節

-   [x] ステップ 1-3 と同じく、`-pr 2` で 2 つの確かめを実行して記録する。

**完了の条件（ステップ 2-1〜2-3）:** グリーンゲートが通る。`task_docs_unchanged.sh` と `adrchecks -pr 2` が 0 で終わり、② までの ADR と移しに `PENDING` がない。

### PR-2 作成ポイント: A-series ADRs

**対象ステップ**: 2-1 / 2-2 / 2-3

**推奨タイトル**: `docs(0065): パイプラインの構成要素、strictjson、LLMClient の契約などの A 系の ADR を加える`

**レビュー観点**: 各 ADR の「決定」が PR の基点のコードと正本の文書に合うこと / ADR-0001・0004 から除いた項目と移し先の項目の対応 / A8 の ADR と ADR-0018 の相互のリンクと、未実装の 0066 の扱い / A2 の食い違いの issue

**実装モデル要件**: standard

**判定理由**: 規約と設計書 3.1 の一覧に沿って既存の文書を写す作業で、どの判定の条件にも当たらない

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: B 系の ADR（設計書 8 章の ③）

#### ステップ 3-1: B 系の 5 本の ADR

**変更するファイル:** `docs/dev/adr/0026-extension-platform.md`〜`0030-minimum-chrome-version.md`（新規。1.5 の表の PR 3 の 5 本）、`docs/dev/adr/0005-server-architecture.md`、`0006-threat-model.md`、`0010-extension-stores-nothing.md`、`0011-webhook-publishing.md`、`docs/dev/adr/README.md`、本書（1.6・1.7・1.9 の表）

-   [ ] ステップ 2-1 の作業（索引と番号の確かめ、`622db232` の後の照合、ADR の書き方、移し、移さない重なりのリンク、issue、PR の説明）を、B3・B6・B9・B10・B12 の 5 本と、1.7 の表の PR 3 の行について行う。移さない重なりは、設計書 3.1 の表の B3・B6 の行である。B6 と B9 の ADR は 1 つのコミットに入れる（1.2）。
-   [ ] `threat-model` の `allowed_origins` の行の ③ の「書き換えた後の文面」を埋めるコミットで、1.7 の表の ④ の同じ行の「PR の基点の文面」を同じ文面に改める（1.7 の表の後の段落）。
-   [ ] B6 の ADR の「決定」は、拡張とホストとサーバがジョブの API だけを使うこと（ホストが `v` が `1` のメッセージだけを受け付けること）を述べ、旧 API を述べない。旧 API との併存、`v` の判別、0059 による削除は「経緯」に書く（AC-06）。0058 の照会の操作と 0059 の旧 API の削除を、B6 の ADR の「決定」と「経緯」に書く（設計書 3.4「F-002 と B6 の ADR の分担」）。
-   [ ] B12 の経緯は、設計書 1.2 のとおり 0007（102）・0020（103）・0046（103 を保った）・0026（103 を保った）・0050（110）を、出典の節を `main` に加えた順に並べる。
-   [ ] B10 の ADR の「決定」には、`README.md` の表示の言語の段落とコードのずれ（設計書 3.5）を書き、ステップ 2-1 と同じく issue を扱う。

#### ステップ 3-2: `project_overview.md` の B3・B6 へのリンク

**変更するファイル:** `docs/dev/project_overview.md`

-   [ ] 「言語」の項目の末尾に B3 の ADR へのリンクを、「実行形式」の項目の末尾に B6 の ADR へのリンクを、ステップ 2-2 と同じ形で足す。「実行形式」は、すでにある ADR-0005・ADR-0008 のリンクの後に足す。

#### ステップ 3-3: 確かめの実行と記録

**変更するファイル:** 本書 7.2 節

-   [ ] ステップ 1-3 と同じく、`-pr 3` で 2 つの確かめを実行して記録する。

**完了の条件（ステップ 3-1〜3-3）:** グリーンゲートが通る。`task_docs_unchanged.sh` と `adrchecks -pr 3` が 0 で終わり、③ までの ADR と移しに `PENDING` がない。1.7 の表の ④ の `threat-model` の行の「PR の基点の文面」が、head の ADR-0006 の行と一致する。

### PR-3 作成ポイント: B-series ADRs

**対象ステップ**: 3-1 / 3-2 / 3-3

**推奨タイトル**: `docs(0065): 拡張の基盤、ジョブの API、自動で送り直さない方針などの B 系の ADR を加える`

**レビュー観点**: B6 の ADR が旧 API を「決定」に述べず「経緯」に書くこと / ADR-0010・0011 から除いた項目と移し先の項目の対応（ADR-0010 の結果の行の B6 と B9 への分け方） / B12 の経緯の値の推移 / `project_overview.md` の 2 項目の文が変わっていないこと

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 3-1 で、ADR-0010 の「結果」の 1 行を B6 と B9 の 2 本へ分け、旧 API の併存と削除を 0044・0050・0058・0059 から「経緯」に組み立て、④ が引き継ぐ ADR-0006 の行を書き換える。分け方の判断と、同じコミットに収める制約を伴う、隔離すべき処理である

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: C 系と D 系の ADR（設計書 8 章の ④）

#### ステップ 4-1: C 系と D 系の 8 本の ADR

**変更するファイル:** `docs/dev/adr/0031-secret-closure.md`〜`0038-deadcode-and-race-free-timing.md`（新規。1.5 の表の PR 4 の 8 本）、`docs/dev/adr/0006-threat-model.md`、`0011-webhook-publishing.md`、`0012-lightweight-check-in-bash.md`、`0014-deterministic-checks-in-go.md`、`docs/dev/adr/README.md`、本書（1.6・1.7・1.9 の表）

-   [ ] ステップ 2-1 の作業を、C1〜C4・D5〜D8 の 8 本と、1.7 の表の PR 4 の行について行う。移さない重なりは、設計書 3.1 の表の D5・D7 の行である。
-   [ ] C1 の移しでは、ADR-0006 の `allowed_origins` の文と、ADR-0011 の `*url.Error` を直す文を移し元に残す（設計書 3.1）。両方の移し元から除くことと C1 の ADR を書くことを、同じコミットに入れる（設計書 5.2）。
-   [ ] D 系（D5・D7・D8）は、CLAUDE.md の規則の文を ADR に移さず、規則へリンクする（設計書 3.2）。CLAUDE.md は変えない（AC-07）。
-   [ ] C4 の ADR の「決定」には、`docs/dev/security.md` §6 の出典のリンクの記述とコードのずれ（設計書 3.5）を書き、ステップ 2-1 と同じく issue を扱う。

#### ステップ 4-2: 確かめの実行と記録

**変更するファイル:** 本書 7.2 節

-   [ ] ステップ 1-3 と同じく、`-pr 4` で 2 つの確かめを実行して記録する。

**完了の条件（ステップ 4-1・4-2）:** グリーンゲートが通る。`task_docs_unchanged.sh` と `adrchecks -pr 4` が 0 で終わり、`PENDING` がない。

### PR-4 作成ポイント: C- and D-series ADRs

**対象ステップ**: 4-1 / 4-2

**推奨タイトル**: `docs(0065): 秘密の持ち方、testdata のライセンス、言語の方針などの C 系と D 系の ADR を加える`

**レビュー観点**: C1 の移しで ADR-0006・0011 から秘密の扱いが抜け落ちず、残す文（`allowed_origins`、`*url.Error` の直し方）が残ること / D 系の ADR が CLAUDE.md の規則を写さずにリンクし、理由・採らなかった案・経緯を持つこと / C4 の食い違いの issue

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 4-1 で、セキュリティの決定（秘密の持ち方）を 2 本の ADR の文の中から分けて 1 つのコミットで移す。分け方の判断を伴い、誤ると決定がどの ADR からも抜け落ちる、隔離すべき処理である

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: 0058・0059 の反映 | ADR-0006・0008・0009・0010・0011 の経緯と「決定」、`_checks/` の 2 つの確かめ | ① がマージされた |
| M2: A 系 | A 系の 7 本、ADR-0001・0004 からの移し、ADR-0003・0005・0018 からのリンク、索引の 7 行、`project_overview.md` の 1 項目のリンク | ② がマージされた |
| M3: B 系 | B 系の 5 本、ADR-0010・0011 からの移し、ADR-0005・0006・0011 からのリンク、索引の 5 行、`project_overview.md` の 2 項目のリンク | ③ がマージされた |
| M4: C 系と D 系 | C 系と D 系の 8 本、ADR-0006・0011 からの移し、ADR-0012・0014 からのリンク、索引の 8 行 | ④ がマージされた |

① → ② → ③ → ④ の順にマージする（設計書 8 章）。番号は抜けなく続く必要があり、ADR を書く作業を並行して進めても、PR を開くのは前の PR のマージの後にする。

### 3.2. PR 構成

設計書 8 章の 4 つの PR に、フェーズを 1 つずつ当てる。ステップの並べ替えはない。PR の数は、設計書 1.2 で利用者と決めた分け方（H-04）なので変えない。② と ④ は 7 本と 8 本の ADR を持つが、ADR を 1 本ずつ索引の行と移しとともにコミットする（1.2）ので、レビュアーはコミットごとに読める。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 | ADR-0006・0008・0009・0010・0011 への 0058・0059 の反映、`_checks/` の 2 つの確かめ | frontier-recommended |
| PR-2 | 2-1 / 2-2 / 2-3 | A 系の 7 本、ADR-0001・0004 からの移し、ADR-0003・0005・0018 からのリンク、`project_overview.md` の 1 項目 | standard |
| PR-3 | 3-1 / 3-2 / 3-3 | B 系の 5 本、ADR-0010・0011 からの移し、ADR-0005・0006・0011 からのリンク、`project_overview.md` の 2 項目 | frontier-recommended |
| PR-4 | 4-1 / 4-2 | C 系と D 系の 8 本、ADR-0006・0011 からの移し、ADR-0012・0014 からのリンク | frontier-recommended |

## 4. テスト戦略 (Test Strategy)

設計書 7 章のとおりである。どの AC をどの確かめで確かめるかは 7 章に示す。

-   **単体テスト・結合テスト:** N/A。恒久的なテストは足さない（要件書 2.3、設計書 7.1・7.2）。
-   **実際の文書に対する確かめ:** 既存の `TestADRRecords` が ADR と索引を、既存の `TestInstructionDocPaths` がリンク先の実在を、`make test` と CI の `doc-guard` ジョブで確かめる。
-   **後方互換:** 既存のテストは変えない。`project_overview.md` を読む既存のテストが通ることを、各フェーズのグリーンゲートで確かめる。
-   **1 回だけの確かめ:** `task_docs_unchanged.sh`（AC-12）と `_checks/adrchecks`（AC-01〜AC-03・AC-05・AC-07〜AC-11・AC-13）を各 PR で実行し、7.2 節に記録する。
-   **壊して確かめる:** `_checks/` の各確かめは、対象を壊して `NG` になることを実装のときに確かめ、コミットメッセージに書く（ステップ 1-2）。
-   **読んで確かめる:** 文の内容を定める AC は、PR のレビューで読む。そのために、各 PR の説明に設計書 7.4 の 4 つを挙げる。
-   **カバレッジ:** 数値の目標は置かない。本タスクはテストを削除しないので、`make coverage-compare` は使わない。
-   **セキュリティ:** N/A（設計書 7.5）。

## 5. リスク管理 (Risk Management)

| リスク | 影響 | 対策 |
|---|---|---|
| ほかのタスクが本タスクの途中で ADR を作り、番号が埋まる | 予定の番号が使えない | 各フェーズの最初に索引を読み直し、次の空き番号を使い、1.5 の表を改める。`adrchecks` は `<kebab>` で ADR を探すので、番号が変わっても動く |
| 0066 が、本タスクの PR の間に ADR-0003・ADR-0018 や `project_overview.md` の「決定済みの方針」、`internal/config` の環境変数の読み取りを変える | A8 の ADR の「決定」と経緯がコードと食い違う。`overview-items` の目印や文の比較が合わなくなる | `main` を取り込んだら、設計書 6.1 の照合をやり直す。`overview-items` は PR ごとに基点と head を比べるので、ほかのタスクの変更は比較に入らない。目印が合わずに終了コード 2 になったら、目印を直す |
| 移し元から除いた決定が、移し先に書かれない | 決定がどの ADR にも書かれなくなる | 除くことと書くことを同じコミットに入れる（1.2）。`moved-items` で除いた行が一覧と一致することを確かめ、移し先が同じ内容を述べることを PR のレビューで読む（設計書 5.2） |
| ADR に出典にない採らなかった案や理由を書き足す | AC-04 を満たさない | PR の説明に出典を挙げ、レビュアーが出典と照合する（設計書 7.4） |
| issue の作成の承認が得られない | AC-03 を満たせない | PR を開かずに止まり、利用者に扱いを確かめ、決めた扱いを本書に記録する（設計書 4 章） |
| ③ と ④ が ADR-0006・0011 を共に変える | ③ の書き換えた行と ④ の移す行が重なる | 1.7 の表の後の段落のとおり、③ で書き換えた行について、④ の表の文面を改める |

## 6. 実装チェックリスト (Implementation Checklist)

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3）
- [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別は `test`（`make test` で実行するテスト）、`static`（`_checks/` の、PR ごとに 1 回実行するプログラムのうち head だけを読む確かめ）、`diff`（`_checks/` の、マージの基点と PR の head を比べる確かめ）、`manual`（PR のレビューで読む）である。`static` と `diff` の実行の記録は 7.2 節に書く。設計書の付録 B が `diff` とする AC-05・AC-08〜AC-11 の経緯のリンクは、head だけを読む確かめなので、本書では `static` とする（0060 の実装計画と同じ）。確かめそのものは設計書 7.3 の表のとおりである。

| AC | 種別 | 確かめ | ステップ |
|---|---|---|---|
| AC-01 | static・test・manual | static: `docs/tasks/0065_adr_remaining/_checks/adrchecks/main.go`（`history-sources`。`-pr 4` で 1.5 の表の 20 本がすべて head にある）。test: `cmd/yt2column/adr_test.go::TestADRRecords`（既存）。manual: 1.5 の表の主題と ADR が一対一で、各 ADR の主題が表の主題であることを読む | 1-2・2-1・3-1・4-1・4-2 |
| AC-02 | diff・static・manual | diff: `_checks/adrchecks`（`moved-items`：1.7 の表の行の文面が、基点と head の移し元に期待どおり残っているか、なくなっているかと、移し元から除かれた行がすべて表の行に当たること）。static: 同（`move-links`：`move` と `split` の行の移し元の「決定」に移し先の ADR へのリンクがある）。manual: 移し先が除いた項目と同じ内容を述べていることを、`moved-items` の並べた表示で読む。ほかの有効な ADR が新しい主題を自分の決定として述べていないことを読む | 1-2・2-1・3-1・4-1 |
| AC-03 | static・manual | static: `_checks/adrchecks`（`issue-links`：1.9 の issue の表の各 ADR の「決定」に、その issue へのリンクがある）。manual: PR の説明の「決定」の出典（コードの `file:line` と PR の基点のコミット ID）と照合し、「決定」がコードと食い違わないことと、issue の内容を読む（設計書 7.4） | 1-2・2-1・3-1・4-1 |
| AC-04 | manual | PR の説明の「採らなかった案」の出典と照合する | 2-1・3-1・4-1 |
| AC-05 | static・manual | static: `_checks/adrchecks`（`history-sources`：1.6 の表の各タスクについて、経緯に `docs/tasks/<番号>_*/` の文書へのリンクがある）。manual: 項目の要旨・日付・並びを読み、1.6 の表の「設計書との違い」の理由を読む | 1-2・2-1・3-1・4-1 |
| AC-06 | manual | ③ のレビューで、B6 の ADR の「決定」と「経緯」を読む | 3-1 |
| AC-07 | diff・static・manual | diff: `_checks/adrchecks`（`claude-md-unchanged`：CLAUDE.md が基点と head で同じ）。static: 同（`spec-links`：1.9 の正本の表の各 ADR の「決定」に、各文書へ解決される相対リンクがある）。manual: リンクが正本の節を指すことと、表・列挙・規則の一覧を書き写していないことを読む | 1-2・2-1・3-1・4-1 |
| AC-08 | static・manual | static: `_checks/adrchecks`（`history-sources` の `threat-model` の行）。manual: 項目が T-10 の再評価を述べていることを読む | 1-1・1-2 |
| AC-09 | static・manual | static: 同上（`extension-stores-nothing` の行）。manual: 項目が方針を保ったことを述べていることを読む | 1-1・1-2 |
| AC-10 | static・manual | static: 同上（`webhook-publishing` の行）。manual: 項目が失敗の通知の削除を述べ、「決定」が通知の経路を述べていないことを読む | 1-1・1-2 |
| AC-11 | static・manual | static: 同上（`threat-model`・`launch-path`・`viewer-opening`・`extension-stores-nothing`・`webhook-publishing` の行）。manual: 判断と理由が設計書 3.4 の表にあること、ADR-0008・0009・0010 の「決定」が PR の基点のコードと食い違わないことを読む | 1-1・1-2 |
| AC-12 | diff | `docs/tasks/0065_adr_remaining/_checks/task_docs_unchanged.sh`（各 PR で実行） | 1-2・1-3・2-3・3-3・4-2 |
| AC-13 | diff | `_checks/adrchecks`（`overview-items`：3 項目の文が、足したリンクを除いて基点と同じで、末尾のリンク先が基点のリンク先に設計書 3.6 の表の ADR を足したものと一致する）。リンクの実在は `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（既存） | 1-2・2-2・3-2・各 PR の確かめ |
| AC-14 | test | `cmd/yt2column/adr_test.go::TestADRRecords`（既存。番号の順、題名と状態の一致） | 2-1・3-1・4-1 |

### 7.1. `_checks/adrchecks` の確かめ

| 確かめ | 読むもの | 内容 |
|---|---|---|
| `history-sources` | head | 1.6 の表の各 `<kebab>` の ADR の経緯に、各タスクの `docs/tasks/<番号>_*/` の下の文書へ解決される相対リンクがある。新しい ADR がなければ、1.5 の表の PR で `PENDING` か `NG` を決める |
| `moved-items` | 基点と head | 1.7 の表の `-pr` の PR の行は、「PR の基点の文面」が基点の移し元にあって head の移し元になく、`split` と `link` の行は「書き換えた後の文面」が head の移し元にある。`-pr` より前の PR の行は、「PR の基点の文面」が head の移し元にない。後の PR の行は `PENDING`。あわせて、`-pr` の PR の行を持つ移し元の「背景」「決定」「採らなかった案」「結果」から基点と head の間に除かれた行が、すべてその PR の行の「PR の基点の文面」に当たる。`move` と `split` の行について、除いた文面と移し先の ADR の同じ節を並べて表示する |
| `move-links` | head | `-pr` 以前の PR の `move` と `split` の行の移し元ごとに、「決定」の節に移し先の ADR への相対リンクがある |
| `spec-links` | head | 1.9 の正本の表の各 ADR の「決定」の節に、各文書へ解決される相対リンクがある（アンカーは除く）。ADR の有無は `history-sources` と同じく扱う |
| `issue-links` | head | 1.9 の issue の表の各 ADR の「決定」の節に、その番号の issue（`https://github.com/isseis/yt2column/issues/<番号>`）へのリンクがある。`-pr` の PR 以前の行で番号が埋まっていなければ `NG` |
| `overview-items` | 基点と head | 1.8 の表の項目を、位置と目印で探す。文の比較とリンク先の比較は 0060 の `overview-items` と同じで、足すリンク先は 1.8 の表のとおり。後の PR の ADR は `PENDING` |
| `claude-md-unchanged` | 基点と head | CLAUDE.md が基点と head で同じ |

### 7.2. 実行の記録

（ステップ 1-3・2-3・3-3・4-2 で記入する。各 PR の `task_docs_unchanged.sh` と `_checks/adrchecks` の出力・終了コード・マージの基点と head のコミット ID、作った issue の URL を書く。）

#### PR-1

マージの基点 `bc4b71bd0efc69ebcc6c2d06dc8e87f6eabdf545`、head `e7a9df3f4941008d7390749959ad2a899204c75e`（`git fetch origin main` の後に実行した）。作った issue はない。

`docs/tasks/0065_adr_remaining/_checks/task_docs_unchanged.sh origin/main HEAD`：終了コード 0。

```text
merge-base: bc4b71bd0efc69ebcc6c2d06dc8e87f6eabdf545
head: e7a9df3f4941008d7390749959ad2a899204c75e
docs/tasks/: only docs/tasks/0065_adr_remaining/ changed
```

`go run -tags test ./docs/tasks/0065_adr_remaining/_checks/adrchecks/ -pr 1 bc4b71bd0efc69ebcc6c2d06dc8e87f6eabdf545 e7a9df3f4941008d7390749959ad2a899204c75e`：終了コード 0。`claude-md-unchanged` が `OK`、ほかの 6 つが `PENDING`。`history-sources` の既存の 5 本は `ok`、`overview-items` の文の比較は `OK` で、`PENDING` は後の PR の ADR と行だけである。

```text
PENDING history-sources
        ok: docs/dev/adr/0006-threat-model.md
        ok: docs/dev/adr/0008-launch-path.md
        ok: docs/dev/adr/0009-viewer-opening.md
        ok: docs/dev/adr/0010-extension-stores-nothing.md
        ok: docs/dev/adr/0011-webhook-publishing.md
        pending: pipeline-component-interfaces
        pending: strict-json
        pending: llm-client-contract
        pending: cost-estimate
        pending: artifact-resume
        pending: env-access-in-config
        pending: claude-output-limit
        pending: extension-platform
        pending: article-job-api
        pending: no-automatic-resend
        pending: extension-ui-languages
        pending: minimum-chrome-version
        pending: secret-closure
        pending: testdata-license
        pending: transcript-cache-slots
        pending: template-allowlist
        pending: language-policy
        pending: task-document-status-records
        pending: test-deletion-coverage
        pending: deadcode-and-race-free-timing
PENDING moved-items
        pending: pull request 2: llm-adapter-stdlib -> pipeline-component-interfaces (move)
        pending: pull request 2: llm-adapter-stdlib -> pipeline-component-interfaces (move)
        pending: pull request 2: llm-adapter-stdlib -> pipeline-component-interfaces (split)
        pending: pull request 2: llm-adapter-stdlib -> pipeline-component-interfaces (move)
        pending: pull request 2: llm-adapter-stdlib -> pipeline-component-interfaces (move)
        pending: pull request 2: llm-adapter-stdlib -> pipeline-component-interfaces (split)
        pending: pull request 2: llm-adapter-stdlib -> pipeline-component-interfaces (move)
        pending: pull request 2: llm-adapter-stdlib -> strict-json (split)
        pending: pull request 2: llm-adapter-stdlib -> strict-json (move)
        pending: pull request 2: llm-adapter-stdlib -> strict-json (move)
        pending: pull request 2: llm-adapter-stdlib -> strict-json (move)
        pending: pull request 2: server-architecture -> pipeline-component-interfaces (link)
        pending: pull request 2: llm-config-contract -> env-access-in-config (link)
        pending: pull request 2: server-config-file -> env-access-in-config (link)
        pending: pull request 2: production-profile -> claude-output-limit (move)
        pending: pull request 3: extension-stores-nothing -> article-job-api (move)
        pending: pull request 3: extension-stores-nothing -> article-job-api (move)
        pending: pull request 3: webhook-publishing -> no-automatic-resend (split)
        pending: pull request 3: webhook-publishing -> no-automatic-resend (move)
        pending: pull request 3: webhook-publishing -> no-automatic-resend (move)
        pending: pull request 3: webhook-publishing -> article-job-api (link)
        pending: pull request 3: server-architecture -> article-job-api (link)
        pending: pull request 3: server-architecture -> extension-platform (link)
        pending: pull request 3: threat-model -> extension-platform (link)
        pending: pull request 4: lightweight-check-in-bash -> language-policy (link)
        pending: pull request 4: deterministic-checks-in-go -> test-deletion-coverage (link)
        pending: pull request 4: threat-model -> secret-closure (split)
        pending: pull request 4: webhook-publishing -> secret-closure (split)
PENDING move-links
        pending: llm-adapter-stdlib -> pipeline-component-interfaces
        pending: llm-adapter-stdlib -> strict-json
        pending: production-profile -> claude-output-limit
        pending: extension-stores-nothing -> article-job-api
        pending: webhook-publishing -> no-automatic-resend
        pending: threat-model -> secret-closure
        pending: webhook-publishing -> secret-closure
PENDING spec-links
        pending: pipeline-component-interfaces
        pending: strict-json
        pending: llm-client-contract
        pending: cost-estimate
        pending: artifact-resume
        pending: claude-output-limit
        pending: extension-platform
        pending: article-job-api
        pending: no-automatic-resend
        pending: extension-ui-languages
        pending: minimum-chrome-version
        pending: secret-closure
        pending: testdata-license
        pending: transcript-cache-slots
        pending: template-allowlist
        pending: language-policy
        pending: task-document-status-records
        pending: test-deletion-coverage
        pending: deadcode-and-race-free-timing
PENDING issue-links
        pending: strict-json
        pending: extension-ui-languages
        pending: template-allowlist
PENDING overview-items
        text: OK
        links: PENDING
        links: bullet 1 (`docs/tasks/0007_browser_extension_skeleton/`) pending: extension-platform
        links: bullet 2 (`cmd/yt2column-nmhost`) pending: article-job-api
        links: bullet 6 (LangChain) pending: pipeline-component-interfaces
OK      claude-md-unchanged
```


#### PR-2

マージの基点 `a657854d60aafae593fd013cc0289fed3dc39475`、head `73c5ee4f2e87917c2f393577ab5f832d94435b09`（`git fetch origin main` の後に実行した）。作った issue は [#713](https://github.com/isseis/yt2column/issues/713)。

`docs/tasks/0065_adr_remaining/_checks/task_docs_unchanged.sh origin/main HEAD`：終了コード 0。

```text
merge-base: a657854d60aafae593fd013cc0289fed3dc39475
head: 73c5ee4f2e87917c2f393577ab5f832d94435b09
docs/tasks/: only docs/tasks/0065_adr_remaining/ changed
```

`go run -tags test ./docs/tasks/0065_adr_remaining/_checks/adrchecks/ -pr 2 a657854d60aafae593fd013cc0289fed3dc39475 73c5ee4f2e87917c2f393577ab5f832d94435b09`：終了コード 0。`claude-md-unchanged` が `OK`、7 本の新しい ADR と ② の移しはすべて `ok`。`PENDING` は後の PR の ADR・移し・リンクだけである。

```text
PENDING history-sources
        ok: docs/dev/adr/0006-threat-model.md
        ok: docs/dev/adr/0008-launch-path.md
        ok: docs/dev/adr/0009-viewer-opening.md
        ok: docs/dev/adr/0010-extension-stores-nothing.md
        ok: docs/dev/adr/0011-webhook-publishing.md
        ok: docs/dev/adr/0019-pipeline-component-interfaces.md
        ok: docs/dev/adr/0020-strict-json.md
        ok: docs/dev/adr/0021-llm-client-contract.md
        ok: docs/dev/adr/0022-cost-estimate.md
        ok: docs/dev/adr/0023-artifact-resume.md
        ok: docs/dev/adr/0024-env-access-in-config.md
        ok: docs/dev/adr/0025-claude-output-limit.md
        pending: extension-platform
        pending: article-job-api
        pending: no-automatic-resend
        pending: extension-ui-languages
        pending: minimum-chrome-version
        pending: secret-closure
        pending: testdata-license
        pending: transcript-cache-slots
        pending: template-allowlist
        pending: language-policy
        pending: task-document-status-records
        pending: test-deletion-coverage
        pending: deadcode-and-race-free-timing
PENDING moved-items
        ok: llm-adapter-stdlib -> pipeline-component-interfaces (move, pull request 2)
        ok: llm-adapter-stdlib -> pipeline-component-interfaces (move, pull request 2)
        ok: llm-adapter-stdlib -> pipeline-component-interfaces (split, pull request 2)
        ok: llm-adapter-stdlib -> pipeline-component-interfaces (move, pull request 2)
        ok: llm-adapter-stdlib -> pipeline-component-interfaces (move, pull request 2)
        ok: llm-adapter-stdlib -> pipeline-component-interfaces (split, pull request 2)
        ok: llm-adapter-stdlib -> pipeline-component-interfaces (move, pull request 2)
        ok: llm-adapter-stdlib -> strict-json (split, pull request 2)
        ok: llm-adapter-stdlib -> strict-json (move, pull request 2)
        ok: llm-adapter-stdlib -> strict-json (move, pull request 2)
        ok: llm-adapter-stdlib -> strict-json (move, pull request 2)
        ok: server-architecture -> pipeline-component-interfaces (link, pull request 2)
        ok: llm-config-contract -> env-access-in-config (link, pull request 2)
        ok: server-config-file -> env-access-in-config (link, pull request 2)
        ok: production-profile -> claude-output-limit (move, pull request 2)
        pending: pull request 3: extension-stores-nothing -> article-job-api (move)
        pending: pull request 3: extension-stores-nothing -> article-job-api (move)
        pending: pull request 3: webhook-publishing -> no-automatic-resend (split)
        pending: pull request 3: webhook-publishing -> no-automatic-resend (move)
        pending: pull request 3: webhook-publishing -> no-automatic-resend (move)
        pending: pull request 3: webhook-publishing -> article-job-api (link)
        pending: pull request 3: server-architecture -> article-job-api (link)
        pending: pull request 3: server-architecture -> extension-platform (link)
        pending: pull request 3: threat-model -> extension-platform (link)
        pending: pull request 4: lightweight-check-in-bash -> language-policy (link)
        pending: pull request 4: deterministic-checks-in-go -> test-deletion-coverage (link)
        pending: pull request 4: threat-model -> secret-closure (split)
        pending: pull request 4: webhook-publishing -> secret-closure (split)
        ---- llm-adapter-stdlib -> pipeline-component-interfaces (move), section 2
        removed: -   プロバイダに依存しない型と番兵: `GenerateRequest`・`GenerateResponse`・`LLMClient`、および不正なリクエスト・打ち切り・想定外の終了理由・空の応答。
        destination docs/dev/adr/0019-pipeline-component-interfaces.md, section 2:
          記事を作る処理は、字幕の取得・記事の作成・公開という段階を順に呼ぶ。段階の実装は、CLI と localhost サーバ、テストの fake で差し替える。段階の間で受け渡すデータ型と、失敗を呼び出し側が判別するための番兵も、段階ごとに決まる。
          実装を差し替えられるようにするには、呼び出す側が実装の型ではなく契約だけに依存する必要がある。契約を各段階のパッケージに置けば、実装を足してもパイプラインは変わらない。逆に、プロバイダに依存する型や番兵を共通の場所へ集めると、プロバイダを足すたびに共通の型を変えることになる。
          LLM の失敗は、どのプロバイダでも起こるもの（不正なリクエスト・打ち切り・想定外の終了理由・空の応答）と、プロバイダの API に固有のもの（HTTP の状態・応答本文の形・通信の失敗）に分かれる。前者を共通の型に、後者を各アダプタに置くと、呼び出し側は共通の番兵だけで打ち切りなどを判別できる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) §1.1）。Claude のアダプタも同じ方針を保った（[0009 設計書](../../tasks/0009_claude_llm_client/02_architecture.md) §1.1）。
        ---- llm-adapter-stdlib -> pipeline-component-interfaces (move), section 3
        removed: -   **プロバイダに依存しない型と番兵は `internal/llm` に置く。** `LLMClient` と `GenerateRequest`・`GenerateResponse`、`ErrInvalidRequest`・`ErrTruncated`・`ErrUnexpectedFinishReason`・`ErrEmptyResponse`、リクエストの検証はここにある。
        destination docs/dev/adr/0019-pipeline-component-interfaces.md, section 3:
          -   **段階と `LLMClient` は、それぞれのパッケージが interface と入出力のデータ型を持つ。** `TranscriptSource` は `internal/transcript`、`ArticleWriter` は `internal/writer`、`Publisher` は `internal/publisher`、`LLMClient` は `internal/llm` が持つ。`internal/pipeline` は interface だけに依存し、実装の詳細を知らない（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §1.1・§3.2）。
          -   **段階の間で受け渡すデータ型が契約である。** 段階は値を受け渡し、空の結果を正常として返さないことを各 interface の英語のドキュメントコメントに書く。すべてのメソッドは第 1 引数に `context.Context` を受け取り、キャンセルとタイムアウトに従う。
          -   **プロバイダに依存しない番兵とリクエストの検証は `internal/llm` に置く。** `ErrInvalidRequest`・`ErrTruncated`・`ErrUnexpectedFinishReason`・`ErrEmptyResponse` と `GenerateRequest.Validate` がここにある。
          -   **プロバイダに固有の番兵は各アダプタに置く。** HTTP の状態・応答本文の形・通信の失敗は、プロバイダの名前を持つので `internal/llm/deepseek` と `internal/llm/claude` がそれぞれ持つ。
          LLM のアダプタの構成と送信の経路、プロバイダ固有のパラメータの型は [ADR-0001](0001-llm-adapter-stdlib.md) が、サーバの構成は [ADR-0005](0005-server-architecture.md) が定める。
          この決定の仕様の正本は [CLAUDE.md](../../../CLAUDE.md) の「Key Design Patterns」の「Separation of Concerns」と「Interface-based Design」、[project_overview.md](../project_overview.md) の「パイプライン」、[package_reference.md](../conventions/package_reference.md) のパッケージの表である。
        ---- llm-adapter-stdlib -> pipeline-component-interfaces (split), section 3
        removed: -   **HTTP の送信の経路は `internal/llm/llmhttp` に置き、両方のアダプタが使う。** 期限の適用、リダイレクトの禁止、応答本文の上限、失敗の分類をここで行う。`llmhttp` はプロバイダ固有の番兵を持たず、アダプタが渡した番兵で失敗を報告する。HTTP の状態・応答本文の形・通信の失敗の番兵は、プロバイダの名前を持つので各アダプタに残す。
        kept:    -   **HTTP の送信の経路は `internal/llm/llmhttp` に置き、両方のアダプタが使う。** 期限の適用、リダイレクトの禁止、応答本文の上限、失敗の分類をここで行う。`llmhttp` はプロバイダ固有の番兵を持たず、アダプタが渡した番兵で失敗を報告する。番兵の置き場所は [ADR-0019](0019-pipeline-component-interfaces.md) が定める。
        destination docs/dev/adr/0019-pipeline-component-interfaces.md, section 3:
          -   **段階と `LLMClient` は、それぞれのパッケージが interface と入出力のデータ型を持つ。** `TranscriptSource` は `internal/transcript`、`ArticleWriter` は `internal/writer`、`Publisher` は `internal/publisher`、`LLMClient` は `internal/llm` が持つ。`internal/pipeline` は interface だけに依存し、実装の詳細を知らない（[0001 設計書](../../tasks/0001_pipeline_skeleton/02_architecture.md) §1.1・§3.2）。
          -   **段階の間で受け渡すデータ型が契約である。** 段階は値を受け渡し、空の結果を正常として返さないことを各 interface の英語のドキュメントコメントに書く。すべてのメソッドは第 1 引数に `context.Context` を受け取り、キャンセルとタイムアウトに従う。
          -   **プロバイダに依存しない番兵とリクエストの検証は `internal/llm` に置く。** `ErrInvalidRequest`・`ErrTruncated`・`ErrUnexpectedFinishReason`・`ErrEmptyResponse` と `GenerateRequest.Validate` がここにある。
          -   **プロバイダに固有の番兵は各アダプタに置く。** HTTP の状態・応答本文の形・通信の失敗は、プロバイダの名前を持つので `internal/llm/deepseek` と `internal/llm/claude` がそれぞれ持つ。
          LLM のアダプタの構成と送信の経路、プロバイダ固有のパラメータの型は [ADR-0001](0001-llm-adapter-stdlib.md) が、サーバの構成は [ADR-0005](0005-server-architecture.md) が定める。
          この決定の仕様の正本は [CLAUDE.md](../../../CLAUDE.md) の「Key Design Patterns」の「Separation of Concerns」と「Interface-based Design」、[project_overview.md](../project_overview.md) の「パイプライン」、[package_reference.md](../conventions/package_reference.md) のパッケージの表である。
        ---- llm-adapter-stdlib -> pipeline-component-interfaces (move), section 4
        removed: -   **番兵をすべて各アダプタに置く。** `ArticleWriter` が打ち切りを判別するために、プロバイダのパッケージを import することになる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
        destination docs/dev/adr/0019-pipeline-component-interfaces.md, section 4:
          -   **番兵をすべて各アダプタに置く。** `ArticleWriter` が打ち切りを判別するために、プロバイダのパッケージを import することになる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
          -   **番兵をすべて `internal/llm` に置く。** SDK を使うプロバイダが、HTTP ステータスや応答本文の形に固有の番兵を `internal/llm` に合わせる必要が生じる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
        ---- llm-adapter-stdlib -> pipeline-component-interfaces (move), section 4
        removed: -   **番兵をすべて `internal/llm` に置く。** SDK を使うプロバイダが、HTTP ステータスや応答本文の形に固有の番兵を `internal/llm` に合わせる必要が生じる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
        destination docs/dev/adr/0019-pipeline-component-interfaces.md, section 4:
          -   **番兵をすべて各アダプタに置く。** `ArticleWriter` が打ち切りを判別するために、プロバイダのパッケージを import することになる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
          -   **番兵をすべて `internal/llm` に置く。** SDK を使うプロバイダが、HTTP ステータスや応答本文の形に固有の番兵を `internal/llm` に合わせる必要が生じる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
        ---- llm-adapter-stdlib -> pipeline-component-interfaces (split), section 5
        removed: -   プロバイダを足すときは、アダプタのパッケージと、そのプロバイダのパラメータの型のパッケージを足す。`internal/llm` の型と `llmhttp` は変えない。
        kept:    -   プロバイダを足すときは、アダプタのパッケージと、そのプロバイダのパラメータの型のパッケージを足す。`llmhttp` は変えない。プロバイダに依存しない型は [ADR-0019](0019-pipeline-component-interfaces.md) が定める。
        destination docs/dev/adr/0019-pipeline-component-interfaces.md, section 5:
          -   パイプラインは段階の interface だけに依存するので、実装を fake に差し替えてテストでき、段階を足してもパイプラインの呼び出し以外は変わらない。
          -   呼び出し側は、プロバイダに依存しない 4 つの番兵だけで、不正なリクエスト・打ち切り・想定外の終了理由・空の応答を判別できる。プロバイダ固有の失敗は、そのアダプタを選んだ呼び出し側だけが扱う。
          -   プロバイダを足すときは、アダプタのパッケージと、そのプロバイダのパラメータの型のパッケージを足す。`llmhttp` は変えない。プロバイダに依存しない型は変えない。
          -   LLM のアダプタの構成と送信の経路は [ADR-0001](0001-llm-adapter-stdlib.md) に残る。この ADR は、段階の構成要素と番兵の置き場所だけを定める。
        ---- llm-adapter-stdlib -> pipeline-component-interfaces (move), section 5
        removed: -   プロバイダ固有の失敗（HTTP ステータス・応答本文の形・通信の失敗）は、プロバイダごとの番兵で報告される。呼び出し側は、プロバイダに依存しない 4 つの番兵だけで、打ち切り・想定外の終了理由・空の応答・不正なリクエストを判別できる。
        destination docs/dev/adr/0019-pipeline-component-interfaces.md, section 5:
          -   パイプラインは段階の interface だけに依存するので、実装を fake に差し替えてテストでき、段階を足してもパイプラインの呼び出し以外は変わらない。
          -   呼び出し側は、プロバイダに依存しない 4 つの番兵だけで、不正なリクエスト・打ち切り・想定外の終了理由・空の応答を判別できる。プロバイダ固有の失敗は、そのアダプタを選んだ呼び出し側だけが扱う。
          -   プロバイダを足すときは、アダプタのパッケージと、そのプロバイダのパラメータの型のパッケージを足す。`llmhttp` は変えない。プロバイダに依存しない型は変えない。
          -   LLM のアダプタの構成と送信の経路は [ADR-0001](0001-llm-adapter-stdlib.md) に残る。この ADR は、段階の構成要素と番兵の置き場所だけを定める。
        ---- llm-adapter-stdlib -> strict-json (split), section 2
        removed: -   応答の厳格な検証: 標準ライブラリのデコーダが黙って補正する入力を拒否する部品。
        kept:    -   応答の厳格な検証: 標準ライブラリのデコーダが黙って補正する入力を拒否する部品（[ADR-0020](0020-strict-json.md) が定める）。
        destination docs/dev/adr/0020-strict-json.md, section 2:
          LLM の API の応答、字幕の json3 と info.json、中間生成物、サーバとホストのメッセージなど、外部から受け取る JSON を読む箇所が複数ある。標準ライブラリの `encoding/json` は、不正な UTF-8、対になっていないサロゲートのエスケープ、後続のデータ、重複したメンバー、`null` を黙って補正することがある。補正された値をそのまま使うと、検証した値と実際に受理される値がずれる。
          最初はこの部品が字幕のパーサの中にだけあった。二つ目の利用者として LLM のアダプタが加わり、部品を `internal/transcript` に残すと LLM の段階が字幕取得の段階に依存し、複製するとサロゲートの走査などの細かいコードが 2 か所になる。検証済みの値だけを扱えることを型で保証すれば、公開範囲を広げても不変条件を保てる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) §3.5）。
          利用者が増えるにつれて、宣言したキー以外を拒否する読み取り、真偽値、`null` の判定、元のバイト列、負でない整数の読み取りも必要になった。負でない整数の検査を各利用者が書くと、`-0` や小数の扱いが箇所ごとにずれる。
        ---- llm-adapter-stdlib -> strict-json (move), section 3
        removed: -   **応答の厳格な検証は `internal/strictjson` に置き、複数のパッケージで共有する。** 字幕のパーサ、両方のアダプタ、`ArticleWriter`、`internal/publisher`、`internal/artifact`、`internal/fixprguard`、`internal/server`、`internal/nmhost`、`internal/codexreview`、`internal/coveragecompare` が使う。
        destination docs/dev/adr/0020-strict-json.md, section 3:
          -   **検証済みの JSON から値を取り出す部品は `internal/strictjson` にだけ置き、複数のパッケージで共有する。** `ParseObject` は、不正な UTF-8、対になっていないサロゲートのエスケープ、後続のデータ、オブジェクトでない入力を拒否する。
          -   **`Object` と `Value` は不透明な型にし、`ParseObject` から得た値だけを扱えるようにする。** フィールドは非公開で、`AsString`・`AsObject`・`AsArray` などは、検証を通っていないバイト列を受け取れない。ゼロ値の `Value` は中身を持たず、`AsString` などの読み取りはエラーを返す。ゼロ値の `Object` はメンバーを持たない。
          -   **読み取りの手段を、利用者の必要に合わせて 1 か所に持つ。** `Object.Collect` は消費しないキーを無視し、`Object.CollectOnly` は宣言していないキーと重複したキーを拒否する。`Object.Has` はエスケープを解いた名前の完全一致で有無を返し、振る舞いの選択に使える。`Value.AsBool` は `true` と `false` だけを受理する。`Value.IsNull` は `null` とそれ以外を区別する。`Value.Raw` は値の元のバイト列を返す。
          -   **負でない整数の規則は `Value.AsNonNegativeInt64` の 1 か所にする。** 符号・小数部・指数部のない 10 進の整数で `int64` の範囲にあり、負でないものだけを受理する。`-0` も拒否し、拒否のエラーには入力の値を含めない。字幕の `tStartMs`、トークン数、中間生成物の数値、ホストのメッセージの版がこれを使う。`AsInt64` は符号と `-0` を受理するので、負でない整数が要る箇所では使わない。
          -   **この部品を使うパッケージの一覧は [package_reference.md](../conventions/package_reference.md) が持つ。** 現在のコードでは、一覧に挙げられたパッケージに加え、`internal/server`・`internal/nmhost`・`internal/codexreview`・`internal/coveragecompare` もこの部品を使う。一覧にこの 4 つがないことは、記録した issue [#713](https://github.com/isseis/yt2column/issues/713) で扱う。
          -   **字幕の json3 と info.json の読み取りは、この部品を使う。** 部品を移すことで `internal/transcript` の返す番兵とその条件は変えない（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) §3.5）。
          この決定の仕様の正本は [package_reference.md](../conventions/package_reference.md) の `internal/strictjson` の行である。部品の置き場所と、LLM のアダプタとの関係は [ADR-0001](0001-llm-adapter-stdlib.md) が定める。
        ---- llm-adapter-stdlib -> strict-json (move), section 4
        removed: -   **厳格な JSON の部品を `internal/llm/deepseek` に複製する。** サロゲートの走査などの細かいコードが 2 か所になる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
        destination docs/dev/adr/0020-strict-json.md, section 4:
          -   **厳格な JSON の部品を `internal/llm/deepseek` に複製する。** サロゲートの走査などの細かいコードが 2 か所になる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
          -   **`internal/transcript` の非公開の関数を公開して LLM の段階から使う。** LLM の段階が字幕取得の段階に依存する（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
          -   **宣言していないキーの拒否を、メンバーの数と消費したキーの数の比較で行う。** `Object` のメンバーが非公開で数を取れず、数を公開すると呼び出し元ごとに比べる処理を書くことになり、比べ忘れる余地が残る（[0010 設計書](../../tasks/0010_post_generation_review/02_architecture.md) §3.5.3）。
        ---- llm-adapter-stdlib -> strict-json (move), section 4
        removed: -   **`internal/transcript` の非公開の関数を公開して LLM の段階から使う。** LLM の段階が字幕取得の段階に依存する（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
        destination docs/dev/adr/0020-strict-json.md, section 4:
          -   **厳格な JSON の部品を `internal/llm/deepseek` に複製する。** サロゲートの走査などの細かいコードが 2 か所になる（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
          -   **`internal/transcript` の非公開の関数を公開して LLM の段階から使う。** LLM の段階が字幕取得の段階に依存する（[0003 設計書](../../tasks/0003_deepseek_llm_client/02_architecture.md) 付録 A）。
          -   **宣言していないキーの拒否を、メンバーの数と消費したキーの数の比較で行う。** `Object` のメンバーが非公開で数を取れず、数を公開すると呼び出し元ごとに比べる処理を書くことになり、比べ忘れる余地が残る（[0010 設計書](../../tasks/0010_post_generation_review/02_architecture.md) §3.5.3）。
        ---- production-profile -> claude-output-limit (move), section 3
        removed: -   **Claude アダプタの出力の上限の既定は 40,000 トークンとし、3 つの段階で共有する。** 本番の構成で長い動画の出力が打ち切られないようにするためである。
        destination docs/dev/adr/0025-claude-output-limit.md, section 3:
          -   **Claude アダプタの `max_tokens` の既定は 40,000 とする。** リクエストの `MaxOutputTokens` が 0 のときだけ既定を使い、正の値が指定されたときはその値を使う。3 つの段階は同じ既定を共有する。
          -   **ストリーミングを使わない。** 1 回の HTTP リクエストで応答を受け取り、送信から本文の読み取りまでを 1 本の `context` の期限で覆う。
          -   **打ち切りは `ErrTruncated` として報告する。** メッセージは、そのリクエストが送った `max_tokens` を示す。テキストがないまま打ち切られた場合は、effort を下げるよう案内する。
          -   **上限の値はコードの 1 つの定数に置き、値の根拠と測り直しの手順は [claude_output_limit.md](../guides/claude_output_limit.md) に置く。** README と上限のガイドは数値を書かず、本番の構成の文書は本番の上限として 40,000 を記す。値が満たす条件はテストで固定する。
          -   **非ストリーミングの長い無通信で接続が切れるリスクは受け入れる。** HTTP/2 の PING などの仕組みは、無通信による切断が実際に観測されるまで足さない。
          -   **既定の値でも、`max` の effort では推論だけで上限に達しうる。** その場合は打ち切りとして報告し、記事を公開しない。README で案内する。
          この決定の仕様の正本は [claude_output_limit.md](../guides/claude_output_limit.md) と [production_profile.md](../production_profile.md) である。値の文法と effort の既定は [ADR-0003](0003-llm-config-contract.md) が定める。
PENDING move-links
        ok: docs/dev/adr/0001-llm-adapter-stdlib.md -> docs/dev/adr/0019-pipeline-component-interfaces.md
        ok: docs/dev/adr/0001-llm-adapter-stdlib.md -> docs/dev/adr/0020-strict-json.md
        ok: docs/dev/adr/0004-production-profile.md -> docs/dev/adr/0025-claude-output-limit.md
        pending: extension-stores-nothing -> article-job-api
        pending: webhook-publishing -> no-automatic-resend
        pending: threat-model -> secret-closure
        pending: webhook-publishing -> secret-closure
PENDING spec-links
        ok: docs/dev/adr/0019-pipeline-component-interfaces.md
        ok: docs/dev/adr/0020-strict-json.md
        ok: docs/dev/adr/0021-llm-client-contract.md
        ok: docs/dev/adr/0022-cost-estimate.md
        ok: docs/dev/adr/0023-artifact-resume.md
        ok: docs/dev/adr/0025-claude-output-limit.md
        pending: extension-platform
        pending: article-job-api
        pending: no-automatic-resend
        pending: extension-ui-languages
        pending: minimum-chrome-version
        pending: secret-closure
        pending: testdata-license
        pending: transcript-cache-slots
        pending: template-allowlist
        pending: language-policy
        pending: task-document-status-records
        pending: test-deletion-coverage
        pending: deadcode-and-race-free-timing
PENDING issue-links
        ok: docs/dev/adr/0020-strict-json.md
        pending: extension-ui-languages
        pending: template-allowlist
PENDING overview-items
        text: OK
        links: PENDING
        links: bullet 1 (`docs/tasks/0007_browser_extension_skeleton/`) pending: extension-platform
        links: bullet 2 (`cmd/yt2column-nmhost`) pending: article-job-api
OK      claude-md-unchanged
```

## 8. 成功基準 (Success Criteria)

-   **機能の完成度:** 7 章のすべての行の確かめが済み、7.2 節に 4 つの PR の確かめの記録がある。`adrchecks -pr 4` が `PENDING` を表示しない。
-   **品質:** 各 PR でグリーンゲート（`make test && make lint`）が通る。① で `go vet -tags test` が `_checks/adrchecks` に通る。`_checks/` の確かめは、対象を壊して `NG` になることをコミットメッセージに記録している。
-   **セキュリティ:** N/A（要件書 4.2）。
-   **文書:** 20 本の ADR と索引の行があり、移し元の ADR は移した位置から移し先へリンクし、`project_overview.md` の 3 項目が ADR へリンクしている。

## 9. 次のステップ (Next Steps)

-   ④ のマージの後、`/runpr` が issue #666 を閉じる案内をする（本タスクの最後の PR のため）。
-   設計書 9 章の将来の拡張（A8 と C1 の正本の文書の欠け、本タスクより前からある ADR 同士の重なり）は、その主題を変える後のタスクに任せる。

## 変更履歴

- 2026-11-06 承認（isseis、#708）
