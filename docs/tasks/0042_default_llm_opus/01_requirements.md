# 要件定義書：既定の LLM を Claude Opus 5.5（claude-opus-5-5）にする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-25 |
| Review date | - |
| Reviewer | - |
| Comments | 2026-10-25、#171 を待つ範囲を、既定を変える変更（F-001〜F-004）とそれを述べる文書の変更（F-006）に狭めた（§1「#171 を前提にする」、§5）。振る舞いを変えない準備（テストの環境で DeepSeek を明示する変更と、その検査）は、#171 より先にマージしてよい。決定の変更なので本書を `draft` に戻し、PR の `/runpr` のマージの問いで改めて承認を受ける。それまでの承認は、レビュー日 2026-10-25、レビュアー isseis である。 |

## 1. 概要 (Overview)

**目的:** 本書は、LLM のプロバイダとモデルの既定を、本番の構成（claude-opus-5-5）に合わせるための要件を定義する。LLM のプロバイダとモデルの変数をどれも設定しなければ、生成・字数の調整・検証・推敲を claude-opus-5-5 で行う。

**背景:**

タスク 0008 の最終の版の評価で、本番の構成を、生成・字数の調整・検証・推敲のすべてで claude-opus-5-5（effort `high`）を使うものに決めた（`docs/tasks/0008_column_prompt/02_evaluation.md` §6.1・§6.4）。一方、コード上の既定は次のとおりで、本番の構成と一致しない。

-   `YT2COLUMN_LLM_PROVIDER` が未設定なら、生成のプロバイダは `deepseek` になる（`internal/config/config.go:437-450` の `parseProvider`）。生成を実行しない実行（§3.1 で定義する実行の種類）で生成のプロバイダを読む `loadProviderOptional`（同 `:399-411`）も同じである。
-   `YT2COLUMN_MODEL` には既定がなく、未設定なら `ErrMissing` で拒否する（同 `:545-552` の `loadModel`）。
-   `YT2COLUMN_CLAUDE_EFFORT` には既定がなく、生成または検証・推敲のプロバイダが `claude` のときに未設定なら拒否する（同 `:590-601` の `loadClaudeEffort`）。
-   検証・推敲は、`YT2COLUMN_REVIEW_PROVIDER` と `YT2COLUMN_REVIEW_MODEL` がどちらも未設定なら、生成のプロバイダとモデルを使う。`YT2COLUMN_REVIEW_PROVIDER` だけを設定すると、`YT2COLUMN_REVIEW_MODEL` を `ErrMissing` で拒否する（同 `:461-514` の `loadReview`）。

そのため、本番の構成を使うには、利用者が上の 3 つの環境変数を必ず設定しなければならない。0008 評価記録 §6.3 の製品の制約の 8 番目の項目も、「既定のプロバイダは DeepSeek のままなので、本書の構成は環境変数で指定する必要がある」としている。

設定は、CLI（`Load`・`LoadFor`）と localhost サーバ（`LoadServer`）が、どちらも同じ `loadLLM`（同 `:369-380`）で読む。したがって、既定の変更は 2 つの入口の両方に及ぶ。

