# 実装計画書：タスク番号と ADR 番号が並行して重複しないようにする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-17 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `8252ed17` で確かめた。設計書が確かめたコミット `b48db108` から、本計画が触れるファイルのうち変わったのは、`.claude/commands/runpr.md`・`docs/dev/adr/0014-deterministic-checks-in-go.md`・`docs/dev/adr/0051-number-reservation.md` の 3 つである（`git diff --stat b48db108 8252ed17` をこれらのパスに限って実行した結果）。設計書は、この 3 つの行番号を引いていない。そのため、設計書が引く行番号は今も合う。

用語は、要件書 6 章と設計書の冒頭と同じ意味で使う（番号の空間・番号を決める道具・番号の記録・欠番・`numreserve`・記録の tag・記録の ruleset・名前の部品・使用中の番号・基準の最大値・しきい値・止める理由）。本計画は、次の用語を加える。

-   **試しの tag:** REST の前提を確かめるために、記録の名前空間の外（`refs/tags/numreserve-probe/` の下）に作り、確かめた後に消す tag（§1.3 の決定 2）。番号の記録ではない。
-   **導入の確かめ:** 利用者が `main` から道具を入れ、番号の空間ごとに 1 回ずつ番号を決め、記録の ruleset を設定し、削除が拒まれることを確かめる手作業（ステップ 5-1）。
-   **番号の確かめの部品:** `internal/mergepr` が、PR の加える番号を確かめるために呼ぶ部品。本番では `numreserve` の `Tool`、テストでは fake である（§1.3 の決定 3）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書のとおりに、名前の部品 `internal/tasknaming`、番号を決める道具 `numreserve`、`mergepr` によるマージの前の確かめを作る。あわせて、`/mkreq` と ADR の規約を道具の呼び出しに切り替え、`TestADRRecords` が欠番を許すようにして、要件書の F-001〜F-007 を満たす。

### 1.2. 実装の原則

-   **設計書 8 章の依存の順に、7 つのフェーズで進める。** フェーズは 8 章の 1〜7 に 1 つずつ対応する。PR はフェーズ 1、フェーズ 2〜3、フェーズ 4〜5、フェーズ 6〜7 の 4 つにまとめる（§1.3 の決定 1）。
-   **道具を作る前に、前提を実地で確かめる。** REST の前提（設計書 7.3）が崩れたら、または手元か cloud のセッションでの取得や `git ls-remote` の結果が設計書の前提と違ったら、道具を作らずに止め、設計書を直す（ステップ 2-5）。
-   **切り替えは、道具と ruleset が使える状態になってから `main` に入れる。** 導入の確かめ（ステップ 5-1）が済むまで、PR-3 はマージしない（申し送りの H-08）。
-   **判定は `internal/numreserve` に置き、外部のコマンドはすべて fake の Runner で差し替えて確かめる。** テストは実の GitHub に tag を作らない（設計書 7.1）。
-   **契約を変える PR は、道具・文書・ガードを同じ PR で変える**（[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)）。
-   **ADR-0051 と ADR-0014 の食い違いの記述は、各 PR が直した分をその PR で取り除く**（AC-24、設計書 8 章）。
-   **利用者が行う手作業の結果は、その手作業を含む `/runplan` が、利用者の報告から §7 に書く。**

### 1.3. 計画の際に決めたこと（2026-11-17）

1.  **PR は 4 つにする。** PR-1 はフェーズ 1（名前の部品）、PR-2 はフェーズ 2〜3（前提の実地の確かめと、番号を決める側の道具）、PR-3 はフェーズ 4〜5（欠番の許容と `adr.md`、導入の確かめと文書の切り替え）、PR-4 はフェーズ 6〜7（`mergepr` の確かめと、cloud のセッションでの確かめの記録）である。`adr.md` の変更も `numreserve adr` を呼ぶ切り替えの一部である。フェーズ 4 とフェーズ 5 を同じ PR にまとめると、`adr.md` の変更も、導入の確かめ（ステップ 5-1）が済むまで `main` に入らない。2 つを別々の PR に分けると、`adr.md` だけが先に切り替わり、ruleset のない間の守りが `CLAUDE.md` の規則だけになる時期ができる。利用者との確認で決めた。ADR: なし（このタスクの PR の分け方であり、タスクの中で閉じる）。
2.  **REST の前提を確かめる tag は、記録の名前空間の外（`refs/tags/numreserve-probe/` の下）に作り、確かめた後に利用者の承認を得て消す。** 記録の名前空間で試すと、番号が欠番として残り、跳びの基準も押し上げる。前提は名前空間によらない GitHub の振る舞いなので、記録の外で確かめられる。設計書 7.3 の「試しに作る記録の tag は欠番として残る」は、記録の名前空間で試した場合の結果を述べたものである。本計画はその場合をとらない。利用者との確認で決めた。ADR: なし（確かめ方であり、タスクの中で閉じる）。
3.  **`mergepr` のテストでは、番号の確かめの部品を interface で差し替える。** `internal/mergepr` に確かめの部品の interface を置き、`mergepr.New` で受け取る。本番では `numreserve` の `Tool` を、テストでは fake を渡す。`git` と `gh` の応答を `mergepr` のテストの台本に足して確かめる案は、`mergepr` の各テストが `numreserve` の読み方の詳細に依存するので採らない（設計書 3.10 の「計画で決める」）。ADR: なし（テストの差し替え方であり、タスクの中で閉じる）。
4.  **`runpr.md` は変えない。`mergepr.md` には、番号の確かめで止まったときの扱いだけを足す。** `runpr.md` は `mergepr.md` の手順を実行し（`.claude/commands/runpr.md:910`）、`mergepr prepare` が失敗したら報告して止まる（`.claude/commands/runpr.md:1226`）。`mergepr.md` は「道具が止まったら、その誤りと示された対処を報告する」と定めている（`.claude/commands/mergepr.md:31`）。どれも `8252ed17` で読んだ。番号の確かめで止まったときも、この扱いで足りる。`mergepr.md` には、`merge` が番号の確かめで止まったら準備を `mergepr discard` で捨て、手引きの直し方の節を示して止まることを足す。`/runpr` がマージの直前に積んだ記録の commit（文書の `approved` など）は、マージされない PR に残る。タスク番号の作り直しでは、その commit は新しい PR に引き継がれない。このことと、作り直した PR で承認を記録し直すことを、`mergepr_guide.md` の直し方に書く（ステップ 6-6）。ADR: なし（手順の文言であり、タスクの中で閉じる）。
5.  **`CLAUDE.md` の「Declare, don't infer」の `numreserve` の例外、「Build Commands」の `make install-numreserve`、「Development Notes」と CI の `doc-guard` のパッケージの一覧は、道具のコードと同じ PR-2 に入れる。** どれも PR-2 で入るコードとガードを指すためである。例外を PR-3 まで遅らせると、規則に反するコードが `main` に入る。設計書 8 章の 5 は `CLAUDE.md` の変更をまとめて挙げているが、これらは PR-2 のコードとガードが前提とするので、先に入れる（フェーズの順の入れ替えではない）。ADR: なし（入れる PR の選択であり、タスクの中で閉じる）。
6.  **`cmd/codexreview` の設定のガードが許す語の集まり（`launchFree`、`cmd/codexreview/docs_test.go:49`）に `numreserve` を加える。** このガードは、`.claude/settings.json` の `permissions.allow` の Bash の規則の先頭の語を、`codex` を起動しないと確かめた語に限る（`cmd/codexreview/docs_test.go:53-69`、`8252ed17`）。`numreserve` は `git` と `gh` だけを、自分で組んだ引数で起動する。ADR: なし（既存のガードの許可の一覧への追加であり、タスクの中で閉じる）。
7.  **導入の確かめでは、番号の空間ごとに 1 回ずつ `numreserve` を実行し、そのときに作った記録の tag で削除の拒否を確かめる。** ruleset の確かめには、実際の名前の形の記録の tag が要る（設計書 7.3）。そのために決めた 2 つの番号（タスク番号 1 つ、ADR 番号 1 つ）は欠番になる。ADR: なし（確かめ方であり、タスクの中で閉じる）。
8.  **`cmd/numreserve` と `cmd/mergepr` は、番号を決める部品・確かめの部品を引数で受け取る `run` 関数に処理を置き、`main` だけが本番の部品を渡す。** テストは fake の部品で終了コードと出力を確かめる。本番の `Tool` を作る `numreserve.New` は、`testing.Testing()` でテストの実行の中だと分かったら、git と gh を呼ばずに誤りを返す `Tool` を作る（設計書 3.8）。`testing` を製品のコードで読み込むことは、depguard の許可（標準ライブラリ）の内である。ADR: なし（コードの形であり、タスクの中で閉じる）。
9.  **外部のコマンドの出力を読む部分のテストの入力は、フェーズ 2 で採った実の出力のサンプルから作る。** 読むのは、`gh api -i` の状態行と見出し、`git ls-remote` と `git ls-tree` の行である。本文、トークン、`origin` の URL を除いたサンプルを `internal/numreserve/testdata/` に置く。`chief` と `codexreview` の外部の出力の読み取りと同じく、読み取りの形を想像で作らないためである。ADR: なし（テストの入力の作り方であり、タスクの中で閉じる）。
10. **しきい値は、読み取りだけの git の操作で手で数え、使った操作とその出力をすべて §7 に記録する。** `internal/numreserve` の集め方を使う一度きりのプログラムで数える案は、そのためだけに読み取りの API を公開することになるので採らない。記録は PR-4 のレビューで読み直す（ステップ 6-1）。ADR: なし（一度きりの値の決め方であり、タスクの中で閉じる）。

