# 実装計画書：テキストが選択されていないとき、文字起こしパネルの DOM から字幕を読み取る

## Document Status

| Item | Value |
|---|---|
| Status | `withdrawn` |
| Created | 2026-10-20 |
| Review date | 2026-10-20 |
| Reviewer | isseis |
| Comments | 本タスクの取り下げにより `withdrawn` にした（2026-10-20。[observation_gate_review.md](observation_gate_review.md)）。フェーズ 1〜5 は実装しない |

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

[01_requirements.md](01_requirements.md)（以下、要件書）で定義した、ポップアップを開いたときに選択範囲が空なら文字起こしパネルの DOM から字幕を読み、送る内容にする機能を、[02_architecture.md](02_architecture.md)（以下、設計書）のとおりに実装する。具体的には、次のことを行う。

-   実際の YouTube の動画ページで観測のゲート（設計書 8.1）を行い、結果を設計書 3.1 に記録する。
-   字幕の読み取りの型、受理する形の検査（`parseTranscriptRead`）、取得元の選択と拒否の理由・拒否の詳細への対応づけを、`extension/src/core/` に作る（設計書 3.2・3.4・3.5）。
-   選択範囲を読む既存の注入を、選択範囲が空のときだけパネルを読む `readTranscript` に置き換える（設計書 3.3）。
-   要約に取得元・末尾のプレビュー・理由のコードを加え、メッセージを改める（設計書 3.6・3.7）。
-   README・`docs/dev/security.md`・`docs/dev/project_overview.md`・用語集を改める（要件書 F-007）。
-   Chrome と Brave で手動の確認を行い、結果を本計画の §5.1 に記録する（要件書 F-006）。

本計画が使う記号は次のとおりである。AC-NN は要件書の受け入れ基準、F-NNN は要件書の機能要件、H-NN は [design_handoff.md](design_handoff.md) の項目を指す。「§N」は本計画の節、「設計書 N」は設計書の節を指す。

本計画は、設計書 8 章を次のようにフェーズへ対応させる。8.1（観測のゲート）はフェーズ 0、8.2 の項目 1（`core/` の型と判定）はフェーズ 1、項目 2（注入と配線）はフェーズ 2、項目 3（表示）はフェーズ 3、項目 4（文書）はフェーズ 4、項目 5（手動の確認）はフェーズ 5 に対応する。

### 1.2. 実装原則

-   設計書 1.1 の設計原則に従う。特に、次の 5 点を守る。選択範囲が空白文字だけでないときは、パネルの DOM に触れない。DOM の知識は注入の中に閉じ込める。判定できない状態は、字幕を使わない側に倒す。ページを操作しない。取得元・拒否の理由・拒否の詳細は列挙の値で決め、字幕の文字列の内容から推し量らない。
-   **フェーズ 0 の結果が出るまで、コードを書かない。** 設計書 3.1.3（H-04）と 8.1 は、SPA の画面遷移の観測の結果によらず設計書を `draft` に戻すと定めている（結びつける信号が見つかれば照合を加え、見つからなければ要件のレビューに諮る）。したがって、フェーズ 1〜5 は、現在承認されている設計書に基づく暫定の計画である。設計書が改めて承認されたら、フェーズ 1 を始める前に、本計画をその設計書に合わせて改める（ステップ 0-7）。
-   Go の本体のソースと manifest を変更しない（設計書 1.3・3.12）。Go で変更するのは、文書を検査する `cmd/yt2column` のテストだけである。
-   拡張のソースのコメント・識別子・文字列リテラル・利用者に表示する文言は英語で書く（`_context.md` の「Source-language rule」、CLAUDE.md）。`AC-NN`・`F-NNN`・`H-NN` はソースに書かず、本計画にだけ記録する（`requirements_process.md` §4）。
-   テストの `describe` の名前は計画上の名前である。実装で変える場合は、§5 を同じコミットで更新する。
-   各テストは、対象の処理を実際に壊して失敗することを確かめ、そのことをコミットメッセージに書く（CLAUDE.md「Testing Strategy」）。壊す対象は各フェーズの最後のステップに挙げる。
-   各フェーズの完了条件は、`make ext-check` → `make test` → `make lint` が通ることである。フェーズ 1〜3 は Go を変えないが、`_context.md` の Green gate（`make test && make lint`）を PR ごとに通すために実行する。

### 1.3. 既存コード調査結果

調査はコミット `39b488a` で行った。本節の `file:line` はこのコミットの行番号を指す。設計書の `file:line` はコミット `9698b97` のもので、その後 0026 が `extension/src/core/sendMessages.ts`・README・`docs/dev/security.md` を変えたので、これらの行番号は設計書と異なる。

#### 1.3.1. 名前の変更の台帳

本計画の名前の変更はすべてこの表に従う。ほかの節の記述はこの表から導く。

| 種類 | 旧 | 新 | 場所 |
|---|---|---|---|
| 型 | `PageSelection` | `TranscriptRead` | `extension/src/core/collect.ts:30-33` |
| メンバ | `PageSelection.text` | `TranscriptRead.selection` | 同上 |
| 型 | `SelectionReader` | `TranscriptReader` | `extension/src/core/collect.ts:36-39` |
| メンバ | `CollectedInput.selection` | `CollectedInput.transcript` | `extension/src/core/collect.ts:11` |
| 関数（注入） | `readSelection` | `readTranscript` | `extension/src/browser/chromeDeps.ts:35-40` |
| 関数 | `isPageSelection` | 削除（`parseTranscriptRead` が代わる） | `extension/src/browser/chromeDeps.ts:43-51` |
| 関数 | `createSelectionReader` | `createTranscriptReader` | `extension/src/browser/chromeDeps.ts:57-74` |
| テストの `describe` | `selection reader` | `transcript reader` | `extension/test/chromeDeps.test.ts:48` |

古い名前が出現するのは、次の箇所だけである（検索は `rg -n "PageSelection|SelectionReader|createSelectionReader|readSelection|isPageSelection"`、`docs/tasks/` を除く。`39b488a` で実行）。`extension/src/browser/chromeDeps.ts`・`extension/src/core/collect.ts`・`extension/src/launch.ts:2`・`:39`・`extension/src/popup.ts:8`・`:20`・`extension/test/collect.test.ts`・`extension/test/chromeDeps.test.ts`・`extension/test/launch.test.ts:7-8`・`:69-76`。README・`docs/dev/` には出現しない。型と関数の名前の残りは `make ext-check` の型検査で見つかるが、コメント（例: `extension/src/browser/chromeDeps.ts:54` の `The SelectionReader backed by chrome.scripting`）とテストの `describe` の名前は見つからないので、ステップ 2-5 で検索して確かめる。

#### 1.3.2. 変更するソース

| ファイル | 既存のもの | 変えること |
|---|---|---|
| `extension/src/core/types.ts` | `RejectionReason` と `rejectionReasons`（`:2-18`）、`OutcomeSummary`（`:28-40`） | 設計書 3.5・3.6 の型を加える。`OutcomeSummary` の拒否の `detail` は `core/collect.ts` の `PanelUnavailableReason` を参照するので、`core/types.ts` から `core/collect.ts` への型だけの import が加わる（設計書 2.2。本計画の作成時に設計書へ編集上の訂正として記録した） |
| `extension/src/core/collect.ts` | `CollectedInput`（`:10-15`）、`CollectOutcome`（`:17-27`）、`whitespaceOnly`（`:45`）、`collect`（`:53-94`） | 設計書 3.2・3.4・3.5 のとおり。`CollectedInput` を作るのは `collect` だけで、`extension/eslint.config.js:210-221` の規則が、`src/` の `collect.ts` 以外のファイルで `CollectedInput`（ブランドを持つ型）への型の表明を禁じる。取得元は型の値（`origin`）で表し、具象の型で振り分けないので、構築の経路を新たに閉じる必要はない |
| `extension/src/core/messages.ts` | `rejectionMessage`（`:31-63`） | 設計書 3.7 の文言（ステップ 1-3・2-2） |
| `extension/src/core/sendMessages.ts` | `blockMessage` の委譲（`:13-17`）、`body-too-large`（`:52-56`）、`body_too_large`（`:77-84`）、`invalid_transcript`（`:95-102`） | 設計書 3.7 の委譲と文言（ステップ 1-4） |
| `extension/src/core/summary.ts` | `truncate`（`:13-22`）、`previewLimit = 200`（`:6`）、`summarize`（`:35-54`） | `truncateEnd` を加え、`summarize` に取得元・末尾のプレビュー・拒否の詳細を加える。`truncate` と同じくコードポイントで数える |
| `extension/src/core/prepare.ts` | `buildArticleBody`（`:32-49`。`:38` で `input.selection` を読む） | `input.transcript` を読む。振る舞いは変えない |
| `extension/src/browser/chromeDeps.ts` | §1.3.1 の 3 つの関数、`createMetadataReader`（`:127-142`） | 設計書 3.3。`createMetadataReader` は変えない |
| `extension/src/ui/render.ts` | `field`（`:21-40`）、`collectedNodes`（`:43-68`）、`rejectedNodes`（`:90-95`）、`displayValue`（`:11-13`。末尾に `…` を付ける） | Source・End の欄と理由のコード（設計書 3.6） |
| `extension/static/style.css` | Preview の規則（`:33-39`・`:43-46`） | `summary-end-preview` を同じ規則に加える |
| `extension/src/launch.ts` | `PopupLaunchDeps.reader`（`:39`） | 型を `TranscriptReader` にする |
| `extension/src/popup.ts` | `createSelectionReader(chrome.scripting)`（`:20`） | `createTranscriptReader` にする |

