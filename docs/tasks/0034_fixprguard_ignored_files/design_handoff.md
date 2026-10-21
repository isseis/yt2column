# 設計への申し送り：0034 fixprguard_ignored_files

要件定義の際に挙がった、設計で扱う事項を記録する。`02_architecture.md` は、各項目について採った方針、または当てはまらない理由を記録する。

## H-01: 対象のファイルの列挙の手段と費用

-   無視されるファイルの列挙の手段（例: `git ls-files --others --ignored --exclude-standard -z`）と、無視されるディレクトリをまとめて報告する `--directory` を使うかどうかを決める。`--directory` を使うと、`build/` や `extension/node_modules/` の中の `.go`・`Makefile` が見えなくなる（AC-07）。
-   `extension/node_modules/` がある作業ツリーで、列挙と内容の読み取りにかかる時間と出力の大きさを測る（要件書 4.1）。`CheckState` は `finish` の中で何度も呼ばれる（`internal/fixprguard/worktree.go:196-267`）。
-   列挙の出力の上限は、既存の git の出力の上限（`cmd/fixprguard/main.go:45` の 64 MiB）で足りるか確かめる（AC-16）。
-   列挙の結果のパスに、実行関連のファイルの定義（`internal/fixprguard/scope.go`）をどう当てはめるか決める。定義の関数を範囲の規則と共有し、定義を二重に持たない（要件書 4.5）。

## H-02: 正当な変化による誤検出

-   macOS の Finder はルートに `.DS_Store` を作る。これが `~/.gitignore_global` などで無視されると、ルートのドットファイル（`/.*`）として対象のファイルに当たる。修正のフェーズの間に Finder がそれを書き換えると、誤って止まる。エディタがルートに作るスワップファイル（`.<name>.swp` など）も同じである。
-   設計で、このような誤検出が実際に起こるかを確かめる。除外が必要なら、それは対象の範囲を変える決定であり、要件書に戻して決める（要件書 1「要件定義の際に決めたこと」）。
-   Claude Code が修正のフェーズの間に `.claude/` の下に書くファイルが、`settings.local.json` のほかにあるか確かめる。あれば、要件書の AC-05 と同じ扱い（止まる）でよいかを要件書に戻して決める。

## H-03: 基準値の形と protocol

-   基準値のトークンの版（`baselineVersion = "v1"`、`internal/fixprguard/gitstate.go:30`）と欄の数を変えるか決める。古い基準値を拒否すること（AC-03）を、形の検査でどう保証するか決める。
-   `parts` に `ignored` を加えることは出力の契約の変更なので、`Protocol`（`internal/fixprguard/types.go:20`）を上げるかを決める。上げる場合は、`/fixpr`・`/runpr` の文書が渡す `--protocol 1`（`.claude/commands/fixpr.md` など）と、ガイド 4 章の「現在の番号」を合わせて改める。
-   ガイドの `parts` の一覧を検査する文書のテスト（`cmd/fixprguard/docs_test.go`）に、`ignored` を加えるか決める。

## H-04: シンボリックリンクと、通常のファイルでないもの

-   対象のファイルがシンボリックリンクのとき、リンクの先の文字列と、リンクがたどる先の内容のどちらを状態に含めるか決める（AC-08）。hooks の digest（`internal/fixprguard/gitstate.go:312-356`）は両方を含める。リンクの先が作業ツリーの外を指す場合も扱う。
-   FIFO・デバイスなど、通常のファイルでないものは、内容を読まずに種類として記録するか、`git-failed` で止まるかを決める。どちらでも、読み取りで待ち続けないこと（AC-16、`openRegular`。`internal/fixprguard/gitstate.go:236-254`）。

## H-05: 比較の順序

-   `CheckState`（`internal/fixprguard/gitstate.go:514-560`）は、ファイルだけから読める config・info を先に比べ、git を起動するのはその後である。対象のファイルの列挙には git を起動するので、config の比較の後のどこで比べるかを決める。
-   ほかの部分と対象のファイルが同時に違う場合に `parts` に何を並べるかを決める（AC-14 は、対象のファイルだけが違う場合だけを定める）。

## H-06: 0030 の承認済みの文書を改める手順

-   0030 の `01_requirements.md` 4.2 と `02_architecture.md` 5.3 を改めること（AC-20）は、残余のリスクの範囲を変える。`requirements_process.md`「Editing an approved document」の「Decision change」と「Editorial correction」のどちらに当たるか判断する。過去のタスクの記録であることを踏まえ、本文を書き換えるのか、本タスクへの参照を追記するのかを決める。
