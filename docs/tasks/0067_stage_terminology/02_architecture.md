# アーキテクチャ設計書：用語「段階」（stage）を「ステージ」に改め、「構成要素」「工程」と使い分ける

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-06 |
| Review date | 2026-11-08 |
| Reviewer | isseis |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（要件定義書。以下、要件書）の設計である。既存のファイルの内容は、コミット `bc4b71bd` のファイルで確かめた。`file:line` の行番号は、このコミットのものである。F-NNN・AC-NN は要件書の項番、H-NN は [design_handoff.md](design_handoff.md) の項番を指す。申し送りの H-02・H-03 への対応は付録 A に、受け入れ基準との対応は付録 B にまとめる。

本書は、要件書 1 章の表と 6 章の用語（ステージ・構成要素・工程）を同じ意味で使う。そのほかに、次の用語を使う。

-   **対象の文書:** 要件書 2.2 の定義のとおり、実装時点の `main` の `docs/dev/` の下（ADR とその索引を含む）、`prompts/README.md`、用語集 `docs/translation_glossary.md` である。
-   **出現箇所:** 対象の文書の中で「段階」という語が現れる 1 か所。1 行に複数あれば、それぞれを 1 つと数える。
-   **割り当て:** 出現箇所ごとに、それが指すもの（ステージ・構成要素・工程・一般の語）を決め、置き換える語を決めること。2026-11-06 時点の割り当ては要件書の付録 A・付録 B にある。
-   **許可の一覧:** 置き換えずに残す「段階」を、（ファイル、文言、回数）の組で並べた一覧（3.7）。要件書の付録 B を、一回の確認（次の項）のスクリプトが機械的に読める形にしたものである。
-   **一回の確認:** タスクのディレクトリの `_checks/` の下に置き、PR の基点と head に対して走らせるスクリプト（要件書の手続き `requirements_process.md` §4）。恒久的なテストではない。

## 1. 設計の全体像 (Design Overview)

### 1.1. 設計原則

-   **語は、出現箇所が指すもので選ぶ。** すべてを「ステージ」にはしない（要件書 1 章）。出現箇所ごとに、1.4 の概念モデルと 6.1 の判定で、ステージ・構成要素・工程・一般の語のどれかに割り当てる。
-   **文の意味は変えない。** 変えるのは語と、語を変えたことで必要になる文の組み立て（助数詞など）だけである。例外は、用語集の定義（F-001）と `production_profile.md` の定義の文（AC-10）の 2 つで、どちらも要件書が書き換えを定めている。
-   **コードは変えず、文書のテストの locator だけを変える。** 製品とテストの Go の変更は `cmd/yt2column/docs_test.go` の 1 か所の locator と、その失敗のメッセージだけである（AC-16、3.3）。一回の確認のプログラム（`docs/tasks/0067_stage_terminology/_checks/`）は製品にもテストにも組み込まれないので、AC-16 の「Go のコード」に数えない（実装計画 §1.3 の読み方）。識別子・コメント・ほかのメッセージの英語の stage は変えない（要件書 2.3）。
-   **恒久的なテストは足さない。** 文言の変更は PR のレビューで読む（`CLAUDE.md` の「Document tests check values, structure, and executable examples, never prose」）。範囲と「変えないもの」は一回の確認で確かめる（7.3）。

### 1.2. 設計の際に決めたこと（2026-11-06）

利用者に確かめて決めたこと:

