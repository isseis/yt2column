# 実装計画書：拡張だけの経路への移行と、サーバとホストの撤去

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-16 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。「0090 設計書」は [タスク 0090 の設計書](../0090_extension_only_architecture/02_architecture.md) を指し、設計書と同じく手段と値の正本とする。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `bac09b3f` で確かめた。

用語は、要件書 §6、設計書 §1、0090 設計書 §1 と同じ意味で使う。本計画は、次の語を加える。

-   **切り替えの PR:** 拡張がサーバとホストを使わなくなる PR（PR-8）。
-   **撤去の PR:** サーバ、ホスト、開発用のサーバと、それだけが使うコードを除く PR（PR-9）。
-   **まだ使われない拡張の部品:** 切り替えの PR より前に加え、切り替えの PR で入口から呼ばれるようになる拡張のモジュール（PR-6・PR-7）。入口のページのスクリプト（`popup.ts`・`viewer.ts`・`background.ts`・`options.ts`）と manifest は、切り替えの PR まで変えない。
-   **試作の拡張:** 手動の確認のためだけに、リポジトリの外（作業者の一時のディレクトリ）に作る拡張（§1.3 の決定 3）。リポジトリに入れない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書と 0090 設計書のとおりに、記事生成モジュール（`cmd/yt2column-wasm`・`cmd/yt2column-wasm-dev`）、`internal/extjob` と 2 つの LLM の実装、開発用のコマンド `cmd/yt2column-extjob`、Go を呼ぶ拡張のビルドと開発用のビルド、オプションページ、ジョブの記録と SW の調停、実行するビューアを作り、拡張だけの経路に切り替える。その後、サーバ、ホスト、開発用のサーバを撤去し、仕様の文書と ADR を拡張だけの経路に合わせる。要件書の F-001〜F-007 を満たす。

### 1.2. 実装の原則

-   **切り替えより前の PR は、今の経路を壊さない。** PR-1〜PR-7 は、Go の部品、拡張のビルド、まだ使われない拡張の部品を加えるだけにする。入口のページのスクリプトと manifest を変えず、利用者から見える振る舞いは変わらない（設計書 §1.1）。新旧の経路を併存させる仕組みは作らない（要件書 AC-32）。
-   **0090 設計書と設計書を正本とし、計画は写さない。** 各ステップは、作るものと確かめることと、その正本の節を挙げる。関数の形、値の集合、期限の値は、正本の節を読んで実装する。
-   **各文書は、それが述べるコードを変える PR で書き換える（要件書 AC-33）。** 新しいパッケージの `package_reference.md` の行は、そのパッケージを加える PR で書く（`TestPackageReferenceListsPackages` が確かめる）。
-   **ADR の食い違いの記述は、食い違いを解いた PR で除く（要件書 AC-35）。** 一部だけを解いた PR は、残る食い違いを述べる形に書き換える。どの PR がどの ADR の記述を除くかは §3.3 にある。除くことと書き換えることは、`docs/dev/conventions/adr.md` の「Editorial corrections」の 2 つ目の種類で、commit のメッセージの訂正の行と、PR の説明の訂正の段落に書く。
-   **テストの削除は、削除だけの commit でカバレッジを比べる（CLAUDE.md「Testing Strategy」）。** Go のテストの削除は `make coverage-compare`、拡張のテストの削除は `make ext-coverage-compare` を、その commit の親を `BASE` にして走らせ、出力を commit のメッセージと PR の説明に書く。
-   **各テストは、壊して失敗を確かめる。** 確かめた結果を commit のメッセージに書く（CLAUDE.md「Testing Strategy」）。
-   **実 API の呼び出しとネットワークを使う操作は、その都度、人の承認を得る（CLAUDE.md「Tool Execution Safety」）。** PR-6 の 401 の確かめと、PR-8 の本番の構成での確かめが当たる。拡張のビルドが `go.mod` の版のツールチェーンを初めてダウンロードするときも当たる（設計書 §1.2）。

### 1.3. 計画の際に決めたこと（2026-11-16）

1.  **PR を 9 つに分け、まだ使われない拡張の部品を、切り替えより前の 2 つの PR に入れる。** PR-6 は、Worker の呼び出し、Worker から受け取る結果の検査、ジョブの記録の検査、ページのタイトルを決める関数、オプションページの表示を決める関数を加える。PR-7 は、`chrome.storage` の決まった関数と lint の規則、API キーの経路のソースの検査、新しい SW の調停、実行するビューアの流れ、オプションページの保存と削除の流れを加える。切り替えの PR（PR-8）は、入口の配線、HTML、manifest、ホストとの送信の撤去、文書を受け持つ。理由: 切り替えの PR には、配線と撤去に加えて、README を含む 8 つの文書の書き換えが入る（要件書 AC-33）。部品を先に入れれば、切り替えの PR のレビューを配線と文書に絞れる。配線のときに部品の形の誤りが見つかると直しが要るが、部品は単体テストで振る舞いを固めてから入る。採らなかった案は、部品を 1 つの PR にまとめる案（SW の調停と実行するビューアが切り替えの PR に残る）と、前倒ししない案（切り替えの PR がレビューできない大きさになる）である。ADR: なし（PR の区切りの決定で、本タスクの中で閉じる）。
2.  **切り替えと撤去を別の PR にし、切り替えの PR で、サーバとホストを述べる文書の節と、その節を読む文書のガードのテストを除く。** 除くテストは、`TestServerDocsContract`（`cmd/yt2column-server/docs_test.go:36`）、`TestReadmeHostExitCodes`（`cmd/yt2column-nmhost/main_test.go:439`）、`TestDevelopmentDocsDevServer`（`cmd/yt2column-devserver/docs_test.go:29`）、`TestNativeHostContract`（`cmd/yt2column/extension_docs_test.go:159`）、`extension/test/serverReasons.test.ts` と、それだけが使う補助である。サーバとホストのコードは撤去の PR まで残るが、どの文書も述べない。README には、移行の手順だけが残る（要件書 AC-34）。理由: AC-33 の例外を使って節を「撤去の予定のもの」として残すと、利用者が読む README に、切り替えから撤去までの間、使えない手順が残る。1 つの PR にすると、Go と拡張のテストの削除と 8 つの文書が同じ PR に入り、レビューできない。ADR: なし（PR の区切りの決定で、本タスクの中で閉じる）。
3.  **401 を読めるかの確認と、履歴のタイトルが上書きされるかの確認は、試作の拡張で、PR-5 のマージの後、PR-6 の `/runplan` の前に、人と対話のセッションで行う。** 試作の拡張はリポジトリの外に作り、PR-5 までに `main` に入った記事生成モジュールを使う。結果は §7 の「PR-6 の前の手動の確認の記録」に書き、PR-6 のブランチに commit する。PR-6 の自動送信は `hold` にする。理由: どちらの結果も、PR-6 のページのタイトルの関数と、PR-7 以降のオプションページの流れの前提になる（設計書 §1.2・§3.8.2・§8 の 3）。確かめのためだけのコードを `main` に入れると、成果物の検査と lint の対象が増え、撤去も要る。ADR: なし（確かめの手段と時期の決定で、本タスクの中で閉じる）。
4.  **Worker のファイルの名前を、`extension/src/wasmWorker.ts`（Worker の中の読み込み）と `extension/src/wasmWorkerClient.ts`（ページの側の呼び出し）にする。** 設計書 §3.12 の `worker.ts`・`workerClient.ts` に当たる（設計書は「または同等のもの」とした）。理由: 今のコードの「Worker」は SW を指す（`createWorkerSend`、`extension/src/browser/chromeDeps.ts:320`、`pendingWorker`、`extension/test/helpers/chrome.ts:131`）。ADR: なし（ファイルの名前の決定である）。
5.  **新しい SW の調停と、実行するビューアの流れと、オプションページの保存と削除の流れは、新しいファイル（`extension/src/jobCoordinator.ts`・`extension/src/viewerRunner.ts`・`extension/src/optionsFlow.ts`）に作り、切り替えの PR で今のファイル（`startCoordinator.ts` など）を除く。** 0090 設計書 §3.11 の「`startCoordinator.ts`・`background.ts` を変える」に当たる。理由: 決定 1 で、まだ使われない部品を切り替えより前に入れるためである。今のファイルを書き換えると、今の経路が壊れる。ページの間のメッセージの新しい型も、`extension/src/core/jobMessages.ts` に作る（今の `extensionMessages.ts` は撤去する `exchange.ts`・`response.ts` を import するため）。ADR: なし（ファイルの分け方の決定である）。
6.  **拡張の検査を走らせる変更の一覧から `internal/server/response.go` を除くのは、`extension/test/serverReasons.test.ts` を除く切り替えの PR（PR-8）にする。** 設計書 §3.10 は「それと `serverReasons.test.ts` を撤去する PR」とした。この一覧に `response.go` があるのは、`serverReasons.test.ts` が、`response.go` の拒否の理由を写した development.md の表を読むためである。決定 2 で、このテストを切り替えの PR で除くので、拡張の側の依存はそこで消える。ADR: なし（設計書 §3.10 の条件を、決定 2 の区切りに当てたもの）。
7.  **今のサーバのテストの行は、PR-3 で `internal/extjob` のテストに写し、撤去の PR（PR-9）までは両方を走らせる（設計書 §1.2、H-03）。** 写した行の対応（サーバのテストの名前 → `internal/extjob` のテストの名前）を、PR-3 の説明に表で書く。撤去までの間、サーバの関数と `internal/extjob` の関数は重複する。サーバは使われなくなるので、重複を共有にする作業はしない。ADR: なし（設計書の決定を当てるだけである）。
8.  **AC-26 は、各 PR で、`docs/tasks/0091_extension_only_migration/_checks/cli_unchanged.sh` を走らせて確かめる。** 要件書の「`cmd/yt2column/` の `_test.go` でないファイルの差分はない」は、変更の前後の関係を述べる基準である。スクリプトは PR-1 で作り、各 PR のブランチで走らせ、出力と 2 つのコミット ID を §7 に書く。ADR: なし（受け入れ基準の確かめ方の決定である）。
9.  **撤去の PR は、ADR の決定の文のうち、撤去を未来のこととして #773 にリンクする 2 か所を、撤去した後の形に直す。** ADR-0045 の「撤去と置き換えは #773 が行う」（`docs/dev/adr/0045-dev-fake-llm.md:25`）と、ADR-0048 の「結果」の「#773 で撤去する」（`docs/dev/adr/0048-extension-only-generation.md:48`）である。どちらも食い違いの記述ではないが、撤去の後は事実と合わなくなる。決定は変わらないので、編集上の訂正として行う。廃止した ADR-0005・ADR-0018 の「廃止の理由」の行は、廃止の時点の記録なので直さない。ADR: なし（編集上の訂正である）。

### 1.4. 既存コード調査結果

コミット `bac09b3f` で確かめた。設計書 §3 が挙げる箇所のほかに、計画に効くものを挙げる。

**Go の LLM の層と設定**

