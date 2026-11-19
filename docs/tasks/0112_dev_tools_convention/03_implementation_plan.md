# 実装計画書：開発の道具の規則を ADR-0014 から規約 dev_tools.md に移す

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-19 |
| Review date | 2026-11-19 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番、H-NN は [design_handoff.md](design_handoff.md)、I-NN は [implementation_handoff.md](implementation_handoff.md) の項目を指す。節番号は、とくに断らない限り [02_architecture.md](02_architecture.md)（以下、設計書）の節である。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

英語の規約 `docs/dev/conventions/dev_tools.md` を作り、開発の道具の規則 1〜6 の正本をそこに移す。ADR-0014 の 7 つの項目を要約と規約へのリンクに書き直し、`CLAUDE.md`・コマンドの定義・ガイド・スクリプトのコメントを規約に合わせて直し、`.claude/settings.json` に拒否の規則を 2 つ足す（設計書 1 章・3 章）。

### 1.2. 実装の原則

-   **書く中身の出どころ。** 書く中身は設計書に従う。規約の見出しと各節が持つものは設計書 3.1、ADR-0014 から外す文の行き先は 3.2、`CLAUDE.md` の 4 種類の箇所は 3.3、主語の直しは 3.4、例外への参照は 3.5、参照の付け替えは 3.6、拒否の規則と用語集は 3.7 に従う。本計画は、それらを繰り返さない。
-   **規則の内容。** 規則の内容は変えない。規約の各節は、ADR-0014 の対応する項目の条件を落とさず、広げない（AC-02）。行き先の決まっていない文は、ADR-0014 から外さない（設計書 1.1）。
-   **直す場所の探し方。** 直す場所は中身で探す。設計書の行番号は `e43f8161` のもので、`CLAUDE.md` はその後のコミットで行がずれている（1.4）。
-   **既存のガードが読む値と構造。** これらは変えない。書式の規約に揃える直しでも、ガードが読む見出し・表・値は変えない。直す箇所を読むテストは設計書 3.8 の表のとおりで、どのテストも直さない。`mergepr_guide.md` の `--protocol` を含む記述（設計書 3.4）と、`CLAUDE.md`「Tool Execution Safety」の `numreserve` の呼び出しの形（設計書 3.3）には触れない。
-   **日本語の文書の書式。** ADR-0014 の「決定」の節と、中身を直すガイドの節は、`docs/dev/conventions/document_formatting.md` に揃える（AC-10・AC-22、設計書 3.2「『決定』の節の書式」・3.4・3.5）。揃えるのは書式だけである。
-   **編集上の訂正の記録。** ADR-0014 の書き換えは、コミットのメッセージに訂正行を書き、PR の説明に訂正の段落を書く（AC-11、設計書 3.2「記録」）。書式の規約に揃える直しも、編集上の訂正の 1 つ目の種類（基準に沿った書き直し）として記録する（`docs/dev/conventions/adr.md`「Format」）。訂正行は文を写さずに自分の言葉で書き、`<` と `@` を含めない（I-03）。

### 1.3. 計画の際に決めたこと（2026-11-19）

-   **差分の範囲の AC の確かめ方。** 差分の範囲の AC（AC-10・AC-18・AC-20・AC-21・AC-22）と AC-24 は、記録した diff を読んで確かめる。2026-11-19 に人が選んだ。どれも小さな文書の差分で、スクリプトで判定するより読むほうが確かなためである（`docs/dev/conventions/requirements_process.md` §4「`diff` verification」の「Proportion」）。I-02 のとおり、AC ごとに記録を持つ。読み切れないと分かったら、同じ節の「Output too long to read」に従う。
    -   採らなかった案：AC ごとに `_checks/` のシェルのスクリプトを書く案。読む量は減るが、作る量と、壊して確かめる量が増える。
    -   ADR: なし。確かめ方の選択で、本タスクの中で閉じる。
-   **拒否の規則の効き方の確かめ。** 拒否の規則が Claude Code で効くことは、PR のレビューで人が確かめる（`manual`）。2026-11-19 に人が選んだ。理由は次のとおりである。実装モデル要件が `sonnet` の `/runplan` は OpenCode で走る（`.claude/commands/_context.md`「Command routing」）。OpenCode には `.claude/settings.json` の拒否の規則が効かない（要件書 1 章の「要件定義の際に決めたこと」の拒否の規則の項目）。そのため、設計書 7 章の確認を `/runplan` の中では行えない。人は、PR のブランチを checkout した作業ツリーで Claude Code のセッションを開いて確かめる。結果は PR のコメントに書き、`/runpr` のマージの質問に答える前に済ませる。確かめるのは PR を作った後なので、自動送信は `auto` のままとする（3.2）。
    -   採らなかった案：自動送信を `hold` にし、`/runplan` の前後に人が対話のセッションで確かめる案。PR のレビューで確かめれば足りる。
    -   採らなかった案：Claude Code では確かめず、`cmd/chief/docs_test.go` の `matchesRule` のモデルに頼る案。設計書 7 章と食い違う。
    -   ADR: なし。確かめ方の選択で、本タスクの中で閉じる。
-   **AC-11 の訂正行の確かめ方。** AC-11 の訂正行は、`_checks/` のシェルのスクリプトで確かめる（`static`）。2026-11-19 に人が選んだ。訂正行はコミットのメッセージにあるが、今の `/mergepr` の手順はそれを読まない（`.claude/commands/mergepr.md` に訂正行を扱う手順がない。`efb51aac` で確かめた）。`mergepr` の道具には訂正行を読んで分ける処理があるが（`internal/mergepr/corrections.go`、`1b8da1cf`）、手順にはまだ組み込まれていない。読むだけでは、AC に `test`・`static`・`diff` のいずれかの確かめを持たせる規則を満たせない。PR の説明の訂正の段落は、PR のレビューで読む（`manual`）。
    -   採らなかった案：コミットのメッセージと PR の説明を読むだけにする案。
    -   ADR: なし。確かめ方の選択で、本タスクの中で閉じる。
-   **書式の規約の当て方。** ADR-0014 の「決定」の節と、AC-16・AC-23 で中身を直すガイドの節は、日本語の文書の書式の規約 `docs/dev/conventions/document_formatting.md` に揃える。揃えるのは書式（太字の使い方と、項目の多い節の構成）だけで、中身は変えない。このために、要件書の AC-10・AC-22 と 2.3、設計書の 3.2・3.4・3.5・3.6 と 7 章を本 PR で直し、両方を `draft` に戻した。2026-11-19 に人が選んだ。書式の規約は、要件書の承認と、設計書が基にしたコミット `e43f8161` の後に入った（`94e3a1f5`、タスク 0111）。この規約は、項目の中身を書き換える節を、触れない項目も含めて節全体で揃えるよう求める（`document_formatting.md`「Where the rules apply」）。これが、今の AC-10・AC-22 の差分の限定とぶつかる。
    -   採らなかった案：差分の限定を優先し、本タスクでは書式の規約を当てない案。後から入った規約から外れた節が残る。
    -   採らなかった案：本タスクの書き換えを、規約の義務の範囲に入らない編集上の訂正と読む案。規約が義務から除くのは、リンク・名前・見出しの直しなどだけである。
    -   ADR: なし。書式を揃えることは編集上の訂正の 1 つ目の種類（`docs/dev/conventions/adr.md`「Editorial corrections」）で、ADR-0014 の決定を変えない。
-   **フェーズと PR の数。** フェーズは 1 つ、PR は 1 本とする。I-01（確かめ済み）と設計書 8 章のとおりである。ステップの順は設計書 8 章に従う。実装モデル要件は I-01 の決定（`sonnet`）を採る。I-01 は「文書だけ」の PR を前提にしていたので、拒否の規則・確かめのスクリプト・書式の直しが範囲に加わった後に、2026-11-19 に人が `sonnet` のままとすることを確かめ直した。拒否の規則は設計書 3.7 が形まで決めた 2 行で、効き方は PR のレビューで人が確かめるためである。
    -   採らなかった案：`sonnet+opus-review` または `opus` とする案（拒否の規則の追加を、`mkplan2.md` step 4 のセキュリティの関門のステップと見る案）。
    -   ADR: なし（I-01 のとおり）。

### 1.4. 既存コード調査結果

コミット `efb51aac`（`origin/main`）で確かめた。

-   **設計書の行番号のずれ。** 設計書は `e43f8161` で確かめた。`e43f8161..efb51aac` で、本タスクが変えるファイルのうち変わったのは `CLAUDE.md`・`docs/dev/guides/chief_guide.md`・`docs/translation_glossary.md` だけである（`git diff --stat e43f8161 efb51aac -- <AC-21 のファイル>`）。
    -   `CLAUDE.md` は、`:35` の後に 4 行、`:582` の後に 30 行が足された。そのため、設計書の `CLAUDE.md:1051-1052`・`:1060-1061`・`:1076-1077` は今の `:1085-1086`・`:1093-1095`・`:1110-1111`、`:842-843` は今の `:876-877` である。
    -   `chief_guide.md` の変更は 3.1 節と 5 章だけで、本タスクが直す `:22` と `:30` は変わっていない。
    -   用語集の変更は「ラウンド」の行の後への 5 行の追加で、設計書 3.7 が 3 行を足す位置の「ガードツール」の行（`docs/translation_glossary.md:134`）は変わっていない。
