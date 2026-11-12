# 実装計画書：chief が workspace の状況を示し、人の判断を待つものを選択式で示し、マージの後の次のコマンドを自動で送る

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-12 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `88618e43` で確かめた。

用語（chief・chief の workspace・タスクの workspace・エージェント・入力を受け付ける状態・自動送信・答えの中継・chief の待ち受け・実装計画書の PR・プランの PR・chief のセッション・道具・`/chief` の定義・送り先の表・状態のディレクトリ・周・観測の記録・画面の読み取り・照合値・送信の記録・Claude Code の transcript）は、要件書 3 章・6 章と設計書の冒頭と同じ意味で使う。本計画は、次の用語を加える。

-   **入力の型のパッケージ:** pane へ送る入力を表す型だけを持つ葉のパッケージ `internal/chief/paneinput`（§1.3 の決定 5）。
-   **採取用の workspace:** 画面のサンプルを採るために人が用意する、使い捨ての Herdr の workspace（§1.3 の決定 1）。タスクの作業には使わない。
-   **実地の確かめ:** 本物の Herdr とエージェントで行う手での確かめ（設計書 7.3）。結果を §7 の実行の記録に書く。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、Go の道具 `chief`（`cmd/chief`・`internal/chief`）、`/chief` の定義、`_context.md` の送り先の表、PR マーカーの `**自動送信**` の欄を作り、要件書の F-001〜F-011 を満たす。

### 1.2. 実装の原則

-   **読む部品から順に作る。** 画面の読み取り → ほかの読み取り → 状態の区分 → 送信 → 中継 → 待ち受けと自動送信 → 定義と文書の順に PR を分ける（設計書 8 章）。道具は PR-7 で `make install-chief` と `/chief` の定義が入るまで使われないので、途中の PR のマージで入力が送られることはない。
-   **送る入力は入力の型のパッケージの型でだけ作る。** Herdr へ書く部品は、その型だけを引数に取り、文字列を取らない（設計書 3.1.3、§1.3 の決定 5）。
-   **外部のコマンドは fake で置き換える。** ユニットテストは `internal/cmdrunner/testutil` の `Fake`（`internal/cmdrunner/testutil/mocks.go:34`）で herdr・git・gh を置き換え、本物を呼ばない。本物の Herdr を使うのは実地の確かめ（PR-1・PR-7）だけである。
-   **時刻と待ちは差し込む。** 道具は時刻と待ちを差し込める形で持ち、テストは実時間で待たない（ADR-0038 の race のない時間の扱い）。
-   **ADR の決定の中のずれの記述は、ずれを解いた PR で直す。** ADR-0042 は PR-2、ADR-0014 は PR-7 で消す。ADR-0046 は、PR-2・PR-6 で残るずれを述べる形に書き換え、PR-7 で消す。
-   **各テストは壊して失敗を確かめる。** 確かめた結果を commit のメッセージに書く（CLAUDE.md の「Testing Strategy」）。

### 1.3. 計画の際に決めたこと（2026-11-12）

1.  **画面のサンプルは、PR-1 の `/runplan` のセッションが人と一緒に採る。** 人が採取用の workspace を用意し、エージェントが `AskUserQuestion` で状態を作るよう頼み、`herdr agent read` で読んで、採った人を特定するものを置き換え、エージェントの版を記録する。理由: どちらの方法でも状態を作るのに人の手が要る。エージェントが採れば、置き換えと版の記録を毎回同じ手順で行える。ADR: なし（採取の手順はこのタスクの中で閉じるため）。
2.  **PR 番号の検証は、`internal/chief` が `fixprguard.ParsePRNumber`（`internal/fixprguard/types.go:132-140`）をそのまま使う。** 理由: 規則を 1 か所に保ったまま、信頼の境界の道具である `fixprguard` を変えずに済む（変えると再インストールが要る）。ADR: なし。
3.  **PR を 7 つに分け、設計書 8 章の 1〜7 に 1 つずつ対応させる。** 送信・中継・自動送信は、それぞれ信頼の境界なので、別の PR に分けてレビューの焦点を保つ。8 章の 1（採取）は、採ったサンプルを確かめるテストが要るので、2 のうち画面の読み取りと文字の無害化を PR-1 に入れる。8 章の 6 の通知の記録は、3 の並べ方（待っている時間）の前提なので PR-3 に入れる（8 章は「目安の順序」とする）。ADR: なし。
4.  **待ち受けと自動送信の時間の値は次のとおりにする。** `wait` は 3 秒ごとに `herdr agent list` を読み、既定の時間切れを 10 分にする。観測の記録を自動送信の条件に数えるのは、その周と今の周の間が 15 分より短いときだけにする。持ち越しの観測は、最初に `MERGED` を見てから 10 分まで数える（設計書 3.7）。理由: `/runpr` はマージの後も数分 pane を動かすことがあり、窓に余裕が要る。ADR: なし（定数の値）。
5.  **入力の型を葉のパッケージ `internal/chief/paneinput` に置く。** 設計書 3.1.3 の `CommandKind`・`Command`・`RouteKey`・`AgentKind`・`ClaudeModel`・`OpenCodeModel`・`Route`、`TaskID`・`IssueNumber`、リセットとモデルの切り替えの入力、中継のキー入力を、unexported の欄とコンストラクタで持つ。`internal/chief` は composite literal で作れないので、コンストラクタが作る唯一の経路になる。理由: 送る入力の型で振る舞いを分けるので、作る経路をコンパイラに守らせる（`mkplan.md` step 5 の具体型の作り方の確かめ）。ADR: なし（パッケージの境界はこのタスクの中で閉じるため。設計書 3.1.2 が計画に委ねた）。
6.  **`internal/chief` は 1 つのパッケージとし、部品をファイルで分ける。** 画面の読み取りは `screen` で始まるファイル（`screen.go`・`screen_claude.go`・`screen_opencode.go` など）だけに置き、CLAUDE.md の「Declare, don't infer」の例外の範囲をこのファイル名で書く（設計書 5.2）。文字の無害化は `sanitize.go` に置く。ADR: なし。
7.  **外部のコマンドの時間の上限と出力の上限を、名前のある定数で持つ。** herdr は 10 秒、`git fetch` は 120 秒、そのほかの git と gh は 30 秒とする。コマンドを送った後に実行中になるのを待つのは 15 秒、Claude Code の transcript の切り替えの記録は 500 ミリ秒の間隔で 10 回まで読み直す（設計書 3.1.7）。PR の詳細の控えは 14 日より古いものを `start` が消す（設計書 3.5）。理由: どれも実地の確かめで観測した遅れに余裕を持たせた値で、変えるときは定数 1 か所を直す。ADR: なし。
8.  **`--protocol` の番号は 1 から始める。** `internal/doctestutil/protocol.go:17-22` の道具の一覧に `chief` を足す。足さないと、`chief` の呼び出しの `--protocol` が `fixprguard` のガードに数えられる（`internal/doctestutil/protocol.go:49-52`）。ADR: なし。
9.  **`.claude/settings.json` で拒む herdr の呼び出しは、`herdr agent prompt`・`herdr agent send-keys`・`herdr agent start`・`herdr pane send-text`・`herdr pane send-keys`・`herdr pane run` とする。** どれも Herdr 0.9.1 の `herdr agent --help`・`herdr pane --help` が挙げる、pane へ入力を送るか pane でコマンドを走らせる操作である。`herdr api` は `snapshot`・`schema` だけで読み取りなので拒まない。パスで書いた呼び出しなど、規則の外の書き方は残る危険とする（設計書 5.2 の残る危険のまとめ）。ADR: なし（設計書 3.9 の範囲を具体にしただけである）。
10. **gh は `GH_REPO` と `GH_HOST` を除いた環境で走らせる。** gh がタスクの worktree の remote から PR を引くようにし、環境変数で別のリポジトリの PR を読んで自動送信の引き金にしないためである。`fixprguard` がこの 2 つを拒むのと同じ考え方である（`internal/config/envaccess_test.go:70-72` の注記）。終了コードの契約（設計書 3.1.1）は変えない。ADR: なし。
11. **workspace の名前は `herdr workspace list` の `label` から取る。** Herdr 0.9.1 の `herdr agent list` の出力は `label` を持たず、`herdr workspace list` が持つ（2026-11-12 に両方の出力で確かめた）。ADR: なし。
12. **gh の配列の出力のために `strictjson.ParseArray` を足す。** `gh pr list --json` と `gh pr checks --json` は配列を返す見込みで、今の `strictjson` はオブジェクトだけを読む（`internal/strictjson/strictjson.go:70-87`）。配列であることは PR-1 の採取で gh の実物の出力で確かめ、違えば PR-2 の前に計画を直す。ADR: なし（ADR-0020 の部品の中の追加である）。
13. **状態のディレクトリでは、workspace ごとの記録と排他を、worktree のパスの SHA-256 の 16 進を名前にした下のディレクトリに置く。** `internal/cachelock` はディレクトリごとに決まった名前のファイルをロックする（`internal/cachelock/cachelock.go:21,73`）ので、workspace ごとにディレクトリを分ける。ロックを引き継ぐときの排他は、状態のディレクトリの直下に `cachelock.Acquire` を取る。ディレクトリは先に `privatefile.MkdirBelow` で作り、`cachelock.Acquire` の `MkdirAll` が作る経路を使わない。ADR: なし。
14. **セッションの振る舞いだけで実現する部分は、`/chief` の定義を読む `manual` と実地の確かめで確かめる。** AC-05・AC-13 の「何もしない」・AC-14・AC-16・AC-22 の問い・AC-24 のバックグラウンド・AC-58 の通知の送信・AC-60 の `wait` の失敗の扱いは、`/chief` の定義の手順の文だけで実現する。エージェントが文に従うかはテストで確かめられず、文の語を探すテストは置かない（ADR-0016、0081 の計画書 §1.2 と同じ扱い）。道具が出す材料（並べた一覧、送り先、排他など）は `test` で確かめる。ADR: なし。
15. **`mkreq.md` は変えない。** 確かめた結果、`mkreq.md` は「PR 構成」の節の行を書かず、`_lib/simple-process.md:162-165` と `mkplan.md:175,205-209` がそれを書く（設計書 2.2 が確かめを計画に委ねた）。ADR: なし。
16. **PR-7 は、試作を退かしてから `/chief` の定義を書く。** 試作の `.claude/commands/chief.md` と `.claude/chief/` は、この worktree にあり、git の共通ディレクトリの `info/exclude` で除いている（2026-11-12 に確かめた）。除いたままだと、同じパスに書く新しい定義を `git add` が拾わない。ADR: なし。

