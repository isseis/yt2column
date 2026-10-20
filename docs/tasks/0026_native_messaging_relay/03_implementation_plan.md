# 実装計画書：Native Messaging ホストを中継役にした Unix ドメインソケット経由の送信

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-18 |
| Review date | 2026-10-19 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）で定義した、ブラウザ拡張とサーバの通信を HTTP（loopback）と共有トークンから Native Messaging ホストと Unix ドメインソケット（UDS）に移す変更を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。設計書 8 章の 3 つのフェーズをそのまま本計画のフェーズにする。

-   **フェーズ 1（サーバの UDS 化）:** ソケットのパスの導出（`internal/config`）、待ち受け（新しい `internal/serversock`）、ヘッダの検査の削減（`internal/server`）、起動の手順（`cmd/yt2column-server`）、`internal/authtoken` の削除、サーバの文書とガード。
-   **フェーズ 2（ホスト）:** `config.LoadHost`・`config.LoadHostHome`、中継と manifest の install/uninstall（新しい `internal/nmhost`・`cmd/yt2column-nmhost`）、実際のサーバとホストの統合テスト、Makefile、ホストの文書とガード。
-   **フェーズ 3（拡張）:** `sendNativeMessage` への切り替え、ホストの応答の検査（新しい `core/native.ts`）、送信の結果の分類と文言、オプションページとトークンの削除、拡張の文書とガード、Chrome と Brave の手動の確認。

本計画が使う記号は次のとおりである。AC-NN・F-NNN は要件書、H-NN は [design_handoff.md](design_handoff.md) の項目を指す。「§N」は本計画の節、「設計書 N」は設計書の節を指す。用語は設計書の冒頭の「用語」に従う。

### 1.2. 実装原則

-   設計書 1.1 の設計原則に従う。特に、ジョブ・本文の検証・失敗の通知のコードを変えないこと、ホストがサーバの応答を解釈しないこと、失敗を「どこで止まったか」で分類し分類できないものを結果不明に倒すこと、権限や manifest を修復せずに拒否することを守る。
-   Go と拡張のソースのコメント・識別子・文字列リテラル・利用者に表示する文言は英語で書く（CLAUDE.md「Go Idioms」）。`AC-NN`・`F-NNN`・`H-NN` はソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
-   テストの名前は計画上の名前である。実装で変える場合は、§5 を同じコミットで更新する。
-   各テスト・ガード・lint の規則について、対象の処理を実際に壊して失敗することを確かめ、そのことをコミットメッセージに書く（CLAUDE.md「Testing Strategy」）。各 PR の壊して確かめるステップ（§3.2）に、壊す対象のうち取り違えやすいものを挙げる。挙げていないものも含め、§5 でその PR が作る・改めるテストとガードのすべてについて、壊す対象を 1 つ以上決めて確かめる。壊す対象は本番のコード（またはガードが読む文書）であり、テストの入力ではない。
-   テストの結果の再利用（`go test` のキャッシュ）で、壊したのに古い成功が返ることがある。壊して確かめるときは `-count=1` を付けて実行する。
-   各フェーズの完了条件は、`make fmt` → `make test` → `make lint` → `make ext-check` が通ることである。フェーズ 1 も、README の拒否の理由の表と拡張の `serverRejectStatus` を同時に変えるので `make ext-check` を通す（§1.3「フェーズごとの文書とガードの割り当て」）。
-   既存のテストを削除するときは、削除の前後で `go tool cover -func` の関数ごとの結果を比べ、削除したテストだけが確かめていた関数の行が残らないことを確かめ、コミットメッセージに書く（CLAUDE.md「Deleting a test is a claim that must be checked」）。既存のテストの行のうち、削除する拒否の理由（例: `unauthorized`）を入力に使って別の振る舞い（状態コードと理由の組、拒否の表示）を確かめている行は、削除せず、残る拒否の理由に入れ替える。
-   gosec が指摘する構成（変数のパスの `os.OpenFile`・`os.Rename`、変数を引数にした `exec.Command` など）を本番のコード、または `_test.go` ではない補助のファイルに入れる場合は、その最小の範囲に理由を書いた `//nolint:gosec` を付ける（`.golangci.yml` の gosec の除外は `_test.go` だけに効く。既存の例は `internal/cachelock/cachelock.go:79`・`cmd/yt2column-server/test_helpers.go:628`）。統合テストのホストのビルド（`exec.Command`）は `_test.go` の中に置く。

### 1.3. 既存コード調査結果

HEAD `1f80866` で確認した（2026-10-18）。設計書が記す既存のファイルの行番号は、コミット `9e34229` のものである。`git diff --stat 9e34229 HEAD -- internal cmd extension README.md docs/dev Makefile .golangci.yml CLAUDE.md .github` の出力は、0025 の PR-1・PR-2 が変更した拡張のファイル（`extension/src/browser/chromeDeps.ts`・`extension/src/core/collect.ts`・`extension/src/core/messages.ts`・`extension/src/core/metadata.ts`・`extension/src/launch.ts` とそのテスト）の 7 件だけだった。したがって、設計書が引く Go のファイルと文書の `file:line` は HEAD `1f80866` でもそのまま有効である。拡張の `launch.ts`・`chromeDeps.ts` の行番号は数行ずれている（例: `logSend` は設計書の `launch.ts:135-153` から `launch.ts:141` 以降に移った）。以下の `file:line` は HEAD `1f80866` のものである。

**再利用する既存の実装**

-   `internal/cachelock` の `Acquire`（`internal/cachelock/cachelock.go:71-102`）を起動のロックにそのまま使う（設計書 3.2.2）。変えるのはパッケージのコメント（`:3-5`）とエラーの文言だけである。`ErrLocked` の文言（`:33`）と `Acquire` の `MkdirAll` の失敗の文言（`:73`）が「cache directory」を名指している。
-   `internal/cachelock/test_helpers.go:13` の `requireNonRoot` と `:24` の `chmodForTest` は package 内の補助で、他のパッケージから使えない。`requireNonRoot` は root のとき `t.Fatal` で失敗させる（`:13-18`）。設計書 7.1 は所有者と権限に依存するテストを root では「飛ばす」とするので、`internal/serversock`・`internal/nmhost` では `t.Skip` で飛ばす補助を自分の `test_helpers.go` に置く（数行の補助で、パッケージをまたぐ補助にするほどの量ではない）。
-   `internal/config` の `absoluteEnv`（`internal/config/cachedir.go:99-107`）、`missingVarWithReason`（`internal/config/config.go:744`）、`invalidVar` を、ソケットのパスの導出が返す `VarError` を作るのに使う。`userDirBase`（`cachedir.go:74-97`）は `XDG_*` を読むので使わない（設計書 3.1）。
-   `internal/config` は `net` を import できない（`internal/pipeline/pipeline_test.go:876` の `TestConfigImports`）。設計書 3.1 のとおり、OS ごとの上限は定数で持ち、`syscall.RawSockaddrUnix` との一致は `internal/serversock` のテストで確かめる。`go doc syscall.RawSockaddrUnix` で `Path` の長さが darwin で 104、`GOOS=linux` で 108 であることを確かめた（Go 1.27.1、2026-10-18）。
-   `internal/server` の本文の検証・枠・ジョブ・失敗の通知（`request.go`・`job.go`）と、その大半のテスト（`job_test.go`・`request_test.go` の `TestRejectBodyTable` など）は、`httptest.NewRecorder` で `handler` を直接呼ぶ。ヘッダを組み立てる補助（`internal/server/test_helpers.go:174-187` の `newRequest`・`withValidHeaders`）だけを改めれば、これらのテストの入力の表はそのまま使える。AC-08・AC-10・AC-13 は既存のテストで確かめ、新しい表を書かない。
-   AC-07 の「応答が先に返る」は、既存の `TestServeShutdownWaitsForJobs`（`internal/server/server_test.go:145`）が、ゲート付きの writer でジョブを止めたまま `202` を受け取ることで確かめている。UDS のリスナーに移すだけで、新しいテストを書かない。
-   `cmd/yt2column-server/test_helpers.go` の `testDeps`・`loopbackDeps`（`:396`）・`webhookServer`（`:426-434`）・子プロセスの起動（`:608-663`）は、待ち受けの確かめ方（ポート → ソケットのパス）を除いてそのまま使う。統合テスト（§2 ステップ 2-15）もこれらを使う。ただし、空いたポートを探して起動をやり直す 2 つの再試行のループ（`startServerWithStdout` の `:277-303` と子プロセスの `:626-659`）は削除する。UDS では `exitFailure` はディレクトリの不備・既存の待ち受け・二重起動を意味し、やり直す理由がない。
-   `cmd/yt2column/extension_docs_test.go:448` の `extensionIDFromManifestKey` を、`nmhost.ExtensionOrigin` の照合に使う。`:552` の `checkManualCheckTable` を、§5.1 の手動の確認の表の検査に使う。
-   `internal/pipeline/pipeline_test.go:669` の `TestPackageReferenceListsPackages` は、`cmd/`・`internal/` の本番のコードを持つパッケージと `package_reference.md` の行が一致することを確かめる。パッケージを足す・消すフェーズで、同じコミットに行の追加・削除を入れる（§1.3「フェーズごとの文書とガードの割り当て」）。同じファイルの `isTestOnlyFile`（`:1004`）は、新しいガード（下記）が本番のファイルを選ぶのに使える。
-   拡張の `classifyResponse`（`extension/src/core/response.ts:145-215`）・`hasExactKeys`（`extension/src/core/summary.ts:70`）・`maxResponseBytes`（`response.ts:65`）をそのまま使う（設計書 3.7.3）。`ReceivedResponse` の `body-over-limit` の形も使う。

**名前の変更の一覧（本計画の正）**

形の変わる既存の名前は、この表を正とする。タスクの本文と §5 はこの表に従う。

| 種類 | 旧 | 新 | 場所 | ステップ |
|---|---|---|---|---|
| メソッド | `(*server.Server).Serve(ctx, []net.Listener)` | `Serve(ctx, net.Listener)` | `internal/server/server.go:174` | 1-8 |
| 関数 | `run(ctx, args, lookup, stdout, stderr, d)` | `run(ctx, args, lookup, stderr, d)` | `cmd/yt2column-server/run.go:135` | 1-11 |
| 関数 | `buildServer(d, cfg, token, logOutput, secrets)` | `buildServer(d, cfg, logOutput, secrets)` | `cmd/yt2column-server/run.go:230` | 1-11 |
| テスト | `TestLoadServerTokenPathUnderivable` | `TestLoadServerSocketPathUnderivable` | `internal/config/config_test.go:980` | 1-7 |
| テスト | `TestListenBindsLoopbackOnly` | `TestServeOnUnixListener` | `internal/server/server_test.go:95` | 1-9 |
| テスト | `TestServeListenerFailureStops` | 同名（リスナー 1 つの形に書き直す） | `internal/server/server_test.go:120` | 1-9 |
| 関数 | `prepareSend(outcome, settings, metadata): Promise<PrepareOutcome>` | `prepareSend(outcome, metadata): PrepareOutcome` | `extension/src/core/prepare.ts:95` | 3-4 |
| メンバ | `ArticleBody.json` | `ArticleBody.message` | `extension/src/core/prepare.ts:20-23` | 3-4 |
| interface | `SendDeps { fetch; timeoutSignal }` | `SendDeps { sendNative; schedule }` | `extension/src/send.ts:26-30` | 3-5 |
| 関数の戻り値 | `classifyResponse(): Exclude<SendOutcome, unknown>` | `Extract<SendResult, accepted \| rejected \| unexpected>` | `extension/src/core/response.ts:145` | 3-3 |

削除する名前（Go の `ServerConfig.Port`・`TokenPath`、`server.Options.Port`・`Token`、`(*Server).Listen`、`internal/authtoken` の全体、拡張の `settings.ts`・`optionsPage.ts`・`options.ts`・`createSettingsStore`・`PreparedRequest` の `url`・`port`・`token`、`SendOutcome` の `unknown`・`redirected` など）は設計書 3.1・3.3・3.7・3.8 のとおりである。新しい名前（`serversock.Acquire`・`Lock`、`config.HostConfig`・`LoadHost`・`LoadHostHome`、`nmhost.Relay`・`HostError`・`Install`・`Uninstall`、`core/native.ts` の各宣言など）は設計書 3.1〜3.7 のとおりである。

**削除する Go のテスト（1 つずつ）**

設計書 3.8 の表のうち、テスト関数ごと削除するものは次のとおりである。それぞれ、確かめていた振る舞いが消えるので代わりのテストは要らない。ただし、削除の前後で `go tool cover -func` を比べる（§1.2）。

-   `internal/config/config_test.go:912` `TestLoadServerDefaultPort`、`:999` `TestServerPort`、`:1050` `TestLoadServerPortRejected`、`internal/config/cachedir_test.go:92` `TestDefaultConfigDir`
-   `cmd/yt2column-server/run_test.go:232` `TestDisplayFailureKeepsServing`、`:303` `TestTokenShownOnEveryStart`、`:361` `TestClosedStdoutPipeKeepsServing`
-   `internal/authtoken/authtoken_test.go`（パッケージごと）

**設計書 3.8 の表にない、更新が必要な既存のテストとガード**

-   `internal/server/server_test.go:210` `TestBodyReadFailureWritesNoResponse` は `httptest.NewServer`（TCP）と `Host`・`Origin`・`Authorization` のヘッダを使う。UDS のリスナーとパスとメソッドだけのリクエストに改める。
-   `internal/server/server_test.go:26-91` の補助（`requireIPv6Loopback`・`freePort`・`newServerOnPort`・`listenOnFreePort`・`closeListeners`）は、UDS のリスナーの補助に置き換える。
-   `cmd/yt2column/extension_docs_test.go:197-204`（`### The shared token` の節があることの確認）は、フェーズ 1 でこの節を削除するので、同じフェーズで削除する。トークンを貼り付ける手順が残っていないことの確認は、拡張の節を改めるフェーズ 3 で加える。
-   `extension/test/serverReasons.test.ts:34-45` は README の拒否の理由の表と `serverRejectStatus` が完全に一致することを確かめる。フェーズ 1 で表から 4 つの理由を消すので、同じフェーズで拡張から同じ 4 つを消す（下記）。
-   `cmd/yt2column-server/docs_test.go` は `run_test.go` と同じパッケージにあり、ステップ 1-6・1-7・1-9 で削除する次の補助を呼んでいる。`cfg.Port()`（`:263`）、`config.ServerPortEnvForTest()`（`:33`）、`config.TokenFileNameForTest()`（`:35`）、`server.ExtensionOriginForTest()`・`server.JSONContentTypeForTest()`（`:101-102`）である。これらを削除すると `cmd/yt2column-server` のテストがすべてコンパイルできなくなる。そこで、コンパイルを通すための削除（`serverDefaultPort` と、削除した補助の呼び出し）はステップ 1-12 に含め、文書の内容の確認の書き換えはステップ 1-15 で行う。ステップ 1-6〜1-12 はこのようにコンパイルで互いに結び付いているので、PR の境界は、この範囲の途中に置かない（§3.2）。
-   `extension/test/manifest.test.ts:94` のテストの名前は「requires Chrome 103 for AbortSignal.timeout」である。フェーズ 3 で `AbortSignal.timeout` を使わなくなる（設計書 3.7.1）ので、版を変えずに名前を改める。
-   `internal/pipeline/pipeline_test.go:582-586` の `testOnlyPackageDirs` は、テスト専用のパッケージのすべてのファイルが `test` のビルドタグを持つことを `TestFakesCarryBuildTag` に確かめさせる一覧である。新しい `internal/udstest` を加える。
-   `make -n build` の出力は、HEAD `1f80866` で `mkdir -p build`・`go build -trimpath -ldflags "-s -w" -o build/yt2column ./cmd/yt2column`・`go build ... -o build/yt2column-server ./cmd/yt2column-server` の 3 行だった（終了コード 0、2026-10-18 に実行）。Makefile は変数（`$(SERVER_PKG)` など）でパッケージを渡すので、Makefile の文字列ではなく、展開した後のレシピを確かめる（`TestMakeBuildBuildsEveryBinary`）。

**フェーズごとの文書とガードの割り当て（本計画の決定）**

設計書 8 章は、README と security.md のサーバの節をフェーズ 1、拡張の節をフェーズ 3 とする。文書とガードは互いに読み合うので、どのフェーズでも `make test`・`make ext-check` が通るように、次のように割り当てる。最後のフェーズを終えた時点で、文書とガードは設計書 3.9 の表のとおりになる。