-   **書式の規約と訂正行の道具。** `e43f8161..efb51aac` で、日本語の文書の書式の規約 `docs/dev/conventions/document_formatting.md` と、`docs/dev/conventions/adr.md` の「Format」が入った（`94e3a1f5`、タスク 0111）。ADR-0014 の「決定」の項目は、今は文全体が太字で（`docs/dev/adr/0014-deterministic-checks-in-go.md:17-26`）、規約の「Bold」に合わない。中身を直すガイドの節は、`mergepr_guide.md` の 3 章（`:38-51`）と 7 章（`:252-304`）、`fixprguard_guide.md` の 3 章（`:36-59`）、`chief_guide.md` の 2 章（`:16-43`）、`numreserve_guide.md` の 1 章（`:5-26`）である。また、`mergepr` の道具に訂正行を読んで分ける処理が入った（`internal/mergepr/corrections.go`、`1b8da1cf`）。
-   **ADR-0014。** `e43f8161` から変わっていない。「決定」の項目は `:17`〜`:26`、「結果」の拒否の規則の文は `:61` で、設計書 3.2 の表のとおりである。
-   **直す箇所の今の文。** 設計書 3.4〜3.6 の表の「今の文」は、`efb51aac` でも同じである。`.claude/commands/mergepr.md:18-19`、`.claude/commands/_lib/local-cli-review.md:91-93`、`.claude/commands/_lib/adr-correction-candidates.sh:63-64`、`docs/dev/guides/numreserve_guide.md:11`、`docs/dev/guides/chief_guide.md:22`・`:30`、`docs/dev/guides/fixprguard_guide.md:54`、`docs/dev/guides/mergepr_guide.md:48`・`:286` を読んで確かめた。
-   **`.claude/settings.json`。** `permissions.deny` は 33 の規則（`herdr` と `git worktree`）で、最後は `Bash(git worktree move*)`（`.claude/settings.json:46`）である。`make` と `go` の規則はない。
-   **読むテスト。** 設計書 3.8 の表のテストは、すべて次のファイルにある。テストのコードは変えない（AC-21 にテストのファイルは入らない）。
    -   `cmd/yt2column`：`TestADRRecords`（`adr_test.go`）、`TestInstructionDocPaths`（`docpaths_test.go`）、`TestCodexMentionOnlyInRequestRow`（`codex_review_docs_test.go`）、`TestADRCorrectionCandidates`（`adr_candidates_docs_test.go`）
    -   `cmd/mergepr/docs_test.go`：`TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`・`TestProtocolToolsAgree`
    -   `cmd/codexreview/docs_test.go`：`TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`・`TestNoDirectCodexInvocation`・`TestSettingsAllowsTool`
    -   `cmd/chief/docs_test.go`：`TestSettingsDeniesPaneInput`・`TestSettingsDeniesWorktreeChanges`・`TestSettingsAsksBeforeRemove`・`TestGuideValuesMatchTool`
    -   `cmd/numreserve/docs_test.go`：`TestDocumentedInvocations`・`TestSettingsAllowsTool`・`TestGuideValuesMatchTool`
    -   `cmd/fixprguard/docs_test.go`：`TestGuideTable`・`TestGuideCoversStopKindsAndScope`・`TestDocumentedProtocolMatches` などのガイドの表と値を読むテスト
-   **規約の置き場所を読むテスト。** `cmd/chief`・`cmd/mergepr`・`cmd/codexreview`・`cmd/fixprguard`・`cmd/numreserve` の文書のテストは、`docs/dev/conventions/` のうち `adr.md` だけを読み（`cmd/numreserve/docs_test.go:197`）、ほかの規約を走査しない。新しい規約が読まれるのは `TestInstructionDocPaths`（リンクの解決）だけである。
-   **再利用。** 新しいテストも補助のファイルも足さない（設計書 7 章）。確かめは既存のガードと diff の記録、AC-11 のスクリプトで行う。`_checks/` のスクリプトは、本タスクで新しく書く。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 規約の作成と、規則の置き場所の付け替え（設計書 3 章・8 章、AC-01〜AC-25）

ステップごとの確かめ（I-04）では、そのステップが変えるファイルを読むパッケージのテストを走らせる。対象は `go test -tags test` に、ステップごとに示すパッケージを渡す。

#### ステップ 1-1: 規約と用語集

**変えるファイル:** `docs/dev/conventions/dev_tools.md`（新規）、`docs/translation_glossary.md`

- [x] 設計書 3.1 の見出しの表と、規則 5 の段落の表に従って、規約を英語で書く（AC-01〜AC-07）。規則ごとに 1 つの節とし、理由は書かずに ADR-0014 へリンクする
- [x] 規約の規則 5 の節に、設計書 3.1 の「止まって頼むときの決まり」を書く（AC-05）
- [x] 用語集の「ガードツール」の行の後に、設計書 3.7 の 3 行を足す（H-03）
- [x] 規約を書き終えたら、書き直す前の ADR-0014（PR の merge-base の版）の、規約に移す 7 つの項目（`:17`・`:18`・`:20`・`:22`・`:23`・`:25`・`:26`）の条件を 1 つずつ規約の該当する節と照らし、落ちた条件と広がった条件がないことを確かめる（AC-02）。規約は ADR-0014 の文を写したものではないので、照らした結果は PR の説明に書く

**完了の条件:** `go test -tags test ./cmd/yt2column` が通る（規約のリンクが解決する）。

#### ステップ 1-2: ADR-0014 の 7 つの項目の書き直し

**変えるファイル:** `docs/dev/adr/0014-deterministic-checks-in-go.md`

- [x] 設計書 3.2 の表の 7 つの項目（`:17`・`:18`・`:20`・`:22`・`:23`・`:25`・`:26`）を、「外す文の行き先」の表のとおりに書き直す（AC-09）。規約の節は、アンカーを使わず見出しの名前を本文で示してリンクする（`docs/dev/conventions/adr.md`「Links」）
- [x] 「決定」の節を、`:19`・`:21`・`:24` の 3 項目も含めて、書式の規約に揃える（AC-10、設計書 3.2「『決定』の節の書式」）。3 項目は書式だけを直し、中身は変えない
- [x] 「背景」「採らなかった案」「結果」「経緯」には触れない（AC-10）。「結果」の `:61` の文も直さない（設計書 3.2「『結果』の 1 文」）
- [x] コミットのメッセージに、ADR-0014 の訂正行を書く（AC-11）。消す 2 つの参照（「（タスク 0112 の実装で足す）」と「規則は…0112 要件定義書にある」）は古くなった参照、それ以外は基準に沿った書き直しとして書き分ける（設計書 3.2「記録」）。3 項目の書式の直しも、基準に沿った書き直しとして書く。訂正行は自分の言葉で書き、`<` と `@` を含めない（I-03）

**完了の条件:** `go test -tags test ./cmd/yt2column` が通る（`TestADRRecords` と、ADR のリンクの解決を含む）。

#### ステップ 1-3: CLAUDE.md の 4 種類の箇所

**変えるファイル:** `CLAUDE.md`

- [x] 「Quick Links」の「Development Guides」に、規約へのリンクと要旨の行を足す（AC-13、設計書 3.3）
- [x] 「Tool Execution Safety」に、設計書 3.3 の 1 項目を足す（AC-14）。道具を名前で並べず、`numreserve task` のような呼び出しの形をコードスパンで書かない
- [x] 「Tool Execution Safety」の `codexreview`・`numreserve`・`chief` の「install or build」の文に、開発のための例外があることを規約へのリンクで添える（AC-23）。今の文は残す
- [x] 「Protocol numbers」の「The merge checks it.」の項目の「reinstall the tool from the branch」を、利用者が入れ直すことを述べる形にする（AC-15）

**完了の条件:** `go test -tags test ./cmd/yt2column ./cmd/numreserve ./cmd/codexreview ./cmd/chief ./cmd/mergepr` が通る（`TestDocumentedInvocations`・`TestCodexMentionOnlyInRequestRow`・リンクの解決を含む）。

#### ステップ 1-4: コマンドの定義・ガイド・スクリプトのコメント

**変えるファイル:** `.claude/commands/mergepr.md`、`.claude/commands/_lib/local-cli-review.md`、`.claude/commands/_lib/adr-correction-candidates.sh`、`docs/dev/guides/mergepr_guide.md`、`docs/dev/guides/fixprguard_guide.md`、`docs/dev/guides/chief_guide.md`、`docs/dev/guides/numreserve_guide.md`

- [x] `mergepr.md` の `mergepr` を入れる指示を、設計書 3.4 のとおりに直す（AC-17）
- [x] `mergepr_guide.md` の `:48` と 7 章の手順 5、`fixprguard_guide.md:54`、`chief_guide.md:30` の主語を、設計書 3.4 のとおりに利用者にする（AC-16）。`mergepr_guide.md:48` は主語の部分だけを直し、`--protocol` を含む記述と「マージの直後に `main` から再インストールする」は変えない
- [x] `local-cli-review.md:91-93` と `numreserve_guide.md:11` に、開発のための例外への参照を設計書 3.5 のとおりに添える（AC-23）
- [x] `chief_guide.md:22` と `numreserve_guide.md:11` の ADR-0014 への参照を、規約の規則 5 の節へのリンクに付け替える（AC-19、設計書 3.6）
- [x] 中身を直したガイドの節（設計書 3.4・3.5 に挙げた節）を、書式の規約に揃える（AC-22）。中身と、ガードが読む見出し・表・値は変えない。AC-19 のリンクの付け替えだけでは、節を揃える義務は生じない
- [x] `adr-correction-candidates.sh:63-64` のコメントの ADR-0014 のパスを、規約のパスに付け替える（AC-20）。コードは変えない

**完了の条件:** `go test -tags test ./cmd/yt2column ./cmd/mergepr ./cmd/codexreview ./cmd/chief ./cmd/numreserve ./cmd/fixprguard` が通る（`TestADRCorrectionCandidates`・`TestDocumentedProtocolMatches`・`TestGuideValuesMatchTool`・リンクの解決を含む）。

#### ステップ 1-5: 拒否の規則

**変えるファイル:** `.claude/settings.json`

- [x] `permissions.deny` の最後に、設計書 3.7 の 2 つの規則（`Bash(make install-*)` と `Bash(go install *)`）を足す（AC-24）。ほかの規則は、並びも含めて変えない

**完了の条件:** `go test -tags test ./cmd/chief ./cmd/codexreview ./cmd/numreserve` が通る（`settings.json` を読むテスト）。

#### ステップ 1-6: 確かめの記録と仕上げ

**変えるファイル:** `docs/tasks/0112_dev_tools_convention/_checks/correction_lines.sh`（新規）、本計画書

- [x] AC-11 のスクリプトを `_checks/correction_lines.sh` に書く。merge-base から `HEAD` までの範囲を 1 つにまとめて、ADR-0014 を変えたコミットのどれかに ADR-0014 のパスを名指す訂正行が 1 つ以上あることと、範囲のどの訂正行も `<` と `@` を含まず、決まった形（パス、空でない説明、`; no decision changed` の終わり）を持つことを確かめる。ADR-0014 を変えたコミットのそれぞれに訂正行を求めない。PR が足した文を後のコミットで直すことは編集上の訂正ではないためである（`docs/dev/conventions/adr.md`「After the PR is created」）。訂正行は、行の 1 文字目から `Editorial correction: ` で始まる行だけとする（`requirements_process.md`「Recording in the PR description」）。言語はシェルとする。各確かめが少なくとも 1 つの壊し方で失敗することを確かめ、そのことをコミットのメッセージに書く。壊し方は、作業ツリーの外の使い捨てのクローンに作ったコミットで試し、PR のブランチの履歴は書き換えない
- [x] スクリプトを PR のブランチで走らせ、出力と、merge-base と比べたコミットの ID を 7 章に記録する。記録の後に ADR-0014 を変えるコミットか訂正行を持つコミットが足されたら（`/fixpr` のラウンドなど）、走らせ直して記録し直す
- [x] `make test` と `make lint` を通す（AC-12・AC-25）
- [x] 7 章の diff の記録（AC-10・AC-18・AC-20・AC-21・AC-22・AC-24）を、`requirements_process.md` §4 の決まったコマンドで取り、書く。AC-21 の記録は、PR の実装の最後のコミットで取る（I-02）
- [ ] PR の説明に、ステップ 1-1 の AC-02 の照らし合わせの結果と、7 章の AC-24 の Claude Code での確認の手順（人が行い、結果を PR のコメントに書く）を書く

