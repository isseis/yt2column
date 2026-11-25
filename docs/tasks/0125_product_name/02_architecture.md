# アーキテクチャ設計書：利用者向け製品名（ヨムドウガ / YomuDoga）の適用

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-25 |
| Review date | 2026-11-26 |
| Reviewer | isseis |
| Comments | - |

## 1. 設計の全体像

本書は、[01_requirements.md](01_requirements.md)（以下、要件書）が定める利用者向け製品名および説明文を、ブラウザ拡張と文書へ一貫して適用するための構成と既存テスト契約の変更を定義する。適用する製品名は、コア製品名 `ヨムドウガ` / `YomuDoga`、Chrome ウェブストア表示名 `ヨムドウガ for YouTube™` / `YomuDoga for YouTube™`、開発用のビルド表示名 `ヨムドウガ（開発用のビルド）` / `YomuDoga (development build)` である。実行時のアーキテクチャやデータフローを変更しないため、簡易な進め方（`requirements_process.md`「Simple Process」）に従い Mermaid 図は省略する。既存コードの行番号はコミット `1a5b6cab`（`origin/main`）で確認した。

### 1.1. 設計原則

- **既存の仕組みの再利用（DRY / YAGNI）:** ブラウザ拡張のローカライズ（`extension/static/_locales/{ja,en}/messages.json` および `extension/src/core/i18n/{ja,en}.ts`）、ビルド種別の切り替え（`extension/scripts/build-variant.ts`）、README の 2 言語同期（`/mktrans README.md` と `TestReadmeSyncMarker`）をそのまま使い、新しい実行機構やスクリプトを追加しない。
- **利用者向け名称と開発側名称の明確な分離:** 利用者に見える表示名・説明文・利用者向け文書のみを新製品名へ更新し、開発側の識別子（リポジトリ名、Go モジュール、CLI、環境変数、拡張内部ログ、WASM ファイル名、パッケージ名）は `yt2column` のまま保つ。
- **既存ガードテストとの整合:** 拡張 ID・権限・Chrome 最低版・引用 UI 文言・README 同期・ADR 形式の各ガードテストが引き続き機能する形で、旧製品名前提のテスト期待値のみを更新する。

### 1.2. 設計の際に決めたこと（2026-11-25）

1. **Chrome ウェブストア詳細説明文の配置と形式:** `docs/store/chrome_web_store.md` の 1 ファイルにまとめ、日本語の見出し構成の中に日本語版・英語版の詳細説明文を `text` コードブロックで収め、あわせて表示名・短い説明文・商標運用条件を記載する。
   - **理由:** 日本語版・英語版の本文に加えて表示名・短い説明文・商標条件を 1 か所で管理でき、`CLAUDE.md`「Documents」の「文書は `docs/` 配下に置く」方針にも合う。また `cmd/yt2column/docpaths_test.go:111` の指示文書チェック対象（`docs/dev/` 配下）の外になるため、指示文書の規則と混ざらない。
   - **採らなかった案:** `docs/store/description_ja.txt` と `description_en.txt` の言語別プレーンテキストに分ける案。コピーはしやすいが、表示名や商標条件の注記を別ファイルで管理する必要がある。
   - **ADR:** [ADR-0035](../../dev/adr/0035-language-policy.md) を更新し、具体的な保管先を `docs/store/chrome_web_store.md` として記録した（`CLAUDE.md` への反映は実装 PR で行う）。

## 2. システム構成

変更はブラウザ拡張の静的アセット・UI カタログ・テストと、利用者向け文書および関連文書の 2 領域に分かれる。Go の製品コード（`cmd/`・`internal/`）およびビルドスクリプト（`extension/scripts/`）のロジックは変更しない。

### 2.1. コンポーネント配置と変更ファイル一覧

