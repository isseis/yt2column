# アーキテクチャ設計書：chief の状況の表を道具が等幅のテキストで組み、workspace ごとに 2 行で示す

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-18 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本書は [01_requirements.md](01_requirements.md)（要件定義書。以下、要件書）の設計である。以下、実装計画書を計画書と略し、「計画で決める」は本タスクの `03_implementation_plan.md` で決めることを指す。既存のファイルの内容は、コミット `0bbba2c8` で確かめた。`file:line` の行番号は、このコミットでの行番号である。F-NNN・AC-NN は要件書の項番を指し、H-NN は [design_handoff.md](design_handoff.md)（以下、申し送り）の項番を指す。申し送りへの対応は付録 A に、受け入れ基準との対応は付録 B にまとめる。

本書は、要件書 3 章と 6 章の用語（表・表示幅・使える幅・状態の文字列・タブの表示・凡例）と、[0093 設計書](../0093_chief_status_sheet/02_architecture.md) の用語（急ぎの度合い・進行の位置・文書の段・進み具合・実行中のコマンド・行の仕上げ）を、同じ意味で使う。ただし 0093 設計書の「周」は、用語集に合わせて「ラウンド」と書く。そのほかに、次の用語を使う。

-   **表の材料:** ラウンドの中で、表のためだけに Herdr から読む 3 つの値。chief の pane の幅、pane とタブの対応、タブのラベルである（3.7）。
-   **表示の単位:** 表を組むときに、それ以上分けない文字の並び。`Sanitize` が作るエスケープ 1 つか、それ以外の文字 1 つである（3.5）。
-   **語:** 要件書 AC-15 の語。空白を含まない ASCII の印字できる文字の並びで、表示の単位で数える（3.6）。

## 1. 設計の全体像 (Design Overview)

### 1.1. 設計原則

-   **表は道具が組み、セッションは貼るだけにする。** 列の幅、折り返し、切り方、決まった語は、すべて道具の Go のコードが決め、テストを持つ（F-001、[ADR-0046](../../dev/adr/0046-chief-of-staff.md)・[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md)）。`/chief` の定義は、道具が返した行を code block にそのまま並べる。
-   **表と `rows` は同じ行から作る。** 表は、`rows` を出力する直前の、行の仕上げと並べ替えを終えた行（`Row`）から組む。表だけのために値を決め直さないので、表と `rows` は食い違わない（AC-02）。
-   **表の材料は表示にだけ使う。** pane の幅とタブは表の配置と表示にだけ使い、行の状態、急ぎの度合い、送る判断、`problems` を左右しない。読めなければ、決まった代わりの値（80 桁、pane の ID）で組み、ラウンドは失敗しない（AC-05・AC-19、要件書 4.3）。
-   **外から来る値は、エスケープの後の単位で数える。** 表に入る外から来る値は、`Sanitize` と同じ規則で表示の単位に分けてから並べる。エスケープを途中で折り返すことも切ることもなく、表の文字は `rows` の値と同じエスケープを持つ（AC-15・AC-21）。
-   **どの行も、外から来る値で始まらない。** 表のどの行も、決まった語か 4 桁以上の空白で始まるように組む。これで、値がどんな文字列でも code block を閉じない（AC-22）。
-   **分からないものは推し量らない。** 決まった語の表にない値は `?` と示し、ほかの値から推し量らない（申し送りの H-06）。

### 1.2. 設計の際に決めたこと（2026-11-18）

