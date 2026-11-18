# 要件定義書：/runpr の NEXT 行の例を chief の読み取りで確かめる

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `simple` |
| Created | 2026-11-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`/runpr` の定義（`.claude/commands/runpr.md`）が定める NEXT 行の形を、chief が読める形のまま保つガードの要件を定める。

**背景:**

-   chief は、`/runpr` の報告の NEXT を、自分で決めた次のコマンドと照らし合わせる。NEXT は `NEXT: <値>` の形の行（前に `- ` があってもよい）からだけ読む（`internal/chief/screen_report.go:56-62` の `nextLine`・`nextCommand`・`nextNoneValue`）。値は `なし` か、`/mkarch`・`/mkplan`・`/runplan` のどれかと 4 桁のタスク番号である。そのあとに全角の括弧で理由などが続いてもよい。読み取りの経路は 2 つある。OpenCode の pane は画面から読む（`readRunprNext`、`screen_report.go:72`。画面全体を読む `readScreenNext` も同じ関数を使う、`internal/chief/autosend.go:517`）。Claude Code の pane は transcript から読む（`readReportNext`、`internal/chief/screen_transcript_report.go:303`）。どちらの経路も、NEXT 行の値を同じ `readNextValue` で読む。
-   issue #900 では、PR #897 のマージの後、OpenCode の pane の最後の報告に `NEXT:` で始まる行がなかった。NEXT は見出しと番号付きのリストで書かれていた。そのため chief は報告に NEXT がないものとして扱い、リセット（`/new`）を送らなかった。当時の `runpr.md` は、マージの後の最後の報告に `NEXT:` の行を求めていなかった（issue #900「原因」）。
-   その後、#940（タスク 0095、2026-11-17）が `runpr.md` の step 5 を変えた。最後の報告は、chief が読める形の NEXT 行をちょうど 1 行持つ（`.claude/commands/runpr.md:1396-1403`）。マージの質問の要約の 6 行目も、NEXT を同じ形で書く（`runpr.md:1046`・`1090-1098`）。ADR-0046 は「形は `runpr.md` の step 5 が定め、chief は文法を広げない」と決めている（`docs/dev/adr/0046-chief-of-staff.md:41`）。
-   ただし、`runpr.md` の NEXT 行の例が chief の文法で読めることを確かめるものはない。`runpr.md` を読むテストのうち、NEXT 行の形を見るものはない（`cmd/yt2column/codex_review_docs_test.go` と `cmd/yt2column/fixpr_docs_test.go` は、呼び出しの形とフィルタだけを読む）。chief の文法か `runpr.md` の形のどちらかが変わって食い違っても、気づくのは次に `/runpr` を走らせて chief がリセットを送らなかったときになる。

以上の行番号は、コミット `790652e3`（`origin/main`）で確かめた。

