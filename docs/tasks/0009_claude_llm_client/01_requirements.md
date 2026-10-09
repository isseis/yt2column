# 要件定義書：Claude アダプタ（LLMClient 実装）

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-09 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`LLMClient` の 2 つ目の実装である Claude アダプタ（`internal/llm/claude`）と、それを CLI から選べるようにする設定に関する要件を定義する。Claude アダプタは、system プロンプトと user プロンプトを Anthropic の Messages API へ送り、生成テキストと、生成に使われたモデル名を返す。

**背景:**
yt2column は、LLM プロバイダを差し替えられるように作られている（[project_overview.md](../../dev/project_overview.md)「決定済みの方針」）。現在の実装は DeepSeek だけである（タスク `0003_deepseek_llm_client`）。記事の品質を比べるため、Claude でも記事を生成できるようにする。

Claude のモデルは、Opus・Sonnet・Haiku などの系列がある。また、Messages API の effort（`output_config.effort`）で、推論の深さと出力トークンの量を調整できる。どのモデルを、どの effort で使うかは未定であり、コラム生成プロンプトの評価（タスク `0008_column_prompt`）のような比較の結果を見て決める。そのため、モデルと effort は、コードを変えずに環境変数だけで切り替えられるようにする。

現行の Claude のモデルでは、thinking（推論）が常に有効、または既定で有効である。推論過程は `thinking` ブロックとして、生成テキストの `text` ブロックとは別に返る。記事には `text` ブロックだけを使う。

**本書の記述範囲:** 本書は、観測できる振る舞い（何を送り、何を受理し、何をどの番兵エラーで拒否するか）と、設定として受け付ける値を定める。その振る舞いを実現する手段（型や関数の名前、標準ライブラリの API の使い方、上限値など）は設計（`02_architecture.md`）で決める。要件レビューの過程で挙がった実装上の注意点と、設計で決める事項は [design_handoff.md](design_handoff.md) に申し送る。

DeepSeek アダプタと共通する規則（秘密情報の保護、通信の失敗の報告、信頼できない入力の検証など）は、本書でも AC として改めて定める。アダプタごとに独立して検証できるようにするためである。

対応 issue: #9

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `GenerateRequest` を Claude の Messages API へ送り、生成テキストとモデル名を `GenerateResponse` として返せる。
-   モデルと effort を、環境変数だけで切り替えられる。
-   空の応答、途中で打ち切られた応答、`end_turn` 以外の終了理由の応答（拒否を含む）を、成功として返さずエラーとして報告する。
-   推論過程（`thinking` ブロック）を生成テキストに含めない。
-   API キーを、送信先への `x-api-key` ヘッダー以外には、エラーにもログにも、その他のどの出力にも漏らさない。
-   API の応答という信頼できない入力を、補正せずに検証する。不正な入力は対応する番兵エラーで拒否する（3.2）。
-   Claude の API もネットワークも使わずに、アダプタと設定の振る舞いをテストできる。

### 2.2. スコープ (In Scope)

-   `internal/llm/claude` パッケージと、`llm.LLMClient` を実装する型
-   構築時の入力（API キー・モデル名・effort・タイムアウト）の検証
-   `GenerateRequest` の検証と、Messages API へのリクエストの組み立て
-   応答の検証（HTTP ステータス、JSON の形、終了理由、空の本文）と `GenerateResponse` への変換
-   タイムアウト・キャンセル・通信失敗の報告
-   `internal/config` での `YT2COLUMN_LLM_PROVIDER=claude`・`ANTHROPIC_API_KEY`・effort の環境変数の読み込みと検証
-   `internal/llm/provider` での Claude アダプタの構築
-   `httptest` を使うユニットテスト
-   実 API を使う手動実行の統合テスト（既定のテストからは除外）
-   文書の更新（[project_overview.md](../../dev/project_overview.md) の設定の表とパッケージ構成、[security.md](../../dev/security.md) §4、[package_reference.md](../../dev/developer_guide/package_reference.md)、`README.md` の設定の説明）

### 2.3. スコープ外 (Out of Scope)

-   どのモデル・effort を使うかの決定と、そのための記事の評価。本タスクは切り替えの仕組みだけを作る。
-   Anthropic の公式 SDK（`github.com/anthropics/anthropic-sdk-go`）の利用。DeepSeek アダプタと同じく、標準ライブラリ（`net/http`・`encoding/json`）で呼ぶ。SDK の JSON のデコードは寛容で、応答を補正せずに拒否する検証（3.2）と両立しにくいためである。外部モジュールを増やさない方針（CLAUDE.md「Dependencies」）にも合う。
-   ストリーミング応答（`stream: true`）。非ストリーミングで 1 回の応答として受け取る。高い effort で生成が長くなり、タイムアウトが実際に問題になると分かった場合は、別の issue にする。
-   thinking の設定（`thinking` パラメータ）の切り替え。リクエストに `thinking` を含めず、モデルの既定に任せる。推論の深さは effort で調整する。
-   出力トークン数の上限を設定（環境変数）で変えること。`MaxOutputTokens` が 0 の場合の値はアダプタの定数とする（F-002）。
-   拒否（`refusal`）時のフォールバック（server-side fallbacks・クライアント側のミドルウェア）。フォールバックは別のモデルで生成し直すため、どのモデルで生成したかの比較を損なう。拒否はエラーとして報告する（F-003）。
-   `temperature` などのサンプリングパラメータ、tool use、構造化出力、プロンプトキャッシュ、Batch API、`anthropic-beta` ヘッダーで有効にする機能。
-   リトライ。失敗は 1 回で呼び出し元へ報告する。
-   プロバイダごとのプロンプトの調整（[project_overview.md](../../dev/project_overview.md)「未確定事項」）。
-   トークン使用量（`usage`）や料金の記録。
-   Amazon Bedrock・Google Cloud Vertex AI など、Anthropic の API 以外の経路。
-   Gemini の実装（#15）。
-   CI での統合テストの実行（実 API と API キーが必要なため、ローカルでの手動実行に限る）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 構築

