# 設計書：文書の言語を読み手で決める方針への整理と、README の日英二言語化

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-28 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（`approved`）の設計である。要件定義から申し送られた設計事項は [design_handoff.md](design_handoff.md) にあり、各項目への対応は 10 章にまとめる。本書が引用するファイルの行番号は、特に断らない限り、コミット `364c95b8`（本書の作成時の `main`）のものである。

## 1. 設計の全体像 (Design Overview)

### 1.1. 設計原則

-   **言語は読み手と承認者で決まる。** 文書の言語は、場所ごとの例外ではなく、要件 AC-01 の 3 つの原則で決める。置き場所の規則（`docs/dev/conventions/` は英語、`docs/dev/guides/` は日本語）は、原則を当てはめた結果である。言語の原則の例外は、二言語で持つ README と、コマンドの定義がエージェントに日本語の手引きを読ませることの 2 つだけである。ほかに、`README.en.md` をリポジトリの直下に置く置き場所の例外が 1 つある。どれも CLAUDE.md に範囲と理由とともに書く。
-   **正本は 1 つ。** 二言語の README では `README.md`（日本語）が正本であり、`README.en.md` はその翻訳である。以下、`README.md` と `README.en.md` を合わせて「両版」と呼ぶ。`README.en.md` がどの時点の内容の `README.md` から翻訳されたかは、`README.en.md` の先頭の同期の印（翻訳の元にした `README.md` のブロブの ID を書いた HTML のコメント。2.4 節）が示す。手引きは日本語版の手引きに一本化し、英語版の手引きは持たない。
-   **値と構造はテストで、文の意味は読み合わせで確かめる。** ガードテストは、値（変数名、終了コード、`reason` など）と構造（見出しの並び、コードブロックの数）を、その値を持つ文書で検査する。振り分けや翻訳の内容など文の意味は、PR のレビューの読み合わせ（`manual`）で確かめる（要件 §5）。
-   **ロケータは 1 度だけ定義する。** ガードテストが節や表を探すための見出しと列名（以下「ロケータ」）は、Go ではテスト専用の共有パッケージに、TypeScript では拡張のテストの補助ファイルに、1 度だけ定義する。README のロケータは日本語と英語の組で持つ。
-   **既存の仕組みを使う。** リンクの検査とコードブロックの走査は、既存の補助関数（`cmd/yt2column/docs_test.go:53-700`）を共有パッケージに移して使う。範囲の判定は `fixprguard` の既存の規則のままで足りる（2.5 節）。CI の判定は既存の `extension/scripts/has-extension-changes.sh` に行を加えるだけにする。新しい CI のジョブは作らない。

### 1.2. 概念モデル

図 1-1 は、本タスクの後の文書と、文書どうしの関係を示す。矢印 A → B は「A が B を参照する、または A の内容が B に渡る」を表す。ラベルで関係の種類を示す。

```mermaid
flowchart LR
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef newpkg fill:#ffe8f5,stroke:#d946ef,stroke-width:2px,color:#701a75;

    RJA[("README.md<br>(正本・日本語)")]
    REN[("README.en.md<br>(英語版)")]
    CLI[("docs/dev/guides/cli.md")]
    DEV[("docs/dev/guides/development.md")]
    GUIDES[("docs/dev/guides/<br>手引き 4 つ")]
    CONV[("docs/dev/conventions/<br>規約 5 つ")]
    CMDS[("CLAUDE.md・AGENTS.md・<br>.claude/commands/")]
    MKTRANS["/mktrans"]

    RJA -->|"翻訳の元"| MKTRANS
    MKTRANS -->|"差分翻訳と同期の印"| REN
    RJA -->|"開発者向けの文書への案内"| CLI
    RJA -->|"開発者向けの文書への案内"| DEV
    CLI -->|"設定の節への参照"| RJA
    CMDS -->|"従わせる"| CONV
    CMDS -->|"読ませる (原則 (b) の例外)"| GUIDES

    class RJA,CMDS,MKTRANS enhanced
    class REN,CLI,DEV,GUIDES,CONV newpkg
```

**図 1-1 の凡例**

```mermaid
flowchart LR
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef newpkg fill:#ffe8f5,stroke:#d946ef,stroke-width:2px,color:#701a75;
    subgraph Legend["Legend"]
        L1[("本タスクで変える既存の文書")]
        L2["本タスクで変える既存のコマンド"]
        L3[("本タスクで新設する、または移動する文書")]
    end
    class L1,L2 enhanced
    class L3 newpkg
```

文書の分類と言語は、要件 AC-06 の表のとおりである。本タスクが CLAUDE.md に書く原則と、その当てはめは次のとおりである。

| 原則 | 当てはまる文書 | 言語 |
|---|---|---|
| (a) 人間が判断・承認する文書と、人間向けの手引き | `docs/dev/` の直下の文書、`docs/tasks/`、`docs/translation_glossary.md`、`docs/dev/guides/`、`prompts/README.md`、`testdata/README.md` | 日本語 |
| (b) エージェントの実行を制御する指示と、リポジトリ内のコード | CLAUDE.md、AGENTS.md、`.claude/commands/`、`docs/dev/conventions/`、ソースコードとコメント | 英語 |
| (c) エンドユーザー（拡張の利用者）向けの文書 | `README.md`（正本）、`README.en.md`（英語版） | 日本語を正本とし、英語版を同期する |

例外は次のとおりである（AC-04・AC-04a）。

-   **二言語の README（言語の例外）:** 二言語で持つ文書は README だけである。
-   **`README.en.md` の置き場所（置き場所の例外）:** `README.en.md` は、`README.md` と同じくリポジトリの直下に置く（「Documents should be placed under docs」の例外）。英語の読み手が `README.md` の先頭のリンクから同じ階層の英語版へ移れるようにし、GitHub がリポジトリの直下に置く README と並べて見つけられるようにするためである。
-   **エージェントが読む日本語の手引き（言語の例外）:** コマンドの定義は、`docs/dev/guides/` の日本語の手引きを参照してエージェントに読ませてよい（原則 (b) の例外）。範囲は、コマンドの定義が `docs/dev/guides/` の手引きを参照することに限る。今の参照は `.claude/commands/fixpr.md:34-35` の `fixprguard_guide` と `.claude/commands/mergepr.md:20-21` の `mergepr_guide` である。理由は要件 §1「エージェントが読む手引き」のとおりで、同じ内容を英語でも持つと二重管理に戻るためである。

### 1.3. 設計の際に決めたこと（2026-10-28）

-   **`/mktrans` の同期点はブロブの ID で持つ（H-10）。** `README.en.md` の先頭に、翻訳の元にした `README.md` のブロブの ID を書いた HTML のコメント（同期の印）を置く。`/mktrans` は、その ID のブロブと今の `README.md` の差分を翻訳し、終わったら印を今の `README.md` のブロブの ID に書き換える。同期点を、出力先のファイルを最後に変えたコミットにすると（`.claude/commands/mktrans.md:73-79`）、次の問題が起きる。`README.md` に未反映の変更があるときに `README.en.md` だけを直すコミットができると、同期点が先へ進み、未反映の変更が差分から落ちる（AC-38 に反する）。ブロブの ID なら、`README.en.md` だけを直しても印は翻訳の元を指したまま残る。印のブロブがどの複製でも取り出せることは、`main` の上で印が `README.md` の内容と一致することで保つ（2.4 節）。
-   **README に残す節の中の Job API の記述は `development.md` に移す（振り分けの A6）。** Localhost server の冒頭、Running the server、Stopping the server にある Job API の文と句を `development.md` に移し、README には `development.md` への短い案内を残す。拡張は #503 まで Job API を使わず、要件 §1 の決定（サーバの API は拡張の利用者の操作に影響しないので開発者向けの文書に移す）に合うためである。AC-19 の「各小節」を、文の単位で境界の規則を当てはめる F-003 の規則と合わせて読んだ結果である。
-   **日本語の見出しのアンカーを、本書の作成時に GitHub の表示で確かめた（H-01）。** 結果は 3.4 節にある。
-   **ロケータと、2 つ以上のパッケージが使う Markdown の補助関数は、テスト専用の共有パッケージ `internal/doctestutil` に置く（H-02）。** 英日のロケータの組を Go の中で 1 度だけ定義し、`cmd/yt2column-server/docs_test.go:363-522` にある補助関数のコピーもなくすためである。置き場所は、テスト専用のコードだけを持つ既存のパッケージ（`internal/maketestutil` など。`internal/pipeline/pipeline_test.go:583` の `testOnlyPackageDirs`）にならう。TypeScript は `extension/test/helpers/` に自分のロケータを持つ。Go と TypeScript で共有するデータファイル（JSON など）は作らない。TypeScript のテストが使うロケータは 3 つ（3.5 節）だけで、データファイルの読み込みと検査を両方の言語に加える費用に見合わないためである。

## 2. システム構成 (System Structure)

### 2.1. 全体の構成

図 2-1 は、本タスクの後に、本タスクが変える文書をどのガードテストが読むかを示す。本タスクが変えない文書（CLAUDE.md・`security.md`・`project_overview.md`・コマンドの文書など）を読む関係は省く。実線の矢印 A → B は「テスト A が文書 B を読んで検査する」を、点線の矢印 A → B は「テスト A が補助のコード B を使う」を表す。

```mermaid
flowchart LR
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef newpkg fill:#ffe8f5,stroke:#d946ef,stroke-width:2px,color:#701a75;

    subgraph Docs["文書"]
        RJA[("README.md")]
        REN[("README.en.md")]
        CLI[("cli.md")]
        DEV[("development.md")]
        FG[("guides/fixprguard_guide.md")]
        PR[("conventions/package_reference.md")]
    end

    subgraph GoTests["Go のガードテスト"]
        Y["cmd/yt2column<br>docs_test など"]
        S["cmd/yt2column-server<br>docs_test"]
        N["cmd/yt2column-nmhost<br>TestReadmeHostExitCodes"]
        F["cmd/fixprguard<br>docs_test"]
        P["internal/pipeline<br>TestPackageReferenceListsPackages"]
    end

    subgraph TsTests["拡張のテスト"]
        SR["serverReasons.test.ts"]
        MD["metadataDocs.test.ts"]
    end

    DG["internal/doctestutil"]
    LOC["extension/test/helpers"]

    Y --> RJA
    Y --> REN
    Y --> CLI
    Y --> DEV
    S --> RJA
    S --> REN
    S --> DEV
    N --> RJA
    N --> REN
    F --> FG
    P --> PR
    SR --> DEV
    MD --> RJA
    MD --> REN
    Y -.-> DG
    S -.-> DG
    N -.-> DG
    SR -.-> LOC
    MD -.-> LOC

    class RJA,Y,S,N,F,P,SR,MD enhanced
    class REN,CLI,DEV,FG,PR,DG,LOC newpkg
```