| 対象 | フェーズ 1 | フェーズ 2 | フェーズ 3 |
|---|---|---|---|
| README | 「## Localhost server」のポートとトークンの記述を削除し、ソケットのパス、起動、サーバが起動していない場合のトラブルシューティング、`curl --unix-socket` の例、終了コードを書く。古い版からの更新の手順は、独立した見出し「### Upgrading from the shared-token version」の節に書く（トークンのファイル `server-token` を削除してよいことを含む。この節だけが、ガードの禁止する語の例外になる）。トークンが不要になったことを述べる文を書く。`### The shared token` を削除する。拒否の理由の表を 8 行にする（列は 2 つのまま）。「## Configuration」の表から `YT2COLUMN_SERVER_PORT` を削除する。「## Development」に開発者向けの独立したサーバの手順の節を加える（下記） | ホストの install/uninstall（Chrome・Brave、macOS での共有）、バイナリの移動、TCC（macOS の、書類などのフォルダへのアクセスを制限する仕組み。設計書 2.2）、トラブルシューティング（ホストが見つからない場合、ホストのログの見方）。`README.md:332-333`・`:510` の「`make build` が 2 つのバイナリを作る」記述を 3 つに改める。「## Development」の独立したサーバの手順の節にホストの制約を加える（下記） | 「### Browser extension」からトークン・オプションページ・`Site access`・`127.0.0.1` を削除し、ホストの install とホストが見つからない場合の案内を書く。拒否の理由の表に「拡張の表示」の列を加える。「### Upgrading from the shared-token version」に、旧版の拡張が保存したトークンを消したい場合は拡張を一度削除してから読み込み直すことを加える（設計書 3.7.1） |
| security.md | §8 の冒頭（UDS、相手ごとの排除の根拠、UDS の権限が同じ uid のプロセスを区別しないこと、同一ユーザーを信頼すること、共有トークンを削除した理由）、§8.2 のサーバの項目（ディレクトリ・起動のロック・stale）、残余リスク（設計書 5.4 のサーバの項目）。§6 の `--artifacts` の項目（`security.md:78`）から「共有トークン」を除く | §8.2 にホストの項目（標準出力が通信路、ログは標準エラー出力、秘密を読まない、上限、`allowed_origins`）と、ホストの残余リスク | §8.1（権限の一覧を `activeTab`・`scripting`・`nativeMessaging` にする、固定のホスト名、ホストの応答の検査、拡張が `chrome.storage` を使わないこと、旧版が `chrome.storage.local` に保存した値が残りうるが通用する相手がないこと）。§8.2 に、ホストを起動できるのは `nativeMessaging` の権限を持つ固定の拡張だけであることを加える（設計書 3.9 の表のガードが §8.2 の `nativeMessaging` を確かめる） |
| `project_overview.md`・`package_reference.md`・CLAUDE.md | `project_overview.md` の実行形式の記述、ディレクトリ構成（`internal/authtoken` → `internal/serversock`）、設定の表から `YT2COLUMN_SERVER_PORT`（`:120`）を削除。`package_reference.md` の `cmd/yt2column-server`・`internal/config`・`internal/server` の行を改める。`internal/authtoken` の行は、ディレクトリを削除するステップ 1-10 で削除する。`internal/udstest`・`internal/serversock` の行は、パッケージを作るステップ 1-2・1-3 で加え、`internal/cachelock` の行（起動のロックにも使うこと）もステップ 1-3 で改める（`TestPackageReferenceListsPackages`）。CLAUDE.md の Architecture Overview の `internal/authtoken` → `internal/serversock` | `cmd/yt2column-nmhost`・`internal/nmhost` の記述（`package_reference.md` の行は、パッケージを作るステップ 2-1・2-10 で加える）、CLAUDE.md の Build Commands（`make build` が 3 つのバイナリを作ること、`yt2column-nmhost install`・`uninstall <browser>`）、`project_overview.md` の install/uninstall のコマンド | CLAUDE.md の「Development Notes」の、`manifest.test.ts` が security.md §8.1 のホストの権限の項目を読むという記述を、権限の一覧を読むという記述に改める |
| `translation_glossary.md` | 「共有トークン」の行を削除し、起動のロックを加える | ホスト・ホスト名・ホストの応答 | 未送信・結果不明 |
| 拡張のコード | `ServerRejectReason` から `forbidden_host`・`unsupported_media_type`・`forbidden_origin`・`unauthorized` を削除する（`serverReasons.test.ts` の一致を保つため） | なし | 設計書 3.7 のすべて（`not_found`・`method_not_allowed` の削除を含む）。`static/style.css` のオプションページの規則（`.options-token`・`.options-port` など、`style.css:52-66`）の削除、`eslint.config.js` の `chrome.storage.sync` の規則（トークンを同期しないという理由）の、`chrome.storage` の参照の禁止への置き換え（ステップ 3-13） |

-   拡張の 4 つの理由の削除は、設計書 8 章ではフェーズ 3（設計書 3.7.3）の作業である。README の拒否の理由の表をフェーズ 1 でサーバの 8 つの理由に合わせると、`serverReasons.test.ts` の一致の検査が拡張の側の変更を同時に求めるので、本計画はこの部分だけをフェーズ 1 に移す。フェーズの順序と内容の境界は変えず、設計書の最終の状態も変わらない。設計書 8 章の表に、この移動を編集上の訂正として記録した（設計書の `Comments`）。

**本計画で置く静的な検査の場所（本計画の決定）**

設計書 7.3 は、静的な確認の内容を定めるが、置き場所を定めていない。本計画は次のように置く。どれも `make test` で実行され、CI の `doc-guard` のジョブ（`.github/workflows/ci.yml:184` の `go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./internal/pipeline`）でも実行される。

-   `internal/pipeline/relay_guard_test.go`（新規）。リポジトリ全体を読むガードは既に `internal/pipeline` にある（`TestPackageReferenceListsPackages`・`TestConfigImports`）ので、同じ場所に置く。本番のファイルは、`_test.go` と、`test` タグを付けたときだけビルドされるファイル（ビルドの制約を評価して判定する）を除いたものとする（`internal/config/envaccess_test.go:280-300` の `isTestCode` と同じ方法。別のパッケージの `_test.go` なので import できず、同じ判定を `relay_guard_test.go` に書き写す）。`internal/pipeline/pipeline_test.go:1004` の `isTestOnlyFile` は `//go:build test` の行と完全に一致するものだけを選び、`//go:build test || integration` のファイルを本番と見なすので使わない。
    -   `TestNoTCPListenerInProduction`（AC-01）: 本番のファイルに、TCP のリスナーを作る呼び出し（`net.Listen`・`net.ListenTCP`・`(*net.ListenConfig).Listen` で第 1 引数が `"tcp"`・`"tcp4"`・`"tcp6"` のもの、`http.ListenAndServe`・`http.ListenAndServeTLS`）と、`127.0.0.1`・`[::1]`・`::1` を含む文字列リテラルがないこと。HEAD `1f80866` では、該当するのは `internal/server/server.go:158`・`:160`・`:162`・`:165`（呼び出しとエラーの文言）と `internal/server/handler.go:192` である（`internal/llm/llmhttp/llmhttptest/llmhttptest.go:194`・`:247` は `//go:build test`）。
    -   `TestSharedTokenRemoved`（AC-05）: 識別子とファイル名 `authtoken`・`YT2COLUMN_SERVER_PORT`・`server-token` が、`cmd/`・`internal/` の本番の Go のファイル（テストのファイルは、この語を検索の対象やガードの検査に使う語として持つので除く）と、現在の文書（README・CLAUDE.md・`docs/dev/`・`docs/translation_glossary.md`。`docs/tasks/` を除く）にないこと。README の「### Upgrading from the shared-token version」の節だけは `server-token` を許す（設計書 3.9 がトークンのファイルを削除してよいと書くため）。この節でも `YT2COLUMN_SERVER_PORT` と `authtoken` は名指さない。フェーズ 3 で、走査の対象に `extension/src`・`extension/static` を加える（フェーズ 1 の時点では `extension/src/core/sendMessages.ts:49` がまだ `YT2COLUMN_SERVER_PORT` を含むため）。文書の中の「共有トークン」「shared token」という説明の語は、security.md が削除の理由を述べるのに必要である（要件書 F-005）ので、このガードでは禁止しない。トークンを貼り付ける手順が残っていないことは、`TestServerDocsContract`・`TestExtensionSendDocumentation` が節と語句を限って確かめ（ステップ 1-15・3-11）、説明の語の残りは §4.4 の横断検索で確かめる。
    -   `TestHostStdoutOnlyInMain`（AC-17、フェーズ 2。ステップ 2-4 で `internal/nmhost` を、ステップ 2-13 で `cmd/yt2column-nmhost` を走査の対象にする）: `internal/nmhost` と `cmd/yt2column-nmhost` の本番のファイルのうち、`cmd/yt2column-nmhost/main.go` だけが `os.Stdout`・`fmt.Print*` を参照すること。`internal/config/envaccess_test.go` と同じく構文木を走査する（設計書 7.3）。
    -   `TestHostSpawnsNoProcess`（AC-18、フェーズ 2）: `internal/nmhost` と `cmd/yt2column-nmhost` の本番のファイルが `os.StartProcess`・`syscall.Exec`・`syscall.ForkExec`・`syscall.StartProcess` を参照しないこと（`os/exec` の import は depguard が禁止する）。
-   `cmd/yt2column/makefile_test.go` に `TestMakeBuildBuildsEveryBinary`（AC-20・AC-33、フェーズ 2）: `make -n build` で展開したレシピが `./cmd/yt2column`・`./cmd/yt2column-server`・`./cmd/yt2column-nmhost` の 3 つをビルドし、どの行も `yt2column-nmhost` のバイナリに `install`・`uninstall` を渡さないこと（§1.3 の `make -n build` の出力）。`all` の目標についても同じことを確かめる。
-   `cmd/yt2column/extension_docs_test.go` に `TestNativeHostContract`（フェーズ 2 で作り、フェーズ 3 で広げる）: 設計書 3.9 の「（新規）同期のテスト」。フェーズ 2 では、`nmhost.ExtensionOrigin` と manifest の `key` から導出した ID が一致すること、`nmhost.RelayTimeout` が `extension/src/send.ts` の `sendTimeoutMs`（`send.ts:12` に既にある）より短いことを確かめる。フェーズ 3 で `core/native.ts` の `nativeHostName`・`hostReplyVersion`・`hostReplyKinds`・`hostErrorValues` と `nmhost` の定数の一致を加える。
-   `.golangci.yml` の `depguard` の規則（AC-18・AC-21、`make lint`）: フェーズ 1 で `internal/server` が `internal/serversock` を import しない規則、フェーズ 2 で `internal/nmhost` が `internal/server`・`internal/config`・`os/exec` を、`cmd/yt2column-nmhost` が `os/exec` を import しない規則。既存の `publisher-no-server`（`.golangci.yml` の `depguard.rules`）と同じ形にする。

**UDS のテストの補助（本計画の決定）**

macOS の `t.TempDir()`（`/var/folders/...`）の下にソケットを置くと、`sun_path` の上限（103 バイト）を超えうる（設計書 7.2）。`/tmp` の下に短い名前のディレクトリを作り `t.Cleanup` で消す補助と、そこに UDS のリスナーを作る補助を、`internal/serversock`・`internal/server`・`internal/nmhost`・`cmd/yt2column-server` のテストが共有する。

-   置き場所は、テスト専用の新しいパッケージ `internal/udstest`（`//go:build test`）とする。`test_organization.md` は、パッケージをまたぐ補助を `<package>/testutil/` に置くとする。本計画はこれに従わず、テスト専用のパッケージ（`internal/loopbacktest`・`internal/maketestutil` という前例がある）にする。理由は次の項目のとおりである。設計書 3.12 は「`internal/server/test_helpers.go` に UDS のリスナーの補助を加える」としていたので、設計書 3.12 の表を編集上の訂正として改めた（設計書の `Comments`）。
-   `internal/serversock/testutil` に置かない理由: フェーズ 1 で加える depguard の規則（`internal/server` が `internal/serversock` を import しない）は、`pkg` の前方一致で `internal/serversock/testutil` にも当たる。`internal/server/test_helpers.go` は `_test.go` ではないので、depguard の `$test` の除外が効かない。
-   ディレクトリの名前は、`os.MkdirTemp("/tmp", <短い接頭辞>)` で実行のたびに乱数を含めて作る。`go test ./...` は複数のパッケージを並行に実行し、中断した実行は `t.Cleanup` を通らないので、固定の名前やテストの名前から作る名前は、並行の実行や再実行でぶつかる（ぶつかると、起動のロックと stale の判定の結果として `ErrAlreadyRunning`・`ErrInUse` が返る）。補助自身のテストで、作るディレクトリのパスにソケットのパスの末尾（`HOME` を除く 54 バイト）を足しても macOS の上限（103 バイト）に収まることを確かめる。
-   `package_reference.md` に `internal/udstest` の行を加え（`TestPackageReferenceListsPackages` が求める）、`internal/pipeline/pipeline_test.go` の `testOnlyPackageDirs` に加える（`TestFakesCarryBuildTag` がすべてのファイルのビルドタグを確かめる）。
-   この補助を使うのは、`internal/serversock`・`internal/server`・`internal/nmhost`・`cmd/yt2column-server`・`cmd/yt2column-nmhost` のテストのうち、実際にソケットを作るものである。`cmd/yt2column-nmhost` の `LoadHost` の失敗の行は、`HOME` を未設定か相対パスにして作る（`t.TempDir()` の長さに頼らない）。

**開発者向けの独立したサーバの手順（本計画の決定）**

フェーズ 1 の後、ソケットのパスは `HOME` だけから決まり、起動のロックがユーザーごとに 1 つのサーバしか許さない（設計書 3.1・3.2）。これまでのように `YT2COLUMN_SERVER_PORT` でポートを変えて、worktree ごとにサーバを同時に動かす方法はなくなる。今後のサーバの変更（設計書 9 章の `/status` など）を並行して開発できるよう、README の「## Development」に次の手順の節を加える。設計書 3.9 の表にない文書の追加であり、設計の判断は変えない。

-   フェーズ 1（ステップ 1-14・1-15）: `HOME` を `/tmp` の下の短いディレクトリ（macOS の `mktemp -d` の既定のディレクトリではソケットのパスが上限を超える）にしてサーバを起動すること、そのソケットのパスに `curl --unix-socket` で送って確かめること、`HOME` を変えれば worktree ごとに別のサーバを同時に動かせること。偽の API キーと Webhook URL で起動する場合は、受理される入力を送らないこと（受理すると記事の生成と通知を試みる）。
-   フェーズ 2（ステップ 2-12・2-13）: ブラウザが起動するホストは本当の `HOME` のサーバにだけつなぐので、この手順のサーバに拡張から送ることはできないこと。manifest はブラウザごとに 1 つ（macOS では Chrome と Brave で共有）なので、ある worktree のビルドで install すると、他の worktree のものを置き換えること。拡張からの確認は 1 つの worktree ずつ行うこと。

**型と構築の経路**

-   本設計には、具体的な型で処理を分ける箇所はない。列挙型（`nmhost.HostError`・`nmhost.Browser`）は値で分け、ゼロ値を安全側に置く（設計書 3.5.3・3.6）。
-   `config.HostConfig` の非公開のフィールドは、既存の `ServerConfig`（`internal/config/config.go:235-241`）と同じ規約で守る（`internal/config` の中で `LoadHost` だけが値を作る）。`serversock.Lock` は非公開のフィールドを持ち、`Acquire` だけが作る（設計書 3.2）。どちらも構築の経路をガードで数えない（既存の `ServerConfig`・`cachelock.Lock` と同じ扱い）。

**0025 との順序（本計画の決定）**

設計書 8 章は、フェーズ 3 と 0025（拡張の動画メタデータ）のどちらを先に入れるかを本計画に委ねる。HEAD `1f80866` で 0025 は PR-1・PR-2 がマージ済み（`git log` の `6e808fb`（#316）・`32293c1`（#317）。0025 の計画の PR-2 の「マージされた」のチェックボックスは未更新）で、PR-3〜PR-5（表示、経路と配線、手動の確認と文書）が残っている。0025 の PR-4 は `launch.ts`・`popup.ts`・`background.ts` を、PR-5 は README の「### Browser extension」・security.md §8.1・`docs/translation_glossary.md`・`TestExtensionSendDocumentation` を変え、`extension/test/metadataDocs.test.ts`（README と security.md §8.1 を読む）を加える。本計画のフェーズ 3 は同じファイルと節を変えるので、**0025 のすべての PR がマージされた後にフェーズ 3 を始める**。フェーズ 3 では、`prepareSend` に渡す `ArticleMetadata`（0025 が `launch.ts` で組み立てる）をそのまま受け取り、0025 のガードが求める README と security.md §8.1 の語句を保つ。フェーズ 1・2 は README・security.md・用語集の別の節（サーバとホストの節）と、拡張の `response.ts`・`sendMessages.ts` とその表の行を変える。0025 の PR-5 と同じファイルの別の節なので、どちらかが先にマージされた後、後の側が取り込み直す。

**0027 との順序（本計画の決定）**

`0027_remove_context_menu`（右クリックメニューの経路の削除）は、拡張の service worker（`background.ts`）・結果のウィンドウ（`result.html`・`result.ts`）・`chrome.storage.session` の使用・ツールバーのバッジ・`contextMenus` の権限と、`launch.ts`・`chromeDeps.ts`・`core/resultView.ts`・`core/sendMessages.ts` のうちメニューの経路だけが使う部分を削除する。本計画のフェーズ 3 は、0027 の後の構成を前提にする（設計書 8 章）。

-   **0027 のすべての PR がマージされた後に PR-2 を始める。** 0026 は PR-1 の後、0027 が終わるまで止まる。理由は次の 2 つである。
    -   PR-2 のマージの後、`main` のサーバは UDS だけを待ち受けるので、フェーズ 3 の前の拡張（HTTP と共有トークンで送る）はサーバに届かない（設計書 8 章）。0027 の手動確認のうち、ポップアップから送って受付（`202`）の表示を確かめる行（0027 AC-07）は、届くサーバを要する。0027 を先に終えれば、0027 の手動確認は `main` のサーバでそのまま行える。
    -   PR-2 のステップ 1-13 は `core/response.ts`・`core/sendMessages.ts` とそのテストを、ステップ 1-15 は `TestExtensionSendDocumentation` を変える。0027 も `core/sendMessages.ts`（`unreadableResultMessage`）とそのテスト、`TestExtensionSendDocumentation` を変える。0027 を先に入れれば、PR-2 は 0027 の後のファイルの上で書け、取り込み直しが要らない。
