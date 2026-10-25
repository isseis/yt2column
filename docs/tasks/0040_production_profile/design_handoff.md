# 設計への申し送り：本番の構成と品質を共通の文書にまとめ、文書の主経路をブラウザ拡張に改める

要件定義（[01_requirements.md](01_requirements.md)）の段階で挙がった、設計で扱う事項を記す。`02_architecture.md` は、各項目について採った方法か、当てはまらない理由を記録する。

## H-01: 文書の値を検査するテスト

`CLAUDE.md`「Testing Strategy」は、文書に書いた値や構造を変えたら、それに合うテストも要ると定めている（「A change to a documented value or structure still needs the matching test」）。本タスクで文書に書く値のうち、コードや設定が定めるものをどのテストで検査するかを決める。候補は次のとおりである。

-   共通の文書と README の推奨構成の小節に書く環境変数の名前と値。名前は `internal/config` が読む変数、`claude` は受理するプロバイダ名、`high` は受理する effort の値である。
-   モデル名 `claude-opus-5-5` が `internal/pricing` の単価の表にあること。
-   README の Quick start のリンクが README の中の見出しに解決すること（AC-09）と、既存の見出しの文言と順序が保たれること（AC-12）。

既存の文書のテスト（`cmd/yt2column/docs_test.go`・`cmd/yt2column-server/docs_test.go` など）に加えるか、新しいテストにするかもあわせて決める。文の意味を単語の照合で確かめるテストは書かない（同節）。共通の文書の評価の数値（件数・金額）は評価記録の値であり、コードが定める値ではない。テストで評価記録の値と照合するかどうかを決める。照合しない場合は PR のレビューで確かめる（AC-04・AC-05）。

## H-02: 共通の文書の節の構成と順序

AC-01〜AC-07 の内容を、どの節にどの順で置くかを決める。製品の制約の各項目について、主経路の利用者が何を見るかを書き分けるかも決める。0008 評価記録 §6.3 の 5（記事が出ない）は「CLI は記事を出さない」と書いている。一方、主経路（localhost サーバ）では、失敗の通知（`internal/failure`）が投稿される。主経路を主とする文書では、主経路での見え方を先に書くのが自然である。ただし、値は 0008 評価記録 §6.3 から変えない。

## H-03: README の Quick start の手順の粒度と見出し

Quick start の見出しの文言と、手順ごとにどこまで README の既存の節の内容を繰り返すかを決める。既存の節（「Localhost server」「Installing the native messaging host」「Browser extension」「Configuration」）へのリンクが主で、内容の重複は最小にするのが DRY の方針（`CLAUDE.md`「Key Design Patterns」）に沿う。「Browser extension」は「Development」の下の `###` の節（`README.md:818`）である。利用者向けの手順が開発の節の下にあることを Quick start でどう案内するかも決める（節は動かさない。01_requirements.md 2.3）。

## H-04: README の冒頭の箇条書きとパイプラインの図

README の冒頭のパイプラインの図（`README.md:9-11`、`URL → TranscriptSource → ...`）と箇条書き（`README.md:13-16`）を、主経路に合わせてどう書き直すかを決める。`CLAUDE.md` の「Architecture Overview」の図（`URL → TranscriptSource → ...`）と `project_overview.md` の図（`docs/dev/project_overview.md:21`）も同じ形である。3 つの図をそろえるかどうかもあわせて決める。`CLAUDE.md` の図を変えるかは F-005 の範囲外（冒頭の一文だけ）なので、変えない場合は理由を記録する。

## H-05: README の Requirements の書き分け

AC-10 の、拡張の経路に必要なもの（Go、Node.js、ブラウザ、API キー、`SLACK_WEBHOOK_URL`）と CLI だけに必要なもの（`yt-dlp`）の書き分けの形（2 つの小見出し、表、箇条書きの注記）を決める。`### ` の見出しを Requirements の下に加えると、AC-12 の「既存の見出しの順序」は保たれるが、README の見出しを読むテストが節の範囲を `##` の次の見出しまでとして切り出している場合に影響しうる。既存のテストが節をどう切り出しているかを確かめる。

## H-06: `project_overview.md` の書き直しと、既存のテストの検査対象

AC-20 のとおり、`project_overview.md` を読む既存のテスト（`cmd/yt2column/docs_test.go:82`・`:528`、`cmd/yt2column-server/docs_test.go:109-113`）が検査する値が、書き直しの後も残るようにする。「字幕取得」の項は `CLAUDE.md`「Key Design Patterns」の「Delegate the fragile part」と同じ内容を持つ。`CLAUDE.md` 側の記述を書き直すかは F-005 の範囲外である。書き直さない場合は、両者が矛盾しないことを確かめる。

## H-07: 0008 評価記録への参照の置き場所

AC-22 の、0008 評価記録 §6 に加える参照を、§6 の冒頭に置くか、§6.3・§6.4 のそれぞれに置くかを決める。§6.5 の最後の項目（「共通の文書としてまとめる予定」）へのリンクの足し方もあわせて決める。
