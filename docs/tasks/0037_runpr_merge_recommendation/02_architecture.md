# アーキテクチャ設計書：/runpr のマージの質問で、次のコマンドを続けて実行する選択肢を推奨にしない

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-22 |
| Review date | 2026-10-22 |
| Reviewer | isseis |
| Comments | 7.2 の検索の語 `and the next command` が、3.5 で変えない `runpr.md:66` の Authorization の文にも一致していたため、`pre-merge bookkeeping and the next command` に絞った（編集上の訂正。決定は変えていない）。2026-10-23: `e529b68` の後に入った #454 が、`cmd/yt2column/codex_review_docs_test.go:119` で step 5 の見出しを定数として固定し、step 5 の節を読むようになった。そのため、1.1・1.4・3.8・7.1・9 章の「step 5 の見出しはどのテストも読まない」「変えるテストはない」という記述を改め、この定数が 3.5 の見出しの変更に従うことを記した（編集上の訂正。見出しを改める決定（3.5、AC-11）を含め、決定は変えていない。レビュアーが確認した）。2026-10-23（実装時）: 冒頭の「次の PR マーカー」の定義を、「NEXT が開く PR のマーカー」を言い換えた「『PR を作成した』が `[ ]` の最初のマーカー」に改めた。実装計画書の途中の改訂を承認するタスク文書の PR では、最初のマーカーが NEXT の PR のものではないためである。あわせて、3.4 の再開の方法に、ブランチがローカルにだけある場合と、`origin` にあるかを確かめられなかった場合を加えた。どちらも 3.4 の「実際の状態を述べ、その状態からの再開の方法を示す」の具体化で、決定は変えていない（編集上の訂正） |

本書は [01_requirements.md](01_requirements.md)（要件定義書。以下、要件書）の設計である。既存のファイルについての記述は、コミット `e529b68` のファイルで確かめた。`file:line` はこのコミットの行番号を指す。F-NNN・AC-NN は要件書の項番を指す。要件のレビューで設計に申し送られた事項（[design_handoff.md](design_handoff.md) の H-01〜H-03）への対応は 3.7 にまとめる。

本書は、要件書 §6 の用語（マージの質問、マージ前の記録（PRE_MERGE）、マージだけの選択肢、続けて実行する選択肢、最終報告、次のコマンド（NEXT）、実装モデル要件）を同じ意味で使う。ほかに次の用語を使う。

-   **コマンドの定義:** `.claude/commands/` の下の Markdown ファイル。エージェントへの指示であり、英語で書く（`CLAUDE.md` の「Language」）。
-   **次のブランチ:** マージの後、NEXT があるときに `runpr.md` の step 5 が、`_lib/task-pr-handoff.md` Part A step 4 で `origin/main` から作るブランチ（`runpr.md:451-454`）。NEXT はこのブランチで作業を始める。
-   **次のブランチを作り終えた:** step 5 が Part A step 4 を最後まで終えたこと。ブランチが `origin` やローカルにあるだけでは、作り終えたとはいわない（3.4）。
-   **`/clear` の案内:** マージだけの選択肢を選び、次のブランチを作り終えたときに、最終報告で `/clear` してから NEXT を実行するよう示す文（AC-05・AC-06）。
-   **再開の方法:** step 5 が次のブランチを作り終える前に止まったとき、最終報告で示す、NEXT を始めるための手順（AC-07）。
-   **次の PR マーカー:** NEXT が `/runplan` のとき、NEXT が開く PR の PR マーカー（`### PR-N 作成ポイント`）。実装計画書のマーカーのうち、「PR を作成した」がまだ `[ ]` の最初のマーカーである。プランの PR では、マージする PR の後のマーカーである。実装計画書を初めて承認するタスク文書の PR では、最初のマーカーである（3.2）。

## 1. 設計の全体像 (Design Overview)

### 1.1. 設計原則

-   **振る舞いの種類は変えず、推奨と報告だけを変える。** マージの質問の選択肢の種類と、それぞれを選んだときの記録・マージ・ブランチの作成・NEXT の実行は、今のままとする（要件書 4.4）。変えるのは、「Merge」の選択肢の推奨の付け方と順序、step 4 のメッセージの `実装モデル要件` の値がないときの扱い、step 5 の最終報告である。
-   **推奨は step 4 の 1 か所で定める。** 「Merge」の選択肢のどれを推奨にするかは、`runpr.md` の step 4 の選択肢の一覧にだけ書く（要件書 4.5）。ほかの文書（`CLAUDE.md`・`_lib/task-pr-handoff.md`・`requirements_process.md`・`mergepr.md`・`_context.md`）は、`/runpr` が NEXT を示し、利用者が選んだときに実行するとだけ述べ、推奨には触れない。推奨を変えるときに書き直す場所を 1 つにするためである。
-   **NEXT を始める場所は次のブランチに限る。** `/clear` の案内は、step 5 が次のブランチを作り終えた後にだけ示し、そのブランチの名前と worktree を添える。作り終える前に止まったときは、案内の代わりに再開の方法を示す（AC-07）。案内どおりに同じ worktree で NEXT を実行したとき、NEXT は次のブランチで始まる（`_lib/task-pr-handoff.md` Part A step 3、`.claude/commands/_lib/task-pr-handoff.md:69-74`）。
-   **報告は、実際にある状態を述べる。** 止まったときの最終報告は、次のブランチがどこまでできたか（ない、`origin` にだけある、チェックアウトされている）を確かめて述べる。推測で「作っていない」と書かない（3.4）。
-   **利用者が見た値を報告する。** `/clear` の案内に添える `実装モデル要件` は、マージを決めた step 4 の質問で示した値をそのまま使う。マージの後に読み直さない（1.2）。
-   **実行時のコードを変えない。** 変えるのは、コマンドの定義と、`CLAUDE.md`・`docs/dev/conventions/requirements_process.md` の記述だけである。Go のコード・拡張のコードは変えない。Go のテストは、step 5 の見出しの定数（1.4）が見出しの変更に従うだけである。

