# アーキテクチャ設計書：/runpr のマージの質問を、画面に収まる 1 つの問いにする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-12 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（要件定義書。以下、要件書）の設計である。既存のファイルの内容は、コミット `41b2d31e` で確かめた。`file:line` の行番号は、このコミットでの行番号である。F-NNN・AC-NN は要件書の項番を、H-NN は [design_handoff.md](design_handoff.md)（以下、申し送り）の項番を指す。申し送りへの対応は付録 A に、受け入れ基準との対応は付録 B にまとめる。

本書は、要件書 6 章の用語（マージの質問・deferred のスレッド・deferred の問い・除外スレッド・追跡 issue・詳細ファイル・要約・NEXT・PRE_MERGE・LAST_REVIEWED・Escalation・LIMITS_NOTE・旧値）を、要件書と同じ意味で使う。そのほかに、次の用語を使う。

-   **詳細ディレクトリ:** `/runpr` の 1 回の実行で 1 つ作る、詳細ファイルを置くディレクトリ（3.2）。
-   **問いの回:** 1 回の実行の中で、deferred の問いかマージの質問を尋ねるたびに 1 つ増える番号。1 から数える。詳細ファイルの名前に使う（3.2）。
-   **承認のメッセージのファイル:** `mergepr.md` step 2 が準備のディレクトリに書く squash commit の件名のファイルと本文のファイル、および `runpr.md` が持つ承認の行のない版の本文（`runpr.md:723-725`）。マージに使うメッセージの、唯一の元である。

## 1. 設計の全体像 (Design Overview)

### 1.1. 設計原則

-   **step の番号と見出しを変えずに、step 4 の中で組み替える。** deferred の問いは step 4 の新しい項目 1 にし、今の項目 1〜4 を 2〜5 に繰り下げる。`_lib/simple-process.md` は `runpr.md` の step 2〜4 と戻す手順を番号と見出しで参照し（`runpr.md:44-46`）、文書のガードのテストは step の見出しを定数として読む（`cmd/yt2column/fixpr_docs_test.go:42-44`、`cmd/yt2column/codex_review_docs_test.go:84`）。step の番号を保てば、それらを直さずに済む。変わる見出しは、要件どおり step 5 の 1 つだけである（AC-16）。
-   **マージに使うのは承認のメッセージのファイルだけで、詳細ファイルは読み戻さない。** 詳細ファイルは、承認のメッセージのファイルを最後に書いて件名を確かめた後に、その中身を写して書く表示用のファイルである。人が詳細ファイルを編集しても、マージのメッセージは変わらない（AC-12、H-01）。
-   **古い回の詳細ファイルを指さない。** 問いの回ごとに新しい名前のファイルを書き、前の回のファイルは上書きも再利用もしない。要約が示すパスは、常にその回のファイルである（AC-11、H-01）。
-   **今の手順を移し、写さない。** 追跡 issue に移す手順（`runpr.md:929-961`）は、マージの答えの後から deferred の問いの答えの後へ移す。戻す手順（`runpr.md:835-927`）は、deferred の問いからも同じ手順を呼び、読み替える点だけを書く。PR レビューを依頼する答え、承認なしでマージする答え、マージしない答え、PRE_MERGE の記録とマージの手順は変えない（AC-18）。
-   **会話には 1 項目 1 行の要約、詳細はファイルに置く。** 要約の行の種類と形は 3.3 で固定し、どの PR でも 12 行を超えないようにする。値が長くなりうるもの（squash commit の本文、PRE_MERGE の行、diff 記録、旧値や読めない値の詳細、LIMITS_NOTE の全文）は、すべて詳細ファイルに置く（AC-06〜AC-09）。
-   **新しい道具とテストは足さない。** 変更は、コマンドの定義と、それを説明する文書だけである。テストの変更は、step 5 の見出しの定数 1 つだけにする（7 章、H-03）。

### 1.2. 設計の際に決めたこと（2026-11-12）

-   **deferred の問いは、`runpr.md` step 4 の新しい項目 1 に置き、今の項目 1〜4 を 2〜5 に繰り下げる（H-02）。** ADR: なし（`runpr.md` の中で閉じる配置であり、後のタスクもほかのコンポーネントも縛らないため）。
    -   **理由:** step の番号と見出しが変わらないので、`_lib/simple-process.md` の参照も、`postingSections` の節（`codex_review_docs_test.go:60-77`）と `TestFixprguardCallSites` の節（`fixpr_docs_test.go:726-730`）も、そのまま使える。`SectionByHeading` は同じ水準かそれより上の見出しまでを節とする（`internal/doctestutil/markdown.go:90-113`）ので、step 4 の中の項目はすべて step 4 の節に入る。戻す手順と同じ step にあるので、deferred の問いから戻す入口（AC-05a）も同じ節の中で書ける。step 4 の項目の番号を参照する箇所は、戻す手順 1 の「ask step 4 again, from its item 1」（`runpr.md:843`）だけで、ほかの文書は step 4 の項目を番号で参照していない（`git grep` で確かめた）。
    -   **採らなかった案:** step 3 の末尾に置く案。`postingSections` に step 3 を足す必要があるうえ、step 3 の一覧の命令が本文を出さないことを確かめるテスト（`fixpr_docs_test.go:187-189` の `listings_print_no_body`）が、新しく step 3 に入る `gh issue list` を扱うことになる。新しい step を作る案。後ろの step の番号がすべて変わり、`simple-process.md` とテストの定数を直すことになる。
-   **詳細ファイルは、実行ごとに `mktemp -d /tmp/runpr.XXXXXX` で作る詳細ディレクトリに、問いの回ごとに別の名前で書く（H-01）。** 名前は、deferred の問いでは `deferred-<回>.md`、マージの質問では `merge-<回>.md` とする（例: `/tmp/runpr.Ab12Cd/merge-3.md`）。Claude Code でも OpenCode でも、同じ場所に書く。ADR: なし（`runpr.md` の中で閉じる、ファイルの置き場所の決定であるため）。
    -   **理由:** パスは約 30 桁で、要約の 1 行として表示幅 80 桁に収まる（AC-07）。`mktemp -d` は、ほかの利用者が読めない（モード `0700`）、名前を推測できないディレクトリを作る。作業ツリーの外にあるので、`git status --short` の出力は変わらない（AC-06）。回ごとに別の名前にするので、エディタで開いたままの前の回のファイルと、今の回のファイルを取り違えない。OpenCode は、作業ディレクトリの外のパスを `external_directory` の権限で判定し、その既定は許可である（OpenCode の公式ドキュメント「Permissions」、2026-11-12 に確かめた）。利用者の OpenCode の設定には、この権限の規則がない。
    -   **採らなかった案:** Claude Code ではスクラッチパッドに書く案。スクラッチパッドのパスは、worktree の名前とセッションの ID を含んで 100 桁を超え、AC-07 の 80 桁に収まらない。`mergepr prepare` の準備のディレクトリに書く案。linked worktree ではそれが作業ツリーの中にあり（`.claude/commands/mergepr.md:34-41`）、`discard` で消え、deferred の問いを尋ねる時点ではまだない（AC-01a）。`.claude/settings.json` に `/tmp/runpr.*` への書き込みの許可を足す案。エージェントの権限の設定を変えることになり、開発用の道具の信頼の境界に触れる。同じ名前のファイルに上書きする案。古い中身を開いたままの利用者が、取り違えうる。
    -   **受け入れること:** Claude Code では、権限のモードによって、詳細ディレクトリを作るときと詳細ファイルを書くときに承認を求められることがある。そのときは、利用者がそのつど許可する。
