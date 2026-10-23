# 調査の記録：大きな入力の解析のテストが CI で時間の上限を超える原因

本書は、タスク 0039 の調査の記録である（[02_architecture.md](02_architecture.md)（以下「設計書」）3.8）。[01_requirements.md](01_requirements.md)（以下「要件書」）の AC-01〜AC-04・AC-06・AC-08・AC-13（パーサを改善した場合は AC-15）の記録を置く。承認ではなく PR のレビューで中身を確かめる文書なので、`Document Status` を持たない。用語は要件書 6 章と設計書の冒頭の用語を同じ意味で使う。

値はすべて `TestParseCopiedLargeInputs` の `t.Logf` の行（`internal/transcript/copied_format_test.go:1088`。時間、メモリの確保量、1 バイトあたりの確保量）から読んだ。入力の名前は、サブテストの名前（`go test -v` の出力の形）で書く。

## 1. 計測の環境とコミット

### 1.1. 手元

| 項目 | 値 |
|---|---|
| OS | macOS 26.6.2（25G83） |
| CPU とコア数 | Apple M3 Pro、12 コア（性能コア 6、効率コア 6。`sysctl hw.ncpu hw.perflevel0.physicalcpu hw.perflevel1.physicalcpu`） |
| Go の版 | go1.26.5 darwin/arm64（`go.mod:3` の版。`GOTOOLCHAIN=go1.26.5` で指定した。手元の既定は go1.27.1） |
| 計測したコミット | `fddb4f7`（`main`。本タスクの変更の前） |
| 計測した日時 | 2026-10-23 07:01〜07:22（UTC）。プロファイル（3 章）は 07:22〜07:24（UTC） |

### 1.2. CI

| 項目 | 値 |
|---|---|
| runner | `ubuntu-latest`（イメージ `ubuntu24 20261012.334.1`）、4 コア。CPU の型名はジョブごとに違う（5.3） |
| Go の版 | go1.26.5 linux/amd64（`actions/setup-go` の `go-version-file: go.mod`） |
| 計測したコミット | `9f4065f`。`fddb4f7` に計測のワークフローの 1 ファイル（5.5）だけを加えたコミット |
| 計測した日時 | 2026-10-23 07:00〜07:04 と 10:05〜10:08（UTC） |

## 2. 手元の計測（AC-01）

### 2.1. 実行したコマンド

各条件を 3 回ずつ、(a)→(b)→(c)→(d)→(e) の順で 3 巡させて実行した。どの条件も、リポジトリのルートで `GOTOOLCHAIN=go1.26.5` を環境に置いて実行した。

| 条件 | コマンド |
|---|---|
| (a) `-race` 付き・単独 | `go test -tags test -race -count=1 -v -run '^TestParseCopiedLargeInputs$' ./internal/transcript` |
| (b) `make test` の中 | `go clean -testcache` の後に `make test` |
| (c) (a) にカバレッジ | `go test -tags test -race -count=1 -v -coverprofile=<一時ファイル> -run '^TestParseCopiedLargeInputs$' ./internal/transcript` |
| (d) `make test-ci` と同じ条件 | `go test -tags test -race -count=1 -v -coverprofile=<一時ファイル> ./...` |
| (e) pre-commit のフックと同じ条件（AC-01 の外。実装計画 §6 のリスクの判断に使う） | `go test -tags test -count=1 -v ./...` |

(b) では、3 回とも `internal/transcript` の行が `ok  	github.com/isseis/yt2column/internal/transcript	34.843s` のように時間を示し、`(cached)` でなかった（3 回めは 2.4 のとおり `FAIL` の行）。どの条件の出力にも `internal/transcript` の `(cached)` はなかった。

### 2.2. 結果

時間の単位は秒、メモリの確保量の単位は入力 1 バイトあたりのバイトである。各欄は 3 回の最小 / 中央 / 最大である。

**(a) `-race` 付き・単独**

| 入力 | 時間（秒） | 1 バイトあたりの確保量 |
|---|---|---|
| `shortest_form_A_lines` | 2.792 / 2.809 / 2.862 | 1714.38 / 1715.92 / 1716.82 |
| `one_long_form_A_line` | 0.032 / 0.032 / 0.032 | 0.00 / 0.00 / 0.00 |
| `textless_form_B_timestamps` | 3.029 / 3.031 / 3.411 | 1867.62 / 1869.03 / 1871.47 |
| `trailing_zero-length_lines` | 0.105 / 0.110 / 0.113 | 16.01 / 16.02 / 16.02 |
| `one-byte_prefix_lines` | 0.278 / 0.289 / 0.291 | 8.02 / 8.02 / 8.04 |
| `form_A_headings_between_segments` | 1.122 / 1.156 / 1.166 | 587.55 / 590.20 / 591.51 |
| `one-byte_form_B_headings` | 0.407 / 0.424 / 0.437 | 8.02 / 8.02 / 8.03 |

**(b) `make test` の中**

| 入力 | 時間（秒） | 1 バイトあたりの確保量 |
|---|---|---|
| `shortest_form_A_lines` | 3.012 / 3.042 / 4.826 | 1716.30 / 1717.93 / 1719.15 |
| `one_long_form_A_line` | 0.033 / 0.033 / 0.033 | 0.00 / 0.00 / 0.00 |
| `textless_form_B_timestamps` | 3.141 / 3.354 / 3.413 | 1866.56 / 1869.88 / 1875.20 |
| `trailing_zero-length_lines` | 0.108 / 0.110 / 0.119 | 16.01 / 16.02 / 16.03 |
| `one-byte_prefix_lines` | 0.310 / 0.317 / 0.328 | 8.01 / 8.01 / 8.02 |
| `form_A_headings_between_segments` | 1.220 / 1.230 / 1.607 | 589.44 / 591.72 / 592.97 |
| `one-byte_form_B_headings` | 0.458 / 0.459 / 0.526 | 8.00 / 8.01 / 8.01 |

**(c) (a) にカバレッジ**

| 入力 | 時間（秒） | 1 バイトあたりの確保量 |
|---|---|---|
| `shortest_form_A_lines` | 3.833 / 3.874 / 4.047 | 1711.93 / 1718.91 / 1718.92 |
| `one_long_form_A_line` | 0.417 / 0.434 / 0.439 | 0.00 / 0.00 / 0.00 |
| `textless_form_B_timestamps` | 4.251 / 4.270 / 4.485 | 1865.29 / 1871.98 / 1879.72 |
| `trailing_zero-length_lines` | 1.776 / 1.821 / 1.834 | 16.01 / 16.03 / 16.03 |
| `one-byte_prefix_lines` | 1.723 / 1.779 / 1.817 | 8.02 / 8.03 / 8.03 |
| `form_A_headings_between_segments` | 1.716 / 1.740 / 1.824 | 589.87 / 592.48 / 595.08 |
| `one-byte_form_B_headings` | 2.871 / 2.871 / 2.983 | 8.01 / 8.02 / 8.02 |

**(d) `make test-ci` と同じ条件**

| 入力 | 時間（秒） | 1 バイトあたりの確保量 |
|---|---|---|
| `shortest_form_A_lines` | 4.160 / 4.326 / 6.635 | 1712.72 / 1715.75 / 1717.32 |
| `one_long_form_A_line` | 0.431 / 0.432 / 0.450 | 0.00 / 0.00 / 0.01 |
| `textless_form_B_timestamps` | 4.562 / 4.652 / 5.007 | 1863.24 / 1872.89 / 1876.94 |
| `trailing_zero-length_lines` | 1.863 / 1.889 / 1.932 | 16.02 / 16.02 / 16.03 |
| `one-byte_prefix_lines` | 1.871 / 1.883 / 1.891 | 8.02 / 8.02 / 8.02 |
| `form_A_headings_between_segments` | 1.818 / 1.833 / 1.903 | 589.05 / 590.24 / 591.60 |
| `one-byte_form_B_headings` | 3.008 / 3.098 / 3.175 | 8.01 / 8.01 / 8.02 |