**図 2-1 の凡例**

```mermaid
flowchart LR
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef newpkg fill:#ffe8f5,stroke:#d946ef,stroke-width:2px,color:#701a75;
    subgraph Legend["Legend"]
        L1[("本タスクで変える既存の文書")]
        L2["本タスクで変える既存のテスト"]
        L3[("本タスクで新設・移動する文書")]
        L4["本タスクで新設する補助のコード"]
    end
    class L1,L2 enhanced
    class L3,L4 newpkg
```

### 2.2. コンポーネントの配置

| 置き場所 | 置くもの | 言語 |
|---|---|---|
| `docs/dev/conventions/` | `requirements_process.md`・`test_organization.md`・`mermaid_reference.md`・`package_reference.md`・`task_identification.md`（`git mv` で移す。移動の PR では、内容を相対リンクのほかは変えない） | 英語 |
| `docs/dev/guides/` | `mergepr_guide.md`・`fixprguard_guide.md`（今の `.ja.md` を移して改名する）、`pricing_update.md`・`claude_output_limit.md`（日本語に翻訳する）、`cli.md`・`development.md`（新設） | 日本語 |
| リポジトリの直下 | `README.md`（日本語の正本）、`README.en.md`（新設の英語版） | 日本語・英語 |
| `internal/doctestutil/` | 新設のテスト専用パッケージ `doctestutil`（3.3 節） | Go |
| `extension/test/helpers/` | README の版とロケータ（3.5 節） | TypeScript |
| `docs/tasks/0048_doc_language_policy/_checks/` | AC-08〜AC-15 の差分の検査のスクリプト（7.3 節） | シェルまたは Go |

`docs/dev/developer_guide/` は削除する（AC-12）。

### 2.3. README の振り分け

境界の規則（要件 F-003）を、今の README（1,387 行）の各節に当てはめた結果を 3.6 節に示す。規則は文の単位で当てはめる。

### 2.4. README の同期のデータフロー

図 2-2 は、`README.md` を変えたときに、変更が `README.en.md` に反映される流れを示す。矢印は呼び出しと応答を表す。図と以下の本文の「下位の手順」は、`/runplan`・`/fixpr`・`/runpr` が自分の手順の途中で `/mktrans` を呼ぶ使い方を指し、人間が `/mktrans` を直接呼ぶ「単独」の使い方と区別する。

```mermaid
sequenceDiagram
    participant C as 呼び出し元のコマンド<br>(/runplan・/fixpr・/runpr)
    participant T as /mktrans
    participant G as git
    participant E as README.en.md

    C->>G: この実行の変更に README.md があるか
    alt README.md を変えていない
        G-->>C: ない
        Note over C: 何もしない
    else README.md を変えた
        G-->>C: ある
        C->>T: README.md (下位の手順として)
        alt README.en.md がない
            T-->>C: 対象外
            Note over C: 何もせずに続ける
        else 印が今の README.md のブロブと同じ
            T-->>C: 反映済み
        else 印が古い
            T->>E: 同期の印のブロブ ID を読む
            T->>G: 印のブロブと今の README.md の差分
            G-->>T: 差分
            T->>E: 差分を翻訳して反映し、印を書き換える
            T-->>C: 反映した
        else 印がない・形が違う・ブロブがない・レビューで解消できない指摘
            T-->>C: 止まった (理由)
            Note over C: push せずに止まり、理由を報告する
        end
    end
```

`README.en.md` がないときに全文を翻訳するのは、`/mktrans` を単独で呼んだときだけである（3.8 節）。下位の手順では、`README.en.md` がなければ何もしない。英語の `README.md` から `README.en.md` を作ってしまわないためである（8 章の途中の PR）。`README.en.md` を作った後にそれが消えた場合は、構造の一致のテスト（4.1 節）が、ファイルがないことで失敗する。

**同期の印の形。** `README.en.md` の 1 行目に、次の形の HTML のコメントを 1 つだけ置く。GitHub の表示には現れず、見出しやコードブロックではないので、構造の一致（AC-29）にも影響しない。

```text
<!-- translated-from: README.md <object-id> -->
```

-   `<object-id>` は、翻訳の元にした `README.md` の内容のブロブの ID である。`/mktrans` は `git hash-object -w README.md` で求める。パスを渡すので、チェックアウトのときと同じ属性（改行の変換など）が適用され、`-w` でブロブがオブジェクトのデータベースに入る。
-   ID は省略しない 16 進の小文字で、長さはリポジトリのオブジェクトの形式（`git rev-parse --show-object-format`。SHA-1 なら 40 文字）に従う。`/mktrans` は、印の ID が `git cat-file -t` で `blob` であることを確かめてから差分を求める。

**印のブロブを取り出せることの保ち方。** PR は squash でマージされ、ブランチのコミットは `main` に残らない。`main` に残るのは、PR の先頭のコミットの `README.md` の内容のブロブである。そのため、印が `main` の `README.md` の内容と一致していれば、印のブロブはどの複製でも取り出せる。これを次の 2 つで保つ。

-   同期の手順（4.3 節）を、各コマンドで `README.md` を変えうる最後の手順の後、push の前に置く。
-   `/runpr` は、手順 4 でマージの質問をする前に、印を検査する。検査するのは、PR の先頭のコミットに `README.en.md` があり、PR の変更に `README.md` がある場合である（`README.en.md` がないときは、同期の手順と同じく検査しない）。印が PR の先頭のコミット（`headRefOid`）の `README.md` のブロブ（`git rev-parse <headRefOid>:README.md`）と一致しないときは、マージせずに止まり、`/mktrans` で反映して push するよう報告する。手順 4 では反映もコミットもしない。手順 4 のコミットは、`runpr.md` で PRE_MERGE として定めたファイルの変更に限られ、先頭のコミットが動くと手順 2 に戻るためである（`.claude/commands/runpr.md` の手順 4）。人間が直接変えた PR も `/runpr` でマージするので、この検査により `main` の上の印は `README.md` の内容と一致する。

それでも印のブロブを取り出せない場合（`/runpr` を使わずにマージした場合など）の復旧の手順は、4.3 節にある。

### 2.5. `fixprguard` の範囲の判定（H-07）

`/fixpr` が `README.md` のスレッドへの対応で `README.en.md` も変えると、`README.en.md` はスレッドの `path` とは別のファイルになる。`fixprguard` の範囲の規則 2（同じディレクトリのファイル。`internal/fixprguard/scope.go:16-21`）と許可リストの `*.md`（同 `:76`）により、`README.en.md` は `README.md` のスレッドに対して範囲に入る。`README.en.md` は実行関連ファイル（同 `:50-53` の `instructionFileNames` など）に当たらない。逆に、`README.en.md` のスレッドに対しては `README.md` が範囲に入る。そのため、`/fixpr` の中で同期の手順を実行してよく、別のコミットや `/runpr` への委任は要らない。`fixprguard` の変更は要らない。

一方、用語集（`docs/translation_glossary.md`）は `docs/` の下にあり、リポジトリの直下の `README.md` のスレッドに対して範囲に入らない。そのため、下位の手順として呼ばれた `/mktrans` は用語集を変えない（3.8 節）。

## 3. コンポーネント設計 (Component Design)

### 3.1. コンポーネントの責務

次の表は、本タスクで新設・変更・移動・削除するファイルの全体である。過去のタスク文書は数が多い。本タスクを除いて、`docs/dev/developer_guide/` を含むものは 92 ファイルある（`364c95b8` で `git grep -l`）。そのため 1 行にまとめる。