### 1.2. 設計の際に決めたこと（2026-10-22）

-   **文書のガードのテストは加えない（H-01）:** マージの選択肢の推奨と順序（AC-01〜AC-03）と、最終報告の案内（AC-05〜AC-08）を固定するテストは加えず、静的な確認（7.2）で確かめる。レビュアーが決めた。`TestRunprDocContract`（`cmd/yt2column/fixpr_docs_test.go:282`）に `requirePhrases` のサブテストを加える案も示したが、採らなかった。後の変更で推奨が戻っても、テストでは検出されない。このリスクは 4.4 に記す。
-   **案内に添える `実装モデル要件` は、step 4 の質問で示した値を使う（H-02）:** マージを決めた step 4 の質問のメッセージで示した値を、最終報告にもそのまま示す。値がないときは、質問と報告の両方で値がないことを示し、分かる範囲で理由（次の PR マーカーがない、またはマーカーに `実装モデル要件` の欄がない）を添える。マージの後に `origin/main` の実装計画書から読み直す案は採らない。PRE_MERGE が書くのは文書の承認とチェックポイントのチェックボックスだけで（`runpr.md:163-176`）、マーカーの `実装モデル要件` の欄は含まないので、読み直しても同じ値になる。それなのに手順が増え、利用者が質問で見た値と報告の値が食い違う余地も生まれるためである。
-   **ブランチが途中までできた状態を、AC-07 に補って扱う:** AC-07 は、次のブランチを作る前に止まった場合（ブランチがない）を定める。Part A step 4 の途中で止まると、ブランチが `origin` にだけある、またはチェックアウトされているのに後の確かめが失敗した、という状態がありうる（`.claude/commands/_lib/task-pr-handoff.md:91-103`）。要件書はこの場合を定めていないので、本書は AC-07 と同じ報告（`/clear` の案内を載せず、止まった理由と再開の方法を示す）を適用し、「次のブランチを作っていない」の代わりにブランチの実際の状態と、その状態からの再開の方法を示す（3.4）。ブランチがない場合の報告は AC-07 のとおりで、要件は変えない。実際とは違う「作っていない」を報告すると、利用者が残ったブランチに気づかず、NEXT が別の番号のブランチを作るためである。
-   **案内の文面は、載せる項目だけを英語で定める（H-03）:** `runpr.md` には、`/clear` の案内に載せる項目（3.3）だけを英語で書く。日本語の文例は載せない。`runpr.md` のほかの報告（issue を閉じるときのコメント。`runpr.md:470-471`）も、日本語で書くことと載せる項目だけを定めており、それにそろえる。

### 1.3. 概念モデル

マージの質問の「Merge」の選択肢と、それぞれを選んだときに `/runpr` がマージの後にすることを示す。矢印 A → B は「A を選ぶと、マージの後に B を行う」を表す。

```mermaid
flowchart LR
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef problem fill:#ffe6e6,stroke:#d62728,stroke-width:2px,color:#7b0000;

    subgraph Before["現在"]
        B1["続けて実行する選択肢<br>(推奨・1 番目)"]
        B2["マージだけの選択肢<br>(2 番目)"]
        BR1["NEXT を同じセッションで実行"]
        BR2["最終報告<br>(状態だけ)"]
        B1 --> BR1
        B2 --> BR2
        class B1 problem
        class B2,BR1,BR2 process
    end

    subgraph After["本タスクの後"]
        A2["マージだけの選択肢<br>(推奨・1 番目)"]
        A1["続けて実行する選択肢<br>(推奨なし・2 番目)"]
        AR2["最終報告<br>(/clear の案内)"]
        AR1["NEXT を同じセッションで実行"]
        A2 --> AR2
        A1 --> AR1
        class A2,AR2,A1 enhanced
        class AR1 process
    end
```

```mermaid
flowchart LR
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef problem fill:#ffe6e6,stroke:#d62728,stroke-width:2px,color:#7b0000;
    subgraph Legend["凡例"]
        L1["変わらない選択肢・振る舞い"]
        L2["本タスクで変わる選択肢・振る舞い"]
        L3["本タスクで直す問題"]
    end
    class L1 process
    class L2 enhanced
    class L3 problem
```

図は NEXT があり、次のブランチを作り終えたときを示す。NEXT がないときは、続けて実行する選択肢を示さず（AC-02）、マージだけの選択肢を推奨にする（AC-01）。最終報告に `/clear` の案内は載せない（AC-08）。「承認なしでマージ」と「マージしない」は図から省いた。どちらも今のままで、推奨にしない（AC-03）。

### 1.4. 既存の仕組みとの関係

