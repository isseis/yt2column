# ADR-0014: 決定的な判定はエージェントに任せず、テストのある Go の開発ツールに置く

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

開発の手順には、入力が同じなら結果が同じで、分岐が多く、安全側に倒すことが要る判定がある。`/fixpr` と `/runpr` のガードは、どのレビューのスレッドを処理するか、修正が及んでよい範囲はどこか、git の状態が始めたときと同じか、ビルドとテストの後に何を commit して push するかを決める。ローカルの CLI レビューは、Codex CLI の実行と、その最終メッセージが指摘を述べているかの判定を行う。カバレッジの比較は、テストを削除する前後で、製品の関数の網羅が下がっていないかを関数ごとに比べる。

これらをエージェントの判断や、コマンドの定義に書いたシェルの手順に任せると、同じ入力でも結果が変わりうる。誤ると、レビューの信頼の境界が実行時に働かなくなったり、範囲外の変更が commit されたりする。誤りは、テストで壊して失敗を確かめられなければ見つけられない。

## 決定

判定と副作用を持つ開発の道具はテストのある Go の道具に置き、基準値は状態のファイルから読み返さず、利用者が `main` から入れて使う。契約とプロトコル番号は文書とガードで揃え、状態を読むだけの判定と chief の記録に限り、範囲を定めた例外を置く。

-   **Go の開発の道具。** 判定と副作用を持つ開発の道具は Go で書き、テストを持つ。今日あるのは `fixprguard`・`codexreview`・`coveragecompare`・`mergepr`・`chief`・`numreserve` の 6 つで、どれも `cmd/<名前>` と `internal/<名前>` の組であり、製品の 3 つのバイナリには入らない。
    -   理由：判定の誤りを、テストで壊して失敗を確かめることで見つけられるようにするためである。何を決めるかは [ADR-0015](0015-untrusted-review-threads.md)・[ADR-0013](0013-review-setup.md)・[ADR-0037](0037-test-deletion-coverage.md)・[ADR-0046](0046-chief-of-staff.md)・[ADR-0051](0051-number-reservation.md) と [mergepr_guide.md](../guides/mergepr_guide.md) が定め、導入手順は [CLAUDE.md](../../../CLAUDE.md) の「Build Commands」にある。
-   **基準値の受け渡し。** 道具は、判定に使う基準値を状態のファイルから読み返さない。基準値と判定した集合は、サブコマンドの出力として返し、セッションが次の呼び出しの引数で渡す。`fixprguard` は利用者のために git のディレクトリへ記録を書くが、それを判定に読み返さない。
    -   理由：判定をディスクに置くと、修正のフェーズや build checks が実行するコードに書き換えられるためである。保証の内容は [fixprguard_guide.md](../guides/fixprguard_guide.md) の「7. ツールが保証すること・しないこと」にある。
-   **例外（chief の状態の記録）。** chief の道具は、リポジトリの外の状態のディレクトリにある次の記録を、判定に読み返す。自動送信の記録、送信の記録、送信の停止、待ち受けの記録（待ち受けのラウンドごとの記録と、PR が開いているのを見た記録）、通知の記録、PR の詳細の控え、ロック、workspace を作ったときの記録である。
    -   記録は、送るコマンドとタスクを決めるためには使わない。送ることについては、送ることを止める条件（リセットの候補を狭める PR の詳細の控えを含む）として使うか、自動送信の候補の workspace を名指して Herdr・git・GitHub で確かめ直す条件として使うだけである。
    -   通知の記録は通知の重複を防ぐことと待っている時間による並べ方に、送信の記録は送った入力の履歴と「実行中」の行の表示に使い、どちらも送る中身を左右しない。コマンドが `/mergemain` の送信の記録は、head ごとに 1 回の自動送信を止める条件と「解消できず」の表示にも使う。送ることに関わる記録が読めなければ送らない。
    -   workspace を作ったときの記録は、削除の候補の workspace を名指すことと、その行に示すタスクを探す手がかりにだけ使い、名指した workspace は Herdr・git・`origin/main` の文書で確かめ直す。読めなければ削除を提案しない。作ったときの記録は worktree ごとにあり、同じパスに作り直す前と、消し終えた後に、そのパスの分をすべて消す。
    -   理由：起動し直しても自動送信を 2 回しないこと、送信を止めた workspace を覚えていること、chief を 1 つに限ることを、セッションの記憶では守れないためである（[ADR-0046](0046-chief-of-staff.md)）。範囲と、記録が条件として働く範囲は [0088 設計書](../../tasks/0088_chief_of_staff/02_architecture.md) の 3.5「状態のディレクトリの記録とロック」と [0092 設計書](../../tasks/0092_chief_task_workspace/02_architecture.md) の 3.5「作ったことの記録」にある。