Claude アダプタの値を、API キー・モデル名・effort・タイムアウトから構築する。構築時の入力は補正せず、不正なら構築をエラーにする。

-   API キーは `secret.Secret` 型で受け取る。ゼロ値の `Secret` は拒否する。
-   モデル名は文字列で受け取る。空文字列、前後に空白文字を含む値、および正しい UTF-8 でない値は拒否する。それ以外の値は検証せずそのまま API へ送る（モデルは頻繁に追加されるため、既知の名前の一覧と照合しない）。
-   effort は、`low`・`medium`・`high`・`xhigh`・`max` の 5 つの値を表す列挙型で受け取る。列挙型のゼロ値は「未指定」を表し、構築はこれを拒否する。effort をモデルの既定に任せず、必ず明示するためである（Opus 5.5 の既定は `medium`、Sonnet 5.5 の既定は `high` のように、モデルによって既定値が異なる）。
-   アダプタは、モデル名から effort の可否や thinking の扱いを推測しない（CLAUDE.md「Declare, don't infer」）。モデルが受け付けない effort の値は、API が `400` で拒否し、F-003 の HTTP ステータスのエラーとして報告される。
-   タイムアウトは正の値でなければならない。
-   送信先は `https://api.anthropic.com/v1/messages` とする。送信先をテストから差し替えられるようにする。差し替えの手段と、本番の送信先を利用者の設定から変えられないようにする方法は設計で決める（DeepSeek アダプタと同じ手段を使ってよい）。
-   構築したアダプタは `llm.LLMClient` を実装する。

**Acceptance Criteria**:
- **AC-01**: 有効な API キー・モデル名・5 つの effort のいずれか・正のタイムアウトから構築したアダプタは、`llm.LLMClient` として使える。
- **AC-02**: ゼロ値の `Secret`、空のモデル名、前後に空白文字を含むモデル名（例: `" claude-opus-5-5"`、`"claude-opus-5-5\n"`）、不正な UTF-8 のバイト列を含むモデル名、未指定（ゼロ値）の effort、5 つの値のどれでもない effort、0 以下のタイムアウトのいずれかを与えた構築はエラーになり、アダプタを返さない。

#### F-002: リクエストの送信

`GenerateRequest` を検証し、Messages API へ 1 回の HTTP リクエストとして送る。

-   `GenerateRequest` の検証は `llm.GenerateRequest.Validate` の規則に従う。条件に反するリクエストは、HTTP リクエストを送らずに `llm.ErrInvalidRequest` で拒否する。
-   HTTP メソッドは `POST`、本文は JSON とする。リクエストは次のヘッダーを持つ。
    -   `Content-Type: application/json`
    -   `x-api-key: <API キー>`
    -   `anthropic-version`: 設計で固定する API のバージョン（例: `2023-06-01`）。
-   `anthropic-beta` ヘッダーは送らない。
-   本文には、次のメンバーを含める。プロンプトは加工せず、そのまま送る。
    -   `model`: 構築時のモデル名
    -   `system`: `SystemPrompt` の文字列
    -   `messages`: `UserPrompt` を内容とする `user` のメッセージ 1 件だけの配列
    -   `max_tokens`: `MaxOutputTokens` が正の値ならその値、0 ならアダプタの定数（値は設計で、実測をもとに決める。effort を上げると推論過程の分だけ出力トークンが増えるため、記事の生成に十分な余裕を持たせる）
    -   `output_config.effort`: 構築時の effort を表す文字列（`low`・`medium`・`high`・`xhigh`・`max`）
-   `thinking`・`stream`・`temperature`・`top_p`・`top_k`・`tools`・`tool_choice`・`fallbacks` など、上に挙げた以外のメンバーは送らない。
-   API キーは `x-api-key` ヘッダーだけで送る。アダプタは、リクエスト本文と URL に API キーを加えない。呼び出し元がプロンプトに API キーと同じ文字列を含めた場合、その文字列はプロンプトの一部として加工せずに送る。
-   リダイレクトには従わない。3xx の応答は F-003 の HTTP ステータスのエラーとして扱う。API キーを別の送信先へ送らないためである。
-   リトライしない。