-   **AC-22 の手動の確認は、見本の値で書いた要約と選択肢を、OpenCode のエージェントに `runpr.md` の書式どおりに出させて行う（H-04）。** ADR: なし（確かめ方はこのタスクの中で閉じるため）。
    -   **理由:** 画面に収まるかは、要約の行数と各行の長さ、question の文、選択肢の数と文言だけで決まり、値が本物かどうかには左右されない。AC-22 の状態（diff 記録の再記録、PR の説明に追記できなかった訂正、`#未記入` の行、期限が近い LIMITS_NOTE、読めない実装モデル要件がすべてそろう PR）を実際に作るのは、手間に見合わない。
    -   **採らなかった案:** AC-22 の状態をすべて満たす PR を作り、`/runpr` を走らせて確かめる案。
-   **マージの質問の question の 1 行に、PR 番号と、要約と詳細ファイルを確認するよう促す句を入れる（H-04）。** 例:「PR #859 をどうしますか（要約と詳細ファイルを確認のうえ）」。ADR: なし（文言はこのタスクの中で閉じるため）。
    -   **理由:** 0088 の chief は、質問と選択肢を画面のとおりに人へ示して中継する（[ADR-0046](../../dev/adr/0046-chief-of-staff.md)）。question の文に入れておけば、chief を通して答える人も、要約を見ずに選ぶことを避けやすい。question の文は 1 行のままなので、AC-10 を満たす。
    -   **採らなかった案:** question の文を PR 番号と問いだけにする案。

### 1.3. なぜ今の仕組みでは足りないか

今のマージの質問は、`mergepr.md` step 3 の承認に要る項目（件名と本文の全文を含む）、PRE_MERGE、diff 記録の再記録、deferred と除外スレッド、NEXT とモデルを 1 つのメッセージに出す（`runpr.md:729-766`）。そして、最大 3 問を 1 回の `AskUserQuestion` で尋ねる（`runpr.md:766-806`）。問いの数を減らすだけでは、本文と PRE_MERGE が長い PR で、メッセージが画面からはみ出す。メッセージを短くするだけでは、3 つの問いの選択肢が画面からはみ出す。そのため、メッセージと問いの両方を変える必要がある（要件書 1 章）。deferred のスレッドの問いを残したまま 1 問にまとめると、選択肢が 4 つを超える。そのため、deferred のスレッドは前の別の問いへ移す（F-001）。

### 1.4. 概念モデル

`/runpr` step 4 の 2 つの問いと、それぞれが読むデータの関係を示す。矢印 A → B は「B が A の中身を表示に使う」か「A の答えで B へ進む」を表し、矢印のラベルでどちらかを示す。

```mermaid
flowchart TD
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    FR[("fixpr の Final report<br>（deferred・除外スレッド）")]
    DQ["deferred の問い<br>（step 4 項目 1）"]
    DF[("詳細ファイル<br>deferred-回.md")]
    PREP["mergepr の準備と<br>メッセージの起草<br>（step 4 項目 2）"]
    MSG[("承認のメッセージのファイル<br>（件名・本文・承認の行のない版）")]
    MQ["マージの質問<br>（step 4 項目 3）"]
    MF[("詳細ファイル<br>merge-回.md")]
    SUM[("要約<br>（会話に出す 12 行以内）")]
    MERGE["PRE_MERGE とマージ<br>（step 4 項目 4・5）"]

    FR -->|"表示に使う"| DQ
    DQ -->|"書く"| DF
    DQ -->|"移した・直さない答えで進む"| PREP
    PREP -->|"書く"| MSG
    MSG -->|"写す"| MF
    MQ -->|"書く"| MF
    MQ -->|"書く"| SUM
    MQ -->|"マージする答えで進む"| MERGE
    MSG -->|"マージに使う"| MERGE

    class FR,DF,MSG,MF,SUM data
    class PREP,MERGE process
    class DQ,MQ enhanced

    subgraph Legend["Legend"]
        L1[("データ")]
        L2["変えない手順"]
        L3["新しく作るか変える手順"]
    end
    class L1 data
    class L2 process
    class L3 enhanced
```

詳細ファイルからマージへの矢印はない。マージは承認のメッセージのファイルだけを使う（1.1）。

## 2. システム構成 (System Structure)

### 2.1. 全体構成

step 3 を終えてから step 5 に至るまでの流れを示す。矢印は処理の順を表し、ラベルはその分岐の条件か答えである。

```mermaid
flowchart TD
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    S3["step 3<br>（レビュー・fixpr・CI）"]
    D{"未解決の deferred の<br>スレッドがあるか"}
    DQ["項目 1: deferred の問い"]
    ISS["追跡 issue に移す"]
    FIX["fixpr に caller override で渡す"]
    RET["戻す手順"]
    STOP["止まる"]
    P2["項目 2: mergepr の準備と起草"]
    MQ["項目 3: マージの質問"]
    P4["項目 4・5: PRE_MERGE とマージ"]
    S5["step 5: ブランチを切り替え、<br>NEXT を示す"]
    S2["step 2"]

    S3 --> D
    D -->|"ない"| P2
    D -->|"ある"| DQ
    DQ -->|"すべて追跡 issue に移す"| ISS
    DQ -->|"すべて直す"| FIX
    DQ -->|"番号を指定"| ISS
    DQ -->|"通常の進め方へ戻す"| RET
    DQ -->|"マージせずに止める"| STOP
    ISS -->|"番号を指定した答え"| FIX
    ISS -->|"すべて移す答え"| P2
    FIX --> S2
    RET --> S2
    S2 --> S3
    P2 --> MQ
    MQ -->|"マージ・承認なしでマージ"| P4
    MQ -->|"先に PR レビュー・除外スレッドを直す"| S2
    MQ -->|"通常の進め方へ戻す"| RET
    MQ -->|"マージしない"| STOP
    MQ -->|"メッセージを直す"| MQ
    P4 --> S5

    class S3,P2,P4,S2,STOP process
    class D,DQ,ISS,FIX,RET,MQ,S5 enhanced

    subgraph Legend["Legend"]
        L1["変えない手順"]
        L2["新しく作るか、置き場所か入口を変える手順"]
    end
    class L1 process
    class L2 enhanced
```

`ISS` と `FIX` と `RET` の中身は今の手順である。変わるのは、それを呼ぶ場所である（1.1）。PRE_MERGE の push が拒まれたとき、PRE_MERGE の後に新しいスレッドが現れたとき、マージの時に head が動いていたときは、今と同じく step 2 か step 3 に戻る（`runpr.md:968-991`）。そこから step 4 へ戻るときは、項目 1 から始める。そのため、新しく現れたスレッドが deferred のスレッドなら、マージの質問の前に deferred の問いを尋ねる（AC-11）。

### 2.2. コンポーネント配置

