# 開発の手順

本書は yt2column の開発者のための文書である。`make` のターゲット、開発用のコマンド、開発用の拡張のビルド、拡張の開発のための検査、ジョブの記録の値、統合テストを説明する。ブラウザ拡張の使い方は [README](../../../README.md) に、CLI は [cli.md](cli.md) にある。

## make のターゲット

```sh
make build   # build the binaries into build/
make test    # unit tests (no network, no yt-dlp)
make lint    # golangci-lint (pinned version) と make wasm-check
make wasm-check  # 2 つの記事生成モジュールを Go の WASM（GOOS=js GOARCH=wasm）でビルドできることの検査
make fmt     # gofumpt on changed files
make test-integration  # real yt-dlp + network; see below
make test-integration-deepseek  # real DeepSeek API; see below
make test-integration-claude  # real Anthropic API; see below
make test-integration-openai  # real OpenAI API; see below
make test-integration-cli  # the CLI with the real DeepSeek API; see below
```

開発は、受け入れ基準を明示した、要件定義 → 設計 → 実装計画の手順で進める。[CLAUDE.md](../../../CLAUDE.md) と
[docs/dev/conventions/requirements_process.md](../conventions/requirements_process.md) を参照。
設計の文書は日本語で書く。まず
[docs/dev/project_overview.md](../project_overview.md) を読む。

## 開発用のコマンドで記事の生成を再現する

`cmd/yt2column-extjob` は、拡張の記事生成モジュールが登録する JSON の入口（`internal/extjob` の `prepare` と `generate`）を、ブラウザの外でネイティブに動かす開発用のコマンドである。拡張で起きた記事の生成の不具合を、同じ入力で再現し、デバッガで追うために使う。本番のバイナリではなく、`make build` は作らない。

```sh
go build -o /tmp/yt2column-extjob ./cmd/yt2column-extjob
/tmp/yt2column-extjob -llm=fake -article=standard -failure=none -fail-at=none -delay=0 -settings-check=ok -input=/tmp/request.json
/tmp/yt2column-extjob -llm=claude -input=/tmp/request.json
```

終了コードを見るときは、ビルドしたバイナリを直接動かす。`go run` は、終了コードが `0` でないときに自分の終了コード `1` で終わり、このコマンドの終了コードを返さない。

`-llm` は必ず指定する。既定の値はない。指定を忘れた実行が、料金のかかる呼び出しにならないためである。`-llm` がないかほかの値のとき、`-input` がないとき、同じ引数を 2 回以上与えたとき、余分な語があるときは、何も呼ばずに終了コード `2` で終わる。

-   **`-llm=fake`**: fake の LLM（`internal/llm/devfake`）で動き、API を呼ばない。シナリオを次の 5 つの引数で指定する。どれも省けず、空の値と受理しない綴りでは、何も呼ばずに終了コード `2` で終わる。

    | 引数 | 意味 | 受理する値 |
    |---|---|---|
    | `-article` | 記事の種類 | `standard`・`long`・`elements` |
    | `-failure` | 失敗の種類 | `none`・`empty`・`truncated`・`api-error`・`timeout`・`api_key_rejected` |
    | `-fail-at` | 失敗させる工程 | `none`・`draft`・`shorten`・`review` |
    | `-delay` | LLM の呼び出し 1 回ごとの遅れ（秒） | `0` から `600` までの 10 進の整数 |
    | `-settings-check` | 設定の検査（`checkSettings`）の結果 | `ok`・`key_rejected`・`unverified` |

    `-delay` の値は、`05` のように先頭が `0` の 2 桁以上の数も受理しない。`-failure` と `-fail-at` は、どちらも `none` か、どちらも `none` でないかの組で指定する。`-fail-at shorten` は `-article long` のときだけ受理する。`-settings-check` は、このコマンドが呼ばない設定の検査の結果なので結果を変えないが、ほかの 4 つと同じく省けない。
-   **`-llm=claude`**: 本番の構成の Claude を呼ぶ（料金がかかる）。API キーとワークスペース ID は、CLI と同じ環境変数 `ANTHROPIC_API_KEY`・`ANTHROPIC_WORKSPACE_ID` から読み、拡張のオプションページが保存したのと同じ形で記事生成モジュールに渡す。ワークスペース ID が未設定なら、指定なしになる。ほかの環境変数（`YT2COLUMN_LLM` など）は読まないので、3 つの工程は本番の構成になる。シナリオの引数は受け付けない。`GODEBUG` に `http2debug` があるときは、CLI と同じく、API キーが標準エラー出力に書かれうることを警告してから呼ぶ（[security.md](../security.md) の §2）。`ANTHROPIC_API_KEY` が未設定なら、何も呼ばずに終了コード `2` で終わる。空の値は未設定とせずに記事生成モジュールに渡すので、`settings_invalid` の結果になる。