**Acceptance Criteria**:
- **AC-03**: `Generate` は、送信先へ `POST` を 1 回だけ送る。送信先が受け取るリクエストの `Content-Type` ヘッダーは `application/json`、`x-api-key` ヘッダーは API キー、`anthropic-version` ヘッダーは設計で固定した値であり、`anthropic-beta` ヘッダーと `Authorization` ヘッダーを含まない。API キーを含まないプロンプトで呼び出したとき、リクエスト本文と URL には API キーが現れない。
- **AC-04**: リクエスト本文の JSON は、構築時のモデル名の `model`、`SystemPrompt` と同一の文字列の `system`、`UserPrompt` と同一の文字列を内容とする `user` のメッセージ 1 件だけの `messages` を含む。各プロンプトは送信前と同一の文字列である（前後の空白や改行も含めて変更されない）。
- **AC-05**: `MaxOutputTokens` が正の値のとき、リクエスト本文の `max_tokens` はその値である。0 のとき、`max_tokens` は設計で固定したアダプタの定数である。
- **AC-06**: リクエスト本文の `output_config.effort` は、構築時の effort を表す文字列である。5 つの effort のそれぞれについて確かめる。
- **AC-07**: リクエスト本文のトップレベルのメンバーは `model`・`system`・`messages`・`max_tokens`・`output_config` だけであり、`output_config` のメンバーは `effort` だけである（`thinking`・`stream` などを含まない）。
- **AC-08**: `SystemPrompt` または `UserPrompt` が空のリクエスト、いずれかが不正な UTF-8 のバイト列を含むリクエスト、および `MaxOutputTokens` が負のリクエストは、`errors.Is(err, llm.ErrInvalidRequest)` が真になるエラーになり、送信先へ HTTP リクエストを送らない。
- **AC-09**: 送信先が 3xx（例: `307` と `Location` ヘッダー）を返した場合、`Generate` はリダイレクト先へリクエストを送らず、HTTP ステータスのエラー（AC-11）を返す。

#### F-003: 応答の検証と変換

応答を検証し、生成テキストとモデル名を `GenerateResponse` として返す。検証は次の順序で行い、最初に失敗した手順の番兵を返す。

1.  HTTP ステータス: `200` 以外は `ErrHTTPStatus` とする。エラーからステータスコードを取り出せるようにする。応答本文はエラーに含めない（信頼できない入力であり、利用者の端末へそのまま出力しないため）。
2.  応答本文の形: 3.2 の受理する形に合致しない場合、およびサイズの上限を超える場合は `ErrInvalidResponse` とする。
3.  終了理由: `stop_reason` が `max_tokens` の場合は `llm.ErrTruncated`、`end_turn` と `max_tokens` 以外の値（例: `refusal`・`stop_sequence`・`tool_use`・`pause_turn`・`model_context_window_exceeded`・未知の値）の場合は `llm.ErrUnexpectedFinishReason` とする。
4.  生成テキスト: `text` ブロックがない場合、または `text` ブロックの `text` が空文字列か空白文字だけの場合は `llm.ErrEmptyResponse` とする。

すべての手順を通過した場合に限り、次の `GenerateResponse` を返す。

-   `Text` は `text` ブロックの `text` の文字列を加工せずそのまま入れる。`thinking` ブロック・`redacted_thinking` ブロックの内容は含めない。
-   `Model` は応答の `model` の値を入れる。構築時のモデル名ではない。
-   `ModelVersion` は空文字列とする。Messages API の応答には、`model` のほかにモデルの版を表す値がないためである（`model` が版を含む識別子である）。

いずれの拒否時も、部分的な結果（それまでに得た `text` など）を返さない。

**Acceptance Criteria**:
- **AC-10**: `stop_reason` が `end_turn` で、空白文字以外を含む `text` ブロックを 1 つ持つ応答に対し、`Generate` はエラーを返さず、`Text` が `text` と同一の文字列（前後の空白や改行も含めて変更されない）、`Model` が応答の `model` の値、`ModelVersion` が空文字列である `GenerateResponse` を返す。応答の `model` が構築時のモデル名と異なる場合も、`Model` は応答の値である。
- **AC-11**: HTTP ステータスが `200` 以外の応答（少なくとも `400`・`401`・`403`・`404`・`413`・`429`・`500`・`529`、および AC-09 の `307`）に対し、`errors.Is(err, ErrHTTPStatus)` が真になるエラーを返す。エラーからステータスコードを `errors.AsType` で取り出せ、その値は応答のステータスコードと一致する。エラーメッセージに応答本文に含まれていた文字列（テストで本文に埋め込んだ目印）は現れない。
- **AC-12**: `stop_reason` が `max_tokens` の応答は、空でない `text` ブロックを持っていても `errors.Is(err, llm.ErrTruncated)` が真になるエラーになり、打ち切られた `text` を返さない（返る `GenerateResponse` はゼロ値である）。
- **AC-13**: `stop_reason` が `end_turn`・`max_tokens` 以外の文字列（`refusal`・`stop_sequence`・`tool_use`・`pause_turn`・`model_context_window_exceeded`・テスト用の未知の値）の応答は、`errors.Is(err, llm.ErrUnexpectedFinishReason)` が真になるエラーになり、`llm.ErrTruncated` ではない。
- **AC-14**: `stop_reason` が `end_turn` で、`text` ブロックがない応答（`content` が空の配列、または `thinking` ブロックだけ）、および `text` が空文字列か空白文字（半角空白・改行・タブ）だけの応答は、`errors.Is(err, llm.ErrEmptyResponse)` が真になるエラーになる。
- **AC-15**: `thinking` ブロック（`thinking` に目印の文字列を持つもの）・`redacted_thinking` ブロックと `text` ブロックの両方を持つ応答に対し、`Text` は `text` ブロックの `text` だけであり、`thinking` ブロックの目印を含まない。`content` の中でのブロックの順序によらず同じである。
- **AC-16**: 検証は F-003 の順序で行う。`stop_reason` が `max_tokens` で `text` ブロックがない応答は `llm.ErrTruncated` になり、`llm.ErrEmptyResponse` ではない。`200` 以外のステータスで本文が不正な JSON の応答は `ErrHTTPStatus` になり、`ErrInvalidResponse` ではない。
- **AC-17**: 番兵 `llm.ErrInvalidRequest`・`ErrHTTPStatus`・`ErrInvalidResponse`・`llm.ErrTruncated`・`llm.ErrUnexpectedFinishReason`・`llm.ErrEmptyResponse`・`ErrTransport`（F-004）は、`errors.Is` で相互に区別できる。タイムアウトとキャンセルは `context.DeadlineExceeded` / `context.Canceled` で判別できる（AC-18・AC-19）。

