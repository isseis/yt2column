# 要件定義書：DeepSeek アダプタ（LLMClient 実装）

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-02 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`LLMClient` の初期実装である DeepSeek アダプタ（`internal/llm/deepseek`）に関する要件を定義する。DeepSeek アダプタは、system プロンプトと user プロンプトを DeepSeek 公式 API へ送り、生成テキストと、生成に使われたモデル名を返す。

**背景:**
yt2column は `URL → TranscriptSource → Transcript → ArticleWriter → Article → Publisher` というパイプラインで動作する（[project_overview.md](../../dev/project_overview.md)）。`ArticleWriter` は記事の生成に `LLMClient` を使う。`LLMClient` の interface と `GenerateRequest`・`GenerateResponse` の型、および秘密情報を保持する `Secret` 型は、パイプラインの骨格（#2、タスク `0001_pipeline_skeleton`）で定義済みである。本タスクはその interface を満たす最初の実装を作る。

LLM の初期実装には DeepSeek を使うことが決まっている。DeepSeek の API は OpenAI 互換の Chat Completions 形式（`POST https://api.deepseek.com/chat/completions`）であり、SDK を使わず標準ライブラリ（`net/http`・`encoding/json`）で呼び出す。

LLM はエラーを返さずに、空の応答や途中で打ち切られた応答を返すことがある。そうした応答を記事として投稿しないよう、アダプタはこれをエラーとして報告する。また、DeepSeek の現行モデルは thinking（推論）モードが既定で有効であり、推論過程（`reasoning_content`）が `content` とは別に返る。記事には `content` だけを使う。

モデル名 `deepseek-flash` は、提供元が指す実際のモデルを入れ替えられるエイリアスである。どのモデルで生成したかを追えるよう、応答に含まれるモデル名を返す。

**本書の記述範囲:** 本書は、観測できる振る舞い（何を送り、何を受理し、何をどの番兵エラーで拒否するか）を定める。番兵エラーとは、`errors.Is` で判別できる、あらかじめ定めたエラー値である。その振る舞いを実現する手段（標準ライブラリの API の使い方、型や関数の名前、上限値など）は設計（`02_architecture.md`）で決める。要件レビューの過程で挙がった実装上の注意点と、設計で決めるべき事項は [design_handoff.md](design_handoff.md) に申し送る。

対応 issue: #4

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `GenerateRequest` を DeepSeek の Chat Completions API へ送り、生成テキストとモデル名を `GenerateResponse` として返せる。
-   空の応答、途中で打ち切られた応答、`stop` 以外の終了理由の応答を、成功として返さずエラーとして報告する。
-   推論過程（`reasoning_content`）を生成テキストに含めない。
-   API キーを、送信先への `Authorization` ヘッダー以外には、エラーにもログにも、その他のどの出力にも漏らさない。
-   API の応答という信頼できない入力を、補正せずに検証する。不正な入力は対応する番兵エラーで拒否する（3.2）。
-   DeepSeek の API もネットワークも使わずに、アダプタの振る舞いをテストできる。

### 2.2. スコープ (In Scope)

-   `internal/llm/deepseek` パッケージと、`llm.LLMClient` を実装する型
-   構築時の入力（API キー・モデル名・タイムアウト）の検証
-   `GenerateRequest` の検証と、Chat Completions API へのリクエストの組み立て
-   応答の検証（HTTP ステータス、JSON の形、終了理由、空の本文）と `GenerateResponse` への変換
-   タイムアウト・キャンセル・通信失敗の報告
-   `httptest` を使うユニットテスト
-   実 API を使う手動実行の統合テスト（既定のテストからは除外）

### 2.3. スコープ外 (Out of Scope)

