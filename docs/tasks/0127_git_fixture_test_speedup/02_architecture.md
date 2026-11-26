# アーキテクチャ設計書：review_records・high_review_decision・mergemain のテストの高速化

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-26 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（要件定義書。以下、要件書）の設計である。既存のファイルについての記述は、コミット `95793184` のファイルで確かめた。`file:line` はこのコミットの行番号を指す。F-NNN・AC-NN は要件書の項番を指す。要件の起草で設計に申し送られた事項（[design_handoff.md](design_handoff.md) の H-01〜H-05）への対応は、付録 A にまとめる。

本書の計測は、すべて次の機械で行った（以下、計測の機械）: 12 コアの macOS、`PATH` の bash は 5.3.15、`/bin/bash` は 3.2.57、Go 1.27.1、`GOMAXPROCS` は設定しない。本書の数値は設計を決めるための予備の計測で、AC-03 の記録の代わりにはしない。

## 1. 設計の全体像 (Design Overview)

### 1.1. 設計原則

-   **変えるのはテストのコードだけにする。** スクリプト（`review-records.sh`・`high-review-decision.sh`）と `internal/mergepr` の製品コードは変えない（AC-09）。
-   **減らすのは、テストのバイナリが起動する git のプロセスである。** `-race` 付きのテストのバイナリから git を起動する 1 回の負担は、bash が git を起動する 1 回より大きい（2.1）。そこで、ケースの準備で起動する git を減らす。確かめる対象の実行（スクリプトと `mergemain`）は減らさない。
-   **同じ状態は 1 回だけ作り、ケースごとに複製する。** 複数のケースが同じ手順で同じ状態を作っているところは、最初に必要になったケースがその状態を 1 回だけ作る。ほかのケースは、その状態のディレクトリを自分の一時ディレクトリに複製して始める。複製はファイルの読み書きだけで行い、git を起動しない。
-   **確かめる実行は、各ケースの bash で各ケースが行う。** ある bash の実行の結果を、ほかの bash の名前の下のケースに使わない（AC-05）。準備（git だけで作るブランチ）は bash に依らないので、2 本の bash の間で共有してよい。準備の中でスクリプトを走らせるケースは、準備を bash ごとに行う。
-   **共有の状態を作れなかったときは、それに依るケースを失敗させる。** 作る途中で失敗したり skip したりしたときは、その結果を記録し、後から来たケースも同じく失敗させるか skip させる（AC-08、AC-04）。
-   **ケースの表と期待する出力は、今の場所に残す。** ケースの行・`setup`・`want` は、今と同じ表に置く（要件書 4.5）。変えるのは、表を走らせる関数とフィクスチャを作る関数である。

### 1.2. 設計の際に決めたこと（2026-11-26）

人間のレビュアーが決めたもの:

-   **`cmd/yt2column` の、ケースの準備の 2 本の bash の間での共有:** 共有する。ただし、準備の中で `requireProbe` を呼んでスクリプトを走らせる 6 件のケース（`cmd/yt2column/review_records_test.go:1645-1759`）は、今と同じく bash ごとに準備する。理由: 要件書の試作（共通の初期状態の複製、設定の前の実行の共有、最上位のテスト関数の並列化）では 0.76 で、AC-01 の 75% に届かない。準備を共有すれば、bash ごとの準備の半分が減る。`requireProbe` はケースの前提の検査（AC-07）としてそのケースの bash でスクリプトを走らせるので、共有するとその検査が 1 本の bash でしか走らなくなる。それを避けるため、その 6 件は共有から外す。採らなかった案: 6 件も共有して probe を `PATH` の bash だけで走らせる案（bash 3 での probe がなくなる）と、共有しない案（AC-01 に届かないおそれが高い）。ADR: なし（このタスクの中で閉じるテストの作りである）。
-   **`internal/mergepr` の手段:** `newTakeIn` の状態をテンプレートから複製し、復元と push のケースの準備（`conflictSetup` と begin・merge、`pushFlow`）の後の状態を 1 回だけ作って複製する。その PR の計測で `internal/mergepr` の割合が 75% を超えたときは、同じ PR の中で、`Setup` の git の起動を減らす作り（3.7）を足す。理由: 2.1 の見込みは 0.72〜0.74 で、目標との差が小さい。最初から `Setup` を作り直すと、作り直した状態と同じであることを固定する義務が増えるので、届かないときだけ足す。採らなかった案: 最初から両方入れる案と、設計の前に試作で確かめる案。ADR: なし（同上）。

設計で決めたもの（人間のレビュアーに諮らず、理由とともに記す）:

-   **共有の状態の置き場所と寿命（H-03）:** パッケージごとに 1 つのストア（`internal/snapshottest` の `Store`、3.1）を置く。`TestMain` がテストの開始の前に作り、終わった後に消す。理由: 共有の状態は、それを使う最初のケースが作るので、そのケースの `t.TempDir()` に置くと、そのケースが終わったときに消え、後から来たケースが複製できない。最上位のテスト関数ごとにストアを持つ案は、フィクスチャを作る関数（`newRRFixture` は 30 か所以上から呼ばれる）にストアを渡し回す必要があるので採らない。ADR: なし。
-   **複製の仕組みを置くパッケージ:** テストだけのパッケージ `internal/snapshottest` を新しく作る。`cmd/yt2column` と `internal/mergepr` の両方が同じ仕組み（1 回だけ作る、失敗と skip を伝える、ディレクトリを複製する）を使うためである（CLAUDE.md「DRY」）。パッケージは git を起動せず、ファイルの読み書きだけを行う。git の状態を整える処理（3.2・3.6）は、それぞれのパッケージの補助の関数に置く。AC-09 が認める範囲（`test` のビルドタグのファイルだけを持つパッケージ、`package_reference.md` の行、`internal/pipeline/pipeline_test.go` の数）に収まる。ADR: なし。
-   **コミットの日時を固定しない（H-01）:** 複製した状態は、複製のもとと同じオブジェクトを持つので、コミットの ID も同じになる。共有のために日時を固定する必要はない。日時を固定すると、フィクスチャの作り（`cmd/yt2column/test_helpers_gitfixture.go:56-58`）が変わり、ケースの意味を確かめ直す手間が増える。ADR: なし。
-   **計測と比較の手段（H-05）:** タスクの `_checks/` に 2 つの一時スクリプトを置く（7.3）。BASE と AFTER は、作業ツリーで交互に取り出して流す。git の worktree は作らない（作業ツリーの追加は、開発の規則で使わない操作である: `.claude/commands/_lib/harness-claude.md` の shell-run）。ADR: なし。

