# 設計への申し送り：大きな入力の解析のテストが CI で時間の上限を超える原因を調べて改善する

要件定義（[01_requirements.md](01_requirements.md)）の段階で挙がった、設計で扱う事項を記す。`02_architecture.md` は、各項目について採った方法か、当てはまらない理由を記録する。

## H-01: `-race` なしの時間の確認の実行の仕方

`-race` なしのビルドの時間の確認（AC-05・AC-07）を、CI と手元でどう実行するかを決める。候補は、`make test-ci` に `-race` なしの `go test` を 1 つ加える、別のターゲットを設ける、CI の `Test` ジョブに手順を加える、などである。今の `make test`（`Makefile:83-84`）は `-race` 付きだけなので、手元の開発者が `make test` で時間の確認を実行できるかどうかもあわせて決める。

## H-02: `-race` 付きのビルドで時間を確かめないようにする手段

AC-08 を満たす手段を決める。メモリの確保量の上限は、すでにビルドタグ（`race` / `!race`）でファイルを分けて定めている（`internal/transcript/copied_format_race_test.go:1`・`copied_format_norace_test.go:1`）。同じ方法を時間の確認に使えるかを確かめる。`make lint` は `test,integration` のタグだけを渡すので、`race` のファイルは lint されない（`copied_format_race_test.go:13-15`）。

## H-03: CI での単独と並行の比較の手順

AC-02 の計測を CI の runner でどう行うかを決める。一時的なワークフローの変更や、一時的なテストの出力を使う場合は、計測が終わったら元に戻し、`main` に残さない。CI の runner の計測には、ネットワークに触れる操作（push・CI の再実行）が要る。

## H-04: `-race` 付きのメモリの確保量が約 92 倍になる理由

AC-03 のプロファイルで確かめる。仮説（未確認）: パーサは `regexp` を使っており（`internal/transcript/copied_format.go:78-88`）、`regexp` は内部で `sync.Pool` を使う。`-race` の下では `sync.Pool` が `Put` の一部をわざと捨てるため、照合のたびに新しい状態を確保している可能性がある。この仮説が正しければ、確保量の増加は `-race` に固有であり、本番のビルドには現れない。

## H-05: 時間の上限の値

AC-05・AC-06 の上限の値を、CI の runner での `-race` なしの実測（AC-02 と同じ条件で取る）に余裕を持たせて決める。手元の `-race` なしの最大は 0.116 秒（`shortest_form_A_lines`）である。0015 の目標（1 秒未満）と予算（30 秒未満）との関係を記録する。

## H-06: 0015 の文書の扱い

本タスクは、0015 の設計 §7.3「時間」と実装計画のステップ 4-2 の決定（`-race` 付きで 15 秒）を変える（01_requirements.md §5）。0015 の文書の `Comments` に本タスクでの変更を記録するか、本タスクの文書の記録だけにするかを決める。タスク 0038 は、承認済みの文書の `Comments` に `(decision)` を記録した。テストのコメント（`internal/transcript/copied_format_test.go:1024-1030`）も、新しい測り方に合わせて直す。
