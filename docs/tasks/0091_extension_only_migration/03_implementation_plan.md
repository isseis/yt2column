# 実装計画書：拡張だけの経路への移行と、サーバとホストの撤去

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-16 |
| Review date | 2026-11-16 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。「0090 設計書」は [タスク 0090 の設計書](../0090_extension_only_architecture/02_architecture.md) を指し、設計書と同じく手段と値の正本とする。H-NN は [design_handoff.md](design_handoff.md) の項番で、どれも設計書の付録 A が扱いを記録している。本タスクに `implementation_handoff.md` はない。既存のファイルの内容と `file:line` の行番号は、コミット `bac09b3f` で確かめた。

用語は、要件書 §6、設計書 §1、0090 設計書 §1 と同じ意味で使う。本計画は、次の語を加える。

-   **切り替えの PR:** 拡張がサーバとホストを使わなくなる PR（PR-8）。
-   **撤去の PR:** サーバ、ホスト、開発用のサーバと、それだけが使うコードを除く PR（PR-9）。
-   **まだ使われない拡張の部品:** 切り替えの PR より前の PR で加え、切り替えの PR で入口から呼ばれるようになる拡張のモジュール（PR-6・PR-7）。入口のページのスクリプト（`popup.ts`・`viewer.ts`・`background.ts`）と manifest は、切り替えの PR まで変えない。
-   **呼ばれなくなった拡張のコード:** 切り替えの PR の後、入口から呼ばれなくなる、ホストとの送信、応答の検査、ポーリング、今の SW の調停のファイル。撤去の PR で除く（§1.3 の決定 10）。
-   **試作の拡張:** 手動の確認のためだけに、リポジトリの外（作業者の一時のディレクトリ）に作る拡張（§1.3 の決定 3）。リポジトリに入れない。
-   **確認のセッション:** `/runplan` の前に、人と対話で手動の確認を行うセッション。PR-1 と PR-6 の前に行う（§1.3 の決定 3）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

設計書と 0090 設計書のとおりに、記事生成モジュール（`cmd/yt2column-wasm`・`cmd/yt2column-wasm-dev`）、`internal/extjob` と 2 つの LLM の実装、開発用のコマンド `cmd/yt2column-extjob`、Go を呼ぶ拡張のビルドと開発用のビルド、オプションページ、ジョブの記録と SW の調停、実行するビューアを作り、拡張だけの経路に切り替える。その後、サーバ、ホスト、開発用のサーバを撤去し、仕様の文書と ADR を拡張だけの経路に合わせる。要件書の F-001〜F-007 を満たす。

### 1.2. 実装の原則

-   **切り替えより前の PR は、今の経路を壊さない。** PR-1〜PR-7 は、Go の部品、拡張のビルド、まだ使われない拡張の部品を加えるだけにする。入口のページのスクリプトと manifest を変えず、利用者から見える振る舞いは変わらない（設計書 §1.1）。新旧の経路を併存させる仕組みは作らない（要件書 AC-32）。
-   **0090 設計書と設計書を正本とし、計画は写さない。** 各ステップは、作るものと確かめることと、その正本の節を挙げる。関数の形、値の集合、期限の値は、正本の節を読んで実装する。
-   **各文書は、それが述べるコードを変える PR で書き換える（要件書 AC-33）。** 新しいパッケージの `package_reference.md` の行は、そのパッケージを加える PR で書く（`TestPackageReferenceListsPackages` が確かめる）。除くパッケージの行と `internal/strictjson` の利用者の一覧は、そのパッケージを除く削除だけの commit で直す。文書を直すのが後の commit になると、削除だけの commit で `TestPackageReferenceListsPackages`・`TestPackageReferenceStrictjsonUsers` が失敗し、`make coverage-compare` がカバレッジを比べる前に止まる。`make wasm-check` の対象を変える PR は、同じ PR で CLAUDE.md の説明を直す。
-   **ADR の食い違いの記述は、食い違いを解いた PR で除く（要件書 AC-35）。** 一部だけを解いた PR は、残る食い違いを述べる形に書き換える。どの PR がどの ADR の記述を除くかの見込みは §3.3 にある。除くことと書き換えることは、`docs/dev/conventions/adr.md` の「Editorial corrections」の 2 つ目の種類で、commit のメッセージの訂正の行と、PR の説明の訂正の段落に書く。PR を作る前に、`adr.md`「Discrepancy statements」の手順（`origin/main` で記述を探し、`HEAD` で残りを確かめる）で、§3.3 の見込みと実際の記述を照合する。
-   **テストの削除は、削除だけの commit でカバレッジを比べる（CLAUDE.md「Testing Strategy」）。** テストの関数か表の行を、その対象が残るまま除くときは、削除だけの commit にし、その親を `BASE` にして、Go は `make coverage-compare`、拡張は `make ext-coverage-compare` を走らせ、出力を commit のメッセージと PR の説明に書く。製品のファイルをそのテストとともに除く commit も同じ手順で比べる（除いた関数は失敗にならず、一覧に出る）。振る舞いを変えたために期待を直すことは、削除に当たらない。`make coverage-compare` は `-coverpkg=./...` で測るので（`internal/coveragecompare/target_go.go:40`）、除くテストだけが残るパッケージの文を実行していると、そのパッケージのカバレッジが下がり、比較は失敗する。そのときは、削除の前の commit で、残るパッケージのテストを足す。
-   **各テストは、壊して失敗を確かめる。** 確かめた結果を commit のメッセージに書く（CLAUDE.md「Testing Strategy」）。
-   **決まった上限と期限は、境界の前後をテストする。** 上限ちょうどを受理し、1 つ超えを拒否する行を置き、期限は差し込んだ時計で確かめる。対象は、入力の 4 MiB、Worker の結果と記録の 512 KiB、記録 50 件・合計 8 MiB・記事 256 KiB、同時に 2 つ、URL のタイトルの 64 コードポイント（設計書 §3.8.2 の案 B）、キーの確かめの 30 秒、Worker の読み込みの 30 秒、オプションページの検査の 90 秒、実行の 50 分である。
-   **`cmd/yt2column/` の `_test.go` でないファイルを変えない（要件書 AC-26）。** `cmd/yt2column/test_helpers*.go` も `_test.go` でないので変えない。変える必要が出たら、人と決める。
-   **実 API の呼び出しとネットワークを使う操作は、その都度、人の承認を得る（CLAUDE.md「Tool Execution Safety」）。** 確認のセッションの 401 の確かめと、PR-8 の本番の構成での確かめが当たる。拡張のビルドが `go.mod` の版のツールチェーンを初めてダウンロードするときも当たる（設計書 §1.2）。

### 1.3. 計画の際に決めたこと（2026-11-16）

1.  **PR を 9 つに分け、まだ使われない拡張の部品を、切り替えより前の 2 つの PR に入れる。** PR-6 は、API キーの経路のソースの検査、`chrome.storage` の決まった関数と lint の規則、Worker の呼び出し、Worker から受け取る結果の検査、ジョブの記録の検査、ページのタイトルを決める関数、オプションページの表示を決める関数を加える。PR-7 は、新しいビューアの URL と問い、オプションページの保存と削除の流れ、実行するビューアの流れ、新しい SW の調停を、この順に加える（状態機械の調停を最後に置く）。切り替えの PR（PR-8）は、入口の配線、HTML、manifest、文書のガードの撤去、文書を受け持つ。理由: 切り替えの PR には、配線に加えて、README を含む 8 つの文書の書き換えが入る（要件書 AC-33）。部品を先に入れれば、切り替えの PR のレビューを配線と文書に絞れる。PR-6 に API キーの経路の検査を入れるのは、PR-6 の新しいモジュールが、今の `secretsAbsent.test.ts` が拒否する語（`api_key` など）を持つためである。配線のときに部品の形の誤りが見つかると直しが要るが、部品は単体テストで振る舞いを固めてから入る。採らなかった案は、部品を 1 つの PR にまとめる案（SW の調停と実行するビューアが切り替えの PR に残る）、前倒ししない案（切り替えの PR がレビューできない大きさになる）、PR-7 を 3 つに分ける案（PR が 11 になる）である。ADR: なし（PR の区切りの決定で、本タスクの中で閉じる）。
2.  **切り替えと撤去を別の PR にし、切り替えの PR で、サーバとホストを述べる文書の節と、その節を読む文書のガードのテストを除く。** 除くテストは、`TestServerDocsContract`（`cmd/yt2column-server/docs_test.go:36`）、`TestReadmeHostExitCodes`（`cmd/yt2column-nmhost/main_test.go:439`）、`TestDevelopmentDocsDevServer`（`cmd/yt2column-devserver/docs_test.go:29`）、`extension/test/serverReasons.test.ts` と、それだけが使う補助である。切り替えの後、どの文書も、サーバとホストを主経路として、または動かすものとして述べない。ただし、まだあるコードを挙げる箇所（package_reference.md の行、project_overview.md のディレクトリの構成、CLAUDE.md の「Build Commands」と文書のガードのコマンド）は、撤去の PR まで、そのコードを撤去の予定のものとして残す（要件書 AC-33 の例外）。README には、移行の手順だけが残る（要件書 AC-34）。理由: AC-33 の例外を使って README の節を残すと、利用者が読む README に、切り替えから撤去までの間、使えない手順が残る。1 つの PR にすると、Go と拡張のテストの削除と 8 つの文書が同じ PR に入り、レビューできない。ADR: なし（PR の区切りの決定で、本タスクの中で閉じる）。
3.  **手動の確認は、試作の拡張で、2 回の確認のセッションで行う。** PR-1 の前のセッションは、`fetch` だけの試作の拡張で、拡張のページの送信が Anthropic の 401 を読めるかと、記録がないビューアの固定のタイトルが履歴の項目の動画のタイトルを書き換えるかを確かめる。PR-6 の前のセッションは、PR-5 までに `main` に入った記事生成モジュールを試作の拡張の Worker で読み込み、Go の送信でも 401 を読めることを確かめ直す。結果は §7 に書き、その PR のブランチに commit する。PR-1 と PR-6 の自動送信は `hold` にする。理由: 401 を読めるかは、PR-1・PR-3・PR-4 の `CheckKey` と `api_key_rejected` の設計の前提で、Go の部品を作る前に確かめれば、読めなかったときの作り直しがない。CORS の振る舞いは送信の形で決まり、WASM なしで確かめられる。タイトルの結果は PR-6 のタイトルの関数の前提である。確かめのためだけのコードを `main` に入れると、成果物の検査と lint の対象が増え、撤去も要る。ADR: なし（確かめの手段と時期の決定で、本タスクの中で閉じる）。
4.  **Worker のファイルの名前を、`extension/src/wasmWorker.ts`（Worker の中の読み込み）と `extension/src/wasmWorkerClient.ts`（ページの側の呼び出し）にする。** 設計書 §3.12 の `worker.ts`・`workerClient.ts` に当たる（設計書は「または同等のもの」とした）。理由: 今のコードの「Worker」は SW を指す（`createWorkerSend`、`extension/src/browser/chromeDeps.ts:320`、`pendingWorker`、`extension/test/helpers/chrome.ts:131`）。ADR: なし（ファイルの名前の決定である）。
5.  **まだ使われない部品のうち、今のファイルを書き換えると今の経路が壊れるものは、新しいファイルに作る。** 新しい SW の調停（`extension/src/jobCoordinator.ts`）、実行するビューアの流れ（`extension/src/viewerRunner.ts`）、オプションページの保存と削除の流れ（`extension/src/optionsFlow.ts`）、ページの間のメッセージと生成の開始の入力の大きさの検査（`extension/src/core/jobMessages.ts`）、ビューアの URL の組み立てと解析（`extension/src/core/jobViewerUrl.ts`）、ジョブ ID でビューアを探す問い（`extension/src/jobViewerQuery.ts`）である。0090 設計書 §3.11 の `startCoordinator.ts`・`viewerUrl.ts`・`viewerQuery.ts`・`extensionMessages.ts`・`prepare.ts` を「変える」に当たる。今のファイルは、切り替えの後に呼ばれなくなり、撤去の PR で除く（決定 10）。理由: 決定 1 で、まだ使われない部品を切り替えより前に入れるためである。今のファイルは今の SW の調停とビューアが使っている。ADR: なし（ファイルの分け方の決定である）。
6.  **設計書 §8 の挙げ方とは次の 2 点で区切りを変え、計画の決定として記録する（設計書は変えない）。** 1 つ目は、拡張のホストとの送信とポーリングの撤去を、§8 の 4（切り替え）でなく 5（撤去）の PR で行うこと（決定 10）。2 つ目は、サーバ、ホスト、開発用のサーバの文書のガードのテストを、それぞれのバイナリと一緒でなく（0090 設計書 §3.11 のテストの表）、文書の節を除く切り替えの PR で除くこと（決定 2）。理由: §8 は「PR の区切りは、実装計画書で決める。依存の順は次のとおりである」とし、どちらの変更も依存の順を保つ（撤去は切り替えの後、ガードの撤去は、ガードが読む節を除くのと同時）。`internal/server/response.go` を拡張の検査を走らせる変更の一覧から除くのは、設計書 §3.10 のとおり、`response.go` と `serverReasons.test.ts` の両方がなくなる撤去の PR にする。人が 2026-11-16 に、設計書を先に直さずに計画の決定として記録することを選んだ。ADR: なし（PR の区切りの決定で、本タスクの中で閉じる）。
7.  **今のサーバのテストの行は、PR-3 で `internal/extjob` のテストに写し、撤去の PR まで両方を走らせる（設計書 §1.2、H-03）。加えて、同じ入力をサーバの関数と `internal/extjob` の関数に通して出力を比べる一時的なテストを、`internal/server` のテストに置く。** 比べるテストは、`internal/server` の公開しない関数を呼べるサーバのパッケージのテストに置き、撤去の PR でサーバとともに除く。写した行の対応（サーバのテストの名前 → `internal/extjob` のテストの名前）を、PR-3 の説明に表で書く。撤去までの間、サーバの関数と `internal/extjob` の関数は重複する。`.golangci.yml:36` が有効にする `dupl` は、実装では写した関数の組を指摘しなかったので、`//nolint:dupl` は付けない。理由: 写した期待の値の誤りは、写したテストだけでは見つからない（両方のテストが、それぞれのコードに対して通る）。比べるテストなら、同じ入力で 2 つの実装の出力が違えば失敗する。ADR: なし（設計書の決定を確かめる手段の追加で、本タスクの中で閉じる）。
8.  **AC-26 は、各 PR で、`docs/tasks/0091_extension_only_migration/_checks/cli_unchanged.sh` を走らせて確かめる。** 要件書の「`cmd/yt2column/` の `_test.go` でないファイルの差分はない」は、変更の前後の関係を述べる基準である。スクリプトは PR-1 で作る。各 PR の完了の条件として、そのブランチで走らせ、出力と 2 つのコミット ID を §7 に書く。ADR: なし（受け入れ基準の確かめ方の決定である）。
9.  **撤去の PR は、ADR の決定の文のうち、撤去をこれから行うこととして #773 にリンクする 2 か所を、撤去した後の形に直す。** ADR-0045 の「撤去と置き換えは #773 が行う」（`docs/dev/adr/0045-dev-fake-llm.md:25`）と、ADR-0048 の「結果」の「#773 で撤去する」（`docs/dev/adr/0048-extension-only-generation.md:48`）である。どちらも食い違いの記述ではないが、撤去の後は事実と合わなくなる。決定は変わらないので、編集上の訂正として行う。廃止した ADR-0005・ADR-0018 の「廃止の理由」の行は、廃止の時点の記録なので直さない。ADR: なし（編集上の訂正である）。
10. **呼ばれなくなった拡張のコードとそのテストは、撤去の PR（PR-9）で除く。** 対象は、0090 設計書 §3.11 の「撤去」の行（`send.ts`・`poller.ts`・`core/native.ts`・`core/hostMessage.ts`・`core/exchange.ts`・`core/response.ts`・`core/poll.ts`）と、決定 5 で新しいファイルに置き換えた今のファイル、`core/articleKey.ts`、`test/helpers/native.ts`、それらのテストと、拡張と `internal/nmhost` の取り決めを確かめる `TestNativeHostContract`（`cmd/yt2column/extension_docs_test.go:159`。拡張の `send.ts`・`core/native.ts`・`core/exchange.ts` を読む）である。切り替えの PR で直すファイルの型が変わり、そのために型の検査を通らなくなるものだけは、切り替えの PR で除く（そのファイルを `TestNativeHostContract` が読むときは、そのテストも同じ commit で除く）。理由: 切り替えの PR を、配線、文書のガード、文書に絞るためである。呼ばれないファイルは、残っていてもビルドとテストを通る。人が 2026-11-16 に選んだ。ADR: なし（PR の区切りの決定で、本タスクの中で閉じる）。
11. **`make ext-build-dev` を `make ext-check` と CI の `extension` の手順に加える。** 理由: 開発用のビルドは PR-5 で入るが、PR-8 まで画面で使わないので、PR-6〜PR-8 の変更が開発用のビルドを壊しても気づかない。ADR: なし（検査の範囲の決定である）。