### 1.3. 用語

本書は、要件書 6 章の用語（フィクスチャのリポジトリ、末端のサブテスト、実時間、ケースの前提の検査、計測の条件）を同じ意味で使う。ほかに次の用語を使う。

-   **共有の状態:** 複数のケースが使う、1 回だけ作るディレクトリと値の組。次の 3 種類がある。
    -   **共通の初期状態:** フィクスチャのリポジトリを作る関数（`newRRFixtureWithContext`・`newHRDFixture`・`newTakeIn`）が作る、ケースの中身に依らない初めの状態。
    -   **共有する準備:** 1 つのケースの準備（ケースの `setup` が作るブランチ、git の設定のシナリオのブランチ、`mergemain` の復元と push のケースの準備）の後の状態。
    -   **共有する実行の結果:** git の設定のシナリオの、設定の前のスクリプトの実行の結果。bash ごとに 1 つある。
-   **ストア:** 共有の状態を鍵ごとに持つ、パッケージに 1 つの入れ物（3.1）。
-   **作り手:** ある鍵の共有の状態を最初に求め、それを作るケース。後から求めたケースは、作り手が作り終えるのを待ってから複製する。
-   **複製:** 共有の状態のディレクトリを、ケースの一時ディレクトリに、ファイルの中身・モード・シンボリックリンクを保って写すこと。git は起動しない。
-   **整え直し:** 複製の後に、複製した場所に合わせて git の状態を直すこと。index の stat の情報を読み直すことと、`remote.origin.url` を複製した origin に向けることの 2 つがある（3.2・3.6）。

### 1.4. 概念モデル

ケースと共有の状態の関係を示す。矢印 A → B は「A が B を使う、または B から作られる」ことを表す。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef newpkg fill:#ffe8f5,stroke:#d946ef,stroke-width:2px,color:#701a75;

    STORE["Store"]
    INIT[("共通の初期状態")]
    PREP[("共有する準備")]
    RUN[("共有する実行の結果<br>（bash ごと）")]
    CASE["ケース<br>（bash ごとの末端のサブテスト）"]
    COPY[("ケースのリポジトリ<br>（複製と整え直し）")]
    SCRIPT["確かめる実行<br>（スクリプト・mergemain）"]

    STORE --> INIT
    STORE --> PREP
    STORE --> RUN
    PREP --> INIT
    RUN --> PREP
    CASE --> COPY
    COPY --> PREP
    COPY --> INIT
    CASE --> RUN
    CASE --> SCRIPT

    class INIT,PREP,RUN,COPY data
    class SCRIPT process
    class CASE enhanced
    class STORE newpkg

    subgraph Legend["Legend"]
        L1[("状態")]
        L2["既存の構成要素（変えない）"]
        L3["変える構成要素"]
        L4["新しいパッケージの型"]
    end
    class L1 data
    class L2 process
    class L3 enhanced
    class L4 newpkg
