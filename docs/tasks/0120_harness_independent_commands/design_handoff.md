# 設計への申し送り（Design Handoff）

本書は、`01_requirements.md` の起草・セルフレビューおよび issue #1072（コメント 1・2 を含む）の整理で挙がったアーキテクチャ・設計レベルの検討事項を、`02_architecture.md` へ申し送るものである。

## H-01: Antigravity（Gemini 系）のクロスチェック担当モデル（Carbon / Argon）および各役割モデルの評価と選定

- **論点:** issue #1072 コメント 1 では Antigravity のクロスチェックの部分レビュー用モデルとして `gemini-4-argon-medium` が例示され、コメント 2 および要件定義時の確認で「Fable 部分レビュー自体を一般化し、Antigravity（Gemini 系）では Carbon（`gemini-4-carbon-*`）または Argon（`gemini-4-argon-*`）を比較評価して採用モデルを 1 つに決定すること」と定まった。あわせて、`_context.md` の対応表に載せる Antigravity の `high`・`standard`・`low` 役割の具体モデルと起動手段を決める必要がある。特に Antigravity の `invoke_subagent` は `inherit / flash_lite / flash / pro` の 4 値しか取れず、`inherit` ではメインセッションの上位モデル（Carbon 等）をそのまま消費し、`flash` では Gemini 3.8 Flash になるため、`japrose.md` の `standard` 役割などには `swarm(action: "add", model: "gemini-4-neon-medium", ...)` を割り当て、`low` 役割（`fixpr.md` Phase 6）や確認パスにおける `swarm` と `invoke_subagent` の使い分けを設計で確定する。
- **重要である理由:** クロスチェックの部分レビューは 4 基準に対する検出精度、決められた出力形式の遵守率、実行時間（30 分上限内）、割り当ての消費量に依存する（`swarm(action: "models", all: true)` の確認では、`gemini-4-argon-*` と `gemini-4-carbon-*` を含む Gemini モデル群は同じ割り当て枠を共有している）。また、上位モデルのレビューとエスカレーション判定（`high` 役割）も `swarm` でモデルを固定するため、正確なモデル識別子と effort 水準を実測で確かめてから `_context.md` に固定する必要がある。
- **候補となる方針:** 設計段階で過去の PR 標本または試験プロンプトを用いて `gemini-4-carbon-*` と `gemini-4-argon-*` の出力形式遵守・指摘の質・所要時間・トークン消費を比較評価し、採用モデル 1 つと各役割モデルを `02_architecture.md` に記録する。
- **関連する要件:** `F-001`（`AC-06`、`AC-07`）、`F-003`（`AC-16`）、`F-004`（`AC-21`）、`F-005`（`AC-27`、`AC-30`）

## H-02: ハーネスのアダプタと `.agents/skills/` 薄い入口のファイル構成・ガードテストの設計

- **論点:** `.claude/commands/` の本体から切り出す抽象操作の粒度、ハーネスのアダプタ文書（例: `.claude/commands/_lib/harness-claude.md` と `.claude/commands/_lib/harness-antigravity.md`）の章立てと判別規則、`.agents/skills/<cmd>/SKILL.md` の YAML フロントマター（`name`・`description`）と委譲文面の標準形、および既存の文書のガードテスト群への組み込み方を決める必要がある。また、実装時点で `mergemain.md` などのコマンドが `main` に加わっている場合の対象一覧も確認する。
- **重要である理由:** `.agents/skills/` はエージェントが実行する指示文書であるため、`cmd/yt2column/docpaths_test.go`（`TestInstructionDocPaths`）のリンク検査だけでなく、`internal/doctestutil/protocol.go`（`CommandDocs`）を使う信頼境界ガード（`cmd/yt2column/codex_review_docs_test.go`・`cmd/chief/docs_test.go`・`cmd/numreserve/docs_test.go`）や `cmd/yt2column/fixpr_docs_test.go`（Phase 6 のモデル指定検査を含む）、および非 `chief` コマンドとの 1 対 1 対応検査の設計が必要になる。なお、`AC-41` のうち自然言語の命名方針の遵守確認は実装計画書で `manual` 検証として位置づける。
- **候補となる方針:** `_lib/harness-claude.md` と `_lib/harness-antigravity.md` に共通の見出し構造を持たせ、`.agents/skills/<cmd>/SKILL.md` は手順本文を持たない定型テンプレートにし、`doctestutil.CommandDocs` または共通ヘルパを拡張して `.agents/skills/` をすべてのコマンド指示ガードの対象に含める。
- **関連する要件:** `F-001`（`AC-01`〜`AC-08`）、`F-002`（`AC-09`〜`AC-11`）、`F-008`（`AC-41`〜`AC-43`）

## H-03: 一般化した手順ファイル名・モード名・記録行・判定スクリプトの具体的な文法と移行順序

