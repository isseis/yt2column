# 要件定義書：開発コマンド定義と補助ツールのハーネス非依存化（Antigravity 対応）

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-21 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`.claude/commands/` 配下の開発コマンド定義・共有手順・補助スクリプトおよび設定をハーネスに依存しない形へ改め、Claude Code に加えて Antigravity CLI（`agy`）でも同じ開発フローを実行できるようにするための要件を定義する。

対応 issue: [#1072](https://github.com/isseis/yt2column/issues/1072)
前提 issue: [#1059](https://github.com/isseis/yt2column/issues/1059)（タスク 0118：Codex の PR レビューと Codex CLI の撤去）
関連 issue: [#1057](https://github.com/isseis/yt2column/issues/1057)（計画書と実装 PR の部分レビューの置き換え。本タスクの後に行う）、[#760](https://github.com/isseis/yt2column/issues/760)（タスク 0081：実装モデル要件と Opus のレビューの導入）

**背景:**

現在、`.claude/commands/` 配下の開発コマンド定義（`mkreq.md`・`mkarch.md`・`mkplan.md`・`mkplan2.md`・`runplan.md`・`runpr.md`・`fixpr.md`・`mergepr.md`・`weakreview.md`・`japrose.md`・`mktrans.md`・`chief.md`、`CLAUDE.md:208-220`）と共有手順（`_lib/`）、判定用のシェルスクリプトは、Claude Code（一部 OpenCode）で動くことを前提に書かれている。Antigravity CLI（`agy`）で同じ手順を動かそうとすると、モデルの名前を置き換えるだけでは次の 4 か所の前提に阻まれる。

1.  **組み込みツールと実行制約の違い:**
    -   **選択式の質問:** コマンド定義と `_lib/settle-decisions.md:38-50` は Claude Code の `AskUserQuestion` を名指ししている。Antigravity の質問ツールは `ask_question` である。
    -   **サブエージェント起動とモデル指定:** `_lib/review-subagent-pattern.md:30-39`・`_lib/opus-review.md:33-55`・`_lib/fable-review.md:68` は Claude Code の `Agent` ツールで任意のモデル名を指定できることを前提としている。Antigravity の `invoke_subagent` は 4 段階の列挙値（`inherit`・`flash_lite`・`flash`・`pro`）しか取れず、具体的なモデルの ID を固定できるのは `swarm(action: "add", model: "<model-id>", ...)` だけである（issue #1072 コメント 1）。
    -   **スクラッチパッド:** `_context.md:210`・`mkreq.md:56` などは Claude Code の scratchpad を前提としている。Antigravity のセッション用一時ディレクトリは `<appDataDir>/brain/<conversation-id>/scratch/` である。
    -   **シェル実行・タイマー・通知・停止:** `_lib/fable-review.md:147-161` は `Bash` のバックグラウンド `sleep 1800` と `TaskStop` を使い、`_context.md:52` は `PushNotification` を名指ししている。Antigravity は `run_command`・`schedule`・`manage_task` を持ち、シェルの `sleep` をタイマーとして使わず、`PushNotification` に相当するツールを持たない。
2.  **決定論的スクリプト・設定のハードコード:**
    -   `_lib/opus-review-decision.sh:307-318` と `_lib/opus-review.md:53-75` は、上位モデルの ID を `claude-opus-*` にハードコードし、システムプロンプトから自分のモデルの ID を読めることを前提としている。Antigravity のセッションとサブエージェントはシステムプロンプトから自身のモデルの ID を読めない（issue #1072 コメント 1）。
    -   `_lib/review-records.sh:192-193` と `_lib/fable-review.md:340-344` は、部分レビューのモデルの ID を `claude-fable-[a-z0-9.-]{1,48}` にハードコードし、サブエージェント出力 1 行目の自己申告（`Model: <model-id>`）で検査している。また、記録行の接頭辞も `Opus review:`（`_lib/opus-review.md:85,112`）・`Fable review:`（`_lib/fable-review.md:446-451`）とモデル固有名を持つ。
    -   実装計画書の `**実装モデル要件**` の値が `opus`・`sonnet+opus-review`・`sonnet` というモデル固有名になっており（`_context.md:67`、`mkplan2.md:41-45`、`internal/chief/task.go:250-274`）、[ADR-0042](../../dev/adr/0042-implementation-model-tiers.md) でも `sonnet` の値が実際には DeepSeek V4.1 Flash を指していて名前と実態が合わない課題が残っていた。
    -   `_context.md:157-174` の `Command routing` 表と `internal/chief/routing.go:27-34,51-56` は `Claude Code` と `OpenCode` の 2 値だけを前提としている。
3.  **権限・サンドボックスモデルの違い:**
    -   Claude Code は `.claude/settings.json:1-51` の接頭辞一致ルールで許可と拒否（`herdr` の書き込み系、`git worktree` の追加・削除・移動、`make install-*`、`go install` の拒否）を強制している（[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)、[ADR-0046](../../dev/adr/0046-chief-of-staff.md)）。
    -   Antigravity CLI は `.claude/settings.json` を読まず、既定の OS サンドボックス（ネットワークとワークスペース外へのアクセスを遮断）と、必要な操作に対する `BypassSandbox: true`（利用者の明示承認）で保護されている。外部の git worktree（`.git` がワークスペース外の本体リポジトリを指す構成）での `git` 操作や、`gh`・`coderabbit review`・`numreserve` などは標準サンドボックスでは拒否される。
4.  **コマンドの配置とクロスチェックレビューの扱い:**
    -   Antigravity CLI は `.claude/commands/*.md` をそのままではカスタムスキルとして自動検出しない。
    -   PR を開く前の部分レビュー（[ADR-0050](../../dev/adr/0050-fable-partial-review.md)）について、Antigravity でどのモデルを固定し、モデルの検証と独立性をどう扱うかを定める必要がある。

**本書の記述範囲:** 本書は、対象とするコマンドとハーネスの範囲、Antigravity からの呼び出しの入口、`**実装モデル要件**` の新しい役割名と旧値の読み替え、上位モデルのレビューとクロスチェックの部分レビューのハーネス別の振る舞い・検証条件・失敗時の扱い、記録行の互換性、Antigravity のサンドボックス・権限規則と受け入れる危険、`chief` との境界を定める。次の事項はアーキテクチャ設計書（`02_architecture.md`）で決める（[design_handoff.md](design_handoff.md) に申し送る）。

-   Antigravity のクロスチェックの部分レビュー（および各役割）で採用する具体的なモデル（Argon・Carbon 等の実測評価と選定。H-01）。
-   ハーネスのアダプタ文書と `.agents/skills/` 配下の薄い入口の具体的なファイル構成・抽象操作の記述形式・ガードテストの走査方法（H-02）。
-   新しいコミット記録行の接頭辞・不一致理由コード・PR 説明の見出しの具体的な文字列、判定スクリプトのファイル名と新旧両読みの正規表現、`_context.md` からのモデル表の読み取り方法（H-03）。
-   Antigravity での一般サブエージェント（`invoke_subagent` の種類と段階指定）とモデル固定ワーカー（`swarm`）の使い分け、および状態チェックの詳細（H-04）。
-   `_context.md` の表の配置・列構造と、`internal/chief` の変更に伴うプロトコル番号更新の要否（H-05）。

**issue #1072 で決めたこと:**

-   **モデル指定の役割への抽象化:** 計画書・手順書からは具体的なモデル名を切り離して役割名で表し、既存計画書の旧値との後方互換を保ち、`_context.md` にハーネス別の役割と実モデルの対応表を置いて判定スクリプトがそれを参照する。
-   **ハーネス差分のアダプタ分離:** 質問ツール・サブエージェント起動・スクラッチパッド・サンドボックスと権限の注意点をハーネス別のアダプタ文書へ切り出し、コマンド定義の本体には抽象化した操作を書く。ハーネスの判別は利用可能なツールで決定的に行う（issue コメント 1）。
-   **Antigravity での部分レビューのモデル固定と検証方法（issue コメント 1）:** Antigravity ではクロスチェックの部分レビューに `invoke_subagent`（`Model: "pro"` を含む）を使わず、起動前に `swarm(action: "models", all: true)` で `_context.md` 指定モデルの存在と割り当ての残りを確認する（利用不可なら別モデルへフォールバックせず `not run` として記録する）。利用可能なら `swarm(action: "add", ...)` で起動し、サブエージェント本文の自己申告ではなく返却 JSON の `"model"` フィールド（必要なら `status`）が指定モデルと完全一致することを検証し、不一致なら即座に `swarm(action: "kill")` してモデル不一致の `not run` で終える。待機・30 分タイムアウト（`schedule`）・結果取得（`read_board`）・終了時の `swarm(action: "kill")` を使い、worktree の状態チェックは維持する。
-   **Antigravity のクロスチェック担当モデルは Gemini 系の Carbon または Argon を候補とし、設計段階で評価して決定する（issue コメント 2、および 2026-11-21 の追加確認）:** Antigravity 上の `swarm` では Gemini 4 Carbon（`gemini-4-carbon-*`）および Gemini 4 Argon（`gemini-4-argon-*`）が利用可能であるため、本書ではどちらか一方に固定せず、`02_architecture.md` で比較評価して `_context.md` に設定する採用モデル（および effort 水準）を決める。

**要件定義の際に決めたこと（2026-11-21）:**

以下は、起草にあたって利用者に確認して決めた事項と、それぞれの理由および ADR の判断である。

-   **進め方は通常の進め方（`full`）にする:** 簡易な進め方の除外（`docs/dev/conventions/requirements_process.md:551-568`）に当たると判断し、推奨どおり利用者が通常の進め方を選んだ。除外と判断した理由は、開発の道具の信頼の境界とレビューの関門に関わる判定スクリプト（`_lib/opus-review-decision.sh`・`_lib/review-records.sh`）、モデル検証の手順、権限とサンドボックスの扱いに変更が及ぶためである（[ADR-0013](../../dev/adr/0013-review-setup.md)、[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)、[ADR-0050](../../dev/adr/0050-fable-partial-review.md)）。あわせて、10 を超えるコマンド定義と複数の ADR にまたがり、設計段階で Antigravity 上のモデル評価を行うため、段階ごとに合意する方が手戻りを防げる。ADR の判断: ADR なし（このタスク自身の進め方の選択であり、タスクをまたぐ設計決定ではないため）。
-   **`chief` の Antigravity 操作対応は別タスクに切り出し、本タスクは計画書の新 tier 値の読み取り追随と既存 `Command routing` の維持までとする（F-007）:** `chief` が Antigravity の pane・Herdr 連携・`transcript.jsonl` を読み取り入力を送る機能は、実画面と記録の標本採取や画面読み取り例外の拡大を伴う独立した大きな設計主題であるため、先に開発コマンドを手動で Antigravity 上で完走できるようにしてから別タスクで扱う。`Command routing` 表に送信できない `Antigravity` を先に足す案や、完全対応を本タスクに含める案は採らない。ADR の判断: 新しい値名自体の決定は [ADR-0042](../../dev/adr/0042-implementation-model-tiers.md) を更新し、`chief` の送り先の値名の追随と境界の明記として [ADR-0046](../../dev/adr/0046-chief-of-staff.md) と新規の [ADR-0055](../../dev/adr/0055-harness-independent-commands.md) を更新・作成する。
-   **`chief` を除く全開発コマンドを対象にし、`.claude/commands/` を正本として `.agents/skills/` に薄い入口を置き、OpenCode は現行範囲のままにする（F-001、F-002）:** 手順の二重管理を避けつつ、Antigravity から各開発コマンドをカスタムスキルとして直接呼び出せるようにするためである。`.agents/skills/` を置かず毎回 `@[.claude/commands/...]` のファイル参照で呼ぶ案は操作の手間が残るため採らない。OpenCode 用のサブエージェントアダプタまで新設する案は、現在の運用で `/runpr` と最下位 tier の `/runplan` にしか使っておらず不要な仕組みを増やす（YAGNI に反する）ため採らない。ADR の判断: [ADR-0055](../../dev/adr/0055-harness-independent-commands.md) を新規作成する（コマンド定義の正本とハーネス別アダプタ・入口分離の方針は、今後のすべての開発コマンド変更を縛る主要な決定であるため。`CLAUDE.md:132-142`）。
-   **`**実装モデル要件**` の値を `high`・`standard+high-review`・`low` の 3 つにし、自己のモデルの ID を検証できない Antigravity では `high` でも `swarm` で上位モデルのレビューを走らせる（F-003、F-004）:** 最下位 tier（現 `sonnet`）は実際には軽量モデルで動かしているため `low` と名付けることで 3 段階の役割が一意になり、既存の旧値（`opus`・`sonnet+opus-review`・`sonnet`・`frontier-required`・`frontier-recommended`・`standard`）もすべて `_context.md` の読み替え表で衝突なく吸収できる。`high` / `standard+high-review` / `standard` にする案は、`standard` が中間 tier の実装モデルと最下位 tier の 2 つの意味を持つため採らない。また、Antigravity のメインセッションはシステムプロンプトから自身のモデルの ID を検証できないため、`Implementation model:` の記録を書かず、`high` の PR でも `swarm`（モデル固定と返却 JSON の `"model"` 検証）で上位モデルのレビューを実行する。利用者の自己申告などで上位モデルのレビューを省略できるようにする案は、モデル検証なしに関門を飛ばす経路が増えるため採らない。Antigravity で `high` を実装した PR に上位モデルのレビュー 1 回分の追加費用がかかる点は結果として受け入れる。ADR の判断: [ADR-0042](../../dev/adr/0042-implementation-model-tiers.md) を更新する。
-   **Antigravity でも `swarm` 固定モデルでクロスチェックの部分レビューを行い、メインセッションと同系統になりうる点は受け入れる危険とする（F-005）:** 利用不可・モデル不一致のときは [ADR-0050](../../dev/adr/0050-fable-partial-review.md) の現行方針どおり別モデルで代替せず `not run` と記録して進む。Antigravity ではメインセッションのモデルをセッション側から検証できないため、メインセッションとクロスチェック担当モデルが同系統（または同一モデル）になりうるが、独立したワーカーの文脈で 4 基準に絞って検証することと、ローカルの CLI レビュー（CodeRabbit CLI）が別系統の目として働くことから、この危険を受け入れる。毎回利用者に確認を求める案や、Antigravity で部分レビュー自体を走らせない案は採らない。ADR の判断: [ADR-0050](../../dev/adr/0050-fable-partial-review.md) を更新する。
-   **コミットメッセージと PR 説明に残すレビュー記録行の接頭辞と不一致理由コードも役割名に基づく形へ改め、判定スクリプトと読み取り手順は新旧両方を読むようにする（F-004、F-005）:** モデル固有名（`Opus`・`Fable`・`not-fable`）を記録行からも外し、計画書の値・手順・記録の用語を一貫させるためである。既存のブランチ・過去のコミット履歴・集計との互換性を保つため、判定スクリプトと PR 説明・`/runpr` の読み取りは新旧両方の形式を同じ意味で読む。接頭辞を `Opus review:` / `Fable review:` のまま固定する案は、Antigravity のモデルで走った記録にも Claude のモデル名が残り続けるため採らない。ADR の判断: [ADR-0042](../../dev/adr/0042-implementation-model-tiers.md) と [ADR-0050](../../dev/adr/0050-fable-partial-review.md) の更新に含める。
-   **Antigravity では標準サンドボックス優先・必要最小限の `BypassSandbox` 承認と禁止規則の文書化で守り、リポジトリ設定による機械的 `deny` の不在は受け入れる危険とする（F-006）:** Claude Code 向けの `.claude/settings.json` は維持し、Antigravity では既定の OS サンドボックスを先に試して必要な単一コマンドだけを `BypassSandbox: true`（利用者の承認）にする規則と、入れる道具のインストール・ビルド禁止や `herdr`・`git worktree` の直接変更禁止の明文化で守る。Antigravity 向けに Hook 等で禁止コマンドを機械的に拒否する仕組みを新設する案は、ハーネス固有の保守対象を増やすため採らない。ADR の判断: [ADR-0055](../../dev/adr/0055-harness-independent-commands.md) に記録し、[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) を更新する。
-   **先行タスク #1059（タスク 0118：Codex 撤去）の完了を実装の前提とし、#1057 は待たずに進める:** 撤去が決まっている `codexreview` と Codex の PR レビューに対する Antigravity 対応は無駄な手戻りになるため行わない。一方、#1057（計画書・実装 PR の部分レビューの置き換え）は本タスクで整備したハーネス非依存の土台の上で並走・移行する方が自然であるため待たない。ADR の判断: ADR なし（タスク間の実施順序とスコープ境界であり、恒久的な設計決定ではないため）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `chief` を除くすべての開発コマンドを、Claude Code と Antigravity CLI のどちらのセッションからでも同じ品質関門・承認規則・記録契約で実行できるようにする。
-   計画書・コマンド定義・記録行からモデル固有名への直接依存を切り離し、`_context.md` の対応表 1 か所でハーネスごとの実モデルを管理できるようにする。
-   Antigravity 上での上位モデルのレビューとクロスチェックの部分レビューにおいて、モデルのすり替わりや自己申告の誤認をハーネスの返却値で防ぐ。

### 2.2. スコープ (In Scope)

-   `.claude/commands/`（`chief.md` を除く全コマンドと `_lib/`）のハーネス非依存化と、Claude Code 用・Antigravity 用のハーネスのアダプタの作成（F-001）。
-   `.agents/skills/` 配下への Antigravity 用薄い入口（カスタムスキル）の配置（F-002）。
-   `**実装モデル要件**` の役割名（`high`・`standard+high-review`・`low`）への変更、旧値の読み替え、および `_context.md` へのハーネス別役割→実モデル対応表の追加（F-003）。
-   上位モデルのレビュー（旧 Opus のレビュー）と判定スクリプトのハーネス非依存化・新旧記録行の両読み対応（F-004）。
-   クロスチェックの部分レビュー（旧 Fable の部分レビュー）と記録判定スクリプトのハーネス非依存化・`swarm` によるモデル固定と検証・新旧記録行の両読み対応（F-005）。
-   Antigravity のサンドボックス実行規則・権限境界・禁止コマンドの明文化（F-006）。
-   `internal/chief` と `docs/dev/chief_architecture.md` における計画書の新 tier 値の読み取り追随（F-007）。
-   `CLAUDE.md`・`AGENTS.md`・`docs/dev/` 配下の規約とガイド・ADR（ADR-0055 の作成、ADR-0014・0042・0046・0050 と索引の更新）・用語集・文書のガードテストの更新（F-008）。

### 2.3. スコープ外 (Out of Scope)

-   **`chief` の Antigravity 操作対応:** Herdr や Hooks による `conversation-id` の紐付け、`transcript.jsonl` / `transcript_full.jsonl` の読み取り、`paneinput` や `Command routing` による Antigravity への送信、および `/chief` 自体の Antigravity 上での実行は別タスクに切り出す（1 章「要件定義の際に決めたこと」）。
-   **OpenCode 用のサブエージェントアダプタの新設:** OpenCode は現行の運用範囲（`/runpr` と `low` の `/runplan`）のまま維持し、全コマンド対応は行わない（同上）。
-   **Codex の PR レビューと `codexreview` の Antigravity 対応:** 前提タスク #1059（タスク 0118）で撤去されるため対象外とする（同上）。
-   **#1057 の計画書・実装 PR における部分レビューの置き換え:** 本タスクの後に別タスクとして実施する（同上）。
-   **Antigravity 向けの機械的コマンド拒否 Hook・プラグインの新設:** 禁止コマンドは指示文書と `BypassSandbox` の利用者承認で守り、機械的 `deny` の不在は受け入れる危険とする（同上）。
-   **`docs/tasks/` の過去のタスク文書と `docs/research/` の記録の書き換え:** 各タスク・調査時点の記録であり、計画書の旧値は `_context.md` の読み替えの表で扱うため書き換えない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 用語

本章では、次の用語を使う。

-   **ハーネス:** 開発コマンドを実行するエージェント実行環境。本リポジトリでは Claude Code、Antigravity CLI（`agy`）、OpenCode を指す。
-   **ハーネスのアダプタ:** ハーネスごとの組み込みツール、モデルの固定と検証、スクラッチパッドの場所、タイマーとバックグラウンド制御、サンドボックスと権限の規則の差分を定義する手順文書。
-   **上位モデルのレビュー:** 実装モデル要件が `high` または `standard+high-review`（および読めない値）のプランの PR を開く前、または `/weakreview` で、ハーネスの `high` 役割のモデルを用いて行う正しさのレビュー・弱いモデルの失敗のレビュー・確認のレビューの組（旧称: Opus のレビュー）。
-   **クロスチェックの部分レビュー:** タスクの PR を開く前に、信頼の境界・先行文書との照合・外部の振る舞いについての技術的な主張・人に問わずに決めたことの 4 つの基準に当たる箇所に絞って、ハーネスのクロスチェック役割のモデルで行う部分レビュー（旧称: Fable の部分レビュー）。
-   **スクラッチパッド:** 作業ツリーの外にある、セッション専用の一時ファイル置き場。

### 3.2. 機能一覧

#### F-001: コマンド定義・共有手順のハーネス非依存化とハーネスのアダプタ

`.claude/commands/` のうち `chief.md` を除くすべての開発コマンドと `_lib/` の共有手順をハーネス非依存の記述に改め、ハーネス固有の操作を Claude Code 用と Antigravity 用のハーネスのアダプタへ分離する。

**Acceptance Criteria**:
- **AC-01**: 対象コマンドは、現時点で `.claude/commands/` にある `mkreq.md`・`mkarch.md`・`mkplan.md`・`mkplan2.md`・`runplan.md`・`runpr.md`・`fixpr.md`・`mergepr.md`・`weakreview.md`・`japrose.md`・`mktrans.md`（および実装時点で `main` にある `chief.md` 以外の利用者が起動するコマンド）と、それらが参照する `_lib/` のすべての共有手順である。これらの手順の正本は `.claude/commands/` に 1 つだけ置かれる。
- **AC-02**: AC-01 のコマンド定義の本体と共有手順（ハーネスのアダプタ自身を除く）は、特定のハーネスの組み込みツール名（`AskUserQuestion`・`Agent`・`TaskStop`・`ToolSearch`・`PushNotification`・`ask_question`・`invoke_subagent`・`swarm`・`run_command`・`schedule`・`manage_task`）を直接呼ばず、抽象化された操作とハーネスのアダプタへの参照で記述される。
- **AC-03**: ハーネスの判別は、セッションで利用可能な組み込みツールの有無によって決定的に行われる。Claude Code と Antigravity のどちらのツール群も持たないハーネス（OpenCode など）で実行されたときは、質問・シェル実行・ファイル読み書きはそのまま動き、モデル固定レビューを呼べない箇所は F-004・F-005 の規則（停止、または `not run` の記録）に従う。
- **AC-04**: 選択式の質問の操作は、Claude Code（`AskUserQuestion`）と Antigravity（`ask_question`）の両方で、1 回の呼び出しにつき最大 4 問、各問 2〜4 個の選択肢、推奨する選択肢があるときは先頭に置いて推奨であることを示す規則で動く。どの選択肢にも当たらない回答を受けたときは、推測で進めずに聞き直す（`mkreq.md:165-166` の規則を維持する）。
- **AC-05**: スクラッチパッドの操作は、Claude Code と Antigravity（`<appDataDir>/brain/<conversation-id>/scratch/`）のどちらでも、作業ツリーの外のセッション用ディレクトリに一時ファイルを作り、git の追跡対象やコミットに含めない。
- **AC-06**: 一般のサブエージェント（セルフレビュー、パネルモードのレビュー、`/fixpr` のトリアージと修正確認、エスカレーション判定、`japrose`、`mktrans` の翻訳確認）は、Claude Code と Antigravity の両方で起動・結果回収ができる。発見パスはセッションのモデルを引き継ぎ、確認パスで軽量モデルを使ってよい箇所（`_lib/review-subagent-pattern.md:100-110`）もハーネスごとのアダプタの規則に従う。
- **AC-07**: バックグラウンド待機・タイマー・利用者への通知の操作は、ハーネスごとの手段に分離される。Antigravity アダプタはタイマーにシェルの `sleep` を使わず `schedule` ツールを使い、通知ツールを持たないハーネスでは `_context.md:52` の規則どおり最終の報告をもって利用者への通知とする。

#### F-002: Antigravity からのコマンド呼び出しの入口

Antigravity CLI から F-001 の各開発コマンドをカスタムスキルとして呼び出せるように、`.agents/skills/` 配下に薄い入口を置く。

**Acceptance Criteria**:
- **AC-08**: F-001 の対象コマンド（`chief` を除く各開発コマンド）ごとに、`.agents/skills/<コマンド名>/SKILL.md` が存在する。各ファイルは英語（`CLAUDE.md`「Document language」原則 (b)）で書かれ、対応する `.claude/commands/<コマンド名>.md` と `.claude/commands/_context.md`、および Antigravity 用のハーネスのアダプタを読んで手順を実行するよう指示する。
- **AC-09**: `.agents/skills/<コマンド名>/SKILL.md` は、対応する `.claude/commands/<コマンド名>.md` の各ステップの手順本文を複製しない。コマンドの引数仕様・事前認可（Authorization）・停止条件は、対応する `.claude/commands/<コマンド名>.md` の定義がそのまま適用される。
- **AC-10**: `chief` に対応するスキルは `.agents/skills/` に置かない（`chief` の Antigravity 対応はスコープ外のため）。

#### F-003: 実装モデル要件の役割名への抽象化とハーネス別のモデル対応表

計画書に書く `**実装モデル要件**` の値をモデル固有名から役割名へ改め、旧値の読み替えとハーネス別の実モデル対応表を `_context.md` に一元化する。

**Acceptance Criteria**:
- **AC-11**: 計画書の PR マーカーおよび単一フェーズ計画書の `PR 構成` 節に新しく書く `**実装モデル要件**` の値は、次の 3 つのどれか 1 つである。計画書を作成するコマンド（`mkplan2.md`・`mkplan.md`・簡易な進め方の `mkreq.md`）は、この 3 つの値だけを書く値として挙げる。

  | 値 | 意味 | 当てはめる契機 |
  |---|---|---|
  | `high` | 上位役割（`high`）のモデルで実装し、上位モデルで実装された記録で覆われない commit があれば PR を開く前に上位モデルのレビュー（F-004）を行う | 現行の `opus` の契機（`mkplan2.md:42`。前例のない設計判断、または `mkplan.md` step 8 の panel mode の契機） |
  | `standard+high-review` | 標準役割（`standard`）のモデルで実装し、PR を開く前に上位モデルのレビュー（F-004）を行う | 現行の `sonnet+opus-review` の契機（`mkplan2.md:43`。競合する実装方針があるステップ、Conditional checks に 2 つ以上当たるステップ、復旧処理・並行処理・状態機械などの高リスク・複雑なステップ） |
  | `low` | 軽量役割（`low`）のモデルで実装し、上位モデルのレビューは行わない | それ以外（現行の `sonnet` の契機、`mkplan2.md:44`） |

- **AC-12**: `**判定理由**` は、`high` と `standard+high-review` のときは当たった契機と「`high` より軽いモデルに任せたときに何が起きやすいか」を 1 行の日本語で書き、`low` のときは当たる契機がないことを書く（モデル固有名の「Sonnet に任せると」を必須としない）。
- **AC-13**: `_context.md` の旧値の読み替えの表は、次の 6 つの旧値をそれぞれの新しい値に読み替える。完了した計画書および進行中の計画書の本文にある旧値は書き換えない。

  | 旧値 | 読み替える値 |
  |---|---|
  | `opus` | `high` |
  | `sonnet+opus-review` | `standard+high-review` |
  | `sonnet` | `low` |
  | `frontier-required` | `high` |
  | `frontier-recommended` | `standard+high-review` |
  | `standard` | `low` |

- **AC-14**: 正規の 3 値でも AC-13 の旧値でもない値は、推測して別の値に読み替えず「読めない値」として示し、上位モデルのレビューを行う側に倒す。`**実装モデル要件**` の欄がない計画書は「値なし」として扱い、上位モデルのレビューを行わない（`_context.md:91-95,127-128` の規則を維持する）。
- **AC-15**: `_context.md` に、ハーネス（Claude Code・Antigravity・OpenCode）ごとの役割と実モデルの対応表が 1 か所だけ置かれる。対応表は、少なくとも実装と上位モデルのレビューで使う役割（`high`・`standard`・`low`）とクロスチェックの部分レビューの役割について、ハーネスごとのモデルの指定値・起動手段・検証対象のモデルの ID（または一致規則）を持つ。クロスチェックの部分レビューの担当モデルは、Claude Code では `fable` とし、Antigravity（Gemini 系）では Gemini 4 Carbon（`gemini-4-carbon-*`）または Gemini 4 Argon（`gemini-4-argon-*`）を候補として `02_architecture.md` で評価して決定する。
- **AC-16**: `runplan.md`（開始時の案内）と `runpr.md`（マージの質問と次のコマンドの案内）は、計画書から読んだ `**実装モデル要件**` の値（旧値のときは元の値と読み替えた値の両方）と、`_context.md` の対応表に基づく推奨モデルおよび上位モデルのレビューの見込みを示す。

#### F-004: 上位モデルのレビューと判定スクリプトの一般化・ハーネス非依存化

`/runplan` と `/weakreview` が共有する上位モデルのレビュー（旧 Opus のレビュー）の手順・ファイル名・記録行・判定スクリプトをモデル固有名に依存しない形へ一般化し、Claude Code と Antigravity の両方で動くようにする。

**Acceptance Criteria**:
- **AC-17**: 上位モデルのレビューを行うかどうかを決める判定スクリプトは、レビュー対象ブランチの変更で判定が変わらないよう `origin/main` の版を取り出して実行し、`origin/main` の `_context.md` から新旧の `**実装モデル要件**` の値と、`high` 役割として認めるモデルの ID の規則を読む。スクリプト内に `claude-opus-*` や旧値の一覧をハードコードせず、スクリプトの出力（現行の `all-opus` を含む理由コード）や共有手順のファイル名からもモデル固有名（`opus`）を外す（移行中の `origin/main` 参照互換の扱いは `02_architecture.md` で決める）。git エラー・読めない計画書・不正な引数・複数の値欄などの失敗時は、すべて安全側である `RUN` を標準出力に出して終了コード `0` で終わる。
- **AC-18**: セッションがシステムプロンプトから自身のモデルの ID を確かめられるハーネス（Claude Code）では、`runplan.md` は現行どおり各 phase group の終わりに `Implementation model: <START12>..<END12> <model-id>[, ...]` を記録し、除外できる commit（`_lib/opus-review.md:102-106`）を除くすべての commit が `_context.md` の `high` 役割のモデルの ID だけで記録されているとき、判定スクリプトはレビュー不要を返す。
- **AC-19**: セッションがシステムプロンプトから自身のモデルの ID を確かめられないハーネス（Antigravity）では、`runplan.md` は `Implementation model:` の記録行を書かない。その結果、`high`・`standard+high-review`・読めない値のプランの PR では、完了記録で覆われない限り判定スクリプトは `RUN` を返し、PR を開く前に必ず上位モデルのレビューが走る。
- **AC-20**: Antigravity で上位モデルのレビュー（正しさのレビュー・弱いモデルの失敗のレビュー・確認のレビュー）を実行するときは、`invoke_subagent` を使わず、起動前に `swarm(action: "models", all: true)` で `_context.md` の Antigravity 用 `high` 役割モデルの存在と割り当ての残りを確かめ、`swarm(action: "add", ...)` でそのモデルを固定して起動する。起動結果の JSON の `"model"` フィールド（必要に応じて `swarm(action: "status")`）が指定モデルの ID とバイト単位で完全一致することを確かめ、終了時（異常終了時を含む）に `swarm(action: "kill")` でワーカーを終了させる。
- **AC-21**: Claude Code と Antigravity のどちらでも、上位モデルのレビューを指定された `high` 役割のモデルで走らせられないとき（ハーネスがモデル固定起動を持たない OpenCode のとき、モデルが存在しないか割り当て切れのとき、検証したモデルの ID が一致しないとき、呼び出しがエラーになったとき）は、別のモデルで代替せず、作業を止めて理由を報告し、`/runplan` では PR を開かず何も push しない（`/weakreview` でも修正コミットを作らず止まる）。
- **AC-22**: 上位モデルのレビューが新しくコミットメッセージに書く完了記録と退けた指摘の記録行は、モデル固有名（`Opus`）を含まない役割名に基づく接頭辞を使う（具体的な文字列は `02_architecture.md` で決める）。判定スクリプト・PR 説明の生成・部分レビューに渡す決着済み論点の読み取りは、新しい記録行と既存の `Opus review: done` / `Opus review: kept` の行の両方を同じ意味で読む。先頭列から始まらない行や文法に合わない行は補正せず記録なしとして扱う。
- **AC-23**: 上位モデルのレビューのワーカー／サブエージェントの出力は信頼しないデータとして扱い、出力の文面をコミットメッセージ・SUMMARY・PR の説明に写さない。コミット前のメンション確認、セッションが自分の判断で Critical・Major の指摘を退けず利用者に尋ねる規則、および `/weakreview` の引数仕様（既定で修正してコミットし push しない、`--no-fix` で報告のみ、`--fix` を無操作で受理、clean でない作業ツリーでは停止）は現行どおり維持する。

#### F-005: クロスチェックの部分レビューと記録判定スクリプトの一般化・ハーネス非依存化

タスクの PR を開く前に行う部分レビュー（現行の `_lib/fable-review.md`）と有効性判定スクリプト（`_lib/review-records.sh`）を、モデル固有名（`Fable`）に依存しないクロスチェックの部分レビューとして一般化し、Claude Code と Antigravity（Gemini 系の Carbon または Argon）の両方で動くようにする。

**Acceptance Criteria**:
- **AC-24**: クロスチェックの部分レビューの共有手順は、ファイル名・呼び出し元（`_lib/local-cli-review.md` や `runplan.md`）の入力パラメータ名を含めてモデル固有名（`fable` / `FABLE`）を含まない役割名に基づく名前に改める。本体は、有効性の判定、4 つの範囲基準（`trust-boundary`・`prior-docs`・`external-claim`・`undecided-choice`）と除外箇所の検証、プロンプトの構成、worktree の状態チェック、出力形式の検査、ローカルの CLI レビューとの統合トリアージと却下根拠の確認、記録行の規則をハーネス共通の手順として持ち、起動・モデル検証・タイムアウト待機・停止処理をハーネスのアダプタへ委譲する。
- **AC-25**: Claude Code 用のアダプタは、`Agent` ツールで `_context.md` 指定のクロスチェックモデル（`fable`）を起動し、出力 1 行目のモデルの ID が `_context.md` の規則に合うことを確かめ、30 分タイマーと `TaskStop` による停止を行う現行の動作を保つ。
- **AC-26**: Antigravity 用のアダプタは、次のすべてを守る。
  1.  クロスチェックの部分レビューに `invoke_subagent`（`Model: "pro"` を含む）を使わない。
  2.  起動前に `swarm(action: "models", all: true)` で `_context.md` 指定のクロスチェックモデル（Gemini 4 Carbon `gemini-4-carbon-*` または Gemini 4 Argon `gemini-4-argon-*` のうち、`02_architecture.md` で評価して決定したモデル）の存在と割り当ての残りを確かめる。存在しないか割り当てがないときは、別モデルへフォールバックせず `not run`（理由 `unavailable`）とする。
  3.  利用可能なときは `swarm(action: "add", name: "<worker>", model: "<model-id>", workspace: "inherit", brief: ...)` で起動し、サブエージェント本文の自己申告ではなく、`swarm(action: "add")` の返却 JSON の `"model"` フィールド（必要に応じて `swarm(action: "status")`）が `_context.md` 指定モデルの ID とバイト単位で完全一致することを確かめる。一致しないか読めないときは即座に `swarm(action: "kill")` で終了させ、別モデルで代替せず `not run`（モデル不一致の理由）とする。
  4.  待機には `swarm` の待機またはターン終了と `schedule(DurationSeconds: 1800, TimerCondition: "any")` による 30 分タイムアウトを使い、結果の取得には `read_board` を使い、完了・エラー・タイムアウト・呼び出し元停止のすべての出口でタイマーの解除と `swarm(action: "kill")` によるワーカー終了、および worktree の状態チェックを行う。1 回目の呼び出しがエラーまたは利用上限で終わったときの 1 回限りの再試行規則（現行 `_lib/fable-review.md:182-187`）を保つ。
- **AC-27**: クロスチェックの部分レビューを呼べないハーネス（OpenCode など）では、現行どおり `not run`（理由 `agent-cannot-call`）を記録して進む。タスク 0114 で導入された記録後の merge commit に対する merge の後の確認（[ADR-0050](../../dev/adr/0050-fable-partial-review.md) 決定 5）も、`Opus` 固定ではなくハーネスの `high` 役割のモデル（Antigravity では AC-20 と同じ `swarm` 固定・検証、走らせられないときは `not run` 記録）で行う。
- **AC-28**: 新しく書く部分レビューの記録行（実施・未実施・範囲に入れた箇所・外した箇所・指摘・チェックポイント）の接頭辞、モデル不一致の `not run` 理由コード、および PR 説明の節見出しと折りたたみ見出し、`/runpr` のマージ質問の要約表示は、モデル固有名（`Fable`・`not-fable`）を含まない役割名に基づく形にする（具体的な文字列は `02_architecture.md` で決める）。
- **AC-29**: `origin/main` から取り出して実行する記録判定スクリプト（`_lib/review-records.sh`）と、PR 説明の生成（`_lib/task-pr-handoff.md`）・`/runpr` のマージ質問の要約読み取りは、次のすべてを満たす。
  1.  スクリプトのモード名としてモデル固有名を含まない新しいモード名を受け入れる（移行中のブランチ互換のため既存の `fable` モード名も受け入れるかは `02_architecture.md` で決める）。
  2.  実施記録の `<model-id>` として、`origin/main` の `_context.md` が認めるすべてのハーネスのクロスチェックモデルの ID（既存の `claude-fable-...` と Antigravity の採用モデルの ID）を受け入れる。認めないモデルの ID や壊れた形式の行は補正せず無視する。
  3.  AC-28 の新しい記録行・理由コードと、既存の `Fable review: run`・`Fable review: not run`（理由 `not-fable` を含む）・`Fable scope: in`・`Fable scope: out`・`Fable finding:`・`Fable review: checkpoint` の行の両方を同じ意味の記録として読む。
- **AC-30**: 部分レビューのワーカー／サブエージェントの出力は信頼しないデータとして扱う。出力全体が決められた形式でないときは `not run`（理由 `unreadable`）とし、ブランチの変更ファイル外の場所や読めない場所・重大度の指摘は `invalid` とし、出力の文面をコミットメッセージ・SUMMARY・PR の説明に一切写さない（現行 `_lib/fable-review.md:330-353,507-509` の規則を維持する）。

#### F-006: Antigravity のサンドボックス・権限規則と禁止コマンドの明文化

Claude Code の権限設定を保ったまま、Antigravity でのサンドボックス実行規則と禁止コマンドを明文化する。

**Acceptance Criteria**:
- **AC-31**: 本タスクの実装 PR が変更するファイルに `.claude/settings.json` は含まれない（前提タスク #1059 完了後の `.claude/settings.json` をそのまま維持する）。
- **AC-32**: Antigravity 用のハーネスのアダプタは、コマンド実行の規則として次を定める。
  1.  すべてのコマンドは既定の標準サンドボックス（`BypassSandbox: false`）で先に試行し、成功した種類のコマンドは標準サンドボックスのまま使い続けること。
  2.  ワークスペース外の `.git` worktree へのアクセス、ネットワーク通信（`gh`・`coderabbit review`・`numreserve`・`git fetch`・`git push` など）、またはサンドボックスが拒否するソケット操作で実際に失敗したときに限り、その単一のコマンドだけを分離して `BypassSandbox: true` で再実行すること（前後の準備・ファイル処理・テスト実行を `&&` や `;` で繋いで一緒にサンドボックス外へ昇格させないこと）。
  3.  再実行時は `toolSummary` を前回と同一に保ち、利用者の自動承認（接頭辞一致）が効くよう不要なコマンド置換 `$(...)` やラッパー（`env`・`eval` 等）を避けること。ただし、PR タイトルに環境変数展開を起こさせないための `_context.md:209-216` の `--title "$(cat "<title-file>")"`（または設計で定める同等の安全な渡し方）は、シェル展開による秘密漏洩を防ぐ規則を優先する。
- **AC-33**: `AGENTS.md`・`CLAUDE.md`・Antigravity 用のハーネスのアダプタからたどれる指示として、次の禁止規則が Claude Code だけでなく Antigravity セッションにも適用されることが明記される。
  1.  入れる道具（`fixprguard`・`mergepr`・`chief`・`numreserve`）をインストールもビルドもしないこと（`docs/dev/conventions/dev_tools.md` Rule 5 の開発のための例外を除く）。
  2.  `herdr` の書き込み・状態変更・作成・削除系サブコマンド、および `git worktree` の追加・削除・移動を実行しないこと（`docs/dev/guides/chief_guide.md` 6 章の一覧）。
  3.  承認された `numreserve task`・`numreserve adr` 以外の手段で記録タグ（`refs/tags/task-number/`・`refs/tags/adr-number/`）やその ruleset を作成・変更・削除しないこと、および事前承認のないネットワーク操作・PR マージ・実 LLM API 呼び出しを行わないこと（`CLAUDE.md`「Tool Execution Safety」）。
- **AC-34**: Antigravity には `.claude/settings.json` のようなリポジトリ内の設定ファイルによる機械的なコマンド拒否（`deny`）がないため、AC-33 の禁止規則が指示文書と `BypassSandbox: true` 時の利用者承認によって守られることと、エージェントが指示に反する残余の危険を受け入れることが、[ADR-0055](../../dev/adr/0055-harness-independent-commands.md) と [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) に記録される。

#### F-007: `chief` の計画書読み取りの追随

`chief` が新しい `**実装モデル要件**` の値を持つ計画書を正しく読めるように追随させ、送り先の表の安全性を維持する。

**Acceptance Criteria**:
- **AC-35**: `internal/chief`（`internal/chief/task.go`）が計画書から読む `**実装モデル要件**` は、`high`・`standard+high-review`・`low` を正規の 3 値として受け入れ、`origin/main` の `_context.md` の読み替えの表から AC-13 の 6 つの旧値をそれぞれの正規の値に読み替える。それ以外の文字列、同じ節の中の複数の値欄、フェンス内の値欄、閉じられていないコードフェンスは補正せず `errTaskDocument` 相当（`RequirementUnreadable`）とし、`chief` はその PR の `/runplan` を自動送信しない。
- **AC-36**: `internal/chief` の計画書マーカー読み取りと、F-004 の判定スクリプトの計画書マーカー読み取りを同じ計画書群に対して照合するテスト（現行の `internal/chief/markers_parity_test.go`）が、新値（`high`・`standard+high-review`・`low`）・AC-13 の全旧値・読めない値・値なしのすべてで一致して成功する。
- **AC-37**: `internal/chief/routing.go` が読む `_context.md` の `Command routing` 表は、引き続き `Claude Code` と `OpenCode` の送り先だけを保持し、未知のエージェント名・未知のモデル名・欠けた行・重複した行・余分な列を含む表を `errRouting` で全体として拒否し、何も送信しない。F-003（AC-15）で `_context.md` に追加するハーネス別のモデル対応表は、`ParseRouting` と `ParseOldValues` のどちらの読み取りも壊さない。
- **AC-38**: `docs/dev/chief_architecture.md` の計画書読み取り・送り先に関する記述が AC-35〜AC-37 の変更と同じ PR で更新され、実装と一致する。

#### F-008: 現在の仕様を書く文書・ADR・文書のガードテストの更新

現在の仕様を書く文書、ADR、および文書のガードテストを本タスクの決定に揃える。

**Acceptance Criteria**:
- **AC-39**: 現在の仕様を書く文書（`CLAUDE.md`・`AGENTS.md`・`.claude/commands/`・`.agents/skills/`・`docs/dev/`・`docs/translation_glossary.md`）は、現在の仕組みの名前として `Opus review` / `Opus のレビュー` や `Fable partial review` / `Fable の部分レビュー` を書かず、一般化した名称（上位モデルのレビュー、クロスチェックの部分レビュー等）で記述する（`_context.md` の Claude Code 用モデル指定値、旧値・旧記録行の後方互換規則、過去の経緯・採らなかった案の出典としてモデル名を挙げる箇所は除く。確かめは PR のレビューで読む `manual` とする）。あわせて、`CLAUDE.md` の「Document language」原則 (b) の対象に `.agents/skills/` が含まれ、「Language」の対話言語の規則（日本語）が特定のハーネス名に限定されない記述になり、「Commands」の節がハーネス非依存化されたコマンドと `.agents/skills/` の入口を説明し、判定スクリプトのファイル名が変わる場合は `CLAUDE.md`「Go Idioms」の日本語リテラル例外（`CLAUDE.md:954-970`）と `docs/dev/conventions/dev_tools.md` Rule 6 が同じ PR で更新される（新たに例外を取る第 3 のシェルスクリプトは増やさない）。
- **AC-40**: 文書のリンクとパス解決を検査するガードテスト（`cmd/yt2column/docpaths_test.go` の `TestInstructionDocPaths`）の検査対象に `.agents/skills/` 配下の Markdown ファイルが含まれ、`CLAUDE.md`・`AGENTS.md`・`.claude/commands/`・`.agents/skills/`・`docs/dev/` のすべての相対リンクと `docs/dev/` コードスパン参照が実在するファイルに解決される。
- **AC-41**: コマンド定義・権限規則・記録行を検査するすべてのガードテストと単体テスト（`make test`・`make lint`・`make deadcode`・`make readme-sync-check`、および CI の `doc-guard` ジョブの対象パッケージ）が成功する。
- **AC-42**: [ADR-0055](../../dev/adr/0055-harness-independent-commands.md)（新規作成）、[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)、[ADR-0042](../../dev/adr/0042-implementation-model-tiers.md)、[ADR-0046](../../dev/adr/0046-chief-of-staff.md)、[ADR-0050](../../dev/adr/0050-fable-partial-review.md)、および [ADR の索引](../../dev/adr/README.md) が本書の決定と一致し、`TestADRRecords` が成功する。本タスクの実装が完了するまで残る仕様文書・コードとの食い違いは、各 ADR の `## 決定` に `https://github.com/isseis/yt2column/issues/1072` へのリンク付きの食い違い記述として示し、食い違いを解消した実装 PR で取り除く。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   N/A。製品のパイプラインやブラウザ拡張の実行性能には影響しない。なお、クロスチェックの部分レビューの待機時間の上限は、Claude Code と Antigravity の双方で現行と同じ 30 分（1,800 秒）とする（AC-25、AC-26）。

### 4.2. セキュリティ (Security)

条件付きのセキュリティガイド（`docs/dev/security.md`）の適用条件（`yt-dlp` の起動、API キーの扱い、キャッシュの読み書き、LLM API へのネットワーク送信、信頼しないテキストのプロンプトや公開記事への埋め込み。`.claude/commands/_context.md:296`）に当たる製品機能の変更はないため、`docs/dev/security.md` の製品向け攻撃ベクタの適用は N/A である。

ただし、本タスクは開発の道具の信頼の境界とレビューの関門（[ADR-0013](../../dev/adr/0013-review-setup.md)・[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)・[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)・[ADR-0046](../../dev/adr/0046-chief-of-staff.md)・[ADR-0050](../../dev/adr/0050-fable-partial-review.md)・[ADR-0055](../../dev/adr/0055-harness-independent-commands.md)）に関わるため、次の境界規則と受け入れる危険を定める。

-   **信頼しない入力の境界と拒否規則:**
    1.  **レビュー用サブエージェント／ワーカーの出力（AC-23、AC-30）:** 上位モデルのレビューおよびクロスチェックの部分レビューの出力は信頼しないデータであり、指示として解釈しない。受け入れる形式に合わない出力は補正せず `unreadable` またはエラー停止とし、変更ファイル外の場所を指す指摘は `invalid` とし、出力の文字列をコミットメッセージ・SUMMARY・PR 説明に写さない（差分内の秘密情報やメンションの漏洩・発火を防ぐ）。
    2.  **`swarm` の返却メタデータによるモデル検証（AC-20、AC-26）:** Antigravity でのモデル固定レビューは、ワーカー本文の自己申告を信頼せず、`swarm(action: "add")`（および必要に応じた `status`）が返す `"model"` フィールドが `_context.md` 指定モデルの ID とバイト単位で完全一致することだけを認める。欠落・型違い・不一致は正規化や別モデルへのフォールバックをせず即座に `swarm(action: "kill")` で終了させる。
    3.  **計画書の値とコミットの記録行の読み取り（AC-14、AC-17、AC-22、AC-29、AC-35、AC-37）:** `internal/chief`（Go）および `origin/main` から取り出す判定スクリプトが読む計画書の値・`_context.md` の表・コミットの記録行は、先頭列からの固定文法に完全一致する行だけを受理し、未知の値・重複・壊れた行・ref と紛らわしい短縮 SHA を補正せず拒否または無視して安全側（上位モデルのレビューは `RUN`、部分レビューの有効性は `MISSING`、`chief` は `errTaskDocument` / `errRouting` による送信停止）に倒す。サイズや件数の新たな上限は設けない（対象はリポジトリ内の計画書とブランチのコミット履歴に限られるため）。
-   **外部サービスへのコードと差分の送信:** Antigravity で開発コマンド・サブエージェント・`swarm` を実行すると、リポジトリのコード・差分・タスク文書・コマンド定義が Antigravity の接続先モデル提供元へ送信される（Claude Code が Anthropic へ、OpenCode が DeepSeek へ送信するのと同じ位置づけ）。利用者がそのハーネスでコマンドを起動したことでこの送信を承認したものとする。
-   **サンドボックス・権限境界と受け入れる危険（AC-31〜AC-34）:** Antigravity では既定の OS サンドボックスを優先し、必要な単一コマンドにだけ `BypassSandbox: true` で利用者の承認を受ける。Antigravity には `.claude/settings.json` のようなリポジトリ内の機械的 `deny` 設定がないため、禁止コマンド（入れる道具のインストール・ビルド、`herdr` と `git worktree` の変更操作）に対してエージェントが指示に反する残余の危険を受け入れる。
-   **クロスチェックのモデル系統に関する受け入れる危険:** Antigravity ではメインセッションのモデルの ID をセッション側から検証できないため、利用者が選んだメインセッションのモデルと `swarm` のクロスチェック担当モデルが同系統または同一モデルになりうる。別ワーカーの独立した文脈での検証と CodeRabbit CLI による別系統レビューで補うものとし、この危険を受け入れる（[ADR-0050](../../dev/adr/0050-fable-partial-review.md)）。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   上位モデルのレビューが指定モデルで走らせられないときは、不十分なレビューのまま PR が開かれないよう確実に停止し、何も push しない（AC-21）。
-   クロスチェックの部分レビューは追加の防御層であるため、モデル利用不可・割り当て切れ・30 分のタイムアウト・モデル不一致のときはワーカーとタイマーを確実に片付けたうえで `not run` と理由を記録し、PR の作成を止めない（AC-26）。
-   サブエージェントや `swarm` ワーカーが実行中に worktree の状態を変えたときは、状態チェックが差分を検出して結果を使わずに停止する（AC-24、AC-26）。

### 4.4. 互換性 (Compatibility)

-   **進行中および過去のブランチ・計画書との互換性:**
    -   既存の計画書にある 6 種類の旧値（`opus`・`sonnet+opus-review`・`sonnet`・`frontier-required`・`frontier-recommended`・`standard`）は `_context.md` の読み替えの表で新しい 3 値として読まれる（AC-13）。
    -   既存のコミットメッセージにある `Opus review:`・`Fable review:`・`Fable scope:`・`Fable finding:`・`Fable review: checkpoint` および理由 `not-fable` は、更新後の判定スクリプト・PR 説明生成・`/runpr` でも引き続き有効な記録として読まれる（AC-22、AC-29）。
-   **OS・実行環境:** リポジトリの既存の前提（macOS および Linux、`bash` 4 以上、`jq` が `PATH` にあること。`CLAUDE.md:824-834`）を維持する。`_lib/review-records.sh` が bash 3.2 互換を保っている契約（`_lib/review-records.sh:70-71`）も壊さない。
-   **前提タスクとの順序:** 本タスクの実装 PR は、前提タスク #1059（タスク 0118：Codex 撤去）の実装が `main` に入った後にマージする。

### 4.5. 保守性 (Maintainability)

-   手順の本文は `.claude/commands/` に 1 か所だけ保ち、`.agents/skills/` は薄い参照ラッパーにとどめることで、コマンド手順の変更時に 2 か所の本文を同期する保守負担を作らない（AC-01、AC-09）。
-   ハーネスごとの実モデルの ID は `_context.md` の対応表 1 か所に集約し、モデルの版や採用モデルが変わったときは `_context.md` の該当行の更新だけで済むようにする（AC-15、AC-17、AC-29）。

## 5. 制約条件 (Constraints)

-   `docs/dev/project_overview.md` の「決定済みの方針」「前提・制約」に従う。
-   `CLAUDE.md` の「Document language」「Language」「Go Idioms」「Protocol numbers」「Testing Strategy」「Tool Execution Safety」および `docs/dev/conventions/dev_tools.md` に従う。
    -   `CLAUDE.md:954-970`（「Go Idioms」のシェルスクリプトに対する日本語リテラル例外）および `docs/dev/conventions/dev_tools.md` Rule 6 の境界を守り、`.claude/commands/` 配下に日本語リテラルを持つ新しい第 3 のシェルスクリプトは追加しない（AC-39）。
    -   `internal/chief` の変更がプロトコル番号を上げる基準（`docs/dev/chief_architecture.md`）に当たるときは、`CLAUDE.md`「Protocol numbers」に従い、タスク文書には具体的な番号を書かず「`main` の値 + 1」と記す。
-   既存のポリシーからの変更点（承認済み先行タスクの決定の更新）は 1 章「要件定義の際に決めたこと」に記録したとおりであり、対応する [ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)・[ADR-0042](../../dev/adr/0042-implementation-model-tiers.md)・[ADR-0046](../../dev/adr/0046-chief-of-staff.md)・[ADR-0050](../../dev/adr/0050-fable-partial-review.md) の更新と [ADR-0055](../../dev/adr/0055-harness-independent-commands.md) の作成を本 PR に含める。

## 6. 用語集 (Glossary)

-   用語は `docs/translation_glossary.md` と統一する（「通常の進め方」「簡易な進め方」「進め方の記録」「プランの PR」「割り当て」「試行」「指摘」「確認」「照合する」「モデルの ID」「開発の道具」「入れる道具」「指示ファイル」「バックグラウンド」など）。
-   本タスクで導入・定義する「ハーネス」「ハーネスのアダプタ」「上位モデルのレビュー」「クロスチェックの部分レビュー」「スクラッチパッド」は 3.1 で定義したとおりとする。`README.md` → `README.en.md` の翻訳やリポジトリ横断の用語統一で必要になる語は、設計・実装段階で `docs/translation_glossary.md` に追加する。