-   **文書のガードのテスト:** `cmd/yt2column/fixpr_docs_test.go:36-38` は、`runpr.md` の step 1・step 3・step 4 の見出しを定数として読む。本タスクは step 4 の見出しを変えないので、この定数は変わらない。step 5 の見出し（`runpr.md:443`）は変える。この見出しは、#454 の後は `cmd/yt2column/codex_review_docs_test.go:119` の定数が固定し、その節を読む（`7b26533` で確かめた）。この定数は、3.5 の見出しの変更に合わせて改める。`e529b68` で、`*_test.go` と `extension/` の `*.ts` に、本タスクが書き換える文言（`run the next command`、`starts the next command`、`run NEXT`、`carry it out`、`recommended`）を固定する箇所がないことを `git grep` で確かめた。そのため、本タスクで直す既存のテストは、上の step 5 の見出しの定数だけである。
-   **`AskUserQuestion` の慣習:** `AskUserQuestion` のツールの説明は、推奨する選択肢を先頭に置き、ラベルに「(Recommended)」を付けるよう求める。今の step 4 も推奨を先頭に置いている（`runpr.md:359`）。本タスクの後も、推奨（マージだけの選択肢）を先頭に置くので、この慣習と食い違わない。
-   **PR レビューの質問の推奨:** Escalation（PR レビューをもう一度求めるかの判定。`.claude/commands/_lib/pr-review-escalation.md`）が `ask` のとき、PR レビューの質問は「request a PR review first」を推奨にする（`runpr.md:352-356`）。これは「Merge」とは別の質問なので、同じ `AskUserQuestion` の呼び出しで、「まだマージしない」と「マージする」の両方に推奨が付くことがある。今は NEXT があるときだけ起きるが、本タスクの後はマージだけの選択肢が常に推奨になるので、standalone change でも起きる。「request a PR review first」を選ぶとマージしないので（`runpr.md:377-381`）、両方の質問に推奨が付いても、振る舞いは今と同じく一意に決まる。本タスクはこの質問を変えず（要件書 2.3）、この重なりを受け入れる（4.4）。

## 2. システム構成 (System Structure)

### 2.1. ファイル構成

本タスクが変えるファイルと、それらの参照関係を示す。矢印 A → B は「A の記述が B の手順を参照する、または B の振る舞いを説明する」を表す。実線は今ある参照、点線は本タスクで加わる参照を表す。

```mermaid
flowchart LR
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    RPR4["runpr.md<br>(step 4: マージの質問)"]
    RPR5["runpr.md<br>(step 5: 最終報告)"]
    RPRH["runpr.md<br>(description・冒頭の説明)"]
    CLAUDE["CLAUDE.md<br>(Claude Commands)"]
    HAND["_lib/task-pr-handoff.md<br>(冒頭の説明・Part A)"]
    REQP["requirements_process.md<br>(Review Flow)"]
    MRG["mergepr.md<br>(step 3)"]
    CTXM["_context.md<br>(PR marker conventions)"]

    RPRH --> RPR4
    RPRH --> RPR5
    CLAUDE --> RPR4
    HAND --> RPR4
    REQP --> RPR4
    MRG --> RPR4
    CTXM --> RPR4
    CTXM -.-> RPR5
    RPR5 --> HAND

    class RPR4,RPR5,RPRH,CLAUDE,HAND,REQP,MRG,CTXM enhanced
```

```mermaid
flowchart LR
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    subgraph Legend["凡例"]
        L1["本タスクで変える記述"]
    end
    class L1 enhanced
```

-   `RPR5 → HAND` は、step 5 が次のブランチを `_lib/task-pr-handoff.md` Part A step 4 で作ること（`runpr.md:451-454`）を表す。本タスクで、再開の方法が Part A step 3〜4 を参照するようになるが、同じファイルへの参照なので矢印は 1 本とした。`HAND` で変えるのは冒頭の説明だけで、Part A は変えない。
-   `CTXM -.-> RPR5` は、`_context.md` の `実装モデル要件` の説明が、値を示す場所として step 5 の `/clear` の案内を挙げるようになることを表す（3.6）。今の記述はマージの質問だけを挙げる（`_context.md:63`）。
-   `/runpr` のマージの後の振る舞いを述べるが変えない記述として、`.claude/commands/mkplan2.md:107`（「`/runpr` offers the approval and `/runplan` as the next command at the merge.」）、`.claude/commands/runplan.md:111-115`（`/runpr` が次のブランチへ切り替え、利用者が `/runplan` を再び実行する）、`.claude/commands/mkreq.md:15-17` を確かめた。どれも NEXT を必ず実行するとは読めないので、図に含めない。

### 2.2. マージの質問から最終報告までの流れ

マージの質問の答えから、step 5 の最終報告までの分かれ方を示す。矢印 A → B は「A の次に B へ進む」を表す。

```mermaid
flowchart TD
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    Q{"マージの質問の答え<br>(step 4)"}
    NOM["マージせずに止まる"]
    MRG["(PRE_MERGE の記録と) マージ<br>(step 4)"]
    CLN["片付けと git fetch<br>(step 5)"]
    HASN{"NEXT があるか"}
    HASF{"NEXT があるか"}
    BR["次のブランチを作る<br>(step 5)"]
    DET["origin/main で detach<br>(step 5)"]
    STOPN["最終報告:<br>止まった理由"]
    STOP["最終報告:<br>止まった理由・次のブランチの状態・再開の方法"]
    OPT{"選んだ選択肢"}
    RUN["NEXT を実行する"]
    GUIDE["最終報告:<br>/clear の案内"]
    PLAIN["最終報告:<br>状態だけ"]

    Q -->|"マージしない"| NOM
    Q -->|"マージする選択肢"| MRG
    MRG --> CLN
    CLN -->|"失敗"| HASF
    HASF -->|"ない"| STOPN
    HASF -->|"ある"| STOP
    CLN -->|"成功"| HASN
    HASN -->|"ある"| BR
    HASN -->|"ない"| DET
    BR -->|"作り終える前に失敗"| STOP
    BR -->|"作り終えた"| OPT
    OPT -->|"続けて実行する選択肢"| RUN
    OPT -->|"マージだけの選択肢"| GUIDE
    DET --> PLAIN

    class Q,NOM,MRG,CLN,HASN,HASF,BR,DET,STOPN,OPT,RUN,PLAIN process
    class STOP,GUIDE enhanced
```