### 1.4. 既存コード調査結果

コミット `bac09b3f` で確かめた。設計書 §3 が挙げる箇所のほかに、計画に効くものを挙げる。

**Go の LLM の層と設定**

-   `internal/llm/llm.go:12-19` の番兵は `ErrInvalidRequest`・`ErrTruncated`・`ErrUnexpectedFinishReason`・`ErrEmptyResponse` で、`LLMClient` の doc コメント（`:65-75`）がそれを挙げる。`ErrAuthRejected` はない。
-   `internal/llm/claude/errors.go:33` の `HTTPStatusError` は `StatusCode` だけを持ち、`Unwrap` で `ErrHTTPStatus` を返す（`:44`）。直接の単体テストはなく、`claude_test.go::TestGenerateHTTPStatus`（`:500`）が送信を通して確かめる。
-   `internal/llm/claude/request.go:105-123` の `newRequest` は、`anthropic-version` のヘッダを付け、パッケージで唯一の `Reveal` の呼び出しで `x-api-key` を付け、ワークスペース ID があるときだけ `anthropic-workspace-id` を付ける。テストは `test_helpers_endpoint.go` のループバックの送り先の差し替えを使う。
-   `internal/llm/llmhttp/llmhttp.go` は `CheckAPIKey`（`:46`）、`Call`（`:81`）、`Post`（`:94`）を持つ。`Status` はない。テストの送り先は `internal/llm/llmhttp/llmhttptest` にある。`js` のビルドだけのファイルは、`internal/llm/claude` にも `internal/llm/llmhttp` にもない（リポジトリで唯一の `//go:build js` は `internal/writer/template_open_js.go`）。
-   `internal/llm/provider/provider.go:25` の `LLMTimeout` は 15 分で、Claude のクライアントは `:93-104` で組み立てる。記事生成モジュールが `provider` を使わない理由は設計書 §3.3 にある。
-   `GOOS=js GOARCH=wasm go build ./internal/config` は、`internal/config/serverfile.go:13` の `internal/privatefile` の import（`privatefile.go:1` が `//go:build unix`）で失敗する（`bac09b3f` で走らせて確かめた。出力の最後の行は `build constraints exclude all Go files in .../internal/privatefile`）。`serverfile.go`・`configpath.go`・`socketpath.go` はビルドタグを持たない。`LoadServer`（`config.go:370`）・`LoadHost`（`:425`）・`LoadHostHome`（`:438`）は `config.go` にある。
-   サーバの設定の読み込みは、サーバのほかからも使われる。`ParseServerFileForTest`（`internal/config/test_helpers.go:130`）を使うのは `cmd/yt2column/docs_test.go:499` だけである。`ServerLookupForTest`（`:194`）と `LoadServer` は、`cmd/yt2column/docs_test.go:558,649`（どちらも `TestProductionProfileDocsContract` の補助）と `internal/llm/provider/provider_test.go:1058`（`TestNewSendsStageEffort`）が使う。`LoadServer` は、`TestLoadClaudeEffortDefault`（`config_test.go:302`）と `TestLoadHTTP2Debug`（`:1006`）の中のサーバの行からも呼ばれる。この 2 つのテストには、同じ値の CLI の行がすでにある。
-   `TestFakesCarryBuildTag`（`internal/pipeline/pipeline_test.go:554`）は、`test_helpers*.go` の 1 行目に `//go:build test` か `//go:build test || integration` だけを許す。テストの補助のファイルに `!js` を付けられないので、`internal/config` の補助は動かさない（WASM のビルドはテストを含まない）。
-   すべてのファイルが `//go:build js` のパッケージは、ネイティブの `go build ./...`・`go vet ./...`・`go test ./...`・`deadcode` で飛ばされる（レビューの際に小さなパッケージを作って確かめた）。`GOOS=js GOARCH=wasm go vet ./internal/llm/claude ./internal/llm/llmhttp` は今も通る。

**fake と開発用のサーバ**

-   `internal/llm/devfake/scenario.go:171` の `Scenario` は公開しないフィールドを持ち、`NewScenario`（`:185`）だけが作る。`Failure` の値は `:42-48`、綴りは `:77-92`、`ErrSimulatedAPIFailure` は `:22`。キーの確かめの結果と `ParseScenario` はない。
-   開発用のサーバの引数の解析は `cmd/yt2column-devserver/args.go:130` の `parseArgs` で、テストは `run_test.go::TestRunArgsAccepted`（`:58`）・`TestRunArgsRejected`（`:88`）。`TestScenarioCombinations`（`:294`）は組み合わせの数と失敗の値の対応を固定する。`TestDevelopmentDocsDevServer`（`docs_test.go:29`）は、development.md の受理する値の表を `devfake` の値と照合する。
-   `internal/llm/devfake/deps_test.go:10-16` は `internal/server`・`articlecache`・`serversock`・`nmhost`・`privatefile` を import するので、撤去の PR で一覧を直さないとコンパイルできない。`internal/llm/devfake/articles/elements.md:12` は fake の記事の文の中で `make build-dev` に触れる。

**サーバの処理とテスト（`internal/extjob` に写すもの）**

-   `internal/server/request.go:63` の `validateBody`、`generation.go:66` の `completeGeneration`、`:146-163` の `classifyWrite`（順は `writer.ErrPromptTooLarge` → `context.DeadlineExceeded` → `llm.ErrEmptyResponse` → `llm.ErrTruncated` → 内部の失敗 → `FailureLLM`）、`status.go:131` の `articleStatus`、`internal/articlecache/key.go:81` の `Keyer.Key`。
-   写すテストは `request_test.go`（382 行、7 つのテスト）、`status_test.go`（404 行、9 つ）、`generation_test.go`（442 行、9 つ）で、補助は `internal/server/test_helpers.go` にある（サーバのパッケージの中だけで使える）。

**依存の検査とガード**

-   `internal/godepstest` は darwin と linux だけを対象にする。使い手は `cmd/yt2column`・`cmd/yt2column-server`・`cmd/yt2column-nmhost` の `devdeps_test.go::TestNoDevDependency` で、どれも `devfake` と `./cmd/yt2column-devserver` を許さない。`godepstest_test.go:48,78` は `internal/server` を例に使う。
-   `internal/config/envaccess_test.go` は、`allowedEnvRefs`（`:65-91`）にサーバ・ホスト・開発用のサーバの `main.go` を持ち、`:407-411` でホストと開発用のサーバの `os.LookupEnv` の観測を確かめる。
-   `internal/pipeline/relay_guard_test.go` の `hostFiles`（`:405`）は、ホストのファイルがないと `t.Fatal` する。`TestHostStdoutOnlyInMain`（`:492`）と `TestHostSpawnsNoProcess`（`:507`）は、撤去の PR でホストとともに除く。`TestSharedTokenRemoved`（`:328`）が拒否する語は `authtoken`・`YT2COLUMN_SERVER_PORT`・`server-token`（`:323`）だけで、`sharedToken`・`serverPort` は `extension/test/secretsAbsent.test.ts:38-45` の `token`・`serverPort` が拒否している。
-   `TestPackageReferenceStrictjsonUsers`（`internal/pipeline/strictjson_users_test.go:657`）は、package_reference.md の `internal/strictjson` の利用者の行（今は `internal/server` と `internal/nmhost` を含む）を import と照合する。`internal/pipeline/pipeline_test.go:540-547` の `testOnlyPackageDirs` は、テストだけのパッケージを挙げる。
-   `internal/udstest` は、残る `internal/privatefile/privatefile_test.go`・`internal/config/config_test.go`・`internal/llm/provider/provider_test.go` も使うので、撤去の PR の後も残る。
-   `cmd/yt2column/makefile_test.go` の `TestMakeBuildBuildsEveryBinary`（`:257`）と `TestMakeBuildDev`（`:280`）は、`make build` の 3 つのバイナリと `build-dev` を固定している。`make wasm-check` の対象を固定するテストはない。`TestExtensionDocumentation`（`extension_docs_test.go:56`）は、`ext-` の目標がすべて CLAUDE.md にあることと、security.md の `### 8.1. ブラウザ拡張（#110）` の `**要求する権限**` の一覧が manifest と同じであることを確かめる。
-   `Makefile:328-333` の `wasm-check` は 5 つのパッケージをビルドし、`internal/writer` と `internal/transcript` を `vet` する。`make lint` は `wasm-check` を含む（`:318`）。`make deadcode` は `-test -tags test,integration ./...` で、テストからだけ呼ばれる関数も到達できるものに数える（`:372`）。
-   `.github/workflows/ci.yml` の `check-changes` の手順は Go を用意しない。`extension` の手順は、`ext-build` の後で Go を用意する（`:154-163`）。`extension/test/repository.test.ts:161-174` が手順の順を固定する。`doc-guard`（`:191`）のコマンドは CLAUDE.md:703 と同じ 7 つのパッケージで、`.claude/commands/_lib/task-pr-handoff.md:40`・`.claude/commands/_lib/simple-process.md:429`・`docs/dev/guides/fixprguard_guide.md:595` の写しは、それぞれ少しずつ違う。写しが同じであることを確かめるテストはない。
-   `.golangci.yml` は、撤去するパッケージを名指す規則を 6 つ持つ（`:91-162`）。`:36` で `dupl` を有効にし、`_test.go` だけを除く。

**拡張**

-   設計書と本計画が加えるファイル（オプションページ、Worker、記録、結果、タイトル、ビルドの種類）は、どれもまだない。Worker と WebAssembly を参照するコードもない。
-   `extension/eslint.config.js` は、`storage`（`:44-52`）、`session`（`:94-102` の `menuPathNames`）、`fetch` などの送信（`:58-82`）を、`chromeDeps.ts` を含むすべてのファイルで拒否する。後の設定が `no-restricted-syntax` の一覧を丸ごと置き換えるので、例外はファイルごとの設定で一覧を組み直す（`:175-177`）。`no-restricted-imports` は使っていない。型の印（`UsdAmount` など）を作ってよいファイルは `creatorConfig`（`:235-302`）が決め、`UsdAmount` は `core/response.ts` だけが作る。`:251` の `ignores` は `dist/` と `node_modules/` だけで、`extension/.prettierignore` も同じである。
-   `extension/test/secretsAbsent.test.ts` は、`src/` と `static/` の中の「api key」「apikey」「api_key」「token」「serverPort」などの語を、識別子・コメント・文字列のどこでも拒否する（`:14-45`）。
-   `extension/scripts/check-dist.ts` は、`dist/` を、`src/` の `.ts` ごとの `.js` と `static/` の写しだけに限る（`:50-62`）。manifest の参照は `action.default_popup` と `background.service_worker` だけを確かめる（`:130-133`）。
-   `extension/tsconfig.json:4` の `lib` は `["ES2022","ES2023.Intl","DOM"]` で、WebWorker を含まない。`package.json` の `typecheck` は `tsconfig.json` と `tsconfig.core.json` を、`build` は `tsconfig.build.json` を使う。
-   `extension/test/manifest.test.ts` は、権限が 3 つであること（`:96-103`）、最上位のキーの集合（`:107-122`）、`minimum_chrome_version` が 110 で両方の README がその値を述べることを確かめる。`metadataDocs.test.ts` は README の `## ブラウザ拡張` の節の語を、`jobCodes.test.ts` は development.md の `### ジョブの API のコード` の表を読む。`style.test.ts:56-59` は、確かめるスタイルシートの一覧を固定する。
-   `extension/scripts/has-extension-changes.sh:20` は、`internal/server/response.go` を拡張の検査を走らせる変更に数え、`extension/test/ciChanges.test.ts` がそれを固定する。
-   今の SW の調停（`extension/src/startCoordinator.ts`、462 行）は、メモリの中の集合と期限だけで直列にしている（`:167-203`）。ビューアの URL は `core/viewerUrl.ts` の `viewerQuery`（`h`・`c`・`t`・`u`・`s`・`k`・`e`）で作る（`startCoordinator.ts:375`）。

**文書**

-   README の `## localhost サーバ`（`README.md:128-392`）と `## ブラウザ拡張`（`:498-`）、development.md の 3 つのサーバの節（`docs/dev/guides/development.md:26,54,102`）、security.md の §8（`docs/dev/security.md:99-181`）、project_overview.md の `:151`（`TestServerDocsContract` が読む）が、サーバとホストを今の経路として述べる。`TestReadmeLinks`（`cmd/yt2column/docs_test.go:697`）は、`development.md:105,210` と `cli.md:234` から README のサーバの節へのリンクを確かめる。
-   README の `## 設定`（`README.md:393`）の変数の表は CLI の環境変数を述べ、`TestLLMDefaultsDocsContract`（`docs_test.go:279`）と `TestEffortValuesDocsContract`（`:332`）が読む。表の後の段落（`:404-405`）がサーバの設定ファイルに触れる。
-   `TestProductionProfileDocsContract`（`docs_test.go:666`）は、README の `### 推奨構成` と production_profile.md の `## 本番の構成` の塊を、サーバの設定ファイルの解析（`ParseServerFileForTest`）で読む。
-   CLAUDE.md は、`TestReadmeHostExitCodes` と `serverReasons.test.ts`（「Development Notes」`:678-702`）、`TestServerDocsContract` のソケットのパスの見出し（「Go Idioms」`:778-779`）に触れる。

### 1.5. 置き換えの名前の表

本タスクで、今のものを新しい名前のもので置き換えるものを挙げる。ほかの節は、この表に従う。

| 今の名前 | 後の名前 | 新しいものを加える PR | 今のものを除く PR |
|---|---|---|---|
| `extension/test/secretsAbsent.test.ts` | `extension/test/apiKeyPath.test.ts` | PR-6 | PR-6 |
| 設計書の `worker.ts`・`workerClient.ts` | `extension/src/wasmWorker.ts`・`extension/src/wasmWorkerClient.ts` | PR-6 | ― |
| `extension/src/core/viewerUrl.ts` | `extension/src/core/jobViewerUrl.ts` | PR-7 | PR-9 |
| `extension/src/viewerQuery.ts` | `extension/src/jobViewerQuery.ts` | PR-7 | PR-9 |
| `extension/src/core/extensionMessages.ts` の生成の開始の型と、`extension/src/core/prepare.ts` の本文の組み立て | `extension/src/core/jobMessages.ts` | PR-7 | PR-9 |
| `extension/src/startCoordinator.ts` | `extension/src/jobCoordinator.ts` | PR-7 | PR-9 |
| 記事のキー（`extension/src/core/articleKey.ts`） | ジョブ ID | PR-7 | PR-9 |

### 1.6. 実装の際に決めたこと（2026-11-16）

1.  **2 つの記事生成モジュールが共有する `syscall/js` の登録を、`js` のビルドだけのパッケージ `internal/extjob/jsmodule` に置く（ステップ 4-1・4-2・4-4・4-7）。** 2 つの `main` は、LLM の実装から作った `extjob.Module` を `jsmodule.Serve` に渡すだけを持つ。理由: 登録の処理（約 50 行）は 2 つの `main` で同じで、それぞれに置くと片方だけを直す誤りが起きる。採らなかった案は、登録の処理をそれぞれの `main` に置く案（本計画の当初の形）である。人が 2026-11-16 に、PR-4 の実装の際に選んだ。設計は設計書 §1.4 にある。ADR: [ADR-0049](../../dev/adr/0049-wasm-core-sharing.md) を更新した。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: LLM の失敗の分類と、キーの確かめ（設計書 §8 の 1 のうち LLM の層）

**変更するファイル:** `internal/llm/llm.go`、`internal/llm/claude/`（`errors.go`・`request.go`、キーの確かめのファイル、`js` と `!js` のファイルの組）、`internal/llm/llmhttp/`（`llmhttp.go`、`js` と `!js` のファイルの組）とそのテスト、`Makefile`、`CLAUDE.md`、`docs/dev/conventions/package_reference.md`、`docs/tasks/0091_extension_only_migration/_checks/cli_unchanged.sh`（新規）、本計画

