# 要件定義書：文書の同期 guard を node から Go へ移す

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-13 |
| Review date | 2026-10-13 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、ブラウザ拡張の node のテスト（`extension/test/docs.test.ts`）が行っている文書の**テキスト同期**の guard を Go のテストへ移し、文書（`README.md`・`CLAUDE.md`・`docs/`）と `.gitignore` だけを変える変更で node のインストールを要求しないようにするための要件を定義する。

**背景:**

- `CLAUDE.md:206-210` は、文書だけでなく `.gitignore` だけを変えた変更でも、コミットの前に `make ext-test` をローカルで実行するよう求めている。理由は、文書のみの変更は拡張の CI のジョブを走らせないが、拡張の guard テストが文書を読んでいて壊れ得るためである。
- `make ext-test` は `extension/.node-version` に固定した node と `make ext-install`（`npm ci`、ネットワークが必要）を前提とする（`Makefile` の `ext_preflight`）。文書しか変えていない変更で node のインストールを要求するのは費用対効果が悪い。
- `extension/test/docs.test.ts` が検査しているのは、いずれもテキストの同期であり node を必要としない（拡張 ID、`ext-` ターゲットの一覧、`security.md` の permission、`project_overview.md` の記述、タスク `0007` の記録）。
- 同じ種類の guard は既に Go 側にもある。`cmd/yt2column/docs_test.go` が README の設定表・終了コード表・`project_overview.md` の設定表などを `make test` で検査しており、Go のテストは node を要求しない。
- `docs.test.ts` はタスク `0007_browser_extension_skeleton` のステップ 5-6 で作られ、各検査が壊して落ちることはステップ 5-8 で確認されている。