**(e) pre-commit のフックと同じ条件（`-race` なし・全パッケージ並行）**

| 入力 | 時間（秒） | 1 バイトあたりの確保量 |
|---|---|---|
| `shortest_form_A_lines` | 0.102 / 0.102 / 0.107 | 18.67 / 18.67 / 18.68 |
| `one_long_form_A_line` | 0.007 / 0.007 / 0.007 | 0.00 / 0.00 / 0.00 |
| `textless_form_B_timestamps` | 0.101 / 0.102 / 0.104 | 12.80 / 12.81 / 12.82 |
| `trailing_zero-length_lines` | 0.054 / 0.055 / 0.055 | 16.00 / 16.01 / 16.01 |
| `one-byte_prefix_lines` | 0.041 / 0.042 / 0.043 | 8.00 / 8.00 / 8.01 |
| `form_A_headings_between_segments` | 0.050 / 0.050 / 0.051 | 6.58 / 6.58 / 6.58 |
| `one-byte_form_B_headings` | 0.051 / 0.052 / 0.052 | 8.00 / 8.01 / 8.01 |

参考として、`-race` なし・単独（`go test -tags test -count=1 -v -run '^TestParseCopiedLargeInputs$' ./internal/transcript`）も各巡の最後に 1 回ずつ実行した。`shortest_form_A_lines` は 0.099 / 0.108 / 0.113 秒、`textless_form_B_timestamps` は 0.096 / 0.098 / 0.098 秒で、ほかの 5 つは 0.06 秒未満だった。

### 2.3. 手元の計測から分かること

-   メモリの確保量は、同じビルドの中では条件に依らない。どの入力でも、(a)〜(d) の中央の値の差は 1% 未満である。`-race` 付きと `-race` なしの差は、`shortest_form_A_lines` で約 92 倍（1715.92 ÷ 18.67）、`textless_form_B_timestamps` で約 146 倍（1869.03 ÷ 12.81）、`form_A_headings_between_segments` で約 90 倍（590.20 ÷ 6.58）である。ほかの 4 つの入力は、両方のビルドで同じ程度（1 バイトあたり 0〜16 バイト）である。
-   時間は、カバレッジで増える。(c) ÷ (a) の中央の値は、`shortest_form_A_lines` で 1.38、`textless_form_B_timestamps` で 1.41 だが、時間の短い入力では大きく、`trailing_zero-length_lines` で 16.6、`one_long_form_A_line` で 13.6 である。理由は計測していない。仮説としては、`-race` 付きの `-coverprofile` の既定の `covermode=atomic` により、文ごとの計数がアトミックな操作となって検出器を通ることが考えられる。要因の切り分けは結論（AC-04）で扱う。
-   (d) は (c) よりさらに時間がかかる。(d) ÷ (c) の中央の値は、`shortest_form_A_lines` で 1.12、`textless_form_B_timestamps` で 1.09 である。この比は、同じパッケージの先行のテスト（(d) では大きな入力のテストより前に同じプロセスで動く）と、ほかのパッケージとの並行の両方を含む。(b) ÷ (a) も同じである。2 つの切り分けは、CI の `race-cover-package` の計測と結論（AC-04）で行う。
-   `-race` なしでは、全パッケージの並行（(e)）でも単独（参考の値）とほとんど変わらず、最大は `shortest_form_A_lines` の 0.107 秒である。

### 2.4. 計測の間の機械の負荷

1 巡めの (b) の実行中（07:03 ごろ）に `ps` で見たとき、同じ機械では Spotlight の索引作り（`mds_stores`）が CPU を約 270%（約 2.7 コア分）使っていた。ほかの時刻には確かめていない。各実行の開始時の 1 分の負荷の平均は 2.9〜10.1 で、3 巡めの (c)〜(e) は 6.5〜7.2 と高かった。3 巡めの (b)・(d)・(e) では、本タスクの対象外のテスト `TestCommandExecutorDrainsStderr`（`internal/transcript/exec_test.go`。2 秒の時間の上限を持つ。要件書 2.3）が `signal: killed` で失敗し、`internal/transcript` の行が `FAIL` になった。大きな入力のテストは、この 3 回を含むすべての実行で成功し、値をログに出した。(b) と (d) の `shortest_form_A_lines` の最大（4.826 秒・6.635 秒）は、この 3 巡めの値である。

## 3. プロファイル（AC-03）

### 3.1. 取り方

手元（1.1 の環境、`fddb4f7`）で、2 つの入力をサブテストの名前で絞り、`-race` 付きと `-race` なしのそれぞれで取った。メモリのプロファイルは `-memprofilerate=1` で取った。`-memprofilerate=1` の記録の負担は CPU のプロファイルを歪めるので、CPU のプロファイルは別の実行で、`-memprofilerate` を付けずに取った。

```sh
# memory (the -race build adds -race)
GOTOOLCHAIN=go1.26.5 go test -tags test [-race] -count=1 -v \
  -run '^TestParseCopiedLargeInputs$/^<input>$' \
  -cpuprofile <file> -memprofile <file> -memprofilerate=1 -o <binary> ./internal/transcript
# CPU
GOTOOLCHAIN=go1.26.5 go test -tags test [-race] -count=1 -v \
  -run '^TestParseCopiedLargeInputs$/^<input>$' \
  -cpuprofile <file> -o <binary> ./internal/transcript
# read
GOTOOLCHAIN=go1.26.5 go tool pprof -sample_index=alloc_space -focus=parseCopiedTranscript -top <binary> <file>
```

プロファイルのファイルはリポジトリに置いていない。メモリのプロファイルは、`-focus=parseCopiedTranscript` で解析の中の確保だけに絞って読んだ（絞らないと、テストが入力を作る確保も数える）。

### 3.2. メモリの確保量の上位

解析の中の確保量（`-focus=parseCopiedTranscript`）の上位である。量の単位は `go tool pprof` の表示のまま（1 KB = 1024 バイト）とする。割合は、解析の中の確保量の合計に対する、その関数自身の確保量（flat）の割合である。

| 入力 | ビルド | 解析の中の確保量の合計 | 上位の関数（割合） |
|---|---|---|---|
| `shortest_form_A_lines` | `-race` 付き | 6.71 GB | `regexp.(*bitState).reset` 6.58 GB（98.1%）、`parseCopiedFormA` 0.05 GB（0.7%）、`regexp.(*Regexp).backtrack` 0.02 GB（0.3%） |
| `shortest_form_A_lines` | `-race` なし | 74.65 MB | `parseCopiedFormA` 49.39 MB（66.2%。`segments` の `append`、`copied_format.go:512`）、`regexp.(*Regexp).backtrack` 17.45 MB（23.4%。`FindStringSubmatchIndex` が返すスライス）、`strings.genSplit` 5.82 MB（7.8%。行のスライス、`copied_format.go:202`）、`fmt.Sprintf` 1.94 MB（2.6%。ラベルの組み立て） |
| `textless_form_B_timestamps` | `-race` 付き | 7.31 GB | `regexp.(*bitState).reset` 7.22 GB（98.8%）、`regexp.(*Regexp).backtrack` 0.04 GB（0.5%） |
| `textless_form_B_timestamps` | `-race` なし | 51.24 MB | `regexp.(*Regexp).backtrack` 38.40 MB（74.9%）、`strings.genSplit` 12.80 MB（25.0%） |

