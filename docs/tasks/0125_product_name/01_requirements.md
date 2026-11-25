# 要件定義書：利用者向け製品名（ヨムドウガ / YomuDoga）の適用

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `simple` |
| Created | 2026-11-25 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、ブラウザ拡張および利用者向け文書において、プロジェクト名 `yt2column` に代わり利用者向けの製品名（日本語 `ヨムドウガ`／英語 `YomuDoga`、Chrome ウェブストア表示名 `ヨムドウガ for YouTube™`／`YomuDoga for YouTube™`）および更新後の説明文を適用するための要件を定める。

対応 issue: [#1148](https://github.com/isseis/yt2column/issues/1148)（関連: [#1042](https://github.com/isseis/yt2column/issues/1042) リリースに向けた利用者向け README・INSTALL 文書）

**背景:**
現在、利用者に見える製品名が GitHub のプロジェクト名 `yt2column` のままになっており、何をするツールかが伝わりにくく覚えにくい。対象箇所は `extension/static/_locales/ja/messages.json:2-8`、`extension/static/_locales/en/messages.json:2-8`、`extension/src/core/i18n/ja.ts:27,35,279,289`、`extension/src/core/i18n/en.ts:29,37,268,278`、`extension/static/{popup,options,viewer}.html:5`、`README.md:3,5,32`、`README.en.md:4,6,33`、`INSTALL.md:3,80,201,241` である（コミット `1a5b6cab` で確認）。リリースおよび将来の Chrome ウェブストア掲載に備え、issue #1148 で決定した製品名・短い説明文・詳細説明文・README 反映方針を利用者に見える箇所へ一貫して反映する必要がある。

**本書が決めることと、設計に委ねること:**

- **本書が決めること:** 適用する製品名と説明文の確定値、変更対象の範囲と変更しない開発側名称の境界、各画面・各文書の観測可能な振る舞いと受け入れ基準、Chrome ウェブストア詳細説明文をリポジトリで保管することと [ADR-0035](../../dev/adr/0035-language-policy.md) での位置づけ。
- **`02_architecture.md` に委ねること:** Chrome ウェブストア詳細説明文の具体的なファイル配置と形式、関連する拡張テストの契約変更の詳細、関連文書間の整合方針。

### issue #1148 で決めたこと

issue #1148 の本文およびコメント（2026-11-25）で、次の事項が決定されている。

- **「YouTube」の扱いと商標条件:** コア製品名には `YouTube` および略称（`YT` 等）を含めない。Chrome ウェブストア表示名（`extName`）は末尾に互換性表記として `for YouTube™` を添える。ストアの詳細説明（および必要に応じて README）に Google LLC / YouTube 非公式である旨と帰属表示（`YouTube is a trademark of Google LLC.`）を明記し、アイコンやプロモーション画像に YouTube ロゴや紛らわしい再生ボタン意匠を使わない。万一ストア審査で `for YouTube™` の削除を求められた場合はコア製品名を変えず末尾のみ外す（コメント 1）。
- **製品名の確定値:**
  - コア製品名（UI・README 本文・日常の呼称）: 日本語 `ヨムドウガ` ／ 英語 `YomuDoga`
  - Chrome ウェブストア表示名（`extName`）: 日本語 `ヨムドウガ for YouTube™` ／ 英語 `YomuDoga for YouTube™`
  - 開発用のビルド表示名（`extNameDev`）: 日本語 `ヨムドウガ（開発用のビルド）` ／ 英語 `YomuDoga (development build)`（コメント 4）
  - **既存方針への例外:** `docs/tasks/0043_extension_i18n/02_architecture.md:507,571` は `extName` を日英共通の `yt2column` としていたが、日本語話者向けのカタカナ製品名と英語表記に互換性表記を添える決定のため、ロケール別の値とする。
- **短い説明文（`extDescription`）の確定値:**
  - 日本語: `YouTube™ 動画を、さっと読める新聞・雑誌風の日本語コラム記事に。選んだ文字起こしから、AI が読みやすく編集します。`
  - 英語: `Turns YouTube™ videos into readable Japanese magazine-style column articles from your selected transcript with AI.`（コメント 4）
- **変更する範囲と維持する範囲:** 利用者に見える名前（`messages.json`、拡張の UI 表示、`README.md`・`README.en.md`、`INSTALL.md`、Chrome ウェブストア掲載情報）を変更し、開発側の名前（リポジトリ名、Go モジュール名、CLI バイナリ名、環境変数 `YT2COLUMN_*`、`docs/` の開発者向け文書）は原則 `yt2column` のまま維持する。拡張機能の ID は `manifest.json` の公開鍵で決まるため変えない（本文・コメント 4）。
- **Chrome ウェブストア詳細説明文の確定案:** 日本語版・英語版の本文がコメント 5 で確定している。
- **README への反映方針:** 冒頭の概要文にストア詳細説明文冒頭の趣旨（話し言葉の脱線や重複を整理し、編集・校正まで行って新聞・雑誌風の日本語コラム記事に仕立てること）を取り入れ、`## 特徴` 節（4 項目）を新設する。ストア向けの「こんな方に向いています」や既存節と重複する「使い方（3 ステップ）」「注意点」は入れず、既存の節（`仕組み`・`必要なもの`・`インストールと使い方`・`プライバシー`・`開発者向けの文書`）を活かして製品名を更新する。英語版（`README.en.md`）は `/mktrans README.md` で同期する（コメント 6）。

### 要件定義の際に決めたこと（2026-11-25）

要件定義の対話で、以下を確認・決定した。

1. **タスクの進め方:** 簡易な進め方（`simple`）とする。
   - **理由:** 新しい仕組みやアーキテクチャ変更を伴わず、既存の拡張メッセージカタログ・UI 文言・利用者向け文書の製品名と説明文および対応テストの更新が中心であるため、3 文書を 1 本の PR でまとめてレビューするほうが適切である。
   - **除外の判定:** 除外に当たらない。`docs/dev/security.md` の規則や前提（[ADR-0006](../../dev/adr/0006-threat-model.md)）、開発ツールの信頼境界（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)）、外部依存のいずれも変更しないためである。
   - **ADR:** なし（タスクの進め方の選択であり、本タスク内で閉じるため）。