| ファイル | 責務・変更の内容 | 種類 |
|---|---|---|
| `CLAUDE.md` | "Documents" 節に 3 つの原則（AC-01）、ディレクトリごとの言語（AC-02）、例外（AC-04・AC-04a）を書き、`CLAUDE.md:25` の例外の列挙を消す。"Language" 節を原則と合わせる（AC-03）。"Translation Guidelines" 節を README の組の同期に改める（AC-40）。Quick Links と本文のパスを新しいパスにする。`CLAUDE.md:212-213` を、サーバの API は `development.md`、起動は README に書くと改める（AC-07）。Go Idioms の例外（`CLAUDE.md:507-516`）が Go のテストにも適用されると明記する（AC-05）。Development Notes の文書を読むガードテストの説明（`CLAUDE.md:431-448`）を 3.7 節の対応に合わせ、`TestReadmeHostExitCodes` が両版を読むようになっても `doc-guard` では走らないことを書く（AC-41） | 変更 |
| `AGENTS.md` | `requirements_process.md`・`test_organization.md` のリンクを `docs/dev/conventions/` に改める | 変更 |
| `.claude/commands/_context.md` | 規約のパスの行（`:38-42`）と Guard tool の行（`:188`）を新しいパスにする。「Translation language pair」の行（`:54`）を、README の組だけを扱う `/mktrans` に合わせる（AC-40） | 変更 |
| `.claude/commands/mktrans.md` | 3.8 節のとおり改訂する（AC-38・AC-39） | 変更 |
| `.claude/commands/_lib/task-pr-handoff.md` | Part B の手順 1（ローカルのレビュー）と手順 2（CHECKS）の間に、同期の手順を加える（4.3 節） | 変更 |
| `.claude/commands/runplan.md`・`fixpr.md`・`runpr.md` | 4.3 節の同期の手順を加える（AC-37）。`runplan.md` の手順 5a の CHECKS の説明（`:82`）を、同期のコミットがあれば green gate を走らせ直すと改める。旧パスを新しいパスにする | 変更 |
| `.claude/commands/` のほかのコマンド（`mkarch.md`・`mkplan.md`・`mkplan2.md`・`mkreq.md`・`mergepr.md`・`weakreview.md`） | 旧パスを新しいパスにする | 変更 |
| `docs/dev/developer_guide/` の 11 ファイル | 2.2 節のとおり移す。`mergepr_guide.md`・`fixprguard_guide.md`（英語版）は、AC-11 の読み比べ（7.3 節）の後に削除する | 移動・削除 |
| `docs/dev/guides/pricing_update.md`・`claude_output_limit.md` | 全文を日本語に翻訳する（AC-10）。README の節の参照（`pricing_update.md:55-67`、`claude_output_limit.md:61`・`:104`）を移った先の文書と見出しにする（AC-26） | 移動・翻訳 |
| `docs/dev/guides/cli.md` | 新設。AC-20 と 3.6 節の内容 | 新設 |
| `docs/dev/guides/development.md` | 新設。AC-21 と 3.6 節の内容 | 新設 |
| `README.md` | 3.6 節で README に残す内容を日本語にする（AC-27）。先頭に `README.en.md` へのリンク（AC-28）、開発者向けの文書へのリンク（AC-23）を置く。Browser extension を最上位の節にする（AC-19） | 変更 |
| `README.en.md` | 新設。改訂した `/mktrans` を単独で呼んで、`README.md` から全文を翻訳する。1 行目に同期の印、先頭に `README.md` へのリンク | 新設 |
| `docs/translation_glossary.md` | AC-34 の訳語を加える。`:53`（推奨構成）の README の節の名前を日本語の見出しにする（AC-26） | 変更 |
| `docs/dev/project_overview.md` | `:5` のリンクを `conventions/` にする。`:157` の「Migrating from the retired LLM variables」を移った後の見出しにする（AC-26） | 変更 |
| `docs/dev/production_profile.md` | `:5` の「Recommended configuration」を移った後の見出しにする（AC-26） | 変更 |
| `docs/tasks/*/`（本タスクを除く 92 ファイル） | 移動した文書を指すパスだけを置き換える（AC-13・AC-14。6.2 節）。文書でないファイル `docs/tasks/0032_japanese_pr_title/check_rules.sh:654`・`:657` も同じに扱う（6.2 節） | 変更 |
| `Makefile` | コメントのパス（`:71`・`:77`）を改める | 変更 |
| `cmd/fixprguard/main.go` | `guidePath`（`:132`）を `docs/dev/guides/fixprguard_guide.md` にする（AC-16） | 変更 |
| `cmd/yt2column/run.go` | ヘルプの参照先（`:225` の `--slack`、`:259`）を `docs/dev/guides/cli.md` にする（AC-36） | 変更 |
| `cmd/yt2column-server/run.go` | 診断の参照先（`:145`）を `README.en.md` にする（AC-35a） | 変更 |
| `extension/src/core/i18n/en.ts` | ソケットのディレクトリの権限の説明の参照先（`:83`。要件の作成時の `:77` から移った）を `README.en.md` にする（AC-35）。`ja.ts:81` は変えない | 変更 |
| `internal/config/config.go:595`、`internal/llm/claude/request.go:29`、`internal/llm/provider/provider.go:22`、`internal/llm/provider/provider_test.go:406`、`internal/pricing/rates.go:2` | コメントのパスを `docs/dev/guides/` にする | 変更 |
| `internal/doctestutil/`（新設） | ロケータ、README の版の一覧、共有の Markdown の補助関数とそのテスト、構造の抽出（3.3 節） | 新設 |
| `internal/pipeline/pipeline_test.go` | `:707` のパスを `docs/dev/conventions/package_reference.md` にする（AC-17）。`testOnlyPackageDirs`（`:583`）に `internal/doctestutil` を加える | 変更 |
| `docs/dev/conventions/package_reference.md` | `internal/doctestutil` の行を加える（`TestPackageReferenceListsPackages` が要求する。AC-08 の検査の対象外。7.3 節） | 変更 |
| `cmd/yt2column/*_test.go`（`docs_test.go`・`output_layout_docs_test.go`・`extension_docs_test.go`・`fixpr_docs_test.go`・`codex_review_docs_test.go`・`copied_transcript_docs_test.go`）、`cmd/yt2column-server/docs_test.go`、`cmd/yt2column-nmhost/main_test.go` | 3.7 節のとおり、読む文書とロケータを改める。共有パッケージに移した補助関数の呼び出し元を改める。サーバのパッケージの補助関数のコピー（`docs_test.go:363-522`）と、その一致を固定する `TestSectionByHeading`（同 `:213`）は消す（3.3 節） | 変更 |
| `cmd/yt2column/readme_structure_test.go`（新設、名前は計画で決める） | 構造の一致のテスト（AC-29。4.1 節） | 新設 |
| `cmd/yt2column/docpaths_test.go`（新設、名前は計画で決める） | 文書が示すパスの存在の検査（4.2 節） | 新設 |
| `cmd/fixprguard/docs_test.go` | `guideFiles`（`:27-30`）を 1 つのファイル `docs/dev/guides/fixprguard_guide.md` にし、英語の手引きの印（`"**Execution-related files.**"` など）の行を消す（AC-17） | 変更 |
| `extension/test/serverReasons.test.ts`・`metadataDocs.test.ts`、`extension/test/helpers/` | 3.5 節・3.7 節のとおり改める | 変更 |
| `extension/scripts/has-extension-changes.sh`、`extension/test/ciChanges.test.ts` | `README.en.md` と `docs/dev/guides/development.md` を判定に加え、その行の検査を加える（AC-33） | 変更 |

### 3.2. 先のタスクの方針の置き換え（方針の例外）

本タスクは、先のタスクで承認された次の決定を置き換える（要件 §1「先のタスクの決定の置き換え」）。置き換えるたびに、元の方針、置き換える理由、古い振る舞いを検査しているテストを挙げる。

| 元の方針（記載箇所） | 置き換え | 理由 | 古い振る舞いを検査しているテスト |
|---|---|---|---|
| Job API の値を README の表に書く（`docs/tasks/0044_article_job_api/01_requirements.md:235`、AC-29） | 表を `development.md` に置く | サーバの API は拡張の利用者の操作に影響しない（要件 §1） | `TestServerDocsContract` の `api_codes` サブテスト（`cmd/yt2column-server/docs_test.go:124`）。README の `## Localhost server` の節で表を探す。`README.md` サブテスト（`:42`）の Job API の curl の例（`POST /jobs` など） |
| README の拒否の理由の表がサーバと一致する（同 `:299`、AC-42） | ステータスコードを含む完全な表は `development.md`、`reason` と拡張での表示の対応は README の両版 | 同上。ただし、ポップアップが拒否の理由を表示するので、利用者が意味を README で調べられるようにする | `TestServerDocsContract` の `rejection_reasons` サブテスト（`:96`）、`extension/test/serverReasons.test.ts:44` |
| README が引用するポップアップの英語の文言が英語の文言の表と一致する（`docs/tasks/0046_popup_redesign/01_requirements.md:196`、AC-31） | `README.en.md` は英語の、`README.md` は日本語の文言を引用する | 日本語の README の読み手は日本語のポップアップを見る | `extension/test/metadataDocs.test.ts:24-35`。0046 の PR-4 が加える `catalogs.en.primaryButton` の確認（`docs/tasks/0046_popup_redesign/03_implementation_plan.md:340`）も含む |
| 手引きは、英語版の手引きと日本語版の手引きの両方を持つ（`docs/tasks/0033_multiple_trusted_reviewers/02_architecture.md:189`） | 日本語版の手引きに一本化する | issue #537 の決定（要件 §1） | `cmd/fixprguard/docs_test.go:27-30` の `guideFiles` と、英語の印を使う各テスト |
| サーバの API と起動は README に書く（`CLAUDE.md:212-213`） | API は `development.md`、起動は README | 要件 §1「サーバの API」 | なし（CLAUDE.md のこの文を読むテストはない） |

### 3.3. 共有パッケージ `internal/doctestutil`

テスト専用のパッケージ（すべてのファイルに `//go:build test`。パッケージ名は `doctestutil`）である。テスト専用のコードだけを持つ既存のパッケージ（`internal/maketestutil` など）と同じく、`internal/pipeline/pipeline_test.go` の `testOnlyPackageDirs` に加える。親のパッケージの横に置く `testutil/` の形（`docs/dev/conventions/test_organization.md` の分類 A）は取らない。対応する本番のパッケージがないためである。

**移す補助関数。** 次の関数を移す。`ReadDoc`・`SectionByHeading`・`MarkdownTables`・`FindTable` は、今 `cmd/yt2column` と `cmd/yt2column-server` の両方にある。`AnchorFromHeading`・`HeadingsOutsideBlocks`・`SectionOutsideBlocks`・`FencedBlocks` は、今は `cmd/yt2column` だけが使うが、フェーズ 6 で `ParseStructure` と、両版を読む `cmd/yt2column-server`・`cmd/yt2column-nmhost` のテストが使う。どちらにも当たらない関数（`numberedItems`・`checkExportBlock`・`tablesByFirstHeader` など）は、今のパッケージに残す。

```go
// ReadDoc returns the document at path, or fails t.
func ReadDoc(t testing.TB, path string) string

// AnchorFromHeading returns the anchor GitHub derives from a heading line.
func AnchorFromHeading(heading string) string

// HeadingsOutsideBlocks returns the heading lines of doc outside fenced code
// blocks, in order.
func HeadingsOutsideBlocks(doc string) []string

// SectionByHeading returns the section of doc that starts at the line equal
// to heading, up to the next heading of the same or a higher level.
func SectionByHeading(doc, heading string) string

// SectionOutsideBlocks is SectionByHeading with fenced code blocks skipped
// when looking for headings.
func SectionOutsideBlocks(doc, heading string) string

// MarkdownTables returns every table of doc as rows of cells.
func MarkdownTables(doc string) [][][]string

// FindTable returns the first table of doc with a header cell equal to header,
// ignoring case, or nil.
func FindTable(doc, header string) [][]string

// FencedBlocks returns the fenced code blocks of doc.
func FencedBlocks(doc string) []FencedBlock
```

`FencedBlock` は、今の `fencedBlock`（`cmd/yt2column/docs_test.go:684`）を公開した型である。これらが内部で使う小さな関数（`splitTableRow`・`stripBackticks`・`headingLevel`・`scanDocLines` など）も一緒に移す。公開するかどうかを含む最終の一覧は、呼び出し元を数えて実装計画で決める。移した関数のテスト（`TestAnchorFromHeading`・`TestScanDocLines`・`TestFencedBlocks`・`TestSectionOutsideBlocks`・`TestSectionByHeading` など）も共有パッケージに移す。サーバのパッケージのコピーと、その一致を固定する `TestSectionByHeading`（`cmd/yt2column-server/docs_test.go:213`）は消す。テストを消すので、`go tool cover -func` の関数ごとの結果が変わらないことを確かめる（CLAUDE.md、Testing Strategy）。

**README の版とロケータ。**

```go
// Lang is the language of one version of a bilingual document. The zero
// value is not a language: a Locator asked for it fails the test.
type Lang int

const (
	LangUnknown Lang = iota
	LangJA
	LangEN
)

// ReadmeVersion is one version of the README: its path from the repository
// root and its language.
type ReadmeVersion struct {
	Path string
	Lang Lang
}

// ReadmeVersions returns README.md (LangJA) and README.en.md (LangEN), in
// that order.
func ReadmeVersions() []ReadmeVersion

// Locator is a README heading line or table column name that a guard test
// looks for, spelled once per language.
type Locator struct {
	JA string
	EN string
}

// In returns the spelling for lang. It fails t for LangUnknown and for an
// empty spelling, so a missing translation fails instead of matching
// nothing silently.
func (l Locator) In(t testing.TB, lang Lang) string
```