| 区分 | ファイルパス | 変更内容 |
|---|---|---|
| 変更 | `extension/static/_locales/ja/messages.json` | `extName`・`extNameDev`・`extDescription` を日本語の確定値に更新 |
| 変更 | `extension/static/_locales/en/messages.json` | `extName`・`extNameDev`・`extDescription` を英語の確定値に更新 |
| 変更 | `extension/static/popup.html` | 静的 `<title>` を `YomuDoga` に更新 |
| 変更 | `extension/static/options.html` | 静的 `<title>` を `YomuDoga` に更新 |
| 変更 | `extension/static/viewer.html` | 静的 `<title>` を `YomuDoga` に更新 |
| 変更 | `extension/src/core/i18n/ja.ts` | 拒否手順の再起動案内・`viewerTitle.name`・`optionsPage.title` を新製品名に更新 |
| 変更 | `extension/src/core/i18n/en.ts` | 拒否手順の再起動案内・`viewerTitle.name`・`optionsPage.title` を新製品名に更新 |
| 変更 | `extension/test/manifest.test.ts` | `extName`・`extNameDev`・`extDescription` の検証を日英の確定値へ更新 |
| 変更 | `extension/test/buildVariant.test.ts` | `extNameDev` の先頭一致検査をロケール別コア製品名へ更新 |
| 変更 | `extension/test/viewerTitle.test.ts` | ビューアタイトルの期待値と `messages.json` との照合契約をコア製品名基準へ更新 |
| 変更 | `extension/test/viewerApp.test.ts` | ビューアタイトルのアサーションを `YomuDoga` へ更新 |
| 変更 | `extension/test/catalog.test.ts` | `optionsPage.title`・`rejection` 手順が各ロケールのコア製品名（`viewerTitle.name.release`）を含むことの性質検証を追加 |
| 新規 | `docs/tasks/0125_product_name/_checks/old_name_absent.sh` | 日英カタログ・`messages.json`・静的 HTML に旧名 `yt2column` が残っていないことを一度だけ確かめる移行確認のスクリプト（恒久テストにしない） |
| 変更 | `extension/test/popupHtml.test.ts` | `popup.html`・`viewer.html` の静的 `<title>` が `YomuDoga` であることの検証を追加 |
| 変更 | `extension/test/optionsHtml.test.ts` | `options.html` の静的 `<title>` が `YomuDoga` であることの検証を追加 |
| 新規 | `docs/store/chrome_web_store.md` | Chrome ウェブストアの表示名・短い説明文・商標運用条件・日英詳細説明文を収録 |
| 新規 | `cmd/yt2column/store_listing_test.go` | `docs/store/chrome_web_store.md` の掲載項目の表の値が `messages.json`（日英）の `extName`・`extNameDev`・`extDescription` と一致することを検証するガードテスト |
| 変更 | `README.md` | H1・冒頭概要文・製品名を更新し、`## 特徴` 節および非公式表記と帰属表示を追加 |
| 変更 | `README.en.md` | `/mktrans README.md` により `README.md` と同期 |
| 変更 | `INSTALL.md` | 製品名の 4 箇所（冒頭・ツールバー固定・ビューアタイトル・アンインストール）を更新 |
| 変更 | `docs/dev/guides/development.md` | 119 行目の開発用のビルド表示名の引用のみを更新 |
| 変更 | `docs/translation_glossary.md` | コア製品名と Chrome ウェブストア表示名の 2 行を追加 |
| 変更 | `CLAUDE.md` | 「Document language」に `docs/store/chrome_web_store.md` の位置づけを追記し、「Development Notes」の Go のガードテストの一覧に掲載項目の表と `messages.json` の照合を足す |
| 変更 | `docs/dev/adr/0035-language-policy.md` | 実装 PR で `CLAUDE.md` を更新した際に、未反映のずれを示す一文を除去 |

## 3. コンポーネント設計

### 3.1. ブラウザ拡張の manifest とロケールカタログ（F-001）

`extension/static/manifest.json` は `"name": "__MSG_extName__"`・`"description": "__MSG_extDescription__"`（`:3-4`）および公開鍵 `"key"`（`:7`）・`"permissions"`（`:16`）をそのまま維持し、変更しない。開発用のビルドは `extension/scripts/build-variant.ts:23-26,79-94` の `devManifest` が `"__MSG_extName__"` を `"__MSG_extNameDev__"` に置き換える既存の仕組みをそのまま使う。

ロケールカタログの各項目は次の確定値へ更新する。