変えるファイルと、その変更の範囲を示す。新しく作るファイルはない。

| ファイル | 変更 | 関わる AC |
|---|---|---|
| `.claude/commands/runpr.md` | description（`:2`）、導入の段落（`:48-50`）、Authorization（`:82-83`、`:91`）、step 3 の deferred の扱い（`:672`、`:691`）、step 1 の LIMITS_NOTE の扱い（`:232-234`）、step 4 全体（`:712-991`）、step 5 の見出しと終わり方（`:993`、`:1035-1061`） | AC-01〜AC-18、AC-20、AC-21 |
| `.claude/commands/mergepr.md` | step 3 の「`runpr.md` の中で実行するとき」を述べる文（`:122-126`）だけ | AC-19 |
| `.claude/commands/fixpr.md` | deferred のスレッドを誰が尋ねるか（`:144-146`）と、caller override の呼び出し元（`:167-168`） | AC-21 |
| `.claude/commands/_lib/simple-process.md` | 除外スレッドと「fix all」の関係（`:334-339`）、THREADS の入力（`:380-384`）と、戻す手順を呼ぶ箇所（`:364-367`） | AC-21 |
| `.claude/commands/_lib/task-pr-handoff.md` | `/runpr` が次のコマンドを実行しうると述べる文（`:27-32`） | AC-20 |
| `CLAUDE.md` | 「Claude Commands」の `/runpr` の行（`:207`）と、その下の流れの説明（`:226-230`） | AC-20 |
| `docs/dev/conventions/requirements_process.md` | `/runpr` が次のコマンドを実行しうると述べる文（`:89`） | AC-20 |
| `cmd/yt2column/codex_review_docs_test.go` | 定数 `runprStep5`（`:84`）を新しい見出しに合わせる | AC-16 |
| `docs/dev/adr/0047-runpr-merge-question.md` | 「この決定は、まだ…反映していない」の文を、実装の PR で除く（[adr.md](../../dev/conventions/adr.md) の扱い） | - |

### 2.3. データフロー

マージの質問の 1 回で、承認のメッセージが会話と詳細ファイルとマージにどう渡るかを示す。

```mermaid
sequenceDiagram
    participant R as /runpr（step 4）
    participant M as mergepr
    participant F as 詳細ファイル
    participant U as 利用者

    R->>M: prepare（項目 2）
    R->>R: 件名・本文のファイルを書き、件名を確かめる
    R->>F: 件名・本文・接頭辞の記録・承認の行のない版などを写して書く（merge-回.md）
    R->>U: 要約（件名の行と詳細ファイルのパスを含む）と、1 つの問い
    U->>R: 答え
    alt メッセージを直す
        R->>R: 件名・本文のファイルを書き直し、件名を確かめ直す
        R->>F: 次の回の名前で書く（merge-回+1.md）
        R->>U: 新しい要約と問い
    else マージする
        R->>M: merge（件名・本文のファイルを使う）
    end
```

詳細ファイルは、この図のどこでも読み戻さない。

## 3. コンポーネント設計 (Component Design)

本章の文言の例は、`runpr.md` が実際に出す文の形を示す。`runpr.md` は英語で書くが、要約・選択肢・詳細ファイルの文は、会話の言語である日本語で出すよう指示する（`CLAUDE.md`「Language」）。

### 3.1. deferred の問い（`runpr.md` step 4 項目 1。F-001、H-02、H-04）

**尋ねる時点（AC-01a）。** step 3 を最後まで終えて step 4 に来た時点で、未解決の deferred のスレッドがあれば尋ねる。step 3 は、信頼できないスレッドが残るとき（`runpr.md:689-706`）、deferred と除外スレッドのほかに未解決のスレッドがあるとき（`runpr.md:684-688`）、CI が通らないとき（`runpr.md:707-710`）、未コミットの変更があるとき（`runpr.md:646-652`）に止まり、step 4 に来ない。そのため、step 4 の先頭に置けば、AC-01a の条件はそのまま満たされる。項目 1 は項目 2（`mergepr prepare`）より前にあるので、deferred の問いを尋ねる時点では、マージの準備はまだない。step 3 の、deferred のスレッドを「step 4 のために残す」とする 2 か所（`runpr.md:672`、`:691`）は、「step 4 の deferred の問いのために残す」と書き直す（AC-21）。

step 4 の冒頭の、問いの前に毎回メンションの一覧を読み直す規則（`runpr.md:717-720`）は、deferred の問いにも当てはめる。deferred の問いの答えは GitHub に投稿するので、問いの前に、ほかのアカウントのメンションがないことを確かめる。

対象は、最新の `fixpr` の Final report が deferred として挙げたスレッドで、`fixprguard list` の最新の出力で未解決のものである。除外スレッドは含めない（AC-05）。

**問いの形（AC-01）。** 1 つの問いだけを持つ単一選択の `AskUserQuestion` で、推奨の印はどの選択肢にも付けない。

-   **question の文:** 「PR #<番号> の deferred のスレッド <件数> 件をどうしますか（要約と詳細ファイルを確認のうえ）」。1 行である。
-   **選択肢（この順）:**

    | label | description | 示す条件 |
    |---|---|---|
    | すべて追跡 issue に移す | 追跡 issue に書き写し、各スレッドに返信して resolve する | 常に |
    | すべて直す | /fixpr で直し、レビューと CI を経てから次の問いへ進む | 常に |
    | 通常の進め方へ戻す | マージせず、要件定義書だけの PR に作り直す | 3 文書の PR のときだけ |
    | マージせずに止める | 何も投稿せずに止まる | 常に |

    選択肢は 3 つか 4 つで、`AskUserQuestion` の上限に収まる。

**メッセージ（AC-01）。** 会話には、問いの前に次の行だけを出す。12 行以内である。

```text
deferred のスレッド: 4 件（PR #859）
[1] fixpr.md の round cap の説明が step 番号と合わない
[2] テストの名前が確かめる内容を表していない
[3] ...
[4] ...
一部だけ直すとき: 番号を自由記述で（例: 1,3）。残りは追跡 issue へ移す
要確認: 操作制限の期限が近い（2026-11-20、詳細ファイル）
詳細: /tmp/runpr.Ab12Cd/deferred-1.md
```

-   1 行目は件数と PR 番号、続く行はスレッドごとに番号と要旨を 1 行ずつ示す。要旨は `fixpr` の Final report の `topic` を基に `/runpr` が自分の言葉で書いたもので、スレッドの本文をそのまま写さない（要件書 4.2）。
-   固定の行は、件数の行、自由記述の行、詳細ファイルの行、LIMITS_NOTE の行（あるときだけ）の最大 4 行である。スレッドの行がこれと合わせて 12 行を超えるとき（スレッドが 9 件以上のとき）は、スレッドの行を出さずに「一覧は詳細ファイル」の 1 行に置き換え、一覧は詳細ファイルにだけ書く。
-   LIMITS_NOTE の行は、step 1 が「step 4 の問いのメッセージに含める」としている（`runpr.md:232-234`）ので、deferred の問いでも出す。会話には状態と期限だけを示し、全文は詳細ファイルに書く（AC-08 と同じ形）。
-   各行は表示幅 80 桁以内とする。要旨が収まらないときは、要旨を短く書き直す。

