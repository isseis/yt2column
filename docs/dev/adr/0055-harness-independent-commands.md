# ADR-0055: 開発コマンドの手順の正本を .claude/commands/ に保ち、ハーネス固有の操作・入口・権限境界をアダプタと薄いスキルに分離する

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

リポジトリの開発コマンド（`/mkreq`・`/mkarch`・`/mkplan`・`/mkplan2`・`/runplan`・`/runpr`・`/fixpr`・`/mergepr`・`/weakreview`・`/japrose`・`/mktrans`）と共有手順は、Claude Code（一部 OpenCode）で動くことを前提に書かれてきた。これらを Antigravity CLI（`agy`）でも同じ品質関門・承認規則・記録契約で動かしたいが、ハーネスの間には次の違いがある。

- **組み込みツールとモデル固定の手段:** 質問ツール、ファイル読み書き、サブエージェントの起動、具体的なモデルの ID を固定して検証する手段、スクラッチパッドの場所、バックグラウンド実行とタイマー・通知の手段がハーネスごとに異なる。特にモデルの固定と検証は、Claude Code の `Agent` ツールに対し、Antigravity では `swarm` ツールと返却 JSON の `"model"` 検証を使う。
- **コマンドの検出方法:** Claude Code は `.claude/commands/*.md` をスラッシュコマンドとして読むが、Antigravity CLI は `.agents/skills/<名前>/SKILL.md` をカスタムスキルとして検出する。
- **権限・サンドボックスと監視対象:** Claude Code はリポジトリの `.claude/settings.json` の前方一致規則で許可と拒否を行うが、Antigravity CLI は `.claude/settings.json` を読まず、OS サンドボックス（既定でネットワークとワークスペース外へのアクセスを遮断）と `BypassSandbox: true` に対する利用者の承認で保護する。また、`.agents/` や `AGENTS.md`・`GEMINI.md` が実行に関わる指示の場所として加わる。

手順をハーネスごとに別々のファイルへ複製すると、手順を変えるたびに二重管理のずれが生じる。一方で、モデル名だけを機械的に置き換えても、ツールやサンドボックスの制約で手順が止まったり、出力の自己申告に頼るモデル検証を別モデルがすり抜けたりするおそれがある。

## 決定

開発コマンドの手順の正本は `.claude/commands/` の 1 か所に保ち、ハーネス固有のツール操作とサンドボックス規則はハーネスのアダプタへ分離し、Antigravity からは `.agents/skills/` の薄い入口で同じ正本を呼び出す。`chief`（[ADR-0046](0046-chief-of-staff.md)）の Antigravity 操作対応は対象外とし、OpenCode は現行の運用範囲にとどめる。

仕様は [0120 要件定義書](../../tasks/0120_harness_independent_commands/01_requirements.md) の F-001・F-002・F-006〜F-008 が持ち、プロジェクト固有の値とハーネス別のモデル対応表は [_context.md](../../../.claude/commands/_context.md) が持つ。

**食い違い:** `.claude/commands/` のコマンド定義と共有手順はまだ Claude Code の組み込みツールを直接指定しており、ハーネスのアダプタと `.agents/skills/` の薄い入口は未作成で、`fixprguard`（[ADR-0015](0015-untrusted-review-threads.md)）とモデル固定レビューの状態チェック（[ADR-0050](0050-fable-partial-review.md)）の監視対象にもまだ `.agents/` 等が入っていない。この食い違いは https://github.com/isseis/yt2column/issues/1072 の実装で解消する。

### 1. 手順の正本とハーネスのアダプタ

- **正本の単一化:** `chief` を除く開発コマンドと共有手順の正本は `.claude/commands/` に 1 つだけ置き、本体はハーネス固有の組み込みツール名を直接呼ばず、抽象化された操作（選択式の質問、一般・段階指定のサブエージェント、モデル固定ワーカー、ファイルの読み書き、スクラッチパッド、タイムアウト付きシェル実行、タイマー待機、利用者への通知）で記述する。
  - **理由:** コマンド手順の二重管理と仕様のずれを防ぐためである。