```mermaid
flowchart LR
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    subgraph Legend["凡例"]
        L1["変わらない処理・報告"]
        L2["本タスクで加わる・変わる報告"]
    end
    class L1 process
    class L2 enhanced
```

-   「承認なしでマージ」は PRE_MERGE を記録せず、NEXT もないので（`runpr.md:363-365`）、「ない」の枝に進み、最終報告に `/clear` の案内を載せない（AC-08）。
-   NEXT がないときの片付け・`git fetch`・detach の失敗は、今と同じく止まった理由を報告する。次のブランチを作らないので、再開の方法（AC-07）は示さない。
-   タスクの最後のプランの PR で issue を閉じる手順（`runpr.md:459-471`）は、NEXT がないときだけ行われ（`runpr.md:178-179`）、図の「ない」の枝に含まれる。本タスクはこの手順を変えない。

## 3. コンポーネント設計 (Component Design)

本タスクは Go の型やインターフェースを加えない。以下、各ファイルで述べることを設計する。英語の具体的な文は、実装で決める。

### 3.1. マージの質問の「Merge」の選択肢（`runpr.md` の step 4。F-001・AC-01〜AC-03）

`runpr.md:357-368` の選択肢の一覧を、次の順序と推奨に改める。

| 順序 | 選択肢 | 示す条件 | 推奨 |
|---|---|---|---|
| 1 | マージだけの選択肢: PRE_MERGE を記録してマージし、NEXT を実行しない（PRE_MERGE がないときは「merge」） | 常に | 推奨と示す |
| 2 | 続けて実行する選択肢: PRE_MERGE を記録してマージし、NEXT を実行する | NEXT があるとき | 示さない |
| 3 | 承認なしでマージ | タスク文書の PR で、PRE_MERGE に承認があるとき（今のまま） | 示さない |
| 4 | マージしない | 常に（今のまま） | 示さない |

-   推奨の付け方は、NEXT の有無と PR の種類（プランの PR、タスク文書の PR、standalone change）によらないと明記する（AC-01）。
-   今の一覧は、推奨を 1 番目の選択肢の括弧書き「(recommended; only when NEXT is set)」で示している（`runpr.md:359-360`）。本タスクの後は、推奨をマージだけの選択肢の項目に書き、続けて実行する選択肢には条件（NEXT があるとき）だけを書く。
-   選択肢は最大 4 つで、`AskUserQuestion` の上限を超えない（要件書 4.4、`runpr.md:347`）。
-   「Offer those of these that apply, and always the last」（`runpr.md:357-358`）は残す。

### 3.2. マージの質問のメッセージ（`runpr.md` の step 4 の 2。AC-04）

`runpr.md:345-347` の、NEXT を示す箇所を次のように改める。

-   示す値は、次の PR マーカー（本書冒頭の用語）の `実装モデル要件` であると明記する。今の「the next PR marker」は、どのマーカーかを定めていない。「PR がマージされた」の状態や位置でマーカーを選ぶと、マージする PR 自身のマーカーと取り違えうるためである。
-   値がないときは、値がないことを示し、分かる範囲で理由を添える。値がない例は、PR マーカーのない単一フェーズの実装計画書を承認するタスク文書の PR である（`.claude/commands/runplan.md:213`）。マーカーはあるが `実装モデル要件` の欄がないときも、値がないものとして扱う。
-   値を示す理由を、「このセッションのモデルが合うか」に加え、「`/clear` の後に新しいセッションをどのモデルで始めるか」を判断するためと改める。マージだけの選択肢が推奨になると、値の主な使い道は後者になるためである（要件書 1 節の「要件定義の際に決めたこと」）。

ここで示した値（または値がないこと）を、step 5 の `/clear` の案内でもそのまま使う（1.2）。step 4 を 2 回以上尋ねたとき（deferred のスレッドを直した後など。`runpr.md:383-388`）は、step 2 を再び実行して NEXT を決め直すので、使うのはマージを決めた最後の質問で示した値である。

### 3.3. 最終報告（`runpr.md` の step 5。F-002・AC-05〜AC-10）

`runpr.md:472-474` の最後の項目を、選んだ選択肢と step 5 の進み方に応じて、次のように分ける。

| 場合 | 最終報告 | 対応する AC |
|---|---|---|
| 続けて実行する選択肢を選び、次のブランチを作り終えた | 今と同じく NEXT を実行する。NEXT が PR を開いて止まったところで、このコマンドも終わる | AC-09 |
| マージだけの選択肢を選び、NEXT があり、次のブランチを作り終えた | マージの結果に加え、`/clear` の案内を載せる | AC-05・AC-06・AC-10 |
| NEXT がない（「承認なしでマージ」を含む） | 今と同じく最終の状態を報告する。`/clear` の案内は載せない | AC-08 |
| NEXT があるが、step 5 が次のブランチを作り終える前に止まった | `/clear` の案内は載せない。止まった理由、次のブランチの状態、再開の方法を示す（3.4） | AC-07 |
| マージしなかった | 今と同じく止まる（`runpr.md:374-375`）。step 5 に進まないので、`/clear` の案内は載せない | AC-08 |

`/clear` の案内に載せる項目は次のとおりである。