本計画の決定に、主要なもの（CLAUDE.md の「Design decisions (ADR)」の基準）はない。どれもこのタスクの中で閉じる。

### 1.4. 既存コード調査結果

| 対象 | 今あるもの | このタスクで行うこと |
|---|---|---|
| `internal/cmdrunner` | `Runner`・`InputRunner`・`Options`（`Dir`・`Env`・`MaxOutput` など、`runner.go:25-60`）、`GitEnv`（`git.go:36`）、`testutil.Fake`（`testutil/mocks.go:34-90`） | そのまま使う。時間の上限は `context` で持つ |
| `internal/strictjson` | `ParseObject` と値の読み取り（`strictjson.go:72-330`）。最上位の配列は読めない | `ParseArray` を足す（決定 12）。`package_reference.md` の利用者の一覧に `internal/chief` を足す（`TestPackageReferenceStrictjsonUsers`、`internal/pipeline/strictjson_users_test.go:657`） |
| `internal/privatefile` | `Read`・`Create`（`link(2)` で置き、すでにあれば作らない）・`MkdirBelow`（`privatefile.go:226,314,402`）。消す操作はない | `Remove` を足す（設計書 3.5） |
| `internal/cachelock` | `Acquire(dir)` が `dir` の `.yt2column.lock` を待たずに flock する（`cachelock.go:21,73-100`） | workspace ごとと、ロックの引き継ぎの排他に使う（決定 13） |
| `internal/fixprguard` | `PRNumber`・`ParsePRNumber`（`types.go:126-140`）。0 と先頭の 0 を拒む（`types.go:158-170`） | `internal/chief/paneinput` から使う（決定 2）。変えない |
| `internal/doctestutil` | `ProtocolFlags`・`ToolFixprguard`・`ToolCodexreview`（`protocol.go:17-80`） | `ToolChief` を足す（決定 8） |
| `cmd/codexreview/docs_test.go` | `TestSettingsAllowsTool`（80 行）、`TestDocumentedProtocolMatches`（158 行）、`TestInstallTargetInMakefile`（655 行） | `cmd/chief/docs_test.go` で同じ形のガードを作る。共通にできる部分は `doctestutil` の今の関数を使う |
| `cmd/yt2column/test_helpers_bash.go` | `scriptBashes`（コマンドの文書の bash のスクリプトを走らせる） | PR マーカーの読み方の比較のテストで、bash を探す考え方を参考にする。`opus-review-decision.sh` は bash 4 以上を要する（`.claude/commands/_lib/opus-review-decision.sh:49`）ので、PATH の bash だけを使い、4 より古ければ失敗にする |
| `internal/config/envaccess_test.go` | 環境を読んでよい場所の一覧 `allowedEnvRefs`（65-80 行） | `cmd/chief/main.go` の行を足す（PR-3） |
| `internal/pipeline/pipeline_test.go` | `TestPackageReferenceListsPackages`（629 行）が、非テストのソースを持つパッケージすべての行を求める | パッケージを作る PR で、`package_reference.md` に行を足す（`internal/chief` は PR-1、`internal/chief/paneinput` は PR-2、`cmd/chief` は PR-3） |
| `.claude/commands/_context.md` | 「PR marker conventions」の第 5 部の表（108-117 行）が値とモデルの対応を持つ。欄の一覧（62 行） | 送り先の表の節を足し、第 5 部のモデルの列を置き換える（PR-2）。欄の一覧と読み方に `**自動送信**` を足す（PR-6） |
| `runplan.md`・`runpr.md` | モデルの案内（`runplan.md:20,60`、`runpr.md:749,763,1059`。§3.3 の検索の結果） | 送り先の表を名指す形にする（PR-2） |
| `mkplan2.md`・`mkplan.md`・`_lib/simple-process.md` | PR マーカーの形（`mkplan2.md:59-77`）、値の決め方（`mkplan2.md:41-50`）、単一フェーズの「PR 構成」の 2 行（`mkplan.md:175,205-209`、`_lib/simple-process.md:162-165`） | `**自動送信**` を足す（PR-6） |
| `.claude/commands/_lib/opus-review-decision.sh` | `origin/main` の `_context.md` の「PR marker conventions」の節の 2 つのセルの行を旧い値の表として読む（134-150 行） | 変えない。送り先の表は節の外に置き、`**自動送信**` の説明に 2 つのセルの表を置かない |
| `.claude/settings.json` | `permissions.allow` に 2 行だけ（`coderabbit review`・`codexreview`） | `chief` を許し、決定 9 の呼び出しを `permissions.deny` で拒む（PR-7） |
| ADR | ADR-0014 の 19 行、ADR-0042 の 21 行、ADR-0046 の 40 行に、0088 の実装で解くずれの記述がある | §1.2 のとおり、解いた PR で直す |
| 試作 | この worktree の `.claude/commands/chief.md` と `.claude/chief/`（`hold.tsv` に `0084_wasm_pipeline_poc` の `PR-1` の行）。リポジトリには入っていない | PR-7 で移行する（決定 16） |
| 画面の読み取り、Herdr の読み書き、状態の区分、送信、中継、待ち受け、記録 | ない | 新しく作る。今ある実装で代わりになるものはない |

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 画面のサンプルと画面の読み取り（設計書 8 章の 1、2 のうち画面の読み取りと文字の無害化）

**変更するファイル:** `internal/chief/testdata/screens/`（新規）、`internal/chief/testdata/README.md`（新規）、`internal/chief/doc.go`・`sanitize.go`・`screen*.go` とそのテスト（新規）、`docs/dev/conventions/package_reference.md`、本計画

