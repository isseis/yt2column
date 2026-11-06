# ADR-0003: LLM のプロバイダ・モデル・effort を 1 つの値 <プロバイダ>/<モデル ID>+<effort> で指定する

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

生成（字数の調整を含む）と検証・推敲は、それぞれ別のプロバイダ・モデル・effort を使える。プロバイダとモデルを段階ごとに別の環境変数で指定すると、組み合わせの規則が複雑になる。

-   モデルの変数だけを設定してプロバイダの変数を設定しないと、利用者が選んでいない既定のプロバイダへデータが送られうる。これを防ぐために拒否の規則が要る。
-   プロバイダを足すたびに、effort のための環境変数が増える。
-   effort が 1 つの変数しかないと、生成と検証・推敲で違う effort を選べない。

## 決定

-   **生成の設定は `YT2COLUMN_LLM`、検証・推敲の設定は `YT2COLUMN_REVIEW_LLM` で指定する。** どちらも 1 つの値 `<プロバイダ>/<モデル ID>[+<effort>]` を持つ。CLI はこれらと下のプロバイダ固有の設定を環境変数から読み、サーバは同じ名前のキーを設定ファイルから読む（[ADR-0018](0018-server-config-file.md)）。値の規則は、環境変数でも設定ファイルでも同じである。ただし設定ファイルでは、空の値の行を、使わないプロバイダのキーでも拒否する。
-   **値を最初の `/` で区切る。** 前半はプロバイダで、`claude` か `deepseek` に完全に一致しなければならない。後半の全体がモデル ID で、その中の `/` も含めて変換せずに API へ送る。
-   **effort は後半の最初の `+` より後ろである。** `+` は後半に 1 つまでとし、2 つ以上は設定エラーにする。`+` より前のモデル ID が空の値も拒否する。`+` を含むモデル ID は指定できない。
-   **受理する effort の値はプロバイダごとに決まっている。** Claude は `low`・`medium`・`high`・`xhigh`・`max` で、effort を省くと `high` を送る。DeepSeek は `none`・`low`・`high`・`max` で、effort を省くと `reasoning_effort` を送らず API の既定に任せる。
-   **値は補正しない。** effort の部分は前後の空白を取り除かず、大文字・小文字も変換せず、受理する値に完全一致しなければ設定エラーにする。不正な effort をモデル ID の一部とみなして受理することはない（`CLAUDE.md`「Reject, don't normalize」）。
-   **空の値は未設定ではなく設定エラーにする。** 既定を使うのは未設定のときだけである。
-   **未設定のときの既定と継承を決める。** `YT2COLUMN_LLM` が未設定なら、本番の構成（[ADR-0004](0004-production-profile.md)）を既定として使う。`YT2COLUMN_REVIEW_LLM` が未設定なら、検証・推敲は生成のプロバイダ・モデル・effort をそのまま受け継ぐ。設定して effort を省いた場合は、生成の effort を受け継がず、そのプロバイダの「effort を省いたとき」に従う。
-   **プロバイダ固有の設定は別の変数のままにする。** `ANTHROPIC_API_KEY`・`DEEPSEEK_API_KEY`・`ANTHROPIC_WORKSPACE_ID` は、その実行が使うプロバイダのものだけを読んで検査する。ほかのプロバイダの変数は、未設定・空・不正な値のいずれでも拒否しない。
-   **CLI は、廃止した変数をどの実行でも拒否する。** `YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL`・`YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL`・`YT2COLUMN_CLAUDE_EFFORT` は、生成も検証・推敲も実行しない実行（`--after review` など）を含めて設定されていれば設定エラーにし、値を読まない。古い設定が残っていることに、いつ実行しても気づけるようにするためである。サーバは環境変数から設定を読まないので、これらが環境にあっても止まらず、名前だけを警告する。設定ファイルに書かれていれば、未知のキーとして拒否する（[ADR-0018](0018-server-config-file.md)）。ホスト（`yt2column-nmhost`）は LLM の設定を読まないので対象にしない。
-   **拒否のメッセージは、変数の名前と固定の理由だけを示し、値を含めない。**

この決定の仕様の正本は [project_overview.md](../project_overview.md) の設定の節である。値の解析と既定の当てはめは `internal/config` が行い、effort の値の一覧は `internal/llm/claudeparam` と `internal/llm/deepseekparam` が 1 か所ずつ持つ。

## 採らなかった案