-   **タブの表示は、タブの番号を除いてラベルだけを `[claude]` のように示す。要件書の AC-18 をこの形に改め、要件書を `draft` に戻した。** Herdr の画面にはタブの番号が出ない（人が確かめた）。Herdr のタブの一覧の `number` は、タブを作った順の通し番号で、画面での並びの位置とは限らない（2026-11-18 に herdr 0.9.1 で確かめた。chief の workspace では、`tab_id` が `wC:t1` と `wC:tG` の 2 つのタブの `number` が 1 と 16 だった）。番号をそのまま示す案は、画面と対応しない番号を示すので採らない。workspace の中での並びの位置を示す案は、タブの一覧の並びが画面の並びと同じかを確かめていないので採らない。ADR: なし（表示の細部のため）。
-   **質問の pane がどのタブにあるかは、ラウンドがすでに読んでいる `herdr agent list` の同じ出力から、表のために別に読む。** 今のエージェントの読み取り（`parseAgent`、`internal/chief/herdr.go:273-321`）は `tab_id` を読まない。そこに必須のメンバーとして足すと、Herdr が `tab_id` を出さなくなったときに、表示のためだけの値でラウンド全体が失敗する（要件書 4.3 に反する）。`herdr pane list` の呼び出しを足す案は、ラウンドの Herdr の呼び出しが 3 つ増え、要件書 4.1 の「2 つ増える」と食い違うので採らない。ADR: なし（chief の中の読み方の細部のため）。
-   **使える幅は「chief の pane の幅 − 2」とする。2 は、Claude Code が code block の左に取る余白である。code block の右端で折り返す桁は、実装の最初に実物で確かめる。** 2026-11-18 に、Claude Code 2.1.327 を herdr 0.9.1 の幅 93 の pane で動かし、code block の行が左に 2 桁の空白を取って描かれることを、`herdr agent read` で読んだ画面で確かめた。右端は、試しの行が画面で要約に置き換えられたため確かめられなかった。右にも 1 桁の余裕を取る案は、要件書 AC-04 の「左の余白を引いた値」から外れるので、確かめる前には採らない。確かめて右端が pane の幅より手前だと分かれば、そのとき値を直す（計画で決める手順）。ADR: なし（表示の細部のため）。
-   **code block が途中で閉じないことは、行の組み方で保つ。** 表の各行は、凡例・見出し・区切り線・行がないときの語のどれかの決まった語か、状態の列の印か、4 桁以上の空白で始まる（3.2）。CommonMark では、囲いの閉じは 0〜3 桁の字下げしか許さないので、4 桁以上の空白で始まる行は囲いを閉じない。表の中のバッククォートより長い囲いを `/chief` の定義で使う案は、行の組み方だけで閉じないことが保たれ、定義の手順が増えるだけなので採らない。値に ```` ``` ```` や `~~~` を含む表のテストで確かめる（7.3）。ADR: なし（表示の細部のため）。
-   **表は、行ごとの文字列の配列 `table` として出力に足す（申し送りの H-01）。** 各要素は改行を含まず、末尾に空白を持たない。1 つの文字列にする案は採らない。セッションは道具の出力を JSON のテキストとして読むので、1 つの文字列だと、行の区切りが `\n` のエスケープになり、セッションが改行に戻して写す手間が増えるためである。配列なら、要素を 1 行ずつ並べるだけで済み、行の数も出力から読める。ADR: なし（出力の形の細部のため）。
-   **道具の JSON の出力で、`<`・`>`・`&` を `<` などにエスケープしない。** 今の出力は `json.Marshal` で作られ（`cmd/chief/main.go:325`）、この 3 文字を `<`・`>`・`&` と書く。PR の題名などに含まれるこれらの文字を、セッションが元の文字に戻して写す手間をなくし、写し違いの危険を減らすためである（要件書 4.3 の残る危険）。JSON として読んだ値は変わらない。ADR: なし（出力の綴りの細部のため）。
-   **表示幅は、Unicode の East Asian Width が W か F の文字を 2 桁、結合文字（一般カテゴリー Mn・Me）を 0 桁、そのほか（曖昧な文字 A を含む）を 1 桁とする。W・F の範囲は、Go の `unicode.Version`（今は 17.0.0）と同じ版の `EastAsianWidth.txt` から作った表として道具に持つ（申し送りの H-02）。** 標準ライブラリーは East Asian Width を持たず、外部のモジュールは使えない（CLAUDE.md の「Dependencies」）。曖昧な文字を 1 桁とするのは、表が使う `─`（U+2500）と `…`（U+2026）が曖昧な文字で、Claude Code が `─` を 1 桁として描くことを確かめたためである（2026-11-18、Claude Code 2.1.327 が幅 93 の pane の区切り線に `─` を 93 個並べていた）。`🛑`・`🔶`・`🟢` は W である。異体字セレクタ（U+FE0F、Mn）を伴う絵文字は、端末によって 2 桁で描かれ、その行の後ろの列がずれうるが、その文字は外から来る値にしか現れないので受け入れる（8 章の既知の制限に足す）。ADR: なし（表示の細部のため）。
-   **名前・タスク・PR の列の幅は、その列の見出しと値のうち最も広いものの表示幅とする（名前は 20 桁が上限、AC-08）。状態の列も同じ規則で決まる。列と列の間は空白 2 つとする。内容の列の幅は、使える幅からほかの列と間の幅を引いた値で、最小は 20 桁とする（申し送りの H-03）。** 最小を 20 桁にするのは、`Sanitize` が作る最も長いエスケープ（`\U` と 8 桁、10 桁）と、`[claude]` に質問の頭の数文字が並ぶ幅を、1 行に収めるためである。80 桁の pane では、状態 15・名前 20・タスク 6・PR 5 と間 8 で 54 桁を使い、内容の列は 78 − 54 = 24 桁になる。ADR: なし（表示の細部のため）。
-   **行が 1 つもないときは、凡例・空行・見出し・区切り線の後に、決まった 1 行 `workspace なし` を置く（AC-10、申し送りの H-06）。** 凡例と見出しだけだと、表を読めなかったのか行がないのかが分からないためである。ADR: なし（表示の細部のため）。
-   **読めなかったもの（`unreadable`）が 2 つ以上あるときは、道具の順に `, ` で結ぶ。タブのラベルが空のときは、タブを引けないときと同じく pane の ID を示す（申し送りの H-06）。** 空のラベルを `[]` と示しても、どのタブか分からないためである。ADR: なし（表示の細部のため）。
-   **折り返しは、語の途中を避け、語と ASCII でない文字の境目と、ASCII でない文字どうしの境目では折り返してよいとする。全角の空白（U+3000）は空白に含めず、ASCII でない文字の 1 つとして扱う（申し送りの H-06）。** 日本語の文には空白がないので、境目で折り返せないと、日本語の質問の文がほとんど語の途中扱いになるためである。ADR: なし（表示の細部のため）。
-   **表の日本語の literal は `internal/chief/table.go` とそのテストだけに置く。表示幅の数え方は `internal/chief/textwidth.go` に置く（申し送りの H-07）。** CLAUDE.md の 2 つの例外（AC-26・AC-28）が、ファイルの名前で範囲を名指せるようにするためである。ADR: なし（ファイルの置き方の細部のため）。

この設計で新しく作る ADR も、更新する ADR もない。表を道具が組むこと、印、pane の幅を Herdr から読むこと、2 つの例外は、要件定義のときに [ADR-0046](../../dev/adr/0046-chief-of-staff.md) と [ADR-0035](../../dev/adr/0035-language-policy.md) に記録済みである。上の各項は、chief の中で閉じる表示と読み取りの細部である。

### 1.3. なぜ今の仕組みでは足りないか

今は、道具が行の値を JSON で返し（`internal/chief/output.go:30-54`）、`/chief` のセッションが定義の値の表（`.claude/commands/chief.md:150-178`）を引いて Markdown の表に写す（同 :131-202）。この仕組みのまま定義の書き方だけを変えても、次の要件を満たせない。

-   **列を表示幅でそろえ、2 行に流し込む（AC-07・AC-15）。** セッションは文字ごとの表示幅を数えられず、同じ入力から同じ配置を作る保証もない。Claude Code の Markdown の表は、行ごとに罫線を引き、セルの中で改行できない（要件書 1 章の背景）。
-   **使える幅を pane の幅から決める（AC-04）。** セッションの Bash ツールは端末を持たず、端末の幅を読めない（要件書「要件定義の際に決めたこと」）。pane の幅は Herdr からしか読めず、Herdr を読むのは道具だけである（`TestNoDirectHerdrInvocation`、`cmd/chief/docs_test.go:290`）。
-   **質問のタブを示す（AC-18）。** 今の出力は pane の ID しか持たず（`internal/chief/output.go:82`）、タブのラベルは Herdr のタブの一覧にしかない。

### 1.4. 概念モデル

矢印 A → B は「A から B を作る」を表す。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    ROWS[("Row<br>(after finishRow and sortRows)")]
    SRC[("Table sources<br>(pane width, pane to tab, tab labels)")]
    CELLS["Cells<br>(fixed words and sanitized values)"]
    UNITS["Display units<br>(one escape or one character)"]
    LAYOUT["Layout<br>(column widths, wrap, cut)"]
    TABLE[("table<br>(array of lines)")]
    ROWOUT[("rows<br>(unchanged)")]

    ROWS --> CELLS
    SRC --> CELLS
    SRC --> LAYOUT
    CELLS --> UNITS --> LAYOUT --> TABLE
    ROWS --> ROWOUT

    class ROWS,SRC,TABLE,ROWOUT data
    class CELLS,UNITS,LAYOUT enhanced

    subgraph Legend["Legend"]
        L1[("Data")]
        L2["New step of the table"]
    end
    class L1 data
    class L2 enhanced
```

-   **セル:** 行の各列の 1 行目と 2 行目に入る文字列。決まった語（3.3）と、外から来る値（3.4）からなる。
-   **表の材料:** pane の幅は使える幅を決め、pane とタブの対応とタブのラベルはタブの表示を決める。
-   **表示の単位:** セルの文字列を、`Sanitize` と同じ規則で分けたもの。幅は 3.5 の規則で決まる。
-   **配置:** 列の幅を決め、内容の列を 2 行に流し込み、名前と内容を切る（3.2・3.6）。

## 2. システム構成 (System Structure)

### 2.1. 全体構成

矢印 A → B は「A が B を呼ぶか、B に値を渡す」を表す。点線の矢印は、このタスクで足す呼び出しである。

```mermaid
flowchart TD
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef newpkg fill:#ffe8f5,stroke:#d946ef,stroke-width:2px,color:#701a75;

    DEF["/chief definition<br>(.claude/commands/chief.md)"]
    MAIN["cmd/chief main.go"]
    ROUND["readRound<br>(snapshot.go)"]
    FINISH["finishSnapshot<br>(snapshot.go)"]
    HERDR["Herdr reader<br>(herdr.go)"]
    TABLE["layoutTable<br>(table.go)"]
    WIDTH["display width<br>(textwidth.go)"]
    SAN["Sanitize and its units<br>(sanitize.go)"]
    OUT["Snapshot.Output<br>(output.go)"]
    HCLI[("herdr CLI<br>agent list, tab list, pane layout")]

    DEF -->|"snapshot / tick"| MAIN
    MAIN --> ROUND
    ROUND --> HERDR
    HERDR --> HCLI
    ROUND --> FINISH
    FINISH -.-> TABLE
    TABLE -.-> WIDTH
    TABLE -.-> SAN
    FINISH --> OUT
    OUT -->|"JSON with table"| DEF

    class HCLI data
    class DEF,MAIN,ROUND,FINISH,HERDR,SAN,OUT enhanced
    class TABLE,WIDTH newpkg

    subgraph Legend["Legend"]
        L1[("External command output")]
        L2["Changed component"]
        L3["New file"]
    end
    class L1 data
    class L2 enhanced
    class L3 newpkg
```

### 2.2. コンポーネント配置