#### 1.3.3. 更新するテストと、設計書 3.10 に挙がっていない箇所

`extension/tsconfig.json` は `test/` も型検査するので、フェーズ 1 の型の変更で壊れるテストは、すべてフェーズ 1 で改める（ステップ 1-8）。設計書 3.10 の表に挙がる箇所に加えて、次の箇所も更新が要る。いずれもフィクスチャの組み立てで、検査する内容は変えない。

-   `extension/test/launch.test.ts`: `:68-76` の `page()`・`readerReturning`（`TranscriptRead` にする）、`:121-126` の `collectedOutcome`（`{ selection, ... } as CollectedInput`）、`:250` の `popupOutcome.input.selection`、`:315-333` の拒否の一覧（`page()` の形の変更に合わせる）、`:588-592` の `launched({ kind: "rejected", reason: "empty-selection" })`（`detail` が要る）。
-   `extension/test/summary.test.ts:11-19` の `collected` と、`:93-107`「reduces a collected input to exactly the nine summary members」（`origin` が加わるので、メンバの数を含む名前と期待値を改める）と、`:109-125`「drops the cause of every rejection, keeping only kind and reason」。拒否の要約が `detail` を持つようになるので、`detail` を持つ理由と持たない理由に分けて書き直す。`:127-136` の `empty-selection` の拒否の組み立てにも `detail` が要る。
-   `extension/test/render.test.ts:11-21` の `collected`（`origin` が要る）。`:88` の `{ kind: "rejected", reason: "empty-selection" }` は、要約の `detail` が任意なので型の上はそのままでよい。
-   `extension/test/prepare.test.ts:22-28` の `collectedInput` と、`:146-150` の `empty-selection` の拒否（`detail` が要る）。
-   設計書 3.10 に挙がる箇所のうち、行を示していないもの: `extension/test/send.test.ts:35-38`（`{ selection: "hello", ... } as CollectedInput`）、`extension/test/collect.test.ts` の `page()`・`readerReturning`・`readingReader`（`:16-40`）、`:83`・`:175` の `SelectionReader`、`:121` の `outcome.input.selection`、`:194` の `deepEqual`。
-   `extension/test/renderSend.test.ts:86` は `BlockReason` として `empty-selection` を使うだけで、変更は要らない。

既存のテストで再利用するもの:

-   `extension/test/chromeDeps.test.ts:29-46` の `recordingScripting`。注入の記録と結果の差し替えに使う。
-   `extension/test/chromeDeps.test.ts:184-197` の `run` の方法（記録した関数のソースを `vm.runInNewContext` で jsdom の `document`・`location` を渡し、JSON にした引数 1 つで実行する）。`run` は `describe("page metadata injection")` の中の関数で、`window` を渡さず引数も 1 つなので、ファイルの直下に移し、`window`（`getSelection` と、AC-10 で確かめる jsdom の `Selection`）と任意の数の引数を渡せる形に広げて、2 つの注入で共有する（設計書 7.1）。
-   `extension/test/chromeDeps.test.ts:280-302`「does not operate the page」の、直列化した DOM の比較と、ページを操作する呼び出しのソースの走査。字幕の読み取りの注入にも同じ一覧を使うので、一覧をファイルの中の定数に移して 2 つの注入で共有する。**注意:** 一覧の `.value =` は、比較の `.value ===` にも部分一致する。`readTranscript` は検索欄の `value` を読むので、比較の書き方と走査の規則のどちらを変えるかを実装時に決める。どちらにしても、注入のソースに代入を入れたときに走査が失敗することを確かめる。
-   `extension/test/helpers/repoFile.ts` の `readRepoFile`・`markdownSection`（文書とソースの読み取り）、`extension/test/helpers/logger.ts` の `recordingLogger`、`extension/test/helpers/dom.ts` の `container`・`valueOf`・`valueElement`。
-   `extension/test/metadataDocs.test.ts:8-22` の `flatSection`・`expectPhrases`。新しい `transcriptDocs.test.ts` でも使うので、`extension/test/helpers/repoFile.ts` へ移して 2 つのファイルで共有する。
-   `cmd/yt2column/extension_docs_test.go:630-664` の `checkManualCheckTable` と `:673-679` の `acLabel`。§5.1 の記録の検査に使う。`cmd/yt2column/docs_test.go:313` の `readDoc` と `:464` の `sectionByHeading`。

拡張のテストは `testdata/` を読まない。拡張の CI の変更の判定（`extension/scripts/has-extension-changes.sh`）は `testdata/` を対象にせず、設計書 3.12 はこの判定を変えないと定めている。そのため、スピナーだけの開いた状態の DOM（AC-07）は、`testdata/2tcCWM-sRBw.transcript_panel_hidden.html:4-48` の `yt-content-loading-renderer` と `tp-yt-paper-spinner` の構成を最小限に写して、テストの中で組み立てる。設計書 7.1 の「`transcript_panel_hidden.html` の `visibility` を EXPANDED に替えた形を使う」は、この形を使うことと読む（同じ形をファイルから読むか写すかの違いで、確かめる内容は変わらない）。

#### 1.3.4. 注入の数（AC-05 の静的な確認の起点）

`grep -rn "executeScript(" extension/src` を `39b488a` で実行した結果は次の 2 行である。

```
extension/src/browser/chromeDeps.ts:62:      const results = await scripting.executeScript({
extension/src/browser/chromeDeps.ts:130:      const results = await scripting.executeScript({
```

本タスクの後も 2 つ（字幕の読み取りと動画メタデータの読み取り）であることを、`chromeDeps.test.ts` のテストで確かめる（ステップ 2-3）。

#### 1.3.5. フェーズ 1 の間の暫定の読み取り

フェーズ 1 で `collect` の引数が `TranscriptReader` になると、`chromeDeps.ts` の既存の注入（選択範囲だけを読む）がそのままでは型に合わない。フェーズ 1 の完了条件（`make ext-check`）を満たすため、フェーズ 1 では `createSelectionReader` を残し、既存の注入の結果を `TranscriptRead` に変換する暫定の処理を置く。選択範囲が `whitespaceOnly` に一致するときは `panel` を `absent`、一致しないときは `skipped` とし、`parseTranscriptRead` に通す。この間、選択範囲が空なら、パネルが開いていても `empty-selection` で拒否し、ログの `detail` は常に `panel-absent` になる（パネルを読まないため）。パネルを開けば選択しなくてよいと案内する `empty-selection` の文言は、パネルを読めるようになるステップ 2-2 まで変えない。この暫定の処理は、ステップ 2-1 で `createTranscriptReader` に置き換えて削除する。フェーズ 1（PR-3）とフェーズ 2（PR-4）は別の PR なので（§3.2）、その間の `main` では、ログの `detail` だけが正確でない。

#### 1.3.6. 並行するタスク

0026 の PR-8（`docs/tasks/0026_native_messaging_relay/03_implementation_plan.md` の「PR-8 作成ポイント」。本計画の作成時点で未着手）は、拡張の送信を Native Messaging に移す。本タスクと重なるのは次のものである。

-   `prepare.ts`（`prepareSend` が設定を読まない同期の関数になる。0026 ステップ 3-4）と `prepare.test.ts`。
-   `sendMessages.ts` の文言の表（0026 ステップ 3-6）と `sendMessages.test.ts`。
-   `chromeDeps.ts`・`popup.ts`・`launch.ts` の送信と設定の部分。
-   manifest と `manifest.test.ts`（ホストの権限と `storage` の削除。0026 ステップ 3-1）。
-   README の「Browser extension」節（`README.md:594-599` の「Transitional note」）と `docs/dev/security.md` §8.1（`:104` の移行中の注記）。

本タスクが変えるのは、`CollectedInput` のフィールド名、拒否の理由の委譲、選択を前提にした文言、注入の置き換え、文書の記載だけで、送信の経路と manifest は変えない。後からマージする側が衝突を解消する（§6.1）。

### 1.4. 申し送りの項目への対応

-   `implementation_handoff.md` はない。
-   `design_handoff.md` の H-01〜H-08 は、すべて設計書 3.11 に記録されている。本計画で実装に関わるもの: H-01・H-02・H-03・H-04 はフェーズ 0 で観測を確かめ、ステップ 2-1 で実装する。H-05 はステップ 2-3、H-06 はステップ 1-3・1-4・2-2・3-1、H-07 はステップ 1-2・2-3・2-4、H-08 はステップ 1-2・2-1 で扱う。

## 2. 実装ステップ (Implementation Steps)

各ステップの末尾に **対象:**（そのステップが触るファイル）と **完了:**（そのステップの検証）を記す。フェーズの「完了」は、そのフェーズのすべてのステップの後に確かめる。

### フェーズ 0: 観測のゲート（設計書 8.1）

**対象ファイル:** `docs/tasks/0029_transcript_panel_read/02_architecture.md`（3.1 に記録を加える）、本計画

Chrome と Brave の安定版で観測する。各記録に、動画 ID、ブラウザとその版、観測した日付、ログインの状態、表示言語を含める。観測用のスクリプトはリポジトリに入れない。ページ全体のスナップショットはリポジトリに保存しない（`docs/dev/security.md` §7）。

-   [x] **ステップ 0-1**: 形 B のページ（例: `Yoboo9g7saY`）でパネルを開き、パネルの `target-id`、セグメントの一覧・検索欄・読み込み中を示す要素を記録する（設計書 8.1 の表の 1 行目）。
    -   **対象:** なし（観測。記録はステップ 0-6 で設計書 3.1 に書く）。**完了:** Chrome と Brave の両方について、パネルの `target-id`、セグメントの一覧・検索欄・読み込み中を示す要素の記録が、動画 ID・ブラウザとその版・日付・ログインの状態・表示言語とともに作業メモにある。