-   環境変数からの設定読み込み（`DEEPSEEK_API_KEY`・`YT2COLUMN_MODEL` など）と、プロバイダの選択（#6）。本タスクのアダプタは環境変数を読まず、API キー・モデル名・タイムアウトを構築時に受け取る。
-   タイムアウトの既定値（#6 が設定として与える）。
-   thinking モードの切り替え。初期は API の既定（thinking 有効）のままにし、リクエストに thinking の指定を含めない（[project_overview.md](../../dev/project_overview.md)「前提・制約」）。
-   `temperature` などのサンプリングパラメータ（thinking モードでは `temperature` が無視される）。
-   ストリーミング応答（`stream: true`）、tool calling、JSON 出力モード、複数候補の生成（`n`）。
-   リトライ。失敗は 1 回で呼び出し元へ報告する。
-   プロンプトの内容と、生成テキストの後処理（#5 の `ArticleWriter`）。
-   トークン使用量（`usage`）や料金の記録。
-   DeepSeek 以外のプロバイダ（Gemini・Claude）の実装。
-   CI での統合テストの実行（実 API と API キーが必要なため、ローカルでの手動実行に限る）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 構築

DeepSeek アダプタの値を、API キー・モデル名・タイムアウトから構築する。構築時の入力は補正せず、不正なら構築をエラーにする。

-   API キーは `secret.Secret` 型で受け取る。ゼロ値の `Secret` は拒否する。
-   モデル名は文字列で受け取る。空文字列、および前後に空白文字を含む値は拒否する。それ以外の値は検証せずそのまま API へ送る（モデル名は頻繁に更新されるため、既知の名前の一覧と照合しない）。
-   タイムアウトは正の値でなければならない。
-   送信先は `https://api.deepseek.com/chat/completions` とする。送信先をテストから差し替えられるようにする。差し替えの手段と、本番の送信先を利用者の設定から変えられないようにする方法は設計で決める（[design_handoff.md](design_handoff.md) H-06）。
-   構築したアダプタは `llm.LLMClient` を実装する。

**Acceptance Criteria**:
- **AC-01**: 有効な API キー・モデル名・正のタイムアウトから構築したアダプタは、`llm.LLMClient` として使える。
- **AC-02**: ゼロ値の `Secret`、空のモデル名、前後に空白文字を含むモデル名（例: `" deepseek-flash"`、`"deepseek-flash\n"`）、0 以下のタイムアウトのいずれかを与えた構築はエラーになり、アダプタを返さない。

#### F-002: リクエストの送信

`GenerateRequest` を検証し、Chat Completions API へ 1 回の HTTP リクエストとして送る。

-   `SystemPrompt` と `UserPrompt` は、どちらも空でなく、正しい UTF-8 の文字列でなければならない。`MaxOutputTokens` は 0 以上でなければならない。これらの条件に反するリクエストは、HTTP リクエストを送らずに `ErrInvalidRequest` で拒否する。不正な UTF-8 を拒否するのは、JSON への変換で置換文字（U+FFFD）に置き換えられ、送信される文字列が元と一致しなくなるためである。
-   HTTP メソッドは `POST`、本文は JSON とする。
-   本文には、構築時のモデル名、system メッセージ（`SystemPrompt`）と user メッセージ（`UserPrompt`）をこの順に並べたメッセージ列、および非ストリーミングの指定を含める。プロンプトは加工せず、そのまま送る。
-   `MaxOutputTokens` が正の値なら、それを出力トークン数の上限（`max_tokens`）として送る。0 なら上限を送らず、API の既定に任せる。
-   thinking モードの指定（`thinking`）は送らない。2.3 で除外した他のパラメータも送らない。
-   API キーは `Authorization` ヘッダーで `Bearer` トークンとして送る。リクエスト本文と URL には API キーを含めない。
-   リダイレクトには従わない。3xx の応答は F-003 の HTTP ステータスのエラーとして扱う。API キーを別の送信先へ送らないためである。
-   リトライしない。

