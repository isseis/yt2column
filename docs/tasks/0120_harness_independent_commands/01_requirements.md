# 要件定義書：開発コマンド定義と補助ツールのハーネス非依存化（Antigravity 対応）

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Process | `full` |
| Created | 2026-11-21 |
| Review date | 2026-11-22 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`.claude/commands/` 配下の開発コマンド定義・共有手順・補助スクリプトおよび開発ツールの設定をハーネス（開発コマンドを実行するエージェント実行環境。本書の主要用語は 3.1 で定義する）に依存しない形へ改め、Claude Code に加えて Antigravity CLI（`agy`）でも同じ開発フローを実行できるようにするための要件を定義する。

対応 issue: [#1072](https://github.com/isseis/yt2column/issues/1072)
前提 issue: [#1059](https://github.com/isseis/yt2column/issues/1059)（タスク 0118：Codex の PR レビューと Codex CLI の撤去）
関連 issue: [#988](https://github.com/isseis/yt2column/issues/988)（タスク 0114：記録後の merge commit の確認。先にマージされた側に後から実装する側が合わせる）、[#1057](https://github.com/isseis/yt2column/issues/1057)（計画書と実装 PR の部分レビューの置き換え。本タスクの後に行う）、[#760](https://github.com/isseis/yt2column/issues/760)（タスク 0081：実装モデル要件と Opus のレビューの導入）

**背景:**

現在、`.claude/commands/` 配下の開発コマンド定義（`mkreq.md`・`mkarch.md`・`mkplan.md`・`mkplan2.md`・`runplan.md`・`runpr.md`・`fixpr.md`・`mergepr.md`・`weakreview.md`・`japrose.md`・`mktrans.md`・`chief.md`、`CLAUDE.md:207-221`）と共有手順（`_lib/`）、判定用のシェルスクリプトは、Claude Code（一部 OpenCode）で動くことを前提に書かれている。Antigravity CLI（`agy`）で同じ手順を動かそうとすると、モデルの名前を置き換えるだけでは次の 4 か所の前提に阻まれる。

1.  **組み込みツールと実行制約の違い:**
    -   **選択式の質問:** コマンド定義と `.claude/commands/_lib/settle-decisions.md:37` は Claude Code の `AskUserQuestion` を名指ししている。Antigravity の質問ツールは `ask_question` である。
    -   **サブエージェント起動とモデル指定:** `.claude/commands/_lib/review-subagent-pattern.md:30-39`・`.claude/commands/_lib/opus-review.md:255,393-397`・`.claude/commands/_lib/fable-review.md:68`・`.claude/commands/_lib/pr-review-escalation.md:83`・`.claude/commands/japrose.md:133`・`.claude/commands/fixpr.md:25` は、Claude Code の `Agent` ツールで任意のモデル名（`opus`・`fable`・`sonnet`・`haiku`）を指定できることを前提としている。Antigravity の `invoke_subagent` は 4 段階の列挙値（`inherit`・`flash_lite`・`flash`・`pro`）しか取れず（`inherit` はメインセッションの上位モデルをそのまま引き継ぎ、`flash` は軽量な Flash 系になる）、中間モデル（`gemini-4-neon-medium` など）や具体的なモデルの ID を指定できるのは `swarm(action: "add", model: "<model-id>", ...)` だけである（issue #1072 コメント 1）。
    -   **ファイル読み書きとスクラッチパッド:** `.claude/commands/_context.md:210`・`.claude/commands/mkreq.md:56`・`.claude/commands/mergepr.md:182-184` などは Claude Code の `Write` ツールとスクラッチパッド（作業ツリー外のセッション用一時ファイル置き場。3.1）を名指ししている。Antigravity のセッション用一時ディレクトリは `<appDataDir>/brain/<conversation-id>/scratch/` であり、ツール名も異なる。
    -   **シェル実行・タイマー・通知・停止:** `.claude/commands/_lib/fable-review.md:147-161` は `Bash` のバックグラウンド `sleep 1800` と `TaskStop`・`ToolSearch` を使い、`.claude/commands/_context.md:52` は `PushNotification` を名指ししている。Antigravity は `run_command`・`schedule`・`manage_task` を持ち、シェルの `sleep` をタイマーとして使わず、`PushNotification` に相当するツールを持たない。
2.  **決定論的スクリプト・設定のハードコード:**
    -   `.claude/commands/_lib/opus-review-decision.sh:307-318` と `.claude/commands/_lib/opus-review.md:53-75` は、上位モデルの ID を `claude-opus-*` にハードコードし、システムプロンプトから自分のモデルの ID を読めることを前提としている。Antigravity のセッションとサブエージェントはシステムプロンプトから自身のモデルの ID を読めない（issue #1072 コメント 1）。
    -   `.claude/commands/_lib/review-records.sh:195-196` と `.claude/commands/_lib/fable-review.md:317-318,340-347` は、部分レビューのモデルの ID を `claude-fable-[a-z0-9.-]{1,48}` にハードコードし、サブエージェント出力 1 行目の自己申告（`Model: <model-id>`）を必須としている。記録行の接頭辞も `Opus review:`（`_lib/opus-review.md:85,112`）・`Fable review:`（`_lib/fable-review.md:446-451`）とモデル固有名を持つ。
    -   実装計画書の `**実装モデル要件**` の値が `opus`・`sonnet+opus-review`・`sonnet` というモデル固有名になっており（`.claude/commands/_context.md:67`、`.claude/commands/mkplan2.md:41-45`、`internal/chief/task.go:250-274`）、[ADR-0042](../../dev/adr/0042-implementation-model-tiers.md) でも `sonnet` の値が実際には DeepSeek V4.1 Flash を指していて名前と実態が合わない課題があった。
    -   `.claude/commands/_context.md:157-174` の `Command routing` 表と `internal/chief/routing.go:27-34,127-140` は行キーに `runplan-opus` 等のモデル固有名を持ち、エージェントを `Claude Code` と `OpenCode` の 2 値に限定している。
3.  **権限・サンドボックスモデルと監視の一覧の違い:**
    -   Claude Code は `.claude/settings.json:1-50` の接頭辞一致ルールで許可と拒否（`herdr` の書き込み系、`git worktree` の追加・削除・移動、`make install-*`、`go install` の拒否）を強制している（[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)、[ADR-0046](../../dev/adr/0046-chief-of-staff.md)）。
    -   Antigravity CLI は `.claude/settings.json` を読まず、既定の OS サンドボックス（ネットワークとワークスペース外へのアクセスを遮断）と `BypassSandbox: true`（利用者の明示承認）で保護されている。外部の git worktree（`.git` がワークスペース外の本体リポジトリを指す構成）での `git` 操作や、`gh`・`coderabbit review`・`numreserve`・ループバック HTTP テストなどは標準サンドボックスでは拒否される。
    -   また、git が無視する実行関連ファイルの改ざんを検出する `fixprguard` の監視の一覧（`internal/fixprguard/ignored.go:38-42`）と部分レビューの状態チェック（`.claude/commands/_lib/fable-review.md:389`）は `.claude/` と `claude.md` だけを監視しており、Antigravity が読む `.agents/` や `AGENTS.md`・`GEMINI.md` をまだ監視していない。
4.  **コマンドの配置とクロスチェックの部分レビューの一般化:**
    -   Antigravity CLI は `.claude/commands/*.md` をそのままではカスタムスキルとして自動検出しない。
    -   PR を開く前の部分レビュー（[ADR-0050](../../dev/adr/0050-fable-partial-review.md)）自体が `Fable` 固有の名称・手順構成になっており、Antigravity を含む複数ハーネス向けに一般化する必要がある。

**本書の記述範囲:** 本書は、対象とするコマンドとハーネスの範囲、Antigravity からの呼び出しの入口、`**実装モデル要件**` の新しい役割名と旧値の読み替え、上位モデルのレビュー（旧 Opus のレビュー。3.1）・クロスチェックの部分レビュー（旧 Fable の部分レビュー。3.1）・エスカレーション等のハーネス別の振る舞いと検証条件、記録行の互換性、Antigravity のサンドボックス・権限規則と git 無視ファイルの監視拡張、`chief` との境界を定める。次の事項はアーキテクチャ設計書（`02_architecture.md`）で決める（[design_handoff.md](design_handoff.md) に申し送る）。

-   Antigravity の各役割モデルの具体的な選定およびクロスチェックの部分レビュー担当モデルの評価と選定根拠（H-01）。
-   ハーネスのアダプタ（ハーネスごとのツール・検証・権限規則の差分を定義する手順文書。3.1）と `.agents/skills/` 配下の薄い入口の具体的なファイル構成・抽象操作の記述形式・ガードテストの実装方法（H-02）。
-   一般化した手順ファイル名・モード名・コミット記録行の接頭辞・不一致理由コード・PR の説明の見出しの具体的な文字列と、`origin/main` からスクリプトを取り出す移行順序（H-03）。
-   外部 git worktree 上での `swarm` ワーカーによる `git` 実行可否の実測確認と差分の渡し方、`swarm(action: "add")` / `status` の返却スキーマ確認、一般サブエージェントの種別選択、`schedule` タイマー条件の詳細（H-04）。
-   `_context.md` のハーネス別モデル対応表の具体的な節見出しと列構成（H-05）。

**issue #1072 で決めたこと:**

以下は、issue 本文およびコメント 1・2 で定まっていた事項である。

-   **モデル指定の役割への抽象化:** 計画書・手順書からは具体的なモデル名を切り離して役割名で表し、既存計画書の旧値との後方互換を保ち、`_context.md` にハーネス別の役割と実モデルの対応表を置いて判定スクリプトがそれを参照する。
-   **ハーネス差分のアダプタ分離:** 質問ツール・サブエージェント起動・スクラッチパッド・サンドボックスと権限の注意点をハーネス別のアダプタ文書へ切り出し、コマンド定義の本体には抽象化した操作を書く。ハーネスの判別は利用可能なツールで決定的に行う（issue コメント 1）。
-   **Antigravity での部分レビューのモデル固定と検証方法（issue コメント 1）:** Antigravity ではクロスチェックの部分レビューに `invoke_subagent`（`Model: "pro"` を含む）を使わず、起動前に `swarm(action: "models", all: true)` で `_context.md` 指定モデルの存在と割り当ての残りを確認する（利用不可なら別モデルへフォールバックせず `not run` として記録する）。利用可能なら `swarm(action: "add", ...)` で起動し、サブエージェント本文の自己申告ではなく返却 JSON の `"model"` フィールド（必要なら `status`）が指定モデルと完全一致することを検証し、不一致なら即座に `swarm(action: "kill")` してモデル不一致の `not run` で終える。待機・30 分タイムアウト（`schedule`）・結果取得（`read_board`）・終了時の `swarm(action: "kill")` を使い、worktree の状態チェックは維持する。
-   **Antigravity のクロスチェック担当モデルの評価方針（issue コメント 2）:** Argon だけでなく Carbon も選択肢となるため、設計段階（`02_architecture.md`）で比較評価して採用するモデルを 1 つに決める。

**要件定義の際に決めたこと（2026-11-21）:**

以下は、起草とセルフレビューの際に利用者に確認して決めた事項と、それぞれの理由および ADR の判断である。

-   **進め方の選択:** 通常の進め方（`full`）にする。簡易な進め方の除外（`docs/dev/conventions/requirements_process.md:551-568`）に当たると判断し、推奨どおり利用者が通常の進め方を選んだ。除外と判断した理由は、開発ツールの信頼の境界とレビューの関門に関わる判定スクリプト（`_lib/opus-review-decision.sh`・`_lib/review-records.sh`）、`fixprguard` の監視の一覧、モデル検証の手順、権限とサンドボックスの扱いに変更が及ぶためである（[ADR-0013](../../dev/adr/0013-review-setup.md)、[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)、[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)、[ADR-0050](../../dev/adr/0050-fable-partial-review.md)）。10 を超えるコマンド定義と複数の ADR にまたがり、設計段階でモデル評価と外部 worktree の実測を行うため、段階ごとに合意する方が手戻りを防げる。ADR の判断: ADR なし（このタスク自身の進め方の選択であり、タスクをまたぐ設計決定ではないため）。
-   **`chief` とのスコープ境界とルーティング行キーの改名（F-007）:** `chief` の Antigravity 操作対応（Herdr 連携、`transcript.jsonl` 読み取り、pane への入力送信、`/chief` 自体の Antigravity 実行）は別タスクに切り出し、本タスクでは計画書の新しい役割名の読み取り追随、`Command routing` の行キー改名（`runplan-high`・`runplan-standard+high-review`・`runplan-low`）、および `chief` のプロトコル番号更新（`main` の値 + 1）までを行う。`chief` の Antigravity 操作対応は実画面と記録の標本採取を伴う独立した大きな変更であるため分離し、一方で `_context.md` と `chief` の契約変更に伴うプロトコル番号更新とモデル固有名の行キー解消は同時に行う方が不整合を残さないためである。ADR の判断: [ADR-0046](../../dev/adr/0046-chief-of-staff.md) を更新し、[ADR-0055](../../dev/adr/0055-harness-independent-commands.md) に境界を記録する。
-   **対象コマンドと `.agents/skills/` の薄い入口（F-001、F-002、F-008）:** `chief` を除く全開発コマンドを対象にし、`.claude/commands/` を正本として `.agents/skills/` に英語の薄い入口を置き、OpenCode は現行の運用範囲（`/runpr` と `low` の `/runplan`）のままにする。手順の二重管理を避けつつ Antigravity からカスタムスキルとして直接呼び出せるようにするためであり、毎回 `@[.claude/commands/...]` で参照する案や OpenCode 用の汎用アダプタを新設する案（YAGNI に反する）は採らない。あわせて、`CLAUDE.md` の対話言語規則と英語指示文書の対象（`.agents/skills/` 追加）、および `/runpr` のマージの質問における単一選択質問ツールの記述をハーネス非依存化する。ADR の判断: [ADR-0055](../../dev/adr/0055-harness-independent-commands.md) を新規作成し、[ADR-0035](../../dev/adr/0035-language-policy.md) と [ADR-0047](../../dev/adr/0047-runpr-merge-question.md) を更新する。
-   **`**実装モデル要件**` の新役割名と Antigravity での上位モデルのレビュー（F-003、F-004）:** 値を `high`・`standard+high-review`・`low` の 3 つにし、自己のモデルの ID を検証できない Antigravity では `high` でも `swarm` で上位モデルのレビューを走らせる。最下位の役割（現 `sonnet`）を `low` とすることで 3 段階の役割が一意になり、旧値 6 種もすべて `_context.md` の読み替えの表で衝突なく吸収できる（`high` / `standard+high-review` / `standard` にすると `standard` が 2 つの意味を持つため採らない）。また、モデル検証なしに関門を省略する案は採らず、Antigravity で `**実装モデル要件**` が `high` の PR を実装したときに上位モデルのレビュー 1 回分の追加費用がかかる点を結果として受け入れる。ADR の判断: [ADR-0042](../../dev/adr/0042-implementation-model-tiers.md) を更新する。
-   **エスカレーション判定と明示モデル指定を持つサブエージェントのプロンプト・モデル指定（F-001）:** `_lib/pr-review-escalation.md`（現在 `model: "opus"` 固定）はセッションのモデルを引き継がせずハーネスの `high` 役割固定ワーカーで実行し、`high` 役割ワーカーを呼べないハーネス（OpenCode 等）や検証失敗時は現行規則どおり安全側の `ask`（PR レビュー推奨）とする。あわせて、`japrose.md`（現在 `MODEL: sonnet` 固定）、`fixpr.md` Phase 6（現在 `haiku` 固定）、および `_lib/review-subagent-pattern.md` の確認パスのプロンプトも本タスクでハーネス非依存の役割指定（`standard`・`low`）へ改める。Antigravity では、`_context.md` とハーネスのアダプタにより役割に応じて `swarm(action: "add", model: "<model-id>", ...)`（例: `standard` 役割に `gemini-4-neon-medium` を指定し完了時に `kill` する）と `invoke_subagent` を使い分けられるようにする。エスカレーションをセッション引き継ぎにすると軽量モデルが `skip` を返してレビュー関門を弱め、一方で `japrose` 等の定型チェックリストレビューを `invoke_subagent` の 4 段階列挙だけに縛るとメインセッションの上位モデル（Carbon 等）を過剰に消費するか Flash へ落ちるためである。ADR の判断: [ADR-0013](../../dev/adr/0013-review-setup.md) と [ADR-0055](../../dev/adr/0055-harness-independent-commands.md) に反映する。
-   **Fable 部分レビューの一般化と担当モデルの選定（F-005、F-008）:** 部分レビュー自体をモデル固有名に依存しない「クロスチェックの部分レビュー」として一般化する（手順ファイル名・呼び出し入力名・`review-records.sh` のモード名・記録行・PR の説明の見出し・現在の仕様文書から `Fable` の固有名を外し、既存記録行は両読みする）。担当モデルは Claude Code では `fable` とし、Antigravity では当初 Gemini 系の `gemini-4-carbon-*` / `gemini-4-argon-*` から選ぶ方針としていたが、設計段階（2026-11-22）での運用実績と実測評価を踏まえて Antigravity 組み込みの `claude-opus-5-5-medium` に改めた（直下の「要件定義の決定変更（2026-11-22）」）。利用不可・不一致時は別モデルへフォールバックせず `not run` とし、利用者が手動で Claude 系をメインセッションに選んだ場合に同系統になりうる点は別コンテキスト分離と CodeRabbit CLI で補う受け入れる危険とする。`_context.md` に複数モデルを持たせて順序付き試行する案は複雑さが増えるため採らない。ADR の判断: [ADR-0050](../../dev/adr/0050-fable-partial-review.md) と [ADR-0013](../../dev/adr/0013-review-setup.md) を更新する。
-   **要件定義の決定変更（2026-11-22、Antigravity のクロスチェック担当モデル）:** `02_architecture.md` の比較評価において、`gemini-4-argon-medium` / `high` は 136 PR の実運用（7 日間）で延べ 9 回すべて指摘 0 件と検出力が不足し、`gemini-4-carbon-medium` と `gemini-4-neon-xhigh` は実測で 10 分超タイムアウトかつ割り当て消費が大きかった。一方、Antigravity 組み込みの `claude-opus-5-5-medium` は実 PR `#1076` を 4 分 19 秒で完了し、4 つの範囲基準と `exclusion-error` の計 5 区分すべてにわたって 6 件の具体的な指摘を厳密形式で出力し、通常セッションの Gemini 4 Neon に対する異系統クロスチェックとして機能した。これにより Antigravity のクロスチェック担当モデルを `claude-opus-5-5-medium` に改め、週次リセットの Claude 枠を消費する点（枯渇時は `not run` の `unavailable` で PR を止めない）を受け入れる。ローカルの Claude Code CLI（`claude -p --model fable`）経由の呼び出しは実行境界の設計が別途要るため、[#1082](https://github.com/isseis/yt2column/issues/1082)（`#1072` が前提）に分離する。ADR の判断: [ADR-0050](../../dev/adr/0050-fable-partial-review.md) を更新する。
-   **外部 git worktree における `swarm` ワーカーの `git` 実行方式の扱い（F-004、F-005）:** Herdr の外部 worktree 構成でバックグラウンドの `swarm(workspace: "inherit")` ワーカー自身が `git` を実行できるかは未検証であるため、`02_architecture.md` で実測確認し、ワーカーが直接 `git` を実行できない（または承認待ちで止まる）場合はメインセッションが `git log`・`git diff` をスクラッチパッドへ書き出してワーカーに読ませる方式を採れるようにする。未検証のままワーカー自身の `BypassSandbox` 実行に固定する案は手戻りの危険があるため採らない。ADR の判断: ADR なし（設計で確定する実現手段の選択であり、[ADR-0042](../../dev/adr/0042-implementation-model-tiers.md)・[ADR-0050](../../dev/adr/0050-fable-partial-review.md) の決定を変えないため）。
-   **Antigravity のサンドボックス境界と git 無視ファイルの監視拡張（F-006）:** Claude Code の `.claude/settings.json` は維持し、Antigravity では標準サンドボックス優先・必要最小限の `BypassSandbox` 承認と禁止規則の明文化で守り、リポジトリ設定による機械的 `deny` の不在は受け入れる危険とする。一方で、git が無視する `.agents/` 配下および指示ファイル（`AGENTS.md`・`AGENT.md`・`GEMINI.md`・`AGENTS.override.md`）の改ざん検出は、部分レビュー／上位モデルのレビューの状態チェックと `fixprguard` の監視の一覧の両方に本タスクで追加し、`fixprguard` のプロトコル番号を `main` の値 + 1 に上げる（`fixprguard` を別タスクへ先送りする案は `/fixpr` 経路に監視の穴を残すため採らない）。ADR の判断: [ADR-0055](../../dev/adr/0055-harness-independent-commands.md) と [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) に記録する（`fixprguard` の監視の一覧の細部は `fixprguard_guide.md` が正本であり [ADR-0015](../../dev/adr/0015-untrusted-review-threads.md) の決定は変わらない）。
-   **先行・並行タスク（#1059・#988・#1057）との順序:** #1059（タスク 0118：Codex 撤去）の完了は本タスク実装の前提とし、Codex 系の対応は行わない。#988（タスク 0114：記録後の merge commit 確認）は前提として待たず、先にマージされた側に後から実装する側が合わせる条件付きとする。#1057 は本タスクの後に行う。ADR の判断: ADR なし（タスク間の実施順序であるため）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `chief` を除くすべての開発コマンドを、Claude Code と Antigravity CLI のどちらのセッションからでも同じ品質関門・承認規則・記録契約で実行できるようにする。
-   計画書・コマンド定義・手順ファイル・記録行からモデル固有名（`Opus`・`Sonnet`・`Haiku`・`Fable`）への直接依存を切り離し、`_context.md` の対応表 1 か所でハーネスごとの実モデルを管理できるようにする。
-   Antigravity 上でのモデル固定レビューと `.agents/` ディレクトリ導入において、モデルのすり替わり・自己申告の誤認・git 無視ファイルの改ざんを防ぐ。

### 2.2. スコープ (In Scope)

-   `.claude/commands/`（`chief.md` を除く全コマンドと `_lib/`）のハーネス非依存化と、Claude Code 用・Antigravity 用のハーネスのアダプタの作成（F-001）。
-   `.agents/skills/` 配下への Antigravity 用薄い入口（カスタムスキル）の配置（F-002）。
-   `**実装モデル要件**` の役割名（`high`・`standard+high-review`・`low`）への変更、旧値の読み替え、および `_context.md` へのハーネス別役割→実モデル対応表の追加（F-003）。
-   上位モデルのレビュー（旧 Opus のレビュー）と判定スクリプトの一般化・ハーネス非依存化・新旧記録行の両読み対応（F-004）。
-   クロスチェックの部分レビュー（旧 Fable の部分レビュー）と記録判定スクリプトの一般化・ハーネス非依存化・`swarm` によるモデル固定と検証・新旧記録行の両読み対応（F-005）。
-   Antigravity のサンドボックス実行規則・禁止コマンドの明文化、および状態チェックと `fixprguard` の監視の一覧への `.agents/`・指示ファイルの追加（F-006）。
-   `internal/chief` と `docs/dev/chief_architecture.md` における計画書の新しい役割名の読み取り追随・`Command routing` 行キー改名・プロトコル番号更新（F-007）。
-   現在の仕様を書く文書・ADR（ADR-0055 の作成、ADR-0013・0014・0035・0042・0046・0047・0050 と索引の更新）・用語集・文書のガードの更新（F-008）。

### 2.3. スコープ外 (Out of Scope)

-   **`chief` の Antigravity 操作対応:** Herdr や Hooks による `conversation-id` の紐付け、`transcript.jsonl` / `transcript_full.jsonl` の読み取り、`paneinput` や `Command routing` による Antigravity への送信、および `/chief` 自体の Antigravity 上での実行は別タスクに切り出す（1 章「要件定義の際に決めたこと」）。
-   **OpenCode 用のサブエージェントアダプタの新設:** OpenCode は現行の運用範囲（`/runpr` と `low` の `/runplan`）のまま維持し、全コマンド対応は行わない（同上）。
-   **Codex の PR レビューと `codexreview` の Antigravity 対応:** 前提タスク #1059（タスク 0118）で撤去されるため対象外とする（同上）。
-   **#1057 の計画書・実装 PR における部分レビューの置き換え:** 本タスクの後に別タスクとして実施する（同上）。
-   **Antigravity 向けの機械的コマンド拒否 Hook・プラグインの新設:** 禁止コマンドは指示ファイル・アダプタ文書と `BypassSandbox` の利用者承認で守り、機械的 `deny` の不在は受け入れる危険とする（同上）。
-   **`docs/tasks/` の過去のタスク文書と `docs/research/` の記録の書き換え:** 各タスク・調査時点の記録であり、計画書の旧値は `_context.md` の読み替えの表で扱うため書き換えない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 用語

本章では、次の用語を使う。

-   **ハーネス:** 開発コマンドを実行するエージェント実行環境。本リポジトリでは Claude Code、Antigravity CLI（`agy`）、OpenCode を指す。
-   **ハーネスのアダプタ:** ハーネスごとの組み込みツール、モデルの固定と検証、スクラッチパッドの場所、ファイルの読み書き、シェル実行とタイムアウト、バックグラウンド制御とタイマー、サンドボックスと権限の規則の差分を定義する手順文書。
-   **上位モデルのレビュー:** 実装モデル要件が `high` または `standard+high-review`（および読めない値）のプランの PR を開く前、または `/weakreview` で、ハーネスの `high` 役割のモデルを用いて行う正しさのレビュー・弱いモデルの失敗のレビュー・確認のレビューの組（旧称: Opus のレビュー）。
-   **クロスチェックの部分レビュー:** タスクの PR を開く前に、信頼の境界・先行文書との照合・外部の振る舞いについての技術的な主張・人に問わずに決めたことの 4 つの基準に当たる箇所に絞って、ハーネスのクロスチェック役割のモデルで行う部分レビュー（旧称: Fable の部分レビュー）。
-   **スクラッチパッド:** 作業ツリーの外にある、セッション専用の一時ファイル置き場。

### 3.2. 機能一覧

#### F-001: コマンド定義・共有手順のハーネス非依存化とハーネスのアダプタ

`.claude/commands/` のうち `chief.md` を除くすべての開発コマンドと `_lib/` の共有手順をハーネス非依存の記述に改め、ハーネス固有の操作を Claude Code 用と Antigravity 用のハーネスのアダプタへ分離する。

**Acceptance Criteria**:
- **AC-01**: 対象コマンドは、現時点で `.claude/commands/` にある `mkreq.md`・`mkarch.md`・`mkplan.md`・`mkplan2.md`・`runplan.md`・`runpr.md`・`fixpr.md`・`mergepr.md`・`weakreview.md`・`japrose.md`・`mktrans.md`（および実装時点で `main` にある `chief.md` 以外の利用者が起動するコマンド）と、それらが参照する `_lib/` のすべての共有手順である。これらの手順の正本は `.claude/commands/` に 1 つだけ置かれる。
- **AC-02**: AC-01 のコマンド定義の本体と共有手順（ハーネスのアダプタ自身を除く）は、特定のハーネス固有の組み込みツール名（`Bash`・`Write`・`Read`・`Edit`・`WebFetch`・`WebSearch`・`AskUserQuestion`・`Agent`・`TaskStop`・`ToolSearch`・`PushNotification`・`ask_question`・`invoke_subagent`・`swarm`・`swarm_wait`・`read_board`・`run_command`・`read_file`・`write_file`・`edit_file`・`schedule`・`manage_task`・`search_web` など）を直接呼ばない。代わりに、抽象化された操作（選択式の質問、一般サブエージェント、モデル固定ワーカー、ファイルの読み書き、スクラッチパッド、タイムアウト付きシェル実行、バックグラウンド待機とタイマー、利用者への通知）とハーネスのアダプタへの参照で記述する。
- **AC-03**: ハーネスの判別は、セッションで利用可能な組み込みツールの有無によって決定的に行われる。Claude Code 用と Antigravity 用のどちらのアダプタの条件も満たさないハーネス（OpenCode など）で実行されたときは、質問・ファイル読み書き・シェル実行および OpenCode 自身のサブエージェント機構による `/fixpr` の各フェーズはそのまま動き、モデル固定ワーカーを要する箇所は AC-07（エスカレーションは `ask`）・AC-22（上位モデルのレビューは停止）・AC-28（クロスチェックの部分レビューは `not run` 記録）に従う。
- **AC-04**: 選択式の質問の操作は、Claude Code（`AskUserQuestion`）と Antigravity（`ask_question`）の両方で、1 回の呼び出しにつき最大 4 問、各問 2〜4 個の選択肢、推奨する選択肢があるときは先頭に置いて推奨であることを示す規則で動く。どの選択肢にも当たらない回答を受けたときは、推測で進めずに聞き直す（`.claude/commands/mkreq.md:165-166` の規則を維持する）。
- **AC-05**: スクラッチパッドの操作は、Claude Code と Antigravity（`<appDataDir>/brain/<conversation-id>/scratch/`）のどちらでも、作業ツリーの外のセッション用ディレクトリに一時ファイルを作り、git の追跡対象やコミットに含めない。
- **AC-06**: モデルの ID の厳密な検証を要しない一般のサブエージェント（セルフレビュー、パネルモードのレビュー、`/fixpr` の Triage・Fix・Wrap、`japrose`、`mktrans` の翻訳確認）は、Claude Code と Antigravity の両方で起動・結果回収ができる。発見パスは既定でセッションのモデルを引き継ぐ。軽量・標準段階を明示するプロンプト箇所（`japrose.md:133` の `standard` 役割、`fixpr.md:25` Phase 6 の `low` 役割、および `_lib/review-subagent-pattern.md:30-35,100-110` の段階指定・確認パス）はモデル固有名（`sonnet`・`haiku`）を書かず、`_context.md` とハーネスのアダプタが定める起動手段とモデル指定（Antigravity では `invoke_subagent` の列挙値だけでは指定できない `standard` 役割などへの `swarm(action: "add", model: "<model-id>", ...)` 指定と完了時の `kill` を含む）に対応する。
- **AC-07**: PR レビューのエスカレーション判定（`.claude/commands/_lib/pr-review-escalation.md`）は、セッションのモデルを引き継がず、ハーネスの `high` 役割に固定したワーカー（Claude Code では `Agent`、Antigravity では AC-21 と同じ `swarm` のモデル固定・返却 JSON 検証・終了時の `kill`）で実行する。`high` 役割のワーカーを呼べないハーネス（OpenCode など）、利用不可・割り当て切れ、モデル不一致、または応答が決まった JSON 形式でないときは、安全側に倒して `ask`（マージの質問で PR レビュー依頼を推奨）として扱う。
- **AC-08**: バックグラウンド待機・タイマー・利用者への通知の操作は、ハーネスごとの手段に分離される。Antigravity 用のハーネスのアダプタはタイマーにシェルの `sleep` を使わず `schedule` ツールを使い、通知ツールを持たないハーネスでは `.claude/commands/_context.md:52` の規則どおり最終の報告をもって利用者への通知とする。

#### F-002: Antigravity からのコマンド呼び出しの入口

Antigravity CLI から F-001 の各開発コマンドをカスタムスキルとして呼び出せるように、`.agents/skills/` 配下に薄い入口を置く。

**Acceptance Criteria**:
- **AC-09**: F-001 の対象コマンド（`chief` を除く各開発コマンド）と 1 対 1 に対応する `.agents/skills/<コマンド名>/SKILL.md` が存在する。各ファイルは英語（`CLAUDE.md`「Document language」原則 (b)）で書かれ、対応する `.claude/commands/<コマンド名>.md` と `.claude/commands/_context.md`、および Antigravity 用のハーネスのアダプタを読んで手順を実行するよう指示する。
- **AC-10**: `.agents/skills/<コマンド名>/SKILL.md` は、対応する `.claude/commands/<コマンド名>.md` の各ステップの手順本文を複製しない。コマンドの引数仕様・事前認可（Authorization）・停止条件は、対応する `.claude/commands/<コマンド名>.md` の定義がそのまま適用される。
- **AC-11**: `chief` に対応するスキルは `.agents/skills/` に置かない（`chief` の Antigravity 対応はスコープ外のため）。

#### F-003: 実装モデル要件の役割名への抽象化とハーネス別のモデル対応表

計画書に書く `**実装モデル要件**` の値をモデル固有名から役割名へ改め、旧値の読み替えとハーネス別の実モデル対応表を `_context.md` に一元化する。

**Acceptance Criteria**:
- **AC-12**: 計画書の PR マーカーおよび単一フェーズ計画書の `PR 構成` 節に新しく書く `**実装モデル要件**` の値は、次の 3 つのどれか 1 つである。計画書を作成するコマンド（`mkplan2.md`・`mkplan.md`・簡易な進め方の `mkreq.md`）は、この 3 つの値だけを書く値として挙げる。

  | 値 | 意味 | 当てはめる契機 |
  |---|---|---|
  | `high` | 上位役割（`high`）のモデルで実装し、上位モデルで実装した記録がないコミットがあれば PR を開く前に上位モデルのレビュー（F-004）を行う | 現行の `opus` の契機（`.claude/commands/mkplan2.md:42`。前例のない設計判断、または `mkplan.md` ステップ 8 のパネルモードの契機） |
  | `standard+high-review` | 標準役割（`standard`）のモデルで実装し、PR を開く前に上位モデルのレビュー（F-004）を行う | 現行の `sonnet+opus-review` の契機（`.claude/commands/mkplan2.md:43`。競合する実装方針があるステップ、Conditional checks に 2 つ以上当たるステップ、復旧処理・並行処理・状態機械などの高リスク・複雑なステップ） |
  | `low` | 軽量役割（`low`）のモデルで実装し、上位モデルのレビューは行わない | それ以外（現行の `sonnet` の契機、`.claude/commands/mkplan2.md:44`） |

- **AC-13**: `**判定理由**` は、`high` と `standard+high-review` のときは当たった契機と「`high` より軽いモデルに任せたときに何が起きやすいか」を 1 行の日本語で書き、`low` のときは当たる契機がないことを書く（モデル固有名の「Sonnet に任せると」を必須としない）。
- **AC-14**: `_context.md` の旧値の読み替えの表は、次の 6 つの旧値をそれぞれの新しい値に読み替える。完了した計画書および進行中の計画書の本文にある旧値は書き換えない。

  | 旧値 | 読み替える値 |
  |---|---|
  | `opus` | `high` |
  | `sonnet+opus-review` | `standard+high-review` |
  | `sonnet` | `low` |
  | `frontier-required` | `high` |
  | `frontier-recommended` | `standard+high-review` |
  | `standard` | `low` |

- **AC-15**: 正規の 3 値でも AC-14 の旧値でもない値は、推測して別の値に読み替えず「読めない値」として示し、上位モデルのレビューを行う側に倒す。`**実装モデル要件**` の欄がない計画書は「値なし」として扱い、上位モデルのレビューを行わない（`.claude/commands/_context.md:91-95,127-128` の規則を維持する）。
- **AC-16**: `_context.md` に、ハーネス（Claude Code・Antigravity・OpenCode）ごとの役割と実モデルの対応表が 1 か所だけ置かれる。対応表は、実装・上位モデルのレビュー・エスカレーション・一般サブエージェントの段階指定で使う役割（`high`・`standard`・`low`）とクロスチェックの部分レビューの役割について、ハーネスごとのモデルの指定値・起動手段・検証対象のモデルの ID（または一致規則）を持つ。クロスチェックの部分レビューの担当モデルは、Claude Code では `fable` とし、Antigravity では組み込みの `claude-opus-5-5-medium` とする（`02_architecture.md` の比較評価に基づく決定）。
- **AC-17**: `runplan.md`（開始時の案内）と `runpr.md`（マージの質問と次のコマンドの案内）は、計画書から読んだ `**実装モデル要件**` の値（旧値のときは元の値と読み替えた値の両方）と、`_context.md` の対応表に基づく推奨モデルおよび上位モデルのレビューの見込みを示す。

#### F-004: 上位モデルのレビューと判定スクリプトの一般化・ハーネス非依存化

`/runplan` と `/weakreview` が共有する上位モデルのレビュー（旧 Opus のレビュー）の手順・ファイル名・記録行・判定スクリプトをモデル固有名に依存しない形へ一般化し、Claude Code と Antigravity の両方で動くようにする。

**Acceptance Criteria**:
- **AC-18**: 上位モデルのレビューを行うかどうかを決める判定スクリプトは、レビュー対象ブランチの変更で判定が変わらないよう `origin/main` の版を取り出して実行し、`origin/main` の `_context.md` から新旧の `**実装モデル要件**` の値と、`high` 役割として認めるモデルの ID の規則を読む。スクリプト内に `claude-opus-*` や旧値の一覧をハードコードせず、スクリプトの出力（現行の `all-opus` を含む理由コード）や共有手順のファイル名からもモデル固有名（`opus`）を外す（移行中の `origin/main` 参照互換の扱いは `02_architecture.md` で決める）。git エラー・読めない計画書・不正な引数・複数の値欄などの失敗時は、すべて安全側である `RUN` を標準出力に出して終了コード `0` で終わる。
- **AC-19**: セッションがシステムプロンプトから自身のモデルの ID を確かめられるハーネス（Claude Code）では、`runplan.md` は現行どおり各 `phase group` の終わりに `Implementation model: <START12>..<END12> <model-id>[, ...]` を記録し、除外できるコミット（現行 `.claude/commands/_lib/opus-review.md:102-106`）を除くすべてのコミットが `_context.md` の `high` 役割のモデルの ID だけで記録されているとき、判定スクリプトはレビュー不要を返す。
- **AC-20**: セッションがシステムプロンプトから自身のモデルの ID を確かめられないハーネス（Antigravity）では、`runplan.md` は `Implementation model:` の記録行を書かない。その結果、`high`・`standard+high-review`・読めない値のプランの PR では、有効な完了記録の対象になっていない限り判定スクリプトは `RUN` を返し、PR を開く前に必ず上位モデルのレビューが走る。
- **AC-21**: 上位モデルのレビュー（並列の 2 つの発見レビューと確認のレビュー）の起動・モデル検証・出力先頭行の扱い・待機・停止はハーネスのアダプタが行う。
  1.  Claude Code では `Agent` ツールで `high` 役割モデルを指定し、出力 1 行目の `Model: <model-id>` が `_context.md` の `high` 役割の規則に合うことを確かめる。
  2.  Antigravity では `invoke_subagent` を使わず、プロンプトに `Model:` 行の出力を求めず、出力は 1 行目から指摘（または指摘なしの定型行）として検査する。起動前に `swarm(action: "models", all: true)` で `_context.md` の Antigravity 用 `high` 役割モデルの存在と割り当ての残りを確かめ、並列ワーカーには互いに異なる名前を付けて `swarm(action: "add", ...)` でモデルを固定して起動し、ハーネス返却 JSON の `"model"` が指定モデルの ID とバイト単位で完全一致することを確かめる。待機には 30 分のタイムアウト（`schedule`）と一時的な呼び出し失敗時の 1 回限りの再試行（現行 `.claude/commands/_lib/opus-review.md:402` と同じ回数）を設け、`read_board` でワーカー別の結果を回収する。起動前と、完了・不一致・タイムアウト・エラーのすべての出口で worktree の状態チェック（AC-36 を含む）、全アクティブワーカーの `swarm(action: "kill")`、およびタイマー解除を行う（外部 git worktree での差分の渡し方は `02_architecture.md` の実測結果に基づき決定する）。
- **AC-22**: Claude Code と Antigravity のどちらでも、上位モデルのレビューを指定された `high` 役割のモデルで完了できないときは、別のモデルで代替せず作業を止めて理由を報告し、`/runplan` では PR を開かず何も push しない（`/weakreview` でも修正コミットを作らず止まる）。対象となる失敗条件は、ハーネスがモデル固定起動を持たないとき（OpenCode など）、モデルが存在しないか割り当て切れのとき、検証したモデルの ID が一致しないとき、再試行後もエラーまたはタイムアウトになったとき、および状態チェックで差分が出たときの 5 つである。
- **AC-23**: 上位モデルのレビューが新しくコミットメッセージに書く完了記録と退けた指摘の記録行は、モデル固有名（`Opus`）を含まない役割名に基づく接頭辞を使う（具体的な文字列は `02_architecture.md` で決める）。判定スクリプト・PR の説明の生成・部分レビューに渡す決着済み論点の読み取りは、新しい記録行と既存の `Opus review: done` / `Opus review: kept` の行の両方を同じ意味で読む。先頭列から始まらない行や文法に合わない行は補正せず記録なしとして扱う。
- **AC-24**: 上位モデルのレビューのワーカー／サブエージェントの出力は信頼しないデータとして扱い、出力の文面をコミットメッセージ・SUMMARY・PR の説明に写さない。コミット前のメンション確認（現行 `.claude/commands/_lib/opus-review.md:135-143`）、セッションが自分の判断で Critical・Major の指摘を退けず利用者に尋ねる規則（同 `:323-336`）、および `/weakreview` の引数仕様と作業ツリー検査（既定で修正してコミットし push しない、`--no-fix` で報告のみ、`--fix` を無操作で受理、未コミットの変更が残る作業ツリーでは停止。`.claude/commands/weakreview.md:18-48`）は現行どおり維持する。

#### F-005: クロスチェックの部分レビューと記録判定スクリプトの一般化・ハーネス非依存化

タスクの PR を開く前に行う部分レビュー（現行の `_lib/fable-review.md`）と有効性判定スクリプト（`_lib/review-records.sh`）を、モデル固有名（`Fable`）に依存しないクロスチェックの部分レビューとして一般化し、Claude Code と Antigravity の両方で動くようにする。

**Acceptance Criteria**:
- **AC-25**: クロスチェックの部分レビューの共有手順は、ファイル名・呼び出し元（`_lib/local-cli-review.md` や `runplan.md`）の入力パラメータ名を含めてモデル固有名（`fable` / `FABLE`）を含まない役割名に基づく名前に改める。本体は、有効性の判定、4 つの範囲基準（`trust-boundary`・`prior-docs`・`external-claim`・`undecided-choice`）と除外箇所の検証、指摘ブロックの形式検査、worktree の状態チェック（AC-36 を含む）、ローカルの CLI レビューとの統合トリアージと却下根拠の確認、記録行の規則をハーネス共通の手順として持ち、起動・モデル検証（先頭の `Model:` 行の有無を含む）・タイムアウト待機・停止処理をハーネスのアダプタへ委譲する。
- **AC-26**: Claude Code 用のアダプタは、`Agent` ツールで `_context.md` 指定のクロスチェックモデル（`fable`）を起動し、プロンプトで求めた出力 1 行目の `Model: <model-id>` が `_context.md` の規則に合うことを確かめ、30 分タイマーと `TaskStop` による停止を行う現行の動作を保つ。
- **AC-27**: Antigravity 用のアダプタは、次のすべてを守る。
  1.  クロスチェックの部分レビューに `invoke_subagent`（`Model: "pro"` を含む）を使わない。
  2.  起動前に `swarm(action: "models", all: true)` で `_context.md` 指定のクロスチェックモデル（`claude-opus-5-5-medium`）の存在と割り当ての残りを確かめる。存在しないか割り当てがないときは、別モデルへフォールバックせず `not run`（理由 `unavailable`）とする。
  3.  利用可能なときは、プロンプトに `Model:` 行の出力を求めず（出力は 1 行目から指摘ブロックまたは `No findings.` として検査する）、`swarm(action: "add", name: "<worker>", model: "<model-id>", workspace: "inherit", brief: ...)` で起動し、返却 JSON の `"model"`（`add` のスキーマが `"model"` を持たない場合のみ起動直後の `status`）が `_context.md` 指定モデルの ID とバイト単位で完全一致することを確かめる。一致しないか読めないときは即座に `swarm(action: "kill")` で終了させ、別モデルで代替せず `not run`（モデル不一致の理由）とする（外部 git worktree での差分の渡し方は `02_architecture.md` の実測結果に基づき決定する）。
  4.  待機には `swarm` の待機またはターン終了と `schedule` による 30 分（1,800 秒）のタイムアウトを使い、並列に走る別タスク（CodeRabbit CLI など）の完了通知でタイマーが早期解除されないようにし、結果の取得には `read_board` を使う。完了・エラー・タイムアウト・呼び出し元停止のすべての出口でタイマーの明示的な解除と `swarm(action: "kill")` によるワーカー終了、および worktree の状態チェックを行う。1 回目の呼び出しがエラーまたは利用上限で終わったときの 1 回限りの再試行規則（現行 `.claude/commands/_lib/fable-review.md:182-187`）を保ち、ハーネスがワーカーのトークンの使用量を返さないときは経過秒数と `tokens unknown` を記録する。
- **AC-28**: クロスチェックの部分レビューを呼べないハーネス（OpenCode など）では、現行どおり `not run`（理由 `agent-cannot-call`。`.claude/commands/_lib/fable-review.md:142-145`）を記録して進む。関連タスク 0114（#988）の記録後のマージコミットに対するマージ後の確認（[ADR-0050](../../dev/adr/0050-fable-partial-review.md) 決定 5）については、#988 が本タスクの該当実装より先に `main` に入っていれば、本タスクでハーネスの `high` 役割ワーカー（Antigravity では `swarm` 固定・検証、走らせられないときは `not run` 記録）へ一般化する。本タスクが先に `main` に入った場合は、#988 側が本タスクのハーネスのアダプタの上で実装する。
- **AC-29**: 新しく書く部分レビューの記録行（実施・未実施・範囲に入れた箇所・外した箇所・指摘・チェックポイント）の接頭辞、モデル不一致の `not run` 理由コード、および PR の説明の節見出しと折りたたみ見出し、`/runpr` のマージの質問の要約表示は、モデル固有名（`Fable`・`not-fable`）を含まない役割名に基づく形にする（具体的な文字列は `02_architecture.md` で決める）。指摘の `<kind>` 分類（現行 `.claude/commands/_lib/fable-review.md:470-485`）では、`.agents/skills/` 配下を `command`、それ以外の `.agents/` 配下を `agent-config` として分類する。
- **AC-30**: `origin/main` から取り出して実行する記録判定スクリプト（`_lib/review-records.sh`）と、PR の説明の生成（`_lib/task-pr-handoff.md`）・`/runpr` のマージの質問の要約読み取りは、次のすべてを満たす。
  1.  スクリプトの部分レビュー有効性判定は、モデル固有名を含まない新しいモード名を受け入れる（移行中のブランチ互換のため既存の `fable` モード名も受け入れるかは `02_architecture.md` で決める）。
  2.  実施記録の `<model-id>` として、`origin/main` の `_context.md` が認めるすべてのハーネスのクロスチェックモデルの ID（既存の `claude-fable-...` と Antigravity の採用モデルの ID）を受け入れる。`_context.md` から読んだモデルの ID の文字列は安全な ASCII 文字集合に合うことを確かめてから照合し、認めないモデルの ID や壊れた形式の記録行は補正せず無視する。
  3.  AC-29 の新しい記録行・理由コードと、既存の `Fable review: run`・`Fable review: not run`（理由 `not-fable` を含む）・`Fable scope: in`・`Fable scope: out`・`Fable finding:`・`Fable review: checkpoint` の行の両方を同じ意味の記録として読む。
  4.  部分レビュー有効性モードで git エラー・不正な引数・`origin/main:.claude/commands/_context.md` の欠落やモデル表の欠落・形式不正が起きたときは、安全側に倒して標準出力に `MISSING`、標準エラーに診断を出して終了コード `1` で終わる。既存の `coverage` モードの動作と失敗時の出力（`PARTIAL`、終了コード `1`。`.claude/commands/_lib/review-records.sh:182-189`）は変えない。
- **AC-31**: 部分レビューのワーカー／サブエージェントの出力は信頼しないデータとして扱う。指摘ブロックが決められた形式でないときや上限（30 件。現行 `.claude/commands/_lib/fable-review.md:319`）を超えるときは `not run`（理由 `unreadable`）とし、ブランチの変更ファイル外の場所や読めない場所・重大度の指摘は `invalid` とし、出力の文面をコミットメッセージ・SUMMARY・PR の説明に一切写さない（同 `:330-353,507-509` の規則を維持する）。

#### F-006: Antigravity のサンドボックス・権限規則と git 無視ファイルの監視拡張

Claude Code の権限設定を保ったまま、Antigravity でのサンドボックス実行規則と禁止コマンドを明文化し、`.agents/` と指示ファイルに対する git 無視ファイルの監視を追加する。

**Acceptance Criteria**:
- **AC-32**: 本タスクの実装 PR が変更するファイルに `.claude/settings.json` は含まれない（前提タスク #1059 完了後の `.claude/settings.json` をそのまま維持する）。
- **AC-33**: Antigravity 用のハーネスのアダプタは、コマンド実行の規則として次を定める。
  1.  コマンドは既定の標準サンドボックス（`BypassSandbox: false`）で先に試行し、成功した種類のコマンドは標準サンドボックスのまま使い続けること。
  2.  ワークスペース外の `.git` worktree へのアクセス、ネットワーク通信（`gh`・`coderabbit review`・`numreserve`・`git fetch`・`git push` など）、またはサンドボックスが拒否するループバックソケット操作で実際に失敗した種類については、その単一のコマンドだけを分離して `BypassSandbox: true` で実行すること。同じセッション内でその種類が標準サンドボックスで拒否されることを確認した後は、同種の単一コマンドに直接 `BypassSandbox: true` を指定してよく、他の種類のコマンドは引き続き `BypassSandbox: false` で実行する。また、リポジトリが定める判定・状態チェック用のスクリプトファイルを `bash <script>` として 1 回起動する呼び出しは単一のコマンドとして扱い、前後のファイル編集・ビルド・テストを `&&` や `;` で即席に繋いで一緒に昇格させることは禁じる。
  3.  再試行時は `toolSummary` を前回と同一に保ち、利用者の自動承認（前方一致）が効くよう不要なコマンド置換 `$(...)` やラッパー（`env`・`eval` 等）を避けること。ただし、PR タイトルに環境変数展開を起こさせないための `.claude/commands/_context.md:209-216` の `--title "$(cat "<title-file>")"`（または設計で定める同等の安全な渡し方）は、シェル展開による秘密漏洩を防ぐ規則を優先する。
- **AC-34**: `AGENTS.md`・`CLAUDE.md`・Antigravity 用のハーネスのアダプタからたどれる指示として、次の禁止規則が Claude Code だけでなく Antigravity セッションにも適用されることが明記される。
  1.  インストール型ツール（`fixprguard`・`mergepr`・`chief`・`numreserve`）をインストールもビルドもしないこと（`docs/dev/conventions/dev_tools.md` Rule 5 の開発のための例外を除く）。
  2.  `herdr` の書き込み・状態変更・作成・削除系サブコマンド、および `git worktree` の追加・削除・移動を実行しないこと（`docs/dev/guides/chief_guide.md` 6 章の一覧）。
  3.  承認された `numreserve task`・`numreserve adr` 以外の手段で記録タグ（`refs/tags/task-number/`・`refs/tags/adr-number/`）やその ruleset を作成・変更・削除しないこと、および事前承認のないネットワーク操作・PR マージ・実 LLM API 呼び出しを行わないこと（`CLAUDE.md`「Tool Execution Safety」）。
- **AC-35**: Antigravity には `.claude/settings.json` のようなリポジトリ内の設定ファイルによる機械的なコマンド拒否（`deny`）がないため、AC-34 の禁止規則が指示ファイル・アダプタ文書と `BypassSandbox: true` 時の利用者承認によって守られることと、エージェントが指示に反する残余の危険を受け入れることが、[ADR-0055](../../dev/adr/0055-harness-independent-commands.md) と [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) に記録される。
- **AC-36**: git が無視する対象のファイルの改ざんを検出する監視の一覧として、`fixprguard`（`internal/fixprguard/ignored.go`）と F-004・F-005 の worktree 状態チェックの両方が、既存の監視対象に加えて最上位の `.agents/` 配下のすべてのパス、および任意の階層の `agents.md`・`agent.md`・`gemini.md`・`agents.override.md`（大文字・小文字を区別しない一致、`internal/fixprguard/scope.go:54` の指示ファイル名と揃える）を監視対象に含める。`fixprguard` のプロトコル番号を `main` の値 + 1 に上げ、呼び出し箇所と `docs/dev/guides/fixprguard_guide.md` を同じ PR で更新する。

#### F-007: `chief` の計画書読み取り追随とルーティング行キーの改名

`chief` が新しい `**実装モデル要件**` の値を持つ計画書と改名後のルーティング行キーを正しく読めるように追随させ、プロトコル番号を更新する。

**Acceptance Criteria**:
- **AC-37**: `internal/chief`（`internal/chief/task.go`）が計画書から読む `**実装モデル要件**` は、`high`・`standard+high-review`・`low` を正規の 3 値として受け入れ、`origin/main` の `_context.md` の読み替えの表から AC-14 の 6 つの旧値をそれぞれの正規の値に読み替える。未知の文字列、同じ節の中の複数の値欄、閉じられたコードフェンス内の値欄は補正せず `RequirementUnreadable` としてその PR の `/runplan` を自動送信せず、閉じられていないコードフェンスや重複した PR マーカー見出しは現行どおり番兵エラー `errTaskDocument` を返してそのワークスペースを「要確認」とする。
- **AC-38**: 同じ計画書群に対する `internal/chief` の計画書マーカー読み取り結果と、F-004 の判定スクリプトの計画書マーカー読み取り結果が、新値（`high`・`standard+high-review`・`low`）・AC-14 の全旧値・読めない値・値なしのすべてで一致する。
- **AC-39**: `internal/chief/routing.go` と `_context.md` の `Command routing` 表の `/runplan` 用行キーは、`runplan-high`・`runplan-standard+high-review`・`runplan-low` に改名される（他の 3 行 `task-documents`・`runplan-takeover`・`runpr` は維持する）。表は引き続き `Claude Code` と `OpenCode` の送り先だけを保持し、未知のエージェント名・未知のモデル名・欠けた行・重複した行・余分な列を含む表を番兵エラー `errRouting` で全体として拒否し、何も送信しない。AC-16 で `_context.md` に追加するハーネス別のモデル対応表は、`ParseRouting` と `ParseOldValues` のどちらの読み取りも壊さない。
- **AC-40**: `internal/chief` のプロトコル番号を `main` の値 + 1 に上げ、`.claude/commands/chief.md`・`docs/dev/guides/chief_guide.md`・`docs/dev/chief_architecture.md` の該当記述を同じ PR で更新する。

#### F-008: 現在の仕様を書く文書・ADR・文書のガードの更新

現在の仕様を書く文書、ADR、および文書のガードを本タスクの決定に揃える。

**Acceptance Criteria**:
- **AC-41**: 現在の仕様を書く文書（`CLAUDE.md`・`AGENTS.md`・`.claude/commands/`・`.agents/skills/`・`docs/dev/`・`docs/translation_glossary.md`）は、現在の仕組みの名前として `Opus review` / `Opus のレビュー` や `Fable partial review` / `Fable の部分レビュー` を書かず、一般化した名称（上位モデルのレビュー、クロスチェックの部分レビュー等）で記述する（`_context.md` の Claude Code 用モデル指定値、旧値・旧記録行の後方互換規則、過去の経緯・採らなかった案の出典としてモデル名を挙げる箇所は除く）。あわせて、同じ PR で次の 4 点を更新する。
  1.  `CLAUDE.md` の「Document language」原則 (b) の対象に `.agents/skills/` を加える。
  2.  `CLAUDE.md` の「Language」の対話言語の規則（日本語）を特定のハーネス名に限定されない記述に改める。
  3.  `CLAUDE.md` の「Commands」の節で、ハーネス非依存化されたコマンドと `.agents/skills/` の入口を説明する。
  4.  判定スクリプトのファイル名が変わる場合は、`CLAUDE.md`「Go Idioms」の日本語リテラル例外（`CLAUDE.md:936-952`）と `docs/dev/conventions/dev_tools.md` Rule 6 を更新する（新たに例外を取る第 3 のシェルスクリプトは増やさない）。
- **AC-42**: `.agents/skills/` 配下のすべての Markdown ファイルが、既存の `.claude/commands/` と同様に、(1) 相対リンクと `docs/dev/` パス参照の実在検査、(2) コマンド指示文書の信頼境界検査（禁止メンション、生の投稿コマンド、直接の `herdr` 呼び出し、未承認形の `numreserve` 呼び出しが含まれないこと）、および (3) `chief` を除く開発コマンドとの 1 対 1 対応と `chief` スキルが存在しないことの検査の対象になる。
- **AC-43**: すべての単体テスト・文書のガード・静的検査（`make test`・`make lint`・`make deadcode`・`make readme-sync-check`、および CI の `doc-guard` ジョブの対象パッケージ）が成功する。
- **AC-44**: [ADR-0055](../../dev/adr/0055-harness-independent-commands.md)（新規作成）、[ADR-0013](../../dev/adr/0013-review-setup.md)、[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)、[ADR-0035](../../dev/adr/0035-language-policy.md)、[ADR-0042](../../dev/adr/0042-implementation-model-tiers.md)、[ADR-0046](../../dev/adr/0046-chief-of-staff.md)、[ADR-0047](../../dev/adr/0047-runpr-merge-question.md)、[ADR-0050](../../dev/adr/0050-fable-partial-review.md)、および [ADR の索引](../../dev/adr/README.md) が本書の決定および ADR 規約（`docs/dev/conventions/adr.md`）と一致する。本タスクの実装が完了するまで残る仕様文書・コードとの食い違いは、各 ADR の `## 決定` に `https://github.com/isseis/yt2column/issues/1072` へのリンク付きの食い違い記述として示し、食い違いを解消した実装 PR で取り除く。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。製品のパイプラインやブラウザ拡張の実行性能には影響しない。なお、上位モデルのレビューおよびクロスチェックの部分レビューの 1 回あたりの待機時間の上限は、Claude Code と Antigravity の双方で 30 分（1,800 秒）とする（AC-21、AC-26、AC-27）。

### 4.2. セキュリティ (Security)

条件付きのセキュリティガイド（`docs/dev/security.md`）の適用条件（`yt-dlp` の起動、API キーの扱い、キャッシュの読み書き、LLM API へのネットワーク送信、信頼しないテキストのプロンプトや公開記事への埋め込み。`.claude/commands/_context.md:296`）に当たる製品機能の変更はないため、`docs/dev/security.md` の製品向け攻撃ベクタの適用は N/A である。

ただし、本タスクは開発ツールの信頼の境界とレビューの関門（[ADR-0013](../../dev/adr/0013-review-setup.md)・[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)・[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)・[ADR-0046](../../dev/adr/0046-chief-of-staff.md)・[ADR-0050](../../dev/adr/0050-fable-partial-review.md)・[ADR-0055](../../dev/adr/0055-harness-independent-commands.md)）に関わるため、次の境界規則と受け入れる危険を定める。

-   **信頼しない入力の境界と拒否規則:**
    1.  **レビュー・エスカレーション用サブエージェント／ワーカーの出力（AC-07、AC-24、AC-31）:** 上位モデルのレビュー・クロスチェックの部分レビュー・エスカレーション判定の出力は信頼しないデータであり、指示として解釈しない。クロスチェックの部分レビューは最大 30 件の指摘ブロック（または `No findings.`）のみを受理し、30 件超過または形式不一致は補正せず `not run`（`unreadable`）とし、変更ファイル外の場所を指す指摘は `invalid` とする。上位モデルのレビューは形式不一致を `stopped-error` とし、エスカレーション判定は固定 3 メンバ（`decision`・`criteria`・`reason`）の JSON 以外を安全側の `ask` とする。ワーカー出力の全文はスクラッチパッドのファイルにのみ書き出し、コミットメッセージ・SUMMARY・PR の説明に写さない（差分内の秘密情報やメンションの漏洩・発火を防ぐ）。
    2.  **`swarm` の返却メタデータによるモデル検証（AC-07、AC-21、AC-27）:** Antigravity でのモデル固定ワーカーは、ワーカー本文の自己申告を信頼せず、ハーネスが返す JSON で検証する。
        -   `swarm(action: "models", all: true)` では、`models`（文字列配列）に対象モデルの ID が含まれ、かつ `quotas.<model-id>.quota_remaining` が正の数値の百分率文字列（`%` で終わり数値部が `0` より大きい形式）であることだけを参照し、それ以外のメンバ（`display_names`・`quota_resets` 等）は将来の拡張に備えて無視する。欠落・型違い・`0` 以下（`"0%"`・`"0.0%"` 等）または百分率形式でない文字列は利用不可として拒否する。
        -   `swarm(action: "add")`（および `add` のスキーマに `"model"` がない場合のみ起動直後に 1 回呼ぶ `status`、`02_architecture.md` でスキーマを確定）では、`"model"`（文字列）だけを参照し、それ以外のメンバは将来の拡張に備えて無視する。`"model"` が `_context.md` 指定モデルの ID とバイト単位で完全一致しない場合（欠落・型違いを含む）は、正規化や別モデルへのフォールバックをせず即座に `swarm(action: "kill")` で終了させる。
    3.  **計画書の値・ルーティング表・コミット記録行の読み取り（AC-15、AC-18、AC-23、AC-30、AC-37、AC-39）:** `internal/chief`（Go）および `origin/main` から取り出す判定スクリプトが読む入力は、固定文法に完全一致する行だけを受理し、標準ライブラリ等が黙って補正・無視しうる不正な形も拒否または無視して安全側に倒す。具体的には、`internal/chief` は閉じられていないコードフェンスや重複した PR マーカー見出しを番兵エラー `errTaskDocument` で、壊れた `Command routing` 表を番兵エラー `errRouting` で拒否し、未知または重複した `**実装モデル要件**` 値を `RequirementUnreadable` として自動送信を止める。判定スクリプトは失敗時に上位モデルのレビューを `RUN`、部分レビューの有効性を `MISSING`（終了コード `1`）、`coverage` を `PARTIAL`（終了コード `1`）とする。リポジトリ内の計画書とコミット履歴を対象とするため、新たなサイズ上限は設けない。
-   **git 無視ファイルを経由したスキル・指示ファイルの永続化防止（AC-36）:** `.agents/` 配下や `AGENTS.md`・`GEMINI.md` 等に自己無視 `.gitignore` 付きのファイルが置かれても検出できるよう、`fixprguard` の監視の一覧とモデル固定レビューの状態チェックの両方にこれらを加える。
-   **外部サービスへのコードと差分の送信:** Antigravity で開発コマンド・サブエージェント・`swarm` を実行すると、リポジトリのコード・差分・タスク文書・コマンド定義が Antigravity の接続先モデル提供元へ送信される（Claude Code が Anthropic へ、OpenCode が DeepSeek へ送信するのと同じ位置づけ）。利用者がそのハーネスでコマンドを起動したことでこの送信を承認したものとする。
-   **サンドボックス・権限境界と受け入れる危険（AC-32〜AC-35）:** Antigravity では既定の OS サンドボックスを優先し、必要な単一コマンドにだけ `BypassSandbox: true` で利用者の承認を受ける。Antigravity には `.claude/settings.json` のようなリポジトリ内の機械的 `deny` 設定がないため、禁止コマンド（インストール型ツールのインストール・ビルド、`herdr` と `git worktree` の変更操作）に対してエージェントが指示に反する残余の危険を受け入れる。
-   **クロスチェックのモデル系統・割り当て枠に関する受け入れる危険:** Antigravity の通常のメインセッションは Gemini 4 Neon 系であるため、`claude-opus-5-5-medium` の採用により異系統クロスチェックが成立する。ただし、メインセッションのモデルの ID をセッション側から検証できないため、利用者が手動で Claude 系のモデルをメインセッションに選んだ場合は同系統または同一モデルになりうる。別ワーカーの独立した文脈での検証と CodeRabbit CLI による別系統レビューで補うものとし、この危険を受け入れる。また、Antigravity 組み込みの `claude-opus-5-5-medium` は週次リセットの Claude 枠を消費するため、枠が尽きた場合は `not run`（`unavailable`）として PR を止めずに進む運用を受け入れる（[ADR-0050](../../dev/adr/0050-fable-partial-review.md)）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   上位モデルのレビューが指定モデルで走らせられないときは、不十分なレビューのまま PR が開かれないよう確実に停止し、何も push しない（AC-22）。
-   クロスチェックの部分レビューは追加の防御層であるため、モデル利用不可・割り当て切れ・30 分のタイムアウト・モデル不一致のときはワーカーとタイマーを確実に片付けたうえで `not run` と理由を記録し、PR の作成を止めない（AC-27）。
-   エスカレーション判定が指定モデルで走らせられないときは、安全側の `ask` として扱い、マージの質問で PR レビュー依頼を推奨する（AC-07）。
-   サブエージェントや `swarm` ワーカーが実行中に worktree の状態（git 無視の `.claude/`・`.agents/`・指示ファイルを含む）を変えたときは、状態チェックが差分を検出して結果を使わずに停止する（AC-21、AC-25、AC-27、AC-36）。

### 4.4. 互換性 (Compatibility)

-   **進行中および過去のブランチ・計画書との互換性:**
    -   既存の計画書にある 6 種類の旧値（`opus`・`sonnet+opus-review`・`sonnet`・`frontier-required`・`frontier-recommended`・`standard`）は `_context.md` の読み替えの表で新しい 3 値として読まれる（AC-14）。
    -   既存のコミットメッセージにある `Opus review:`・`Fable review:`・`Fable scope:`・`Fable finding:`・`Fable review: checkpoint` および理由 `not-fable` は、更新後の判定スクリプト・PR の説明の生成・`/runpr` でも引き続き有効な記録として読まれる（AC-23、AC-30）。
-   **OS・実行環境:** リポジトリの既存の前提（macOS および Linux、`bash` 4 以上、`jq` が `PATH` にあること。`CLAUDE.md:806-816`）を維持する。`_lib/review-records.sh` が bash 3.2 互換を保っている契約（`.claude/commands/_lib/review-records.sh:70-71`）も壊さない。
-   **前提・並行タスクとの順序と再インストール:**
    -   本タスクの実装 PR は、前提タスク #1059（タスク 0118：Codex 撤去）の実装が `main` に入った後にマージする。
    -   タスク 0114（#988：記録後の merge commit 確認）とは互いを前提にして待たず、後から実装をマージする側がもう一方に合わせる（AC-28）。
    -   `fixprguard` と `chief` のプロトコル番号を `main` の値 + 1 に上げる PR では、`CLAUDE.md`「Protocol numbers」に従い利用者が対象ツールを再インストールする（AC-36、AC-40）。

### 4.5. 保守性 (Maintainability)

-   手順の本文は `.claude/commands/` に 1 か所だけ保ち、`.agents/skills/` は薄い参照ラッパーにとどめることで、コマンド手順の変更時に 2 か所の本文を同期する保守負担を作らない（AC-01、AC-10）。
-   ハーネスごとの実モデルの ID は `_context.md` の対応表 1 か所に集約し、モデルの版や採用モデルが変わったときは `_context.md` の該当行の更新だけで済むようにする（AC-16、AC-18、AC-30）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   `CLAUDE.md` の「Document language」「Language」「Go Idioms」「Protocol numbers」「Testing Strategy」「Tool Execution Safety」および `docs/dev/conventions/dev_tools.md` に従う。
    -   `CLAUDE.md:48-75`（「Document language」原則 (b)）および `CLAUDE.md:161-165`（「Language」の会話言語）に対する対象拡張（`.agents/skills/` の追加、および対話言語規則のハーネス非依存化）は 1 章「要件定義の際に決めたこと」に記録したとおりであり、[ADR-0035](../../dev/adr/0035-language-policy.md) を更新する（AC-41、AC-44）。
    -   `CLAUDE.md:936-952`（「Go Idioms」のシェルスクリプトに対する日本語リテラル例外）および `docs/dev/conventions/dev_tools.md` Rule 6 の境界を守り、`.claude/commands/` 配下に日本語リテラルを持つ新しい第 3 のシェルスクリプトは追加しない（既存スクリプトを改名する場合は同箇所のファイル名を同じ PR で更新する。AC-41）。
    -   `fixprguard` と `chief` のプロトコル番号を上げる変更（AC-36、AC-40）では、`CLAUDE.md:826-871`（「Protocol numbers」）に従い、タスク文書には具体的な番号を書かず「`main` の値 + 1」と記す。
-   承認済み先行タスクの決定に対する変更（タスク 0081 の [ADR-0042](../../dev/adr/0042-implementation-model-tiers.md)、タスク 0088・0115・0117 の [ADR-0046](../../dev/adr/0046-chief-of-staff.md)、タスク 0089・0118 の [ADR-0047](../../dev/adr/0047-runpr-merge-question.md)、タスク 0098・0114・0118 の [ADR-0050](../../dev/adr/0050-fable-partial-review.md)、タスク 0112・0118 の [ADR-0013](../../dev/adr/0013-review-setup.md)・[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)）と新規の [ADR-0055](../../dev/adr/0055-harness-independent-commands.md) の作成は 1 章に記録したとおりであり、すべての ADR 更新を本 PR に含める。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する（「通常の進め方」「簡易な進め方」「進め方の記録」「プランの PR」「割り当て」「試行」「指摘」「確認」「照合する」「モデルの ID」「開発ツール」「インストール型ツール」「ガードツール」「監視の一覧」「対象のファイル」「指示ファイル」「バックグラウンド」など）。
-   本タスクで導入・定義する「ハーネス」「ハーネスのアダプタ」「上位モデルのレビュー」「クロスチェックの部分レビュー」「スクラッチパッド」は 3.1 で定義したとおりとする。リポジトリ横断の用語統一で必要になる語は、設計・実装段階で `docs/translation_glossary.md` に追加する。

## 変更履歴

- 2026-11-22 承認（isseis、#1079）
- 2026-11-22 draft への戻し：Antigravity のクロスチェック担当モデルを Gemini 系（Carbon/Argon）候補から `claude-opus-5-5-medium` へ改めた（#1083）
- 2026-11-22 承認（isseis、#1083）
