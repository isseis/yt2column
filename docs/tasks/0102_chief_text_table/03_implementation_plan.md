# 実装計画書：chief の状況の表を道具が等幅のテキストで組み、workspace ごとに 2 行で示す

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-18 |
| Review date | 2026-11-19 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。

既存のファイルの内容と `file:line` の行番号は、コミット `790652e3` で確かめた。設計書が確かめたコミット `0bbba2c8` の後に、タスク 0092 の PR #986 と 0103 の PR #992 が `internal/chief`・`.claude/commands/chief.md`・`CLAUDE.md` を変えたので、設計書が引く行番号のうち次のものはずれている。本計画は今の行番号で書く。

-   CLAUDE.md の「Every other file of the tool chooses behavior only by the types the screen readers return」は `CLAUDE.md:518-519`（設計書 3.11 の `:523-524`）。「Go Idioms」の最初の例外の項は `CLAUDE.md:872`、画面の読み取りの例外の項は `:909-918`、タスクの読み取りの例外の項は `:919-933`。
-   `/chief` の定義の手順 2 の項目 2 は `.claude/commands/chief.md:131-203`（設計書 1.3 の `:131-202`）。

用語は、要件書 3 章・6 章と設計書の冒頭と同じ意味で使う（表・表示幅・使える幅・状態の文字列・タブの表示・凡例・表の材料・表示の単位・語 など）。本計画は、次の用語を加える。

-   **実物の確かめ:** 本物の Herdr と Claude Code で、人が画面を見て行う確かめ。PR-1 の最初（設計書 1.2・8 章の 1）と PR-3 の最後（設計書 7.3）に行い、結果を §7 の実行の記録に書く。
-   **幅の表:** East Asian Width が W か F の符号位置の範囲を並べた、`textwidth.go` の Go の表（設計書 3.5）。
-   **生成の道具:** `EastAsianWidth.txt` から幅の表を作る、`//go:build ignore` の Go のプログラム（§1.3 の決定 4）。道具 `chief` には入らない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、道具 `chief` が状況の表を等幅のテキストとして組んで `snapshot` と `tick` の出力に含め、`/chief` の定義はそれを code block にそのまま貼り、chief 利用ガイドと CLAUDE.md をそれに合わせて、要件書の F-001〜F-006 を満たす。

### 1.2. 実装の原則

-   **出力の形を変える配線は最後の PR に置く。** 入力の部品（PR-1）→ 表を組む部品と CLAUDE.md の例外（PR-2）→ 配線・出力・`Protocol`・文書（PR-3）の順に PR を分ける（設計書 8 章、§1.3 の決定 1）。PR-1 と PR-2 は、道具の出力も、ラウンドが走らせるコマンドも変えない。
-   **表は `rows` と同じ `Row` から組み、`rows` は変えない。** 表のために値を決め直さない（設計書 1.1・3.1）。
-   **表の材料は表示にだけ使い、読めなくてもラウンドを失敗させない。** 読めなければ 80 桁と pane の ID で組む（設計書 1.1・4 章）。
-   **外部のコマンドは差し込む。** テストは herdr・git・gh を本物で呼ばない。Herdr の答えは記録係（`internal/chief/test_helpers.go:217-221` の `readOnlyCommands` と `recorder`）とテストの world（`internal/chief/world_test.go:101`）が返す。
-   **各テストは壊して失敗を確かめる。** 確かめた結果をコミットのメッセージに書く（CLAUDE.md の「Testing Strategy」）。

### 1.3. 計画の際に決めたこと（2026-11-18）

