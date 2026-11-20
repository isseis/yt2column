# 設計への申し送り（0118）

要件定義の時点で挙がった、設計（`02_architecture.md`）で決める事項。`02_architecture.md` は、各項目にとった方針か、当てはまらない理由を記録する。

## H-01: 手動の PR レビューの完了をどの値で判定するか

**懸念:** AC-12・AC-13 は、CodeRabbit の完了の印と、上限に当たったことを読むことを求める。0031 より前は、`StatusContext` の `CodeRabbit` が `SUCCESS` であることと、本文の「between `<sha>` and `<headRefOid>`.」の行で判定していた（`docs/tasks/0031_codex_pr_review/01_requirements.md` の 1 節）。一方、上限に当たっても CheckRun が `pass`（説明は `Review rate limited`）になる観測がある（PR #404）。

**なぜ重要か:** 状態だけで判定すると、上限に当たった依頼をレビュー済みとしうる。本文の行を読むなら、本文をセッションに出さない原則（[ADR-0015](../../dev/adr/0015-untrusted-review-threads.md)、4.2）を保つ絞り込みが要る。今の `runpr.md` step 3 は、Codex の要約コメントを `--jq` の中で判定し、決まった値だけを出している。

**候補:**

-   F-001 の記録から、完了・レビューしたコミット・上限を示す値を選び、`--jq` の中で形を判定して、真偽値・16 進と確かめた SHA・URL・時刻だけを出す。
-   本文をセッションに出さないことを確かめるガードテストを、今の要約コメントの絞り込みのものから移す。

**関連:** F-001、F-004、AC-03・AC-12・AC-13、4.2

## H-02: LAST_REVIEWED と見たかの判定の範囲

**懸念:** AC-15 は、エスカレーションの範囲を、最後に完了した手動の PR レビューの後、それがなければ PR の全体とする。今は、LAST_REVIEWED が「なし」のときはファイルを走らせずに `ask` とし（`.claude/commands/_lib/pr-review-escalation.md` の Inputs）、`_lib/review-records.sh` の `coverage` も LAST_REVIEWED を起点にする。

**なぜ重要か:** 手動の PR レビューはほとんどの PR で行われないので、LAST_REVIEWED はほぼいつも「なし」になる。今の規則のままだと、どの PR でも利用者に尋ねる形になり、AC-16〜AC-18 の推奨の分け方が働かない。この変更はレビューの関門の変更に当たる（[ADR-0013](../../dev/adr/0013-review-setup.md)）。

**候補:**

-   LAST_REVIEWED が「なし」のときの起点を、`origin/main` との merge-base にする。`review-records.sh` の `coverage` が `none` を受けたときの起点を確かめ、合わなければ変える。
-   エスカレーションの答え（`request`・`ask`・`skip`）を、マージの質問の推奨に写す規則を `runpr.md` に 1 か所で書く。

**関連:** F-005、AC-15〜AC-19

## H-03: ほかの道具のプロトコル番号を上げるか

**懸念:** F-009 で `fixprguard` が拒否するメンションが増える。F-010 で自動レビュアーの一覧が変わるが、一覧は文書が `--reviewer` で渡すので、道具の中にはない。chief は `/runpr` の報告の形を読む（F-012）。

**なぜ重要か:** `CLAUDE.md` の「Protocol numbers」は、道具と文書の約束が変わるときに番号を上げ、`main` の値 + 1 にすることを求める。拒否するものが増えるだけの変更を約束の変更とみなすかで、利用者の入れ直しの要否と、並行するブランチとの衝突が変わる。

**候補:**

-   `fixprguard` の拒否を広げることが、古い文書と新しい道具、新しい文書と古い道具の組み合わせで何を起こすかを並べ、上げるかを決める。
-   chief の番号は、F-012 で読み取りの形が変わるときだけ検討する。

**関連:** F-008、F-009、F-012、AC-29・AC-31

## H-04: 撤去する道具とガードテストの削除の確かめ方

