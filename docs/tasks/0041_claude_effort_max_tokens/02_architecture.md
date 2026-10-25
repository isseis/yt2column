# アーキテクチャ設計書：Claude の effort の既定を high にし、出力の上限 16,000 を緩和する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-25 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（要件定義書。以下、要件書）の設計である。既存のファイルについての記述は、コミット `d92286a` のファイルで確かめた。`file:line` はこのコミットの行番号を指す。F-NNN・AC-NN は要件書の項番を指し、先行タスクの項番は「0009 F-006」のようにタスクの番号を付けて書く。要件のレビューで設計に申し送られた事項（[design_handoff.md](design_handoff.md) の H-01〜H-08）への対応は、付録 A にまとめる。

本書は、要件書 6 章の用語（effort、出力の上限、V、段階、本番の構成、打ち切り）を同じ意味で使う。「出力の上限」は、リクエストの `max_tokens` の値である。ほかに次の用語を使う。

-   **モデルの出力の上限:** 公開文書がモデルごとに示す、同期の Messages API で 1 回に出力できるトークン数の最大（公開文書の「Max output」）。リクエストの `max_tokens` は、この値を超えられない。
-   **既定の effort:** `YT2COLUMN_CLAUDE_EFFORT` が未設定のときに設定の読み込みが使う effort。本書では `high` である。
-   **既定の上限:** Claude アダプタが、`llm.GenerateRequest` の `MaxOutputTokens` が 0 のときに送る `max_tokens`。今は 16,000 で、本書の変更の後は V（40,000）である。
-   **最も遅い出力の速さ:** 本番の構成の測定（0018 評価記録 §4.7.2・§4.7.4）から計算した出力の速さ（1 回の CLI の実行の出力のトークン数 ÷ その実行の所要時間）のうち、最も小さい値。本書では 94.9 トークン/秒である（1.4）。
-   **上限のガイド:** 本タスクで新設する英語の開発者ガイド `docs/dev/developer_guide/claude_output_limit.md`。既定の上限の根拠を測り直す手順を書く（3.5）。

## 1. 設計の全体像 (Design Overview)

### 1.1. 設計原則

-   **環境変数の解釈と既定値は `internal/config` だけが持つ。** 既定の effort は、`internal/config` の読み込みが、`YT2COLUMN_CLAUDE_EFFORT` が未設定のときに入れる。アダプタ（`internal/llm/claude`）とプロバイダの構築（`internal/llm/provider`）は、今までどおり検証済みの `config.LLMConfig` から値を受け取るだけである。既定のプロバイダ `deepseek` も `internal/config` が入れている（`loadProvider`・`loadProviderOptional`。`internal/config/config.go:382-411`）。
-   **既定の effort は、今は未設定がエラーになる条件の中だけで当てる。** 既定を入れるのは `loadClaudeEffort` で、この関数は、実行する段階の中に、プロバイダが `claude` の段階があるときだけ呼ばれる（`loadProviderVars`。`internal/config/config.go:525-542`）。CLI の `LoadFor` も、サーバの `LoadServer`（`config.go:259-260`）も、同じ `loadLLM` から `loadProviderVars` を通る。呼ばれる条件は変えないので、`--after` で実行しない段階や DeepSeek だけの実行では、今までどおりこの変数を読まない（AC-05）。
-   **ゼロ値の `EffortUnset` をアダプタに届けない仕組みは変えない。** 既定を入れた後も、`claudeparam.EffortUnset` はゼロ値のまま、アダプタの構築が拒否する（`internal/llm/claudeparam/claudeparam.go` の `Effort` の doc コメント）。本書は `EffortUnset` の意味を変えず、未設定のときに `EffortHigh` を代入するだけである（CLAUDE.md「Declare, don't infer」）。
-   **既定の上限は、アダプタの 1 つの定数だけで決まる。** 3 つの段階は、どれも `MaxOutputTokens` を 0 にして呼ぶ（`internal/writer/writer.go:463`・`internal/writer/shorten.go:28`・`internal/writer/review.go:256`）。そのため、Claude アダプタの定数 `defaultMaxOutputTokens`（`internal/llm/claude/request.go:23-26`）を 1 か所変えれば、3 つの段階の上限が同じ値で変わる。段階ごとの値や、利用者が変える手段は作らない（要件書 2.3）。DeepSeek アダプタには触れない。
-   **非ストリーミングと 15 分のタイムアウトは変えない。** V は、V トークンの出力にかかる見積もりの時間が `provider.LLMTimeout`（15 分）の半分に収まるように選ぶ（1.4）。新しい通信の仕組みは足さない（1.2 の 3 つ目）。
-   **数値は 1 か所に置き、文書には写さない。参照も 1 か所に集める。** V の値は定数 `defaultMaxOutputTokens` だけにある。README と上限のガイドは数値を書かない。数値を写すと、値の一致を確かめるテストが要り（CLAUDE.md「Document tests check values, structure, and executable examples, never prose」）、写した値がコードと食い違いうるためである。コードのコメントも、タスクの文書ではなく上限のガイドだけを指す。ガイドが、今の根拠の記録（本書 1.4）の場所を示す。V が満たすべき (a)・(b) の条件（定義は 1.4）は、文書ではなくテストで固定する（7.1）。

### 1.2. 設計の際に決めたこと（2026-10-25）

人間のレビュアーが決めたもの:

-   **V は 40,000 にする。** 要件書 F-002 の (a)・(b) から決まる範囲は 31,768 以上、42,702 以下である（1.4）。範囲の下端に近い 32,000 ではなく 40,000 を選んだ。理由は 2 つある。1 つ目は、40 分前後の動画を v5 で測っていないことである（要件書 5. 制約条件）。2 つ目は、16,000 で打ち切られた実行（0018 評価記録 §4.7.4 の roNZ-rLxhUE-1-f2）の出力の真の値が、16,000 以上としか分からないことである。どちらの分からない点にも、観測した出力の最大の 2 倍より大きい余裕を取って備える。範囲の上端に近い値は、(b) の見積もりの時間が 7.5 分にほぼ等しく、混雑時の速さの低下に余裕がないので選ばなかった。
-   **既定の effort は `internal/config` に置く。** `internal/config` に、既定の effort を表す名前付きの値を置き、その値は `claudeparam.EffortHigh` とする。`claudeparam` には足さない。既定は「環境変数が未設定のときにどう解釈するか」の規則であり、環境変数の解釈は `internal/config` の責務だからである（0009 F-006）。`claudeparam` の doc コメントは「effort の値の一覧はここだけにある」と定めるが、既定はその一覧の要素ではない。
-   **非ストリーミングの長い無通信のリスクは受け入れ、仕組みを足さない。** HTTP/2 の PING（`ReadIdleTimeout`）は足さない。無通信で接続が切れたという観測がなく、CLAUDE.md「Performance」の「Do not close a review finding by adding a mechanism until the finding's premise is verified」に従う。リスクの範囲は 3.6 と 4.3 に書く。
-   **README と上限のガイドに V の数値を書かない。** 1.1 の最後の原則のとおりである。README の `max` の注意は「既定の出力の上限でも」と書き、数値を挙げない（3.4）。

### 1.3. 概念モデル

矢印 A → B は「A の値が B の値を決める」を表す。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    ENV[("YT2COLUMN_CLAUDE_EFFORT")]
    DEF[("defaultClaudeEffort")]
    LOAD["config.loadClaudeEffort"]
    CFG["config.LLMConfig"]
    REQ["llm.GenerateRequest"]
    CONST[("claude.defaultMaxOutputTokens")]
    BODY["claude.messagesRequest"]

    ENV -->|"設定されているとき"| LOAD
    DEF -->|"未設定のとき"| LOAD
    LOAD -->|"ClaudeEffort"| CFG
    CFG -->|"output_config.effort"| BODY
    REQ -->|"正の MaxOutputTokens"| BODY
    CONST -->|"MaxOutputTokens が 0 のとき"| BODY

    class ENV data
    class DEF,CONST,LOAD enhanced
    class CFG,REQ,BODY process