| キー | `extension/static/_locales/ja/messages.json` | `extension/static/_locales/en/messages.json` |
|---|---|---|
| `extName` | `ヨムドウガ for YouTube™` | `YomuDoga for YouTube™` |
| `extNameDev` | `ヨムドウガ（開発用のビルド）` | `YomuDoga (development build)` |
| `extDescription` | `YouTube™ 動画を、さっと読める新聞・雑誌風の日本語コラム記事に。選んだ文字起こしから、AI が読みやすく編集します。`（62 文字） | `Turns YouTube™ videos into readable Japanese magazine-style column articles from your selected transcript with AI.`（114 文字） |

いずれの `extDescription` も Chrome の上限 132 文字以内であり、プレースホルダ文字 `$` を含まない。

**既存設計方針への例外（`extName` の日英分離）:**

- **元の方針と場所:** `docs/tasks/0043_extension_i18n/02_architecture.md:507,571` は、`extName` を英語・日本語とも同一の `yt2column` とする方針を定めていた。
- **例外とする理由:** issue #1148 で日本語話者向けのカタカナ製品名（`ヨムドウガ`）と英語表記（`YomuDoga`）を定め、それぞれに互換性表記 `for YouTube™` を添えると決定したためである。
- **更新が必要な既存テスト:**
  - `extension/test/manifest.test.ts:255-273`: `extName` が両ロケールとも `"yt2column"` であることを検証している箇所を、ロケール別の確定値へ更新する。
  - `extension/test/buildVariant.test.ts:73-77`: `extNameDev` が `"yt2column"` で始まることを検証している箇所を、ロケール別のコア製品名（`"YomuDoga"` / `"ヨムドウガ"`）で始まることの検証へ更新する。

### 3.2. ブラウザ拡張の UI カタログと静的 HTML（F-002）

UI カタログ（`extension/src/core/i18n/ja.ts`・`en.ts`）は既存の `Catalog` 型（`extension/src/core/i18n/catalog.ts`）を変更せず、製品名を含む 5 項目（4 行）の文字列値のみを更新する。

| カタログ項目 | `ja.ts`（変更前 → 変更後） | `en.ts`（変更前 → 変更後） |
|---|---|---|
| `rejection["not-watch-page"].steps[2]`（`ja.ts:27`、`en.ts:29`） | `もう一度 yt2column を起動してください。` → `もう一度ヨムドウガを起動してください。` | `Launch yt2column again.` → `Launch YomuDoga again.` |
| `rejection["empty-selection"].steps[2]`（`ja.ts:35`、`en.ts:37`） | `もう一度 yt2column を起動してください。` → `もう一度ヨムドウガを起動してください。` | `Launch yt2column again.` → `Launch YomuDoga again.` |
| `viewerTitle.name.release`（`ja.ts:279`、`en.ts:268`） | `yt2column` → `ヨムドウガ` | `yt2column` → `YomuDoga` |
| `viewerTitle.name.dev`（`ja.ts:279`、`en.ts:268`） | `yt2column（開発用のビルド）` → `ヨムドウガ（開発用のビルド）` | `yt2column (development build)` → `YomuDoga (development build)` |
| `optionsPage.title`（`ja.ts:289`、`en.ts:278`） | `yt2column の設定` → `ヨムドウガの設定` | `yt2column settings` → `YomuDoga settings` |

静的 HTML（`extension/static/popup.html:5`・`options.html:5`・`viewer.html:5`）は `<html lang="en">` に合わせ、`<title>yt2column</title>` を `<title>YomuDoga</title>` へ更新する。`extension/src/launch.ts` への動的な `doc.title` 設定は追加しない。

**既存設計方針への例外（公開用のビルドの `viewerTitle` と `extName` の分離）:**

