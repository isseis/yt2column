# 要件定義書：用語「段階」（stage）を「ステージ」に改め、「構成要素」「工程」と使い分ける

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-06 |
| Review date | 2026-11-08 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、パイプラインの処理の単位を指す訳語「段階」（stage）を「ステージ」に改め、「構成要素」（component）と「工程」（step）との使い分けを用語集に定め、それに合わせて文書の文言を直すための要件を定義する。

対応 issue: [#704](https://github.com/isseis/yt2column/issues/704)

関連: タスク 0065 の設計書（[#703](https://github.com/isseis/yt2column/pull/703)）は、設計書の項目 A1 の主題（`LLMClient` を含む）を「構成要素」、ADR-0002 の生成・字数の調整・検証・推敲を「工程」と書いた。0065 の範囲は正本の文書と既存の ADR の文言を変えないので、それ以外の文書の置き換えをこのタスクで扱う。

本書は、用語集の定義、直す範囲、2026-11-06 の時点で直す箇所とその訳語（付録 A）、置き換えずに残す箇所（付録 B）、変えない文書を定める。言い換えた文の細かな言い回しは、`02_architecture.md` 以降の作業に委ねる。ただし、各箇所が指すもの（ステージ・構成要素・工程）は本書で決める。

**背景:**
用語集 `docs/translation_glossary.md:49` は、パイプラインが順に呼ぶ処理の単位（`TranscriptSource`・`ArticleWriter`・`Publisher`、`pipeline.Stage` の `transcript`・`write`・`publish`）の訳語を「段階」と定めている。しかし、日本語では「段階」をこの意味（パイプラインが順に呼ぶ処理の単位）では使わないので、英語の stage の直訳に読める。また、文書の中の「段階」は、ステージ、`ArticleWriter` の中の工程（生成・字数の調整・検証・推敲）、パッケージとしての部品（構成要素）のどれも指しており、どれを指すかが語からは分からない（例: `docs/dev/production_profile.md:9` は「段階」がステージと工程の両方を指すと断っている）。英語でも、`CLAUDE.md:363` の「Every pipeline stage (`TranscriptSource`, `LLMClient`, `Publisher`)」は `LLMClient` をステージに含め、`ArticleWriter` を含めていない。

### issue #704 で決めたこと

-   **stage の訳語を「ステージ」（パイプラインのステージ）に改める。** 理由: 「段階」はこの意味の日本語として使われず、直訳に読めるため。ADR の判断: ADR は作らない。訳語の決定で、`CLAUDE.md` の「Design decisions (ADR)」の基準では主要な決定ではない（「a wording」に当たる）。訳語の正本は用語集である。
-   **ステージ・構成要素・工程を次の表のとおり使い分ける。** 理由: 指すものが違う 3 つの概念を、1 つの語「段階」で呼んでいたため。ADR の判断: ADR は作らない。使い分けは後の文書にも及ぶが、指すもの（コードの構造）を変えない訳語の規則であり、その正本は用語集である。

    | 用語 | 英語 | 指すもの | 使い分けの手がかり |
    |---|---|---|---|
    | ステージ | stage | パイプラインが順に呼ぶ 3 つ：`TranscriptSource`・`ArticleWriter`・`Publisher`（`pipeline.Stage` の `transcript`・`write`・`publish`） | パイプラインの流れ（順序、どこで失敗したか、`StageError`）を述べるとき |
    | 構成要素 | component | interface とデータ型を持つパイプラインの部品すべて：上の 3 つと、`ArticleWriter` の中で使う `LLMClient` | 部品の分け方・置き場所（パッケージ、interface、fake）を述べるとき。`LLMClient` を含むかどうかが区別の決め手 |
    | 工程 | step | `ArticleWriter` の中の生成・字数の調整・検証・推敲（`writer.Stage`、API の `steps`） | 記事の生成の中の処理を述べるとき（既存の項のまま） |

-   **置き換えでは、各箇所が上の表のどれを指すかで訳語を選ぶ。** すべてを「ステージ」にはしない。理由: 今の「段階」の多くは工程や構成要素を指しているため。
-   **タスク文書（`docs/tasks/`）は直さない。** 理由: 書かれた時点の変わらない記録であるため。ADR の判断: このタスクの範囲の決定で、主要な決定ではない。
-   **一般の語としての「構成要素」（`README.md:240` の「構成要素の組み立て」、図の凡例の「変える構成要素」など）は置き換えない。** 理由（要件定義の際に補った）: パイプラインの部品を指さず、日本語として自然な用法であるため。ADR の判断: 文言の決定で、主要な決定ではない。
-   **`CLAUDE.md` の「Interface-based Design」の列挙を、stage と component の使い分けに合わせて直す。** 理由: 今の列挙は `LLMClient` をステージに含め、`ArticleWriter` を含めておらず、どちらの定義にも合わないため。ADR の判断: 文言の決定で、主要な決定ではない。
-   **ADR の題名を変えるときは、索引の題名も同じに直す。** 理由: `TestADRRecords`（`cmd/yt2column/adr_test.go:434`）が、ADR の 1 行目の題名と索引の題名の一致を確かめるため（同 `:98`）。ADR の判断: 既存の規約に従うだけで、新しい決定ではない。

### 要件定義の際に決めたこと（2026-11-06）

-   **軽量プロセスの条件に当たらないので、通常のプロセスで進める。** issue は、軽量プロセス（wording fix）の条件に当たるかを `/mkreq` で確かめることを求めた。`docs/dev/conventions/requirements_process.md` の「Lightweight Process」に照らすと、次の点で条件に当たらない。`CLAUDE.md` の変更が誤字の修正ではない（「Changes that always take the normal process」）。文書のテストの literal を変える（同）。除外のパス `docs/dev/security.md` と `prompts/` を変える（「Mechanically checked conditions」）。「段階」を含む行だけで 30 行を超えるので、類型 W（文言の修正）の上限（追加・削除それぞれ 30 行）に収まらない。ADR の判断: 手続きの選択で、主要な決定ではない。
-   **直す範囲は、ファイルの一覧ではなく内容で定める。** 範囲は、実装時点の `main` の `docs/dev/` の下（ADR とその索引を含む）、`prompts/README.md`、用語集にある「段階」のうち、ステージ・構成要素・工程を指すものすべてである。付録 A は、2026-11-06 の時点でこれに当たる箇所の一覧である。理由: タスク 0065 の設計書（`docs/tasks/0065_adr_remaining/02_architecture.md` の付録の表の 0019・0024 の行と、PR の分け方の表の ②）は、ADR-0001 の 36〜37 行目と ADR-0004 の 22 行目を、新しい ADR-0019・ADR-0024 へ文面のまま移す。0065 の PR ② とこのタスクのどちらが先にマージされても「段階」が残らないよう、範囲をファイルの一覧ではなく内容で定める。ADR の判断: このタスクの範囲の決定で、主要な決定ではない。
-   **`docs/dev/security.md` にある Native Messaging ホストのログの「段階」は置き換えない（付録 B）。** 「固定のイベント名・段階・…」と「メッセージの読み込みと形の検査の段階（`read`・`form`）」は、ホストのログの属性 `stage`（`internal/nmhost/relay.go:166`）を指す。理由: パイプラインのステージ・構成要素・工程のどれでもなく、ホストの中の処理の区切りを指す一般の語であるため。ADR の判断: 文言の決定で、主要な決定ではない。
-   **issue が挙げていない `docs/dev/production_profile.md`・`docs/dev/security.md`・`prompts/README.md` の「段階」も、このタスクで直す。** 理由: 用語集だけを改めると、これらの文書が新しい定義と食い違ったまま残るため（例: `production_profile.md:9` は「段階」がステージと工程の両方を指すと定義している）。`production_profile.md:25` の表は工程だけを並べるので列名を「工程」とし、この列名を探す文書のテスト（`cmd/yt2column/docs_test.go:574`）の literal も合わせて変える。`production_profile.md:11` の表はステージ（字幕の取得）と工程（生成など）とパイプラインの外の処理（投稿）を混ぜて並べるので、列名を「処理」とし、9 行目の定義の文を、表の行がそれぞれどれに当たるかを述べる文に書き換える。ADR の判断: ADR は作らない。訳語と文言の決定で、`CLAUDE.md` の「Design decisions (ADR)」の基準では主要な決定ではない（「a wording」に当たる）ため。
-   **一般の語としての「段階」は置き換えない（付録 B）。** `README.md:48` の「開発の初期段階」、ADR-0004 の「観点と段階の基準」（評価の尺度）、`docs/dev/guides/cli.md:169` の料金の区分、`docs/dev/guides/mergepr_guide.md` の手順の区切りが当たる。この結果、`README.md` は変わらず、`README.en.md` への同期（`/mktrans`）も要らない。issue は「`README.md`（1 か所。`README.en.md` への同期が要る）」と書いたが、その 1 か所は一般の語であった。理由: これらはパイプラインのステージ・構成要素・工程のどれも指さず、日本語として自然な用法であるため。ADR の判断: ADR は作らない（文言の決定で、主要な決定ではない）。
-   **`CLAUDE.md` は 363 行目に加えて、9 行目と 650 行目の stage も直す。** 9 行目（production profile の「each stage」）と 650 行目（「stage table」）が指すのは生成・字数の調整・検証・推敲で、新しい定義では step に当たる。`docs/dev/conventions/package_reference.md` の stage は、コードの識別子 `writer.Stage`・`pricing.StageCost` に沿った書き方なので直さない。理由: `CLAUDE.md` の 9 行目と 650 行目は従うべき識別子を持たない地の文で、同じ使い分けに揃えないと、エージェントが読む指示で stage が step の意味で残るため。`package_reference.md` は、パッケージの中の識別子（`writer.Stage` など）を説明する文書で、その語に従う。ADR の判断: ADR は作らない（文言の決定で、主要な決定ではない）。
-   **`production_profile.md` の表の「投稿」の行は、ステージではなく、パイプラインの外の処理とする（設計の際に改めた。2026-11-06）。** その行が実行する `SlackWebhookPublisher` は `Publisher` ではなく（`internal/publisher/publisher.go:1-5`）、サーバの投稿はパイプラインの実行とは別のジョブで行う。承認した版の AC-10 は投稿をステージと述べることを求めており、新しい定義と食い違っていた。ADR の判断: ADR は作らない（文言の決定で、主要な決定ではない）。
-   **ADR-0001〜0004 の文言を直しても、経緯に項目を足さない。** 理由: 規約（`docs/dev/conventions/adr.md` の「History entries」）は、決定を下したとき、または変えたときに、経緯の項目を 1 つ足すと定めており、このタスクは決定を変えないため。ADR の判断: 既存の規約に従うだけで、新しい決定ではない。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   用語集が、stage の訳語を「ステージ」と定め、ステージ・構成要素・工程の使い分けを示す。
-   正本の文書・ADR・指示の文書の中で、パイプラインのステージ・構成要素・工程を指す語が、用語集の使い分けに従う。

### 2.2. スコープ (In Scope)

-   用語集 `docs/translation_glossary.md` の「段階」「構成要素パッケージ」「工程」「フェーズ」の項。
-   実装時点の `main` で、`docs/dev/` の下（ADR とその索引を含む）と `prompts/README.md` にある「段階」のうち、ステージ・構成要素・工程を指すものの置き換え（以下、対象の文書）。2026-11-06 時点の対象の文書は、`docs/dev/project_overview.md`、ADR-0001・ADR-0002・ADR-0003・ADR-0004、ADR の索引 `docs/dev/adr/README.md`、`docs/dev/production_profile.md`、`docs/dev/security.md`、`prompts/README.md` で、箇所は付録 A のとおりである。
-   `production_profile.md` の表の列名の変更に合わせた、文書のテスト（`cmd/yt2column/docs_test.go`）の literal とメッセージの変更。
-   `CLAUDE.md` の 9 行目・363 行目・650 行目の英語の文言。

### 2.3. スコープ外 (Out of Scope)

-   タスク文書（`docs/tasks/` の下）。理由: 変わらない記録であるため（issue #704）。
-   一般の語としての「段階」（付録 B）と「構成要素」。理由: パイプラインの用語ではないため（1 章「要件定義の際に決めたこと」）。
-   `README.md` と `README.en.md`。理由: `README.md` の「段階」は一般の語だけであるため。
-   `docs/dev/conventions/package_reference.md` と、コードの識別子（`writer.Stage`・`pricing.StageCost`・`pipeline.Stage` など）・コードのコメント。理由: これらはコードの識別子に沿った書き方であり、識別子の名前を変えることは訳語の修正の範囲を超えるため。文書のテストの literal は例外で、2.2 のとおり変える。
-   ステージ・構成要素・工程の指すもの、コードの振る舞い、ADR の決定の中身を変えること。

## 3. 機能要件 (Functional Requirements)

本タスクの「振る舞い」は、文書の文言と、文書を読むテストの結果である。

### 3.1. 機能一覧

#### F-001: 用語集の定義

用語集 `docs/translation_glossary.md` に、stage の訳語を「ステージ」と定め、ステージ・構成要素・工程の使い分けを書く。今の定義は `docs/translation_glossary.md:42`（工程）、`:49`（段階）、`:56`（構成要素パッケージ）、`:116`（フェーズ）にある。

**Acceptance Criteria**:
- **AC-01**: 用語集に、日本語の列が「ステージ」、英語の列が `stage` の項が 1 つある。その説明は、指すもの（`TranscriptSource`・`ArticleWriter`・`Publisher`、`pipeline.Stage` の `transcript`・`write`・`publish`）を述べる。備考には「旧称: 段階」と書く（用語集の冒頭の規則 4、`docs/translation_glossary.md:10`）。日本語の列が「段階」の項はない。
- **AC-02**: 「構成要素パッケージ」の項の説明は、構成要素が 3 つのステージと `LLMClient` を指すことと、1 章の表の 3 語（ステージ・構成要素・工程）の使い分けを述べ、`LLMClient` を含むかどうかが構成要素とステージの区別の決め手であることを述べる。また、一般の語としての「構成要素」はこの定義の外であることを述べる。
- **AC-03**: 「工程」と「フェーズ」の項の説明は、区別する相手のパイプラインの単位を「ステージ」（stage）と書き、「段階」と書かない。
- **AC-04**: 用語集の中で「段階」という語が現れるのは、AC-01 の旧称の記述だけである。

#### F-002: 文書の訳語の置き換え

対象の文書（2.2）の「段階」のうち、ステージ・構成要素・工程を指すものを、各箇所が指すものに従って「ステージ」「構成要素」「工程」に置き換える。2026-11-06 時点の箇所と割り当てる語は付録 A のとおりで、実装時点の `main` で新たに見つかった箇所にも同じ規則を当てはめる。文の意味（何が何をするか、決定の中身、規則）は変えない。

**Acceptance Criteria**:
- **AC-05**: 付録 A の各箇所は、付録 A がその箇所に割り当てた語（ステージ・構成要素・工程。表の列名は「工程」「処理」。ADR-0002 の 17 行目は一般の語「処理」を使う）で、その箇所が指すものを呼ぶ。
- **AC-06**: 変更後、対象の文書（用語集を除く）に残る「段階」は、付録 B に挙げる文言のものだけである。この確認は、PR の基点の `main` を取り込んだ後の状態で行う。
- **AC-07**: ADR-0002 と ADR-0004 の 1 行目の題名は「段階」を含まず、索引 `docs/dev/adr/README.md` の同じ ADR の題名と一致する。`TestADRRecords` が成功する。
- **AC-08**: `docs/dev/production_profile.md` の本番の構成の表（今の 25 行目）の 1 列目の名前は「工程」であり、その表を読む文書のテスト（`go test -tags test ./cmd/yt2column` の中の、本番の構成の表を確かめるテスト）が成功する。
- **AC-09**: 変更した ADR の決定・背景・結果・採らなかった案の中身は、変更の前と同じことを述べる。各 ADR の「経緯」の節には、行が加わらず、行が消えない。
- **AC-10**: `docs/dev/production_profile.md` のパイプラインの表（今の 11 行目）の 1 列目の名前は「処理」であり、表の前の文は、表の行が、ステージ `TranscriptSource` に当たる字幕の取得、ステージ `ArticleWriter` の中の工程（生成・字数の調整・検証・推敲）、パイプラインの外の処理である投稿であることを、「段階」を使わずに述べる。

#### F-003: `CLAUDE.md` の英語の文言

`CLAUDE.md` の英語の文言を、stage・component・step の使い分けに揃える。今の文言は `CLAUDE.md:9`（「the template, provider, model, and effort of each stage」）、`CLAUDE.md:363-364`（「Every pipeline stage (`TranscriptSource`, `LLMClient`, `Publisher`) is an interface with a fake for tests.」）、`CLAUDE.md:650`（「the production profile's configuration block and stage table」）である。

**Acceptance Criteria**:
- **AC-11**: 「Interface-based Design」の項は、interface と fake を持つものを pipeline component と呼び、`TranscriptSource`・`ArticleWriter`・`LLMClient`・`Publisher` の 4 つを挙げる。
- **AC-12**: Quick Links の Production Profile の説明（今の 9 行目）と、Development Notes の text-sync guards の列挙（今の 650 行目）は、生成・字数の調整・検証・推敲を step と呼び、stage と呼ばない。
- **AC-13**: `CLAUDE.md` の stage のうち、AC-11・AC-12 の箇所以外は変わらない。

#### F-004: 変えないもの

**Acceptance Criteria**:
- **AC-14**: `docs/tasks/` の下のファイル（このタスクのディレクトリ `docs/tasks/0067_stage_terminology/` を除く）、`README.md`、`README.en.md`、`docs/dev/conventions/package_reference.md` は、このタスクの変更で変わらない。
- **AC-15**: 付録 B に挙げる文言の「段階」は、変更の後も残る。
- **AC-16**: Go のコードの変更は `cmd/yt2column/docs_test.go` の中だけである。ほかの Go のファイルと拡張（`extension/`）のファイルは変わらない。
- **AC-17**: ADR を除く対象の文書（`docs/dev/security.md` を含む）で変えた文は、語のほかは、変更の前と同じ事実・規則を述べる。用語集の定義の変更（F-001）と、`docs/dev/production_profile.md` の定義の文の書き換え（AC-10）は除く。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A（文書の文言の変更であり、実行時の処理は変わらない）。

### 4.2. セキュリティ (Security)
-   N/A。このタスクは条件付きのガイド `docs/dev/security.md` の適用の条件（外部コマンドの起動、API キー・Webhook URL の扱い、キャッシュの読み書き、ネットワークの送信、信頼できない文字列の埋め込み）のどれにも当たらない。`docs/dev/security.md` の文言は直すが（付録 A）、述べる規則の中身は変えない（AC-17）。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A。

### 4.4. 互換性 (Compatibility)
-   用語集の「段階」は旧称として残すので、タスク文書など、直さない文書の「段階」を読む人は、用語集から新しい語にたどれる（AC-01）。

### 4.5. 保守性 (Maintainability)
-   `make test` と `make lint` が成功する。CI の `doc-guard` ジョブが走らせる文書のテストが成功する。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   文書の言語は `CLAUDE.md` の「Document language」に従う（日本語の文書は日本語、`CLAUDE.md` は英語）。
-   ADR の文言の変更は、規約 `docs/dev/conventions/adr.md` に従う。題名を変えたら索引の題名も同じに直す。
-   文書のテストの literal は、`CLAUDE.md` の「Go Idioms」の guard-test literal exception の範囲で日本語にする。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。本タスクの変更の後の定義は 1 章の表のとおりである。
-   **ステージ（stage）:** パイプラインが順に呼ぶ `TranscriptSource`・`ArticleWriter`・`Publisher`。旧称は「段階」。
-   **構成要素（component）:** 3 つのステージと `LLMClient`。
-   **工程（step）:** `ArticleWriter` の中の生成・字数の調整・検証・推敲。

## 付録 A: 置き換える箇所と、割り当てる語

行番号は 2026-11-06 の `main`（`00682d91`）のものである。実装時点の `main` では、行が動いたり、ほかの文書（タスク 0065 が足す ADR など）へ移ったりしうる（1 章「要件定義の際に決めたこと」）。

| 文書 | 行 | 今の文言（抜粋） | 指すもの | 割り当てる語 |
|---|---|---|---|---|
| `docs/translation_glossary.md` | 42 | パイプラインの「段階」（stage）とは区別する | ステージ | ステージ |
| `docs/translation_glossary.md` | 49 | 段階 \| stage \| パイプラインを構成する処理の単位 | ステージ | 日本語の列を「ステージ」とし、備考に「旧称: 段階」 |
| `docs/translation_glossary.md` | 56 | パイプラインの構成要素（3 つの段階と、…） | ステージ | ステージ |
| `docs/translation_glossary.md` | 116 | パイプラインの「段階」（stage）とは区別する | ステージ | ステージ |
| `docs/dev/project_overview.md` | 41 | 段階の間で受け渡すデータ（図の凡例） | ステージ | ステージ |
| `docs/dev/project_overview.md` | 42 | 段階（interface）（図の凡例） | ステージ | ステージ |
| `docs/dev/project_overview.md` | 48 | 各段階は interface で抽象化し | 構成要素（直後の列挙は `LLMClient` を含む） | 構成要素 |
| `docs/dev/project_overview.md` | 55 | 段階ごとのモデル名 | 工程 | 工程 |
| `docs/dev/project_overview.md` | 57 | 2 つの段階を既定で行う、いずれかの段階で失敗した | 工程 | 工程 |
| `docs/dev/project_overview.md` | 58 | 各段階の出力、その段階から再開、後の段階、段階が完了する、前の段階、実行した段階、各段階の呼び出し | 工程 | 工程 |
| `docs/dev/project_overview.md` | 59 | 段階ごとに記録、段階ごとに 1 行、その段階の使用量 | 工程 | 工程 |
| `docs/dev/project_overview.md` | 60 | 段階ごとの料金の行、`cost: <段階>`、各段階のモデルと版 | 工程 | 工程 |
| `docs/dev/project_overview.md` | 104 | 3段階を束ねるオーケストレーション | ステージ | ステージ |
| `docs/dev/project_overview.md` | 148 | その段階のクライアント | 工程 | 工程 |
| `docs/dev/project_overview.md` | 161 | 実行する段階の LLM の変数、実行する段階によらず | 工程 | 工程 |
| `docs/dev/adr/0001-llm-adapter-stdlib.md` | 37 | LLM の段階から使う、LLM の段階が字幕取得の段階に依存する | 構成要素（パッケージの依存） | 構成要素 |
| `docs/dev/adr/0002-post-generation-steps.md` | 1 | 2 段階を置き（題名） | 工程 | 工程 |
| `docs/dev/adr/0002-post-generation-steps.md` | 17 | 段階の順序は、生成、字数の調整、検証・推敲、最終の字数の確認、出典の付与とする | 工程と、工程でない後処理（最終の字数の確認、出典の付与） | 処理（「処理の順序」） |
| `docs/dev/adr/0002-post-generation-steps.md` | 13, 19, 24, 33, 42, 43 | 縮める段階、直す段階、段階を省ける、段階の効果、いずれかの段階、前の段階、各段階、この段階、2 つの段階 | 工程 | 工程 |
| `docs/dev/adr/0003-llm-config-contract.md` | 11 | 段階ごとに別の環境変数 | 工程 | 工程 |
| `docs/dev/adr/0004-production-profile.md` | 1 | 3 段階とし（題名） | 工程 | 工程 |
| `docs/dev/adr/0004-production-profile.md` | 17, 22, 34 | 3 つの段階、3 つの段階で共有、発言を整理する段階 | 工程 | 工程 |
| `docs/dev/adr/README.md` | 10, 12 | ADR-0002・ADR-0004 の題名 | 工程 | 工程（ADR の題名と同じ文言） |
| `docs/dev/production_profile.md` | 3 | パイプラインの各段階が使うテンプレート | 工程 | 工程 |
| `docs/dev/production_profile.md` | 9 | 記事ができるまでの段階、各段階の役割、その段階、「段階」は…両方を指す | ステージ・工程・パイプラインの外の処理 | 表の行がそれぞれどれに当たるかを述べる文（AC-10） |
| `docs/dev/production_profile.md` | 11 | 表の列名「段階」 | ステージ・工程・パイプラインの外の処理の混在 | 処理 |
| `docs/dev/production_profile.md` | 19 | 最初の段階 | ステージ | ステージ |
| `docs/dev/production_profile.md` | 23 | LLM を呼ぶ 3 つの段階 | 工程 | 工程 |
| `docs/dev/production_profile.md` | 25 | 表の列名「段階」 | 工程 | 工程 |
| `docs/dev/production_profile.md` | 38 | 段階ごとに指定できる、3 つの段階のどれも | 工程 | 工程 |
| `docs/dev/production_profile.md` | 81 | DeepSeek の段階の推定コスト | 工程 | 工程 |
| `docs/dev/security.md` | 58 | 字数の調整の段階、検証・推敲の段階 | 工程 | 工程 |
| `docs/dev/security.md` | 59 | いずれの段階でも | 工程 | 工程 |
| `docs/dev/security.md` | 66 | 固定の段階名 | 工程 | 工程 |
| `docs/dev/security.md` | 80 | 段階の出力、来歴（段階・モデル名…）、再利用した段階のトークン数 | 工程 | 工程 |
| `prompts/README.md` | 3 | 生成した後の 2 つの段階 | 工程 | 工程 |

## 付録 B: 置き換えずに残す箇所

一般の語としての「段階」で、パイプラインのステージ・構成要素・工程のどれも指さない。残す箇所は文言で特定する。行番号は 2026-11-06 の `main` のもので、参考である。

| 文書 | 行 | 今の文言（抜粋） | 意味 |
|---|---|---|---|
| `README.md` | 48 | 開発の初期段階 | 開発の進み具合 |
| `docs/dev/adr/0004-production-profile.md` | 19 | 観点と段階の基準 | 評価の尺度の段階 |
| `docs/dev/security.md` | 145 | 固定のイベント名・段階・ホストのエラー、メッセージの読み込みと形の検査の段階（`read`・`form`） | Native Messaging のホストのログの属性 `stage`（`internal/nmhost/relay.go:166`） |
| `docs/dev/guides/cli.md` | 169 | プロンプトの長さによる 2 つの段階 | 料金の区分 |
| `docs/dev/guides/mergepr_guide.md` | 72, 79, 215, 231 | 段階（表の列名）、承認の段階、取り消せない段階 | `/mergepr` の手順の区切り |

`README.md` は対象の文書ではない。`docs/dev/guides/cli.md` と `docs/dev/guides/mergepr_guide.md` は `docs/dev/` の下にあるので対象の文書に入るが、「段階」は上の一般の語だけである。

## 変更履歴

- 2026-11-06 承認（isseis、#707）
- 2026-11-06 draft への戻し：AC-10 の投稿の行を、ステージではなくパイプラインの外の処理とした（#709）
- 2026-11-08 承認（isseis、#709）