-   README のロケータは、このパッケージの 1 つの表（変数の一覧）として定義する。ガードテストは、README の両版を `ReadmeVersions()` で回し、各版でロケータを `In` で引く。失敗のメッセージには版のパス（`README.md` か `README.en.md`）を含める（AC-31）。
-   日本語だけの文書（`cli.md`・`development.md`）の見出しと列名は、`Locator` を使わず、日本語の文字列の定数として同じ表に置く。
-   `Lang`・`ReadmeVersion`・`ReadmeVersions`・`Locator` は、`README.md` を日本語にし `README.en.md` を作る PR（8 章のフェーズ 6）で加える。それより前の PR では `README.md` は英語で `README.en.md` はなく、`ReadmeVersions` の前提（両版があること）が成り立たないためである。また、使う側がない関数は `make deadcode` が報告する（CLAUDE.md、Commands）。

Go のテストに日本語のリテラルを置くのは、CLAUDE.md の Go Idioms の例外（日本語の文書の見出しや表のセルに一致させるガードテストのリテラル）に当たり、本タスクでその例外が Go のテストにも適用されると明記する（AC-05）。

**構造の抽出。**

```go
// HeadingPos is one ATX heading outside a fenced code block.
type HeadingPos struct {
	Line  int // 1-based line number
	Level int // 1-6
}

// Structure is the shape that README.md and README.en.md must share. Two
// Structures match when their heading levels, in order, and their code block
// counts are equal; the line numbers are only for the failure message.
type Structure struct {
	Headings   []HeadingPos
	CodeBlocks int
}

// ParseStructure extracts the Structure of doc, or returns an error wrapping
// one of the sentinel errors in section 4.1.
func ParseStructure(doc string) (Structure, error)
```

### 3.4. 日本語の見出しのアンカー（H-01）

GitHub のアンカーの作り方を、本書の作成時に確かめた。日本語の見出しを持つ 4 つの文書（`docs/tasks/0048_doc_language_policy/01_requirements.md`・`docs/dev/developer_guide/fixprguard_guide.ja.md`・`docs/dev/production_profile.md`・`docs/dev/security.md`、`364c95b8`）を、GitHub の API（`repos/isseis/yt2column/contents/<path>`、`Accept: application/vnd.github.html`）で描画した。その `id="user-content-…"` の 68 個を、`anchorFromHeading`（`cmd/yt2column/docs_test.go:70-83`）がコードブロックの外の見出しから作るアンカーと順に比べた。結果は、68 個すべてが一致した（2026-10-28）。

-   確かめた見出しに含まれていた ASCII でも文字・数字でもない文字は、`（` `）` `・` `、` `：` と、ASCII の `.` `` ` `` `(` `)` `[` `]` `:` `=` `_` `/` である。GitHub はどれも落とし、日本語の文字（漢字・かな・長音の記号）はそのまま残す。`anchorFromHeading` も同じである。
-   確かめていない文字は、`「` `」`、全角の空白（U+3000）、全角の英数字である。`README.md` の見出しにこれらを使う場合は、同じ方法で確かめ、結果を実装計画に記録する。確かめた文字だけで書けるなら、使わない。
-   Markdown の API（`POST /markdown`）は、今は見出しに `id` を付けないので、確かめる手段にならない。

この結果から、リンクの検査は `AnchorFromHeading`（共有パッケージに移した `anchorFromHeading`）をそのまま使う。その表のテストに、確かめた日本語の見出しの行を加える。`cli.md` と `development.md` のリンクも、同じ補助関数で確かめる（4.2 節）。

### 3.5. 拡張のテストの補助（TypeScript）

`extension/test/helpers/` に、README の版（パスと言語）と、拡張のテストが使う 3 つのロケータを置く。

-   言語は、拡張の既存の `Locale` 型（`extension/src/core/i18n/locale.ts:2` の `"en" | "ja"`）で表す。`README.md` は `"ja"`、`README.en.md` は `"en"` である。版の言語をそのままメッセージカタログの言語として使えるので、AC-31a の比較（`README.md` と日本語のカタログ、`README.en.md` と英語のカタログ）が言語の値 1 つで決まる。
-   ロケータは、ブラウザ拡張の節の見出し（今の `"### Browser extension"`、`extension/test/metadataDocs.test.ts:25`）の英日の組、`development.md` の拒否の理由の表の列名（今の README の `reason | status code | extension shows`、`serverReasons.test.ts:17`）、その表の「表示」の列の値を種類ごとにまとめた区分（3.7 節の末尾）の書き方である。後の 2 つは `development.md` が日本語だけなので、日本語の文字列だけを持つ。
-   ブラウザ拡張の節は最上位（`##`）になる。今の `markdownSection`（`extension/test/helpers/repoFile.ts:15`）は、レベル 1〜3 の次の見出しで節を終える。そのため、節の中に `###` の小節を置くと、節が途中で切れる。節の中に小節を置くなら、`markdownSection` を見出しのレベルに合わせて節を終えるように改める。

### 3.6. README の振り分け（H-12）

境界の規則を今の README に当てはめた結果である。`364c95b8` の `README.md`（0044 の PR-7 がマージされた後）に対して作った。0046 の PR-4 は、計画では Browser extension の節のポップアップの説明（`README.md:1238-1252` の付近）だけを変える（`docs/tasks/0046_popup_redesign/03_implementation_plan.md:339`）。その節は README に残るので、この振り分けは PR-4 の後も変わらない見込みである。PR-4 のマージの後に、PR-4 が README のどこを変えたかを確かめる。その範囲の外を変えていたら、3.6 節と 3.7 節を作り直す。範囲の中だけなら、PR-4 の後の README とこの表を照らし合わせる。

**節の単位の振り分け。** R は README の両版、C は `cli.md`、D は `development.md` である。

| 行 | 見出し | 行き先 |
|---|---|---|
| 1-49 | `# yt2column`（概要・図・状態） | R |
| 51-77 | Quick start | R |
| 79-97 | Requirements | R と C に分ける（表 3-2） |
| 99-136 | Usage | C |
| 138-144 | Exit codes | C |
| 146-199 | Post-generation steps | R と C に分ける（表 3-2） |
| 201-266 | Token usage | C |
| 268-273 | Output format の冒頭 | R と C の両方（AC-22 の「両方の出力に関わる説明」） |
| 275-320 | The `--out` file | C |
| 322-351 | The Slack post | R |
| 353-440 | Estimated cost | R と C に分ける（表 3-2） |
| 442-468 | Cache and concurrency | C |
| 470-524 | Webhook publishing | R と C に分ける（表 3-2） |
| 526-554 | Localhost server の冒頭 | R。Job API の句は D（表 3-2） |
| 556-592 | Running the server | R。Job API の文は D |
| 594-603 | Stopping the server | R。Job API の句は D |
| 605-676 | Installing the native messaging host | R |
| 678-720 | Checking the server with curl | D。`reason` と拡張での表示の対応は R にも置く |
| 722-814 | The job API | D（下の注） |
| 816-865 | Job API codes | D |
| 867-929 | Troubleshooting | R。curl の項目は D、CLI の終了の句は C |
| 931-945 | Upgrading from the shared-token version | R |
| 947-974 | Setting up yt-dlp | C |
| 976-1044 | Configuration | R と C に分ける（表 3-2） |
| 1046-1067 | Recommended configuration | R |
| 1069-1113 | Migrating from the retired LLM variables | R |
| 1115-1134 | Development | D |
| 1136-1177 | Running a separate server for development | D |
| 1179-1304 | Browser extension | R（最上位の節にする）。`make ext-check` の行（`:1199`）は D |
| 1306-1387 | Integration test と 5 つの統合テストの節 | D |

> 注：「The job API」の節は、AC-21 の表に名前がない（要件の作成の後に 0044 の PR-7 が加えた）。境界の規則の「サーバの API」に当たるので D に置く（要件 §1「サーバの API」）。

**表 3-2：文の単位の振り分け（分ける節）**

| 節 | R に残す | C に移す | D に移す |
|---|---|---|---|
| Requirements | 主経路の要件（`:81-87`）、選んだプロバイダの API キー（`:95-97`）、CLI の要件への案内 | 「For the CLI only」の一覧（`:89-93`。`#setting-up-yt-dlp` へのリンクを含む） | - |
| Post-generation steps | 2 段階が何をするか（`:148-161`。字数の調整の失敗で記事を書き出しも投稿もしないことを含む）、費用が増えること（`:163-165` の前半）、C への案内 | `--no-shorten`・`--no-review` から後の段落のすべて（`:165` の後半-`:199`） | - |
| Estimated cost | 両方の出力が料金を持つことと脚注の形（`:404-411` のうち一般の句と Slack の脚注）、Output format への参照（`:413`）、推定と請求額が異なりうる理由（`:415-432` の前半）、サーバの投稿では料金が不明な理由を示せないこと（`:437-439` の前半）、C への案内 | `cost:`・`cost-total:` の行の説明と例（`:355-402`）、`--out` の `estimated_cost_usd`、再開した実行の料金（`:432` の後半-`:435`）、理由を CLI で調べる方法（`:439` の後半-`:440`） | - |
| Webhook publishing | 投稿先が `SLACK_WEBHOOK_URL` の Webhook であること、URL の規則（`:473-478`。終了コード `2` はサーバにも当てはまるので残す）、分割、`silent`、メンションの検査と拒否（`:481-519` のうち CLI の句を除く）、分割した投稿の途中の失敗（`:521-524`） | `--slack` で投稿すること、URL の拒否で `--out` の実行も止まること、`--out` のファイルが変わらないこと、行番号が `--out` のファイルと異なること、`--out` で確かめる方法 | - |
| Localhost server の冒頭と小節 | Job API の句を除くすべて。`YT2COLUMN_CACHE_DIR`・`YT2COLUMN_YTDLP_PATH` を読まないこと（`:543-544`）も残す（AC-22 の、今の README の中ですでに重複している記述） | - | Job API の句と文（`:530-532` の Job API の句、`:541` の括弧、`:568-570`、`:598-600` の生成のキャンセル、`:601` の「and the publish jobs」）。R に D への案内を残す |
| Checking the server with curl | `reason` と拡張での表示の 2 列の表（13 行すべて）と D への案内 | - | 節の全体（ステータスコードを含む 3 列の表、`v` を持たないメッセージの説明を含む） |
| Troubleshooting | curl の項目と CLI の句を除くすべて | 「A CLI run exits with `2` before it starts;」（`:889`） | 「The server is not running. `curl` reports …」の項目（`:869-872`）の全体 |
| Configuration | 表の見出しの行、主経路の変数の行（`:980-986`。`SLACK_WEBHOOK_URL` の「未設定のとき」はサーバでの扱い）、主経路の注記、C への案内 | 表の見出しの行、`YT2COLUMN_CACHE_DIR`・`YT2COLUMN_YTDLP_PATH` の行（`:987-988`）、`SLACK_WEBHOOK_URL` の `--slack`・`--out` での扱い、サーバがこの 2 つを読まない注記（`:991-993`）、CLI がクライアントを作る時点（`:1007`）、再開した実行での設定の読み方（`:1012-1014`・`:1021-1022`）、README の設定の節への参照 | - |
| Development | - | - | 全体（`make` のターゲットの一覧、開発の進め方への案内） |
| Browser extension | `make ext-check` の行を除くすべて | - | `make ext-check` の行 |