#### F-004: 通信の失敗・タイムアウト・キャンセル

-   構築時のタイムアウトは、1 回の `Generate` の中で、HTTP リクエストの送信を始めてから応答本文の読み取りを終えるまでの全体に適用する。
-   タイムアウト（構築時のタイムアウト、または `ctx` の期限）で失敗した場合は、`errors.Is(err, context.DeadlineExceeded)` が真になるエラーを返す。`ctx` のキャンセルで失敗した場合は、`errors.Is(err, context.Canceled)` が真になるエラーを返す。
-   `ctx` が既にキャンセルされている場合は、HTTP リクエストを送らない。
-   接続の失敗など、タイムアウトとキャンセル以外の通信の失敗は `ErrTransport` とする。応答のヘッダーを受け取った後、応答本文の読み取り中に接続が切れた場合も `ErrTransport` とし、`ErrInvalidResponse` としない。受け取れた本文の一部を検証に使わず、部分的な結果を返さない。

**Acceptance Criteria**:
- **AC-18**: 応答を返さずに待ち続ける送信先に対し、構築時のタイムアウトを短く設定した `Generate` は、構築時のタイムアウトが経過してから `02_architecture.md` で固定した猶予時間以内に戻り、`errors.Is(err, context.DeadlineExceeded)` が真になるエラーを返す。ヘッダーを返した後に本文の送信を止める送信先に対しても同様である。
- **AC-19**: `Generate` の実行中に `ctx` をキャンセルすると、`Generate` は戻り、`errors.Is(err, context.Canceled)` が真になるエラーを返す。呼び出し前に `ctx` が既にキャンセルされている場合は、送信先へ HTTP リクエストを送らずに同じエラーを返す。
- **AC-20**: 接続できない送信先（例: 閉じた `httptest` サーバーのアドレス）に対し、`errors.Is(err, ErrTransport)` が真になるエラーを返し、`context.DeadlineExceeded` でも `context.Canceled` でもない。ステータス `200` と応答本文の長さを示すヘッダーを返し、示した長さより前で本文の途中（有効な応答の JSON の一部まで）を送って接続を閉じる送信先に対しても同様であり、`ErrInvalidResponse` ではなく、返る `GenerateResponse` はゼロ値である。

#### F-005: 秘密情報の保護

アダプタは、API キーを送信先への `x-api-key` ヘッダー以外のどこにも出さない（呼び出し元がプロンプトに含めた文字列は F-002 による）。

-   構築と `Generate` が返すエラー、およびアダプタの値を `fmt`・`log/slog`・`encoding/json` で出力した結果に、API キーを含めない。
-   `*url.Error` など、標準ライブラリのエラーをラップするときも、リクエストのヘッダーや API キーを含めない。

**Acceptance Criteria**:
- **AC-21**: AC-02（有効な API キーと不正なモデル名・effort・タイムアウトの組み合わせ）・AC-08・AC-09・AC-11・AC-12・AC-13・AC-14・AC-18・AC-19・AC-20 の各エラーケース、および 3.2 の各拒否ケースについて、返るエラーを `Error()`、`%v`、`%+v`、`%#v` で文字列にした結果に、テストで使った API キーの文字列が現れない。
- **AC-22**: 構築したアダプタの値を `fmt`（`%v`・`%+v`・`%#v`）、`log/slog`（TextHandler・JSONHandler）、`encoding/json` で出力した結果に、API キーの文字列が現れない。

#### F-006: 設定の読み込みとプロバイダの選択

CLI が環境変数から Claude アダプタを選べるようにする。環境変数の読み込みは `internal/config` だけが行い、アダプタは環境変数を読まない。

-   `YT2COLUMN_LLM_PROVIDER` は `deepseek` に加えて `claude` を受理する。値は補正しない（大文字・前後の空白を含む値は拒否する）。未設定のときの既定は `deepseek` のままとする。
-   `ANTHROPIC_API_KEY` は、プロバイダが `claude` のとき必須とする。扱いは `DEEPSEEK_API_KEY` と対称にする。空の値はプロバイダによらず拒否し、空でない値はプロバイダが `claude` のときだけ保持する。
-   effort は新しい環境変数 `YT2COLUMN_CLAUDE_EFFORT` で指定する。プロバイダ固有の値であることを名前で示すため、`CLAUDE` を含める。
    -   受理する値は `low`・`medium`・`high`・`xhigh`・`max` の 5 つだけとする。それ以外の値（大文字、前後の空白、`none`・`min` など）は、補正・丸めをせずに拒否する（CLAUDE.md「Reject, don't normalize」）。
    -   プロバイダが `claude` のとき必須とし、既定値を持たない。どの effort で生成したかを、設定から常に読み取れるようにするためである。
    -   空の値と、5 つの値以外の値は、プロバイダによらず拒否する。5 つの値のいずれかである値は、プロバイダが `claude` のときだけ保持する（`ANTHROPIC_API_KEY` と同じく、プロバイダの切り替えを `YT2COLUMN_LLM_PROVIDER` だけで行えるようにするため）。