```

Legend:

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    L1[("変えないデータ（環境変数）")]
    L2["変えない型・関数"]
    L3[("新設する、または値を変える定数")]
    L4["振る舞いを変える関数"]
    class L1 data
    class L2 process
    class L3,L4 enhanced
```

### 1.4. V の決定と根拠（F-002・AC-13・H-01）

**使った測定。** 実 API での追加の測定はしていない（要件書「要件定義の際に決めたこと」）。

| 出典 | 構成 | 本書で使う値 |
|---|---|---|
| 0009 `02_architecture.md` §1.4 | Opus 5.5・Sonnet 5.5。当時の仮のテンプレート。各 1 回、`max_tokens` 64,000 の非ストリーミング。`curl` で送った | `high`・`xhigh` の出力の最大 8,099 トークン。`max` はどちらも 64,000 をすべて推論に使った。速さは Opus 約 114 トークン/秒（64,000 を 559 秒）、Sonnet 約 145 トークン/秒（64,000 を 442 秒）。64,000 の打ち切りの応答の本文は 245,601 バイト |
| 0008 評価記録 §5.7 | Sonnet 5.5・`high`・v5 | 5 本中 2 本が 16,000 で打ち切られ、うち 1 本（roNZ-rLxhUE）は推論だけで 16,000 に達した |
| 0018 評価記録 §4.7.2（B3） | Opus 5.5・`high`・v5（本番の構成） | 1 段目（生成だけ）の 10 件の出力は 5,030〜13,688 トークン。2 段目（字数の調整と検証・推敲）では、字数の調整が 7,837・7,371、検証・推敲が最大 4,777 |
| 0018 評価記録 §4.7.4（B4g） | Opus 5.5・`high`・v7（テンプレートだけを変えた構成） | 成功した 10 件の出力は 5,273〜15,884 トークン。roNZ-rLxhUE-1-f2 は 16,000 で打ち切られた（出力の真の値は 16,000 以上としか分からない） |

**出力の速さ。** 0018 の実行のうち、1 回の CLI の実行で LLM を 1 回だけ呼んだもの 29 件について、出力のトークン数を所要時間で割った。B3 の 1 段目の 10 件（生成）、B4g の 10 件（生成）、B3 の 2 段目のうち字数の調整が走らなかった 9 件（検証・推敲）である。字数の調整が走った実行は、1 回の実行で 2 回呼ぶので、呼び出しごとの速さを分けられず、除いた。

-   値は 94.9〜124.4 トークン/秒である。最も遅いのは B3 の 2 段目の EqnGQDU71-Y-1 の検証・推敲（1,364 トークンを 14.374 秒）の 94.9 トークン/秒である。生成だけで見ると、最も遅いのは B4g の 18tbwwPM91A-1（11,586 トークンを 118.05 秒）の 98.1 トークン/秒である。
-   0018 の所要時間は CLI の実行全体の時間で、LLM の呼び出し以外の処理（設定や素材の読み込み、プロンプトの組み立て）を含む。これは速さを実際より遅く見積もる向きに働く。出力が短い実行ほどその割合が大きく、最も遅い値が短い出力（検証・推敲）から出たのはそのためとみる。(b) の判定では安全側なので、そのまま使う。
-   0018 の実行の多くは 10 件を並列に起動した（0018 評価記録 §3.3）。ただし B4g の roNZ-rLxhUE-1（15,884）と 18tbwwPM91A-2 は、やり直しとして別に起動した。並列の起動がサーバ側の速さを下げるかは分からない。
-   0018 の出力はどれも 16,000 トークン以下で、40,000 トークンの出力の速さは測っていない。長い出力でも速さが大きく落ちないことの根拠は、0009 §1.4 の Opus の 64,000 トークンの出力（約 114 トークン/秒）である。ただし 1 回だけの測定で、テンプレートも違う。混雑時の速さは測っていない。

**(a)・(b) から決まる範囲。**

-   (a) V ≥ 15,884 × 2 = 31,768。
-   (b) V ÷ 最も遅い出力の速さ ≤ 450 秒（`provider.LLMTimeout` 15 分の半分）。最も遅い出力の速さは 1,364 ÷ 14.374 トークン/秒なので、V ≤ 1,364 ÷ 14.374 × 450 ≒ 42,702.1、すなわち V ≤ 42,702 である。
-   したがって、V は 31,768 以上、42,702 以下である。

**選んだ値と根拠。**

| 項目 | V = 40,000 のときの値 |
|---|---|
| (a) 観測した出力の最大 15,884 に対する倍率 | 約 2.52 倍。本番の構成（v5）の最大 13,688 に対しては約 2.92 倍 |
| (b) 最も遅い出力の速さでの見積もりの時間 | 40,000 ÷ 94.9 ≒ 422 秒（約 7.0 分）。7.5 分以内 |
| (b) 生成だけの最も遅い速さ（98.1）、0009 の Opus の速さ（114.5）での見積もりの時間 | 約 408 秒（約 6.8 分）、約 349 秒（約 5.8 分） |
| (c) モデルが受け付けるか | 受け付ける。公開文書「Models overview」（https://platform.claude.com/docs/en/about-claude/models/overview 、2026-10-25 に確認）は、claude-opus-5-5・claude-sonnet-5-5 のモデルの出力の上限を 128K トークンとしている。0009 §1.4 では、どちらも非ストリーミングで 64,000 を受け付けた |
| 16,000 で打ち切られた実行の扱い | roNZ-rLxhUE-1-f2 の出力は 16,000 以上としか分からない。V はその 2.5 倍である。同じ動画・同じ構成の成功した試行は 15,884 と 11,569 だった。V を超える出力の見込みは小さいとみるが、保証はしない |
| 40 分前後の動画の扱い | v5 では測っていない。0018 でいちばん長い動画は roNZ-rLxhUE（32:29）で、v5 の出力は 10,986・11,122 だった。出力のトークン数は動画の長さの順に並ばない（要件書「要件定義の際に決めたこと」）。2.5 倍の余裕で備えるが、保証はしない（要件書 5. 制約条件） |
| Sonnet 5.5 の扱い | 0008 §5.7 では、`high`・v5 で推論だけで 16,000 に達した実行がある。V で足りるかは測っていない。README で、既定の effort は claude-opus-5-5 に合わせた値であり、ほかのモデルでは打ち切られうると案内する（AC-19、3.4） |

**ほかのモデルについて（H-02）。** 公開文書（各モデルの overview のページ、2026-10-25 に確認）で、次のモデルのモデルの出力の上限を確かめた。

| モデル | モデルの出力の上限 |
|---|---|
| 現行: claude-fable-5-1、claude-opus-5-5、claude-sonnet-5-5、claude-haiku-5-5 | 128K |
| 旧世代（Models overview の「Legacy models」に挙がるもの）: claude-fable-5、claude-opus-5、claude-opus-4-8、claude-opus-4-7、claude-opus-4-6、claude-sonnet-5、claude-sonnet-4-6 | 128K |
| 旧世代: claude-opus-4-5、claude-haiku-4-5 | 64K |