### 1.4. 既存コード調査結果

コミット `8252ed17` で確かめた。

| 領域 | 今あるもの | 足りないもの・変えるもの |
|---|---|---|
| タスクのブランチ名とディレクトリ名の形 | `internal/chief/task.go:739` の `featureBranch`、`:742` の `taskDirName`、`:746` の `branchTask`。使う所は `task.go:787`・`:808` と `internal/chief/workspace.go:234`。テストは `internal/chief/task_test.go:278` の `TestBranchTask` | 2 つの正規表現を `internal/tasknaming` に移し、`branchTask` はその呼び出しにする。`TestBranchTask` と `TestTasksRead` は変えずに通す |
| 4 桁の番号の型 | `internal/chief/paneinput/paneinput.go:29-47` の `TaskID`・`ParseTaskID`。`internal/chief/detail.go:497-508` の `taskOfDir` と `screen_report.go:102` も `ParseTaskID` を使う | `tasknaming.Number` を加える。`paneinput.TaskID` は残し、`branchTask` で `Number.String()` から作る（設計書 3.7）。`taskOfDir` は、すでに選んだディレクトリのパスの先頭 4 桁だけを読み、名前の形を判定しないので変えない。`screen_report.go` は画面の文を読むので変えない |
| ADR のファイル名の形と欠番の判定 | `cmd/yt2column/adr_test.go:50` の `adrNameRE`。使う所は `:274` で、`:275` が `0000` を別に拒む（ケース「number 0000」）。`:65` の `adrMissingNumber` と `:87` の説明、`:336-355` の `checkADRNumbers`、`:656-667` の「missing number」のケース、`:647-654` の「duplicate file number」のケース | `adrNameRE` を `tasknaming.ParseADRFile` に置き換える。`0000` の拒否は残す（`Number` は `0000` を受け入れるため）。欠番の判定と `adrMissingNumber` を除く |
| 外部のコマンドの実行 | `internal/cmdrunner`（`NewOS(Options)`、`Options.MaxOutput`・`WaitDelay`・`Env`、`internal/cmdrunner/runner.go:52-60`）。git の引数の前置きは `GitHardening`、環境は `GitEnv`（`internal/cmdrunner/git.go:19-45`）。fake は `internal/cmdrunner/testutil/mocks.go` の `Fake`・`Step`。`Fake` は `ctx` も `Options` も見ない | `numreserve` はこれを使う。`Options` は `Env` を 1 つしか持たないので、git と gh の Runner を分ける。時間の上限は `Options` ではなく `context.WithTimeout` で課す |
| `mergepr` | `internal/mergepr/mergepr.go:57` の `New(runner)`、`:113` の `Prepare`（CI を待つのは `:137`、作業ディレクトリを作るのは `:162`）、`:228` の `Merge`（`gh pr merge` は `:254`）。Runner は上限を持たない（`internal/mergepr/runner.go:7`）。テストの補助は `internal/mergepr/test_helpers.go:75` の `newTool`。`cmd/mergepr/main.go:25` の `run` は、引数を読む前に `mergepr.New` を呼ぶ（`:29`） | `New` が番号の確かめの部品を受け取るようにし、`Prepare` と `Merge` で呼ぶ（§1.3 の決定 3）。パッケージの doc コメント（`mergepr.go:1-7`）の「Its only safety obligations」に番号の確かめを足す |
| 道具のインストールと文書のガード | `Makefile:82-102` の `install-*`。`cmd/chief/docs_test.go:387` と `cmd/codexreview/docs_test.go:491` の `TestInstallTargetInMakefile`。`cmd/codexreview/docs_test.go:80` の `TestSettingsAllowsTool`、`:49` の `launchFree`、`:108` の `TestMayBypassTool`。呼び出しを読む補助は `internal/doctestutil/commands.go:32` の `Invocations`、`:56` の `PromptFree`、`internal/doctestutil/protocol.go:86` の `CommandDocs`。見出しで節を取り出す補助は `internal/doctestutil/markdown.go:44` の `SectionOutsideBlocks`。既存のガードは `CommandDocs`（`.claude/commands/`）だけを走査し、`CLAUDE.md` は走査しない（`cmd/codexreview/docs_test.go:219`、`cmd/chief/docs_test.go:260`） | 同じ形のガードを `cmd/numreserve` に作る。`CLAUDE.md` では「Tool Execution Safety」の節だけを読む。`CLAUDE.md` の他の節には `go install ./cmd/<tool>` と `doc-guard` の `go test` の一覧（`CLAUDE.md:182-185`・`:707`）があり、`Invocations` はそこの `./cmd/numreserve` も呼び出しとして読むためである。`launchFree` に `numreserve` を加える（§1.3 の決定 6） |
| `doc-guard` のジョブ | `.github/workflows/ci.yml:215` のパッケージの一覧。同じ一覧が `CLAUDE.md:707` にある | 両方に `./cmd/numreserve` を加える |
| パッケージの一覧のガード | `internal/pipeline/pipeline_test.go:630` の `TestPackageReferenceListsPackages`（製品のコードを持つすべてのパッケージに `package_reference.md` の行を求める） | パッケージを加える PR で、同じ PR に行を加える |
| `/mkreq` の番号の決め方 | `.claude/commands/mkreq.md:69-76`（step 2 の「Without it」）、Authorization は `:34-40` | step 2 を `numreserve task` の呼び出しに置き換え、Authorization に加える |
| ADR の規約 | `docs/dev/conventions/adr.md:30-34`（Number）、`:96`（Index の「A new row goes at the end」）、`:367-374`（Taking in `main` の手順 2）、`:379-388`（When `TestADRRecords` fails on `main`。「Taking in `main`」の節の中の太字の段落） | AC-18 の 4 つの差分（設計書 3.9） |
| `_lib/settle-decisions.md` | ADR を作る段（`.claude/commands/_lib/settle-decisions.md:58-61`） | 番号は規約の「Location and naming」の手順で決めると明記する |
| `.claude/settings.json` | `permissions.allow` は `coderabbit review *`・`codexreview *`・`chief *` の 3 つ | `Bash(numreserve task)` と `Bash(numreserve adr)` を加える |
| ADR-0014 | 「決定」の最初の項目が道具を「5 つ」と数える | `numreserve` を加えて 6 つにする |
| 並行する変更 | タスク 0099（PR #942、設計の段）は、`mergepr` に `--protocol` の番号を持たせ、マージの時点の確かめを足す予定である（ADR-0014 の「決定」） | 0099 の実装が先にマージされたら、フェーズ 6 で `mergepr` の契約を変えるときに、ADR-0014 の規則に従って `mergepr` の番号を上げる（§5 のリスク 4） |

**型による守り:** `tasknaming.Number` は、標準ライブラリだけに依存する新しいパッケージに置き、非公開のフィールドを持つ。そのため、`ParseNumber` などの読み取りを通らずに作れるのはゼロ値だけである（コンパイル時の守り）。ゼロ値は正しくない値として扱う。`numreserve` の `Space`・`StopKind`・`AddedIssueKind` は `int` の型なので、ほかのパッケージからも範囲外の値を作れる。そのため、これらで振る舞いを分ける `switch` には、安全側に倒す `default` を置く（ステップ 3-1・6-2）：未知の空間は拒み、未知の止める理由は理由の語を持たずに終了コード 1 で止まり、未知の種類の指摘はマージを止める。文字列から `Space` を作る入口は `ParseSpace` だけである。