- **元の方針と場所:** `docs/tasks/0091_extension_only_migration/02_architecture.md:487`・§3.8.2 および `extension/test/viewerTitle.test.ts:65-91` は、公開用のビルドのビューアタイトル末尾が `messages.json` の `extName` と完全一致することを前提としていた（当時は `extName` とコア製品名がともに `yt2column` であった）。
- **例外とする理由:** `extName` に `for YouTube™` が付く一方で、ビューアのタブ名とブラウザ履歴では冗長を避け、開発用のビルド（`ヨムドウガ（開発用のビルド）`）やオプションページと揃えるため、公開用のビルドのビューアタイトルにはコア製品名（`ヨムドウガ` / `YomuDoga`）を使う（要件書「要件定義の際に決めたこと（2026-11-25）」決定 3）。
- **更新が必要な既存テスト:**
  - `extension/test/viewerTitle.test.ts:19-106`: 公開用のビルドのタイトル期待値を `YomuDoga` / `ヨムドウガ` に改める。`:65-91` の `messages.json` 照合テストは、`dev` が `extNameDev` と完全一致すること、および `release` の製品名に ` for YouTube™` を添えた文字列が `extName` と一致することを検証するよう更新する。
  - `extension/test/viewerApp.test.ts:281,337,356,371,400,619,824,835`: 英語ロケールでの `doc.title` 期待値を `YomuDoga` へ更新する。
  - なお `extension/test/optionsApp.test.ts:110` は `catalogs[locale].optionsPage.title` を直接参照しており、カタログ更新に自動追従する。

### 3.3. Chrome ウェブストア詳細説明文と文書言語方針（F-003）

`docs/store/chrome_web_store.md` を新設し、次の構成でストア掲載情報をまとめる。

1. **概要・掲載項目一覧:** 製品名（コア製品名・Chrome ウェブストア表示名・開発用のビルド表示名）および短い説明文（`extDescription`）の日英対照と、`messages.json` が正本であることの明記。
   - **掲載項目の表:** `messages.json` の値は、列が `キー`・`日本語`・`英語` の表に、`extName`・`extNameDev`・`extDescription` の 3 行で置く。値のセルは 1 つのコードスパン（`` `...` ``）だけからなる。
   - **写しの整合:** 表の値は `messages.json` の写しなので、片方だけが変わるとずれる。`cmd/yt2column/store_listing_test.go::TestStoreListingMatchesMessages` が、表の 3 行がちょうど 1 回ずつあり、各値が `extension/static/_locales/{ja,en}/messages.json` の `message` と完全に一致することを確かめる。値のセルがコードスパン 1 つでない、行が欠ける・重複する、知らないキーの行があるときは失敗する（補わない）。テストは `cmd/yt2column` に置くので、CI の `doc-guard` ジョブで文書だけの変更にも走る。詳細説明文の本文は照合する元がなく、この文書が正本なので対象外とする。
2. **商標・審査上の運用条件:** issue #1148 コメント 1 の条件（非公式表記・`YouTube is a trademark of Google LLC.` の帰属表示、アイコン・プロモーション画像での YouTube ロゴおよび紛らわしい再生ボタン意匠の不使用、審査で `for YouTube™` の削除を求められた場合はコア製品名を変えず末尾のみ外す方針）。
3. **詳細説明文（日本語版）:** issue #1148 コメント 5 の日本語確定案を `text` コードブロックで収録。
4. **詳細説明文（英語版）:** issue #1148 コメント 5 の英語確定案を `text` コードブロックで収録。

[ADR-0035](../../dev/adr/0035-language-policy.md) には本 3 文書 PR で `docs/store/chrome_web_store.md` が `/mktrans`・`TestReadmeSyncMarker` の同期対象外である決定を記録し、実装 PR で `CLAUDE.md`「Document language」の例外に追記したうえで、ADR-0035 の `CLAUDE.md` 未反映のずれを示す一文を除去する。

### 3.4. 利用者向け文書と開発手順・用語集（F-004・F-005）