```

## 2. システム構成 (System Structure)

### 2.1. 時間の内訳（H-01）

要件書 §1「設計の着手の前の計測で決めたこと」の計測に、本書で次の計測を足した。どれも計測の機械での 1 回の計測である。

| 計測 | 値 |
|---|---|
| `internal/mergepr` のパッケージ全体（テストのバイナリを直接実行） | 実時間 70.9 秒（user 82.6 秒、sys 136.6 秒） |
| `TestMergemain` で始まるテストだけ | 実時間 50.2 秒（user 75.3 秒、sys 126.8 秒） |
| 復元と push の 7 つの最上位のテスト関数を `-skip` で除いたパッケージ全体 | 実時間 41.7 秒 |

-   **sys の時間が大きい:** `TestMergemain*` の CPU の時間の 6 割以上が sys で、プロセスの起動が時間の大半を占める。`-race` 付きのテストのバイナリは大きなメモリの領域を持つので、そこから git を起動する負担が大きいと推測する（確かめていない）。
-   **git の呼び出しの内訳:** `PATH` に記録用の git を置き、`mergemain_test.go` の最上位のテスト関数を 1 つずつ走らせて数えた。テストの準備（`cmdrunner.GitHardening` の引数を付けない呼び出し）が約 2,570 回、テストする `mergemain`（付ける呼び出し）が約 5,240 回だった。
-   **復元と push のケースの準備:** `TestMergemainPushSuccessRejectedAndUnknown/pushed` は、準備（`newTakeIn`・`Setup`・begin・merge・マージのコミット）で約 68 回、確かめる push で 51 回の git を起動した。復元のケースも、準備はほぼ同じ回数である。この準備を、`conflictSetup` と begin・merge の後の状態を使う 12 件と、`pushFlow` の後の状態を使う 22 件、計 34 件が同じ手順で繰り返す（`mergemain_test.go:642,686,747,800,813,824,841,874,924,967,1254,1267`）。
-   **`internal/mergepr` の見込み:** 34 件を含む 7 つの最上位のテスト関数を除くと、70.9 秒が 41.7 秒になった。この 29.2 秒のうち、準備の分（34 件の git の呼び出しの約 6 割）を除けると見込み、約 18 秒減る。`newTakeIn` のテンプレート（1 ケースあたり 6 回の起動が 1 回になる）を足して、割合の見込みは **0.72〜0.74** である。
-   **`cmd/yt2column` の見込み:** 要件書の試作は、共通の初期状態の複製、設定の前の実行の共有、4 つの最上位のテスト関数の並列化で 0.76 だった。準備の共有（3.4）で、bash 3 のケースの準備がなくなる。ケースの準備の git はテストのバイナリから起動され、1 回の負担が大きいので、割合の見込みは要件書と同じ **0.6〜0.7** とする。
-   **`high_review_decision_test.go`:** 末端のサブテストの時間の合計（170.9 秒、要件書 §1）は 3 つのファイルの合計の約 5% である。`newHRDFixture` を同じ入力で呼ぶ所が多い（`realContext` と `markersPlanWith(t, "high")` の組で 16 か所）ので、共通の初期状態の複製だけを当てる。

### 2.2. 全体の構成

テストのファイルとパッケージの関係を示す。矢印 A → B は「A が B を呼ぶ」ことを表す。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef newpkg fill:#ffe8f5,stroke:#d946ef,stroke-width:2px,color:#701a75;

    subgraph CMD["cmd/yt2column"]
        MAIN1["main_test.go<br>（TestMain）"]
        RR["review_records_test.go"]
        HRD["high_review_decision_test.go"]
        GF["test_helpers_gitfixture.go"]
        EQ1["fixture_snapshot_test.go"]
    end
    subgraph MPR["internal/mergepr"]
        MAIN2["main_test.go<br>（TestMain）"]
        MM["mergemain_test.go"]
        TH["test_helpers_git.go"]
        EQ2["takein_snapshot_test.go"]
    end
    SNAP["internal/snapshottest"]
    RRS[("review-records.sh")]
    HRDS[("high-review-decision.sh")]
    TOOL["mergemain<br>（製品コード）"]

    MAIN1 --> SNAP
    MAIN2 --> SNAP
    RR --> GF
    HRD --> GF
    EQ1 --> GF
    GF --> SNAP
    MM --> TH
    EQ2 --> TH
    TH --> SNAP
    RR --> RRS
    HRD --> HRDS
    MM --> TOOL

    class RRS,HRDS data
    class TOOL process
    class MAIN1,RR,HRD,GF,EQ1,MAIN2,MM,TH,EQ2 enhanced
    class SNAP newpkg

    subgraph Legend["Legend"]
        L1[("確かめる対象のスクリプト")]
        L2["既存の構成要素（変えない）"]
        L3["変える・足すテストのファイル"]
        L4["新しいパッケージ"]
    end
    class L1 data
    class L2 process
    class L3 enhanced
    class L4 newpkg
```

### 2.3. コンポーネントの配置

-   **`internal/snapshottest`（新規、テストだけのパッケージ）:** ストア。鍵ごとに共有の状態を 1 回だけ作り、作った結果（成功・失敗・skip）を記録し、ディレクトリを複製する。git を知らない。
-   **`cmd/yt2column` の補助の関数:** 共通の初期状態（`rrFixture`・`hrdFixture`）の鍵と作り方、複製の後の整え直し、共有する準備の間にスクリプトを走らせないことの検査を持つ。
-   **`cmd/yt2column` のテストの本体:** `runRRCases`（`review_records_test.go:839-889`）と `checkConfigScenarios`（同 805-834）が、共有する準備と共有する実行の結果を使う。ケースの表は変えない（ただし、準備を bash ごとに行うケースの印を、表の行に足す）。
-   **`internal/mergepr` の補助の関数:** `newTakeIn`（`test_helpers_git.go:245-270`）の共通の初期状態、`takeIn` の複製と整え直しを持つ。
-   **`internal/mergepr` のテストの本体:** `conflictSetup`（`mergemain_test.go:613-616`）と begin・merge の後の状態、`pushFlow`（同 767-776）の後の状態を、共有する準備にする。

## 3. コンポーネント設計 (Component Design)

### 3.1. `internal/snapshottest`

ストアは、鍵ごとに 1 つの共有の状態を持つ。共有の状態は、ストアの下のディレクトリ 1 つと、それを作る関数が返した値 1 つの組である。

```go
//go:build test

package snapshottest

// Store keeps the directories that tests build once, one per key, under
// a root the caller owns. It is safe for concurrent use by parallel tests.
type Store struct{ /* unexported */ }

// NewStore returns a store whose directories live under root.
func NewStore(root string) *Store

// Close removes every directory the store built.
func (s *Store) Close() error

// Snapshot is one directory built once and the value its build returned.
// It gives no path to the directory: a test can only copy it.
type Snapshot[V any] struct{ /* unexported */ }

// Load returns the snapshot of key. The first test that asks for key builds
// it with build, in its own goroutine, into a new directory; the others wait
// for that build. When the build failed or skipped (t.Fatal, t.Skip, or any
// other runtime.Goexit), Load fails or skips every later caller the same way,
// naming the test that built it.
func Load[V any](t testing.TB, s *Store, key string, build func(t testing.TB, dir string) V) Snapshot[V]

// CopyTo copies the snapshot's directory to dst, which must not exist,
// keeping each file's content and mode and each symbolic link's target.
func (sn Snapshot[V]) CopyTo(t testing.TB, dst string)

// Value returns the value the build returned.
func (sn Snapshot[V]) Value() V
```