対応 issue: [#900](https://github.com/isseis/yt2column/issues/900)（関連: [#892](https://github.com/isseis/yt2column/issues/892)、タスク 0088・0095）

**本書の記述範囲:** 本書は、ガードが何を確かめ、どこで走り、何を変えないかを定める。ガードの置き場所と、例の取り出し方は `02_architecture.md` で、確かめ方は `03_implementation_plan.md` で定める。

**issue #900 で決めたこと:**

-   **NEXT 行は `runpr.md` の側で必ず書かせ、番号付きの `/clear` の案内はそのまま残す。** chief の文法を広げず、報告の書き方を定めるほうで揃えるためである。この部分は #940 で入っている（背景）。
    -   ADR: なし。決定は ADR-0046 に入っている（#940 が足した）。
-   **その形を守っていることを、chief が読む側のテストか文書のガードで確かめる。** chief が頼っている形を、`/runpr` の側の文が保証しているかどうかを、偶然に頼らず確かめるためである。
    -   ADR: なし。ADR-0046 の決定（形は `runpr.md` が定め、chief は文法を広げない）を確かめる手段を足すだけで、決定は変わらない。

**要件定義の際に決めたこと（2026-11-19）:**

-   **進め方は簡易な進め方（`simple`）とする。この変更は、除外される変更に当たらない。** 足すのは chief のパッケージのテスト 1 つと、`CLAUDE.md` の一覧の 1 項目である。新しい仕組みは作らず、どの ADR の決定も変えず、未決の判断も少ない。除外の判断：次のどれも変えない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。
    -   `docs/dev/security.md` の規則と前提
    -   開発の道具（`fixprguard`・`codexreview`・`mergepr`）とその信頼境界。chief が送るものと、送る条件も変わらない
    -   外部依存
    -   ADR: なし。本タスクの中で閉じる、進め方の選択である。
-   **本タスクで扱うのは、issue #900 の 2 つ目（ガード）だけとする。`runpr.md` の文と chief の文法は変えない（F-002）。** 1 つ目は #940 で入っているためである。
    -   採らなかった案：コマンドを名指す NEXT 行を含む `/runpr` の報告（OpenCode）を新しいサンプルとして採り直す案。今のサンプルの NEXT 行は、どれも `NEXT: なし（…）` である（`internal/chief/testdata/screens/opencode-runpr-report.txt:188`、`internal/chief/testdata/transcript/claude-runpr-report.jsonl`）。人手で `/runpr` を 1 回走らせる手間がかかる。文書の形と chief の文法の食い違いは、ガードで捉えられる。
    -   採らなかった案：タスクにせず、#940 で直ったとして #900 を閉じる案。2 つ目の確かめが残らない。
    -   ADR: なし。本タスクの中で閉じる。
-   **ガードは、step 5 の最後の報告の NEXT 行の例と、step 4 のマージの質問の要約の 6 行目の例の両方を確かめる（F-001）。** chief は画面全体の NEXT 行を読み（`autosend.go:517` の `readScreenNext`）、食い違う行があれば読めないものとする（`screen_report.go:65-71`）。そのため、マージの質問の要約の NEXT 行も chief に届きうる。
    -   採らなかった案：issue #900 が名指す step 5 の例だけを確かめる案。step 4 の形が変わったときに気づけない。
    -   ADR: なし。本タスクの中で閉じる。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `runpr.md` が定める NEXT 行の形と、chief の NEXT の文法が食い違ったら、`make test` と CI で気づけるようにする。

### 2.2. スコープ (In Scope)

-   `runpr.md` の step 4 と step 5 にある NEXT 行の例を、chief の 2 つの読み取りの経路に通して確かめるテスト（F-001）。
-   `CLAUDE.md` の「Development Notes」の、文書を読むガードの一覧への 1 項目の追記（F-001）。

### 2.3. スコープ外 (Out of Scope)

-   `runpr.md` の文の変更と、chief の NEXT の文法の変更（F-002）。理由は 1 章の「要件定義の際に決めたこと」にある。
-   エージェントが実際に書く報告が、定義どおりの NEXT 行を持つかどうかの確かめ。ガードが確かめるのは文書の例で、エージェントの出力ではない。エージェントの書き方は、サンプルを採るときと、chief の自動送信の結果（`report_checks` の `no_next` など）で見る。サンプルの採り直しはしない（1 章）。
-   長い報告が画面から読めない問題（[#892](https://github.com/isseis/yt2column/issues/892)）。タスク 0095 が扱った。
-   ADR-0046 の変更。決定は変わらない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: runpr.md の NEXT 行の例を chief の読み取りで確かめる

本書の「NEXT 行の例」は、`runpr.md` の step 4（見出し `## 4.`）と step 5（見出し `## 5.`）の節にあり、`NEXT: ` に続けて空白でない文字を持つ、次のどちらかのテキストを指す。

-   インラインのコードスパンの中身
-   フェンスで囲んだコードブロックの 1 行

形の定義として書かれた、値のない `NEXT: ` そのもののコードスパン（`runpr.md:1399`）は例に含めない。

**Acceptance Criteria**:
- **AC-01**: テストは、step 5 の節の NEXT 行の例のそれぞれを、chief の画面からの読み取りと transcript からの読み取りの両方に 1 行のテキストとして渡す。どちらの結果も「次のコマンドなし」か「コマンドとタスク」であることを確かめる。
- **AC-02**: テストは、step 4 の節の NEXT 行の例のそれぞれを、AC-01 と同じく確かめる。
- **AC-03**: step 4 と step 5 のどちらかの節に NEXT 行の例が 1 つもないとき、または節が見つからないとき、テストは失敗する。
- **AC-04**: 例のどれかが、どちらかの読み取りで「読めない NEXT」か「NEXT なし」になるとき、テストは失敗する。失敗のメッセージは、その節と例のテキストと、読めなかった経路を示す。
- **AC-05**: テストは `make test` と、CI の `doc-guard` ジョブ（`go test -tags test ... ./internal/chief`）で走る。
- **AC-06**: `CLAUDE.md` の「Development Notes」の、文書を読むガードの一覧（`internal/chief` の読み取りを挙げる箇所）が、`runpr.md` の NEXT 行の例を読むことを述べる。

#### F-002: 文書と文法を変えない

**Acceptance Criteria**:
- **AC-07**: 実装の PR の差分は、`internal/chief` のテストファイル（`_test.go`）の追加と変更、`CLAUDE.md` の AC-06 の箇所、本タスクのディレクトリ（`docs/tasks/0107_runpr_next_line/`）に限る。`.claude/commands/runpr.md` と、`internal/chief` の `_test.go` 以外のファイルは変わらない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A。リポジトリの 1 つのファイルを読むテストを 1 つ足すだけである。

### 4.2. セキュリティ (Security)
-   N/A。`docs/dev/security.md` の条件（外部コマンドの起動、API キー、キャッシュ、ネットワーク、信頼できない文字列の埋め込み）のどれにも当たらない。テストが読むのはリポジトリの中の文書で、chief が送るものと送る条件は変わらない。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A。chief の振る舞いを変えない。

### 4.4. 互換性 (Compatibility)
-   chief の protocol 番号を変えない。道具を入れ直す必要はない。

### 4.5. 保守性 (Maintainability)
-   テストは例の文字列を写し持たず、`runpr.md` から読む（`CLAUDE.md`「Testing Strategy」の「pin a literal copied into the test」の禁止）。`runpr.md` の例が変わっても、chief が読める限りテストは変更なしで通る。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   文書のテストは、値と構造と実行できる例を確かめ、文の意味を確かめない（`CLAUDE.md`「Testing Strategy」）。NEXT 行の例は、chief の文法という定義元と照らし合わせる値として扱う。
-   Go のコメント、識別子、文字列リテラルは英語で書く（`CLAUDE.md`「Go Idioms」）。`runpr.md` の見出しは英語なので、日本語のリテラルは要らない。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **NEXT 行:** `/runpr` の報告で、次のコマンドを `NEXT: <値>` の形で示す行（`runpr.md` step 5 の「NEXT line」）。
-   **NEXT 行の例:** F-001 の冒頭で定めるテキスト。
-   **chief の読み取り:** chief が NEXT 行を読む 2 つの経路。画面から読む `readRunprNext` と、transcript から読む `readReportNext`（1 章の背景）。
