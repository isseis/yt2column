# 設計への申し送り（タスク 0057）

要件定義の際に挙がった、設計（`02_architecture.md`）で扱う事項である。設計書は、各項目をどう扱ったか、または扱わない理由を記録する。

## H-01: interaction limits の「制限がない」応答の形

GitHub の文書は、制限がなければ「空の応答」を返すとする。HTTP の状態（`200` か `204` か）と本文（0 バイトか `{}` か）は、文書からは決まらない。設計の際に、制限がない状態の実際の応答を観測し（読むだけの `gh api` の呼び出し。ネットワークの操作なので利用者に確かめる）、受け入れる形を 1 つに決める。決めた形のほかは「確かめられない」とする（要件定義書 4.2）。

観測は、利用者が F-003 の interaction limits を有効にする前に行う。有効にした後では、制限を外さない限り「制限がない」応答を観測できない。

## H-02: 状態を読む処理を置く場所

`fixprguard` のサブコマンドにすれば、応答の検査（`internal/strictjson`）と境界のテストを Go のコードで書ける。コマンドの文書の `gh api --jq` にする場合は、要件定義書 4.2 の拒否の規則を満たせることを示す必要がある。`jq` は重複した項目を黙って後勝ちにするので、そのままでは 4.2 を満たさない。`fixprguard` に置く場合は、`protocol` の番号の上げ（F-005）と同じ PR に入れられるか、ガイドのサブコマンドの章と文書のガードをどう合わせるかも決める。

## H-03: 現在の時刻と時間の上限

期限の比較に、ローカルの時計と GitHub の時計（`/runpr` がすでに使う `Date` ヘッダ。`.claude/commands/runpr.md` の「Authorization」）のどちらを使うかを決める。境界（AC-18）のテストで時刻を固定できる形にする。取得の時間の上限（時間切れは「確かめられない」になる）も決める。

## H-04: 範囲の規則の `named` の引数

F-001 の後、実行関連ファイルの判定は名指しの有無によらない。`inScopeOf`（`internal/fixprguard/scope.go:231-258`）で `named` が影響するのは `.go` の規則だけになる。関数の形と、ガイド 6 章の「判定の順序」の書き方を合わせて決める。

## H-05: F-003 の手順を書くガイド

手順を書く場所（`fixprguard_guide.md` の新しい節か、別のガイドか）を決める。`/runpr` の警告（AC-14）は、その場所を示す。

## H-06: 自動レビュアーの一覧を変えたときの文書のガード

自動レビュアーの一覧の名前は、`cmd/yt2column/fixpr_docs_test.go:58` の `wantReviewers` が固定している（`CLAUDE.md` が文書のテストの例外として残す「the automated-reviewer list」）。これを `chatgpt-codex-connector` だけにする。`cmd/fixprguard/docs_test.go` の `TestCheckDocumentedReviewer`・`TestGuideDescribesReviewerList` などが、ガイドの `--reviewer coderabbitai` の例をどう扱っているかを確かめ、例を一覧の名前に変える。

## H-07: GitHub の設定が使えることと、読む権限

次の点は、要件定義の際に文書で確かめられなかった。実装の PR より前、設計の際に確かめ（作成の制限と push の制限は、実際に設定して利用者の push と `/mergepr` が止まらないことを試す）、成り立たなければ決定の変更として要件定義書に戻す。AC-11 で最後の PR まで待つと、失敗が遅れて分かるためである。

-   個人の公開リポジトリで、issue の作成を collaborators に限る設定（2026-06 に追加）と、ruleset の「Restrict creations」「Restrict updates」が使えること。
-   `GET /repos/{owner}/{repo}/interaction-limits` を、利用者の `gh` の資格情報で読めること。読めなければ、すべての `/runpr` が「確かめられない」になり、警告への慣れを生む（要件定義書 4.2）。
