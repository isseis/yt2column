# 実装計画書：文書だけの変更でも CI で文書の同期 guard を走らせる

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-14 |
| Review date | 2026-10-14 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下「requirements」）の項番、H-NN は [design_handoff.md](design_handoff.md) の項目、節番号は [02_architecture.md](02_architecture.md)（以下「architecture」）の節を指す。既存のファイルの行番号は、断りがない限り HEAD `d7362ca`（ブランチ `issei/doc-check-on-ci-01`）で確かめた。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

requirements が定める 4 つの機能要件（F-001〜F-004）を、architecture の設計どおりに実装する。`ci.yml` に、PR ごとに必ず走る `doc-guard` ジョブを追加し、文書の同期 guard（`cmd/yt2column`・`internal/pipeline`）を `go test -tags test ./cmd/yt2column ./internal/pipeline` で実行する。`check-changes` の分類と既存ジョブは変えない。`CLAUDE.md` の規則と `0012` の `02_architecture.md` §7.4 を更新し、最後に、文書だけの PR でジョブが走ることと、guard を壊すと赤になることを捨てコミットの PR で確認し、その結果を §5.1 に記録する。

本計画のフェーズ 1〜3 は、architecture §8 の実装優先順位のフェーズ 1〜3 と同じ名前・同じ順序である。用語（文書の同期 guard、`doc-guard` ジョブ、常時実行ジョブ、docs-only の変更、`check-changes`、壊し確認）は requirements §6 に従う。

### 1.2. 実装原則

- architecture §1.1 の設計原則 4 項目に従う。特に、経路を 1 つ足すだけで `check-changes`・既存ジョブを変えないこと（原則 1）、対象パッケージを明示列挙し `-run` を付けないこと（原則 2）、secrets を扱わず権限を増やさないこと（原則 3）。
- 新設・変更するファイルは architecture §2.2 の責務表だけである。Go のソースコード、`Makefile`、`extension/` は変更しない。
- **Go のソースコードを変更しないので、`make fmt`・`make test`・`make lint` の対象は変わらない。** 各フェーズの完了時に `make test` と `make lint` が通ることを確認する（変更が無くても回帰の確認として実行する）。
- 捨てコミットは `main` にマージしない。検証用の PR は、確認の後に閉じる（マージしない）。記録には workflow の実行 URL を含める。
- `AC-NN`・`F-NNN`・`H-NN` は Go ソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。本タスクは Go ソースを書かないので、この制約に触れる箇所はない。

### 1.3. 既存コード調査結果

