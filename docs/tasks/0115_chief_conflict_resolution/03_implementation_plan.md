# 実装計画書：chief は PR のマージコンフリクトを見つけて `/mergemain` で解消させ、解消できなければ人の判断を求める

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-23 |
| Review date | 2026-11-24 |
| Reviewer | isseis |
| Comments | - |

## 1. 実装概要

### 目的

[01_requirements.md](01_requirements.md)（以下、要件定義書）の F-001〜F-007（AC-01〜AC-37）と [02_architecture.md](02_architecture.md)（以下、設計書）に基づき、次の 4 点を順に実装する。

1. `mergepr` への取り込みの道具（`mergemain-begin`・`mergemain-merge`・`mergemain-restore`・`mergemain-push`）の追加。
2. コマンド定義 `.claude/commands/mergemain.md`、`Makefile` の `ext-preflight` ターゲット、`.claude/settings.json` の allow 規則、および文書のガードテストの追加。
3. `chief` の PR 値読み取り拡張、コンフリクトの行とコンフリクトの段階の分類・表・出力、送り先の表 7 行化、`paneinput` と人の後の入力の読み取りの追加。
4. `chief` の解消の送信の記録保持、解消の自動送信、人が選ぶ送信、送る前の報告の控えと `chief detail` の親たどり引き継ぎの追加。

### 実装原則

- 設計書の順と境界を守る。 フェーズの順は設計書 8 章の実装の優先順位 1〜4 に一致させる。決定的な判定と確かめは `internal/mergepr` に置き、送る入力の組み立ては `internal/chief/paneinput` の型に閉じる。
- 既存の部品とテスト基盤を再利用する。 `internal/mergepr/protocolcheck.go` のプロトコル番号読み取り（`acceptedLine`・`commitChange`）と `gitReader`、既存テストヘルパー（`internal/mergepr/test_helpers.go`・`internal/mergepr/test_helpers_git.go`）、`internal/chief` の送信・記録・確かめ直し（`sendCommand`・`autoSender` の共通処理・`quietness`・`readReport`・`ReadRouting`）と既存のテスト基盤（`world_test.go`・`test_helpers.go`・`send_test.go`・`screen_test.go`）を再利用する。
- プロトコル番号は常に `main` の値 + 1 にする。 `mergepr` と `chief` の契約を変える PR では、`Protocol` 定数と対応するコマンド定義・ガイドの `--protocol` を同じ PR で `main` の値 + 1 に揃える。エージェントはインストール型ツールをインストールもビルドもしない（`CLAUDE.md`・ADR-0014）。
- **計画書で確定する細部。** 設計書が計画書へ委ねた細部のうち、次の 3 つをここで確定する（共通パッケージの要否・`commitID` の移行範囲・拡張の検査の準備の形・PR の境界は「計画の際に決めたこと」と §3.2 で確定する）。
  - **取り込みの道具の行出力（設計書 4.1）。** 既存の `cmd/mergepr/main.go:285-328`（`printPrepared` 等、コミット `d74acaeb` で確認）と `docs/dev/guides/mergepr_guide.md:166-177` の `key: value` 形式に合わせ、1 行目を `status: ok` または `status: stop`、停止時は続けて `reason: <reason>`・`restore: <needed|not_needed|unknown>` を出力し、以降に `base_head`・`head_branch`・`main_head`・`conflict`（`<class> <path>`）・`protocol_tool`（`<tool> <new_protocol>`）・`readme_changed`・`adr_main_changed`・`adr_conflict_number`・`ext_check_hint`・`new_head`・`detail` の各行を `key: value` 形式で出力する。`conflict` の `<path>`・`detail`・`head_branch` などの可変値に含まれる改行や制御文字は 1 行にエスケープして出力し、行プロトコルの `status:`・`reason:`・`restore:` 等のフィールド偽装を防ぐ（`/mergemain` 側も可変値をフィールド名として解釈しない）。
  - **解消の自動送信の理由の種類（設計書 3.4.3）。** 設計書 3.4.3 の表の英語名に加え、作成排他を取れないときの `creation_locked` と、同ラウンドのマージ後自動送信が同じ workspace へ送ったときの `autosent_same_round` を定義する。
  - **報告の控えの引き継ぎ上限（設計書 3.7）。** `CollectDetail` が第 1 の親をたどる回数上限を `maxDetailMergeHops = 10` とする。
- 壊して失敗を確かめる。 新設・更新するテストとガードは、対応する分岐・検証・制約を一時的に外した状態で失敗することを実装時に確かめ、コミットメッセージに記録する（`CLAUDE.md`「Testing Strategy」）。

### 計画の際に決めたこと（2026-11-23）

- フェーズ 3（検出と表示）とフェーズ 4（送信）を別 PR に分ける。 検出・表示の PR と送信・記録保持・引き継ぎの PR を分離し、各 PR のレビュー規模とリスクを抑える（PR 境界設計ではさらにフェーズ 1 を `internal/mergepr` と `cmd/mergepr` の 2 PR に、フェーズ 4 を自動送信と手動送信・引き継ぎの 2 PR に分け、計 6 PR 構成とする）。`chief` のプロトコル番号は契約を変える各 PR で `main` の値 + 1 に上げる。人の答え：別 PR に分ける。
  - ADR：なし。PR の分割と実装順序の選択で本タスク内に閉じる。
- `mergepr` と `fixprguard` の共通パッケージは作らず、`internal/mergepr` 内に実装する。 push の引数形（`--no-follow-tags --recurse-submodules=no -- <remote> <commit>:refs/heads/<branch>`）のみ `internal/fixprguard/worktree.go:640-644` に合わせ、`internal/fixprguard` には変更を入れない。人の答え：共有せず `internal/mergepr` 内に実装する。
  - ADR：なし。パッケージ内部の実装構成で本タスク内に閉じる。
- `make ext-check` の準備確認（AC-26）は `Makefile` に `ext-preflight` ターゲットを追加して行う。 既存の `define ext_preflight`（`Makefile:407-413`）をそのまま呼ぶ `ext-preflight` ターゲットを `.PHONY` に加え、`CLAUDE.md` の `### Extension Commands` にも追記する（`cmd/yt2column/extension_docs_test.go:71-80` の `TestExtensionDocumentation` と整合させる）。`/mergemain` は `make ext-check` の前に `make ext-preflight` を実行し、失敗を「検査の準備がない」として「検査の失敗」と区別する。人の答え：`Makefile` に `ext-preflight` ターゲットを追加する。
  - ADR：なし。既存マクロを 1 ターゲットとして露出するだけで本タスク内に閉じる。
- `CommitID` 型は新規箇所（`OpenPR.Head` とコンフリクト解消関連）だけに適用する。 既存の `commitID` 正規表現を使う箇所（設計書 3.1 が挙げる `internal/chief/detail.go:129`、`internal/chief/workspace.go:134`、`internal/chief/create.go:483` に加え、`internal/chief/workspace.go:318,338,357`）のシグネチャは変えない。`CommitID` の構築は `internal/chief/construction_test.go` の `guardedTypes` で `ParseCommitID` に限定する。人の答え：新規箇所だけに適用する。
  - ADR：なし。型適用の範囲の選択で本タスク内に閉じる。

### 実装の際に決めたこと（2026-11-24）

- `mergemain-push` の push の送り先の確かめに、git の remote が `origin` だけであることと、環境変数 `GH_REPO` がないことの確かめを足す。`mergemain-begin` も、PR を読む前に同じことを確かめる（設計書 1.3・3.8.3）。 PR から読む値（AC-30）には PR のリポジトリの URL がなく、`gh` は PR を読むリポジトリを git の remote と `GH_REPO` から選ぶためである。人の答え：URL が同じことに、remote が `origin` だけであることと `GH_REPO` がないことを足す。
  - ADR：なし。`mergemain-push` の確かめの細部で、このタスクの中で閉じる。
- 取り込みの道具は、git がテキストの合わせ以外で衝突を解消する設定（マージドライバ、`info/attributes`、`merge.default`、作業ツリーの `.gitattributes`、`branch.<ブランチ>.mergeOptions`）を止まる条件にするか、読まないようにする（設計書 1.3）。 これらは Go の衝突を git に黙って解消させ、`mergemain-push` の確かめも通してしまうためである。マージの前に止まるときは「対象外の衝突・戻さない」とし、設計書 4.1 の表を道具の出力に従う形に直す。人の答え：決定として記録する。
  - ADR：なし。取り込みの道具の確かめの細部で、このタスクの中で閉じる。

### 実装の際に決めたこと（2026-11-25）

- 実物の PR での `mergeable` の確かめ（ステップ 3-3、設計書 1.2・H-01）は、開いている PR だけで行い、コンフリクト中の PR での確かめは PR-5 の `hold` の点検に回す。 確かめた時点で開いていた PR（#1143・#1144）はどちらも `MERGEABLE` で、コンフリクト中の PR がなかった。§5 のリスクの行は一時的な draft PR を作って確かめるとしていたが、それには GitHub への書き込み（ブランチの push、PR の作成と close、ブランチの削除）が要る。`CONFLICTING` を一覧で読めなくても、`UNKNOWN` と同じく送らない側に倒れ、影響は気づくのが遅れることだけである（設計書 1.2）。人の答え：今回は確かめない。
  - ADR：なし。このタスクの確かめの段取りで、このタスクの中で閉じる。

### 既存コード調査結果

以下はコミット `d74acaeb`（`origin/main`）で確かめた事実である。

- **`internal/mergepr` と `cmd/mergepr`:**
  - `internal/mergepr/mergepr.go:57` の `prViewFields` は `body` を含むため、`mergemain-begin`・`mergemain-push` は `state,baseRefName,headRefName,headRefOid,isCrossRepository` のみの別定数を使う（AC-30）。
  - `internal/mergepr/mergepr.go:63` の `Protocol` は現在 `3`、`mainRefspec` は既存 `prepare`/`merge` と同形。`internal/mergepr/protocolcheck.go:392` に `acceptedLine`、`:407-415` に `cmdrunner.GitHardening()` 付きの `gitReader`、`:533-547` に `listPRCommits`、`:851-871` に `commitChange` があり、そのまま再利用できる。
  - `internal/mergepr/test_helpers_git.go`（`//go:build test`）には一時 git リポジトリ構築ヘルパー `gitRepo`（`newGitRepo`・`Git`・`Commit`・`RawCommit`・`SetMain` 等）が、`internal/mergepr/test_helpers.go` には `fakeRunner`・`gitStep`・`ghStep` がすでにある。また `internal/mergepr/protocolcheck.go:885-909` の `WorktreeProtocolTools` は `internal/mergepr/` 直下の非 `_test.go` ファイルをすべて読むため（`test_helpers_git.go:33-35` の注記）、`const Protocol = ...` を含むテスト用 Go ソース文字列は非 `_test.go` ヘルパーに置かず `mergemain_test.go` 内に置く。
  - 実行環境の `git`（`git version 2.50.1 (Apple Git-155)`）で `git merge-tree --write-tree -z --no-messages <branch1> <branch2>` の存在を `git merge-tree -h` で確認した。作業ツリーやインデックスを汚さずに 2 つの親の自動マージ結果の木と衝突一覧を取得できるため、設計書 3.8.3 の検証に利用できる。
  - `cmd/mergepr/main.go:205-218` の `shapes` テーブルでサブコマンドごとのフラグを定義し、`:285-328` が `key: value` 形式で行出力を行う。
  - `cmd/mergepr/docs_test.go:31-34` の `definitionCalls`、`:61-83` の `TestDocumentedProtocolMatches`、`:86-149` の `placeholderValues`・`TestDocumentedInvocationsParse` を拡張する。`.claude/settings.json` allow 規則ガードは `cmd/numreserve/docs_test.go:42-146`（`namesTool`・`toolRuleMismatches` 単体テスト付き）のパターンに合わせ、allow 対象の 3 サブコマンド呼び出しが前置や連結のない単独のコマンド呼び出しであることを `doctestutil.PromptFree` で確かめる。また `mergemain.md` の直接 `git merge`/`git reset`/`git push` 禁止ガードは、`doctestutil.RunsCommand` がバイナリ名のみ判定し `doctestutil.CommandTexts` が散文中のインラインコードも拾う（`internal/doctestutil/commands.go:123-169,223-226`）ため、実行コマンドブロックを対象に git グローバルオプション（`-C`/`-c`/`--git-dir` 等）を読み飛ばしてサブコマンド名を判定する形にする（`git commit --no-edit` や AC-30/AC-31 の散文記述を誤検知しないようにする）。
