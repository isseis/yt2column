# アーキテクチャ設計書：主要な設計の決定を ADR として docs/dev に記録する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-04 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（要件定義書。以下、要件書）の設計である。既存のファイルについての記述は、コミット `8749e5d` のファイルで確かめた。`file:line` は、このコミットでの行番号を指す。F-NNN・AC-NN は要件書の項番、H-NN は [design_handoff.md](design_handoff.md) の項番を指す。申し送りの H-01〜H-08 への対応は付録 A に、受け入れ基準との対応は付録 B にまとめる。調査の番号（A3・B1 など）は、要件書 6 章の定義のとおり、issue #602 のコメント（2026-11-04）で ADR の候補に付けた番号である。

本書は、要件書 6 章の用語（ADR、主題、主要な決定、索引、有効・廃止、経緯、仕様の正本の文書、調査の番号）を同じ意味で使う。そのほかに、次の用語を使う。

-   **規約:** ADR の書式と運用の規約の文書、`docs/dev/conventions/adr.md`（F-003。新しく作る。英語）。
-   **共通の手順:** `.claude/commands/_lib/settle-decisions.md`。`/mkreq`・`/mkarch`・`/mkplan`・`/runplan` が、未決の決定を人に確かめるときに従う手順である（同書 7〜13 行）。
-   **呼び出し元のコマンド:** 共通の手順に従うコマンド（`/mkreq`・`/mkarch`・`/mkplan`・`/runplan`）。
-   **ADR の guard:** 索引と ADR の名前・番号・節・状態を確かめる、新しい Go のテスト（3.6）。
-   **ADR-NNNN:** 番号が `NNNN` の ADR の呼び名。`NNNN` はファイル名の先頭の 4 桁の数字である。

## 1. 設計の全体像 (Design Overview)

### 1.1. 設計原則

-   **規則は 1 か所に書き、ほかの文書は参照する。** ADR の書式と運用は規約にだけ書く。CLAUDE.md・共通の手順・`mkarch.md`・`requirements_process.md` は規約を参照し、その内容を書き写さない（要件書 4.5、AC-23）。ただし、主要な決定の基準だけは、要件書の決定（要件書 4.5）に従って CLAUDE.md と規約の 2 か所に書く（3.3）。
-   **構造の定義は規約に置き、guard はそれを読む。** ADR の 6 つの節の見出し、状態の値、索引の列は、規約の雛形と表で定める。ADR の guard は、規約からこれらを読んで ADR と索引に照らす。テストに見出しの文字列を写して固定すると、規約を変えたときにテストと規約が食い違う（CLAUDE.md「Do not ... pin a literal copied into the test」）。
-   **ADR は生きた文書、タスク文書は変わらない記録。** ADR の本文には、今効いている決定だけを書く。決定が変わったら本文を書き換え、「経緯」に 1 項目を足す。タスク文書は、そのタスクで決めたことの記録として残し、本タスクでは書き換えない（AC-30）。
-   **仕様は正本の文書に残す。** ADR の「決定」の節は、正本の文書がある主題では、決定を要約して正本の文書へリンクする。理由・採らなかった案・経緯を持つのが ADR の役割である（AC-07）。
-   **既存の仕組みを使う。** 文書の中のリンクとパスが実在するかは、既存の guard（`cmd/yt2column/docpaths_test.go:140-160` の `TestInstructionDocPaths`）が確かめる。ADR の guard は、リンクが実在するかは確かめず、索引と ADR が互いに一致するかだけを確かめる。同じ値を 2 つのテストで確かめないためである（CLAUDE.md「Do not check the same value of the same document against the same source in more than one test」）。新しいパッケージと外部の依存は加えない。

### 1.2. 設計の際に決めたこと（2026-11-04）

利用者に確かめて決めたこと:

-   **F-006 の 15 件の主題は、主題ごとに 1 本の ADR にする（15 本。H-06）。** AC-25 は 1 本の ADR に 2 件以上の主題を入れることを許すが、まとめると、主題のどれか 1 つが変わるたびに大きな文書を書き換えることになり、廃止も主題の単位でできなくなる。主題ごとに 1 本にすれば、AC-25 の対応が一対一になり、更新と廃止も主題の単位で行える。B5（起動の経路）と B7（ビューアの開き方）のように内容が重なる主題は、ADR どうしのリンクで扱う。
-   **PR は 4 本に分け、F-007 の issue は 1 本目の PR の段階で作る（H-07）。** ①規約・手順・空の索引・ADR の guard、②LLM の ADR（A 系の 4 本）、③構成と拡張の ADR（B 系の 7 本）、④プロセスの ADR（D 系の 4 本）である。issue #602 のコメントの 3 本の案では、2 本目の PR に 11 本の ADR が入る。ADR は 1 本ごとに、出典のタスク文書と今のコードに照らして確かめる必要があるので（AC-26・AC-27）、1 本の PR としては大きすぎる。F-007 の issue を早く作るのは、0058・0059 が本タスクの途中で終わっても、後で足す経緯（AC-33）が失われないようにするためである。
-   **索引と ADR の整合は、恒久的な Go の guard で確かめる（H-02）。** これからの PR で、エージェントが ADR を足したり状態を変えたりする。索引の更新漏れは起こりやすい誤りであり、並行する worktree の 2 つの PR が同じ番号を取ることもある（H-06）。索引が食い違うと、「今どの決定が効いているかを一か所で分かるようにする」という目的（要件書 2.1）が果たせなくなる。本タスクの `_checks/` で 1 回だけ確かめる案では、これからの食い違いを見つけられないので採らない。
-   **ADR の雛形は、規約の文書の中のコードブロックに置く（H-06）。** `docs/dev/adr/` に雛形のファイルを置くと、ADR の guard がそれを ADR として数えるので、除外の規則が要る。

レビュアーに個別には確かめず、上の決定と現行の規則から導いたこと（PR のレビューで異論があれば改める）:

-   **規約の文書の名前は `docs/dev/conventions/adr.md` とする。** 同じディレクトリのほかの規約（`requirements_process.md`・`mermaid_reference.md`）と同じく、内容を表す短い名前にする。
-   **状態の値は、ADR の中では日本語の `有効`・`廃止` と書き、英語の規約では "active"・"obsolete" と呼ぶ（H-08）。** 規約は、値そのものを書くときは日本語の `有効`・`廃止` をコードスパンで書き、英語の呼び名を添える。"retired" は、翻訳の用語集で「廃止した LLM の変数」の訳に使われている（`docs/translation_glossary.md:149`）ので避ける。"withdrawn" はタスク文書の状態の値なので使わない。"superseded" も使わない。廃止は主題そのものが終わったときだけで、代わりの ADR があるとは限らないためである（要件書 1 章「要件定義の際に決めたこと」）。
-   **経緯の日付は、出典の文書の承認日とする（H-05）。** 3.2.4 で定める。
-   **タスク文書から ADR へは、`ADR-NNNN` を文字にした相対リンクで指す（H-04）。** 3.2.5 で定める。
-   **ADR の変更をコマンドの PR に入れる規則は共通の手順に書き、`mkreq.md` はコミットの対象の 1 文だけを直す（H-01）。** 3.4 で定める。
-   **CLAUDE.md の「Document language」の原則 (a) に `docs/dev/adr/` を足す。** 今の原則 (a) が挙げるのは「`docs/dev/` の直下の文書」（CLAUDE.md 42〜46 行）で、`docs/dev/adr/` の下の ADR はそれに当たらない。ADR と索引が日本語であること（AC-04）を、言語の規則の側でも読めるようにする。

