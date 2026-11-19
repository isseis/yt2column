# 設計への引き継ぎ：Fable の部分レビューの記録の後、push の前にできた commit を、PR を開く前に確かめる

`02_architecture.md` が扱う項目。どれも、要件定義（[01_requirements.md](01_requirements.md)）とそのセルフレビューで出た、手段の選択である。

## H-01: push の直前の再判定を、手順のどこに、どの入口として置くか

**懸念:** 再判定（AC-01〜AC-09）は、`_lib/task-pr-handoff.md` Part B の step 2（CHECKS）と step 3（push）の間に置くことになる。決まっていないのは、`fable-review.md` に新しい入口（たとえば「pre-push」）を足すか、既存の入口を広げて使うかである。チェックポイントの入口（Entry 3）は、有効なら印の commit を作るが、AC-02 は有効なときに commit を足さないと決めているので、そのままは使えない。

判定のスクリプトの `<ENTRY>` は今、`launch` と `checkpoint` だけを受け付ける（`review-records.sh:509`）。`/runplan` の PR では、push の直前の判定でも、チェックポイントの印の後の commit を無効にしない規則 3 が効く必要がある（AC-10）。

**なぜ大事か:** 入口の数と、それぞれが何を作るかは、有効かの判定の規則（`fable-review.md`「The validity decision」）と、判定のスクリプトの引数に響く。

AC-05 は、push するのを判定した commit とし、判定の後にブランチの先頭が動いたら判定からやり直すことを求める。今の Part B step 3 は、題名と本文を書き、ラベルを読んでから、ブランチの先頭を `git push -u origin <current-branch>` で push する（`task-pr-handoff.md:209-230`）。

**案:** 新しい入口を足す。有効なら何も作らない。push は、判定した commit を名指しして行う（例：`git push -u origin <判定した commit>:refs/heads/<branch>`）か、push の直前にブランチの先頭が判定した commit と同じかを確かめる。判定が merge の後の確認だけを求めるときは、それを走らせる（AC-24、H-05）。Fable の部分レビューを求めるときは、単独の経路（`fable-review.md`「The standalone path」）を走らせる。どちらも、その後にREADME の同期と CHECKS を走らせ直してから（AC-04）、もう一度判定する（AC-05）。`/runplan` では、その間に Opus のレビューの判断をやり直す（AC-09）。`<ENTRY>` は、`/runplan` の PR では `checkpoint`、タスク文書の PR では H-02 の手段に合う値を渡す。

**関連:** F-001、AC-01〜AC-09、AC-24

## H-02: タスク文書の PR の CHECKS の直しを、記録を無効にしない commit と見分ける方法

**懸念:** `/runplan` では、チェックポイントの印の後の commit を、判定のスクリプトの規則 3 が無効にしない（`review-records.sh:104-108`）。要件定義では、タスク文書の PR でも Part B step 2 の前に印を残し、印の後の commit（merge commit を除く）を無効にしないと決めた（要件定義の際に決めたこと「CHECKS の直しの見分け方」）。残るのは、規則 3 の広げ方である。セッションが「記録の後に commit がないこと」を確かめてから印を作る形は、ADR-0050 が採らなかった「セッションが手順の文に従って判定する」に当たるので、採れない。

**なぜ大事か:** 見分け方を誤ると、CHECKS の直しでない中身のある commit まで無効にしない側に入り（AC-12）、Fable を飛ばして PR が開きうる。判定の関門で誤って有効と判定する側の誤りである。

**案:** タスク文書の PR でも、Part B step 2 の前に有効かを判定し、有効なら印の commit を作る（チェックポイントの入口と同じ形）。規則 3 を、タスク文書の PR でも効くように広げる（`<ENTRY>` の値で選ぶか、印があれば効くようにするか）。印の後の merge commit は、merge の後の確認の記録がない限り無効のままになる（AC-13、AC-27）。印の後に phase group の実装の記録があるときに印を使わない、今の除外との関係も決める。Part B step 1 の README の同期の commit が記録と印の間に入ることも考える（中身のない commit なので、今の規則 1 で無効にならない）。

**関連:** F-002、AC-10〜AC-13

## H-03: 判定のスクリプトの変更（merge の後の確認の記録と、START の決め方）と、そのテスト

**懸念:** 判定のスクリプトには 3 つの変更が要る。
- **記録の START の確かめ:** 今は、START が今の merge-base かブランチの commit である記録だけを読む（`review-records.sh:46-52`、`:87-91`）。merge で merge-base が動くと、最初の記録（START が古い merge-base）が無視され、Fable が PR の全体を走り直す。AC-32 は、その記録を読み続けることを求める。START が `origin/main` の commit で END の祖先であることを確かめれば、記録の範囲（START..END）はブランチの commit だけのままなので、有効と判定する側は広がらない。
- **merge の後の確認の記録:** merge commit を名指しした確認の記録（`not run` を含む）があれば、その merge commit を記録を無効にしない commit に数える（AC-27、AC-28）。また、記録を無効にする commit が、確認の記録のない merge commit だけのときは、Fable ではなく merge の後の確認を求めることを、出力で区別する必要がある（AC-24）。今の出力は `VALID` か `MISSING <START>` だけである。
- **START の決め方:** Fable を走らせ直すとき、記録の後に merge commit が 1 つでもあれば、START を新しい merge-base にする（AC-14）。今は、最初に規則に当たらない commit の第 1 親がブランチの commit なら、その第 1 親を START にする（`review-records.sh:570-578`）。