| ファイル | 新規・変更 | 責務 |
|---|---|---|
| `internal/chief/table.go` | 新規 | 行と表の材料から表の行の配列を組む（`layoutTable`）。決まった語（見出し・凡例・状態の文字列・文書の段・実行中のコマンドが決まらない理由・「＋指示」・件数・「マージ確認」・`workspace なし`）を持つ、chief の道具で日本語の literal を置く唯一のファイル。列の幅、内容の文字列の組み方、折り返し、切り方 |
| `internal/chief/table_test.go` | 新規 | 表のテスト（7.1）。日本語の literal を置いてよいテストのファイル |
| `internal/chief/textwidth.go` | 新規 | 表示の単位の表示幅（3.5）。East Asian Width の W・F の範囲の表 |
| `internal/chief/textwidth_test.go` | 新規 | 表示幅のテスト |
| `internal/chief/sanitize.go` | 変更 | `Sanitize` の置き換えを、表示の単位の列を返す関数に分ける。`Sanitize` はその単位をつないだものを返し、結果は今と同じである |
| `internal/chief/herdr.go` | 変更 | `herdr tab list` と `herdr pane layout` の読み取り、`herdr agent list` の出力からの pane とタブの対応の読み取りを足す。ラウンドのためのエージェントの一覧の読み取り（`agentListing`）を足す。`Agents` の振る舞いは変えない |
| `internal/chief/snapshot.go` | 変更 | `readRound` が表の材料を読み、`roundView` に持つ。`finishSnapshot` が、行を並べた後に表を組んで `Snapshot` に入れる |
| `internal/chief/output.go` | 変更 | `SnapshotOutput` に `table` と `table_unreadable` を足す。`OutputValueNames` を除く（3.12） |
| `internal/chief/records.go` | 変更 | `Protocol` を `main` の値 + 1 にする（AC-25） |
| `internal/chief/test_helpers.go` | 変更 | テストの記録係が許す読み取りのコマンドに、`tab list` と `pane layout` を足す |
| `internal/chief/testdata/herdr/` | 変更 | `herdr tab list` と `herdr pane layout` の実物のサンプルを足す。`testdata/README.md` に採った日と版を書く |
| `cmd/chief/main.go` | 変更 | JSON の出力で `<`・`>`・`&` をエスケープしない（1.2） |
| `cmd/chief/docs_test.go` | 変更 | 定義の値の表を確かめる `TestDisplayValuesMatchTool` を除く（3.12） |
| `.claude/commands/chief.md` | 変更 | 手順 2 の項目 2 を、道具の `table` を code block で示すことに置き換え、値の表と内容の列の規則を除く。`--protocol` の値（3.9） |
| `docs/dev/guides/chief_guide.md` | 変更 | 2 章のプロトコル番号、3.1 の表の見方、8 章の既知の制限（3.10） |
| `CLAUDE.md` | 変更 | 「Go Idioms」と「Declare, don't infer」の例外（3.11） |
| `docs/tasks/0102_chief_text_table/01_requirements.md` | 変更 | AC-18 のタブの表示をラベルだけにする（1.2）。`draft` に戻す |

### 2.3. データフロー

`snapshot` と `tick` は同じ `readRound` と `finishSnapshot` を通るので、どちらの出力にも表が入る。`tick` は自動送信の結果を行に当てた後に `finishSnapshot` を呼ぶ（`internal/chief/tick.go:85-86`）ので、表は自動送信の後の行を示す。

```mermaid
sequenceDiagram
    participant D as /chief session
    participant M as cmd/chief
    participant R as readRound
    participant H as herdr CLI
    participant F as finishSnapshot
    participant T as layoutTable

    D->>M: chief tick --protocol N
    M->>R: read the round
    R->>H: agent list
    H-->>R: agents, and pane to tab from the same output
    R->>H: tab list
    H-->>R: tab labels (or unreadable)
    R->>H: pane layout --pane chief pane
    H-->>R: chief pane width (or unreadable)
    R->>R: read and classify the workspaces (unchanged)
    R-->>F: roundView with table sources
    F->>F: finishRow, pending, sortRows (unchanged)
    F->>T: sorted rows and table sources
    T-->>F: table lines and what could not be read
    F-->>M: Snapshot
    M-->>D: JSON with rows, table, table_unreadable
    D->>D: paste table in a code block, unchanged
```

## 3. コンポーネント設計 (Component Design)

### 3.1. 型

表の材料と、表を組む関数の形を示す。名前は計画で変えてよいが、責務の分け方はこのとおりにする。

```go
// tableSources is what a round reads from Herdr for the table only. A
// source that could not be read holds its error and its zero value; the
// table is still laid out (80 columns, pane IDs).
type tableSources struct {
	paneWidth   int // the chief pane's width; 0 when widthErr is set
	widthErr    error
	paneTabs    map[PaneID]tabID // nil when paneTabsErr is set
	paneTabsErr error
	tabLabels   map[tabID]string // raw labels; nil when tabsErr is set
	tabsErr     error
}

// agentListing is one `herdr agent list` read for a round: the agents as
// Agents returns them, and the pane-to-tab map read from the same output.
type agentListing struct {
	Agents      []Agent
	PaneTabs    map[PaneID]tabID
	PaneTabsErr error
}

// Readers added to herdr.go. Each returns errHerdrOutput for output of
// another shape (section 3.7).
func parseAgentTabs(out []byte) (map[PaneID]tabID, error)
func parseTabList(out []byte) (map[tabID]string, error)
func parsePaneLayout(out []byte, pane PaneID) (width int, err error)

// layoutTable lays out the status table from the sorted rows. It returns
// the lines, each without a newline or trailing spaces, and the reasons
// a table source could not be used, in English.
func layoutTable(rows []Row, src tableSources) (lines []string, unreadable []string)
```

`Snapshot` に `Table []string` と `TableUnreadable []string` を足し、`SnapshotOutput` に次の 2 つを足す。`TickOutput` は `SnapshotOutput` を埋め込む（`internal/chief/output.go:714-719`）ので、`tick` の出力にも入る。

```go
type SnapshotOutput struct {
	// ... the members of today, unchanged ...
	// Table is the status table, one line per element, which /chief
	// pastes in a code block unchanged.
	Table []string `json:"table"`
	// TableUnreadable names the table sources that could not be read, for
	// investigation; /chief does not show it.
	TableUnreadable []string `json:"table_unreadable"`
}
```

-   どちらも空のときは `null` ではなく `[]` を出力する（今の出力の決まり、`internal/chief/output.go:255-256`）。表は、行がなくても凡例などの行を持つので、空にはならない。
-   `table_unreadable` の各項目は、読めなかった材料（`pane layout`・`tab list`・`agent list tabs`）と、エラーの文を `Sanitize` したものである。表のタブの表示を pane の ID に戻した質問の pane（ラベルが空か、タブが一覧にない）も、その pane の ID とともに挙げる。`/chief` の定義は示さない。chief 利用ガイドの「調べ方」が、80 桁で組まれた理由や pane の ID が出た理由をここで調べると書く（3.10）。
-   表の文字列は、`rows` と同じ `Row` から組むので、`rows` の値は変わらない（AC-02）。

### 3.2. 表の形

表の行は、上から次の順に並ぶ（AC-06）。

| 順 | 行 | 行の頭 |
|---|---|---|
| 1 | 凡例 `🛑 すぐ  🔶 あとで  🟢 待つ` | `🛑` |
| 2 | 空行（空の文字列） | （なし） |
| 3 | 見出し（状態・名前・タスク・PR・内容） | `状態` |
| 4 | 区切り線（`─` を表の幅だけ並べたもの） | `─` |
| 5〜 | workspace ごとに 1 行目と 2 行目。workspace の間に空行を 1 つ | 1 行目は状態の列の印、2 行目は状態の列の幅と間の空白 |
| 行がないとき | `workspace なし` | `workspace` |