-   `internal/llm/llm.go:12-19` の番兵は `ErrInvalidRequest`・`ErrTruncated`・`ErrUnexpectedFinishReason`・`ErrEmptyResponse` で、`LLMClient` の doc コメント（`:65-75`）がそれを挙げる。`ErrAuthRejected` はない。
-   `internal/llm/claude/errors.go:33` の `HTTPStatusError` は `StatusCode` だけを持ち、`Unwrap` で `ErrHTTPStatus` を返す（`:44`）。直接の単体テストはなく、`claude_test.go::TestGenerateHTTPStatus`（`:500`）が送信を通して確かめる。
-   `internal/llm/llmhttp/llmhttp.go` は `CheckAPIKey`（`:46`）、`Call`（`:81`）、`Post`（`:94`）を持つ。`Status` はない。`js` のビルドだけのファイルは、`internal/llm/claude` にも `internal/llm/llmhttp` にもない（リポジトリで唯一の `//go:build js` は `internal/writer/template_open_js.go`）。
-   `internal/llm/provider/provider.go:25` の `LLMTimeout` は 15 分で、Claude のクライアントは `:93-104` で組み立てる。`provider` は `internal/config` を通して `internal/privatefile` に依存するので、WASM でビルドできない。
-   `GOOS=js GOARCH=wasm go build ./internal/config` は、`internal/config/serverfile.go:13` の `internal/privatefile` の import（`privatefile.go:1` が `//go:build unix`）で失敗する（`bac09b3f` で走らせて確かめた）。`serverfile.go`・`configpath.go`・`socketpath.go` はビルドタグを持たない。`LoadServer`（`config.go:370`）・`LoadHost`（`:425`）・`LoadHostHome`（`:438`）は `config.go` にある。
-   サーバの設定の読み込みは、サーバのほかからも使われる。`config.LoadServer` と `ServerLookupForTest`（`internal/config/test_helpers.go:194`）は `cmd/yt2column/docs_test.go:558,649` と `internal/llm/provider/provider_test.go:1058` が、`ParseServerFileForTest`（`:130`）は `cmd/yt2column/docs_test.go:499` が、`LoadServer` は CLI の名前のテスト `TestLoadClaudeEffortDefault`（`config_test.go:302`）と `TestLoadHTTP2Debug`（`:1006`）が使う。撤去の PR で、これらを CLI の読み込みに改める。
-   `TestFakesCarryBuildTag`（`internal/pipeline/pipeline_test.go:554`）は、`test_helpers*.go` の 1 行目に `//go:build test` か `//go:build test || integration` だけを許す。テストの補助のファイルに `!js` を付けられないので、`internal/config` の補助は動かさない（WASM のビルドはテストを含まない）。

**fake と開発用のサーバ**

-   `internal/llm/devfake/scenario.go:171` の `Scenario` は公開しないフィールドを持ち、`NewScenario`（`:185`）だけが作る。`Failure` の値は `:42-48`、綴りは `:77-92`、`ErrSimulatedAPIFailure` は `:22`。キーの確かめの結果と `ParseScenario` はない。
-   開発用のサーバの引数の解析は `cmd/yt2column-devserver/args.go:130` の `parseArgs` で、テストは `run_test.go::TestRunArgsAccepted`（`:58`）・`TestRunArgsRejected`（`:88`）。`TestDevelopmentDocsDevServer`（`docs_test.go:29`）が、development.md の受理する値の表を `devfake` の値と照らし合わせる。`Failure` に値を足すと、この表も変わる。
-   `internal/llm/devfake/deps_test.go:10-16` は `internal/server`・`articlecache`・`serversock`・`nmhost`・`privatefile` を import するので、撤去の PR で一覧を直さないとコンパイルできない。

**サーバの処理とテスト（`internal/extjob` に写すもの）**

-   `internal/server/request.go:63` の `validateBody`、`generation.go:66` の `completeGeneration`、`:146-163` の `classifyWrite`（順は `writer.ErrPromptTooLarge` → `context.DeadlineExceeded` → `llm.ErrEmptyResponse` → `llm.ErrTruncated` → 内部の失敗 → `FailureLLM`）、`status.go:131` の `articleStatus`、`internal/articlecache/key.go:81` の `Keyer.Key`。
-   写すテストは `request_test.go`（382 行、7 つのテスト）、`status_test.go`（404 行、9 つ）、`generation_test.go`（442 行、9 つ）で、補助は `internal/server/test_helpers.go` にある。

**依存の検査とガード**

-   `internal/godepstest` は darwin と linux だけを対象にする。使い手は `cmd/yt2column`・`cmd/yt2column-server`・`cmd/yt2column-nmhost` の `devdeps_test.go::TestNoDevDependency` で、どれも `devfake` と `./cmd/yt2column-devserver` を許さない。`godepstest_test.go:48,78` は `internal/server` を例に使う。
-   `internal/config/envaccess_test.go:65-91` の `allowedEnvRefs` は、サーバ・ホスト・開発用のサーバの `main.go` を含む。
-   `internal/pipeline/relay_guard_test.go` の `hostFiles`（`:405`）は、ホストのファイルがないと `t.Fatal` する。`TestHostStdoutOnlyInMain`（`:492`）と `TestHostSpawnsNoProcess`（`:507`）は、撤去の PR でホストとともに除く。`TestSharedTokenRemoved`（`:328`）は拡張のソースも調べ、残る。
-   `TestPackageReferenceStrictjsonUsers`（`internal/pipeline/strictjson_users_test.go:657`）は、package_reference.md の `internal/strictjson` の利用者の行（今は `internal/server` と `internal/nmhost` を含む）を import と照らし合わせる。
-   `cmd/yt2column/makefile_test.go` の `TestMakeBuildBuildsEveryBinary`（`:257`）と `TestMakeBuildDev`（`:280`）は、`make build` の 3 つのバイナリと `build-dev` を固定している。`TestExtensionDocumentation`（`extension_docs_test.go:56`）は、`ext-` の目標がすべて CLAUDE.md にあることと、security.md の `### 8.1. ブラウザ拡張（#110）` の `**要求する権限**` の一覧が manifest と同じであることを確かめる。
-   `Makefile:328-333` の `wasm-check` は 5 つのパッケージをビルドし、`internal/writer` と `internal/transcript` を `vet` する。`make lint` は `wasm-check` を含む（`:318`）。`make deadcode` は `-test -tags test,integration ./...` で、テストからだけ呼ばれる関数も到達できるものに数える（`:372`）。
-   `.github/workflows/ci.yml` の `extension` の手順は、`ext-build` の後で Go を用意する（`:154-163`）。`extension/test/repository.test.ts:161-174` が手順の順を固定する。`doc-guard`（`:191`）のコマンドは、CLAUDE.md:703 と同じ 7 つのパッケージで、`.claude/commands/_lib/task-pr-handoff.md:40`・`.claude/commands/_lib/simple-process.md:429`・`docs/dev/guides/fixprguard_guide.md:595` の写しは、それぞれ少しずつ違う。写しが同じであることを確かめるテストはない。
-   `.golangci.yml` は、撤去するパッケージを名指す規則を 6 つ持つ（`:91-162`）。

**拡張**

-   設計書と本計画が加えるファイル（オプションページ、Worker、記録、結果、タイトル、ビルドの種類）は、どれもまだない。Worker と WebAssembly を参照するコードもない。
-   `extension/eslint.config.js` は、`storage`（`:44-52`）、`session`（`:94-102` の `menuPathNames`）、`fetch` などの送信（`:58-82`）を、`chromeDeps.ts` を含むすべてのファイルで拒否する。後の設定が `no-restricted-syntax` の一覧を丸ごと置き換えるので、例外はファイルごとの設定で一覧を組み直す（`:175-177`）。`no-restricted-imports` は使っていない。`extension/test/lintRules.test.ts` は ESLint を呼んで規則を確かめる。
-   `extension/test/secretsAbsent.test.ts` は、`src/` と `static/` の中の「api key」「token」などの語を拒否する（`:14-45`）。en.ts と ja.ts の文字列だけに、`SLACK_WEBHOOK_URL` を許す例外がある（`:31-32`）。
-   `extension/scripts/check-dist.ts` は、`dist/` を、`src/` の `.ts` ごとの `.js` と `static/` の写しだけに限る（`:50-62`）。manifest の参照は `action.default_popup` と `background.service_worker` だけを確かめる（`:130-133`）。
-   `extension/tsconfig.json:4` の `lib` は `["ES2022","ES2023.Intl","DOM"]` で、WebWorker を含まない。
-   `extension/test/manifest.test.ts` は、権限が 3 つであること（`:96-103`）、最上位のキーの集合（`:107-122`）、`minimum_chrome_version` が 110 で両方の README がその値を述べることを確かめる。`metadataDocs.test.ts` は README の `## ブラウザ拡張` の節の語を、`jobCodes.test.ts` は development.md の `### ジョブの API のコード` の表を読む。
-   `extension/scripts/has-extension-changes.sh:20` は、`internal/server/response.go` を拡張の検査を走らせる変更に数え、`extension/test/ciChanges.test.ts` がそれを固定する。
-   今の SW の調停（`extension/src/startCoordinator.ts`、462 行）は、メモリの中の集合と期限だけで直列にしている（`:167-203`）。

**文書**

-   README の `## localhost サーバ`（`README.md:128-392`）と `## ブラウザ拡張`（`:498-`）、development.md の 3 つのサーバの節（`docs/dev/guides/development.md:26,54,102`）、security.md の §8（`docs/dev/security.md:99-181`）、project_overview.md の `:151`（`TestServerDocsContract` が読む）が、サーバとホストを今の経路として述べる。`TestReadmeLinks`（`cmd/yt2column/docs_test.go:697`）は、`development.md:105,210` と `cli.md:234` から README のサーバの節へのリンクを確かめるので、節とリンクを同じ commit で直す。
-   `TestProductionProfileDocsContract`（`cmd/yt2column/docs_test.go:666`）は、README の `### 推奨構成` と production_profile.md の `## 本番の構成` の塊を、サーバの設定ファイルの解析（`ParseServerFileForTest`）で読む。

### 1.5. 撤去と置き換えの名前の表

本タスクで名前を替えるか、除いて別の名前に置き換えるものを挙げる。ほかの節は、この表に従う。

| 今の名前 | 後の名前 | PR |
|---|---|---|
| `extension/src/startCoordinator.ts` | `extension/src/jobCoordinator.ts`（新しい調停） | PR-7 で加え、PR-8 で今のものを除く |
| `extension/src/core/extensionMessages.ts` の生成の開始の型 | `extension/src/core/jobMessages.ts` | PR-7 で加え、PR-8 で今の型を除く |
| `extension/test/secretsAbsent.test.ts` | `extension/test/apiKeyPath.test.ts` | PR-7 |
| 記事のキー（`extension/src/core/articleKey.ts`） | ジョブ ID | PR-8 |
| 設計書の `worker.ts`・`workerClient.ts` | `wasmWorker.ts`・`wasmWorkerClient.ts` | PR-6 |

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: LLM の失敗の分類と、キーの確かめ（設計書 §8 の 1 のうち LLM の層）

**変更するファイル:** `internal/llm/llm.go`、`internal/llm/claude/`（`errors.go`、キーの確かめのファイル、`js` のビルドだけのファイル）、`internal/llm/llmhttp/`（`llmhttp.go`、`js` のビルドだけのファイル）とそのテスト、`Makefile`、`docs/dev/conventions/package_reference.md`、`docs/tasks/0091_extension_only_migration/_checks/cli_unchanged.sh`（新規）