2. **J-PlatPat での国内商標の最終確認:** 2026-11-25 に利用者が J-PlatPat で調査し、問題がないことを確認した。
   - **理由:** issue #1148 コメント 4 に残課題として挙げられていた未確認の前提を解消し、`ヨムドウガ` / `YomuDoga` を確定させるためである。
   - **ADR:** なし（事実確認であり設計決定ではないため）。
3. **公開用のビルドのビューアタイトル（`viewerTitle`）:** コア製品名（`<動画のタイトル> - ヨムドウガ` ／ `<video title> - YomuDoga`）を使う。
   - **理由:** タブ幅やブラウザ履歴で冗長にならず、開発用のビルド（`ヨムドウガ（開発用のビルド）`）やオプションページとも表記が揃うためである。
   - **既存方針への例外:** `docs/tasks/0091_extension_only_migration/02_architecture.md:487`・§3.8.2 および `extension/test/viewerTitle.test.ts:65-91` は公開用のビルドのビューアタイトル末尾が `extName` と一致する前提であったが、`extName` に `for YouTube™` が付くことになったため、公開用のビルドのビューアタイトルはコア製品名を使う例外とする。
   - **ADR:** なし（ビューアタイトルの表示形式であり、コンポーネント境界や他タスクを縛らないため）。
4. **Chrome ウェブストア詳細説明文のリポジトリ保管:** コメント 5 の日英確定案を本タスクでリポジトリ内の文書として保存する。
   - **理由:** 確定稿が issue コメントに散逸するのを防ぎ、将来の Chrome ウェブストア申請 issue でそのまま参照・版管理できるようにするためである。
   - **既存方針との関係:** `CLAUDE.md`「Document language」の Exceptions「The bilingual README」および [ADR-0035](../../dev/adr/0035-language-policy.md) は、日英の 2 ファイル対で機械同期する文書を `README.md` / `README.en.md` のみと定めている。ストア掲載文は `/mktrans` による差分同期の対象外とし、日英の確定稿を 1 文書で保管する位置づけとして整理する。
   - **ADR:** [ADR-0035](../../dev/adr/0035-language-policy.md) を更新する。なお、製品名方針そのものの新規 ADR は作成せず、タスク文書で記録する。
