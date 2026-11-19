# 実装計画書：開発の道具の規則を ADR-0014 から規約 dev_tools.md に移す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-19 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番、H-NN は [design_handoff.md](design_handoff.md)、I-NN は [implementation_handoff.md](implementation_handoff.md) の項目を指す。節番号は、とくに断らない限り [02_architecture.md](02_architecture.md)（以下、設計書）の節である。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

英語の規約 `docs/dev/conventions/dev_tools.md` を作り、開発の道具の規則 1〜6 の正本をそこに移す。ADR-0014 の 7 つの項目を要約と規約へのリンクに書き直し、`CLAUDE.md`・コマンドの定義・ガイド・スクリプトのコメントを規約に合わせて直し、`.claude/settings.json` に拒否の規則を 2 つ足す（設計書 1 章・3 章）。

### 1.2. 実装の原則

-   **書く中身は設計書に従う。** 規約の見出しと各節が持つものは設計書 3.1、ADR-0014 から外す文の行き先は 3.2、`CLAUDE.md` の 4 種類の箇所は 3.3、主語の直しは 3.4、例外への参照は 3.5、参照の付け替えは 3.6、拒否の規則と用語集は 3.7 に従う。本計画は、それらを繰り返さない。
-   **規則の内容は変えない。** 規約の各節は、ADR-0014 の対応する項目の条件を落とさず、広げない（AC-02）。行き先の決まっていない文は、ADR-0014 から外さない（設計書 1.1）。
-   **直す場所は中身で探す。** 設計書の行番号は `e43f8161` のもので、`CLAUDE.md` はその後のコミットで行がずれている（1.4）。
-   **既存のガードが読む値と構造を変えない。** 直す箇所を読むテストは設計書 3.8 の表のとおりで、どのテストも直さない。`mergepr_guide.md` の `--protocol` を含む記述（設計書 3.4）と、`CLAUDE.md`「Tool Execution Safety」の `numreserve` の呼び出しの形（設計書 3.3）には触れない。
-   **編集上の訂正として記録する。** ADR-0014 の書き換えは、コミットのメッセージに訂正行を書き、PR の説明に訂正の段落を書く（AC-11、設計書 3.2「記録」）。訂正行は文を写さずに自分の言葉で書き、`<` と `@` を含めない（I-03）。

### 1.3. 計画の際に決めたこと（2026-11-19）

-   **差分の範囲の AC（AC-10・AC-18・AC-20・AC-21・AC-22）と AC-24 は、記録した diff を読んで確かめる。** 2026-11-19 に人が選んだ。どれも小さな文書の差分で、スクリプトで判定するより読むほうが確かなためである（`docs/dev/conventions/requirements_process.md` §4「`diff` verification」の「Proportion」）。I-02 のとおり、AC ごとに記録を持つ。読み切れないと分かったら、同じ節の「Output too long to read」に従う。
    -   採らなかった案：AC ごとに `_checks/` のシェルのスクリプトを書く案。読む量は減るが、作る量と、壊して確かめる量が増える。
    -   ADR: なし。確かめ方の選択で、本タスクの中で閉じる。
-   **拒否の規則が Claude Code で効くことは、PR のレビューで人が確かめる（`manual`）。** 2026-11-19 に人が選んだ。実装モデル要件が `sonnet` の `/runplan` は OpenCode で走り（`.claude/commands/_context.md`「Command routing」）、OpenCode には `.claude/settings.json` が効かないので、設計書 7 章の確認を `/runplan` の中では行えないためである。人は、PR のブランチを checkout した作業ツリーで Claude Code のセッションを開き、確かめる。確かめるのは PR を作った後なので、自動送信は `auto` のままとする（3.2）。
    -   採らなかった案：自動送信を `hold` にし、`/runplan` の前後に人が対話のセッションで確かめる案。PR のレビューで確かめれば足りる。
    -   採らなかった案：Claude Code では確かめず、`cmd/chief/docs_test.go` の `matchesRule` のモデルに頼る案。設計書 7 章と食い違う。
    -   ADR: なし。確かめ方の選択で、本タスクの中で閉じる。
