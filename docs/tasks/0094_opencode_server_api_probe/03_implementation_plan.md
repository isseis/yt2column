# 実装計画書：OpenCode のサーバの API で chief の判定と送信を置き換えられるかの調査

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-11-16 |
| Review date | 2026-11-17 |
| Reviewer | isseis |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下、要件書）の項番を、節番号は [02_architecture.md](02_architecture.md)（以下、設計書）の節を指す。本タスクに `design_handoff.md` と `implementation_handoff.md` はない。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

使い捨ての workspace で OpenCode のサーバの API を確かめ、問い 1〜7 の答えと再現の手順を `docs/research/opencode_server_api/README.md` に記録する。コード・コマンドの定義・`.claude/settings.json` は変えない。

### 1.2. 実装の原則

-   **書き込みは、設計書 3.2 節の確かめを通った実験のセッションだけに送る**（AC-18）。
-   **エージェントは pane へ入力を送らない。** TUI への入力と pane の起動は人に頼み、エージェントは `opencode api` と herdr の読み取りだけを使う（設計書 1 章）。
-   **記録には形だけを写す**（AC-17）。
-   **サーバの全体を返す読み取り（`session.list`・`session.active`・`permission.request.list` など）は、`/openapi.json` のスキーマで名前を確かめた形のフィールドに絞ってから読む**（設計書 3.2 節）。中身のフィールドが要るときは、宛先の一覧のセッションのものだけに絞る。
-   **答えが出ない問いも、節を省かずに理由を書く**（AC-03）。

### 1.3. 計画の際に決めたこと（2026-11-16）

-   **実装の PR は 1 本とし、フェーズも 1 つにする。** 成果は調査記録 1 つで、問いの答えは互いに参照しあう（問い 4 の対応は問い 3 の `/new` の結果に左右される、など）。分けるとレビューのたびに前提を読み直すことになるためである。ADR: なし（本タスクの中で閉じる）。
-   **LLM の費用は、実験を始める前に 1 度だけ人に承認を得る（ステップ 1-1）。** 規模は、OpenCode を通した DeepSeek V4.1 Flash の呼び出しが数十回、`claude -p` が 30 回程度（形ごとに 2 回と、試し直しと目印の確かめ）である。呼び出しごとに尋ねると実験が細切れになるためである。規模を超えそうなら止めて尋ねる（設計書 4 章）。ADR: なし。

### 1.4. 既存コード調査結果

コミット `7669dc0eda84e8a2a1cb5020629d1e96545c4f45` で確かめた。コードは変えないので、変える箇所はない。実験の前提として次を確かめた。

-   `opencode --version` は `opencode v2.2.6` を返す。`opencode --help` は、`--standalone`（私用のサーバ）・`--server <URL>`・`--session, -s <ID>`（そのセッションを続けるか、なければ作る）のフラグと、`api`・`run`・`session`・`service` のサブコマンドを示す。
-   `claude --help` は、`--settings <file-or-json>`（追加の設定を読む）、`-p, --print`、`--setting-sources <sources>`（設定の読み込み元を限る）、`--permission-mode <mode>`、`--allowedTools`・`--disallowedTools`、`--output-format <format>`（`--print` でだけ効く）を示す。
-   `opencode run --help` は、`--session, -s <ID>`（そのセッションを続けるか、なければ作る）を示す。
-   `.claude/commands/runpr.md` の 1 章（104〜128 行）は、`git fetch origin main` と PR の検索の後、PR がなく、ブランチが detached の `HEAD` なら止まる。
-   `.claude/settings.json:8`〜`33` の拒否は herdr の呼び出しだけで、`herdr pane read` などの読み取りは拒まない。`.claude/settings.local.json` は `.gitignore:3` で無視される。
-   2026-11-17 に、`claude` 2.1.326 と `opencode` v2.2.6 で次を確かめた（`--help` と実行ファイルの読み取りだけ）。
    -   `claude --help`：`--permission-mode` の選択肢は `acceptEdits`・`auto`・`bypassPermissions`・`manual`・`dontAsk`・`plan`。`--tools <tools...>`、`--no-session-persistence`、`--output-format stream-json`（`--print` では `--verbose` が要る）がある。`-p` では、検査で退けた設定ファイルは黙って無視される。
    -   `opencode --help`：`--auto`（拒否されていない権限を自動で許す）・`--prompt`・`acp`・`mini` がある。`opencode run --help` は `--continue, -c`（直前のセッションを続ける）と `--fork` を示す。
    -   `~/.opencode/bin/opencode2` は、`opencode` を呼ぶだけのシェルスクリプトである。
    -   実行ファイルには、`session.interrupt`・`session.remove`・`session.shell`・`session.synthetic`・`session.skill` の操作の名前がある。