-   **1 回だけ作る:** 同じ鍵の `Load` が同時に来ても、作るのは 1 回である。待つケースは、作り手が作り終えるか、失敗するか、skip するまで待つ。
-   **作った結果の記録:** 作る関数が正常に戻ったときだけ「作った」と記録する。`t.Fatal`・`t.Skip` などで作る関数が途中で終わったときは、失敗か skip かと、作り手のテストの名前を記録する。skip のときは理由の文も記録し、後から来たケースを同じ理由で skip させる（3.4 の `TestReviewRecordsUnreviewed` の skip を、bash の間で同じにするため。AC-04）。
-   **鍵の型:** 同じ鍵を、違う値の型で `Load` したときは、そのケースを失敗させる。
-   **ディレクトリを見せない:** `Snapshot` は複製だけを許し、ディレクトリのパスを返さない。共有の状態をケースが書き換えると、ほかのケースが読む状態が変わる（AC-10）ので、書き換える手段を与えない（CLAUDE.md「Keep internals unobservable」）。
-   **寿命:** `TestMain` が `os.MkdirTemp` で作った場所を `NewStore` に渡し、`m.Run` の後に `Close` で消す。各パッケージのストアは、パッケージの変数 1 つに置く。

### 3.2. `cmd/yt2column`: 共通の初期状態

`newRRFixtureWithContext`（`review_records_test.go:136-152`）と `newHRDFixture`（`high_review_decision_test.go:32-50`）は、次のように作る。

-   **鍵:** 共通の初期状態を決める入力のすべてから作る。`rrFixture` は、origin/main に置く `_context.md` の中身（空なら置かない）。2 つの計画のサンプルは `testdata` の固定のファイルなので、同じテストのバイナリの中では変わらない。`hrdFixture` は、`_context.md`・PR の印のある計画・印のない計画の 3 つの中身である。鍵は、種類（`rr` と `hrd`）を先頭に付けて、ほかの共有の状態の鍵と重ならないようにする。
-   **作り方:** 今の 2 つの関数のリポジトリの作り方（`git init` から `feature` のブランチまで）を、そのまま作る関数にする。作る関数は、ストアのディレクトリの下に、自分の `HOME`・`TMPDIR` とリポジトリを作る。複製するのはリポジトリだけである。
-   **ケースの側:** ケースは、今と同じく自分の `HOME`・`TMPDIR`（`t.TempDir()`）を作り、リポジトリを複製する（AC-10）。`m0`・`mb` は、共通の初期状態の値として受け取る。
-   **整え直し:** 複製した後、リポジトリで `git update-index -q --refresh` を 1 回走らせる。複製でファイルの時刻と inode が変わるので、index の stat の情報が作業ツリーと合わなくなり、`git diff-files` のような index を読み直さない git の読み方では、ファイルが変わったように見えるためである。
-   **絶対パス:** `git init` と `cmd/yt2column` のフィクスチャの作り方は、リポジトリの設定に絶対パスを書かない。3.8 の比較のテストが、複製の設定に作り手のディレクトリのパスがないことを確かめる。
-   **起動の数:** 1 ケースあたり、`rrFixture` は約 8 回、`hrdFixture` は約 6 回の git の起動が 1 回になる。

### 3.3. `cmd/yt2column`: git の設定のシナリオ

`checkConfigScenarios`（`review_records_test.go:805-834`）は今、16 のシナリオ（スコープ 2 × 設定 8）のそれぞれで、フィクスチャとブランチを作り、設定の前と後にスクリプトを 1 回ずつ走らせる。これを、グループ（`checkConfigScenarios` の 1 回の呼び出し。今は 4 つのテスト関数から計 4 つ）ごとに次のように変える。

-   **グループのブランチ（共有する準備）:** 鍵は、最上位のテスト関数の名前とグループの名前（`git config`・`git config with a merge`）である。作る関数は、共通の初期状態を複製し、`build`（`rrConfigBranch` など）でブランチを作り、`build` が返した引数を値として返す。bash は鍵に含めない。`build` はスクリプトを走らせない（3.4 の検査で確かめる）。
-   **設定の前の実行（共有する実行の結果）:** 鍵は、グループの鍵に bash の名前を足したものである。作る関数は、グループのブランチを自分のディレクトリに複製し、その bash でスクリプトを 1 回走らせ、ケースの前提の検査（終了コードが 0 で、出力が `base` で始まること。今の `review_records_test.go:815-818`）を行う。検査が失敗したときは、作る関数が失敗し、そのグループのその bash のシナリオはすべて失敗する（H-04、AC-08）。
-   **各シナリオ:** グループのブランチを自分のリポジトリに複製し、設定を書く前の plain git の出力を取り、設定を書き、plain git の出力が変わることを確かめる（今の 819-827 行。ケースの前提の検査をシナリオごとに残す）。そして、そのシナリオの bash でスクリプトを走らせ、その bash の設定の前の実行の結果と比べる。
-   **同じであることの根拠:** 設定の前の実行と各シナリオは、同じ共有の状態の複製の上で走る。複製はオブジェクトを写すのでコミットの ID が同じで、引数と出力に含まれる ID が一致する（要件書 §1 の「同じリポジトリの中だけ」の条件を満たす）。設定の前の実行の `HOME` は空で、シナリオの設定の前の状態と同じである。
-   **名前:** 末端の名前（`<bash>/git config/<scope>/<設定の名前>`）は変えない（AC-04）。

### 3.4. `cmd/yt2column`: ケースの準備の bash の間での共有

`runRRCases`（`review_records_test.go:839-889`）は今、bash ごとに、ケースごとにフィクスチャを作り、`setup` を走らせる。これを、次のように変える。

