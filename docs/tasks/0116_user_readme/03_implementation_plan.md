# 実装計画書：利用者向けの README と INSTALL 文書

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-20 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。設計への申し送り [design_handoff.md](design_handoff.md) の H-01〜H-07 は、設計書 10 章が対応を記録している。本タスクに `implementation_handoff.md` はない。

本計画で「移すブロック」とは、設計書 2.3 節の表で扱いが「移す」の行の内容を指す。「今の README」は設計書と同じく、コミット `417d2ddf` の `README.md` を指す（`README.md` は本計画を書いた `bec22f3d` まで変わっていない）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

README を利用者向けの概要と案内に作り直し、利用者の手順を新しい `INSTALL.md` に、開発者向けの内容を `docs/dev/guides/` の手引き（新しい `generation.md`、既存の `cli.md`・`development.md`）に移す。README の値を確かめていたテストの読む先を、値が移った先の文書に変える（設計書 1 章〜3 章）。

### 1.2. 実装の原則

-   **どのコミットでもテストが通る。** 各コミットで `make test`・`make lint`・`make ext-check`・`make deadcode` が通る（設計書 8 章）。README から値を除く変更と、その値を読む検査の読む先を変える変更は、同じコミットに入れる。
-   **`README.md` を変えるコミットは `/mktrans README.md` の同期を含む**（設計書 6.2 節）。`INSTALL.md` と手引きは翻訳の対ではない。
-   **移した内容への参照は、移したコミットで付け替える。** 付け替える箇所は設計書 4.2 節の表を元にし、各フェーズの初めに `git grep -n 'README' -- ':!docs/tasks'` で探し直す。
-   **検査を外すコミットは単独にする。** 外した後に `make coverage-compare` か `make ext-coverage-compare` をそのコミットの親と比べ、結果の最後の行までと、外した値をまだ確かめている検査の名前と壊し方を、コミットのメッセージと PR の説明に書く（設計書 3.3 節「外す検査」、CLAUDE.md「Deleting a test is a claim that must be checked」）。
-   **読む先を変えた検査と新しい検査は、壊して失敗することを確かめてからコミットし、確かめたことをコミットのメッセージに書く**（設計書 3.3 節「読む先を変える検査の確かめ方」、7.1 節）。
-   **値と構造はテストで、文の意味は PR のレビューで確かめる**（設計書 1.1 節）。
-   Go と TypeScript のコメント・識別子・文字列は英語で書く。例外は、ガードテストのロケータが日本語の文書の見出しと列名に一致させる文字列（CLAUDE.md「Go Idioms」の guard-test literal exception）である。

### 1.3. 計画の際に決めたこと（2026-11-20）

1. **移すブロックは、フェーズ 1 で README からすべて除く。** 冒頭の説明のうち CLI と構成の部分（今の README `:7-9`・`:39-45` のうち `generation.md` に移す文）と構成要素の図（`:11-37`）も含む。README に残る利用者向けの部分は、フェーズ 3 で書き直す。
   - **理由:** 移す作業がフェーズ 1 の PR で終わり、設計書 7.3 節の AC-04 のスクリプトの「移す作業の最後の PR」での確かめ（どのブロックも README になく、移った先にあること）を PR-1 で済ませられる。冒頭の `[推奨構成](#推奨構成)`（`README.md:43`）は、推奨構成を移すフェーズ 1 でどのみち直す必要がある。
   - **採らなかった案:** 冒頭をフェーズ 3 の作り直しまで残す案。AC-04 の確かめが PR-1 と PR-3 にまたがり、両方の PR がスクリプトを使う。
   - **ADR:** なし。タスクの中のフェーズの分け方で、このタスクの中で閉じる。
2. **PR は 4 本にする。** フェーズ 1、2、3 をそれぞれ 1 本、フェーズ 4 と 5 を 1 本にする（3.2 節）。
   - **理由:** フェーズ 5 は記録がそろっていることの確かめだけで、単独の PR にするほどの変更がない。README の作り直し（フェーズ 3）と、CLAUDE.md・規約・コマンドの変更（フェーズ 4）は、読む観点が違うので分ける。
   - **採らなかった案:** フェーズ 3〜5 を 1 本にする 3 本の案。README の作り直しと、CLAUDE.md・コマンドの変更が 1 つのレビューに混ざる。
   - **ADR:** なし。
3. **AC-24 の対応表は、本計画の 7 章の AC-24 の行の下に置く。** 今の README `:182-264` の文ごとに、`INSTALL.md` のどの節で述べたかを書く（設計書 7.3 節が置き場所を実装計画に委ねた）。
   - **理由:** ほかの `diff` 検証の記録と同じ場所にあり、レビュアーが 1 か所で読める。
   - **採らなかった案:** タスクのディレクトリに別のファイルを置く案、PR の説明にだけ書く案（リポジトリに残らない）。
   - **ADR:** なし。