-   [x] **ステップ 1-1**（PR-1 の `/runplan` の前に、確認のセッションで行う。§1.3 の決定 3）: 次の 2 つを確かめ、結果を §7 の「PR-1 の前の手動の確認の記録」に書き、PR-1 のブランチに commit する。このステップのチェックボックスに印を付けて同じ commit に入れ、commit の件名は `docs(0091): draft` で始めない（`_lib/task-pr-handoff.md` Part A）。
    -   401 を読めるか（設計書 §1.2・§7.2）。試作の拡張に、`host_permissions` を持たず、CSP を 0090 設計書 §3.7.2 の値にしたページを置く。Chrome と Brave で、誤ったキーで `GET /v1/models?limit=1` と `POST /v1/messages` を `fetch` で送り、`anthropic-workspace-id` のありとなしの両方で、状態コード 401 を読める（通信の失敗にならない）ことを確かめる。ヘッダは、公開用のビルドが送るもの（`x-api-key`・`anthropic-version`・`content-type`・`anthropic-dangerous-direct-browser-access`）と同じにする。実 API の呼び出しなので、人の承認を得る。料金はかからない。読めないときは、先へ進まない。人と決め直し、設計書と本計画を決定の変更として改め、ADR-0010 の保存のときの確かめを改める（設計書 §3.14）。
    -   履歴の項目のタイトルが書き換わるか（設計書 §3.8.2、H-05）。試作の拡張に、`chrome.storage.session` に値があれば動画のタイトルを、なければ固定のタイトルを出すページを置く。Chrome と Brave で、ページを開き、ブラウザを再起動してセッションを復元し、履歴の項目のタイトルを読む。上書きされなければ案 A、されれば案 B で実装すると決める。
-   [x] **ステップ 1-2**: `llm.ErrAuthRejected` を加え、`LLMClient` の doc コメントの番兵の一覧を改める（設計書 §3.1）。Claude アダプタの状態コード 401 の `HTTPStatusError` が `errors.Is(err, llm.ErrAuthRejected)` に当たるようにする。`errors.AsType[*claude.HTTPStatusError]` で状態コードを取れること、`ErrHTTPStatus` に当たること、エラーの文は、いずれも変えない（要件書 AC-26）。テストは `internal/llm/claude/errors_test.go` に置き、401 で当たり、400・403・429・500 で当たらないことと、どの状態コードでも `ErrHTTPStatus` に当たることを確かめる。
-   [x] **ステップ 1-3**: `llmhttp.Status` を加え、パッケージの doc コメントを「1 回の POST」から改める（設計書 §3.1）。テストは `llmhttptest` の送り先で、状態コードを返すこと、本文を読まずに閉じること、リダイレクトに従わないことを確かめる。期限切れと通信の失敗の分類は、`Post` の今のテストの行を、表を共有して `Status` にも当てる。
-   [x] **ステップ 1-4**: `claude.CheckKey` と `KeyCheckOptions` を加える（設計書 §3.1）。`GET https://api.anthropic.com/v1/models?limit=1` を `llmhttp.Status` で 1 回送る。ヘッダは、`newRequest` のヘッダを付ける部分を切り出した 1 つの関数で付け、`Reveal` の呼び出しを 1 か所に保つ。テストは、ループバックの送り先の差し替え（`test_helpers_endpoint.go`）を使い、200 で `nil`、401 で `llm.ErrAuthRejected` に当たるエラー、ほかの状態・通信の失敗・期限切れで当たらないエラーを返すこと、`anthropic-version` を送ること、ワークスペース ID を指定したときだけ `anthropic-workspace-id` を送ること、API キーを `x-api-key` だけで送ることを確かめる。
-   [x] **ステップ 1-5**: ブラウザから呼ぶための指定を、`js` と `!js` のファイルの組で加える（設計書 §3.1、0090 設計書 §5.3）。`llmhttp` は、`Post` と `Status` が同じ 1 つの関数を通してリクエストに `js.fetch:redirect: error` を付け、`internal/llm/claude` は、`Generate` と `CheckKey` のリクエストの組み立てが同じ 1 つの関数を通して `anthropic-dangerous-direct-browser-access` のヘッダを付ける。`!js` の側は何もしない。テストは、ネイティブのテストで `Post`・`Status`・`Generate`・`CheckKey` がその関数を通ることと、`go/parser` で `js` のファイルを読み、それぞれの関数が決まった値を付けることを確かめる。`Makefile` の `WASM_PACKAGES` に `./internal/llm/llmhttp` を、`vet` の対象に `internal/llm/llmhttp` と `internal/llm/claude` を加え（設計書 §3.10）、CLAUDE.md の `make wasm-check` の説明を同じ対象に直す。
-   [x] **ステップ 1-6**: package_reference.md の `internal/llm`（`ErrAuthRejected` と、それを返すアダプタが Claude だけであること）、`internal/llm/claude`（`CheckKey`）、`internal/llm/llmhttp`（`Status`）の行を改める（設計書 §3.13）。
-   [x] **ステップ 1-7**: `_checks/cli_unchanged.sh` を作る（§1.3 の決定 8）。merge-base と `HEAD` の間で、`cmd/yt2column/` の `_test.go` でないファイルの差分がないことを確かめ、差分があれば失敗する。検査が、少なくとも 1 つの壊し方で失敗することを確かめる。

**完了の条件:** `make test`・`make lint`（`make wasm-check` を含む）・`make deadcode` が通る。`_checks/cli_unchanged.sh` が通り、§7 の AC-26 の記録に書いた。§7 に PR-1 の前の手動の確認の記録がある。

### PR-1 作成ポイント: LLM auth sentinel, status-only send, and key check

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7

**推奨タイトル**: `feat(0091): キーの拒否の番兵、本文を読まない送信、Models API でのキーの確かめを加える`

**レビュー観点**: 手動の確認の記録が 2 つのブラウザとヘッダのありとなしの組み合わせをすべて含み、案の選択が結果と合うか / 401 だけが `ErrAuthRejected` に当たり、CLI のエラーの文と分類が変わらないか / `CheckKey` が `newRequest` と同じヘッダを同じ関数で付けるか / `js` のファイルの指定が `Post` と `Status`、`Generate` と `CheckKey` の両方に付くか

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 1-5 は、ネイティブのテストに見えない `js` のビルドだけの送信の守りで、孤立した高いリスクのステップに当たる。実現性の確かめのステップ 1-1 は `/runplan` の前の確認のセッションで済み、`/runplan` はその結果を前提に進む。Sonnet が実装すると、`Status` の側の経路を共通の関数に通さずに指定を付け忘れても、ビルドが通ったまま残りやすい。

**自動送信**: hold

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/893
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 設定の読み込みと fake のシナリオ（設計書 §8 の 1 のうち `internal/config` と `devfake`）

**変更するファイル:** `internal/config/`（`config.go`、`!js` のタグを付けるファイル、拡張用の読み込みのファイルとそのテスト）、`internal/llm/devfake/`、`cmd/yt2column-devserver/args.go` とそのテスト、`Makefile`、`CLAUDE.md`、`docs/dev/guides/development.md`、`docs/dev/conventions/package_reference.md`

-   [x] **ステップ 2-1**: `LoadServer`・`LoadHost`・`LoadHostHome` と `ServerConfig`・`HostConfig` を、`//go:build !js` を付けた新しいファイルに移し、`serverfile.go`・`configpath.go`・`socketpath.go` にも同じタグを付ける（設計書 §3.2）。テストの補助のファイルにはタグを付けない（§1.4）。これで `GOOS=js GOARCH=wasm go build ./internal/config` が通る。続けて `Makefile` の `WASM_PACKAGES` に `./internal/config` を加え、CLAUDE.md の `make wasm-check` の説明を直す。
-   [x] **ステップ 2-2**: `LoadExtension`・`ExtensionLookup`・`ReadExtensionSettings` を加える（設計書 §3.2、0090 設計書 §3.1.2・§3.1.3）。テストは、`LoadExtension` が 2 つの名前のほかを読まず、本番の構成になり、CLI と同じ値を拒否すること（0090 設計書 §7.1）、`ExtensionLookup` が 2 つの名前のほかに答えないこと、`ReadExtensionSettings` が 2 つの値とあるかどうかを検査せずに返すことを確かめる。秘密情報の変数名は `internal/config` の外に書かない（`TestEnvAccessConfined`）。
-   [x] **ステップ 2-3**: `devfake` に、失敗の種類 `FailureAPIKeyRejected`（綴りは `api_key_rejected`、エラーは `llm.ErrAuthRejected` に当たる）、キーの確かめの結果 `SettingsCheck`（`ok`・`key_rejected`・`unverified`、ゼロ値は `ok`）とそれを返す関数、`ScenarioSpelling` と `ParseScenario` を加える（設計書 §3.4）。依存は `internal/llm` と標準ライブラリのままである。テストは、追加した値の振る舞い、シナリオの 5 つの値のそれぞれで、空の値と合わない綴りを拒否すること（`ErrInvalidScenario`）、遅れの上限の境界、補正しないことを確かめる。
-   [x] **ステップ 2-4**: 開発用のサーバの引数の解析を `devfake.ParseScenario` に替える（設計書 §3.4）。開発用のサーバには、キーの確かめの結果の引数を加えず、`ok` を渡す。開発用のサーバの引数のテストは、引数の解析の経路を確かめるので残し、綴りの規則の行は `ParseScenario` のテストに置く。`-fail` が `api_key_rejected` を受理するようになるので、`TestScenarioCombinations` の数と失敗の値の対応（開発用のサーバでは `llm_failed` になる）と、development.md の開発用のサーバの受理する値の表を直す（`TestDevelopmentDocsDevServer` が確かめる）。
-   [x] **ステップ 2-5**: package_reference.md の `internal/config`（3 つの関数と、`!js` のファイルが撤去までの間だけのものであること）と `internal/llm/devfake`（追加した値と `ParseScenario`）の行を改める。

**完了の条件:** `make test`・`make lint`（`GOOS=js GOARCH=wasm` での `internal/config` のビルドを含む）・`make deadcode` が通る。`_checks/cli_unchanged.sh` が通り、§7 の AC-26 の記録に書いた。

### PR-2 作成ポイント: extension config loading and fake scenario parsing

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5

**推奨タイトル**: `feat(0091): 拡張用の設定の読み込みと、fake のシナリオの解析を加える`

**レビュー観点**: `!js` のタグがサーバとホストの読み込みだけに付き、CLI の読み込みが変わらないか / `ExtensionLookup` と `ReadExtensionSettings` が 2 つの名前だけを扱い、変数名が `internal/config` の外に出ないか / `ParseScenario` が補正せずに拒否し、開発用のサーバが新しい失敗の値を `llm_failed` として示すか

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない。宣言の移動とビルドタグ、決まった形の関数の追加で、正本の形が設計書 §3.2・§3.4 にあり、秘密情報の変数名の置き場所は今の `TestEnvAccessConfined` が確かめる。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/896
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: `internal/extjob` と 2 つの LLM の実装（設計書 §8 の 1 のうち `internal/extjob`）

**変更するファイル:** `internal/extjob/`（新規）、`internal/extjob/claudebackend/`（新規）、`internal/extjob/devbackend/`（新規）とそのテスト、`internal/server/`（比べるテスト）、`Makefile`、`CLAUDE.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/project_overview.md`

-   [x] **ステップ 3-1**: `internal/extjob` に、`Prepare`・`Prepared`・`Run`・`Result`・`Failure`・`Observer`・`Deps`・`ErrInvalidInput` と、`Backend` の interface を作る（0090 設計書 §2.3・§3.1.2、設計書 §3.3）。入力の検査は今の `validateBody` を、`inputDigest` は `Keyer.Key` から秘密の鍵を除いたものを、完了の時点の検査は `completeGeneration` を、結果は `articleStatus` の規則を、それぞれ写す（§1.3 の決定 7）。失敗の分類は `classifyWrite` に、`llm.ErrAuthRejected` を `FailureAPIKeyRejected` にする規則を、設計書 §3.3 の位置に加える。`Run` は panic を `FailureInternal` にする。`dupl` は写した関数の組を指摘しなかったので、`//nolint:dupl` は付けない（§1.3 の決定 7）。
-   [x] **ステップ 3-2**: サーバの `request_test.go`・`status_test.go`・`generation_test.go` の行を、`internal/extjob` のテスト（`prepare_test.go`・`result_test.go`・`run_test.go`）に写す（要件書 AC-19、H-03）。補助は `internal/extjob/test_helpers.go`（`//go:build test`）に置く。失敗の値は 0090 設計書 §4.1 と設計書 §4.1 の集合にする。加えて、`inputDigest` が同じ入力で同じで、どの値を変えても変わること、工程の進み具合の通知、3 つの工程のどれで `llm.ErrAuthRejected` が返っても `api_key_rejected` になること、入力の 4 MiB の境界を確かめる（0090 設計書 §7.1）。同じ入力をサーバの関数と `internal/extjob` の関数に通して、拒否の理由、表示用の Markdown、保存するファイルのバイト列、推定の料金、失敗の分類が同じであることを確かめる一時的なテストを（失敗の分類は、値の集合の違いを対応させて比べ、`llm.ErrAuthRejected` の入力は比べない。サーバはそれを `llm_failed` にし、`internal/extjob` は `api_key_rejected` にするので、こちらは上の写したテストで確かめる）、`internal/server` のテストに置く（§1.3 の決定 7）。写した行の対応表を PR の説明に書く。
-   [x] **ステップ 3-3**: `claudebackend.New` を作る（設計書 §3.3）。本番の構成の Claude のクライアントと `claude.CheckKey` を使い、呼び出しごとの期限 15 分を持つ。テストは、クライアントの組み立て（`claude.Options` の値、検証・推敲のクライアントの選び方）と期限が `internal/llm/provider` と同じであることを、ネイティブのビルドで確かめる。
-   [x] **ステップ 3-4**: `devbackend.New` を作る（設計書 §3.4）。テストは、シナリオごとのクライアントとキーの確かめの結果と、`devbackend` の依存に `internal/llm/claude` と `os/exec` が入らないこと（`go list -deps`）と、`devbackend` 自身のファイルが `net` と `internal/llm/claude` を import しないことを確かめる。`net` を依存の全体から除くことはできない（`internal/extjob` が API キーの検査に使う `llmhttp` が `net/http` を import する）。
-   [x] **ステップ 3-5**: `BackendInfo`・`ClaudeInfo`・`FakeInfo`・`Module` と 4 つの JSON の入口を、設計書 §3.3 と 0090 設計書 §3.1.2 のとおりに作る。テストは次を確かめる。
    -   閉じたメンバの集合の拒否（`prepare` の拒否と `generate` の `request_shape`）。
    -   保存した設定の検査の各規則（`settings_invalid` で LLM の実装を呼ばないこと、ワークスペース ID の空の文字列が指定なしになること、前後に空白のある値を補正しないこと）と、`input_mismatch`。
    -   `checkSettings` の 4 つの結果、形の検査を通った後だけ `Backend.CheckKey` を呼ぶこと、キーの確かめの 30 秒の期限が `unverified` になること。
    -   結果と進み具合の JSON に API キーが入らないこと。
    -   `BackendInfo` のゼロ値で `buildInfo` が実装を名乗らないこと。`BackendInfo` を作るのは 2 つの関数だけで、同じパッケージの中のゼロ値の組み立ては、このテストで確かめる。
-   [x] **ステップ 3-6**: `Makefile` の `WASM_PACKAGES` に `./internal/extjob/...` を加え、CLAUDE.md の `make wasm-check` の説明を直す。package_reference.md に 3 つのパッケージの行を加え、`internal/strictjson` の利用者に `internal/extjob` を加える（`TestPackageReferenceStrictjsonUsers`）。project_overview.md のディレクトリの構成に 3 つのパッケージを加える（設計書 §3.13）。

**完了の条件:** `make test`・`make lint`・`make deadcode` が通る。サーバのテスト、写した `internal/extjob` のテスト、比べるテストが通る。`_checks/cli_unchanged.sh` が通り、§7 の AC-26 の記録に書いた。

### PR-3 作成ポイント: extension job core and the two LLM backends

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6

**推奨タイトル**: `feat(0091): 拡張のジョブの処理 internal/extjob と、Claude と fake の LLM の実装を加える`

**レビュー観点**: 写したテストの行がサーバのテストと同じ期待を持つか（PR の説明の対応表と、比べるテスト） / 保存した設定の検査が補正せず、`settings_invalid` で LLM の実装を呼ばないか / API キーが結果と進み具合と保持する値に残らないか / `//nolint:dupl` が写した関数の組だけに付くか

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 3-5 は API キーを受け取る JSON の境界で、孤立した高いリスクのステップに当たる。Sonnet が実装すると、検査の前の文字列を保持したままにしたり、`checkSettings` が形の検査の前にキーの確かめを呼んだりしやすい。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/901
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 3 つの `main`、`make wasm-check`、依存の検査（設計書 §8 の 1 の残り）

