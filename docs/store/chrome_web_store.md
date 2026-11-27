# Chrome ウェブストア掲載情報

本書は、Chrome ウェブストアに申請・掲載する情報の確定稿を保管する。表示名と短い説明文は `extension/static/_locales/{ja,en}/messages.json` を正本とし、掲載項目の表はその写しである。詳細説明文は本書を正本とする。

[ADR-0035](../dev/adr/0035-language-policy.md)（文書の言語方針）のとおり、本書は日本語版と英語版の本文を 1 つのファイルに持ち、`/mktrans` と `TestReadmeSyncMarker` による 2 言語同期の対象外である。内容を改めるときは、日本語版と英語版をあわせて更新する。

## 1. 掲載項目

### 1.1 製品名

- **コア製品名（UI・README 本文・日常の呼称）:** 日本語 `ヨムドウガ`、英語 `YomuDoga`。
- **Chrome ウェブストア表示名（`extName`）と開発用のビルド表示名（`extNameDev`）:** 次の 1.2 の表に、`messages.json` の写しとして置く。

コア製品名には `YouTube` を含めない。Chrome ウェブストア表示名はコア製品名を先頭にし、末尾に互換性表記として `for YouTube™` を添える。

### 1.2 表示名と短い説明文

`extension/static/_locales/{ja,en}/messages.json` の `message` の値を写したものである。片方だけを変えるとずれるので、変えるときは両方をあわせて更新する。値のセルは 1 つのコードスパンだけで書く。

| キー | 日本語 | 英語 |
|---|---|---|
| `extName` | `ヨムドウガ for YouTube™` | `YomuDoga for YouTube™` |
| `extNameDev` | `ヨムドウガ（開発用のビルド）` | `YomuDoga (development build)` |
| `extDescription` | `YouTube™ 動画を、さっと読める新聞・雑誌風の日本語コラム記事に。選んだ文字起こしから、AI が読みやすく編集します。` | `Turns YouTube™ videos into readable Japanese magazine-style column articles from your selected transcript with AI.` |

## 2. 商標・審査上の運用条件

- **帰属表示と非公式の明記:** 詳細説明文と README に、Google LLC および YouTube と無関係の非公式ツールである旨と、`YouTube is a trademark of Google LLC.` の帰属表示を明記する。
- **ロゴと意匠:** アイコンやプロモーション画像に YouTube のロゴや、紛らわしい再生ボタンの意匠を使わない。
- **`for YouTube™` の削除要求:** 万一ストアの審査で `for YouTube™` の削除を求められた場合は、コア製品名を変えず、末尾の `for YouTube™` だけを外して対応する。

## 3. 詳細説明文（日本語版）