`-race` なしのビルドでは、`regexp.(*bitState).reset` の確保は、どちらの入力でも 1 回（36 KiB）だけだった。`-race` 付きのビルドでは、`shortest_form_A_lines`（381,300 行）で 190,905 回、`textless_form_B_timestamps`（838,861 行）で 209,806 回だった。確保の回数は、バックトラックの照合の回数の約 4 分の 1 に当たり、`sync.Pool.Put` が `runtime_randn(4) == 0` のときに捨てる割合（`sync/pool.go:104`）と合う。`shortest_form_A_lines` の行（10 バイト）は 2 つの正規表現の最小の入力の長さ以上なので、1 行あたり 2 回照合する（381,300 × 2 ÷ 4 = 190,650。観測 190,905）。`textless_form_B_timestamps` の行（`0:00`、4 バイト）は時・分・秒の正規表現の最小の入力の長さ（7 バイト）より短く、照合の前に除かれる（`regexp/exec.go:527`）ので、1 行あたり 1 回である（838,861 ÷ 4 = 209,715。観測 209,806）。1 回の確保の中身は、`b.visited` の 32 KiB（`regexp/backtrack.go:91`。`-race` 付きの `shortest_form_A_lines` で 5.83 GB）と `b.jobs` の 4 KiB（同 `:84`。0.73 GB）が大半である。

確保はすべて、時刻の文法の照合（`parseTimestampPrefix` の `FindStringSubmatchIndex`、`copied_format.go:225`・`:230`）から来ていた。章の見出しの照合（`copiedIsChapterHeading` の `MatchString`）は、どちらのビルドでも確保の上位に現れなかった。

### 3.3. CPU の時間の上位

テストのプロセス全体の CPU のサンプルである（入力を作る時間を含む）。

| 入力 | ビルド | サブテストの解析の時間 | サンプルの合計 | 上位（割合） |
|---|---|---|---|---|
| `shortest_form_A_lines` | `-race` 付き | 2.76 秒 | 3.65 秒 | `__tsan_read` 29.6%、`__tsan::MemoryAccessRangeT` 18.1%、`runtime.pthread_cond_wait` 12.1%、`runtime.usleep` 7.7%、`runtime.pthread_cond_signal` 6.0%、`runtime.kevent` 4.4%、`__tsan::MemoryRangeSet` 3.8%、`runtime.findObject` 3.0% |
| `shortest_form_A_lines` | `-race` なし | 0.10 秒 | 0.12 秒 | `parseCopiedFormA` 25%（累積 67%）、`runtime.madvise` 17%、ほかは 1 サンプルずつ |
| `textless_form_B_timestamps` | `-race` 付き | 2.92 秒 | 4.17 秒 | `__tsan_read` 46.3%、`runtime.pthread_cond_signal` 9.6%、`runtime.pthread_cond_wait` 8.4%、`runtime.usleep` 7.9%、`runtime.findObject` 7.0%、`__tsan::MemoryRangeSet` 3.4%、`runtime.scanObject`（累積）12.7% |
| `textless_form_B_timestamps` | `-race` なし | 0.12 秒 | 0.12 秒 | `runtime.madvise` 33%、`parseCopiedFormB`（累積）50%、`regexp.(*Regexp).tryBacktrack`（累積）25% |

`-race` 付きのビルドの CPU の時間の大半は、データ競合の検出器の実行時（`__tsan_*`。Go のスタックを持たず、`runtime._System` に数えられる）と、GC（`runtime.findObject`・`runtime.scanObject`）と、それらのスレッドの待ち合わせ（`pthread_cond_*`・`usleep`）である。`runtime._System` の累積は 54.5%（`shortest_form_A_lines`）・50.6%（`textless_form_B_timestamps`）で、`parseCopiedTranscript` の累積は 6.0%・5.5% しか見えない。ただし `__tsan_read` は、計装された読み出しのたびに呼ばれるので、パーサと `regexp` 自身の読み出しの検出器の負担も `runtime._System` に数えられる。このプロファイルからは、検出器の時間のうち、確保（3.2）による分と、読み出しの計装による分を分けられない。`-race` なしのビルドは、サンプルが 12 個しかなく、関数ごとの割合の精度は低い。

### 3.4. H-04 の仮説の成否

**成り立つ。** H-04 の仮説は、`regexp` の照合の状態が `-race` の下では `sync.Pool` で再利用されず、照合のたびに確保されるというものである（設計書 3.6.3）。根拠は次のとおりである。

-   **標準ライブラリ（go1.26.5）:** `regexp` のバックトラックの照合は、照合の状態 `bitState` を `sync.Pool` の `bitStatePool` から取り出し、照合の後に戻す（`regexp/backtrack.go:47-60`）。`sync.Pool.Put` は、`race.Enabled` のとき、戻された値を 4 回に 1 回、無作為に捨てる（`sync/pool.go:103-107`。コメントは「Randomly drop x on floor」）。捨てられた後の照合は `new(bitState)` から始め、`reset` で `b.visited` を、必要な長さに依らず容量 `maxBacktrackVector/visitedBits`（256 × 1024 ÷ 32 = 8192 個の `uint32`、32 KiB）で確保し（`regexp/backtrack.go:33`・`:91`）、`b.jobs` を容量 256 で確保する（同 `:84`）。
-   **プロファイル:** `-race` 付きのビルドでは `bitState.reset` の確保が解析の中の確保量の 98% を占め、確保の回数はバックトラックの照合の回数の約 4 分の 1 に当たる（3.2）。`-race` なしのビルドでは、同じ関数の確保は 1 回だけで、`sync.Pool` による再利用が働いている。
-   **時間:** 確保の仕組みは上の 2 項目で確かめた。`-race` 付きのビルドの CPU の時間は検出器の実行時と GC に費やされている（3.3）が、そのうち確保による分は定量していない。時間の増加が `-race` に固有であること（`-race` なしのビルドでは同じ入力が約 0.1 秒）は、3.3 と 2.2 の値で確かめた。

### 3.5. `-race` 付きの確保量の妥当性

**`-race` 付きのビルドに固有の、妥当な確保量であり、本番の解析を表さない。** 理由は次のとおりである。

-   同じ入力の `-race` なしのビルドの確保量は、1 バイトあたり 18.67（`shortest_form_A_lines`）・12.81（`textless_form_B_timestamps`）である（2.2 の (e) と参考の値）。`-race` 付きの 1 バイトあたり約 1,710〜1,875 バイトとの差の 98% は、3.4 の標準ライブラリの仕組み（`-race` のときだけ `sync.Pool` が値を捨てる）による。パーサのコードは、両方のビルドで同じ回数の照合を行っている。
-   本番の `yt2column-server` は `-race` なしでビルドされる（`Makefile:60-64`）。`-race` なしの解析は、4 MiB 以下の 7 つの合成の入力を手元で 0.12 秒程度以下（計測の実行の最大は 0.113 秒）、確保量 1 バイトあたり 19 バイト以下（`shortest_form_A_lines` で約 75 MB。pprof の単位）で終える。0015 の解析の予算（30 秒未満、目標 1 秒未満）に対して、時間は目標の約 9 分の 1 である。
-   `-race` 付きの確保量の上限（1 バイトあたり 8192。`copied_format_race_test.go:16`）は、観測した最大（1879.72）の約 4.4 倍で、超過していない。