### 1.3. なぜ既存の方法では足りないか

今の記録の置き場所（`requirements_process.md:96-108` の 5 つの置き場所、設計書の付録「決定の履歴」、`project_overview.md` の「決定済みの方針」）は、どれもタスクの単位か、方針の要約の単位である。タスクの単位の記録では、決定の理由と採らなかった案を知るのに、関係するタスクの文書をすべて探す必要がある。要約の単位の記録には、採らなかった案と経緯が入らない。主題の単位で今の決定と経緯を 1 つの文書に重ねるという要件（要件書 2.1 の 1 つ目と 2 つ目）は、既存の置き場所を広げても満たせない。

決定を記録する仕組みのほかの部分は、既存のものを使う。決定を人に確かめる段は共通の手順に足す（新しい手順書を作らない）。リンクの実在の確認は、既存の `TestInstructionDocPaths` に任せる。承認は PR のレビューとマージで行い、`/runpr` の承認の記録は広げない（要件書 1 章）。

### 1.4. 概念モデル

矢印 A → B は「A が B を参照する（リンクする、または読んで従う）」を表す。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef newpkg fill:#ffe8f5,stroke:#d946ef,stroke-width:2px,color:#701a75;

    CLAUDE[("CLAUDE.md")]
    CONV[("規約<br>docs/dev/conventions/adr.md")]
    INDEX[("索引<br>docs/dev/adr/README.md")]
    ADR[("ADR<br>docs/dev/adr/NNNN-#lt;kebab#gt;.md")]
    TASK[("タスク文書<br>docs/tasks/NNNN_#lt;name#gt;/")]
    SPEC[("仕様の正本の文書<br>security.md・production_profile.md など")]
    OVERVIEW[("project_overview.md")]
    SETTLE["共通の手順<br>_lib/settle-decisions.md"]

    CLAUDE -->|"規則と基準"| CONV
    CLAUDE -->|"Quick Links"| INDEX
    SETTLE -->|"段の中身"| CONV
    SETTLE -->|"関係する ADR を探す"| INDEX
    INDEX -->|"番号・題名・状態"| ADR
    ADR -->|"「決定」の要約の出典"| SPEC
    ADR -->|"「経緯」の出典"| TASK
    OVERVIEW -->|"理由と採らなかった案"| ADR
    TASK -.->|"主要な決定（本タスクの後に書く文書だけ）"| ADR

    class SPEC,TASK data
    class CLAUDE,OVERVIEW,SETTLE enhanced
    class CONV,INDEX,ADR newpkg

    subgraph Legend["Legend"]
        L1[("既存の文書（変えない）")]
        L2["変える文書・手順"]
        L3[("新しく作る文書")]
    end
    class L1 data
    class L2 enhanced
    class L3 newpkg
```

変える文書の一覧は 2.2 にある。点線の矢印は、本タスクの規則が `main` に入った後に書くタスク文書だけが持つリンクを表す。既存のタスク文書には足さない（AC-16・AC-30）。

ADR の状態は、次のように移る。矢印は状態の移りを表し、ラベルはその移りを起こす PR を表す。

```mermaid
stateDiagram-v2
    [*] --> 有効: 主要な決定を下した PR のマージ
    有効 --> 有効: 決定を変えた PR、または決定を下したタスクの取り下げ
    有効 --> 廃止: 主題が終わった PR、または ADR を作ったタスクの取り下げ
    廃止 --> [*]
```

「有効」から「有効」への移りでは、本文の「決定」の節を書き換え（取り下げなら前の決定に戻し）、「経緯」に 1 項目を足す（3.2.3）。廃止した ADR は削除しない（AC-10）。状態の図は Mermaid の `stateDiagram` で、classDef の色分けを使わないので、Legend を持たない。

## 2. システム構成 (System Structure)

### 2.1. 全体構成

本タスクが作る・変えるファイルと、それを読む者の関係を示す。矢印 A → B は「A が B を読む」を表す。

```mermaid
flowchart TD
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef newpkg fill:#ffe8f5,stroke:#d946ef,stroke-width:2px,color:#701a75;

    AGENT["エージェント<br>（/mkreq・/mkarch・/mkplan・/runplan）"]
    FIXPR["エージェント<br>（/fixpr・コマンドを通らない PR）"]
    HUMAN["人（レビュアー）"]
    GUARD["ADR の guard<br>cmd/yt2column"]
    PATHS["TestInstructionDocPaths<br>cmd/yt2column/docpaths_test.go"]

    CLAUDE[("CLAUDE.md")]
    SETTLE[("_lib/settle-decisions.md")]
    CONV[("docs/dev/conventions/adr.md")]
    INDEX[("docs/dev/adr/README.md")]
    ADR[("docs/dev/adr/NNNN-#lt;kebab#gt;.md")]

    AGENT --> SETTLE
    AGENT --> CLAUDE
    FIXPR --> CLAUDE
    SETTLE --> INDEX
    CLAUDE --> CONV
    HUMAN --> INDEX
    HUMAN --> ADR
    GUARD --> CONV
    GUARD --> INDEX
    GUARD --> ADR
    PATHS --> INDEX
    PATHS --> ADR
    PATHS --> CONV

    class AGENT,FIXPR,HUMAN,PATHS process
    class CLAUDE,SETTLE enhanced
    class CONV,INDEX,ADR newpkg
    class GUARD newpkg

    subgraph Legend["Legend"]
        L1["既存の読み手・テスト（変えない）"]
        L2[("変える文書")]
        L3[("新しく作る文書・テスト")]
    end
    class L1 process
    class L2 enhanced
    class L3 newpkg