-   **AC-11 の訂正行は、`_checks/` のシェルのスクリプトで確かめる（`static`）。** 2026-11-19 に人が選んだ。訂正行はコミットのメッセージにあり、今の `/mergepr` の手順はそれを読まない（`.claude/commands/mergepr.md` に訂正行を扱う手順がない。`efb51aac` で確かめた）。読むだけでは、AC に `test`・`static`・`diff` の確かめを持たせる規則を満たさないためである。PR の説明の訂正の段落は、PR のレビューで読む（`manual`）。
    -   採らなかった案：コミットのメッセージと PR の説明を読むだけにする案。
    -   ADR: なし。確かめ方の選択で、本タスクの中で閉じる。
-   **フェーズは 1 つ、PR は 1 本とする。** I-01（確かめ済み）と設計書 8 章のとおりである。ステップの順は設計書 8 章に従う。実装モデル要件は I-01 の決定（`sonnet`）を採る。
    -   ADR: なし（I-01 のとおり）。

### 1.4. 既存コード調査結果

コミット `efb51aac`（`origin/main`）で確かめた。

-   **設計書の行番号のずれ。** 設計書は `e43f8161` で確かめた。`e43f8161..efb51aac` で、本タスクが変えるファイルのうち変わったのは `CLAUDE.md`・`docs/dev/guides/chief_guide.md`・`docs/translation_glossary.md` だけである（`git diff --stat e43f8161 efb51aac -- <AC-21 のファイル>`）。
    -   `CLAUDE.md` は、`:35` の後に 4 行、`:582` の後に 30 行が足された。そのため、設計書の `CLAUDE.md:1051-1052`・`:1060-1061`・`:1076-1077` は今の `:1085-1086`・`:1093-1095`・`:1110-1111`、`:842-843` は今の `:876-877` である。
    -   `chief_guide.md` の変更は 3.1 節と 5 章だけで、本タスクが直す `:22` と `:30` は変わっていない。
    -   用語集の変更は「ラウンド」の行の後への 5 行の追加で、設計書 3.7 が 3 行を足す位置の「ガードツール」の行（`docs/translation_glossary.md:134`）は変わっていない。
-   **ADR-0014。** `e43f8161` から変わっていない。「決定」の項目は `:17`〜`:26`、「結果」の拒否の規則の文は `:61` で、設計書 3.2 の表のとおりである。
-   **直す箇所の今の文。** 設計書 3.4〜3.6 の表の「今の文」は、`efb51aac` でも同じである。`.claude/commands/mergepr.md:18-19`、`.claude/commands/_lib/local-cli-review.md:91-93`、`.claude/commands/_lib/adr-correction-candidates.sh:63-64`、`docs/dev/guides/numreserve_guide.md:11`、`docs/dev/guides/chief_guide.md:22`・`:30`、`docs/dev/guides/fixprguard_guide.md:54`、`docs/dev/guides/mergepr_guide.md:48`・`:286` を読んで確かめた。
-   **`.claude/settings.json`。** `permissions.deny` は 33 の規則（`herdr` と `git worktree`）で、最後は `Bash(git worktree move*)`（`.claude/settings.json:46`）である。`make` と `go` の規則はない。
-   **読むテスト。** 設計書 3.8 の表のテストは、すべて次のファイルにある。テストのコードは変えない（AC-21 にテストのファイルは入らない）。
    -   `cmd/yt2column`：`TestADRRecords`（`adr_test.go`）、`TestInstructionDocPaths`（`docpaths_test.go`）、`TestCodexMentionOnlyInRequestRow`（`codex_review_docs_test.go`）、`TestADRCorrectionCandidates`（`adr_candidates_docs_test.go`）
    -   `cmd/mergepr/docs_test.go`：`TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`・`TestProtocolToolsAgree`
    -   `cmd/codexreview/docs_test.go`：`TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`・`TestNoDirectCodexInvocation`・`TestSettingsAllowsTool`
    -   `cmd/chief/docs_test.go`：`TestSettingsDeniesPaneInput`・`TestSettingsDeniesWorktreeChanges`・`TestSettingsAsksBeforeRemove`・`TestGuideValuesMatchTool`
    -   `cmd/numreserve/docs_test.go`：`TestDocumentedInvocations`・`TestSettingsAllowsTool`・`TestGuideValuesMatchTool`
    -   `cmd/fixprguard/docs_test.go`：ガイドの表と値を読むテスト
