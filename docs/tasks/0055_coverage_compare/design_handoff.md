# 設計への申し送り（0055）

要件の作成で挙がった、設計（`02_architecture.md`）で扱う事項。`02_architecture.md` は、各項目の扱い（採った方針、または当てはまらない理由）を記録する。

## H-01: 道具を書く言語と置き場所（F-001・F-006、要件 4.5）

-   **論点:** Go の比較と拡張の比較で、判定の大部分（差分による行の対応、削除した関数の判定、結果の出し方）は共通である。1 つの道具で両方の報告を読むか、Go と拡張で別々に書くか。
-   **候補:** `cmd/` の下の Go の開発者向けの道具（`cmd/mergepr`・`cmd/fixprguard`・`cmd/codexreview` の先例）として、Go の報告と lcov の両方を読む。拡張の `make ext-coverage-compare` もこの道具を呼ぶ。この場合、拡張の比較で Go の環境が要る。拡張の側に置く場合は、`extension/scripts/` の TypeScript で書く（0046・0054 の `_checks/` の先例）。
-   **決めること:** 置き場所、`make` の対象からの呼び方、道具のテストが `make test` と `make ext-test` のどちらで動くか（AC-24）、`docs/dev/conventions/package_reference.md` への記載。

## H-02: カバレッジの報告の取り方（AC-01・AC-05・AC-09）

-   **Go:** 0038 は `go test -tags test -count=1 -coverpkg=./... -coverprofile=<file> ./...` を使った（0038 `02_architecture.md` 7.1）。`-race` を付けるかどうか、`-count=1` でテストのキャッシュを使わないこと、テストの数の数え方（`-json` の出力など）を決める。
-   **Go の時間の検査:** `TestParseCopiedLargeInputs` の解析の時間の上限は、race 検出なしのビルドでだけ確かめられる（`Makefile:106-114`）。カバレッジの計測を加えたビルドで、この上限を超えて失敗しないかを確かめ、失敗する場合の扱いを決める（そのテストを除くなら、除いたことを結果に出すかどうか）。
-   **拡張:** 0054 は `extension/` をカレントディレクトリにして、`node --test --experimental-test-coverage --test-reporter=lcov --test-reporter-destination=<出力先> --test-reporter=spec --test-reporter-destination=stdout "test/**/*.test.ts"` を実行した（0054 `02_architecture.md` 3.2）。テストの対象の指定は `extension/package.json:12` の `test` スクリプトと同じにする。
-   **網羅率の精度:** `go tool cover -func` は網羅率を小数第 1 位に丸めて出す。丸めで小さな減少が見えなくなることがあるので、プロファイルの文の数から比べるかどうかを決める（AC-08）。

## H-03: BASE のチェックアウトの作り方（AC-03・AC-04）

-   **論点:** BASE のテストを、利用者の作業ツリーを変えずに実行する。`git worktree add` で一時的なディレクトリに作るなど。
-   **拡張の依存パッケージ:** 一時的なチェックアウトには `extension/node_modules` がない。0054 は、前と後の報告を同じ `node_modules` で取った（0054 `02_architecture.md` 3.2）。HEAD の `node_modules` を BASE でも使う方法と、`extension/package-lock.json` が BASE と HEAD で違う場合の扱い（止めるかどうか）を決める。
-   **後始末:** 合格・不合格・途中のエラー・シグナルによる中断のどれでも、一時的なチェックアウトと報告を残さない方法（`git worktree remove`・`git worktree prune` を含む）。
-   **HEAD の測り方:** HEAD も一時的なチェックアウトで測るか、利用者の作業ツリーで測るか。作業ツリーで測る場合、git が無視するファイル（`build/`・`extension/dist/` など）がテストの結果に影響しないかを確かめる。

## H-04: 差分による行の対応（F-004）

-   **論点:** BASE の行と HEAD の行の対応を、`git diff` の hunk から作る。rename の検出を切るか（AC-17 はファイルの移動を削除と追加として扱う）、diff のアルゴリズムの選択、空白の扱いのオプションを使わないこと（変わった行を変わらない行と取り違えないため）を決める。
-   **始まりの行の意味:** Go の `go tool cover -func` の行は関数の宣言の行、拡張の lcov の `FN` の行は関数の始まりの行である。宣言の前のコメントやデコレータの削除で、始まりの行がどう動くかを確かめる。

## H-05: 本番のコードの見分け方（AC-13）

-   **Go:** テスト用のビルドタグ `test` を付けたときだけビルドされるファイルを、どう見分けるか（`go list` でタグなしとタグありのファイルの一覧を比べるなど）。`//go:build test && integration` のような組み合わせも含めて扱う。
-   **拡張:** lcov の `SF` は `extension/` からの相対パスで出る（0054 `02_architecture.md` 3.2 の `SF` が `src/` で始まるファイル）。`src/` と `scripts/` の判定を、パスの文字列の先頭の比較で行うか。`..` を含むパスや絶対パスの扱い。

## H-06: lcov の読み方（AC-07・AC-11）

-   **論点:** `FNDA` の行は名前しか持たないので、ファイルのまとまりの中で `FN` と `FNDA` を出現の順で組にする（0054 `02_architecture.md` 3.2）。`FN` と `FNDA` の数が違うとき、未知のレコードの種類、重複する `SF` の扱いを決める。
-   **node の版:** 道具が前提とする lcov の形は、`extension/.node-version` の版（`24.21.0`）の出力である。版を上げたときに形が変わったことを、道具が止まって知らせるかを確かめる。

## H-07: 道具のテストの作り方（F-006）

-   **論点:** 用意した報告と差分を入力にして判定を確かめる（AC-24）。報告の例を `testdata/` に置くか、テストの中で組み立てるか。実際の node の lcov の出力の例を 1 つ置き、形が今の node の版と合っていることを確かめるかどうか。
-   **壊して確かめる:** 道具の各止まり方のテストが、その止まり方の処理を外すと失敗することを、実装のときに確かめる（CLAUDE.md「Every test must be able to fail for its stated reason」）。

## H-08: 結果の出し方（F-005）

-   **論点:** 合格のときの結果の形（コミットメッセージにそのまま貼れる数行）と、削除した関数が多いときの一覧の出し方。標準出力と標準エラー出力の使い分け。不合格と止まった場合を出力で区別する方法（AC-20）。
