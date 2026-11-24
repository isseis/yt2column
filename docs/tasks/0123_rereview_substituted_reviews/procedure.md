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

2026-11-24 に `fetch.sh 2026-11-20` を実行して作りました。対象は 2026-11-20 以降に `main` へマージされた 34 本で、`classify.sh` が候補とした 25 本を人の目で読んで分けています。

- **最初の代替:** #1070（2026-11-20 マージ）。それより前の 2026-11-20 の文書 PR（#1049・#1050・#1056・#1058・#1060・#1062・#1063・#1065）は、Fable が `claude-fable-5-2` で実行されています。
- **行の順:** マージの順。**優先度:** 1 は信頼境界・セキュリティ・レビューゲート・CI ゲートを変えた PR、2 はそれ以外の実装 PR、3 はタスク文書 PR。
- **「代替の根拠」の欄:** PR ブランチの記録行と PR 本文から読んだものです。

#### A. 代替でレビューされた PR（クオータ枯渇による。再レビューの対象）

| 優先度 | PR | squash | 種別 | Opus 再レビュー | Fable 再レビュー | 代替の根拠 | 結果 | 修正 PR |
|---|---|---|---|---|---|---|---|---|
| 2 | #1069 | `7bf8f68b08f0` | docs(0117) chief_architecture.md | 要 | 要 | Opus レビューを Gemini のセッションでの Codex CLI レビューで「完了扱い」にした。Fable は `not run`。実装は OpenCode | | |
| 3 | #1070 | `dc224b5c7754` | docs(0118) 設計書 | 不要 | 要 | Fable を Gemini 4 Neon のフルレビューで代替 | | |
| 3 | #1074 | `6a266e990548` | docs(0118) 計画書 | 不要 | 要 | Fable を `gemini-4-argon-medium`（1 回目は Argon 以外）で代替 | | |
| 3 | #1075 | `b004f8c01ba2` | docs(0119) 要件・ADR-0054 | 不要 | 要 | Fable を `gemini-4-argon-medium` で代替 | | |
| 3 | #1076 | `d45909cf9d36` | docs(0119) 設計書 | 不要 | 要 | Fable を `gemini-4-argon-medium` で代替、残りは `not run` | | |
| 1 | #1077 | `21400a98ad5b` | refactor(0118) codexreview 撤去 | 不要（`claude-opus-5-5-high` で実行） | 要 | Fable を `gemini-4-argon-medium` で代替（記録は `not-fable`） | Fable（`claude-fable-5-2`、2026-11-24）: Major 1・Minor 2。Major は有効で `origin/main` でも残る（`settings.json` の allow 規則と `defaultMode` を見るガードが、`cmd/codexreview` の削除で失われた）。Minor 2 件のうち、1 件（計画書の editorial correction の扱い）は記録上の問題でコードに害はなく、もう 1 件（ADR-0013 の食い違いの記述）は後の PR で解消済み | #1103（マージ済み、2026-11-24） |
| 3 | #1078 | `3fa62267b0c8` | docs(0119) 計画書 | 不要 | 要 | Fable を `gemini-4-argon-high` で代替（記録は `not run`） | | |
| 3 | #1079 | `8fbbfc4b8b77` | docs(0120) 要件 | 不要 | 要 | Fable を `gemini-4-argon-medium` で代替、残りは `not run` | | |
| 1 | #1080 | `4e8fd02e21e0` | feat(0118) fixprguard review-state | 要 | 要 | Opus を `gemini-4-neon-high`、Fable を `gemini-4-argon-high` で代替 | Opus（`claude-opus-5-5`、2026-11-24）: 0/1/9。Major は `review_state.go` の読み取りの上限（GraphQL のページ数、1 ページの件数、CheckSuite と CheckRun の数、`gh api user` の大きさ）と期限切れの扱いにテストがなく、壊しても通る。`origin/main` でも残る。Fable（`claude-fable-5-2`、2026-11-24）: Major 1・Minor 7。Major は、設計書 3.3.1 で厳密な検査に `null` を許す例外（(b)・(c)）を足したのに、編集上の訂正（決定の変更なし）として記録した。動作は安全側だが決定の変更に当たり、1.4 に記載がない。`origin/main` でも残る（決定の変更なので人の判断が要る）。Minor のうち `--request` の前後の空白、CheckRun 名の確認がクエリの文面だけであること、`unreadable` の `now` が空であることの 3 件は、Opus の Minor と重なる | #1110（Opus の Major の修正、マージ済み、2026-11-24）。Fable の Major は決定の変更なので #1109 で扱う |
| 1 | #1081 | `fc4bb3dfbb5a` | feat(0119) internal/ciskip | 要 | 要 | Opus を Neon High、Fable を `gemini-4-argon-high` と `claude-opus-5-5-medium` で代替。実装も Neon High | Opus（`claude-opus-5-5`、2026-11-24）: 0/1/9。Major は `TestValidateDepDirsFile` の 64 KiB+1 の行が、1 行だけで `/` で終わる入力なので、読み取りの上限（`LimitReader` の `+1`）を壊しても末尾 `/` の検査で拒否されて通る。行の境目で切れた依存一覧を受け入れる誤りを検出できない。`origin/main` でも残る。Fable（`claude-fable-5-2`、2026-11-24）: Major 1・Minor 3。Major は、親から head への差分だけで非コードと判定するため、`main` がファイルを非コードのパスからコードのパスへ移した後に head が旧パスを編集すると、合成マージコミットではコードが変わるのにスキップされる（作業用リポジトリで再現）。設計書 §3.3.2・§5 が扱っていない場合で、`origin/main` でも残る（判定方式の変更なので人の判断が要る）。Minor のうち submodule の `ignore = all` で差分から抜ける件は Opus の Minor と重なる。CLAUDE.md の例外が未実装の `gh` の読み取りを含む件と、ADR-0054・ADR-0014 の「未作成」の記述の件は、後の PR で解消済み | #1113（Opus の Major の修正、マージ済み、2026-11-24）。Fable の Major は判定方式の変更なので #1112 で扱う |
| 3 | #1083 | `b93a22023ef7` | docs(0120) 設計書 | 不要 | 要 | Fable は `not run`（Antigravity のセッション） | | |
| 1 | #1084 | `720fc313c595` | feat(0118) 手動 CodeRabbit・メンション拒否 | 要 | 要 | Opus が `stopped-unavailable` で Neon High に代替、Fable は `not run` | Opus（`claude-opus-5-5`、2026-11-24）: 0/2/5。Major の 1 件目は、CLAUDE.md の「`make test` に `jq` が要る」の項目が、この PR で削除した `TestHandoffListingPrintsNoBody` と、なくなった `_lib/task-pr-handoff.md` Part B step 5 を挙げたままだったこと。`origin/main` でも残っていた。2 件目は、削除したテストの coverage-compare の出力がコミットのメッセージでは要約だけだったこと。ただし PR の説明に、最後の行（`coverage-compare: PASS`）までの全文があるので、記録は残っている。Minor は、`runpr.md` step 4 の状態の変化で `none` に変わったときの扱いがないこと、`AppendPR` で大きさの検査がメンションの検査より先に走ること、`current-body-mention` の判定が検査の順序に頼っていること、計画書の `TestPostingPathsCheckMention` の説明が実際の検査より強いこと、テスト名 `TestPostTextMentionExitsUsage` が内容と合わないこと。Fable（`claude-fable-5-2`、2026-11-24）: Major 0・Minor 6。`<mention-check>` の `grep` が NUL や不正な UTF-8 を含むファイルを見逃しうること、`pr-review-escalation.md` の判定基準 4 のゲートの一覧が設計書より変わったこと、`weakreview.md` が「最初の PR レビュー」を前提にしていること、`fixprguard_guide.md` の 467 行目の CodeRabbit の解釈に関する主張が確かめられていないこと、範囲から外した 2 ファイルの判断の誤り。`fixprguard_guide.md` 10.4 の件は後の PR で解消済み | #1116（Opus の Major 1 件目の修正、マージ済み、2026-11-24） |
| 3 | #1085 | `3562af5a17b3` | docs(0120) 計画書 | 不要 | 要 | Fable を `gemini-4-neon-high` で代替（記録は `not run unavailable`） | | |
| 1 | #1086 | `cd4a90aef0ae` | feat(0119) ciskip GitHub API 検証 | 要 | 要 | Opus を Neon High で代替（HTTP 429）、Fable は `not run` | Opus（`claude-opus-5-5`、2026-11-24）: 0/0/9。Minor は、`git.go` の `validateDepDirsFile` を `O_NONBLOCK` で開いて通常ファイル以外を拒否するよう変えたのに、計画書（ステップ 1-2 など）と設計書が `os.Open` のままであること、要件の「閉じた列挙値」に対し `status`・`conclusion` を非空文字列として受けること、`ErrWaitDelay` を常に `api_timeout` にすること、通常ファイルの検査・`New` の配線・`headRepoNull`・呼び出し前の期限の検査を壊しても通るテストがあること、`prRunMatch` の `switch` に安全側の `default` がないこと、設計にない RFC 3986 の URI 検査が実在の Link を拒みうること。どれも判定はスキップしない側に倒れる。Fable（`claude-fable-5-2`、2026-11-24）: Major 0・Minor 3。`git.go` の変更が文書にないこと（Opus の Minor と重なる）、Link の扱いが設計に記録されていないこと（最初の `rel` だけを読む点は RFC 8288 の規定どおりなので、安全性の主張は無効。厳格な URI 検査の件は Opus の Minor と重なる）、すべての検査を通った後に取り消されたとき理由が空（`ReasonUnspecified`）になること | なし（Critical・Major なし） |
| 1 | #1090 | `2db8551f0a10` | feat(0120) fixprguard 監視一覧 | 要 | 要 | Opus を `gemini-4-neon-high` で代替、Fable は `not run` | Opus（`claude-opus-5-5`、2026-11-24）: 0/0/4。Minor は、`fixpr.md`・`runpr.md` のプロトコル不一致の復旧手順で、ツールの番号のほうが大きいときに 2 つの分岐が同時に当てはまり、`main` の番号を読まないと選べないこと、計画書のステップ 1-1 と設計書 3.6.1 が `watchTopDirs` を `[]string` と書いたままであること、`ignored.go` の監視の一覧の doc コメントが `var` ブロックから離れたこと、復旧手順の書き換えが計画書にも `Editorial correction:` の行にも記録されていないこと。Fable（`claude-fable-5-2`、2026-11-24）: Major 1・Minor 2。Major は、Antigravity CLI が `.agents/` だけでなく `.agent/`・`_agents/`・`_agent/` からもワークスペースの設定（`hooks.json`・スキル・規則）を読むのに、`fixprguard` の監視の一覧と状態チェックが `.agents/` だけを見ているので、自分自身を無視させる `.gitignore` と一緒に置かれたフックやスキルを検出できないこと（`agy` の組み込みの説明で確認、Fable が写しで再現）。`origin/main` でも残る（監視の範囲の決定の変更なので人の判断が要る）。Minor のうち `fixpr.md` の禁止コマンドに `ls .agents` がない件は後の PR で解消済み。もう 1 件は ADR-0055 の食い違いの記述に、この PR より前からまだ解消されていなかった項目が抜けていたこと | なし。Fable の Major は監視の範囲の決定の変更なので #1121 で扱う |
| 1 | #1091 | `d419fb5ddce7` | ci(0119) check-changes のスキップ統合 | 要 | 要 | Opus と Fable の両方を Neon High で代替 | | |
| 3 | #1092 | `a3f3830e4a68` | docs(0121) 要件 | 不要 | 要 | Fable は `not run`（Antigravity のセッション） | | |
| 1 | #1094 | `4aee4aa65011` | feat(0120) 役割名への移行・判定スクリプト改名 | 要 | 要 | Opus を `gemini-4-neon-high` で代替、Fable は `not run` | | |
| 3 | #1098 | `7d86a397b237` | docs(0122) 要件・ADR | 不要 | 要 | Fable は `not run`（Antigravity のセッション） | | |
| 1 | #1097 | `bd72ac9e8e20` | feat(0114) push 前の Fable 記録の再確認 | 要 | 要 | Opus を Neon High で代替、Fable は `not run` | | |