-   [ ] **ステップ 1-1**: `llm.ErrAuthRejected` を加え、`LLMClient` の doc コメントの番兵の一覧を改める（設計書 §3.1）。Claude アダプタの状態 401 の `HTTPStatusError` が `errors.Is(err, llm.ErrAuthRejected)` に当たるようにする。`errors.AsType[*claude.HTTPStatusError]` で状態コードを取れることと、`ErrHTTPStatus` に当たることと、エラーの文は変えない（要件書 AC-26）。テストは `internal/llm/claude/errors_test.go` に置き、401 で当たり、400・403・429・500 で当たらないことを確かめる。
-   [ ] **ステップ 1-2**: `llmhttp.Status` を加え、パッケージの doc コメントを「1 回の POST」から改める（設計書 §3.1）。テストは `net/http/httptest` で、状態コードを返すこと、本文を読まずに閉じること、リダイレクトに従わないこと、期限切れと通信の失敗を `Post` と同じく分類することを確かめる。
-   [ ] **ステップ 1-3**: `js` のビルドだけのファイルを加える。`llmhttp` の送信（`Post` と `Status`）に `js.fetch:redirect: error` を、`internal/llm/claude` の `Generate` とキーの確かめのリクエストに `anthropic-dangerous-direct-browser-access` のヘッダを、同じ関数で付ける（設計書 §3.1、0090 設計書 §5.3）。ネイティブのビルドのリクエストは変わらず、今のテストがそのまま通る。`Makefile` の `WASM_PACKAGES` に `./internal/llm/llmhttp` を、`vet` の対象に `internal/llm/llmhttp` と `internal/llm/claude` を加える（設計書 §3.10）。
-   [ ] **ステップ 1-4**: `claude.CheckKey` と `KeyCheckOptions` を加える（設計書 §3.1）。`GET https://api.anthropic.com/v1/models?limit=1` を `llmhttp.Status` で 1 回送る。テストは、200 で `nil`、401 で `llm.ErrAuthRejected` に当たるエラー、ほかの状態・通信の失敗・期限切れで当たらないエラーを返すこと、ワークスペース ID を指定したときだけ `anthropic-workspace-id` を送ること、API キーを `x-api-key` だけで送ることを確かめる。
-   [ ] **ステップ 1-5**: package_reference.md の `internal/llm`（`ErrAuthRejected` と、それを返すアダプタが Claude だけであること）、`internal/llm/claude`（`CheckKey`）、`internal/llm/llmhttp`（`Status`）の行を改める（設計書 §3.13）。
-   [ ] **ステップ 1-6**: `_checks/cli_unchanged.sh` を作る（§1.3 の決定 8）。merge-base と `HEAD` の間で、`cmd/yt2column/` の `_test.go` でないファイルの差分がないことを確かめ、差分があれば失敗する。チェックが、少なくとも 1 つの壊し方で失敗することを確かめる。このブランチで走らせ、出力と 2 つのコミット ID を §7 の AC-26 の記録に書く。

**完了の条件:** `make test`・`make lint`（`make wasm-check` を含む）・`make deadcode` が通る。

### PR-1 作成ポイント: LLM auth sentinel, status-only send, and key check

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6

**推奨タイトル**: `feat(0091): キーの拒否の番兵、本文を読まない送信、Models API でのキーの確かめを加える`

**レビュー観点**: 401 だけが `ErrAuthRejected` に当たり、CLI のエラーの文と分類が変わらないか / `Status` が本文を読まずに閉じ、リダイレクトに従わないか / `js` のビルドだけのファイルが送信の指定を `Post` と `Status` の両方に付けるか

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 1-3 の `js` のビルドだけのファイルは golangci-lint とネイティブのテストに見えず、`vet` とビルドだけで確かめる送信の守りである。Sonnet が実装すると、`Status` の側にリダイレクトの指定を付け忘れても、テストが通ったまま残りやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 設定の読み込みと fake のシナリオ（設計書 §8 の 1 のうち `internal/config` と `devfake`）

**変更するファイル:** `internal/config/`（`config.go`、`!js` のタグを付けるファイル、拡張用の読み込みのファイルとそのテスト）、`internal/llm/devfake/`、`cmd/yt2column-devserver/args.go` とそのテスト、`Makefile`、`docs/dev/guides/development.md`、`docs/dev/conventions/package_reference.md`

-   [ ] **ステップ 2-1**: `LoadServer`・`LoadHost`・`LoadHostHome` と `ServerConfig`・`HostConfig` を、`//go:build !js` を付けた新しいファイルに移し、`serverfile.go`・`configpath.go`・`socketpath.go` にも同じタグを付ける（設計書 §3.2）。テストの補助のファイルにはタグを付けない（§1.4）。`GOOS=js GOARCH=wasm go build ./internal/config` が通るようになり、`Makefile` の `WASM_PACKAGES` に `./internal/config` を加える。
-   [ ] **ステップ 2-2**: `LoadExtension`・`ExtensionLookup`・`ReadExtensionSettings` を加える（設計書 §3.2、0090 設計書 §3.1.2・§3.1.3）。テストは、`LoadExtension` が 2 つの名前のほかを読まず、本番の構成になり、CLI と同じ値を拒否すること（0090 設計書 §7.1）と、`ReadExtensionSettings` が 2 つの値とあるかどうかを検査せずに返すことを確かめる。秘密情報の変数名は `internal/config` の外に書かない（`TestEnvAccessConfined`）。
-   [ ] **ステップ 2-3**: `devfake` に、失敗の種類 `FailureAPIKeyRejected`（綴りは `api_key_rejected`、エラーは `llm.ErrAuthRejected` に当たる）、キーの確かめの結果 `SettingsCheck`（`ok`・`key_rejected`・`unverified`、ゼロ値は `ok`）とそれを返す関数、`ScenarioSpelling` と `ParseScenario` を加える（設計書 §3.4）。依存は `internal/llm` と標準ライブラリのままである。テストは、追加した値の振る舞い、空の値と合わない綴りの拒否（`ErrInvalidScenario`）、補正しないことを確かめる。
-   [ ] **ステップ 2-4**: 開発用のサーバの引数の解析を `devfake.ParseScenario` に替える（設計書 §3.4）。開発用のサーバには、キーの確かめの結果の引数を加えず、`ok` を渡す。`TestRunArgsAccepted` と `TestRunArgsRejected` の綴りの行を、`ParseScenario` のテストに写す。`-fail` が `api_key_rejected` を受理するようになるので、development.md の開発用のサーバの受理する値の表を直す（`TestDevelopmentDocsDevServer` が確かめる）。
-   [ ] **ステップ 2-5**: package_reference.md の `internal/config`（3 つの関数と、`!js` のファイルが撤去までの間だけのものであること）と `internal/llm/devfake`（追加した値と `ParseScenario`）の行を改める。

**完了の条件:** `make test`・`make lint`・`make deadcode` が通る。`GOOS=js GOARCH=wasm go build ./internal/config` が通る（`make wasm-check` に含まれる）。

### PR-2 作成ポイント: extension config loading and fake scenario parsing

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5

**推奨タイトル**: `feat(0091): 拡張用の設定の読み込みと、fake のシナリオの解析を加える`

**レビュー観点**: `!js` のタグがサーバとホストの読み込みだけに付き、CLI の読み込みが変わらないか / `ExtensionLookup` と `ReadExtensionSettings` が 2 つの名前だけを扱い、変数名が `internal/config` の外に出ないか / `ParseScenario` が補正せずに拒否し、開発用のサーバの振る舞いが変わらないか

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない。宣言の移動とビルドタグ、決まった形の関数の追加で、正本の形が設計書 §3.2・§3.4 にある。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: `internal/extjob` と 2 つの LLM の実装（設計書 §8 の 1 のうち `internal/extjob`）

**変更するファイル:** `internal/extjob/`（新規）、`internal/extjob/claudebackend/`（新規）、`internal/extjob/devbackend/`（新規）とそのテスト、`Makefile`、`docs/dev/conventions/package_reference.md`、`docs/dev/project_overview.md`

-   [ ] **ステップ 3-1**: `internal/extjob` に、`Prepare`・`Prepared`・`Run`・`Result`・`Failure`・`Observer`・`Deps`・`ErrInvalidInput` を作る（0090 設計書 §2.3・§3.1.2）。入力の検査は今の `validateBody`、`inputDigest` は `Keyer.Key` から秘密の鍵を除いたもの、完了の時点の検査は `completeGeneration`、結果は `articleStatus` の規則を写す（§1.3 の決定 7）。失敗の分類は `classifyWrite` に、`llm.ErrAuthRejected` を `FailureAPIKeyRejected` にする規則を `llm.ErrEmptyResponse` などと同じ並びに加え、ほかの HTTP の状態の規則より前に置く（設計書 §3.3）。`Run` は panic を `FailureInternal` にする。
-   [ ] **ステップ 3-2**: サーバの `request_test.go`・`status_test.go`・`generation_test.go` の行を、`internal/extjob` のテスト（`prepare_test.go`・`result_test.go`・`run_test.go`）に写す（要件書 AC-19、H-03）。失敗の値は 0090 設計書 §4.1 と設計書 §4.1 の集合にする。加えて、`inputDigest` が同じ入力で同じで、どの値を変えても変わること、工程の進み具合の通知、`llm.ErrAuthRejected` を返す fake で `api_key_rejected` になることを確かめる（0090 設計書 §7.1）。写した行の対応表を PR の説明に書く。
-   [ ] **ステップ 3-3**: `Backend`・`BackendInfo`・`ClaudeInfo`・`FakeInfo`・`Module` と 4 つの JSON の入口を作る（設計書 §3.3、0090 設計書 §3.1.2）。
    -   入力の JSON は `internal/strictjson` で読み、閉じたメンバの集合に合わなければ、`prepare` は固定の理由で拒否し、`generate` は `internal`（`request_shape`）にする。
    -   保存した設定の検査（設計書 §3.3 の規則）に合わなければ、LLM の実装を呼ばずに `settings_invalid` にする。ワークスペース ID の空の文字列は、`ExtensionLookup` が未設定として答え、指定なしになる。
    -   `generate` の `inputDigest` が持っている入力と違えば `internal`（`input_mismatch`）にする。
    -   `checkSettings` は、形の検査を通った後だけ `Backend.CheckKey` を 30 秒の期限で呼び、`ok`・`rejected`・`key_rejected`・`unverified` を返す。
    -   API キーは、文字列であることを確かめた直後に `secret.Secret` にし、JSON の文字列を保持しない。結果と進み具合の JSON に API キーが入らないことをテストで確かめる。
-   [ ] **ステップ 3-4**: `claudebackend.New` を作る（設計書 §3.3）。本番の構成の Claude のクライアントと `claude.CheckKey` を使い、呼び出しごとの期限 15 分を持つ。テストは、クライアントの組み立て（`claude.Options` の値、検証・推敲のクライアントの選び方）と期限が `internal/llm/provider` と同じであることを、ネイティブのビルドで確かめる。
-   [ ] **ステップ 3-5**: `devbackend.New` を作る（設計書 §3.4）。`NewClients` は渡された設定を使わずにシナリオの fake を返し、`CheckKey` はシナリオのキーの確かめの結果を返す。テストは、シナリオごとの結果と、`devbackend` の依存に `internal/llm/claude` と `net` が入らないことを確かめる。
-   [ ] **ステップ 3-6**: `Makefile` の `WASM_PACKAGES` に `./internal/extjob/...` を加える。package_reference.md に 3 つのパッケージの行を加え、`internal/strictjson` の利用者に `internal/extjob` を加える（`TestPackageReferenceStrictjsonUsers`）。project_overview.md のディレクトリの構成に 3 つのパッケージを加える（設計書 §3.13）。

**完了の条件:** `make test`・`make lint`・`make deadcode` が通る。サーバのテストも、写した `internal/extjob` のテストも通る。

### PR-3 作成ポイント: extension job core and the two LLM backends

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6

**推奨タイトル**: `feat(0091): 拡張のジョブの処理 internal/extjob と、Claude と fake の LLM の実装を加える`

**レビュー観点**: 写したテストの行がサーバのテストと同じ期待を持つか（PR の説明の対応表） / 保存した設定の検査が補正せず、`settings_invalid` で LLM の実装を呼ばないか / API キーが結果と進み具合と保持する値に残らないか / `claudebackend` の組み立てが `provider` と同じか

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 3-3 は API キーを受け取る JSON の境界で、`secret.Secret` にする時点と閉じたメンバの集合の検査に、セキュリティの規則が集まる。Sonnet が実装すると、検査の前の文字列を保持したままにしたり、失敗の分類の順を誤ったりしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 3 つの `main`、`make wasm-check`、依存の検査（設計書 §8 の 1 の残り）

