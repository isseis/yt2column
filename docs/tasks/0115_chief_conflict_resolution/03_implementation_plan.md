# 実装計画書：chief は PR のマージコンフリクトを見つけて `/mergemain` で解消させ、解消できなければ人の判断を求める

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-23 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 実装概要

### 目的

[01_requirements.md](01_requirements.md)（以下、要件定義書）の F-001〜F-007（AC-01〜AC-37）と [02_architecture.md](02_architecture.md)（以下、設計書）に基づき、次の 4 点を順に実装する。

1. `mergepr` への取り込みの道具（`mergemain-begin`・`mergemain-merge`・`mergemain-restore`・`mergemain-push`）の追加。
2. コマンド定義 `.claude/commands/mergemain.md`、`Makefile` の `ext-preflight` ターゲット、`.claude/settings.json` の allow 規則、および文書のガードテストの追加。
3. `chief` の PR 値読み取り拡張、コンフリクトの行とコンフリクトの段階の分類・表・出力、送り先の表 7 行化、`paneinput` と人の後の入力の読み取りの追加。
4. `chief` の解消の送信の記録保持、解消の自動送信、人が選ぶ送信、送る前の報告の控えと `chief detail` の親たどり引き継ぎの追加。

### 実装原則

- **設計書の順と境界を守る。** フェーズの順は設計書 8 章の実装の優先順位 1〜4 に一致させる。決定的な判定と確かめは `internal/mergepr` に置き、送る入力の組み立ては `internal/chief/paneinput` の型に閉じる。
- **既存の部品とテスト基盤を再利用する。** `internal/mergepr/protocolcheck.go` のプロトコル番号読み取り（`acceptedLine`・`commitChange`）と `gitReader`、`internal/chief` の送信・記録・確かめ直し（`sendCommand`・`autoSender` の共通処理・`quietness`・`readReport`・`ReadRouting`）と既存テスト世界（`world_test.go`・`test_helpers.go`）を再利用する。
- **プロトコル番号は常に `main` の値 + 1 にする。** `mergepr` と `chief` の契約を変える PR では、`Protocol` 定数と対応するコマンド定義・ガイドの `--protocol` を同じ PR で `main` の値 + 1 に揃える。エージェントはインストール型ツールをインストールもビルドもしない（`CLAUDE.md`・ADR-0014）。
- **計画書で確定する細部。** 設計書が計画書へ委ねた 3 つの細部を次のとおり確定する。
  - **取り込みの道具の行出力（設計書 4.1）。** 既存の `cmd/mergepr/main.go` の `key=value` 形式に合わせ、1 行目を `status=ok` または `status=stop reason=<reason> restore=<needed|not_needed|unknown>` とし、2 行目以降に `base_head`・`head_branch`・`main_head`・`conflict`（`<class> <path>`）・`protocol_tool`（`<tool> <new_protocol>`）・`readme_changed`・`adr_main_changed`・`adr_conflict_number`・`ext_check_hint`・`new_head`・`detail` の各行を出力する。
  - **解消の自動送信の理由の種類（設計書 3.4.3）。** 設計書 3.4.3 の表の英語名に加え、作成排他を取れないときの `creation_locked` と、同ラウンドのマージ後自動送信が同じ workspace へ送ったときの `autosent_same_round` を定義する。
  - **報告の控えの引き継ぎ上限（設計書 3.7）。** `CollectDetail` が第 1 の親をたどる回数上限を `maxDetailMergeHops = 10` とする。
- **壊して失敗を確かめる。** 新設・更新するテストとガードは、対応する分岐・検証・制約を一時的に外した状態で失敗することを実装時に確かめ、コミットメッセージに記録する（`CLAUDE.md`「Testing Strategy」）。

### 計画の際に決めたこと（2026-11-23）

- **フェーズ 3（検出と表示）とフェーズ 4（送信）を別 PR に分ける。** 検出・表示の PR と送信・記録保持・引き継ぎの PR を分離し、各 PR のレビュー規模とリスクを抑える。`chief` のプロトコル番号は各 PR で `main` の値 + 1 に上げる。人の答え：別 PR に分ける。
  - ADR：なし。PR の分割と実装順序の選択で本タスク内に閉じる。
- **`mergepr` と `fixprguard` の共通パッケージは作らず、`internal/mergepr` 内に実装する。** push の引数形（`--no-follow-tags --recurse-submodules=no -- <remote> <commit>:refs/heads/<branch>`）のみ `internal/fixprguard/worktree.go:640-644` に合わせ、`internal/fixprguard` には変更を入れない。人の答え：共有せず `internal/mergepr` 内に実装する。
  - ADR：なし。パッケージ内部の実装構成で本タスク内に閉じる。
- **`make ext-check` の準備確認（AC-26）は `Makefile` に `ext-preflight` ターゲットを追加して行う。** 既存の `define ext_preflight`（`Makefile:407-413`）をそのまま呼ぶ `ext-preflight` ターゲットを `.PHONY` に加え、`CLAUDE.md` の `### Extension Commands` にも追記する（`cmd/yt2column/extension_docs_test.go:71-80` の `TestExtensionDocumentation` と整合させる）。`/mergemain` は `make ext-check` の前に `make ext-preflight` を実行し、失敗を「検査の準備がない」として「検査の失敗」と区別する。人の答え：`Makefile` に `ext-preflight` ターゲットを追加する。
  - ADR：なし。既存マクロを 1 ターゲットとして露出するだけで本タスク内に閉じる。
- **`CommitID` 型は新規箇所（`OpenPR.Head` とコンフリクト解消関連）だけに適用する。** 既存の `commitID` 正規表現を使う 3 か所（`internal/chief/detail.go:129`、`internal/chief/workspace.go:134`、`internal/chief/create.go:483`）のシグネチャは変えない。`CommitID` の構築は `internal/chief/construction_test.go` の `guardedTypes` で `ParseCommitID` に限定する。人の答え：新規箇所だけに適用する。
  - ADR：なし。型適用の範囲の選択で本タスク内に閉じる。

### 既存コード調査結果

以下はコミット `d74acaeb`（`origin/main`）で確かめた事実である。

- **`internal/mergepr` と `cmd/mergepr`:**
  - `internal/mergepr/mergepr.go:57` の `prViewFields` は `body` を含むため、`mergemain-begin`・`mergemain-push` は `state,baseRefName,headRefName,headRefOid,isCrossRepository` のみの別定数を使う（AC-30）。
  - `internal/mergepr/mergepr.go:63` の `Protocol` は現在 `3`、`mainRefspec` は既存 `prepare`/`merge` と同形。`internal/mergepr/protocolcheck.go:392` に `acceptedLine`、`:407-415` に `cmdrunner.GitHardening()` 付きの `gitReader`、`:533-547` に `listPRCommits`、`:851-871` に `commitChange` があり、そのまま再利用できる。
  - 実行環境の `git`（`git version 2.50.1 (Apple Git-155)`）で `git merge-tree --write-tree -z --no-messages <branch1> <branch2>` の存在を `git merge-tree -h` で確認した。作業ツリーやインデックスを汚さずに 2 つの親の自動マージ結果の木と衝突一覧を取得できるため、3.8.3 の検証に利用できる。
  - `cmd/mergepr/main.go:205-218` の `shapes` テーブルでサブコマンドごとのフラグを定義している。
  - `cmd/mergepr/docs_test.go:31-34` の `definitionCalls`、`:61-83` の `TestDocumentedProtocolMatches`、`:86-149` の `placeholderValues`・`TestDocumentedInvocationsParse` を拡張し、`.claude/settings.json` allow 規則ガードは `cmd/numreserve/docs_test.go:42-146`、直接呼び出し禁止ガードは `cmd/chief/docs_test.go:302-310` と同じ `internal/doctestutil` パターンで追加できる。
