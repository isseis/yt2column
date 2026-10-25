# 要件定義書：本番の構成と品質を共通の文書にまとめ、文書の主経路をブラウザ拡張に改める

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-25 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、次の 2 つの要件を定義する。1 つ目は、本番の構成（システム構成）・品質の目標・製品の制約・構成の比較を、プロジェクトに共通の文書にまとめること。2 つ目は、README・`docs/dev/project_overview.md`・`CLAUDE.md` の主経路の書き方を、ブラウザ拡張の経路を主とするように改めること（[#488](https://github.com/isseis/yt2column/issues/488)）。

**背景:**

タスク 0008（コラム生成プロンプト）の最終の版の評価で、次のことが決まった（[0008 評価記録](../0008_column_prompt/02_evaluation.md) §6）。

-   **本番の構成:** 生成・字数の調整・検証・推敲の 3 つの段階を、すべて claude-opus-5-5（effort `high`）で行う。生成のテンプレートは v5 で、`prompts/` の既定のテンプレートになっている（0008 評価記録 §6.1・§6.5）。
-   **品質の目標:** 記事が必須の観点をすべて満たし、品質の観点に 1 がないこと。目標は、既定にするための関門ではなく、記事が目指す水準である（[0008 要件定義書](../0008_column_prompt/01_requirements.md) F-006）。最終の版では、10 件中 8 件（文言どおりの判定では 7 件）が目標を満たした（0008 評価記録 §6.2）。
-   **製品の制約:** 目標に届かなかった点と、この構成で起こりうる誤りや欠けの 8 項目である（0008 評価記録 §6.3）。

これらはタスクに固有の情報ではなく、プロジェクトに共通の情報である。それでも今は、タスクの評価記録の中にしかない。0008 評価記録 §6.5 は、タスクを終えた後に `docs/` の下の共通の文書へまとめる予定だと記録している。

あわせて、文書全体での主経路の位置づけを直す。現在の README は、冒頭で本ツールを「YouTube 動画の URL を受け取る CLI」と説明し、字幕は yt-dlp で取得すると書いている（`README.md:3-15`）。Requirements（`README.md:19-27`）と Usage（`README.md:29`）も CLI を前提にしている。`docs/dev/project_overview.md` も、冒頭で本ツールを CLI ツールと説明している（`docs/dev/project_overview.md:3`）。決定済みの方針の「実行形式」は CLI を先に書き（同 `:10`）、「字幕取得」は yt-dlp を主として、コピーした字幕のパーサを例外として書いている（同 `:11`）。パイプラインの図は `URL →` から始まる（同 `:21`）。`CLAUDE.md` の「Architecture Overview」の冒頭の一文も、本ツールを「A Go CLI」と書いている。

実際の主経路は、ブラウザ拡張が YouTube の文字起こしパネルからコピーした字幕を集め、Native Messaging のホストと localhost サーバを経て記事を生成し、Webhook へ投稿する経路である（[#43](https://github.com/isseis/yt2column/issues/43)。README「Localhost server」`README.md:448`、「Browser extension」`README.md:818`）。CLI は、LLM の呼び出しや投稿の処理を開発・検証するための補助の経路である。

対応 issue: [#488](https://github.com/isseis/yt2column/issues/488)（関連: [#43](https://github.com/isseis/yt2column/issues/43)・[#171](https://github.com/isseis/yt2column/issues/171)・[#146](https://github.com/isseis/yt2column/issues/146)）

**本書の記述範囲:** 本書は、共通の文書の置き場所と、載せる内容を定める。また、README・`project_overview.md`・`CLAUDE.md`・`prompts/README.md`・0008 評価記録の各文書で、何を書き直し、何を変えないかを定める。共通の文書の節の構成と順序、README に加える Quick start の節の見出しの文言と手順の粒度、文書の値を検査する手段（テスト）は、設計（`02_architecture.md`）で決める。要件の作成で挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

本タスクは文書だけを変える。コード・設定の既定値・テンプレートは変えない（2.3）。

**issue #488 で決めたこと:**

-   **共通の文書に載せる内容:** システム構成（パイプラインと各段階の役割・プロバイダ・モデル・effort、本番の構成を表す環境変数）、品質の目標と判定の物差し（0008 F-006 への参照）、製品の制約の 8 項目（頻度が 10 件からの値であることを含む）、構成の比較の要点と構成を決めた理由。issue がこれらを、タスクではなくプロジェクトに共通の情報としたためである。
-   **主経路:** README（英語）と `project_overview.md` の冒頭の説明・使い方・要件の書き方を、ブラウザ拡張の経路を主、CLI を開発用の補助として書き直す。README の推奨構成を本番の構成に合わせる。実際の使い方と文書の書き方を一致させるためである。
-   **参照の張り替え:** `prompts/README.md`・0008 評価記録 §6 などから共通の文書を参照し、`CLAUDE.md` の Quick Links にも加える。
-   **既定のプロバイダは変えない:** コード上の既定のプロバイダは DeepSeek のままとする（`README.md:718`、`docs/dev/project_overview.md:109`）。既定を変えるかは本 issue のスコープ外とし、必要なら別の issue にする。
-   **言語:** `CLAUDE.md` の文書の言語の規則に従う（`docs/` は日本語、README は英語）。
-   **README を読むテストを壊さない:** README の値を読むテスト（拡張の `serverReasons.test.ts`・`metadataDocs.test.ts`、`TestReadmeHostExitCodes` など。`CLAUDE.md`「Development Notes」）があるので、README の構成を変えるときはそれらを壊さない。

**要件定義の際に決めたこと（2026-10-25）:**

-   **共通の文書は新しい 1 つの文書にする:** `docs/dev/production_profile.md`（題は「本番の構成と品質」）を新しく作り、4 つの内容をまとめる（F-001）。4 つの内容は「本番の構成と、その構成で得られた結果」として一続きだからである。`project_overview.md` に節として加える案は採らなかった。決定済みの方針の文書に評価の数値が混ざるからである。2 つの文書に分ける案も採らなかった。
-   **共通の文書の更新の規則を明記する:** 本番の構成・品質の目標・製品の制約を変えるタスクは、共通の文書も更新する。このことを共通の文書の冒頭に書く（AC-06）。`project_overview.md` の冒頭の規則（`docs/dev/project_overview.md:5`）と同じ形である。タスクの評価記録はその時点の記録として残るので、共通の文書が最新の値を持ち続けるには、更新する者を決めておく必要がある。
-   **関連 issue の一覧は共通の文書に書かない:** #171（claude の max_tokens の前提）・#146（話者の区別が難しい動画）は、共通の文書に「未解決の関連 issue」として挙げない（2.3）。レビュアーの判断による。
-   **0008 評価記録 §6 は残し、参照を足す:** 0008 評価記録 §6 の中身は、2026-10-25 時点の記録として残す。共通の文書へ移すことはしない。§6 には共通の文書への参照を足す（F-005）。共通の文書は構成が変わるたびに更新する文書であり、評価記録はその時点の判定の根拠だからである。参照を足すのは編集上の修正なので、0008 評価記録の status は `approved` のまま保ち、`Comments` に記録する（[requirements_process.md](../../dev/developer_guide/requirements_process.md)「Editing an approved document」）。
-   **README は冒頭と Quick start を書き直し、既存の節の順序と見出しを保つ:** README の冒頭と Requirements を書き直す。拡張の経路の手順をたどる Quick start の節を冒頭の近くに加え、既存の節へリンクする。CLI の Usage は開発・検証用の CLI として位置づけ直す（F-002）。既存の節は並べ替えず、見出しも変えない。README の見出しを頼りに節を読むテスト（`cmd/yt2column-server/docs_test.go:39-105`、`extension/test/metadataDocs.test.ts:25`）や、ほかの文書からのアンカーを壊す危険を小さくし、差分を読みやすくするためである。
-   **README の推奨構成は小節として加える:** README の「Configuration」に推奨構成の小節を加え、本番の構成を表す環境変数と、共通の文書へのリンクを書く（F-003）。README は英語なので、日本語の共通の文書を読まない利用者にも値が伝わるようにするためである。現在の README には推奨構成の節がない。「Configuration」の表（`README.md:714-726`）が例としてモデル名を挙げているだけである。
-   **`project_overview.md` は冒頭・方針・パイプラインの説明を書き直す:** 冒頭の説明、決定済みの方針の「実行形式」「字幕取得」、パイプラインの説明を、ブラウザ拡張の経路を主、CLI と yt-dlp を補助として書き直す。方針の中身は変えない。変えないのは、字幕の取得を yt-dlp に委譲すること、コピーした字幕のパーサの例外とその範囲、`yt-dlp` の経路を代わりの手段として残すことである。「LLM」の項には、既定は DeepSeek のままであることと、本番の構成は共通の文書にあることを書き足す（F-004）。冒頭だけを直すと、方針の項が CLI を主とする書き方のまま残り、文書の中で主経路の位置づけがぶつかるためである。
-   **`CLAUDE.md` は Quick Links と Architecture Overview の冒頭の一文を直す:** Quick Links に共通の文書を加え、「Architecture Overview」の冒頭の一文も主経路に合わせる（F-005）。文書全体の主経路を改めるという issue の目的に沿い、変更も 1〜2 文で済むからである。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   本番の構成・品質の目標・製品の制約・構成の比較を、タスクの評価記録を読まずに、1 つの共通の文書で把握できるようにする。
-   README・`project_overview.md`・`CLAUDE.md` を読んだ人が、ブラウザ拡張の経路が主な使い方であり、CLI は開発・検証用の補助であると読み取れるようにする。
-   README を読んだ利用者が、本番の構成を表す環境変数の値を README だけで知れるようにする。
-   既存の文書のテストと、コードの振る舞いを変えない。

### 2.2. スコープ (In Scope)

-   共通の文書 `docs/dev/production_profile.md` の新規作成（F-001）。
-   README の冒頭・Requirements・Usage の位置づけの書き直しと、Quick start の節の追加（F-002）。
-   README の「Configuration」への推奨構成の小節の追加（F-003）。
-   `docs/dev/project_overview.md` の冒頭・「実行形式」「字幕取得」「LLM」の項・パイプラインの説明の書き直し（F-004）。
-   `prompts/README.md`・0008 評価記録 §6・`CLAUDE.md` の参照の張り替えと、`CLAUDE.md` の Architecture Overview の冒頭の一文の書き直し（F-005）。

### 2.3. スコープ外 (Out of Scope)

-   コードの変更。既定のプロバイダ（DeepSeek）、`YT2COLUMN_MODEL` が必須であること、`YT2COLUMN_CLAUDE_EFFORT` の既定がないこと（`README.md:717-724`）などの設定の振る舞いは変えない。既定を本番の構成に変えるかは別の issue で扱う（issue #488）。
-   プロンプトテンプレートの変更。
-   README の effort の案内（「`low` から `xhigh` を使う」。`README.md:750-752`）と、claude の `max_tokens` の見直し。これらは #171 で決める。
-   #171・#146 を、共通の文書に未解決の関連 issue として挙げること（1 の「要件定義の際に決めたこと」）。
-   README の既存の節の並べ替えと、既存の見出しの変更（1 の「要件定義の際に決めたこと」）。
-   0008 評価記録 §6 の中身の移動・削除と、0018 評価記録の変更。
-   新しい評価（記事の生成・採点）。共通の文書の数値は、0008 評価記録・0018 評価記録に記録済みの値だけを使う。
-   英語の翻訳版の作成。`docs/` の文書は日本語だけとする（`CLAUDE.md`「Documents」）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 本番の構成と品質の共通の文書

`docs/dev/production_profile.md` を日本語で新しく作り、本番の構成・品質の目標・製品の制約・構成の比較をまとめる。数値と判断は、0008 評価記録 §6 と、そこから参照する 0018 評価記録の記録から取り、出典の節を示す。

**Acceptance Criteria**:
- **AC-01**: `docs/dev/production_profile.md` が存在し、その文書だけを読んで次のことが分かる。パイプラインの段階（コピーした字幕の取得 → 生成 → 字数の調整 → 検証・推敲 → 投稿）の順序と各段階の役割。LLM を呼ぶ 3 つの段階のそれぞれについて、本番の構成のテンプレート・プロバイダ・モデル・effort。
- **AC-02**: 本番の構成を表す環境変数として、`YT2COLUMN_LLM_PROVIDER=claude`・`YT2COLUMN_MODEL=claude-opus-5-5`・`YT2COLUMN_CLAUDE_EFFORT=high` が書かれている。また、検証・推敲のプロバイダとモデルは、`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL` を指定しなければ生成と同じになること、effort は 3 つの段階で共通であること、コード上の既定のプロバイダは DeepSeek のままなので本番の構成は環境変数で指定する必要があることが書かれている。
- **AC-03**: 品質の目標（記事が必須の観点をすべて満たし、品質の観点に 1 がないこと）が書かれている。また、目標が既定にするための関門ではなく記事が目指す水準であることが書かれ、判定の物差しとして 0008 要件定義書 F-006 にリンクしている。最終の版の結果（解釈による判定で 10 件中 8 件、文言どおりの判定で 10 件中 7 件）が、0008 評価記録 §6.2 への参照とともに書かれている。
- **AC-04**: 0008 評価記録 §6.3 の製品の制約の 8 項目が、すべて書かれている（意味の変わる言い換え、節の中身と見出しのずれ、発言の主体を広げたまとめ、話者の取り違え、字数の調整の失敗で記事が出ないこと、概要欄を開かずに送った字幕、費用、ほかの構成での品質の低下）。各項目の頻度や金額は 0008 評価記録 §6.3 の値と一致する。また、頻度が 5 本の動画で 2 回ずつ生成した 10 件からの、幅のある値であることが書かれている。
- **AC-05**: 構成の比較の要点が、0008 評価記録 §6.4 への参照とともに書かれている。要点は、3 つの構成（生成と検証・推敲のどちらも claude-opus-5-5、生成が deepseek-flash で検証・推敲が claude-opus-5-5、生成が deepseek-flash で検証・推敲なし）のそれぞれの、目標を満たした記事の件数（8 / 7、6 / 6、4 / 4。解釈 / 文言）と記事 1 本あたりの推定コストである。また、本番の構成を決めた理由（検証・推敲の効果と生成のモデルの効果）が書かれている。
- **AC-06**: 文書の冒頭に、本番の構成・品質の目標・製品の制約を変えるタスクは本書も更新するという規則が書かれている。また、本書の値がどのタスクのいつの評価から来たか（0008 の最終の版の評価、2026-10-25）が書かれている。
- **AC-07**: 文書は、#171・#146 を未解決の関連 issue として挙げる節や一覧を持たない。

#### F-002: README の主経路

README（英語）の冒頭・Requirements・Usage の書き方を、ブラウザ拡張の経路を主、CLI を開発・検証用の補助として改める。既存の節の順序と見出しは変えない。

**Acceptance Criteria**:
- **AC-08**: README の冒頭の説明が、主な使い方として、ブラウザ拡張が文字起こしパネルからコピーした字幕を集め、Native Messaging のホストと localhost サーバを経て記事を生成し、Webhook へ投稿する経路を述べている。また、CLI（動画の URL を渡し、yt-dlp で字幕を取得する）を、LLM の呼び出しや投稿の処理を開発・検証するための補助の経路として述べている。
- **AC-09**: README の Requirements の節より前に、拡張の経路で最初の記事を投稿するまでの手順を、順に既存の節へのリンクでたどれる Quick start の節がある。手順には、ビルド、サーバの起動、ホストの登録、拡張のビルドと読み込み、本番の構成の環境変数（F-003 の小節へのリンク）が含まれる。リンクはすべて README の中の既存の見出しに解決する。
- **AC-10**: README の Requirements が、拡張の経路に必要なものと、CLI だけに必要なもの（`yt-dlp`）を区別して書いている。
- **AC-11**: README の Usage の節（CLI の使い方）に、CLI が開発・検証用の補助の経路であることが書かれている。
- **AC-12**: README の変更の前にあった `#`・`##`・`###` の見出しが、変更の後もすべて同じ文言で、同じ順序で残っている。新しい見出し（Quick start と推奨構成の小節）を加えることは妨げない。

#### F-003: README の推奨構成

README の「Configuration」に、本番の構成を表す推奨構成の小節を加える。

**Acceptance Criteria**:
- **AC-13**: README の「Configuration」の節の中に、推奨構成の小節がある。その小節に `YT2COLUMN_LLM_PROVIDER=claude`・`YT2COLUMN_MODEL=claude-opus-5-5`・`YT2COLUMN_CLAUDE_EFFORT=high` と `ANTHROPIC_API_KEY` が必要であることが書かれ、検証・推敲の変数を指定しなければ生成と同じ構成になることが書かれている。
- **AC-14**: その小節に、コード上の既定のプロバイダは DeepSeek のままであり、推奨構成は環境変数で指定する必要があることが書かれている。また、共通の文書 `docs/dev/production_profile.md`（品質の目標と製品の制約）へのリンクがある。
- **AC-15**: その小節と README のほかの箇所は、API キーや Webhook URL の値の例として、実在しうる値を書かない（変数名だけを書く）。

#### F-004: `project_overview.md` の主経路

`docs/dev/project_overview.md` の冒頭・決定済みの方針・パイプラインの説明を、ブラウザ拡張の経路を主として書き直す。方針の中身は変えない。

**Acceptance Criteria**:
- **AC-16**: 冒頭の説明が、ブラウザ拡張から localhost サーバを経て記事を生成・投稿する経路を主な使い方として述べ、CLI を開発・検証用の補助の経路として述べている。
- **AC-17**: 決定済みの方針の「実行形式」と「字幕取得」が、主経路（拡張・ホスト・localhost サーバ、コピーした字幕）を先に、CLI と yt-dlp の経路を補助として述べている。次の方針の中身は、変更の前と同じである。字幕の取得を yt-dlp に委譲し、Go で字幕の取得を自前で実装しないこと。コピーした字幕のパーサの例外とその範囲（Go から YouTube のページやパネルを取得・解析しない）。書式が変わって拒否が続く間は yt-dlp の経路を代わりの手段として残すこと。サーバが yt-dlp を呼ばないこと。
- **AC-18**: パイプラインの説明が、主経路の入力（コピーした字幕）を示している。CLI の入力（URL と yt-dlp）は、補助の経路の入力として示されている。
- **AC-19**: 決定済みの方針の「LLM」の項に、コード上の既定のプロバイダは DeepSeek のままであることと、本番の構成は `docs/dev/production_profile.md` にあることが、リンクとともに書かれている。
- **AC-20**: 「設定（環境変数）」の表の変数名と、`yt2column-server`・`project_overview.md` を読む既存のテストが検査する値（`YT2COLUMN_CACHE_DIR` と `YT2COLUMN_YTDLP_PATH` を 1 行に書くこと、`costPrefix`、`16,383`。`CLAUDE.md`「Development Notes」）は、変更の後も残っている。

#### F-005: 参照の張り替え

共通の文書を、関連する文書から参照する。

**Acceptance Criteria**:
- **AC-21**: `prompts/README.md` の、評価と製品の制約が 0008 の評価記録 §6 にあるという記述が、共通の文書 `docs/dev/production_profile.md` への参照に改められている。最終の版の出典（`docs/tasks/0008_column_prompt/v5/`）への参照は残っている。
- **AC-22**: 0008 評価記録 §6（§6.3・§6.4・§6.5 を含む）の中身は、変更の前と同じである。§6 に共通の文書への参照が加えられ、§6.5 の「共通の文書としてまとめる予定」という記述に、まとめた先の文書へのリンクがある。0008 評価記録の status は `approved` のままで、`Comments` に、参照を足した日付と、決定の変更がないことが書かれている。
- **AC-23**: `CLAUDE.md` の Quick Links の「Project」に、共通の文書へのリンクと、その内容（本番の構成・品質の目標・製品の制約。日本語）の英語の説明がある。
- **AC-24**: `CLAUDE.md` の「Architecture Overview」の冒頭の一文が、ブラウザ拡張の経路を主、CLI を補助として述べている。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A（文書だけを変える）。

### 4.2. セキュリティ (Security)

-   N/A。本タスクは文書だけを変え、外部コマンドの起動・秘密情報の扱い・キャッシュ・ネットワーク・信頼できないテキストの埋め込みのいずれにも触れない（条件付きのガイド [security.md](../../dev/security.md) の適用の条件に当たらない）。文書に環境変数を書くときは変数名だけを書き、値の例として秘密情報を書かない（AC-15）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   `make test`（文書を読む Go のテストを含む）・`make ext-test`（README を読む拡張のテストを含む）・`make lint` が、変更の後も通ること。README を変えるので、`make ext-test` も必要である（`CLAUDE.md`「Development Notes」）。

### 4.4. 互換性 (Compatibility)

-   README の既存の見出しへのアンカー（ほかの文書や README 自身からのリンク）が、変更の後も解決すること（AC-12）。

### 4.5. 保守性 (Maintainability)

-   共通の文書の数値は、出典の評価記録の節を示し、評価記録の値と一致させる（AC-04・AC-05）。値を写すときに丸めや言い換えをしない。
-   本番の構成が変わったときに更新する文書が、共通の文書と README の推奨構成の小節の 2 か所であることが、共通の文書の更新の規則から分かること（AC-06）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。本タスクは方針の書き方を変えるが、方針の中身は変えない（F-004）。
-   文書の言語は `CLAUDE.md`「Documents」に従う（`docs/` は日本語、README と `CLAUDE.md` は英語）。
-   文書を読むテストは、値・構造・実行できる例を検査し、文の意味を検査しない（`CLAUDE.md`「Testing Strategy」の「Document tests check values, structure, and executable examples, never prose」）。文面だけを定める受け入れ基準は、PR のレビューでの読み取り（`manual`）で確かめる（[requirements_process.md](../../dev/developer_guide/requirements_process.md) §4）。
-   0008 評価記録は `approved` の文書である。参照を足す編集は、決定の変更ではなく編集上の修正として扱う（F-005、AC-22）。

## 6. 用語集 (Glossary)

-   **主経路**: 本ツールの主な使い方。ブラウザ拡張が文字起こしパネルからコピーした字幕を集め、Native Messaging のホストが localhost サーバへ中継し、サーバが記事を生成して Webhook へ投稿する経路。
-   **補助の経路**: CLI（`yt2column`）が動画の URL を受け取り、yt-dlp で字幕を取得して記事を生成する経路。LLM の呼び出しや投稿の処理を開発・検証するために使う。
-   **本番の構成**: 主経路で記事を作るときの、3 つの段階（生成・字数の調整・検証・推敲）のテンプレート・プロバイダ・モデル・effort の組。0008 評価記録 §6.1 の構成（0018 の基準の集合 B3 と同じ）。
-   **共通の文書**: 本タスクで作る `docs/dev/production_profile.md`。
-   **品質の目標・製品の制約**: 0008 要件定義書 F-006 と 0008 評価記録 §6.3 の用語に従う。
-   用語は `docs/translation_glossary.md` と統一する（コピーした字幕・文字起こし・ホストなど）。