### 1.5. 申し送りへの対応

本タスクに `implementation_handoff.md` はない。`design_handoff.md` の H-01〜H-10 は、どれも設計書の付録 A に記録がある。H-07（cloud のセッション）は、フェーズ 2 とフェーズ 7 で確かめる。H-08（移行の順序）は §1.3 の決定 1 とステップ 5-1 で扱う。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 名前の部品（設計書 8 章の 1）

**ステップ 1-1: `internal/tasknaming` を作る**

-   対象ファイル：`internal/tasknaming/tasknaming.go`、`internal/tasknaming/tasknaming_test.go`
-   [ ] 設計書 3.7・3.8 の `Number`・`ParseNumber`・`String`・`ParseBranch`・`ParseTaskDir`・`ParseADRFile` を作る。形は `featureBranch`・`taskDirName`（`internal/chief/task.go:739`・`:742`）と `adrNameRE`（`cmd/yt2column/adr_test.go:50`）と同じにする。`ParseADRFile` は `0000` を受け入れる（`0000` の拒否は `TestADRRecords` に残す）
-   [ ] 呼び出し側が番号を比べ、最大値を求め、1 を足せるように、`Number` に整数を返すメソッドを加える
-   [ ] テストを書く。今の `TestBranchTask` が受け入れる名前と拒む名前、`adrNameRE` が受け入れる名前と拒む名前、`0000`・`9999`、4 桁でない数字、ASCII でない数字を入力にする。ゼロ値の `Number` が正しくない値であることも確かめる
-   完了の基準：`go test -tags test ./internal/tasknaming` が通る。テストの各行が、対応する判定を壊すと失敗することを確かめ、コミットメッセージに書く

**ステップ 1-2: `chief` を名前の部品に移す**

-   対象ファイル：`internal/chief/task.go`
-   [ ] `featureBranch` と `taskDirName` を除き、`branchTask` と `Read` の判定（`task.go:808`）を `tasknaming.ParseBranch`・`ParseTaskDir` の呼び出しにする。`branchTask` の返す `paneinput.TaskID` は `Number.String()` から作る
-   完了の基準：`internal/chief` の既存のテスト（`TestBranchTask`・`TestTasksRead` を含む）を変えずに通す（AC-04）

**ステップ 1-3: `TestADRRecords` の名前の判定を名前の部品に移す**

-   対象ファイル：`cmd/yt2column/adr_test.go`
-   [ ] `adrNameRE` を除き、`:274` の判定を `tasknaming.ParseADRFile` の呼び出しにする。`:275` の `0000` の拒否は残す。欠番の判定は、このステップでは変えない（フェーズ 4）
-   完了の基準：`TestADRRecords` と `TestCheckADRRecords`（ケース「number 0000」を含む）を変えずに通す

**ステップ 1-4: 文書**

-   対象ファイル：`docs/dev/conventions/package_reference.md`
-   [ ] `internal/tasknaming` の行を加える
-   完了の基準：`make test`・`make lint`・`make deadcode` が通る

### PR-1 作成ポイント: shared name forms for task branches, task directories, and ADR files

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4

**推奨タイトル**: `refactor(0100): タスクのブランチ名・ディレクトリ名と ADR のファイル名の形を共有のパッケージに移す`

**レビュー観点**: 名前の部品の形が chief と TestADRRecords の今の正規表現と同じか / chief と TestADRRecords の既存のテストを変えずに通しているか / ゼロ値の Number を正しくない値として扱い、0000 の ADR を今までどおり拒んでいるか

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない（振る舞いを変えない移し替えで、競う手段も外部の面もない）。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 前提の実地の確かめ（設計書 8 章の 2）

このフェーズは GitHub への読み書きを伴う。どの操作も、実行の前に利用者の承認を得る（`CLAUDE.md` の「Tool Execution Safety」）。結果は §7 の「PR-2 の実地の確かめの記録」に書く。日付、`gh` と `git` の版、HTTP の状態コード、終了コード、確かめた事実を書き、応答の本文、トークン、`origin` の URL は書かない。

**ステップ 2-1: REST の前提を手元で確かめる**

-   [ ] 試しの tag（§1.3 の決定 2）の名前に、実行ごとに違う接尾辞を付ける。`refs/heads/main` の SHA を指すように `POST git/refs` で作り、201 を確かめる
-   [ ] 同じ名前で同じ SHA の作成と、同じ名前で違う SHA の作成が、どちらも 2xx にならないことを確かめる（設計書 7.3）。そのときの状態コードを記録する
-   [ ] `GET git/ref/tags/...` が 200 を返し、応答の `ref` が問い合わせた名前と一致することと、`gh api -i` の標準出力の先頭の状態行の形を確かめる（設計書 3.3.1・3.3.2）
-   [ ] 途中の確かめが失敗しても、試しの tag を消す。消えたことを `GET` の 404 で確かめる

**ステップ 2-2: SHA による取得を手元で確かめる**

-   [ ] `git ls-remote` が返した SHA を、`--no-tags` を付け `FETCH_HEAD` を書かずに `origin` から取得でき、その commit の `docs/dev/adr/` を `git ls-tree` で読めることを確かめる（設計書 3.2 の「取得」）。取得の前後の `git for-each-ref` の出力が同じで、`FETCH_HEAD` のファイルが変わらないことを記録する。`git` の版が 2.29 以上であることも記録する。テストの台本に書く取得の引数は、ここで確かめた形にする

**ステップ 2-3: 出力のサンプルを採る**

-   対象ファイル：`internal/numreserve/testdata/`
-   [ ] ステップ 2-1・2-2 の、`gh api -i` の状態行と見出しの名前と値（201・422・200・404）、`git ls-remote` の行、`git ls-tree` の行を、本文・トークン・URL を除いてファイルに残す（§1.3 の決定 9）

**ステップ 2-4: cloud のセッションの到達性を確かめる（利用者が行う）**

-   [ ] `/runplan` が利用者に依頼する。利用者は cloud のセッション（claude.ai/code）で次を確かめ、結果を報告する：`gh` があり認証されているか、REST でこのリポジトリを読めるか、試しの tag を作って消せるか、`git ls-remote origin` が `refs/heads/` と `refs/tags/` を返すか、ステップ 2-2 の取得ができるか、`git` と `go` の版、`main` から `go install ./cmd/mergepr` で既存の道具を入れられるか（AC-22 の一部、申し送りの H-07。`numreserve` 自体はまだ `main` にないので、フェーズ 7 で入れる）
-   [ ] `/runplan` が、利用者の報告を §7 に書く

**ステップ 2-5: 結果を判断する**

-   [ ] ステップ 2-1 の同じ名前での 2 回目と 3 回目の作成のどちらかが 2xx になったら、フェーズ 3 に進まない。§7 に「REST の前提が崩れた」と書いて、その記録とサンプルだけをコミットし、PR は作らずに止まる。方式の決め直しは、決定の変更として利用者と相談して進め（`docs/dev/conventions/requirements_process.md` の「Editing an approved document」）、設計書と本計画を直してから続ける
-   [ ] ステップ 2-2 またはステップ 2-4 の結果が設計書の前提（3.2 の取得、`git ls-remote` の行の形）と違ったら、同じく止めて設計書を直す。cloud で `gh` や道具が使えないことは止める理由にしない。制約として記録し、フェーズ 5 の手引きとフェーズ 7 の ADR-0051 に書く
-   完了の基準：§7 の記録があり、方式を続けられるという判断が書かれている

### フェーズ 3: 番号を決める側の道具（設計書 8 章の 3）

**ステップ 3-1: 型と上限**

-   対象ファイル：`internal/numreserve/numreserve.go`、`internal/numreserve/bounds.go`
-   [ ] 設計書 3.8 の `Space`・`ParseSpace`・`StopKind`・`StopError` を作る。`StopKind` ごとに、設計書 4.1 の表の理由の語を返すメソッドと、すべての `StopKind` を返す関数を持たせる。ゼロ値と範囲外の値は理由の語を持たない（§1.4 の「型による守り」）
-   [ ] 設計書 3.4 の表の値を、テストの補助が小さくできる構造体に置く（3 分の上限をテストで待たないため）
-   完了の基準：`go test -tags test ./internal/numreserve` が通る