- **`Makefile`・`.pre-commit-config.yaml`・`CLAUDE.md`:**
  - `Makefile:407-413` に `define ext_preflight`（Node/npm 版確認 + `extension/node_modules` 存在確認）があり、`ext-preflight` ターゲットとして公開すれば `TestExtensionDocumentation`（`cmd/yt2column/extension_docs_test.go:71-80`）が `Makefile` と `CLAUDE.md` の `### Extension Commands` の一致を自動検証する。
  - `.pre-commit-config.yaml` のフック（`go-test`, `readme-sync`, `go-tidy`, `golangci-lint`, `go-vet-integration`, `trailing-whitespace`, `end-of-file-fixer`, `check-yaml`, `check-added-large-files`, `check-merge-conflict`, `detect-private-key`, `no-commit-to-branch`, `gofumpt`）と `make test`/`make lint`/`make ext-check` が未追跡ファイルを残さないかの実測確認はフェーズ 2（ステップ 2-2）で実施して記録する。
- **`internal/chief/paneinput` と `CommandKind` の全分岐箇所:**
  - `internal/chief/paneinput/paneinput.go:92-177` に `CommandKind`・`commandKinds`・`ParseCommandKind`・`RouteKey`・`RouteKeys`、`:322-421` に `Command`・`NewRunpr`・`Text`・`Parts` がある。`CommandMergemain`・`RouteMergemain`・`NewMergemain(fixprguard.PRNumber)` を追加する。
  - `CommandKind` を参照・分岐する全箇所を横断検索した結果：`internal/chief/running.go:50-64`（`validRunningArg`、追加要）、`internal/chief/position.go:247-270`（`taskDocumentOf`、`/mergemain` は作業項目の表示を変えないため既存分岐で足りる）、`internal/chief/position.go:280-307`（`progressOf`、`ProgressResolving` を追加要）、`internal/chief/screen_transcript.go:309-345`（`readRunningCommand`・`taskCommandOf`、`scanHumanInputs` へ統合して `LaterInput` も返す；既存テストは `internal/chief/screen_test.go:780-836`）、`internal/chief/skiprecord.go:84-90,109-147`（`commandKinds`・`checkRanCommand`、`/mergemain` は追加せず拒否をテストで確認）、`internal/chief/records.go:689-695`（`SendRecord.Kind` のコメント更新要）、`internal/chief/routing.go:27-34`（`routingRows` に `"mergemain": paneinput.RouteMergemain` を追加要）。
- **`internal/chief` と `cmd/chief` の PR 読み取り・状態・表示・出力:**
  - `internal/chief/workspace.go:18-73,142-148` の `OpenPR`・`parsePRList`・`OpenPRs` に 4 メンバーを足す。既存の `gh pr list` 応答モックは `internal/chief/` 配下 8 ファイル（`workspace_test.go`・`world_test.go`・`snapshot_test.go`・`maintask_test.go`・`autosend_test.go`・`resetoffer_test.go`・`removal_test.go`・`send_test.go`）と `cmd/chief/main_test.go:634` の計 9 ファイルにあり、フェーズ 3 ですべて 4 メンバー付きに更新する。実物採取データ `internal/chief/testdata/gh/pr-list-open-867.json`（`internal/chief/testdata/README.md:3,21,98`）は手編集でフィールドを捏造せずそのまま残し（旧 4 メンバー応答の拒否テストとして活用し）、新 8 メンバーの実物採取サンプルをステップ 3-3 で匿名化規約に沿って追加する。
  - **`CommitID` の構築・変更経路の制限（具象型の選択）:** 設計書 3.1・3.11 に従い `CommitID` は新しいリーフパッケージへ切り出さず `internal/chief` に置く。同一パッケージ内の構築・書き換え経路（複合リテラル `CommitID{...}`、型省略リテラル `[]CommitID{{...}}`、ポインタリテラル `&CommitID{...}`、フィールド代入 `c.hex = ...`、メソッド・クロージャ内構築）は既存の `internal/chief/construction_test.go:20-32` の `guardedTypes` に `"CommitID": {"ParseCommitID"}` を追加して AST/型ガードで禁じ、各経路のミューテーションテストを同ファイルへ追加する。なお `construction_test.go:129` は `test_helpers.go` を含む全非 `_test.go` ファイルを走査し、`:94` はメソッド内のフィールド代入も拒否するため、`test_helpers.go` も `ParseCommitID` 経由で構築し、`SendRecord` の JSON 変換は `MarshalText`/`UnmarshalText`（`*c = parsed` 代入）＋ `omitzero` または検証付き文字列ワイヤ型とする。ゼロ値 `CommitID{}` はコンストラクタ外で作った無効値として `IsZero()` で判別し、送信や記録のキーに使わない。
  - `internal/chief/state.go:13-62,496-554` に `WorkspaceState`・`stateOrder`・`classify`、`internal/chief/position.go:58-79,207-226,280-307` に `Progress`・`urgencyOf`・`progressOf`、`internal/chief/table.go:204-223,308-341` に `progressWord`・`contentOf`、`internal/chief/output.go` に `RowOutput`・`PROutput`・`TickOutput`・`PendingOutput`・`progressName` がある。既存の網羅テスト `internal/chief/output_test.go:21-56`（`TestOutputValueNames`）・`internal/chief/table_test.go:100-125`（`TestTableFixedWords`）・`internal/chief/position_test.go:48-104`（`TestUrgencyTable`）を新状態・新 `Progress` に合わせて更新する。
  - `internal/chief/records.go:29` の `Protocol` は現在 `6`。`docs/dev/chief_architecture.md` のサブコマンド表・終了コード表・状態ディレクトリ節は `cmd/chief/docs_test.go:520-570` の `TestArchitectureValuesMatchTool` で検証されており、仕様更新に合わせて同期する。
- **`internal/chief` の記録・送信・詳細:**
  - `internal/chief/records.go:680-704,739-755,830-851,942-973,1443-1477` の `SendRecord`・`sendEnd`・`cleanup`・`ClearRecords`、および `internal/chief/send.go:447-560` を拡張する。
  - `internal/chief/autosend.go:318-332,406-447,781-835` の `checkRemote`・`quietness`・`ownerStill`・`checkOwner`・`locate`・`recheck` の仕組みを `conflictSender` と共有し、`internal/chief/tick.go:76-89,121-214` で `autoSender.run` の直後に実行する。
  - `internal/chief/pending.go:36-74,130-225`（`kindOrder` 含む）・`internal/chief/request.go:23-141,221-292,425-479`・`cmd/chief/main.go:71-96,244-258` で `PendingConflict` と `chief send --command mergemain --item <id>` を実装する（既存の `Send` テスト群は `internal/chief/send_test.go:450-1200` にある）。なお `request.go:221-292` は現在 `origin/main` の取得確認を行っていないため、AC-18 が求める AC-05 (3) の条件（`git.Fetch`・`checkRemote`）を `RequestMergemain` 経路へ組み込む。
  - `internal/chief/detail.go:452-631` に `keepPaneReport` と `CollectDetail`（`ReportRecordAllowed` 時のみ）の親たどり引き継ぎを足す。`internal/chief/test_helpers.go:219` の `readOnlyCommands` に `rev-list --parents` と PR head ブランチ取得（`fetch --no-tags --no-prune origin`）の形を許可追加する。
- **ADR の現在状態（AC-37）:**
  - `docs/dev/adr/0046-chief-of-staff.md:39-49,73-116` と `docs/dev/adr/0014-deterministic-checks-in-go.md:31,40,69` は、設計書承認 PR（`#1051`）で既に 0115 の決定が反映済みであることをコミット `d74acaeb` で確認した。本計画では新たな ADR 変更は発生せず、完了時に要件定義書との整合を再確認する。
- **`.claude/commands/_context.md` と関連文書:**
  - `.claude/commands/_context.md:157-167` の Command routing 表は現在 6 行。フェーズ 3 で `mergemain` 行（`Claude Code`, `opus`）を追加して 7 行とし、Claude Code 限定の例外を記載する（なお、ADR-0055 / issue #1072 の `runplan-*` 行キー改名は本タスクの対象外であり、既存 6 行のキー名は変えない）。`internal/chief/markers_parity_test.go:43-101` は `_context.md` を動的に読むため変更不要で、回帰テストとして実行確認する。
  - `docs/dev/conventions/package_reference.md` の `internal/mergepr`・`cmd/mergepr`・`internal/chief`・`internal/chief/paneinput` の記述を、各フェーズの公開 API・責務変更に合わせて更新する。

### 申し送りの扱い

- `design_handoff.md` の全項目（H-01〜H-12）は、すべて設計書 付録 A に方針が記録されていることを確認した。本計画では設計書付録 A のうち実装時に検証・記録が必要な項目（H-01 の実物 PR での `mergeable` 確認、H-05 の `review-records.sh coverage` 確認）を該当ステップに組み込む。
- `implementation_handoff.md` は本タスクディレクトリに存在しないため、`I-NN` 項目はない。

## 2. 実装ステップ

### フェーズ 1: 取り込みの道具（`mergepr`）

設計書 3.8・4.1・5.2・8 章（優先順位 1）に基づき、`mergepr` に 4 つのサブコマンドと決定的な判定・検証を実装する。

#### ステップ 1-1: `internal/mergepr` の前提確認・取り込み・衝突分類・プロトコル番号更新

**対象ファイル:** `internal/mergepr/mergemain.go`（新規）, `internal/mergepr/mergemain_test.go`（新規）, `internal/mergepr/test_helpers_git.go`, `internal/mergepr/test_helpers.go`

- [x] `PathConflictClass`・`MergemainStop`・`RestoreNeed`（ゼロ値はそれぞれ `PathConflictOutOfScope`・`MergemainStopUnknown`・`RestoreUnknown`）と、狭い PR 読み取り（`state,baseRefName,headRefName,headRefOid,isCrossRepository` のみ、本文・コメント・レビューを読まない）を実装する（AC-21・AC-30）。
- [x] `MergemainBegin` を実装する：PR 番号（10 進数字のみ）と PR 状態・`base=main`・別リポジトリでないことの検証、作業ツリーのクリーン確認（未追跡ファイル含む）、remote が `origin` だけで `GH_REPO` がないことの確認（PR を読む前）、`main`（`refs/mergemain/<PR>/main`）と PR head ブランチ（`refs/remotes/origin/<branch>`）の明示 refspec での取得、HEAD・PR head・remote-tracking ref の一致確認、取り込むものがない場合の判定（`MergemainStopNothingToTakeIn`）を行う（AC-20・AC-21）。
- [x] `MergemainMerge` を実装する：引数と `refs/mergemain/<PR>/main` の照合、無視ファイル上書きの事前検知、`--no-ff --no-commit` マージ、ステージ種別・バイナリ判定・名前変更検出・`acceptedLine` による衝突分類、ブランチが merge base から変更した道具（衝突あり・なし両方）のプロトコル番号を `main` の値 + 1 へ更新、および規則適用の事実（README 変更・ADR 変更/番号重複・`ext-check` 見込み・番号変更した道具一覧）の収集を行う（AC-22・AC-23）。
- [x] 既存の `test_helpers_git.go`・`test_helpers.go` を再利用・拡張し、`mergemain_test.go` に単体テスト（`.md`・両側追加・プロトコル行のみ・他行衝突・`_test.go`・modify/delete・3 種の名前変更・シンボリックリンク・バイナリ、早送り禁止、無視ファイル上書き拒否、ゼロ値の安全側動作）を追加して、各検証を外したときの失敗を確認する（`const Protocol = ...` を含むテスト用 Go ソース文字列は `mergemain_test.go` 内に置く）。

**完了基準:** `MergemainBegin`・`MergemainMerge` の単体テストがすべて通り、検証無効化時の失敗が確認されていること。

#### ステップ 1-2: `internal/mergepr` の戻し検証と push 検証・実行

**対象ファイル:** `internal/mergepr/mergemain.go`, `internal/mergepr/mergemain_push.go`（新規）, `internal/mergepr/mergemain_restore.go`（新規）, `internal/mergepr/mergemain_test.go`, `CLAUDE.md`