1.  **PR を 3 つに分ける（人が選んだ）。** PR-1 は、実物の確かめ、Herdr のサンプル、`Sanitize` の表示の単位、`CapHead` の上限の引数、Herdr の 3 つの読み取り（設計書 8 章の 1、2 の一部、3 の一部）。PR-2 は、表示幅（`textwidth.go`）、表を組む処理（`table.go`）、CLAUDE.md の 2 つの例外（8 章の 2 の残り、4 の一部、5 の CLAUDE.md）。PR-3 は、ラウンドへの配線、出力の `table`・`table_unreadable`、`Protocol`、`/chief` の定義、ガイド、テストの削除、実物の確かめ（8 章の 3 の残り、4 の残り、5 の残り、6）。`textwidth.go` を `table.go` と同じ PR-2 に置くのは、CLAUDE.md の「Declare, don't infer」の例外（AC-28）がこの 2 つのファイルを名指すので、例外の文とファイルを同じ PR で入れるためである。CLAUDE.md は、設計書 8 章の 5 では `Protocol` を上げる PR に置いていたが、PR-2 に置くことにした。人が選んだ（2026-11-19）ので、本計画を入れる PR で設計書 8 章の 5 を改め、設計書を `draft` に戻した。日本語の literal と、値の中身で振る舞いを選ぶ処理が、CLAUDE.md の例外より先に `main` に入らないようにするためである。PR-3 は 8 つのステップを持つが、`Protocol` を上げる変更と、道具が表を出力する変更と、定義が表を示す変更は、設計書 8 章の 5 のとおり同じ番号で入る必要があるので、さらには分けない。そのかわり、ラウンドの振る舞いを変えるステップ 3-1 を PR-3 の最初のコミットにし、テストの削除・定義・ガイドをそれぞれ別のコミットにして、レビューで振る舞いの変更を文書の差分と分けて読めるようにする。PR-2 を PR-3 にまとめる案は PR-3 がさらに大きくなり、CLAUDE.md を PR-3 に残す案は PR-2 のマージの後に CLAUDE.md の規則に反する状態が `main` に残るので採らない。2 つの PR にまとめる案は 1 つの PR のレビューが大きくなり、4 つに分ける案は PR の手間が増えるので採らない。ADR: なし（PR の切り方はこのタスクの中で閉じるため）。
2.  **設計書 1.2 の実物の確かめ（code block の右端で折り返す桁と、タブを分割・拡大したときの `rect.width`）は、PR-1 の `/runplan` のセッションの最初に、人と一緒に行う（人が選んだ）。** エージェントが試しの行を用意して示し、人が画面を見て答える（ステップ 1-1）。確かめが成り立たなければ、コードを書く前に止まる。(1) が成り立たないときは、設計書 1.2 のとおり、要件書 AC-04 の「左の余白」の言葉を直す判断の変更として人と決め直す（requirements_process.md の「Editing an approved document」）。(2) が成り立たないときは、幅の読み方を決め直すか既知の制限として受け入れるかを人に尋ねる。セッションの中で行うので、PR-1 の自動送信は `auto` のままにする。人が `/runplan` の前に済ませる案は自動送信を `hold` にする手間がかかり、PR-3 の実物の確かめにまとめる案は、余白が違ったときに表を組む処理を作り直すことになるので採らない。ADR: なし。
3.  **幅の表は Unicode 17.0.0 の `EastAsianWidth.txt` から作る（人が選んだ）。** 作業環境の Go 1.27.1 の `unicode.Version` と同じで、最近の端末の表に近い。`go.mod` の Go 1.26.5 の `unicode.Version` は 15.0.0 で、`Sanitize` の書式文字の判定（`internal/chief/sanitize.go:98`、`unicode.Is(unicode.Cf, r)`）はこれによる。15.0 の後に書式文字になった符号位置は、Go の版によってエスケープにも 1 桁の文字にもなるが、設計書 1.2 のとおりまれなので受け入れる。15.0.0 にする案は、15.0 の後に足された絵文字を 1 桁と数えるので採らない。ADR: なし（表示の細部のため）。
4.  **幅の表は生成の道具で作り、元のファイルは置かず、版と SHA-256 を表の注釈に書く（人が選んだ）。** 生成の道具は `internal/chief/textwidth_gen.go`（`//go:build ignore`、`package main`）に置き、`go run` に `EastAsianWidth.txt` のパスを渡す。道具は、`textwidth.go` の中の 2 つの印の注釈の間の行だけを書き換え、手で書いた関数には触れない。印の形は実装で決める。生成の道具は `make test`・`make lint` が通らないので、PR-2 の完了の条件に、道具を `go run` で走らせ直して `textwidth.go` に差分がないことを入れる。生成の道具は、Unicode が配る決まった形のデータのファイルを、ビルドの前に Go の表へ写すもので、道具 `chief` の呼び出し元から来る値を読まないので、CLAUDE.md の「Declare, don't infer」の例外に含めない。元のファイルの版、取得元の URL、SHA-256 を、表の注釈と §7 の「PR-2 の実行の記録」に書く。元のファイルを取りに行くのはネットワークの操作なので、`/runplan` は取りに行く前に人の承認を得る（CLAUDE.md の「Tool Execution Safety」）。`EastAsianWidth.txt` を `testdata/` に置き、テストで表を作り直して比べる案は、第三者のデータのファイル（約 100 KB）とその利用許諾の扱い（[ADR-0032](../../dev/adr/0032-testdata-license.md)）を増やすので採らない。走らせ直して差分がないことは、生成が決まった結果を返すことしか示さない。そのため、表が元のファイルと一致することは、生成の道具によらずに確かめる。`TestUnitWidth` の期待値は、元のファイルの行から人手で選んだ境目の符号位置にし、引いた行を §7 に書く。さらに、元のファイルの W・F の符号位置の数を `awk` で数え、表の範囲の大きさの合計と比べた結果を §7 に書く。ADR: なし（表の作り方の細部のため）。
5.  **AC-03 の「手順 2 の差分は項目 2 と `--protocol` の値だけ」は、`_checks/` のシェルのスクリプトで確かめる。** `/chief` の定義の差分は、項目 2 の全文を置き換え、`--protocol` の値をファイル全体で直すので小さくない。記録した diff を読む方法は採らない（requirements_process.md §4 の「`diff` verification」）。スクリプトは、マージの基点と PR のコミットの手順 2 の節を比べる。項目 2 の本文と `--protocol` の値のほかに違いがないことを確かめ、項目 2 が変わっていることも確かめる（何も確かめずに通らないため）。ADR: なし。
6.  **`TestDisplayValuesMatchTool` の削除は、`/chief` の定義から値の表を除くコミットより前に、単独のコミットで行う。** 定義から値の表を除くと、このテストが落ちるためである（`cmd/chief/docs_test.go:469-503`）。削除のコミットは、CLAUDE.md の「Deleting a test is a claim that must be checked」のとおり `make coverage-compare BASE=<親のコミット>` で確かめる。削除の後も `OutputValueNames` は `TestOutputValueNames`（`internal/chief/output_test.go:21`）が呼ぶので、カバレッジは下がらない見込みだが、見込みでなく比較の出力で確かめる。`OutputValueNames` を除き、`TestOutputValueNames` を直すことは、その後の別のコミットにする（設計書 8 章の 5）。ADR: なし。
7.  **パッケージの一覧（`docs/dev/conventions/package_reference.md:122`）から `OutputValueNames` を除く。** 設計書 2.2 の表にない文書だが、`OutputValueNames` を除く（設計書 3.12）と、一覧の名前が残る。設計の変更ではなく、除く名前の後始末なので、設計書は直さない。`CapHead` は引数が変わるだけで名前は残るので、一覧はそのままにする。ADR: なし。
8.  **Herdr のサンプルは、PR-1 の `/runplan` のセッションが、読み取りのコマンドだけで採る。** `herdr tab list` と `herdr pane layout` は `.claude/settings.json` の拒否（`.claude/settings.json:11-34`）に当たらない読み取りである。タブを分割した workspace で `herdr pane layout` を採り、chief の pane でない項目を含むサンプルにする。採った人を特定できる値は、`internal/chief/testdata/README.md` の「置き換えたもの」の規則で置き換える。ADR: なし。
9.  **表の材料の型 `tableSources` と `tabLabel` は PR-2 と PR-1 で作り、`readTableSources` は PR-3 で作る。** `tabLabel` は Herdr の読み取りが返す型なので `herdr.go`（PR-1）に、`tableSources` は `layoutTable` の引数なので `table.go`（PR-2）に置く。`readTableSources` はラウンドへの配線なので `snapshot.go`（PR-3）に置く。設計書 3.1 の「名前は計画で変えてよい」の範囲で、名前は設計書のままにする。ADR: なし。
10. **CLAUDE.md の 2 つの例外は、PR-2 で最終の形で書く。** 例外の文は、表の文字列が道具の出力に入ることを述べる。PR-2 の時点では表は出力に入らないが、PR-2 と PR-3 の間だけ例外の文に現在の形を書くと、PR-3 でまた直すことになるので、最終の形で書く。ADR: なし（例外の文は AC-26・AC-28 が定め、ADR-0035・ADR-0046 は要件定義のときに更新済み）。
11. **PR-3 の実物の確かめは、PR-3 の `/runplan` のセッションの中で、人が chief の worktree を PR-3 のブランチの先頭（detached）にして `make install-chief` を行い、確かめてからマージする。** 0093 の計画書の決定 3 と同じ手順である。セッションの中で行うので、PR-3 の自動送信は `auto` のままにする。ADR: なし。

### 1.4. 既存コード調査結果

