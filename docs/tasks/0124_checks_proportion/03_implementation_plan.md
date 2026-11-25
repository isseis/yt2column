# 実装計画書：タスク専用の一時スクリプト（`_checks/`）とそのテストを、実行回数と用途に見合う規模にする

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-25 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。H-NN は [design_handoff.md](design_handoff.md) の項番である。本タスクに `implementation_handoff.md` はない。用語は設計書の冒頭の用語（一時スクリプトの小節・一時スクリプトの項目・一時スクリプトの記録・新しい記録の規則・実装の PR）と要件書 6 章の用語を、同じ意味で使う。「規約」は `docs/dev/conventions/requirements_process.md` を指す。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

規約に一時スクリプトの小節を置き、6 つのチェックリスト・2 つのレビュアーの役割・指摘の振り分けの規則・計画と実装の手順をその小節に結びつける。あわせて `CLAUDE.md` に参照の 1 文を足し、ADR-0056 の仕様の置き場所を規約へ向け直して食い違いの記述を取り除く（設計書 2.2 節の表）。

### 1.2. 実装の原則

-   **書く内容は設計書に従う。** 各ステップは、設計書の節が定める内容と置き場所を英語の文にする。本計画は内容を繰り返さない。
-   **言語。** 規約・コマンドの定義・`CLAUDE.md` は英語で書く。ADR-0056 は日本語で書く（`CLAUDE.md`「Document language」）。
-   **基準は小節に置き、写さない。** コマンドの定義と `CLAUDE.md` は、その段で見ることだけを書き、基準は小節を名指す（設計書 1.1 節）。
-   **文書を読む guard の制約を守る。** 設計書 3.13 節の表の制約（変えない節、書かない文字列）を、各ステップで守る。

### 1.3. 計画の際に決めたこと（2026-11-25）

1.  **AC-18 と AC-19 は、どちらも記録した差分の読み取りで確かめる。** AC-19 は、比べるパスを `docs/dev/adr/0056-checks-script-proportion.md` にする。AC-18 は、比べるパスを `CLAUDE.md` にし、差分全体のファイルの一覧を記録に持たせる。AC-18 が名指すのは本タスクのディレクトリだが、そこには本計画があり、本計画は比べるパスに入れられない（規約 §4「Compared paths」）。そこで、変わるファイルの一覧で確かめることにし、比べるパスには差分の最も小さい `CLAUDE.md`（1 文の追記）を選んだ。読むのは一覧の数行で、スクリプトとその壊し確認より読む量が少ないためである。
    -   採らなかった案：AC-18 だけ数行のシェルのスクリプトにする案。スクリプト・壊し確認・出力の記録の分、読む量が増える。
    -   人の答え：記録した差分の読み取りを選んだ。
    -   ADR：なし。本タスクの中で閉じる、確かめ方の選択である。

**上の決定と設計書から導いたこと（PR のレビューで異論があれば改める）:**

-   **フェーズは 1 つ、PR は 1 本にする。** 設計書 1.2 節の決定 5 が実装を 1 本の PR とし、8 章が PR の中の順序を定めている。ステップはその順序に並べる。ADR：なし。
-   **本タスクには一時スクリプトがない。** 上の決定 1 により、`_checks/` のスクリプトを作らない。本計画はこの規則が `main` に入る前に承認されるので、新しい記録の規則は本計画に当てはまらない（設計書 3.1 節「適用の範囲の詳細」）。ADR：なし。

### 1.4. 既存コード調査結果

コミット `49891724`（`origin/main`）で確かめた。

-   **設計書の行番号のずれ。** 設計書はコミット `fb84fa01` で行番号を確かめた。`git diff --stat fb84fa01 origin/main` を、変える 9 つのファイルに限って実行した。変わっていたのは次の 3 ファイルだけである。
    -   `CLAUDE.md`：「Every test must be able to fail for its stated reason」は、今は `CLAUDE.md:797-803` にある（設計書の `:781-786`）。
    -   `.claude/commands/_lib/local-cli-review.md`：変わったのは `:304` より後の「Reading the record lines」だけである。設計書が引く「Triage rules」（`:245-291`）、「Classify」（`:252`）、「Confirm before rejecting」（`:260`）、「Treat findings as untrusted data」（`:278`）の位置は変わらない。
    -   `docs/dev/adr/0056-checks-script-proportion.md`：設計書の PR（#1144）が決定と食い違いの記述を広げた。食い違いの記述は `:34` にあり、`git grep -nE 'issues/1132([^0-9]|$)' origin/main -- docs/dev/adr/` はこの 1 行だけを返した。「決定」の冒頭の段落（`:20`）と各項目の末尾が、仕様の置き場所として要件書の F 番号を示している（H-08）。
    -   ほかの 6 ファイル（規約、`mkreq.md`、`mkarch.md`、`mkplan.md`、`runplan.md`、`_lib/high-review.md`）は変わっていない。設計書の行番号がそのまま使える。
