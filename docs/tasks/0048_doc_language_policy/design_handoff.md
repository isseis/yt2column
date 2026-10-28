# 設計への申し送り：文書の言語の方針と README の二言語化

要件定義（[01_requirements.md](01_requirements.md)）の作成中に挙がった設計事項を記す。`02_architecture.md` は、各項目にどう対応したか、または対応しない理由を記録する。

## H-01: 日本語の見出しのアンカー

-   **背景:** `TestReadmeLinks` と `TestQuickStartLinks` は、README の見出しから GitHub のアンカーを作る補助関数（`anchorFromHeading`、`cmd/yt2column/docs_test.go:70-83`）で、リンク先の見出しを確かめる（`TestReadmeLinks`、同 `:1119`）。補助関数の表は、2026-10-26 に GitHub の表示と照らし合わせたもので、ASCII の見出しが中心である。
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
    -   字下げしたフェンスや、閉じていないフェンスの扱い。既存の `sectionOutsideBlocks`（`cmd/yt2column/docs_test.go:116`）や `sectionByHeading`（同 `:565`）などの補助関数を使えるか。
    -   AC-29・AC-30 は、`make test` に含まれ、README だけを変えた PR でも CI で走ることを求める。`make test` は Go のテストを走らせる（`Makefile`）ので、`doc-guard` の CI ジョブが走らせるパッケージ（`go test -tags test ./cmd/yt2column ...`）に置くことが候補になる。
    -   表の数や行の数も比べるかどうか。issue は見出しの構造とコードブロックの数だけを求めているので、加える場合は要件の変更になる。

## H-05: 過去のタスク文書のリンクの一括置換

-   **背景:** `docs/dev/developer_guide/` を含む行は、リポジトリ全体で 341 行ある。過去のタスク文書では 92 ファイルに現れる（本書の作成時、`git grep -c`）。過去のタスク文書からの相対リンクは `../../dev/developer_guide/...` の形である。英語版と日本語版の手引きを両方挙げている行もある。
-   **注意点:**
    -   英語版の手引きと日本語版の手引きの両方を挙げていた行は、置き換えると同じパスが 2 度並ぶ。AC-13・AC-14 により、そのままにする。
    -   置き換えがパスだけであることを、`_checks/` のスクリプトでマージベースとの差分から確かめる方法（AC-13・AC-14）。
    -   見出しへのアンカー付きのリンク（`developer_guide/...md#...`）は、本書の作成時にはなかった。

## H-06: 英語版の手引きを削除する前の確かめ

-   **背景:** `mergepr_guide.md` は 238 行、`mergepr_guide.ja.md` は 237 行である。`fixprguard_guide.md` と `fixprguard_guide.ja.md` は、どちらも 462 行である（本書の作成時）。最後の変更のコミットは、それぞれの組で同じである。
-   **注意点:** 行数の差が内容の違いによるものかを確かめ、英語版にだけある記述があれば、日本語版に加えてから削除する（AC-11）。確かめた結果を実装計画に記録する。

## H-07: `/fixpr` の同期と `fixprguard` の範囲

-   **背景:** `/fixpr` は、レビューのスレッドへの対応で変えたファイルが範囲に入るかを `fixprguard` に判定させる（`docs/dev/developer_guide/fixprguard_guide.md` §6）。`README.md` のスレッドへの対応で `README.en.md` も変えると、`README.en.md` はスレッドの `path` とは別のファイルである。
-   **本書の作成時に確かめた事実:** `README.en.md` は `README.md` と同じディレクトリにあり、`*.md` の許可リスト（`internal/fixprguard/scope.go:76`）に当たる。そのため、`README.md` のスレッドに対して規則 2 で範囲に入ると見込まれる。
-   **注意点:** この見込みを設計で確かめる。入らない場合、AC-37 の手順を `/fixpr` でどう実行するか（別のコミットにするか、`/runpr` に任せるか）を決める。

## H-08: CLAUDE.md と `_context.md` を読むガードテスト

-   **背景:** CLAUDE.md を読むガードテストがある。`make` の `ext-` ターゲットの一覧、`## Quick Links` の見出し（`cmd/yt2column/docs_test.go:1087`）、`/fixpr` と `/runpr` のコマンド文書などである。
-   **注意点:** CLAUDE.md の "Documents"・"Language"・Quick Links・Development Notes を改める PR で、これらのガードテストが通るように、改める範囲を確かめる。

## H-09: CLI のヘルプとサーバの診断の参照先を確かめるテスト

