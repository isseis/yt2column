# 設計への引き継ぎ：Fable の部分レビューの記録の後、push の前にできた commit を、PR を開く前に確かめる

`02_architecture.md` が扱う項目。どれも、要件定義（[01_requirements.md](01_requirements.md)）とそのセルフレビューで出た、手段の選択である。

## H-01: push の直前の再判定を、手順のどこに、どの入口として置くか

**懸念:** 再判定（AC-01〜AC-09）は、`_lib/task-pr-handoff.md` Part B の step 2（CHECKS）と step 3（push）の間に置くことになる。決まっていないのは、`fable-review.md` に新しい入口（たとえば「pre-push」）を足すか、既存の入口を広げて使うかである。チェックポイントの入口（Entry 3）は、有効なら印の commit を作るが、AC-02 は有効なときに commit を足さないと決めているので、そのままは使えない。

判定のスクリプトの `<ENTRY>` は今、`launch` と `checkpoint` だけを受け付ける（`review-records.sh:509`）。`/runplan` の PR では、push の直前の判定でも、チェックポイントの印の後の commit を無効にしない規則 3 が効く必要がある（AC-10）。

**なぜ大事か:** 入口の数と、それぞれが何を作るかは、有効かの判定の規則（`fable-review.md`「The validity decision」）と、判定のスクリプトの引数に響く。

**案:** 新しい入口を足す。有効なら何も作らない。有効でなければ、単独の経路（`fable-review.md`「The standalone path」）を走らせ、README の同期と CHECKS を走らせ直してから（AC-04）、もう一度判定する（AC-05）。`/runplan` では、その間に Opus のレビューの判断をやり直す（AC-09）。`<ENTRY>` は、`/runplan` の PR では `checkpoint`、タスク文書の PR では H-02 の手段に合う値を渡す。

**関連:** F-001、AC-01〜AC-09

## H-02: タスク文書の PR の CHECKS の直しを、記録を無効にしない commit と見分ける方法

**懸念:** `/runplan` では、チェックポイントの印の後の commit を、判定のスクリプトの規則 3 が無効にしない（`review-records.sh:104-108`）。タスク文書の PR には印がないので、CHECKS の直しは、今の規則では記録を無効にする。見分けは、判定のスクリプトが commit のデータから行う（要件定義の際に決めたこと）。セッションが「記録の後に commit がないこと」を確かめてから印を作る形は、ADR-0050 が採らなかった「セッションが手順の文に従って判定する」に当たるので、採れない。

**なぜ大事か:** 見分け方を誤ると、CHECKS の直しでない中身のある commit まで無効にしない側に入り（AC-12）、Fable を飛ばして PR が開きうる。判定の関門で誤って有効と判定する側の誤りである。

**案:** タスク文書の PR でも、Part B step 2 の前に有効かを判定し、有効なら印の commit を作る（チェックポイントの入口と同じ形）。規則 3 を、タスク文書の PR でも効くように広げる。印の後に merge commit があるときは、今の「merge commit はどの規則にも当たらない」で無効のままになる（AC-13）。印の後に phase group の実装の記録があるときに印を使わない、今の除外との関係も決める。Part B step 1 の README の同期の commit が記録と印の間に入ることも考える（中身のない commit なので、今の規則 1 で無効にならない）。

**関連:** F-002、AC-10〜AC-13

## H-03: 判定のスクリプトの START の決め方の変更と、そのテスト

**懸念:** AC-14 は、最初に規則に当たらない commit が merge commit のとき、START を新しい merge-base にすることを求める。今は、その第 1 親がブランチの commit なら、第 1 親を START にする（`review-records.sh:570-578`）。merge commit を、「最初に規則に当たらない commit」としてだけでなく、それより後にあるときにも START を merge-base にするかを決める必要がある。たとえば、記録の後に中身のある commit X があり、その後に merge commit があるとき、START を X の親にすると、範囲に `main` 側の変更が入る。

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

**懸念:** AC-16〜AC-19 は、`/runpr` の `coverage` と同じ定義で、見ていない commit を挙げる。セッションが手順の文に従って手で判定すると、`coverage` と食い違いうる。今の規則（`task-pr-handoff.md:250-260`）は、セッションのレビューの SUMMARY、`not run` の行、Opus のレビューの直しを組み合わせて範囲を書く。

**なぜ大事か:** 一覧の誤りは、人が PR レビューを追加で頼むかの判断を誤らせる。ただし、PR を開く関門ではない。

**案:** 判定のスクリプトの `coverage` モードを、LAST_REVIEWED に `none` を渡して走らせる。これで `origin/main..HEAD` の見ていない commit の一覧が得られ、それをそのまま使う（`review-records.sh:26-42`）。今の規則のうち、SUMMARY の範囲と `not run` の行で書く部分を、この一覧で置き換えるか、併記するかを決める。計画書のパス（`<PLAN>`）を渡すかも、`/runpr` と揃える。

**関連:** F-003、AC-16〜AC-19
