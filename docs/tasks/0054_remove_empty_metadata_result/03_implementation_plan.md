# 実装計画書：使われなくなった emptyMetadataResult を削除する

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-02 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）の実装計画である。F-NNN・AC-NN は要件書の項番、「設計書 3.2 節」などは設計書の節を指す。本タスクに `implementation_handoff.md` はない。[design_handoff.md](design_handoff.md) の H-01 は設計書 10 章が記録している。

既存のコードについての記述は、コミット `6ce554f8`（2026-11-02 の `main`）で確かめた。設計書が行番号を引用したコミット `7478e36a` から `6ce554f8` までに、`extension/` は変わっていない（`git diff --stat 7478e36a 6ce554f8 -- extension` の出力は空だった）。そのため、設計書の `file:line` はそのまま使える。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

`extension/src/core/metadata.ts` の `emptyMetadataResult` と、それだけを確かめる `extension/test/metadata.test.ts` のテストを削除する。削除の前と後のカバレッジの報告を比べて、ほかの `src/` の関数の確認が失われていないことを確かめる。何を削除し、どう確かめるかは設計書 2.2 節と 3.2 節で決まっている。本書は、それをどの順序・どの確かめで行うかを定める。

### 1.2. 実装の原則

-   設計書 8 章の順序（1〜7）で実装する。フェーズは 1 つ、PR も 1 つである。
-   拡張のコードの変更は、設計書 2.2 節の表の行の削除だけにする。ほかの行、ほかのファイルは変えない。
-   カバレッジの比較（AC-05）は一度だけ行う確認である。比較の手順は `make` の対象にも `_checks/` のスクリプトにもせず、結果をテストを削除するコミットのメッセージに書く（設計書 3.2 節「残さないもの」）。比較に使う一時的なスクリプトや報告は、リポジトリの外（作業用の一時ディレクトリ）に置く。
-   リポジトリに加えるのは、AC-01〜AC-03 を確かめる `_checks/` のスクリプト 1 つだけである（ステップ 1-4）。

### 1.3. 計画の際に決めたこと（2026-11-02）

レビュアーに尋ねるべき未決の事項はなかった。次の 2 点は、結果が大きく変わらない選択なので、理由とともにここに記録する。

-   **AC-01 の確かめを `_checks/` のスクリプトに含める:** 設計書 7.4 節は AC-01 を `git grep` が何も返さないことで確かめるとした。文書に書いただけのコマンドは実行されないので、同じ確かめを AC-02・AC-03 のスクリプトの中で PR の head に対して行い、AC-01 の行もそのスクリプトを名指しする。確かめる内容は設計書 7.4 節と変わらない。
-   **スクリプトは node で動く TypeScript にする:** 拡張のファイルだけを扱う確かめであり、0046 の `_checks/unchanged_values.ts` と同じ形にそろえる。実行に要る node の版は、AC-04 の `make ext-check` が既に求めている（`extension/.node-version`）。依存パッケージは使わず、`node:child_process` から `git` を呼ぶ。

### 1.4. 既存コード調査結果

**削除するもの（`6ce554f8` で確かめた）**

-   `extension/src/core/metadata.ts`（220 行）: `:216` が空行、`:217` が説明のコメント `/** The empty result used when the collection was rejected: no read is attempted. */`、`:218-220` が `emptyMetadataResult` の定義である。`:220` がファイルの最後の行である。
-   `extension/test/metadata.test.ts`（345 行）: `:6` が import の `  emptyMetadataResult,`、`:336` が空行、`:337-345` が `describe("emptyMetadataResult")` である。この `describe` の `it` は `:338` の 1 つだけで、`:345` がファイルの最後の行である。
-   `git grep -n emptyMetadataResult -- . ':!docs/tasks'` が返すのは、`metadata.ts:218`、`metadata.test.ts:6`・`:337`・`:339` の 4 行だけである。Go のコード、`docs/dev/`、README、`extension/scripts/`、`extension/static/` に名前は現れない。

**変えないもの**