-   **ADR-0002 の題名は「生成の後に字数の調整と検証・推敲の 2 つの工程を置き、検証・推敲は直す箇所の一覧だけを返す」とする（H-02）。** 理由: 語を置き換え、助数詞「つの」を補うだけで、文の組み立てが変わらないため。索引 `docs/dev/adr/README.md:10` の題名も同じ文言にする（AC-07）。ADR の判断: ADR は作らない。文言の決定で、`CLAUDE.md` の「Design decisions (ADR)」の基準では主要な決定ではない（「a wording」に当たる）。
-   **ADR-0004 の題名は「本番の構成を 3 つの工程とも claude-opus-5-5（effort high）とし、品質の目標と製品の制約を共通の文書に置く」とする（H-02）。** 理由: 今の題名（`docs/dev/adr/0004-production-profile.md:1`）の「段階」を「工程」に置き換えるだけでは、「本番の構成を 3 つの工程とする」と読め、工程の数が本番の構成によって決まるかのように見えるため。決定の中身（3 つの工程のすべてを同じモデルと effort で行う。同 `:17`）は変えない（AC-09）。索引 `docs/dev/adr/README.md:12` も同じ文言にする（AC-07）。ADR の判断: 同上。
-   **`docs_test.go` の表の失敗のメッセージは、列名の locator の定数から組み立てる（H-03）。** 列名「工程」を定数に置き、`doctestutil.FindTable` の引数と失敗のメッセージの両方がその定数を使う（3.3）。理由: 日本語の literal が locator の 1 か所にまとまり、guard-test literal exception（`CLAUDE.md` の「Go Idioms」）の範囲に収まるため。同じ関数の「the profile table has no %s column」（`cmd/yt2column/docs_test.go:582`）と同じ書き方になる。メッセージを英語だけで書く案は、表が見つからないときに、どの列名で探したかが分からなくなるので採らない。ADR の判断: ADR は作らない。テストの書き方の決定で、タスクの中で閉じる。
-   **`docs_test.go` の識別子・コメント・ほかのメッセージの英語の stage は変えない。** 例は `profileStages`（`cmd/yt2column/docs_test.go:552`）、「which is not a stage of the profile」（同 `:587`）、「the profile's configuration block and stage table」（同 `:38`）である。最後のものは、このタスクが直す `CLAUDE.md:650` と同じ句であり、直した後は `CLAUDE.md` の「step table」とテストのコメントの「stage table」が食い違う。理由: 要件書 2.3 は、コードの識別子とコメントを範囲外とし、文書のテストの例外を列名の literal とそのメッセージに限っているため。ADR の判断: ADR は作らない。このタスクの範囲の決定で、主要な決定ではない。
-   **`production_profile.md` の表の「投稿」の行は、ステージではなく、パイプラインの外の処理とする。要件書の AC-10 を改め、要件書を `draft` に戻した。** その行が実行する `SlackWebhookPublisher` は `Publisher` ではない（`internal/publisher/publisher.go:1-5`）。承認した版の AC-10 は、9 行目の文が投稿をステージと述べることを求めており、新しいステージの定義と食い違っていた。ADR の判断: ADR は作らない。文言の決定で、主要な決定ではない。
-   **タスク 0065 の PR ② とは、どちらが先にマージされても扱えるようにする（6.2）。** 一回の確認の範囲は、ファイルの一覧ではなく、要件書 AC-14・AC-16 の禁止の規則で判定する（7.3）。ADR の判断: ADR は作らない。このタスクの範囲と確かめ方の決定で、タスクの中で閉じる。
-   **付録 B にない一般の語の「段階」が実装時点で見つかったときは、F-002（「新たに見つかった箇所にも同じ規則を当てはめる」）に従い、実装者が 6.1 の判定で一般の語と判断して、許可の一覧に足す。** 足した組は PR の説明に挙げ、PR のレビューで承認する。一回の確認は、使った許可の組をすべて出力する（7.3）。これは、AC-06 の「付録 B に挙げる文言」を、付録 B に加えて、F-002 に従って PR のレビューで承認した追加分も含むものと読む、という解釈である。要件書に戻して付録 B を直す案は、承認のやり直しの手間に見合わないので採らない。ADR の判断: ADR は作らない。同上。

レビュアーに個別には確かめず、要件書と現行の規則から導いたこと（PR のレビューで異論があれば改める）:

-   **`production_profile.md` の 9 行目は、3.2 の文に書き換える（AC-10）。** 要件書は文の内容を定め、文言を設計に委ねた。
-   **PR は 1 本にする。** 変更は文書の文言とテストの 1 か所で、互いに依存する（列名とテストの literal、ADR の題名と索引）。分けると、途中の `main` で「段階」と新しい語が混在する。
-   **`CLAUDE.md` の 3 か所の英文は 3.4 の文にする（AC-11・AC-12）。** 要件書が指すもの（component の 4 つ、step）を定めているので、残る選択は英文の組み立てだけである。
-   **要件書の付録 A と 6.1 の判定が食い違うときは、付録 A に従う。** 付録 A は承認した割り当てである。食い違いは PR の説明に挙げる。

### 1.3. なぜ「段階」を一律に「ステージ」へ置き換えないか

一律の置き換えは最も簡単だが、要件を満たさない。2026-11-06 時点の出現箇所の多くは、`ArticleWriter` の中の生成・字数の調整・検証・推敲（工程）を指している（要件書の付録 A）。たとえば `docs/dev/project_overview.md:58` の「`--after <stage>` でその段階から再開して」は、`writer.Stage` の値（`generation`・`shorten`・`review`。`internal/writer/artifact.go:39-43`）を指す。これを「ステージ」にすると、パイプラインの `pipeline.Stage`（`transcript`・`write`・`publish`。`internal/pipeline/pipeline.go:37-41`）と取り違える。要件書 F-002 は、指すものに従って語を選ぶことを求めている。

### 1.4. 概念モデル

3 つの語が指すものの関係を図 1 に示す。囲み（subgraph）は「含む」を表す。破線の矢印 A -.-> B は、「A が B を呼ぶ」を表す。

```mermaid
flowchart TB
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef newpkg fill:#ffe8f5,stroke:#d946ef,stroke-width:2px,color:#701a75;

    subgraph COMP["構成要素（component）"]
        subgraph STAGE["ステージ（stage、pipeline.Stage）"]
            SRC["TranscriptSource"]
            subgraph WRITER["ArticleWriter"]
                subgraph STEP["工程（step、writer.Stage）"]
                    DRAFT["生成"]
                    SHORTEN["字数の調整"]
                    REVIEW["検証・推敲"]
                end
            end
            PUB["Publisher"]
        end
        LLM["LLMClient"]
    end

    STEP -.-> LLM

    class SRC,PUB enhanced
    class WRITER enhanced
    class LLM process
    class DRAFT,SHORTEN,REVIEW newpkg

    subgraph Legend["凡例"]
        L1["ステージに当たる構成要素"]
        L2["ステージに当たらない構成要素"]
        L3["工程"]
    end
    class L1 enhanced
    class L2 process
    class L3 newpkg
```

図 1 の色は、この図に限り、変更の有無ではなく語の区別を表す。`ArticleWriter` は、工程を含むステージなので、ステージの色の囲みで示した。