**変更するファイル:** `cmd/yt2column-wasm/`・`cmd/yt2column-wasm-dev/`・`cmd/yt2column-extjob/`（新規）とそのテスト、`internal/godepstest/`、`devdeps_test.go` の使い手、`internal/config/envaccess_test.go`、`Makefile`、`CLAUDE.md`、`docs/dev/guides/development.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/project_overview.md`、ADR（§3.3）

-   [ ] **ステップ 4-1**: `cmd/yt2column-wasm` を作る（設計書 §3.3、0090 設計書 §3.1.2）。`extjob.NewModule(claudebackend.New())` の 4 つの関数を `syscall/js` で 1 つの名前空間のオブジェクトに登録するだけを持ち、`js` のビルドだけでビルドする。ネイティブのビルドの `go build ./...`・`go vet ./...`・`make deadcode` が、このパッケージで失敗しないことを確かめる。
-   [ ] **ステップ 4-2**: `cmd/yt2column-wasm-dev` を作る（設計書 §3.4）。シナリオの 5 つの値を `main` のパッケージの文字列の変数で持ち、起動のときに `ParseScenario` で解析して、合えば `extjob.NewModule(devbackend.New(scenario))` の関数を登録し、合わなければ登録しない。`syscall/js` を使う登録のファイルだけを `js` のビルドにし、ネイティブのビルドでは、同じ変数を解析して結果だけを返す `main` を持つ（PR-5 のビルドのスクリプトが使う）。ネイティブのテストで、解析の受理と拒否を確かめる。
-   [ ] **ステップ 4-3**: `cmd/yt2column-extjob` を作る（設計書 §3.4、0090 設計書 §7.2）。`-llm=claude` か `-llm=fake` を必ず指定させ、指定がないかほかの値なら何もせずに終わる。`-llm=fake` は 5 つの引数を `ParseScenario` で解析する。`-llm=claude` は `config.ReadExtensionSettings` で読んだ 2 つの値を `StoredSettings` にする。入力のファイル（4 MiB まで）を `Module.PrepareJSON` に、続けて `Module.GenerateJSON` に渡し、`GenerateResult` の JSON を出す。テストは fake で、`-llm` の指定がないときと合わないシナリオで何も呼ばないこと、合わない入力のファイルで拒否の理由を出すこと、結果の JSON を出すこと、API キーを出力に出さないことを確かめる。
-   [ ] **ステップ 4-4**: `make wasm-check` の対象を、`cmd/yt2column-wasm` と `cmd/yt2column-wasm-dev` のビルド（それぞれの依存を含む）にし、`vet` の対象を `internal/llm/llmhttp`・`internal/llm/claude`・2 つの `main`・`internal/writer`・`internal/transcript` にする（設計書 §3.10）。
-   [ ] **ステップ 4-5**: `internal/godepstest` が `GOOS=js GOARCH=wasm` の依存を求められるようにし、依存の検査を加える（設計書 §7.3）。`cmd/yt2column/devdeps_test.go` の許さない依存に `internal/extjob/devbackend`・`cmd/yt2column-wasm-dev`・`cmd/yt2column-extjob` を加える。公開用の記事生成モジュールの依存に `internal/llm/devfake` と `internal/extjob/devbackend` が、開発用の記事生成モジュールの依存に `internal/llm/claude` と `internal/extjob/claudebackend` が入らないことを確かめるテストを、ネイティブのテストのビルドで走る場所に置く。サーバとホストの `devdeps_test.go` は、撤去の PR まで今のまま残す。
-   [ ] **ステップ 4-6**: `envaccess_test.go` の入口の一覧に `cmd/yt2column-extjob/main.go` を加える。記事生成モジュールは環境を参照しない（0090 設計書 §3.11 のテストの表）。
-   [ ] **ステップ 4-7**: 文書を書き換える。package_reference.md に 3 つの `main` の行を、project_overview.md のディレクトリの構成に 3 つの `main` を加える。CLAUDE.md の `make wasm-check` の説明を新しい対象に改める。development.md に `cmd/yt2column-extjob` の使い方（`-llm` の指定が要ること、入力のファイルをリポジトリに入れないこと）を書く（設計書 §3.13）。ADR の食い違いの記述を §3.3 のとおりに除くか書き換える。

**完了の条件:** `make test`・`make lint`（新しい `make wasm-check` を含む）・`make deadcode` が通る。

### PR-4 作成ポイント: article generation module mains and dependency checks

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7

**推奨タイトル**: `feat(0091): 記事生成モジュールと開発用のコマンドの main、WASM の検査、依存の検査を加える`

**レビュー観点**: 公開用の記事生成モジュールに fake が、開発用のものに Claude のアダプタが入らないことを、テストが `GOOS=js GOARCH=wasm` で確かめるか / 開発用のコマンドが `-llm` の指定なしで何も呼ばないか / `js` だけの `main` がネイティブの `./...` のビルド・vet・deadcode を壊さないか

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 4-5 は ADR-0045 の「公開する成果物に fake を入れない」を守る依存の検査で、`godepstest` に新しい対象の OS を足す。Sonnet が実装すると、ネイティブの依存だけを見て WASM の依存を確かめない検査になっても、テストが通ったまま気づきにくい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 拡張のビルド（設計書 §8 の 2）

**変更するファイル:** `extension/scripts/`（ビルドのスクリプト、`check-dist.ts`、`has-extension-changes.sh`、依存のディレクトリを求めるスクリプト）、`extension/package.json`、`extension/static/buildKind.js`・`extension/src/buildKind.d.ts`（新規）、`extension/static/_locales/*/messages.json`、`extension/test/`（`checkDist.test.ts`・`ciChanges.test.ts`・`repository.test.ts` など）、`Makefile`、`.github/workflows/ci.yml`、`.gitignore`、`CLAUDE.md`、`docs/dev/guides/development.md`、ADR（§3.3）

-   [ ] **ステップ 5-1**: 拡張のビルドが Go を呼ぶようにする（設計書 §3.5、0090 設計書 §3.1.4）。ビルドのスクリプトは、`go.mod` の `go` の行の版を読んで `GOTOOLCHAIN` に与え（パッチの版がなければ理由を示して止める）、`GOOS=js GOARCH=wasm go build -trimpath` で記事生成モジュールを、同じツールチェーンの `wasm_exec.js` を成果物の直下に置く。`npm run build` は、Go を呼ぶ手順まで含めて、成果物の検査の済んだ成果物を作る。
-   [ ] **ステップ 5-2**: `extension/static/buildKind.js`（値は `release`）と `extension/src/buildKind.d.ts` を加える（設計書 §3.5）。`_locales` に、開発用の印を付けた名前のメッセージを加える。
-   [ ] **ステップ 5-3**: 開発用のビルドを加える（設計書 §3.5・§6.4）。`make ext-build-dev` は 5 つの変数（既定の値は `Makefile` が持つ）をいつも渡し、ビルドのスクリプトは、`cmd/yt2column-wasm-dev` のネイティブのビルドで値を解析してから、`extension/dist-dev/` を空にし、`tsc` と `static/` を写し、開発用の manifest（`key` を除き、名前を開発用のメッセージに替える）を書き、`buildKind.js` を `dev` に置き換え、1 つの `-ldflags` の文字列に 5 つの `-X` を並べて開発用の記事生成モジュールをビルドする。`.gitignore` に `extension/dist-dev/` を加え、`repository.test.ts` の無視の規則の確かめに加える。
-   [ ] **ステップ 5-4**: `check-dist.ts` を、ビルドの種類を引数で受け取り、種類ごとの期待で検査するように改める（設計書 §3.5 の「成果物の検査」の各項目）。記事生成モジュールは Node で `wasm_exec.js` とともに読み込んで `buildInfo` を呼び、`mainPath`・`goVersion`・`llm`（`dev` ではシナリオの 5 つの値）を確かめる。`checkDist.test.ts` は、種類と出力のディレクトリの組み合わせ、`buildKind.js` の値、開発用の manifest、WASM と `wasm_exec.js` の例外、`buildInfo` の各値の違いでの拒否を確かめる（`buildInfo` を読む部分は差し込めるようにし、単体テストは Go を呼ばない）。
-   [ ] **ステップ 5-5**: CI を改める（設計書 §3.10）。
    -   `extension` の手順で、`ext-build` より前に Go を用意する。`repository.test.ts` の手順の順の確かめを直す。
    -   記事生成モジュールの依存のディレクトリを `GOOS=js GOARCH=wasm go list -deps -f` で求める処理を、スクリプトに置く。標準ライブラリを除き、`-e` を付けず、失敗したときは拡張の検査を走らせる側に倒す。
    -   `has-extension-changes.sh` は、変わったファイルの一覧に加えて依存のディレクトリの一覧を受け取り、末尾の `/` で比べる。`go.sum` を加える。`.md` の除外より前の、変わったファイルのすべての一覧で判定する。
    -   `ciChanges.test.ts` と、依存のディレクトリを求めるスクリプトのテストで、`.go`・`.tmpl`・fake の記事の `.md` の変更と削除、サブディレクトリ、`internal/llm` と `internal/llmx` の境界、求める処理の失敗を確かめる。
-   [ ] **ステップ 5-6**: 文書と ADR を書き換える。CLAUDE.md の「Extension Commands」に `make ext-build-dev` を加え、`make ext-build` が Go を呼ぶことを書く（`TestExtensionDocumentation`）。development.md に、開発用のビルドの 5 つの変数と綴り、見分け方（開発用の印、別の拡張の ID）、初めのビルドが `go.mod` の版のツールチェーンをダウンロードしうることを書き、開発用のビルドが切り替えの PR まで画面で使えないことを添える（設計書 §3.4）。ADR の食い違いの記述を §3.3 のとおりに除くか書き換える。

**完了の条件:** `make test`・`make lint`・`make ext-check`・`make ext-build-dev`（既定の値と、値を変えたもの）が通り、合わないシナリオの値で `make ext-build-dev` が失敗する。

### PR-5 作成ポイント: extension build with the WASM module and the development build

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6

**推奨タイトル**: `build(0091): 拡張のビルドに記事生成モジュールを入れ、開発用のビルドと CI の変更の検出を加える`

**レビュー観点**: 成果物の検査が WASM の出どころとツールチェーンの版を `buildInfo` で確かめるか / 開発用のビルドが合わないシナリオで止まり、`key` を持たないか / CI の変更の検出が依存のディレクトリの境界と失敗を正しく扱うか / 手順の順の変更で拡張の検査が Go なしで走らないか

**実装モデル要件**: opus

**判定理由**: CI と外部のツール（`go list`・`GOTOOLCHAIN`・Node での WASM の読み込み）の面が大きく、`mkplan.md` step 8 のパネルの条件（CI と外部の資源の面が大きい計画）に当たる。Sonnet が実装すると、`-e` による失敗の見落としや、`-ldflags` を 2 回渡して後の値だけが効くことを、検査が通ったまま残しやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: 手動の確認と、まだ使われない拡張の部品（その 1）（設計書 §8 の 3 と、4 の一部）

**変更するファイル:** `extension/src/wasmWorker.ts`・`extension/src/wasmWorkerClient.ts`・`extension/src/core/wasmResult.ts`・`extension/src/core/jobRecord.ts`・`extension/src/core/viewerTitle.ts`・`extension/src/core/optionsView.ts`（新規）、`wasm_exec.js` の型の宣言（新規）、Worker の型を確かめる `tsconfig` の設定、`extension/src/core/i18n/*`、`extension/eslint.config.js`、`extension/test/`（新しいテスト、`lintRules.test.ts`・`secretsAbsent.test.ts`）、本計画

