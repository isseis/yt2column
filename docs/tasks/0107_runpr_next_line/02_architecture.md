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

本タスクは、`internal/chief` パッケージにテストを 1 つ足す。テストは `.claude/commands/runpr.md` の step 4 と step 5 から NEXT 行の例を取り出し、chief の 2 つの読み取りの経路（3.3 節）に通す（[01_requirements.md](01_requirements.md) F-001）。例を取り出すために、`cmd/yt2column` のテストにある既存の関数を `internal/doctestutil` に移して使う。`CLAUDE.md` の文書を読むガードの一覧にも 1 項目を足す（AC-06）。chief の製品コードと `runpr.md` は変えない（F-002）。

**設計原則:**

-   **定義元のコードで読む。** 例は、chief が実際に使う読み取りの関数に渡す。例を取り出す形も、chief が NEXT を名指す行と見なす `nextNaming`（3.2 節）をそのまま使う。正規表現をテストに写すと、chief の文法が変わったときに、写しのほうが古いまま通ってしまうためである。
-   **例は文書から読む。** テストは例の文字列を持たない（要件書 4.5 節）。

**設計の際に決めたこと（2026-11-19）:**

-   なし。人に聞くべき未決の判断はなかった。テストを置くパッケージと、通す読み取りの経路は、次の理由でコードから決まる。読み取りの関数は `internal/chief` の非公開の関数で、同じパッケージのテストからしか呼べない。経路は、chief が NEXT 行を読む 2 つ（要件書 1 章の背景）をそのまま使う。例を取り出す関数を移すことは、要件定義で決めた（要件書 1 章の「要件定義の際に決めたこと」）。

本書の行番号は、コミット `790652e3`（`origin/main`）で確かめた。

概念モデルとシステム構成の Mermaid の図は省く。足すのはテストとテストの関数で、コンポーネントの関係は変わらない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「May be omitted」）。

## 2. システム構成

N/A。コンポーネントの構成とデータの流れを変えない。

## 3. コンポーネント設計

### 3.1. 変更するファイル

| ファイル | 変更 | 責務 |
|---|---|---|
| `internal/chief/runpr_next_doc_test.go`（新規） | 追加 | `runpr.md` の NEXT 行の例を取り出し、chief の読み取りに通すテスト（AC-01〜AC-05） |
| `internal/doctestutil/markdown.go` と、そのテスト | 関数の追加 | 文書から、コードブロックの行と、ブロックの外のコードスパンの中身を返す関数（3.2 節）。`cmd/yt2column/docs_test.go:444` の `codeTexts` を、振る舞いを変えずに移す |
| `cmd/yt2column/docs_test.go` ほか `codeTexts` を呼ぶテスト | 呼び出しの置き換え | `codeTexts`（`docs_test.go:442-460`）の定義を消し、呼び出しを移した関数に替える。`codeSpanRE`（`docs_test.go:440`）も `internal/doctestutil` に移して公開し、`firstCodeSpan`（`docs_test.go:463`）など `cmd/yt2column` のほかの使用箇所もそれに替える。正規表現の定義を 2 つにしないためである |
| `CLAUDE.md` | 1 項目の追記 | 「Development Notes」の文書を読むガードの一覧（`CLAUDE.md:753` の `internal/chief` の読み取りを挙げる箇所）に、`runpr.md` の NEXT 行の例を足す（AC-06） |

`codeTexts` を呼ぶテストは、`cmd/yt2column/adr_candidates_docs_test.go:473`・`:489`、`cmd/yt2column/fable_state_docs_test.go:51`、`cmd/yt2column/review_records_test.go:1236`・`:1256`・`:1331` にある。

新しいテストのファイル名は実装で変えてよい。ただし、ファイルは `internal/chief` の中の `_test.go` とし、名前は `screen` で始めない。`screen` で始まるファイルは、画面の読み取りとして CLAUDE.md「Declare, don't infer」の例外の対象になるためである。このテストは画面を読まず、文書を読む。

### 3.2. 例の取り出し