-   **順序の強制。** 順序を、コマンドの定義ではなく道具の中で強制する。確認より前に build checks を動かさないこと、commit と push の間で hooks や config が変わっていたら push しないこと、は道具のコードが守る。コマンドの定義に残るのは、サブコマンドを呼ぶ順序と、利用者への確認だけである。
    -   理由：エージェントが手順を読み違えても、範囲外の変更が commit されたり、信頼の境界が迂回されたりしないためである。保証の内容は [fixprguard_guide.md](../guides/fixprguard_guide.md) の「7. ツールが保証すること・しないこと」にある。
-   **拡張の関数の範囲の補助。** 拡張の関数の範囲だけは、TypeScript の補助に求める。`coveragecompare` は Go と拡張の両方の比較を受け持つが、拡張の関数の終わりの行は node の lcov が出さないので、`extension/scripts/function-ranges.ts` が入っている `typescript` の構文木から求める。新しい npm のパッケージは加えない。
    -   理由：拡張の関数の終わりの行を、lcov からは求められないためである。規則は [0055 設計書](../../tasks/0055_coverage_compare/02_architecture.md) の 3.5「拡張の測り方」にある。
-   **`main` からの導入。** 道具は `main` からインストールして使う。`mergepr`・`fixprguard`・`codexreview`・`chief`・`numreserve` は `make install-mergepr`・`make install-fixprguard`・`make install-codexreview`・`make install-chief`・`make install-numreserve` で入れ、道具を変えたマージの後は再インストールする。`--protocol` の番号を上げる PR では、その PR の `/runpr` の前に、その PR のブランチで再インストールする（[fixprguard_guide.md](../guides/fixprguard_guide.md) §3）。`coveragecompare` は `make coverage-compare` と `make ext-coverage-compare` が、そのつど一時ディレクトリにビルドして実行する。`fixprguard` は `--protocol` の番号で、呼び出し側の文書と道具の版が合っているかを照合する。あわせて、文書と道具を変える順序は、両方が動き続ける形にする（[fixprguard_guide.md](../guides/fixprguard_guide.md) §9）。
    -   理由：インストール済みの古い道具が、古い想定で黙って判定しないようにするためである。
-   **エージェントによる導入・ビルドの禁止。** エージェントは、利用者が `make install-<名前>` で `main` から入れる道具を、どれもインストールもビルドもしない。ビルドするとは、道具を走らせるためのバイナリを作ること（`go build` など）か、ソースから走らせること（`go run` など）で、テストや lint のために道具のパッケージをコンパイルすることは含まない。make のターゲットがそのつどビルドする `coveragecompare` は対象ではない。
    -   例外として、道具を開発するか調べるときは、ソースからビルドして走らせてよい。ただし、作ったバイナリは `PATH` の通った場所（`$(go env GOPATH)/bin` を含む）に置かず、手順の中で入れた道具の代わりにも使わず、外に副作用を持つ走らせ方（ネットワークや GitHub への書き込み、記録の tag の作成、pane への入力、外のサービスへの送信）もしない。
    -   プロトコル番号を上げる PR のブランチで道具を入れ直すのも利用者である。エージェントは、道具を入れるかビルドする手順に来たとき、または道具が見つからないか番号が合わないときは、そこで止まって、最新の `main` で入れるように利用者に頼む。PR のブランチで入れ直すように頼むのは、その PR がプロトコル番号を上げる場合だけである。手順が別の扱いを定めている場合（`codexreview` が見つからないときなど）は、その手順に従う。Claude Code では、`.claude/settings.json` に `make install-*` と `go install` を拒む規則を置く（タスク 0112 の実装で足す）。
    -   理由：エージェントが PR のブランチから `fixprguard` や `mergepr` を入れるか走らせると、そのブランチが変えた判定で `/fixpr`・`/runpr`・`/mergepr` が動くことになり、`main` の判定を迂回できるためである。利用者が PR のブランチで入れ直したあと、その道具がブランチの判定で動くことは受け入れる。利用者がその PR の変更を見たうえで入れ直すためである。拒否の規則は、auto モードで確認なしに入れることを防ぐ層である（[ADR-0046](0046-chief-of-staff.md) と同じ理由）。規則は、実装の PR で開発の道具の規約に移すまで、[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md) の「要件定義の際に決めたこと（2026-11-19）」にある。