**ステップ 3-2: 使用中の番号を集める**

-   対象ファイル：`internal/numreserve/collect.go`、`internal/numreserve/collect_test.go`
-   [ ] 設計書 3.2 の表の入力を、空間ごとに読む。`git ls-remote` の行の分け方、SHA だけの取得、`origin/main` の一覧の形（3.2.1）、記録の tag の名前の拒否（3.3.3）、上限の超過（3.4）を、設計書のとおりにする
-   [ ] ブランチの木を読む部分は、ステップ 6-2 が `docs/tasks/` にも使えるように、1 つの関数にまとめる。読むディレクトリを引数にするのは、2 つ目の呼び出し側が加わるステップ 6-2 で行う
-   [ ] 作業ツリーの `docs/dev/adr/` は、`Tool` が持つ作業ツリーの根から読む。テストは根を `t.TempDir()` にできる
-   [ ] 名前の形の判定は `internal/tasknaming` を呼ぶ。同じ判定を書き直さない
-   完了の基準：このステップの判定のテスト（ステップ 3-6 の規則に従う）を、このステップで書いて通す

**ステップ 3-3: 番号を決める**

-   対象ファイル：`internal/numreserve/reserve.go`、`internal/numreserve/reserve_test.go`
-   [ ] 設計書 3.3.2 の表と 3.4 の候補・跳び・4 桁の上限・回数の上限を、6.1 の流れのとおりに作る。状態コードと rate limit の見出しは `gh api -i` の出力から読み、応答の本文の `message` は読まない
-   [ ] 止める理由の詳細には、設計書 3.1 の「詳細」の規則（`%q` の引用、200 バイトで切る、本文・標準エラー・URL・認証の情報を出さない）を当てる
-   完了の基準：このステップの判定のテスト（ステップ 3-6 の規則に従う）を、このステップで書いて通す

**ステップ 3-4: 本番の `Tool` とテストの補助**

-   対象ファイル：`internal/numreserve/tool.go`、`internal/numreserve/test_helpers.go`（`//go:build test`）
-   [ ] 本番の Runner の `cmdrunner.Options` と、git と gh の環境（設計書 3.2 の「git と gh の環境」）を返す純粋な関数を作り、値と外す変数をテストで確かめる。git と gh は Runner を分ける。git の引数の前に `GitHardening` を付ける
-   [ ] コマンド 1 つと 1 回の実行の時間の上限は、`context.WithTimeout` で課す。テストの補助に、呼び出しごとの `ctx` の期限を記録する、Runner を包む部品を置き、各呼び出しの期限と全体の期限を確かめる
-   [ ] `New` は、`testing.Testing()` が真なら git と gh を呼ばずに誤りを返す `Tool` を作る（§1.3 の決定 8）。この拒否のテストは、テストの中で（`t.Setenv`）`PATH` を失敗するだけの `git`・`gh` の置き場所にし、`GH_TOKEN`・`GITHUB_TOKEN` を外してから `New` を呼ぶ。拒否を壊して失敗を確かめる作業も、このテストの中の環境で行う。実の GitHub に届かないためである
-   [ ] テストの補助で、`cmdrunner/testutil` の `Fake` と作業ツリーの根から `Tool` を作る関数を置く（テストの組み立ての指針の Classification B）
-   完了の基準：ステップ 3-6 のテストが通る

**ステップ 3-5: `cmd/numreserve`**

-   対象ファイル：`cmd/numreserve/main.go`、`cmd/numreserve/main_test.go`
-   [ ] 番号を決める部品を引数で受け取る `run` に処理を置き、`main` だけが `numreserve.New()` を渡す（§1.3 の決定 8）。終了コード 0・1・2 と、標準出力・標準エラーの形を設計書 3.1 のとおりにする
-   [ ] テストで、番号を決めたとき、止まったとき（すべての `StopKind` の理由の語と、`StopError` でない誤り）、引数の誤り（引数なし、3 つ目の語、フラグ、`task` と `adr` 以外）の終了コードと出力を、fake の部品で確かめる。引数の誤りでは fake の部品が呼ばれないことも確かめる
-   完了の基準：`go test -tags test ./cmd/numreserve` が通る

**ステップ 3-6: `internal/numreserve` のテスト**

-   対象ファイル：`internal/numreserve/*_test.go`
-   [ ] 設計書 7.1 の項目のうち、マージの前の確かめと名前の部品を除くすべてをテストにする。外部の出力はステップ 2-3 のサンプルから作る
-   [ ] 使用中の番号の各入力（作業ツリーだけにある ADR のファイルを含む）は、その入力だけが最大値を決める状況を作り、ほかの入力だけでは同じ候補にならないことを先に確かめる（`CLAUDE.md` の「A layered path needs inputs only one layer can handle」）。fork の除外は、`Fake` が台本にない呼び出し（`gh pr list` を含む）で失敗することで、すべての集め方のテストが確かめる
-   [ ] 上限は、ちょうど上限で番号が決まる場合と、1 つ超えて止まる場合を対にする（`POST` の 10 回目と 11 回目、候補 9999 と 10000、基準 + 20 と + 21）。タスクと ADR の両方の空間で確かめる
-   [ ] 誤りの種類は `errors.AsType[*StopError]` と `Kind` で確かめ、メッセージの文字列では確かめない
-   [ ] 各テストが、確かめる判定を壊すと失敗することを確かめ、コミットメッセージに書く
-   完了の基準：`go test -tags test ./internal/numreserve` が通る

**ステップ 3-7: インストール、`CLAUDE.md`、CI**

-   対象ファイル：`Makefile`、`cmd/numreserve/docs_test.go`、`CLAUDE.md`、`.github/workflows/ci.yml`、`docs/dev/conventions/package_reference.md`
-   [ ] `Makefile` に `install-numreserve` を加える（`.PHONY` の一覧を含む）。`cmd/numreserve/docs_test.go` に、そのレシピが `./cmd/numreserve` を入れることを確かめるテストを置く（`cmd/chief/docs_test.go:387` と同じ形）
-   [ ] `CLAUDE.md` の「Build Commands」に `make install-numreserve` を、「Declare, don't infer」に `numreserve` の例外（設計書 3.9 の範囲と境界）を、「Development Notes」の `doc-guard` のパッケージの一覧に `./cmd/numreserve` を加える。`.github/workflows/ci.yml` の `doc-guard` の一覧にも加える（§1.3 の決定 5）
-   [ ] `package_reference.md` に `cmd/numreserve` と `internal/numreserve` の行を加える
-   完了の基準：`make test`・`make lint` が通る

**ステップ 3-8: ADR**

-   対象ファイル：`docs/dev/adr/0051-number-reservation.md`、`docs/dev/adr/0014-deterministic-checks-in-go.md`
-   [ ] ADR-0051 の「決定」の食い違いの記述から `numreserve` を除く。「結果」の「すでにある ref の作成を GitHub が成功として受けないことは、決めた時点では確かめていない」を、ステップ 2-1 の結果に合わせて直す
-   [ ] ADR-0014 の「決定」の最初の項目の道具の一覧に `numreserve` を加え（6 つ）、食い違いの記述から `numreserve` と `make install-numreserve` を除く（手引きは残す）
-   完了の基準：`make test`・`make lint`・`make deadcode` が通る

### PR-2 作成ポイント: field checks and the numreserve tool that decides numbers

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8

**推奨タイトル**: `feat(0100): REST の前提を実地で確かめ、番号の記録の tag を作って番号を決める道具 numreserve を加える`

**レビュー観点**: 422 の後に読み直して確かめた失敗だけで次の番号へ進み、ほかはすべて止まるか / 受け入れない形の入力を読み飛ばさずに止まり、詳細に本文や URL を出さないか / 上限が呼び出し側によらず道具の中にあり、テストが実の GitHub に tag を作れないか / §7 の実地の確かめの記録とサンプルが方式を続ける判断を支えているか

**実装モデル要件**: opus

**判定理由**: ステップ 2-1・2-2・2-4 は確かめていない外部の振る舞い（REST の ref の作成、SHA による取得、cloud の到達性）を本物の GitHub で確かめる実現性の確かめで、ステップ 3-3 は消せない tag を作る安全の境界である。Sonnet は、想定と違う結果を確かめたものとして記録するか、422 をそのまま「使用中」とみなして次の番号へ進めやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 欠番の許容と `adr.md`（設計書 8 章の 4）