## 4. パーサの改善の見込み

ステップ 3-2 で改善するかを決めるための材料である（実装計画 §1.3）。設計書 3.7 では、条件 1 が崩れたときに人間のレビュアーがこの材料で決める。条件が両方成り立つときは改善しないが、その判断も人間のレビュアーが確かめた（7.2）。プロファイル（3.2・3.3）の上位に現れたパーサの箇所ごとに書く。コードは書いておらず、試作もしていない。効果の値は、プロファイルの値から見積もったもので、計測した値ではない。

### 4.1. 時刻の文法の照合（`parseTimestampPrefix`、`copied_format.go:224-240`）

-   **プロファイルでの位置:** `-race` 付きのビルドの確保量の 98%（3.2）と、時間の大半（3.3 の検出器と GC の時間）の原因である。`-race` なしのビルドでも、確保量の 23%（`shortest_form_A_lines`）・75%（`textless_form_B_timestamps`）を占める。
-   **改善の方法の案:** 2 つの正規表現（`copied_format.go:87-88`）による照合を、バイトを先頭から読む手書きの照合に置き換える。文法は固定の長さの数字とコロンの並びなので、正規表現の機械は要らない。部分一致の位置のスライスも作らない。
-   **見込める効果:** `-race` 付きのビルドでは、`bitState` の確保がなくなるので、確保量は 1 バイトあたり約 1,710〜1,875 から、`-race` なしの値に近い 10〜20 程度に下がり、時間も短くなると見込む。時間の見込みは、確保量が 1 バイトあたり 8 バイト程度の入力の、`-race` 付き ÷ `-race` なしの比（手元の (a) と、`-race` なし・単独の参考の値（2.2）のそれぞれの中央で、`one-byte_prefix_lines` 0.289 ÷ 0.041 ≈ 7.0、`one-byte_form_B_headings` 0.424 ÷ 0.049 ≈ 8.7）を、確保のない場合の検出器の負担と見て求める。`shortest_form_A_lines` の今の比は 2.809 ÷ 0.108 ≈ 26 なので、この比が 7〜9 になれば、手元の `-race` 付きの時間は約 0.8〜0.95 秒、今の約 3 分の 1 になる。読み出しの計装の負担は入力ごとに違うので、この値は目安であり、計測した値ではない。`-race` なしのビルドでは、確保量が `shortest_form_A_lines` で 1 バイトあたり約 4、`textless_form_B_timestamps` で約 10 減り、時間は照合の分（0.1 秒のうち数十ミリ秒）だけ短くなると見込む。本番の解析の時間は、0015 の目標（1 秒）に対してすでに約 9 分の 1 であり、予算の面で得るものは小さい。
-   **AC-14 を保つうえでの危険:** 時刻の文法（時の範囲 1〜99 と分の範囲 0〜59 で先頭の 0 を許さないこと、分と秒が 2 桁であること、`m:ss` の後に `:` が続く行を時刻と読まないこと。`copied_format.go:220-223`・`:234`）を、手書きの照合が正規表現と完全に同じに受け入れ、拒否しなければならない。一致しない入力の位置（`consumed`）も同じでなければならない。食い違うと、受理と拒否の判定、セグメントの `StartMs`、拒否の理由と行番号が変わる。
-   **確かめ方:** 既存の 0015・0024 のテスト（fuzz のテストの種を含む）を変更なしで通す（AC-14）。加えて、正規表現の照合と手書きの照合の結果（受理か、時・分・秒、`consumed`）を任意の文字列で比べる差分の fuzz のテストを、改善の PR の間だけ置いて実行する。

### 4.2. セグメントのスライスの伸長（`parseCopiedFormA`、`copied_format.go:512`）

-   **プロファイルでの位置:** `-race` なしのビルドの `shortest_form_A_lines` で、確保量の 66%（49.39 MB）を占める。`-race` 付きのビルドでは 0.7% である。
-   **改善の方法の案:** `segments` の容量を、本文の行数（`end - start`）を上限として先に確保する。
-   **見込める効果:** `-race` なしのビルドの `shortest_form_A_lines` で、確保量は最終の大きさ（381,300 個 × 24 バイト、約 9 MB）程度になり、1 バイトあたり約 10 減ると見込む。時間の差は小さい。`-race` 付きのビルドの確保量と時間はほとんど変わらない。CI で上限を超えた原因（`-race` 付きの時間）には効かない。
-   **AC-14 を保つうえでの危険:** 返すセグメントの内容と数は変わらない。容量が行数に比例するので、テキストのない行の多い入力では、使わない容量を確保する。危険は小さい。
-   **確かめ方:** 既存のテストを変更なしで通す。

### 4.3. 行の分割（`splitCopiedLines`、`copied_format.go:202`）

-   **プロファイルでの位置:** `-race` なしのビルドで、確保量の 8%（`shortest_form_A_lines`）・25%（`textless_form_B_timestamps`）を占める。
-   **改善の方法の案:** 全行のスライスを作らず、行を順に読む。ただし、末尾の領域の判定（`copiedTrailingStart`）は後ろから行を読むので、行の位置の索引か、後ろから読む別の走査が要る。
-   **見込める効果:** 確保量が 1 バイトあたり 1.5〜3.2 減ると見込む。時間の差は小さい。`-race` 付きのビルドにはほとんど効かない。
-   **AC-14 を保つうえでの危険:** 末尾の領域・前置き・行番号の扱いを書き換えることになり、拒否の行番号と判定の順序が変わる危険が 4.1・4.2 より大きい。
-   **確かめ方:** 既存のテストを変更なしで通す。

### 4.4. まとめ

CI で上限を超えた原因（`-race` 付きのビルドの時間）に効くのは 4.1 だけである。4.1〜4.3 のどれも、本番の解析（`-race` なしのビルド）の時間を、0015 の予算（30 秒）と目標（1 秒）に照らして意味のある分だけ縮めない。

## 5. CI での計測（AC-02・AC-06）

### 5.1. 実行

計測のワークフロー（5.5）を、`origin/main`（`fddb4f7`）から作った一時的なブランチ `isseis/0039-measure-ci` に 1 ファイルだけ加えたコミット `9f4065f` で、push で起動した。5 つの条件を各 5 回、matrix のジョブとして実行し、3 時間あけて実行のし直しで 2 回めを実行した。各条件の標本は 10 である。計測の後、ブランチを `origin` とローカルから削除した。

| 試行 | URL | 開始（UTC） | 結果 |
|---|---|---|---|
| 1 | https://github.com/isseis/yt2column/actions/runs/39915049143/attempts/1 | 2026-10-23 07:00:49 | 25 ジョブすべて成功 |
| 2 | https://github.com/isseis/yt2column/actions/runs/39915049143/attempts/2 | 2026-10-23 10:05:10（`gh run rerun`） | 25 ジョブすべて成功 |

どのジョブの出力にも `internal/transcript` の `(cached)` はなかった。今の上限（15 秒）を超えたサブテストはなかった。

### 5.2. 結果

各欄は 10 回の最小 / 中央 / 最大である。中央は、中ほどの 2 つの値の平均である。時間の単位は秒、メモリの確保量の単位は入力 1 バイトあたりのバイトである。

**`race-cover-alone`（AC-02 (a)：`-race` 付き・カバレッジ付き・単独）**

