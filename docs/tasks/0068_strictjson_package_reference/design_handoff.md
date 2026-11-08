# 設計への申し送り：internal/strictjson の使い手の一覧

要件の作成で挙がった、設計（`02_architecture.md`）で決める事項。`02_architecture.md` は、項目ごとに採った方法か、当てはまらない理由を記録する。

-   **H-01 一覧の文の形と読み取り方:** `package_reference.md` の `internal/strictjson` の行は、表の 1 つのセルに部品の契約の説明と使い手の一覧（`Shared by ...` の文）を並べて持つ。テストが一覧を読み取れるように、一覧の文の形（並べる順序、区切り、コードスパン）と、その文を見つける手段を決める。順序を決める場合（たとえば名前の順）、テストで順序まで確かめるかも決める。
-   **H-02 コードの使い手の求め方:** import を読む手段（`go/parser` で import の宣言だけを読む、`go list` を呼ぶなど）と、ビルドタグ付きのソース（`//go:build test` の `test_helpers.go` など、`_test.go` でないテスト用のファイル）を使い手の判定に含めるかを決める。`TestPackageReferenceListsPackages` は、そうしたファイルもパッケージを数える根拠にしている（`internal/pipeline/pipeline_test.go:625-648`）。
-   **H-03 テストの置き場所:** CI の `doc-guard` ジョブが実行するパッケージ（CLAUDE.md「Development Notes」、`.github/workflows/ci.yml`）のどれに置くか。既存の `TestPackageReferenceListsPackages` と同じ `internal/pipeline` に置くなら、文書の読み取りの補助（`packageReferenceRows` など）を共有できるかを確かめる。
-   **H-04 CLAUDE.md の guard の列挙:** CLAUDE.md「Development Notes」は、文書を読む Go の guard を列挙している。新しいテストをその列挙に足す必要があるかを決める。
-   **H-05 ADR-0020 の直し方:** AC-12 の場合に、ADR-0020 の「決定」のずれの文を消すか書き換えるか、「結果」の「利用者を足すときは、`package_reference.md` の一覧を同じ変更で更新する」にテストが確かめることを足すか、「経緯」に項目を足すか（規約の "History entries" は、決定を作るか変えるたびに 1 項目とする）を決める。