**詳細ファイル（AC-01）。** `deferred-<回>.md` に、次を書く。

-   PR 番号、head のコミット、問いの回、問いの種類。
-   各スレッドの番号、要旨、直すと何が変わるか、ファイルのパス、URL。
-   自由記述の答えの書き方（番号の区切りと、指定しなかったスレッドを追跡 issue に移すこと）。
-   3 文書の PR では、追跡 issue に移したスレッドは、後で通常の進め方へ戻しても申し送りのファイルの項目にならないこと。戻すなら、この問いで「通常の進め方へ戻す」を選ぶこと。
-   LIMITS_NOTE の全文（あるとき）。

**自由記述の答え（AC-01）。** 受け付けるのは、`1` からスレッドの件数までの整数を、`,` か空白で区切って並べたものだけである。重複してはならず、ほかの文を含めてはならない。それ以外の答え（番号を読めない、範囲の外の番号を含む、重複がある、選択肢の答えや別の文と番号を混ぜている）では、何もせずに、新しい問いの回として同じ問いを尋ね直す。すべての番号を指定した答えは、「すべて直す」と同じに扱う。

**答えごとの動き。**

| 答え | 動き |
|---|---|
| すべて追跡 issue に移す | すべての deferred のスレッドを、3.6 の追跡 issue の手順で移す。止まらなければ項目 2 へ進む（AC-02） |
| 番号を指定 | 指定しなかったスレッドを 3.6 の手順で移す。そこで止まったら、直さずに止まる。止まらなければ、指定したスレッドの ID を caller override として `fixpr.md` に渡し（`fixpr.md:167-170`）、step 2 に戻る（AC-02、AC-03） |
| すべて直す | すべての deferred のスレッドの ID を caller override として `fixpr.md` に渡し、step 2 に戻る（AC-03） |
| 通常の進め方へ戻す（3 文書の PR） | 3.7 の読み替えで、戻す手順を行う（AC-05a） |
| マージせずに止める | 何も投稿せずに止まる。deferred のスレッドは未解決のまま残る（AC-04） |

step 2 に戻った後は、今の「deferred のスレッドを直す答え」（`runpr.md:823-828`）と同じく、step 3 を経て step 4 の項目 1 から始める。未解決の deferred のスレッドが残っていれば deferred の問いを尋ね、なければ項目 2 へ進む（AC-03）。どの答えでも、項目 1 ではマージしない。

### 3.2. 詳細ディレクトリと詳細ファイル（F-002、H-01）

**場所と名前。** 1 回の実行で、最初の問いを尋ねる直前に `mktemp -d /tmp/runpr.XXXXXX` で詳細ディレクトリを 1 つ作る。以後、その実行のすべての問いの詳細ファイルを、そこに書く。ファイルの名前は `<種類>-<回>.md` で、種類は `deferred` か `merge`、回は問いの回である。回は deferred の問いとマージの質問で共通に数えるので、同じ名前のファイルは 1 回の実行の中に 2 つできない。尋ね直しも、新しい回である。

**書く時点と中身。**

-   deferred の問いは、問いを尋ねる直前に書く。中身は 3.1 のとおりである。
-   マージの質問は、承認のメッセージのファイルを書き、件名を確かめた後（`mergepr.md` step 2 の確認を通った後）、問いを尋ねる直前に書く。中身は、AC-06 の項目のすべてである。
    -   PR の URL、ベースブランチ、CI の結果。
    -   squash commit の件名と本文の全文。承認のメッセージのファイルの中身をそのまま写す。
    -   接頭辞を決めた規則の記録（`mergepr.md` step 2）。
    -   承認の行のない版の本文（承認なしでマージする選択肢を示すとき）。
    -   PRE_MERGE の変更。どのファイルのどの行か、承認の行が書くレビュアーの名前と日付、PR 番号の記入の行を分けて書く。分けるのは、承認なしでマージする答えが記入の行だけを commit するためである（今の `runpr.md:732-735` と同じ）。
    -   diff 記録の再記録。AC の文、記録した出力と新しい出力の違い、新しい出力で AC が成り立つという判断を書く。
    -   PR の説明に追記できなかった編集上の訂正と、`#未記入` の行。
    -   3 文書の PR の除外スレッド。番号、除外の印、要旨、理由、URL を書く。
    -   Escalation の判定とその理由（未レビューの変更があるとき）。
    -   NEXT。NEXT が `/runplan` のときは、実装モデル要件の値と推奨するモデル、および読み方の詳細も書く。旧値ならその値と読み替えた値、読めない値なら値の文、値がないならその理由である（今の `runpr.md:748-761` の内容）。
    -   LIMITS_NOTE の全文（あるとき）。
-   どちらの詳細ファイルも、先頭に、PR 番号、head のコミット、問いの回、問いの種類と、「このファイルを編集しても、マージのメッセージもほかの動きも変わらない」旨を書く（H-01）。

**読み戻さない。** `/runpr` は詳細ファイルを読まない。マージは承認のメッセージのファイルで行う（今の項目 3 の「approved subject and body files」、`runpr.md:964-966`、`:986-987`）。

**書けないとき。** 詳細ディレクトリを作れないか詳細ファイルを書けないときは、問いを尋ねずに止まり、何に失敗したかを報告する。マージの質問の前なら、止まる前に `mergepr discard --state <state-file>` を行う。詳細ファイルのない問いは尋ねない。

**片付け。** 詳細ディレクトリは、`/runpr` が消さない。マージの後に利用者が読み返せるようにするためである。中身については 5 章で扱う。

### 3.3. マージの質問の要約（AC-07〜AC-09、H-04）

会話には、問いの前に次の行だけを、この順で出す。括弧の中は、その行を出す条件である。

| 順 | 行の形（例） | 出す条件 |
|---|---|---|
| 1 | `PR #859 → main ／ CI: 成功` | 常に |
| 2 | `件名: docs(0089): draft the architecture ... (#859)` | 常に |
| 3 | `PRE_MERGE: 承認 1 件・PR 番号の記入 2 行（2 ファイル）` または `PRE_MERGE: なし` | 常に |
| 4 | `未レビューの変更: あり（Escalation: ask）` または `未レビューの変更: なし` | 常に |
| 5 | `除外スレッド: 2 件（番号で判定を覆せる）` | 3 文書の PR で、未解決の除外スレッドがあるとき |
| 6 | `NEXT: /runplan 0083（要件 sonnet → Sonnet）` など（下の表） | 常に |
| 7 | `要確認: diff 記録の再記録 1 件（詳細ファイル）` | 再記録があるとき |
| 8 | `要確認: PR の説明に追記できなかった訂正 2 件（詳細ファイル）` | 追記できなかった訂正があるとき |
| 9 | `要確認: #未記入 の行 1 件（詳細ファイル）` | `#未記入` の行があるとき |
| 10 | `要確認: 操作制限の期限が近い（2026-11-20、詳細ファイル）` | LIMITS_NOTE があるとき |
| 11 | `自由記述: メッセージの修正 ／ 除外スレッドの判定を番号で覆す` | 常に（後半は、5 行目を出すときだけ） |
| 12 | `詳細: /tmp/runpr.Ab12Cd/merge-3.md` | 常に |