どのモデルも、モデルの出力の上限が V（40,000）以上である。そのため、要件書 4.4 の「V を超えて `400` で失敗しうるモデル」は、公開文書が挙げるモデルにはない。README には、この点についての案内を足さない。公開文書が挙げないモデル名を設定した場合に `400` が返れば、今までどおり HTTP ステータスのエラーとして報告され、`errors.go` の案内文が `max_tokens` を確かめるよう促す（`internal/llm/claude/errors.go:60`）。

## 2. システム構成 (System Structure)

### 2.1. 全体の構成

矢印 A → B は、A がコンポーネントなら「A が B を呼び、B に値を渡す」を、A がデータなら「B が A を読む」を表す。緑のノードが本タスクで振る舞いが変わる要素である。

```mermaid
flowchart TD
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    ENV[("環境変数")]
    CMD["cmd/yt2column<br>cmd/yt2column-server"]
    CFG["internal/config"]
    PRV["internal/llm/provider"]
    WR["internal/writer"]
    CL["internal/llm/claude"]
    DS["internal/llm/deepseek"]
    API[("Anthropic<br>Messages API")]

    ENV --> CFG
    CMD -->|"LoadFor / LoadServer"| CFG
    CMD -->|"config.LLMConfig"| PRV
    PRV -->|"claude.Options"| CL
    PRV -->|"deepseek.Options"| DS
    CMD -->|"llm.LLMClient"| WR
    WR -->|"llm.GenerateRequest"| CL
    WR -->|"llm.GenerateRequest"| DS
    CL -->|"messagesRequest"| API

    class ENV,API data
    class CMD,PRV,WR,DS process
    class CFG,CL enhanced
```

Legend:

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    L1[("データ・外部のサービス")]
    L2["変えないコンポーネント"]
    L3["変えるコンポーネント"]
    class L1 data
    class L2 process
    class L3 enhanced
```

図の矢印は、すべて今のコードにある関係である（`internal/llm/provider/provider.go:27-39`・`:55-82`、`internal/writer/writer.go:463`・`shorten.go:28`・`review.go:256`、`internal/config/config.go:259-260`・`:346`）。本タスクは新しい関係を足さない。`internal/config` で変わるのは `loadClaudeEffort`、`internal/llm/claude` で変わるのは `request.go` の `defaultMaxOutputTokens` である。

### 2.2. コンポーネント配置

| 変更 | 置き場所 | 理由 |
|---|---|---|
| 既定の effort | `internal/config`（`config.go` の `loadClaudeEffort` と、既定を表す名前付きの値） | 環境変数の解釈は `internal/config` の責務（1.2） |
| 既定の上限 V | `internal/llm/claude/request.go` の `defaultMaxOutputTokens` | Messages API は `max_tokens` を必須とし、0 を受け付けない。そのため「0 のときに送る値」は Claude アダプタの事情で、アダプタの外に出さない（CLAUDE.md「Provider details stay in their adapter package」） |
| タイムアウトの根拠のコメント | `internal/llm/provider/provider.go:16-19` | 値は変えず、根拠の記述だけを直す（3.3） |
| V の (a)・(b) を固定するテスト | `internal/llm/provider/provider_test.go` | (b) は `LLMTimeout`（provider）と V（claude）の関係なので、両方を見られる provider のテストに置く（7.1） |
| 上限のガイド | `docs/dev/developer_guide/claude_output_limit.md`（新設、英語） | 要件書「要件定義の際に決めたこと」と H-08 |

### 2.3. データの流れ

プロバイダが `claude` で、`YT2COLUMN_CLAUDE_EFFORT` が未設定の実行の流れを示す。

```mermaid
sequenceDiagram
    participant M as cmd（CLI・サーバ）
    participant C as internal/config
    participant P as internal/llm/provider
    participant W as internal/writer
    participant G as Claude アダプタ（生成用）
    participant R as Claude アダプタ（検証・推敲用）
    participant API as Messages API

    M->>C: LoadFor / LoadServer
    C->>C: loadClaudeEffort（未設定なら high）
    C-->>M: 設定（ClaudeEffort は EffortHigh）
    M->>P: New / NewReview
    P-->>M: G と R（どちらも effort high、タイムアウト 15 分）
    M->>W: Write
    W->>G: 生成（MaxOutputTokens 0）
    G->>API: max_tokens 40000、effort high
    API-->>G: 応答
    opt 本文が字数の上限を超えたとき
        W->>G: 字数の調整（MaxOutputTokens 0）
        G->>API: max_tokens 40000、effort high
        API-->>G: 応答
    end
    W->>R: 検証・推敲（MaxOutputTokens 0）
    R->>API: max_tokens 40000、effort high
    API-->>R: 応答