このフェーズの `adr.md` の変更は、同じ PR-3 のステップ 5-1（導入の確かめ）が済むまで `main` に入らない（§1.3 の決定 1）。

**ステップ 4-1: `TestADRRecords` が欠番を許すようにする**

-   対象ファイル：`cmd/yt2column/adr_test.go`
-   [ ] `checkADRNumbers` から欠番の判定を除き、`adrMissingNumber` とその説明（`:65`・`:87`）、doc コメント（`:336-337`）を直す（設計書 3.10）。重複の issue の並びは、番号の順のまま決まるようにする
-   [ ] 「missing number」のケース（`:656-667`）を、欠番があってもどの種類の issue も返さないことを確かめるケースに変える（AC-16）。「duplicate file number」のケース（`:647-654`）は残す（AC-17）
-   完了の基準：欠番の判定を戻すと、変えたケースが失敗することを確かめ、コミットメッセージに書く。`make test`・`make deadcode` が通る

**ステップ 4-2: `adr.md` の 4 つの差分**

-   対象ファイル：`docs/dev/conventions/adr.md`
-   [ ] 設計書 3.9 の「`adr.md`」の 4 項目だけを変える（AC-18）。呼び出しは `numreserve adr` の形で書く（AC-12・AC-13・AC-20）。道具が止まったときに示す手引きの節（`docs/dev/guides/numreserve_guide.md`）への参照は、「Location and naming」の番号の決め方の項目（AC-18 の 1 つ目の差分）の中に書く。手引きはステップ 5-6 で同じ PR に加わる
-   完了の基準：§7 の「Diff record (AC-18)」を記録する（ステップ 5-8）

### フェーズ 5: 導入の確かめと、文書と設定の切り替え（設計書 8 章の 5）

**ステップ 5-1: 導入の確かめ（利用者が行う）**

-   [ ] `/runplan` が利用者に依頼する。利用者は、`main` から `make install-numreserve` で道具を入れ、手元で `numreserve task` と `numreserve adr` を 1 回ずつ実行し、標準出力の番号と標準エラーの行を報告する。決めた番号は欠番になる（§1.3 の決定 7）
-   [ ] 利用者が、設計書 3.3.3 のとおりに記録の ruleset を設定する
-   [ ] 前の項で作った 2 つの記録の tag を、利用者の承認のもとで REST の `DELETE git/refs/...` で消そうとし、状態コードが 2xx でないことと、その後の `GET` が 200 を返すことを確かめる。消えてしまったら、同じ名前と同じ SHA で直ちに作り直し、ruleset の対象を見直してから確かめ直す（設計書 7.3）
-   [ ] `/runplan` が結果を §7 の「導入の確かめの記録」に書く
-   完了の基準：削除が拒まれたことが §7 に書かれている。これがないうちは、PR-3 を作らない

**ステップ 5-2: `mkreq.md`**

-   対象ファイル：`.claude/commands/mkreq.md`
-   [ ] step 2 の「Without it」の番号の計算（`:69-74`）を、`numreserve task` の呼び出しと、止まったときの扱い（設計書 3.9 の「`mkreq.md` の step 2」、4.1 の末尾）に置き換える。`<name>` の決め方の文は残す
-   [ ] Authorization（`:34-40`）に `numreserve task` を加え、それが `origin` に記録の tag を 1 つ作ることを書く

**ステップ 5-3: `_lib/settle-decisions.md`**

-   対象ファイル：`.claude/commands/_lib/settle-decisions.md`
-   [ ] ADR を作る段に、番号は ADR の規約の「Location and naming」の手順で決めることを明記する（AC-12）

**ステップ 5-4: `CLAUDE.md`**

-   対象ファイル：`CLAUDE.md`
-   [ ] 「Tool Execution Safety」に、設計書 3.9 の項目（AC-19・AC-21）を加える。`numreserve task` と `numreserve adr` の両方の形をコードとして書く
-   [ ] 「Testing Strategy」の信頼の境界の例外の一覧に `numreserve` の呼び出しの形を加え、Quick Links に手引きを加える

**ステップ 5-5: 設定**

-   対象ファイル：`.claude/settings.json`、`cmd/codexreview/docs_test.go`
-   [ ] `permissions.allow` に `Bash(numreserve task)` と `Bash(numreserve adr)` を加える
-   [ ] `launchFree` に `numreserve` を加え、その doc コメントに、`numreserve` は `git` と `gh` だけを起動することを書く（§1.3 の決定 6）。`TestMayBypassTool` の表に、2 つの規則が通る行を加える

**ステップ 5-6: 手引き**

-   対象ファイル：`docs/dev/guides/numreserve_guide.md`
-   [ ] 設計書 3.9 の「手引き」の 6 項目で書く。止める理由の表は設計書 4.1 から作る。記録の ruleset の対象の形は、道具が読む記録の名前空間と同じにする。cloud のセッションの制約は、ステップ 2-4 の結果を書く（フェーズ 7 で確かめ直す）。マージの前の確かめの直し方は、フェーズ 6 で足すと書いておく

**ステップ 5-7: 文書のガード**

-   対象ファイル：`cmd/numreserve/docs_test.go`
-   [ ] 次を確かめるテストを書く（設計書 7.2）。規則と呼び出しの分け方は、受け入れる例と拒む例の表のテストも持たせる
    -   `.claude/settings.json` の、先頭の語が `numreserve`（パスや引用の形を含む）の規則が、`Bash(numreserve task)` と `Bash(numreserve adr)` のちょうど 2 つである。`Bash(numreserve *)`・`Bash(numreserve:*)`・`Bash(numreserve task *)` は拒む
    -   `.claude/commands/` の下のすべての文書、`docs/dev/conventions/adr.md`、`CLAUDE.md` の「Tool Execution Safety」の節の `numreserve` の呼び出しが、どれも単独のコマンドで（`doctestutil.PromptFree`）、引数が `numreserve.ParseSpace` の受け付ける 1 語である。`CLAUDE.md` のほかの節（`go install ./cmd/numreserve` など）は読まない
    -   `mkreq.md` に `numreserve task` の呼び出しがあり、`adr.md` の「Location and naming」の節、「Taking in `main`」の手順 2、「When `TestADRRecords` fails on `main`」の段落に `numreserve adr` の呼び出しがあり、`CLAUDE.md` の「Tool Execution Safety」に 2 つの形の呼び出しがある
    -   手引きの止める理由の表の語が、すべての `StopKind` の理由の語とちょうど一致し、手引きの ruleset の対象の形が道具の記録の名前空間と一致する
-   [ ] 各テストが、文書か設定を壊すと失敗することを確かめ、コミットメッセージに書く
-   完了の基準：`go test -tags test ./cmd/numreserve ./cmd/codexreview` が通る

**ステップ 5-8: ADR と差分の記録**

-   対象ファイル：`docs/dev/adr/0051-number-reservation.md`、`docs/dev/adr/0014-deterministic-checks-in-go.md`
-   [ ] ADR-0051 の食い違いの記述から、記録の ruleset、`adr.md`・`TestADRRecords`・`CLAUDE.md`・`mkreq.md` の分を除く
-   [ ] ADR-0014 の食い違いの記述から手引きを除き、記述が空になったら食い違いの項目ごと除く
-   [ ] AC-18 の差分を記録する（§7 の「Diff record (AC-18)」）
-   完了の基準：`make test`・`make lint`・`make deadcode` が通る

### PR-3 作成ポイント: allow ADR gaps and switch the numbering procedures to numreserve

**対象ステップ**: 4-1 / 4-2 / 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 / 5-8

**推奨タイトル**: `feat(0100): ADR 番号の欠番を許し、タスク番号と ADR 番号の決め方を numreserve の呼び出しに切り替える`

**レビュー観点**: adr.md の差分が AC-18 の 4 つに限られているか / 常設の承認が決まった 2 つの形だけで、記録の tag と ruleset の書き換えに承認を求めているか / 文書のガードが呼び出しの形と settings と手引きの値を道具の定義と照らしているか / §7 の導入の確かめの記録で削除が拒まれたことを確かめているか

**実装モデル要件**: opus

**判定理由**: ステップ 5-1・5-2・5-4・5-5 は、手順の切り替え（移行の段）と、エージェントの信頼の境界（常設の承認、記録の書き換えの禁止、settings の許可）を変えるステップである。Sonnet は、許可の形を広げるか、道具が止まったときに今の計算で続ける余地を文書に残しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: マージの前の確かめ（設計書 8 章の 6）