- **`README.md` と `README.en.md`:**
  - `README.md` の H1 を `# ヨムドウガ for YouTube™` にし、冒頭の概要文でコア製品名 `ヨムドウガ` を用いて、話し言葉の脱線や重複を整理し編集・校正まで行って新聞・雑誌風の日本語コラム記事に仕立てることを伝える（未リリースの注記は維持する）。
  - `## 特徴` 節を新設し、issue #1148 コメント 6 の 4 項目（結論が先・重要度順、4 形式への編集と文字起こし突き合わせチェック、非自動操作による画面変更への強さ、押下時のみの読み取り・中継サーバなし・オープンソース）を箇条書きでまとめる。ストア向けの「こんな方に向いています」や既存節と重複する「使い方（3 ステップ）」「注意点」は置かない。
  - 既存の節（`## 仕組み`・`## 必要なもの`・`## インストールと使い方`・`## プライバシー`・`## 開発者向けの文書`）と `Chrome 142 以降` の表記（`extension/test/manifest.test.ts:141-151` が検査）を保ち、`README.md:32` 等の製品名呼称を `ヨムドウガ` に改める。ファイル末尾では `## 開発者向けの文書` 節の本文と紛れないよう、空行と水平線（`---`）で区切った後ろに非公式である旨と `YouTube is a trademark of Google LLC.` の帰属表示を置く（`internal/doctestutil/structure.go:119-125` は空行後の `---` を Setext 見出しではなく水平線として受理する）。
  - `README.en.md` は `/mktrans README.md` で同期し、`TestReadmeSyncMarker`（`cmd/yt2column/readme_sync_test.go`）および `TestReadmeVersionsShareStructure`（`cmd/yt2column/readme_structure_test.go`）を通す。
- **`INSTALL.md`:**
  - `INSTALL.md:3` を「ヨムドウガ（拡張機能の一覧での名前: ヨムドウガ for YouTube™）のブラウザ拡張」に、`INSTALL.md:80` と `:241` をブラウザの拡張機能メニュー・`chrome://extensions/` 上の実際の表示名である `ヨムドウガ for YouTube™` に、`INSTALL.md:201` を `<動画のタイトル> - ヨムドウガ` に更新する。
  - `INSTALL.md:62` の Releases URL（`https://github.com/isseis/yt2column/releases`）および `metadataDocs.test.ts`・`manifest.test.ts` が照合する文言・Chrome 最低版は維持する。
- **`docs/dev/guides/development.md`:**
  - 119 行目の開発用のビルド表示名の引用のみを `ヨムドウガ（開発用のビルド）` および `YomuDoga (development build)` へ更新し、他行は変更しない。
- **`docs/translation_glossary.md`:**
  - コア製品名（`ヨムドウガ` / `YomuDoga`）と Chrome ウェブストア表示名（`ヨムドウガ for YouTube™` / `YomuDoga for YouTube™`）を追加する。

### 3.5. 受け入れ基準との対応

| AC | 対応する設計節 |
|---|---|
| AC-01・AC-02・AC-03 | 3.1 節 |
| AC-04・AC-05・AC-06・AC-07 | 3.2 節 |
| AC-08・AC-09 | 3.3 節 |
| AC-10・AC-11・AC-12・AC-13・AC-14 | 3.4 節 |
| AC-15・AC-16 | 3.4 節 |

## 4. エラーハンドリング設計

N/A（エラー型・エラーコード・エラー遷移のいずれも変更しない。既存の収集拒否手順 2 箇所の文言に含まれる製品名表記のみ 3.2 節のとおり更新する）。

## 5. セキュリティ考慮事項

N/A（`docs/dev/security.md` の適用条件に該当しない。`extension/static/manifest.json` の `permissions`・`content_security_policy`・`incognito`・`key` は変更しない）。

## 6. 処理フロー詳細

N/A（実行時の処理フローおよびメッセージ授受を変更しない）。

## 7. テスト戦略

### 7.1. ユニットテスト・ガードテスト

- **ブラウザ拡張（`make ext-check`）:** 次のテストを実行する。
  - `manifest.test.ts`: `extName`・`extNameDev`・`extDescription`・文字数上限・言語・`$` 禁止・Chrome 最低版・`manifest.json` キーと拡張 ID
  - `buildVariant.test.ts`: `extNameDev` と `extName` の区別・先頭一致
  - `viewerTitle.test.ts`: 日英の公開用・開発用タイトルおよび `messages.json` との整合
  - `viewerApp.test.ts`: `doc.title` の統合確認
  - `optionsApp.test.ts`: `optionsPage.title` の反映
  - `catalog.test.ts`: 日英カタログの全項目非空・禁止語検査・`optionsPage.title` と `rejection` 手順が各ロケールのコア製品名（`viewerTitle.name.release`）を含むこと
  - `popupHtml.test.ts`・`optionsHtml.test.ts`: 静的 `<title>` が `YomuDoga` であること
  - `metadataDocs.test.ts`: `INSTALL.md` の引用文言整合