-   PR-2 を始める前に、0027 の後の行番号（ステップ 1-13 が引く `response.test.ts`・`sendMessages.test.ts`・`renderSend.test.ts`、ステップ 1-15 が引く `extension_docs_test.go`）を確かめ直す。
-   **フェーズ 3 の前の確認。** フェーズ 3 を始める前に、0027 の設計（`design_handoff.md` の H-01 の決定）で次の名前が残ったかを確かめ、ステップ 3-6・3-7 の対象を合わせる: `chromeDeps.ts` の `schedule`（削除されていれば、ステップ 3-7 で `SendDeps` のために加え直す）、`core/resultView.ts` の `ResultView` の型の名前と置き場所、`launch.ts` の `runPopupSend`・`logSend` の行番号。

**古くなるコメントと文書**

`make lint`・`make test` では検出できないので、各フェーズの削除と同じステップで改め、§4.4 の横断検索で確かめる。

-   `internal/server`: `server.go:1`（「localhost HTTP server」）・`:28`（「loopback client」）・`:39-41`（`ErrInvalidOptions` の「zero token, a zero port」）・`:66`（「Listen and Serve」）・`:77`（「authenticated requests」）、`job.go:16`（「authenticated requests」）、`request.go:28`（「already-authenticated request」）、`test_helpers.go:246`（`MaxInFlightForTest` の「authenticated」）。
-   `cmd/yt2column-server`: `main.go:1`（「localhost server」）・`:21-25`（`SIGPIPE` の理由。設計書 3.3.2）、`run.go:26-30`（終了コードのコメントの「the token file」）・`:128-134`（`run` のコメント）。
-   拡張: Chrome 103 の理由に `AbortSignal.timeout` を挙げるコメント（HEAD `1f80866` では `background.ts:48-51`。0027 で `background.ts` は削除されるので、0027 の後に別の場所に残っていれば改める）、`chromeDeps.ts` の `createSendDeps` のコメント（HEAD `1f80866` では `:275`）、`eslint.config.js:21-25`（`storage.sync` を禁じる理由にトークンを挙げる。ステップ 3-13 で規則ごと置き換える）、`test/manifest.test.ts:94` のテストの名前。`chromeDeps.ts:15` の `StorageArea` のコメントは、ステップ 3-7 で型ごと削除する。

### 1.4. 申し送りの項目への対応

-   [implementation_handoff.md](implementation_handoff.md) の I-01（UDS のテストの補助の具体）は、§1.3「UDS のテストの補助」に方針を記録し、フェーズ 1 のステップ 1-2 で扱う。
-   `design_handoff.md` の H-01〜H-10 は、すべて設計書 3.11 に採った方針が記録されている。本計画はその方針に従う。H-01〜H-04・H-10 はフェーズ 1（ステップ 1-3・1-6・1-11・1-14）、H-06 はフェーズ 1（ステップ 1-8）とフェーズ 2（ステップ 2-2）、H-05・H-08 はフェーズ 2（ステップ 2-2）、H-09 はフェーズ 2（ステップ 2-2 の応答の形）とフェーズ 3（それを拡張が検査するステップ 3-2）、H-07 はフェーズ 2（ステップ 2-12 の TCC の案内）とフェーズ 3（ステップ 3-2 の lastError の扱い、ステップ 3-14 の手動の確認）で扱う。

## 2. 実装ステップ (Implementation Steps)

各ステップの末尾に **対象:**（そのステップが触るファイル）と **完了:**（そのステップの検証）を記す。フェーズの「完了条件」は、そのフェーズのすべてのステップの後に確かめる。設計の詳細は設計書の節を参照し、本計画に書き写さない。

### フェーズ 1: サーバの UDS 化

他に依存しない `internal/serversock` とテストの補助 `internal/udstest` を先に作って単独で確かめ（設計書 8 章）、その後 `internal/config` → `internal/server` → `cmd/yt2column-server` → 文書とガードの順に進める。

-   [x] **ステップ 1-1**: `internal/cachelock` のパッケージのコメントとエラーの文言を、キャッシュのディレクトリに限らない言い方に改める（設計書 3.2.2。`cachelock.go:3-5`・`:33`・`:73`）。振る舞いは変えない。`LockedError` の文言（`:45`）は `yt-dlp` に触れたまま残す（`serversock` はこれを利用者に見せない）。
    -   **対象:** `internal/cachelock/cachelock.go`。**完了:** 既存の `go test -tags test ./internal/cachelock` が通る（テストはエラーを `errors.Is` で判定し、文言に依存しない）。
-   [x] **ステップ 1-2**: テスト専用のパッケージ `internal/udstest`（`//go:build test`）を作る。`os.MkdirTemp("/tmp", ...)` で実行のたびに異なる短いディレクトリを作り `t.Cleanup` で消す補助と、そこに UDS のリスナーを作り、作った時点で閉じる処理を `t.Cleanup` に登録する補助を置く（§1.3「UDS のテストの補助」）。補助自身のテストで、パスの長さの余裕（§1.3）と、2 回の呼び出しが別のディレクトリを返すことを確かめる。`internal/pipeline/pipeline_test.go` の `testOnlyPackageDirs` に `internal/udstest` を加え、`package_reference.md` に `internal/udstest` の行を加える（§1.3「UDS のテストの補助」）。
    -   **対象:** `internal/udstest/udstest.go`・`udstest_test.go`・`internal/pipeline/pipeline_test.go`・`docs/dev/developer_guide/package_reference.md`。**完了:** `go test -tags test ./internal/udstest` と `go test -tags test -run 'TestFakesCarryBuildTag|TestPackageReferenceListsPackages' ./internal/pipeline` が通る。
-   [x] **ステップ 1-3**: `internal/serversock`（`//go:build unix`）を設計書 3.2 のとおりに作る（`Acquire`・`Lock.Listen`・`Lock.Release`、`ErrAlreadyRunning`・`ErrInUse`・`ErrUnsafe`・`UnsafeError`・`UnsafeReason`）。検査の順序は設計書 3.2.1、ロックは 3.2.2、ソケットのパスの扱いは 3.2.3 の表に従う。所有者の検査は、`Lstat` の結果を受け取る非公開の関数に分け、他人の所有の場合をテストで作れるようにする（設計書 7.1）。`cachelock.ErrLocked` は `ErrAlreadyRunning` に読み替え、`LockedError` を `%w` で包まない。`package_reference.md` に `internal/serversock` の行を加え、`internal/cachelock` の行に起動のロックにも使うことを加える（`TestPackageReferenceListsPackages` は、パッケージを加えるのと同じ変更に行を求める。§1.3）。
    -   **対象:** `internal/serversock/serversock.go`・`docs/dev/developer_guide/package_reference.md`。**完了:** `go build ./internal/serversock` と `go test -tags test -run TestPackageReferenceListsPackages ./internal/pipeline` が通る。
-   [x] **ステップ 1-4**: `internal/serversock` のテストを書く（設計書 7.1 の `internal/serversock` の行）。ディレクトリ（作成と `0700`、`0700` でない・symlink・他人の所有で中止し権限を変えないこと、親にグループかその他の書き込みのビットがあるときの中止）、ロック（取られているときの `ErrAlreadyRunning`、`Release` の後の再取得）、ソケット（stale の削除と bind、接続できるソケットでの `ErrInUse` と削除しないこと、通常ファイル・symlink・FIFO・ディレクトリでの中止と削除しないこと、作成後の `0600`、リスナーを閉じるとソケットが消えること）。所有者と権限に依存するテストは、root では `t.Skip` で飛ばす（設計書 7.1。補助を `internal/serversock/test_helpers.go` に置く。§1.3）。一時ディレクトリには `internal/udstest` を使う。
    -   **対象:** `internal/serversock/serversock_test.go`・`test_helpers.go`。**完了:** `go test -tags test ./internal/serversock` が通る。
-   [x] **ステップ 1-5**: ステップ 1-1〜1-4 のテストを壊して失敗することを確かめ、コミットメッセージに記録する（§1.2）。取り違えやすいもの: `serversock` の各検査（ディレクトリの権限・所有者・symlink・親の書き込みのビット、ロックの取得、stale の判定の各行、作成後の `chmod`）を 1 つずつ外す、`SetUnlinkOnClose(false)` を加える（閉じるとソケットが消えるテスト）、`internal/udstest` のディレクトリの名前を固定にする（2 回の呼び出しが別のディレクトリを返すことのテスト）。`internal/serversock` の公開の関数について、§4.4 の `go tool cover -func` の確認を行う。
    -   **対象:** なし（確認のみ）。**完了:** 各対象で該当するテストが失敗することを確かめ、記録する。

### PR-1 作成ポイント: socket directory and start lock package

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `feat(0026): add the serversock package for the socket directory and start lock`

**レビュー観点**: ディレクトリとソケットの検査の順序（設計書 3.2.1・3.2.3）と、検査に失敗したときに権限を変えず何も削除しないこと / stale の判定で、接続できるソケットや通常ファイル・symlink・FIFO を削除しないこと / 起動のロックの取得と `Release` の後の再取得、`cachelock.ErrLocked` の読み替え / `internal/udstest` のディレクトリが macOS のパスの上限に収まり、並行の実行でぶつからないこと

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-3・1-4 は、トークンを削除した後にソケットへの接続を守る唯一の仕組み（ディレクトリの権限・所有者・symlink・stale の判定・起動のロック）を作るセキュリティの関門である（mkplan のパネル方式の条件「security-gate」）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）
-   [x] **ステップ 1-6**: `internal/config/socketpath.go` を作り、`HOME` だけからソケットのパスを導出する非公開の関数（`runtime.GOOS` の値を引数に取る）と OS ごとの上限の定数を置く（設計書 3.1）。`ServerConfig` から `port`・`tokenPath` と `Port()`・`TokenPath()` を削除し、`SocketPath()` を加え、`LoadServer` が導出を呼ぶようにする。設計書 3.1「削除するもの」の関数・定数を削除する（`tokenFileName`・`configDirOverride`・`defaultConfigDir`・`reasonConfigDirAbsent` を含む）。`LoadServer` のコメント（`config.go:268-273`）から、ポートとトークンのパスの記述を除く。
    -   **対象:** `internal/config/socketpath.go`・`config.go`・`cachedir.go`。**完了:** `go build ./internal/config` が通る。
-   [x] **ステップ 1-7**: `internal/config` のテストを改める。`socketpath_test.go` を作り、設計書 7.1 の `internal/config` の行のうち `LoadServer` に関わるもの（OS ごとのパス、`windows` などで導出しないこと、`HOME` の未設定・空・相対、パスの長さの上限ちょうどと 1 バイト超、`XDG_CONFIG_HOME`・`XDG_STATE_HOME`・`XDG_RUNTIME_DIR`・`TMPDIR` を設定しても結果が変わらないこと、エラーが `HOME` を名指しし、その値を含まないこと）を確かめる。§1.3 の 4 つのテストを削除し、`TestLoadServerValid` からポートとトークンのパスの確認を除いてソケットのパスの確認に替え、`TestLoadServerTokenPathUnderivable` を `TestLoadServerSocketPathUnderivable` に改め、補助 `wantTokenPath`（`config_test.go:867`）を削除する。`test_helpers.go` の `ServerPortEnvForTest`・`TokenFileNameForTest` を削除し、ソケットのパスの末尾（`HOME` を除く部分）を OS ごとに返すアクセサを加える（文書のガードが使う。設計書 3.9）。`test_helpers.go` に OS ごとの上限の定数を返すアクセサも加え、`internal/serversock/serversock_test.go`（`//go:build unix`）に、`internal/config` の上限の定数が `len(syscall.RawSockaddrUnix{}.Path) - 1` と一致することの確認を加える（定数はステップ 1-6 で作る。設計書 3.1）。
    -   **対象:** `internal/config/socketpath_test.go`・`config_test.go`・`cachedir_test.go`・`test_helpers.go`・`internal/serversock/serversock_test.go`。**完了:** `go test -tags test ./internal/config ./internal/serversock` が通る。
-   [x] **ステップ 1-8**: `internal/server` を設計書 3.3.1 のとおりに改める。`Listen`・`Options.Port`・`Options.Token` とその検査、`hostAllowed`・`checkAuthorization`・`Content-Type` と `Origin` の検査、4 つの定数、4 つの拒否の理由とその詳細、`WWW-Authenticate` の付与、ログの `remoteAddr` を削除する。`Serve` は 1 つのリスナーを受け取る（§1.3 の名前の変更の一覧）。§1.3「古くなるコメントと文書」の `internal/server` の行を改める。
    -   **対象:** `internal/server/server.go`・`handler.go`・`response.go`・`job.go`・`request.go`。**完了:** `go build ./internal/server` が通る。
-   [x] **ステップ 1-9**: `internal/server` のテストを改める。設計書 3.8 の表の `internal/server` の行に加えて、§1.3 の「設計書 3.8 の表にない」行を改める。`test_helpers.go` からトークン・オリジン・`Content-Type` の補助と定数を削除し、`newRequest` をパスとメソッドだけの形にする。ヘッダを付ける補助 `withValidHeaders` は付けるヘッダがなくなるので削除する。`TestListenBindsLoopbackOnly` を `TestServeOnUnixListener`（`Serve` が UDS のリスナーで受理の応答を返すこと）に改める。新しく次を加える。
    -   `TestExpectContinueOversizedBodyNotSent`（AC-09）: UDS の上で、`Content-Length` が 4 MiB を超え `Expect: 100-continue` を付けたリクエストに `413` が返り、クライアントが本文を送らないこと（設計書 7.1）。クライアントの `http.Transport` の `ExpectContinueTimeout` は 0 でない値にする（0 では本文をすぐに送る。`go doc net/http.Transport` の `ExpectContinueTimeout` の説明）。本文は読まれたバイト数を記録する reader で渡し、0 バイトであることを確かめる。
    -   `TestUnparsableRequestBypassesHandler`（AC-12）: HTTP として読めないバイト列に `net/http` が `400` を返し、枠を取らず、通知も生成も起きないこと（設計書 3.4「フレームの不一致」）。
    -   `TestRejectReasonSet`（AC-12）: `RejectReasonsForTest()` の値の集合がちょうど設計書 3.3.1 の表の 8 つであること。
    -   `TestAcceptsWithoutBrowserHeaders`（AC-12）: 任意の `Host` で、`Origin`・`Content-Type`・`Authorization` のないリクエストが `202` で受理されること（削除した検査が残っていないこと）。
    -   **対象:** `internal/server/*_test.go`・`test_helpers.go`。**完了:** `go test -tags test ./internal/server` が通る。
-   [x] **ステップ 1-10**: `internal/authtoken` のディレクトリを削除し、`package_reference.md` からその行を削除する。
    -   **対象:** `internal/authtoken/`・`docs/dev/developer_guide/package_reference.md`。**完了:** `go build ./...` が通る（ステップ 1-11 の後）。
-   [x] **ステップ 1-11**: `cmd/yt2column-server` を設計書 3.3.2 のとおりに改める（手順の表、終了コード、標準出力の削除、起動のログの `socket`、伏せ字の対象からのトークンの除去、`Serve` の後の `Lock.Release`）。`serversock` のエラーごとの 1 行の案内は設計書 4.2 に従う。`listenURLs` を削除する。§1.3「古くなるコメントと文書」の `cmd/yt2column-server` の行を改める。
    -   **対象:** `cmd/yt2column-server/run.go`・`main.go`。**完了:** `go build ./cmd/yt2column-server` が通る。
-   [x] **ステップ 1-12**: `cmd/yt2column-server` のテストを改める。設計書 3.8 の表の `cmd/yt2column-server` の行に従い、`test_helpers.go` を「`HOME` を `internal/udstest` の短いディレクトリにし、ソケットのパスへの接続で待ち受けを確かめる」形にし、`postArticle`・`mustPostArticle` を UDS のクライアントにし、2 つの再試行のループを削除する（§1.3）。§1.3 の 3 つのテストを削除する。`docs_test.go` からは、コンパイルを通すための削除（`serverDefaultPort` と、削除した `config`・`server` の補助の呼び出し）だけを行い、文書の内容の確認はステップ 1-15 で書き換える（§1.3）。次を確かめる行を加える・改める。
    -   `TestStartupFailures`（AC-04）: `HOME` なし・パスの長さの超過で `2`、ソケットのディレクトリの権限の不備・既存の待ち受けで `1`、どれもソケットを作らないこと。
    -   `TestStartDuringShutdownRefused`（H-03・AC-06）: シャットダウン中（ジョブを待っている間）の 2 つ目の起動が `1` で終わること。2 つ目の起動には期限のあるコンテキストを渡し、誤って起動してしまう場合にもテストが止まらずに失敗するようにする。
    -   `TestRunLogsListeningAndStop`（AC-04）: 起動のログにソケットのパスがあること。
    -   `TestSignalWaitsForJob`（AC-06）: ジョブを待ってから終了し、終了後にソケットのパスがないこと。
    -   `TestSignalStopsServer`（AC-05）: 本番の `main` の子プロセスの標準出力が空であること。
    -   `TestOutputsOmitSecrets`・`TestServerEndToEnd`（AC-11）: UDS でトークンなしの形に書き直す。
    -   **対象:** `cmd/yt2column-server/*_test.go`・`test_helpers.go`。**完了:** `go test -tags test -skip TestServerDocsContract ./cmd/yt2column-server` が通る（`-skip` は `go help testflag` で確かめた。`TestServerDocsContract` はステップ 1-15 の後に通る）。