`-input` は、生成の開始の入力のファイルである。形は記事生成モジュールの `prepare` が受け取る JSON（`videoUrl`・`transcript` と、省ける `title`・`channelName`・`description`）で、4 MiB までである。読めないファイルは、何も呼ばずに終了コード `2` で終わる。**入力のファイルはリポジトリに入れない。** 利用者がコピーした字幕を含みうるためである（[security.md](../security.md) の §7）。

標準出力には、結果の JSON を 1 行書く。記事生成モジュールが入力を受理しなければ `prepare` の拒否の結果を、受理すれば `generate` の結果を書く。標準エラー出力には、工程が変わるごとの進み具合の JSON と、メッセージを書く。API キーは、どちらにも出ない。終了コードは、記事ができたときに `0`、入力が拒否されたか生成が失敗したとき、または結果を標準出力に書けなかったときに `1` である。

ブラウザに固有の部分（`fetch` を通した送信、Worker、`syscall/js` の受け渡し）は、このコマンドでは再現できない。

## 開発用の拡張のビルド

`make ext-build-dev` は、公開用の `extension/dist/` とは別に、fake の LLM（`internal/llm/devfake`）で記事を作る開発用の拡張を `extension/dist-dev/` に作る。拡張に入れる記事生成モジュールは開発用の `cmd/yt2column-wasm-dev` で、Anthropic の API を呼ばない。シナリオはビルドのときに決まり、次の 5 つの変数で選ぶ。変数は `make` の引数で与え、与えない変数は既定の値になる。環境変数では選べない（`make -e` で走らせたときを除く）。

```sh
make ext-build-dev
make ext-build-dev FAKE_ARTICLE=long FAKE_FAILURE=timeout FAKE_FAIL_AT=shorten
```

| 変数 | 意味 | 受理する値 | 既定の値 |
|---|---|---|---|
| `FAKE_ARTICLE` | 記事の種類 | `standard`・`long`・`elements` | `standard` |
| `FAKE_FAILURE` | 失敗の種類 | `none`・`empty`・`truncated`・`api-error`・`timeout`・`api_key_rejected` | `none` |
| `FAKE_FAIL_AT` | 失敗させる工程 | `none`・`draft`・`shorten`・`review` | `none` |
| `FAKE_DELAY` | LLM の呼び出し 1 回ごとの遅れ（秒） | `0` から `600` までの 10 進の整数 | `0` |
| `FAKE_SETTINGS_CHECK` | オプションページの設定の検査（`checkSettings`）の結果 | `ok`・`key_rejected`・`unverified` | `ok` |