**変更するファイル:** `cmd/yt2column-wasm/`・`cmd/yt2column-wasm-dev/`・`cmd/yt2column-extjob/`（新規）とそのテスト、`internal/extjob/jsmodule/`（新規。`js` のビルドだけで、テストはない）、`internal/godepstest/` とそのテスト、`internal/extjob/deps_test.go`（新規）、`cmd/yt2column/devdeps_test.go`、`cmd/yt2column/makefile_test.go`、`internal/config/envaccess_test.go`、`internal/config/extension.go`（空のワークスペース ID を、設計書 §3.2 と ADR-0003 のとおり未設定として答える）、`Makefile`、`CLAUDE.md`、`docs/dev/guides/development.md`、`docs/dev/security.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/project_overview.md`、ADR（§3.3）

-   [x] **ステップ 4-1**: `internal/extjob/jsmodule` と `cmd/yt2column-wasm` を作る（設計書 §1.4・§3.3、0090 設計書 §3.1.2）。`jsmodule` は、`extjob.Module` の 4 つの関数を `syscall/js` で 1 つの名前空間のオブジェクトに登録する（§1.6）。`cmd/yt2column-wasm` は、`extjob.NewModule(claudebackend.New())` を `jsmodule.Serve` に渡すだけを持つ。どちらも `js` のビルドだけでビルドする。
-   [x] **ステップ 4-2**: `cmd/yt2column-wasm-dev` を作る（設計書 §3.4）。シナリオの 5 つの値を `main` のパッケージの文字列の変数で持ち、起動のときに `ParseScenario` で解析して、合えば `extjob.NewModule(devbackend.New(scenario))` を `jsmodule.Serve` に渡して関数を登録し、合わなければ登録しない。`syscall/js` を使う登録のファイルだけを `js` のビルドにし、ネイティブのビルドでは、同じ変数を解析して結果だけを返す `main` を持つ（PR-5 のビルドのスクリプトが使う）。ネイティブのテストで、解析の受理と拒否を確かめる。
-   [x] **ステップ 4-3**: `cmd/yt2column-extjob` を作る（設計書 §3.4、0090 設計書 §7.2）。`-llm=claude` か `-llm=fake` を必ず指定させ、指定がないかほかの値なら何もせずに終わる。`-llm=fake` は 5 つの引数を `ParseScenario` で解析する。`-llm=claude` は `config.ReadExtensionSettings` で読んだ 2 つの値を `StoredSettings` にする。入力のファイル（4 MiB まで）を `Module.PrepareJSON` に、続けて `Module.GenerateJSON` に渡し、`GenerateResult` の JSON を出す。テストのために、`main` の処理は LLM の実装を作る関数を差し替えられる形にする。テストは、`-llm` の指定がないときと合わないシナリオで実装の関数を 1 度も呼ばないこと、`-llm=claude` で環境に置いた目印のキーが、成功と拒否のどちらの道でも標準出力と標準エラーに出ないこと、ワークスペース ID が未設定のときに空の文字列を渡すこと、合わない入力のファイルと 4 MiB を超えるファイルで拒否の理由を出すこと、結果の JSON を出すことを確かめる。
-   [x] **ステップ 4-4**: `make wasm-check` の対象を、`cmd/yt2column-wasm` と `cmd/yt2column-wasm-dev` のビルド（それぞれの依存を含む）にし、`vet` の対象を `internal/llm/llmhttp`・`internal/llm/claude`・`internal/extjob/jsmodule`・2 つの `main`・`internal/writer`・`internal/transcript` にする（設計書 §3.10）。PR-1〜PR-3 で `WASM_PACKAGES` に加えたパッケージは、2 つの `main` の依存に含まれるので、ビルドの対象からは除く。`cmd/yt2column/makefile_test.go` に、`make -n wasm-check` が 2 つの記事生成モジュールを `GOOS=js GOARCH=wasm` でビルドし、挙げたパッケージを `vet` することを確かめるテストを加える（今の `makeDryRun` の補助を使う）。CLAUDE.md の説明を直す。
-   [x] **ステップ 4-5**: 依存の検査を加える（設計書 §7.3）。
    -   `internal/godepstest` が `GOOS=js GOARCH=wasm` の依存を求められるようにし、`godepstest_test.go` に、`js` の対象で `js` のビルドだけのファイルの依存が出ることを確かめる行を加える。
    -   `internal/extjob/deps_test.go` に、公開用の記事生成モジュールの `js` の依存が `internal/extjob/claudebackend` を含み `internal/llm/devfake` と `internal/extjob/devbackend` を含まないこと、開発用の記事生成モジュールの `js` の依存が `internal/extjob/devbackend` を含み `internal/llm/claude` と `internal/extjob/claudebackend` を含まないことを確かめるテストを置く。
    -   `cmd/yt2column/devdeps_test.go` の許さない依存に `internal/extjob/devbackend` を加える。`cmd/yt2column-wasm-dev`・`cmd/yt2column-extjob` は `main` のパッケージで import できないので加えない。サーバとホストの `devdeps_test.go` は、撤去の PR まで今のまま残す。
-   [x] **ステップ 4-6**: `envaccess_test.go` の入口の一覧に `cmd/yt2column-extjob/main.go` を加え、その観測を確かめる。記事生成モジュールは環境を参照しない（0090 設計書 §3.11 のテストの表）。
-   [x] **ステップ 4-7**: 文書を書き換える。package_reference.md に 3 つの `main` と `internal/extjob/jsmodule` の行を、project_overview.md のディレクトリの構成に 3 つの `main` と `internal/extjob/jsmodule` を加える。development.md に `cmd/yt2column-extjob` の使い方（`-llm` の指定が要ること、入力のファイルをリポジトリに入れないこと）を、security.md の §7 に開発用のコマンドの入力のファイルをリポジトリに入れないことを書く（設計書 §3.13）。ADR の食い違いの記述を §3.3 のとおりに除くか書き換える。

**完了の条件:** `make test`・`make lint`（新しい `make wasm-check` を含む）・`make deadcode` が通る。ネイティブの `go build ./...`・`go vet ./...` が、`js` だけの `main` で失敗しない。`_checks/cli_unchanged.sh` が通り、§7 の AC-26 の記録に書いた。

### PR-4 作成ポイント: article generation module mains and dependency checks

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7

**推奨タイトル**: `feat(0091): 記事生成モジュールと開発用のコマンドの main、WASM の検査、依存の検査を加える`

**レビュー観点**: 依存の検査が `GOOS=js GOARCH=wasm` の依存を求め、含むべきものと含まないものの両方を確かめるか / 開発用のコマンドが `-llm` の指定なしで実装を呼ばず、キーを出力に出さないか / `make wasm-check` の対象と、それを固定するテストが合うか

**実装モデル要件**: opus

**判定理由**: ステップ 4-5 は、ADR-0045 の「公開する成果物に fake を入れない」を守る依存の検査で、パネルの条件のセキュリティの関門に当たる。Sonnet が実装すると、ネイティブの依存だけを見て WASM の依存を確かめない検査になっても、含まないことだけを確かめるテストが通ったまま気づきにくい。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/914
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 拡張のビルド（設計書 §8 の 2）

**変更するファイル:** `extension/scripts/`（ビルドのスクリプト、`check-dist.ts`、`has-extension-changes.sh`、依存のディレクトリを求めるスクリプト）、`extension/package.json`、`extension/static/buildKind.js`・`extension/src/buildKind.d.ts`（新規）、`extension/static/_locales/*/messages.json`、`extension/eslint.config.js`、`extension/.prettierignore`、`extension/test/`（`checkDist.test.ts`・`ciChanges.test.ts`・`repository.test.ts`、ビルドのスクリプトのテスト）、`Makefile`、`.github/workflows/ci.yml`、`.gitignore`、`CLAUDE.md`、`docs/dev/guides/development.md`、`README.md`・`README.en.md`、`cmd/yt2column/` の文書のテスト（新規）と `internal/doctestutil/guides.go`（その表の位置の値）、ADR（§3.3）

-   [x] **ステップ 5-1**: 拡張のビルドが Go を呼ぶようにする（設計書 §3.5、0090 設計書 §3.1.4）。ビルドのスクリプトは、`go.mod` の `go` の行の版を読んで `GOTOOLCHAIN` に与え（パッチの版がなければ理由を示して止める）、`GOOS=js GOARCH=wasm go build -trimpath` で記事生成モジュールをビルドし、同じツールチェーンの `wasm_exec.js` を成果物の直下に置く。`npm run build` は、Go を呼ぶ手順まで含めて、検査を通った成果物を作る。`go` を呼ぶ部分は差し替えられる形にし、node のテストで、版の読み取り（パッチの版がないときの停止を含む）と、`go` に渡す引数と環境を確かめる。
-   [x] **ステップ 5-2**: `extension/static/buildKind.js`（値は `release`）と `extension/src/buildKind.d.ts` を加える（設計書 §3.5）。`_locales` に、開発用の印を付けた名前のメッセージを加える。
-   [x] **ステップ 5-3**: 開発用のビルドを加える（設計書 §3.5・§6.4）。`make ext-build-dev` は 5 つの変数（既定の値は `Makefile` が持つ）をいつも渡す。ビルドのスクリプトは、`cmd/yt2column-wasm-dev` のネイティブのビルドで値を解析してから、`extension/dist-dev/` を空にし、`tsc` と `static/` を写し、開発用の manifest（`key` を除き、名前を開発用のメッセージに替える）を書き、`buildKind.js` を `dev` に置き換え、1 つの `-ldflags` の文字列に 5 つの `-X` を並べて開発用の記事生成モジュールをビルドする。node のテストで、合わないシナリオで `dist-dev/` に触れる前に止まること、`-ldflags` を 1 回だけ 5 つの `-X` で渡すこと、開発用の manifest の変換を確かめる。`.gitignore`・`eslint.config.js` の `ignores`・`extension/.prettierignore` に `dist-dev/` を加え、`repository.test.ts` の無視の規則の確かめに加える。`make ext-check` に `ext-build-dev` を加える（§1.3 の決定 11）。
-   [x] **ステップ 5-4**: `check-dist.ts` を、ビルドの種類を引数で受け取り、種類ごとの期待で検査するように改める（設計書 §3.5 の「成果物の検査」の各項目）。記事生成モジュールは Node で `wasm_exec.js` とともに読み込んで `buildInfo` を呼び、`mainPath`・`goVersion`・`llm`（`dev` ではシナリオの 5 つの値）を確かめる。`buildInfo` を読む部分は差し込めるようにし、`checkDist.test.ts` は Go を呼ばずに、種類と出力のディレクトリの組み合わせ、`buildKind.js` の値、開発用の manifest、WASM と `wasm_exec.js` の例外、`buildInfo` の各値の違いでの拒否を確かめる。
-   [x] **ステップ 5-5**: CI を改める（設計書 §3.10）。
    -   `check-changes` の手順に Go を用意する。記事生成モジュールの依存のディレクトリを `GOOS=js GOARCH=wasm go list -deps -f` で求める処理をスクリプトに置き、標準ライブラリを除き、`-e` を付けず、失敗したときは拡張の検査を走らせる側に倒す。
    -   `has-extension-changes.sh` は、変わったファイルの一覧に加えて依存のディレクトリの一覧を受け取り、末尾の `/` で比べる。`go.sum` を加える。`.md` の除外より前の、変わったファイルのすべての一覧で判定する。
    -   `ciChanges.test.ts` と、依存のディレクトリを求めるスクリプトのテスト（`go` を差し替えて走らせる）で、`.go`・`.tmpl`・fake の記事の `.md` の変更と削除、サブディレクトリ、`internal/llm` と `internal/llmx` の境界、求める処理の失敗を確かめる。
    -   `extension` の手順で、`ext-build` より前に Go を用意し、`ext-build` の後に `ext-build-dev` を走らせる。`repository.test.ts` の手順の順の確かめを直す。
-   [x] **ステップ 5-6**: 文書と ADR を書き換える。CLAUDE.md の「Extension Commands」に `make ext-build-dev` を加え、`make ext-build` が Go を呼ぶことと `make ext-check` の中身を直す（`TestExtensionDocumentation`）。development.md に、開発用のビルドの 5 つの変数と受理する綴りの表、見分け方（開発用の印、別の拡張の ID）、初めのビルドが `go.mod` の版のツールチェーンをダウンロードしうることを書き、開発用のビルドが切り替えの PR まで画面で使えないことを添える（設計書 §3.4）。README の拡張のビルドの説明に、ビルドが `go.mod` の版の Go も呼ぶこと（初めのビルドがその版をダウンロードしうること）を加え、`README.en.md` に写す。development.md の変数の表と綴りを、`devfake` の綴りと `Makefile` の変数と照合する文書のテストを `cmd/yt2column` に加える（開発用のサーバの表を確かめる `TestDevelopmentDocsDevServer` に当たるもの）。ADR の食い違いの記述を §3.3 のとおりに除くか書き換える。

**完了の条件:** `make test`・`make lint`・`make ext-check`（`ext-build-dev` を含む）が通る。`_checks/cli_unchanged.sh` が通り、§7 の AC-26 の記録に書いた。

### PR-5 作成ポイント: extension build with the WASM module and the development build

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6

**推奨タイトル**: `build(0091): 拡張のビルドに記事生成モジュールを入れ、開発用のビルドと CI の変更の検出を加える`

**レビュー観点**: 成果物の検査が WASM の出どころとツールチェーンの版を `buildInfo` で確かめるか / 開発用のビルドが合わないシナリオで出力に触れる前に止まり、`key` を持たないか / CI の変更の検出が依存のディレクトリの境界と失敗を正しく扱うか / 手順の順の変更で拡張の検査が Go なしで走らないか

**実装モデル要件**: opus

**判定理由**: CI と外部のツール（`go list`・`GOTOOLCHAIN`・Node での WASM の読み込み）の面が大きく、`mkplan.md` step 8 のパネルの条件（CI と外部の資源の面が大きい計画）に当たる。Sonnet が実装すると、`-e` による失敗の見落としや、`-ldflags` を 2 回渡して後の値だけが効くことを、検査が通ったまま残しやすい。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/925
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: API キーの経路の検査と、まだ使われない拡張の部品（その 1）（設計書 §8 の 4 の一部）

**変更するファイル:** `extension/test/apiKeyPath.test.ts`（新規）、`extension/test/secretsAbsent.test.ts`（削除）、`extension/src/browser/chromeDeps.ts`、`extension/eslint.config.js`、`extension/src/wasmWorker.ts`・`extension/src/wasmWorkerClient.ts`・`extension/src/core/workerCalls.ts`・`extension/src/core/jsonShape.ts`・`extension/src/core/wasmResult.ts`・`extension/src/core/jobRecord.ts`・`extension/src/core/viewerTitle.ts`・`extension/src/core/optionsView.ts`（新規）、`wasm_exec.js` の型の宣言（新規）、Worker の型を確かめる `tsconfig` の設定と、それを走らせる `package.json` の `typecheck` と `build`、`extension/src/core/i18n/*`、`extension/test/`（新しいテスト、`lintRules.test.ts`、`helpers/chrome.ts`、`helpers/worker.ts`（新規））、本計画

-   [x] **ステップ 6-1**（PR-6 の `/runplan` の前に、確認のセッションで行う。§1.3 の決定 3）: `make ext-build` が作った公開用の記事生成モジュールを、試作の拡張の Worker で読み込み、Chrome と Brave で、誤ったキーで `checkSettings` と `generate` を `anthropic-workspace-id` のありとなしの両方で呼び、`key_rejected` と `api_key_rejected` になることを確かめる。実 API の呼び出しなので、人の承認を得る。結果を §7 の「PR-6 の前の手動の確認の記録」に書き、ステップ 1-1 と同じ形で PR-6 のブランチに commit する。読めないときは、先へ進まず、ステップ 1-1 と同じく人と決め直す。
-   [x] **ステップ 6-2**: `secretsAbsent.test.ts` を `apiKeyPath.test.ts` に置き換える（設計書 §5.4 の「ソースの検査」、0090 設計書 §7.3）。この PR のほかのどのファイルよりも前に行う。
    -   先に `apiKeyPath.test.ts` を加える。`onChanged` を参照するのが `chromeDeps.ts` の `chrome.storage.session.onChanged` だけであること、`chrome.storage.local.get` と `chrome.storage.local.remove` を呼ぶのは決まった関数だけで、その引数が固定の 2 つの名前だけであること、`chrome.storage.local.clear` と `chrome.storage.sync` を参照しないこと、ログの関数に API キーを渡さないこと、古い版の値の名前（`sharedToken`・`serverPort`）を参照しないこと（要件書 AC-32）を確かめる。今の `secretsAbsent.test.ts` の確かめのうち、Webhook の URL と、トークンとポートの設定がないことの確かめも引き継ぐ。
    -   次に、`secretsAbsent.test.ts` を削除だけの commit で除き、`make ext-coverage-compare` で比べる。
