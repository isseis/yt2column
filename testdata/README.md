# testdata

このディレクトリには、パーサのテスト用に保存した `yt-dlp` の実出力がある。**リポジトリに保存してよいのは、再配布が許諾されたライセンスの動画からの出力だけ**とする（通常の YouTube 標準ライセンスの動画の全文字起こしは保存しない）。

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