**完了の条件:** グリーンゲートが通り、7 章の記録がすべてある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | 規約 `dev_tools.md`、用語集の 3 行、ADR-0014 の書き直しと訂正の記録、`CLAUDE.md` の 4 種類の箇所、コマンドの定義・ガイド・スクリプトのコメントの直し、拒否の規則、AC-11 のスクリプトと 7 章の記録 |

ステップは設計書 8 章の順に進める。規約を先に作るのは、後のステップがリンクする先を先に作るためである。

### 3.2 PR 構成

実装は 1 本の PR で行う。フェーズ 1 のステップ 1-1〜1-6 を含む。AC-24 の Claude Code での確認は、PR を作った後に、PR のレビューで人が行う（1.3）。

**実装モデル要件**: sonnet

**判定理由**: どのトリガーにも当たらない（文書と書式の直し、設計書 3.7 が形まで決めた拒否の規則 2 行、小さな確かめのスクリプトだけで、方式の選択はなく、拒否の規則の効き方は人が PR のレビューで確かめる。範囲が広がった後に 2026-11-19 に人が確かめ直した）

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   **単体テスト・結合テスト:** 足さない（設計書 7 章）。AC-21 は、実装の PR が変えてよいファイルにテストのファイルを入れていない。
-   **既存のガード:** 設計書 3.8 の表のテストを、ステップごとの完了の条件で走らせ、ステップ 1-6 で `make test` 全体を走らせる（AC-12・AC-25、I-04）。
-   **差分の確かめ:** 7 章の diff の記録を PR のレビューで読む（1.3）。
-   **訂正行:** `_checks/correction_lines.sh` を PR のブランチで一度走らせ、記録する（AC-11）。常設のテストではない。
-   **拒否の規則の効き方:** 人が、PR のブランチの Claude Code のセッションで一度確かめ、結果を PR のコメントに書く（7 章の AC-24）。設計書 3.7 の表の各例が当たるか当たらないかは、PR のレビューで規則の形に照らして読む。引数のない `go install` は、拒まれなかったときにパッケージを入れてしまうので試さない（設計書 7 章）。
-   **文の内容:** 規約・ADR・`CLAUDE.md`・ガイドの文は、PR のレビューで読む（`CLAUDE.md`「Testing Strategy」）。
-   **後方互換:** N/A。道具とコマンドの振る舞いと、プロトコル番号を変えない（要件書 4.4）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 規約を書くときに、ADR-0014 の条件を落とすか広げる | ステップ 1-1 で、ADR-0014 の項目ごとに規約と照らし、結果を PR の説明に書く。レビューで読む（AC-02） |
| ADR-0014 から、行き先のない文を外す | 設計書 3.2 の「外す文の行き先」の表の行ごとに書き直す。AC-10 の diff の記録をレビューで読む |
| 文の直しで、ガードが読む値（`--protocol N`、`numreserve` の呼び出しの形、`@codex`）を壊す | 設計書 3.3・3.4 の注意に従い、ステップごとに読むパッケージのテストを走らせる（I-04） |
| 書式の規約に揃える直しで、中身を変えるか、ガードが読む見出し・表・値を壊す | 書式だけを直す。ステップごとに読むパッケージのテストを走らせ、AC-10・AC-22 の diff の記録をレビューで読む |
| 訂正行に `<` や `@` が入る | 自分の言葉で書く（I-03）。`_checks/correction_lines.sh` で確かめる |
| 拒否の規則の形を誤り、守りが効かないか、`make test` などを拒む | 設計書 3.7 の形のとおりに書き、PR のレビューで人が Claude Code で確かめる（AC-24） |
| 本タスクのマージの前に、ほかの PR が同じファイル（`CLAUDE.md`、ガイド、`settings.json`）を変える | 後からマージする側が `origin/main` を取り込んで解く。直す場所は中身で探す（1.2） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-6）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `static`・`manual` | `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（`CLAUDE.md` からのリンクが解決し、規約のファイルがある）。英語で書かれ、規則ごとに 1 つの節があることは、PR のレビューで読む |
| AC-02 | `manual` | PR のレビューで、規約の各節を、PR の merge-base の ADR-0014（書き直す前の版）の対応する項目と照らして読む。ステップ 1-1 の照らし合わせの結果（PR の説明）を手がかりにする |
| AC-03 | `manual` | PR のレビューで、規約の Kinds of tools を読む |
| AC-04 | `manual` | PR のレビューで、規約の Rule 2 を読む |
| AC-05 | `manual` | PR のレビューで、規約の Rule 5 の各段落を読む |
| AC-06 | `manual` | PR のレビューで、規約の冒頭のリンクを読む |
| AC-07 | `manual` | PR のレビューで、規約の Not covered here を読む |
| AC-08 | `static`・`manual` | 要件定義の PR（#1024）で満たし、そのレビューで読んだ。ADR の構造は `cmd/yt2column/adr_test.go::TestADRRecords` が確かめる。AC-08 で足した「結果」「採らなかった案」「経緯」の部分が実装の PR で変わらないことは、AC-10 の記録で読む。書き直した `:23` の項目が要約と理由を持つことは、AC-09 の読み取りで読む |
| AC-09 | `static`・`manual` | `cmd/yt2column/adr_test.go::TestADRRecords`（ADR の構造）、`cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（規約へのリンクの解決）。7 つの項目が規則の本文を持たないことは、PR のレビューで書き直した後の ADR-0014 を読む |
| AC-10 | `diff` | 下の記録で、差分が 7 つの項目の書き換えと「決定」の節の書式の直しに限られ、`:19`・`:21`・`:24` の 3 項目の中身が変わらないことを読む |
| AC-11 | `static`・`manual` | `docs/tasks/0112_dev_tools_convention/_checks/correction_lines.sh`（下の記録）。PR の説明の訂正の段落は、PR のレビューで読む |
| AC-12 | `static` | `make test`（`cmd/yt2column/adr_test.go::TestADRRecords` を含む） |
| AC-13 | `static`・`manual` | `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（リンクの解決）。要旨は PR のレビューで読む |
| AC-14 | `static`・`manual` | `cmd/numreserve/docs_test.go::TestDocumentedInvocations`（節の `numreserve` の呼び出しの形が変わらない）、`cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（規約へのリンクの解決）。項目の内容は PR のレビューで読む |
| AC-15 | `manual` | PR のレビューで、`CLAUDE.md`「Protocol numbers」の直した文を読む。差分の位置は AC-18 の記録で読む |
| AC-16 | `static`・`manual` | `cmd/mergepr/docs_test.go::TestDocumentedProtocolMatches`（`mergepr_guide.md` の `--protocol N` が変わらない）。4 つの箇所の主語は PR のレビューで読む |
| AC-17 | `static`・`manual` | `cmd/mergepr/docs_test.go::TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`（`mergepr.md` の呼び出しと番号が変わらない）。指示の主語は PR のレビューで読む |
| AC-18 | `diff` | 下の記録で、`CLAUDE.md` の差分が AC-13・AC-14・AC-15・AC-23 の箇所に限られることを読む |
| AC-19 | `static`・`manual` | `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（2 つのガイドから規約へのリンクの解決）。リンクが規則 5 の節を見出しの名前で指すことは、PR のレビューで読む |
| AC-20 | `diff`・`test` | 下の記録で、差分がコメントの行だけであることを読む。`cmd/yt2column/adr_candidates_docs_test.go::TestADRCorrectionCandidates` が通る |
| AC-21 | `diff` | 下の記録の、変わったファイルの一覧を読む |
| AC-22 | `diff` | 下の記録で、差分が AC-16・AC-17・AC-19・AC-20・AC-23・AC-24 の箇所と、中身を直したガイドの節の書式の直しに限られることを読む |
| AC-23 | `static`・`manual` | `cmd/codexreview/docs_test.go::TestNoDirectCodexInvocation`・`TestDocumentedInvocationsParse`（`local-cli-review.md` の呼び出しが変わらない）、`cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（規約へのリンクの解決）。例外への参照は PR のレビューで読む |
| AC-24 | `diff`・`static`・`manual` | 下の記録で、`deny` の最後に 2 つの規則が足され、ほかの規則が変わらないことを読む。`settings.json` を読む既存のテスト（`cmd/chief/docs_test.go::TestSettingsDeniesPaneInput`・`TestSettingsDeniesWorktreeChanges`・`TestSettingsAsksBeforeRemove`、`cmd/codexreview/docs_test.go::TestSettingsAllowsTool`、`cmd/numreserve/docs_test.go::TestSettingsAllowsTool`）が通る。PR のレビューで、設計書 3.7 の表の各例が当たるか当たらないかを、規則の前方一致の形に照らして読む。引数のない `go install` は、`cmd/chief/docs_test.go` の `matchesRule` のモデル（`Bash(x *)` は引数のない `x` にも当たる。`cmd/chief/docs_test.go:320-342`）に照らして読む。<br>あわせて、人が PR のブランチの Claude Code のセッションで、`/permissions` に 2 つの規則がプロジェクトの設定から読まれていることを確かめる。そのうえで、拒む形（`make install-` と `go install` で始まり、当たっても何も入らない書き方。たとえば存在しないターゲットやパス）を Bash で走らせると、ツールの結果が拒否の規則による拒否になり、拒まない形（たとえば `make -n test`）は拒まれないことを確かめる。エージェントが指示に従って自分で断った場合は、拒否の規則を確かめたことにならない。結果は PR のコメントに書く（1.3） |
| AC-25 | `static` | `make test`（`settings.json` を読む上記のテストを含む） |

**I-NN の扱い:**

| 項目 | 扱い |
|---|---|
| I-01 | 採る。実装モデル要件は `sonnet`（3.2） |
| I-02 | 採る。差分の範囲の AC は AC ごとに記録を持ち、AC-21 の記録はファイルの一覧を持って PR の実装の最後のコミットで取る。AC-20 にもそのパスの記録を持たせる（下の記録、ステップ 1-6） |
| I-03 | 採る。ステップ 1-2 で、訂正行を自分の言葉で書き、`<` と `@` を含めない。AC-11 のスクリプトで確かめる |
| I-04 | 採る。各ステップの完了の条件に、変えるファイルを読むパッケージのテストを入れた |

Script record (AC-11): `_checks/correction_lines.sh`

- Merge-base: `9122d1aaad7244eac37198da4ca30e1da133a504`
- Compared commit: `66431e94b81b40c5a6bec4a412713fad68e6306c`

Output（スクリプトの出力の全体を、行の頭から始まるコードブロックに書く）:

```text
ok: a commit that changes docs/dev/adr/0014-deterministic-checks-in-go.md has an editorial correction line naming it
ok: every editorial correction line has the fixed form
ok: no editorial correction line contains '<' or '@'
commits in 9122d1aaad7244eac37198da4ca30e1da133a504..66431e94b81b40c5a6bec4a412713fad68e6306c: 10
commits that change docs/dev/adr/0014-deterministic-checks-in-go.md: 2
editorial correction lines: 8
```

Diff record (AC-10):

- Compared paths: `docs/dev/adr/0014-deterministic-checks-in-go.md`
- Merge-base: `9122d1aaad7244eac37198da4ca30e1da133a504`
- Compared commit: `66431e94b81b40c5a6bec4a412713fad68e6306c`

Path diff:

```text
diff --git a/docs/dev/adr/0014-deterministic-checks-in-go.md b/docs/dev/adr/0014-deterministic-checks-in-go.md
index 0d18821715691cdaf5ffbd6484dca976268f21bf..51f9089b033c50b0d23f24d2e2e5150044c0db6b 100644
--- a/docs/dev/adr/0014-deterministic-checks-in-go.md
+++ b/docs/dev/adr/0014-deterministic-checks-in-go.md
@@ -14,16 +14,18 @@

 ## 決定

--   **判定と副作用を持つ開発の道具は Go で書き、テストを持つ。** 判定の誤りを、テストで壊して失敗を確かめることで見つけられるようにするためである。今日あるのは `fixprguard`・`codexreview`・`coveragecompare`・`mergepr`・`chief`・`numreserve` の 6 つで、どれも `cmd/<名前>` と `internal/<名前>` の組であり、製品の 3 つのバイナリには入らない。何を決めるかは [ADR-0015](0015-untrusted-review-threads.md)・[ADR-0013](0013-review-setup.md)・[ADR-0037](0037-test-deletion-coverage.md)・[ADR-0046](0046-chief-of-staff.md)・[ADR-0051](0051-number-reservation.md) と [mergepr_guide.md](../guides/mergepr_guide.md) が定め、導入手順は [CLAUDE.md](../../../CLAUDE.md) の「Build Commands」にある。
--   **道具は、判定に使う基準値を状態のファイルから読み返さない。** 基準値と判定した集合は、サブコマンドの出力として返し、セッションが次の呼び出しの引数で渡す。`fixprguard` は利用者のために git のディレクトリへ記録を書くが、それを判定に読み返さない。判定をディスクに置くと、修正のフェーズや build checks が実行するコードに書き換えられるためである。保証の内容は [fixprguard_guide.md](../guides/fixprguard_guide.md) の「7. ツールが保証すること・しないこと」にある。
--   **例外として、chief の道具は、リポジトリの外の状態のディレクトリにある次の記録を、判定に読み返す。自動送信の記録、送信の記録、送信の停止、待ち受けの記録（待ち受けのラウンドごとの記録と、PR が開いているのを見た記録）、通知の記録、PR の詳細の控え、ロック、workspace を作ったときの記録である。記録は、送るコマンドとタスクを決めるためには使わない。送ることについては、送ることを止める条件（リセットの候補を狭める PR の詳細の控えを含む）として使うか、自動送信の候補の workspace を名指して Herdr・git・GitHub で確かめ直す条件として使うだけである。通知の記録は通知の重複を防ぐことと待っている時間による並べ方に、送信の記録は送った入力の履歴と「実行中」の行の表示に使い、どちらも送る中身を左右しない。送ることに関わる記録が読めなければ送らない。workspace を作ったときの記録は、削除の候補の workspace を名指すことと、その行に示すタスクを探す手がかりにだけ使い、名指した workspace は Herdr・git・`origin/main` の文書で確かめ直す。読めなければ削除を提案しない。作ったときの記録は worktree ごとにあり、同じパスに作り直す前と、消し終えた後に、そのパスの分をすべて消す。** 起動し直しても自動送信を 2 回しないこと、送信を止めた workspace を覚えていること、chief を 1 つに限ることを、セッションの記憶では守れないためである（[ADR-0046](0046-chief-of-staff.md)）。範囲と、記録が条件として働く範囲は [0088 設計書](../../tasks/0088_chief_of_staff/02_architecture.md) の 3.5「状態のディレクトリの記録とロック」と [0092 設計書](../../tasks/0092_chief_task_workspace/02_architecture.md) の 3.5「作ったことの記録」にある。
--   **順序を、コマンドの定義ではなく道具の中で強制する。** 確認より前に build checks を動かさないこと、commit と push の間で hooks や config が変わっていたら push しないこと、は道具のコードが守る。コマンドの定義に残るのは、サブコマンドを呼ぶ順序と、利用者への確認だけである。エージェントが手順を読み違えても、範囲外の変更が commit されたり、信頼の境界が迂回されたりしないためである。保証の内容は [fixprguard_guide.md](../guides/fixprguard_guide.md) の「7. ツールが保証すること・しないこと」にある。
--   **拡張の関数の範囲だけは、TypeScript の補助に求める。** `coveragecompare` は Go と拡張の両方の比較を受け持つが、拡張の関数の終わりの行は node の lcov が出さないので、`extension/scripts/function-ranges.ts` が入っている `typescript` の構文木から求める。新しい npm のパッケージは加えない。拡張の関数の終わりの行を、lcov からは求められないためである。規則は [0055 設計書](../../tasks/0055_coverage_compare/02_architecture.md) の 3.5「拡張の測り方」にある。
--   **道具は `main` からインストールして使う。** `mergepr`・`fixprguard`・`codexreview`・`chief`・`numreserve` は `make install-mergepr`・`make install-fixprguard`・`make install-codexreview`・`make install-chief`・`make install-numreserve` で入れ、道具を変えたマージの後は再インストールする。`--protocol` の番号を上げる PR では、その PR の `/runpr` の前に、その PR のブランチで再インストールする（[fixprguard_guide.md](../guides/fixprguard_guide.md) §3）。`coveragecompare` は `make coverage-compare` と `make ext-coverage-compare` が、そのつど一時ディレクトリにビルドして実行する。`fixprguard` は `--protocol` の番号で、呼び出し側の文書と道具の版が合っているかを照合する。あわせて、文書と道具を変える順序は、両方が動き続ける形にする（[fixprguard_guide.md](../guides/fixprguard_guide.md) §9）。インストール済みの古い道具が、古い想定で黙って判定しないようにするためである。
--   **エージェントは、利用者が `make install-<名前>` で `main` から入れる道具を、どれもインストールもビルドもしない。** ビルドするとは、道具を走らせるためのバイナリを作ること（`go build` など）か、ソースから走らせること（`go run` など）で、テストや lint のために道具のパッケージをコンパイルすることは含まない。make のターゲットがそのつどビルドする `coveragecompare` は対象ではない。例外として、道具を開発するか調べるときは、ソースからビルドして走らせてよい。ただし、作ったバイナリは `PATH` の通った場所（`$(go env GOPATH)/bin` を含む）に置かず、手順の中で入れた道具の代わりにも使わず、外に副作用を持つ走らせ方（ネットワークや GitHub への書き込み、記録の tag の作成、pane への入力、外のサービスへの送信）もしない。プロトコル番号を上げる PR のブランチで道具を入れ直すのも利用者である。エージェントは、道具を入れるかビルドする手順に来たとき、または道具が見つからないか番号が合わないときは、そこで止まって、最新の `main` で入れるように利用者に頼む。PR のブランチで入れ直すように頼むのは、その PR がプロトコル番号を上げる場合だけである。手順が別の扱いを定めている場合（`codexreview` が見つからないときなど）は、その手順に従う。Claude Code では、`.claude/settings.json` に `make install-*` と `go install` を拒む規則を置く（タスク 0112 の実装で足す）。エージェントが PR のブランチから `fixprguard` や `mergepr` を入れるか走らせると、そのブランチが変えた判定で `/fixpr`・`/runpr`・`/mergepr` が動くことになり、`main` の判定を迂回できるためである。利用者が PR のブランチで入れ直したあと、その道具がブランチの判定で動くことは受け入れる。利用者がその PR の変更を見たうえで入れ直すためである。拒否の規則は、auto モードで確認なしに入れることを防ぐ層である（[ADR-0046](0046-chief-of-staff.md) と同じ理由）。規則は、実装の PR で開発の道具の規約に移すまで、[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md) の「要件定義の際に決めたこと（2026-11-19）」にある。
--   **`--protocol` の番号（プロトコル番号）を上げる PR は、番号を実装の時点の `origin/main` の値にちょうど 1 を足した値にし、マージの時点でも `origin/main` の値 + 1 であることを確かめる。食い違えばマージしない。タスク文書には上げた後の番号を書かず、「`main` の値 + 1」と書く。確かめは、`/runpr` からのマージにも `/mergepr` を単独で使うマージにも当てはめ、マージの直前にも確かめ直す。`mergepr` も番号を持ち、`/mergepr` の定義と版を照合する。** 並行する 2 つのブランチが同じ番号に上げると、git は定数の行の同じ変更を衝突なしにまとめ、文書のガードテストも通る。その結果、違う約束が 1 つの番号を共有したまま `main` に入り、古い文書と新しい道具が黙って違う規則で動きうるためである。対象は、道具とそれを呼ぶ文書の版を照合する番号で、今は `fixprguard`・`codexreview`・`chief` と、確かめを入れる `mergepr` の 4 つである。`mergepr` に番号を持たせるのは、確かめを入れる前の `mergepr` や古い定義でマージして、確かめが黙って抜けることを防ぐためである。確かめは `mergepr` の `prepare` と `merge` が行い、`merge` は GitHub にマージを依頼する直前に、`origin/main` を取り直して確かめ直す。`/runpr` と単独の `/mergepr` のどちらのマージも、この 2 つのサブコマンドを必ず通るためである。番号を持つ道具は、`internal/<名前>/` の直下のファイルに、`const Protocol = <整数>` の 1 行で番号を置く。確かめがこの形の行だけを番号として読むためである。この形でない宣言は、確かめが読めない値として止まるか、確かめの対象の道具と文書のガードの一覧を突き合わせるテストが CI で止める。確かめの仕様は [mergepr_guide.md](../guides/mergepr_guide.md) にある。受け入れた危険は 4 つある。(1) マージの前は、同じ道具の番号を上げる並行するブランチが同じ番号を共有し、片方のブランチから入れた道具がもう片方の文書を受け付けうる。ブランチごとに一意の番号を配る仕組みは、issue #933 の範囲を超えるためである。(2) マージの直前の確かめ直しと GitHub のマージの間の数秒に、ほかの PR が同じ番号でマージされうる。(3) 番号を上げたブランチを、同じ番号の入った `main` の上へ rebase するか履歴をまとめ直すと、確かめが衝突を見落としうる。(4) 一度消した道具を同じ名前で足し直すと、番号が `1` に戻る。規則の正本は [CLAUDE.md](../../../CLAUDE.md) の「Development Notes」の「Protocol numbers」にあり、確かめの要件は [0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md) の F-003 にある。
--   **道具の引数・出力・終了コードは、コマンドの定義と文書のガードが読む契約にする。** 変えたときは、道具と、それを呼ぶ文書と、ガードを同じ PR で変える。契約が食い違うと、レビューの信頼の境界が実行時に働かなくなるためである。契約は、[CLAUDE.md](../../../CLAUDE.md) の「Build Commands」「Test Commands」「Development Notes」、[.claude/commands/_context.md](../../../.claude/commands/_context.md) の「Guard tool」の行、[fixprguard_guide.md](../guides/fixprguard_guide.md)、[mergepr_guide.md](../guides/mergepr_guide.md)、[chief_guide.md](../guides/chief_guide.md)、[numreserve_guide.md](../guides/numreserve_guide.md) にある。
--   **例外として、記録の行を読んで判定するだけで副作用を持たない判定は、コマンドの定義から独立したシェルスクリプトにしてよい。スクリプトは `.claude/commands/_lib/` に置く。呼ぶ手順は、このスクリプトを `origin/main` の commit から作業ツリーの外に取り出して走らせる。失敗は安全な側の判定とし、規則を壊して失敗することを Go のテストで確かめる。** 今あるのは、Opus のレビューの判断のスクリプト `_lib/opus-review-decision.sh`（[ADR-0042](0042-implementation-model-tiers.md)）と、ローカルの CLI レビューが見たかと Fable の記録が有効かを決める `_lib/review-records.sh`（[ADR-0013](0013-review-setup.md)、[ADR-0050](0050-fable-partial-review.md)）である。`opus-review-decision.sh` は、この例外より前に作られ、常設の Go テストが確かめるのは計画書の値の読み方だけで、判断の規則はタスク 0081 の一度きりの確かめで確かめた。新しいスクリプトは、規則の全体を常設の Go テストで確かめる。判定は記録の行と `git rev-list` の組み合わせで足り、Go の道具にすると、新しい `cmd/` と `internal/` を足し、利用者にも入れ直しを求めることになるためである。採らなかった案の「判定をコマンドの定義に書いたシェルの手順」では、エージェントが文を読んで判定する。この例外では、テストを持つ 1 つのファイルが判定する点が違う（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md) 1.2 の決定 15）。
+開発の道具について、規則の要約とその理由を決めている。規則の本文と手順は [dev_tools.md](../conventions/dev_tools.md) が持ち、規則 1〜6 の項目は規約の節をその見出しの名前で指す。
+
+-   **Go の道具とテスト:** 判定と副作用を持つ開発の道具は、Go で書き、テストを持つ。判定の誤りを、テストで壊して失敗を確かめることで見つけられるようにするためである。何を決めるかは [ADR-0015](0015-untrusted-review-threads.md)・[ADR-0013](0013-review-setup.md)・[ADR-0037](0037-test-deletion-coverage.md)・[ADR-0046](0046-chief-of-staff.md)・[ADR-0051](0051-number-reservation.md) と [mergepr_guide.md](../guides/mergepr_guide.md) が定める。規則は [dev_tools.md](../conventions/dev_tools.md) の「Rule 1: Write a tool in Go, with tests」と「Kinds of tools」にある。
+-   **基準値を状態のファイルから読み返さないこと:** 道具は、判定に使う基準値を状態のファイルから読み返さない。基準値と判定した集合は、サブコマンドの出力として返し、セッションが次の呼び出しの引数で渡す。判定をディスクに置くと、修正のフェーズや build checks が実行するコードに書き換えられるためである。`fixprguard` は利用者のために git のディレクトリへ記録を書くが、それを判定に読み返さない。例外は次の項目にある。規則は [dev_tools.md](../conventions/dev_tools.md) の「Rule 2: Do not read a decision's baseline back from a state file」に、保証の内容は [fixprguard_guide.md](../guides/fixprguard_guide.md) の「7. ツールが保証すること・しないこと」にある。
+-   **chief の例外:** chief の道具は、リポジトリの外の状態のディレクトリにある次の記録を、判定に読み返す。自動送信の記録、送信の記録、送信の停止、待ち受けの記録（待ち受けのラウンドごとの記録と、PR が開いているのを見た記録）、通知の記録、PR の詳細の控え、ロック、workspace を作ったときの記録である。記録は、送るコマンドとタスクを決めるためには使わない。送ることについては、送ることを止める条件（リセットの候補を狭める PR の詳細の控えを含む）として使うか、自動送信の候補の workspace を名指して Herdr・git・GitHub で確かめ直す条件として使うだけである。通知の記録は通知の重複を防ぐことと待っている時間による並べ方に、送信の記録は送った入力の履歴と「実行中」の行の表示に使い、どちらも送る中身を左右しない。送ることに関わる記録が読めなければ送らない。workspace を作ったときの記録は、削除の候補の workspace を名指すことと、その行に示すタスクを探す手がかりにだけ使い、名指した workspace は Herdr・git・`origin/main` の文書で確かめ直す。読めなければ削除を提案しない。作ったときの記録は worktree ごとにあり、同じパスに作り直す前と、消し終えた後に、そのパスの分をすべて消す。起動し直しても自動送信を 2 回しないこと、送信を止めた workspace を覚えていること、chief を 1 つに限ることを、セッションの記憶では守れないためである（[ADR-0046](0046-chief-of-staff.md)）。範囲と、記録が条件として働く範囲は [0088 設計書](../../tasks/0088_chief_of_staff/02_architecture.md) の 3.5「状態のディレクトリの記録とロック」と [0092 設計書](../../tasks/0092_chief_task_workspace/02_architecture.md) の 3.5「作ったことの記録」にある。
+-   **順序の道具のコードでの強制:** 処理の順序は、コマンドの定義ではなく道具のコードで強制する。コマンドの定義に残るのは、サブコマンドを呼ぶ順序と、利用者への確認だけである。エージェントが手順を読み違えても、範囲外の変更が commit されたり、信頼の境界が迂回されたりしないためである。例は [fixprguard_guide.md](../guides/fixprguard_guide.md) の「5. サブコマンド」の `finish` と `push` の説明と、「7. ツールが保証すること・しないこと」にある。規則は [dev_tools.md](../conventions/dev_tools.md) の「Rule 3: Enforce the order in the tool's code」にある。
+-   **拡張の関数の範囲を求める TypeScript の補助:** 拡張の関数の範囲だけは、TypeScript の補助に求める。`coveragecompare` は Go と拡張の両方の比較を受け持つが、拡張の関数の終わりの行は node の lcov が出さないので、`extension/scripts/function-ranges.ts` が入っている `typescript` の構文木から求める。新しい npm のパッケージは加えない。拡張の関数の終わりの行を、lcov からは求められないためである。規則は [0055 設計書](../../tasks/0055_coverage_compare/02_architecture.md) の 3.5「拡張の測り方」にある。
+-   **`main` からのインストール:** 道具は `main` からインストールして使う。道具を変えたマージの後は入れ直す。`--protocol` の番号を上げる PR では、その PR の `/runpr` の前に、その PR のブランチで入れ直す（[fixprguard_guide.md](../guides/fixprguard_guide.md) の「3. インストール」）。インストール済みの古い道具が、古い想定で黙って判定しないようにするためである。規則は [dev_tools.md](../conventions/dev_tools.md) の「Rule 5: Install tools from main; agents never install or build them」に、文書と道具を変える順序は同じ規約の「Rule 4: Arguments, output, and exit codes are a contract」にある。
+-   **エージェントがインストールもビルドもしないこと:** エージェントは、利用者が `main` から入れる道具を、どれもインストールもビルドもしない。道具を開発するか調べるときは、ソースからビルドして走らせてよいが、外に副作用を持たない走らせ方に限る。エージェントが PR のブランチから `fixprguard` や `mergepr` を入れるか走らせると、そのブランチが変えた判定で `/fixpr`・`/runpr`・`/mergepr` が動くことになり、`main` の判定を迂回できるためである。利用者が PR のブランチで入れ直したあと、その道具がブランチの判定で動くことは受け入れる。利用者がその PR の変更を見たうえで入れ直すためである。Claude Code では、`.claude/settings.json` に拒否の規則を置く。拒否の規則は、auto モードで確認なしに入れることを防ぐ層である（[ADR-0046](0046-chief-of-staff.md) と同じ理由）。規則は [dev_tools.md](../conventions/dev_tools.md) の「Rule 5: Install tools from main; agents never install or build them」に、拒否の規則の形は同じ節の The permission layer にある。
+-   **プロトコル番号:** `--protocol` の番号（プロトコル番号）を上げる PR は、番号を実装の時点の `origin/main` の値にちょうど 1 を足した値にし、マージの時点でも `origin/main` の値 + 1 であることを確かめる。食い違えばマージしない。タスク文書には上げた後の番号を書かず、「`main` の値 + 1」と書く。確かめは、`/runpr` からのマージにも `/mergepr` を単独で使うマージにも当てはめ、マージの直前にも確かめ直す。`mergepr` も番号を持ち、`/mergepr` の定義と版を照合する。並行する 2 つのブランチが同じ番号に上げると、git は定数の行の同じ変更を衝突なしにまとめ、文書のガードテストも通る。その結果、違う約束が 1 つの番号を共有したまま `main` に入り、古い文書と新しい道具が黙って違う規則で動きうるためである。対象は、道具とそれを呼ぶ文書の版を照合する番号で、今は `fixprguard`・`codexreview`・`chief` と、確かめを入れる `mergepr` の 4 つである。`mergepr` に番号を持たせるのは、確かめを入れる前の `mergepr` や古い定義でマージして、確かめが黙って抜けることを防ぐためである。確かめは `mergepr` の `prepare` と `merge` が行い、`merge` は GitHub にマージを依頼する直前に、`origin/main` を取り直して確かめ直す。`/runpr` と単独の `/mergepr` のどちらのマージも、この 2 つのサブコマンドを必ず通るためである。番号を持つ道具は、`internal/<名前>/` の直下のファイルに、`const Protocol = <整数>` の 1 行で番号を置く。確かめがこの形の行だけを番号として読むためである。この形でない宣言は、確かめが読めない値として止まるか、確かめの対象の道具と文書のガードの一覧を突き合わせるテストが CI で止める。確かめの仕様は [mergepr_guide.md](../guides/mergepr_guide.md) にある。受け入れた危険は 4 つある。(1) マージの前は、同じ道具の番号を上げる並行するブランチが同じ番号を共有し、片方のブランチから入れた道具がもう片方の文書を受け付けうる。ブランチごとに一意の番号を配る仕組みは、issue #933 の範囲を超えるためである。(2) マージの直前の確かめ直しと GitHub のマージの間の数秒に、ほかの PR が同じ番号でマージされうる。(3) 番号を上げたブランチを、同じ番号の入った `main` の上へ rebase するか履歴をまとめ直すと、確かめが衝突を見落としうる。(4) 一度消した道具を同じ名前で足し直すと、番号が `1` に戻る。規則の正本は [CLAUDE.md](../../../CLAUDE.md) の「Development Notes」の「Protocol numbers」にあり、確かめの要件は [0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md) の F-003 にある。
+-   **引数・出力・終了コードの契約:** 道具の引数・出力・終了コードは、コマンドの定義と文書のガードが読む契約にする。変えたときは、道具と、それを呼ぶ文書と、ガードを同じ PR で変える。契約が食い違うと、レビューの信頼の境界が実行時に働かなくなるためである。規則と、契約が書かれている場所の一覧は [dev_tools.md](../conventions/dev_tools.md) の「Rule 4: Arguments, output, and exit codes are a contract」にある。
+-   **副作用のない判定のシェルスクリプト:** 例外として、記録の行を読んで判定するだけで副作用を持たない判定は、コマンドの定義から独立したシェルスクリプトにしてよい。判定は記録の行と `git rev-list` の組み合わせで足り、Go の道具にすると、新しい `cmd/` と `internal/` を足し、利用者にも入れ直しを求めることになるためである。今あるのは、Opus のレビューの判断のスクリプト `_lib/opus-review-decision.sh`（[ADR-0042](0042-implementation-model-tiers.md)）と、ローカルの CLI レビューが見たかと Fable の記録が有効かを決める `_lib/review-records.sh`（[ADR-0013](0013-review-setup.md)、[ADR-0050](0050-fable-partial-review.md)）である。`opus-review-decision.sh` は、この例外より前に作られ、常設の Go テストが確かめるのは計画書の値の読み方だけで、判断の規則はタスク 0081 の一度きりの確かめで確かめた。採らなかった案の「判定をコマンドの定義に書いたシェルの手順」では、エージェントが文を読んで判定する。この例外では、テストを持つ 1 つのファイルが判定する点が違う（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md) 1.2 の決定 15）。4 つの条件と、この例外より後に足すスクリプトの扱いは [dev_tools.md](../conventions/dev_tools.md) の「Rule 6: Exception: a side-effect-free record check may be a shell script」にある。

 ## 採らなかった案

```