**Acceptance Criteria**:
- **AC-03**: `Generate` は、送信先へ `POST` を 1 回だけ送る。リクエストの `Authorization` ヘッダーは `Bearer <API キー>` であり、リクエスト本文と URL には API キーが現れない。
- **AC-04**: リクエスト本文の JSON は、構築時のモデル名、`SystemPrompt` を内容とする system メッセージと `UserPrompt` を内容とする user メッセージをこの順に並べた 2 件のメッセージ列、および非ストリーミングの指定を含む。各プロンプトは送信前と同一の文字列である（前後の空白や改行も含めて変更されない）。
- **AC-05**: `MaxOutputTokens` が正の値のとき、リクエスト本文の `max_tokens` はその値である。0 のとき、リクエスト本文は `max_tokens` を含まない。
- **AC-06**: リクエスト本文は `thinking` を含まない。
- **AC-07**: `SystemPrompt` または `UserPrompt` が空のリクエスト、いずれかが不正な UTF-8 のバイト列を含むリクエスト、および `MaxOutputTokens` が負のリクエストは、`errors.Is(err, ErrInvalidRequest)` が真になるエラーになり、送信先へ HTTP リクエストを送らない。
- **AC-08**: 送信先が 3xx（例: `307` と `Location` ヘッダー）を返した場合、`Generate` はリダイレクト先へリクエストを送らず、HTTP ステータスのエラー（AC-10）を返す。

#### F-003: 応答の検証と変換

応答を検証し、生成テキストとモデル名を `GenerateResponse` として返す。検証は次の順序で行い、最初に失敗した手順の番兵を返す。

1.  HTTP ステータス: `200` 以外は `ErrHTTPStatus` とする。エラーからステータスコードを取り出せるようにする。応答本文はエラーに含めない（信頼できない入力であり、利用者の端末へそのまま出力しないため）。
2.  応答本文の形: 3.2 の受理する形に合致しない場合、およびサイズの上限を超える場合は `ErrInvalidResponse` とする。
3.  終了理由: `finish_reason` が `length` の場合は `ErrTruncated`、`stop` と `length` 以外の値（例: `content_filter`・`insufficient_system_resource`・未知の値）の場合は `ErrUnexpectedFinishReason` とする。
4.  `content`: 空文字列、または空白文字だけの場合は `ErrEmptyResponse` とする。

すべての手順を通過した場合に限り、次の `GenerateResponse` を返す。

-   `Text` は `content` の文字列を加工せずそのまま入れる。`reasoning_content` は含めない。
-   `Model` は応答の `model` の値を入れる。構築時のモデル名ではない（`deepseek-flash` のようなエイリアスが実際に指したモデルを記録するため）。

いずれの拒否時も、部分的な結果（それまでに得た `content` など）を返さない。

**Acceptance Criteria**:
- **AC-09**: `finish_reason` が `stop` で `content` が空白文字以外を含む応答に対し、`Generate` はエラーを返さず、`Text` が `content` と同一の文字列（前後の空白や改行も含めて変更されない）、`Model` が応答の `model` の値である `GenerateResponse` を返す。応答の `model` が構築時のモデル名と異なる場合も、`Model` は応答の値である。
- **AC-10**: HTTP ステータスが `200` 以外の応答（少なくとも `400`・`401`・`402`・`429`・`500`・`503`、および AC-08 の `307`）に対し、`errors.Is(err, ErrHTTPStatus)` が真になるエラーを返す。エラーからステータスコードを `errors.AsType` で取り出せ、その値は応答のステータスコードと一致する。エラーメッセージに応答本文に含まれていた文字列（テストで本文に埋め込んだ目印）は現れない。
- **AC-11**: `finish_reason` が `length` の応答は、`content` が空でなくても `errors.Is(err, ErrTruncated)` が真になるエラーになり、打ち切られた `content` を返さない（返る `GenerateResponse` はゼロ値である）。
- **AC-12**: `finish_reason` が `stop`・`length` 以外の文字列（`content_filter`・`insufficient_system_resource`・`tool_calls`・テスト用の未知の値）の応答は、`errors.Is(err, ErrUnexpectedFinishReason)` が真になるエラーになり、`ErrTruncated` ではない。
- **AC-13**: `finish_reason` が `stop` で、`content` が空文字列、または空白文字（半角空白・改行・タブ）だけの応答は、`errors.Is(err, ErrEmptyResponse)` が真になるエラーになる。
- **AC-14**: `content` と `reasoning_content` の両方を持つ応答に対し、`Text` は `content` だけであり、`reasoning_content` の文字列（テストで埋め込んだ目印）を含まない。`reasoning_content` だけがあり `content` が空の応答は `ErrEmptyResponse` になる。
- **AC-15**: 検証は F-003 の順序で行う。`finish_reason` が `length` で `content` が空の応答は `ErrTruncated` になり、`ErrEmptyResponse` ではない。`200` 以外のステータスで本文が不正な JSON の応答は `ErrHTTPStatus` になり、`ErrInvalidResponse` ではない。
- **AC-16**: 番兵 `ErrInvalidRequest`・`ErrHTTPStatus`・`ErrInvalidResponse`・`ErrTruncated`・`ErrUnexpectedFinishReason`・`ErrEmptyResponse`・`ErrTransport`（F-004）は、`errors.Is` で相互に区別できる。タイムアウトとキャンセルは `context.DeadlineExceeded` / `context.Canceled` で判別できる（AC-17・AC-18）。