5. **README の H1 見出しと非公式・商標帰属表示:** H1 見出しは Chrome ウェブストア表示名（`# ヨムドウガ for YouTube™` ／ `# YomuDoga for YouTube™`）とし、本文ではコア製品名を使い、`README.md` と `README.en.md` にも Google LLC / YouTube 非公式である旨と `YouTube is a trademark of Google LLC.` の帰属表示を入れる。
   - **理由:** H1 で Chrome ウェブストア上の正式な拡張名と一目で対応づけられ、かつ `for YouTube™` を README に載せる以上、帰属表示と非公式表記を README 側にも揃えるほうが商標指針上も一貫するためである。
   - **ADR:** なし（決定 4 のとおり製品名方針の新規 ADR は作らないため）。
6. **UI 細部の表記と `docs/dev/guides/development.md:119`・用語集の更新:** 拡張 UI の残存箇所（拒否手順の再起動案内・オプションページタイトル・静的 HTML の `<title>`）に加え、`docs/dev/guides/development.md:119` の開発用のビルド表示名の引用と `docs/translation_glossary.md` を合わせて更新する。
   - **理由:** `development.md:119` は `chrome://extensions/` 上の実際の表示名（`extNameDev`）を引用しており、未更新のままだと実装と矛盾する。また用語集に製品名を追加することで `/mktrans` と `japrose` での訳語・表記のぶれを防げる。その他の開発側記述は `yt2column` のまま維持する。
   - **ADR:** なし（既存記述の事実整合と用語集の追加であり、主要決定ではないため）。
7. **ポップアップ（`popup.html`）の `<title>` の扱い:** 静的 HTML の `<title>` を `YomuDoga` に変更するのみとし、`popup.html` に対するスクリプトからの動的タイトル書き換えは追加しない。
   - **理由:** ポップアップの `<title>` は通常のブラウザ UI に表示されず、現行仕様からの変更を最小限に保つためである。
   - **ADR:** なし（個別画面の細部であり、本タスク内で閉じるため）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

- ブラウザ拡張の表示名・説明文・UI 文言から旧プロジェクト名 `yt2column` を除き、新製品名 `ヨムドウガ` / `YomuDoga`（および Chrome ウェブストア表示名 `ヨムドウガ for YouTube™` / `YomuDoga for YouTube™`）へ一貫して切り替えること。
- 利用者向け文書（`README.md`・`README.en.md`・`INSTALL.md`）を新製品名へ更新するとともに特徴説明を拡充し、日英の整合と既存ガードテストの通過を保つこと。
- Chrome ウェブストア詳細説明文（日英）をリポジトリで版管理できる状態にし、文書言語方針（ADR-0035・`CLAUDE.md`）との整合を取ること。

### 2.2. スコープ (In Scope)

- `extension/static/_locales/{ja,en}/messages.json` の `extName`・`extNameDev`・`extDescription` の更新。
- `extension/src/core/i18n/{ja,en}.ts` の製品名を含む UI 文言（`rejection` の再起動手順、`viewerTitle.name`、`optionsPage.title`）の更新。
- `extension/static/{popup,options,viewer}.html` の静的 `<title>` の更新。
- 上記変更に伴う関連する拡張テストの更新・追加。
- Chrome ウェブストア詳細説明文（日本語・英語）のリポジトリ内への保存、および [ADR-0035](../../dev/adr/0035-language-policy.md) と `CLAUDE.md` の更新。
- `README.md` の改訂（H1・冒頭概要・`## 特徴` 新設・製品名更新・非公式表記と帰属表示）および `/mktrans README.md` による `README.en.md` の同期。
- `INSTALL.md` の利用者向け製品名箇所の更新。
- `docs/dev/guides/development.md:119`（開発用のビルド表示名の引用）と `docs/translation_glossary.md`（製品名エントリの追加）の更新。

### 2.3. スコープ外 (Out of Scope)