振り分けの判断が分かれうる箇所は、次のとおり決める。

-   **CLI を主語にした料金の理由（`:418-426`）:** 「The CLI judges…」などの文は、サーバでも同じ理由が当てはまるが、CLI を主語にしている。README に残し、文言は変えない。主語を書き換えると内容を加えることになり、AC-27 に反するためである。
-   **The Slack post の「or a disabled review」（`:343-344`）:** 検証・推敲を外せるのは CLI の `--no-review` だけだが、利用者が見る脚注の規則の例なので R に残す。
-   **リンクの付け替え（AC-25）:** 移る文の中のリンク（`:92`・`:103`・`:171`・`:188`・`:199`・`:312`・`:413` の C の写し・`:1309`）は、移った先の文書の見出しか、文書をまたぐリンク（例：`../../../README.md#<アンカー>`）にする。README に残る文の中のリンクは、どれも README に残る見出しを指す。D に移る文の中の相対リンク（`:1131`・`:1132`・`:1134`）は、`development.md` からの相対パスにする。

**AC-22 で 2 つの文書に置いてよい重複**は、Output format の冒頭（R・C）、Estimated cost の一般の句と Output format への参照（R・C）、`reason` と表示の対応（R・D）、旧変数で実行が止まること（C の `:196-199`、R の `:885-897`・`:1018-1022`）、チャンネル名が `invalid_body` で拒否されること（C の `:467-468`、R の `:1256-1258`）、サーバがキャッシュと yt-dlp の変数を読まないこと（R の `:543-544`、C の `:991-993`）、投稿先の Webhook を示す句（R・C）だけである。

### 3.7. ガードテストの振り分け（H-11、AC-31・AC-32）

本書の作成時に README を読むガードテストと、本タスクの後にそれぞれが読む文書は次のとおりである（`364c95b8` で調べた。0044 の PR-7 のマージ後）。両版（1.1 節）のロケータは 3.3 節の表で引く。実装計画は、この表をもとに、テストごとに検査する値の前後の表を作る（AC-32、`manual`）。

| テスト（ファイル:行） | 今読むもの | 本タスクの後に読むもの |
|---|---|---|
| `TestREADMEDocumentsFlags`（`cmd/yt2column/docs_test.go:173`） | README のフラグの表 | `cli.md` |
| `TestSlackDocsContract`（`:191`） | README の `16,383` と、統合テストの変数・`make` のターゲット | `16,383` は両版、統合テストの値は `development.md`（分かれる） |
| `TestClaudeDocsContract`（`:239`） | README の Claude の統合テストの変数 | `development.md` |
| `TestDeepSeekDocsContract`（`:261`） | README の 3 つの統合テストの節 | `development.md` |
| `TestLLMDefaultsDocsContract`（`:302`） | README の設定の表（`Variable`・`When unset`） | 両版（日本語の列名は `変数`・`未設定のとき` とし、`project_overview.md` の列名と同じにする） |
| `TestProductionProfileDocsContract`（`:934`） | README の Recommended configuration | 両版 |
| `TestMigrationDocsContract`（`:976`） | README の Migrating | 両版 |
| `TestQuickStartLinks`（`:1036`） | README の Quick start | 両版 |
| `TestProductionProfileLinks`（`:1079`） | README の Recommended configuration ほか | 両版（README 以外は今のまま） |
| `TestReadmeLinks`（`:1119`） | README のリンク | 両版・`cli.md`・`development.md`（4.2 節） |
| `TestTokenUsageDocsContract`（`:1255`） | README の Token usage | `cli.md` |
| `TestCostDocsContract`（`:1313`） | README の Estimated cost（CLI の料金の行） | `cli.md`。README に残る推定の料金の節は読まない（脚注の形は `TestOutputLayoutDocsContract` が Slack の投稿の例で検査する） |
| `TestOutputLayoutDocsContract`（`output_layout_docs_test.go:92`） | README の 2 つの出力の例 | `--out` の例は `cli.md`、Slack の投稿の例は両版（分かれる） |
| `TestExtensionDocumentation`（`extension_docs_test.go:52`） | README の拡張の ID | 両版（CLAUDE.md と `security.md` は今のまま） |
| `TestServerDocsContract` の `README.md`（`cmd/yt2column-server/docs_test.go:42`） | README の Localhost server の節 | `SLACK_WEBHOOK_URL`・ソケットのパス・終了コードの表は両版。curl の例（`POST /articles` と Job API の 3 つ）は `development.md`（分かれる） |
| 同 `rejection_reasons`（`:96`） | README の拒否の理由の 3 列の表 | `development.md` の 3 列の表をサーバの定義と照らす。加えて、両版の 2 列の表の `reason` の集合がサーバの定義と一致し、各行の表示の区分が `development.md` の表と一致することを検査する（H-03） |
| 同 `api_codes`（`:124`） | README の Job API codes の 6 つの表 | `development.md` |
| `TestReadmeHostExitCodes`（`cmd/yt2column-nmhost/main_test.go:436`） | README のホストの終了コードの表 | 両版 |
| `serverReasons.test.ts:44` | README の 3 列の表の「固定のメッセージ」の行 | `development.md` の 3 列の表 |
| `metadataDocs.test.ts:24-35` | README の Browser extension の節と英語のカタログ | `README.md` は日本語のカタログ（`ja.ts`）、`README.en.md` は英語のカタログ（`en.ts`）と比べる（AC-31a） |

**拒否の理由の表示の区分。** 表示の列は、少数の決まった書き方の値である。今の README では「fixed message」「unexpected reply」と、条件付きの「fixed message (unexpected reply when `requestId` is empty)」（`README.md:715`）の 3 つである。これらはロケータではなく値なので、言語ごとの区分の表（区分ごとの日本語と英語の書き方。条件付きの形を含む）として共有パッケージに置き、両版と `development.md` のセルを区分に直して比べる。TypeScript は、`development.md` の書き方だけを持つ。

README を読まない、次のテストも本タスクの影響を受ける。

-   **CLAUDE.md とコマンドの文書を読むテスト（H-08）:** `TestProductionProfileLinks`（CLAUDE.md の `## Quick Links`）、`TestExtensionDocumentation`（CLAUDE.md の `make ext-` の一覧）、`cmd/yt2column/fixpr_docs_test.go` と `codex_review_docs_test.go` の各テスト（`fixpr.md`・`runpr.md`・`_context.md`・`_lib/task-pr-handoff.md` の見出し、行、`fixprguard` の呼び出し、メンションの位置）。本タスクは、これらが検査する見出しと行を変えず、`fixprguard` の新しい呼び出しも Codex のメンションも書かない。Quick Links の見出しと、`production_profile.md` へのリンクは残す。同期の手順（4.3 節）は、テストが区切りに使う見出しや番号付きの項目（例：`_lib/task-pr-handoff.md` の「3. Push and create the PR for the current branch.」と「4. Run AFTER_CREATE」）を変えない場所に加える。これらのテストのファイルは、共有パッケージに移した補助関数の呼び出しだけが変わる（`copied_transcript_docs_test.go` も同じ）。
-   **`fixprguard` の手引きのテスト（`cmd/fixprguard/docs_test.go`）:** 1 つのファイルだけを読む。日本語の印（`"**実行関連ファイル。**"` など）を使う行は残り、英語の印の行はなくなる。
-   **`TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:671`）:** 移動後のパスを読み、新設の `internal/doctestutil` の行を要求する。

### 3.8. `/mktrans` の改訂

改訂後の `/mktrans` が扱う翻訳の組は、`README.md`（日本語）から `README.en.md`（英語）への 1 組だけである。

| 入力 | 単独で呼んだとき | 下位の手順として呼んだとき |
|---|---|---|
| `README.md`、`README.en.md` がない | 全文を翻訳し、1 行目に同期の印を書く | 何もしない（「対象外」） |
| `README.md`、印が今の `README.md` のブロブと同じ | 反映済みとして何も変えずに終わる | 同じ（「反映済み」） |
| `README.md`、印が古い | 印のブロブと今の `README.md` の差分を翻訳して反映し、印を書き換える | 同じ（「反映した」） |
| `README.md`、印がない・形が違う・2 つある、または印の ID が `blob` でない・取り出せない | 翻訳せずに止まり、理由と復旧の手順（4.3 節）を報告する | 同じ（「止まった」） |
| それ以外のファイル（`README.en.md` を含む） | 翻訳せずに止まり、利用者に尋ねる（AC-39） | 呼び出し元は渡さない |

-   `.ja.md` の組を前提にした方向の規則（`.claude/commands/mktrans.md:27-35`）と、developer_guide の翻訳の組への言及（同 `:20-22`）を消す（AC-39）。
-   用語集の読み込み、翻訳の原則、翻訳のレビュー（同 `:114-144`）は、今のまま使う。
-   **用語集の更新:** 単独で呼んだときは、今のとおり用語集を更新し、別のコミットにする（同 `:141` の「Commit」）。下位の手順として呼んだときは、用語集を変えず、用語集にない語を結果とともに報告する。呼び出し元は、その語を最終の報告と PR の本文に挙げる。`/fixpr` の範囲の外のファイル（2.5 節）を変えないためである。
-   **コミット:** 下位の手順として呼ばれたときは、コミットしない（`japrose.md:127` と同じ形）。呼び出し元が、自分のコミットに `README.en.md` を含める。単独で呼ばれたときは、今の手順どおりにコミットする。

差分翻訳の同期点がコミットからブロブに変わるので、`README.md` と `README.en.md` を同じコミットで変えても、`README.en.md` だけを直すコミットがあっても、未反映の変更は印のブロブとの差分に含まれる（H-10）。