-   **ステージ:** パイプラインが順に呼ぶ 3 つ。`internal/pipeline/pipeline.go:19` の `Stage` の値（`transcript`・`write`・`publish`）が対応する。
-   **構成要素:** interface とデータ型を持つパイプラインの部品の全体で、ステージの 3 つに `LLMClient` を加えた 4 つ（要件書 1 章、`docs/translation_glossary.md:56`）。`LLMClient` を含むかどうかが、構成要素とステージの区別の決め手である。
-   **工程:** ステージ `ArticleWriter` の中の 3 つ。`internal/writer/artifact.go:18` の `Stage` と、API の `steps` が対応する。

パイプラインの外の処理もある。サーバの Webhook への投稿は、`Publisher` ではない `SlackWebhookPublisher` が行う（`internal/publisher/publisher.go:1-5`）。これはステージにも工程にも当たらない（3.2）。

### 1.5. 実装の際に決めたこと（2026-11-08）

-   **ADR の「経緯」の節にある「段階」は、行を増減せずに語だけを置き換える。** 実装の時点の `main` には、タスク 0065 の PR ②（#716）が足した ADR-0019・ADR-0022・ADR-0023・ADR-0025 があり、「経緯」の節にも「段階」がある（0019 の 42 行目、0022 の 44 行目、0023 の 41 行目、0025 の 41 行目）。1.2 と 7.3 は経緯の節を変えない形で書いたが、それは ADR-0001〜0004 の経緯に「段階」がない時点の記述である。要件書 F-002 は実装時点で増えた出現箇所も置き換えるとし、AC-06 は残る「段階」を付録 B の文言だけとし、AC-09 は経緯の節に行が加わらず行が消えないことだけを求める。そこで、5 か所を 6.1 の割り当てどおりに置き換え、経緯の確かめ（7.3）は「基点と head で見出しが 1 つずつ、行数が同じで、各行が基点の行の『段階』を割り当て語に置き換えたもの」に改める。ADR の判断: ADR は作らない。一回の確認の確かめ方の決定で、タスクの中で閉じる。レビュアーは 2026-11-08 のセッションでこの扱いを承認した。

## 2. システム構成 (System Structure)

### 2.1. 全体構成

変える文書と、変える箇所を読むテスト・道具の関係を図 2 に示す。矢印 A → B は、「A が B を読む」を表す。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    GLOSS[("docs/translation_glossary.md")]
    PROFILE[("docs/dev/production_profile.md")]
    ADRS[("ADR")]
    INDEX[("docs/dev/adr/README.md")]
    TARGETS[("対象の文書と CLAUDE.md")]

    DOCSTEST["checkProfileTable<br>(cmd/yt2column/docs_test.go)"]
    ADRTEST["TestADRRecords<br>(cmd/yt2column/adr_test.go)"]
    JAPROSE["japrose"]
    CHECKS["一回の確認<br>(_checks/)"]

    DOCSTEST -->|"本番の構成の表"| PROFILE
    ADRTEST -->|"1 行目の題名"| ADRS
    ADRTEST -->|"索引の題名"| INDEX
    JAPROSE -->|"用語の基準と旧称"| GLOSS
    CHECKS -->|"残る「段階」と差分"| TARGETS

    class GLOSS,PROFILE,ADRS,INDEX,TARGETS data
    class ADRTEST,JAPROSE process
    class DOCSTEST,CHECKS enhanced

    subgraph Legend["凡例"]
        L1[("変える文書")]
        L2["変えないテスト・道具"]
        L3["変える、または足すテスト・確認"]
    end
    class L1 data
    class L2 process
    class L3 enhanced
