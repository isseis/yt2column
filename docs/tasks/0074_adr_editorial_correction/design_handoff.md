# 設計への申し送り（0074）

要件定義の際に出た、設計で決める事項。`02_architecture.md` が各項目の扱いを記録する。

-   **H-01: `/fixpr` が ADR の項目を確かめる方法。** AC-06 の「マージ先の `main` にそのファイルがある」を、どの参照（`origin/main`、`HEAD` とのマージベースなど）とどのコマンドで確かめるかを決める。`/fixpr` は `origin/main` を取得しない場合があるので、古い参照で判定したときに何が起きるかも述べる。AC-05 の、パスを ADR のファイルの形に限る照合の書き方も決める。
-   **H-02: `requirements_process.md` と規約の分担。** 規約の「Recording」の、パスを ADR に読み替える文（the document path in them is the ADR's repository-relative path）は、AC-01 によって要らなくなる。残すか消すかを決める。`requirements_process.md` の「Recording in the PR description」は、タスク文書の訂正の規則（「Editing an approved document」）の下にあるので、ADR の訂正の条件を規約へ委ねる文をどこに置くかも決める。
-   **H-03: 文書のテスト。** 変更で古くなる文書のテスト（`cmd/yt2column/fixpr_docs_test.go` の、`fixprguard` の呼び出しと Phase 6 の `--jq` の式を確かめるテストなど）を洗い出し、どれを変えるかを決める。訂正の行の例にタスク文書のパスだけを使うテストを、ADR のパスにも広げるかどうかも決める。
-   **H-04: `fixprguard` のコメント。** `internal/fixprguard/write.go` の `editorialCorrectionPrefix` と `checkCommitBody` のコメントは、訂正の行を「承認済みのタスク文書への訂正」と述べる。振る舞いを変えずにコメントだけを直すかどうかを決める（AC-16 は振る舞いを変えないことだけを求める）。
-   **H-05: `/fixpr` の Final report と `/runpr` の質問。** 落とした ADR の項目と、PR の説明に追記できなかった ADR の訂正が、Final report と `runpr.md` のマージの質問に、タスク文書の訂正と同じく載ることを確かめる。文面を変える必要があるかを決める。