-   [x] **ステップ 6-3**: `chromeDeps.ts` に、設計書 §5.4 の表の決まった関数を加える。`StorageArea` も `chrome.storage.local` のオブジェクトも外へ渡さない。`extension/test/helpers/chrome.ts` に、その fake と失敗の注入を加え、`fakes.test.ts` で fake を確かめる。lint の規則が今は `chromeDeps.ts` の `storage` も拒否するので、このステップはステップ 6-4 の `storage` の規則の変更と同じ commit にする。
-   [x] **ステップ 6-4**: lint の規則を設計書 §5.4 のとおりに改める（`storage` は `chromeDeps.ts` のほかで拒否、`session` は `chrome.session` だけに狭める、決まった関数を import してよいファイルを限る、型だけの import は許す、`fetch` は `wasmWorker.ts` のほかで拒否する）。`sendNativeMessage` の規則は撤去の PR まで残す。`lintRules.test.ts` に、各規則の拒否と許可の行と、例外を持つファイル（`chromeDeps.ts`・`wasmWorker.ts`）にもほかの規則が当たることの行を加える（今の `lintRules.test.ts:457` の行は `chrome.storage.local.get` を例にしているので、例を替える）。HTML を解釈する API の禁止が新しいファイルにも当たることを確かめる（0090 設計書 §7.3）。
-   [x] **ステップ 6-5**: `wasmWorker.ts`・`wasmWorkerClient.ts`・`wasm_exec.js` の型の宣言を作る（設計書 §3.8.4、0090 設計書 §3.3.1・§3.4）。Worker の側の規則（呼び出しごとに 1 回だけ答える、Go のプログラムの終わりを見張る、`try`/`catch` と戻り値の種類の確かめ）と、ページとの間の `WorkerCall`・`WorkerMessage` の型と検査は `extension/src/core/workerCalls.ts` に置いて単体テストで確かめ、`wasmWorker.ts` は Worker の大域の値をつなぐだけにする。Worker は、記事生成モジュールが名前空間を置いた時点で 1 度だけ `{ kind: "ready" }` を送り、ページの側の読み込みの期限（30 秒）は、Worker を作ってからこの知らせを受け取るまでとする（設計書 §3.6 の読み込みの期限を、その後の `checkSettings` の時間と分けるため）。呼び出しを待たない間に起きた失敗も、その後の呼び出しの結果にする。WebWorker の `lib` で `wasmWorker.ts` の型を確かめる `tsconfig` の設定を作り、`typecheck` と `build` の両方で使い、成果物に `wasmWorker.js` が入ることを `check-dist` で確かめる。テストは `extension/test/helpers/worker.ts` の Worker の fake で、30 秒の読み込みの期限、`callId` での応答の対応、`exit` を `runtime_exit` に、`error` のイベントを `worker_error` に、Promise の拒否を `panic` にすること、呼び出しごとに最初の失敗だけを使い、Worker を止め、その後の知らせを無視することを確かめる。
-   [x] **ステップ 6-6**: `wasmResult.ts` を作る（設計書 §3.12、H-07）。Worker から受け取る JSON の文字列を `strictJson.ts` で解析する。閉じた形の JSON を読む手順（メンバの集合、型、大きさの上限）は、ステップ 6-7 の `jobRecord.ts` と共有するので `extension/src/core/jsonShape.ts` に置く。テストは、閉じたメンバの集合、重複したメンバ、知らないメンバ、型の違い、状態ごとのメンバ、512 KiB の境界で、合わないものが `result_shape` になることを確かめる。`UsdAmount` などの型の印を作るときは、`eslint.config.js` の `creatorConfig` にこのファイルを加える。
-   [x] **ステップ 6-7**: `jobRecord.ts` を作る（0090 設計書 §3.2.1・§3.3、H-07）。ジョブの記録と開始の文脈の型と、読み戻すときの検査を持つ（設計書 §3.12）。テストは、`version`、閉じたメンバの集合、状態ごとのメンバ、ジョブ ID の桁数、512 KiB の境界で、合わない記録をないものとして扱うことと、記録の型が字幕と概要欄を持たないことを確かめる。
-   [x] **ステップ 6-8**: `viewerTitle.ts` を、ステップ 1-1 で決めた案で作る（設計書 §3.8.2・§6.3）。状態、表示の値、ビルドの種類、表示の言語（案 B では URL のタイトル）から文字列を返すだけの関数で、テストは設計書 §3.8.2 の表の各行と、どの入力でも空の文字列を返さないことを確かめる。
-   [x] **ステップ 6-9**: `optionsView.ts` を作る（設計書 §3.6 の表、§4.1 の `OptionsOutcome`）。結果ごとに、示す固定の文言と、欄を空にするかと、合わない欄（`field_rejected` のとき）を返す。設定されているかの表示は、設計書 §3.6 のとおり保存か削除の後に結果によらず読み直すので、結果ごとには返さない。テストは `OptionsOutcome` のすべての値の行を確かめる。この PR の関数が使う文言だけを、英語と日本語の両方に加える。

**完了の条件:** `make ext-check` が通る。`secretsAbsent.test.ts` の削除の commit に `make ext-coverage-compare` の通った出力がある。`_checks/cli_unchanged.sh` が通り、§7 の AC-26 の記録に書いた。§7 に PR-6 の前の手動の確認の記録がある。

### PR-6 作成ポイント: key path checks, storage entry points, and the first unused extension parts

**対象ステップ**: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 6-9

**推奨タイトル**: `feat(0091): API キーの経路の検査と chrome.storage の決まった関数、Worker の呼び出し、結果と記録の検査を加える`

**レビュー観点**: API キーの経路の検査と lint が、`get(null)` のような全部を読む呼び出しと決まった関数の外の `storage` の参照を拒否し、今の禁止を落としていないか / `secretsAbsent.test.ts` の確かめで引き継ぐものが漏れていないか / Go の実行の終わりと Worker の失敗を、呼び出しごとに 1 度だけ失敗にするか / 記録と結果の検査が補正せず、合わないものを捨てるか

**実装モデル要件**: opus

**判定理由**: ステップ 6-2・6-4 は API キーの経路を守るソースの検査と lint の規則を作り直すもので、パネルの条件のセキュリティの関門に当たる。Sonnet が実装すると、`no-restricted-syntax` の一覧の組み直しで今の禁止を落としたり、検査が名前を書かない読み出しを見逃したりしやすい。

**自動送信**: hold

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/946
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 7: まだ使われない拡張の部品（その 2）（設計書 §8 の 4 の一部）

**変更するファイル:** `extension/src/core/jobViewerUrl.ts`・`extension/src/jobViewerQuery.ts`・`extension/src/core/jobMessages.ts`・`extension/src/optionsFlow.ts`・`extension/src/viewerRunner.ts`・`extension/src/jobCoordinator.ts`（新規）、`extension/test/`（新しいテスト）。`extension/eslint.config.js` は変えない（新しいモジュールは決まった関数を型でだけ受け取り、値では import しないため。入口のページのスクリプトが import する PR-8 で許可の一覧を扱う）

-   [x] **ステップ 7-1**: ビューアの URL と問いと、ページの間のメッセージを、新しいファイルに作る（§1.3 の決定 5）。`jobViewerUrl.ts` は 0090 設計書 §5.2 の `r`・`k`・`u`・`e`（案 B では設計書 §3.8.2 の `t`）の組み立てと解析を、`jobViewerQuery.ts` はジョブ ID で同じジョブのビューアを探す問い（今の `viewerQuery.ts` の規則）を、`jobMessages.ts` は 0090 設計書 §3.3.1 の型と検査と、生成の開始の入力の 4 MiB の確かめを持つ。テストは、名前の閉じた集合と重複の拒否、`u` の上限、案 B では `t` の 64 コードポイントの切り方（サロゲートの組を分けない）と検査、メッセージの閉じたメンバの集合、4 MiB の境界を確かめる。
-   [x] **ステップ 7-2**: `optionsFlow.ts` にオプションページの保存と削除の流れを作る（設計書 §3.6・§6.1）。テストは Worker と決まった関数の fake で、押した時点の値の写しを検査して書くこと、検査と書き込みの間の欄と操作の無効化、90 秒の全体の期限（読み込み 30 秒を含み、書き込みに及ばない）、`set` と `remove` をそれぞれ 1 回だけ呼ぶこと、空のワークスペース ID を空の文字列で書くこと、保存しないときに書き込みを呼ばないこと、書き込みの失敗と削除の失敗、結果によらず `localKeys` で表示を読み直すこと、ページを開いたときの `buildInfo` の表示と読めないときの表示を確かめる。
-   [x] **ステップ 7-3**: `viewerRunner.ts` に実行するビューアの流れを作る（0090 設計書 §3.4・§6.2・§6.3、設計書 §2.4・§3.8.1）。`wasmWorkerClient.ts` の読み込みの期限は Worker の `ready` で終わるので、0090 設計書 §3.4 の「読み込みと `prepare` に 30 秒」を保つには、`prepare` の呼び出しに残りの期限を `timeoutMs` で渡し、その `timeout` を `worker_load` にする。テストは Worker と決まった関数の fake で、予約の後に 2 つの項目だけを読むこと、項目がないときだけ `generate` を呼ばずに `api_key_missing` にすること（空の文字列と `null` ではならないこと。H-06）、読む呼び出しの失敗が `settings_read` になること、読んだ値を検査せずに渡し持ち続けないこと、進み具合と結果を記録が結果を持つまで送り直すこと、50 分の期限で止めて `run_timeout` にすること、実行の間だけ読み込み直しの確認を出すこと、`runner` が替わったら Worker を止めることを確かめる。
-   [x] **ステップ 7-4**: `jobCoordinator.ts` に新しい SW の調停を作る（0090 設計書 §3.2・§3.3・§5.2・§6.1〜§6.3、設計書 §3.9）。`chrome.*` に依存しない部分は、決まった関数を引数で受け取る関数にする。案 B では、ビューアを開くときに `t` を作る（設計書 §3.8.2）。`r` を `k` に書き換えるときに `t` を残すのはビューアのページ（PR-8 の配線）で、本 PR の `jobViewerUrl.ts` の `jobViewerQuery` は job の目標でも `t` を持てるところまでとする。テストは fake で、0090 設計書 §7.1 の「SW の調停」の項目、送り手の確かめ（要件書 AC-24）、同じ入力と同じ動画の照会（AC-10）、同時に 2 つまでの上限（AC-13）、記録 50 件・合計 8 MiB・記事 256 KiB の上限と捨てる順（AC-09）、SW が API キーの値を読まず `localKeys` だけを使うこと（AC-20）、結果不明と、入力を受け取る前に SW が止まった場合と、完成したが保持できなかった場合（H-06）を確かめる。

**完了の条件:** `make ext-check` が通る。`_checks/cli_unchanged.sh` が通り、§7 の AC-26 の記録に書いた。

### PR-7 作成ポイント: viewer URL and messages, options flow, runner, and coordinator

**対象ステップ**: 7-1 / 7-2 / 7-3 / 7-4

**推奨タイトル**: `feat(0091): ビューアの URL とメッセージ、オプションページの流れ、実行するビューアと新しい SW の調停を加える`

**レビュー観点**: 調停が直列に処理し、`runner` と一致しない知らせで記録を変えず、同時の 2 つの開始で記録を二重に加えないか / 実行するビューアが保存した設定を検査せずに渡し、値を持ち続けないか / 保存と削除がそれぞれ 1 回の書き込みで、期限が書き込みに及ばないか / URL とメッセージの検査が補正しないか

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 7-4 は直列の処理と止まった実行の判定を持つ状態機械で、7-2・7-3 も期限と送り直しを持つ流れであり、孤立した高いリスクのステップ（並行の処理と状態機械）に当たる。Sonnet が実装すると、同時の開始の扱いや、止まった記録が同じ `runner` の結果で完成に戻る道を落としやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 8: 切り替え（設計書 §8 の 4）

**変更するファイル:** `extension/static/`（`manifest.json`、`options.html`（新規）、`popup.html`、`viewer.html`、`_locales`、オプションページのスタイルシート）、`extension/src/`（入口、`options.ts`（新規）、表示）、`extension/scripts/check-dist.ts`・`popup-preview.ts`、`extension/test/`、サーバとホストの文書のガード（§1.3 の決定 2）、`internal/doctestutil/`、`internal/server/test_helpers.go`、`internal/config/test_helpers.go`、`cmd/yt2column/docs_test.go`、`cmd/yt2column-nmhost/`（README の移行の手順のテスト）、README と `README.en.md`、`docs/dev/`（security.md・development.md・project_overview.md・production_profile.md・cli.md・package_reference.md）、`docs/translation_glossary.md`、`CLAUDE.md`、ADR（§3.3）、本計画

-   [ ] **ステップ 8-1**: manifest を 0090 設計書 §3.7 の値にする（権限 `activeTab`・`scripting`・`storage`、CSP、`incognito`、`minimum_chrome_version` `"142"`、オプションページをタブで開く、今の `key`）。`manifest.test.ts` を直し、`check-dist.ts` がオプションページの参照も確かめるようにする。`_locales` の `extDescription` を拡張だけの経路の説明に改める（設計書 §3.5）。
-   [ ] **ステップ 8-2**: 入口を新しい部品に配線する。
    -   `background.ts` は `jobCoordinator.ts` を登録する。
    -   `popup.ts`・`launch.ts` は、`setLocalAccessLevel` が成功したときだけ関数を注入し、生成の開始を SW に送る（0090 設計書 §3.5・§3.6、要件書 AC-23）。`launch.test.ts` に、`setLocalAccessLevel` が失敗したときに注入せず、開始が拒否された状態を示す行を加える。
    -   `viewer.ts`・`viewerApp.ts` は、実行するビューア（`viewerRunner.ts`）と表示するビューア（`chrome.storage.session.onChanged`）を持つ。
    -   `options.html`・`options.ts` を作る（文字を伏せた欄、`buildInfo` の表示。設計書 §3.6）。オプションページの HTML のテスト（`optionsHtml.test.ts`）を加え、`style.test.ts` の確かめるスタイルシートの一覧にオプションページのものを加える。
    -   `popup.html` に、詳細の節の後のリンクと、設定が要る状態の操作を置く（設計書 §3.7）。`viewer.html` の静的なタイトルを空でない値にし、`viewerTitle.ts` を使う（設計書 §3.8.2）。開発用の印を示す（設計書 §3.5）。
-   [ ] **ステップ 8-3**: 表示のファイル（`jobView.ts`・`popupView.ts`・`sendMessages.ts`・`types.ts`・`i18n/*`、`renderJob.ts`・`renderViewer.ts`・`render.ts`）を、0090 設計書 §3.8 の状態と設計書 §4.2 の表に合わせ、文言を英語と日本語の両方に置く（要件書 AC-14・AC-15）。直したファイルを使うテスト（`popupHtml`・`popupView`・`viewerApp`・`launch`・`renderJob`・`renderViewer`・`render`・`sendMessages`・`chromeDeps`・`fakes`・`popupPreview`・`entryPoints`・`jobCodes`・`checkDist` の各テスト）と `scripts/popup-preview.ts` を直す。直したファイルの型が変わり、呼ばれなくなった拡張のコードが型の検査を通らなくなるときは、そのファイルとテストをこの PR で除く（§1.3 の決定 10。削除だけの commit と `make ext-coverage-compare`）。除くファイルに `TestNativeHostContract` が読むもの（`send.ts`・`core/native.ts`・`core/exchange.ts`）があるときは、同じ削除の commit で、そのテストとその定数も除き、`make coverage-compare` でも比べる。
-   [ ] **ステップ 8-4**: サーバとホストの文書のガードを除く（§1.3 の決定 2）。`TestServerDocsContract`・`TestReadmeHostExitCodes`・`TestDevelopmentDocsDevServer`・`serverReasons.test.ts` と、それだけが使う補助（`internal/doctestutil` のサーバとホストの位置の値、`internal/server/test_helpers.go` の `RejectReasonsForTest`・`APICodesForTest` など。`make deadcode` が確かめる）を除く。除く前に、Go の削除を仮の commit で `make coverage-compare` にかけ、残るパッケージの関数のカバレッジが下がるなら、その関数のテストを先に足す（§1.2）。削除は削除だけの commit にし、`make coverage-compare` と `make ext-coverage-compare` で比べる。
-   [ ] **ステップ 8-5**: 仕様の文書を書き換える（要件書 AC-33・AC-34、設計書 §3.13、0090 設計書 §3.14.2）。
    -   README を先に書き換え、`/mktrans` で `README.en.md` に移す。要件書 AC-34 の内容を書き、サーバとホストの節を除く。`## 設定` の変数の表は CLI のために残し、サーバの設定ファイルに触れる段落を直す。`## ブラウザ拡張` の語（`metadataDocs.test.ts`）、「Chrome 142 以降」（`manifest.test.ts`）、拡張の ID（`TestExtensionDocumentation`）、リンク（`TestReadmeLinks`）を合わせる。
    -   README の移行の手順が示すブラウザごとのホストの manifest の場所を、`internal/nmhost` の manifest の場所の値と照合するテストを、`cmd/yt2column-nmhost` に加える（撤去の PR でホストとともに除く）。
    -   security.md・development.md・project_overview.md・production_profile.md・cli.md・用語集を、0090 設計書 §3.14.2 と設計書 §3.13 の要点のとおりに書き換える。security.md の `### 8.1. ブラウザ拡張（#110）` の見出しと `**要求する権限**` の一覧は、`TestExtensionDocumentation` が読む。
    -   CLAUDE.md は、「Architecture Overview」の主経路、「Accessibility」の対象にオプションページを含めること、「Development Notes」の `TestReadmeHostExitCodes`・`serverReasons.test.ts` とサーバの設定ファイルの記述、「Go Idioms」の `TestServerDocsContract` の見出しの例を直す。「Build Commands」の 2 つのバイナリと文書のガードのコマンドは、撤去の PR まで、撤去の予定のものとして残す（§1.3 の決定 2）。
    -   README の `### 推奨構成` と production_profile.md の `## 本番の構成` の塊が、サーバの設定ファイルの形でなくなるので、`TestProductionProfileDocsContract` の照合を CLI の読み込み（環境変数の名前を引く関数を `config.Load` に渡す形）に改め、使われなくなる `ParseServerFileForTest` を除く。