```

`TestInstructionDocPaths` は、`docs/dev` の下のすべての `.md` を対象にする（`cmd/yt2column/docpaths_test.go:106-131` の `instructionDocs`。除くのは `readmeGuides` だけ）。そのため、`docs/dev/adr/` と `docs/dev/conventions/adr.md` は、追加の設定なしにリンクの確認の対象になる（H-03。3.7）。

### 2.2. コンポーネント配置

| ファイル | 種類 | 変更 | 担う要件 | PR |
|---|---|---|---|---|
| `docs/dev/conventions/adr.md` | 規約（英語） | 新規 | F-003（AC-12・AC-13）、F-001・F-002 の定義 | ① |
| `docs/dev/adr/README.md` | 索引（日本語） | 新規（① では行のない表） | AC-03・AC-04 | ①〜④ |
| `docs/dev/adr/0001-*.md`〜`0015-*.md` | ADR（日本語） | 新規 | F-002・F-006（AC-25〜AC-29） | ②〜④ |
| `CLAUDE.md` | 指示 | 変更（ADR の節、Quick Links、Document language） | F-004（AC-14〜AC-17） | ① |
| `docs/dev/conventions/requirements_process.md` | 規約 | 変更（96〜108 行の段落） | AC-18 | ① |
| `.claude/commands/_lib/settle-decisions.md` | 共通の手順 | 変更（段を足す） | F-005（AC-20〜AC-24） | ① |
| `.claude/commands/mkarch.md` | コマンドの定義 | 変更（74 行） | AC-19 | ① |
| `.claude/commands/mkreq.md` | コマンドの定義 | 変更（110〜112 行のコミットの対象） | AC-24（H-01） | ① |
| `cmd/yt2column/adr_test.go` | ADR の guard | 新規 | AC-01〜AC-03・AC-05・AC-06 の値と構造 | ① |
| `docs/dev/project_overview.md` | 方針の要約 | 変更（9〜16 行の 4 項目にリンクを足す） | AC-31 | ②・③ |
| `docs/tasks/0060_adr_records/_checks/` | 1 回だけの確認 | 新規 | AC-30 | 計画で決める |
| GitHub の issue（F-007） | issue | 新規 | AC-32・AC-33 | ① |
| GitHub の issue（AC-26 の食い違い） | issue | 見つけたときだけ新規 | AC-26 | ②〜④ |

変えないもの: `fixprguard` とその範囲の規則（3.4.3）、`/runpr`・`/fixpr`・`/mkplan`・`/runplan` の定義（3.4.2）、`docs/tasks/` の下の既存の文書（AC-30）、仕様の正本の文書（要件書 2.3）、`AGENTS.md`（自動レビュアーは CLAUDE.md に従うよう、すでに指示されている。`AGENTS.md:3`）。

この変更が振る舞いを変える既存のテストはない。`TestInstructionDocPaths` は、新しい文書のリンクとパスにも効くようになるが、テストのコードは変えない。

### 2.3. データフロー：呼び出し元のコマンドが ADR を作るか更新する

矢印は、呼び出しと応答、またはファイルの読み書きを表す。

```mermaid
sequenceDiagram
    participant C as 呼び出し元のコマンド
    participant S as 共通の手順
    participant I as 索引
    participant A as ADR
    participant H as 人（レビュアー）
    participant G as git（コマンドの PR のブランチ）

    C->>S: SCOPE と RECORD を渡す
    S->>I: 索引を読む
    S->>A: 関係する ADR を読む
    S->>H: 決定を尋ねる<br>（ADR を作るか更新するか、どの ADR かの判断を添える）
    H-->>S: 決定（判断を直すこともある）
    S->>C: RECORD に決定を記録する
    S->>A: 主要な決定なら ADR を作るか更新する<br>（索引も直す）
    C->>G: コマンドの文書と ADR を同じコミットに入れる
    C->>C: 自己レビュー・japrose の対象に ADR を含める
```

## 3. コンポーネント設計 (Component Design)

### 3.1. 置き場所・名前・番号（F-001・H-06）

-   **置き場所:** `docs/dev/adr/` の直下。このディレクトリには、ADR と索引の `README.md` のほかのファイルとサブディレクトリを置かない。
-   **名前:** `NNNN-<kebab>.md`。`NNNN` は `0001` から始まる 4 桁の連番、`<kebab>` は英小文字・数字を `-` でつないだ名前である（AC-01）。
-   **番号の割り当て:** 新しい ADR には、ADR を書く時点の `main` とその PR のブランチにある番号のうち、最も大きいものの次の番号を使う。番号は `0001` から抜けなく続く。廃止した ADR も削除しないので（AC-10）、番号は使い回されない（AC-02）。
-   **呼び名:** 文書の中では `ADR-NNNN` と呼ぶ。

**並行する PR が同じ番号を取ったとき（H-06）。** 索引の表は番号の順に並び、新しい行は表の末尾に足す。そのため、同じ番号を取った 2 つの PR は、索引の同じ位置に行を足すことになり、後からマージされる PR は `main` と衝突する。衝突した PR は CI が動かない（GitHub の衝突のある PR では `pull_request` のワークフローが起動しない）ので、`main` を取り込んで衝突を解く必要がある。その PR は、ADR のファイル名・見出しの番号・索引の行を、空いている次の番号に直す。取り込んだ後の CI では、ADR の guard（3.6）が、番号の重複と抜けを見つける。索引の行を末尾以外に足す誤りで衝突が起きなかった場合も、`main` を取り込んだ PR の CI で ADR の guard が見つける。

本タスクの 15 本は、調査の番号の順（A 系、B 系、D 系）に `0001`〜`0015` を割り当てる予定である。PR の順（②→③→④）と同じ順なので、ほかのタスクが間に ADR を足さない限り、この番号になる。ほかのタスクが先に番号を取ったときは、上の規則で次の番号を使う。番号の予定は次のとおりで、`<kebab>` の名前は実装計画で決める。

| 予定の番号 | 調査の番号 | 主題 | PR |
|---|---|---|---|
| 0001 | A3 | LLM のアダプタは標準ライブラリで書き、送信の経路と共有の型を決まったパッケージに置く | ② |
| 0002 | A6 | 生成の後の 2 段階 | ② |
| 0003 | A9 | LLM の設定の契約 | ② |
| 0004 | A10 | 本番の構成と品質の目標 | ② |
| 0005 | B1 | サーバの構成 | ③ |
| 0006 | B2 | 脅威モデル | ③ |
| 0007 | B4 | 字幕の入力は利用者の手動のコピー | ③ |
| 0008 | B5 | 起動の経路と送信の契機 | ③ |
| 0009 | B7 | ビューアの開き方とポップアップを閉じないこと | ③ |
| 0010 | B8 | 拡張は何も保存せず、権限を加えない | ③ |
| 0011 | B11 | 投稿先と投稿の契機 | ③ |
| 0012 | D1 | レビューの体制 | ④ |
| 0013 | D2 | 決定的な判定を Go の開発ツールに置く | ④ |
| 0014 | D3 | レビューのスレッドの本文を信頼しない | ④ |
| 0015 | D4 | 文書のテストの方針 | ④ |

**本タスクの途中で、ほかのタスクが同じ主題の ADR を作ったとき。** ① がマージされると、CLAUDE.md の規則（AC-14）は、進行中のほかのタスクにも効く（要件書 4.4）。たとえば 0058 が ③ より前に脅威モデル（B2）の決定を変えれば、0058 の PR が B2 の ADR を作ることがある。② 〜 ④ で ADR を書く前に索引を読み、その主題の有効な ADR がすでにあれば、新しく作らずにその ADR を足りない内容で更新する。1 つの主題は、有効な ADR のうち 1 本だけに属する（AC-25、要件書 6 章「主題」）。

### 3.2. ADR の書式（F-002）

#### 3.2.1. 雛形

規約は、次の雛形をコードブロックで示す。ADR の guard は、このコードブロックから見出しを読む（3.6）。雛形の見出しの文言と順序が、ADR の構造の定義になる。

```markdown
# ADR-NNNN: <題名>

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