-   [ ] **ステップ 1-1**: 採取の準備をする。人に採取用の workspace（Claude Code と OpenCode の pane を 1 つずつ）を用意してもらい、Claude Code・OpenCode・Herdr の版を記録する（設計書 3.4.2）。`herdr agent read` の `--source`（`visible`・`recent` など）と行数のうち、OpenCode のマージの確認の選択肢が見えるものを確かめて決める（設計書 1.2 の観測）。
-   [ ] **ステップ 1-2**: 画面と出力を採る。設計書 3.4.1 の表のすべての行と 8 章の 1 が挙げる画面について、人に状態を作ってもらい、読む。あわせて、`herdr agent list`・`get`・`workspace list` の出力、Claude Code の transcript の置き場所の規則とモデルを切り替えた記録の行、`/clear` の前後の `agent_session`、gh の `pr list`・`pr view`・`pr checks` の `--json` の出力（このリポジトリの PR を読む読み取り専用の操作で、人の承認を得てから行う）を採る。
-   [ ] **ステップ 1-3**: 採ったものから、採った人を特定するもの（利用者の名前、ホームのパス、ホスト名、セッション ID）を置き換える（security.md §7、ADR-0032）。置き換えの規則、版、採った日を `testdata/README.md` に書く。
-   [ ] **ステップ 1-4**: 選択肢を選ぶキー入力をエージェントごとに確かめる（設計書 3.4.2）。採取用の workspace の pane に実際に送り、今の選択の位置に依らずに同じ選択肢が選ばれることを確かめる。確かめられないエージェントは、中継の選択肢を示さないものとして記録する。
-   [ ] **ステップ 1-5**: `sanitize.go` に、文字の無害化と読む量の上限を書く（設計書 3.4.3）。テストは、置き換える文字の種類ごとと上限の境界で、置き換えと切ったことの印を確かめる。
-   [ ] **ステップ 1-6**: `screen*.go` に画面の読み取りを書く（設計書 3.4.1・3.4.2）。返す型の列挙のゼロ値を「合わない」にし、サンプルで確かめた形だけを受け付ける。`/runpr` の報告の NEXT は、コマンドの種類と `TaskID` の型で返す（`TaskID` は PR-2 で入力の型のパッケージに移すまで、このファイルの非公開の型で持つ）。テストは、サンプルごとの読み取りと、合わない画面（一部だけ合う、2 つの形に合う、上限で切った）が「合わない」になることを確かめる。
-   [ ] **ステップ 1-7**: `package_reference.md` に `internal/chief` の行を足す。
-   [ ] **ステップ 1-8**: §7 の「PR-1 の実行の記録」に、版、`herdr agent read` の引数、画面の行数（200 行で足りたか。足りなければ値と理由）、キー入力、transcript の規則と行の形、gh の出力の形（配列かどうか。決定 12）を書く。設計書と違う事実が出たら、設計書の誤りとして人に示し、計画と設計書を直してから先へ進む。

**完了の条件:** 設計書 3.4.1 の表の各行のサンプルがあり、テストが通る。§7 に PR-1 の実行の記録がある。`make test`・`make lint`・`make deadcode` が通る。

### PR-1 作成ポイント: screen samples and screen reading

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8

**推奨タイトル**: `feat(0088): 画面のサンプルを採り、chief の画面の読み取りと文字の無害化を作る`

**レビュー観点**: サンプルに採った人を特定するものが残っていないか / 合わない画面を「合わない」にするか（広く解釈していないか） / キー入力が選択の位置に依らないか / 無害化の文字の範囲

**実装モデル要件**: opus

**判定理由**: ステップ 1-2・1-4 が、外部の画面の形とキー入力を本物の Herdr で確かめる実現性の確かめ（Phase 0 型の調査）である。Sonnet で進めると、形の見落としや選択の位置に依るキー入力を確かめたものとして記録しやすい。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 入力の型、Herdr・タスク・送り先の表の読み取り（設計書 8 章の 2）

**変更するファイル:** `internal/chief/paneinput/`（新規）、`internal/strictjson/strictjson.go`、`internal/chief/herdr.go`・`task.go`・`routing.go` とそのテスト（新規）、`.claude/commands/_context.md`・`runplan.md`・`runpr.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/adr/0042-implementation-model-tiers.md`・`0046-chief-of-staff.md`

-   [ ] **ステップ 2-1**: `internal/chief/paneinput` を作る（決定 5、設計書 3.1.3）。`PRNumber` は `fixprguard.PRNumber` を使う（決定 2）。ステップ 1-6 の非公開の `TaskID` をこの型に置き換える。テストは、各コンストラクタが不正な値を拒むことと、ゼロ値が送れる入力にならないことを確かめる。`package_reference.md` に行を足す。
-   [ ] **ステップ 2-2**: `strictjson.ParseArray` を足す（決定 12）。`ParseObject` と同じ検査と要素の数の上限を持たせ、テストで確かめる。`package_reference.md` の `strictjson` の利用者の一覧に `internal/chief` を足す。
-   [ ] **ステップ 2-3**: `herdr.go` に Herdr の読み取りを書く（設計書 3.1.2）。`herdr agent list`・`get`・`read`・`workspace list` の出力を、`AgentStatus`（ゼロ値は読めない）、エージェントの種類、`agent_session`、作業ディレクトリ、`state_change_seq`、workspace の名前（決定 11）の型に読む。テストは、ステップ 1-2 の実物の出力と、知らない状態の値・欠けた項目・形の崩れた JSON を読めないとすることを確かめる。
-   [ ] **ステップ 2-4**: `task.go` にタスクの読み取りを書く（設計書 3.1.4 のタスクの決め方、`_context.md` の「PR marker conventions」の第 2〜4 部）。ブランチ名からのタスク、`origin/main` の文書の Status と Process（requirements_process.md の「Cell text」）、`### PR-N 作成ポイント` と「PR 構成」の節の値（コードブロックの中を読まない）、旧い値の表、`PR を作成した`・`PR がマージされた` の記録、次に作るプランの PR、`**自動送信**` の欄（設計書 3.8。欄がなければ `auto`、2 つ以上か値が違えば読めない）を読む。テストは、各読み取りの場合と、読めない場合を確かめる。
-   [ ] **ステップ 2-5**: PR マーカーの読み方を `opus-review-decision.sh` と比べるテストを書く（設計書 3.8、7.1）。一時的な git のリポジトリに同じ計画書のサンプルを置き、道具の読み取りと、スクリプトの出力（`RUN`・`NONE value-sonnet`・`NONE value-absent`）が対応することを確かめる。スクリプトの出力が区別しない読み取り（`opus`・`sonnet+opus-review`・読めない値は Opus の記録がなければどれも `RUN`）は、その対応で比べる。bash は PATH のものを使い、版が 4 より古ければ失敗にする。
-   [ ] **ステップ 2-6**: `_context.md` に「Command routing」の節を「PR marker conventions」の外に足し、設計書 3.3 の表を置く。第 5 部の表の「Model to start the session with」の列を、送り先の表の行を名指す形にする。`runplan.md:20,60` と `runpr.md:749,763,1059` のモデルの案内を、送り先の表を読む形にする（§3.3 の検索の結果のすべての箇所）。
-   [ ] **ステップ 2-7**: `routing.go` に送り先の表の読み取りを書く（設計書 3.3）。`origin/main` の `_context.md` を `git show` で読み、節・列・行の数と形・モデルの集合を検証して、`RouteKey` から `paneinput.Route` を引く。テストは、リポジトリの今の `_context.md` が読めること、形の崩れた表と集合の外のモデルを拒むこと、`sonnet+opus-review` の行を OpenCode・`DeepSeek V4.1 Flash` に変えた写しで、その `RouteKey` の送り先だけが変わること（AC-39）を確かめる。
-   [ ] **ステップ 2-8**: ADR-0042 の 21 行のずれの記述のうち、`_context.md`・`runplan.md`・`runpr.md` に反映していないという文を消す。値の名前の見直し（#760）の文は残す。ADR-0046 の 40 行のずれの記述を、このマージの後に残るずれ（道具、コマンドの定義、CLAUDE.md、`**自動送信**` の欄）を述べる形に書き換える。どちらも commit のメッセージに訂正の行を書く。

