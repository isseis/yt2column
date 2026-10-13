# 設計への申し送り：文書だけの変更でも CI で文書の同期 guard を走らせる

本書は、[01_requirements.md](01_requirements.md) のレビュー（と起草）で挙がった事項のうち、設計の段階で決めるものを集める。`02_architecture.md` は、各項目について、採った方針か、当てはまらない理由を記録する。

## H-01: `doc-guard` ジョブのコマンドの置き場

-   **事項:** `doc-guard` ジョブが実行するコマンドを、`ci.yml` に直接 `go test -tags test ./cmd/yt2column` と書くか、`Makefile` にターゲット（例: `test-doc-guard`）を足して `make` で呼ぶかを決める。
-   **理由:** 実行コマンドを 1 か所に保つと、対象パッケージを変えるときに更新する場所が明確になる。一方、既存の `test` ジョブは `make test-ci` を呼び、`secret-scan` ジョブは `git grep` を直接書いており、CI のコマンドは両方の形が混在している。Makefile に足せば手元でも同じコマンドを再現できる。
-   **候補の方針:** 対象は `./cmd/yt2column` だけなので、`ci.yml` にコマンドを直接書く（`secret-scan` と同じ形）。Makefile のターゲットを足すのは、複数のジョブ・手順から同じコマンドを呼ぶ必要が出たときに限る。採らなかった場合の理由を `02_architecture.md` に記す。
-   **関連:** F-001、AC-01、AC-02、保守性

## H-02: `doc-guard` が対象とするパッケージの範囲

-   **事項:** `go test -tags test ./cmd/yt2column` の対象範囲で足りるか、文書を読む guard が他のパッケージにもあるかを確認し、対象を決める。
-   **理由:** `0012` は文書の同期 guard を `cmd/yt2column`（`extension_docs_test.go`・`docs_test.go`・`makefile_test.go`）に集約した。`internal/pipeline/pipeline_test.go` の静的な契約の guard は文書ではなくコードの契約を固定するもので、文書だけの変更では壊れない。対象を広げると `./...` の全テスト（race 付き）を文書だけの PR で走らせることになり、性能の要件に反する。
-   **候補の方針:** 対象は `./cmd/yt2column` とする。文書を読む guard が他パッケージに見つかった場合は、対象に加える（そのときは `-run` を付けず、パッケージを列挙する）。確認の手順と結果を `02_architecture.md` に記す。
-   **関連:** F-001、AC-02、性能

## H-03: `-tags` の選択（`test` のみで足りるか）

-   **事項:** `doc-guard` が `-tags test` のみで足りるか、`integration` タグが要るかを確認する。
-   **理由:** 文書の guard は `//go:build test` で、integration を要しない。`integration` を付けると外部リソースを要求するテストがビルド・実行対象になりうる。CI の `lint` ジョブは `--build-tags test,integration` を使うが、これは lint が両ビルドを検査するためであって、`doc-guard` の実行範囲の根拠にはならない。
-   **候補の方針:** `-tags test` のみとする。`./cmd/yt2column` の文書 guard が `integration` を要しないことを確認し、結果を `02_architecture.md` に記す。
-   **関連:** F-001、AC-02、互換性

## H-04: ジョブの権限と実行条件の明示

-   **事項:** `doc-guard` ジョブに `permissions` を明示するか、workflow レベルの `permissions: contents: read` に委ねるか。また、将来 `check-changes` の分類が変わっても `doc-guard` が独立に走り続けることをどう保証するか。
-   **理由:** `secret-scan` は workflow レベルの権限に委ね、`needs` も `if` も持たない。`doc-guard` も同じ形にすれば、分類の変更に影響されない。`ci.yml` を読むレビューが「`needs` が付いていないこと」を確認できる必要がある。
-   **候補の方針:** workflow レベルの `permissions` に委ね、ジョブに `needs`・`if`・追加の `permissions` を付けない（`secret-scan` と同じ）。`02_architecture.md` にこの形と、その理由（分類の変更に依存しないこと）を記す。
-   **関連:** F-001、AC-01、セキュリティ