**ステップ 6-1: しきい値を決める**

-   [ ] 実装を始めるときに、利用者の承認のもとで `git fetch origin` を行い、番号の空間ごとに、`origin/main` とすべての `origin` のブランチで使われている番号の最大値を、読み取りだけの git の操作で数える（設計書 3.6、§1.3 の決定 10）。使った操作、その出力のすべて、日付、`origin/main` の commit を §7 の「しきい値の記録」に書く

**ステップ 6-2: `CheckAdded`**

-   対象ファイル：`internal/numreserve/check.go`、`internal/numreserve/check_test.go`
-   [ ] 設計書 3.5 の `CheckAdded` を作る。しきい値は定数にし、コメントで §7 の記録を指す。読み方と上限はステップ 3-2 の部品を使い、書き直さない。ブランチの木を読む関数に、読むディレクトリの引数を加える。`AddedIssueKind` の `switch` は安全側の `default` を持つ
-   [ ] 設計書 7.1 の「マージの前の確かめ」の項目をテストにする。しきい値の上と下は、ちょうどしきい値と 1 つ上を対にする。各テストが、確かめる判定を壊すと失敗することを確かめ、コミットメッセージに書く
-   完了の基準：`go test -tags test ./internal/numreserve` が通る

**ステップ 6-3: `docs/tasks/` の形の文書のガード**

-   対象ファイル：`cmd/yt2column/tasks_test.go`
-   [ ] `docs/tasks/` の項目の一覧を受け取り、タスクのディレクトリ名の形（`tasknaming.ParseTaskDir`）のディレクトリでない項目を返す関数を作り、表のテストで確かめる：ファイルの項目、形に合わないディレクトリ、`0000_template`（受け入れる）、ASCII でない数字。実の `docs/tasks/` にこの関数を当てるテストも置く（設計書 3.5 の「形の確かめ」）。`TestADRRecords` と同じく、git の管理外の手元のファイルも項目として扱う
-   完了の基準：`go test -tags test ./cmd/yt2column` が通る

**ステップ 6-4: `internal/mergepr`**

-   対象ファイル：`internal/mergepr/mergepr.go`、`internal/mergepr/errors.go`、`internal/mergepr/test_helpers.go`、`internal/mergepr/mergepr_test.go`
-   [ ] 番号の確かめの部品の interface を置き、`New` で受け取る（§1.3 の決定 3）。`Prepare` は CI を待った後、作業ディレクトリを作る前に、`Merge` は `gh pr merge` の直前に呼ぶ。base が `main` でなければ呼ばない（設計書 3.5）
-   [ ] パッケージの doc コメントの「Its only safety obligations」に、番号の確かめを足す
-   [ ] fake の部品でテストを書く：見つけたものがあれば止まって作業ディレクトリを作らない・マージしない、誤りなら止まる、base が `main` でなければ呼ばない、`Prepare` では `gh pr checks` の後に呼ぶ、`Merge` では `gh pr merge` より前に呼ぶ、渡す値が state の `HeadRefOID` と `HeadRefName` である。今のテストは、何も見つけない fake で通す
-   完了の基準：`go test -tags test ./internal/mergepr` が通る

**ステップ 6-5: `cmd/mergepr`**

-   対象ファイル：`cmd/mergepr/main.go`、`cmd/mergepr/main_test.go`
-   [ ] `run` が確かめの部品を引数で受け取り、`main` だけが本番の部品を渡す（§1.3 の決定 8）。見つけたものを 1 件 1 行（種類・番号・PR の名前・使っている所）で出し、終了コード 1 で終わる（設計書 4.2）。この出力を fake の部品でテストする
-   [ ] `TestRunRejectsExtraArguments` が実の GitHub に触れずに通ることを確かめる
-   [ ] `mergepr` が `--protocol` の番号を持っていたら（タスク 0099 の実装が先にマージされた場合）、ADR-0014 の規則に従って番号を上げ、`mergepr.md` を合わせる
-   完了の基準：`go test -tags test ./cmd/mergepr` が通る

**ステップ 6-6: 文書**

-   対象ファイル：`.claude/commands/mergepr.md`、`docs/dev/guides/mergepr_guide.md`、`docs/dev/guides/numreserve_guide.md`、`docs/dev/adr/0051-number-reservation.md`
-   [ ] `mergepr.md` に、番号の確かめで止まったときの扱いを足す（§1.3 の決定 4）。`runpr.md` は変えない
-   [ ] `mergepr_guide.md` に、番号の確かめ、保証すること、止まったときの対処（設計書 3.5 の「止まったときの直し方」と、§1.3 の決定 4 の記録の commit）を書く。`numreserve_guide.md` のマージの前の確かめの節を、この節を指すように直す
-   [ ] ADR-0051 の食い違いの記述から、`mergepr` によるマージの前の確かめを除く
-   完了の基準：`make test`・`make lint`・`make deadcode` が通る

### フェーズ 7: cloud のセッションでの確かめ（設計書 8 章の 7）

**ステップ 7-1: cloud のセッションで番号を決める（利用者が行う）**

-   [ ] `/runplan` が利用者に依頼する。利用者は cloud のセッションで `main` から道具を入れ、`numreserve task` と `numreserve adr` を 1 回ずつ実行し、標準出力と標準エラーの行を報告する（AC-22）
-   [ ] `/runplan` が、決まった番号が AC-02・AC-11 の条件を満たすことを確かめる：その番号が、その時点の `origin/main` の一覧、`origin` のブランチ名とブランチの `docs/dev/adr/`、ほかの記録の tag のどれにもないことを、読み取りの操作で確かめる。決められなかったときは、止まって理由の語が示されたことを確かめる
-   [ ] `/runplan` が、報告と確かめの結果を §7 の「cloud のセッションでの確かめの記録」に書く

**ステップ 7-2: 結果を文書に書く**

-   対象ファイル：`docs/dev/adr/0051-number-reservation.md`、`docs/dev/guides/numreserve_guide.md`
-   [ ] ADR-0051 の「結果」の「cloud のセッションで番号を決められるかは、決めた時点では確かめていない」を、ステップ 7-1 の結果に置き換える。決められなかったときは、その制約を書く
-   [ ] 手引きの cloud のセッションの制約を、ステップ 7-1 の結果に合わせて直す
-   [ ] ADR-0051 の食い違いの記述が空になったことを確かめ、項目ごと除く（AC-24）
-   完了の基準：`make test`・`make lint`・`make deadcode` が通る

### PR-4 作成ポイント: pre-merge number check in mergepr and the cloud session record

**対象ステップ**: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 7-1 / 7-2

**推奨タイトル**: `feat(0100): mergepr でマージの前に PR が加える番号を確かめ、cloud のセッションでの確かめを記録する`

**レビュー観点**: PR が加える番号を merge-base と比べて数え、しきい値と記録の tag と最新の main とほかのブランチで確かめているか / 読めないときにマージせずに止まり、確かめが何も書かないか / prepare と merge の両方で、確かめが CI の後と gh pr merge の直前にあるか / §7 のしきい値の記録と cloud の結果が、定数と ADR-0051 と手引きに合っているか

**実装モデル要件**: opus

**判定理由**: ステップ 6-2・6-4 は、マージを止める安全の関門（重複・記録のない番号・ほかのブランチ）で、ステップ 7-1 は本物の cloud のセッションでの確かめである。Sonnet は、読めない状態を確かめを通ったものとして扱うか、最新の main ではなく PR の base と比べて、main で名前を変えたものを加えたものと数えやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 対応する PR |
|---|---|---|
| M1：名前の部品 | `internal/tasknaming`。`chief` と `TestADRRecords` がそれを使う | PR-1 |
| M2：道具 | 前提の実地の確かめの記録とサンプル、`numreserve`、`make install-numreserve` | PR-2 |
| M3：切り替え | 欠番を許す `TestADRRecords`、導入の確かめの記録、記録の ruleset、道具を呼ぶ `mkreq.md`・`adr.md`、`CLAUDE.md`、settings、手引き、文書のガード | PR-3 |
| M4：マージの前の確かめ | `mergepr` の確かめ、`docs/tasks/` の形のガード、cloud のセッションでの確かめの記録 | PR-4 |

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 | 名前の部品を作り、`chief` と `TestADRRecords` を移す | sonnet | auto |
| PR-2 | 2-1 〜 2-5 / 3-1 〜 3-8 | 前提の実地の確かめとサンプル、`numreserve`、インストール | opus | auto |
| PR-3 | 4-1 / 4-2 / 5-1 〜 5-8 | 欠番の許容と `adr.md`、導入の確かめ、文書と設定の切り替え、文書のガード | opus | auto |
| PR-4 | 6-1 〜 6-6 / 7-1 / 7-2 | `mergepr` の確かめ、`docs/tasks/` の形のガード、cloud の記録 | opus | auto |