対応 issue: [#180](https://github.com/isseis/yt2column/issues/180)（label: `devenv`）

要件レビュー（と起草）の過程で挙がった、設計で決める事項は [design_handoff.md](design_handoff.md) に申し送る。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   拡張の node のテストのうち、文書のテキスト同期の guard を Go のテストへ移す（F-001）。移した検査は `make test` の一部となり、node を起動しない。
-   移した検査を node 側から削除し、同じ検査を二重に持たない（F-002）。
-   文書のみの変更で node（`make ext-test`）を要求しないよう、`CLAUDE.md` の規則を見直す（F-003）。
-   移した各 guard が、対象を壊すと落ちることを確認する（F-004、`CLAUDE.md` の testing strategy）。

### 2.2. スコープ (In Scope)

-   現行 `extension/test/docs.test.ts` が行う 6 種類の検査の Go への実装（F-001。配置は設計で決める。[design_handoff.md](design_handoff.md) H-01・H-02）。
-   拡張の node テストの整理（`extension/test/docs.test.ts` の削除と、未使用になったヘルパ・import の始末。F-002）。
-   `.gitignore` のみの変更の扱い（node の ignore 規則の guard を残すか Go へ移すか）の決定と、`CLAUDE.md` への反映（F-003）。
-   `CLAUDE.md` の「Development Notes」の該当規則の見直し（F-003）。
-   移した guard の壊し確認と、その結果の記録（F-004）。

### 2.3. スコープ外 (Out of Scope)

-   拡張の**挙動**のテスト（`manifest`・`checkDist`・`typecheck`・`build`・`lint`・`repository` などの各テスト）の変更。これらは node 側に残す（`CLAUDE.md` の「Extension Commands」）。
-   移した guard が読む文書の内容の変更。本タスクは現行の記載（拡張 ID、permission の列挙、`TypeScript`・`extension/` の記述、タスク `0007` の記録）を固定するだけであり、値そのものは変えない。`CLAUDE.md` の規則の見直し（F-003）だけを例外的に変更する。
-   タスク `0007` の承認済み文書（`02_architecture.md`・`03_implementation_plan.md`）の書き換え。`docs.test.ts` を指す記述は履歴であり、必要なら注記するに留める（[design_handoff.md](design_handoff.md) H-04）。
-   CI（`.github/workflows/ci.yml`）のジョブの分類・トリガの変更。文書のみの変更で Go のテストのジョブも走らない現状は、本タスクでは変えない（[design_handoff.md](design_handoff.md) H-03）。
-   新しい外部モジュールの追加。移した guard は Go の標準ライブラリだけで実装する。
-   上記以外の文書（`README.md`・`docs/dev/security.md`・`docs/dev/project_overview.md` など）の記載の変更。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 文書の同期 guard の Go への移設

現行 `extension/test/docs.test.ts` が行う次の 6 種類の検査を、node を必要としない Go のテストとして実装し、`make test` で実行する。

1.  **拡張 ID の記載:** `README.md` と `docs/dev/security.md` に、`extension/static/manifest.json` の `key` から導出した固定の拡張 ID が含まれる。拡張 ID は文書側に固定値として書かず、manifest の `key` から導出する（現行 `docs.test.ts` の `extensionIdFromManifestKey` と同じ計算）。
2.  **`ext-` ターゲットの一覧:** `Makefile` が `ext-<名前>:` の形で定義する `ext-` ターゲットの集合と、`CLAUDE.md` が `- \`make ext-<名前>\`` の形で列挙する `ext-` ターゲットの集合が一致する（重複を除いて比較する。どちらか片方だけにあるターゲットがあれば落ちる）。
3.  **manifest の permission:** `docs/dev/security.md` の `### 8.1. ブラウザ拡張（#110）` の節が、`extension/static/manifest.json` の `permissions` の各値を、バッククォートで囲んだ形で列挙する。
4.  **project overview の記述:** `docs/dev/project_overview.md` の `## 決定済みの方針` の節に `TypeScript` があり、`## 想定ディレクトリ構成` の節に `extension/` がある。
5.  **タスク 0007 の調査の記録:** `docs/tasks/0007_browser_extension_skeleton/02_architecture.md` に `#### 3.12.1.`・`#### 3.12.2.`・`#### 3.12.3.` の見出しがあり、`3.12.1` の本文に拡張 ID があり、`3.12.2`・`3.12.3` の見出しの直後の本文が空でない。
6.  **タスク 0007 の手動確認の記録:** `docs/tasks/0007_browser_extension_skeleton/03_implementation_plan.md` の `### 5.1. 手動の確認と記録` の表が、定められた AC の行（`AC-02`・`AC-03`・`AC-05`・`AC-07`・`AC-08`・`AC-10・AC-11`・`AC-13`・`AC-20`・`AC-21・AC-22・AC-31`・`AC-28・AC-29`・`AC-25`）をこの順で持ち、各行が 5 列で、確認内容・ブラウザか環境・日付・結果の列（2〜4 列目）が空でない。

**Acceptance Criteria**:
- **AC-01**: 上記 1〜6 の各検査が、Go のテストとして実装され、`make test`（`go test -tags test ./...`）で実行される。検査は node も npm も起動しない。
- **AC-02**: 検査 1・5 は、拡張 ID を manifest の `key` から導出した値で照合する。文書側の拡張 ID、または manifest の `key` を変えて導出値と一致しなくなると、検査が落ちる。
- **AC-03**: 検査 2 は、`Makefile` の `ext-` ターゲットを増減する、または `CLAUDE.md` の一覧を増減して両者が一致しなくなると落ちる。
- **AC-04**: 検査 3 は、`extension/static/manifest.json` の permission を増減する、または `security.md` の §8.1 から permission の記載を消すと落ちる。
- **AC-05**: 検査 4 は、`project_overview.md` の `## 決定済みの方針` から `TypeScript` を、または `## 想定ディレクトリ構成` から `extension/` を消すと落ちる。
- **AC-06**: 検査 5 は、`0007` の `02_architecture.md` の `3.12.x` の見出しを消すか、`3.12.2`・`3.12.3` の本文を空にすると落ちる。
- **AC-07**: 検査 6 は、`0007` の `03_implementation_plan.md` の §5.1 の行を消すか、結果などの列のセルを空にすると落ちる。

#### F-002: 拡張の node テストからの削除と整理

-   `extension/test/docs.test.ts` を削除する。移した 6 種類の検査と同じ内容を、node 側のテストに残さない（重複を避ける）。
-   拡張に残す挙動のテスト（`manifest.test.ts`・`checkDist.test.ts`・`typecheck.test.ts`・`lintRules.test.ts`・`repository.test.ts`・`render.test.ts`・`messages.test.ts`・`collect.test.ts`・`summary.test.ts`・`acceptedUrl.test.ts`・`launch.test.ts`・`ciChanges.test.ts`・`chromeDeps.test.ts`・`checkLockfile.test.ts`）は、その役割を変えずに残す。
-   `docs.test.ts` だけが使っていたヘルパ・import は削除し、他のテストが使うもの（`extensionId.ts`・`repoFile.ts`・`paths.ts` など）は残す。判断は設計で行う（[design_handoff.md](design_handoff.md) H-06）。

**Acceptance Criteria**:
- **AC-08**: `extension/test/` に、移した 6 種類の検査を行う node のテストが残っていない（`docs.test.ts` が削除され、同じ検査が `make ext-test` に含まれない）。
- **AC-09**: 拡張に残る node のテストが `make ext-check`（typecheck・lint・fmt-check・test・build）で通る。未使用になったヘルパ・import・文字列リテラルが残っていない（`make ext-lint`・`make ext-typecheck` が通る）。

#### F-003: 文書のみの変更で node を要求しない

-   `CLAUDE.md:206-210` の規則を、文書（`README.md`・`CLAUDE.md`・`docs/`）だけを変える変更で `make ext-test` を要求しない形に見直す。移した検査は Go のテストとして `make test` に含まれ、node を必要としない。
-   `.gitignore` のみの変更は、文書のテキスト同期 guard では守られない。現行の ignore 規則の guard（`extension/test/repository.test.ts` の `git check-ignore`）は node 側にあり、`.gitignore` を変えると落ち得る。`.gitignore` のみの変更で node を要求するかは、ignore 規則の guard を Go へ移すかで決まる。どちらに寄せるかは設計で決める（[design_handoff.md](design_handoff.md) H-07）。決めた方針は `CLAUDE.md` の規則に明記する。
-   `CLAUDE.md:204` の「編集の後に `make test` と `make lint` を実行する」規則は維持する。

**Acceptance Criteria**:
- **AC-10**: `CLAUDE.md` の「Development Notes」の規則が、文書（`README.md`・`CLAUDE.md`・`docs/`）だけの変更で `make ext-test` を要求しない。移した検査が `make test`（node 不要）で実行されることが分かる。`.gitignore` のみの変更の扱い（`make ext-test` を要求するか、ignore 規則の guard も `make test` に含めるか）が明記されている（静的確認）。
- **AC-11**: 文書のテキスト同期の guard が `make test` の対象に含まれ、`make test` のレシピと Go のテストは node・npm を起動しない（`Makefile` の `test:` レシピに node・npm が現れず、`repository.test.ts` の「go targets do not need node」の確認と整合する）。
- **AC-12**: `.gitignore` のみの変更で、ignore 規則の guard（`git check-ignore` で `extension/node_modules`・`extension/dist`・`*.pem` を検査する）が、設計で決めた方針（node の `make ext-test` か、Go の `make test` か）で実行され、`CLAUDE.md` の記載と一致する。

#### F-004: 移した guard が壊して落ちることの確認

-   移した 6 種類の各検査について、対象（文書・`Makefile`・manifest）を壊すと落ちることを確認する。対象は、`CLAUDE.md` の testing strategy が求める「テストが主張する対象を無効にして落ちることを確かめる」手順に従う。
-   確認は、対象を壊す、Go のテストを実行してその検査だけが落ちることを見る、壊した変更を戻す、の順で行う。
-   確認の手順と結果を `03_implementation_plan.md` の手動確認の節に記録する（タスク `0007` の §5.1 と同じ形式）。

**Acceptance Criteria**:
- **AC-13**: 移した 6 種類の各検査について、対象を壊すと落ちることを確認し、その手順と結果が `03_implementation_plan.md` の手動確認の節に記録されている（静的確認。記録の形式はタスク `0007` の §5.1 に倣う）。

### 3.2. 信頼できない入力の境界 (Untrusted Input Boundaries)

本タスクの guard が読む入力は、リポジトリにコミットされたファイル（`README.md`・`CLAUDE.md`・`docs/` の各文書・`Makefile`・`extension/static/manifest.json`）だけであり、いずれも信頼できる入力である。ネットワーク・外部コマンド・利用者が実行時に与える入力は扱わない。したがって、本タスクに信頼できない入力の境界はない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   移した検査は、リポジトリのファイルを読んで文字列と表を照合するだけである。ファイルの読み込みは 1 回にまとめ、テストの実行時間を大きく増やさない。

### 4.2. セキュリティ (Security)

-   移した検査は、コミットされたファイルだけを読む。秘密情報（API キー、Webhook URL、共有トークン）と、利用者のデータを扱わない。
-   拡張 ID は manifest の `key`（公開情報）から導出する。秘密鍵は扱わない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   移した検査は、対象のファイルが存在しない、見出しがない、表の形が違う場合に、落ちる（無言で通さない）。ファイルの読み込みの失敗はテストの失敗として報告する。

### 4.4. 互換性 (Compatibility)

-   移した検査は Go の標準ライブラリだけで実装し、`make test` で macOS と Linux（CI）のどちらでも動く。node を要求しない。
-   拡張 ID の導出は、現行の `extensionIdFromManifestKey`（SHA-256 の先頭 16 バイトを 16 進で表し、各桁 `0`〜`f` を `a`〜`p` に置き換える）と同じ値になる。

### 4.5. 保守性 (Maintainability)

-   文書のテキスト同期の guard を Go に集約し、拡張の node のテストは挙動の検査に絞る。拡張の挙動のテストは node 側に残す。
-   拡張 ID の導出など、複数の検査で使う処理は 1 か所にまとめる。
-   検査が読む見出し・表の形は、実装の詳細ではなく文書の契約として固定する。文書の構成を変える変更は、同じ変更で guard を更新する（`AGENTS.md` の「contracts are pinned」と同じ扱い）。
-   移した guard を配置する場所と、既存の `cmd/yt2column/docs_test.go` との役割の境界は、設計で決める（[design_handoff.md](design_handoff.md) H-01・H-02）。

## 5. 制約条件 (Constraints)

-   [project_overview.md](../../dev/project_overview.md) の「決定済みの方針」「前提・制約」に従う。外部ライブラリは追加しない（`CLAUDE.md`「Dependencies」）。
-   既存の `cmd/yt2column/docs_test.go` の検査（README の設定表・終了コード表、`project_overview.md` の設定表、`security.md` の Slack・Claude の記載、タスク `0005`・`0006`・`0009` の手動実行の記録）を壊さない。ヘルパ（Markdown の表の分解など）は重複を避けて再利用する（[design_handoff.md](design_handoff.md) H-02）。
-   移した guard が読む文書（`README.md`・`CLAUDE.md`・`docs/dev/security.md`・`docs/dev/project_overview.md`・タスク `0007` の `02_architecture.md`・`03_implementation_plan.md`）は現行の記載を維持する。変更するのは `CLAUDE.md` の規則（F-003）だけとする。
-   タスク `0007` は承認済みで完了している。`docs.test.ts` を指す記述（`03_implementation_plan.md` の 3 節・ステップ 5-6・5-8・§5.1 の説明、`02_architecture.md` の関連箇所）は履歴として残す。訂正する場合は、`requirements_process.md` §0 の「Editorial correction」に従い、`Comments` にその旨を記す（[design_handoff.md](design_handoff.md) H-04）。
-   各 guard は、`CLAUDE.md` の testing strategy に従い、対象を壊すと落ちることを確認する。確認の後に対象を戻し、`make test`・`make lint`（および `make ext-check`）を通す。
-   CI のジョブの分類・トリガ（`.github/workflows/ci.yml` の `check-changes`）は変更しない。文書のみの変更では拡張のジョブも Go のテストのジョブも走らない現状は、本タスクでは変えない。この点の扱いは [design_handoff.md](design_handoff.md) H-03 に記す。
-   `internal/pipeline/pipeline_test.go` の guard（`AGENTS.md` が説明する静的な契約の guard）と、`cmd/yt2column/docs_test.go` の役割の境界を保つ。本タスクは新しい fake や `testutil` を追加しないので、`TestFakesCarryBuildTag` のファイル数の固定は変えない。
-   実装は `requirements_process.md` の承認の流れ（`01_requirements.md` → `02_architecture.md` → `03_implementation_plan.md` の順に承認）に従う。

## 6. 用語集 (Glossary)

-   **文書の同期 guard:** 文書・`Makefile`・manifest の間で一致すべきテキスト（拡張 ID、`ext-` ターゲットの一覧、permission の列挙）を検査するテスト。
-   **テキスト同期の検査:** リポジトリのファイルを読み、文字列・見出し・表を照合するだけで、ブラウザ・npm・ネットワークを使わない検査。
-   **挙動のテスト:** manifest の妥当性、`dist/` の内容、型検査、lint、build など、拡張の実際の振る舞いを検査する node のテスト。
-   **docs-only の変更:** `README.md`・`CLAUDE.md`・`docs/` と `.gitignore` だけを変える変更。`ci.yml` の `check-changes` がコードの変更とみなさない。
-   **`ext-` ターゲット:** `Makefile` が定義する `ext-install`・`ext-typecheck`・`ext-lint`・`ext-fmt-check`・`ext-fmt`・`ext-test`・`ext-build`・`ext-check` の各ターゲット。
-   **拡張 ID:** manifest の `key`（公開鍵）から導出される Chrome の拡張 ID。値は `clfmbbcdpnjcefbdihdoahomaifbabkk`（タスク `0007` の `02_architecture.md` §3.12.1）。
-   **壊し確認:** テストが主張する対象を無効にして、そのテストが落ちることを確かめる手順（`CLAUDE.md` の testing strategy）。
-   用語は [translation_glossary.md](../../translation_glossary.md) と統一する。