#### F-004: 通信の失敗・タイムアウト・キャンセル

-   構築時のタイムアウトは、1 回の `Generate` の中で、HTTP リクエストの送信を始めてから応答本文の読み取りを終えるまでの全体に適用する。
-   タイムアウト（構築時のタイムアウト、または `ctx` の期限）で失敗した場合は、`errors.Is(err, context.DeadlineExceeded)` が真になるエラーを返す。`ctx` のキャンセルで失敗した場合は、`errors.Is(err, context.Canceled)` が真になるエラーを返す。
-   `ctx` が既にキャンセルされている場合は、HTTP リクエストを送らない。
-   接続の失敗など、タイムアウトとキャンセル以外の通信の失敗は `ErrTransport` とする。

**Acceptance Criteria**:
- **AC-17**: 応答を返さずに待ち続ける送信先に対し、構築時のタイムアウトを短く設定した `Generate` は、構築時のタイムアウトが経過してから `02_architecture.md` で固定した猶予時間以内に戻り、`errors.Is(err, context.DeadlineExceeded)` が真になるエラーを返す。ヘッダーを返した後に本文の送信を止める送信先に対しても同様である。
- **AC-18**: `Generate` の実行中に `ctx` をキャンセルすると、`Generate` は戻り、`errors.Is(err, context.Canceled)` が真になるエラーを返す。呼び出し前に `ctx` が既にキャンセルされている場合は、送信先へ HTTP リクエストを送らずに同じエラーを返す。
- **AC-19**: 接続できない送信先（例: 閉じた `httptest` サーバーのアドレス）に対し、`errors.Is(err, ErrTransport)` が真になるエラーを返し、`context.DeadlineExceeded` でも `context.Canceled` でもない。

#### F-005: 秘密情報の保護

API キーは、送信先への `Authorization` ヘッダー以外のどこにも現れない。

-   `Generate` が返すエラー、およびアダプタの値を `fmt`・`log/slog`・`encoding/json` で出力した結果に、API キーを含めない。
-   `*url.Error` など、標準ライブラリのエラーをラップするときも、リクエストのヘッダーや API キーを含めない。

**Acceptance Criteria**:
- **AC-20**: AC-07・AC-08・AC-10・AC-11・AC-12・AC-13・AC-17・AC-18・AC-19 の各エラーケース、および 3.2 の各拒否ケースについて、返るエラーを `Error()`、`%v`、`%+v`、`%#v` で文字列にした結果に、テストで使った API キーの文字列が現れない。
- **AC-21**: 構築したアダプタの値を `fmt`（`%v`・`%+v`・`%#v`）、`log/slog`（TextHandler・JSONHandler）、`encoding/json` で出力した結果に、API キーの文字列が現れない。

#### F-006: 実 API を使う統合テスト

実際の DeepSeek API を使ってアダプタの動作を確認する統合テストをリポジトリに含める。既定のテスト（`make test`）では実行せず、専用の Make ターゲットで手動実行する。