-   **共有する準備:** 鍵は、最上位のテスト関数の名前とケースの名前である。作る関数は、共通の初期状態を複製し、ケースの `setup` を走らせ、`setup` が返した引数と、フィクスチャの Go の側の値（`m0`・`mb`）を値として返す。ケースは、それを自分のリポジトリに複製し、自分の bash でスクリプトを走らせ、自分のリポジトリで `want` を計算して比べる。`env` も、今と同じく各ケースが自分のフィクスチャで作る。
-   **bash ごとに準備するケース:** `rrCase` に、準備を bash ごとに行うことを表す欄を足す。値を書くのは、準備の中で `requireProbe` を呼ぶ 6 件（1.2）だけである。この欄のケースは、今と同じく bash ごとにフィクスチャを作り、`setup` を走らせる。
-   **共有する準備の中でスクリプトを走らせない検査:** 共有する準備を作るフィクスチャ（その中で `sibling` が作るフィクスチャを含む）には、そのことを表す印を付ける。印のあるフィクスチャで `runScript` を呼ぶと、ケースを失敗させる。欄を書き忘れて `requireProbe` を呼ぶケースを足しても、黙って 1 本の bash の probe にはならず、失敗で気づく（CLAUDE.md「Declare, don't infer」: 欄で宣言し、宣言と食い違う使い方を失敗にする）。
-   **作り手の bash に依らないこと:** 共有する準備は git だけで作り、スクリプトを走らせないので、どちらの bash のケースが作り手になっても同じ状態になる。並列のどちらが先に来るかで結果が変わらない（AC-11）。
-   **skip の扱い:** `TestReviewRecordsUnreviewed` は、`LAST_REVIEWED..HEAD` を確かめるケースを `setup` の中で skip する（`review_records_test.go:1003-1009`）。共有する準備では、作り手の skip を記録し、もう一方の bash のケースも同じ理由で skip する（3.1）。skip の事象の `Test` の値の集まりは変わらない（AC-04）。
-   **ケースの前提の検査:** `mainMerge`・`rrSubmodule` などの検査は、共有する準備の中で 1 回行う。検査はどれも git だけで確かめるので、bash に依らない。検査が失敗すれば、作る関数が失敗し、両方の bash のケースが失敗する（AC-07、AC-08）。
-   **対象:** `runRRCases` を使う 4 つのテスト関数のケースである。`TestReviewRecordsCrossCheck` の中に直接書かれた「pattern matching」と「fail-closed context validation」のサブテストは、準備が数回の git の起動だけなので、共通の初期状態の複製だけを当て、準備は共有しない。

### 3.5. `cmd/yt2column`: 最上位のテスト関数の並列化

`TestReviewRecordsCoverage`・`TestReviewRecordsUnreviewed`・`TestReviewRecordsCrossCheck`・`TestReviewRecordsFableAlias` は、今は最上位で `t.Parallel()` を呼ばない（`review_records_test.go:891,992,1943,2070`）。そのため、4 つは順に走り、それぞれの中のサブテストだけが並列に走る。4 つの最上位で `t.Parallel()` を呼び、4 つが重なって走るようにする。`TestReviewRecordsMode`（同 1036-1046）も同じく並列にする。どれも `t.Setenv` や作業ディレクトリの変更を使わず、フィクスチャの環境は許可リストで作る（`test_helpers_gitfixture.go:41-46`）ので、並列にしてもほかのテストの環境を読まない。

### 3.6. `internal/mergepr`: テンプレートと準備の共有

`takeIn` の共有の状態は、origin（bare）と作業ツリーの 2 つのディレクトリを 1 つのディレクトリの下に持つ。

| 共有の状態 | 種類 | 作り方 | 値 | 使うケース |
|---|---|---|---|---|
| 空の `takeIn` | 共通の初期状態 | `newTakeIn` の今の手順（bare の origin の作成、clone、4 つの設定） | なし | `newTakeIn` を呼ぶすべてのケース |
| 競合のマージの途中 | 共有する準備 | 空の `takeIn` の複製に `conflictSetup`、begin、merge | base head、main、begin の結果、merge の結果とエラー | `conflictSetup` の後に begin・merge を呼ぶ 12 件 |
| マージのコミットの後 | 共有する準備 | 空の `takeIn` の複製に `pushFlow` | begin の結果 | `pushFlow` を呼ぶ 22 件 |

-   **整え直し:** 複製した後、作業ツリーで次の 2 つを行う。(1) `remote.origin.url` を、複製した origin のパスにする。clone は origin の絶対パスをこの設定に書く（`test_helpers_git.go:265`）ので、直さないと、ケースが作り手の origin に push する（H-03）。(2) `git update-index -q --unmerged --refresh` で index の stat の情報を読み直す。マージの途中の状態では、競合したファイルが unmerged なので `--unmerged` を付ける。`mergemain` は `GIT_OPTIONAL_LOCKS=0` で git を走らせる（`internal/cmdrunner/git.go:37`）ので、自分では index を書き直さず、作業ツリーが変わっていないかを stat で判断する読み方をすれば、読み直していない index で誤る。
-   **ケースの側:** ケースは、今と同じく自分の `HOME`・`XDG_CONFIG_HOME` を作り（AC-10）、`PR`・`Env`・`GHCalls` は空から始める。共有する準備の作り手が偽の gh に記録した呼び出しは、ケースに引き継がない。これらのケースのどれも、準備の後の `GHCalls` を確かめていない（`GHCalls` を確かめるのは `mergemain_test.go:159,1093` の 2 つのテストだけで、どちらも共有する準備を使わない）。
-   **ケースの確かめ:** 今のケースは、準備の merge のエラーが nil であることを確かめている（例: `mergemain_test.go:643-646`）。共有する準備は merge の結果とエラーを値として返し、各ケースが今と同じ確かめを行う。
-   **起動の数:** 空の `takeIn` は 1 ケースあたり 6 回が 1 回に、準備の共有は 1 ケースあたり約 68 回が 2 回になる（2.1）。

### 3.7. `internal/mergepr`: 届かないときに足す `Setup` の作り

3.6 を入れた PR の計測（F-001 の計測の条件）で、その PR の `internal/mergepr` の割合が 0.75 を超えたときだけ、同じ PR の中で、`Setup`（`test_helpers_git.go:343-355`）の git の起動を減らす。

-   **やること:** 3 つのコミット（base・branch・main）を、テストのバイナリからの少ない起動で作る（例: 1 回の `git fast-import` で作業ツリーのリポジトリに作り、2 つのブランチを 1 回の push で origin に送る）。
-   **保つこと:** `Setup` の後の状態（origin の 2 つのブランチ、作業ツリーのリポジトリのブランチと remote-tracking の ref、取り出したブランチ、index、作業ツリーのファイル）を今と同じにする。これは 3.8 の比較のテストで確かめる。コミットの順序と親子は今と同じにする。
-   **記録:** 足したかどうかと、そのときの計測の値を、実装計画の受け入れ基準の検証の節に書く。

