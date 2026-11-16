# アーキテクチャ設計書：chief が Claude Code の質問を transcript から判定し、答えの中継の照合も transcript で行う

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-17 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（要件定義書。以下、要件書）の設計である。「計画で決める」は、本タスクの実装計画書（`03_implementation_plan.md`）で決めることを指す。既存のファイルの内容は、コミット `97ca1fc7` で確かめた。`file:line` の行番号は、このコミットでの行番号である。F-NNN・AC-NN は要件書の項番を、H-NN は [design_handoff.md](design_handoff.md)（以下、申し送り）の項番を指す。申し送りへの対応は付録 A に、受け入れ基準との対応は付録 B にまとめる。

本書は、要件書 3 章と 6 章の用語（呼び出し・答え・答えを待っている呼び出し・質問の呼び出し・サブエージェントの呼び出し・権限の確認・中継・照合値・transcript）と、[0088 設計書](../0088_chief_of_staff/02_architecture.md)（以下、0088 設計書）の冒頭の用語（道具・`/chief` の定義・周・状況の表・送信の記録など）を、同じ意味で使う。そのほかに、次の用語を使う。

-   **待ち状態:** Claude Code の pane 1 つについて、transcript の答えを待っている呼び出しと Herdr の状態から決めた、その pane が何を待っているかの判定（3.3）。「要確認」「なし」「質問」「権限の確認」の 4 つのどれかである。
-   **画面の確かめ:** 答えを中継する直前に、pane の画面に transcript の問いと同じ 1 つを選ぶ問いが出ていることを確かめること（3.5.3、AC-15）。

## 1. 設計の全体像 (Design Overview)

### 1.1. 設計原則

-   **Claude Code の pane の質問は、transcript と Herdr の状態だけで決める。** 状況の表、人の判断を待つものの一覧、送る前の確かめの「質問で止まっていない」は、待ち状態から決める。Claude Code の pane の画面は、状況の表の判定に使わない（AC-26）。
-   **画面は送るのを止めるためだけに使う。** 中継の直前の画面の確かめ（AC-15）と、送る前に入力欄の形を求める今の確かめ（AC-25）は、通らなければ送らないだけで、送る理由にはならない。
-   **判定は 3 つの層に分ける。** transcript の中身を調べる読み取り（`screen` で始まる名前のファイル）、読み取りの結果の型と Herdr の状態だけで決める判定、その判定を使う表と送信の 3 つである。中身を調べるのは読み取りの層だけで、CLAUDE.md の「Declare, don't infer」の例外の範囲をそのファイルに閉じる。
-   **読めなければ推し量らない。** transcript を読めないとき、受け付ける形に合わないとき、transcript と Herdr の状態が食い違うときは、「要確認」にするか送らない。一部を読んだ結果は使わない（4.2、4.3）。
-   **今ある部品を使う。** transcript の場所と開き方（`internal/chief/transcript.go`）、厳密な JSON の読み取り（`internal/strictjson`）、1 つを選ぶ問いの画面の読み取り（`claudeOneQuestion`、`internal/chief/screen_claude.go:228`）、`/runpr` のマージの確認の文の形（`mergeQuestionText`、`internal/chief/screen.go:113`）、制御文字の置き換え（`Sanitize`、`internal/chief/sanitize.go:75`）を使い、二重に持たない（4.5）。

### 1.2. 設計の際に決めたこと（2026-11-17）

-   **transcript は、状況の表を作るたびに、その都度すべてを読む。前回の位置から続きを読む仕組みは作らない（申し送りの H-01、4.1）。** 手元の 25 MB の transcript をメモリの中で 64 MiB まで繰り返し、今の `strictjson` で各行を読む形で、読み取りと判定の時間を測った（2026-11-17、Apple M3 Pro）。3 回で 0.31〜0.42 秒で、1 秒の予算に収まった。CLAUDE.md の「Performance」は、仕組みを足す前に実際の時間を測ることを求める。続きから読む仕組みは、ファイルの差し替えや切り詰めの見分けという正しさの義務を足すので、採らない。ADR: なし（このタスクの中で閉じる読み方の細部のため）。
-   **使わなくなる Claude Code の画面の読み取り（複数を選ぶ問い、タブのある問いとその確認のページ、権限の確認）は、このタスクでは消さず、#908 に残す（申し送りの H-07）。** 消しても送るものは増えない。送る前の確かめ（`internal/chief/send.go:333-337`、`internal/chief/autosend.go:384-385`）は、知らない形の画面でも止まるからである。ただし、そのときに示す理由は「質問を示している」から「どの形にも合わない」に変わる。また、テストの削除を単独のコミットにしてカバレッジを比べる手間が、このタスクの差分に加わる。人が #908 に残すことを選んだ。ADR: なし（このタスクの中で閉じる範囲の分け方のため）。
-   **照合値には、今と同じく Herdr の `state_change_seq` を入れる（申し送りの H-02）。** 人に示した後に Herdr の状態が変わっていれば、中継は送らない。今の照合値はこの値を含み（`internal/chief/relay.go:41`）、0088 の実地の確かめで、質問の間に値が変わらず中継が通ることを確かめている。値を外して呼び出しの ID と中身だけにする案は、守りを緩める理由がないので採らない。人もこの案を選んだ。ADR: なし（照合値は中身を読めない内部の値で、その作り方はこのタスクの中で閉じるため）。
-   **答えを待っている間の transcript と Herdr の状態のサンプルは、実装の最初の段で採る（申し送りの H-05）。** 確かめるのは、Herdr が権限の確認の間に `blocked` と報告するかと、答えを待っている間に transcript に書かれる行である。どちらも、人が採取用の pane（`chief-poc`）で状態を作る必要がある。前提が成り立たなくても、AC-13 のとおり権限の確認が示されないだけで、送るものは増えない。設計もどちらの結果でも変わらない。前提が成り立たないと分かれば、要件書を `draft` に戻して決め直す（要件書 1 章の背景）。人もこの案を選んだ。ADR: なし（確かめの時期はこのタスクの中で閉じるため）。
-   **サブエージェントの見分け（申し送りの H-04）は、要件書のとおり道具の名前 `Agent` で行う。サブエージェントの行は親の transcript に書かれないことを確かめたので、要件書は直さない。** 手元の Claude Code のディレクトリ（2026-11-17）で確かめた。サブエージェントの行は、親のセッションの下の `subagents/agent-*.jsonl`（2,134 ファイル）に書かれていた。直近 3 日の親の transcript 88 ファイルには、`isSidechain` が `true` の行が 1 行もなかった。同じ 88 ファイルの呼び出しの道具の名前は、`Agent` のほかに `Bash`・`Read`・`AskUserQuestion` などで、サブエージェントを起動する別の名前の道具は見当たらなかった。ADR: なし（要件書のとおりのため）。
-   **答えを待っている間に書かれる行の種類は、答えた後の transcript から確かめた（申し送りの H-05、H-06）。** 直近 3 日の 88 ファイルで、質問の呼び出しとその答えの間にあった行の `type` は、`api-request`・`api-request-blob`・`session-owner`・`atis-latch`・`mode`・`last-prompt`・`ai-title` だけだった。どれも、4.2 の「広げてよいと宣言する部分」に当たり、読まずに受け付ける。質問の呼び出しの入力 113 件は、どれも `questions` の各要素に `header`・`multiSelect`（真偽値）・`options`・`question` を持ち、`options` の各要素に `label` と `description` を持っていた。どれも 4.2 の受け付ける形に合っていた。ADR: なし（確かめた事実の記録のため）。

### 1.3. なぜ今の仕組みでは足りないか

今は、Claude Code の pane の質問を、`parseClaudeScreen`（`internal/chief/screen_claude.go:69`）が Claude Code 2.1.322 の画面のサンプルの形で読む。画面の形は Claude Code の版で変わるので、版が上がると、その pane はサンプルを採り直すまで「要確認」になり、答えも中継できない（要件書 1 章の背景）。画面の読み取りを直し続ける案では、この問題は版ごとに繰り返す。transcript には、同じ問いと選択肢が版に依らない構造で書かれているので、情報源を移す（要件書の「issue #904 で決めたこと」）。

