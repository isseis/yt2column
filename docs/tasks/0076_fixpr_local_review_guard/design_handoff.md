# 設計への申し送り：0076 fixpr_local_review_guard

要件定義の際に挙がった、設計で扱う事項を記録する。`02_architecture.md` は、各項目について採った方針、または当てはまらない理由を記録する。

## H-01: ガードツールの呼び出しの形

-   比較（AC-01・AC-02）と push（AC-07・AC-08）を、どのサブコマンドで行うかを決める。issue #426 は、比較だけを行うサブコマンド（例: `check --baseline <TOKEN>`）を挙げていた。既存の `push` に引数を加える案もある。
-   今の `Push` は、HEAD の親が基準値の commit だけであることと、push 先の OID が基準値と同じであることを求める（`internal/fixprguard/worktree.go:571-589`）。Phase 4.5 では、HEAD から基準値の commit までが 1 本につながることと、push 先の OID が Phase 4 の commit であることを求める（要件定義書 3 章の冒頭の定義）。この確かめ方を、今の比較（`compareState`）とどう共有するかを決める。比べ方の規則は Phase 4 の `push` と共有する（要件定義書 4.5）。
-   Phase 4 の commit は、ツールが git の状態から求める（AC-02）。呼び出し元に commit を渡させる場合は、その値を求めた値との照合にだけ使い、検査を緩めないことを確かめる。HEAD から親をたどる長さに上限を置くかを決める（Phase 4.5 の commit は多くても数個である）。
-   ローカルの CLI レビューの BASE は、今はセッションが Phase 4 の結果の `commitSha` から求める（`fixpr.md:703-704`）。ツールが求めた Phase 4 の commit と照合するか、ツールの値を使うかを決める。違う値はレビューの範囲をずらすが、比較を緩めはしない。

## H-02: 呼び出しを書く場所

-   比較の呼び出しを、`fixpr.md` の Phase 4.5 と `_lib/local-cli-review.md` のどちらに書くかを決める。`local-cli-review.md` は 6 つの呼び出し元が共有し、`fixpr.md` 以外は基準値を持たない（AC-11）。呼び出し元が「git を起動する前に行うこと」を渡す入力を `local-cli-review.md` に加える案と、`fixpr.md` に置き換えの記述を書く案がある。
-   AC-01 の時点の一覧が、手順が定める git の起動をすべて覆っているかを、設計の時点の `fixpr.md`・`local-cli-review.md`・`mktrans.md` に照らして確かめる。漏れがあれば要件定義書に戻す。

## H-03: `--protocol` と古いツール

-   引数や出力の契約を変えるなら `Protocol` を上げる（`fixprguard_guide.md` 9 章）。上げるかどうかと、`/fixpr`・`/runpr` の文書が渡す `--protocol 5` を揃える手段を決める。
-   AC-10 を満たす手段を決める。`--protocol` を上げれば、古いツールは最初の呼び出し（`/runpr` の `interaction-limits` か、`/fixpr` の `begin`）で止まる。上げない場合は、Phase 4.5 の最初の呼び出しが、古いツールで使い方の誤り（終了コード `2`）になって止まるかを確かめる。

## H-04: 止まったときの報告

-   AC-05 の Final report の内容を、`fixpr.md` の Final report の既存の項目（「Stops to act on」「Editorial corrections」「Local CLI review」）にどう組み込むかを決める。
-   対象のファイルの違いで止まったとき、止まった時点の記録のファイル（`fixprguard-ignored-stop`）を、既存の `inspect`・`finish`・`push` と同じく書く（AC-05）。
-   push されていない commit の数と、ローカルの CLI レビューを受けていない範囲の求め方を決める。
-   (b)・(c)・push 先の OID のどれに合わなかったかを、`parts` の今の値（`head`・`remote` など）で表すか、新しい値を加えるかを決める。新しい値を加えるなら出力の契約の変更である（H-03）。

## H-05: テスト

-   1 本につながるかの判定に、マージの commit、Phase 4 の commit を含まない HEAD、ブランチの切り替え、基準値の commit を親に持たない列を入れたテストを用意する。
-   文書のガード（`cmd/fixprguard/docs_test.go`）が、`fixpr.md` の Phase 4.5 のガードツールの呼び出しと、`git push` を直接実行しないことを確かめるかを決める。CLAUDE.md の「Document tests」は、エージェントが信頼の境界で実行するコマンド（`fixprguard` の呼び出し箇所、ガードを迂回するコマンド）を、テストしてよい例外としている。