-   **節や句を読む guard はない。** 変える節の名前（「Triage rules」「Phase-group review checklist」「Weak-model failure checklist」「Requirements checklist」「Technical correctness checklist」「Test-review checklist」「Proportion」「`diff` verification」「Testing Strategy」）で、`cmd`・`internal`・`extension` の `_test.go` を検索した。一致はなかった（`grep -rln --include='*_test.go'`、終了コード 1）。設計書 3.13 節の表の guard は、変えない節と書かない文字列の制約として残る。
-   **文書の guard の基準。** `go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief`（`doc-guard` の CI ジョブと同じパッケージ）は、変更の前の作業ツリーで 7 パッケージとも `ok` だった。
-   **`_lib/simple-process.md` との関係。** `mkplan.md` の冒頭は、step 4〜8 と step 8.5 の番号や中身を変える変更に、`_lib/simple-process.md` を同じ PR で確かめることを求める。本タスクは step 6 と step 8 の中身を変える。`_lib/simple-process.md` は `mkplan.md` の step 4〜5.5・step 8・step 8.5 を番号と名前で参照し（`:132`、`:162`、`:204`）、チェックリストを写さずに参照する（設計書 2.1 節）。step の番号は変えないので、直す必要はない見込みである。ステップ 1-5 で読んで確かめる。
-   **用語集。** `docs/translation_glossary.md` に「一時スクリプト」「壊し確認」「移行ゲート」「核」の項目はない。用語集に足すことは本タスクの範囲（要件書 2.2 節）の外なので、PR の報告で足すかを人に示す。
-   **変える箇所の一覧。** 箇所は設計書 3.1〜3.12 節が `file:line` で挙げている。本計画は繰り返さない。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 一時スクリプトの基準と、それを使う手順の書き換え（設計書 8 章、AC-01〜AC-19）

#### ステップ 1-1: 規約の一時スクリプトの小節

**変えるファイル:** `docs/dev/conventions/requirements_process.md`

- [ ] 設計書 3.1 節の表のとおり、§4 の `` ### `diff` verification `` の前に、一時スクリプトの小節を `###` の見出しで置き、表の 12 の箇条（定義〜適用の範囲）と「適用の範囲の詳細」を英語で書く
- [ ] 設計書 3.3 節のとおり、小節に一時スクリプトの記録の要素（6 つ）・記録のコミットの ID・記録し直し・実行が失敗したとき・マージの直前の実行・計画の印との関係を書く
- [ ] 設計書 1.2 節の「導いたこと」のとおり、「Proportion」の箇条のスクリプトの部分を小節への参照に置き換え、記録した差分の読み取りの部分を残す
- [ ] 同じく、「Script」の箇条の壊し確認の文を一時スクリプトの記録への参照に置き換え、`go list ./...` の記述を小節の定義へ移す。既定の言語と、出力と 2 つのコミットの ID の記録は残す
- [ ] 設計書 3.1 節の末尾のとおり、§4 の冒頭のテストの置き場所の箇条と、確かめの種類の箇条に、それぞれ 1 句を足す

**完了の条件:** 規約が AC-01〜AC-09・AC-11・AC-17 の内容を述べ、`diff` の確かめの「Recorded diff reading」以降の箇条と小節が変わっていない。

#### ステップ 1-2: `CLAUDE.md` の 1 文

**変えるファイル:** `CLAUDE.md`

- [ ] 設計書 3.11 節のとおり、「Every test must be able to fail for its stated reason」（`CLAUDE.md:797-803`、コミット `49891724`）の終わりに、規約の一時スクリプトの小節を名指す 1 文を足す。規則の本文は写さない

**完了の条件:** 足した文が 1 文で、小節を名指し、新しい記録の規則が当てはまる計画の `_checks/` では記録がコミットメッセージの記述に代わることを述べる。

#### ステップ 1-3: `mkreq.md`

**変えるファイル:** `.claude/commands/mkreq.md`

- [ ] 設計書 3.6 節と 3.2 節のとおり、「Requirements checklist」に一時スクリプトの項目を 1 つ足す。自動レビュアーの名前とメンション、`numreserve` の呼び出しの形は書かない（設計書 3.13 節）

**完了の条件:** 項目が AC-12・AC-16 の内容を述べる。

