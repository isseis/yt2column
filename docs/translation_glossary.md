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
| 字幕 | subtitles | yt-dlp が出力する字幕ファイル（json3） |
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
| 動画 ID | video ID | YouTube の 11 文字の ID |
| 段階 | stage | パイプラインを構成する処理の単位（`TranscriptSource`・`ArticleWriter`・`Publisher`） |
| 構成要素パッケージ | component package | パイプラインの構成要素（3 つの段階と、`ArticleWriter` が使う `LLMClient`）の interface とデータ型を持つパッケージ（`internal/transcript`・`internal/llm`・`internal/writer`・`internal/publisher`）。リーフとは限らない |
| リーフパッケージ | leaf package | 依存先を持たないパッケージ（例: `internal/llm`）。旧称: 葉パッケージ |
| 秘密情報 | secret | API キー・Webhook URL など。`Secret` 型で保持する |
| 起動のロック | start lock | localhost サーバがソケットのディレクトリのロックのファイルに取る排他の `flock`。起動から、シャットダウン中のジョブの完了までを持ち続け、同じ `HOME` の 2 つのサーバが同時に動くことを防ぐ（`internal/serversock` の `Lock`） |
| ホスト | host | Native Messaging のホスト。ブラウザが拡張の送信のたびに起動し、1 つのリクエストを localhost サーバの UDS へ中継する実行ファイル（`yt2column-nmhost`）。文脈が曖昧なときは "native messaging host" とする |
| ホスト名 | host name | ホストの manifest の `name`・manifest のファイル名・拡張が送る相手として指定する固定の名前（`io.github.isseis.yt2column`、`nmhost.HostName`） |
| ホストの応答 | host reply | ホストが標準出力に書く 1 つのフレーム。サーバの応答を包む形（`kind` が `response`）とホストのエラーの形（`kind` が `error`）がある |
| 未送信 | not sent | 拡張の送信の結果のうち、リクエストがサーバに届いていないと確定するもの（ホストが返す `server_unavailable`・`message_too_large`）。Slack の確認を求めない |
| 結果不明 | result unknown | 拡張の送信の結果のうち、リクエストがサーバに届いた可能性があるもの（応答なし・ホストの失敗・想定外の応答）。送り直す前に Slack を確かめるよう案内する |
| 枠 | slot | localhost サーバが同時に処理するリクエストの数を数える単位（`maxInFlight`） |
| 拒否の理由 | reject reason | 応答の本文の `reason` に載せる固定の値（`rejectReason`） |
| 動画メタデータ | video metadata | 拡張が動画ページの DOM から読む動画タイトル・チャンネル名・概要欄の総称。API の任意フィールド `title`・`channelName`・`description` |
| 動画タイトル | video title | 動画ページの DOM にある動画のタイトル。タブのタイトル（tab title）とは別 |
| チャンネル名 | channel name | 動画を投稿したチャンネルの名前 |
| 概要欄 | description | 動画の説明文。文脈が曖昧なときは "video description" とする |
| 促しのメッセージ | expand prompt | 概要欄が折りたたまれているとき、ポップアップで「…もっと見る」（"...more"）を開いてから開き直すよう促す固定の文言（`expandPromptMessage`） |
| リクエスト ID | request ID | サーバがリクエストごとに生成する 16 文字の識別子。応答・ログ・失敗の通知を結び付ける |
| Unix ドメインソケット（UDS） | Unix domain socket (UDS) | ファイルシステム上のパスを介したプロセス間通信。localhost サーバの待ち受け手段。初出で「Unix ドメインソケット（UDS）」と書き、以降は「UDS」とする |
| 片付け | cleanup | `/mergepr` のマージ後処理（base の更新、ローカル head ブランチの削除） |
| 材料 | material | `/mergepr` がコミットメッセージの下書きに使う PR 情報（log.txt・stat.txt・body.txt と、必要なときに読む diff）。PR のタイトルは含めない |
| 件名 | subject | コミットメッセージの 1 行目 |
| 接頭辞 | prefix | conventional-commit の `<type>(<scope>): ` または `<type>: ` の部分。PR のタイトルでも ASCII の英語で書く |
| 受け付ける接頭辞 | accepted prefix | `.claude/commands/_context.md` の「PR title and squash subject」の表に合う接頭辞。合わないものは直さず、ないものとして扱う |
| 要約 | summary | PR のタイトルまたは件名のうち、接頭辞の後の部分。PR のタイトルでは日本語、件名では英語 |
| プランの PR | plan PR | `/runplan` が開く、実装計画書に基づく PR。判定は `.claude/commands/_context.md` の「Plan PR」 |
| 対処 | remedy | 問題が起きたときに開発者が取る手順。見出しでは "What to Do When ..." とする |
| 高圧縮ファイル爆弾 | decompression bomb | 展開すると膨大なサイズになり、メモリなどを使い切らせる圧縮データ、およびそれを使った攻撃。HTTP 応答の gzip も含む。旧称: 圧縮爆弾 |