```

変える文書を読むが、変える箇所を読まないテストは、次のとおりで、結果は変わらない。どれも値・見出し・表の locator を読み、「段階」を含む文や、このタスクが変える列名・題名を読まない。

-   `TestPromptsREADMEMatchesContract`（`internal/pipeline/pipeline_test.go:709`）: `prompts/README.md` の「…で参照できる値」の節だけを読む。変える 3 行目は、どの節にも入らない。
-   `cmd/yt2column/docs_test.go` のほかのテストと `cmd/yt2column-server/docs_test.go`: `security.md`・`project_overview.md`・`production_profile.md` の環境変数の名前・`costPrefix`・`16,383`・設定のブロックなどの値を読む。
-   `cmd/yt2column/docpaths_test.go`・`cmd/yt2column/makefile_test.go`・`cmd/yt2column/extension_docs_test.go`: `CLAUDE.md` のリンクのパス、`make` のターゲット、`ext-` のターゲットを読む。

Go のテストで「段階」を含む literal は、`cmd/yt2column/docs_test.go:574`・`:576` だけである（`bc4b71bd` で `git grep -n 段階 -- '*.go'` を実行して確かめた）。

### 2.2. コンポーネント配置

次の表は、2026-11-06 の時点で変えるファイルである。実装時点の `main` で対象の文書に新しい出現箇所があれば、そのファイルも変える（6.2）。

| ファイル | 変更 | 要件 |
|---|---|---|
| `docs/translation_glossary.md` | 「段階」の項を「ステージ」の項にし、「工程」「構成要素パッケージ」「フェーズ」の項の説明を直す（3.1） | F-001 |
| `docs/dev/project_overview.md` | 出現箇所の語の置き換え | F-002 |
| `docs/dev/adr/0001-llm-adapter-stdlib.md` | 37 行目の語の置き換え | F-002 |
| `docs/dev/adr/0002-post-generation-steps.md` | 題名と本文の語の置き換え（17 行目は「処理の順序」） | F-002・AC-07 |
| `docs/dev/adr/0003-llm-config-contract.md` | 11 行目の語の置き換え | F-002 |
| `docs/dev/adr/0004-production-profile.md` | 題名と本文の語の置き換え（19 行目の「観点と段階の基準」は残す） | F-002・AC-07・AC-15 |
| `docs/dev/adr/README.md` | ADR-0002・ADR-0004 の題名 | AC-07 |
| `docs/dev/production_profile.md` | 9 行目の定義の文、2 つの表の列名、ほかの出現箇所（3.2） | F-002・AC-08・AC-10 |
| `docs/dev/security.md` | 58・59・66・80 行目の語の置き換え（145 行目は残す） | F-002・AC-15 |
| `prompts/README.md` | 3 行目の語の置き換え | F-002 |
| `cmd/yt2column/docs_test.go` | 表の locator を定数にし、列名を「工程」にする（3.3） | AC-08・AC-16 |
| `CLAUDE.md` | 9・363〜364・650 行目の英文（3.4） | F-003 |
| `docs/tasks/0067_stage_terminology/_checks/` | 一回の確認のスクリプト（新規、7.3） | AC-04・AC-06・AC-09・AC-13〜AC-16 |

`docs/dev/guides/cli.md` と `docs/dev/guides/mergepr_guide.md` は対象の文書に入るが、「段階」は一般の語だけなので変えない（要件書の付録 B）。`README.md`・`README.en.md`・`docs/dev/conventions/package_reference.md`・`docs/tasks/` の下（このタスクのディレクトリを除く）は変えない（AC-14）。

### 2.3. データフロー：1 つの出現箇所を直す

1 つの出現箇所を直す流れを図 3 に示す。判定の中身は 6.1、文の組み立ての規則は 3.6 にある。

```mermaid
sequenceDiagram
    participant IMPL as 実装者
    participant DOC as 対象の文書
    participant REQ as 要件書の付録 A・B

    IMPL->>DOC: 「段階」を検索する
    IMPL->>REQ: 出現箇所の割り当てを探す
    alt 付録にある
        REQ-->>IMPL: 割り当てた語、または残すこと
    else 付録にない（実装時点の main で増えた）
        IMPL->>IMPL: 6.1 の判定で割り当てる
    end
    IMPL->>DOC: 語を置き換え、文の組み立てを整える（3.6）
