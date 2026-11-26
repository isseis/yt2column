# 実装計画書：利用者向け製品名（ヨムドウガ / YomuDoga）の適用

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-25 |
| Review date | 2026-11-26 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装概要

**目的:** [01_requirements.md](01_requirements.md)（以下、要件書）と [02_architecture.md](02_architecture.md)（以下、設計書）に基づき、ブラウザ拡張の表示名・説明文・UI 文言・テスト、Chrome ウェブストア詳細説明文、利用者向け文書、および関連文書を新製品名（`ヨムドウガ` / `YomuDoga`、`ヨムドウガ for YouTube™` / `YomuDoga for YouTube™`）へ更新する。

**実装原則:**

- 拡張側の更新（フェーズ 1 / PR-1）と文書側の更新（フェーズ 2 / PR-2）を独立してグリーンゲート（`_context.md` の "Green gate" 参照）を通過する 2 本の PR に分ける。
- 開発側の名称（リポジトリ名、Go モジュール、CLI、環境変数、拡張内部ログ、WASM ファイル名、パッケージ名、`development.md:119` を除く開発者向け文書）は変更しない。
- 各テスト更新後は壊し確認（対象を旧値に戻してテストが失敗することの確認）を行い、コミットメッセージに記録する（`CLAUDE.md`「Testing Strategy」）。

### 1.1. 既存コード調査結果

コミット `1a5b6cab`（`origin/main`）で全変更対象および影響を受けるテストを確認した。既存の実装・ビルド機構（`extension/scripts/build-variant.ts`）・ガードテストを再利用し、新規のテストヘルパーファイル（`testutil/` や `test_helpers.go`）は追加しない。

- **ロケールカタログ:** `extension/static/_locales/ja/messages.json:2-11` と `extension/static/_locales/en/messages.json:2-11` に `extName`・`extNameDev`・`extDescription` がある。`extension/static/manifest.json:3-4,7,16` はプレースホルダと公開鍵・権限を定義しており変更不要である。
- **静的 HTML:** `extension/static/popup.html:5`・`options.html:5`・`viewer.html:5` がいずれも `<title>yt2column</title>` を持つ。
- **UI カタログ:** `extension/src/core/i18n/ja.ts:27,35,279,289` と `extension/src/core/i18n/en.ts:29,37,268,278` に `yt2column` を含む 5 項目（4 行）がある。`extension/src/core/viewerTitle.ts`・`optionsApp.ts`・`launch.ts` はカタログを参照しておりロジック変更は不要である。
- **拡張テスト:**
  - `extension/test/manifest.test.ts:255-273` が両ロケールの `extName === "yt2column"` を検証している。
  - `extension/test/buildVariant.test.ts:73-77` が `extNameDev.startsWith("yt2column")` を検証している。
  - `extension/test/viewerTitle.test.ts:19-106` が `yt2column` と `extName` 完全一致を検証している。
  - `extension/test/viewerApp.test.ts:281,337,356,371,400,619,824,835` が英語ロケールで `... - yt2column` を検証している。
  - `extension/test/catalog.test.ts`・`popupHtml.test.ts`・`optionsHtml.test.ts` は現在静的 `<title>` の具体値やコア製品名を検証していないため、AC-05・AC-06・AC-07 を直接確かめるテストを追加する。旧名 `yt2column` が残っていないことは移行確認なので、恒久テストにせず `_checks/` のスクリプトで一度だけ確かめる。
  - `extension/test/metadataDocs.test.ts` が `INSTALL.md` と照合する項目には `yt2column` を含む文言がないため、PR-1 と PR-2 は独立して `make ext-check` を通過できる。
  - 拡張内のコンソールログラベル（`launch.ts:122-128`、`background.ts:43`、`viewerApp.ts:109-111`）、`chromeDeps.ts:570`、`workerCalls.ts:12,15`、`package.json:2` は開発側識別子であり変更しない。
- **文書と Go ガードテスト:**
  - `README.md:3,5,32`・`README.en.md:4,6,33`・`INSTALL.md:3,80,201,241` に利用者向け製品名としての `yt2column` がある（`INSTALL.md:62` は Releases URL のため維持）。
  - `docs/dev/guides/development.md:119` に開発用のビルド表示名の引用がある。
  - `cmd/yt2column/docpaths_test.go:111` は `docs/dev/` のみを走査するため、`docs/store/chrome_web_store.md` は指示文書チェックの対象外である。
  - `cmd/yt2column/readme_sync_test.go::TestReadmeSyncMarker`・`readme_structure_test.go::TestReadmeVersionsShareStructure`・`docs_test.go::TestReadmeLinks`・`adr_test.go::TestADRRecords`・`extension_docs_test.go::TestExtensionDocumentation` が既存ガードとして動作する。

### 1.2. 計画の際に決めたこと（2026-11-25）

1. **実装 PR の分割:** フェーズ 1（ブラウザ拡張とテスト）を PR-1、フェーズ 2（ストア掲載文・利用者向け文書・関連文書）を PR-2 の 2 本の PR に分ける。
   - **理由:** `metadataDocs.test.ts` の照合対象に製品名が含まれないことから 2 本の PR が独立してグリーンゲートを通過でき、コード変更と日本語・英語文書のレビュー観点を分離できるためである。
   - **ADR:** なし（本タスク内の実装分割であり、主要決定ではないため）。

### 1.3. 実装の際に決めたこと（2026-11-26）

