# ADR-0016: 文書のテストは値・構造・実行できる例だけを確かめ、文の意味は PR のレビューで読む

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

リポジトリの文書には、コードや設定と一致していなければならない値がある。コマンドのフラグと終了コード、LLM のプロトコル番号、拡張の ID と権限、`make` の対象、拒否の理由の表などである。これらは、文書とコードのどちらかを変えたときに食い違いやすい。文書だけを変える PR でも、その食い違いを見つけられるようにしたい。

一方で、文書のテストは、文の意味を確かめられない。語を照合して文の意味を固定すると、言い換えのたびにテストを直すことになり、意味を反転させた文を見逃すこともある。テストの維持の費用が、防ぐ誤りを上回る確認を足すと、文書を直すたびにテストを直す手間だけが増える。

この決定の仕様の正本は [CLAUDE.md](../../../CLAUDE.md) の「Testing Strategy」の「Document tests check values, structure, and executable examples, never prose」以降と「Development Notes」の文書を読む guard の一覧、[.github/workflows/ci.yml](../../../.github/workflows/ci.yml) の `doc-guard` ジョブ、[requirements_process.md](../conventions/requirements_process.md) の §4（受け入れ基準の確かめ方の種類）である。

## 決定

-   **文書のテストは、値・構造・実行できる例だけを確かめる。** 文の意味は、PR のレビューで読む。どの値・構造・例を確かめてよいか、直書きのリテラル・古い名前の不在・タスクの記録を扱わないこと、2 つの例外、同じ値を 2 つのテストで確かめないこと、廃止した仕様を残させるテストを足さないこと、維持の費用が防ぐ誤りを上回る確認を足さないことは、[CLAUDE.md](../../../CLAUDE.md) の「Testing Strategy」の「Document tests check values, structure, and executable examples, never prose」の段落と、その後の 3 つの規則に従う。この ADR は、その規則の理由と、採らなかった案を持つ。
-   **変更の前後関係（既存の見出しが変わっていないことなど）は、文書のテストにしない。** `diff` の確かめとして、マージの基点と PR の head を比べ、PR ごとに 1 回だけ実行する（[requirements_process.md](../conventions/requirements_process.md) §4）。
-   **`diff` の確かめの作りは、確かめる差分の大きさに見合ったものにする。既定はタスクの `_checks/` に置くシェルスクリプト（`git diff`、`cmp` など）とし、ほかの言語はシェルで足りない理由を計画に書いたときに使う。** スクリプトはマージの後に保守しないので、費用は PR のレビューで読む量と書く作業であり、それを確かめる変更に見合ったものにするためである（[requirements_process.md](../conventions/requirements_process.md) §4、[mkplan.md](../../../.claude/commands/mkplan.md) step 6）。
-   **差分が小さい文書の変更では、PR のブランチで実行した `git diff` の出力を 2 つのコミットの ID とともに計画に記録し、PR のレビューで読むことも、`diff` の確かめとする。** 「小さい」かは計画を書く者が判断し、レビュアーは異論を述べられる。数の閾値は設けない。差分が数行の変更では、差分そのものを読むほうが、それを確かめるスクリプトを読むより少なく済むためである。この形には、壊して失敗を確かめる対象のコードがないので、その手順は要らない（[requirements_process.md](../conventions/requirements_process.md) §4）。
-   **前後の関係を述べる受け入れ基準は、「X の差分は Y と Z に限られる」の形で書き、守る部分を並べ挙げない。** 並べ挙げた部分の一つ一つを解析するコードが生まれるためである（[requirements_process.md](../conventions/requirements_process.md) §1）。
-   **計画は、前のタスクの `_checks/` を手本として写させず、壊し方は「チェックごとに少なくとも 1 つ」とだけ書いて並べ挙げない。** 前のタスクの大きなプログラムの構造が受け継がれ、計画に並べた壊し方がそのまま作業になるためである（[mkplan.md](../../../.claude/commands/mkplan.md) step 6）。
-   **確かめるものに比べて過大な `diff` の確かめを、計画のレビュー・フェーズのまとまりのレビュー・弱いモデルの作業のレビューで指摘する。** 大きさは計画を書くときに決まるので、実装の前にも見つけられるようにするためである（[mkplan.md](../../../.claude/commands/mkplan.md) step 8、[runplan.md](../../../.claude/commands/runplan.md) step 7、[weakreview.md](../../../.claude/commands/weakreview.md) のチェックリスト）。