- **`Makefile`・`.pre-commit-config.yaml`・`CLAUDE.md`:**
  - `Makefile:407-413` に `define ext_preflight`（Node/npm 版確認 + `extension/node_modules` 存在確認）があり、`ext-preflight` ターゲットとして公開すれば `TestExtensionDocumentation`（`cmd/yt2column/extension_docs_test.go:71-80`）が `Makefile` と `CLAUDE.md` の `### Extension Commands` の一致を自動検証する。
  - `.pre-commit-config.yaml` のフック（`go-test`, `readme-sync`, `go-tidy`, `golangci-lint`, `go-vet-integration`, `trailing-whitespace`, `end-of-file-fixer`, `check-yaml`, `check-added-large-files`, `check-merge-conflict`, `detect-private-key`, `no-commit-to-branch`, `gofumpt`）と `make test`/`make lint`/`make ext-check` が未追跡ファイルを残さないかの実測確認はフェーズ 2（ステップ 2-2）で実施して記録する。
- **`internal/chief/paneinput` と `CommandKind` の全分岐箇所:**
  - `internal/chief/paneinput/paneinput.go:92-177` に `CommandKind`・`commandKinds`・`ParseCommandKind`・`RouteKey`・`RouteKeys`、`:322-421` に `Command`・`NewRunpr`・`Text`・`Parts` がある。`CommandMergemain`・`RouteMergemain`・`NewMergemain(fixprguard.PRNumber)` を追加する。
  - `CommandKind` を参照・分岐する全箇所を横断検索した結果：`internal/chief/running.go:50-64`（`validRunningArg`、追加要）、`internal/chief/position.go:247-270`（`taskDocumentOf`、`/mergemain` は作業項目の表示を変えないため既存分岐で足りる）、`internal/chief/position.go:280-307`（`progressOf`、`ProgressResolving` を追加要）、`internal/chief/screen_transcript.go:309-345`（`readRunningCommand`・`taskCommandOf`、`scanHumanInputs` へ統合して `LaterInput` も返す）、`internal/chief/skiprecord.go:84-90,109-147`（`commandKinds`・`checkRanCommand`、`/mergemain` は追加せず拒否をテストで確認）、`internal/chief/records.go:689-695`（`SendRecord.Kind` のコメント更新要）、`internal/chief/routing.go:27-34`（`routingRows` に `"mergemain": paneinput.RouteMergemain` を追加要）。
- **`internal/chief` の PR 読み取り・状態・表示・出力:**
  - `internal/chief/workspace.go:18-73,142-148` の `OpenPR`・`parsePRList`・`OpenPRs` に 4 メンバーを足す。既存の `gh pr list` 応答モック・サンプルは `internal/chief/testdata/gh/pr-list-open-867.json`（および `testdata/README.md`）・`workspace_test.go`・`world_test.go`・`snapshot_test.go`（`openPRJSON`）・`maintask_test.go`・`autosend_test.go`・`resetoffer_test.go`・`removal_test.go`・`send_test.go` にあり、フェーズ 3 ですべて 4 メンバー付きに更新する。
  - **`CommitID` の構築・変更経路の制限（具象型の選択）:** 設計書 3.1・3.11 に従い `CommitID` は新しいリーフパッケージへ切り出さず `internal/chief` に置く。同一パッケージ内の構築・書き換え経路（複合リテラル `CommitID{...}`、型省略リテラル `[]CommitID{{...}}`、ポインタリテラル `&CommitID{...}`、フィールド代入 `c.hex = ...`、メソッド・クロージャ内構築）は既存の `internal/chief/construction_test.go:20-32` の `guardedTypes` に `"CommitID": {"ParseCommitID"}` を追加して AST/型ガードで禁じ、各経路のミューテーションテストを同ファイルへ追加する。ゼロ値 `CommitID{}` はコンストラクタ外で作った無効値（空文字）として `IsZero()` で判別し、送信や記録のキーに使わない。
  - `internal/chief/state.go:13-62,496-554` に `WorkspaceState`・`stateOrder`・`classify`、`internal/chief/position.go:58-79,207-226,280-307` に `Progress`・`urgencyOf`・`progressOf`、`internal/chief/table.go:204-223,308-341` に `progressWord`・`contentOf`、`internal/chief/output.go` に `RowOutput`・`PROutput`・`TickOutput`・`PendingOutput`・`progressName` がある。`internal/chief/output_test.go:21-56` の `TestOutputValueNames` も `StateConflict` と新規 `Progress` 3 値に合わせて更新する。
  - `internal/chief/records.go:29` の `Protocol` は現在 `6`。`docs/dev/chief_architecture.md` のサブコマンド表・終了コード表・状態ディレクトリ節は `cmd/chief/docs_test.go:520-570` の `TestArchitectureValuesMatchTool` で検証されており、仕様更新に合わせて同期する。
- **`internal/chief` の記録・送信・詳細:**
  - `internal/chief/records.go:680-704,739-755,830-851,942-973,1443-1477` の `SendRecord`・`sendEnd`・`cleanup`・`ClearRecords`、および `internal/chief/send.go:447-560` を拡張する。
  - `internal/chief/autosend.go:318-332,406-447,781-835` の `checkRemote`・`quietness`・`ownerStill`・`checkOwner`・`locate`・`recheck` の仕組みを `conflictSender` と共有し、`internal/chief/tick.go:76-89,121-214` で `autoSender.run` の直後に実行する。
  - `internal/chief/pending.go:36-74,130-214`・`internal/chief/request.go:23-141,221-292,425-479`・`cmd/chief/main.go:71-96,244-258` で `PendingConflict` と `chief send --command mergemain --item <id>` を実装する。
  - `internal/chief/detail.go:452-631` に `keepPaneReport` と `CollectDetail`（`ReportRecordAllowed` 時のみ）の親たどり引き継ぎを足す。`internal/chief/test_helpers.go:219` の `readOnlyCommands` に `rev-list --parents` と PR head ブランチ取得（`fetch --no-tags --no-prune origin`）の形を許可追加する。
- **ADR の現在状態（AC-37）:**
  - `docs/dev/adr/0046-chief-of-staff.md:39-49,73-116` と `docs/dev/adr/0014-deterministic-checks-in-go.md:31,40,69` は、設計書承認 PR（`#1051`）で既に 0115 の決定が反映済みであることをコミット `d74acaeb` で確認した。本計画では新たな ADR 変更は発生せず、完了時に要件定義書との整合を再確認する。
- **`.claude/commands/_context.md` と関連文書:**
  - `.claude/commands/_context.md:157-167` の Command routing 表は現在 6 行。フェーズ 3 で `mergemain` 行（`Claude Code`, `opus`）を追加して 7 行とし、Claude Code 限定の例外を記載する（なお、ADR-0055 / issue #1072 の `runplan-*` 行キー改名は本タスクの対象外であり、既存 6 行のキー名は変えない）。
  - `docs/dev/conventions/package_reference.md` の `internal/mergepr`・`cmd/mergepr`・`internal/chief`・`internal/chief/paneinput` の記述を、各フェーズの公開 API・責務変更に合わせて更新する。

### 申し送りの扱い

- `design_handoff.md` の全項目（H-01〜H-12）は、すべて設計書 付録 A に方針が記録されていることを確認した。本計画では設計書付録 A のうち実装時に検証・記録が必要な項目（H-01 の実物 PR での `mergeable` 確認、H-05 の `review-records.sh coverage` 確認）を該当ステップに組み込む。
- `implementation_handoff.md` は本タスクディレクトリに存在しないため、`I-NN` 項目はない。

## 2. 実装ステップ

### フェーズ 1: 取り込みの道具（`mergepr`）

設計書 3.8・4.1・5.2・8 章（優先順位 1）に基づき、`mergepr` に 4 つのサブコマンドと決定的な判定・検証を実装する。

#### ステップ 1-1: `internal/mergepr` の前提確認・取り込み・衝突分類・プロトコル番号更新

**対象ファイル:** `internal/mergepr/mergemain.go`（新規）, `internal/mergepr/mergemain_test.go`（新規）

