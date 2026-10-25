# 要件定義書：既定の LLM を Claude Opus 5.5（claude-opus-5-5）にする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-25 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、LLM のプロバイダとモデルの既定を、本番の構成（claude-opus-5-5）に合わせるための要件を定義する。環境変数で LLM の設定を何も指定しなければ、生成・字数の調整・検証・推敲の 3 つの段階を claude-opus-5-5 で行う。

**背景:**

タスク 0008 の最終の版の評価で、本番の構成を、3 つの段階すべてで claude-opus-5-5（effort `high`）を使う構成に決めた（`docs/tasks/0008_column_prompt/02_evaluation.md` §6.1・§6.4）。一方、コード上の既定は次のとおりで、本番の構成と一致しない。

-   `YT2COLUMN_LLM_PROVIDER` を指定しないと、生成のプロバイダは `deepseek` になる（`internal/config/config.go:437-451` の `parseProvider`。生成を実行しない実行で読む `loadProviderOptional`、同 `:397-410` も同じ）。
-   `YT2COLUMN_MODEL` には既定がなく、指定しないと `ErrMissing` で拒否される（同 `:545-552` の `loadModel`）。
-   `YT2COLUMN_CLAUDE_EFFORT` には既定がなく、生成または検証・推敲のプロバイダが `claude` のときに指定しないと拒否される（同 `:590-601` の `loadClaudeEffort`）。
-   検証・推敲は、`YT2COLUMN_REVIEW_PROVIDER` と `YT2COLUMN_REVIEW_MODEL` の両方を指定しなければ、生成のプロバイダとモデルを使う。`YT2COLUMN_REVIEW_PROVIDER` だけを指定すると、`YT2COLUMN_REVIEW_MODEL` を `ErrMissing` で拒否する（同 `:461-528` の `loadReview`）。

そのため、本番の構成を使うには、利用者が 3 つの環境変数を必ず指定しなければならない。0008 評価記録 §6.3 の製品の制約の 8 項目めも、「既定のプロバイダは DeepSeek のままなので、本書の構成は環境変数で指定する必要がある」としている。

設定は CLI（`Load`・`LoadFor`）と localhost サーバ（`LoadServer`）が同じ関数で読む（`LoadServer`・`Load`・`LoadFor` がどれも同 `:369-380` の `loadLLM` を呼ぶ）。したがって、既定の変更は 2 つの入口の両方に及ぶ。