-   chief が OpenCode の pane に送る手順は `internal/chief/send.go:425`〜`427`、モデルの確かめは同 `:351`〜`352`、質問の読み取りは `internal/chief/screen_opencode.go:121` にある。調査記録は、これらと API の操作を対応づけるときに参照する。

## 2. 実装のステップ (Implementation Steps)

### フェーズ 1: 実験と調査記録（設計書 3 章・6 章、AC-01〜AC-19）

#### ステップ 1-1: 環境の用意と承認

**変えるファイル:** なし

- [x] 人に、使い捨ての worktree `~/.herdr/worktrees/yt2column/probe-0094`（`origin/main` から detached）と、それを開く Herdr の workspace `probe-0094` を作り、OpenCode の pane を 1 つ、送り先の表の `runpr` の行のモデルで、`--auto` なしで起動するよう頼む（設計書 2 章・3.1 節・3.3 節）。以後の pane の起動も `--auto` なしで頼む
- [x] `opencode --version` を記録し、1.4 節の版と違えば人に知らせてから続けるかを尋ねる
- [x] 実験で行う LLM の呼び出しの規模（1.3 節）と、`/runpr` が行う読み取りだけのネットワークの呼び出し（`git fetch origin main`・`gh pr view`・`gh pr list`）を示し、人の承認を得る
- [x] 作業ディレクトリを使い捨ての worktree にして `session.list` を読み、ID と作業ディレクトリのフィールドだけに絞ってから、人の示す画面と突き合わせて実験の pane のセッションの ID を特定し、設計書 3.2 節の宛先の一覧に入れる。この時点では `/openapi.json` をまだ読んでいないので、絞るフィールドは `opencode api GET /openapi.json` の `Session` のスキーマを先に読んで決める（設計書 3.2 節）

**完了の条件:** 承認があり、実験のセッションの一覧が 1 つ以上の ID を持つ。

#### ステップ 1-2: 読み取りだけの問い（問い 5、問い 6 の前半）

**変えるファイル:** なし（観測は作業メモに残し、ステップ 1-6 で記録に書く）

- [x] 問い 5：`/openapi.json` の版の情報、応答のヘッダ、存在しない操作への要求の応答、スキーマが追加のフィールドを許すかを確かめる（AC-12・AC-13）
- [x] 問い 6 の前半：サーバへの届き方（TCP かソケットか、その場所の知り方）と認証の有無を確かめる（AC-14）

**完了の条件:** AC-12・AC-13・AC-14 の観測がそろう。

#### ステップ 1-3: 問い 4（pane とセッションの対応）

**変えるファイル:** なし

- [x] 人に、同じ worktree に 2 つ目の OpenCode の pane を起動するよう頼み、`session.list`（作業ディレクトリで絞る）・`session.active` を 1.2 節のとおり形のフィールドに絞って読み、画面と突き合わせる
- [x] 人に、`opencode --session <ID>` で起動した pane、TUI でのセッションの切り替え、`/new` を順に行うよう頼み、各場合に候補の方法で pane のセッションを決められるかを確かめる（AC-11）
- [x] 人の操作のたびに、設計書 3.2 節の規則 3 で pane のセッションを特定し直し、宛先の一覧を直す
- [x] `session.list` の結果が、`opencode api` を呼んだ作業ディレクトリで変わるかを確かめる（設計書 3.1 節）