- **一度だけの移行確認（`_checks/`）:** 旧名 `yt2column` が UI の文言から消えたことは改名の移行確認であり、新しい仕様の規則ではない。恒久テストにすると、改名が定着した後も文字列を永久に禁じることになるため、`docs/tasks/0125_product_name/_checks/old_name_absent.sh` で PR-1 の実装時に一度だけ確かめ、出力を実装計画書の検証の行に記録する。
- **Go ガードテスト（`make test`）:** 次のテストを実行する。
  - `TestReadmeSyncMarker`・`TestReadmeVersionsShareStructure`・`TestReadmeLinks`: `README.md` / `README.en.md` / `INSTALL.md` の同期・構造一致・相対リンクとアンカー
  - `TestADRRecords`: ADR-0035 の形式と索引整合
  - `TestInstructionDocPaths`: `CLAUDE.md`・ADR の相対リンク解決
  - `TestExtensionDocumentation`: 拡張 ID・権限の整合
  - `TestStoreListingMatchesMessages`: `docs/store/chrome_web_store.md` の掲載項目の表と `messages.json`（日英）の値の一致

### 7.2. 統合テスト・セキュリティテスト

- **統合テスト:** N/A（外部コマンドや LLM API、ブラウザ実行経路のロジックを変更しない）。
- **セキュリティテスト:** N/A（セキュリティ境界を変更しない）。

### 7.3. 差分検証と目視確認

- 変更前後の保持条件および小規模な文書値・構造の変更（AC-10・AC-11・AC-12 の `README.md`、AC-14 の `INSTALL.md`、AC-15 の `docs/dev/guides/development.md`、AC-16 の `docs/translation_glossary.md`）はいずれも差分が小さいため、実装計画の recorded diff reading（`diff` 検証）で確かめる。
- 自然言語の記述内容の部分（AC-08 の詳細説明文と商標運用条件、および AC-09〜AC-14・AC-16 の文章部分）は、`CLAUDE.md`「Document tests check values, structure, and executable examples, never prose」に従い PR レビューでの目視確認（`manual`）とし、値・構造・差分部分は上記のテストおよび `diff` 検証と組み合わせる。

## 8. 実装の優先順位

1. **フェーズ 1（ブラウザ拡張の製品名・UI 文言・テスト更新）:** `messages.json`（ja/en）、静的 HTML の `<title>`、`ja.ts` / `en.ts`、2.1 節に挙げた拡張テスト 7 ファイル（`manifest.test.ts`・`buildVariant.test.ts`・`viewerTitle.test.ts`・`viewerApp.test.ts`・`catalog.test.ts`・`popupHtml.test.ts`・`optionsHtml.test.ts`）を更新し、`make ext-check` および `make test && make lint` を通す。
2. **フェーズ 2（ストア掲載文・利用者向け文書・関連文書の更新）:** `docs/store/chrome_web_store.md` を作成し、`docs/translation_glossary.md`、`README.md`・`README.en.md`（`/mktrans README.md`）、`INSTALL.md`、`docs/dev/guides/development.md`、`CLAUDE.md` を更新し、ADR-0035 の `CLAUDE.md` 未反映のずれを示す一文を除去する。`make test && make lint` および `make ext-check` を通し、AC-10〜AC-12・AC-14・AC-15・AC-16 の差分記録を残す。

## 9. 将来の拡張性

- **Chrome ウェブストア審査での `for YouTube™` 削除要求への対応:** コア製品名と Chrome ウェブストア表示名を分離しているため、万一審査で `for YouTube™` の削除を求められた場合は、`messages.json` の `extName`、`docs/store/chrome_web_store.md`、`README.md` / `README.en.md` の H1、`INSTALL.md` の該当箇所、および `manifest.test.ts`・`viewerTitle.test.ts` の Chrome ウェブストア表示名参照のみを改めればよい。ビューアタイトルや拡張 ID には影響しない。
- **`INSTALL.md` の英語版追加（#1044）:** 本タスクで `docs/translation_glossary.md` に日英の製品名を登録しておくことで、将来の英語版作成時にも表記ゆれを防げる。

## 変更履歴

- 2026-11-26 承認（isseis、#1164）