**列。** 列は、状態・名前・タスク・PR・内容の順に並び、間に空白 2 つを置く。各列の値は、列の幅に足りない分を右に空白で埋める。各行の末尾の空白は除く。

| 列 | 幅 | 1 行目 | 2 行目 |
|---|---|---|---|
| 状態 | 見出しと各行の値のうち最も広いもの | 印と状態の文字列を空白 1 つで区切ったもの（AC-11・AC-12） | 空 |
| 名前 | 見出しと各行の名前のうち最も広いもの。20 桁が上限 | workspace の名前（`Sanitize` の後）。20 桁を超えれば、19 桁以内に切って `…` を付ける（AC-08） | 空 |
| タスク | 見出しと各行の値（1 行目と 2 行目の両方）のうち最も広いもの | タスク番号。なければ `-` | 文書の段（AC-13） |
| PR | 見出しと各行の値のうち最も広いもの | `#<番号>`。開いた PR がなければ `-` | 空 |
| 内容 | 使える幅 − ほかの列の幅 − 間の空白 8 桁。最小 20 桁 | 内容の文字列の 1 行目（3.6） | 内容の文字列の 2 行目 |

状態の列の値は決まった語だけなので、今の語では幅は 15 桁（`🛑 レビュー待ち`）になる。2 行目は、状態の列の空白（15 桁）と間の空白（2 桁）で始まり、タスクの列の文書の段がいつも値を持つ（タスクがなければ `-`）ので、空の行にはならない。

**使える幅。** 使える幅は、chief の pane の幅 − 2 である（1.2）。pane の幅を読めなければ、pane の幅を 80 桁とみなす（AC-05）。内容の列が最小の 20 桁を取れないほど使える幅が狭いときは、内容の列を 20 桁として組み、行が使える幅を超えることを受け入れる（AC-05）。凡例（24 桁）は、使える幅が 24 桁未満のときにだけ使える幅を超える。これも同じく受け入れる。

**区切り線の長さ。** 区切り線は、列の幅と間の空白の合計（表の幅）だけ `─` を並べる。内容の列が最小の幅に広げられたときも、表の幅に合わせる。

**code block を閉じないこと（AC-22）。** 表の各行の頭は上の表のとおりで、外から来る値で始まる行はない。外から来る値は、名前の列（2 列目）か、内容の列（5 列目）にしか入らない。2 列目より前には状態の列があり、1 行目は印で、2 行目は 17 桁の空白で始まる。

### 3.3. 決まった語

決まった語は、すべて `internal/chief/table.go` に置く。各語の値は要件書のとおりで、ここでは対応の元と、表にない値の示し方だけを定める。

| 語 | 対応の元 | 値 |
|---|---|---|
| 印 | 行の急ぎの度合い（`Row.Urgency`） | AC-11 |
| 状態の文字列 | 行の進み具合（`Row.Position.Progress`） | AC-12 |
| 文書の段 | 行の文書（`Row.Position.Document`、`PR-N` は `Row.Position.PlanPR`） | AC-13 |
| 実行中のコマンドが決まらない理由 | `Row.RunningUnknown` | AC-20 |
| 凡例、見出し、「＋指示」、`<件数> 件`、`マージ確認` | なし | AC-06・AC-14・AC-16 |
| 行がないときの語 | なし | `workspace なし`（1.2） |

-   各対応は、型の値ごとの `switch` で決める。どの値にも当たらない値（`default`）は `?` と示す（申し送りの H-06）。今の型の値はすべてどれかの語に当たり、`?` を示す道はない。新しい値を型に足して語を足し忘れたときに、そのことがテストで分かるようにする（7.1）。
-   印を決めるのは急ぎの度合いで、状態の文字列を決めるのは進み具合である。どちらも、行の仕上げが決めた値をそのまま使い、表が決め直さない（0093 要件書の AC-21 を保つ）。

### 3.4. 内容の文字列

内容の文字列は、進み具合から決まる状態の文字列ごとに、次の部品を空白 1 つで区切って並べたものである（AC-14）。値がない（空の）部品は並べず、その部品の区切りの空白も置かない。

| 状態の文字列 | 部品（この順） |
|---|---|
| マージ確認、質問で停止 | `<件数> 件`（質問が 2 つ以上のとき）、`マージ確認`（状態の文字列が「質問で停止」で、示す質問がマージの確認のとき）、タブの表示、質問の文 |
| レビュー待ち | PR の題名 |
| 次を提案 | 送れる提案（`Next.Command` が nil でない）なら、コマンド（`Next.Text`）と `(<モデル>)`（モデルが空なら置かない）。送れない提案なら、送れない理由（`Next.Problem`） |
| 作業中 | 実行中のコマンドが決まれば、コマンド（`Running.Command.Text()`）、`＋指示`（`Running.Later` のとき）、`(<モデル>)`（モデルが空でないとき）。決まらなければ、決まらない理由（3.3） |
| 要確認 | 読めなかったもの（`Row.Unreadable`）を道具の順に `, ` で結んだもの |
| タスク完了 | なし |

-   **示す質問（AC-16）。** マージの確認である質問（`Question.Merge`）があれば、道具の順で最初のマージの確認を示し、なければ道具の順で最初の質問を示す。示さない質問は表に出さない。
-   **タブの表示（AC-18・AC-19）。** 示す質問の pane から、pane とタブの対応でタブを引き、タブのラベルで引いたラベルを `[<ラベル>]` と示す。次のどれかのときは、`[<pane の ID>]` を示し、`table_unreadable` にその pane を挙げる。pane とタブの対応を読めない、対応にその pane がない、タブのラベルを読めない、ラベルの一覧にそのタブがない、ラベルが空である。どのときも、行の状態、急ぎの度合い、送れるかどうかは変わらない。
-   **外から来る値。** workspace の名前、PR の題名、質問の文、送れない理由、読めなかったもの、モデル、タブのラベル、pane の ID は、行（`Row`）の生の値から、`Sanitize` と同じ規則で表示の単位に分ける（3.5）。決まった語と、型から作るコマンドの文字列（`Next.Text`・`Running.Command.Text()`）も同じ道を通す。改行とタブは、表の中でだけ空白の単位 1 つにする（AC-17）。
-   **モデルの括弧** は ASCII の `(` と `)` である。今の定義の「its model in parentheses」（`.claude/commands/chief.md:192`・`:196`）と同じく、コマンドの後に空白を挟んで置く。

### 3.5. 表示幅と表示の単位

**表示の単位。** 値の文字列を、`Sanitize` と同じ規則で前から分ける。

-   `Sanitize` が置き換える文字（C0 制御文字、DEL、C1 制御文字、書式文字、タグの文字、行と段落の区切り、UTF-8 として正しくないバイト）は、そのエスケープ（`\x1b`・`​` など）が 1 つの単位である。
-   バックスラッシュは、`\\` が 1 つの単位である。
-   改行とタブは、表の中では空白の単位 1 つである（`Sanitize` は残す。AC-17）。
-   そのほかの文字は、1 文字が 1 つの単位である。

`Sanitize` の今の実装（`internal/chief/sanitize.go:52-69`）を、この単位の列を作る部分と、それをつなぐ部分に分ける。`Sanitize` の結果は今と同じで、表の単位をつないだものは、改行とタブを除いて `Sanitize` の結果と同じになる。同じ規則を 2 か所に書かないためである（CLAUDE.md の「DRY」）。

**表示幅。** 単位の表示幅は次のとおりである。