-   `acceptMetadata`（`metadata.ts:174`）と `readFailureResult`（`metadata.ts:208`）、その呼び出し（`extension/src/launch.ts:112`・`:99`・`:105`）とテスト（`metadata.test.ts` のほかの `describe`）。
-   import の残りの 3 つ（`acceptMetadata`・`parsePageMetadata`・`readFailureResult`）は、`emptyMetadataResult` の行を消した後もアルファベット順のままで、Prettier の書式も変わらない。

**実行の環境**

-   `extension/.node-version` は `24.21.0`、手元の `node --version` は `v24.21.0` で一致する。
-   この作業ツリーには `extension/node_modules` がない。`make ext-install`（ネットワークを使う）が要るので、実装の始めに利用者の承認を得る（設計書 3.2 節「実行の前提」）。
-   `extension/package.json:12` の `test` スクリプトは `node --test "test/**/*.test.ts"` で、設計書 3.2 節のコマンドのテストの対象の指定と同じである。

**再利用するもの**

-   新しいテスト、テストの補助のファイルは加えない（設計書 7.1 節）。`_checks/` のスクリプトの形は 0044 の `_checks/extension_unchanged.sh`（終了コード `0`・`1`・`2` の分け方）と 0046 の `_checks/unchanged_values.ts`（`git` からコミットの内容を読み、作業ツリーを読まない）に倣う。

## 2. 実装手順 (Implementation Steps)

### フェーズ 1: emptyMetadataResult の削除と確かめ（設計書 8 章）

#### ステップ 1-1: 実行の前提を確かめ、削除の前の報告を取る

**変更するファイル:** なし

- [ ] 設計書 3.2 節「実行の前提」の 3 つ（node の版、`extension/node_modules`、`extension/` にコミットしていない変更がないこと）を確かめる。`node_modules` がなければ、利用者の承認を得て `make ext-install` を実行する。
- [ ] 設計書 3.2 節「報告の取り方」のコマンドで、削除の前の lcov の報告と spec 形式の出力を、リポジトリの外に取る。このときの `HEAD` のコミット ID を控える（テストを削除するコミットの親になる）。
- [ ] テストが失敗した（終了コードが 0 でない）ときは、設計書 4 章のとおり止める。

**完了の条件:** 削除の前の報告があり、テストがすべて通っている。

#### ステップ 1-2: 関数とテストを削除する

**変更するファイル:** `extension/src/core/metadata.ts`、`extension/test/metadata.test.ts`

- [ ] `metadata.ts` の `:216-220`（空行、説明のコメント、`emptyMetadataResult` の定義）を削除する。削除の後、ファイルは `readFailureResult` の定義の閉じ括弧 `}`（今の `:215`）と改行で終わる。
- [ ] `metadata.test.ts` の `:6`（import の `  emptyMetadataResult,`）を削除する。
- [ ] `metadata.test.ts` の `:336-345`（空行と `describe("emptyMetadataResult")`）を削除する。削除の後、ファイルは `describe("readFailureResult")` の閉じ `});`（今の `:335`）と改行で終わる。
- [ ] `make ext-check` を実行し、成功することを確かめる（AC-04）。失敗したときは設計書 4 章のとおり止める。

**完了の条件:** `make ext-check` が成功する。

#### ステップ 1-3: コミットし、削除の後の報告を取って比べる

**変更するファイル:** なし（コミットのメッセージだけを直す）

- [ ] ステップ 1-2 の変更を 1 つのコミットにする。親はステップ 1-1 で控えたコミットである。
- [ ] そのコミットで、ステップ 1-1 と同じコマンドを実行して削除の後の報告を取る。
- [ ] 設計書 3.2 節「比べるもの」「関数の対応の付け方」「測れていることの確認」の順に、2 つの報告を比べる。比べる処理は一時的なスクリプトとしてリポジトリの外に書く。どれかが成り立たないとき、または削除の前に 1 以上だった実行回数が 0 になった関数があるときは、PR を出さずに、その内容とともにレビュアーに報告する（AC-05、設計書 4 章）。
- [ ] 比較が成り立ったら、`git commit --amend` でメッセージだけを直し、設計書 3.2 節「コミットのメッセージ」に挙げた項目を書く（AC-06）。直した後、`git diff HEAD@{1} HEAD` が空であること（ファイルの内容が変わっていないこと）を確かめる。