- **アダプタの分離と決定的な判別:** ハーネス固有のツール呼び出し・モデル検証・一時領域・タイマー・サンドボックス規則は、Claude Code 用と Antigravity 用の 2 つのハーネスのアダプタ文書に切り出す。実行中のハーネスは、セッションで利用可能な組み込みツールの有無によって決定的に判別する。
  - **理由:** ハーネスごとの制約（Antigravity でモデル固定や `invoke_subagent` の 4 段階列挙にない中間モデル（`standard` 役割の `gemini-4-neon-medium` 等）の指定に `swarm` と終了時の `kill` が要ること、タイマーにシェルの `sleep` ではなく `schedule` を使うこと、通知ツールがないときは最終報告を使うこと）を本体に混ぜずに守らせるためである。
- **OpenCode の範囲:** OpenCode 用の汎用サブエージェントアダプタは新設せず、現行の運用範囲（`/runpr` と実装モデル要件が `low`（[ADR-0042](0042-implementation-model-tiers.md)）の `/runplan`）を維持する。モデル固定ワーカーを呼べない箇所では、各手順の規則（エスカレーションは安全側の `ask`、上位モデルのレビューは停止、クロスチェックの部分レビューは `not run` の記録）に従う。
  - **理由:** 現在の運用で使っていない機能を足さないためである（[CLAUDE.md](../../../CLAUDE.md) の「Key Design Patterns」の YAGNI）。

### 2. Antigravity からの呼び出しの入口と `chief` の境界

- **`.agents/skills/` の薄い入口:** `chief` を除く各開発コマンドについて、`.agents/skills/<コマンド名>/SKILL.md` に英語の薄いスキル定義を置き、対応する `.claude/commands/<コマンド名>.md` と `_context.md`・Antigravity 用のアダプタを読んで実行させる。手順の本文は複製しない。これらのファイルは [CLAUDE.md](../../../CLAUDE.md) の「Document language」原則 (b) の英語の指示文書（[ADR-0035](0035-language-policy.md)）に位置づけ、文書のパス解決およびコマンド指示の信頼境界ガードの対象にも加える。
  - **理由:** 手順の正本を 1 か所に保ったまま、Antigravity から毎回ファイルパスを書かずにカスタムスキルとして起動できるようにし、`.claude/commands/` と同じガードで守るためである。
- **`chief` との境界:** `chief`（[ADR-0046](0046-chief-of-staff.md)）の Antigravity 操作対応（Herdr 連携、`transcript.jsonl` の読み取り、pane への入力送信、`/chief` 自体の Antigravity 実行）は別タスクとし、`.agents/skills/` に `chief` の入口は置かない。本決定では、`chief` が計画書の新しい `**実装モデル要件**` の値（[ADR-0042](0042-implementation-model-tiers.md)）と改名後のルーティング行のキーを読めるように対応させ、既存の送り先エージェント（`Claude Code` と `OpenCode`）を維持するにとどめる。
  - **理由:** `chief` の Antigravity 対応は実画面と記録の標本採取を伴う独立した大きな変更であり、開発コマンド単体の Antigravity 対応を先に完了させる方が手戻りと変更規模を抑えられるためである。

### 3. Antigravity のサンドボックス実行規則・権限境界と監視対象の拡張

- **標準サンドボックス優先と最小限の昇格:** Claude Code 向けの `.claude/settings.json` は維持する。Antigravity では、コマンドを既定の標準サンドボックス（`BypassSandbox: false`）で先に試す。ワークスペース外の `.git` worktree 参照・ネットワーク通信（`gh`・`coderabbit review`・`numreserve`・`git fetch`・`git push` 等）・ループバックソケット制限で拒否されるコマンドに限って、単一のコマンド（リポジトリが定める判定・状態チェック用スクリプトの単一起動を含む）だけを分離して `BypassSandbox: true` で実行する。前後の編集・ビルド・テストを即席に連結して一緒に昇格させず、前方一致で承認しやすいコマンドの形を保つ（ただし PR タイトルのシェル展開を防ぐ規則を優先する）。
  - **理由:** 利用者の手動承認が要る範囲を本当に必要な単一コマンドだけに絞り、確認のたびに無関係な操作までサンドボックス外で動くことを防ぐためである。