| 単位 | 表示幅 |
|---|---|
| エスケープ | そのバイト数（ASCII だけからなるため） |
| East Asian Width が W か F の文字 | 2 |
| 一般カテゴリーが Mn か Me の文字（結合文字・異体字セレクタ） | 0 |
| そのほかの文字（曖昧な文字 A を含む） | 1 |

-   W・F の範囲は、Go の `unicode.Version` と同じ版の Unicode の `EastAsianWidth.txt` から作った範囲の表として `textwidth.go` に持つ。表の版は、ファイルの注釈と、`unicode.Version` と比べるテストで示す。Go の版が上がって `unicode.Version` が変わると、テストが失敗し、表を作り直すことを促す。表を作る手順は計画で決める。
-   書式文字（ゼロ幅の空白、ゼロ幅の接合子など）は `Sanitize` が見える形に置き換えるので、幅 0 の書式文字が表に残ることはない。

### 3.6. 折り返しと切り方

内容の文字列を、内容の列の幅で 2 行に流し込む（AC-15）。名前の列は 1 行に収め、収まらなければ切る（AC-08）。どちらも表示の単位の境目でだけ折り返し、切る。

**折り返してよい位置。** 次の境目で折り返してよい。

-   空白の単位の前後。空白の位置で折り返したときは、その空白を 2 行目の頭に置かない。
-   ASCII でない文字の単位の前後。ただし、幅 0 の単位の直前では折り返さない。結合文字を直前の文字から離さないためである。
-   語の中（語を作る単位どうしの境目）では折り返さない。ただし、その語が 1 行の幅に収まらないときだけ、語の中の単位の境目で折り返す。エスケープは 1 つの単位なので、その途中では折り返さない。

1 行目には、折り返してよい位置のうち、1 行目の表示幅が内容の列の幅以下に収まる最も後ろの位置までを入れる。表示幅 2 の単位が 1 行目の残りの幅に入らないときは、その単位から 2 行目に送る。

**切り方。** 残りが 2 行目に収まらないときは、2 行目の末尾に `…` を置き、`…` を含めた 2 行目の表示幅が内容の列の幅以下になるところまで、単位の境目で切る。切った位置の直前の空白は除く。エスケープの途中では切らない。名前の列も同じく、`…` を含めて 20 桁以内になるところまで切る。

**語。** ASCII の印字できる文字（U+0021〜U+007E）の単位と、エスケープの単位が続く並びを 1 つの語とする。エスケープは ASCII の文字だけからなり、`Sanitize` の結果では語の一部として見えるので、語に含める。

### 3.7. Herdr の読み取り

ラウンドは、Herdr の呼び出しを 2 つ足す（要件書 4.1）。どちらも今の読み取りの封筒の確かめ（`herdrResult`、`internal/chief/herdr.go:190-224`）を使い、正しい UTF-8 の、ちょうど 1 つの JSON のオブジェクトで、後ろにデータがなく、対になっていないサロゲートのエスケープを含まず、`error` を持たない出力だけを受け付ける（`strictjson`）。出力の上限は、今の外部コマンドの上限（4 MiB、`internal/chief/runner.go:52`）である。形に合わない出力は `errHerdrOutput`（`internal/chief/herdr.go:19`）にし、直したり一部だけを使ったりしない（AC-23）。

| 読み取り | 呼び出し | 受け付ける形と使うメンバー | 読めないとき |
|---|---|---|---|
| タブのラベル | `herdr tab list`（`--workspace` なし。すべての workspace のタブを 1 回で返す） | 封筒の `id` が `cli:tab:list`、`result.type` が `tab_list`、`result.tabs` が配列。各タブの `tab_id`（今の pane の一覧と同じ形、`tabIDForm`、`internal/chief/herdr.go:483`）と `label`（文字列。空でもよい）。同じ `tab_id` が 2 つ以上あれば読めない | すべての質問のタブの表示が pane の ID になる |
| chief の pane の幅 | `herdr pane layout --pane <chief の pane の ID>` | 封筒の `id` が `cli:pane:layout`、`result.type` が `pane_layout`、`result.layout` がオブジェクトで、その `panes` が配列。各 pane の `pane_id`（`paneIDForm`）。`pane_id` が chief の pane の ID と等しい項目がちょうど 1 つあり、その `rect.width` が 1 以上で `int` に収まる整数 | 80 桁とみなす |
| pane とタブの対応 | 増やさない。ラウンドが読む `herdr agent list` の同じ出力から読む | 封筒の `id` が `cli:agent:list`、`result.type` が `agent_list`、`result.agents` が配列。各エージェントの `pane_id`（`paneIDForm`）と `tab_id`（`tabIDForm`）。同じ `pane_id` が 2 つ以上あれば読めない | すべての質問のタブの表示が pane の ID になる |

-   どの読み取りでも、使うメンバーが欠けているか、型か形が違うとき、`strictjson` が重なりとして拒むときは、その読み取り全体を読めないものとする。ほかのメンバー（タブの `number`・`agent_status`・`focused`・`pane_count`、配置の `area`・`splits`・`zoomed` など）は読まず、Herdr が足しても受け付ける。
-   幅には、`layout.area` ではなく chief の pane の `rect.width` を使う。`area` はタブ全体の大きさで、タブを分割したときに chief の pane より広い（申し送りの H-04）。
-   pane の配置を読む pane は、ラウンドがすでに確かめた chief の pane（`HERDR_PANE_ID` を `ParsePaneID` で読み、エージェントの一覧で確かめたもの、`internal/chief/snapshot.go:94-109`）である。ID は `paneIDForm` に合うので、`-` で始まって Herdr にオプションと読まれることはない。
-   2026-11-18 に herdr 0.9.1 で、3 つの出力が上の形であることを確かめた。`herdr agent list` の出力は、今のサンプル（`internal/chief/testdata/herdr/agent-list.json`）でも、すべてのエージェントが `tab_id` を持つ。
-   **エージェントの一覧の読み取りは 1 回のまま。** `readRound` は、今の `herdr.Agents` の代わりに、同じ出力からエージェントの一覧と pane とタブの対応の両方を返す読み取り（`agentListing`）を呼ぶ。エージェントの一覧の読み取りが失敗すれば、今と同じくラウンドが失敗する。pane とタブの対応だけが読めないときは、ラウンドは失敗しない。`Agents` を呼ぶほかの場所（`wait.go`・`create.go`・`request.go`・`records.go`）は変えない。
-   3 つの読み取りは、いつも 1 回ずつ行う。表の材料がいらないラウンドはない（行がなくても表を示す）。

### 3.8. 出力の形と `--protocol`

-   `snapshot` と `tick` の出力に `table` と `table_unreadable` を足す（3.1）。ほかのサブコマンドの出力は変えない。
-   `rows` の各項目の名前・値・意味は変えない（AC-02）。`rows` の値を日本語にすることもない。
-   道具の `Protocol`（`internal/chief/records.go:29`）を `main` の値 + 1 にする（AC-25）。古い道具は、新しい定義の `--protocol` を `protocol_mismatch` で断り、定義は入れ直しを促す（`.claude/commands/chief.md:77-79`）。新しい道具は、古い定義の `--protocol` を同じく断る。
-   `cmd/chief/main.go` の JSON の書き出しを、HTML のためのエスケープをしない形に変える（1.2）。JSON として読んだ値は、どのサブコマンドでも変わらない。

### 3.9. `/chief` の定義（`.claude/commands/chief.md`）