4. **ブラウザの警告の確かめ（AC-18、AC-34 のブラウザの文言）は、PR-2 の `/runplan` が実行の途中で人に頼む。PR-2 の自動送信は `hold` にする。**
   - **理由:** 拡張をパッケージ化せずに Chrome と Brave で読み込み、表示を見る作業は、エージェントにはできない。人がブラウザの前にいるときに `/runplan` を始められるよう、`chief` に自動で送らせない。
   - **採らなかった案:** PR-2 の前に人が確かめて issue に書く案（`/runplan` の外の作業が増える）。自動送信のまま `/runplan` が途中で頼む案（人がいないときに始まりうる）。
   - **ADR:** なし。
5. **AC-05 の確かめはフェーズ 3（PR-3）で走らせて記録し、用語集の移行の手順の行（「ホスト」と「設定ファイル」の備考、「以前の版からの移行」の項目）もフェーズ 3 で直す。** 設計書 8 章の表は、用語集の「残り」をフェーズ 4 に置き、AC-05 をフェーズ 3 の AC に挙げる。本計画は、移行の節を消すフェーズで、それを述べる用語集の行も直す。
   - **理由:** 設計書 7.3 節の AC-05 の (b) は元の README の移行の節の行を、(c) はマージベースとの組の比較を使う。どちらも、PR のマージベースに移行の節がまだある PR-3 で走らせる必要がある。(c) は用語集の組が 4.2 節の変更の後の組と一致することを求めるので、用語集の移行の行もその PR で直す。決定 2 の PR の分け方は変わらない。
   - **ADR:** なし。
6. **INSTALL.md と手引きを足すことに伴い README から手順を除くフェーズ 2 では、除いた箇所に `INSTALL.md` への一時的なリンクを置く。** フェーズ 3 の作り直しで、設計書 2.4 節の「インストールと使い方」の節に置き換える。
   - **理由:** PR-2 と PR-3 の間の `main` で、README から手順への道が切れないようにするため。
   - **ADR:** なし。

### 1.4. 既存コード調査結果

コミット `bec22f3d`（`origin/main`）で確かめた。設計書の行番号は `417d2ddf` のもので、次の差を除き、本計画が触れるファイルは同じである（`git diff --stat 417d2ddf bec22f3d` で確かめた）。

-   **行番号のずれ:** `docs/translation_glossary.md` は 3 行増え、設計書 4.2 節の `:61`・`:168`・`:175-181`・`:179` は、今は `:61`・`:171`・`:178-184`・`:182` である。CLAUDE.md の行も変わったので、CLAUDE.md の箇所は節の名前で探す。ADR-0004・ADR-0025・ADR-0030 のずれを示す一文は、設計書の PR（#1050）で置いたものが今ある（ADR-0004 `:22`、ADR-0025 `:20`、ADR-0029 `:24`、ADR-0030 `:23`、ADR-0035 `:24`）。
-   **設計書 4.2 節にない箇所:** `docs/translation_glossary.md:58` の備考は、一般の語の例として「`README.md` の「構成要素の組み立て」」を挙げるが、今の README にその文字列はない（`grep -n 構成要素の組み立て README.md` は何も出さない）。フェーズ 4 で、この例を今ある箇所に直すか除く。
-   **Go のガードテスト:**
    - `cmd/yt2column/docs_test.go` の `readmeVersionsUnder`（`:50`）を使うのは、`TestOpenAIDocsContract`（`:208`）、`TestProductionProfileDocsContract`（`:710`）、`extension_docs_test.go` の拡張の ID の部分テスト（`:63`）である。`TestLLMDefaultsDocsContract`（`:288`）と `TestEffortValuesDocsContract`（`:337`）は `doctestutil.ReadmeVersions()` を直接回す。`TestReadmeLinks`（`:729`）は `cli.md` と `development.md` をハードコードし（`:739-743`）、README の 2 つの版を足す。
    - `internal/doctestutil/readme.go` の `Locator`（`:46-76`）と README のロケータ（`:78-92`）を使うのは、`docs_test.go` の上の 3 つのテストだけで、`Locator` の単体テストは `internal/doctestutil/readme_test.go::TestLocatorIn` だけである。`ReadmeVersion` と `Lang` は、`readme_structure_test.go:37-38` と `TestReadmeLinks` が使い続ける（`grep -rn 'Locator\|LangJA\|ReadmeVersions()' cmd internal`）。
    - `make deadcode` は使われない関数を報告するので、`Locator.In` と `readmeVersionsUnder` は、最後の呼び出し元を除くコミットで除く。
    - `cmd/yt2column/docpaths_test.go` の `readmeGuides`（`:94`）は、`TestInstructionDocPaths` が見ない手引きの一覧である（`:121`）。
    - `internal/doctestutil/guides.go` は 2 つの手引きのロケータを日本語の 1 つの文字列の定数で持つ。`generation.md` のロケータもここに足す。
-   **拡張のテスト:**
    - `extension/test/helpers/readmeVersions.ts` は `readmeVersions`・`browserExtensionHeading`・`minimumChromePhrase` を持つ。使うのは `metadataDocs.test.ts`（前の 2 つ）と `manifest.test.ts`（1 つ目と 3 つ目）だけである。
    - `extension/test/helpers/developmentDoc.ts` が、補助のファイルにパスとロケータを置く形の例である（`installGuide.ts` はこれに倣う）。
    - AC-33 で照合するカタログの項目は、`extension/src/core/i18n/ja.ts` にすべて固定の文字列の `text` を持つ形である（`rejection["empty-selection"]` `:30`、`startRejected.invalid_body` `:86`、`setup.api_key_missing` `:115`、`keyRejected` `:130`、`failure.llm_failed` `:158`、`stopped` `:178`）。
    - `extension/scripts/has-extension-changes.sh` の判定（`:45`）と冒頭の説明（`:11-13`）、`extension/test/ciChanges.test.ts` の走らせる行（`:54-56`）と似た名前の行（`:109-114`）が、AC-42 で変える箇所である。
