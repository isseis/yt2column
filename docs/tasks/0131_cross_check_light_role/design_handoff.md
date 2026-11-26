# 設計への申し送り：クロスチェックの部分レビューを重い側と軽い側の役割に分け、強めたプロンプトと単独実行のコマンドを足す

`01_requirements.md` の起草時に挙がった設計・実装上の論点を記録する。`02_architecture.md` でそれぞれの扱い（採用した方針、または該当しない理由）を記録すること。

## H-01: Antigravity で `claude-opus-5-5-high` が使えるかの実測

-   **懸念:** Antigravity の軽い側のモデル `claude-opus-5-5-high` は、利用者の環境に残っていた 2026-11-21 の `swarm(action: "models", all: true)` の出力で `models` にあり、`quota_remaining` もあった。しかし、実際に走らせてはいない。タスク 0120 で実測したのは `claude-opus-5-5-medium` だけである（`docs/tasks/0120_harness_independent_commands/02_architecture.md` 1.4）。1 PR あたりの割り当ての消費（数 %）も、`claude-opus-5-5-medium` で測った値である。
-   **重要な理由:** 一覧から消えていれば、Antigravity の軽い側の部分レビューはいつも `not run unavailable` になる。`high` で時間切れになる、出力の形が違う、割り当てを大きく使う、といった場合も、本タスクの目的（軽い側の部分レビューを走らせる）が Antigravity で果たせない。
-   **候補案:** `/mkarch` の段階で、Antigravity の `swarm(action: "models", all: true)` で `models` に `claude-opus-5-5-high` があることと `quota_remaining` を確かめ、結果を設計書に記録する。なければ、要件の決定の変更として `01_requirements.md` に戻す（要件定義の際に決めたこと）。あわせて、実 PR で 1 回走らせて、時間と指摘の形を確かめるかも決める。
-   **関連 F-/AC-:** F-001、AC-04

## H-02: 役割の名前と「Harness model mapping」の表の形

-   **懸念:** 今の表の `Role` 列は `high`・`standard`・`low`・`cross-check` の 4 つに限られる（`.claude/commands/_context.md:222-246`）。2 つの判定のスクリプトは、同じ文法をそれぞれの写しで検証している（`.claude/commands/_lib/review-records.sh:379-380`、`.claude/commands/_lib/high-review-decision.sh:206`、`cmd/yt2column/harness_mapping_cases_test.go`）。次の 3 点を決める必要がある。
    -   新しい役割の名前（今の `cross-check` を重い側として残すか、両方を改名するか）。
    -   Claude Code の軽い側の effort `high` の表し方（表には effort の列がない。Claude Code の `Agent` の起動で effort を渡す方法）。
    -   Antigravity の重い側（`fable`、Antigravity からは起動しない）の行の形（`Launch` の値、検証パターン）。
-   **重要な理由:** 両スクリプトは `origin/main` の表を読むので、表と両スクリプトとテストを同じ PR で変えないと、判定がフェイルクローズで失敗する。effort はモデルの ID に出ないので、記録からは effort `high` で走ったかを確かめられない。
-   **候補案:**
    -   重い側を `cross-check` のまま残し、軽い側に `cross-check-light` などを足す。改名しないので、既存の記録と文書への影響が小さい。
    -   effort は、表に列を足すか、アダプタの `pinned-worker` の節に役割ごとの起動の引数として書く。
    -   Antigravity の重い側の行は、`Launch` に `none`（OpenCode の行と同じく検証パターン `-`）を使うか、引き継ぎを表す新しい値を足す。
-   **関連 F-/AC-:** F-001・F-006、AC-03〜AC-06・AC-36

## H-03: 判定のスクリプトが PR の種類を知る方法

-   **懸念:** 判定のスクリプトは `cross-check <ENTRY> <HEAD> <PLAN>` の引数しか受け取らず、計画書のパスからは、重い側の PR か軽い側の PR かを区別できない（`.claude/commands/_lib/cross-check-review.md:101-111`）。F-002 の照合には、PR の種類の入力が要る。種類を渡さない古い呼び出しはどちらの役割の記録も数え（AC-12）、本タスク自身の PR は新しいスクリプトがまだ `origin/main` にない間も開ける必要がある（AC-13）。今の cross-check のモードは引数の数が違うと失敗する（`.claude/commands/_lib/review-records.sh:888`）。
-   **重要な理由:** 種類を誤ると、重い側の PR が軽い側の記録で有効になる（AC-08 に反する）。宣言した種類を信頼する危険は受け入れた（要件定義の際に決めたこと）が、受け付ける値の形と、引数の足し方は決める必要がある。`/crosscheck` が種類を決める方法には、`CLAUDE.md` の「Declare, don't infer」が関わる。
-   **候補案:**
    -   省略できる引数を 1 つ足し（例: `heavy`・`light`）、呼び出し元（コマンドの手順）が宣言する。省略はどちらの役割の記録も数え、ほかの値は失敗とする（AC-11・AC-12）。
    -   本タスクの PR は、手順が引数を渡す前にスクリプトを `main` に入れる順にするか、古いスクリプトが引数を拒否したときに引数なしで呼び直すかを決める（AC-13）。どちらでも、PR の種類を受け付けない版のスクリプトと新しい手順の組み合わせで記録が有効と判定されることを、どう確かめるかも決める。
    -   `/crosscheck` は、利用者が引数で種類を宣言する形（例: `/crosscheck heavy`）か、ブランチが足す文書（`01_requirements.md`・`02_architecture.md`・`03_implementation_plan.md`）と計画書の `PR を作成した` の行から決める形かを選ぶ（AC-22）。
