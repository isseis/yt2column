# アーキテクチャ設計書：chief が issue からタスク用の worktree と workspace を作り、タスクが終わったら確かめて消す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-16 |
| Review date | - |
| Reviewer | - |
| Comments | 決定の変更で draft に戻した：0093 の表での削除の提案の行の扱い（申し送りの H-09）と、0093 と重なる `--protocol` の番号の決め方を足した |

本書は [01_requirements.md](01_requirements.md)（要件定義書。以下、要件書）の設計である。「計画で決める」は、本タスクの実装計画書（`03_implementation_plan.md`）で決めることを指す。既存のファイルの内容は、コミット `bc41cf17` で確かめた。`file:line` の行番号は、このコミットでの行番号である。F-NNN・AC-NN は要件書の項番を、H-NN は [design_handoff.md](design_handoff.md)（以下、申し送り）の項番を指す。申し送りへの対応は付録 A に、受け入れ基準との対応は付録 B にまとめる。

本書は、要件書 3 章と 6 章の用語（作った workspace・タスクの完了・人の確認・`issue-<N>`）と、[0088 設計書](../0088_chief_of_staff/02_architecture.md)（以下、0088 設計書）の冒頭の用語（chief のセッション・道具・`/chief` の定義・状態のディレクトリ・周・送信の記録など）を、同じ意味で使う。そのほかに、次の用語を使う。

-   **作る場所:** issue 番号 `<N>` から決まる、作る worktree のパス（`$HOME/.herdr/worktrees/yt2column/issue-<N>`）と workspace の名前（`issue-<N>`）の組（3.2）。
-   **作ったことの記録:** 要件書の用語と同じで、道具が worktree を作ったときに状態のディレクトリに残す記録（3.5）。
-   **パスの workspace:** あるパスかその下を作業ディレクトリとする pane を持つ Herdr の workspace（3.3.2）。
-   **削除の候補:** 作ったことの記録が名指し、AC-23 の条件を満たした workspace（3.4.1）。
-   **削除の確かめ:** AC-28 の確かめに、本書で足す確かめ（1.2、3.4.2）を加えたもの。

## 1. 設計の全体像 (Design Overview)

### 1.1. 設計原則

-   **作ることと消すことは道具が行い、決まった操作に限る。** 道具は、issue 番号だけから決まるパスと名前で作り、作ったことの記録が名指して確かめを通った workspace だけを消す。セッションが渡すのは、issue 番号と、道具が返した workspace の識別だけである。道具は、どちらも検証してから使う（要件書 4.2、[ADR-0046](../../dev/adr/0046-chief-of-staff.md)、[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)）。
-   **Herdr に書く部品は型で限る。** 0088 では、pane へ入力を送るのは `PaneWriter` だけで、chief の workspace へは送らないことを型で守る（`internal/chief/herdr.go:388-394`）。本書は、workspace を作る部品と閉じる部品を、それぞれ検証済みの値からしか作れない型にする（3.6）。
-   **0088 の部品を使い、送る入力の範囲を広げない。** 作った workspace への `/mkreq` は、0088 の送信の手順（`internal/chief/send.go:399` の `sendCommand`）で送る。送る入力は、0088 の F-006 の表のものだけである。
-   **記録は名指すだけで、送るコマンドを決めない。** 作ったことの記録は、削除の候補を名指すことと、その行に示すタスクを探す手がかりにだけ使う。名指された workspace は、Herdr・git・`origin/main` の文書で確かめ直す。送るコマンドと送り先を記録から決めないことは、0088 のとおりである。
-   **分からないものは消さない。** Herdr・git・gh の出力、作ったことの記録、`origin/main` の文書のどれかが決まった形に合わなければ、作らないか、消さないか、提案しない。理由も示す（CLAUDE.md の「Reject, don't normalize」）。
-   **消す最後の守りを git に置く。** worktree は `git worktree remove` で `--force` を付けずに消す。道具の確かめの後に変更が入っても、git が消すことを拒む。

### 1.2. 設計の際に決めたこと（2026-11-16）

