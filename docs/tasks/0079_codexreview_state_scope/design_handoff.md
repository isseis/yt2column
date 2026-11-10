# 設計への引き継ぎ事項（0079_codexreview_state_scope）

要件定義の際に挙がった、設計（`02_architecture.md`）で決める事項である。`02_architecture.md` は、項目ごとに、とった方法か、当てはまらない理由を記録する。

## H-01: HEAD のブランチの ref を、どう読んで比べるか

AC-06 は、HEAD のブランチの ref の変化を `head` として示すことを求める。HEAD がブランチにいるとき、今の `readHead` が読む commit（`git rev-parse --verify HEAD`。`internal/codexreview/state.go:82`）は、そのブランチの ref を通して読まれる。そのため、ref の変化は commit の変化として必ず現れる。ブランチの ref を別に読む（`refs/heads/<branch>` を指定した `for-each-ref` など）か、HEAD の読み取りで足りるとするかを決める。足りるとする場合は、その根拠を設計に書く。比べるものを読めないときの扱い（AC-07）も保つ。

## H-02: 使われなくなるコードと、プロトコルを上げる順序

`changed_refs` を外すと（AC-09）、`describeRefs`・`parseWorktrees`（`state.go:146-182`）と、`refs`・`stash` の読み取り（`state.go:42-43`・`64-75`）が使われなくなる。`make deadcode` が報告するので、消すものを決める。プロトコルを 2 に上げる（AC-10・AC-11）ときの、ツールと共通手順を変える順序も決める。どちらも同じ PR で変えるか、分けるかである。その PR の `/runpr` の前に、その PR のブランチから入れ直すこと（[ADR-0014](../../dev/adr/0014-deterministic-checks-in-go.md) の「道具は `main` からインストールして使う」）も、手順として決める。

## H-03: テストの作り方

AC-01〜AC-03 は、Codex CLI の実行の間に、ほかのブランチ・タグ・stash が変わる状況を作る必要がある。今の `cmd/codexreview/main_test.go:188` は、ブランチ `made-by-codex` を作る偽の `codex` で `state_changed` を確かめている。この行は、AC-01 では `state_changed` にならない入力になる。HEAD のブランチの ref だけを動かす入力（AC-06）と、ほかの worktree が checkout しているブランチを動かす入力（AC-01）を、どう作るかを決める。`CLAUDE.md` の「Testing Strategy」に従い、各テストが、比較の範囲を戻すと失敗することを確かめる。

## H-04: 比べるものを記述している文書

`docs/dev/conventions/package_reference.md` の `internal/codexreview` の行は、比べるものとして「branches and tags, and stash list」と `ChangedRefs` を挙げている。`internal/codexreview` のパッケージのコメントと `types.go` のコメントも、同じ内容を述べる。どれを改めるかを決め、AC-08〜AC-10 と矛盾しないようにする。`CLAUDE.md` の document test の例外（「the `--protocol` value of the `codexreview` calls」）に当たるテストが、プロトコルの値をどう確かめているかも確かめる。