## 決定

## 採らなかった案

## 結果

## 経緯
```

-   **題名（1 行目）:** `ADR-NNNN: ` に続く文が題名である。索引の題名の列と同じ文にする（AC-03）。
-   **状態:** 「状態」の節の表の `状態` の行に、`有効` か `廃止` を書く（AC-06）。廃止したときは、同じ表に `廃止の日付`・`廃止の理由`・`代わりの ADR` の 3 行を足す。`代わりの ADR` は、代わりの主題の ADR へのリンクか、`なし` である（AC-10）。状態を見出しではなく表のセルに書くのは、ADR の guard が日本語の値を表のセルとして読めるようにするためである（CLAUDE.md の guard-test literal exception は、表のセルの文字列を許す）。
-   **背景:** その主題で決定が要る理由と、決定を縛る前提を書く。
-   **決定:** 今効いている決定だけを書く。正本の文書がある主題では、決定を要約して正本の文書へリンクする（AC-07）。過去の決定は書かない。
-   **採らなかった案:** 案ごとに、採らなかった理由を書く。出典の文書に理由がない案は、「理由は記録されていない」と明記する（AC-08）。
-   **結果:** その決定が後のタスクや部品に課す制約と、受け入れた代償を書く。
-   **経緯:** 3.2.4 のとおり。

6 つの節（AC-05）の下には、`###` の小見出しを置いてよい。`##` の見出しは、雛形の 6 つだけにする。

#### 3.2.2. 状態の意味（AC-06・H-08）

規約は、状態を次の表で定める。ADR の guard は、この表の 1 列目を状態の値として読む（3.6）。

| 値 | 英語の呼び名 | 意味 |
|---|---|---|
| `有効` | active | 承認された決定である。実装を終えたかどうかは表さない |
| `廃止` | obsolete | 主題が終わり、決定がもう効いていない |

「承認された」は、その ADR を含む PR がレビューを経てマージされたことを指す（要件書 1 章）。ADR は Document Status の表を持たず、`/runpr` の承認の記録の対象にならない。

#### 3.2.3. 決定が変わったときと、タスクを取り下げたとき（AC-10・AC-13）

-   **決定が変わったとき:** 同じ ADR の「決定」の節を今の決定に書き換え、過去の決定を「採らなかった案」に移すかどうかを判断し、「経緯」に 1 項目を足す。新しい ADR を作って古い ADR を廃止することはしない。
-   **主題が終わったとき:** 状態を `廃止` にし、廃止の日付・理由・代わりの ADR を書き、「経緯」に 1 項目を足す。索引の状態の列も `廃止` にする。
-   **決定を下したタスクを取り下げたとき:** 取り下げの PR で、ADR の「決定」の節を前の決定に戻し、「経緯」に 1 項目を足す。そのタスクで新しく作った ADR なら、状態を `廃止` にする（削除しない）。

#### 3.2.4. 経緯の項目（AC-09・H-05）

経緯の各項目は、次の形の箇条書き 1 行にする。古い順に並べ、日付が同じなら出典のタスクの番号の順にする。

```markdown
- YYYY-MM-DD <その時点の決定または変更の要旨>（[<出典の文書の呼び名>](<出典の文書への相対リンク>)）
```

日付は、決定を記録した文書が承認された日とする。

-   **これから書く項目:** 決定を下した PR がマージされた日を書く。ADR はその PR に入るので、項目を書く時点ではまだマージされていない。書く時点の日付を書き、マージの日がそれと違っても直さない。承認の記録に日付の精密さは要らず、直す手順を足すほうが重いためである。
-   **既存の決定を ADR にするときの項目（本タスク）:** 次の順で、最初に見つかる日付を使う。
    1.  決定の節の見出しの日付（`実装の際に決めたこと（YYYY-MM-DD）` など）。その節の決定のための日付だからである。
    2.  出典の文書の変更履歴の承認の行の日付。2 回以上承認されている文書では、その決定を本文に書いた変更の後の、最初の承認の日付にする。
    3.  出典の文書の状態表の `Review date`。
    4.  上のどれもないとき：出典の文書を `main` に加えたコミットの日付（`git log --diff-filter=A`）。項目の要旨の後に「（日付は文書を加えたコミットの日付）」と書き添える。

承認日を使うのは、「有効」が承認された決定であることを表すからである（AC-06）。`Created` は、文書を書き始めた日で、決定が承認された日ではないので使わない。

#### 3.2.5. ADR へのリンクの書き方（AC-11・AC-16・H-04）

-   **ADR から:** 仕様の正本の文書とタスク文書へ、相対リンクで指す。行番号は書かない（AC-11）。節を指すときは、見出しの名前を文で書くか、アンカーを使う。
-   **タスク文書・`project_overview.md` から ADR へ:** `[ADR-NNNN](<相対パス>)` の形で指す。リンクの文字は `ADR-NNNN` とし、題名を添えるときはリンクの外に書く。番号はファイルの名前と一緒に変わらないので、呼び名とパスが食い違わない。題名をリンクの文字にすると、題名を直したときにリンクの文字が古くなる。

### 3.3. CLAUDE.md の変更（F-004）

CLAUDE.md の「## Documents」の章（22 行から）に、新しい小節「Design decisions (ADR)」を足す。中身は次のとおりで、どれも英語である。

-   **規則（AC-14）:** 主要な決定を下すか変える PR は、同じ PR で ADR を作るか更新する。この規則は、コマンドを通らない PR を含むすべての PR に効く。書き方は規約に従う（規約へのリンク）。
-   **主要な決定の基準（AC-15）:** 影響がそのタスクの外に及ぶこと（後のタスクや複数の部品を縛る、または覆すと複数の箇所が変わる）。例として、構成と部品の境界、プロトコル、外部の依存やサービス、セキュリティの前提、製品の範囲、開発のプロセスとツールの方針を挙げる。既存の ADR の決定を変える決定は、常に主要とする。1 つのタスクの中で閉じる決定の例（テストの名前、文言、1 つの関数の形、定数の値、実装の順序）も挙げる。
-   **分担（AC-16）:** 要件書 AC-16 の 5 項目を、そのまま規則として書く。タスク文書から ADR へのリンクは、この規則が `main` に入った後に書くタスク文書に求め、既存のタスク文書には足さないことも書く。
-   **Quick Links（AC-17）:** 「Project:」の一覧に、ADR の索引（`docs/dev/adr/README.md`）と規約（`docs/dev/conventions/adr.md`）を足す。索引の説明には、設計の前に今の決定と理由を探す入口であることを書く。
-   **Document language:** 原則 (a) の一覧に `docs/dev/adr/` の ADR と索引を足す（1.2）。原則 (b) の「`docs/dev/conventions/` の規約」には、規約の `adr.md` がそのまま含まれるので、書き足さない。