### 3.8. 複製が作り直しと同じであることのテスト（H-03）

CLAUDE.md「Performance」の「An optimization that adds a correctness obligation」の義務は、「複製して整え直した状態は、ケースの中で作り直した状態と同じである」ことである。これを、パッケージごとに新しいテストのファイルで固定する。

-   **`cmd/yt2column/fixture_snapshot_test.go`:** `rrFixture` と `hrdFixture` の共通の初期状態、git の設定のシナリオのブランチの 1 つ、共有する準備の代表の 1 件について、複製した状態と、今の手順で作り直した状態を比べる。
-   **`internal/mergepr/takein_snapshot_test.go`:** 空の `takeIn`、競合のマージの途中、マージのコミットの後の 3 つについて、同じく比べる。3.7 を足したときは、`Setup` の後の状態も比べる。
-   **比べるもの:** コミットの日時が違うので、コミットの ID ではなく、次を比べる。ref の名前の集まり、各 ref が指すコミットの木と件名と親の形、`git config --local --list`（origin のパスは、それぞれの origin のパスを同じ記号に置き換えて比べる）、index の項目（`git ls-files -s`、unmerged の段を含む）、`git diff-files` が何も出さないこと、作業ツリーのファイル、`MERGE_HEAD` の有無とそれが指すコミットの形。さらに、複製の `.git/config` に作り手のディレクトリのパスがないことを確かめる。
-   **壊して確かめる:** 整え直しの 2 つ（index の読み直しと `remote.origin.url` の書き換え）をそれぞれ外すと、このテストが失敗することを確かめ、コミットのメッセージに書く（CLAUDE.md「Testing Strategy」）。

### 3.9. コンポーネントの責務

| ファイル | 新規/変更 | 責務 |
|---|---|---|
| `internal/snapshottest/snapshot.go` | 新規 | `Store`・`Snapshot`・`Load`・`CopyTo`（3.1）。`//go:build test` |
| `internal/snapshottest/snapshot_test.go` | 新規 | 1 回だけ作ること、失敗と skip を後から来たケースに伝えること、モードとシンボリックリンクを保って複製すること、鍵の型の食い違いを失敗させること |
| `cmd/yt2column/main_test.go` | 変更 | `TestMain` でストアを作り、`m.Run` の後に消す |
| `cmd/yt2column/test_helpers_gitfixture.go` | 変更 | 複製と整え直し、共有する準備の印と `runScript` の検査 |
| `cmd/yt2column/review_records_test.go` | 変更 | `newRRFixtureWithContext` の共通の初期状態（3.2）、`checkConfigScenarios` のグループ（3.3）、`runRRCases` の共有する準備と `rrCase` の欄（3.4）、最上位の並列化（3.5） |
| `cmd/yt2column/high_review_decision_test.go` | 変更 | `newHRDFixture` の共通の初期状態（3.2） |
| `cmd/yt2column/fixture_snapshot_test.go` | 新規 | 3.8 の比較のテスト |
| `internal/mergepr/main_test.go` | 新規 | `TestMain` でストアを作り、`m.Run` の後に消す |
| `internal/mergepr/test_helpers_git.go` | 変更 | `newTakeIn` の共通の初期状態、`takeIn` の複製と整え直し（3.6）。3.7 を足すときは `Setup` |
| `internal/mergepr/mergemain_test.go` | 変更 | 競合のマージの途中とマージのコミットの後の共有する準備（3.6） |
| `internal/mergepr/takein_snapshot_test.go` | 新規 | 3.8 の比較のテスト |
| `internal/pipeline/pipeline_test.go` | 変更 | `testOnlyPackageDirs` に `internal/snapshottest` を足す（`internal/pipeline/pipeline_test.go:540-547`） |
| `docs/dev/conventions/package_reference.md` | 変更 | `internal/snapshottest` の行 |
| `docs/tasks/0127_git_fixture_test_speedup/_checks/` | 新規 | 計測と比較の一時スクリプト（7.3） |

`internal/snapshottest` は `testutil/` の下に置かないので、`TestFakesCarryBuildTag` の `testutil` のファイルの数（`internal/pipeline/pipeline_test.go:578`）は変わらない。`TestPackageReferenceListsPackages` は、新しいパッケージの行を求める。

### 3.10. 変わる既存のテスト

-   製品コードの挙動を確かめるテストの期待は変えない。変わるのは、3 つのファイルのテストの準備の作り方と、`internal/pipeline/pipeline_test.go` のテストだけのパッケージの一覧である。
-   ほかのタスクの設計書が定めた方針に対する例外はない。`newGitRepo` の「HOME を変えるので並列にできない」という制約（`test_helpers_git.go:44-45`）は、`newGitRepo` を変えないので、そのまま残る。

## 4. エラーハンドリング設計 (Error Handling Design)

新しいエラーの型は作らない。失敗は、今と同じく `t.Fatal` 系でケースを失敗させる。

| 場面 | 振る舞い | メッセージの形 |
|---|---|---|
| 共有の状態を作る途中で失敗した | 作り手は、作る関数の中の失敗のメッセージで失敗する。後から来たケースは失敗する | `the shared state <鍵の種類> failed to build in <作り手のテストの名前>` |
| 共有の状態を作る途中で skip した | 後から来たケースは、同じ理由で skip する | 作り手の skip の理由の文と、作り手のテストの名前 |
| 同じ鍵を違う値の型で求めた | そのケースを失敗させる | `the shared state <鍵の種類> holds another type` |
| 共有する準備の中でスクリプトを走らせた | そのケースを失敗させる | `a shared preparation ran the script; mark the case to prepare per bash` |
| 複製・整え直しの失敗（ファイルの読み書き、git） | そのケースを失敗させる | 今の `gitIn` と同じ形（`git <args>: <err>: <stderr>`） |