-   `runpr.md` は `doctestutil.ReadDoc`（`internal/doctestutil/read.go:13`）で、リポジトリの中の相対パスから読む。
-   節は `doctestutil.SectionOutsideBlocks`（`internal/doctestutil/markdown.go:44`）で、見出しから探す。step 4 は `## 4. Confirm, record, and merge`（`runpr.md:777`）、step 5 は `## 5. Switch branch, then show NEXT`（`runpr.md:1338`）である。コードブロックの中の `#` の行で節が切れないよう、この関数を選ぶ。見出しが見つからなければ、テストは失敗する（AC-03）。
-   節から、移した関数でコードブロックの外のコードスパンの中身を取り出す。この関数は、コードブロックの外の行をつないでからスパンを探すので、行をまたぐスパンも 1 つとして取れる。スパンの中の改行と、それに続く空白は、1 つの空白にまとめてから次の判定と読み取りに渡す（要件書 F-001）。`runpr.md` は、`internal/chief/task_test.go:27` の `repoContext` と同じく、パッケージからの相対パス（`../../.claude/commands/runpr.md`）で読む。
-   スパンのうち、`nextNaming`（`internal/chief/screen_transcript_report.go:293`）に合うものを例とする。ただし、コロンの後に何もないもの（`runpr.md:1399` の `NEXT: `）は除く（要件書 F-001）。コードブロックの行は例にしない。
-   2026-11-19 の `runpr.md` では、step 4 から 6 つ（`runpr.md:1090` の 2 つと、`:1092`・`:1095`・`:1097`・`:1098` の各 1 つ）、step 5 から 2 つ（`runpr.md:1401`・`:1403`）の例が取れる。この数はテストに書かない。数の下限は、節ごとに 1 つ以上とする（AC-03）。

### 3.3. 読み取りの経路

例のそれぞれを、1 行のテキストとして次の 2 つに渡す。

| 経路 | 関数と戻り値 | 比べる値 | chief が使う場所 |
|---|---|---|---|
| 画面 | `readRunprNext`（`internal/chief/screen_report.go:72`）、`RunprNext` を返す | `.Kind` | OpenCode の pane の最後の報告（`readRunprReport`、`screen_report.go:233`）と、画面全体の NEXT 行（`readScreenNext`、`screen_report.go:263`） |
| transcript | `readReportNext`（`internal/chief/screen_transcript_report.go:303`）、`RunprReport` を返す | `.Next.Kind` | Claude Code の pane の最後の報告（`autosend.go:506`・`:557`） |

比べる値が `NextNone` か `NextCommand` であれば、読めたとする。`NextUnmatched` か `NextAbsent` であれば失敗とし、節と例のテキストと経路を示す（AC-04）。

コマンドとタスク番号の中身（`/mkarch` か、`0089` か）までは比べない。比べる正解を作るには、例を読み直すか、テストに写すしかないためである。前者は同じ読み取りを 2 度行うだけになり、後者は要件書 4.5 節に反する。

## 4. エラーハンドリング設計

N/A。製品コードのエラーの扱いを変えない。テストの失敗の形は 3.2 節と 3.3 節にある。

## 5. セキュリティ考慮事項

N/A。`docs/dev/security.md` の条件のどれにも当たらない（[01_requirements.md](01_requirements.md) 4.2 節）。

## 6. 処理フローの詳細

N/A。chief の処理の流れは変わらない。

## 7. テスト戦略

-   新しいテスト（3.1 節）が AC-01〜AC-04 を確かめる。テストの本体は、文書全体のテキストを受け取り、節ごとの失敗を返す関数にする。ファイルを読む部分は、その関数に `runpr.md` の中身を渡すだけにする。テストが想定した理由で失敗できることは、実装の際に確かめる。確かめ方は、`runpr.md` の中身を変えたコピーをこの関数に渡すことである（`CLAUDE.md`「Testing Strategy」）。
-   移した関数には `internal/doctestutil` のテストを足す。`cmd/yt2column` の既存のテストは、呼び出しを替えた後もそのまま通る。
-   AC-05 は、テストが `internal/chief` パッケージにあり、ビルドタグ `test` で走ることで満たす。`doc-guard` ジョブのコマンド（`.github/workflows/ci.yml`）は `./internal/chief` を含む。
-   AC-06 は PR のレビューで読む（`manual`）。AC-07 は記録した diff を読む（`diff`）。

## 8. 実装の優先順位

1 フェーズ、1 本の PR で、関数を移し、テストと `CLAUDE.md` の 1 項目を足す。

## 9. 将来の拡張性

-   chief が NEXT 行を読む経路を足したら、3.3 節の表とテストにも足す。
-   `runpr.md` の step の番号か見出しが変わったら、テストの見出しも変える。見出しが見つからないとテストが失敗するので、変え忘れは残らない。
