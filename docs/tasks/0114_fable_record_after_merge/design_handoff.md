# 設計への引き継ぎ：Fable の部分レビューの記録の後、push の前にできた commit を、PR を開く前に確かめる

`02_architecture.md` が扱う項目。どれも、要件定義（[01_requirements.md](01_requirements.md)）とそのセルフレビューで出た、手段の選択である。

## H-01: push の直前の再判定を、手順のどこに、どの入口として置くか

**懸念:** 再判定（AC-01〜AC-09）は、`_lib/task-pr-handoff.md` Part B の step 2（CHECKS）と step 3（push）の間に置くことになる。決まっていないのは、`fable-review.md` に新しい入口（たとえば「pre-push」）を足すか、既存の入口を広げて使うかである。チェックポイントの入口（Entry 3）は、有効なら印の commit を作るが、AC-02 は有効なときに commit を足さないと決めているので、そのままは使えない。

判定のスクリプトの `<ENTRY>` は今、`launch` と `checkpoint` だけを受け付ける（`review-records.sh:509`）。`/runplan` の PR では、push の直前の判定でも、チェックポイントの印の後の commit を無効にしない規則 3 が効く必要がある（AC-10）。

**なぜ大事か:** 入口の数と、それぞれが何を作るかは、有効かの判定の規則（`fable-review.md`「The validity decision」）と、判定のスクリプトの引数に響く。

AC-05 は、push するのを判定した commit とし、判定の後にブランチの先頭が動いたら判定からやり直すことを求める。今の Part B step 3 は、題名と本文を書き、ラベルを読んでから、ブランチの先頭を `git push -u origin <current-branch>` で push する（`task-pr-handoff.md:209-230`）。

**案:** 新しい入口を足す。有効なら何も作らない。push は、判定した commit を名指しして行う（例：`git push -u origin <判定した commit>:refs/heads/<branch>`）か、push の直前にブランチの先頭が判定した commit と同じかを確かめる。有効でなければ、単独の経路（`fable-review.md`「The standalone path」）を走らせ、README の同期と CHECKS を走らせ直してから（AC-04）、もう一度判定する（AC-05）。`/runplan` では、その間に Opus のレビューの判断をやり直す（AC-09）。`<ENTRY>` は、`/runplan` の PR では `checkpoint`、タスク文書の PR では H-02 の手段に合う値を渡す。

**関連:** F-001、AC-01〜AC-09

## H-02: タスク文書の PR の CHECKS の直しを、記録を無効にしない commit と見分ける方法

**懸念:** `/runplan` では、チェックポイントの印の後の commit を、判定のスクリプトの規則 3 が無効にしない（`review-records.sh:104-108`）。要件定義では、タスク文書の PR でも Part B step 2 の前に印を残し、印の後の commit（merge commit を除く）を無効にしないと決めた（要件定義の際に決めたこと「CHECKS の直しの見分け方」）。残るのは、規則 3 の広げ方である。セッションが「記録の後に commit がないこと」を確かめてから印を作る形は、ADR-0050 が採らなかった「セッションが手順の文に従って判定する」に当たるので、採れない。

**なぜ大事か:** 見分け方を誤ると、CHECKS の直しでない中身のある commit まで無効にしない側に入り（AC-12）、Fable を飛ばして PR が開きうる。判定の関門で誤って有効と判定する側の誤りである。

**案:** タスク文書の PR でも、Part B step 2 の前に有効かを判定し、有効なら印の commit を作る（チェックポイントの入口と同じ形）。規則 3 を、タスク文書の PR でも効くように広げる（`<ENTRY>` の値で選ぶか、印があれば効くようにするか）。印の後に merge commit があるときは、今の「merge commit はどの規則にも当たらない」で無効のままになる（AC-13）。印の後に phase group の実装の記録があるときに印を使わない、今の除外との関係も決める。Part B step 1 の README の同期の commit が記録と印の間に入ることも考える（中身のない commit なので、今の規則 1 で無効にならない）。

**関連:** F-002、AC-10〜AC-13

## H-03: 判定のスクリプトの START の決め方の変更と、そのテスト

**懸念:** AC-14 は、記録の後に merge commit が 1 つでもあれば、START を新しい merge-base にすることを求める。記録の後に中身のある commit X を足してから merge したときも同じである。今は、最初に規則に当たらない commit の第 1 親がブランチの commit なら、その第 1 親を START にする（`review-records.sh:570-578`）。

スクリプトは `origin/main` から取り出して走るので、変更がスクリプトに効くのは本タスクのその PR のマージ後である。本タスクの実装の PR 自身の判定には、変更の前のスクリプトが使われる。

**なぜ大事か:** 範囲に `main` 側の変更が入ると、Fable が PR のものでない変更を見て、PR 自身の commit を見ない（要件定義書 1 章の背景）。

**案:** 記録の END から HEAD までに merge commit が 1 つでもあれば、START を merge-base にする。スクリプトのテスト（`cmd/yt2column/review_records_test.go`）に、次の形のブランチを足す。
- 記録（START がブランチの commit）→ merge commit（`MISSING <新しい merge-base>`）
- 記録 → 中身のある commit → merge commit（`MISSING <新しい merge-base>`）
- 起動の後、記録の commit の前の merge commit
- 記録 → 印 → CHECKS の直し（有効）
- 記録 → 印 → merge commit（有効でない）
- 記録 → 中身のある commit → 印 → CHECKS の直し（有効でない）

実装の PR の順序（スクリプトの変更を先の PR に分けるか）は、計画で決める。

**関連:** AC-10〜AC-15

## H-04: PR の説明の「どちらの CLI もレビューしなかった commit」を求める方法

**懸念:** AC-16〜AC-19 は、中身のない commit の定義を `/runpr` の `coverage` と揃えるが、見た範囲の行は、merge で merge-base が動いた後も数える。`coverage` は、START が古い merge-base の見た範囲の行を無視する（`review-records.sh:46-52`）ので、そのままは使えない。#987 のブランチでは、`coverage none` が 6 件を挙げる（Fable の部分レビューの実験）。今の規則（`task-pr-handoff.md:250-260`）は、セッションのレビューの SUMMARY、`not run` の行、Opus のレビューの直しを組み合わせて範囲を書く。

**なぜ大事か:** 一覧の誤りは、人が PR レビューを追加で頼むかの判断を誤らせる。ただし、PR を開く関門ではない。

**案:** 判定のスクリプトに、見た範囲の行の START の確かめを「merge-base か、ブランチの commit か、その祖先で `origin/main` が届く commit」に緩めたモードを足す。または、セッションのレビューの記録から範囲を組み立てる今の規則を、記録の後の commit まで広げる。前者は、`coverage` の判定（`/runpr` の関門）を変えずに、PR の説明だけに使う。計画書のパス（`<PLAN>`）を渡すかも、`/runpr` と揃える。

**関連:** F-003、AC-16〜AC-19