**完了の条件:** 設計書 3.1.2 の「Herdr の読み書き」の読み取りの部分、「タスクの読み取り」「送り先の表の読み取り」、3.3 の項目がそろっている。`make test`・`make lint`・`make deadcode` が通る。

### PR-2 作成ポイント: input types, readers, and the routing table

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8

**推奨タイトル**: `feat(0088): chief の入力の型と読み取りの部品を作り、送り先の表を _context.md に置く`

**レビュー観点**: 入力の型がコンストラクタでしか作れないか / PR マーカーの読み方が opus-review-decision.sh と食い違わないか / 送り先の表が読めないときに送らない側に倒れるか / runplan.md・runpr.md の案内が送り先の表と一致するか

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 2-4・2-5 の PR マーカーの読み取りは、コードブロックと旧い値の表を bash の実装と同じ規則で読む必要があり、2-7 は送るモデルを決める。Sonnet では、フェンスの扱いや表の形の検証の細部が食い違いやすい。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 状態の区分、`snapshot`、`start` とロック、`cmd/chief`（設計書 8 章の 3）

**変更するファイル:** `internal/privatefile/privatefile.go`、`internal/chief/records.go`・`workspace.go`・`state.go`・`pending.go`・`network_test.go` など（新規）、`internal/chief/test_helpers.go`（新規、`//go:build test`）、`cmd/chief/main.go`・`main_test.go`（新規）、`internal/config/envaccess_test.go`、`docs/dev/conventions/package_reference.md`

-   [ ] **ステップ 3-1**: `privatefile.Remove` を足す（設計書 3.5 の片付け）。決まった形の通常のファイルだけを消し、シンボリックリンクと持ち主の違うファイルを拒む。テストで、拒む場合のそれぞれを確かめる。
-   [ ] **ステップ 3-2**: `records.go` に、状態のディレクトリ（設計書 3.5 の場所、決定 13）、記録の作成（`privatefile.Create`）と読み取り（`privatefile.Read`）、chief のロックと引き継ぎ、通知の記録を書く。周・観測・送信などの記録は、それを使うフェーズで足す。テストは、同じ名前の記録を 2 回作れないこと、読めない記録を読めないとすること、生きている持ち主のロックで `start` が拒むこと、引き継ぐ 2 つの `start` が同時に走ったとき一方だけがロックを持つことを確かめる。
-   [ ] **ステップ 3-3**: `workspace.go` に、chief の workspace とタスクの workspace の見分け（設計書 3.1.4、git の共通ディレクトリで比べる）、worktree の git の状態の読み取り、ブランチの開いている PR の読み取り（gh は決定 10 の環境）を書く。
-   [ ] **ステップ 3-4**: `state.go` に、状態の区分、行の中身と並べ方、次のコマンド、送り先の表が読めないときの扱い、AC-40 の引き継ぎの提案を書く（設計書 3.1.4）。テストは、状態ごと、要確認の理由ごと、次のコマンドの規則の行ごと（簡易な進め方の 3 文書のマージの後を含む）、実装モデル要件が読めないかないときに `/runplan` を示さないこと、AC-40 の条件を 1 つずつ外した場合を確かめる。
-   [ ] **ステップ 3-5**: `pending.go` に、人の判断を待つものの並べ方と新しい通知を書く（設計書 3.1.5）。テストは、種類の順、待っている時間の順、同じ項目をセッションの中で 2 回通知しないこと、記録を作れないときの時刻の扱いを確かめる。
-   [ ] **ステップ 3-6**: `cmd/chief/main.go` に、サブコマンドの解析、`--protocol`（決定 8）、終了コード（設計書 3.1.1）、出力の JSON（3.1.8）、`start` と `snapshot` を書く。環境（`HERDR_PANE_ID`・`XDG_STATE_HOME`・`HOME`・`CLAUDE_CONFIG_DIR` と、子プロセスに渡す環境）は `main` でだけ読み、`internal/config/envaccess_test.go` の一覧に足す。テストは、Herdr の外で起動したとき何も読まずに 3 で終わること、拒む理由ごとの終了コード、`--protocol` の食い違い、使い方の誤りを確かめる。`package_reference.md` に `cmd/chief` の行を足す。
-   [ ] **ステップ 3-7**: `network_test.go` に、道具が走らせる gh と git の呼び出しが読み取りの操作の一覧に入っていることを確かめるテストを書く（AC-61）。`snapshot` の経路で確かめ、PR-4・PR-6 で `detail`・`send`・`tick` の経路を足す。`snapshot` が `herdr agent prompt`・`send-keys` と `gh pr checks` を呼ばないことも確かめる（AC-04・AC-15）。

**完了の条件:** `chief start` と `chief snapshot` が fake の外部コマンドで動き、設計書 3.1.4・3.1.5・3.5 のロックと通知の記録がそろっている。`make test`・`make lint`・`make deadcode` が通る。

### PR-3 作成ポイント: state classification, snapshot, and the lock

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7

**推奨タイトル**: `feat(0088): chief の状態の区分と snapshot、start とロックを作る`

**レビュー観点**: 読めないものが「要確認」になり、推し量って別の状態にしないか / ロックの引き継ぎで 2 つの chief が同時に持たないか / 次のコマンドの規則が runpr.md の定義と一致するか / 環境を読む場所が main に限られているか

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 3-2 のロックの引き継ぎは並行して走る 2 つのプロセスの排他で、3-4 は 6 つの状態と次のコマンドの規則を持つ状態機械である。Sonnet では、引き継ぎで確かめ直す順序や、どれにも当たらない場合の扱いを誤りやすい。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 送信、リセット、送信の停止と `release`、PR の詳細と `detail`（設計書 8 章の 4）

**変更するファイル:** `internal/chief/herdr.go`、`internal/chief/send.go`・`transcript.go`・`reset.go`・`detail.go`・`records.go` とそのテスト、`internal/chief/network_test.go`、`cmd/chief/main.go`・`main_test.go`

-   [ ] **ステップ 4-1**: `herdr.go` に、`herdr agent prompt`・`send-keys` で書く部品を足す。引数は入力の型のパッケージの型だけを取り、chief の workspace の pane を拒む（AC-31）。テストで、chief の workspace の pane に何も送らないことを確かめる。
-   [ ] **ステップ 4-2**: `send.go` に、送り先の pane の決定と、入力ごとの確かめ（設計書 3.1.3 の表）を書く。OpenCode のモデルは語ごとに比べる。`transcript.go` に、Claude Code の transcript の場所の決定と、モデルを切り替えた記録の確かめ（設計書 3.1.7、ステップ 1-8 の規則と行の形、決定 7 の回数と間隔）を書く。テストは、送り先の種類の pane が 0 か 2 つ以上のとき送らないこと、最初の入力の前の確かめのそれぞれで何も送らないこと、入力の間で確かめが外れたとき残りを送らないこと、モデルの確かめのそれぞれを確かめる。
-   [ ] **ステップ 4-3**: `records.go` に、送信の記録（始めと終わり）、送信の停止、workspace ごとの排他、終わり方の分からない送信の扱い、`release` を足す（設計書 3.5）。送った後の失敗で送り直さず停止の記録を作ること、終わりの記録のない始めの記録が停止になること（入力の間で道具を止める fake で作る）、道具を作り直しても停止が残ること、`release` が停止を解くこと、ほかの送信の途中で `NotSentBusy` を返すことをテストする。`start` の片付けに送信の記録を足す。
-   [ ] **ステップ 4-4**: `detail.go` に PR の詳細を書く（設計書 3.1.6）。CI の結果をまとめずに返すこと、レビューの本文を出力に含めないこと、報告と PR の説明の出どころ、項目ごとの「取得できなかった」、「レビュー省略可」の判定、PR と head ごとの控え（`start` の片付けに 14 日より古い控えを足す）、`/runpr` の送り先の表示をテストする。
-   [ ] **ステップ 4-5**: `send` の引数の検証とコマンドの組み立て（設計書 3.1.1）、`/mkreq` の条件（3.1.4）、`runplan-takeover`、`/runpr` の後のリセット（3.1.3、`reset.go`）を書き、`cmd/chief` に `send`・`detail`・`release` を足す。テストは、引数の組み合わせの誤り、開いていない PR の番号、タスクのある workspace への `/mkreq`、リセットの候補の条件のそれぞれ（同じ pane、報告がない、`state_change_seq` が変わった、入力欄に文字がある）、リセットの前に控えを作ることを確かめる。
-   [ ] **ステップ 4-6**: 行の「実行中」の表示を、送信の記録から示す形にする（設計書 3.1.4 の行の中身）。`network_test.go` に `detail` と `send` の経路を足す。
-   [ ] **ステップ 4-7**: `ActionResult` の結果（送らなかった・送った後に失敗した・送った）を、送信とリセットの各経路でテストする（AC-62・AC-63）。