**完了の条件:** AC-11 の 3 つの場合の観測が、2 つ以上の候補についてそろう。

#### ステップ 1-4: 問い 1・2・7（`form`・権限の確認・`event.subscribe`）

**変えるファイル:** なし

- [x] `event.subscribe` の購読を背後のプロセスで始め、イベントの種類とセッションの ID だけを、実験のセッションのものに絞ってイベントの控えに書く。フィールドの場所はステップ 1-2 で読んだイベントのスキーマから取り、絞る前の流れは端末にもファイルにも出さない（問い 7、設計書 3.3 節）
- [x] 1 つを選ぶ問い・複数を選ぶ問い・タブのある問いのそれぞれを、決まったプロンプトで実験のセッションに出させ、`session.form.list` を読み、`session.form.reply` で答え、TUI の画面を読む（AC-05・AC-06）
- [x] OpenCode の権限の設定を読み取りだけで確かめ、確認を求める操作のうち、確認なしに実行されても害のない操作（使い捨ての worktree の中の存在しないファイルの読み取りなど）を選ぶ。そのような操作が設定になければ送らない。その操作を求め、`permission.request.list`（1.2 節のとおり絞る）と `session.permission.list` を読み、API で拒否を答える。60 秒のうちに確認が出なければ「確かめられなかった」とし、より強い操作で試し直さない（AC-07、設計書 3.3 節）
- [x] イベントの控えの種類を、問いと確認の出現・終わりに対応づけ、購読を止める（AC-16）

**完了の条件:** AC-05・AC-06・AC-07・AC-16 の観測がそろう。

#### ステップ 1-5: 問い 3（モデル・コマンド・`/new`）と問い 6 の後半（拒否の試行）

**変えるファイル:** なし（一時的な設定ファイルは scratchpad に置く）

- [x] `session.switchModel` で実験のセッションのモデルを切り替え、画面のモデルの行と、次の実行の `session.message.list` のモデルを確かめる（AC-08）
- [x] `session.command` で `/runpr` を送り、人が 2 つ目の pane の実験のセッションで TUI に `/runpr` を打ったものと比べる。そのセッションも、設計書 3.2 節の規則 3 で作業ディレクトリを確かめてから宛先の一覧に入れる。どちらかが PR を探した後の段階に入ったら、そのセッションを `session.interrupt` で止めて人に知らせる。止まったと応答しなければ、人に止めるよう頼む（AC-09、設計書 4 章）
- [x] `/new` に当たる操作を API で行い、TUI の pane がどのセッションを示すかを確かめる。作ったセッションは設計書 3.2 節の規則 2 で確かめる（AC-10）
- [x] 入力を送る呼び出しの形（要件書 AC-15 の (a)〜(e)）を挙げ、形ごとに、対照の試行と拒否の候補を足した試行を `claude -p` で行う。宛先を指定できない形は試さず、「確かめられなかった」とする。最初に 1 度、目印の規則を除いた設定ファイルで、目印のコマンドが拒まれることを確かめる。作業ディレクトリ・固定するフラグ・リポジトリの設定の許可を打ち消す拒否・完全なコマンドの許可の規則・目印の規則・判定の 3 つの条件・宛先の明示と確かめ直し・資格情報の扱いは設計書 3.4 節のとおりにする（AC-15）

**完了の条件:** AC-08〜AC-10・AC-15 の観測がそろう。

#### ステップ 1-6: 調査記録と片付け

**変えるファイル:** `docs/research/opencode_server_api/README.md`（新規）、本計画

