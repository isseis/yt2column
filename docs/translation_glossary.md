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