-   統合テストは `//go:build integration` で分離し、`make test`・`make test-ci` には含めない。
-   専用の Make ターゲット `make test-integration-deepseek` を追加する。このターゲットは、実 API を使うこと（料金が発生すること）を表示する。テスト結果のキャッシュを避けるため `-count=1` を付け、明示的な `-timeout` を設定する。
-   既存の `make test-integration`（実 `yt-dlp` を使う）とは分ける。`yt-dlp` の統合テストを実行するたびに API の料金が発生しないようにするためである。
-   API キーは環境変数 `DEEPSEEK_API_KEY` から読む。未設定または空の場合は、変数名を示すメッセージで `t.Skip` する（CI では実行しないため）。統合テストは、API キーをテストの出力に含めない。
-   モデル名は環境変数 `YT2COLUMN_TEST_DEEPSEEK_MODEL` から読む。`make test-integration-deepseek` は既定値を Make 変数としてエクスポートし、実行時に差し替え可能にする。テスト自身は既定値を持たない（モデル名をコードにハードコードしないため）。未設定または空の場合は、変数名を示すメッセージで失敗する。
-   送るプロンプトは、テストのために用意した短い固定の文字列とする。字幕・API キー・ローカルのファイルパス・利用者の個人情報を含めない（[security.md](../../dev/security.md) §4）。
-   確認する内容は、正常な生成と、出力トークン数の上限による打ち切りの検出である。

2.3 との関係: 統合テストが環境変数を読むのはテストのためであり、アダプタ自身は環境変数を読まない。

**Acceptance Criteria**:
- **AC-22**: 統合テストは既定の `make test` と `make test-ci` の対象に含まれず、これらの実行では DeepSeek の API もネットワークも呼ばれない。
- **AC-23**: `DEEPSEEK_API_KEY` が設定された環境で `make test-integration-deepseek` を実行すると、`-count=1` と明示的な `-timeout` を付けて統合テストが走り、少なくとも 1 件のテストが実際に実行されたこと（スキップやテスト結果のキャッシュではないこと）が `-v` 出力から確認できる。ターゲットは実 API を使うことを表示する。`DEEPSEEK_API_KEY` が未設定の場合、統合テストは変数名を示すメッセージでスキップする。
- **AC-24**: 統合テストは、短い固定のプロンプトで `Generate` を呼び、エラーがなく、`Text` が空白文字以外を含み、`Model` が空でないことを検証する。また、生成が完了しないほど小さな `MaxOutputTokens` を指定した `Generate` が、`errors.Is(err, ErrTruncated)` が真になるエラーを返すことを検証する。

#### F-007: テスト可能性

**Acceptance Criteria**:
- **AC-25**: アダプタのユニットテストは、DeepSeek の API もネットワーク上の外部ホストも呼ばない。`net/http/httptest` のサーバーを送信先にして、F-001〜F-005 と 3.2 の各 AC を検証する。

### 3.2. 信頼できない入力の境界 (Untrusted Input Boundaries)

API の応答は信頼できない入力である。応答は次の規則に従って検証する。

**受理する形（HTTP ステータスが `200` の応答本文）:**

-   応答本文は、正しい UTF-8 のバイト列で、ちょうど 1 つの JSON オブジェクトである。トップレベルの値の後に空白以外のデータが続く入力は拒否する。
-   アダプタが値を読み取って検証や `GenerateResponse` の組み立てに使うメンバーを、以下「消費するメンバー」と呼ぶ。消費するメンバーは次のとおりである。
    -   トップレベルの `model`: 空でない JSON 文字列。
    -   トップレベルの `choices`: ちょうど 1 要素の JSON 配列。要素は JSON オブジェクトである（リクエストで候補数を指定しないため、API は 1 件を返す）。
    -   `choices[0].finish_reason`: JSON 文字列。値の扱いは F-003 の手順 3 による。
    -   `choices[0].message`: JSON オブジェクト。
    -   `choices[0].message.content`: JSON 文字列。空文字列は形としては受理し、F-003 の手順 4 で `ErrEmptyResponse` とする。