鍵は長い（`_context.md` の中身を含む）ので、メッセージには鍵そのものではなく、鍵の種類（`rr-initial`・`config-branch` など）と、ケースやグループの名前を出す。

## 5. セキュリティ考慮事項 (Security Considerations)

N/A。要件書 4.2 のとおり、変わるのはテストのコードだけで、`docs/dev/security.md` の規則を適用する場面にならない。ストアのディレクトリは、今のフィクスチャと同じく、テストのプロセスの一時ディレクトリの下に置き、テストの終わりに消す。共有の状態をケースが書き換えられないこと（3.1）と、各ケースの環境を分けること（AC-10）は、セキュリティではなく検証の質の要件として扱う。脅威モデルの図は置かない。

## 6. 処理フロー (Processing Flow Details)

### 6.1. 共有の状態を求めるときの流れ

2 本の bash のケースが、同じ共有する準備を求める場合を示す。

```mermaid
sequenceDiagram
    participant A as ケース（path-bash）
    participant B as ケース（bin-bash-3）
    participant S as Store
    participant G as git

    A->>S: Load（鍵）
    S-->>A: まだない（A が作り手）
    B->>S: Load（鍵）
    Note over B,S: A が作り終えるまで待つ
    A->>G: 共通の初期状態の複製と setup の git
    A->>S: 作った（値）
    S-->>B: 作った（値）
    A->>A: 自分の場所へ複製と整え直し
    B->>B: 自分の場所へ複製と整え直し
    A->>A: path-bash でスクリプトを実行し want と比べる
    B->>B: /bin/bash でスクリプトを実行し want と比べる
```

作り手が途中で失敗したときは、`Store` は「失敗」を記録し、待っていた B に返す。B は 4 章のメッセージで失敗する。

### 6.2. git の設定のシナリオの流れ

```mermaid
flowchart TD
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    BR[("グループのブランチ<br>（bash に共通）")]
    BEFORE["設定の前の実行<br>（bash ごとに 1 回）"]
    PRE{"出力が base で始まるか"}
    FAILALL(["その bash のシナリオをすべて失敗させる"])
    SC["各シナリオ（16 件）"]
    PLAIN{"設定で plain git の出力が変わるか"}
    FAILONE(["そのシナリオを失敗させる"])
    AFTER["設定の後の実行<br>（シナリオの bash）"]
    CMP{"設定の前の実行と同じか"}

    BR --> BEFORE --> PRE
    PRE -->|"いいえ"| FAILALL
    PRE -->|"はい"| SC
    BR --> SC
    SC --> PLAIN
    PLAIN -->|"いいえ"| FAILONE
    PLAIN -->|"はい"| AFTER --> CMP

    class BR data
    class BEFORE,SC,AFTER enhanced

    subgraph Legend["Legend"]
        L1[("共有する準備")]
        L2["変える処理"]
    end
    class L1 data
    class L2 enhanced
```

矢印 A → B は、処理の順序を表す。グループのブランチから各シナリオへの矢印は、シナリオがブランチを複製することを表す。

## 7. テスト戦略 (Test Strategy)

### 7.1. 単体テスト

-   **`internal/snapshottest`:** 3.9 の表のとおり。並列のテストから同じ鍵を同時に求めても、作る関数が 1 回だけ走ることを、作る関数の呼び出しの回数で確かめる。失敗と skip を伝えることは、作る関数の中で `t.Fatal`・`t.Skip` を呼ぶテストを、サブテストとして走らせて確かめる。
-   **複製と作り直しの比較:** 3.8 のとおり。

### 7.2. 受け入れ基準の検証

| AC | 確かめ方 |
|---|---|
| AC-01・AC-02・AC-03 | `_checks/` の計測のスクリプト（7.3）で、実装の PR ごとに BASE と AFTER を 3 回ずつ測る。記録は実装計画に書く。`internal/mergepr` が 0.75 を超えたときは 3.7 を足して測り直す |
| AC-04 | `_checks/` の比較のスクリプト（7.3）で、末端の名前・`fail`・`skip` の集まりを比べる |
| AC-05 | 3.3 と 3.4 の作り。確かめる実行（設定の前の実行を含む）は、鍵に bash を含めるか、ケースが自分の bash で走らせる。レビューで読み、AC-04 の比較で `path-bash` と `bin-bash-3` の両方の名前の下で合格することを見る |
| AC-06 | `make coverage-compare BASE=<その PR の BASE>` を AFTER で流す |
| AC-07 | 13 か所の検査（下の表）のそれぞれについて、前提を一時的に崩し、ケースが失敗するのを見る。結果をコミットのメッセージに書く |
| AC-08 | `internal/snapshottest` の単体テスト（失敗と skip を伝えること）と、共有の状態を作る関数を一時的に失敗させて `go test` が失敗するのを見ること |
| AC-09 | 実装の PR ごとに、`git diff --name-only <merge-base>...HEAD` が 3.9 の表の範囲に収まることを読む |
| AC-10 | 3.2・3.6 の作り（ケースごとの `HOME` など）と、3.8 の比較のテスト |
| AC-11 | `_checks/` の比較のスクリプト（7.3）で、単独の選択と `-shuffle=on` の実行を行う |
| AC-12 | `make test` と `make lint` |

AC-07 の 13 か所の検査は、AFTER で次の場所に残る。

| 検査（BASE の場所） | AFTER での場所 |
|---|---|
| `requireProbe`（`review_records_test.go:125`） | 準備を bash ごとに行うケースの `setup` の中。今と同じく bash ごとに走る |
| `mainMerge`（同 235）、`rrSubmodule`（同 324）、ケースの `setup` の中の 7 か所（同 379、447、690、1356、1439、1817、1832） | ケースの `setup` の中。共有する準備では 1 回、bash ごとに準備するケースでは bash ごとに走る |
| 設定の前の出力（同 817） | 共有する実行の結果を作る関数の中（3.3）。bash ごと、グループごとに走る |
| 設定による plain git の変化（同 826） | 各シナリオの中（3.3） |
| refs/replace（`high_review_decision_test.go:546`） | 今のまま |

