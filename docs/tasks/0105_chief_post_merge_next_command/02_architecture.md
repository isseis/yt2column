# 設計書：chief は `main` をチェックアウトした worktree をタスクのない workspace と読まず、マージを確かめた PR からタスクを特定する

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-19 |
| Review date | 2026-11-19 |
| Reviewer | isseis |
| Comments | - |

## 1. 設計の全体像

本書は [01_requirements.md](01_requirements.md)（以下、要件定義書）の F-001〜F-004 を実現する設計である。対象は chief（`internal/chief`・`cmd/chief`）、`/chief` の定義（`.claude/commands/chief.md`）、chief 利用ガイド（`docs/dev/guides/chief_guide.md`）である。既存のコードについての記述は、コミット `910bb12b`（`origin/main`）で確かめた。

語は要件定義書 3.1 の冒頭の定義に従う（「`main` をチェックアウトした worktree」「chief の pane のセッション」「chief がマージを確かめた PR」「PR のブランチ名から抽出したタスク」「候補のタスク T」「chief が確かめたほかのタスク」「chief が最後に開いていると確かめた PR」）。本書では次の語を足す。

-   **履歴の記録**：chief の pane のセッションが、worktree ごとに、PR とブランチについて確かめたことを 1 件ずつ残す記録（3.1）。1 件を**事象**と呼ぶ。要件定義書 4.2 と AC-09 の「特定に使う観測の記録」は、この記録を指す。自動送信が使う今の観測の記録（`Observation`、`internal/chief/records.go:1153-1162`）は変えず、特定には使わない。
-   **`main` の行**：`main` をチェックアウトした worktree の行。
-   **特定の結果**：`main` の行について、履歴の記録から決めた結果（3.2）。タスクを特定したか、特定しなかった理由の種類のどちらかである。
-   **H-NN**：設計への申し送り（[design_handoff.md](design_handoff.md)）の項目番号。
-   **項目 ID**：`snapshot`・`tick` の出力が、人の判断を待つ項目ごとに示す `id`（`ItemID`）。提案の項目 ID は、workspace・コマンドの種類・コマンドの文（タスク番号を含む）・計画書の PR から作る（`internal/chief/pending.go:162-164` の `proposalID`）。

### 1.1. 設計原則

-   **`main` を HEAD の種類として宣言する。** 今の HEAD の種類（`headState`、`internal/chief/state.go:222-233`）は、ブランチ・detached・読めないの 3 つである。これに「ブランチ `main`」（`headMain`）を足す。ブランチの名前とバイト列でちょうど `main` を比べるのは、行を作るための読み取り（`readWorkspace`、`internal/chief/snapshot.go:277-309`）の 1 か所だけにする。以降の判定は、HEAD の種類と特定の結果の型だけで分かれる（CLAUDE.md の「Declare, don't infer」、要件定義書 5 章）。自動送信の `gather` はブランチを自分で読み、`main` を今のとおり `headBranch` とする。自動送信の理由（AC-10）を変えないための、意図した使い分けである。
-   **タスクを決める源は 2 つに限る。** ブランチの名前（今の `branchTask`、`internal/chief/task.go:741-751`）と、履歴の記録から決めた特定の結果だけである。`main` の行にだけ後者を使い、ほかの HEAD の行は今のまま読む（AC-11、要件定義書 4.5）。
-   **分からなければ要確認にする。** 記録がない、食い違う、開いている PR がある、読めない、のどれでも、タスクを決めず、次のコマンドを示さず、`/mkreq` も示さない（AC-06・AC-07・AC-09、要件定義書 4.3）。
-   **記録は書き換えず、変わったときに追記する。** 履歴の記録は、今の記録と同じく、同名のものがあれば作らず、上書きもしない（ADR-0046 の「記録」）。セッション中の状態は、そのセッションの事象をすべて読み、順に当てはめて決める。
-   **自動送信は変えない。** 自動送信の条件、窓（15 分）、引き継ぎの期限（10 分）、観測の記録とその書き方、作成の排他の持ち方は変えない（AC-10）。
-   **人が選ぶ送信は、人が選んだ項目で縛る。** `chief send` は人が選んだ項目の項目 ID を受け取り、送る直前に作り直した行が同じ項目を提案しているときだけ送る（AC-05）。

### 1.2. 設計の際に決めたこと（2026-11-19）

