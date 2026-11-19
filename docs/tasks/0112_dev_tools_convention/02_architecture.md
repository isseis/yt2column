# 設計書：開発の道具の規則を ADR-0014 から規約 dev_tools.md に移す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 設計の全体像

本タスクは文書だけを変える。英語の規約 `docs/dev/conventions/dev_tools.md` を新しく作り、規則 1〜6 を書く（[01_requirements.md](01_requirements.md)（以下、要件書）F-001）。ADR-0014 の「決定」のうち、規則 1〜6 の 6 つの項目を、要約・短い理由・規約へのリンクに書き直す（F-002）。`CLAUDE.md` に 2 項目を足し（F-003）、ADR-0014 を規則の置き場所として指している 3 か所を、規約を指すように直す（F-004）。

規則 5 を広げる決定は、本 PR（3 文書の PR）で ADR-0014 にすでに入っている（AC-07）。実装の PR は、ADR-0014 を編集上の訂正だけで変え、「経緯」には項目を足さない（AC-09・AC-10）。

**設計原則:**

-   **規則の正本は規約、理由の正本は ADR。** 規約は規則だけを書き、理由は ADR-0014 へリンクする（AC-05）。ADR は要約と理由を書き、規則の本文は規約へリンクする（AC-08）。同じ内容を 2 か所に持たない。
-   **一覧は持たない。** 規約は道具の名前を並べず、種類で示して `CLAUDE.md` の「Build Commands」へリンクする（AC-02）。道具を足しても規約は変わらない。
-   **ほかの正本を写さない。** chief の例外の範囲は ADR-0014 と ADR-0046 に、プロトコル番号の規則は `CLAUDE.md` の「Protocol numbers」に、道具ごとの許可は `CLAUDE.md` の「Tool Execution Safety」にある。規約はそれらへリンクするだけにする（AC-03・AC-04・AC-06）。

**設計の際に決めたこと（2026-11-19）:**

-   **規約は、対象の道具を名前の一覧ではなく種類で示し、一覧は `CLAUDE.md` の「Build Commands」へのリンクで示す（AC-02）。** 2026-11-19 に人が選んだ。名前を並べると、道具を足すたびに規約も直す必要があり、直し忘れを確かめる文書のテストも要るためである。`CLAUDE.md`「Tool Execution Safety」に足す項目（AC-13）も、同じ種類で書く。
    -   採らなかった案：今の 6 つの道具の名前を規約に並べる案。
    -   ADR: なし。規約の書き方の選択で、本タスクの中で閉じる。

本書の行番号は、コミット `a73b2c4e`（`origin/main`）で確かめた。

概念モデルとシステム構成の Mermaid の図は省く。変えるのは文書だけで、コンポーネントの関係は変わらない（`docs/dev/conventions/requirements_process.md`「Simple Process」の「May be omitted」）。

## 2. システム構成

N/A。コンポーネントの構成とデータの流れを変えない。

## 3. コンポーネント設計

### 3.1. 変更するファイル

| ファイル | 変更 | 内容 |
|---|---|---|
| `docs/dev/conventions/dev_tools.md` | 新規 | 規則 1〜6 の規約（3.2 節、AC-01〜AC-06） |
| `docs/dev/adr/0014-deterministic-checks-in-go.md` | 書き換え | 「決定」の規則 1〜6 の 6 項目（3.3 節、AC-08〜AC-10） |
| `CLAUDE.md` | 2 項目の追加 | 「Quick Links」と「Tool Execution Safety」（3.4 節、AC-12〜AC-14） |
| `docs/dev/guides/chief_guide.md` | 1 か所の書き換え | `:22` の参照（3.5 節、AC-15） |
| `docs/dev/guides/numreserve_guide.md` | 1 か所の書き換え | `:11` の参照（3.5 節、AC-15） |
| `.claude/commands/_lib/adr-correction-candidates.sh` | コメントの書き換え | `:62-64` の参照（3.5 節、AC-16） |

### 3.2. 規約 dev_tools.md の構成

英語で書く。見出しは次のとおりとする。ADR-0014 は、この見出しの名前を本文で示して節を指す（3.3 節）。見出しの言い回しは実装で変えてよいが、変えたら ADR-0014 の参照も同じ PR で揃える。