-   消費するメンバーが欠落している場合、`null` である場合、期待する JSON の種類でない場合は拒否する。
-   消費するメンバーが同じオブジェクト内で重複している場合は拒否する。どちらの値を採るかを推測しないためである。
-   トップレベル、`choices` の要素、`message` の各オブジェクトは、**拡張可能**と宣言する。これらが持つ消費しないメンバー（`id`・`object`・`created`・`usage`・`system_fingerprint`・`index`・`logprobs`・`reasoning_content`・`role` など、および未知のメンバー）は、値の種類や重複を問わず無視して受理する。OpenAI 互換の API は応答にメンバーを追加することがあり、それを拒否理由にしないためである。ただし、応答本文全体に対する検査（正しい UTF-8 であること、ちょうど 1 つのトップレベルの値であること）は、拡張可能であっても消費しないメンバーに及ぶ。
-   文字列として正しくエンコードされていない入力（不正な UTF-8 のバイト列、対になっていない UTF-16 サロゲートのエスケープ）は、デコーダが置換文字（U+FFFD）に置き換えて受理しうるが、これも補正とみなして拒否する。
-   応答本文のサイズには上限を設ける。上限値は `02_architecture.md` で固定する。上限を超える応答は拒否し、ちょうど上限の応答は受理する。`200` 以外の応答の本文を読む場合も、同じ上限を超えて読まない。

HTTP ステータス以外の応答ヘッダー（`Content-Type` など）は消費せず、検証しない。応答本文の形は上記の規則だけで判定する。

**拒否時:** 上記に合致しない入力は、補正・正規化・切り詰めをせずに `ErrInvalidResponse` で拒否し、部分的な結果を返さない。標準ライブラリのデコーダが黙って置換・切り捨て・無視しうる入力（置換文字への置き換え、後続データの読み残し、重複したメンバーの後勝ち、`null` の文字列のゼロ値化など）も、受理する形に合致しないものとして拒否する。そうした入力の洗い出しと、その検出手段の決定は設計で行う（[design_handoff.md](design_handoff.md) H-05）。

| 境界 | 受理する形 | 拒否時の番兵 | 主な AC |
|---|---|---|---|
| HTTP ステータス | `200` | `ErrHTTPStatus` | AC-08・AC-10・AC-15 |
| 応答本文（`200`） | 3.2 | `ErrInvalidResponse` | AC-26〜AC-31 |
| 終了理由 | `stop` | `ErrTruncated`・`ErrUnexpectedFinishReason` | AC-11・AC-12 |
| `content` | 空白文字以外を含む文字列 | `ErrEmptyResponse` | AC-13・AC-14 |

各 AC に挙げた入力例は、要件レビューで確認した具体例であり、拒否すべき入力の網羅ではない。網羅は上の規則が担い、その具体化は設計で行う。

