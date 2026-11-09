# 設計への申し送り：0076 fixpr_local_review_guard

要件定義の際に挙がった、設計で扱う事項を記録する。`02_architecture.md` は、各項目について採った方針、または当てはまらない理由を記録する。

## H-01: ガードツールの呼び出しの形

-   比較（AC-01・AC-03・AC-04）と push（AC-07・AC-08）を、どのサブコマンドで行うかを決める。issue #426 は、比較だけを行うサブコマンド（例: `check --baseline <TOKEN>`）を挙げていた。既存の `push` に、Phase 4 の commit を渡す引数を加える案もある。
-   今の `Push` は、HEAD の親が基準値の commit だけであることと、push 先の OID が基準値と同じであることを求める（`internal/fixprguard/worktree.go:571-589`）。Phase 4.5 の push では、HEAD から Phase 4 の commit までが 1 本につながること（AC-04）と、push 先の OID が Phase 4 の commit であること（AC-07）を求める。この 2 つの確かめ方を、今の `compareState`（`checkWithRemote`）とどう共有するかを決める。比べ方の規則は Phase 4 の `push` と共有する（要件定義書 4.5）。
-   Phase 4 の commit をツールにどう渡すかを決める。セッションが渡す値を信じてよいかも決める。たとえば、ツールが基準値の commit を親に持つことを確かめれば、セッションが別の commit を渡しても、検査を緩められない。

## H-02: 呼び出しを書く場所

-   比較の呼び出しを、`fixpr.md` の Phase 4.5 と `_lib/local-cli-review.md` のどちらに書くかを決める。`local-cli-review.md` は 6 つの呼び出し元が共有し、`fixpr.md` 以外は基準値を持たない（AC-11）。呼び出し元が「commit の前に行うこと」を渡す入力を `local-cli-review.md` に加える案と、`fixpr.md` に手順 5 の commit を置き換える記述を書く案がある。
-   AC-03 の「build checks の前」の比較は、手順 5 の build checks の直前である。修正の編集は、それより前に行われる。セッションの編集そのものは git を起動しないので、編集の前には比べない。この区切りでよいかを確かめる。
-   README の同期の commit（`fixpr.md:712-716`）は、`mktrans.md` の手順が git を起動するかを確かめ、その前に比べるかを決める（AC-03 は stage の前の比較だけを求める）。

## H-03: `--protocol` と古いツール

-   引数や出力の契約を変えるなら `Protocol` を上げる（`fixprguard_guide.md` 9 章）。上げるかどうかと、`/fixpr`・`/runpr` の文書が渡す `--protocol 5` を揃える手段を決める。
-   AC-10 を満たす手段を決める。`--protocol` を上げれば、古いツールは最初の呼び出し（`/runpr` の `interaction-limits` か、`/fixpr` の `begin`）で止まる。上げない場合は、Phase 4.5 の最初の呼び出しが、古いツールで使い方の誤り（終了コード `2`）になることで止まるかを確かめる。

## H-04: 止まったときの報告

-   AC-02・AC-05・AC-07 の Final report の文面を、`fixpr.md` の Final report の既存の項目（`stopped: the local CLI review stopped` など）にどう組み込むかを決める。
-   push されていない commit の数（AC-05・AC-07）の求め方を決める。Phase 4 の commit から HEAD までの数でよいか確かめる。

## H-05: テスト

-   AC-04・AC-07 の「1 本につながる」の判定に、マージの commit、Phase 4 の commit を含まない HEAD、ブランチの切り替えを入れたテストを用意する。
-   文書のガード（`cmd/fixprguard/docs_test.go`）が、`fixpr.md` の Phase 4.5 のガードツールの呼び出しと、`git push` を直接実行しないことを確かめるかを決める。CLAUDE.md の「Document tests」は、エージェントが信頼の境界で実行するコマンド（`fixprguard` の呼び出し箇所、ガードを迂回するコマンド）を、テストしてよい例外としている。