- **禁止コマンドの適用と git 無視ファイルの監視拡張:** インストール型ツールのインストール・ビルド禁止（[ADR-0014](0014-deterministic-checks-in-go.md)）、`herdr` と `git worktree` の変更操作の禁止（[ADR-0046](0046-chief-of-staff.md)）、記録タグの直接操作禁止とネットワーク操作の事前承認規則（[CLAUDE.md](../../../CLAUDE.md) の「Tool Execution Safety」）は、Antigravity セッションにも等しく適用し、`AGENTS.md`・`CLAUDE.md`・アダプタ文書に明記する。あわせて、git が無視する `.agents/` 配下および指示ファイル（`agents.md`・`agent.md`・`gemini.md`・`agents.override.md`）を `fixprguard` の監視の一覧（[ADR-0015](0015-untrusted-review-threads.md)）とモデル固定レビューの状態チェック（[ADR-0050](0050-fable-partial-review.md)）の両方に加える。
  - **理由:** ハーネスが変わっても開発ツールの信頼の境界を同じ規則で守り、git 無視ファイルを経由した不正スキル・指示ファイルの残留を防ぐためである。

## 採らなかった案

以下は、コマンド配置、対応ハーネスの範囲、`chief` との境界、および権限強制の手段について比べて採らなかった案と理由である。

- **コマンド手順を `.claude/commands/` と `.agents/skills/` の両方に書く案:** 手順を直すたびに両方を同期する必要が生じ、片方だけが古くなる。
- **`.agents/skills/` を置かず、Antigravity では毎回 `@[.claude/commands/...]` のファイル参照で実行する案:** 薄い入口の追加は不要だが、毎回の起動にファイルパス指定の手間が残る。
- **OpenCode も含めた 3 ハーネスすべての汎用アダプタを作り、全コマンドを揃える案:** 現在の運用で使っていない OpenCode 向けのサブエージェント機能まで作ることになり、YAGNI に反する。
- **`chief` の `Command routing` 表への Antigravity 追加や、Herdr・`transcript.jsonl`・pane 送信の対応まで同じタスクに含める案:** 前者は送信できないエージェント値を `chief` に持たせる中間状態を作り、後者は標本採取を伴う大きな変更を同じタスクに抱え込むことになる。
- **Antigravity 向けに Hook やプラグインで禁止コマンドを機械的に拒否する仕組みを新設する案:** 機械的な拒否は増えるが、ハーネス固有の仕組みの保守対象が増える。
- **`fixprguard` の監視の一覧への `.agents/` 等の追加を別タスクへ先送りする案:** `/fixpr` の経路で git 無視された不正スキルの書き込みを検出できない期間が残る。

## 結果

本決定により、開発コマンドの正本を 1 つに保ったまま複数ハーネスで実行できる一方、次の運用条件と残余の危険が生じる。

- **正本の保守とスキルの追加:** `chief` を除く開発コマンドを Claude Code と Antigravity CLI のどちらからでも実行でき、手順の変更は `.claude/commands/` の正本だけを直せばよい。新しい開発コマンドを足すときは、`.agents/skills/` にも薄い入口を足す。
- **プロバイダへの送信:** Antigravity で開発コマンド・サブエージェント・`swarm` を実行すると、リポジトリのコード・差分・文書が Antigravity の接続先プロバイダへ送られる。利用者がそのハーネスを選んで実行することでこれを承認したものとする。
- **外部 worktree での承認操作とツール再インストール:** 外部の git worktree で Antigravity を使う場合、`git`・`gh`・`coderabbit review`・`numreserve` の呼び出しには `BypassSandbox: true` に対する利用者の承認が要る。また、プロトコル番号が上がる `fixprguard` と `chief` はマージ後に利用者が再インストールする。
- **機械的 `deny` 不在の残余の危険:** Antigravity には `.claude/settings.json` のようなリポジトリ内の機械的 `deny` 設定がないため、禁止コマンド（インストール型ツールのインストール・ビルド、`herdr` や `git worktree` の変更操作）は指示ファイル・アダプタ文書と `BypassSandbox: true` 時の利用者承認で守る。エージェントが指示に反する残余の危険は受け入れる（[ADR-0014](0014-deterministic-checks-in-go.md) と同じ整理）。

## 経緯

- 2026-11-21 `chief` を除く開発コマンドの手順の正本を `.claude/commands/` に保ったまま、ハーネス固有の操作とサンドボックス規則を Claude Code 用・Antigravity 用のアダプタに分離し、`.agents/skills/` に薄い入口を置くことにした。OpenCode は現行範囲を維持し、`chief` の Antigravity 操作対応は別タスクに切り出し、`fixprguard` と状態チェックの監視対象に `.agents/` と指示ファイルを加え、Antigravity に機械的 `deny` 設定がない点は受け入れる危険とした（[0120 要件定義書](../../tasks/0120_harness_independent_commands/01_requirements.md)「issue #1072 で決めたこと」「要件定義の際に決めたこと（2026-11-21）」）
