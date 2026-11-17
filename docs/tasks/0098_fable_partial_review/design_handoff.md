# 設計への引き継ぎ事項（0098_fable_partial_review）

要件定義の際に挙がった、設計（`02_architecture.md`）で決める事項である。`02_architecture.md` は、項目ごとに、とった方法か、当てはまらない理由を記録する。

## H-01: 共通の手順の置き場所と、ほかの手順との分担

**懸念:** Fable の部分レビューは、`_lib/task-pr-handoff.md` Part B step 1 と `runplan.md` step 7.7 の 2 か所から、ローカルの CLI レビューと並列に呼ばれる（AC-02、AC-05）。どちらも同じ手順を使う。

**なぜ重要か:** 置き場所を誤ると、範囲の選び方・記録・トリアージの規則が 2 か所に分かれる。

**候補:**
-   新しい共通の手順（たとえば `_lib/fable-review.md`）に、範囲の選び方・指示・モデルの確かめ・記録を置く。
-   まとめたトリアージ（AC-17〜AC-19）は `_lib/local-cli-review.md` の step 4〜6 に入れる。
-   `task-pr-handoff.md` Part B には、Fable を呼ぶかを決める入力を足す（`runplan.md` は LOCAL_REVIEW を `done` で渡すので、別の入力が要る）。

並列に走らせたときの待ち合わせ方も決める。たとえば、Agent ツールのバックグラウンドの実行と、CLI の実行の終わりをどう待つか。

**関連:** F-001、F-004、AC-01〜AC-05、AC-17〜AC-19。

## H-02: 範囲の基準の具体的な文言と、範囲の一覧の形

**懸念:** AC-08〜AC-11 の基準を、セッションが同じように当てはめられる文言にする必要がある。

**なぜ重要か:** 文言が曖昧だと、書き手のセッションが自分の盲点を外しうる。外した箇所を Fable に渡す形（AC-14）が決まっていないと、Fable が外した判断を確かめられない。

**候補:** 基準ごとに、当たる例と当たらない例を置く。範囲の一覧は、箇所（ファイルと行、または文書の節）・基準・理由の表にする。

**関連:** F-002、F-003、AC-08〜AC-16。

## H-03: `/runplan` での Opus のレビューとの順序と、範囲の重なり

**懸念:** `/runplan` は、PR を開く前に Opus のレビュー（`runplan.md`「Before the PR」の B2〜B5）を行う。Fable の部分レビューは、最後の phase group の step 7.7 と並列に走る（AC-05）。

**なぜ重要か:** Fable の修正の commit が Opus のレビューの範囲に入るか、Opus の修正の commit が Fable に見られるかが、順序で決まる。Opus のレビューの直しは、ローカルの CLI レビューにかけ直さない（ADR-0042）。

**候補:** Fable の部分レビューとその修正を step 7.7 の中で済ませ、Opus のレビューの範囲に入れる。指示では、Opus のレビューのチェックリスト（正しさ、弱いモデルの失敗）の役目を Fable に重ねて与えない。

**関連:** F-001、F-002、AC-04、AC-05、AC-09。

## H-04: 記録の行の形と、記録を読む側への影響

**懸念:** AC-24 と AC-25 の行の形を決める必要がある。

**なぜ重要か:** 次の手順が、既存の記録の行を読む。
-   `_lib/local-cli-review.md`「Reading the record lines」
-   `_lib/opus-review.md`「Record lines」と `_lib/opus-review-decision.sh`
-   `task-pr-handoff.md` Part B step 3

新しい行がこれらの読み方を変えてはならない（AC-28）。

**候補:**
-   `Fable review: run <START12>..<END12>`・`Fable review: not run (<reason>)`・`Fable review: <path>:<start>-<end> <handling>: <summary>` のように、既存の行と前置きの違う行にする。
-   `_lib/opus-review-decision.sh` の除外できるコミットの判断で、記録の行だけの空のコミットが除外されることを確かめる。
-   `internal/chief` がコミットメッセージを読まないことを、改めて確かめる。

**関連:** F-006、AC-24〜AC-29。

## H-05: 複数のセッションにまたがる PR と、最後の phase group の見分け方

**懸念:** `/runplan` は phase group ごとに走りうる。step 7.7 の時点では、その phase group が PR を開く前の最後のものかを知る必要がある（AC-05）。

**なぜ重要か:** 最後の phase group を OpenCode で実装し、PR のチェックポイントを Claude Code が引き継ぐ場合（`_context.md` の `runplan-takeover` の行）がある。この場合、step 7.7 と並列に走らせる時機がない。

**候補:**
-   step 4 で次の未完了の項目が PR のチェックポイントかを見て、最後の phase group を見分ける。
-   記録（AC-06）がないまま 5a に来たときは、5a で単独に走らせる。

**関連:** F-001、AC-04〜AC-07、AC-21 (d)。

## H-06: Fable で走ったことの確かめ方

**懸念:** AC-22 は、サブエージェントが報告するモデル ID で確かめることを求める。

**なぜ重要か:** 確かめ方が Opus のレビューと違う形になると、規則が 2 つになる。

**候補:**
-   Opus のレビュー（`_lib/opus-review.md`「Running on Opus」）にならい、出力の 1 行目を `Model: claude-fable-<...>` の形として、`grep -Eqx` で確かめる。
-   Fable の ID の判定の規則（前置き `claude-fable-`）を 1 か所に置く。
-   呼び出しが失敗したときに 1 回だけ呼び直すかを決める（AC-21 (b)）。

**関連:** F-005、AC-21〜AC-23。

## H-07: Escalation の判断の入力と、ローカルの CLI レビューが見たかの判定

**懸念:** AC-31 は、LAST_REVIEWED..HEAD のすべての commit をローカルの CLI レビューが見たかで、`skip` と `ask` を分ける。

**なぜ重要か:** 判定を Escalation のサブエージェントの解釈に任せると、見ていない範囲を見落としうる。

**候補:**
-   `runpr.md` が、`Local CLI review: not run` の行の範囲と LAST_REVIEWED..HEAD の重なりを決まったコマンドで求め、FINDINGS とともにサブエージェントに渡す。
-   サブエージェントは、信頼の境界・セキュリティ・耐久性に当たるか（AC-30）と、ほかの基準に当たるかだけを判断する。

**関連:** F-007、AC-30〜AC-34。

## H-08: Fable の結果を待つ時間の上限と費用の目安

**懸念:** これまでの実績では、1 回あたり 2〜16 分、出力 2k〜260k トークンだった（issue #924）。

**なぜ重要か:** 待つ時間に上限がないと、Fable が返らないときに PR を開けない。

**候補:** 上限を決め、超えたら AC-21 (b) と同じく `not run`（理由: 時間切れ）として進む。上限の値は実績から決める。

**関連:** F-005、4.1 節、4.3 節。

## H-09: 変わる文書を読む既存のテスト

**懸念:** 次のテストが、`runpr.md`・`task-pr-handoff.md`・`fixpr.md` の文面とコマンドを読む。
-   `TestRunprDocContract`
-   `TestHandoffListingPrintsNoBody`
-   `TestFixprPhase6CountFilter`
-   `cmd/fixprguard` と `cmd/codexreview` の文書の確かめ

**なぜ重要か:** 本タスクで文書を変えると、これらのテストが失敗しうる。

**候補:** 変える節を読むテストを設計の時点で洗い出す。値と構造の確かめだけを直し、文の意味を確かめるテストは足さない（`CLAUDE.md` の「Testing Strategy」、ADR-0016）。

**関連:** F-006〜F-009。