| 入力 | 回数 | 時間（秒） | 1 バイトあたりの確保量 |
|---|---|---|---|
| `shortest_form_A_lines` | 10 | 4.244 / 7.671 / 8.283 | 1710.72 / 1716.47 / 1722.84 |
| `one_long_form_A_line` | 10 | 0.571 / 0.854 / 0.962 | 0.00 / 0.00 / 0.01 |
| `textless_form_B_timestamps` | 10 | 4.777 / 8.497 / 9.318 | 1864.91 / 1871.12 / 1875.45 |
| `trailing_zero-length_lines` | 10 | 2.802 / 3.718 / 4.553 | 16.01 / 16.02 / 16.03 |
| `one-byte_prefix_lines` | 10 | 2.237 / 3.207 / 3.718 | 8.00 / 8.02 / 8.04 |
| `form_A_headings_between_segments` | 10 | 1.918 / 3.366 / 3.604 | 589.26 / 591.29 / 593.47 |
| `one-byte_form_B_headings` | 10 | 3.713 / 5.505 / 6.118 | 8.00 / 8.01 / 8.03 |

**`race-alone`（AC-02 (b)：`-race` 付き・カバレッジなし・単独）**

| 入力 | 回数 | 時間（秒） | 1 バイトあたりの確保量 |
|---|---|---|---|
| `shortest_form_A_lines` | 10 | 2.736 / 5.160 / 5.316 | 1713.74 / 1717.51 / 1719.09 |
| `one_long_form_A_line` | 10 | 0.028 / 0.056 / 0.066 | 0.00 / 0.00 / 0.00 |
| `textless_form_B_timestamps` | 10 | 2.675 / 5.214 / 5.449 | 1860.30 / 1870.07 / 1874.75 |
| `trailing_zero-length_lines` | 10 | 0.180 / 0.309 / 0.363 | 16.01 / 16.01 / 16.03 |
| `one-byte_prefix_lines` | 10 | 0.215 / 0.387 / 0.406 | 8.00 / 8.02 / 8.04 |
| `form_A_headings_between_segments` | 10 | 1.013 / 2.022 / 2.072 | 588.04 / 589.70 / 592.90 |
| `one-byte_form_B_headings` | 10 | 0.318 / 0.552 / 0.585 | 8.01 / 8.01 / 8.02 |

**`race-cover-package`（AC-04 の切り分け：`internal/transcript` の全テスト、`-race` 付き・カバレッジ付き）**

| 入力 | 回数 | 時間（秒） | 1 バイトあたりの確保量 |
|---|---|---|---|
| `shortest_form_A_lines` | 10 | 4.583 / 7.808 / 8.843 | 1710.27 / 1715.01 / 1722.37 |
| `one_long_form_A_line` | 10 | 0.619 / 0.857 / 1.102 | 0.00 / 0.00 / 0.01 |
| `textless_form_B_timestamps` | 10 | 5.094 / 8.668 / 9.885 | 1864.05 / 1869.06 / 1873.58 |
| `trailing_zero-length_lines` | 10 | 2.921 / 3.760 / 5.122 | 16.00 / 16.02 / 16.03 |
| `one-byte_prefix_lines` | 10 | 2.467 / 3.250 / 4.126 | 8.00 / 8.02 / 8.04 |
| `form_A_headings_between_segments` | 10 | 2.067 / 3.411 / 3.889 | 587.70 / 590.94 / 595.65 |
| `one-byte_form_B_headings` | 10 | 3.975 / 5.455 / 6.726 | 8.01 / 8.01 / 8.02 |

**`ci-equivalent` の 1 つめの実行（AC-02 (c)：`make test-ci` と同じ条件）**

| 入力 | 回数 | 時間（秒） | 1 バイトあたりの確保量 |
|---|---|---|---|
| `shortest_form_A_lines` | 10 | 9.209 / 14.480 / 14.628 | 1711.19 / 1717.66 / 1721.04 |
| `one_long_form_A_line` | 10 | 0.893 / 0.945 / 1.305 | 0.00 / 0.00 / 0.01 |
| `textless_form_B_timestamps` | 10 | 6.762 / 8.628 / 9.422 | 1865.72 / 1868.42 / 1874.35 |
| `trailing_zero-length_lines` | 10 | 2.942 / 3.755 / 4.723 | 16.02 / 16.02 / 16.04 |
| `one-byte_prefix_lines` | 10 | 2.496 / 3.237 / 3.823 | 8.01 / 8.02 / 8.04 |
| `form_A_headings_between_segments` | 10 | 2.584 / 3.430 / 3.653 | 588.75 / 591.58 / 593.94 |
| `one-byte_form_B_headings` | 10 | 4.203 / 5.475 / 6.280 | 8.00 / 8.01 / 8.02 |

**`norace-alone`（AC-02 (d)：`-race` なし・カバレッジなし・単独）**

| 入力 | 回数 | 時間（秒） | 1 バイトあたりの確保量 |
|---|---|---|---|
| `shortest_form_A_lines` | 10 | 0.100 / 0.240 / 0.340 | 18.66 / 18.67 / 18.69 |
| `one_long_form_A_line` | 10 | 0.006 / 0.011 / 0.012 | 0.00 / 0.00 / 0.00 |
| `textless_form_B_timestamps` | 10 | 0.088 / 0.176 / 0.214 | 12.80 / 12.81 / 12.81 |
| `trailing_zero-length_lines` | 10 | 0.041 / 0.071 / 0.098 | 16.00 / 16.01 / 16.01 |
| `one-byte_prefix_lines` | 10 | 0.030 / 0.054 / 0.057 | 8.00 / 8.00 / 8.01 |
| `form_A_headings_between_segments` | 10 | 0.047 / 0.102 / 0.106 | 6.58 / 6.58 / 6.59 |
| `one-byte_form_B_headings` | 10 | 0.042 / 0.080 / 0.086 | 8.00 / 8.00 / 8.01 |

**`ci-equivalent` の 2 つめの実行（AC-06：`test-parse-norace` が行う予定の実行。同じジョブで 1 つめの後に実行）**

| 入力 | 回数 | 時間（秒） | 1 バイトあたりの確保量 |
|---|---|---|---|
| `shortest_form_A_lines` | 10 | 0.138 / 0.221 / 0.254 | 18.66 / 18.67 / 18.68 |
| `one_long_form_A_line` | 10 | 0.007 / 0.011 / 0.012 | 0.00 / 0.00 / 0.00 |
| `textless_form_B_timestamps` | 10 | 0.113 / 0.175 / 0.210 | 12.80 / 12.80 / 12.81 |
| `trailing_zero-length_lines` | 10 | 0.054 / 0.070 / 0.074 | 16.00 / 16.01 / 16.01 |
| `one-byte_prefix_lines` | 10 | 0.041 / 0.054 / 0.058 | 8.00 / 8.00 / 8.01 |
| `form_A_headings_between_segments` | 10 | 0.063 / 0.101 / 0.104 | 6.58 / 6.58 / 6.58 |
| `one-byte_form_B_headings` | 10 | 0.057 / 0.079 / 0.088 | 8.00 / 8.01 / 8.01 |

### 5.3. runner の情報

各ジョブの開始の時刻、CPU の型名、コア数、runner のイメージ、Go の版と、時間の長い 2 つの入力（`shortest_form_A_lines` / `textless_form_B_timestamps`）の時間（秒）である。`ci-equivalent` は、1 つめの実行の値の後に、括弧の中に 2 つめの実行（後段）の値を書く。