**完了の条件:** テストを削除するコミットのメッセージに、比較のコマンド、node の版、親コミットの ID、`src/` のファイルの数・関数の数・テストの数の前後の値、1 以上から 0 になった関数がないことが書かれている。

#### ステップ 1-4: `_checks/` のスクリプトを作って実行する

**変更するファイル:** `docs/tasks/0054_remove_empty_metadata_result/_checks/removal_only.ts`（新規）、本書 7.1 節

- [ ] `_checks/removal_only.ts` を作る。スクリプトは 2 つのコミット（merge-base と PR の head）を引数に取り、どちらの内容も `git` から読む（作業ツリーは読まない）。次を確かめる。
    -   `extension/` の中で 2 つのコミットの間に変わったファイルが、`extension/src/core/metadata.ts` と `extension/test/metadata.test.ts` の 2 つだけである（AC-02・AC-03 の「ほかのファイルは変えない」）。
    -   merge-base の 2 つのファイルから、設計書 2.2 節の表の行（1.4 節の内容の行）を取り除いたものが、head の 2 つのファイルとバイト単位で一致する（AC-02・AC-03）。取り除く行は行番号ではなく内容で特定し、merge-base にその行の並びがちょうど 1 か所ないときは確かめられないとして止める。
    -   head の `extension/src` と `extension/test` の追跡されているファイルに、`emptyMetadataResult` という名前が現れない（AC-01）。
- [ ] 終了コードは、すべて成り立てば `0`、成り立たないものがあれば `1`（違いを示す）、引数やリポジトリが使えないときは `2` とする。ファイルの冒頭のコメントに、確かめる内容、実行の仕方、終了コードを英語で書く。
- [ ] スクリプトが成り立たない場合を見分けることを確かめる。少なくとも、(a) head に merge-base を渡した場合（削除がない）、(b) 削除に加えて `extension/` のほかの 1 行を変えた一時的なコミットを head に渡した場合、(c) コメントにだけ `emptyMetadataResult` を残した一時的なコミットを head に渡した場合に、終了コードが `1` になることを確かめる。一時的なコミットはブランチに残さない。
- [ ] スクリプトをコミットし、PR のブランチで merge-base（`git merge-base origin/main HEAD`）と `HEAD` を渡して実行する。出力、終了コード、2 つのコミット ID、負の場合 (a)〜(c) の結果を本書 7.1 節に書いてコミットする。

**完了の条件:** スクリプトが PR のブランチで終了コード `0` を返し、負の場合 (a)〜(c) で `1` を返したことが 7.1 節に記録されている。

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 完了の判定 |
|---|---|---|
| M1: 削除と確かめ | `emptyMetadataResult` とそのテストの削除、比較の結果を書いたコミットのメッセージ、`_checks/removal_only.ts` とその実行の記録 | PR のマージ |

ステップの順序は設計書 8 章のとおりである。ステップ 1-1 は 8 章の 1、1-2 は 2・3、1-3 は 4〜6、1-4 は 7 に当たる。

### 3.2. PR 構成

フェーズは 1 つなので、PR の作成ポイントの印は置かない。`/runplan` がフェーズ 1 を終えた時点で 1 つの PR を出す。

## 4. テスト戦略 (Test Strategy)

### 4.1. 単体テスト

テストは加えない。`describe("emptyMetadataResult")` を削除し、残るテストは変えない（設計書 7.1 節）。テストを削除しても、ほかの `src/` の関数の確認が失われていないことは、ステップ 1-3 のカバレッジの比較で確かめる（AC-05）。

### 4.2. 統合テスト

統合テストは加えない。`make ext-check` の成功を確かめる（AC-04、設計書 7.2 節）。export を削除するので、型の検査とビルドで、ほかに参照する場所が残っていないことも確かめられる。

### 4.3. 後方互換のテスト

拡張の振る舞いを変えないこと（要件書 4.4）は、`acceptMetadata`・`readFailureResult` を含む既存のテストが `make ext-check` でそのまま通ること、および `_checks/removal_only.ts` が `extension/src` の変更が削除だけであることを確かめることで確かめる。

