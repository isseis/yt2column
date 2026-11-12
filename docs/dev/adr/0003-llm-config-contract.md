# ADR-0003: LLM のプロバイダ・モデル・effort を 1 つの値 <プロバイダ>/<モデル ID>+<effort> で指定する

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

生成（字数の調整を含む）と検証・推敲は、それぞれ別のプロバイダ・モデル・effort を使える。プロバイダとモデルを工程ごとに別の環境変数で指定すると、組み合わせの規則が複雑になる。

-   モデルの変数だけを設定してプロバイダの変数を設定しないと、利用者が選んでいない既定のプロバイダへデータが送られうる。これを防ぐために拒否の規則が要る。
-   プロバイダを足すたびに、effort のための環境変数が増える。
-   effort が 1 つの変数しかないと、生成と検証・推敲で違う effort を選べない。

## 決定

-   **生成の設定は `YT2COLUMN_LLM`、検証・推敲の設定は `YT2COLUMN_REVIEW_LLM` で指定し、どちらも 1 つの値 `<プロバイダ>/<モデル ID>[+<effort>]` を持つ。** 1 つの値にすれば、「モデルだけが設定されていて、プロバイダは既定の値になる」状態は構文の上で起こらない。effort も値に含めるのは、プロバイダを足すたびに effort の変数が増えないようにし、effort を工程ごとに選べるようにするためである。規則は [project_overview.md](../project_overview.md) の「設定（環境変数）」にある。
-   **CLI はこれらと下のプロバイダ固有の設定を環境変数から読む。拡張の WASM の入口は、拡張が保存した API キーとワークスペース ID を、`ANTHROPIC_API_KEY` と `ANTHROPIC_WORKSPACE_ID` の 2 つの名前だけに値を返す名前を引く関数として `internal/config` に渡す。値の規則は、空の値の扱いを含め、どちらの入口でも同じである。** 2 つの入口が同じ名前の表と同じ値の検証を通るようにし、規則を 1 か所に保つためである。拡張では `YT2COLUMN_LLM` と `YT2COLUMN_REVIEW_LLM` が未設定として扱われるので、3 つの工程とも本番の構成になる（[ADR-0004](0004-production-profile.md)・[ADR-0048](0048-extension-only-generation.md)）。環境変数を読む場所は [ADR-0024](0024-env-access-in-config.md) が定める。規則は [project_overview.md](../project_overview.md) の「設定（環境変数）」と、[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md) の「設定の読み込み」にある。
-   **値の解析と既定の当てはめは `internal/config` が行う。** 理由は記録されていない。コードは `internal/config` にある。
-   **値を最初の `/` で区切る。前半はプロバイダで、受理するプロバイダの名前に完全に一致しなければならない。後半の全体がモデル ID で、その中の `/` も含めて変換せずに API へ送る。** 中継するプロバイダ（OpenRouter など）を足しても、構文を変えずに表せるようにするためである。受理するプロバイダの名前は、[project_overview.md](../project_overview.md) の「設定（環境変数）」にある。OpenAI を加えることは [0077 要件定義書](../../tasks/0077_openai_llm_client/01_requirements.md) F-001 が定める。
-   **effort は後半の最初の `+` より後ろである。** `+` は後半に 1 つまでとし、2 つ以上は設定エラーにする。`+` より前のモデル ID が空の値も拒否する。`+` を含むモデル ID は指定できない。`+` は、モデル ID に現れず、シェルで引用符なしに書いても意味が変わらず、dotenv の書式で壊れないためである。規則は [project_overview.md](../project_overview.md) の「設定（環境変数）」にある。
-   **受理する effort の値はプロバイダごとに決まっており、その一覧はプロバイダごとに 1 か所に置く。** 受理する effort の値はプロバイダごとに異なるためである。一覧は `internal/llm/claudeparam`、`internal/llm/deepseekparam`、`internal/llm/openaiparam` が 1 か所ずつ持ち、その表は [project_overview.md](../project_overview.md) の「設定（環境変数）」にある。
-   **effort を省くと、Claude では `high` を送り、DeepSeek では `reasoning_effort` を送らず API の既定に任せる。** Claude の `high` は、何も指定しない利用者が本番の構成の effort で記事を作れるようにするためである。DeepSeek で何も送らないのは、今までの DeepSeek の設定が同じ要求を送り続けるようにするためである。規則は [project_overview.md](../project_overview.md) の「設定（環境変数）」にある。
-   **OpenAI は API 全体の effort の値を受理し、省略時は `medium` を明示して送る。** モデルの一覧を制限せず、省略時の振る舞いを固定するためである。値の一覧と拒否の条件は [0077 要件定義書](../../tasks/0077_openai_llm_client/01_requirements.md) F-001 が持つ。モデルが対応しない組み合わせは API のエラーとして通知し、設定の読み込みではモデルごとの対応表を持たない。
-   **値は補正しない。** effort の部分は前後の空白を取り除かず、大文字・小文字も変換せず、受理する値に完全一致しなければ設定エラーにする。不正な effort をモデル ID の一部とみなして受理することはない。指定と異なる振る舞いを隠さないためである（`CLAUDE.md`「Reject, don't normalize」）。規則は [project_overview.md](../project_overview.md) の「設定（環境変数）」にある。
-   **空の値は未設定ではなく設定エラーにする。既定を使うのは未設定のときだけである。** 展開に失敗して空になった値のために、利用者が選んでいない既定のプロバイダへデータが送られることを防ぐためである。規則は [project_overview.md](../project_overview.md) の「設定（環境変数）」にある。
-   **`YT2COLUMN_LLM` が未設定なら、本番の構成（[ADR-0004](0004-production-profile.md)）を既定として使う。** 既定を本番の構成に合わせるためである。規則は [project_overview.md](../project_overview.md) の「設定（環境変数）」にある。
-   **`YT2COLUMN_REVIEW_LLM` が未設定なら、検証・推敲は生成のプロバイダ・モデル・effort をそのまま受け継ぐ。設定して effort を省いた場合は、生成の effort を受け継がず、そのプロバイダの「effort を省いたとき」に従う。** 理由は記録されていない。規則は [project_overview.md](../project_overview.md) の「設定（環境変数）」にある。
-   **プロバイダ固有の設定は別の変数のままにする。** `ANTHROPIC_API_KEY`・`DEEPSEEK_API_KEY`・`ANTHROPIC_WORKSPACE_ID`・`OPENAI_API_KEY` は、その実行が使うプロバイダのものだけを読んで検査する。ほかのプロバイダの変数は、未設定・空・不正な値のいずれでも拒否しない。OpenAI のキーも、CLI は環境変数から読む（[0077 要件定義書](../../tasks/0077_openai_llm_client/01_requirements.md) F-001）。拡張が保存するのは Anthropic の API キーとワークスペース ID だけである。環境変数はほかのツールと共有する場所で、検査すると読まない値を検査することになるためである。規則は [project_overview.md](../project_overview.md) の「設定（環境変数）」にある。
-   **廃止した変数は読まない。** `YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL`・`YT2COLUMN_CLAUDE_EFFORT` は、CLI も拡張の WASM の入口も読まず、環境にあっても拒否も警告もしない。読まない変数を検査するためだけのコードは持たない。CLI では、サーバの経路から検査を外したのと同じ変更で、CLI にだけ残す理由がなかったためである。規則は [0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) の「1. 概要 (Overview)」にある。
-   **拒否のメッセージは、変数の名前と固定の理由だけを示し、値を含めない。** 秘密情報を誤って別の変数に設定した場合でも、エラーの出力から漏れないようにするためである。規則は [0045 要件定義書](../../tasks/0045_llm_env_var/01_requirements.md) の「4.2. セキュリティ (Security)」にある。