-   **コマンドと規約:** `.claude/commands/japrose.md:147` と `docs/dev/conventions/document_formatting.md:18-19` が README を対象から外す。`.claude/commands/_lib/fable-review.md:483` が文書の種類 `doc` を定める。CLAUDE.md の「Development Notes」の「A change that touches only documents (`README.md`, `README.en.md`, `CLAUDE.md`, `docs/`)」の一覧も、`INSTALL.md` を挙げる必要がある（設計書 3.4 節の表の `make ext-test` の行に含めて直す）。
-   **カバレッジの比較:** `make coverage-compare` と `make ext-coverage-compare` は `Makefile:203`・`:489` にある。
-   **新しい Go の補助のファイル:** 要らない。ロケータは既存の `guides.go` に足す。

## 2. 実装のステップ (Implementation Steps)

各ステップのコミットは、1.2 節の原則に従う。ステップの中で「同じコミット」と書いたものは分けない。

### フェーズ 1: 開発者向けの内容を手引きに移す（設計書 8 章のフェーズ 1、AC-04・AC-39）

#### ステップ 1-1: AC-04 の差分のスクリプト

**変えるファイル:** `docs/tasks/0116_user_readme/_checks/`（新規。シェルのスクリプト 1 つ）

- [ ] 設計書 7.3 節の AC-04 の行のとおりのスクリプトを書く。引数は比べるコミットと、その PR で移すブロックの一覧である。移すブロックの全体の一覧（設計書 2.3 節の「移す」の行。見出しのないブロックも最初と最後の文で決める）をスクリプトの中に持ち、移す作業の最後の PR（本計画では PR-1）のための完全性の確かめを持つ
- [ ] 各確かめが少なくとも 1 つの壊し方で失敗することを確かめ、そのことをコミットのメッセージに書く

**完了の条件:** スクリプトがコミットされ、壊したときに失敗することを確かめてある。

#### ステップ 1-2: README.en.md の部分テストを外す（検査の削除）

**変えるファイル:** `cmd/yt2column/docs_test.go`、`cmd/yt2column/extension_docs_test.go`

- [ ] `TestOpenAIDocsContract`・`TestLLMDefaultsDocsContract`・`TestEffortValuesDocsContract`・`TestProductionProfileDocsContract` と、拡張の ID の部分テストが、`README.md` だけを読むようにする（設計書 3.3 節「外す検査」の 2 行目）
- [ ] このコミットだけで `make coverage-compare BASE=<親のコミット>` を走らせ、1.2 節の原則のとおり記録する

**完了の条件:** 外すだけのコミットがあり、比較が `coverage-compare: PASS` で、記録がコミットのメッセージにある。

#### ステップ 1-3: `Locator` の単体テストを外す（検査の削除）

**変えるファイル:** `internal/doctestutil/readme_test.go`（削除）

- [ ] `TestLocatorIn` と、それだけが使う補助を持つ `readme_test.go` を削除する（設計書 3.3 節「外す検査」の 3 行目）。`Locator` の型は、まだ呼び出し元があるのでステップ 1-4 で除く
- [ ] このコミットだけで `make coverage-compare BASE=<親のコミット>` を走らせ、記録する

**完了の条件:** 外すだけのコミットがあり、比較が通り、記録がある。

#### ステップ 1-4: 内容を移し、検査の読む先と参照を変える

次のすべてを 1 つのコミットにする（README から値を除く変更と、それを読む検査の読む先を変える変更を分けないため）。

**変えるファイル:** `README.md`・`README.en.md`、`docs/dev/guides/generation.md`（新規）・`development.md`・`cli.md`、Go のガードテストと `internal/doctestutil/`、`extension/test/helpers/readmeVersions.ts`・`manifest.test.ts`、`extension/scripts/has-extension-changes.sh`、参照を持つ文書（下の一覧）、CLAUDE.md、ADR-0004・0025・0030