```

応答が `max_tokens` で終われば、アダプタは `llm.ErrTruncated` を返し、その段階で実行が失敗する（4.1）。`--after` による再開や、DeepSeek だけを使う実行では、`loadClaudeEffort` が呼ばれない（AC-05）。

## 3. コンポーネント設計 (Component Design)

### 3.1. `internal/config`：既定の effort（F-001・H-06）

**型と値。** 新しい公開の型は足さない。`internal/config` に、既定の effort を表す非公開の名前付きの値を足す。

```go
// defaultClaudeEffort is the effort used when YT2COLUMN_CLAUDE_EFFORT is
// unset and a stage the run needs uses the claude provider. The adapter's
// default output limit was sized for this effort; changing it calls for the
// re-measurement in docs/dev/developer_guide/claude_output_limit.md.
const defaultClaudeEffort = claudeparam.EffortHigh
```

**`loadClaudeEffort` の規則。** 呼ばれる条件（`loadProviderVars` の `claude` の判定、`internal/config/config.go:525-542`）は変えない。呼ばれたときの規則を次のようにする。

| `YT2COLUMN_CLAUDE_EFFORT` | 今 | 本書の変更後 |
|---|---|---|
| 未設定 | `ErrMissing`（変数名を持つ `*VarError`） | `defaultClaudeEffort`（`high`）を入れ、エラーにしない |
| 空の値 | `ErrMissing` | 変えない（`ErrMissing`） |
| 5 つの値のいずれか | その値 | 変えない |
| それ以外 | `ErrInvalid`（固定の理由 `reasonClaudeEffort`） | 変えない |

空の値を未設定と区別するのは、`docs/dev/project_overview.md`「設定（環境変数）」と README の「an empty value is treated as set and rejected, not as unset」（`README.md:735-736`）の方針に従うためである。`loadAnthropicWorkspaceID`（`config.go:603-620`）も、未設定と空の値を同じように区別している。

**doc コメントの更新。** 次の 2 か所は「claude なら必須」と書いているので、「未設定なら `high`」に直す。

-   `LLMConfig.ClaudeEffort` の doc コメント（`internal/config/config.go:158-159`）。「`EffortUnset` でない」という保証は変わらない。
-   `loadClaudeEffort` の doc コメント（`internal/config/config.go:586-589`）。

**ほかの変数への影響（AC-06）。** `loadProviderVars` は、`ANTHROPIC_API_KEY`・`YT2COLUMN_CLAUDE_EFFORT`・`ANTHROPIC_WORKSPACE_ID` のエラーを別々に集める（`config.go:534-540`）。effort の既定はこのうち 1 つの関数の中だけで当てるので、`ANTHROPIC_API_KEY` の未設定は今までどおり `ErrMissing` になる。

### 3.2. `internal/llm/claude`：既定の上限（F-002）

`defaultMaxOutputTokens`（`internal/llm/claude/request.go:23-26`）の値を 16,000 から 40,000 に変える。doc コメントには、値は要件の条件（観測した出力の最大の 2 倍以上、最も遅い出力の速さでタイムアウトの半分以内）から決めたこと、根拠と測り直しの手順は上限のガイドにあることを書く。タスクの文書は指さない（1.1）。`effectiveMaxTokens`（`request.go:66-75`）の規則（正の `MaxOutputTokens` はその値、0 なら既定の上限）は変えない。打ち切りのエラーのメッセージは、`requestSummary` 経由で `effectiveMaxTokens` が返す値を使っている（`response.go:297-307`）。そのため、V を送ったリクエストのメッセージは自動的に 40,000 を示す（AC-10）。

### 3.3. `internal/llm/provider`：タイムアウトの根拠（H-03・AC-12）

`LLMTimeout` の値（15 分。`internal/llm/provider/provider.go:20`）は変えない。doc コメント（`provider.go:16-19`）の「Claude アダプタの既定の上限の非ストリーミングの呼び出しは、十分に短い時間で終わると測定した」を、次の内容に直す。Claude アダプタの既定の上限は、最も遅い出力の速さで出力してもこのタイムアウトの半分以内に収まるように選んであり、その関係は provider のテストが固定している（7.1）。根拠と測り直しの手順は上限のガイドにある。CLI のタイムアウトのメッセージ（`cmd/yt2column/run.go:965`）と、サーバの停止の待ち時間（`cmd/yt2column-server/run.go:40-43`・`:232-235`）は `LLMTimeout` から計算しているので、変わらない。

### 3.4. 利用者向けの文書（F-003・AC-14〜AC-16・AC-19）

| 文書 | 変更 |
|---|---|
| README「Configuration」の表の `YT2COLUMN_CLAUDE_EFFORT` の行（`README.md:724`） | 「When unset」を「`high` when the generation or review provider is `claude`」に直す。空の値がエラーになることは、表の後の既存の文（`README.md:735-736`）が述べている（AC-14） |
| README の表の後の `max` の注意（`README.md:750-752`） | 残す。「既定の出力の上限でも、`max` は推論だけで上限を使い切り、記事の本文を返さないと見込まれる」と書き、数値は挙げない。根拠は、0009 §1.4 で 64,000（V より大きい）でも推論だけで上限に達したことである（AC-15） |
| README の `max` の注意の後 | 段落を足す。既定の `high` は claude-opus-5-5 に合わせて選んだ値であること、ほかのモデル（例: claude-sonnet-5-5）では `high` でも出力の上限に達して打ち切られうること、その場合は `YT2COLUMN_CLAUDE_EFFORT` で effort を下げられることを書く（AC-19） |
| README の `--after` の段落（`README.md:116-121`） | 「`claude` its effort) are still required」を削り、API キーだけが必須である書き方にする（AC-14） |
| `docs/dev/project_overview.md`「設定（環境変数）」の表の `YT2COLUMN_CLAUDE_EFFORT` の行（`:115`） | 「未設定のとき」を「生成または検証・推敲のプロバイダが `claude` なら `high`」に直す（AC-16） |
| `docs/dev/developer_guide/package_reference.md` の `internal/config` の行（`:27`） | 「the Claude effort via `claudeparam.ParseEffort`」に、未設定なら `high` であることを加える |

README の打ち切りの料金（H-05）は書かない。料金は単価の表の改定で変わり、文書に写すと古くなるためである。本書 4.3 に記録するだけにする。

### 3.5. 上限のガイド（F-003・AC-17・AC-18・H-08）

`docs/dev/developer_guide/claude_output_limit.md` を英語で新設する。`pricing_update.md` と同じく、繰り返し行う作業の手順を書く。節の構成は次のとおりである。

| 節 | 内容 |
|---|---|
| When to re-measure | 本番の構成の 3 つの段階のいずれかで、テンプレート、effort（`defaultClaudeEffort` の変更を含む）、モデルを変えたとき |
| Cost and approval | 測り直しは実 API を呼び、料金がかかる。人の承認を得てから行う（CLAUDE.md「Tool Execution Safety」）。評価の手順は 0008 評価記録 §2.3 と 0018 評価記録 §3 を参照する |
| How to measure | 呼び出しごとの速さを得るため、1 回の CLI の実行で LLM を 1 回だけ呼ぶよう、段階ごとに実行する。字数の調整と検証・推敲は、`--artifacts` と `--after` で前の段階の出力を再利用する。字数の調整は本文が字数の上限を超えたときだけ走る。例: 生成は `--no-review --no-shorten`、字数の調整は `--after generation --no-review`、検証・推敲は `--after shorten` |
| What to collect | 段階ごとの出力のトークン数（README「Token usage」の `tokens:` の行の `output=`、`--artifacts` の中間生成物に記録したトークン数）、その実行の所要時間（実時間）、打ち切られた実行の有無（CLI の `generation truncated by the output token limit` などの打ち切りのエラー） |
| How to judge | 要件書 F-002 の (a)・(b)・(c) と照らす。(a) 観測した出力の最大の 2 倍以上。(b) 出力のトークン数 ÷ 所要時間で速さを計算し（呼び出し以外の処理を含むので遅めに出る）、その最も遅い値で V トークンを出力する時間が 7.5 分以内。(c) 対象のモデルについて、V がモデルの出力の上限以下 |
| What to update | 条件を満たさない場合: 定数 `defaultMaxOutputTokens`（`internal/llm/claude/request.go`）、(a)・(b) を固定するテストの名前付きの値（観測した出力の最大と最も遅い出力の速さ。`internal/llm/provider/provider_test.go`）、根拠の記録（その変更を扱うタスクの `02_architecture.md`）、このガイドが示す根拠の記録の場所、README の `max` の注意と既定の effort の案内 |
| Current record | 今の根拠の記録の場所（本書 1.4）を示す。値は写さない |

ガイドは V の値も出力の速さも写さない（1.2）。`CLAUDE.md` の Quick Links の「Development Guides」に、ガイドへのリンクと 1 行の説明を足す（AC-18）。

### 3.6. 先行タスクの方針の変更（0009）

本書は、0009 の 2 つの方針を変える。どちらも要件書 5. 制約条件で決まっており、ここでは設計に関わる点を書く。

-   **0009 F-006（`YT2COLUMN_CLAUDE_EFFORT` は claude なら必須で、既定値を持たない）。**
    -   元の方針: `docs/tasks/0009_claude_llm_client/01_requirements.md` F-006 と、その設計（0009 `02_architecture.md` §3.2）。どの effort で生成したかを、設定から常に読み取れるようにするためだった。
    -   例外とする理由: issue #171 の決定。何も指定しない利用者が、本番の構成の effort で記事を作れるようにする。未設定のときの値は README と `project_overview.md` に書くので、文書から読み取れる。
    -   古い振る舞いを確かめているテスト: `internal/config/config_test.go:281` の `TestLoadClaudeMissing` の行 `"effort unset"`。未設定を `ErrMissing` とすることを確かめるテストは、この行だけである（ほかのテストは、どれも `YT2COLUMN_CLAUDE_EFFORT` を設定している）。この行を外し、既定を確かめるテストに置き換える（7.1）。同じテストの `"effort empty"` の行（`:282`）は残す。
-   **0009 §3.6（既定の上限は、Anthropic の非ストリーミングの目安の約 16,000 を超えない）。**
    -   元の方針: 0009 `02_architecture.md` §3.6 の表の「`MaxOutputTokens` が 0 のときの `max_tokens`」の行。
    -   今の公開文書: API errors「Long requests」（https://platform.claude.com/docs/en/api/errors 、2026-10-25 に確認）は、16,000 という数値を挙げていない。大きな `max_tokens` にはストリーミングかバッチを勧め、10 分を超える呼び出しに特に注意を促す。直接 API を呼ぶ場合は TCP の keep-alive を勧め、公式 SDK は非ストリーミングの呼び出しが 10 分を超えると見込まれるかを確かめる、と書いている。
    -   公式 SDK の確認の中身: anthropic-sdk-python の `_calculate_nonstreaming_timeout`（`src/anthropic/_base_client.py`、`main` を 2026-10-25 に確認）は、見込みの時間を 3,600 秒 × `max_tokens` ÷ 128,000 とし、呼び出し元がタイムアウトを指定していないとき、600 秒を超えると非ストリーミングの呼び出しを拒否する。しきい値は約 21,333 で、V はその約 1.9 倍である。つまり V は、公式 SDK をタイムアウトの指定なしで使うと非ストリーミングでは送れない値である。この確認は SDK の中の仕組みで、標準ライブラリで HTTP を送る本アダプタには当てはまらない。API そのものは、0009 §1.4 で 64,000 の非ストリーミングを受け付けた。V の見積もりの時間（約 7.0 分）は 10 分に収まる。
    -   無通信のリスクの範囲: HTTP クライアントは Transport を `nil` のままにしており（`internal/llm/llmhttp/llmhttp.go:29-38`）、Go 1.26.5 の `http.DefaultTransport` が使われる。その dialer は TCP の keep-alive（30 秒）を有効にする（Go 1.26.5 の `net/http/transport.go` の `DefaultTransport`）。ただし keep-alive は、利用者の端末から次の機器までの TCP 接続にしか効かない。プロキシの環境変数を設定した環境では、プロキシから Anthropic までの区間はプロキシの無通信のタイムアウトに従い、それが V の見積もりの時間より短ければ切れうる。また、0009 §1.4 の「約 9 分の無通信でも切れなかった」は、1 つのネットワークから `curl` で 1 回ずつ送った観測で、Go のクライアントでは確かめていない。なお 0018 評価記録 §4.7.4 では、並列に起動した 2 件が同じ時刻に HTTP `400` で失敗したが、原因は確かめられておらず、無通信による切断の観測ではない。切れた場合の振る舞いは 4.3 に書く。このリスクは受け入れる（1.2）。
    -   例外とする理由: v5 以降のテンプレートと effort `high` では 16,000 で打ち切られる（要件書 1. の背景）。
    -   古い振る舞いを確かめているテスト: 値そのものを固定するテストはない。`internal/llm/claude/claude_test.go:342-372` の `TestGenerateMaxTokens` と `response_test.go:160-181` の `TestGenerateValidationOrder` は定数 `defaultMaxOutputTokens` を参照しているので、値を変えても通る。

### 3.7. コンポーネントの責務

| ファイル | 新規 / 変更 | 責務と変更 | 更新が要る既存のテスト |
|---|---|---|---|
| `internal/config/config.go` | 変更 | `defaultClaudeEffort` を足し、`loadClaudeEffort` が未設定のときに使う。doc コメント 2 か所を直す | `config_test.go:281`（`"effort unset"` の行を外す） |
| `internal/config/config_test.go` | 変更 | 既定のテストを足す（7.1） | — |
| `internal/llm/claude/request.go` | 変更 | `defaultMaxOutputTokens` を 40,000 にし、doc コメントで上限のガイドを指す | なし（3.6） |
| `internal/llm/provider/provider.go` | 変更 | `LLMTimeout` の doc コメントの根拠を直す。値は変えない | なし |
| `internal/llm/provider/provider_test.go` | 変更 | effort を設定しない環境のテストと、V の (a)・(b) を固定するテストを足す（7.1）。`claudeRequestFor` を、検証・推敲のプロバイダとモデルでも作れるようにする | `claudeRequestFor` の呼び出し元（`TestNewClaudeSendsConfiguredRequest`）。期待する値は変わらない |
| `internal/writer/steps_test.go` | 変更 | `TestWriteStepOrder`（`:43`）に、字数の調整と検証・推敲の呼び出しが `MaxOutputTokens` 0 を渡すことの確認を足す（7.1）。生成は `writer_test.go:153` がすでに確かめている | — |
| `README.md` | 変更 | 3.4 | なし（3.8） |
| `docs/dev/project_overview.md` | 変更 | 3.4 | なし（3.8） |
| `docs/dev/developer_guide/package_reference.md` | 変更 | 3.4 | なし（3.8） |
| `docs/dev/developer_guide/claude_output_limit.md` | 新規 | 3.5 | — |
| `CLAUDE.md` | 変更 | Quick Links にリンクを足す | なし（3.8） |

`internal/writer` と `internal/llm/deepseek` のコードは変えない。

### 3.8. 文書を読む既存のテスト

本書が変える文書を読む既存のテストと、本書の変更との関係は次のとおりである。どのテストも、本書が変える箇所（README の `YT2COLUMN_CLAUDE_EFFORT` の行、`max` の注意、`--after` の段落、`project_overview.md` の同じ変数の行、`package_reference.md` の `internal/config` の行、`CLAUDE.md` の Quick Links）の値を読まない。

| テスト | 読む文書と値 |
|---|---|
| `cmd/yt2column/docs_test.go` | README・`project_overview.md`・`security.md` のフラグ、Slack・Claude の統合テストの環境変数の名前、トークン数とコストの行の形 |
| `cmd/yt2column/copied_transcript_docs_test.go`・`extension_docs_test.go`・`codex_review_docs_test.go` | README の拡張機能とコピーした字幕の節、`CLAUDE.md` の `ext-` のターゲットの一覧、コマンドの文書 |
| `cmd/yt2column/output_layout_docs_test.go` | README の出力の例 |
| `cmd/yt2column-server/docs_test.go` | README・`project_overview.md` のサーバの節 |
| `cmd/yt2column-nmhost/main_test.go` | README の Native Messaging のホストの終了コード |
| `internal/pipeline/pipeline_test.go:707`（`TestPackageReferenceListsPackages`） | `package_reference.md` のパッケージの一覧（パッケージ名だけ） |
| `extension/test/serverReasons.test.ts`・`metadataDocs.test.ts` | README の拒否の理由の表と、拡張機能の節の展開のプロンプト |

README を変えるので、CLAUDE.md「Development Notes」に従い、実装では `make ext-test` も実行する。

## 4. エラーハンドリング設計 (Error Handling Design)

### 4.1. エラー型

新しいエラー型も sentinel も足さない。使うものは今までどおりである。

| 状況 | エラー |
|---|---|
| `YT2COLUMN_CLAUDE_EFFORT` が空の値 | `config.ErrMissing` を包む `*config.VarError`（`Name` は変数名、値を含まない） |
| `YT2COLUMN_CLAUDE_EFFORT` が 5 つの値以外 | `config.ErrInvalid` を包む `*config.VarError`（固定の理由 `reasonClaudeEffort`） |
| 応答が V で打ち切られた | `llm.ErrTruncated`。メッセージは `max_tokens 40000` と effort を示し、`text` ブロックがなければ effort を下げるよう案内する（`response.go:297-307`） |
| モデルが V を受け付けない | HTTP ステータスが `200` 以外のエラー。メッセージはモデル名、`max_tokens`、effort を示す（`internal/llm/claude/claude.go:134-142`）。公開文書が挙げるモデルでは起きない（1.4） |
| 無通信で接続が切れた、または 15 分を超えた | 今までどおり、通信の失敗（`ErrTransport`）か、`context.DeadlineExceeded` に当たるタイムアウト |

### 4.2. エラーメッセージ

メッセージの形は変えない。未設定の `YT2COLUMN_CLAUDE_EFFORT` についてのメッセージは出なくなる。

### 4.3. 失敗の費用と時間（H-05）

上限を上げると、失敗した 1 回の呼び出しの料金と、失敗するまでの時間が増えうる。単価は `internal/pricing` の表（`internal/pricing/rates.go:96-107`。claude-opus-5-5 は入力 4 USD・出力 20 USD／百万トークン、claude-sonnet-5-5 は入力 2 USD・出力 10 USD／百万トークン）を使い、入力は roNZ-rLxhUE-1-f2 と同じ 23,857 トークンとした。

| 状況 | 出力 | 推定コスト | 失敗するまでの時間の見積もり | 料金の表示 |
|---|---|---|---|---|
| 今の既定の上限で打ち切り（Opus。0018 §4.7.4 の実測） | 16,000 | 0.415428 USD | 約 2.5 分（実測） | CLI の `cost:` の行に出る |
| V で打ち切り（Opus、`high` または `max`） | 40,000 | 0.895428 USD | 約 5.8〜7.0 分 | 同上 |
| V で打ち切り（Sonnet、`high` または `max`） | 40,000 | 0.447714 USD | 約 4.6〜7.0 分 | 同上 |
| タイムアウト、または無通信による切断 | 不明（最大 V） | 不明（Opus で最大 0.895428 USD 程度） | 最大 15 分 | 出ない。応答がないので使用量が分からない（`internal/llm/llm.go:72-75`） |

-   打ち切りの行の時間は、出力の速さを 0009 の Opus（114.5）または Sonnet（145）から最も遅い出力の速さ（94.9）までとして計算した。
-   タイムアウトになるのは、実際の出力の速さが 40,000 ÷ 900 秒 ≒ 44.4 トークン/秒を下回り、しかも V 近くまで出力が続く場合である。16,000 では 17.8 トークン/秒を下回る場合だったので、出力が長い呼び出しでは、速さの低下に対する余裕が小さくなる。ただし、観測した出力の最大（15,884）までの呼び出しなら、タイムアウトになるのは速さが 17.6 トークン/秒を下回る場合で、今と変わらない。
-   1 回の実行は最大 3 回 LLM を呼ぶ。後の段階で失敗した実行の料金は、前の段階の料金を含む。
-   localhost サーバは、同時に 2 件までのリクエストを受け付け、待ち行列を持たない（`maxInFlight`。`internal/server/job.go:16-19`）。長い呼び出しの間、その件は枠を 1 つ占め、ほかの送信は断られる。1 回の呼び出しが枠を占める時間の上限は 15 分で、今までどおり変わらない（1 件で最大 3 回呼ぶので、1 件では最大 45 分になりうる）。

料金に上限は設けない（要件書 4.3）。effort `max` は、0009 §1.4 のとおり V まで推論に使って打ち切られると見込まれ、そのときの料金は上の表の値になる。

### 4.4. 副作用の契約と、既定によって止まらなくなる設定

本タスクは、フラグもモードも足さない。既定の effort によって、今まで `YT2COLUMN_CLAUDE_EFFORT` の未設定で送る前に止まっていた設定が、Anthropic へリクエストを送るようになる（要件書 4.2）。送る相手、送るデータの種類、書き込むファイル、投稿先は、その設定で effort に `high` を指定した場合と同じである。

これは、今まで設定の誤りとして止まっていた実行が、`high` で進むことでもある。例えば、変数名を書き間違えた場合（`YT2COLUMN_CLAUDE_EFORT=low`）や、サーバを利用者のシェルの環境を引き継がない方法で起動した場合である。どちらも、意図した effort ではなく `high` で記事を作り、`low` などより料金が高い。要件書は、使った effort を出力（標準エラーの要約、`--out` のフロントマター、Slack の投稿）に出さないと決めており（要件書「要件定義の際に決めたこと」、2.3）、本書もサーバの起動の行（`cmd/yt2column-server/run.go:185-189`）に effort を足さない。この見えにくさは受け入れる。使った effort は、成功した実行の出力からは分からない。分かるのは次の場合に限る。

-   設定を読める場合: `YT2COLUMN_CLAUDE_EFFORT` が未設定なら `high` である（README の表）。ただし、変数が実行の環境に届いていない場合は、設定を見ても分からない。
-   失敗した場合: 打ち切りと HTTP ステータスのエラーのメッセージは、送った effort と `max_tokens` を示す（4.1）。

### 4.5. 元に戻す方法

-   既定の上限で問題が起きた場合（`400` やタイムアウトが続くなど）、利用者が変える手段はない（要件書 2.3）。元に戻すには、定数 `defaultMaxOutputTokens` を戻したビルドが要る。設定やキャッシュの移行は要らない。
-   中間生成物の `id` は内容から計算した値で、`max_tokens` と effort を含まない（`internal/artifact/format.go:488-515` の `recordID`。`internal/writer/artifact.go:133-151` の `Digest`・`MaterialID` を使う）。そのため、本タスクの前に保存した中間生成物からも `--after` で再開できる。
-   localhost サーバは起動のときに設定を読むので、更新したビルドの振る舞いは、サーバを起動し直してから反映される。
-   利用者が実行のときに変えられるのは effort だけである。打ち切りが続く場合は、`YT2COLUMN_CLAUDE_EFFORT` で effort を下げられる（README、AC-19）。

## 5. セキュリティ考慮事項 (Security Considerations)

### 5.1. 脅威モデル

矢印 A → B は「A から B へデータが流れる」を表す。本タスクは境界も流れも変えず、赤の要素はない。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    ENV[("環境変数")]
    CFG["internal/config"]
    CL["internal/llm/claude"]
    API[("Anthropic<br>Messages API")]
    RESP[("応答の本文")]

    ENV --> CFG
    CFG -->|"secret.Secret・claudeparam.Effort"| CL
    CL -->|"素材・プロンプト・max_tokens"| API
    API --> RESP
    RESP -->|"8 MiB まで読み、検証する"| CL

    class ENV,API,RESP data
    class CFG,CL enhanced
```