```

## 3. コンポーネント設計 (Component Design)

### 3.1. 用語集（F-001）

-   **ステージの項:** 今の「段階」の項（`docs/translation_glossary.md:49`）の日本語の列を「ステージ」にする。説明は、指すもの（`TranscriptSource`・`ArticleWriter`・`Publisher`。`pipeline.Stage` の `transcript`・`write`・`publish`）を述べ、`ArticleWriter` の中の生成・字数の調整・検証・推敲の 3 つを「工程」と呼んで区別する。今の説明の「API の `steps` として返すとき」という条件は除く。新しい使い分けでは、`ArticleWriter` の中の 3 つの処理は常に工程だからである。英語の "post-generation steps" の記述は引き継ぐ。備考の末尾に「旧称: 段階」を足す（用語集の規則 4、`docs/translation_glossary.md:10`）。項の位置（表の中の行の順）は変えない（AC-01・AC-04）。
-   **構成要素パッケージの項:** 今の説明（`docs/translation_glossary.md:56`）の「3 つの段階」を「3 つのステージ」にする。加えて、次の 3 点を述べる（AC-02）。
    -   構成要素は、3 つのステージと `LLMClient` の 4 つを指す。
    -   ステージ・構成要素・工程の使い分け（要件書 1 章の表の「使い分けの手がかり」の列）。`LLMClient` を含むかどうかが、構成要素とステージの区別の決め手である。
    -   一般の語としての「構成要素」（`README.md:240` の「構成要素の組み立て」など）は、この定義の外である。
-   **工程とフェーズの項:** 「パイプラインの「段階」（stage）とは区別する」（`:42`・`:116`）を「パイプラインの「ステージ」（stage）とは区別する」にする（AC-03）。

使い分けを「構成要素パッケージ」の項に置くのは、要件書 AC-02 の定めによる。新しい項（「構成要素」だけの項）は足さない。

### 3.2. `production_profile.md`（AC-08・AC-10）

-   **9 行目の定義の文:** 次の文に書き換える（AC-10）。

    > 記事ができるまでの処理と、各処理の役割、その処理を実行する部品を示す。表の行のうち、字幕の取得はパイプラインのステージ `TranscriptSource`、生成・字数の調整・検証・推敲はステージ `ArticleWriter` の中の工程に当たる。投稿は、パイプラインの外で、利用者の操作を受けて行う。

    今の文は「段階」がステージと工程の両方を指すと定義している（`docs/dev/production_profile.md:9`）。新しい文は、表の行がどれに当たるかを述べ、「段階」を使わない。投稿の行（同 `:17`）が実行する `SlackWebhookPublisher` は `Publisher` ではない（`internal/publisher/publisher.go:1-5`）ので、ステージと書かない（1.2）。
-   **パイプラインの表（11 行目）:** 1 列目の名前を「処理」にする。行は変えない。
-   **本番の構成の表（25 行目）:** 1 列目の名前を「工程」にする。行は変えない。この表を `checkProfileTable` が読む（3.3）。
-   **そのほかの出現箇所（3・19・23・38・81 行目）:** 要件書の付録 A の割り当てのとおりに置き換える。19 行目の「最初の段階」は、表の 1 行目（コピーした字幕の取得、ステージ `TranscriptSource`）を指すので「最初のステージ」にする。

### 3.3. `docs_test.go` の locator（AC-08・AC-16・H-03）

`checkProfileTable`（`cmd/yt2column/docs_test.go:572`）は、本番の構成の節から、列名「段階」を持つ表を `doctestutil.FindTable` で探す（同 `:574`）。表がないときの失敗のメッセージは、列名を日本語のまま書いている（同 `:576`）。

列名を定数に置き、`FindTable` の引数とメッセージの両方がこの定数を使う。メッセージは書式の引数として列名を受け取る。

```go
// profileTableColumn is the name of a column of the production profile
// table, by which checkProfileTable finds the table.
const profileTableColumn = "工程"
```

-   定数の literal は locator なので、guard-test literal exception の範囲に入る。
-   `FindTable` は、ヘッダーのどの列の名前でも一致を探す（`internal/doctestutil/tables.go:63`）。探す範囲は本番の構成の節だけ（`productionProfileSection`、`cmd/yt2column/docs_test.go:39`）なので、パイプラインの節の表（列名「処理」）を拾うことはない。
-   識別子 `profileStages`・`reviewProfileStage`、コメント、ほかのメッセージは変えない（1.2）。

### 3.4. `CLAUDE.md` の英文（F-003）

| 行 | 今の文言 | 変えた後 |
|---|---|---|
| 9 | 「(the template, provider, model, and effort of each stage)」 | 「(the template, provider, model, and effort of each step)」 |
| 363〜364 | 「Every pipeline stage (`TranscriptSource`, `LLMClient`, `Publisher`) is an interface with a fake for tests.」 | 「Every pipeline component (`TranscriptSource`, `ArticleWriter`, `LLMClient`, `Publisher`) is an interface with a fake for tests.」 |
| 650 | 「the production profile's configuration block and stage table」 | 「the production profile's configuration block and step table」 |

-   363〜364 行目の新しい文は事実と合う。4 つの構成要素は、どれも interface（`internal/transcript/transcript.go:26`・`internal/writer/writer.go:77`・`internal/llm/llm.go:76`・`internal/publisher/publisher.go:17`）と、テスト用の fake（各パッケージの `testutil/mocks.go`。`ArticleWriter` は `internal/writer/testutil/mocks.go:30` の `FakeArticleWriter`）を持つ。
-   ほかの stage は変えない（AC-13）。`CLAUDE.md:300`・`:301`・`:334`・`:337` はパイプラインのステージを、`:543` は一般の語を指す。

### 3.5. 変えるテストと変えないテスト

-   **変える:** `checkProfileTable` の locator とメッセージ（3.3）。
-   **変えないが、結果が変わりうる:** `TestADRRecords`（`cmd/yt2column/adr_test.go:434`）は、ADR の 1 行目の題名と索引の題名の一致を確かめる（同 `:98` の `adrTitleMismatch`）。ADR-0002・ADR-0004 と索引を同じコミットで直せば成功する（AC-07）。
-   **既存のテストで、古い振る舞いを確かめるもの:** `checkProfileTable` の列名「段階」だけである（2.1）。

### 3.6. 言い換えの規則（H-02）

-   **助数詞:** 「N 段階」「N 段階を」は「N つの<語>」にする（例: 「3段階を束ねる」→「3 つのステージを束ねる」、「2 段階を置き」→「2 つの工程を置き」）。
-   **ADR の題名:** 1.2 の文言にする。
-   **ADR-0002 の 17 行目:** 「段階の順序は、…」を「処理の順序は、…」にする。列挙に工程でない後処理（最終の字数の確認、出典の付与）が入るため（要件書の付録 A）。
-   **`production_profile.md` の 9 行目:** 3.2 の文にする。
-   **そのほか:** 語を置き換えて文が自然な日本語にならないときは、意味を変えない範囲で組み立てを直す。直した文は PR のレビューで読む（7.4）。

### 3.7. 許可の一覧（AC-06・AC-15）

2026-11-06 の時点の許可の一覧は次のとおりである。文言は、ファイルの中の文字列そのままで、回数は `bc4b71bd` で数えた。一回の確認は、組ごとに、そのファイルだけで文言を数える。

| ファイル | 文言 | 回数 | 要件書の付録 B |
|---|---|---|---|
| `docs/dev/adr/0004-production-profile.md` | `観点と段階の基準` | 1 | 評価の尺度 |
| `docs/dev/security.md` | `固定のイベント名・段階・` | 1 | ホストのログの属性 |
| `docs/dev/security.md` | `形の検査の段階` | 1 | ホストのログの属性 |
| `docs/dev/guides/cli.md` | `2 つの段階` | 1 | 料金の区分 |
| `docs/dev/guides/mergepr_guide.md` | `\| 段階 \|`（表の列名。72・215 行目） | 2 | 手順の区切り |
| `docs/dev/guides/mergepr_guide.md` | `承認の段階` | 1 | 手順の区切り |
| `docs/dev/guides/mergepr_guide.md` | `取り消せない段階` | 1 | 手順の区切り |

表の `\|` は、表のセルの中の `|` を表す。一回の確認が持つ文言は `| 段階 |` である。この文言は `mergepr_guide.md` の中だけで数えるので、`production_profile.md` の列名を変え忘れても、この組では許されない。

実装時点で一般の語の出現箇所が増えたときは、組を足す（1.2）。

## 4. エラーハンドリング設計 (Error Handling Design)

実行時のエラーはない。失敗しうるのは文書のテストと一回の確認で、それぞれ次のとおり扱う。

| 失敗 | 原因 | 対処 |
|---|---|---|
| `checkProfileTable` の「no table with a 工程 column」 | `production_profile.md` の 25 行目の列名と定数が食い違う | 列名か定数を直す。メッセージは探した列名を示す（3.3） |
| `TestADRRecords` の題名の不一致 | ADR の 1 行目と索引の片方だけを直した | 両方を同じ文言にする |
| 一回の確認が、許されない「段階」を報告する | 置き換え忘れ、または実装時点の `main` で増えた出現箇所 | 6.1 の判定で割り当てて直す。一般の語なら許可の一覧に組を足し、PR の説明に挙げる（1.2） |
| 一回の確認が、許可の組の回数の食い違いを報告する | 残すべき「段階」を置き換えた、または文言が変わった | 置き換えを戻す。文言が `main` で変わったのなら組を直し、PR の説明に挙げる |
| 一回の確認が、禁止のファイルの変更を報告する | 範囲外のファイルを変えた | 変更を戻す |
| 一回の確認が終了コード 2 で止まる | 引数のコミットが読めない、ADR の「## 経緯」が見つからないなど、確認できない入力 | 入力を直して走らせ直す。確認できないことを成功と扱わない |

## 5. セキュリティ考慮事項 (Security Considerations)

N/A。要件書 4.2 のとおり、このタスクは条件付きのガイド `docs/dev/security.md` の適用の条件のどれにも当たらない。`docs/dev/security.md` の 58・59・66・80 行目の語を置き換えるが、述べる規則（LLM に送るもの、中間生成物の名前と中身）は変えない（AC-17）。145 行目の Native Messaging ホストのログの「段階」（ログの属性 `stage`、`internal/nmhost/relay.go:166`）は、パイプラインの語ではないので残す（要件書の付録 B）。脅威モデルの図は要らない。

## 6. 処理フロー詳細 (Processing Flow Details)

### 6.1. 出現箇所の割り当ての判定

要件書の付録 A・付録 B にない出現箇所（実装時点の `main` で増えたもの）は、図 4 の判定で割り当てる。付録にある出現箇所は、付録の割り当てに従う（1.2）。矢印 A → B は、「A の答えが B に進む」を表す。

```mermaid
flowchart TD
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    START(["「段階」の出現箇所"]) --> Q1{"パイプラインの部品か、<br>その中の処理を指すか？"}
    Q1 -->|"いいえ"| GENERAL["一般の語"]
    Q1 -->|"はい"| Q2{"ArticleWriter の中の<br>生成・字数の調整・検証・推敲か？"}
    Q2 -->|"はい"| STEP["工程"]
    Q2 -->|"いいえ"| Q2B{"ArticleWriter の中の<br>工程でない処理か？"}
    Q2B -->|"はい"| PROC["一般の語「処理」"]
    Q2B -->|"いいえ"| Q3{"指す部品の集まりに<br>LLMClient が入るか？"}
    Q3 -->|"はい"| COMP["構成要素"]
    Q3 -->|"いいえ"| STAGE["ステージ"]

    class Q1,Q2,Q2B,Q3 process
    class GENERAL,PROC,STEP,COMP,STAGE enhanced

    subgraph Legend["凡例"]
        L1["問い"]
        L2["割り当てる語"]
    end
    class L1 process
    class L2 enhanced