1.  `/clear` してから NEXT を実行すること。
2.  NEXT を、引数を含めてそのまま実行できる形で示す（例: `/runplan 0037`）。
3.  次のブランチの名前と、今の worktree のパス。NEXT はこの worktree で、このブランチをチェックアウトしたまま実行する。ほかの worktree で実行すると、NEXT は Part A step 4 で別の番号のブランチを作り、step 5 が作ったブランチは使われずに残る（4.4）。
4.  NEXT が `/runplan` のときは、3.2 で示した `実装モデル要件`（値がないときは、値がないことと分かる範囲の理由）。

-   次のブランチの作成（`runpr.md:451-454`）と、プランの PR の「次のブランチへ切り替えた」の記録（`runpr.md:174-176`）は変えない（AC-10）。どちらも「Merge」の選択肢によらず、NEXT があるかだけで決まる。
-   「作り終える前に止まった」の扱いは、続けて実行する選択肢を選んだときにも同じく適用する。今の step 5 も、次のブランチを作れなければ NEXT を実行せずに止まる。その報告に、次のブランチの状態と再開の方法を加える。

### 3.4. 止まったときの報告と再開の方法（`runpr.md` の step 5。AC-07）

step 5 が次のブランチを作り終える前に止まる場合は、たとえば、片付けを終えられないとき、`git fetch origin main` が失敗したとき、Part A step 4 の途中（`gh api user`、`gh pr list`、リンクする issue の選択、`gh issue develop`、その後の `HEAD` の確かめと `git merge --ff-only`）で失敗したときである（`runpr.md:447-454`、`.claude/commands/_lib/task-pr-handoff.md:84-104`）。境界は「Part A step 4 を最後まで終えたか」であり、どの手順で止まったかを列挙して決めるものではない。

このとき最終報告は、次のことを示す。

1.  **止まった理由。**
2.  **次のブランチの状態。** 確かめたうえで、次のことを示す。作ろうとしたブランチの名前（名前を決める前に止まったときは、決める前に止まったこと）、そのブランチが `origin` とローカルにあるか、今チェックアウトしているブランチがどれか、の 3 点である。Part A step 4 の途中で止まると、ブランチが `origin` にだけある、またはチェックアウトされているのに確かめが失敗した、という状態がありうるので、「次のブランチを作っていない」と決めつけない。ブランチがないときは、AC-07 のとおり、次のブランチを作っていないことを示す。
3.  **計画書の記録との食い違い（プランの PR のとき）。** PRE_MERGE は、`main` の実装計画書に「次のブランチへ切り替えた」を `[x]` と記録している（`runpr.md:174-176`）。実際には切り替えていないが、これは想定どおりであり、直さなくてよいと示す（要件書 1 節の「起草の際に補った振る舞い」）。
4.  **再開の方法。** 原因の解消と、NEXT の実行からなる。
    -   **原因の解消:** 原因ごとに、利用者がすることを示す。`mergepr` の片付けが残ったときは、実際の状態ファイルのパスを入れた `mergepr cleanup --state <state-file>` を示す。片付けが残ると、`mergepr` の作業ディレクトリ（linked worktree では worktree の直下の `mergepr-*`。`.claude/commands/mergepr.md:36-41`）が作業ツリーに残り、NEXT は Part A step 1 の、作業ツリーがきれいかの確かめで止まるためである（`.claude/commands/_lib/task-pr-handoff.md:65-66`）。片付けが成功すれば、作業ディレクトリは消える（`mergepr.md:137-138`）。
    -   **NEXT の実行:** 次のブランチの状態に応じて、次のいずれかを示す。そのうえで、利用者が NEXT を引数ごと実行するよう案内する。
        -   次のブランチがチェックアウトされている: そのまま NEXT を実行する。Part A step 3 は、規則どおりの名前で自分の commit を持たないブランチをそのまま使う（`.claude/commands/_lib/task-pr-handoff.md:69-74`）。
        -   次のブランチがローカルにあるが、チェックアウトされていない: そのブランチに切り替えてから（`git switch <branch>`）、NEXT を実行する。
        -   次のブランチが `origin` にだけある: そのブランチをチェックアウトしてから（例: `git switch <branch>`。`origin` の同名のブランチを追跡するローカルのブランチができる）、NEXT を実行する。上と同じ理由で、Part A step 3 がそのブランチを使う。
        -   次のブランチがない: マージしたブランチにまだいるときは、そこから離れて（例: `git switch --detach origin/main`）から、NEXT を実行する。NEXT が Part A step 4 で次のブランチを作る。
        -   `origin` にあるかを確かめられなかった: 原因を解消してから確かめ直すコマンドと、その結果に応じた上の手順を示す。

マージしたブランチから離れる必要があるのは、NEXT が Part A step 3 で、規則どおりの名前で `origin/main` より先の commit と PR を持つブランチを見つけると止まるためである（`.claude/commands/_lib/task-pr-handoff.md:81-83`）。squash マージの後、マージしたブランチの commit は `main` の祖先ではない（`runpr.md:455-456`）。

`/runpr` の再実行は、再開の方法として示さない。マージ済みの PR では、step 1 の `MERGED` の場合に進み、残った片付けは終えるが、次のブランチを作らずに止まるためである（`runpr.md:89-95`）。

### 3.5. `/runpr` の説明（`runpr.md` の frontmatter・冒頭・step 5 の見出し。F-003・AC-11）

| 箇所 | 今の記述 | 改めた後に述べること |
|---|---|---|
| `runpr.md:2`（`description`） | 「... merge it, and run the next command」 | NEXT を示し、利用者が選んだときに実行する |
| `runpr.md:30-31`（冒頭の説明） | 「ask the user one question that covers the bookkeeping, the merge, and the next command, and carry it out」 | 1 つの質問で記録とマージを決め、NEXT を示して、利用者が選んだときに実行する |
| `runpr.md:443`（step 5 の見出し） | 「## 5. Switch branch, then run NEXT」 | ブランチを切り替え、NEXT を実行する、または示す（例: 「## 5. Switch branch, then run or show NEXT」） |

