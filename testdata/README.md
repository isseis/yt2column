# testdata

このディレクトリには、パーサのテスト用に保存した `yt-dlp` の実出力、YouTube の動画ページの文字起こしパネルの HTML の抜粋、DeepSeek の API の実応答（後述）がある。`yt-dlp` の出力と HTML の抜粋については、**リポジトリに保存してよいのは、再配布が許諾されたライセンスの動画からの出力だけ**とする（通常の YouTube 標準ライセンスの動画の全文字起こしは保存しない）。

## 出典とライセンス

- 動画: 【日本語で☆話しまSHOW】ネパール人留学生による日本語スピーチコンテスト 2019 #6 (日本語／ENG)
- URL: https://www.youtube.com/watch?v=2tcCWM-sRBw
- チャンネル: 日本語で☆話しまSHOW
- ライセンス: Creative Commons Attribution 3.0 Unported（CC BY 3.0）、https://creativecommons.org/licenses/by/3.0/ 。YouTube の動画ページの表示は "Creative Commons Attribution license (reuse allowed)" であり、YouTube のこの指定は CC BY 3.0 を指す
- 取得日: 2026-09-30
- 取得に使ったコマンド: `yt-dlp --ignore-config --no-plugin-dirs --skip-download --write-auto-subs --sub-langs ja --sub-format json3 --write-info-json -P <dir> -o "%(id)s" -- <URL>`
- yt-dlp: 2026.08.19

CC BY 3.0 の条件に従い、上記の出典（動画 URL・チャンネル名・ライセンスとその URI）を明記する。

`2tcCWM-sRBw.ja.json3` は無改変で保存している。`2tcCWM-sRBw.info.json` は、取得者の IP アドレスまたはエッジサーバーのホスト名を含む JSON 文字列（`formats` のストリーム URL・マニフェスト URL・フラグメント URL と、一部の字幕 URL の計 24 個）の値だけを `https://redacted.invalid/` に置き換えており、それ以外は無改変である。パーサが消費するフィールド（`id`・`title`・`channel`・`description`）は変更していない。

`yt-dlp` は取得者の IP アドレスを署名付き URL に埋め込むため、再取得した info.json をコミットする前にも同じ置き換えを行い、IP アドレス（URL エンコードされた形も含む）が残っていないことを確認する。

- `2tcCWM-sRBw.ja.json3`: 上記動画の自動字幕（json3）。
- `2tcCWM-sRBw.info.json`: 同上の info.json。

## 文字起こしパネルの HTML の抜粋

