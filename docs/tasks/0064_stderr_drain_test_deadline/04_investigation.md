# 調査の記録：標準エラーの読み出しのテストが負荷の下で遅くなる原因

本書は、タスク 0064 の調査の記録である（[02_architecture.md](02_architecture.md)（以下「設計書」）3.5）。[01_requirements.md](01_requirements.md)（以下「要件書」）の AC-04・AC-06・AC-07 の記録を置く。承認ではなく PR のレビューで中身を確かめる文書なので、`Document Status` を持たない。用語は要件書 6 章と設計書の冒頭の用語（対象のテスト、計測用のテスト、race の実行の区間）を同じ意味で使う。時刻はすべて UTC で書く。

## 1. 計測の環境

| 項目 | 値 |
|---|---|
| OS | macOS 26.6.2（25G83） |
| CPU とコア数 | Apple M3 Pro、12 コア（性能コア 6、効率コア 6。`sysctl hw.ncpu hw.perflevel0.physicalcpu hw.perflevel1.physicalcpu`） |
| メモリ | 18 GiB（`sysctl -n hw.memsize` が `19327352832`） |
| Go の版 | go1.27.1 darwin/arm64（手元の既定。`GOTOOLCHAIN` は指定しない。設計書 1.2） |
| 計測したコミット | `c6361e16`（`main`。対象のテストの期限を変える前） |
| 計測した日時 | 2026-11-05。区間の長さの計測（2.1）は 22:00〜22:04、(a)・(b) は 23:39:47〜23:43:00、(c)・(d) は 23:43:46〜23:51:50 |

## 2. AC-06 の計測

### 2.1. race の実行の区間の長さと、回と回の間隔

ビルドのキャッシュを温め（2.3）、`go clean -testcache` を実行してから、`make test` を 1 回、出力の行ごとに時刻を付けて実行した。race の実行の区間は 22:01:00.161（最初の `=== RUN` の行）から 22:03:23.281（`test-parse-norace` のコマンドの行）までの 143.1 秒だった。`make test` は成功し、`(cached)` の行はなかった。

実装計画のステップ 1-3 の規則では、60 回の計測を 1 回の区間の 8 割（114.5 秒）に収める間隔はない（1 回を最大 2 秒と見込むと、間隔が 0 秒でも 120 秒）。そこで、人間のレビュアーの決定により、(c)・(d) を 3 回の `make test` の区間に分けて、1 回の区間で 20 回（(c)・(d) を交互に 10 回ずつ）計測することにした（設計書 1.4）。20 回 ×（2 秒 + 間隔）≤ 114.5 秒を満たす 3 秒以下で最も長い間隔は 3 秒なので、間隔は 3 秒とした。(a)・(b) も同じ 3 秒の間隔で計測した。

### 2.2. 計測用のテストのソース

`internal/transcript/measure0064_test.go` に一時的に置いた。計測の後に消し、どのコミットにも入れていない（`git status --short` に残っていないことを確かめた）。`-tags test` だけのビルドではビルドされないことを、`go test -tags test -list '.*' ./internal/transcript` の出力に `TestMeasure0064` がないことで確かめた（`-tags 'test measure0064'` では出る）。

