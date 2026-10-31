# 要件定義書：`YT2COLUMN_LLM` に effort を含め、`YT2COLUMN_CLAUDE_EFFORT` を廃止する

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-29 |
| Review date | 2026-10-29 |
| Reviewer | isseis |
| Comments | 2026-10-31（決定の変更。`/runplan` の PR-2 のコードレビューで判明し、この日のセッションでレビュアーが承認した）: `YT2COLUMN_CLAUDE_EFFORT` は `claude` の工程にだけ効いていたので、拒否のメッセージと移行の説明は、`+<effort>` を付ける先を「プロバイダが `claude` の LLM の変数の値」に限ると改めた。区別せずに示すと、生成が `deepseek` の利用者が `deepseek` の値にも effort を付け、それまで送っていなかった `reasoning_effort` を送るか、値が拒否されるためである。1 章「要件定義の際に決めたこと」の該当の項目、AC-24、AC-35 を直した。 |

## 1. 概要 (Overview)

**目的:** 本書は、LLM の effort を `YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM` の値に `+` で区切って含め、工程ごとに指定できるようにし、プロバイダ専用の変数 `YT2COLUMN_CLAUDE_EFFORT` を廃止するための要件を定義する。あわせて、DeepSeek でも effort を指定できるようにする。

**背景:**

現在、Claude の effort は `YT2COLUMN_CLAUDE_EFFORT` の 1 つの変数で指定する（`internal/config/config.go:36`・`config.go:598-617`）。未設定なら `high` を使う（`config.go:596`）。DeepSeek には effort を指定する手段がなく、要求に thinking の指定を含めない（`internal/llm/deepseek/request.go:20-28`、タスク 0003 の要件定義書 2.3）。この方式には次の 2 つの問題がある。

-   **プロバイダを足すたびに変数が増える。** OpenAI API のプロバイダの追加を検討している。その reasoning effort にも同じ方式を取ると、`YT2COLUMN_OPENAI_EFFORT` のような変数が増える。受理する effort の値も、プロバイダごとに異なる。
-   **effort を工程ごとに選べない。** effort は設定に 1 つしかなく（`config.go:170`）、生成と検証・推敲のクライアントのどちらにも同じ値を渡す（`internal/llm/provider/provider.go:35`・`provider.go:42`・`provider.go:75`）。字数の調整は生成のクライアントを使う（`config.go:393-396`）。そのため、3 つの工程の effort は常に同じになる。たとえば、検証・推敲だけ effort を上げることはできない。

プロバイダとモデルは、すでに工程ごとに 1 つの値 `<プロバイダ>/<モデル ID>` で指定している（`YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM`。タスク 0045、`config.go:509-530`）。effort もこの値に含めれば、上の 2 つの問題が解消する。

