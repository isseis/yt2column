# 設計への申し送り：0034 fixprguard_ignored_files

要件定義の際に挙がった、設計で扱う事項を記録する。`02_architecture.md` は、各項目について採った方針、または当てはまらない理由を記録する。

## H-01: 対象のファイルの列挙の手段と費用

-   無視されるファイルの列挙の手段（例: `git ls-files --others --ignored --exclude-standard -z`）と、無視されるディレクトリをまとめて報告する `--directory` を使うかどうかを決める。`--directory` を使うと、`build/` や `extension/node_modules/` の中の `.go`・`Makefile` が見えなくなる（AC-07）。
-   無視されるディレクトリの中の入れ子のリポジトリは、`git ls-files --others` が中身を個別に列挙せず、ディレクトリとして示すことがある。要件定義書 3 章は対象をパスで決めるので（AC-07）、その中のファイルをどう数え上げるかを決める。
-   `extension/node_modules/` がある作業ツリーで、列挙と内容の読み取りにかかる時間と出力の大きさを測る（要件定義書 4.1）。`CheckState` は `finish` の中で何度も呼ばれる（`internal/fixprguard/worktree.go:196-267`）。
-   列挙の出力の上限は、既存の git の出力の上限（`cmd/fixprguard/main.go:45` の 64 MiB）で足りるか確かめる。上限が数えるのが対象のファイルか、git が列挙するすべての無視されるファイルかを決め、`extension/node_modules/` がある作業ツリーで `begin` が止まらないことを確かめる（AC-01a・AC-16）。
-   列挙の結果のパスに、コマンドの実行に関わるファイルの定義（`internal/fixprguard/scope.go`）をどう当てはめるか決める。定義の関数を範囲の規則と共有し、定義を二重に持たない（要件定義書 4.5）。

## H-02: 正当な変化による誤検出

-   macOS の Finder はルートに `.DS_Store` を作る。これが `~/.gitignore_global` などで無視されると、ルートのドットファイル（`/.*`）として対象のファイルに当たる。修正のフェーズの間に Finder がそれを書き換えると、誤って止まる。エディタがルートに作るスワップファイル（`.<name>.swp` など）も同じである。
-   設計で、このような誤検出が実際に起こるかを確かめる。除外が必要なら、それは対象の範囲を変える決定であり、要件定義書に戻して決める（要件定義書 1「要件定義の際に決めたこと」）。
-   build checks（`make fmt`・`make test`・`make lint`）が、無視される場所に対象のファイル（生成された `.go` ファイルなど）を書くかを確かめる。書けば `finish` が毎回止まる（AC-10）。
-   Claude Code が修正のフェーズの間に `.claude/` の下に書くファイルが、`settings.local.json` のほかにあるか確かめる。あれば、要件定義書の AC-05 と同じ扱い（止まる）でよいかを要件定義書に戻して決める。

## H-03: 基準値の形と protocol

-   基準値のトークンの版（`baselineVersion = "v2"`、`internal/fixprguard/gitstate.go:30`）と欄の数を変えるか決める。古い基準値を拒否すること（AC-03）を、形の検査でどう保証するか決める。
-   `parts` に `ignored` を加えることは出力の契約の変更なので、`Protocol`（`internal/fixprguard/types.go:20`）を上げる（ガイド 9 章「出力または引数の契約を変えるときは `Protocol` を上げる」、要件定義書 4.4）。設計では、新しい番号と、`/fixpr`・`/runpr` の文書が渡す `--protocol 2`（`.claude/commands/fixpr.md` など）とガイド 4 章の「現在の番号」をその番号に揃えて改める手段を決める。
-   ガイドの `parts` の一覧を検査する文書のテスト（`cmd/fixprguard/docs_test.go`）に、`ignored` を加えるか決める。

## H-04: シンボリックリンクと、通常のファイルでないもの

-   対象のファイルがシンボリックリンクのとき、リンクの先の文字列と、リンクがたどる先の内容のどちらを状態に含めるか決める（AC-08）。hooks の digest（`internal/fixprguard/gitstate.go:322-366`）は両方を含める。リンクの先が作業ツリーの外を指す場合も扱う。
-   FIFO・デバイスなど通常のファイルでないものは、内容を読まずに種類として記録する（要件定義書 1 章の決定、AC-08・AC-16）。種類を確かめてから開くまでの間に置き換えられても、読み取りで待ち続けない手段を決める（`openRegular`。`internal/fixprguard/gitstate.go:243-261`）。

## H-07: 基準値に置く値の形

-   基準値・出力に、対象のファイルごとの digest を置かない（要件定義書 AC-15・4.2）。`.env` のように内容の候補が少ないファイルは、ファイルごとの digest から内容を推測できるからである。すべての対象のファイルの状態をまとめた 1 つの値にする手段と、残る推測の可能性（要件定義書 4.2 の残余のリスク）を記録する。

## H-05: 比較の順序

-   `CheckState`（`internal/fixprguard/gitstate.go:557-607`）は、ファイルだけから読める git のディレクトリの場所・config のファイルの種類・info を先に比べ、git を起動するのはその後である（最初に起動するのは、コマンドを起動しない `git config --list`）。対象のファイルの列挙には git を起動するので、config の比較の後のどこで比べるかを決める。
-   ほかの部分と対象のファイルが同時に違う場合に `parts` に何を並べるかを決める（AC-14 は、対象のファイルだけが違う場合だけを定める）。

## H-06: 0030 の承認済みの文書を改める手順

-   0030 の `01_requirements.md` 4.2 と `02_architecture.md` 5.3 を改めること（AC-20・AC-20a）は、残余のリスクの範囲を変える。`requirements_process.md`「Editing an approved document」の「Decision change」と「Editorial correction」のどちらに当たるか判断する。過去のタスクの記録であることを踏まえ、本文を書き換えるのか、本タスクへの参照を追記するのかを決める。