-   **規約の置き場所を読むテスト。** `cmd/chief`・`cmd/mergepr`・`cmd/codexreview`・`cmd/fixprguard`・`cmd/numreserve` の文書のテストは、`docs/dev/conventions/` のうち `adr.md` だけを読み（`cmd/numreserve/docs_test.go:197`）、ほかの規約を走査しない。新しい規約が読まれるのは `TestInstructionDocPaths`（リンクの解決）だけである。
-   **再利用。** 新しいテストも補助のファイルも足さない（設計書 7 章）。確かめは既存のガードと diff の記録、AC-11 のスクリプトで行う。`_checks/` のスクリプトは、本タスクで新しく書く。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 規約の作成と、規則の置き場所の付け替え（設計書 3 章・8 章、AC-01〜AC-25）

ステップごとの確かめ（I-04）では、そのステップが変えるファイルを読むパッケージのテストを走らせる。対象は `go test -tags test` に、ステップごとに示すパッケージを渡す。

#### ステップ 1-1: 規約と用語集

**変えるファイル:** `docs/dev/conventions/dev_tools.md`（新規）、`docs/translation_glossary.md`

- [ ] 設計書 3.1 の見出しの表と、規則 5 の段落の表に従って、規約を英語で書く（AC-01〜AC-07）。規則ごとに 1 つの節とし、理由は書かずに ADR-0014 へリンクする
- [ ] 規約の規則 5 の節に、設計書 3.1 の「止まって頼むときの決まり」を書く（AC-05）
- [ ] 用語集の「ガードツール」の行の後に、設計書 3.7 の 3 行を足す（H-03）
- [ ] 規約を書き終えたら、ADR-0014 の `:17`〜`:26` の各項目の条件を 1 つずつ規約の該当する節と照らし、落ちた条件と広がった条件がないことを確かめる（AC-02）。規約は ADR-0014 の文を写したものではないので、照らした結果は PR の説明に書く

**完了の条件:** `go test -tags test ./cmd/yt2column` が通る（規約のリンクが解決する）。

#### ステップ 1-2: ADR-0014 の 7 つの項目の書き直し

**変えるファイル:** `docs/dev/adr/0014-deterministic-checks-in-go.md`

- [ ] 設計書 3.2 の表の 7 つの項目（`:17`・`:18`・`:20`・`:22`・`:23`・`:25`・`:26`）を、「外す文の行き先」の表のとおりに書き直す（AC-09）。規約の節は、アンカーを使わず見出しの名前を本文で示してリンクする（`docs/dev/conventions/adr.md`「Links」）
- [ ] `:19`・`:21`・`:24` の 3 項目と、「背景」「採らなかった案」「結果」「経緯」には触れない（AC-10）。「結果」の `:61` の文も直さない（設計書 3.2「『結果』の 1 文」）
- [ ] コミットのメッセージに、ADR-0014 の訂正行を書く（AC-11）。消す 2 つの参照（「（タスク 0112 の実装で足す）」と「規則は…0112 要件定義書にある」）は古くなった参照、それ以外は基準に沿った書き直しとして書き分ける（設計書 3.2「記録」）。訂正行は自分の言葉で書き、`<` と `@` を含めない（I-03）

**完了の条件:** `go test -tags test ./cmd/yt2column` が通る（`TestADRRecords` と、ADR のリンクの解決を含む）。

#### ステップ 1-3: CLAUDE.md の 4 種類の箇所

**変えるファイル:** `CLAUDE.md`

- [ ] 「Quick Links」の「Development Guides」に、規約へのリンクと要旨の行を足す（AC-13、設計書 3.3）
- [ ] 「Tool Execution Safety」に、設計書 3.3 の 1 項目を足す（AC-14）。道具を名前で並べず、`numreserve task` のような呼び出しの形をコードスパンで書かない
- [ ] 「Tool Execution Safety」の `codexreview`・`numreserve`・`chief` の「install or build」の文に、開発のための例外があることを規約へのリンクで添える（AC-23）。今の文は残す
- [ ] 「Protocol numbers」の「The merge checks it.」の項目の「reinstall the tool from the branch」を、利用者が入れ直すことを述べる形にする（AC-15）