```text
長い講演や対談、解説動画を見る時間がないときに。
ヨムドウガ for YouTube™ は、YouTube™ 動画の文字起こしを新聞・雑誌風の日本語コラム記事に仕立てるブラウザ拡張機能です。話し言葉の脱線や重複を整理し、編集・校正まで行うことで、動画を見なくても論点がすっと頭に入る読み物を目指しました。

■ こんな方に向いています
・30〜40 分の動画を見る余裕はないが、中身や論点はきちんと把握したい
・箇条書きの簡単な要約ではなく、自然な日本語の「読み物」として読みたい
・英語などの外国語動画を、日本語の記事としてじっくり読みたい
・アカウント登録なしで、プライバシーやセキュリティ面でも安心して使いたい

■ 一般的な「要約ツール」との違い
・結論が先、重要度順に読める構成
  冒頭のリード文に主題と最も重要な結論をまとめ、本文も重要度順に並べます。新聞記事のように、途中で読むのをやめても動画の大枠をつかめます。
・動画に合った形式への編集とチェック
  インタビュー・対談・レビュー・解説コラムから動画に合った形式を選び、字数を整えたうえで、元の文字起こしと突き合わせて誤字や根拠のない記述をチェックします。

■ 使い方（3 ステップ）
1. YouTube™ の動画ページで文字起こしパネルを開き、記事にしたい範囲を選択します。
2. ツールバーの「ヨムドウガ」アイコンを押して生成を始めます。
3. 専用タブに記事が表示されます。その場で読めるほか、文章ファイル（.md 形式）として保存したり、コピーしてメモアプリに貼り付けたりできます。

■ 壊れにくさと安心の設計
・ページを自動操作しません
  あなたが選んだ文字起こしだけを受け取る仕組みのため、YouTube™ の画面変更に強く、安定して使い続けられます。
・プライバシーとセキュリティに配慮した設計
  ボタンを押したときだけ対象ページを読み、ふだんの閲覧内容にはアクセスしません。中継サーバを持たずブラウザから AI サービスへ直接通信し、ソースコードやプロンプトはすべてオープンソースとして公開しています。

■ ご利用前の注意点
・拡張機能は無料ですが、記事の生成にはご自身の Anthropic 社の API キー（有料・使った分だけの従量課金）が必要です。Claude.ai の月額プランとは別契約となり、記事 1 本あたりの目安は約 0.4 米ドルです（設定手順は公開ドキュメントで案内しています）。
・文字起こし（字幕）がない動画には使えません。
・AI が作成するため、重要な事実は元の動画でご確認ください。
・作成した記事はブラウザを閉じると消えます。残したい場合は保存またはコピーしてください。

※ 本拡張機能は Google LLC または YouTube とは無関係の非公式ツールです。YouTube is a trademark of Google LLC.
```

## 4. 詳細説明文（英語版）

```text
When you don't have time to watch a long lecture, interview, or explainer video.
YomuDoga for YouTube™ is a browser extension that turns YouTube™ video transcripts into magazine- and newspaper-style Japanese column articles. By trimming tangents and repetition in spoken language and editing and proofreading the result, it aims to deliver clear, well-structured reading material whose key points come through even without watching the video.

■ Who it's for
- You don't have time to watch a 30–40 minute video, but still want to grasp its substance and arguments
- You want a natural Japanese article to read rather than a brief bullet-point summary
- You want to read foreign-language videos (such as English) as Japanese articles
- You want a tool with no account registration that prioritizes privacy and security

■ How it differs from typical summary tools
- Conclusions first, ordered by importance
  The opening lead states the topic and main conclusion, and the sections follow in order of importance. Like a newspaper article, you get the main picture even if you stop reading partway through.
- Format-aware editing and fact-checking
  It chooses an article format suited to the video (interview, dialogue, review, or explainer column), adjusts the length, and checks the draft against the original transcript for transcription errors or unsupported claims.

■ How to use (3 steps)
1. Open the transcript panel on a YouTube™ video page and select the lines you want to include.
2. Click the YomuDoga icon in the browser toolbar to start generation.
3. Read the finished article in a dedicated tab. You can also save it as a text file (.md) or copy it into your notes app.

■ Built for reliability and peace of mind
- No automated page manipulation
  Because it only reads the transcript text you select, it is resilient to YouTube™ UI changes and stays reliable over time.
- Designed for privacy and security
  It reads the active page only when you click the button and never accesses your regular browsing. There is no relay server—requests go directly from your browser to the AI service—and all source code and prompts are published as open source.

■ Before you start
- The extension itself is free, but generating articles requires your own Anthropic API key (pay-as-you-go, separate from a Claude.ai subscription). A typical article costs roughly $0.40 USD (setup instructions are provided in the public documentation).
- Videos without a transcript (captions) are not supported.
- Because articles are generated by AI, please verify important facts against the original video.
- Generated articles are cleared when you close the browser. Save or copy any article you want to keep.

Note: This extension is an unofficial tool and is not affiliated with Google LLC or YouTube. YouTube is a trademark of Google LLC.
```