-   [ ] **ステップ 6-1**（PR-5 のマージの後、`/runplan` の前に、人と対話のセッションで行う。§1.3 の決定 3）: 401 を読めるかを確かめる（設計書 §1.2・§7.2）。
    -   試作の拡張を、`host_permissions` を持たず、CSP を 0090 設計書 §3.7.2 の値にして作り、`make ext-build` が作った記事生成モジュールを Worker で読み込む。
    -   Chrome と Brave で、誤ったキーで `checkSettings`（`GET /v1/models`）と `generate`（`POST /v1/messages`）を、`anthropic-workspace-id` のありとなしの両方で呼び、`key_rejected` と `api_key_rejected` になる（通信の失敗にならない）ことを確かめる。実 API の呼び出しなので、人の承認を得る。料金はかからない。
    -   読めないときは、先へ進まない。人と決め直し、設計書と本計画を決定の変更として改め、ADR-0010 の保存のときの確かめを改める（設計書 §3.14）。
-   [ ] **ステップ 6-2**（ステップ 6-1 と同じセッション）: 記録がないビューアが固定のタイトルを出したときに、履歴の項目の動画のタイトルが書き換わるかを確かめる（設計書 §3.8.2、H-05）。試作の拡張に、`chrome.storage.session` に値があれば動画のタイトルを、なければ固定のタイトルを出すページを置く。Chrome と Brave で、ページを開き、ブラウザを再起動してセッションを復元し、履歴の項目のタイトルを読む。上書きされなければ案 A、されれば案 B で実装すると決める。
-   [ ] **ステップ 6-3**（ステップ 6-1 と同じセッション）: 2 つの確認の結果（ブラウザの版、手順、見たもの、決めた案）を §7 の「PR-6 の前の手動の確認の記録」に書き、PR-6 のブランチに commit する。
-   [ ] **ステップ 6-4**: `wasmWorker.ts`・`wasmWorkerClient.ts`・`wasm_exec.js` の型の宣言を作る（設計書 §3.8.4、0090 設計書 §3.3.1・§3.4）。Worker の型を確かめる設定を加える（今の `lib` は WebWorker を含まない。§1.4）。lint の `fetch` の規則を、`wasmWorker.ts` のほかで拒否する形に改める（設計書 §5.4）。テストは Worker の fake で、30 秒の読み込みの期限、`callId` での応答の対応、`exit` を `runtime_exit` に、`error` のイベントを `worker_error` に、Promise の拒否を `panic` にすること、呼び出しごとに最初の失敗だけを使い、Worker を止めることを確かめる。
-   [ ] **ステップ 6-5**: `wasmResult.ts` を作る（設計書 §3.12、H-07）。Worker から受け取る JSON の文字列を `strictJson.ts` で解析し、重複したメンバ・知らないメンバ・型の違い・512 KiB を超えるものを `result_shape` にする。
-   [ ] **ステップ 6-6**: `jobRecord.ts` を作る（0090 設計書 §3.2.1・§3.3、H-07）。ジョブの記録と開始の文脈の型と、読み戻すときの検査（`version`、閉じたメンバの集合、状態ごとのメンバ、512 KiB）を持ち、合わない記録をないものとして扱う。
-   [ ] **ステップ 6-7**: `viewerTitle.ts` を、ステップ 6-2 で決めた案で作る（設計書 §3.8.2・§6.3）。状態、表示の値、ビルドの種類、表示の言語（案 B では URL のタイトル）から文字列を返すだけの関数で、テストは設計書 §3.8.2 の表の各行と、どの入力でも空の文字列を返さないことを確かめる。案 B の `t` の解析は、ビューアの URL を改める PR-8 で行う。
-   [ ] **ステップ 6-8**: `optionsView.ts` を作る（設計書 §3.6 の表、§4.1 の `OptionsOutcome`）。結果ごとに、示す固定の文言と、欄を空にするか、設定されているかの表示を読み直すかを返す。この PR の関数が使う文言だけを英語と日本語の目録に加える。`secretsAbsent.test.ts` の、en.ts と ja.ts の文字列だけに許す例外に、API キーを指す語を加える（ほかの場所の拒否は PR-7 で置き換えるまで残す）。

**完了の条件:** `make ext-check` が通る。§7 に PR-6 の前の手動の確認の記録があり、決めた案でステップ 6-7 を実装した。

### PR-6 作成ポイント: manual checks and the first unused extension parts

**対象ステップ**: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8

**推奨タイトル**: `feat(0091): Worker の呼び出し、結果と記録の検査、ページのタイトルとオプションページの表示の関数を加える`

**レビュー観点**: 手動の確認の記録が設計書 §7.2 の組み合わせ（2 つのブラウザ、ヘッダのありとなし）を尽くし、案の選択が結果と合うか / Go の実行の終わりと Worker の失敗を呼び出しごとに 1 度だけ失敗にするか / 記録と結果の検査が補正せず、合わないものを捨てるか / タイトルがどの道でも空にならないか

**実装モデル要件**: opus

**判定理由**: ステップ 6-1・6-2 は、ブラウザの CORS と履歴の振る舞いを試作で確かめる実現性の確かめ（Phase 0 型の調査）で、結果が設計の分かれ道になる。Sonnet が進めると、ヘッダの組み合わせを 1 つだけ試して確かめたものとして記録しやすい。

**自動送信**: hold

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 7: まだ使われない拡張の部品（その 2）（設計書 §8 の 4 の一部）

**変更するファイル:** `extension/src/browser/chromeDeps.ts`、`extension/eslint.config.js`、`extension/src/jobCoordinator.ts`・`extension/src/viewerRunner.ts`・`extension/src/optionsFlow.ts`・`extension/src/core/jobMessages.ts`（新規）、`extension/test/`（新しいテスト、`lintRules.test.ts`、`apiKeyPath.test.ts`（新規）、`secretsAbsent.test.ts`（削除）、`helpers/chrome.ts`）

-   [ ] **ステップ 7-1**: `chromeDeps.ts` に、設計書 §5.4 の表の決まった関数を加える。`StorageArea` も `chrome.storage.local` のオブジェクトも外へ渡さない。`extension/test/helpers/chrome.ts` に、その fake と失敗の注入を加える。
-   [ ] **ステップ 7-2**: lint の規則を設計書 §5.4 のとおりに改める（`storage` は `chromeDeps.ts` のほかで拒否、`session` は `chrome.session` だけに狭める、決まった関数を import してよいファイルを限る、型だけの import は許す）。`sendNativeMessage` の規則は、PR-8 まで残す。`lintRules.test.ts` に、各規則の拒否と許可の行を加える。
-   [ ] **ステップ 7-3**: `secretsAbsent.test.ts` を、API キーの経路を確かめる `apiKeyPath.test.ts` に置き換える（設計書 §5.4 の「ソースの検査」、0090 設計書 §7.3）。`onChanged` を参照するのが `chromeDeps.ts` の `chrome.storage.session.onChanged` だけであること、`chrome.storage.local.get` を呼ぶのが `readAnthropicSettings` だけで引数が固定の 2 つの名前であること、`chrome.storage.sync` を参照しないこと、ログの関数に API キーを渡さないことを確かめる。`secretsAbsent.test.ts` の削除は削除だけの commit にし、`make ext-coverage-compare` で比べる。
-   [ ] **ステップ 7-4**: `jobMessages.ts` にページの間のメッセージと実行の Port の型と検査を、`jobCoordinator.ts` に新しい SW の調停を作る（0090 設計書 §3.2・§3.3・§3.3.1・§5.2・§6.1〜§6.3、設計書 §3.9）。`chrome.*` に依存しない部分は、決まった関数を引数で受け取る関数にする。テストは fake で、0090 設計書 §7.1 の「SW の調停」の項目、送り手の確かめ（要件書 AC-24）、同じ入力と同じ動画の照会（AC-10）、同時に 2 つまでの上限（AC-13）、上限と捨てる順（AC-09）、SW が API キーの値を読まず `localKeys` だけを使うこと（AC-20）、結果不明と、入力を受け取る前に SW が止まった場合と、完成したが保持できなかった場合（H-06）を確かめる。
-   [ ] **ステップ 7-5**: `viewerRunner.ts` に実行するビューアの流れを作る（0090 設計書 §3.4・§6.2・§6.3、設計書 §2.4・§3.8.1）。予約の後に `readAnthropicSettings` で 2 つの項目を読み、項目がなければ `generate` を呼ばずに `api_key_missing`、読む呼び出しの失敗は `internal`（`settings_read`）、あれば検査せずに `StoredSettings` にして渡す。進み具合と結果を SW に送り、記録が結果を持つまで送り直す。テストは Worker と決まった関数の fake で、50 分の期限で止めて `run_timeout` にすること、実行の間だけ読み込み直しの確認を出すこと、`runner` が替わったら Worker を止めること、空の文字列と `null` の項目では `api_key_missing` にならないこと（H-06）を確かめる。
-   [ ] **ステップ 7-6**: `optionsFlow.ts` にオプションページの保存と削除の流れを作る（設計書 §3.6・§6.1）。押した時点の値の写しを検査して書くこと、検査と書き込みの間の欄と操作の無効化、90 秒の全体の期限（読み込み 30 秒を含み、書き込みに及ばない）、`set` と `remove` をそれぞれ 1 回だけ呼ぶこと、保存しないときに書き込みを呼ばないこと、結果によらず `localKeys` で表示を読み直すことを、Worker と決まった関数の fake で確かめる。

**完了の条件:** `make ext-check` が通る。`secretsAbsent.test.ts` の削除の commit に `make ext-coverage-compare` の通った出力がある。

### PR-7 作成ポイント: storage entry points, key path checks, coordinator, runner, and options flow

**対象ステップ**: 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6

**推奨タイトル**: `feat(0091): chrome.storage の決まった関数と API キーの経路の検査、新しい SW の調停と実行するビューアの流れを加える`

**レビュー観点**: lint とソースの検査が、`get(null)` のような全部を読む呼び出しと、決まった関数の外の `storage` の参照を拒否するか / 調停が直列に処理し、`runner` と一致しない知らせで記録を変えないか / 実行するビューアが保存した設定を検査せずに渡し、値を持ち続けないか / 保存と削除がそれぞれ 1 回の書き込みか

**実装モデル要件**: opus

**判定理由**: ステップ 7-2・7-3 は API キーの経路を守るセキュリティの検査で、7-4 は直列の処理と止まった実行の判定を持つ状態機械である（パネルの条件のセキュリティの関門に当たる）。Sonnet が実装すると、規則の組み直しで既存の禁止を落としたり、同時の 2 つの開始で記録を二重に加えたりしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 8: 切り替え（設計書 §8 の 4）

**変更するファイル:** `extension/static/`（`manifest.json`、`options.html`（新規）、`popup.html`、`viewer.html`、`_locales`）、`extension/src/`（入口、表示、`core/` の今の型、撤去するファイル）、`extension/scripts/check-dist.ts`・`has-extension-changes.sh`、`extension/test/`、サーバとホストの文書のガード（§1.3 の決定 2）、`internal/doctestutil/`、README と `README.en.md`、`docs/dev/`（security.md・development.md・project_overview.md・production_profile.md・cli.md・package_reference.md）、`docs/translation_glossary.md`、`CLAUDE.md`、ADR（§3.3）、本計画

