# 設計への申し送り：タスク 0061

要件定義（[01_requirements.md](01_requirements.md)）の作成とレビューで挙がった、設計で決める事項。`02_architecture.md` は、各項目をどう扱ったか、または扱わない理由を記録する。

-   **H-01 確かめ直しの方法:** AC-25 の確かめ直しを、スクリプト（Go のツールなど）で行うか、コマンドの定義の手順（`git diff --numstat` など）で行うかを決める。あわせて、次のものの数え方も決める。バイナリのファイル、名前の変更（rename）、改行だけの変更、`README.md` に伴う `README.en.md` の変更である。手順で行う場合は、LLM が数え間違えない形（コマンドの出力をそのまま比べるなど）にする。
-   **H-02 除外するパスの一覧:** AC-05 のうちパスで決まるものの正確な一覧と、その置き場所（`_context.md` か `requirements_process.md` か）を決める。候補は、`cmd/fixprguard`・`internal/fixprguard`・`cmd/codexreview`・`internal/codexreview`・`cmd/mergepr`・`internal/mergepr`・`docs/tasks/`・`docs/dev/security.md`・`go.mod`・`go.sum`・`extension/package.json`・`extension/package-lock.json`・`.golangci.yml` などである。秘密情報・`yt-dlp` の呼び出し・キャッシュに当たるパッケージ（`internal/secret`・`internal/redact`・`internal/transcript` の一部・`internal/articlecache`・`internal/cachelock` など）をパスで除くか、レビューで確かめるかも決める。類型 W は、`CLAUDE.md`・`.claude/commands/`・`docs/dev/conventions/` の誤字の修正を認める。そのため、これらのパスを類型ごとに扱い分けるかも決める。
-   **H-03 道具が生成した行の確かめ方:** 類型 M の、道具が生成した行の例外（AC-02）について、次のことを決める。再実行して同じ diff になることを、どう確かめるか（変更の前の状態の worktree での再実行など）。手で書いた行と道具が生成した行を、どう分けて数えるか（コミットを分けるなど）。
-   **H-04 仕様のコメントの書式:** 目印の行の正確な文言、やること・やらないこと・類型・確かめ方の見出し、見込みの変更量を書くかどうかを決める。また、`/runissue` が、認証済みの利用者のコメントだけを読む方法も決める。たとえば、`--jq` で書き手を絞ってから本文を読み、他人のコメントの本文を会話に入れない方法である。
-   **H-05 `/runpr` が軽量プロセスの PR を見分ける方法:** ブランチ名の形、issue のラベル、PR の本文の `Closes #<N>` のどれで見分けるかを決める。本文の文字列から振る舞いを選ぶことは、CLAUDE.md の「Declare, don't infer」に当たりうるので、それを踏まえて決める。
-   **H-06 ブランチ名・コミットの接頭辞・記録の書式:** `/runissue` のブランチ名の形を決める。タスクのない形なので、`_context.md` の「Feature branch name」とは別の形になる。あわせて、コミットの接頭辞（`<type>(<scope>)` の scope）と、AC-22 の記録をコミットメッセージに書く書式を決める。
-   **H-07 `/mkreq` の手順のどこで振り分けるか:** 振り分けを、手順 1（issue の読み取りと重複の検索）と手順 3（ブランチの作成）の間のどこに入れるかを決める。あわせて、軽量プロセスを選んだときに `/mkreq` の報告（手順 9）が何を示すかも決める。
-   **H-08 文書のテスト:** 新しい値（ラベル名、CLAUDE.md のコマンドの一覧、テンプレートの見出し）と、メンションの確認などの信頼境界のコマンドに、document test を足すかどうかを決める。CLAUDE.md の「Document tests check values, structure, and executable examples, never prose」に従って決める。
-   **H-09 ラベルの作成:** `lightweight` ラベル（AC-36）を、誰がどの時点で作るかを決める（実装の PR の手順か、利用者の手作業か）。色と説明も決める。