**完了の条件:** `go test -tags test ./cmd/yt2column ./cmd/numreserve ./cmd/codexreview ./cmd/chief ./cmd/mergepr` が通る（`TestDocumentedInvocations`・`TestCodexMentionOnlyInRequestRow`・リンクの解決を含む）。

#### ステップ 1-4: コマンドの定義・ガイド・スクリプトのコメント

**変えるファイル:** `.claude/commands/mergepr.md`、`.claude/commands/_lib/local-cli-review.md`、`.claude/commands/_lib/adr-correction-candidates.sh`、`docs/dev/guides/mergepr_guide.md`、`docs/dev/guides/fixprguard_guide.md`、`docs/dev/guides/chief_guide.md`、`docs/dev/guides/numreserve_guide.md`

- [ ] `mergepr.md` の `mergepr` を入れる指示を、設計書 3.4 のとおりに直す（AC-17）
- [ ] `mergepr_guide.md` の `:48` と 7 章の手順 5、`fixprguard_guide.md:54`、`chief_guide.md:30` の主語を、設計書 3.4 のとおりに利用者にする（AC-16）。`mergepr_guide.md:48` は主語の部分だけを直し、`--protocol` を含む記述と「マージの直後に `main` から再インストールする」は変えない
- [ ] `local-cli-review.md:91-93` と `numreserve_guide.md:11` に、開発のための例外への参照を設計書 3.5 のとおりに添える（AC-23）
- [ ] `chief_guide.md:22` と `numreserve_guide.md:11` の ADR-0014 への参照を、規約の規則 5 の節へのリンクに付け替える（AC-19、設計書 3.6）
- [ ] `adr-correction-candidates.sh:63-64` のコメントの ADR-0014 のパスを、規約のパスに付け替える（AC-20）。コードは変えない

**完了の条件:** `go test -tags test ./cmd/yt2column ./cmd/mergepr ./cmd/codexreview ./cmd/chief ./cmd/numreserve ./cmd/fixprguard` が通る（`TestADRCorrectionCandidates`・`TestDocumentedProtocolMatches`・`TestGuideValuesMatchTool`・リンクの解決を含む）。

#### ステップ 1-5: 拒否の規則

**変えるファイル:** `.claude/settings.json`

- [ ] `permissions.deny` の最後に、設計書 3.7 の 2 つの規則（`Bash(make install-*)` と `Bash(go install *)`）を足す（AC-24）。ほかの規則は、並びも含めて変えない

**完了の条件:** `go test -tags test ./cmd/chief ./cmd/codexreview ./cmd/numreserve` が通る（`settings.json` を読むテスト）。

#### ステップ 1-6: 確かめの記録と仕上げ

**変えるファイル:** `docs/tasks/0112_dev_tools_convention/_checks/correction_lines.sh`（新規）、本計画書

- [ ] AC-11 のスクリプトを `_checks/correction_lines.sh` に書く。merge-base から `HEAD` までのコミットについて、ADR-0014 を変えたコミットに ADR-0014 のパスを名指す訂正行が 1 つ以上あることと、範囲のどの訂正行も `<` と `@` を含まないことを確かめる。言語はシェルとする。各確かめが少なくとも 1 つの壊し方で失敗することを確かめ、そのことをコミットのメッセージに書く
- [ ] スクリプトを PR のブランチで走らせ、出力と、merge-base と比べたコミットの ID を 7 章に記録する
- [ ] `make test` と `make lint` を通す（AC-12・AC-25）
- [ ] 7 章の diff の記録（AC-10・AC-18・AC-20・AC-21・AC-22・AC-24）を、`requirements_process.md` §4 の決まったコマンドで取り、書く。AC-21 の記録は、PR の実装の最後のコミットで取る（I-02）
- [ ] PR の説明に、ステップ 1-1 の AC-02 の照らし合わせの結果と、7 章の AC-24 の Claude Code での確認の手順（人が行う）を書く

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

