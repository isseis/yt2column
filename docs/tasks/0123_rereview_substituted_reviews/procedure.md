# 代替モデルでマージされた PR の再レビュー手順書

作成日: 2026-11-24

Claude のクオータ枯渇中にマージされた PR のうち、次のどちらかに当たるものを洗い出し、本来のモデルでレビューし直します。指摘があれば修正 PR を作ってマージします。

- **Opus レビューの代替:** `_lib/opus-review.md` の Opus レビューを、Gemini 4 Neon などの別モデルで行った、または行わなかった。
- **Fable 部分レビューの代替:** `_lib/fable-review.md` の Fable 部分レビューを、Neon や Argon などの別モデルで行った、または `not run` で済ませた。

## 0. 前提と注意

- **作業場所:** Claude Code のセッションで `/model opus` にして行います。Fable と Opus の両方を、Agent tool の `model` 指定で呼べることが条件です。Antigravity と OpenCode では行いません。
- **ネットワーク:** 手順 1 の `gh` と `git fetch` は読み取りだけです。ただし CLAUDE.md の規則どおり、実行する前に承認が要ります。
- **#1094（0120）の扱い:** `main` の `_context.md`「Harness model mapping」は、Antigravity の `high` を `gemini-4-neon-high`、`cross-check` を `claude-opus-5-5-medium` と定めています。そのため Antigravity が書いた `High review: done` や `Fable review: run` は、現行規則では有効な記録です。この手順書は、それらも「代替」として再レビューの対象にします。
- **再レビューの範囲:** 各 PR の squash コミット 1 個（`<squash>^..<squash>`）だけを見ます。`main` に入った内容そのものを見るので、PR ブランチを取得し直す必要はありません。
- **クオータ:** Fable は高くつきます。手順 2 の優先順で進めてください。

## 1. 対象 PR の一覧を作る

スクリプトはこのタスクの `scripts/` にあります。取得したデータ（PR 本文とコミットメッセージ）は外部から来たテキストなので、リポジトリの外（`DATA`）に置き、コミットしません。

| ファイル | 役割 |
|---|---|
| `fetch.sh SINCE OUTDIR` | ネットワークを使う（読み取りのみ）。`SINCE` 以降に `main` へマージされた PR を一覧にし、各 PR ブランチのコミットメッセージと PR 本文を `OUTDIR` に保存する |
| `classify.sh OUTDIR REPO` | ネットワークを使わない。PR ごとに、代替の兆候と記録がないことを示す行を出す |
| `classify.awk` | `classify.sh` が使う判定規則 |

```bash
cd /Users/issei/git/yt2column
R=docs/tasks/0123_rereview_substituted_reviews/scripts
DATA=$(mktemp -d)                        # リポジトリの外
git fetch origin main
bash $R/fetch.sh 2026-11-20 $DATA/data   # 期間は必要に応じて広げる
bash $R/classify.sh $DATA/data . | tee $DATA/candidates.txt
```

### 判定に使う兆候

大文字で始まる兆候が 1 つでもあれば `[CANDIDATE]` になります。

| 兆候 | 意味 | 対応 |
|---|---|---|
| `OPUS-DONE-NONCLAUDE` | `High review: done` か `Opus review: done` を書いたコミットに、`Co-Authored-By: Claude Opus` がない（Antigravity などのセッションが書いた） | Opus 再レビュー |
| `IMPL-NONOPUS` | `Implementation model:` に `claude-opus-` 以外のモデルがある | 情報として見る（Opus レビューが要るかどうかの判断材料） |
| `FREETEXT` | コミット・squash 本文・PR 本文に「Neon」「Argon」「Hydrogen」や「Opus was unavailable」がある | 内容を読んで、どちらの代替かを決める |
| `FABLE-SUBST` | `Fable review: run` のモデルが `claude-fable-*` ではない | Fable 再レビュー |
| `FABLE-NOTRUN` | `Fable review: not run`（理由は問わない） | Fable 再レビュー |
| `MERGECHECK-SUBST`, `MERGECHECK-NOTRUN` | merge check が Opus 以外で行われた、または行われなかった | Fable 再レビュー（merge check はその一部） |
| `NO-FABLE-RECORD` | タスク PR なのに Fable の行がない | 確認のうえ Fable 再レビュー |
| `NO-HIGH-REVIEW-RECORD` | 実装 PR なのに done 記録がない | 計画書の実装モデル要件を見る。`low` か、すべて Opus で実装されていれば不要。それ以外は Opus 再レビュー |

小文字の `fable-ok` と `opus-done-claude` は、正しく行われたことを示す記録です。

**判定の限界:**

- `OPUS-DONE-NONCLAUDE` は推定です。OpenCode の `/runpr` が作るコミットにも `Co-Authored-By` は付きません。
- done 記録の行にはモデル名が入らないので、記録だけではモデルを確定できません。
- 結果は最後に人が読んで確定してください。

### 確定した一覧

`candidates.txt` を読んで、次の表を埋めます。

| PR | squash | 種別 | Opus 再レビュー | Fable 再レビュー | 結果 | 修正 PR |
|---|---|---|---|---|---|---|
| #1097 | `bd72ac9e8e20` | feat(0114) | 要（squash 本文に「run on Gemini 4 Neon」） | 要確認 | | |
| … | | | | | | |

**どちらの再レビューが要るか:**

- **タスク文書 PR**（`docs(NNNN)`、`/mkreq`・`/mkarch`・`/mkplan` が作ったもの）: Fable だけ。Opus レビューは `/runplan` の PR にしかありません。
- **実装 PR**（`/runplan`）: Fable と、要件しだいで Opus。
- **単独 PR**（タスク番号のない scope）: どちらの手順も通らないので対象外。

