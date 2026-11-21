# 設計への申し送り（Design Handoff）

本書は、`01_requirements.md` の起草および issue #1072（コメント 1・2 を含む）の整理で挙がったアーキテクチャ・設計レベルの検討事項を、`02_architecture.md` へ申し送るものである。

## H-01: Antigravity（Gemini 系）のクロスチェック担当モデル（Carbon / Argon）および各役割モデルの評価と選定

- **論点:** issue #1072 コメント 1 では Antigravity のクロスチェックの部分レビュー用モデルとして `gemini-4-argon-medium` が例示され、コメント 2 および要件定義時の追加確認で「Fable 部分レビュー自体を一般化し、Antigravity（Gemini 系）では Carbon（`gemini-4-carbon-*`）または Argon（`gemini-4-argon-*`）を候補として設計段階で評価し、採用するモデルを決定すること」と指定されている。あわせて、`_context.md` の対応表に載せる Antigravity の `high`・`standard`・`low` 役割の具体モデル（例: `gemini-4-neon-*`・`gemini-3.8-flash-*` 等の候補）も決める必要がある。
- **重要である理由:** クロスチェックの部分レビューは 4 基準（信頼の境界・先行文書との照合・外部の振る舞いの主張・未確認の選択）に対する検出精度と、決められた出力形式の遵守率、実行時間（30 分上限内）、割り当ての消費量に依存する（なお `swarm(action: "models", all: true)` の確認では、`gemini-4-argon-*` と `gemini-4-carbon-*` を含む Gemini モデル群は同じ割り当て枠を共有している）。また、上位モデルのレビュー（`high`）も `swarm` でモデルを固定するため、正確なモデル識別子と effort 水準を実測で確かめてから `_context.md` に固定する必要がある。
- **候補となる方針:** 設計段階で過去の PR 標本または試験プロンプトを用いて `gemini-4-carbon-*` と `gemini-4-argon-*` の出力形式遵守・指摘の質・所要時間・トークン消費を比較評価し、採用モデルを `02_architecture.md` に記録する。
- **関連する要件:** `F-003`（`AC-15`）、`F-004`（`AC-20`）、`F-005`（`AC-26`、`AC-29`）

## H-02: ハーネスのアダプタと `.agents/skills/` 薄い入口のファイル構成・抽象操作の定義

- **論点:** `.claude/commands/` の本体から切り出す抽象操作の粒度、ハーネスのアダプタ文書（例: `.claude/commands/_lib/harness-claude.md` と `.claude/commands/_lib/harness-antigravity.md`）の章立てと判別規則、および `.agents/skills/<cmd>/SKILL.md` の YAML フロントマター（`name`・`description`）と委譲文面の標準形を決める必要がある。また、実装時点で `mergemain.md` などのコマンドが `main` に加わっている場合の対象一覧も確認する。
- **重要である理由:** 抽象操作の入出力が曖昧だと各コマンド定義にハーネス固有の前提が漏れ出し、`TestInstructionDocPaths` やコマンド契約のガードテスト（`fixpr_docs_test.go`・`cmd/mergepr/docs_test.go`・`cmd/numreserve/docs_test.go` など）の走査対象とも噛み合わなくなるためである。
- **候補となる方針:** `_lib/harness-claude.md` と `_lib/harness-antigravity.md` に共通の見出し構造（ハーネス判別、質問、一般サブエージェント、モデル固定ワーカー、スクラッチパッド、コマンド実行とサンドボックス、バックグラウンドとタイマー、通知）を持たせ、`.agents/skills/<cmd>/SKILL.md` は 10〜20 行程度の定型テンプレートにして `cmd/yt2column/docpaths_test.go` でリンクと対象コマンドとの対応を検査する。
- **関連する要件:** `F-001`（`AC-01`〜`AC-07`）、`F-002`（`AC-08`〜`AC-10`）、`F-008`（`AC-40`、`AC-41`）

## H-03: 一般化した手順ファイル名・モード名・記録行・判定スクリプトの具体的な文法と移行順序