- [ ] **手引きを置く:** 設計書 2.4 節の構成で `generation.md` を置き、移すブロック（設計書 2.3 節の表の「移す」の行のうち `generation.md` 行き）を移す。設定の表を文書の中で列名「変数」を持つ最初の表にする（設計書 3.3 節）
- [ ] **`development.md` に足す:** 設計書 2.4 節の「拡張のビルドと読み込み」（今の README の Go と Node.js の行と `:120-155`）と「拡張のログ」（`:266-268`）を移す
- [ ] **README から除く:** 移したブロックを README から除く（1.3 節の決定 1。冒頭の CLI と構成の文と図を含む）。`## ブラウザ拡張` の見出しと、その下の利用者向けの節（API キーの設定・ポップアップ・ビューア）は、`metadataDocs.test.ts` が読むので残す（設計書 8 章）。README の中の、除いた節へのリンクを `generation.md` か `development.md` へ付け替える。`/mktrans README.md` で同期する
- [ ] **Go のガード:** 設計書 3.3 節の表のとおり、4 つの設定・上限のテストの読む先を `generation.md` に、拡張の ID の読む先を `development.md` に変える。`guides.go` に `generation.md` のロケータを足し、冒頭の説明を 3 つの手引きに直す。`readmeGuides` に `generation.md` を足し、`TestReadmeLinks` が手引きを `readmeGuides` から読むようにする。呼び出し元がなくなる `readmeVersionsUnder`、`Locator` の型、README のロケータを除き、`readme.go` の `Lang` の説明を直す（設計書 3.1 節の該当の行）
- [ ] **拡張のテスト:** `readmeVersions.ts` に `minimumChromeDocs`（この時点では 2 つの README と `development.md`）を足し、`manifest.test.ts` がそれを回すようにする（設計書 3.5 節）
- [ ] **参照の付け替え:** 設計書 4.2 節の表のうちフェーズ 1 の行を付け替える。`cli.md` の範囲の文（`:3-7`）と「設定」の書き出し（`:233-234`）の書き直しと、`:7`・`:58`・`:189`・`:234` のリンク、`development.md:99`、`package_reference.md:103`、`production_profile.md:5`（推奨構成の行き先）、`pricing_update.md:38`・`:40`、`project_overview.md:89`・`:161`、`claude_output_limit.md:60`、`openai_output_limit.md:59`、用語集の「推奨構成」（`:55`）と、見出しが `generation.md` に移る「出力の書式」「設定」（`:180`・`:181`）の備考
- [ ] **説明:** CLAUDE.md の「Development Notes」の Go のガードの一覧（「the configuration block of both READMEs」「the values `cli.md` and `development.md` hold」）を `generation.md` に直す。`make ext-test` の説明と `has-extension-changes.sh` の冒頭の説明に、`development.md` の最低の Chrome の版を加える（設計書 8 章「説明は、説明する変更と同じ PR で直す」）
- [ ] **ADR:** ADR-0025 のずれを示す一文を除く。ADR-0004 と ADR-0030 のずれを示す一文を、`INSTALL.md` の分だけが残ることを示す文に書き直す（設計書 4.2 節、編集上の訂正。訂正の行をコミットのメッセージに書く）
- [ ] **壊して確かめる:** 読む先を変えた検査（4 つの設定・上限のテスト、拡張の ID、`manifest.test.ts` の `development.md`、`TestReadmeLinks` の `generation.md`）を、それぞれ新しい文書で 1 度壊して失敗することを確かめ、コミットのメッセージに書く（設計書 3.3 節。文書全体から値を探す検査は、すべての出現を壊す）

**完了の条件:** `make test`・`make lint`・`make ext-check`・`make deadcode` が通り、`git grep -n 'README' -- ':!docs/tasks'` に、移した節を指す箇所が残っていない。

#### ステップ 1-5: AC-04 の記録

- [ ] ステップ 1-1 のスクリプトを、移したすべてのブロックについて PR のブランチで走らせ、出力とマージベースと比べたコミットを 7 章の AC-04 の記録に書く
- [ ] 残った差分の各行が重複の削除か書式の規約による組み直しであること、リンクの組が正しい節を指すことを、記録に添えてレビュアーに示す

**完了の条件:** 7 章に AC-04 の記録がある。

### PR-1 作成ポイント: move developer content into the guides

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5

**推奨タイトル**: `docs(0116): 開発者向けの内容を README から generation.md と development.md に移す`

**レビュー観点**: 移した文が AC-04 の 4 種類の差分しか持たないか（AC-04 の記録） / 読む先を変えた検査が同じ照合の基準と照らし、壊して失敗することを確かめたか / 外した検査のカバレッジの比較と、値をまだ確かめている検査の記録 / 付け替えた参照とリンクの先

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 1-1 の差分のスクリプト（見出しの範囲の取り出し、正規化、完全性の確かめ）と、ステップ 1-2〜1-4 の検査の削除と読む先の変更の順序が、危険度の高い単独のステップに当たる。Sonnet が正規化で差分を隠すか、外す順を誤ると、移した文の書き換えや、どの検査も読まない値が見逃される

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: INSTALL.md を置く（設計書 8 章のフェーズ 2、AC-03・AC-08〜AC-35・AC-39・AC-42）

#### ステップ 2-1: ブラウザの表示を人に確かめてもらう

- [ ] `AskUserQuestion` で、ステップ 1-4 の後のビルド（`make ext-build` の `extension/dist`）を Chrome と Brave でパッケージ化せずに読み込んだときの警告の文言と、AC-34 のうち確かめられる失敗（`manifest.json` のないフォルダを選んだとき）の文言を、人に貼ってもらう（1.3 節の決定 4）
- [ ] 得た文言と確かめた日付を、`INSTALL.md` に引用する文言として控える。確かめられなかった表示は引用せず、言い換えて書く（AC-18・AC-34）

**完了の条件:** 引用する文言と日付がそろっているか、引用しないと決めた表示が分かっている。

#### ステップ 2-2: README.en.md の英語のカタログとの照合を外す（検査の削除）