### 7.3. 計測と比較の一時スクリプト（H-05）

`docs/dev/conventions/requirements_process.md`「Temporary scripts (`_checks/`)」に従い、2 つのスクリプトを置く。どちらも bash で書き、`jq` を使う（`make test` も `jq` を求める）。

-   **計測のスクリプト（AC-01〜AC-03）:** BASE と AFTER のコミットを引数に取る。作業ツリーがきれいでなければ止まる。BASE・AFTER・BASE・AFTER・BASE・AFTER の順に、作業ツリーで取り出して（`git switch --detach`）計測の条件のコマンドを流す。交互に流すのは、機械の状態のゆっくりした変化を BASE と AFTER に均等に分けるためである（要件書 §1 の計測で、別の日の基準が約 3 割違った）。`go test` の `ok <パッケージ> <秒>s` の行から時間を読み、1 回でも失敗や読めない行があれば、合格を報告せずに止まる。終わったとき（失敗したときを含む）は、もとのブランチに戻す。AC-03 の機械の情報（OS、コアの数、2 つの bash の版、Go の版、`GOMAXPROCS`）も出す。
-   **比較のスクリプト（AC-04・AC-11）:** 同じく BASE と AFTER を取り出して `-json` で流し、AC-04 のとおり末端の名前・`fail`・`skip` の集まりを、BASE の最上位のテスト関数ごとに比べる。AFTER では、3 つのファイルの最上位のテスト関数ごとに、末端の名前を並べた最初と最後を単独で走らせる。共有の状態の種類ごとの「最初でも最後でもない 1 つ」は、実装計画が名前を挙げ、スクリプトに引数で渡す。`-shuffle=on` での 2 回の実行と、そのシードも出す。
-   **取り出しの注意:** BASE にはスクリプトがないので、スクリプトは作業ツリーの外に写してから走らせる（取り出しで自分のファイルが消えないように）。
-   **作りの大きさ:** 必須の 4 つの性質（目的のものを見つける、健全な状態で誤って見つけない、決められないときは合格にしない、呼び出し元を汚さない）を持たせ、それを超える守りは入れない。テストはコミットしない。

## 8. 実装の優先順位 (Implementation Priorities)

`CLAUDE.md`「Optimizations go in their own commit」に従い、共通の初期状態の複製、実行の結果の共有、準備の共有は、それぞれ revert で切り離せるコミットにする（要件書 5 章）。

1.  `internal/snapshottest` と、2 つのパッケージの `TestMain` のストア。
2.  `cmd/yt2column` の共通の初期状態（3.2）と比較のテスト（3.8）。
3.  git の設定のシナリオの共有（3.3）。
4.  準備の bash の間での共有（3.4）。
5.  最上位のテスト関数の並列化（3.5）。
6.  `internal/mergepr` の共通の初期状態と準備の共有（3.6）と比較のテスト（3.8）。計測で届かなければ 3.7。
7.  計測と比較の一時スクリプト（7.3）。最初の実装の PR の計測の前に置く。

PR の分け方は実装計画で決める。`cmd/yt2column`（1〜5）と `internal/mergepr`（6）を別の PR にすれば、PR ごとの割合（F-001）が、それぞれのパッケージの変更の効果をそのまま表す。

## 9. 将来の拡張性 (Future Extensibility)

-   `scriptBashes` を使うほかのテスト（`cross_check_state_docs_test.go`・`adr_candidates_docs_test.go`）も、同じストアで共通の初期状態を共有できる。本タスクの範囲外（要件書 2.3）なので、変えない。
-   `internal/snapshottest` は git を知らないので、ほかの重いフィクスチャ（一時ディレクトリに作るファイルの木）にも使える。

## 付録 A: 設計への申し送りへの対応

| 項目 | 対応 | 本書の場所 |
|---|---|---|
| H-01 | 時間の内訳を測った（要件書 §1 の計測に、`internal/mergepr` の git の呼び出しの内訳と、復元と push のケースを除いた時間を足した）。その内訳から、テストのバイナリが起動する準備の git を減らす手段を選んだ。共有のためにコミットの日時は固定せず、1 つのリポジトリを複製する。`high_review_decision_test.go` と `mergemain_test.go` にも、内訳を見て同じ考え方を当てた | 1.2、2.1、3.2〜3.6 |
| H-02 | 要件の段階で、目標を 75% に下げる決定の変更を経た（要件書「設計の着手の前の計測で決めたこと」）。本書の見込みは `cmd/yt2column` が 0.6〜0.7、`internal/mergepr` が 0.72〜0.74 で、`internal/mergepr` の差が小さいので、届かないときに足す手段（3.7）を決めた | 1.2、2.1、3.7 |
| H-03 | 共通の初期状態はストアに 1 回だけ作り、ケースは自分の `HOME`・`TMPDIR` を持つ。`remote.origin.url` を書き換え、index の stat の情報を読み直す。作れなかったときは、それに依るケースを失敗させる。複製と作り直しを比べるテストで固定する | 3.1、3.2、3.6、3.8 |
| H-04 | 共有するのは、同じ共有の状態の上の同じ実行の結果だけにする。設定の前の出力の検査は共有の側で行い、失敗したときはその bash のグループのシナリオをすべて失敗させる。plain git の比較は各シナリオに残す | 3.3、7.2 |
| H-05 | `_checks/` に計測と比較の 2 つの一時スクリプトを置く。worktree は作らず、作業ツリーで BASE と AFTER を交互に取り出す。機械の情報も同じスクリプトで出す | 1.2、7.3 |