## 4. エラーハンドリング設計 (Error Handling Design)

本タスクは yt2column の実行時のエラーを変えない（要件 §4.1・§4.3）。設計するのは、ガードテストの失敗と `/mktrans` の止まり方である。

### 4.1. 構造の一致のテスト（AC-29・H-04）

`ParseStructure`（3.3 節）は、共有パッケージに移したコードブロックの走査（今の `scanDocLines`・`isClosingFence`・`fencedBlocks`、`cmd/yt2column/docs_test.go:657-700`）を使い、走査の規則は変えない。今の走査は、字下げしたフェンスを受け入れ、`~~~` を認めず、3 個以上のバッククォートだけの行をどれも閉じのフェンスとし（同 `:677-679`）、`headingLevel`（同 `:544`）は 7 個以上の `#` も見出しとする。これらは GitHub の解釈と異なりうる。ただし、コマンドの文書には字下げしたフェンスがあり（例：`.claude/commands/_lib/task-pr-handoff.md:127`）、走査を変えるとほかのガードテストの振る舞いが変わるので、走査は変えない。

そのため、`ParseStructure` は、README で走査と GitHub の解釈がずれうる形を、数える前にエラーとして拒否する。README は今どの形も使っていない（`364c95b8` で確かめた）。黙って数え方を決めると、両版の構造が違うのに一致と判定しうるためである（CLAUDE.md、"Reject, don't normalize"）。

-   **見出し:** コードブロックの外にある ATX 形式の見出し（行頭の 1〜6 個の `#` と空白）を数える。Mermaid のブロックやコードの中の `#` で始まる行（`README.md:293`・`:299`・`:328`・`:332`）は数えない。
-   **拒否する形:** 閉じていないフェンス、行頭に空白のあるフェンス、`~~~` のフェンス、4 個以上のバッククォートのフェンス、7 個以上の `#` で始まる行、コードブロックの外の Setext 形式の見出し（文の行の直後の、`=` または `-` だけの行）。Setext の判定はコードブロックの外だけで行う（`README.md:281`・`:291` の `---` は ```` ```text ```` のブロックの中にある）。

```go
// ErrUnclosedFence reports a fenced code block with no closing fence.
var ErrUnclosedFence = errors.New("unclosed fenced code block")

// ErrUnsupportedFence reports a fence the README does not use: indented,
// opened with tildes, or longer than three backticks.
var ErrUnsupportedFence = errors.New("unsupported code fence")

// ErrUnsupportedHeading reports a Setext heading, or a line of seven or more
// '#' outside a code block.
var ErrUnsupportedHeading = errors.New("unsupported heading")
```

テストは、両版の `Structure` を比べ、一致しないときに次を示して失敗する（AC-29）。

-   一致しなかった項目（見出しの並び、またはコードブロックの数）。
-   見出しの並びが違うときは、最初に食い違った見出しの位置（何番目の見出しか、両版のその見出しの行番号とレベル）。
-   どちらかのファイルがないときは、読めなかったファイルのパス（スキップしない）。

テストの検査は `errors.Is` で行い、エラーの文言は比べない（CLAUDE.md、Testing Strategy）。表の数や行の数は比べない（H-04 の最後の点。比べると要件の変更になる）。HTML のブロックと引用の中の見出しは、README が使っていないので扱わない。

あわせて、`README.en.md` の 1 行目が同期の印の形（2.4 節）であり、印が 1 つだけであることを検査する。AC-38 の差分翻訳が印に頼るので、印が壊れたことを `/mktrans` を呼ぶ前に CI で知るためである。印のブロブが今の `README.md` と一致することは検査しない（AC-29 が求めるのは構造の一致だけで、内容の対応は AC-37 の手順とレビューで保つ。要件 §4.5）。

### 4.2. リンクとパスの検査（AC-15・AC-25）

**README と開発者向けの文書のリンク。** `TestReadmeLinks` を、`README.md`・`README.en.md`・`cli.md`・`development.md` の 4 つに広げる。各文書のリンクについて次を確かめ、失敗のメッセージで文書とリンクを示す。

-   相対リンクのファイルが、リンク元の文書のディレクトリからの相対パスで存在する。
-   `#…` のリンクと、`<file>#…` のリンクの見出しが、リンク先の文書の、コードブロックの外の見出しから `AnchorFromHeading` で作ったアンカーにある。今のテストは同じ文書の中の見出しだけを確かめるので、文書をまたぐアンカーの検査を加える。

**指示と規約の文書が示すパス。** CLAUDE.md・AGENTS.md・`.claude/commands/` 配下・`docs/dev/` 配下の文書について、相対リンクの先と、バッククォートの中に書いた `docs/dev/` で始まり `.md` で終わるパスが、存在するファイルであることを確かめるテストを `cmd/yt2column` に加える（`doc-guard` で走る）。コードブロックの中のリンクと、`<…>` の形の仮の値のリンク（例：`.claude/commands/mkreq.md` の `](<url>)`）は対象にしない。移動の PR より前に作られたブランチが、マージの後に旧パスを持ち込むと、エージェントが規約を読めないまま進む。この検査は、文書の値（パス）を確かめるもので、旧パスがないことを確かめるものではない（CLAUDE.md、Testing Strategy の "Document tests check values"）。過去のタスク文書は、当時の記録なので対象にしない（AC-15 は 7.3 節の差分の検査で確かめる）。

### 4.3. 同期の手順の止まり方（AC-37）

`/runplan`・`/fixpr`・`/runpr` は、`README.md` を変えた実行で `/mktrans README.md` を下位の手順として呼ぶ。「`README.md` を変えた」は、PR のブランチの変更（`$(git merge-base origin/main HEAD)..HEAD` のコミットと作業ツリー）に `README.md` があることで判定する。フェーズのグループの範囲（`${PHASE_START}..HEAD`）は使わない。同じ PR の前のグループや前のセッションの変更を見落とすためである。

| `/mktrans` の結果 | 呼び出し元の振る舞い |
|---|---|
| 反映した | `README.en.md` を自分のコミットに含める。用語集にない語を最終の報告と PR の本文に挙げる。手順を続ける |
| 反映済み・対象外 | 何もせずに続ける |
| 止まった（理由） | push せずに止まり、理由と復旧の手順を報告する。`/mktrans` が書きかけた `README.en.md` の変更は `git restore README.en.md` で戻す。`/runplan` と `/runpr` では、`README.md` を変えたコミットは push されずに手元に残る。`/fixpr` では、Phase 3 の修正はまだコミットされていないので、作業ツリーに残る。そのため、次の `/fixpr` は、Phase 3.5 で範囲の外の変更を断って止まったときと同じく、汚れた作業ツリーで止まる。報告に、残った変更とその扱い（コミットするか捨てるか）を人間が決めることを書く |

各コマンドで手順を置く場所は、`README.md` を変えうる最後の手順の後、検査と push の前である。

-   **`/runplan`:** `_lib/task-pr-handoff.md` の Part B の手順 1（ローカルの CodeRabbit のレビュー。レビューの修正で `README.md` が変わりうる）の後、手順 2（CHECKS）の前。同期でコミットしたときは、`runplan.md` が CHECKS として渡す green gate（`:82`）を走らせ直す。この位置は、Part B を使う文書のコマンド（`mkreq` など）にも当たるが、それらは `README.md` を変えないので、手順は何もしない。
-   **`/fixpr`:** Phase 3 の修正のエージェントが戻った後、Phase 3.5（範囲の検査）の前に、このセッションが実行する。修正のエージェントは `.git` に書く git のコマンドを使えず、`/mktrans` も自分のレビューのサブエージェントを起こすためである。Phase 4.5（ローカルの CodeRabbit のレビュー）が `README.md` を変えるコミットをしたときは、その push の前に、もう一度同期の手順を実行する。`README.en.md` の変更は、2.5 節のとおり範囲に入る。
-   **`/runpr`:** 単独の変更の PR を作る手順 1 で、ローカルのレビュー（`.claude/commands/runpr.md:115-118`）の後、push（同 `:131`）の前。チェックアウトしていないブランチを渡されたときは、ローカルのレビューと同じく同期の手順も実行できないので、そのブランチの変更に `README.md` があれば止まって報告する。加えて、手順 4 のマージの質問の前に、印を検査する（2.4 節。検査だけで、反映はしない。`README.en.md` がなければ検査しない）。`/runpr` が呼ぶ `/fixpr` の中の変更は、`/fixpr` の手順が扱う。

**`main` を取り込むときの衝突。** 2 つの PR がどちらも `README.md` を変えて同期すると、`README.en.md` の 1 行目の印が必ず衝突する。どちらか一方の印を選ぶと、印と本文の対応が崩れる（ブランチの印を選ぶと `main` の変更を 2 度訳し、`main` の印を選ぶとブランチの変更を 2 度訳す）。衝突したときは、`README.en.md` を `main` の内容（印と本文）で丸ごと置き換え、`README.md` の衝突を解いてから、同期の手順を実行する。これで差分はブランチの変更だけになる。この規則を `/runpr` の `main` を取り込む手順と、`/mktrans` の復旧の手順に書く。

**止まったときの復旧の手順。** `/mktrans` の報告に、理由ごとに次を示す。

| 理由 | 復旧 |
|---|---|
| 印がない・形が違う・2 つある | 人間が、`README.en.md` が反映している `README.md` の内容を特定し、その `main` のコミットの `README.md` のブロブの ID（`git rev-parse <commit>:README.md`）を印に書く。PR のレビューで確かめる |
| 印の ID が取り出せない | `git fetch origin 'refs/pull/*/head'` で PR の先頭のコミットを取り込んでから、もう一度実行する。それでもなければ、上の行と同じく人間が印を書く。最後の手段として、`README.en.md` を消して `/mktrans` を単独で呼び、全文を翻訳する |
| 翻訳のレビューで解消できない指摘 | 指摘を報告に挙げ、人間が判断する |

## 5. セキュリティ考慮事項 (Security Considerations)

N/A。本タスクは、外部コマンドの起動、API キーや Webhook の URL の扱い、キャッシュの読み書き、ネットワークへの送信、信頼できない文字列のプロンプトや記事への埋め込みを変えない（要件 §4.2。条件付きの指針 `docs/dev/security.md` の適用条件に当たらない）。脅威モデルの図は作らない。

補足として、次の 3 点は変わらないことを確かめた。