- **`.github/workflows/ci.yml`（フェーズ 1 で変える）。** `pull_request`（`main` 向け）で起動する。workflow レベルの `permissions: contents: read` を持つ（`:7-8`）。ジョブは 5 つ。`check-changes`（`:12-53`。分類の grep は `:36`、拡張の分類は `:50-51` が `extension/scripts/has-extension-changes.sh` を呼ぶ）、`test`（`:55-74`。`needs: check-changes` と `if: has-code-changes == 'true'`）、`lint`（`:76-113`。同じ条件）、`extension`（`:115-165`。`if: has-extension-changes == 'true'`）、`secret-scan`（`:169-184`。`needs` も `if` も持たず常時実行）。`doc-guard` は `secret-scan` と同じく `needs`・`if` を持たないジョブとして `secret-scan` の近くに足す。`actions/checkout@v5`・`actions/setup-go@v6`（`go-version-file: go.mod`、`cache: true`）を再利用する（`:61-68`）。
- **`extension/scripts/has-extension-changes.sh`（変更しない）。** `doc-guard` はこれを呼ばない。`.github/workflows/ci.yml` はこのスクリプトのトリガ一覧に含まれる（`extension/test/ciChanges.test.ts:25`）が、`doc-guard` は拡張のジョブではないので、この一覧に影響しない。
- **`extension/test/ciChanges.test.ts`（変更しない）。** `has-extension-changes.sh` の入出力を検査する（`:8-16` が script を、`:18-69` がケースを持つ）。`doc-guard` はこのスクリプトを変えないので、このテストは変わらず通る（AC-03 の検証に使う）。
- **`CLAUDE.md`（フェーズ 2 で変える）。** 「Development Notes」は `:205-226`。変える規則は `:211-215`（文書だけの変更は `make ext-test` を要求しない、移した guard は `make test` に含まれ node を必要としない）。`:209`（編集の後に `make test` と `make lint` を実行する）と `:216-218`（`.gitignore` だけの変更は `make ext-test` を要求する）は維持する。
- **`docs/tasks/0012_doc_sync_guards/02_architecture.md`（フェーズ 2 で変える）。** §7.4「CI の実行範囲（H-03）」は `:290-294`。`:292` が「本タスクはこの分類を変えない」、`:294` が「残るリスク（明示）」を述べる。`:306`（§9 の将来の拡張性）と `:326`（§11 の決定の履歴）も「文書だけの変更を CI で守る案は別のタスク（#181）」と記しており、本タスクがその #181 である。`0012` は承認済み・完了済みなので、編集の区分を `requirements_process.md` §0 に従って判断する（§1.5）。
- **文書の guard（変更しない）。** `cmd/yt2column/docs_test.go`（`:21-26` のパス定数。`//go:build test`）、`cmd/yt2column/extension_docs_test.go`（`:22-25`。`//go:build test`）、`internal/pipeline/pipeline_test.go`（`:646` の `package_reference.md`、`:696` の `prompts/README.md`。`//go:build test`）。本タスクはこれらを実行するだけである。
- **`Makefile`（変更しない）。** `test:`（`:68-69`）は `go test -tags test -race -v ./...`。`test-ci`（`:71` 以降）は CI 用に coverage を付ける。`doc-guard` は `make` を呼ばず `go test` を直接実行する（H-01。architecture §10）。
- **外部前提の確認。** `doc-guard` が使う `actions/checkout@v5`・`actions/setup-go@v6`・`go-version-file: go.mod`・`cache: true` は、既存の `test`・`lint` ジョブが同じ形で使っている（`ci.yml:61-68`・`:83-89`）。`go test -tags test ./cmd/yt2column ./internal/pipeline` が手元で通ることは実測済み（約 14 秒。requirements §4.1）。`-tags test` だけで対象の文書 guard（`//go:build test`）がビルドされることも実測で確かめた。新しい外部モジュールは追加しない。
- **意図して変えないもの。** `check-changes` の分類、`test`・`lint`・`extension`・`secret-scan` の各ジョブ、`extension/scripts/has-extension-changes.sh`、`extension/test/ciChanges.test.ts` ほかの拡張テスト、`Makefile`、Go のソース、`.gitignore`、文書の guard が読む文書（`README.md`・`docs/dev/security.md`・`docs/dev/project_overview.md`・`prompts/README.md`・`package_reference.md`・タスク `0007`/`0008`/`0010` の文書）。

### 1.4. 名前の変更の一覧

本タスクはシンボル・ファイル・テストの名前を変更しない。名前の変更の一覧は該当なし。

### 1.5. 申し送り（design_handoff）への対応

`implementation_handoff.md` はない。`design_handoff.md` の H-NN はすべて architecture §10 が方針を記録しており、本計画は architecture のとおりに実装する。

| 項目 | 本計画での扱い |
|---|---|
| H-01（コマンドの置き場） | フェーズ 1 で `ci.yml` の `doc-guard` ジョブに `go test -tags test ./cmd/yt2column ./internal/pipeline` を直接書く（`Makefile` にターゲットを足さない） |
| H-02（対象パッケージの範囲） | フェーズ 1 で 2 パッケージを列挙する。architecture §3.2 の `rg` の結果を前提とする |
| H-03（`-tags` の選択） | フェーズ 1 で `-tags test` のみを使う（`integration` を付けない） |
| H-04（ジョブの権限と実行条件の明示） | フェーズ 1 で `needs`・`if`・追加の `permissions` を付けない（`secret-scan` と同じ形） |

### 1.6. `0012` の文書更新の区分

`0012` の `02_architecture.md` §7.4 は、本タスク（#181）で文書だけの変更を CI が守るようになることを受けて更新する。§7.4 は「本タスク（`0012`）は分類を変えない」「文書だけの変更を CI で守る案は別のタスク（#181）」と述べており、本タスクはその続きを実装する。したがってこれは、`0012` が「何を作るか」を変える**決定変更ではなく**、`0012` の記述が指した後続タスクが完了したことによる**記述の更新**である。編集の際は、`0012` の §7.4 に「#181（タスク `0014`）で `doc-guard` ジョブが追加され、文書だけの変更を CI が守るようになった」旨と、残った限界（対象パッケージの明示列挙）を追記する。状態は `approved` のまま保ち、`0012` の `02_architecture.md` の `Comments` に、決定を変えない記述の更新である旨を記す。判断に迷う場合は `requirements_process.md` §0 に従い再承認を求める。