```go
//go:build test && measure0064

package transcript

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestMeasure0064 times the child run of TestCommandExecutorDrainsStderr under
// two alternating conditions: a script written as a new file for every run
// (new path and new content), and one script file that has already been run
// once. Each run is timed from just before osExecutor.Run to just after it
// returns, which covers the request to start the child, any wait before it
// runs, and the parent receiving its exit.
//
// MEASURE0064_LABELS names the two conditions ("a,b" or "c,d"),
// MEASURE0064_RUNS is the number of runs of each condition, and
// MEASURE0064_INTERVAL is the pause after every run.
func TestMeasure0064(t *testing.T) {
	const (
		burst    = 256 << 10
		deadline = 60 * time.Second
	)
	freshLabel, reusedLabel, ok := strings.Cut(os.Getenv("MEASURE0064_LABELS"), ",")
	if !ok {
		t.Fatal("MEASURE0064_LABELS must be two labels separated by a comma")
	}
	runs, err := strconv.Atoi(os.Getenv("MEASURE0064_RUNS"))
	if err != nil || runs < 1 {
		t.Fatalf("MEASURE0064_RUNS must be a positive integer: %q", os.Getenv("MEASURE0064_RUNS"))
	}
	interval, err := time.ParseDuration(os.Getenv("MEASURE0064_INTERVAL"))
	if err != nil {
		t.Fatalf("MEASURE0064_INTERVAL: %v", err)
	}
	// Same content as the script of TestCommandExecutorDrainsStderr.
	base := "#!/bin/sh\nprintf '%s' '" + strings.Repeat("x", burst) + "' >&2\nexit 3\n"

	runOnce := func(path string) (time.Time, time.Duration, error) {
		ctx, cancel := context.WithTimeout(t.Context(), deadline)
		defer cancel()
		var stderr cappedWriter
		start := time.Now()
		err := osExecutor{}.Run(ctx, path, nil, allowlistEnv(os.Environ()), nil, &stderr)
		return start, time.Since(start), err
	}

	// The reused script is written and run once before the measured runs;
	// that run is not counted.
	reused := writeHelperScript(t, base)
	if _, _, err := runOnce(reused); err == nil {
		t.Fatal("warm-up run error = nil, want exit code 3")
	}

	for i := 1; i <= runs; i++ {
		for _, fresh := range []bool{true, false} {
			label, path := reusedLabel, reused
			if fresh {
				// A new directory per run (t.TempDir) and a comment line unique
				// to the run, so the file is new by path and by content.
				label = freshLabel
				path = writeHelperScript(t, base+fmt.Sprintf("# run %d %d\n", i, time.Now().UnixNano()))
			}
			start, elapsed, err := runOnce(path)
			exitCode := -1
			if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
				exitCode = exitErr.ExitCode()
			}
			t.Logf("MEASURE cond=%s run=%d start=%s elapsed=%.3f exit=%d err=%v",
				label, i, start.UTC().Format("15:04:05.000"), elapsed.Seconds(), exitCode, err)
			time.Sleep(interval)
		}
	}
}
```

### 2.3. 実行したコマンド

どれもリポジトリのルートで実行した。

ビルドのキャッシュを温めるコマンド（計測の前に実行した）:

```bash
go test -tags 'test measure0064' -race -count=1 -run '^$' ./internal/transcript
go test -tags test -race -count=1 -run '^$' ./...
```

計測のコマンド。`-timeout 90m` は、1 回の `go test` の回がすべて 60 秒の期限に達しても足りる値である。

| 条件 | コマンド |
|---|---|
| (a)・(b) | `MEASURE0064_LABELS=a,b MEASURE0064_RUNS=30 MEASURE0064_INTERVAL=3s go test -tags 'test measure0064' -race -count=1 -v -timeout 90m -run '^TestMeasure0064$' ./internal/transcript` |
| (c)・(d)（3 回） | `MEASURE0064_LABELS=c,d MEASURE0064_RUNS=10 MEASURE0064_INTERVAL=3s go test -tags 'test measure0064' -race -count=1 -v -timeout 90m -run '^TestMeasure0064$' ./internal/transcript` を、次の `concurrent.sh` から実行した |

行ごとの時刻を付けるスクリプト（`ts.sh`）:

```sh
#!/bin/sh
# Prefix every stdin line with a UTC timestamp in milliseconds.
exec /usr/bin/perl -MTime::HiRes=time -MPOSIX=strftime -ne 'BEGIN{$|=1} $t=time; printf "%s.%03d %s", strftime("%H:%M:%S", gmtime($t)), int(($t-int($t))*1000), $_'
```

負荷の指標を 1 秒ごとに採るスクリプト（`sample.sh`）。`ps` の行は、名前に `xprotect`、`syspolicyd`、`amfid` を含むプロセス（大文字と小文字を区別しない）に絞った。そのとき動いていないプロセスは出ない:

```sh
#!/bin/sh
# Every second: UTC time, 1-minute load average, CPU of scan-related processes.
while :; do
  now=$(/usr/bin/perl -MTime::HiRes=time -MPOSIX=strftime -e '$t=time; printf "%s.%03d", strftime("%H:%M:%S", gmtime($t)), int(($t-int($t))*1000)')
  la=$(sysctl -n vm.loadavg | awk '{print $2}')
  procs=$(ps -Ao %cpu,comm | awk 'tolower($0) ~ /xprotect|syspolicyd|amfid/ {n=$2; sub(".*/","",n); printf "%s=%s ", n, $1}')
  echo "$now load1=$la $procs"
  sleep 1
done
```

`make test` と並行させるスクリプト（`concurrent.sh`。設計書 6.2 の手順）。リポジトリのルートに移る `cd` の行は、手元の絶対パスを環境変数 `REPO` に置き換えて載せた。ほかの行は実行したものと同じである:

```bash
#!/bin/bash
# Usage: concurrent.sh <out-prefix> <command...>
# Runs `make test` (after go clean -testcache) with timestamped output, starts
# <command> once the first "=== RUN" line appears, and samples load meanwhile.
S=$(dirname "$0"); P=$1; shift
cd "${REPO:?set REPO to the repository root}" || exit 1
"$S/sample.sh" > "$P.load" 2>&1 & SAMP=$!
go clean -testcache
( make test 2>&1 | "$S/ts.sh" > "$P.mt"; echo "${PIPESTATUS[0]}" > "$P.mtexit"; date -u +%H:%M:%S.%N | cut -c1-12 > "$P.mtend" ) & MT=$!
until grep -q '=== RUN' "$P.mt" 2>/dev/null; do sleep 0.2; done
date -u +%H:%M:%S.%N | cut -c1-12 > "$P.mstart"
( "$@" ) 2>&1 | "$S/ts.sh" > "$P.m"; echo "${PIPESTATUS[0]}" > "$P.mexit"
date -u +%H:%M:%S.%N | cut -c1-12 > "$P.mend"
wait $MT
kill $SAMP
echo done
```

### 2.4. 結果

時間の単位は秒である。どの回も、終了コード 3 の `*exec.ExitError`（`exit status 3`）で終わった。

| 条件 | 回数 | 最小 | 中央値 | 最大 | 1 秒を超えた回 | 2 秒を超えた回 |
|---|---|---|---|---|---|---|
| (a) 新しいファイル・並行しない | 30 | 0.330 | 0.367 | 0.387 | 0 | 0 |
| (b) 同じファイル・並行しない | 30 | 0.016 | 0.020 | 0.042 | 0 | 0 |
| (c) 新しいファイル・`make test` と並行 | 30 | 0.317 | 0.340 | 2.144 | 8 | 1 |
| (d) 同じファイル・`make test` と並行 | 30 | 0.015 | 0.018 | 0.170 | 0 | 0 |

(c)・(d) の区間ごとの内訳:

| 区間 | (c) 最小 / 中央値 / 最大 | (c) 1 秒を超えた回 | (d) 最小 / 中央値 / 最大 |
|---|---|---|---|
| 1 | 0.317 / 0.330 / 1.593 | 2 | 0.015 / 0.017 / 0.039 |
| 2 | 0.321 / 0.344 / 1.725 | 3 | 0.015 / 0.020 / 0.170 |
| 3 | 0.327 / 0.413 / 2.144 | 3 | 0.015 / 0.018 / 0.110 |

30 秒を超えた回（60 秒の期限が来た回を含む）はなかった。

### 2.5. 負荷の指標

各条件の回ごとに、その回の中ほどの時刻に最も近い採取の値を取り、条件ごとにまとめた。`XprotectService` の値は、採取の時刻に動いていなかった回を 0 として平均した。CPU 使用率は 1 コアを 100% とする `ps` の値である。

| 条件 | 1 分間の負荷平均（最小 / 平均 / 最大） | `XprotectService`（平均 / 最大） | `syspolicyd`（平均 / 最大） |
|---|---|---|---|
| (a) | 1.57 / 2.09 / 2.66 | 1.3 / 28.6 | 4.9 / 16.5 |
| (b) | 1.61 / 2.07 / 2.66 | 0.0 / 0.0 | 2.6 / 16.9 |
| (c) | 4.85 / 10.08 / 14.80 | 33.9 / 71.5 | 7.1 / 15.5 |
| (d) | 6.06 / 10.32 / 14.26 | 26.1 / 65.2 | 5.6 / 17.6 |

`XProtectBehaviorService`、`XProtectPluginService` は、どの採取にも現れなかった。設計書 3.3.2 の一覧にない `XProtectBridgeService` が、(c)・(d) の区間 3 の採取にだけ現れた（161 回のうち 91 回。最大 6.2%）。`xprotectd` と `amfid` は、どの採取でも 2% 以下だった。

1 秒を超えた回（すべて (c)）と、その時刻の負荷の指標:

| 区間 | 回 | 開始の時刻 | 時間 | 採取の時刻 | 負荷平均 | `XprotectService` | `syspolicyd` |
|---|---|---|---|---|---|---|---|
| 1 | 1 | 23:43:49.688 | 1.593 | 23:43:50.647 | 4.85 | 65.3 | 15.3 |
| 1 | 2 | 23:43:57.309 | 1.369 | 23:43:58.223 | 6.68 | 51.6 | 4.6 |
| 2 | 1 | 23:46:18.994 | 1.725 | 23:46:20.303 | 7.51 | 71.5 | 9.3 |
| 2 | 2 | 23:46:26.742 | 1.295 | 23:46:27.800 | 9.61 | 53.3 | 8.6 |
| 2 | 8 | 23:47:06.121 | 1.576 | 23:47:07.333 | 11.16 | 54.3 | 11.6 |
| 3 | 1 | 23:49:15.726 | 2.144 | 23:49:16.465 | 10.04 | 46.6 | 8.3 |
| 3 | 2 | 23:49:23.991 | 1.538 | 23:49:25.100 | 14.80 | 31.0 | 4.1 |
| 3 | 3 | 23:49:31.550 | 1.952 | 23:49:32.587 | 14.26 | 53.8 | 10.9 |