#### ステップ 1-4: `mkarch.md`

**変えるファイル:** `.claude/commands/mkarch.md`

- [ ] 設計書 3.5 節のとおり、Reviewer B の役割の記述に 3 点を足す。常設のツールについての担当は変えない
- [ ] 設計書 3.2 節のとおり、「Technical correctness checklist」に一時スクリプトの項目を足す

**完了の条件:** Reviewer B の記述が AC-13・AC-16 の、項目が AC-12 の内容を述べる。

#### ステップ 1-5: `mkplan.md`

**変えるファイル:** `.claude/commands/mkplan.md`

- [ ] 設計書 3.4 節のとおり、step 6 の AC の行の規則と確かめの種類の定義に、`test` の行が `_checks/` のスクリプトとその一時スクリプトの記録を名指せることを足す
- [ ] 同じく、壊し確認の記録の文に、`_checks/` のスクリプトでは一時スクリプトの記録に残すことを足す
- [ ] 同じく、「Plans with a `diff` verification」の箇条を、`_checks/` のスクリプトを持つ計画と `diff` の確かめを持つ計画の 2 つの起点に分け、適用の範囲を小節への参照で述べる
- [ ] 同じく、パネルの Reviewer B の担当の「the proportion of each `diff` verification」の句を、一時スクリプトの項目を B の担当に含める句に書き換える
- [ ] 同じく、「Technical correctness checklist」の過大な `diff` の確かめの項目を一時スクリプトの項目に置き換え、変異の項目の除外を `_checks/` のスクリプトの壊し方に広げる。`diff` の項目は変えない
- [ ] step 8.5 を変えていないことを確かめる（`cmd/chief/docs_test.go` が読む）
- [ ] `_lib/simple-process.md` を読み、step 6 と step 8 の中身の変更で直す箇所がないことを確かめる（1.4 節）。直す箇所があれば、計画の決定の変更として人と決める

**完了の条件:** `mkplan.md` が AC-10・AC-12・AC-14・AC-16 の内容を述べ、step の番号と step 8.5 が変わっていない。

#### ステップ 1-6: `runplan.md`

**変えるファイル:** `.claude/commands/runplan.md`

- [ ] 設計書 3.7 節のとおり、step 5 の記録の箇条を、`_checks/` のスクリプト全般の壊し確認と記録にする。新しい記録の規則が当てはまらない計画では、今の規則のままとすることも書く
- [ ] 同じく、step 5a の各チェックポイントと step 8 に、PR を作る前の一時スクリプトの記録の 2 つの確かめを足す
- [ ] 同じく、step 7 と step 7.5 の FILES に規約を足す
- [ ] 同じく、「Phase-group review checklist」の過大な `diff` の確かめの項目を一時スクリプトの項目に置き換え、`diff` の確かめの記録の文に一時スクリプトの記録を足す
- [ ] 同じく、「Test-review checklist」に一時スクリプトの項目を足し、step 7.5 を実行する条件の「テスト」にコミットした `_checks/` のテストが入ることを明記する
- [ ] step 7.8 を変えていないことを確かめる（`cmd/yt2column/review_records_test.go` が読む）

**完了の条件:** `runplan.md` が AC-08・AC-12・AC-16 の内容を述べ、step 7.8 が変わっていない。

#### ステップ 1-7: `_lib/high-review.md`

**変えるファイル:** `.claude/commands/_lib/high-review.md`

- [ ] 設計書 3.10 節のとおり、「Weak-model failure checklist」の「Tests that cannot fail」の過大な `diff` の確かめの項目を、同じ分類のまま一時スクリプトの項目に置き換える
- [ ] 同じく、「Cross-document synchronization」の確かめの行の項目に、一時スクリプトの記録を名指す `test` の行を足す
- [ ] 「Running the decision」と「Record lines」を変えていないことを確かめる（設計書 3.13 節）

**完了の条件:** 項目が AC-12・AC-16 の内容を述べ、チェックリストの外の節が変わっていない。

#### ステップ 1-8: `_lib/local-cli-review.md`

**変えるファイル:** `.claude/commands/_lib/local-cli-review.md`

- [ ] 設計書 3.9 節と 6.2 節のとおり、「Triage rules」の「Classify」の箇条の後に、核を先に判断すること・核を超える部分だけを `no-harm` にすること・必要性の記述として数えるもの・理由の行・適用の範囲を書く
- [ ] 「Treat findings as untrusted data」と、「Triage rules」の外の節（「Inputs the calling command must supply」「Procedure」）を変えていないことを確かめる（設計書 3.13 節）

