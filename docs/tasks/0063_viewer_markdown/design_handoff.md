# 設計への申し送り：ビューアで記事の Markdown を描画する

要件定義（[01_requirements.md](01_requirements.md)）の作成とレビューで挙がった、設計で決める事項を記録する。`02_architecture.md` は、各項目の扱い（採った方式、または当てはまらない理由）を記録する。

## H-01: 解釈の時間が入力の長さの 2 乗にならないこと

AC-32 は、512 KiB の入力を 1 秒以内に描画することを求める。閉じる記号を開く記号ごとに行の終わりまで探す実装は、閉じない記号の多い行（例: `**a` の繰り返し）で 2 乗の時間になる。行ごとに 1 回の走査で閉じる位置を決める方式など、時間が入力の長さにほぼ比例する方式を選び、病的な入力のテストで確かめる。

## H-02: URL の解釈と、文字列に戻したものの比較

F-003 は、URL の文字列が、URL として解釈して文字列に戻したものと一致することを求める。WHATWG の URL の解釈（`new URL()` と `href`）を使う場合、末尾の `/` の補い（`https://example.com` は `https://example.com/` になる）でも一致しなくなる。出典の URL（`https://www.youtube.com/watch?v=…`）が一致することを確かめ、末尾の `/` のない URL をリンクにしないことを受け入れるか、比較の規則を設計で決める。要件の「1 文字も違わない」を緩める場合は、表示とリンク先が食い違わないことを保つ根拠を記録する。node のテストとブラウザで URL の解釈が同じであることも確かめる。

## H-03: 規則が決めない並びの扱い

3.2 節は、`*` が 3 つ以上並ぶ並び（`***x***`）や、強い強調と強調の記号が重なる並び（`**a*b**`）の扱いを決めていない。左から順に当てはめる手順など、決定的な扱いを設計で決め、テストで固定する。扱いは F-004 の要素と属性の範囲を出ず、文字を失わない（AC-20）。

## H-04: 要素と属性の一覧の確かめ方

AC-19 は、どの入力でも、作る要素と属性が一覧の範囲に収まることを求める。描画した DOM を走査して、要素の名前と属性の名前を一覧と比べるテストを、AC-17・AC-18 の形とランダムな入力（性質のテスト）で行う方式を検討する。テストの環境（node のテストの DOM の代わり）で、要素と属性を数えられるかも確かめる。

## H-05: 描き直しの判定

今のビューアは、記事の要素の `textContent` と表示用の Markdown を比べて、違うときだけ描き直す（`extension/src/ui/renderViewer.ts:319-321`）。描画した後の `textContent` は表示用の Markdown と一致しない（記号を除くため）ので、この比較のままでは毎回描き直し、AC-31（フォーカスと選択を保つ）を満たせない。描いた表示用の Markdown を覚えて比べるなど、比較の方式を設計で決める。0050 の `extension/test/renderViewer.test.ts:705-727` のテストも、これに合わせて直す。

## H-06: 文字組みの値とコードブロックのはみ出し

AC-21・AC-22 の範囲の中で、本文の最大の幅、文字の大きさ、行の高さ、見出し・リスト・コードブロックの余白の値を決める。今のページは `body` の `max-width: 48rem` で、本文に枠と内側の余白がある（`extension/static/viewer.css`）。本文の行の幅が 700〜800 CSS px に入るように、ページの幅と本文の余白をあわせて決める。コードブロックの長い行を折り返すか、ブロックの中で横にスクロールするかも決める（後者の場合、スクロールする領域をキーボードで操作できるかを WCAG 2.1.1 で確かめる）。明るいテーマと暗いテーマのコントラスト（AC-26）は `extension/static/theme.css` の色の変数で確かめる。

## H-07: 双方向の文字の扱い

今の本文は `dir="auto"` と CSS の `unicode-bidi: plaintext` で、行ごとに文字の方向を決めている（`extension/src/ui/articleBody.ts`、`extension/static/viewer.css`）。AC-27 を満たすように、描画したブロックごとの方向の決め方（ブロックの要素の `dir="auto"` など）を決める。

## H-08: CLAUDE.md「Declare, don't infer」の例外の文

要件 1 章「方針の変更」のとおり、レンダラーは表示用の Markdown の中身を調べて描画する要素を選ぶ。CLAUDE.md の例外の一覧に、ほかの例外（`resolveLocale` など）と同じ形で、範囲（レンダラーの関数の名前とファイル）・理由・境界を書き加える。CLAUDE.md は英語で書く。

## H-09: 文書とテストの更新

-   `docs/dev/security.md` §8.1「記事を untrusted テキストとして表示する」と残余のリスク、§8.2「返す記事は untrusted なテキスト」を、本書の規則に合わせて書き換える。
-   `README.md:630` の「Markdown を解釈せずに文字のまま表示し」を書き換え、`/mktrans` で `README.en.md` に反映する。README の変更は `metadataDocs.test.ts` などの拡張の node のテストが読むので、`make ext-test` を実行する（CLAUDE.md「Development Notes」）。
-   0050 の AC-25 を確かめていたテスト（`extension/test/articleBody.test.ts`、`extension/test/renderViewer.test.ts` の `.article-body` の `textContent` を比べるテスト）を、本書の AC に合わせて直す。テストを削除する場合は、CLAUDE.md「Deleting a test is a claim that must be checked」に従って `make ext-coverage-compare` で比べる。
-   ESLint の HTML を解釈する API の禁止（`extension/eslint.config.*`）が、新しいレンダラーのファイルにも効くことを確かめる。