| 試行 | ジョブ | 開始 | CPU | コア数 | イメージ | Go | 時間（秒） |
|---|---|---|---|---|---|---|---|
| 1 | `ci-equivalent (1)` | 2026-10-23T07:01:07Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 14.628 / 8.886（後段 0.222 / 0.179） |
| 1 | `ci-equivalent (2)` | 2026-10-23T07:01:40Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 14.569 / 8.636（後段 0.221 / 0.175） |
| 1 | `ci-equivalent (3)` | 2026-10-23T07:01:03Z | INTEL(R) XEON(R) PLATINUM 8573C | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 11.880 / 9.422（後段 0.172 / 0.153） |
| 1 | `ci-equivalent (4)` | 2026-10-23T07:01:05Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 14.536 / 8.792（後段 0.254 / 0.210） |
| 1 | `ci-equivalent (5)` | 2026-10-23T07:01:04Z | AMD EPYC 9V74 80-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 9.209 / 6.806（後段 0.138 / 0.125） |
| 1 | `norace-alone (1)` | 2026-10-23T07:01:24Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 0.224 / 0.178 |
| 1 | `norace-alone (2)` | 2026-10-23T07:01:32Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 0.262 / 0.186 |
| 1 | `norace-alone (3)` | 2026-10-23T07:01:04Z | Intel(R) Xeon(R) Platinum 8370C CPU @ 2.80GHz | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 0.196 / 0.172 |
| 1 | `norace-alone (4)` | 2026-10-23T07:01:01Z | AMD EPYC 9V45 96-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 0.100 / 0.089 |
| 1 | `norace-alone (5)` | 2026-10-23T07:01:03Z | AMD EPYC 9V45 96-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 0.100 / 0.088 |
| 1 | `race-alone (1)` | 2026-10-23T07:01:04Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 5.152 / 5.357 |
| 1 | `race-alone (2)` | 2026-10-23T07:01:05Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 5.316 / 5.449 |
| 1 | `race-alone (3)` | 2026-10-23T07:01:05Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 5.065 / 5.203 |
| 1 | `race-alone (4)` | 2026-10-23T07:01:03Z | AMD EPYC 9V45 96-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 2.754 / 2.675 |
| 1 | `race-alone (5)` | 2026-10-23T07:01:05Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 5.167 / 5.179 |
| 1 | `race-cover-alone (1)` | 2026-10-23T07:01:05Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 7.696 / 8.519 |
| 1 | `race-cover-alone (2)` | 2026-10-23T07:01:04Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 7.726 / 8.475 |
| 1 | `race-cover-alone (3)` | 2026-10-23T07:01:04Z | AMD EPYC 9V45 96-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 4.361 / 4.816 |
| 1 | `race-cover-alone (4)` | 2026-10-23T07:01:03Z | Intel(R) Xeon(R) Platinum 8370C CPU @ 2.80GHz | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 8.278 / 9.318 |
| 1 | `race-cover-alone (5)` | 2026-10-23T07:01:24Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 7.646 / 8.545 |
| 1 | `race-cover-package (1)` | 2026-10-23T07:01:05Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 7.837 / 8.749 |
| 1 | `race-cover-package (2)` | 2026-10-23T07:01:04Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 8.231 / 8.873 |
| 1 | `race-cover-package (3)` | 2026-10-23T07:01:38Z | AMD EPYC 9V74 80-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 5.930 / 6.528 |
| 1 | `race-cover-package (4)` | 2026-10-23T07:01:06Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 7.755 / 8.587 |
| 1 | `race-cover-package (5)` | 2026-10-23T07:01:02Z | AMD EPYC 9V45 96-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 4.597 / 5.266 |
| 2 | `ci-equivalent (1)` | 2026-10-23T10:05:26Z | AMD EPYC 9V74 80-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 9.378 / 6.762（後段 0.139 / 0.126） |
| 2 | `ci-equivalent (2)` | 2026-10-23T10:05:29Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 14.424 / 8.619（後段 0.220 / 0.182） |
| 2 | `ci-equivalent (3)` | 2026-10-23T10:05:27Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 14.628 / 8.661（後段 0.248 / 0.180） |
| 2 | `ci-equivalent (4)` | 2026-10-23T10:05:26Z | Intel(R) Xeon(R) 6973P-C | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 9.372 / 8.277（後段 0.140 / 0.113） |
| 2 | `ci-equivalent (5)` | 2026-10-23T10:05:28Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 14.607 / 8.572（後段 0.232 / 0.175） |
| 2 | `norace-alone (1)` | 2026-10-23T10:05:29Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 0.340 / 0.214 |
| 2 | `norace-alone (2)` | 2026-10-23T10:05:30Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 0.264 / 0.191 |
| 2 | `norace-alone (3)` | 2026-10-23T10:06:00Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 0.237 / 0.175 |
| 2 | `norace-alone (4)` | 2026-10-23T10:05:29Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 0.243 / 0.175 |
| 2 | `norace-alone (5)` | 2026-10-23T10:05:30Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 0.252 / 0.177 |
| 2 | `race-alone (1)` | 2026-10-23T10:05:26Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 4.971 / 5.138 |
| 2 | `race-alone (2)` | 2026-10-23T10:05:53Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 5.222 / 5.429 |
| 2 | `race-alone (3)` | 2026-10-23T10:05:30Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 5.248 / 5.293 |
| 2 | `race-alone (4)` | 2026-10-23T10:05:28Z | AMD EPYC 9V45 96-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 2.736 / 2.865 |
| 2 | `race-alone (5)` | 2026-10-23T10:05:53Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 5.171 / 5.225 |
| 2 | `race-cover-alone (1)` | 2026-10-23T10:05:30Z | Intel(R) Xeon(R) Platinum 8370C CPU @ 2.80GHz | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 8.283 / 9.310 |
| 2 | `race-cover-alone (2)` | 2026-10-23T10:05:26Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 7.508 / 8.390 |
| 2 | `race-cover-alone (3)` | 2026-10-23T10:05:25Z | AMD EPYC 9V45 96-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 4.244 / 4.777 |
| 2 | `race-cover-alone (4)` | 2026-10-23T10:05:30Z | AMD EPYC 9V74 80-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 5.864 / 6.474 |
| 2 | `race-cover-alone (5)` | 2026-10-23T10:05:30Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 7.699 / 8.614 |
| 2 | `race-cover-package (1)` | 2026-10-23T10:05:26Z | AMD EPYC 9V45 96-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 4.583 / 5.094 |
| 2 | `race-cover-package (2)` | 2026-10-23T10:05:50Z | Intel(R) Xeon(R) Platinum 8370C CPU @ 2.80GHz | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 8.431 / 9.614 |
| 2 | `race-cover-package (3)` | 2026-10-23T10:05:26Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 7.961 / 8.760 |
| 2 | `race-cover-package (4)` | 2026-10-23T10:05:47Z | INTEL(R) XEON(R) PLATINUM 8573C | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 8.843 / 9.885 |
| 2 | `race-cover-package (5)` | 2026-10-23T10:05:28Z | AMD EPYC 7763 64-Core Processor | 4 | ubuntu24 20261012.334.1 | go1.26.5 | 7.780 / 8.580 |

CPU の型名は 6 種類あった（AMD EPYC 7763・9V45・9V74、Intel Xeon Platinum 8370C・Platinum 8573C・6973P-C）。50 ジョブのうち 31 が AMD EPYC 7763 である。同じ条件の値のばらつきの大半は、CPU の型の違いによる。たとえば `race-alone` の `shortest_form_A_lines` は、AMD EPYC 7763 で 4.971〜5.316 秒、AMD EPYC 9V45 で 2.736〜2.754 秒である。

### 5.4. AC-06 の値