組み合わせの規則は、[開発用のコマンド](#開発用のコマンドで記事の生成を再現する)の `-llm=fake` と同じである。`FAKE_DELAY` は、`05` のように先頭が `0` の 2 桁以上の数を受理しない。`FAKE_FAILURE` と `FAKE_FAIL_AT` は、どちらも `none` か、どちらも `none` でないかの組で与える。`FAKE_FAIL_AT=shorten` は `FAKE_ARTICLE=long` のときだけ受理する。受理しない値では、ビルドは `extension/dist-dev/` に触れる前に、理由を出して失敗する。前のビルドの `extension/dist-dev/` はそのまま残る。

開発用のビルドは、次の点で公開用のビルドと見分けられる。

-   manifest が `key` を持たないので、拡張の ID が公開用のビルドと違う。`chrome.storage.local` も別になり、開発用のビルドで保存した値が公開用のビルドの値を上書きしない。
-   拡張機能のページでの名前が `yt2column（開発用のビルド）`（英語の画面では `yt2column (development build)`）である。
-   ビルドの種類を宣言するモジュール（`buildKind.js`）が `dev` を宣言する。

どちらのビルドも、最後に成果物を検査する。成果物が `tsc` の出力、`static/` の写し、記事生成モジュール、`wasm_exec.js` だけであること、記事生成モジュールを Node.js で読み込んで呼んだ `buildInfo` の main のパッケージと Go の版と LLM の実装（開発用のビルドでは、ビルドに与えたシナリオ）が期待どおりであることを確かめ、合わなければビルドを失敗させる。

**Go のツールチェーン。** どちらのビルドも、記事生成モジュールを `go.mod` の `go` の行の版の Go で（`GOTOOLCHAIN` を与えて）ビルドし、そのツールチェーンの `wasm_exec.js` を成果物に写す。手元の Go がその版でないときは、初めのビルドが、その版のツールチェーンをダウンロードする（ネットワークを使う）。`GOFLAGS` が設定されているとき（`go env -w` の設定を含む）は、ビルドするものを変えうるので、ビルドは何もせずに失敗する。

**画面で確かめる。** `extension/dist-dev/` を、公開用のビルドと同じく **パッケージ化されていない拡張機能を読み込む** で読み込む。公開用のビルドと並べて読み込める。ポップアップとビューアは、開発用のビルドであることを文字の印で示す。オプションページは、ビルドの情報の LLM に、fake とそのシナリオを示す。

-   記事の生成の前に、オプションページで API キーを保存する。開発用のビルドは API を呼ばないので、値は形の合う偽のものでよい（空白を含まない、印字可能な ASCII の文字だけの文字列）。保存の結果は、`FAKE_SETTINGS_CHECK` のシナリオで決まる。
-   シナリオを変えるには、`make ext-build-dev` をほかの値でもう一度実行し、拡張機能のページで開発用のビルドを再読み込みする。再読み込みで、開いている拡張のページとジョブの記録は消える。
-   保存した設定が使えない状態（`settings_invalid`）は、開発用のビルドの DevTools で `chrome.storage.local` の `anthropic.apiKey` を文字列でない値に書き換えて出す。
-   記事のタイトルは `[FAKE] ` で始まる。記事のモデル名は `yt2column-devfake` で単価の表にないので、ビューアは料金を不明として表示する。
-   遅れの間も、同時に走る実行（2 つ）に数えるので、遅れの長い記事の生成を 2 つ走らせている間の 3 つ目の開始は、本番と同じく拒否される。
-   失敗の後の「やり直す」で成功する流れは確かめられない。シナリオはビルドのときに決まるので、やり直しても同じ失敗になる。

## 拡張の検査

拡張のビルドと読み込みは、README の
[ブラウザ拡張](../../../README.md#ブラウザ拡張)にある。同じ Node.js の版を有効にしたまま、もう 1 つのターゲットで CI と同じ検査を走らせる。

```sh
make ext-check     # typecheck, lint, format check, tests, then the release and development builds
```

## ジョブの記録

拡張は、記事の生成ごとの記録（ジョブの記録）を `chrome.storage.session` に置く。書くのは Service Worker だけで、ポップアップとビューアは読んで表示する。項目の名前は `job:` とジョブ ID を連結したもので、ジョブ ID は Service Worker が作る 128 ビットの乱数の 32 文字の小文字の 16 進である。記録は、状態（`state`）、工程の状態（`steps`）、完成した記事（`article`）、失敗の値（`failure`）を持ち、コピーした字幕と概要欄を持たない。形と検査は `extension/src/core/jobRecord.ts` にある。開発者ツールで記録を見るには、拡張のページの DevTools のコンソールで `await chrome.storage.session.get(null)` を実行する。

上限は、記録 50 件、記録の大きさの合計 8 MiB、記録 1 つの記事 256 KiB である。上限を超える記録を加えるときは、失敗と止まった記録のうち古いものから、次に完成した記録のうち古いものから捨てる。処理中の記録は捨てない。

### ジョブの記録の値

記録は、それぞれの値を次のいずれかで持つ。

| ジョブの状態 | 意味 |
|---|---|
| `running` | 実行するビューアが記事を生成している |
| `done` | 記事ができた |
| `failed` | 生成が失敗した |
| `stopped` | 実行するビューアのページが閉じられたか、読み込み直されたか、連絡が取れなくなった |

| 工程 | 生成の中の工程 |
|---|---|
| `draft` | 下書きの生成 |
| `shorten` | 長い本文の字数の調整 |
| `review` | 元の資料と照らす検証・推敲 |

| 工程の状態 | 意味 |
|---|---|
| `pending` | 始まっていない |
| `running` | 始まって、終わっていない |
| `done` | 終わった |
| `skipped` | この記事には要らなかった |
| `failed` | 生成がこの工程で失敗した |

| 失敗 | 意味 |
|---|---|
| `internal` | 内部の失敗 |
| `transcript_too_long` | 文字起こしがプロンプトに対して長すぎる |
| `llm_timeout` | LLM の呼び出しが時間切れになった |
| `llm_empty` | LLM が空の応答を返した |
| `llm_truncated` | LLM の応答が途中で切れた |
| `llm_failed` | そのほかの LLM の失敗 |
| `article_invalid` | 生成した記事を公開できない |
| `article_too_large` | 生成した記事が記録に収まらない |
| `api_key_rejected` | API が API キーを拒んだ（401） |
| `settings_invalid` | 保存した設定が形に合わず、API を呼ばなかった |
| `api_key_missing` | 予約の後に API キーがなく、API を呼ばなかった |
| `run_timeout` | 記事の生成の全体が 50 分の期限を過ぎた |

| 内部の失敗の種類 | 意味 |
|---|---|
| `panic` | 記事生成モジュールの関数が回復しない失敗で終わった |
| `worker_error` | Worker が読み込みの後に失敗した |
| `worker_load` | Worker が記事生成モジュールを期限までに読み込めなかった |
| `result_shape` | 記事生成モジュールの結果が形に合わなかった |
| `store_failed` | Service Worker が結果を記録に書けなかった |
| `input_mismatch` | `generate` のリクエストの入力のダイジェストが、Worker が受け取って検査した入力と合わなかった |
| `request_shape` | 記事生成モジュールへのリクエストが形に合わなかった |
| `settings_read` | 実行するビューアが保存した設定を読めなかった |
| `runtime_exit` | 呼び出しを待っている間に Go のプログラムが終わった |

| 止まった理由 | 意味 |
|---|---|
| `tab_closed` | 実行するビューアのタブが閉じられた |
| `page_closed` | 実行するビューアのページとの接続が切れた |
| `page_reloaded` | 実行するビューアのページが読み込み直された |
| `no_contact` | 予約のときに、実行するビューアと連絡が取れなかった |

内部の失敗の種類と止まった理由は、開発者が記録から原因を読むための値で、画面には文として示さない。

## 統合テスト

### yt-dlp の統合テスト

`make test-integration` は本物の `yt-dlp` をネットワークに向けて実行するので、
[yt-dlp の準備](cli.md#yt-dlp-の準備)のとおりに準備した `yt-dlp` が `PATH` 上に要る。
本物の動画（既定では
`https://www.youtube.com/watch?v=EQCUZyB4DqE`）を取得し、YouTube に 2 分の間隔で 2 つのリクエストを送るので、1 回の実行に数分かかる。動画は
`make test-integration YT2COLUMN_TEST_VIDEO_URL=... YT2COLUMN_TEST_VIDEO_ID=...`
で変えられる（空の `YT2COLUMN_TEST_VIDEO_ID` は動画 ID の確認を飛ばす）。YouTube の流量の制限にかからないよう、実行の間を空ける。

### DeepSeek の統合テスト

`make test-integration-deepseek` は本物の DeepSeek の API を呼ぶので、課金される。`YT2COLUMN_TEST_DEEPSEEK_API_KEY` に、テスト用の DeepSeek の API キーを設定する必要がある。これは本番の `DEEPSEEK_API_KEY` とは別のもので、テストは本番のキーを読まない。`YT2COLUMN_TEST_DEEPSEEK_API_KEY` がないか空のときは、その変数を示すメッセージを出してテストを飛ばすので、`DEEPSEEK_API_KEY` だけを設定しても実行されない。モデルは `YT2COLUMN_TEST_DEEPSEEK_MODEL` から取り、その変数が定義されていなければ `deepseek-flash` になる（空の値はそのまま渡り、テストを失敗させる）。ターゲットは、統合テストのためだけに同意の変数
`YT2COLUMN_DEEPSEEK_INTEGRATION=1` を export する。これがなければ、テストはその変数を示すメッセージを出して飛ばすので、ただの
`go test -tags integration` や IDE からの実行は API を呼ばない。1 回の実行は、普通の生成と、`MaxOutputTokens` で打ち切った生成の 2 つを行う。それぞれ 15 分の時間切れで区切るので、API が混んでいると数分かかることがある。

### Claude の統合テスト

`make test-integration-claude` は本物の Anthropic の Messages API を呼ぶので、課金される。`YT2COLUMN_TEST_ANTHROPIC_API_KEY` に、テスト用の Anthropic の API キーを設定する必要がある。これは本番の `ANTHROPIC_API_KEY` とは別のもので、テストは本番のキーを読まない。`YT2COLUMN_TEST_ANTHROPIC_API_KEY` がないか空のときは、その変数を示すメッセージを出してテストを飛ばすので、
`ANTHROPIC_API_KEY` だけを設定しても実行されない。モデルと effort は
`YT2COLUMN_TEST_CLAUDE_MODEL` と `YT2COLUMN_TEST_CLAUDE_EFFORT` から取り、定義されていなければ
`claude-haiku-5-5` と `low` になる（空の値はそのまま渡り、テストを失敗させる）。ワークスペース ID は、
`YT2COLUMN_TEST_ANTHROPIC_WORKSPACE_ID` が設定されていればそこから読み、本番の
`ANTHROPIC_WORKSPACE_ID` からは読まない。設定されていなければワークスペース ID を送らない。空の値や形の正しくない値を設定すると、テストは失敗する。ターゲットは、統合テストのためだけに同意の変数
`YT2COLUMN_CLAUDE_INTEGRATION=1` を export する。これがなければ、テストはその変数を示すメッセージを出して飛ばすので、ただの
`go test -tags integration` や IDE からの実行は API を呼ばない。1 回の実行は、普通の生成と、`MaxOutputTokens` で打ち切った生成の 2 つを行う。それぞれ 5 分の時間切れで区切るので、ターゲットの `-timeout` は 15 分である。

### OpenAI の統合テスト

`make test-integration-openai` は本物の OpenAI の Responses API を呼ぶので、課金される。`YT2COLUMN_TEST_OPENAI_API_KEY` に、テスト用の OpenAI の API キーを設定する必要がある。これは本番の `OPENAI_API_KEY` とは別のもので、テストは本番のキーを読まない。`YT2COLUMN_TEST_OPENAI_API_KEY` がないか空のときは、その変数を示すメッセージを出してテストを飛ばすので、
`OPENAI_API_KEY` だけを設定しても実行されない。モデルと effort は
`YT2COLUMN_TEST_OPENAI_MODEL` と `YT2COLUMN_TEST_OPENAI_EFFORT` から取り、定義されていなければ
`gpt-6.1-sol` と `medium` になる（空の値はそのまま渡り、テストを失敗させる）。受け付けない effort の値と、`GODEBUG` の HTTP/2 のデバッグの設定も、要求を送らずにテストを失敗させる。ターゲットは、統合テストのためだけに同意の変数
`YT2COLUMN_OPENAI_INTEGRATION=1` を export する。これがなければ、テストはその変数を示すメッセージを出して飛ばすので、ただの
`go test -tags integration` や IDE からの実行は API を呼ばない。1 回の実行は、普通の生成と、`MaxOutputTokens` を 16 として打ち切った生成の 2 つを行い、どちらも送り直さない。普通の生成が打ち切られたときや、打ち切るはずの生成が完了したときは、テストの前提が成り立たなかったとして失敗する。ログに出すのはモデル名・effort・トークン数（不明ならその理由）・推定の料金・所要時間だけで、応答の文面は出さない。普通の生成では、応答が Standard の処理方式を示すことも確かめる。それぞれ 5 分の時間切れで区切るので、ターゲットの `-timeout` は 15 分である。

テストが使う応答の実例（`testdata/openai_responses_*.json`）の取得の条件と加工は、[testdata/README.md](../../../testdata/README.md) にある。

### CLI の統合テスト

`make test-integration-cli` は、あらかじめ用意した文字起こしのキャッシュから `--out` のファイルまで、CLI 自身が組み立てる処理を実行する。本物の DeepSeek の API を呼び（そのため課金される）、
`yt-dlp` は起動しない。DeepSeek の統合テストと同じく
`YT2COLUMN_TEST_DEEPSEEK_API_KEY` を読み、`YT2COLUMN_TEST_DEEPSEEK_MODEL` が定義されていなければ
`deepseek-flash` を使い、同意の変数
`YT2COLUMN_CLI_INTEGRATION=1` をこのターゲットのためだけに export する。DeepSeek の統合テストと違い、テスト用の API キーがないときはテストを飛ばさずに失敗させるので、同意した実行が API を呼ばずに通ることはない。
