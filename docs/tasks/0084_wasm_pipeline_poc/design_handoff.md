# 設計への申し送り：Go の WASM 版のパイプラインの PoC

## 本書の位置づけ

[01_requirements.md](01_requirements.md) の作成の過程で挙がった事項のうち、要件（観測できる振る舞い）ではなく、設計で決める事項や、設計で注意すべき事項をまとめる。`02_architecture.md` は、各項目について、採った手段か、扱わない理由を記録する。本書の手段は候補であり、設計で別の手段を選んでもよい。ただし、対応する AC を満たすこと。

## 項目一覧

| ID | 対象 | 概要 | 関連 AC |
|---|---|---|---|
| H-01 | F-001 | `internal/transcript` の分け方 | AC-01・AC-02・AC-04 |
| H-02 | F-001 | WASM のビルドで上書き用のテンプレートを指定されたとき | AC-01・AC-05 |
| H-03 | F-002・F-003 | 拡張のページから Anthropic API を呼ぶ方法 | AC-07・AC-14 |
| H-04 | F-002 | `fetch` に変換された `net/http` での、`internal/llm/llmhttp` の守り | AC-08 |
| H-05 | F-003 | PoC の成果物と CSP | AC-10 |
| H-06 | F-003 | 字幕をビューアに渡す方法 | AC-10・AC-18 |
| H-07 | F-002・F-003 | WASM の実行がビューアの操作を止めないか | AC-10・AC-15 |
| H-08 | F-004 | 測定の手段 | AC-14〜AC-17 |

## H-01: `internal/transcript` の分け方

**懸念:** `CopiedSource` と yt-dlp の経路を分ける手段には、パッケージを分ける案と、ビルドタグで yt-dlp の経路のファイルを `js` から外す案がある。ビルドタグでは、`GOOS=js` 以外のビルドの依存に yt-dlp の経路が残り、AC-02 の「依存に含まれない」をパッケージの単位で示せない。パッケージを分けると、`cmd/yt2column`・`internal/pipeline`・`internal/job` などの import が変わり、[package_reference.md](../../dev/conventions/package_reference.md) の表も直す必要がある。

**理由:** [ADR-0019](../../dev/adr/0019-pipeline-component-interfaces.md) は、`TranscriptSource` と `Transcript` を `internal/transcript` に置くと定める。分けた後もこの決定を変えないためには、型と interface を残すパッケージを決める必要がある。

**候補:** `internal/transcript` に、型・interface・`CopiedSource` を残す。yt-dlp の経路（`exec.go`・`ytdlp.go`・`cache.go`・`info.go`・`json3.go`・`video_id.go` のうち yt-dlp だけが使うもの）は、新しいパッケージに移す。

**関連:** F-001、AC-01・AC-02・AC-04。

## H-02: WASM のビルドで上書き用のテンプレートを指定されたとき

**懸念:** WASM にはファイルシステムがないので、上書き用のテンプレートのファイルを開く処理は `js` では意味を持たない。上書きのパスを受け取ったときに、黙って埋め込みの既定を使うと、[CLAUDE.md](../../../CLAUDE.md)「Reject, don't normalize」に反する。

**理由:** 呼び出し側が上書きを指定したつもりで、別のテンプレートで記事が作られるおそれがある。

**候補:** `js` のビルドでは、上書きのパスを受け取ったら番兵のエラーで拒否する。または、上書きを受け取る経路そのものを `js` のビルドから外す。

**関連:** F-001、AC-01・AC-05。

## H-03: 拡張のページから Anthropic API を呼ぶ方法

**懸念:** 拡張のページの `fetch` で `api.anthropic.com` を呼ぶには、CORS を通す必要がある。手段には、manifest の `host_permissions` に `https://api.anthropic.com/*` を加える案と、Anthropic API がブラウザからの直接の呼び出しに求めるヘッダ（`anthropic-dangerous-direct-browser-access`）を送る案がある。後者は `internal/llm/claude` の送るヘッダを変える。

**理由:** どちらを選ぶかで、#772 が決める権限（`host_permissions` の追加）とアダプタの変更の要否が変わる。PoC で両方の振る舞いを観測できれば、#772 の判断の材料になる。

