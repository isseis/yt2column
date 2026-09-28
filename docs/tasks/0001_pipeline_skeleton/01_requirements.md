# 要件定義書：パイプラインの骨格

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-09-28 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** このドキュメントは、yt2column のパイプラインの骨格（段階間で受け渡す共通データ型、各段階の interface、段階を順に呼び出すオーケストレーション、テスト用の fake、秘密情報を保持する型）に関する要件を定義する。

**背景:**
yt2column は `URL → TranscriptSource → Transcript → ArticleWriter → Article → Publisher` というパイプラインで動作する（[project_overview.md](../../dev/project_overview.md)）。字幕取得（#3）・DeepSeek アダプタ（#4）・記事生成（#5）・Slack 投稿（#7）は、それぞれ独立したタスクとして並行して進めたい。そのためには、段階間の受け渡し方と各段階の責務を先に固め、各タスクが fake を相手に単体で開発・テストできる状態を作る必要がある。

また、API キーと Webhook URL はログやエラーメッセージに漏れてはならない（[security.md](../../dev/security.md) §2）。この保証を実装者の注意に頼らず型で担保するため、秘密情報を保持する型を、それを受け取る #4・#7 より先に本タスクで用意する。

対応 issue: #2

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   後続タスク（#3・#4・#5・#7）が、本タスクで定めた型と interface だけに依存して並行開発できる
-   パイプライン全体の制御（段階の実行順、失敗時の停止、キャンセル）を、実際の外部サービスなしにテストできる
-   秘密情報が文字列化・ログ出力・JSON 化のどの経路でも表に出ないことを、型によって保証する

### 2.2. スコープ (In Scope)

-   段階間で受け渡す共通データ型（字幕、LLM への要求と応答、記事）
-   各段階の interface（`TranscriptSource`、`LLMClient`、`ArticleWriter`、`Publisher`）
-   各段階を順に呼び出すパイプライン（`internal/pipeline`）
-   各 interface のテスト用 fake
-   秘密情報を保持する型

### 2.3. スコープ外 (Out of Scope)

-   各段階の実装（yt-dlp による字幕取得は #3、DeepSeek アダプタは #4、`ArticleWriter` の実装は #5、`FilePublisher` と CLI は #6、Slack 投稿は #7）
-   環境変数からの設定の読み込みと検証（#6）
-   プロンプトの内容（#5・#8）
-   字幕のタイムスタンプを使った機能（#10）。本タスクではタイムスタンプを保持できるようにするだけで、使う機能は作らない
-   リトライ、並列実行、進捗表示

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 共通データ型

段階間で受け渡すデータを表す型を定義する。

-   **字幕（`Transcript`）**: 動画の字幕本文と、動画のメタ情報を保持する。本文は字幕のセグメントの並びとして保持し、各セグメントは動画内の開始時刻（ミリ秒）と文字列を持つ。メタ情報は、動画 ID・動画 URL・タイトル・チャンネル名・概要欄を含む。
-   **LLM への要求（`GenerateRequest`）と応答**: 要求は system プロンプト・user プロンプト・最大出力トークン数など、プロバイダ共通の最小限の項目だけを持つ。応答は生成されたテキストと、実際に生成に使われたモデル名を持つ。
-   **記事（`Article`）**: タイトル・Markdown 形式の本文・出典（元動画の URL）と、生成に使われたモデル名を保持する。

**Acceptance Criteria**:
- **AC-01**: `Transcript` は、セグメントごとの開始時刻（ミリ秒）と文字列を、字幕の出現順を保ったまま保持できる。
- **AC-02**: `Transcript` は、動画 ID・動画 URL・タイトル・チャンネル名・概要欄を保持できる。
- **AC-03**: LLM の応答は、生成されたテキストとともに、実際に生成に使われたモデル名を呼び出し元へ返せる。
- **AC-04**: `Article` は、タイトル・Markdown 本文・出典 URL・生成に使われたモデル名を保持できる。
- **AC-05**: 共通データ型と interface のシグネチャは、LLM プロバイダの SDK の型や、プロバイダ固有の項目（DeepSeek の `thinking` パラメータ、`reasoning_content` など）を含まない。

#### F-002: 段階の interface

パイプラインの各段階を interface として定義する。すべての段階は `context.Context` を受け取り、キャンセルとタイムアウトに従う。

-   **`TranscriptSource`**: 動画 URL を受け取り、`Transcript` を返す。
-   **`LLMClient`**: `GenerateRequest` を受け取り、LLM の応答を返す。
-   **`ArticleWriter`**: `Transcript` を受け取り、`Article` を返す。
-   **`Publisher`**: `Article` を受け取り、投稿先へ出力する。

**Acceptance Criteria**:
- **AC-06**: 4 つの interface（`TranscriptSource`、`LLMClient`、`ArticleWriter`、`Publisher`）が定義され、それぞれ上記の入力を受け取り、上記の出力とエラーを返す。
- **AC-07**: 4 つの interface のすべてのメソッドが、第 1 引数に `context.Context` を受け取る。
- **AC-08**: 各 interface の定義に、その実装が満たすべき契約（失敗時にエラーを返すこと、空の結果を正常な結果として返さないこと）が英語のドキュメントコメントとして書かれている。

#### F-003: パイプラインの実行

動画 URL を受け取り、`TranscriptSource` → `ArticleWriter` → `Publisher` の順に各段階を呼び出す。パイプラインは各段階の実装を interface として受け取り、具体的な実装に依存しない。