- [x] `MergemainRestore` を実装する（設計書 3.8.4）：戻してよい 2 状態（取り込み途中は `HEAD` が `base_head` かつ `MERGE_HEAD` が取り込む `main` と一致すること、未 push の merge コミットは `HEAD` の親が `(base_head, 取り込む main)` の順かつ手元の remote-tracking ref `refs/remotes/origin/<branch>` が `base_head` と一致すること）と戻さない 6 条件（ブランチ不一致、HEAD 不一致、コミット後変更、取り込み途中の対象外変更、基準 head 追跡パスを塞ぐ無視ファイル、未追跡ファイル存在）を検証し、安全なときのみ `git reset --hard <base_head>` で戻して戻し後の状態を検証する（`git clean`・`git merge --abort` は使わない、AC-28）。
- [x] `MergemainPush` を実装する（設計書 3.8.3）：PR 再読み取り（`main` 禁止・別リポジトリでないこと・ブランチ一致）、merge コミットの 2 親検証（第 1 の親が `base_head` と再読み取りした PR の head の両方に一致すること、および第 2 の親が取り込む `main` かつ `refs/mergemain/<PR>/main` と一致し同呼び出し内で再取得した `FETCH_HEAD` の祖先であること）、`git merge-tree --write-tree` による衝突分類と木の差分検証（許可パス・プロトコル番号 `main` の値 + 1 の全道具検証）、作業ツリー確認、`git ls-remote` での `origin` ブランチ確認、push URL と fetch URL の一致確認、remote が `origin` だけで `GH_REPO` がないことの確認（PR を読み直す前）を行う。すべて通ったうえで、force なし・`--no-follow-tags --recurse-submodules=no` で push し、失敗時は `git ls-remote` で 3 結果（通った・拒まれた・結果不明）を判定する（AC-27・AC-30）。
- [x] `internal/mergepr/mergemain_test.go` に戻し・push の単体テスト（戻し時の `MERGE_HEAD` 不一致・親順序不一致・`refs/remotes/origin/<branch>` 不一致の拒否、および push 時の第 1 の親と `base_head` または再読み取りした PR の head の不一致拒否を含む）、セキュリティテスト（設計書 7.1・7.3 の全拒否ケース）、および `begin`→`merge`→`push` と `begin`→`merge`→`restore` の結合テストを追加し、各検証を外したときのテスト失敗を確認する。
- [x] 出力の内容で振る舞いを選ぶ読み取り（衝突のステージ種別・`git merge-tree` の出力・`acceptedLine` による行の判定）を入れるこの PR で、`CLAUDE.md` の「Declare, don't infer」に設計書 3.8.5 の取り込みの道具の読み取り例外（範囲と境界）を追記する。

**完了基準:** `make test`・`make lint`・`make deadcode`・`make readme-sync-check` がすべてパスし、設計書 7.1・7.2・7.3 の取り込みの道具に関する全ケースと壊して失敗の確認が揃っていること。

### PR-1 作成ポイント: internal/mergepr take-in logic and security gates

**対象ステップ**: 1-1 / 1-2

**推奨タイトル**: `feat(0115): internal/mergepr に main 取り込みの判定・戻し・push 検証を追加`

**レビュー観点**: 衝突分類の網羅性と名前変更・バイナリの拒否 / mergemain-push の親・木の差分・プロトコル番号・URL 検証 / mergemain-restore の戻し条件と未追跡・無視ファイル保護 / ゼロ値の安全側動作と一時 git リポジトリ結合テスト / CLAUDE.md の Declare, don't infer 例外

**実装モデル要件**: opus

**判定理由**: ステップ 1-1・1-2 が信頼の境界の検証ゲート（衝突分類・push 前の親と木の差分とプロトコル値検証・安全な戻し検証）を担い mkplan.md ステップ 8 のセキュリティゲート条件に該当するため。Sonnet では名前変更やバイナリの衝突分類、merge-tree 比較とプロトコル行の除外、reset --hard 前の安全条件に抜けが生じうる。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1114
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 1-3: `cmd/mergepr` CLI 口、プロトコル番号更新、ガイド・関連文書更新

**対象ファイル:** `cmd/mergepr/main.go`, `cmd/mergepr/main_test.go`, `cmd/mergepr/docs_test.go`, `internal/mergepr/mergepr.go`, `internal/mergepr/mergemain.go`, `internal/mergepr/mergemain_test.go`, `internal/config/envaccess_test.go`, `.claude/commands/mergepr.md`, `.claude/commands/runpr.md`, `docs/dev/guides/mergepr_guide.md`, `docs/dev/conventions/package_reference.md`

- [x] `cmd/mergepr/main.go` の `shapes` とディスパッチに `mergemain-begin`・`mergemain-merge`・`mergemain-restore`・`mergemain-push` を追加し、§1 で確定した `key: value` 形式の出力（`conflict` のパスや `detail` 等の可変値に含まれる改行・制御文字の 1 行エスケープを含む）と終了コードを実装する。`cmd/mergepr/main_test.go` にフラグ検証・出力形式テスト（改行・制御文字を含むパスや詳細でも `status:`・`reason:`・`restore:` 等の行が偽装されないことの確認を含む）を追加する。
- [x] `internal/mergepr/mergepr.go` の `Protocol` を `main` の値 + 1 に上げ、`.claude/commands/mergepr.md`・`.claude/commands/runpr.md`・`docs/dev/guides/mergepr_guide.md` の `--protocol` を合わせる。
- [x] `docs/dev/guides/mergepr_guide.md` に取り込みの道具 4 サブコマンドの使い方・出力形式・保証事項を追記し、`docs/dev/conventions/package_reference.md` の `internal/mergepr`・`cmd/mergepr` 行を更新する。
- [x] `cmd/mergepr/docs_test.go` の `placeholderValues` を新フラグのプレースホルダに対応させ、`TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`・`TestProtocolToolsAgree` を通す。

**完了基準:** `make test`・`make lint`・`make deadcode`・`make readme-sync-check` がすべてパスし、`cmd/mergepr/docs_test.go` の全ガードが通ること。

### PR-2 作成ポイント: cmd/mergepr CLI wiring, protocol bump, and docs

**対象ステップ**: 1-3

**推奨タイトル**: `feat(0115): cmd/mergepr に mergemain サブコマンドの CLI と利用ガイドを追加`

**レビュー観点**: 4 サブコマンドのフラグ定義と key: value 出力契約 / mergepr プロトコル番号更新と mergepr.md・runpr.md・ガイドの同期 / package_reference.md の整合

**実装モデル要件**: sonnet

**判定理由**: PR-1 で確定済みの internal/mergepr API を cmd/mergepr へ接続しプロトコル番号と文書を同期する定型変更であり、mkplan2.md ステップ 4 の opus・sonnet+opus-review トリガーに該当しないため。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1123
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

---

### フェーズ 2: コマンドの定義（`mergemain.md`）

設計書 3.8.2・3.8.4・3.9・3.10・7.4・8 章（優先順位 2）に基づき、`/mergemain` のコマンド定義、拡張事前確認ターゲット、許可規則、および信頼境界ガードを整備する。前提として、PR-2 マージ後に利用者が手で `make install-mergepr` を実行し、新サブコマンドと更新後プロトコル番号を持つ `mergepr` を入れ直してから `/mergemain` や更新後の `/mergepr`・`/runpr` を実行する（§3.1）。

#### ステップ 2-1: `Makefile` の `ext-preflight`、`.claude/commands/mergemain.md`、`CLAUDE.md` 更新

**対象ファイル:** `Makefile`, `.claude/commands/mergemain.md`（新規）, `.agents/skills/mergemain/SKILL.md`（新規）, `CLAUDE.md`

- [x] `Makefile` に `.PHONY` 付きの `ext-preflight` ターゲット（`$(call ext_preflight)` の呼び出し）を追加し、`CLAUDE.md` の `### Extension Commands` に `make ext-preflight` の説明を追記する（AC-26）。
- [x] `.claude/commands/mergemain.md` を英語で新規作成し、Antigravity CLI 向けの薄い入口 `.agents/skills/mergemain/SKILL.md`（本計画承認後に `main` へ入った `TestSkillsMatchCommands` と整合）を併せて作成する（設計書 3.9）。引数検証（AC-20）、`mergepr mergemain-begin`/`mergemain-merge` 呼び出し（`--protocol` は `mergepr` の現行値、allow 対象の 3 サブコマンドは前置や連結のない単独のコマンド呼び出しで記載し、行出力の可変値をフィールド名として解釈しない扱いも明記）、README 同期（`mktrans.md` 参照、`stopped` で停止、AC-24）、ADR 確認（番号・主題重複と食い違い記述で停止し `numreserve` を使わない、AC-25）、文書 `--protocol` 更新と `.md` 衝突解消（両側を保てなければ停止、AC-22）を記載する。続けて、`git commit --no-edit`、`make test`・`make lint`・条件付き `make ext-preflight`→`make ext-check`（AC-26）、`mergepr mergemain-push`（AC-27）、条件に応じた `mergepr mergemain-restore` と一時ファイルの作業ツリー外配置（AC-28）を定める。あわせて、日本語報告の必須項目（停止時の 11 種の理由と衝突ファイル一覧、解消成功時の変更ファイルごとの適用規則・解消内容の要約・新しい head・プロトコル番号変更時の入れ直し案内、AC-29）、AC-30 の 8 つの禁止事項と直接 `git merge`/`git reset`/`git push` 禁止、AC-31 の承認段落をすべて明記する。
- [x] `CLAUDE.md` の「Claude Commands」に `/mergemain` を追加し（AC-36 の `/mergemain` 部分）、「Tool Execution Safety」に AC-31 の 4 操作（`gh pr view`・`git fetch origin`・PR head ブランチの `git ls-remote`・PR head ブランチへの `git push`）の承認段落を追加する（AC-31）。

**完了基準:** `cmd/yt2column` の `TestExtensionDocumentation` を含む既存文書テストがすべて通ること。

#### ステップ 2-2: `.claude/settings.json` allow 規則、信頼境界ガード、未追跡ファイル・`review-records.sh` 実測確認

**対象ファイル:** `.claude/settings.json`, `cmd/yt2column/settings_test.go`, `cmd/mergepr/docs_test.go`, `internal/doctestutil/commands.go`, `internal/doctestutil/commands_test.go`, `docs/tasks/0115_chief_conflict_resolution/03_implementation_plan.md`（実測結果の追記）

- [x] `.claude/settings.json` の `permissions.allow` に `Bash(mergepr mergemain-begin *)`・`Bash(mergepr mergemain-merge *)`・`Bash(mergepr mergemain-push *)` の 3 つだけを追加する（`mergemain-restore` や `prepare`・`merge` は追加しない、設計書 3.10）。併せて、本計画承認後に `main` へ入った `cmd/yt2column/settings_test.go`（`TestSettingsAllowsOnlyVettedCommands`）の `unpromptedWords` に `mergepr` を登録する（許可するサブコマンドの限定は `cmd/mergepr/docs_test.go` の `TestSettingsAllowsOnlySafeMergemainSubcommands` が担う）。さらに、allow に無いだけのコマンドは auto モードでは確認なしに走りうるため、設計書 1.2 の人の答え（`mergemain-restore` は確認を出す）を守るよう `permissions.ask` に `Bash(mergepr mergemain-restore *)` を足し、`TestSettingsAsksBeforeMergemainRestore` で固定する（`cmd/chief/docs_test.go` の `TestSettingsAsksBeforeRemove` は、chief を名指す ask 規則が `Bash(chief remove *)` だけであることを確かめる形に改める）。
- [x] `cmd/mergepr/docs_test.go` と `internal/doctestutil/commands.go`（および `commands_test.go`）を拡張する：`definitionCalls` に `.claude/commands/mergemain.md`（4 サブコマンドすべて）を追加し、`TestDocumentedInvocationsParse` で `mergemain.md` の allow 対象 3 サブコマンド呼び出しが `doctestutil.PromptFree(call, toolWord)` を満たすことを検証する。さらに、共通ヘルパー `doctestutil.BashToolRuleMismatches`（単体テスト `TestBashToolRuleMismatches` は `internal/doctestutil/commands_test.go` に配置）を用いてワイルドカード・引用符付き表記・パス表記を含む逸脱規則を検出する `TestSettingsAllowsOnlySafeMergemainSubcommands` と、共通ヘルパー `doctestutil.FencedCommandBlocks`・`doctestutil.CommandArgs`（単体テストは `internal/doctestutil/commands_test.go`）を用いて `mergemain.md` の実行コマンドブロックから git グローバルオプションを読み飛ばして直接呼べる git のサブコマンドを `commit`・`diff` に限って `git merge`・`git reset`・`git push`・`git clean`・`git rebase` などの呼び出しを禁止する `TestMergemainNoDirectGitMutation`（および `TestDirectGitMutations`）を追加する（設計書 7.4）。各ガードは設定や定義を一時的に壊して失敗することを確認する。
- [x] 設計書 3.8.4・8 章の要求に従い、`make test`・`make lint`・`make ext-check`（準備がある場合）および `.pre-commit-config.yaml` の全フックを実行して作業ツリーに未追跡ファイルを残さないことを確かめ、また設計書付録 B・8 章の要求に従い、一時リポジトリまたはブランチの merge コミットに対して `.claude/commands/_lib/review-records.sh coverage` を実行して実測結果と対象コミットを本計画書（§5）に記録する。