1. **§7 に一時スクリプトの記録の枠を足す:** ステップ 1-3 で作る `docs/tasks/0125_product_name/_checks/old_name_absent.sh` について、`docs/dev/conventions/requirements_process.md`「Temporary scripts (`_checks/`)」が求める一時スクリプトの記録の枠を、§7 の受け入れ基準の検証に足す。
   - **理由:** 本計画は同節の規則が `main` に入った後に承認されており、`_checks/` のスクリプトごとに §7 の記録を求める規則が当てはまる。計画が既に決めているスクリプト・チェック・予定する実行を記録の形に写すだけで、新しいチェックや実行は足さない。壊し確認の記録と実行の記録は実装時に埋める。
   - **ADR:** なし（承認済みの計画に既存の規則の記録を当てはめるもので、主要決定ではないため）。
2. **AC-05・AC-06 の確かめを確定値の完全一致に変える:** `extension/test/catalog.test.ts` で、`optionsPage.title` と再起動案内を、各ロケールのコア製品名の包含だけでなく、AC-05・AC-06 が定める確定値との完全一致で確かめる。
   - **理由:** コア製品名を含むだけの検査では、`ヨムドウガの設定（拡張機能）` のように確定値から外れた文言でも通り、AC の文言の退行を検出できない。`manifest.test.ts` が決めた値を固定するのと同じく、AC の文言をテストで固定する。
   - **CLAUDE.md の規則との関係:** CLAUDE.md「Testing Strategy」の「Do not assert that a constant equals its own literal」は、定義をそのまま写した値と比べて何も確かめないテストを禁じる規則である。このテストの期待値は要件定義書の AC-05・AC-06 が定める文言で、カタログの値とは出どころが違う。カタログを AC から外れた文言に変えると失敗するので、この規則には当たらないと判断した。承認済みの計画で `manifest.test.ts` が `extName`・`extNameDev`・`extDescription` の確定値を日英のリテラルで固定しているのと同じ扱いであり、日本語のリテラルもこれと同じく、日本語カタログのメッセージを確かめるテストのリテラルとして置く。
   - **ADR:** なし（テストの確かめ方の変更であり、主要決定ではないため）。

## 2. 実装ステップ

### フェーズ 1: ブラウザ拡張の製品名・UI 文言・テスト更新

設計書 3.1 節・3.2 節および 8 章のフェーズ 1 に対応する。

#### ステップ 1-1: ロケールカタログと静的 HTML の更新

**変更するファイル:** `extension/static/_locales/ja/messages.json`、`extension/static/_locales/en/messages.json`、`extension/static/popup.html`、`extension/static/options.html`、`extension/static/viewer.html`

- [x] `extension/static/_locales/ja/messages.json` の `extName`・`extNameDev`・`extDescription` を設計書 3.1 節の表の確定値へ更新する
- [x] `extension/static/_locales/en/messages.json` の `extName`・`extNameDev`・`extDescription` を設計書 3.1 節の表の確定値へ更新する
- [x] `extension/static/popup.html`・`options.html`・`viewer.html` の `<title>yt2column</title>` を `<title>YomuDoga</title>` へ変更する（`extension/static/manifest.json` は変更しない）

**完了基準:** 全 5 ファイルが設計書 3.1 節・3.2 節の確定値と一致し、`manifest.json` に差分がないこと。

#### ステップ 1-2: UI カタログの更新

**変更するファイル:** `extension/src/core/i18n/ja.ts`、`extension/src/core/i18n/en.ts`

- [x] `extension/src/core/i18n/ja.ts` の 5 項目（`rejection["not-watch-page"].steps[2]`・`rejection["empty-selection"].steps[2]`・`viewerTitle.name.release`・`viewerTitle.name.dev`・`optionsPage.title`）を設計書 3.2 節の表の確定値へ更新する
- [x] `extension/src/core/i18n/en.ts` の同じ 5 項目を設計書 3.2 節の表の確定値へ更新する

**完了基準:** 設計書 3.2 節の表どおりに両カタログが更新され、`extension/src/` の他のファイルに変更がないこと。

#### ステップ 1-3: 拡張テストの更新・追加と検証

**変更するファイル:** `extension/test/manifest.test.ts`、`extension/test/buildVariant.test.ts`、`extension/test/viewerTitle.test.ts`、`extension/test/viewerApp.test.ts`、`extension/test/catalog.test.ts`、`extension/test/popupHtml.test.ts`、`extension/test/optionsHtml.test.ts`、`docs/tasks/0125_product_name/_checks/old_name_absent.sh`（新規）