**Acceptance Criteria**:
- **AC-26**: JSON として不正な本文、空の本文、トップレベルが JSON オブジェクトでない本文（`null`・配列・文字列）、およびトップレベルの値の後に空白以外のデータが続く本文（例: 有効な応答の後に ` {}` や ` x` が続く本文）は、`errors.Is(err, ErrInvalidResponse)` が真になるエラーになる。
- **AC-27**: 消費するメンバー（`model`・`choices`・`finish_reason`・`message`・`content`）のいずれかが欠落している、`null` である（例: `"content":null`、`"finish_reason":null`）、または期待する JSON の種類でない（例: `"content":1`、`"choices":{}`）本文は、`errors.Is(err, ErrInvalidResponse)` が真になるエラーになり、`ErrEmptyResponse` ではない。`model` が空文字列の本文も同様である。
- **AC-28**: `choices` が空の配列、2 要素以上の配列、または JSON オブジェクト以外の要素（例: `[null]`）を含む本文は、`errors.Is(err, ErrInvalidResponse)` が真になるエラーになる。
- **AC-29**: 消費するメンバーが同じオブジェクト内で重複している本文（例: `message` に `content` が 2 回現れる、トップレベルに `model` が 2 回現れる）は、`errors.Is(err, ErrInvalidResponse)` が真になるエラーになる。
- **AC-30**: 不正な UTF-8 のバイト列、または対になっていないサロゲートのエスケープ（例: `"content":"\ud800"`）を含む本文は、その入力が消費するメンバーにあっても消費しないメンバー（例: `reasoning_content`）にあっても、`errors.Is(err, ErrInvalidResponse)` が真になるエラーになり、部分的な結果を返さない。
- **AC-31**: 応答本文のサイズが `02_architecture.md` で固定した上限ちょうどの応答は受理され、上限を超える応答は `errors.Is(err, ErrInvalidResponse)` が真になるエラーになる。
- **AC-32**: 拡張可能と宣言したオブジェクトが消費しないメンバー（`id`・`usage`・`system_fingerprint`・`logprobs`・`reasoning_content`・テスト用の未知のメンバー）を含んでいても、それらは無視されて受理され、`GenerateResponse` が組み立てられる。テストは、`02_architecture.md` の作成時に記録した実 API の応答の形を元にした `testdata/` のサンプルで行う。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   API 呼び出しが実行時間を支配するため、本タスクにローカル処理の性能要件はない。

### 4.2. セキュリティ (Security)

-   [security.md](../../dev/security.md) §2（秘密情報）・§3（ネットワーク通信）・§4（LLM プロバイダへ送るデータ）・§6（信頼できないテキスト）に従う。
-   API キーは `Secret` 型で受け取り、`Authorization` ヘッダー以外に出さない（F-002・F-005・AC-03・AC-20・AC-21）。
-   `http.DefaultClient` を使わず、タイムアウトを設定する（F-004・AC-17）。
-   応答本文の読み取りに上限を設ける（3.2・AC-31）。
-   リダイレクトに従わず、API キーを別の送信先へ送らない（F-002・AC-08）。
-   送るのは呼び出し元が与えたプロンプトとモデル名だけである。アダプタ自身はプロンプトに情報を付け加えない（AC-04）。
-   応答は信頼できない入力として 3.2 の規則で検証する。`200` 以外の応答の本文はエラーに含めない（F-003・AC-10）。
-   推論過程（`reasoning_content`）を生成テキストに含めない（AC-14）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   `context` のキャンセルとタイムアウトに従い、設定された時間内に呼び出し元へ戻る（F-004・AC-17・AC-18）。
-   空の応答、打ち切られた応答、`stop` 以外の終了理由の応答を成功として返さない（F-003・AC-11〜AC-13）。

### 4.4. 互換性 (Compatibility)

-   macOS と Linux でビルド・テストできること。
-   Go のバージョンは `go.mod` に従う。

### 4.5. 保守性 (Maintainability)

-   標準ライブラリ以外のモジュールを追加しないこと（`.golangci.yml` の depguard `deps` ルール）。
-   DeepSeek 固有のリクエスト・応答の形（`thinking`・`reasoning_content`・`finish_reason` の値など）は `internal/llm/deepseek` の中に閉じ込め、`llm.GenerateRequest`・`llm.GenerateResponse` に DeepSeek 固有の項目を追加しない。
-   統合テストは既定のテストから分離し、専用の Make ターゲットで実行できること（F-006）。
-   Go のコメント・識別子・文字列リテラルは英語で書くこと。

## 5. 制約条件 (Constraints)

