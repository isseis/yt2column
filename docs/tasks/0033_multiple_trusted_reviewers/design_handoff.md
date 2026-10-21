# 設計への申し送り（0033 multiple_trusted_reviewers）

[01_requirements.md](01_requirements.md) の起草で挙がった、設計（`02_architecture.md`）で決める事項。設計書は、各項目をどう扱ったかを記録する。

## H-01: ツールと文書の変更の順序と `protocol` の番号

-   **事項:** `fixprguard` の変更と、`fixpr.md`・`runpr.md`・`_context.md` の変更を、どの順で入れるかを決める。あわせて、`protocol` の番号を上げるかと、古いツールで止まったときに利用者に何を示すか（再インストールを促すか）を決める。
-   **理由:** 文書が 2 つの `--reviewer` を渡し、インストール済みのツールが古いと、古いツールは使い方のエラーで止まる（要件定義書 4.3）。0030 は、ツールを含む PR をマージして再インストールした後に文書を切り替えた（`docs/tasks/0030_fixpr_untrusted_threads/02_architecture.md` 8 章の 6）。`--reviewer` の繰り返しは、1 回だけ渡す呼び出しと両立する（AC-04）。そのため、番号を上げなくても、古いツールは誤った判定をせずに止まる。一方、番号を上げれば、古いツールが止まる理由が「`--protocol` が合わない」とはっきり出る。
-   **関連:** AC-04・AC-07・AC-10、4.3

## H-02: `_context.md` の一覧の書き方と、文書のガードの読み方

-   **事項:** 自動レビュアーの一覧と待つ PR レビュアーを、`_context.md` の表にどう書くか（1 行に並べるか、行を分けるか、名前の区切り）と、文書のガードがそれをどう読み出すかを決める。
-   **理由:** 今のガードは、"Automated reviewer" の行の最初のバッククォートの中を名前として読む（`cmd/yt2column/fixpr_docs_test.go:341-346`）。一覧にすると、名前を取りこぼしたり、説明の文中のコード（`[bot]` など）を名前として読んだりしない読み方が要る。AC-02 の重複と空の一覧の検出も、この読み方に依存する。
-   **関連:** AC-01〜AC-03・AC-10・AC-11

## H-03: 名前の一覧の型と、空の一覧を通さない保証

-   **事項:** `internal/fixprguard` で名前の一覧を表す型と、`NewGitHub` の引数の形を決める。空の一覧と重複を、構築の時点で拒否する形にするかを決める。
-   **理由:** 今は、`NewGitHub` が使えない `Login` を拒否し、判定する呼び出しの前に `requireReviewer` で確かめる（`internal/fixprguard/github.go:35-63`）。一覧にしても、空の一覧で判定できないことを、型で保証する必要がある（`CLAUDE.md` の「Enforce invariants with the type, not with convention」）。重複の拒否を、フラグの解析（`cmd/fixprguard`）と型の構築のどちらで行うかも決める。
-   **関連:** AC-04・AC-07

## H-04: 判定の表のテストと、壊して失敗を確かめる対象

-   **事項:** 名前が 2 つのときの判定の表のテストの行と、各テストが失敗することを確かめるための壊し方を決める。
-   **理由:** 判定のテストには、1 つめの名前だけに一致する `Bot` と、2 つめの名前だけに一致する `Bot` を、別々の行として用意する（AC-05）。どの壊し方で各行が失敗するかは、実装の後に壊して確かめる（`CLAUDE.md` の「Every test must be able to fail for its stated reason」）。`processThread` の `authorType`（AC-08）と `reply` の書き込みの前の判定（`internal/fixprguard/write.go:98-126`）も、同じ一覧を使うことを確かめる。
-   **関連:** AC-05・AC-06・AC-08