対応 issue: [#490](https://github.com/isseis/yt2column/issues/490)（前提: [#171](https://github.com/isseis/yt2column/issues/171)、関連: [#488](https://github.com/isseis/yt2column/issues/488)）

#171 の要件は、タスク `0041_claude_effort_max_tokens` の [要件定義書](../0041_claude_effort_max_tokens/01_requirements.md) が定める。

**本書の記述範囲:** 本書は、環境変数の既定、既定を当てはめる条件と当てはめない条件、拒否するときに利用者が見るもの、文書に書く内容を定める。既定のモデル名をコードのどこに持つか、既定を当てはめる処理の組み立て、拒否の理由の正確な文言、テストの手段は、設計（`02_architecture.md`）で決める。要件のレビューで挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #490 で決めたこと:**

-   **生成のプロバイダの既定を `claude` にする:** 本番の構成のプロバイダに合わせるため。
-   **effort の既定は #171 で扱う:** effort の既定（`high`。`YT2COLUMN_CLAUDE_EFFORT` で上書きできる）と `max_tokens` の上限の緩和は #171 で行い、本書は扱わない。2 つの作業は別の issue として起票されている。
-   **#488 との関係:** 本 issue は #488 をブロックする。#488 の文書（本番の構成の共通の文書、README の推奨構成など）は、既定が claude-opus-5-5 であることを前提に書く。

**要件定義の際に決めたこと（2026-10-25）:**

-   **モデルの既定は、プロバイダが `claude` のときだけ当てはめる:** 生成のプロバイダが `claude` で `YT2COLUMN_MODEL` が未設定なら、`claude-opus-5-5` を使う。プロバイダが `deepseek` のときは、今までどおり `YT2COLUMN_MODEL` を必須とする（F-002）。理由: Claude のモデル名を DeepSeek の API に送ることを防ぐため。モデルを必須のままにする案は、何も設定しない実行ができず issue の目的を満たさないので、採らなかった。
-   **モデルだけを設定し、プロバイダを設定しない設定は拒否する:** モデルの変数（`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_MODEL`）を設定したのに、そのモデルを使う段階のプロバイダが F-001 の既定で決まる場合は、設定エラーにする（F-004）。理由: 本書の変更の前は、この設定のプロバイダは `deepseek` だった。`YT2COLUMN_LLM_PROVIDER` を設定せずに DeepSeek を使っていた利用者が `ANTHROPIC_API_KEY` を別の用途（たとえば、検証・推敲を Claude で行う構成や、ほかのツール。README「Configuration」を参照）で設定していると、DeepSeek 用のモデル名と、字幕・概要欄・プロンプトが、利用者が選んでいない Anthropic に送られる。拒否すれば、通信の前に止まる。判定は変数が存在するかどうかだけで行い、モデル名の文字列の中身は見ない（`CLAUDE.md`「Declare, don't infer」）。その代償として、プロバイダを設定せずにモデルだけを設定して、既定以外の Claude のモデルを使うことはできなくなる。文書で案内するだけの案は、この経路を防げないので採らなかった。`YT2COLUMN_REVIEW_MODEL` にも同じ規則を当てはめる。
-   **検証・推敲のモデルにも同じ既定を当てはめる:** `YT2COLUMN_REVIEW_PROVIDER` が `claude` で `YT2COLUMN_REVIEW_MODEL` が未設定なら、検証・推敲のモデルを `claude-opus-5-5` にする（F-003）。生成のプロバイダが `claude` で、生成のモデルが `claude-opus-5-5` 以外であっても、生成のモデルは受け継がない。理由: 「プロバイダが `claude` ならモデルの既定は `claude-opus-5-5`」という 1 つの規則を、生成と検証・推敲でそろえるため。検証・推敲のプロバイダが生成と同じなら生成のモデルを受け継ぐ案は、規則が 2 つになるので採らなかった。`YT2COLUMN_REVIEW_PROVIDER` と `YT2COLUMN_REVIEW_MODEL` がどちらも未設定なら、今までどおり生成と同じプロバイダとモデルを使う。
-   **DeepSeek を使い続けるための案内を文書に書く:** README と `docs/dev/project_overview.md` に、`YT2COLUMN_LLM_PROVIDER=deepseek` を設定すれば今までどおり DeepSeek で動くことを書く（F-006）。拒否の理由の文言は固定の文で（`internal/config/config.go:59-73`）、既定が変わったことまでは伝えないからである。
-   **#171 を前提にする:** 本タスクの実装のうち既定を変える変更は、#171 の実装（effort の既定 `high` と `max_tokens` の上限の緩和）が main に入った後にマージする（§5）。#171 より先に入れると、何も設定しない利用者は `YT2COLUMN_CLAUDE_EFFORT` の未設定で拒否される。effort を設定した利用者も、長い動画で `max_tokens` 16,000 の打ち切りに当たりうる（0018 評価記録 §4.7.4）。
-   **`project_overview.md` の方針を書き直す:** 「LLM: 初期実装は DeepSeek」（`docs/dev/project_overview.md:12`）は、既定の LLM が Claude（claude-opus-5-5）で、DeepSeek も選べるという記述に改める。「モデル名は頻繁に更新される。コードにハードコードせず設定で与える」（同 `:67`）には、既定のモデル名 `claude-opus-5-5` をコードに持つことを例外として書き足す（F-006、§5）。経緯を残して注記を加える案は、方針と振る舞いが食い違って読めるので採らなかった。
-   **文書の分担:** 本タスクでは、振る舞いの変更に直接伴う記述だけを直す。対象は、README「Configuration」と `project_overview.md`「設定（環境変数）」の既定と必須の記述、`project_overview.md` の方針の 2 か所、DeepSeek を使い続けるための案内である。本番の構成の説明、README の推奨構成、主経路の書き直しは #488 に任せる（§2.3）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   LLM の変数のうち `ANTHROPIC_API_KEY` だけを設定すれば、本番の構成で記事を作れるようにする。
-   プロバイダとモデルを設定している利用者の振る舞いは、本書が定める既定を除いて変えない。
-   プロバイダを設定せずに DeepSeek を使っていた利用者の設定は、通信の前に拒否し、利用者の選んでいない事業者にデータを送らないようにする。設定を 1 つ足せば今までどおり使えることを、文書で示す。

### 2.2. スコープ (In Scope)

-   `YT2COLUMN_LLM_PROVIDER` の既定を `claude` にする（F-001）。
-   プロバイダが `claude` のとき、`YT2COLUMN_MODEL` と `YT2COLUMN_REVIEW_MODEL` の既定を `claude-opus-5-5` にする（F-002・F-003）。
-   モデルの変数を設定したのにプロバイダが既定で決まる設定を拒否する（F-004）。
-   CLI と localhost サーバの両方に、上の振る舞いを当てはめる。CLI の中間生成物からの再開（`--after`）で、検証・推敲だけを実行する実行（§3.1 で定義）も含む（F-001〜F-004）。
-   README「Configuration」と `project_overview.md`「設定（環境変数）」の既定と必須の記述、`project_overview.md` の方針の 2 か所を改め、DeepSeek を使い続けるための案内を書く（F-006）。

### 2.3. スコープ外 (Out of Scope)

-   **effort の既定と `max_tokens` の上限:** #171 で扱う（issue #490 で決めたこと）。本書は #171 の結果を前提にする（§5）。
-   **本番の構成の共通の文書、README の推奨構成、主経路をブラウザ拡張に改める書き直し:** #488 で扱う（要件定義の際に決めたこと）。README の冒頭の「initial LLM provider is DeepSeek」の類の記述も、#488 の書き直しの範囲である。
-   **モデル名とプロバイダの組み合わせの検査:** 設定したモデル名がプロバイダに合うかを、名前の中身で調べて拒否することはしない（`CLAUDE.md`「Declare, don't infer」）。プロバイダを設定したうえで合わないモデル名を設定した場合は、今までどおり API の呼び出しで失敗する。
-   **統合テストの既定:** `make test-integration-*` のモデルの既定（README「DeepSeek integration test」「CLI integration test」の `deepseek-flash` など）は、テスト用の変数の既定であり、本番の既定ではないので変えない。ただし、CLI の統合テストが今までどおり DeepSeek だけを呼ぶことは F-005 で定める。
-   **終わったタスクの記録の書き直し:** 0008 評価記録 §6.3 の 8 項目めなど、終わったタスクの文書に残る「既定のプロバイダは DeepSeek」という記述は、記録の時点の事実なので直さない。共通の文書からの参照の張り替えは #488 で扱う。
-   **単価の表:** `internal/pricing` の単価の表は変えない。claude-opus-5-5 の単価はすでにある（`internal/pricing/pricing_test.go:32-40` が引く）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

**本書の用語:**

-   **未設定:** 環境変数が存在しないこと。変数が存在して値が空の場合は「空」と呼び、未設定に含めない。空の値は、今までどおり `ErrMissing` で拒否し、既定を当てはめない（`project_overview.md`「設定（環境変数）」の「空の値は未設定ではなくエラーとして扱う」）。
-   **実行の種類:** 設定を読む段階の違いによって、実行を次の 3 つに分ける（`internal/config/config.go:331-334` の `Need`、`cmd/yt2column/run.go:481-491` の `resumeNeed`）。
    -   **生成を実行する実行:** 生成または字数の調整を実行する実行。CLI の `--after` のない実行、`--after generation` で `--no-shorten` を指定しない実行、localhost サーバのすべての実行（`internal/config/config.go:260`）がこれに当たる。`--no-review` を指定した実行も含む。
    -   **検証・推敲だけを実行する実行:** `--after` による再開で、生成も字数の調整も実行せず、検証・推敲を実行する実行。
    -   **LLM を呼ばない実行:** `--after review` など、どの LLM の段階も実行しない実行。
-   **検証・推敲を実行する実行:** 検証・推敲の設定を読む実行（`Need.Review` が真）。生成を実行する実行のうち `--after` のないものは、`--no-review` を指定しても、検証・推敲の設定を読むのでこれに含める（README「Post-generation steps」の「The review step's configuration is validated even with `--no-review`」）。検証・推敲だけを実行する実行も含む。
-   **既定で決まるプロバイダ:** 生成のプロバイダは、`YT2COLUMN_LLM_PROVIDER` が未設定のとき既定で決まる。検証・推敲のプロバイダは、`YT2COLUMN_REVIEW_PROVIDER` と `YT2COLUMN_LLM_PROVIDER` がどちらも未設定のとき既定で決まる。

特に断らない限り、各要件は CLI と localhost サーバの両方に当てはまる。拒否は、今までどおり、変数の名前と固定の理由だけを示し、値を含めない。CLI は終了コード 2 で終了し、localhost サーバは待ち受けを始める前に終了コード 2 で終了する（README「Exit codes」と「Localhost server」の終了コードの表）。どちらも LLM の API にも Webhook にも通信しない。CLI は `yt-dlp` も起動しない。

#### F-001: 生成のプロバイダの既定を `claude` にする

`YT2COLUMN_LLM_PROVIDER` が未設定なら、生成のプロバイダを `claude` にする。

**Acceptance Criteria**:
- **AC-01**: 生成を実行する実行で、`YT2COLUMN_LLM_PROVIDER` と `YT2COLUMN_MODEL` がどちらも未設定のとき、生成のプロバイダは `claude` になり、`ANTHROPIC_API_KEY` を読んで検査する。検証・推敲のプロバイダも `deepseek` でなければ、`DEEPSEEK_API_KEY` は読まず、未設定・空・どの値でも拒否しない。
- **AC-02**: `YT2COLUMN_LLM_PROVIDER` が `deepseek` または `claude` のときは、その値のプロバイダを使う。空なら `ErrMissing`、それ以外の値なら `ErrInvalid` で、変数の名前を挙げて拒否する（今までどおり）。

#### F-002: 生成のモデルの既定を、プロバイダが `claude` のとき `claude-opus-5-5` にする

**Acceptance Criteria**:
- **AC-03**: 生成を実行する実行で、生成のプロバイダが `claude`（`YT2COLUMN_LLM_PROVIDER=claude` を設定した場合も、F-001 の既定の場合も）で、`YT2COLUMN_MODEL` が未設定のとき、生成と字数の調整（実行される場合）は、`claude-opus-5-5` をモデル名として Anthropic の API へ送る。
- **AC-04**: 生成を実行する実行で、`YT2COLUMN_LLM_PROVIDER=deepseek` で `YT2COLUMN_MODEL` が未設定のとき、`YT2COLUMN_MODEL` を挙げて `ErrMissing` で拒否する（今までどおり）。
- **AC-05**: 生成を実行する実行で、`YT2COLUMN_LLM_PROVIDER` が `deepseek` または `claude` で、`YT2COLUMN_MODEL` が空なら、`YT2COLUMN_MODEL` を挙げて `ErrMissing` で拒否する。空でない値なら、その値をそのまま生成のモデルにする（既定で置き換えない。今までどおり）。
- **AC-06**: 生成を実行する実行で、`YT2COLUMN_LLM_PROVIDER` が空または不正（AC-02 の拒否）で、`YT2COLUMN_MODEL` が未設定のときは、モデルの既定を当てはめない。`YT2COLUMN_LLM_PROVIDER` の拒否と、`YT2COLUMN_MODEL` の `ErrMissing` の両方を報告する（今までどおり。README「Configuration」の「errors for common variables such as `YT2COLUMN_MODEL` are still reported」）。

#### F-003: 検証・推敲のプロバイダとモデルの既定

検証・推敲のプロバイダとモデルは、今までどおり、どちらも未設定なら生成と同じにする。`YT2COLUMN_REVIEW_PROVIDER=claude` だけを設定した場合は、検証・推敲のモデルを `claude-opus-5-5` にする。

**Acceptance Criteria**:
- **AC-07**: 生成を実行する実行で、`YT2COLUMN_REVIEW_PROVIDER` と `YT2COLUMN_REVIEW_MODEL` がどちらも未設定のとき、検証・推敲は、生成と同じプロバイダとモデル（F-001・F-002 の既定を含む）を使う。
- **AC-08**: 検証・推敲だけを実行する実行で、`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL` がどれも未設定のとき、検証・推敲は `claude` のプロバイダで `claude-opus-5-5` をモデル名として Anthropic の API へ送る。
- **AC-09**: 検証・推敲だけを実行する実行で、`YT2COLUMN_REVIEW_PROVIDER` と `YT2COLUMN_REVIEW_MODEL` がどちらも未設定のとき、次のとおりにする。`YT2COLUMN_LLM_PROVIDER=claude` で `YT2COLUMN_MODEL` が未設定なら、検証・推敲は `claude-opus-5-5` を使う。`YT2COLUMN_LLM_PROVIDER=deepseek` で `YT2COLUMN_MODEL` が未設定なら、`YT2COLUMN_REVIEW_MODEL` を挙げて `ErrMissing` で拒否する（今までどおり）。
- **AC-10**: 検証・推敲を実行する実行で、`YT2COLUMN_REVIEW_PROVIDER=claude` で `YT2COLUMN_REVIEW_MODEL` が未設定のとき、検証・推敲は `claude-opus-5-5` をモデル名として Anthropic の API へ送る。生成のモデル（`YT2COLUMN_MODEL` の値や既定）は受け継がない。
- **AC-11**: 検証・推敲を実行する実行で、`YT2COLUMN_REVIEW_PROVIDER=deepseek` で `YT2COLUMN_REVIEW_MODEL` が未設定のとき、`YT2COLUMN_REVIEW_MODEL` を挙げて `ErrMissing` で拒否する（今までどおり）。
- **AC-12**: 検証・推敲を実行する実行で、`YT2COLUMN_REVIEW_PROVIDER` が設定されていて（`deepseek` または `claude`）、`YT2COLUMN_REVIEW_MODEL` が空でない値のとき、その 2 つを検証・推敲のプロバイダとモデルにする。`YT2COLUMN_REVIEW_MODEL` が空なら、`YT2COLUMN_REVIEW_MODEL` を挙げて `ErrMissing` で拒否する（今までどおり）。
- **AC-13**: 検証・推敲を実行する実行で、`YT2COLUMN_REVIEW_PROVIDER` が空または不正なとき、`YT2COLUMN_REVIEW_PROVIDER` を拒否し（空なら `ErrMissing`、不正なら `ErrInvalid`）、検証・推敲のモデルの既定を当てはめない。`YT2COLUMN_REVIEW_MODEL` が未設定でも、`YT2COLUMN_REVIEW_MODEL` の拒否は加えない（今までどおり。`internal/config/config.go:466-472`・`:495-498`）。生成の側の拒否は、この規則と関係なく報告する。
- **AC-14**: 検証・推敲を実行する実行で、`YT2COLUMN_REVIEW_MODEL` が空なら、プロバイダがどう決まるか（`YT2COLUMN_REVIEW_PROVIDER` が設定されている、未設定、空、不正のどれか）によらず、`YT2COLUMN_REVIEW_MODEL` を挙げて `ErrMissing` で拒否する（今までどおり）。検証・推敲だけを実行する実行では、検証・推敲が `YT2COLUMN_MODEL` を受け継ぐ場合（AC-09・AC-18）に限り、`YT2COLUMN_MODEL` が空なら、`YT2COLUMN_MODEL` を挙げて `ErrMissing` で拒否する（今までどおり）。
- **AC-15**: LLM を呼ばない実行では、今までどおり、LLM のプロバイダ・モデル・API キー・effort・ワークスペース ID の変数をどれも読まず、どの値でも拒否しない。既定も当てはめない。

#### F-004: モデルだけを設定し、プロバイダが既定で決まる設定を拒否する

モデルの変数を設定したのに、そのモデルを使う段階のプロバイダが既定で決まる場合は、設定エラーにする。

**Acceptance Criteria**:
- **AC-16**: 生成を実行する実行で、`YT2COLUMN_LLM_PROVIDER` が未設定で、`YT2COLUMN_MODEL` が設定されている（空を含む）とき、`YT2COLUMN_LLM_PROVIDER` を挙げて `ErrMissing` で拒否する。`YT2COLUMN_MODEL` が空なら、`YT2COLUMN_MODEL` の `ErrMissing` も報告する。
- **AC-17**: 検証・推敲を実行する実行で、`YT2COLUMN_REVIEW_PROVIDER` と `YT2COLUMN_LLM_PROVIDER` がどちらも未設定で、`YT2COLUMN_REVIEW_MODEL` が設定されている（空を含む）とき、`YT2COLUMN_LLM_PROVIDER` を挙げて `ErrMissing` で拒否する。
- **AC-18**: 検証・推敲だけを実行する実行で、`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL` がどれも未設定で、`YT2COLUMN_MODEL` が設定されている（空を含む）とき、`YT2COLUMN_LLM_PROVIDER` を挙げて `ErrMissing` で拒否する。
- **AC-19**: AC-16〜AC-18 の拒否は、1 つの実行で複数の条件に当てはまっても、`YT2COLUMN_LLM_PROVIDER` の拒否を 1 件だけ報告する。拒否の理由は固定の文で、値を含まず、モデルの変数が設定されているのでプロバイダの変数が必要であることを示す。この拒否があっても、既定で決まったプロバイダ（`claude`）の変数（`ANTHROPIC_API_KEY`・`YT2COLUMN_CLAUDE_EFFORT`・`ANTHROPIC_WORKSPACE_ID`）は検査し、その拒否も合わせて報告する。どの場合も通信はしない（§3.1）。
- **AC-20**: `YT2COLUMN_LLM_PROVIDER` が設定されていれば（`deepseek` または `claude`）、AC-16〜AC-18 の拒否は起こらない。`YT2COLUMN_REVIEW_PROVIDER` が設定されていれば、AC-17 の拒否は起こらない。

#### F-005: 何も設定しない実行と、DeepSeek の利用者の振る舞い

**Acceptance Criteria**:
- **AC-21**: 生成を実行する実行で、`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL`・`ANTHROPIC_API_KEY` がどれも未設定のとき（`DEEPSEEK_API_KEY` の有無と値は問わない）、報告される拒否には、`ANTHROPIC_API_KEY` を挙げた `ErrMissing` が含まれ、`DEEPSEEK_API_KEY` の拒否は含まれない。エラーに API キーの値を含めない。
- **AC-22**: #171 の実装が入っている状態で、`ANTHROPIC_API_KEY` だけが空でない値で設定され、ほかの LLM の変数がどれも未設定のとき、生成を実行する実行は、生成・字数の調整（実行される場合）・検証・推敲のどれでも、`claude-opus-5-5` をモデル名として、effort を #171 の既定（`high`）にして Anthropic の API へ送る。ここでの LLM の変数は、本書の時点で `internal/config` が読む `YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL`・`YT2COLUMN_CLAUDE_EFFORT`・`ANTHROPIC_API_KEY`・`ANTHROPIC_WORKSPACE_ID`・`DEEPSEEK_API_KEY` である。
- **AC-23**: `YT2COLUMN_LLM_PROVIDER=deepseek`・`YT2COLUMN_MODEL`（空でない値）・`DEEPSEEK_API_KEY` を設定し、`YT2COLUMN_REVIEW_PROVIDER` と `YT2COLUMN_REVIEW_MODEL` が未設定の実行は、本書の変更の前と同じく、生成・字数の調整・検証・推敲のすべてを DeepSeek で行う。Anthropic の変数（`ANTHROPIC_API_KEY`・`YT2COLUMN_CLAUDE_EFFORT`・`ANTHROPIC_WORKSPACE_ID`）を読まず、どの値でも拒否しない。
- **AC-24**: CLI の統合テスト（`make test-integration-cli`・`make test-integration-cli-slack`）は、本書の変更の後も DeepSeek の API だけを呼び、Anthropic の API を呼ばない。

#### F-006: 文書を既定の変更に合わせる

README（英語）と `docs/dev/project_overview.md`（日本語）の既定と必須の記述、方針を、本書の振る舞いに合わせる。

**Acceptance Criteria**:
- **AC-25**: README「Configuration」の表と、`project_overview.md`「設定（環境変数）」の表で、次の値が書かれている。`YT2COLUMN_LLM_PROVIDER` の未設定のときの値は `claude`。`YT2COLUMN_MODEL` の未設定のときの値は、プロバイダが `claude` なら `claude-opus-5-5`、`deepseek` ならエラー（必須）。`YT2COLUMN_REVIEW_MODEL` の未設定のときの値は、`YT2COLUMN_REVIEW_PROVIDER` が `claude` なら `claude-opus-5-5`、`deepseek` ならエラー（必須）、`YT2COLUMN_REVIEW_PROVIDER` も未設定なら生成と同じ。
- **AC-26**: README「Configuration」と `project_overview.md`「設定（環境変数）」の、表の説明の列と本文に、`YT2COLUMN_REVIEW_PROVIDER` を設定すれば `YT2COLUMN_REVIEW_MODEL` が常に必須である、という記述が残っていない。代わりに AC-10・AC-11 の規則が書かれている。F-004 の拒否（モデルの変数だけを設定し、プロバイダが既定で決まる設定は拒否する）も書かれている。
- **AC-27**: README と `project_overview.md` に、DeepSeek を使い続けるための案内がある。内容は次のとおり。`YT2COLUMN_LLM_PROVIDER` を設定せずに DeepSeek を使っていた場合、既定の変更の後は設定エラー（`ANTHROPIC_API_KEY` または `YT2COLUMN_LLM_PROVIDER` の拒否）になる。`YT2COLUMN_LLM_PROVIDER=deepseek` を設定すれば、今までどおり DeepSeek で動く。
- **AC-28**: `project_overview.md`「決定済みの方針」の LLM の項が、既定の LLM が Claude（claude-opus-5-5）で、DeepSeek も選べることを述べている。DeepSeek と Claude の API を、SDK を使わず標準ライブラリで呼ぶことの記述は残っている。
- **AC-29**: `project_overview.md`「前提・制約」のモデル名の方針に、既定のモデル名 `claude-opus-5-5` をコードに持つことが、例外として範囲と理由とともに書かれている。範囲は、プロバイダが `claude` でモデルの変数が未設定のときの既定に限ること。理由は、0008 の評価で本番の構成として決めたモデルであること。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。設定の読み込みの処理量は変わらない。
-   費用は変わる。何も設定しない実行の推定費用は、記事 1 本あたり約 0.36 USD である（0008 評価記録 §6.3 の 7 番目の項目・§6.4 の B3）。すべての段階を DeepSeek で行う、本書の変更の前の既定の構成は測っていない。測った中で最も近いのは、deepseek-flash で生成し検証・推敲を行わない B1g で、約 0.018 USD である（0008 評価記録 §6.4）。本書は費用の上限を定めない。

### 4.2. セキュリティ (Security)

`docs/dev/security.md` を適用する。本タスクは、API キーを扱い、LLM の API にデータを送るので、条件付きのガイドの対象である。

-   **送り先の事業者:** プロバイダの変数をどれも設定しない実行の送り先は、DeepSeek から Anthropic に変わる。送るデータ（字幕・概要欄・動画タイトル・チャンネル名・生成した文面・プロンプト）は変わらない。Anthropic に送るデータの扱いは `security.md` §4 にすでに書かれているので、本書は `security.md` を変えない。
-   **利用者の選んでいない送り先:** プロバイダを設定せずに DeepSeek を使っていた利用者が、別の用途で `ANTHROPIC_API_KEY` を設定していると、モデル名とデータが、利用者の選んでいない Anthropic へ、別の用途の API キーで送られうる。F-004 は、モデルの変数を設定した設定を通信の前に拒否して、これを防ぐ。モデルの変数もプロバイダの変数も設定していない設定は、今まで `YT2COLUMN_MODEL` の未設定で拒否されていたので、本書の変更の前に動いていた設定ではない。
-   **新しく読む変数:** プロバイダの変数をどれも設定しない実行では、`ANTHROPIC_API_KEY`・`YT2COLUMN_CLAUDE_EFFORT`・`ANTHROPIC_WORKSPACE_ID` を読み、`DEEPSEEK_API_KEY` を読まなくなる。選んだプロバイダの変数だけを読んで検査し、ほかのプロバイダの変数は読まないという規則（`project_overview.md`「設定（環境変数）」）は変えない（AC-01・AC-23）。
-   **エラーに値を含めない:** 拒否のエラーは、変数の名前と固定の理由だけを持ち、値を含めない（`internal/config/config.go:59-73` の固定の理由、`:87-105` の `VarError`）。F-004 の拒否も、この規則を保つ（AC-19・AC-21）。
-   **信頼できない入力:** 本書が新しく受け付ける信頼できない入力はない。環境変数は利用者が与える設定である。受理する値の形（`deepseek`・`claude`、空でない任意のモデル名）、拒否の番兵（`ErrMissing`・`ErrInvalid`）、前後の空白の除去などの補正をしない規則は変えない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   設定の拒否は、今までどおり、LLM の API の呼び出しの前に起こる（§3.1）。
-   #171 を前提にする（§5）ので、何も設定しない実行が effort の未設定で拒否されることも、`max_tokens` 16,000 の打ち切りに当たりやすいまま既定の構成になることもない。

### 4.4. 互換性 (Compatibility)

-   **プロバイダを設定せずに DeepSeek を使っていた利用者:** 今まで動いていた設定は、`YT2COLUMN_MODEL`（検証・推敲だけを実行する実行では、`YT2COLUMN_MODEL` または `YT2COLUMN_REVIEW_MODEL`）を必ず含むので、変更の後は、F-004 の拒否（`YT2COLUMN_LLM_PROVIDER` の `ErrMissing`）で通信の前に止まる。`YT2COLUMN_LLM_PROVIDER=deepseek` を設定すれば今までどおり動く（AC-23・AC-27）。
-   **プロバイダを設定せず、モデルだけを設定して Claude を使う設定:** 今までは成り立たなかった（プロバイダが `deepseek` になる）ので、互換性の問題はない。変更の後も、F-004 で拒否する。
-   **プロバイダとモデルを設定していた利用者:** 振る舞いは変わらない。ただし、`YT2COLUMN_REVIEW_PROVIDER=claude` を設定して `YT2COLUMN_REVIEW_MODEL` を設定していなかった設定は、今まで拒否されていたが、`claude-opus-5-5` で動くようになる（AC-10）。

### 4.5. 保守性 (Maintainability)

-   既定のモデル名は、本番の構成を見直したときに変える値である。値を変えるときに直す箇所が、コードの 1 か所と、それを述べる文書（README・`project_overview.md`）に限られるようにする（[design_handoff.md](design_handoff.md) H-02）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。ただし、次の 2 つは本書で方針を改める。
    -   **「LLM: 初期実装は DeepSeek」**（`project_overview.md:12`）: 既定の LLM を Claude（claude-opus-5-5）にし、DeepSeek は選べるプロバイダとして残す。理由: 0008 の評価で、本番の構成を claude-opus-5-5 に決めた（0008 評価記録 §6.1・§6.4）。
    -   **「モデル名は頻繁に更新される。コードにハードコードせず設定で与える」**（`project_overview.md:67`）: 例外として、既定のモデル名 `claude-opus-5-5` をコードに持つ。範囲は、プロバイダが `claude` で、モデルの変数（`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_MODEL`）が未設定のときの既定に限る。設定したモデル名は、今までどおりそのまま使う。理由: 何も設定しなければ本番の構成で動くことが issue の目的であり、モデル名の既定がなければその目的を満たせない。本番の構成が変われば、既定も変える（§4.5）。
-   **既定を変える実装は #171 の後にマージする:** 本タスクの実装のうち、既定を変える変更（F-001〜F-004）と、それを述べる文書の変更（F-006）は、#171 の実装（effort の既定 `high`・`max_tokens` の上限の緩和）が main に入った後にマージする。どちらか一方が欠けた状態で既定が変わると、§1「要件定義の際に決めたこと」の「#171 を前提にする」の 2 つの問題が起こるからである。振る舞いを変えない準備（テストの環境で DeepSeek を明示する変更と、その検査）は #171 に依存しないので、#171 より先にマージしてよい。設計・実装計画と、既定を変える変更の実装の作業は、#171 と並行して進めてよい。
-   **#488 との順序:** 本 issue は #488 をブロックするので、#488（タスク 0040。要件定義書は PR [#489](https://github.com/isseis/yt2column/pull/489)）の実装は本タスクの実装の後にマージし、本タスクが直した README と `project_overview.md` の記述に合わせる。特に、`project_overview.md`「決定済みの方針」の LLM の項（AC-28）は、両方のタスクが直す。
-   CLI と localhost サーバの設定は、`internal/config` の同じ関数で読む（`CLAUDE.md`「Separation of Concerns」）。既定の当てはめと F-004 の拒否も、2 つの入口で共有する。
-   既定を当てはめるかと F-004 の拒否は、変数が存在するかどうかとプロバイダの型（`config.Provider`）で決め、モデル名などの文字列の中身で決めない（`CLAUDE.md`「Declare, don't infer」）。

## 6. 用語集 (Glossary)

用語は `docs/translation_glossary.md` と統一する。

-   **本番の構成:** 0008 の評価で決めた、生成・字数の調整・検証・推敲をすべて claude-opus-5-5（effort `high`）で行う構成（0008 評価記録 §6.1）。
-   **既定:** 環境変数が未設定のときに使う値。
-   **未設定・空:** §3.1 の定義による。未設定は変数が存在しないこと、空は変数が存在して値が空であること。
-   **実行の種類（生成を実行する実行・検証・推敲だけを実行する実行・LLM を呼ばない実行）:** §3.1 の定義による。
-   **既定で決まるプロバイダ:** §3.1 の定義による。
-   **検証・推敲を実行する実行:** §3.1 の定義による。検証・推敲の設定を読む実行であり、`--no-review` で検証・推敲を行わない実行も含む。
-   **プロバイダ（provider）:** LLM の提供元。本書では `deepseek` と `claude`。