-   [ ] **ステップ 8-6**: ADR の食い違いの記述を §3.3 のとおりに除くか書き換える。案 B で実装したときは、人と決めたうえで、ADR-0010 の「ビューアの URL」の決定を改め、要件書 §3.2 の「ビューアの URL」の行の扱いを PR の説明に書く（設計書 §3.8.2・§3.14）。
-   [ ] **ステップ 8-7**: 手動の確認を行い、§7 の「PR-8 の手動の確認の記録」に書く（要件書 §5、設計書 §7.2）。
    -   開発用のビルドで、要件書 AC-29 の状態と、AC-07・AC-08・AC-09・AC-11・AC-16・AC-20・AC-23 の手動の確認の項目を確かめる。
    -   オプションページで、VoiceOver の読み上げ（H-12。読み上げるときは欄の説明に注意を添える）と、同期を有効にしたプロファイルでパスワードの保存を勧められないこと（AC-03）を確かめる。
    -   ポップアップ、ビューア、オプションページで、CLAUDE.md「Accessibility」の各項目を、キーボードと明るい・暗いテーマで確かめる（AC-18）。
    -   本番の構成で、Chrome と Brave で 1 本ずつ、ビューアのタブを背面に置いたまま記事を完成させ、キーの確かめに料金がかからないことを Console の使用量で確かめる（AC-01）。実 API の呼び出しなので、人の承認を得る。

**完了の条件:** `make test`・`make lint`・`make deadcode`・`make ext-check` が通る。削除の commit に、カバレッジの比較の通った出力がある。`_checks/cli_unchanged.sh` が通り、§7 の AC-26 の記録に書いた。§7 に PR-8 の手動の確認の記録がある。サーバとホストを主経路として述べる文書が残らない。

### PR-8 作成ポイント: switch the extension to the extension-only path

**対象ステップ**: 8-1 / 8-2 / 8-3 / 8-4 / 8-5 / 8-6 / 8-7

**推奨タイトル**: `feat(0091): 拡張を拡張だけの経路に切り替え、サーバとホストを述べる文書とそのガードを除く`

**レビュー観点**: manifest と CSP が 0090 設計書 §3.7 の値で、`setLocalAccessLevel` の失敗で注入しないか / 各状態の表示と文言が設計書 §4.2 の表と要件書 AC-14・AC-15 に合うか / 除いた文書のガードと直した文書のガード、削除の commit のカバレッジの比較の出力 / README の移行の手順が要件書 AC-34 の 3 つを含み、§3.3 の ADR の記述を除いたか

**実装モデル要件**: opus

**判定理由**: 拡張の権限と保存と送信の経路を一度に切り替える移行の段で、パネルの条件（セキュリティの関門と移行、多くのテストの更新）に当たる。Sonnet が実装すると、`setLocalAccessLevel` の失敗のときにも関数を注入したり、文書の書き換えで固定された見出しやリンクを崩したりしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 9: 撤去と文書（設計書 §8 の 5）

**変更するファイル:** 撤去するパッケージ（要件書 AC-31）、呼ばれなくなった拡張のコードとそのテスト（§1.3 の決定 10）、`extension/eslint.config.js`、`extension/scripts/has-extension-changes.sh`・`extension/test/ciChanges.test.ts`、`internal/config/`（`!js` のファイル、そのテストと補助、`LoadServer` を使う行）、`internal/llm/provider/provider_test.go`、`cmd/yt2column/`（`_test.go` だけ）、`internal/llm/devfake/`（`deps_test.go`、`articles/elements.md`）、`internal/godepstest/godepstest_test.go`、`internal/pipeline/`（`relay_guard_test.go`、必要なら `pipeline_test.go` の `testOnlyPackageDirs`）、`internal/config/envaccess_test.go`、`Makefile`、`.golangci.yml`、`.github/workflows/ci.yml`、文書のガードのコマンドの写し、`CLAUDE.md`、`docs/dev/`、ADR（§3.3）、`docs/tasks/0091_extension_only_migration/_checks/removal.sh`（新規）

-   [ ] **ステップ 9-1**: サーバの設定の読み込みを使う行を直す。`TestLoadClaudeEffortDefault` と `TestLoadHTTP2Debug` のサーバの行は、同じ値の CLI の行があるので、削除だけの commit で除き、`make coverage-compare` で比べる。`TestNewSendsStageEffort` は CLI の読み込みに改める。
-   [ ] **ステップ 9-2**: 要件書 AC-31 のパッケージ、`internal/config` の `!js` のファイルとそのテストと補助、呼ばれなくなった拡張のコードとそのテスト、`TestNativeHostContract` とその定数を除く。除く前に、ステップ 8-4 と同じく仮の commit で比べ、残るパッケージ（`internal/job`・`internal/redact`・`internal/signalrun`・`internal/cachelock`・`internal/privatefile`・`internal/publisher`・`internal/writer`・`internal/config` など）の関数のカバレッジが下がるなら、そのテストを先に足す。削除の commit には、コンパイルに要る次の直しも入れる: `devfake/deps_test.go` の一覧（`internal/privatefile` は残す）、`godepstest_test.go` の例、`cmd/yt2column/devdeps_test.go` の許さない依存、`relay_guard_test.go` のホストの 2 つのテスト、`envaccess_test.go` の入口の一覧と `:407-411` の観測、比べるテスト（§1.3 の決定 7）、`package_reference.md` の撤去したパッケージの行と `internal/strictjson` の利用者の一覧（§1.2）。どのパッケージからも import されなくなるテストの補助があれば、`go list -deps -test` で確かめてから除き、`testOnlyPackageDirs` を直す。削除の commit を `make coverage-compare` と `make ext-coverage-compare` で比べる。
-   [ ] **ステップ 9-3**: `make build` を `yt2column` だけにし、`build-dev` とその変数を除く。`makefile_test.go` を直す。`.golangci.yml` の撤去したパッケージを指す規則を、`ci.yml` の `doc-guard` の対象から `./cmd/yt2column-server` を、`has-extension-changes.sh` と `ciChanges.test.ts` から `internal/server/response.go` を除く（設計書 §3.10・§3.11、§1.3 の決定 6）。lint の `sendNativeMessage` の規則は、`chromeDeps.ts` に許していた例外だけを除き、禁止は残す（設計書 §5.4）。`lintRules.test.ts` の行を直す。
-   [ ] **ステップ 9-4**: 文書を書き換える（設計書 §3.13）。文書のガードのコマンドの 4 つの写し（CLAUDE.md「Development Notes」、`.claude/commands/_lib/task-pr-handoff.md`、`.claude/commands/_lib/simple-process.md`、`docs/dev/guides/fixprguard_guide.md`）から `./cmd/yt2column-server` を、それぞれの今の形のまま除く。CLAUDE.md の「Build Commands」と「Architecture Overview」に残る撤去したバイナリとパッケージ、package_reference.md の `internal/privatefile` の規則のリンク先、project_overview.md のディレクトリの構成、development.md の `make` の目標、security.md に残る撤去したコードの記述を直す。`devfake/articles/elements.md` の `make build-dev` の例を、残る目標の例に替える。
-   [ ] **ステップ 9-5**: `_checks/removal.sh` を作る。撤去したパッケージのディレクトリがないこと、`internal/job`・`internal/privatefile`・`internal/cachelock`・`internal/signalrun`・`internal/redact` が残ること、`internal/config` の Go のファイルに `!js` のビルドタグがないこと、`GOOS=js GOARCH=wasm go list -deps ./internal/config` が `internal/privatefile` を含まないこと、`make -n build-dev` が失敗すること、`Makefile`・`.golangci.yml`・`ci.yml`・`has-extension-changes.sh` が撤去したパッケージを指さないこと、追跡している文書（README、CLAUDE.md、AGENTS.md、`docs/dev/` の ADR を除く文書、`.claude/commands/`）のコマンドの例のうち、`go` のコマンドが挙げるパッケージが `go list` で見つかり、`make` のコマンドの目標が `make -n` で通ることを確かめる。各検査が、少なくとも 1 つの壊し方で失敗することを確かめる。出力とコミット ID を §7 に書く。
-   [ ] **ステップ 9-6**: ADR の食い違いの記述を §3.3 のとおりに除き、§1.3 の決定 9 の 2 か所を直す。

**完了の条件:** `make test`・`make lint`・`make deadcode`・`make ext-check` が通る。`_checks/removal.sh` が通る。削除の commit にカバレッジの比較の通った出力がある。`_checks/cli_unchanged.sh` が通り、§7 の AC-26 の記録に書いた。有効な ADR に #773 の食い違いの記述が残らない（§1.2 の手順で `HEAD` を確かめる。廃止した ADR-0005・ADR-0018 の「廃止の理由」の行は数えない）。

### PR-9 作成ポイント: remove the server, the host, the development server, and the unused extension code

**対象ステップ**: 9-1 / 9-2 / 9-3 / 9-4 / 9-5 / 9-6

**推奨タイトル**: `refactor(0091): localhost サーバ、Native Messaging のホスト、開発用のサーバと、呼ばれなくなった拡張のコードを撤去する`

**レビュー観点**: 残すパッケージ（`internal/privatefile`・`internal/udstest` など）を誤って除いていないか / 削除の commit のカバレッジの比較と、先に足したテスト / 撤去したパッケージを指す設定と文書のコマンドが残らないか / ADR に #773 の食い違いの記述が残らないか

**実装モデル要件**: opus

**判定理由**: Go と拡張の多くのパッケージ、テスト、設定、文書が結びついた撤去で、パネルの条件（移行と多くのテストの更新）に当たる。Sonnet が実装すると、コンパイルに要る直しを削除の commit から外してカバレッジの比較を止めたり、写しの 1 つにコマンドを残したりしやすい。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 状態の変化 |
|---|---|---|
| M0（PR-1 の前） | 401 とタイトルの手動の確認の記録 | Go の部品の前提が確かめられる |
| M1（PR-1〜PR-4） | LLM の層、設定の読み込み、`internal/extjob`、3 つの `main`、依存の検査 | 記事生成モジュールが WASM でビルドされる（まだ使われない） |
| M2（PR-5） | 拡張のビルドと開発用のビルド、CI | 成果物に記事生成モジュールが入る（まだ使われない） |
| M3（PR-6・PR-7） | API キーの経路の検査、まだ使われない拡張の部品 | 部品が単体テストで固まる |
| M4（PR-8） | 切り替え、仕様の文書 | 拡張だけの経路で記事ができる。サーバとホストは使われない |
| M5（PR-9） | 撤去 | サーバ、ホスト、開発用のサーバ、呼ばれなくなった拡張のコードがなくなる |

順序は設計書 §8 の依存の順のとおりである。§8 の 4（切り替え）を、まだ使われない部品（PR-6・PR-7）と切り替え（PR-8）に分けた理由は §1.3 の決定 1 に、§8 の 3（手動の確認）を PR-1 と PR-6 の前に置いた理由は決定 3 に、§8 の挙げ方と違う区切りは決定 6 にある。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 | 手動の確認、`ErrAuthRejected`、`Status`、`CheckKey`、`js` のビルドの指定 | sonnet+opus-review | hold |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 | `internal/config` の拡張用の読み込みと `!js`、fake のシナリオ | sonnet | auto |
| PR-3 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 | `internal/extjob`、`claudebackend`、`devbackend`、JSON の境界 | sonnet+opus-review | auto |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 | 3 つの `main`、`make wasm-check`、依存の検査 | opus | auto |
| PR-5 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 | 拡張のビルド、開発用のビルド、成果物の検査、CI | opus | auto |
| PR-6 | 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 6-9 | 手動の確認、API キーの経路の検査、`chrome.storage` の決まった関数と lint、Worker、結果と記録の検査、タイトルとオプションページの表示 | opus | hold |
| PR-7 | 7-1 / 7-2 / 7-3 / 7-4 | ビューアの URL とメッセージ、オプションページの流れ、実行するビューア、SW の調停 | sonnet+opus-review | auto |
| PR-8 | 8-1 / 8-2 / 8-3 / 8-4 / 8-5 / 8-6 / 8-7 | 切り替え、サーバの文書のガードの撤去、仕様の文書、手動の確認 | opus | auto |
| PR-9 | 9-1 / 9-2 / 9-3 / 9-4 / 9-5 / 9-6 | サーバ、ホスト、開発用のサーバ、呼ばれなくなった拡張のコードの撤去、残りの文書 | opus | auto |

PR-1 と PR-6 を `hold` にするのは、ステップ 1-1 と 6-1 を、前の PR のマージの後、その PR の `/runplan` の前に、確認のセッションで行うためである（§1.3 の決定 3）。PR-8 の手動の確認（ステップ 8-7）は `/runplan` の中で人に頼むので、`auto` のままにする。

### 3.3. ADR の食い違いの記述を除く PR

issue #773 にリンクする食い違いの記述を、どの PR で除くかの見込みを示す。コミット `bac09b3f` で `git grep -n 'issues/773' docs/dev/adr/` を走らせると 27 のファイルが出る。そのうち、有効な 25 本の ADR の「決定」の記述（ADR-0045 は 2 か所）が対象で、廃止した ADR-0005・ADR-0018 の「廃止の理由」の行と、ADR-0048 の「結果」の行（`:48`）は食い違いの記述でない（§1.3 の決定 9）。「書き換える PR」は、記述の一部を解いて、残る食い違いを述べる形に書き換える PR である。各 PR は、表と違っても、変えたコードと文書で記述が事実と合わなくなったときは、同じ規則で書き換えるか除く（§1.2）。

| ADR | 書き換える PR | 除く PR |
|---|---|---|
| ADR-0004・0006・0008・0009・0010・0017・0022・0027・0028・0029・0030・0031・0033 | ― | PR-8 |
| ADR-0007・0019・0020・0034・0038 | ― | PR-4 |
| ADR-0003・0043 | PR-4（記事生成モジュールの側はコードに入った） | PR-8 |
| ADR-0026 | ― | PR-5 |
| ADR-0049 | PR-4・PR-5 | PR-8 |
| ADR-0024 | PR-4（記事生成モジュールと開発用のコマンドを加えた） | PR-9 |
| ADR-0045（`:27`） | PR-4・PR-5 | PR-9 |
| ADR-0048（`:28`） | PR-8（コードの撤去だけが残る） | PR-9 |

## 4. テスト戦略 (Test Strategy)