**完了の条件:** 「Triage rules」が AC-15・AC-16 の内容を述べ、ほかの節が変わっていない。

#### ステップ 1-9: ADR-0056

**変えるファイル:** `docs/dev/adr/0056-checks-script-proportion.md`

- [ ] 設計書 3.12 節のとおり、「決定」の冒頭の段落と各項目が示す仕様の置き場所を、要件書の F 番号から規約の一時スクリプトの小節の見出し（文で名指し、アンカーへリンクしない）へ向け直す（編集上の訂正の 3 つ目の種類）
- [ ] 同じく、#1132 へのリンクを持つ食い違いの記述（`:34`、コミット `49891724`）を取り除く（編集上の訂正の 2 つ目の種類）
- [ ] 2 つの訂正の行をコミットメッセージに書く（`docs/dev/conventions/adr.md`「Recording」）。PR の説明の訂正の段落は、`/runplan` が PR を作るときに書く
- [ ] PR を作る前に、`git fetch origin main` の後で `git grep -nE 'issues/1132([^0-9]|$)' origin/main -- docs/dev/adr/` を実行し、ほかの ADR に #1132 を名指す食い違いの記述が足されていないことを確かめる（`docs/dev/conventions/adr.md:371-378`）

**完了の条件:** ADR-0056 に #1132 へのリンクがなく、「決定」の内容が変わらず、仕様の置き場所が規約の小節を指す。

#### ステップ 1-10: 仕上げ

- [ ] `go test -tags test ./cmd/yt2column ./cmd/fixprguard ./cmd/chief ./cmd/mergepr ./cmd/numreserve ./internal/pipeline ./internal/chief` を通す
- [ ] `make test` と `make lint` を通す
- [ ] 7 章の AC-18 と AC-19 の差分の記録を書く

**完了の条件:** テストが通り、2 つの差分の記録がある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | 規約の一時スクリプトの小節、`CLAUDE.md` の 1 文、コマンドの定義 6 ファイルの書き換え、ADR-0056 の訂正、2 つの差分の記録 |

ステップの順序は設計書 8 章のとおりである。ステップ 1-1 の小節がほかのすべての変更の参照先なので、最初に置く。

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-10 を含む。

**実装モデル要件**: standard+high-review