PR の分け方の理由は §1.3 の決定 1 にある。利用者の手作業（ステップ 2-4・5-1・7-1）は、どれも実行中の `/runplan` が依頼する。前の PR のマージと次の `/runplan` の間に置く手作業はないので、どの PR も `auto` にする。

## 4. テスト戦略 (Test Strategy)

-   **ユニットテスト:** `internal/tasknaming`・`internal/numreserve`・`cmd/numreserve`・`internal/mergepr`・`cmd/mergepr` は、外部のコマンドと部品を fake で差し替えて確かめる（設計書 7.1）。テストは `gh` と `git` を起動せず、実の GitHub に触れない。外部の出力を読む部分の入力は、フェーズ 2 で採った実のサンプルから作る（§1.3 の決定 9）。
-   **上限と環境:** 上限の値と git・gh の環境は純粋な関数の値として、時間の上限は呼び出しごとの `ctx` の期限として確かめる。`Fake` は上限を課さないためである（ステップ 3-4）。上限は、ちょうど上限と 1 つ超えを対にする（ステップ 3-6）。
-   **本番の `Tool` の拒否:** テストの中で git と gh を呼ばないことを、`PATH` と認証の変数を外した環境で確かめる（ステップ 3-4）。
-   **振る舞いを変えない移し替え:** `chief` と `TestADRRecords` の既存のテストを変えずに通すことで確かめる（PR-1）。
-   **文書のガード:** 呼び出しの形、settings の許可、手引きの止める理由の表と ruleset の対象、`Makefile` の対象を、道具の定義と照らす（ステップ 3-7・5-7）。`CLAUDE.md` の「Testing Strategy」の信頼の境界の例外に当たるので、文言ではなく呼び出しの形と値を確かめる。
-   **`docs/tasks/` の形:** 判定の関数を表のテストで確かめ、実のディレクトリにも当てる（ステップ 6-3）。CI の `doc-guard` のジョブでも動く。
-   **実地の確かめ（manual）:** REST の前提、SHA による取得、cloud の到達性（フェーズ 2）、記録の ruleset（ステップ 5-1）、cloud のセッションでの番号の決定（フェーズ 7）。何を確かめれば通ったとするかは各ステップに書き、結果は §7 に記録する。
-   **後方互換:** 既存の番号とブランチ名・ディレクトリ名の形を変えない（AC-04）。この仕組みの導入より前に作られた PR は、しきい値で記録の確かめから外す（ステップ 6-1）。
-   **壊して確かめる:** 新しいテストは、どれも確かめる判定を壊して失敗することを確かめ、コミットメッセージに書く（`CLAUDE.md` の「Testing Strategy」）。

## 5. リスク管理 (Risk Management)

| # | リスク | 対策 |
|---|---|---|
| 1 | REST の前提が崩れる（すでにある ref の作成が 2xx になる） | フェーズ 2 で、道具を作る前に確かめる。崩れたらフェーズ 3 に進まず、設計書を直す（ステップ 2-5） |
| 2 | cloud のセッションで `gh` や道具が使えない | 制約として手引きと ADR-0051 に書く（要件書 AC-22）。番号の決定は止まり、推測の番号では進まない |
| 3 | 記録の ruleset の対象（`**/*`）が、実際の名前の形に当たらない | 導入の確かめで、実際の名前の記録の tag の削除が拒まれることを確かめる。消えたら直ちに作り直す（ステップ 5-1） |
| 4 | タスク 0099 も `mergepr` の `Prepare`・`Merge` を変える | PR-4 の実装のときに `main` を取り込んで衝突を解く。0099 が `mergepr` に `--protocol` を入れていたら、ADR-0014 の規則で番号を上げる（ステップ 6-5） |
| 5 | しきい値を決めてから PR-4 のマージまでの間に、古い手順で番号が使われる | しきい値より大きい番号は記録の確かめで、しきい値以下の番号は重複とほかのブランチの確かめで見つける（設計書 3.6） |
| 6 | しきい値を数え間違える（大きすぎると AC-27 が弱まり、小さすぎると作業中の PR が止まる） | 使った操作と出力をすべて §7 に残し、PR-4 のレビューで読み直す（ステップ 6-1） |
| 7 | 導入の確かめと cloud の確かめで、番号が欠番として残る | 欠番は誤りではない（要件書 4.3）。数は空間ごとに 2 つ以下に限る（§1.3 の決定 7、ステップ 7-1） |
| 8 | テストの壊して確かめる作業で、本番の `Tool` が実の GitHub に届く | `PATH` と認証の変数を外した環境で行う（ステップ 3-4） |
| 9 | 古い `mergepr` のバイナリが確かめなしにマージする | PR-4 のマージの後に、利用者が `make install-mergepr` を実行し直す（§9） |
| 10 | PR-2 が大きく（前提の確かめ・道具・CLAUDE.md・ADR）、レビューの負担が重い | 利用者と決めた 4 つの PR の分け方を保つ（§1.3 の決定 1）。ステップごとにテストと完了の基準を置き、レビュー観点を 4 つに絞る。前提が崩れたら PR を作らずに止まる（ステップ 2-5） |

日程のリスク：PR-2〜PR-4 は、`/runplan` の中で利用者の手作業（cloud のセッション、導入の確かめ）を待つ。

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5 / 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8）
- [ ] PR-3 マージ済み（対象ステップ: 4-1 / 4-2 / 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 / 5-8）
- [ ] PR-4 マージ済み（対象ステップ: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 7-1 / 7-2）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

テストの名前は、実装のときに決める。下の表の `path::TestName` のうち新しいものは、置き場所と確かめる内容を指定する。実装で名前が変わったら、`/runplan` がこの表を直す。