-   モデル名は既存の `YT2COLUMN_MODEL` で指定する（既定値なし）。プロバイダごとの変数は設けない。
-   拒否時のエラーは、既存の `config.VarError` と同じく、変数名と固定の理由だけを持ち、変数の値を含めない。
-   `internal/llm/provider` は、プロバイダが `claude` のとき、設定の API キー・モデル名・effort と、既存の LLM のタイムアウトから Claude アダプタを構築する。タイムアウトの値を DeepSeek と共有するか分けるかは設計で決める。

**Acceptance Criteria**:
- **AC-23**: `YT2COLUMN_LLM_PROVIDER=claude`、`YT2COLUMN_MODEL`、`ANTHROPIC_API_KEY`、および 5 つの値のいずれかの `YT2COLUMN_CLAUDE_EFFORT` を設定した環境から読み込んだ設定は、プロバイダが Claude、effort がその値であり、`provider` パッケージはそこから Claude アダプタを構築する。5 つの値のそれぞれについて確かめる。構築したアダプタが送るリクエストの `model`・`output_config.effort`・`x-api-key` は、それぞれ環境変数の値である。
- **AC-24**: プロバイダが `claude` で、`ANTHROPIC_API_KEY` または `YT2COLUMN_CLAUDE_EFFORT` が未設定か空の場合、読み込みは `errors.Is(err, config.ErrMissing)` が真になるエラーになり、エラーから変数名を取り出せる。`DEEPSEEK_API_KEY` だけが設定されていても同じである。
- **AC-25**: `YT2COLUMN_CLAUDE_EFFORT` が 5 つの値以外（例: `High`、` high`、`high\n`、`none`、`min`、`medium-high`）の場合、プロバイダによらず読み込みは `errors.Is(err, config.ErrInvalid)` が真になるエラーになる。`YT2COLUMN_LLM_PROVIDER` が `claude` 以外の未知の値（例: `Claude`、`anthropic`）の場合も、`config.ErrInvalid` になる。どのエラーのメッセージにも、拒否した値（テストで埋め込んだ目印）は現れない。
- **AC-26**: プロバイダが `deepseek` のとき、5 つの値のいずれかの `YT2COLUMN_CLAUDE_EFFORT` と空でない `ANTHROPIC_API_KEY` が設定されていても、読み込みは成功し、`ANTHROPIC_API_KEY` が未設定のときと同じ DeepSeek アダプタが構築される。
- **AC-27**: 既存の DeepSeek の設定（`YT2COLUMN_LLM_PROVIDER` が未設定または `deepseek`）での読み込みとアダプタの構築の振る舞いは、本タスクの前と変わらない（既存のテストがそのまま通る）。

#### F-007: 実 API を使う統合テスト

実際の Claude の API を使ってアダプタの動作を確認する統合テストをリポジトリに含める。既定のテスト（`make test`）では実行せず、専用の Make ターゲットで手動実行する。

-   統合テストは `//go:build integration` で分離し、`make test`・`make test-ci` には含めない。
-   専用の Make ターゲット `make test-integration-claude` を追加する。このターゲットは、実 API を使うこと（料金が発生すること）を表示する。テスト結果のキャッシュを避けるため `-count=1` を付け、明示的な `-timeout` を設定する。既存の `test-integration`・`test-integration-deepseek` とは分ける。
-   API キーは、テスト専用の環境変数 `YT2COLUMN_TEST_ANTHROPIC_API_KEY` から読む。本番の CLI が読む `ANTHROPIC_API_KEY` は読まない。`YT2COLUMN_TEST_ANTHROPIC_API_KEY` が未設定または空の場合は、変数名を示すメッセージで `t.Skip` する。統合テストは、API キーをテストの出力に含めない。
-   モデル名と effort は、テスト専用の環境変数 `YT2COLUMN_TEST_CLAUDE_MODEL`・`YT2COLUMN_TEST_CLAUDE_EFFORT` から読む。`make test-integration-claude` は、これらが環境で設定されていなければ既定値（料金を抑える組み合わせ。値は設計で決める）を与えてエクスポートし、設定されていればその値を使う。テスト自身は既定値を持たない。未設定・空・effort が 5 つの値以外の場合は、変数名を示すメッセージで失敗する（5.1）。
-   送るプロンプトは、テストのために用意した短い固定の文字列とする。字幕・API キー・ローカルのファイルパス・利用者の個人情報を含めない（[security.md](../../dev/security.md) §4）。
-   確認する内容は、正常な生成と、出力トークン数の上限による打ち切りの検出である。