- **論点:** モデル固有名を外した共有手順の新しいファイル名（例: `_lib/high-review.md`・`_lib/high-review-decision.sh`・`_lib/cross-check-review.md`）、呼び出し入力名（例: `CROSS_CHECK`）、`review-records.sh` のモード名（例: `cross-check` と旧 `fable` の扱い）、判定スクリプトの理由コード（例: `all-high`）、新しい記録行の接頭辞（例: `High review: done` / `High review: kept`、`Cross-check review: run` / `Cross-check review: not run` / `Cross-check scope: in` / `Cross-check scope: out` / `Cross-check finding:` / `Cross-check review: checkpoint`）、モデル不一致の理由コード（例: `wrong-model`）、および PR 説明の見出し（例: `## クロスチェックの部分レビュー`）を設計で一意に決める必要がある。
- **重要である理由:** コミットの記録行と PR 説明のブロックは判定スクリプト・`review-records.sh`・`task-pr-handoff.md`・`runpr.md` および PR 集計（#936）が機械的に読む契約である。また、進行中ブランチのコマンド定義は `origin/main` からスクリプトをパス指定で取り出すため（現行 `.claude/commands/_lib/opus-review-decision.sh` と `.claude/commands/_lib/review-records.sh`）、スクリプトのファイル名やモード名を変える場合は `CLAUDE.md:954-970` の日本語リテラル例外・`docs/dev/conventions/dev_tools.md` Rule 6・移行中の旧ブランチ互換を考慮した PR 分割順序が要る。
- **候補となる方針:** 記録行の正確な正規表現（新旧両読み）と `_context.md` からのモデル ID 読み取り規則を設計で一意に定め、`origin/main` からスクリプトを取り出す移行中に進行中ブランチが失敗しない移行手順（段階的なモード追加またはラッパー経由の切り替え）を設計する。
- **関連する要件:** `F-004`（`AC-17`、`AC-22`）、`F-005`（`AC-24`、`AC-28`、`AC-29`）、`F-008`（`AC-39`、`AC-41`）

## H-04: Antigravity における一般サブエージェント（`invoke_subagent`）と `swarm` の使い分けおよび状態チェック

- **論点:** Antigravity でモデル固定が不要な一般サブエージェント（セルフレビュー、パネルレビュー、`/fixpr` のトリアージと修正確認、`japrose`、`mktrans`）を呼ぶ際、`invoke_subagent` の `TypeName`（読み取り専用の `research` とコマンド実行可能な `self`）と `Model`（`inherit` と確認パスの `flash` 等）をどう使い分けるか。また、`swarm` ワーカーや `research` サブエージェントを外部 git worktree 構成で動かしたときに、サンドボックス制約（ワークスペース外の `.git/worktrees/...` が読めない問題）や `_lib/fable-review.md` の worktree 状態チェックスクリプトがどう振る舞うかを確かめる必要がある。
- **重要である理由:** Antigravity の `research` サブエージェントは読み取り専用サンドボックスで動くため、ワークスペース内のファイルは読めても外部 worktree の `git diff` / `git show` は失敗する一方、`swarm(workspace: "inherit")` や `self` はコマンド実行権限（および状態変更の可能性）を持つ。ここを誤るとサブエージェントが差分を読めないか、状態チェックが動かない恐れがある。
- **候補となる方針:** 設計段階で外部 git worktree 上の `invoke_subagent`（`research` / `self`）と `swarm` の挙動を確認し、ファイルを直接読むだけのレビュー（文書セルフレビュー・`japrose` 等）と `git` コマンドやテスト実行を要するレビューの起動方法、および状態チェックの実行方法（`BypassSandbox: true` が要る `git` 呼び出しの扱いを含む）を定める。
- **関連する要件:** `F-001`（`AC-06`）、`F-004`（`AC-20`）、`F-005`（`AC-24`、`AC-26`）、`F-006`（`AC-32`）

## H-05: `_context.md` の表構造と `internal/chief` のプロトコル番号更新の要否

- **論点:** `_context.md` の `### PR marker conventions` 内では 2 列の表が旧値の読み替え表として `opus-review-decision.sh` と `internal/chief/task.go` に読まれる（`_context.md:154-155`）一方、`### Command routing` は `internal/chief/routing.go` が厳格な 4 列・固定 6 行（`runplan-opus` 等の行キー）として読む（`internal/chief/routing.go:27-34,51-56`）。ハーネス別の役割→実モデル対応表をどの見出し・列構成で置くか、`Command routing` の行キー名（`runplan-opus` 等）を維持するか `runplan-high` 等へ改めるか、また `internal/chief` の変更が `docs/dev/chief_architecture.md` のプロトコル番号更新基準に当たるかを設計で決める必要がある。
- **重要である理由:** 表の置き場所や列数を誤ると `ParseOldValues` や `ParseRouting` が表を拒否して `chief` が全送信を停止するか、移行中の古い `chief` バイナリとの不整合が生じるためである。
- **候補となる方針:** ハーネス別の役割→実モデル対応表は `### PR marker conventions` と `### Command routing` の外側の独立した `###` 節に置き、`internal/chief` のプロトコル番号を上げるか（`Command routing` の行キー改名を含めるか、`chief` と `chief.md` の契約・副作用条件が変わるか）を `docs/dev/chief_architecture.md` の基準に照らして判定する。
- **関連する要件:** `F-003`（`AC-13`、`AC-15`）、`F-007`（`AC-35`〜`AC-38`）