**完了基準:** `make test`・`make lint`・`make deadcode`・`make readme-sync-check` がすべてパスし、追加したガードの壊して失敗確認と、未追跡ファイル・`review-records.sh` の実測記録が完了していること。

### PR-3 作成ポイント: mergemain command definition and trust-boundary guards

**対象ステップ**: 2-1 / 2-2

**推奨タイトル**: `feat(0115): mergemain コマンド定義と許可・直接呼び出しガードを追加`

**レビュー観点**: mergemain.md の手順・禁止事項・承認段落（AC-20〜AC-31） / settings.json allow 規則の最小範囲と逸脱検出ガード / git グローバルオプションを考慮した直接呼び出し禁止ガードと PromptFree 検証 / 未追跡ファイルと review-records.sh の実測記録

**実装モデル要件**: opus

**判定理由**: ステップ 2-1・2-2 が自動送信コマンドの承認境界（AC-30・AC-31）と、取り消せない merge と同一バイナリに対する settings.json の allow 規則および信頼境界ガード（allow 逸脱検出・git サブコマンド解析）を扱い mkplan.md ステップ 8 のセキュリティゲート条件に該当するため。Sonnet では承認範囲の過剰拡大、直接 git 呼び出しの迂回経路の残存、allow 規則のワイルドカード・引用符付き表記の検証漏れが生じうる。

**自動送信**: hold

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1137
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

---

### フェーズ 3: chief の検出と表示

設計書 3.1・3.2・3.6・3.10・8 章（優先順位 3）に基づき、PR 値の読み取り、コンフリクトの行・段階・表・出力、送り先の表 7 行化、`paneinput`、および人の後の入力の読み取りを実装する。

#### ステップ 3-1: PR 値読み取り・コンフリクト状態・段階型・表・出力

**対象ファイル:** `internal/chief/workspace.go`, `internal/chief/state.go`, `internal/chief/position.go`, `internal/chief/conflict.go`（新規）, `internal/chief/snapshot.go`, `internal/chief/table.go`, `internal/chief/output.go`, `internal/chief/construction_test.go`, `internal/chief/workspace_test.go`, `internal/chief/position_test.go`, `internal/chief/table_test.go`, `internal/chief/output_test.go`, `internal/chief/test_helpers.go`, 既存 `gh pr list` モックを含む `internal/chief/*_test.go`（7 ファイル：`world_test.go`・`snapshot_test.go`・`maintask_test.go`・`autosend_test.go`・`resetoffer_test.go`・`removal_test.go`・`send_test.go`）, `cmd/chief/main_test.go`

- [x] `internal/chief/workspace.go` に `Mergeable`（ゼロ値 `MergeableUnknown`）・`CommitID`（`ParseCommitID` のみで構築、`IsZero`/`String` を持つ）を追加し、`OpenPR`・`OpenPRs`・`parsePRList` に `mergeable`・`headRefOid`・`baseRefName`（`BaseIsMain` へ変換）・`isCrossRepository` の厳格検証を実装する（AC-01・AC-04）。`internal/chief/construction_test.go` の `guardedTypes` に `"CommitID": {"ParseCommitID"}` と各構築・代入経路のミューテーションテストを追加する。
- [x] `internal/chief/state.go` に `StateConflict`（`stateOrder` で `StateQuestion` の直後・`StateAwaitingReview` の直前に配置）と `classify` 分岐（実行中判定の後・レビュー待ちの前、`MergeableConflicting` のみ `StateConflict`、AC-02・AC-03・AC-16）を追加し、`internal/chief/conflict.go` に `ConflictPhase` 型（6 値、ゼロ値 `ConflictRecordsUnreadable`）と段階名変換を定義する。`snapshot.go` の `addRecords` では、記録ディレクトリを読めないとき `ConflictRecordsUnreadable`、それ以外は `ConflictUnsent` を付与する（フェーズ 3 時点では `/mergemain` 送信が存在しないため仮の記録解析は置かず、`conflictPhaseOf` はステップ 4-1 で `SendRecord` 拡張と一括導入する）。
- [x] `internal/chief/position.go`（`ProgressConflict`・`ProgressConflictUnresolved`、`urgencyOf`・`progressOf`）、`internal/chief/table.go`（`progressWord` の「コンフリクト」「解消できず」、`contentOf`）、`internal/chief/output.go`（`RowOutput.Conflict`、`PROutput.Head`、`progressName`）を更新する。
- [x] 既存の `gh pr list` モック（`internal/chief/` 8 ファイルと `cmd/chief/main_test.go:634`、および `test_helpers.go`）を新 4 メンバー付きに更新する（`testdata/gh/pr-list-open-867.json` は手編集せず旧形式拒否ケースとして残す）。`workspace_test.go`（AC-04 全拒否形と一覧全体拒否）、`position_test.go`（`TestUrgencyTable` 拡張含む）、`table_test.go`（`TestTableFixedWords` 拡張含む）、`output_test.go`（`TestOutputValueNames` 更新含む）を追加・更新する。

**完了基準:** `go test -tags test ./internal/chief/... ./cmd/chief/...` がすべて通り、`CommitID` 構築ガードと AC-01〜AC-04 の全テストが壊して失敗を確認済みであること。

#### ステップ 3-2: `paneinput`、送り先の表 7 行化、`LaterInput` と実行中 `/mergemain` の読み取り・進行表示

**対象ファイル:** `internal/chief/paneinput/paneinput.go`, `internal/chief/paneinput/paneinput_test.go`, `internal/chief/routing.go`, `internal/chief/routing_test.go`, `internal/chief/running.go`, `internal/chief/running_test.go`, `internal/chief/screen_transcript.go`, `internal/chief/screen_test.go`, `internal/chief/position.go`, `internal/chief/position_test.go`, `internal/chief/table.go`, `internal/chief/table_test.go`, `internal/chief/output.go`, `internal/chief/output_test.go`, `internal/chief/skiprecord_test.go`, `.claude/commands/_context.md`

- [x] `internal/chief/paneinput/paneinput.go` に `CommandMergemain`（`"mergemain"`）・`RouteMergemain`・`NewMergemain(fixprguard.PRNumber)` を追加し、`Name`・`ParseCommandKind`・`RouteKeys`・`Command.Text`・`Command.Parts` を更新する（AC-33）。既存の `paneinput_test.go` のテーブル駆動テスト（`TestCommandText`・`TestParseCommandKind`・`TestConstructorsRejectInvalid` 等）へ `/mergemain` の正常・異常ケースを追加する（`Makefile:364` の `make deadcode` は `-test` 付きで実行されるためテスト経由で到達する）。
- [x] `.claude/commands/_context.md` の Command routing 表に `mergemain` 行（`/mergemain`, `Claude Code`, `opus`）を追加して 7 行にし、行数記述と Claude Code 限定の例外を追記する。同時に `internal/chief/routing.go` の `routingRows` へ `"mergemain": paneinput.RouteMergemain` を追加し、`routing_test.go` を 7 行構成に合わせて更新する（`markers_parity_test.go` は変更せず回帰実行で確認、AC-32）。
- [x] `internal/chief/screen_transcript.go` の走査を `scanHumanInputs` に統合して実行中コマンドと `LaterInput`（6 値、ゼロ値 `LaterInputUnreadable`）を同時に返すようにし、`internal/chief/running.go` の `validRunningArg` に `CommandMergemain` の PR 番号形式を追加する。併せて `position.go`（`ProgressResolving` と `progressOf` の `CommandMergemain` 分岐）・`table.go`（`progressWord` の「解消中」）・`output.go`（`progressName`）を更新する。
- [x] `skiprecord_test.go` に `/mergemain` がリセット見送り記録の種類として拒否されるテストを追加する。`running_test.go`（`TestRunningCommandTextMatchesThePaneInput` 拡張含む）と `screen_test.go` で `RunningCommand`（`/mergemain <PR>`）と `LaterInput` 全 6 値をテストし、`position_test.go` で `ProgressResolving`、`table_test.go`（`TestTableFixedWords` 等）で日本語文字列「解消中」、`output_test.go`（`TestOutputValueNames`）で出力名をテストする（設計書 3.6、`CLAUDE.md`「Go Idioms」の日本語リテラル範囲を守る）。

**完了基準:** `go test -tags test ./internal/chief/...` がすべて通り、7 行の送り先の表・`paneinput`・`LaterInput` 全 6 値・`ProgressResolving`・`skiprecord` 拒否がテストで検証されていること。

#### ステップ 3-3: `chief` プロトコル番号更新・文書同期・実物 PR の `mergeable` 確認と採取サンプル追加

**対象ファイル:** `internal/chief/records.go`, `.claude/commands/chief.md`, `docs/dev/guides/chief_guide.md`, `docs/dev/chief_architecture.md`, `docs/dev/conventions/package_reference.md`, `cmd/chief/docs_test.go`, `internal/chief/testdata/gh/`（新 8 メンバー実物採取サンプル追加）, `internal/chief/testdata/README.md`, `internal/chief/testdata/gh/exits.txt`, `internal/chief/workspace_test.go`, `docs/tasks/0115_chief_conflict_resolution/03_implementation_plan.md`（実測結果の追記）

- [x] `internal/chief/records.go` の `Protocol` を `main` の値 + 1 に上げ、`.claude/commands/chief.md`・`docs/dev/guides/chief_guide.md` のプロトコル番号を合わせる。
- [x] `docs/dev/guides/chief_guide.md` に表の状態の文字列（「コンフリクト」「解消できず」「解消中」）と、送り先の表 7 行化に伴う「`make install-chief` で入れ直すまで古い chief は表を拒み自動送信が止まる」旨と、PR-6 がマージされるまではコンフリクトの行に待っている項目が出ないため人が `/mergemain <PR>` を直接打つ旨を追記する（設計書 3.10・8 章）。`docs/dev/chief_architecture.md` と `docs/dev/conventions/package_reference.md` の関連節を更新する。
- [x] 設計書 1.2・8 章の要求に従い、開いている実物 PR で（コンフリクト中の実物 PR での確かめは PR-5 の `hold` の点検に回す。§1「実装の際に決めたこと（2026-11-25）」） `gh pr list --state open --json headRefName,number,title,url,mergeable,headRefOid,baseRefName,isCrossRepository` と `gh pr view` の `mergeable` 値を比較して結果と日時を本計画書（§5）に記録する（`UNKNOWN` が継続する場合は利用者に報告して判断を仰ぐ）。併せて採取した 8 メンバー実物応答を `internal/chief/testdata/README.md` の匿名化規約（`TestSamplesFollowReplacementRules`）に従って `internal/chief/testdata/gh/` へ追加し、`workspace_test.go` で `parsePRList` 受理を検証する。

**完了基準:** `make test`・`make lint`・`make deadcode`・`make readme-sync-check` がすべてパスし、`cmd/chief/docs_test.go`（プロトコル番号・`chief_architecture.md` 整合）と `TestSamplesFollowReplacementRules` がすべて通ること。

### PR-4 作成ポイント: chief conflict detection, display, and routing

**対象ステップ**: 3-1 / 3-2 / 3-3

**推奨タイトル**: `feat(0115): chief にマージコンフリクト検出・表示と送り先の表 7 行化を追加`

**レビュー観点**: parsePRList の厳格検証（AC-04）と CommitID 構築ガード / StateConflict・ConflictPhase・LaterInput・paneinput の分類と表・JSON 出力 / _context.md 7 行化と routing.go の破壊的同時切替 / 実物 PR の mergeable 実測結果と匿名化採取サンプル

**実装モデル要件**: opus

**判定理由**: ステップ 3-3 が未検証の外部環境確認（実物 PR での gh pr list 対 gh pr view の mergeable 実測と UNKNOWN 継続時の判断、匿名化サンプル採取）を含み、ステップ 3-1・3-2 が信頼境界パーサ（AC-04）・CommitID AST ガード・_context.md 7 行化の破壊的切替と 9 ファイル移行を伴うため mkplan2.md ステップ 4 の opus 条件に該当する。Sonnet では一時的な mergeable 応答の確認不足や UNKNOWN 継続時の報告漏れ、testdata 匿名化漏れ、AC-04 境界入力や AST 経路の取りこぼしが生じうる。

**自動送信**: auto

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1155
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

---

### フェーズ 4: chief の送信

設計書 3.3・3.4・3.5・3.7・8 章（優先順位 4）に基づき、解消の送信の記録保持、自動送信、人が選ぶ送信、報告の控えと引き継ぎ、および文書仕上げを実装する。前提として、PR-4・PR-5・PR-6 の各マージ後に利用者が手で `make install-chief` と `/chief` 再起動を行い、更新したバイナリとコマンド定義のプロトコル番号を揃えてから運用・次段階の呼び出しへ進む（§3.1）。

