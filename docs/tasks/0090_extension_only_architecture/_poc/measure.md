# 0090 F-003 の実測の手順（PoC のブランチだけ）

このファイルは PoC のブランチ `poc/0090-extension-only-checks` だけにあり、`main` には入れない。結果は、測った後に `docs/research/wasm_pipeline_poc/README.md` に追記する（0090 要件定義書 AC-22）。0084 の規則（[0084 要件定義書](../../0084_wasm_pipeline_poc/01_requirements.md) §4.2・§5）に従う。テスト用の API キーだけを使い、API キー・ワークスペース ID・記事の本文・字幕の本文を記録しない。

## ビルドと読み込み

1. 作業ツリーを変更のない状態にする（`make poc-build` は変更があると失敗する）。
2. 既定の版: `make poc-build`。CSP の `connect-src` の版: `make poc-build POC_CSP=connect`（`extension/dist/manifest.json` の CSP が `script-src 'self' 'wasm-unsafe-eval'; object-src 'self'; connect-src 'self' http://127.0.0.1:8787` になる）。
3. 出力の `poc-build: revision` の SHA を記録する。
4. 測定用のプロファイル（0084 と同じ条件）で `extension/dist` を「パッケージ化されていない拡張機能」として読み込む。版を切り替えたら、拡張の再読み込みボタンを押す（2 つの版は同じ ID なので、同時には読み込まない）。
5. 拡張の ID を確かめ、`chrome-extension://<ID>/poc.html` を前面のタブで開く（以下「操作のページ」）。Runtime の `vcs.modified` が `false` であることを確かめる。
6. プローブ用サーバ: `go run ./cmd/yt2column-wasmprobe -preflight allow > <リポジトリの外>/probe.log`（(b) の CSP の確認では、先の転送を許す設定で、CSP だけが止める側になるようにする）。リダイレクトの確認で 0084 と比べるときは `-preflight deny` も使う。

## (a) 一度も前面にならない背面のタブ（Worker）

ブラウザごと（Chrome・Brave）、Worker の形ごと（classic・module）に 1 回ずつ。

1. 操作のページで API キー・ワークスペース ID・動画の値・字幕を入れ、`anthropic-dangerous-direct-browser-access` を有効にする。生成と検証・推敲は `claude-haiku-5-5`・`medium`。期限の欄は空にする。
2. 「Worker form」を選び、「Generate in background runner」を押す。背面のタブが開く。**そのタブを一度も選ばない。**
3. 操作のページの「Runs」に記録が出る。`status` が `finished` などの終わりの値になるまで待つ。
    -   その間に、操作のページの「Ping runner」（Run ID は空でよい。最新の実行を指す）を押す。押すのは、生成の始まった直後、背面のまま約 5 分たった後（実行がそれだけ続くとき）、完了の後の 3 回以上。「Pings from this page」の各行の `rttMs`（`null` は 3 秒以内に答えがなかったこと）・`instanceId`・`runnerStatus`・`runnerVisibility` を写す。
4. 記録から次を写す: `swPings`（SW が状態の変わるたびに 1 回送った問い合わせの `atStatus` と `rttMs`）、`workerForm`、`visibilityAtLoad`、`visibilityEvents`、`everVisible`（`false` であること）、`workerLoadMs`、`callElapsedMs`、`ok`、`failure`、`stages`（工程ごとの所要時間・トークン数・料金）、`workerFetchOk`、`build.vcsRevision`。
5. 終わった後に、ランナーのタブを閉じる。

## (b) Worker からの送信の守り（ダミーのキーだけ）

プローブは API キーを送らない。ブラウザごとに行う。「Probe in background runner」は Worker から、「Run probe」は操作のページ（メインスレッド）から送る。

1. 既定の版で、Worker から `redirect_error` と `redirect_manual` を 1 回ずつ（`-preflight allow` と `deny` の両方）。記録の `probe` と、probe.log の `foreign` の行（`OPTIONS` と本番の要求が届いたか）を写す。
2. 既定の版で、Worker とページから `direct_foreign` を 1 回ずつ（`-preflight allow`）。CSP がないときに先へ届くことの対照である。
3. `POC_CSP=connect` の版で、Worker とページから、それぞれ次を 1 回ずつ（`-preflight allow`）。
    -   `direct_same`（A へ。届くはず）
    -   `direct_foreign`（B へ。止まるはず）
    -   `redirect_foreign`（A から B への 307 を、転送を許す既定のまま追う）
    各回で、結果の `failure` と、probe.log で B（`foreign-v4`・`foreign-v6`）に何か届いたか（`OPTIONS` を含む）を写す。Worker の記録が `loadFailed` なら、その文も写す（CSP が Worker の WASM の読み込みを止めた場合）。
4. 期限で止める: 既定の版で、期限の欄に `10` を入れ、Worker から `stall_body` を送る。記録の `status` が `deadline: worker terminated` になり、probe.log に `/stall/body closed by client after` 約 10,000 ms の行が出るかを写す。期限を空にした同じ回（止まらないことの対照）は省いてよい。

## (c) `chrome.storage.local` の読める文脈

ブラウザごとに 1 回。

1. 別のタブで `http://127.0.0.1:8787/` を開く（404 の画面でよい）。
2. 操作のページで「Run the access-level check」を押す。
3. 結果の `injectedBefore`（既定で、注入したスクリプトがダミーの値を読めたか）、`setAccessLevelType`、`setAccessLevel`、`injectedAfter`、`restore` を写す。

## (d) `chrome.storage.session` の容量

ブラウザごとに 1 回。「Run the quota check」を押し、`quotaBytes`・`recordsWritten`・`bytesInUseAtStop`・`stopReason`・`bytesInUseAfterClear` を写す。

## 記録すること（共通）

測った日、ブラウザの版（`Info.plist` から、0084 と同じ）、OS、ビルドの SHA、CSP の版、Worker の形、プローブの `-preflight`。測定の記録はリポジトリの外に置き、調査記録には値だけを書く。測定の後にテスト用の API キーを失効させる。
