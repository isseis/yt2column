# 設計への申し送り：文書の言語の方針と README の二言語化

要件定義（[01_requirements.md](01_requirements.md)）の作成中に挙がった設計事項を記す。`02_architecture.md` は、各項目にどう対応したか、または対応しない理由を記録する。

## H-01: 日本語の見出しのアンカー

-   **背景:** `TestReadmeLinks` と `TestQuickStartLinks` は、README の見出しから GitHub のアンカーを作る補助関数で、リンク先の見出しを確かめる（`cmd/yt2column/docs_test.go:1116-1165`）。補助関数の表は、2026-10-26 に GitHub の表示と照らし合わせたもので、ASCII の見出しが中心である。
-   **注意点:** 日本語の見出しでは、GitHub は日本語の文字をアンカーに残し、全角の記号や空白を独自の規則で扱う。日本語の見出しのアンカーを GitHub の表示と照らし合わせ、補助関数がそれと同じアンカーを作ることを確かめる（AC-25）。`cli.md`・`development.md` のリンクも同じ補助関数で確かめるかどうかを決める。

## H-02: ガードテストのリテラルを言語ごとに持つ方法

-   **背景:** README を読むガードの多くは、英語の見出し（`"## Estimated cost"`、`"### Installing the native messaging host"` など）や表の列名（`"Variable"`・`"When unset"`）で節を探す（`cmd/yt2column/docs_test.go:41-42`・`:310`、`cmd/yt2column-nmhost/main_test.go:447`、`extension/test/metadataDocs.test.ts:25`）。
-   **注意点:**
    -   日本語版と英語版の見出しと列名の組を、どこに 1 度だけ定義するかを決める（Go と TypeScript の両方で使う）。
    -   失敗のメッセージで版がわかるようにする（AC-31）。
    -   `README.md` を日本語にしてから英語のリテラルを `README.en.md` に向ける順序と、途中の PR で README が英語のままの状態（issue の PR の構成の案の 3）でガードが何を読むかを、実装計画で決める。
    -   Go のテストに日本語のリテラルを置くことは、AC-05 の例外の範囲に入れる。

## H-03: 拒否の理由の表が 2 つの文書に分かれること

-   **背景:** `extension/test/serverReasons.test.ts` は、README の拒否の理由の表から「拡張で固定のメッセージを表示する」行の `reason` とステータスコードの組を読む（`extension/test/serverReasons.test.ts:8-50`）。`cmd/yt2column-server/docs_test.go:86-107` は、同じ表の `reason` とステータスコードを `internal/server` の定義と照らし合わせる。本書の決定で、ステータスコードを含む完全な表は `development.md` に移り、README には `reason` と拡張での表示の対応だけが残る（AC-19・AC-21）。
-   **注意点:**
    -   どちらのテストが、どの文書のどの表を読むかを決める。README の表（`reason` と表示）と `development.md` の完全な表が食い違わないことを、どう確かめるかを決める。
    -   `development.md` を拡張のテストが読むなら、`extension/scripts/has-extension-changes.sh` の判定にそのファイルを加える（AC-33）。

## H-04: 構造の一致のテストの定義

-   **背景:** AC-29 は、見出しのレベルの並びと数、コードブロックの数の一致を求める。README には、コードブロックの中に `#` で始まる行がある（`README.md:292`・`:298`・`:327`・`:331`）。Mermaid のブロックもある（`README.md:12-38`）。
-   **注意点:**
    -   見出しは、コードブロックの外の ATX 形式の見出しだけを数えるか。Setext 形式の見出しを許すか、拒否するか。
    -   字下げしたフェンスや、閉じていないフェンスの扱い。既存の `sectionOutsideBlocks` などの補助関数（`cmd/yt2column/docs_test.go:560-620`）を使えるか。
    -   テストを Go と TypeScript のどちらに置くか。`doc-guard` の CI ジョブ（`go test -tags test ./cmd/yt2column ...`）で走るように置けば、README だけの変更でも走る（AC-30）。
    -   表の数や行の数も比べるかどうか。issue は見出しの構造とコードブロックの数だけを求めているので、加える場合は要件の変更になる。