-   `fixprguard` の変更は、診断で示す手引きのパスの文字列（`cmd/fixprguard/main.go:132`）だけである。範囲の判定とメンションの検査は変えない（2.5 節）。
-   同期の手順は、`fixprguard` の新しい呼び出しも、GitHub に投稿する新しい文も加えない。そのため、メンションの検査の位置を固定するテスト（`TestPostingPathsCheckMention` など）の前提は変わらない。
-   `/mktrans` が読む `README.md` と差分は、リポジトリの中の文書であり、翻訳の結果は `README.en.md` にだけ書く。

## 6. 処理フローの詳細 (Processing Flow Details)

### 6.1. `/mktrans` の振る舞いの判定

図 6-1 は、改訂後の `/mktrans` が入力から振る舞いを決める流れである。矢印は判定の順序を表す。

```mermaid
flowchart TD
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef problem fill:#ffe6e6,stroke:#d62728,stroke-width:2px,color:#7b0000;

    Start(["入力のファイル"]) --> IsReadme{"README.md か"}
    IsReadme -->|"いいえ"| Ask["止まり、利用者に尋ねる"]
    IsReadme -->|"はい"| Exists{"README.en.md があるか"}
    Exists -->|"ない、下位の手順"| Skip(["対象外"])
    Exists -->|"ない、単独"| Full["全文を翻訳し、印を書く"]
    Exists -->|"ある"| Marker{"印は 1 つで、形が正しく、<br>ブロブを取り出せるか"}
    Marker -->|"いいえ"| Stop["止まり、理由と復旧を報告する"]
    Marker -->|"はい"| Same{"印は今の README.md の<br>ブロブと同じか"}
    Same -->|"はい"| Done(["反映済み"])
    Same -->|"いいえ"| Diff["印のブロブとの差分を翻訳し、<br>印を書き換える"]
    Full --> Review{"翻訳のレビューの<br>指摘を解消できたか"}
    Diff --> Review
    Review -->|"いいえ"| Stop
    Review -->|"はい"| Commit{"下位の手順か"}
    Commit -->|"はい"| Report["結果と新しい語を報告する<br>(コミットしない)"]
    Commit -->|"いいえ"| Self["コミットし、<br>用語集を更新する"]

    class Full,Diff,Report,Self enhanced
    class Ask,Stop problem
```

`([…])` の形のノードは、何も変えずに終わる結果である。

**図 6-1 の凡例**

```mermaid
flowchart LR
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef problem fill:#ffe6e6,stroke:#d62728,stroke-width:2px,color:#7b0000;
    subgraph Legend["Legend"]
        L1["翻訳して結果を残す手順"]
        L2["止まる手順"]
    end
    class L1 enhanced
    class L2 problem
```

### 6.2. 旧パスの一括置換（H-05）

移動した文書を指す参照は、ファイル名を含む形で現れる（`364c95b8` のリポジトリ全体で、`requirements_process.md` が 49 か所、`package_reference.md` が 165 か所など、11 ファイルで計 354 か所）。置換は、旧パスごとに新しいパスを決めた対応表で行う。

| 旧パス（`docs/dev/developer_guide/` の下） | 新しいパス |
|---|---|
| `requirements_process.md`・`test_organization.md`・`mermaid_reference.md`・`package_reference.md`・`task_identification.md` | `docs/dev/conventions/` の同名のファイル |
| `mergepr_guide.md`・`mergepr_guide.ja.md` | `docs/dev/guides/mergepr_guide.md` |
| `fixprguard_guide.md`・`fixprguard_guide.ja.md` | `docs/dev/guides/fixprguard_guide.md` |
| `pricing_update.md`・`claude_output_limit.md` | `docs/dev/guides/` の同名のファイル |

-   相対リンク（例：過去のタスク文書の `../../dev/developer_guide/…`）は、ディレクトリの部分だけを置き換えるので、相対の深さは変わらない。
-   英語版と日本語版の手引きを並べて挙げていた行は、同じパスが 2 度並ぶ形のままにする（AC-13・AC-14）。
-   **過去のタスクのスクリプト:** `docs/tasks/0032_japanese_pr_title/check_rules.sh:654`・`:657` は、`mergepr_guide` の日本語版の手引きと英語版の手引きを読む、そのタスクの検査のスクリプトである。文書と同じく、パスだけを置き換える。置き換えた後は、どちらも日本語の手引きを読み、英語の文を探す検査は失敗する。この種のスクリプトはマージの後は保守しない（`docs/dev/conventions/requirements_process.md` §4）ので、直さない。
-   **ファイルを指さず、ディレクトリだけを指す記述は置き換えない。** 過去のタスク文書には、`docs/dev/developer_guide/` や `docs/dev/developer_guide/*.md` のように、ディレクトリだけを述べる箇所がある（`docs/tasks/0033_multiple_trusted_reviewers/02_architecture.md:189`、`docs/tasks/0037_runpr_merge_recommendation/01_requirements.md:114`、同 `02_architecture.md:297`、`docs/tasks/0041_claude_effort_max_tokens/01_requirements.md:55`・`:127`、同 `design_handoff.md:71`）。これらは当時の言語の方針を述べた経緯の記録で、移動した特定の文書を指していない。新しいディレクトリは 2 つに分かれるので、1 つの置換先を決められない。AC-13 の対象は「移動した文書を旧パスで指している参照」なので、この 6 か所は対象外とする。CLAUDE.md の `:25` と `mktrans.md:21` のディレクトリへの言及は、AC-01・AC-39 の改訂で消える。
-   見出しへのアンカー付きのリンク（`developer_guide/…md#…`）は、`364c95b8` にもない。

## 7. テスト戦略 (Test Strategy)

### 7.1. 単体テスト（ガードテスト）

-   **構造の一致（AC-29・AC-30）:** 4.1 節のテストを `cmd/yt2column` に置く。`doc-guard` の CI ジョブが走らせるパッケージ（`.github/workflows/ci.yml:187`）なので、README だけを変えた PR でも走る。`make test` にも含まれる。`ParseStructure` 自体のテストは、共有パッケージに、見出しの数の違い、レベルの違い、コードブロックの数の違い、コードブロックの中の `#`、4.1 節の拒否する形のそれぞれの行を持つ表で置く。
-   **README の両版（AC-31）:** 3.7 節の振り分けに従う。各テストは版ごとのサブテスト（`t.Run` の名前に版のパス）で回し、片方の版が値を欠くと、その版の名前で失敗する。
-   **移った値（AC-32）:** 3.7 節の振り分けに従う。
-   **拒否の理由の表（H-03）:** 3.7 節の `rejection_reasons` の行と、表示の区分の表のとおり。
-   **README の先頭のリンク（AC-23・AC-28）:** 両版の、最初の見出しより前の部分に、もう一方の版へのリンクがあることと、両版に `cli.md`・`development.md` へのリンクがあることを確かめる。リンクの先が存在することは 4.2 節の検査が確かめる。
-   **UI が示すパス（H-09）:** CLI のヘルプ（`newFlagSet` と使い方の文）が `docs/dev/guides/cli.md` を示し、そのファイルが存在すること、サーバの診断が `README.en.md` を示し、そのファイルが存在すること、`fixprguard` の `guidePath` のファイルが存在すること（AC-16）を確かめるテストを加える。どれも文書の値（パス）の検査であり、文の意味は確かめない。拡張の英語のメッセージ（AC-35）は、`en.ts` の文字列が `README.en.md` を含むことを `static` で確かめる。
-   **指示と規約の文書が示すパス:** 4.2 節。
-   **CI の判定（AC-33）:** `ciChanges.test.ts` の、拡張のジョブを走らせるパスの表に、`README.en.md` と `docs/dev/guides/development.md` を加える。走らせないパスの表に、`docs/dev/guides/cli.md` を加える（拡張のテストは `cli.md` を読まない）。

各テストは、守る対象を壊して失敗することを確かめてからコミットする（CLAUDE.md、Testing Strategy）。例：`README.en.md` の見出しを 1 つ消す、`README.md` だけにコードブロックを加える、`README.en.md` を消す。

**CI で走らない検査。** `TestReadmeHostExitCodes`（`cmd/yt2column-nmhost`）は `doc-guard` のパッケージに入っておらず、`check-changes` は `*.md` だけの変更をコードの変更としない（`.github/workflows/ci.yml:36`）。そのため、`README.md` または `README.en.md` だけを変えた PR では、CI はこのテストを走らせない。CI のジョブの分担は変えない（要件 §2.3）ので、この点を CLAUDE.md の Development Notes に書く（AC-41）。

### 7.2. 統合テスト

N/A。本タスクは外部のサービスを呼ぶ振る舞いを変えない。

### 7.3. 差分の検査（`_checks/`）

変更そのものの性質を確かめる AC は、本タスクの `_checks/` のスクリプトで、マージベースと PR の先頭のコミットを比べて確かめる（`docs/dev/conventions/requirements_process.md` §4）。各スクリプトは、その AC の変更を含む PR で 1 度走らせ、出力と 2 つのコミットの ID を実装計画に記録する。

| AC | 走らせる PR | 確かめること |
|---|---|---|
| AC-08 | 移動の PR（フェーズ 1） | `docs/dev/conventions/` の 5 ファイルが、移動前の同名のファイルと、リンクのパスの違いを除いて同じである。後の PR で `package_reference.md` に行を加えるのは、移動ではない通常の変更で、この検査の対象外である |
| AC-09（の一部） | 移動の PR | `mergepr_guide.md`・`fixprguard_guide.md` が、移動前の `.ja.md` と、リンクのパス、AC-11 で加えた段落、AC-26 の参照先の違いを除いて同じである。残った差分の行を一覧で出力し、レビューで読む |
| AC-12・AC-13 | 移動の PR | `docs/dev/developer_guide/` がない。移動した 11 ファイルの旧パス（ファイル名を含む形）が、本タスクの文書のほかにない |
| AC-14 | 移動の PR | 過去のタスク文書の変更された行が、6.2 節の対応表を旧の行に当てはめた結果と一致する（パス以外の変更がない） |
| AC-15 | 移動の PR | 置き換えた相対リンクが、リンク元のファイルのディレクトリから、存在するファイルを指す。過去のタスク文書、CLAUDE.md、AGENTS.md、`.claude/commands/`、`docs/dev/` のすべてが対象である |

**AC-11（H-06）:** 英語版の手引きを削除する前に、英語版の手引きと日本語版の手引きを節ごとに読み比べる。本書の作成時に確かめた事実は次のとおりである。

-   2 つの組とも、英語版の手引きと日本語版の手引きを最後に変えたコミットは同じである（`mergepr_guide` は `778317f3`、`fixprguard_guide` は `6022d4b5`）。
-   見出しの数は、2 つの手引きで同じである。`fixprguard_guide` は見出しのレベルの並びも同じである。
-   `mergepr_guide` の 1 行の差（238 行と 237 行）は、英語版の 140 行目の付近から見出しの行の位置が 1 行ずれることによる。段落の折り返しの違いの可能性があるが、内容の違いかどうかは確かめていない。