AC-06 の値は、`ci-equivalent` の 2 つめの実行（5.2 の最後の表）である。7 つの入力の全計測（70 個）を通した最大は `shortest_form_A_lines` の 0.254 秒（試行 1 の `ci-equivalent (4)`、AMD EPYC 7763）である。同じ入力の最小は 0.138 秒（試行 1 の `ci-equivalent (5)`、AMD EPYC 9V74）である。

### 5.5. 計測のワークフローの全文

ブランチ `isseis/0039-measure-ci` の `.github/workflows/measure-0039.yml`（コミット `9f4065f`）の全文である。`actions/checkout`・`actions/setup-go` の版と `go-version-file: go.mod`・`cache: true` は `ci.yml:61-68` に揃えた。

```yaml
name: Measure 0039

# Temporary measurement workflow for task 0039 (large-input parse test
# timeout). It lives only on its own branch and is never merged.
on:
  push:
    branches: [ isseis/0039-measure-ci ]

permissions:
  contents: read

jobs:
  measure:
    name: ${{ matrix.condition }} (${{ matrix.run }})
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        condition: [ race-cover-alone, race-alone, race-cover-package, ci-equivalent, norace-alone ]
        run: [ 1, 2, 3, 4, 5 ]
    steps:
      - name: Checkout code
        uses: actions/checkout@v5

      - name: Setup Go
        uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
          cache: true

      - name: Runner info
        run: |
          echo "start: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
          echo "cpu: $(grep -m 1 'model name' /proc/cpuinfo | cut -d: -f2- | sed 's/^ *//')"
          echo "cores: $(nproc)"
          echo "image: ${ImageOS:-unknown} ${ImageVersion:-unknown}"
          go version

      - name: Measure (race, coverage, alone)
        if: matrix.condition == 'race-cover-alone'
        run: go test -tags test -race -count=1 -v -coverprofile=coverage.out -run '^TestParseCopiedLargeInputs$' ./internal/transcript

      - name: Measure (race, alone)
        if: matrix.condition == 'race-alone'
        run: go test -tags test -race -count=1 -v -run '^TestParseCopiedLargeInputs$' ./internal/transcript

      - name: Measure (race, coverage, whole package)
        if: matrix.condition == 'race-cover-package'
        run: go test -tags test -race -count=1 -v -coverprofile=coverage.out ./internal/transcript

      - name: Measure (make test-ci equivalent)
        if: matrix.condition == 'ci-equivalent'
        run: go test -tags test -race -count=1 -v -coverprofile=coverage.out ./...

      - name: Measure (planned test-parse-norace, after test-ci)
        if: ${{ !cancelled() && matrix.condition == 'ci-equivalent' }}
        run: go test -tags test -count=1 -v -timeout 5m -run '^TestParseCopiedLargeInputs$' ./internal/transcript

      - name: Measure (no race, alone)
        if: matrix.condition == 'norace-alone'
        run: go test -tags test -count=1 -v -run '^TestParseCopiedLargeInputs$' ./internal/transcript
```

## 6. 結論（AC-04）

### 6.1. 要因ごとの比

設計書 3.6.4 の比を、入力ごとの中央の値で求めた。「手元 (a)」は 2.2 の (a) である。

| 入力 | パーサの処理：AC-02 (d) の中央（最小〜最大）と目標 1 秒 | `-race` の負担：(b) ÷ (d) | カバレッジの負担：(a) ÷ (b) | runner の速さ：(b) ÷ 手元 (a) | 先行のテスト：`race-cover-package` ÷ (a) | 並行の負荷：(c) ÷ `race-cover-package` |
|---|---|---|---|---|---|---|
| `shortest_form_A_lines` | 0.240（0.100〜0.340）秒 | 21.50 | 1.49 | 1.84 | 1.02 | 1.85 |
| `one_long_form_A_line` | 0.011（0.006〜0.012）秒 | 5.09 | 15.25 | 1.75 | 1.00 | 1.10 |
| `textless_form_B_timestamps` | 0.176（0.088〜0.214）秒 | 29.63 | 1.63 | 1.72 | 1.02 | 1.00 |
| `trailing_zero-length_lines` | 0.071（0.041〜0.098）秒 | 4.35 | 12.03 | 2.81 | 1.01 | 1.00 |
| `one-byte_prefix_lines` | 0.054（0.030〜0.057）秒 | 7.17 | 8.29 | 1.34 | 1.01 | 1.00 |
| `form_A_headings_between_segments` | 0.102（0.047〜0.106）秒 | 19.82 | 1.66 | 1.75 | 1.01 | 1.01 |
| `one-byte_form_B_headings` | 0.080（0.042〜0.086）秒 | 6.90 | 9.97 | 1.30 | 0.99 | 1.00 |

ばらつき（各条件の最小と最大）は 5.2 と 2.2 の表のとおりである。最小と最大の差は、主に runner の CPU の型の違いによる（5.3）。CPU の型が混ざった中央の値は、どの条件でも AMD EPYC 7763 のジョブの値で決まる（50 ジョブのうち 31、`race-alone` では 10 のうち 8）。AMD EPYC 7763 のジョブだけで求めると、`shortest_form_A_lines` の `-race` の負担は約 20.5、カバレッジの負担は約 1.49 で、上の表とほぼ同じである。CPU の型をそろえた比として、AMD EPYC 7763 のジョブだけで `shortest_form_A_lines` を比べると、`ci-equivalent` の 1 つめの実行（6 ジョブ）は 14.424〜14.628 秒（中央 14.588）、`race-cover-package`（5 ジョブ）は 7.755〜8.231 秒（中央 7.837）で、並行の比は 1.86 である。同じ CPU の `textless_form_B_timestamps` では、8.572〜8.886 秒（中央 8.648）と 8.580〜8.873 秒（中央 8.749）で、比は 0.99 である。

### 6.2. 結論

PR #461 で上限を超えたのは、`-race` 付き・カバレッジ付きのビルドを、ほかのパッケージと並行に実行したときの `shortest_form_A_lines` である。計測した各要因の効き方は次のとおりである。

-   **パーサの処理:** 効いていない。`-race` なし・単独（AC-02 (d)）の全計測の最大は 0.340 秒、中央は 0.240 秒（`shortest_form_A_lines`）で、0015 の目標（1 秒）を満たす。
-   **`-race` の負担:** 最も大きい要因である。`shortest_form_A_lines` で 21.5 倍、`textless_form_B_timestamps` で 29.6 倍、`form_A_headings_between_segments` で 19.8 倍である。この 3 つの入力は、`-race` 付きの確保量が大きい入力と一致する（3.2・3.4）。ほかの 4 つの入力は 4.4〜7.2 倍である。
-   **カバレッジの負担:** 1.49〜1.66 倍（時間の長い 3 つの入力）。時間の短い入力では 8〜15 倍だが、絶対の時間は 6.2 秒以下である。
-   **CI の runner の速さ:** 手元の約 1.7〜1.8 倍遅い（時間の長い 3 つの入力の中央）。ただし runner の CPU の型で約 2 倍違い、AMD EPYC 9V45 は手元とほぼ同じ速さ、AMD EPYC 7763 は手元の約 1.7〜1.9 倍遅い（`race-alone` の `shortest_form_A_lines` と `textless_form_B_timestamps`。5.3）。
-   **同じパッケージの先行のテストの影響:** ほとんどない（0.99〜1.02 倍）。
-   **CI での並行の負荷:** `shortest_form_A_lines` だけに効き、1.85 倍（AMD EPYC 7763 のジョブだけで 1.86 倍）である。ほかの入力は 1.00〜1.10 倍である。`shortest_form_A_lines` は大きな入力のテストの最初のサブテストなので、ほかのパッケージのテストがまだ動いている時間に当たると見られるが、今回の計測では確かめていない。