**懸念:** F-008 は `cmd/codexreview` と `internal/codexreview` を消す。製品のコードと一緒に、その `_test.go`、`cmd/yt2column/codex_review_docs_test.go`、`internal/doctestutil` の関連する部分も消える。`CLAUDE.md` の「Testing Strategy」は、テストの削除をそれだけの commit にし、`make coverage-compare` で網羅が下がらないことを確かめるよう求める。

**なぜ重要か:** 製品のコードごと消えるテストは、比べる相手の関数も消える。比べ方の結果（消えた関数は一覧に出るだけで失敗しない）がそのまま使えるかを決めないと、実装の PR で手順が止まる。

**候補:**

-   製品のコードとテストを一緒に消す commit で `make coverage-compare` を走らせ、消えた関数が一覧に出るだけで PASS になることを確かめる。
-   残る製品のコード（`internal/fixprguard`、`internal/doctestutil` など）のテストを消すものは、別の commit に分けて比べる。

**関連:** F-008、AC-26・AC-30

## H-05: PR の分け方と移行の順序

**懸念:** 4.4 は、このタスクの PR を Codex の連携が動いている間の `/runpr` で回すことを求める。F-002・F-003 を入れる PR がマージされると、以後の PR は新しい `/runpr` で回る。`codexreview` を撤去する PR より前に、ローカルの CLI レビューが `codexreview` を呼ばなくなっている必要がある。

**なぜ重要か:** 順序を誤ると、途中の PR で、ローカルの CLI レビューが撤去された道具を呼んで失敗したり、`/runpr` が来ない Codex のレビューを待ったりする。並行するほかのタスクのブランチも、マージの後の `main` の手順で回る（AC-22）。

**候補:**

-   文書と手順の変更（F-002〜F-007、F-009、F-010）を先に入れ、道具の撤去（F-008）を後にする。
-   各 PR がマージされた時点で、`main` の手順と道具がそろって動くことを、PR ごとに確かめる。

**関連:** F-002〜F-010、AC-22、4.4

## H-06: 古い記録の行の読み方

**懸念:** AC-22 は、`Codex local review:` の行と `(codex)` を持つ `Local CLI reviewed:` の行を、それを持つブランチで今と同じ意味に読むことを求める。今の `Local CLI review: not run` の行は `(coderabbit: <kind>; codex: <kind>)` の形を持つ（`.claude/commands/_lib/local-cli-review.md:287`）。`review-records.sh` の正規表現も `(coderabbit|codex)` を受ける（`.claude/commands/_lib/review-records.sh:189`）。

**なぜ重要か:** 新しく書く行の形を変えるとき、古い形を読めなくすると、マージの前に作られたブランチで、見た範囲が見ていない範囲になる。新しい形と古い形を両方読むと、読み方の規則が残る。

**候補:**

-   新しく書く `not run` の行の形を CodeRabbit CLI だけのものにし、古い形も読み続ける。
-   古い形を読む規則は、古い形を持つ開いたブランチがなくなったら外せるものとして、その条件を書く（`_context.md` の古い値の表と同じ扱い）。

**関連:** F-006、AC-20〜AC-22

## H-07: Codex にだけ触れる ADR と文書の直し方

**懸念:** ADR-0040・0046・0047・0050 は、背景・理由・結果で Codex の PR レビューに触れる。例えば ADR-0050 は、「PR を開く前に直せば、Codex の最初の PR レビューが修正を見る」ことを、PR を開く前に Fable を走らせる理由にしている。`CLAUDE.md` の「Declare, don't infer」には `codexreview` の例外がある。

**なぜ重要か:** AC-37 は、現在の仕様を書く文書が Codex を今の振る舞いとして書かないことを求める。ADR の直しが、古い参照の修正（編集上の修正）で済むか、決定の変更になるかで、経緯の行と PR の説明の書き方が変わる（`docs/dev/conventions/adr.md` の「Editorial corrections」）。

**候補:**

-   ADR ごとに、Codex に触れる箇所が決定・理由・結果のどれかを分け、理由が崩れる箇所は人に確かめてから直す。
-   `CLAUDE.md` の例外と、ガードテストや CI の `doc-guard` のジョブが挙げる `codexreview` は、F-008 と同じ PR で外す。

**関連:** F-011、AC-37・AC-39