-   [x] **ステップ 1-13**: 拡張から `forbidden_host`・`unsupported_media_type`・`forbidden_origin`・`unauthorized` を削除する（`ServerRejectReason`・`serverRejectStatus`・`serverRejectMessage`）。`not_found`・`method_not_allowed` はフェーズ 3 まで残す。テストのうち、4 つの理由そのものを確かめる行は削除し、4 つの理由を入力に使って別の振る舞い（状態コードと理由の組の検査、拒否の表示）を確かめる行（`response.test.ts:213`・`:258`、`sendMessages.test.ts:91-96`、`renderSend.test.ts:44` など）は、残る理由に入れ替える（§1.2。§1.3「フェーズごとの文書とガードの割り当て」）。
    -   **対象:** `extension/src/core/response.ts`・`extension/src/core/sendMessages.ts`・`extension/test/response.test.ts`・`sendMessages.test.ts`・`renderSend.test.ts`。**完了:** `make ext-typecheck` と `make ext-test` の該当するテストが通る（`serverReasons.test.ts` は README の表を改めるステップ 1-14 の後）。
-   [x] **ステップ 1-14**: フェーズ 1 の文書を改める（§1.3「フェーズごとの文書とガードの割り当て」のフェーズ 1 の列）。README の `curl` の例は、`curl --unix-socket "<パス>" ... http://localhost/articles` の形でパスを二重引用符で囲む（設計書 3.9、H-04）。security.md §8.2 の見出しは変えない。残余リスクは設計書 5.4 のうちサーバに関わるものを書く。README の「## Development」に、開発者向けに独立したサーバを動かす手順の節を加える（§1.3「開発者向けの独立したサーバの手順」）。
    -   **対象:** `README.md`・`docs/dev/security.md`・`docs/dev/project_overview.md`・`docs/dev/developer_guide/package_reference.md`・`CLAUDE.md`・`docs/translation_glossary.md`。**完了:** ビルドした `build/yt2column-server` を、`HOME` を `/tmp` の下の短いディレクトリ（macOS の `mktemp -d` が返す `/var/folders/...` では、ソケットのパスが上限を超える）にし、偽の API キーと偽の Webhook URL（`config.LoadServer` が受理する形の値）で起動する。書いた `curl` の例の形で、パスかメソッドを誤らせたリクエストを送り、`not_found`・`method_not_allowed` の固定の応答を受け取ることを確かめる。拒否の応答は記事の生成も通知もしないので、外部のサービスに届かない。受理の例は実行しない（CLAUDE.md「Tool Execution Safety」）。「## Development」の手順のとおりに、異なる 2 つの `HOME` で 2 つのサーバを同時に起動し、それぞれのソケットに同じ形のリクエストを送って、どちらも固定の拒否の応答を返すことも確かめる。実行したコマンドと結果をコミットメッセージに記録する。
-   [x] **ステップ 1-15**: フェーズ 1 のガードを改める・加える。
    -   `cmd/yt2column-server/docs_test.go` の `TestServerDocsContract` を、設計書 3.9 の表のうちフェーズ 1 に当たる確認に改める。
        -   `README.md`: 「## Localhost server」の節（「### Upgrading from the shared-token version」の節を除く）が、ソケットのパスの末尾（ステップ 1-7 のアクセサから OS ごとに導出）、`--unix-socket` と `/articles` を含む `curl` の例、終了コードの表、トークンが不要であることを述べる文、サーバが起動していない場合のトラブルシューティングを含むこと。同じ範囲が `YT2COLUMN_SERVER_PORT`・`8765`・`Authorization: Bearer`・`options page` を含まないこと（「port」のような部分文字列は `Application Support` などに現れるので、禁止する語は語句で定める）。
        -   `README.md` の「## Development」: 開発者向けの独立したサーバの手順の節が、`/tmp` の下の短いディレクトリを `HOME` にしてサーバを起動する例と、そのソケットのパスに送る `curl --unix-socket` の例を含むこと。
        -   `project_overview.md`: `portEnv` の確認を外す（`yt2column-nmhost` の確認はフェーズ 2 で加える）。
        -   `security.md`: §8 の冒頭が、相手ごとの排除の根拠（他ユーザー・ウェブページ・サンドボックス化された Agent）と、UDS の権限が同じ uid のプロセスを区別しないことと、同一ユーザーを信頼することを含むこと。§8.2 が `0700`・`0600`・起動のロック・stale・枠の数・本文の上限・残余リスク（ソケットの `unlink` と再 bind）を含み、トークンのファイルと表示の語句（`server-token`・`O_NOFOLLOW` のトークンの項目など、現在の確認の語句）を含まないこと。語句は実装で決め、ここに挙げた観点ごとに 1 つ以上確かめる。
        -   `CLAUDE.md`: `internal/authtoken` の代わりに `internal/serversock`。`translation_glossary.md`: 「共有トークン」の行の代わりに起動のロックの行。
    -   `cmd/yt2column/extension_docs_test.go:197-204` のサブテストを削除する。
    -   リポジトリ全体を読むガードと depguard の規則は、ステップ 1-17 で加える。
    -   **対象:** `cmd/yt2column-server/docs_test.go`・`cmd/yt2column/extension_docs_test.go`。**完了:** `make test`・`make lint` が通る。
-   [x] **ステップ 1-16**: 壊して失敗することを確かめ、コミットメッセージに記録する（§1.2。§5 のフェーズ 1 のテストとガードのうち、ステップ 1-6〜1-15 が作る・改めるものが対象）。取り違えやすいもの: `SetUnlinkOnClose(false)` を加える（`TestSignalWaitsForJob`）、上限の定数を 1 つずらす（ステップ 1-7 で加えた `serversock_test.go` の定数の一致の確認）、ソケットのパスの長さの上限を 1 つずらす（`socketpath_test.go` と `TestStartupFailures`）、`XDG_STATE_HOME` を読むようにする、`run` の `Release` を `Serve` の前に移す（`TestStartDuringShutdownRefused`）、`readRequest` が `Content-Length` の検査の前に本文を読むようにする（`TestExpectContinueOversizedBodyNotSent`）、`http.Server` の `ConnState` の処理で新しい接続ごとに枠を取るか通知する（`TestUnparsableRequestBypassesHandler`）、`hostAllowed` と `Origin` の検査を 1 つずつ戻す（`TestAcceptsWithoutBrowserHeaders`）、`forbidden_host` の理由を戻す（`TestRejectReasonSet`）、起動のログからソケットのパスを外す（`TestRunLogsListeningAndStop`）、`run` から標準出力に 1 行書く（`TestSignalStopsServer`）、README の節に `8765` を戻す・「## Development」の独立したサーバの手順の節から `--unix-socket` の例を外す（`TestServerDocsContract`）。§1.2 の `go tool cover -func` の比較と、§4.4 の `go tool cover -func` の確認（`internal/config` のソケットのパスの導出）を行う。§4.4 の横断検索を行い、検索した語と結果をコミットメッセージに記録する。
    -   **対象:** なし（確認のみ）。**完了:** 各対象で該当するテストか lint が失敗することを確かめ、記録する。

### PR-2 作成ポイント: Unix-socket-only server

**前提**: 0027 のすべての PR がマージされている（§1.3「0027 との順序」）。ステップ 1-6 を始める前に確かめる。

**対象ステップ**: 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 / 1-12 / 1-13 / 1-14 / 1-15 / 1-16

**推奨タイトル**: `feat(0026): serve only on a Unix socket and remove the shared token`

**レビュー観点**: `internal/config` が `HOME` だけからソケットのパスを導出し、長さの上限で拒否し、エラーに値を含めないこと / TCP のリスナー・トークン・ポートの削除と UDS の待ち受けの追加が、コードと文書で食い違わないこと / `Expect: 100-continue` で上限を超える本文を受け取らないこと、HTTP として読めない入力が枠を取らないこと / 起動の手順と終了コード、`Serve` の後の `Lock.Release` によるシャットダウン中の起動の拒否、README の拒否の理由の表と拡張の 4 つの理由の一致

**実装モデル要件**: frontier-required

**判定理由**: TCP とトークンの削除と UDS の追加を同時に行う移行の段階で、多くのテストと文書のガードを書き換える（mkplan のパネル方式の条件「振る舞いの追加と削除を同時に行う移行」）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した（#366）
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）
-   [x] **ステップ 1-17**: リポジトリ全体を読むガードと depguard の規則を加え、壊して失敗することを確かめる。`internal/pipeline/relay_guard_test.go` に `TestNoTCPListenerInProduction` と `TestSharedTokenRemoved` を作る（§1.3「本計画で置く静的な検査の場所」）。`.golangci.yml` に、`internal/server` が `internal/serversock` を import しない depguard の規則を加える。壊す対象: `server.go` に TCP の `net.Listen` を戻す・`hostAllowed` の loopback の文字列の一覧を戻す（`TestNoTCPListenerInProduction`）、README と本番の Go のファイルに `YT2COLUMN_SERVER_PORT` を戻す（`TestSharedTokenRemoved`）、`internal/server` に `internal/serversock` の import を加える（`make lint`）。結果をコミットメッセージに記録する（§1.2）。
    -   **対象:** `internal/pipeline/relay_guard_test.go`・`.golangci.yml`。**完了:** `make test`・`make lint` が通り、各対象で該当するガードか lint が失敗することを確かめた。

**完了条件:** `make fmt` → `make test` → `make lint` → `make ext-check` が通る。`go test -run '^$' -tags test,integration ./...` でテスト専用のファイルがすべてコンパイルされる。

### PR-3 作成ポイント: repository guards for the socket-only server

**対象ステップ**: 1-17

**推奨タイトル**: `test(0026): guard against TCP listeners and the shared token`

**レビュー観点**: 本番のファイルの判定（`_test.go` と `test` タグのときだけビルドされるファイルを除く）が `isTestCode` と同じであること / TCP のリスナーの呼び出しと loopback の文字列の検出の範囲 / `TestSharedTokenRemoved` の禁止する語と、「### Upgrading from the shared-token version」の節の例外

**実装モデル要件**: standard

**判定理由**: 該当する条件はない。PR-2 の後のコードと文書に対して、既に満たされている性質を確かめる静的なガードと depguard の規則を加えるだけで、振る舞いは変えない。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した（#369）
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: ホスト

-   [x] **ステップ 2-1**: `internal/nmhost/frame.go` に、Native Messaging の長さの読み取り（`encoding/binary.NativeEndian`）とフレームの書き込み（長さと JSON を 1 回の `Write`、1 MiB を超えないことの確認）を置く（設計書 3.5.2・3.5.4・3.5.6）。テスト: 長さの読み取り、長さが読めない場合、フレームの形、1 MiB を超えるフレームを書かないこと。`package_reference.md` に `internal/nmhost` の行を加える（§1.3）。
    -   **対象:** `internal/nmhost/frame.go`・`frame_test.go`・`docs/dev/developer_guide/package_reference.md`。**完了:** `go test -tags test ./internal/nmhost` と `go test -tags test -run TestPackageReferenceListsPackages ./internal/pipeline` が通る。
-   [x] **ステップ 2-2**: `internal/nmhost/relay.go` に `Dialer`・`RelayTimeout`・`RelayOptions`・`Relay`・`HostError` と 2 つの応答の構造体を置く（設計書 3.5.2〜3.5.5）。`http.Transport` の設定は設計書 3.5.2 の手順 4 のとおりにし、`DialContext` は手順 3 の接続を 1 回だけ返す。ホストのエラーは段階で選ぶ（設計書 3.5.3 の表）。ログは標準エラー出力に `log/slog` のテキスト形式で、設計書 3.5.6 の項目だけを書く。
    -   **対象:** `internal/nmhost/relay.go`。**完了:** `go build ./internal/nmhost` が通る。
-   [x] **ステップ 2-3**: `internal/nmhost` のフレームと中継のテストを書く（設計書 7.1 の `internal/nmhost` の行のうち、manifest を除くもの）。テストの名前と主な確認は次のとおりである。中継のテストは、`Dial` の fake と、`internal/udstest` の UDS の上の `httptest` 風のサーバ（固定の状態コードと本文を返すもの、応答しないもの、途中で切るもの、HTTP として読めないバイト列を返すもの）を使う。
    -   `TestRelayForwardsOneRequest`（AC-15）: 受け取ったバイト列が変わらずに本文として届き、リクエストの行が `POST /articles` で `Content-Length` と `Expect: 100-continue` を持ち、応答のフレームが 1 つだけ書かれること。
    -   `TestRelayLengthChecks`（AC-16）: 上限を超える長さと長さ 0 で、どちらも `Dial` を呼ばず、stdin の本文を読まないこと（stdin の fake が読まれたバイト数を記録する）。上限ちょうどでは接続すること。計画の `TestRelayMessageTooLarge`・`TestRelayZeroLength` は、準備と確認が同じなので 1 つの表にまとめた（実装で変えた）。
    -   `TestRelayServerRejectsWithoutBody`（AC-16）: サーバが本文を読まずに `413` を返す場合に、本文を送らず、stdin の本文を読まずに、その応答を包むこと。
    -   `TestRelayResponseOverLimit`（AC-16）: 64 KiB を超える応答の本文で `protocol_error` になり、途中までを転送しないこと。
    -   `TestRelayHostErrors`（AC-19）: 設計書 3.5.3 の表の各行（接続できない場合はソケットなし・stale・権限のないディレクトリ、応答しないサーバでの `server_timeout`、途中の切断・HTTP として読めない応答・stdin が長さに満たない場合の `protocol_error`、本文の途中で止まる応答と接続の期限のエラーでの `server_timeout`）。返る `HostError` の値と、接続できない場合のログの errno の説明を確かめる。接続できない場合は、stdin の本文が読まれないことも確かめる（設計書 7.1「残りを読まない」）。時間の上限と書き込みの失敗が重なる場合は、`http.Transport` がどちらのエラーを返すかが競合で決まり、この表の行では優先の判定だけが結果を決める入力にできないので、`TestExchangeFailurePrecedence` で確かめる（実装で変えた）。
    -   `TestRelayForwardsOneRequest`・`TestRelayHostErrors` などで作る接続とサーバは、作った時点で `t.Cleanup` に閉じる処理を登録する。
    -   `TestOneUseTransportRefusesRedial`（AC-19。計画の `TestRelayDialsOnce`）: 中継の `Transport` の `DialContext` が、1 回目に手順 3 の接続を返し、2 回目は失敗すること。`Dial` が 1 回しか呼ばれないことは、`TestRelayForwardsOneRequest`・`TestRelayHostErrors` のすべての行で、呼び出しを数える `Dial` の包みで確かめる（実装で、この 2 つに分けた）。
    -   プロキシの環境変数（AC-18。計画の `TestRelayIgnoresProxyEnv`）: `HTTP_PROXY` を設定してもリクエストの行が `POST /articles` のままであること。`net/http` はプロキシの環境変数をプロセスで 1 回だけ読むので、`TestMain` がどのテストよりも前に `HTTP_PROXY` などを設定し `NO_PROXY` を消す。すると `TestRelayForwardsOneRequest` のリクエストの行の確認がそのままこの確認になるので、別のテストは置かない（実装で変えた）。
    -   `TestExchangeFailurePrecedence`（AC-19）: 中継の期限が過ぎていれば、期限切れではないエラーでも `server_timeout` になること。`http.Transport` は、期限と書き込みの失敗が同時に起きると書き込みのエラーを返すことがあり、`TestRelayHostErrors` の該当する行は結果がどちらの経路で決まるかを選べないので、優先の判定だけが結果を決める入力をこの表で与える（実装で加えた）。
    -   `TestRelayNoReplyFrame`（AC-15）: 長さが読めない場合、標準出力への書き込みが失敗した場合、`RelayOptions` が欠けている場合にだけ `Relay` がエラーを返し、標準出力に何も書かないこと（実装で加えた）。
    -   `TestReplyShapes`（AC-16・AC-27）: `HostError` の各値の文字列と、ゼロ値と範囲外の値が `protocol_error` になること、1 MiB を超える応答が `protocol_error` の応答になること。2 つの形のメンバの集合と base64 の本文は、中継のテストの確認の補助（`requireResponse`・`requireError`）がメンバの全体を比べて確かめ、無効な UTF-8 の本文は `TestRelayForwardsOneRequest` のサーバの応答で確かめる（実装で変えた）。
    -   `TestRelayOutputs`（AC-17）: 標準出力が 1 フレームだけで、本文とサーバの応答に入れた目印が標準エラー出力に現れないこと。
    -   所有者と権限に依存するテストは root では `t.Skip` で飛ばす（設計書 7.1。§1.3）。
    -   **対象:** `internal/nmhost/relay_test.go`・必要なら `internal/nmhost/test_helpers.go`（`//go:build test`。非公開の関数を使う補助だけ）。**完了:** `go test -tags test ./internal/nmhost` が通る。