上の作りの基準・記録した差分の読み取り・受け入れ基準の書き方・計画の規則・レビューの観点は、要件定義の段階であり、requirements_process.md・mkplan.md・runplan.md・weakreview.md・CLAUDE.md にはまだ反映していない。この差分は [#789](https://github.com/isseis/yt2column/issues/789) の実装で解消する。
-   **文書を読む Go の guard は、`doc-guard` の CI ジョブで、変更の種類にかかわらずすべての PR に実行する。** 対象のパッケージと、文書だけの変更で `make ext-test` を要求しない範囲は、[CLAUDE.md](../../../CLAUDE.md) の「Development Notes」と [.github/workflows/ci.yml](../../../.github/workflows/ci.yml) を正本とする。変えるときは、同じ PR で正本とこの ADR をそろえる。

## 採らなかった案

-   **文書のテキスト同期の guard を、拡張の node のテストに残す。** 文書だけの変更で node と `npm ci` を要求する。guard はテキストの同期だけで node を必要としない（[0012 要件定義書](../../tasks/0012_doc_sync_guards/01_requirements.md) §1）。
-   **文書だけの変更を CI で守らず、ローカルの `make test` に委ねる。** 手元でテストを実行しない経路（Web での編集、bot）で guard が壊れると、原因の PR と別の PR で初めて検出される（[0014 要件定義書](../../tasks/0014_doc_guard_ci/01_requirements.md) §1）。
-   **`doc-guard` で `go test -tags test ./...` を走らせる。** 文書だけの PR でも全パッケージのテストが走り、実行時間が延びる。対象を明示して列挙し、新しい文書の guard を足すときに更新する（[0014 設計書](../../tasks/0014_doc_guard_ci/02_architecture.md) §11）。
-   **`check-changes` の除外から `.md` と `docs/` を外す。** 文書だけの PR でも race 付きの全テストと lint が走る。実行時間が延び、分類の設計も変わる（[0014 設計書](../../tasks/0014_doc_guard_ci/02_architecture.md) §11）。
-   **`Makefile` に `test-doc-guard` の対象を足す。** 対象は 2 パッケージで固定であり、`ci.yml` に直接書けば足りる。同じコマンドを複数の手順から呼ぶ必要が出たときに検討する（[0014 設計書](../../tasks/0014_doc_guard_ci/02_architecture.md) §11）。
-   **タスクの文書の値をコードと照合する確認を残す。** タスクの文書は、その時点の記録であり、完了の後に変わらない。コードと照合すると、記録をコードに合わせて書き換えることになる（[0038 設計書](../../tasks/0038_doc_test_pruning/02_architecture.md) §1.2）。
-   **過去の 1 回の変更の性質を確かめるテストを `_checks/` へ移す。** 元のタスクはマージ済みで、`_checks/` はマージの基点と PR の head を比べて 1 回だけ実行するものなので、移しても実行する機会がない（[0052 要件定義書](../../tasks/0052_doc_test_pruning/01_requirements.md) §1）。
-   **`doctestutil` に同じ処理がある補助を置き換えず、使い手を失った補助を消すだけにする。** レビュアーは、重複する補助を `internal/doctestutil` の関数に置き換えることを選んだ。ただし `cmd/fixprguard` の `cutSection` は置き換えない。`doctestutil` の節を切り出す関数は見出しの全体の一致で探すので、見出しの題を言い換えるだけで複数の guard が落ちるようになる（[0052 設計書](../../tasks/0052_doc_test_pruning/02_architecture.md) §1.2）。

## 結果

-   文書の値や構造を変えると、対応するテストが落ちる。文言だけを変えても、テストは落ちない。
-   文書だけを変える PR も、`doc-guard` のジョブで対象の guard が走る。node と `npm ci` は要らない（対象と例外は CLAUDE.md の「Development Notes」に従う）。
-   文書のテストは、文の意味の誤り（意味を反転させた文など）を見つけられない。それは PR のレビューで読む。
-   文書を直すときにテストを直す手間は、値・構造・実行できる例に限られる。新しい文書の guard を足すときは、`doc-guard` の対象にパッケージを足すか、既存のパッケージに置く。
-   `diff` の確かめは、PR ごとに 1 回だけ手で実行するか、記録した差分を PR のレビューで読む。`make test` と CI は実行しない。
-   記録した差分の読み取りを使うかは人の判断なので、同じ大きさの変更でも、計画によって確かめの形が分かれうる。その判断の誤りは、PR のレビューで見つける。
-   作りの基準を定める前に書かれたタスクの `_checks/` は、書き換えない。新しい基準は、[#789](https://github.com/isseis/yt2column/issues/789)（タスク 0080）の実装がマージされた後に書く計画から当てはめる（[0080 要件定義書](../../tasks/0080_diff_check_proportion/01_requirements.md)）。

## 経緯

- 2026-10-13 文書のテキスト同期の guard を、拡張の node のテストから Go のテストへ移した。文書だけの変更で node のインストールを要求せず、`.gitignore` だけの変更では拡張の ignore 規則の guard を node 側に残す（[0012 要件定義書](../../tasks/0012_doc_sync_guards/01_requirements.md)）
- 2026-10-14 文書だけの PR でも文書の guard を走らせる `doc-guard` のジョブを CI に足した。対象は `cmd/yt2column` と `internal/pipeline` で、`-run` で絞らない（[0014 要件定義書](../../tasks/0014_doc_guard_ci/01_requirements.md)）
- 2026-10-23 自然言語の意味や不要な文言を固定する文書のテストを削除する判定の基準（D1〜D4）と、残す確認の種類（K1〜K5）を定めた。タスクの文書の記録の確認と、コードと重複する値の確認を取り除いた（[0038 要件定義書](../../tasks/0038_doc_test_pruning/01_requirements.md)）
- 2026-10-31 古い仕様を確かめ続けるテスト、一度きりの確かめ、過剰な確認を取り除いた。重複する補助を `internal/doctestutil` に寄せ、`CLAUDE.md`「Testing Strategy」に、古い仕様を確かめるテストと過剰なテストを恒久的に足さない規則を加えた（[0052 要件定義書](../../tasks/0052_doc_test_pruning/01_requirements.md)）
- 2026-11-10 `diff` の確かめの作りを、確かめる差分の大きさに見合ったものにすることにした。既定をシェルスクリプトとし、小さな文書の変更では記録した `git diff` の出力をレビューで読むことも認め、前後の関係を述べる受け入れ基準の書き方、計画の規則、レビューの観点を定めた（[0080 要件定義書](../../tasks/0080_diff_check_proportion/01_requirements.md)「要件定義の際に決めたこと（2026-11-10）」）
