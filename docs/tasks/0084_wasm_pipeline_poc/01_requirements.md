# 要件定義書：Go の WASM 版のパイプラインを拡張のページで動かす PoC と、構成の判断に要る値の測定

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Process | `full` |
| Created | 2026-11-11 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、記事を作る Go のパイプラインを WASM にビルドし、ブラウザ拡張のページで動かす PoC の要件を定める。測るのは、拡張だけの構成の設計（[#772](https://github.com/isseis/yt2column/issues/772)）で使う値である。成果は 2 つある。1 つは、`main` に入れる小さな変更で、パイプラインのコアを WASM でビルドできるようにする。もう 1 つは、測った値と 3 つの問いへの答えを `docs/research/` の下に記録する。PoC のコードそのものは `main` にマージしない。

**背景:**

[#769](https://github.com/isseis/yt2column/issues/769) は、拡張だけで記事を作る構成への移行を方針としている。この構成では、localhost サーバと Native Messaging のホストを廃止し、利用者が自分の Anthropic API キーを拡張に設定する。記事を作る処理は Go のコアを WASM にビルドして拡張から使い、CLI と共有する案である。#772 でこの構成を決めるには、次の 3 つの問いに実測で答える必要がある（#771）。

1. Go の WASM 版のパイプラインは、拡張のページで現実的に動くか。WASM のサイズ、読み込みの時間、`net/http` が `fetch` に変換されて Claude API を呼べるかを確かめる。
2. 生成をビューアのタブで実行できるか。数分かかる生成が、背面のタブのまま完走するか、メモリセーバーなどによるタブの破棄を受けないかを確かめる。
3. ストリーミングは要るか。ストリーミングなしで、本番の `max_tokens` 40,000 の応答を受け取りきれるかを確かめる。

今の構成では、ビューアは生成の開始と同時に背面のタブで開く（[ADR-0009](../../dev/adr/0009-viewer-opening.md)）。生成はサーバが行い、ビューアは状態を問い合わせるだけである。Claude アダプタの出力の上限の既定は 40,000 トークンで（`internal/llm/claude/request.go:31`）、ストリーミングを使わない（[ADR-0025](../../dev/adr/0025-claude-output-limit.md)）。

いまのコアは WASM でビルドできない。`GOOS=js GOARCH=wasm go build` を 2026-11-10 に試した結果は、次のとおりである。

-   `internal/transcript` は、`internal/transcript/exec.go:99` の `syscall.SysProcAttr{Setpgid: true}` で失敗する（yt-dlp を起動する経路の一部）。
-   `internal/writer` は、`internal/transcript` を import するので失敗する。さらに、`internal/writer/template.go:38` の `syscall.O_NONBLOCK`・`syscall.O_NOCTTY` でも失敗する（上書き用のテンプレートのファイルを開くフラグ）。issue #771 は「`internal/writer` はそのままビルドできることを確認済み」としているが、この 2 つ目の原因は issue の確認から漏れていた。
-   `internal/pricing` は `internal/writer` に依存するので、同じ理由で失敗する。
-   `internal/llm/claude` は、そのままビルドできる。
-   `internal/transcript` から yt-dlp の経路のファイル（`exec.go`・`ytdlp.go`・`cache.go`・`info.go`・`json3.go`）を外し、`template.go` のフラグを `os.O_RDONLY` だけにすると、上の 4 つのパッケージはどれも WASM でビルドできた（`go build -overlay` による試行）。

対応 issue: [#771](https://github.com/isseis/yt2column/issues/771)（親: [#769](https://github.com/isseis/yt2column/issues/769)、後続: [#772](https://github.com/isseis/yt2column/issues/772)・[#773](https://github.com/isseis/yt2column/issues/773)）

**本書の記述範囲:** 本書は、`main` に入れる変更の観測できる振る舞い（何が WASM でビルドでき、CLI とサーバの何が変わらないか）を定める。PoC で行う操作、測る値、調査記録に残す内容も定める。次のものは、設計（`02_architecture.md`）で決める。

-   パッケージの分け方（パッケージの分割かビルドタグか）。
-   WASM の入口の関数の形。
-   拡張のページから API を呼ぶ方法（CORS・権限・ヘッダ）。
-   WASM の読み込みと測定の手段。
-   API キーを貼り付ける画面の形。

要件のレビューで挙がった設計事項は [design_handoff.md](design_handoff.md) に申し送る。

**issue #771 で決めたこと:**

-   **`main` に入れるのは分割だけにする。** PoC のコードは使い捨てにし、`main` にマージしない。`internal/transcript` の分割は本番でも要るので、別の小さな PR として `main` に入れる。
-   **結果はリポジトリに残す。** 調査記録を `docs/research/` の下に置き、#772 の ADR の根拠として引用する。
-   **WASM の入口を 1 つ作る。** 拡張のページから呼ぶ入口が、字幕・メタ情報・API キーを受け取り、記事を返す。ビューアが WASM を読み込み、3 つの工程（生成・字数の調整・検証・推敲）を実行する。
-   **API キーはメモリにだけ持つ。** 開発用に、API キーをその場で貼り付ける。メモリにだけ持ち、保存しない。
-   **測るもの:** WASM のサイズと読み込みの時間、工程ごとの所要時間を測る。背面のタブで完走するかを、`chrome://discards` による破棄の試行も含めて確かめる。タブを閉じたときに中止されるかも確かめる。
-   **API の費用:** 仕組みの確認と繰り返しの試行には `claude-haiku-5-5` を使い、Haiku が `effort` を受け付けるかも確かめる。本番の構成（`claude-opus-5-5+high`）を使うのは、成功の基準の確認だけにする。本番の構成での実行は、3 本の動画で 1 回ずつ（約 1.1 USD）を基本とし、全体で約 4 USD を上限の目安とする。実 API の呼び出しは、その都度承認を得る。
-   **成功の基準:** 本番の構成で、3 本の動画のすべてについて、背面のタブのまま記事が完成し、CLI と同じ形式の記事が出る。
-   **含めないもの:** API キーの保存と設定の UI、ストリーミングの実装、Offscreen Document の実装、TS 版との比較の実装、Web Store への対応、アクセシビリティと多言語への対応は含めない。ストリーミングは、問い 3 の答えが「要る」なら #772 の設計の範囲に入れる。Offscreen Document は、問い 2 が失敗したときだけ試す。

**要件定義の際に決めたこと（2026-11-11）:**

-   **進め方は full とする。** 変更は 3 つの部品（`internal/transcript`・`internal/writer`・拡張）にまたがる。新しい仕組み（WASM の入口、ビューアでの読み込み）を作り、決めることも多いためである。除外の判断は「除外」とした。`main` に入れるのは分割と調査記録だけで、セキュリティの規則を変えない。しかし PoC は、拡張のページに API キーを持たせ、拡張から外部のサービス（Anthropic API）を直接呼ぶ。これは [security.md](../../dev/security.md) §2・§8 の前提（API キーは CLI の環境変数かサーバの設定ファイルだけにある）に触れるので、疑わしい変更として除外と判断した。人は full を選び、除外の判断を覆していない。ADR: なし（このタスクの進め方で、タスクの外に及ばない）。
-   **`internal/writer` の修正も、`main` に入れる分割の PR に含める（F-001）。** issue は `internal/transcript` の分割だけを挙げている。しかし、背景のとおり `internal/writer/template.go:38` も WASM でビルドできない。本番の WASM 化でも必ず要る変更で、規模も小さいためである。ADR: なし。WASM でビルドできることを規則として守り続けるかは #772 が決める（次の決定）。
-   **WASM でビルドできることを守る検査は加えない（§2.3）。** `make test`・`make lint`・CI に、`GOOS=js GOARCH=wasm` のビルドの検査を加えない。加えると「コアは WASM でビルドできる」が後のタスクを縛る規則になる。Go の WASM にするか TS に一本化するかは #772 でまだ決まっていないので、その判断を先取りしないためである。ADR: なし。検査を加えるなら #772・#773 が ADR とともに決める。
-   **PoC のコードは origin のブランチに残す（F-005）。** PoC のブランチを origin に push してマージせずに残し、調査記録にブランチ名とコミットの SHA を書く。測定を再現できるようにし、かつ `main` に PoC のコードを入れないためである。ADR: なし。
-   **問い 2 が失敗したら、このタスクの中で Offscreen Document を試す（F-004）。** 背面のタブで完走しない、または破棄を受ける場合に限り、同じ測定を Offscreen Document で行い、同じ調査記録に書く。#772 の判断には、代わりの案の値も要るためである。ADR: なし。
-   **測るブラウザは Chrome と Brave の両方とし、本番の構成での実行は Chrome だけにする（F-003・F-004）。** 対象のブラウザは Chrome と Brave である（[ADR-0026](../../dev/adr/0026-extension-platform.md)）。タブの破棄の振る舞いはブラウザごとに違いうるので、Haiku での試行は両方で行う。本番の構成での 3 本は、費用を抑えるため Chrome だけで行う。ADR: なし。
-   **問い 3 は、Haiku で上限の近くまで出力させる長い応答を足して確かめる（F-004）。** 0018 の評価では、10 件の出力が最大でも 13,688 トークンだった（[production_profile.md](../../dev/production_profile.md)「品質の目標と結果」）。本番の 3 本だけでは、40,000 トークンの近くの長さを確かめられない見込みであるためである。ADR: なし。
-   **本番の構成で試す動画は、0008 の評価の 5 本から、字幕の最も長い動画 1 を含む 3 本とし、CLI は走らせずに記事の構成で形式を判定する（F-004）。** 評価の 5 本は [0008 評価記録](../0008_column_prompt/02_evaluation.md) §3.2 にある。最も長いのは動画 1（`roNZ-rLxhUE`、32:29）で、応答が長くなりやすく、問い 2・3 の条件として厳しい。形式は、記事の構成（§3.1 F-004 の AC-17）を CLI の出力の構成と見比べて判定する。同じ 3 本を CLI でも生成すると約 1.1 USD 増え、得られるのは同じコード（`internal/writer`）の出力の比較だけであるためである。ADR: なし。
-   **PoC に貼り付ける API キーは、テスト用の API キーとする（§4.2）。** テスト用の API キーとそのワークスペース ID は、`YT2COLUMN_TEST_ANTHROPIC_API_KEY`・`YT2COLUMN_TEST_ANTHROPIC_WORKSPACE_ID` に割り当てているものである。そのワークスペースには利用の上限がある（[security.md](../../dev/security.md) §2）。漏れたときや暴走したときの被害を上限で抑え、issue の費用の目安も上限で守るためである。ADR: なし（例外は PoC のブランチの中に限り、`main` の規則を変えない）。

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   #772 が構成を決めるのに要る 3 つの問いに、実測の値で答える。
-   パイプラインのコア（コピーした字幕の取得、記事の生成、Claude のアダプタ、料金の概算）を、CLI とサーバの振る舞いを変えずに WASM でビルドできるようにする。
-   測定の結果と条件を、#772 の ADR が根拠として引用できる形でリポジトリに残す。

### 2.2. スコープ (In Scope)

-   `main` に入れる変更（F-001）。`internal/transcript` を、コピーした字幕の経路（`CopiedSource`）と yt-dlp の経路に分け、`internal/writer` の上書き用のテンプレートのファイルを開く処理を分けて、コアを WASM でビルドできるようにする。
-   PoC のブランチだけに置く変更（F-002・F-003）。WASM の入口を作り、ビューアから読み込んで 3 つの工程を実行できるようにする。
-   測定（F-004）。測定は、Chrome と Brave での Haiku の試行と、Chrome での本番の構成の 3 本からなる。問い 2 が失敗した場合に限り、Offscreen Document での測定も行う。
-   調査記録（F-005）。`docs/research/` の下に置き、`main` に入れる。

### 2.3. スコープ外 (Out of Scope)

-   API キーの保存と設定の UI。issue #771 で決めたとおりである。
-   ストリーミングの実装。issue #771 で決めたとおりで、問い 3 の答えが「要る」なら #772 の設計の範囲に入れる。
-   問い 2 が成功した場合の Offscreen Document の実装。issue #771 で決めたとおりで、失敗した場合の試行はスコープに含める（F-004）。
-   TS 版との比較の実装、Web Store への対応。issue #771 で決めたとおりである。
-   PoC の画面のアクセシビリティと多言語への対応。issue #771 で決めたとおりである。PoC の画面は `main` にマージせず利用者に届かないので、[CLAUDE.md](../../../CLAUDE.md)「Accessibility」の例外（機能を確かめるためだけの最小限の UI）の手続き（issue の作成と、PR の説明への記載）の対象にもならない。
-   WASM でビルドできることを守る検査（`make` の目標や CI）。要件定義の際に決めたとおり、#772・#773 が決める。
-   同じ 3 本を CLI でも本番の構成で生成して比べること。要件定義の際に決めたとおりである。
-   本番の構成での Brave の測定。要件定義の際に決めたとおりで、調査記録に「確かめていない事項」として書く（AC-24）。
-   localhost サーバ・Native Messaging のホスト・今の拡張の生成の経路の変更。`main` の拡張・サーバ・ホストは変えない。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: パイプラインのコアを WASM でビルドできるようにする（`main` に入れる）

`internal/transcript` を、コピーした字幕の経路と yt-dlp の経路に分ける。`internal/writer` は、上書き用のテンプレートのファイルを開く処理を分ける。これにより、コピーした字幕を解析する部分・記事の生成・Claude のアダプタ・料金の概算を、`GOOS=js GOARCH=wasm` でビルドできるようにする。CLI とサーバの振る舞いは変えない。この変更は PoC と別の PR として `main` に入れる。

**Acceptance Criteria**:

-   **AC-01**: 次のパッケージが、`GOOS=js GOARCH=wasm go build` でエラーなくビルドできる。
    -   `CopiedSource` と、それが返す `Transcript` の型を持つパッケージ。
    -   `internal/writer`。
    -   `internal/llm/claude`。
    -   `internal/pricing`。
-   **AC-02**: AC-01 のパッケージの依存（`go list -deps` で得られるもの。`GOOS=js GOARCH=wasm` を指定する）に、yt-dlp を起動するコード（`os/exec` を使うパッケージ）が含まれない。
-   **AC-03**: `GOOS`・`GOARCH` を指定しない通常のビルドで、`make test`・`make lint`・`make deadcode` が通る。
-   **AC-04**: 既存のテストのファイルの差分は、次のものに限られる。
    -   ファイルの移動。
    -   パッケージ名の行と import の行。
    -   移動に伴う、パッケージの名前で修飾した識別子の書き換え。
    -   ビルドの制約の行。

    テストの関数の削除と、アサーションの変更はない。
-   **AC-05**: CLI（`cmd/yt2column`）とサーバ（`cmd/yt2column-server`）の振る舞いは変わらない。次の 3 つが今と同じに動く。
    -   上書き用のテンプレートのファイルの読み込み。FIFO で止まらないことと、端末を制御端末にしないことを含む。
    -   yt-dlp による字幕の取得とキャッシュ。
    -   コピーした字幕の解析。

    今の振る舞いを確かめる既存のテストが、AC-04 の範囲の変更だけで通ることで確かめる。
-   **AC-06**: `.golangci.yml` の `depguard` の `deps` の規則に、モジュールを加えない（標準ライブラリの外のモジュールを増やさない）。

#### F-002: WASM の入口（PoC のブランチだけ）

拡張のページから呼べる WASM の入口を 1 つ作る。入口は、次の値を受け取る。

-   コピーした字幕の文字列。
-   動画の URL・タイトル・チャンネル名・概要欄。
-   API キーとワークスペース ID。
-   3 つの工程のそれぞれのモデルと effort。

入口は、CLI と同じコード（`internal/writer`）で 3 つの工程を実行し、記事を返す。PoC のブランチにだけ置き、`main` にマージしない。

**Acceptance Criteria**:

-   **AC-07**: 入口は、生成・字数の調整・検証・推敲の 3 つの工程を、CLI と同じ順と同じ条件で実行する。字数の調整は、生成した本文が長すぎるときだけ実行する。入口は、CLI が出力する記事と同じ文字列の形（`internal/writer` が組み立てる Markdown）を返す。
-   **AC-08**: どの工程の失敗でも、入口は失敗を返し、記事を返さない。失敗には、字幕の拒否、API の失敗、出力の打ち切り、空の応答、検証・推敲の応答の拒否を含む。失敗は、`internal/writer`・`internal/llm` の番兵で分かる種類として、呼び出し元の JavaScript に返す。途中の工程までの記事を、成功として返さない。
-   **AC-09**: 入口は、工程ごとに次の値を呼び出し元に返す。
    -   開始から終了までの所要時間。
    -   モデル名。
    -   入力と出力のトークン数。
    -   料金の概算（`internal/pricing` が出すもの）。

    失敗した工程も、分かる範囲で同じ値を返す。

#### F-003: ビューアでの実行（PoC のブランチだけ）

ビューアが WASM を読み込み、利用者が貼り付けた API キーで、入口（F-002）を実行する。

**Acceptance Criteria**:

-   **AC-10**: ビューアは、WASM を読み込み、API キーとワークスペース ID の入力欄を表示する。生成は、利用者がボタンを押したときだけ始まる。
-   **AC-11**: API キーとワークスペース ID は、ビューアのタブのメモリにだけ持つ。次のどれにも書かない。
    -   `chrome.storage`（`local`・`session`・`sync`）。
    -   `localStorage`・`sessionStorage`・IndexedDB・Cookie。
    -   ページの URL。
    -   コンソールと開発者向けのログ。
    -   エラーの表示。

    タブを閉じると、どこにも残らない。
-   **AC-12**: 生成の結果として、成功したときは記事を、失敗したときは失敗の種類を表示する。工程ごとの値（AC-09）と、WASM の読み込みの時間も表示する。記事は、今のビューアと同じく untrusted のテキストとして描画する（[security.md](../../dev/security.md) §8.1「記事を untrusted テキストとして描画する」）。
-   **AC-13**: 生成は、ビューアのタブの中だけで実行する。タブを閉じた後に、Service Worker など別の場所で生成を続けない。タブを閉じたときに API へのリクエストが中止されるかは、F-004 で観測して記録する（AC-16）。

#### F-004: 測定

PoC で次の値を測る。実 API を呼ぶ測定は、その都度、人の承認を得てから行う。

**Acceptance Criteria**:

-   **AC-14**: 問い 1 について、次の値を Chrome と Brave で測る。
    -   WASM のファイルのサイズ。圧縮しないものと gzip で圧縮したもの。
    -   ビューアを開いてから入口を呼べるようになるまでの読み込みの時間。3 回以上測り、各回の値を記録する。
    -   `claude-haiku-5-5` で、3 つの工程が WASM から Claude API を呼んで完了するか。
    -   Haiku が `effort` を受け付けるか。受け付けない場合は、その応答（状態コードとエラーの種類）も記録する。
-   **AC-15**: 問い 2 について、Chrome と Brave で、`claude-haiku-5-5` を使い次の試行を行う。
    -   ビューアを背面のタブにしたまま、3 つの工程が完走するか。
    -   生成の途中で、ブラウザのタブの破棄の操作（Chrome は `chrome://discards`、Brave はそれに当たる画面）で破棄を試みたとき、タブが破棄されるか。破棄されたら、生成がどうなるか。
    -   メモリセーバーを有効にした状態で、背面のまま完走するか。
-   **AC-16**: 生成の途中でビューアのタブを閉じたときに、API へのリクエストが中止されるかを、Chrome と Brave で観測する。
-   **AC-17**: 問い 3 について、次の 2 つを測る。
    -   本番の構成の 3 本（AC-18）の各工程の、出力のトークン数と所要時間。
    -   `claude-haiku-5-5` で、`max_tokens` 40,000 の上限の近くまで出力させる、ストリーミングなしの応答 1 回。応答を受け取りきれたか、出力のトークン数、所要時間、終了の理由を記録する。
-   **AC-18**: 成功の基準を、本番の構成（3 つの工程とも `claude-opus-5-5`、effort `high`）で、Chrome で確かめる。動画は、0008 評価記録 §3.2 の 5 本から、動画 1（`roNZ-rLxhUE`）を含む 3 本とし、各 1 回生成する。入力の字幕は、本番の経路と同じく、文字起こしパネルからコピーしたものである。各回について、次の 3 点を記録する。
    -   ビューアを生成の開始から完了まで背面のタブにしたままで、記事が完成したか。
    -   記事が CLI の出力と同じ構成を持つか。構成とは、1 行目のレベル 1 の見出しのタイトル、本文、出典の段落（`出典: ` と動画の URL）である（`internal/writer/output.go`）。
    -   料金の概算。

    3 本のすべてで前の 2 点を満たせば、成功の基準を満たす。
-   **AC-19**: 問い 2 が失敗した場合に限り、同じ試行（AC-15・AC-16）を Offscreen Document で生成を実行する形で行う。対象は、失敗したブラウザである。失敗とは、AC-15 のどれかで完走しない場合と、破棄によって生成が失われる場合である。
-   **AC-20**: 費用の見込みが、全体で約 4 USD の目安を超えそうになったら、超える前に測定を止め、人に続けるかを尋ねる。

#### F-005: 調査記録

測定の結果を、`docs/research/` の下の日本語の文書に残し、`main` に入れる。PoC のブランチは origin に残す。

**Acceptance Criteria**:

-   **AC-21**: 調査記録は、3 つの問いのそれぞれに答えを書く。答えは「はい」「いいえ」「条件つき」のどれかとし、その根拠として AC-14〜AC-19 の測定の値を挙げる。
-   **AC-22**: 調査記録は、測定の条件を書く。条件は次のとおりである。
    -   ブラウザの名前と版、OS と版、測った日。
    -   Go の版。
    -   使ったモデルと effort。
    -   PoC のブランチの名前とコミットの SHA。
    -   動画の ID。
    -   実行ごとの料金の概算と、その合計。
-   **AC-23**: PoC のブランチは origin に push され、`main` にマージされない。AC-22 のコミットは、そのブランチから辿れる。
-   **AC-24**: 調査記録は、確かめていない事項を挙げる。少なくとも次の 2 つを含む。
    -   本番の構成での Brave の測定。
    -   問い 2 が成功した場合の Offscreen Document。
-   **AC-25**: 調査記録には、API キー、ワークスペース ID、記事の本文、字幕の本文を書かない。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   WASM のサイズと読み込みの時間には、合否の閾値を設けない。値を測って記録し（AC-14）、現実的かどうかは #772 が判断する。

### 4.2. セキュリティ (Security)

[security.md](../../dev/security.md) を当てはめる。本タスクは、API キーを扱い、LLM API へデータを送り、字幕と動画メタデータ（untrusted のテキスト）をプロンプトに入れるので、条件つきのガイドの対象である。

-   **`main` に入れる変更（F-001）は、セキュリティの規則を変えない。** 上書き用のテンプレートのファイルを開くときの守り（FIFO で止まらない・端末を制御端末にしない）を、通常のビルドで保つ（AC-05）。yt-dlp の起動の守り（[security.md](../../dev/security.md) §1）も変えない。
-   **PoC の API キーは、テスト用の API キーとする。** テスト用の API キーとそのワークスペース ID（[security.md](../../dev/security.md) §2。利用の上限のあるワークスペースのもの）を使い、本番の `ANTHROPIC_API_KEY` は使わない。人がビューアに貼り付け、メモリにだけ持ち、どこにも書かない（AC-11・AC-25）。
-   **方針の例外（PoC のブランチの中だけ）:** PoC のブランチは、`main` の次の方針と異なる。どれも PoC のブランチの中に限り、`main` にはマージしない。測定のために、拡張のページから API を呼ぶ必要があるためである。
    -   [security.md](../../dev/security.md) §2：API キーは CLI では環境変数から、サーバでは設定ファイルからだけ読む。PoC は、ビューアに貼り付けた値を使う。
    -   [ADR-0010](../../dev/adr/0010-extension-stores-nothing.md)：拡張は権限を加えない。PoC は、Anthropic API を呼ぶのに要る権限やヘッダを加えうる。その手段は設計で決める。
    -   [ADR-0026](../../dev/adr/0026-extension-platform.md)：成果物は自前のコードだけにする。PoC の成果物には、Go のツールチェーンの `wasm_exec.js` と、ビルドした WASM が入る。
    -   [security.md](../../dev/security.md) §8.1：拡張のページの既定の Content Security Policy を緩めない。PoC は、WASM を動かすのに要る `'wasm-unsafe-eval'` を加えうる。
-   **記事は untrusted のテキストとして描画する（AC-12）。** PoC でも、今のビューアと同じ描画（[ADR-0017](../../dev/adr/0017-viewer-markdown-rendering.md)）を使う。
-   **LLM に送るデータは今と同じにする。** 工程ごとに送るものは [security.md](../../dev/security.md) §4 のとおりで、API キーをプロンプトに入れない。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   入口は、失敗を成功として返さない（AC-08）。空の応答と打ち切りは失敗である（[CLAUDE.md](../../../CLAUDE.md)「Empty is an error」）。
-   PoC は、自動で API に送り直さない（[ADR-0028](../../dev/adr/0028-no-automatic-resend.md) と同じく、結果が分からないときに費用を重ねないため）。

### 4.4. 互換性 (Compatibility)

-   CLI とサーバの振る舞いを変えない（AC-03〜AC-05）。
-   PoC は、Chrome と Brave で動かす（[ADR-0026](../../dev/adr/0026-extension-platform.md)）。版は測定の条件として記録する（AC-22）。

### 4.5. 保守性 (Maintainability)

-   F-001 は、新しいモジュールを加えない（AC-06）。
-   F-001 は、[ADR-0019](../../dev/adr/0019-pipeline-component-interfaces.md) の決定を変えない。構成要素ごとのパッケージが interface と入出力の型を持ち、`internal/pipeline` が interface だけに依存する、という決定である。分けた後にどのパッケージが何を持つかは、設計で決め、[package_reference.md](../../dev/conventions/package_reference.md) に反映する。

## 5. 制約条件 (Constraints)

-   [project_overview.md](../../dev/project_overview.md) の「決定済みの方針」「前提・制約」に従う。
-   実 API の呼び出しは、その都度、人の承認を得る（[CLAUDE.md](../../../CLAUDE.md)「Tool Execution Safety」と issue #771）。
-   本番の構成での実行は 3 本で 1 回ずつを基本とし、全体の費用は約 4 USD を目安とする（AC-20）。
-   PoC のコードは `main` にマージしない。`main` に入れるのは F-001 の変更と F-005 の調査記録だけである。

## 6. 用語集 (Glossary)

用語は [docs/translation_glossary.md](../../translation_glossary.md) と統一する。

-   **WASM:** WebAssembly。本書では、Go のコードを `GOOS=js GOARCH=wasm` でビルドしたものを指す。
-   **入口:** 拡張のページの JavaScript から呼ぶ、WASM の側の関数（F-002）。
-   **PoC のブランチ:** PoC のコードを置き、`main` にマージしないブランチ（F-005）。
-   **本番の構成:** 3 つの工程とも `claude-opus-5-5`、effort `high` の構成（[production_profile.md](../../dev/production_profile.md)）。
-   **3 つの工程:** 生成・字数の調整・検証・推敲（[ADR-0002](../../dev/adr/0002-post-generation-steps.md)）。
-   **調査記録:** `docs/research/` の下に置く、測定の結果の文書（F-005）。
-   **テスト用の API キー:** 本番とは別に用意し、利用の上限のあるワークスペースに割り当てた Anthropic API のキー（[security.md](../../dev/security.md) §2）。