-   [x] **ステップ 0-2**: 検索欄に文字を入れ、`textarea` の `value` と一覧の変化を記録する（2 行目）。
    -   **対象:** なし（観測。記録はステップ 0-6 で設計書 3.1 に書く）。**完了:** Chrome と Brave の両方について、`textarea` の `value` と一覧の変化の記録が、動画 ID・ブラウザとその版・日付・ログインの状態・表示言語とともに作業メモにある。
-   [-] **ステップ 0-3**:（2026-10-20、Brave で 1 回目の計測の途中で中断。2 時間を超える動画 `l8pRSuU81PU` では、設計書の 2 つの版ではなく `target-id` を持たないパネルが開いた。Chrome は未実施。設計書 3.1.4） 2 時間を超える動画でパネルを開き、DOM のセグメントの数と最後の時刻を動画の長さと比べる。パネルをスクロールする前と後で比べる（3 行目）。
    -   **対象:** なし（観測。記録はステップ 0-6 で設計書 3.1 に書く）。**完了:** Chrome と Brave の両方について、スクロールの前と後のセグメントの数・最後の時刻と動画の長さの記録が、動画 ID・ブラウザとその版・日付・ログインの状態・表示言語とともに作業メモにある。
-   [-] **ステップ 0-4**:（未実施。0-1・0-2 で 2 つの版のどちらにも結びつける信号がなかったので、試行によらず要件のレビューに諮ることになった。設計書 3.1.4） 設計書 8.1「SPA の画面遷移の手順」のとおり、観測用のスクリプトで、遷移の方法 5 種 × 回線 2 種 × 各 3 回（各ブラウザ 30 回）の試行を記録する。失敗の定義（判定が「読める」なのに前の動画のセグメントである時点が 1 つでもある）に当たる試行の有無と、パネルの子孫の要素に中身と動画を結びつける信号があるかを記録する。遷移の方法ごとに数回、遷移の直後に手でポップアップを開き、そのときの判定をスクリプトの記録と照らす（設計書 8.1 の「実際のポップアップの経路」）。この時点ではパネルを読むコードがまだないので、見るのはポップアップを開く操作の間のパネルの状態であり、拡張の読み取りではない。拡張の読み取りはステップ 5-1 で確かめる。
    -   **対象:** なし（観測。記録はステップ 0-6 で設計書 3.1 に書く）。**完了:** Chrome と Brave の両方について、30 回の試行ごとの結果、失敗の定義に当たる試行の有無、結びつける信号の有無、手で開いたポップアップの判定との照合の記録が、動画 ID・ブラウザとその版・日付・ログインの状態・表示言語とともに作業メモにある。
-   [-] **ステップ 0-5**:（未実施。0-3 の途中で観測を中断したため） ログインした状態としていない状態、日本語と英語の表示、チャプターのある動画、シアターモード・狭いウィンドウで、パネルの要素の構成と読んだ文字列を記録する（5 行目）。
    -   **対象:** なし（観測。記録はステップ 0-6 で設計書 3.1 に書く）。**完了:** Chrome と Brave の両方について、挙げた各条件でのパネルの要素の構成と読んだ文字列の記録が、動画 ID・ブラウザとその版・日付・ログインの状態・表示言語とともに作業メモにある。
-   [x] **ステップ 0-6**:（0-1〜0-3 の途中までの結果で行った。記録と行ごとの判定は設計書 3.1.4、要件のレビューへの提案は [observation_gate_review.md](observation_gate_review.md)） 0-1〜0-5 の結果を設計書 3.1 に加え、設計書 8.1 の表の「本書を `draft` に戻す条件」に当たるかを行ごとに記す。SPA の画面遷移（0-4）は、信号の有無によらず設計書を `draft` に戻す（設計書 3.1.3）。信号がない場合、またはどれかの試行で前の動画の字幕を収集した場合は、要件のレビューに諮ることも記す。設計書の Status を `draft` にし、Comments に理由を書く。`make test`（設計書を読む文書の検査を含む）を通してからコミットする。
    -   **対象:** `docs/tasks/0029_transcript_panel_read/02_architecture.md`（3.1 と Document Status）。**完了:** 設計書 3.1 に 0-1〜0-5 の記録と、設計書 8.1 の表の行ごとの判定があり、設計書の Status が `draft` で Comments に理由があり、`make test` が通る。

### PR-1 作成ポイント: transcript panel observation gate

**対象ステップ**: 0-1 / 0-2 / 0-3 / 0-4 / 0-5 / 0-6

**推奨タイトル**: `docs(0029): record the transcript panel observation gate`

**レビュー観点**: 設計書 3.1 の各記録に動画 ID・ブラウザとその版・日付・ログインの状態・表示言語があり、Chrome と Brave の両方がそろっていること / SPA の画面遷移の 30 回の試行の結果と、失敗の定義・結びつける信号の有無の記録が、設計書 8.1 の手順に合っていること / 設計書 8.1 の表の行ごとの判定と、設計書を `draft` に戻す理由・要件のレビューに諮るかの記述 / ページ全体のスナップショットと観測用のスクリプトがリポジトリに入っていないこと（`docs/dev/security.md` §7）

**実装モデル要件**: frontier-required

**判定理由**: ステップ 0-1〜0-6 は、コードを書く前に実際の YouTube の DOM で設計の前提を確かめる観測のゲート（Phase 0 型の実現性の調査）で、結果から設計書 8.1 の各行の判定を下す前例のない判断を含む。ブラウザでの観測は人が行い、この区分は観測用のスクリプトとステップ 0-6 の判定の起草に当てはまる。

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した（https://github.com/isseis/yt2column/pull/384）
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 0（続き）: 本計画の改訂

-   [ ] **ステップ 0-7**: 設計書が改めて承認された後、本計画のフェーズ 1〜5 を改めた設計書に合わせる（例: 結びつける信号の照合をステップ 2-1・2-3 に加える）。PR の境界（§3.2、各 PR の作成ポイント、§7）も、改めたステップに合わせて導き直す。タスクが変わる場合は本計画を `draft` に戻して承認を受ける。設計書の承認と本計画の承認が揃うまで、フェーズ 1 を始めない。
    -   **対象:** 本計画（フェーズ 1〜5）。**完了:** 改めた設計書に本計画のフェーズ 1〜5 が合っており、設計書と本計画がともに `approved` である。

**完了:** 設計書 3.1 に観測の記録があり、要件書・設計書・本計画がすべて `approved` である（要件のレビューで要件書が改められた場合を含む）。

### PR-2 作成ポイント: plan alignment with the revised architecture

**対象ステップ**: 0-7

**推奨タイトル**: `docs(0029): align the implementation plan with the revised architecture`

**レビュー観点**: 改めた設計書の変更点（例: 結びつける信号の照合）が、フェーズ 1〜5 のステップ・§5 の AC の対応・§3.2 の PR 構成に漏れなく反映されていること / PR の境界を `/mkplan2` の分析で導き直し、各作成ポイントの **対象ステップ**・**実装モデル要件**・**判定理由** と §3.2・§7 が改めたステップに合っていること / ステップの番号を変えた場合に、本計画の中のステップの参照が改まっていること / タスクが変わる場合に本計画を `draft` に戻していること

**実装モデル要件**: frontier-recommended

**判定理由**: ステップ 0-7 は、§1.2 の分岐（結びつける信号の照合を加えるか、要件のレビューの判断に合わせるか）のどちらになるかが PR-1 の結果まで決まらず、方針が未確定のステップである。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 1: `core/` の型と判定（設計書 8.2 の 1）

**対象ファイル:** `extension/src/core/types.ts`・`collect.ts`・`messages.ts`・`sendMessages.ts`・`summary.ts`・`prepare.ts`、`extension/src/launch.ts`・`extension/src/browser/chromeDeps.ts`（§1.3.5 の暫定の処理）、`extension/test/collect.test.ts`・`messages.test.ts`・`sendMessages.test.ts`・`summary.test.ts`・`prepare.test.ts`・`render.test.ts`・`send.test.ts`・`launch.test.ts`・`chromeDeps.test.ts`

-   [ ] **ステップ 1-1**: `core/types.ts` に設計書 3.5・3.6 の型を加える（`TranscriptOrigin`・`EmptySelectionDetail`・`OriginSummary`、`RejectionReason` と `rejectionReasons` の `transcript-unavailable`、`OutcomeSummary` の取得元と `detail`）。`PanelUnavailableReason` は `core/collect.ts` から `import type` で参照する。`RejectionReason` の `empty-selection` のコメントを `"empty-selection" // the selection is empty or whitespace only` から `"empty-selection" // the selection is blank and no open transcript panel was found` に改め、`transcript-unavailable` に設計書 3.5 のコメントを付ける。`collection-failed` のコメントは変えない。
    -   **対象:** `extension/src/core/types.ts`。**完了:** 記したとおりの変更がある。型検査は、暫定の処理が入るステップ 1-7 の後にまとめて確かめる（それまでは本番のコードが型に合わない）。
-   [ ] **ステップ 1-2**: `core/collect.ts` を設計書 3.2・3.4・3.5 のとおりにする。§1.3.1 の台帳の名前の変更、`PanelReadStep`・`PanelDomReason`・`PanelUnavailableReason`・`PanelRead`、許可する値の `Record`（`dom-error-<段階>` の 6 つを 1 つずつ列挙）、`parseTranscriptRead`、`CollectedInput` の `transcript`・`origin`、`CollectOutcome` の拒否の変種、`collect` の判定（設計書 3.5 の表の順 1〜11）。`collect` の文書コメント（`:47-52`）の順序の記述を、表の順序に合わせて改める。
    -   **対象:** `extension/src/core/collect.ts`。**完了:** 記したとおりの変更がある。型検査は、暫定の処理が入るステップ 1-7 の後にまとめて確かめる（それまでは本番のコードが型に合わない）。
