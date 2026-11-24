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
| メンション | mention | Mattermost などのチャットで、`@` に続く名前を通知や強調の対象として扱うもの |
| 対象の `@` | target `@` | `@` のうち、先頭にあるもの、または直前が文字でも数字でもないもの。`＠` などの見た目の似た文字と、`@` を表しうる文字参照を含む（要件書 §3.1） |
| メンションの無効化 | mention neutralization | 対象の `@` の直後（文字参照が表す `@` ではその `;` の直後）に U+200B を 1 つ入れること。`@` そのものは変えない。URL のパスの `/@` は除く（要件書 §3.1） |
| 表示用の Markdown | display Markdown | タイトルの見出しの行・チャンネル名の行・本文からなる記事の Markdown。無効化する前の文字列を指す（要件書 §3.1） |
| 無効化した表示用の Markdown | neutralized display Markdown | 表示用の Markdown の対象の `@` を無効化した文字列。記事生成モジュールの結果の `markdown` に入り、ビューアが表示し、コピーする（`NeutralizedDisplay`） |
| コピーする文章 | copied text | ビューアの「Markdown をコピーする」がクリップボードに書く文字列。無効化した表示用の Markdown と同じである |
| 出典リンク | source link | 記事末尾に付与する元動画へのリンク |
| 投稿先 | publisher | `Publisher` interface |
| プロバイダ | provider | LLM の提供元（DeepSeek, Gemini, Claude など） |
| プロンプトテンプレート | prompt template | `prompts/` 以下のファイル |
| キャッシュ | cache | 動画 ID ごとの字幕・info.json の保存 |
| 記録 | record | ジョブの記録。拡張の Service Worker が `chrome.storage.session` に記事の生成ごとに置く値の組（状態、工程の状態、記事、失敗の値、時刻、実行するページ）。ジョブ ID ごとに 1 つ。コピーした字幕と概要欄を持たない |
| ジョブの状態 | job state | ジョブの記録の `state`。`running`・`done`・`failed`・`stopped` |
| 工程 | step | 記事の生成の中の 3 つの処理（`writer.Stage`）。API の `steps` のメンバ。拡張は、`draft` を「下書きの生成」（Draft）、`shorten` を「字数の調整」（Shorten）、`review` を「検証・推敲」（Review）と表示する。パイプラインの「ステージ」（stage）とは区別する |
| 工程の状態 | step state | `pending`・`running`・`done`・`skipped`・`failed` |
| 生成の枠 | generation slot | 同時に走る記事の生成の数の上限（`2`）。Service Worker が、走っているジョブの記録の数で数える |
| 完了の時点の検査 | completion check | 記事生成モジュールが記事を `done` にする前に行う検査（公開できる形か、結果の JSON が上限に収まるか） |
| 動画 ID | video ID | YouTube の 11 文字の ID |
| ステージ | stage | パイプラインが順に呼ぶ処理の単位（`TranscriptSource`・`ArticleWriter`・`Publisher`。`pipeline.Stage` の `transcript`・`write`・`publish`）。`ArticleWriter` の中の生成・字数の調整・検証・推敲の 3 つは「工程」（step、`writer.Stage`）と呼び、区別する。英語では、生成の後の 2 つ（字数の調整と検証・推敲）を "post-generation steps" と呼ぶ。旧称: 段階 |
| 主経路 | primary path | ブラウザ拡張がコピーした字幕を集め、拡張の記事生成モジュールがビューアのタブの中で記事を生成し、拡張のビューアに表示する経路。保存とコピーは、利用者がビューアで選ぶ |
| 補助の経路 | development path | CLI が動画の URL を受け取り、yt-dlp で字幕を取得する経路。開発・検証に使う |
| 本番の構成 | production profile | 生成・字数の調整・検証・推敲のテンプレート・プロバイダ・モデル・effort の組。`docs/dev/production_profile.md`。文脈により profile と略す |
| 推奨構成 | recommended configuration | `generation.md` の推奨構成の小節が、本番の構成を環境変数で示したもの |
| 品質の目標 | quality target | 0008 F-006 |
| 製品の制約 | product limitations | 本番の構成で起こりうる記事の誤りや欠け。0008 評価記録 §6.3 |
| 構成要素パッケージ | component package | パイプラインの構成要素（`TranscriptSource`・`ArticleWriter`・`Publisher` の 3 つのステージと、`ArticleWriter` が使う `LLMClient` の 4 つ）の interface とデータ型を持つパッケージ（`internal/transcript`・`internal/llm`・`internal/writer`・`internal/publisher`）。リーフとは限らない。ステージはパイプラインが順に呼ぶ 3 つ、構成要素はそれに `LLMClient` を加えた 4 つ、工程は `ArticleWriter` の中の生成・字数の調整・検証・推敲を指す。パイプラインの流れ（順序、どこで失敗したか、`StageError`）を述べるときはステージ、部品の分け方・置き場所（パッケージ、interface、fake）を述べるときは構成要素、記事の生成の中の処理を述べるときは工程を使う。`LLMClient` を含むかどうかが、構成要素とステージの区別の決め手である。一般の語としての「構成要素」（`README.md` の「構成要素の組み立て」など）は、この定義の外である |
| リーフパッケージ | leaf package | 依存先を持たないパッケージ（例: `internal/llm`）。旧称: 葉パッケージ |
| 秘密情報 | secret | API キーなど。`Secret` 型で保持する |
| ホスト | host | 以前の版の Native Messaging のホスト。ブラウザが拡張の送信のたびに起動し、1 つのリクエストを localhost サーバへ中継した実行ファイル（`yt2column-nmhost`）。撤去した。README の移行の手順で使う。文脈が曖昧なときは "native messaging host" とする |
| 結果不明 | result unknown | 生成の開始から 60 秒たっても予約の結果が届かず、記事の生成が始まったかどうか分からないこと。拡張は生成の開始を自動では送り直さない。同じ選択範囲でもう一度押しても、その記事が処理中か完成していれば二重に生成されないことを示す |
| 拒否の理由 | reject reason | 応答の本文の `reason` に載せる固定の値（`rejectReason`） |
| 動画メタデータ | video metadata | 拡張が動画ページの DOM から読む動画タイトル・チャンネル名・概要欄の総称。API の任意フィールド `title`・`channelName`・`description` |
| 動画タイトル | video title | 動画ページの DOM にある動画のタイトル。タブのタイトル（tab title）とは別 |
| チャンネル名 | channel name | 動画を投稿したチャンネルの名前 |
| 概要欄 | description | 動画の説明文。文脈が曖昧なときは "video description" とする |
| 促しのメッセージ | expand prompt | 概要欄が折りたたまれているとき、ポップアップで「もっと見る」（"...more"）を開いてから開き直すよう促す固定の文言（`expandPromptMessage`） |
| セクション | section | ポップアップの縦の区分。動画のセクション・確認のセクション・操作のセクションの 3 つ |
| 動画のセクション | video section | 見出しとチャンネル名を表示するセクション |
| 見出し | heading | 動画のセクションの先頭に表示する、動画タイトル（取れないときはタブのタイトル） |
| 節の見出し | section heading | ビューアが描画する記事の中の、節の見出し（Markdown の `##` の行）。動画のセクションの見出しとは別 |
| 自動リンク | autolink | 記事の中の `<https://…>` の形の記法。ビューアが条件を満たすものだけをリンクにする |
| 確認のセクション | checks section | 選択範囲の行と概要欄の行を表示するセクション |
| 操作のセクション | action section | 主ボタンと、選択の状態のときは副次のボタン、状態の変化の要約を表示するセクション |
| 確認の行 | check row | 確認のセクションの 1 行。✓ か ⚠ の状態を持つ。選択範囲の行と概要欄の行がある |
| 選択範囲の行 | selection row | 選択範囲の文字数と行数、先頭と末尾のプレビューを表示する確認の行 |
| 概要欄の行 | description row | 送る概要欄が全文か、冒頭だけか、送られないかを表示する確認の行 |
| 主ボタン | primary button | 操作のセクションのボタン。送る前は送信を始め（`Generate article`）、送った後は状態に応じて「ビューアを開く」（`Open viewer`）か「やり直す」（`Try again`）の操作になる。選択の状態では「既存の記事を表示する」（`Open existing article`）、選択の状態を終えた後は「記事を生成する」（`Generate article`）になる。送信中と、キーを受け取れなかった結果（拒否・未送信・結果不明）の表示では押せない。選択範囲が送れないときと、収集を拒否した表示でも押せない |
| 詳細 | details | URL とタブのタイトルを入れた、開閉できる区分 |
| ビューア | viewer | 記事の状態と記事を表示する拡張のページ（`viewer.html`）。タブで開く。記事の `.md` での保存・Markdown のコピーの操作を持つ |
| Service Worker | service worker | Manifest V3 の拡張のバックグラウンドの処理を行うスクリプト（`background.js`）。ポップアップの求めを受けて、生成の開始を送る |
| ビューアの URL | viewer URL | ビューアのタブの URL。予約の前の開始の ID か予約の後のジョブ ID、集めた URL、ページのタイトル（動画のタイトルを 64 コードポイントまでに切ったもの）を持つ。既存の記事のビューアは、既存の記事を表示していることを示す値（`e=1`）も持つ。コピーした字幕と概要欄は持たない |
| 送信の時刻 | send time | 利用者が生成の開始を送る操作（主ボタン、「現在の選択範囲で生成する」、選択の状態を終えた後の「記事を生成する」）を押した時刻（「やり直す」のときは、押し直した時刻）。「既存の記事を表示する」が新しいタブを開くときは、押した時刻をそのビューアの送信の時刻とする。処理中の経過時間の起点。ただし既存の記事のビューアは経過時間を示さないので、起点として使わない |
| 同じ動画の照会 | same-video inquiry | 生成の開始で、同じ動画（動画 ID が同じ）の記事があれば生成せずに知らせるよう求めること。照会付きか照会なしかは、生成の開始の本文の必須のメンバ `mode`（`inquiry`・`plain`）で宣言する |
| 送り方 | send mode | 生成の開始が照会付きか照会なしか。ポップアップが Service Worker へ送る生成の開始の `mode` の値（`inquiry`・`plain`）で、拡張の `SendMode` が表す |
| 照会付きの生成の開始 | inquiry start | `mode` が `inquiry` の生成の開始。主ボタンの「記事を生成する」、選択の状態を終えた後の「記事を生成する」、それらの送信をやり直した送信で送る。`mode` が `plain` のものは照会なしの生成の開始（plain start）と呼ぶ |
| 選択の状態 | choice | 同じ動画の応答を受け取った後のポップアップの状態。「既存の記事を表示する」（`Open existing article`）と「現在の選択範囲で生成する」（`Generate from current selection`）を選べる。確認のセクションとは別のもの |
| 既存の記事 | existing article | 同じ動画の応答のキーの記事 |
| 既存の記事のビューア | existing-article viewer | URL に、既存の記事を表示していることを示す値（`e=1`）を持つビューア。表示する記事は、そのビューアの直前の押下より前に受け付けた記事である。処理中の経過時間を示さない |
| 副次のボタン | secondary button | 選択の状態で、主ボタンの下に置く「現在の選択範囲で生成する」のボタン |
| 現在の選択範囲 | current selection | ポップアップを開いたときに集めた選択範囲。押した時点で集め直したものではない |
| 拒否のカード | rejection card | 収集が拒否されたときに、収集の拒否の理由の文と手順をまとめて表示する 1 枚の区分 |
| 収集の拒否の理由 | collection rejection reason | 拡張が収集を拒否する理由（`RejectionReason`。`not-watch-page` など） |
| 先頭のプレビュー | head preview | 選択範囲の最初の 100 コードポイントの表示 |
| 末尾のプレビュー | tail preview | 選択範囲の最後の 100 コードポイントの表示 |
| 選択範囲 | selection | 利用者が文字起こしパネルで選んだ文字列。記事生成モジュールへはコピーした字幕として渡す。既存の大きすぎるときの文言では「選択したテキスト」と呼ぶ（AC-11 の文言は変えない） |
| 支援技術 | assistive technology | スクリーンリーダーなど、画面の要素の名前と状態を利用者に伝える技術 |
| フォーカスの表示 | focus indicator | キーボードの操作の対象になっている要素を示す、枠などの見た目 |
| リクエストの本文 | request body | 生成の開始の入力の JSON（`videoUrl`・`transcript` と動画メタデータ）。概要欄の文章とは別 |
| 片付け | cleanup | `/mergepr` のマージ後処理（base の更新、ローカル head ブランチの削除） |
| 材料 | material | `/mergepr` がコミットメッセージの下書きに使う PR 情報（log.txt・stat.txt・body.txt と、必要なときに読む diff）。PR のタイトルは含めない |
| 件名 | subject | コミットメッセージの 1 行目 |
| 接頭辞 | prefix | conventional-commit の `<type>(<scope>): ` または `<type>: ` の部分。PR のタイトルでも ASCII の英語で書く |
| 受け付ける接頭辞 | accepted prefix | `.claude/commands/_context.md` の「PR title and squash subject」の表に合う接頭辞。合わないものは直さず、ないものとして扱う |
| 要約 | summary | PR のタイトルまたは件名のうち、接頭辞の後の部分。PR のタイトルでは日本語、件名では英語。日本語の要約にも、コマンド名・ファイル名・識別子を表記を変えずに含めてよい |
| プランの PR | plan PR | `/runplan` が開く、実装計画書に基づく PR。判定は `.claude/commands/_context.md` の「Plan PR」 |
| 通常の進め方 | full process | 要件定義書・アーキテクチャ設計書・実装計画書を 1 本ずつ別の PR で作り、前の文書が `approved` になってから次の文書を書く進め方。進め方の記録が `` `full` `` か、記録の行がないタスク（`requirements_process.md`「Review Flow」） |
| 簡易な進め方 | simple process | 3 文書を変更の規模に合わせた簡易な内容で、`/mkreq` が 1 本の PR で作る進め方。人は 3 文書を 1 回のマージの質問で承認する。進め方の記録が `` `simple` `` のタスク（`requirements_process.md`「Simple Process」） |
| 進め方の記録 | process record | 要件定義書の状態表の `Process` 行。値は `` `simple` `` か `` `full` ``。値と記録の状態は `requirements_process.md`「Document Status Format」の「Process record」に定める |
| 対処 | remedy | 問題が起きたときに開発者が取る手順。見出しでは "What to Do When ..." とする |
| 高圧縮ファイル爆弾 | decompression bomb | 展開すると膨大なサイズになり、メモリなどを使い切らせる圧縮データ、およびそれを使った攻撃。HTTP 応答の gzip も含む。旧称: 圧縮爆弾 |
| フェーズ | phase | `/fixpr` を構成する処理の単位（Phase 1〜6 と、その間の Phase 3.5・4.5。取得・トリアージ・修正・範囲の確認・build と push・ローカルのレビュー・返信と解決・PR の説明の確認）。パイプラインの「ステージ」（stage）とは区別する |
| 信頼する著者 | trusted author | リポジトリへの書き込み権限（write・maintain・admin）を持つ、Bot でないアカウントと、自動レビュアーの一覧のどれかの名前の Bot のアカウント |
| 自動レビュアー | automated reviewer | PR にレビューのスレッドを投稿する Bot |
| 自動レビュアーの一覧 | automated reviewer list | `.claude/commands/_context.md` の「Automated reviewer list」に定める、スレッドを信頼する自動レビュアーの名前（GraphQL の `author.login` の形）の一覧。`fixprguard` には `--reviewer` で 1 つずつ渡す |
| 完了を待つ PR レビュアー | awaited PR reviewer | `/runpr` がレビュー済みの判定に使う Bot。承認を待つのではなく、レビューがそのコミットまで完了したことを待つ。`.claude/commands/_context.md` の「Awaited PR reviewer」に、自動レビュアーの一覧とは別に定める |
| 割り当て | quota | 自動レビュアーが一定時間に受け付けるレビューの回数の上限 |
| 手動の PR レビュー | manual PR review | 依頼のコメントで始まる CodeRabbit の PR レビュー。依頼のコメントは、利用者のアカウントが書いた、前後の空白を除く本文が `_context.md` の「PR review request」の値と等しいコメント（0118 要件定義書 3.1） |
| 完了の印 | completion marker | 手動の PR レビューが、あるコミットまでのレビューを終えたことを示す、CodeRabbit の CheckRun の値。`fixprguard review-state` が読む（0118 設計書 3.3） |
| 試行 | attempt | ローカルの CLI レビューを 1 回試みること。CodeRabbit CLI の実行 1 回から成る。範囲ごとに最大 2 回（タスク 0049） |
| 指摘 | finding | レビューが挙げる個々の問題。スレッドとして投稿されるものと、レビューの本文にだけ書かれるものがある |
| （行為の動詞で書く） | cover | レビュー・テスト・検査が、コミットやコードに及ぶこと。「覆う」とは訳さず、その行為の動詞で書く（例: the CLI did not review the commit →「CLI がレビューしなかったコミット」、a test covers the branch →「テストがその分岐を確かめる」）。名詞の coverage は「カバレッジ」 |
| スレッド | thread | PR のファイル・行に紐づくレビューコメントの連なり。`/fixpr` が処理する単位 |
| 信頼する著者のスレッド | trusted thread | すべてのコメントを取得でき、そのすべての著者が信頼する著者と確かめられたレビュースレッド |
| 想定外の著者のスレッド | untrusted thread | コメントの 1 件以上が信頼する著者のものでない、または著者を確かめられないレビュースレッド。本文を取得せず、名指しされるまで処理しない |
| 範囲外の変更 | out-of-scope change | 修正のフェーズが変えたファイルのうち、処理したスレッドの `path` から決まる範囲に入らないもの |
| 実行関連ファイル | execution-related file | `fixprguard` の範囲の規則が、スレッドの著者と `path` にかかわらず範囲外とするファイル。build checks と CI が設定として読む、またはコンパイル・実行するファイル、指示ファイル、`.` で始まる要素を含むパスの 3 つからなる。旧称: コマンドの実行に関わるファイル（`fixprguard_guide` では初出で併記する） |
| interaction limits | interaction limits | GitHub のリポジトリまたはアカウントの設定で、許可した範囲の外のアカウントによる、コメント・issue・PR の作成を一定の期間制限するもの。`collaborators_only` は、書き込み権限を持つ人（collaborators）以外のアカウントを制限する。期限は最長 6 か月で、期限が来ると自動的に解除される。本リポジトリではアカウント全体の設定で有効にする（`fixprguard_guide` 10 章） |
| 作成の制限 | creation restriction | PR の作成と issue の作成を collaborators に限る、期限のないリポジトリの設定。コメントは制限しない（`fixprguard_guide` 10 章） |
| push の制限 | push restriction | 既定のブランチを除くすべてのブランチを対象にした ruleset で、ブランチの作成と更新を bypass の権限を持つ相手（Repository admin）に限る設定（`fixprguard_guide` 10 章） |
| 拒否リスト | deny list | 実行関連ファイルの定義を、名前・拡張子・パスの要素の一覧として見たときの呼び方 |
| 許可リスト | allow list | `fixprguard` の範囲の規則で、同じディレクトリとタスクのディレクトリの下（規則 2・3）で範囲に入りうるファイルの種類 |
| 指示ファイル | instruction file | Claude Code や自動レビュアーが、置かれたディレクトリへの指示として自動で読むファイル（`CLAUDE.md`・`AGENTS.md` など） |
| Go のビルドの入力 | Go build input | Go がパッケージのビルドで、`.go` のファイルのほかに扱うファイル（`.c`・`.s`・`.syso` など） |
| ガードツール | guard tool | `/fixpr` と `/runpr` が判定と書き込みを任せる開発ツール `fixprguard` |
| 開発ツール | developer tool | `cmd/<名前>` と `internal/<名前>` の組で作る、開発手順のための Go 製 CLI ツール（実行コマンド・バイナリ）。製品のバイナリには入らない（`docs/dev/conventions/dev_tools.md`）。旧称: 開発の道具（ADR の経緯と終わったタスクの文書には、旧称が記録として残る） |
| インストール型ツール | installed tool | 開発ツールのうち、利用者が `make install-<名前>` で `main` からローカルにインストールして使う CLI ツール。旧称: 入れる道具（ADR の経緯と終わったタスクの文書には、旧称が記録として残る） |
| 都度ビルド型ツール | per-run tool | 開発ツールのうち、make のターゲットが実行のたびに一時ディレクトリへビルドして使う CLI ツール。旧称: そのつどビルドする道具（ADR の経緯と終わったタスクの文書には、旧称が記録として残る） |
| CI 専用ツール | CI tool | 開発ツールのうち、GitHub Actions ワークフロー内から `go run` で直接実行する CLI ツール（例: `ciskip`）。ローカルへのインストール（`make install-*`）やプロトコル番号の対象外 |
| 区切りのブロック | fenced block | 信頼できないテキストを、実行ごとの乱数（nonce）を含む `BEGIN-UNTRUSTED-<nonce>` と `END-UNTRUSTED-<nonce>` の行で囲んだもの |
| 指紋 | fingerprint | スレッドの ID と、コメントの ID・著者・時刻、コメントの数、続きのページの有無から作る識別子。コメントの追加・編集・削除で変わる |
| 基準値 | baseline | `fixprguard begin` が出力する git の状態の識別子の組。セッションが会話に持ち、後のサブコマンドに渡す |
| 監視の一覧 | watch list | `fixprguard` が、git が無視していても基準値と比べるファイルの、パスの形の一覧。範囲の規則の拒否リストとは別に持つ |
| 対象のファイル | target file | git が無視するファイルのうち監視の一覧に当たるもの。`.claude/settings.local.json` は必ず含める |
| 記録のファイル | record file | `fixprguard` が git のディレクトリに書く、対象のファイルごとに状態の digest と引用したパスを 1 行に書いたファイル。`begin` の記録と止まった時点の記録がある |
| 判定した集合 | judged set | `fixprguard inspect` が範囲と照合した変更の集合。その識別子は `judgedSet` |
| 名指し | naming | 利用者が内容を確かめた想定外の著者のスレッドを、スレッド ID と指紋の組で指定して処理を求めること。動詞は "name"（例: a named thread） |
| バックグラウンド | background | 端末の前面でなく裏で動くプロセスやジョブ（例: バックグラウンドのプロセス）。「背景」とは訳さない |
| ラウンド | round | `/chief` が状況を読み取って示す 1 回分（`snapshot` か `tick` の 1 回）。`tick` のラウンドは、ラウンドの記録を作り（`Store.BeginRound`）、条件を満たせば自動送信もする。会話や文章でも「ラウンド」と書き、「周」とは訳さない |
| タスクの進行 | task progression | タスクは、決まった順の作業項目を 1 つずつ進む。通常の進め方の順は、要件定義書の作成 → アーキテクチャ設計書の作成 → 実装計画書の作成 → プランの PR の PR-1 → PR-2 → … である。簡易な進め方では、3 つの文書を 1 つの作業項目（3 つの文書の作成）で作り、その後にプランの PR が続く |
| 作業項目 | work item | タスクの進行の 1 つの単位。まだ終わっていない最初の作業項目を「今の作業項目」（current work item）と呼ぶ |
| 作業項目の表示 | work item display | chief の状況の表の行が示す、今の作業項目か、作業項目でない 3 つの値（-：タスクがない、なし：すべての作業項目が終わった、要確認：今の作業項目を決められない）のどれか。旧称: 進行の位置の文書、文書（chief の状況の表の行の値の意味に限る）。作業項目の表示と状態の詳細の組の旧称は「進行の位置」で、その組は「作業項目の表示と状態の詳細」と書く。ADR の経緯と終わったタスクの文書には、旧称が記録として残る |
| 作業項目の文字列 | work item string | chief の状況の表が作業項目の表示を示す短い語（要件・設計・計画など）。「状態の文字列」と対になる。旧称: 文書の段（chief の状況の表に示す語の意味に限る。0093 の設計書と計画書の「文書の段」は、`origin/main` の文書がどこまで進んだかを表す内部の段（`docStage`）を指し、旧称ではない）。ADR の経緯と終わったタスクの文書には、旧称が記録として残る |
| 状態の詳細 | state detail | chief の状況の表の行の状態を、実行中のコマンドやマージの確認かどうかで細かくしたもの。作業項目のない行にも示す。旧称: 進み具合（chief の状況の表での意味に限る。拡張の工程の進み具合など、ほかの意味の「進み具合」は旧称ではない）。ADR の経緯と終わったタスクの文書には、旧称が記録として残る |
| マージコンフリクト | merge conflict | GitHub が開いている PR の `mergeable` を `CONFLICTING` と返す状態。PR の状態を指す。略して「コンフリクト」と書く。ファイルの中の状態は「衝突」と書き、区別する（0115 要件定義書 3.1） |
| 衝突 | conflict | `/mergemain` が手元で `origin/main` を取り込んだときに、git が解消できずに残したファイルの中の箇所。ファイルの状態を指す。そのうち、両側がファイルの内容を変えたか、両側が同じパスにファイルを足したものを「内容の衝突」（content conflict）と呼ぶ。PR の状態は「マージコンフリクト」と書き、区別する（0115 要件定義書 3.1） |
| 取り込みの道具 | take-in tool | `/mergemain` の決定的な判定と確かめを行う、`mergepr` の 4 つのサブコマンド（`mergemain-begin`・`mergemain-merge`・`mergemain-restore`・`mergemain-push`）の総称（0115 設計書） |
| 解消の送信 | resolution send | chief が `/mergemain <PR>` を送る 1 回の送信。chief が人に尋ねずに送るものと、人が選んで送るものの両方を指す（0115 設計書） |
| 送信の到達点 | sent step | chief の 1 回の送信で、最後に打った入力。何も打っていない、`/clear`、`/model`、`/new`、コマンドのいずれか（`SentStep`、0115 設計書） |
| コンフリクトの段階 | conflict phase | chief の状況の表のコンフリクトの行を、PR の今の head の解消の送信の記録から細かく分けたもの（`ConflictPhase`、0115 設計書）。ここでの「段階」は、パイプラインの「ステージ」の旧称の「段階」とは別の語である |
| 確認 | check | 文書のテストなど 1 つのテストの中の、1 つの条件の検査。1 つのテストは複数の確認を含みうる（タスク 0038）。動詞は「確かめる」。画面の「確認のセクション」「確認の行」の「確認」は UI の区分の名前（checks section・check row）で、この語とは別 |
| 照合する | check against | 2 つのもの（文書とコード、ADR とタスク文書など）を突き合わせ、一致するか、規則に当たるかを確かめること。「A を B と照合する」と書き、何と照合するかを省かない。名詞は「照合」（例: 本書の照合）。「タスクを照らす」のように「照らす」を「照合する」の意味で使わない |
| 料金表 | rate table | `internal/pricing` の `rateTable`。モデルごとの、トークンあたりの料金の表（`pricing_update.md`） |
| 方式 | scheme | 料金表の項目が、料金を選ぶ方法。単一の料金、週ごとのピークの時間帯、プロンプトの長さのしきい値のいずれか（`pricing_update.md`） |
| ピークの時間帯 | peak window | 時刻で料金が変わるモデルで、高い料金が適用される曜日と時刻の範囲（`pricing_update.md`） |
| 出力の上限 | output limit | LLM への 1 回のリクエストで受け付ける出力のトークン数の上限。Claude のアダプタの既定値は `defaultMaxOutputTokens`（`claude_output_limit.md`） |
| 実時間 | wall-clock time | 処理に実際にかかった時間。出力の速さを求める測定に使う（`claude_output_limit.md`） |
| 打ち切り | truncate | LLM の応答が出力の上限に達して途中で終わること。打ち切られた実行の出力は、上限以上であることしか分からない |
| 生成の後の工程 | post-generation steps | 生成の後に既定で走る、字数の調整と検証・推敲の 2 つの工程（`cli.md`） |
| 推定の料金 | estimated cost | 料金表から求めた記事の料金の推定。CLI は標準エラーの `cost:` の行に、`--out` のファイルは `estimated_cost_usd` に書く。請求額とは別 |
| トークンの使用量 | token usage | LLM の API が返したトークンの数。CLI は工程ごとに標準エラーの `tokens:` の行に書く（`cli.md`） |
| 保存した出力 | artifact | `--artifacts` のディレクトリに工程ごとに保存した出力。`--after` で再開するときに再利用する |
| 再開した実行 | resumed run | `--after` で、保存した出力から再開した CLI の実行 |
| 同意の変数 | opt-in variable | 統合テストが実際の API を呼ぶ前に求める環境変数（`YT2COLUMN_DEEPSEEK_INTEGRATION=1` など）。`make` のターゲットがそのテストのためだけに export する（`development.md`） |
| ソケット | socket | UNIX ドメインソケットのファイル |
| manifest | manifest | ブラウザがホストを見つけるための JSON のファイル（ホストの manifest）、または拡張の `manifest.json`。日本語の文でも英語のまま書く |
| 終了コード | exit code | プロセスが終了するときに返す数値 |
| 設定エラー | configuration error | 設定を拒否したときのエラー。CLI では環境変数の値の拒否で、実行が始まる前に終了コード `2` で終わる |
| 設定ファイル | configuration file | 以前の版の localhost サーバが設定を読んだファイル（`$HOME/.config/yt2column/config.env`）。README の移行の手順で、消すファイルとして使う |
| 元のエラーを含めて返す | wrap an error | Go の `%w` などで、元のエラーを `errors.Is`・`errors.AsType` で取り出せる形のまま、新しいエラーに含めて返すこと。エラーについては「包む」と書かない |
| フロントマター | front matter | `--out` のファイルの先頭の YAML の部分 |
| ポップアップ | popup | 拡張のツールバーのアイコンから開く画面 |
| 文字起こしパネル | transcript panel | YouTube の動画ページで、字幕の行を時刻とともに一覧にするパネル |
| モデルの ID | model ID | プロバイダの API に送るモデルの名前。`YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM` の最初の `/` より後の部分。そこに `+` があれば `+` より前の部分 |
| effort の部分 | effort part | `YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM` の値の、最初の `/` より後ろの `+` より後の部分。工程の effort を指定する |
| クイックスタート | Quick start | README の見出し |
| 必要なもの | Requirements | README と `cli.md` の見出し |
| 出力の書式 | Output format | `generation.md` と `cli.md` の見出し |
| 設定 | Configuration | `generation.md` と `cli.md` の見出し。CLI の環境変数による設定 |
| 以前の版からの移行 | Migrating from the earlier version | README の見出し。localhost サーバと Native Messaging のホストを使う版から、拡張だけの版へ移る手順 |
| ブラウザ拡張 | Browser extension | 本文では "browser extension"、文脈が明らかなら "extension" |
| 記事生成モジュール | article generation module | `cmd/yt2column-wasm`（Go の `main` パッケージ）を WebAssembly にビルドした `.wasm` ファイル。拡張の Worker が読み込み、記事を作る関数を JavaScript に公開する。Go で書かれ WASM にコンパイルされたことを強調するときは「WASM にコンパイルされた記事生成モジュール」と修飾する（0090 設計書） |
| オプションページ | options page | 拡張の設定の画面（`options.html`）。Anthropic の API キーとワークスペース ID を保存し、削除する。タブで開く。英語の画面では、ポップアップからの操作を "Settings" と呼ぶ |
| 開発用のビルド | development build | `make ext-build-dev` が `extension/dist-dev/` に作る拡張。fake の LLM を使う開発用の記事生成モジュールを持ち、API を呼ばない。拡張の ID と名前が公開用のビルドと違う。公開用のビルド（`make ext-build` の `extension/dist/`）と区別する |
| ジョブ ID | job ID | Service Worker が予約のときに作る、記事の生成を指す不透明な値（128 ビットの乱数の 32 文字の小文字の 16 進）。入力から計算せず、記事の中身を表さない |
| 実行するビューア | running viewer | 記事の生成を実際に行うビューアのページ。Service Worker が開いたビューアで、Worker で記事生成モジュールを動かす。ジョブの記録を表示するだけのビューア（表示するビューア）と区別する |