```

-   Q3 は、文の話題（interface、パッケージの置き場所など）ではなく、出現箇所が指す部品の集まりで決める。集まりが明示されていないときは、同じ文や直後の列挙が挙げる部品で決める。例: `project_overview.md:48` の「各段階は interface で抽象化し」は、直後の列挙が `LLMClient` を挙げるので構成要素である。`project_overview.md:42` の図の凡例「段階（interface）」は、図の中の部品が `TranscriptSource`・`ArticleWriter`・`Publisher` だけなのでステージである。
-   Q1 の「いいえ」の例: 評価の尺度（ADR-0004 の「観点と段階の基準」）、手順の区切り（`mergepr_guide.md`）、ホストのログの属性（`security.md:145`）。一般の語は許可の一覧に組を足す（3.7）。
-   Q2B の「はい」の例: 最終の字数の確認、出典の付与。工程と工程でない処理が混ざる列挙（ADR-0002 の 17 行目）も、一般の語「処理」にする（3.6）。
-   ADR-0001 の 37 行目は、LLM のアダプタ（構成要素 `LLMClient`）が `internal/transcript` に依存することを述べる。付録 A は、この行を構成要素どうしの 1 つの依存として読み、2 つの出現箇所とも構成要素とする。2 つ目の「字幕取得の段階」は、Q3 だけで判定すればステージになるが、付録 A に従う（1.2）。
-   サーバの Webhook への投稿など、パイプラインの外の処理を指す出現箇所は、Q1 で「いいえ」に進む。ステージとも工程とも呼ばず、一般の語「処理」を使う（3.2）。

### 6.2. タスク 0065 の PR ② との順序

タスク 0065 の実装計画（`docs/tasks/0065_adr_remaining/03_implementation_plan.md:162`・`:166`）は、PR ② で新しい ADR へ移す文として、ADR-0001 の 37 行目（「LLM の段階から使う」）と ADR-0004 の 22 行目（「3 つの段階で共有する」）を、今の文面のまま引用している。0065 の一回の確認は、引用した文が PR の基点にあることを確かめる（`docs/tasks/0065_adr_remaining/02_architecture.md` §7.3）。

-   **このタスクの PR が先にマージされた場合:** 0065 の PR ② の基点には引用した文がないので、0065 の一回の確認が失敗して止まる。0065 の実装者は、計画のこの 2 行を、直した後の文面で引用し直す。このタスクは `docs/tasks/` の下の他のタスクを変えない（AC-14）ので、この PR の説明に、0065 の計画の 2 行を引用し直す必要があることを書く。
-   **0065 の PR ② が先にマージされた場合:** 2 つの文は、`docs/dev/adr/` の下の新しい ADR（番号は 0019 以降。タスク 0066 が 0018 を使った）に移る。新しい ADR は対象の文書に入るので、このタスクの PR は `main` を取り込み、6.1 の判定（付録 A の ADR-0001・ADR-0004 の行の割り当て）で直す。一回の確認は、範囲をファイルの一覧ではなく禁止の規則で判定し、経緯の確かめを差分のすべての ADR に当てる（7.3）ので、この場合も成功する。

0065 の PR ②〜④ が新しく書く ADR の文に「段階」を使ったときは、用語集の「旧称: 段階」により、`japrose` の旧称の確認（`.claude/commands/japrose.md`）が指摘する。このタスクの後に `main` へ入る文書の「段階」を防ぐ恒久的な仕組みは、これだけである（7.5 の残るリスク）。

## 7. テスト戦略 (Test Strategy)

### 7.1. ユニットテスト

-   `checkProfileTable` の変更は、既存のテスト（`go test -tags test ./cmd/yt2column`）で確かめる（AC-08）。テストが列名で失敗できることを、`production_profile.md` の 25 行目の列名を一時的に別の語にして、テストが失敗することで確かめる（`CLAUDE.md` の「Every test must be able to fail for its stated reason」）。確かめたことはコミットのメッセージに書く。
-   `TestADRRecords` の成功で AC-07 を確かめる。

### 7.2. 結合テスト

-   `make test`・`make lint` が成功し、CI の `doc-guard` ジョブのテストが成功する（要件書 4.5）。拡張は変えないので `make ext-test` は要らない（`CLAUDE.md` の「Development Notes」。`README.md`・`README.en.md`・`docs/dev/guides/development.md` を変えない）。

### 7.3. 一回の確認（`_checks/`）

スクリプトは、PR の基点と head の 2 つのコミットを引数に取り、それぞれのコミットのツリー（作業ツリーではない）を読む。対象の文書は、head のツリーの `docs/dev/` の下のすべての Markdown のファイル（`main` が足したファイルを含む）と、`prompts/README.md` と、用語集である。用語集だけは、下の表の 1 つ目の行ではなく、3 つ目の行の方法で確かめる。恒久的なテストにはしない（要件書 2.3、`CLAUDE.md` の「Document tests …」の「Do not add a permanent test that a document still holds a retired specification」）。

| 確かめること | 方法 | AC |
|---|---|---|
| 対象の文書（用語集を除く）に残る「段階」は、許可の一覧の文言だけ | head の対象の文書のファイルごとに、許可の組の文言をその回数だけ除き、なお「段階」が残らないこと | AC-06 |
| 許可の組の文言は、その回数だけ残る | 各組の文言が、head のそのファイルにちょうどその回数あること | AC-15 |
| 用語集の「段階」は旧称の記述だけ | head の用語集で「段階」がちょうど 1 回、文言「旧称: 段階」の中にだけあること。表の 1 列目が「段階」の行がなく、「ステージ」の行がちょうど 1 つあること | AC-01・AC-04 |
| 禁止のファイルを変えていない | 基点と head の差分のファイルに、`docs/tasks/` の下（このタスクのディレクトリを除く）、`README.md`、`README.en.md`、`docs/dev/conventions/package_reference.md`、`extension/` の下がないこと。差分の `.go` のファイルが、このタスクのディレクトリの下のものを除くと `cmd/yt2column/docs_test.go` だけであること | AC-14・AC-16 |
| `CLAUDE.md` の変更は 3 か所だけ | head の `CLAUDE.md` で 3.4 の 3 つの「変えた後」の文言を「今の文言」に戻すと、基点の `CLAUDE.md` と一致すること。363〜364 行目の文は 2 行にまたがるので、照合では改行を空白と同じに扱う。行番号では照合しない | AC-13 |
| ADR の経緯の節は行が増減しない | 差分にある ADR（`docs/dev/adr/[0-9][0-9][0-9][0-9]-*.md` に当たるファイル。索引 `README.md` は除く）ごとに、「## 経緯」の見出しが基点と head でちょうど 1 つずつあり、その見出しから末尾までの行数が同じで、各行が基点の同じ位置の行の「段階」を割り当て語（ステージ・構成要素・工程）に置き換えたものであること。基点にない ADR は、head で見出しがちょうど 1 つあることだけを確かめる | AC-09（経緯） |

スクリプトが守ること:

-   **終了コード:** 0 は成功、1 は違反、2 は確かめられない入力（コミットが読めない、見出しが見つからないなど）である。2 を成功と扱わない。
-   **出力:** 違反は `ファイル:行: 文` の形で出す。使った許可の組と、その回数を、成功のときもすべて出す（1.2。PR のレビューで追加分を見られるようにする）。基点と head のコミット ID を出す。
-   **一致の方法:** バイトの完全一致で数える（`LC_ALL=C`、固定の文字列）。差分のファイルの一覧は名前の変更を検出しない形で取る。
-   **行をまたぐ語:** 「段」と「階」が改行で分かれた出現も数える（ファイルの改行を除いた文字列でも「段階」を探す）。

走らせ方:

-   走らせる前に `git fetch origin main` を行い、`origin/main` を head に取り込む。`git merge-base --is-ancestor origin/main HEAD` が成功することを確かめてから走らせる。PR の基点は `git merge-base origin/main HEAD` とする。`git fetch` はネットワークの操作なので、実装の手順の中で許可を得て行う。
-   PR の head が変わったら（`main` の取り込みを含む）、マージの前に最後の head で走らせ直す（AC-06 の「`main` を取り込んだ後の状態」）。
-   出力の全文と、基点・head のコミット ID を、実装計画の記録と PR の説明に残す。

スクリプトの言語とファイル名は、実装計画で決める。

### 7.4. 読んで確かめる（manual）

文の意味の同一性は、テストでは確かめられない（`CLAUDE.md` の「a test cannot tell a reworded sentence from a reversed one」）。PR のレビューで、次を読んで確かめる。

-   AC-05: 各出現箇所の語が、付録 A の割り当てのとおりであること。
-   AC-02・AC-03: 用語集の説明。
-   AC-09: ADR の決定・背景・結果・採らなかった案が、同じことを述べていること。
-   AC-10: `production_profile.md` の 9 行目の文。
-   AC-11・AC-12: `CLAUDE.md` の 3 か所。
-   AC-17: ADR を除く対象の文書で変えた文が、同じ事実・規則を述べていること。
-   許可の一覧に足した組（1.2）が、一般の語であること。

### 7.5. セキュリティテスト

N/A（5 章）。

残るリスク: このタスクの後に `main` へ入る文書が「段階」を使っても、テストは失敗しない。捉えるのは、用語集の旧称による `japrose` の指摘だけである（6.2）。用語の規則を恒久的なテストにしないのは、`CLAUDE.md` の文書のテストの方針（文の意味をテストで確かめない）による。

## 8. 実装の優先順位 (Implementation Priorities)

PR は 1 本にする（1.2）。実装の順序は次のとおりとする。

1. 用語集（3.1）。以降の置き換えは、この定義を基準にする。
2. `production_profile.md` と `docs_test.go`（3.2・3.3）。列名と locator を同じコミットで変え、テストが成功する状態を保つ。
3. ADR と索引（AC-07）。題名と索引を同じコミットで変える。
4. `project_overview.md`・`security.md`・`prompts/README.md`。
5. `CLAUDE.md`（3.4）。
6. 一回の確認のスクリプトを置き、PR の基点に対して走らせる（7.3）。

## 9. 将来の拡張 (Future Extensibility)

-   新しい文書や ADR がパイプラインの処理の単位を述べるときは、用語集の使い分けに従う。用語集の項が正本なので、後のタスクが訳語を変えるときも、用語集から変える。
-   コードの識別子（`writer.Stage`・`pricing.StageCost` など）とコメントの英語の stage は、工程を指したまま残る（要件書 2.3）。識別子の名前を step に揃えることは、コードの変更を伴う別のタスクで扱う。

## 付録 A: 申し送り（design_handoff.md）への対応

| 項目 | 対応 |
|---|---|
| H-02（言い換えた文の文言） | 3.6 に、助数詞の規則、ADR-0002 の 17 行目の扱い、`production_profile.md` の 9 行目の文（3.2）を定めた。ADR-0002・ADR-0004 の題名は 1.2 に定めた。そのほかの文は 3.6 の規則で直し、PR のレビューで読む（7.4） |
| H-03（表が見つからないときの失敗のメッセージ） | 列名を locator の定数に置き、メッセージをその定数から組み立てる（1.2・3.3） |

## 付録 B: 受け入れ基準との対応

| AC | 設計 | 確かめ方 |
|---|---|---|
| AC-01 | 3.1 | 一回の確認・読む（7.4） |
| AC-02 | 3.1 | 読む |
| AC-03 | 3.1 | 読む |
| AC-04 | 3.1 | 一回の確認 |
| AC-05 | 3.6・6.1 | 読む |
| AC-06 | 3.7・6.1・6.2 | 一回の確認 |
| AC-07 | 1.2・3.5 | `TestADRRecords` |
| AC-08 | 3.2・3.3 | `checkProfileTable`（7.1） |
| AC-09 | 1.1 | 一回の確認（経緯）・読む（中身） |
| AC-10 | 3.2 | 読む |
| AC-11 | 3.4 | 読む |
| AC-12 | 3.4 | 読む |
| AC-13 | 3.4 | 一回の確認 |
| AC-14 | 2.2 | 一回の確認 |
| AC-15 | 3.7・5 章 | 一回の確認 |
| AC-16 | 3.3 | 一回の確認 |
| AC-17 | 1.1 | 読む |

## 変更履歴

- 2026-11-08 承認（isseis、#709）
- 2026-11-08 セッション中の再承認：ADR の「経緯」の「段階」を行を増減せず語だけ置き換え、一回の確認の経緯の確かめ方を合わせる（isseis、#718）