**同時期の未マージ PR:** 0115 の計画、0121 の設計書、0120 PR-3 の各ブランチにも、同じ記録が残っています。これらは事後の再レビューではなく、マージ前に正規のレビューを通してください（手順 5）。

#### B. OpenCode での実装による構造上の `not run`（対象外）

実装モデル要件が `sonnet` で OpenCode が実装した PR です。OpenCode は Fable を呼べないので `not run agent-cannot-call` になります。これは設計どおりの動作で、クオータ枯渇による代替ではありません。2026-11-24 に人が決めたとおり、この手順書の対象は A だけで、B は再レビューしません。

| PR | squash | 種別 | 記録 |
|---|---|---|---|
| #1048 | `00e21e18ba21` | docs(0112) dev_tools 規約 | Fable `not run`（OpenCode） |
| #1088 | `d7467569603e` | docs(0118) Codex 記述の更新 | Fable `not run`（OpenCode） |
| #1089 | `812f94a0dc3e` | feat(0119) cmd/ciskip | Fable `not run`（OpenCode）。Opus は `NONE value-sonnet` |
| #1093 | `063dd6d55e10` | docs(0119) 実機確認の記録・ADR | Fable `not run`（OpenCode） |

#### C. 候補になったが対象外

- **#1066:** `NO-HIGH-REVIEW-RECORD` が出ました。ただし、すべてのコミットが `claude-opus-5-5` の実装記録の範囲に入っていて Opus レビューは不要（`NONE all-opus`）であり、Fable も `claude-fable-5-2` で実行されています。

**どちらの再レビューが要るか（判断の基準）:**

- **タスク文書 PR**（`docs(NNNN)`、`/mkreq`・`/mkarch`・`/mkplan` が作ったもの）: Fable だけ。Opus レビューは `/runplan` の PR にしかありません。
- **実装 PR**（`/runplan`）: Fable と、要件しだいで Opus。
- **単独 PR**（タスク番号のない scope）: どちらの手順も通らないので対象外。

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