**完了の条件:** 設計書 3.1.3・3.1.6・3.1.7 と 3.5 の送信の記録・送信の停止・workspace ごとの排他がそろっている。`make test`・`make lint`・`make deadcode` が通る。

### PR-4 作成ポイント: sending, reset, stop, and detail

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7

**推奨タイトル**: `feat(0088): chief の送信とリセット、送信の停止、PR の詳細を作る`

**レビュー観点**: 送る入力が型の外から作れないか / 送った後の失敗と終わり方の分からない送信で送り直さないか / リセットの候補を画面で広げていないか / レビューの本文と制御文字が出力に出ないか

**実装モデル要件**: opus

**判定理由**: ステップ 4-1〜4-5 は、ほかのエージェントのセッションへ入力を送る信頼の境界（security-gate）である。Sonnet では、入力の間の確かめの抜けや、送った後の失敗を「送らなかった」と扱う誤りが起きやすい。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 答えの中継と `relay`（設計書 8 章の 5）

**変更するファイル:** `internal/chief/relay.go` とそのテスト、`internal/chief/state.go`、`internal/chief/paneinput/`、`cmd/chief/main.go`・`main_test.go`

-   [ ] **ステップ 5-1**: 照合値を作る部品を書く（設計書 3.6）。`snapshot` の出力に、中継できる質問の問いごとの文と選択肢（無害化したもの）と照合値を足し、中継しない質問には照合値を返さない。テストは、照合値が `state_change_seq`・セッション・問いの文・選択肢・選択の位置のどれが変わっても変わること、中継しない質問の種類ごとに照合値がないことを確かめる。
-   [ ] **ステップ 5-2**: `relay.go` に中継を書く（設計書 3.6）。排他、読み直し、照合値の比較、Claude Code の `blocked` の確かめ、すべての問いの答え、選択肢の範囲、除く選択肢（自分で書く選択肢、マージの確認の「NEXT をこのセッションで実行する」）、ステップ 1-4 で確かめたキー入力、送信の記録、送った後の失敗での停止を持つ。キー入力は入力の型のパッケージで、選択肢の番号からだけ作る。
-   [ ] **ステップ 5-3**: `cmd/chief` に `relay` を足す。テストは、照合値が違うとき・一部の問いだけに答えたとき・範囲の外の番号・除いた選択肢を選んだときに何も送らないこと、キー入力を送り始めた後の失敗で停止の記録を作ること、送った結果を返すことを確かめる。

**完了の条件:** 設計書 3.6 の項目がそろっている。`make test`・`make lint`・`make deadcode` が通る。

### PR-5 作成ポイント: answer relay

**対象ステップ**: 5-1 / 5-2 / 5-3

**推奨タイトル**: `feat(0088): chief の答えの中継を作る`

**レビュー観点**: 照合値が質問の変化をすべて拾うか / 人が選んでいない答えや選択肢の文を送らないか / 中継しない質問と除く選択肢が漏れていないか

**実装モデル要件**: opus

**判定理由**: ステップ 5-2 は、止まっているエージェントに人の答えを送る信頼の境界（security-gate）である。Sonnet では、照合値に入れる値の漏れや、選択の位置に依るキー入力を送る誤りが起きやすい。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: 待ち受けと自動送信、PR マーカーの `**自動送信**` の欄（設計書 8 章の 6）

**変更するファイル:** `internal/chief/wait.go`・`tick.go`・`autosend.go`・`records.go` とそのテスト、`internal/chief/network_test.go`、`cmd/chief/main.go`・`main_test.go`・`docs_test.go`（新規）、`.claude/commands/_context.md`・`mkplan2.md`・`mkplan.md`・`_lib/simple-process.md`、`docs/dev/adr/0046-chief-of-staff.md`

-   [ ] **ステップ 6-1**: `wait.go` に `wait` を書く（設計書 3.1.1、決定 4）。基準は不透明な値とし、タスクの workspace のエージェントの状態か `state_change_seq` が変わると戻り、chief の workspace の変化では戻らない。テストは、変化のある場合とない場合、`blocked` のまま変わらないエージェントで戻らないこと、時間切れ、herdr の失敗で失敗の終了コードになることを確かめる。
-   [ ] **ステップ 6-2**: `records.go` に、周の記録、観測の記録（持ち越しを含む）、自動送信の記録を足す（設計書 3.5）。`start` の片付けに、終わったセッションの周・観測の記録を足す。
-   [ ] **ステップ 6-3**: `autosend.go` に自動送信を書く（設計書 3.7）。AC-41 の条件の表、タスクと workspace の確かめ直し、報告の照合、ブランチのない状態の規則、順序（排他、読み直し、最初の確かめ、自動送信の記録、送信）、満たさないときの提案と持ち越し、自動送信の後のリセットを持つ。テストは、AC-41 の条件を 1 つずつ外した入力の表で送らないことと、設計書 7.1 の 3 項目めが挙げる場合（直前の周より前の観測、タスクの違い、ブランチのない状態で報告の PR の番号が違う、持ち越しの時間切れ、マージの直後の周で実行中だったときに次の周で送れる）を確かめる。
-   [ ] **ステップ 6-4**: `tick.go` に `tick` を書き、`cmd/chief` に `tick` と `wait` を足す。`network_test.go` に `tick` の経路を足す。
-   [ ] **ステップ 6-5**: `**自動送信**` の欄を文書に足す（設計書 3.8）。`_context.md` の「PR marker conventions」の欄の一覧と、読み方の規則（第 2〜3 部と同じ場所と行の形、欠けたら `auto`、2 つ以上か値が違えば読めない）、`mkplan2.md` の step 4（`hold` にする場合）・step 6 の形・step 7 の表の例、`mkplan.md` の step 8.5 と step 8 のチェックリストの単一フェーズの項目（`mkplan.md:175,205-209`）、`_lib/simple-process.md:162-165` を直す。「PR marker conventions」の節に、2 つのセルの表を置かない（`opus-review-decision.sh:134-150` が旧い値の表として読むため）。
-   [ ] **ステップ 6-6**: `cmd/chief/docs_test.go` に、`mkplan2.md` の step 6 の PR マーカーの形のコードブロックと、`mkplan.md` の step 8.5 が書かせる行の形を、道具の `**自動送信**` の読み取り（ステップ 2-4）で読めることを確かめるテストを書く（設計書 7.2 の 2 項目め）。
-   [ ] **ステップ 6-7**: ADR-0046 の 40 行のずれの記述を、このマージの後に残るずれ（コマンドの定義、CLAUDE.md、道具のインストール）を述べる形に書き換え、commit のメッセージに訂正の行を書く。

**完了の条件:** 設計書 3.1.1 の `wait`・`tick`、3.5 の周・観測・自動送信の記録、3.7・3.8 の項目がそろっている。`make test`・`make lint`・`make deadcode` が通る。

### PR-6 作成ポイント: wait, tick, and automatic sending

**対象ステップ**: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7

**推奨タイトル**: `feat(0088): chief の待ち受けとマージの後の自動送信を作り、PR マーカーに自動送信の欄を足す`

**レビュー観点**: 自動送信の引き金が画面に依らないか / 1 つのマージで 2 回送らないか（起動し直し、持ち越し、記録の後の確かめの失敗） / 見ていないマージを提案にするか / 欄の読み方が実装モデル要件と同じ場所の規則か