-   [project_overview.md](../../dev/project_overview.md) の「決定済みの方針」「前提・制約」に従う。
-   ユニットテストは DeepSeek の API・ネットワーク上の外部ホストを呼ばない（AC-25）。統合テスト（F-006）は実 API を使うが、既定のテストには含めず、専用の Make ターゲットで実行する。
-   環境変数の読み込みは #6 の責務とする。本タスクのアダプタは、API キー・モデル名・タイムアウトを構築時に受け取る。
-   番兵エラーを `internal/llm`（プロバイダ共通）と `internal/llm/deepseek` のどちらに置くかは設計で決める。打ち切り・空の応答はプロバイダに依存しない概念であり、後続のプロバイダや `ArticleWriter`（#5）が同じ番兵で判別できる配置を検討すること（[design_handoff.md](design_handoff.md) H-09）。
-   事前調査は `02_architecture.md` の作成時（承認を求める前）に行う。調査項目は、実 API の応答の形（消費するメンバーと、実際に現れる消費しないメンバー）、thinking モードで `max_tokens` が推論過程のトークンを含むかどうか、`200` 以外の応答の形である。調査は実 API を呼ぶため、実施前に人間の明示的な承認を得る（CLAUDE.md の Tool Execution Safety）。調査結果は `02_architecture.md` に記録し、API キーなどの秘密情報を除いた実応答を `testdata/` に保存する。
-   `02_architecture.md` は、[design_handoff.md](design_handoff.md) の各項目について、採用した手段または扱わない理由を記録する。

### 5.1. 他の文書との差分

-   `0002_ytdlp_transcript_source` の統合テストは、対象の指定（環境変数）が欠けている場合にスキップせず失敗する。本タスクの統合テストは、issue #4 の完了条件に従い、API キーが未設定の場合はスキップする（F-006）。API キーは秘密情報であり、利用者ごとに設定の有無が異なるためである。モデル名の未設定は Make ターゲットが既定値を与えるため、`0002` と同じく失敗とする。
-   `llm.GenerateRequest` の `MaxOutputTokens` は、`0001_pipeline_skeleton` では値の意味を定めていなかった。本書では、0 を「上限を指定せず、プロバイダの既定に任せる」、負の値を不正と定める（F-002）。後続のプロバイダも同じ意味で扱えるよう、`llm.GenerateRequest` の doc コメントにこの意味を書く。
-   [project_overview.md](../../dev/project_overview.md) の決定済みの方針は変更しない。

## 6. 用語集 (Glossary)

用語は [translation_glossary.md](../../translation_glossary.md) と統一する。本タスクで新たに使う用語は次のとおり。

-   **DeepSeek アダプタ:** `internal/llm/deepseek` が提供する `llm.LLMClient` の実装。DeepSeek 公式 API の Chat Completions API を呼ぶ。
-   **Chat Completions API:** OpenAI 互換の、メッセージ列を受け取って生成結果を返す HTTP API。DeepSeek では `POST https://api.deepseek.com/chat/completions`。
-   **終了理由（finish reason）:** 応答の `choices[].finish_reason`。生成が終わった理由を表す。`stop` は自然な終了、`length` は出力トークン数の上限による打ち切りである。
-   **thinking モード:** DeepSeek の推論モード。現行モデルでは既定で有効で、推論過程を `reasoning_content` として `content` とは別に返す。
-   **推論過程（reasoning content）:** thinking モードで返る `reasoning_content`。記事には含めない。
-   **番兵エラー（sentinel error）:** `errors.Is` で判別できる、あらかじめ定めたエラー値。
-   **消費するメンバー（consumed member）:** アダプタが値を読み取って検証や `GenerateResponse` の組み立てに使う、応答の JSON オブジェクトのメンバー（3.2）。
-   **拡張可能（extensible）:** 消費しないメンバーを、値の種類や重複を問わず無視して受理すると要件で宣言したオブジェクトの性質。
-   **`ErrInvalidRequest` / `ErrHTTPStatus` / `ErrInvalidResponse` / `ErrTruncated` / `ErrUnexpectedFinishReason` / `ErrEmptyResponse` / `ErrTransport`:** それぞれ、不正な `GenerateRequest`・`200` 以外の HTTP ステータス・受理する形に合致しない応答本文・出力トークン数の上限による打ち切り・`stop` と `length` 以外の終了理由・空の本文・タイムアウトとキャンセル以外の通信の失敗を表す番兵エラー（名前は仮称で、設計で確定する）。
-   **統合テスト（integration test）:** 実際の外部サービスを使うテスト。本タスクでは DeepSeek の API を使い、`make test-integration-deepseek` で手動実行する。