ブラウザ拡張から文字起こしを取得する方法（[issue #43](https://github.com/isseis/yt2column/issues/43)）を検討するために保存した、上記と同じ動画のページの HTML の抜粋である。出典とライセンスは前節と同じ。

- 取得日: 2026-10-07
- 取得方法: ブラウザで動画ページを開き、DevTools の Elements からページ全体の HTML を保存した。YouTube の表示言語は日本語で、ログインしていない状態で取得した（ページの設定値は `"LOGGED_IN": false` で、ヘッダーには「ログイン」ボタンがある）。
- 抜粋の範囲: `ytd-engagement-panel-section-list-renderer[target-id="PAmodern_transcript_view"]` の要素 1 つ（開始タグから対応する終了タグまで）を、保存したページから切り出した。末尾に改行を 1 つ加えた以外は無改変である。DevTools でその要素を選び「Copy outerHTML」を実行した結果と同じものである。

ページ全体の HTML は**コミットしない**。YouTube 自身のマークアップ・スクリプト・CSS と、他チャンネルの動画のタイトルやサムネイル（おすすめ欄）を含むためである。ログインした状態で保存した場合は、さらにアカウントの情報（アバター、登録チャンネルの一覧、視聴履歴に基づくおすすめなど）も含む。抜粋には文字起こし本文、パネルの UI のラベル（「文字起こし」「閉じる」など）、アイコン 1 つの SVG パスだけが含まれ、URL・画像・アカウントの情報を含まないことを確認した。

ページ全体の HTML は、どの worktree からも参照できるように、リポジトリの外の `${YT2COLUMN_SNAPSHOT_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/yt2column/snapshots}` に `<動画 ID>.<連番>.html` という名前で置く。環境変数は任意で、設定しなければ既定のパスを使う。テストはこのディレクトリを参照しない。抜粋を作り直すときは、ここに置いたファイルから上記の要素を切り出すか、DevTools で直接その要素の outerHTML をコピーする。

- `2tcCWM-sRBw.transcript_panel.html`: 「文字起こしを表示」をクリックした直後のパネル（`visibility="ENGAGEMENT_PANEL_VISIBILITY_EXPANDED"`）。`transcript-segment-view-model` が 153 個あり、最後のタイムスタンプは 24:13（動画の長さは 24:22）。
- `2tcCWM-sRBw.transcript_panel_hidden.html`: ページの読み込みが終わった直後の、まだ開いていないパネル（`visibility="ENGAGEMENT_PANEL_VISIBILITY_HIDDEN"`）。中身は読み込み中の表示（`yt-content-loading-renderer`）だけである。

パネルの文字起こしは `2tcCWM-sRBw.ja.json3` とは別のデータで、区切りが粗く、句読点や「…」を含む。

## テスト用の合成サンプル（実出力ではない）

次の 4 ファイルは、実 `yt-dlp` の出力ではなく、パーサの拒否経路をテストするために手で作った合成サンプルである。実際の動画からの出力ではないため、上記の出典・ライセンス（CC BY）と IP 置換の条件は適用されない。

- `invalid_utf8.json3`: json3 の `utf8` に不正な UTF-8 バイト列（`0xFF`）を含む。
- `unpaired_surrogate.json3`: json3 の `utf8` に、対になっていないサロゲートのエスケープ `\ud800` を含む。
- `invalid_utf8.info.json`: info.json の `title` に不正な UTF-8 バイト列（`0xFF`）を含む。
- `unpaired_surrogate.info.json`: info.json の `title` に、対になっていないサロゲートのエスケープ `\ud800` を含む。

`invalid_utf8` の 2 ファイルは不正な UTF-8 バイト列を含み、`unpaired_surrogate` の 2 ファイルは ASCII のまま `\ud800` のエスケープを含む。テストは、パースの前にサンプル自身がこの性質を持つことを検証する（別の理由で不正な JSON と判定されていないことを確かめる）。

## DeepSeek の API の実応答

次の 2 ファイルは、DeepSeek アダプタ（`internal/llm/deepseek`）の応答の検証のテスト用に保存した、DeepSeek の Chat Completions API の実応答の本文である。取得の経緯と観測結果は [docs/tasks/0003_deepseek_llm_client/02_architecture.md](../docs/tasks/0003_deepseek_llm_client/02_architecture.md) §1.4 に記録している。

- 取得日: 2026-10-02
- 送信先: `POST https://api.deepseek.com/chat/completions`（HTTP/2）
- モデル名: `deepseek-flash`。thinking モードは API の既定（有効）のまま
- プロンプト: system `You are a concise assistant.`、user `Explain in two sentences why the sky is blue.`（テストのために用意した固定の文で、字幕や個人情報を含まない）

生成テキストは固定のプロンプトに対するモデルの出力であり、第三者の著作物を含まない。応答本文は受け取ったバイト列のまま無改変で保存している。応答本文に API キーは含まれず、保存前に API キーとその末尾 4 文字が含まれないことを確認した。

- `deepseek_chat_completion_stop.json`: `max_tokens` を送らないリクエストへの応答。`finish_reason` は `stop`。
- `deepseek_chat_completion_length.json`: `max_tokens` を 16 としたリクエストへの応答。`finish_reason` は `length`、`content` は空文字列。