#### ステップ 4-1: 解消の送信の記録と `conflictPhaseOf` の導入

**対象ファイル:** `internal/chief/records.go`, `internal/chief/send.go`, `internal/chief/conflict.go`, `internal/chief/snapshot.go`, `internal/chief/records_test.go`, `internal/chief/send_test.go`, `internal/chief/conflict_test.go`（新規）

- [x] `internal/chief/records.go` の `SendRecord` に `/mergemain` 専用の PR 番号と PR head（`CommitID`、§1 の `construction_test.go` 制約に沿った JSON 変換を実装）を追加し、`SendRecord.Kind` コメントを更新する。`sendEnd` に送信の到達点（`SentStep`、メンバー欠落を `StepUnknown` と区別できるポインタまたは省略可能型）を追加し、`Release` は到達点メンバーを書かずに記録する（設計書 3.3.1）。
- [x] `internal/chief/send.go` の送信終了時に到達点（`SentStep`）を `EndSend` へ渡すようにする。`internal/chief/records.go` の `cleanup` と `ClearRecords` を更新し、`/mergemain` の始まり・終わりの組および読めない始まりの記録を期間でも worktree ライフサイクルでも消さず保持する（他コマンドの記録は既存の 25 分・`ClearRecords` で消す挙動を保つ）。
- [x] `internal/chief/conflict.go` に `conflictPhaseOf` を実装し、`snapshot.go` の `addRecords` から接続して設計書 3.3.2 の全組み合わせと worktree 再作成後の段階・停止優先を完成させる（AC-08・AC-09・AC-14・AC-15）。
- [x] `records_test.go`・`send_test.go`・`conflict_test.go` に、`/mergemain` 記録の保持（消去しないこと）と他コマンド記録の消去、到達点の記録・欠落判別、記録が読めないときの安全側動作、`release` 後の段階遷移のテストを追加する。

**完了基準:** 設計書 3.3.2 の全表行と記録保持・読めない記録のテストがすべて通り、壊して失敗を確認済みであること。

#### ステップ 4-2: 送る前の控え（`keepPaneReport`）、解消の自動送信（`conflictSender`）と `tick` 組み込み

**対象ファイル:** `internal/chief/detail.go`, `internal/chief/detail_test.go`, `internal/chief/autosend.go`, `internal/chief/conflict.go`, `internal/chief/tick.go`, `internal/chief/output.go`, `internal/chief/records.go`, `.claude/commands/chief.md`, `docs/dev/guides/chief_guide.md`, `internal/chief/conflict_test.go`, `internal/chief/sim_test.go`

- [x] `internal/chief/detail.go` に `keepPaneReport` を追加し（`gh pr view` を呼ばず pane 報告の控えだけを作り、読めなければ控えを作らない、AC-12）、`detail_test.go` に単体テストを追加する。
- [x] `internal/chief/autosend.go` の共通部品（`checkRemote`・`quietness`・`ownerStill`・`checkOwner`・`locate`・`recheck` の枠組み）を `conflictSender` から再利用できるよう整理する（マージ後自動送信の挙動は変えない）。
- [x] `internal/chief/conflict.go` に `conflictSender` を実装する（設計書 3.4）：AC-05 の (1)〜(10) と再確認（持ち主・workspace 一意・`SentSince`）、`git show HEAD:` と `git show origin/main:` の `.claude/commands/mergemain.md` バイト比較、送り先の OpenCode 拒否（AC-34）、1 ラウンド 1 件制限（入力を打ったかもしれない送信で打ち切り）を実装する。あわせて、全 workspace のエージェント状態に基づく `/mergemain` 実行中・`RunningUnknown` 検出、同ラウンドのマージ後自動送信済み workspace 除外、送る前の `keepPaneReport` 呼び出し、および §1 確定の理由名出力を実装する（AC-05・AC-06・AC-07・AC-08・AC-10・AC-11・AC-12・AC-13・AC-34）。
- [x] `internal/chief/tick.go` で `LockCreation` 保持中の `autoSender.run` 直後に `conflictSender` を走らせ、結果を行と項目へ反映する `applyConflictSends` を追加し、`internal/chief/output.go` の `TickOutput` に `conflict_sends` を追加する（`snapshot` では送らない、AC-06）。`TickOutput` の契約変更に合わせて `internal/chief/records.go` の `Protocol` を `main` の値 + 1 に上げ、`.claude/commands/chief.md`・`docs/dev/guides/chief_guide.md` の `--protocol` を合わせる。
- [x] `conflict_test.go` に AC-05 の (1)〜(10) と再確認項目を 1 つずつ外す単体・セキュリティテストに加え、設計書 7.1 の境界ケース（同ラウンドのマージ後自動送信済み workspace 除外、`sent_then_failed` 後の 1 件打ち切り、行状態が `StateQuestion`/`StateNeedsCheck` でもエージェント状態から実行中 `/mergemain`・`RunningUnknown` を検出すること、ラウンド読み取り後に (6) が変わったら `recheck` で止まること）を追加し、`sim_test.go` に `tick` 結合テスト（`/clear`・`/model opus`・`/mergemain <PR>` のみの送信、次ラウンド非再送、head 変更後の再送）を追加する。

**完了基準:** `make test`・`make lint`・`make deadcode`・`make readme-sync-check` がすべてパスし、AC-05〜AC-11・AC-13・AC-15・AC-34 の検証と壊して失敗の確認が完了していること。

### PR-5 作成ポイント: chief conflict auto-send security gate and send-record persistence

**対象ステップ**: 4-1 / 4-2

**推奨タイトル**: `feat(0115): chief に mergemain の自動送信と送信記録の保持を追加`

**レビュー観点**: mergemain 送信記録の選択的保持（cleanup・ClearRecords）と到達点による段階遷移 / conflictSender の AC-05 全条件・再確認・1 ラウンド 1 件制限・OpenCode 拒否 / keepPaneReport の控え作成と TickOutput.conflict_sends / sim_test.go 結合テストとプロトコル番号同期

**実装モデル要件**: opus

**判定理由**: ステップ 4-1・4-2 が人に尋ねずに他エージェントへ入力を送る自動送信セキュリティゲート（AC-05 全条件と再確認・排他・1 件制限・OpenCode 拒否）と永続記録保持を担い mkplan.md ステップ 8 のセキュリティゲート条件に該当するため。Sonnet では再確認条件の抜け、sent_then_failed 後の打ち切り誤り、ClearRecords の選択保持漏れ、送る前の控え作成漏れが生じうる。

**自動送信**: hold

- [x] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [x] PR を作成した: https://github.com/isseis/yt2column/pull/1163
- [x] PR がマージされた
- [x] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

#### ステップ 4-3: 人が選ぶ送信（`PendingConflict`・`chief send`）と `CollectDetail` の親たどり引き継ぎ

**対象ファイル:** `internal/chief/pending.go`, `internal/chief/request.go`, `internal/chief/detail.go`, `internal/chief/workspace.go`, `internal/chief/output.go`, `internal/chief/test_helpers.go`, `cmd/chief/main.go`, `internal/chief/pending_test.go`, `internal/chief/send_test.go`, `internal/chief/detail_test.go`, `cmd/chief/main_test.go`

- [ ] `internal/chief/pending.go` と `internal/chief/output.go` に `PendingConflict`（`kindOrder` で `PendingMergeConfirmation` の直後・`PendingReview` の直前に配置）を追加する：項目 ID は（種類・workspace・PR 番号・head・「送る必要がある」か「解消できず」か）から生成し（head ごとに最大 2 回通知の 0088 AC-58 例外、設計書 1.2・3.5）、`ConflictSending` は項目を作らない。AC-18 の条件充足時のみ選択肢を示し、非充足時（`mergemain` 行が OpenCode の場合を含む、AC-34）は理由を項目へ載せ、`LaterInputAfter`/`LaterInputNoTaskInput` 時は会話消失の印を添える（AC-17）。コンフリクト行に `/runpr` 選択肢を出さない（設計書 3.5）。
- [ ] `internal/chief/request.go` と `cmd/chief/main.go` に `chief send --command mergemain --item <id>`（`--item` 必須、`--pr`/`--issue` 拒否）を追加する：workspace 排他下で行を再構築し、`origin/main` の取得確認（AC-05 (3)）を含む AC-18 全条件と項目 ID 一致を確かめ、通れば送信前に `keepPaneReport` を呼んでから（AC-12）送信・記録する（AC-18・AC-19・AC-34）。
- [ ] `internal/chief/workspace.go` の `GitReader` にコミット親取得（`git rev-list --parents -n 1 <commit>`）と PR head ブランチ限定取得（`git fetch --no-tags --no-prune origin +refs/heads/<branch>:refs/remotes/origin/<branch>`）を追加し、`internal/chief/test_helpers.go` の `readOnlyCommands` にその形を許可追加する。
- [ ] `internal/chief/detail.go` の `CollectDetail` に、`ReportRecordAllowed` 時限定の親たどり引き継ぎ（2 親 merge コミット・第 2 の親が `origin/main` の祖先・手元にない head は取得してから再読・上限 `maxDetailMergeHops = 10`・`d.record` やリセット判定や `snapshot` の `ReviewSkippable` には影響させない）を実装する（AC-12）。
- [ ] `pending_test.go`・`send_test.go`（既存 `sendWorld` を再利用）・`detail_test.go`・`cmd/chief/main_test.go` に各機能の単体テスト（通知回数、項目 ID 不一致拒否、OpenCode 送り先拒否、`/runpr` 非提示、手動送信前の `keepPaneReport` 実行、一時 git リポジトリでの引き継ぎ・非引き継ぎ・取得成功/失敗・上限・リセット非使用）を追加する。

**完了基準:** AC-12・AC-17〜AC-19・AC-34 の全単体・CLI テストが通り、壊して失敗を確認済みであること。

#### ステップ 4-4: `chief` プロトコル番号更新と文書仕上げ

**対象ファイル:** `internal/chief/records.go`, `.claude/commands/chief.md`, `docs/dev/guides/chief_guide.md`, `docs/dev/chief_architecture.md`, `docs/dev/conventions/package_reference.md`, `CLAUDE.md`, `cmd/chief/docs_test.go`

- [ ] `internal/chief/records.go` の `Protocol` を `main` の値 + 1 に上げ、`.claude/commands/chief.md`・`docs/dev/guides/chief_guide.md` の `--protocol` を合わせる。
- [ ] `.claude/commands/chief.md` にコンフリクト項目の示し方（理由・会話消失の印）と `chief send --command mergemain --item <項目 ID>` の呼び出しを追記し（AC-36）、`cmd/chief/docs_test.go` の `TestDocumentedInvocationsParse` が新呼び出しを受理することを確認する。
- [ ] `docs/dev/guides/chief_guide.md` に AC-35 の 4 点（状態の文字列・急ぎの度合い・状態の詳細、自動送信の条件、送り直しの規則、人が選んで送る選択肢）を完成させ、`CLAUDE.md` の「Claude Commands」の `/chief` 説明にコンフリクト解消を追記し（AC-36）、`docs/dev/chief_architecture.md` と `docs/dev/conventions/package_reference.md` を最終仕様へ同期する。
- [ ] `CLAUDE.md` の「Tool Execution Safety」の `/chief` 段落にある、`/chief` の起動が承認する読み取り操作の一覧に、ステップ 4-3 の PR の head ブランチの取得（`git fetch --no-tags --no-prune origin +refs/heads/<ブランチ>:refs/remotes/origin/<ブランチ>`）を加える。設計書 3.7 はこの取得を既存の承認の範囲内と読んでいるが、一覧の文言どおりに読むと範囲外になるため、一覧に明記して読み違いの余地をなくす。書き込み操作を承認しないという段落の記述は変えない。`.claude/commands/chief.md` に承認する操作の記述があれば、同じ取得を足す。
- [ ] `docs/dev/adr/0046-chief-of-staff.md` が要件定義書および実装結果と矛盾していないことを確認する（AC-37）。

**完了基準:** `make test`・`make lint`・`make deadcode`・`make readme-sync-check` がすべてパスすること。

### PR-6 作成ポイント: chief manual mergemain send, report inheritance, and final docs

**対象ステップ**: 4-3 / 4-4

**推奨タイトル**: `feat(0115): chief に人が選ぶ mergemain 送信と報告の控え引き継ぎおよび文書仕上げを追加`