-   **手順 2 の項目 2 を置き換える（AC-03）。** 新しい項目 2 は、`table` の各要素を道具の順に 1 行ずつ、3 つのバッククォートの囲いの code block に入れて示すことだけを定める。要素の文字は 1 文字も変えず、行を並べ替えず、除かず、足さず、空白を詰めることも足すこともしない。値を写すこと、値を切ること、列を足すことも除くこともしない。
-   **除くもの。** 今の項目 2 の列の表、値の表（「Item / Value / Shown as」の表）、内容の列の規則（`state` ごとの中身、20 文字ほどで切ること、「（定義にない値）」）。
-   **変えないもの。** 項目 1（送信の結果）、項目 3（表の下）、項目 4（通知）、手順 3 以降（AC-03）。項目 3 の「for each row, by its 名前」は、表の名前の列と同じ `label` を指すので、そのまま読める。
-   **`--protocol` の値**を、`start`・`snapshot`・`tick` とほかの呼び出しのすべてで、新しい番号にする（AC-25）。`TestDocumentedProtocolMatches`（`cmd/chief/docs_test.go:205`）が、定義の番号と道具の番号を比べる。

### 3.10. chief 利用ガイド（`docs/dev/guides/chief_guide.md`）

-   **2 章。** プロトコル番号の値（`docs/dev/guides/chief_guide.md:28`）を新しい番号にする（AC-25）。`TestGuideValuesMatchTool`（`cmd/chief/docs_test.go:405`）が道具の番号と比べる。
-   **3.1。** 表の見方を、本書の形で書き直す（AC-24）。凡例、5 つの列、2 行目（文書の段と内容の続き）、使える幅の決め方（pane の幅 − 2、読めなければ 80 桁）、内容の列の流し込みと切り方、タブの表示（`[claude]`、引けなければ pane の ID）。急ぎの度合いと番の表、行の並び、実行中のコマンドの節は残し、印と語を新しいものにする。「調べ方」に、`table_unreadable` で表の材料を読めなかった理由が分かることを足す。
-   **8 章（既知の制限）。** 次を足す（AC-24）。Claude Code の code block の余白が変わると表が折り返しうること。pane の幅を変えた表は次のラウンドまで古い幅のままであること。狭い pane では行がはみ出しうること。セッションの写しは機械的には保証されないこと。異体字セレクタを伴う絵文字など、端末が道具と違う幅で描く文字を含む行は列がずれうること（1.2）。

### 3.11. CLAUDE.md の例外

**「Go Idioms」（AC-26）。** chief の画面の読み取りとタスクの読み取りの例外の項の後に、次の範囲・理由・境界の項を足す。

-   範囲: `internal/chief/table.go` とそのテストの、状況の表の決まった語の literal。語は、見出し、凡例、状態の文字列、文書の段、実行中のコマンドが決まらない理由、「＋指示」、件数と「マージ確認」の添え書き、行がないときの語に限る。これらは表の文字列として道具の出力に入る。
-   理由: 表は人に示す日本語の表で、道具が組むので、語は道具の中に置くほかない。
-   境界: 道具の出力のほかの値、エラーのメッセージ、ほかのサブコマンドの出力、表に入る外から来る値、`table_unreadable` は範囲に含めない。ほかのファイルに日本語の literal を置かない。画面の読み取りの例外の項とタスクの読み取りの例外の項の「the text the tool prints stay English」は、これらの項の範囲の文字列を指し、この項と食い違わない。そのことを、この項の中で一文で述べる。

**「Declare, don't infer」（AC-28）。** chief の画面の読み取りの例外の後に、次の範囲・理由・境界の例外を足す。

-   範囲: `internal/chief/table.go` と `internal/chief/textwidth.go` が、表に入る値の中身を調べて、表示の単位の幅、語の境目、折り返す位置と切る位置を決めること。
-   理由: 値は、ほかのエージェント、GitHub、Herdr、人が書いた文字列で、型で表せない。等幅の端末で列をそろえるには、文字ごとの幅を中身から決めるほかない。
-   境界: 結果は表の配置にだけ使い、何を送るか、行の状態、急ぎの度合い、`rows` の値を左右しない。値を書き換えるのは、改行とタブを表の中で空白にすることと、切った位置に `…` を置くことだけである。表示の単位に分ける規則は `Sanitize` の規則そのものであり、`sanitize.go` はこの例外に含めない（`Sanitize` は今も文字ごとに置き換えを選んでいる）。画面の読み取りの例外の項にある「Every other file of the tool chooses behavior only by the types the screen readers return」は、画面の読み取りについての文で、この例外と食い違わない。そのことを、この例外の中で一文で述べる。

### 3.12. 既存のテストと 0093 の決まりへの影響

| テスト | 今の確かめ | 変わること |
|---|---|---|
| `TestDisplayValuesMatchTool`（`cmd/chief/docs_test.go:464-503`。定数 `displayTableHeader` は :462） | 定義の値の表が、道具の出力の値の集合と等しい | 定義から値の表がなくなるので、テストを除く。テストを除くことは、CLAUDE.md の「Deleting a test is a claim that must be checked」のとおり、そのコミットだけで `make coverage-compare` で確かめる（計画で決める）。決まった語が型のすべての値を覆うことは、表のテスト（7.1）が引き継ぐ |
| `TestOutputValueNames`（`internal/chief/output_test.go:21`） | `OutputValueNames` の名前の集合 | `OutputValueNames` は、上のテストのためだけにあるので除く。出力の名前（`urgency` など）が値ごとに 1 つずつあることの確かめは、`namesUpToUnknown` を使って残す |
| `TestDocumentedProtocolMatches`・`TestGuideValuesMatchTool` | 定義とガイドの番号が道具の番号と等しい | 変えない。番号を上げたときに、定義とガイドを直し忘れると失敗する |
| `snapshot_test.go` などの、出力を JSON で比べるテスト | 出力のメンバー | `table` と `table_unreadable` が増える。テストの記録係が `tab list` と `pane layout` を許し、その答えを持つようにする |
| 記録係の許すコマンド（`internal/chief/test_helpers.go:217-221`） | 読み取りのコマンドだけを許す | `tab list` と `pane layout` を足す。どちらも読み取りである |

0093 の決まりのうち、示し方を変えるもの（要件書 5 章）は次のとおりである。値の決め方（`urgencyOf`・`documentOf`・`progressOf`、`internal/chief/position.go:206-311`）は変えない。

-   0093 の 1.1 の原則「`/chief` の定義は、道具が返した ASCII の値を日本語の表示に置き換え、道具の順のまま示す」は、この設計で「道具が日本語の表を組み、定義はそのまま貼る」に変わる。ADR-0046 は要件定義のときに更新済みである。0093 の設計書は変えない（完了したタスクの記録のため）。

## 4. エラーハンドリング設計 (Error Handling Design)

新しいエラーの型も sentinel error も足さない。

-   表の材料の読み取りは、形に合わない出力を今の `errHerdrOutput` にし、Herdr の失敗（終了コード・時間切れ・出力の上限）は今の `Commands` のエラーのまま返す。
-   どちらのエラーも、ラウンドの失敗にも、`problems` にも、拒否の理由にもしない（要件書 4.2）。`table_unreadable` に、材料の名前とエラーの文（`Sanitize` の後）を英語で残す。
-   表を組む関数（`layoutTable`）はエラーを返さない。表の材料のどれが読めなくても、決まった代わりの値で組む。行の値が型の値のどれにも当たらなければ `?` を示す（3.3）。
-   `herdr pane layout` と `herdr tab list` は、今の Herdr の時間の上限（10 秒、`internal/chief/runner.go:38`）を受ける。時間切れのラウンドは、その分だけ遅れ、表は代わりの値で組まれる。

## 5. セキュリティ考慮事項 (Security Considerations)

### 5.1. 製品

N/A。製品（ブラウザ拡張と記事の生成）に触れない。条件付きのガイド（`docs/dev/security.md`）の適用条件のどれにも当たらない（要件書 4.2）。

### 5.2. 開発の道具の信頼の境界

chief が送る入力の範囲、送る判断の条件、`.claude/settings.json` の拒否は変えない。新しく読む Herdr の出力と、表に入る値は、表示にだけ使う。