-   [x] **ステップ 2-4**: `internal/nmhost` のガードを加える。ステップ 1-17 で作った `internal/pipeline/relay_guard_test.go` に、同じ本番のファイルの判定を使って `TestHostStdoutOnlyInMain`・`TestHostSpawnsNoProcess` を加え、この時点では `internal/nmhost` を走査する（`cmd/yt2column-nmhost` はステップ 2-13 で走査の対象に加える）。`.golangci.yml` に、`internal/nmhost` が `internal/server`・`internal/config`・`os/exec` を import しない depguard の規則を加える（§1.3）。2 つのガードが使う参照の検出（別名の import とドットの import を含む）は、`TestSelectorDetection` の表で確かめる（実装で加えた。既存の `TestTCPListenerDetection` と同じ流儀）。
    -   **対象:** `internal/pipeline/relay_guard_test.go`・`.golangci.yml`。**完了:** `make test`・`make lint` が通る。
-   [x] **ステップ 2-5**: ステップ 2-1〜2-4 のテストとガードを壊して失敗することを確かめ、コミットメッセージに記録する（§1.2）。取り違えやすいもの: 本文のバイト列を 1 バイト変える・`Content-Length` を付けない（`TestRelayForwardsOneRequest`）、長さの上限の判定を外す・長さ 0 の判定を外す（`TestRelayLengthChecks`）、`DialContext` が 2 回目も接続を返すようにする（`TestOneUseTransportRefusesRedial`）、`Proxy` を `http.ProxyFromEnvironment` にする（`TestRelayForwardsOneRequest`）、応答の上限の判定を外す（`TestRelayResponseOverLimit`）、時間の上限の優先を外す（`TestRelayHostErrors` の該当する行）、接続の段階の判定をエラーの値で行う形に変える（同）、本文を JSON の文字列として埋め込む（`TestRelayForwardsOneRequest`）、`HostError` のゼロ値を `server_unavailable` にする（同）、ホストのリクエストから `Expect: 100-continue` を外すか `ExpectContinueTimeout` を 0 にする（`TestRelayServerRejectsWithoutBody`）、ログに本文を書く（`TestRelayOutputs`）、`internal/nmhost` に `fmt.Println` を加える（`TestHostStdoutOnlyInMain`）、`os.StartProcess` の呼び出しを加える（`TestHostSpawnsNoProcess`）、`os/exec` の import を加える（`make lint`）。`internal/nmhost` の中継の公開の関数について、§4.4 の `go tool cover -func` の確認を行う。
    -   **対象:** なし（確認のみ）。**完了:** 各対象で該当するテストか lint が失敗することを確かめ、記録する。

### PR-4 作成ポイント: native messaging relay library

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5

**推奨タイトル**: `feat(0026): add the nmhost frame and relay library`

**レビュー観点**: 接続の段階によるホストのエラーの選択と、時間の上限と書き込みの失敗が重なる場合の優先（設計書 3.5.3） / 長さの上限の超過と長さ 0 で、接続も stdin の本文の読み取りもしないこと、`Expect: 100-continue` で拒否を受けたとき本文を送らないこと / 標準出力が 1 フレームだけで、本文とサーバの応答がログに出ないこと（`TestHostStdoutOnlyInMain`） / depguard と `TestHostSpawnsNoProcess` で、`internal/nmhost` がサーバ・設定・プロセスの起動に依存しないこと

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-2 の中継は、メッセージと応答の大きさの上限と標準出力の純度という、ブラウザとサーバの間のホストの関門を作る（mkplan のパネル方式の条件「security-gate」）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）
-   [x] **ステップ 2-6**: `internal/nmhost/manifest.go` に `HostName`・`ExtensionOrigin`・`Browser`・`ParseBrowser`・4 つのエラー・`Install`・`Uninstall` を置く（設計書 3.6）。OS とブラウザの組から 2 つのディレクトリを選ぶ処理は `runtime.GOOS` の値を受け取る非公開の関数にする。manifest の読み取りは `internal/strictjson` を使う。
    -   **対象:** `internal/nmhost/manifest.go`。**完了:** `go build ./internal/nmhost` が通る。
-   [x] **ステップ 2-7**: manifest のテストを書く（設計書 7.1 の `internal/nmhost` の manifest の行）。
    -   `TestInstall`・`TestUninstall`・`TestManifestLocation`（AC-20）: 設計書 7.1 の manifest の行（中身、`0600`、一時ファイルが残らないこと、ブラウザのディレクトリがないときに何も作らないこと、macOS の Brave の置き場所と存在の判定、他人の manifest を上書きも削除もしないこと、自分の manifest の置き換えと削除、`path` が相対と一時ディレクトリの下のときの拒否、ブラウザ名の完全一致）。
    -   `TestParseBrowser`（AC-20）: ブラウザ名の完全一致（大文字・前後の空白・別名を拒否する）。計画では上の 3 つに含めていたが、置き場所と無関係なので分けた（実装で加えた）。
    -   所有者と権限に依存するテストは root では `t.Skip` で飛ばす（設計書 7.1。§1.3）。
    -   **対象:** `internal/nmhost/manifest_test.go`。**完了:** `go test -tags test ./internal/nmhost` が通る。
-   [x] **ステップ 2-8**: ステップ 2-6・2-7 のテストを壊して失敗することを確かめ、コミットメッセージに記録する（§1.2）。取り違えやすいもの: manifest の判定の各条件（他人の manifest、`path` が相対か一時ディレクトリの下、ブラウザ名の完全一致、ブラウザのディレクトリの存在）と macOS の Brave の置き場所を 1 つずつ外す、一時ファイルを経ずに直接書く（一時ファイルが残らないことのテスト）。`Install`・`Uninstall` について、§4.4 の `go tool cover -func` の確認を行う。
    -   **対象:** なし（確認のみ）。**完了:** 各対象で該当するテストが失敗することを確かめ、記録する。

### PR-5 作成ポイント: native messaging host manifest

**対象ステップ**: 2-6 / 2-7 / 2-8

**推奨タイトル**: `feat(0026): add the nmhost manifest install and uninstall`

**レビュー観点**: 他人の manifest を上書きも削除もしないこと、`path` が相対か一時ディレクトリの下のときの拒否 / OS とブラウザの組から選ぶディレクトリ（macOS の Brave の置き場所を含む）と、ブラウザのディレクトリがないときに何も作らないこと / manifest の中身（`allowed_origins`）、`0600`、一時ファイルが残らないこと

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-6・2-7 の `Install`・`Uninstall` は、ホストを起動できる拡張を決める manifest を書き、他人の manifest を守る関門である（mkplan のパネル方式の条件「security-gate」）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）
-   [x] **ステップ 2-9**: `internal/config` に `HostConfig`・`LoadHost`・`LoadHostHome` を加える（設計書 3.1）。`LoadHost` はステップ 1-6 の導出の関数を通る。テストを加える: `LoadHost` が `HOME` 以外を読まないこと（読んだ変数名を記録する `LookupFunc` で確かめる）、`LoadServer` と同じパスを返すこと、`LoadHostHome` がソケットのパスの長さに関わらず `HOME` を返し、未設定・空・相対で `VarError` を返すこと（設計書 7.1）。`internal/config/envaccess_test.go` の `allowedEnvRefs`（`:65-69`）に `cmd/yt2column-nmhost/main.go` の `value:os.LookupEnv` を 1 回だけ加え、`cmd/yt2column/main.go` と同じく観測した回数がちょうど 1 であることの確認（`:383-385` と同じ形）を加える。
    -   **対象:** `internal/config/config.go`（設計書 3.12）・`socketpath_test.go`・`envaccess_test.go`。**完了:** `go test -tags test -skip TestEnvAccessConfined ./internal/config` が通る（`allowedEnvRefs` は上限なので先に加えても失敗しないが、観測した回数の確認はステップ 2-10 で `main.go` ができた後に通る）。
-   [x] **ステップ 2-10**: `cmd/yt2column-nmhost/main.go` を作る（設計書 3.5.1 の起動の形、設計書 4.3 の終了コード）。中継では `config.LoadHost` の失敗を `server_unavailable` の応答にする（設計書 4.3 の注記）。実装では、`LoadHost` が失敗したとき、すぐに失敗する `Dialer` を `nmhost.Relay` に渡す。応答の書き込みを `Relay` の 1 か所に保つためである。長さの検査は接続の前に行われるので、上限を超える長さと長さ 0 は、それぞれ `message_too_large`・`protocol_error` になる（どちらも接続せず本文を読まない。実装で加えた）。install/uninstall は `os.Executable` と `filepath.EvalSymlinks` で得たパスを渡し、manifest のパスと macOS での共有を標準エラー出力に書く（設計書 3.6）。`main` は `os.LookupEnv`・`os.Stdin`・`os.Stdout`・`os.Stderr` と、実行ファイルのパスを得る関数（`os.Executable` と `filepath.EvalSymlinks`）を渡すだけにし、処理は `run` 相当の関数に置いてテストする。実行ファイルのパスを関数で受け取るのは、`go test` のバイナリが `os.TempDir()` の下で動き、そのままでは `Install` が `ErrUnstableExecutable` で拒否するためである（設計書 3.6）。テスト `cmd/yt2column-nmhost/main_test.go`:
    -   引数の各形と使い方の誤り（終了コード `2`、標準出力が空）、オリジンが一致しない起動で標準出力が空、長さが読めない場合に終了コード `1` で標準出力が空（設計書 4.3）。
    -   `LoadHost` の失敗（`HOME` を未設定か相対パスにする）で、`server_unavailable` の応答、終了コード `0`、stdin の本文が読まれないこと。
    -   install/uninstall が、テストが渡した一時ディレクトリの外のパスで manifest を書き、manifest のパスと macOS での共有の注記を標準エラー出力に書くこと（AC-20）。
    -   すべての役割（中継・install・uninstall・使い方の誤り）で、読んだ変数名を記録する `LookupFunc` を渡し、`HOME` 以外を読まないこと（AC-18）。
    -   `package_reference.md` に `cmd/yt2column-nmhost` の行を加える（§1.3）。
    -   **対象:** `cmd/yt2column-nmhost/main.go`・`run.go`・`main_test.go`・`docs/dev/developer_guide/package_reference.md`（`main.go` には `main` と実行ファイルのパスを得る関数だけを置き、役割の処理は `run.go` に置く。`TestHostStdoutOnlyInMain` が `main.go` だけを除いて走査するので、役割の処理を走査の対象にするため。実装で変えた）。**完了:** `go test -tags test ./cmd/yt2column-nmhost ./internal/config` と `go test -tags test -run TestPackageReferenceListsPackages ./internal/pipeline` が通る。
-   [x] **ステップ 2-11**: Makefile の `build` に `build/yt2column-nmhost` を加える（install/uninstall の目標は設けない）。`cmd/yt2column/makefile_test.go` に `TestMakeBuildBuildsEveryBinary` を加える（§1.3）。
    -   **対象:** `Makefile`・`cmd/yt2column/makefile_test.go`。**完了:** `make build` が 3 つのバイナリを作り、`go test -tags test -run TestMakeBuildBuildsEveryBinary ./cmd/yt2column` が通る。
-   [x] **ステップ 2-12**: フェーズ 2 の文書を改める（§1.3「フェーズごとの文書とガードの割り当て」のフェーズ 2 の列）。README のトラブルシューティングには、ホストのログを見る手順（ブラウザを一度終了し、ターミナルからブラウザの実行ファイルを直接起動する）を書く（設計書 3.5.6）。README の「## Development」の独立したサーバの手順の節に、ホストの制約を加える（§1.3「開発者向けの独立したサーバの手順」）。
    -   **対象:** `README.md`・`docs/dev/security.md`・`docs/dev/project_overview.md`・`docs/dev/developer_guide/package_reference.md`・`CLAUDE.md`・`docs/translation_glossary.md`。**完了:** README の install/uninstall の手順を、ステップ 2-11 でビルドしたホスト（`/tmp` の外の作業ディレクトリの `build/` に置いたもの）で `HOME` を `/tmp` の下の短いディレクトリ（Chrome・Brave のユーザーデータのディレクトリを作ったもの）にして macOS で実行し、書いたとおりの manifest のパスと標準エラー出力の注記になることを確かめ、実行したコマンドと結果をコミットメッセージに記録する。README の他の Unix の置き場所は、手で実行せず `TestManifestLocation` の値と照らして確かめる（要件書 2.3）。
-   [x] **ステップ 2-13**: フェーズ 2 のガードを改める・加える。`TestServerDocsContract` に、次の確認を加える（設計書 3.9 の表）。README の `yt2column-nmhost install`・`yt2column-nmhost uninstall`、ホストが見つからない場合のトラブルシューティング。`project_overview.md` の `yt2column-nmhost` と install/uninstall のコマンド。security.md §8.2 の `allowed_origins`・標準出力が通信路であること・ホストの残余リスク（ホストのバイナリと manifest の置き換え）。CLAUDE.md の `internal/nmhost`、`make build` が 3 つのバイナリを作ること、install/uninstall のコマンド。用語集のホスト・ホスト名の行。README の「## Development」の独立したサーバの手順の節が、拡張とホストからは届かないことと、manifest がブラウザごとに 1 つであることを述べること（語句は実装で決める）。`cmd/yt2column/extension_docs_test.go` に `TestNativeHostContract` のフェーズ 2 の部分を作る（§1.3）。`TestHostStdoutOnlyInMain`・`TestHostSpawnsNoProcess`（ステップ 2-4）の走査の対象に `cmd/yt2column-nmhost` を加える。`.golangci.yml` に `cmd/yt2column-nmhost` が `os/exec` を import しない depguard の規則を加える（§1.3）。
    -   **対象:** `cmd/yt2column-server/docs_test.go`・`cmd/yt2column/extension_docs_test.go`・`internal/pipeline/relay_guard_test.go`・`.golangci.yml`。**完了:** `make test`・`make lint` が通る。
-   [x] **ステップ 2-14**: ステップ 2-9〜2-13 のテストとガードを壊して失敗することを確かめ、コミットメッセージに記録する（§1.2）。取り違えやすいもの: オリジンの照合を外す・`LoadHost` の失敗を終了コード `1` にする・macOS での共有の注記を外す・`config.LoadServer` を呼ぶ（`main_test.go`）、`cmd/yt2column-nmhost` に `os.StartProcess` の呼び出しを加える（`TestHostSpawnsNoProcess`）、`cmd/yt2column-nmhost` に `main.go` 以外の本番のファイルを置いて `fmt.Println` を書く（`TestHostStdoutOnlyInMain`）、`cmd/yt2column-nmhost/main.go` に `os/exec` の import を加える（`make lint`）、`ExtensionOrigin` の 1 文字を変える（`TestNativeHostContract`）、`RelayTimeout` を 25 秒にする（同）、README の「## Development」の独立したサーバの手順の節からホストの制約の文を外す（`TestServerDocsContract`）、Makefile の `build` からホストを外す・`build` に変数を通して `$(NMHOST_BINARY) install chrome` のような行を加える（`TestMakeBuildBuildsEveryBinary`）。`cmd/yt2column-nmhost` と `config.LoadHost`・`LoadHostHome` について、§4.4 の `go tool cover -func` の確認を行う。§4.4 の横断検索を行い、検索した語と結果をコミットメッセージに記録する。
    -   **対象:** なし（確認のみ）。**完了:** 各対象で該当するテストか lint が失敗することを確かめ、記録する。

### PR-6 作成ポイント: host command, build, and docs

**対象ステップ**: 2-9 / 2-10 / 2-11 / 2-12 / 2-13 / 2-14

**推奨タイトル**: `feat(0026): add the yt2column-nmhost command and its install docs`

**レビュー観点**: 呼び出し元のオリジンの照合（`allowed_origins` の後の二つ目の防御。設計書 3.5.1）と、一致しないときに標準出力に何も書かないこと / `main` がすべての役割で `HOME` 以外の環境変数を読まないこと、`LoadHost` の失敗で stdin を読まずに `server_unavailable` を返すこと / `make build` が 3 つのバイナリを作り install を実行しないこと、README の install/uninstall の手順と実際の manifest のパスの一致

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-10 の呼び出し元のオリジンの照合は、ホストを起動した拡張を確かめるセキュリティの関門である（mkplan のパネル方式の条件「security-gate」。設計書 3.5.1）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）
-   [x] **ステップ 2-15**: `cmd/yt2column-server/relay_e2e_test.go`（`//go:build test`）を作る（設計書 7.2）。テストの中でホストを一時ディレクトリにビルドする（`GOOS`・`GOARCH` を実行中の値に、`GOFLAGS` を空にする）。`go` コマンドは `exec.LookPath` で探す（`make test` と CI はどちらも `go` から実行されるので `PATH` にある）。サーバは既存の `run` を `testDeps`・`loopbackDeps` で起動し、`HOME` は `internal/udstest` の短いディレクトリにする。`TestRelayEndToEnd` のサブテストで設計書 7.2 のシナリオ (1)〜(5) を確かめる。(1) では、受付応答を読み、ホストが終了したことを確かめてから fake の遅い LLM を解放し、記事が Webhook に届くことを確かめる。(5) は独立したサブテストにせず、ホストを起動する補助（`runHost`）が、(1)〜(4) のすべての実行で、終了コード `0` と標準出力がちょうど 1 つのフレームであることを確かめる（実装で変えた）。(3) は、サーバを起動して止めた後にホストを起動する。(1) の遅い LLM は、`run_test.go` の既存の `gatedLLM` を使う。
    -   `go build` の子プロセスには、親の環境を引き継いだうえで `GOOS`・`GOARCH`・`GOFLAGS` を上書きして渡す（`HOME`・`GOCACHE` のない最小の環境ではビルドが失敗する）。ホストの子プロセスには、ブラウザが起動する場合に近い最小の環境（`HOME` と、プロキシを無視することを確かめるための `HTTP_PROXY`）と、拡張のオリジンの引数を渡す（引数がなければ、設計書 3.5.1 のとおり標準出力に何も書かない）。
    -   ホストの子プロセスは、起動の時点で `t.Cleanup` に止めて待つ処理を登録し、`WaitDelay` を付ける（既存の `startServerChild` の `:643-646` と同じ形）。サーバの `run` は、`HOME` のディレクトリを消す前に止めて待つ。
    -   テストの結果の再利用（`go test` のキャッシュ）は、子プロセスの `go build` が読むソースを入力として数えない。テストは `cmd/yt2column-nmhost` と `internal/nmhost` の `.go` のファイルをすべて `os.Open` で開き、キャッシュの入力に含める（`go help test` の、テストが開いたファイルをキャッシュの判定に使うという説明）。
    -   **対象:** `cmd/yt2column-server/relay_e2e_test.go`（ビルドの `exec.Command` は `_test.go` の中に置く。§1.2）。**完了:** `go test -tags test -run TestRelayEndToEnd ./cmd/yt2column-server` が通る。
