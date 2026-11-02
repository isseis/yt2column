# 設計への申し送り（0055）

要件の作成で挙がった、設計（`02_architecture.md`）で扱う事項。`02_architecture.md` は、各項目の扱い（採った方針、または当てはまらない理由）を記録する。

## H-01: 道具を書く言語と置き場所、既存の道具の扱い（F-001・F-007、要件 4.5）

-   **論点:** Go の比較と拡張の比較で、判定の大部分（差分による行の対応、削除した関数の判定、結果の出し方）は共通である。1 つの道具で両方の報告を読むか、Go と拡張で別々に書くか。
-   **既存の道具:** 0038 の `docs/tasks/0038_doc_test_pruning/check_pruning.sh:480-505` は、BASE と HEAD の作業用のチェックアウト（`git worktree add --detach`）を作って Go のカバレッジを取る。0052 の `docs/tasks/0052_doc_test_pruning/_checks/coverage/main.go` は、`go tool cover -func` の出力からモジュールのパスの接頭辞と `total:` の行を取り除き、ビルドの制約が `test` で始まるファイルを除いて比べる。これらを再利用するか、ここで決めた形に置き換えるか、残すかを決め、それぞれで決めてあったこと（テストのタグのファイルの見分け方など）を引き継ぐ。
-   **候補:** `cmd/` の下の Go の開発者向けの道具（`cmd/mergepr`・`cmd/fixprguard`・`cmd/codexreview` の先例）として、Go の報告と lcov の両方を読む。拡張の `make ext-coverage-compare` もこの道具を呼ぶ。この場合、拡張の比較で Go の環境が要る。拡張の側に置く場合は、`extension/scripts/` の TypeScript で書く。ただし `extension/scripts/` は要件の「製品のコード」に入るので、道具が自分自身のカバレッジを比べる対象に含めることになる。
-   **決めること:** 置き場所、`make` の対象からの呼び方、道具のテストが `make test` と `make ext-test` のどちらで動くか（AC-35）、`docs/dev/conventions/package_reference.md` への記載。

## H-02: カバレッジの報告とテストの結果の取り方（AC-01・AC-08・AC-10・AC-11・AC-30）

-   **Go:** 0038 は `go test -tags test -count=1 -coverpkg=./... -coverprofile=<file> ./...` を使った（0038 `02_architecture.md` 7.1）。`-race` を付けるかどうか、`-count=1` でテストのキャッシュを使わないこと、テストの数と失敗したテストの名前の読み方（`-json` の出力など）を決める。
-   **Go の網羅率の丸め:** 要件は丸めずに比べることを求める（AC-10）。`go tool cover -func` は丸めた値を出すので、プロファイルの文の数から比べる方法などを決める。
-   **Go の時間の検査:** `TestParseCopiedLargeInputs` の解析の時間の上限は、race 検出なしのビルドでだけ確かめられる（`Makefile:106-114`）。カバレッジの計測を加えたビルドで、この上限を超えて失敗しないかを確かめ、失敗する場合の扱いを決める（そのテストを除くなら、除いたことを結果に出すかどうか）。
-   **拡張:** 0054 は `extension/` をカレントディレクトリにして、`node --test --experimental-test-coverage --test-reporter=lcov --test-reporter-destination=<出力先> --test-reporter=spec --test-reporter-destination=stdout "test/**/*.test.ts"` を実行した（0054 `02_architecture.md` 3.2）。テストの対象の指定は `extension/package.json:12` の `test` スクリプトと同じにする。テストの数をどの出力から読むかを決める。

## H-03: BASE のチェックアウトの作り方と後始末（AC-03・AC-04・AC-06）

-   **論点:** BASE のテストを、利用者の作業ツリーを変えずに実行する。0038 は `git worktree add --detach` で一時的なディレクトリに作った。
-   **拡張の依存パッケージ:** 一時的なチェックアウトには `extension/node_modules` がない。0054 は、前と後の報告を同じ `node_modules` で取った（0054 `02_architecture.md` 3.2）。HEAD の `node_modules` を BASE でも使う方法と、`extension/package-lock.json` が BASE と HEAD で違う場合の扱い（止めるかどうか）を決める。
-   **後始末:** 合格・不合格・途中のエラー・SIGINT と SIGTERM による中断のどれでも、一時的なチェックアウトと報告を残さない方法（`git worktree remove`・`git worktree prune` を含む）。SIGKILL などで残ったものを、次の実行がどう扱うか。
-   **HEAD の測り方:** HEAD も一時的なチェックアウトで測るか、利用者の作業ツリーで測るか。作業ツリーで測る場合、git が無視するファイル（`build/`・`extension/dist/` など）がテストの結果に影響しないかを確かめる。