対応 issue: [#490](https://github.com/isseis/yt2column/issues/490)（前提: [#171](https://github.com/isseis/yt2column/issues/171)、関連: [#488](https://github.com/isseis/yt2column/issues/488)）

**本書の記述範囲:** 本書は、環境変数の既定と、既定が当てはまる条件・当てはまらない条件、拒否するときに利用者が見るもの、文書に書く内容を定める。既定のモデル名をコードのどこに持つか、既定を当てはめる処理の組み立て、テストの手段は、設計（`02_architecture.md`）で決める。要件のレビューで挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #490 で決めたこと:**

-   **生成のプロバイダの既定を `claude` にする:** 本番の構成のプロバイダに合わせるため。
-   **effort の既定は #171 で扱う:** effort の既定（`high`。`YT2COLUMN_CLAUDE_EFFORT` で上書きできる）と `max_tokens` の上限の緩和は #171 で行い、本書は扱わない。#171 と本書は別の issue として起票されている。
-   **#488 との関係:** 本 issue は #488 をブロックする。#488 の文書（本番の構成の共通の文書、README の推奨構成など）は、既定が claude-opus-5-5 であることを前提に書く。

**要件定義の際に決めたこと（2026-10-25）:**

-   **モデルの既定はプロバイダが `claude` のときだけ当てはめる:** 生成のプロバイダが `claude` のとき（`YT2COLUMN_LLM_PROVIDER` を指定しないで `claude` になった場合を含む）、`YT2COLUMN_MODEL` を指定しなければ `claude-opus-5-5` を使う。プロバイダが `deepseek` のときは、今までどおり `YT2COLUMN_MODEL` を必須とする（F-002）。Claude のモデル名を DeepSeek の API に送ることを防ぐためである。モデルを必須のままにする案は、何も指定しない実行ができず、issue の目的を満たさないので採らなかった。
-   **検証・推敲のモデルにも同じ既定を当てはめる:** `YT2COLUMN_REVIEW_PROVIDER=claude` を指定し、`YT2COLUMN_REVIEW_MODEL` を指定しない場合も、検証・推敲のモデルを `claude-opus-5-5` にする（F-003）。プロバイダが `claude` ならモデルの既定がある、という規則を生成と検証・推敲でそろえるためである。`YT2COLUMN_REVIEW_PROVIDER` も `YT2COLUMN_REVIEW_MODEL` も指定しなければ、今までどおり生成と同じプロバイダとモデルを使う。
-   **DeepSeek を使っていた利用者には文書で移行を案内する:** `YT2COLUMN_LLM_PROVIDER` を指定しないで DeepSeek を使っていた利用者のために、互換の仕組み（モデルだけを指定した設定を拒否する、など）は足さない。代わりに、README と `docs/dev/project_overview.md` に、`YT2COLUMN_LLM_PROVIDER=deepseek` を指定すれば今までどおり動くことを書く（F-005）。レビュアーはどちらの案にも希望がなく、推奨の案を採った。理由は 2 つある。プロジェクトの利用者はリポジトリの外にいない（`CLAUDE.md`「Declare, don't infer」）。また、移行しない利用者は、`ANTHROPIC_API_KEY` が未設定なら、ネットワーク通信の前に設定エラーで止まる（AC-13）。
-   **#171 を前提にする:** 本タスクの実装は、#171 の実装（effort の既定 `high` と `max_tokens` の上限の緩和）が main に入った後にマージする（§5）。#171 より先に入れると、何も指定しない利用者は effort の未設定で拒否される。effort を指定した利用者も、長い動画で `max_tokens` 16,000 の打ち切りに当たりうる（0018 評価記録 §4.7.4）。
-   **`project_overview.md` の方針を書き直す:** 「LLM: 初期実装は DeepSeek」（`docs/dev/project_overview.md:12`）は、既定の LLM が Claude（claude-opus-5-5）で、DeepSeek も選べるという記述に改める。「モデル名は頻繁に更新される。コードにハードコードせず設定で与える」（同 `:67`）には、既定のモデル名 `claude-opus-5-5` をコードに持つことを例外として書き足す（F-005、§5）。経緯だけを残して注記を加える案は、方針と振る舞いが食い違って読めるので採らなかった。
-   **文書の分担:** 本タスクでは、振る舞いの変更に直接ついてくる記述だけを直す。対象は、README「Configuration」と `project_overview.md`「設定（環境変数）」の既定の記述、`project_overview.md` の方針の 2 か所、DeepSeek への移行の案内である。本番の構成の説明、README の推奨構成、主経路の書き直しは #488 に任せる（§2.3）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   LLM の設定を何も指定しなければ（`ANTHROPIC_API_KEY` だけを設定すれば）、本番の構成で記事を作れるようにする。
-   プロバイダを指定した利用者の振る舞いは、本書が定める既定を除いて変えない。
-   DeepSeek を使っていた利用者が、設定を 1 つ足すだけで今までどおり使えるようにし、その手順を文書で示す。

### 2.2. スコープ (In Scope)

-   `YT2COLUMN_LLM_PROVIDER` の既定を `claude` にする（F-001）。
-   生成のプロバイダが `claude` のとき、`YT2COLUMN_MODEL` の既定を `claude-opus-5-5` にする（F-002）。
-   検証・推敲のプロバイダが `claude` のとき、`YT2COLUMN_REVIEW_MODEL` の既定を `claude-opus-5-5` にする（F-003）。
-   CLI と localhost サーバの両方に、上の既定を当てはめる。CLI の中間生成物からの再開（`--after`）で生成を実行しない実行も含む（F-001〜F-003）。
-   README「Configuration」と `project_overview.md`「設定（環境変数）」の既定の記述、`project_overview.md` の方針の 2 か所を改め、DeepSeek への移行を案内する（F-005）。

### 2.3. スコープ外 (Out of Scope)

-   **effort の既定と `max_tokens` の上限:** #171 で扱う（issue #490 で決めたこと）。本書は #171 の結果を前提にする（§5）。
-   **本番の構成の共通の文書、README の推奨構成、主経路をブラウザ拡張に改める書き直し:** #488 で扱う（要件定義の際に決めたこと）。
-   **モデル名とプロバイダの組み合わせの検査:** `YT2COLUMN_MODEL` などに指定されたモデル名が、プロバイダに合うかを調べて拒否することはしない。モデル名の文字列の中身で振る舞いを選ばない（`CLAUDE.md`「Declare, don't infer」）。合わない組み合わせは、今までどおり API のエラーで失敗する。
-   **統合テストの既定:** `make test-integration-*` のモデルの既定（README「DeepSeek integration test」「CLI integration test」の `deepseek-flash` など）は、テスト用の変数の既定であり、本番の既定ではないので変えない。統合テストが本番の既定に依存して動かなくなる場合の扱いは、設計で決める（[design_handoff.md](design_handoff.md) H-01）。
-   **終わったタスクの記録の書き直し:** 0008 評価記録 §6.3 の 8 項目めなど、終わったタスクの文書に残る「既定のプロバイダは DeepSeek」という記述は、記録の時点の事実なので直さない。共通の文書からの参照の張り替えは #488 で扱う。
-   **単価の表:** `internal/pricing` の単価の表は変えない。claude-opus-5-5 の単価はすでにある（`internal/pricing/pricing_test.go:32-40` が引く）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

以下で「生成を実行する実行」は、生成または字数の調整を実行する実行（`internal/config` の `Need.Generation` が真）を指す。「生成を実行しない実行」は、`--after` による再開などで生成も字数の調整も実行せず、検証・推敲だけを実行する実行を指す。どちらの要件も、特に断らない限り CLI と localhost サーバの両方に当てはまる（localhost サーバは常に生成を実行する）。

既定が当てはまるのは、変数が**存在しない**場合だけである。変数が存在して値が空の場合は、今までどおり `ErrMissing` で拒否し、既定を当てはめない（`project_overview.md`「設定（環境変数）」の「空の値は未設定ではなくエラーとして扱う」）。

#### F-001: 生成のプロバイダの既定を `claude` にする

`YT2COLUMN_LLM_PROVIDER` が存在しなければ、生成のプロバイダを `claude` にする。

**Acceptance Criteria**:
- **AC-01**: 生成を実行する実行で `YT2COLUMN_LLM_PROVIDER` が存在しないとき、生成のプロバイダは `claude` になり、`ANTHROPIC_API_KEY` を読んで検査する。`DEEPSEEK_API_KEY` は、検証・推敲のプロバイダが `deepseek` でない限り読まず、未設定・空・どんな値でも拒否しない。
- **AC-02**: 生成を実行しない実行で、`YT2COLUMN_LLM_PROVIDER` と `YT2COLUMN_REVIEW_PROVIDER` がどちらも存在しないとき、検証・推敲は生成のプロバイダの既定 `claude` を受け継ぐ。
- **AC-03**: `YT2COLUMN_LLM_PROVIDER` が `deepseek` または `claude` のときは、指定した値のプロバイダを使う。値が空なら `ErrMissing`、それ以外の値なら `ErrInvalid` で、変数の名前を挙げて拒否する。拒否の理由の文言と、エラーに値を含めないことは変えない。

#### F-002: 生成のモデルの既定を、プロバイダが `claude` のとき `claude-opus-5-5` にする

生成のプロバイダが `claude` のときだけ、`YT2COLUMN_MODEL` が存在しなければ `claude-opus-5-5` を使う。

**Acceptance Criteria**:
- **AC-04**: 生成を実行する実行で、生成のプロバイダが `claude`（指定した場合も、F-001 の既定の場合も）で、`YT2COLUMN_MODEL` が存在しないとき、生成と字数の調整は `claude-opus-5-5` を Anthropic の API へのリクエストのモデル名として送る。
- **AC-05**: 生成を実行する実行で、生成のプロバイダが `deepseek` で `YT2COLUMN_MODEL` が存在しないとき、今までどおり `YT2COLUMN_MODEL` を挙げて `ErrMissing` で拒否し、LLM の API を呼ばない。
- **AC-06**: `YT2COLUMN_MODEL` が存在して値が空なら、プロバイダによらず `YT2COLUMN_MODEL` を挙げて `ErrMissing` で拒否する。値が空でなければ、プロバイダによらずその値をそのまま使う（既定で置き換えない）。
- **AC-07**: `YT2COLUMN_LLM_PROVIDER` の値が不正（AC-03 の `ErrInvalid`）で `YT2COLUMN_MODEL` が存在しないときは、モデルの既定を当てはめない。今までどおり、`YT2COLUMN_LLM_PROVIDER` の拒否と、`YT2COLUMN_MODEL` の `ErrMissing` の両方を報告する（README「Configuration」の「errors for common variables such as `YT2COLUMN_MODEL` are still reported」）。
- **AC-08**: 生成を実行しない実行で、`YT2COLUMN_REVIEW_PROVIDER` と `YT2COLUMN_REVIEW_MODEL` がどちらも存在せず、生成のプロバイダが `claude`（指定または既定）で、`YT2COLUMN_MODEL` が存在しないとき、検証・推敲は `claude-opus-5-5` を使う。生成のプロバイダが `deepseek` なら、今までどおり `YT2COLUMN_REVIEW_MODEL` を挙げて `ErrMissing` で拒否する。

#### F-003: 検証・推敲のモデルの既定を、プロバイダが `claude` のとき `claude-opus-5-5` にする

検証・推敲のプロバイダとモデルは、今までどおり、どちらも指定しなければ生成と同じにする。`YT2COLUMN_REVIEW_PROVIDER=claude` だけを指定した場合は、検証・推敲のモデルを `claude-opus-5-5` にする。

**Acceptance Criteria**:
- **AC-09**: `YT2COLUMN_REVIEW_PROVIDER` と `YT2COLUMN_REVIEW_MODEL` がどちらも存在しないとき、検証・推敲は生成と同じプロバイダとモデルを使う。生成のプロバイダとモデルが F-001・F-002 の既定になった場合は、その既定を使う。
- **AC-10**: `YT2COLUMN_REVIEW_PROVIDER` が `claude` で `YT2COLUMN_REVIEW_MODEL` が存在しないとき、検証・推敲は `claude-opus-5-5` を Anthropic の API へのリクエストのモデル名として送る。生成のモデル（`YT2COLUMN_MODEL` の値や既定）は受け継がない。
- **AC-11**: `YT2COLUMN_REVIEW_PROVIDER` が `deepseek` で `YT2COLUMN_REVIEW_MODEL` が存在しないとき、今までどおり `YT2COLUMN_REVIEW_MODEL` を挙げて `ErrMissing` で拒否する。
- **AC-12**: `YT2COLUMN_REVIEW_MODEL` だけが存在する（値は空でない）とき、今までどおり、検証・推敲は生成のプロバイダ（F-001 の既定を含む）とその値のモデルを使う。`YT2COLUMN_REVIEW_MODEL` が存在して値が空なら、今までどおり `ErrMissing` で拒否する。`YT2COLUMN_REVIEW_PROVIDER` の値が不正なときは、今までどおりその拒否だけを報告し、モデルの既定を当てはめない。

#### F-004: 何も指定しない実行と、DeepSeek の利用者の振る舞い

LLM の設定を何も指定しない実行が本番の構成で動くこと、DeepSeek を使っていた利用者が何を見て、どうすれば今までどおり使えるかを定める。

**Acceptance Criteria**:
- **AC-13**: 生成を実行する実行で、`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_REVIEW_PROVIDER`・`ANTHROPIC_API_KEY` のどれも存在しないとき（`DEEPSEEK_API_KEY` と `YT2COLUMN_MODEL` の有無と値は問わない）、CLI と localhost サーバは、`ANTHROPIC_API_KEY` を挙げた `ErrMissing` の設定エラーで止まり、LLM の API にも Webhook にも通信しない。エラーに API キーの値を含めない。
- **AC-14**: LLM に関わる変数（`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL`・`YT2COLUMN_CLAUDE_EFFORT`・`ANTHROPIC_API_KEY`・`ANTHROPIC_WORKSPACE_ID`・`DEEPSEEK_API_KEY`）のうち `ANTHROPIC_API_KEY` だけが空でない値で存在し、#171 の実装が入っているとき、生成・字数の調整・検証・推敲の 3 つの段階は、どれも `claude-opus-5-5` を、#171 の effort の既定（`high`）で、Anthropic の API へ送る。記事のモデルの記録（`--out` のフロントマターと Slack の投稿の脚注）は、今までどおり応答が報告したモデル名を示す。
- **AC-15**: `YT2COLUMN_LLM_PROVIDER=deepseek`・`YT2COLUMN_MODEL`・`DEEPSEEK_API_KEY` を指定した実行は、検証・推敲の変数を指定しなければ、本書の変更の前と同じく、生成・字数の調整・検証・推敲のすべてを DeepSeek で行い、Anthropic に関わる変数（`ANTHROPIC_API_KEY`・`YT2COLUMN_CLAUDE_EFFORT`・`ANTHROPIC_WORKSPACE_ID`）を読まず、どんな値でも拒否しない。

#### F-005: 文書を既定の変更に合わせる

README（英語）と `docs/dev/project_overview.md`（日本語）の既定の記述と方針を、本書の振る舞いに合わせる。

**Acceptance Criteria**:
- **AC-16**: README「Configuration」の表と `project_overview.md`「設定（環境変数）」の表で、`YT2COLUMN_LLM_PROVIDER` の未設定のときの値が `claude` になっている。`YT2COLUMN_MODEL` の未設定のときの値は、プロバイダが `claude` なら `claude-opus-5-5`、`deepseek` ならエラー（必須）と書かれている。`YT2COLUMN_REVIEW_MODEL` の未設定のときの値は、`YT2COLUMN_REVIEW_PROVIDER` を設定した場合に、それが `claude` なら `claude-opus-5-5`、`deepseek` ならエラー（必須）、設定しない場合は生成と同じ、と書かれている。2 つの表の値はコードの振る舞いと一致する。
- **AC-17**: README と `project_overview.md` に、DeepSeek への移行の案内がある。内容は、`YT2COLUMN_LLM_PROVIDER` を指定しないで DeepSeek を使っていた場合、既定の変更で `ANTHROPIC_API_KEY` が必要になり、`YT2COLUMN_LLM_PROVIDER=deepseek` を指定すれば今までどおり DeepSeek で動くこと、である。
- **AC-18**: `project_overview.md`「決定済みの方針」の LLM の項が、既定の LLM が Claude（claude-opus-5-5）で、DeepSeek も選べることを述べている。DeepSeek と Claude の API を SDK を使わず標準ライブラリで呼ぶことの記述は残っている。
- **AC-19**: `project_overview.md`「前提・制約」のモデル名の方針に、既定のモデル名 `claude-opus-5-5` をコードに持つことが例外として、その範囲と理由とともに書かれている。範囲は、プロバイダが `claude` でモデルの変数が存在しないときの既定に限ること、理由は 0008 の評価で本番の構成として決めたモデルであること、である。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。設定の読み込みの処理量は変わらない。ただし、何も指定しない実行の LLM の費用は、DeepSeek の構成より高くなる（記事 1 本あたりの推定コストは約 0.36 USD で、DeepSeek を生成に使う構成の約 2.8 倍。0008 評価記録 §6.3 の 7 項目め・§6.4）。この費用は本番の構成を既定にすることの結果であり、本書は費用の上限を定めない。

### 4.2. セキュリティ (Security)

`docs/dev/security.md` を適用する。本タスクは、API キーを扱い、LLM の API にデータを送るので、条件付きのガイドの対象である。

-   **送り先の事業者:** プロバイダを指定しない実行の送り先は、DeepSeek から Anthropic に変わる。送るデータ（字幕・概要欄・動画タイトル・チャンネル名・生成した文面・プロンプト）は変わらない。Anthropic に送るデータの扱いは `security.md` §4 にすでに書かれているので、本書は `security.md` を変えない。
-   **読む秘密情報:** プロバイダの既定が変わるので、何も指定しない実行で読む API キーは `DEEPSEEK_API_KEY` から `ANTHROPIC_API_KEY` に変わる。選んだプロバイダの変数だけを読んで検査し、ほかのプロバイダの変数は読まないという規則（`project_overview.md`「設定（環境変数）」）は変えない（AC-01・AC-15）。
-   **エラーに値を含めない:** 拒否のエラーは、変数の名前と固定の理由だけを持ち、値を含めない（`internal/config/config.go:59-73` の固定の理由、`:86-105` の `VarError`）。本書が足す既定でも、この規則を保つ（AC-03・AC-13）。
-   **信頼できない入力:** 本書が新しく受け付ける信頼できない入力はない。環境変数は利用者が与える設定で、受理する値の形（`deepseek`・`claude`、空でない任意のモデル名）と、拒否の番兵（`ErrMissing`・`ErrInvalid`）は変えない。前後の空白の除去などの補正をしない規則も変えない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   設定の拒否は、今までどおり LLM の API の呼び出しの前に起こる（AC-05・AC-13）。
-   #171 の前提（§5）により、何も指定しない実行が effort の未設定で拒否されることも、`max_tokens` 16,000 の打ち切りに当たりやすい状態で既定になることもない。

### 4.4. 互換性 (Compatibility)

-   **DeepSeek を使っていた利用者:** `YT2COLUMN_LLM_PROVIDER` を指定しないで DeepSeek を使っていた利用者は、変更後、`ANTHROPIC_API_KEY` が未設定なら設定エラーで止まる（AC-13）。`ANTHROPIC_API_KEY` を別の用途で設定していて、`YT2COLUMN_MODEL=deepseek-flash` などを指定していた場合は、そのモデル名が Anthropic の API に送られ、API のエラーで記事が出ない（§2.3 のモデル名とプロバイダの組み合わせの検査はしない）。どちらも、`YT2COLUMN_LLM_PROVIDER=deepseek` を指定すれば今までどおり動く（AC-15・AC-17）。
-   **プロバイダを指定していた利用者:** `YT2COLUMN_LLM_PROVIDER` と `YT2COLUMN_MODEL` を指定していた利用者の振る舞いは変わらない。`YT2COLUMN_REVIEW_PROVIDER=claude` を指定して `YT2COLUMN_REVIEW_MODEL` を指定していなかった設定は、今まで拒否されていたが、`claude-opus-5-5` で動くようになる（AC-10）。

### 4.5. 保守性 (Maintainability)

-   既定のモデル名は、本番の構成を見直したときに変える値である。値を変えるときに直す箇所が、コードの 1 か所と、それを述べる文書（README・`project_overview.md`）に限られるようにする（[design_handoff.md](design_handoff.md) H-02）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。ただし、次の 2 つは本書で方針を改める。
    -   **「LLM: 初期実装は DeepSeek」**（`project_overview.md:12`）: 既定の LLM を Claude（claude-opus-5-5）にし、DeepSeek は選べるプロバイダとして残す。理由: 0008 の評価で本番の構成を claude-opus-5-5 に決めた（0008 評価記録 §6.1・§6.4）。
    -   **「モデル名は頻繁に更新される。コードにハードコードせず設定で与える」**（`project_overview.md:67`）: 例外として、既定のモデル名 `claude-opus-5-5` をコードに持つ。範囲は、プロバイダが `claude` で、モデルの変数（`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_MODEL`）が存在しないときの既定に限る。指定されたモデル名は、今までどおり設定で与えたものをそのまま使う。理由: 何も指定しなければ本番の構成で動くことが issue の目的であり、モデル名の既定がなければその目的を満たせない。本番の構成が変われば、既定も変える（§4.5）。
-   **#171 の後にマージする:** 本タスクの実装は、#171 の実装（effort の既定 `high`・`max_tokens` の上限の緩和）が main に入った後にマージする。設計と実装計画は #171 と並行して進めてよい。
-   **#488 との順序:** #488（タスク 0040）の文書は、本タスクが定める既定を前提に書く。本タスクが直す README と `project_overview.md` の箇所は、#488 も直す可能性がある。後にマージするほうが、先にマージされた内容に合わせる。
-   CLI と localhost サーバの設定は `internal/config` の同じ関数で読む（`CLAUDE.md`「Separation of Concerns」）。既定の当てはめも 2 つの入口で共有する。
-   既定を当てはめるかは、プロバイダの型（`config.Provider`）で決め、モデル名などの文字列の中身で決めない（`CLAUDE.md`「Declare, don't infer」）。

## 6. 用語集 (Glossary)

用語は `docs/translation_glossary.md` と統一する。

-   **本番の構成:** 0008 の評価で決めた、生成・字数の調整・検証・推敲の 3 つの段階をすべて claude-opus-5-5（effort `high`）で行う構成（0008 評価記録 §6.1）。
-   **既定:** 環境変数が存在しないときに使う値。変数が存在して値が空の場合には当てはめない。
-   **生成を実行する実行／生成を実行しない実行:** §3.1 の冒頭の定義による。
-   **プロバイダ（provider）:** LLM の提供元。本書では `deepseek` と `claude`。