- [x] 設計書 3.5 節の構成で記録を書く（AC-01〜AC-17）。答えが出なかった問いは、その理由を書く（AC-03）
- [x] 記録を読み直し、API の応答・画面から写した本文がないことを確かめる（AC-17）
- [x] 片付ける。宛先の一覧にある実験のセッションだけを、それぞれ設計書 3.2 節の規則 1 で確かめてから API で消す（規則 4。消せなければ人に示す）。一時的な設定ファイルとイベントの控えを消し、購読のプロセスが残っていないことを確かめる。人に、使い捨ての workspace と worktree を消すよう頼む。片付けの手順を記録に書く（設計書 2 章の表）
- [x] 記録をコミットした後、7 章の AC-19 の diff の記録を、[requirements_process.md](../../dev/conventions/requirements_process.md) §4「`diff` verification」の固定コマンドと記録の形で、別のコミットで書く（ファイルの一覧を含む）

**完了の条件:** 記録がコミットされ、7 章の AC-19 の記録がある。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | 成果物 |
|---|---|
| M1（フェーズ 1） | `docs/research/opencode_server_api/README.md`、本計画の 7 章の記録 |

### 3.2 PR 構成

フェーズ 1 のすべて（ステップ 1-1〜1-6）を 1 本の PR にする。

**実装モデル要件**: opus

**判定理由**: ステップ 1-2〜1-5 は前例のない実地の確かめで、観測から答えを判断し、書き込みの宛先を毎回確かめる必要があり、Sonnet では宛先の確かめの抜けや観測の読み違いが起きやすい

**自動送信**: hold

## 4. テスト戦略 (Test Strategy)

-   **単体テスト・統合テスト:** 足さない。コードを変えない。
-   **既存のテスト:** PR のグリーンゲート（`make test && make lint`）は、コードを変えないので今のまま通る。
-   **記録した diff の読み取り:** AC-19 を、[requirements_process.md](../../dev/conventions/requirements_process.md) §4「`diff` verification」の記録した diff の読み取りで確かめる。比べるパスと判定は 7 章に書く。
-   **読んで確かめる:** 記録の内容（AC-01〜AC-18）は、PR のレビューで読む。
-   **後方互換のテスト:** N/A。

## 5. リスク管理 (Risk Management)

| リスク | 対策 |
|---|---|
| 実際のタスクのセッションに書き込む | 書き込み（削除と中断を含む）の前に毎回、設計書 3.2 節の確かめを行う。問い 6 の試行の宛先も実験のセッションにし、子の `claude -p` を起動する直前に確かめ直す |
| 問い 6 で、拒否の候補が効いたと誤って記録する | 設計書 3.4 節の目印の規則（規則がなければ拒まれることを確かめたもの）と 3 つの条件で判定し、そろわなければ「確かめられなかった」とする |
| 記録にセッションの中身が入る | 形だけを写す規則（AC-17）で書き、ステップ 1-6 で読み直す |
| 問い 1 で、モデルが質問ツールを使わない | プロンプトを変えて試し直す。それでも出なければ「確かめられなかった」として理由を書く（AC-03） |
| OpenCode が実験の途中で更新される | ステップ 1-1 で版を記録し、途中で変わったら人に知らせ、どの版での観測かを記録に分けて書く |
| LLM の費用が承認の規模を超える | 超えそうなら止めて尋ねる（1.3 節） |

## 6. 実装チェックリスト (Implementation Checklist)

- [ ] フェーズ 1 の PR マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6）

## 7. 受け入れ基準の検証 (Acceptance Criteria Verification)

