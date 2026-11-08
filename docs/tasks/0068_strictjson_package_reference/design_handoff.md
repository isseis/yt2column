# 設計への申し送り：internal/strictjson の使い手の一覧

要件の作成で挙がった、設計（`02_architecture.md`）で決める事項。`02_architecture.md` は、項目ごとに採った方法か、当てはまらない理由を記録する。

-   **H-01 一覧の文の形と読み取り方:** `package_reference.md` の `internal/strictjson` の行は、表の 1 つのセルに部品の契約の説明と使い手の一覧（`Shared by ...` の文）を並べて持つ。テストが一覧を読み取れるように、一覧の文の形（並べる順序、区切り、コードスパン）と、その文を見つける手段を決める。形に合わない書き方（コードスパンでない名前など）をどう扱うか、順序を決める場合はテストで順序まで確かめるかも決める。
-   **H-02 コードの使い手の求め方:** import を読む手段（`go/parser` で import の宣言だけを読む、`go list` を呼ぶなど）を決める。手段は、ビルドタグ付きのソース（`//go:build test` など）も読む必要がある（要件 3 章の定義）。タグを指定しない `go list` は、そうしたファイルを落とす。go のツールが無視するディレクトリ（`testdata`、`_` や `.` で始まる名前）の下の `.go` ファイルを数えないことを、その手段でどう保つかも決める（2026-11-08 の時点で、`cmd/` と `internal/` の下にそうしたファイルはない）。
-   **H-03 テストの置き場所:** CI の `doc-guard` ジョブが実行するパッケージ（CLAUDE.md「Development Notes」、`.github/workflows/ci.yml`）のどれに置くか。既存の `TestPackageReferenceListsPackages` と同じ `internal/pipeline` に置くなら、文書の読み取りの補助（`packageReferenceRows` など）を共有できるかを確かめる。
-   **H-04 CLAUDE.md の guard の列挙:** CLAUDE.md「Development Notes」は、文書を読む Go の guard を列挙している。新しいテストをその列挙に足す必要があるかを決める。
-   **H-05 失敗の経路の確かめ方:** 実際のリポジトリは AC-01 を満たすので、AC-06〜AC-09 の失敗の経路は、そのままでは観測できない。比べる処理に作った文書とソースを与えるテストで確かめるか、実装の後に一覧を壊して失敗を確かめ、コミットメッセージに記録するか（CLAUDE.md「Every test must be able to fail for its stated reason」）、あるいは両方かを決める。