### 4.4. セキュリティのテスト

N/A（設計書 5 章、7.3 節）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| `extension/node_modules` がなく、`make ext-install` にネットワークが要る | 実装の始めに利用者の承認を得る（ステップ 1-1）。前と後の報告は同じ `node_modules` で取る |
| カレントディレクトリやテストの対象の指定を誤り、報告が空のまま比較が通る | 設計書 3.2 節「測れていることの確認」の 3 項目を、比較の前に確かめる（ステップ 1-3） |
| 名前のない関数の `anonymous_<番号>` が実行ごとに変わり、関数の対応を誤る | 関数を (ファイル, `FN` の行番号) で識別し、行番号の集合の一致を先に確かめる（設計書 3.2 節） |
| `git commit --amend` でファイルの内容まで変わり、測った内容とコミットした内容が食い違う | amend の前に作業ツリーとインデックスに変更がないことを確かめ、amend の後に `git diff HEAD@{1} HEAD` が空であることを確かめる（ステップ 1-3） |
| `_checks/` のスクリプトが、どんな差分でも成功を返す | 負の場合 (a)〜(c) で終了コード `1` を返すことを確かめ、記録する（ステップ 1-4） |
| `/runpr` の中の `/fixpr` が、スクリプトの実行の後に `extension/` を変える commit を加える | 7.1 節の記録は、実行した時点の head までを確かめたことを示す。それより後に `extension/` を変える commit があれば、スクリプトを実行し直して記録を改める |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 完了（ステップ 1-1 / 1-2 / 1-3 / 1-4）
- [ ] PR マージ済み

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

種別は `test`（実行するテストや `make` の対象）、`static`（コミットしたスクリプトによる検索）、`diff`（`_checks/` のスクリプト、または設計書 3.2 節の一度だけの比較）、`manual`（PR のレビューで読む）である。

| AC | 種別 | 確かめ | ステップ |
|---|---|---|---|
| AC-01 | static | `docs/tasks/0054_remove_empty_metadata_result/_checks/removal_only.ts` が、head の `extension/src`・`extension/test` の追跡されているファイルに `emptyMetadataResult` が現れないことを確かめる（実行の記録は 7.1 節） | 1-2・1-4 |
| AC-02 | diff | `docs/tasks/0054_remove_empty_metadata_result/_checks/removal_only.ts` を merge-base と PR の head で実行する（7.1 節） | 1-2・1-4 |
| AC-03 | diff | 同上 | 1-2・1-4 |
| AC-04 | test | `make ext-check` | 1-2 |
| AC-05 | diff | 設計書 3.2 節の比較を、テストを削除するコミットの親とそのコミットで一度行う。`_checks/` のスクリプトにしない理由は設計書 3.2 節「残さないもの」。結果はそのコミットのメッセージにある（7.1 節にコミット ID を記録する） | 1-1・1-3 |
| AC-06 | diff・manual | AC-05 の比較の結果をテストを削除するコミットのメッセージに書く。PR のレビューで、そのメッセージに設計書 3.2 節「コミットのメッセージ」の項目がそろっていることを読む | 1-3 |

### 7.1. 実行の記録

（ステップ 1-3 のテストを削除するコミットの ID と、ステップ 1-4 のスクリプトの実行の結果・負の場合の結果を記録する。）

## 8. 成功基準 (Success Criteria)

-   **機能の完成度:** `emptyMetadataResult` とそのテストが削除され、7 章のすべての行の確かめが済んでいる。
-   **品質:** `make ext-check` と、グリーンゲート（`make test && make lint`）が通る。削除の前に実行回数が 1 以上だった `src/` の関数で、削除の後に 0 になったものがない。
-   **セキュリティ:** N/A（設計書 5 章）。
-   **文書:** 変える文書は本書だけである。過去のタスク文書（0025・0046）は変えない（要件書 2.3）。

## 9. 次のステップ (Next Steps)

-   PR のマージの後、`/runpr` が issue #560 を閉じる案内をする（本タスクの最後の PR のため）。
-   ほかの使われていない export の要否は、それぞれのタスクで決める（要件書 2.3）。
