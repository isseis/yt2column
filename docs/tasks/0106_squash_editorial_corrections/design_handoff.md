# 設計への申し送り（0106）

要件定義の際に出た、設計で決める事項。`02_architecture.md` が各項目の扱いを記録する。

-   **H-01: 訂正の行を取り出して確かめる場所。** 訂正の行の取り出し、重複の除去と順序（AC-01〜AC-04）、制御文字と不正な UTF-8 の確かめ（AC-09・AC-11）、マージする本文に訂正の行がすべてあることの確かめ（AC-08）を、`mergepr` のツール（`prepare` が行を書いたファイルを作る、`merge` が本文を確かめるなど）に置くか、`mergepr.md` のエージェントの手順に置くかを決める。[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) との関係、ツールに置くときの番兵のエラーの名前、プロトコル番号を上げるかどうか（`CLAUDE.md`「Protocol numbers」）も決める。`log.txt` は今 `%h %s%n%n%b` の形で書かれ（`internal/mergepr/mergepr.go:250`）、コミットの境目と各行がコミットメッセージの何行目かを機械的に読めないことにも触れる（AC-09 の報告）。
-   **H-02: squash の本文の中の置き方。** 訂正の行の段落を本文のどこに置くか（AC-06 の「トレーラーの段落より前」の中で、リードと箇条書きの後ろなど）と、段落の前に見出しや導入の文を付けるかを決める。`Co-Authored-By:` などのトレーラーの段落と混ざらないことを、どう確かめるかも決める。
-   **H-03: 承認の質問での保ち方。** 人が本文の修正を頼んだときに訂正の行を保つことと、行を消す修正を断ること（AC-08）を、`mergepr.md` の step 3 と `runpr.md` のマージの質問のどこに書くかを決める。H-01 で `merge` が本文を確かめるなら、その検査で止まったときの報告と直し方も決める。
-   **H-04: 規則の文書の分担。** 訂正の記録の置き場所を述べる文（`docs/dev/conventions/requirements_process.md` の「Recording in the PR description」と、記録の置き場所を 6 つ挙げる段落（`requirements_process.md:132-142`）、`docs/dev/conventions/adr.md` の「Editorial corrections」の「Recording」、`docs/dev/guides/mergepr_guide.md`）のうち、どれを変え、どこで squash の本文に触れるかを決める（AC-12・AC-13）。`CLAUDE.md` に関係する記述があるかも確かめる。
-   **H-05: 文書のテストとコードのテスト。** 変更で古くなる文書のテスト（`cmd/mergepr/docs_test.go` など）を洗い出す。H-01 でツールを変えるなら、F-001〜F-003 をコードのテストで確かめる方法を決める（AC-15）。文書のテストは値・構造・実行できる例だけを確かめる（[ADR-0016](../../dev/adr/0016-document-test-policy.md)）。
-   **H-06: 止まったときの直し方の案内。** AC-10 の「コミットのメッセージをブランチで書き直す」手順（`git rebase` での書き直しと force push）が、プロトコル番号を上げたブランチの「履歴を書き換えない」規則（`CLAUDE.md`「Protocol numbers」）とぶつかるときの案内を決める。`docs/dev/guides/mergepr_guide.md` に止まる理由として載せるかも決める。