-   [ ] **ステップ 1-3**: `core/messages.ts` の文言を改める。`empty-selection` の文言はステップ 2-2 で改める（§1.3.5）。
    -   `transcript-unavailable`（新規）: 文と 3 つの手順を、設計書 3.7 の表の `transcript-unavailable` の行のとおりにする。
    -   `collection-failed`: 文を `"The selection could not be read from this tab (the page may have changed)."` から `"The page could not be read from this tab (the page may have changed)."` にする。
    -   **対象:** `extension/src/core/messages.ts`。**完了:** 記したとおりの変更がある。型検査は、暫定の処理が入るステップ 1-7 の後にまとめて確かめる（それまでは本番のコードが型に合わない）。
-   [ ] **ステップ 1-4**: `core/sendMessages.ts` を改める。`blockMessage` の委譲の `case` に `"transcript-unavailable"` を加える。`body-too-large` の文を `"The selected text is too large (over 4 MiB)."` から `"The text to send is too large (over 4 MiB)."` に、手順を `"Select fewer caption lines and try again."` から `"Select fewer caption lines in the transcript panel and try again."` にする。`body_too_large` の 1 つ目の手順を同じく `"Select fewer caption lines in the transcript panel and try again."` にする（文と 2 つ目の手順は変えない）。`invalid_transcript` の文を `"The selected text is not in the transcript panel's caption format."` から `"The text sent is not in the transcript panel's caption format."` にする（手順は変えない）。
    -   **対象:** `extension/src/core/sendMessages.ts`。**完了:** 記したとおりの変更がある。型検査は、暫定の処理が入るステップ 1-7 の後にまとめて確かめる（それまでは本番のコードが型に合わない）。
-   [ ] **ステップ 1-5**: `core/summary.ts` に `truncateEnd` を加え、`summarize` を設計書 3.6 のとおりにする（取得元ごとの要約、`panel` のときの末尾 200 コードポイント、拒否の `detail` の引き継ぎ）。
    -   **対象:** `extension/src/core/summary.ts`。**完了:** 記したとおりの変更がある。型検査は、暫定の処理が入るステップ 1-7 の後にまとめて確かめる（それまでは本番のコードが型に合わない）。
-   [ ] **ステップ 1-6**: `core/prepare.ts` の `buildArticleBody` が `input.transcript` を読むようにする。文書コメント（`:27-31`）のうち `:28-29` では、`The URL` ／ `and selection are copied verbatim;` と行をまたいで書かれている。この `selection` を `transcript` にし、`The URL and transcript are copied verbatim;` とする。
    -   **対象:** `extension/src/core/prepare.ts`。**完了:** 記したとおりの変更がある。型検査は、暫定の処理が入るステップ 1-7 の後にまとめて確かめる（それまでは本番のコードが型に合わない）。
-   [ ] **ステップ 1-7**: フェーズの完了条件のための変更。`launch.ts` の `PopupLaunchDeps.reader` の型を `TranscriptReader` にする。`chromeDeps.ts` に §1.3.5 の暫定の処理を置く。
    -   **対象:** `extension/src/launch.ts`・`extension/src/browser/chromeDeps.ts`。**完了:** `tsc --noEmit -p tsconfig.build.json`（`extension/` で実行。本番のコードだけを型検査する）が通る（テストの更新はステップ 1-8）。
-   [ ] **ステップ 1-8**: テストを書く・改める（§1.3.3 のフィクスチャを含む）。
    -   `collect.test.ts::parseTranscriptRead`: 受理する形（設計書 3.4 の表の各行）と、拒否する形（未知のメンバ、欠けたメンバ、`kind` の未知の値、`kind` ごとのメンバの過不足、`segment-list-blank` を含む未知の `reason`、型の違い、配列、`null`、選択範囲と `skipped` の対応の食い違い）。受理した値が補正されないこと。
    -   `collect.test.ts::collect`: 設計書 3.5 の表の各行。既存の空の選択範囲の行（`:105-116`・`:156-162`）は、パネルが `absent` のときの `empty-selection`（`detail` は `panel-absent`）として残す。全角空白・改行だけの選択範囲でパネルが `read` なら取得元が `panel` になること。`skipped` なら字幕が選択範囲の文字列になり、取得元が `selection` になること。`unknown-visibility` が `transcript-unavailable` になること。
    -   順序の行（AC-03・AC-05）: URL の不一致を、パネルの `read`・`absent`・`closed`・`unavailable`・空白だけの `read` のそれぞれと組み合わせ、すべて `collection-failed` になること。タイトルが空のときに、パネルの `absent`・`closed`・`unavailable`・空白だけの `read` がそれぞれの拒否（`empty-selection` または `transcript-unavailable`）になり、`empty-title` にならないこと。
    -   `summary.test.ts::truncateEnd`・`::summarize`: 上限ちょうど・1 つ超え・境界のサロゲートペア、`panel` の末尾のプレビューが字幕の末尾と一致すること、`selection` の要約のメンバが取得元を除いて今までと同じこと（メンバの完全な一致で確かめる）、拒否の `detail` の引き継ぎ。
    -   `messages.test.ts::rejectionMessage`: `:19-26` を、手順を持つ理由が `not-watch-page`・`empty-selection`・`transcript-unavailable` であることに書き直す。`transcript-unavailable` の手順に、パネルを開く・行を選択する・開き直す、の 3 つと、検索欄を空にする手順が含まれること（AC-12）。
    -   `sendMessages.test.ts`: 既存の網羅の検査（`:39-62`）は `rejectionReasons` から作るので、`transcript-unavailable` の委譲と文言の区別は自動で加わる。`body-too-large` の文言が 4 MiB を含むことは既存の `:64-68` が確かめる。
    -   `chromeDeps.test.ts::selection reader`: §1.3.5 の暫定の処理に合わせて、期待する値を `TranscriptRead` にする（空白だけの選択範囲は `absent`、それ以外は `skipped`）。この `describe` はステップ 2-3 で書き直す。
    -   `prepare.test.ts::buildArticleBody`・`::prepareSend`: 取得元が `panel` の入力から作る本文が `transcript` の値を持ち、メンバが選択範囲の場合と同じであること。`panel` の入力で 4,194,304 バイトを超える場合の `body-too-large`。
    -   **対象:** `extension/test/collect.test.ts`・`summary.test.ts`・`messages.test.ts`・`sendMessages.test.ts`・`chromeDeps.test.ts`・`prepare.test.ts` と、§1.3.3 のフィクスチャを使うテスト。**完了:** `make ext-check` が通る。
-   [ ] **ステップ 1-9**: 壊して失敗することを確かめ、壊し方ごとに失敗したテストの行をコミットメッセージに記録する。`parseTranscriptRead` の各検査を 1 つずつ外す（拒否する形の各行）。`documentUrl` の照合をパネルの判定の後に移す（URL の不一致とパネルの `absent` などを組み合わせた順序の行）。タイトルの検査をパネルの判定の前に移す（タイトルが空とパネルの拒否を組み合わせた順序の行）。`summarize` が `detail` を落とす、`truncateEnd` を先頭からの切り出しにする（`summary.test.ts`）。`buildArticleBody` に `origin` を加える（`prepare.test.ts`）。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確かめ、コミットメッセージに記録している（壊し方ごとに失敗したテストの行を含む）。

**完了:** `make ext-check` → `make test` → `make lint` が通る。

### PR-3 作成ポイント: core transcript read types and origin selection

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9

**推奨タイトル**: `feat(0029): validate the transcript read and select the transcript origin in core`

**レビュー観点**: `parseTranscriptRead` が設計書 3.4 の表の形だけを受理し、選択範囲と `skipped` の対応の食い違いを拒否し、受理した値を補正しないこと / `collect` の判定の順序が設計書 3.5 の表のとおりで、URL の不一致がパネルの結果によらず `collection-failed` になり、タイトルの検査がパネルの拒否より後にあること / §1.3.5 の暫定の処理が `whitespaceOnly` だけで `absent` と `skipped` を分け、`empty-selection` の文言を変えていないこと / §1.3.1 の名前の変更と、§1.3.3 のフィクスチャの更新が検査の内容を変えていないこと

**実装モデル要件**: frontier-required

**判定理由**: ステップ 1-2 の `parseTranscriptRead` は注入の結果という信頼できない入力の境界の検査（設計書 5.2、AC-22）で、`collect` の順序が URL の不一致をパネルの結果より先に拒否する（AC-03・AC-05）。どちらもセキュリティのゲートである。ステップ 1-7 は PR-4 で削除する暫定の処理を置く段階的な移行でもある（mkplan のパネル方式の条件「セキュリティのゲート」と「段階的な移行」）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: 注入と配線（設計書 8.2 の 2）

**対象ファイル:** `extension/src/browser/chromeDeps.ts`・`extension/src/popup.ts`・`extension/src/launch.ts`・`extension/src/core/messages.ts`、`extension/test/chromeDeps.test.ts`・`extension/test/launch.test.ts`・`extension/test/messages.test.ts`