**変えるファイル:** `extension/test/metadataDocs.test.ts`

- [ ] `README.en.md` を英語のカタログと照合する 3 つの `it` を外し、`README.md` だけを読むようにする（設計書 3.3 節「外す検査」の 1 行目、1.3 節の決定 6）
- [ ] このコミットだけで `make ext-coverage-compare BASE=<親のコミット>` を走らせ、1.2 節の原則のとおり記録する

**完了の条件:** 外すだけのコミットがあり、比較が通り、記録がある。

#### ステップ 2-3: INSTALL.md を書き、検査と参照を変える

次のすべてを 1 つのコミットにする。

**変えるファイル:** `INSTALL.md`（新規）、`README.md`・`README.en.md`、`extension/test/helpers/installGuide.ts`（新規）・`readmeVersions.ts`、`metadataDocs.test.ts`・`manifest.test.ts`、`extension/scripts/has-extension-changes.sh`・`extension/test/ciChanges.test.ts`、`cmd/yt2column/docs_test.go`、参照を持つ文書（下の一覧）、CLAUDE.md、ADR-0004・0029・0030

- [ ] **INSTALL.md:** 設計書 2.4 節の構成で、AC-08〜AC-35 の内容を日本語で書く。画面の名前とメッセージは日本語のカタログの文を写し、画面の名前は太字で引用する。契約のクレジットの額は写さない（設計書 1.3 節の決定 3）。未リリースの注記は「zip を入手する」に置く（同 決定 7）。ステップ 2-1 の文言を日付とともに引用する。AC-24 の細部は「参考」に置く
- [ ] **README から除く:** 手順（クイックスタート、API キーの設定、表示の言語、ポップアップ、ビューア、ZDR の注記）を README から除き、除いた箇所に `INSTALL.md` へのリンクを置く（1.3 節の決定 6）。`## ブラウザ拡張` の見出しも、読む検査がなくなるので除いてよい。`/mktrans README.md` で同期する
- [ ] **拡張のテスト:** 設計書 3.5 節のとおり `installGuide.ts` を置き、`minimumChromeDocs` に `INSTALL.md` を加え、`browserExtensionHeading` を除く。`metadataDocs.test.ts` が `INSTALL.md` 全体を日本語のカタログと照合するようにし、AC-33 の 6 つのメッセージを照合する `it` を足す（設計書 3.3 節の表）。選んだ項目が文字列でないときは失敗する
- [ ] **Go のガード:** `TestReadmeLinks` が `INSTALL.md` も読むようにする（設計書 3.3 節の表）
- [ ] **CI の判定:** `has-extension-changes.sh` の判定と冒頭の説明に `INSTALL.md` を加え、README の説明を今の検査に合わせる。`ciChanges.test.ts` に、`INSTALL.md` が走らせる行と、`docs/INSTALL.md`・`INSTALL.md.bak` が走らせない行を足す（AC-42）
- [ ] **参照の付け替え:** 設計書 4.2 節の表のうちフェーズ 2 の行（`cli.md:4`・`:16`、`development.md:3`、`security.md:142`）を付け替え、`production_profile.md:5` と `pricing_update.md` に `INSTALL.md` の費用の目安を加える（AC-41）
- [ ] **説明:** CLAUDE.md の「Development Notes」の `make ext-test` が要る文書の一覧と読む内容、「A change that touches only documents」の文書の一覧に `INSTALL.md` を加える（設計書 3.4 節の表、AC-40・AC-42）
- [ ] **ADR:** ADR-0004・ADR-0029・ADR-0030 のずれを示す一文を除く（編集上の訂正）
- [ ] **壊して確かめる:** `INSTALL.md` の版を 141 にする、引用した AC-33 のメッセージの 1 文字を変える、画面の名前の 1 つを変える、`INSTALL.md` のリンクを 1 つ壊す、`has-extension-changes.sh` から `INSTALL.md` を除く、のそれぞれで対応する検査が失敗することを確かめ、コミットのメッセージに書く

**完了の条件:** `make test`・`make lint`・`make ext-check`・`make deadcode` が通る。

#### ステップ 2-4: AC-24 の対応表

- [ ] 今の README `:182-264` の文ごとに、`INSTALL.md` のどの節で述べたかを、7 章の AC-24 の行の下に表で書く（1.3 節の決定 3）。述べていない挙動があれば `INSTALL.md` に足す

**完了の条件:** 対応表があり、どの文にも `INSTALL.md` の節が対応している。

### PR-2 作成ポイント: add the install guide

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4

**推奨タイトル**: `docs(0116): インストールと使い方の手順を INSTALL.md に置く`

**レビュー観点**: INSTALL.md の手順と対処が拡張の挙動と合っているか（AC-09〜AC-35、AC-24 の対応表） / 引用したブラウザの文言が確かめたものだけか / 照合の検査が INSTALL.md 全体と AC-33 のメッセージを読み、英語の照合を外した記録があるか / CI の判定が INSTALL.md を拾うか

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 2-3 が、CI の判定のスクリプトの変更（Conditional checks の CI の判定）と、検査の削除を含む複数の文書と検査の同時の変更（Conditional checks の結び付いた成果物の変更）の 2 つに当たる。Sonnet が README の手順の挙動を INSTALL.md に写し漏らすか、セキュリティの注意（ワークスペースの上限、キーの無効化、入手先）を落とすと、利用者が誤った手順を取る