## 2. 実装ステップ (Implementation Steps)

### フェーズ 1: `doc-guard` ジョブ（F-001）

**対象ファイル:** `.github/workflows/ci.yml`（変更）

- [ ] **ステップ 1-1**: `.github/workflows/ci.yml` に `doc-guard` ジョブを追加する。`secret-scan` と同じく `needs` も `if` も付けず、ジョブに `permissions` を付けない。ステップは `actions/checkout@v5` → `actions/setup-go@v6`（`go-version-file: go.mod`、`cache: true`）→ `go test -tags test ./cmd/yt2column ./internal/pipeline` の 3 つ。`-run` を付けない（architecture §3.1）。
- [ ] **ステップ 1-2**: `check-changes`・`test`・`lint`・`extension`・`secret-scan` の各ジョブを変更していないことを `git diff .github/workflows/ci.yml` で確かめる。`check-changes` の分類ロジック（`:24-53`）に差分がないこと、追加が `doc-guard` ジョブだけであることを確認する。
- [ ] **ステップ 1-3**: フェーズ 1 の完了条件として、`make test` と `make lint` を通す（Go のソースは変えていないので、回帰の確認）。`doc-guard` が実行する `go test -tags test ./cmd/yt2column ./internal/pipeline` が手元で通ることを確かめる。

**完了条件:** `ci.yml` に `doc-guard` ジョブがあり、`check-changes` と既存ジョブに差分がない。`go test -tags test ./cmd/yt2column ./internal/pipeline` が手元で通る。

### フェーズ 2: 文書の更新（F-002・F-003）

**対象ファイル:** `CLAUDE.md`（変更）、`docs/tasks/0012_doc_sync_guards/02_architecture.md`（変更）

- [ ] **ステップ 2-1**: `CLAUDE.md` の「Development Notes」の文書だけの変更に関する規則（`:211-215`）に、文書だけの変更でも CI の `doc-guard` ジョブが文書の guard を実行することを追記する。現行の趣旨（文書だけの変更は `make ext-test` を要求しない、移した guard は `make test` に含まれ node を必要としない）は維持する（AC-04）。
- [ ] **ステップ 2-2**: `0012` の `02_architecture.md` §7.4 を §1.6 の方針で更新する。`#181`（タスク `0014`）で `doc-guard` ジョブが追加されたことと、残った限界（対象パッケージを明示列挙していること）を記す。`0012` の `02_architecture.md` の `Comments` に、決定を変えない記述の更新である旨を記す（AC-05）。

**完了条件:** `CLAUDE.md` と `0012` §7.4 が architecture §3.3 の内容を満たす。`make test` と `make lint` が通る。

### フェーズ 3: 検証と記録（F-004）

**対象ファイル:** `docs/tasks/0014_doc_guard_ci/03_implementation_plan.md`（§5.1 に記録）

- [ ] **ステップ 3-1**: フェーズ 1 が `main` にマージされた後、`main` から捨てブランチを切り、文書だけを変える（例: `CLAUDE.md` に無害な 1 行を足す）。PR を出し、`doc-guard` ジョブが走って成功すること（`check-changes` が `has-code-changes=false` でも走ること）を確かめる。確認の後、PR を閉じる（マージしない）。workflow の実行 URL を控える（AC-06）。
- [ ] **ステップ 3-2**: `main` から捨てブランチを切り、(a) `cmd/yt2column` の文書 guard が読むテキストを壊す（例: `CLAUDE.md` の `ext-` ターゲットの一覧から 1 つ削る）、(b) `internal/pipeline` の文書 guard が読むテキストを壊す（例: `docs/dev/developer_guide/package_reference.md` からパッケージの行を 1 つ削る）を別々に行う。(a)・(b) それぞれの PR で `doc-guard` ジョブが赤になることを確かめる。これは、`doc-guard` が `cmd/yt2column` と `internal/pipeline` の両方を対象にし、`-run` で絞っていないことの確認でもある（AC-07・AC-02）。確認の後、PR を閉じ、`main` に戻す。
- [ ] **ステップ 3-3**: ステップ 3-1・3-2 の結果（対象・実行 URL・日付・結果）を §5.1 に記録する。記録の欄は空にしない。