-   [ ] **ステップ 2-1**: `chromeDeps.ts` に `readTranscript` と `createTranscriptReader` を設計書 3.1.3・3.3 のとおりに作る。注入は外の名前を参照せず、パネルの変種の定義を関数の中に持つ。`createTranscriptReader` は注入の引数を `[whitespaceOnly.source, whitespaceOnly.flags]` とし、結果を `parseTranscriptRead` に通し、合わなければ固定の文言の `Error` で拒否する。`readSelection`・`isPageSelection`・`createSelectionReader` と §1.3.5 の暫定の処理を削除する。選択範囲だけを読むことを前提にした `launch.ts` の文書コメント 2 つを、取得元によらない語に改める（`:75` の `Reads the video metadata of the tab the selection was collected from` の `the selection` を `the transcript` に、`:161` の `Collects the active tab's selection` を `Collects the active tab's transcript (the selection or the transcript panel)` にする）。
    -   **対象:** `extension/src/browser/chromeDeps.ts`・`extension/src/launch.ts`。**完了:** 記したとおりの変更がある。`popup.ts` が `createSelectionReader` を参照しているので、本番のコードの型検査は配線を改めるステップ 2-2 の後に確かめる（`chromeDeps.test.ts` の書き直しはステップ 2-3）。
-   [ ] **ステップ 2-2**: `popup.ts` で `createTranscriptReader(chrome.scripting)` を渡す。`core/messages.ts` の `empty-selection` の文言を改める。文を `"No text is selected."` から `"No text is selected, and no open transcript panel was found."` に、手順を `['Open the transcript panel with "Show transcript".', "Select the caption lines.", "Launch yt2column again."]` から設計書 3.7 の表の `empty-selection` の行の 3 つの手順にする。`messages.test.ts::rejectionMessage` に、`empty-selection` の手順に、パネルを開けば選択が要らないことと、手で選択する手順が含まれることを加える（AC-12）。
    -   **対象:** `extension/src/popup.ts`・`extension/src/core/messages.ts`・`extension/test/messages.test.ts`。**完了:** `tsc --noEmit -p tsconfig.build.json`（`extension/` で実行。本番のコードだけを型検査する）が通り、`messages.test.ts` が通る。
-   [ ] **ステップ 2-3**: `chromeDeps.test.ts` を書く（H-05）。
    -   `::transcript reader`（`:48-107` の `selection reader` を書き直す）: 注入の引数が `whitespaceOnly` の `source` と `flags` であること、`tabId`、受理する形に合わない結果と注入の例外で `read` が拒否されること。
    -   `::transcript injection`: 設計書 3.1.2 の形を最小限に抜き出した DOM をテストの中で組み立て、記録した注入を、§1.3.3 で広げた `run` に記録した引数を渡して実行する。セグメントの一覧の要素と、見出し・検索欄・同期のボタンの要素に、それぞれ別の印（テストで識別するための固有の文字列。以下、印）を返す `innerText` を定義する。一覧の印には前後の空白・改行・全角空白・サロゲートペアの文字を含める。設計書 7.1 の表の `chromeDeps.test.ts` の行に挙がる各分岐の結果を確かめる。動画 ID の判定より後の分岐を確かめるテストでは、URL の `v` と一致する `video-id` を持つ `ytd-watch-flexy` の中にパネルを置いた DOM を使う。スピナーだけの開いた状態は、§1.3.3 のとおりテストの中で組み立てる。
    -   空白の判定の規則（H-08）: 記録した引数で実行し、U+0085 だけの選択範囲ではパネルを読み、U+FEFF だけの選択範囲では `skipped` になること。
    -   選択範囲が空でない場合: 文脈に `document` を渡さずに実行でき、`panel` が `skipped` であること（`:89-106` の性質を保つ）。開いたパネルがある DOM でも同じであること。
    -   パネルの処理の例外: 6 つの段階のそれぞれで例外を起こし、`dom-error-<段階>` が返り、選択範囲と URL がそのまま返ること。
    -   ページを操作しないこと: jsdom の `Selection` の範囲の数と各範囲の境界、直列化した DOM が注入の前後で変わらないこと。§1.3.3 で共有する一覧に、選択範囲を変える呼び出し（`addRange`・`removeAllRanges`・`selectAllChildren` など）とスクロールの呼び出しを加え、注入のソースがそれらを含まないこと。
    -   `::injection count`: `extension/src/` の `executeScript(` の呼び出しが 2 つであること（§1.3.4）。
    -   **対象:** `extension/test/chromeDeps.test.ts`。**完了:** `make ext-typecheck` が通り、`chromeDeps.test.ts` が通る。
-   [ ] **ステップ 2-4**: `launch.test.ts` を改める。`:315-333` の拒否の一覧に `transcript-unavailable` の行を加える（動画メタデータを読まないこと）。新しい `describe`「transcript panel path」で、パネルから収集したときに送信の本文の `transcript` がパネルの字幕と一致すること、`transcript-unavailable` と `empty-selection` の拒否でログの `outcome` に `detail` が残り、パネルに入れた識別できる印がログに現れないことを確かめる。
    -   **対象:** `extension/test/launch.test.ts`。**完了:** `launch.test.ts` が通る。
-   [ ] **ステップ 2-5**: 壊して失敗することを確かめる。注入の各判定を 1 つずつ外す（開閉・動画 ID・一覧の数・描画の途中・検索欄）、選択範囲が空でないときもパネルを読む、パネルの処理の例外を捕らえずに注入全体を失敗させる、`innerText` の値を `trim` する、見出しを含む外側の要素を読む、注入が引数を使わず `/^\s*$/` で判定する、ログの `outcome` に字幕を入れる、`empty-selection` の手順からパネルの手順を消す。最後に、§1.3.1 の古い名前と、選択範囲だけを前提にした文書コメントが `extension/` に残っていないことを `rg -n "PageSelection|SelectionReader|createSelectionReader|readSelection|isPageSelection|selection reader" extension` で確かめ、出力（0 件）をコミットメッセージに記録する。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確かめ、コミットメッセージに記録している。最後の `rg` の出力が 0 件であることもコミットメッセージに記録している。

**完了:** `make ext-check` → `make test` → `make lint` が通る。

### PR-4 作成ポイント: transcript panel injection and popup wiring

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5

**推奨タイトル**: `feat(0029): read the transcript panel when the selection is blank`

**レビュー観点**: `readTranscript` が選択範囲が空白文字だけでないときにパネルの DOM に触れず、判定の流れ（設計書 3.3 の図）の各分岐で判定できない状態を `unavailable` に倒すこと / 動画 ID の照合（`ytd-watch-flexy` が 1 つ、`video-id` と URL の `v` の一致、パネルがその子孫）と、パネルの処理の例外を `dom-error-<段階>` に閉じ込めること / 注入がページを操作しないことの検査（`Selection` の範囲、直列化した DOM、ソースの走査）と、走査の一覧の `.value =` と検索欄の値の比較の扱い（§1.3.3） / 注入が外の名前を参照せず、`whitespaceOnly` の `source` と `flags` だけから正規表現を作ること

**実装モデル要件**: frontier-required

**判定理由**: ステップ 2-1 は、ページの DOM を字幕として使うかを決める fail closed の判定（設計書 3.3、AC-09・AC-10）で、既存の注入を削除して新しい注入に置き換える（mkplan のパネル方式の条件「セキュリティのゲート」と「振る舞いの追加と削除を同時に行う移行」）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 表示（設計書 8.2 の 3）

**対象ファイル:** `extension/src/ui/render.ts`・`extension/static/style.css`、`extension/test/render.test.ts`・`extension/test/launch.test.ts`

-   [ ] **ステップ 3-1**: `render.ts` に設計書 3.6 の表示を加える。欄の順序は Title・URL・Source・Characters・Lines・Preview・End。Source の値は `Record<TranscriptOrigin, string>`（`selection` は `Selected text`、`panel` は `Transcript panel`）で選ぶ。この `Record` と理由のコードのラベルは、ステップ 4-5 の文書の検査が import するので export する。End は取得元が `panel` のときだけ表示し、切り詰めたときは値の先頭に `…` を付け、クラス `summary-end-preview` と `dir="auto"` を持たせる。拒否の要約が `detail` を持つとき、メッセージの下に `Reason code: <値>` を `textContent` で表示する。
    -   **対象:** `extension/src/ui/render.ts`。**完了:** `tsc --noEmit -p tsconfig.build.json`（`extension/` で実行。本番のコードだけを型検査する）が通る。
-   [ ] **ステップ 3-2**: `style.css` の Preview の 2 つの規則（`unicode-bidi: plaintext` と `white-space: pre-wrap`）に `.summary-end-preview` を加え、`:41-42` のコメントの「the selection's」を取得元によらない語に改める。
    -   **対象:** `extension/static/style.css`。**完了:** Preview の 2 つの規則のセレクタに `.summary-end-preview` があり、`:41-42` のコメントに「the selection's」が残っていない。
-   [ ] **ステップ 3-3**: `render.test.ts::renderSummary` に、欄のラベルの並び（`panel` のときと `selection` のとき）、Source の値、`panel` のときだけ End があること、切り詰めたときの先頭の `…`、`<img src=x onerror=alert(1)>` を含む Preview と End で `img` が生まれず `textContent` に元の文字列が含まれること、End の `dir="auto"`、`detail` を持つ拒否で理由のコードが表示され、持たない拒否では表示されないことを加える。`launch.test.ts` の「transcript panel path」に、パネルから収集したときの要約に Source と End があり、`transcript-unavailable` の拒否で理由のコードが表示されることを加える。
    -   **対象:** `extension/test/render.test.ts`・`extension/test/launch.test.ts`。**完了:** `render.test.ts` と `launch.test.ts` が通る。
-   [ ] **ステップ 3-4**: 壊して失敗することを確かめる。End を `selection` のときも表示する、`…` を末尾に付ける、End を `textContent` 以外で設定する、End の `dir="auto"` を外す、Source の 2 つのラベルを入れ替える、End を Preview の前に置く、理由のコードを表示しない。
    -   **対象:** なし（確認のみ）。**完了:** 各対象を壊して失敗することを確かめ、コミットメッセージに記録している。

