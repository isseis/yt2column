# 開発の手順

本書は yt2column の開発者のための文書である。`make` のターゲット、開発用の別のサーバ、拡張の開発のための検査、localhost サーバの HTTP の API、統合テストを説明する。ブラウザ拡張と localhost サーバの使い方は [README](../../../README.md) に、CLI は [cli.md](cli.md) にある。

## make のターゲット

```sh
make build   # build the three binaries into build/
make test    # unit tests (no network, no yt-dlp)
make lint    # golangci-lint (pinned version)
make fmt     # gofumpt on changed files
make test-integration  # real yt-dlp + network; see below
make test-integration-deepseek  # real DeepSeek API; see below
make test-integration-claude  # real Anthropic API; see below
make test-integration-cli  # the CLI with the real DeepSeek API; see below
make test-integration-slack  # posts fixed articles to a real Mattermost test Webhook; see below
make test-integration-cli-slack  # the CLI with the real DeepSeek API and a real Webhook; see below
```

開発は、受け入れ基準を明示した、要件定義 → 設計 → 実装計画の手順で進める。[CLAUDE.md](../../../CLAUDE.md) と
[docs/dev/conventions/requirements_process.md](../conventions/requirements_process.md) を参照。
設計の文書は日本語で書く。まず
[docs/dev/project_overview.md](../project_overview.md) を読む。

## 開発用に別のサーバを動かす

ソケットのパスは `HOME` だけで決まり、1 つの `HOME` で動かせるサーバは 1 つである。自分のサーバと同時に、ワークツリーからビルドしたサーバを動かすには、`/tmp` の下の短いディレクトリを専用の `HOME` として与え（macOS では、`mktemp -d` の既定のディレクトリではソケットのパスが長すぎる）、そのサーバのソケットにリクエストを送る。

```sh
dev_home="$(mktemp -d /tmp/y2c.XXXXXX)"
HOME="$dev_home" ./build/yt2column-server 2>>"$dev_home/server.log" &
server_pid=$!
socket="$dev_home/.local/state/yt2column/run/server.sock"
if [ "$(uname -s)" = Darwin ]; then socket="$dev_home/Library/Application Support/yt2column/run/server.sock"; fi
until [ -S "$socket" ] || ! kill -0 "$server_pid" 2>/dev/null; do sleep 0.1; done
if [ -S "$socket" ]; then
  curl -sS --unix-socket "$socket" -X GET http://localhost/articles
else
  echo "server exited at startup; $dev_home/server.log:" >&2; cat "$dev_home/server.log" >&2
fi
kill "$server_pid" 2>/dev/null; wait "$server_pid"; rm -rf "$dev_home"
```

待機のループは、サーバが起動の途中で終了したとき（設定の誤り、またはその `HOME` の起動のロックを別のサーバが持っているとき）に待つのをやめる。そのときは、最後の行がログを消すので、リクエストを送る代わりにログを表示する。最後の行は、サーバがまだ動いていてもいなくても実行できる。

`HOME` が異なれば、ワークツリーごとのサーバはほかのサーバと同時に動く。それでも、サーバは LLM と Webhook の変数を利用者の環境から読む。本物の値を設定していると、受け付けたリクエストは記事を生成して投稿する。偽の値（任意の API キーと、`https://example.invalid/hook` のような `https` の Webhook の URL）を設定したときは、ジョブが始まる前に拒否されるリクエストだけを送る。例えば上の誤ったメソッド（`method_not_allowed`）や誤ったパス（`not_found`）である。受け付けられたリクエストは、LLM の API を呼ぼうとし、失敗の通知を投稿する。

拡張は、そのようなサーバにはつながらない。ブラウザは本物の `HOME` で Native Messaging のホストを起動するので、ホストは本物の `HOME` で起動したサーバのソケットにだけつなぐ。ホストの manifest はブラウザごとに 1 つのファイル（macOS では Chrome と Brave で共有する 1 つのファイル）なので、あるワークツリーのビルドからホストを登録すると、別のワークツリーが登録した manifest を置き換える。拡張を使ってビルドを確かめるときは、一度に 1 つのワークツリーだけで行う。

## 拡張の検査