-   [x] **ステップ 2-16**: ステップ 2-15 の統合テストを壊して失敗することを確かめ、コミットメッセージに記録する（§1.2。統合テストは `-count=1` で実行する）。取り違えやすいもの: サーバの失敗の通知を外す（`TestRelayEndToEnd` の (2)）、ホストの上限を 4 MiB にする（同 (4)）、サーバのハンドラが `202` を書く前にジョブの完了を待つように変える（同 (1)）。
    -   **対象:** なし（確認のみ）。**完了:** 各対象で該当するテストが失敗することを確かめ、記録する。

**完了条件:** `make fmt` → `make test` → `make lint` → `make ext-check` → `make build` が通る。`go test -run '^$' -tags test,integration ./...` が通る。

### PR-7 作成ポイント: relay end-to-end test

**対象ステップ**: 2-15 / 2-16

**推奨タイトル**: `test(0026): add the server and host relay end-to-end test`

**レビュー観点**: テストの中のホストのビルドの環境（`GOOS`・`GOARCH`・`GOFLAGS`）と、ホストのソースを `go test` のキャッシュの入力に含めること / ホストの子プロセスとサーバを、`HOME` のディレクトリを消す前に止めて待つこと / シナリオ (1)〜(5) が、受付応答の後の記事の配信・失敗の通知・上限を確かめていること

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-15 が、テストの中で `go build` し、実際のサーバとホストの子プロセスを動かす重い統合テストである（mkplan のパネル方式の条件「重い統合テスト」）。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 拡張

**前提:** 0025 と 0027 のすべての PR がマージされている（§1.3「0025 との順序」「0027 との順序」）。フェーズ 3 を始める前に、拡張のファイルの行番号（`launch.ts`・`chromeDeps.ts` に加え、0027 が変える `scripts/check-dist.ts`・`test/manifest.test.ts`・`test/checkDist.test.ts` などの、本計画と設計書が引く `file:line`）と、0025 が加えた `ArticleMetadata` の受け渡しと、0027 の後に残った名前（§1.3「0027 との順序」）を確かめ直す。

-   [x] **ステップ 3-1**: `extension/static/manifest.json` を設計書 3.7.1 のとおりに改め、`static/options.html`・`src/options.ts`・`src/optionsPage.ts`・`src/core/settings.ts` を削除する。`scripts/check-dist.ts:117-133` の `options_ui.page` の確認を削除する。`static/style.css` からオプションページだけが使う規則（`style.css:52-66`）を削除する。
    -   **対象:** `extension/static/manifest.json`・削除する 4 ファイル・`extension/scripts/check-dist.ts`・`extension/static/style.css`。**完了:** ステップ 3-7 の後に `make ext-build` が通る。
-   [x] **ステップ 3-2**: `extension/src/core/native.ts` を設計書 3.7.2・3.7.3 のとおりに作る（`nativeHostName`・`hostReplyVersion`・`hostReplyKinds`・`HostErrorValue`・`hostErrorValues`・`NativeReply`・`classifyNativeReply`）。検査の順序は設計書 3.7.3 の 1〜8 に従い、lastError の文字列を見ない（設計書 3.7.5）。base64 は復号して再び符号化した文字列が元と一致するものだけを受理する（`atob` で復号し `btoa` で符号化し直す。`src/core/` を検査する `tsconfig.core.json` は DOM のライブラリを読まないので、`types/core-encoding.d.ts` に `atob`・`btoa` の宣言を加え、`test/typecheck.test.ts` の `usesTextEncoding` に使用を加える）。`hasExactKeys`（`response.ts` の非公開の関数）を export して使う。
    -   **対象:** `extension/src/core/native.ts`・`extension/types/core-encoding.d.ts`・`extension/test/typecheck.test.ts`。**完了:** `make ext-typecheck` が通る（ステップ 3-3 の後）。
-   [x] **ステップ 3-3**: `extension/src/core/response.ts` を設計書 3.7.3・3.7.4 のとおりに改める（`not_found`・`method_not_allowed` の削除、`NotSentCause`・`UnexpectedCheck`・`SendOutcome`・`SendResult` の改定、`ReceivedResponse` と `SendOutcome` の `redirected` の削除、`toSendOutcome` が `unexpected` の `status` と `host-failed` の `lastError` を落とすこと、`classifyResponse` の戻り値の型）。
    -   **対象:** `extension/src/core/response.ts`。**完了:** `make ext-typecheck` が通る（ステップ 3-7 の後）。
-   [x] **ステップ 3-4**: `extension/src/core/prepare.ts` を設計書 3.7.2 のとおりに改める（設定の読み取りの削除、同期の関数、`ArticleBody.message` と `byteLength`、`PreflightReason` を `body-too-large` だけに、`PrepareOutcome` の `blocked` から `cause` を削除）。0025 が渡す `ArticleMetadata` の扱いは変えない。
    -   **対象:** `extension/src/core/prepare.ts`。**完了:** `make ext-typecheck` が通る（ステップ 3-7 の後）。
-   [x] **ステップ 3-5**: `extension/src/send.ts` を設計書 3.7.2 のとおりに改める（`SendDeps` の `sendNative`・`schedule`、`sendArticle` が `sendNative` を 1 回だけ呼び、25 秒の上限を `schedule` で設けること、`classifyNativeReply` で分類すること）。`readBody` などの `fetch` の処理を削除する。
    -   **対象:** `extension/src/send.ts`。**完了:** `make ext-typecheck` が通る（ステップ 3-7 の後）。
-   [x] **ステップ 3-6**: 表示を改める。`core/sendMessages.ts` の文言を設計書 3.7.4 の表のとおりにし、削除する文言（設計書 3.7.4）を削除する。`NotSentCause`・`UnexpectedCheck` の分岐も網羅的な `switch` で書く。`ui/renderSend.ts` から `unexpected` の状態コードの表示（`send-status`）を削除する（`static/style.css` の `.send-status` の規則も、ステップ 3-1 で削除した）。
    -   **対象:** `extension/src/core/sendMessages.ts`・`ui/renderSend.ts`。**完了:** `make ext-typecheck` が通る（ステップ 3-7 の後）。
-   [x] **ステップ 3-7**: 経路と配線を改める。`launch.ts` のポップアップの経路から `settings` の依存を削除し、`logSend` の `port` を `host: nativeHostName` にし、`unexpected` の状態コードと `host-failed` の lastError をログにだけ出す（設計書 3.7.6）。`browser/chromeDeps.ts` から `createSettingsStore` と `StorageArea` を削除し、`createSendDeps` を `chrome.runtime.sendNativeMessage` のコールバックの形と同期の例外の捕捉で実装し、`schedule` を `SendDeps` に渡す（0027 が `schedule` を削除していれば加え直す。§1.3「0027 との順序」。設計書 3.7.2）。`popup.ts` から `settings` を渡す配線を削除する。旧版の保存値を削除する処理は置かない（設計書 3.7.1）。§1.3「古くなるコメントと文書」の拡張の行を改める。
    -   **対象:** `extension/src/launch.ts`・`browser/chromeDeps.ts`・`popup.ts`。**完了:** `make ext-typecheck` と `make ext-build` が通る。
-   [x] **ステップ 3-8**: `extension/eslint.config.js` を設計書 7.3 のとおりに改める。`SharedToken`・`ServerPort` のブランドの規則と `settings.ts` の `creatorConfig`（`:192`）を削除する。`chrome.storage.sync` の規則は、ステップ 3-13 で `chrome.storage` の参照の禁止に置き換えるまで残す。接続先を限る新しい規則も、ステップ 3-13 で加える。
    -   **対象:** `extension/eslint.config.js`。**完了:** `make ext-lint` が通る。
-   [x] **ステップ 3-9**: 拡張のテストを改める。設計書 3.8 の表の拡張の行（`send.test.ts` の書き直し、`helpers/fetch.ts` の削除、`launch.test.ts`・`chromeDeps.test.ts`、`settings.test.ts`・`optionsPage.test.ts`・`helpers/settings.ts` の削除、`prepare.test.ts`、`checkDist.test.ts:135-157`、`lintRules.test.ts`、`manifest.test.ts`、`sendMessages.test.ts`・`response.test.ts`・`renderSend.test.ts`）に従う。新しく次を作る・加える。
    -   `extension/test/native.test.ts`（AC-26・AC-27）と `extension/test/helpers/native.ts`（`sendNative` の fake）: 設計書 6.3 のすべての葉、外側のメンバの未知・欠落・型の不一致、`v` の不一致、base64 の非正規形（余分なパディング、改行）、復号後の長さ超過、本文の既存の検査、lastError がどの文字列でも（`undefined` を含む）`host-failed` になること。
    -   `send.test.ts`（AC-24）: `sendNative` が 1 回だけ呼ばれ再送しないこと、`schedule` の上限が先なら `extension-timeout`、上限の後に届いた応答を使わないこと。
    -   `chromeDeps.test.ts`（AC-24）: `createSendDeps` が固定のホスト名で 1 回だけ呼ぶこと、lastError と同期の例外が `failed` になること。
    -   `launch.test.ts`（AC-24）: ポップアップの経路が、送信のボタンを押したときに本文を 1 回だけ送ること、ログにホスト名があり、トークンとポートがないこと。
    -   `sendMessages.test.ts`（AC-25・AC-26）: 送信の結果ごとの文言が互いに異なること、未送信の文言が Slack に触れないこと、結果不明の 3 つの文言が Slack の確認を含むこと、`host-failed` の文言が install の手順を含むこと、どの文言もオプションページ・トークン・ポートに触れないこと。
    -   `secretsAbsent.test.ts`（AC-23・AC-29）: トークン・ポートの設定を名指す語が `src/`・`static/` にないこと（例外はない。旧版の保存値のキーも宣言しない）。
    -   `lintRules.test.ts`: ステップ 3-8 で削除した規則の行を削除する（新しい規則の行はステップ 3-13 で加える）。
    -   `manifest.test.ts`（AC-22・AC-23）: 権限がちょうど `activeTab`・`scripting`・`nativeMessaging` の 3 つで（`storage` がない）、`host_permissions`・`options_ui` がなく、security.md §8.1 の「**要求する権限**」の一覧に `nativeMessaging` があること。`minimum_chrome_version` のテストの名前から `AbortSignal.timeout` を除く（§1.3）。
    -   **対象:** `extension/test/*`・`extension/test/helpers/*`。**完了:** `make ext-check` が通る（`serverReasons.test.ts` はステップ 3-10 の後）。
-   [x] **ステップ 3-10**: フェーズ 3 の文書を改める（§1.3「フェーズごとの文書とガードの割り当て」のフェーズ 3 の列）。README の拒否の理由の表に「拡張の表示」の列を加える（設計書 3.3.1 の表の列）。security.md §8.2 に、ホストを起動できるのは `nativeMessaging` の権限を持ち `allowed_origins` に載った固定の拡張だけであることを加える。
    -   **対象:** `README.md`・`docs/dev/security.md`・`CLAUDE.md`・`docs/translation_glossary.md`。**完了:** README の「拡張の表示」の列の各行が、ステップ 3-6 の `serverRejectMessage` と `classifyResponse` の扱い（固定の文言か想定外の応答か）と一致することを、`serverReasons.test.ts`（ステップ 3-11）で確かめる。
-   [x] **ステップ 3-11**: フェーズ 3 のガードを改める・加える。
    -   `extension/test/serverReasons.test.ts`: README の表のうち「拡張の表示」の列が固定の文言である行だけを `serverRejectStatus` と照合する（設計書 3.9）。
    -   `cmd/yt2column/extension_docs_test.go`: `TestExtensionSendDocumentation` の README のサブテストを設計書 3.9 の表のとおりに改め、トークンを貼り付ける手順（`shared token`・`Options` の語）が README の「### Browser extension」と security.md §8.1 に残っていないことの確認を加える。security.md §8 の冒頭は削除の理由として共有トークンに触れるので、この確認の対象にしない（§1.3 の `TestSharedTokenRemoved` の説明）。`TestNativeHostContract` に `native.ts` の値の照合を加える（Go の側の値は、`internal/nmhost/test_helpers.go` の `WireValuesForTest` が、ホストが実際に作る応答を `encoding/json` で読み直して返す）。`TestExtensionDocumentation` の「security.md 8.1 has no trace of the removed menu path」のサブテストは、「**要求する権限**」の項目に `chrome.storage.local` があることを求めていた（`storage` の権限の理由）ので、どの権限も `chrome.storage` で説明しないことを求める形に改める。
    -   `cmd/yt2column-server/docs_test.go`: security.md §8.2 の `nativeMessaging` の確認を加える（設計書 3.9 の表）。用語集の未送信・結果不明の行の確認を加える。
    -   `internal/pipeline/relay_guard_test.go`: `TestSharedTokenRemoved` の走査の対象に `extension/src`・`extension/static` を加える。
    -   **対象:** 上記のファイル。**完了:** `make test`・`make ext-test` が通る。
-   [x] **ステップ 3-12**: 壊して失敗することを確かめ、コミットメッセージに記録する（§1.2。§5 のフェーズ 3 のテストとガードのうち、ステップ 3-1〜3-11 が作る・改めるものが対象）。取り違えやすいもの: ポップアップの経路が `sendArticle` を 2 回呼ぶ・ログにポートを戻す（`launch.test.ts`）、`prepareSend` が設定を読む処理を戻す・理由の順序を入れ替える（`prepare.test.ts`）、`src/` に `sharedToken` を読む処理を加える（`secretsAbsent.test.ts`）、`not_found` を `ServerRejectReason` に戻す（`response.test.ts`）、`classifyNativeReply` の検査 1〜8 を 1 つずつ外す・lastError の文字列で分ける分岐を加える・base64 の再符号化の比較を外す・`body-over-limit` への受け渡しを外す（`native.test.ts`）、`sendArticle` が `timed-out` の後に届いた応答を使う・2 回呼ぶ（`send.test.ts`）、`createSendDeps` の同期の例外の捕捉を外す（`chromeDeps.test.ts`）、`host-failed` の文言から install の手順を外す・未送信の文言に Slack の確認を加える（`sendMessages.test.ts`）、manifest に `host_permissions` を戻す・`storage` を戻す（`manifest.test.ts`）、`nativeHostName` の 1 文字を変える（`TestNativeHostContract`）、README の表の「拡張の表示」の 1 行を誤らせる（`serverReasons.test.ts`）、README の「### Browser extension」に `shared token` を戻す（`TestExtensionSendDocumentation`）、`sendMessages.ts` に `YT2COLUMN_SERVER_PORT` を戻す（`TestSharedTokenRemoved`）。§4.4 の横断検索を行い、検索した語と結果をコミットメッセージに記録する。
    -   **対象:** なし（確認のみ）。**完了:** 各対象で該当するテストか lint が失敗することを確かめ、記録する。 PR-8 のマージの前に、このブランチでステップ 3-14 の手動の確認（§5.1）を行い、結果を PR-10 まで保持する（§3.2）。

### PR-8 作成ポイント: extension send through the native host

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 3-11 / 3-12

**推奨タイトル**: `feat(0026): send from the extension through the native messaging host`

**レビュー観点**: `classifyNativeReply`（ステップ 3-2）の検査の順序と、lastError の文字列で分けないこと（設計書 3.7.3・3.7.5） / `sendArticle`（ステップ 3-5）が `sendNative` を 1 回だけ呼び、25 秒の上限の後に届いた応答を使わないこと / オプションページ・設定・`fetch` の経路と `storage` の権限の削除 / 0025 が渡す `ArticleMetadata` の受け渡しと、0025 のガードが求める文書の語句を保つこと

**実装モデル要件**: frontier-required