**Acceptance Criteria**:
- **AC-28**: 統合テストは既定の `make test` と `make test-ci` の対象に含まれず、これらの実行では Claude の API もネットワークも呼ばれない。
- **AC-29**: `YT2COLUMN_TEST_ANTHROPIC_API_KEY` が設定された環境で `make test-integration-claude` を実行すると、`-count=1` と明示的な `-timeout` を付けて統合テストが走り、少なくとも 1 件のテストが実際に実行されたこと（スキップやテスト結果のキャッシュではないこと）が `-v` 出力から確認できる。ターゲットは実 API を使うことを表示する。`YT2COLUMN_TEST_ANTHROPIC_API_KEY` が未設定の場合、統合テストは変数名を示すメッセージでスキップする。`ANTHROPIC_API_KEY` が設定されていても、`YT2COLUMN_TEST_ANTHROPIC_API_KEY` が未設定ならスキップし、`ANTHROPIC_API_KEY` の値を使わない。
- **AC-30**: 統合テストは、短い固定のプロンプトで `Generate` を呼び、エラーがなく、`Text` が空白文字以外を含み、`Model` が空でないことを検証する。また、生成が完了しないほど小さな `MaxOutputTokens` を指定した `Generate` が、`errors.Is(err, llm.ErrTruncated)` が真になるエラーを返すことを検証する。

#### F-008: テスト可能性

**Acceptance Criteria**:
- **AC-31**: アダプタと設定のユニットテストは、Claude の API もネットワーク上の外部ホストも呼ばない。アダプタは `net/http/httptest` のサーバーを送信先にして、F-001〜F-005 と 3.2 の各 AC を検証する。

### 3.2. 信頼できない入力の境界 (Untrusted Input Boundaries)

API の応答は信頼できない入力である。応答は次の規則に従って検証する。

**受理する形（HTTP ステータスが `200` の応答本文）:**

-   応答本文は、正しい UTF-8 のバイト列で、ちょうど 1 つの JSON オブジェクトである。トップレベルの値の後に空白以外のデータが続く入力は拒否する。
-   アダプタが値を読み取って検証や `GenerateResponse` の組み立てに使うメンバーを、以下「消費するメンバー」と呼ぶ。消費するメンバーは次のとおりである。
    -   トップレベルの `model`: 空でない JSON 文字列。
    -   トップレベルの `stop_reason`: JSON 文字列。値の扱いは F-003 の手順 3 による。
    -   トップレベルの `content`: JSON 配列。各要素は JSON オブジェクトである。空の配列は形としては受理し、F-003 の手順 4 で `llm.ErrEmptyResponse` とする。
    -   `content` の各要素の `type`: JSON 文字列。受理する値は `text`・`thinking`・`redacted_thinking` だけとし、それ以外の値（`tool_use`・`server_tool_use`・未知の値）は拒否する。リクエストで tool を指定しないため、それ以外のブロックは現れないはずであり、現れた場合にその内容を黙って捨てないためである。
    -   `type` が `text` の要素の `text`: JSON 文字列。空文字列は形としては受理し、F-003 の手順 4 で `llm.ErrEmptyResponse` とする。
-   `type` が `text` の要素は高々 1 つとする。2 つ以上ある応答は拒否する。どれを記事に使うか、どう連結するかを推測しないためである。事前調査（§5）で、通常の応答が `text` ブロックを 2 つ以上返すと分かった場合は、本規則を見直す。
-   `thinking`・`redacted_thinking` の要素は、`type` 以外のメンバーを消費しない。
-   消費するメンバーが、`null` である場合、期待する JSON の種類でない場合、欠落している場合は拒否する。
-   消費するメンバーが同じオブジェクト内で重複している場合は拒否する。どちらの値を採るかを推測しないためである。
-   トップレベルと `content` の各要素のオブジェクトは、**拡張可能**と宣言する。これらが持つ消費しないメンバー（`id`・`type`・`role`・`stop_sequence`・`stop_details`・`usage`・`thinking`・`signature`・`data`・`citations` など、および未知のメンバー）は、値の種類や重複を問わず無視して受理する。API は応答にメンバーを追加することがあり、それを拒否理由にしないためである。ただし、応答本文全体に対する検査（正しい UTF-8 であること、対になっていないサロゲートのエスケープを含まないこと、ちょうど 1 つのトップレベルの値であること）は、拡張可能であっても消費しないメンバーに及ぶ。
-   文字列として正しくエンコードされていない入力（不正な UTF-8 のバイト列、対になっていない UTF-16 サロゲートのエスケープ）は、デコーダが置換文字（U+FFFD）に置き換えて受理しうるが、これも補正とみなして拒否する。
-   応答本文のサイズには上限を設ける。上限値は `02_architecture.md` で固定する。上限を超える応答は拒否し、ちょうど上限の応答は受理する。`200` 以外の応答の本文を読む場合も、同じ上限を超えて読まない。

HTTP ステータス以外の応答ヘッダー（`Content-Type`・`request-id` など）は消費せず、検証しない。

**拒否時:** 上記に合致しない入力は、補正・正規化・切り詰めをせずに `ErrInvalidResponse` で拒否し、部分的な結果を返さない。標準ライブラリのデコーダが黙って置換・切り捨て・無視しうる入力（置換文字への置き換え、後続データの読み残し、重複したメンバーの後勝ち、`null` の文字列のゼロ値化など）も、受理する形に合致しないものとして拒否する。

| 境界 | 受理する形 | 拒否時の番兵 | 主な AC |
|---|---|---|---|
| HTTP ステータス | `200` | `ErrHTTPStatus` | AC-09・AC-11・AC-16 |
| 応答本文（`200`） | 3.2 | `ErrInvalidResponse` | AC-32〜AC-38 |
| 終了理由 | `end_turn` | `llm.ErrTruncated`・`llm.ErrUnexpectedFinishReason` | AC-12・AC-13 |
| 生成テキスト | 空白文字以外を含む `text` ブロック | `llm.ErrEmptyResponse` | AC-14・AC-15 |

