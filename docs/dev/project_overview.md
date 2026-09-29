# yt2column プロジェクト概要

YouTube 動画の URL を受け取り、その文字起こしを元に LLM で「雑誌コラム記事風」の文章を生成し、Markdown として Slack などへ Webhook 経由で投稿する CLI ツール。

本書はプロジェクト全体の決定済み方針をまとめたもの。個々の機能は `docs/tasks/` 以下のタスクで要件 → 設計 → 実装計画の順に具体化する（[requirements_process.md](developer_guide/requirements_process.md)）。本書の方針を変更する場合は、タスクの要件定義書でその旨を明記し、本書も更新する。

## 決定済みの方針

- **言語: Go**。依存関係を最小限にし、シングルバイナリで配布する。
- **実行形式: ローカル実行の CLI**。将来的に Slack Bot や定期実行へ拡張する可能性があるので、コアロジックは CLI から分離したパッケージとして実装する。
- **字幕取得: 外部コマンドの `yt-dlp` に委譲する**。YouTube の非公式な仕様に依存する最も壊れやすい部分を、保守が活発なツールに任せるため。Go で字幕取得を自前実装しないこと。
- **LLM: 初期実装は DeepSeek**（DeepSeek 公式 API を直接契約して使う）。API は OpenAI 互換の Chat Completions 形式（`POST https://api.deepseek.com/chat/completions`）で単純なため、SDK は使わず標準ライブラリ（`net/http`・`encoding/json`）で呼び出す。
  - OpenCode Go 経由の利用は採用しない。利用条件が「コーディングエージェントのトラフィック」を前提としており、本ツールの用途に合わないため。
- **LLM プロバイダは差し替え可能にする**。将来 Gemini（`google.golang.org/genai`）や Claude（`github.com/anthropics/anthropic-sdk-go`）などを追加する前提。プロバイダとモデル名は設定で切り替える。Gemini を追加する場合、旧 SDK の `github.com/google/generative-ai-go` はレガシーなので使わないこと。
- **LangChain 相当のフレームワークや、複数プロバイダを束ねる抽象化ライブラリは使わない**。自前の薄い interface で十分。
- **外部ライブラリは必要最小限にする**。HTTP・JSON・CLI 引数・設定は標準ライブラリで書く。追加する場合は理由を明示すること。

## パイプライン

```
URL → TranscriptSource → Transcript → ArticleWriter → Article → Publisher
```

各段階は interface で抽象化し、実装を差し替えられるようにする。

- `TranscriptSource`: 初期実装は `YtDlpSource`。
  - 例: `yt-dlp --skip-download --write-subs --write-auto-subs --sub-langs ja --sub-format json3 --write-info-json -o "<cache>/%(id)s" <URL>`
  - json3 の `events[].segs[].utf8` を連結して本文にする。`tStartMs` も保持する（将来、見出しごとに動画の該当時刻へのリンクを付けるため）。
  - info.json からタイトル・チャンネル名・概要欄を取り出し、メタ情報として `Transcript` に含める。
  - 手動字幕と自動字幕が両方ある場合の挙動（どちらが優先されるか、出力ファイル名）は `02_architecture.md` の作成時に実 `yt-dlp` の出力で確認し、同書に記録すること（承認前に確定させる）。
- `ArticleWriter`: プロバイダに依存しない。プロンプトテンプレートにタイムスタンプを除いた本文とメタ情報を埋め込み、`LLMClient` を呼び出して、結果を `Article`（タイトル・Markdown 本文・出典 URL・生成モデル名）に変換する。
  - プロンプトの組み立てと出力の後処理はここに集約し、全プロバイダで共有する。
- `LLMClient`: プロバイダごとの薄いアダプタ。責務は「system プロンプトと user プロンプトを受け取り、生成テキストとモデル名を返す」ことだけ。

  ```go
  type LLMClient interface {
      Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
  }
  // GenerateRequest: SystemPrompt, UserPrompt, MaxOutputTokens など、プロバイダ共通の最小限の項目のみ
  // GenerateResponse: Text, Model（生成テキストと、生成に使われたモデル名）
  ```

  - 初期実装は `internal/llm/deepseek`。Gemini・Claude は必要になったら `internal/llm/gemini`・`internal/llm/claude` として追加する。
  - プロバイダ SDK の import は各実装パッケージの中に閉じ込める。他のパッケージから SDK の型を参照しないこと。
  - プロバイダの選択は `internal/config` の値を見て、`main.go`（または小さなファクトリ関数）で行う。
- `Publisher`: 初期実装は `SlackWebhookPublisher` と `FilePublisher`（ローカル保存。デバッグ用）。

## 前提・制約