矢印 A → B は「A の値が B に流れる」を表す。

```mermaid
flowchart LR
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;
    classDef problem fill:#ffe6e6,stroke:#d62728,stroke-width:2px,color:#7b0000;

    EXT[("Untrusted text<br>(labels, PR titles, questions, tab labels)")]
    HOUT[("Herdr output<br>(tab list, pane layout, agent list)")]
    STRICT["Strict readers<br>(herdr.go)"]
    SAN["Sanitize units<br>(sanitize.go)"]
    LAY["layoutTable<br>(table.go)"]
    TBL[("table<br>(lines)")]
    SESSION["/chief session<br>(code block)"]
    SEND["Send decisions<br>(unchanged)"]

    HOUT --> STRICT --> LAY
    EXT --> SAN --> LAY --> TBL --> SESSION
    EXT --> SEND

    class EXT,HOUT,TBL data
    class STRICT,SAN,LAY enhanced
    class SESSION,SEND process

    subgraph Legend["Legend"]
        L1[("Data")]
        L2["Changed or new component"]
        L3["Unchanged component"]
    end
    class L1 data
    class L2 enhanced
    class L3 process
```

| 脅威 | 守り |
|---|---|
| 値の中の端末の制御文字が、セッションの画面を書き換える | 表のどの値も、`Sanitize` と同じ規則で表示の単位に分けた後のもので、制御文字はエスケープになる（AC-21）。エスケープの途中で折り返しも切りもしないので、切れ端の `\` が別のエスケープに見えることもない（3.6） |
| 値の中の ```` ``` ```` や `~~~` が code block を閉じ、後ろの行が Markdown（見出し・リンクなど）として描かれる | どの行も決まった語か 4 桁以上の空白で始まる（3.2）。値に囲いの文字を含む表のテストで確かめる（7.3） |
| 値の中の改行が表の行を増やし、偽の行を作る | 改行とタブは表の中で空白になる（AC-17）。表の行は道具が作る分だけである |
| 細工したタブのラベルや Herdr の出力が、送り先や送る判断を変える | タブ・幅・pane とタブの対応は表の配置と表示にだけ使い、行の状態・急ぎの度合い・送れるかどうかに渡らない（1.1）。Herdr の出力は厳しく読み、形に合わなければ使わない（3.7） |
| 長い値で表が巨大になる | 内容は 2 行、名前は 20 桁で切る。表の大きさは行の数に比例する |
| セッションが表を写し違える | 機械的には防げない（要件書 4.3 の残る危険）。行の配列と HTML のためのエスケープをしない出力で、写す手間を減らす（1.2）。全文の値は `rows` にある |

## 6. 処理フローの詳細 (Processing Flow Details)

### 6.1. ラウンドの中の表

```mermaid
flowchart TD
    classDef data fill:#e6f7ff,stroke:#1f77b4,stroke-width:1px,color:#0b3d91;
    classDef process fill:#fff1e6,stroke:#ff7f0e,stroke-width:1px,color:#8a3e00;
    classDef enhanced fill:#e8f5e8,stroke:#2e8b57,stroke-width:2px,color:#006400;

    A["readRound: agent list"] --> B{"Agents read?"}
    B -->|"No"| FAIL["Round fails (unchanged)"]
    B -->|"Yes"| C["pane to tab from the same output<br>(error kept, round goes on)"]
    C --> D["tab list<br>(error kept, round goes on)"]
    D --> E["pane layout of the chief pane<br>(error kept, round goes on)"]
    E --> F["Workspaces read and classified (unchanged)"]
    F --> G["finishSnapshot: finishRow, pending, sortRows (unchanged)"]
    G --> H["layoutTable"]
    H --> I[("Snapshot: rows, table, table_unreadable")]

    class A,B,F,G,FAIL process
    class C,D,E,H enhanced
    class I data

    subgraph Legend["Legend"]
        L1["Unchanged step"]
        L2["New step"]
        L3[("Result")]
    end
    class L1 process
    class L2 enhanced
    class L3 data
```

矢印は処理の順を表す。

### 6.2. 表を組む

1.  使える幅を決める（pane の幅 − 2、読めなければ 80 − 2）。
2.  各行のセルを作る（3.3・3.4）。値は表示の単位に分ける（3.5）。
3.  状態・名前・タスク・PR の列の幅を、見出しと値から決める（名前は 20 桁が上限）。名前を切る（3.6）。
4.  内容の列の幅を、使える幅から決める（最小 20 桁）。
5.  各行の内容を 2 行に流し込み、収まらなければ切る（3.6）。
6.  凡例・空行・見出し・区切り線を置き、行ごとの 2 行を、間に空行を挟んで並べる。行がなければ `workspace なし` を置く。各行の末尾の空白を除く。

## 7. テスト戦略 (Test Strategy)

### 7.1. 単体テスト

-   **表示幅（`textwidth_test.go`）。** W・F の文字（漢字・かな・全角の英数字・`🛑`・`🔶`・`🟢`）が 2、ASCII と曖昧な文字（`…`・`─`）が 1、結合文字と異体字セレクタが 0、エスケープがそのバイト数であること。範囲の表の境目の前後の文字。範囲の表の版が `unicode.Version` と等しいこと。
-   **表示の単位（`sanitize_test.go`）。** 単位をつないだものが `Sanitize` の結果と等しいこと（今の `Sanitize` のテストの入力すべてで）。エスケープと `\\` が 1 つの単位であること。
-   **表の形（`table_test.go`）。** 凡例・空行・見出し・区切り線・行の 2 行・間の空行の順（AC-06）。同じ列が、見出しとすべての行で同じ表示幅の位置から始まること（AC-07。全角と絵文字を含む値で）。名前の幅と 20 桁での切り方（AC-08）。各列の 1 行目と 2 行目の値（AC-09）。行がないときの形（AC-10）。末尾に空白がないこと。
-   **幅（`table_test.go`）。** どの行の表示幅も使える幅以下であること（AC-04）。pane の幅を読めないときに 80 桁で組むこと（AC-05）。内容の列が最小の幅を取れないときに 20 桁で組むこと。
-   **決まった語（`table_test.go`）。** 急ぎの度合い・進み具合・文書・実行中のコマンドが決まらない理由の、型のすべての値（0 から、出力の名前が unknown になる最初の値の手前まで）が、`?` でない語を持つこと。語が要件書の表のとおりであること（AC-11〜AC-13・AC-20）。表にない値が `?` になること。
-   **内容（`table_test.go`）。** 状態の文字列ごとの部品と、値がない部品を並べないこと（AC-14）。折り返しの位置（空白、語と ASCII でない文字の境目、語の途中を避けること、収まらない語の途中、幅 2 の文字を 2 行目に送ること、結合文字を離さないこと）と、`…` での切り方（AC-15）。エスケープの途中で折り返しも切りもしないこと。示す質問と添え書き（AC-16）。改行とタブが空白になり、`rows` の値は変わらないこと（AC-17）。タブの表示と、引けない 5 つの場合に pane の ID を示し `table_unreadable` に挙げること（AC-18・AC-19）。
-   **`rows` との一致（`snapshot_test.go`）。** 同じスナップショットの出力の `rows` の値（名前・タスク・PR の番号）が表に現れ、行の並びが同じであること（AC-01・AC-02）。
-   **Herdr の読み取り（`herdr_test.go`）。** 実物のサンプル（`tab list`・`pane layout`・今の `agent list`）を読めること。要件書 4.2 と 3.7 の、形に合わないそれぞれの出力（封筒の `id` か `type` が違う、メンバーが欠ける、型が違う、重なる、`tab_id` が重なる、chief の pane の項目が 0 か 2 つ、幅が 0・負・`int` を超える、`error` を持つ、後ろにデータがある）を `errHerdrOutput` で拒み、一部だけを使わないこと（AC-23）。