-   **行数（AC-07）。** 行の種類は 12 で、各種類は多くて 1 行である。どの PR でも 12 行を超えない。AC-22 の 2 つの状態は、(1) 5 行目のない 11 行、(2) 12 行のすべてである。
-   **桁（AC-07）。** 2 行目のほかは、固定の文と短い値（PR 番号、ベースブランチ、件数、日付、タスク ID、実装モデル要件の値、モデル名、詳細ファイルのパス）だけからなる。全角の文字を 2 桁と数えて 80 桁以内に収まるよう、文言を決める。2 行目は、承認する件名を省かずに示すため、桁の上限を設けない。
-   **3 行目（AC-07）。** 変えるファイルの数と行の数、承認か PR 番号の記入かを示す。どのファイルのどの行かは、詳細ファイルに書く。
-   **4 行目（AC-07）。** 未レビューの変更があるのは、LAST_REVIEWED が head より前か、ないときである（今の `runpr.md:776-777` と同じ）。そのとき、Escalation の判定（`ask`・`skip` など）を同じ行に示す。理由は詳細ファイルに書く。
-   **7〜10 行目（AC-08）。** 該当するものがある種類だけを出す。7〜9 行目は種類と件数を示し、10 行目は件数ではなく状態と期限を示す。
-   **6 行目（AC-09）。** 次の形にする。

    | NEXT | 6 行目の形 |
    |---|---|
    | ない | `NEXT: なし` |
    | `/runplan`、値が 3 つの値のどれか | `NEXT: /runplan 0083（要件 sonnet → Sonnet）` |
    | `/runplan`、旧値 | `NEXT: /runplan 0083（要件 standard＝sonnet → Sonnet）` |
    | `/runplan`、読めない値 | `NEXT: /runplan 0083（要件の値を読めない、推奨モデルなし）` |
    | `/runplan`、値がない | `NEXT: /runplan 0083（要件の値がない、推奨モデルなし）` |
    | `/mkarch`・`/mkplan` | `NEXT: /mkarch 0089` |

    読めない値の文と、値がない理由は、詳細ファイルに書く。実装モデル要件の値と推奨するモデルの読み方は、今と同じく `_context.md`「PR marker conventions」に従う。step 5 は、今と同じく、マージを決めた回の問いで示した値とモデルを使う（`runpr.md:763-766`）。

squash commit の本文、接頭辞を決めた規則の記録、承認の行のない版は、会話に出さない（AC-07）。

### 3.4. マージの質問の問いと選択肢（F-003、AC-10、AC-13〜AC-15、AC-17）

**問い。** 1 つの問いだけを持つ単一選択の `AskUserQuestion` である（AC-13）。question の文は「PR #<番号> をどうしますか（要約と詳細ファイルを確認のうえ）」の 1 行で、要約の行も詳細ファイルの中身も含まない（AC-10、1.2）。

**選択肢（除外スレッドが残っていないとき。AC-14）。** 次の表の条件に合うものを、表の順に示す。

| label | description | 示す条件 | 推奨 |
|---|---|---|---|
| 先に PR レビューを依頼 | Codex に PR レビューを頼み、終わったらこの問いに戻る | 未レビューの変更があり、Escalation の判定が `ask` のとき | 推奨 |
| PRE_MERGE を記録してマージ（PRE_MERGE がないときは「マージ」） | 要約の件名と詳細ファイルの本文でマージする | 常に | `ask` でないとき推奨 |
| 先に PR レビューを依頼 | （同上） | 未レビューの変更があり、Escalation の判定が `ask` でないとき | しない |
| 承認なしでマージ | 文書は draft のまま、PR 番号だけを記録してマージする | 今と同じ条件（`runpr.md:788-799`） | しない |
| 通常の進め方へ戻す | マージせず、要件定義書だけの PR に作り直す | 3 文書の PR | しない |
| マージしない | 何もせずに止まる | 常に。最後に置く | しない |

「先に PR レビューを依頼」は、1 行目か 3 行目のどちらか一方だけに示す。「承認なしでマージ」は 3 文書の PR に示さないので、「通常の進め方へ戻す」と同時には示されない。そのため、示す数は多くて 4 つである。推奨は、label の後に推奨の印を付けて示す（`AskUserQuestion` の決まり）。

**選択肢（3 文書の PR で除外スレッドが残っているとき。AC-15）。** 「先に PR レビューを依頼」（未レビューの変更があるときだけ。`ask` なら推奨）、「通常の進め方へ戻す」、「マージしない」だけを、この順に示す。2 つか 3 つである。

**自由記述（AC-17）。** 要約の 11 行目が、自由記述でできる操作を示す。

-   **メッセージを直す:** 今と同じく、件名と本文のファイルを書き直し、件名を `mergepr.md` step 2 と同じく確かめ直す（`runpr.md:808-811`）。そのうえで、項目 3 を新しい問いの回として尋ね直す。マージの準備は残っているので、項目 1・2 には戻らない（AC-12）。
-   **除外スレッドの判定を覆す（除外スレッドがあるときだけ）:** 番号で指定する。今と同じく、覆すスレッドとして `<threadId>=<fingerprint>` の形で `fixpr.md` に渡し（`runpr.md:829-833`）、`mergepr discard` をして step 2 に戻る。マージしない（AC-15）。番号の受け付け方は、3.1 の自由記述と同じ規則にする。読めない答えでは、新しい回として項目 3 を尋ね直す。

「NEXT をこのセッションで実行」の選択肢はない（AC-16）。

### 3.5. 答えの後の手順（AC-11、AC-12、AC-18）

項目 3 の答えを受けた手順は、今の `runpr.md:808-991` から、次の 4 点だけを変える（AC-18）。

1.  **deferred のスレッドの手順の移動。** 「deferred threads to fix now」の答え（`runpr.md:823-828`）と、マージの答えの前に追跡 issue に移す手順（`runpr.md:929-961`）を、項目 1 に移す（3.1、3.6）。除外スレッドを覆す答え（`runpr.md:829-833`）は、自由記述の答えとして項目 3 に残す。
2.  **NEXT を実行する答えの削除と step 5（AC-16）。** 「record PRE_MERGE, merge, and run NEXT in this session」（`runpr.md:786-787`）と、それを受ける step 5 の分岐（`runpr.md:1039-1041`）を除く（3.8）。
3.  **尋ね直しのたびの書き直し（AC-11）と、マージのメッセージ（AC-12）。** 項目 3 を尋ねるたびに、そのときの head と承認のメッセージのファイルから、新しい回の詳細ファイルと要約を書く。尋ね直す場面は、今と同じである。PR レビューの後、除外スレッドを直した後、メッセージを直した後、PRE_MERGE の push が拒まれた後、PRE_MERGE の後に新しいスレッドが現れた後、マージの時に head が動いていた後（`runpr.md:808-833`、`:968-991`）である。そのうちメッセージを直した場合のほかは、step 2 か step 3 を経て項目 1 から始め直すので、新しい deferred のスレッドがあれば、先に deferred の問いを尋ねる。マージする答えは、今と同じく、その回の承認のメッセージのファイル（承認なしでマージする答えでは承認の行のない版）でマージする。
4.  **deferred の問いから戻す入口（AC-05a）。** 戻す手順に、項目 1 から入るときの読み替えを足す（3.7）。