「Authorization」の段落の「the next command (NEXT) when the user chooses to run it in step 4」（`runpr.md:66`）は、「利用者が選んだときに実行する」とすでに述べているので変えない。

### 3.6. ほかの文書の記述（F-003・AC-12〜AC-14）

| ファイル | 箇所 | 改めた後に述べること |
|---|---|---|
| `CLAUDE.md` | 「Claude Commands」の `/runpr` の行（`CLAUDE.md:86`） | 記録し、マージし、NEXT を示す（利用者が選んだときに実行する） |
| `CLAUDE.md` | タスクの PR の流れの段落（`CLAUDE.md:98-102`） | 1 つの質問で記録とマージを決め、NEXT を示す。利用者が選んだときは続けて実行し、NEXT が次の PR を開いて止まる |
| `.claude/commands/_lib/task-pr-handoff.md` | 冒頭の説明（`:19-23`） | 質問が扱うのは、マージ、その直前の記録、マージの後に示す NEXT であり、NEXT を実行するのは利用者が選んだときである |
| `docs/dev/conventions/requirements_process.md` | 「Review Flow」の段落（`:55`） | 質問で承認とマージを決め、NEXT を示す。利用者が選んだときは続けて実行する |
| `.claude/commands/mergepr.md` | step 3 の、`runpr.md` の中で実行するときの説明（`:122-125`） | `runpr.md` の step 4 が、この質問をマージ前の記録と NEXT の扱いと合わせて尋ねる |
| `.claude/commands/_context.md` | 「PR marker conventions」の `実装モデル要件`（`:63`） | 値を示す場所として、マージの質問と、`runpr.md` の step 5 の最終報告の `/clear` の案内を挙げる。案内を示す条件は書かず、`runpr.md` の step 5 を参照する |

-   どの記述も英語で書く（AC-14）。`requirements_process.md` は `docs/dev/developer_guide/*.md` なので、`CLAUDE.md` の「Documents」の例外として英語である。
-   どの記述も推奨には触れない（1.1）。
-   `requirements_process.md` の「Review Flow」の図（`:41-53`）は、承認とマージの流れだけを示し、NEXT を実行するとは書いていないので変えない。

### 3.7. design_handoff の各項目への対応

| 項目 | 採った方針 | 本書の節 |
|---|---|---|
| H-01: 文書のガードのテスト | 加えない（レビュアーの判断）。静的な確認で確かめる。step 4 の見出しは変えないので、`fixpr_docs_test.go:38` の定数も変わらない | 1.2・1.4・7.2 |
| H-02: 最終報告に載せる `実装モデル要件` の取り方 | マージを決めた step 4 の質問で示した値を使う。値がないときは、値がないことと分かる範囲の理由を示す | 1.2・3.2・3.3 |
| H-03: 案内の文面 | 載せる項目だけを英語で定め、日本語の文例は載せない | 1.2・3.3 |

### 3.8. コンポーネント責務表

すべて既存のファイルの変更である。新しいファイルはない。

| ファイル | 本タスクで述べること | 対応する AC |
|---|---|---|
| `.claude/commands/runpr.md` | step 4: 「Merge」の選択肢の順序と推奨（3.1）、次の PR マーカーと `実装モデル要件` の値がないときの扱い（3.2）。step 5: 最終報告の分け方、`/clear` の案内、止まったときの報告と再開の方法（3.3・3.4）。frontmatter・冒頭・step 5 の見出し（3.5） | AC-01〜AC-11 |
| `CLAUDE.md` | 「Claude Commands」の `/runpr` の行と、タスクの PR の流れの段落（3.6） | AC-12・AC-14 |
| `.claude/commands/_lib/task-pr-handoff.md` | 冒頭の説明（3.6） | AC-12・AC-14 |
| `docs/dev/conventions/requirements_process.md` | 「Review Flow」の段落（3.6） | AC-12・AC-14 |
| `.claude/commands/mergepr.md` | step 3 の、`runpr.md` の中で実行するときの説明（3.6） | AC-12・AC-14 |
| `.claude/commands/_context.md` | 「PR marker conventions」の `実装モデル要件`（3.6） | AC-13・AC-14 |

変えるテストは、`codex_review_docs_test.go` の step 5 の見出しの定数だけである（1.4）。

## 4. エラーハンドリング設計 (Error Handling Design)

### 4.1. エラーの表し方

本タスクは Go のコードを変えないので、新しいエラー型はない。

### 4.2. 止まる場合と報告

| 場合 | 扱い |
|---|---|
| step 5 が次のブランチを作り終える前に止まった（NEXT がある） | `/clear` の案内を載せず、止まった理由、次のブランチの状態、計画書の記録との食い違い（プランの PR のとき）、再開の方法を示す（3.4）。続けて実行する選択肢を選んでいても、NEXT は実行しない |
| step 5 が片付け・`git fetch`・detach で止まった（NEXT がない） | 今と同じく、止まった理由を報告する |
| `実装モデル要件` の値がない | 止まらない。質問と報告の両方で、値がないことと分かる範囲の理由を示す（3.2） |

### 4.3. 副作用の契約

本タスクは、副作用を変えるフラグやモードを加えない。各選択肢を選んだときの外部への副作用（PRE_MERGE の commit と `git push`、`gh pr merge`、`gh issue develop` による次のブランチの作成、issue を閉じる操作、NEXT の実行）の種類と条件は変わらない（要件書 4.4）。