-   **worktree は `git worktree add --detach <パス> <コミット>` で作り、Herdr の workspace は `herdr workspace create --cwd <パス> --label issue-<N> --no-focus` で作る。消すときは `herdr workspace close` で workspace を閉じ、`git worktree remove <パス>` で worktree を `--force` なしに消す。`herdr worktree create`・`open`・`remove` は使わない（申し送りの H-02）。** 理由は 2 つある。1 つめに、Herdr 0.9.1 の `herdr worktree create` は `--branch`・`--base` を持つが、ブランチを作らない指定がなく、AC-08 に合うかを確かめられない。2 つめに、`git worktree add --detach` と `git worktree remove` は、ブランチを作らないことと、変更のある worktree を拒むことが git の仕様で決まっている。こうすると、Herdr への依存は workspace とタブとエージェントの操作だけになる。`herdr worktree open` で開く案は、`--trust-repository` の要否と、`herdr worktree remove` が workspace を閉じるかが確かめられていないので採らない。ADR: なし（ADR-0046 の「detached の worktree を作り、workspace で開く」という決定を実現する手段で、このタスクの中で閉じるため）。
-   **作る workspace の pane は、今の手作りの形と同じく、タブ 3 つに 1 つずつ置く。タブの名前は `cmd`（シェル）、`opencode`、`claude` とする。** 今の手作りの workspace がこの形である（2026-11-16、この worktree の workspace で `herdr tab list` を実行して確かめた）。1 つのタブを `pane split` で 3 つに分ける案は、手作りの形と違い、人が慣れた配置を変えるので採らない。ADR: なし（配置はこのタスクの中で閉じるため）。
-   **確かめていない外部の振る舞いは、実装の最初のフェーズで人が実地に確かめる（7.3）。** 確かめていないのは次のものである。(i) 新しい worktree で Claude Code がフォルダの信頼を問う画面を出すか。(ii) `herdr agent start` が新しいタブのシェルの pane で動くか、時間切れのときに何を返すか。(iii) `herdr workspace create --cwd` と `herdr tab create --cwd` が作業ディレクトリをそのとおりにするか。(iv) `herdr workspace close` が、その workspace のすべてのタブと pane だけを閉じ、`workspace_group_close_required` を返さないか。あわせて、Herdr が `~/.herdr/worktrees/yt2column/` に git で作った worktree を、Herdr 自身が作った worktree として扱わないかも確かめる。(v) 起動したばかりの Claude Code が、0088 の送る前の確かめ（画面の形、Herdr のセッション ID、transcript の場所）を通るか。(vi) `.claude/settings.json` の `ask` の規則が、chief の pane の許可の設定（auto mode を含む）で確認を出すか。設計は、どの結果でも安全側に倒れる（3.3.4・3.4.3）。(vi) が出さないときは、計画の段で、要件書 4.2 の残る危険に戻すかを人と決め直す。設計の前に試す案は採らない。試すには人の Herdr のセッションに実の workspace を作る必要があり、Claude Code のセッションからの `herdr --session` の呼び出しは `.claude/settings.json` が拒むためである。ADR: なし（確かめる時機はこのタスクの中で閉じるため）。
-   **issue は `gh api graphql` の `repository.issueOrPullRequest(number: <N>)` で読み、`__typename` で issue と PR を見分ける。問い合わせるのは、issue の `number`・`state`・`title` と、PR の `number` だけである（申し送りの H-07）。** 2026-11-16 に、このリポジトリで 3 回読んで確かめた。issue #839 は、終了コード 0 で `__typename` が `Issue`、`state` が `OPEN` だった。PR #897 は、終了コード 0 で `__typename` が `PullRequest` だった。存在しない #999999 は、終了コード 1 で、標準出力の JSON の `issueOrPullRequest` が `null`、`errors` が `type` `NOT_FOUND` の 1 つだった。REST の `GET /repos/{owner}/{repo}/issues/<N>` は、issue の本文（65,536 文字まで）と作者・ラベルなどを返すので、本文の長い issue が要件書 4.2 の上限 64 KiB を超え、正しい issue を拒みうる。そのため採らない。`gh issue view` は PR の番号と存在しない番号を同じ失敗として返しうるので採らない。`--jq` で形を変える案は、重なったメンバーを拒めなくなる（申し送りの H-07）ので採らない。ADR: なし（どの操作を使うかは CLAUDE.md の「Tool Execution Safety」に書く。ADR-0046 の「issue の取得を承認に加える」という決定は変わらない）。
-   **`.claude/settings.json` の拒否に、作る herdr の呼び出し（`herdr workspace create`・`herdr tab create`・`herdr pane split`・`herdr pane move`・`herdr worktree create`・`herdr worktree open`）と、`git worktree add`・`git worktree remove`・`git worktree move` を足す。道具の削除のサブコマンド `chief remove` は `ask` の規則に置き、呼ぶたびに Claude Code の許可の確認を出させる（申し送りの H-05）。** 拒否を足すと、セッションが道具を通さずに workspace や worktree を作ること、git で消すことを防げる。`ask` の規則を足すと、人の確認（AC-27）を、セッションの外の Claude Code の許可の仕組みでも求められる。要件書 4.2 で残る危険とした「人の確認なしに削除のサブコマンドを呼びうる」ことが狭まる。代わりに、削除のたびに `/chief` の問いと許可の確認の 2 回、人が答える。人が Claude Code に Herdr の pane や git の worktree を作らせると拒まれる。この 2 つを受け入れる。拒否だけを足す案と、何も足さない案は、人の確認をセッションだけに任せることになるので採らない。ADR: [ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新する（拒否の一覧は ADR-0046 の決定にあるため）。
-   **消す直前の確かめに、エージェントでない各 pane の前面のプロセスがその pane のシェル自身であることを足す。`herdr pane process-info` で読み、読めないときと知らない形のときは消さない（申し送りの H-06）。** workspace を閉じると、シェルの pane で動いているビルドやエディタも終わり、保存していない内容が失われるためである。`herdr pane process-info` は、前面のプロセスグループの ID と、前面のプロセスの一覧と、シェルの pid を返す（2026-11-16、この workspace のシェルの pane で確かめた。前面は `zsh` だけで、前面のプロセスグループの ID がシェルの pid と同じだった）。pane の数を示すだけにする案は、閉じる前に止める手段がなく、作業を失いうるので採らない。ADR: [ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新する（消す前の確かめは ADR-0046 の決定にあるため）。
-   **作ったことの記録は、0088 の workspace ごとの記録のディレクトリ（worktree の正規のパスの SHA-256）に `created.json` として置き、削除の候補を名指すことにだけ読み返す。そのディレクトリの記録は、作るときに前の使い方の分を消してから作り、消し終えたときにすべて消す（申し送りの H-01、3.5）。** 0088 の記録と同じ置き場所と読み書きの部品を使えば、置き場所の規則が増えず、workspace ごとの排他の中で作成と削除と送信を順に並べられるためである。同じパスを作り直すと、前の使い方の送信の停止や終わりのない送信の記録が残り、新しい workspace への `/mkreq` を止めるので、作り直す前に消す。要件書は、記録を削除の候補を名指すことにも使うと決めた（ADR-0046 の決定）。ADR-0014 は、道具が判定に読み返す記録を例外として列挙しているので、その一覧に足す。ADR-0046 の「同名のものがあれば作らず、上書きしない」記録の決まりに、この記録の置き換えと消し方を足す。ADR: [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) と [ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新する。

### 1.3. なぜ今の仕組みでは足りないか

-   **作る workspace へは、0088 の `send --command mkreq` で送れない。** 0088 の `send` は、その workspace の行が `/mkreq` を提案しているときだけ `/mkreq` を送る（`internal/chief/request.go:344-353` の `offeredMkreq`）。作った直後の worktree は HEAD が detached なので、行は「要確認」になり、`/mkreq` を提案しない（`internal/chief/state.go:256-263` の `headProblem`）。そのため、作成に続く送信は、行の状態によらずに送る別の入口が要る（AC-15、3.3.4）。
-   **完了したタスクの worktree からは、タスクを読めない。** 0088 の道具はタスクをブランチから読み（`internal/chief/snapshot.go:241-250`）、HEAD が detached の worktree はタスクがあるともないとも決めない（0088 設計書の 1.3.1）。`/runpr` は最後の PR の後に HEAD を detached にする（`.claude/commands/runpr.md:1254-1256`）。そのため、どの workspace のタスクが完了したかは、作ったときに記録した issue からしか分からない（AC-23）。
-   **道具には、Herdr の workspace と git の worktree を作る口も、閉じて消す口もない。** 0088 の道具が Herdr に書くのは、`PaneWriter` の `herdr agent prompt`・`herdr agent send-keys` だけである（`internal/chief/herdr.go:395` 以下）。
-   **始める先の候補を選ぶ前に、issue 番号の形を道具で確かめる口がない。** 0088 の `/mkreq` の提案は、`snapshot`・`tick` の一覧の行ごとの項目で、番号は `send` の引数で初めて検証される（`internal/chief/request.go:107-110`）。AC-02 は、番号の形を確かめてから始める先を尋ねることを求めるので、番号を検証して候補を返す `targets` を足す（3.3.5）。

### 1.4. 概念モデル

図 1 は、本書が足すものと、その関係を示す。矢印 A → B は「A から B が決まる」を表す。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef newpkg fill:#ffe8f5,stroke:#d946ef,stroke-width:2px,color:#701a75;

    NUM["issue 番号"]
    ISSUE[("GitHub の issue")]
    SPACE["作る場所"]
    WT[("worktree")]
    WS["作った workspace"]
    REC[("作ったことの記録")]
    DOCS[("origin/main の要件定義書と計画書")]
    CAND["削除の候補"]
    ROW["タスクの workspace の行"]

    NUM --> ISSUE
    NUM --> SPACE
    SPACE --> WT
    SPACE --> WS
    SPACE --> REC
    REC --> CAND
    DOCS --> CAND
    WS --> CAND
    CAND --> ROW

    class ISSUE,WT,REC,DOCS data
    class NUM,ROW process
    class SPACE,WS,CAND newpkg

    subgraph Legend["Legend"]
        L1[("データ")]
        L2["今ある概念"]
        L3["本タスクで足す概念"]
    end
    class L1 data
    class L2 process
    class L3 newpkg
```

-   作る場所は、issue 番号だけから決まる。worktree のパスも、workspace の名前も、作ったことの記録の置き場所も、そこから決まる（3.2）。
-   削除の候補は、作ったことの記録、`origin/main` の文書、Herdr の今の workspace から決まる（3.4.1）。
-   削除の候補のうち AC-20 の条件も満たすものは、行の状態を「タスク完了」とする（3.4.1）。

### 1.5. 設計の際に決めたこと（2026-11-17）

-   **削除の提案の行（3.4.1 の AC-20 の行）は、[0093](../0093_chief_status_sheet/02_architecture.md) の部品のうち `main` にあるものに 1 行ずつ足す（申し送りの H-09）。** 0093 は、状況の表の各行に急ぎの度合い・番・進行の位置を示し、行を急ぎの度合いの順に並べる。0093 の要件書は、削除の提案の行を「🟡 あとで」・人・文書「なし」とし、「あとで」の位置に並べると定める（0093 の AC-05、AC-07 の規則 2、AC-15）。0093 の 3 つの部品ごとに、次のとおりにする。
    -   **急ぎの度合いの対応表（0093 設計書 3.2）。** 送信が止まった行（順 1）と質問の行（順 2）の後、使っていないか残った worktree の行（順 3）の前に、削除の提案の行を「あとで」・人とする行を足す。削除の提案の行は質問で止まっていない（AC-20）が、送信が止まっていることはありうる。そのときは順 1 の「すぐ」・人が勝つ。
    -   **文書の規則（0093 設計書 3.3）。** 規則 2 として、削除の提案の行を文書「なし」とする。規則 1（fetch の失敗か文書を読めない）が先に当たる。
    -   **並べ方（0093 設計書 3.7）。** 何も足さない。削除の提案の行は `Row.Idle` でない（3.4.1）ので、0093 の並べ方が、急ぎの度合いの「あとで」の位置に並べる。
-   **行が削除の提案の行であることは、3.6 で `Row` に足す印で宣言する。印は AC-20 の行にだけ付け、AC-23 だけを満たす行（エージェントが動いている、HEAD がブランチにある）には付けない。0093 の対応表と文書の規則は、この印を読む。** AC-23 だけを満たす行は、削除の項目を持つが、急ぎの度合いも文書も 0093 の通常の規則のままである（0093 の要件書 3.1 の表は、この行を「タスク完了で、削除の提案があるもの」に限る）。行の状態や文字列から推し量らない（CLAUDE.md の「Declare, don't infer」）。
-   **何も足さないときの行。** 削除の提案の行は「タスク完了」で `Row.Idle` でないので、0093 の対応表の順 6 で「🟢 待つ」・番なし、文書は規則 10 の「要確認」になり、使っていないか残った worktree の行の前の「待つ」の位置に並ぶ。0093 の要件書に合わない。
-   **どちらが先に入るか。** 0093 と本タスクは並行して進むので、順を固定しない。本タスクの計画書は、削除の提案の行を作る PR と、その後の本タスクのすべての PR を作る前に、0093 の部品ごとに「`main` にあり、削除の提案の行がまだない」かを確かめ、当たるものに足す。0093 の計画書も、自分の PR の前と、そのマージの前に、本タスクの印が `main` にあるかを確かめる（0093 設計書の 1.2・3.12）。残る危険は、0093 の PR が、本タスクの印が `main` に入る前に確かめ、本タスクの最後の PR の後にマージされる場合である。このときはどちらも足さない。本タスクの最後の PR の記録に確かめの結果を書き、足りなければ人に示す。0093 が先に入るのを待ってから削除の提案を実装する案は、2 つのタスクの進みを結び付けるので採らない。ADR: なし（表示の細部で、0093 との実装の順に応じた扱いはタスクの中で閉じるため）。
-   **`--protocol` の番号は、本タスクの道具の出力とサブコマンドを足す PR の時点の `origin/main` の値より 1 大きくする。** 0093 も、出力を変えるので `--protocol` を 1 から 2 に上げる（0093 設計書の 1.2）。本書の「2」は、0093 の実装がまだ入っていないときの値で、0093 が先に入って 2 にしていれば 3 になる。番号を上げる目的（古い定義と新しい道具の組み合わせを止めること）は、どちらの順でも同じである（3.1）。ADR: なし（番号の値はタスクの中で閉じるため）。

## 2. システム構成 (System Structure)

### 2.1. 全体構成

図 2 は、本書が変えるものと、ほかの道具との関係を示す。矢印 A → B は「A が B を呼ぶ、または読む・書く」を表す。点線の矢印は、人の操作を表す。

```mermaid
flowchart TB
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef newpkg fill:#ffe8f5,stroke:#d946ef,stroke-width:2px,color:#701a75;

    HUMAN["人"]
    CC["Claude Code の許可の仕組み"]
    SET[(".claude/settings.json")]
    SESSION["chief のセッション"]
    DEF["/chief の定義"]
    CMD["cmd/chief"]
    CORE["internal/chief"]
    SJ["internal/strictjson"]
    RUN["internal/cmdrunner"]
    HERDR["herdr CLI"]
    GIT["git"]
    GH["gh"]
    STATE[("状態のディレクトリ<br>(作ったことの記録を含む)")]
    DOCS[("origin/main のタスクの文書")]
    NEWWS["作った workspace"]

    HUMAN -.->|"選ぶ・答える"| SESSION
    HUMAN -.->|"削除の許可"| CC
    SESSION --> DEF
    SESSION --> CC
    CC --> SET
    CC --> CMD
    CMD --> CORE
    CORE --> SJ
    CORE --> RUN
    CORE --> STATE
    RUN --> HERDR
    RUN --> GIT
    RUN --> GH
    GIT --> DOCS
    HERDR --> NEWWS

    class HUMAN,RUN,HERDR,GIT,GH,CC process
    class SESSION,DEF,CMD,CORE,SJ enhanced
    class NEWWS newpkg
    class STATE,DOCS,SET data

    subgraph Legend["Legend"]
        L1[("データ")]
        L2["既存のもの (変えない)"]
        L3["変えるもの"]
        L4["本タスクで作るもの"]
    end
    class L1 data
    class L2 process
    class L3 enhanced
    class L4 newpkg
```

-   道具は、外部のコマンドを 0088 と同じく `internal/cmdrunner` でシェルを通さずに走らせる（`internal/chief/runner.go`）。`herdr agent start` は起動を待つので、10 秒の `herdrTimeout`（`internal/chief/runner.go:38`）より長い上限を、その呼び出しにだけ使う。
-   herdr の呼び出しは、0088 と同じく、chief の pane が引き継いだ環境（Herdr のソケット）により、chief が動いている Herdr のセッションに届く。`--session` などの引数は付けない（AC-09）。
-   `.claude/settings.json` は、Claude Code の許可の仕組みが読む設定で、本書で `deny` と `ask` の規則を足す（1.2）。

### 2.2. コンポーネント配置

| ファイル | 種類 | 責務 |
|---|---|---|
| `cmd/chief/main.go` | 変更 | サブコマンド `targets`・`create`・`remove` とその引数を足す。パッケージのコメントのサブコマンドの一覧を直す（3.1） |
| `internal/chief/records.go` | 変更 | `Protocol` を `2` にする（`internal/chief/records.go:28`）。作ったことの記録を作る・読む・消す操作と、workspace ごとの記録のディレクトリを空にする操作を足す。`Worktree` の型のコメントに、作る前の正規のパスの作り方を足す（3.2、3.5） |
| `internal/chief/taskspace.go` | 新規 | 作る場所（`TaskSpace`）と、パスの workspace の読み取り（3.2、3.3.2） |
| `internal/chief/issue.go` | 新規 | issue の読み取り（3.3.1） |
| `internal/chief/create.go` | 新規 | `targets`・`create`、workspace を作る部品（`WorkspaceMaker`）（3.3） |
| `internal/chief/removal.go` | 新規 | 削除の候補、削除の確かめ、`remove`、workspace を閉じる部品（`WorkspaceCloser`）（3.4） |
| `internal/chief/herdr.go` | 変更 | `herdr workspace create`・`herdr tab create`・`herdr tab rename`・`herdr agent start`・`herdr pane list`・`herdr pane process-info`・`herdr workspace close` の出力とエラーの応答を型に読む部品を足す。`PaneWriter` のコメントを「pane へ入力を送る唯一の部品」に直す（3.6） |
| `internal/chief/runner.go` | 変更 | `herdr agent start` の長い上限と、`create`・`remove` の全体の時間の上限（3.1） |
| `internal/chief/send.go` | 変更 | 作った workspace へ送る入口 `sendAfterCreate`（3.3.4）。0088 の `send` と自動送信の振る舞いは変えない |
| `internal/chief/workspace.go` | 変更 | `git worktree add --detach`・`git worktree remove`・`git worktree list --porcelain`・`git merge-base --is-ancestor` の呼び出しを足す |
| `internal/chief/task.go` | 変更 | `origin/main` の要件定義書の `対応 issue:` の行を読み、issue 番号からタスクのディレクトリを探す（3.4.4）。タスクのディレクトリの名前から文書を読む口を足す（今の `Tasks.Read` はブランチ名から読む。`internal/chief/task.go:786`） |
| `internal/chief/state.go`・`snapshot.go`・`pending.go`・`output.go` | 変更 | 削除の候補の行（AC-20）、人の判断を待つものの種類 `removal`（AC-22）、出力の項目（3.6）。`Row.Task` のコメントを、削除の候補の行では記録した issue のタスクであると直す |
| `internal/chief/tick.go`・`autosend.go` | 変更 | 作成の排他を持たれている周では自動送信をしない（3.1）。削除の項目は自動送信の対象にしない（AC-26） |
| `internal/chief/testdata/` | 変更 | Herdr の新しい出力と gh の GraphQL の応答のサンプル、起動したばかりの Claude Code の画面のサンプル（7.3） |
| `internal/strictjson/strictjson.go` | 変更 | 読むメンバーと大文字と小文字だけが違う名前のメンバーを拒む、新しい読み方を足す（3.3.1）。今の `Collect`・`CollectOnly` の振る舞いは変えない |
| `.claude/commands/chief.md` | 変更 | 新しいタスクを始める問いと、削除の提案と人の確認の問い（3.7）。`--protocol 2` |
| `.claude/settings.json` | 変更 | 拒否を足し、`ask` に `Bash(chief remove *)` を置く（1.2、3.8） |
| `CLAUDE.md` | 変更 | 「Tool Execution Safety」に issue の読み取りを足す。「Go Idioms」の task reader の例外に `対応 issue: ` を足す。「Claude Commands」の `/chief`（3.8） |
| `docs/dev/guides/chief_guide.md` | 変更 | 1〜8 章（3.8） |
| `docs/dev/conventions/package_reference.md` | 変更 | `cmd/chief`・`internal/chief`・`internal/strictjson` の行 |
| `docs/dev/adr/0046-chief-of-staff.md`・`docs/dev/adr/0014-deterministic-checks-in-go.md` | 変更（本書の PR） | 1.2 の決定 |
| `cmd/chief/docs_test.go`・`cmd/chief/main_test.go` | 変更 | 拒否・`ask`・プロトコル番号・サブコマンドの一覧（3.9） |

### 2.3. データフロー

図 3 は、人が issue 番号を答えてから `/mkreq` が送られるまでを示す。矢印は呼び出しと応答を表す。

```mermaid
sequenceDiagram
    participant H as 人
    participant S as chief のセッション
    participant T as 道具
    participant GH as gh
    participant G as git
    participant HD as herdr

    H->>S: 「新しいタスクを始める」を選び、issue 番号を答える
    S->>T: chief targets --issue N --protocol 2
    T-->>S: 番号の形の結果、既存の候補の workspace
    S->>H: 始める先を尋ねる (AskUserQuestion)
    H-->>S: 新しい workspace
    S->>T: chief create --issue N --protocol 2
    T->>GH: gh api graphql (issueOrPullRequest)
    T->>G: git fetch origin main、作る先がないことの確かめ
    T->>G: git worktree add --detach
    T->>T: 前の記録を消し、作ったことの記録を作る
    T->>HD: workspace create、tab rename、tab create (2 回)
    T->>HD: agent start (OpenCode、Claude Code)
    T->>HD: /clear、/model opus、/mkreq N (0088 の送信)
    T-->>S: 作ったもの、題名、/mkreq の送信の結果
    S->>H: 結果を示す
```

既存の workspace を選んだときは、`create` の代わりに 0088 の `chief send --workspace <W> --command mkreq --issue <N> --protocol 2` を呼ぶ（AC-04）。シーケンス図は色分けしたノードを持たないので、凡例を置かない。

## 3. コンポーネント設計 (Component Design)

### 3.1. サブコマンドと終了コード

道具の引数・出力・終了コードは、`/chief` の定義とガードのテストが読む契約である（ADR-0014）。サブコマンドと出力が変わるので、`--protocol` を `1` から `2` に上げる（申し送りの H-08）。0093 の実装が先に入って 2 にしていれば、3 に上げる（1.5）。0088 のサブコマンドの引数と振る舞いは変えない。

| サブコマンド | 何をするか | Herdr・git への書き込み | ネットワーク |
|---|---|---|---|
| `targets --issue <N>` | 番号の形を確かめ（AC-06）、既存の候補の workspace（0088 の AC-38 を満たすもの）を返す（AC-02） | なし | `git fetch origin main`（0088 の周と同じ読み取り） |
| `create --issue <N>` | issue を確かめ（F-002）、worktree と workspace を作ってエージェントを起動し（F-003）、`/mkreq <N>` を送る（F-004） | worktree を足す、workspace とタブを作る、エージェントを起動する、0088 の F-006 の表の入力 | `gh api graphql`（読み取り）、`git fetch origin main` |
| `remove --workspace <W>` | 削除の確かめを行い（AC-28）、通れば workspace を閉じて worktree を消す（AC-30） | workspace を閉じる、worktree を消す | `git fetch origin main` |

-   **`--issue` の値の扱い。** `targets` と `create` は、`--issue` の値を文字列のまま受け取り、道具の中で AC-06 の形を確かめる。形が違うときは、使い方の誤り（終了コード 2）にせず、終了コード 0 で、作らなかったことと理由 `issue_form` を返す。人が打った値の誤りは、`/chief` の定義の誤りと区別して、人に示すためである。0088 の `send --issue` は、`targets` で確かめた値を受けるので、今のまま使い方の誤りとする（`internal/chief/request.go:107-110`）。
-   **`--workspace` の値（`remove`）。** 0088 の `locate`（`internal/chief/request.go:128-175`）で、この呼び出しで読み直したタスクの workspace の正規のパスと比べ、対象の workspace を決める。一致しなければ、何もせずに `no_workspace` を返す。`locate` の契約のとおり、セッションが渡したパスでは git を走らせない。git は、一致したタスクの workspace の worktree でだけ走らせる。
-   **ロック。** 3 つとも、0088 のほかのサブコマンドと同じく、呼んだ pane とセッションが chief のロックの持ち主であることを確かめてから動く（`cmd/chief/main.go` の `requireOwner`）。
-   **終了コードと出力。** 0088 の 4 つの値の意味を変えない。終了コード 3 と出力の `outcome` `refused` は、0088 と同じく、道具が何もせずに拒んだこと（ロック、`--protocol` など）だけを表す。作らなかった判断、消さなかった判断、途中で止まったこと（AC-12・AC-32）は、終了コード 0 で返す。出力の `create`・`remove` の結果は、作らなかった理由（`create_refusal`）、消さなかった理由（`remove_refusal`）、止まった段を持つ（3.6、4.1）。
-   **時間の上限。** `create` と `remove` は、道具の中で全体の時間の上限を持ち、上限を過ぎたら、そこを失敗した段として止まり、出力を書く（AC-12・AC-32）。上限は、Claude Code の Bash の呼び出しに許される最大の時間（10 分）より短くし、値は計画で決める。`/chief` の定義は、この 2 つの呼び出しに Bash の上限を明示して渡す。Bash の上限で道具が殺されると、作ったものを示す出力が失われるためである。
-   **作成と周の関係（AC-13）。** `create` は、作り始める前に状態のディレクトリの作成の排他（`internal/cachelock` の flock）を取り、終えるまで持つ。`tick` は、自動送信を始める前に同じ排他を取り、自動送信を終えるまで持つ。取れない周では自動送信をせず、そのことを出力の問題に示す。`create` は、`tick` がこの排他を持っている間は `busy` で拒む。こうして、確かめてから送るまでの間に作成が始まることはない。`/chief` の定義も、`create` を前景で呼び、終わるまで次の周を呼ばない。そのため、作成の間は、ほかの workspace にも chief の workspace にも何も送られない。道具の `create` が Herdr に書くのは、自分が作った workspace と、そのタブと pane だけである（3.3.3、3.6 の `WorkspaceMaker`）。

### 3.2. 作る場所

worktree のパスと workspace の名前の規則は、次の型 1 つに置く（要件書 4.5）。

```go
// TaskSpace is where chief makes a task's worktree and workspace for one
// issue (requirements F-003): the path $HOME/.herdr/worktrees/yt2column/issue-<N>
// and the label issue-<N>. Only NewTaskSpace makes one, from an issue number
// and the home directory, so no caller string reaches either value.
type TaskSpace struct {
	issue paneinput.IssueNumber
	path  string // as built from $HOME, before links are resolved
	label string
}

// NewTaskSpace returns the space of issue under home, which must be absolute.
func NewTaskSpace(home string, issue paneinput.IssueNumber) (TaskSpace, error)

// Worktree returns the canonical path the worktree will have: the nearest
// existing ancestor with its links resolved, joined with the rest of the
// path. Contract: once
// the worktree exists, CanonicalWorktree on it returns the same Worktree, or
// the creation stops at that stage.
func (s TaskSpace) Worktree() (Worktree, error)
```

-   **パス。** `$HOME`（`Env.Home`。絶対パスでなければ作らない）に、`.herdr/worktrees/yt2column/issue-<N>` を足したものである。リポジトリの名前 `yt2column` は、要件書で決めた値で、リポジトリの remote から推し量らない。
-   **正規のパス。** 0088 の `Worktree` は `CanonicalWorktree` だけが作り、それは既にある worktree で `git rev-parse` を走らせる（`internal/chief/records.go:519-548`）。作る前は worktree がないので、`TaskSpace.Worktree` を 2 つめの作り方として足し、`Worktree` の型のコメントに書く。作った後に `CanonicalWorktree` で読み直し、同じでなければ、作成の段の失敗として止める（AC-12）。作ったことの記録と workspace ごとの記録の置き場所は、この正規のパスの SHA-256 で決まる（`internal/chief/records.go:551`）。
-   **送れない文字（AC-11）。** パスが、0088 の transcript の規則の文字（`internal/chief/transcript.go:45` の `transcriptDirChars`。英数字・`/`・`.`・`-`）だけでできていなければ、作らずに `unsupported_path` で拒む。組み立てたパスと正規のパスの両方に当てる。Claude Code は、解決した作業ディレクトリから transcript の場所を求めるためである。
-   **親のディレクトリ。** 道具は親のディレクトリ（`$HOME/.herdr/worktrees/yt2column`）を自分では作らない。ないときは、`git worktree add` が作る。`TaskSpace.Worktree` は、パスのうち今ある最も深い祖先のリンクを解決し、残りをつないで正規のパスとする。その途中にあるものがディレクトリでないとき（ファイルなど）は、作らずに `target_exists` で拒む。

### 3.3. 作成（`targets`・`create`）

#### 3.3.1. issue の読み取り（F-002）

道具は、`create` の中で、何かを作る前に、chief の worktree で `gh api graphql` を呼ぶ（1.2）。問い合わせは次の形で、`<N>` は検証した番号である。`{owner}`・`{repo}` は gh が chief の worktree の remote から埋める。

```graphql
query($owner: String!, $repo: String!, $n: Int!) {
  repository(owner: $owner, name: $repo) {
    issueOrPullRequest(number: $n) {
      __typename
      ... on Issue { number state title }
      ... on PullRequest { number }
    }
  }
}
```

応答は、要件書 4.2 の表の形（上限 64 KiB、UTF-8、1 つの JSON のオブジェクト、後に続くデータがない）でなければ `issue_unreadable` とする。そのうえで、次の 3 つの形だけを受け付ける。

| gh の終了コード | 応答の形 | 結果 |
|---|---|---|
| 0 | `errors` がなく、`issueOrPullRequest` が `__typename` `Issue` のオブジェクトで、`number` が求めた番号、`state` が `OPEN` か `CLOSED`、`title` が文字列 | `OPEN` なら作成に進む。`CLOSED` なら `issue_closed` |
| 0 | `errors` がなく、`issueOrPullRequest` が `__typename` `PullRequest` のオブジェクトで、`number` が求めた番号 | `issue_is_pr` |
| 1 | `issueOrPullRequest` が `null` で、`errors` がちょうど 1 つ、その `type` が `NOT_FOUND`、`path` が `["repository", "issueOrPullRequest"]` | `issue_not_found` |

-   ほかの形はすべて `issue_unreadable` とする。読むメンバーの重なり（`strictjson.Object.Collect` が拒む。`internal/strictjson/strictjson.go:91`）、名前が読むメンバーと大文字と小文字だけ違うメンバー（`Title` など）、`null`・違う型・知らない `__typename` を含む。大文字と小文字だけ違う名前は今の `strictjson` が拒まないので、拒む読み方を `strictjson` の新しいメソッドとして足す。今の `Collect`・`CollectOnly` を使うほかの読み手の振る舞いは変えない。
-   振る舞いは、GitHub が宣言する値である `errors` の `type` で選び、`message` の文では選ばない。`message` は読まない。
-   **題名。** 題名は、0088 の無害化（`Sanitize`）で制御文字と見えない文字を見える形に置き換え、上限の文字数（計画で決める）で切る。切ったときは、切ったことを出力に示す。題名は、パス、名前、送る入力のどれも左右しない（AC-07）。

#### 3.3.2. 作る前の確かめ

`create` は、次の順に確かめ、外れたところで何も作らずに拒む。理由の綴りは 4.1 にある。

1.  **番号の形（AC-06）。** `paneinput.ParseIssueNumber`（`internal/chief/paneinput/paneinput.go:66`）で確かめる。0088 の `/mkreq` の issue 番号と同じ規則である。
2.  **作る場所（3.2）。** `$HOME` が絶対パスであること、送れない文字がないこと。
3.  **issue（3.3.1）。**
4.  **fetch。** chief の worktree で `git fetch origin main` を行い、`git rev-parse refs/remotes/origin/main` でコミットを読む。fetch が失敗すれば `fetch_failed` で拒む（AC-11）。
5.  **排他。** 状態のディレクトリの作成の排他（3.1）と、作るパスの workspace ごとの排他（0088 の `internal/cachelock` の flock。`internal/chief/records.go:642` の `Lock`）を取る。取れなければ `busy` で拒む。同じ issue で 2 つの `create` が同時に走っても、作るのは 1 つだけである（要件書 4.3）。workspace ごとの排他は `/mkreq` の送信を終えるまで持ち続ける。そのため、作成の途中に、0088 の `send` がその workspace へ送ることもない。
6.  **作る先がないこと（AC-11）。** 排他を持ったまま、次の 3 つを確かめる。どれかに当たれば、当たったものを示して `target_exists` で拒む。
    -   パスに、ファイル、ディレクトリ、シンボリックリンクのどれかがある（`os.Lstat`）。
    -   `git worktree list --porcelain` に、そのパス（正規のパスで比べる）の worktree がある。ディレクトリがなく、登録だけが残っている worktree も含む。
    -   パスの workspace がある。

**パスの workspace の読み方。** `herdr pane list` のすべての pane のうち、作業ディレクトリ（`cwd`）がそのパスかその下にあるものを持つ workspace を、パスの workspace とする。chief の workspace も、シェルだけの pane も数える。`herdr pane list` が pane ごとに `cwd` と `workspace_id` を返すことは、2026-11-16 にこの workspace で確かめた。この読み方は、作る前の確かめ（AC-11）、削除の候補（AC-23。3.4.1）、削除の確かめ（3.4.2）の 3 か所で同じものを使う。0088 のタスクの workspace の見つけ方（エージェントの作業ディレクトリ。`internal/chief/workspace.go:175-230`）は、行を作るためのもので、本書では変えない。

#### 3.3.3. 作る段

確かめを通ったら、次の段の順に作る。どの段も、Herdr か git が返した ID だけを使い、ほかの workspace の ID を引数に取らない（AC-13）。Herdr への書き込みは、`WorkspaceMaker`（3.6）だけが行う。

| 段 | 呼び出し | 失敗のときに残るもの |
|---|---|---|
| `records` | そのパスの workspace ごとの記録のディレクトリに残る記録（前の使い方の作ったことの記録、送信の記録、送信の停止、PR の詳細の控え）を、排他を持ったまま消す。排他のファイルは消さない（3.5） | 消せなかった記録 |
| `worktree` | chief の worktree で `git worktree add --detach <パス> <4 で読んだコミット>`。ブランチは作らない（AC-08）。その後、正規のパスが `TaskSpace.Worktree` と同じことを確かめる | git が途中で止めたもの（ディレクトリが残ることがある） |
| `record` | 作ったことの記録を作る（3.5、AC-14） | worktree |
| `workspace` | `herdr workspace create --cwd <パス> --label issue-<N> --no-focus`。返った workspace の ID と、最初のタブとその pane の ID を読む | worktree、記録 |
| `tabs` | 最初のタブを `herdr tab rename` で `cmd` にする。`herdr tab create --workspace <ID> --cwd <パス> --label opencode --no-focus` と、同じく `--label claude` のタブを作る | 上に加えて、作った workspace とタブ |
| `opencode` | `opencode` のタブの pane で、シェルが前面にあることを `herdr pane process-info` で確かめてから、`herdr agent start issue-<N>-opencode --kind opencode --pane <ID>` を、エージェントの引数なしに呼ぶ（AC-10） | 上に加えて、起動したエージェント |
| `claude` | `claude` のタブの pane で同じく、`herdr agent start issue-<N>-claude --kind claude --pane <ID>` | 同上 |
| `panes` | `herdr agent list` を読み直し、作った workspace に Claude Code と OpenCode の pane がちょうど 1 つずつあり、どちらの作業ディレクトリも worktree のパスそのもの（0088 の送り先の確かめ `inWorktree` と同じ。`internal/chief/send.go:387`）であることを確かめる（AC-09） | 同上 |

-   **途中の失敗（AC-12）。** どの段で失敗しても、そこで止まり、作ったもの（worktree のパス、記録、workspace の ID と名前、タブと pane、起動したエージェント）と、失敗した段と理由を返す。作ったものは消さず、`/mkreq` を送らない。人は、示されたものを端末で片付けるか、残す。残した workspace は、`/mkreq` が送られずに detached のままなので、0088 の規則で「要確認」の行になる。
-   **エージェントの名前。** `herdr agent start` の名前は、生きているエージェントの間で重複してはならない（Herdr の skill の説明）。`issue-<N>-opencode`・`issue-<N>-claude` は、Herdr の名前の形（`[a-z][a-z0-9_-]{0,31}`）に収まる。同じ名前のエージェントがすでにあれば、その段の失敗として止まる。
-   **段の順。** OpenCode を先に起動するのは、`/mkreq` の送り先の Claude Code の起動を最後にして、その起動から送信までの間を短くするためである。

#### 3.3.4. エージェントの起動と `/mkreq` の送信（F-004）

-   **起動の待ち（AC-16）。** `herdr agent start` は、エージェントが入力を受け付ける状態になるまで待ち、待つ時間の上限を `--timeout` で取る（Herdr の skill の説明。上限は 3 秒より長く 300 秒以下）。道具は、上限を 60 秒とする（計画で変えうる）。
-   **起動の結果の読み方。** 成功の応答のほか、Herdr のエラーの応答（標準エラーの JSON、`{"error":{"code":…,"message":…}}`。`herdr api schema` の `error_response`）を読む。エラーの応答のときは、`herdr agent get <pane>` でその pane にエージェントがいるかを確かめる。
    -   エージェントがいる（`agent_not_ready`、時間切れなど）: 作成の段の失敗とはしない。作成を終えたものとし、`/mkreq` は送らない（下の「送らないとき」）。
    -   エージェントがいないか、読めない: その段の失敗（AC-12）。
    -   `code` は Herdr が宣言する値で、示すことにだけ使う。振る舞いは、`agent get` で読んだエージェントの有無と状態で決める。`message` は示すだけに使う。
-   **`/mkreq` の送信（AC-15）。** 作成の段を終え、Claude Code の起動が成功したときは、`sendAfterCreate` で `/mkreq <N>` を送る。
    -   送り先の workspace は、0088 の `FindTaskWorkspaces`（`internal/chief/workspace.go:183`）を読み直し、作った workspace の ID のものを使う。0088 の行と同じ識別になる。
    -   その workspace の排他は、作成から持ち続けたものを使う。0088 の `holdWorkspace`（`internal/chief/request.go:258-298`）は自分で排他を取るので、すでに持っている排他を受け取る入口を足し、`holdWorkspace` と同じく、終わりのない送信を送信の停止に変え、送信の停止があれば送らない。作る前に記録を消している（3.3.3 の `records`）ので、前の使い方の停止は残っていない。
    -   コマンドは `paneinput.NewMkreq` で検証した番号から組み立て、送り先は `_context.md` の送り先の表の `task-documents` の行（Claude Code・`opus`）である。送り先の pane の選び方、入力ごとの確かめ、`/clear` と `/model opus` とその確かめは、0088 の `sendCommand` のままである。
    -   0088 の `Send` から省くのは、`offered` による行の状態の確かめ（`internal/chief/request.go:314-342`）だけである。作った直後の行は、HEAD が detached なので「要確認」になるためである（1.3）。
-   **送らないとき（AC-16・AC-17）。** 次のときは何も送らず、送らなかったことと理由を返す。どの場合も、人がその pane で `/mkreq <N>` を打つよう示す。
    -   Claude Code の起動がエラーの応答で終わったが、エージェントはいるとき: 理由は `agent_not_ready` とし、`agent get` で読んだ状態を添える。状態が `blocked` なら、その pane の画面で答えるよう示す。フォルダの信頼を問う画面は、Herdr が `blocked` と報告すればここに入る。`unknown` などそのほかの状態なら、その pane を確かめるよう示す。この理由は `create` の出力だけが持ち、0088 の `send` と自動送信の理由の一覧は変えない。
    -   `sendAfterCreate` の確かめが外れたとき: 0088 の理由（`not_accepting`・`input_left`・`unsupported_path` など）をそのまま返す。
-   **送った後の失敗（AC-18）。** 最初の入力を送った後に確かめが外れたときは、0088 のとおり、送り直さずにその workspace の送信の停止の記録を作る（0088 設計書の 3.1.3）。
-   **後の周（AC-17・AC-19）。** 送れなかった workspace は HEAD が detached のままなので、0088 の規則で「要確認」になり、`/mkreq` は提案されない（`internal/chief/state.go:256-263`）。本書は、この規則を変えない。`/mkreq` がタスクのブランチを作った後は、0088 の規則でそのタスクの行になる。

#### 3.3.5. `targets`（AC-01〜AC-04）

-   `targets` は、番号の形を確かめ、0088 の周の読み取り（`internal/chief/snapshot.go:90` の `readRound`）で、行が `/mkreq` を提案している workspace（0088 の AC-38 を満たすもの）を返す。何も作らず、通知の記録も作らない。
-   issue の存在は確かめない。既存の workspace を選んだときは F-002 の確かめを行わない（AC-04）。新しい workspace を選んだときは、`create` が確かめる。
-   既存の workspace を選んだときは、セッションが 0088 の `chief send --workspace <W> --command mkreq --issue <N> --protocol 2` を呼ぶ。その振る舞いは 0088 のままである（AC-03・AC-04）。

### 3.4. 削除の候補と削除（`remove`）

#### 3.4.1. 削除の候補と行（F-005・F-006）

`snapshot` と `tick` は、0088 の周の読み取りの後に、タスクの workspace ごとに次を確かめる。0088 の周がすでに fetch した `origin/main` を使う。

| AC-23 の条件 | 道具が確かめる値 |
|---|---|
| chief が作ったと記録から読める | そのタスクの workspace の正規のパスの記録のディレクトリに、作ったことの記録があり、決まった形で読め、次の 2 つが成り立つ。(a) 記録のパスが、その正規のパスと等しい。(b) 記録の issue 番号から作った `TaskSpace` の `Worktree` が、その正規のパスと等しい |
| 記録と同じパスを開いたまま Herdr にある | 記録のパスを対象とする、パスの workspace（3.3.2）がちょうど 1 つで、それがこのタスクの workspace である |
| 記録した issue のタスクがちょうど 1 つあり、完了している | 3.4.4 の読み方で、記録した issue 番号を `対応 issue` とするタスクのディレクトリが `origin/main` にちょうど 1 つあり、その計画書が `approved` で、すべての PR がマージされている（`internal/chief/task.go:693` の `Plan.Done`。0088 の「タスク完了」と同じ判定） |

-   **記録のパスと issue の結び付き。** 記録の issue 番号から作る場所を作り直し、それが記録のパスと同じであることを確かめる。偽られた記録が名指せるのは、`issue-<N>` の形のパスの worktree だけになり、本体のチェックアウトや人が手で作った worktree を名指せない。
-   **行（AC-20・AC-21）。** 上の条件をすべて満たし、HEAD が detached で、どのエージェントも実行中でも質問で止まってもいない（0088 の `classify` の見分け。`internal/chief/state.go`）行は、状態を「タスク完了」、タスクの欄を記録した issue のタスクの番号、次にやることを「削除を提案」とする。どれかが外れる行は、0088 の規則のままである。この行は、0088 の「使っていないか残った worktree の行」（`Row.Idle`）にはせず、0088 の行の並べ方の「タスク完了」の位置に並べる（0088 設計書の 3.1.4）。0093 の急ぎの度合いの対応表・文書の規則・並べ方が `main` にあれば、この行の急ぎの度合いを「あとで」、番を「人」、進行の位置の文書を「なし」とし、「あとで」の位置に並べる（1.5）。
-   **0088 の規則の例外。** これは、0088 設計書 3.1.4 の状態の区分（HEAD が detached の行を「要確認」にする規則。`internal/chief/state.go:256-263`、`internal/chief/state.go:430-434`）の例外である。例外にするのは、作ったことの記録と `origin/main` の文書から、タスクが完了したことを確かめられる場合に限るためである。その規則を確かめる今のテスト（`internal/chief/snapshot_test.go:103` の `TestTakeSnapshotStates` の detached の行、`internal/chief/snapshot_test.go:467` の `TestRowOrderRules`、`internal/chief/snapshot_test.go:508` の `TestDetachedRowIdleOnlyWhenEverythingWasRead`、`internal/chief/autosend_test.go:343` の `TestAutoSendDetached`）は、作ったことの記録のない workspace について、変わらずに成り立つ。例外を確かめるテストを足す（7.1）。
-   **人の判断を待つもの（AC-22〜AC-25）。** AC-23 の条件を満たす workspace ごとに、新しい種類 `removal` の項目を 1 つ作る。項目は、workspace の名前、worktree のパス、issue の番号、タスクの番号、workspace の pane の数（`herdr pane list --workspace <ID>`）、その周の削除の確かめの結果（3.4.2）を持つ。HEAD が detached でないときやエージェントが動いているときも項目は作り、確かめの結果に示す（AC-20 は行の示し方の条件で、提案の条件は AC-23 である）。
-   **並べ方（AC-22）。** 種類の順を、0088 の 4 つ（`internal/chief/pending.go:167` の `kindOrder`）の後に `removal` を足したものにする。同じ種類の中では、0088 のとおり最初に見つけた時刻の古いものから並べる。「新しいタスクを始める」は、`/chief` の定義が一覧の最後に置く（3.7）。
-   **削除しないこと（AC-26）。** `tick` の自動送信は、削除の項目を送信の対象にしない。削除は `remove` だけが行う。
-   **エージェントのいない workspace。** 0088 の見つけ方はエージェントの作業ディレクトリから workspace を見つけるので、エージェントがすべて終了した workspace は、表に出ず、削除も提案されない。人が端末で片付ける（手引きの既知の制限に書く）。

#### 3.4.2. 削除の確かめ（AC-28）

| 確かめ | 道具が確かめる値 | 理由の綴り |
|---|---|---|
| AC-23 の条件がまだ満たされる | 3.4.1 の表を、`remove` の中で fetch した後に確かめ直す | `not_candidate` |
| HEAD が detached である | `git rev-parse --abbrev-ref HEAD` が `HEAD`（0088 の `GitReader.Branch`）。AC-20 の行の条件にそろえ、完了した後に人が同じ worktree で別のブランチを始めていれば消さない | `head_on_branch` |
| 変更がない | `git status --porcelain --untracked-files=all` が空（0088 の `GitReader.Clean`。`internal/chief/workspace.go:242`）。git が無視するファイルは数えない | `not_clean` |
| HEAD が `origin/main` から届く | `git merge-base --is-ancestor HEAD refs/remotes/origin/main` が 0 で終わる。1 なら届かない。そのほかは読めない | `head_not_on_main` |
| worktree が固定されていない | `git worktree list --porcelain` のその worktree に `locked` がない。閉じた後に git が消すことを拒まないため | `worktree_locked` |
| どのエージェントも動いていない | その workspace のどのエージェントも、Herdr の状態が `idle` か `done` で、画面が質問の形でない（0088 の F-001・F-002 の見分け）。状態と画面がすべて読める | `agent_active` |
| エージェントでない pane でシェル以外が走っていない | `herdr pane list --workspace <ID>` の、エージェントのいない各 pane について、`herdr pane process-info` の前面のプロセスグループの ID がシェルの pid と等しく、前面のプロセスがシェルだけである（1.2） | `shell_busy` |
| ほかの workspace がパスを使っていない | パスの workspace（3.3.2）が、消す workspace だけである。ほかの workspace の pane がパスの下で動いていれば、worktree を消すとその作業ディレクトリがなくなるため | `path_in_use` |

-   どの確かめも、読めないときは通らないものとし、読めなかったものの名前を添えて `unreadable` とする。
-   確かめが 1 つでも通らなければ、何も消さず、通らなかったものをすべて返す。

#### 3.4.3. 消す手順

1.  **排他。** そのパスの workspace ごとの排他を取る。取れなければ `busy` で何もしない。排他は、記録を消すまで持ち続ける。そのため、0088 の送信と自動送信は、消している最中の workspace へ送らない。
2.  **fetch（AC-29）。** chief の worktree で `git fetch origin main` を行う。失敗すれば `fetch_failed` で何もしない。
3.  **確かめ（3.4.2）。**
4.  **workspace を閉じる。** 閉じる直前に、`herdr pane list` と `herdr agent list` を読み直し、3 のときと比べて、その workspace の ID、pane の ID の集まり、エージェントの状態が同じで、エージェントでない pane の前面のプロセスがシェルのままであることを確かめる。違えば閉じない（`changed`）。同じなら `herdr workspace close <ID>` を、`--group` を付けずに呼ぶ。閉じるのは `WorkspaceCloser`（3.6）だけである。
5.  **pane が消えるのを待つ。** `herdr pane list` で、その workspace の pane がなくなったことを、決まった回数まで確かめる。止まりかけのプロセスがファイルを書き残すと、次の段で git が拒むためである。
6.  **worktree を消す。** その直前に、HEAD が 3 のときと同じで、まだ `origin/main` から届くことを確かめ直す。違えば消さずに止まる（段 `recheck_head`）。3 の後に作られたコミットは、worktree が clean でも git が拒まないためである。通れば、chief の worktree で `git worktree remove <パス>` を、`--force` なしで行う。3 の後に変更か追跡していないファイルが入っていれば、git が拒む。git が無視するファイルは、worktree とともに消える（要件書 4.2）。
7.  **記録を消す。** そのパスの workspace ごとの記録のディレクトリの記録（送信の記録、送信の停止、PR の詳細の控え、作ったことの記録）を、作ったことの記録を最後にして消す。排他のファイルは、排他を持ったままなので消さない（3.5）。

-   **順序の理由。** workspace を先に閉じるのは、エージェントとシェルのプロセスを止めてから、その作業ディレクトリを消すためである。worktree を先に消すと、動いているプロセスの作業ディレクトリがなくなり、その間に書かれたものが行き場を失う。
-   **途中の失敗（AC-32）。** 4〜7 のどこで失敗しても、そこで止まり、残ったもの（workspace、worktree のディレクトリ、git の worktree の登録、記録）と、失敗した段と理由を返す。自分から消し直さない。6 で失敗して workspace だけが閉じた場合、その worktree は表に出ず（エージェントがいないため）、削除も提案されない。人が端末で片付ける。
-   **消さないもの（AC-31）。** `git worktree remove` は、ブランチを消さない。道具は、ブランチ、ほかの worktree、worktree の外のファイルを消さない。消す記録は、chief 自身の、そのパスの workspace ごとの記録だけである。
-   **残る隙間。** 4 の読み直しと `workspace close` の間に、人がエージェントに入力して実行が始まると、その実行は失われうる。読み直しと `workspace close` の間の隙間は、Herdr の呼び出し 1 回分である。ファイルについては、6 で git が最後に拒む。
-   **二度と提案しない（AC-33）。** 消し終えた workspace は Herdr になく、作ったことの記録もないので、表に出ず、提案されない。

#### 3.4.4. issue からタスクを見つける（申し送りの H-04）

-   **読み方。** 道具は、chief の worktree で `git grep -n '^対応 issue: ' refs/remotes/origin/main -- ':(glob)docs/tasks/*/01_requirements.md'` を 1 回呼び、すべての要件定義書の `対応 issue:` の行を得る。作ったことの記録が 1 つもない周では呼ばない。
-   **数える番号。** 各行の、最初の全角の `（` より前の部分を見る（AC-23）。`（` がなければ行の全体を見る。その部分にある `[#<数字>](<URL>)` の形の Markdown のリンクの表示文字列の `#<数字>` を、その行が挙げる issue 番号とする。
-   **安全側に倒すこと。** 行の読み方が分からない場合は、提案を減らす向きに倒す。
    -   URL がその番号の issue（`/issues/<数字>` で終わる）を指さない、表示文字列と URL の番号が違うなどの、形の合わないリンクは、表示文字列と URL の両方の番号を挙げているものとして数える。そうすると「ちょうど 1 つ」が成り立ちにくくなり、提案は減る向きにだけ働く。
    -   その部分にある、リンクの形でない `#<数字>`（閉じ括弧のない壊れたリンク、リンクでない番号）も、その行が挙げる番号として数える。読めない書き方が、同じ issue を挙げるタスクを見落とさせないためである。
    -   1 つの要件定義書に `対応 issue:` の行が 2 つ以上あるときは、どの行の番号も、その文書が挙げる番号として数える。
    -   コードのフェンスの中の行も数える。
-   **リンクでない番号。** コミット `bc41cf17` の `origin/main` では、0001〜0011・0013・0015〜0023 の要件定義書の `対応 issue:` の行が、`対応 issue: #2` のようにリンクでない番号で始まる。上の規則でこれらの番号も数えるが、それは提案を減らす向きにだけ働く。chief が作る workspace の issue は、`/mkreq` が書くリンクの形（`.claude/commands/mkreq.md:142`）の行で挙がる。
-   **`/mkreq` との関係。** `/mkreq` の重複の探し方（`.claude/commands/mkreq.md:53-55` の `git grep -l '対応 issue: \[#N\]'`）と、chief の数え方は違う。`/mkreq` は、すべてのブランチの `docs/tasks` の下のすべてのファイルを、行の頭に限らずに探し、`対応 issue: ` の直後のリンクだけを見る。chief は、`origin/main` の要件定義書の行の頭の `対応 issue: ` だけを読み、最初の `（` より前のすべての `#<数字>` を数える。chief は、2 つの探し方が同じ結果になることを前提にしない。chief がちょうど 1 つのタスクを見つけても、提案するだけで、消す前に削除の確かめ（3.4.2）と人の確認が要る。
-   **文書の読み方。** 見つけたタスクのディレクトリの文書は、0088 の `Tasks` の読み方（Status、計画書の PR マーカー）で読む。今の `Tasks.Read` はブランチ名からディレクトリを探すので、ディレクトリの名前から読む口を足し、両方が同じ読み方を使うようにする。
-   **「Go Idioms」の例外。** `対応 issue: ` の文字列は、日本語の要件定義書の行の頭と等しくなければならない。そのため、読み方は `internal/chief/task.go` に置き、CLAUDE.md の「Go Idioms」の task reader の例外の範囲に、要件定義書の `対応 issue: ` の行の頭を足す（要件書 5 章）。
-   **「Declare, don't infer」。** この行は文書の決まった欄で、計画書の PR マーカーを読むのと同じく、文書の文法を読むものである。この方針の例外にはしない（要件書 5 章）。

### 3.5. 作ったことの記録（申し送りの H-01）

| 項目 | 値 |
|---|---|
| 置き場所 | 0088 の workspace ごとの記録のディレクトリ（正規のパスの SHA-256。`internal/chief/records.go:551`）の `created.json` |
| 持つもの | worktree の正規のパス、issue 番号、作った時刻。workspace の ID は持たない（記録を作る時点ではまだなく、記録は作った後に書き換えないため。workspace はパスから見つける。3.4.1） |
| 作るとき | 作る段の `record`（`git worktree add` が成功した直後。AC-14） |
| 読むとき | `snapshot`・`tick`（3.4.1）、`remove`（3.4.2） |
| 消すとき | `create` の段 `records`（作る先がないことを確かめた後。前の使い方の記録として）と、`remove` の段 7（消し終えた後） |

-   **置き換え（AC-14）。** 0088 の記録は、`privatefile.Create` で作り、同じ名前のものを置き換えない（`internal/privatefile/privatefile.go:314`）。`create` は、そのパスの workspace ごとの排他を持ったまま、そのディレクトリに残る記録をすべて `privatefile.Remove` で消してから、新しい記録を `Create` で作る。排他の中で行うので、2 つの `create` が同じ記録を置き換え合うことはない。前の記録の issue 番号は、新しい workspace の削除の提案に使わない。作る先がない（AC-11）ことを確かめた後なので、消す記録が名指す worktree と workspace は、もうない。
-   **読めない記録（AC-24）。** 形が違う、権限が違う、記録のパスがディレクトリの名前と合わない、記録のパスが記録の issue 番号の作る場所と合わない記録は、読めないものとし、その workspace の削除を提案しない。行は 0088 の規則のままにする。
-   **記録が条件として働く範囲。** 作ったことの記録は、削除の候補を名指すことと、その行に示すタスクを探す手がかり（記録の issue 番号）にだけ使う。名指された workspace は、Herdr（3.4.1 の表の 2 行め）、git（3.4.2）、`origin/main` の文書（3.4.4）で確かめ直す。作ったことの記録は、送るコマンドと送り先を決めない。作った直後の `/mkreq` の番号は、記録からではなく、`create` の引数を検証した値から組み立てる。
-   **ADR-0014 と ADR-0046。** ADR-0014 は、道具が判定に読み返す状態のディレクトリの記録を、例外として列挙している（[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) の決定）。作ったことの記録をその一覧に足す。ADR-0046 の記録の決まり（同名のものがあれば作らず、上書きしない）に、この記録とそのディレクトリの記録を消す場面を足す。この PR で両方を更新した。

### 3.6. 出力の型と、Herdr に書く部品

道具の主な型を示す。項目の細部は計画で決める。

```go
// WorkspaceMaker makes one task's workspace in Herdr. Only NewWorkspaceMaker
// makes one, from a TaskSpace; it creates the workspace labelled with the
// space's label at the space's path, and every later call (tab rename, tab
// create, agent start) takes only an ID that its own workspace create
// returned. It never takes a workspace, tab, or pane ID from a caller.
type WorkspaceMaker struct{ /* unexported */ }

// WorkspaceCloser closes one workspace. Only a RemovalCandidate that passed
// every check of design 3.4.2 in this call makes one, so no other workspace
// (chief's own, a hand-made one) can be closed. It never passes --group.
type WorkspaceCloser struct{ /* unexported */ }

// RemovalCandidate is a created workspace that met AC-23 when read; only the
// reader of design 3.4.1 makes one.
type RemovalCandidate struct{ /* unexported */ }

const (
	PendingQuestion PendingKind = iota + 1 // 0088's kinds, unchanged
	PendingMergeConfirmation
	PendingReview
	PendingProposal
	PendingRemoval // "removal": ranks after PendingProposal (AC-22)
)

// RemovalCheck is one check before a removal (design 3.4.2). The zero value
// is no check and never passes.
type RemovalCheck int

const (
	RemovalCheckUnknown RemovalCheck = iota
	RemovalCheckCandidate  // not_candidate
	RemovalCheckDetached   // head_on_branch
	RemovalCheckClean      // not_clean
	RemovalCheckHeadOnMain // head_not_on_main
	RemovalCheckUnlocked   // worktree_locked
	RemovalCheckAgentsIdle // agent_active
	RemovalCheckShellsIdle // shell_busy
	RemovalCheckPathUnused // path_in_use
)

// CheckResult is one check's outcome. Unreadable names what could not be
// read; a check with any is not passed.
type CheckResult struct {
	Check      RemovalCheck
	Passed     bool
	Unreadable []string
}

// RemovalProposal is what a removal item shows (AC-25).
type RemovalProposal struct {
	Workspace string // canonical worktree path: the API identity
	Label     string
	Issue     paneinput.IssueNumber
	Task      paneinput.TaskID
	Panes     int
	Checks    []CheckResult
}

// Made is one thing a creation made, for the human to see or clean up.
type Made struct {
	Kind MadeKind // worktree, record, workspace, tab, pane, agent
	ID   string   // the path, or the ID Herdr returned
}

// CreateResult is the kind of a creation's outcome; only the fields of
// that kind are set. The zero value is no result and is never shown as done.
type CreateResult int

const (
	CreateResultUnknown CreateResult = iota
	CreateRefused // Refusal set; nothing was made
	CreateStopped // Stage and Made set; /mkreq not sent (AC-12)
	CreateMade    // Made and Mkreq set
)

type CreateOutcome struct {
	Result   CreateResult
	Issue    paneinput.IssueNumber
	Title    string // sanitized and capped (AC-07)
	Cut      bool
	Refusal  CreateRefusal
	Existing []string
	Made     []Made
	Stage    CreateStage
	Failure  string
	Mkreq    *ActionResult // OutcomeNotSent with agent_not_ready, or 0088's result
}

// RemoveResult is the kind of a removal's outcome, as CreateResult.
type RemoveResult int

const (
	RemoveResultUnknown RemoveResult = iota
	RemoveRefused // Refusal (and Checks) set; nothing was removed
	RemoveStopped // Stage and Left set (AC-32)
	RemoveDone
)

type RemoveOutcome struct {
	Result    RemoveResult
	Workspace string
	Refusal   RemoveRefusal
	Checks    []CheckResult
	Stage     RemoveStage
	Left      []Made
	Failure   string
}
```

-   **行の項目。** 0088 の行（`internal/chief/state.go` の `Row`）に、削除の提案の行（3.4.1 の AC-20 の行）であることを示す印を足す。AC-23 だけを満たす行には付けない。0093 の対応表と文書の規則は、この印を読む（1.5）。`snapshot`・`tick` の出力の人の判断を待つものの項目に、`removal` の種類の `RemovalProposal` を足す。
-   **不透明な値。** 項目の識別（`ItemID`）は、0088 と同じく道具が作る不透明な値で、`removal` の項目は workspace のパスと issue 番号から作る。セッションは、その中身に依らない。
-   **Herdr の出力。** `workspace create` の応答（`.result.workspace`・`.result.tab`・`.result.root_pane`）、`tab create` の応答（`.result.tab`・`.result.root_pane`）、`pane list`、`pane process-info`、`agent start` の応答とエラーの応答は、0088 の Herdr の出力と同じく `strictjson` で厳密に読み、知らない値や欠けた項目は読めないものとする（0088 設計書の 3.1.2）。形は、実装の最初のフェーズで採ったサンプルで固定する。

### 3.7. `/chief` の定義（`.claude/commands/chief.md`）

-   **入口（AC-01）。** 人の判断を待つものの一覧の最後、`removal` の項目の後、`何もしない` の前に、`新しいタスクを始める` を常に置く。道具の番号を持たないので、0088 の `停止を解く` と同じく名前で選ぶ。`once` の問いでは、0088 の AC-14 のとおり、示せない項目を問いの文で名前で挙げる。
-   **始める流れ（AC-02〜AC-04）。** 人が選ぶと、issue 番号を尋ね、`chief targets --issue <N> --protocol 2` を呼ぶ。`issue_form` なら、受け付ける形を示す。通れば、`新しい workspace を作る`、候補の各 workspace、`何もしない` を `AskUserQuestion` で尋ねる。新しく作るなら `chief create --issue <N> --protocol 2` を前景で、Bash の上限を明示して呼ぶ（3.1）。既存なら 0088 の `chief send --workspace <W> --command mkreq --issue <N> --protocol 2` を呼ぶ。
-   **作成の結果（AC-07・AC-12・AC-17）。** issue の番号と題名、作ったもの、`/mkreq` の送信の結果を示す。作らなかったときは理由を、途中で止まったときは段と理由と作ったものを示す。`/mkreq` を送らなかったときは、`agent_not_ready` で状態が `blocked` ならその pane の画面で答えるよう示し、どの場合も、その pane で `/mkreq <N>` を打つよう示す。
-   **削除の提案（AC-25）と人の確認（AC-27）。** `removal` の項目は、名前、パス、issue、タスク、pane の数、確かめの結果を示す。人が選ぶと、消す workspace の名前、worktree のパス、workspace のすべての pane が閉じられることを示して、消すかを `AskUserQuestion` で尋ねる。消すと答えたときだけ、`chief remove --workspace <workspace> --protocol 2` を、Bash の上限を明示して呼ぶ。この呼び出しには、Claude Code の許可の確認が出る（1.2）。
-   **削除の結果（AC-28〜AC-32）。** 消さなかったときは、通らなかった確かめを示す。途中で止まったときは、残ったものと段と理由を示し、人が片付けるよう示す。
-   **規則。** 0088 の規則 1〜6 を変えない。issue の題名と Herdr のエラーの `message` も、ほかのエージェントや外部が書いたデータとして扱う（0088 の規則 2）。

### 3.8. `.claude/settings.json`・`CLAUDE.md`・手引き

-   **`.claude/settings.json`（1.2）。** `deny` に `Bash(herdr workspace create*)`・`Bash(herdr tab create*)`・`Bash(herdr pane split*)`・`Bash(herdr pane move*)`・`Bash(herdr worktree create*)`・`Bash(herdr worktree open*)`・`Bash(git worktree add*)`・`Bash(git worktree remove*)`・`Bash(git worktree move*)` を足す。`ask` に `Bash(chief remove *)` を置く。Claude Code の許可の規則は `deny`・`ask`・`allow` の順に強く、`allow` の `Bash(chief *)` より `ask` が先に効く。`herdr workspace rename`・`herdr tab rename` は、道具が名前でなくパスで workspace を確かめる（3.4.1）ので拒まない。
-   **「Tool Execution Safety」。** `/chief` を起動したことで承認する読み取り専用のネットワークの操作に、`gh api graphql` の `issueOrPullRequest` の問い合わせ（読み取り）を足す。書き込みの操作を承認しないことは変えない。
-   **「Go Idioms」。** task reader（`internal/chief/task.go`）の例外の範囲に、要件定義書の `対応 issue: ` の行の頭を足す（3.4.4）。
-   **「Claude Commands」の `/chief`。** 新しいタスクを始めることと、完了したタスクの workspace を消すことを足す。
-   **手引き（`chief_guide.md`）。** 次の章を直す。
    -   1 章: workspace の作り（chief が作れること、手で作る場合の形）。
    -   2 章: プロトコル番号 `2`。
    -   3 章: 使い方。
    -   4 章: サブコマンドの表。
    -   5 章: 保証すること・しないこと（作ることと消すことの範囲、消す前の確かめ、`ask` の規則）。
    -   6 章: 拒否の表に足したものと `ask`。
    -   7 章: 記録の表（`created.json`、作るときと消すときに workspace ごとの記録を消すこと）。
    -   8 章: 既知の制限。エージェントのいない workspace は提案されないこと。作ったことの記録だけが残った worktree（作成が止まった、消す途中で止まった）は表に出ないこと。シェルの背景のジョブと止めたジョブ、Claude Code の背景の実行は確かめられず、workspace を閉じると終わること。別の workspace で `make coverage-compare` が動いている間に作るか消すと、その比較が `git worktree list` の変化で止まること（`internal/coveragecompare/userstate.go`）。

### 3.9. 既存のテストと既存の読み手への影響

| テスト | 変わること |
|---|---|
| `cmd/chief/docs_test.go` の `TestSettingsDeniesPaneInput` | 拒む規則の一覧（`deniedHerdr`）に、1.2 の herdr の規則を足す。git の規則は、別の一覧として確かめる |
| `cmd/chief/docs_test.go` の `TestSettingsAllowsTool` | `allow` はそのままである。`ask` に `Bash(chief remove *)` があることを確かめるテストを足す（信頼の境界のコマンドなので、文書のテストの例外に当たる。CLAUDE.md の「Testing Strategy」） |
| `cmd/chief/docs_test.go` の `TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`・`TestGuideValuesMatchTool` | プロトコル番号 `2`、新しいサブコマンドの呼び出しの形、手引きのサブコマンドの表 |
| `cmd/chief/main_test.go` | サブコマンドの一覧と引数 |
| `internal/chief/pending_test.go` | 種類の順に `removal` を足す |
| `internal/chief/snapshot_test.go` の `TestTakeSnapshotStates`・`TestRowOrderRules`・`TestDetachedRowIdleOnlyWhenEverythingWasRead`、`internal/chief/autosend_test.go` の `TestAutoSendDetached` | 作ったことの記録のない workspace について変わらない。例外（3.4.1）のテストを足す |

-   テストではない読み手として、`opus-review-decision.sh` が `_context.md` の「PR marker conventions」を読むが、本書はその節を変えない。

## 4. エラーハンドリング設計 (Error Handling Design)

### 4.1. 理由と段

0088 と同じく、失敗を型と番兵のエラーで区別し、エラーの文で判断しない。理由の型は、ゼロ値を「分からない」とし、ゼロ値では作らず、消さない。

```go
// CreateRefusal says why nothing was created (requirements 4.2). The zero
// value is never treated as permission to create.
type CreateRefusal int

const (
	CreateRefusalUnknown         CreateRefusal = iota
	CreateRefusalIssueForm                     // issue_form (AC-06)
	CreateRefusalIssueNotFound                 // issue_not_found
	CreateRefusalIssueClosed                   // issue_closed
	CreateRefusalIssueIsPR                     // issue_is_pr
	CreateRefusalIssueUnreadable               // issue_unreadable
	CreateRefusalTargetExists                  // target_exists (AC-11)
	CreateRefusalPathUnsupported               // unsupported_path (AC-11)
	CreateRefusalFetchFailed                   // fetch_failed (AC-11)
	CreateRefusalBusy                          // busy
)

// RemoveRefusal says why nothing was removed. The zero value is never
// treated as permission to remove.
type RemoveRefusal int

const (
	RemoveRefusalUnknown      RemoveRefusal = iota
	RemoveRefusalNoWorkspace                // no_workspace (as 0088's send)
	RemoveRefusalFetchFailed                // fetch_failed (AC-29)
	RemoveRefusalBusy                       // busy
	RemoveRefusalChecksFailed               // checks_failed: Checks says which (3.4.2)
	RemoveRefusalChanged                    // changed: the re-read before close differed
)

// CreateStage and RemoveStage name the stage a creation or a removal
// stopped at (design 3.3.3, 3.4.3). The zero value is no stage.
type CreateStage int
type RemoveStage int
```

| 出力の綴り | 意味 |
|---|---|
| `issue_form` | 番号の形が違う（AC-06） |
| `issue_not_found` | issue が存在しない |
| `issue_closed` | issue が閉じている |
| `issue_is_pr` | 番号が PR を指す |
| `issue_unreadable` | 読めないか、形が違う |
| `target_exists` | 作る先がすでにある。あるものを `existing` に挙げる |
| `unsupported_path` | 作るパスに送れない文字がある |
| `fetch_failed` | `git fetch origin main` が失敗した |
| `busy` | 作成か、同じパスへの送信か削除が、すでに動いている |
| `no_workspace` | `--workspace` が今のタスクの workspace のどれでもない |
| `checks_failed` | 削除の確かめのどれかが通らなかった。どれかは確かめの結果の綴り（3.4.2）と `unreadable` で示す |
| `changed` | 閉じる直前の読み直しで、確かめたときから変わっていた |
| `agent_not_ready` | 起動した Claude Code が入力を受け付ける状態にならなかった（`/mkreq` を送らなかった理由。`create` の出力だけが持つ） |

作成の段の綴りは `records`・`worktree`・`record`・`workspace`・`tabs`・`opencode`・`claude`・`panes`・`deadline`、削除の段の綴りは `close_workspace`・`wait_panes`・`recheck_head`・`remove_worktree`・`remove_records`・`deadline` とする。`deadline` は、全体の時間の上限を過ぎて止まったことを表す（3.1）。

### 4.2. 示し方

-   道具のエラーの文は英語で、`chief: ` で始める。人に示す日本語の文は、chief のセッションが理由と段から作る。セッションはエラーの文の中身で振る舞いを選ばない（0088 設計書の 4.2）。
-   issue の題名、Herdr のエラーの `message`、git と gh のエラーの文は、0088 の無害化を通してから出力に入れる（要件書 4.2 の「示す文字列」）。
-   途中で止まったときは、作ったものか残ったものを、パスと ID で列挙する。人が端末で片付けるときに、何を消すかを推し量らずに済むためである。

## 5. セキュリティ考慮事項 (Security Considerations)

### 5.1. 製品

N/A。chief は開発用の道具で、製品のバイナリに入らない。製品の外部コマンドの起動、API キー、キャッシュ、プロンプトや記事への信頼できないテキストの埋め込みにも触れない（要件書 4.2）。

### 5.2. 開発の道具の信頼の境界

図 4 は、本書が足す信頼の境界と、信頼できない入力の流れを示す。矢印 A → B は「A のデータが B に入る、または A が B を操作する」を表し、ラベルはその入力が何に使われるかを表す。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef problem fill:#ffe6e6,stroke:#d62728,stroke-width:2px,color:#7b0000;

    ISSUE[("issue の応答")]
    HOUT[("Herdr の出力")]
    GOUT[("git の出力")]
    REC[("作ったことの記録")]
    DOCS[("origin/main の要件定義書と計画書")]
    TOOL["道具"]
    SESSION["chief のセッション"]
    PERM["Claude Code の許可の仕組み"]
    HUMAN["人"]
    FS["worktree と workspace"]

    ISSUE -->|"拒む・示す"| TOOL
    HOUT -->|"止める・確かめる"| TOOL
    GOUT -->|"止める・確かめる"| TOOL
    REC -->|"名指す"| TOOL
    DOCS -->|"完了を確かめる"| TOOL
    TOOL -->|"無害化した出力"| SESSION
    SESSION --> HUMAN
    HUMAN -->|"issue 番号・消すかの答え"| SESSION
    HUMAN -->|"削除の許可"| PERM
    SESSION -->|"issue 番号・道具が返したパス"| PERM
    PERM --> TOOL
    TOOL -->|"issue-N だけを作る・確かめたものだけを消す"| FS

    class ISSUE,HOUT,GOUT problem
    class REC,DOCS data
    class TOOL,SESSION,PERM enhanced
    class HUMAN,FS process

    subgraph Legend["Legend"]
        L1[("信頼できない入力")]
        L2[("判定に使うデータ")]
        L3["本書が変えるもの"]
        L4["既存のもの"]
    end
    class L1 problem
    class L2 data
    class L3 enhanced
    class L4 process
```

| 脅威 | 守り |
|---|---|
| 画面や PR の説明に書かれた指示に従い、chief のセッションが道具を通さずに workspace や worktree を作る・消す | `.claude/settings.json` で、作る herdr の呼び出し、閉じるか消す herdr の呼び出し、`git worktree add`・`remove`・`move` を拒む（1.2、3.8）。`/chief` の定義に、herdr を直接呼ばないことを書く（0088 の規則 1） |
| 同じ指示で、chief のセッションが人の確認なしに `chief remove` を呼ぶ | `ask` の規則で、呼ぶたびに Claude Code の許可の確認を出す（1.2）。確認が通っても、道具が削除の確かめ（3.4.2）を行う。chief が作っていない workspace と、完了していないタスクの workspace は消えない |
| 同じ指示で、セッションが任意のパスを `remove --workspace` に渡し、そこで git を走らせる | `--workspace` は今のタスクの workspace の正規のパスと比べるだけで、渡されたパスでは git を走らせない（3.1） |
| 同じ指示で、セッションが任意のパスや名前で作らせる | 道具が受け取るのは issue 番号だけで、パスと名前はその番号だけから決まる（3.2）。セッションが渡す文字列はパスに入らない |
| 同じ指示で、セッションが開いた issue ごとに `chief create` を繰り返し呼び、workspace とエージェントを増やす | 防がない。作るだけでは作業は失われず、作った workspace は表に示される。タスクのない作った workspace は削除の候補にならず、人が端末で片付ける。残る危険として受け入れる |
| issue の題名に書かれた指示や制御文字 | 題名は無害化して示すだけで、パス、名前、送る入力を左右しない（3.3.1）。`/chief` の定義が、題名をデータとして扱う |
| 存在しない issue、閉じた issue、PR の番号で、片付けられずに残る workspace を作らせる | 作る前に issue を確かめる（3.3.1） |
| 打ち間違えた番号が、ほかの開いた issue に当たる | 防げない。作成の結果に題名を示し（AC-07）、`/mkreq` の最初の問いで人が止められる（要件書「要件定義の際に決めたこと」） |
| 作ったことの記録を偽り、chief が作っていない workspace を削除の候補にさせる | 記録のパスは、記録の issue 番号の作る場所と等しくなければならないので、名指せるのは `issue-<N>` の形の worktree だけである（3.4.1）。記録は名指すだけで、Herdr・git・`origin/main` で確かめ直す（3.5）。消すには、人の確認、`ask` の許可、削除の確かめが要る。そのため、コミットして `origin/main` に届いた作業、追跡しているファイルの変更、追跡していないファイル、シェルの pane の前面で動いているプロセスは失われない。残る危険として受け入れる |
| 確かめの後に変更が入り、確かめていないものを消す | 閉じる直前に pane とエージェントとシェルを読み直す。worktree は `--force` なしで消し、git が最後に拒む（3.4.3） |
| ほかの workspace がパスの下で動いているのに worktree を消す | パスの workspace が消す workspace だけであることを確かめる（3.4.2 の `path_in_use`） |
| `~/.herdr` のシンボリックリンクで、ほかの場所を作る先や消す先にする | パスは解決した後で比べ、送れない文字の規則も解決した後のパスに当てる（3.2）。消すのは、作ったことの記録の正規のパスと、Herdr の workspace のパスと、git の worktree の登録が合うものだけである |
| 書き込みのネットワークの操作 | 道具が足すネットワークの操作は、issue の読み取りと `git fetch origin main` だけである（3.1） |

**残る危険のまとめ。**

-   0088 と同じく、同じ利用者のほかのプロセスは、`chief`・`herdr`・`git` を直接呼べ、状態のディレクトリも書ける。chief の守りは、chief のセッションが信頼できない文字列に従っても、決まった範囲の外の操作をしないことにあり、悪意のあるプロセスを防ぐことにはない。
-   拒否と `ask` は前方一致の規則なので、`git -C <dir> worktree add`、`env chief remove …`、インストール先のパスで書いた `chief`、`sh -c` の中の呼び出しなどは拒めず、確認も出ない。0088 で、パスで書いた呼び出しを残る危険としたのと同じである。
-   `ask` の規則が chief の pane の許可の設定（auto mode を含む）で確認を出すかは、実装の最初のフェーズで確かめる（1.2）。出さないときは、要件書 4.2 のとおり、人の確認は `/chief` の問いだけになる。
-   シェルの背景のジョブと止めたジョブ（保存していないエディタを中断したものなど）、Claude Code の背景の実行は、前面のプロセスとエージェントの状態に現れないので確かめられず、workspace を閉じると終わる。
-   `git worktree add` は、リポジトリに設定された `post-checkout` のフックを走らせる。人が手で `git worktree add` を実行するときと同じで、chief が新しく信頼を与えるものではない。
-   git が無視するファイル（ビルドの成果物など）は、worktree とともに消える（要件書 4.2）。

## 6. 処理フロー詳細 (Processing Flow Details)

### 6.1. 作成

図 5 は、`create` の判断の流れを示す。矢印は処理の順を表し、ラベルは分かれる条件を表す。

```mermaid
flowchart TD
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef problem fill:#ffe6e6,stroke:#d62728,stroke-width:2px,color:#7b0000;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    A["番号の形と作る場所"]
    B["issue の読み取り"]
    C["fetch と排他"]
    D["作る先の確かめ"]
    E["作る段"]
    F["sendAfterCreate"]
    R1["作らない"]
    R2["止まる"]
    R3["送らない"]
    OK["送った"]

    A -->|"通る"| B
    A -->|"外れる"| R1
    B -->|"開いた issue"| C
    B -->|"ない・閉じた・PR・読めない"| R1
    C -->|"通る"| D
    C -->|"fetch 失敗・busy"| R1
    D -->|"ない"| E
    D -->|"ある"| R1
    E -->|"すべて済み、Claude Code が起動した"| F
    E -->|"段が失敗"| R2
    E -->|"Claude Code が入力を受け付けない"| R3
    F -->|"確かめが通る"| OK
    F -->|"確かめが外れる"| R3

    class A,B,C,D,E,F process
    class R1,R2,R3 problem
    class OK enhanced

    subgraph Legend["Legend"]
        L1["処理"]
        L2["作らない・止まる・送らない"]
        L3["送った"]
    end
    class L1 process
    class L2 problem
    class L3 enhanced
```

R2 では作ったものを示し、R3 では人がその pane で `/mkreq <N>` を打つよう示す。

### 6.2. 削除の提案と削除

図 6 は、削除の提案から削除までを示す。矢印は呼び出しと応答を表す。

```mermaid
sequenceDiagram
    participant H as 人
    participant S as chief のセッション
    participant P as Claude Code の許可
    participant T as 道具
    participant G as git
    participant HD as herdr

    S->>T: chief tick --protocol 2
    T->>T: 作ったことの記録、対応 issue のタスク、完了、削除の確かめ
    T-->>S: 行 (タスク完了・削除を提案)、removal の項目
    S->>H: 一覧 (名前、パス、issue、タスク、pane の数、確かめの結果)
    H->>S: 削除の提案を選ぶ
    S->>H: 消すかを尋ねる (すべての pane が閉じられる)
    H-->>S: 消す
    S->>P: chief remove --workspace W --protocol 2
    P->>H: 許可の確認
    H-->>P: 許可する
    P->>T: chief remove --workspace W --protocol 2
    T->>T: タスクの workspace と比べ、排他を取る
    T->>G: git fetch origin main
    T->>G: rev-parse、status、merge-base --is-ancestor、worktree list
    T->>HD: agent list、agent read、pane list、pane process-info
    alt すべて通る
        T->>HD: pane list・agent list (読み直す)、workspace close
        T->>HD: pane list (pane が消えるまで)
        T->>G: git worktree remove (--force なし)
        T->>T: そのパスの記録を消す
        T-->>S: 消した
    else 通らない
        T-->>S: 消さなかった (通らなかった確かめ)
    end
```

## 7. テスト戦略 (Test Strategy)

### 7.1. ユニットテスト

-   0088 と同じく、`internal/cmdrunner/testutil` の fake で herdr・git・gh を置き換え、実物を呼ばない（CLAUDE.md の「Testing Strategy」）。
-   **作る場所。** 番号からパスと名前が決まること。`$HOME` が相対パスのとき、送れない文字を含むとき（シンボリックリンクを解決した後に含むときを含む）は作らないこと。`TaskSpace.Worktree` と、作った後の `CanonicalWorktree` が違えば止まること。
-   **issue の読み取り。** 1.2 で確かめた 3 つの応答のサンプルで、開いた issue・PR・存在しない番号を見分けること。閉じた issue のサンプルも足す。要件書 4.2 の表の拒む形（上限を超える、後に続くデータ、重なったメンバー、大文字と小文字だけが違う名前、`null`、違う型、違う番号、知らない `__typename`、`errors` が 2 つ、終了コードと形の組が表にないもの）を、1 つずつ拒むこと。
-   **作成。** AC-11 の 3 つの「ある」を 1 つずつ作り、何も作らないこと。各段を 1 つずつ失敗させ、そこで止まり、後の段の呼び出しも `/mkreq` もないこと、作ったものを示すこと（AC-12）。前の使い方の記録（作ったことの記録、送信の停止、終わりのない送信）が残っているときに、それを消してから作り、`/mkreq` を送れること（AC-14）。Claude Code の起動がエラーの応答で終わり、エージェントがいるときに、`/mkreq` を送らず `agent_not_ready` を返すこと。`WorkspaceMaker` が、自分の作った workspace の ID のほかの ID で herdr を呼ばないこと（AC-13）。作成の排他が持たれている周で、`tick` が自動送信をしないこと。`tick` が排他を持つ間に `create` が `busy` で拒むこと。
-   **削除の候補。** AC-23 の条件を 1 つずつ外した入力の表で、提案しないこと。AC-20 だけの条件（HEAD が detached、エージェントが動いていない）を 1 つずつ外した入力で、行が 0088 の規則のままで、提案は作られ、確かめの結果にその条件が通らないと示されること。記録のパスが記録の issue 番号の作る場所と違うときに提案しないこと。`対応 issue` の行の読み方は、`（` の中のリンク、リンクでない番号、形の合わないリンク、2 つの行、2 つのタスクが同じ issue を挙げる場合を含める。
-   **0093 の表への削除の提案の行（1.5）。** 0093 の部品が `main` にあるときだけ確かめる。削除の提案の行が「あとで」・人・文書「なし」になり、「あとで」の位置に並ぶこと。送信が止まった削除の提案の行は「すぐ」になり、fetch に失敗した周では文書が「要確認」になること。印のない AC-23 だけの行（エージェントが動いている）は、0093 の通常の規則のままであること。
-   **削除。** 削除の確かめを 1 つずつ外し、何も消さないこと（`workspace close` と `git worktree remove` を呼ばないこと）。閉じる直前の読み直しで変わったときに閉じないこと。各段を失敗させ、残ったものを示し、消し直さないこと（AC-32）。`git worktree remove` に `--force` を、`workspace close` に `--group` を渡さないこと。`--workspace` に今のタスクの workspace でないパスを渡したとき、そのパスで git を走らせないこと。
-   **並べ方。** `removal` の項目が、0088 の 4 つの種類の後に並ぶこと（AC-22）。
-   各テストは、確かめる対象を壊して失敗することを確かめる（CLAUDE.md の「Testing Strategy」）。

### 7.2. 結合テストと文書のガード

-   **結合テスト。** 0088 の、Herdr と git と gh をまとめて模した世界（`internal/chief/world_test.go`・`internal/chief/sim_test.go`）を広げ、作成から `/mkreq` の送信まで、`/runpr` の後の detached の worktree が削除の候補になるまで、削除して表から消えるまでを、続けて走らせる。
-   **文書のガード。** `.claude/settings.json` の `deny` が 1.2 の規則を持ち、`ask` が `Bash(chief remove *)` を持つこと（信頼の境界のコマンドのテスト。3.9）。`/chief` の定義の呼び出しの形とプロトコル番号が道具と合うこと、手引きのサブコマンドの表が道具と合うこと（0088 のガードの拡張）。
-   「Tool Execution Safety」の承認と「Go Idioms」の例外の範囲は文の意味なので、テストにせず、PR のレビューで読む。

### 7.3. 実地の確かめ（manual）

-   **最初のフェーズ。** 1.2 の (i)〜(vi) と、Herdr の出力の形（`workspace create`・`tab create`・`agent start` とそのエラーの応答と時間切れ・`pane list`・`pane process-info`・`workspace close`）を、人が実の Herdr で確かめる。起動したばかりの Claude Code の画面のサンプルと、閉じた issue の GraphQL の応答のサンプルも採り、`internal/chief/testdata/` に置く。結果を計画書に記録する。合わないときは、計画の段で設計に戻す。
-   **最後のフェーズ。** chief を動かし、新しいタスクを始めること（新しい workspace と既存の workspace）、作成の拒否（存在しない番号、閉じた issue、PR、すでにあるパス）、削除の提案と削除と拒否を、それぞれ 1 回以上行う。

### 7.4. セキュリティのテスト

-   パスと名前が、issue 番号のほかの入力（題名、セッションの引数）で変わらないこと。題名の制御文字が無害化されること。偽った作ったことの記録（ほかの worktree のパス、ほかの issue の作る場所）が提案も削除も起こさないこと。ユニットテストで確かめる。

## 8. 実装の優先順位 (Implementation Priorities)

計画で PR に分ける。目安の順序は次のとおりである。

1.  **実地の確かめとサンプルの採取（7.3）。** 確かめた結果に応じて、計画を直す。
2.  **読み取りの部品。** 作る場所、issue の読み取りと `strictjson` に足す読み方、Herdr の新しい出力の読み取り、git の新しい呼び出し、`対応 issue` の読み方とタスクの見つけ方。
3.  **作成。** 作ったことの記録、`WorkspaceMaker`、`sendAfterCreate`、`targets`・`create`。
4.  **削除。** 削除の候補の行と項目、削除の確かめ、`WorkspaceCloser`、`remove`。
5.  **定義と文書。** `/chief` の定義、`.claude/settings.json`、`CLAUDE.md`、手引き、`package_reference.md`、文書のガード。そのうえで、実地の最後の確かめ（7.3）を行う。

`Protocol` の値（`internal/chief/records.go`）と、`/chief` の定義の `--protocol` の値は、同じ PR で変える。間の PR でどちらかだけが変わると、入れ直した道具が `/chief` の定義を `protocol_mismatch` で拒むためである。新しいサブコマンドを足す PR は、`/chief` の定義がまだ呼ばないので、`Protocol` を上げる PR より前に置いてよい。

## 9. 将来の拡張 (Future Extensibility)

-   **エージェントのいない workspace の削除。** 削除の候補を、タスクの workspace ではなくパスの workspace から作れば、エージェントが終了した workspace も提案できる。行を持たない提案の示し方を決める必要があるので、本書では行わない。
-   **リポジトリの名前。** パスの `yt2column` を remote から決めるようにすれば、ほかのリポジトリでも使える。今は要件書の値に固定する。
-   **作成の途中で止まった workspace の片付け。** 作ったことの記録に止まった段を持たせれば、それを示して人の片付けを助けられる。今は、出力に作ったものを示すだけである。

## 付録 A: 申し送り（design_handoff.md）への対応

| 項目 | 対応 |
|---|---|
| H-01: 作ったことの記録の置き場所と形 | 0088 の workspace ごとの記録のディレクトリの `created.json`。正規のパス、issue 番号、時刻を持つ。`git worktree add` の直後に作る（途中で失敗しても残る）。作る前に前の使い方の記録をすべて消し、消し終えたらすべて消す。記録のパスは記録の issue 番号の作る場所と等しくなければならない（1.2、3.3.3、3.4.1、3.5） |
| H-02: worktree と workspace の作り方 | `git worktree add --detach` と `herdr workspace create --cwd`。タブ 3 つ（1.2、3.3.3）。親のディレクトリがなければ作り、シンボリックリンクは解決してから比べる（3.2） |
| H-03: エージェントの起動と、入力を受け付ける状態の確かめ | `herdr agent start` を引数なしに呼び、上限 60 秒で待つ。エラーの応答でエージェントがいるときは、送らずに人に示す（3.3.4）。`--trust-repository` は Herdr の skill の説明で Git の信頼をリクエストごとに与えるもので、`herdr worktree` を使わないので使わない（1.2）。信頼の画面と、起動したばかりの Claude Code が送る前の確かめを通るかは、実装の最初のフェーズで確かめる（7.3） |
| H-04: issue からタスクを見つけ、完了を読む方法 | `git grep` で `対応 issue:` の行を集め、最初の `（` より前の `#<数字>` をリンクの形によらずに数え、読み方の分からないものは提案を減らす向きに数える。完了は `Plan.Done`。`/mkreq` の探し方との違いも記録した（3.4.4） |
| H-05: セッションへの拒否と、削除のサブコマンドの許可 | 作る herdr の呼び出しと `git worktree add`・`remove`・`move` を拒み、`chief remove` を `ask` にする（1.2、3.8） |
| H-06: 消す順序と、消すときの git と Herdr の振る舞い | 確かめ、読み直してから workspace を閉じる、pane が消えるのを待つ、`git worktree remove`（`--force` なし）、記録を消す、の順。エージェントでない pane の前面のプロセスと、ほかの workspace がパスを使っていないことを確かめる（1.2、3.4.2、3.4.3） |
| H-07: issue を読む操作と、issue か PR かの見分け | `gh api graphql` の `issueOrPullRequest`。`__typename` と、`NOT_FOUND` の `errors` で見分ける。2026-11-16 に確かめた（1.2、3.3.1） |
| H-08: `/chief` の流れと道具の契約 | `targets`・`create`・`remove` を足し、`--protocol` を `2` に上げる。理由の綴りは 4.1。ガードは 3.9・7.2 |
| H-09: 削除の提案がある行の急ぎの度合い・番・進行の位置（0093 から） | 削除の提案の行（AC-20 の行）に `Row` の印を付け、`Row.Idle` にしない。0093 の部品のうち `main` にあるものに、対応表では順 2 の後・順 3 の前に「あとで」・人の行を、文書の規則では規則 2 の文書「なし」を足す。並べ方は 0093 のままで「あとで」の位置になる。計画書が、削除の提案の行を作る PR とその後の PR の前に、部品ごとに確かめて足し、0093 の AC-05・AC-07 の規則 2・AC-15 を確かめる（1.5、3.4.1、3.6、7.1） |

## 付録 B: 受け入れ基準との対応

| AC | 設計の箇所 |
|---|---|
| AC-01 | 3.7 |
| AC-02 | 1.3、3.3.5、3.7 |
| AC-03、AC-04 | 3.3.5 |
| AC-05 | 3.3.1 |
| AC-06 | 3.1、3.3.2 |
| AC-07 | 3.3.1、3.7 |
| AC-08 | 1.2、3.3.2、3.3.3 |
| AC-09 | 1.2、2.1、3.3.3 |
| AC-10 | 3.3.3、3.3.4 |
| AC-11 | 3.2、3.3.2 |
| AC-12 | 3.1、3.3.3 |
| AC-13 | 3.1、3.3.2、3.3.3、3.6 |
| AC-14 | 3.3.3、3.5 |
| AC-15 | 1.3、3.3.4 |
| AC-16、AC-17 | 3.3.4、3.7 |
| AC-18 | 3.3.4 |
| AC-19 | 3.3.4 |
| AC-20、AC-21 | 3.4.1 |
| AC-22 | 3.4.1、3.7 |
| AC-23、AC-24 | 3.4.1、3.4.4、3.5 |
| AC-25 | 3.4.1、3.6、3.7 |
| AC-26 | 3.4.1 |
| AC-27 | 1.2、3.7 |
| AC-28 | 1.2、3.4.2 |
| AC-29 | 3.4.3 |
| AC-30 | 3.4.3 |
| AC-31 | 3.4.3 |
| AC-32 | 3.1、3.4.3 |
| AC-33 | 3.4.3、3.5 |

## 変更履歴

- 2026-11-16 承認（isseis、#910）
- 2026-11-17 draft への戻し：0093 の表での削除の提案の行の扱い（申し送りの H-09）と、0093 と重なる `--protocol` の番号の決め方を足した（#未記入）
