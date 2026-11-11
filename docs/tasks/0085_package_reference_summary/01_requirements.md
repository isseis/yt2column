# 要件定義書：package_reference.md の行を責務の要約に絞る

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Process | `simple` |
| Created | 2026-11-11 |
| Review date | 2026-11-11 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`docs/dev/conventions/package_reference.md`（以下、パッケージの参照表）の各行を、パッケージの責務の要約に絞る要件を定義する。あわせて、行に書く粒度の規則と、行から外した詳細の行き先も定める。

**背景:**

-   パッケージの参照表は、コミット `04dc4824` で 75,604 バイトある。表は 1 つで 56 行あり、1 行が 1 パッケージに当たる。大きい行は、`internal/coveragecompare` が約 6.4KB、`internal/config` が約 5.3KB、`internal/server` が約 4.7KB である。
-   行の多くは、パッケージの責務ではなく実装の詳細を書いている。たとえば、非公開の関数の挙動、定数と上限値、終了コード、子プロセスに渡す環境変数の一覧、ファイル名である。これらはコードを読めば分かり、タスクの設計書、`docs/dev/security.md`、`docs/dev/guides/cli.md`・`development.md`、README の設定ブロックとも重なる。
-   冒頭の運用の規則は「パッケージを足す・消す・責務を変えるコミットで更新する」だけである（`docs/dev/conventions/package_reference.md:3-6`）。行に何をどこまで書くかの上限がないので、タスクごとに詳細が書き足されてきた。
-   この文書を読むテストは 2 つだけである。
    -   `TestPackageReferenceListsPackages`（`internal/pipeline/pipeline_test.go:629`）は、表の行の最初のコードスパン（パッケージ名）だけを読む。
    -   `TestPackageReferenceStrictjsonUsers`（`internal/pipeline/strictjson_users_test.go:609`）は、`internal/strictjson` の行の `Shared by ` に続く一覧だけを読む（`internal/pipeline/strictjson_users_test.go:168`）。

    行のほかの文は、PR のレビューで読む以外に古さを見つける手段がない。`docs/dev/guides/pricing_update.md:39` も、行が古くなっていないかを PR のレビューで読んで確かめるとしている。
-   ADR の 0001・0005・0019・0020・0021・0023・0027・0031 は、規則の置き場所として、パッケージの参照表の表や特定の行を参照している（コミット `04dc4824` で `git grep -n package_reference -- docs/dev/adr/` が返す 19 行）。