**レビュー観点**: PendingConflict の項目 ID 拘束・通知回数上限（0088 AC-58 例外）・会話消失印 / chief send --command mergemain --item の AC-18 全条件再確認と送信前 keepPaneReport / CollectDetail の親をたどる引き継ぎと --no-tags --no-prune 取得で他処理・他 ref に副作用を出さない分離 / chief.md・chief_guide.md・CLAUDE.md・ADR-0046 の最終整合

**実装モデル要件**: sonnet+opus-review

**判定理由**: ステップ 4-3 が独立した高リスク・複雑な処理（PendingConflict 項目 ID 拘束と AC-18 の origin/main 取得・OpenCode 拒否の再確認、および CollectDetail の複数ホップ親たどり・限定 git fetch で他処理・他 ref に副作用を出さない分離）を含むため mkplan2.md ステップ 4 の sonnet+opus-review 条件に該当する。Sonnet では request.go での origin/main 取得や keepPaneReport 呼び出しの抜け、親控えの d.record への混入や fetch の --no-tags --no-prune 欠落が生じうるため Opus レビューで補強する。

**自動送信**: hold

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

## 3. 実装順序とマイルストーン

### 3.1 マイルストーン

| マイルストーン | 対象フェーズ | 主な成果物 | 完了条件 |
|---|---|---|---|
| M1: 取り込みの道具 | フェーズ 1（ステップ 1-1〜1-3 / PR-1・PR-2） | `internal/mergepr` 4 サブコマンドと `CLAUDE.md` 例外（PR-1）、`cmd/mergepr`、`mergepr` プロトコル番号更新、`mergepr_guide.md`（PR-2） | 道具の単体・結合・セキュリティ・文書ガードテストがすべて通る（PR-2 マージ後に利用者が手で `make install-mergepr` を実行） |
| M2: `/mergemain` 定義 | フェーズ 2（ステップ 2-1〜2-2 / PR-3） | `.claude/commands/mergemain.md`、`Makefile` `ext-preflight`、`.claude/settings.json` allow とガード、`CLAUDE.md` 承認・一覧、未追跡ファイルと `review-records.sh` 実測記録 | 人が直接 `/mergemain <PR>` を実行できる状態になり、文書・許可ガードが通る |
| M3: chief 検出・表示 | フェーズ 3（ステップ 3-1〜3-3 / PR-4） | `OpenPR` 4 メンバー、`CommitID` ガード、`StateConflict`・`ConflictPhase`・表・出力、`_context.md` 7 行化、`paneinput`・`LaterInput`、`chief` プロトコル番号更新、実物 PR `mergeable` 記録と実物採取サンプル | コンフリクト PR が chief の表と JSON に示され、全既存・新規テストと文書ガードが通る（PR-4 マージ後に利用者が手で `make install-chief` と `/chief` 再起動を実行） |
| M4: chief 自動・手動送信 | フェーズ 4（ステップ 4-1〜4-4 / PR-5・PR-6） | 送信記録の PR/head/到達点と保持、`conflictPhaseOf`、`keepPaneReport`、`conflictSender`、`PendingConflict`、`chief send --command mergemain --item`、`CollectDetail` 親たどり引き継ぎ、`chief.md`・`chief_guide.md`・`CLAUDE.md` 仕上げ | 全 37 件の受け入れ基準を満たし、グリーンゲートが通る（PR-5・PR-6 の各マージ後に利用者が手で `make install-chief` と `/chief` 再起動を実行） |

依存順序は M1（PR-1 → PR-2）→ M2（PR-3）→ M3（PR-4）→ M4（PR-5 → PR-6）である。PR-3 は PR-2 の `mergepr` 契約とプロトコル番号を前提とし、PR-5 は PR-4 の `OpenPR`・`ConflictPhase`・`paneinput`・`LaterInput` と PR-3 の `.claude/commands/mergemain.md` を前提とする。エージェントはインストール型ツールを入れ直さない（`CLAUDE.md`・ADR-0014）ため、ツールを変更する各 PR のマージ後に利用者が手動で入れ直しと再起動を行う前提とする。

- **PR-2 のマージ前:** PR-2 は `mergepr.md`・`runpr.md` の `--protocol` を上げるため、PR-2 自身の `/runpr` が呼ぶ `mergepr` は古い番号で断る。利用者が PR-2 のブランチから `make install-mergepr` で入れ直してからマージする（`CLAUDE.md`「Protocol numbers」）。
- **PR-2 マージ後（M2 開始前・`/mergepr` や `/runpr` の実行前）:** 利用者が手で `main` から `make install-mergepr` を実行し、新サブコマンドと更新後プロトコル番号を持つ `mergepr` を入れ直す（そのため PR-3 は `**自動送信**: hold` とする）。
- **PR-4 マージ後（PR-5 着手前）:** `_context.md` が 7 行になり `chief` のプロトコル番号も上がるため、利用者が手で `make install-chief` を実行して `/chief` を再起動し、ステップ 3-3 で記録した実物 PR の `mergeable` の確認結果を点検してから進む（そのため PR-5 は `**自動送信**: hold` とする）。
- **PR-5 マージ後（更新後 `/chief` の実行前・PR-6 着手前）:** `chief` のプロトコル番号と `TickOutput` 契約が変わるため、利用者が手で `make install-chief` を実行して `/chief` を再起動する（そのため PR-6 は `**自動送信**: hold` とする）。
- **PR-6 マージ後:** `chief` のプロトコル番号と `chief send --command mergemain` 契約が変わるため、利用者が手で `make install-chief` を実行して `/chief` を再起動する。

### 3.2 PR 構成

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 | 自動送信 |
|---|---|---|---|---|
| PR-1 | 1-1 / 1-2 | `internal/mergepr` の 4 サブコマンド判定・戻し・push 検証と単体・結合・セキュリティテスト、`CLAUDE.md` の読み取り例外 | opus | auto |
| PR-2 | 1-3 | `cmd/mergepr` CLI 口、`mergepr` プロトコル番号更新、ガイド・`package_reference.md` | sonnet | auto |
| PR-3 | 2-1 / 2-2 | `Makefile` の `ext-preflight`、`.claude/commands/mergemain.md`、`CLAUDE.md` 承認・一覧、`.claude/settings.json` allow と信頼境界ガード、未追跡ファイル・`review-records.sh` 実測記録 | opus | hold |
| PR-4 | 3-1 / 3-2 / 3-3 | `chief` の PR 値読み取り、`CommitID` ガード、`StateConflict`・`ConflictPhase`・表・出力、`_context.md` 7 行化、`paneinput`・`LaterInput`、`chief` プロトコル番号更新、実物 PR `mergeable` 記録と採取サンプル | opus | auto |
| PR-5 | 4-1 / 4-2 | 解消の送信の記録保持と `conflictPhaseOf`、`keepPaneReport`、`conflictSender`・`tick` 組み込み、`chief` プロトコル番号更新 | opus | hold |
| PR-6 | 4-3 / 4-4 | `PendingConflict`、`chief send --command mergemain --item`、`CollectDetail` 親たどり引き継ぎ、`chief` プロトコル番号更新と文書仕上げ | sonnet+opus-review | hold |

## 4. テスト戦略

### 単体テスト

- **`internal/mergepr`（`mergemain_test.go`）・`cmd/mergepr`（`main_test.go`）:** 既存の `test_helpers_git.go`・`test_helpers.go` を再利用し、一時 git リポジトリとローカル bare リモートで衝突分類（`.md`・両側追加・プロトコル行のみ・他行衝突・`_test.go`・modify/delete・3 種の名前変更・シンボリックリンク・バイナリ）、プロトコル番号更新（衝突あり/なし）、早送り禁止、無視ファイル保護、戻しの許可/拒否全条件（`MERGE_HEAD` 不一致・親順序不一致・`refs/remotes/origin/<branch>` 不一致を含む）、push の検証全項目（第 1 の親の `base_head`・再読み取り PR head 一致を含む）と 3 結果、ゼロ値の安全側動作を検証し、`cmd/mergepr/main_test.go` で行出力の改行・制御文字エスケープを検証する（設計書 7.1）。
- **`internal/chief`（`workspace_test.go`・`position_test.go`・`conflict_test.go`・`table_test.go`・`output_test.go`・`records_test.go`・`send_test.go`・`pending_test.go`・`detail_test.go`・`running_test.go`・`screen_test.go`・`skiprecord_test.go`・`routing_test.go`・`paneinput_test.go`）:** AC-04 の厳格 JSON 検証（欠落・`null`・型違い・大小文字違い・空白・head 形式・重複・1 要素拒否で一覧全体拒否）に加え、`classify`・`conflictPhaseOf`・`TestUrgencyTable`・`progressOf`・`TestTableFixedWords`・`TestOutputValueNames` を検証する。また、AC-05 の (1)〜(10) と再確認の 1 条件欠落ごとの拒否と境界ケース、`/mergemain` 記録の保持と他記録の消去、`LaterInput` 全 6 値、項目 ID 一致・`/runpr` 非提示・手動送信前の控え作成、および一時 git リポジトリでの `CollectDetail` 親たどり引き継ぎ（上限・取得成功/失敗・リセット非使用）を検証する（設計書 7.1）。日本語の状態の文字列リテラルを検証するテストは `CLAUDE.md`「Go Idioms」に従い `internal/chief/table_test.go` に限定する。

### 結合テスト

- **`internal/mergepr/mergemain_test.go`:** `mergemain-begin` → `mergemain-merge` → `git commit --no-edit` → `mergemain-push` の成功経路と、`mergemain-begin` → `mergemain-merge` → `mergemain-restore` の戻し経路を一連の呼び出しで検証する（設計書 7.2）。
- **`internal/chief/sim_test.go`:** 既存のシミュレーション基盤を使い、`tick` ラウンドでの自動送信（`/clear`・`/model opus`・`/mergemain <PR>` のみ）、同一 head での非再送、head 更新後の再送を検証する（設計書 7.2）。

### セキュリティテストと静的ガード

- **入力・送り先・別リポジトリの防御:** `paneinput_test.go`・`conflict_test.go`・`send_test.go`・`mergemain_test.go`・`cmd/mergepr/main_test.go` で、送る入力に画面/transcript/PR 題名が入らないこと（AC-33）、`mergemain` 行が OpenCode なら自動送信でも手動送信でも送らないこと（AC-34）、`isCrossRepository` 真の PR を chief と `mergepr` の両方で拒否すること、`mergemain-push` の改ざん防御（第 1 の親と `base_head`/再読み取り PR head の不一致・偽りの `main` 指定・別 `main` コミット・`refs/mergemain/<PR>/main` の履歴外書き換え・プロトコル値誤り・許可外ファイル変更・force push 禁止）、および `cmd/mergepr` 行出力での改行・制御文字によるフィールド偽装防止を検証する（設計書 7.3）。
- **型構築ガード:** `internal/chief/construction_test.go` の `guardedTypes` に `"CommitID": {"ParseCommitID"}` を追加し、複合リテラル・省略リテラル・ポインタリテラル・フィールド代入・メソッド内構築・クロージャ内構築の各経路のミューテーションテストで検証する（`test_helpers.go` も走査対象のため `ParseCommitID` を使う）。
- **文書・設定ガード:** `cmd/mergepr/docs_test.go`（`TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`（allow 対象 3 サブコマンドの `PromptFree` 検証含む）・`TestProtocolToolsAgree`・`TestSettingsAllowsOnlySafeMergemainSubcommands`・`TestMergemainNoDirectGitMutation`）、`cmd/chief/docs_test.go`（`TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`・`TestGuideProtocolMatchesTool`・`TestArchitectureValuesMatchTool`）、`cmd/yt2column/extension_docs_test.go`（`TestExtensionDocumentation`）、`internal/chief/routing_test.go`・`markers_parity_test.go` を実行する（設計書 7.4）。

### 後方互換性と既存テストの更新方針

- `_context.md` の Command routing 表 7 行化は、要件定義書 4.4 のとおり `internal/chief/routing.go` と同一 PR（フェーズ 3）で変更し、互換レイヤは設けない。
- 既存の `gh pr list` モック（§1 調査結果の `internal/chief/` 8 ファイルと `cmd/chief/main_test.go:634`）はフェーズ 3 で新 4 メンバーを補って更新し、既存テストの検証観点を落とさない。実物採取サンプル `testdata/gh/pr-list-open-867.json` は手編集せず旧形式拒否テストとして残し、新 8 メンバーの実物採取サンプルを匿名化規約に沿って追加する。

### テストヘルパー構成（`test_organization.md` 準拠）