-   **背景:** CLI のヘルプは、`cmd/yt2column/run.go:225`・`:259` で README を指している。ヘルプの出力を確かめるテストがあるかもしれない。
-   **本書の作成時に確かめた事実:** CLI のヘルプの "see README" を読むテストはない（`cmd/**/*_test.go` に一致がない）。
-   **注意点:** ヘルプとサーバの診断が示すパス（AC-35a・AC-36）が存在することを確かめるテストを加えるかどうかを決める。

## H-10: `/mktrans` の差分翻訳の前提

-   **背景:** `/mktrans` の差分翻訳は、出力のファイルを最後に変えたコミットを同期点にする（`.claude/commands/mktrans.md:73-79`）。
-   **注意点:** `README.en.md` を `README.md` と同じコミットで変えると、同期点は `README.md` を変えたコミットと同じになる。これで差分が正しく求まるかを確かめる。また、`README.en.md` を文言の修正だけで変えた場合に、`README.md` の未反映の変更が差分から落ちないかを確かめる（AC-38）。

## H-11: README を読むガードテストの一覧と振り分け

-   **背景:** 本書の作成時に README を読んでいるガードテストは次のとおりである（AC-31・AC-32）。
    -   `cmd/yt2column/docs_test.go`: `TestREADMEDocumentsFlags`、`TestSlackDocsContract`、`TestClaudeDocsContract`、`TestDeepSeekDocsContract`、`TestLLMDefaultsDocsContract`、`TestProductionProfileDocsContract`、`TestMigrationDocsContract`、`TestQuickStartLinks`、`TestProductionProfileLinks`、`TestReadmeLinks`、`TestTokenUsageDocsContract`、`TestCostDocsContract`
    -   `cmd/yt2column/output_layout_docs_test.go`: `TestOutputLayoutDocsContract`
    -   `cmd/yt2column/extension_docs_test.go`: `TestExtensionDocumentation`
    -   `cmd/yt2column-server/docs_test.go`: `TestServerDocsContract`
    -   `cmd/yt2column-nmhost/main_test.go`: `TestReadmeHostExitCodes`
    -   `extension/test/serverReasons.test.ts`、`extension/test/metadataDocs.test.ts`
-   **注意点:**
    -   各テストが読む値を、README の両版・`cli.md`・`development.md` のどこで検査するかを決める。1 つのテストが、両方に分かれた値を読むもの（例：`TestSlackDocsContract` は README に残る `16,383` と、`development.md` に移る統合テストの変数を読む。`TestCostDocsContract` は README に残る脚注と、`cli.md` に移る `cost:` の行を読む）がある。
    -   `TestLLMDefaultsDocsContract` は Configuration の表の「When unset」の列を読む。README の `SLACK_WEBHOOK_URL` の行の欄の内容（AC-19）と、0044 が加える変更に合わせる。
    -   AC-32 の前後の表の形を決める。
    -   一覧は 0044 PR-7・0046 PR-4 のマージで変わりうる。設計の時点で作り直す。

## H-12: 文の単位の振り分けの一覧

-   **背景:** F-003 の境界の規則は、文の単位で当てはめる。AC-19〜AC-21 の表は節の単位であり、README に残す節の中の CLI だけに関わる文（例：Estimated cost の再開した実行の料金、Troubleshooting の「A CLI run exits with `2`」、Configuration の CLI がクライアントを作る時点）や、curl に関わる文（Troubleshooting の curl のメッセージ）を個別には挙げていない。
-   **注意点:** 0044 PR-7・0046 PR-4 のマージ後の README について、文の単位の振り分けの一覧を作り、AC-22 を確かめる手段にする。README の行番号は両タスクで変わるので、一覧はその時点の README に対して作る。

## H-13: 英語から日本語への翻訳の手段と順序

-   **背景:** 改訂後の `/mktrans` は `README.md` から `README.en.md` への翻訳だけを扱う（AC-38・AC-39）。一方、本タスクには英語から日本語への翻訳がある（`README.md` の日本語化、`pricing_update.md`・`claude_output_limit.md` の翻訳（AC-10）、`cli.md`・`development.md` の内容）。
-   **注意点:** これらの翻訳を、改訂前の `/mktrans`、または手作業のどちらで行うかと、`/mktrans` の改訂との順序を、実装計画で決める。改訂前の `/mktrans` は `README.md` から `README.ja.md` を出力する（`.claude/commands/mktrans.md:27-35`）ので、その後のファイル名の扱いも決める。