-   **プロトコル番号の規則とマージ時の確かめ。** `--protocol` の番号（プロトコル番号）を上げる PR は、番号を実装の時点の `origin/main` の値にちょうど 1 を足した値にし、マージの時点でも `origin/main` の値 + 1 であることを確かめる。食い違えばマージしない。タスク文書には上げた後の番号を書かず、「`main` の値 + 1」と書く。確かめは、`/runpr` からのマージにも `/mergepr` を単独で使うマージにも当てはめ、マージの直前にも確かめ直す。`mergepr` も番号を持ち、`/mergepr` の定義と版を照合する。
    -   対象は、道具とそれを呼ぶ文書の版を照合する番号で、今は `fixprguard`・`codexreview`・`chief` と、確かめを入れる `mergepr` の 4 つである。確かめは `mergepr` の `prepare` と `merge` が行い、`merge` は GitHub にマージを依頼する直前に、`origin/main` を取り直して確かめ直す。番号を持つ道具は、`internal/<名前>/` の直下のファイルに、`const Protocol = <整数>` の 1 行で番号を置く。この形でない宣言は、確かめが読めない値として止まるか、確かめの対象の道具と文書のガードの一覧を突き合わせるテストが CI で止める。
    -   受け入れた危険は 4 つある。(1) マージの前は、同じ道具の番号を上げる並行するブランチが同じ番号を共有し、片方のブランチから入れた道具がもう片方の文書を受け付けうる（ブランチごとに一意の番号を配る仕組みは、issue #933 の範囲を超えるためである）。(2) マージの直前の確かめ直しと GitHub のマージの間の数秒に、ほかの PR が同じ番号でマージされうる。(3) 番号を上げたブランチを、同じ番号の入った `main` の上へ rebase するか履歴をまとめ直すと、確かめが衝突を見落としうる。(4) 一度消した道具を同じ名前で足し直すと、番号が `1` に戻る。
    -   理由：並行する 2 つのブランチが同じ番号に上げると、git は定数の行の同じ変更を衝突なしにまとめ、文書のガードテストも通り、違う約束が 1 つの番号を共有したまま `main` に入りうるためである。`mergepr` に番号を持たせるのは、確かめを入れる前の `mergepr` や古い定義でマージして、確かめが黙って抜けることを防ぐためである。確かめの仕様は [mergepr_guide.md](../guides/mergepr_guide.md) に、規則の正本は [CLAUDE.md](../../../CLAUDE.md) の「Development Notes」の「Protocol numbers」に、確かめの要件は [0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md) の F-003 にある。