- パッケージ横断の新規 `testutil/` ファイルは不要。`internal/doctestutil` は既存の `internal/doctestutil/commands.go`（`//go:build test`）と `internal/doctestutil/commands_test.go` をフェーズ 2 で拡張して `CommandArgs`・`FencedCommandBlocks`・`BashToolRuleMismatches` を提供する。`internal/chief` は既存の `internal/chief/test_helpers.go`（`//go:build test`）・`world_test.go`・`send_test.go`・`screen_test.go` を拡張し、`internal/mergepr` は既存の `internal/mergepr/test_helpers_git.go`（`//go:build test`）と `internal/mergepr/test_helpers.go` を拡張して新規 `internal/mergepr/mergemain_test.go` から利用する（`test_helpers_git.go:33-35` に従い `const Protocol = ...` を含むテスト用 Go ソース文字列は非 `_test.go` ヘルパーに置かない）。`//go:build test` 付きの非 `_test.go` ファイルを変更したフェーズでは、`go test -run '^$' -tags test ./...` と `make lint` の通過を完了ゲートで確認する。

## 5. リスク管理

| リスク | 影響 | 発生可能性 | 軽減策 |
|---|---|---|---|
| 実物 PR で `gh pr list --json mergeable` が `UNKNOWN` を返し続ける（設計書 1.2・H-01） | コンフリクト検出が遅れる、または検出できない | 中 | ステップ 3-3 で実物 PR に対し `gh pr list` と `gh pr view` を比較して結果を本節へ記録する。`UNKNOWN` は安全側（送らない）に倒れ、継続する場合は利用者と方針を確認する |
| merge コミット後の `review-records.sh coverage` の振る舞いが設計書付録 B の推論と食い違う（H-05） | `/runpr` 時のマージ質問での確認量が増減する | 低 | ステップ 2-2 で実際の merge コミットに対して `review-records.sh coverage` を実行し、結果を本節へ記録する |
| 検査・pre-commit フックが未追跡ファイルを残し、`mergemain-restore` が「ほかの変更があり戻さなかった」で止まる（設計書 3.8.4） | 自動解消失敗時の自動復旧が阻害される | 低 | ステップ 2-2 で `make test`・`make lint`・`make ext-check`・pre-commit フック実行後の `git status --porcelain` を確認し、結果を本節へ記録する |
| フェーズ 3 マージ後に古い `chief` が 7 行の `_context.md` を拒み、自動送信が止まる（設計書 3.10） | 入れ直しまでマージ後の自動送信も止まる | 高（仕様どおり） | ステップ 3-3 で `chief_guide.md` に明記し、PR-5 を `**自動送信**: hold` として、PR-4 マージ後に利用者が `make install-chief` と `/chief` 再起動および実物 PR 確認結果の点検を行ってから PR-5 へ進む |
| 並行ブランチとのプロトコル番号衝突（`mergepr`・`chief`） | マージ時に `mergepr prepare`/`merge` が停止する | 中 | 本計画では上げた番号を書かず常に `main` の値 + 1 とし、実装・マージ時点の `origin/main` に合わせる |
| `git merge-tree --write-tree` の挙動差や特殊衝突ケース（名前変更と内容変更、バイナリ、シンボリックリンク）の取りこぼし | 対象外の衝突を誤って解消・push する | 低 | ステップ 1-1・1-2 で設計書 7.1・7.3 の全特殊衝突ケースを一時 git リポジトリで網羅テストし、`mergemain-merge` と `mergemain-push` の両方で同一分類ロジックを使う |
| PR-4 で `StateConflict` が `StateAwaitingReview` より先に決まる一方、`PendingConflict` は PR-6 で入るため、PR-4〜PR-6 の間はコンフリクト中の PR に待っている項目（`/runpr` の選択肢も `/mergemain` の選択肢も）が出ない | 間の期間、コンフリクトの PR を人が表で見つけて手で `/mergemain` か対処をする必要がある | 中（仕様どおり） | 間の期間は表の「コンフリクト」を人が見て `/mergemain <PR>` を直接打つ。`chief_guide.md`（ステップ 3-3）にこの期間の扱いを書く |
| 実物のコンフリクト PR がすぐ用意できない、または PR-4 マージ後の手動入れ直し・実測確認（PR-5 の `hold`）で待ちが発生する（スケジュールリスク） | ステップ 3-3 の実測完了や PR-5 着手が遅れる | 中 | ステップ 3-3 で既存の開いているコンフリクト PR がない場合は、作業用の一時ドラフト PR を作って確認し即座に閉じることで外部の PR 待ちを避ける。PR-5 の `hold` 解除待ちの間は、本タスクとは独立した他タスクの作業を並行して進める |

### 実測記録（ステップ 2-2、2026-11-24）

- **`review-records.sh coverage` の実測（設計書付録 B・H-05）:**
  - 対象スクリプト: `origin/main` コミット `d224e1826a1a8a949405479d2ff25a4231077f88` の `.claude/commands/_lib/review-records.sh`。
  - 再現手順: 一時リポジトリで `origin/main` の初期コミット `M0`（`b74b0c5f30d4`）から分岐した `feature` ブランチに、`C1`（`1e0c4b213121`、`Local CLI reviewed: M0..C1`）と `C2`（`a2550ce74a44`、`Local CLI reviewed: C1..C2`）を積んだ（merge 前は `coverage none` が `ALL`）。`origin/main` を `M1`（`3f5af424f702`）へ進めた後、`git merge --no-ff --no-edit` で既定メッセージの merge コミット `MG`（`a622859febad`、`Merge commit '3f5af424f7027043f305aedf7a751f5f15960053' into feature`）を作成して実行した。
  - 実測結果: `coverage none <MG>` は `PARTIAL 2`（`C1` と `MG` が未見。古い merge base `M0` を始点とする見済み記録は merge base 移動後に数えられなくなり、`C1` を始点とする `C2` の見済み記録は有効のまま、`M1` は `origin/main` 到達のため除外）。`coverage <C2> <MG>` は `PARTIAL 1`（`MG` のみ未見）。
  - 判定: 設計書付録 B のコード読解による推論と一致することを確認した。
- **検査・pre-commit フック実行後の未追跡ファイル確認（設計書 3.8.4）:**
  - 対象基盤: `d224e1826a1a8a949405479d2ff25a4231077f88` にフェーズ 2 の変更を加えた作業ツリー。
  - 実行項目: `make test`・`make lint`・`make deadcode`・`make readme-sync-check`・`make ext-preflight`（本 worktree には `extension/node_modules` が未導入のため `make ext-preflight` が終了コード 1 で停止し、`make ext-check` は準備なしとして未実行、`make ext-install` は非実行）、および `pre-commit run --all-files`（`.pre-commit-config.yaml` の全 13 フックすべて通過）。
  - 結果: 実行後の `git status --porcelain --untracked-files=all` に意図しない未追跡ファイルは 0 件であり、`mergemain-restore` を阻害する未追跡ファイルが残らないことを確認した。

### 実測記録（ステップ 3-3、2026-11-25）

- **一覧の呼び出しの `mergeable` の実測（設計書 1.2・H-01）:**
  - 実行: 2026-11-25T05:39:07Z（UTC）に `gh pr list --state open --json headRefName,number,title,url,mergeable,headRefOid,baseRefName,isCrossRepository` を 1 回、2026-11-25T05:39:12Z までに開いている各 PR について `gh pr view <番号> --json number,mergeable,headRefOid` を実行した（gh 2.96.0）。
  - 結果: 開いていた PR は #1143（head `68e550907dec`）と #1144（head `cf1b86c57cb8`）の 2 件で、どちらも一覧と `gh pr view` が同じ head と `MERGEABLE` を返した。base はどちらも `main`、`isCrossRepository` はどちらも `false` だった。`UNKNOWN` は返らなかった。
  - 確かめなかったこと: コンフリクト中の PR がなかったので、一覧が `CONFLICTING` を返すかは確かめていない。§1「実装の際に決めたこと（2026-11-25）」のとおり、PR-4 のマージの後、PR-5 の `hold` の点検で、実際にコンフリクトした PR を chief の表で確かめる。
  - 採取したサンプル: #1144 の一覧の要素を、匿名化して `internal/chief/testdata/gh/pr-list-open-1144.json` に置いた（`internal/chief/testdata/README.md` の「タスク 0115 の実物の PR の確かめ」）。

## 6. 実装チェックリスト

- [x] PR-1 マージ済み（対象ステップ: 1-1 / 1-2）
- [x] PR-2 マージ済み（対象ステップ: 1-3）
- [x] PR-3 マージ済み（対象ステップ: 2-1 / 2-2）
- [x] PR-4 マージ済み（対象ステップ: 3-1 / 3-2 / 3-3）
- [x] PR-5 マージ済み（対象ステップ: 4-1 / 4-2）
- [ ] PR-6 マージ済み（対象ステップ: 4-3 / 4-4）

## 7. 受け入れ基準の検証

各受け入れ基準の検証種別は `test`（実行テスト）、`static`（ガードテスト・`make` ターゲット）、`manual`（散文のみの要件に対する PR レビュー読解）とする。混合要件は値・構造を `test` または `static` で、散文を `manual` で検証する。