各 AC に挙げた入力例は、要件レビューで確認した具体例であり、拒否すべき入力の網羅ではない。網羅は上の規則が担い、その具体化は設計で行う。

**Acceptance Criteria**:
- **AC-32**: JSON として不正な本文、空の本文、トップレベルが JSON オブジェクトでない本文（`null`・配列・文字列）、およびトップレベルの値の後に空白以外のデータが続く本文（例: 有効な応答の後に ` {}` や ` x` が続く本文）は、`errors.Is(err, ErrInvalidResponse)` が真になるエラーになる。
- **AC-33**: 消費するメンバー（`model`・`stop_reason`・`content`・要素の `type`・`text` ブロックの `text`）のいずれかが欠落している、`null` である（例: `"stop_reason":null`、`"text":null`）、または期待する JSON の種類でない（例: `"content":{}`、`"text":1`、`"type":1`）本文は、`errors.Is(err, ErrInvalidResponse)` が真になるエラーになり、`llm.ErrEmptyResponse` ではない。`model` が空文字列の本文も同様である。
- **AC-34**: `content` が JSON オブジェクト以外の要素（例: `[null]`、`["x"]`）を含む本文、`type` が `text`・`thinking`・`redacted_thinking` 以外の要素（`tool_use`・テスト用の未知の値）を含む本文、および `type` が `text` の要素を 2 つ以上含む本文は、`errors.Is(err, ErrInvalidResponse)` が真になるエラーになる。
- **AC-35**: 消費するメンバーが同じオブジェクト内で重複している本文（例: トップレベルに `model` が 2 回現れる、`text` ブロックに `text` が 2 回現れる）は、`errors.Is(err, ErrInvalidResponse)` が真になるエラーになる。
- **AC-36**: 不正な UTF-8 のバイト列、または対になっていないサロゲートのエスケープ（例: `"text":"\ud800"`）を含む本文は、その入力が消費するメンバーにあっても消費しないメンバー（例: `thinking` ブロックの `thinking`）にあっても、`errors.Is(err, ErrInvalidResponse)` が真になるエラーになり、部分的な結果を返さない。
- **AC-37**: 応答本文のサイズが `02_architecture.md` で固定した上限ちょうどの応答は受理され、上限を超える応答は `errors.Is(err, ErrInvalidResponse)` が真になるエラーになる。
- **AC-38**: 拡張可能と宣言したオブジェクトが消費しないメンバー（`id`・`usage`・`stop_details`・`thinking` ブロックの `signature`・テスト用の未知のメンバー）を含んでいても、それらは無視されて受理され、`GenerateResponse` が組み立てられる。テストは、`02_architecture.md` の作成時に記録した実 API の応答の形を元にした `testdata/` のサンプルで行う。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   API 呼び出しが実行時間を支配するため、本タスクにローカル処理の性能要件はない。

### 4.2. セキュリティ (Security)

-   [security.md](../../dev/security.md) §2（秘密情報）・§3（ネットワーク通信）・§4（LLM プロバイダへ送るデータ）・§6（信頼できないテキスト）に従う。
-   API キーは `Secret` 型で受け取り、`x-api-key` ヘッダー以外に出さない（F-002・F-005・AC-03・AC-21・AC-22）。
-   `http.DefaultClient` を使わず、タイムアウトを設定する（F-004・AC-18）。
-   応答本文の読み取りに上限を設ける（3.2・AC-37）。
-   リダイレクトに従わず、API キーを別の送信先へ送らない（F-002・AC-09）。
-   送るのは呼び出し元が与えたプロンプトと、構築時のモデル名・effort・出力トークン数の上限だけである。アダプタ自身はプロンプトに情報を付け加えない（AC-04・AC-07）。
-   応答は信頼できない入力として 3.2 の規則で検証する。`200` 以外の応答の本文はエラーに含めない（F-003・AC-11）。
-   推論過程（`thinking` ブロック）を生成テキストに含めない（AC-15）。
-   [security.md](../../dev/security.md) §4 に、Anthropic の API へ送るデータの扱い（保存期間、モデルの学習への利用の有無）を、Anthropic の公開文書を確認したうえで追記する。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   `context` のキャンセルとタイムアウトに従い、設定された時間内に呼び出し元へ戻る（F-004・AC-18・AC-19）。
-   空の応答、打ち切られた応答、`end_turn` 以外の終了理由の応答を成功として返さない（F-003・AC-12〜AC-14）。

### 4.4. 互換性 (Compatibility)

-   macOS と Linux でビルド・テストできること。
-   Go のバージョンは `go.mod` に従う。

### 4.5. 保守性 (Maintainability)

-   標準ライブラリ以外のモジュールを追加しないこと（`.golangci.yml` の depguard `deps` ルール）。
-   Claude 固有のリクエスト・応答の形（`output_config.effort`・`stop_reason` の値・`content` のブロックの種類・`x-api-key`・`anthropic-version` など）は `internal/llm/claude` の中に閉じ込め、`llm.GenerateRequest`・`llm.GenerateResponse` に Claude 固有の項目を追加しない。effort は構築時の入力であり、リクエストごとの値ではない。
-   effort の 5 つの値は、アダプタと `internal/config` で別々に列挙しない。一方の値の一覧を変えたとき、もう一方が追随しないことがないようにする（定義の置き場所は設計で決める）。
-   `ArticleWriter` を変更しない。
-   統合テストは既定のテストから分離し、専用の Make ターゲットで実行できること（F-007）。
-   Go のコメント・識別子・文字列リテラルは英語で書くこと。