**完了:** `make ext-check` → `make test` → `make lint` が通る。

### PR-5 作成ポイント: summary display of origin, end preview, and reason code

**対象ステップ**: 3-1 / 3-2 / 3-3 / 3-4

**推奨タイトル**: `feat(0029): show the transcript origin, end preview, and reason code`

**レビュー観点**: 欄の順序（Title・URL・Source・Characters・Lines・Preview・End）と、End を取得元が `panel` のときだけ表示し、切り詰めたときに先頭に `…` を付けること / Preview・End・理由のコードを `textContent` で設定し、End が `dir="auto"` と `summary-end-preview` を持つこと（AC-16） / ステップ 4-5 が import する Source のラベルの `Record` と理由のコードのラベルの export

**実装モデル要件**: standard

**判定理由**: 設計書 3.6 で表示が決まっており、どの条件にも当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 文書（設計書 8.2 の 4）

**対象ファイル:** `README.md`・`docs/dev/security.md`・`docs/dev/project_overview.md`・`docs/translation_glossary.md`、`extension/test/metadataDocs.test.ts`・`extension/test/transcriptDocs.test.ts`（新規）・`extension/test/helpers/repoFile.ts`、`cmd/yt2column/docs_test.go`

-   [ ] **ステップ 4-1**: README の「Browser extension」節（`README.md:584-700`）に設計書 3.12 の内容を加える。改める段落は、冒頭の収集の説明（`:588-591`）、送る手順（`:643-647`）、取得できない値（`:659-664`。「since the selection was read」「The extension still sends the selection and the URL」を取得元によらない文にする）、送る文字列（`:682-693`）である。0026 PR-8 が先にマージされていれば、その後の節の構成に合わせる（§6.1）。
    -   **対象:** `README.md`。**完了:** 挙げた 4 つの段落が改まり、「since the selection was read」と「The extension still sends the selection and the URL」が README に残っていない（句の検査はステップ 4-5）。
-   [ ] **ステップ 4-2**: `docs/dev/security.md` §8（`:90`）と §8.1（`:102`・`:106`・`:113`・`:114`・`:116`）に要件書 F-007 の内容を加える。`:113` の「選択範囲はページの DOM の構造に依存せず」を、選択範囲が空のときだけパネルの DOM を読むことに合わせて改める。`:114` の「値を省略して選択範囲と URL を送る」を、取得元によらない語に改める。0026 PR-8 が先にマージされていれば、§8.1 の改める行を PR-8 の後の節で選び直す（§6.1）。
    -   **対象:** `docs/dev/security.md`。**完了:** 挙げた行が改まり、「選択範囲はページの DOM の構造に依存せず」と「値を省略して選択範囲と URL を送る」が残っていない（句の検査はステップ 4-5）。
-   [ ] **ステップ 4-3**: `docs/dev/project_overview.md` の「決定済みの方針」（`:7-17`）に、新しい項目を 1 つ加える。字幕の取得を利用者に任せる方針（#43。0007 の要件書 §1 で決めたもので、`project_overview.md` には現在書かれていない）を一部変えること、その理由（長い動画での選択の負担）、読めないときは手動の選択に戻るので DOM が変わっても機能を失わないことを書く。拡張がパネルから読んだ字幕も送るので、`:10` の「文字起こしパネルからコピーした字幕を受け取り」と `:26` の「コピーした字幕の文字列から作る `CopiedSource`」に、パネルから読んだ字幕も同じ書式で受け取ることを加える。`:101` の `extension/` の説明に、パネルからの読み取りを加える。
    -   **対象:** `docs/dev/project_overview.md`。**完了:** 「決定済みの方針」に新しい項目があり、`:10`・`:26`・`:101` の 3 箇所にパネルから読んだ字幕のことが加わっている（句の検査はステップ 4-5）。
-   [ ] **ステップ 4-4**: `docs/translation_glossary.md` に、取得元・末尾のプレビュー・拒否の詳細・理由のコードの訳語を加える。英語の訳語は、ソースの識別子と表示のラベル（`origin`・`End`・`detail`・`Reason code`）と矛盾しないものにする。
    -   **対象:** `docs/translation_glossary.md`。**完了:** 取得元・末尾のプレビュー・拒否の詳細・理由のコードの 4 つの訳語が用語集にある。
-   [ ] **ステップ 4-5**: 文書の検査を書く。
    -   `metadataDocs.test.ts:8-22` の `flatSection`・`expectPhrases` を `test/helpers/repoFile.ts` へ移す（§1.3.3）。`metadataDocs.test.ts:34` の句を、ステップ 4-1 で改めた文に合わせる。
    -   `transcriptDocs.test.ts`（新規）: README の「Browser extension」節と `security.md` の §8・§8.1 に要件書 F-007 の各項目の句があること。README が引用する表示の文字列（Source の値、理由のコードのラベル）は、`metadataDocs.test.ts` が `expandPromptMessage` を import するのと同じく、ソースから import した値と照合する。そのため、`render.ts` の Source のラベルの `Record` と理由のコードのラベルを export する。
    -   `cmd/yt2column/docs_test.go` に `TestTranscriptPanelReadDocs` を加え、`project_overview.md` に F-007 の 3 つの項目の句があることを確かめる。照合する句は日本語の文書の句なので、Go の文字列リテラルに日本語を含める。これは、日本語の文書の見出しや句と一致させる検査の文字列リテラルに限った例外で、既存の検査（`cmd/yt2column/docs_test.go:139-152` の `TestReviewEvaluationRecords` など）と同じ扱いである。
    -   **対象:** `extension/test/helpers/repoFile.ts`・`extension/test/metadataDocs.test.ts`・`extension/test/transcriptDocs.test.ts`（新規）・`extension/src/ui/render.ts`（export のみ）・`cmd/yt2column/docs_test.go`。**完了:** `make ext-test` と `make test` が通る。
-   [ ] **ステップ 4-6**: 書いた文が実装と一致することを確かめる。README と `security.md` の各文について、根拠になる実装の箇所（`readTranscript` の判定、`collect` の表、`render.ts` のラベル、`rejectionMessage` の文言）を突き合わせ、PR の説明に対応を書く。各検査から句を 1 つずつ消して失敗することを確かめ、コミットメッセージに記録する。
    -   **対象:** なし（確認のみ。対応は PR の説明に書く）。**完了:** 各文と実装の箇所の対応が PR の説明にあり、各検査から句を 1 つずつ消して失敗することを確かめ、コミットメッセージに記録している。

**完了:** `make ext-check`（`make ext-test` を含む）→ `make test` → `make lint` が通る。

### PR-6 作成ポイント: documentation of the transcript panel read

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6

**推奨タイトル**: `docs(0029): document reading the transcript panel`

**レビュー観点**: README と `security.md` §8・§8.1 の各文が、`readTranscript` の判定・`collect` の表・`render.ts` のラベル・`rejectionMessage` の文言と一致すること（ステップ 4-6 の対応を PR の説明に書く） / `transcriptDocs.test.ts` が表示の文字列をソースから import した値と照合し、`flatSection`・`expectPhrases` の移動で `metadataDocs.test.ts` の検査が弱まっていないこと / `project_overview.md` の「決定済みの方針」の新しい項目が、字幕の取得を利用者に任せる方針（#43）を一部変えることと、その理由を書いていること

**実装モデル要件**: standard

**判定理由**: 書く内容は要件書 F-007 と設計書 3.12 で決まっており、どの条件にも当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 手動の確認（設計書 8.2 の 5）

**対象ファイル:** 本計画（§5.1）、`cmd/yt2column/extension_docs_test.go`

-   [ ] **ステップ 5-1**: `make ext-build` で作った拡張を Chrome と Brave の安定版に読み込み、§5.1 の各行を設計書 7.2 のとおりに確かめ、記録する。
    -   **対象:** 本計画 §5.1。**完了:** §5.1 のすべての行が §5.1 の完了の規則を満たす。
-   [ ] **ステップ 5-2**: `cmd/yt2column/extension_docs_test.go` に `TestTranscriptPanelReadRecords` を加え、`checkManualCheckTable` と `acLabel` で §5.1 の表の行のラベルと、すべての欄が埋まっていることを確かめる。行のラベルは `acLabel(19) + "（形 A）"` のように組み立てる（日本語の部分は、ステップ 4-5 と同じく文書と一致させる検査の文字列リテラルである）。§5.1 の欄を 1 つ空にして失敗することを確かめ、コミットメッセージに記録する。
    -   **対象:** `cmd/yt2column/extension_docs_test.go`。**完了:** `make test` が通り、§5.1 の欄を 1 つ空にして失敗することを確かめ、コミットメッセージに記録している。

**完了:** §5.1 のすべての行が §5.1 の完了の規則を満たし、`make test` → `make lint` が通る。

### PR-7 作成ポイント: manual browser verification record

**対象ステップ**: 5-1 / 5-2

**推奨タイトル**: `test(0029): record the manual Chrome and Brave verification`

**レビュー観点**: §5.1 の各行が設計書 7.2 の確認の内容に対応し、Chrome と Brave の両方の結果が「Chrome: …。Brave: …」の形で埋まっていること / AC-19 の差の記録と、形 B の古い版のパネルを拒否して手で選択して送れたことの記録が §5.1 の完了の規則に合うこと / `TestTranscriptPanelReadRecords` が行のラベルと空の欄を確かめていること

**実装モデル要件**: standard