| 見出し | 書くこと | 規則 |
|---|---|---|
| `# Development Tools` | 冒頭で、規約が何の規則か（開発の手順のための Go の道具を足すときと変えるとき）、規則の理由は ADR-0014 にあること（AC-05）を書く。関連する規則として、プロトコル番号は `CLAUDE.md` の「Protocol numbers」にあることを示す（AC-06） | — |
| `## Kinds of tool` | 道具の 2 つの種類を定める。1 つは `make install-<name>` で `main` から入れる道具（installed tool）、もう 1 つは make のターゲットがそのつどビルドして走らせる道具（built-per-run tool）。道具の一覧は `CLAUDE.md` の「Build Commands」へリンクする（AC-02） | — |
| `## Write a tool in Go, with tests` | 判定と副作用を持つ道具は Go で書きテストを持つこと。`cmd/<name>` と `internal/<name>` の組とし、製品のバイナリには入れないこと | 1 |
| `## Do not read a baseline back from a state file` | 判定の基準値を状態のファイルから読み返さないこと。基準値はサブコマンドの出力として返し、セッションが次の呼び出しの引数で渡すこと。道具が利用者のために記録を書いても、判定には読み返さないこと。chief には例外があることだけを書き、範囲は書かずに ADR-0014 と ADR-0046 へリンクする（AC-03） | 2 |
| `## Enforce the order in the tool` | 処理の順序は道具のコードで強制すること。コマンドの定義には、サブコマンドを呼ぶ順序と利用者への確認だけを書くこと | 3 |
| `## The interface is a contract` | 引数・出力・終了コードは、コマンドの定義と文書のガードが読む契約であること。変えるときは、道具・呼ぶ文書・ガードを同じ PR で変えること | 4 |
| `## Install from main` | installed tool は利用者が `main` から入れ、道具を変えたマージの後に入れ直すこと。エージェントは installed tool をインストールもビルドもしないこと。built-per-run tool はこの節の対象でないこと。道具ごとの記述と許可は `CLAUDE.md` の「Tool Execution Safety」へ、プロトコル番号を上げる PR での入れ直しは `CLAUDE.md` の「Protocol numbers」と各道具のガイドへリンクする（AC-04） | 5 |
| `## Exception: a shell script for a decision that only reads records` | 例外の条件の 4 つ。`.claude/commands/_lib/` に置くこと、`origin/main` の commit から作業ツリーの外に取り出して走らせること、失敗は安全な側の判定とすること、規則の全体を常設の Go のテストで確かめ、規則を壊して失敗することを確かめること | 6 |

規約は、道具の名前を一覧として並べない（AC-02）。規則を説明するために 1 つの道具を例に挙げることは妨げない（例：規則 2 の chief）。

### 3.3. ADR-0014 の書き直し

「決定」の 6 項目を次のように書き直す。各項目は、太字の要約 1 文、短い理由、規約の節への参照（`[dev_tools.md](../conventions/dev_tools.md) の「<見出し>」`）の順に書く（`docs/dev/conventions/adr.md`「What 決定 holds」「Links」）。理由は今の項目の理由を使い、新しい理由を足さない。

| 今の項目 | 書き直し後の参照先の節 | 外す本文（規約か、ほかの正本にあるもの） |
|---|---|---|
| `:17` 判定と副作用を持つ道具は Go で書く | `Write a tool in Go, with tests` | 道具の名前の一覧と「製品の 3 つのバイナリ」の記述。ほかの ADR へのリンク（何を決めるか）は残す |
| `:18` 基準値を状態のファイルから読み返さない | `Do not read a baseline back from a state file` | 基準値の渡し方の詳細。`fixprguard_guide.md` の保証へのリンクは残す |
| `:20` 順序を道具の中で強制する | `Enforce the order in the tool` | コマンドの定義に残すものの詳細。`fixprguard_guide.md` の保証へのリンクは残す |
| `:22` 道具は `main` からインストールして使う | `Install from main` | 道具とターゲットの一覧、`coveragecompare` のビルドの詳細。プロトコル番号を上げる PR の入れ直しは、ガイドへのリンクとして残す。本 PR が足したエージェントの規則（AC-07）は、要約と理由を残し、置き場所の参照を規約と `CLAUDE.md`「Tool Execution Safety」にする |
| `:24` 引数・出力・終了コードは契約にする | `The interface is a contract` | 契約が書かれている文書の一覧 |
| `:25` 記録の行を読むだけの判定はシェルスクリプトにしてよい | `Exception: a shell script for a decision that only reads records` | 4 つの条件の本文。今あるスクリプトの名前と、`opus-review-decision.sh` が例外より前に作られ常設のテストが規則の全体を確かめていないことは、残す。規約にない事実で、規則ではないためである |