## 5. 制約条件 (Constraints)

-   [project_overview.md](../../dev/project_overview.md) の「決定済みの方針」「前提・制約」に従う。
-   ユニットテストは Claude の API・ネットワーク上の外部ホストを呼ばない（AC-31）。統合テスト（F-007）は実 API を使うが、既定のテストには含めず、専用の Make ターゲットで実行する。
-   番兵 `ErrHTTPStatus`・`ErrInvalidResponse`・`ErrTransport` を `internal/llm/claude` に置くか、DeepSeek アダプタと共有するかは設計で決める（[design_handoff.md](design_handoff.md) H-01）。`ErrInvalidRequest`・`ErrTruncated`・`ErrUnexpectedFinishReason`・`ErrEmptyResponse` は `internal/llm` の既存の番兵を使う。
-   事前調査は `02_architecture.md` の作成時（承認を求める前）に行う。調査項目は、実 API の応答の形（消費するメンバー、実際に現れる消費しないメンバー、`thinking` ブロックの有無と位置、`text` ブロックの数）、記事の生成に要する出力トークン数（`max_tokens` の定数を決めるため。effort の値ごと）、`200` 以外の応答の形、および非ストリーミングで高い effort の生成が既存のタイムアウト内に終わるかどうかである。調査は実 API を呼び料金が発生するため、実施前に人間の明示的な承認を得る（CLAUDE.md の Tool Execution Safety）。調査結果は `02_architecture.md` に記録し、API キーなどの秘密情報を除いた実応答を `testdata/` に保存する（実応答に含まれる生成テキストは、調査用の固定のプロンプトから生成したものに限る）。
-   `02_architecture.md` は、[design_handoff.md](design_handoff.md) の各項目について、採用した手段または扱わない理由を記録する。

### 5.1. 他の文書との差分

-   `0003_deepseek_llm_client` の統合テストは、モデル名を本番の CLI と同じ `YT2COLUMN_MODEL` から読む。本タスクの統合テストは、テスト専用の `YT2COLUMN_TEST_CLAUDE_MODEL` から読む（F-007）。`YT2COLUMN_MODEL` はプロバイダ間で共有する変数であり、`.envrc` に DeepSeek のモデル名が設定されたまま `make test-integration-claude` を実行すると、Claude の API に DeepSeek のモデル名を送ってしまうためである。DeepSeek の統合テストの読み方は変更しない。
-   `0003_deepseek_llm_client` では、`MaxOutputTokens` が 0 のとき `max_tokens` を送らず API の既定に任せる。Messages API は `max_tokens` を必須とするため、本タスクでは 0 のときアダプタの定数を送る（F-002）。`llm.GenerateRequest` の「0 は上限をプロバイダの既定に任せる」という意味は変えず、Claude ではアダプタの定数がその「既定」にあたる。`llm.GenerateRequest` の doc コメントにこの解釈が読み取れない場合は、設計で doc コメントを補う。
-   `llm.GenerateResponse.ModelVersion` は、Claude では常に空文字列とする（F-003）。`ModelVersion` の doc コメントの「プロバイダが返さない場合は空」の規則に従う。
-   [project_overview.md](../../dev/project_overview.md) の設定の表を更新する（`YT2COLUMN_LLM_PROVIDER` の受理する値に `claude`、`ANTHROPIC_API_KEY` の説明、`YT2COLUMN_CLAUDE_EFFORT` の追加）。決定済みの方針は変更しない。

## 6. 用語集 (Glossary)

用語は [translation_glossary.md](../../translation_glossary.md) と統一する。本タスクで新たに使う用語は次のとおり。

-   **Claude アダプタ:** `internal/llm/claude` が提供する `llm.LLMClient` の実装。Anthropic の Messages API を呼ぶ。
-   **Messages API:** Anthropic の、system プロンプトとメッセージ列を受け取って生成結果を返す HTTP API。`POST https://api.anthropic.com/v1/messages`。
-   **effort:** Messages API の `output_config.effort`。推論の深さと出力トークンの量を調整する値で、`low`・`medium`・`high`・`xhigh`・`max` の 5 段階がある。省略時の既定値はモデルによって異なる。
-   **終了理由（stop reason）:** 応答の `stop_reason`。`end_turn` は自然な終了、`max_tokens` は出力トークン数の上限による打ち切り、`refusal` は安全性の判定による拒否である。
-   **コンテンツブロック（content block）:** 応答の `content` 配列の要素。`type` で種類を表す。生成テキストは `text` ブロック、推論過程は `thinking` ブロック（または `redacted_thinking` ブロック）として返る。
-   **番兵エラー（sentinel error）・消費するメンバー（consumed member）・拡張可能（extensible）・統合テスト（integration test）:** `0003_deepseek_llm_client` の [01_requirements.md](../0003_deepseek_llm_client/01_requirements.md) §6 と同じ意味。
-   **`ErrHTTPStatus` / `ErrInvalidResponse` / `ErrTransport`:** それぞれ、`200` 以外の HTTP ステータス・受理する形に合致しない応答本文・タイムアウトとキャンセル以外の通信の失敗を表す番兵エラー（名前は仮称で、配置とあわせて設計で確定する）。