| AC | 種類 | 検証 |
|---|---|---|
| AC-01 | `manual` | PR のレビューで、記録の冒頭に日付・版・時点の記録であることがあるかを読む |
| AC-02 | `manual` | PR のレビューで、問い 1〜7 の節がそれぞれ答え・観測・再現の手順を持つかを読む |
| AC-03 | `manual` | PR のレビューで、答えが「確かめられなかった」「一部」の節が理由を書くかを読む |
| AC-04 | `manual` | PR のレビューで、確かめていない事項と前提の節を読む |
| AC-05 | `manual` | PR のレビューで、問い 1 の節の 3 種の問いの記述を読む |
| AC-06 | `manual` | PR のレビューで、`form.reply` の後の TUI の観測と答えの値の検査の記述を読む |
| AC-07 | `manual` | PR のレビューで、問い 2 の節を読む |
| AC-08 | `manual` | PR のレビューで、問い 3 の節のモデルの切り替えの記述を読む |
| AC-09 | `manual` | PR のレビューで、問い 3 の節の `/runpr` の比較を読む |
| AC-10 | `manual` | PR のレビューで、問い 3 の節の `/new` に当たる操作の記述を読む |
| AC-11 | `manual` | PR のレビューで、問い 4 の節の候補と 3 つの場合の記述を読む |
| AC-12 | `manual` | PR のレビューで、問い 5 の節の版の情報の記述を読む |
| AC-13 | `manual` | PR のレビューで、問い 5 の節の 2 つの応答の記述を読む |
| AC-14 | `manual` | PR のレビューで、問い 6 の節の届き方と認証の記述を読む |
| AC-15 | `manual` | PR のレビューで、問い 6 の節の形・候補・試行の結果・残る危険を読む |
| AC-16 | `manual` | PR のレビューで、問い 7 の節を読む |
| AC-17 | `manual` | PR のレビューで、記録が API の応答と画面の本文・秘密を含まないかを読む |
| AC-18 | `manual` | PR のレビューで、再現の手順の書き込みの呼び出しが実験のセッションを宛先にし、宛先の確かめを含むかを読む |
| AC-19 | `diff` | 記録した diff の読み取り（requirements_process.md §4「`diff` verification」）。比べるパスは `docs/tasks/0094_opencode_server_api_probe/01_requirements.md` と `docs/tasks/0094_opencode_server_api_probe/02_architecture.md` で、本計画は含まない。AC-19 は変わるファイルを限るので、記録は差分全体のファイルの一覧も持つ。コマンドと記録の形は同節の「Fixed commands」と「Diff record form」により、記録は `/runplan` が下に書く。ファイルの一覧が `docs/research/opencode_server_api/README.md`（新規）と本タスクのディレクトリの中のファイルだけを挙げるなら、AC-19 を満たす。調査記録は比べるパスに入れない。その中身は AC-01〜AC-18 として PR のレビューで読み、ほかのパスの変更はファイルの一覧に現れるためである |

Diff record (AC-19):

- Compared paths: `docs/tasks/0094_opencode_server_api_probe/01_requirements.md` `docs/tasks/0094_opencode_server_api_probe/02_architecture.md`
- Merge-base: `6968c4897214eec5d750d2e9933cc445c8658d17`
- Compared commit: `9161469018b65c2bd3e7324d50f7462d5852a040`

Path diff:

```text
```

File list (only for an AC that limits which files change):

```text
docs/research/opencode_server_api/README.md
docs/tasks/0094_opencode_server_api_probe/03_implementation_plan.md
```

## 8. 成功の基準 (Success Criteria)

-   AC-01〜AC-19 の検証が済み、7 章に AC-19 の記録がある。
-   #906・#907 が、記録の問いの節を引用できる。
-   `make test` と `make lint` が通る。

## 9. 次のステップ (Next Steps)

-   PR のマージの後、`/runpr` が issue #903 を閉じる。#906・#907 の `/mkreq` が、記録を前提として引用する。

## 変更履歴

- 2026-11-17 承認（isseis、#911）
- 2026-11-17 draft への戻し：設計書 3.2〜3.4 節の守りの強化をステップに反映し、2026-11-17 に確かめた前提を足した（#941）
- 2026-11-17 承認（isseis、#941）