`:19`（chief の例外）、`:21`（TypeScript の補助）、`:23`（プロトコル番号）と、「背景」「採らなかった案」「結果」「経緯」は変えない（AC-09）。

**決定を変えないことの確かめ。** 書き直しは編集上の訂正の 1 つ目の種類（基準に合わせた書き直し）に当たる（`docs/dev/conventions/adr.md`「Editorial corrections」）。外す本文は、規約（3.2 節）か、すでにある正本（`CLAUDE.md`、ガイド）に同じ意味で書かれていなければならない。実装の際は、外す本文ごとに、それが規約か正本のどこにあるかを確かめる。どこにもない本文は外さずに残す。

### 3.4. CLAUDE.md の 2 項目

-   **「Quick Links」の「Development Guides」（`CLAUDE.md:15-18`）:** `[Development Tools](docs/dev/conventions/dev_tools.md)` に、要旨（開発の道具を足すときと変えるときの規則）を添えた 1 行を足す（AC-12）。置く位置は `Package Reference` の行の後とする。
-   **「Tool Execution Safety」（`CLAUDE.md:990`）:** 「エージェントは、利用者が `make install-<name>` で `main` から入れる道具を、インストールもビルドもしない。make のターゲットがそのつどビルドする道具は対象でない。詳細は規約の `Install from main`」という 1 項目を、`codexreview` の項目の前に足す（AC-13）。道具の名前は並べない（設計の際に決めたこと）。`codexreview`・`numreserve`・`chief` の今の記述は変えない（要件書 2.3 節）。

`cmd/numreserve/docs_test.go` の `TestDocumentedInvocations` は「Tool Execution Safety」の節を読み、`numreserve` の呼び出しの形を確かめる。足す項目には `numreserve` の語を書かない。

### 3.5. 参照の付け替え

-   **`docs/dev/guides/chief_guide.md:22` と `docs/dev/guides/numreserve_guide.md:11`:** 「（ADR-0014 の『道具は `main` からインストールして使う』）」を、規約の `Install from main` の節への参照（`[dev_tools.md](../conventions/dev_tools.md) の「Install from main」`）に替える。文のほかの部分は変えない（AC-15・AC-18）。
-   **`.claude/commands/_lib/adr-correction-candidates.sh:62-64`:** コメントの `docs/dev/adr/0014-deterministic-checks-in-go.md` を `docs/dev/conventions/dev_tools.md` に替え、行の折り返しを整える。コメントのほかの意味と、コードは変えない（AC-16）。

## 4. エラーハンドリング設計

N/A。コードの振る舞いを変えない。

## 5. セキュリティ考慮事項

N/A。`docs/dev/security.md` の条件のどれにも当たらない（要件書 4.2 節）。規則 5 を広げることは、エージェントができることを減らす向きの変更で、道具のコードは変えない。

## 6. 処理フローの詳細

N/A。

## 7. テスト戦略

-   規約の文、ADR-0014 の各項目、`CLAUDE.md` の 2 項目、参照の付け替えは、PR のレビューで読む（`manual`）。文書のテストは文の意味を確かめないためである（`CLAUDE.md`「Testing Strategy」）。
-   新しいリンクが解決することは、既存の `TestInstructionDocPaths`（`cmd/yt2column/docpaths_test.go:140`）が確かめる。この関数は `CLAUDE.md` と `docs/dev/` の下の文書（規約、ガイド、ADR）のリンクを読む。
-   ADR-0014 の形は `TestADRRecords`（`cmd/yt2column/adr_test.go`）が確かめる（AC-11）。
-   `adr-correction-candidates.sh` の動作が変わらないことは、`TestADRCorrectionCandidates`（`cmd/yt2column/adr_candidates_docs_test.go`）が確かめる（AC-16）。
-   差分の範囲（AC-09・AC-14・AC-17・AC-18）は、記録した diff を読んで確かめる（`diff`）。
-   新しい文書のテストは足さない。規約は道具の一覧のような、コードと照合する値を持たないためである（設計の際に決めたこと）。

## 8. 実装の優先順位

1 フェーズ、1 本の PR で行う。

## 9. 将来の拡張性

-   ADR-0024 と ADR-0019 の規則を規約に移すときは、別の規約にするか、この規約に節を足すかを、その issue で決める。
-   道具を足しても規約は変わらない。`CLAUDE.md` の「Build Commands」に `make install-<name>` の行を足せば、規則 5 の対象になる。