**完了条件:** §5.1 に AC-06・AC-07 の記録がそろっている。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

| マイルストーン | 成果物 | 対応するフェーズ |
|---|---|---|
| M1: `doc-guard` ジョブが CI で走る | `ci.yml` の `doc-guard` ジョブ（マージ済み） | フェーズ 1 |
| M2: 文書が CI の新しい実行範囲を反映する | `CLAUDE.md`・`0012` §7.4 の更新（マージ済み） | フェーズ 2 |
| M3: 挙動が確認され記録される | §5.1 の記録（捨て PR は閉じる） | フェーズ 3 |

フェーズ 3 はフェーズ 1 の `doc-guard` が `main` にあることを前提とする。フェーズ 1 とフェーズ 2 は同じ PR にまとめてよい（フェーズ 2 はフェーズ 1 に依存する）。PR の境界は `/mkplan2` で埋め込む。

## 4. テスト戦略 (Test Strategy)

### 4.1. 静的 guard（CI の設定）

`doc-guard` ジョブは、それ自体が「PR ごとに文書 guard を実行する」という CI の契約を固定する。Go のテストでこの契約を固定しない（`ci.yml` を読む Go のテストは既存にない）。`ci.yml` の形（`needs`・`if` を持たないこと、ステップの順、`-run` を付けないこと）は、フェーズ 1 の `git diff` と、フェーズ 3 の捨て PR の挙動で確かめる。

### 4.2. 既存テストの回帰

`check-changes`・`has-extension-changes.sh`・Go のソースを変えないので、`make test`・`make lint`・`make ext-test`（`ciChanges.test.ts` を含む）の対象と期待値は変わらない。各フェーズの完了時に `make test` と `make lint` を通す。`make ext-test` はフェーズ 3 の前に一度通し、`ciChanges.test.ts` が変わらず通ることを確かめる（AC-03）。

### 4.3. 統合テスト（CI の挙動）

GitHub Actions は手元で再現できないので、捨てコミットの PR で確認する（フェーズ 3。architecture §7.2）。文書だけの PR でジョブが走ること（AC-06）と、guard を壊すと赤になること（AC-07）を確かめる。捨て PR は `main` にマージしない。

### 4.4. テストヘルパー

新しいテストファイル・テストヘルパを追加しない（`test_organization.md` の対象外）。`internal/pipeline/pipeline_test.go` の `TestFakesCarryBuildTag` が固定するファイル数は変わらない（`testutil/`・`test_helpers*.go` を追加しない）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 検証の区分 | 検証 |
|---|---|---|
| AC-01 | static / manual | `ci.yml` の `doc-guard` ジョブに `needs`・`if` がないこと（static。ステップ 1-2 の `git diff` とファイルの確認）。文書だけの PR で `check-changes` が `has-code-changes=false` でも `doc-guard` が走ること（manual。ステップ 3-1 の捨て PR と §5.1 の記録） |
| AC-02 | static / manual | `ci.yml` の `doc-guard` のステップが checkout → setup-go → `go test -tags test ./cmd/yt2column ./internal/pipeline` で、`-run` がないこと（static。ステップ 1-1・1-2）。`cmd/yt2column` の guard を壊した PR と `internal/pipeline` の guard を壊した PR が、どちらも `doc-guard` を赤にすること（manual。ステップ 3-2。両パッケージが対象で、`-run` で絞っていないことの確認） |
| AC-03 | static / test | `check-changes` の分類ロジックと `has-extension-changes.sh` に差分がないこと（static。ステップ 1-2 の `git diff`）。`extension/test/ciChanges.test.ts::has-extension-changes` が変わらず通ること（test。`make ext-test`） |
| AC-04 | static | `CLAUDE.md`「Development Notes」の該当規則が、文書だけの変更でも CI の `doc-guard` が文書の guard を実行することと、`make ext-test` を要求しないことを述べていること（ステップ 2-1。`requirements_process.md` §4 は文書の存在についての基準に `static` を認める） |
| AC-05 | static | `0012` の `02_architecture.md` §7.4 が、#181（タスク `0014`）で `doc-guard` が追加されたことを反映し、古い「CI はこの変更を守らない」が現在も有効な記述として残っていないこと（ステップ 2-2） |
| AC-06 | manual | 文書だけを変える捨て PR で `doc-guard` ジョブが走り成功すること（ステップ 3-1。§5.1 に実行 URL を記録） |
| AC-07 | manual | 文書の guard が読むテキストを壊す捨て PR で `doc-guard` ジョブが赤になること（ステップ 3-2。`cmd/yt2column` と `internal/pipeline` の両方。§5.1 に実行 URL を記録） |