### 2.6. 並行した `make test`

ビルドのキャッシュは、2.3 のコマンドで温めた。各区間の前に `go clean -testcache` を実行した。

| 区間 | race の実行の区間 | 計測の `go test` | `make test` の結果 | `(cached)` の行 | 対象のテスト（2 秒の期限） |
|---|---|---|---|---|---|
| 1 | 23:43:46.362〜23:46:03.684 | 23:43:46.571〜23:44:56.776 | 成功 | 0 | 成功（0.54 秒） |
| 2 | 23:46:14.907〜23:48:58.893 | 23:46:14.976〜23:47:27.461 | 成功 | 0 | 成功（0.16 秒） |
| 3 | 23:49:10.976〜23:51:49.206 | 23:49:11.108〜23:50:25.448 | 成功 | 0 | 成功（0.39 秒） |

区間の始まりと終わりは、設計書 3.3.2 のとおり、最初の `=== RUN` の行と `test-parse-norace` のコマンドの行の時刻である。3 回とも、計測の `go test` は区間の中で始まって終わったので、やり直しはなかった。

## 3. AC-07 の結論

### 3.1. 条件の比較

設計書 3.3.3 の規則で、中央値を比べた。

| 比べる条件 | 中央値（遅いほう / 速いほう） | 比 | 候補か |
|---|---|---|---|
| (a) と (b)：並行しないときの、新しいファイルの効果 | 0.367 / 0.020 | 18.4 倍 | 候補（新しいファイル） |
| (c) と (d)：並行するときの、新しいファイルの効果 | 0.340 / 0.018 | 18.9 倍 | 候補（新しいファイル） |
| (b) と (d)：同じファイルでの、並行の負荷の効果 | 0.020 / 0.018 | 1.1 倍 | 候補でない |

### 3.2. 結論

遅れの主な原因は、**新しいファイルの初回の検査**である。新しいファイルの実行は、並行の有無によらず、中央値で同じファイルの約 18 倍（約 0.35 秒）かかった。一方、同じファイルの実行は、`make test` と並行して負荷平均が 10 前後に上がっても、中央値がほとんど変わらず（0.020 秒と 0.018 秒）、最大も 0.170 秒だった。

ただし、1 秒を超える遅れは、新しいファイルを `make test` と並行して実行したとき（(c)）にだけ現れた（30 回のうち 8 回。最大 2.144 秒）。並行しない (a) では最大 0.387 秒で、並行する (d) では最大 0.170 秒だった。1 秒を超えた 8 回は、どれも `XprotectService` が CPU を 31〜72% 使っている時刻と重なり、8 回のうち 7 回は区間の始めの 3 回までに集まっていた。区間の始めは、`make test` がパッケージごとのテストのバイナリを次々に実行し始める時期である。このことから、2 秒の期限を超える遅れは、新しいファイルの初回の検査が、`make test` と並行して検査の処理が混み合うときに長くなって起きると読める。並行の負荷は、それだけでは遅れを生まない（(b) と (d) の比較）が、新しいファイルの検査の待ちを長くする。

設計書 3.3.3 の最後の規則（遅い回が条件によらずバックグラウンドの検査の時刻と重なる場合）には当たらない。`XprotectService` が 65% まで動いていた時刻にも、(d) の回は 0.170 秒以下で終わっており、遅い回は (c) という条件に偏っているためである。

### 3.3. 待ちそのものを減らす変更を提案するか

別の issue としては提案しない。理由は次のとおりである。

-   対象のテストの失敗は、期限を 30 秒にすることで止まる。観察された最大の遅れ（2.144 秒）に対して、30 秒は 14 倍の余裕がある。待ちを減らす変更（スクリプトを使い回すなど）は、期限の延長と重ねて行う必要がない。
-   新しいファイルの初回の検査は、`writeHelperScript` を使うほかのテストにも同じく掛かるが、それらのテストは今のところこの遅れで失敗していない（要件書 2.3）。待ちを減らすには、ヘルパーのスクリプトの作り方を多くのテストにわたって変えることになり、そのテストが確かめていること（新しいプロセスとしての振る舞いなど）を確かめ直す範囲が広い。
-   ほかのテストが同じ形で失敗したときは、本書の結論（新しいファイルの初回の検査が、並行の負荷の下で長くなる）を判断の材料に、その issue で扱う（実装計画 §9）。