**判定理由**: どのトリガーにも当たらない（文書と設定の 2 行だけの変更で、方式の選択も危険度の高いステップもない。I-01 で確かめ済み）

**自動送信**: auto

## 4. テスト戦略 (Test Strategy)

-   **単体テスト・結合テスト:** 足さない（設計書 7 章）。AC-21 は、実装の PR が変えてよいファイルにテストのファイルを入れていない。
-   **既存のガード:** 設計書 3.8 の表のテストを、ステップごとの完了の条件で走らせ、ステップ 1-6 で `make test` 全体を走らせる（AC-12・AC-25、I-04）。
-   **差分の確かめ:** 7 章の diff の記録を PR のレビューで読む（1.3）。
-   **訂正行:** `_checks/correction_lines.sh` を PR のブランチで一度走らせ、記録する（AC-11）。常設のテストではない。
-   **拒否の規則の効き方:** 人が、PR のブランチの Claude Code のセッションで一度確かめる（7 章の AC-24）。引数のない `go install` は、拒まれなかったときにパッケージを入れてしまうので試さない（設計書 7 章）。
-   **文の内容:** 規約・ADR・`CLAUDE.md`・ガイドの文は、PR のレビューで読む（`CLAUDE.md`「Testing Strategy」）。
-   **後方互換:** N/A。道具とコマンドの振る舞いと、プロトコル番号を変えない（要件書 4.4）。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 規約を書くときに、ADR-0014 の条件を落とすか広げる | ステップ 1-1 で、ADR-0014 の項目ごとに規約と照らし、結果を PR の説明に書く。レビューで読む（AC-02） |
| ADR-0014 から、行き先のない文を外す | 設計書 3.2 の「外す文の行き先」の表の行ごとに書き直す。AC-10 の diff の記録をレビューで読む |
| 文の直しで、ガードが読む値（`--protocol N`、`numreserve` の呼び出しの形、`@codex`）を壊す | 設計書 3.3・3.4 の注意に従い、ステップごとに読むパッケージのテストを走らせる（I-04） |
| 訂正行に `<` や `@` が入る | 自分の言葉で書く（I-03）。`_checks/correction_lines.sh` で確かめる |
| 拒否の規則の形を誤り、守りが効かないか、`make test` などを拒む | 設計書 3.7 の形のとおりに書き、PR のレビューで人が Claude Code で確かめる（AC-24） |
| 本タスクのマージの前に、ほかの PR が同じファイル（`CLAUDE.md`、ガイド、`settings.json`）を変える | 後からマージする側が `origin/main` を取り込んで解く。直す場所は中身で探す（1.2） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1（ステップ 1-1〜1-6）
- [ ] PR を作成した: <!-- PR URL -->
- [ ] PR がマージされた

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `static`・`manual` | `cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（`CLAUDE.md` からのリンクが解決し、規約のファイルがある）。英語で書かれ、規則ごとに 1 つの節があることは、PR のレビューで読む |
| AC-02 | `manual` | PR のレビューで、規約の各節を ADR-0014 の対応する項目と照らして読む。ステップ 1-1 の照らし合わせの結果（PR の説明）を手がかりにする |
| AC-03 | `manual` | PR のレビューで、規約の Kinds of tools を読む |
| AC-04 | `manual` | PR のレビューで、規約の Rule 2 を読む |
| AC-05 | `manual` | PR のレビューで、規約の Rule 5 の各段落を読む |
| AC-06 | `manual` | PR のレビューで、規約の冒頭のリンクを読む |
| AC-07 | `manual` | PR のレビューで、規約の Not covered here を読む |
| AC-08 | `static`・`manual` | 要件定義の PR（#1024）で満たし、そのレビューで読んだ。ADR の構造は `cmd/yt2column/adr_test.go::TestADRRecords` が確かめる。AC-08 の項目が実装の PR で変わらないことは、AC-10 の記録で読む |
| AC-09 | `static`・`manual` | `cmd/yt2column/adr_test.go::TestADRRecords`（ADR の構造）、`cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（規約へのリンクの解決）。7 つの項目が規則の本文を持たないことは、PR のレビューで書き直した後の ADR-0014 を読む |
| AC-10 | `diff` | 下の記録で、差分が 7 つの項目の書き換えに限られることを読む |
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
| AC-22 | `diff` | 下の記録で、差分が AC-16・AC-17・AC-19・AC-20・AC-23・AC-24 の箇所に限られることを読む |
| AC-23 | `static`・`manual` | `cmd/codexreview/docs_test.go::TestNoDirectCodexInvocation`・`TestDocumentedInvocationsParse`（`local-cli-review.md` の呼び出しが変わらない）、`cmd/yt2column/docpaths_test.go::TestInstructionDocPaths`（規約へのリンクの解決）。例外への参照は PR のレビューで読む |
| AC-24 | `diff`・`static`・`manual` | 下の記録で、`deny` の最後に 2 つの規則が足され、ほかの規則が変わらないことを読む。`settings.json` を読む既存のテスト（`cmd/chief/docs_test.go::TestSettingsDeniesPaneInput`・`TestSettingsDeniesWorktreeChanges`・`TestSettingsAsksBeforeRemove`、`cmd/codexreview/docs_test.go::TestSettingsAllowsTool`、`cmd/numreserve/docs_test.go::TestSettingsAllowsTool`）が通る。PR のレビューで、人が PR のブランチの Claude Code のセッションで、拒む形（`make install-` と `go install` で始まり、当たっても何も入らない書き方。たとえば存在しないターゲットやパス）が拒まれ、拒まない形（たとえば `make -n test`）が拒まれないことを確かめる（1.3） |
| AC-25 | `static` | `make test`（`settings.json` を読む上記のテストを含む） |

