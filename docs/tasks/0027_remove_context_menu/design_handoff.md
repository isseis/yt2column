# 設計への申し送り：ブラウザ拡張の右クリックメニューの経路の削除

本書は、[01_requirements.md](01_requirements.md) の起草で挙がった事項のうち、設計の段階で決めるものを集める。`02_architecture.md` は、各項目について、採った方針か、当てはまらない理由を記録する。

## H-01: 共有しているコードの切り分け

-   **事項:** メニューの経路とポップアップの経路が共有しているコードのうち、残すものと削除するものを決める。メニューの経路にだけ使う型・関数・定数が残っていないことの確かめ方を決める。
-   **理由:** `extension/src/launch.ts` は両方の経路の処理を持つ。ポップアップは送らない理由の表示に `renderResultView`（`ui/renderSend.ts`）を使い、その引数の型 `ResultView` は `core/resultView.ts` にある。`core/resultView.ts` には、結果のウィンドウだけが使う `parseResultView` もある。`chromeDeps.ts` の `createResultViewStore`・`createResultWindowOpener`・`createBadge`・`schedule` と、その型（`StorageChangeEvents`・`WindowsApi`・`ActionApi` など）はメニューの経路だけが使う。ほかに、`launch.ts` の `runMenuLaunch`・`runResultWindow`・`MenuLaunchDeps`・`ResultWindowDeps`・`ResultViewStore`・`resultWaitMs` と、`core/sendMessages.ts` の `unreadableResultMessage` もメニューの経路（結果のウィンドウ）だけが使う。`launchContextFromTab` は両方の経路が使う。`make deadcode` は Go だけを対象にし、拡張の未使用の export は検出しない。
-   **候補の方針:** ポップアップから到達できる export だけを残す。未使用の export の検出は、ESLint・`tsc` の既存の設定で検出できるかを確かめ、できなければ削除の後に export ごとの参照を確かめて記録する。`ResultView` の型の名前と置き場所を、結果のウィンドウがなくなった後の用途に合うように変えるかを決める。`schedule` は、0026 が `SendDeps` の `schedule` としてポップアップの経路で使う予定である。0026 の実装計画（§1.3「0027 との順序」）は、本タスクが `schedule` を削除した場合はステップ 3-7 で加え直すとしている。削除するか残すかを決める。
-   **関連:** F-001、F-002、§4.5

## H-02: ビルドの検査と manifest のテスト

-   **事項:** ビルドの成果物の検査（`extension/scripts/check-dist.ts`）と manifest のテスト（`extension/test/manifest.test.ts`）を、service worker と結果のウィンドウがない構成に合わせる方法を決める。
-   **理由:** `check-dist.ts` は manifest の `background.service_worker` が指すファイルの存在を確かめている（値が文字列でなければ確かめないので、`background` がなくても失敗しない）。また、`check-dist.ts` は成果物のファイルの集合を `src/` と `static/` から導き、`make ext-build` は `dist/` を作り直すので、ソースを削除すれば成果物にないことも既存の検査で確かめられる。`manifest.test.ts` は `background` のメンバと `contextMenus` の権限を期待している。AC-01・AC-02 は、これらがないことを確かめる必要がある。
-   **候補の方針:** `background` がないことと、`permissions` の集合を、既存のテストの期待値として固定する。成果物に `background.js`・`result.html`・`result.js` がないことは、ソースの削除と既存の `check-dist.ts` の集合の比較で確かめ、ファイル名を列挙する検査は加えない。`check-dist.ts` の `background.service_worker` の参照の検査を残すかを決める。
-   **関連:** AC-01、AC-02

## H-03: テストの削除の前後のカバレッジの比べ方

-   **事項:** 拡張のテストを削除する前後で、残るコードのカバレッジが下がっていないことを確かめる手順を決める（AC-09）。
-   **理由:** CLAUDE.md の「Deleting a test is a claim that must be checked」は、Go について `go tool cover -func` で関数ごとに比べることを定めている。拡張の node のテストには、同じ目的の手順がまだない。`launch.test.ts`・`chromeDeps.test.ts` は、メニューの経路のテストとポップアップの経路のテストを同じファイルに持つので、ファイルごとの削除では済まない。
-   **候補の方針:** `node --test --experimental-test-coverage --test-reporter=lcov` で削除の前後の報告を取り、残る関数ごとの到達の有無（lcov の `FN`・`FNDA`）を比べる。既定の報告はファイルごとの割合だけを示し、`launch.ts`・`chromeDeps.ts` のように行を削除するファイルでは分母が変わるので、比べる対象にしない。報告の取り方は実装計画に記録し、Makefile の対象には加えない（一度きりの確認のため）。
-   **関連:** AC-09、AC-10

## H-04: `sendTimeoutMs` の理由の記述

-   **事項:** 送信の時間の上限（`extension/src/send.ts` の `sendTimeoutMs`、25 秒）の値を選んだ理由の記述を、メニューの経路がない構成に合わせて改めるかを決める。
-   **理由:** 0020 の設計（`02_architecture.md` §3.5）は、メニューの経路が service worker から送り、service worker が 30 秒を超えて応答を待つと停止されうることを、上限を 30 秒より短くした理由にしている。本タスクの後は、送信はポップアップからだけ行う。値は変えない（要件書 §2.3）。0026 は、ホストの 20 秒 < 拡張の 25 秒、という順序をこの値に依存させている（0026 `02_architecture.md` §3.5.5）。
-   **候補の方針:** 値を変えない。コード（`send.ts:11-12`）のコメントは上限の意味だけを述べ、30 秒の理由を持たないので、コードの変更は要らない見込みである。理由は 0020 の設計書にだけあり、完了したタスクの文書は書き換えない。本タスクの後に当てはまる理由（サーバの応答は通常は短い時間で返る、0026 のホストの上限との順序）を `02_architecture.md` に記録する。0026 の設計（§3.5.5）は、[#338](https://github.com/isseis/yt2column/pull/338) で時間の順序から service worker の 30 秒を外し、25 秒を保つとしたので、本タスクの記録と整合する。
-   **関連:** §2.3、§5

## H-05: 既存の `chrome.storage.session` の値

-   **事項:** メニューの経路が入った版で `chrome.storage.session` に残った結果の表示内容が、本タスクの版に読み込み直した後に残るか、残る場合に削除する処理が要るかを確かめる。
-   **理由:** 結果のウィンドウは、表示を終えると値を削除する（`runResultWindow`）。結果のウィンドウを開けなかった場合なども削除するが、削除に失敗した値が残りうる。本タスクの版はこの値を読まない。`chrome.storage.session` は、拡張の再読み込み・更新・無効化で消えると見込む。
-   **候補の方針:** 拡張の再読み込みで `chrome.storage.session` が空になることを、Chrome と Brave の手動確認（AC-05 と同時）で確かめる。空になれば、削除する処理を加えない。
-   **関連:** AC-05
