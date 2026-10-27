# 要件定義書：LLM のプロバイダとモデルを 1 つの環境変数 `<プロバイダ>/<モデル ID>` で指定する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-27 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、生成と検証・推敲が使う LLM のプロバイダとモデルを、段階ごとに 1 つの環境変数で `<プロバイダ>/<モデル ID>` の形で指定するための要件を定義する（[#512](https://github.com/isseis/yt2column/issues/512)）。

**背景:**

現在は、プロバイダとモデルを段階ごとに別々の環境変数で設定する（`internal/config/config.go:32-35`）。

- 生成（字数の調整を含む）: `YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`
- 検証・推敲: `YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL`

プロバイダとモデルを別々に設定できるので、組み合わせの規則が複雑になっている。

- モデルの変数だけを設定してプロバイダの変数を設定しないと、拒否する（`config.go:465-479` の `modelWithoutProvider`）。拒否しないと、利用者が選んでいない既定のプロバイダへデータが送られるためである。
- `YT2COLUMN_REVIEW_PROVIDER` だけを設定すると、検証・推敲のモデルは生成のモデルではなく、そのプロバイダの既定のモデルになる（`config.go:578-590`）。
- 検証・推敲だけを実行する再開の実行では、生成側の変数を必須とせずに読み、検証・推敲がそれを受け継ぐ（`config.go:557-569`・`config.go:591-607`）。

本タスクでは、1 つの値でプロバイダとモデルの組を指定する。こうすると、「モデルだけが設定されていて、プロバイダは既定の値になる」状態は構文の上で起こらなくなる。