-   マージだけの選択肢は、今と同じく、NEXT を実行しない。NEXT が行う操作（ブランチへの commit、`git push`、`gh pr create` など）は、利用者が `/clear` の後に NEXT を実行したときに、そのコマンド自身の許可で行われる。
-   次のブランチは、どちらの選択肢でも、NEXT があれば step 5 が作る（AC-10）。マージだけの選択肢を選んだ利用者がその後 NEXT を実行しなくても、次のブランチは `origin` に残る。これは今の振る舞いと同じである。
-   止まったときに次のブランチの状態を確かめる操作（`git ls-remote` などの読み取り）は、外部に書き込まない。
-   最終報告は会話に示すだけで、外部に送らない。

### 4.4. 残るリスク

-   **推奨が戻っても検出されない。** 推奨と案内を固定するテストを加えないので（1.2）、後の変更で続けて実行する選択肢が推奨に戻っても、`make test` と CI の `doc-guard` は通る。変更は `runpr.md` の step 4 の 1 か所に集まっており（1.1）、PR のレビューで見つける前提とする。
-   **使われないブランチと issue が残りうる。** 次のことが起きると、`gh issue develop` が作ったブランチや、リンク用の子 issue が使われずに残る。ブランチは、1 つの issue にリンクできる数の上限（10 個。`_lib/task-pr-handoff.md` の「Choosing the issue to link」）のうち 1 つを使ったままになる。
    -   利用者が `/clear` の案内と違う worktree や、次のブランチから切り替えた後で NEXT を実行する（3.3 の項目 3 で防ぐ）。
    -   Part A step 4 で、リンク用の子 issue を作った後、親の issue への追加が失敗する。この子 issue は、最後のプランの PR で閉じる対象（`runpr.md:464-467`）に含まれない。
    -   これらの片付けは今の `/runpr` も行わず、本タスクの範囲外とする。マージだけの選択肢が推奨になり、最終報告と NEXT の実行の間が空くことが増えるので、起きる機会は増えうる。
-   **2 つの質問の推奨が逆を向く。** Escalation が `ask` のとき、PR レビューの質問は「まだマージしない」を、「Merge」の質問は「マージする」を推奨にする（1.4）。振る舞いは一意に決まるので、受け入れる。

## 5. セキュリティ考慮事項 (Security Considerations)

### 5.1. 脅威モデル

N/A。本タスクは外部コマンドの実行、秘密情報、キャッシュ、ネットワークへの送信、信頼できないテキストの埋め込みのいずれも変えない（要件書 4.2、`.claude/commands/_context.md` の「Conditional-guide trigger」に当たらない）。マージには、今と同じく利用者の明示の答えが要る（`runpr.md:370-371`）。新しく受け取る入力もない（要件書 3.2）。最終報告に示す NEXT と `実装モデル要件` は、`runpr.md` の step 2 と step 4 がすでに決めて示す値で、新しい入力の出どころではない。

### 5.2. 対象クライアント環境の検証

N/A。本タスクは Mattermost に送る記事の形を変えず、新しく頼る外部 API の機能もない。

## 6. 処理フロー詳細 (Processing Flow Details)

### 6.1. マージだけの選択肢を選んだときの流れ

NEXT が `/runplan` のプランの PR で、マージだけの選択肢を選んだときの流れを示す。実線の矢印は呼び出しまたは報告を、点線の矢印は応答を表す。

```mermaid
sequenceDiagram
    participant U as 利用者
    participant R as /runpr
    participant M as mergepr
    participant G as GitHub
    participant N as NEXT (/runplan)

    R->>U: マージの質問<br>(NEXT と実装モデル要件を示す)
    U-->>R: マージだけの選択肢
    R->>G: PRE_MERGE の commit を push
    R->>M: prepare・merge
    M->>G: squash マージ
    M-->>R: マージの結果
    R->>G: 次のブランチを作る (Part A step 4)
    alt 作り終えた
        R->>U: 最終報告 (/clear の案内: NEXT、次のブランチ、worktree、実装モデル要件)
        U->>U: /clear
        U->>N: 同じ worktree で NEXT を実行 (新しいセッション)
        N->>N: Part A step 3 で次のブランチを使う
    else 作り終える前に止まった
        R->>U: 最終報告 (止まった理由、次のブランチの状態、再開の方法)
    end
```

`/runpr` 自身は最終報告で終わる。NEXT は `/clear` の後の新しいセッションで動く別のコマンドである。

## 7. テスト戦略 (Test Strategy)

### 7.1. ユニットテスト

追加しない。本タスクは Go のコードと拡張のコードを変えない。文書のガードのテストも加えない（1.2）。`make test`・`make lint` と、CI の `doc-guard` のジョブ（`go test -tags test ./cmd/yt2column ./cmd/yt2column-server ./cmd/fixprguard ./internal/pipeline`）は、変更の前後で同じく通ることを確かめる（AC-15）。`fixpr_docs_test.go` が読む step 4 の見出しは変えず、`codex_review_docs_test.go` が読む step 5 の見出しの定数は見出しの変更に合わせて改める（1.4）。

### 7.2. 静的な確認

各 AC を、変更後のファイルの記述を読んで確かめる（7.5）。あわせて、`docs/tasks/` を除く追跡中のファイルに、次の記述が残っていないことを `git grep` で確かめる。`git grep` で確かめられるのは古い記述がないことだけで、新しい記述があることは読んで確かめる。