Legend:

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    L1[("データ・外部のサービス")]
    L2["値が変わるコンポーネント"]
    class L1 data
    class L2 enhanced
```

応答の本文は、今までどおり信頼しない入力として扱い、検証してから使う（0009 要件書 3.2）。

### 5.2. 秘密情報

変えない。`YT2COLUMN_CLAUDE_EFFORT` の拒否のエラーは、今までどおり変数名と固定の理由だけを持つ（AC-04）。既定の effort は秘密情報ではない。

### 5.3. ネットワークと送るデータ

送る相手（`https://api.anthropic.com/v1/messages`）と送るデータの種類は変えない（security.md §4）。変わるのはリクエスト本文の `max_tokens` の値と、未設定のときの `output_config.effort` の値だけである。

### 5.4. 応答の本文の大きさ（H-04）

応答の本文は `llmhttp.MaxResponseBytes`（8 MiB = 8,388,608 バイト。`internal/llm/llmhttp/llmhttp.go:19`）まで読む。0009 §1.4 では、64,000 トークンの打ち切りの応答の本文が 245,601 バイト（約 3.84 バイト/トークン。大部分は `thinking` ブロックの `signature`）だった。同じ比で、40,000 トークンの応答は約 154 KB であり、上限の約 54 分の 1 である。上限は変えない。