-   **`/mergemain` の取り込みの判定。** `/mergemain` が `main` を取り込むときの決定的な判定と確かめは、`mergepr` のサブコマンドに置く。判定し、確かめるのは、手元と PR の前提、衝突の分類（解消してよい衝突か）、戻してよい状態か、push の先、merge コミットの親、2 つの親の間の衝突の分類、および git の自動の合わせと違うパスの範囲である。エージェントに残すのは、`.md` の両側の変更を保てるかの判断と解消、README と ADR の規則の当てはめと文書の `--protocol` の書き換え、merge コミットの作成、検査、報告だけである。基準の head は、ほかの道具と同じく、サブコマンドの出力をセッションが次の呼び出しの引数で渡し、道具が GitHub と手元の git で確かめ直す。
    -   理由：chief が人に尋ねずに送るコマンドが、手順の読み違いで範囲の外の衝突を解消したり、範囲の外の変更を push したり、人の作業を消したりしないためである。`mergepr` に置くのは、プロトコル番号の行の読み方がすでにあり、新しい道具では入れる手順と許可の規則がすべて新しく要るためである。費用として、`mergemain-` のサブコマンドを変えるたびに `mergepr` の番号が上がって入れ直しが要り、確認なしに許すサブコマンドが `merge` と同じバイナリに入る（[0115 設計書](../../tasks/0115_chief_conflict_resolution/02_architecture.md)「設計の際に決めたこと（2026-11-19）」、[ADR-0046](0046-chief-of-staff.md)）。
-   **契約の同期。** 道具の引数・出力・終了コードは、コマンドの定義と文書のガードが読む契約にする。変えたときは、道具と、それを呼ぶ文書と、ガードを同じ PR で変える。
    -   理由：契約が食い違うと、レビューの信頼の境界が実行時に働かなくなるためである。契約は、[CLAUDE.md](../../../CLAUDE.md) の「Build Commands」「Test Commands」「Development Notes」、[.claude/commands/_context.md](../../../.claude/commands/_context.md) の「Guard tool」の行、[fixprguard_guide.md](../guides/fixprguard_guide.md)、[mergepr_guide.md](../guides/mergepr_guide.md)、[chief_guide.md](../guides/chief_guide.md)、[numreserve_guide.md](../guides/numreserve_guide.md) にある。
-   **例外（副作用のないシェルスクリプト）。** 記録の行を読んで判定するだけで副作用を持たない判定は、コマンドの定義から独立したシェルスクリプトにしてよい。スクリプトは `.claude/commands/_lib/` に置く。呼ぶ手順は、このスクリプトを `origin/main` の commit から作業ツリーの外に取り出して走らせる。失敗は安全な側の判定とし、規則を壊して失敗することを Go のテストで確かめる。今あるのは、Opus のレビューの判断のスクリプト `_lib/opus-review-decision.sh`（[ADR-0042](0042-implementation-model-tiers.md)）と、ローカルの CLI レビューが見たかと Fable の記録が有効かを決める `_lib/review-records.sh`（[ADR-0013](0013-review-setup.md)、[ADR-0050](0050-fable-partial-review.md)）である。`opus-review-decision.sh` は、この例外より前に作られ、常設の Go テストが確かめるのは計画書の値の読み方だけで、判断の規則はタスク 0081 の一度きりの確かめで確かめた。新しいスクリプトは、規則の全体を常設の Go テストで確かめる。
    -   理由：判定は記録の行と `git rev-list` の組み合わせで足り、Go の道具にすると、新しい `cmd/` と `internal/` を足し、利用者にも入れ直しを求めることになるためである。採らなかった案の「判定をコマンドの定義に書いたシェルの手順」では、エージェントが文を読んで判定する。この例外では、テストを持つ 1 つのファイルが判定する点が違う（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md) 1.2 の決定 15）。

## 採らなかった案

退けた案の多くは、判定をエージェントやシェルの手順・ディスクの状態へ残す案、確かめや禁止の範囲を狭める案、または別の道具を増やす案である。