**I-NN の扱い:**

| 項目 | 扱い |
|---|---|
| I-01 | 採る。実装モデル要件は `sonnet`（3.2） |
| I-02 | 採る。差分の範囲の AC は AC ごとに記録を持ち、AC-21 の記録はファイルの一覧を持って PR の実装の最後のコミットで取る。AC-20 にもそのパスの記録を持たせる（下の記録、ステップ 1-6） |
| I-03 | 採る。ステップ 1-2 で、訂正行を自分の言葉で書き、`<` と `@` を含めない。AC-11 のスクリプトで確かめる |
| I-04 | 採る。各ステップの完了の条件に、変えるファイルを読むパッケージのテストを入れた |

Script record (AC-11): `_checks/correction_lines.sh`

- Merge-base:
- Compared commit:

Output:

Diff record (AC-10):

- Compared paths: `docs/dev/adr/0014-deterministic-checks-in-go.md`

Diff record (AC-18):

- Compared paths: `CLAUDE.md`

Diff record (AC-20):

- Compared paths: `.claude/commands/_lib/adr-correction-candidates.sh`

Diff record (AC-21):

- Compared paths: `docs/translation_glossary.md`

Diff record (AC-22):

- Compared paths: `.claude/commands/mergepr.md`, `.claude/commands/_lib/adr-correction-candidates.sh`, `.claude/commands/_lib/local-cli-review.md`, `.claude/settings.json`, `docs/dev/guides/chief_guide.md`, `docs/dev/guides/numreserve_guide.md`, `docs/dev/guides/mergepr_guide.md`, `docs/dev/guides/fixprguard_guide.md`

Diff record (AC-24):

- Compared paths: `.claude/settings.json`

## 8. 成功の基準 (Success Criteria)

-   7 章のすべての AC の確かめが済み、記録がある。
-   `make test` と `make lint` が通る。
-   規約が規則 1〜6 の正本になり、ADR-0014・`CLAUDE.md`・ガイド・コマンドの定義が規約を指す。
-   セキュリティ：拒否の規則が Claude Code で効くことを、人が確かめた（AC-24）。

## 9. 次のステップ (Next Steps)

-   実装の PR のマージの後、設計書 9 章の別の issue の候補（拒否の規則のガード、依頼にビルド元を添えること、番号の不一致の直し方に PR のブランチの場合を足すこと、OpenCode の許可の設定、ADR-0014 の「結果」の 1 文、規則 1 と規則 6 の範囲の食い違い、外部の道具をまとめて入れるターゲット）を、人が issue にするかを決める。
