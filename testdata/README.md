# testdata

このディレクトリには、パーサのテスト用に保存した `yt-dlp` の実出力がある。**リポジトリに保存してよいのは、再配布が許諾されたライセンスの動画からの出力だけ**とする（通常の YouTube 標準ライセンスの動画の全文字起こしは保存しない）。

## 出典とライセンス

- 動画: 【日本語で☆話しまSHOW】ネパール人留学生による日本語スピーチコンテスト 2019 #6 (日本語／ENG)
- URL: https://www.youtube.com/watch?v=2tcCWM-sRBw
- チャンネル: 日本語で☆話しまSHOW
- ライセンス: Creative Commons Attribution license (reuse allowed)（CC BY。YouTube の動画ページの表示）
- 取得日: 2026-09-30
- 取得に使ったコマンド: `yt-dlp --ignore-config --no-plugin-dirs --skip-download --write-auto-subs --sub-langs ja --sub-format json3 --write-info-json -P <dir> -o "%(id)s" -- <URL>`
- yt-dlp: 2026.08.19

CC BY の条件に従い、上記の出典（動画 URL・チャンネル名・ライセンス）を明記する。ファイルは無改変で保存している。

- `2tcCWM-sRBw.ja.json3`: 上記動画の自動字幕（json3）。
- `2tcCWM-sRBw.info.json`: 同上の info.json。