**候補:** PoC では `host_permissions` を加える案を基本とし、ヘッダの案も試せるなら観測して調査記録に書く。

**関連:** F-002・F-003、AC-07・AC-14。

## H-04: `fetch` に変換された `net/http` での、`internal/llm/llmhttp` の守り

**懸念:** Go の `js/wasm` の `net/http` は、ブラウザの `fetch` で送る。そのため、`internal/llm/llmhttp` の守りが、ネイティブのビルドと同じに働くとは限らない。守りとは、リダイレクトの拒否（`CheckRedirect`）、本文の上限、失敗の分類である。たとえば、`fetch` の既定はリダイレクトに従う。また、タイムアウトと `context` のキャンセルが、`fetch` の中止（`AbortController`）に変換されるかも確かめる必要がある。

**理由:** 守りが働かないまま本番の構成に持ち込むと、[security.md](../../dev/security.md) §3 の規則（リダイレクトに従わない、応答の大きさに上限を設ける）が拡張の経路で破れる。

**候補:** PoC で各守りの振る舞いを確かめ、ネイティブと違う点を調査記録に書く。

**関連:** F-002、AC-08。

## H-05: PoC の成果物と CSP

**懸念:** Go の WASM を動かすには、Go のツールチェーンの `wasm_exec.js` と、manifest の `content_security_policy.extension_pages` に `'wasm-unsafe-eval'` が要る。今のビルドの成果物の検査（[ADR-0026](../../dev/adr/0026-extension-platform.md)）は、`tsc` の出力と `static/` の写し以外を拒否する。

**理由:** PoC のブランチでは、この検査を通すか外すかを決める必要がある（要件書 §4.2 の方針の例外）。

**候補:** PoC のブランチでは、成果物の検査に PoC の WASM と `wasm_exec.js` を許す。その変更が `main` に入らないことは、PoC のブランチをマージしないことで守る。

**関連:** F-003、AC-10。

## H-06: 字幕をビューアに渡す方法

**懸念:** 今の拡張は、ポップアップで集めた字幕を、Native Messaging のホストを通してサーバに送る。PoC のビューアに字幕を渡すには、次の案がある。

-   ポップアップの収集を使い、拡張の中で渡す。
-   ビューアに貼り付けの欄を設ける。

**理由:** 本番の経路に近いほど、#772 の判断の材料として正確になる。一方、PoC の実装の量は増える。

**候補:** ビューアに字幕・メタ情報の貼り付けの欄を設ける。字幕は文字起こしパネルからコピーしたものを貼り付ける（AC-18）。

**関連:** F-003、AC-10・AC-18。

## H-07: WASM の実行がビューアの操作を止めないか

**懸念:** Go の WASM は、読み込んだページのメインスレッドで動く。API の応答の待ちは `fetch` の非同期で待つので、操作を止めない見込みである。一方、字幕の解析や Markdown の検査のような CPU の処理の間は、ビューアの描画と操作が止まりうる。

**理由:** ビューアの操作が長く止まると、背面のタブの振る舞い（問い 2）の測定にも影響しうる。

**候補:** PoC ではメインスレッドで動かし、止まった時間が目立てば調査記録に書く。Web Worker で動かす案は、#772 の検討に残す。

**関連:** F-002・F-003、AC-10・AC-15。

## H-08: 測定の手段

**懸念:** 次の測り方を決める必要がある。

-   読み込みの時間：どこからどこまでを測るか。
-   工程ごとの所要時間。
-   タブを閉じたときにリクエストが中止されたかの観測：DevTools・`chrome://net-export`・Anthropic のコンソールの使用量など。
-   Brave での破棄の操作。

**理由:** 測り方が回ごとに違うと、値を比べられない。

**候補:** 読み込みの時間は、ページの読み込みの開始から入口を呼べるまでを `performance.now()` で測る。工程ごとの時間は、入口が返す値（AC-09）を使う。中止は `chrome://net-export` の記録で確かめる。

**関連:** F-004、AC-14〜AC-17。