対応 issue: [#566](https://github.com/isseis/yt2column/issues/566)（前提: タスク 0045 `docs/tasks/0045_llm_env_var/`。完了済み）

**README の参照の仕方:** README は、タスク 0048 で日本語の正本（`README.md`）と英語版（`README.en.md`）に分かれ、見出しと文が日本語になる（`docs/tasks/0048_doc_language_policy/01_requirements.md`）。そのため本書は、README の節を、英語の見出しではなく役割で呼ぶ（設定の節、推奨構成の節、旧変数からの移行の節、トラブルシューティングの節）。行番号は 2026-10-29 の `main`（README の開発者向けの内容を `docs/dev/guides/cli.md`・`development.md` に移した後）のものである。

**本書の記述範囲:** 本書は、観測できる振る舞いを定める。対象は、どの値を受理し、どの値を拒否するか、拒否したときに利用者に何を示すか、どの工程がどのプロバイダ・モデル・effort を使い、プロバイダに何を送るか、文書に何を書くかである。拒否を分類する番兵エラー、エラーの理由（`VarError` の `Reason`）の正確な文言、effort を設定のどの型に持たせるか、受理する値の一覧をどのパッケージに置くか、既存の仕組み（`ClaudeEffort` のアクセサ、旧変数の一覧など）をどう整理するか、テストの手段は、設計（`02_architecture.md`）で決める。要件の検討で挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。以下で使う「プロバイダの部分」「後半」「モデル ID」「effort の部分」「設定エラー」は、3.1 で定義する。

**issue #566 で決めたこと:**

-   **値の形:** `YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM` の値の後ろに、`+` で区切って effort を付けられるようにする（`claude/claude-opus-5-5+high`）。プロバイダとモデルを工程ごとに 1 つの値で指定する今の方式に、effort も載せるためである。
-   **区切りの記号は `+`:** 条件は、モデル ID に現れないこと、シェル（`.envrc`）で引用符なしに書いても意味が変わらないこと、dotenv の書式（`.env`）で壊れないことの 3 つである。`:` は OpenAI のファインチューニング済みモデルの ID（`ft:gpt-4o-mini:org::abc123`）と衝突し、`@` は Vertex のモデル ID と衝突する。`,` は 2 つのモデルの並びに見えやすく、`#` は dotenv の実装によってはコメントになる。`!`・`^`・`?`・`*`・`[`・`;`・`&`・`$` などは、シェルで展開されるか、メタ文字として扱われる。`+` は実在のモデル ID と衝突せず、semver のビルドメタデータ（`1.0.0+build`）と同じく「付加情報を後ろに付ける」意味で読める。
-   **effort を省いた場合:** そのプロバイダの既定を使う。Claude の既定は `high` であり、今の `YT2COLUMN_CLAUDE_EFFORT` が未設定のときと同じである。
-   **検証・推敲の継承:** `YT2COLUMN_REVIEW_LLM` が未設定なら、検証・推敲は生成のプロバイダ・モデル・effort をそのまま受け継ぐ。
-   **`YT2COLUMN_CLAUDE_EFFORT` の廃止:** 設定されていれば（空の値を含む）、既存の旧変数（`YT2COLUMN_LLM_PROVIDER` など）と同じく、その変数を名指しする設定エラーで止める。
-   **範囲外:** OpenAI API のプロバイダの追加（別の issue で扱う。本タスクは、そのアダプタが effort の値の一覧を足すだけで載せられるようにする前段である）、本番の構成の値（`claude/claude-opus-5-5`、effort `high`）の変更、effort 以外のパラメータ（出力の上限など）を値に含めること。

**要件定義の際に決めたこと（2026-10-29）:**

issue は「決めること（要件定義で確定する）」として 4 つの案を挙げていた。そのうち 3 つは案のとおりに採用し、DeepSeek の扱いは案を変えた。

-   **文法（issue の案のとおり）:** 最初の `/` より後ろに `+` が 1 つあれば、その後ろを effort とする。`+` が 2 つ以上あれば設定エラーにする。`+` の後ろが effort の値として不正なら拒否し、モデル ID の一部とみなして受理することはしない（CLAUDE.md の「Reject, don't normalize」）。effort の部分が空の値（`claude/claude-opus-5-5+`）も拒否する。
-   **字数の調整（issue の案のとおり）:** 今と同じく、生成と同じクライアント（同じ effort）を使う。
-   **移行（issue の案のとおり）:** README の旧変数からの移行の節（今の見出しは「Migrating from the retired LLM variables」）に、`YT2COLUMN_CLAUDE_EFFORT` から移る手順を足す。
-   **DeepSeek も effort を受理する（issue の案を変更）:** issue の案は、DeepSeek に effort を付けた値を設定エラーにするものだった。レビュアーの判断で、DeepSeek でも effort を指定できるようにする。DeepSeek の Chat Completions API は、要求本文の `reasoning_effort` で thinking の有無と強さを指定でき、受け付ける値は `none`・`low`・`high`・`max` の 4 つ、既定は `high` である（`none` は thinking を無効にし、ほかの 3 つは有効にする）。2026-10-29 に DeepSeek の公式文書（[Chat Completions API](https://api-docs.deepseek.com/api/create-chat-completion/)・[Thinking Mode](https://api-docs.deepseek.com/guides/thinking_mode/)）で確かめた。プロジェクトの概要も、DeepSeek の thinking を「初期は API のデフォルトのままにし、必要に応じて設定可能にする」としている（`docs/dev/project_overview.md:88`）。
    -   **受理する値は API の仕様の 4 つに限る:** Thinking Mode のガイドは、ほかの値（`minimal`・`medium`・`xhigh`・`ultra`）を API の側で 3 つのどれかに読み替えると記している。これらを受理すると、`deepseek/deepseek-flash+medium` が黙って `high` として動く。指定と異なる振る舞いを隠さないよう、仕様の値以外は拒否する（「Reject, don't normalize」）。
    -   **effort を省いたら何も送らない:** effort を省いた DeepSeek の値では、今と同じ要求を送り、thinking の扱いを API の既定に任せる。今の DeepSeek の設定が、そのまま同じ要求を送り続けるためである。
-   **推奨構成の例には effort まで書く:** 本番の構成を表す環境変数の例（`docs/dev/production_profile.md`「本番の構成」、README の推奨構成の節）は、`export YT2COLUMN_LLM=claude/claude-opus-5-5+high` の 1 行にする。今の例は `YT2COLUMN_LLM` と `YT2COLUMN_CLAUDE_EFFORT` の 2 行である（`docs/dev/production_profile.md:33-36`・`README.md:575-578`）。README は、この例を export すれば、後で既定が変わっても構成が固定されると説明している（`README.md:586-587`）。effort を省くと effort だけが既定に任されて固定されなくなるので、effort まで書く。
-   **`YT2COLUMN_CLAUDE_EFFORT` を拒否するメッセージには、2 つの変数と既定を示す:** メッセージは、代わりに `YT2COLUMN_LLM` と `YT2COLUMN_REVIEW_LLM` のうちプロバイダが `claude` の値に `+<effort>` を付けることと、effort を省けば Claude の既定の `high` を使うことを示す。旧変数は検証・推敲の Claude の工程にも効いていたので、`YT2COLUMN_REVIEW_LLM` に `claude` を設定している利用者は、その値にも effort を付ける必要がある。たとえば、`low` を指定していた利用者が、旧変数を外しただけで気づかないまま `high` に戻ってしまうことを防ぐためである。今の旧変数のメッセージも、代わりの変数と、それが未設定のときに使われるものを示している（`config.go:92-100`）。メッセージと移行の説明は、`+<effort>` を付ける先を、プロバイダが `claude` の値に限って示す（2026-10-31 のレビュアーの判断。旧変数は `deepseek` の工程には効いておらず、`deepseek` の値に付けると、それまで送っていなかった `reasoning_effort` を送るためである）。
-   **effort の部分が不正なときは、受理する値の一覧を示す:** メッセージは、そのプロバイダが受理する effort の値を示す。今の `YT2COLUMN_CLAUDE_EFFORT` の理由は「must be a supported effort value」だけで、値の一覧を示さない（`config.go:84`）。effort はプロバイダとモデルを合わせた 1 つの値の一部になり、受理する値がプロバイダごとに異なるので、一覧を示す。一覧は固定の文であり、利用者が設定した値を含まない。
-   **effort の表示はスコープ外とする:** サーバの起動時のログと記事に載るモデルの表記は変えず、effort を加えない（2.3）。

**先行タスクの決定の変更:**

-   **タスク 0045:** issue #512 の決定として、`YT2COLUMN_CLAUDE_EFFORT` をプロバイダ固有の設定として別の変数のまま残し、新しい変数の値にオプションとして埋め込まないと決めた（`docs/tasks/0045_llm_env_var/01_requirements.md` §1「issue #512 で決めたこと」の「範囲外」）。また、0045 の AC-18 は、`YT2COLUMN_CLAUDE_EFFORT` を、Claude を使う工程があるときだけ読むプロバイダ固有の変数に数えていた。本タスクは、issue #566 の 2 つの問題（プロバイダごとに変数が増えること、effort を工程ごとに選べないこと）を理由に、これらを変更する。`YT2COLUMN_CLAUDE_EFFORT` は、旧変数として、どの実行でも設定されているかどうかを調べる変数になる（5 章）。`ANTHROPIC_WORKSPACE_ID` と API キーは、0045 の決定のまま別の変数に残す（2.3）。
-   **タスク 0003:** thinking の切り替えをスコープ外とし、初期は API の既定のままにして、要求に thinking の指定を含めないと決めた（`docs/tasks/0003_deepseek_llm_client/01_requirements.md` 2.3）。本タスクは、利用者が DeepSeek の値に effort を付けたときに限り、`reasoning_effort` を送るように変更する（F-002）。effort を省いた値の要求は、今と同じである。`thinking` のメンバーは、今と同じく送らない。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   生成（字数の調整を含む）と検証・推敲で、effort を別々に指定できるようにする。
-   プロバイダを足しても、effort のための環境変数が増えないようにする。
-   Claude と DeepSeek の両方で、同じ形の値で effort を指定できるようにする。
-   `YT2COLUMN_CLAUDE_EFFORT` が残っていても、使われる effort が知らないうちに変わることがなく、利用者が設定を直す必要に気づけるようにする。

### 2.2. スコープ (In Scope)

-   `YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM` の値の effort の部分の解釈、検証、既定、継承（CLI と localhost サーバ）
-   effort を工程ごとに LLM のクライアントへ渡し、Claude と DeepSeek の要求に載せること
-   `YT2COLUMN_CLAUDE_EFFORT` を旧変数として拒否すること
-   README、`docs/dev/guides/cli.md`、`docs/dev/project_overview.md`、`docs/dev/production_profile.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/guides/claude_output_limit.md` の effort の記載の更新と、これらの文書の値を検査するガードテストの追従。`README.en.md` があれば、CLAUDE.md「Translation Guidelines」に従って `README.md` の変更を反映する

### 2.3. スコープ外 (Out of Scope)

-   OpenAI API のプロバイダの追加（issue #566 で決めたこと）
-   本番の構成の値（`claude/claude-opus-5-5`、effort `high`）の変更（issue #566 で決めたこと）
-   effort 以外のパラメータ（出力の上限など）を値に含めること（issue #566 で決めたこと）
-   `ANTHROPIC_WORKSPACE_ID`・API キーの変更。タスク 0045 の決定のまま、別の変数に残す。issue #566 は effort だけを対象にしているためである。
-   `YT2COLUMN_CLAUDE_EFFORT` を読み替える互換層。issue #566 は、廃止して設定エラーで止めると決めているためである。
-   DeepSeek の要求の `thinking` のメンバーを送ること。thinking の有無は `reasoning_effort` の `none` で指定でき、2 つのメンバーで同じことを指定する必要がないためである。
-   サーバの起動時のログ（`cmd/yt2column-server/run.go:186-191`）、記事の出力（`--out` のファイルの front matter、Slack の投稿）、成果物ファイルに載るモデルの表記の変更。モデル ID だけを載せる今の形を保ち、effort を加えない。issue は出力の変更を求めておらず、タスク 0045 も同じ理由で表記を変えなかったためである（1 章「要件定義の際に決めたこと」）。
-   Claude の結合テストが effort を取る変数 `YT2COLUMN_TEST_CLAUDE_EFFORT`（`docs/dev/guides/development.md` の「Claude の統合テスト」・`Makefile:166-178`）の変更。テスト専用の変数で、廃止する `YT2COLUMN_CLAUDE_EFFORT` とは名前が異なるためである。DeepSeek の結合テストに effort を指定する手段を加えることも、issue が求めていないので含めない。
-   Claude が受理する effort の値と既定、`max` の扱い（README の設定の節の注記）の変更。issue が求めておらず、変えるときは出力の上限の測り直しが要るためである（`docs/dev/guides/claude_output_limit.md`）。
-   完了したタスクの文書（`docs/tasks/` の下）に残る `YT2COLUMN_CLAUDE_EFFORT` の記載の書き換え。当時の記録だからである。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

本書の「実行を止める」、「生成の設定」、「検証・推敲の設定」の意味と、CLI がフラグの組み合わせに応じてどの設定を検査するかは、タスク 0045 の要件定義書の 3.1 の定義と表に従う（`docs/tasks/0045_llm_env_var/01_requirements.md` §3.1）。本書は、その表を変えない。「設定エラー」は、設定の読み込みで変数を拒否して実行を止めることをいう。サーバは、常に生成と検証・推敲の両方の設定を検査する（`internal/config/config.go:324`）。設定の読み込みで変数を拒否するときのメッセージは、拒否した変数の名前を含み、その変数の値を含まない（`config.go:132-139` の方針）。

本書では、`YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM` の値の、最初の `/` より前を「プロバイダの部分」、最初の `/` より後ろを「後半」と呼ぶ。後半に `+` があれば、`+` より前を「モデル ID」、`+` より後ろを「effort の部分」と呼ぶ。後半に `+` がなければ、後半の全体がモデル ID である。

プロバイダごとに受理する effort の値と、effort を省いたときの扱いは、次のとおりである。

| プロバイダ | 受理する effort の値 | effort を省いたとき |
|---|---|---|
| `claude` | `low`・`medium`・`high`・`xhigh`・`max`（今の `YT2COLUMN_CLAUDE_EFFORT` と同じ。`internal/llm/claudeparam/claudeparam.go:26-34`） | `high` を送る（今の既定と同じ。`config.go:596`） |
| `deepseek` | `none`・`low`・`high`・`max`（DeepSeek の Chat Completions API の `reasoning_effort` の値。1 章「要件定義の際に決めたこと」） | 何も送らない（今の要求と同じ） |

#### F-001: 値の effort の部分の解釈

`YT2COLUMN_LLM` と `YT2COLUMN_REVIEW_LLM` の値で、後半の `+` より後ろを effort として解釈する。2 つの変数で規則は同じである。

**Acceptance Criteria**:
- **AC-01**: 生成の設定を検査する実行で、`YT2COLUMN_LLM=claude/claude-sonnet-5-5+low` なら、生成と字数の調整は、プロバイダ `claude`・モデル `claude-sonnet-5-5`・effort `low` を使う。API に送るモデルは `claude-sonnet-5-5` であり、`+low` を含まない。
- **AC-02**: effort の部分は、3.1 の表がそのプロバイダについて挙げる値のどれでも受理し、その effort を使う。
- **AC-03**: 後半に `+` がなければ、後半の全体をモデル ID とし、effort は 3.1 の表の「effort を省いたとき」に従う。たとえば `YT2COLUMN_LLM=claude/claude-sonnet-5-5` なら、生成と字数の調整は `claude-sonnet-5-5` を effort `high` で使う。
- **AC-04**: `+` を探すのは後半だけである。モデル ID の中の `/` は、今と同じく、そのまま保持して API に送る。たとえば `YT2COLUMN_LLM=claude/vendor/model-x+medium` なら、モデルは `vendor/model-x`、effort は `medium` である。
- **AC-05**: 生成の設定を検査する実行で、`YT2COLUMN_LLM` が次の値なら、実行を止める。メッセージは `YT2COLUMN_LLM` を名指しし、値を含まない。
  - 後半に `+` が 2 つ以上ある値（`claude/claude-opus-5-5+high+low`、`claude/claude-opus-5-5++high`）
  - effort の部分が空の値（`claude/claude-opus-5-5+`、`deepseek/deepseek-flash+`）
  - effort の部分が、3.1 の表がそのプロバイダについて挙げる値に完全一致しない値（`claude/claude-opus-5-5+High`、`claude/claude-opus-5-5+ high`、`claude/claude-opus-5-5+high `、`claude/claude-opus-5-5+none`、`claude/model+x/y`、`deepseek/deepseek-flash+medium`、`deepseek/deepseek-flash+xhigh`、`deepseek/deepseek-flash+ultra`）。effort の部分の前後の空白は取り除かず、大文字・小文字も変換しない。
  - モデル ID が空の値（`claude/+high`）
- **AC-06**: AC-05 のメッセージは、拒否の理由に応じて次を示す。示す内容は固定の文であり、設定した値を含まない。
  - effort の部分が空か、受理する値に完全一致しない値: effort の部分が受理されないことと、そのプロバイダが受理する effort の値の一覧（3.1 の表）
  - 後半に `+` が 2 つ以上ある値: `+` が 1 つまでであること
  - モデル ID が空の値: 値の形が、`<プロバイダ>/<モデル ID>` の後ろに、省略できる `+<effort>` を付けたものであること。`/` を含まない値（`claude`）と、`+` を含まずモデル ID が空の値（`claude/`）のメッセージも、同じ形を示す
- **AC-07**: 1 つの値が AC-05 の複数の規則に当たるとき（`claude/+`、`claude/+ultra`、`deepseek/deepseek-flash+a+b`）も、実行を止め、その変数の拒否を 1 つだけ報告する。メッセージは、当たる規則のうち、どれか 1 つについて AC-06 の内容を示せばよい。
- **AC-08**: effort の部分が不正な値を、モデル ID の一部とみなして受理することはない。たとえば `YT2COLUMN_LLM=claude/claude-opus-5-5+ultra` を、モデル `claude-opus-5-5+ultra` として API に送ることはない。設定の読み込みが拒否した実行は、LLM の API に何も送らない。
- **AC-09**: モデル ID の前後の空白と、UTF-8 として不正なモデル ID の扱いは、今と同じである（タスク 0045 の AC-06）。モデル ID は `+` より前の部分なので、`claude/claude-opus-5-5 +high` のモデル ID は `claude-opus-5-5 `（後ろに空白がある）であり、その工程のクライアントを作るときに、LLM の API に何も送らずに実行を止める。
- **AC-10**: 検証・推敲の設定を検査する実行で、`YT2COLUMN_REVIEW_LLM` に対して、AC-01 から AC-09 と同じ規則を当てはめる。effort は検証・推敲の工程に使う。メッセージが名指しする変数は `YT2COLUMN_REVIEW_LLM` である。

#### F-002: プロバイダへの effort の送信

各工程のクライアントは、その工程の effort をプロバイダの要求に載せる。

**Acceptance Criteria**:
- **AC-11**: プロバイダが `claude` の工程は、その工程の effort（effort を省いた値なら `high`）を、今の `YT2COLUMN_CLAUDE_EFFORT` と同じく Claude の要求の effort として送る。
- **AC-12**: プロバイダが `deepseek` で effort を付けた工程（`deepseek/deepseek-flash+none`・`+low`・`+high`・`+max`）は、DeepSeek の要求本文の `reasoning_effort` に、その値を変換せずに入れて送る。要求本文は、ほかに `thinking` のメンバーを含まない。
- **AC-13**: プロバイダが `deepseek` で effort を省いた工程（`deepseek/deepseek-flash`）は、今と同じ要求を送る。要求本文は `reasoning_effort` と `thinking` のどちらも含まない（今の振る舞い。`internal/llm/deepseek/request.go:20-28`、タスク 0003 の AC-06）。

#### F-003: 工程ごとの effort と継承

生成（字数の調整を含む）と検証・推敲は、それぞれの変数の effort を使う。

**Acceptance Criteria**:
- **AC-14**: `YT2COLUMN_LLM=claude/claude-opus-5-5+low` と `YT2COLUMN_REVIEW_LLM=claude/claude-opus-5-5+xhigh` を設定すると、生成と字数の調整は effort `low` で、検証・推敲は effort `xhigh` で、Claude の API に要求を送る。`YT2COLUMN_LLM=deepseek/deepseek-flash+low` と `YT2COLUMN_REVIEW_LLM=claude/claude-opus-5-5+xhigh` のように、プロバイダが異なる場合も、それぞれの工程が自分の effort を送る。
- **AC-15**: `YT2COLUMN_REVIEW_LLM` が未設定なら、検証・推敲は、生成のプロバイダ・モデル・effort をそのまま使う。たとえば `YT2COLUMN_LLM=claude/claude-sonnet-5-5+medium` だけを設定すると、検証・推敲も `claude-sonnet-5-5` を effort `medium` で使う。生成の effort を省いた場合は、検証・推敲も 3.1 の表の「effort を省いたとき」に従う。
- **AC-16**: `YT2COLUMN_REVIEW_LLM` を設定し、その effort を省いた場合は、生成の effort を受け継がず、3.1 の表の「effort を省いたとき」に従う。たとえば `YT2COLUMN_LLM=claude/claude-opus-5-5+low` と `YT2COLUMN_REVIEW_LLM=claude/claude-opus-5-5` なら、検証・推敲の effort は `high` である。
- **AC-17**: 字数の調整は、生成と同じ effort を使う。
- **AC-18**: `YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM`・`YT2COLUMN_CLAUDE_EFFORT` がどれも未設定で、`ANTHROPIC_API_KEY` だけを設定すれば、生成・字数の調整・検証・推敲のすべてを、`claude` の `claude-opus-5-5`、effort `high` で行う（今の振る舞い。`docs/dev/project_overview.md:153`、タスク 0045 の AC-21）。
- **AC-19**: `YT2COLUMN_REVIEW_LLM` が未設定で、`YT2COLUMN_LLM` が AC-05 の値なら、生成と検証・推敲の両方の設定を検査する実行でも、`YT2COLUMN_LLM` の拒否は 1 回だけ報告する（今の振る舞い。`config.go:465-469`）。
- **AC-20**: 検証・推敲の設定を検査し、生成の設定を検査しない実行（`--after shorten` など。タスク 0045 の AC-10）で、`YT2COLUMN_REVIEW_LLM` が未設定なら、検証・推敲は `YT2COLUMN_LLM` の effort（effort を省いていれば 3.1 の表の「effort を省いたとき」）を使う。`YT2COLUMN_LLM` が AC-05 の値なら、`YT2COLUMN_LLM` を名指しして実行を止める。`YT2COLUMN_REVIEW_LLM` が設定されていれば、今と同じく `YT2COLUMN_LLM` を読まない。

#### F-004: `YT2COLUMN_CLAUDE_EFFORT` の拒否

`YT2COLUMN_CLAUDE_EFFORT` を、タスク 0045 の旧変数（`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL`）と同じく拒否する。

**Acceptance Criteria**:
- **AC-21**: `YT2COLUMN_CLAUDE_EFFORT` が設定されていれば（空の値を含む）、CLI は実行を止める。タスク 0045 の 3.1 の表のどの行の実行でも止める（`--after review`、`--no-review` を含む）。生成と検証・推敲のプロバイダがどちらも `deepseek` の実行でも止める。`-h` と使い方の誤りは、今と同じく設定を読む前に報告する。
- **AC-22**: `YT2COLUMN_CLAUDE_EFFORT` が設定されていれば（空の値を含む）、localhost サーバは待ち受ける前に実行を止める。
- **AC-23**: `YT2COLUMN_CLAUDE_EFFORT` が設定されている実行は、その値によらず、LLM の API に何も送らない。`YT2COLUMN_LLM` などのほかの変数がすべて正しくても同じである。
- **AC-24**: メッセージは `YT2COLUMN_CLAUDE_EFFORT` を名指しし、その値を含まない。メッセージは、ほかの旧変数のメッセージと同じく、この変数を外さなければならないこと（空の値も設定されているとみなすこと）を示す（`config.go:97-100`）。あわせて、代わりに `YT2COLUMN_LLM` と `YT2COLUMN_REVIEW_LLM` のうちプロバイダが `claude` の値に `+<effort>` を付けることと、effort を省けば Claude の既定の `high` を使うことを示す（1 章「要件定義の際に決めたこと」）。
- **AC-25**: `YT2COLUMN_CLAUDE_EFFORT` の拒否は、ほかの旧変数の拒否、ほかの設定の読み込みのエラーと合わせて、1 回の実行でまとめて報告する（タスク 0045 の AC-15・AC-20 と同じ）。
- **AC-26**: ホスト（`yt2column-nmhost` の中継、`install`、`uninstall`）は、`YT2COLUMN_CLAUDE_EFFORT` が設定されていても、そのために止まらない（タスク 0045 の AC-17 と同じ）。

#### F-005: 既存の規則の維持

effort を値に含めても、プロバイダ固有の変数とエラーの報告については今の振る舞いを保つ。

**Acceptance Criteria**:
- **AC-27**: プロバイダ固有の変数（`DEEPSEEK_API_KEY`・`ANTHROPIC_API_KEY`・`ANTHROPIC_WORKSPACE_ID`）は、その実行が設定を検査する工程が使うプロバイダのものだけを読んで検査する。ほかのプロバイダの変数は、未設定・空・不正な値のいずれでも拒否しない（今の振る舞い。`config.go:541-558`）。
- **AC-28**: プロバイダの部分が正しく、effort の部分またはモデル ID だけが不正な値（AC-05 の値）なら、プロバイダは決まるので、そのプロバイダの変数を検査する。たとえば `YT2COLUMN_LLM=claude/claude-opus-5-5+ultra` で `ANTHROPIC_API_KEY` が未設定なら、`YT2COLUMN_LLM` と `ANTHROPIC_API_KEY` の両方のエラーを報告する。今の、モデル ID だけが空の値（`claude/`）の扱いと同じである（タスク 0045 の AC-19）。プロバイダの部分が決まらない値の扱いは、今と同じである。
- **AC-29**: タスク 0045 が定めた、effort に関わらない規則（値を最初の `/` で区切ること、プロバイダの部分の完全一致、空の値の拒否、未設定のときの既定値、旧変数 4 つの拒否）は変えない。`+` を含まない値の振る舞いは、AC-06 が示す値の形の文を除いて、今と同じである。

#### F-006: 文書の更新

利用者と開発者が読む文書の effort の記載を、新しい規則に合わせる。`YT2COLUMN_CLAUDE_EFFORT` を現行の設定として載せたままにすると、文書のとおりに設定した利用者の実行が F-004 で止まるためである。`YT2COLUMN_CLAUDE_EFFORT` を使っていた利用者には、移行の方法を示す。

**Acceptance Criteria**:
- **AC-30**: README の設定の節の表と `docs/dev/project_overview.md` の「設定（環境変数）」の表は、`YT2COLUMN_CLAUDE_EFFORT` を現行の設定として載せない。`YT2COLUMN_LLM` の行は、値に `+<effort>` を付けられることを示す。
- **AC-31**: README の設定の節と `docs/dev/project_overview.md` の「設定（環境変数）」は、effort の部分の規則を説明する。説明は、`+` で区切ること、`+` を探すのは後半だけで、1 つまで許すこと、3.1 の表の内容（プロバイダごとに受理する値と、省いたときの扱い）、空の effort と不正な effort を拒否すること、`YT2COLUMN_REVIEW_LLM` が未設定なら effort も生成から受け継ぎ、設定して effort を省けば 3.1 の表に従うことを含む。
- **AC-32**: 生成と検証・推敲の継承を述べる既存の文（`README.md:540-544`、`docs/dev/guides/cli.md:273`、`docs/dev/production_profile.md:38`、`docs/dev/project_overview.md:151`）は、プロバイダとモデルに加えて effort も受け継ぐことがわかる形にする。プロバイダの部分が正しくモデル ID だけが空ならそのプロバイダの変数を検査する、と述べる文（`README.md:560-562`、`docs/dev/project_overview.md:153`）は、effort の部分が不正な場合も同じであることがわかる形にする（AC-28）。
- **AC-33**: 旧変数を列挙する文と、「実行に使わない変数は読まない」方針の例外を述べる文（`README.md:546-551`、`docs/dev/guides/cli.md:72-74`・`:276-278`、`docs/dev/project_overview.md:155-157`）は、`YT2COLUMN_CLAUDE_EFFORT` を旧変数に含める。`docs/dev/guides/cli.md` の `--no-review` の段落のうち effort を述べる文（`docs/dev/guides/cli.md:72`）と、README の `max` および既定の effort の注記（`README.md:564-571`）は、`YT2COLUMN_CLAUDE_EFFORT` ではなく値の effort の部分で説明する。
- **AC-34**: 本番の構成を表す環境変数の例（`docs/dev/production_profile.md`「本番の構成」、README の推奨構成の節）は、`export YT2COLUMN_LLM=claude/claude-opus-5-5+high` の 1 行にする。例の変数の数を述べる文（`docs/dev/production_profile.md` の「次の 2 つ」「上の 2 つの変数」と、README の推奨構成の節でそれに当たる文）は、例に合わせる。`docs/dev/production_profile.md` の「effort は 3 つの段階で共通である」という記述は、effort を工程ごとに指定できることと、本番の構成では 3 つの工程が同じ `high` であることがわかる形にする。
- **AC-35**: README の旧変数からの移行の節は、`YT2COLUMN_CLAUDE_EFFORT` から移る方法を示す。少なくとも、`YT2COLUMN_CLAUDE_EFFORT=low` と `YT2COLUMN_LLM=claude/claude-sonnet-5-5` を `YT2COLUMN_LLM=claude/claude-sonnet-5-5+low` に書き換える例と、`YT2COLUMN_REVIEW_LLM` に `claude` の値を設定していた場合は、その値にも effort を付ける必要があること（付けなければ既定の `high` になること）と、プロバイダが `deepseek` の値には付けないことを示す。
- **AC-36**: README のトラブルシューティングで旧変数を名指しする設定エラーを扱う項目（`README.md:446-457`）は、`YT2COLUMN_CLAUDE_EFFORT` を名指しする設定エラーも扱い、その対処（AC-35 の移行の方法）を示す。
- **AC-37**: `docs/dev/project_overview.md` の DeepSeek の thinking の記述（`docs/dev/project_overview.md:88`）は、effort を付けた値で `reasoning_effort` を指定でき、省けば API の既定のままであることがわかる形にする。
- **AC-38**: `docs/dev/conventions/package_reference.md` の `internal/config`・`internal/llm/provider`・`internal/llm/deepseek` の行（`docs/dev/conventions/package_reference.md:27`・`:37` など）は、effort が工程ごとの値から決まること、プロバイダごとに受理する値と省いたときの扱い（3.1 の表）、`YT2COLUMN_CLAUDE_EFFORT` が旧変数であることを述べる。`docs/dev/guides/claude_output_limit.md` の既定の effort の所在の記述（`docs/dev/guides/claude_output_limit.md:10`）は、Claude の既定の effort を定義している箇所を指す。
- **AC-39**: README、`docs/dev/guides/cli.md`、`docs/dev/project_overview.md`、`docs/dev/production_profile.md`、`docs/dev/conventions/package_reference.md`、`docs/dev/guides/claude_output_limit.md` は、`YT2COLUMN_CLAUDE_EFFORT` を現行の設定として説明しない（触れるのは、拒否と移行の説明だけである）。
- **AC-40**: 文書に書く値は、コードと一致する。`docs/dev/production_profile.md` の設定のブロックと工程の表、README の推奨構成の節の effort は、effort を省いたときに Claude の工程が使う effort と一致する。README と `docs/dev/project_overview.md` の環境変数の名前と既定値は、設定の読み込みが読む変数と既定値に一致する。README と `docs/dev/project_overview.md` が挙げる、プロバイダごとに受理する effort の値は、設定の読み込みが受理する値と一致する。
- **AC-41**: AC-06 のメッセージが示す effort の値の一覧は、設定の読み込みがそのプロバイダについて受理する値と一致する。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A（環境変数を読む処理と要求のパラメータの変更であり、実行時間に関わる要件はない）。DeepSeek で effort を指定したときの応答の速さは、利用者が選んだ値による。

### 4.2. セキュリティ (Security)

本タスクは、API キーの変数を読む処理と、字幕などを LLM のプロバイダへ送る要求の内容（effort）を決める処理を変える。そのため `docs/dev/security.md` を当てはめる。

-   **送る内容を利用者が選んだものに限る:** effort の部分の誤り（空、不正な値、`+` が 2 つ以上）を、既定の effort で補ったり、モデル ID の一部とみなしたりしない。誤りがあれば、LLM の API に何も送らずに実行を止める（AC-05・AC-08）。`YT2COLUMN_CLAUDE_EFFORT` が残っている場合も、その値を使わずに止める（AC-21〜AC-23）。
-   **要求に加えるのは受理した値だけ:** DeepSeek の要求に加える `reasoning_effort` は、3.1 の表の 4 つの値のどれかであり、利用者の文字列をそのほかの形で要求に入れない（AC-12）。effort を省いた値の要求は今と同じである（AC-13）。
-   **値をメッセージに出さない:** 設定の読み込みの拒否のメッセージは、変数の名前と固定の理由だけからなり、値を含まない（AC-05・AC-06・AC-24）。effort の値の一覧も固定の文である。秘密情報を誤って別の変数に設定した場合でも、エラーの出力から漏れない（`config.go:132-139` の方針）。
-   **送り先の事業者は変わらない:** effort はプロバイダとモデルを変えないので、字幕などの送り先は今と同じく、プロバイダの部分で決まる。security.md §4 が述べる、検証・推敲のプロバイダを生成と別にしたときの性質も変わらない。
-   **秘密情報の扱いは変えない:** API キーの読み込み、`secret.Secret` による保持、出力の伏せ字（`internal/redact`）は変えない。
-   本タスクの入力は利用者が設定する環境変数であり、信頼できない入力（ネットワークの応答、外部コマンドの出力、それらから作ったファイル）を新たに読まない。DeepSeek の応答の解釈も変えない（`none` で thinking を無効にした応答も、今と同じく `content` だけを記事に使う。タスク 0003 の AC-14）。要件定義の手順の「信頼できない入力の境界」は当てはまらない。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   設定の読み込みが報告するエラーは、実行を始める前（サーバは待ち受ける前）にまとめて報告する（AC-25、タスク 0045 の AC-20）。

### 4.4. 互換性 (Compatibility)
-   `YT2COLUMN_CLAUDE_EFFORT` を設定している環境では、本タスクの後、CLI もサーバも起動時に止まる（F-004）。生成と検証・推敲のどちらも DeepSeek で、この変数が効いていなかった環境も止まる。互換層は設けない（2.3）。移行の方法は README に示す（AC-35）。
-   今は、モデル ID に `+` を含む値（`claude/model+x`）を受理し、`+` を含んだモデル ID を API に送る（`config.go:509-530` はモデル ID を変換しない）。本タスクの後は、`+` の後ろを effort として解釈し、effort の値として正しくなければ拒否する（AC-05・AC-08）。issue #566 の調べでは、実在のモデル ID に `+` を含むものはない。
-   `+` を含まない DeepSeek の値は、今と同じ要求を送る（AC-13）。
-   DeepSeek が受理する effort の値は公式文書の記述に基づく（1 章）。あるモデルが `reasoning_effort` を受け付けず API がエラーを返した場合は、設定エラーではなく、今と同じ LLM の呼び出しの失敗として報告する。実際の API での確かめ方は設計で決める（design_handoff.md H-09）。
-   記事の出力、サーバのログ、成果物ファイルに載るモデルの表記は変えない（2.3）。

### 4.5. 保守性 (Maintainability)
-   effort を受け付けるプロバイダを足すときは、そのプロバイダが受理する effort の値と省いたときの扱いを加えるだけで、`YT2COLUMN_LLM` の値で指定できるようにする。環境変数を足さない（2.1）。
-   各プロバイダが受理する effort の値の一覧は、プロバイダごとに 1 か所にだけ書き、AC-06 のメッセージが示す一覧がそれと食い違わないようにする。
-   Claude の既定の effort `high` は、Claude のアダプタの既定の出力の上限の根拠になっている（`config.go:592-596`、`docs/dev/guides/claude_output_limit.md`）。本タスクは既定を変えないので、出力の上限の測り直しは要らない。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   空の値はエラーとし、未設定と同じには扱わない（`docs/dev/project_overview.md`「設定（環境変数）」）。本タスクも例外を作らない。
-   標準ライブラリの外のモジュールを加えない（CLAUDE.md「Dependencies」）。
-   拒否のエラーは `*VarError` の形を保ち、値を持たない（`config.go:132-149`）。
-   プロバイダの API の要求の形（Claude の effort、DeepSeek の `reasoning_effort`）は、そのアダプタのパッケージにとどめる（CLAUDE.md「Provider details stay in their adapter package」）。
-   **「実行に使わない変数は読まない」方針の例外の範囲を広げる:** 方針の所在は、`docs/dev/guides/cli.md` の「再開した実行は、実行する工程の設定だけを検査する」（`docs/dev/guides/cli.md:72`）、`docs/dev/project_overview.md`「設定（環境変数）」の「中断した実行の再開では、実行する段階の LLM の変数だけを読む」（`docs/dev/project_overview.md:155`）、`LoadServer` の「It reads no variable the server does not use」（`config.go:319-322`）である。タスク 0045 は、旧変数 4 つが設定されているかどうかの検査を、この方針の例外とした（`docs/tasks/0045_llm_env_var/01_requirements.md` §1）。本タスクは、例外の対象に `YT2COLUMN_CLAUDE_EFFORT` を加える（AC-21）。理由はタスク 0045 と同じく、古い設定が残っていることに、いつ実行しても気づけるようにするためである。例外の範囲は、設定されているかどうかの検査だけであり、値は使わない（AC-23）。そのため、`YT2COLUMN_CLAUDE_EFFORT` は、今は Claude を使わない実行では読まない（`config.go:548-556`）が、本タスクの後は、CLI とサーバのどの実行でも設定されているかどうかだけを調べる（ホストは調べない。AC-26）。
-   タスク 0045 とタスク 0003 の決定は、1 章「先行タスクの決定の変更」のとおり変更する。

## 6. 用語集 (Glossary)

用語は `docs/translation_glossary.md` と統一する。生成・字数の調整・検証・推敲の 3 つは、用語集に従って「工程」と呼ぶ。本書に固有の用語は次のとおり。

-   **プロバイダの部分:** `YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM` の値の、最初の `/` より前の部分。
-   **後半:** 同じ値の、最初の `/` より後ろの部分。
-   **モデル ID:** 後半に `+` があれば `+` より前の部分、なければ後半の全体。プロバイダの API が受け付けるモデルの識別子。
-   **effort の部分:** 後半の `+` より後ろの部分。
-   **effort を省いたときの扱い:** effort の部分がない値で、そのプロバイダの工程が送るもの（3.1 の表）。Claude は `high` を送り、DeepSeek は何も送らない。
-   **旧変数:** 廃止して拒否する変数。タスク 0045 の 4 つ（`YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL`）と、本タスクの `YT2COLUMN_CLAUDE_EFFORT`。
-   **設定エラー:** 設定の読み込みで変数を拒否して実行を止めること（3.1）。
-   **生成の設定・検証・推敲の設定:** タスク 0045 の要件定義書 3.1 の表の見出しの意味。