Diff record (AC-18):

- Compared paths: `CLAUDE.md`
- Merge-base: `9122d1aaad7244eac37198da4ca30e1da133a504`
- Compared commit: `ff31c6465eeda9de15ad1cc3e9f0d21d5036db3b`

Path diff:

```text
diff --git a/CLAUDE.md b/CLAUDE.md
index 0a8c98ac24a7a49059d6b7c74838e258cb9fce3f..1f2d894e579189fdeaf45b287e461cbb468108ba 100644
--- a/CLAUDE.md
+++ b/CLAUDE.md
@@ -23,6 +23,7 @@ This file provides guidance to Claude Code (claude.ai/code) when working with co
 - [fixprguard Guide](docs/dev/guides/fixprguard_guide.md) - Setup, subcommands, guarantees, scope rules, and recovery for the `fixprguard` guard tool for `/fixpr` and `/runpr`
 - [chief Guide](docs/dev/guides/chief_guide.md) - Setup, usage, subcommands, guarantees, the settings' herdr denials, the state directory, and known limits of the `chief` tool that `/chief` drives
 - [numreserve Guide](docs/dev/guides/numreserve_guide.md) - Installation, invocation, stop reasons and what to do next, the record ruleset and removing a wrong record tag, and the cloud-session limits of the `numreserve` tool that decides task and ADR numbers
+- [Developer Tools](docs/dev/conventions/dev_tools.md) - Rules for adding or changing a developer tool: write it in Go with tests, install installed tools from `main` (the user installs them; agents never install or build them), and keep its arguments, output, and exit codes as a contract

 ## Documents

@@ -873,8 +874,8 @@ stops the merge check as unreadable (the form is in
   otherwise. How to fix a stop is in
   [mergepr_guide.md](docs/dev/guides/mergepr_guide.md) chapter 7: merge
   `origin/main` into the branch, set the number and every `--protocol` the
-  documents write to the expected value, commit and push, reinstall the tool
-  from the branch, and merge again; when the head number equals
+  documents write to the expected value, commit and push, ask the user to
+  reinstall the tool from the branch, and merge again; when the head number equals
   `origin/main`'s, rebuilding the history on `origin/main` is a fix too, but
   only when the PR's own changes do not change the tool's contract.
 - **Accepted risks.** (1) Before the merge, parallel branches that raise the
@@ -1086,7 +1087,9 @@ section of the Requirements Process Guide.
   Codex CLI (`codex exec review`), which sends the diff and whatever Codex reads
   during the review to OpenAI's service and spends the Codex allowance the PR
   review shares. Codex CLI itself is never run directly, and the agents never
-  install or build `codexreview`: the user installs it from `main`
+  install or build `codexreview`: the user installs it from `main`. The
+  development exception is in
+  [Developer Tools](docs/dev/conventions/dev_tools.md), Rule 5.
 - `numreserve task` and `numreserve adr` (the developer tool that decides a task
   or an ADR number) may be executed without explicit approval, written exactly
   in one of these two forms and run only when a procedure needs a new number
@@ -1095,7 +1098,21 @@ section of the Requirements Process Guide.
   writes to the network: it creates one record tag on `origin`
   (`refs/tags/task-number/NNNN` or `refs/tags/adr-number/NNNN`), which cannot
   be deleted. The agents never install or build the tool: the user installs
-  it from `main` (`make install-numreserve`)
+  it from `main` (`make install-numreserve`). The development exception is in
+  [Developer Tools](docs/dev/conventions/dev_tools.md), Rule 5
+- The agents never install or build an installed developer tool. "Build" means
+  making a binary that runs the tool or running it from source
+  (`make install-<name>`, `go install`, `go build ./cmd/<name>`,
+  `go run ./cmd/<name>`); compiling a tool's package for tests or lint is not
+  building. When a step comes to installing or building an installed tool, or
+  a command fails because a tool is missing or its protocol number does not
+  match, stop there, ask the user, and show the command to run. Show it for an
+  up-to-date `main` checkout; ask the user to reinstall from the PR's branch
+  only when that PR raises the tool's protocol number. A procedure that
+  decides otherwise wins (for `codexreview`, step 3 of
+  `.claude/commands/_lib/local-cli-review.md`). The development exception and
+  the details are in [Developer Tools](docs/dev/conventions/dev_tools.md),
+  Rule 5
 - Creating, changing, or deleting a record tag (a ref under
   `refs/tags/task-number/` or `refs/tags/adr-number/`) other than through the
   two calls above, by `git push`, `gh api`, or any other means, requires the
@@ -1111,4 +1128,6 @@ section of the Requirements Process Guide.
   with the tool's one `gh api graphql` query of `issueOrPullRequest` (its
   number, state, title, and labels). It approves no write (push, merge,
   comment, or issue operation); the tool runs none. The tool is called only in the form `.claude/commands/chief.md` gives,
-  and the agents never install or build it: the user installs it from `main`
+  and the agents never install or build it: the user installs it from `main`.
+  The development exception is in
+  [Developer Tools](docs/dev/conventions/dev_tools.md), Rule 5
```