テストは、守るものを外して失敗することを確かめてからコミットする（CLAUDE.md の「Every test must be able to fail for its stated reason」）。

### 7.2. 結合テストと文書のガードテスト

-   **ラウンド（`snapshot_test.go`・`tick` のテスト）。** テストの記録係で、`tab list` と `pane layout` が失敗する・形に合わないラウンドでも、ラウンドが失敗せず、行の状態・急ぎの度合い・`pending`・`problems` が、読めたときと同じであること（AC-05・AC-19、要件書 4.3）。`tick` の自動送信の後の行が表に出ること。
-   **文書のガードテスト。** `TestDocumentedProtocolMatches` と `TestGuideValuesMatchTool` は、定義とガイドの番号を道具の番号と比べる（AC-25）。定義の新しい項目 2 の文（AC-03）、ガイドの 3.1 と 8 章（AC-24）、CLAUDE.md の例外（AC-26・AC-28）は文の中身なので、PR のレビューで読んで確かめる（CLAUDE.md の「Document tests check values, structure, and executable examples, never prose」）。

### 7.3. セキュリティのテストと手で確かめること

-   **code block を閉じないこと（AC-22）。** 名前・PR の題名・質問の文・タブのラベル・送れない理由・読めなかったものに、```` ``` ````・`~~~`・空白で始まる値・`#`・`|`・`-` で始まる値・改行のあとの ```` ``` ```` を入れた表を組み、どの行も 0〜3 桁の空白に続く ```` ``` ```` か `~~~` で始まらないこと、どの行も決まった語か 4 桁以上の空白で始まることを確かめる。
-   **端末の制御文字（AC-21）。** ESC・C1 制御文字・ゼロ幅の文字・正しくない UTF-8 を含む値が、表の中でエスケープとして現れること。
-   **手で確かめること（計画で手順を決める）。** 実物の `/chief` で、(1) code block の右端で折り返す桁（1.2）、(2) 全角・絵文字・`…`・`─` を含む表の列が実際にそろうこと、(3) 幅の狭い pane と 80 桁とみなしたときの表の形を、Claude Code の画面で確かめる。(1) で右端が pane の幅より手前だと分かれば、使える幅の値を直す。

## 8. 実装の優先順位 (Implementation Priorities)

計画で PR の境目と順を決める。依存の順は次のとおりである。

1.  実物の確かめ: code block の右端の桁（1.2）。`herdr tab list` と `herdr pane layout` のサンプルを採る。
2.  表示幅と表示の単位（`textwidth.go`、`sanitize.go` の分割）。
3.  Herdr の読み取り（`herdr.go` の 3 つの読み取り、`readRound` の表の材料）。
4.  表を組む（`table.go`、`finishSnapshot`、出力の `table`・`table_unreadable`、JSON のエスケープ）。
5.  `Protocol` を上げ、`/chief` の定義、ガイド、CLAUDE.md を同じ PR で直す。定義が表を前提にするので、道具が表を出力する変更と、定義が表を示す変更は、同じ番号で入る必要がある。`TestDisplayValuesMatchTool` を除くコミットは単独にし、`make coverage-compare` で確かめる。
6.  手で確かめること（7.3）。

## 9. 将来の拡張 (Future Extensibility)

-   **モデル名の短縮と経過時間の列**（要件書 2.3）は、内容の列の部品か、新しい列として足せる。列の幅の規則（見出しと値の最も広いもの）はそのまま使える。
-   **人の判断を待つものの一覧や、表の下の項目**を道具が組むことにするときも、表示の単位・表示幅・折り返しの部品をそのまま使える。
-   **Herdr がタブの並びの位置を返すようになれば**、タブの表示に番号を足せる（1.2 で採らなかった理由が解ける）。そのときは要件を変える。

## 付録 A: 申し送りへの対応

| 項目 | 対応 |
|---|---|
| H-01 道具の出力に足す表の形 | 行ごとの文字列の配列 `table` にする（1.2・3.1）。各要素は改行を含まず、末尾に空白を持たない。定義は要素を 1 行ずつ code block に並べる（3.9） |
| H-02 表示幅の判定の規則 | W・F は 2、Mn・Me は 0、曖昧な文字を含むほかは 1 とし、W・F の範囲は `unicode.Version` と同じ版の表を持つ（1.2・3.5）。`…` と `─` は曖昧な文字のまま使い、Claude Code が `─` を 1 桁で描くことを確かめた。エスケープは 1 つの単位で、その途中で折り返しも切りもしない（3.6） |
| H-03 code block の余白と内容の列の最小の幅 | 左の余白は 2 桁（2026-11-18、Claude Code 2.1.327 で確かめた）。右端は実装の最初に確かめる（1.2・7.3）。列の幅は見出しと値の最も広いもの、間は空白 2 つ、内容の列の最小は 20 桁（1.2・3.2） |
| H-04 Herdr のタブの一覧と pane の配置の読み方 | 3.7 のとおり読む。`type` は `tab_list` と `pane_layout`。幅は chief の pane の `rect.width`。配置を読む pane は、ラウンドが確かめた chief の pane。`number` は画面の番号と対応しないので使わず、タブの表示をラベルだけにし、要件書の AC-18 を改めた（1.2）。サンプルは計画の最初のステップで `internal/chief/testdata/herdr/` に採る（8 章） |
| H-05 code block を閉じないことの保ち方 | 行の組み方で保つ（1.2・3.2）。値に囲いの文字を含む表のテストで確かめる（7.3）。長い囲いは使わない |
| H-06 行がないときの形と細部 | 行がないときは `workspace なし` を置く。読めなかったものは `, ` で結ぶ。ラベルが空のタブは pane の ID を示す。語と ASCII でない文字の境目でも折り返してよく、全角の空白は空白に含めない。表にない値は `?` と示す（1.2・3.3・3.4・3.6） |
| H-07 日本語の literal を置くファイル | `internal/chief/table.go` とそのテストだけに置く。CLAUDE.md の例外はこのファイルを名指す（1.2・3.11） |

## 付録 B: 受け入れ基準との対応

| AC | 設計 |
|---|---|
| AC-01 | 3.1・3.8（`snapshot` と `tick` の出力の `table`。並びは `rows` と同じ） |
| AC-02 | 1.1・3.1・3.8（同じ `Row` から組み、`rows` は変えない） |
| AC-03 | 3.9 |
| AC-04 | 1.2・3.2（pane の幅 − 2） |
| AC-05 | 3.2・3.7・4 章（80 桁、最小 20 桁） |
| AC-06 | 3.2 |
| AC-07 | 3.2・3.5 |
| AC-08 | 3.2・3.6 |
| AC-09 | 3.2 |
| AC-10 | 1.2・3.2（`workspace なし`） |
| AC-11 | 3.3 |
| AC-12 | 3.3 |
| AC-13 | 3.3 |
| AC-14 | 3.4 |
| AC-15 | 3.5・3.6 |
| AC-16 | 3.4 |
| AC-17 | 3.4・3.5 |
| AC-18 | 1.2・3.4（ラベルだけ。要件書を改めた） |
| AC-19 | 3.4・3.7 |
| AC-20 | 3.3 |
| AC-21 | 3.4・3.5・5.2 |
| AC-22 | 1.2・3.2・5.2・7.3 |
| AC-23 | 3.7・7.1 |
| AC-24 | 3.10 |
| AC-25 | 3.8・3.9・3.10 |
| AC-26 | 3.11 |
| AC-27 | 要件定義のときに ADR-0046 を更新済み（1.2 の末尾） |
| AC-28 | 3.11 |