スクリプトは `origin/main` から取り出して走るので、変更がスクリプトに効くのは本タスクのその PR のマージ後である。本タスクの実装の PR 自身の判定には、変更の前のスクリプトが使われる。

**なぜ大事か:** 確認の記録で merge commit を数える規則は、有効と判定する側を広げる。名指しの読み違えで、確認していない merge commit を数えると、関門を素通りする。

**案:** 出力に、merge の後の確認だけを求める形（例：`MERGE <merge commit の ID> ...`）を足す。確認の記録の行は、merge commit の 40 桁の ID を持ち、その ID が merge commit で、行を持つ commit の祖先であるときだけ数える。スクリプトのテスト（`cmd/yt2column/review_records_test.go`）に、次の形のブランチを足す。
- 記録 → merge commit（確認を求める）
- 記録（START＝古い merge-base）→ merge commit → 確認の記録（有効）
- 記録 → merge commit → 直し → 確認の記録（有効）
- 記録 → 中身のある commit X → merge commit → 見た範囲が X を含む確認の記録（有効でない。Fable が PR の全体を見る）
- 記録 → merge commit → 確認の記録（有効）
- 記録 → merge commit → `not run` の確認の記録（有効）
- 記録 → 確認の記録が別の commit を名指し → merge commit（確認を求める）
- 記録 → 中身のある commit → merge commit（`MISSING <新しい merge-base>`）
- 起動の後、記録の commit の前の merge commit（確認を求める）
- 記録 → 印 → CHECKS の直し（有効）
- 記録 → 印 → merge commit（確認を求める）
- 記録 → 中身のある commit → 印 → CHECKS の直し（有効でない）

実装の PR の順序（スクリプトの変更を先の PR に分けるか）は、計画で決める。

**関連:** AC-10〜AC-15、AC-24、AC-27、AC-28、AC-30、AC-32

## H-04: PR の説明の「どちらの CLI もレビューしなかった commit」を求める方法

**懸念:** AC-16〜AC-19 は、中身のない commit の定義を `/runpr` の `coverage` と揃えるが、見た範囲の行は、merge で merge-base が動いた後も数える。`coverage` は、START が古い merge-base の見た範囲の行を無視する（`review-records.sh:46-52`）ので、そのままは使えない。#987 のブランチでは、`coverage none` が 6 件を挙げる（Fable の部分レビューの実験）。今の規則（`task-pr-handoff.md:250-260`）は、セッションのレビューの SUMMARY、`not run` の行、Opus のレビューの直しを組み合わせて範囲を書く。

**なぜ大事か:** 一覧の誤りは、人が PR レビューを追加で頼むかの判断を誤らせる。ただし、PR を開く関門ではない。

**案:** 判定のスクリプトに、見た範囲の行の START の確かめを「merge-base か、ブランチの commit か、その祖先で `origin/main` が届く commit」に緩めたモードを足す。または、セッションのレビューの記録から範囲を組み立てる今の規則を、記録の後の commit まで広げる。前者は、`coverage` の判定（`/runpr` の関門）を変えずに、PR の説明だけに使う。計画書のパス（`<PLAN>`）を渡すかも、`/runpr` と揃える。

**関連:** F-003、AC-16〜AC-19

## H-05: merge の後の確認の手順の置き場所と形

**懸念:** merge の後の確認（F-005）は、Fable の部分レビューと同じ部品（範囲の一覧、プロンプト、状態の確かめ、出力の確かめ、トリアージ、直しのローカルの CLI レビュー、記録の commit）を持つ。決まっていないのは、`fable-review.md` に入口として足すか、別の共通の手順にするかである。ほかに次の点も決める。
- 「手で解いた箇所」の求め方。たとえば `git show --remerge-diff <merge>` や `git merge-tree` で、自動の merge の結果と比べる。
- 「`main` から入った文書の変更を参照する箇所」の求め方。
- 記録の行の形と、モデル ID の確かめ方（Opus の ID の接頭辞）。
- 待つ時間の上限。

**なぜ大事か:** 部品を写すと、Fable の手順と食い違いが生じうる。手で解いた箇所を取り違えると、確認の範囲から衝突の解決が漏れる。

**案:** `fable-review.md` に merge の後の確認の入口を足し、状態の確かめ、出力の確かめ、トリアージ、記録の commit の部品を共有する。手で解いた箇所は `git show --remerge-diff` で求め、その git の振る舞いを実験で確かめる。

**関連:** F-005、AC-24〜AC-31

## H-06: `/runplan` で、merge の後の確認と Opus のレビューのやり直しが重なる

**懸念:** `/runplan` で merge commit ができると、AC-09 によって Opus のレビューの判断をやり直し、実装モデル要件によっては Opus のレビューが走る。そのうえ、AC-24 によって merge の後の確認（Opus のサブエージェント）も走る。同じ merge を、Opus のサブエージェントが 2 回見ることがある。

**なぜ大事か:** 費用が重なる。一方で、2 つのレビューの役目（Opus のレビューは正しさと弱いモデルの失敗、merge の後の確認は衝突の解決と文書の整合）は違う。

**案:** 順序（どちらを先に走らせるか）と、片方の結果をもう片方に渡すかを決める。重なりを受け入れるなら、その理由を設計書に書く。

**関連:** AC-09、AC-24