PR レビューを依頼する答え（`runpr.md:815-821`）、承認なしでマージする答え（`runpr.md:788-799`）、通常の進め方へ戻す答え（`runpr.md:835-927`）、マージしない答え（`runpr.md:812-813`）、PRE_MERGE の記録とマージ（`runpr.md:962-991`）は、上の点のほかは変えない。

マージの答えは、追跡 issue に移す手順を通らなくなる。そのため、マージの答えの後からマージまでの間に、GitHub へ投稿する文はなくなる。

### 3.6. 追跡 issue に移す手順の移動（AC-02、H-03）

今の `runpr.md:929-961` の手順を、中身を変えずに項目 1 へ移す。追跡 issue を探し（`gh issue list --state open --search "PR #<number> のレビュー指摘の追跡 in:title"`）、すでに書いたスレッドを除き、投稿する文と各スレッドへの返信にメンションの確認をし、`gh issue comment` か `gh issue create` で書き、`fixprguard reply` で返信して resolve する。

変えるのは、止まるときの動きだけである。今は止まる前に `mergepr discard --state <state-file>` をする（`runpr.md:939-940`、`:955-957`）。移した後はマージの準備がまだないので、準備を破棄せずに止まる。報告の中身（止まったスレッドの URL と止まった種類、`reply-not-resolved` のときは手で resolve するよう頼むこと、追跡 issue にはもう書いてあること）は変えない。

追跡 issue に移したスレッドは、後のマージの質問でどの答えを選んでも、移して resolve した状態のまま残る（AC-02）。

**文書のガードのテストへの影響（H-03）。** 手順は step 4 の節の中に残るので、`postingSections` の「runpr.md step 4」の節と命令の一覧（`codex_review_docs_test.go:65-68`）は、そのまま当てはまる。`TestPostingCommandsInventory` は、各節で最初の `**Mention check.**` が、その節の投稿の命令より前にあることを確かめる（`codex_review_docs_test.go:200-259`）。移した後は、step 4 の最初の投稿の命令が項目 1 の `gh issue comment`・`gh issue create` になる。その前に、項目 1 のメンションの確認の段落を置く。今の step 4 でも、最初のメンションの確認は戻す手順 5 にあり、それより後に最初の投稿の命令（戻す手順 5 の `fixprguard edit-pr`）がある。この関係を保つ。`TestFixprguardCallSites` の step 4 の `fixprguard reply ` も、そのまま当てはまる（`fixpr_docs_test.go:729`）。

### 3.7. 戻す手順の入口（AC-05a、H-02）

戻す手順（今の `runpr.md:835-927`）を、項目 3 の答えからだけでなく、項目 1 の答えからも行う。項目 1 から入るときは、次の 2 点を読み替える。ほかの手順 2〜7 は変えない。

-   **戻す手順 1 の準備の破棄（`runpr.md:840`）:** 行わない。マージの準備がまだないからである。
-   **ほかの答えと混ぜた答え（`runpr.md:842-843`）:** 今は「step 4 を項目 1 から尋ね直す」。項目 1 から入ったときは、deferred の問いを新しい回として尋ね直す。項目 3 から入ったときは、今と同じく step 4 を項目 1 から尋ね直す（繰り下げた後も、項目 1 から始めれば、deferred の問いと準備を経て項目 3 に至る）。

THREADS は、今と同じく、最新の `fixpr` Final report の deferred と除外スレッドのうち、パスが `02_architecture.md` か `03_implementation_plan.md` のものである（`simple-process.md:380-384`）。項目 1 から戻すときは、deferred のスレッドを追跡 issue に移していないので、それらはすべて申し送りのファイルの項目になる。`simple-process.md` の「Returning to the full process」の冒頭（`:364-367`）の「`/runpr` runs this from step 4, return step 2」は、項目 1 からも入ることを書き足す（AC-21）。

戻す手順 7（`runpr.md:924-927`）の後は、step 2、step 3 を経て step 4 の項目 1 から始め直す。戻した後の PR は 3 文書の PR ではないので、`01_requirements.md` に残る deferred のスレッドがあれば、deferred の問いを、「通常の進め方へ戻す」のない形で尋ねる。

### 3.8. step 5 と、`/runpr` を説明する文（AC-16、AC-20）

-   **step 5 の見出し:** `## 5. Switch branch, then show NEXT` にする。NEXT を実行すると述べない。
-   **step 5 の終わり方（`runpr.md:1035-1048`）:** 「answer chose to run NEXT」の分岐を除く。NEXT があり、次のブランチの用意が終わったときは、`/clear` の案内（`runpr.md:1052-1061`）を付けて最終の状態を報告して終わる。用意の前に止まったときの報告（`runpr.md:1063-1103`）の「whichever Merge option the answer chose, and do not run NEXT」は、選んだ選択肢による区別がなくなるので、「do not run NEXT」を除く。
-   **AC-20 の記述:** 次の記述を、「次のコマンドを示し、実行しない」と述べるように書き直す。
    -   `runpr.md:2`（description）と `runpr.md:48-50`。
    -   `runpr.md:90-91`（Authorization の「the next command (NEXT) when the user chooses to run it in step 4」）。この句を除く。
    -   `CLAUDE.md:207` と `CLAUDE.md:226-230`。
    -   `_lib/task-pr-handoff.md:27-32`。
    -   `requirements_process.md:89`。
-   **Authorization の追跡 issue（`runpr.md:82-83`、AC-21）:** 「the deferred findings the user moves to an issue in step 4」を、「in step 4's deferred question」に書き直す。

### 3.9. `mergepr.md` と、deferred のスレッドを説明する文（AC-19、AC-21）

-   **`mergepr.md` step 3（AC-19）:** 「When this command runs inside `runpr.md`」で始まる文（`mergepr.md:122-126`）だけを書き直す。新しい文は次の 2 点を述べる。`runpr.md` step 4 は、件名を会話に、本文と接頭辞を決めた規則の記録を作業ツリーの外の詳細ファイルに示してマージの質問を尋ねること。マージの質問は次のコマンドを実行するかを尋ねないこと。PRE_MERGE とあわせて尋ねること、記録の commit の後に準備をやり直す方法を `runpr.md` が述べることは、今の文のまま残す。単独の `/mergepr` の振る舞いを述べる文（`mergepr.md:118-121`）は変えない。
-   **`fixpr.md`（AC-21）:** `:144-146` の「`runpr.md` step 4 asks whether to fix each one now or move it to an issue」を、step 4 の deferred の問いで尋ねると書き直す。`:167-168` の caller override の呼び出し元「`runpr.md` step 4, the ones the user chose」を、step 4 の deferred の問いで利用者が選んだもの、と書き直す。除外スレッドを述べる `:157`、`:184`、`:494`、`:1192` は、除外スレッドを step 4 のマージの質問で扱う点が変わらないので、変えない。
-   **`simple-process.md`（AC-21）:** `:334-339` は、除外スレッドを step 4 のマージの質問で示し、判定を覆すスレッドを番号で選ぶことを述べる形にする。「fix all」は deferred の問いの選択肢で、除外スレッドを含まないことを書く（AC-05）。マージの質問で示す選択肢が「通常の進め方へ戻す」と「マージしない」に限られることは、「先に PR レビューを依頼」が加わりうることとあわせて書く（AC-15）。`:380-384` の THREADS は、戻す答えを deferred の問いでも選べることを書き足す（3.7）。