これは、0088 設計書 3.4（画面の読み取り）と 3.6（答えの中継）の方針を、Claude Code の pane について変える。その方針は「質問と選択肢は画面から読み、中継の照合値も画面から作る」である。変える理由は要件書の「要件定義の際に決めたこと（2026-11-16）」にあり、決定は [ADR-0046](../../dev/adr/0046-chief-of-staff.md) に記録済みである。今の振る舞いを確かめている既存のテストは、3.9 に挙げる。OpenCode の pane の方針は変えない。

### 1.4. 概念モデル

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef newpkg fill:#ffe8f5,stroke:#d946ef,stroke-width:2px,color:#701a75;

    TR[("transcript<br>(Claude Code が書く)")]
    ST[("Herdr の状態<br>(blocked・working・idle・done)")]
    SC[("pane の画面")]
    CALLS["答えを待っている呼び出し<br>(PendingCall)"]
    WAIT["待ち状態<br>(ClaudeWait)"]
    ROW["状況の表の行と<br>人の判断を待つもの"]
    QUIET["送る前の確かめ<br>(コマンドの送信・自動送信・削除)"]
    RELAY["答えの中継"]

    TR -->|"読み取り"| CALLS
    CALLS --> WAIT
    ST --> WAIT
    WAIT --> ROW
    WAIT --> QUIET
    WAIT --> RELAY
    SC -.->|"止めるためだけ"| QUIET
    SC -.->|"止めるためだけ"| RELAY

    class TR,ST,SC data
    class ROW,QUIET,RELAY enhanced
    class CALLS,WAIT newpkg

    subgraph Legend["Legend"]
        L1[("情報源")]
        L2["変える部品"]
        L3["新しい型"]
    end
    class L1 data
    class L2 enhanced
    class L3 newpkg
```

実線の矢印 A → B は、B が A から作られることを表す。点線の矢印は、画面が B の送信を止める向きにだけ働くことを表す。Claude Code の pane の画面から待ち状態への矢印はない。

## 2. システム構成 (System Structure)

### 2.1. 全体構成

```mermaid
flowchart TD
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef newpkg fill:#ffe8f5,stroke:#d946ef,stroke-width:2px,color:#701a75;

    FILE[("transcript のファイル<br>(&lt;config&gt;/projects/&lt;dir&gt;/&lt;session&gt;.jsonl)")]
    HERDR[("herdr agent get・<br>herdr agent read")]
    LOC["transcript.go<br>transcriptPath・readTranscript"]
    RD["screen_transcript_calls.go<br>readPendingCalls"]
    JD["claudewait.go<br>judgeClaudeWait"]
    RW["snapshot.go<br>readWorkspace"]
    CL["state.go<br>classify"]
    QT["autosend.go<br>quietness"]
    RL["relay.go<br>Relay"]
    SA["screen_claude.go<br>claudeScreenAgrees"]
    OUT["output.go・pending.go<br>QuestionOutput・questionID"]

    FILE --> LOC --> RD --> JD
    HERDR --> RW
    RW --> LOC
    JD --> RW
    RW --> CL --> OUT
    RW --> QT
    RL --> LOC
    RL --> JD
    HERDR --> RL
    RL --> SA

    class FILE,HERDR data
    class LOC,RW,CL,QT,RL,SA,OUT enhanced
    class RD,JD newpkg

    subgraph Legend["Legend"]
        L1[("外部の値")]
        L2["変えるファイル"]
        L3["新しいファイル"]
    end
    class L1 data
    class L2 enhanced
    class L3 newpkg