**基準を 2 か所に書くこと（要件書 4.5・H-02）。** 主要な決定の基準は、CLAUDE.md と規約の両方に書く。CLAUDE.md は毎回読まれるので基準を省けず、規約は ADR の書き方の一揃いを 1 つの文書で示すために基準を持つ（AC-13）。2 か所を同じ内容に保つ手段として、それぞれの基準の直後に「同じ基準が〈もう一方の文書〉にあり、変えるときは両方を同じ PR で変える」と書く。テストでは確かめない。文の意味の一致はテストで確かめられないためである（CLAUDE.md「Document tests check values, structure, and executable examples, never prose」）。PR のレビューで読んで確かめる。

### 3.4. コマンドの手順（F-005・H-01）

#### 3.4.1. 共通の手順に足す段

`.claude/commands/_lib/settle-decisions.md` の手順（21〜43 行）を、次のように変える。段の中身は規約を参照し、規約の内容（書式・状態・基準）を書き写さない（AC-23）。

| 位置 | 足す・変える内容 | AC |
|---|---|---|
| 手順 1 の前に新しい段 | 索引を読み、SCOPE の決定に関係する有効な ADR を探して読む。関係する ADR の今の決定は、未決の決定を挙げるときの入力（手順 1 の「the input」）に含める | AC-20 |
| 手順 2 | 背景の説明に、その決定で ADR を作るか、どの ADR を更新するか、主要でないので ADR にしないか、の判断とその理由を添える。主要な決定の基準は CLAUDE.md と規約を参照する | AC-21 |
| 手順 5 の後に新しい段 | 主要な決定なら、規約に従って ADR を作るか更新する（決定を変えること、廃止することを含む）。索引も同じ変更で直す。RECORD の記録から ADR へリンクする | AC-22 |
| 同じ段 | ADR と索引の変更は、呼び出し元のコマンドの PR に入れる。呼び出し元のコマンドが RECORD の文書をコミットするとき、同じコミットに ADR と索引を入れる。呼び出し元のコマンドの自己レビューのサブエージェントに渡すファイル（`_lib/review-subagent-pattern.md` の FILES）と、`japrose` の対象に、変えた ADR を含める | AC-24 |

「Inputs the calling command must supply」（15〜19 行）には入力を足さない。ADR の扱いは、どの呼び出し元でも同じだからである。

#### 3.4.2. 呼び出し元のコマンド

| コマンド | 変える箇所 | 理由 |
|---|---|---|
| `/mkreq` | 手順 6 のコミットの対象（`.claude/commands/mkreq.md:110-112` の「Commit the task directory」）を、タスクのディレクトリと、手順 5 で変えた ADR と索引に改める | コミットの対象をタスクのディレクトリと明記しているので、共通の手順の段だけでは、エージェントが ADR を外してコミットしうる |
| `/mkarch` | 変えない | 手順 7.5 は「Commit the first version」で、対象のファイルを限っていない。共通の手順の段が、同じコミットに ADR を入れることを求める |
| `/mkplan` | 変えない | 手順 7.5 は `/mkarch` と同じ形である |
| `/runplan` | 変えない | 決定の変更は手順 5 の「Decision change」で共通の手順に従い、変えた文書を同じコミットに入れる（`.claude/commands/runplan.md:64`）。ADR もその段で同じコミットに入る |
| `/mkplan2` | 変えない | PR の境界を足すだけで、共通の手順を使わない |

`/mkarch` の手順 7 の最後の箇条（`.claude/commands/mkarch.md:74`）は、AC-19 のとおり、付録「決定の履歴」に置くものを、そのタスクの中で閉じる決定の経緯に限り、主要な決定のタスクをまたぐ経緯は ADR に置く、と改める。本文を今の仕組みの記述に保つという、この箇条の今の指示は残す。

#### 3.4.3. `/fixpr` とコマンドを通らない PR

`/fixpr` が PR のレビューの後に主要な決定を改めるときと、タスクに属さない単独の PR は、共通の手順を通らない。これらには CLAUDE.md の規則（AC-14）が効く。

`/fixpr` が ADR を変えるとき、`fixprguard inspect` の範囲の規則（`docs/dev/guides/fixprguard_guide.md` 6 章）では、次のようになる。

-   スレッドの `path` がタスク文書（`docs/tasks/NNNN_<name>/` の下）のとき、`docs/dev/adr/` の ADR は、規則 2（同じディレクトリ）にも規則 3（同じタスクのディレクトリ）にも当たらないので、範囲外になる。`inspect` は終了コード `3` で範囲外の変更を示し、利用者が確かめてから続ける。
-   スレッドの `path` が ADR か索引のとき、同じディレクトリの `.md` は、規則 2 と許可リストの (a) で範囲に入る。

範囲の規則は変えない。主要な決定を `/fixpr` で改めることはまれで、そのときに利用者が範囲外の変更として ADR の変更を確かめることは、決定を人に示すという要件（要件書 1 章「主要かどうかはエージェントが判断し、人に示す」）にも合う。範囲を広げると、レビューのスレッドを介して ADR を変える経路が、利用者の確認なしに開く。

### 3.5. 索引と、ほかの文書の変更

#### 3.5.1. 索引（`docs/dev/adr/README.md`）

索引は、短い説明と 1 つの表からなる。説明には、索引が今どの決定が効いているかを示すこと、書き方は規約にあることを書く。表は次の形で、番号の順に並べる。

```markdown
| 番号 | 題名 | 状態 |
|---|---|---|
| [ADR-0001](0001-<kebab>.md) | <ADR の 1 行目の題名> | `有効` |
```

規約は、この表の見出しの行をコードブロックで示す。ADR の guard は、規約から列の名前を読み、索引の表を探す（3.6）。① の時点の索引は、見出しと区切りの行だけを持ち、行を持たない。

#### 3.5.2. `requirements_process.md`（AC-18）

96〜108 行の段落「Five places hold a record about a task document」は、タスク文書についての記録の置き場所を挙げている。ADR は、タスク文書そのものについての記録ではなく、タスクをまたぐ主題についての記録だが、エージェントが置き場所を選ぶときに見落とさないように、同じ段落に 6 つ目として挙げる。段落の冒頭の数を「Six places」に改め、ADR に置くもの（主要な決定の今の形、理由、採らなかった案、タスクをまたぐ経緯）と、規約へのリンクを足す。

#### 3.5.3. `project_overview.md`（AC-31）

「決定済みの方針」（9〜16 行）のうち、AC-31 の表の 4 項目の末尾に、`[ADR-NNNN](adr/NNNN-<kebab>.md)` の形のリンクを足す。項目の要約の文は変えない。リンクは、リンク先の ADR を作る PR で足す。

| 項目 | リンク先 | PR |
|---|---|---|
| 実行形式 | B1・B5 の ADR | ③ |
| 字幕取得 | B4 の ADR | ③ |
| LLM（既定は Claude） | A3・A10 の ADR | ② |
| LLM プロバイダは差し替え可能にする | A3 の ADR | ② |