-   **コマンド定義のシェル手順による判定。** 判定をコマンドの定義に書いたシェルの手順にする案は採らなかった。判定が多く、安全側に倒すことが要るので、エージェントの判断では結果が変わらないようにできず、テストで壊して失敗を確かめることもできない（[0030 設計書](../../tasks/0030_fixpr_untrusted_threads/02_architecture.md) §1.1、§1.3）。
-   **`git add -A` による commit。** commit を `git add -A` で行う案は採らなかった。修正のフェーズが作った未追跡のファイル（作業用に作った一時ファイルを含む）も commit に入り、範囲の判定と照らしていない変更が commit されるためである（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md) AC-21、[0030 設計書](../../tasks/0030_fixpr_untrusted_threads/02_architecture.md) §3.6）。
-   **ディスクの状態ファイルによる基準値の保持。** ガードの道具が基準値をディスクのファイルに持つ案は採らなかった。そのファイルは、修正のフェーズのエージェントや build checks が実行するコードから書き換えられる（[0030 設計書](../../tasks/0030_fixpr_untrusted_threads/02_architecture.md) §1.1）。
-   **エージェントによる Codex 最終メッセージの判定。** Codex の最終メッセージの判定をエージェントに任せる案は採らなかった。文法だけで決まる機械的な判定なので、テストのあるコードで行う（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **素の `codex review` 出力やセッション記録による完了判定。** 素の出力には完了を示す欄がなく、セッションの記録にある構造化された結果は公開された仕様ではなく、実行ごとに 2 つできるファイルから選ぶ手順が要る（[0049 設計書](../../tasks/0049_codex_local_review/02_architecture.md) §1.2）。
-   **node スクリプトまたは別道具でのカバレッジ比較。** カバレッジの比較を node のスクリプトで書く、または Go と拡張で別の道具にする案は採らなかった。差分による関数の対応、分類、比較、結果の出し方を 1 か所に保てない。判定の論理は Go に置き、TypeScript は拡張の関数の範囲だけを求める（[0055 設計書](../../tasks/0055_coverage_compare/02_architecture.md) §1.3）。
-   **node だけで完結する `ext-coverage-compare`。** 拡張の lcov は関数の終わりの行を出さないので、関数の範囲を求めるのに TypeScript の構文解析が要り、それは node の側でしか行えない（[0055 設計書](../../tasks/0055_coverage_compare/02_architecture.md) §1.3、§3.5）。
-   **CLAUDE.md への手順記述だけのプロトコル番号確認。** プロトコル番号のマージの時点の確かめを CLAUDE.md に人の手順として書くだけにする案は採らなかった。確かめを忘れたときに、同じ番号の衝突を防げない（[0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）。
-   **マージ前の一意なプロトコル番号配付。** マージの前も並行するブランチが同じ番号を使わないようにする案は採らなかった。ブランチごとに一意の番号を配る仕組みが要り、issue #933 の範囲を超える（[0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）。
-   **`/runpr` だけへのプロトコル番号確認の配置。** `/mergepr` を直接使うと確かめが抜ける（[0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）。
-   **CI 必須チェックと「ブランチを最新にする」設定での確認。** すべての PR で、マージの前に更新が要る（[0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）。
-   **プロトコル番号の決め方の別 ADR 化。** 同じ主題の決定が 2 つの ADR に分かれる（[0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）。
-   **`mergepr` の独立サブコマンドとしてのプロトコル番号確認。** `/mergepr` と `/runpr` の定義から呼ぶ形では、定義が呼び忘れると確かめが黙って抜ける（[0099 設計書](../../tasks/0099_protocol_number_rule/02_architecture.md)「設計の際に決めたこと（2026-11-17）」）。
-   **`/runpr` によるプロトコル番号の自動書き換え。** プロトコル番号が食い違ったときに `/runpr` が番号と文書の `--protocol` を書き換えて自動で直す案は採らなかった。`/runpr` を受け持つモデルに複数の文書を書き換えさせると誤りうる（[0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md)「要件定義の際に決めたこと（2026-11-17）」）。
-   **導入・ビルド禁止の対象を 3 つの道具に限定。** `codexreview`・`numreserve`・`chief` の 3 つに限る案は採らなかった。`fixprguard` と `mergepr` について規則がないままになる（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **インストールのみの禁止。** エージェントに禁じるのをインストールだけにする案は採らなかった。`go run` や `go build` で作ったバイナリで、ブランチの判定を走らせられる（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **開発のためのソース実行を含めた全面禁止。** 道具を変えるタスクの実装の確かめが、利用者頼みになる（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **指示の文だけで導入・ビルド禁止を守る形。** auto モードでは、確認なしに道具を入れられる（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **`.claude/settings.json` での `go build ./cmd/*` と `go run ./cmd/*` の拒否。** 開発のための例外が使えなくなる（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **開発のための例外から副作用禁止条件を外す形。** 道具を変えるタスクで、ソースから走らせた道具が取り消せない記録の tag を作るなどしうる（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **導入・ビルド禁止を規約だけに書く形。** `CLAUDE.md` の「Tool Execution Safety」に写しを置かないと、規約を読まないエージェントには届かない（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **導入手順の主語の統一を別 issue に分ける形。** 規則と手順がぶつかったまま `main` に入る（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）。
-   **`/mergemain` 用の新しい道具の作成または `fixprguard` の拡張。** 新しい道具では、入れる手順、利用ガイド、プロトコル番号、許可の規則がすべて新しく要る。`fixprguard` の push を merge コミットにも広げると、`/fixpr` の修正のコミットが基準の head だけを親とするという保証が弱まる（[0115 設計書](../../tasks/0115_chief_conflict_resolution/02_architecture.md)「設計の際に決めたこと（2026-11-19）」）。

## 結果

-   判定の誤りは、道具のテストで見つかる。壊して失敗を確かめた結果は、そのテストを加えたコミットのメッセージに残る。
-   エージェントが手順を読み違えても、範囲外の commit・push や、信頼の境界の迂回は、道具が実行時に止める。
-   道具を変えたマージの後は、利用者が `make install-<tool>` を実行し直す必要がある（`mergepr`・`fixprguard`・`codexreview`・`chief`・`numreserve`）。再インストールを忘れると、`fixprguard` は `--protocol` の食い違いで止まる。ただし、同じ道具の番号を上げる並行するブランチの間では、番号が同じなので止まらない。確かめを入れた後は、`mergepr` を入れ直さないと、`/mergepr` が `--protocol` の食い違いで止まる。
-   拡張のカバレッジの比較には Go と node の両方が要る。`make ext-coverage-compare` も Go の道具を呼ぶ。
-   プロトコル番号を上げるタスクの文書は、上げた後の番号を書かず「`main` の値 + 1」と書く。マージの時点で番号が食い違えば、マージは止まる。番号と文書を直し、道具を入れ直してから、マージをやり直す。
-   道具の引数・出力・終了コードは、コマンドの定義と文書のガードが前提にする契約になる。変えるときは、道具・文書・ガードを同じ PR で変える。
-   エージェントが道具をインストールもビルドもしない規則は、指示の文と、Claude Code にだけ効く `make install-*` と `go install` の拒否の規則で守る。拒否の規則は、タスク 0112 の実装の PR が `.claude/settings.json` に足すまでは効かない。`go build` と `go run` は指示の文だけで守る。エージェントが従わない危険は受け入れる。`/runpr` などを走らせる OpenCode が `CLAUDE.md` を読むかは確かめておらず、読まない場合の危険も受け入れる。
-   `PATH` にある道具のビルド元のコミットは `go version -m` の `vcs.revision` で分かるが、コミットしていない変更からの入れ直し（`vcs.modified` にだけ出る）、`go run` や別の場所のバイナリでの実行、`-buildvcs=false` のビルドは分からない。
-   プロトコル番号を上げる PR で利用者がブランチから入れ直した道具は、`/runpr` が途中で止まるか PR をやめたときも、利用者が `main` から入れ直すまで、すべての worktree で使われる。
-   道具を入れる手順に来たエージェントは止まるので、利用者が入れる手間が増える。

## 経緯

- 2026-10-21 `/fixpr` と `/runpr` の著者の信頼・修正の範囲・git の状態の判定と、build checks・commit・push・返信を、`fixprguard` に移した。判定の基準値を会話に持ち、副作用を信頼できない本文を読んだエージェントにさせない形にした（[0030 要件定義書](../../tasks/0030_fixpr_untrusted_threads/01_requirements.md)）
- 2026-10-28 CodeRabbit CLI が失敗したときの Codex CLI の実行と、最終メッセージの形の判定を、`codexreview` に移した。時間と大きさの上限、環境変数、ツールの許可も、テストのあるコードで扱う（[0049 要件定義書](../../tasks/0049_codex_local_review/01_requirements.md)）
- 2026-11-02 テストを削除する前後のカバレッジの比較を、Go の `coveragecompare` で行うことにした。拡張の関数の範囲だけを TypeScript の補助に求め、`make coverage-compare` と `make ext-coverage-compare` から使う（[0055 要件定義書](../../tasks/0055_coverage_compare/01_requirements.md)）
- 2026-11-11 chief の判定と送信を Go の道具 `chief` に置いた。自動送信の記録・送信の停止・ロックなどを状態のディレクトリから判定に読み返すことは、例外として認めた。その記録は、送るコマンドとタスクを決めない。送ることについては、送ることを止める条件か、自動送信の候補の workspace を名指して確かめ直す条件としてだけ使い、通知の記録と送信の記録は、通知の重複の防止と並べ方、履歴と「実行中」の行の表示にだけ使う（[0088 設計書](../../tasks/0088_chief_of_staff/02_architecture.md)「設計の際に決めたこと（2026-11-11）」）
- 2026-11-16 chief が workspace を作ったときの記録を、判定に読み返す記録の例外に加えた。その記録は、削除の候補の workspace を名指して Herdr・git・`origin/main` の文書で確かめ直す条件としてだけ使う（[0092 設計書](../../tasks/0092_chief_task_workspace/02_architecture.md)「設計の際に決めたこと（2026-11-16）」）
- 2026-11-17 プロトコル番号を上げる PR は、番号を `origin/main` の値 + 1 にし、マージの時点で確かめて食い違えばマージしないことにした。タスク文書には上げた後の番号を書かない。規則の正本を CLAUDE.md に置き、対象を `fixprguard`・`codexreview`・`chief` と、番号を持たせる `mergepr` の 4 つとした（[0099 要件定義書](../../tasks/0099_protocol_number_rule/01_requirements.md)「issue #933 で決めたこと」「要件定義の際に決めたこと（2026-11-17）」）
- 2026-11-17 タスク番号と ADR 番号を決める道具 `numreserve` を、`main` から入れる Go の開発の道具に加えた（[0100 設計書](../../tasks/0100_number_reservation/02_architecture.md)「設計の際に決めたこと（2026-11-17）」）
- 2026-11-17 プロトコル番号の確かめを `mergepr` の `prepare` と `merge` に置き、`merge` は GitHub にマージを依頼する直前に `origin/main` を取り直して確かめ直すことにした。番号は `internal/<名前>/` に `const Protocol = <整数>` の 1 行で置くこととし、規則の正本を CLAUDE.md の「Development Notes」の「Protocol numbers」に置いた（[0099 設計書](../../tasks/0099_protocol_number_rule/02_architecture.md)「設計の際に決めたこと（2026-11-17）」）
- 2026-11-17 記録の行を読むだけの判定を、`origin/main` から取り出して走らせるテスト付きのシェルスクリプトにしてよいとする例外を加えた（[0098 設計書](../../tasks/0098_fable_partial_review/02_architecture.md)「設計の際に決めたこと（2026-11-17）」）
- 2026-11-19 エージェントが道具をインストールもビルドもしないことを、利用者が `main` から入れる道具のすべて（`fixprguard`・`mergepr` を含む）に当てはめ、ビルドの意味と開発のための例外を定め、プロトコル番号を上げる PR のブランチで入れ直すのも利用者とした。規則は指示の文と、`make install-*` と `go install` を拒む `.claude/settings.json` の規則で守る（[0112 要件定義書](../../tasks/0112_dev_tools_convention/01_requirements.md)「要件定義の際に決めたこと（2026-11-19）」）
- 2026-11-19 `/mergemain` が `main` を取り込むときの決定的な判定と push の前の確かめを `mergepr` のサブコマンドに置き、`/mergemain` の送信の記録を、head ごとに 1 回の自動送信を止める条件と「解消できず」の表示に使うことにした（[0115 設計書](../../tasks/0115_chief_conflict_resolution/02_architecture.md)「設計の際に決めたこと（2026-11-19）」）
