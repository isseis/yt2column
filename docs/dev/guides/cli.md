# CLI の使い方

CLI は補助の経路である。LLM の呼び出しと投稿を開発し確かめるために使う。主経路は
[Quick start](../../../README.md#quick-start)の経路である。

本書は、CLI だけが使う、または CLI だけが示すものを説明する。CLI が localhost サーバと共有する環境変数は README の
[Configuration](../../../README.md#configuration)にあり、CLI だけが読む変数は下の[設定](#設定)にある。

## 必要なもの

**CLI だけに必要なもの:**

- `PATH` 上の `yt-dlp`（または `YT2COLUMN_YTDLP_PATH` を設定する）。[yt-dlp の準備](#yt-dlp-の準備)を参照
- `SLACK_WEBHOOK_URL`（`--slack` を使うときだけ）

Go や、選んだプロバイダの API キーなど、ほかに必要なものは README の
[Requirements](../../../README.md#requirements)にある。

## 使い方

```
yt2column [flags] <video URL>
```

フラグは動画の URL より前に置かなければならない。`--out`（キャッシュのディレクトリの外のファイルに記事を書く）と
`--slack`（設定した Webhook に投稿する）のどちらか一方だけが必要である。

| フラグ | 意味 |
|---|---|
| `--out <path>` | 記事を書くファイル。これか `--slack` のどちらかを使う。このファイルはまだ存在してはならず、その親のディレクトリは存在しなければならず、ファイルはキャッシュのディレクトリの外になければならない。 |
| `--slack` | `SLACK_WEBHOOK_URL` に設定した Slack 互換の Incoming Webhook（例: Mattermost）に記事を投稿する。これか `--out` のどちらかを使う。 |
| `--refresh` | キャッシュした文字起こしを使わず、`yt-dlp` を実行し直してキャッシュを置き換える。 |
| `--keep-cache` | 実行が成功した後も、この動画のキャッシュを残す。 |
| `--system-prompt <path>` | システムプロンプトのテンプレートを、組み込みのものではなくこのファイルから読む。 |
| `--user-prompt <path>` | ユーザープロンプトのテンプレートを、組み込みのものではなくこのファイルから読む。 |
| `--no-shorten` | 長すぎる本文を短くせず、最後の長さの検査もしない。 |
| `--no-review` | 記事を元の資料と照らす検証・推敲をしない。 |
| `--review-log <path>` | 検証・推敲の工程の変更の一覧をこのファイルに書く。検証・推敲の工程が必要である。`--out` と一緒に使うときは、そのパスと異なるパスにする。 |
| `--shorten-system-prompt <path>` | 字数の調整のシステムプロンプトのテンプレートを、組み込みのものではなくこのファイルから読む。 |
| `--shorten-user-prompt <path>` | 字数の調整のユーザープロンプトのテンプレートを、組み込みのものではなくこのファイルから読む。 |
| `--review-system-prompt <path>` | 検証・推敲のシステムプロンプトのテンプレートを、組み込みのものではなくこのファイルから読む。 |
| `--review-user-prompt <path>` | 検証・推敲のユーザープロンプトのテンプレートを、組み込みのものではなくこのファイルから読む。 |
| `--artifacts <path>` | 各工程の出力（生成した記事、短くした記事、検証・推敲の一覧とそれを適用した記事）をこのディレクトリに保存し、再開に使う。ディレクトリは存在し、キャッシュのディレクトリの外になければならない。 |
| `--after <stage>` | `stage`（`generation`・`shorten`・`review` のいずれか）より後の工程だけを実行し、その工程の保存した出力を `--artifacts` から再利用する。`--artifacts` が必要である。`--after review` は `--review-log` を受け付けない。 |
| `-h`, `--help` | この使い方を表示して終わる。 |

記事はファイルモード `0o644` で書く。出力先のディレクトリはハードリンクを作れなければならない。公開に
`link(2)` を使うので、ハードリンクに対応しないファイルシステム上の出力先は使えない。リンクに失敗したときは、書き終えた記事が出力先の隣の一時ファイルに残る（そのパスは標準エラーに出力する）。

### 終了コード

| コード | 意味 |
|---|---|
| `0` | 記事を `--out` に書いた、または設定した Webhook に投稿した。キャッシュの片付けについての警告があっても変わらない。`-h`/`--help` も `0` で終わる。 |
| `1` | 実行の失敗。文字起こしの取得、記事の生成、または公開が失敗した。別の実行がキャッシュのディレクトリを占有していた。実行が中断された。`--artifacts` のディレクトリが使えなかった、または保存した出力がない・壊れている・別の実行のものだった。あるいは、記事は公開したが、検証・推敲のログを書けなかった。 |
| `2` | 使い方または設定の誤り。引数か動画の URL が正しくない、環境変数を拒否した、`--artifacts`/`--after` の組み合わせが許されない、または工程を組み立てられなかった。 |

## 生成の後の工程

字数の調整と検証・推敲の工程が何をするか、なぜ費用が増えるかは、README の
[Post-generation steps](../../../README.md#post-generation-steps)にある。

`--no-shorten` では、字数の調整の工程と最後の長さの検査を飛ばす。`--no-review` では、検証・推敲の工程を飛ばす。
`--no-shorten --no-review` は、`--out` の書式を含めて、以前の版の 1 回だけ呼ぶ振る舞いに戻す。

記事に記すモデルの項目は、生成のモデルから始まり、実行した工程ごとに 1 つずつ、字数の調整、検証・推敲の順に加わる（[出力の書式](#出力の書式)を参照）。
無人で実行するときは、
`--review-log <path>` を渡し、検証・推敲の工程が何を変えたかを別のファイルに書き残す。このログは記事を公開した後に書く。書けなかったときは、実行は `1` で終わり、記事を公開済みであることを伝えるメッセージを出す。

`--artifacts <dir>` を付けると、実行は各工程の出力（生成した記事、短くした記事、検証・推敲の一覧とそれを適用した記事）も、工程が終わるたびに、記事を公開する前に、それぞれのファイル（`<video ID>.generation.json`・`.shorten.json`・`.review.json`）に保存する。そのため、後の工程が失敗しても、前の工程の出力はディスクに残る。`--after <stage>` は、保存した工程から再開し、それより後の工程だけを実行する。飛ばした工程では LLM を呼ばず、保存した出力を再利用する。`--after review` は LLM を呼ばず、最後の長さの検査と公開だけを行う。再開した実行も実行した工程の出力を保存するので、`--artifacts` のディレクトリは書き込めなければならない。このディレクトリはキャッシュのディレクトリの外にあり、次の実行でも残っている。再開できるのはそのためである（[キャッシュと同時実行](#キャッシュと同時実行)を参照）。

設定はフラグと関係なく決まるので、検証・推敲の工程の設定は `--no-review` でも検査する。`YT2COLUMN_REVIEW_LLM` は検査し、これが別のプロバイダを選ぶときは、そのプロバイダの変数も検査する。そのプロバイダの API キーは必要であり、`claude` では、設定した effort はこの場合も正しくなければならない（設定しなければ `high` になる）。再開した実行は、実行する工程の設定だけを検査するので、`--after review` には LLM の設定が一切要らない。唯一の例外は廃止した LLM の変数
（`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_PROVIDER`・
`YT2COLUMN_REVIEW_MODEL`）で、どれか 1 つでも設定されていると、`--after review` を含むすべての実行が止まる（[Configuration](../../../README.md#configuration)を参照）。

## トークンの使用量

すべての実行は、LLM の API が返したトークンの使用量を、工程ごとに 1 行ずつ標準エラーに出力する。成功した実行は、ほかの要約の行の後にこれらの行を書く。失敗した実行は、失敗を示す行の前に書く。実行も、保存した出力の再利用もしなかった工程は行を書かない。行の形は次のとおり。

```text
yt2column: tokens: <stage> <origin> known input=<n> cache_read=<n> cache_write=<n> output=<n> <model-field>
yt2column: tokens: <stage> <origin> unknown reason=<reason> <model-field>
```

- `<stage>` は `generation`・`shorten`・`review` のいずれかである。
- `<origin>` は、この実行がその工程の LLM を呼んだときは `run`、その工程を `--after` で保存した出力から再利用したときは `reused` である。
- `known` は 4 つの数が分かっていることを示す。数は順に、`input=`
  （プロンプトのキャッシュから読まず、キャッシュにも書かなかった分）、`cache_read=`（プロンプトのキャッシュから読んだ分）、`cache_write=`（プロンプトのキャッシュに書いた分。この区分を持たない DeepSeek では常に 0）、`output=`（出力。推論と思考を含む）である。`reused` の工程では、保存した出力が記録した数である。
- `unknown` は、使用量を読めなかったか、規則を満たさなかったことを示す。数は書かず、`reason=` が空白を含まない 1 語を示す。`not-read`（使用量を読めなかった。使用量を読む前に失敗した呼び出し、または使用量を報告しないクライアント）、`recorded-unknown`（保存した出力が、その工程を不明として記録していた）、`invalid-rule`（既知のどの規則にも当たらない規則）のいずれか、または、規則を破ったのが `usage` メンバか、読み取るメンバの 1 つであるときの `usage:missing`・
  `usage:duplicate`・`usage:not-an-object`・
  `usage:not-a-non-negative-integer` と、その `usage.<member>:missing`・
  `usage.<member>:duplicate`・`usage.<member>:not-a-non-negative-integer` の形である。
- `<model-field>` は、モデルの名前を示す `model=<model>` か、モデルの名前の規則を満たすモデルを工程が報告しなかったときの固定の語
  `nomodel` である。モデルの名前は、`redact.Line` が標準エラーのすべての行をエスケープするのと同じ方法でエスケープするので、名前の中の `"` と `\` はエスケープされる。元に戻すには、
  `model=` の後のすべてを二重引用符で囲み、`strconv.Unquote` に渡す。設定した秘密情報（またはその末尾の 8 文字）を含むモデルの名前は、伏せ字の印に置き換わり、元に戻せない。

例:

```text
yt2column: tokens: generation reused known input=46 cache_read=0 cache_write=0 output=85 model=deepseek-flash
yt2column: tokens: shorten run known input=1800 cache_read=0 cache_write=0 output=2400 model=deepseek-flash
yt2column: tokens: review run unknown reason=usage.output_tokens:not-a-non-negative-integer model=claude-opus-5-5
yt2column: tokens: generation run unknown reason=not-read nomodel
```

正しくない `usage` だけでは実行は失敗しない。呼び出しは成功し、記事を公開するかどうかと、終了コードが 0 かどうかは、残りの処理で決まる。失敗した呼び出しは実行を失敗させる（終了コードは 0 以外）が、その行が `unknown` になるとは限らない。打ち切り、想定外の終了の理由、空の応答では、拒否した応答にもモデルと使用量が含まれるので、行は `known` になりうる。`reason=not-read` を書くのは、使用量をまったく読めなかったときだけである。

LLM を 1 度も呼ばずに終わった実行（使い方または設定の誤り、文字起こしの取得の失敗、`--after` の入力の拒否、使えない `--artifacts` のディレクトリ、または組み立てられなかった工程）は、トークンの行を書かない。要約を書かずに止められた実行（2 度目のシグナル、`kill -9`、panic）も書かない。

数は、`--artifacts` のファイルにも `usage` の要素として保存する。工程の呼び出しの時刻はその隣に `call_time` の要素として保存する。工程の LLM の呼び出しが戻った時点を、UTC の秒単位で表す。保存した出力の形式は版 3 である。版 1 と版 2 のファイルは再開のときに拒否するので、更新した後は、新しい `--artifacts` のディレクトリを使うか、古いディレクトリを移す。

## 出力の書式

記事はどちらの出力でも同じ文章だが、情報の並べ方は出力ごとに異なる。読み手のための部分（タイトル、チャンネル名、本文、出典リンク）を先に置き、開発のための部分（モデル、推定の料金）は、Slack への投稿では最後に、ファイルではフロントマターに置く。

Slack への投稿は README の
[The Slack post](../../../README.md#the-slack-post)にある。

### `--out` のファイル

ファイルは YAML のフロントマターで始まり、タイトル、チャンネル名（あるとき）、本文が続く。

```text
---
channel: "Example Channel"
source: "https://www.youtube.com/watch?v=abcdefghijk"
model: "deepseek-flash"
model_version: "v4"
shorten_model: "deepseek-flash"
shorten_model_version: "v4"
review_model: "claude-haiku-5-5"
review_model_version: "2026-01-01"
estimated_cost_usd: "0.0094"
---

# Caching transcripts by video ID

Example Channel

The cache key comes from the video ID alone.

## One key per video

Two URLs for one video share one entry.

出典: <https://www.youtube.com/watch?v=abcdefghijk>
```

- キーはこの順に並ぶ。`channel` は動画がチャンネル名を持つときだけ、`shorten_*` は記事が字数の調整の工程を経たとき（今回の実行か、`--after` での再利用）だけ、`review_*` は検証・推敲の工程を経たときだけ現れる。
- どの値も Go の `strconv.Quote` で二重引用符に囲んだ文字列なので、引用符、バックスラッシュ、`:`、`#`、`---` を含む値でも構造を変えられない。`estimated_cost_usd` も文字列で、小数 4 桁か `unknown` である
  （[推定の料金](#推定の料金)を参照）。空のモデルの版は
  `(none)` と書く。
- チャンネル名はタイトルの下にもう一度現れるので、フロントマターを隠す Markdown のビューアでも表示される。Markdown のブロックのように始まる名前
  （`# `、`- `、`> ` など）は、表示した行が名前そのものになるよう、先頭（番号付きの箇条書きの印では数字の後）にバックスラッシュを付ける。先頭の空白は落とす。
- この書式は、モデルと料金をタイトルの下に並べていた以前の書式（`- Model: …`）に代わるものである。

## 推定の料金

LLM の工程を少なくとも 1 つ呼ぶか再利用したすべての実行は、推定の料金も標準エラーに出力する。工程ごとに 1 行、その後に合計の 2 行である。成功した実行は、ほかの要約の行の後にこれらの行を書く。失敗した実行は、失敗を示す行の前に書く。実行も、保存した出力の再利用もしなかった工程は料金の行を書かず、トークンの行を書かない実行は料金の行も書かない。行の形は次のとおり。

```text
yt2column: cost: <stage> <origin> known usd=<amount> <basis> at=<time>
yt2column: cost: <stage> <origin> unknown reason=<reason> at=<time>
yt2column: cost-total: <total> known usd=<amount>
yt2column: cost-total: <total> unknown
```

- `<stage>` と `<origin>` は、トークンの使用量の行と同じ語である。
  `generation`・`shorten`・`review` のいずれかと、`run` か `reused` である。
- `known` の後には、`usd=` と小数 6 桁の金額、その後に `basis`、すなわち推定に適用した料金の区分が続く。`flat`（モデルの料金が 1 つ）、
  `off-peak` か `peak`（DeepSeek の時間帯別の料金の 2 つの時間帯）、または
  `short-prompt` か `long-prompt`（Claude Haiku のプロンプトの長さによる 2 つの段階）である。
  この位置の `invalid` は、`unknown` の下のものと同じく、不変条件が破れたことを示し、正しいビルドでは起こらない。
- `unknown` は、料金を推定できなかったことを示す。数は書かず、
  `reason` が 1 語を示す。`usage-unknown`（工程のトークン数が不明）、`no-model`（工程がモデルの名前を報告しなかった）、`unpriced-model`
  （モデルの名前が料金表にない）である。`invalid` は、CLI が料金のパッケージの不変条件を破ったことを示し、正しいビルドでは起こらない。
- `at` は、推定に使った呼び出しの時刻で、UTC の `YYYY-MM-DDTHH:MM:SSZ` の形である。再利用した工程は、保存した出力が記録した時刻を示す。`none` は不変条件が破れたことを示し、正しいビルドでは起こらない。
- `cost-total` の行は 2 つの合計である。`run` はこの実行の料金で、この実行が呼んだ工程から求める。`article` は記事全体の料金で、それらの工程に `--after` で再利用した工程を加えて求める。合計は、その集合のどれか 1 つの工程が不明なら `unknown` になる。`article` の合計は、生成の工程を実行も再利用もしなかったときも `unknown` になる（記事には必ず生成の工程がある）。`--after` がなければ 2 つの合計は等しい。合計は、丸めた後の工程の金額ではなく、工程の正確な和を丸めるので、表示した工程の行の和と最後の桁で異なりうる。

例:

```text
yt2column: cost: generation reused known usd=0.000158 peak at=2026-10-16T01:12:09Z
yt2column: cost: shorten run known usd=0.001710 off-peak at=2026-10-16T04:02:41Z
yt2column: cost: review run unknown reason=unpriced-model at=2026-10-16T04:03:15Z
yt2column: cost-total: run unknown
yt2column: cost-total: article unknown
```

どちらの出力も記事の料金を、小数 4 桁か `unknown` で持つ。
`--out` のファイルはフロントマターのキー `estimated_cost_usd`（`"0.0023"` か
`"unknown"`）として、Slack への投稿は脚注の末尾として持つ。脚注の形は README の
[Estimated cost](../../../README.md#estimated-cost)にある。

2 つの出力の並びは[出力の書式](#出力の書式)を参照。

推定がプロバイダの請求額と異なりうる理由は、README の
[Estimated cost](../../../README.md#estimated-cost)にある。再利用した工程の料金は、出力を保存した CLI ではなく、再開した CLI の料金表で求める。そのため、料金が変わった後に再開した実行は、同じ保存した出力から異なる記事の料金を示す。保存した出力が記録するのは呼び出しの時刻であり、料金ではない。

localhost サーバからの Slack への投稿は、不明な料金がなぜ不明かを示せない。理由を調べるには、同じモデルの設定で CLI を実行し、料金の行の `reason` の語を読む。

## キャッシュと同時実行

すべての実行はキャッシュのディレクトリに排他のロックを取るので、キャッシュのディレクトリを共有する実行は 1 つずつ動く。最初の実行がロックを持つ間、2 つ目の実行は待たずにすぐ失敗する。異なるキャッシュのディレクトリは互いに影響しない。

`--artifacts` で保存した出力はキャッシュのディレクトリの外にあるので、キャッシュを消しても消えず、再開に使える。
CLI はそれらを間引くことも消すこともない。要らなくなったら、ディレクトリを自分で消す。

CLI が SIGKILL で止められると、`yt-dlp` が動き続けることがある。その `yt-dlp` が終わるまでロックは持たれたままなので、それまで次の実行は、別の実行が動いているときと同じく失敗する（メッセージはロックのファイルを示し、そのファイルに `lsof` を使うよう勧める）。残った `yt-dlp` が終われば、次の実行は成功する。

キャッシュが壊れている（例えば文字起こしや `info.json` が途中で切れている）と、実行はキャッシュが壊れていることを示すメッセージを出して失敗する。`--refresh` を付けて実行し直し、文字起こしを取得し直してキャッシュを置き換える。

保存した `info.json` のチャンネル名は、LLM を呼ぶ前に検査する。空白だけの名前、制御文字（または U+202E のような双方向の制御文字）を含む名前、200 コードポイントを超える名前は、実行を失敗させる。以前の版は受け付けていた。
`--refresh` を付けて実行し、動画を取得し直してキャッシュを置き換える。localhost サーバは、そのような `channelName` を `invalid_body` で拒否する。

## Webhook への投稿

`--slack` を付けると、記事はファイルではなく、`SLACK_WEBHOOK_URL` の Slack 互換の Incoming Webhook
に投稿する。URL の規則、複数のメッセージへの分割、`silent`、メンションになりうる書式の検査は、README の
[Webhook publishing](../../../README.md#webhook-publishing)にある。

それらの規則が拒否する URL は終了コード `2` の設定の誤りになる。変数は設定されていれば必ず検査するので、これは
`--out` の実行も止める。

メンションの対象に U+200B を入れて投稿するとき、
`--out` が書くファイルと記事そのものは変わらない。

記事を拒否したときのエラーは、投稿の中の行と桁を示す（フロントマターで始まる `--out` のファイルとは行が異なる）。実行し直すと別の記事を生成するので、問題がなくなることがある。モデルが生成したものを確かめるには、
`--out` を付けて実行する。

## yt-dlp の準備

yt2column は `yt-dlp` を実行して字幕を取得するので、`yt-dlp` が YouTube に安定してアクセスできる必要がある。それには `yt-dlp` 本体のほかに、2 つの任意の部品が要る。部品がないと
`yt-dlp` が警告を出し、YouTube が HTTP 429
（Too Many Requests）を返しやすくなる。

- JavaScript のランタイム（[deno](https://deno.com/) は `yt-dlp` が既定で有効にするもの）
- ブラウザのなりすましのための [curl_cffi](https://github.com/lexiforest/curl_cffi)

macOS では、Homebrew の formula が両方を入れる（`deno` に依存し、
`curl_cffi` を同梱する）。

```sh
brew install yt-dlp
command -v yt-dlp                   # must be the Homebrew one, not an older copy earlier on PATH
yt-dlp --list-impersonate-targets   # lists targets whose Source is curl_cffi
```

別の `yt-dlp` が `PATH` で先に見つかるときは、それを消すか、
`YT2COLUMN_YTDLP_PATH` で使う方のパスを指定する。

ほかのプラットフォームでは、yt-dlp の
[インストール](https://github.com/yt-dlp/yt-dlp/wiki/Installation)、
[EJS（JavaScript のランタイム）](https://github.com/yt-dlp/yt-dlp/wiki/EJS)、
[なりすまし](https://github.com/yt-dlp/yt-dlp#impersonation)の文書を参照。

## 設定

CLI は README の
[Configuration](../../../README.md#configuration)の変数と、CLI だけが読む次の 2 つの変数を読む。

| 変数 | 説明 | 未設定のとき |
|---|---|---|
| `YT2COLUMN_CACHE_DIR` | 字幕と動画の情報（`info.json`）を保存するキャッシュのディレクトリ | macOS では `$HOME/Library/Caches/yt2column`。ほかの Unix では `$XDG_CACHE_HOME/yt2column` か `$HOME/.cache/yt2column` |
| `YT2COLUMN_YTDLP_PATH` | `yt-dlp` のパス | `PATH` 上の `yt-dlp` |

`yt2column-server` は、`yt-dlp` を呼ばないので、`YT2COLUMN_CACHE_DIR` も `YT2COLUMN_YTDLP_PATH` も読まない。

`SLACK_WEBHOOK_URL` は、`--slack` では必要で、`--out` では任意である（値はなくてよい）。

前後に空白のあるモデルの ID は、工程のクライアントを組み立てるときに、リクエストを送る前に拒否する。CLI は実行が始まる前にクライアントを組み立てる。

`YT2COLUMN_REVIEW_LLM` を設定しなければ、検証・推敲の工程は、検証・推敲だけを行う再開した実行でも、生成のプロバイダとモデルを使う。
`YT2COLUMN_REVIEW_LLM` を設定していれば、検証・推敲だけを行う再開した実行は `YT2COLUMN_LLM` を読まない。

廃止した LLM の変数は、`--after review` のように LLM の設定を読まない実行を含めて、すべての実行を止める。CLI の実行は、始まる前に終了コード `2` で終わる。
README の
[Migrating from the retired LLM variables](../../../README.md#migrating-from-the-retired-llm-variables)を参照。