- [ ] `PathConflictClass`・`MergemainStop`・`RestoreNeed`（ゼロ値はそれぞれ `PathConflictOutOfScope`・`MergemainStopUnknown`・`RestoreUnknown`）と、狭い PR 読み取り（`state,baseRefName,headRefName,headRefOid,isCrossRepository` のみ、本文・コメント・レビューを読まない）を実装する（AC-21・AC-30）。
- [ ] `MergemainBegin` を実装する：PR 番号（10 進数字のみ）と PR 状態・`base=main`・非別リポジトリの検証、作業ツリーのクリーン確認（未追跡ファイル含む）、`main`（`refs/mergemain/<PR>/main`）と PR head ブランチ（`refs/remotes/origin/<branch>`）の明示 refspec での取得、HEAD・PR head・remote-tracking ref の一致確認、取り込むものがない判定（`MergemainStopNothingToTakeIn`）を行う（AC-20・AC-21）。
- [ ] `MergemainMerge` を実装する：引数と `refs/mergemain/<PR>/main` の照合、無視ファイル上書きの事前検知、`--no-ff --no-commit` マージ、ステージ種別・バイナリ判定・名前変更検出・`acceptedLine` による衝突分類、ブランチが merge base から変更した道具（衝突あり・なし両方）のプロトコル番号を `main` の値 + 1 へ更新、および規則適用の事実（README 変更・ADR 変更/番号重複・`ext-check` 見込み・番号変更した道具一覧）の収集を行う（AC-22・AC-23）。
- [ ] 一時 git リポジトリを用いた単体テストを `internal/mergepr/mergemain_test.go` に追加し、各分類（`.md`・両側追加・プロトコル行のみ・他行衝突・`_test.go`・modify/delete・3 種の名前変更・シンボリックリンク・バイナリ）、早送り禁止、無視ファイル上書き拒否、ゼロ値安全側、および各検証を外したときのテスト失敗を確認する。

**完了基準:** `MergemainBegin`・`MergemainMerge` の単体テストがすべて通り、検証無効化時の失敗が確認されていること。

#### ステップ 1-2: `internal/mergepr` の戻し検証と push 検証・実行

**対象ファイル:** `internal/mergepr/mergemain.go`, `internal/mergepr/mergemain_push.go`（新規）, `internal/mergepr/mergemain_restore.go`（新規）, `internal/mergepr/mergemain_test.go`

- [ ] `MergemainRestore` を実装する（設計書 3.8.4）：戻してよい 2 状態（取り込み途中、または未 push の merge コミット）と戻さない 6 条件（ブランチ不一致、HEAD 不一致、コミット後変更、取り込み途中の対象外変更、基準 head 追跡パスを塞ぐ無視ファイル、未追跡ファイル存在）を検証し、安全なときのみ `git reset --hard <base_head>` で戻して戻し後の状態を検証する（`git clean`・`git merge --abort` は使わない、AC-28）。
- [ ] `MergemainPush` を実装する（設計書 3.8.3）：PR 再読み取り（`main` 禁止・非別リポジトリ・ブランチ一致）、merge コミットの 2 親検証（第 2 の親と `refs/mergemain/<PR>/main` の一致、および同呼び出し内で再取得した `FETCH_HEAD` の祖先確認）、`git merge-tree --write-tree` による衝突分類と木差分検証（許可パス・プロトコル番号 `main` の値 + 1 の全道具検証）、作業ツリー確認、`git ls-remote` での `origin` ブランチ確認、push URL と fetch URL の一致確認を行ったうえで、force なし・`--no-follow-tags --recurse-submodules=no` で push し、失敗時は `git ls-remote` で 3 結果（通った・拒まれた・結果不明）を判定する（AC-27・AC-30）。
- [ ] `internal/mergepr/mergemain_test.go` に戻し・push の単体テスト、セキュリティテスト（設計書 7.1・7.3 の全拒否ケース）、および `begin`→`merge`→`push` と `begin`→`merge`→`restore` の結合テストを追加し、各検証を外したときのテスト失敗を確認する。

**完了基準:** `go test -tags test ./internal/mergepr/...` がすべて通り、設計書 7.1・7.2・7.3 の取り込みの道具に関する全ケースと壊して失敗の確認が揃っていること。

#### ステップ 1-3: `cmd/mergepr` CLI 口、プロトコル番号更新、ガイド・関連文書更新

**対象ファイル:** `cmd/mergepr/main.go`, `cmd/mergepr/main_test.go`, `cmd/mergepr/docs_test.go`, `internal/mergepr/mergepr.go`, `.claude/commands/mergepr.md`, `.claude/commands/runpr.md`, `docs/dev/guides/mergepr_guide.md`, `CLAUDE.md`, `docs/dev/conventions/package_reference.md`

- [ ] `cmd/mergepr/main.go` の `shapes` とディスパッチに `mergemain-begin`・`mergemain-merge`・`mergemain-restore`・`mergemain-push` を追加し、§1 で確定した `key=value` 形式の出力と終了コードを実装する。`cmd/mergepr/main_test.go` にフラグ検証・出力形式テストを追加する。
- [ ] `internal/mergepr/mergepr.go` の `Protocol` を `main` の値 + 1 に上げ、`.claude/commands/mergepr.md`・`.claude/commands/runpr.md`・`docs/dev/guides/mergepr_guide.md` の `--protocol` を合わせる。
- [ ] `docs/dev/guides/mergepr_guide.md` に取り込みの道具 4 サブコマンドの使い方・出力形式・保証事項を追記し、`CLAUDE.md` の「Declare, don't infer」に設計書 3.8.5 の取り込みの道具の読み取り例外（範囲と境界）を追記し、`docs/dev/conventions/package_reference.md` の `internal/mergepr`・`cmd/mergepr` 行を更新する。
- [ ] `cmd/mergepr/docs_test.go` の `placeholderValues` を新フラグのプレースホルダに対応させ、`TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`・`TestProtocolToolsAgree` を通す。

**完了基準:** `make test`・`make lint` がパスし、`cmd/mergepr/docs_test.go` の全ガードが通ること。

---

### フェーズ 2: コマンドの定義（`mergemain.md`）

設計書 3.8.2・3.8.4・3.9・3.10・7.4・8 章（優先順位 2）に基づき、`/mergemain` のコマンド定義、拡張事前確認ターゲット、許可規則、および信頼境界ガードを整備する。

#### ステップ 2-1: `Makefile` の `ext-preflight`、`.claude/commands/mergemain.md`、`CLAUDE.md` 更新

**対象ファイル:** `Makefile`, `.claude/commands/mergemain.md`（新規）, `CLAUDE.md`

- [ ] `Makefile` に `.PHONY` 付きの `ext-preflight` ターゲット（`$(call ext_preflight)` の呼び出し）を追加し、`CLAUDE.md` の `### Extension Commands` に `make ext-preflight` の説明を追記する（AC-26）。
- [ ] `.claude/commands/mergemain.md` を英語で新規作成する（設計書 3.9）。引数検証（AC-20）、`mergepr mergemain-begin`/`mergemain-merge` 呼び出し（`--protocol` は `mergepr` の現行値）、README 同期（`mktrans.md` 参照、`stopped` で停止、AC-24）、ADR 確認（番号・主題重複と食い違い記述で停止し `numreserve` を使わない、AC-25）、文書 `--protocol` 更新と `.md` 衝突解消（両側を保てなければ停止、AC-22）、`git commit --no-edit`、`make test`・`make lint`・条件付き `make ext-preflight`→`make ext-check`（AC-26）、`mergepr mergemain-push`（AC-27）、条件に応じた `mergepr mergemain-restore`（AC-28）、日本語報告の必須項目と 11 種の理由（AC-29）、AC-30 の 8 つの禁止事項と直接 `git merge`/`git reset`/`git push` 禁止、AC-31 の承認段落をすべて明記する。
- [ ] `CLAUDE.md` の「Claude Commands」に `/mergemain` を追加し（AC-36 の `/mergemain` 部分）、「Tool Execution Safety」に AC-31 の 4 操作（`gh pr view`・`git fetch origin`・PR head ブランチの `git ls-remote`・PR head ブランチへの `git push`）の承認段落を追加する（AC-31）。

**完了基準:** `cmd/yt2column` の `TestExtensionDocumentation` を含む既存文書テストがすべて通ること。