**判定理由**: 確認の手順は設計書 7.2・8.1 で、記録の検査は既存の `checkManualCheckTable` で決まっており、どの条件にも当たらない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン (Implementation Order and Milestones)

### 3.1. マイルストーン

| マイルストーン | 成果物 | 条件 |
|---|---|---|
| M0: 観測のゲート | 設計書 3.1 の観測の記録。改めて承認された設計書と本計画 | フェーズ 0 |
| M1: `core/` の判定 | 取得元の選択・受理する形の検査・メッセージ・要約・本文の読み替え | フェーズ 1 の完了条件 |
| M2: パネルの読み取り | `readTranscript` と配線。ポップアップがパネルから収集できる | フェーズ 2 の完了条件 |
| M3: 表示 | Source・End・理由のコード | フェーズ 3 の完了条件 |
| M4: 文書 | README・`security.md`・`project_overview.md`・用語集と、その検査 | フェーズ 4 の完了条件 |
| M5: 手動の確認 | §5.1 の記録と、その検査 | フェーズ 5 の完了条件 |

### 3.2. PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 0-1 / 0-2 / 0-3 / 0-4 / 0-5 / 0-6 | 観測のゲートの記録を設計書 3.1 に加え、設計書を `draft` に戻す | frontier-required |
| PR-2 | 0-7 | 改めて承認された設計書に、本計画のフェーズ 1〜5 を合わせる | frontier-recommended |
| PR-3 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9 | `core/` の字幕の読み取りの型、`parseTranscriptRead`、取得元の選択と拒否の詳細、文言・要約・本文の読み替え、§1.3.5 の暫定の処理、そのテスト | frontier-required |
| PR-4 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 | 注入 `readTranscript` と `createTranscriptReader`、`popup.ts` の配線、`empty-selection` の文言、`chromeDeps.test.ts`・`launch.test.ts` | frontier-required |
| PR-5 | 3-1 / 3-2 / 3-3 / 3-4 | 要約の Source・End の欄と理由のコード、`style.css`、そのテスト | standard |
| PR-6 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 | README・`security.md`・`project_overview.md`・用語集と、`transcriptDocs.test.ts`・`TestTranscriptPanelReadDocs` | standard |
| PR-7 | 5-1 / 5-2 | Chrome と Brave の手動の確認の記録（§5.1）と `TestTranscriptPanelReadRecords` | standard |

-   PR-1 と PR-2 を分けるのは、その間に設計書の改訂と再承認（要件のレビューを含みうる）が入るためである。PR-2 は、設計書が改めて承認された後に作る。PR-3 は、PR-2 がマージされ、本計画が `approved` である状態で始める（§1.2）。
-   フェーズ 1 は 1 つの PR にする。ステップ 1-1〜1-6 は、ステップ 1-7 の暫定の処理が入るまで本番のコードが型に合わず、ステップ 1-8 のテストの更新がそろうまで `make ext-check` が通らないためである。名前の変更や文言だけの変更を別の PR に分けるには、ステップ 1-3・1-4・1-8 を分割しなければならないので、分けない。危険の大きいステップ 1-2 を最後に置くこともできない（ステップ 1-5〜1-8 がその型に依存する）。そのため、PR-3 はステップごとにコミットし、ステップ 1-2 の `parseTranscriptRead` と `collect` を個別に読めるようにする。
-   フェーズ 2 も 1 つの PR にする。ステップ 2-1 が `createSelectionReader` を削除すると、`popup.ts` の配線（ステップ 2-2）と `chromeDeps.test.ts` の書き直し（ステップ 2-3）がなければ型の検査とテストが通らないためである。PR-3 から PR-4 までの間の `main` の振る舞いは §1.3.5 のとおりである。
-   表示（PR-5）は注入から分ける。PR-5 は PR-4 の後に作る。ステップ 3-3 が、ステップ 2-4 で作る `launch.test.ts` の「transcript panel path」に検査を加えるためである。
-   文書（PR-6）は PR-5 の後に作る。ステップ 4-5 の `transcriptDocs.test.ts` が、ステップ 3-1 で export する `render.ts` のラベルを import するためである。
-   PR-4 のマージから PR-6 のマージまでの間の `main` では、拡張がパネルを読むが、PR-5 までは要約に Source と End がない（設計書 5.4 の残るリスクの緩和がない）。PR-6 までは `docs/dev/security.md` §8.1 が「選択範囲はページの DOM の構造に依存せず」と記したままで、実装と食い違う。この間の `main` から作った拡張は利用に使わず、PR-5・PR-6 を PR-4 の後に続けてマージする。
-   手動の確認（PR-7）は、PR-6 までがマージされた `main` から `make ext-build` で作った拡張で行う。手動の確認で見つかった不具合は、PR-7 とは別の `fix(0029)` の PR で直し、その後に該当する §5.1 の行を確かめ直す。
-   コードやテストを変える PR-3〜PR-7 は、その PR のテストを壊して確かめるステップ（1-9・2-5・3-4・4-6、PR-7 はステップ 5-2 の中）を含む。
-   グリーンゲートは `make test && make lint` だが、拡張を変える PR-3〜PR-5 は `make ext-check` も、README と `security.md` を変える PR-6 は `make ext-check`（`make ext-test` を含む）も通す（各フェーズの完了条件）。PR-1・PR-2・PR-7 は `make test` を通す。
-   0026 PR-8 のマージの有無は、PR-3〜PR-6 のそれぞれを作る前に確かめる（§6.1）。

### 3.3. 実装順序の根拠

-   フェーズ 0 を最初に置くのは、設計書 3.1.3・8.1 が観測の前に実装に進まないと定めているためである。
-   フェーズ 1〜5 の順序は設計書 8.2 のとおりである。フェーズ 1 が型を変えると注入と配線が型に合わなくなるので、フェーズ 1 の中で暫定の処理（§1.3.5）を置き、各フェーズの完了条件を保つ。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

-   拡張のテストは `node --test` で実行し、実際のブラウザも実際のサーバも起動しない（要件書 5.）。注入は、記録した関数のソースを jsdom の文書で実行する（設計書 7.1）。jsdom は `innerText` を実装しないので、テストで要素に定義する（H-05）。`innerText` と手でコピーした文字列の一致は、手動の確認（AC-19）で確かめる。
-   網羅の目標: 設計書 3.3 の判定の図の各分岐、設計書 3.4 の受理する形の各行、設計書 3.5 の表の各行に、少なくとも 1 つのテストがある。
-   新しいテストのファイルは `extension/test/transcriptDocs.test.ts` だけである。新しいヘルパーのファイルは加えない（`flatSection`・`expectPhrases` を共有する場合は既存の `test/helpers/` のファイルに置く）。Go のテストのヘルパーは既存のものを使い、`testutil/` と `test_helpers.go` を加えない。

### 4.2. 統合テスト（手動）

設計書 7.2 の AC-16（手動の部分）・AC-19・AC-20 を、フェーズ 5 で Chrome と Brave の安定版で確かめ、§5.1 に記録する。

### 4.3. 後方互換性

-   選択範囲から送る場合の振る舞いは、取得元の表示と、選択を前提にした文言の変更（ステップ 1-3・1-4・2-2）を除いて変えない。`collect.test.ts` の既存の行（選択範囲の空白の判定、URL の照合、タイトルの検査、値を補正しないこと）を残し、`summary.test.ts` で `selection` の要約のメンバが取得元を除いて今までと同じことを確かめる。
-   本文の形とサーバの API は変えない（`prepare.test.ts`）。manifest は変えない（`manifest.test.ts` を変更しない）。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

ラベルの意味は次のとおりである。`test` は、実際に実行され、失敗しうるテスト。`static` は、文書やソースを読んで確かめる、実行される検査。`manual` は、手動の確認。拡張のテストは `extension/test/<file>::<describe の名前>`、Go のテストは `<path>::<関数名>` で示す。`manual` は `test`・`static` を置き換えず、補う。