**自動送信**: hold

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: README を作り直す（設計書 8 章のフェーズ 3、AC-01・AC-02・AC-05・AC-07・AC-36・AC-37）

#### ステップ 3-1: README の作り直しと移行の節の削除

**変えるファイル:** `README.md`・`README.en.md`、`docs/translation_glossary.md`

- [ ] 設計書 2.4 節の構成で README を作り直す。冒頭の「開発者向けの文書」の行（今の `:3`）と、フェーズ 2 の一時的なリンクを除き、「インストールと使い方」「プライバシー」「開発者向けの文書」の節を置く（AC-01・AC-07・AC-36）
- [ ] 「以前の版からの移行」の節とクイックスタートの移行の案内を削除する（AC-05）
- [ ] 用語集の移行の手順の行を直す（1.3 節の決定 5）：「ホスト」（`:61`）と「設定ファイル」（`:171`）の備考から移行の手順を除き、「以前の版からの移行」（`:182`）を除く
- [ ] `/mktrans README.md` で同期する（AC-37）
- [ ] 作り直した README の版の記述が `manifest.test.ts` で照合されることを、版を 141 にして確かめる

**完了の条件:** `make test`・`make lint`・`make ext-check`・`make deadcode` が通る。

#### ステップ 3-2: AC-05 のスクリプトと記録

**変えるファイル:** `docs/tasks/0116_user_readme/_checks/`（シェルのスクリプト 1 つを足す）

- [ ] 設計書 7.3 節の AC-05 の行の (a)〜(c) を確かめるスクリプトを書く。各確かめが少なくとも 1 つの壊し方で失敗することを確かめ、コミットのメッセージに書く
- [ ] PR のブランチで走らせ、出力とマージベースと比べたコミットを 7 章の AC-05 の記録に書く

**完了の条件:** スクリプトが通り、7 章に AC-05 の記録がある。

### PR-3 作成ポイント: rebuild the README for users

**対象ステップ**: 3-1 / 3-2

**推奨タイトル**: `docs(0116): README を利用者向けの概要と案内に作り直す`

**レビュー観点**: README が AC-01 の内容だけを持ち、AC-02 の内容を持たないか / プライバシーの節が AC-36 の 6 項目を正しく述べるか / 2 つの版が同じ内容を述べるか（AC-07） / 移行の節がどこにも残らないか（AC-05 の記録）

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 3-1 のプライバシーの節は security.md §8 の受け入れた危険を利用者に伝える記述で、ステップ 3-2 のスクリプトはマージベースとの組の比較を持つ。Sonnet が送るものや保存の範囲を誤って書くか、組の比較を誤ると、誤った約束か移行の手順の残りが見逃される

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: 残りの参照と規約（設計書 8 章のフェーズ 4、AC-06・AC-38・AC-40・AC-41・AC-43）

#### ステップ 4-1: CLAUDE.md・規約・コマンド

**変えるファイル:** `CLAUDE.md`、`docs/dev/conventions/document_formatting.md`、`.claude/commands/japrose.md`、`.claude/commands/_lib/fable-review.md`、ADR-0035

- [ ] 設計書 3.4 節の表のうち、まだ直していない行を直す。CLAUDE.md の「Documents」（AC-38）、「Document language」（AC-38）、「Architecture Overview」、`document_formatting.md:18-19` と `japrose.md:147`（設計書 1.3 節の決定 4）、`fable-review.md:483`
- [ ] ADR-0035 のずれを示す一文を除く（編集上の訂正）

**完了の条件:** 設計書 3.4 節の表の「変更」の行がすべて反映されている。

#### ステップ 4-2: 残りの参照と用語集

**変えるファイル:** `docs/translation_glossary.md`、`git grep` で見つかる文書

- [ ] `git grep -n 'README' -- ':!docs/tasks'` で探し直し、今の README の節を指す箇所が残っていれば付け替える（AC-06。`security.md:131` は変えない）
- [ ] 用語集の README の見出しの行（`:178-184` のうち、フェーズ 1 と 3 で直していないもの）を直すか除き、`:58` の例を直す（1.4 節）
- [ ] 利用者向けの文書が新しく使う語（API のクレジット、利用の上限、ワークスペースなど、`INSTALL.md` と README の新しい見出しの語）を用語集に足す（AC-43）

**完了の条件:** `make test` が通り、README の古い節を指す箇所が残っていない。

### フェーズ 5: 記録の確かめ（設計書 8 章のフェーズ 5、AC-04・AC-05・AC-24）

#### ステップ 5-1: 記録がそろっていることの確かめ

- [ ] 7 章の AC-04（PR-1）、AC-24（PR-2）、AC-05（PR-3）の記録がそろっていることを確かめる
- [ ] フェーズ 4 の変更の後で、ステップ 3-2 のスクリプトの (a) が通ることを確かめる（(b)・(c) は移行の節がマージベースにある PR-3 でだけ走らせる）

**完了の条件:** 3 つの記録があり、(a) が通る。