`project_overview.md` は `docs/dev` の直下にあり、`TestInstructionDocPaths` の対象である。リンクをリンク先より先の PR で足すと、そのテストが失敗する（3.7）。

### 3.6. ADR の guard（H-02）

`cmd/yt2column/adr_test.go` に、ADR と索引の値と構造を確かめるテストを置く。`cmd/yt2column` のテストは、CI の `doc-guard` ジョブで、変更の種類にかかわらずすべての PR で実行される（`.github/workflows/ci.yml:173-187`）。文書だけの PR でも、索引の食い違いを見つけられる。

確かめること:

| 確かめること | 定義の出典 | AC |
|---|---|---|
| `docs/dev/adr/` の直下のファイルは、`README.md` と、名前が `NNNN-<kebab>.md` のファイルだけで、サブディレクトリがない | 名前の形（テストの正規表現。規約の定義と同じ形） | AC-01 |
| 番号に重複がなく、`0001` から抜けなく続く | 同上 | AC-02 |
| 各 ADR の 1 行目が `# ADR-NNNN: <題名>` で、`NNNN` がファイル名の番号と同じ | 規約の雛形 | AC-01・AC-03 |
| 索引の表の行と ADR のファイルが一対一に対応する。各行のリンク先が、その番号の ADR のファイル名と同じ | 規約の索引の見出しの行 | AC-03 |
| 索引の各行の題名と状態が、その ADR の題名と状態に一致する | 同上 | AC-03 |
| 各 ADR の `##` の見出しが、雛形の `##` の見出しと同じ文言・同じ順序である | 規約の雛形 | AC-05 |
| 各 ADR の「状態」の節の表の `状態` の行の値が、規約の状態の表の値のどれかである | 規約の状態の表 | AC-06 |

**定義を規約から読む。** 雛形の見出し、状態の値、索引の列の名前は、規約の中の決まった英語の見出しの下にある、コードブロックと表から読む。テストが持つ日本語の文字列は、表を探すための列の名前（`状態` など）に限り、CLAUDE.md の guard-test literal exception の範囲（見出し・表のセル・列の名前）に収める。

**読めないときは失敗にする。** 規約の雛形のコードブロック、状態の表、索引の表のどれかが見つからない、または空のときは、テストを飛ばさずに失敗にする。読めないことを「確かめることがない」と扱うと、規約や索引の形を壊したときに guard が黙って効かなくなるためである。索引の表に行がないこと（① の時点）は、ADR のファイルもないときに限って一致とみなす。

**失敗の報告。** 各失敗は、ファイルのパスと、食い違いの種類（名前の形、番号の重複、番号の抜け、索引にない ADR、ファイルのない索引の行、題名の不一致、状態の不一致、見出しの不一致、状態の値）を述べる。直す人が、どのファイルのどこを直すかを報告だけで分かるようにする。

**確かめないこと。** リンク先が実在するか（`TestInstructionDocPaths` が確かめる）。節の中の文の内容、日本語で書かれているか（AC-04）、経緯の項目の形と日付、採らなかった案の出典（AC-08・AC-09・AC-27）、廃止したときの 3 行（AC-10）。これらは文の意味か、誤りの害が小さい書式で、PR のレビューで読んで確かめる（CLAUDE.md「Do not add a document check whose mistake is unlikely, whose harm is small, or whose upkeep costs more than the mistake it prevents」）。

**恒久のテストにする理由。** CLAUDE.md の文書のテストの方針は、値と構造の確認を許す（「A test that reads a document may compare a value in it with the code or configuration that defines it ... check the heading structure」）。ADR の guard が確かめるのは、索引と ADR の値の一致と、規約が定める見出しの構造で、方針の許す範囲に入る。廃止した仕様を文書に残させるテストでもない。

### 3.7. 既存のリンクの guard との関係（H-03）

`TestInstructionDocPaths`（`cmd/yt2column/docpaths_test.go:140-160`）は、`CLAUDE.md`・`AGENTS.md`・`.claude/commands` と `docs/dev` の下の `.md` について、相対リンクの先と、コードスパンに書いた `docs/dev/` の下のパスが実在することを確かめる（同 101〜131 行）。対象を決めるのはディレクトリの走査なので、次のファイルは追加の設定なしに対象になる。

-   `docs/dev/conventions/adr.md`、`docs/dev/adr/README.md`、`docs/dev/adr/NNNN-*.md`。
-   変える `CLAUDE.md`・`settle-decisions.md`・`mkarch.md`・`mkreq.md`・`requirements_process.md`・`project_overview.md`。

`docs/tasks/` の下の文書は対象外である（同 103〜104 行のコメント）。そのため、ADR からタスク文書へのリンクは確かめられるが、これから書くタスク文書から ADR へのリンクは確かめられない（5.2）。

PR の中の順序の規則:

-   リンク先のファイルを作る PR より前の PR で、リンクを書かない。CLAUDE.md の Quick Links と規約からの索引へのリンクは、索引を作る ① に入れる。`project_overview.md` から ADR へのリンクは、その ADR を作る ② か ③ に入れる（3.5.3）。
-   規約・CLAUDE.md・共通の手順の中で、ADR のファイル名の例を書くときは、`<kebab>` などの山括弧の仮の名前か、コードブロックの中に書く。`TestInstructionDocPaths` は、`<` で始まるリンク先とコードブロックの中のリンクを例として扱う（同 34〜37 行・69〜78 行）。
-   コミットの単位では、リンクとリンク先を同じコミットに入れるか、リンク先を先にコミットする。各コミットで `make test` が通るようにするためである。

## 4. エラーハンドリング設計 (Error Handling Design)

本タスクは Go の製品コードを変えないので、エラー型を加えない。テストの失敗の報告は 3.6 で定めた。

手順の中の失敗の扱い:

| 場面 | 扱い |
|---|---|
| 共通の手順で索引が読めない（ファイルがない、表が見つからない） | 関係する ADR がないとみなさずに止まり、利用者に報告する。① がマージされた後は索引が必ずあるので、読めないことは誤りである |
| 主要かどうかの判断が人と違った | 人の判断に従い、RECORD と ADR をそれに合わせる（要件書 1 章） |
| ADR を書く時点で、正本の文書とコードが食い違っている（AC-26） | ADR にはコードの振る舞いを書き、食い違いを GitHub の issue にし、ADR の「決定」の節にその食い違いと issue へのリンクを書く。issue の作成はネットワークの操作なので、その都度利用者の承認を得る（CLAUDE.md「Tool Execution Safety」） |
| 出典の文書に採らなかった案の理由がない | 理由を補わずに、「理由は記録されていない」と書く（AC-08・AC-27） |

## 5. セキュリティ考慮事項 (Security Considerations)

### 5.1. 脅威モデル

N/A。本タスクは、文書とコマンドの定義と、文書を読むテストだけを変える。外部コマンドの起動、API キーや Webhook URL の扱い、キャッシュ、ネットワークへの送信、信頼しない文字列のプロンプトや記事への埋め込みのどれも変えない（要件書 4.2）。