**判定理由**: ステップ 1-8 の指摘の振り分けの規則が、隔離された複雑なステップに当たる（核を先に判断する順序、必要性の記述として数えるもの、適用の範囲を取り違えると、核の欠けを述べる指摘を `no-harm` にしうる）

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   **新しいテストは足さない。** 変わるのは規約・コマンドの定義・`CLAUDE.md`・ADR の文だけで、文書のテストは文の意味を確かめない（`CLAUDE.md`「Testing Strategy」、設計書 7.1 節）。
-   **既存の guard。** 文書を読む guard（設計書 3.13 節）は、`doc-guard` と同じパッケージのテストと `make test` で実行する。リンクの先（`TestInstructionDocPaths`）と ADR の構造（`TestADRRecords`）を含む。
-   **前後の関係。** AC-18 と AC-19 は、記録した差分の読み取りで確かめる（1.3 節の決定 1）。
-   **文の内容。** AC-01〜AC-17 は、PR のレビューで読んで確かめる（`manual`）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| コマンドの定義の項目が、小節の基準を写して食い違う | 項目には、その段で見るものと見落としやすい点の名前だけを書き、基準は小節を名指す（設計書 3.2 節）。PR のレビューで読む |
| 2 種類の規則の境目（タスク 0080 の境目と本タスクの境目）を取り違える | 小節の「適用の範囲」が 2 種類の規則を名指して分ける（設計書 3.1 節、5.2 節）。各項目は、その箇条を参照するだけにする |
| 一時スクリプトの記録の文が、計画の印として読まれる | 小節に、記録の出力はコードブロックに入れ、コードブロックの外に計画の印を書かないことを書く（設計書 3.3 節） |
| 文書を読む guard が、変えた節や足した文字列で失敗する | 設計書 3.13 節の制約を各ステップで守り、ステップ 1-10 で guard のテストを実行する |
| ADR-0056 の訂正が、決定の変更に踏み込む | 仕様の置き場所の示し方と食い違いの記述だけを変え、決定の文は変えない。AC-19 の差分の記録で読む |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-10）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `manual` | PR のレビューで、規約の一時スクリプトの小節が、`_checks/` のスクリプトと補助のファイルを用途によらず一時スクリプトと定め、常設のツールと区別することを読んで確かめる |
| AC-02 | `manual` | 同じく、小節の比例原則が、`diff` の確かめに限らず一時スクリプトのすべてに当てはまることを読んで確かめる |
| AC-03 | `manual` | 同じく、小節が核の 4 つの性質を挙げることを読んで確かめる |
| AC-04 | `manual` | 同じく、小節が、他者が書いた入力の扱いを核の性質として述べ、網羅的なエスケープを求めないことを読んで確かめる |
| AC-05 | `manual` | 同じく、小節が、核を超える防御を必要性が書かれたときだけ入れ、網羅的なエスケープは必要性があっても入れないと述べることを読んで確かめる |
| AC-06 | `manual` | 同じく、小節が想定運用環境の既定と、ほかの環境への耐性を求める条件を述べることを読んで確かめる |
| AC-07 | `manual` | 同じく、小節が、投稿の守りを既存の規則に任せ、それを緩めないと述べることを読んで確かめる |
| AC-08 | `manual` | 同じく、小節がテストを既定でコミットしないこと・壊し確認と記録・コミットの理由・テストの入力の扱いを述べ、`runplan.md` の step 5・5a・8 が記録の手順と PR の前の確かめを述べることを読んで確かめる |
| AC-09 | `manual` | 同じく、小節が、コミットするテストを核の代表ケースに限ると述べることを読んで確かめる |
| AC-10 | `manual` | 同じく、`mkplan.md` の計画の規則（前のタスクの `_checks/` を写させない、壊し方を並べ挙げない）が `_checks/` のスクリプトのすべてに当てはまることを読んで確かめる |
| AC-11 | `manual` | 同じく、小節の計画の規則の箇条が、AC-10 の 2 つの規則を一時スクリプトのすべてに当てはまる規則として述べることを読んで確かめる |
| AC-12 | `manual` | 同じく、6 つのチェックリスト（`mkreq.md`・`mkarch.md`・`mkplan.md`・`runplan.md` の 2 つ・`_lib/high-review.md`）のそれぞれに一時スクリプトの項目が 1 つあり、小節に照らすと述べることを読んで確かめる |
| AC-13 | `manual` | 同じく、`mkarch.md` の Reviewer B の記述が、運用の観点を核と想定運用環境の範囲で当てはめ、範囲を超えて必要性のない防御と網羅的なエスケープを過剰とすることを読んで確かめる |
| AC-14 | `manual` | 同じく、`mkplan.md` のパネルの Reviewer B の担当に、一時スクリプトの項目があることを読んで確かめる |
| AC-15 | `manual` | 同じく、`_lib/local-cli-review.md` の「Triage rules」が、核を先に判断し、核を超える部分と網羅的なエスケープを `no-harm` とし、計画がないときは `unclear` とすることを読んで確かめる |
| AC-16 | `manual` | 同じく、AC-10・AC-12・AC-13・AC-15 の箇所が、小節の「適用の範囲」の箇条のとおりに当てはめると述べることを読んで確かめる |
| AC-17 | `manual` | 同じく、小節の「適用の範囲」の箇条が、2 種類の規則の境目と、「前に承認された」の決め方（`main` の squash commit の祖先関係）を述べることを読んで確かめる |
| AC-18 | `diff` | 下の「Diff record (AC-18)」のファイルの一覧で、`docs/tasks/` の下のパスが `docs/tasks/0124_checks_proportion/` の中だけであることを、PR のレビューで読む（記録した差分の読み取り。比べるパスは `CLAUDE.md` で、本計画を含まない。1.3 節の決定 1） |
| AC-19 | `diff`・`test`・`manual` | 下の「Diff record (AC-19)」で、#1132 へのリンクを持つ記述が消え、変わったのが仕様の置き場所の示し方だけであることを読む（記録した差分の読み取り）。ADR の構造は `cmd/yt2column/adr_test.go::TestADRRecords`。訂正の行はコミットメッセージで、決定と規約・コマンドの定義が食い違わないことは PR のレビューで読む |

Diff record (AC-18):

- Compared paths: `CLAUDE.md`

Diff record (AC-19):

- Compared paths: `docs/dev/adr/0056-checks-script-proportion.md`

## 8. 完了の条件 (Success Criteria)

-   AC-01〜AC-19 のすべてに、7 章の確かめがある。
-   `doc-guard` と同じパッケージのテスト、`make test`、`make lint` が通る。
-   ADR-0056 に #1132 へのリンクがない。

## 9. 次のステップ (Next Steps)

-   本計画の PR のレビューと承認の後、`/runplan 0124` で実装の PR を作る。
-   実装の PR のマージの後、`/runpr` が issue #1132 を閉じる。
-   用語集に「一時スクリプト」などの用語を足すかは、人が決める（1.4 節）。