拡張のビルドと読み込みは、README の
[ブラウザ拡張](../../../README.md#ブラウザ拡張)にある。同じ Node.js の版を有効にしたまま、もう 1 つのターゲットで CI と同じ検査を走らせる。

```sh
make ext-check     # typecheck, lint, format check, tests, then build
```

## localhost サーバの API

localhost サーバの起動と停止は、README の
[localhost サーバ](../../../README.md#localhost-サーバ)にある。この節では、サーバの HTTP の API を説明する。

### curl でサーバを確かめる

リクエストの本文（動画の URL とコピーした字幕）を `request.json` に書く。

```json
{"videoUrl":"https://www.youtube.com/watch?v=dQw4w9WgXcQ","transcript":"0:00\nHello\n"}
```

それをソケットに送る。macOS ではパスに空白が入るので、二重引用符で囲んだままにする。

```sh
socket="$HOME/.local/state/yt2column/run/server.sock"
if [ "$(uname -s)" = Darwin ]; then socket="$HOME/Library/Application Support/yt2column/run/server.sock"; fi
curl -sS --unix-socket "$socket" -X POST http://localhost/articles \
  -H 'Content-Type: application/json' \
  --data-binary @request.json
```

`curl` がつながらないとき:

- **サーバが動いていない。** `curl` は `Couldn't connect to server` を報告する
  （そのメッセージは URL のホストと既定のポート、つまり `localhost port 80` を示すが、接続そのものはソケットに向かう）。README の手順でサーバを起動する。ログの最初の行が、待ち受けるソケットを示す。

受け付けたリクエストは `202` と `{"status":"accepted","requestId":"…"}` を返す。
拒否したリクエストは `4xx` か `5xx` と
`{"status":"rejected","reason":"…","requestId":"…"}` を返す。`reason` は次のいずれかである。

| reason | ステータスコード | 拡張の表示 |
|---|---|---|
| `not_found` | 404 | 想定外の応答 |
| `method_not_allowed` | 405 | 想定外の応答 |
| `too_many_requests` | 429 | 固定のメッセージ |
| `body_too_large` | 413 | 固定のメッセージ |
| `invalid_body` | 400 | 固定のメッセージ |
| `invalid_video_url` | 400 | 固定のメッセージ |
| `invalid_transcript` | 400 | 固定のメッセージ |
| `webhook_not_configured` | 503 | 想定外の応答 |
| `invalid_key` | 400 | 想定外の応答 |
| `article_not_ready` | 409 | 想定外の応答 |
| `article_not_found` | 404 | 想定外の応答 |
| `article_not_postable` | 409 | 想定外の応答 |
| `internal_error` | 500 | 固定のメッセージ（`requestId` が空のときは想定外の応答） |

`not_found` と `method_not_allowed` は、誤ったパスかメソッドへの応答である。ジョブを始めず、失敗の通知も投稿しない。今の拡張のメッセージは
`v` のメンバを宣言しないので、ホストはそれを変えずに `POST /articles` へ中継し、拡張はどちらの拒否も想定外の応答として表示する。

### ジョブの API

記事の投稿は、それ自体で 1 つの操作である。今の拡張が送る
`/articles` のリクエストは生成と投稿を一度に行い、下のジョブの API は、記事を生成して返し、求められたときに投稿する。

サーバは `/articles` のほかに、生成と投稿を分ける 3 つの操作を受け付ける。ビューアが記事を表示し、利用者が求めたときだけ投稿できるようにするためである。今の拡張はまだこれらを使わない（#503 で拡張を移す）。
`/articles` はそれまで動き、その後に取り除く。

| 操作 | リクエスト | 成功の応答 |
|---|---|---|
| 生成の開始 | `POST /jobs`、`/articles` と同じ JSON のオブジェクト | `202`、`{"status":"accepted","key":"…","requestId":"…"}` |
| 状態の取得 | `GET /jobs/<key>`、本文なし | `200`、`{"status":"ok","requestId":"…","webhookConfigured":true,"job":{…}}` |
| 投稿 | `POST /jobs/<key>/publish`、本文なし | `202`、`{"status":"accepted","requestId":"…"}` |

`<key>` は記事のキーである。不透明な値なので、生成の開始が返したものをそのまま送り返し、その形にも計算方法にも依存しない。キーはサーバのプロセスごとにも異なる。再起動の後は同じ入力に新しいキーが付くので、新しい生成の開始が返すキーを使う。受け付けない形のキーを指定した状態の取得と投稿のリクエストは
`invalid_key` で拒否する。どちらの操作も本文を取らない。クエリの付いたジョブの API のリクエストには `not_found` を返す。

状態の取得の応答は、ジョブの状態を下のコードのいずれかで示す。`running` のジョブは
`steps` を持つ。`done` のジョブは `steps`（`done` か `skipped` だけ）、
`article`、`publish` を持つ。`failed` のジョブは `steps` と、失敗のコードである `failure` を持つ。工程が `failed` になるのは、生成がその工程で失敗したときだけである。完了の時点の検査の失敗（`article_invalid`・`article_too_large`）と、工程より前の失敗は、どの工程も `failed` にしない。`unknown` のキーは `state` のほかにメンバを持たない。
`article` は、`markdown`（表示用の Markdown）、
`file`（`--out` が書くものとバイト単位で同じ）、`cost`、`durationMs` のメンバを持つ。`durationMs` は、記事を生成した開始を受け付けた時点から、または失敗の後の再試行を受け付けた時点から数える。`cost` は `{"state":"known","usd":"…"}` か `{"state":"unknown"}` である。
`publish` は `{"state":"…"}` の形で、状態が `failed` か `unpostable` のときは `reason` も持つ。
`webhookConfigured` は、このサーバが投稿できるかどうかを示す。

```json
{
  "status": "ok",
  "requestId": "0123456789abcdef",
  "webhookConfigured": true,
  "job": {
    "state": "done",
    "steps": { "draft": "done", "shorten": "skipped", "review": "done" },
    "article": {
      "markdown": "# …",
      "file": "---\n…",
      "cost": { "state": "known", "usd": "0.1234" },
      "durationMs": 81234
    },
    "publish": { "state": "not_posted" }
  }
}
```

同じ入力をもう一度送ると、その記事を生成している間か生成し終えた後は、LLM を呼ばずに同じキーを返す。失敗したキーは生成し直す。同時に走る生成は、`/articles` のジョブも数えて最大 2 つである。生成の開始は 2 つの読み込みの枠の範囲で本文を読み、`/articles` のエンドポイントは生成の枠を 1 つ持っている間だけ本文を読む。そのため、同時に読む本文は最大 4 つである。開始は、読み込みの枠が埋まっているとき、生成の枠が埋まっているとき、またはキャッシュが埋まっていて落とせる記録がないときに拒否する。拒否した開始には
`too_many_requests` を返し、ジョブは始まらない。

記事はサーバのメモリにだけある。記事のキャッシュが持つ記録は最大 100 で、埋まると、完了した記事か失敗のうち最も古いものを落とす。生成中や投稿中の記録は落とさない。再起動するとすべての記事と投稿の状態が失われるので、その後の状態の取得は `unknown` を返す。送る前の検査が拒否する記事は、投稿の状態が `unpostable` になり、投稿できない。そのような記事も `done` として返る（表示も保存もできる）。投稿できる記事になるのは、再起動の後か、その記録をキャッシュが落とした後に生成し直したときだけである。

投稿の操作は Webhook の返事を待たずに `202` を返し、同じ記事を 2 度投稿しない。`posting` か `posted` の記事をもう一度投稿しても何も送らず、`failed` の投稿は最初に届かなかったメッセージから再開する（そのメッセージは 2 度届くことがある）。投稿には `SLACK_WEBHOOK_URL` が要る。なければ、操作は `webhook_not_configured` を返し、状態の取得の応答は
`"webhookConfigured": false` を持ち、`/articles` も同じく拒否する。`SLACK_WEBHOOK_URL` なしで起動したサーバも記事は生成するが、投稿（ジョブの API の投稿の操作と `/articles`）は
`webhook_not_configured` で拒否する。

受け付けて応答した状態の取得はログに書かない。ビューアが数秒ごとに問い合わせるためで、その応答の `requestId` に対応するログの行はない。拒否した状態の取得は、その拒否をログに書く。ほかの操作はどれも `requestId` を付けた行をログに書き、ジョブの API のジョブはキーを書く。

サーバのログのリクエストの行は、応答が返す `requestId` を持つ（README の [サーバの起動](../../../README.md#サーバの起動) を参照）。
ジョブの API の生成と投稿のジョブは、代わりに記事のキーをログに書くので、1 つのジョブは
`grep "$key" server.log` で追う。

最初のシグナルで、サーバはジョブの API の実行中の生成を取り消し（その LLM の呼び出しは止まる。サーバが終わればキャッシュも消えるので、その記事は失われる）、実行中の `/articles` のジョブと投稿のジョブは終わるまで待つ。

「curl でサーバを確かめる」のソケットと `request.json` の本文を使うと、3 つの操作は次のとおりである。

```sh
curl -sS --unix-socket "$socket" -X POST http://localhost/jobs --data-binary @request.json
curl -sS --unix-socket "$socket" -X GET "http://localhost/jobs/$key"
curl -sS --unix-socket "$socket" -X POST "http://localhost/jobs/$key/publish"
```

`$key` は生成の開始が返したキーである。

### ジョブの API のコード

ジョブの API の状態の取得の応答は、それぞれの値を次のいずれかのコードで報告する。

| ジョブの状態 | 意味 |
|---|---|
| `unknown` | サーバがそのキーの記録を持たない |
| `running` | 記事を生成している |
| `done` | 記事を生成し、返した |
| `failed` | 生成が失敗した |

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
| `article_too_large` | 生成した記事が状態の取得の応答に収まらない |

| 投稿の状態 | 意味 |
|---|---|
| `unpostable` | Slack へ送る前の検査が記事を拒否する |
| `not_posted` | 投稿でき、まだ投稿していない |
| `posting` | 投稿している |
| `posted` | すべてのメッセージが届いた |
| `failed` | 最後の投稿が終わりの前に止まった |

| 投稿の理由 | 投稿の状態 | 意味 |
|---|---|---|
| `other` | `unpostable` | メンション以外の、送る前の検査による拒否 |
| `mention` | `unpostable` | 記事がメンションを含む |
| `internal` | `failed` | 投稿のジョブが異常に終わった |
| `webhook` | `failed` | Webhook がメッセージを受け取ったと返さなかった |

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

### CLI の統合テスト

`make test-integration-cli` は、あらかじめ用意した文字起こしのキャッシュから `--out` のファイルまで、CLI 自身が組み立てる処理を実行する。本物の DeepSeek の API を呼び（そのため課金される）、
`yt-dlp` は起動しない。DeepSeek の統合テストと同じく
`YT2COLUMN_TEST_DEEPSEEK_API_KEY` を読み、`YT2COLUMN_TEST_DEEPSEEK_MODEL` が定義されていなければ
`deepseek-flash` を使い、同意の変数
`YT2COLUMN_CLI_INTEGRATION=1` をこのターゲットのためだけに export する。DeepSeek の統合テストと違い、テスト用の API キーがないときはテストを飛ばさずに失敗させるので、同意した実行が API を呼ばずに通ることはない。

### Webhook の統合テスト

`make test-integration-slack` は、固定の 2 つの記事（LLM は呼ばない）を、Mattermost のテスト用のチャンネルの本物の Incoming Webhook に投稿するので、
`YT2COLUMN_TEST_SLACK_WEBHOOK_URL` にそのチャンネルの Webhook の URL を設定する必要がある。これは本番の `SLACK_WEBHOOK_URL` とは別のもので、テストは本番の URL を読まない。
ターゲットは、統合テストのためだけに同意の変数 `YT2COLUMN_SLACK_INTEGRATION=1` を export する。これがなければ、テストはその変数を示すメッセージを出して飛ばすので、ただの `go test -tags integration` や IDE からの実行は投稿しない。テスト用の Webhook の URL がないか正しくないとき、または `GODEBUG` が `http2debug` を有効にしているときは、テストを飛ばさずに失敗させ、何も投稿しない。投稿するメッセージの本文は URL を含まない。失敗のメッセージでは、URL、そのパス・クエリ・userinfo、URL の末尾の 8 文字を伏せる。通信の失敗のメッセージには宛先のホストとポートが出ることがあるが、ホストとポートは秘密情報として扱わない。

### CLI と Webhook の統合テスト

`make test-integration-cli-slack` は、`--slack` を付けて、CLI 自身が組み立てる処理を実行する。本物の DeepSeek の API で記事を生成し（そのため課金される）、本物のテスト用の Webhook に投稿する。`yt-dlp` は起動しない。
`YT2COLUMN_TEST_DEEPSEEK_API_KEY` と `YT2COLUMN_TEST_SLACK_WEBHOOK_URL` の両方が要り、どちらかがなければテストは失敗する。ほかの CLI のターゲットと同じく、
`YT2COLUMN_TEST_DEEPSEEK_MODEL` が定義されていなければ `deepseek-flash` を使い、
`YT2COLUMN_CLI_SLACK_INTEGRATION=1` をこのターゲットのためだけに export する。