#### ステップ 2-2: `.claude/settings.json` allow 規則、信頼境界ガード、未追跡ファイル・`review-records.sh` 実測確認

**対象ファイル:** `.claude/settings.json`, `cmd/mergepr/docs_test.go`, `docs/tasks/0115_chief_conflict_resolution/03_implementation_plan.md`（実測結果の追記）

- [ ] `.claude/settings.json` の `permissions.allow` に `Bash(mergepr mergemain-begin *)`・`Bash(mergepr mergemain-merge *)`・`Bash(mergepr mergemain-push *)` の 3 つだけを追加する（`mergemain-restore` や `prepare`・`merge` は追加しない、設計書 3.10）。
- [ ] `cmd/mergepr/docs_test.go` を拡張する：`definitionCalls` に `.claude/commands/mergemain.md`（4 サブコマンドすべて）を追加し、`.claude/settings.json` の `mergepr` allow 規則が上記 3 サブコマンドのみであることを検証する `TestSettingsAllowsOnlySafeMergemainSubcommands` と、`mergemain.md` が `git merge`・`git reset`・`git push` を直接呼ばないことを検証する `TestMergemainNoDirectGitMutation` を追加する（設計書 7.4）。各ガードは設定や定義を一時的に壊して失敗することを確認する。
- [ ] 設計書 3.8.4・8 章の要求に従い、`make test`・`make lint`・`make ext-check`（準備がある場合）および `.pre-commit-config.yaml` の全フックを実行して作業ツリーに未追跡ファイルを残さないことを確かめ、また設計書付録 B・8 章の要求に従い、一時リポジトリまたはブランチの merge コミットに対して `.claude/commands/_lib/review-records.sh coverage` を実行して実測結果と対象コミットを本計画書（§5 または実施記録）に記録する。

**完了基準:** `make test`・`make lint` がパスし、追加した 2 つのガードの壊して失敗確認と、未追跡ファイル・`review-records.sh` の実測記録が完了していること。

---

### フェーズ 3: chief の検出と表示

設計書 3.1・3.2・3.6・3.10・8 章（優先順位 3）に基づき、PR 値の読み取り、コンフリクトの行・段階・表・出力、送り先の表 7 行化、`paneinput`、および人の後の入力の読み取りを実装する。

#### ステップ 3-1: PR 値読み取り・状態・コンフリクトの段階・表・出力

**対象ファイル:** `internal/chief/workspace.go`, `internal/chief/state.go`, `internal/chief/position.go`, `internal/chief/conflict.go`（新規）, `internal/chief/snapshot.go`, `internal/chief/table.go`, `internal/chief/output.go`, `internal/chief/construction_test.go`, `internal/chief/workspace_test.go`, `internal/chief/position_test.go`, `internal/chief/conflict_test.go`（新規）, `internal/chief/table_test.go`, `internal/chief/output_test.go`, `internal/chief/test_helpers.go`, `internal/chief/testdata/gh/pr-list-open-867.json`, `internal/chief/testdata/README.md`, 既存 `gh pr list` モックを含む `internal/chief/*_test.go`（§1 調査結果列挙の 7 ファイル）

- [ ] `internal/chief/workspace.go` に `Mergeable`（ゼロ値 `MergeableUnknown`）・`CommitID`（`ParseCommitID` のみで構築、`IsZero`/`String` を持つ）を追加し、`OpenPR`・`OpenPRs`・`parsePRList` に `mergeable`・`headRefOid`・`baseRefName`（`BaseIsMain` へ変換）・`isCrossRepository` の厳格検証を実装する（AC-01・AC-04）。`internal/chief/construction_test.go` の `guardedTypes` に `"CommitID": {"ParseCommitID"}` と各構築・代入経路のミューテーションテストを追加する。
- [ ] `internal/chief/state.go` に `StateConflict` と `classify` 分岐（実行中判定の後・レビュー待ちの前、`MergeableConflicting` のみ `StateConflict`、AC-02・AC-03・AC-16）を追加し、`internal/chief/conflict.go` に `ConflictPhase`（ゼロ値 `ConflictRecordsUnreadable`）と `conflictPhaseOf`（設計書 3.2.2・3.3.2、フェーズ 3 時点では既存記録構造の上で未送信・送信中・読めない等を判定し、フェーズ 4 で `SendRecord` の PR/Head・`Step` 拡張と接続する）を実装して `snapshot.go` の `addRecords` で付与する。
- [ ] `internal/chief/position.go`（`ProgressConflict`・`ProgressConflictUnresolved`・`ProgressResolving`、`urgencyOf`・`progressOf`）、`internal/chief/table.go`（`progressWord` の「コンフリクト」「解消できず」「解消中」、`contentOf`）、`internal/chief/output.go`（`RowOutput.Conflict`、`PROutput.Head`、`progressName`）を更新する。
- [ ] 既存の `gh pr list` テストデータ・モック（`testdata/gh/pr-list-open-867.json`・`testdata/README.md`・`test_helpers.go`・`world_test.go`・`snapshot_test.go`・`maintask_test.go`・`autosend_test.go`・`resetoffer_test.go`・`removal_test.go`・`send_test.go`）を更新し、`workspace_test.go`（AC-04 全拒否形と一覧全体拒否）、`position_test.go`、`conflict_test.go`、`table_test.go`、`output_test.go`（`TestOutputValueNames`）を追加・更新する。

**完了基準:** `go test -tags test ./internal/chief/...` がすべて通り、`CommitID` 構築ガードと AC-01〜AC-04 の全テストが壊して失敗を確認済みであること。

#### ステップ 3-2: `paneinput`、送り先の表 7 行化、`LaterInput` と実行中 `/mergemain` の読み取り

**対象ファイル:** `internal/chief/paneinput/paneinput.go`, `internal/chief/paneinput/paneinput_test.go`, `internal/chief/routing.go`, `internal/chief/routing_test.go`, `internal/chief/running.go`, `internal/chief/running_test.go`, `internal/chief/screen_transcript.go`, `internal/chief/screen_transcript_test.go`, `internal/chief/skiprecord_test.go`, `internal/chief/markers_parity_test.go`, `.claude/commands/_context.md`

- [ ] `internal/chief/paneinput/paneinput.go` に `CommandMergemain`（`"mergemain"`）・`RouteMergemain`・`NewMergemain(fixprguard.PRNumber)` を追加し、`Text`・`Parts`・`RouteKeys` を更新する（AC-33）。`paneinput_test.go` に正常・異常（ゼロ PR 番号・不正種別）テストを追加する。
- [ ] `.claude/commands/_context.md` の Command routing 表に `mergemain` 行（`/mergemain`, `Claude Code`, `opus`）を追加して 7 行にし、行数記述と Claude Code 限定の例外を追記する。同時に `internal/chief/routing.go` の `routingRows` へ `"mergemain": paneinput.RouteMergemain` を追加し、`routing_test.go` と `markers_parity_test.go` を 7 行構成に合わせて更新する（AC-32）。
- [ ] `internal/chief/screen_transcript.go` の走査を `scanHumanInputs` に統合して実行中コマンドと `LaterInput`（6 値、ゼロ値 `LaterInputUnreadable`）を同時に返すようにし、`internal/chief/running.go` の `validRunningArg` に `CommandMergemain` の PR 番号形式を追加する。`skiprecord_test.go` に `/mergemain` がリセット見送り記録の種類として拒否されるテストを追加し、`running_test.go`・`screen_transcript_test.go` で `LaterInput` 全 6 値と実行中 `/mergemain` の「解消中」表示をテストする（設計書 3.6）。

**完了基準:** `go test -tags test ./internal/chief/...` がすべて通り、7 行の送り先の表・`LaterInput` 全 6 値・`skiprecord` 拒否がテストで検証されていること。

#### ステップ 3-3: `chief` プロトコル番号更新・文書同期・実物 PR の `mergeable` 確認