-   **カバレッジの目標:** 新しい Go のパッケージと拡張の新しいモジュールは、公開する関数と分岐の各道を単体テストで通す。撤去の PR の後も、残るパッケージの関数の文のカバレッジを下げない（`make coverage-compare` が確かめる）。
-   **Go の単体テスト:** 設計書 §7.1 と 0090 設計書 §7.1 の各項目を、§2 の各ステップのテストが受け持つ。LLM は fake の `LLMClient`（`internal/llm/testutil`）か `devfake` で置き換え、HTTP は `llmhttptest` とループバックの送り先の差し替えで確かめる。実 API を呼ばない。
-   **今のサーバと同じ値:** PR-3 で写したテストと比べるテスト（要件書 AC-19）が確かめる。写したテストは、撤去の PR の後も `internal/extjob` で同じ値を確かめる（§1.3 の決定 7）。
-   **`js` のビルドだけの部分:** 送信の指定とヘッダは、共通の関数を通ることのネイティブのテストと、`js` のファイルを読むテスト（ステップ 1-5）で確かめる。`syscall/js` の登録は、`make wasm-check` のビルドと `vet`、PR-5 の成果物の検査（Node で `buildInfo` を呼ぶ）、確認のセッションと PR-8 の手動の確認で確かめる。Go の手順は Node.js を呼ばない（ADR-0026）。
-   **TypeScript の単体テスト:** `chromeDeps.ts` の決まった関数と Worker の fake に失敗を注入し、開発用のビルドで出せない状態（要件書 AC-29、H-06）を確かめる。fake は `extension/test/helpers/chrome.ts` と `extension/test/helpers/worker.ts` に置き、`fakes.test.ts` で fake 自体を確かめる。
-   **境界の値:** §1.2 の上限と期限は、境界の前後をテストする。
-   **セキュリティのテスト:** 依存の検査（PR-4）、成果物の検査（PR-5）、API キーの経路のソースの検査と lint（PR-6）、manifest（PR-8）が、設計書 §7.3 と 0090 設計書 §7.3 を受け持つ。
-   **テストの補助:** Go の新しいパッケージの中だけで使う補助は、そのパッケージの `test_helpers.go`（`//go:build test`）に置く（test_organization.md の Classification B）。ほかのパッケージから使う fake は作らない。`testutil/` のファイルを加えたときは、`TestFakesCarryBuildTag` の数を直す。
-   **文書のガード:** 各文書の値と構造は、今のガード（`TestPackageReferenceListsPackages`・`TestPackageReferenceStrictjsonUsers`・`TestExtensionDocumentation`・`TestReadmeLinks`・`TestReadmeSyncMarker`・`TestReadmeVersionsShareStructure`・`TestLLMDefaultsDocsContract`・`TestEffortValuesDocsContract`・`metadataDocs.test.ts`・`manifest.test.ts`・`jobCodes.test.ts`・`TestProductionProfileDocsContract`・`TestADRRecords`）と、加える文書のテスト（開発用のビルドの変数の表、README の移行の手順の manifest の場所）が確かめる。文書の文は PR のレビューで読む。
-   **手動の確認:** PR-1 の前と PR-6 の前（401 とタイトル）と PR-8（開発用のビルド、アクセシビリティ、本番の構成）で行い、§7 に記録する。
-   **後方互換:** 互換の仕組みは作らない（要件書 AC-32）。古い版の `sharedToken`・`serverPort` を読まず、消しもしないことは、`apiKeyPath.test.ts`（ステップ 6-2）が確かめる。

## 5. リスク管理 (Risk Management)

**技術のリスク**

| リスク | 対策 |
|---|---|
| 拡張の送信で Anthropic の 401 を読めず、キーの確かめと `api_key_rejected` が成り立たない | PR-1 の前に `fetch` で、PR-6 の前に Go の送信で確かめ、読めなければ止めて人と決め直す |
| 履歴の項目のタイトルが上書きされ、ビューアの URL の形が変わる | PR-1 の前に確かめ、案 B なら PR-7 で URL を、PR-8 で ADR-0010 を改める |
| まだ使われない部品の形が、配線のときに合わないと分かる | 部品は 0090 設計書と設計書の型のとおりに作り、単体テストで固める。合わなければ PR-8 で直し、設計の変更が要るときは人と決める |
| 写したテストの行が、サーバのテストの期待とずれる | 比べるテストと、PR-3 の説明の対応表 |
| テストの削除で、残るパッケージのカバレッジが下がり、カバレッジの比較が失敗する | 削除の前に仮の commit で比べ、下がる関数のテストを先に足す（ステップ 8-4・9-2） |
| 公開する成果物に fake が入る | 依存の検査（PR-4）と成果物の検査（PR-5） |
| CI の変更の検出が、記事生成モジュールの依存の変更を見落とす | 依存を `go list -deps` で求め、失敗したら拡張の検査を走らせる（ステップ 5-5） |
| 拡張のビルドが手元の Go の版で変わる | `GOTOOLCHAIN` に `go.mod` の版を与え、成果物の検査が `buildInfo` の版を確かめる |
| 初めの拡張のビルドが、ツールチェーンのダウンロードでネットワークを使う | development.md に書く。エージェントは、そのビルドの前に人の承認を得る |
| 開発用のビルドが、PR-8 まで画面で使われず、壊れても気づかない | `make ext-check` と CI で `ext-build-dev` を走らせる（§1.3 の決定 11） |
| 切り替えの PR で、固定された見出しやリンクを崩して文書のガードが失敗する | §1.4 の固定された箇所をステップ 8-5 に挙げた。README の変更は `/mktrans` と同じ commit にする |
| 撤去の PR で、残すパッケージを除くか、撤去したパッケージを指す設定とコマンドが残る | `_checks/removal.sh`（ステップ 9-5） |
| API キーを削除しても、始まった実行が最長 50 分 API を呼び続ける | 受け入れる（設計書 §5.1）。README に、止めるにはビューアを閉じることを書く |

**日程のリスク**

| リスク | 対策 |
|---|---|
| 確認のセッションに人の時間が要り、PR-1 と PR-6 の始まりが人の都合で決まる | 2 つの PR を `hold` にし、自動では送らない。ほかの PR は `auto` で続けられる |
| PR-8 の手動の確認の項目が多く、PR-8 が長くかかる | 確かめる項目をステップ 8-7 に挙げ、開発用のビルドで料金をかけずに進める。本番の構成での確かめは 2 本だけにする |

## 6. 実装チェックリスト (Implementation Checklist)