- [x] `extension/test/manifest.test.ts` を更新し、日英それぞれの `extName`・`extNameDev`・`extDescription` の確定値と既存制約（132 文字以内・`$` 禁止・言語判定・`manifest.json` キー・権限・拡張 ID）を検証する
- [x] `extension/test/buildVariant.test.ts` の `extNameDev` 先頭一致検査をロケール別コア製品名（`YomuDoga` / `ヨムドウガ`）へ更新する
- [x] `extension/test/viewerTitle.test.ts` を設計書 3.2 節の例外契約に合わせて更新する（公開用のビルドはコア製品名、開発用のビルドは `extNameDev` と完全一致、かつ公開用のビルドのコア製品名（`viewerTitle.name.release`）に ` for YouTube™` を添えた値が `extName` と一致すること）
- [x] `extension/test/viewerApp.test.ts` の `doc.title` アサーション全 8 箇所を `YomuDoga` へ更新する
- [x] `extension/test/catalog.test.ts` に、`optionsPage.title` と `rejection["not-watch-page"].steps`・`rejection["empty-selection"].steps` の再起動案内が、各ロケールの確定値（AC-05・AC-06 の文言）と完全に一致することを確かめるテストを追加する（確定値と完全一致で確かめ、コア製品名を含むだけの検査にしない）
- [x] `extension/test/popupHtml.test.ts`（`popup.html`・`viewer.html`）と `extension/test/optionsHtml.test.ts`（`options.html`）に、静的 `<title>` が `YomuDoga` であることの検証を追加する
- [x] `docs/tasks/0125_product_name/_checks/old_name_absent.sh`（シェル）を追加する。`extension/src/core/i18n/ja.ts`・`en.ts`、`extension/static/_locales/{ja,en}/messages.json`、`extension/static/{popup,viewer,options}.html` に `yt2column` が 1 か所もないことを `grep` で確かめ、見つかれば該当行を出力して非 0 で終わる。旧値を 1 か所戻すと失敗することを確かめたうえで PR-1 のブランチで実行し、出力と両コミット ID を §7 の AC-05・AC-06 の行に記録する
- [x] 更新・追加した各テストについて、対象文字列を一時的に旧値へ戻す壊し確認を行い失敗を確かめたうえ、元に戻して `make ext-check` と `make test && make lint` がすべて通過することを確認する

**完了基準:** 壊し確認が記録され、`make ext-check`・`make test`・`make lint` がすべて成功すること。

### PR-1 作成ポイント: extension product name and UI wording

**対象ステップ**: 1-1 / 1-2 / 1-3

**推奨タイトル**: `feat(0125): ブラウザ拡張の表示名・説明文・UI 文言をヨムドウガ / YomuDoga へ更新`

**レビュー観点**: `messages.json`（ja/en）の確定値と文字数・言語制約 / `viewerTitle.test.ts` のコア製品名と `extName` の分離契約 / 静的 HTML と `catalog.test.ts` の新製品名検証 / `_checks/old_name_absent.sh` の旧名不在の記録 / 拡張 ID・権限および開発側識別子が不変であること

**実装モデル要件**: low

**判定理由**: 高リスク・未確定アプローチ・Conditional check の複数該当のいずれのトリガにも該当せず、確定済みの文字列とテスト期待値の更新のみであるため。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1165
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

---

### フェーズ 2: ストア掲載文・利用者向け文書・関連文書の更新

設計書 3.3 節・3.4 節および 8 章のフェーズ 2 に対応する。

#### ステップ 2-1: ストア掲載文の作成と用語集の更新

**変更するファイル:** `docs/store/chrome_web_store.md`（新規）、`cmd/yt2column/store_listing_test.go`（新規）、`docs/translation_glossary.md`

- [x] `docs/store/chrome_web_store.md` を設計書 3.3 節の構成で作成し、issue #1148 コメント 1・4・5 の表示名・短い説明文・商標運用条件・日本語版および英語版の詳細説明文（`text` コードブロック）を正確に収録する。`extName`・`extNameDev`・`extDescription` は設計書 3.3 節の掲載項目の表（列 `キー`・`日本語`・`英語`）に置く
- [x] `cmd/yt2column/store_listing_test.go::TestStoreListingMatchesMessages` を追加する。`internal/doctestutil` の表の読み取りで掲載項目の表を見つけ、3 行がちょうど 1 回ずつあり各値が `extension/static/_locales/{ja,en}/messages.json` の `message` と完全に一致することを確かめる。値のセルの形が違う・行の欠け・重複・知らないキーは失敗とする。表の値を 1 つ変える、行を 1 つ消す、行を重ねる、それぞれで失敗することを確かめ、コミットメッセージに記す
- [x] `docs/translation_glossary.md` にコア製品名（`ヨムドウガ` / `YomuDoga`）および Chrome ウェブストア表示名（`ヨムドウガ for YouTube™` / `YomuDoga for YouTube™`）の 2 行を追加する

**完了基準:** `docs/store/chrome_web_store.md` が issue #1148 コメント 1・4・5 の内容と過不足なく一致し、`TestStoreListingMatchesMessages` が通り、`docs/translation_glossary.md` に新製品名 2 行が追加されていること。

#### ステップ 2-2: 利用者向け文書（`README.md`・`README.en.md`・`INSTALL.md`）の更新

**変更するファイル:** `README.md`、`README.en.md`、`INSTALL.md`

- [x] `README.md` を設計書 3.4 節のとおり更新する（H1 の `# ヨムドウガ for YouTube™` への変更、冒頭概要文の刷新、`## 特徴` の 4 項目新設、既存節の維持と製品名更新、ファイル末尾（空行と水平線 `---` の後ろ）への非公式表記と帰属表示（`YouTube is a trademark of Google LLC.`）の追加、Chrome 142 表記と未リリース注記の維持）
- [x] `/mktrans README.md` を実行して `README.en.md` を同期し、`README.md` と `README.en.md` を同一コミットに含める
- [x] `INSTALL.md` の 4 箇所（冒頭 `:3`、ツールバー固定 `:80`、ビューアタイトル `:201`、アンインストール `:241`）を設計書 3.4 節のとおり更新する（`:62` の Releases URL と既存引用文言・Chrome 142 表記は維持）