```

矢印 A → B は、B が A を呼ぶか読むことを表す（データの流れの向き）。どのファイルも `internal/chief` の中にある。

### 2.2. コンポーネント配置

| ファイル | 種類 | 責務 |
|---|---|---|
| `internal/chief/screen_transcript_calls.go` | 新規 | transcript の各行を 4.2 の受け付ける形で読み、答えを待っている呼び出しの一覧を返す。質問の呼び出しの入力を問いと選択肢に読み、`/runpr` のマージの確認を見分ける（3.2）。transcript の中身を調べるので、名前を `screen` で始め、CLAUDE.md の例外の範囲に入れる |
| `internal/chief/claudewait.go` | 新規 | 答えを待っている呼び出しの一覧と Herdr の状態から待ち状態を決める（3.3）。読み取りの層が返した型だけで決め、文字列の中身は調べない |
| `internal/chief/transcript.go` | 変更 | 上限を超えたときの誤りを、型で見分けられる誤り（`errTranscriptLimit`）にする。待ち状態のための読み取りの上限（64 MiB、`resetTranscriptLimit` と同じ値）を使う。ファイルがないことを見分けられるようにする（3.2.3） |
| `internal/chief/screen.go` | 変更 | `Question` に、transcript から読んだ問いのための部分（見出し、複数の問い、道具の名前と入力）を足す。`QuestionKind` と `RelayBlock` に値を足す（3.4.1） |
| `internal/chief/screen_claude.go` | 変更 | 中継の直前に、画面の 1 つを選ぶ問いと transcript の問いが同じかを確かめる `claudeScreenAgrees` を足す（3.5.3）。今の読み取りは変えない |
| `internal/chief/state.go` | 変更 | `agentFacts` に待ち状態を足す。`classify` は Claude Code の pane を待ち状態で分け、画面を見ない（3.4.2）。`QuestionItem` に呼び出しの ID とエージェントが動いていることを足す |
| `internal/chief/snapshot.go` | 変更 | `readWorkspace` が Claude Code の pane の transcript を読み、待ち状態を決める（3.4.2）。画面は今と同じく読む（送る前の確かめと入力欄の判定が使う） |
| `internal/chief/autosend.go` | 変更 | `quietness` が Claude Code の pane の待ち状態を確かめる（3.6）。画面の確かめは今のまま残す |
| `internal/chief/request.go` | 変更 | `offered` が送らない理由に、行が「要確認」のときの理由を添える（3.6、AC-24） |
| `internal/chief/relay.go` | 変更 | Claude Code の pane では、照合値を transcript の呼び出しから作り、送る直前に transcript・Herdr・画面を確かめる（3.5）。OpenCode の pane は今のまま |
| `internal/chief/send.go` | 変更 | 送らない理由に `screen_mismatch` を足す（4.1） |
| `internal/chief/output.go` | 変更 | `QuestionOutput` に見出し、複数の問い、道具の名前と入力、エージェントが動いていることを足す（3.4.3） |
| `internal/chief/pending.go` | 変更 | Claude Code の質問の項目の同一性を、呼び出しの ID から作る（3.4.3） |
| `internal/chief/testdata/transcript/` と `testdata/README.md` | 変更 | 答えを待っている間の transcript と、同じ問いの画面のサンプルを足す（7.1、申し送りの H-05） |
| `internal/chief/*_test.go` | 変更 | 3.9 と 7 章 |
| `.claude/commands/chief.md` | 変更 | step 4 の質問の示し方を、足した出力の欄に合わせる（3.4.3） |
| `CLAUDE.md` | 変更 | 「Declare, don't infer」と「Go Idioms」の chief の画面の読み取りの例外（3.8、AC-27） |
| `docs/dev/guides/chief_guide.md` | 変更 | 1 章の表、5.1 の中継の照合、8 章の中継しない質問と transcript の場所（3.8、AC-28） |

`internal/chief/paneinput`、`cmd/chief`、`internal/strictjson` は変えない。OpenCode の pane の読み取り（`screen_opencode.go`）、報告の読み取り（`screen_report.go`、#905 の範囲）、モデルの切り替えと人の最後の入力の読み取り（`screen_transcript.go`）、リセット（`reset.go`）も変えない。

### 2.3. データフロー

状況の表を作る 1 周（`readRound` から `classify` まで）は、次のように流れる。

```mermaid
sequenceDiagram
    participant R as readWorkspace
    participant H as Herdr
    participant T as transcript
    participant J as judgeClaudeWait
    participant C as classify

    R->>H: herdr agent read（画面）
    H-->>R: 画面（入力欄の判定と送る前の確かめに使う）
    R->>T: readTranscript（64 MiB まで）
    alt 読めない・上限を超える・形が合わない
        T-->>R: errTranscript
    else ファイルがない
        T-->>R: 呼び出しなし（AC-20）
    else 読めた
        T-->>R: 行の並び
        R->>R: readPendingCalls
    end
    R->>J: Herdr の状態と、呼び出しの一覧か誤り
    J-->>R: 待ち状態
    R->>C: workspaceFacts（各 pane の待ち状態を含む）
    C-->>R: 行（質問・要確認・実行中など）
```

Herdr の状態は、`herdr workspace list` から作る `TaskWorkspace` のエージェントの一覧のものを使う（今と同じ）。答えの中継の流れは 6.2 にある。

## 3. コンポーネント設計 (Component Design)

### 3.1. 型の全体

```mermaid
classDiagram
    class PendingCall {
        <<struct>>
        +ID string
        +Tool string
        +Input string
        +Question *Question
    }
    class ClaudeWait {
        <<struct>>
        +Kind ClaudeWaitKind
        +Why string
        +Call *PendingCall
        +Working bool
    }
    class Question {
        <<struct>>
        +Kind QuestionKind
        +Relay RelayBlock
        +Merge bool
        +Header string
        +Text string
        +Options []Option
        +Tabs []string
        +Parts []Question
        +Tool string
        +Input string
        +InputCut bool
        +Cursor int
    }
    class QuestionItem {
        <<struct>>
        +Pane string
        +Agent paneinput.AgentKind
        +Question Question
        +CallID string
        +Working bool
        +Check CheckValue
    }
    class agentFacts {
        <<struct>>
        +Agent Agent
        +ScreenRead bool
        +Screen Screen
        +Wait ClaudeWait
    }
    ClaudeWait --> PendingCall : Call
    PendingCall --> Question : Question
    QuestionItem --> Question : Question
    agentFacts --> ClaudeWait : Wait
```

矢印 A → B は、A が B を欄に持つことを表す。`Question` の `Header`・`Parts`・`Tool`・`Input`・`InputCut`、`QuestionItem` の `CallID`・`Working`、`agentFacts` の `Wait` が足す欄で、ほかは今の欄である（`internal/chief/screen.go:69-87`、`internal/chief/state.go:103-111`・`:149-154`）。

### 3.2. transcript の読み取り（`screen_transcript_calls.go`）

#### 3.2.1. 型

```go
// PendingCall is one call of a Claude Code transcript that has no answer
// yet: a tool_use element of an "assistant" line with no tool_result of a
// later "user" line for its ID.
type PendingCall struct {
	// ID is the call's tool_use ID.
	ID string
	// Tool is the tool's name, such as "AskUserQuestion" or "Bash".
	Tool string
	// Input is the call's input as the line wrote it, as raw JSON text.
	Input string
	// Question is the question an AskUserQuestion call asks, nil for any
	// other tool.
	Question *Question
}

// readPendingCalls reads a transcript and returns its calls that wait for
// an answer, in the order of their lines.
func readPendingCalls(transcript []byte) ([]PendingCall, error)

// errTranscriptForm reports a line of a transcript that is not of the
// accepted form; it is wrapped in errTranscript.
var errTranscriptForm = errors.New("a line of the transcript is not of the accepted form")
```

`readPendingCalls` の契約は次のとおりである。

-   要件書 4.2 の表の受け付ける形・使う部分・使う部分の形・広げてよい部分・拒む入力のとおりに読む。拒む入力は、`errTranscript` に包んだ `errTranscriptForm` を返し、一部を読んだ結果は返さない。
-   行は LF で区切る。最後の LF より後のバイト（改行で終わっていない最後の行）は書きかけとして読まない（AC-21）。空白だけの行は読まない。
-   各行は `strictjson.ParseObject` で読む。UTF-8 として正しくないバイト、対になっていないサロゲート、1 行に 2 つ以上の値は、そこで拒まれる。メンバーの重複は、使う部分のメンバーについて `Collect` で拒む（`internal/strictjson/strictjson.go:73`・`:91`）。
-   `type` が `assistant` と `user` の行だけを読む。そのほかの行（`api-request-blob` など）は、`type` が空でない文字列であることだけを確かめて読み飛ばす（申し送りの H-06）。
-   `assistant` の行では、`message.content` の配列の、`type` が `tool_use` の要素を呼び出しとする。`user` の行では、`message.content` が配列のときに、`type` が `tool_result` の要素を答えとする。文字列のときは答えはない。
-   同じ ID の呼び出しが `assistant` の行どうしで 2 つあれば拒む。同じ呼び出しへの答えが `user` の行どうしで 2 つあれば拒む。答えが、同じ ID の呼び出しより前の行にあれば拒む。対応する呼び出しのない答えは受け付ける。
-   質問の呼び出し（道具の名前が `AskUserQuestion`）の入力は、答えがあるかに関わらず、4.2 の「使う部分の形」で読む。合わなければ拒む。読んだ問いは 3.2.2 の `Question` にする。
-   `Input` は、その要素の `input` のメンバーの値の、行に書かれたとおりの JSON の文字列である（`strictjson.Value.Raw`）。

#### 3.2.2. 質問の呼び出しから作る `Question`

質問の呼び出しの入力から、示すための `Question` を次のように作る。どの値も transcript のとおりで、制御文字は示すときに置き換える（`Sanitize`、AC-01）。

| 呼び出しの問い | `Kind` | `Relay` | 中身 |
|---|---|---|---|
| 問いが 1 つ、`multiSelect` が `false`、選択肢が 1〜9 | `QuestionSingle` | `RelayAllowed` | `Header`・`Text`・`Options` |
| 問いが 1 つ、`multiSelect` が `false`、選択肢が 10 以上 | `QuestionSingle` | `RelayBlockTooMany`（新規） | 同上 |
| 問いが 1 つ、`multiSelect` が `true` | `QuestionMulti` | `RelayBlockMulti` | 同上 |
| 問いが 2 つ以上 | `QuestionSeveral`（新規） | `RelayBlockSeveral`（新規） | `Parts` に、各問いを上の 3 行のどれかで作った `Question`。`Text` と `Options` は空 |

-   **選択肢の番号。** transcript の選択肢に、順に 1 から番号を付ける。その後ろに、Claude Code の問いの部品が描く 2 つの選択肢を、`Relayable` を偽にして足す。`n` を transcript の選択肢の数として、`n+1` が自由に書く答え、`n+2` がチャットへ移る答えである。その名前は、画面の読み取りが使う決まった文字（`claudeFreeText`・`claudeChat` など、`internal/chief/screen_claude.go:15-18`）を使う。人がその pane で答えるとき、pane に出る番号と同じ番号で選べるようにするためである（AC-01）。
-   **マージの確認。** 問いが 1 つで `QuestionSingle` かつ `RelayAllowed` の問いだけを、`mergeQuestionText`（`internal/chief/screen.go:113`）で見分け、合えば `Merge` を真にする（AC-08）。今の画面の読み取り（`finishQuestion`、`internal/chief/screen.go:166-183`）と同じ規則である。文字列はこのファイルに増やさない。
-   **`Cursor`** は 0 のままにする。transcript には選択の位置がない。

#### 3.2.3. 読む量の上限と、ファイルがないとき

-   読む量の上限は 64 MiB である（AC-22）。今の `resetTranscriptLimit`（`internal/chief/transcript.go:30`）と同じ値なので、その定数を待ち状態の読み取りにも使う。名前は計画で決める。上限を超えたときは、`errTranscript` に包んだ `errTranscriptLimit`（新規）を返す。今は理由を文字列で返しているので（`internal/chief/transcript.go:95`）、型で見分けられるようにする。リセットの読み取りの振る舞いは変わらない。
-   transcript のファイルがないとき（`os.OpenFile` が `fs.ErrNotExist` を返す）は、答えを待っている呼び出しがないものとする（AC-20）。場所が決まらないとき（`transcriptPath` の誤り、AC-19）と、ファイルを開けない・通常のファイルでないとき（AC-23）は、今の `openTranscript` の誤りのまま、読めないものとする。

### 3.3. 待ち状態の判定（`claudewait.go`）

```go
// ClaudeWaitKind is what a Claude Code pane waits on. The zero value is
// ClaudeWaitNeedsCheck, the reading that assumes least: nothing is relayed
// or sent to the workspace and its row needs a check.
type ClaudeWaitKind int

const (
	// ClaudeWaitNeedsCheck is a pane whose wait cannot be told; Why says why.
	ClaudeWaitNeedsCheck ClaudeWaitKind = iota
	// ClaudeWaitNone is a pane that waits on no question or permission prompt.
	ClaudeWaitNone
	// ClaudeWaitQuestion is a pane with one AskUserQuestion call waiting.
	ClaudeWaitQuestion
	// ClaudeWaitPermission is a pane blocked with one other call waiting.
	ClaudeWaitPermission
)

// ClaudeWait is the wait of one Claude Code pane.
type ClaudeWait struct {
	Kind ClaudeWaitKind
	// Why says in English what could not be told, for ClaudeWaitNeedsCheck.
	Why string
	// Call is the call waited on, for ClaudeWaitQuestion and
	// ClaudeWaitPermission.
	Call *PendingCall
	// Working is true for a question whose agent Herdr reports working.
	Working bool
}

// judgeClaudeWait decides the wait of a Claude Code pane from Herdr's status
// of its agent and the calls its transcript leaves waiting, or the error
// that kept the transcript from being read.
func judgeClaudeWait(status AgentStatus, calls []PendingCall, readErr error) ClaudeWait
```

`judgeClaudeWait` は、次の表の最初に当たる行で決める。表の「質問」は質問の呼び出し、「サブエージェント」はサブエージェントの呼び出し、「そのほか」はどちらでもない呼び出しである。

| 条件 | 待ち状態 | AC |
|---|---|---|
| transcript を読めない（`readErr` が誤り） | 要確認（読めない理由） | AC-19・AC-21・AC-22・AC-23 |
| 答えを待っている呼び出しが 2 つ以上あり、その中に質問がある | 要確認（質問とほかの呼び出しが同時に待っている） | AC-09 |
| 状態が `blocked`、呼び出しが質問 1 つだけ | 質問 | AC-01 |
| 状態が `blocked`、呼び出しがサブエージェント 1 つだけ | 要確認（サブエージェントが質問か権限の確認で待っているかもしれない） | AC-11 |
| 状態が `blocked`、呼び出しがそのほか 1 つだけ | 権限の確認 | AC-10 |
| 状態が `blocked`、呼び出しがない（ファイルがないときを含む）か、2 つ以上 | 要確認（何を待っているか分からない） | AC-12・AC-20 |
| 状態が `working`、呼び出しが質問 1 つだけ | 質問（`Working` が真） | AC-06 |
| 状態が `working` | なし | AC-13 |
| 状態が `blocked`・`working` 以外で、答えを待っている質問がある | 要確認（transcript と Herdr の状態が食い違う） | AC-07 |
| 状態が `blocked`・`working` 以外 | なし | AC-13 |

-   「`blocked`・`working` 以外」は、`idle`・`done` と、Herdr が報告した知らない状態（`AgentStatusUnknown`）である。知らない状態でも、答えを待っている質問があれば「要確認」になる。質問がなければ「なし」で、今の `classify` が知らない状態を止めないこと（`internal/chief/state.go:401`）と同じである。
-   権限の確認の `Call.Input` を示す量の上限は 2,000 文字（rune）とし、超えた部分は示さず `InputCut` を真にする（AC-10）。切るのは示す `Question` を作るときで、`PendingCall.Input` は切らない。
-   `Why` は英語の文で、pane の ID を含む。示す文言の例は 4.2 にある。

### 3.4. 状況の表と人の判断を待つもの

#### 3.4.1. `Question` に足す値

```go
// Appended to the QuestionKind constants, after QuestionPermission.

	// QuestionSeveral is an AskUserQuestion call with two or more
	// questions, read from a transcript; Parts holds them.
	QuestionSeveral

// Appended to the RelayBlock constants, after RelayBlockPermission.

	// RelayBlockSeveral blocks a call with two or more questions.
	RelayBlockSeveral
	// RelayBlockTooMany blocks a question with more options than a relay
	// key can select.
	RelayBlockTooMany
```

上は今の定数の並び（`internal/chief/screen.go:28-61`）の末尾に足す値の抜き書きである。

`Question` に足す欄（3.1 の図）は、OpenCode の画面からは作られず、空のままである。`Question` の説明は「画面から読んだ問い」から「画面か transcript から読んだ問い」に改める。

#### 3.4.2. `readWorkspace` と `classify`

-   `readWorkspace`（`internal/chief/snapshot.go:230`）は、Claude Code の pane ごとに、今の画面の読み取りに加えて transcript を読み（3.2）、`judgeClaudeWait` で待ち状態を決めて `agentFacts.Wait` に入れる。transcript の場所を決めるのに `Env` が要るので、引数に足す。呼び出し元の 3 か所（`internal/chief/snapshot.go:132`、`internal/chief/request.go:221`、`internal/chief/autosend.go:660`）は、どれも `Deps` を持っている。
-   `classify`（`internal/chief/state.go:382`）は、Claude Code の pane を待ち状態だけで分け、画面の読み取りの成否と形を見ない（AC-26）。

| 待ち状態 | `classify` の扱い |
|---|---|
| 要確認 | `Unreadable` に `Why` を足す。行は「要確認」になる |
| 質問 | `QuestionItem` を足す。`Question` は 3.2.2 のもの、`CallID` は呼び出しの ID、`Working` は待ち状態のもの、`Check` は 3.5.1 の照合値（`Working` が偽で、`RelayAllowed` のときだけ）。`Merge` が真なら `Row.MergeQuestion` を真にする |
| 権限の確認 | `QuestionItem` を足す。`Question` は `Kind` が `QuestionPermission`、`Relay` が `RelayBlockPermission`、`Tool`・`Input`・`InputCut` は呼び出しのもの。`Check` は空 |
| なし | 何も足さない。実行中かどうかは、今と同じく Herdr の状態で決まる（`internal/chief/state.go:418`） |

-   OpenCode の pane は今のままである。
-   質問の行が `working` のエージェントの質問だけのときも、行は「質問で停止」にする（AC-06 の「質問として示す」）。今は、`working` のエージェントの画面は質問として読まない（`internal/chief/state.go:398-400`）ので、ここは変わる。

#### 3.4.3. 出力と `/chief` の定義

`QuestionOutput`（`internal/chief/output.go:59`）に次の欄を足す。今の欄の意味は変えないので、`--protocol` の値は 1 のままにする。

| 欄 | 中身 |
|---|---|
| `kind` | 今の値に `several_questions` と、選択肢が多すぎる 1 つを選ぶ問いを足す。後者は `single_select` のまま `relayable` が偽になる |
| `header` | 問いの見出し。Claude Code の問いが 1 つのときだけ。そのほかは空 |
| `parts` | `several_questions` の各問い（`header`・`text`・`kind`・`options`）。そのほかは空の配列 |
| `tool`・`input`・`input_cut` | 権限の確認の道具の名前、入力、入力を切ったか。そのほかは空と偽 |
| `working` | エージェントが動いているか（AC-06） |

-   どの文字列も、今と同じく `Sanitize` を通す。
-   `questionID`（`internal/chief/pending.go:84`）は、Claude Code の項目について、問いの文ではなく、workspace・pane・呼び出しの ID から作る。同じ呼び出しが示され続ける間は同じ項目になり、同じ文で問い直した別の呼び出しは別の項目になる。OpenCode の項目は今のまま。
-   `.claude/commands/chief.md` の step 4 の「A question」の項（`.claude/commands/chief.md:211-231`）を、次のように直す。`header` があれば示す。`several_questions` は `parts` のすべての問いと選択肢を示す（AC-03）。`permission` は `tool` と `input` を示し、`input_cut` が真なら切ったことを示す（AC-10）。`working` が真なら、エージェントが動いていることを添える（AC-06）。「`tabs` が空でなければ、画面にはほかの問いがある」という今の文は、OpenCode の質問だけに当たると書き直す。

### 3.5. 答えの中継（`relay.go`）

#### 3.5.1. 照合値

Claude Code の質問の照合値は、今の `checkValue`（`internal/chief/relay.go:35`）と同じく、部分ごとに長さを付けて並べた文字列の SHA-256 の 16 進とする。中身を読めない値にすることも、今と同じである（CLAUDE.md の「Keep internals unobservable」）。部分は次のとおりで、先頭の版の文字列を変えるので、画面から作った照合値と等しくならない。

```go
// transcriptCheckVersion is the first part of the check value of a
// question read from a Claude Code transcript.
const transcriptCheckVersion = "chief-check-transcript-1"

// transcriptCheckValue returns the check value of call, an AskUserQuestion
// call waiting in agent a's transcript.
func transcriptCheckValue(a Agent, call PendingCall) CheckValue
```

-   部分：版、エージェントの種類、pane、`agent_session`、`state_change_seq`、呼び出しの ID、問いの見出し、問いの文、`multiSelect`、選択肢の数、各選択肢の `label` と `description`（申し送りの H-02）。
-   選択の位置は入れない。transcript にないためである。中継のキーは、選択の位置に関わらず番号で選ぶ（`internal/chief/screen.go:81-85` の今の説明のとおり）。
-   `relayCheck` は、Claude Code の項目では、待ち状態が質問で、`Working` が偽で、`RelayAllowed` のときだけこの値を返す。

#### 3.5.2. 中継先を見つける

`findRelayTarget`（`internal/chief/relay.go:149`）は、workspace の pane を順に読み、照合値が一致する pane を探す。OpenCode の pane は今のまま `paneQuestion` で画面を読む。Claude Code の pane は次の順に読む。

1.  `herdr agent get` でエージェントを読む。一覧のときと同じエージェントでなければ（`sameAgent`）その pane は飛ばす。
2.  そのエージェントの `agent_session` と作業ディレクトリから transcript を読み、待ち状態を決める（3.2、3.3）。
3.  待ち状態が質問なら、手順 1 のエージェントと呼び出しから照合値を作り、リクエストの値と比べる。

どの pane も一致しないときは、次の順に理由を決める。

| 当たるもの | 理由 | AC |
|---|---|---|
| ある Claude Code の pane の transcript を読めない | `transcriptFailure` の理由（`no_transcript` か `unsupported_path`、`internal/chief/transcript.go:73-79`） | AC-23 |
| ある pane を読めない（Herdr の誤り） | `unreadable`（今のまま） | - |
| ある Claude Code の pane の待ち状態が、`Working` が真の質問 | `not_accepting`（エージェントが質問で止まっていない） | AC-06 |
| そのほか | `check_mismatch`（今のまま） | AC-14・AC-17 |

#### 3.5.3. 送る直前の確かめ

一致した Claude Code の pane には、今の `relay` の確かめ（`internal/chief/relay.go:210-245`：中継する問いか、Herdr が `blocked` と報告しているか、選択肢があり中継するものか）をそのまま当てる。そのうえでキーを送る直前に、次の 2 つを順に確かめる。今の「画面とエージェントを読み直して照合値を比べる」確かめ（`internal/chief/relay.go:247`）は、Claude Code の pane では、この 2 つに置き換わる。

1.  **transcript と Herdr。** エージェントと transcript を 3.5.2 の手順 1・2 で読み直す。エージェントが一致したときの読み取りと同じで、同じ呼び出しが同じ照合値で答えを待っているときだけ進む。そうでなければ `check_mismatch` で送らない（AC-14）。
2.  **画面。** `herdr agent read` で画面を読み、`claudeScreenAgrees` で確かめる。合わなければ `screen_mismatch`（新規、4.1）で送らない（AC-15）。行は「要確認」にしない。

```go
// claudeScreenAgrees reports whether a Claude Code screen shows the
// single-select question q read from the transcript, ready for an option
// key; why says what differed when it does not.
func claudeScreenAgrees(raw string, q Question) (ok bool, why string)
```

`claudeScreenAgrees` の契約は次のとおりである（申し送りの H-03）。

-   画面を今の `parseClaudeScreen` で読み、`QuestionSingle` かつ `RelayAllowed` で、タブのない問いのときだけ比べる。それ以外の形は合わない。複数を選ぶ問い、タブのある問い、権限の確認、入力欄、知らない形がこれに当たる。
-   問いの文：画面の文と transcript の文から、空白と改行をすべて除いて比べる。画面は文を pane の幅で折り返し、日本語は空白のない位置でも折り返すので、空白をそろえるだけでは等しくならない。今のマージの確認の見分け（`mergeQuestionSqueezed`、`internal/chief/screen.go:117`）と同じ考え方である。
-   選択肢：画面の選択肢の数が、transcript の選択肢の数に 2 を足した数であること。番号 1〜`n` の `label` が、transcript の同じ番号の `label` と、空白と改行を除いて等しいこと。番号 `n+1` が自由に書く答え、番号 `n+2` がチャットへ移る答えであること。説明（`description`）は比べない。画面は説明を折り返して描き、AC-15 も求めていないためである。
-   選択の位置：番号 1〜`n` のどれかにあること。自由に書く答え（`n+1`）とチャット（`n+2`）にあれば合わない（AC-15）。自由に書く答えに文字が打たれた画面では、その選択肢の名前が `Type something.` でなくなり、今の読み取りが 1 つを選ぶ問いとして読まないので、合わない側に倒れる（`internal/chief/screen_claude.go:328`・`:344`）。
-   このとき数字のキーが文字として入るかは確かめない。上の規則で、そのような画面には送らないからである。

この関数は画面の中身を調べるので、`screen_claude.go` に置き、CLAUDE.md の例外の範囲に入れる。

確かめの後は、今と同じく、送信の記録を書き、キーを送り、エージェントが質問から離れるのを待つ（`internal/chief/relay.go:250-281`）。中継のほかの守り（AC-18）は変えない。

### 3.6. 送る前の確かめ（AC-24・AC-25）

-   **コマンドの送信（`chief send`）。** 今も、workspace の排他を取った後に `readWorkspace` と `classify` で行を決め、その行が提案していないコマンドは送らない（`internal/chief/request.go:221-226`、`offered`）。3.4.2 で `classify` が待ち状態を使うので、Claude Code のどれかの pane に答えを待っている質問があるか、`blocked` か、「要確認」か、transcript を読めなければ、行は「質問で停止」か「要確認」になり、送らない。今の `offered` が送らないときに示すのは行の状態だけである（`internal/chief/request.go:317`）。そこで、行が「要確認」のときは `Unreadable` の理由も添える（AC-24 の「理由を示す」）。
-   **自動送信。** 今は、排他を取った後に `readWorkspace` を読み直し（`internal/chief/autosend.go:660`）、`quietness`（`internal/chief/autosend.go:376`）で確かめる。`quietness` は、Claude Code の pane ごとに、待ち状態が「なし」でなければ送らない理由を返す。理由の文は、質問か権限の確認なら「質問で止まっている」、要確認なら `Why` である。今の確かめ（`working`、`blocked`、画面が質問の形、画面が入力欄の形でない）はそのまま残す（AC-25）。
-   **workspace の削除。** 削除はまだ実装されていない。[0092 設計書](../0092_chief_task_workspace/02_architecture.md) の 3.4.2 の `agent_active` の確かめは、「0088 の F-001・F-002 の見分け」でエージェントが動いていないことを確かめる。この見分けは本書の後、Claude Code の pane では待ち状態になる。0092 の実装が本書より後なら、`quietness` と同じ待ち状態の確かめを使う。先なら、本書の実装で `agent_active` の確かめに待ち状態を足す。どちらが先かは計画で確かめる。
-   **送り先の pane の確かめ。** 送り先の pane の画面が入力欄の形で、入力欄が空であることを求める今の確かめ（`checkPane`、`internal/chief/send.go:331-342`）は変えない（AC-25）。
-   **リセット。** `/runpr` の後のリセット（0088 の F-009、`internal/chief/reset.go`）は AC-24 の対象ではないので変えない。リセットする pane は、今の `checkPane` で入力欄の形を確かめてから送る。

### 3.7. 待ち受け

`chief wait`（`internal/chief/wait.go`）は、Herdr のエージェントの一覧の変化で戻る。Claude Code が質問を出すと、Herdr の状態が `working` から `blocked` に変わるので、transcript を見なくても待ち受けは戻る。変えない。

### 3.8. 文書の更新（F-006）

-   **CLAUDE.md の「Declare, don't infer」の chief の例外（`CLAUDE.md:488`）。** 範囲の文を、次の振る舞いに合わせる（AC-27）。Claude Code の pane について、エージェントが質問か権限の確認を待っているか、問いの形と文と選択肢、サブエージェントが待っていることは、transcript の答えを待っている呼び出しから読む。`/runpr` のマージの確認は transcript の問いの文で見分ける。中継の直前に、画面の 1 つを選ぶ問いが transcript の問いと同じかを確かめる。画面からの判断は送るのを止めるためにだけ使う。「screen readers」が transcript を読むことは、今の文がすでに含む（モデルの切り替えと人の最後の入力）。新しい読み取りも `screen` で始まるファイルに置くので、ファイルの範囲は変えない。
-   **CLAUDE.md の「Go Idioms」の chief の例外（`CLAUDE.md:810`）。** マージの確認の文の正規表現は `screen.go` に 1 つだけ置き、transcript の読み取りもそれを使う（3.2.2）。そのため、置く場所は変わらない。例外の文の「pane の画面に描く決まった文字と同じでなければならない文字列」に、「transcript の問いの文と同じでなければならない文字列」を加える（AC-27）。
-   **chief 利用ガイド（AC-28）。** 1 章の表のエージェントの行に、Claude Code の質問を transcript で判定することと、確かめた transcript の版を書く。5.1 の「人が選ぶまで送らない」の中継の説明を、3.5 の確かめ（transcript の同じ呼び出しと、画面の 1 つを選ぶ問い）に書き直す。8 章の「中継しない質問」に、Claude Code では複数の問いをすべて示すこと、選択肢が 10 以上の問い、サブエージェントの待ちを足す。8 章の「transcript の場所の規則が変わったとき」に、場所が決まらない pane は状況の表で「要確認」になることを足す。
-   **ADR-0046。** 要件書の PR で、本書の決定を含めて更新済みである。本書の「設計の際に決めたこと」には ADR に当たるものがないので、変えない。

### 3.9. 既存のテストと既存の読み手への影響

本書の振る舞いの変更で、次の既存のテストは直す必要がある。どれも、Claude Code の pane の質問を画面のサンプルだけで作っていて、transcript がない。直した後は、transcript のサンプルで同じ状態を作る。

| テスト | 今確かめていること | 本書の後 |
|---|---|---|
| `TestTakeSnapshotStates`（`internal/chief/snapshot_test.go:51`）の `question`・`merge question`・`blocked on an unknown shape`・`detached HEAD with a question` | 画面の質問で「質問で停止」、知らない形の画面で「要確認」 | transcript の答えを待っている呼び出しで決まる。`blocked on an unknown shape` は、答えを待っている呼び出しのない `blocked` の「要確認」（AC-12）になる |
| `TestTakeSnapshotOrder`・`TestRowOrderRules`（`internal/chief/snapshot_test.go:336`・`:467`） | 質問の画面の workspace の並び | 同じ状態を transcript で作る |
| `TestWaitKeepsWaitingOnBlocked`（`internal/chief/wait_test.go:161`） | `blocked` のままなら待ち続ける | 待ち受けは変えないが、周の中で読む transcript を足す |
| `TestCheckValueChanges`・`TestNoCheckValueForUnrelayable`・`TestRelaySendsChosenOption`・`TestRelayRereadsTheScreenBeforeTheKey`・`TestRelayCheckValueMismatch`・`TestRelayRereadsBeforeTheKey`・`TestRelayRefusesTabbedQuestion`・`TestRelayNeedsAnAgentOnTheQuestion`・`TestRelayTakesTheSnapshotsValue`（`internal/chief/relay_test.go:63`〜`:423`）の Claude Code の場合 | 画面から作る照合値と、送る直前の画面の読み直し | Claude Code の場合は transcript の照合値と 3.5.3 の確かめになる。OpenCode の場合は今のまま |
| `TestSendChecksBeforeFirstInput`（`internal/chief/send_test.go:146`）と、`blocked` の Claude Code の pane を含む自動送信のテスト（`internal/chief/autosend_test.go:714`・`:835`） | 画面と Herdr の状態で送らない | 待ち状態でも送らないことを足す |

-   テストの世界（`internal/chief/world_test.go:63` の `fakeWorkspace`、`internal/chief/sim_test.go:107-128` の transcript の書き込み）に、Claude Code の pane の transcript を足す。
-   画面のサンプルの一覧の確かめ（`TestScreenSamplesCoverEveryRow`、`internal/chief/samples_test.go:132`）は変えない。H-07 で画面の読み取りを残すためである。
-   `/chief` の定義（`.claude/commands/chief.md`）は、出力の読み手である。3.4.3 のとおり、同じ PR で直す。

## 4. エラーハンドリング設計 (Error Handling Design)

### 4.1. 誤りと理由の型

```go
// errTranscriptLimit reports a transcript larger than the read cap; it is
// wrapped in errTranscript.
var errTranscriptLimit = errors.New("the transcript exceeds the read cap")

// errTranscriptForm is defined in 3.2.1.

// Appended to the NotSentReason constants, after NotSentConditions.

	// NotSentScreenMismatch is a relay whose pane screen does not show the
	// transcript's question as a single-select question ready for a key;
	// spelled "screen_mismatch".
	NotSentScreenMismatch
```

-   どちらの誤りも `errTranscript` に包むので、今の `errors.Is(err, errTranscript)` の確かめは変わらない。テストは `errors.Is` で見分ける（CLAUDE.md の「Testing Strategy」）。
-   `NotSentScreenMismatch` の綴りは `screen_mismatch` とする。`NotSentReason` の並び（`internal/chief/send.go:85-148`）の末尾に足す。

### 4.2. 示し方

行の `Unreadable` と、送らない結果の `detail` は、今と同じく英語の文である。`/chief` のセッションがそのまま示す。

| 場面 | 示す文の形 |
|---|---|
| transcript の場所が決まらない（AC-19） | `transcriptFailure` の文。作業ディレクトリの文字が理由なら、人ができることを含む（`internal/chief/transcript.go:73-79`） |
| 上限を超える（AC-22） | `the transcript of pane <P> exceeds the 64 MiB read cap` |
| 受け付ける形に合わない行（AC-21） | `a line of the transcript of pane <P> is not of the accepted form` |
| サブエージェント（AC-11） | `pane <P> waits on a subagent, which may be stopped on a question or a permission prompt; check the pane` |
| 何を待っているか分からない（AC-12） | `pane <P> is blocked, but its transcript shows no single call waiting for an answer; check the pane` |
| 状態の食い違い（AC-07） | `pane <P> has a question waiting in its transcript, but Herdr reports it <status>; check the pane` |
| 質問とほかの呼び出し（AC-09） | `pane <P> has a question and other calls waiting in its transcript; check the pane` |
| 画面が合わない（AC-15） | `screen_mismatch` と、`claudeScreenAgrees` の `why`（どこが違ったか） |

文の細部は計画と実装で決める。

## 5. セキュリティ考慮事項 (Security Considerations)

### 5.1. 製品

N/A。変えるのは開発用の道具 `chief` だけで、製品の外部コマンドの起動、API キー、キャッシュ、LLM API への送信、プロンプトや記事への信頼できないテキストの埋め込みには触れない（要件書 4.2）。

### 5.2. 開発の道具の信頼の境界

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef problem fill:#ffe6e6,stroke:#d62728,stroke-width:2px,color:#7b0000;

    LLM["タスクのエージェントの LLM<br>(問いの文・選択肢・道具の入力を書く)"]
    TR[("transcript")]
    SC[("pane の画面")]
    RD["読み取りと判定<br>(4.2 の形、待ち状態)"]
    SESS["/chief のセッション"]
    HUMAN["人"]
    KEY["中継のキー<br>(選択肢の番号だけ)"]

    LLM -->|"書く"| TR
    LLM -->|"描かせる"| SC
    TR --> RD
    SC -->|"止めるためだけ"| RD
    RD -->|"Sanitize した中身と照合値"| SESS
    SESS -->|"示す"| HUMAN
    HUMAN -->|"選ぶ"| SESS
    SESS -->|"照合値と番号"| RD
    RD -->|"確かめが通ったときだけ"| KEY

    class TR,SC data
    class LLM problem
    class SESS,HUMAN process
    class RD,KEY enhanced

    subgraph Legend["Legend"]
        L1[("信頼できない外部の値")]
        L2["信頼できない書き手"]
        L3["今の部品と人"]
        L4["本書で変える部品"]
    end
    class L1 data
    class L2 problem
    class L3 process
    class L4 enhanced
```

矢印 A → B は、A の値か操作が B に届くことを表す。

| 脅威 | 守り |
|---|---|
| 問いの文や選択肢、道具の入力に、セッションへの指示や制御文字を入れる | 示す文字列はすべて `Sanitize` を通す（AC-01・AC-10）。`/chief` の定義は、pane の文字列をデータとして扱う（`.claude/commands/chief.md` の規則 2）。権限の確認の入力は 2,000 文字で切る |
| 人が選んでいない入力を送らせる | 送る入力は、人が選んだ選択肢の番号から決まるキー入力だけで、選択肢の文は送らない（AC-16）。照合値はリクエストの値と比べるだけで、セッションは作れない |
| transcript の行に別の行を紛れ込ませる | JSON の文字列の中の改行はエスケープされるので、問いや入力の中身で行は増えない。行の形が合わなければ transcript 全体を読めないものとする（4.2 の拒む入力） |
| 問いの意味を変えるメンバーを足し、1 つを選ぶ問いに見せかける | 要件書 4.2 の残る危険のとおり。中継の直前の画面の確かめ（3.5.3）が、1 つを選ぶ問いの形でない画面への送信を止める |
| 選択肢の名前を `Type something.` などにして画面の確かめを惑わす | 画面の確かめは、番号の位置で自由に書く答えとチャットを確かめる。名前の同じ選択肢が途中にあると、今の画面の読み取りが 1 つを選ぶ問いとして読まず、合わない側に倒れる（`internal/chief/screen_claude.go:328`・`:344`） |
| 質問とマージの確認を取り違えさせる | マージの確認の見分けは、問いの文の形だけを今と同じ規則で使う（AC-08）。見分けた結果は、項目の種類を変えるだけで、送るものは増えない |
| サブエージェントや並んだ呼び出しを権限の確認に見せかける | 呼び出しが 1 つでないとき、サブエージェントのとき、transcript と Herdr が食い違うときは「要確認」にする（AC-09・AC-11・AC-12）。権限の確認は中継しないので、見分けを誤っても送るものは増えない |
| 同じ利用者のほかのプロセスが transcript を書き換える | 0088 設計書 5.2 の残る危険のとおり、守りの範囲の外である。その場合も、中継は画面の確かめと Herdr の `blocked` が通らなければ送らない |
| 読み取りの時間で待ち受けを止める | 読む量は 64 MiB で切る（AC-22）。上限の transcript でも 0.42 秒以内だった（1.2） |

## 6. 処理フロー詳細 (Processing Flow Details)

### 6.1. 待ち状態の判定

```mermaid
flowchart TD
    S(["Claude Code の pane"]) --> R{"transcript を<br>読めるか"}
    R -->|"読めない"| NC1["要確認<br>(AC-19・AC-21〜AC-23)"]
    R -->|"読めた・ファイルがない"| M{"答えを待っている呼び出しが<br>2 つ以上で、質問を含むか"}
    M -->|"はい"| NC2["要確認 (AC-09)"]
    M -->|"いいえ"| B{"Herdr の状態"}
    B -->|"blocked"| BB{"答えを待っている<br>呼び出し"}
    BB -->|"質問 1 つ"| Q1["質問 (AC-01)"]
    BB -->|"サブエージェント 1 つ"| NC3["要確認 (AC-11)"]
    BB -->|"そのほか 1 つ"| P1["権限の確認 (AC-10)"]
    BB -->|"ない・2 つ以上"| NC4["要確認 (AC-12・AC-20)"]
    B -->|"working"| W{"質問 1 つだけか"}
    W -->|"はい"| Q2["質問・動いている<br>(AC-06)"]
    W -->|"いいえ"| N1["なし (AC-13)"]
    B -->|"それ以外"| I{"答えを待っている<br>質問があるか"}
    I -->|"ある"| NC5["要確認 (AC-07)"]
    I -->|"ない"| N2["なし (AC-13)"]
```

矢印は判定の順を表す。どの分かれ道も、当たらないものは推し量らず「要確認」か「なし」になり、送る側には倒れない。

### 6.2. 答えの中継（Claude Code の pane）

```mermaid
sequenceDiagram
    participant S as /chief のセッション
    participant R as chief relay
    participant H as Herdr
    participant T as transcript

    S->>R: --check 照合値 --answer 1=n
    R->>R: workspace の排他・送信の停止の確かめ（今のまま）
    loop workspace の各 pane
        R->>H: herdr agent get
        R->>T: transcript を読み、待ち状態を決める
        R->>R: transcriptCheckValue と照合値を比べる
    end
    alt 一致する pane がない
        R-->>S: no_transcript・unreadable・not_accepting・check_mismatch
    else 一致した
        R->>R: 中継する問いか・blocked か・選択肢 n があるか（今のまま）
        R->>H: herdr agent get（読み直し）
        R->>T: transcript を読み直す
        alt 照合値が変わった
            R-->>S: check_mismatch（AC-14）
        else 同じ
            R->>H: herdr agent read（画面）
            alt 画面が合わない
                R-->>S: screen_mismatch（AC-15）
            else 合う
                R->>R: 送信の記録を書く
                R->>H: 番号のキー（AC-16）
                R->>H: 質問から離れたかを待つ（今のまま）
                R-->>S: sent
            end
        end
    end
```

矢印は呼び出しと応答を表す。`--answer` の形の確かめ（`ParseRelayRequest`、`internal/chief/relay.go:95`）と、キーを送った後の扱い（AC-18）は今のまま。

## 7. テスト戦略 (Test Strategy)

### 7.1. サンプル（申し送りの H-05）

実装の最初の段で、人が採取用の pane（`chief-poc`）で状態を作る。エージェントは読み取りだけを使う（`herdr agent get`・`herdr agent read`、transcript のファイルの写し。`internal/chief/testdata/README.md` の「採り方」）。採るものは次のとおりである。

-   答えを待っている間の transcript と、そのときの `herdr agent get` の状態。1 つを選ぶ問い、複数を選ぶ問い、問いが 2 つの呼び出し、権限の確認、サブエージェントの待ちのそれぞれについて採る。権限の確認の間に Herdr が `blocked` と報告することを、ここで確かめる。成り立たなければ、要件書を `draft` に戻す（1.2）。
-   1 つを選ぶ問いの、同じ呼び出しの transcript と画面の組。画面の確かめ（3.5.3）を、実物の折り返しで確かめるためである。問いの文が pane の幅で折り返すものを含める。
-   採った値は、`testdata/README.md` の「置き換えたもの」のとおりに置き換える（[security.md](../../dev/security.md) §7）。

### 7.2. ユニットテスト

-   **読み取り（3.2）。** 7.1 のサンプルと、そこから 1 か所を変えた入力で確かめる。要件書 4.2 の拒む入力の各行、書きかけの最後の行（AC-21）、`api-request-blob` の行にある同じ呼び出しの ID（H-06）、対応する呼び出しのない答え、`is_error` の答え（AC-02）、上限を超えるファイル（AC-22）、ファイルがないとき（AC-20）を含める。誤りは `errors.Is` で見分ける。
-   **判定（3.3）。** `judgeClaudeWait` を、3.3 の表の各行について、表で前の行に当たらない入力で確かめる（CLAUDE.md の「A layered path needs inputs only one layer can handle」）。
-   **画面の確かめ（3.5.3）。** 7.1 の組と、既存の画面のサンプル（`claude-question-single-cursor.ansi` など）で確かめる。選択の位置が自由に書く答えかチャットにある画面、文字を打った画面、問いの文が違う画面、選択肢の数が違う画面、日本語の折り返しの画面を含める。
-   **表と出力（3.4）。** テストの世界に transcript を置き、AC-01〜AC-13、AC-19〜AC-23、AC-26 の各状態の行と出力を確かめる。画面が知らない形でも、Claude Code の pane の行が「要確認」にならないこと（AC-26）も確かめる。
-   **中継（3.5）。** テストの世界で、AC-14〜AC-18 と AC-23・AC-06 の理由を確かめる。照合値が一致した後、送る直前に transcript で答えが書かれる場合、Herdr の状態が変わる場合、画面が自由に書く答えに移る場合に、何も送らないことを含める。
-   **送る前の確かめ（3.6）。** コマンドの送信と自動送信が、AC-24 の (a)〜(d) のどれでも送らないことを確かめる。
-   CLAUDE.md の「Every test must be able to fail for its stated reason」のとおり、各テストは確かめる部分を壊して落ちることを確かめる。

### 7.3. 結合テストと文書のガード

`internal/chief` の今のガードのテスト（送り先の表、PR のマーカー）は変わらない。CLAUDE.md・利用ガイド・`/chief` の定義の文の変更は、文書のテストで確かめず、PR のレビューで読む（CLAUDE.md の「Document tests check values, structure, and executable examples, never prose」）。

### 7.4. 実地の確かめ（manual）

実装の最後に、採取用の workspace で、Claude Code の 1 つを選ぶ問いを `/chief` で示し、中継できることを確かめる。複数の問い、権限の確認、サブエージェントの待ちが中継されずに示されることも確かめる。

### 7.5. セキュリティのテスト

5.2 の表の守りのうち、テストで確かめられるものを 7.2 に含める。制御文字を含む問いの文と道具の入力が置き換えて示されること、2,000 文字を超える入力が切られること、`Type something.` を名前に持つ選択肢の画面が合わないこと、行の形の違いで transcript 全体が読めなくなることである。

## 8. 実装の優先順位 (Implementation Priorities)

1.  **サンプル（7.1）。** 権限の確認の前提を最初に確かめる。成り立たなければ、ここで止まる。
2.  **読み取りと判定（3.2・3.3）。** 型と、ほかに依らないユニットテスト。
3.  **表と出力（3.4）と、送る前の確かめ（3.6）。** テストの世界に transcript を足し、3.9 の既存のテストを直す。
4.  **中継（3.5）。** 照合値、送る直前の確かめ、画面の確かめ、新しい理由。
5.  **文書（3.8）と `/chief` の定義（3.4.3）。**
6.  **実地の確かめ（7.4）。**

PR の区切りと各段のステップは計画で決める。

## 9. 将来の拡張 (Future Extensibility)

-   **#908（画面の読み取りの整理）。** 本書の後、Claude Code の画面の読み取りのうち、複数を選ぶ問い、タブのある問いとその確認のページ、権限の確認は、送る前の確かめの理由の文にだけ使われる。消す範囲は #908 で決める（1.2）。
-   **#905（報告の読み取り）。** transcript の読み取りの規則（書きかけの行、受け付ける形）を 1 つにそろえるかは、#905 で最後の報告を transcript で読むときに考える（要件書の「要件定義の際に決めたこと」）。本書の読み取りは、行の読み方を `readPendingCalls` の中に閉じ、そろえるときに使えるようにしておく。
-   **OpenCode（#906・#907）。** OpenCode の質問を画面以外で読めるようになれば、`QuestionItem` の `CallID` と照合値の作り方を同じ形で使える。
-   **選択肢が 10 以上の中継。** 2 桁のキーを送る方法ができれば、`RelayBlockTooMany` を外す。送る入力の範囲を変えるので、ADR-0046 の変更になる。

## 付録 A: 申し送り（design_handoff.md）への対応

| 項目 | 対応 |
|---|---|
| H-01 | 毎回すべてを読む。64 MiB で 0.31〜0.42 秒を測った（1.2、3.2.3） |
| H-02 | 照合値は、呼び出しの ID、問いと選択肢の中身、pane、`agent_session`、`state_change_seq` から作る。中身を読めない値にする（1.2、3.5.1） |
| H-03 | 今の 1 つを選ぶ問いの読み取りで画面を読み、空白と改行を除いて問いの文と選択肢の `label` を比べる。自由に書く答えとチャットを番号の位置で確かめ、選択の位置がそこにあれば合わないとする。数字のキーが文字として入るかは、送らないので確かめない（3.5.3） |
| H-04 | サブエージェントの行は親の transcript に書かれないことを確かめた。道具の名前 `Agent` で見分ける（1.2、3.3） |
| H-05 | 答えた後の transcript で、間に書かれる行の種類と質問の入力の形を確かめた（1.2）。答えを待っている間の transcript と、権限の確認の間の Herdr の状態は、実装の最初の段で採る（7.1） |
| H-06 | `assistant` と `user` の行だけを読み、ほかの行は読まない。テストのサンプルに含める（3.2.1、7.2） |
| H-07 | 消さずに #908 に残す（1.2、9 章） |

## 付録 B: 受け入れ基準との対応

| AC | 本書 |
|---|---|
| AC-01 | 3.2.2、3.3、3.4.2、3.4.3 |
| AC-02 | 3.2.1（`is_error` を問わない答え）、3.3 |
| AC-03 | 3.2.2（`QuestionSeveral`）、3.4.3 |
| AC-04 | 3.2.2（`QuestionMulti`） |
| AC-05 | 3.2.2（`RelayBlockTooMany`）、3.5.1 |
| AC-06 | 3.3、3.4.2、3.4.3、3.5.2 |
| AC-07 | 3.3、4.2 |
| AC-08 | 3.2.2 |
| AC-09 | 3.3、4.2 |
| AC-10 | 3.3、3.4.2、3.4.3 |
| AC-11 | 3.3、4.2 |
| AC-12 | 3.3、4.2 |
| AC-13 | 3.3 |
| AC-14 | 3.5.2、3.5.3 |
| AC-15 | 3.5.3、4.1 |
| AC-16 | 3.5.3、5.2 |
| AC-17 | 3.5.1、3.5.2 |
| AC-18 | 3.5.3 |
| AC-19 | 3.2.3、3.3、4.2 |
| AC-20 | 3.2.3、3.3 |
| AC-21 | 3.2.1、4.2 |
| AC-22 | 3.2.3、4.2 |
| AC-23 | 3.2.3、3.3、3.5.2 |
| AC-24 | 3.6 |
| AC-25 | 3.6 |
| AC-26 | 3.4.2 |
| AC-27 | 3.8 |
| AC-28 | 3.8 |