-   [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7）
-   [x] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5）
-   [x] PR-3 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6）
-   [x] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7）
-   [x] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6）
-   [x] PR-6 マージ済み（対象ステップ: 6-1 / 6-2 / 6-3 / 6-4 / 6-5 / 6-6 / 6-7 / 6-8 / 6-9）
-   [ ] PR-7 マージ済み（対象ステップ: 7-1 / 7-2 / 7-3 / 7-4）
-   [ ] PR-8 マージ済み（対象ステップ: 8-1 / 8-2 / 8-3 / 8-4 / 8-5 / 8-6 / 8-7）
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
| AC-01 | `cy/makefile_test.go::TestMakeBuildBuildsEveryBinary`（`yt2column` だけ）、`et/manifest.test.ts`（権限）、PR-8 の手動の確認（ホストもサーバもないプロファイルで記事が完成する） | test、manual | 8-1・8-7・9-3 | PR-8、PR-9 |
| AC-02 | `ej/module_test.go::TestCheckSettingsRejectsShape`・`TestCheckSettingsOutcomes`（形の検査、4 つの結果、30 秒）、`internal/llm/claude/keycheck_test.go::TestCheckKeyStatus`・`TestCheckKeyNotRejected`・`TestCheckKeySendsRequest`、`ej/devbackend/devbackend_test.go`、`et/optionsFlow.test.ts`（期限、1 回の書き込み、保存しないときに書かない、検査できない場合、書き込みの失敗）、`et/optionsView.test.ts`、§7 の 2 つの前の確認、PR-8 の手動の確認 | test、manual | 1-1・1-4・3-4・3-5・6-1・6-9・7-2・8-2 | PR-1、PR-3、PR-6、PR-7、PR-8 |
| AC-03 | `et/optionsFlow.test.ts`（保存の後に欄を空にする、保存した値を読まない）、`et/optionsHtml.test.ts`（文字を伏せた欄）、PR-8 の手動の確認（パスワードの保存を勧めない） | test、manual | 7-2・8-2・8-7 | PR-7、PR-8 |
| AC-04 | `et/optionsFlow.test.ts`（空のワークスペース ID を空の文字列で書く、1 回の `remove`）、`ej/module_test.go::TestGenerateAcceptsEmptyWorkspace`（空の文字列は指定なし）、PR-8 の手動の確認（再起動の後も残る） | test、manual | 3-5・7-2・8-7 | PR-3、PR-7、PR-8 |
| AC-05 | `ej/claudebackend/claudebackend_test.go::TestClientsMatchProvider`、`internal/config/extension_test.go::TestLoadExtensionProductionProfile`、`et/optionsHtml.test.ts`（欄が 2 つだけ） | test | 2-2・3-3・8-2 | PR-2、PR-3、PR-8 |
| AC-06 | `et/popupHtml.test.ts`（いつもあるリンク）、`et/popupView.test.ts`（API キーがないときにビューアを開かない）、`et/viewerRunner.test.ts`（`api_key_missing`）、`ej/module_test.go::TestGenerateSettingsInvalid`（`settings_invalid` で LLM を呼ばない）、`et/viewerApp.test.ts`（設定が要る状態と料金に触れないこと） | test | 3-5・7-3・8-2・8-3 | PR-3、PR-7、PR-8 |
| AC-07 | `et/jobCoordinator.test.ts`（生成の開始はポップアップからだけ、自動で送り直さない）、`et/viewerRunner.test.ts`（記録やビューアの URL を開くだけでは始めない）、PR-8 の手動の確認 | test、manual | 7-3・7-4・8-7 | PR-7、PR-8 |
| AC-08 | `et/jobCoordinator.test.ts`（止まった実行の 4 つの経路）、`et/viewerRunner.test.ts`（実行の間だけ読み込み直しの確認）、PR-8 の手動の確認 | test、manual | 7-3・7-4・8-7 | PR-7、PR-8 |
| AC-09 | `et/jobCoordinator.test.ts`（上限の境界と捨てる順、上限での拒否）、`et/jobRecord.test.ts`（記録の型が字幕と概要欄を持たない）、PR-8 の手動の確認 | test、manual | 6-7・7-4・8-7 | PR-6、PR-7、PR-8 |
| AC-10 | `et/jobCoordinator.test.ts`（同じ入力と同じ動画の照会、API キーがないときの順） | test | 7-4 | PR-7 |
| AC-11 | `et/viewerApp.test.ts`（3 つの状態で不明と示し、金額を示さない。`api_key_missing`・`settings_invalid` で触れない）、PR-8 の手動の確認 | test、manual | 8-3・8-7 | PR-8 |
| AC-12 | `internal/llm/claude/errors_test.go::TestHTTPStatusErrorAuthRejected`・`TestGenerateAuthRejected`、`ej/run_test.go::TestRunAuthRejectedAtEveryStep`（3 つの工程のそれぞれ）、`et/viewerApp.test.ts`（キーが拒まれた状態と操作）、`et/jobCoordinator.test.ts`（同じジョブ ID でやり直す）、§7 の 2 つの前の確認 | test、manual | 1-1・1-2・3-2・6-1・7-4・8-3 | PR-1、PR-3、PR-6、PR-7、PR-8 |
| AC-13 | `et/jobCoordinator.test.ts`（3 つ目の開始の拒否）、`et/popupView.test.ts`（開始が拒否された状態） | test | 7-4・8-3 | PR-7、PR-8 |
| AC-14 | `et/viewerApp.test.ts`・`et/popupView.test.ts`（状態ごとの固定の文言、例外の文を含まない）、`et/catalog.test.ts`（英語と日本語の文言が同じ項目を持つ）、文言の読み合わせ | test、manual | 8-3 | PR-8 |
| AC-15 | 文言の読み合わせ（PR のレビュー） | manual | 8-3 | PR-8 |
| AC-16 | `et/viewerTitle.test.ts`（設計書 §3.8.2 の表の各行、空にしない）、`et/jobViewerUrl.test.ts`（案 B の `t`）、`et/popupHtml.test.ts`（`viewer.html` の静的なタイトル）、§7 の PR-1 の前の確認、PR-8 の手動の確認（再起動の後の履歴） | test、manual | 1-1・6-8・7-1・7-4・8-2・8-7 | PR-1、PR-6、PR-7、PR-8 |
| AC-17 | `et/checkDist.test.ts`（開発用の manifest に `key` がない、`buildKind.js` が `dev`）、`et/popupView.test.ts`・`et/viewerApp.test.ts`（開発用の印の表示）、PR-8 の手動の確認 | test、manual | 5-3・5-4・8-2 | PR-5、PR-8 |
| AC-18 | `et/popupHtml.test.ts`・`et/optionsHtml.test.ts`（`lang`、名前、状態の領域、フォーカスの順）、`et/style.test.ts`（押せる領域、動きを止める指定。オプションページのスタイルシートを含む）、PR-8 の手動の確認（キーボード、コントラスト、VoiceOver） | test、manual | 8-2・8-7 | PR-8 |
| AC-19 | `ej/prepare_test.go`・`ej/result_test.go`・`ej/run_test.go`（サーバの 3 つのテストの行を写したもの）、`internal/server` の比べるテスト（PR-9 で除く） | test | 3-1・3-2 | PR-3 |
| AC-20 | `et/apiKeyPath.test.ts`、`et/lintRules.test.ts`（`storage` と決まった関数の規則）、`et/jobCoordinator.test.ts`（SW が値を読まない）、`et/wasmResult.test.ts`、`ej/module_test.go::TestGenerateKeepsKeyOutOfOutput`、`cmd/yt2column-extjob/main_test.go`（目印のキーが出力に出ない）、PR-8 の手動の確認（ログ、URL、`storage.session`） | test、manual | 3-5・4-3・6-2・6-3・6-4・6-6・7-4・8-7 | PR-3、PR-4、PR-6、PR-7、PR-8 |
| AC-21 | `internal/llm/llmhttp/llmhttp_test.go::TestStatusNoRedirect`・`TestPostNoRedirect`、ステップ 1-5 の共通の関数と `js` のファイルのテスト、`ej/claudebackend/claudebackend_test.go::TestClientsMatchProvider`（15 分）、`et/viewerRunner.test.ts`（50 分で `run_timeout`）、`et/lintRules.test.ts`（`fetch` は `wasmWorker.ts` だけ）、`et/manifest.test.ts`（CSP の `connect-src`） | test | 1-3・1-5・3-3・6-4・7-3・8-1 | PR-1、PR-3、PR-6、PR-7、PR-8 |
| AC-22 | `et/manifest.test.ts`（権限、キーの集合、CSP、`incognito`、`minimum_chrome_version`、`key`、オプションページ） | test | 8-1 | PR-8 |
| AC-23 | `et/launch.test.ts`（`setLocalAccessLevel` が失敗したら注入しない）、PR-8 の手動の確認（再起動の後） | test、manual | 8-2・8-7 | PR-8 |
| AC-24 | `et/jobCoordinator.test.ts`（送り手の確かめ）、`et/entryPoints.test.ts` | test | 7-4・8-3 | PR-7、PR-8 |
| AC-25 | `et/checkDist.test.ts`、`make ext-build`（成果物の検査を走らせる）、`ej/deps_test.go`（`js` の依存の含むものと含まないもの）、`cy/devdeps_test.go::TestNoDevDependency` | test、static | 4-5・5-1・5-4 | PR-4、PR-5 |
| AC-26 | `_checks/cli_unchanged.sh`（各 PR）。CLI の今のテスト（`cy/` の `_test.go`）が通ること | diff、test | 1-7 と各 PR の完了の条件 | PR-1〜PR-9 |
| AC-27 | `make wasm-check`（2 つの記事生成モジュールと依存）と、それを固定する `cy/makefile_test.go` のテスト、`ej/` のテスト。拡張の TypeScript が規則を持たないことはレビューで読む | static、test、manual | 3-1・4-4・8-3 | PR-3、PR-4、PR-8 |
| AC-28 | `cmd/yt2column-extjob/main_test.go` | test | 4-3 | PR-4 |
| AC-29 | `make ext-build-dev`（`make ext-check` と CI が走らせる）、ビルドのスクリプトのテスト（合わないシナリオで止まる、`-ldflags`）、`et/checkDist.test.ts`（シナリオの値）、`ej/devbackend/devbackend_test.go`、`internal/llm/devfake/scenario_test.go::TestParseScenario`、`ej/deps_test.go`（開発用に Claude が入らない）、開発用のビルドの変数の表の文書のテスト、手で出せない状態の単体テスト（`et/jobCoordinator.test.ts`・`et/viewerRunner.test.ts`・`et/optionsFlow.test.ts`・`et/wasmWorkerClient.test.ts`）、PR-8 の手動の確認 | test、static、manual | 2-3・3-4・4-5・5-3・5-6・6-5・7-2・7-3・7-4・8-7 | PR-2〜PR-8 |
| AC-30 | `et/ciChanges.test.ts`、依存のディレクトリのスクリプトのテスト、`et/repository.test.ts`（手順の順）、`_checks/removal.sh`（設定が撤去したパッケージを指さない） | test、static | 5-5・9-3・9-5 | PR-5、PR-9 |
| AC-31 | `_checks/removal.sh`（ディレクトリ、残るパッケージ、`make -n build-dev` の失敗）、`TestPackageReferenceListsPackages`、`make deadcode`（どこからも使われないテストの補助）、`et/lintRules.test.ts`（`sendNativeMessage` の禁止が `chromeDeps.ts` にも当たる） | static、test | 9-2・9-3・9-5 | PR-9 |
| AC-32 | `et/apiKeyPath.test.ts`（`sharedToken`・`serverPort` を参照しない、`get` と `remove` の引数が 2 つの名前だけ、`clear` を使わない）、`et/manifest.test.ts`（`nativeMessaging` を持たない） | test | 6-2・8-1 | PR-6、PR-8 |
| AC-33 | 文書の読み合わせ（PR のレビュー）、`_checks/removal.sh`（文書のコマンドの例）、文書のガード（§4） | manual、static | 1-6・2-5・3-6・4-7・5-6・8-5・9-4 | PR-1〜PR-5、PR-8、PR-9 |
| AC-34 | README の読み合わせ（PR のレビュー）、`cmd/yt2column-nmhost` の移行の手順の manifest の場所のテスト（ステップ 8-5）、`TestReadmeSyncMarker`・`TestReadmeVersionsShareStructure` | manual、test | 8-5 | PR-8 |
| AC-35 | 各 PR のレビューでの ADR の読み合わせ。作業として、各 PR は §1.2 の手順で、`HEAD` に残る記述を §3.3 と照合する | manual | 4-7・5-6・8-6・9-6 | PR-4、PR-5、PR-8、PR-9 |

AC-15・AC-35 は文書と文言の自然言語の内容だけを定める基準で、PR のレビューの読み合わせで確かめる。AC-33・AC-34 は、文の内容を読み合わせで、値とコマンドの例をテストとスクリプトで確かめる。

### AC-26 の記録

各 PR のブランチで `_checks/cli_unchanged.sh` を走らせ、PR ごとに次の形で書く（PR の番号、merge-base、比べたコミット、出力）。

**PR-1**（PR の番号は PR を作った後に決まるので、PR のチェックポイントの `PR を作成した` の行に URL がある）

-   merge-base: `9ef11235d31bc1cdd0c8368e7c14903859215500`
-   比べたコミット: `946f01d4691dbc78660475b2e0e585b1bd40394f`
-   出力（終了コード 0）:

    ```text
    merge-base 9ef11235d31bc1cdd0c8368e7c14903859215500 head 946f01d4691dbc78660475b2e0e585b1bd40394f
    ok: no non-test file under cmd/yt2column/ changed
    ```

-   この記録より後の commit は、計画書とその記録だけを変える（Go のコードを変える commit は、記録の前に済ませた）。
-   スクリプトは、`cmd/yt2column/main.go` の変更で終了コード 1、`_test.go` だけの変更で 0、`main.go` を `_test.go` の名前に変えるリネームで 1 になることを、使い捨ての worktree で確かめた（ステップ 1-7）。

**PR-2**（PR の番号は PR を作った後に決まるので、PR のチェックポイントの `PR を作成した` の行に URL がある）

-   merge-base: `6d9bdfa951602b7d9815b6f3d8c6bb87e5ee73a2`
-   比べたコミット: `6843188e51fb55986f36633a26d50476f6e431f3`
-   出力（終了コード 0）:

    ```text
    merge-base 6d9bdfa951602b7d9815b6f3d8c6bb87e5ee73a2 head 6843188e51fb55986f36633a26d50476f6e431f3
    ok: no non-test file under cmd/yt2column/ changed
    ```

-   この記録より後の commit は、テストの行の移動、計画書とその記録だけを変える（Go のコードを変える commit は、記録の前に済ませた）。
-   スクリプトは PR-1 から変えていないので、壊し方ごとの失敗の確かめはステップ 1-7 の記録のとおりである。

**PR-3**（PR の番号は PR を作った後に決まるので、PR のチェックポイントの `PR を作成した` の行に URL がある）

-   merge-base: `d731f09fdf32727c210db854b1acbf1240a399bf`
-   比べたコミット: `f359e30893d75d88d4fa75dcffb5966b8d6b852a`
-   出力（終了コード 0）:

    ```text
    merge-base d731f09fdf32727c210db854b1acbf1240a399bf head f359e30893d75d88d4fa75dcffb5966b8d6b852a
    ok: no non-test file under cmd/yt2column/ changed
    ```

-   この記録より後の commit は、計画書とその記録だけを変える（Go のコードを変える commit は、記録の前に済ませた）。
-   スクリプトは PR-1 から変えていないので、壊し方ごとの失敗の確かめはステップ 1-7 の記録のとおりである。

**PR-4**（PR の番号は PR を作った後に決まるので、PR のチェックポイントの `PR を作成した` の行に URL がある）

-   merge-base: `e56df9846d18e57b00d224418ac9fde0a1881ca4`
-   比べたコミット: `19450aa3db0f7fdc23dbbf1156289315a9a03903`
-   出力（終了コード 0）:

    ```text
    merge-base e56df9846d18e57b00d224418ac9fde0a1881ca4 head 19450aa3db0f7fdc23dbbf1156289315a9a03903
    ok: no non-test file under cmd/yt2column/ changed
    ```

-   この記録より後の commit は、計画書とその記録だけを変える（Go のコードを変える commit は、記録の前に済ませた）。
-   スクリプトは PR-1 から変えていないので、壊し方ごとの失敗の確かめはステップ 1-7 の記録のとおりである。

**PR-5**（PR の番号は PR を作った後に決まるので、PR のチェックポイントの `PR を作成した` の行に URL がある）

-   merge-base: `05662669d92643e2ff782aaf469ead3fa6dd29b2`
-   比べたコミット: `67145404c7968e84f240f78f24bb43c563117b87`
-   出力（終了コード 0）:

    ```text
    merge-base 05662669d92643e2ff782aaf469ead3fa6dd29b2 head 67145404c7968e84f240f78f24bb43c563117b87
    ok: no non-test file under cmd/yt2column/ changed
    ```

-   この記録より後の commit は、計画書とその記録だけを変える（Go のコードを変える commit は、記録の前に済ませた）。
-   スクリプトは PR-1 から変えていないので、壊し方ごとの失敗の確かめはステップ 1-7 の記録のとおりである。

**PR-6**（PR の番号は PR を作った後に決まるので、PR のチェックポイントの `PR を作成した` の行に URL がある）

-   merge-base: `e638505db18c0cebe2ddceefa65b31c523c99250`
-   比べたコミット: `bbce89ede7c4e948998e9d62c5633be33a4e2eb4`
-   出力（終了コード 0）:

    ```text
    merge-base e638505db18c0cebe2ddceefa65b31c523c99250 head bbce89ede7c4e948998e9d62c5633be33a4e2eb4
    ok: no non-test file under cmd/yt2column/ changed
    ```

-   この記録より後の commit は、計画書とその記録だけを変える（コードを変える commit は、記録の前に済ませた）。
-   スクリプトは PR-1 から変えていないので、壊し方ごとの失敗の確かめはステップ 1-7 の記録のとおりである。

### PR-1 の前の手動の確認の記録

ステップ 1-1 で書く。ブラウザの名前と版、確かめた組み合わせ（`GET /v1/models` と `POST /v1/messages`、`anthropic-workspace-id` のありとなし）、見たもの、履歴のタイトルの確かめの手順と結果、決めた案（A か B）を書く。

2026-11-16 に、リポジトリの外に作った試作の拡張（`host_permissions` なし、CSP は 0090 設計書 §3.7.2 の値、確認用に `storage` と `history` の権限を加えた）で確かめた。実 API の呼び出しは、人の承認を得て、ダミーのキーだけで行った（401 と 400 は料金がかからない）。

-   **ブラウザ。** Chrome 157 と Brave（Chromium 157）。どちらも macOS。版は `navigator.userAgentData.brands` の値で、細かい版（ビルド番号）は記録していない。Brave のユーザエージェントは Chrome と同じなので、`brands` の `Brave` で区別した。
-   **401 を読めるか（設計書 §1.2・§7.2）。** 拡張のページから、公開用のビルドが送る 4 つのヘッダ（`x-api-key`・`anthropic-version`・`content-type`〔POST のみ〕・`anthropic-dangerous-direct-browser-access`）と `redirect: "error"` で `fetch` を送った。**Chrome と Brave の両方で、通信の失敗にならず、状態コードを読めた。**

    | 組み合わせ | Chrome | Brave |
    |---|---|---|
    | `GET /v1/models?limit=1`、ワークスペース ID なし | 401 `authentication_error`（`invalid x-api-key`） | 同じ |
    | `POST /v1/messages`、ワークスペース ID なし | 401 `authentication_error` | 同じ |
    | `GET`、形式の整った ID（`wrkspc_01AbCdEfGhJkLmNpQrStUvWx`） | 401 `authentication_error` | 同じ |
    | `POST`、形式の整った ID | 401 `authentication_error` | 同じ |
    | `GET`、形式不正な ID（`wrkspc_probe_invalid`） | 400 `invalid_request_error`（`anthropic-workspace-id header must be a valid workspace ID.`） | 同じ |
    | `POST`、形式不正な ID | 400 `invalid_request_error`（同上） | 同じ |

    最初の試行は、形式不正な ID だけを「あり」として送ったので 400 になった。ID の形式の検査は、キーの検査より前に行われる。形式の整った ID なら、誤ったキーは 401 になる。401 を読めたので、事前の確認（CORS の preflight）も通っている。形式不正な ID は 400 になり、`CheckKey` は `llm.ErrAuthRejected` に当たらないエラーを返すので、`checkSettings` は `unverified` として保存する（設計書の分類のとおり。変更は要らない）。
-   **履歴の項目のタイトルが書き換わるか（設計書 §3.8.2、H-05）。** 試作のページは、`chrome.storage.session` に値があれば `VIDEO: <値> - yt2column` を、なければ `FIXED: lost - yt2column` を `document.title` にする。両方のブラウザで、(1) 値を置いてページを読み込み、履歴の項目（`chrome.history.search`）のタイトルが `VIDEO: Probe Video Title ABC - yt2column` であることを確かめ、(2) ブラウザを完全に終了し、前回のタブを復元して、タブを前面にして読み込ませた。`storage.session` は再起動で消えるので、ページは `FIXED: lost - yt2column` を出した。

    | | 再起動の前の履歴のタイトル | 復元した後の履歴のタイトル |
    |---|---|---|
    | Chrome | `VIDEO: Probe Video Title ABC - yt2column` | `FIXED: lost - yt2column` |
    | Brave | `VIDEO: Probe Video Title ABC - yt2column` | `FIXED: lost - yt2column` |

    **両方のブラウザで、履歴の項目のタイトルが固定のタイトルに上書きされた。**
-   **決めた案。** **案 B** で実装する（設計書 §3.8.2）。ビューアの URL にクエリ `t` を加える。PR-6 のタイトルを決める関数と、PR-7 のビューアの URL の組み立てと解析（`core/jobViewerUrl.ts`）は、案 B の規則（64 コードポイント、制御文字なし、`r` か `k` と一緒にだけ持つ）で実装する。案 B のコードを入れる PR で、ADR-0010 の「ビューアの URL」の決定を改め、要件書 §3.2 の「ビューアの URL」の行の扱いを PR の説明に書く（設計書 §3.8.2）。

### PR-6 の前の手動の確認の記録

ステップ 6-1 で書く。ブラウザの名前と版、確かめた組み合わせ、`checkSettings` と `generate` の結果を書く。

2026-11-17 に、リポジトリの外に作った試作の拡張で確かめた。試作の拡張は、`host_permissions` を持たず、CSP を 0090 設計書 §3.7.2 の値にした manifest と、拡張のページ、そのページが作る Worker からなる。Worker は `wasm_exec.js` を読み込み、記事生成モジュールを `fetch` で読んで動かし、ページからのメッセージで 4 つの関数を呼ぶ。記事生成モジュールと `wasm_exec.js` は、`main`（`e638505d`）で `make ext-build` が作った `extension/dist/` のものを写した。実 API の呼び出しは、人の承認を得て、誤ったダミーのキーだけで行った（401 は料金がかからない）。

-   **ブラウザ。** Chrome 157 と Brave（Chromium 157）。どちらも macOS。版は `navigator.userAgentData.brands` の値である（Chrome は `Google Chrome 157`、Brave は `Brave 157`）。
-   **読み込んだ記事生成モジュール。** `buildInfo` は、両方のブラウザで `{"goVersion":"go1.26.5","vcsRevision":"","vcsModified":false,"mainPath":"github.com/isseis/yt2column/cmd/yt2column-wasm","llm":{"kind":"claude"}}` を返した。公開用の記事生成モジュールである。
-   **呼び出し。** ワークスペース ID のなし（空の文字列）とあり（形式の整った `wrkspc_01AbCdEfGhJkLmNpQrStUvWx`）のそれぞれで、`checkSettings` を呼び、続けて `prepare`（`videoUrl` は `https://www.youtube.com/watch?v=dQw4w9WgXcQ`、`transcript` は `0:00\nHello\n`）で入力を渡し、`generate` を呼んだ。**Chrome と Brave の両方で、4 つとも通信の失敗にならず、期待した結果になった。**

    | 呼び出し | ワークスペース ID | 期待 | Chrome | Brave |
    |---|---|---|---|---|
    | `checkSettings`（`GET /v1/models?limit=1`） | なし | `key_rejected` | `{"status":"key_rejected"}` | 同じ |
    | `generate`（`POST /v1/messages`） | なし | `failed`・`api_key_rejected` | `failed`、`failure` は `api_key_rejected`、`steps` は `draft` が `failed`、ほかは `pending` | 同じ |
    | `checkSettings` | あり | `key_rejected` | `{"status":"key_rejected"}` | 同じ |
    | `generate` | あり | `failed`・`api_key_rejected` | `failed`、`failure` は `api_key_rejected`、`steps` は `draft` が `failed`、ほかは `pending` | 同じ |

    `prepare` は、どちらのブラウザでも `ok` を返した。`generate` は、進み具合の関数を `draft` が `running` の知らせで 1 回呼んでから結果を返した。各呼び出しは 0.4〜0.6 秒で返った。ステップ 1-1 の `fetch` での結果と同じく、Worker の中の Go の送信でも 401 を読めたので、設計書の `CheckKey` と `api_key_rejected` の分類は変えない。

### PR-8 の手動の確認の記録

ステップ 8-7 で書く。確かめた項目ごとに、ブラウザと版、手順、結果を書く。VoiceOver の結果は、PR-8 の説明にも書く（設計書 §3.6）。

（まだない）

### `_checks/removal.sh` の記録

ステップ 9-5 で書く。

（まだない）

## 8. 成功の基準 (Success Criteria)

-   **機能:** 要件書の AC-01〜AC-35 が、§7 の確かめで満たされる。拡張のほかに何も導入していないプロファイルで、Chrome と Brave の両方で記事が完成する。
-   **品質:** 各 PR で、`make test`・`make lint`・`make deadcode` と、拡張を変えた PR で `make ext-check` が通る。テストを削除した commit は、カバレッジの比較が通る。
-   **セキュリティ:** 公開する成果物に fake が入らず、API キーが要件書 AC-20 の経路の外に出ず、送り先が拡張自身と `https://api.anthropic.com` に限られることを、§7 の `test` と `static` の確かめが示す。
-   **文書:** 最後の PR のマージの時点で、サーバとホストを主経路として述べる文書がなく、#773 にリンクする食い違いの記述を持つ有効な ADR がない。

## 9. 次のステップ (Next Steps)

-   PR-1 の前に、ステップ 1-1 の確認のセッションを人と行い、その後 PR-1 から順に `/runplan` で実装する。PR-6 の前にも、ステップ 6-1 の確認のセッションを行う。
-   最後の PR のマージの後、issue #773 を閉じる（`/runpr` が案内する）。
-   Web Store の掲載と申請（#869）、シークレットモード（#874）は、本タスクの外で扱う（要件書 §2.3）。

## 変更履歴

- 2026-11-16 承認（isseis、#891）
- 2026-11-16 セッション中の再承認：2 つの記事生成モジュールが共有する `syscall/js` の登録を `internal/extjob/jsmodule` に置いた（isseis、#914）