**完了基準:** `README.md`・`README.en.md`・`INSTALL.md` が要件書 F-004 を満たし、`TestReadmeSyncMarker`・`TestReadmeVersionsShareStructure`・`TestReadmeLinks`・`metadataDocs.test.ts`・`manifest.test.ts` が通過すること。

#### ステップ 2-3: 開発手順の事実整合・`CLAUDE.md` 更新・ADR-0035 のずれ除去と検証

**変更するファイル:** `docs/dev/guides/development.md`、`CLAUDE.md`、`docs/dev/adr/0035-language-policy.md`、本実装計画書（§7 の AC-10〜AC-12・AC-14・AC-15・AC-16 の差分記録）

- [x] `docs/dev/guides/development.md:119` のみを `ヨムドウガ（開発用のビルド）` および `YomuDoga (development build)` へ書き換える（他行は変更しない）
- [x] `CLAUDE.md` の「Document language」例外に `docs/store/chrome_web_store.md`（日英の Chrome ウェブストア詳細説明文を 1 ファイルで保管し `/mktrans` 同期対象外とする扱い）を追記し、「Development Notes」の Go のガードテストの一覧に掲載項目の表と `messages.json` の照合を足す
- [x] `docs/dev/adr/0035-language-policy.md` の `## 決定` から `docs/store/chrome_web_store.md` と `CLAUDE.md` の未反映のずれを示す一文を除去し、コミットメッセージに `Editorial correction:` 行を、PR の説明に修正の段落を記録する（`docs/dev/conventions/adr.md`「Editorial corrections」第 2 種）
- [x] `make test && make lint` と `make ext-check` がすべて通過することを確認し、AC-10〜AC-12（`README.md`）・AC-14（`INSTALL.md`）・AC-15（`docs/dev/guides/development.md`）・AC-16（`docs/translation_glossary.md`）の差分を `requirements_process.md` §4 の固定コマンドで取得して本実装計画書 §7 の各 `Diff record` に記録する

**完了基準:** すべてのテスト・lint・拡張検査が通過し、§7 の全 `Diff record` が実装コミットに対して記録され、記録後のコミットで比較対象パスが変更されていないこと。

### PR-2 作成ポイント: store listing, user docs, and language policy sync

**対象ステップ**: 2-1 / 2-2 / 2-3

**推奨タイトル**: `docs(0125): ストア掲載文の追加と利用者向け文書・関連文書の製品名更新`

**レビュー観点**: `docs/store/chrome_web_store.md` の日英詳細説明文と商標条件の正確性 / `README.md`・`README.en.md` の H1・`## 特徴`・帰属表示と `/mktrans` 同期 / `INSTALL.md` の 4 箇所更新と既存ガード通過 / `development.md` の 1 行限定差分（AC-15）および `CLAUDE.md`・ADR-0035 の整合

**実装モデル要件**: low

**判定理由**: 高リスク・未確定アプローチ・Conditional check の複数該当のいずれのトリガにも該当せず、確定案に基づく文書更新・README 同期・差分記録の適用であるため。

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン

### 3.1. マイルストーン

1. **マイルストーン 1（PR-1 マージ）:** ブラウザ拡張の manifest 表示名・説明文・UI 文言・静的 HTML・拡張テストがすべて新製品名へ移行し、`make ext-check` と `make test && make lint` が通過している状態。
2. **マイルストーン 2（PR-2 マージ）:** Chrome ウェブストア詳細説明文がリポジトリに保管され、`README.md`・`README.en.md`・`INSTALL.md`・`development.md`・`translation_glossary.md`・`CLAUDE.md`・ADR-0035 がすべて整合した状態。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 | ブラウザ拡張の `messages.json`・静的 HTML・UI カタログ・拡張テストの更新 | low | auto |
| PR-2 | 2-1 / 2-2 / 2-3 | ストア掲載文の新設、`README.md` / `README.en.md`・`INSTALL.md`・`development.md`・用語集・`CLAUDE.md`・ADR-0035 の更新 | low | auto |

## 4. テスト戦略

- **PR-1（ブラウザ拡張）:** `make ext-check`（`ext-typecheck`・`ext-lint`・`ext-fmt-check`・`ext-test`・`ext-build`・`ext-build-dev`）および `make test && make lint` を実行する。新規のテストヘルパーは追加せず、既存の `extension/test/` スイートを更新・拡張する。更新・追加したテストはいずれも旧値に戻す壊し確認を実施する。
- **PR-2（文書）:** `make test && make lint`（`TestReadmeSyncMarker`・`TestReadmeVersionsShareStructure`・`TestReadmeLinks`・`TestADRRecords`・`TestInstructionDocPaths`・`TestExtensionDocumentation`・`TestStoreListingMatchesMessages` を含む）および `make ext-check`（`manifest.test.ts` の Chrome 最低版検査、`metadataDocs.test.ts` の `INSTALL.md` 引用文言検査を含む）を実行する。AC-10〜AC-12・AC-14・AC-15・AC-16 は recorded diff reading（`requirements_process.md` §4 の `diff` 検証）で検証し、自然言語文書の内容は PR レビューの目視確認（`manual`）で検証する。

## 5. リスク管理

