# 開発の手順

本書は yt2column の開発者のための文書である。`make` のターゲット、開発用の別のサーバ、拡張の開発のための検査、localhost サーバの HTTP の API、統合テストを説明する。ブラウザ拡張と localhost サーバの使い方は [README](../../../README.md) に、CLI は [cli.md](cli.md) にある。

## make のターゲット

```sh
make build   # build the three binaries into build/
make build-dev  # build the development server on the fake LLM into build/dev/
make test    # unit tests (no network, no yt-dlp)
make lint    # golangci-lint (pinned version) と make wasm-check
make wasm-check  # Go の WASM（GOOS=js GOARCH=wasm）でビルドできることの検査
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

## 開発用に別のサーバを動かす

ソケットのパスと設定ファイルのパスは `HOME` だけで決まり、1 つの `HOME` で動かせるサーバは 1 つである。自分のサーバと同時に、ワークツリーからビルドしたサーバを動かすには、`/tmp` の下の短いディレクトリを専用の `HOME` として与え（macOS では、`mktemp -d` の既定のディレクトリではソケットのパスが長すぎる）、その `HOME` の下に設定ファイルを置いてから起動し、そのサーバのソケットにリクエストを送る。

```sh
dev_home="$(mktemp -d /tmp/y2c.XXXXXX)"
HOME="$dev_home" ./build/yt2column-server config init
printf '%s\n' 'ANTHROPIC_API_KEY=dev-dummy-key' \
  >>"$dev_home/.config/yt2column/config.env"
HOME="$dev_home" ./build/yt2column-server 2>>"$dev_home/server.log" &
server_pid=$!
socket="$dev_home/.local/state/yt2column/run/server.sock"
if [ "$(uname -s)" = Darwin ]; then socket="$dev_home/Library/Application Support/yt2column/run/server.sock"; fi
until [ -S "$socket" ] || ! kill -0 "$server_pid" 2>/dev/null; do sleep 0.1; done
if [ -S "$socket" ]; then
  curl -sS --unix-socket "$socket" -X GET http://localhost/jobs
else
  echo "server exited at startup; $dev_home/server.log:" >&2; cat "$dev_home/server.log" >&2