-   **プロバイダとモデルを別々の環境変数で指定し続ける。** 組み合わせの規則が複雑になり、「モデルだけを設定してプロバイダが既定で決まる」状態を拒否する規則が要る。1 つの値にすれば、その状態は構文の上で起こらない（[0045 要件定義書](../../tasks/0045_llm_env_var/01_requirements.md) §1）。
-   **effort をプロバイダごとの別の変数（`YT2COLUMN_CLAUDE_EFFORT` など）に残す。** プロバイダを足すたびに effort の変数が増え、effort を工程ごとに選べない（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **区切りに `:` を使う。** OpenAI のファインチューニング済みモデルの ID（`ft:gpt-4o-mini:org::abc123`）と衝突する（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **区切りに `@` を使う。** Vertex のモデル ID と衝突する（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **区切りに `,` を使う。** 2 つのモデルの並びに見えやすい（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **区切りに `#` を使う。** dotenv の実装によってはコメントになる（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **区切りに `!`・`^`・`?`・`*`・`[`・`;`・`&`・`$` などを使う。** シェルで展開されるか、メタ文字として扱われる（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **廃止した変数を新しい変数に読み替える互換層を置く。** リポジトリの外に利用者がいない（[0045 要件定義書](../../tasks/0045_llm_env_var/01_requirements.md) §1）。
-   **廃止した変数を黙って無視する。** 古い `.envrc` で DeepSeek を指定している利用者のデータが、気づかないうちに既定の Claude へ送られる（[0045 要件定義書](../../tasks/0045_llm_env_var/01_requirements.md) §1）。
-   **effort の部分が不正な値を、モデル ID の一部とみなして受理する。** 指定と異なる振る舞いを隠さないためである（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **DeepSeek で、`minimal`・`medium`・`xhigh`・`ultra` も effort として受理する。** API の側がこれらの値を別の値に読み替えるので、指定と異なる振る舞いになる（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。
-   **DeepSeek に effort を付けた値を設定エラーにする。** DeepSeek の API が `reasoning_effort` を受け付けるため、レビュアーの判断で、DeepSeek でも effort を指定できるようにした（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md) §1）。

## 結果

-   値の文法は 1 か所で決まり、プロバイダごとに受理する effort の値の一覧は `claudeparam` と `deepseekparam` に 1 か所ずつある。
-   未設定のときの既定の値（本番の構成）は [ADR-0004](0004-production-profile.md) が定める。本番の構成を変えるときは、両方の ADR を照らす。
-   プロバイダを足すときは、プロバイダの部分にその名前を加え、そのプロバイダの effort の値の一覧を足す。環境変数は増えない。
-   プロバイダの部分の誤りは起動時の設定エラーになる。既定のプロバイダに黙って落ちることはない。
-   モデル ID は不透明な文字列として扱う。プロバイダに合わないモデル名は、設定の読み込みではなく API の呼び出しで失敗する。
-   廃止した変数が環境に残っている CLI の実行は、その値によらず、通信の前に止まる。サーバは止まらず、名前を警告する。effort は出力（標準エラーの要約、記事のフロントマター、Slack の投稿）に出さないので、使った effort は成功した実行の出力からは分からない。

## 経緯

- 2026-10-10 プロバイダとモデルを環境変数で選び、Claude の effort の値の一覧と、effort を必須とする規則を決めた（[0009 要件定義書](../../tasks/0009_claude_llm_client/01_requirements.md)）
- 2026-10-25 Claude の effort の既定を `high` にし、出力の上限を 40,000 にした（[0041 設計書](../../tasks/0041_claude_effort_max_tokens/02_architecture.md)）
- 2026-10-25 既定のプロバイダとモデルを `claude` / `claude-opus-5-5` にした（[0042 要件定義書](../../tasks/0042_default_llm_opus/01_requirements.md)）
- 2026-10-27 プロバイダとモデルを 1 つの値 `<プロバイダ>/<モデル ID>` で指定するようにした（[0045 要件定義書](../../tasks/0045_llm_env_var/01_requirements.md)）
- 2026-10-29 値に `+<effort>` を含め、effort を工程ごとに指定できるようにした（[0051 要件定義書](../../tasks/0051_llm_effort/01_requirements.md)）
- 2026-11-06 サーバは同じ名前のキーを設定ファイルから読むようにし、廃止した変数が環境にあってもサーバは止まらず名前を警告し、ファイルでは空の値をどのキーでも拒否するようにした（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md)）