### 5.2. 残余のリスクと注意

-   **ADR はエージェントが読む資料になる。** 共通の手順は、エージェントに索引と ADR を読ませる。ADR は PR のレビューを経てマージされたリポジトリの文書で、エージェントがすでに読んでいるタスク文書と同じ信頼の範囲にある。ADR は手順に従う「指示」ではなく、決定を知るための「資料」として読ませる（CLAUDE.md「Document language」の原則 (b) の、指示と資料の区別）。
-   **`/fixpr` の範囲外の確認。** 3.4.3 のとおり、`/fixpr` が ADR を変えると、利用者の確認に回る。範囲の規則を広げないので、レビューのスレッドを介して ADR が変わる経路は増えない。
-   **タスク文書から ADR へのリンクは、guard で確かめられない。** `TestInstructionDocPaths` は `docs/tasks/` を対象にしない（3.7）。ADR の番号と名前は変えない前提なので、リンクが切れるのは ADR の番号を直したとき（3.1 の衝突の解決）に限られる。その場合は、番号を直す PR の中で、同じ PR のタスク文書のリンクも直す。
-   **同じ番号が `main` に入る経路。** 2 つの PR が、どちらも `main` を取り込む前に CI を通り、索引の衝突もなくマージされた場合に限る。索引の行は末尾に足すので、ふつうは衝突が起きる（3.1）。そうならずに入った場合は、次に `main` を取り込む PR の CI で ADR の guard が失敗し、その PR で番号を直す。

## 6. 処理フロー詳細 (Processing Flow Details)

### 6.1. 共通の手順での ADR の扱い

矢印は、手順の進む向きを表す。

```mermaid
flowchart TD
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    START(["呼び出し元のコマンドが共通の手順に入る"])
    READ["索引を読み、関係する ADR を読む（新しい段）"]
    LIST["未決の決定を挙げる（手順 1）"]
    ASK["背景と ADR の判断を添えて人に尋ねる（手順 2・3）"]
    RECORD["RECORD に決定を記録する（手順 5）"]
    MAJOR{"主要な決定か"}
    EXIST{"関係する有効な ADR があるか"}
    UPDATE["ADR を更新する<br>（決定の書き換え、または廃止。経緯に 1 項目）"]
    CREATE["ADR を作る<br>（次の番号。索引に行を足す）"]
    COMMIT["RECORD の文書と同じコミットに<br>ADR と索引を入れる"]
    END(["呼び出し元のコマンドに戻る"])

    START --> READ --> LIST --> ASK --> RECORD --> MAJOR
    MAJOR -->|"いいえ"| END
    MAJOR -->|"はい"| EXIST
    EXIST -->|"ある"| UPDATE --> COMMIT
    EXIST -->|"ない"| CREATE --> COMMIT
    COMMIT --> END

    class LIST,ASK,RECORD process
    class READ,MAJOR,EXIST,UPDATE,CREATE,COMMIT enhanced

    subgraph Legend["Legend"]
        L1["今の手順の段"]
        L2["足す段"]
    end
    class L1 process
    class L2 enhanced
```

「主要な決定か」の判断は、手順 2 で人に示した判断を、人の答えで直したものである。

### 6.2. 既存の決定を ADR にする（② 〜 ④）

1 本の ADR を書くときの流れを示す。矢印は作業の進む向きを表す。

```mermaid
flowchart TD
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    SRC[("出典のタスク文書<br>（F-006 の表の括弧のないもの）")]
    CODE[("main のコードと正本の文書")]
    IDX{"索引に、その主題の<br>有効な ADR があるか"}
    WRITE["ADR を書く<br>（決定・採らなかった案・経緯）"]
    EXTEND["既存の ADR を更新する"]
    CHECK{"決定がコードと食い違うか"}
    ISSUE["食い違いの issue を作り<br>「決定」の節にリンクする"]
    DONE["索引に行を足す・状態を合わせる"]

    SRC --> IDX
    IDX -->|"ない"| WRITE
    IDX -->|"ある"| EXTEND
    WRITE --> CHECK
    EXTEND --> CHECK
    CODE --> CHECK
    CHECK -->|"食い違う"| ISSUE --> DONE
    CHECK -->|"食い違わない"| DONE

    class SRC,CODE data
    class IDX,WRITE,EXTEND,CHECK,ISSUE,DONE enhanced

    subgraph Legend["Legend"]
        L1[("読む資料")]
        L2["本タスクの作業"]
    end
    class L1 data
    class L2 enhanced
```

B7 の ADR は、`docs/tasks/0050_viewer_tab/02_architecture.md` の付録 B（B.1〜B.3）の採らなかった案をすべて写し、付録 B へリンクする（AC-29）。B2・B8・B11 は、括弧に入れた 0058・0059 を経緯に入れず、その ADR を書く時点の `main` のコードと文書の状態で書く（要件書 1 章）。

## 7. テスト戦略 (Test Strategy)

### 7.1. ユニットテスト

ADR の guard のうち、規約・索引・ADR を解析する部分は、文字列を入力にした表のテストで確かめる。入力は、正しい形と、3.6 の食い違いの種類ごとに 1 つずつ壊した形である。規約の雛形や表が見つからない入力で、テストが飛ばさずに失敗することも確かめる。

### 7.2. 壊して失敗を確かめる

CLAUDE.md「Every test must be able to fail for its stated reason」に従い、ADR の guard の各確認について、実際の文書を一時的に壊して失敗することを確かめ、コミットメッセージに書く。壊し方の例は、索引の行を 1 つ消す、ADR の状態の値を変える、`##` の見出しを 1 つ変える、番号を重複させる、規約の雛形のコードブロックの見出しを変える、である。具体的な手順は実装計画で決める。

### 7.3. 1 回だけの確認

-   **AC-30:** 本タスクの各 PR で、マージの基点と PR の先頭を比べ、`docs/tasks/` の下の既存の文書（`docs/tasks/0060_adr_records/` を除く）が変わっていないことを確かめる。前後の関係を述べる受け入れ基準なので、`docs/tasks/0060_adr_records/_checks/` のスクリプトで行う（`requirements_process.md` §4 の `diff`）。
-   **AC-31 の「項目の要約の文は残っている」:** 前後の関係なので、同じく `_checks/` のスクリプトで、変更の前後の 4 項目の文を比べる（リンクを除いて同じであること）。

### 7.4. 読んで確かめる（manual）

文の内容だけを定める受け入れ基準（AC-04・AC-07〜AC-09・AC-11・AC-13〜AC-16・AC-18〜AC-23・AC-26〜AC-29・AC-33）は、PR のレビューで読んで確かめる。AC-26 と AC-27 は、ADR の各文の出典を PR の説明に挙げて、レビュアーが照らせるようにする。

### 7.5. セキュリティテスト

N/A（5.1）。

## 8. 実装の優先順位 (Implementation Priorities)