Diff record (AC-20):

- Compared paths: `.claude/commands/_lib/adr-correction-candidates.sh`
- Merge-base: `9122d1aaad7244eac37198da4ca30e1da133a504`
- Compared commit: `ff31c6465eeda9de15ad1cc3e9f0d21d5036db3b`

Path diff:

```text
diff --git a/.claude/commands/_lib/adr-correction-candidates.sh b/.claude/commands/_lib/adr-correction-candidates.sh
index 277da715d1f51b7af69c4b69aa11d260b6683cc3..ff22e12a544600d127b8dd4c05d413cb012524ed 100644
--- a/.claude/commands/_lib/adr-correction-candidates.sh
+++ b/.claude/commands/_lib/adr-correction-candidates.sh
@@ -61,7 +61,7 @@
 #
 # Why bash: the script only reads git and prints, with no side effect, so it
 # is not a development tool with decisions and side effects that
-# docs/dev/adr/0014-deterministic-checks-in-go.md puts in Go; the comparison
+# docs/dev/conventions/dev_tools.md puts in Go; the comparison
 # is still made by the script, not by the agent, and TestADRCorrectionCandidates
 # (cmd/yt2column/adr_candidates_docs_test.go) checks each property above.

```

Diff record (AC-21):

比べるパスに用語集を選ぶのは、AC-21 のうち条件付きのファイルが用語集だけで、ほかの記録がその差分を持たないためである。

- Compared paths: `docs/translation_glossary.md`
- Merge-base: `9122d1aaad7244eac37198da4ca30e1da133a504`
- Compared commit: `0bd9a6a72eee0d068edca375335f23fb4c4fb6f0`

Path diff:

```text
diff --git a/docs/translation_glossary.md b/docs/translation_glossary.md
index 49aa874fc46a259cbc60e494d13b501eaa8fe177..fc340514ce9edb93fed60dc369e2793597187c26 100644
--- a/docs/translation_glossary.md
+++ b/docs/translation_glossary.md
@@ -132,6 +132,9 @@
 | 指示ファイル | instruction file | Claude Code や自動レビュアーが、置かれたディレクトリへの指示として自動で読むファイル（`CLAUDE.md`・`AGENTS.md` など） |
 | Go のビルドの入力 | Go build input | Go がパッケージのビルドで、`.go` のファイルのほかに扱うファイル（`.c`・`.s`・`.syso` など） |
 | ガードツール | guard tool | `/fixpr` と `/runpr` が判定と書き込みを任せる開発用ツール `fixprguard` |
+| 開発の道具 | developer tool | `cmd/<名前>` と `internal/<名前>` の組で作る、開発の手順のための Go の道具。製品のバイナリには入らない（`docs/dev/conventions/dev_tools.md`） |
+| 入れる道具 | installed tool | 開発の道具のうち、利用者が `make install-<名前>` で `main` から入れるもの |
+| そのつどビルドする道具 | per-run tool | 開発の道具のうち、make のターゲットが走らせるたびに一時ディレクトリにビルドするもの |
 | 区切りのブロック | fenced block | 信頼できないテキストを、実行ごとの乱数（nonce）を含む `BEGIN-UNTRUSTED-<nonce>` と `END-UNTRUSTED-<nonce>` の行で囲んだもの |
 | 指紋 | fingerprint | スレッドの ID と、コメントの ID・著者・時刻、コメントの数、続きのページの有無から作る識別子。コメントの追加・編集・削除で変わる |
 | 基準値 | baseline | `fixprguard begin` が出力する git の状態の識別子の組。セッションが会話に持ち、後のサブコマンドに渡す |
```

File list:

```text
.claude/commands/_lib/adr-correction-candidates.sh
.claude/commands/_lib/local-cli-review.md
.claude/commands/mergepr.md
.claude/settings.json
CLAUDE.md
docs/dev/adr/0014-deterministic-checks-in-go.md
docs/dev/conventions/dev_tools.md
docs/dev/guides/chief_guide.md
docs/dev/guides/fixprguard_guide.md
docs/dev/guides/mergepr_guide.md
docs/dev/guides/numreserve_guide.md
docs/tasks/0112_dev_tools_convention/03_implementation_plan.md
docs/tasks/0112_dev_tools_convention/_checks/correction_lines.sh
docs/translation_glossary.md
```

Diff record (AC-22):

- Compared paths: `.claude/commands/mergepr.md`, `.claude/commands/_lib/adr-correction-candidates.sh`, `.claude/commands/_lib/local-cli-review.md`, `.claude/settings.json`, `docs/dev/guides/chief_guide.md`, `docs/dev/guides/numreserve_guide.md`, `docs/dev/guides/mergepr_guide.md`, `docs/dev/guides/fixprguard_guide.md`
- Merge-base: `9122d1aaad7244eac37198da4ca30e1da133a504`
- Compared commit: `66431e94b81b40c5a6bec4a412713fad68e6306c`

Path diff:

````text
diff --git a/.claude/commands/_lib/adr-correction-candidates.sh b/.claude/commands/_lib/adr-correction-candidates.sh
index 277da715d1f51b7af69c4b69aa11d260b6683cc3..ff22e12a544600d127b8dd4c05d413cb012524ed 100644
--- a/.claude/commands/_lib/adr-correction-candidates.sh
+++ b/.claude/commands/_lib/adr-correction-candidates.sh
@@ -61,7 +61,7 @@
 #
 # Why bash: the script only reads git and prints, with no side effect, so it
 # is not a development tool with decisions and side effects that
-# docs/dev/adr/0014-deterministic-checks-in-go.md puts in Go; the comparison
+# docs/dev/conventions/dev_tools.md puts in Go; the comparison
 # is still made by the script, not by the agent, and TestADRCorrectionCandidates
 # (cmd/yt2column/adr_candidates_docs_test.go) checks each property above.

diff --git a/.claude/commands/_lib/local-cli-review.md b/.claude/commands/_lib/local-cli-review.md
index ee43ea8ac5b805cd42725ed2b9b245b4e4a9f190..65a0e7cb4fcda44ddb2dedb28e489205e5f5d5a4 100644
--- a/.claude/commands/_lib/local-cli-review.md
+++ b/.claude/commands/_lib/local-cli-review.md
@@ -90,7 +90,9 @@ asking the user.

 Never run Codex CLI directly or by any other path, and never install or build
 the tool (`make install-codexreview`, `go build`): the user installs it from
-`main`, and the permission settings allow it under its own name only.
+`main`, and the permission settings allow it under its own name only. The
+development exception is in `docs/dev/conventions/dev_tools.md`, Rule 5; even
+under it, a run may not stand in for the installed tool inside this procedure.

 ## Terms

diff --git a/.claude/commands/mergepr.md b/.claude/commands/mergepr.md
index 603faee9a80582c05c70d0978c90ea7057f99cb0..29652b69b23f67c1a36ff46a0cddff4a6484131d 100644
--- a/.claude/commands/mergepr.md
+++ b/.claude/commands/mergepr.md
@@ -15,8 +15,10 @@ Invoking this command authorizes the network operations `mergepr` performs
 (`git fetch`, `gh` queries) but **not the merge itself**: step 3 asks the user to
 approve the commit message first (CLAUDE.md, "Tool Execution Safety").

-The mechanics live in the `mergepr` binary (package `internal/mergepr`). Install
-it with `make install-mergepr` from an up-to-date `main` checkout. Every call
+The mechanics live in the `mergepr` binary (package `internal/mergepr`). The
+user installs it from an up-to-date `main` checkout with `make install-mergepr`;
+the agent never installs or builds it (see `docs/dev/conventions/dev_tools.md`,
+Rule 5). Every call
 below passes, with `--protocol`, the protocol number of the `mergepr` these
 steps are written for. When `mergepr` refuses it with `--protocol is missing or does
 not match`, report the error and the two fixes it names (reinstall `mergepr`
diff --git a/.claude/settings.json b/.claude/settings.json
index bd32ef277d72d54bc52d37723a6aabb5a2afcdd6..7ec7a17a6a592a9ca19000315c7a0cbd144c387f 100644
--- a/.claude/settings.json
+++ b/.claude/settings.json
@@ -43,7 +43,9 @@
       "Bash(herdr worktree open*)",
       "Bash(git worktree add*)",
       "Bash(git worktree remove*)",
-      "Bash(git worktree move*)"
+      "Bash(git worktree move*)",
+      "Bash(make install-*)",
+      "Bash(go install *)"
     ]
   }
 }
diff --git a/docs/dev/guides/chief_guide.md b/docs/dev/guides/chief_guide.md
index b648d84fc8eea5e60e1b7bff3b9aa19c3589afc9..325ec135ddfd4bee9aa73e2d62fccca067704dce 100644
--- a/docs/dev/guides/chief_guide.md
+++ b/docs/dev/guides/chief_guide.md
@@ -15,19 +15,21 @@ chief は、Herdr の workspace で並行して進むタスクを見渡し、人

 ## 2. インストール

+`chief` は `main` から入れ、道具を変えたマージの後と、プロトコル番号を上げる PR の前に入れ直す。
+
 ```bash
 make install-chief
 ```

-`go install ./cmd/chief` を実行する。道具は `main` から入れ、道具を変えた PR がマージされたら、`main` を更新してから入れ直す（ADR-0014 の「道具は `main` からインストールして使う」）。インストール済みのバイナリが作られたコミットは、`go version -m "$(command -v chief)"` の `vcs.revision` で確かめられる。
+`go install ./cmd/chief` を実行する。道具は `main` から入れ、道具を変えた PR がマージされたら、`main` を更新してから入れ直す（[dev_tools.md](../conventions/dev_tools.md) の「Rule 5: Install tools from main; agents never install or build them」）。インストール済みのバイナリが作られたコミットは、`go version -m "$(command -v chief)"` の `vcs.revision` で確かめられる。

-`/chief` の定義と道具は `--protocol` の番号で版を照合する。今の番号は次の表のとおりである。
+**プロトコル番号.** `/chief` の定義と道具は `--protocol` の番号で版を照合する。今の番号は次の表のとおりである。

 | 項目 | 値 |
 |---|---|
 | プロトコル番号 | `5` |

-番号が合わないと、道具は何もせずに `protocol_mismatch` で拒む。番号を上げる PR では、その PR の `/runpr` の前に、その PR のブランチで入れ直す。
+番号が合わないと、道具は何もせずに `protocol_mismatch` で拒む。番号を上げる PR では、その PR の `/runpr` の前に、利用者がその PR のブランチで入れ直す。

 **入れ直す順。** 動いている `/chief` を止めてから `make install-chief` を実行し、`/chief` を起動し直す。番号を上げた道具を `/chief` を止めずに入れると、動いている `/chief` は古い定義の番号のまま次のラウンドを走らせ、新しい道具に `protocol_mismatch` で拒まれる。定義はこのとき `make install-chief` での入れ直しを促して止まるが、道具はすでに新しいので、入れ直しても直らない。手当ては、`/chief` を起動し直して新しい定義を読ませることである。

diff --git a/docs/dev/guides/fixprguard_guide.md b/docs/dev/guides/fixprguard_guide.md
index c8e08c17313f0cfddeec5bf046a2b8e94d36489b..e97814a142725449b9cd84076494f204e935c3df 100644
--- a/docs/dev/guides/fixprguard_guide.md
+++ b/docs/dev/guides/fixprguard_guide.md
@@ -35,7 +35,9 @@

 ## 3. インストール

-最新の `main` をチェックアウトした状態で、次を実行する。
+`fixprguard` は `main` から入れ、プロトコル番号を上げる変更では、決まった順で入れ直す。
+
+**入れる手順.** 最新の `main` をチェックアウトした状態で、次を実行する。

 ```sh
 make install-fixprguard
@@ -49,9 +51,9 @@ make install-fixprguard
 go version -m "$(command -v fixprguard)"
 ```

-1 つのバイナリが、同じ利用者のすべての worktree で共有される。一方、`/fixpr`・`/runpr` は、その worktree の checkout にあるコマンドの文書を読み、そこに書かれた `--protocol` を渡す。そのため、`protocol` の番号を上げる変更（例: タスク 0035 で `1` から `2`）では、次の手順に従う。
+**番号を上げる変更での入れ直し.** 1 つのバイナリが、同じ利用者のすべての worktree で共有される。一方、`/fixpr`・`/runpr` は、その worktree の checkout にあるコマンドの文書を読み、そこに書かれた `--protocol` を渡す。そのため、`protocol` の番号を上げる変更（例: タスク 0035 で `1` から `2`）では、次の手順に従う。

-- その PR の `/runpr` の前に、その PR のブランチで再インストールする。その PR のコマンドの文書が新しい番号を渡すためである。マージまでの間、`main` を取り込んでいないほかの worktree の `/fixpr`・`/runpr` は、終了コード `2` で止まる。
+- その PR の `/runpr` の前に、利用者がその PR のブランチで再インストールする。その PR のコマンドの文書が新しい番号を渡すためである。マージまでの間、`main` を取り込んでいないほかの worktree の `/fixpr`・`/runpr` は、終了コード `2` で止まる。
 - マージの直後に、マージの後の `main` で再インストールする。再インストールの前に新しい番号の文書から実行すると、終了コード `2` で止まる。
 - 作業中のほかのブランチには、次の `/fixpr`・`/runpr` の前に `main` を取り込む。取り込む前は、そのブランチの文書が古い番号を渡すので、終了コード `2` で止まる。

diff --git a/docs/dev/guides/mergepr_guide.md b/docs/dev/guides/mergepr_guide.md
index 7180ac59870abeadaf5b49a04ada5862aa1f8a47..342eefb81dcd28a3b2dcd55160675aa44c95e172 100644
--- a/docs/dev/guides/mergepr_guide.md
+++ b/docs/dev/guides/mergepr_guide.md
@@ -37,7 +37,9 @@ gh api repos/isseis/yt2column --jq .delete_branch_on_merge

 ## 3. インストール