節ごとの読み比べの結果と、英語版にだけあった記述を日本語版に加えたかどうかは、実装計画に記録する。

### 7.4. セキュリティのテスト

N/A（5 章）。

## 8. 実装の優先順位 (Implementation Priorities)

以下、本タスクの実装の区切りを「フェーズ」と呼ぶ（`/fixpr` の Phase とは別である）。要件 §1「進行中のタスクとの順序」により、F-001・F-002・F-006 は先に進めてよく、README の作業（F-003・F-004、README を読むガードテストの変更）は 0044 の PR-7（マージ済み、#550）と 0046 の PR-4（本書の作成時に未マージ）の後に始める。

| フェーズ | 内容 | 要件 | 前提 |
|---|---|---|---|
| 1 | developer_guide の分割。英語版の手引きの確かめと削除、`pricing_update.md`・`claude_output_limit.md` の翻訳、旧パスの一括置換、`fixprguard` と 2 つのガードテストのパスの変更、指示と規約の文書が示すパスの検査（4.2 節）、`_checks/` の AC-08〜AC-15 | F-002 | なし |
| 2 | 言語の原則。CLAUDE.md・`_context.md`・AGENTS.md の改訂（README と開発者向けの文書に関わる記述は、それらを作るフェーズ 6 で合わせる） | F-001 | フェーズ 1（新しいパスを書くため） |
| 3 | `/mktrans` の改訂と、`/runplan`・`/fixpr`・`/runpr` と `_lib/task-pr-handoff.md` の同期の手順 | F-006 | なし |
| 4 | 共有パッケージ `internal/doctestutil` の新設と、既存の補助関数とそのテストの移動。この PR では、今ある補助関数を移すだけで、README の版とロケータ、構造の抽出は加えない | F-004 の準備 | なし |
| 5 | README の振り分け。`cli.md`・`development.md` の新設と、移った値を読むガードテストの付け替え | F-003 | 0046 の PR-4 のマージ、フェーズ 4 |
| 6 | README の日本語化、`README.en.md` の作成、README の版とロケータ、両版のガードテスト、構造の抽出と構造の一致のテスト、CI の判定、用語集、UI の参照先の変更、CLAUDE.md の Development Notes | F-004・F-005 | フェーズ 3・5 |

-   **英語から日本語への翻訳（H-13）:** 改訂後の `/mktrans` は `README.md` から `README.en.md` への翻訳だけを扱う。そのため、`README.md` の日本語化、`pricing_update.md`・`claude_output_limit.md`・`cli.md`・`development.md` の日本語の内容は、`/mktrans` を使わずに書き、`/japrose` で整え、翻訳の正確さを `/mktrans` のレビューの手順（`.claude/commands/_lib/review-subagent-pattern.md` と `mktrans.md` の Accuracy checklist）で確かめる。改訂前の `/mktrans` は使わない（`README.ja.md` を出力してしまうため）。具体的な手段と順序は実装計画で決める。
-   **途中の PR での README（H-02）:** フェーズ 5 の後、フェーズ 6 の前は、`README.md` は英語のままで、`README.en.md` はない。この間のガードテストは英語の `README.md` だけを読む。フェーズ 3 の同期の手順は、`README.en.md` がなければ何もしない（2.4 節）ので、この間に `README.md` を変える PR が英語の `README.en.md` を作ることはない。`README.en.md` は、フェーズ 6 で `README.md` を日本語にした後、`/mktrans` を単独で呼んで作る。英語のロケータを `README.en.md` に向け、日本語のロケータを加えるのは、その PR である。PR の分け方は実装計画で決める。
-   **移動の PR より前のブランチ:** フェーズ 1 のマージの時点で開いているブランチは、旧パスを持ち込みうる。指示と規約の文書については、4.2 節の検査が CI で失敗させる。過去のタスク文書については検査しないので、フェーズ 1 の PR の本文で、開いている PR に `main` を取り込んでパスを直すよう知らせる。
-   **`make install-fixprguard`（AC-18）:** フェーズ 1 を含む PR の本文に、マージの後で入れ直しが必要であることを書く。入れ直すまでは、インストール済みの `fixprguard` が旧パスを示す（要件 §4.4）。

## 9. 将来の拡張 (Future Extensibility)

-   **二言語の文書を増やすとき:** `/mktrans` の組の表と、共有パッケージの版の一覧に行を加え、構造の一致のテストをその組にも当てる。同期の印は文書ごとに置く。ただし、二言語で持つのは README だけとする方針（AC-04）を変えることになるので、要件の変更として扱う。
-   **#503 の後:** 拡張が Job API を使うようになると、Job API の一部が利用者の観測するものになる。そのときは、境界の規則に従って、利用者が観測する部分を README に戻す。`development.md` の表と、それを読むガードテスト（3.7 節）はそのまま使える。

## 10. 設計への申し送りへの対応 (Design Handoff)

| 項目 | 対応 |
|---|---|
| H-01 | GitHub の表示で確かめ、68 個の見出しで `anchorFromHeading` と一致した（3.4 節）。確かめていない文字の扱い、`cli.md`・`development.md` も同じ補助関数で確かめることを決めた（3.4 節・4.2 節） |
| H-02 | README のロケータは Go の共有パッケージと TypeScript の補助に 1 度ずつ定義する（3.3 節・3.5 節）。失敗のメッセージに版のパスを含める（7.1 節）。途中の PR での README の扱いの原則を 8 章に書き、PR の分け方は実装計画に任せる。日本語のリテラルは AC-05 の例外に入れる（3.3 節） |
| H-03 | `development.md` の 3 列の表をサーバと拡張の定義と照らし、README の両版の 2 列の表を、表示の区分に直して `development.md` の表と照らす（3.7 節）。`has-extension-changes.sh` に `development.md` を加える（3.1 節） |
| H-04 | ATX の見出しを数え、走査と GitHub の解釈がずれうる形を拒否する。既存の走査を共有パッケージに移して規則を変えずに使う。テストは `cmd/yt2column` に置き、`doc-guard` で走らせる。表は比べない（4.1 節・7.1 節） |
| H-05 | 6.2 節の対応表で置き換え、`_checks/` で AC-13〜AC-15 を確かめる（7.3 節）。並んだ同じパスはそのままにする。ディレクトリだけを指す 6 か所は対象外とする。過去のタスクのスクリプトもパスだけを置き換える（6.2 節） |
| H-06 | 本書の作成時の事実を 7.3 節に記録した。節ごとの読み比べの結果は実装計画に記録する |
| H-07 | 範囲に入ることを `internal/fixprguard/scope.go` で確かめた（2.5 節）。`/fixpr` の Phase 3 の後で同期する。用語集は範囲に入らないので、下位の手順では変えない（3.8 節・4.3 節） |
| H-08 | CLAUDE.md とコマンドの文書を読むテストが検査する見出しと行を変えない。補助関数の呼び出しだけが変わる（3.7 節） |
| H-09 | ヘルプ・サーバの診断・`fixprguard` の診断が示すパスが存在することを確かめるテストを加える（7.1 節） |
| H-10 | 同期点をコミットからブロブの ID に変え、同じコミットでの変更と、`README.en.md` だけの修正の両方で未反映の変更が落ちないようにした。印のブロブを取り出せることを、`main` の上で印を `README.md` の内容と一致させることで保つ（1.3 節・2.4 節・3.8 節） |
| H-11 | `364c95b8` で一覧を作り直し、テストごとに読む文書を決めた（3.7 節）。前後の表は実装計画で作る |
| H-12 | `364c95b8` の README で振り分けの一覧を作った（3.6 節）。0046 の PR-4 の後の README で確かめ、PR-4 が想定の外を変えていたら作り直す |
| H-13 | 改訂前の `/mktrans` を使わず、日本語の内容は直接書いて `/japrose` と翻訳のレビューで確かめる（8 章）。具体的な手段と順序は実装計画で決める |

## 11. 受け入れ基準と設計の対応 (AC Traceability)

検査の種類は、実装計画で最終的に決める。ここでは設計の該当箇所と、想定する検査の種類を示す。

| AC | 設計の該当箇所 | 想定する検査 |
|---|---|---|
| AC-01〜AC-04a | 1.2 節、3.1 節（CLAUDE.md） | `manual` |
| AC-05 | 3.3 節 | `manual` |
| AC-06 | 1.2 節の表 | `manual` |
| AC-07 | 3.1 節、3.2 節 | `manual` |
| AC-08 | 2.2 節、7.3 節 | `diff` |
| AC-09 | 2.2 節、7.3 節 | `diff` と `manual`（日本語であること） |
| AC-10 | 3.1 節、8 章 | `manual` |
| AC-11 | 7.3 節 | `manual` |
| AC-12・AC-13 | 6.2 節、7.3 節 | `diff` |
| AC-14 | 6.2 節、7.3 節 | `diff` |
| AC-15 | 4.2 節、7.3 節 | `diff`（移動の PR）と `test`（指示と規約の文書が示すパス） |
| AC-16 | 3.1 節、7.1 節 | `test` |
| AC-17 | 3.7 節 | `test` |
| AC-18 | 8 章 | `manual`（PR の本文） |
| AC-19〜AC-22 | 3.6 節 | `manual` |
| AC-23 | 3.1 節、7.1 節 | `test`（リンクがあること） |
| AC-24 | 2.2 節 | `manual`（日本語であること。英語版のファイルがないことも読み合わせで確かめる） |
| AC-25 | 4.2 節 | `test` |
| AC-26 | 3.1 節 | `manual` |
| AC-27 | 3.6 節、8 章 | `manual` |
| AC-28 | 3.1 節、7.1 節 | `test`（リンクがあること） |
| AC-29・AC-30 | 4.1 節、7.1 節 | `test` |
| AC-31・AC-31a | 3.5 節、3.7 節 | `test` |
| AC-32 | 3.7 節 | `manual`（実装計画の前後の表） |
| AC-33 | 3.1 節、7.1 節 | `test` |
| AC-34 | 3.1 節 | `manual` |
| AC-35・AC-35a・AC-36 | 3.1 節、7.1 節 | `test` と `static` |
| AC-37 | 4.3 節 | `manual` |
| AC-38・AC-39 | 3.8 節、6.1 節 | `manual` |
| AC-40 | 3.1 節 | `manual` |
| AC-41 | 3.1 節、3.7 節、7.1 節 | `manual` |