| AC | ラベル | 検証 | 実装 |
|---|---|---|---|
| AC-01 | test | `collect.test.ts::collect`（`skipped` なら取得元は `selection`）、`chromeDeps.test.ts::transcript injection`（選択範囲が空でなければ、開いたパネルがあっても `skipped`） | 1-2・1-8・2-1・2-3 |
| AC-02 | test | `collect.test.ts::collect`（空・全角空白・改行だけの選択範囲で `read` なら取得元は `panel`）、`chromeDeps.test.ts::transcript injection`（空白だけの選択範囲でパネルを読む） | 1-2・1-8・2-1・2-3 |
| AC-03 | test | `collect.test.ts::collect`（設計書 3.5 の表の順序、選択範囲がある場合の既存の順序） | 1-2・1-8 |
| AC-04 | test | `chromeDeps.test.ts::transcript injection`（一覧の `innerText` の印が補正されずに返り、ほかの要素の印が含まれない）、`launch.test.ts::transcript panel path`（本文の `transcript` と一致） | 2-1・2-3・2-4 |
| AC-05 | test・static | `collect.test.ts::collect`（パネルが `read` でも URL の不一致は `collection-failed`）、`chromeDeps.test.ts::injection count`（`executeScript(` が 2 つ） | 1-2・1-8・2-3 |
| AC-06 | test | `chromeDeps.test.ts::transcript injection`（パネルなし、閉じている、閉じた状態でセグメントを持つ）、`collect.test.ts::collect`（`absent`・`closed` は `empty-selection` と詳細） | 1-2・1-8・2-1・2-3 |
| AC-07 | test | `chromeDeps.test.ts::transcript injection`（一覧なし、スピナーだけ（`testdata` の形））、`collect.test.ts::collect`（空白だけの `read` は `segment-list-blank`） | 1-2・1-8・2-1・2-3 |
| AC-08 | test | `chromeDeps.test.ts::transcript injection`（`filtered`・`search-state-unknown`・`rendering-incomplete`）、`collect.test.ts::collect`（`unavailable` は `transcript-unavailable`） | 1-2・1-8・2-1・2-3 |
| AC-09 | test・manual | `chromeDeps.test.ts::transcript injection`（`video-mismatch`・`video-unverified` の各場合）、`collect.test.ts::collect`、§5.1 の AC-20（SPA の画面遷移） | 1-2・1-8・2-1・2-3・5-1 |
| AC-10 | test・static | `chromeDeps.test.ts::transcript injection`（選択範囲の範囲と DOM が変わらないこと、注入のソースがページを操作する呼び出しを含まないこと） | 2-1・2-3 |
| AC-11 | test・static | `messages.test.ts::rejectionMessage`・`sendMessages.test.ts::blockMessage`（異なる文言、委譲）、`make ext-check` の型検査（`rejectionMessage`・`blockMessage` の `never`） | 1-1・1-3・1-4・1-8 |
| AC-12 | test | `messages.test.ts::rejectionMessage`（`empty-selection` と `transcript-unavailable` の手順） | 1-3・1-8・2-2 |
| AC-13 | test | `launch.test.ts::transcript panel path`（ログの `detail`、印がログに現れない）、`collect.test.ts::collect`（詳細の値、`unknown-visibility` は `transcript-unavailable`）、`chromeDeps.test.ts::transcript injection`（未知の開閉の値、閉じたパネルとの混在） | 1-2・1-8・2-3・2-4 |
| AC-14 | test | `collect.test.ts::collect`（`origin`）、`summary.test.ts::summarize`、`render.test.ts::renderSummary`（Source）、`launch.test.ts::transcript panel path` | 1-2・1-5・1-8・3-1・3-3 |
| AC-15 | test | `summary.test.ts::truncateEnd`・`::summarize`、`render.test.ts::renderSummary`（End は `panel` だけ） | 1-5・1-8・3-1・3-3 |
| AC-16 | test・manual | `render.test.ts::renderSummary`（Preview と End で要素にならない）、§5.1（双方向の制御文字） | 3-1・3-2・3-3・5-1 |
| AC-17 | test | `prepare.test.ts::buildArticleBody`（`panel` の入力の本文）、`launch.test.ts::transcript panel path` | 1-6・1-8・2-4 |
| AC-18 | test | `prepare.test.ts::prepareSend`（`panel` の入力での `body-too-large`）、`sendMessages.test.ts::blockMessage`（既存の `:64-68`。`body-too-large` の文言が 4 MiB を含む） | 1-4・1-6・1-8 |
| AC-19 | static・manual | `cmd/yt2column/extension_docs_test.go::TestTranscriptPanelReadRecords`（§5.1 の記録）、§5.1 | 5-1・5-2 |
| AC-20 | static・manual | `cmd/yt2column/extension_docs_test.go::TestTranscriptPanelReadRecords`、§5.1。判定そのものは AC-08・AC-09 の `test` | 5-1・5-2 |
| AC-21 | static | `extension/test/transcriptDocs.test.ts`（README・`security.md` §8.1）、`cmd/yt2column/docs_test.go::TestTranscriptPanelReadDocs`（`project_overview.md`） | 4-1〜4-5 |
| AC-22 | test | `collect.test.ts::parseTranscriptRead`、`chromeDeps.test.ts::transcript reader`（形の合わない結果で `read` が拒否する） | 1-2・1-8・2-1・2-3 |
| AC-23 | static | `extension/test/manifest.test.ts`（本タスクは manifest と `manifest.test.ts` を変えない。0026 PR-8 が先にマージされた場合は、その後の `describe` の名前を着手のときに確かめて本行を改める） | なし |

### 5.1. 手動の確認と記録

フェーズ 5 で埋める。確認の内容は設計書 7.2 のとおりである。「ブラウザ・環境」の欄には、動画 ID、ブラウザとその版、ログインの状態を書く。

**完了の規則:** すべての行の欄が埋まり、各行の結果に Chrome と Brave の両方の結果（「Chrome: …。Brave: …」の形）があり、どちらも成功であるときだけ、§5.1 を完了とする。AC-19 の行は、一致しない場合でも、その差と扱いを記録していれば成功とする（要件書 AC-19）。形 B のページのパネルが古い版（設計書 3.1.3）で、拡張がそれを読まない場合は、拒否されて手で選択して送れることを記録すれば成功とする（設計書 7.2）。

| AC | 確認の内容（設計書 7.2） | ブラウザ・環境 | 日付 | 結果 |
|---|---|---|---|---|
| AC-16 | 字幕の先頭と末尾に双方向の制御文字（U+202E など）を含む動画で、要約のラベルとほかの値の表示順が変わらない |  |  |  |
| AC-19（形 A） | パネルから読んだ文字列と、パネルの全行を手で選択してコピーした文字列が一致する |  |  |  |
| AC-19（形 B） | 同上。古い版のパネルで読まない場合は、拒否されて手で選択して送れる |  |  |  |
| AC-20（長い動画） | 2 時間を超える動画で、一部だけの字幕が送られない（拒否されるか、全行が描画される） |  |  |  |
| AC-20（検索） | パネル内の検索で絞り込んだ状態では送られない |  |  |  |
| AC-20（SPA の画面遷移） | 設計書 8.1 の手順の遷移の後、前の動画の字幕が送られない |  |  |  |

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 対策 |
|---|---|
| 観測のゲートで、結びつける信号が見つからない、または前の動画の字幕を収集する（設計書 3.1.3） | ステップ 0-6 で要件のレビューに諮る。要件のレビューが判断を示すまでフェーズ 1 に進まない |
| 観測で、読み込み中を示す要素なしの遅延描画が見つかる | 設計書を `draft` に戻す（設計書 8.1）。本計画をステップ 0-7 で合わせる |
| `innerText` と手でコピーした文字列が異なる（設計書 5.4） | §5.1 の AC-19 に差と扱いを記録する。書式が崩れればサーバが `invalid_transcript` で拒否する |
| 0026 PR-8 と同じファイルを変える（§1.3.6） | 着手のときと PR-3〜PR-6 のそれぞれを作る前に、0026 PR-8 のマージの有無を確かめる。後からマージする側が、本計画の変更（§1.3.1 の台帳、ステップ 1-3・1-4・2-2 の文言、ステップ 1-8 の `prepare.test.ts`・`sendMessages.test.ts` の行、ステップ 4-1・4-2 の文書）を相手の変更の上に載せ直し、`make ext-check` を通す。README と `security.md` §8.1 を PR-8 が書き換えた場合は、ステップ 4-1・4-2 の改める箇所を PR-8 の後の節で選び直す。AC-23 の行も確かめ直す |
| ページを操作する呼び出しのソースの走査が、検索欄の値の比較と一致する（§1.3.3） | ステップ 2-3 で走査の規則か比較の書き方を決め、代入を入れたときに走査が失敗することを確かめる |

### 6.2. スケジュールのリスク

-   フェーズ 0 は実際のブラウザでの試行（各ブラウザ 30 回の遷移を含む）と、設計書の再承認を要する。フェーズ 1 以降の着手はその後になる。
-   0026 PR-8 との衝突の解消に、1 フェーズ分の作業が加わりうる。

## 7. 実装チェックリスト (Implementation Checklist)

-   [ ] PR-1 マージ済み（対象ステップ: 0-1 / 0-2 / 0-3 / 0-4 / 0-5 / 0-6）
-   [ ] 設計書が改めて承認されたことを確かめた（PR-2 の前提）
-   [ ] PR-2 マージ済み（対象ステップ: 0-7）
-   [ ] 本計画が `approved` であることを確かめた（PR-3 の前提）
-   [ ] PR-3 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 / 1-9）
-   [ ] PR-4 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5）
-   [ ] PR-5 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3 / 3-4）
-   [ ] PR-6 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6）
-   [ ] 手動の確認で見つかった不具合の `fix(0029)` の PR がマージ済み（PR-7 の前提。不具合がなければ不要）
-   [ ] PR-7 マージ済み（対象ステップ: 5-1 / 5-2）
-   [ ] §5 のすべての AC の検証が通る
-   [ ] §1.3.1 の古い名前が `extension/` に残っていない（`make ext-check` の型検査と、ステップ 2-5 の検索）

## 8. 成功基準 (Success Criteria)

-   **機能:** 要件書の AC-01〜AC-23 が §5 の検証で満たされる。
-   **品質:** `make ext-check`・`make test`・`make lint` が通る。§5 に挙げた各テストを、対象を壊して失敗することを確かめ、コミットメッセージに記録している。
-   **セキュリティ:** 注入がページを操作しないこと（AC-10）、字幕をテキストとして表示すること（AC-16）、拒否のログに字幕が含まれないこと（AC-13）、注入の結果の形を検査すること（AC-22）、権限が変わらないこと（AC-23）を確かめている。
-   **文書:** README・`security.md`・`project_overview.md`・用語集が改められ、AC-21 の検査が通る。§5.1 の記録が埋まり、`TestTranscriptPanelReadRecords` が通る。

## 9. 次のステップ (Next Steps)

-   本計画のレビューと承認。
-   承認の後、フェーズ 0（観測のゲート）を行う。フェーズ 1 以降は、ステップ 0-7 の後に着手する。PR の境界は §3.2 のとおりである。
-   実装の後、構造化したセグメントの送信（#331）の検討に、本タスクの観測の記録（設計書 3.1）と §5.1 を引き継ぐ。