**実装モデル要件**: opus

**判定理由**: ステップ 6-3 は人に尋ねずに入力を送る判断（security-gate）で、排他と記録の順序を持つ並行の処理である。Sonnet では、排他を取った後の確かめ直しや、自動送信の記録を作る順序を誤り、2 回送るか見ていないマージで送る誤りが起きやすい。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 7: `/chief` の定義、設定、文書、移行、実地の確かめ（設計書 8 章の 7）

**変更するファイル:** `.claude/commands/chief.md`（新規）、`.claude/settings.json`、`internal/doctestutil/protocol.go`、`cmd/chief/docs_test.go`、`Makefile`、`CLAUDE.md`、`docs/dev/guides/chief_guide.md`（新規）、`docs/dev/adr/0014-deterministic-checks-in-go.md`・`0046-chief-of-staff.md`、本計画

-   [ ] **ステップ 7-1**: 試作を退かす（決定 16）。人の承認を得て、この worktree の試作の `.claude/commands/chief.md` と `.claude/chief/` をリポジトリの外へ移し、git の共通ディレクトリの `info/exclude` の試作の 3 行を消す。`.claude/chief/hold.tsv` の行（`0084_wasm_pipeline_poc` の `PR-1` の後）について、0084 の計画書にまだ止める必要があるかを人に確かめる。要るなら、0084 の計画書に `**自動送信**: hold` を足す PR を、ステップ 7-6 の実地の確かめの前に別に作る（設計書 8 章の 7）。
-   [ ] **ステップ 7-2**: `.claude/commands/chief.md` を書く（設計書 3.2）。`chief` の呼び出しは `--protocol 1` を書く。`internal/doctestutil/protocol.go` に `ToolChief` を足し、`cmd/chief/docs_test.go` に、`--protocol` の番号が道具と合うことと、定義の中の呼び出しが道具の引数の解析を通ることのガードを書く（設計書 7.2 の 1 項目め、`cmd/codexreview/docs_test.go:158,255` と同じ形）。
-   [ ] **ステップ 7-3**: `.claude/settings.json` に `Bash(chief *)` を `allow` に、決定 9 の呼び出しを `deny` に足す。`cmd/chief/docs_test.go` に、この 2 つを確かめるガードを書く（設計書 7.2 の 3 項目め）。
-   [ ] **ステップ 7-4**: `Makefile` に `install-chief` を足し、`cmd/chief/docs_test.go` に `cmd/codexreview/docs_test.go:655` と同じ形のガードを書く。`docs/dev/guides/chief_guide.md` を書く（設計書 2.2 の表の行）。`CLAUDE.md` の「Build Commands」「Claude Commands」「Declare, don't infer」の例外（範囲は決定 6 のファイル名とセッションの報告の要約）、「Tool Execution Safety」（設計書 3.9）、`/chief` の定義が手引きを参照するときは「Japanese guides an agent reads」の一覧を直す。
-   [ ] **ステップ 7-5**: ADR-0014 の 19 行のずれの記述を消し、道具の一覧とインストールの項目に `chief` を足す。ADR-0046 の 40 行のずれの記述を消す。どちらも commit のメッセージに訂正の行を書く。
-   [ ] **ステップ 7-6**: このブランチで `make install-chief` を行い、`/chief` を新しいセッションで動かして実地の確かめを行う（設計書 7.3）。自動送信、中継、モデルの確かめ（Claude Code と OpenCode）、リセット、ロック（2 つ目の chief が拒まれる）、送信の停止と `release` を、それぞれ 1 回以上、採取用の workspace に向けて行う。§1.3 の決定 14 が挙げるセッションの振る舞い（表の下の質問の要点と pane、示せる数を超えたときの挙げ方、まだ答えのない問い、詳細の後の問い、`/runpr` の送信をバックグラウンドで呼ぶこと、通知、`wait` の失敗）も確かめる。結果を §7 の「PR-7 の実行の記録」に書く。

**完了の条件:** 設計書 3.2・3.9 の項目、2.2 の表の `Makefile`・手引き・`CLAUDE.md` の行がそろっている。§7 に PR-7 の実行の記録がある。`make test`・`make lint`・`make deadcode` が通る。

### PR-7 作成ポイント: chief command, settings, and documents

**対象ステップ**: 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6

**推奨タイトル**: `feat(0088): /chief の定義と設定と手引きを足し、chief を実地で確かめる`

**レビュー観点**: `/chief` の定義が判定を持たず、`herdr` を直接呼ばないか / settings.json の拒否が入力を送る herdr の呼び出しを覆うか / CLAUDE.md の例外と承認の範囲が設計書のとおりか / 実地の確かめの記録

**実装モデル要件**: opus

**判定理由**: ステップ 7-6 は本物の Herdr のほかのエージェントへ入力を送る確かめ（外部の資源に触れる面）で、7-3 は信頼の境界のガードである。Sonnet では、確かめの範囲を落とすか、採取用の workspace の外の pane に送る誤りが起きやすい。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 状態の変化 |
|---|---|---|
| M1（PR-1） | 画面のサンプル、画面の読み取り、無害化 | 画面の形がテストで固定される |
| M2（PR-2） | 入力の型、読み取りの部品、送り先の表 | `runplan.md`・`runpr.md` の案内が送り先の表を示す |
| M3（PR-3） | `chief start`・`chief snapshot` | 道具が状況の表の材料を返す（まだ使われない） |
| M4（PR-4） | `send`・`detail`・`release`、リセット | 道具が人の選んだコマンドを送れる |
| M5（PR-5） | `relay` | 道具が答えを中継できる |
| M6（PR-6） | `wait`・`tick`、自動送信、`**自動送信**` の欄 | 新しい計画書が欄を持つ |
| M7（PR-7） | `/chief`、settings.json、手引き、CLAUDE.md | chief が使える |

順序は設計書 8 章のとおりである。違うのは §1.3 の決定 3 の 2 点（画面の読み取りを PR-1 に、通知の記録を PR-3 に入れる）だけである。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 | 画面のサンプル、画面の読み取り、文字の無害化 | opus |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8 | 入力の型、Herdr・タスク・送り先の表の読み取り、`_context.md` の送り先の表 | sonnet+opus-review |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 | 状態の区分、`snapshot`、`start` とロック、`cmd/chief` | sonnet+opus-review |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 | 送信、リセット、送信の停止、`detail` | opus |
| PR-5 | 5-1 / 5-2 / 5-3 | 答えの中継 | opus |
| PR-6 | 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 | 待ち受け、自動送信、`**自動送信**` の欄 | opus |
| PR-7 | 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6 | `/chief` の定義、設定、手引き、CLAUDE.md、移行、実地の確かめ | opus |

本計画の PR マーカーは `**自動送信**` の欄を持たない。欄は PR-6 で定まり、chief は PR-7 のマージまで使われないためである。欄がないマーカーは `auto` として扱われる（AC-50）。

### 3.3. 送り先の表へ置き換える箇所の検索

ステップ 2-6 が直す箇所を、コミット `88618e43` で次のとおり探した。