fi
kill "$server_pid" 2>/dev/null; wait "$server_pid"; rm -rf "$dev_home"
```

待機のループは、サーバが起動の途中で終了したとき（設定の誤り、またはその `HOME` の起動のロックを別のサーバが持っているとき）に待つのをやめる。そのときは、最後の行がログを消すので、リクエストを送る代わりにログを表示する。最後の行は、サーバがまだ動いていてもいなくても実行できる。

`config init` が作った雛形はモード `0600` で、`>>` で行を足してもモードは変わらない。`HOME` が異なれば、ワークツリーごとのサーバはほかのサーバと同時に動く。サーバは LLM の設定を専用の `HOME` の下の設定ファイルから読み、利用者の環境変数の設定の値も、利用者の `HOME` の設定ファイルも読まない（Go の標準ライブラリがプロキシの変数を使うことは、README の「設定ファイル」を参照）。設定ファイルに本物の値を書くと、受け付けた生成の開始は LLM を呼んで記事を生成する。上の例のように偽の値（任意の API キー）を書いたときは、ジョブが始まる前に拒否されるリクエストだけを送る。例えば上の誤ったメソッド（`GET /jobs` への `method_not_allowed`）や誤ったパス（`not_found`）である。受け付けられた生成の開始は、LLM の API を呼ぼうとして失敗する。

拡張は、そのようなサーバにはつながらない。ブラウザは本物の `HOME` で Native Messaging のホストを起動するので、ホストは本物の `HOME` で起動したサーバのソケットにだけつなぐ。ホストの manifest はブラウザごとに 1 つのファイル（macOS では Chrome と Brave で共有する 1 つのファイル）なので、あるワークツリーのビルドからホストを登録すると、別のワークツリーが登録した manifest を置き換える。拡張を使ってビルドを確かめるときは、一度に 1 つのワークツリーだけで行う。

## fake の LLM でサーバを動かす

`make build-dev` は、本番の 3 つのバイナリとは別に、fake の LLM で動く開発用のサーバを `build/dev/yt2column-devserver` に作る。LLM の API も API キーも設定ファイルも要らず、記事の種類・失敗・遅れを起動のときに選べる。使うのは `internal/llm/devfake` の fake の LLM と、本番と同じ `internal/server` である。規則は [security.md](../security.md) の §8.2 にある。

```sh
make build-dev
```

本番のサーバと同じく `HOME` から導出したソケットのパスで待ち受けるので、本番のサーバとは同時に動かせない。先に動いているサーバがあれば、起動のロックを取れずに終了コード `1` で中止する。開発用のサーバを確かめるときは、本番のサーバを止めるか、[開発用に別のサーバを動かす](#開発用に別のサーバを動かす) のとおり専用の `HOME` を与える。

```sh
HOME="$dev_home" ./build/dev/yt2column-devserver
```

引数は次のとおりである。知らない引数・知らない値・繰り返し・余分な語・値の欠けは、何も待ち受けずに、受理する値の案内を標準エラー出力に出して終了コード `2` で終わる。

| 引数 | 意味 | 受理する値 | 省いたとき |
|---|---|---|---|
| `-article` | 記事の種類 | `standard`・`long`・`elements` | `standard` |
| `-fail` | 失敗の種類。`-fail-step` と組で指定する | `empty`・`truncated`・`api-error`・`timeout` | 失敗しない |
| `-fail-step` | 失敗させる工程。`-fail` と組で指定する | `draft`・`shorten`・`review` | 失敗しない |
| `-delay` | LLM の呼び出し 1 回ごとの遅れ（秒） | `0` から `600` までの 10 進の整数 | `0` |

`-fail-step shorten` は、本文が 4,000 字を超える `-article long` のときだけ受理する。`-h`・`-help` は案内を出して終了コード `0` で終わる。

記事のタイトルは `[FAKE] ` で始まる。記事のモデル名は `yt2column-devfake` で単価の表にないので、ビューアは料金を不明として表示する。

待ち受けを始めた後に、fake の LLM で動いていることと選んだシナリオを 1 行で標準エラー出力に書く。例えば `-article long -delay 2` のときの行は次のとおりである（時刻と `socket` のパスはその環境による）。

```text
time=... level=INFO msg=listening socket=/… llm=fake model=yt2column-devfake scenario.article=long scenario.fail=none scenario.delay=2
```

拡張から確かめるときは、本番のサーバを止めてから開発用のサーバを起動し、本番と同じく拡張のポップアップから生成を始め、ビューアで記事を見る。ホストと拡張は変えない。記事が fake のものであることは、タイトルの印と、料金が不明であることで見分ける。

失敗の後の「やり直す」で成功する流れは確かめられない。シナリオは起動のときに決まるので、同じサーバでやり直しても同じ失敗になり、記事のキャッシュはメモリにあるので、起動し直すと失敗したジョブも消える。

遅れの間も生成の枠（2 つ）を使うので、遅れの長いジョブを 2 つ動かしている間の 3 つ目の生成の開始は、本番と同じく `too_many_requests` で拒否される。

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

リクエストの本文（動画の URL、コピーした字幕、送り方）を `request.json` に書く。

```json
{"videoUrl":"https://www.youtube.com/watch?v=dQw4w9WgXcQ","transcript":"0:00\nHello\n","mode":"plain"}
```

それをソケットに送る。macOS ではパスに空白が入るので、二重引用符で囲んだままにする。

```sh
socket="$HOME/.local/state/yt2column/run/server.sock"
if [ "$(uname -s)" = Darwin ]; then socket="$HOME/Library/Application Support/yt2column/run/server.sock"; fi
curl -sS --unix-socket "$socket" -X POST http://localhost/jobs \
  -H 'Content-Type: application/json' \
  --data-binary @request.json
```

これはジョブの API の生成の開始である（下の「ジョブの API」）。

`curl` がつながらないとき:

- **サーバが動いていない。** `curl` は `Couldn't connect to server` を報告する
  （そのメッセージは URL のホストと既定のポート、つまり `localhost port 80` を示すが、接続そのものはソケットに向かう）。README の手順でサーバを起動する。ログの最初の行が、待ち受けるソケットを示す。