AMD EPYC 7763 の runner では、`make test-ci` と同じ条件の `shortest_form_A_lines` が 14.424〜14.628 秒で、今の上限（15 秒）まで 0.4〜0.6 秒しか余裕がない。中央の値の積（0.240 × 21.5 × 1.49 × 1.85 ≈ 14.2 秒）も、この値と合う。計測した 10 回の `make test-ci` と同じ条件の実行（`ci-equivalent` の 1 つめの実行）は、どれも上限を超えず、最大は 14.628 秒だった。PR #461 の失敗の値（15.02〜15.44 秒）は計測の最大より 2.7〜5.5% 長く、その差は説明できていない。PR #461 の runner の CPU の型は確かめていない。並行の負荷はほかのパッケージのテストとの重なり方に依るので、コミットのテストの構成で変わりうるが、これも確かめていない。結論として、上限を超えた原因は、`-race`・カバレッジ・runner の速さ・並行の負荷の積が上限に近づいたことであり、パーサの処理の劣化ではない。失敗そのものは再現していない。

## 7. パーサの改善の判断（AC-05・AC-13）

### 7.1. 設計書 3.7 の条件の判定

| 条件 | 判定 | 根拠 |
|---|---|---|
| 条件 1: `-race` 付きのビルドのメモリの確保量と時間の増加が、`-race` に固有の仕組みによる（`-race` なしのビルドには現れない） | 成り立つ | 確保量の増加は、`-race` のときだけ `sync.Pool` が値を捨てる仕組み（3.4）によることを、標準ライブラリの箇所と確保の回数で確かめた。`-race` なしのビルドでは同じ確保は 1 回だけである。時間の増加は、検出器の実行時と GC に費やされ（3.3）、`-race` なしのビルドでは同じ入力が CI で 0.340 秒以下、手元で 0.113 秒以下である（2.2・5.2）。設計書 3.7 は条件 1 を「`-race` なしのビルドには現れない」と定めており、時間についてもそれを確かめた。時間のうち確保による分と読み出しの計装による分は分けていない（3.4）が、どちらも `-race` に固有の仕組みなので、この分け方は条件 1 の判定に要らない。加えて、`-race` の負担は、`bitState` の確保の大きい 3 つの入力で 19.8〜29.6 倍、確保の小さい 4 つの入力で 4.4〜7.2 倍であり（6.1）、確保の増加と時間の増加は対応している |
| 条件 2: AC-02 (d) の全計測の最大が 1 秒未満 | 成り立つ | 最大は 0.340 秒（試行 2 の `norace-alone (1)`、`shortest_form_A_lines`、AMD EPYC 7763） |

AC-05 (a) には当たらない（条件 2 が成り立つ）。AC-05 (b) にも当たらない（`-race` なしのビルドにも現れる劣化は原因に含まれない。6.2）。F-002 に進む。

### 7.2. 判断と理由（AC-13）

**パーサを改善しない。** 理由は次のとおりである。

-   `CLAUDE.md`「Performance」は、最適化を実際の実行の絶対的な予算で正当化するよう求める。本番の `yt2column-server` は `-race` なしでビルドされ（`Makefile:60-64`）、その解析は、4 MiB 以下の 7 つの合成の入力を CI の runner で 0.340 秒以下、手元で 0.113 秒以下で終える。0015 の解析の予算（30 秒未満）の約 88 分の 1、目標（1 秒未満）の約 3 分の 1 である。
-   CI で上限を超えた原因は `-race` 付きのビルドの時間であり（6.2）、本番の解析の時間ではない。時間の確認を `-race` なしのビルドに移せば（F-002）、この原因は確認から外れる。
-   4.1 の改善は `-race` 付きの時間を約 3 分の 1 にすると見込むが、本番の時間はほとんど変わらない（4.4）。一方で、時刻の文法を正規表現と同じに受け入れ、拒否することを新たに保証する負担（AC-14）が加わる。

このため、フェーズ 4（パーサの改善）は行わず、PR-2 は作らない。この判断は、2026-10-23 に人間のレビュアー（isseis）が `/runplan` のセッションで確かめた。

## 8. 時間の上限（AC-08）

### 8.1. 上限の値

設計書 3.5 の規則（AC-06 の全計測を通した最大を 10 倍し、0.5 秒の単位に切り上げる）で求めた。

| 項目 | 値 |
|---|---|
| AC-06 の最大の時間 | 0.254 秒（`shortest_form_A_lines`。5.4） |
| AC-06 の最小の時間（同じ入力） | 0.138 秒（5.4）。最大 ÷ 最小 = 1.84 で、3 を超えない |
| 規則で求めた上限 | 0.254 × 10 = 2.54 秒を 0.5 秒の単位に切り上げて **3.0 秒** |
| 上限の倍率（上限 ÷ AC-06 の最大） | 11.8 |
| 上限と 0015 の目標（1 秒）との比 | 3.0 |
| 上限と 0015 の予算（30 秒）との比 | 0.10（上限は予算の 10 分の 1 で、30 秒未満を満たす） |

**AC-02 (d) の値との関係:** AC-02 (d)（`norace-alone`）は、AC-06 と同じ `go test` を（`-timeout` を除いて）同じ引数で単独に実行した値である。その最大は 0.340 秒（試行 2 の `norace-alone (1)`、AMD EPYC 7763）で、AC-06 の最大より 34% 長い。同じ CPU のほかの `norace-alone` の 6 回は 0.224〜0.264 秒である。上限 3.0 秒はこの 0.340 秒の 8.8 倍である。AC-06 と AC-02 (d) の 20 回を合わせると、最大 ÷ 最小は 0.340 ÷ 0.100 = 3.4 で、設計書 3.5 の揺れの条件（3）を超える。最小の 0.100 秒は AMD EPYC 9V45 の値で、AC-06 の 10 回にはこの CPU がない。合わせたときの揺れは、主に CPU の型の差による。設計書 3.5 の規則は AC-06 の値だけを使うと定めているので、規則どおり 3.0 秒とした。この扱いは、2026-10-23 に人間のレビュアー（isseis）が `/runplan` のセッションで選んだ。

### 8.2. 手元の条件との比

-   手元の pre-commit のフックと同じ条件（2.2 の (e)）の最大は 0.107 秒で、上限の約 28 分の 1 である。上限を超えない。
-   手元の `-race` なし・単独（2.2 の参考の値）の最大は 0.113 秒で、上限の約 27 分の 1 である。
-   AC-02 (d) の最大（0.340 秒、5.2）は上限の約 9 分の 1 である。

### 8.3. この上限が検出するもの

設計書 3.5「この上限が検出するもの」のとおり、上限は 7 つの入力に共通の 1 つの値で、最も遅い入力の AC-06 の最大の約 12 倍である。最も遅い入力では、時間が上限に届くまでの倍率より小さい劣化は検出しない。その倍率は runner によって違い、計測した値に対して約 8.8 倍（AC-02 (d) の 0.340 秒）から約 21.7 倍（AC-06 の最小の 0.138 秒）である。検出するのは、入力の長さの 2 乗に比例するような、桁の違う劣化である。上限は 0015 の目標（1 秒）を超えるが、目標の達成は、AC-02 (d)・AC-06 の値として本書に残る。