-   [ ] **ステップ 8-1**: manifest を 0090 設計書 §3.7 の値にする（権限 `activeTab`・`scripting`・`storage`、CSP、`incognito`、`minimum_chrome_version` `"142"`、オプションページをタブで開く、今の `key`）。`manifest.test.ts` を直し、`check-dist.ts` がオプションページの参照も確かめるようにする。`_locales` の `extDescription` を拡張だけの経路の説明に改める（設計書 §3.5）。
-   [ ] **ステップ 8-2**: 入口を新しい部品に配線する。`background.ts` は `jobCoordinator.ts` を登録する。`popup.ts`・`launch.ts` は、`setLocalAccessLevel` が成功したときだけ関数を注入し、生成の開始を SW に送る（0090 設計書 §3.5・§3.6、要件書 AC-23）。`viewer.ts` は、実行するビューア（`viewerRunner.ts`）と表示するビューア（`chrome.storage.session.onChanged`）を持つ。`options.html`・`options.ts` を作る（文字を伏せた欄、`buildInfo` の表示。設計書 §3.6）。`popup.html` に、詳細の節の後のリンクと、設定が要る状態の操作を置く（設計書 §3.7）。`viewer.html` の静的なタイトルを空でない値にし、`viewerTitle.ts` を使う（設計書 §3.8.2）。開発用の印を示す（設計書 §3.5）。
-   [ ] **ステップ 8-3**: 0090 設計書 §3.11 の表の「変える」拡張のファイル（`viewerUrl.ts`（案 B では `t` を加える）、`viewerQuery.ts`、`prepare.ts`、`jobView.ts`・`popupView.ts`・`sendMessages.ts`・`types.ts`・`i18n/*`、`renderJob.ts`・`renderViewer.ts`）を、§3.8 の状態と設計書 §4.2 の表に合わせる。文言は英語と日本語の両方に置く（要件書 AC-14・AC-15）。0090 設計書 §3.11 のテストの表の「直す」テスト（`popupHtml.test.ts`・`popupView.test.ts`・`viewerApp.test.ts`・`viewerUrl.test.ts`・`extensionMessages.test.ts`・`viewerQuery.test.ts`・`entryPoints.test.ts`・`jobCodes.test.ts`・`checkDist.test.ts`・`lintRules.test.ts`）を直し、オプションページの HTML のテストを加える。
-   [ ] **ステップ 8-4**: 拡張のホストとの送信、応答の検査、ポーリングを撤去する（0090 設計書 §3.11 の「撤去」の行、`startCoordinator.ts`、`articleKey.ts`、`extensionMessages.ts` の今の型、`test/helpers/native.ts` と、それらのテスト）。lint の `sendNativeMessage` の規則を除く。テストの削除は削除だけの commit にし、`make ext-coverage-compare` で比べる。
-   [ ] **ステップ 8-5**: サーバとホストの文書のガードを除く（§1.3 の決定 2）。`TestServerDocsContract`・`TestReadmeHostExitCodes`・`TestDevelopmentDocsDevServer`・`TestNativeHostContract`・`serverReasons.test.ts`と、それだけが使う補助（`internal/doctestutil` のサーバとホストの位置の値、`internal/server/test_helpers.go` の `RejectReasonsForTest`・`APICodesForTest` など。`make deadcode` が確かめる）を除く。`has-extension-changes.sh` と `ciChanges.test.ts` から `internal/server/response.go` を除く（§1.3 の決定 6）。テストの削除は削除だけの commit にし、`make coverage-compare` と `make ext-coverage-compare` で比べる。
-   [ ] **ステップ 8-6**: 仕様の文書を書き換える（要件書 AC-33・AC-34、設計書 §3.13、0090 設計書 §3.14.2）。
    -   README を先に書き換え、`/mktrans` で `README.en.md` に移す。要件書 AC-34 の内容を書き、サーバとホストの節を除く。`## ブラウザ拡張` の語（`metadataDocs.test.ts`）、「Chrome 142 以降」（`manifest.test.ts`）、拡張の ID（`TestExtensionDocumentation`）、リンク（`TestReadmeLinks`）を合わせる。
    -   security.md・development.md・project_overview.md・production_profile.md・cli.md・用語集・CLAUDE.md を、0090 設計書 §3.14.2 と設計書 §3.13 の要点のとおりに書き換える。security.md の `### 8.1. ブラウザ拡張（#110）` の見出しと `**要求する権限**` の一覧は、`TestExtensionDocumentation` が読む。
    -   README の `### 推奨構成` と production_profile.md の `## 本番の構成` の塊が、サーバの設定ファイルの形でなくなるので、`TestProductionProfileDocsContract` の照合を CLI の読み込みに改める。
-   [ ] **ステップ 8-7**: ADR の食い違いの記述を §3.3 のとおりに除くか書き換える。案 B で実装したときは、人と決めたうえで、ADR-0010 の「ビューアの URL」の決定を改め、要件書 §3.2 の「ビューアの URL」の行の扱いを PR の説明に書く（設計書 §3.8.2・§3.14）。
-   [ ] **ステップ 8-8**: 手動の確認を行い、§7 の「PR-8 の手動の確認の記録」に書く（要件書 §5、設計書 §7.2）。
    -   開発用のビルドで、要件書 AC-29 の状態と、AC-07・AC-08・AC-09・AC-11・AC-16・AC-20・AC-23 の手動の確認の項目を確かめる。
    -   オプションページで、VoiceOver の読み上げ（H-12。読み上げるときは欄の説明に注意を添える）と、同期を有効にしたプロファイルでパスワードの保存を勧められないこと（AC-03）を確かめる。
    -   ポップアップ、ビューア、オプションページで、CLAUDE.md「Accessibility」の各項目を、キーボードと明るい・暗いテーマで確かめる（AC-18）。
    -   本番の構成で、Chrome と Brave で 1 本ずつ、背面のまま記事を完成させ、キーの確かめに料金がかからないことを Console の使用量で確かめる（AC-01）。実 API の呼び出しなので、人の承認を得る。

**完了の条件:** `make test`・`make lint`・`make deadcode`・`make ext-check` が通る。削除の commit に、カバレッジの比較の通った出力がある。§7 に PR-8 の手動の確認の記録がある。サーバとホストを主経路として述べる文書が残らない。

### PR-8 作成ポイント: switch the extension to the extension-only path

**対象ステップ**: 8-1 / 8-2 / 8-3 / 8-4 / 8-5 / 8-6 / 8-7 / 8-8

**推奨タイトル**: `feat(0091): 拡張を拡張だけの経路に切り替え、ホストとの送信と、サーバを述べる文書を除く`

**レビュー観点**: manifest と CSP が 0090 設計書 §3.7 の値で、ホストとの送信が残らないか / 各状態の表示と文言が設計書 §4.2 の表と要件書 AC-14・AC-15 に合うか / README の移行の手順が要件書 AC-34 の 3 つを含むか / 手動の確認の記録が要件書 §5 の項目を尽くすか

**実装モデル要件**: opus

**判定理由**: 拡張の権限と保存と送信の経路を一度に変える移行の段で、パネルの条件（セキュリティの関門と移行、多くのテストの更新）に当たる。Sonnet が実装すると、`setLocalAccessLevel` の失敗のときにも関数を注入したり、文書の書き換えで固定された見出しを崩したりしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 9: 撤去と文書（設計書 §8 の 5）

**変更するファイル:** 撤去するパッケージ（要件書 AC-31）、`internal/config/`（`!js` のファイル、そのテストと補助、`LoadServer` を使う CLI のテスト）、`internal/llm/provider/provider_test.go`、`cmd/yt2column/`（`_test.go` だけ）、`internal/llm/devfake/deps_test.go`、`internal/godepstest/godepstest_test.go`、`internal/pipeline/relay_guard_test.go`、`internal/config/envaccess_test.go`、`Makefile`、`.golangci.yml`、`.github/workflows/ci.yml`、文書のガードのコマンドの写し、`CLAUDE.md`、`docs/dev/`、ADR（§3.3）、`docs/tasks/0091_extension_only_migration/_checks/removal.sh`（新規）

-   [ ] **ステップ 9-1**: サーバとホストの設定の読み込みを使うテストを、CLI の読み込みに改める（§1.4 の `config_test.go` の 2 つのテスト、`provider_test.go::TestNewSendsStageEffort`、`cmd/yt2column/docs_test.go` に残る呼び出し）。
-   [ ] **ステップ 9-2**: 要件書 AC-31 のパッケージと、`internal/config` の `!js` のファイルとそのテストと補助を除く。直すテストは、`devfake/deps_test.go` の一覧（`internal/privatefile` は残す）、`godepstest_test.go` の例、`relay_guard_test.go` のホストの 2 つのテスト、`envaccess_test.go` の入口の一覧、`devdeps_test.go` の許さない依存の一覧である（設計書 §3.11）。どのパッケージからも import されなくなるテストの補助（`internal/udstest` など）を、`go list -deps -test` で確かめてから除く。テストの削除は削除だけの commit にし、`make coverage-compare` で比べる。
-   [ ] **ステップ 9-3**: `make build` を `yt2column` だけにし、`build-dev` とその変数を除く。`makefile_test.go` を直す。`.golangci.yml` の撤去したパッケージを指す規則を、`ci.yml` の `doc-guard` の対象から `./cmd/yt2column-server` を除く（設計書 §3.10・§3.11）。
-   [ ] **ステップ 9-4**: 文書を書き換える（設計書 §3.13）。文書のガードのコマンドの 4 つの写し（CLAUDE.md「Development Notes」、`.claude/commands/_lib/task-pr-handoff.md`、`.claude/commands/_lib/simple-process.md`、`docs/dev/guides/fixprguard_guide.md`）から `./cmd/yt2column-server` を除く。CLAUDE.md の「Build Commands」と「Architecture Overview」、package_reference.md（撤去したパッケージの行、`internal/strictjson` の利用者、`internal/privatefile` の規則のリンク先）、project_overview.md のディレクトリの構成、development.md の `make` の目標、security.md に残る撤去したコードの記述を直す。
-   [ ] **ステップ 9-5**: `_checks/removal.sh` を作る。撤去したパッケージのディレクトリがないこと、`internal/job`・`internal/privatefile`・`internal/cachelock`・`internal/signalrun`・`internal/redact` が残ること、`internal/config` の Go のファイルに `!js` のビルドタグがないこと、`GOOS=js GOARCH=wasm go list -deps ./internal/config` が `internal/privatefile` を含まないこと、`Makefile`・`.golangci.yml`・`ci.yml`・`has-extension-changes.sh` が撤去したパッケージを指さないこと、ステップ 9-4 の文書の `go test`・`go build`・`make` のコマンドの例が挙げるパッケージが `go list` で見つかることを確かめる。各チェックが、少なくとも 1 つの壊し方で失敗することを確かめる。出力とコミット ID を §7 に書く。
-   [ ] **ステップ 9-6**: ADR の食い違いの記述を §3.3 のとおりに除き、§1.3 の決定 9 の 2 か所を直す。PR を作る前に `git grep -nE 'issues/773([^0-9]|$)' origin/main -- docs/dev/adr/` で、残る食い違いの記述がないことを確かめる（`docs/dev/conventions/adr.md`「Discrepancy statements」）。

**完了の条件:** `make test`・`make lint`・`make deadcode`・`make ext-check` が通る。`_checks/removal.sh` が通る。削除の commit に `make coverage-compare` の通った出力がある。

### PR-9 作成ポイント: remove the server, the host, and the development server

**対象ステップ**: 9-1 / 9-2 / 9-3 / 9-4 / 9-5 / 9-6

**推奨タイトル**: `refactor(0091): localhost サーバ、Native Messaging のホスト、開発用のサーバを撤去する`

**レビュー観点**: 残すパッケージ（`internal/privatefile` など）を誤って除いていないか / 撤去したパッケージを指す設定と文書のコマンドが残らないか / 書き換えた CLI のテストが元のテストと同じことを確かめるか / ADR に #773 の食い違いの記述が残らないか

**実装モデル要件**: opus

