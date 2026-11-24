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
- `fetch.sh` が取る PR は 300 本まで（`gh pr list --limit 300`）、PR ごとのコミットは 250 個まで（GitHub の API の上限）です。300 本に達したときは `fetch.sh` が止まるので、期間を分けて取り直してください。250 個を超える PR は、251 個目以降の記録を読めません。
- 結果は最後に人が読んで確定してください。

### 確定した一覧

2026-11-24 に `fetch.sh 2026-11-20` を実行して作りました。対象は 2026-11-20 以降に `main` へマージされた 34 本で、`classify.sh` が候補とした 25 本を人の目で読んで分けています。

- **最初の代替:** #1070（2026-11-20 マージ）。それより前の 2026-11-20 の文書 PR（#1049・#1050・#1056・#1058・#1060・#1062・#1063・#1065）は、Fable が `claude-fable-5-2` で実行されています。
- **行の順:** マージの順。**優先度:** 1 は信頼境界・セキュリティ・レビューゲート・CI ゲートを変えた PR、2 はそれ以外の実装 PR、3 はタスク文書 PR。
- **「代替の根拠」の欄:** PR ブランチの記録行と PR 本文から読んだものです。

#### A. 代替でレビューされた PR（クオータ枯渇による。再レビューの対象）

| 優先度 | PR | squash | 種別 | Opus 再レビュー | Fable 再レビュー | 代替の根拠 | 結果 | 修正 PR |
|---|---|---|---|---|---|---|---|---|
| 2 | #1069 | `7bf8f68b08f0` | docs(0117) chief_architecture.md | 要 | 要 | Opus レビューを Gemini のセッションでの Codex CLI レビューで「完了扱い」にした。Fable は `not run`。実装は OpenCode | Opus（`claude-opus-5-5`、2026-11-24）: 0/2/9。Major の 1 件目は、設計書 2.3 節・3.5 節と計画書（ステップ 1-2 の完了の条件、4.1 節、7 章の AC の行）が新しい文書を冒頭と 9 つの章としているのに、マージされた `chief_architecture.md` は第 1 部・第 2 部の 6 章で、設計書にない 1.3・1.4・3.1〜3.2 を持つこと。組み替えは PR を作った後のコミットで入り、決定の変更としても記録されず、代替のレビューの記録もその前で終わっている。2 件目は、squash コミットの `Editorial correction:` の行が ADR-0046 のリンク先を古い章番号（7 章・6 章・2 章）で書き、マージされた ADR（3.5・3 章・1〜2 章）と食い違うこと。どちらも `origin/main` でも残る。Minor は、計画書の「新しいヘルパーは足さない」に反する `outsideBlocks`、テストのコメントが最終の見出しより前の壊し方の記録を指すこと、`RowOutput.task` の空の値（`-` ではなく空文字列）、6.7 の標準エラーの行（子の標準エラーは標準出力の `message` に入る）、chief_guide 2 章の「引数の形が合わなければ番号の照合より前に終了コード `2`」が 5.3 と食い違うこと、`StopOutput.at` が空ではなくゼロ時刻になること、`removed_sends` の数え方と終了コード `3` の一覧の漏れ、package_reference と ADR が要約だけの 3.5 を仕様の置き場所として指すこと、`check-ac22.sh` の日本語の出力と `git diff` の失敗で通る確かめ。Fable（`claude-fable-5-2`、2026-11-24）: Major 2・Minor 12（報告の Major 1 件を Minor に下げた）。Major の 1 件目は、ADR-0046:179 の新しい文書が持つ話題の一覧に「テストのしかた」が足されたこと。設計書はリンクのほかは変えないと定め、同じ一覧への追加を決定 10 では決定の変更として経緯に記録したのに、編集上の訂正として記録された。2 件目は、3.5 の表が「次のコマンドの提案」を副作用の条件に「あたらない（人が選べば、送信の条件に当たる）」としたこと。マージの後の自動送信はこの判定のコマンドを人の選択なしに打つので、送るコマンドを変える PR が番号を上げずに済むと読まれうる。要件の基準は「あたる」「あたらない」だけで、決めた記録がない。どちらも `origin/main` でも残る。Minor は、送信ごとに `/clear`・`/model`（OpenCode は `/new`）を打つことが図 1-2 と 6.7 にないこと（5.4.16 の `step` の値と 1.4 にはある）、4.4 に要件の条件 (3) の補足の文がないこと、決定 14 が例と図にしかないこと、5.4 の冒頭の規則に設計書 3.2 節の 2 つの決まりがないこと、ADR:181 がない「プロトコル番号の章」を指すこと、ADR:69 が書き方を持たない 3.5 を指すこと、定義の終了コード `2` の行が意味の欄も変え、道具が古いときの直し方しか示さないこと、6.7 が自動送信と `create` の `/mkreq` を挙げないこと、1.3 が `targets` を挙げないこと、3.2 が `origin` を文字列と書くこと（実際はオブジェクト）、図 1-1 に Legend がないこと、3.5 が手順の細部を繰り返すこと | #1129（Opus の Major 2 件と Fable の Major 1 件目の修正。0117 の設計書に決定 17・18 として記録して再承認し、ADR-0046 の経緯に 1 行を足した。マージ済み、2026-11-24）。Fable の Major 2 件目は副作用の条件の解釈の決定なので #1127 で扱う |
| 3 | #1070 | `dc224b5c7754` | docs(0118) 設計書 | 不要 | 要 | Fable を Gemini 4 Neon のフルレビューで代替 | Fable（`claude-fable-5-2`、2026-11-24）: Major 2・Minor 11。Major の 1 件目は、PR を開く前の追加のローカルの CLI レビュー（F-007）を 0114 の Entry 4 より前に置いたので、Entry 4 が救済を走らせると、救済の Opus レビューの直しと CHECKS の直しがローカルの CLI レビューを受けずに PR に入ること（AC-29・ADR-0042 と食い違う）。#1097 の Fable の指摘と同じで、#1125 の参考にある。`origin/main`（`_lib/task-pr-handoff.md` Part B step 2.6）でも残る。2 件目は、最新の依頼の応答を「依頼の後に始まった最初の数える CheckRun」とし、始まった時刻に上限がないので、CheckRun の付かないまま時間切れになった依頼に、後で Web などから始まって `Finished` で終わった CheckRun が応答として数えられ、完了の印になること（要件 3.1・AC-54・設計書 5.1 の脅威の行と食い違う）。`origin/main` の `review_state.go` の `decide` でも残る（判定方式の決定なので人の判断が要る）。Minor は、AC-16 の (b)〜(d) で投稿の URL を示さないこと、手で書いた依頼や前の実行の依頼が待つ間に終わった場合を扱わないこと、`ReviewURL` が応答の後の PR レビューのオブジェクトを 1 つと仮定すること（スレッドへの返信もオブジェクトになる）、`details_url` を外す根拠に観測の記録にない記述を挙げること、編集された依頼の判定が観測していない `updated_at` に頼ること、ADR-0013 の追加が編集された依頼の規則を欠き 15 分を決定に書くこと、ADR-0042 の追加で CHECKS の直しを Opus の再判定から外したこと、要約の行数が ADR-0047 の 14 行と食い違うこと、AC-43・AC-44 を PR-3 に移さない理由が成り立たないこと、`<mention-check>` の展開を実行時に確かめられないこと、CodeRabbit が PR の説明に書く要約が 5.1 の表にないこと、merge commit があると追加のレビューがいつも走ること（AC-31） | なし（Major の 1 件目は #1125 で扱う。2 件目は判定方式の決定なので #1131 で扱う） |
| 3 | #1074 | `6a266e990548` | docs(0118) 計画書 | 不要 | 要 | Fable を `gemini-4-argon-medium`（1 回目は Argon 以外）で代替 | Fable（`claude-fable-5-2`、2026-11-24）: Major 2・Minor 6。Major の 1 件目は、フェーズ 1 のステップの順では、ステップ 1-1 で `_lib/local-cli-review.md` から `codexreview` の呼び出しを消した時点で `cmd/codexreview` と `cmd/mergepr` のガードテストが落ち、ステップ 1-3・1-4 の `coverage-compare` が STOP になること（設計書 6.3 が同じ PR にまとめた理由を commit の単位で崩す）。2 件目は、新しい Part B の手順の番号 `2.5` を、0114 の承認済みの設計が同じ番号を使うのに人に尋ねずに決め、その理由が成り立たないこと。どちらも計画の実行の問題で、`origin/main` では当てはまらない（#1077 は 3 つのテスト削除の commit で `coverage-compare` が通ったと記録し、Part B は step 2.5 と 2.6 に分かれた）。Minor は、AC-35・AC-26 の検証が AC に反しても落ちないこと、AC-14・AC-16・AC-18・AC-39・AC-28 の文書の側に検証がないこと、AC-50（標本が残る）に `diff` の検証がないこと、自動レビュアーの一覧の変更を 1 commit にする `fixprguard_guide.md` の決まりと食い違い、間のステップで文書のガードテストがコンパイルできないこと、ステップ 2-2 が `go build ./cmd/fixprguard` を指示すること、ステップ 3-3 の issue の作成に承認とメンションの確かめの指示がないこと | なし（有効な Critical・Major で `origin/main` に残るものはない） |
| 3 | #1075 | `b004f8c01ba2` | docs(0119) 要件・ADR-0054 | 不要 | 要 | Fable を `gemini-4-argon-medium` で代替 | Fable（`claude-fable-5-2`、2026-11-24）: Major 2・Minor 6。Major の 1 件目は、F-002 が親の実行で決まった 4 つのジョブだけを確かめ、ほかのジョブの結論を無視するので、後で足した重いジョブを一覧に足し忘れると、親でそのジョブが失敗していても文書だけの commit がスキップされ、マージできること（一覧を保つのは決まりだけで、AC もガードもない）。`origin/main` の `internal/ciskip/github.go` の `verifyParentJobs` でも残る（今の `ci.yml` では成り立たない潜在的な危険。判定方式の決定なので人の判断が要る）。2 件目は、リダイレクトの拒否と応答の大きさの上限を、人の答えなしに要件として決めたこと。#1076 で要件を draft に戻し、`gh api` を採ってこの 2 点を制限として受け入れたので、`origin/main` では解消済み。Minor は、親の実行が確かめた `main` の状態を API で読めるかを確かめていないこと、取り消しの後の再実行で `skipped` が正当なスキップと見分けられないこと、重複キーの検査の範囲が `internal/strictjson` と食い違うこと（#1076 で解消済み）、トークンを記録しないことと PR 由来の文字列をシェルで展開しないことに AC がないこと、申し送りのリネームの扱いがどの分類を指すか分からないこと、AC-17 を確かめられないこと | なし。Major の 1 件目は判定方式の決定なので #1133 で扱う |
| 3 | #1076 | `d45909cf9d36` | docs(0119) 設計書 | 不要 | 要 | Fable を `gemini-4-argon-medium` で代替、残りは `not run` | Fable（`claude-fable-5-2`、2026-11-24）: Major 0・Minor 9。`Check changed files` ジョブの `normal` の step が変更のパスを `echo` するので、PR のファイル名で偽の `ci-base-sha` のアノテーションを足しうること（本物と重なると `parent_base_duplicate` でスキップしないだけで、誤ったスキップの道は見つからなかった）、同じリポジトリの書き手を脅威の外に置く前提が人の答えのない安全の前提で ADR-0054 にもないこと（#1124 の背後の前提）、要件が「読み込みを打ち切る」と書くが `cmdrunner` は捨てながら読み続けること、重複キーの検査を使う項目に狭めた変更に人の答えがないこと、ADR-0054 の編集上の訂正の行がないこと、用語集の 3 つの語の言い換えに記録がなく古い語が今の決定に残ること、ADR-0054 が知らせを「起動するたびに」書くと設計より広く書くこと、`fixprguard`・`chief` も同じ `gh api --hostname github.com -i` を使うという記述が事実と違うこと、`gh` がリダイレクトで `Authorization` を外すという主張と、Jobs API の `id` を `check_run_id` に使えるという前提を確かめていないこと | なし（Critical・Major なし） |
| 1 | #1077 | `21400a98ad5b` | refactor(0118) codexreview 撤去 | 不要（`claude-opus-5-5-high` で実行） | 要 | Fable を `gemini-4-argon-medium` で代替（記録は `not-fable`） | Fable（`claude-fable-5-2`、2026-11-24）: Major 1・Minor 2。Major は有効で `origin/main` でも残る（`settings.json` の allow 規則と `defaultMode` を見るガードが、`cmd/codexreview` の削除で失われた）。Minor 2 件のうち、1 件（計画書の editorial correction の扱い）は記録上の問題でコードに害はなく、もう 1 件（ADR-0013 の食い違いの記述）は後の PR で解消済み | #1103（マージ済み、2026-11-24） |
| 3 | #1078 | `3fa62267b0c8` | docs(0119) 計画書 | 不要 | 要 | Fable を `gemini-4-argon-high` で代替（記録は `not run`） | Fable（`claude-fable-5-2`、2026-11-24）: Major 1・Minor 7。Major は、ADR-0054 と ADR-0014 の食い違いの記述を消すのを PR-5 だけに置き、食い違いを解消する PR-3・PR-4 では消さないこと（F-005・AC-19 と adr.md の「Discrepancy statements」は同じ PR で消すと定める）。PR-4 のマージから PR-5 のマージまで、ADR が実装済みの事柄を未実装と述べる。#1093（PR-5）で 2 つの記述は消え、`origin/main` では当てはまらない。Minor は、実動作の確認（AC-20）を PR-4 のマージの後に置くので、本物の API での最初のスキップが PR-4 自身のマージを通し、違いが見つかったときにスキップを止める手順がないこと、5 つの PR の構成と PR-3 の `sonnet` に人の答えの記録がないこと、PR-3 の `sonnet` の理由が安全の関門の最後の部分（`skip=true` と `ci-base-sha` の行を書く）に当たらないこと、`New(environ)` がランナーごとに環境を分けることを確かめるテストがないこと、CLAUDE.md の例外を API の部分のコードより前の PR-1 で書くこと、調査の行番号と件数が #1077 の後の `main` とずれていること、`setup-go` が入れる Go の版と ubuntu-latest の `gh` の版を確かめていないこと | なし（有効な Critical・Major で `origin/main` に残るものはない） |
| 3 | #1079 | `8fbbfc4b8b77` | docs(0120) 要件 | 不要 | 要 | Fable を `gemini-4-argon-medium` で代替、残りは `not run` | Fable（`claude-fable-5-2`、2026-11-25）: Major 2・Minor 12。Major の 1 件目は、AC-21・AC-27・§4.2 が Antigravity のワーカーのモデルを `swarm` の `add` の直後に 1 回だけ確かめ、結果を集めるときに確かめ直さないので、`agy` の組み込みの案内どおりにワーカーが `action:"model"` で別のモデルに切り替えても、`high` のレビューとして記録されること。2 件目は、要件と ADR-0055 が Antigravity のサンドボックスを既定とし、禁じたコマンドを `BypassSandbox: true` の承認で守るとするが、`agy --help` では `--sandbox` は有効にするためのフラグで、Auto モードや保存した前置きの規則でも承認なしに走りうること（既定はオフラインでは確かめられない）。どちらも `origin/main` の `_lib/harness-antigravity.md`（step 6 と 40 行目）でも残る（判定方式とレビューの関門の前提の決定なので人の判断が要る）。Minor は、`.agents/hooks.json` の `PreToolUse` で `deny` できるので「機械的に拒めない」の前提が成り立たないこと、`.agents/` を親のディレクトリへたどって読むのに最上位だけを監視すること、`swarm` の呼び出しをツール自身が利用者かスキルの明示に限ること、`quota_remaining` だけで使えるかを決め AI クレジットを見ないこと、上位モデルのレビューの 30 分の上限に人の答えがないこと、`bash <script>` の昇格で書き換えられた台本が走りうること、セッションのモデルをフックの `modelName` で読める可能性、ADR-0050 の決定 5 と経緯・理由の差し替え・訂正の行の漏れ、ADR-0042 の「どちらかが走らせると判断すれば走らせる」の脱落と採らなかった案の取り下げの記録漏れと結果の項目の削除、ADR-0046 の訂正の行の漏れ | なし。Major 2 件はどちらも判定方式とレビューの関門の前提の決定なので #1135 で扱う |
| 1 | #1080 | `4e8fd02e21e0` | feat(0118) fixprguard review-state | 要 | 要 | Opus を `gemini-4-neon-high`、Fable を `gemini-4-argon-high` で代替 | Opus（`claude-opus-5-5`、2026-11-24）: 0/1/9。Major は `review_state.go` の読み取りの上限（GraphQL のページ数、1 ページの件数、CheckSuite と CheckRun の数、`gh api user` の大きさ）と期限切れの扱いにテストがなく、壊しても通る。`origin/main` でも残る。Fable（`claude-fable-5-2`、2026-11-24）: Major 1・Minor 7。Major は、設計書 3.3.1 で厳密な検査に `null` を許す例外（(b)・(c)）を足したのに、編集上の訂正（決定の変更なし）として記録した。動作は安全側だが決定の変更に当たり、1.4 に記載がない。`origin/main` でも残る（決定の変更なので人の判断が要る）。Minor のうち `--request` の前後の空白、CheckRun 名の確認がクエリの文面だけであること、`unreadable` の `now` が空であることの 3 件は、Opus の Minor と重なる | #1110（Opus の Major の修正、マージ済み、2026-11-24）。Fable の Major は決定の変更なので #1109 で扱う |
| 1 | #1081 | `fc4bb3dfbb5a` | feat(0119) internal/ciskip | 要 | 要 | Opus を Neon High、Fable を `gemini-4-argon-high` と `claude-opus-5-5-medium` で代替。実装も Neon High | Opus（`claude-opus-5-5`、2026-11-24）: 0/1/9。Major は `TestValidateDepDirsFile` の 64 KiB+1 の行が、1 行だけで `/` で終わる入力なので、読み取りの上限（`LimitReader` の `+1`）を壊しても末尾 `/` の検査で拒否されて通る。行の境目で切れた依存一覧を受け入れる誤りを検出できない。`origin/main` でも残る。Fable（`claude-fable-5-2`、2026-11-24）: Major 1・Minor 3。Major は、親から head への差分だけで非コードと判定するため、`main` がファイルを非コードのパスからコードのパスへ移した後に head が旧パスを編集すると、合成マージコミットではコードが変わるのにスキップされる（作業用リポジトリで再現）。設計書 §3.3.2・§5 が扱っていない場合で、`origin/main` でも残る（判定方式の変更なので人の判断が要る）。Minor のうち submodule の `ignore = all` で差分から抜ける件は Opus の Minor と重なる。CLAUDE.md の例外が未実装の `gh` の読み取りを含む件と、ADR-0054・ADR-0014 の「未作成」の記述の件は、後の PR で解消済み | #1113（Opus の Major の修正、マージ済み、2026-11-24）。Fable の Major は判定方式の変更なので #1112 で扱う |
| 3 | #1083 | `b93a22023ef7` | docs(0120) 設計書 | 不要 | 要 | Fable は `not run`（Antigravity のセッション） | Fable（`claude-fable-5-2`、2026-11-25）: Major 4・Minor 13。Major の 1 件目は、Antigravity のワーカーに渡すレビューの入力（差分・ログ・ファイルの一覧）を scratchpad のファイルにし、サンドボックスのワーカーが書けるのにハッシュを控えて確かめ直さないので、差分の文に誘導されたワーカーが入力を書き換えると、誰も見ていない差分に `High review: done` が記録されうること。2 件目は、`agy` の組み込みの案内がコーディネーターに割り当て切れのワーカーを `action:"model"` で切り替えるよう指示し、設計がそれをワーカーにしか禁じないこと（#1135 の指摘 1 の新しいきっかけ）。3 件目は、Antigravity のクロスチェックのモデルの選定理由が、600 秒の打ち切り（関門の上限は 1,800 秒、Fable も平均 695 秒）、1 回だけの試行、週の割り当ての 6% という文書自身の数字で成り立たないこと。4 件目は、OpenCode も `.agents/skills/` を読むので、11 本の入口が DeepSeek のセッションのスキルになり、ADR-0055・F-001 の範囲と食い違うこと。4 件とも `origin/main` でも残る（判定方式・関門の前提・範囲の決定なので人の判断が要る）。Minor は、Antigravity のセッションが実装の記録を書かないので Opus の記録の範囲に覆われて `NONE all-high` になりうること、`.agents` の下の `.git` の扱いが 2 案のままで人の答えのない受け入れた危険があること、検出の表の行が `ask_question` のない `self` のサブエージェントや `AskUserQuestion` のない Claude Code の文脈で誤りうること、`high` などの Antigravity の割り当てが実測されていないこと、順序つきの代替の不採用の理由を書き換えたこと、ADR-0050 の既存の経緯の行の書き換えと決定への手順の細部の記載と存在しない節の名前の引用、要件と ADR にモデル ID を直接書いたこと、`/runpr` の要約の文字列を決めていないこと、`low` の意味が対応表と送り先の表で分かれること、`standard`・`low` の固定モデルが走らないときにセッションのモデルで代える決まりに人の答えがないこと、`subagent-role` にモデルの確かめがないこと、補助のワーカーの持ち主の判定が確かめられていないこと | なし。Major の 1〜3 件目は Antigravity のアダプタの決定なので #1135 にコメントで足し、4 件目は #1138 で扱う |
| 1 | #1084 | `720fc313c595` | feat(0118) 手動 CodeRabbit・メンション拒否 | 要 | 要 | Opus が `stopped-unavailable` で Neon High に代替、Fable は `not run` | Opus（`claude-opus-5-5`、2026-11-24）: 0/2/5。Major の 1 件目は、CLAUDE.md の「`make test` に `jq` が要る」の項目が、この PR で削除した `TestHandoffListingPrintsNoBody` と、なくなった `_lib/task-pr-handoff.md` Part B step 5 を挙げたままだったこと。`origin/main` でも残っていた。2 件目は、削除したテストの coverage-compare の出力がコミットのメッセージでは要約だけだったこと。ただし PR の説明に、最後の行（`coverage-compare: PASS`）までの全文があるので、記録は残っている。Minor は、`runpr.md` step 4 の状態の変化で `none` に変わったときの扱いがないこと、`AppendPR` で大きさの検査がメンションの検査より先に走ること、`current-body-mention` の判定が検査の順序に頼っていること、計画書の `TestPostingPathsCheckMention` の説明が実際の検査より強いこと、テスト名 `TestPostTextMentionExitsUsage` が内容と合わないこと。Fable（`claude-fable-5-2`、2026-11-24）: Major 0・Minor 6。`<mention-check>` の `grep` が NUL や不正な UTF-8 を含むファイルを見逃しうること、`pr-review-escalation.md` の判定基準 4 のゲートの一覧が設計書より変わったこと、`weakreview.md` が「最初の PR レビュー」を前提にしていること、`fixprguard_guide.md` の 467 行目の CodeRabbit の解釈に関する主張が確かめられていないこと、範囲から外した 2 ファイルの判断の誤り。`fixprguard_guide.md` 10.4 の件は後の PR で解消済み | #1116（Opus の Major 1 件目の修正、マージ済み、2026-11-24） |
| 3 | #1085 | `3562af5a17b3` | docs(0120) 計画書 | 不要 | 要 | Fable を `gemini-4-neon-high` で代替（記録は `not run unavailable`） | Fable（`claude-fable-5-2`、2026-11-25）: Major 2・Minor 6。Major の 1 件目は、設計書 8 章の移行の順序の書き換え（CLAUDE.md などの更新と AC-02 の確かめを、手順の名前を変える PR-4 から PR-5 へ移した）が決定の変更なのに、状態を `approved` のまま編集上の訂正として記録したこと。`origin/main` の設計書の変更履歴は #1083 の承認だけで残るが、人の答えは計画書 1.2 の「利用者に確認して決めたこと」にあり、タスクも終わっているので、記録だけにした。2 件目は、AC-02 の確かめのスクリプトを、変更を入れる PR-4 ではなく PR-5 で作って走らせ、種類も `static` と書いたこと。0120 の PR はすべてマージ済みで、`origin/main` の `.claude/commands`・`CLAUDE.md`・`AGENTS.md` に旧い手順の名前は残っておらず、当てはまらない。Minor は、名前を変えた手順への参照の確かめがないこと、並行するブランチへの `git merge origin/main` の指示に行う者がないこと、`.agents/skills` の入口に `disable-model-invocation: true` がなく Antigravity がスキルを自動で呼びうること、`fable` モードのテストを 3 件に減らしたこと、PR-2 から PR-4 まで `runplan.md` に古い理由のコードが残ること、PR-1 の要件の値と AC-41 の 4 項目目の食い違い | なし（Major の 1 件目は人の答えが計画書にあり、タスクも終わっているので記録だけにした。2 件目は `origin/main` で当てはまらない） |
| 1 | #1086 | `cd4a90aef0ae` | feat(0119) ciskip GitHub API 検証 | 要 | 要 | Opus を Neon High で代替（HTTP 429）、Fable は `not run` | Opus（`claude-opus-5-5`、2026-11-24）: 0/0/9。Minor は、`git.go` の `validateDepDirsFile` を `O_NONBLOCK` で開いて通常ファイル以外を拒否するよう変えたのに、計画書（ステップ 1-2 など）と設計書が `os.Open` のままであること、要件の「閉じた列挙値」に対し `status`・`conclusion` を非空文字列として受けること、`ErrWaitDelay` を常に `api_timeout` にすること、通常ファイルの検査・`New` の配線・`headRepoNull`・呼び出し前の期限の検査を壊しても通るテストがあること、`prRunMatch` の `switch` に安全側の `default` がないこと、設計にない RFC 3986 の URI 検査が実在の Link を拒みうること。どれも判定はスキップしない側に倒れる。Fable（`claude-fable-5-2`、2026-11-24）: Major 0・Minor 3。`git.go` の変更が文書にないこと（Opus の Minor と重なる）、Link の扱いが設計に記録されていないこと（最初の `rel` だけを読む点は RFC 8288 の規定どおりなので、安全性の主張は無効。厳格な URI 検査の件は Opus の Minor と重なる）、すべての検査を通った後に取り消されたとき理由が空（`ReasonUnspecified`）になること | なし（Critical・Major なし） |
| 1 | #1090 | `2db8551f0a10` | feat(0120) fixprguard 監視一覧 | 要 | 要 | Opus を `gemini-4-neon-high` で代替、Fable は `not run` | Opus（`claude-opus-5-5`、2026-11-24）: 0/0/4。Minor は、`fixpr.md`・`runpr.md` のプロトコル不一致の復旧手順で、ツールの番号のほうが大きいときに 2 つの分岐が同時に当てはまり、`main` の番号を読まないと選べないこと、計画書のステップ 1-1 と設計書 3.6.1 が `watchTopDirs` を `[]string` と書いたままであること、`ignored.go` の監視の一覧の doc コメントが `var` ブロックから離れたこと、復旧手順の書き換えが計画書にも `Editorial correction:` の行にも記録されていないこと。Fable（`claude-fable-5-2`、2026-11-24）: Major 1・Minor 2。Major は、Antigravity CLI が `.agents/` だけでなく `.agent/`・`_agents/`・`_agent/` からもワークスペースの設定（`hooks.json`・スキル・規則）を読むのに、`fixprguard` の監視の一覧と状態チェックが `.agents/` だけを見ているので、自分自身を無視させる `.gitignore` と一緒に置かれたフックやスキルを検出できないこと（`agy` の組み込みの説明で確認、Fable が写しで再現）。`origin/main` でも残る（監視の範囲の決定の変更なので人の判断が要る）。Minor のうち `fixpr.md` の禁止コマンドに `ls .agents` がない件は後の PR で解消済み。もう 1 件は ADR-0055 の食い違いの記述に、この PR より前からまだ解消されていなかった項目が抜けていたこと | なし。Fable の Major は監視の範囲の決定の変更なので #1121 で扱う |
| 1 | #1091 | `d419fb5ddce7` | ci(0119) check-changes のスキップ統合 | 要 | 要 | Opus と Fable の両方を Neon High で代替 | Opus（`claude-opus-5-5`、2026-11-24）: 0/0/3。Minor は、設計書 §3.3.4・コミットメッセージ・実行テストが GitHub Actions の既定のシェルを `set -eo pipefail` としていること（`shell:` のない `run:` は `bash -e` で動く。今の `check` の本文はどちらでも同じ動作）、`go run` は ciskip の終了コードによらず 1 で終わるので、フォールバックのログの `status=` に ciskip の終了コード 2 が出ないこと、`id: normal` で `has-extension-changes.sh` が失敗すると `check-changes` が失敗してフォールバックしないこと（PR より前からある行で、設計書が通常分類を変えないと決めている）。Fable（`claude-fable-5-2`、2026-11-24）: Major 1・Minor 2。Major は、スキップの判定を、判定の対象のコミットに含まれる ciskip 自身（`go run ./cmd/ciskip`）が行うので、ciskip を変えるコミットに「スキップしてはいけないのにスキップと答える」誤りが入ると、その誤りを検出する `test` と `lint` がスキップされ、`check-changes` が成功したままマージできること。設計書 §5.3 は意図的な改ざんしか扱っていない。コードから組み立てた筋道で、GitHub 上では再現していない。`origin/main` でも残る（判定方式の変更なので人の判断が要る）。Minor は、`actions/checkout` が `persist-credentials: false` なしで使われているので、権限を広げたジョブのトークンが git の設定に残り、`id: normal` からも読めること（設計書 §5.2 と食い違う。読み取りの権限だけ）と、`workflow_pin_test.go` が行ごとに一部のキーしか読まないので、`check` への `if: false`・`continue-on-error: true`・`shell: sh`、ジョブ単位の `env:` でのトークンの付与などを足してもピン留めのテストが通ること | なし。Fable の Major は判定方式の変更なので #1124 で扱う |
| 3 | #1092 | `a3f3830e4a68` | docs(0121) 要件 | 不要 | 要 | Fable は `not run`（Antigravity のセッション） | Fable（`claude-fable-5-2`、2026-11-25）: Major 2・Minor 9。Major の 1 件目は、要件が「3 形式を撤去し、今の CodeRabbit CLI の形だけを受け入れる」と書く一方で `NOT_RUN_RE` の `\(.*\)` を変えないと決めたので、形式 3 の `not run ... (coderabbit: <kind>; codex: <kind>)` が撤去の後も読まれること（目的と決定の食い違い）。`origin/main` の `review-records.sh` でも残るが、レビュー済みの範囲を広げる `(codex)` の済みの行は撤去され、誤って通す道ではないので、記録だけにした。2 件目は、AC-01・AC-02 のマージ前の確かめを行う者と手順がないこと。後の設計と計画が `_checks/check_open_branches.sh` とマージ前のゲートを定め、実装の PR #1130 もマージ済みなので、当てはまらない。Minor は、範囲の外に残る古い Codex の行が PR の説明から黙って消えることを受け入れた危険として書いていないこと、merge check の記録の commit の `(codex)` の行の AC がないこと、AC-10 と H-01 が `unreviewedCases` の受け入れの例を挙げないこと、#1066・#1051 の後の `main` と行番号と記述が合わないこと、AC-04 が本文を書き写さない決まりを CodeRabbit の行に広げること、安全の効果を大きく書くこと、0120 の別名を撤去しないことを範囲の外に書かないこと、2 つのブランチだけが古い行を持つという主張をオフラインで確かめられないこと | なし（Major の 1 件目は誤って通す道ではないので記録だけにした。2 件目は `origin/main` で当てはまらない） |
| 1 | #1094 | `4aee4aa65011` | feat(0120) 役割名への移行・判定スクリプト改名 | 要 | 要 | Opus を `gemini-4-neon-high` で代替、Fable は `not run` | Opus（`claude-opus-5-5`、2026-11-24）: 0/1/5。Major は、`_lib/opus-review.md`「Record lines」の **Opus.** 段落が「`claude-opus-` で始まる ID が Opus で、entry 1 もこの規則を使う」と書いたままで、この PR で `Role` = `high`・`Launch` = `agent` の照合（接尾辞は `[a-z0-9.-]` の 1〜48 文字）に変えた entry 1 の規則 3 と判定スクリプトと食い違っていたこと（`[1m]` などの接尾辞の ID は `RUN` になる。安全側）。同じ節が「食い違ったらこの節が正しく、スクリプトを直す」と定めている。`origin/main` では `_lib/high-review.md` の「The high role model」の規則に置き換わり、解消済み。Minor は、`runplan.md` の `NONE all-high` の近くに `sonnet` と「Opus implementation records」が残ったこと（`origin/main` では解消済み）、protocol 6 の chief が残した `/runplan` のスキップ記録が旧行キーのため読めなくなることが、入れ直しの手順と手引きにないこと、`_context.md` の「Harness model mapping」が、まだない読み手（`review-records.sh` の検証、アダプタ、`runplan.md`・`runpr.md` の Antigravity 行の読み取り）を現行の仕様として書いたこと、判定スクリプトの冒頭のコメントが BASH_ENV と export された関数を防げると書くが実際は防げず（`review-records.sh` にある受け入れたリスクの注記が落ちた。`origin/main` でも残る）、`${!GIT_@}` の unset が `env -i` の後で効かないこと、計画書のステップ 2-2 の `TestParsePlanRejects` の拡張が行われていないのに `[x]` になっていること。Fable（`claude-fable-5-2`、2026-11-24）: Major 0・Minor 4。Antigravity の `cross-check` の ID `claude-opus-5-5-medium` が Claude Code の `high` の照合 `claude-opus-*` に合うので、その ID の実装記録で `NONE all-high` になり、「Antigravity の記録は `all-high` を満たせない」という設計書 3.4 と part 5 の記述が成り立たないこと（作業用リポジトリで再現）、`excludable` の `git diff-tree` に `--ignore-submodules=none` がなく `GIT_GRAFT_FILE` も固定しないので、作業ツリーの `.gitmodules` の `ignore = all` で gitlink だけのコミットが除外可能になり判定が変わること（再現。このリポジトリに submodule はない）、新しい役割名 `standard` と旧値の行 `standard` → `low` が衝突し、`standard` と書いた計画書ではレビューが走らず OpenCode に送られうること、ADR-0014:33 に「タスク 0120 で判断の規則全体の常設テストを足した」と書いた行が、規約の 4 種類のどれにも当たらないのに編集上の訂正として記録されたこと。4 件とも `origin/main` でも残る | なし（有効な Critical・Major で `origin/main` に残るものはない） |
| 3 | #1098 | `7d86a397b237` | docs(0122) 要件・ADR | 不要 | 要 | Fable は `not run`（Antigravity のセッション） | Fable（`claude-fable-5-2`、2026-11-25）: Major 2・Minor 13。Major の 1 件目は、AC-18・AC-22 が `LAST_REVIEWED` を CodeRabbit と Sourcery AI の完了の印の和集合から決めるので、push のたびに走る Sourcery AI の自動レビューの完了で `LAST_REVIEWED` が head になり、レビューの関門を書き換える PR でもエスカレーションが飛ばされ、推奨が「マージ」になり、ローカルの CLI レビューの `not run` の範囲とクロスチェックの記録を示す要約の行も出なくなること（人の答えの記録がない）。2 件目は、ADR-0013 の書き換えで、CodeRabbit の応答の選び方（次の依頼より前の最初の CheckRun、同じ秒や重なった実行で止まる）の決定が経緯なしに消え、AC-20・AC-27 と食い違うこと（実装は変わっていない）。どちらも `origin/main` でも残る（0122 は PR-1 だけがマージ済みで、PR-5 の前に決める必要がある）。Minor は、Sourcery AI の完了の印が PR に結び付かず、閉じた PR の完了が作り直した PR に効くこと、PR にリンクした issue の外部の文が信頼するスレッドとして返りうること、範囲の決まりが Sourcery AI の読む指示ファイルによること、AC-19 の CI の集合が定まらず `gh pr checks` の出力にアプリの識別子がないこと、AC-07 の `sourcery-ai[bot]` が `other-bot` ではなく `no-author-name` になること、`_context.md` の行番号が古いこと、F-005 が前提の崩れるコマンドの定義と ADR-0050・ADR-0042 を挙げないこと、ADR-0013 の採らなかった案が新しい決定と矛盾し、出典にない理由を書き、受け入れた危険を直していないこと、ADR-0047・ADR-0015 の編集上の訂正の行の漏れと内容の削除、人の答えのない決定（15 分の上限、和集合、依頼の案内のコメントの除外、外部の人への応答）、Sourcery AI の観測をオフラインで確かめられないこと | なし。Major 2 件は 0122 の PR-5 の前に決める決定なので #1140 で扱う |
| 1 | #1097 | `bd72ac9e8e20` | feat(0114) push 前の Fable 記録の再確認 | 要 | 要 | Opus を Neon High で代替、Fable は `not run` | Opus（`claude-opus-5-5`、2026-11-24）: 0/1/5。Major は、merge の後の確認が作った直しをローカルの CLI レビューにかけたとき、2 回目の試行の後の直し（HEAD2..HEAD3）と CodeRabbit CLI が失敗したときの範囲が `Local CLI review: not run` の行にしかならないこと。規則 4 はこの行の範囲を数えないので、チェックポイントの入口（印より前）では 2 回目の判定が `MISSING` になって止まり、走らせ直すと Fable が PR の全体を見直す。設計書の決定 11 はこの止まり方を「どちらの CLI もレビューできなかったとき」の稀な場合として受け入れたが、2 回目の試行で直したときは CLI がレビューに成功していても止まる。さらに 0118 で Codex CLI がなくなったので、レビューできないのは CodeRabbit CLI の失敗（レート制限）だけで起き、稀でもない。`origin/main`（`_lib/cross-check-review.md` の規則 4 と「The merge check」の step 4・5）でも残る（決定 11・AC-30 の変更なので人の判断が要る）。Minor は、`set-upstream-to` が fetch の refspec がブランチを含まない clone で失敗し、PR ができた後に AFTER_CREATE が飛ぶこと（Fable の実験では通常の clone で成功）、merge の後の確認のモデルの確認が、1 行目だけをファイルに書くと明示していないこと（2 つのレビューが重ねて指摘）、AFTER_REMEDY が PR_TEXT の Opus の結果を `RUN` のときしか更新しないこと、計画書 §1.4（123 行目）が Part B に step 2.5 がないと書いたままであること、言及の確認の後退で `<reason>` を消すと `Merge check: not run` の行の固定の語も消えて M が確認済みにならないこと。Fable（`claude-fable-5-2`、2026-11-24）: Major 1・Minor 2。Major は、設計書 1.2.1 の 2 つ目の決定（step 2.6 の救済の後に step 2.5 を走らせ直さない）の理由「ADR-0042 の『1 回だけ』と一致し ADR を変えない」が成り立たないこと。`/runplan` で step 2.6 が救済を走らせると AFTER_REMEDY の Opus レビューが直しを作るが、B4 にローカルの CLI レビューがなく step 2.5 も走らないので、その直しは CLI が一度も見ずに push される（ADR-0042・ADR-0013 は上位モデルのレビューの直しを PR 前の CLI レビューにかけると定める）。ただし `/runplan` では step 5a の入口 3 が印を残し、その後の 1 親の commit は規則 3 に当たるので、step 2.6 で救済が走るのは step 5a の後に merge commit ができたときだけで、1 回の実行の中で起きる道は見つからなかった。実際の害は小さいので Minor 相当と判断した。`origin/main` でも残る（決定の理由の誤りなので人の判断が要る）。Minor は、merge の後の確認の範囲のコマンドが replace ref と grafts を固定しないので、別のコミットの差分から範囲を作りうること（Fable が実験で再現）、rename/rename の衝突の見出しのパスが M にも M^1 にもなく、型の違う衝突の見出しのパスが空白を含むので、手順どおりに読めないこと（実験で確認） | なし（Opus の Major は決定 11・AC-30 の変更、Fable の Major は決定の理由の誤りなので、どちらも修正 PR にしない）。Opus の Major は #1125 で扱い、Fable の指摘は #1125 の参考に載せた |

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

1 の表の「結果」に書きます。表はこの手順書のブランチにしかないので、レビューの出力はリポジトリの外（スクラッチパッドなど）に置き、手順書のブランチへ戻ってから書いて commit します。次の PR の 3.1 で `git status --short` が空になるように、書き終えたら commit してください。

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
