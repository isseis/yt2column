# 設計への申し送り：本番の構成と品質を共通の文書にまとめ、文書の主経路をブラウザ拡張に改める

要件定義（[01_requirements.md](01_requirements.md)）の段階で挙がった、設計で扱う事項を記す。`02_architecture.md` は、各項目について採った方法か、当てはまらない理由を記録する。

## H-01: 文書の値と構造を検査するテスト

`CLAUDE.md`「Testing Strategy」は、文書に書いた値や構造を変えたら、それに合うテストも要ると定めている（「A change to a documented value or structure still needs the matching test」）。本タスクで文書に書く値と構造のうち、コードや設定が定めるものを、どのテストで検査するかを決める。候補は次のとおりである。

-   共通の文書と README の推奨構成の小節に書く環境変数の名前と値。名前は `internal/config` が読む変数、`claude` は受理するプロバイダ名、`high` は受理する effort の値である。
-   モデル名 `claude-opus-5-5` が `internal/pricing` の単価の表にあること。
-   README の Quick start のリンクが、README の中の見出しに解決すること（AC-09）。

既存の文書のテスト（`cmd/yt2column/docs_test.go`・`cmd/yt2column-server/docs_test.go` など）に加えるか、新しいテストにするかもあわせて決める。同節は、文の意味を単語の照合で確かめるテストと、文書から写したリテラルを固定するテストを禁じている。そのため、AC-12（既存の見出しの文言と順序）は、見出しの一覧を写したテストではなく、PR のレビューで差分を読んで確かめる。リテラルを写さない構造の検査にできるなら、それでもよい。共通の文書の評価の数値（件数・金額）は、評価記録の値であってコードが定める値ではない。テストで評価記録の値と照合するかどうかを決める。照合しない場合は、PR のレビューで確かめる（AC-04・AC-05）。

## H-02: 共通の文書の節の構成と順序

AC-01〜AC-07・AC-25 の内容を、どの節にどの順で置くかを決める。製品の制約の字数の調整の失敗の項目は、主経路での見え方を CLI の見え方と並べて書く（AC-04）。0008 評価記録 §6.3 の 5 は「CLI は記事を出さない」と書いている。一方、主経路では、記事は投稿されず、失敗の通知（`internal/failure`）が Webhook に投稿される（`README.md:76-79`・`README.md:911-912`、`internal/server/job.go` の `classifyWrite`）。どちらを先に書くか、ほかの項目にも主経路と CLI で見え方の違うものがあるかを、設計で確かめる。値は 0008 評価記録 §6.3 から変えない。

## H-03: README の Quick start の手順の粒度と見出し

Quick start の見出しの文言と、手順ごとに既存の節の内容をどこまで繰り返すかを決める。既存の節（「Localhost server」「Installing the native messaging host」「Browser extension」「Configuration」）へのリンクを主にし、内容の重複を最小にするのが、DRY の方針（`CLAUDE.md`「Key Design Patterns」）に沿う。「Browser extension」は「Development」の下の `###` の節である（`README.md:818`）。利用者向けの手順が開発の節の下にあることを、Quick start でどう案内するかも決める。節は動かさない（01_requirements.md 2.3）。

## H-04: 3 つのパイプラインの図の形

README（`README.md:8-10`）・`project_overview.md`（`docs/dev/project_overview.md:20-22`）・`CLAUDE.md`（`CLAUDE.md:136-138`）の図を、主経路の入力（コピーした字幕）と補助の経路の入力（URL と yt-dlp）の両方を示す形にする（AC-08・AC-18・AC-24）。図の形（`TranscriptSource` の 2 つの実装 `CopiedSource`・`YtDlpSource` を並べるか、入力を 2 行で示すかなど）と、3 つの図をそろえるかを決める。図の下の `TranscriptSource` の説明（`CLAUDE.md:140-143`、`docs/dev/project_overview.md:26`）は yt-dlp から書き始めている。図に合わせて順序を直すかもあわせて決める。README の冒頭の箇条書き（`README.md:12-15`）の書き直しも、ここで扱う。

## H-05: README の見出しと表を読むテストへの影響

README を読むテストのパーサは、コードブロックを区別せずに `#` で始まる行を見出しとして数える。該当するのは、`cmd/yt2column/docs_test.go:314-358` の `headingLevel`・`sectionByHeading`、`cmd/yt2column-server/docs_test.go:311`、`cmd/yt2column-nmhost/main_test.go` の `TestReadmeHostExitCodes`、`extension/test/helpers/repoFile.ts:23` の `markdownSection` である。README 自身も、コードブロックの中に `#`・`##` の行を持つ（`README.md:215`・`:221`・`:250`・`:254`）。新しいコードブロックにシェルのコメント（`# ...`）を入れると、節の範囲が気づかれずに短くなりうる。また、`findTable(readme, "Flag")`・`"reason"`・`"Code"` は最初に合う表を取る。新しい表を前に置くと、検査の対象が入れ替わりうる。Quick start・Requirements の書き分け（AC-10）・推奨構成の小節（AC-13）で、コードブロックと表をどう書くかを、これらの制約に照らして決める。Requirements の下に `###` を加える場合は、`##` の節を切り出すテストへの影響も確かめる。

## H-06: `project_overview.md` の書き直しと、既存のテストの検査対象

AC-20 のとおり、`project_overview.md` を読む既存のテストが検査する値が、書き直しの後も残るようにする。該当するテストは `cmd/yt2column/docs_test.go:82`・`:528`、`cmd/yt2column-server/docs_test.go:109-113` である。「字幕取得」の項は、`CLAUDE.md`「Key Design Patterns」の「Delegate the fragile part」と同じ内容を持つ。`CLAUDE.md` 側のこの記述は F-005 の範囲外なので、書き直しの後に両者が矛盾しないことを確かめる。AC-17 の、決定が加わっても除かれてもいないことの確かめ方（変更の前後の決定の対照表を `02_architecture.md` に置くなど）もあわせて決める。

## H-07: 0008 評価記録への参照の置き場所

AC-22 で 0008 評価記録 §6 に加える参照を、§6 の冒頭に置くか、§6.3・§6.4 のそれぞれに置くかを決める。§6.5 の最後の項目（「共通の文書としてまとめる予定」）へのリンクの足し方もあわせて決める。

## H-08: 用語集

主経路・補助の経路・本番の構成・品質の目標・製品の制約を `docs/translation_glossary.md` に加えるかを決める。README と `CLAUDE.md` の英語の書き方（"primary path"・"production profile" など）をそろえるために役立つ。用語集の更新の手順は `mktrans.md` に従う。