**判定理由**: 多くのパッケージ、テスト、設定、文書が結びついた撤去で、パネルの条件（移行と多くのテストの更新）に当たる。Sonnet が実装すると、`LoadServer` を使う CLI のテストを確かめる中身ごと消したり、写しの 1 つにコマンドを残したりしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 状態の変化 |
|---|---|---|
| M1（PR-1〜PR-4） | LLM の層、設定の読み込み、`internal/extjob`、3 つの `main`、依存の検査 | 記事生成モジュールが WASM でビルドされる（まだ使われない） |
| M2（PR-5） | 拡張のビルドと開発用のビルド、CI | 成果物に記事生成モジュールが入る（まだ使われない） |
| M3（PR-6・PR-7） | 手動の確認の記録、まだ使われない拡張の部品 | 401 とタイトルの前提が確かめられ、部品が単体テストで固まる |
| M4（PR-8） | 切り替え、仕様の文書 | 拡張だけの経路で記事ができる。サーバとホストは使われない |
| M5（PR-9） | 撤去 | サーバ、ホスト、開発用のサーバがなくなる |

順序は設計書 §8 のとおりである。§8 の 4（切り替え）を、まだ使われない部品（PR-6・PR-7）と切り替え（PR-8）に分けた理由は §1.3 の決定 1 に、§8 の 3（手動の確認）を PR-6 の前に置いた理由は決定 3 にある。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 | `ErrAuthRejected`、`Status`、`CheckKey`、`js` のビルドの指定 | sonnet+opus-review | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 | `internal/config` の拡張用の読み込みと `!js`、fake のシナリオ | sonnet | auto |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 | `internal/extjob`、`claudebackend`、`devbackend` | sonnet+opus-review | auto |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 | 3 つの `main`、`make wasm-check`、依存の検査 | sonnet+opus-review | auto |
| PR-5 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 | 拡張のビルド、開発用のビルド、成果物の検査、CI | opus | auto |
| PR-6 | 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 | 手動の確認、Worker、結果と記録の検査、タイトルとオプションページの表示 | opus | hold |
| PR-7 | 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6 | `chrome.storage` の決まった関数と lint、API キーの経路の検査、SW の調停、実行するビューア、オプションページの流れ | opus | auto |
| PR-8 | 8-1 / 8-2 / 8-3 / 8-4 / 8-5 / 8-6 / 8-7 / 8-8 | 切り替え、ホストとの送信とサーバの文書のガードの撤去、仕様の文書、手動の確認 | opus | auto |
| PR-9 | 9-1 / 9-2 / 9-3 / 9-4 / 9-5 / 9-6 | サーバ、ホスト、開発用のサーバの撤去、残りの文書 | opus | auto |

PR-6 だけを `hold` にするのは、ステップ 6-1〜6-3 を、PR-5 のマージの後、PR-6 の `/runplan` の前に、人と対話のセッションで行うためである（§1.3 の決定 3）。PR-8 の手動の確認（ステップ 8-8）は `/runplan` の中で人に頼むので、`auto` のままにする。

### 3.3. ADR の食い違いの記述を除く PR

issue #773 にリンクする食い違いの記述（コミット `bac09b3f` で `git grep -n 'issues/773' docs/dev/adr/` が挙げた、有効な ADR の「決定」の記述）を、どの PR で除くかを示す。「書き換える PR」は、記述の一部を解いて、残る食い違いを述べる形に書き換える PR である。各 PR は、表にない ADR でも、変えたコードと文書で記述が事実と合わなくなったときは、同じ規則で書き換える（`docs/dev/conventions/adr.md`「Discrepancy statements」）。

| ADR | 書き換える PR | 除く PR |
|---|---|---|
| ADR-0003・0004・0006・0007・0008・0009・0010・0017・0022・0027・0028・0029・0030・0031・0033・0034・0043 | ― | PR-8 |
| ADR-0019・0020・0038 | ― | PR-4 |
| ADR-0026 | ― | PR-5 |
| ADR-0049 | PR-4・PR-5 | PR-8 |
| ADR-0024 | PR-4（記事生成モジュールと開発用のコマンドを加えた） | PR-9 |
| ADR-0045（`:27`） | PR-4・PR-5 | PR-9 |
| ADR-0048（`:28`） | PR-8（コードの撤去だけが残る） | PR-9 |

## 4. テスト戦略 (Test Strategy)

-   **Go の単体テスト:** 設計書 §7.1 と 0090 設計書 §7.1 の各項目を、§2 の各ステップのテストが受け持つ。LLM は fake の `LLMClient`（`internal/llm/testutil`）か `devfake` で置き換え、HTTP は `net/http/httptest` で確かめる。実 API を呼ばない。
-   **今のサーバと同じ値:** PR-3 で写したテスト（要件書 AC-19）が、撤去の PR の後も `internal/extjob` で同じ値を確かめる。PR-3 から PR-9 までは、サーバのテストと両方が走る（§1.3 の決定 7）。
-   **`js` のビルドだけの部分:** `syscall/js` の登録と、送信の指定とヘッダは、`make wasm-check` のビルドと `vet`、PR-5 の成果物の検査（Node で `buildInfo` を呼ぶ）、PR-6 と PR-8 の手動の確認で確かめる。Go の手順は Node.js を呼ばない（ADR-0026）。
-   **TypeScript の単体テスト:** `chromeDeps.ts` の決まった関数と Worker の fake に失敗を注入し、開発用のビルドで出せない状態（要件書 AC-29、H-06）を確かめる。fake は `extension/test/helpers/chrome.ts` に置き、`fakes.test.ts` で fake 自体を確かめる。
-   **セキュリティのテスト:** 依存の検査（PR-4）、成果物の検査（PR-5）、API キーの経路の lint とソースの検査（PR-7）、manifest（PR-8）が、設計書 §7.3 と 0090 設計書 §7.3 を受け持つ。
-   **テストの補助:** Go の新しいパッケージの中だけで使う補助は、そのパッケージの `test_helpers.go`（`//go:build test`）に置く（test_organization.md の Classification B）。ほかのパッケージから使う fake は作らない。`testutil/` のファイルを加えたときは、`TestFakesCarryBuildTag` の数を直す。
-   **文書のガード:** 各文書の値と構造は、今のガード（`TestPackageReferenceListsPackages`・`TestPackageReferenceStrictjsonUsers`・`TestExtensionDocumentation`・`TestReadmeLinks`・`TestReadmeSyncMarker`・`TestReadmeVersionsShareStructure`・`metadataDocs.test.ts`・`manifest.test.ts`・`jobCodes.test.ts`・`TestProductionProfileDocsContract`・`TestADRRecords`）が確かめる。文書の文は PR のレビューで読む。
-   **手動の確認:** PR-6 の前（401 とタイトル）と PR-8（開発用のビルド、アクセシビリティ、本番の構成）で行い、§7 に記録する。
-   **後方互換:** 互換の仕組みは作らない（要件書 AC-32）。古い版の `sharedToken`・`serverPort` を読まないことは、今の `TestSharedTokenRemoved` が拡張のソースで確かめ続ける。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 拡張の送信で Anthropic の 401 を読めず、キーの確かめと `api_key_rejected` が成り立たない | PR-6 の前に確かめ（ステップ 6-1）、読めなければ止めて人と決め直す |
| 履歴の項目のタイトルが上書きされ、ビューアの URL の形が変わる | PR-6 の前に確かめ（ステップ 6-2）、案 B なら PR-8 で URL と ADR-0010 を改める |
| まだ使われない部品の形が、配線のときに合わないと分かる | 部品は 0090 設計書と設計書の型のとおりに作り、単体テストで固める。合わなければ PR-8 で直し、設計の変更が要るときは人と決める |
| 写したテストの行が、サーバのテストの期待とずれる | PR-3 の説明の対応表をレビューで読む。撤去まで両方を走らせる |
| 公開する成果物に fake が入る | 依存の検査（PR-4）と成果物の検査（PR-5） |
| CI の変更の検出が、記事生成モジュールの依存の変更を見落とす | 依存を `go list -deps` で求め、失敗したら拡張の検査を走らせる（ステップ 5-5） |
| 拡張のビルドが手元の Go の版で変わる | `GOTOOLCHAIN` に `go.mod` の版を与え、成果物の検査が `buildInfo` の版を確かめる |
| 初めの拡張のビルドが、ツールチェーンのダウンロードでネットワークを使う | development.md に書く。エージェントは、そのビルドの前に人の承認を得る |
| 切り替えの PR で、固定された見出しやリンクを崩して文書のガードが失敗する | §1.4 の固定された箇所をステップ 8-6 に挙げた。README の変更は `/mktrans` と同じ commit にする |
| 撤去の PR で、残すパッケージを除くか、撤去したパッケージを指す設定が残る | `_checks/removal.sh`（ステップ 9-5） |
| 文書のガードのコマンドの写しの 1 つに `./cmd/yt2column-server` が残る | ステップ 9-4 で 4 つの写しと `ci.yml` を挙げ、`_checks/removal.sh` がコマンドの例のパッケージを確かめる |
| API キーを削除しても、始まった実行が最長 50 分 API を呼び続ける | 受け入れる（設計書 §5.1）。README に、止めるにはビューアを閉じることを書く |