**判定理由**: オプションページ・`fetch`・トークン・`storage` の権限の削除と Native Messaging の追加を同時に行う移行の段階で、拡張の多くのテストとガードを書き換える（mkplan のパネル方式の条件「振る舞いの追加と削除を同時に行う移行」）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）
-   [ ] **ステップ 3-13**: 拡張の接続先と `chrome.storage` を限る lint の規則を加える（設計書 7.3）。`extension/eslint.config.js` で `fetch`・`XMLHttpRequest`・`WebSocket`・`EventSource` の使用と `connectNative` の参照を禁止し、`sendNativeMessage` の参照を `src/browser/chromeDeps.ts` に限る。`chrome.storage.sync` の規則（`syncPattern`）を、`chrome.storage` の参照（ドット・計算されたメンバ・分割代入の形）を禁じる規則に置き換え、理由のコメントを、拡張が `storage` の権限を持たないことに改める（設計書 3.8）。`lintRules.test.ts`（AC-23・AC-24・AC-29）に、各規則が違反を検出し、`chromeDeps.ts` の `sendNativeMessage` を許すことの確認を加える。`storage.sync` の `describe` と、作成の関数のファイルの共通の規則の `storage.sync` の行を `chrome.storage` の参照の禁止に改め、`chrome.storage.local` を許す行を削除する。各規則を 1 つずつ外して `lintRules.test.ts` が失敗することを確かめ、コミットメッセージに記録する（§1.2）。
    -   **対象:** `extension/eslint.config.js`・`extension/test/lintRules.test.ts`。**完了:** `make ext-check` が通り、各規則を外すと `lintRules.test.ts` が失敗することを確かめた。

### PR-9 作成ポイント: extension connection and storage lint rules

**対象ステップ**: 3-13

**推奨タイトル**: `feat(0026): restrict the extension's connections and storage with lint rules`

**レビュー観点**: 規則が設計書 7.3 の禁止の一覧（`fetch`・`XMLHttpRequest`・`WebSocket`・`EventSource`・`connectNative`・`chrome.storage`）をすべて含むこと / `sendNativeMessage` の参照を `src/browser/chromeDeps.ts` だけに許すこと / 各規則を外すと `lintRules.test.ts` が失敗すること

**実装モデル要件**: standard

**判定理由**: 該当する条件はない。PR-8 の後のコードが既に満たす性質を lint の規則で確かめるだけで、振る舞いは変えない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）
-   [ ] **ステップ 3-14**: 手動の確認を、macOS の Chrome と Brave の作業時点の安定版で行い、§5.1 に記録する（AC-36。設計書 7.2 の最後の項目）。確認は PR-8 のマージの前にそのブランチで行い（§3.2）、PR-9 の後にこのステップで記録する。受理の行（実際の LLM API と Slack Webhook を呼ぶもの）は、実行の前に利用者の承認を得る（CLAUDE.md「Tool Execution Safety」）。記録した後、`cmd/yt2column/extension_docs_test.go` に `TestNativeMessagingRelayDocumentation` を加え、`checkManualCheckTable` で §5.1 の表の各行の名前と、すべての欄が埋まっていることを確かめる（空の欄があると失敗するので、表を埋めた後に加える）。加えた後、表の 1 つの欄を空にして失敗することを確かめ、コミットメッセージに記録する。
    -   **対象:** 本計画 §5.1・`cmd/yt2column/extension_docs_test.go`。**完了:** §5.1 のすべての欄が埋まり、`go test -tags test -run TestNativeMessagingRelayDocumentation ./cmd/yt2column` が通る。

**完了条件:** `make fmt` → `make test` → `make lint` → `make ext-check` → `make build` が通り、§5.1 が埋まっている。

### PR-10 作成ポイント: manual browser verification record

**対象ステップ**: 3-14

**推奨タイトル**: `test(0026): record the manual Chrome and Brave verification`

**レビュー観点**: §5.1 の各行が設計書 7.2 の確認の内容に対応し、ブラウザ・日付・結果が埋まっていること / 受理の行を実行する前に利用者の承認を得たこと / `TestNativeMessagingRelayDocumentation` が行の名前と空の欄を確かめていること

**実装モデル要件**: standard

**判定理由**: 該当する条件はない。新しい設計の判断はなく、コードの変更は既存の `checkManualCheckTable` を使うガード 1 つである。実際の LLM API と Webhook を呼ぶ確認は利用者が承認して自分で行うので、外部の資源の条件は実装するモデルに当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 内容 | 成果物 | 確かめ方 |
|---|---|---|---|
| M1 | フェーズ 1 の完了 | サーバが UDS だけで待ち受け、トークンとポートがない。サーバの文書とガードが UDS を前提にする | フェーズ 1 の完了条件。README の `curl --unix-socket` の例（ステップ 1-14） |
| M2 | フェーズ 2 の完了 | `yt2column-nmhost` の中継と install/uninstall。実際のサーバとホストの統合テスト | フェーズ 2 の完了条件。`TestRelayEndToEnd` |
| M3 | フェーズ 3 の完了 | 拡張がホストを通してサーバに送る。手動の確認の記録 | フェーズ 3 の完了条件。§5.1 |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | `internal/udstest`・`internal/serversock` とそのテスト | frontier-required |
| PR-2 | 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 / 1-12 / 1-13 / 1-14 / 1-15 / 1-16 | `internal/config` のソケットのパス、`internal/server` の UDS 化と検査の削除、`internal/authtoken` の削除、`cmd/yt2column-server` の起動、拡張の 4 つの理由の削除、フェーズ 1 の文書と文書のガード | frontier-required |
| PR-3 | 1-17 | `TestNoTCPListenerInProduction`・`TestSharedTokenRemoved`、`internal/server` → `internal/serversock` の depguard の規則 | standard |
| PR-4 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 | `internal/nmhost` のフレームと中継とそのテスト、`internal/nmhost` のガードと depguard の規則 | frontier-required |
| PR-5 | 2-6 / 2-7 / 2-8 | `internal/nmhost` の manifest の install/uninstall とそのテスト | frontier-required |
| PR-6 | 2-9 / 2-10 / 2-11 / 2-12 / 2-13 / 2-14 | `config.LoadHost`、`cmd/yt2column-nmhost`、Makefile、フェーズ 2 の文書とガード | frontier-required |
| PR-7 | 2-15 / 2-16 | 実際のサーバとホストの統合テスト `TestRelayEndToEnd` | frontier-required |
| PR-8 | 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 3-11 / 3-12 | 拡張の `sendNativeMessage` への切り替え、オプションページと設定の削除、フェーズ 3 の文書とガード | frontier-required |
| PR-9 | 3-13 | 拡張の接続先と `chrome.storage` を限る lint の規則と `lintRules.test.ts` | standard |
| PR-10 | 3-14 | Chrome と Brave の手動の確認の記録と `TestNativeMessagingRelayDocumentation` | standard |

-   PR-1 は、他のパッケージから使われない新しいパッケージ（`internal/udstest`・`internal/serversock`）だけを加えるので、単独でグリーンゲートを通る。ソケットを守る検査をまとめて確かめられるよう、サーバの移行から分けた。
-   PR-2 は大きいが、ステップ 1-6〜1-12 はコンパイルで結び付き（§1.3）、ステップ 1-12 の `TestServerDocsContract` はステップ 1-14・1-15 の後に通り、README の拒否の理由の表と拡張の理由の一致（ステップ 1-13・1-14）も同じ変更を求めるので、これ以上分けない。既に満たされた性質を確かめるだけのリポジトリのガード（PR-3）は分けた。レビューしやすいよう、PR-2 はステップごとにコミットする。
-   PR-4・PR-5 の `internal/nmhost` は、depguard で `internal/server`・`internal/config` に依存しない（ステップ 2-4 で加える）ので、`config.LoadHost` と `cmd/yt2column-nmhost` の前に単独で入れる。中継と manifest は互いに依存しないので、別の PR にした。PR-4 のガード（ステップ 2-4）は PR-3 の `relay_guard_test.go` に加える。`config.LoadHost`（ステップ 2-9）は `envaccess_test.go` の観測した回数の確認が `cmd/yt2column-nmhost/main.go` を求めるので、PR-6 に置く。
-   PR-7 の統合テストは、PR-6 の `cmd/yt2column-nmhost` をビルドして使う。重い統合テストを、ホストのコマンドと文書のレビューから分けた。
-   PR-2 は、0027 のすべての PR がマージされた後に始める（§1.3「0027 との順序」）。PR-8 は、0025 と 0027 のすべての PR がマージされた後に作る（フェーズ 3 の前提。PR-2 の前提を満たしていれば満たされる）。ステップ 3-2〜3-7 は型の検査とビルドで互いに結び付き（各ステップの完了）、テストと文書とガードも同じ変更を求めるので分けない。`native.ts`（ステップ 3-2）は `classifyResponse` とステップ 3-3 で改める結果の型を使うので、別の PR にしない。PR-8 はステップごとにコミットし、ステップ 3-2・3-5 を個別にレビューできるようにする。既に満たされた性質を確かめるだけの lint の規則（PR-9）は分けた。
-   §5.1 の手動の確認は、PR-8 のマージの前にそのブランチで行い（受理の行は、実行の前に利用者の承認を得る）、結果を PR-10 まで保持して記録する。手動の確認で見つかった不具合は PR-8 のブランチで直す。マージ済みの PR（PR-7 以前）で見つかった不具合は、別の `fix(0026)` の PR で直す。
-   各 PR は、その PR のテストとガードを壊して確かめるステップ（1-5・1-16・2-5・2-8・2-14・2-16・3-12、PR-3・PR-9・PR-10 はステップ 1-17・3-13・3-14 の中）を含む。
-   グリーンゲートは `make test && make lint` だが、拡張か README を変える PR（PR-2・PR-6・PR-8・PR-9）は `make ext-check` も、Makefile を変える PR-6 は `make build` も通す（各フェーズの完了条件。CI も実行する）。

### 3.3. 実装順序の根拠

-   設計書 8 章の順序に従う。フェーズ 2 の統合テストはフェーズ 1 の UDS のサーバを、フェーズ 3 の手動の確認はフェーズ 2 のホストを使う。
-   フェーズ 1 の中では、他に依存しない `internal/serversock` を先に作って単独で確かめ（設計書 8 章）、その後 `internal/config` の導出、`internal/server`、`cmd/yt2column-server` を改める。`internal/config` から `ServerConfig.Port()`・`TokenPath()` を削除すると `cmd/yt2column-server` がコンパイルできなくなるので、`internal/config` は `cmd/yt2column-server` と同じ PR に置く（§3.2）。テストの補助 `internal/udstest`（ステップ 1-2）は、それを使う最初のテスト（ステップ 1-4）の前に作る。
-   利用者に渡すのは、フェーズ 1〜3 がそろった版である（設計書 8 章、要件書 4.4「互換性はない」）。フェーズ 1 の後、フェーズ 3 の前の拡張はサーバに届かない。この間の `main` は利用者向けに配らない。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

-   設計書 7.1 のとおりに、`internal/config`・`internal/serversock`・`internal/server`・`cmd/yt2column-server`・`internal/nmhost`・`cmd/yt2column-nmhost`・拡張の各モジュールで確かめる。テストの名前と AC の対応は §5 に示す。
-   既存のテストが既に確かめている振る舞い（本文の検証の各理由、枠の上限、失敗の通知の分類、応答の形、`classifyResponse` の本文の検査、収集の理由と順序）は、新しい表を書かずに既存のテストで確かめる（§1.3）。
-   所有者と権限に依存するテストは、root では飛ばす（設計書 7.1）。CI のランナー（`.github/workflows/ci.yml` の `runs-on: ubuntu-latest`）は root で動かない前提である。`internal/cachelock` の `requireNonRoot` は root で失敗させるので再利用しない（§1.3）。
-   UDS のパスは `internal/udstest` の短いディレクトリに置く。macOS（103 バイト）と Linux（107 バイト）のどちらでも上限に収まる。

### 4.2. 統合テスト

-   `cmd/yt2column-server/relay_e2e_test.go` の `TestRelayEndToEnd`（ステップ 2-15）。ビルドタグは `test` で、`make test` と CI の `doc-guard` のジョブで実行される。実際の LLM API と Slack Webhook は呼ばない（fake の LLM と loopback の Webhook）。
-   ホストのビルドは、テストの中で `go build` を 1 回行う（数秒の増加。設計書 7.2 が受け入れている）。

### 4.3. 後方互換性

-   旧拡張と新サーバ、新拡張と旧サーバの互換性は持たない（要件書 4.4）。互換性のテストは書かない。
-   旧版の拡張が保存した `sharedToken`・`serverPort` は削除しない（設計書 3.7.1）。拡張は `storage` の権限を持たないので読み書きもしない。消したい利用者向けの手順を README に書く（ステップ 3-10）。
-   古いサーバのトークンのファイル（`server-token`）は、サーバが読まず、削除もしない。README の古い版からの更新の手順で、削除してよいことを案内する（ステップ 1-14）。

### 4.4. 網羅率と横断検索

-   `go tool cover -func` で、新しいパッケージ（`internal/serversock`・`internal/nmhost`・`cmd/yt2column-nmhost`）の公開の関数と、`internal/config` のソケットのパスの導出の関数に、テストで通らない分岐がないことを確かめる。
-   次の項目は `make lint`・`make test` では検出できないので、各フェーズの文書を改める PR で検索で確かめ、検索した語・対象・結果をその PR の壊して確かめるステップ（PR-2 のステップ 1-16、PR-6 のステップ 2-14、PR-8 のステップ 3-12）のコミットメッセージに記録する。
    -   [x] 古くなるコメント（§1.3「古くなるコメントと文書」）: `internal/server`・`cmd/yt2column-server` の本番のコードのコメントに、`token`・`loopback`・`127.0.0.1`・`Origin`・`authenticated`・`port` の語が、本タスクの変更の後の振る舞いと食い違う形で残っていないこと（フェーズ 1）。拡張の `src/` のコメントに `AbortSignal.timeout`・`settings`・`options`・`chrome.storage` の語と、メニューの経路・結果のウィンドウ・service worker への言及（0027 の後に残ったもの）が残っていないこと（フェーズ 3）。
    -   [x] トークンの説明の語の残り: 「共有トークン」「shared token」「options page」が、削除の理由と古い版からの更新の手順を除いて残っていないこと（§1.3 の `TestSharedTokenRemoved` の説明）。フェーズ 1・2 では、そのフェーズが改めた範囲（README の「## Localhost server」、security.md §6・§8 の冒頭・§8.2、CLAUDE.md・`project_overview.md`・`package_reference.md`・用語集、`cmd/`・`internal/`）だけを調べる（README の「### Browser extension」と `extension/` には、フェーズ 3 まで正しく残る）。フェーズ 3 では、現在の文書のすべてと `extension/`（`node_modules`・`dist` を除く）を調べる。どのフェーズでも、ガードのテストが検査の値として持つ語（`docs_test.go`・`extension_docs_test.go`・`secretsAbsent.test.ts` など）は残りに数えない。
    -   [x] 用語の一致: 本計画・README・security.md・`project_overview.md` の新しい用語（ホスト、ホスト名、起動のロック、ホストの応答、未送信、結果不明）が `docs/translation_glossary.md` の行と一致すること（各フェーズ）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別は、`test`（実行され、誤った振る舞いで失敗するテスト）、`static`（ガードのテスト・`make` の目標・lint の規則）、`manual`（手動の確認。§5.1）である。テストの場所は `path::TestName` の形で示す。拡張のテストは `describe` の名前を実装で決めるので、ファイルで示す。