### 5.1. 手動確認の記録

ステップ 3-1・3-2 の確認の手順と結果である。記録の欄（確認・捨て PR・日付・結果）は空にしない。

| 確認 | 捨て PR（branch / URL） | 日付 | 結果 |
|---|---|---|---|
| AC-06（文書だけの PR で `doc-guard` が走る） | （ステップ 3-1 で記入） | | |
| AC-07(a)（`cmd/yt2column` の guard を壊すと赤） | （ステップ 3-2 で記入） | | |
| AC-07(b)（`internal/pipeline` の guard を壊すと赤） | （ステップ 3-2 で記入） | | |

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| `doc-guard` ジョブの YAML が不正で、GitHub Actions が workflow を起動しない | ジョブが走らず、AC-01・AC-06 が満たされない | ステップ 1-1 で既存ジョブの形を写し、ステップ 3-1 の捨て PR で workflow が起動することを確かめる。YAML が不正なら workflow が起動しないので、捨て PR が最初の検出点になる |
| 文書だけの PR で `check-changes` が `doc-guard` の起動を妨げる（誤って `needs`・`if` を付ける） | AC-01・AC-06 が満たされない | ステップ 1-1 で `needs`・`if` を付けない。ステップ 3-1 で挙動を確認する |
| `-run` を付けてしまい、新しく足した guard が実行されない | AC-02 が満たされない | ステップ 1-1 で `-run` を付けない。ステップ 3-2 で両パッケージの guard を壊して赤になることを確かめる |
| `internal/pipeline` のテストが CI 環境で外部リソースを要求する | `doc-guard` が環境依存で落ちる | 対象の文書 guard はリポジトリのファイルを読むだけである（architecture §5.1）。手元の `go test -tags test ./cmd/yt2column ./internal/pipeline` が通ることを確かめ、`integration` タグを付けない（`-tags test` のみ） |
| 捨て PR を `main` にマージしてしまう | 壊した文書・検証用のコミットが入る | 捨て PR はステップ 3-1・3-2 の後に閉じる。`main` へ戻す。`mergepr` は使わない |
| ステップ 3 の実行に GitHub の権限・ネットワークが要る | 手元で完結できない | 実行できない場合は、その旨と理由を §5.1 に記録し、未確認であることを明示する（黙って通さない） |
| `0012` の §7.4 の更新が決定変更と見なされる | `0012` の再承認が必要になる | §1.6 のとおり、記述の更新として扱い、`Comments` にその旨を記す。曖昧な場合は再承認を求める |

## 7. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-3）完了
- [ ] フェーズ 2（ステップ 2-1〜2-2）完了
- [ ] フェーズ 3（ステップ 3-1〜3-3）完了
- [ ] §5 のすべての AC の検証が通り、§5.1 の記録がそろっている
- [ ] `make test`・`make lint`・`make ext-test` が通る

## 8. 成功基準 (Success Criteria)

- **機能:** `ci.yml` に常時実行の `doc-guard` ジョブがあり、文書だけの PR で走り（AC-01・AC-06）、文書の guard を壊すと赤になる（AC-07）。`check-changes` の分類と既存ジョブは変わらない（AC-03）。
- **品質:** `make test`・`make lint`・`make ext-test` が通り、既存のテストへの影響がない。§5.1 の手動確認の記録がそろっている。
- **セキュリティ:** `doc-guard` は secrets を扱わず、workflow レベルの `permissions: contents: read` を維持する。新しい外部モジュール・外部コマンドを追加しない。
- **文書:** `CLAUDE.md` と `0012` §7.4 が architecture §3.3 の内容を満たす（AC-04・AC-05）。

## 9. 次のステップ (Next Steps)

- PR の境界は `/mkplan2` で埋め込む。`/runplan 0014` で実装する。
- branch protection で必須チェックを設定している場合は、`doc-guard` を必須チェックに登録する人手の作業を行う（requirements 制約。リポジトリ内のファイルでは完結しない）。
- 別パッケージに文書 guard を足す場合は、`doc-guard` の `go test` の引数にそのパッケージを足す（architecture §9）。