### PR-4 作成ポイント: update CLAUDE.md, conventions, and remaining references

**対象ステップ**: 4-1 / 4-2 / 5-1

**推奨タイトル**: `docs(0116): CLAUDE.md と規約と用語集を新しい文書の構成に合わせる`

**レビュー観点**: CLAUDE.md の「Documents」と「Document language」が INSTALL.md の例外と期限と #1044 を述べ、ADR-0035 と一致するか / 規約とコマンドが INSTALL.md を README と同じに扱うか / 用語集の新しい語と古い項目の直し

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない（文書と規約とコマンドの記述の付け替えで、方式の選択も危険度の高いステップもない）

**自動送信**: auto

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | `generation.md`、`development.md` の拡張のビルドとログ、読む先を変えた Go のガードと `manifest.test.ts`、AC-04 の記録 |
| M2（フェーズ 2） | `INSTALL.md`、`metadataDocs.test.ts` の照合、CI の判定、AC-24 の対応表 |
| M3（フェーズ 3） | 作り直した README と英語版、AC-05 の記録 |
| M4（フェーズ 4・5） | CLAUDE.md・規約・コマンド・用語集の付け替え、記録の確かめ |

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 | 開発者向けの内容を `generation.md` と `development.md` に移し、Go のガードと `manifest.test.ts` の読む先を変える | sonnet+opus-review | auto |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 | `INSTALL.md` を置き、拡張のテストと CI の判定を変える | sonnet+opus-review | hold |
| PR-3 | 3-1 / 3-2 | README を作り直し、移行の節を削除する | sonnet+opus-review | auto |
| PR-4 | 4-1 / 4-2 / 5-1 | CLAUDE.md・規約・コマンド・用語集を合わせ、記録を確かめる | sonnet | auto |

PR-2 の自動送信が `hold` なのは、ステップ 2-1 で人がブラウザで表示を確かめるためである（1.3 節の決定 4）。

## 4. テスト戦略 (Test Strategy)