## H-05: 過去のタスク文書のリンクの一括置換

-   **背景:** `docs/dev/developer_guide/` を参照する行は 341 箇所あり、そのうち過去のタスク文書は 92 ファイルにある（本書の作成時）。過去のタスク文書からの相対リンクは `../../dev/developer_guide/...` の形である。英語版と日本語版の手引きを両方挙げている行もある。
-   **注意点:**
    -   英語版（`mergepr_guide.md`・`fixprguard_guide.md`）と日本語版（`.ja.md`）の両方を挙げていた行は、置き換えると同じパスが 2 度並ぶ。そのままにするか、1 つにまとめるか。まとめると、パス以外の記述が変わる（AC-14）。
    -   置き換えがパスだけであることを、`_checks/` のスクリプトでマージベースとの差分から確かめる方法（AC-13・AC-14）。
    -   見出しへのアンカー付きのリンク（`developer_guide/...md#...`）は、本書の作成時にはなかった。

## H-06: 英語版の手引きを削除する前の確かめ

-   **背景:** `mergepr_guide.md` は 238 行、`mergepr_guide.ja.md` は 237 行である。`fixprguard_guide.md` と `fixprguard_guide.ja.md` は、どちらも 462 行である（本書の作成時）。最後の変更のコミットは、それぞれの組で同じである。
-   **注意点:** 行数の差が内容の違いによるものかを確かめ、英語版にだけある記述があれば、日本語版に加えてから削除する（AC-11）。確かめた結果を実装計画に記録する。

## H-07: `/fixpr` の同期と `fixprguard` の範囲

-   **背景:** `/fixpr` は、レビューのスレッドへの対応で変えたファイルが範囲に入るかを `fixprguard` に判定させる（`docs/dev/developer_guide/fixprguard_guide.md` §6）。`README.md` のスレッドへの対応で `README.en.md` も変えると、`README.en.md` はスレッドの `path` とは別のファイルである。
-   **注意点:** `README.en.md` が、規則 2・3 で範囲に入るかを確かめる（`*.md` は許可リストにある）。入らない場合、AC-37 の手順を `/fixpr` でどう実行するか（別のコミットにするか、`/runpr` に任せるか）を決める。

## H-08: CLAUDE.md と `_context.md` を読むガード

-   **背景:** CLAUDE.md を読むガードがある。`make` の `ext-` ターゲットの一覧、`## Quick Links` の見出し（`cmd/yt2column/docs_test.go:1087`）、`/fixpr` と `/runpr` のコマンド文書などである。
-   **注意点:** CLAUDE.md の "Documents"・"Language"・Quick Links・Development Notes を改める PR で、これらのガードが通るように、改める範囲を確かめる。

## H-09: CLI のヘルプの文言を確かめるテスト

-   **背景:** CLI のヘルプは、`cmd/yt2column/run.go:225`・`:259` で README を指している。ヘルプの出力を確かめるテストがあるかもしれない。
-   **注意点:** 参照先を `docs/dev/guides/cli.md` に変えたとき（AC-36）、ヘルプの出力を確かめるテストを合わせて変える。ヘルプが示すパスが存在することを確かめるテストを加えるかどうかを決める。

## H-10: `/mktrans` の差分翻訳の前提

-   **背景:** `/mktrans` の差分翻訳は、出力のファイルを最後に変えたコミットを同期点にする（`.claude/commands/mktrans.md:73-79`）。
-   **注意点:** `README.en.md` を `README.md` と同じコミットで変えると、同期点は `README.md` を変えたコミットと同じになる。これで差分が正しく求まるかを確かめる。また、`README.en.md` を文言の修正だけで変えた場合に、`README.md` の未反映の変更が差分から落ちないかを確かめる（AC-38）。