拡張の WASM の入口が 2 つの名前だけを返す名前を引く関数で設定を読むことは、まだコードと project_overview.md に反映していない。今はサーバが設定ファイルから同じ名前のキーを読む。この差分は [#773](https://github.com/isseis/yt2column/issues/773) の実装で解消する。

OpenAI の設定はコードと project_overview.md に反映した。アダプタ本体と工程への配線は [#568](https://github.com/isseis/yt2column/issues/568) の残りの実装で加える。

## 採らなかった案

-   **プロバイダとモデルを別々の環境変数で指定し続ける。** 組み合わせの規則が複雑になり、「モデルだけを設定してプロバイダが既定で決まる」状態を拒否する規則が要る。1 つの値にすれば、その状態は構文の上で起こらない（[0045 要件定義書](../../tasks/0045_llm_env_var/01_requirements.md) §1）。
-   **effort をプロバイダごとの別の変数（`YT2COLUMN_CLAUDE_EFFORT` など）に残す。** プロバイダを足すたびに effort の変数が増え、effort を工程ごとに選べない（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **区切りに `:` を使う。** OpenAI のファインチューニング済みモデルの ID（`ft:gpt-4o-mini:org::abc123`）と衝突する（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **区切りに `@` を使う。** Vertex のモデル ID と衝突する（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **区切りに `,` を使う。** 2 つのモデルの並びに見えやすい（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **区切りに `#` を使う。** dotenv の実装によってはコメントになる（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **区切りに `!`・`^`・`?`・`*`・`[`・`;`・`&`・`$` などを使う。** シェルで展開されるか、メタ文字として扱われる（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **廃止した変数を新しい変数に読み替える互換層を置く。** リポジトリの外に利用者がいない（[0045 要件定義書](../../tasks/0045_llm_env_var/01_requirements.md) §1）。
-   **廃止した変数を、どの実行でも拒否し続ける。** 読まない変数を検査するためだけのコードを持たないことにした（レビュアーの判断）（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **effort の部分が不正な値を、モデル ID の一部とみなして受理する。** 指定と異なる振る舞いを隠さないためである（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **DeepSeek で、`minimal`・`medium`・`xhigh`・`ultra` も effort として受理する。** API の側がこれらの値を別の値に読み替えるので、指定と異なる振る舞いになる（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **DeepSeek に effort を付けた値を設定エラーにする。** DeepSeek の API が `reasoning_effort` を受け付けるため、レビュアーの判断で、DeepSeek でも effort を指定できるようにした（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **OpenAI の effort を候補モデル共通の値だけに限る、または省略時に API の既定に任せる。** 他のモデルで有効な値も指定できるようにし、省略時の振る舞いを提供元に依存させないため、どちらも採らない（[0077 要件定義書](../../tasks/0077_openai_llm_client/01_requirements.md) §1）。

## 結果

-   値の文法は 1 か所で決まり、プロバイダごとに受理する effort の値の一覧は `claudeparam`・`deepseekparam`・`openaiparam` に 1 か所ずつある。
-   未設定のときの既定の値（本番の構成）は [ADR-0004](0004-production-profile.md) が定める。本番の構成を変えるときは、両方の ADR を照らす。
-   プロバイダを足すときは、プロバイダの部分にその名前を加え、そのプロバイダの effort の値の一覧を足す。環境変数は増えない。
-   プロバイダの部分の誤りは起動時の設定エラーになる。既定のプロバイダに黙って落ちることはない。
-   モデル ID は不透明な文字列として扱う。プロバイダに合わないモデル名は、設定の読み込みではなく API の呼び出しで失敗する。
-   古い `.envrc` で廃止した変数だけで DeepSeek を指定している場合、CLI は気づかせずに既定の Claude を選ぶ（`ANTHROPIC_API_KEY` がなければ、その設定エラーで止まる）。利用者がリポジトリの外にいないので受け入れる。effort は出力（標準エラーの要約、記事のフロントマター）に出さないので、使った effort は成功した実行の出力からは分からない。

## 経緯

- 2026-10-10 プロバイダとモデルを環境変数で選び、Claude の effort の値の一覧と、effort を必須とする規則を決めた（[0009 要件定義書](../../tasks/0009_claude_llm_client/01_requirements.md)）
- 2026-10-25 Claude の effort の既定を `high` にし、出力の上限を 40,000 にした（[0041 設計書](../../tasks/0041_claude_effort_max_tokens/02_architecture.md)）
- 2026-10-25 既定のプロバイダとモデルを `claude` / `claude-opus-5-5` にした（[0042 要件定義書](../../tasks/0042_default_llm_opus/01_requirements.md)）
- 2026-10-27 プロバイダとモデルを 1 つの値 `<プロバイダ>/<モデル ID>` で指定するようにした（[0045 要件定義書](../../tasks/0045_llm_env_var/01_requirements.md)）
- 2026-10-29 値に `+<effort>` を含め、effort を工程ごとに指定できるようにした（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md)）
- 2026-11-06 サーバは同じ名前のキーを設定ファイルから読むようにし、廃止した変数を CLI もサーバも拒否せずに無視するようにし、ファイルでは空の値をどのキーでも拒否するようにした（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md)）
- 2026-11-06 設定ファイルでも空の値を環境変数と同じに扱い、使わないプロバイダのキーの空の値は拒否しないことにした（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md)）
- 2026-11-10 OpenAI とその API キーを選択肢に加え、API 全体の effort の値を受理し、省略時は medium を送ることにした（[0077 要件定義書](../../tasks/0077_openai_llm_client/01_requirements.md)）
- 2026-11-12 サーバの設定ファイルの代わりに、拡張の WASM の入口が保存した 2 つの値だけを返す名前を引く関数で同じ検査を通ることにした（[0090 設計書](../../tasks/0090_extension_only_architecture/02_architecture.md)）