| リスク | 影響 | 対策 |
|---|---|---|
| `README.md` の更新後に `README.en.md` の同期漏れ、または見出し構造の不一致が起きる | `TestReadmeSyncMarker` または `TestReadmeVersionsShareStructure` が失敗する | ステップ 2-2 で `/mktrans README.md` を実行し、両ファイルを同一コミットに含めて `make test` で確認する |
| `README.md`・`README.en.md`・`INSTALL.md` の編集で `Chrome 142 以降` や引用文言を意図せず変えてしまう | `manifest.test.ts` または `metadataDocs.test.ts` が失敗する | ステップ 2-2・2-3 の完了前に `make ext-check` を実行して検出する |
| `docs/dev/guides/development.md` で 119 行目以外の箇所を誤って編集する | AC-15 違反になる | ステップ 2-3 で AC-15 の固定差分コマンドを実行して 1 行のみの変更であることを確認し、`Diff record (AC-15)` に記録する |
| `docs/dev/adr/0035-language-policy.md` のずれ除去で修正行（`Editorial correction:`）を書き忘れる | ADR 編集規約に反する | ステップ 2-3 のチェックリストに明記し、コミットメッセージと PR の説明へ記録する |

## 6. 実装チェックリスト

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種別 | 検証先・方法 | 対応ステップ |
|---|---|---|---|
| AC-01 | `test` | `extension/test/manifest.test.ts`（`ja` の `extName`・`extNameDev`・`extDescription` 確定値）・`extension/test/buildVariant.test.ts`（`extNameDev`） | 1-1 / 1-3 |
| AC-02 | `test` | `extension/test/manifest.test.ts`（`en` の `extName`・`extNameDev`・`extDescription` 確定値）・`extension/test/buildVariant.test.ts`（`extNameDev`） | 1-1 / 1-3 |
| AC-03 | `test` | `extension/test/manifest.test.ts`（132 文字以内・`$` 禁止・言語判定・`manifest.json` キー・権限・拡張 ID）・`cmd/yt2column/extension_docs_test.go::TestExtensionDocumentation` | 1-1 / 1-3 |
| AC-04 | `test` | `extension/test/viewerTitle.test.ts`（日英の公開用・開発用タイトルおよび `messages.json` との照合）・`extension/test/viewerApp.test.ts`（`doc.title`） | 1-2 / 1-3 |
| AC-05 | `test` / `diff` | `extension/test/catalog.test.ts`（`optionsPage.title` が各ロケールの確定値 `ヨムドウガの設定` / `YomuDoga settings` と一致すること）・`extension/test/optionsApp.test.ts`（`doc.title` 反映）。旧名 `yt2column` が残っていないことは `docs/tasks/0125_product_name/_checks/old_name_absent.sh` の一度だけの実行で確かめ、出力を記録する | 1-2 / 1-3 |
| AC-06 | `test` / `diff` | `extension/test/catalog.test.ts`（`rejection["not-watch-page"]`・`rejection["empty-selection"]` の再起動案内が各ロケールの確定値 `もう一度ヨムドウガを起動してください。` / `Launch YomuDoga again.` と一致すること）。旧名の不在は AC-05 の行と同じ `_checks/old_name_absent.sh` の記録で確かめる | 1-2 / 1-3 |
| AC-07 | `test` | `extension/test/popupHtml.test.ts`（`popup.html`・`viewer.html` の静的 `<title>` が `YomuDoga`）・`extension/test/optionsHtml.test.ts`（`options.html` の静的 `<title>` が `YomuDoga`） | 1-1 / 1-3 |
| AC-08 | `test` / `manual` | `cmd/yt2column/store_listing_test.go::TestStoreListingMatchesMessages` で掲載項目の表の表示名・短い説明文が `messages.json`（日英）と一致することを検証し、PR-2 レビューで詳細説明文と商標運用条件を issue #1148 コメント 1・4・5 と照合して確認 | 2-1 |
| AC-09 | `test` / `manual` | `cmd/yt2column/adr_test.go::TestADRRecords`・`cmd/yt2column/docpaths_test.go::TestInstructionDocPaths` で形式とリンクを検証し、PR-2 レビューで `CLAUDE.md`「Document language」と ADR-0035 の記述・ずれ除去を目視確認 | 2-3 |
| AC-10 | `diff` / `manual` | 下の `Diff record (AC-10・AC-11・AC-12)`（対象パス: `README.md`）で H1 見出し `# ヨムドウガ for YouTube™` の変更を確認し、PR-2 レビューで冒頭概要文を issue #1148 コメント 6 と照合して確認 | 2-2 / 2-3 |
| AC-11 | `test` / `diff` / `manual` | `cmd/yt2column/readme_structure_test.go::TestReadmeVersionsShareStructure` および下の `Diff record (AC-10・AC-11・AC-12)`（対象パス: `README.md`）で `## 特徴` 見出しと 4 項目の構成を確認し、PR-2 レビューで 4 要点の内容と不要項目を含まないことを確認 | 2-2 / 2-3 |
| AC-12 | `test` / `diff` / `manual` | `extension/test/manifest.test.ts`（Chrome 142 表記）・`cmd/yt2column/docs_test.go::TestReadmeLinks`（相対リンクとアンカー）で検証し、下の `Diff record (AC-10・AC-11・AC-12)`（対象パス: `README.md`）で許可された変更以外の差分がないことを確かめ、PR-2 レビューで非公式表記と帰属表示を目視確認 | 2-2 / 2-3 |
| AC-13 | `test` / `manual` | `cmd/yt2column/readme_sync_test.go::TestReadmeSyncMarker`・`cmd/yt2column/readme_structure_test.go::TestReadmeVersionsShareStructure`・`cmd/yt2column/docs_test.go::TestReadmeLinks`・`extension/test/manifest.test.ts` で同期・構造・リンク・Chrome 最低版を検証し、PR-2 レビューで訳文と帰属表示を目視確認 | 2-2 |
| AC-14 | `test` / `diff` / `manual` | `extension/test/metadataDocs.test.ts`・`extension/test/manifest.test.ts`・`cmd/yt2column/docs_test.go::TestReadmeLinks` で引用文言・Chrome 最低版・リンクを検証し、下の `Diff record (AC-14)`（対象パス: `INSTALL.md`）で指定 4 箇所以外に差分がないことを確かめ、PR-2 レビューで内容を目視確認 | 2-2 / 2-3 |
| AC-15 | `diff` | recorded diff reading（対象パス: `docs/dev/guides/development.md`）。PR-2 実装時に下の `Diff record (AC-15)` へ記録し、PR レビューで確認 | 2-3 |
| AC-16 | `diff` / `manual` | 下の `Diff record (AC-16)`（対象パス: `docs/translation_glossary.md`）で追加した 2 行の差分を確かめ、PR-2 レビューで用語集の記述を目視確認 | 2-1 / 2-3 |