### 5.5. 対象クライアント環境の検証

N/A。本タスクは投稿先（Mattermost）への送り方を変えない。外部の API の新しい機能も使わない（`max_tokens` は 0009 から送っているフィールドで、値だけを変える）。値を受け付けることは 1.4 の (c) で確かめた。

## 6. 処理フローの詳細 (Processing Flow Details)

### 6.1. effort の決まり方

矢印は処理の順を表す。

```mermaid
flowchart TD
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    S(["LoadFor / LoadServer"]) --> Q1{"実行する段階に<br>claude の段階があるか"}
    Q1 -->|"ない"| N["YT2COLUMN_CLAUDE_EFFORT を読まない<br>（ClaudeEffort は EffortUnset）"]
    Q1 -->|"ある"| Q2{"YT2COLUMN_CLAUDE_EFFORT"}
    Q2 -->|"未設定"| DFLT["high（defaultClaudeEffort）"]
    Q2 -->|"空の値"| MISS["ErrMissing"]
    Q2 -->|"5 つの値のいずれか"| VAL["その値"]
    Q2 -->|"それ以外"| INV["ErrInvalid"]

    class S,Q1,N,MISS,VAL,INV process
    class Q2,DFLT enhanced
```

Legend:

```mermaid
flowchart LR
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    L1["変えない処理"]
    L2["変わる処理"]
    class L1 process
    class L2 enhanced
```