**対象ファイル:** `internal/chief/records.go`, `.claude/commands/chief.md`, `docs/dev/guides/chief_guide.md`, `docs/dev/chief_architecture.md`, `docs/dev/conventions/package_reference.md`, `cmd/chief/docs_test.go`, `docs/tasks/0115_chief_conflict_resolution/03_implementation_plan.md`（実測結果の追記）

- [ ] `internal/chief/records.go` の `Protocol` を `main` の値 + 1 に上げ、`.claude/commands/chief.md`・`docs/dev/guides/chief_guide.md` のプロトコル番号を合わせる。
- [ ] `docs/dev/guides/chief_guide.md` に表の状態文字列（「コンフリクト」「解消できず」「解消中」）と、送り先の表 7 行化に伴う「`make install-chief` で入れ直すまで古い chief は表を拒み自動送信が止まる」旨を追記する（設計書 3.10・8 章）。`docs/dev/chief_architecture.md` と `docs/dev/conventions/package_reference.md` の関連節を更新する。
- [ ] 設計書 1.2・8 章の要求に従い、開いている実物 PR とコンフリクト中の実物 PR（または本 PR 自身と一時的な衝突 PR）で `gh pr list --json number,mergeable,headRefOid,baseRefName,isCrossRepository,headRefName` と `gh pr view` の `mergeable` 値を比較し、結果と日時を本計画書（§5）に記録する（`UNKNOWN` が継続する場合は利用者に報告して判断を仰ぐ）。

**完了基準:** `make test`・`make lint` がパスし、`cmd/chief/docs_test.go`（プロトコル番号・`chief_architecture.md` 整合）がすべて通ること。

---

### フェーズ 4: chief の送信

設計書 3.3・3.4・3.5・3.7・8 章（優先順位 4）に基づき、解消の送信の記録保持、自動送信、人が選ぶ送信、報告の控えと引き継ぎ、および文書仕上げを実装する。

#### ステップ 4-1: 解消の送信の記録と `conflictPhaseOf` の完全接続

**対象ファイル:** `internal/chief/records.go`, `internal/chief/send.go`, `internal/chief/conflict.go`, `internal/chief/records_test.go`, `internal/chief/send_test.go`, `internal/chief/conflict_test.go`

- [ ] `internal/chief/records.go` の `SendRecord` に `/mergemain` 専用の PR 番号と PR head（`CommitID`）を追加し、`SendRecord.Kind` コメントを更新する。`sendEnd` に送信の到達点（`SentStep`、メンバー欠落を `StepUnknown` と区別できるポインタまたは省略可能型）を追加し、`Release` は到達点メンバーを書かずに記録する（設計書 3.3.1）。
- [ ] `internal/chief/send.go` の送信終了時に到達点（`SentStep`）を `EndSend` へ渡すようにする。`internal/chief/records.go` の `cleanup` と `ClearRecords` を更新し、`/mergemain` の始まり・終わりの組および読めない始まりの記録を期間でも worktree ライフサイクルでも消さず保持する（他コマンドの記録は既存の 25 分・`ClearRecords` で消す挙動を保つ）。
- [ ] `internal/chief/conflict.go` の `conflictPhaseOf` を拡張した送信の記録（PR・head・到達点・結果）に接続し、設計書 3.3.2 の全組み合わせと worktree 再作成後の段階・停止優先を完成させる（AC-08・AC-09・AC-14・AC-15）。
- [ ] `records_test.go`・`send_test.go`・`conflict_test.go` に、`/mergemain` 記録の不消去と他コマンド記録の消去、到達点の記録・欠落判別、読めない記録時の安全側、`release` 後の段階遷移のテストを追加する。

**完了基準:** 設計書 3.3.2 の全表行と記録保持・読めない記録のテストがすべて通り、壊して失敗を確認済みであること。

#### ステップ 4-2: 解消の自動送信（`conflictSender`）と `tick` 組み込み

**対象ファイル:** `internal/chief/autosend.go`, `internal/chief/conflict.go`, `internal/chief/tick.go`, `internal/chief/output.go`, `internal/chief/conflict_test.go`, `internal/chief/sim_test.go`

- [ ] `internal/chief/autosend.go` の共通部品（`checkRemote`・`quietness`・`ownerStill`・`checkOwner`・`locate`・`recheck` の枠組み）を `conflictSender` から再利用できるよう整理する（マージ後自動送信の挙動は変えない）。
- [ ] `internal/chief/conflict.go` に `conflictSender` を実装する（設計書 3.4）：AC-05 の (1)〜(10) と再確認（持ち主・workspace 一意・`SentSince`）、`git show HEAD:` と `git show origin/main:` の `.claude/commands/mergemain.md` バイト比較、送り先の OpenCode 拒否（AC-34）、1 ラウンド 1 件制限（入力を打ったかもしれない送信で打ち切り）、全 workspace のエージェント状態に基づく `/mergemain` 実行中・`RunningUnknown` 検出、同ラウンドのマージ後自動送信済み workspace 除外、送る前の `keepPaneReport` 呼び出し（ステップ 4-3 と接続）、および §1 確定の理由名出力を実装する（AC-05・AC-06・AC-07・AC-08・AC-10・AC-11・AC-13・AC-34）。
- [ ] `internal/chief/tick.go` で `LockCreation` 保持中の `autoSender.run` 直後に `conflictSender` を走らせ、結果を行と項目へ反映する `applyConflictSends` を追加し、`internal/chief/output.go` の `TickOutput` に `conflict_sends` を追加する（`snapshot` では送らない、AC-06）。
- [ ] `conflict_test.go` に AC-05 の (1)〜(10) と再確認項目を 1 つずつ外す単体テスト・セキュリティテストを追加し、`sim_test.go` に `tick` 結合テスト（`/clear`・`/model opus`・`/mergemain <PR>` のみの送信、次ラウンド非再送、head 変更後の再送）を追加する。

**完了基準:** `conflict_test.go`・`sim_test.go` を含む全テストが通り、AC-05〜AC-11・AC-13・AC-15・AC-34 の検証と壊して失敗の確認が完了していること。

#### ステップ 4-3: 人が選ぶ送信（`PendingConflict`・`chief send`）と報告の控え・親たどり引き継ぎ

**対象ファイル:** `internal/chief/pending.go`, `internal/chief/request.go`, `internal/chief/detail.go`, `internal/chief/workspace.go`, `internal/chief/output.go`, `internal/chief/test_helpers.go`, `cmd/chief/main.go`, `internal/chief/pending_test.go`, `internal/chief/request_test.go`, `internal/chief/detail_test.go`, `cmd/chief/main_test.go`

- [ ] `internal/chief/pending.go` と `internal/chief/output.go` に `PendingConflict` を追加する：項目 ID は（種類・workspace・PR 番号・head・「送る必要がある」か「解消できず」か）から生成し（head ごとに最大 2 回通知の 0088 AC-58 例外、設計書 1.2・3.5）、`ConflictSending` は項目を作らない。AC-18 の条件充足時のみ選択肢を示し、非充足時は理由を項目へ載せ、`LaterInputAfter`/`LaterInputNoTaskInput` 時は会話消失の印を添える（AC-17）。コンフリクト行に `/runpr` 選択肢を出さない（設計書 3.5）。
- [ ] `internal/chief/request.go` と `cmd/chief/main.go` に `chief send --command mergemain --item <id>`（`--item` 必須、`--pr`/`--issue` 拒否）を追加し、workspace 排他下で行を再構築して項目 ID 一致と AC-18 全条件を確かめたときのみ送信・記録する（AC-18・AC-19）。
- [ ] `internal/chief/workspace.go` の `GitReader` にコミット親取得（`git rev-list --parents -n 1 <commit>`）と PR head ブランチ限定取得（`git fetch --no-tags --no-prune origin +refs/heads/<branch>:refs/remotes/origin/<branch>`）を追加し、`internal/chief/test_helpers.go` の `readOnlyCommands` にその形を許可追加する。
- [ ] `internal/chief/detail.go` に `keepPaneReport`（`gh pr view` を呼ばず pane 報告の控えだけを作る）と、`CollectDetail` の `ReportRecordAllowed` 時限定の親たどり引き継ぎ（2 親 merge コミット・第 2 の親が `origin/main` の祖先・手元にない head は取得してから再読・上限 `maxDetailMergeHops = 10`・`d.record` やリセット判定や `snapshot` の `ReviewSkippable` には影響させない）を実装する（AC-12）。
- [ ] `pending_test.go`・`request_test.go`・`detail_test.go`・`cmd/chief/main_test.go` に各機能の単体テスト（通知回数、項目 ID 不一致拒否、`/runpr` 非提示、控え作成と非作成、一時 git リポジトリでの引き継ぎ・非引き継ぎ・取得成功/失敗・上限・リセット非使用）を追加する。