| PR | 中身 | 前提 |
|---|---|---|
| ① 規約と手順 | 規約、空の索引、ADR の guard、CLAUDE.md、`requirements_process.md`、共通の手順、`mkarch.md`、`mkreq.md`。F-007 の issue の作成（調査の 35 件のうち F-006 にない 20 件と、AC-33 の 2 点） | なし |
| ② LLM の ADR | A3・A6・A9・A10 の 4 本、索引の 4 行、`project_overview.md` の LLM の 2 項目のリンク | ① のマージ |
| ③ 構成と拡張の ADR | B1・B2・B4・B5・B7・B8・B11 の 7 本、索引の 7 行、`project_overview.md` の実行形式と字幕取得の 2 項目のリンク | ① のマージ |
| ④ プロセスの ADR | D1〜D4 の 4 本、索引の 4 行 | ① のマージ |

-   ① の中では、ADR の guard を、規約と索引と同じコミットか、その後に入れる。guard が読む規約の雛形と表が先にないと、guard が失敗するためである。
-   ② 〜 ④ は、どれも ① のマージを前提とし、互いには依存しない。番号の予定（3.1）を保つため、② → ③ → ④ の順に進める。
-   ① の F-007 の issue は、issue #602 のコメントにある調査の 35 件を読んで作る。issue の作成はネットワークの操作なので、利用者の承認を得て行う。
-   ① は CLAUDE.md と `.claude/commands/` を変えるので、`fixprguard` の実行関連ファイル（`docs/dev/guides/fixprguard_guide.md` 6 章）に当たる。① の PR で `/fixpr` がそれらを直すときは、毎回利用者の確認に回る。

## 9. 将来の拡張 (Future Extensibility)

-   **残りの候補の ADR:** F-007 の issue で、残りの 20 件（B6 を含む）を ADR にする。B2・B8・B11 の経緯に、0058・0059 の決定を足す。
-   **ADR の運用そのものの ADR:** 本タスクの決定（ADR を生きた文書にする、状態を 2 つにする、など）は、開発のプロセスの方針として、主要な決定の基準に当たる。ただし、本タスクの決定は ① より前に下したので、CLAUDE.md の規則（AC-14）の対象ではなく、F-006 にも入っていない。今の仕様の正本は規約と CLAUDE.md にあり、理由は本タスクの文書にある。後でこの方針を変えるタスクが、その時点で ADR を作る。
-   **ADR の guard の確認を足す:** 経緯の項目の形や、廃止した ADR の 3 行の有無を確かめたくなったら、規約に定義を置いて guard がそれを読む形で足す。今は誤りの害が小さいので足さない（3.6）。

## 付録 A: 申し送り（design_handoff.md）への対応

| 項目 | 対応 | 本書の箇所 |
|---|---|---|
| H-01 コマンドが ADR の変更を自分の PR に含める手段 | 共通の手順の新しい段に、RECORD の文書と同じコミットに ADR を入れること、自己レビューの FILES と `japrose` の対象に ADR を含めることを書く。`mkreq.md` はコミットの対象を明記しているので、その 1 文を直す。`/mkarch`・`/mkplan`・`/runplan` は対象を限っていないので変えない。`/fixpr` では ADR の変更が範囲外になり、利用者の確認に回る。範囲の規則は変えない | 3.4 |
| H-02 索引と ADR の整合を確かめる手段 | 恒久的な Go の guard（`cmd/yt2column/adr_test.go`）。定義は規約から読む。基準を 2 か所で同じに保つ手段は、相互の参照の文と PR のレビュー | 1.2、3.3、3.6 |
| H-03 既存の文書のパスを確かめる guard との関係 | ADR・索引・規約・変える文書は、すべて `TestInstructionDocPaths` の対象になる。リンク先を作る PR より前にリンクを書かない。コミットの単位でも順序を守る | 3.7 |
| H-04 タスク文書から ADR を指す書き方 | `[ADR-NNNN](<相対パス>)`。題名はリンクの外に添える | 3.2.5 |
| H-05 移行する ADR の「経緯」の日付 | 決定の節の見出しの日付、変更履歴の承認の日付、`Review date`、文書を加えたコミットの日付、の順に使う。最後の場合はその旨を書き添える | 3.2.4 |
| H-06 番号の割り当てと雛形 | 主題ごとに 1 本、調査の番号の順に `0001`〜`0015` を予定する。番号は次の空き番号で、抜けなく続ける。衝突は索引の衝突と ADR の guard で見つけ、後の PR が番号を直す。雛形は規約のコードブロックに置く | 1.2、3.1、3.2.1 |
| H-07 PR の分け方 | 4 本（①規約と手順、②A 系、③B 系、④D 系）。F-007 の issue は ① で作る | 1.2、8 |
| H-08 規約での状態の英語の呼び名 | ADR の中の値は `有効`・`廃止`。英語の規約では "active"・"obsolete" と呼ぶ | 1.2、3.2.2 |

## 付録 B: 受け入れ基準との対応

| AC | 設計の箇所 | 確かめ方の種類（詳細は実装計画） |
|---|---|---|
| AC-01 | 3.1、3.6 | test（ADR の guard） |
| AC-02 | 3.1、3.6 | test（ADR の guard） |
| AC-03 | 3.5.1、3.6 | test（ADR の guard） |
| AC-04 | 3.2、3.3（Document language） | manual |
| AC-05 | 3.2.1、3.6 | test（ADR の guard） |
| AC-06 | 3.2.2、3.6 | test（値）、manual（意味の文） |
| AC-07 | 3.2.1 | manual |
| AC-08 | 3.2.1 | manual |
| AC-09 | 3.2.4 | manual |
| AC-10 | 3.2.1、3.2.3 | manual |
| AC-11 | 3.2.5 | manual（リンクの実在は `TestInstructionDocPaths`） |
| AC-12 | 2.2 | test（`TestInstructionDocPaths` と ADR の guard が規約を読む）、manual（英語） |
| AC-13 | 3.2、3.3 | manual |
| AC-14 | 3.3 | manual |
| AC-15 | 3.3 | manual |
| AC-16 | 3.3 | manual |
| AC-17 | 3.3 | test（`TestInstructionDocPaths` がリンクの実在を確かめる）、manual |
| AC-18 | 3.5.2 | manual |
| AC-19 | 3.4.2 | manual |
| AC-20 | 3.4.1 | manual |
| AC-21 | 3.4.1 | manual |
| AC-22 | 3.4.1 | manual |
| AC-23 | 1.1、3.4.1 | manual |
| AC-24 | 3.4.1、3.4.2 | manual |
| AC-25 | 1.2、3.1 | manual（主題と ADR の対応表を実装計画に置く） |
| AC-26 | 4、6.2 | manual |
| AC-27 | 6.2、7.4 | manual |
| AC-28 | 6.2 | manual |
| AC-29 | 6.2 | manual |
| AC-30 | 7.3 | diff（`_checks/`） |
| AC-31 | 3.5.3、7.3 | diff（`_checks/`）、test（`TestInstructionDocPaths` がリンクの実在を確かめる） |
| AC-32 | 8 | manual（issue の確認） |
| AC-33 | 8 | manual（issue の確認） |