**Acceptance Criteria**:
- **AC-09**: すべての段階が成功した場合、各段階は `TranscriptSource` → `ArticleWriter` → `Publisher` の順に 1 回ずつ呼ばれ、前の段階の出力がそのまま次の段階の入力として渡される。
- **AC-10**: すべての段階が成功した場合、パイプラインは投稿した `Article` を呼び出し元へ返す。
- **AC-11**: いずれかの段階が失敗した場合、パイプラインはそれ以降の段階を呼ばずにエラーを返す。特に、`TranscriptSource` または `ArticleWriter` が失敗した場合、`Publisher` は呼ばれない。
- **AC-12**: パイプラインが返すエラーから、呼び出し元はどの段階で失敗したかを `errors.Is` または `errors.AsType` で判別でき、かつ段階が返した元のエラーも `errors.Is` で辿れる。
- **AC-13**: 段階と段階の間で `context` がキャンセルされていた場合、パイプラインは次の段階を呼ばずに、キャンセルを示すエラー（`errors.Is(err, context.Canceled)` が真）を返す。
- **AC-14**: 段階の実装が設定されていない（nil の）パイプラインは構築できず、構築時にエラーになる。

#### F-004: テスト用 fake

後続タスクとパイプライン自体のテストで使う fake を、4 つの interface それぞれについて用意する。

**Acceptance Criteria**:
- **AC-15**: 4 つの interface それぞれに fake があり、テストから戻り値とエラーを指定できる。
- **AC-16**: 各 fake は、呼ばれた回数と受け取った引数を記録し、テストから参照できる。
- **AC-17**: fake は本番用のバイナリに含まれない（`//go:build test` でのみビルドされる）。

#### F-005: 秘密情報型

API キーや Webhook URL などの秘密情報を保持する型（仮称 `Secret`）を定義する。

**Acceptance Criteria**:
- **AC-18**: `Secret` を `fmt` パッケージで出力した場合、書式指定子（`%s`、`%v`、`%+v`、`%#v`、`%q`）にかかわらず、元の値ではなく `[REDACTED]` を含む固定文字列が出力される。`Secret` を含む構造体を `%v`・`%+v`・`%#v` で出力した場合も同様に元の値は出力されない。
- **AC-19**: `Secret` を `log/slog` の属性として出力した場合、元の値ではなく `[REDACTED]` が出力される。
- **AC-20**: `Secret` を `encoding/json` でエンコードした場合（`Secret` を含む構造体のエンコードを含む）、元の値ではなく `"[REDACTED]"` が出力される。
- **AC-21**: 元の値は、秘密情報を取り出すことが名前から明らかな専用のメソッドを明示的に呼んだ場合にだけ取得できる。
- **AC-22**: 空文字列から `Secret` を生成しようとするとエラーになる。
- **AC-23**: `Secret` のゼロ値は、専用のメソッドで値を取り出そうとした場合に、空文字列を有効な秘密情報として返さない（エラーまたは判別可能な結果を返す）。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   該当なし（パイプラインの実行時間は外部コマンドと LLM API が支配的で、本タスクの範囲には性能上の要件はない）

### 4.2. セキュリティ (Security)
-   秘密情報の非開示は F-005 の受け入れ基準で担保する
-   パイプラインが返すエラーは、段階が返したエラーをラップするだけで、秘密情報を新たに付け加えない

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   パイプラインは `context` のキャンセルに従う（AC-13）

### 4.4. 互換性 (Compatibility)
-   macOS と Linux でビルド・テストできること
-   Go のバージョンは `go.mod` に従う

### 4.5. 保守性 (Maintainability)
-   LLM プロバイダを追加しても、共通データ型・interface・パイプラインを変更せずに済むこと（AC-05）
-   標準ライブラリ以外のモジュールを追加しないこと（`.golangci.yml` の depguard `deps` ルール）
-   Go のコメント・識別子・文字列リテラルは英語で書くこと

## 5. 制約条件 (Constraints)

-   [project_overview.md](../../dev/project_overview.md) の「決定済みの方針」「前提・制約」に従う。
-   ユニットテストは外部コマンド・外部 API・Webhook を呼ばない。
-   型と interface を置くパッケージの分け方は設計（`02_architecture.md`）で決める。循環 import が生じないこと。

### 5.1. project_overview.md との差分

-   [project_overview.md](../../dev/project_overview.md) の `LLMClient` の例は、戻り値を `(string, error)` としている。本要件では、生成に使われたモデル名を記録するため（#4、`deepseek-flash` はエイリアスで指すモデルが変わる）、応答をテキストとモデル名を持つ型で返す（AC-03）。本タスクの完了時に project_overview.md の例を更新する。

## 6. 用語集 (Glossary)

用語は [translation_glossary.md](../../translation_glossary.md) と統一する。本タスクで新たに使う用語は次のとおり。

-   **段階（stage）:** パイプラインを構成する処理の単位。`TranscriptSource`・`ArticleWriter`・`Publisher` の 3 つ。`LLMClient` は `ArticleWriter` の内部で使われる部品であり、パイプラインが直接呼ぶ段階ではない。
-   **fake:** テストで本物の実装の代わりに使う、戻り値を指定でき呼び出しを記録する実装。
-   **秘密情報（secret）:** API キーや Webhook URL など、ログやエラーメッセージに出してはならない値。