```text
$ rg -n 'Model to start|model each reading recommends|model that reading recommends|recommends none|model to start' .claude/commands docs/dev CLAUDE.md
.claude/commands/runplan.md:20:> through its table), and which model each reading recommends: start the
.claude/commands/runplan.md:60:   - the model that reading recommends (the same section's table), or that it recommends none;
.claude/commands/_context.md:109:     model to start a session with from this table, by naming it rather than
.claude/commands/_context.md:113:     | Reading | Model to start the session with | Opus review |
.claude/commands/runpr.md:749:   NEXT will open, and the model to start the new session with. That PR is
.claude/commands/runpr.md:763:   before NEXT, which model to start the new session with. Step 5 reuses the
.claude/commands/runpr.md:1059:  4. when NEXT is `/runplan`, the `実装モデル要件` and the model to start the
```

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** 道具の判定と副作用は、`internal/chief` と `cmd/chief` のユニットテストで確かめる。外部のコマンドは `cmdrunnertestutil.Fake` で、画面は `testdata/screens/` のサンプルで、時刻と待ちは差し込んだ fake で置き換える（§1.2）。設計書 7.1 の各項目を、§2 の各ステップのテストが受け持つ。
-   **テストの補助:** `internal/chief` の中だけで使う補助（Herdr の出力と画面の fake の組み立て、一時的な git のリポジトリ）は `internal/chief/test_helpers.go`（必要なら `test_helpers_<category>.go`）に `//go:build test` で置く（test_organization.md の Classification B）。ほかのパッケージから使う補助は作らない。
-   **境界の値:** 読む量の上限、`wait` と自動送信の時間の窓（決定 4）、transcript の読み直しの回数（決定 7）、片付けの日数は、境界の前後をテストする。
-   **文書のガード:** `cmd/chief/docs_test.go` に、`--protocol` の番号と呼び出しの解析（ステップ 7-2）、settings.json の許可と拒否（7-3）、`install-chief`（7-4）、PR マーカーの形の `**自動送信**`（6-6）を置く。「Tool Execution Safety」の承認と「Declare, don't infer」の例外の文は、PR のレビューで読む（ADR-0016、設計書 7.2）。
-   **既存のガード:** `TestPackageReferenceListsPackages`・`TestPackageReferenceStrictjsonUsers`・`internal/config/envaccess_test.go`・`TestADRRecords`・`TestInstructionDocPaths`・`fixprguard` と `codexreview` の `--protocol` のガードが、文書と構成の変更を確かめる（§1.4）。
-   **実地の確かめ:** PR-1 で画面とキー入力を、PR-7 で chief 全体を、本物の Herdr で確かめる（設計書 7.3）。
-   **後方互換:** 旧い値の実装モデル要件と、`**自動送信**` の欄のない PR マーカーを、ステップ 2-4・2-5 のテストで読む。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 採ったサンプルに採った人を特定するものが残る | ステップ 1-3 の置き換えの規則を `testdata/README.md` に書き、PR-1 のレビュー観点に入れる |
| エージェントの版が上がって画面の形が変わる | 合わない画面は「要確認」か「送らない」になる（設計書 3.4.2）。採り直しは新しい PR で行う |
| キー入力が選択の位置に依り、違う選択肢を選ぶ | ステップ 1-4 で実際に送って確かめ、確かめられないエージェントには中継を示さない |
| PR マーカーの読み方が `opus-review-decision.sh` と食い違う | ステップ 2-5 のテストで、同じサンプルを両方に読ませる |
| 送り先の表を節の中に置き、`opus-review-decision.sh` が旧い値の表として読む | 表を節の外に置く（ステップ 2-6）。`**自動送信**` の説明に 2 つのセルの表を置かない（6-5） |
| Claude Code の transcript の場所か行の形が変わる | 最初の入力の前の確かめで止まり、何も送らない（設計書 3.1.7） |
| 途中の PR の道具を誰かが手で走らせ、入力が送られる | PR-7 まで `install-chief` と `/chief` がない。途中で手で走らせても、送るのは型で決まった入力だけである |
| 試作の定義が、新しい定義と同じパスで除かれたまま残る | ステップ 7-1 で退かしてから書く（決定 16） |
| `hold.tsv` の止める行を移し忘れ、0084 の PR が自動送信される | ステップ 7-1 で人に確かめ、要るなら実地の確かめの前に 0084 の計画書を直す |
| 実地の確かめで、採取用の workspace の外の pane に送る | 採取用の workspace だけに向けて行う。ほかの workspace は、確かめの間エージェントを止めておくよう人に頼む |

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8）
-   [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 2-6 / 2-7 / 2-8）
-   [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7）
-   [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7）
-   [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3）
-   [ ] PR-6 マージ済み（対象ステップ: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7）
-   [ ] PR-7 マージ済み（対象ステップ: 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種類の意味は次のとおりである。

-   `test`: 下の表のテストで、振る舞いを確かめる。`make test` が走らせる。テストの名前は目安で、実装で名前を変えたときは、この表を同じ commit で直す。
-   `static`: ガードのテストで、文書と設定の値と構造を確かめる。`make test` が走らせる。
-   `manual`: PR のレビューで `/chief` の定義の該当箇所を読み、PR-7 の実地の確かめ（ステップ 7-6）で振る舞いを確かめる（§1.3 の決定 14）。

表のパスは、`internal/chief/` を `ic/`、`cmd/chief/` を `cc/` と略す。

| AC | 確かめ | 種類 | 実装 | PR |
|---|---|---|---|---|
| AC-01 | `ic/state_test.go::TestSnapshotExcludesChiefWorkspace` | test | 3-3・3-4 | PR-3 |
| AC-02 | `ic/state_test.go::TestRowContents`（実行中の行は送信の記録から） | test | 3-4・4-6 | PR-3、PR-4 |
| AC-03 | `ic/state_test.go::TestRowOrder` | test | 3-4 | PR-3 |
| AC-04 | `ic/network_test.go::TestSnapshotSkipsDetail`、`/chief` の定義 | test、manual | 3-7・7-2 | PR-3、PR-7 |
| AC-05 | `ic/state_test.go::TestQuestionSummaryInRow`、`/chief` の定義 | test、manual | 3-4・7-2 | PR-3、PR-7 |
| AC-06 | `ic/state_test.go::TestNeedsCheckReasons` | test | 3-4 | PR-3 |
| AC-07 | `cc/main_test.go::TestStartOutsideHerdr` | test | 3-6 | PR-3 |
| AC-08 | `ic/records_test.go::TestStartRefusesLiveOwner`、`ic/records_test.go::TestLockTakeoverRace` | test | 3-2 | PR-3 |
| AC-09 | `ic/screen_test.go::TestParseScreenSamples`、`ic/state_test.go::TestClassifyWorkspaceStates` | test | 1-6・3-4 | PR-1、PR-3 |
| AC-10 | 同上（OpenCode の質問のサンプル） | test | 1-6・3-4 | PR-1、PR-3 |
| AC-11 | 同上（マージの確認のサンプル）、`ic/state_test.go::TestMergeQuestionOffersNoRunpr` | test | 1-6・3-4 | PR-1、PR-3 |
| AC-12 | `ic/screen_test.go::TestParseScreenRejectsNonMatching`、`ic/state_test.go::TestNeedsCheckReasons` | test | 1-6・3-4 | PR-1、PR-3 |
| AC-13 | `ic/pending_test.go::TestPendingOrder`、`/chief` の定義（「何もしない」） | test、manual | 3-5・7-2 | PR-3、PR-7 |
| AC-14 | `/chief` の定義と実地の確かめ。並べた一覧は AC-13 の test | manual | 7-2・7-6 | PR-7 |
| AC-15 | `ic/network_test.go::TestSnapshotSkipsDetail`（`snapshot` が送らない）、`/chief` の定義 | test、manual | 3-7・7-2 | PR-3、PR-7 |
| AC-16 | `/chief` の定義と実地の確かめ | manual | 7-2・7-6 | PR-7 |
| AC-17 | `ic/detail_test.go::TestDetailItems`、`/chief` の定義（要約） | test、manual | 4-4・7-2 | PR-4、PR-7 |
| AC-18 | `ic/detail_test.go::TestDetailKeepsCheckBuckets` | test | 4-4 | PR-4 |
| AC-19 | `ic/detail_test.go::TestDetailReportSource` | test | 4-4 | PR-4 |
| AC-20 | `ic/detail_test.go::TestDetailOmitsReviewBody` | test | 4-4 | PR-4 |
| AC-21 | `ic/detail_test.go::TestDetailPartialFailure` | test | 4-4 | PR-4 |
| AC-22 | `ic/detail_test.go::TestDetailItems`（送り先の表示）、`/chief` の定義（問い） | test、manual | 4-4・7-2 | PR-4、PR-7 |
| AC-23 | `ic/detail_test.go::TestReviewSkippable` | test | 4-4 | PR-4 |
| AC-24 | `ic/send_test.go::TestWorkspaceBusy`、`/chief` の定義（バックグラウンド） | test、manual | 4-3・7-2 | PR-4、PR-7 |
| AC-25 | `ic/sanitize_test.go::TestSanitize`、`ic/relay_test.go::TestQuestionOutputHasAllTabs` | test | 1-5・5-1 | PR-1、PR-5 |
| AC-26 | `ic/screen_test.go::TestParseScreenRejectsNonMatching`、`ic/relay_test.go::TestNoCheckValueForUnrelayable` | test | 1-6・5-1 | PR-1、PR-5 |
| AC-27 | `ic/relay_test.go::TestRelayRequiresAllAnswers` | test | 5-2 | PR-5 |
| AC-28 | `ic/relay_test.go::TestRelayCheckValueMismatch` | test | 5-2 | PR-5 |
| AC-29 | `ic/paneinput/paneinput_test.go::TestRelayKeysFromNumbersOnly`、`ic/relay_test.go::TestRelaySendsChosenOption` | test | 2-1・5-2 | PR-2、PR-5 |
| AC-30 | `ic/relay_test.go::TestRelayExcludedOptions` | test | 5-2 | PR-5 |
| AC-31 | `ic/paneinput/paneinput_test.go::TestZeroValuesAreUnusable`、`ic/herdr_test.go::TestWriterRefusesChiefPane`、`cc/docs_test.go::TestSettingsDeniesPaneInput` | test、static | 2-1・4-1・7-3 | PR-2、PR-4、PR-7 |
| AC-32 | `ic/task_test.go::TestReadModelRequirement`、`ic/markers_parity_test.go::TestMarkerReadingMatchesDecisionScript` | test | 2-4・2-5 | PR-2 |
| AC-33 | `ic/state_test.go::TestUnreadableRequirementOffersNoRunplan` | test | 3-4 | PR-3 |
| AC-34 | `ic/send_test.go::TestClaudeModelSwitchCheck` | test | 4-2 | PR-4 |
| AC-35 | `ic/send_test.go::TestOpenCodeModelMatch` | test | 4-2 | PR-4 |
| AC-36 | `ic/send_test.go::TestSendChecksBeforeFirstInput` | test | 4-2 | PR-4 |
| AC-37 | `ic/send_test.go::TestSendFailureAfterInputStops`、`ic/records_test.go::TestUnfinishedSendBecomesStop`、`ic/records_test.go::TestStopSurvivesRestart`、`ic/records_test.go::TestReleaseClearsStop` | test | 4-2・4-3 | PR-4 |
| AC-38 | `ic/send_test.go::TestMkreqTarget` | test | 4-5 | PR-4 |
| AC-39 | `ic/routing_test.go::TestRoutingRowChangesOneRoute` | test | 2-7 | PR-2 |
| AC-40 | `ic/state_test.go::TestTakeoverProposal`、`ic/send_test.go::TestSendArgumentValidation`（`runplan-takeover`） | test | 3-4・4-5 | PR-3、PR-4 |
| AC-41 | `ic/autosend_test.go::TestAutoSendConditions` | test | 6-3 | PR-6 |
| AC-42 | `ic/autosend_test.go::TestAutoSendReportMismatch` | test | 6-3 | PR-6 |
| AC-43 | `ic/state_test.go::TestNextCommand`（簡易な進め方の 3 文書の後） | test | 3-4 | PR-3 |
| AC-44 | `ic/autosend_test.go::TestAutoSendOnce`（起動し直し、記録を読めないとき） | test | 6-2・6-3 | PR-6 |
| AC-45 | `ic/autosend_test.go::TestAutoSendConditions`（満たさなかった条件の提案） | test | 6-3 | PR-6 |
| AC-46 | `ic/autosend_test.go::TestAutoSendSkipsRunningNext` | test | 6-3 | PR-6 |
| AC-47 | `ic/autosend_test.go::TestAutoSendNeverMkreqOrRunpr`、`ic/state_test.go::TestClassifyWorkspaceStates`（タスク完了） | test | 3-4・6-3 | PR-3、PR-6 |
| AC-48 | `cc/docs_test.go::TestMarkerFormsCarryAutoSend` | static | 6-5・6-6 | PR-6 |
| AC-49 | `ic/autosend_test.go::TestAutoSendHold` | test | 6-3 | PR-6 |
| AC-50 | `ic/task_test.go::TestReadAutoSendField`（欄がない場合）、`ic/autosend_test.go::TestAutoSendConditions` | test | 2-4・6-3 | PR-2、PR-6 |
| AC-51 | `ic/task_test.go::TestReadAutoSendField`（2 つ以上、値が違う場合）、`ic/autosend_test.go::TestAutoSendConditions` | test | 2-4・6-3 | PR-2、PR-6 |
| AC-52 | `ic/reset_test.go::TestResetAfterRunpr` | test | 4-5 | PR-4 |
| AC-53 | `ic/autosend_test.go::TestResetAfterAutoSend` | test | 6-3 | PR-6 |
| AC-54 | `ic/reset_test.go::TestResetChecks` | test | 4-5 | PR-4 |
| AC-55 | `ic/reset_test.go::TestDetailSavedBeforeReset` | test | 4-4・4-5 | PR-4 |
| AC-56 | `ic/wait_test.go::TestWaitReturnsOnChange`、`ic/autosend_test.go::TestAutoSendConditions`、実地の確かめ | test、manual | 6-1・6-3・7-6 | PR-6、PR-7 |
| AC-57 | `ic/wait_test.go::TestWaitKeepsWaitingOnBlocked`、`ic/wait_test.go::TestWaitIgnoresChiefWorkspace` | test | 6-1 | PR-6 |
| AC-58 | `ic/pending_test.go::TestNoticesOncePerSession`、`/chief` の定義（通知の送信） | test、manual | 3-5・7-2 | PR-3、PR-7 |
| AC-59 | `ic/autosend_test.go::TestAutoSendUnobservedMerge` | test | 6-3 | PR-6 |
| AC-60 | `ic/state_test.go::TestNeedsCheckReasons`（Herdr の失敗）、`ic/wait_test.go::TestWaitFailure`、`/chief` の定義 | test、manual | 3-4・6-1・7-2 | PR-3、PR-6、PR-7 |
| AC-61 | `ic/network_test.go::TestNetworkCommandsAreReadOnly` | test | 3-7・4-6・6-4 | PR-3、PR-4、PR-6 |
| AC-62 | `ic/send_test.go::TestActionResults`、`ic/relay_test.go::TestRelaySendsChosenOption`、`ic/autosend_test.go::TestAutoSendConditions` | test | 4-7・5-3・6-3 | PR-4、PR-5、PR-6 |
| AC-63 | `ic/send_test.go::TestActionResults` | test | 4-7 | PR-4 |

### PR-1 の実行の記録

（ステップ 1-8 で書く）

### PR-7 の実行の記録

（ステップ 7-6 で書く）

## 8. 成功の基準 (Success Criteria)

-   **機能:** §7 の表のすべての行の確かめが通り、PR-7 の実地の確かめで設計書 7.3 の項目をそれぞれ 1 回以上確かめた。
-   **品質:** `make test`・`make lint`・`make deadcode` が各 PR で通る。各テストは壊して失敗を確かめ、commit のメッセージに書いた。
-   **セキュリティ:** 設計書 7.4 の項目（無害化、上限、照合値、型のゼロ値で送らない、chief の workspace に送らない）をテストで確かめ、settings.json の拒否をガードで確かめた。
-   **文書:** `/chief` の定義、手引き、CLAUDE.md、`_context.md`、`mkplan2.md`・`mkplan.md`・`_lib/simple-process.md`、`package_reference.md` が実装と一致し、ADR-0014・ADR-0042・ADR-0046 の決定の中のずれの記述が消えた。

## 9. 次のステップ (Next Steps)

-   PR-7 のマージの後、`make install-chief` を `main` から行い、chief の workspace で `/chief` を動かす。
-   試作から移る手順は手引き（`chief_guide.md`）に従う。
-   `sonnet+opus-review` の `/runplan` を OpenCode・DeepSeek V4.1 Flash に差し替えて比べること、#839（worktree と workspace の作成）、#840（`/runpr` の Opus への切り替え）、#760（値の名前の見直し）は、別のタスクで扱う。