対応 issue: [#817](https://github.com/isseis/yt2column/issues/817)

**本書の記述範囲:** 本書は、冒頭の運用の規則、表の構成、行の中身、行から外した詳細の行き先、ADR の参照の扱い、関係する文書の直しを定める。表の分け方の細部（どのパッケージをどの表に載せるか）と、確かめ方は `02_architecture.md` で決める。

**issue #817 で決めたこと:**

-   **行の中身を、責務・主な公開の名前・詳しい仕様の置き場所に絞る。非公開の名前・数値・終了コード・手順は書かない。** 本書は、issue の「修正の案」の表に挙がった 4 つの対象（行の中身、冒頭の運用の規則、表の構成、詳細の移し先）について、どれを採るかを決める。
    -   ADR：要件定義で判断する（下の「要件定義の際に決めたこと」）。
-   **`internal/strictjson` の行は `Shared by A, B, and C` の書式を保つ。** `TestPackageReferenceStrictjsonUsers` がこの書式で読むためである。
    -   ADR：なし。[ADR-0020](../../dev/adr/0020-strict-json.md) の今の決定（利用者の一覧はこの行が持つ）のままである。
-   **ADR が参照している契約の文は、行に短く残すか、参照先を張り替える。** 張り替えるときは、同じ PR で ADR を直す。
    -   ADR：要件定義で判断する（下の「要件定義の際に決めたこと」）。

**要件定義の際に決めたこと（2026-11-11）:**

-   **進め方は簡易な進め方（`simple`）とする。この変更は、除外される変更に当たらない。** 変わるのは、1 つの文書と、ADR などの文書の参照の文と、Go のコメントである。新しい仕組みは作らず、どの ADR の決定も変えない。未決の判断も少ない。除外の判断：変更は記述を減らして移すだけで、次のどれも変えない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「Excluded changes」）。
    -   `docs/dev/security.md` の規則と前提
    -   開発の道具（`fixprguard`・`codexreview`・`mergepr`）とその信頼境界
    -   外部依存
    -   ADR：なし。本タスクの中で閉じる、進め方の選択である。
-   **行に書くのは、責務（1〜3 文）、主な公開の型・関数・番兵の名前、詳しい仕様の置き場所へのリンクとする（issue の案）。非公開の名前、数値と上限値、終了コード、子プロセスに渡す環境変数の一覧、ファイル名、手順は書かない（F-001・F-003）。** 理由は、これらがコードと二重になり、テストでは古さを見つけられないためである。全体の目安は今の約 1/5 である。
    -   採らなかった案：責務だけを書き、公開の名前も挙げない案。ADR が参照する番兵の名前などが行からなくなり、ADR の参照を張り替える箇所が増える。
    -   ADR：[ADR-0044](../../dev/adr/0044-package-reference-row-granularity.md) を作る。行の粒度は、パッケージを足すか変える後のタスクすべてを縛る、開発の進め方の方針だからである。
-   **ADR が参照する内容のうち、新しい粒度に収まるもの（番兵の名前、責務）は行に残し、ADR はそのままにする。収まらないもの（成果物の形式、ホストが受け付けるメッセージなど）は、ADR がすでに挙げている別の正本か、コード（doc コメント）だけを指すように、ADR の参照の文を直す（F-005）。** ADR の決定は変わらず、古くなった参照を直すだけなので、各直しは編集上の訂正（`docs/dev/conventions/adr.md`「Editorial corrections」の 3 つ目の種類）として扱う。
    -   採らなかった案：ADR が参照する契約の文を、粒度の規則の例外としてすべて行に残す案。規則に例外が増え、行が再び膨らむ。
    -   採らなかった案：契約の文を godoc などへ移し、ADR の参照の張り替えを決定の変更として扱う案。決定の内容が変わらないのに、ADR ごとに「経緯」の項目が増える。
    -   ADR：なし。既存の ADR の決定を変えず、参照を直すだけである。
-   **行から外す詳細の扱いを次のとおりにする（F-004）。過去のタスク文書には書き足さない。** タスク文書はその時点の記録で、完了の後に変わらないためである（[ADR-0016](../../dev/adr/0016-document-test-policy.md) の採らなかった案の項目「タスクの文書の値をコードと照合する確認を残す」）。
    -   今の文書（`docs/dev/` の下の仕様の文書と guide、README、`prompts/README.md`）にすでにあるもの：行から削り、行からその文書へリンクする。
    -   タスクの設計書にあるもの：その節が今の挙動を述べているときに限り、今の文書と同じに扱う。設計書は完了の後に変わらないので、後のタスクが挙動を変えた節を指すと、読み手を古い仕様へ導いてしまうためである。今の挙動を述べていない節にしかないものは、どの文書にもないものとして扱う。
    -   どの文書にもなく、呼び出し側が頼る契約：パッケージか公開の識別子の doc コメントへ移す。呼び出し側が頼る契約とは、公開の識別子の振る舞いのうち呼び出し側から観測できるもの（受理・拒否する入力、どの失敗をどの番兵で報告するか、順序や原子性の保証、秘密の扱い）をいう。
    -   今の文書にもあり、契約でもあるもの：リンクだけにし、doc コメントに写さない。同じ詳細を 2 か所に持たないためである。
    -   それ以外：コードが正本なので削る。
    -   採らなかった案（2026-11-11 のレビューの後に改めた）：行き先を `cli.md`・`development.md`・`security.md`・README・`production_profile.md`・タスクの設計書に限る案。`fixprguard_guide.md` などの guide にすでにある詳細を doc コメントに写すことになり、古いタスクの設計書へのリンクも許してしまう。
    -   採らなかった案：すべてを移さずに削る案。呼び出し側が頼る契約が、どこにも書かれなくなる。
    -   採らなかった案：issue の表のとおり、すべてを移す案。コードを読めば分かる実装の細部まで文書に移すことになり、作業量が最も大きい。
    -   ADR：[ADR-0044](../../dev/adr/0044-package-reference-row-granularity.md)。詳細の置き場所は、粒度の規則の一部である。
-   **表は、パッケージの種類ごとに 6 つに分ける。種類は、パイプラインの構成要素、サーバとホスト、LLM、共有部品、開発ツール、テスト専用である（F-002）。** 1 つの表に 56 行が並び、並び順にも規則がないので、目的のパッケージを探しにくいためである。
    -   採らなかった案：1 つの表のまま、行の中身だけを絞る案。探しにくさが残る。
    -   ADR：[ADR-0044](../../dev/adr/0044-package-reference-row-granularity.md)。
-   **行の粒度の上限は、冒頭に文章の規則として書き、PR のレビューで読んで確かめる。行ごとのバイト数の上限やテストは設けない。本タスクでは、文書全体を 15,000 バイト以下にする（F-001・AC-09）。** テストで止めると、正当に長い行があるたびに規則と衝突するためである。維持の費用が、防ぐ誤りを上回る文書の確認は足さない（`CLAUDE.md`「Testing Strategy」）。15,000 バイトは、issue の「今の 1/5 程度」の目安を数にしたもので、本タスクの完了の判断にだけ使う。
    -   採らなかった案：行ごとのバイト数の上限を冒頭に書き、Go の guard テストで超えた行を失敗にする案。再び膨らむことは機械的に防げるが、上の理由で採らない。
    -   採らなかった案：数値の目安を設けない案。本タスクの完了を確かめる基準がなくなる。
    -   ADR：[ADR-0044](../../dev/adr/0044-package-reference-row-granularity.md)（上限を数で決めずレビューで読むこと）。15,000 バイトという値は本タスクの中で閉じるので、ADR には書かない。
-   **`CLAUDE.md` の Quick Links の説明文（`CLAUDE.md:18` の「Detailed package structure」）を、文書の新しい役割に合わせて直す（F-007）。** 文書が詳細を持たなくなり、今の説明文は事実でなくなるためである。
    -   ADR：なし。リンクの説明文の直しで、本タスクの中で閉じる。
-   **`docs/dev/guides/pricing_update.md:39` の「`internal/pricing` の行は、パッケージが公開する型を列挙している」という文を、新しい行の中身に合わせて直す（F-007）。** 行には主な公開の名前だけを書くことになり、今の文は事実でなくなるためである。
    -   ADR：なし。ガイドの 1 文の直しで、本タスクの中で閉じる。
-   **`TestPackageReferenceStrictjsonUsers` を、`Package` の列を持つすべての表から `internal/strictjson` の行を探すように直す（F-009）。** 今のテストは、`Package` の列を持つ最初の表だけを読む（`internal/pipeline/strictjson_users_test.go:198` が呼ぶ `internal/doctestutil/tables.go:63` の `FindTable`）。表を 6 つに分けると、共有部品の表にある行が見つからなくなるためである。
    -   採らなかった案：テストを変えず、共有部品の表を先頭に置く案。表の並びとテストの間に、どこにも書かれない結びつきが生まれる。
    -   ADR：なし。テストの探し方の直しで、本タスクの中で閉じる。
-   **ADR-0044 のずれの記述がリンクする issue #817 の本文の末尾に、ADR-0044 の名前を書き足す（AC-16）。** ずれの記述を足す PR には、リンク先の issue の本文にその ADR を挙げさせる規則があるためである（`docs/dev/conventions/adr.md`「Editorial corrections」の「Discrepancy statements」）。
    -   ADR：なし。規約に従う手順である。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   パッケージの参照表を、パッケージの責務を短く引ける文書にする。
-   行に書く粒度の上限を冒頭の規則で定め、後のタスクが詳細を書き足して再び膨らむのを防ぐ。
-   行から外した詳細のうち、呼び出し側が頼る契約を失わない。

### 2.2. スコープ (In Scope)

-   パッケージの参照表の冒頭の運用の規則の書き換え（F-001）。
-   表を種類ごとの 6 つに分けること（F-002）。
-   全 56 行の中身を絞ること（F-003）。
-   行から外した詳細の扱い：リンク、doc コメントへの移動、削除（F-004）。
-   パッケージの参照表の行を参照する ADR の、参照の文の直し（F-005）。
-   [ADR-0044](../../dev/adr/0044-package-reference-row-granularity.md) の作成と、ADR の索引への追加（F-006）。
-   `CLAUDE.md:18` の説明文と `docs/dev/guides/pricing_update.md:39` の 1 文の直し（F-007）。
-   `TestPackageReferenceStrictjsonUsers` が、すべての表から行を探すようにする直し（F-009）。
-   issue #817 の本文への、ADR-0044 の名前の書き足し（AC-16）。

### 2.3. スコープ外 (Out of Scope)

-   `TestPackageReferenceListsPackages` の変更。このテストは `|` で始まるすべての行の最初のコードスパンを集める（`internal/pipeline/pipeline_test.go:672`）ので、表を分けても今のまま読める。
-   コードの振る舞いの変更。Go のファイルで変えるのは、doc コメントと、F-009 のテストだけである（AC-12）。
-   過去のタスク文書の変更。理由は 1 章の「要件定義の際に決めたこと」にある。
-   ADR の決定の変更。ADR の参照の直しは、編集上の訂正として行う（F-005）。
-   `.claude/commands/_context.md:42`・`.claude/commands/mkarch.md:57`・`CLAUDE.md:298`・`CLAUDE.md:342`・`docs/dev/project_overview.md:113` の参照。どれも文書の名前と、strictjson の利用者の一覧の置き場所を指すだけで、行を絞った後も事実のままである。
-   行き先の文書（`docs/dev/` の下の仕様の文書と guide、README、`prompts/README.md`）の内容の見直し。行の詳細がこれらにすでにあるときはリンクするだけで、これらの文書は変えない（`pricing_update.md:39` の 1 文を除く）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 冒頭の運用の規則に、行の粒度と詳細の置き場所を定める

パッケージの参照表の冒頭に、行に何を書き、何を書かないかと、挙動の詳細をどこに置くかを書く。

**Acceptance Criteria**:
- **AC-01**: 冒頭は、行に書くものと書かないものを述べる。
    -   書くもの：責務（1〜3 文）、主な公開の型・関数・番兵の名前、詳しい仕様の置き場所へのリンク。
    -   書かないもの：非公開の名前、数値と上限値、終了コード、子プロセスに渡す環境変数の一覧、ファイル名、手順。
- **AC-02**: 冒頭は、挙動の詳細の置き場所を述べる。置き場所は、パッケージと公開の識別子の doc コメント、今の文書（`docs/dev/` の下の仕様の文書と guide、README、`prompts/README.md`）、今の挙動を述べるタスクの設計書の節である。ほかの文書にすでにある詳細は、行に写さずにリンクすることも述べる。
- **AC-03**: 冒頭は、今の更新の規則（パッケージを足すか、消すか、責務を変えるコミットで、この文書を更新する。`docs/dev/conventions/package_reference.md:3-6`）を保つ。
- **AC-04**: 冒頭は、6 つの種類それぞれの定義を述べる。どの種類にも当たらないパッケージを足すタスクが、種類を足すか定義を広げるかを決めることも述べる。

#### F-002: 表を種類ごとに分ける

**Acceptance Criteria**:
- **AC-05**: 文書は 6 つの見出しを持ち、各見出しの下に表が 1 つずつある。見出しは、パイプラインの構成要素、サーバとホスト、LLM、共有部品、開発ツール、テスト専用のパッケージに当たる。
- **AC-06**: `TestPackageReferenceListsPackages` が求めるパッケージのそれぞれが、6 つの表のどれか 1 つに、ちょうど 1 行で載る。

#### F-003: 行の中身を絞る

**Acceptance Criteria**:
- **AC-07**: どの行も、AC-01 の「書くもの」だけを書き、「書かないもの」を書かない。ADR が置き場所として参照する規則は、責務の文の一部として残してよい。リンクは、行き先の文書ごとに 1 つまでとする。
- **AC-08**: `internal/strictjson` の行は、利用者の一覧を `Shared by A, B, and C` の書式で持ち、その欄はこの一覧で終わる。`TestPackageReferenceStrictjsonUsers` が通る。
- **AC-09**: 実装をすべて終えた後、パッケージの参照表は 15,000 バイト以下である。収まらないときは、上限を変えずに行を絞り直す。上限を変えるのは決定の変更なので、人に確かめる。

#### F-004: 行から外した詳細の行き先

**Acceptance Criteria**:
- **AC-10**: 行から外した詳細のうち、今の文書（AC-02）にすでにあるものは、その行から、その文書へのリンクでたどれる。リンク先がタスクの設計書の節のときは、その節が今の挙動を述べている。
- **AC-11**: 行から外した詳細のうち、ほかのどの文書にもなく、呼び出し側が頼る契約（1 章の「要件定義の際に決めたこと」の定義）は、そのパッケージか公開の識別子の doc コメントに書かれている。
- **AC-12**: 実装の PR で、Go のファイルの差分は、既存のファイルのコメントの行の追加・削除・書き換えと、`internal/pipeline/strictjson_users_test.go`（F-009）の変更に限る。
- **AC-13**: 各実装の PR の説明には、その PR で絞った行について、外した詳細の行き先の一覧がある。一覧は行ごとにまとめ、その行から外した文か節のすべてを挙げる。行き先は、それぞれ「既存の文書へのリンク」「doc コメントへ移した」「コードが正本なので削った」のどれかである。

#### F-005: ADR の参照を直す

**Acceptance Criteria**:
- **AC-14**: 実装をすべて終えた後、ADR がパッケージの参照表の行を規則の置き場所として参照する文は、参照先の行がその内容を持つものだけである。持たない内容を参照していた文は、その ADR がすでに挙げている別の正本か、コード（doc コメント）だけを指す。
- **AC-15**: 実装の PR で、ADR（`docs/dev/adr/` の下）の差分は、参照の文（リンクと、置き場所を述べる部分）の書き換えと、ADR-0044 のずれの記述の書き換えか削除（AC-17）に限る。ADR の参照の各直しは、編集上の訂正として、コミットメッセージの訂正の行と PR の説明の訂正の段落に記録されている（`docs/dev/conventions/requirements_process.md`「Recording in the PR description」）。

#### F-006: 行の粒度の規則を ADR に記録する

**Acceptance Criteria**:
- **AC-16**: 本書の PR で ADR-0044 が作られ、ADR の索引（`docs/dev/adr/README.md`）に行がある。ADR-0044 の「決定」は、次を述べる。
    -   行の粒度（AC-01）と詳細の置き場所（AC-02）。
    -   表を種類ごとに分けること。
    -   粒度の上限を数で決めず、PR のレビューで読んで確かめること。
    -   それぞれの理由と正本の場所（パッケージの参照表の冒頭）。
    -   正本の冒頭がまだこの規則を述べていないことを示すずれの記述。issue #817 の URL にリンクする。

    issue #817 の本文は、ADR-0044 の名前を挙げる。
- **AC-17**: 実装をすべて終えた後、ADR-0044 に、issue #817 にリンクしたずれの記述は残らない。途中の実装の PR は、残るずれだけを述べるように、ずれの記述を書き換える（`docs/dev/conventions/adr.md`「Editorial corrections」の「Discrepancy statements」）。

#### F-007: 関係する文書の文を直す

**Acceptance Criteria**:
- **AC-18**: `CLAUDE.md` の Quick Links にあるパッケージの参照表の説明文が、文書の新しい役割（各パッケージの責務の要約）を述べる。`CLAUDE.md` の差分は、その 1 行の書き換えに限る。
- **AC-19**: `docs/dev/guides/pricing_update.md` の `internal/pricing` の行についての文が、新しい行の中身（主な公開の名前）に合う。そのガイドの差分は、その 1 行の書き換えに限る。

#### F-008: 変更するファイルの範囲

**Acceptance Criteria**:
- **AC-20**: 実装の PR が変えるファイルは、次に限る。ほかのファイルの追加・削除もない。
    -   パッケージの参照表
    -   `cmd/`・`internal/`・`prompts/` の下の既存の Go のファイル（AC-12 の範囲）
    -   `docs/dev/adr/` の下の ADR（AC-15 の範囲）
    -   `CLAUDE.md`（AC-18）
    -   `docs/dev/guides/pricing_update.md`（AC-19）
    -   本タスクのディレクトリ（`docs/tasks/0085_package_reference_summary/`）の下のファイル

#### F-009: strictjson の利用者の一覧を、すべての表から探す

**Acceptance Criteria**:
- **AC-21**: `TestPackageReferenceStrictjsonUsers` は、`Package` の列を持つすべての表から `internal/strictjson` の行を探す。行がどの表にもないとき、2 つ以上あるとき（別々の表にあるときを含む）は失敗する。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)
-   N/A。文書とコメントの変更だけで、実行時の処理を変えない。