- **文字起こしがない動画がある**（投稿者が字幕を無効化している、公開直後で自動字幕が未生成など）。yt-dlp が正常終了しても字幕ファイルが出力されないケースを検出し、分かりやすいエラーにする。Whisper 等による音声文字起こしへのフォールバックは**スコープ外**（将来拡張）。
- 日本語の自動字幕には句読点が付いており、品質は実用レベル。
- 動画の長さは 40 分前後を想定（1 万数千字）。**チャンク分割はせず一括で処理する**。数時間級の動画への対応は将来拡張。
- クラウドの IP からは YouTube にブロックされやすい。ローカル実行を前提とする。
- Slack の従来の mrkdwn は標準 Markdown ではない。`markdown` ブロックを使い、文字数上限を超える場合は分割して投稿する。上限値は実装時に Slack の公式ドキュメントで確認すること。
- 記事の末尾には必ず元動画へのリンク（出典）を付ける。
- 字幕と info.json は動画 ID ごとにキャッシュする。プロンプトを調整するたびに再取得しないようにするため。
- LLM はエラーを返さずに空の応答や途中で打ち切られた応答を返すことがある。`finish_reason` を確認し（`stop` 以外、特に `length` は打ち切り）、空の応答とともにエラーとして扱うこと。
- DeepSeek の現行モデルは thinking（推論）モードがデフォルトで有効で、レイテンシとコストに影響する。推論過程は `reasoning_content` として本文（`content`）とは別に返るので、記事には `content` だけを使う。thinking モードでは `temperature` が無視される。初期は API のデフォルト（thinking 有効）のままにし、必要に応じて設定可能にする（リクエストの `thinking` パラメータで切り替えられる）。
- DeepSeek は平日のピーク時間帯（UTC 01:00–04:00 と 06:00–10:00、日本時間では 10–13 時と 15–19 時）の料金が2倍になる。
- DeepSeek の API への入力は中国で処理・保存され、プライバシーポリシー上はデフォルトでモデルの学習に使われる（オプトアウトあり）。送るのは公開動画の字幕・メタ情報とプロンプトであり、機密情報を送らないこと（[security.md](security.md) を参照）。
- モデル名は頻繁に更新される。コードにハードコードせず設定で与える。再現性のため、`-latest` 系のエイリアスより具体的なモデル名を推奨。ただし DeepSeek の `deepseek-flash`（2026-09 時点で DeepSeek-V4.1-Flash を指す）は提供元が指し示すモデルを切り替えるエイリアスで、バージョンを固定した ID は提供されていない。どのモデルで生成したかを追えるよう、応答に含まれる `model` の値を記録すること。

## 未確定事項

- コラム生成プロンプトの詳細（後で決める）。`prompts/` 以下のテンプレートファイルに分離し、`embed` で埋め込む。外部ファイルで上書きできるようにしておく。
- Slack 以外の投稿先（Discord 等）は必要になったら追加する。
- プロバイダごとにプロンプトの微調整が必要になった場合の扱い（共通テンプレート＋プロバイダ別の上書きなど）。

## 想定ディレクトリ構成

```
cmd/yt2column/main.go     # CLI エントリポイント
internal/pipeline/        # 3段階を束ねるオーケストレーション
internal/transcript/      # TranscriptSource と yt-dlp 実装、json3 パーサ
internal/writer/          # ArticleWriter（プロバイダ非依存）
internal/llm/             # LLMClient interface と共通型
internal/llm/deepseek/    # DeepSeek 実装（標準ライブラリで OpenAI 互換 API を呼ぶ）
internal/llm/gemini/      # Gemini 実装（将来追加。google.golang.org/genai）
internal/llm/claude/      # Claude 実装（将来追加）
internal/secret/          # 秘密情報（API キー・Webhook URL）を保持する型
internal/publisher/       # Slack / File
internal/config/          # 環境変数からの設定読み込み
prompts/                  # プロンプトテンプレート
testdata/                 # json3・info.json のサンプル
```

## 設定（環境変数）

| 変数 | 説明 |
|---|---|
| `YT2COLUMN_LLM_PROVIDER` | `deepseek`（デフォルト）。将来 `gemini` \| `claude` を追加 |
| `YT2COLUMN_MODEL` | LLM のモデル名。プロバイダに合ったものを指定（例: `deepseek-flash`） |
| `DEEPSEEK_API_KEY` | DeepSeek 用 |
| `GEMINI_API_KEY` | Gemini 実装を追加したとき用 |
| `ANTHROPIC_API_KEY` | Claude 実装を追加したとき用 |
| `SLACK_WEBHOOK_URL` | 投稿先の Slack Incoming Webhook URL |
| `YT2COLUMN_CACHE_DIR` | 字幕・info.json のキャッシュディレクトリ |
| `YT2COLUMN_YTDLP_PATH` | 省略時は PATH 上の `yt-dlp` を使う |

## 開発ルール

- 各 interface にはテスト用の fake 実装を用意する。ユニットテストでは外部コマンド・API・Webhook を呼ばない。`ArticleWriter` のテストは fake の `LLMClient` で行う。
- json3 のパースは `testdata/` の実データでテストする。
- `go vet` とテストが通る状態でコミットする（`make test && make lint`。`make lint` は `go vet` 相当の `govet` を含む）。
