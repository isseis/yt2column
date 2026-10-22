# 設計への申し送り（0035）

要件定義の際に挙がった、設計（`02_architecture.md`）で扱う事項。`02_architecture.md` は、各項目について採った方法か、当たらない理由を記録する。

-   **H-01: ファイルのバイト列の比較をやめた後の、起動の順序の保証。** 今は、config のファイルのバイト列を git を起動する前に比べ、その後で `git config --list` を起動している（`internal/fixprguard/gitstate.go:514-561`）。バイト列の比較をやめるなら、書き換えられた config のまま `git config --list` を起動しても、コマンドが起動されないことを確かめる。確かめる対象は、`gitHardening` の `-c` が上書きしない設定、`include.path`・`includeIf` の読み込み、壊れた config のときの振る舞いである。確かめられない場合は、バイト列の比較を残したうえで、AC-01・AC-02・AC-10 を満たす方法を考える。
-   **H-02: ほかのブランチの設定を読む git の有無。** ツールが起動する git（`status`・`diff`・`diff-tree`・`add`・`commit`・`ls-files`・`ls-remote`・`push`・`rev-parse`・`symbolic-ref`・`cat-file`）が、今のブランチ以外の `branch.<name>.*` を読まないことを確かめる。`push` は明示した refspec を使うので、`push.default` や upstream の解決には頼らない（`gitstate.go:612-650`）。build checks（`make test`）が起動する git は、ツールの守りの対象ではない（0030 `02_architecture.md` 5.3）。
-   **H-03: 今のブランチの名前の求め方と比較の時点。** `CheckState` は、HEAD の確認（`headState`）より前に呼ばれる（`gitstate.go:747-776`）。今のブランチの名前を、比較のどの時点に、どの手段で求めるかを決める。基準値は、ブランチの名前を digest でしか持たない（`gitstate.go:98-113`）。HEAD が別のブランチに移った場合に、`parts` が `head` と `config` のどちらになるかは、AC-12 の範囲で決めてよい。名前を求める git も、比較の前に起動される git なので、H-01 と同じ順序の保証の対象になる。
-   **H-04: 設定の名前の分解。** `git config --list -z` は、セクションと変数の名前を小文字にして出力し、サブセクションはそのまま出力する（古い `[branch.Name]` の書き方では、サブセクションも小文字になる）。サブセクションを、最初の `.` と最後の `.` の間として取り出す。`--show-origin -z` の出力の形（読み込み元・名前・値の区切り）を、使う git の版で確かめる。AC-04 の `branch.feature.a.remote` と `branch.Feature.remote` は、テストの入力の候補である。
-   **H-05: 基準値のトークンの形と `Protocol`。** トークンの版（`baselineVersion`、`gitstate.go:28-32`）を上げるか、項目の数を変えるかを決める。いずれにしても、前の版のトークンは `ParseBaseline` が `ErrInvalidInput` で拒否する（AC-13）。トークンの形の変更が、ガイド 9 章の「出力または引数の契約の変更」に当たるかを判断する。当たれば `Protocol` を上げる（AC-17）。
-   **H-06: タスク 0034 との順序。** 0034 の要件案（PR #429）も、基準値に項目を加える。本タスクのトークンの形を、0034 が上に重ねやすい形にする。0034 の要件案の `gitstate.go` の行番号の引用は、0034 の側で合わせる。
-   **H-07: テストの構成。** 一時ディレクトリの実際のリポジトリで、別の worktree からほかのブランチの設定を変える場合と、`git config` で直接書く場合を作る（0030 のテストの方法。ガイド 9 章）。CLAUDE.md「A layered path needs inputs only one layer can handle」に従い、バイト列の比較を残す場合は、どちらの比較が止めたのかを区別できる入力を使う。