**完了基準:** AC-12・AC-17〜AC-19 の全単体・CLI テストが通り、壊して失敗を確認済みであること。

#### ステップ 4-4: `chief` プロトコル番号更新と文書仕上げ

**対象ファイル:** `internal/chief/records.go`, `.claude/commands/chief.md`, `docs/dev/guides/chief_guide.md`, `docs/dev/chief_architecture.md`, `docs/dev/conventions/package_reference.md`, `CLAUDE.md`, `cmd/chief/docs_test.go`

- [ ] `internal/chief/records.go` の `Protocol` を `main` の値 + 1 に上げ、`.claude/commands/chief.md`・`docs/dev/guides/chief_guide.md` の `--protocol` を合わせる。
- [ ] `.claude/commands/chief.md` にコンフリクト項目の示し方（理由・会話消失の印）と `chief send --command mergemain --item <項目 ID>` の呼び出しを追記し（AC-36）、`cmd/chief/docs_test.go` の `TestDocumentedInvocationsParse` が新呼び出しを受理することを確認する。
- [ ] `docs/dev/guides/chief_guide.md` に AC-35 の 4 点（状態文字列・急ぎの度合い・状態の詳細、自動送信の条件、送り直しの規則、人が選んで送る選択肢）を完成させ、`CLAUDE.md` の「Claude Commands」の `/chief` 説明にコンフリクト解消を追記し（AC-36）、`docs/dev/chief_architecture.md` と `docs/dev/conventions/package_reference.md` を最終仕様へ同期する。
- [ ] `docs/dev/adr/0046-chief-of-staff.md` が要件定義書および実装結果と矛盾していないことを確認する（AC-37）。

**完了基準:** `make test`・`make lint`・`make deadcode`・`make readme-sync-check` がすべてパスすること。

## 3. 実装順序とマイルストーン

### 3.1 マイルストーン

| マイルストーン | 対象フェーズ | 主な成果物 | 完了条件 |
|---|---|---|---|
| M1: 取り込みの道具 | フェーズ 1（ステップ 1-1〜1-3） | `internal/mergepr` 4 サブコマンド、`cmd/mergepr`、`mergepr` プロトコル番号更新、`mergepr_guide.md`、`CLAUDE.md` 例外 | 道具の単体・結合・セキュリティ・文書ガードテストがすべて通る |
| M2: `/mergemain` 定義 | フェーズ 2（ステップ 2-1〜2-2） | `.claude/commands/mergemain.md`、`Makefile` `ext-preflight`、`.claude/settings.json` allow とガード、`CLAUDE.md` 承認・一覧、未追跡ファイルと `review-records.sh` 実測記録 | 人が直接 `/mergemain <PR>` を実行できる状態になり、文書・許可ガードが通る |
| M3: chief 検出・表示 | フェーズ 3（ステップ 3-1〜3-3） | `OpenPR` 4 メンバー、`CommitID` ガード、`StateConflict`・`ConflictPhase`・表・出力、`_context.md` 7 行化、`paneinput`・`LaterInput`、`chief` プロトコル番号更新、実物 PR `mergeable` 記録 | コンフリクト PR が chief の表と JSON に示され、全既存・新規テストと文書ガードが通る |
| M4: chief 自動・手動送信 | フェーズ 4（ステップ 4-1〜4-4） | 送信記録の PR/head/到達点と不消去、`conflictSender`、`PendingConflict`、`chief send --command mergemain --item`、控えと親たどり引き継ぎ、`chief.md`・`chief_guide.md`・`CLAUDE.md` 仕上げ | 全 37 件の受け入れ基準を満たし、グリーンゲートが通る |

依存順序は M1 → M2 → M3 → M4 である。M2 は M1 の `mergepr` 契約とプロトコル番号を前提とし、M4 は M3 の `OpenPR`・`ConflictPhase`・`paneinput`・`LaterInput` と M2 の `.claude/commands/mergemain.md` を前提とする。

## 4. テスト戦略

### 単体テスト

- **`internal/mergepr`（`mergemain_test.go`）:** 一時 git リポジトリとローカル bare リモートを使い、衝突分類（`.md`・両側追加・プロトコル行のみ・他行衝突・`_test.go`・modify/delete・3 種の名前変更・シンボリックリンク・バイナリ）、プロトコル番号更新（衝突あり/なし）、早送り禁止、無視ファイル保護、戻しの許可/拒否全条件、push の検証全項目と 3 結果、ゼロ値安全側を検証する（設計書 7.1）。
- **`internal/chief`（`workspace_test.go`・`position_test.go`・`conflict_test.go`・`table_test.go`・`output_test.go`・`records_test.go`・`send_test.go`・`pending_test.go`・`request_test.go`・`detail_test.go`・`running_test.go`・`screen_transcript_test.go`・`skiprecord_test.go`・`routing_test.go`・`paneinput_test.go`）:** AC-04 の厳格 JSON 検証（欠落・`null`・型違い・大小文字違い・空白・head 形式・重複・1 要素拒否で一覧全体拒否）、`classify`・`conflictPhaseOf`・`urgencyOf`・`progressOf`・`TestOutputValueNames`、AC-05 の (1)〜(10) と再確認の 1 条件欠落ごとの拒否、`/mergemain` 記録の不消去と他記録の消去、`LaterInput` 全 6 値、項目 ID 一致・`/runpr` 非提示、一時 git リポジトリでの `CollectDetail` 親たどり引き継ぎ（上限・取得成功/失敗・リセット非使用）を検証する（設計書 7.1）。

### 結合テスト

- **`internal/mergepr/mergemain_test.go`:** `mergemain-begin` → `mergemain-merge` → `git commit --no-edit` → `mergemain-push` の成功経路と、`mergemain-begin` → `mergemain-merge` → `mergemain-restore` の戻し経路を一連の呼び出しで検証する（設計書 7.2）。
- **`internal/chief/sim_test.go`:** 既存のシミュレーション基盤を使い、`tick` ラウンドでの自動送信（`/clear`・`/model opus`・`/mergemain <PR>` のみ）、同一 head での非再送、head 更新後の再送を検証する（設計書 7.2）。

### セキュリティテストと静的ガード

- **入力・送り先・別リポジトリの防御:** `paneinput_test.go`・`conflict_test.go`・`mergemain_test.go` で、送る入力に画面/transcript/PR 題名が入らないこと（AC-33）、`mergemain` 行が OpenCode なら送らないこと（AC-34）、`isCrossRepository` 真の PR を chief と `mergepr` の両方で拒否すること、および `mergemain-push` の改ざん防御（偽りの `main` 指定・別 `main` コミット・`refs/mergemain/<PR>/main` の履歴外書き換え・プロトコル値誤り・許可外ファイル変更・force push 禁止）を検証する（設計書 7.3）。
- **型構築ガード:** `internal/chief/construction_test.go` の `guardedTypes` に `"CommitID": {"ParseCommitID"}` を追加し、複合リテラル・省略リテラル・ポインタリテラル・フィールド代入・メソッド内構築のミューテーションテストで検証する。
- **文書・設定ガード:** `cmd/mergepr/docs_test.go`（`TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`・`TestProtocolToolsAgree`・`TestSettingsAllowsOnlySafeMergemainSubcommands`・`TestMergemainNoDirectGitMutation`）、`cmd/chief/docs_test.go`（`TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`・`TestGuideProtocolMatchesTool`・`TestArchitectureValuesMatchTool`）、`cmd/yt2column/extension_docs_test.go`（`TestExtensionDocumentation`）、`internal/chief/routing_test.go`・`markers_parity_test.go` を実行する（設計書 7.4）。