| 領域 | 今あるもの（`790652e3`） | 変えること |
|---|---|---|
| `Sanitize` | `Sanitize`（`internal/chief/sanitize.go:52-69`）が 1 文字ずつ置き換えを選ぶ。置き換える文字の判定は `hiddenRune`（`:85-100`）、エスケープは `writeEscape`（`:103-112`）。テストは `TestSanitize`（`internal/chief/sanitize_test.go:10`） | 単位の列を返す `sanitizeUnits` と `displayUnit`・`unitKind` を足し、`Sanitize` は単位をつないだものを返す（設計書 3.5）。`TestSanitize` は変えない。PR-1 |
| `CapHead` | `CapHead(text string)`（`internal/chief/sanitize.go:32-41`）が `headMaxBytes`（64 KiB、`:17`）で切る。呼び出しは `internal/chief/detail.go:605`・`:627` と `TestCapHead`（`internal/chief/sanitize_test.go:35`）だけ（`790652e3` で `grep -rn 'CapHead(' internal cmd` の結果がこの 3 か所と定義だけだった） | 上限を引数で受け取る。2 つの呼び出しは `headMaxBytes` を渡す（設計書 4 章）。PR-1 |
| Herdr の読み取り | `Agents`（`internal/chief/herdr.go:136-142`）が `herdr agent list` を走らせ `parseAgentList`（`:238-256`）で読む。`parseAgent`（`:273-321`）は `tab_id` を読まない。封筒の確かめは `herdrResult`（`:190-224`）、ID の形は `paneIDForm` と `tabIDForm`（`:483`）、形に合わない出力は `errHerdrOutput`（`:19`）。`tab list` と `pane layout` を読む関数はない | `agentListing`・`parseAgentTabs`・`tabLabels`・`parseTabList`・`paneWidth`・`parsePaneLayout` と `tabLabel` を足し、`Agents` は `agentListing` のエージェントを返す（設計書 3.1・3.7）。`parseAgent` は変えない（設計書 1.2）。封筒は `herdrResult`、配列は `requiredArray`（`:226`）、タブの ID は `tabID`（`:474`）と `tabIDForm` を使い、項目ごとの読み方は `parsePaneList`（`:631-678`）に倣う。`label` は空を受け付けるので `strictjson.RequiredStringAllowEmpty` で読み、空の文字列を拒む `strictjson.RequiredString` と `requiredObjectStrings`（`:832`）は使わない。PR-1 |
| `Agents` のほかの呼び出し | `readRound`（`internal/chief/snapshot.go:98`）、`chiefWorktree`（`internal/chief/create.go:380`）、`checkMadePanes`（`:538`）、`removal.go:657`・`:685`・`:820`、ほかに `wait.go`・`request.go`・`records.go` | `readRound` だけを `agentListing` に替える（PR-3）。ほかは変えない（設計書 3.7） |
| Herdr のサンプル | `internal/chief/testdata/herdr/` に `agent-list.json` などがあり、`tab list` と `pane layout` のサンプルはない。採った日と版は `internal/chief/testdata/README.md` | `tab-list.json` と `pane-layout.json`（分割したタブ）を足し、README に採った日・版・置き換えを書く。PR-1 |
| 記録係 | `readOnlyCommands`（`internal/chief/test_helpers.go:217-221`）の herdr の行が、`agent list/get/read`・`workspace list`・`pane list`・`pane process-info` を許す | `tab list` と `pane layout` を足す。PR-1 |
| テストの world | `newWorld`（`internal/chief/world_test.go:101`）がエージェントの一覧を作る。エージェントの JSON（`world_test.go:24` の周り）は `tab_id` を持たない | エージェントの JSON に `tab_id` を足し、`tab list` と `pane layout` の答えを `world.finish`（`internal/chief/world_test.go:229`）に持たせる。`sendWorld`・`waitWorld`・`mergeWorld`・`createWorld` は `finish` を通るので、それを受け継ぐ。記録係は答えのないコマンドでテストを失敗させる（`internal/chief/test_helpers.go:502`）。PR-3 |
| `cmd/chief` の結合テスト | `fakeHerdr.Run`（`cmd/chief/main_test.go:88-120`）は、知らないコマンドでテストを失敗させる。`TestStartThenSnapshot` と `TestStartThenTickAndWait` が `snapshot` と `tick` を走らせ、出力のリストが `null` でないことを確かめる（`:329-336`・`:357`） | `fakeHerdr` に `tab list` と `pane layout` の答えを足し、出力の確かめに `table` と `table_unreadable` を足す。PR-3 |
| ラウンド | `TakeSnapshot`（`internal/chief/snapshot.go:79-85`）と `Tick`（`internal/chief/tick.go:30-87`）が `readRound`（`snapshot.go:90-145`）の後に `finishSnapshot`（`:151-169`）を呼ぶ。`Tick` の `finishSnapshot` は `tick.go:44`・`:86` の 2 か所。`Targets`（`internal/chief/create.go:816-821`）も `readRound` を呼ぶ | `roundView` に pane とタブの対応を持たせ、`readTableSources` を足し、`finishSnapshot` が表を組む（設計書 2.3・3.7）。`Targets` は変えない。PR-3 |
| 出力 | `SnapshotOutput`（`internal/chief/output.go:17-27`）、`Snapshot.Output`（`:262-`）。リストは `null` でなく `[]`（`:260-261` の注釈）。JSON は `write`（`cmd/chief/main.go:324-`）が `json.Marshal` で書く | `table` と `table_unreadable` を足す（設計書 3.1・3.8）。PR-3 |
| 値の名前 | `OutputValueNames`（`internal/chief/output.go:486-501`）、`namesUpToUnknown`（`:503-517`）、`maxValueNames`（`:519-521`）。`OutputValueNames` を呼ぶのは `cmd/chief/docs_test.go:487` と `internal/chief/output_test.go:22` だけ。`namesUpToUnknown` を呼ぶのは `OutputValueNames` と `output_test.go:36-37` | `OutputValueNames` を除き、`namesUpToUnknown` と `maxValueNames` を `test_helpers.go` に移す（設計書 3.12）。PR-3 |
| `Protocol` | `const Protocol = 3`（`internal/chief/records.go:29`）。`/chief` の定義の `--protocol 3`（`.claude/commands/chief.md:98`・`:107`・`:336`・`:340`・`:346`・`:353`・`:369`・`:371`・`:375`・`:437` ほか）とガイドの値の表（`docs/dev/guides/chief_guide.md:28`）。`TestDocumentedProtocolMatches`（`cmd/chief/docs_test.go:205`）と `TestGuideValuesMatchTool`（`:405`）が比べる | `main` の値 + 1 にする（AC-25）。PR-3 |
| 定義の値の表のテスト | `TestDisplayValuesMatchTool`（`cmd/chief/docs_test.go:469-503`）と定数 `displayTableHeader`（`:462`） | 単独のコミットで除く（§1.3 の決定 6）。PR-3 |
| `/chief` の定義 | 手順 2 の項目 2（`.claude/commands/chief.md:131-203`）が Markdown の表と値の表を定める。規則 2（`:28-33`）がデータとして扱う文字列を並べる。`TestDocumentedInvocationsParse`（`cmd/chief/docs_test.go:257`）は、`chief` で始まるコードスパンをすべて呼び出しとして読む | 項目 2 を置き換え、規則 2 に `table` と `table_unreadable` を足す（設計書 3.9）。新しい文に不完全な呼び出しをコードスパンで書かない。PR-3 |
| ガイド | 2 章の値の表（`docs/dev/guides/chief_guide.md:28`）、入れ直しの手順（`:30-`）、3.1（`:55-100`）、8 章（`:175-199`）。8 章の「人が引数なしで打ったコマンドは示されない」の項（`:196`） | 設計書 3.10 のとおり直す。PR-3 |
| CLAUDE.md | 「Declare, don't infer」の chief の画面の読み取りの例外（`CLAUDE.md:518-519` の「Every other file of the tool …」）、「Go Idioms」の例外の項（`:872`・`:909-918`・`:919-933`） | 設計書 3.11 のとおり 2 つの例外を足し、3 つの文を直す。PR-2 |
| パッケージの一覧 | `docs/dev/conventions/package_reference.md:122` が `OutputValueNames` を挙げる | 除く（§1.3 の決定 7）。PR-3 |
| ADR | ADR-0046 の決定（`docs/dev/adr/0046-chief-of-staff.md:30`・`:46`）と経緯（`:147`）、ADR-0035 の経緯（`docs/dev/adr/0035-language-policy.md:72`）は要件定義のときに更新済み | 変えない（AC-27、設計書 1.2 の末尾） |

具体の型で振る舞いを選ぶ設計は 2 つある。

-   **`displayUnit.kind`。** 改行とタブを空白にする処理が、この種類で選ぶ。`displayUnit` を作るのは `sanitizeUnits` だけにする。
-   **`tabLabel.hasLabel`。** ラベルとして示すか pane の ID を示すかを、この値で選ぶ。`tabLabel` を作るのは `parseTabList` だけにする。

`internal/chief` の中では、どちらも複合リテラルで作れる。本計画は、型をほかのパッケージへ移さず、同じパッケージのガードで守る。今のガード（`internal/chief/construction_test.go:20-25` の `guardedTypes`。`TestGuardedTypesAreBuiltOnlyByTheirConstructors` が、テストでない `.go` のファイルの複合リテラルとフィールドへの代入を、許した関数の外で見つける）に、`displayUnit` は `sanitizeUnits` を、`tabLabel` は `parseTabList` を許した関数として足す（ステップ 1-3・1-5）。型をほかのパッケージへ移す案は、`Sanitize` の規則を `internal/chief` の外へ出すことになり、設計書 3.5 の「同じ規則を 2 か所に書かない」に反するので採らない。`unitKind` の 0 の値は、`unitText` を含むどの種類でもない値にし、`table.go` の `switch` の `default` は、その単位の文字を示さず `?` を示す（CLAUDE.md の「Declare, don't infer」）。

### 1.5. 実装の際に決めたこと（2026-11-19）