### 3.10. ADR

このタスクの決定は、要件定義の PR（#859）で [ADR-0047](../../dev/adr/0047-runpr-merge-question.md) を作り、[ADR-0040](../../dev/adr/0040-simple-task-documents.md) と [ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新して記録した。本書の 1.2 の決定は、どれも主要な決定に当たらない（各項の ADR の判断）。本書の設計は、ADR-0047 の「決定」の各項と合う。そのため、この PR では ADR を変えない。実装の PR は、ADR-0047 の「この決定は、まだ…反映していない」の文を除く（2.2）。

## 4. エラーハンドリング設計 (Error Handling Design)

### 4.1. 実行時のエラー

コマンドの定義だけを変え、Go の型もエラーの型も足さない。N/A。

### 4.2. 手順の中で止まる場合

| 場面 | 止まり方 | 報告 |
|---|---|---|
| 詳細ディレクトリを作れない、詳細ファイルを書けない | 問いを尋ねずに止まる。マージの質問の前なら、先に `mergepr discard` をする | 失敗した操作とパス |
| deferred の問いの前のメンションの一覧に、ほかのアカウントのメンションがある | 今の step 3 の「Mentions by other accounts」のとおり止まるか尋ねる | 今と同じ |
| 追跡 issue の文か返信が、メンションの確認に通らない | 投稿せずに止まる。準備の破棄はない | 今と同じ（3.6） |
| `fixprguard reply` が止まる | 止まる。準備の破棄はない | 止まったスレッドの URL と止まった種類。今と同じ |
| deferred の問いの自由記述が読めない | 止まらず、新しい回として尋ね直す | なし |
| マージの質問の自由記述が読めない | 止まらず、新しい回として項目 3 を尋ね直す | なし |

どの場合も、詳細ファイルのない問いは尋ねず、問いを尋ねた後に詳細ファイルを読み戻して判断を変えることもない。

## 5. セキュリティ考慮事項 (Security Considerations)

### 5.1. 脅威モデル

矢印 A → B は「A が B に届きうる」を表す。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef problem fill:#ffe6e6,stroke:#d62728,stroke-width:2px,color:#7b0000;

    TH[("レビューのスレッドの本文<br>（信頼できない入力）")]
    FX["fixpr の Final report<br>（topic を要約）"]
    RP["/runpr step 4"]
    DF[("詳細ファイル<br>/tmp/runpr.XXXXXX/")]
    LU["同じ機械のほかの利用者"]
    ED["利用者による<br>詳細ファイルの編集"]
    MSG[("承認のメッセージのファイル")]
    GH["GitHub<br>（追跡 issue・返信）"]

    TH --> FX --> RP
    RP --> DF
    RP --> MSG
    RP -->|"メンションの確認と<br>fixprguard reply を通す"| GH
    LU -.->|"0700 で拒まれる"| DF
    ED -.->|"読み戻さないので届かない"| MSG

    class TH,DF,MSG data
    class FX,GH process
    class RP enhanced
    class LU,ED problem

    subgraph Legend["Legend"]
        L1[("データ")]
        L2["変えないコンポーネント"]
        L3["変えるコンポーネント"]
        L4["脅威の元"]
    end
    class L1 data
    class L2 process
    class L3 enhanced
    class L4 problem
```

-   **詳細ファイルの中身。** squash commit の件名と本文、PRE_MERGE の行、スレッドの要旨と URL など、PR と GitHub にすでにあるか、マージで `main` に入る情報である。秘密（API キー、Webhook の URL）は含まない。どこにも送らない（要件書 4.2）。
-   **ほかの利用者。** 詳細ディレクトリは `mktemp -d` が作るので、名前を推測できず、モードは `0700` である。ほかの利用者は読めず、先にシンボリックリンクを置いて書き込み先を差し替えることもできない。
-   **古い中身や編集した中身での承認。** マージは承認のメッセージのファイルだけで行い、詳細ファイルは読み戻さない（1.1）。回ごとに別の名前にするので、要約が古い回のファイルを指すことはない（3.2）。
-   **信頼できないスレッドの本文。** 要旨は、今と同じく `/runpr` が自分の言葉で書いたデータで、指示として読まない。deferred の問いでも、スレッドの本文を取りに行かない（`fixprguard list` は本文を出さない。`runpr.md:12-19`）。
-   **GitHub への投稿。** 追跡 issue への投稿とスレッドへの返信は、投稿する時点が前に移るだけで、今と同じメンションの確認と `fixprguard reply` を通す（AC-02、3.6）。

### 5.2. 残余のリスク

-   詳細ディレクトリは `/runpr` が消さないので、`/tmp` に残る。中身は 5.1 のとおりで、秘密を含まないので受け入れる。
-   Claude Code では、権限のモードによって、詳細ファイルを書くたびに承認を求められうる（1.2）。承認を求められても、安全の側に倒れる。

## 6. 処理フロー詳細 (Processing Flow Details)

### 6.1. deferred の問いで番号を指定したとき

```mermaid
sequenceDiagram
    participant U as 利用者
    participant R as /runpr（step 4 項目 1）
    participant G as GitHub
    participant F as fixpr

    R->>R: メンションの一覧を読み直す
    R->>R: deferred-回.md を書く
    R->>U: 要約と deferred の問い
    U->>R: 自由記述「1,3」
    R->>R: 番号を確かめる（範囲・重複・ほかの文）
    R->>G: 2・4 を追跡 issue に書き写す（メンションの確認の後）
    R->>G: 2・4 に返信して resolve（fixprguard reply）
    R->>F: 1・3 を caller override として渡す
    F-->>R: Final report
    R->>R: step 2、step 3 を経て step 4 項目 1 へ
```

追跡 issue への投稿か返信で止まったときは、`fixpr` を呼ばずに止まる（AC-03）。

### 6.2. マージの質問の尋ね直し

| 尋ね直す理由 | 戻る先 | 詳細ファイル |
|---|---|---|
| メッセージを直す | 項目 3 | 新しい回の `merge-<回>.md` |
| 先に PR レビューを依頼 | step 3 を経て step 2、項目 1 から | 新しい回 |
| 除外スレッドの判定を覆す | step 2、項目 1 から | 新しい回 |
| PRE_MERGE の push が拒まれた | step 2、項目 1 から | 新しい回 |
| PRE_MERGE の後に新しいスレッドが現れた | step 3、step 2、項目 1 から | 新しい回 |
| マージの時に head が動いていた | step 2、項目 1 から | 新しい回 |

## 7. テスト戦略 (Test Strategy)

### 7.1. ユニットテスト

-   **直すテスト:** `cmd/yt2column/codex_review_docs_test.go:84` の定数 `runprStep5` を、新しい見出し `## 5. Switch branch, then show NEXT` に合わせる。この定数を使う `postingSections` の「runpr.md step 5」（`gh issue close`）は、ほかは変えない。
-   **そのまま当てはまるテスト:** `postingSections` の step 4 の節と命令の一覧、`TestPostingPathsCheckMention`、`TestPostingCommandsInventory`、`TestFixprguardCallSites`、`cmd/fixprguard/docs_test.go` の `runpr.md` の呼び出しの検査、`TestRunprDocContract`。3.6 のとおり、step 4 の中の順序の関係を保つので、これらは変えずに通る。実装の後に `make test` で確かめる。
-   **足さないテスト:** 要約の行の形、選択肢の文言、問いの数は、文の意味に当たる。そのため、テストにせず PR のレビューで読む（`CLAUDE.md`「Testing Strategy」の文書のテストの規則、[ADR-0016](../../dev/adr/0016-document-test-policy.md)、H-03）。

