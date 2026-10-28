# Translation Glossary (Japanese to English)

このファイルは、日本語ドキュメントを英語に翻訳する際に使用する用語集です。一貫性のある翻訳を維持するため、ここに記載された訳語を使用してください。`japrose` による日本語の用語統一の基準にもなります。

## 使用ガイドライン

1. **訳語の優先順位**: この単語帳にある用語は、必ず記載された訳語を使用すること
2. **新規用語の追加**: 新しい用語が必要な場合は、このファイルに追加すること
3. **文脈に応じた選択**: 複数の訳語がある場合は、文脈に応じて適切なものを選択すること
4. **旧称**: 使用しなくなった用語は備考欄に「旧称: 〜」と記載する

---

## 用語

| 日本語 | English | 備考 |
|--------|---------|------|
| 受け入れ基準 | acceptance criteria | `AC-NN` |
| アーキテクチャ設計書 | architecture design | `02_architecture.md` |
| 実装計画書 | implementation plan | `03_implementation_plan.md` |
| 要件定義書 | requirements document | `01_requirements.md` |
| 文字起こし | transcript | `Transcript` 型。字幕から得た本文とメタ情報 |
| 字幕 | subtitles | yt-dlp が出力する字幕ファイル（json3）。UI の文言では、文字起こしパネルの行も指し、英語の UI では captions と書く（例: `Selected captions`） |
| 自動字幕 | auto-generated subtitles | YouTube が自動生成する字幕 |
| 手動字幕 | manual subtitles | 投稿者がアップロードした字幕 |
| コピーした字幕 | copied transcript | 利用者が文字起こしパネルで選択・コピーした文字列。形 A と形 B がある |
| 形 A | form A | 1 セグメント 1 行で、時刻・読み上げ用ラベル・本文が区切りなしで連結された書式 |
| 形 B | form B | 時刻の行と本文の行が分かれた書式 |
| 時刻の行 | timestamp line | パネルの各セグメントの先頭に表示される時刻（例: `0:04`、`1:02:03`） |
| 本文の行 | text line | 時刻の行に続く、そのセグメントの本文（形 B） |
| 読み上げ用ラベル | screen-reader label | 形 A で時刻の直後に連結される、画面読み上げ用にその時刻を読み上げる文字列 |
| コラム記事 | column article | `Article` 型。生成結果 |
| 出典リンク | source link | 記事末尾に付与する元動画へのリンク |
| 投稿先 | publisher | `Publisher` interface |
| プロバイダ | provider | LLM の提供元（DeepSeek, Gemini, Claude など） |
| プロンプトテンプレート | prompt template | `prompts/` 以下のファイル |
| キャッシュ | cache | 動画 ID ごとの字幕・info.json の保存 |
| 記事のキャッシュ | article cache | localhost サーバがメモリに持つ、キーごとのジョブ・工程・投稿の状態と記事（`internal/articlecache`）。ディスクの「キャッシュ」とは別のもの |
| 記事のキー | article key | サーバが生成の開始の入力から計算する、記事を指す不透明な値（64 文字の小文字の 16 進）。拡張とホストは、その形にも計算方法にも依存しない |
| 記録 | record | 記事のキャッシュが 1 つのキーについて持つ値の組（ジョブ・工程・投稿の状態、記事、失敗の理由、時刻） |
| ジョブの状態 | job state | `running`・`done`・`failed`・`unknown` |
| 工程 | step | 記事の生成の中の 3 つの処理（下書き・本文の短縮・検証・推敲。`writer.Stage`）。API の `steps` のメンバ。パイプラインの「段階」（stage）とは区別する |
| 工程の状態 | step state | `pending`・`running`・`done`・`skipped`・`failed` |
| 投稿の状態 | publish state | `not_posted`・`posting`・`posted`・`failed`・`unpostable` |
| 生成の枠 | generation slot | 同時に走る記事の生成の数の上限（`2`）。旧 API のジョブと新しい API の生成が合算する |
| 読み込みの枠 | reading slot | 生成の開始の本文を同時に読み込んで検査する数の上限（`2`） |
| 完了の時点の検査 | completion check | 生成のジョブが記事を `done` にする前に行う検査（公開できる形か、状態の取得の応答に収まるか、Slack へ送る前の検査に通るか） |
| 動画 ID | video ID | YouTube の 11 文字の ID |
| 段階 | stage | パイプラインを構成する処理の単位（`TranscriptSource`・`ArticleWriter`・`Publisher`。`pipeline.Stage` の `transcript`・`write`・`publish`）。`ArticleWriter` の中の生成・字数の調整・検証・推敲の 3 つは、API の `steps` として返すとき「工程」（step、`writer.Stage`）と呼び、区別する。英語では、生成の後の 2 つ（字数の調整と検証・推敲）を "post-generation steps" と呼ぶ |
| 主経路 | primary path | ブラウザ拡張がコピーした字幕を集め、Native Messaging のホストと localhost サーバを経て記事を生成・投稿する経路 |
| 補助の経路 | development path | CLI が動画の URL を受け取り、yt-dlp で字幕を取得する経路。開発・検証に使う |
| 本番の構成 | production profile | 生成・字数の調整・検証・推敲のテンプレート・プロバイダ・モデル・effort の組。`docs/dev/production_profile.md`。文脈により profile と略す |
| 推奨構成 | recommended configuration | README の推奨構成の小節が、本番の構成を環境変数で示したもの |
| 品質の目標 | quality target | 0008 F-006 |
| 製品の制約 | product limitations | 本番の構成で起こりうる記事の誤りや欠け。0008 評価記録 §6.3 |
| 構成要素パッケージ | component package | パイプラインの構成要素（3 つの段階と、`ArticleWriter` が使う `LLMClient`）の interface とデータ型を持つパッケージ（`internal/transcript`・`internal/llm`・`internal/writer`・`internal/publisher`）。リーフとは限らない |
| リーフパッケージ | leaf package | 依存先を持たないパッケージ（例: `internal/llm`）。旧称: 葉パッケージ |
| 秘密情報 | secret | API キー・Webhook URL など。`Secret` 型で保持する |
| 起動のロック | start lock | localhost サーバがソケットのディレクトリのロックのファイルに取る排他の `flock`。起動から、シャットダウン中のジョブの完了までを持ち続け、同じ `HOME` の 2 つのサーバが同時に動くことを防ぐ（`internal/serversock` の `Lock`） |
| ホスト | host | Native Messaging のホスト。ブラウザが拡張の送信のたびに起動し、1 つのリクエストを localhost サーバの UDS へ中継する実行ファイル（`yt2column-nmhost`）。文脈が曖昧なときは "native messaging host" とする |
| ホスト名 | host name | ホストの manifest の `name`・manifest のファイル名・拡張が送る相手として指定する固定の名前（`io.github.isseis.yt2column`、`nmhost.HostName`） |
| ホストの応答 | host reply | ホストが標準出力に書く 1 つのフレーム。サーバの応答を包む形（`kind` が `response`）とホストのエラーの形（`kind` が `error`）がある |
| 未送信 | not sent | 拡張の送信の結果のうち、リクエストがサーバに届いていないと確定するもの（ホストが返す `server_unavailable`・`message_too_large`）。Slack の確認を求めない |
| 結果不明 | result unknown | 拡張の送信の結果のうち、リクエストがサーバに届いた可能性があるもの（応答なし・ホストの失敗・想定外の応答）。送り直す前に Slack を確かめるよう案内する |
| 枠 | slot | localhost サーバが同時に処理するリクエストの数を数える単位。生成の枠（`maxInFlight`）は、旧 API のジョブと新しい API の生成のジョブが合算して数える。新しい API の生成の開始は、本文の読み込みと検査に別の読み込みの枠を使う |
| 拒否の理由 | reject reason | 応答の本文の `reason` に載せる固定の値（`rejectReason`） |
| 動画メタデータ | video metadata | 拡張が動画ページの DOM から読む動画タイトル・チャンネル名・概要欄の総称。API の任意フィールド `title`・`channelName`・`description` |
| 動画タイトル | video title | 動画ページの DOM にある動画のタイトル。タブのタイトル（tab title）とは別 |
| チャンネル名 | channel name | 動画を投稿したチャンネルの名前 |
| 概要欄 | description | 動画の説明文。文脈が曖昧なときは "video description" とする |
| 促しのメッセージ | expand prompt | 概要欄が折りたたまれているとき、ポップアップで「もっと見る」（"...more"）を開いてから開き直すよう促す固定の文言（`expandPromptMessage`） |
| セクション | section | ポップアップの縦の区分。動画のセクション・確認のセクション・操作のセクションの 3 つ |
| 動画のセクション | video section | 見出しとチャンネル名を表示するセクション |
| 見出し | heading | 動画のセクションの先頭に表示する、動画タイトル（取れないときはタブのタイトル） |
| 確認のセクション | checks section | 選択範囲の行と概要欄の行を表示するセクション |
| 操作のセクション | action section | 主ボタンと、送信中と送信の結果を表示するセクション |
| 確認の行 | check row | 確認のセクションの 1 行。✓ か ⚠ の状態を持つ。選択範囲の行と概要欄の行がある |
| 選択範囲の行 | selection row | 選択範囲の文字数と行数、先頭と末尾のプレビューを表示する確認の行 |
| 概要欄の行 | description row | 送る概要欄が全文か、冒頭だけか、送られないかを表示する確認の行 |
| 主ボタン | primary button | 操作のセクションの、送信を始めるボタン（`Generate article`） |
| 詳細 | details | URL とタブのタイトルを入れた、開閉できる区分 |
| 拒否のカード | rejection card | 収集が拒否されたときに、収集の拒否の理由の文と手順をまとめて表示する 1 枚の区分 |
| 収集の拒否の理由 | collection rejection reason | 拡張が収集を拒否する理由（`RejectionReason`。`not-watch-page` など）。サーバが返す拒否の理由（`rejectReason`）とは別 |
| 先頭のプレビュー | head preview | 選択範囲の最初の 100 コードポイントの表示 |
| 末尾のプレビュー | tail preview | 選択範囲の最後の 100 コードポイントの表示 |
| 選択範囲 | selection | 利用者が文字起こしパネルで選んだ文字列。サーバへはコピーした字幕として送る。既存の大きすぎるときの文言では「選択したテキスト」と呼ぶ（AC-11 の文言は変えない） |
| 支援技術 | assistive technology | スクリーンリーダーなど、画面の要素の名前と状態を利用者に伝える技術 |
| フォーカスの表示 | focus indicator | キーボードの操作の対象になっている要素を示す、枠などの見た目 |
| リクエストの本文 | request body | 拡張がサーバへ送る JSON の本文（`videoUrl`・`transcript` と動画メタデータ）。概要欄の文章とは別 |
| リクエスト ID | request ID | サーバがリクエストごとに生成する 16 文字の識別子。応答・ログ・失敗の通知を結び付ける |
| Unix ドメインソケット（UDS） | Unix domain socket (UDS) | ファイルシステム上のパスを介したプロセス間通信。localhost サーバの待ち受け手段。初出で「Unix ドメインソケット（UDS）」と書き、以降は「UDS」とする |
| 片付け | cleanup | `/mergepr` のマージ後処理（base の更新、ローカル head ブランチの削除） |
| 材料 | material | `/mergepr` がコミットメッセージの下書きに使う PR 情報（log.txt・stat.txt・body.txt と、必要なときに読む diff）。PR のタイトルは含めない |
| 件名 | subject | コミットメッセージの 1 行目 |
| 接頭辞 | prefix | conventional-commit の `<type>(<scope>): ` または `<type>: ` の部分。PR のタイトルでも ASCII の英語で書く |
| 受け付ける接頭辞 | accepted prefix | `.claude/commands/_context.md` の「PR title and squash subject」の表に合う接頭辞。合わないものは直さず、ないものとして扱う |
| 要約 | summary | PR のタイトルまたは件名のうち、接頭辞の後の部分。PR のタイトルでは日本語、件名では英語。日本語の要約にも、コマンド名・ファイル名・識別子を表記を変えずに含めてよい |
| プランの PR | plan PR | `/runplan` が開く、実装計画書に基づく PR。判定は `.claude/commands/_context.md` の「Plan PR」 |
| 対処 | remedy | 問題が起きたときに開発者が取る手順。見出しでは "What to Do When ..." とする |
| 高圧縮ファイル爆弾 | decompression bomb | 展開すると膨大なサイズになり、メモリなどを使い切らせる圧縮データ、およびそれを使った攻撃。HTTP 応答の gzip も含む。旧称: 圧縮爆弾 |
| フェーズ | phase | `/fixpr` を構成する処理の単位（Phase 1〜6 と、その間の Phase 3.5・4.5。取得・トリアージ・修正・範囲の確認・build と push・ローカルのレビュー・返信と解決・PR の説明の確認）。パイプラインの「段階」（stage）とは区別する |
| 信頼する著者 | trusted author | リポジトリへの書き込み権限（write・maintain・admin）を持つ、Bot でないアカウントと、自動レビュアーの一覧のどれかの名前の Bot のアカウント |
| 自動レビュアー | automated reviewer | PR にレビューのスレッドを投稿する Bot |
| 自動レビュアーの一覧 | automated reviewer list | `.claude/commands/_context.md` の「Automated reviewer list」に定める、スレッドを信頼する自動レビュアーの名前（GraphQL の `author.login` の形）の一覧。`fixprguard` には `--reviewer` で 1 つずつ渡す |
| 完了を待つ PR レビュアー | awaited PR reviewer | `/runpr` がレビュー済みの判定に使う Bot。承認を待つのではなく、レビューがそのコミットまで完了したことを待つ。`.claude/commands/_context.md` の「Awaited PR reviewer」に、自動レビュアーの一覧とは別に定める |
| 割り当て | quota | 自動レビュアーが一定時間に受け付けるレビューの回数の上限。localhost サーバの同時処理数の単位である「枠」（slot）とは別の概念 |
| 指摘 | finding | レビューが挙げる個々の問題。スレッドとして投稿されるものと、レビューの本文にだけ書かれるものがある |
| スレッド | thread | PR のファイル・行に紐づくレビューコメントの連なり。`/fixpr` が処理する単位 |
| 信頼する著者のスレッド | trusted thread | すべてのコメントを取得でき、そのすべての著者が信頼する著者と確かめられたレビュースレッド |
| 想定外の著者のスレッド | untrusted thread | コメントの 1 件以上が信頼する著者のものでない、または著者を確かめられないレビュースレッド。本文を取得せず、名指しされるまで処理しない |
| 範囲外の変更 | out-of-scope change | 修正のフェーズが変えたファイルのうち、処理したスレッドの `path` から決まる範囲に入らないもの |
| 実行関連ファイル | execution-related file | `fixprguard` の範囲の規則が、スレッドの `path` そのものでなければ範囲外とするファイル。build checks と CI が設定として読む、またはコンパイル・実行するファイル、指示ファイル、`.` で始まる要素を含むパスの 3 つからなる。旧称: コマンドの実行に関わるファイル（`fixprguard_guide` では初出で併記する） |
| 拒否リスト | deny list | 実行関連ファイルの定義を、名前・拡張子・パスの要素の一覧として見たときの呼び方 |
| 許可リスト | allow list | `fixprguard` の範囲の規則で、同じディレクトリとタスクのディレクトリの下（規則 2・3）で範囲に入りうるファイルの種類 |
| 指示ファイル | instruction file | Claude Code や自動レビュアーが、置かれたディレクトリへの指示として自動で読むファイル（`CLAUDE.md`・`AGENTS.md` など） |
| Go のビルドの入力 | Go build input | Go がパッケージのビルドで、`.go` のファイルのほかに扱うファイル（`.c`・`.s`・`.syso` など） |
| ガードツール | guard tool | `/fixpr` と `/runpr` が判定と書き込みを任せる開発用ツール `fixprguard` |
| 区切りのブロック | fenced block | 信頼できないテキストを、実行ごとの乱数（nonce）を含む `BEGIN-UNTRUSTED-<nonce>` と `END-UNTRUSTED-<nonce>` の行で囲んだもの |
| 指紋 | fingerprint | スレッドの ID と、コメントの ID・著者・時刻、コメントの数、続きのページの有無から作る識別子。コメントの追加・編集・削除で変わる |
| 基準値 | baseline | `fixprguard begin` が出力する git の状態の識別子の組。セッションが会話に持ち、後のサブコマンドに渡す |
| 監視の一覧 | watch list | `fixprguard` が、git が無視していても基準値と比べるファイルの、パスの形の一覧。範囲の規則の拒否リストとは別に持つ |
| 対象のファイル | target file | git が無視するファイルのうち監視の一覧に当たるもの。`.claude/settings.local.json` は必ず含める |
| 記録のファイル | record file | `fixprguard` が git のディレクトリに書く、対象のファイルごとに状態の digest と引用したパスを 1 行に書いたファイル。`begin` の記録と止まった時点の記録がある |
| 判定した集合 | judged set | `fixprguard inspect` が範囲と照らした変更の集合。その識別子は `judgedSet` |
| 名指し | naming | 利用者が内容を確かめた想定外の著者のスレッドを、スレッド ID と指紋の組で指定して処理を求めること。動詞は "name"（例: a named thread） |
| バックグラウンド | background | 端末の前面でなく裏で動くプロセスやジョブ（例: バックグラウンドのプロセス）。「背景」とは訳さない |
| 確認 | check | 文書のテストなど 1 つのテストの中の、1 つの条件の検査。1 つのテストは複数の確認を含みうる（タスク 0038）。動詞は「確かめる」。画面の「確認のセクション」「確認の行」の「確認」は UI の区分の名前（checks section・check row）で、この語とは別 |