受け付けた生成の開始は `202` と `{"status":"accepted","key":"…","requestId":"…"}` を返す。
`mode` が `inquiry` の生成の開始は、同じ動画の記事があれば `200` と `{"status":"same_video","key":"…","requestId":"…"}` を返す（下の「ジョブの API」）。
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
| `invalid_key` | 400 | 固定のメッセージ |
| `internal_error` | 500 | 固定のメッセージ（`requestId` が空のときは想定外の応答） |

`not_found` と `method_not_allowed` は、誤ったパスかメソッドへの応答である。本文を読まず、ジョブを始めない。拡張は、ホストの新しい形のメッセージで操作を指定し、パスもメソッドも指定しないので、拡張・ホスト・サーバの版がそろっていれば、どちらの拒否も受け取らない。受け取れば（例えば、ジョブの API を持たない古いサーバ）、想定外の応答として表示する。

### ジョブの API

サーバの API は、生成の開始と状態の取得の 2 つの操作（ジョブの API）である。記事を生成して返す。生成の開始は拡張の Service Worker が、状態の取得はポップアップとビューアが送る。

| 操作 | リクエスト | 成功の応答 |
|---|---|---|
| 生成の開始 | `POST /jobs`、上の `request.json` の形の JSON のオブジェクト | `202`、`{"status":"accepted","key":"…","requestId":"…"}`。同じ動画の応答は `200`、`{"status":"same_video","key":"…","requestId":"…"}` |
| 状態の取得 | `GET /jobs/<key>`、本文なし | `200`、`{"status":"ok","requestId":"…","job":{…}}` |

生成の開始の本文は、必須のメンバ `videoUrl`・`transcript`・`mode` と、任意のメンバ `title`・`channelName`・`description` を持つ。`mode` は送り方で、`inquiry`（照会付き）か `plain`（照会なし）のどちらかの文字列である。`mode` がない本文、文字列でない値、この 2 つ以外の値（大文字や前後の空白を含むもの、空の文字列を含む）、`mode` を 2 回書いた本文は、どちらかに補わずに `invalid_body` で拒否し、ジョブを始めない。`mode` はキーの計算に入らないので、同じ入力はどちらの送り方でも同じキーになる。

照会付きの生成の開始は、次の順に振る舞いを決める。順 2 は件数の上限と生成の枠より前にあるので、生成の枠に空きがなくても、キャッシュに落とせる記録がなくても、同じ動画の応答を返す。

| 順 | 条件 | 応答 |
|---|---|---|
| 1 | 今の入力のキーの記事が生成中か完了 | 照会なしと同じく、`202` でそのキー |
| 2 | 同じ動画（URL の書き方によらず、動画 ID が同じ）の別のキーの記事が生成中か完了 | `200` の同じ動画の応答。キーは、その記事のうち受け付けた時点（失敗の後の再試行では再試行を受け付けた時点）が最も新しいもの。生成を始めず、キャッシュも変えない。今の入力のキーが失敗でも、再試行しない |
| 3 | それ以外（失敗した記事しかない場合を含む） | 照会なしと同じ（生成を始めて `202`、または `too_many_requests`） |

照会なしの生成の開始は、同じ動画の記事があっても今の入力の記事を生成し、同じ動画の応答を返さない。同じ動画の応答は、記事の状態を持たない。状態は、返したキーで状態の取得を送って知る。拡張・ホスト・サーバは同じ版で使う。`mode` を知らない前の版のサーバは、照会付きの生成の開始を照会なしとして受け付け、前の版の拡張が送る `mode` のない本文は、このサーバが `invalid_body` で拒否する。

`<key>` は記事のキーである。不透明な値なので、生成の開始が返したものをそのまま送り返し、その形にも計算方法にも依存しない。キーはサーバのプロセスごとにも異なる。再起動の後は同じ入力に新しいキーが付くので、新しい生成の開始が返すキーを使う。受け付けない形のキーを指定した状態の取得のリクエストは
`invalid_key` で拒否する。この操作は本文を取らない。クエリの付いたジョブの API のリクエストには `not_found` を返す。