-最新の `main` をチェックアウトした状態で、次を実行する。
+`mergepr` は `main` から入れ、道具を変えたマージの後に入れ直す。プロトコル番号を上げる PR では、その PR のブランチでも入れ直す。
+
+**入れる手順.** 最新の `main` をチェックアウトした状態で、次を実行する。

 ```sh
 make install-mergepr
@@ -45,9 +47,13 @@ make install-mergepr

 `$(go env GOPATH)/bin` に `PATH` が通っていることを確認する。ツール自体を変更した PR がマージされたら、`main` を更新してから再インストールする。

-`mergepr` は、すべてのサブコマンドで `--protocol` を必須とし、自分のプロトコル番号（`internal/mergepr` の `Protocol`。現在の番号は `3`）と文字列として照合する。`/mergepr`・`/runpr` の定義は、`mergepr` を呼ぶたびに `--protocol 3` を渡す。番号が違うか渡されないとき、`mergepr` は何も読まず、何も書かずに止まる（7 章）。`mergepr` の `Protocol` を上げる PR では、その PR の `/runpr` が最初に `mergepr` を呼ぶ前に、その PR のブランチで再インストールし、マージの直後に `main` から再インストールする。その間、`main` を取り込んでいないほかの worktree の `/mergepr`・`/runpr` は番号の不一致で止まる。途中でほかの PR をマージする必要が生じたら、`main` から再インストールすれば、ほかの worktree は元どおり進める。入れ替える前に、どの worktree にも準備したままの作業ディレクトリ（`mergepr-*`）と終わっていない `cleanup` がないことを確かめる。新しい `mergepr` は、古い定義が書いた `--protocol` のない `discard`・`cleanup` を拒むので、残っていたら新しい `mergepr` で `mergepr discard --protocol 3 --state <state-file>`（片付けなら `mergepr cleanup --protocol 3 --state <state-file>`）を手で実行する。
+**プロトコル番号の照合.** `mergepr` は、すべてのサブコマンドで `--protocol` を必須とし、自分のプロトコル番号（`internal/mergepr` の `Protocol`。現在の番号は `3`）と文字列として照合する。`/mergepr`・`/runpr` の定義は、`mergepr` を呼ぶたびに `--protocol 3` を渡す。番号が違うか渡されないとき、`mergepr` は何も読まず、何も書かずに止まる（7 章）。
+
+**番号を上げる PR での入れ直し.** `mergepr` の `Protocol` を上げる PR では、その PR の `/runpr` が最初に `mergepr` を呼ぶ前に、利用者がその PR のブランチで再インストールし、マージの直後に `main` から再インストールする。その間、`main` を取り込んでいないほかの worktree の `/mergepr`・`/runpr` は番号の不一致で止まる。途中でほかの PR をマージする必要が生じたら、`main` から再インストールすれば、ほかの worktree は元どおり進める。
+
+**入れ替える前の確かめ.** どの worktree にも準備したままの作業ディレクトリ（`mergepr-*`）と終わっていない `cleanup` がないことを確かめる。新しい `mergepr` は、古い定義が書いた `--protocol` のない `discard`・`cleanup` を拒むので、残っていたら新しい `mergepr` で `mergepr discard --protocol 3 --state <state-file>`（片付けなら `mergepr cleanup --protocol 3 --state <state-file>`）を手で実行する。

-`mergepr` を導入する PR そのものは、まだ `main` にツールが無いため `/mergepr` ではマージできない。`gh pr merge --squash` など、別の手段でマージする。
+**導入する PR そのもの.** `mergepr` を導入する PR そのものは、まだ `main` にツールが無いため `/mergepr` ではマージできない。`gh pr merge --squash` など、別の手段でマージする。

 ## 4. 処理の流れ

@@ -387,7 +393,7 @@ Dropped editorial corrections: <a> disputed by a reviewer, <b> reverted on the b
 2. その道具の `Protocol` を期待する値にする。
 3. コマンドの定義と手引きに書いた番号（`--protocol` の値を含む）を同じ値にする。
 4. commit して push する。
-5. その PR のブランチで、その道具を `make install-<道具>` で入れ直す。
+5. 利用者が、その PR のブランチで、その道具を `make install-<道具>` で入れ直す。
 6. マージをやり直す。`/runpr` から使ったときは `/runpr` を、単独のときは `/mergepr` を実行し直す。

 head の値が `origin/main` の値と同じ道具があるとき（上げた番号が `main` の取り込みで吸収された場合と、上げてから元に戻した場合）は、`origin/main` の上で PR の履歴をまとめ直し、番号を変えたコミットを残さないことも直し方になる。まとめ直してよいのは、この PR 自身の変更が道具の約束を変えないとき（番号を変えたコミットがほかのブランチから来たか、上げてから元に戻したとき）に限る。そうでなければ、まとめ直しは衝突を隠すので、上の手順だけが直し方である。
diff --git a/docs/dev/guides/numreserve_guide.md b/docs/dev/guides/numreserve_guide.md
index 9d03a33c1fa7440c8d2b3abf63da4f12057f8a24..81f1ae0d8c888d80d1abf7680301e79fdcecaa21 100644
--- a/docs/dev/guides/numreserve_guide.md
+++ b/docs/dev/guides/numreserve_guide.md
@@ -4,13 +4,15 @@ numreserve は、タスク番号と ADR 番号を、別のクローンや cloud

 ## 1. インストールと呼び出し

+`numreserve` は `main` から入れ、番号を決めるときだけ呼ぶ。
+
 ```bash
 make install-numreserve
 ```

-`go install ./cmd/numreserve` を実行する。道具は `main` から入れ、道具を変えた PR がマージされたら、`main` を更新してから入れ直す（[ADR-0014](../adr/0014-deterministic-checks-in-go.md) の「道具は `main` からインストールして使う」）。エージェントは道具を入れたりビルドしたりしない。道具が見つからないと、`/mkreq` と ADR の手順は番号を決めずに止まる。
+`go install ./cmd/numreserve` を実行する。道具は `main` から入れ、道具を変えた PR がマージされたら、`main` を更新してから入れ直す（[dev_tools.md](../conventions/dev_tools.md) の「Rule 5: Install tools from main; agents never install or build them」）。エージェントは道具を入れたりビルドしたりしない。開発のための例外は、同じ規約の Rule 5 にある。道具が見つからないと、`/mkreq` と ADR の手順は番号を決めずに止まる。

-呼び出しの形は次の 2 つだけである。引数は番号の空間を選ぶ 1 語で、ほかの引数とフラグは受け付けない（終了コード 2）。リポジトリの作業ツリーのどこかで実行する。ADR 番号は、ADR を加える PR のブランチの作業ツリーで実行する。
+**呼び出しの形.** 呼び出しの形は次の 2 つだけである。引数は番号の空間を選ぶ 1 語で、ほかの引数とフラグは受け付けない（終了コード 2）。リポジトリの作業ツリーのどこかで実行する。ADR 番号は、ADR を加える PR のブランチの作業ツリーで実行する。

 | 番号の空間 | 呼び出し | 記録の tag | ruleset の対象 |
 |---|---|---|---|
@@ -20,9 +22,9 @@ make install-numreserve
 -   **番号を決めたとき:** 標準出力に 4 桁の番号を 1 行だけ出し、終了コード 0 で終わる。標準エラーには、作った記録の tag の名前と、それが指す `origin/main` の commit を 1 行出す。
 -   **止まったとき:** 標準出力に何も出さず、標準エラーに `numreserve: stop <理由の語>: <段>: <詳細>` の 1 行を出し、終了コード 1 で終わる。理由の語ごとの意味と次にすることは 2 章の表にある。詳細には、止まった段、HTTP の状態コードやコマンドの終了コード、原因のブランチ名やファイルのパスを出す。ブランチ名とファイルのパスは Go の引用の形で、200 バイトまでに切って出す。GitHub の応答の本文、`gh` と `git` の標準エラー、`origin` の URL、認証の情報は出さない。

-`.claude/settings.json` は `Bash(numreserve task)` と `Bash(numreserve adr)` の 2 つだけを許すので、この 2 つの形なら許可を尋ねない。呼ぶたびに消せない記録の tag を作るので、エージェントは手順が新しい番号を求めるときだけ呼び、試しや確かめのためには呼ばない（[CLAUDE.md](../../../CLAUDE.md) の「Tool Execution Safety」）。番号を決めた後に作業が止まると、その番号は欠番として残る。欠番は誤りではない。
+**許可の規則.** `.claude/settings.json` は `Bash(numreserve task)` と `Bash(numreserve adr)` の 2 つだけを許すので、この 2 つの形なら許可を尋ねない。呼ぶたびに消せない記録の tag を作るので、エージェントは手順が新しい番号を求めるときだけ呼び、試しや確かめのためには呼ばない（[CLAUDE.md](../../../CLAUDE.md) の「Tool Execution Safety」）。番号を決めた後に作業が止まると、その番号は欠番として残る。欠番は誤りではない。

-道具は、`origin` が GitHub の `isseis/yt2column` であることを前提にする。リポジトリの名前は道具の定数で、`origin` の URL は読まない。`origin` が別のリポジトリや古いクローンを指すと、使用中の番号を見落としうる。
+**前提.** 道具は、`origin` が GitHub の `isseis/yt2column` であることを前提にする。リポジトリの名前は道具の定数で、`origin` の URL は読まない。`origin` が別のリポジトリや古いクローンを指すと、使用中の番号を見落としうる。

 ## 2. 止める理由と次にすること

````

Diff record (AC-24):

- Compared paths: `.claude/settings.json`
- Merge-base: `9122d1aaad7244eac37198da4ca30e1da133a504`
- Compared commit: `ff31c6465eeda9de15ad1cc3e9f0d21d5036db3b`

Path diff:

```text
diff --git a/.claude/settings.json b/.claude/settings.json
index bd32ef277d72d54bc52d37723a6aabb5a2afcdd6..7ec7a17a6a592a9ca19000315c7a0cbd144c387f 100644
--- a/.claude/settings.json
+++ b/.claude/settings.json
@@ -43,7 +43,9 @@
       "Bash(herdr worktree open*)",
       "Bash(git worktree add*)",
       "Bash(git worktree remove*)",
-      "Bash(git worktree move*)"
+      "Bash(git worktree move*)",
+      "Bash(make install-*)",
+      "Bash(go install *)"
     ]
   }
 }
```

## 8. 成功の基準 (Success Criteria)

-   7 章のすべての AC の確かめが済み、記録がある。
-   `make test` と `make lint` が通る。
-   規約が規則 1〜6 の正本になり、ADR-0014・`CLAUDE.md`・ガイド・コマンドの定義が規約を指す。
-   セキュリティ：拒否の規則が Claude Code で効くことを人が確かめ、結果が PR のコメントにある（AC-24）。

## 9. 次のステップ (Next Steps)

-   実装の PR のマージの後に、人が次の 2 点を issue にするかを決める。
    -   設計書 9 章の別の issue の候補（拒否の規則のガード、依頼にビルド元を添えること、番号の不一致の直し方に PR のブランチの場合を足すこと、OpenCode の許可の設定、ADR-0014 の「結果」の 1 文、規則 1 と規則 6 の範囲の食い違い、外部の道具をまとめて入れるターゲット）
    -   `docs/dev/guides/chief_guide.md` の 6 章（`.claude/settings.json` の拒否）が herdr と git の規則だけを挙げ、本タスクで足す 2 つの規則を挙げないこと

## 変更履歴

- 2026-11-19 承認（isseis、#1037）