- **開発側の名称の変更:** リポジトリ名、Go モジュール名、CLI バイナリ名（`yt2column`）、環境変数（`YT2COLUMN_*`）、拡張内部の識別子・ログ（`yt2column.wasm`、`wasmNamespace`、`viewerChannelName`、コンソールログのラベル、`extension/package.json` の `name`）、`development.md:119` を除く `docs/` 配下の開発者向け文書は変更しない。
- **`popup.html` に対するスクリプトからの動的タイトル書き換えの追加:** 決定 7 による。静的 `<title>` の更新のみとする。
- **Chrome ウェブストアへの実際の申請作業・アイコンやプロモーション画像の作成・プライバシーポリシーの作成:** ストア申請用の別 issue で扱う。
- **`INSTALL.md` の英語版追加と `/mktrans` の一般化:** [#1044](https://github.com/isseis/yt2column/issues/1044) で扱う。
- **ブラウザ拡張の権限・挙動・拡張 ID の変更:** 機能や権限は一切変えない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: ブラウザ拡張の manifest 表示名と短い説明文

ブラウザの拡張機能一覧および Chrome ウェブストアで使用されるロケールカタログ（`messages.json`）に、確定した製品名と短い説明文を設定する。

**Acceptance Criteria**:

- **AC-01**: `extension/static/_locales/ja/messages.json` は、`extName` に `ヨムドウガ for YouTube™`、`extNameDev` に `ヨムドウガ（開発用のビルド）`、`extDescription` に `YouTube™ 動画を、さっと読める新聞・雑誌風の日本語コラム記事に。選んだ文字起こしから、AI が読みやすく編集します。` を持つ。
- **AC-02**: `extension/static/_locales/en/messages.json` は、`extName` に `YomuDoga for YouTube™`、`extNameDev` に `YomuDoga (development build)`、`extDescription` に `Turns YouTube™ videos into readable Japanese magazine-style column articles from your selected transcript with AI.` を持つ。
- **AC-03**: 日英両ロケールの `extDescription` は 132 文字以内かつ `$` を含まず、日本語版は日本語文字を含み英語版は日本語文字を含まない。また `extension/static/manifest.json` は `name` に `__MSG_extName__`、`description` に `__MSG_extDescription__`、`permissions` に `["activeTab", "scripting", "storage"]`、`key` に拡張 ID `clfmbbcdpnjcefbdihdoahomaifbabkk` を導出する公開鍵を持つ。

#### F-002: ブラウザ拡張 UI の製品名表示

ポップアップ・ビューア・オプションページおよび静的 HTML に表示される製品名を新製品名へ統一する。

**Acceptance Criteria**:

- **AC-04**: ビューアタイトル（`viewerTitle`）は、公開用のビルドで `<タイトル部分> - ヨムドウガ`（日本語）／`<タイトル部分> - YomuDoga`（英語）、開発用のビルドで `<タイトル部分> - ヨムドウガ（開発用のビルド）`（日本語）／`<タイトル部分> - YomuDoga (development build)`（英語）を返す。
- **AC-05**: オプションページのタイトル（`optionsPage.title`）は、日本語で `ヨムドウガの設定`、英語で `YomuDoga settings` である。
- **AC-06**: ポップアップの収集拒否手順（`rejection["not-watch-page"].steps` および `rejection["empty-selection"].steps`）に含まれる再起動案内の文言は、日本語で `もう一度ヨムドウガを起動してください。`、英語で `Launch YomuDoga again.` である。
- **AC-07**: `extension/static/popup.html`・`extension/static/options.html`・`extension/static/viewer.html` の静的 `<title>` はすべて `YomuDoga` である。

#### F-003: Chrome ウェブストア詳細説明文の保管と文書言語方針の整合

Chrome ウェブストアに掲載する日本語版・英語版の詳細説明文をリポジトリで保管し、文書言語方針と整合させる。

**Acceptance Criteria**:

- **AC-08**: issue #1148 コメント 5 で確定した日本語版・英語版の Chrome ウェブストア詳細説明文（Google LLC / YouTube 非公式である旨と `YouTube is a trademark of Google LLC.` の帰属表示を含む）と、表示名・短い説明文・商標運用条件（YouTube ロゴ不使用、審査で `for YouTube™` の削除を求められた場合はコア製品名を変えず末尾のみ外す方針）がリポジトリ内の文書に保存されている。
- **AC-09**: [ADR-0035](../../dev/adr/0035-language-policy.md) および `CLAUDE.md` の「Document language」に、Chrome ウェブストア掲載文が日英両方の本文を保持しつつ `/mktrans` および `TestReadmeSyncMarker` の同期対象外であることが記録されている。

#### F-004: 利用者向け文書（`README.md`・`README.en.md`・`INSTALL.md`）の更新

利用者向け文書の製品名と概要を新製品名と issue #1148 コメント 6 の方針に沿って更新する。

**Acceptance Criteria**:

- **AC-10**: `README.md` の H1 見出しは `# ヨムドウガ for YouTube™` であり、冒頭の概要文はコア製品名 `ヨムドウガ` を用いて、話し言葉の脱線や重複を整理し編集・校正まで行って新聞・雑誌風の日本語コラム記事に仕立てることを伝える。
- **AC-11**: `README.md` に `## 特徴` 節を新設し、issue #1148 コメント 6 の 4 要点（結論が先・重要度順の構成、動画に合った 4 形式への編集と文字起こし突き合わせチェック、非自動操作による画面変更への強さ、押下時のみの読み取り・中継サーバなし・オープンソース）を簡潔にまとめる。ストア向けの「こんな方に向いています」や、既存節と重複する「使い方（3 ステップ）」「注意点」は追加しない。
- **AC-12**: `README.md` の既存の節（`## 仕組み`・`## 必要なもの`・`## インストールと使い方`・`## プライバシー`・`## 開発者向けの文書`）の構成と必要なブラウザの表記（Chrome 142 以降）を維持し、利用者向けの製品名呼称（ツールバーのアイコン表記等）を `ヨムドウガ` に更新する。また、Google LLC / YouTube 非公式である旨と `YouTube is a trademark of Google LLC.` の帰属表示を記載する。
- **AC-13**: `README.en.md` は `/mktrans README.md` で `README.md` と同期されており（`TestReadmeSyncMarker` と見出し・コードブロック構造の一致を満たす）、H1 見出しが `# YomuDoga for YouTube™`、本文のコア製品名が `YomuDoga` であり、非公式・商標帰属表示と Chrome 142 以降の表記を持つ。
- **AC-14**: `INSTALL.md` は、冒頭（`INSTALL.md:3`）に `ヨムドウガ`（および Chrome ウェブストア表示名 `ヨムドウガ for YouTube™`）、ツールバー固定およびアンインストール手順（`INSTALL.md:80`・`:241`）に拡張機能一覧での表示名 `ヨムドウガ for YouTube™`、ビューアタイトルの説明（`INSTALL.md:201`）に `<動画のタイトル> - ヨムドウガ` を記している。また、Releases の URL（`https://github.com/isseis/yt2column/releases`）、Chrome 142 以降の表記、および日本語カタログと一致する画面の引用文言を維持している。

#### F-005: 開発手順の事実整合と用語集の更新

拡張の実表示名を引用する開発手順の 1 箇所と、翻訳・日本語推敲の基準である用語集を更新する。

**Acceptance Criteria**:

- **AC-15**: `docs/dev/guides/development.md` の差分は、開発用のビルドの拡張機能ページ表示名を引用する 1 行（`development.md:119`）を `ヨムドウガ（開発用のビルド）` および `YomuDoga (development build)` に書き換えることのみに限る。
- **AC-16**: `docs/translation_glossary.md` にコア製品名（`ヨムドウガ` / `YomuDoga`）および Chrome ウェブストア表示名（`ヨムドウガ for YouTube™` / `YomuDoga for YouTube™`）の行が追加されている。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

N/A（表示文言と文書の更新のみであり、実行性能や通信量に影響しない）。

### 4.2. セキュリティ (Security)

N/A（外部コマンドの起動、API キーの扱い、キャッシュの読み書き、ネットワーク送信、信頼できないテキストのプロンプト・記事への埋め込みのいずれも変更しないため、`docs/dev/security.md` の適用条件に該当しない）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

N/A（実行ロジックやエラー処理の制御フローを変更しない）。

### 4.4. 互換性 (Compatibility)

- `extension/static/manifest.json` の `key` を維持することで、既存の拡張 ID（`clfmbbcdpnjcefbdihdoahomaifbabkk`）および `chrome.storage.local` に保存済みの API キー・ワークスペース ID の引き継ぎを保つ（AC-03）。
- CLI・環境変数（`YT2COLUMN_*`）・拡張内部の識別子やログを維持することで、開発者向けツールの互換性を保つ。
- **アクセシビリティ（`CLAUDE.md`「Accessibility」、WCAG 2.2 AA）:** 本タスクは製品名文字列と静的 `<title>` の差し替えのみであり、以下の 9 項目は変更対象外（N/A）として既存実装および既存テスト（`popupHtml.test.ts`・`optionsHtml.test.ts`・`style.test.ts` 等）をそのまま維持する。あわせて静的 `<title>` および動的ページタイトルを非空に保つ（WCAG 2.4.2）。
  - **Keyboard（2.1.1・2.1.2・2.4.3）:** DOM 構造・操作要素・フォーカス順を変更しないため N/A。
  - **Visible focus（2.4.7・1.4.11）:** CSS およびフォーカスの表示を変更しないため N/A。
  - **Names, roles, and states（1.3.1・2.5.3・4.1.2）:** コントロールの要素・ロール・状態属性・ボタンラベルを変更しないため N/A。
  - **Status messages（4.1.3）:** ライブリージョン（`role="status"`）の構成を変更しないため N/A。
  - **Contrast（1.4.3・1.4.11）:** 配色およびテーマ CSS を変更しないため N/A。
  - **Not by color alone（1.4.1）:** 状態の伝え方を変更しないため N/A。
  - **Language（3.1.1）:** 静的 HTML の `lang="en"` および `applyPageLocale` による `lang` 切替を変更しないため N/A。
  - **Target size（2.5.5）:** インタラクティブ要素のサイズ・CSS を変更しないため N/A。
  - **Motion（2.3.3）:** アニメーションおよび `prefers-reduced-motion` の指定を変更しないため N/A。

### 4.5. 保守性 (Maintainability)

- 拡張のメッセージカタログは英語・日本語の両方に同じ型で定義する方針（[ADR-0029](../../dev/adr/0029-extension-ui-languages.md)）を維持する。
- `README.md` と `README.en.md` の同期は `/mktrans README.md` と `TestReadmeSyncMarker` により機械的に検証する（[ADR-0035](../../dev/adr/0035-language-policy.md)）。

## 5. 制約条件 (Constraints)

- `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」および `CLAUDE.md` の「Document language」「Language」「Go Idioms」「Accessibility」に従う。
- `CLAUDE.md`「Document language」原則 (c) に従い、`README.md`・`README.en.md`・`INSTALL.md` は一般の利用者向けにです・ます調の平易な表現で記述する。
- ブラウザ拡張のソースコード内のコメント・識別子・ログ文字列は英語を維持し、日本語リテラルは `extension/src/core/i18n/ja.ts`、`extension/static/_locales/ja/messages.json`、および `CLAUDE.md`「Go Idioms」が認めるガードテストの例外範囲に限る。

## 6. 用語集 (Glossary)

既存の用語は `docs/translation_glossary.md` に従う。本タスクで追加・確定する製品名関連の用語は次のとおり。

- **コア製品名:** `YouTube` を含めない独自名。日本語 `ヨムドウガ`、英語 `YomuDoga`。拡張 UI・ビューアタイトル・README 本文での呼称に使う。
- **Chrome ウェブストア表示名:** `manifest.json` の `name`（`extName`）および Chrome ウェブストア・`chrome://extensions/`・README の H1 見出しで使う表示名。日本語 `ヨムドウガ for YouTube™`、英語 `YomuDoga for YouTube™`。
- **開発用のビルド表示名:** `make ext-build-dev` が生成する開発用のビルドの `extNameDev`。日本語 `ヨムドウガ（開発用のビルド）`、英語 `YomuDoga (development build)`。