### 一時スクリプトの記録（`_checks/old_name_absent.sh`）

- **スクリプト:** `docs/tasks/0125_product_name/_checks/old_name_absent.sh`。移行の確認として、利用者に見える拡張の文言から旧名 `yt2column` が消えたことを一度だけ確かめる。確かめる AC は AC-05・AC-06。
- **チェック:**
  - `old_name_absent`: `extension/src/core/i18n/ja.ts`・`extension/src/core/i18n/en.ts`・`extension/static/_locales/ja/messages.json`・`extension/static/_locales/en/messages.json`・`extension/static/popup.html`・`extension/static/options.html`・`extension/static/viewer.html` に `yt2column` が 1 か所もないこと。見つかった行を出力して非 0 で終わる。読み取れないファイルがあるときは、合格を報告せず非 0 で終わる。各チェックは少なくとも 1 つの壊し方で失敗する。
- **予定する実行:** PR-1 を作る前に、実装者が PR-1 のブランチで 1 回実行する。出力と 2 つのコミット ID は下の「AC-05・AC-06 の記録」に書く。
- **テスト:** コミットしない（旧名の不在は移行の確認であり、改名が定着した後も旧名を禁じ続ける恒久テストにはしない）。
- **壊し確認の記録:** スクリプトを入れたコミット `15113b6f38214db45f269e9755cc22977c323912` の内容で確かめた。`extension/static/_locales/ja/messages.json` の `extName` を旧値 `yt2column` に戻すと、スクリプトは該当行を出力し、`old_name_absent: FAIL` を出して終了コード 1 になった。
- **実行の記録:** 同じコミット `15113b6f38214db45f269e9755cc22977c323912` の内容で、PR-1 のブランチで 1 回実行し、`old_name_absent: PASS` と終了コード 0 になった（結果の全文と 2 つのコミット ID は下の「AC-05・AC-06 の記録」）。

### AC-05・AC-06 の記録

`_checks/old_name_absent.sh` を PR-1 のブランチで 1 回実行した（2026-11-26）。スクリプトを入れたコミットは `15113b6f38214db45f269e9755cc22977c323912`、マージベースは `1524ae18b46248384fc187d59fb096c7d5d5d07e` で、結果は `old_name_absent: PASS`（終了コード 0）である。壊し確認は一時スクリプトの記録のとおりで、旧値を 1 か所戻すと終了コード 1 になった。

```text
old_name_absent: PASS
```

Diff record (AC-10・AC-11・AC-12):

- Compared paths: `README.md`
- Merge-base: `1fd959779dd3ba8f33d562b066dd7f4e59182b43`
- Compared commit: `caa8b7f18fd06ba077a3efd729ebea59574e37db`

Path diff:

````text
diff --git a/README.md b/README.md
index 83b30756e7691841636f7df7d49c0eef51884ba9..52fe3b8b0e3ca552a8b25d4496ad4da684a9bae6 100644
--- a/README.md
+++ b/README.md
@@ -1,11 +1,18 @@
 [English](README.en.md)

-# yt2column
+# ヨムドウガ for YouTube™

-yt2column は、YouTube 動画の内容を、短時間で読める雑誌コラム風の記事にまとめてくれるブラウザ拡張機能です。この拡張機能は無料です。ただし、記事を作る AI のサービスには、利用料がかかります。利用料は、AI を提供する会社（Anthropic 社）に直接支払います。記事はブラウザで読めるほか、ファイルに保存したり、クリップボードにコピーすることも可能です。
+ヨムドウガ for YouTube™ は、YouTube™ 動画の文字起こしを、新聞・雑誌風の日本語コラム記事に仕立てるブラウザ拡張機能です。話し言葉の脱線や重複を整理し、編集・校正まで行うことで、動画を見なくても論点がすっと頭に入る読み物を目指しました。この拡張機能は無料です。ただし、記事を作る AI のサービスには、利用料がかかります。利用料は、AI を提供する会社（Anthropic 社）に直接支払います。記事はブラウザで読めるほか、ファイルに保存したり、クリップボードにコピーすることも可能です。

 > **状態:** 開発の初期段階です。まだリリースしていないので、今は一般には公開していません。公開後の導入手順を、公開に備えて [INSTALL.md](INSTALL.md) にまとめています。