### 後方互換性と既存テストの更新方針

- `_context.md` の Command routing 表 7 行化は、要件定義書 4.4 のとおり `internal/chief/routing.go` と同一 PR（フェーズ 3）で変更し、互換レイヤは設けない。
- 既存の `gh pr list` テストデータ・モック（§1 調査結果の 7 ファイルと `testdata/gh/pr-list-open-867.json`）はフェーズ 3 で新 4 メンバーを補って更新し、既存テストの検証観点を落とさない。

### テストヘルパー構成（`test_organization.md` 準拠）

- パッケージ横断の新規 `testutil/` ファイルは不要。`internal/chief` は既存の `internal/chief/test_helpers.go`（`//go:build test`）と `world_test.go` を拡張し、`internal/mergepr` は既存 `internal/mergepr/mergemain_test.go`（必要に応じて `//go:build test` 付きのパッケージ内 `test_helpers_mergemain.go`）で一時 git リポジトリ作成ヘルパーを持つ。`//go:build test` 付きの非 `_test.go` ファイルを追加した場合は、導入フェーズの完了ゲートで `go test -run '^$' -tags test ./...` および `make lint` の通過を確認する。

## 5. リスク管理

| リスク | 影響 | 発生可能性 | 軽減策 |
|---|---|---|---|
| 実物 PR で `gh pr list --json mergeable` が `UNKNOWN` を返し続ける（設計書 1.2・H-01） | コンフリクト検出が遅れる、または検出できない | 中 | ステップ 3-3 で実物 PR に対し `gh pr list` と `gh pr view` を比較して結果を本節へ記録する。`UNKNOWN` は安全側（送らない）に倒れ、継続する場合は利用者と方針を確認する |
| merge コミット後の `review-records.sh coverage` の振る舞いが設計書付録 B の推論と食い違う（H-05） | `/runpr` 時のマージ質問での確認量が増減する | 低 | ステップ 2-2 で実際の merge コミットに対して `review-records.sh coverage` を実行し、結果を本節へ記録する |
| 検査・pre-commit フックが未追跡ファイルを残し、`mergemain-restore` が「ほかの変更があり戻さなかった」で止まる（設計書 3.8.4） | 自動解消失敗時の自動復旧が阻害される | 低 | ステップ 2-2 で `make test`・`make lint`・`make ext-check`・pre-commit フック実行後の `git status --porcelain` を確認し、結果を本節へ記録する |
| フェーズ 3 マージ後に古い `chief` が 7 行の `_context.md` を拒み、自動送信が止まる（設計書 3.10） | 入れ直しまでマージ後の自動送信も止まる | 高（仕様どおり） | ステップ 3-3 で `chief_guide.md` に明記し、フェーズ 3 マージ後に利用者が `make install-chief` して `/chief` を再起動する運用を PR 境界の `hold` で明示する |
| 並行ブランチとのプロトコル番号衝突（`mergepr`・`chief`） | マージ時に `mergepr prepare`/`merge` が停止する | 中 | 本計画では上げた番号を書かず常に `main` の値 + 1 とし、実装・マージ時点の `origin/main` に合わせる |
| `git merge-tree --write-tree` の挙動差や特殊衝突ケース（rename/modify、binary、symlink）の取りこぼし | 対象外の衝突を誤って解消・push する | 低 | ステップ 1-1・1-2 で設計書 7.1・7.3 の全特殊衝突ケースを一時 git リポジトリで網羅テストし、`mergemain-merge` と `mergemain-push` の両方で同一分類ロジックを使う |

## 6. 実装チェックリスト

- [ ] フェーズ 1 完了（ステップ 1-1〜1-3: 取り込みの道具 `mergepr`）
- [ ] フェーズ 2 完了（ステップ 2-1〜2-2: コマンド定義 `mergemain.md` とガード）
- [ ] フェーズ 3 完了（ステップ 3-1〜3-3: chief の検出と表示）
- [ ] フェーズ 4 完了（ステップ 4-1〜4-4: chief の送信と文書仕上げ）

## 7. 受け入れ基準の検証

各受け入れ基準の検証種別は `test`（実行テスト）、`static`（ガードテスト・`make` ターゲット）、`manual`（散文のみの要件に対する PR レビュー読解）とする。混合要件は値・構造を `test` または `static` で、散文を `manual` で検証する。