実行する段階に claude の段階がないとき、`ClaudeEffort()` は `EffortUnset` のままで、Claude アダプタは作られない（`internal/llm/provider/provider.go:55-82` は、そのときプロバイダが `deepseek` なので `claude` の分岐に入らない）。

### 6.2. 既定の上限が送られるまで

`internal/writer` の 3 つの段階が `MaxOutputTokens` 0 で `Generate` を呼び、Claude アダプタの `effectiveMaxTokens` が 0 を V に変えて `max_tokens` に入れる（2.3 の図）。正の `MaxOutputTokens`（統合テストの 1,024 と 16）は、そのまま送られる（AC-09）。

## 7. テスト戦略 (Test Strategy)

### 7.1. 単体テスト

-   **既定の effort（AC-01・AC-02）。** `internal/llm/provider` のテストで、`YT2COLUMN_CLAUDE_EFFORT` を設定しない環境から `config.Load` した設定でアダプタを作り、ループバックの記録用サーバ（`claudeRequestFor`、`provider_test.go:310-332`）に送ったリクエストの `output_config.effort` が `high` であることを確かめる。生成と検証・推敲の両方が `claude` の環境（AC-01）と、一方だけが `claude` の 2 つの組み合わせ（AC-02）を対象にする。検証・推敲のアダプタを確かめるために、`claudeRequestFor` を、生成のプロバイダとモデルだけでなく検証・推敲のものでも作れるようにする。`internal/config` のテストには、同じ 3 つの環境で `ClaudeEffort()` が `EffortHigh` になることを足す。サーバの読み込みの経路も、`LoadServer` で effort を設定しない環境の行を 1 つ足して確かめる。
-   **設定した値（AC-03）。** 既存の `TestLoadClaude`（`config_test.go:219-269`）と `TestNewClaudeSendsConfiguredRequest`（`provider_test.go:257-308`）が 5 つの値を確かめている。変えない。
-   **空の値と不正な値（AC-04）。** 既存の `TestLoadClaudeMissing` の `"effort empty"`、`TestLoadClaudeInvalid`（`config_test.go:300-317`）、拒否した値がメッセージに漏れないことを確かめるテストの行（`config_test.go:720`）が確かめている。変えない。`TestLoadClaudeMissing` の `"effort unset"` の行は外す（3.6）。行を外す前後で `go tool cover -func ./internal/config` の関数ごとの値を比べ、`loadClaudeEffort` のカバレッジが新しい既定のテストで保たれていることを確かめる（CLAUDE.md「Deleting a test is a claim that must be checked」）。
-   **claude の段階がない場合（AC-05）。** 既存の `TestLoadIgnoresOtherProviderVars`（`config_test.go:380-457`）が、DeepSeek の環境で effort の未設定・空・不正・有効のどれでも読み込みが成功することを確かめている。`--after` については、既存の `TestLoadFor` の `"no stage reads no LLM variable"`（`config_test.go:1119-1133`）が `Need{}` を確かめている。`TestLoadFor` に、次の 2 つの組み合わせを足す。1 つ目は、検証・推敲だけが `claude` で `Need{Generation: true}` の場合である。2 つ目は、生成が `claude` で検証・推敲が `deepseek`、`Need{Review: true}` の場合である。どちらも、effort の未設定・空・不正で結果が変わらないことを確かめる。
-   **API キー（AC-06）。** effort を設定しない環境で `ANTHROPIC_API_KEY` が未設定の行を `TestLoadClaudeMissing` に足す。`assertVarError` が見るのは最初の `*VarError` だけなので、この行では、エラーのどの `*VarError` も `YT2COLUMN_CLAUDE_EFFORT` を名指ししないことも確かめる。
-   **V の (a)・(b)（AC-07）。** `internal/llm/provider` のテストで、`MaxOutputTokens` 0 のリクエストを Claude アダプタからループバックに送り、送った `max_tokens` について次の 2 つを確かめる。(a) 観測した出力の最大の 2 倍以上である。(b) 最も遅い出力の速さで出力したときの時間が `LLMTimeout` の半分以内である。観測した出力の最大（15,884）と最も遅い出力の速さ（1,364 トークンを 14.374 秒）は、テストの名前付きの値とし、出典（本書 1.4）をコメントに書く。上限のガイドの「What to update」にこれらの値を挙げる（3.5）。このテストは、定数を 31,767 にしたときは (a) の確認で、42,703 にしたときは (b) の確認で失敗しなければならない。そうなることは、実装のときに実際に定数を変えて確かめる（本節の最後の項目）。このテストは、定数とそれ自身の値を比べるテストではない（CLAUDE.md「Do not assert that a constant equals its own literal」に当たらない）。既存の `TestGenerateMaxTokens` の `"zero sends the default"` はそのまま残す。
-   **3 つの段階（AC-08）。** 2 つの層に分けて確かめる（CLAUDE.md「A layered path needs inputs only one layer can handle」）。1 つ目の層は `internal/writer` で、偽の `LLMClient` が受け取った 3 つの段階のリクエストの `MaxOutputTokens` がどれも 0 であることを確かめる。生成は `writer_test.go:153` が確かめており、字数の調整と検証・推敲の確認を `TestWriteStepOrder`（`steps_test.go:43`）に足す。2 つ目の層は Claude アダプタで、0 を受け取ると V を送ることを `TestGenerateMaxTokens` が確かめる。偽の `LLMClient` は 0 を変換しないので 1 つ目の層だけを、`TestGenerateMaxTokens` は writer を通らないので 2 つ目の層だけを確かめる。
-   **正の値（AC-09）。** 既存の `TestGenerateMaxTokens` の `"positive sends its value"` が確かめている。
-   **打ち切り（AC-10）。** 既存の `TestGenerateValidationOrder`（`response_test.go:160-181`）が、`text` ブロックのない打ち切りで `llm.ErrTruncated` になり、メッセージが既定の上限の値と effort を下げる案内を含むことを確かめている。定数を参照しているので、値を V に変えても、そのまま V を確かめることになる。
-   **DeepSeek（AC-11）。** 本タスクは DeepSeek アダプタを変えない。既存の DeepSeek のテスト（`internal/llm/deepseek/deepseek_test.go:282-284` と、同じファイルの `TestGenerateMaxTokens`（`:287`）の、`MaxOutputTokens` 0 で `max_tokens` を送らないことの確認）がそのまま確かめる。
-   **タイムアウト（AC-12）。** 既存のテストが確かめている。`claudeRequestFor` はアダプタが 15 分のタイムアウトを受け取ることを（`provider_test.go:318-320`）、`cmd/yt2column/run_test.go:633` は CLI のメッセージの「15-minute」を、`cmd/yt2column-server/run_test.go:426-427` は停止の通知の「about 45 minutes」を確かめている。変えない。
-   **各テストが失敗しうること。** 実装では、次の変更でそれぞれ新しいテストが失敗することを確かめ、コミットメッセージに書く（CLAUDE.md「Every test must be able to fail for its stated reason」）。既定の effort の代入を外す。`defaultMaxOutputTokens` を 31,767 と 42,703 にする。writer の 3 か所の `MaxOutputTokens` を、1 か所ずつ順に 1 にする。

