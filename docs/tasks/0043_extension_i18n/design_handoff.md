# 設計への申し送り：ブラウザ拡張の UI の多言語化

要件定義（[01_requirements.md](01_requirements.md)）の検討で挙がった、設計で決める事項。`02_architecture.md` は、各項目について採った方法、または当てはまらない理由を記録する。

-   **H-01: README のガードのテストと英語の文言の対応（issue #506「決める必要があること」）。** `extension/test/metadataDocs.test.ts` は、README が引用する促しのメッセージを `expandPromptMessage` と比べている。表に移した後も、英語版の文言と比べる方法を決める（AC-23）。
-   **H-02: テストで言語を切り替える方法（issue #506「決める必要があること」）。** `chrome.i18n.getUILanguage()` を、`extension/src/browser/chromeDeps.ts` と同じように依存として注入するか、言語を引数で渡すかを決める。言語の判定（AC-01・AC-02）を、ブラウザなしで試せるようにする。
-   **H-03: 日本語の YouTube のボタンの表記の観測（issue #506「決める必要があること」、AC-11）。** 日本語表示の YouTube で、`"...more"`・`"Show transcript"` などに当たる表記を観測し、条件（日付・ブラウザ・YouTube の表示言語）とともに記録する。用語集には「…もっと見る」、README には `動画の時間と同期` があるが、ほかの表記はまだ確かめていない。
-   **H-04: 両方の言語で文言の性質を確かめる方法（AC-09）。** 今のテスト（`extension/test/messages.test.ts`・`extension/test/sendMessages.test.ts`）は英語の文言だけを試している。両方の言語に当てはめる方法を決める。「Slack」などの語の有無を調べるテストを、日本語の文言にも使えるか（日本語でも `Slack` と書くか）を含める。
-   **H-05: 英語の文言が変わらないことの確かめ方（AC-07）。** 要件書の受け入れ基準は変更の前後の関係なので、実装計画書で `diff` の検証（`_checks/`）にするかを決める。
-   **H-06: 言語の判定を置く場所（5 章の「Declare, don't infer」の例外）。** 表示言語の文字列を言語の型に変える処理を 1 か所に置き、それより後は型で分岐する構造にする。
-   **H-07: `lang` 属性と書体の設定の方法（AC-14〜AC-17）。** `popup.html` の静的な `lang="en"` を、最初の描画の前にどう変えるか（英語の書体で一瞬表示されることを避けるか）。日本語の書体と行間を、`:lang(ja)` などの CSS で切り替えるか。日本語の行間の値。
-   **H-08: `_locales` のビルドへの取り込み（AC-18）。** `extension/scripts/copy-static.ts` は `static/` をそのまま `dist/` へ写す。`extension/scripts/check-dist.ts` が `dist/` のファイルの集合を検査しているなら、`_locales` を加える。`manifest.test.ts` の最上位のキーの検査と、0007 の設計の表との関係を更新する。
-   **H-09: Chrome の `_locales` の制約（AC-18・AC-19）。** `_locales` を持つ拡張は `default_locale` が必須で、`description` は 132 文字までという Chrome の制約を確かめる。`__MSG_*__` の名前の付け方を決める。
-   **H-10: 数の書式のテスト（AC-13）。** `Intl.NumberFormat` の結果は実行環境の ICU に依存しうる。Node.js のテストで、両方の言語の書式を安定して確かめる方法を決める。