+## 特徴
+
+- **結論が先、重要度順に読める構成:** 冒頭のリード文に主題と最も重要な結論をまとめ、本文も重要度順に並べます。新聞記事のように、途中で読むのをやめても動画の大枠をつかめます。
+- **動画に合った形式への編集と、文字起こしとの突き合わせ:** インタビュー・対談・レビュー・解説コラムから動画に合った形式を選び、字数を整えたうえで、元の文字起こしと突き合わせて誤字や根拠のない記述をチェックします。
+- **画面の変更に強い:** ページを自動操作せず、あなたが選んだ文字起こしだけを受け取るため、YouTube の画面が変わっても安定して使い続けられます。
+- **安心して使える設計:** ボタンを押したときだけ対象のページを読み、中継サーバを持たずにブラウザから AI サービスへ直接通信します。ソースコードやプロンプトはすべてオープンソースで公開しています。
+
 ## 仕組み

 この拡張機能は、動画の文字起こし（音声を文字にしたもの）のうち、あなたが選んだ部分と、動画の情報を集め、Anthropic 社の有料サービスを利用して AI に記事を書かせます。結果はブラウザの専用タブに表示します。
@@ -29,7 +36,7 @@ API キーとワークスペース ID の作り方、料金の支払い方、使

 1. [INSTALL.md](INSTALL.md) に従って、Anthropic 社で API キーを作り（料金の支払いの準備を含みます）、拡張機能を入れて、API キーを保存します（最初の 1 回だけです）。
 2. YouTube の動画のページで、文字起こしパネルを開き、パネルに出ている行を選びます。
-3. ブラウザのツールバーにある yt2column のアイコンを押し、記事の生成を始めます。
+3. ブラウザのツールバーにあるヨムドウガのアイコンを押し、記事の生成を始めます。
 4. 記事が専用のタブに表示されます。メモ帳などで開ける文章ファイル（Markdown 形式の `.md`）として保存するか、クリップボードにコピーできます。ブラウザを終了すると記事は消えるので、残したいときは、終了する前に保存かコピーをしてください。

 入手から最初の記事の保存までの詳しい手順は、[INSTALL.md](INSTALL.md) で説明しています。
@@ -47,3 +54,7 @@ API キーとワークスペース ID は、使っているブラウザの中に
 ## 開発者向けの文書

 開発者向けに、ブラウザでなくコマンドラインで使えるものもあります。記事の生成の仕組みは[記事の生成の詳細](docs/dev/guides/generation.md)、CLI の実行の仕方は[CLI の使い方](docs/dev/guides/cli.md)、開発の手順は[開発の手順](docs/dev/guides/development.md)にあります。
+
+---
+
+本拡張機能は Google LLC または YouTube とは無関係の非公式ツールです。YouTube is a trademark of Google LLC.
````

Diff record (AC-14):

- Compared paths: `INSTALL.md`
- Merge-base: `1fd959779dd3ba8f33d562b066dd7f4e59182b43`
- Compared commit: `caa8b7f18fd06ba077a3efd729ebea59574e37db`

Path diff:

````text
diff --git a/INSTALL.md b/INSTALL.md
index 93b0545264deb2813fbbfee4f97cae0c8aaa91d6..14f12c62575427c417d979f768772bad46538695 100644
--- a/INSTALL.md
+++ b/INSTALL.md
@@ -1,6 +1,6 @@
 # インストールと使い方

-本書は、yt2column のブラウザ拡張を入手してから、最初の記事を保存し、更新し、削除するまでの手順を説明します。拡張が何をするかは [README](README.md) にあります。
+本書は、ヨムドウガ（拡張機能の一覧での名前: ヨムドウガ for YouTube™）のブラウザ拡張を入手してから、最初の記事を保存し、更新し、削除するまでの手順を説明します。拡張が何をするかは [README](README.md) にあります。

 本書は日本語だけで書いています（英語版は用意していません）。画面の文言は、ブラウザの UI の言語が日本語のときの日本語の版を引用します。

@@ -77,7 +77,7 @@ API キーは、作った直後にしか全体を見られないことがあり

 ### ツールバーに固定する

-ブラウザのツールバーの拡張機能のメニュー（パズルのピースの形のアイコン）を開き、yt2column のピンを押して、アイコンをツールバーに固定します。固定しておくと、ポップアップをすぐ開けます。
+ブラウザのツールバーの拡張機能のメニュー（パズルのピースの形のアイコン）を開き、ヨムドウガ for YouTube™ のピンを押して、アイコンをツールバーに固定します。固定しておくと、ポップアップをすぐ開けます。

 ### ブラウザの警告とシークレットモード

@@ -198,7 +198,7 @@ API キーとワークスペース ID は、このブラウザのプロファイ

 ### ビューア

-ビューアは、リンクが正しくない場合を除くどの状態でも、元の動画へのリンクを表示し、記事が完成する前はポップアップと同じ見出しとチャンネル名を表示します。動画のタイトルを読めなかったときは、見出しは動画の URL になります。記事が完成した後は、拡張が整えた動画のタイトルとチャンネル名を表示します（動画のタイトルがないときは、それまでの見出しのまま）。ページのタイトルは「`<動画のタイトル> - yt2column`」で、ブラウザの履歴とタブに出ます。動画のタイトルがないときは、見出しを使います。
+ビューアは、リンクが正しくない場合を除くどの状態でも、元の動画へのリンクを表示し、記事が完成する前はポップアップと同じ見出しとチャンネル名を表示します。動画のタイトルを読めなかったときは、見出しは動画の URL になります。記事が完成した後は、拡張が整えた動画のタイトルとチャンネル名を表示します（動画のタイトルがないときは、それまでの見出しのまま）。ページのタイトルは「`<動画のタイトル> - ヨムドウガ`」で、ブラウザの履歴とタブに出ます。動画のタイトルがないときは、見出しを使います。

 - **準備中**・**読み込み中**: 動く印と、どちらを待っているかを示す文言を表示します。
 - **生成中**: 3 つの工程のチェックリストと、記事の生成を始めてからの経過時間を表示します。