## H-04: 差分による行の対応と関数の範囲（F-004、AC-31）

-   **先例:** 0027 は、`git diff` で前後の行を対応づけ、行番号のずれを扱った（0027 `02_architecture.md` 3.11）。
-   **論点:** BASE の行と HEAD の行の対応を、`git diff` の hunk から作る。rename の検出を切ること（AC-20 はファイルの移動を削除と追加として扱う）、diff のアルゴリズムの選択、空白の扱いのオプションを使わないこと（変わった行を変わらない行と取り違えないため）、利用者の git の設定（`diff.*`）に左右されないことを決める。
-   **関数の範囲:** Go の `go tool cover -func` と拡張の lcov の `FN` は、関数の始まりの行を出す。終わりの行をどう求めるか（Go はソースの構文解析、拡張は lcov の `FN` の終わりの行の有無の確認など）を決める。拡張の lcov が終わりの行を出さない場合は、TypeScript のソースの構文解析が要る（新しい npm のパッケージは加えられないので、入っている `typescript` を使うことになる）。その答えは H-01 の置き場所の選択を左右するので、`extension/.node-version` の版で報告の形を先に確かめる。
-   **始まりの行の意味:** Go の行は関数の宣言の行、拡張の `FN` の行は関数の始まりの行である。宣言の前のコメントやデコレータの削除で、始まりの行がどう動くかを確かめる。

## H-05: 製品のコードの見分け方（AC-16）

-   **Go:** ビルドの制約が `test` のタグを求める、または `test` のタグで成り立つファイルを、どう見分けるか。0052 は `^//go:build test([^A-Za-z0-9_.]|$)` を `git grep` で探した（`docs/tasks/0052_doc_test_pruning/_checks/coverage/main.go`）。`go list` でタグなしとタグありのファイルの一覧を比べる方法もある。今は `//go:build test` と `//go:build test || integration` がある。
-   **拡張:** lcov の `SF` は `extension/` からの相対パスで出る（0054 `02_architecture.md` 3.2）。`src/` と `scripts/` の判定を、パスの先頭の比較で行うか。`..` を含むパスや絶対パスの扱い。

## H-06: lcov の読み方（AC-29・AC-14）

-   **論点:** `FNDA` の行は名前しか持たないので、ファイルのまとまりの中で `FN` と `FNDA` を出現の順で組にする（0054 `02_architecture.md` 3.2）。要件は、組にした名前が違えば止めることを求める（AC-29）。
-   **node の版:** 道具が前提とする lcov の形は、`extension/.node-version` の版（`24.21.0`）の出力である。版を上げたときに形が変わったことを、道具が止まって知らせることを確かめる。

## H-07: 道具のテストの作り方（F-007）

-   **論点:** 報告と差分を入力にして判定を確かめる（AC-35）。報告の例を `testdata/` に置くか、テストの中で組み立てるか。実際の node の lcov の出力の例を 1 つ置き、形が今の node の版と合っていることを確かめるかどうか。入口の前提の確認と後始末（AC-36）を、テストで確かめるか手動の手順にするか。
-   **壊して確かめる:** 道具の各止まり方のテストが、その止まり方の処理を外すと失敗することを、実装のときに確かめる（CLAUDE.md「Every test must be able to fail for its stated reason」）。

## H-08: 結果の出し方（F-005）

-   **論点:** 最後の行の書式（AC-23）、合格のときの結果の形（コミットメッセージと PR の説明にそのまま貼れる数行）、削除した関数が多いとき（ファイルごと削除したときなど）の一覧の出し方。標準出力と標準エラー出力の使い分け。`make` の終了コードは、レシピの失敗で一律に 2 になるので、不合格と止まった場合の区別は出力の最後の行で行う。

## H-09: go の版の取得の防ぎ方（AC-05）

-   **論点:** `go.mod` が入っている go と別の版を求めると、`GOTOOLCHAIN` の既定（`auto`）では go がその版を取りに行く。BASE と HEAD の `go.mod` の `go`・`toolchain` の行が違う場合も含めて、取りに行かずに止める方法（`GOTOOLCHAIN=local` など）を決める。