状態の取得の応答は、ジョブの状態を下のコードのいずれかで示す。`running` のジョブは
`steps` を持つ。`done` のジョブは `steps`（`done` か `skipped` だけ）、
`article` を持つ。`failed` のジョブは `steps` と、失敗のコードである `failure` を持つ。工程が `failed` になるのは、生成がその工程で失敗したときだけである。完了の時点の検査の失敗（`article_invalid`・`article_too_large`）と、工程より前の失敗は、どの工程も `failed` にしない。`unknown` のキーは `state` のほかにメンバを持たない。
`article` は、`markdown`（無効化した表示用の Markdown）、`mentionsNeutralized`（`markdown` に U+200B を 1 つ以上入れたか）、`videoTitle`・`channelName`（無効化した見出しの値。ないときは空文字列）、
`file`（`--out` が書くものとバイト単位で同じ）、`cost`、`durationMs` のメンバを持つ。`durationMs` は、記事を生成した開始を受け付けた時点から、または失敗の後の再試行を受け付けた時点から数える。`cost` は `{"state":"known","usd":"…"}` か `{"state":"unknown"}` である。

```json
{
  "status": "ok",
  "requestId": "0123456789abcdef",
  "job": {
    "state": "done",
    "steps": { "draft": "done", "shorten": "skipped", "review": "done" },
    "article": {
      "markdown": "# …",
      "mentionsNeutralized": true,
      "videoTitle": "…",
      "channelName": "…",
      "file": "---\n…",
      "cost": { "state": "known", "usd": "0.1234" },
      "durationMs": 81234
    }
  }
}
```

同じ入力をもう一度送ると、その記事を生成している間か生成し終えた後は、LLM を呼ばずに同じキーを返す。失敗したキーは生成し直す。同時に走る生成は最大 2 つで、この生成の枠を数えるのはジョブの API の生成のジョブだけである。生成の開始は 2 つの読み込みの枠の範囲で本文を読むので、同時に読む本文は最大 2 つである。開始は、読み込みの枠が埋まっているとき、生成の枠が埋まっているとき、またはキャッシュが埋まっていて落とせる記録がないときに拒否する。拒否した開始には
`too_many_requests` を返し、ジョブは始まらない。

記事はサーバのメモリにだけある。記事のキャッシュが持つ記録は最大 100 で、埋まると、完了した記事か失敗のうち最も古いものを落とす。生成中の記録は落とさない。再起動するとすべての記事が失われるので、その後の状態の取得は `unknown` を返す。完了の時点の検査を通らない記事（`article_invalid`・`article_too_large`）は `failed` になる。

受け付けた生成の開始のログの行は、`requestId`・`key`・`videoId`・`outcome`（`started`・`restarted`・`existing`）と送り方 `mode`（`inquiry` か `plain`）を持つ。同じ動画の応答を返した生成の開始は、`generation start answered with a same-video article` の 1 行を書き、`requestId`、今の入力のキー `key`、`videoId`、`outcome=same_video`、`mode=inquiry`、返した記事のキー `sameVideoKey` を持つ。本文・字幕・`videoUrl` の文字列・動画メタデータは書かない。`grep "$key" server.log` は、返した記事のジョブの行と、それを返した照会の行の両方を見つける。

受け付けて応答した状態の取得はログに書かない。ビューアが数秒ごとに問い合わせるためで、その応答の `requestId` に対応するログの行はない。拒否した状態の取得は、その拒否をログに書く。ほかの操作はどれも `requestId` を付けた行をログに書き、生成のジョブはキーを書く。

サーバのログのリクエストの行は、応答が返す `requestId` を持つ（README の [サーバの起動](../../../README.md#サーバの起動) を参照）。
生成のジョブは、代わりに記事のキーをログに書くので、1 つのジョブは
`grep "$key" server.log` で追う。

最初のシグナルで、サーバは実行中の生成を取り消し（その LLM の呼び出しは戻るまで待つ。サーバが終わればキャッシュも消えるので、その記事は失われる）、処理中のリクエストと取り消した生成が終わるまで待つ。

「curl でサーバを確かめる」のソケットと `request.json` の本文を使うと、2 つの操作は次のとおりである。

```sh
curl -sS --unix-socket "$socket" -X POST http://localhost/jobs --data-binary @request.json
curl -sS --unix-socket "$socket" -X GET "http://localhost/jobs/$key"
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