1.  **使える幅は、chief のタブを分割していなければ pane の幅 − 2、分割していれば pane の幅 − 4 とし、分割しているかを pane の配置の `layout.splits` が空かどうかで読む（人が選んだ）。** ステップ 1-1 の実物の確かめで、分割した pane では Herdr の枠の分の 2 桁が減り、拡大したときは `rect.width` が分割したときの幅のままだと分かった（§7「PR-1 の実行の記録」）。分割しない使い方が主なので、そのときの幅を最適にする。理由と採らなかった案は、要件書と設計書 1.5 の「実装の際に決めたこと（2026-11-19）」にある。これに合わせ、ステップ 1-5 で `parsePaneLayout` が `splits` を読み、そのテストを足し、ステップ 2-4 の `TestTableWidth` に分割したときの余白を足した。ADR: [ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新する。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 実物の確かめ、サンプル、入力の部品（設計書 8 章の 1、2 の一部、3 の一部）

**変更するファイル:** `internal/chief/sanitize.go`・`sanitize_test.go`、`internal/chief/construction_test.go`、`internal/chief/detail.go`、`internal/chief/herdr.go`・`herdr_test.go`、`internal/chief/test_helpers.go`、`internal/chief/testdata/herdr/`・`testdata/README.md`、本計画（§7 の実行の記録）

-   [x] **ステップ 1-1**: 実物の確かめを行う（§1.3 の決定 2、設計書 1.2）。セッションの pane の幅を `herdr pane layout --pane "$HERDR_PANE_ID"` で読み、次の 2 つを確かめる。(1) 表示幅が「pane の幅 − 2」の行と「pane の幅 − 1」の行を code block で示し、人に折り返すかを尋ねる。(2) 人にタブを分割してもらい、分割した後と拡大した後の `rect.width` を読み、人に画面の幅と等しいかを尋ねる。結果と Claude Code・Herdr の版を §7 の「PR-1 の実行の記録」に書く。どちらかが成り立たなければ、ここで止まって §1.3 の決定 2 のとおり人と決め直す。
-   [x] **ステップ 1-2**: `herdr tab list` と、分割したタブでの `herdr pane layout --pane <pane>` のサンプルを `internal/chief/testdata/herdr/` に採る（§1.3 の決定 8）。採った日・版・置き換えを `testdata/README.md` に足す。`agent-list.json` のすべてのエージェントが `tab_id` を持つことは、設計書 3.7 が確かめている。
-   [ ] **ステップ 1-3**: `sanitize.go` に `displayUnit`・`unitKind`・`sanitizeUnits` を足し、`Sanitize` を単位をつなぐ形にする（設計書 3.1・3.5）。`TestSanitize` は変えずに通す。`TestSanitizeUnits` を足し、エスケープ・`\\`・改行・タブ・そのほかの文字がそれぞれ 1 つの単位で、改行とタブがその種類を持つことを、決まった期待値で確かめる。`TestSanitize` の入力について、単位をつないだものが `Sanitize` と等しいことも確かめる。ASCII でない入力は `\u` か `\U` のエスケープで書き、Unicode 15.0 の後に書式文字かどうかが変わった符号位置は使わない（Go の版で結果が変わるため。§1.3 の決定 3）。`unitKind` の 0 の値を §1.4 のとおりにし、`guardedTypes` に `displayUnit` を足す。`TestConstructionGuardFindsViolations` と同じやり方で、`sanitizeUnits` の外の `displayUnit` の複合リテラルをガードが見つけることを確かめる。
-   [ ] **ステップ 1-4**: `CapHead` が上限を引数で受け取るようにし、`detail.go` の 2 つの呼び出しは `headMaxBytes` を渡す（設計書 4 章）。`TestCapHead` を、上限を渡す形に直し、上限の前後と UTF-8 の文字の途中で切らないことを確かめる。
-   [ ] **ステップ 1-5**: `herdr.go` に `tabLabel`・`agentListing`・`parseAgentTabs`・`tabLabels`・`parseTabList`・`paneWidth`・`parsePaneLayout` を足し、`Agents` は `agentListing` のエージェントを返す形にする（設計書 3.1・3.7）。`readOnlyCommands` に `tab list` と `pane layout` を足し、`guardedTypes` に `tabLabel` を足す。テストは次を確かめる。
    -   `TestReadHerdrSamples` に、ステップ 1-2 のサンプルと `agent-list.json` の pane とタブの対応を足す。空のラベルが `hasLabel` 偽に、空でないラベルが `hasLabel` 真になることも確かめる。サンプルのラベルに ASCII でない文字があれば、`\u` か `\U` のエスケープで書く。
    -   `TestHerdrRejectsUnknownShapes` に、設計書 7.1 の「Herdr の読み取り」の形に合わない出力をそれぞれ足し、`errHerdrOutput` で拒むこと（一部だけを使わないこと）を確かめる。幅は 0・負・1001 を拒み、1 と 1000 を受け付ける。`layout.splits` がない・配列でない出力も拒む（§1.5 の決定 1）。
    -   `parsePaneLayout` が、`splits` が空の配置を分割していないと、空でない配置を分割していると読むこと（§1.5 の決定 1）。
    -   `TestAgentListingTabsUnreadable` を足し、pane とタブの対応だけが読めないとき、エージェントの一覧は読め、エラーが `paneTabsErr` に残ることを確かめる。
    -   `TestHerdrCommandLines` に、`tab list` と `pane layout --pane <ID>` のコマンドの行を足す。

**完了の条件:** 各ステップのテストが通る。`make test`・`make lint`・`make deadcode` が通る。`snapshot` と `tick` の出力と、ラウンドが走らせるコマンドは変わらない（`snapshot_test.go`・`autosend_test.go`・`network_test.go` の今のテストが変えずに通る）。

### PR-1 作成ポイント: real checks, Herdr samples, sanitize units, and Herdr readers

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `feat(0102): 表の材料の Herdr の読み取りと Sanitize の表示の単位を作る`

**レビュー観点**: 実物の確かめの結果が設計書 1.2 の前提（左の余白 2 桁、`rect.width` が描く幅）を満たしているか / Herdr の 3 つの読み取りが形に合わない出力を一部だけ使わずに拒むか / `Sanitize` の結果が変わらず、単位の境目と種類が決まった期待値で確かめられているか / サンプルに採った人を特定できる値が残っていないか

**実装モデル要件**: opus

**判定理由**: ステップ 1-1 は、結果によって要件書 AC-04 を直すかを決める実物の確かめ（mkplan2.md の「Phase 0-style feasibility probe」）で、ステップ 1-5 は Herdr が書く外部の値を厳しく読む信頼の境界である。Sonnet は、確かめが成り立たないのに先へ進んだり、形に合わないタブの一覧の一部だけを使ってしまったりしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 表示幅、表を組む処理、CLAUDE.md の例外（設計書 8 章の 2 の残り、4 の一部、5 の CLAUDE.md）

**変更するファイル:** `internal/chief/textwidth.go`・`textwidth_test.go`・`textwidth_gen.go`（すべて新規）、`internal/chief/table.go`・`table_test.go`（新規）、`CLAUDE.md`、本計画（§7 の実行の記録）

-   [ ] **ステップ 2-1**: 生成の道具（`textwidth_gen.go`）を書き、Unicode 17.0.0 の `EastAsianWidth.txt` から幅の表を作って `textwidth.go` に置く（§1.3 の決定 3・4、設計書 3.5）。元のファイルを取りに行く前に人の承認を得る。`@missing` の既定（設計書 3.5）も表に含める。版・取得元の URL・SHA-256 を表の注釈と §7 の「PR-2 の実行の記録」に書く。§1.3 の決定 4 の `awk` での数の比べと、`TestUnitWidth` の期待値に引いた元のファイルの行も §7 に書く。`TestGuardedTypesAreBuiltOnlyByTheirConstructors` はビルドの印によらずパッケージのすべての `.go` のファイルを読むので、`package main` の生成の道具を置いた後もこのテストが通ることを確かめる。
-   [ ] **ステップ 2-2**: `textwidth.go` に、表示の単位の表示幅を返す関数を書く（設計書 3.5 の表）。`TestUnitWidth` を足し、設計書 7.1 の「表示幅」のとおり、W・F の文字（`🛑`・`🔶`・`🟢` を含む）が 2、ASCII・曖昧な文字（`…`・`─`）・結合文字・異体字セレクタが 1、エスケープがそのバイト数であること、幅の表の範囲の境目の前後の文字を確かめる。境目の期待値は、元のファイルの行から人手で選ぶ（§1.3 の決定 4）。F の文字（U+FF01 など）、W になる未割り当ての符号位置（CJK の区画の未割り当ての符号位置。元のファイルが `@missing` の行で示すか、範囲の行で明示するかは版によるので、どちらの行で W になるかを §7 に書く）と、元のファイルのどの行にもなく 1 桁になる符号位置、Unicode 15.0 の後に W になった符号位置を含める。最後のものは、表が 17.0.0 から作られ、Go 1.26.5 の表によらないことを示す。入力は `\u` か `\U` のエスケープで書く。
-   [ ] **ステップ 2-3**: `table.go` に、`tableSources`、決まった語、内容の文字列の組み方を書く（設計書 3.3・3.4）。決まった語は型の値ごとの `switch` で選び、`default` は `?` にする。外から来る値は、単位に分ける前に `CapHead` で 16 KiB までに切る（設計書 3.4 の「値の長さ」、4 章の「同じ処理を 2 つ書かない」）。表の `─` と `…` は `\u` か `\U` のエスケープで書き、日本語の literal は AC-26 の語だけにする。テストは `table_test.go` に置き、次を確かめる。
    -   `TestTableFixedWords`: 急ぎの度合い・進み具合・文書・実行中のコマンドが決まらない理由の、型のすべての値が `?` でない語を持ち、語が要件書 AC-11〜AC-13・AC-20 の表のとおりであること、表にない値が `?` になること。値は `namesUpToUnknown` に出力の名前の関数を渡して数える（設計書 7.1）。
    -   `TestTableContent`: 状態の文字列ごとの部品と、値がない部品を並べないこと（AC-14）。
    -   `TestTableQuestions`: 示す質問と、件数と `マージ確認` の添え書き（AC-16）。
    -   `TestTableTabDisplay`: タブの表示と、引けない 5 つの場合に pane の ID を示して `table_unreadable` に挙げること（AC-18・AC-19）。
    -   `TestTableNewlinesAndTabs`: 改行とタブが空白になり、`Row` の値が変わらないこと（AC-17）。単位の列を受け取る `table.go` の内部の関数を通して、種類が 0 の値の単位が文字でなく `?` で示されることも確かめる（§1.4）。
-   [ ] **ステップ 2-4**: `table.go` に、列の幅、名前の切り方、内容の折り返しと切り方、`layoutTable` を書く（設計書 3.2・3.6・6.2）。テストは次を確かめる。
    -   `TestTableShape`: 行の並び（AC-06）、同じ列が見出しとすべての行で同じ表示幅の位置から始まること（AC-07。全角と絵文字を含む値で）、各列の 1 行目と 2 行目の値（AC-09）、行がないときの `workspace なし`（AC-10）、末尾の空白がないこと。
    -   `TestTableNameColumn`: 名前の列の幅と 20 桁での切り方（AC-08）。ちょうど 20 桁と 21 桁の名前、20 桁目にまたがる幅 2 の文字を含める。
    -   `TestTableWidth`: どの行の表示幅も使える幅以下であること（AC-04）。使える幅が、分割していなければ pane の幅 − 2、分割していれば pane の幅 − 4 であること（§1.5 の決定 1）。幅が 0（読めない）なら 80 桁で組むこと（AC-05）。内容の列の幅が 19・20・21 桁になる使える幅を試し、内容の列が 20 桁を取れないときは列を 20 桁で組み、行が使える幅を超えても行の頭が変わらないこと（AC-05）。幅が 1 と 1000 のとき。
    -   `TestTableWrap`: 設計書 7.1 の「内容」の折り返しの位置と `…` での切り方、エスケープの途中で折り返しも切りもしないこと（AC-15）。
    -   `TestTableEscapes`: AC-21 の 8 つの外から来る値（名前、PR の題名、質問の文、送れない理由、読めなかったもの、モデル、タブのラベル、pane の ID）のそれぞれに、ESC・C1 制御文字・ゼロ幅の文字・正しくない UTF-8 を入れ、表の中でエスケープとして現れること（値ごとに 1 つの行の表）。
    -   `TestTableLargeValues`: 64 KiB の題名と、結合文字を 1 万個連ねた質問の文でも、内容が 2 行、名前が 20 桁に収まること。16 KiB の境目にまたがる複数バイトの文字を含める。
    -   `TestTableGolden`: 全角の文字・絵文字・エスケープ・折り返し・切りを含む行の表を組み、期待する行を文字列のまま並べて比べる（表示幅の関数を使わない期待値）。
-   [ ] **ステップ 2-5**: code block を閉じないことを確かめるヘルパーを `table_test.go` に置き、表を組むすべてのテストが組んだ表に当てる（設計書 7.3）。ヘルパーはエラーを返す関数にする。`TestTableKeepsCodeBlockOpen` で設計書 7.3 の値を入れた表を組み、`FuzzTableKeepsCodeBlockOpen` で値を任意の文字列にして同じヘルパーを当てる（AC-22）。`TestCodeBlockCheckRejects` で、0〜3 桁の空白に続く ```` ``` ````、`~~~`、外から来る値で始まる行、CR か LF を含む行をヘルパーが拒むことを確かめる。
-   [ ] **ステップ 2-6**: CLAUDE.md に、設計書 3.11 の 2 つの例外を足し、3 つの文を直す（AC-26・AC-28、§1.3 の決定 10）。「Go Idioms」の最初の項の「One exception」を、例外が 2 種類あることに合う言い方に直す。

**完了の条件:** 各ステップのテストが通る。`make test`・`make lint`・`make deadcode` が通る。生成の道具を `go run` で走らせ直して `textwidth.go` に差分がない。`snapshot` と `tick` の出力は変わらない。日本語の literal は `table.go` と `table_test.go` にだけある（PR のレビューで読む）。新しいテストのファイルは `//go:build test` を持つ。

### PR-2 作成ポイント: display width, table layout, and the CLAUDE.md exceptions

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6

**推奨タイトル**: `feat(0102): 表示幅を数え、状況の表を組む処理と CLAUDE.md の例外を足す`

**レビュー観点**: 折り返しと切り方が設計書 3.6 のとおりで、エスケープの途中で折り返しも切りもしないか / どの行も決まった語か印か 4 桁以上の空白で始まり、fuzz のテストがそれを当てているか / 幅の表が Unicode 17.0.0 の元のファイルから作られ、版と SHA-256 が記録されているか / CLAUDE.md の 2 つの例外の範囲と境界が AC-26・AC-28 のとおりで、今の 3 つの文と食い違わないか

**実装モデル要件**: opus

**判定理由**: ステップ 2-3〜2-5 は、外から来る文字列を `/chief` のセッションに渡す表へ入れる信頼の境界で、エスケープ（AC-21）と code block を閉じないこと（AC-22）を守る（mkplan.md の panel-mode の条件のうち security-gate）。Sonnet は、AC-21 の 8 つの値の 1 つにエスケープを当て忘れたり、0〜3 桁の空白に続く `~~~` で始まる行をヘルパーが見逃したり、エスケープの途中で切ったりしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 配線、出力、`Protocol`、`/chief` の定義とガイド、実物の確かめ（設計書 8 章の 3 の残り、4 の残り、5 の残り、6）

**変更するファイル:** `internal/chief/snapshot.go`・`tick.go`・`output.go`・`records.go`・`test_helpers.go` とそのテスト（`snapshot_test.go`・`autosend_test.go`・`output_test.go`・`world_test.go`）、`cmd/chief/main_test.go`・`docs_test.go`、`.claude/commands/chief.md`、`docs/dev/guides/chief_guide.md`、`docs/dev/conventions/package_reference.md`、`docs/tasks/0102_chief_text_table/_checks/`（新規）、本計画（§7 の実行の記録）

-   [ ] **ステップ 3-1**: `readRound` を `agentListing` に替え、`roundView` に pane とタブの対応とその読み取りのエラーを持たせる。`snapshot.go` に `readTableSources` を書き、`TakeSnapshot` と `Tick`（`BeginRound` より前）が呼ぶ。`finishSnapshot` が、行を並べた後に `layoutTable` で表を組み、`Snapshot` の `Table` と `TableUnreadable` に入れる（設計書 2.3・3.7・4 章）。材料のエラーの文は 512 バイトまでに切り（`CapHead`）、`Sanitize` する。同じコミットで、テストの world のエージェントの JSON に `tab_id` を足し、`tab list` と `pane layout` の答えを `world.finish` に、`cmd/chief` の答えを `fakeHerdr` に足す（§1.4）。記録係と `fakeHerdr` は答えのないコマンドでテストを失敗させるので、このコミットだけで `make test` が通るようにする。
-   [ ] **ステップ 3-2**: `SnapshotOutput` に `table` と `table_unreadable` を足す（設計書 3.1・3.8）。テストは次を確かめる。
    -   `TestSnapshotTableMatchesRows`（`snapshot_test.go`）: 出力の表に、同じ出力の `rows` の名前・タスク・PR の番号と、各行の急ぎの度合いと進み具合から `table.go` が決める印と状態の文字列が、`rows` と同じ並びで現れること（AC-01・AC-02）。
    -   `TestTableSourcesUnreadable`（`snapshot_test.go`）: `tab list` と `pane layout` が失敗する・形に合わないラウンドと、エージェントの一覧の pane とタブの対応が読めない（`tab_id` がない、`pane_id` が重なる）ラウンドでも、ラウンドが失敗せず、行の状態・急ぎの度合い・`pending`・`problems` が読めたときと同じで、表が 80 桁と pane の ID で組まれ、`table_unreadable` が空でないこと（AC-05・AC-19、要件書 4.3）。512 バイトを超え ESC を含む Herdr のエラーの文が、`table_unreadable` で切られてエスケープされること。
    -   `TestTickTableSourcesUnreadable`（`autosend_test.go`）: `tick` で表の材料が読めなくても、`AutoSends` と pane への入力が読めたときと同じで、表が 80 桁と pane の ID で組まれること（AC-05・AC-19）。
    -   `TestTickTableShowsAutoSend`（`autosend_test.go`）: `tick` の出力の表に、自動送信の後の行の状態の文字列（`table.go` の対応で決まるもの）が現れること（AC-01）。送る前の行で組んだ表では失敗することを確かめる。
    -   `TestSnapshotOutputTable`（`output_test.go`）: 表の材料がすべて読めたときに `table_unreadable` が `[]` で、`null` でないこと。
    -   `TestTickRunsOnlyReadsInTheRightPlaces` と `TestSnapshotRunsOnlyReadsInTheRightPlaces`（`network_test.go`）が変えずに通ること。
    -   `TestStartThenSnapshot` と `TestStartThenTickAndWait`（`cmd/chief/main_test.go`）: `table` が空でないリストで、`table_unreadable` が `null` でないリストであること（AC-01）。
    -   `internal/chief` の中で表を組むこれらのテストは、ステップ 2-5 のヘルパーを組んだ表に当てる。`cmd/chief` のテストは別のパッケージなのでヘルパーを呼べず、`table` が空でないことだけを確かめる。
-   [ ] **ステップ 3-3**: `TestDisplayValuesMatchTool` と定数 `displayTableHeader` を、単独のコミットで除く（§1.3 の決定 6）。`make coverage-compare BASE=<親のコミット>` を、10 分以上のコマンドの時間制限を付けるか、バックグラウンドで走らせ、最後の行までの出力をコミットのメッセージと PR の説明に書く。
-   [ ] **ステップ 3-4**: `OutputValueNames` を除き、`namesUpToUnknown` と `maxValueNames` を `test_helpers.go` に移す。`TestOutputValueNames` を、`OutputValueNames` なしで出力の名前が値ごとに 1 つずつあることを確かめる形に直す（設計書 3.12）。`package_reference.md` から `OutputValueNames` を除く（§1.3 の決定 7）。
-   [ ] **ステップ 3-5**: `Protocol` を `origin/main` の値 + 1 にし、`/chief` の定義のすべての `--protocol` とガイドの 2 章の値を同じ値にする（AC-25）。`/chief` の定義の手順 2 の項目 2 を、設計書 3.9 のとおり置き換え、規則 2 に `table` と `table_unreadable` を足す（AC-03）。
-   [ ] **ステップ 3-6**: `_checks/` に、§1.3 の決定 5 のシェルのスクリプトを書く。各確かめが少なくとも 1 つの壊し方で失敗することを確かめ、PR のブランチで走らせて、出力とマージの基点・比べたコミットの ID を §7 の「PR-3 の実行の記録」に書く。
-   [ ] **ステップ 3-7**: ガイドを設計書 3.10 のとおり直す（AC-24）。2 章に入れ直しの順と `protocol_mismatch` の意味を、3.1 に表の見方を、8 章に既知の制限を書き、8 章の今の項の語を直す。
-   [ ] **ステップ 3-8**: 実物の確かめを行う（§1.3 の決定 11、設計書 7.3 の (1)〜(5)）。人が、動いている `/chief` を止め、chief の worktree を PR-3 のブランチの先頭にして `make install-chief` を行い、`/chief` を起動し直す（ガイドの 2 章の順）。(3) は、人が chief の pane の幅を狭めて確かめる（幅を読めないときの 80 桁は `TestTableSourcesUnreadable` が確かめる）。(5) は、人が本タスクの workspace（PR-3 の `/runplan` のセッションがある workspace。表には名前の列に出る）の名前に ```` ``` ```` と `~~~` を一時的に入れて確かめ、確かめた後に元の名前に戻す。PR の題名・質問の文・タブのラベルに入れたときは、`TestTableKeepsCodeBlockOpen` が確かめる。ほかのタスクの workspace の名前、PR の題名、質問の文は変えない。`/runplan` のセッションは Herdr にも GitHub にも書かず、これらの操作は人が行う。結果を §7 の「PR-3 の実行の記録」に書く。どれかが成り立たなければ、ここで止まって人と決め直す。(1)・(4) は設計書 1.2 のとおり、(2)・(3)・(5) は行の組み方か幅の数え方を決め直すか、既知の制限として受け入れるかを尋ねる。確かめたコミットを §7 に書く。その後のコミット（`/fixpr` を含む）が `internal/chief` か `.claude/commands/chief.md` を変えたときは、マージの前に、変わった部分に関わる確かめをやり直して §7 に書く。

**完了の条件:** 各ステップのテストとスクリプトが通る。`make test`・`make lint`・`make deadcode` が通る。実物の確かめの結果が §7 にある。

### PR-3 作成ポイント: wiring, output, protocol, chief command, guide, and real checks

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8

**推奨タイトル**: `feat(0102): 状況の表を道具の出力に入れ、/chief の定義とガイドを道具の表に合わせる`

**レビュー観点**: 表の材料を読めないラウンドでも、行の状態・急ぎの度合い・送る判断が変わらないか / テストの削除がそのコミットだけで、カバレッジの比較が通っているか / `/chief` の定義の手順 2 の差分が項目 2 と `--protocol` だけか / ガイドの 3.1 と 8 章が新しい表の形と既知の制限を書いているか / §7 の実物の確かめの結果が設計書 7.3 の (1)〜(5) を満たし、確かめたコミットが PR の先頭と食い違わないか

**実装モデル要件**: opus

**判定理由**: ステップ 3-3〜3-5 は、`Protocol` を上げて `/chief` の定義を道具の表に移す移行と、ガードテストの削除を同時に持ち、ステップ 3-1・3-2 は、表の材料を読めなくてもラウンドと自動送信の振る舞いを変えないことを守る（mkplan.md の panel-mode の条件のうち migration）。Sonnet は、削除のコミットにほかの変更を混ぜたり、テストの world に材料の答えを足すだけで、読めないときの `tick` の振る舞いを確かめ漏らしたりしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 確かめ |
|---|---|---|
| M1（PR-1） | 実物の確かめの結果、Herdr のサンプル、`sanitizeUnits`、`CapHead` の上限の引数、Herdr の 3 つの読み取り | §7 の「PR-1 の実行の記録」。出力が変わらない |
| M2（PR-2） | 幅の表と表示幅、`layoutTable`、CLAUDE.md の 2 つの例外 | §7 の「PR-2 の実行の記録」。出力が変わらない |
| M3（PR-3） | `snapshot` と `tick` の出力の `table`・`table_unreadable`、`Protocol`、`/chief` の定義、ガイド | §7 の「PR-3 の実行の記録」。実物の確かめ |

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | 実物の確かめ、Herdr のサンプルと 3 つの読み取り、`Sanitize` の表示の単位、`CapHead` の上限 | opus | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 | 幅の表と表示幅、表を組む処理、CLAUDE.md の例外 | opus | auto |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 | 配線、出力、`Protocol`、`/chief` の定義、ガイド、テストの削除、実物の確かめ | opus | auto |

どの PR も、前の PR のマージと次の `/runplan` の起動の間に、`/runplan` の外の作業を置かない。PR-1 と PR-3 の実物の確かめは、`/runplan` のセッションの中で人と一緒に行う（§1.3 の決定 2・11）。そのため、自動送信はすべて `auto` とする。

## 4. テスト戦略 (Test Strategy)

-   **単体テスト。** 表示幅（`textwidth_test.go`）、表示の単位（`sanitize_test.go`）、Herdr の読み取り（`herdr_test.go`）、表を組む処理（`table_test.go`）を、設計書 7.1 のとおり確かめる。目標は、新しい関数のすべての分岐をテストが通ることで、`go test -cover` の数値の目標は置かない。
-   **結合テスト。** テストの world と記録係で、`snapshot` と `tick` を走らせ、表と `rows` の一致、材料を読めないラウンド、自動送信の後の表を確かめる（ステップ 3-2）。
-   **fuzz テスト。** `FuzzTableKeepsCodeBlockOpen` は、`make test` では種の入力だけを走らせる（`go test` の既定）。
-   **後方互換。** `rows` の値は変えない（AC-02）。今の `snapshot_test.go`・`autosend_test.go`・`network_test.go` のテストは、テストの world の答えを足すほかは変えずに通す。古い道具と新しい定義は `Protocol` で断る（AC-25、設計書 3.8）。
-   **日本語の literal。** 日本語の文字を置いてよいテストは `table_test.go` だけで、ほかのテストは ASCII でない入力を `\u` か `\U` のエスケープで書く（設計書 7.1 の末尾）。新しいテストのファイルは `//go:build test` を持つ。
-   **Go の版。** `go.mod` の Go（Unicode 15.0.0）と作業環境の Go（17.0.0）で結果が変わらないように、テストは Unicode 15.0 の後に書式文字かどうかが変わった符号位置を使わない。
-   **具体の型のガード。** `displayUnit` と `tabLabel` を `guardedTypes` に足し、今の `TestGuardedTypesAreBuiltOnlyByTheirConstructors` で守る（§1.4）。
-   **文書。** 番号は `TestDocumentedProtocolMatches` と `TestGuideValuesMatchTool` が比べる。定義の項目 2、ガイドの 3.1 と 8 章、CLAUDE.md の例外は文の中身なので、PR のレビューで読んで確かめる（CLAUDE.md の「Document tests check values, structure, and executable examples, never prose」）。
-   **実物の確かめ。** PR-1 の最初と PR-3 の最後に、人と一緒に行う（§1.3 の決定 2・11）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 実物の確かめで、code block の右端が「pane の幅 − 2」より手前と分かる | PR-1 の最初で止まり、要件書 AC-04 の言葉を直す判断の変更として人と決め直す（§1.3 の決定 2） |
| 端末が幅の表と違う幅で描く文字がある（新しい絵文字、曖昧な文字を 2 桁で描く設定） | 設計書 1.2 のとおり受け入れ、ガイドの 8 章に既知の制限として書く（ステップ 3-7） |
| Herdr の出力の形が版で変わる | 形に合わなければ読めないものとし、80 桁と pane の ID で組む（設計書 3.7）。`table_unreadable` で分かる |
| 並行するタスクが同じ道具の `Protocol` を上げる | `mergepr` のマージの確かめが、`origin/main` の値 + 1 でなければ止める（CLAUDE.md の「Protocol numbers」）。止まったときは mergepr_guide.md の 7 章のとおり直す |
| `make coverage-compare` が時間で揺れる関数で `FAIL` を返す | きれいな作業ツリーで走らせ直し、すべての実行の出力を PR の説明に書く。走らせ直しても同じ関数で落ちるときは、止まって人に示す |
| 取りに行った `EastAsianWidth.txt` が 17.0.0 でない | ファイルの頭の版の行を確かめ、版と SHA-256 を §7 に書く（ステップ 2-1） |

日程のリスクは置かない。このタスクには期日がない。

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8）

`make lint` と `make test` で見つからない後始末は次のとおりである。

- [ ] `OutputValueNames`・`TestDisplayValuesMatchTool`・`displayTableHeader` を引く文書が、本タスクの文書と、完了したタスクの記録（0093・0103）のほかに残っていない（ステップ 3-3・3-4。完了したタスクの記録は変えない）。
- [ ] ガイドの 3.1 と 8 章に、使わなくなった語（「🔴 今」「🟡 後」「🟢 待」「（後に指示あり）」「作成中」「`/runpr` 実行中」）が、今の表の説明として残っていない（ステップ 3-7。PR のレビューで読む）。

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 確かめ | 実装 |
|---|---|---|---|
| AC-01 | test | `internal/chief/snapshot_test.go::TestSnapshotTableMatchesRows`、`internal/chief/autosend_test.go::TestTickTableShowsAutoSend`、`cmd/chief/main_test.go::TestStartThenSnapshot`、`cmd/chief/main_test.go::TestStartThenTickAndWait` | ステップ 3-1・3-2 |
| AC-02 | test | `internal/chief/snapshot_test.go::TestSnapshotTableMatchesRows`、今の `TestSnapshotRowOutput`（`snapshot_test.go:876`）を変えずに通すこと、`internal/chief/table_test.go::TestTableNewlinesAndTabs` | ステップ 2-3・3-1 |
| AC-03 | diff・manual | `docs/tasks/0102_chief_text_table/_checks/` のスクリプト（§1.3 の決定 5、ステップ 3-6）。出力と 2 つのコミットの ID は下の「PR-3 の実行の記録」の AC-03 の項に書く。新しい項目 2 の文は PR のレビューで読む | ステップ 3-5 |
| AC-04 | test・manual | `internal/chief/table_test.go::TestTableWidth`。実物の確かめ（ステップ 1-1・3-8） | ステップ 2-4・3-1 |
| AC-05 | test | `internal/chief/table_test.go::TestTableWidth`、`internal/chief/snapshot_test.go::TestTableSourcesUnreadable`、`internal/chief/autosend_test.go::TestTickTableSourcesUnreadable` | ステップ 2-4・3-1 |
| AC-06 | test | `internal/chief/table_test.go::TestTableShape` | ステップ 2-4 |
| AC-07 | test・manual | `internal/chief/table_test.go::TestTableShape`、`internal/chief/table_test.go::TestTableGolden`。実物の確かめ（ステップ 3-8 の (2)） | ステップ 2-4 |
| AC-08 | test | `internal/chief/table_test.go::TestTableNameColumn` | ステップ 2-4 |
| AC-09 | test | `internal/chief/table_test.go::TestTableShape` | ステップ 2-4 |
| AC-10 | test | `internal/chief/table_test.go::TestTableShape` | ステップ 2-4 |
| AC-11 | test | `internal/chief/table_test.go::TestTableFixedWords` | ステップ 2-3 |
| AC-12 | test | `internal/chief/table_test.go::TestTableFixedWords` | ステップ 2-3 |
| AC-13 | test | `internal/chief/table_test.go::TestTableFixedWords` | ステップ 2-3 |
| AC-14 | test | `internal/chief/table_test.go::TestTableContent` | ステップ 2-3 |
| AC-15 | test | `internal/chief/table_test.go::TestTableWrap`、`internal/chief/textwidth_test.go::TestUnitWidth`、`internal/chief/sanitize_test.go::TestSanitizeUnits` | ステップ 1-3・2-2・2-4 |
| AC-16 | test | `internal/chief/table_test.go::TestTableQuestions` | ステップ 2-3 |
| AC-17 | test | `internal/chief/table_test.go::TestTableNewlinesAndTabs`、`internal/chief/construction_test.go::TestGuardedTypesAreBuiltOnlyByTheirConstructors` | ステップ 1-3・2-3 |
| AC-18 | test | `internal/chief/table_test.go::TestTableTabDisplay`、`internal/chief/herdr_test.go::TestReadHerdrSamples`、`internal/chief/construction_test.go::TestGuardedTypesAreBuiltOnlyByTheirConstructors` | ステップ 1-5・2-3 |
| AC-19 | test | `internal/chief/table_test.go::TestTableTabDisplay`、`internal/chief/snapshot_test.go::TestTableSourcesUnreadable`、`internal/chief/autosend_test.go::TestTickTableSourcesUnreadable`、`internal/chief/herdr_test.go::TestAgentListingTabsUnreadable` | ステップ 1-5・2-3・3-1 |
| AC-20 | test | `internal/chief/table_test.go::TestTableFixedWords` | ステップ 2-3 |
| AC-21 | test | `internal/chief/table_test.go::TestTableEscapes`、`internal/chief/snapshot_test.go::TestTableSourcesUnreadable`（`table_unreadable` の文） | ステップ 2-3・2-4・3-1 |
| AC-22 | test・manual | `internal/chief/table_test.go::TestTableKeepsCodeBlockOpen`、`internal/chief/table_test.go::FuzzTableKeepsCodeBlockOpen`、`internal/chief/table_test.go::TestCodeBlockCheckRejects`。実物の確かめ（ステップ 3-8 の (5)） | ステップ 2-4・2-5 |
| AC-23 | test | `internal/chief/herdr_test.go::TestHerdrRejectsUnknownShapes`、`internal/chief/herdr_test.go::TestReadHerdrSamples` | ステップ 1-5 |
| AC-24 | manual | ガイドの 3.1 と 8 章を PR-3 のレビューで読む（文の中身だけを定める AC） | ステップ 3-7 |
| AC-25 | test | `cmd/chief/docs_test.go::TestDocumentedProtocolMatches`、`cmd/chief/docs_test.go::TestGuideValuesMatchTool`。`origin/main` の値 + 1 であることは `mergepr` のマージの確かめ | ステップ 3-5 |
| AC-26 | manual | CLAUDE.md の「Go Idioms」の例外を PR-2 のレビューで読む（文の中身だけを定める AC） | ステップ 2-6 |
| AC-27 | manual | ADR-0046 の決定と経緯（`docs/dev/adr/0046-chief-of-staff.md:46`・`:147`）を PR-3 のレビューで読み、表の実装と食い違わないことを確かめる。要件定義のときに更新済み（設計書 1.2 の末尾） | なし |
| AC-28 | manual | CLAUDE.md の「Declare, don't infer」の例外を PR-2 のレビューで読む（文の中身だけを定める AC） | ステップ 2-6 |

### PR-1 の実行の記録

**ステップ 1-1（実物の確かめ、2026-11-19）。** Claude Code 2.1.328、herdr 0.9.1。chief の代わりに、この `/runplan` のセッションの pane（`w27:p3`）で、`herdr pane layout --pane "$HERDR_PANE_ID"` で幅を読み、code block の試しの行を人が画面で読んだ。

| 状態 | `rect.width` | `area.width` | 1 行に収まった桁 | 差 |
|---|---|---|---|---|
| 分割なし | 82 | 82 | 80（81 桁の行は末尾の 1 桁が折り返した） | rect − 2 |
| 左右に分割 | 41 | 82 | 37 | rect − 4 |
| 左右に分割（境目を動かした後） | 48 | 82 | 44 | rect − 4 |
| 左右に分割して拡大（`zoomed: true`） | 49（分割したときの幅のまま） | 83 | 79 | area − 4 |

-   (1) は成り立った。分割していない pane で、表示幅が「pane の幅 − 2」の行は折り返さず、「pane の幅 − 1」の行は折り返した。
-   (2) は成り立たなかった。分割した pane では、`rect.width` より 4 桁少ない幅までしか収まらなかった（Claude Code の左の余白 2 桁のほかに、Herdr が分割した pane に描く枠の 2 桁）。拡大したときは、`rect.width` が分割したときの幅のままで、描かれる幅とも違った。
-   人と決め直し、§1.5 の決定 1 のとおり、分割していなければ − 2、分割していれば − 4 とした。拡大したときは、表が画面より狭く組まれることを受け入れた。
-   分割したときの 1 回目の試しの行は、エージェントの数え違いで 39 桁でなく 40 桁だった。人が「37 桁の後で折り返した」と答えたので、結果は上の表のとおりで、2 回目（目盛りの行）でも同じ差を確かめた。

**ステップ 1-2（Herdr のサンプル、2026-11-19）。** `herdr tab list` と、分割して拡大したタブでの `herdr pane layout --pane w27:p3` を、読み取りだけで `internal/chief/testdata/herdr/tab-list.json` と `pane-layout-split.json` に採った。採った人を特定できる値はなく、置き換えはない（`internal/chief/testdata/README.md` の「タスク 0102 の表の材料」）。空のラベルのタブはなかったので、空のラベルはテストの中で作った出力で確かめる。

### PR-2 の実行の記録

（ステップ 2-1 の結果を `/runplan` が書く。）

### PR-3 の実行の記録

（ステップ 3-3 の比較の出力と、ステップ 3-8 の結果・確かめたコミット・やり直した確かめを `/runplan` が書く。）

AC-03 の diff の記録（ステップ 3-6 のスクリプトの出力、マージの基点と比べたコミットの ID）:

（`/runplan` が書く。）

## 8. 成功の基準 (Success Criteria)

-   **機能:** §7 のすべての AC の確かめが通り、実物の確かめの結果が §7 にある。
-   **品質:** `make test`・`make lint`・`make deadcode` が各 PR で通る。テストの削除はカバレッジの比較が `PASS` である。
-   **セキュリティ:** 表に入る外から来る値が `Sanitize` の単位を通り（AC-21）、code block を閉じない（AC-22）。Herdr の新しい出力を厳しく読む（AC-23）。表の材料は表示にだけ使う（`TestTableSourcesUnreadable`）。
-   **文書:** `/chief` の定義、ガイド、CLAUDE.md、パッケージの一覧が新しい形に合う。

## 9. 次のステップ (Next Steps)

-   PR-3 のマージの後、動いている `/chief` を止め、chief の worktree を `main` に戻して `make install-chief` で道具を入れ直し、`/chief` を起動し直す（ガイドの 2 章）。
-   要件書 2.3 のスコープ外の、モデル名の短縮と経過時間の列は、別の issue で扱う。

## 変更履歴

- 2026-11-19 承認（isseis、#1000）
- 2026-11-19 セッション中の再承認：使える幅を、タブを分割していれば pane の幅 − 4 にし、parsePaneLayout が splits を読むステップとテストを足した（isseis、#未記入）