対応 issue: [#512](https://github.com/isseis/yt2column/issues/512)（前提: タスク 0040 本番プロファイル文書。完了済み）

**本書の記述範囲:** 本書は、観測できる振る舞いを定める。対象は、どの値を受理し、どの値を拒否するか、拒否したときに利用者に何を示すか、どの段階がどのプロバイダとモデルを使うか、文書に何を書くかである。拒否を分類する番兵エラー、拒否の理由の正確な文言、検査をどの関数のどの段階に置くか、既存の仕組み（`settingOrigin` など）をどう整理するか、テストの手段は、設計（`02_architecture.md`）で決める。要件の検討で挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #512 で決めたこと:**

-   **値の構文:** 値を最初の `/` で区切り、前半をプロバイダ、残りをモデル ID とする。モデル ID の中の `/` はそのまま保持する。将来、中継するプロバイダ（OpenRouter など）を追加しても、`openrouter/anthropic/claude-sonnet-5-5` のように構文を変えずに表せるようにするためである。
-   **モデル名の変換をしない:** モデル ID は、プロバイダの API が受け付ける ID をそのまま書く。プロバイダごとの別名や略称の変換表は持たない。
-   **厳格に検証し、補正しない:** プロバイダの部分は、受理する値（`deepseek`・`claude`）に完全に一致しなければならない。モデル ID が空の値は拒否する（`claude` のように `/` を含まない値、`claude/` も拒否する）。前後の空白は取り除かない。CLAUDE.md の「Reject, don't normalize」に従う。
-   **拒否の理由に値を含めない:** 現行の `VarError` の方針（`config.go:106-113`）を保つ。
-   **既定値と継承:** 生成の変数が未設定なら、現行の既定（`claude` の `claude-opus-5-5`。`config.go:59`・`config.go:62`）を使う。検証・推敲の変数が未設定なら、生成の組をそのまま使う。
-   **モデルだけを設定した場合の拒否をなくす:** その状態は構文の上で起こらないので、関連する拒否の規則（`reasonModelWithoutProvider`。`config.go:86-91`）と、その規則が要る分岐は不要になる。
-   **旧変数は拒否する:** 旧変数のいずれかが設定されていれば、起動時にエラーとし、新しい変数名を示す。黙って無視すると、たとえば古い `.envrc` で DeepSeek を指定している利用者のデータが、気づかないうちに既定の Claude へ送られるためである。
-   **互換層を設けない:** リポジトリの外に利用者はいないので、旧変数を新しい変数に読み替える仕組みは作らない。
-   **範囲外:** `YT2COLUMN_CLAUDE_EFFORT`・`ANTHROPIC_WORKSPACE_ID`・API キーは、プロバイダ固有の設定なので、別の変数のまま残す。新しい変数の値にオプションとして埋め込む形（`?effort=high` など）にはしない。OpenRouter など新しいプロバイダの追加も範囲外である（追加するときは、`internal/pricing` の料金表のキーを「プロバイダとモデル ID の組」にする必要がある）。

**要件定義の際に決めたこと（2026-10-27）:**

-   **変数名:** 生成は `YT2COLUMN_LLM`、検証・推敲は `YT2COLUMN_REVIEW_LLM` とする（issue の仮の名前のまま）。短く、検証・推敲の名前が生成の名前と対になるためである。
-   **空の値は拒否する:** `YT2COLUMN_LLM` と `YT2COLUMN_REVIEW_LLM` を空の値で設定した場合は、未設定と同じには扱わず、設定エラーにする。既定値を使うのは未設定のときだけである。全変数に共通する現行の方針（`docs/dev/project_overview.md`「設定（環境変数）」表の直後、README の Configuration）をそのまま当てはめ、例外を作らない。たとえば展開に失敗して空になった値で、利用者が選んでいない既定のプロバイダへデータが送られることを防ぐ。
-   **旧変数を拒否する範囲:** CLI とサーバは、実行する段階によらず、旧変数を拒否する。LLM の変数を読まない実行（`--after review`）や、検証・推敲だけを実行する実行でも拒否する。古い設定が残っていることに、いつ実行しても気づけるようにするためである。Native Messaging host は LLM の設定を読まない（`HOME` だけを読む。`config.go:318-342`）ので、対象にしない。
-   **空の旧変数も拒否する:** 旧変数が空の値で設定されていても拒否する。「設定されていれば拒否する」という単純な規則にでき、`.envrc` の消し忘れも見つかるためである。
-   **モデル ID の検証の範囲:** 設定で拒否するモデル ID は、空の値だけとする。前後に空白があるモデル ID や、UTF-8 として不正なモデル ID は、今までどおり LLM への通信の前に拒否する（現行はアダプタが拒否する。`internal/llm/claude/claude.go:57-65`・`internal/llm/deepseek/deepseek.go:57-65`）。文字の種類を絞る検証は加えない。範囲を issue の定めたものにとどめるためである。
-   **結合テストのモデルの変数の改名:** DeepSeek の結合テスト（`make test-integration-deepseek`・`test-integration-cli`・`test-integration-cli-slack`）のモデルを指定する変数を、`YT2COLUMN_MODEL`（`Makefile:128`・`internal/llm/deepseek/testutil/integration.go:26`）から `YT2COLUMN_TEST_DEEPSEEK_MODEL` に改名する。廃止して拒否する旧変数と同じ名前をテストだけが使い続けると紛らわしいためである。Claude の結合テストの `YT2COLUMN_TEST_CLAUDE_MODEL`（`Makefile:150`）とも名前の形が揃う。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   段階ごとに 1 つの変数で、プロバイダとモデルの組を指定できるようにする。
-   「モデルだけが設定されていて、プロバイダは既定の値になる」状態をなくし、設定の規則を単純にする。
-   旧変数が残っている利用者が、データの送り先が気づかないうちに変わる前に、設定の変更が必要なことに気づけるようにする。

### 2.2. スコープ (In Scope)

-   `YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM` の読み込み、検証、既定値、継承（CLI と localhost サーバ）
-   旧変数（`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL`）の拒否
-   README、`docs/dev/project_overview.md`、`docs/dev/production_profile.md`、`docs/dev/security.md` の環境変数の記載の更新と、それらを検査するガードテストの追従
-   DeepSeek の結合テストのモデルの変数の改名（Makefile と README の結合テストの説明を含む）

### 2.3. スコープ外 (Out of Scope)

-   `YT2COLUMN_CLAUDE_EFFORT`・`ANTHROPIC_WORKSPACE_ID`・API キーの変更（issue #512 で決めたこと）
-   新しいプロバイダ（OpenRouter・Gemini など）の追加と、それに伴う料金表のキーの変更（issue #512 で決めたこと）
-   旧変数を読み替える互換層（issue #512 で決めたこと）
-   記事の出力（`--out` のファイルの front matter、Slack の投稿）、サーバの起動時のログ、成果物ファイルに載るモデルの表記の変更。モデル ID だけを載せる現行の形（README「Output format」、`cmd/yt2column-server/run.go:185-189`）を保つ。issue は出力の変更を求めておらず、変えると成果物ファイルや既存の出力との互換を考える必要が出るためである。
-   完了したタスクの文書（`docs/tasks/` の下）に残る旧変数の記載の書き換え。当時の記録だからである。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

本書で「実行を止める」とは、設定エラーとして終了コード `2` で終わることをいう。CLI では、キャッシュディレクトリ、`yt-dlp`、LLM の API、Webhook のどれにも触れる前に止める（現行の設定エラーと同じ。README の終了コードの表・`cmd/yt2column/run.go:286-291`）。サーバでは、ソケットで待ち受ける前に止める（README のサーバの終了コードの表）。止めるときに示すメッセージは、拒否した変数の名前を含み、その変数の値を含まない。

#### F-001: 生成の LLM の指定（`YT2COLUMN_LLM`）

生成と字数の調整が使うプロバイダとモデルを、`YT2COLUMN_LLM` の値 `<プロバイダ>/<モデル ID>` で指定する。

**Acceptance Criteria**:
- **AC-01**: `YT2COLUMN_LLM` が未設定なら、生成と字数の調整は、プロバイダ `claude`・モデル `claude-opus-5-5` を使う。
- **AC-02**: `YT2COLUMN_LLM=deepseek/deepseek-flash` なら、生成と字数の調整はプロバイダ `deepseek`・モデル `deepseek-flash` を使う。`YT2COLUMN_LLM=claude/claude-sonnet-5-5` なら、プロバイダ `claude`・モデル `claude-sonnet-5-5` を使う。
- **AC-03**: 値は最初の `/` で区切る。モデル ID は、最初の `/` より後ろの文字列全体であり、その中の `/` を含めて、変換せずにそのまま API に送る。たとえば `YT2COLUMN_LLM=claude/vendor/model-x` なら、プロバイダは `claude`、API に送るモデルは `vendor/model-x` である。
- **AC-04**: `YT2COLUMN_LLM` が空の値で設定されていれば、既定値を使わずに実行を止める。メッセージは `YT2COLUMN_LLM` を名指しする。
- **AC-05**: 次の値は、実行を止める。メッセージは `YT2COLUMN_LLM` を名指しし、値を含まない。
  - `/` を含まない値（`claude`）
  - モデル ID が空の値（`claude/`、`deepseek/`）
  - プロバイダの部分が空の値（`/claude-opus-5-5`）
  - プロバイダの部分が `deepseek`・`claude` に完全一致しない値（`gemini/x`、`Claude/claude-opus-5-5`、` claude/claude-opus-5-5`、`claude /claude-opus-5-5`）
- **AC-06**: モデル ID の前後の空白は取り除かない。前後に空白があるモデル ID（`claude/claude-opus-5-5 `、`claude/ claude-opus-5-5`）は、LLM の API に何も送らずに実行を止める。

#### F-002: 検証・推敲の LLM の指定（`YT2COLUMN_REVIEW_LLM`）

検証・推敲が使うプロバイダとモデルを、`YT2COLUMN_REVIEW_LLM` で指定する。値の構文と検証は `YT2COLUMN_LLM` と同じである。

**Acceptance Criteria**:
- **AC-07**: `YT2COLUMN_REVIEW_LLM` が未設定なら、検証・推敲は生成と同じプロバイダとモデルを使う。`YT2COLUMN_LLM` も未設定なら、それは `claude` の `claude-opus-5-5` である。
- **AC-08**: `YT2COLUMN_REVIEW_LLM=deepseek/deepseek-flash` なら、検証・推敲は `deepseek` の `deepseek-flash` を使う。生成と字数の調整が使うプロバイダとモデルは変わらない。
- **AC-09**: `YT2COLUMN_REVIEW_LLM` に対して、AC-03 から AC-06 と同じ規則を当てはめる。メッセージが名指しする変数は `YT2COLUMN_REVIEW_LLM` である。空の値は、生成の組を受け継がずに実行を止める。
- **AC-10**: 検証・推敲だけを実行する再開の実行（`--after shorten`、または `--after generation --no-shorten`）では、次のとおりにする。
  - `YT2COLUMN_REVIEW_LLM` が設定されていれば、`YT2COLUMN_LLM` を読まない。`YT2COLUMN_LLM` が不正な値でも、そのために実行を止めない。
  - `YT2COLUMN_REVIEW_LLM` が未設定なら、`YT2COLUMN_LLM` の組（未設定なら既定の組）を使う。`YT2COLUMN_LLM` が不正な値なら、実行を止める。
- **AC-11**: `--no-review` を指定しても、`YT2COLUMN_REVIEW_LLM` を検証する（現行の振る舞い。README の「The review step's configuration is validated even with `--no-review`」）。LLM を呼ばない再開の実行（`--after review`）は、`YT2COLUMN_LLM` と `YT2COLUMN_REVIEW_LLM` のどちらも読まない（現行の振る舞い。`cmd/yt2column/run.go:490-491`）。

#### F-003: 旧変数の拒否

旧変数（`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL`）が設定されていれば、実行を止め、代わりに使う新しい変数を示す。

**Acceptance Criteria**:
- **AC-12**: 旧変数のいずれかが設定されていれば（空の値を含む）、CLI は実行を止める。指定したフラグや実行する段階（`--after review`、検証・推敲だけを実行する再開の実行、`--no-review` を含む）によらない。
- **AC-13**: 旧変数のいずれかが設定されていれば（空の値を含む）、localhost サーバは待ち受ける前に実行を止める。
- **AC-14**: メッセージは、設定されていた旧変数を名指しし、代わりに使う新しい変数を示す。`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL` には `YT2COLUMN_LLM` を示し、`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL` には `YT2COLUMN_REVIEW_LLM` を示す。メッセージは旧変数の値を含まない。
- **AC-15**: 旧変数が 2 つ以上設定されていれば、設定されていたすべての旧変数を、1 回の実行で、ほかの設定エラー（`ANTHROPIC_API_KEY` の未設定など）と合わせて報告する。
- **AC-16**: 旧変数の値は、使うプロバイダとモデルの決定に一切使わない。たとえば、旧変数と `YT2COLUMN_LLM` の両方が設定されていても、旧変数の値に従って動くことはなく、実行を止める。
- **AC-17**: Native Messaging host（`yt2column-nmhost` の中継、`install`、`uninstall`）は、旧変数が設定されていても、そのために止まらない。

#### F-004: 既存の規則の維持

プロバイダとモデルの指定の形を変えても、プロバイダ固有の変数とエラーの報告については現行の振る舞いを保つ。

**Acceptance Criteria**:
- **AC-18**: プロバイダ固有の変数（`DEEPSEEK_API_KEY`・`ANTHROPIC_API_KEY`・`YT2COLUMN_CLAUDE_EFFORT`・`ANTHROPIC_WORKSPACE_ID`）は、実行する段階が使うプロバイダのものだけを読んで検査する。ほかのプロバイダの変数は、未設定・空・不正な値のいずれでも拒否しない（現行の振る舞い。`config.go:612-638`）。
- **AC-19**: `YT2COLUMN_LLM` が不正な値で、`YT2COLUMN_REVIEW_LLM` が未設定なら、どのプロバイダ固有の変数も検査しない。`YT2COLUMN_LLM` を名指しするエラーは報告する（現行の `YT2COLUMN_LLM_PROVIDER` の振る舞いに対応する。README の Configuration）。
- **AC-20**: `YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM` のエラーは、1 回の実行で、ほかの設定エラーと合わせて報告する（現行の `Load` は拒否をまとめて返す。`config.go:344-351`）。
- **AC-21**: LLM の変数が `ANTHROPIC_API_KEY` だけなら、生成・字数の調整・検証・推敲のすべてを、`claude` の `claude-opus-5-5`、effort `high` で行う（現行の振る舞い。`docs/dev/project_overview.md`「設定（環境変数）」）。

#### F-005: 文書の更新

**Acceptance Criteria**:
- **AC-22**: README の Configuration の表、`docs/dev/project_overview.md` の「設定（環境変数）」の表、`docs/dev/production_profile.md` の環境変数の例は、`YT2COLUMN_LLM` と `YT2COLUMN_REVIEW_LLM` を載せ、旧変数を現行の設定として載せない。
- **AC-23**: README の Configuration と `docs/dev/project_overview.md` は、値の構文（最初の `/` で区切ること、モデル ID の中の `/` を保持すること、前後の空白を取り除かないこと）、未設定のときの既定値と継承、空の値を拒否すること、旧変数を拒否することを説明する。
- **AC-24**: README は、旧変数を使っていた利用者が新しい変数に移る方法を示す。少なくとも、生成のプロバイダとモデル（`YT2COLUMN_LLM_PROVIDER=deepseek` と `YT2COLUMN_MODEL=deepseek-flash`）を `YT2COLUMN_LLM=deepseek/deepseek-flash` に書き換える例を示す。あわせて、`YT2COLUMN_REVIEW_PROVIDER` だけを設定していた場合（そのプロバイダの既定のモデルを使っていた場合）は、新しい変数にモデル ID も書く必要があることを示す。
- **AC-25**: README のトラブルシューティングは、旧変数を名指しする設定エラーで CLI またはサーバが止まった場合（サーバなら、拡張からは「サーバが動いていない」としか見えない場合）の対処を示す。現行の `configuration: YT2COLUMN_LLM_PROVIDER` の項目は、この内容に置き換える。
- **AC-26**: README、`docs/dev/project_overview.md`、`docs/dev/production_profile.md`、`docs/dev/security.md` は、旧変数を現行の設定として説明しない（旧変数に触れるのは、拒否と移行の説明だけである）。
- **AC-27**: 文書の環境変数の名前と既定値を検査している既存のガードテスト（CLAUDE.md「Development Notes」の `project_overview.md` の注記、`production_profile.md` の設定の例、README の設定の例）は、新しい変数の名前と既定値を検査する。

#### F-006: DeepSeek の結合テストのモデルの変数

**Acceptance Criteria**:
- **AC-28**: `make test-integration-deepseek`・`make test-integration-cli`・`make test-integration-cli-slack` は、DeepSeek のモデルを `YT2COLUMN_TEST_DEEPSEEK_MODEL` から取る。この変数が未定義なら `deepseek-flash` を使う。空の値はそのまま渡し、テストは失敗する（現行の `YT2COLUMN_MODEL` の振る舞いと同じ。`Makefile:122-128`）。
- **AC-29**: 上の 3 つの make ターゲットと、それらが動かす結合テストは、`YT2COLUMN_MODEL` を読まない。
- **AC-30**: README と Makefile の結合テストの説明は、`YT2COLUMN_TEST_DEEPSEEK_MODEL` を載せ、`YT2COLUMN_MODEL` を結合テストの変数として載せない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A（環境変数を読む処理の変更であり、実行時間に関わる要件はない）。

### 4.2. セキュリティ (Security)

本タスクは、API キーの変数（`DEEPSEEK_API_KEY`・`ANTHROPIC_API_KEY`）を読む処理と、字幕などを LLM の API へ送る送り先を決める処理を変える。そのため `docs/dev/security.md` を当てはめる。

-   **送り先を利用者が選んだものに限る:** 設定の誤り（空の値、構文の誤り、旧変数の残り）を既定のプロバイダで補わない。誤りがあれば、LLM の API に何も送らずに実行を止める（AC-04・AC-05・AC-09・AC-12・AC-13・AC-16）。security.md §4 が述べるとおり、検証・推敲のプロバイダを生成と別にすると送り先の事業者が増える。この性質は変わらない。
-   **値をメッセージに出さない:** 拒否のメッセージは、変数の名前と固定の理由だけからなり、値を含まない（AC-05・AC-14）。秘密を誤って別の変数に設定した場合でも、エラーの出力から漏れない（`config.go:106-113` の方針）。
-   **秘密の扱いは変えない:** API キーの読み込み、`secret.Secret` による保持、出力の伏せ字（`internal/redact`）は変えない。
-   本タスクの入力は利用者が設定する環境変数であり、信頼できない入力（ネットワークの応答、外部コマンドの出力、それらから作ったファイル）を新たに読まない。要件定義の手順の「信頼できない入力の境界」は当てはまらない。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   設定の誤りは、実行を始める前（サーバは待ち受ける前）に、すべてまとめて報告する（AC-15・AC-20）。

### 4.4. 互換性 (Compatibility)
-   旧変数を設定している環境では、本タスクの後、CLI もサーバも起動時に止まる（F-003）。リポジトリの外に利用者はいないので、互換層は設けない（issue #512 で決めたこと）。移行の方法は README に示す（AC-24）。
-   `YT2COLUMN_REVIEW_PROVIDER` だけを設定して、そのプロバイダの既定のモデルを使う指定は、新しい変数では表せない。モデル ID も書く必要がある（AC-24）。
-   記事の出力、サーバのログ、成果物ファイルに載るモデルの表記は変えない（2.3）。

### 4.5. 保守性 (Maintainability)
-   受理するプロバイダの名前は 1 か所で定め、`YT2COLUMN_LLM` と `YT2COLUMN_REVIEW_LLM` で共有する（現行の `parseProvider` が 2 つのプロバイダの変数で共有されているのと同じ。`config.go:505-522`）。
-   既定のモデル名 `claude-opus-5-5` をコードに持つことは、`docs/dev/project_overview.md`「前提・制約」の「モデル名をハードコードしない」方針の例外として既に認められている。本タスクでは、この例外の範囲を「`YT2COLUMN_LLM` が未設定のとき（`YT2COLUMN_REVIEW_LLM` が受け継ぐ場合を含む）」に言い換える。プロバイダごとの既定のモデルはなくなるので、範囲は今より狭くなる。新しい例外は作らない。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   空の値はエラーとし、未設定と同じには扱わない（`docs/dev/project_overview.md`「設定（環境変数）」）。本タスクも例外を作らない。
-   標準ライブラリの外のモジュールを加えない（CLAUDE.md「Dependencies」）。
-   拒否のエラーは `*VarError` の形を保ち、値を持たない（`config.go:106-123`）。

## 6. 用語集 (Glossary)

用語は `docs/translation_glossary.md` と統一する。本書に固有の用語は次のとおり。

-   **旧変数:** 本タスクで廃止する 4 つの変数、`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL`。
-   **組:** プロバイダとモデル ID の組み合わせ。`<プロバイダ>/<モデル ID>` の値が表すもの。
-   **モデル ID:** プロバイダの API が受け付けるモデルの識別子。値の最初の `/` より後ろの文字列全体。
-   **検証・推敲だけを実行する再開の実行:** `--after shorten`、または `--after generation --no-shorten` で再開し、LLM を呼ぶ段階が検証・推敲だけになる実行。
