# 設計への引き継ぎ事項（0079_codexreview_state_scope）

要件定義の際に挙がった、設計（`02_architecture.md`）で決める事項である。`02_architecture.md` は、項目ごとに、とった方法か、当てはまらない理由を記録する。

## H-01: HEAD のブランチの ref を、どう読んで比べるか

AC-06 と AC-10 は、HEAD のブランチの ref の変化を `head` と `refs` で示し、`changed_refs` にその ref を入れることを求める。HEAD がブランチにいるとき、今の `readHead` が読む commit（`git rev-parse --verify HEAD`。`internal/codexreview/state.go:82`）は、そのブランチの ref を経由して決まるので、ref の変化は commit の変化としても現れる。ブランチの ref をどう読むか（HEAD のブランチ名を使って `for-each-ref` の対象を 1 つに絞るか、など）を決める。読んだ出力の決まった形と、形に合わない出力を読めないときとして扱う方法（AC-08 の 3。今の `parseRefs`、`state.go:97-115`）も決める。HEAD が detached のときは、ref を読まない。

## H-02: 使われなくなるコード

stash の一覧を比べなくなると、`stashArgs`（`state.go:43`）、`itemStash`（`state.go:19`）、`gitState` の `stash` のフィールド（`state.go:51`）とその読み取り（`state.go:72-75`）が使われなくなる。`make deadcode` は関数だけを報告し、フィールドや定数は報告しないので、消すものを設計で列挙する。`refs` の読み取り（`state.go:42`・`64-71`）、`describeRefs`・`parseWorktrees`（`state.go:146-182`）は、H-01 の決め方によって残るか変わる。

## H-03: テストの作り方

AC-01〜AC-03 は、Codex CLI の実行の間に、ほかのブランチ・タグ・stash が変わる状況を作る必要がある。今の `cmd/codexreview/main_test.go:188` は、ブランチ `made-by-codex` を作る偽の `codex` で `state_changed` を確かめている。この入力は、AC-01 では `state_changed` にならない。HEAD のブランチの ref だけを動かす入力（AC-06）、ほかの worktree が checkout しているブランチを動かす入力（AC-01）、HEAD のブランチの ref を消す入力（AC-08 の 2）をどう作るかを決める。`CLAUDE.md` の「Testing Strategy」に従い、比較の範囲を元の広さに戻すと各テストが失敗することを確かめる。

## H-04: 比べるものを記述している文書とコメント

`docs/dev/conventions/package_reference.md` の `internal/codexreview` の行は、比べるものとして「branches and tags, and stash list」を挙げている。`internal/codexreview` のパッケージのコメント、`types.go:40-41` の `Changed`・`ChangedRefs` のコメント、`state.go` の `refsArgs` 周りのコメント、`run.go:425` 周りも、同じ内容を前提にしている。どれを改めるかを決め、AC-09〜AC-11 と矛盾しないようにする。
