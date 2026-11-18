# 設計書：/runpr の NEXT 行の例を chief の読み取りで確かめる

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 設計の全体像

本タスクは、`internal/chief` パッケージにテストを 1 つ足す。テストは `.claude/commands/runpr.md` の step 4 と step 5 から NEXT 行の例を取り出し、chief の 2 つの読み取りの経路に通す（[01_requirements.md](01_requirements.md) F-001）。`CLAUDE.md` の文書を読むガードの一覧にも 1 項目を足す（AC-06）。chief の製品コードと `runpr.md` は変えない（F-002）。

**設計原則:**

-   **定義元のコードで読む。** 例は、chief が実際に使う読み取りの関数に渡す。正規表現をテストに写すと、chief の文法が変わったときに、写しのほうが古いまま通ってしまうためである。
-   **例は文書から読む。** テストは例の文字列を持たない（要件書 4.5 節）。

**設計の際に決めたこと（2026-11-19）:**

-   なし。人に聞くべき開いた判断はなかった。テストを置くパッケージと、通す読み取りの経路は、次の理由でコードから決まる。読み取りの関数は `internal/chief` の非公開の関数で、同じパッケージのテストからしか呼べない。経路は、chief が NEXT 行を読む 2 つ（要件書 1 章の背景）をそのまま使う。

本書の行番号は、コミット `790652e3`（`origin/main`）で確かめた。

概念モデルとシステム構成の Mermaid の図は省く。足すのはテスト 1 つで、コンポーネントの関係は変わらない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「May be omitted」）。

## 2. システム構成

N/A。コンポーネントの構成とデータの流れを変えない。

## 3. コンポーネント設計

### 3.1. 変更するファイル

| ファイル | 変更 | 責務 |
|---|---|---|
| `internal/chief/runpr_next_doc_test.go`（新規） | 追加 | `runpr.md` の NEXT 行の例を取り出し、chief の読み取りに通すテスト（AC-01〜AC-05） |
| `CLAUDE.md` | 1 項目の追記 | 「Development Notes」の文書を読むガードの一覧（`CLAUDE.md:753` の `internal/chief` の読み取りを挙げる箇所）に、`runpr.md` の NEXT 行の例を足す（AC-06） |

ファイル名は実装で変えてよい。ただし `internal/chief` の中の `_test.go` で、名前を `screen` で始めない。`screen` で始まるファイルは、画面の読み取りとして CLAUDE.md「Declare, don't infer」の例外の対象になるためである。このテストは画面を読まず、文書を読む。

### 3.2. 例の取り出し

-   `runpr.md` は、リポジトリの中の相対パスで読む（`internal/chief/task_test.go:25-32` の `repoContext` が `_context.md` を読むのと同じ形）。
-   節は見出しで探す。step 4 は `## 4. Confirm, record, and merge`（`runpr.md:777`）、step 5 は `## 5. Switch branch, then show NEXT`（`runpr.md:1338`）である。節は、見出しから同じレベルの次の見出しまでとする。見出しで節を探す関数は、`internal/doctestutil` にあれば使う（`SectionByHeading`・`SectionOutsideBlocks`、`internal/doctestutil/markdown.go:44`・`:90`）。見出しが見つからなければ、テストは失敗する（AC-03）。
-   例は、要件書 F-001 の冒頭の定義どおりに取り出す。インラインのコードスパンの中身と、フェンスで囲んだコードブロックの各行のうち、前後の空白を除いて `NEXT: ` に続けて空白でない文字を持つものである。コードブロックの行は、`runpr.md:1046` のように、行の後ろに注記（`(always; the forms below)`）が付いている。注記は、chief がそのような行を画面で読むときと同じく、取り除かずにそのまま渡す。
-   2026-11-19 の `runpr.md` では、step 4 から 6 つ（コードブロックの 1 行と、`runpr.md:1090-1098` のコードスパン 5 つ）、step 5 から 2 つ（`runpr.md:1401`・`1403`）の例が取れる。`runpr.md:1399` の値のない `NEXT: ` は定義から外れる。この数はテストに書かない。数の下限は、節ごとに 1 つ以上とする（AC-03）。

### 3.3. 読み取りの経路

例のそれぞれを、1 行のテキストとして次の 2 つに渡す。

| 経路 | 関数 | chief が使う場所 |
|---|---|---|
| 画面 | `readRunprNext`（`internal/chief/screen_report.go:72`） | OpenCode の pane の最後の報告（`readRunprReport`、`screen_report.go:233`）と、画面全体の NEXT 行（`readScreenNext`、`autosend.go:517`） |
| transcript | `readReportNext`（`internal/chief/screen_transcript_report.go:303`） | Claude Code の pane の最後の報告（`autosend.go:506`・`:557`） |

結果の `Next.Kind` が `NextNone` か `NextCommand` であれば、読めたとする。`NextUnmatched` か `NextAbsent` であれば失敗とし、節と例のテキストと経路を示す（AC-04）。

コマンドとタスク番号の中身（`/mkarch` か、`0089` か）までは比べない。比べる正解を作るには、例を読み直すか、テストに写すしかないためである。前者は同じ読み取りを 2 度行うだけになり、後者は要件書 4.5 節に反する。

## 4. エラーハンドリング設計

N/A。製品コードのエラーの扱いを変えない。テストの失敗の形は 3.2 節と 3.3 節にある。

## 5. セキュリティ考慮事項

N/A。`docs/dev/security.md` の条件のどれにも当たらない（[01_requirements.md](01_requirements.md) 4.2 節）。

## 6. 処理フローの詳細

N/A。chief の処理の流れは変わらない。

## 7. テスト戦略

-   新しいテスト（3.1 節）が AC-01〜AC-04 を確かめる。テストが理由どおりに失敗できることは、実装の際に、例を読めない形に変えたコピーの文書か、節を消したコピーの文書で確かめる（`CLAUDE.md`「Testing Strategy」）。そのため、文書を読む部分と、例を確かめる部分を分け、後者にテキストを渡せるようにする。
-   AC-05 は、テストが `internal/chief` パッケージにあり、ビルドタグ `test` で走ることで満たす。`doc-guard` ジョブのコマンド（`CLAUDE.md:756`）は `./internal/chief` を含む。
-   AC-06 は PR のレビューで読む（`manual`）。AC-07 は記録した diff を読む（`diff`）。

## 8. 実装の優先順位

1 フェーズ、1 本の PR で、テストと `CLAUDE.md` の 1 項目を足す。

## 9. 将来の拡張性

-   chief が NEXT 行を読む経路を足したら、3.3 節の表とテストにも足す。
-   `runpr.md` の step の番号か見出しが変わったら、テストの見出しも変える。見出しが見つからないとテストが失敗するので、変え忘れは残らない。