| AC | 対象ステップ | 種別 | 検証箇所・内容 |
|---|---|---|---|
| AC-01 | 3-1 | `test` | `internal/chief/workspace_test.go::TestOpenPRsReadsConflictFields`（`tick`・`snapshot` 共通の `OpenPRs` が 4 メンバーを読むことを検証） |
| AC-02 | 3-1 | `test` | `internal/chief/position_test.go::TestClassifyConflict`・`internal/chief/table_test.go::TestTableConflictRow`（`CONFLICTING` 時の状態・急ぎの度合い、実行中/質問優先、`ConflictSending` の待つ 🟢 を検証） |
| AC-03 | 3-1, 4-2, 4-3 | `test` | `internal/chief/position_test.go::TestClassifyNonConflict`・`internal/chief/conflict_test.go::TestConflictSenderSkipsNonConflicting`・`internal/chief/pending_test.go::TestPendingNoConflictForMergeableOrUnknown` |
| AC-04 | 3-1 | `test` | `internal/chief/workspace_test.go::TestParsePRListRejectsInvalidConflictFields`（欠落・`null`・型違い・大小文字・空白・head 形式・重複・1 要素拒否で一覧全体拒否、`StateNeedsCheck` を検証） |
| AC-05 | 4-2 | `test` | `internal/chief/conflict_test.go::TestConflictSenderConditions`（(1)〜(10) と再確認の各条件を 1 つずつ外して非送信・理由名を検証、全充足で送信を検証） |
| AC-06 | 4-2, 4-3 | `test` | `internal/chief/conflict_test.go::TestSnapshotDoesNotAutoSendConflict`・`internal/chief/pending_test.go::TestPendingConflictInSnapshot` |
| AC-07 | 4-2 | `test` | `internal/chief/conflict_test.go::TestConflictSenderPreexistingConflict`（前ラウンド未観測でも `tick` で送信することを検証） |
| AC-08 | 4-1, 4-2 | `test` | `internal/chief/records_test.go::TestMergemainRecordsPersistAcrossCleanupAndClear`・`internal/chief/conflict_test.go::TestConflictSenderOncePerHeadAndUnreadableRecord`・`internal/chief/sim_test.go::TestSimConflictAutoSend` |
| AC-09 | 4-1 | `test` | `internal/chief/send_test.go::TestSendMergemainUncertainStopsWorkspace`（結果不明時の非再送と停止を検証） |
| AC-10 | 3-1, 4-2 | `test` | `internal/chief/conflict_test.go::TestConflictSenderSkipsWhileRunprOrQuestion`（`/runpr` 実行中・質問中は送らず、終了後の `CONFLICTING` で送ることを検証） |
| AC-11 | 4-2 | `test` | `internal/chief/conflict_test.go::TestConflictSenderNeverTriggersFromScreenOrTranscript`（`CONFLICTING` 以外では画面・transcript 内容にかかわらず送らないことを検証） |
| AC-12 | 4-3 | `test` | `internal/chief/detail_test.go::TestKeepPaneReport`・`internal/chief/detail_test.go::TestCollectDetailInheritsReportAcrossMerge`（pane 報告控えのみ作成・読めなければ控えなし・2 親 merge コミットの親たどり引き継ぎ・非引き継ぎ・取得成否・上限・リセット非使用を検証） |
| AC-13 | 4-2 | `test` | `internal/chief/sim_test.go::TestSimConflictAutoSend`（送信入力が `/clear`・`/model <モデル>`・`/mergemain <PR>` の順のみであることを検証） |
| AC-14 | 3-1, 4-1, 4-3 | `test` | `internal/chief/conflict_test.go::TestConflictPhaseTransitions`・`internal/chief/pending_test.go::TestPendingConflictNotificationBudget`（`ConflictUnresolved`・`ConflictSending`・`ConflictEndedBeforeCommand` の段階・急ぎの度合い・詳細区別・head ごと最大 2 回通知を検証） |
| AC-15 | 4-1, 4-2 | `test` | `internal/chief/conflict_test.go::TestConflictSenderResendsAfterHeadChanges`・`internal/chief/sim_test.go::TestSimConflictAutoSend` |
| AC-16 | 3-1 | `test` | `internal/chief/position_test.go::TestClassifyReturnsToReviewAfterResolution` |
| AC-17 | 4-3 | `test` | `internal/chief/pending_test.go::TestPendingConflictOfferAndLaterInputNote`（AC-18 充足時のみ選択肢提示、非充足時は理由表示、後の入力ありで会話消失印、`ConflictSending` は選択肢なし、`/runpr` 非提示を検証） |
| AC-18 | 4-3 | `test` | `internal/chief/request_test.go::TestSendMergemainByItem`・`cmd/chief/main_test.go::TestParseSendMergemain`（項目 ID 一致・AC-18 全条件再確認、AC-05 (6)(7)(10) 非要求、`--pr`/`--issue` 拒否を検証） |
| AC-19 | 4-1, 4-3 | `test` | `internal/chief/request_test.go::TestSendMergemainRecordsAndBlocksAutoSend`（手動送信も記録を作り同一 head の自動送信を抑止し、最後の送信で AC-14 を判定することを検証） |
| AC-20 | 1-1, 1-3, 2-1 | `test` / `manual` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainBeginRejectsInvalidPRArg`・`cmd/mergepr/main_test.go::TestMergemainArgValidation`。`manual`: `.claude/commands/mergemain.md` の引数検証手順のレビュー |
| AC-21 | 1-1, 2-1 | `test` / `manual` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainBeginPremisesAndNothingToTakeIn`・`TestMergemainMergeNoFastForward`（前提・`--no-ff`・衝突なし取り込み・取り込むものなしを検証）。`manual`: `.claude/commands/mergemain.md` の手順レビュー |
| AC-22 | 1-1, 1-2, 2-1 | `test` / `manual` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainProtocolUpdateAndRuleFacts`・`TestMergemainPushChecksProtocolValues`（プロトコル番号 `main` + 1 更新・README/ADR 事実出力・push 時値検証）。`manual`: `.claude/commands/mergemain.md` の README・ADR・`.md` 両側保持判断手順のレビュー |
| AC-23 | 1-1, 1-2 | `test` | `internal/mergepr/mergemain_test.go::TestMergemainConflictClassification`・`TestMergemainPushRejectsOutOfScope`（`.md`・プロトコル行のみ許可、他ファイル・他行衝突・`_test.go`・modify/delete・3 種 rename・symlink・binary 拒否を検証） |
| AC-24 | 2-1 | `manual` | `.claude/commands/mergemain.md` が `mktrans.md` の `stopped` 時に停止・戻し手順へ進むことを PR レビューで確認（自然言語手順のみの要件） |
| AC-25 | 1-1, 2-1 | `test` / `manual` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainReportsADRConflictFacts`（`docs/dev/adr/` 変更と番号重複の検出を検証）。`manual`: `.claude/commands/mergemain.md` の ADR 重複・食い違い記述時の停止と `numreserve` 非実行を PR レビューで確認 |
| AC-26 | 1-1, 2-1 | `static` / `manual` | `static`: `cmd/yt2column/extension_docs_test.go::TestExtensionDocumentation`（`Makefile` の `ext-preflight` と `CLAUDE.md` の一致を検証）および `internal/mergepr/mergemain_test.go::TestMergemainExtCheckHint`（`ext-check` 対象パス判定を検証）。`manual`: `.claude/commands/mergemain.md` の `make test`・`make lint`・`make ext-preflight`→`make ext-check` と依存未導入時の停止理由を PR レビューで確認 |
| AC-27 | 1-2, 2-1 | `test` / `manual` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainPushSuccessRejectedAndUnknown`（非 force push・拒否時停止・結果不明時非戻しを検証）。`manual`: `.claude/commands/mergemain.md` の push 結果分岐を PR レビューで確認 |
| AC-28 | 1-2, 2-2 | `test` / `static` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainRestoreSafeAndRefusesOtherChanges`（戻し後の状態、無視・未追跡ファイル保護、他変更時の非戻しを検証）。`static`: `cmd/mergepr/docs_test.go::TestSettingsAllowsOnlySafeMergemainSubcommands`（`mergemain-restore` を allow しないことを検証） |
| AC-29 | 1-3, 2-1 | `test` / `manual` | `test`: `cmd/mergepr/main_test.go::TestMergemainStopReasonsAndOutput`（道具が出す理由種別・衝突ファイル一覧・新 head・番号変更道具一覧の出力を検証）。`manual`: `.claude/commands/mergemain.md` の日本語報告形式・11 種の理由・プロトコル番号変更時の入れ直し案内を PR レビューで確認 |
| AC-30 | 1-1, 1-2, 2-1, 2-2 | `test` / `static` / `manual` | `test`: `internal/mergepr/mergemain_test.go::TestMergemainPRViewReadsNarrowFields`・`TestMergemainPushRejectsNonHeadOrForce`（狭い PR 読み取り・PR head 以外への push 拒否を検証）。`static`: `cmd/mergepr/docs_test.go::TestMergemainNoDirectGitMutation`（`mergemain.md` の直接 `git merge`/`git reset`/`git push` 禁止を検証）。`manual`: `.claude/commands/mergemain.md` の 8 つの禁止事項を PR レビューで確認 |
| AC-31 | 2-1 | `manual` | `CLAUDE.md` の「Tool Execution Safety」と `.claude/commands/mergemain.md` の承認段落に 4 操作（`gh pr view`・`git fetch origin`・PR head ブランチの `git ls-remote`・PR head ブランチへの `git push`）が揃っていることを PR レビューで確認（自然言語承認文のみの要件） |
| AC-32 | 3-2 | `test` / `static` | `test`: `internal/chief/routing_test.go::TestReadRouting`（7 行読み取り・欠落/不明行拒否・`mergemain` 行の `Claude Code`/`opus` 読み取りを検証）。`static`: `internal/chief/markers_parity_test.go::TestMarkerReadingMatchesDecisionScript`（現行 `_context.md` の読み取り整合を検証） |
| AC-33 | 3-2, 4-2 | `test` | `internal/chief/paneinput/paneinput_test.go::TestNewMergemain`・`internal/chief/conflict_test.go::TestConflictSendInputUsesTypedCommandOnly`（型から組み立てた `/mergemain <PR>` のみ送信し、PR 題名・transcript・自由文・禁止コマンドを含まないことと chief 自身への非送信を検証） |
| AC-34 | 3-2, 4-2 | `test` | `internal/chief/conflict_test.go::TestConflictSenderRefusesOpenCodeRoute`（`mergemain` 行が OpenCode のとき送らず `route_not_claude` を示すことを検証） |
| AC-35 | 3-3, 4-4 | `static` / `manual` | `static`: `cmd/chief/docs_test.go::TestGuideProtocolMatchesTool`（`chief_guide.md` のプロトコル番号一致を検証）。`manual`: `docs/dev/guides/chief_guide.md` の 4 項目（状態/急ぎ/詳細、自動送信条件、送り直し規則、人が選ぶ選択肢）を PR レビューで確認 |
| AC-36 | 2-1, 3-3, 4-4 | `static` / `manual` | `static`: `cmd/chief/docs_test.go::TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`・`cmd/mergepr/docs_test.go::TestDocumentedProtocolMatches`・`TestDocumentedInvocationsParse`（`chief.md`・`mergemain.md` のプロトコル番号とコマンド呼び出し構文を検証）。`manual`: `CLAUDE.md` の `/mergemain`・`/chief` 説明と `chief.md` の選択肢表示手順を PR レビューで確認 |
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