-   `/runpr` がマージの後に次のコマンドを必ず実行すると読める記述。3.5・3.6 が置き換える今の文言を、少なくとも含める: `and run the next command`（`runpr.md:2`、`CLAUDE.md:86`、`requirements_process.md:55`）、`and carry it out`（`runpr.md:31`）、`starts the next command`（`requirements_process.md:55`）、`the next command to run`（`_lib/task-pr-handoff.md:22-23`。行をまたぐので、改行を空白と同じに扱って検索する）、`pre-merge bookkeeping and the next command`（`mergepr.md:124`。3.5 で変えない `runpr.md:66` の文に一致しないよう、前の語を含める）、`then run NEXT`（`runpr.md:443`）。
-   続けて実行する選択肢に推奨を付ける記述（`run NEXT (recommended`）。

検索の語の一覧は、実装プランで確定する。

### 7.3. 統合テスト（手動）

本タスクのマージの後、次に `/runpr` を実行するときに確かめる。

-   NEXT があるタスク文書の PR で、マージの質問の「Merge」の 1 番目にマージだけの選択肢が推奨と示され、続けて実行する選択肢が推奨なしで示される（AC-01・AC-02）。
-   マージだけの選択肢を選ぶと、最終報告に `/clear` の案内（引数を含む NEXT、次のブランチ、worktree）が示される（AC-05）。NEXT が `/runplan` のときは `実装モデル要件` も示される（AC-06）。
-   NEXT がない PR（standalone change など）で、マージだけの選択肢が推奨と示され、最終報告に `/clear` の案内が載らない（AC-01・AC-08）。

step 5 が次のブランチを作り終える前に止まる場合（AC-07）は、意図して起こすと実際のリポジトリと issue に副作用が出るので、手動での確認は行わず、静的な確認で確かめる。

### 7.4. セキュリティテスト

N/A（5.1）。

### 7.5. 受け入れ基準と設計の対応

| AC | 設計 | 確かめ方 |
|---|---|---|
| AC-01〜AC-03 | 3.1 | 静的（記述の読み取り）・手動 |
| AC-04 | 3.2 | 静的（記述の読み取り） |
| AC-05・AC-06 | 3.2・3.3 | 静的（記述の読み取り）・手動 |
| AC-07 | 3.3・3.4 | 静的（記述の読み取り） |
| AC-08 | 3.3 | 静的（記述の読み取り）・手動 |
| AC-09・AC-10 | 3.3 | 静的（記述の読み取り） |
| AC-11 | 3.5 | 静的（記述の読み取り＋7.2 の `git grep`） |
| AC-12・AC-13 | 3.6 | 静的（記述の読み取り＋7.2 の `git grep`） |
| AC-14 | 3.6 | 静的（記述の読み取り） |
| AC-15 | 7.1 | `make test`・`make lint`・CI の `doc-guard` |

## 8. 実装優先順位 (Implementation Priorities)

すべての変更を 1 つの PR にまとめる。コマンドの定義はマージされた時点でそのまま次のセッションの振る舞いになるので、`runpr.md` と、それを説明する文書が食い違う状態を `main` に置かないためである。

1.  `runpr.md` の step 4 を改める（3.1・3.2）。推奨を定める唯一の場所なので、最初に置く。
2.  `runpr.md` の step 5 を改める（3.3・3.4）。
3.  `runpr.md` の frontmatter・冒頭・step 5 の見出しを改める（3.5）。
4.  ほかの文書（`CLAUDE.md`・`_lib/task-pr-handoff.md`・`requirements_process.md`・`mergepr.md`・`_context.md`）を改める（3.6）。
5.  7.2 の静的な確認を行い、`make test`・`make lint` と文書のガードを実行する。

**移行の注意:**

-   `/runpr` の定義は、利用者がコマンドを呼んだ時点の worktree の作業ツリーから読み込まれる。step 1 の `gh pr checkout` で読み直されることはない。一方、`_context.md` や `_lib/task-pr-handoff.md` は実行中にチェックアウトしたブランチから読まれる。そのため、本タスクの実装の PR で、改めた後の `runpr.md` のマージの質問が尋ねられるのは、その定義を含むチェックアウトから `/runpr` を呼んだときだけである。古い `main` の worktree から呼ぶと、古い定義と新しい文書が混じった実行になる。どちらの定義でもマージの振る舞いは変わらないので、マージには支障がない。
-   マージの後も古い定義のままの worktree やセッションでは、続けて実行する選択肢が推奨のまま示され、step 5 で止まったときの報告にも次のブランチの状態と再開の方法が載らない。マージの後は、ほかの worktree を `origin/main` に進め、セッションをやり直してから `/runpr` を実行する。この注意は、実装の PR の本文に書く。

## 9. 将来の拡張性 (Future Extensibility)

-   **推奨を変える場合:** `runpr.md` の step 4 の選択肢の一覧だけを改める（1.1）。ほかの文書は推奨に触れないので変えない。
-   **推奨を機械的に固定したくなった場合:** `TestRunprDocContract` に、step 4 の推奨と step 5 の案内を確かめるサブテストを加える（1.2 で採らなかった案）。step 5 を読むときは、`codex_review_docs_test.go` にある step 5 の見出しの定数を使う。
-   **使われないブランチや子 issue を片付ける場合:** 4.4 のリスクが実際に問題になれば、Part A step 4 が途中で失敗したときに作ったものを戻す手順を加える。本タスクの範囲外である。
-   **`/runpr` が自分で `/clear` 相当のことをする場合:** 要件書 2.3 のとおり本タスクの範囲外である。行うなら、NEXT を新しいセッションで始める手段が要り、step 5 の続けて実行する選択肢の扱いを改めることになる。