| AC | 種類 | 検証 | 実装 |
|---|---|---|---|
| AC-01 | test | `internal/numreserve/reserve_test.go::TestReserveCreateOutcomes`（`POST` が 422 で `GET` が 200 のとき次の番号を試し、2 つの実行が同じ番号を返さない） | ステップ 3-3 |
| AC-01 | manual | §7 の「PR-2 の実地の確かめの記録」（すでにある ref の作成が 2xx にならない） | ステップ 2-1 |
| AC-02 | test | `internal/numreserve/collect_test.go::TestCollectEachInputRaisesCandidate`（タスクの空間の各入力が、それだけで候補を押し上げる。`Fake` が `gh pr list` を含む台本にない呼び出しで失敗する） | ステップ 3-2 |
| AC-03 | test | `internal/numreserve/collect_test.go::TestCollectEachInputRaisesCandidate`（記録の tag だけがある番号を避ける） | ステップ 3-2 |
| AC-03 | manual | §7 の「導入の確かめの記録」（記録の tag の削除が拒まれる） | ステップ 5-1 |
| AC-04 | test | `internal/tasknaming/tasknaming_test.go::TestParseBranch`・`::TestParseTaskDir`、`internal/chief/task_test.go::TestBranchTask`・`::TestTasksRead`（変えずに通す） | ステップ 1-1・1-2 |
| AC-05 | static | `cmd/numreserve/docs_test.go::TestDocumentedInvocations`（`mkreq.md` に `numreserve task` の呼び出しがある） | ステップ 5-2・5-7 |
| AC-05 | manual | PR-3 のレビューで、`mkreq.md` の step 2 にエージェントが番号を計算する手順が残っていないことを読む | ステップ 5-2 |
| AC-06 | test | `cmd/numreserve/main_test.go::TestRunStops`（すべての `StopKind` で終了コード 1、標準出力が空、標準エラーに理由の語）、`internal/numreserve/reserve_test.go::TestReserveCreateOutcomes`（状態コードごとの止める理由）、`internal/numreserve/collect_test.go::TestCollectRejects`（受け入れない形と一覧の上限） | ステップ 3-2・3-3・3-5 |
| AC-06 | static | `cmd/numreserve/docs_test.go::TestGuideValuesMatchTool`（手引きの表の語が `StopKind` の語と一致する） | ステップ 5-6・5-7 |
| AC-06 | manual | PR-3 のレビューで、`mkreq.md` が止まったときにブランチもディレクトリも作らず、理由と手引きの節を示すと書いていることを読む | ステップ 5-2 |
| AC-07 | test | `internal/numreserve/reserve_test.go::TestReserveLimits`（`POST` の 10 回目で決まり、11 回目は試さずに `attempts`） | ステップ 3-3 |
| AC-08 | test | `internal/numreserve/reserve_test.go::TestReserveLimits`（候補 9999 で決まり、10000 で `exhausted`、跳びと同時なら `jump`） | ステップ 3-3 |
| AC-09 | test | `internal/numreserve/reserve_test.go::TestReserveLimits`（基準 + 20 で決まり、+ 21 で `jump`、詳細に原因のブランチ名とファイルのパス） | ステップ 3-3 |
| AC-10 | test | `internal/numreserve/reserve_test.go::TestReserveCreateOutcomes`（ADR の空間）、`internal/numreserve/collect_test.go::TestCollectEachInputRaisesCandidate`（ほかのブランチにしかない ADR の番号を避ける） | ステップ 3-2・3-3 |
| AC-11 | test | `internal/numreserve/collect_test.go::TestCollectEachInputRaisesCandidate`（ADR の空間の各入力。作業ツリーだけにあるファイルを含む） | ステップ 3-2 |
| AC-12 | static | `cmd/numreserve/docs_test.go::TestDocumentedInvocations`（`adr.md` の「Location and naming」に `numreserve adr` の呼び出しがある） | ステップ 4-2・5-7 |
| AC-12 | manual | PR-3 のレビューで、規約がコマンドの PR にも手で作る PR にも当てはまると書き、`_lib/settle-decisions.md` が規約に従うと書いていることを読む | ステップ 4-2・5-3 |
| AC-13 | static | `cmd/numreserve/docs_test.go::TestDocumentedInvocations`（「Taking in `main`」の手順 2 と「When `TestADRRecords` fails on `main`」の段落の両方に `numreserve adr` の呼び出しがある） | ステップ 4-2・5-7 |
| AC-14 | test | `internal/numreserve/reserve_test.go::TestReserveLimits`（ADR の空間でも同じ止まり方、跳びの基準に `origin/main` の `docs/dev/adr/` を使う） | ステップ 3-3 |
| AC-14 | manual | PR-3 のレビューで、`adr.md` が、道具が止まったら推測の番号でファイルを作らず付け替えもしないと書いていることを読む | ステップ 4-2 |
| AC-15 | test | `internal/numreserve/collect_test.go::TestCollectSpacesIndependent`（ある空間の記録の tag が、もう一方の空間の候補を変えない） | ステップ 3-2 |
| AC-16 | test | `cmd/yt2column/adr_test.go::TestCheckADRRecords`（「missing number」のケースが issue を 1 つも返さない） | ステップ 4-1 |
| AC-17 | test | `cmd/yt2column/adr_test.go::TestCheckADRRecords`（「duplicate file number」のケース） | ステップ 4-1 |
| AC-18 | diff | 差分の読み取り（recorded diff reading）。下の「Diff record (AC-18)」。比べるパスは `docs/dev/conventions/adr.md` | ステップ 4-2 |
| AC-19 | static | `cmd/numreserve/docs_test.go::TestDocumentedInvocations`（「Tool Execution Safety」に 2 つの形の呼び出しがあり、どれも道具が受け付ける形） | ステップ 5-4・5-7 |
| AC-19 | manual | PR-3 のレビューで、道具が `origin` に記録の tag を 1 つ作ることが書かれていることを読む | ステップ 5-4 |
| AC-20 | test | `cmd/numreserve/docs_test.go::TestSettingsAllowsTool`（settings の `numreserve` の規則が 2 つの形だけ。規則の分け方の表を含む）、`::TestDocumentedInvocations`（すべての呼び出しが単独のコマンドで、引数が `ParseSpace` の受け付ける 1 語）、`cmd/codexreview/docs_test.go::TestSettingsAllowsTool`（ほかの語の規則で道具を介さずにコマンドを許さない）・`::TestMayBypassTool`（2 つの規則が通る行） | ステップ 5-5・5-7 |
| AC-21 | manual | PR-3 のレビューで、「Tool Execution Safety」の記録の書き換えと ruleset の変更の承認の文を読む | ステップ 5-4 |
| AC-22 | manual | §7 の「PR-2 の実地の確かめの記録」（cloud の到達性）と「cloud のセッションでの確かめの記録」。ADR-0051 の「結果」と手引きの記述を PR-4 のレビューで読む | ステップ 2-4・7-1・7-2 |
| AC-23 | static | `cmd/yt2column/adr_test.go::TestADRRecords`（すべての ADR に索引の行がある。既存のテスト） | 既存 |
| AC-24 | manual | 各 PR のレビューで、ADR-0051 の食い違いの記述がその PR の直した分を除き、残りをすべて挙げていることを読む | ステップ 3-8・5-8・6-6・7-2 |
| AC-25 | manual | ADR-0051 の「決定」と「採らなかった案」は、設計書の PR（#943）で書いた。PR-4 のレビューで、設計書 1.2 と一致していることを読み直す | #943 で実装済み |
| AC-26 | static | `cmd/numreserve/docs_test.go::TestGuideValuesMatchTool`（止める理由の語と ruleset の対象の形） | ステップ 5-6・5-7 |
| AC-26 | manual | PR-3 のレビューで、手引きの 6 項目（設計書 3.9）を読む。PR-4 のレビューで、マージの前の確かめの直し方と cloud の制約を読む | ステップ 5-6・6-6・7-2 |
| AC-27 | test | `internal/numreserve/check_test.go::TestCheckAdded`（しきい値より大きく記録のない番号を `AddedUnreserved` で返し、しきい値以下なら返さない）、`internal/mergepr/mergepr_test.go::TestPrepareNumberCheck`・`::TestMergeNumberCheck`（見つけたら止まる）、`cmd/mergepr/main_test.go::TestRunPrintsNumberIssues`（番号と理由を示す） | ステップ 6-2・6-4・6-5 |
| AC-28 | test | `internal/numreserve/check_test.go::TestCheckAdded`（最新の `origin/main` の別のもの、ほかのブランチの別の名前を返す）、`internal/mergepr/mergepr_test.go::TestMergeNumberCheck`（`gh pr merge` の直前に確かめる）、`cmd/mergepr/main_test.go::TestRunPrintsNumberIssues`（使っているディレクトリかファイルを示す） | ステップ 6-2・6-4・6-5 |
| AC-29 | test | `internal/numreserve/check_test.go::TestCheckAdded`（`origin/main` にすでにある番号と、`main` で名前を変えたものが古いヘッドに残る PR を返さない。番号を加えない PR では何も読まない） | ステップ 6-2 |

Diff record (AC-18):

- Compared paths: `docs/dev/conventions/adr.md`

### PR-2 の実地の確かめの記録

（ステップ 2-1〜2-5 で書く）

### 導入の確かめの記録

（ステップ 5-1 で書く）

### しきい値の記録

（ステップ 6-1 で書く）

### cloud のセッションでの確かめの記録

（ステップ 7-1 で書く）

## 8. 成功の基準 (Success Criteria)

-   **機能:** §7 の AC-01〜AC-29 のすべての検証が済み、§6 の 4 つの PR がマージされている。
-   **品質:** `make test`・`make lint`・`make deadcode` が各 PR で通る。新しいテストは、どれも壊して失敗することを確かめてある。
-   **セキュリティ:** 設計書 5.2 の表の守りが、設計書 7.4 のテストで確かめてある。道具の出力に、応答の本文、`gh` と `git` の標準エラー、`origin` の URL、認証の情報が出ない。
-   **文書:** 手引き、`mergepr_guide.md`、`CLAUDE.md`、`adr.md`、ADR-0051・ADR-0014 が、道具の実際の振る舞いと合っている。ADR-0051 と ADR-0014 の食い違いの記述がない。

## 9. 次のステップ (Next Steps)

-   PR-4 のマージの後、利用者が `make install-mergepr` を実行し直す（設計書 5.2 の残る危険）。
-   すべての PR のマージの後、`/runpr` が issue #917 を閉じる。