| AC | 対象ステップ | 種別 | 検証箇所・内容 |
|---|---|---|---|
| AC-01 | 3-1, 3-3 | `test` | `internal/chief/workspace_test.go::TestParsePRList`（`tick`・`snapshot` 共通の `OpenPRs` が読む 4 メンバーの受理と、新 8 メンバー実物採取サンプルの受理を検証）・`internal/chief/snapshot_test.go::TestSnapshotConflictRow`（行と出力の `pr.head`） |
| AC-02 | 3-1 | `test` | `internal/chief/position_test.go::TestUrgencyTable`・`TestConflictUrgency`・`TestClassifyConflict`・`TestProgress`・`internal/chief/snapshot_test.go::TestSnapshotConflictRow`・`internal/chief/table_test.go::TestTableFixedWords`・`TestTableContent`（`CONFLICTING` 時の状態・急ぎの度合い、実行中/質問優先、`ConflictSending` の待つ 🟢 を検証） |
| AC-03 | 3-1, 4-2, 4-3 | `test` | `internal/chief/position_test.go::TestClassifyConflict`（`MERGEABLE`・`UNKNOWN` はレビュー待ち）・`internal/chief/conflict_test.go::TestConflictSenderSkipsNonConflicting`・`internal/chief/pending_test.go::TestPendingNoConflictForMergeableOrUnknown` |
| AC-04 | 3-1 | `test` | `internal/chief/workspace_test.go::TestParsePRList`・`TestParseCommitID`（欠落・`null`・型違い・大小文字・空白・head 形式・重複・旧 4 メンバーサンプル・1 要素拒否で一覧全体拒否を検証）・`internal/chief/snapshot_test.go::TestTakeSnapshotUnreadable`（拒んだ一覧で行が `StateNeedsCheck` になることを検証） |
| AC-05 | 4-2 | `test` | `internal/chief/conflict_test.go::TestConflictSenderConditions`（(1)〜(10) と再確認の各条件を 1 つずつ外して非送信・理由名を検証、境界ケースと全充足での送信を検証） |
| AC-06 | 4-2, 4-3 | `test` | `internal/chief/conflict_test.go::TestSnapshotDoesNotAutoSendConflict`・`internal/chief/pending_test.go::TestPendingConflictInSnapshot` |
| AC-07 | 4-2 | `test` | `internal/chief/conflict_test.go::TestConflictSenderPreexistingConflict`（前ラウンド未観測でも `tick` で送信することを検証） |
| AC-08 | 4-1, 4-2 | `test` | `internal/chief/records_test.go::TestMergemainRecordsPersistAcrossCleanupAndClear`・`internal/chief/conflict_test.go::TestConflictSenderOncePerHeadAndUnreadableRecord`・`internal/chief/sim_test.go::TestSimConflictAutoSend` |
| AC-09 | 4-1 | `test` | `internal/chief/send_test.go::TestSendMergemainUncertainStopsWorkspace`（結果不明時の非再送と停止を検証） |
| AC-10 | 3-1, 4-2 | `test` | `internal/chief/conflict_test.go::TestConflictSenderSkipsWhileRunprOrQuestion`（`/runpr` 実行中・質問中は送らず、終了後の `CONFLICTING` で送ることを検証） |
| AC-11 | 4-2 | `test` | `internal/chief/conflict_test.go::TestConflictSenderNeverTriggersFromScreenOrTranscript`（`CONFLICTING` 以外では画面・transcript 内容にかかわらず送らないことを検証） |
| AC-12 | 4-2, 4-3 | `test` | `internal/chief/detail_test.go::TestKeepPaneReport`・`TestCollectDetailInheritsReportAcrossMerge`・`internal/chief/send_test.go::TestSendMergemainKeepsPaneReport`（自動・手動送信前の pane 報告控えのみ作成・読めなければ控えなし・2 親 merge コミットの親たどり引き継ぎ・非引き継ぎ・取得成否・上限・リセット非使用を検証） |
| AC-13 | 4-2 | `test` | `internal/chief/sim_test.go::TestSimConflictAutoSend`（送信入力が `/clear`・`/model <モデル>`・`/mergemain <PR>` の順のみであることを検証） |
| AC-14 | 4-1, 4-3 | `test` | `internal/chief/conflict_test.go::TestConflictPhaseTransitions`・`internal/chief/pending_test.go::TestPendingConflictNotificationBudget`（`ConflictUnresolved`・`ConflictSending`・`ConflictEndedBeforeCommand` の段階・急ぎの度合い・詳細区別・head ごと最大 2 回通知を検証） |
| AC-15 | 4-1, 4-2 | `test` | `internal/chief/conflict_test.go::TestConflictSenderResendsAfterHeadChanges`・`internal/chief/sim_test.go::TestSimConflictAutoSend` |
| AC-16 | 3-1 | `test` | `internal/chief/position_test.go::TestClassifyConflict`（`MERGEABLE`・`UNKNOWN` の PR はレビュー待ちに戻る） |
| AC-17 | 4-3 | `test` | `internal/chief/pending_test.go::TestPendingConflictOfferAndLaterInputNote`（AC-18 充足時のみ選択肢提示、非充足時は理由表示、後の入力ありで会話消失印、`ConflictSending` は選択肢なし、`/runpr` 非提示を検証） |
| AC-18 | 4-3 | `test` | `internal/chief/send_test.go::TestSendMergemainByItem`・`cmd/chief/main_test.go::TestParseSendMergemain`（項目 ID 一致・`origin/main` 確認を含む AC-18 全条件再確認、AC-05 (6)(7)(10) 非要求、`--pr`/`--issue` 拒否を検証） |
| AC-19 | 4-1, 4-3 | `test` | `internal/chief/send_test.go::TestSendMergemainRecordsAndBlocksAutoSend`（手動送信も記録を作り同一 head の自動送信を抑止し、最後の送信で AC-14 を判定することを検証） |
| AC-20 | 1-1, 1-3, 2-1 | `test` / `manual` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainBeginRejectsInvalidPRArg`・`cmd/mergepr/main_test.go::TestMergemainArgValidation`。`manual`: `.claude/commands/mergemain.md` の引数検証手順のレビュー |
| AC-21 | 1-1, 2-1 | `test` / `manual` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainBeginPremisesAndNothingToTakeIn`・`TestMergemainMergeNoFastForward`（前提・`--no-ff`・衝突なし取り込み・取り込むものなしを検証）。`manual`: `.claude/commands/mergemain.md` の手順レビュー |
| AC-22 | 1-1, 1-2, 2-1 | `test` / `manual` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainProtocolUpdateAndRuleFacts`・`TestMergemainPushChecksProtocolValues`（プロトコル番号 `main` + 1 更新・README/ADR 事実出力・push 時値検証）。`manual`: `.claude/commands/mergemain.md` の README・ADR・`.md` 両側保持判断手順のレビュー |
| AC-23 | 1-1, 1-2 | `test` | `internal/mergepr/mergemain_test.go::TestMergemainConflictClassification`・`TestMergemainPushRejectsOutOfScope`（`.md`・プロトコル行のみ許可、他ファイル・他行衝突・`_test.go`・modify/delete・3 種の名前変更・シンボリックリンク・バイナリ拒否を検証） |
| AC-24 | 2-1 | `manual` | `.claude/commands/mergemain.md` が `mktrans.md` の `stopped` 時に停止・戻し手順へ進むことを PR レビューで確認（自然言語手順のみの要件） |
| AC-25 | 1-1, 2-1 | `test` / `manual` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainReportsADRConflictFacts`（`docs/dev/adr/` 変更と番号重複の検出を検証）。`manual`: `.claude/commands/mergemain.md` の ADR 重複・食い違い記述時の停止と `numreserve` 非実行を PR レビューで確認 |
| AC-26 | 1-1, 2-1 | `test` / `static` / `manual` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainExtCheckHint`（`ext-check` 対象パス判定を検証）。`static`: `cmd/yt2column/extension_docs_test.go::TestExtensionDocumentation`（`Makefile` の `ext-preflight` と `CLAUDE.md` の一致を検証）。`manual`: `.claude/commands/mergemain.md` の `make test`・`make lint`・`make ext-preflight`→`make ext-check` と依存未導入時の停止理由を PR レビューで確認 |
| AC-27 | 1-2, 2-1 | `test` / `manual` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainPushSuccessRejectedAndUnknown`（push が通ったとき・拒否時停止・結果不明時非戻しを検証）・`TestMergemainPushChecksParentsAndMain`（第 1 の親の `base_head`/再読み取り PR head 一致と第 2 の親の検証）・`TestMergemainPushRejectsNonHeadOrForce`（非 force push の引数を検証）。`manual`: `.claude/commands/mergemain.md` の push 結果分岐を PR レビューで確認 |
| AC-28 | 1-2, 2-1, 2-2 | `test` / `static` / `manual` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainRestoreSafeAndRefusesOtherChanges`（戻し後の状態、`MERGE_HEAD`・親順序・`refs/remotes/origin/<branch>` の不一致拒否、無視・未追跡ファイル保護、他変更時の非戻しを検証）。`static`: `cmd/mergepr/docs_test.go::TestSettingsAllowsOnlySafeMergemainSubcommands`（`mergemain-restore` を allow しないことを検証）・`TestSettingsAsksBeforeMergemainRestore`（auto モードでも確認を出す ask 規則を検証）。`manual`: `.claude/commands/mergemain.md` の戻し要否分岐（AC-20/21/27 結果不明時は戻さない）と一時ファイルの作業ツリー外配置を PR レビューで確認 |
| AC-29 | 1-3, 2-1 | `test` / `manual` | `test`: `cmd/mergepr/main_test.go::TestMergemainStopReasonsAndOutput`（道具が出す理由種別と戻すかの全種・衝突ファイル一覧・`detail` の出力と、改行・制御文字・Unicode の行区切りの 1 行エスケープを検証）・`TestMergemainOutput`（解消したときの `new_head`・`protocol_tool`・`readme_changed`・`adr_main_changed`・`adr_conflict_number`・`ext_check_hint` の出力を検証）・`TestEscapedField`（制御文字・行区切りと段落区切り・書式文字・無効な UTF-8 のエスケープを検証）。`manual`: `.claude/commands/mergemain.md` の日本語報告形式（停止時の 11 種の理由と衝突ファイル一覧、解消成功時の変更ファイルごとの適用規則・解消内容の要約・新しい head・プロトコル番号変更時の入れ直し案内）を PR レビューで確認 |
| AC-30 | 1-1, 1-2, 2-1, 2-2 | `test` / `static` / `manual` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainPRViewReadsNarrowFields`・`TestMergemainPushRejectsNonHeadOrForce`（狭い PR 読み取り・PR head 以外への push 拒否を検証）。`static`: `cmd/mergepr/docs_test.go::TestMergemainNoDirectGitMutation`（`mergemain.md` の実行コマンドブロックで直接呼べる git のサブコマンドを `commit`・`diff` に限り、`git merge`/`git reset`/`git push`/`git clean`/`git rebase` などそれ以外を禁止することを検証）・`TestMergemainCommandLinesAreListed`（実行コマンドブロックの各行を `mergepr mergemain-*`・`git commit`・`git diff` の形に限り、プレースホルダー外のシェル記号と `~~~` のフェンスを禁止することを検証）。`manual`: `.claude/commands/mergemain.md` の 8 つの禁止事項を PR レビューで確認 |
| AC-31 | 2-1 | `manual` | `CLAUDE.md` の「Tool Execution Safety」と `.claude/commands/mergemain.md` の承認段落に 4 操作（`gh pr view`・`git fetch origin`・PR head ブランチの `git ls-remote`・PR head ブランチへの `git push`）が揃っていることを PR レビューで確認（自然言語承認文のみの要件） |
| AC-32 | 3-2 | `test` / `manual` | `test`: `internal/chief/routing_test.go::TestRoutingReadsRepositoryContext`・`TestRoutingRowsMapToTheirKeys`・`TestRoutingRejects`・`TestReadRoutingFromOriginMain`・`TestRoutingMergemainRow`（7 行読み取り・欠落/不明行拒否・`mergemain` 行の `Claude Code` 読み取りと、OpenCode の `mergemain` 行も表として読むことを検証；併せて `markers_parity_test.go::TestMarkerReadingMatchesDecisionScript` の回帰通過を確認）。`manual`: `.claude/commands/_context.md` の「7 行」記述と Claude Code 限定の例外文を PR レビューで確認 |
| AC-33 | 3-2, 4-2 | `test` | `internal/chief/paneinput/paneinput_test.go::TestCommandText`・`TestParseCommandKind`・`TestConstructorsRejectInvalid`・`internal/chief/conflict_test.go::TestConflictSendInputUsesTypedCommandOnly`（型から組み立てた `/mergemain <PR>` のみ送信し、PR 題名・transcript・自由文・禁止コマンドを含まないことと chief 自身への非送信を検証） |
| AC-34 | 3-2, 4-2, 4-3 | `test` | `internal/chief/conflict_test.go::TestConflictSenderRefusesOpenCodeRoute`・`internal/chief/pending_test.go::TestPendingConflictRefusesOpenCodeRoute`・`internal/chief/send_test.go::TestSendMergemainRefusesOpenCodeRoute`（`mergemain` 行が OpenCode のとき自動送信でも手動送信でも送らず `route_not_claude` を示すことを検証） |
| AC-35 | 3-3, 4-4 | `static` / `manual` | `static`: `cmd/chief/docs_test.go::TestGuideProtocolMatchesTool`（`chief_guide.md` のプロトコル番号一致を検証）。`manual`: `docs/dev/guides/chief_guide.md` の 4 項目（状態/急ぎ/詳細、自動送信条件、送り直し規則、人が選ぶ選択肢）を PR レビューで確認 |
| AC-36 | 2-1, 3-3, 4-4 | `static` / `manual` | `static`: `cmd/chief/docs_test.go::TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`・`cmd/mergepr/docs_test.go::TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`（`chief.md`・`mergemain.md` のプロトコル番号とコマンド呼び出し構文・`PromptFree` を検証）。`manual`: `CLAUDE.md` の `/mergemain`・`/chief` 説明と `chief.md` の選択肢表示手順を PR レビューで確認 |
| AC-37 | 4-4 | `manual` | `docs/dev/adr/0046-chief-of-staff.md`（コミット `d74acaeb` で 0115 の決定反映済み）が要件定義書および最終実装と一致していることを PR レビューで確認（自然言語 ADR 文書のみの要件） |

## 8. 完了基準

- **機能:** AC-01〜AC-37 のすべてが実装され、§7 の全検証を通過していること。
- **品質:** すべての PR でグリーンゲート（`make test`・`make lint`・`make deadcode`・`make readme-sync-check`、対象変更時は `make ext-check`）がパスし、新設・更新テストの壊して失敗の確認がコミットメッセージに記録されていること。
- **セキュリティ:** `CommitID` の構築ガード、`.claude/settings.json` の allow 範囲ガード、`mergemain.md` の直接 git 操作禁止ガード、`mergemain-push` の範囲・親・プロトコル値検証、および `chief` の入力範囲・送り先検証がすべて有効であること。
- **文書:** `mergemain.md`・`_context.md`・`chief.md`・`mergepr.md`・`runpr.md`・`chief_guide.md`・`mergepr_guide.md`・`chief_architecture.md`・`package_reference.md`・`CLAUDE.md` がコードと同期し、文書ガードがすべて通ること。

## 9. 次のステップ

1. 本実装計画書（ステータス: `draft`）の PR レビューを受け、指摘があれば修正する。
2. レビュー承認後、`/runpr <PR>` で本計画書を `approved` に更新してマージする。
3. マージ後、`/runplan 0115` を実行し、PR-1 から順に実装を進める。

## 変更履歴

- 2026-11-24 承認（isseis、#1096）
- 2026-11-24 セッション中の再承認：`mergemain-push` と `mergemain-begin` の確かめに、remote が `origin` だけであることと `GH_REPO` がないことを足した（isseis、#1114）
- 2026-11-24 セッション中の再承認：テキストの合わせ以外で衝突を解消する設定を取り込みの道具の止まる条件にした（isseis、#1114）
- 2026-11-25 セッション中の再承認：実物の PR での `mergeable` の確かめを開いている PR だけで行い、コンフリクト中の PR での確かめを PR-5 の `hold` の点検に回した（isseis、#1155）