### 7.2. 統合テスト

実 API を呼ぶ統合テスト（`internal/llm/claude/integration_test.go`）は、正の `MaxOutputTokens`（1,024 と 16）を渡すので、本タスクの影響を受けない（0009 §3.6）。本タスクで実 API の呼び出しは行わない。

### 7.3. セキュリティテスト

新しいテストは要らない。effort の拒否のエラーが値を含まないことは、既存の、拒否した値がメッセージに漏れないことを確かめるテストの行（`config_test.go:720`）が確かめている。

### 7.4. 受け入れ基準と設計要素の対応

| AC | 設計要素 | 確かめ方 |
|---|---|---|
| AC-01 | 3.1 | 7.1（provider と config の新しいテスト、`LoadServer` の行） |
| AC-02 | 3.1 | 7.1（同上、一方だけが claude の 2 つの組み合わせ） |
| AC-03 | 3.1（変えない） | 既存の `TestLoadClaude`・`TestNewClaudeSendsConfiguredRequest` |
| AC-04 | 3.1（変えない） | 既存の `TestLoadClaudeMissing`・`TestLoadClaudeInvalid`・値が漏れないことのテスト |
| AC-05 | 1.1・6.1 | 既存の `TestLoadIgnoresOtherProviderVars`・`TestLoadFor`、足す 2 つの組み合わせ |
| AC-06 | 3.1 | 7.1（足す行） |
| AC-07 | 3.2・1.4 | 7.1（provider の (a)・(b) のテスト）、既存の `TestGenerateMaxTokens` |
| AC-08 | 3.2・6.2 | 7.1（writer とアダプタの 2 つの層） |
| AC-09 | 3.2（変えない） | 既存の `TestGenerateMaxTokens` |
| AC-10 | 3.2・4.1 | 既存の `TestGenerateValidationOrder` |
| AC-11 | 1.1（DeepSeek は変えない） | 既存の DeepSeek のテスト |
| AC-12 | 3.3 | 既存のテスト（7.1） |
| AC-13 | 1.4 | PR のレビューで読む（manual） |
| AC-14 | 3.4 | PR のレビューで読む（manual） |
| AC-15 | 3.4 | PR のレビューで読む（manual） |
| AC-16 | 3.4 | PR のレビューで読む（manual） |
| AC-17 | 3.5 | PR のレビューで読む（manual） |
| AC-18 | 3.5 | PR のレビューで読む（manual） |
| AC-19 | 3.4 | PR のレビューで読む（manual） |

AC-13〜AC-19 は文書の文面なので、テストでは確かめない（CLAUDE.md「Document tests check values, structure, and executable examples, never prose」）。文書に数値を写さないので（1.2）、値の一致を確かめるテストも要らない。

## 8. 実装の優先順位 (Implementation Priorities)

1.  **既定の effort（F-001）:** `internal/config` の変更と、config・provider のテスト。
2.  **既定の上限（F-002）:** `defaultMaxOutputTokens` の変更、`LLMTimeout` の doc コメント、provider の (a)・(b) のテストと writer のテスト。
3.  **文書（F-003）:** README、`project_overview.md`、`package_reference.md`、上限のガイド、`CLAUDE.md` のリンク。

1 と 2 は互いに依存しない。3 は 1 と 2 の振る舞いを書くので最後にする。3 つとも小さいので、1 つの PR にまとめてよい（PR の分け方は `03_implementation_plan.md` で決める）。元に戻す方法は 4.5 のとおりで、移行の手順は要らない。

## 9. 将来の拡張性 (Future Extensibility)

-   **ストリーミング。** 出力がさらに増えて (b) を満たせなくなった場合や、無通信による切断が実際に観測された場合は、ストリーミングを別の issue にする（0009 要件書 §2.3、本書 1.2）。
-   **段階ごとの上限。** 字数の調整と検証・推敲の出力は生成より小さい（1.4）。料金の上限が必要になった場合は、段階ごとの上限を別の issue で検討する。今は作らない（YAGNI）。
-   **測り直し。** テンプレート、effort、モデルを変えたときは、上限のガイドの手順で測り直し、V、(a)・(b) のテストの名前付きの値、根拠の記録を更新する。

## 付録 A. 設計への申し送り（H-01〜H-08）への対応

| ID | 対応 |
|---|---|
| H-01 | 1.4。使った測定、出力の速さの計算（1 回の実行で LLM を 1 回だけ呼んだ 29 件。CLI の所要時間による遅めの見積もり、並列の起動、長い出力の速さの根拠を含む）、(a)・(b) の範囲、選んだ倍率（約 2.52 倍）、16,000 で打ち切られた実行と 40 分前後の動画の扱い、Sonnet 5.5 の扱いを記録した |
| H-02 | 1.4。公開文書で、現行の 4 つのモデルと旧世代の 9 つのモデルのモデルの出力の上限を確かめた（2026-10-25）。最も小さいものでも 64K で、V を超えるモデルはないので、README には案内を足さない。README の `max` の注意は 3.4 で直す |
| H-03 | 1.2・3.3・3.6・4.3。公開文書の今の記述（16,000 の数値はなく、大きな `max_tokens` にはストリーミングを勧める）と、公式 SDK の確認（しきい値約 21,333、V はその約 1.9 倍）を記録した。SDK の確認は本アダプタに当てはまらない。無通信のリスクは受け入れ、HTTP/2 の PING は足さない。TCP の keep-alive はプロキシを経由する区間には効かないこと、0009 の観測が `curl` によることを記録した。切断とタイムアウトの費用は 4.3 に書いた。`LLMTimeout` の doc コメントを直す |
| H-04 | 5.4。V の応答の本文は約 154 KB と見積もられ、8 MiB の上限に十分に収まる |
| H-05 | 4.3。V で打ち切られた 1 回の推定コスト（Opus 0.895428 USD、Sonnet 0.447714 USD）を、16,000 での実測（0.415428 USD）と並べて記録した。タイムアウトと切断の費用は分からず、表示もされないことを記録した。README には料金を書かない（3.4） |
| H-06 | 1.2・3.1。既定値は `internal/config` の `defaultClaudeEffort`（値は `claudeparam.EffortHigh`）に置く。`EffortUnset` の意味と、アダプタがそれを拒否する仕組みは変えない。アダプタには既定値を持たせない。doc コメント 2 か所を直す。既定は `loadClaudeEffort` の中だけで当てるので、`--after` で実行しない段階には及ばない |
| H-07 | 3.6・3.7・7.1。`TestLoadClaudeMissing` の `"effort unset"` の行を外し、既定のテストに置き換える（カバレッジを `go tool cover -func` で比べる）。AC-08 は writer とアダプタの 2 つの層で確かめる。AC-07 は送った値を (a)・(b) の条件と比べる。AC-12 は既存のテストが確かめているので変えない |
| H-08 | 3.5。ファイル名は `claude_output_limit.md` とした。人の承認と評価の手順への参照、段階ごとの測り方、集める値の出どころを書く。V の値と出力の速さは写さず、今の根拠の記録の場所を示すので、文書の値の一致を確かめるテストは要らない。(a)・(b) はテストの名前付きの値で固定し、ガイドの更新の対象に挙げる |