### 7.2. 読んで確かめる（manual）と diff の確かめ

-   AC-01〜AC-17、AC-20、AC-21 は、実装の PR のレビューで、`runpr.md` ほかの記述を読んで確かめる。
-   AC-18（step 4・5 の答えを受けた手順の diff の限定）と AC-19（`mergepr.md` の diff の限定）は、マージベースとの diff を読んで記録する（`requirements_process.md` §4「`diff` verification」）。
-   AC-22 は、1.2 のとおり、OpenCode の高さ 24 行の pane で、見本の値で書いた要約と選択肢を出して確かめる。次の 3 つを出す。(1) 要約 11 行と選択肢 4 つ（タスク文書の PR）。(2) 要約 12 行と選択肢 3 つ（3 文書の PR で除外スレッドあり）。(3) deferred のスレッドが 4 件の deferred の問い。それぞれで、すべての選択肢が画面の中に見えて選べることを確かめ、画面の写しか所見を PR に記録する。

### 7.3. 結合テスト・セキュリティテスト

-   結合テスト: N/A。コマンドの定義の文書だけを変える。実装の PR を `/runpr` で回すとき、その PR のマージの質問が新しい形で出ることを、あわせて見る。
-   セキュリティテスト: メンションの確認は、7.1 の既存のテストが step 4 の節で確かめ続ける。

## 8. 実装の優先順位 (Implementation Priorities)

変更は、コマンドの定義、その説明の文書、テストの定数 1 つで、互いに参照し合う。そのため、1 本の PR で行う。順序は次のとおりである。

1.  `runpr.md` step 4 の組み替え（項目 1 の deferred の問い、項目 3 の要約と詳細ファイルと問い、答えの後の手順、戻す入口）と、step 3 の 2 か所、step 1 の LIMITS_NOTE の扱い。
2.  `runpr.md` step 5 の見出しと終わり方、description、導入の段落、Authorization。テストの定数 `runprStep5`。
3.  `mergepr.md`、`fixpr.md`、`_lib/simple-process.md`、`_lib/task-pr-handoff.md`、`CLAUDE.md`、`requirements_process.md`。
4.  ADR-0047 の「まだ反映していない」の文を除く。
5.  `make test`・`make lint`、AC-18・AC-19 の diff の確かめ、AC-22 の手動の確認。

## 9. 将来の拡張 (Future Extensibility)

-   0088 の chief は、deferred の問いとマージの質問の選択肢を中継できる。どちらも、1 つの問いだけを持つ単一選択の問いだからである。番号の自由記述は中継しないので、人がその pane で答える（ADR-0046）。
-   AC-22 の確かめで画面に収まらなかったときは、選択肢の description を除いて行を減らせる。これは文言の変更で、本書の構成は変わらない。

## 付録 A: 申し送り（design_handoff.md）への対応

| 項目 | とった方針 | 本書の箇所 |
|---|---|---|
| H-01（詳細ファイルの場所・名前、古い中身で承認しない書き方） | `mktemp -d /tmp/runpr.XXXXXX` の詳細ディレクトリに、問いの回ごとに別の名前で書く。マージは承認のメッセージのファイルだけで行い、詳細ファイルは読み戻さない。編集しても変わらないことを詳細ファイルの先頭に書く。候補の「準備のディレクトリ」は、作業ツリーの中にあり、deferred の問いの時点ではまだないので採らない | 1.2、3.2、5.1 |
| H-02（deferred の問いの置き場所） | step 4 の新しい項目 1 に置き、今の項目を 2〜5 に繰り下げる。戻す手順の読み替えは、準備の破棄をしないことと、混ぜた答えで deferred の問いを尋ね直すことの 2 点 | 1.2、3.1、3.7 |
| H-03（文書のガードのテストの直し方） | 手順は step 4 の節に残るので、`postingSections` は変えない。変えるのは step 5 の見出しの定数だけ。文言と要約の形はテストにしない | 3.6、7.1 |
| H-04（要約の行と選択肢の文言、chief への配慮、画面の見積もり、AC-22 の確かめ方） | 要約の 12 種類の行と選択肢の文言を固定した。question の 1 行で要約と詳細ファイルの確認を促す。AC-22 は見本の値で表示を再現して確かめる。画面の見積もりは下に示す | 1.2、3.1、3.3、3.4、7.2 |

**画面の行の見積もり（H-04）。** 高さ 24 行の pane で、AC-22 の (1) は、要約 11 行、question の 1 行、選択肢 4 つ（label と description で各 2 行として 8 行）、自由記述の欄 1 行、OpenCode の枠と区切りで約 23〜24 行になる。(2) は、要約 12 行、question の 1 行、選択肢 3 つ（6 行）、自由記述の欄 1 行と枠で約 22〜23 行になる。どちらも上限に近い。OpenCode の質問の画面の正確な行数は、AC-22 の手動の確認で確かめる。収まらなければ、9 章のとおり description を除く。

## 付録 B: 受け入れ基準との対応

| AC | 本書の箇所 |
|---|---|
| AC-01 | 3.1（問いの形、メッセージ、詳細ファイル、自由記述） |
| AC-01a | 3.1（尋ねる時点） |
| AC-02 | 3.1（答えごとの動き）、3.6 |
| AC-03 | 3.1（答えごとの動き）、6.1 |
| AC-04 | 3.1（答えごとの動き） |
| AC-05 | 3.1（対象）、3.9（`simple-process.md`） |
| AC-05a | 3.7 |
| AC-06 | 3.2 |
| AC-07 | 3.3 |
| AC-08 | 3.3（7〜10 行目） |
| AC-09 | 3.3（6 行目） |
| AC-10 | 3.4（問い） |
| AC-11 | 3.5（3 点目）、6.2 |
| AC-12 | 1.1、3.2（読み戻さない）、3.4（メッセージを直す） |
| AC-13 | 3.4（問い） |
| AC-14 | 3.4（除外スレッドが残っていないとき） |
| AC-15 | 3.4（除外スレッドが残っているとき） |
| AC-16 | 3.4、3.5（2 点目）、3.8 |
| AC-17 | 3.3（11 行目）、3.4（自由記述） |
| AC-18 | 3.5、7.2 |
| AC-19 | 3.9、7.2 |
| AC-20 | 3.8 |
| AC-21 | 3.1、3.7、3.8、3.9 |
| AC-22 | 1.2、7.2、付録 A（画面の行の見積もり） |