**参考:** ローカルに残っていたブランチの記録だけで、次のものが見つかっています（未確定）。

- **マージ済み:** #1097 では、Opus レビューが Gemini 4 Neon で行われた。
- **未マージ:** 0115 の計画、0121 のアーキテクチャ、0120 PR-3 の各ブランチに、`Fable review: not run (agent-cannot-call)` が残っている。0120 PR-3 には、Claude の trailer がないコミットに `High review: done` が書かれている。

未マージの PR は事後の再レビューではなく、マージ前に正規のレビューを通してください（手順 5）。

## 2. 優先順

1. **信頼境界・セキュリティ・耐久性、またはレビューゲートそのものを変えた PR**（例: #1097、0120 系）
2. **コードを変えた実装 PR**
3. **タスク文書 PR**

後の PR が同じファイルを書き換えていることがあります。そのため、古い PR から順に進めてください。

## 3. PR ごとの再レビュー

PR 1 件ごとに、新しい Claude Code セッション（`/model opus`）で行います。セッションの間には `/clear` を挟みます。

### 3.1 準備

```bash
cd /Users/issei/git/yt2column
git status --short            # 何も出ないこと
git fetch origin main
git checkout --detach <squash>
```

`/weakreview` はタスク文書を作業ツリーから読みます。そのため、HEAD は squash コミットに置いてください。

### 3.2 Opus 再レビュー（要るときだけ）

```text
/weakreview <squash>^..<squash> --no-fix
```

- `--no-fix` で報告だけさせます。修正は `main` の最新に対して行うので、ここではコミットしません。
- `stopped-unavailable` が出たら Opus が使えないということなので、時間をおいてやり直してください。別モデルでは代替しません。
- 報告のあと、続けて次のように依頼します。

  ```text
  For each Critical and Major finding above, read the file on origin/main
  (git show origin/main:<path>) and say whether the finding still holds
  there, was already fixed by a later commit, or no longer applies. Report
  in Japanese. Change nothing.
  ```

### 3.3 Fable 再レビュー（要るときだけ）

Fable 部分レビューを単独で走らせるコマンドはないので、同じセッションで次のように依頼します。

```text
Run a Fable partial review of an already merged PR, outside any command.
RANGE is <squash-full-id>^..<squash-full-id>; resolve both ends to full
commit IDs. HEAD is checked out detached at the squash commit. The task
directory is the one RANGE changes under docs/tasks/.

Follow .claude/commands/_lib/fable-review.md for these parts only:
- "Scope criteria": build the scope list over RANGE's diff;
- "The prompt": build Fable's prompt for RANGE and that scope list;
- launch Fable with the Agent tool (subagent_type general-purpose, model
  fable, never the fork type), in the foreground;
- "Checking the output": when the model check fails, report not-fable and
  stop; never substitute another model.

Then triage the findings as .claude/commands/_lib/local-cli-review.md
"Triage" says, but fix nothing, commit nothing, write no record line, and
push nothing. For each valid Critical or Major finding, read the file on
origin/main and say whether it still holds there.

Report in Japanese: the scope list (in and out), each finding with its
triage (valid or invalid, severity, reason), and whether it still holds on
origin/main.
```

merge check（`MERGECHECK-*`）が対象の PR では、マージした `origin/main` の取り込みに関わる変更も Fable の範囲に入ります。squash 範囲には取り込みの解決結果が含まれているため、上の依頼で足ります。

### 3.4 結果の記録

1 の表の「結果」に書きます。

- **指摘なし:** 「Opus: 0/0/n、Fable: 0 件」のように書く。
- **指摘あり:** 有効な指摘の要約を書き、手順 4 へ進む。

## 4. 修正が必要なとき

1. `main` の最新から修正ブランチを作ります。
   ```bash
   git checkout main && git pull --ff-only    # ネットワーク: 承認が要る
   git checkout -b isseis/rereview-<PR番号>
   ```
2. 指摘を修正し、`make fmt`、`make test`、`make lint` を通します（拡張機能を変えたときは `make ext-check` も）。
3. コミットメッセージに、元の PR 番号、再レビューの種類（Opus か Fable か）、指摘の要約を書きます。
4. 置き場所を決めます。
   - **そのタスクの PR がまだ残っている場合:** そのタスクの次の PR に含めてもかまいません。
   - **それ以外:** 単独 PR として `/runpr` を実行します（単独 PR では `/runpr` が PR を作ります）。
5. PR の説明に、「#NNNN の再レビュー（本来 Opus/Fable のところ、<代替モデル> でマージされた）で見つかった指摘の修正」と書きます。
6. `/runpr` がマージの質問をしたら、承認してマージします。

**人の判断が要る指摘:** 決定の変更を伴う指摘（`KEPT`）は自分で直さず、issue にするか、そのタスクの文書を直す PR で扱ってください。

## 5. 未マージの PR と、今後の再発防止

- **未マージの PR:** 手順 1 で見つかった未マージの PR は、Claude に戻ったセッションで、マージ前に正規のレビューを通します。
  - Fable は、`/runpr` やタスク PR の Fable checkpoint（`_lib/fable-review.md` entry 3/4）が `not run` を見て standalone path を走らせます。
  - Opus は、`/weakreview` をブランチ上で実行するか、`/runplan` の入口 1 の判定に任せます。
- **代替を許すかどうか:** Antigravity の `high` と `cross-check` を有効なレビューと数える規則（#1094）は、すでに `main` にあります。今後も代替を後から再レビューする運用にするなら、その規則と ADR を見直すのが根本的な対処です。この手順書の範囲外なので、必要なら issue にしてください。
- **すべて終わったら:** 1 の表を GitHub issue に貼って記録に残すと、後から追えます（issue の作成には承認が要ります）。