-   **単体テスト（ガードテスト）:** 設計書 3.3 節の表のとおり、値の検査の読む先を変える。新しい検査は、`manifest.test.ts` の `INSTALL.md` と `development.md` の版（AC-08、ステップ 1-4・2-3）と、`metadataDocs.test.ts` の AC-33 のメッセージ（ステップ 2-3）である。`ciChanges.test.ts` に `INSTALL.md` の行を足す（AC-42）。読む先を変えた検査と新しい検査は、壊して失敗することを確かめる（各ステップの「壊して確かめる」）。
-   **外す検査:** 設計書 3.3 節「外す検査」の 3 種を、単独のコミット（ステップ 1-2・1-3・2-2）で外し、カバレッジを比べる。
-   **統合テスト:** なし。実行時の処理を変えない（設計書 7.2 節）。
-   **後方互換:** リポジトリの外からの README のアンカーは保たない（要件 §4.4）。リポジトリの中のリンクは `TestReadmeLinks` と `TestInstructionDocPaths` が確かめる。
-   **`diff` 検証:** AC-04（ステップ 1-1・1-5）と AC-05（ステップ 3-2）は `_checks/` のシェルのスクリプトで確かめる（設計書 7.3 節）。どちらも、移すブロックか移行の節がマージベースにある PR で走らせる。
-   **読み合わせ:** 手順・対処・プライバシーの記述の正しさは、PR のレビューで読む（`manual`。要件 §5、ADR-0016）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 移すときに文を落とすか書き換え、述べる挙動が黙って変わる | AC-04 のスクリプトが正規化の後の差分を出し、レビュアーが残りを読む。完全性の確かめで、移し忘れたブロックを失敗にする（ステップ 1-1・1-5） |
| 値の検査を外すか読む先を変えたとき、どの検査も読まない値のコピーが残る | 値を除く変更と読む先を変える変更を同じコミットに入れ、外す検査は単独のコミットで比べて記録する（1.2 節） |
| `INSTALL.md` が拡張の挙動と食い違う | 画面の名前とメッセージはカタログとテストで照合する。挙動は AC-24 の対応表と PR のレビューで読む |
| 確かめていないブラウザの警告を「想定どおり」と書き、利用者が警告を読み飛ばす | 引用はステップ 2-1 で人が確かめた文言と日付に限る（1.3 節の決定 4） |
| PR-2 と PR-3 の間の `main` で README から手順への道が切れる | フェーズ 2 で除いた箇所に `INSTALL.md` へのリンクを置く（1.3 節の決定 6） |
| `make deadcode` が使われない補助を報告して失敗する | 最後の呼び出し元を除くコミットで、`readmeVersionsUnder`・`Locator`・`browserExtensionHeading` を除く（ステップ 1-4・2-3） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5）
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4）
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2）
- [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 5-1）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

`make test` が Go のガードを、`make ext-test` が拡張のテストを走らせる。

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `test`、`manual` | 必要なブラウザの版: `extension/test/manifest.test.ts`（`minimumChromeDocs` の `README.md`）。ほかの内容: PR-3 のレビューで読む |
| AC-02 | `manual` | PR-3 のレビューで、2 つの README が挙げた内容を持たないことを読む（設計書 7.3 節） |
| AC-03 | `test`、`manual` | `INSTALL.md` があること: `extension/test/metadataDocs.test.ts` と `manifest.test.ts`（文書がないと失敗する）。節の内容と順: PR-2 のレビューで読む |
| AC-04 | `diff`、`test` | `_checks/` の AC-04 のスクリプト（下の記録）。移した値: `cmd/yt2column/docs_test.go::TestProductionProfileDocsContract`・`TestLLMDefaultsDocsContract`・`TestEffortValuesDocsContract`・`TestOpenAIDocsContract`、`cmd/yt2column/extension_docs_test.go::TestExtensionDocumentation` |
| AC-05 | `diff` | `_checks/` の AC-05 のスクリプト（下の記録） |
| AC-06 | `test`、`manual` | リンクとアンカー: `cmd/yt2column/docs_test.go::TestReadmeLinks`、`cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`。リンクのない名指し: 各 PR のレビューで読む |
| AC-07 | `test`、`manual` | `INSTALL.md` へのリンク: `TestReadmeLinks`。2 つの版の構造: `cmd/yt2column/readme_structure_test.go::TestReadmeVersionsShareStructure`。述べる内容: PR-3 のレビューで読む |
| AC-08 | `test` | `extension/test/manifest.test.ts`（`minimumChromeDocs` の `INSTALL.md`） |
| AC-09〜AC-13 | `manual` | PR-2 のレビューで読む。AC-12 の「約 0.4 USD」は `production_profile.md` の値を丸めたもので、照合するコードの値はない |
| AC-14〜AC-19 | `manual` | PR-2 のレビューで読む。AC-18 の引用は、ステップ 2-1 で確かめた文言と日付であることを読む |
| AC-20〜AC-22 | `test`、`manual` | 画面の名前: `extension/test/metadataDocs.test.ts`。手順と結果: PR-2 のレビューで読む |
| AC-23 | `test`、`manual` | 画面の名前: `extension/test/metadataDocs.test.ts`。流れの順: PR-2 のレビューで読む |
| AC-24 | `manual` | PR-2 のレビューで、下の対応表を読む |
| AC-25 | `test` | `extension/test/metadataDocs.test.ts`（概要欄のメッセージと手順、工程の名前、画面の名前） |
| AC-26〜AC-32 | `manual` | PR-2 のレビューで読む |
| AC-33 | `test`、`manual` | メッセージ: `extension/test/metadataDocs.test.ts`（AC-33 の 6 つのメッセージの `it`）。対処: PR-2 のレビューで読む |
| AC-34、AC-35 | `manual` | PR-2 のレビューで読む |
| AC-36 | `manual` | PR-3 のレビューで読む |
| AC-37 | `test` | `cmd/yt2column/readme_sync_test.go::TestReadmeSyncMarker`、`cmd/yt2column/readme_structure_test.go::TestReadmeVersionsShareStructure` |
| AC-38 | `manual` | PR-4 のレビューで、CLAUDE.md の「Documents」と「Document language」を読む |
| AC-39 | `test` | 設計書 3.3 節の表の各テスト（`manifest.test.ts`、`metadataDocs.test.ts`、`extension_docs_test.go::TestExtensionDocumentation`、`docs_test.go` の 4 つの設定・上限のテストと `TestReadmeLinks`）。読む先を変えた検査を壊して失敗させた記録と、外した検査のカバレッジの比較の記録は、PR-1 と PR-2 のコミットのメッセージにある |
| AC-40 | `manual` | PR-1（Go のガードの一覧）と PR-2（`make ext-test` の一覧）のレビューで、CLAUDE.md の「Development Notes」を読む |
| AC-41 | `manual` | PR-1 と PR-2 のレビューで、`production_profile.md:5` と `pricing_update.md` が新しい置き場所を挙げることを読む |
| AC-42 | `test`、`manual` | CI の判定: `extension/test/ciChanges.test.ts`。CLAUDE.md の一覧: PR-2 のレビューで読む |
| AC-43 | `manual` | PR-4 のレビューで、用語集を読む |

### AC-04 の記録

（PR-1 のステップ 1-5 で、スクリプトの出力とマージベースと比べたコミットを書く。）

### AC-05 の記録

（PR-3 のステップ 3-2 で、スクリプトの出力とマージベースと比べたコミットを書く。）

### AC-24 の対応表

（PR-2 のステップ 2-4 で、今の README `:182-264` の文ごとに、`INSTALL.md` のどの節で述べたかを書く。）

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-43 がすべて確かめられている。
-   各 PR で `make test`・`make lint`・`make ext-check`・`make deadcode` が通る。
-   外した検査について、カバレッジの比較が通り、記録がコミットのメッセージと PR の説明にある。
-   セキュリティの規則と前提を変えていない（設計書 5 章）。利用者に伝える危険と対処を、PR のレビューで読んでいる。

## 9. 次のステップ (Next Steps)

-   #1039 が、未リリースの注記を外し、来歴の検証の手順を `INSTALL.md` に足す（設計書 4.3 節）。
-   #1044 で `INSTALL.md` の英語版を足すとき、英語のカタログとの照合と `minimumChromeDocs` の英語版を戻す（設計書 9 章）。
-   最後の PR のマージの後、issue #1042 を閉じる。