@@ -238,7 +238,7 @@ zip の展開ツールが、別名のフォルダや入れ子のフォルダに
 ## アンインストール

 1. `chrome://extensions/`（Brave: `brave://extensions/`）を開きます。
-2. yt2column の **削除** を押します。
+2. ヨムドウガ for YouTube™ の **削除** を押します。

 拡張を削除すると、保存した API キーとワークスペース ID も消えます。削除した後は、展開したフォルダを消してかまいません。

````

Diff record (AC-15):

- Compared paths: `docs/dev/guides/development.md`
- Merge-base: `1fd959779dd3ba8f33d562b066dd7f4e59182b43`
- Compared commit: `91de86d60136916911d4f091dd0c966d91e7f6e9`

Path diff:

````text
diff --git a/docs/dev/guides/development.md b/docs/dev/guides/development.md
index a8c55e14eefca27c18395a869478eecd2e594bee..b86c030391a36acaacf7ac3ea04dec4a570ef13e 100644
--- a/docs/dev/guides/development.md
+++ b/docs/dev/guides/development.md
@@ -116,7 +116,7 @@ make ext-build-dev FAKE_ARTICLE=long FAKE_FAILURE=timeout FAKE_FAIL_AT=shorten
 開発用のビルドは、次の点で公開用のビルドと見分けられる。

 -   manifest が `key` を持たないので、拡張の ID が公開用のビルドと違う。`chrome.storage.local` も別になり、開発用のビルドで保存した値が公開用のビルドの値を上書きしない。
--   拡張機能のページでの名前が `yt2column（開発用のビルド）`（英語の画面では `yt2column (development build)`）である。
+-   拡張機能のページでの名前が `ヨムドウガ（開発用のビルド）`（英語の画面では `YomuDoga (development build)`）である。
 -   ビルドの種類を宣言するモジュール（`buildKind.js`）が `dev` を宣言する。

 どちらのビルドも、最後に成果物を検査する。成果物が `tsc` の出力、`static/` の写し、記事生成モジュール、`wasm_exec.js` だけであること、記事生成モジュールを Node.js で読み込んで呼んだ `buildInfo` の main のパッケージと Go の版と LLM の実装（開発用のビルドでは、ビルドに与えたシナリオ）が期待どおりであることを確かめ、合わなければビルドを失敗させる。
````

Diff record (AC-16):

- Compared paths: `docs/translation_glossary.md`
- Merge-base: `1fd959779dd3ba8f33d562b066dd7f4e59182b43`
- Compared commit: `b7d42c95432a118a0348dcd8f882f5d4e9918c84`

Path diff:

````text
diff --git a/docs/translation_glossary.md b/docs/translation_glossary.md
index 35d3f48a001ca51fe97564d5cb49e7380eda8371..93b1c62774804a4e3021e21819fba18167f7113c 100644
--- a/docs/translation_glossary.md
+++ b/docs/translation_glossary.md
@@ -213,3 +213,5 @@
 | インストールと使い方 | Installing and using | `README.md` の節の見出しと `INSTALL.md` のタイトル |
 | プライバシー | Privacy | `README.md` の節の見出し |
 | 開発者向けの文書 | Developer documents | `README.md` の節の見出し |
+| コア製品名 | core product name | `YouTube` を含めない独自名。日本語 `ヨムドウガ`、英語 `YomuDoga`。UI・ビューアタイトル・README 本文での呼称に使う |
+| Chrome ウェブストア表示名 | Chrome Web Store display name | `manifest.json` の `name`（`extName`）および Chrome ウェブストア・`chrome://extensions/`・README の H1 見出しで使う表示名。日本語 `ヨムドウガ for YouTube™`、英語 `YomuDoga for YouTube™` |
````

## 8. 成功基準

- **機能完全性:** 要件書の AC-01 〜 AC-16 がすべて満たされ、§7 の検証結果が揃っていること。
- **品質基準:** PR-1・PR-2 のそれぞれで `make test && make lint` および `make ext-check` がエラーなく通過すること。
- **セキュリティ確認:** N/A（セキュリティ要件の変更なし）。
- **文書完全性:** `README.md` / `README.en.md` が同期済みであり、`docs/store/chrome_web_store.md`・`INSTALL.md`・`development.md`・`translation_glossary.md`・`CLAUDE.md`・ADR-0035 に矛盾がないこと。

## 9. 次のステップ

1. 本 3 文書 PR のレビューを受け、`/runpr` で 3 文書の承認とマージを行う。
2. マージ後、`/runplan 0125` で PR-1（フェーズ 1）を実装・マージし、続けて `/runplan 0125` で PR-2（フェーズ 2）を実装・マージする。

## 変更履歴

- 2026-11-26 承認（isseis、#1164）
- 2026-11-26 セッション中の再承認：§7 に一時スクリプトの記録の枠を足した（isseis、#1165）
- 2026-11-26 セッション中の再承認：AC-05・AC-06 の確かめを確定値の完全一致に変えた（isseis、#1165）