| AC | 種別 | 検証 | 主な実装 |
|---|---|---|---|
| AC-01 | static | `internal/pipeline/relay_guard_test.go::TestNoTCPListenerInProduction`（TCP のリスナーを作るコードがないこと） | ステップ 1-17 |
| | test | `internal/server/server_test.go::TestServeOnUnixListener`（UDS で待ち受けること。TCP で接続できないことは上の static と下の manual で確かめる） | ステップ 1-8・1-11 |
| | manual | §5.1（サーバのプロセスが TCP のソケットを持たないこと） | - |
| AC-02 | test | `internal/serversock/serversock_test.go`（ディレクトリの作成と `0700`、`0700` でない・symlink・他人の所有・親の書き込みのビットでの中止、作成後のソケットの `0600`） | ステップ 1-3・1-4 |
| AC-03 | test | `internal/serversock/serversock_test.go`（stale の削除、`ErrInUse` と削除しないこと、通常ファイル・symlink・FIFO・ディレクトリの中止、`ErrAlreadyRunning`） | ステップ 1-3・1-4 |
| AC-04 | test | `internal/config/socketpath_test.go`（パスの長さの境界、`HOME` の不備、値を含まないエラー）、`cmd/yt2column-server/run_test.go::TestStartupFailures`、`::TestRunLogsListeningAndStop`、`internal/serversock/serversock_test.go`（上限の定数と `RawSockaddrUnix` の一致） | ステップ 1-6・1-7・1-11・1-12 |
| AC-05 | static | `internal/pipeline/relay_guard_test.go::TestSharedTokenRemoved` | ステップ 1-10・1-14・1-17・3-11 |
| | test | `cmd/yt2column-server/signal_test.go::TestSignalStopsServer`（標準出力が空）、`internal/config/socketpath_test.go`（`HOME` の値がエラーに現れない）、`cmd/yt2column-server/run_test.go::TestOutputsOmitSecrets`（秘密の値が出力に現れない） | ステップ 1-6・1-7・1-11・1-12 |
| AC-06 | test | `cmd/yt2column-server/signal_test.go::TestSignalWaitsForJob`、`cmd/yt2column-server/run_test.go::TestStartDuringShutdownRefused`、`internal/serversock/serversock_test.go`（閉じるとソケットが消えること、stale の削除） | ステップ 1-3・1-4・1-11・1-12 |
| | manual | §5.1（`SIGKILL` で止めたサーバの次の起動） | - |
| AC-07 | test | `internal/server/server_test.go::TestServeShutdownWaitsForJobs`、`cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd` の (1) | 既存・ステップ 2-15 |
| AC-08 | test | `internal/server/request_test.go::TestRejectBodyTable`・`::TestRejectVideoURLTable`・`::TestRejectInvalidTranscript`（既存） | 既存 |
| AC-09 | test | `internal/server/server_test.go::TestExpectContinueOversizedBodyNotSent`、`internal/server/request_test.go::TestBodyTooLarge`（既存） | ステップ 1-9 |
| AC-10 | test | `internal/server/job_test.go::TestSlotLimit`（既存） | 既存 |
| AC-11 | test | `internal/server/response_test.go::TestResponseOmitsInputs`（既存）、`cmd/yt2column-server/run_test.go::TestOutputsOmitSecrets` | ステップ 1-12 |
| AC-12 | test | `internal/server/response_test.go::TestRejectReasonSet`、`internal/server/handler_test.go::TestAcceptsWithoutBrowserHeaders`・`::TestRejectHeaders`・`::TestRejectStatusCodes`、`internal/server/server_test.go::TestUnparsableRequestBypassesHandler`、`extension/test/response.test.ts`（`not_found` が想定外の応答になること）、`extension/test/serverReasons.test.ts` | ステップ 1-8・1-9・1-13・3-3 |
| AC-13 | test | `internal/server/job_test.go::TestFailureClassification`・`::TestNotifyNoticeFields`（既存）、`cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd` の (2) | 既存・ステップ 2-15 |
| AC-14 | test | `cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd` の (1) | ステップ 2-15 |
| | manual | §5.1（応答の後にブラウザを終了しても Slack に記事が届く） | - |
| AC-15 | test | `internal/nmhost/relay_test.go::TestRelayForwardsOneRequest`・`::TestRelayNoReplyFrame`、`TestRelayEndToEnd` の (1) | ステップ 2-2・2-3 |
| AC-16 | test | `internal/nmhost/relay_test.go::TestRelayLengthChecks`・`::TestRelayServerRejectsWithoutBody`・`::TestRelayResponseOverLimit`、`TestRelayEndToEnd` の (4) | ステップ 2-2・2-3 |
| AC-17 | test | `internal/nmhost/relay_test.go::TestRelayOutputs`、`TestRelayEndToEnd` の (5)（`runHost` がすべての実行で確かめる） | ステップ 2-2・2-3 |
| | static | `internal/pipeline/relay_guard_test.go::TestHostStdoutOnlyInMain`・`::TestSelectorDetection` | ステップ 2-4・2-13 |
| AC-18 | test | `internal/config/socketpath_test.go`（`XDG_*`・`TMPDIR` で変わらないこと、`LoadHost` が `HOME` だけを読むこと）、`internal/config/envaccess_test.go`（既存のテストに許可を 1 行加える）、`internal/nmhost/relay_test.go::TestRelayForwardsOneRequest`（`TestMain` がプロキシの環境変数を設定する）、`cmd/yt2column-nmhost/main_test.go`（すべての役割で `HOME` だけを読むこと） | ステップ 1-6・2-2・2-9・2-10 |
| | static | `make lint`（`depguard` の `os/exec` の規則）、`internal/pipeline/relay_guard_test.go::TestHostSpawnsNoProcess` | ステップ 2-4・2-13 |
| AC-19 | test | `internal/nmhost/relay_test.go::TestRelayHostErrors`・`::TestExchangeFailurePrecedence`・`::TestOneUseTransportRefusesRedial`、`TestRelayEndToEnd` の (3) | ステップ 2-2・2-3 |
| AC-20 | test | `internal/nmhost/manifest_test.go::TestInstall`・`::TestUninstall`・`::TestManifestLocation`・`::TestParseBrowser`、`cmd/yt2column-nmhost/main_test.go`（manifest のパスと macOS での共有の注記） | ステップ 2-6・2-7・2-10 |
| | static | `cmd/yt2column/makefile_test.go::TestMakeBuildBuildsEveryBinary`（ビルドが install を実行しない）、`cmd/yt2column/extension_docs_test.go::TestNativeHostContract`（`allowed_origins` の値が manifest の `key` から導出した ID と一致すること） | ステップ 2-11・2-13 |
| | manual | §5.1（install・uninstall の後のブラウザでの起動の可否） | - |
| AC-21 | static | `make build`、`make lint`（`depguard` の既存の `deps` の規則） | ステップ 2-10・2-11 |
| AC-22 | test | `extension/test/manifest.test.ts`、`cmd/yt2column/extension_docs_test.go::TestExtensionDocumentation` | ステップ 3-1 |
| AC-23 | test | `extension/test/prepare.test.ts`（設定を読まないこと）、`extension/test/launch.test.ts`（ログにトークンとポートがないこと）、`extension/test/secretsAbsent.test.ts`、`extension/test/manifest.test.ts`（`storage` の権限がないこと） | ステップ 3-1・3-4・3-7 |
| | static | `internal/pipeline/relay_guard_test.go::TestSharedTokenRemoved`（フェーズ 3 で `extension/` に広げる）、`extension/test/lintRules.test.ts`（`chrome.storage` の参照の禁止） | ステップ 3-11・3-13 |
| AC-24 | test | `extension/test/send.test.ts`、`extension/test/launch.test.ts`、`extension/test/chromeDeps.test.ts` | ステップ 3-5・3-7 |
| | static | `extension/test/lintRules.test.ts`（`connectNative` の禁止） | ステップ 3-13 |
| | manual | §5.1（ポップアップから 1 回だけ送られること） | - |
| AC-25 | test | `extension/test/sendMessages.test.ts`、`extension/test/renderSend.test.ts` | ステップ 3-6 |
| AC-26 | test | `extension/test/native.test.ts`、`extension/test/sendMessages.test.ts` | ステップ 3-2・3-6 |
| | manual | §5.1（ホスト未インストール、サーバ停止、拒否、上限超過の表示） | - |
| AC-27 | test | `extension/test/native.test.ts`、`extension/test/response.test.ts`（既存）、`internal/nmhost/relay_test.go::TestReplyShapes` | ステップ 2-2・3-2 |
| AC-28 | test | `extension/test/renderSend.test.ts`（既存の `requestId` の表示） | 既存 |
| | manual | §5.1（`requestId` を選択してコピーできること。選択のしやすさは CSS で決まり、ユニットテストでは確かめない） | - |
| AC-29 | static | `extension/test/lintRules.test.ts`（`fetch`・`XMLHttpRequest`・`WebSocket`・`EventSource`・`connectNative` の禁止、`sendNativeMessage` の限定）、`make ext-lint` | ステップ 3-13 |
| | test | `extension/test/secretsAbsent.test.ts` | 既存・ステップ 3-9 |
| AC-30 | test | `extension/test/prepare.test.ts`、`extension/test/collect.test.ts`（既存） | ステップ 3-4 |
| AC-31 | static | `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（security.md のサブテスト。§8 の冒頭の相手ごとの排除の根拠、§8.2 の UDS の権限・ホストの標準出力・残余リスク）、`cmd/yt2column/extension_docs_test.go::TestExtensionSendDocumentation`（トークンの貼り付けの手順がないこと）、`extension/test/manifest.test.ts`（§8.1 の権限の一覧） | ステップ 1-15・2-13・3-11 |
| AC-32 | static | `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（README のサブテスト。ソケットのパス、install/uninstall、トークンが不要であること、ホスト未インストールとサーバ停止のトラブルシューティング、禁止する語句）、`cmd/yt2column/extension_docs_test.go::TestExtensionSendDocumentation`（README のサブテスト） | ステップ 1-15・2-13・3-11 |
| AC-33 | static | `cmd/yt2column-server/docs_test.go::TestServerDocsContract`（`project_overview.md`・CLAUDE.md のサブテスト。パッケージ名、install/uninstall のコマンド、`make build` の 3 つのバイナリ）、`internal/pipeline/pipeline_test.go::TestPackageReferenceListsPackages`（既存）、`cmd/yt2column/makefile_test.go::TestMakeBuildBuildsEveryBinary` | ステップ 1-15・2-11・2-13 |
| AC-34 | static | `make test`（`TestServerDocsContract`・`TestExtensionDocumentation`・`TestExtensionSendDocumentation`・`TestPackageReferenceListsPackages`・`TestNativeHostContract`）、`make ext-test`（`serverReasons.test.ts`・`manifest.test.ts`） | 各 PR |
| AC-35 | test | `cmd/yt2column-server/relay_e2e_test.go::TestRelayEndToEnd` | ステップ 2-15 |
| AC-36 | static | `cmd/yt2column/extension_docs_test.go::TestNativeMessagingRelayDocumentation`（§5.1 の表のすべての欄が埋まっていること） | ステップ 3-14 |
| | manual | §5.1 | ステップ 3-14 |
| AC-37 | static | `make test`・`make lint`・`make ext-check` | 各 PR |

### 5.1. 手動の確認と記録

実装のとき（ステップ 3-14）に埋める。macOS の Chrome と Brave の作業時点の安定版で、ブラウザごとに 1 行ずつ記録する。各行の最初の欄の名前は互いに異なる。`TestNativeMessagingRelayDocumentation`（ステップ 3-14）が、この表の最初の欄の名前を順に並べた一覧と照らして各行の名前を確かめ、すべての欄が埋まっていることを確かめる。受理の行は実際の LLM API と Slack Webhook を呼ぶので、実行の前に利用者の承認を得る。

| AC | 確認の内容（設計書 7.2） | ブラウザ・環境 | 日付 | 結果 |
|---|---|---|---|---|
| AC-01・AC-06 サーバ | サーバの起動中に `lsof -nP -a -p <サーバの pid> -iTCP` が何も表示しない（TCP のソケットを持たない）。`SIGKILL` で止めた後の次の起動が、stale なソケットを削除して起動する | ターミナル（macOS） | | |
| AC-26 未インストール（Chrome） | ホストを install していない状態で送り、ホストの失敗（install の手順と Slack の確認）が表示される | Chrome | | |
| AC-26 未インストール（Brave） | 同上 | Brave | | |
| AC-26 サーバ停止（Chrome） | `install chrome` の後、サーバを止めた状態で送り、未送信（サーバの起動の案内、Slack の確認なし）が表示される | Chrome | | |
| AC-26 サーバ停止（Brave） | `install brave` の後、同上 | Brave | | |
| AC-24 受理（Chrome） | サーバの起動中に、ポップアップから受理される入力を送り、受付と `requestId`（選択してコピーできる。AC-28）が表示され、Slack に記事が 1 件届く（AC-25） | Chrome | | |
| AC-24 受理（Brave） | 同上 | Brave | | |
| AC-26 拒否（Chrome） | 字幕の書式に合わない選択範囲で `invalid_transcript` の拒否が表示される | Chrome | | |
| AC-26 拒否（Brave） | 同上 | Brave | | |
| AC-26 同時の送信（Chrome） | 2 つのジョブが処理中に 3 つ目を送り、`too_many_requests` の拒否が表示される | Chrome | | |
| AC-26 同時の送信（Brave） | 同上 | Brave | | |
| AC-14 ブラウザの終了（Chrome） | 受付応答の後にブラウザを終了し、Slack に記事が届く | Chrome | | |
| AC-14 ブラウザの終了（Brave） | 同上 | Brave | | |
| AC-26 4 MiB 超（Chrome） | 4 MiB を超える本文で、拡張の送る前の確認（`body-too-large`）の表示になる | Chrome | | |
| AC-26 4 MiB 超（Brave） | 同上 | Brave | | |
| AC-20 再インストール（chrome を指定） | バイナリを移動した後にホストの失敗が表示され、`install chrome` をやり直すと送れる。`uninstall chrome` の後は Chrome と Brave の両方でホストの失敗になる（macOS での共有） | Chrome・Brave | | |
| AC-20 再インストール（brave を指定） | `install brave` の後に Chrome でも送れる。`uninstall brave` の後は両方のブラウザでホストの失敗になる | Chrome・Brave | | |

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| UDS のパスが macOS のテストの一時ディレクトリで `sun_path` を超える | テストが環境によって失敗する | `internal/udstest` の `/tmp` の下の短いディレクトリを使う（ステップ 1-2） |
| `Expect: 100-continue` の振る舞いが想定と異なり、サーバの早い拒否が書き込みのエラーになる | `413`・`429` が `protocol_error`（結果不明）として表示される | サーバ側（`TestExpectContinueOversizedBodyNotSent`）とホスト側（`TestRelayServerRejectsWithoutBody`）、統合テストの (4) で確かめる。Go の `net/http` のサーバは、ハンドラが本文を読み始めたときにだけ `100 Continue` を書く（`$(go env GOROOT)/src/net/http/server.go` の `expectContinueReader`、Go 1.27.1 で確認） |
| 統合テストの `go build` が CI で遅い・失敗する | `make test` と `doc-guard` のジョブが遅くなる | ビルドは 1 回だけにし、`GOFLAGS` を空にする（設計書 7.2）。遅さは設計書 7.2 が受け入れている |
| 所有者や権限のテストが root の環境で成り立たない | テストの誤った成功・失敗 | root では飛ばす（設計書 7.1） |
| 0025 の残りの PR とフェーズ 3 が同じファイルを変える | 競合と、`ArticleMetadata` の受け渡しの取りこぼし | 0025 の完了の後にフェーズ 3 を始める（§1.3） |
| 0027 が PR-2 より後に残る（0027 の設計・実装が長引く） | PR-2 の後の `main` では 0027 の手動確認（0027 AC-07 の受付の表示）ができない。0026 は PR-1 の後で止まる | PR-2 は 0027 のすべての PR のマージの後に始める（§1.3「0027 との順序」）。0027 が長引いて 0026 を先に進める必要が出た場合は、PR-2 の直前のコミットからビルドしたサーバで 0027 の手動確認を行う形に、本計画を改めてから進める |
| 0027 の設計で、本計画が前提にする名前（`schedule`・`ResultView`）の扱いが変わる | ステップ 3-6・3-7 の対象の食い違い | 0027 の完了の後にフェーズ 3 を始め、始める前に残った名前を確かめる（§1.3「0027 との順序」） |
| フェーズ 1 の後の `main` で拡張がサーバに届かない | 開発中の利用者が送れない | フェーズ 1〜3 をそろえてから利用者に渡す（§3.3） |

### 6.2. スケジュールのリスク

-   フェーズ 1 は変更の範囲が広い（`internal/config`・新しいパッケージ・`internal/server`・`cmd/yt2column-server`・文書とガード・拡張の 4 つの理由）。新しいパッケージ（`internal/udstest`・`internal/serversock`）は PR-1 に、リポジトリのガードは PR-3 に分けたが、残りの PR-2 はコンパイルと文書のガードで結び付くので大きい（§3.2）。
-   手動の確認（ステップ 3-14）は Chrome と Brave の両方で、実際の LLM API と Webhook を使う行を含む。利用者の承認を待つ時間を見込む。

## 7. 実装チェックリスト (Implementation Checklist)

-   [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
-   [x] 0027 のすべての PR がマージされたことを確かめた（PR-2 の前提。PR-8 の前提も兼ねる）
-   [x] PR-2 マージ済み（対象ステップ: 1-6 / 1-7 / 1-8 / 1-9 / 1-10 / 1-11 / 1-12 / 1-13 / 1-14 / 1-15 / 1-16）
-   [x] PR-3 マージ済み（対象ステップ: 1-17）
-   [x] PR-4 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5）
-   [x] PR-5 マージ済み（対象ステップ: 2-6 / 2-7 / 2-8）
-   [x] PR-6 マージ済み（対象ステップ: 2-9 / 2-10 / 2-11 / 2-12 / 2-13 / 2-14）
-   [x] PR-7 マージ済み（対象ステップ: 2-15 / 2-16）
-   [x] 0025 のすべての PR がマージされたことを確かめた（PR-8 の前提）
-   [ ] PR-8 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4 / 3-5 / 3-6 / 3-7 / 3-8 / 3-9 / 3-10 / 3-11 / 3-12）
-   [ ] PR-9 マージ済み（対象ステップ: 3-13）
-   [ ] PR-10 マージ済み（対象ステップ: 3-14）
-   [ ] §5 のすべての AC の検証が通り、§5.1 が埋まっている
-   [x] §4.4 の横断検索を PR-2・PR-6・PR-8 で確かめた

## 8. 成功基準 (Success Criteria)

-   **機能:** §5 のすべての AC の `test`・`static` の検証が通り、§5.1 の手動の確認が Chrome と Brave の両方で記録されている。
-   **品質:** `make test`・`make lint`・`make ext-check`・`make build` が通る。新しいパッケージの公開の関数に、テストで通らない分岐がない（§4.4）。各テストとガードについて、壊して失敗することを確かめた記録がコミットメッセージにある。
-   **セキュリティ:** TCP のリスナーがコードにないこと（`TestNoTCPListenerInProduction`）、トークンとポートがコードと現在の文書にないこと（`TestSharedTokenRemoved`）、ソケットのディレクトリとソケットの権限（`internal/serversock` のテスト）、ホストの標準出力の純度と秘密を読まないこと（AC-17・AC-18 の検証）、拡張の接続先の限定（AC-29 の検証）が確かめられている。
-   **文書:** 設計書 3.9 の表の文書がすべて改められ、ガードが通る。`0011`・`0020`・`0021` の文書は変更していない。

## 9. 次のステップ (Next Steps)

-   `/runplan 0026` で PR-1 から順に実装する。各 PR のマージの後、本計画のチェックボックスを更新する。
-   フェーズ 3 の完了の後、設計書 9 章の将来の拡張（`/status` の口、相手の資格情報の確認など）は、必要になった時点で別のタスクとして扱う。