### 4.2. セキュリティ (Security)
-   N/A。`docs/dev/security.md` の規則と前提を変えず、コードの振る舞いも変えない。行から外す詳細には、秘密の扱いや子プロセスの環境変数の一覧も含まれる。それらの正本はコード（たとえば `codexreview` が子プロセスに渡す環境変数は `internal/codexreview/run.go`）と、`security.md` か今の挙動を述べる設計書である。本タスクは、行からその写しを外すだけである。

### 4.3. 信頼性・可用性 (Reliability/Availability)
-   N/A。文書とコメントの変更だけである。

### 4.4. 互換性 (Compatibility)
-   N/A。パッケージの参照表を読む 2 つのテストは、F-009 の直しの後、表を分けても通る。

### 4.5. 保守性 (Maintainability)
-   `make test` と `make lint` が通る。文書のガード（`TestADRRecords`・`TestInstructionDocPaths` を含む）も通る。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   ADR の作成と編集上の訂正は、`docs/dev/conventions/adr.md` に従う。
-   パッケージの参照表は指示の文書なので、英語で書く（`CLAUDE.md`「Document language」の原則 (b)）。ADR-0044 は日本語で書く。
-   タスク 0083（Slack Webhook への投稿の削除、[#813](https://github.com/isseis/yt2column/pull/813) で要件が承認済み）も、パッケージの参照表を変える（`docs/tasks/0083_remove_slack_webhook/01_requirements.md:83`）。どちらの実装も、そのときの `main` の表を出発点にする。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する。
-   **パッケージの参照表:** `docs/dev/conventions/package_reference.md`。`cmd/`・`internal/` の下のパッケージと `prompts` パッケージを、責務とともに挙げる文書。
-   **doc コメント（godoc）:** Go のパッケージ宣言か公開の識別子の直前に書くコメント。
-   **ずれの記述:** ADR の「決定」のうち、決定と正本の文書とコードの食い違いを述べ、それを扱う issue にリンクする記述（`docs/dev/conventions/adr.md`「Writing an ADR」）。
-   **編集上の訂正:** 決定を変えない、決まった種類の ADR の直し（`docs/dev/conventions/adr.md`「Editorial corrections」）。

## 変更履歴

- 2026-11-11 承認（isseis、#821）