- **論点:** モデル固有名を外した共有手順の新しいファイル名（例: `_lib/high-review.md`・`_lib/high-review-decision.sh`・`_lib/cross-check-review.md`）、呼び出し入力名（例: `CROSS_CHECK`）、`review-records.sh` のモード名（例: `cross-check` と移行用の旧 `fable`）、判定スクリプトの理由コード（例: `all-high`）、新しい記録行の接頭辞（例: `High review: done` / `High review: kept`、`Cross-check review: run` / `Cross-check review: not run` / `Cross-check scope: in` / `Cross-check scope: out` / `Cross-check finding:` / `Cross-check review: checkpoint`）、モデル不一致の理由コード（例: `wrong-model`）、および PR 説明の見出し（例: `## クロスチェックの部分レビュー`）を設計で一意に決める必要がある。
- **重要である理由:** コミットの記録行と PR 説明のブロックは判定スクリプト・`review-records.sh`・`task-pr-handoff.md`・`runpr.md`・`cmd/yt2column/review_records_test.go`・`cmd/yt2column/fable_state_docs_test.go`・`internal/chief/markers_parity_test.go` および PR 集計（#936）が機械的に読む契約である。また、進行中ブランチのコマンド定義は `origin/main` からスクリプトをパス指定で取り出すため、スクリプトのファイル名やモード名を変える場合は `CLAUDE.md:954-970` の日本語リテラル例外・`docs/dev/conventions/dev_tools.md` Rule 6・移行中の旧ブランチ互換を考慮した PR 分割順序が要る。
- **候補となる方針:** 記録行の正確な正規表現（新旧両読み）と `_context.md` からの安全なモデル ID 読み取り規則を設計で一意に定め、`origin/main` からスクリプトを取り出す移行中に進行中ブランチが失敗しない移行手順（段階的なモード追加または旧パスの段階的廃止）を設計する。
- **関連する要件:** `F-004`（`AC-18`、`AC-23`）、`F-005`（`AC-25`、`AC-29`、`AC-30`）、`F-007`（`AC-38`）、`F-008`（`AC-41`、`AC-43`）

## H-04: 外部 git worktree 上の `swarm` 実測確認・返却スキーマ・タイマー条件・状態チェックの詳細

- **論点:** 設計段階で次の 4 点を実測・確定する必要がある。
  1.  Herdr の外部 git worktree（`.git` がワークスペース外を指す構成）で `swarm(action: "add", ..., workspace: "inherit")` ワーカーが `git log` / `git diff` を直接実行できるか、あるいはサンドボックスで拒否／承認待ち停止になるか。直接実行できない場合は、メインセッションが `git log`・`git diff` をスクラッチパッドのファイルへ書き出してワーカーに読ませる手順を設計する。
  2.  `swarm(action: "add")` と `swarm(action: "status")` の返却 JSON スキーマとトークンの使用量の取得可否を確定する（本タスクの要件定義時の実行では、`swarm(action: "add", ...)` の返却 JSON に `"model": "gemini-4-neon-medium"` が直接含まれることを確認済み）。
  3.  一般・段階指定のサブエージェントにおける `swarm`（`japrose` の `standard` 役割を `gemini-4-neon-medium` で起動して完了時に `kill` する手順など）と `invoke_subagent`（読み取り専用 `research` とコマンド実行可能な `self` の使い分けを含む）の境界を確定する。
  4.  `schedule` ツールで並列の CodeRabbit CLI 完了通知に早期解除されないタイマー指定（`TimerCondition: "never"` と出口ごとの `manage_task(Action: "kill")` の組み合わせ）と、`.agents/`・`agents.md`・`agent.md`・`gemini.md`・`agents.override.md` を加えた worktree 状態チェックスクリプト（`BypassSandbox: true` での単一 `bash <script>` 呼び出しを含む）の詳細を定める。
- **重要である理由:** 外部ツールの未検証の挙動に依存したまま手順を固定すると、実装・運用時にバックグラウンドワーカーが停止したり、タイマーが早期解除されたりする欠陥になるためである。
- **候補となる方針:** `02_architecture.md` の作成中に最小限の読み取り専用ワーカーで (1) と (2) を実測して記録し、確実な単一の手順として設計書に落とし込む。
- **関連する要件:** `F-001`（`AC-06`〜`AC-08`）、`F-004`（`AC-21`）、`F-005`（`AC-27`）、`F-006`（`AC-33`、`AC-36`）

## H-05: `_context.md` のハーネス別モデル対応表の構造と既存パーサとの分離

- **論点:** `_context.md` の `### PR marker conventions` 内では 2 列の表が旧値の読み替え表として判定スクリプトと `internal/chief/task.go` に読まれ（`.claude/commands/_context.md:154-155`）、`### Command routing` は `internal/chief/routing.go` が厳格な 4 列・固定 6 行（`runplan-high` 等の改名後の行キー）として読む。新しく追加するハーネス別の役割→実モデル対応表をどの独立見出し・列構成で置き、判定スクリプトと `review-records.sh` がどの列をどう厳密に抽出するかを決める必要がある。
- **重要である理由:** 表の置き場所や列数を誤ると `ParseOldValues` や `ParseRouting` が表を拒否して `chief` が全送信を停止するか、スクリプト側のモデル ID 抽出が曖昧になるためである。
- **候補となる方針:** ハーネス別の役割→実モデル対応表は `### PR marker conventions` と `### Command routing` の外側の専用 `###` 見出しの下に 4 列以上の固定構造で置き、`ParseOldValues`・`ParseRouting`・両シェルスクリプトの単体テストで相互非干渉を検証する。
- **関連する要件:** `F-003`（`AC-14`、`AC-16`）、`F-007`（`AC-37`〜`AC-40`）