## 6. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6）
-   [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5）
-   [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6）
-   [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7）
-   [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6）
-   [ ] PR-6 マージ済み（対象ステップ: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8）
-   [ ] PR-7 マージ済み（対象ステップ: 7-1 / 7-2 / 7-3 / 7-4 / 7-5 / 7-6）
-   [ ] PR-8 マージ済み（対象ステップ: 8-1 / 8-2 / 8-3 / 8-4 / 8-5 / 8-6 / 8-7 / 8-8）
-   [ ] PR-9 マージ済み（対象ステップ: 9-1 / 9-2 / 9-3 / 9-4 / 9-5 / 9-6）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種類の意味は次のとおりである。

-   `test`: 下の表のテストで振る舞いを確かめる。Go は `make test`、拡張は `make ext-test` が走らせる。テストの名前は目安で、実装で名前を変えたときは、この表を同じ commit で直す。
-   `static`: ガードのテスト、`make` の目標、`_checks/` のスクリプトで、コードと設定の値と構造を確かめる。
-   `diff`: `_checks/cli_unchanged.sh` で、merge-base と各 PR のコミットを比べる（§1.3 の決定 8）。
-   `manual`: PR のレビューで文書と文言を読むか、§7 の記録の手動の確認で確かめる。

表のパスは、`internal/extjob/` を `ej/`、`extension/test/` を `et/`、`cmd/yt2column/` を `cy/` と略す。

| AC | 確かめ | 種類 | 実装 | PR |
|---|---|---|---|---|
| AC-01 | `cy/makefile_test.go::TestMakeBuildBuildsEveryBinary`（`yt2column` だけ）、`et/manifest.test.ts`（権限）、PR-8 の手動の確認（ホストもサーバもないプロファイルで記事が完成する） | test、manual | 8-1・8-8・9-3 | PR-8、PR-9 |
| AC-02 | `ej/module_test.go::TestCheckSettings`（形の検査と 4 つの結果）、`internal/llm/claude/checkkey_test.go::TestCheckKey`、`ej/devbackend/devbackend_test.go::TestCheckKeyFollowsScenario`、`et/optionsFlow.test.ts`（期限、1 回の書き込み、保存しないときに書かない）、`et/optionsView.test.ts`、PR-8 の手動の確認 | test、manual | 1-4・3-3・3-5・6-8・7-6・8-2 | PR-1、PR-3、PR-6、PR-7、PR-8 |
| AC-03 | `et/optionsFlow.test.ts`（保存の後に欄を空にする、保存した値を読まない）、`et/optionsHtml.test.ts`（文字を伏せた欄）、PR-8 の手動の確認（パスワードの保存を勧めない） | test、manual | 7-6・8-2・8-8 | PR-7、PR-8 |
| AC-04 | `et/optionsFlow.test.ts`（空のワークスペース ID を空の文字列で書く、1 回の `remove`）、`ej/module_test.go::TestStoredSettingsCheck`（空の文字列は指定なし）、PR-8 の手動の確認（再起動の後も残る） | test、manual | 3-3・7-6・8-8 | PR-3、PR-7、PR-8 |
| AC-05 | `ej/claudebackend/claudebackend_test.go::TestClientsMatchProvider`、`internal/config/extension_test.go::TestLoadExtensionProductionProfile`、`et/optionsHtml.test.ts`（欄が 2 つだけ） | test | 2-2・3-4・8-2 | PR-2、PR-3、PR-8 |
| AC-06 | `et/popupHtml.test.ts`（いつもあるリンク）、`et/popupView.test.ts`（API キーがないときにビューアを開かない）、`et/viewerRunner.test.ts`（`api_key_missing`）、`ej/module_test.go::TestStoredSettingsCheck`（`settings_invalid` で LLM を呼ばない）、`et/viewerApp.test.ts`（設定が要る状態と料金に触れないこと） | test | 3-3・7-5・8-2・8-3 | PR-3、PR-7、PR-8 |
| AC-07 | `et/jobCoordinator.test.ts`（生成の開始はポップアップからだけ、自動で送り直さない）、`et/viewerRunner.test.ts`（記録やビューアの URL を開くだけでは始めない）、PR-8 の手動の確認 | test、manual | 7-4・7-5・8-8 | PR-7、PR-8 |
| AC-08 | `et/jobCoordinator.test.ts`（止まった実行の 4 つの経路）、`et/viewerRunner.test.ts`（実行の間だけ読み込み直しの確認）、PR-8 の手動の確認 | test、manual | 7-4・7-5・8-8 | PR-7、PR-8 |
| AC-09 | `et/jobCoordinator.test.ts`（上限と捨てる順、上限での拒否）、`et/jobRecord.test.ts`（記録に字幕と概要欄がない形）、PR-8 の手動の確認 | test、manual | 6-6・7-4・8-8 | PR-6、PR-7、PR-8 |
| AC-10 | `et/jobCoordinator.test.ts`（同じ入力と同じ動画の照会、API キーがないときの順） | test | 7-4 | PR-7 |
| AC-11 | `et/viewerApp.test.ts`（3 つの状態で不明と示し、金額を示さない。`api_key_missing`・`settings_invalid` で触れない）、PR-8 の手動の確認 | test、manual | 8-3・8-8 | PR-8 |
| AC-12 | `internal/llm/claude/errors_test.go::TestHTTPStatusErrorAuthRejected`、`ej/run_test.go::TestRunAuthRejected`、`et/viewerApp.test.ts`（キーが拒まれた状態と操作）、`et/jobCoordinator.test.ts`（同じジョブ ID でやり直す）、§7 の PR-6 の前の手動の確認 | test、manual | 1-1・3-1・6-1・7-4・8-3 | PR-1、PR-3、PR-6、PR-7、PR-8 |
| AC-13 | `et/jobCoordinator.test.ts`（3 つ目の開始の拒否）、`et/popupView.test.ts`（開始が拒否された状態） | test | 7-4・8-3 | PR-7、PR-8 |
| AC-14 | `et/viewerApp.test.ts`・`et/popupView.test.ts`（状態ごとの固定の文言、例外の文を含まない）、`et/catalog.test.ts`（英語と日本語の目録が同じ項目を持つ）、文言の読み合わせ | test、manual | 8-3 | PR-8 |
| AC-15 | 文言の読み合わせ（PR のレビュー） | manual | 8-3 | PR-8 |
| AC-16 | `et/viewerTitle.test.ts`（設計書 §3.8.2 の表の各行、空にしない）、`et/popupHtml.test.ts`（`viewer.html` の静的なタイトル）、§7 の PR-6 の前の手動の確認、PR-8 の手動の確認（再起動の後の履歴） | test、manual | 6-2・6-7・8-2・8-8 | PR-6、PR-8 |
| AC-17 | `et/checkDist.test.ts`（開発用の manifest に `key` がない、`buildKind.js` が `dev`）、`et/popupView.test.ts`・`et/viewerApp.test.ts`（開発用の印の表示）、PR-8 の手動の確認 | test、manual | 5-3・5-4・8-2 | PR-5、PR-8 |
| AC-18 | `et/popupHtml.test.ts`・`et/optionsHtml.test.ts`（`lang`、名前、状態の領域、フォーカスの順）、`et/style.test.ts`（押せる領域、動きを止める指定）、PR-8 の手動の確認（キーボード、コントラスト、VoiceOver） | test、manual | 6-8・8-2・8-8 | PR-6、PR-8 |
| AC-19 | `ej/prepare_test.go`・`ej/result_test.go`・`ej/run_test.go`（サーバの 3 つのテストの行を写したもの） | test | 3-1・3-2 | PR-3 |
| AC-20 | `et/apiKeyPath.test.ts`、`et/lintRules.test.ts`（`storage` と決まった関数の規則）、`et/jobCoordinator.test.ts`（SW が値を読まない）、`ej/module_test.go::TestResultHasNoAPIKey`、`cmd/yt2column-extjob/main_test.go`（出力に出さない）、PR-8 の手動の確認（ログ、URL、`storage.session`） | test、manual | 3-3・4-3・7-1・7-2・7-3・7-4・8-8 | PR-3、PR-4、PR-7、PR-8 |
| AC-21 | `internal/llm/llmhttp/llmhttp_test.go::TestStatusNoRedirect`・`TestPostNoRedirect`、`ej/claudebackend/claudebackend_test.go::TestClientsMatchProvider`（15 分）、`et/viewerRunner.test.ts`（50 分で `run_timeout`）、`et/lintRules.test.ts`（`fetch` は `wasmWorker.ts` だけ）、`et/manifest.test.ts`（CSP の `connect-src`） | test | 1-2・1-3・3-4・6-4・7-5・8-1 | PR-1、PR-3、PR-6、PR-7、PR-8 |
| AC-22 | `et/manifest.test.ts`（権限、キーの集合、CSP、`incognito`、`minimum_chrome_version`、`key`、オプションページ） | test | 8-1 | PR-8 |
| AC-23 | `et/launch.test.ts`（`setLocalAccessLevel` が失敗したら注入しない）、PR-8 の手動の確認（再起動の後） | test、manual | 8-2・8-8 | PR-8 |
| AC-24 | `et/jobCoordinator.test.ts`（送り手の確かめ）、`et/entryPoints.test.ts` | test | 7-4・8-3 | PR-7、PR-8 |
| AC-25 | `et/checkDist.test.ts`、`make ext-build`（成果物の検査を走らせる）、`internal/extjob` の依存の検査（ステップ 4-5）、`cy/devdeps_test.go::TestNoDevDependency` | test、static | 4-5・5-1・5-4 | PR-4、PR-5 |
| AC-26 | `_checks/cli_unchanged.sh`（各 PR）。CLI の今のテスト（`cy/` の `_test.go`）が通ること | diff、test | 1-6 と各 PR | PR-1〜PR-9 |
| AC-27 | `make wasm-check`（2 つの記事生成モジュールと依存）、`ej/` のテスト。拡張の TypeScript が規則を持たないことはレビューで読む | static、test、manual | 3-1・4-4・8-3 | PR-3、PR-4、PR-8 |
| AC-28 | `cmd/yt2column-extjob/main_test.go` | test | 4-3 | PR-4 |
| AC-29 | `make ext-build-dev`（合わないシナリオで失敗）、`et/checkDist.test.ts`（シナリオの値）、`ej/devbackend/devbackend_test.go`、`internal/llm/devfake/scenario_test.go::TestParseScenario`、依存の検査（開発用に Claude が入らない）、手で出せない状態の単体テスト（`et/jobCoordinator.test.ts`・`et/viewerRunner.test.ts`・`et/optionsFlow.test.ts`・`et/wasmWorkerClient.test.ts`）、PR-8 の手動の確認 | test、static、manual | 2-3・3-5・4-5・5-3・6-4・7-4・7-5・7-6・8-8 | PR-2〜PR-8 |
| AC-30 | `et/ciChanges.test.ts`、依存のディレクトリのスクリプトのテスト、`et/repository.test.ts`（手順の順）、`_checks/removal.sh`（設定が撤去したパッケージを指さない） | test、static | 5-5・8-5・9-3・9-5 | PR-5、PR-8、PR-9 |
| AC-31 | `_checks/removal.sh`、`TestPackageReferenceListsPackages`、`cy/makefile_test.go::TestMakeBuildDev` を除いた後の `make` の目標の確かめ、`et/entryPoints.test.ts`（ホストへの送信がない） | static、test | 8-4・9-2・9-3・9-5 | PR-8、PR-9 |
| AC-32 | `internal/pipeline/relay_guard_test.go::TestSharedTokenRemoved`、`et/manifest.test.ts`（`nativeMessaging` を持たない） | test | 8-1・8-4 | PR-8 |
| AC-33 | 文書の読み合わせ（PR のレビュー）、`_checks/removal.sh`（文書のコマンドの例）、文書のガード（§4） | manual、static | 1-5・2-5・3-6・4-7・5-6・8-6・9-4 | PR-1〜PR-9 |
| AC-34 | README の読み合わせ（PR のレビュー）、`TestReadmeSyncMarker`・`TestReadmeVersionsShareStructure` | manual、test | 8-6 | PR-8 |
| AC-35 | 各 PR のレビューでの ADR の読み合わせ。作業として、各 PR は PR を作る前に `git grep -nE 'issues/773([^0-9]|$)' origin/main -- docs/dev/adr/` で残る記述を §3.3 と照らし合わせる | manual | 4-7・5-6・8-7・9-6 | PR-4、PR-5、PR-8、PR-9 |

AC-15・AC-34・AC-35 は文書と文言の自然言語の内容だけを定める基準で、PR のレビューの読み合わせで確かめる。AC-33 は、文の内容を読み合わせで、コマンドの例を `_checks/removal.sh` で確かめる。

### AC-26 の記録

各 PR のブランチで `_checks/cli_unchanged.sh` を走らせ、PR ごとに次の形で書く（PR の番号、merge-base、比べたコミット、出力）。

（まだない）

### PR-6 の前の手動の確認の記録

ステップ 6-3 で書く。ブラウザの名前と版、確かめた組み合わせ（`GET /v1/models` と `POST /v1/messages`、`anthropic-workspace-id` のありとなし）、見たもの、履歴のタイトルの確かめの手順と結果、決めた案（A か B）を書く。

（まだない）

### PR-8 の手動の確認の記録

ステップ 8-8 で書く。確かめた項目ごとに、ブラウザと版、手順、結果を書く。VoiceOver の結果は、PR-8 の説明にも書く（設計書 §3.6）。

（まだない）

## 8. 成功の基準 (Success Criteria)

-   **機能:** 要件書の AC-01〜AC-35 が、§7 の確かめで満たされる。拡張のほかに何も導入していないプロファイルで、Chrome と Brave の両方で記事が完成する。
-   **品質:** 各 PR で、`make test`・`make lint`・`make deadcode` と、拡張を変えた PR で `make ext-check` が通る。テストを削除した commit は、カバレッジの比較が通る。
-   **セキュリティ:** 公開する成果物に fake が入らず、API キーが要件書 AC-20 の経路の外に出ず、送り先が拡張自身と `https://api.anthropic.com` に限られることを、§7 の `test` と `static` の確かめが示す。
-   **文書:** 最後の PR のマージの時点で、サーバとホストを主経路として述べる文書がなく、#773 にリンクする食い違いの記述を持つ有効な ADR がない。

## 9. 次のステップ (Next Steps)

-   PR-1 から順に `/runplan` で実装する。PR-6 の前に、ステップ 6-1〜6-3 の手動の確認を人と行う。
-   最後の PR のマージの後、issue #773 を閉じる（`/runpr` が案内する）。
-   Web Store の掲載と申請（#869）、シークレットモード（#874）は、本タスクの外で扱う（要件書 §2.3）。