-   **関連 F-/AC-:** F-002・F-004、AC-08〜AC-13・AC-22・AC-23

## H-04: Antigravity の停止の位置と、再開の入口

-   **懸念:** 重い側の部分レビューは、`_lib/task-pr-handoff.md` Part B step 1 でローカルの CLI レビューと並列に起動する。チェックポイントの入口（Part B step 1.5）と push の直前の入口（step 2.6）でも、単独の経路として起動しうる。Antigravity で止まる位置（AC-14）を、並列のローカルの CLI レビューを先に済ませてからにするか、Part B に入る前にするかを決める必要がある。また、`/mkreq` と `/mkarch` は再実行すると止まる（`.claude/commands/mkreq.md:84-95`、`.claude/commands/mkarch.md:51`）。そのため、再実行の入口と、その条件（AC-19）を決める必要がある。
-   **重要な理由:** ローカルの CLI レビューを済ませずに止まると、再実行したコマンドが Part B step 2.5 でまとめて見ることになる。済ませてから止まると、その記録の commit の後で `/crosscheck` の commit が足される。再実行の条件が緩いと、別のタスクのブランチや PR のあるブランチで誤って PR を開く。簡易プロセスの再開（`_lib/simple-process.md` の「Resuming a stopped run」）とも区別が要る。
-   **候補案:**
    -   Part B step 1 の起動の入口で、判定が `MISSING` なら、ローカルの CLI レビューを済ませ、その記録の commit の後で止まる。チェックポイントの入口と push の直前の入口でも、単独の経路の代わりに止まる。
    -   再実行で続けるのは、step 1 の既存の文書の検索で見つかったブランチが、次の条件をすべて満たすときに限る。条件を満たせば、Part B step 1 の後（README の同期と step 1.5）から続ける。続けた後の commit が `/crosscheck` の記録を無効にしないこと（AC-16）は、step 1.5 のチェックポイントの印で保つ。
        -   AC-14 の停止の後のブランチである（停止を示す印の commit などで見分ける）。
        -   PR がない。
        -   作業ツリーがきれいである。
-   **関連 F-/AC-:** F-003、AC-14〜AC-20

## H-05: `/crosscheck` のレビューの範囲と手順の組み立て

-   **懸念:** `/crosscheck` は、最後のクロスチェックの記録が `not run` のときにも走る（AC-23）。しかし、判定のスクリプトは `not run` の記録を有効（`VALID`）と判定するので、今の `MISSING <START>` からは範囲が得られない。範囲の決め方と、今の「The standalone path」と「Entry 3」をどう使い回すかを決める必要がある。
-   **重要な理由:** 範囲が狭いと、走らなかった部分レビューが見るはずだった commit を見ない。広いと、すでに有効な `run` の記録が見た箇所をもう一度見て、費用が増える。
-   **候補案:**
    -   最後に数える `run` の記録の END の後から HEAD までを範囲とし（なければ merge-base から HEAD まで）、その間の `not run` の記録の範囲をすべて含める。範囲に merge commit があれば、今の規則どおり merge-base から HEAD までとする（AC-23）。今の `MISSING <START>` の START は最初の覆われていない commit の第 1 親なので、`not run` の記録の範囲を飛ばしうる。そのまま使わない。
    -   最後の記録の状態（`run` か `not run` か）と範囲の始点は、判定のスクリプトの新しい出力にする（AC-23。判定はスクリプトだけが決めるという規則、`.claude/commands/_lib/cross-check-review.md:864-865`）。新しいモードにするか、今のモードの出力を足すかを決める。
    -   修正の確かめと記録の commit は、「The standalone path」の手順 2〜4 を使い回す。
-   **関連 F-/AC-:** F-004、AC-23〜AC-26

## H-06: 範囲の終点より後の情報を読ませない方法

-   **懸念:** 今のプロンプトでは、ワーカーが自分で `git diff <RANGE>` と `git log <RANGE>` を実行し、作業ツリーのファイルも読む（`.claude/commands/_lib/cross-check-review.md:533-536`）。AC-32 は、範囲の終点より後の情報（`origin/main`、後の commit）を読むことを禁じる。チェックポイントの入口と push の直前の入口では END が HEAD なので、作業ツリーは END の内容である。範囲の終点が HEAD でない場合があるかも確かめる必要がある。
-   **重要な理由:** 再現実験は、Fable の範囲の終点の commit でレビューさせることを前提にしていた（issue #1057）。後の情報を読むと、範囲の外の直しを根拠に指摘を取り下げたり、`main` の変更を混ぜたりする。
-   **候補案:**
    -   ワーカーには、ファイルを `git show <END>:<path>` で読むように指示する。`origin/main`・`main`・ほかのブランチの ref を読まないことも制約に加える。
    -   END が常に起動の時点の HEAD であることを設計で確かめ、それなら作業ツリーの読み取りも許す。
-   **関連 F-/AC-:** F-005、AC-32