-   **人が選ぶ提案の送信は、`chief send` に `--item <項目 ID>` を足して縛る。タスクのコマンド（`mkarch`・`mkplan`・`runplan`・`runplan-takeover`）では必須とし、`mkreq`・`runpr` では受け付けない。** 道具は、workspace の排他を持って行を作り直し、その行の提案の項目 ID が `--item` と等しいときだけ送る（3.5）。この確かめは `main` の行だけでなく、機能ブランチの行にも当てる。
    -   今の `chief send` は、送る直前に作り直した行のコマンドの種類だけを比べる（`internal/chief/request.go:386-412` の `offered`）。そのため、人が見た後に行のタスクが変わっても、同じ種類のコマンドなら別のタスクへ送ってしまう。
    -   項目 ID はコマンドの文を含むので、タスクが変われば変わる。自動送信の提案（`applyAutoSends`、`internal/chief/tick.go:132`）が行とは別のタスクを示していても、人が選んだ項目のタスクと違うものは送らない。
    -   はじめは、行のタスクを `--task` で渡す案を選んだ。しかし本書のレビューで、自動送信の提案が行と別のタスクを示すとき、人が見ていない行のタスクへ送りうると分かった。人の答え：`--item` にする（`--task` から見直した）。
    -   提案のタスクを出力に足して `--task` に使う案は採らなかった。出力の項目を変えない（要件定義書 4.4）ためである。`relay`・`reset` の照合値（`CheckValue`）を新しく足す案も採らなかった。項目 ID がすでに同じ役を果たすためである。
    -   道具の口の形が変わるので、プロトコル番号を `main` の値 + 1 にし、`/chief` の定義の呼び出しを直す（H-04）。
    -   ADR：[ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新する（「人の判断の中継」に、人が選ぶ送信を人が選んだ項目で縛ることを足す）。送る入力の確かめの決定を変え、道具の口とプロトコル番号にかかわるので、主要な決定に当たる。
-   **履歴では開いているが、そのラウンドでは worktree のブランチで開いているのを見なかった PR は、自動送信の窓の外でも、今の `gh pr view`（`PRStateOf`、`internal/chief/workspace.go:390`）で状態を確かめる（3.3）。** 同じラウンドで自動送信が同じ PR を読むときは、その結果を使い、PR ごとに 1 回だけ読む。
    -   要件定義書 3.1 の「後のどの `tick` でも `MERGED` であることを確かめれば」を満たすには、窓の外でも状態を読む必要がある。これは、要件定義書 4.1 の「新しい呼び出しを足さない」と合わない。そこで、本 PR で要件定義書 4.1 を決定の変更として改めた（要件定義書「設計の際に見直したこと（2026-11-19）」）。
    -   マージを確かめるのを自動送信の窓の中だけにする案は採らなかった。15 分以上あくと、セッションが続いてもタスクを特定できないためである。人の答え：要件定義書 4.1 を直す。
    -   ADR：ADR-0046 を更新する（「ブランチ `main` をチェックアウトした worktree のタスク」に、マージの確かめ方を足す）。
-   **履歴の記録は、worktree ごとの記録のディレクトリに、変化があったときだけ 1 件ずつ追記する事象の記録とする（3.1）。** 名前にセッションのハッシュを含め、今のセッションのものだけを読む。ほかのセッションの事象は `start` が消す。worktree ごとのディレクトリに置くので、同じパスに worktree を作り直す前と workspace を消し終えた後には、今の `ClearRecords`（`internal/chief/records.go:1362-1382`）が消す。
    -   毎ラウンド履歴の全体を新しい記録へ引き継ぎ、古いラウンドの分を消す案は採らなかった。書けなかったことと空の履歴を見分けられず、作り直したパスの記録を消す規則も別に要るためである。人の答え：変化の事象を追記する。
    -   今の観測の記録と、PR が開いていることを二重に記録することになる。それでも観測の記録は変えない。観測の記録はラウンドごとに消え、自動送信の窓と引き継ぎに合わせた形で、自動送信を変えないためである。
    -   ADR：ADR-0046 を更新する（「記録」の記録の種類と、「ブランチ `main` をチェックアウトした worktree のタスク」に、履歴の記録の形と消し方を足す）。記録の種類と消し方の決定を変えるので、主要な決定に当たる。
-   **履歴の事象は、その worktree の排他（`HeldWorkspace`）を持って書く。書く前に、排他の下でその worktree のブランチと履歴の記録を読み直し、ラウンドで読んだブランチと同じで、マージや閉じたことを書く PR の `pr_open` が残っているときだけ書く（3.3）。** `create` と `remove` は、同じ排他を持って記録を消す（`internal/chief/create.go:500`、`internal/chief/removal.go:873`）。排他の下で読み直せば、消された後のディレクトリに古い worktree の事象を書くことはない。排他を取れなければ（送信の最中）、そのラウンドはその worktree の事象を書かず、ラウンドの問題に示す。
    -   作成の排他（`LockCreation`）で書く案は採らなかった。`remove` はその排他を取らず、ラウンドの読み取りは排他の前に行うので、消した直後に古い事象を書きうるためである。
    -   ADR：なし。記録の書き方の細部で、このタスクの中で閉じる。
-   **マージを書くラウンドには、そのときの機能ブランチも書き、同じラウンドの事象の順は安全な側に決める。** worktree の PR のマージか閉じたことを書くラウンドでは、worktree が機能ブランチにいれば、その `branch_task` を、最後の `branch_task` と同じでも書く（3.1）。同じラウンドに、T の PR のマージの事象と、T でない `branch_task` があれば、`branch_task` はマージの後とみなす（3.2 の順 4）。マージの前から T2 のブランチにいた worktree は、変化だけを書く規則ではマージの後に事象が残らず、その後 `main` に戻ると T を特定してしまうためである。ファイルの名前の順（事象のハッシュの順）で決めないのは、順に意味がなく、特定する側に倒れることがあるためである。
    -   ADR：なし。このタスクの中で閉じる。
-   **機能ブランチでも `main` でもないブランチで開いていた PR も、タスクを空にして記録する。タスクが空の PR は「ほかのタスク」とみなし、その worktree は特定しない（3.1・3.2）。** タスクの分からない仕事に使い回した worktree が `main` に戻ったとき、前のタスクを特定しないためである（H-07）。本 PR で要件定義書 3.1 の「chief が確かめたほかのタスク」に足した。人の答え：空のタスクで記録し要確認にする。
    -   ADR：ADR-0046 の同じ更新に含める。
-   **事象を書けなかったときに、古いタスクを示しうることを、残る危険として受け入れる（5 章）。** 書けなかったことはラウンドの問題に示し、送るには人が項目を選ぶ必要がある。書けなかったことを示す印も書き込みなので、完全には守れないためである。人の答え：受け入れる。
    -   ADR：ADR-0046 の「結果」に書く。
-   **`tick` は、そのラウンドの履歴を書いてから行を決める（3.4）。** 今の `readRound`（`internal/chief/snapshot.go:90-149`）を、workspace の読み取りと行の判定の 2 つに分け、`tick` はその間で履歴を書く。こうすると、マージを確かめたラウンドの行に、もう特定したタスクが示される。分けずに次のラウンドから示す案は採らなかった。`chief wait` は変化があるまで次のラウンドを始めないので、要確認の行が長く残りうるためである。
    -   ADR：なし。`internal/chief` の中の処理の順で、このタスクの中で閉じる。
-   **`snapshot`・`chief send`・`chief reset` は、履歴の記録を読むだけで書かない。`chief targets` は読まない（3.4）。** `snapshot` の「記録を作らない」約束（`internal/chief/snapshot.go:74-78`）を保つためである。今のセッションで `tick` が一度も走っていなければ、事象がないので要確認になる（AC-07）。
    -   ADR：なし。ADR-0046 の「結果」がすでに書いている振る舞いの実現である。
-   **`main` の worktree は、新しいタスクを始める先（`chief targets`、`internal/chief/create.go:856`）に入らない。** 今、`targets` は `/mkreq` を提案する workspace を挙げ、その中には `main` の worktree も入る。本タスクで `main` の行は `/mkreq` を提案しなくなり、`/mkreq` の送信も拒む（AC-08）。そのため、`targets` もこれに合わせる。
    -   ADR：なし。AC-08 の帰結である。
-   **特定に使う読み取りの形の求め（要件定義書 4.2）は、履歴の記録にだけ当てる。今の観測の記録の読み取りは変えない（H-06 の 1 つ目）。** 観測の記録は自動送信の照合にしか使わず、自動送信は変えないためである（AC-10）。
    -   ADR：なし。読み取りの細部で、このタスクの中で閉じる。
-   **特定したタスクの文書は、最後にマージを確かめたそのタスクの PR のブランチ名を、今の `Tasks.Read`（`internal/chief/task.go:795-822`）に渡して読む（H-05）。** 機能ブランチの行と同じく、ディレクトリの名前がブランチの slug と合うことまで確かめられる。番号だけで読む `ReadDir` の規則を当てる案は採らなかった。slug の照合を外すことになるためである。
    -   ADR：なし。読み取りの細部で、このタスクの中で閉じる。
-   **`main` の行の git の状態（`origin/main` より先のコミット、未コミットの変更）は、人が選ぶ送信の条件に足さない（H-02 の 2 つ目）。** 機能ブランチの行の人が選ぶ送信も、git の状態を見ない（`internal/chief/request.go:194-254` の `Send`）。送ったコマンドは、自分の手順（`_lib/task-pr-handoff.md` の Part A）で、作業ツリーが汚れていれば止まり、新しいブランチを `origin/main` から作る。
    -   ADR：なし。次のコマンドの規則と送る条件を変えないためである。
-   **AC-07 の行には、使っていないか残った worktree の印（`Row.Idle`）を付ける。AC-06・AC-09 の行には付けない（H-03）。** 食い違いと読み取りの失敗は、人が見るべきものなので、表の最後に回さないためである。AC-07 の行でも、fetch に失敗したときは付けない。今の detached HEAD の行と同じ扱いである（`internal/chief/state.go:464`、H-06 の 4 つ目）。
    -   ADR：なし。ADR-0046 の「結果」がすでに書いている並べ方の実現である。
-   **`main` の行の特定と `--item` の照合は、同じ PR で入れる（8 章）。** 特定だけが先に入ると、特定した `main` の行へ、種類だけの照合で送れてしまうためである。
    -   ADR：なし。実装の順の制約で、このタスクの中で閉じる。

## 2. システム構成

### 2.1. 概念モデル

`main` の行のタスクは、履歴の記録から決まる。図の矢印 A → B は「A から B を決める」を表す。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    HEAD[("worktree の HEAD")]
    BR["branchTask"]
    TRACE[("履歴の記録")]
    JUDGE["mainTaskResult"]
    TASK[("行のタスク")]
    DOCS[("origin/main の文書")]
    ROW["classify"]

    HEAD -->|"機能ブランチ"| BR
    HEAD -->|"ブランチ main"| JUDGE
    TRACE --> JUDGE
    BR --> TASK
    JUDGE -->|"特定した"| TASK
    TASK --> ROW
    DOCS --> ROW
    JUDGE -->|"特定しなかった"| ROW

    class HEAD,TRACE,DOCS,TASK data
    class BR process
    class JUDGE,ROW enhanced

    subgraph Legend["Legend"]
        L1[("データ")]
        L2["今の部品（変えない）"]
        L3["新しい部品・変える部品"]
    end
    class L1 data
    class L2 process
    class L3 enhanced
```

detached HEAD と、`main` でも機能ブランチでもないブランチの行は、`mainTaskResult` を通らない。今のとおりである（AC-11）。

### 2.2. コンポーネント配置

図の矢印 A → B は「A が B を呼ぶか読む」を表す。緑は本タスクで足すか変える部品である。

```mermaid
flowchart TB
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    MAIN["cmd/chief"]
    TICK["Tick"]
    SNAP["TakeSnapshot"]
    SEND["Send"]
    RESET["SendOfferedReset"]
    TARGETS["Targets"]
    READ["readFacts・buildRows"]
    APPLY["applyMainTask"]
    TRACEPKG["履歴の記録の読み書き<br>（trace.go）"]
    CLASSIFY["classify・documentOf"]
    AUTO["autoSender"]
    STORE[("状態のディレクトリ")]
    GH[("GitHub")]

    MAIN --> TICK
    MAIN --> SNAP
    MAIN --> SEND
    MAIN --> RESET
    MAIN --> TARGETS
    TICK --> READ
    SNAP --> READ
    TARGETS --> READ
    READ --> APPLY
    SEND --> APPLY
    RESET --> APPLY
    APPLY --> TRACEPKG
    APPLY --> CLASSIFY
    TICK --> TRACEPKG
    TICK --> GH
    TICK --> AUTO
    AUTO --> GH
    TRACEPKG --> STORE

    class MAIN,TICK,SNAP,SEND,RESET,TARGETS,READ,APPLY,TRACEPKG,CLASSIFY,AUTO enhanced
    class STORE,GH data

    subgraph Legend["Legend"]
        L1[("データ・外部サービス")]
        L3["新しい部品・変える部品"]
    end
    class L1 data
    class L3 enhanced
```

`autoSender` は、PR の状態の読み取りにラウンドの中の表を使うことだけが変わり、条件と結果は変えない。

### 2.3. データの流れ（`tick` の 1 ラウンド）

図の矢印は呼び出しと応答を表す。今のラウンドとの違いは、読み取りと行の判定の間で、履歴を書くことである。作成の排他は、今のとおり自動送信の間だけ持つ。

```mermaid
sequenceDiagram
    participant T as Tick
    participant R as readFacts
    participant S as Store
    participant H as 履歴の記録
    participant G as GitHub
    participant B as buildRows
    participant A as autoSender

    T->>R: workspace を読む（Herdr・git・文書）
    R-->>T: workspace ごとの読み取り
    T->>S: BeginRound
    T->>S: 観測の記録（今のまま）
    T->>G: 履歴で開いていて、今回開いているのを見なかった PR の状態
    G-->>T: MERGED / CLOSED / OPEN
    T->>H: worktree ごとに、排他の下でブランチを読み直して変化の事象を書く
    T->>B: 読み取りと履歴から行を決める
    T->>S: LockCreation
    T->>A: 自動送信（今のまま。PR の状態はラウンドの中の表から）
    T->>S: 作成の排他を放す
```

自動送信は、行を決めた後に走る。今も自動送信は行を読まず、workspace の読み取り（`roundView.found`・`facts`）だけを読む（`internal/chief/autosend.go:350-394` の `gather`）。そのため、順を入れ替えても自動送信の判断は変わらない。行の判定が自動送信の結果を受けるのは、今の `applyAutoSends`（`internal/chief/tick.go:99-137`）のとおり、自動送信の後である。観測の記録は、今と同じく作成の排他を取る前に書く（`internal/chief/tick.go:49-60`）。

## 3. コンポーネント設計

### 3.1. 履歴の記録

worktree ごとの記録のディレクトリ（`workspaceDirOf`、`internal/chief/records.go:551-`）に置く。名前は `trace-<セッションのハッシュ>-<ラウンドの ID>-<事象のハッシュ>.json` である。ラウンドの ID は今のラウンドの記録と同じもの（時刻順に並ぶ）で、事象のハッシュは事象の種類・PR の番号・タスクから作る。同じラウンドで同じ事象は 1 件しか作らない。

```go
// traceKind is the kind of one event of a worktree's history. The zero value
// is traceUnknown, which no reader accepts.
type traceKind int

const (
	traceUnknown traceKind = iota
	// tracePROpen: the round saw the PR open on the worktree's branch, which
	// is not main. Branch is that branch; Task is its task, or zero when the
	// branch is no feature branch or its task directory on origin/main
	// contradicts it.
	tracePROpen
	// tracePRMerged: the round read the PR's state as MERGED. PR only.
	tracePRMerged
	// tracePRClosed: the round read the PR's state as CLOSED. PR only.
	tracePRClosed
	// traceBranchTask: the round saw the worktree on a feature branch of
	// Task. Task only.
	traceBranchTask
)

// traceEvent is one record of a worktree's history in one chief session.
type traceEvent struct {
	Session  string
	Round    string
	Worktree string
	Kind     traceKind
	PR       int
	Task     paneinput.TaskID
	Branch   string
}
```

JSON の項目は `session`・`round`・`worktree`・`kind`（`pr_open`・`pr_merged`・`pr_closed`・`branch_task`）・`pr`・`task`・`branch` とする。種類ごとに使う項目は上の注釈のとおりで、使わない項目は空か 0 でなければならない。

**読み取り（要件定義書 4.2、H-06 の 2 つ目）。**

-   1 件の大きさは `recordLimit`（512 KiB、`internal/chief/records.go:103`）までとする。
-   `internal/strictjson` で読み、不正な UTF-8・対のないサロゲート・項目の繰り返し・知らない項目を拒む。
-   `task` は、空でなければ `paneinput.ParseTaskID`（4 桁の数字だけ、`internal/chief/paneinput/paneinput.go:37-47`）で読み、前後の空白やほかの文字を直さずに拒む。`pr_open` では、`task` は `branch` から `branchTask` で抽出したものと等しいか、空でなければならない。
-   `kind` が 4 つのどれでもないもの、種類に要る項目がないか使わない項目に値があるもの、`session`・`round`・`worktree` が名前やディレクトリと合わないものも拒む。
-   同じ worktree に、`pr_open` のない PR の `pr_merged`・`pr_closed` があれば、その記録を読めない記録とする。開いていると確かめていない PR のマージは、要件定義書 3.1 の「chief がマージを確かめた PR」に当たらないためである。
-   拒んだ記録は `errRecord`（`internal/chief/records.go:32`）で包み、`errors.Is` で判定できる。

**名前とディレクトリ。** 読む側は、ディレクトリを作らずに読む（今の `existingRecords`、`internal/chief/records.go:621`）。次のものは読めない記録として扱う。

-   `trace-` で始まり上の形に合わない名前。どのセッションのものか分からないので、今のセッションのものとみなす。`start` は、ほかのセッションの事象と同じくこれを消し、`isRecordFile` も `trace-` で始まる名前をすべて履歴の記録として扱う（`ClearRecords` が消す）。
-   一覧を読めない worktree ごとのディレクトリ。セッションを変えても戻らないので、理由にそのディレクトリを示し、人が権限などを直す。

ほかのセッションのハッシュを持つ記録は読まない。

**書く条件。** 事象は、そのセッションの事象をすべて当てはめた状態（3.2）と、そのラウンドの読み取りが違うときだけ書く。

| 事象 | 書く条件 |
|---|---|
| `branch_task` | worktree が機能ブランチにいて、その worktree の最後の `branch_task` のタスクと違うか、`branch_task` がまだないか、そのラウンドにその worktree の PR の `pr_merged`・`pr_closed` を書く |
| `pr_open` | worktree が `main` でないブランチにいて、そのブランチに開いている PR が 1 つあり（`readWorkspace` の `f.PR`）、その PR の最後の状態が「開いている」ではない（事象がない、マージ、閉じた）。タスクは、ブランチから抽出でき、そのラウンドの `Tasks.Read` が `origin/main` のタスクのディレクトリと矛盾なく読めたとき（要件定義書 3.1）だけ書き、そうでなければ空にする（3.2 でほかのタスクとみなされる） |
| `pr_merged`・`pr_closed` | 状態が開いている PR を、そのラウンドの worktree のブランチで開いているのを見ず、その PR の状態を読んだら `MERGED`・`CLOSED` だった |

PR の状態を読めなかったときと、まだ `OPEN` だったときは何も書かない。その PR は開いているままになり、`main` の行は特定しない（AC-07）。

**消し方。** 今のセッションの事象は消さない。ほかのセッションの事象は、`start` の片付けで消す。`start` は今も worktree ごとのディレクトリを、その排他を持って片付けている。履歴の記録を `isRecordFile`（`internal/chief/records.go:1349-1353`）に足すので、`create` が同じパスに作り直す前と、`remove` が消し終えた後に、`ClearRecords` が消す。消し終えた後の `remove` の残りの確かめ（`internal/chief/removal.go:900-` の `leftAfter`）も、履歴の記録を含めて数える。

**ディレクトリが増えること。** 事象を書くと、送信したことのない worktree にも記録のディレクトリができる。今の `addRecords` の「送信したことのない workspace には記録がない」（`internal/chief/snapshot.go:172-173`）は、「送信の記録がない」と読み替えることになる。`addRecords` は送信・停止・PR の詳細の記録しか読まないので、振る舞いは変わらない。

**数。** 事象は変化のときだけ書くので、1 つの worktree につき、PR 1 件あたり「開く」「マージ」の 2 件ほどと、ブランチのタスクが変わるたびに 1 件である。

### 3.2. 特定の結果

`main` の行ごとに、今のセッションのその worktree の事象を当てはめて、PR ごとの最後の状態と、ブランチのタスクの移り変わりを得る。順はラウンドの ID の順とし、同じラウンドの中では、マージと閉じたことの事象を先に、`branch_task` と `pr_open` を後に置く（1.2）。そこから次の順で、最初に当たるものを特定の結果とする。

| 順 | 条件 | 結果 | 要件 |
|---|---|---|---|
| 1 | 今のセッションの履歴の記録に、読めないものが 1 つでもある（どの worktree のものでも） | 読めない | AC-09 |
| 2 | マージを確かめた PR がない | マージの記録なし | AC-07 |
| 3 | マージを確かめた PR のタスクが 1 つにそろわない（空のタスクを含む） | ほかのタスクあり | AC-06 |
| 4 | 候補のタスク T のほかに、chief が確かめたほかのタスクがある。すなわち、その worktree の PR のタスクが T でないか空である、または、T の PR のマージを最後に確かめた事象より後（同じラウンドを含む）に、T でない `branch_task` がある | ほかのタスクあり | AC-06 |
| 5 | 最後の状態が開いている PR がある | 開いている PR あり | AC-07 |
| 6 | どれにも当たらない | T と特定 | F-001 |

```go
// mainTaskKind is what the history of a worktree on main gives. The zero
// value is mainTaskUnread, the reading that assumes least.
type mainTaskKind int

const (
	mainTaskUnread mainTaskKind = iota
	mainTaskUnreadable
	mainTaskNoMerge
	mainTaskOtherTask
	mainTaskOpenPR
	mainTaskIdentified
)

// mainTaskResult is the identification of a worktree on main.
type mainTaskResult struct {
	Kind mainTaskKind
	// Task and Branch are set for mainTaskIdentified: the task and the branch
	// of the last PR of it whose merge this session read.
	Task   paneinput.TaskID
	Branch string
	// Detail names the PRs, tasks, or record behind the result, for the
	// row's reason.
	Detail string
}

// applyMainTask fills the facts of a worktree on main from its result: for
// mainTaskIdentified, Task and Docs read with Tasks.Read on Branch; for any
// other kind, the result itself. Facts of any other HEAD are returned as
// they are.
func applyMainTask(ctx context.Context, tasks Tasks, f workspaceFacts, result mainTaskResult) workspaceFacts
```

`applyMainTask` は、`main` の行の読み取りに特定の結果を当てる唯一の関数である。呼ぶのは、`buildRows`（`snapshot`・`tick`。`targets` は履歴を渡さない）、`Send`、`SendOfferedReset`（`internal/chief/resetoffer.go:144`）である。どれも、その後で今の `classifyWorkspace` を呼ぶ。

**行への当てはめ。** `workspaceFacts`（`internal/chief/state.go:190-218`）に、HEAD の種類 `headMain` と特定の結果を足す。

-   T と特定したとき：`Task` を T とし、`Docs` を `Tasks.Read(worktree, Branch)` で読む。読めなければ、今の機能ブランチの行と同じく `DocsUnreadable` になる。そのうえで `classify` は、機能ブランチの行と同じ規則で状態と次のコマンドを決める（AC-01〜AC-03）。T のディレクトリが `origin/main` にないときは、機能ブランチの行と同じく、要件定義書がない要確認になる（H-06 の 5 つ目）。
-   読めないとき（3.2 の順 1）：理由（4.2）を、読み取りの失敗として `workspaceFacts.Unreadable` に入れる。今のほかの読み取りの失敗と同じく、実行中のエージェントや開いている PR より先に、行は要確認になる（AC-09）。
-   そのほかで特定しなかったとき：特定しなかった理由（4.2）は、`decideNext` の HEAD の確かめから返す。detached HEAD の理由と同じ経路で返す。そのため、質問で止まったエージェント、実行中のエージェント、開いている PR は、今の `classify`（`internal/chief/state.go:415-`）の順のとおり、先に行の状態を決める。それらがなければ要確認で、次のコマンドは示さず、`/mkreq` も示さない。

HEAD の確かめ（今の `headProblem(h headState)`、`internal/chief/state.go:291-302`）は、特定の結果も見るように引数を `workspaceFacts` に変える。`headMain` は、T と特定したときだけブランチと同じに扱う。特定しなかった `headMain` には、タスクのないブランチの `/mkreq` の規則（`internal/chief/state.go:312-314`）を当てない。

`main` の行には、引き継ぎの提案を出さない。`needsTakeoverCheck`（`internal/chief/state.go:382-`）は、`headMain` なら偽を返す。引き継ぎは、ブランチにそのタスクのコミットと計画書があることが前提で、`main` にはないためである。こうすると、`classifyWorkspace` が `main` の worktree の先のコミットとブランチの計画書を読むこともない。

**印（`Row.Idle`）。** マージの記録なしと開いている PR ありの行に付け、fetch に失敗した行と、そのラウンドで挙げた PR の状態を読めなかった行には付けない（今の `Row.Idle` の「すべての読み取りが成功した」、`internal/chief/state.go:153-159`）。読めない・ほかのタスクありの行には付けない（1.2）。

**進行の位置の文書（`taskDocumentOf`、`internal/chief/position.go:246-269`）。** `headMain` で特定しなかった行は要確認とし、タスクのないブランチの「-」（`PositionDocNoTask`）にしない（AC-07）。T と特定した行は、機能ブランチの行と同じ規則で決まる。行で `/mkreq` が走っている（人が pane で打った）ときは、今の最初の規則のとおり要件定義書になる。

**削除の提案。** 変えない。T のタスクが完了していれば、行はタスク完了になる。T の workspace を chief が作っていれば、今の `addRemovals`（`internal/chief/removal.go:380-402`）が削除の提案を付け、HEAD が detached でない（`main` にある）ので、削除の確かめは止まる（AC-03）。

**観測の記録と自動送信。** 観測の記録は、今のとおり `f.Head == headBranch` の行だけに書く（`internal/chief/tick.go:52`）。`headMain` の行は、特定したタスクがあっても書かない。自動送信の `gather` は HEAD を自分で読み直し（`internal/chief/autosend.go:367-374`）、`main` を今のとおり `headBranch` とする。そのため `headConditions` は、「ブランチ `main` はタスクの機能ブランチでない」を理由に返す（AC-10、`internal/chief/autosend.go:212-237`）。

### 3.3. 履歴を書く（`tick`）

`tick` は、観測の記録を書いた後、ラウンドの各 worktree について次を行う。

1.  最後の状態が開いている PR のうち、そのラウンドでその worktree のブランチで開いているのを見なかったものについて、`PRStateOf` で今の状態を読む。そのラウンドに Herdr にある worktree の PR に限る。読んだ状態は、ラウンドの中の表（PR の番号ごと）に置く。自動送信の `gather` が同じ PR を読むときは、表の値を使う。こうして、1 つの PR を 1 ラウンドに 2 回読まない。
2.  3.1 の書く条件に当たる事象があれば、その worktree の排他（`Records(...).Lock()`）を取る。排他の下で worktree のブランチと履歴の記録を読み直し、ラウンドで読んだブランチと同じときだけ事象を書く。`pr_merged`・`pr_closed` は、その PR の `pr_open` がディレクトリに残っているときだけ書く。排他を取れないとき（送信の最中）、ブランチが変わったとき、`pr_open` が消えていたときは書かず、書かなかった事象をラウンドの問題に示す。違いが残っていれば、次のラウンドで書く。違いが残らなければ（その間に worktree が `main` に戻った）、その事象は失われる。これは書けなかった事象と同じ、受け入れた残る危険である（5 章）。

GitHub の読み取りの数は、1 ラウンドに、履歴で開いていて今回開いているのを見なかった PR ごとに 1 回までである。ふつうはマージの後の 1 回で、その PR の状態は決まる。開いたまま worktree が別のブランチに移った PR は、閉じるかマージされるまで、ラウンドごとに 1 回読む。

状態を読めなかったことと、事象を書けなかったことは、ラウンドの問題（`problems`）に示す。

### 3.4. ラウンドの読み取りと行の判定を分ける

`readRound` を次の 2 つに分ける。

-   `readFacts`：Herdr・fetch・送り先の表・旧い値の表・各 workspace の読み取り（今の `readWorkspace`）。記録を作らない。
-   `buildRows`：履歴の記録を読み、各 `main` の worktree の特定の結果を決めて `applyMainTask` を当てる。そのうえで、今のとおり `classifyWorkspace` で行を決め、記録・リセット・削除の提案を行に付ける（`addRecords`・`addResets`・`addRemovals`）。記録を作らない。特定したタスクの文書を読む（`Tasks.Read`）ほかは、今の読み取り（先のコミット、ブランチの計画書、削除の確かめ）と同じものを読む。

`TakeSnapshot` と `Targets` は、`readFacts` の後に `buildRows` を呼ぶ。`Targets`（`chief targets`）は、履歴の記録を読まず、`main` の行を特定しなかったもの（マージの記録なし）として決める。`main` の行は特定してもしなくても `/mkreq` を提案しないので、読んでも `targets` の結果は変わらないためである。`Tick` は 2.3 の順で呼ぶ。`BeginRound` に失敗したラウンドは、今のとおり事象も観測も書かずに、`buildRows` で行を決めて返す（`internal/chief/tick.go:41-45`）。

### 3.5. 人が選ぶ送信（`chief send`）

```go
// SendRequest gains the item the human chose.
type SendRequest struct {
	workspace string
	kind      RequestKind
	issue     paneinput.IssueNumber
	pr        fixprguard.PRNumber
	// item is the ID of the proposal item the human chose; required for the
	// task commands and refused for the others.
	item ItemID
}

// ParseSendRequest takes --item as well.
func ParseSendRequest(workspace, command, issue, pr, item string) (SendRequest, error)

// Send takes the caller's owner, whose session's history it reads.
func Send(ctx context.Context, deps Deps, owner Owner, req SendRequest) (SendOutcome, error)
```

-   `--item` は、`mkarch`・`mkplan`・`runplan`・`runplan-takeover` で必須、`mkreq`・`runpr` では受け付けない。項目 ID の形（今の `ItemID` の形）に合わなければ `ErrBadRequest` とする。
-   `Send` は、今のとおり workspace の排他を持ってから行を作り直す。`main` の worktree なら、呼び出した chief の pane のセッション（`Owner.Session`）の履歴の記録を読み、`applyMainTask` を当てる。3.2 の順 1 の判定のため、読むのはすべての worktree のディレクトリである。作り直した行の提案（次のコマンドか引き継ぎ）の項目 ID を今の `proposalID` で作り、`--item` と違えば、何も送らずに `NotSentNotOffered` で拒む（AC-05）。次のコマンドの種類が違うときは、今のとおり拒む。
-   `main` の行は `/mkreq` を提案しないので、`mkreq` の送信は今の `offeredMkreq`（`internal/chief/request.go:416-425`）の確かめで拒まれる（AC-08）。
-   送る前の確かめ（送り先の pane、モデル、送信の記録）は、今の `sendCommand` をそのまま通す（AC-04）。
-   事象を書くのも worktree の排他を持つ処理なので（3.3）、`Send` が排他を持って行を作り直してから送るまでの間に、その worktree の事象は増えない。

`chief reset`（`SendOfferedReset`）も、行を作り直すときに同じく `applyMainTask` を当てる。`applyMainTask` を当てないと、`tick` が特定して示した `main` の行のリセットの項目を、`reset` が要確認の行と読み、`check_mismatch` で拒んでしまうためである。

`cmd/chief` は `--item` を受け取り、`Send` に呼び出し元の `Owner` を渡す。

### 3.6. `/chief` の定義・利用ガイド・プロトコル番号

-   **`/chief` の定義（`.claude/commands/chief.md`）。** 提案を送る呼び出しを `chief send --workspace <workspace> --command <request> --item <id> --protocol <番号>` にする。`<id>` は、人が選んだ項目の `id` である。`mkreq` と `runpr` の呼び出しは変えない（`--item` を付けない）。出力の項目は変えない（要件定義書 4.4）。
-   **プロトコル番号（H-04）。** `chief send` の引数が変わり、古い定義の呼び出し（`--item` なし）は新しい道具に拒まれ、新しい定義の呼び出しは古い道具に拒まれる。そのため `main` の値 + 1 にする（今の値は `internal/chief/records.go:29` の `Protocol`）。値の組み合わせの変化だけなら上げる必要はない。`/chief` の定義は、行の `task` が空かどうかで送るコマンドを決めておらず、`position.document` の値の表（`.claude/commands/chief.md:176-182`）にも新しい値はないためである。
-   **利用ガイド（AC-12）。** `docs/dev/guides/chief_guide.md` の進行の位置の説明（今の `chief_guide.md:82`）に、要件定義書 AC-12 の 4 点と、「-」が `main` をチェックアウトした worktree には使われないことを書く。あわせて次を書く。
    -   要確認から戻す方法：読めない記録とほかのタスクありは、chief の pane で `/clear` をするか Claude Code を起動し直して、新しいセッションにする（`start` が古いセッションの記録を消す。すべての worktree の特定もやり直しになる）。読めない記録のファイルだけを消すことは勧めない。中身の分からない記録を消すと、ほかのタスクを示す事象が失われ、古いタスクを特定しうるためである。一覧を読めないディレクトリは、理由に示したディレクトリの権限などを直す。
    -   道具を入れ直しても、同じ Claude Code のセッションで `/chief` を起動し直すだけなら、セッションは変わらず、特定は続く。
    -   `chief send` の `--item`。

### 3.7. コンポーネントの責務

| ファイル | 変更 | 責務 |
|---|---|---|
| `internal/chief/trace.go` | 新規 | 履歴の記録の型・名前・読み取り（形の求め）・書く条件の判定・特定の結果・`applyMainTask` |
| `internal/chief/records.go` | 変更 | 履歴の記録の名前を `isRecordFile` に足す。排他の下で事象を書くメソッド。`start` の片付けでほかのセッションの事象を消す。`Protocol` を `main` の値 + 1 にする |
| `internal/chief/state.go` | 変更 | `headMain`、`workspaceFacts` の特定の結果、`classify`・`decideNext`・`headProblem`・`needsTakeoverCheck` の `headMain` の扱い、`Row.Idle` |
| `internal/chief/position.go` | 変更 | `taskDocumentOf` の `headMain` の規則 |
| `internal/chief/snapshot.go` | 変更 | `readRound` を `readFacts` と `buildRows` に分ける。`readWorkspace` がブランチ `main` を `headMain` と読む |
| `internal/chief/tick.go` | 変更 | 履歴を書き、行を決める順。PR の状態の読み取りを自動送信と共有する |
| `internal/chief/autosend.go` | 変更 | `gather` の PR の状態の読み取りに、ラウンドの中の表を使う（条件と結果は変えない） |
| `internal/chief/request.go` | 変更 | `SendRequest` の `item`、`ParseSendRequest`、`Send` の `Owner`・`applyMainTask`・項目 ID の照合 |
| `internal/chief/resetoffer.go` | 変更 | `SendOfferedReset` の `applyMainTask` |
| `internal/chief/create.go` | 変更 | `Targets` が `readFacts`・`buildRows` を使う |
| `cmd/chief/main.go` | 変更 | `--item` の受け取り、`Send` への `Owner` |
| `.claude/commands/chief.md` | 変更 | `chief send` の呼び出しの `--item` とプロトコル番号 |
| `docs/dev/guides/chief_guide.md` | 変更 | AC-12 の説明、要確認から戻す方法、`--item` |
| `docs/dev/adr/0046-chief-of-staff.md` | 変更 | 1.2 の ADR の更新（本 PR で更新済み） |

今の振る舞いを確かめるテストのうち、変わるもの：

-   ブランチ `main` をタスクのないブランチの例として使うテスト（`internal/chief/snapshot_test.go:71-73`・`snapshot_test.go:99`・`snapshot_test.go:154`・`snapshot_test.go:172`・`snapshot_test.go:465`・`snapshot_test.go:486-496`・`snapshot_test.go:707`・`snapshot_test.go:730`・`snapshot_test.go:880-881`、`internal/chief/workspace_test.go:53`）。今の「`main` は `/mkreq`」を前提にしているので、`main` でも機能ブランチでもない名前に替える。AC-11 の 3 つ目は、替えた後のテストが確かめる。
-   `ParseSendRequest` と `Send` を呼ぶテスト（`internal/chief/send_test.go`・`create_send_test.go`・`records_test.go`・`reset_test.go`・`resetoffer_test.go`）。引数に項目 ID と `Owner` が加わる。
-   `targets` が `main` の worktree を挙げることを前提にしたテストがあれば、挙げないことに替える。
-   `/chief` の定義の呼び出しの形とプロトコル番号を確かめる文書のテスト（`cmd/chief/docs_test.go`）。
-   `internal/chief/removal_test.go` の `main` は、削除の確かめの HEAD の読み取りで、行の判定ではないので変わらない。

## 4. エラーハンドリング設計

### 4.1. エラーの型

新しい sentinel は足さない。履歴の記録の形に合わない記録は、今の `errRecord` で包む（要件定義書 4.2）。`chief send` の `--item` の誤りは、今の `ErrBadRequest` で包む。

### 4.2. 理由の文言（H-03）

道具の出力は英語である。`main` の行の理由は、次の形とする。`<...>` には `Detail` の値が入る。

| 特定の結果 | 理由 |
|---|---|
| 読めない | `the worktree is on main and a history record of this chief session cannot be read (start a new chief session with /clear): <記録のパスと誤り>` |
| マージの記録なし | `the worktree is on main and this chief session has seen no merged PR of it, so its task is not known` |
| ほかのタスクあり | `the worktree is on main and this chief session has seen it with more than one task or with a PR of no task: <タスクと PR の並び>` |
| 開いている PR あり | `the worktree is on main and a PR this chief session saw open on it is not known to be merged or closed: <PR の並び>` |

開いている PR ありの理由に挙げた PR のうち、そのラウンドで状態を読めなかったものは、ラウンドの問題にも `the state of #<PR> cannot be read for the history of <worktree>: <誤り>` と示す。こうして、まだ開いているのか、読めないのかを見分けられる。

`chief send` の拒否の理由：

| 場合 | 結果と理由 |
|---|---|
| 行の提案が `--item` の項目でない（AC-05） | `not_sent`・`not_offered`、`the workspace no longer offers the chosen item (it offers <今の提案の文> now)` |
| `main` の行への `mkreq`（AC-08） | 今の `offeredMkreq` のとおり `not_sent`・`not_offered`、`the workspace is not one without a task that waits for input` |

履歴を書けなかったときは、ラウンドの問題に `the history record of <worktree> cannot be written: <誤り>` と示す。

### 4.3. 失敗の向き

| 失敗 | 振る舞い |
|---|---|
| 履歴の記録を読めない（形に合わない名前、一覧を読めないディレクトリを含む） | そのラウンドのすべての `main` の行を要確認にする（AC-09）。読めない記録を除いて特定しない |
| 履歴の記録を書けない | ラウンドの問題に示す。次のラウンドで同じ違いがあれば書き直す。残る危険は 5 章 |
| worktree の排他を取れない、排他の下でブランチが変わっていた、`pr_open` が消えていた | そのラウンドはその worktree の事象を書かず、ラウンドの問題に示す。違いが残れば次のラウンドで書く |
| PR の状態を読めない | 事象を書かず、ラウンドの問題に示す。その PR は開いているままで、`main` の行は特定しない |
| `BeginRound` に失敗した | 今のとおり事象も観測も書かず、自動送信もしない。行は書かれている事象から決める |
| 作成の排他を取れない | 今のとおり自動送信をしない。履歴の事象は書く（作成の排他は使わない） |
| 特定したタスクの文書を読めない | 機能ブランチの行と同じく、`DocsUnreadable` で要確認 |

## 5. セキュリティ考慮事項

条件付きのガイド（`docs/dev/security.md`）の発動条件（`yt-dlp`、API キー、キャッシュ、LLM API、信頼できない文字列のプロンプトや記事への埋め込み）には当たらない：N/A。

信頼できない入力の境界は、履歴の記録である（要件定義書 4.2）。状態のディレクトリは同じ利用者のほかのプロセスが書ける（ADR-0046 の「結果」）。図の矢印は「データの流れ」を表し、赤は信頼できない書き手である。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef problem fill:#ffe6e6,stroke:#d62728,stroke-width:2px,color:#7b0000;

    OTHER["同じ利用者のほかのプロセス"]
    TRACE[("履歴の記録")]
    READER["trace.go の読み取り"]
    APPLY["applyMainTask"]
    DOCS[("origin/main の文書")]
    CLASSIFY["classify"]
    HUMAN["人が選んだ項目<br>（--item）"]
    SEND["Send"]
    PANE["タスクの pane"]

    OTHER -->|"偽った事象"| TRACE
    TRACE --> READER --> APPLY --> CLASSIFY
    DOCS --> CLASSIFY
    CLASSIFY --> SEND
    HUMAN --> SEND
    SEND -->|"そのタスクの次のコマンドだけ"| PANE

    class OTHER problem
    class TRACE,DOCS data
    class READER,APPLY,CLASSIFY,SEND enhanced
    class HUMAN,PANE process

    subgraph Legend["Legend"]
        L1["信頼できない書き手"]
        L2[("データ")]
        L3["新しい部品・変える部品"]
        L4["今の部品・人"]
    end
    class L1 problem
    class L2 data
    class L3 enhanced
    class L4 process
```

-   **形の求め。** 3.1 の読み取りが、要件定義書 4.2 の形をすべて求める。形に合わない記録は、直さずに拒み、すべての `main` の行を要確認にする。偽った記録や壊れた記録が 1 つあるだけで、そのセッションが続く間、すべての `main` の行が要確認になりうる。これは安全な側の失敗で、新しいセッションにすれば戻る（3.6）。
-   **送るもの。** 偽った事象が別のタスク T' を示すと、`main` の行は T' の次のコマンドを示しうる。それでも、自動送信はせず（AC-10）、人が行と項目を見て選んだときだけ送る。送られるのは、T' の `origin/main` の文書が示す次のコマンドだけで、送りうる入力の範囲（ADR-0046）は変わらない。これは要件定義書 4.2 が受け入れた残る危険である。
-   **書けなかった事象。** 事象を書けないと、その worktree がほかのタスクのブランチにいたことを履歴が知らないまま `main` に戻ることがある。そのとき行は古いタスクを示しうる。ラウンドの問題に書けなかったことが示され、送るには人の選択が要る。これを残る危険として受け入れた（1.2）。
-   **作り直したパス。** chief の `create` と `remove` を通さずに、人が同じパスに worktree を作り直して `main` をチェックアウトすると、そのセッションの古い事象が当てはまりうる。行に示されるタスクを人が見てから選ぶので、同じく残る危険として受け入れる。chief の `create` と `remove` を通せば、その worktree の排他の下で `ClearRecords` が消す。事象を書くのも同じ排他の下で、ブランチと履歴の記録を読み直してからなので、消された後のディレクトリに古い worktree の事象を書くことはない。
-   **競り合い。** `chief send` は worktree の排他の下で行を作り直してから照合して送り、事象を書くのも同じ排他の下なので、照合と送信の間に履歴は変わらない。履歴以外の変化（ブランチの切り替えなど）が人の見た後に起きていれば、項目 ID の照合で拒まれる。

## 6. 処理フローの詳細

### 6.1. `main` の行の判定

図の矢印は判定の順を表す。質問・実行中・開いている PR による行の状態は、今の `classify` のとおり、この判定より先に決まる。

```mermaid
flowchart TD
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef problem fill:#ffe6e6,stroke:#d62728,stroke-width:2px,color:#7b0000;

    START["ブランチ main の行"]
    Q1{"読めない記録がある"}
    Q2{"マージを確かめた PR がある"}
    Q3{"タスクが 1 つにそろい、<br>ほかのタスクがない"}
    Q4{"開いている PR がない"}
    UNREAD["要確認：記録を読めない<br>（印なし）"]
    NOMERGE["要確認：マージの記録なし<br>（印あり）"]
    OTHER["要確認：ほかのタスクあり<br>（印なし）"]
    OPEN["要確認：開いている PR あり<br>（印あり）"]
    IDENT["T と特定：<br>機能ブランチと同じ規則で行を決める"]

    START --> Q1
    Q1 -->|"はい"| UNREAD
    Q1 -->|"いいえ"| Q2
    Q2 -->|"いいえ"| NOMERGE
    Q2 -->|"はい"| Q3
    Q3 -->|"いいえ"| OTHER
    Q3 -->|"はい"| Q4
    Q4 -->|"いいえ"| OPEN
    Q4 -->|"はい"| IDENT

    class START process
    class Q1,Q2,Q3,Q4,IDENT enhanced
    class UNREAD,NOMERGE,OTHER,OPEN problem

    subgraph Legend["Legend"]
        L1["行"]
        L2["新しい判定と特定"]
        L3["要確認になる結果"]
    end
    class L1 process
    class L2 enhanced
    class L3 problem
```

### 6.2. issue #996 の事象の流れ

1.  ラウンド N-1：worktree は `isseis/0102-chief-text-table-02` にいて、PR が開いている。`branch_task(0102)` と `pr_open(#991, 0102)` を書く。
2.  マージの後、`mergepr cleanup` が worktree を `main` に移す。
3.  ラウンド N：worktree は `main` にいる。#991 は履歴で開いているが、ブランチで開いているのを見ないので状態を読む。`MERGED` なので、worktree の排他の下で `pr_merged(#991)` を書く。特定の結果は T = 0102 である。
4.  行は、`origin/main` の 0102 の文書から決まる。設計書が `draft` なら要確認で、理由は「設計書が approved でない」である（AC-02）。設計書が approved で計画書がなければ、`/mkplan 0102` を提案する（AC-01）。自動送信は、今のとおり「ブランチ `main` はタスク 0102 の機能ブランチでない」を理由に送らない（AC-10）。
5.  人が提案の項目を選ぶと、`/chief` は `chief send ... --command mkplan --item <その項目の id>` を呼ぶ。道具は排他の下で行を作り直し、同じ項目を提案していることを確かめてから送る（AC-04・AC-05）。

## 7. テスト戦略

### 7.1. 単体テスト

-   **特定の結果（3.2）。** 表形式のテストで、6 つの順の条件ごとに結果を確かめる。次の場合を含める。
    -   AC-06 の 2 つの例と、AC-07 の 3 つの例。
    -   T の PR のマージの後に T2 のブランチを見てから、T のブランチに戻った場合（特定しない）。
    -   T2 のブランチを見て T のブランチに戻った後に、T の PR のマージを確かめた場合（特定する）。
    -   T2 のブランチにいる間に T の PR のマージを確かめ、その後 `main` に戻った場合（特定しない）。
    -   T の PR のマージと T2 の `branch_task` が同じラウンドにある場合（特定しない）。
    -   タスクの空の PR がある場合（特定しない）。
    -   閉じた PR が開き直された場合。
-   **書く条件（3.1・3.3）。** 変化がないラウンドでは事象を書かないこと。`OPEN` と読み取りの失敗では書かないこと。同じ PR の状態を 1 ラウンドに 1 回しか読まないこと。排他を取れないときと、排他の下でブランチが変わっていたときに書かないこと。
-   **行の判定（AC-01〜AC-03・AC-07）。** `headMain` と特定の結果を与えた `classify`・`documentOf` の結果（状態・文書・進み具合・次のコマンド・`Idle`）。質問で止まったエージェントがあれば質問の行になること。引き継ぎを出さないこと。AC-03 では、chief が作った workspace の削除の提案が detached の確かめで止まること。
-   **送信（AC-04・AC-05・AC-08）。** `--item` が要るコマンドと要らないコマンドの `ParseSendRequest`。行の提案が変わったときと、自動送信の提案が行と別のタスクを示すときの `not_offered`。`main` の行への `mkreq` の拒否。特定した行への `/mkplan` が今の送る前の確かめを通ること。`chief reset` が特定した `main` の行のリセットを送れること。`targets` が `main` の worktree を挙げないこと。
-   **変えない振る舞い（AC-10・AC-11）。** 特定した `main` の worktree の PR の自動送信の結果が、今と同じ理由で送らないこと。機能ブランチ・detached HEAD・ほかのブランチの行が、事象があっても今と同じ行になること。
-   **消し方。** `ClearRecords` が履歴の記録を消すこと。`start` がほかのセッションの事象を消し、今のセッションの事象を残すこと。

### 7.2. 結合テスト

`tick` の fake の Herdr・git・GitHub（今のテストの道具立て、`internal/chief/test_helpers.go`）で、6.2 の流れを 2 ラウンドで走らせる。次のことを確かめる。

-   マージを確かめたラウンドの行に T が示される。
-   続く `chief send --item` で送られる。
-   別のセッションの `snapshot` では要確認になる。

### 7.3. セキュリティのテスト

履歴の記録の読み取りが、次の記録をそれぞれ `errors.Is(err, errRecord)` で拒み、すべての `main` の行を要確認にすること（AC-09）。

-   `task` が `"../x"`・`"0102 "` の記録。
-   不正な UTF-8、対のないサロゲート、項目の繰り返し、知らない項目、知らない `kind` を含む記録。
-   `pr_open` の `task` が `branch` から抽出したタスクと違う記録。
-   名前やディレクトリと合わない `session`・`worktree` を持つ記録と、上限を超える記録。
-   `trace-` で始まり形に合わない名前の記録。

各テストは、CLAUDE.md の「Every test must be able to fail for its stated reason」のとおり、確かめる部分を壊して失敗することを確かめる。

### 7.4. 文書のテスト

-   `cmd/chief/docs_test.go` の、`/chief` の定義の呼び出しの形とプロトコル番号の確かめに、`--item` を足す。
-   ガイドの文言と、ADR-0046 が本書と合っていること（AC-13）は、PR のレビューで読んで確かめる（CLAUDE.md の「Document tests check values, structure, and executable examples, never prose」）。

## 8. 実装の優先順位

1.  履歴の記録の型・名前・読み取り・書き込み・特定の結果（`trace.go`）と、`isRecordFile`・`start` の片付け。
2.  `headMain`・`applyMainTask`・`classify`・`documentOf`、`readRound` の分割と、`snapshot`・`targets`・`reset` の読み取り。
3.  `tick` の書く順と、PR の状態の読み取りの共有。
4.  `chief send` の `--item`、`Send` の照合、`cmd/chief`、プロトコル番号、`/chief` の定義、利用ガイド。

2 で `main` の行が提案を出すようになるので、2〜4 は同じ PR に入れる（1.2）。1 だけを先の PR に分けてもよい。1 は行を変えないためである。

タスク 0102（[0102 計画書](../0102_chief_text_table/03_implementation_plan.md)）も、chief の出力と `/chief` の定義を変える。プロトコル番号は、実装するときの `origin/main` の値 + 1 にする（CLAUDE.md の「Protocol numbers」）。

## 9. 将来の拡張性

-   detached HEAD の worktree にも履歴の記録からの特定を当てるなら、3.2 の結果を detached の行に当てはめ、削除の提案の行（`RemovalRow`）との順を決めれば足りる。本タスクではしない（要件定義書 2.3）。
-   chief の pane のセッションを越えて特定するなら、履歴の記録をセッションに属さない記録にし、消す規則を足す必要がある。本タスクではしない（要件定義書 2.3）。

## 付録 A. 申し送りの扱い

| 項目 | とった方針 | 本書の節 |
|---|---|---|
| H-01 | 自動送信の引き継ぎとは別に、セッションに属する worktree ごとの履歴の記録を足す。窓と引き継ぎの期限は当てない | 1.2・3.1・3.3 |
| H-02 | `chief send --item` で人が選んだ項目を渡し、送る直前に作り直した行の提案と照合する。`main` の git の状態は送る条件に足さない | 1.2・3.5 |
| H-03 | 理由の文言は 4.2。AC-07 の行に印を付け、AC-06・AC-09 の行には付けない | 1.2・3.2・4.2 |
| H-04 | `/chief` の定義は値の組み合わせに依らないが、`chief send` の引数が変わるのでプロトコル番号を上げる | 3.6 |
| H-05 | 最後にマージを確かめた T の PR のブランチ名で `Tasks.Read` を使う | 1.2・3.2 |
| H-06 | 形の求めは履歴の記録にだけ当てる（1 つ目）。使う項目は worktree・PR の番号・タスク・事象の種類・`pr_open` のブランチで、それぞれの形は 3.1 の読み取りのとおり（2 つ目）。キーは worktree の正規化したパスで、作り直しは排他の下の `ClearRecords` が消す（3 つ目）。fetch に失敗した AC-07 の行には印を付けない（4 つ目）。T のディレクトリが `origin/main` にないときは、機能ブランチの行と同じく要件定義書がない要確認（5 つ目） | 1.2・3.1・3.2・5 |
| H-07 | 履歴の記録の `branch_task`・`pr_open`（タスクが空のものを含む）と PR の最後の状態で、chief が確かめたほかのタスクと、chief が最後に開いていると確かめた PR を決める | 3.1・3.2 |

## 変更履歴

- 2026-11-19 承認（isseis、#1015）
