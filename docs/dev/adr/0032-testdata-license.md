# ADR-0032: testdata に置く実データは再配布できるライセンスの動画のものに限り、取得した人を特定できないようにする

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

パーサのテストは、`testdata/` に保存した実際の取得結果で行う（[0002 要件定義書](../../tasks/0002_ytdlp_transcript_source/01_requirements.md) AC-13、[CLAUDE.md](../../../CLAUDE.md) の「Testing Strategy」）。そのため、外部から取得したデータを公開のリポジトリに置くことになる。

字幕・タイトル・概要欄は著作物でありうるので、置くデータは再配布できるものでなければならない（[0002 設計書](../../tasks/0002_ytdlp_transcript_source/02_architecture.md) §1.4「保存する実データの条件」）。また、取得した結果には、取得した人を特定できる情報が混じる。たとえば `yt-dlp` の info.json は、署名付きの URL に取得した人の IP アドレスを埋め込む（同 付録A、[testdata/README.md](../../../testdata/README.md)）。

## 決定

-   **`testdata/` に置く実データは、再配布が許諾されたライセンス（CC BY など）の動画から取得したものに限る。** 出典とライセンスを [testdata/README.md](../../../testdata/README.md) に明記する。字幕・タイトル・概要欄は著作物でありうるためである（[0002 設計書](../../tasks/0002_ytdlp_transcript_source/02_architecture.md) §1.4「保存する実データの条件」）。規則の正本は [security.md](../security.md) の §7「リポジトリに保存する実データ」で、個々のファイルの出典・加工の内容・作り直し方は testdata/README.md が持つ。
-   **取得した人を特定できる情報は、コミットする前に取り除く。** 何を取り除くかは security.md の §7 に従い、何を取り除いてどう置き換えたかを、取り除いた値そのものは書かずに、ファイルごとに testdata/README.md に記録する。取得した結果には、取得した人を特定できる情報が混じるためである（0002 設計書 付録A）。
-   **ページ全体のスナップショットは、リポジトリにコミットしない。** リポジトリの外に置き、テストはそれを参照しない。YouTube 自身のマークアップ・スクリプトと他チャンネルの動画の情報を含み、ログインした状態で保存した場合はアカウントの情報も含むためである。規則と置き場所は security.md の §7 にある。

## 採らなかった案

-   **通常の YouTube 標準ライセンスの動画の字幕を保存する。** 字幕・タイトル・概要欄は著作物でありうる。通常の YouTube 標準ライセンスの動画の全文字起こしを公開リポジトリに保存するのは適切でない（[0002 設計書](../../tasks/0002_ytdlp_transcript_source/02_architecture.md) §1.4「保存する実データの条件」・付録A）。
-   **info.json から、パーサが使わないメンバーを除いて保存する。** info.json の大半は使わないメンバーだが、IP アドレスを含む URL の置き換えのほかは実出力のまま保ち、パーサのテストの忠実性を優先した。履歴が膨らむ場合の対策として、将来の検討に残した（[0002 設計書](../../tasks/0002_ytdlp_transcript_source/02_architecture.md) 付録A）。

## 結果

-   `testdata/` に実データを加えるときは、動画のライセンスを確かめ、出典とライセンスを testdata/README.md に書く。字幕のない動画の出力のように、CC BY でない動画の出力は、調べるために取得しても保存しない（[0002 設計書](../../tasks/0002_ytdlp_transcript_source/02_architecture.md) §1.4）。
-   info.json を取得し直したときは、コミットする前に同じ置き換えをやり直し、IP アドレスが残っていないことを確かめる（[testdata/README.md](../../../testdata/README.md)）。
-   テストに使える実データは、再配布できるライセンスの動画のものに限られる。タスク 0002 は、統合テストの既定の動画にも CC BY の動画を選んだ（[0002 設計書](../../tasks/0002_ytdlp_transcript_source/02_architecture.md) §1.4「統合テストの既定の動画」）。
-   ページ全体のスナップショットはリポジトリにないので、HTML の抜粋を作り直すときは、リポジトリの外に置いたスナップショットから切り出すか、開発者ツールで要素を直接コピーする（[testdata/README.md](../../../testdata/README.md) の「文字起こしパネルの HTML の抜粋」）。
-   ライセンスの確認と、取得した人を特定できる情報の除去は、テストで止まらない。コミットする人の確認と PR のレビューに頼る。テストが確かめるのは、コピーした字幕のサンプルの記録だけである。`TestCopiedTranscriptSurveyRecords`（`cmd/yt2column/copied_transcript_docs_test.go`）は、`testdata/` の `*.transcript_copy.txt` のそれぞれについて、testdata/README.md の「コピーした字幕（文字起こしパネルからのコピー）」の節がファイル名と動画の URL を挙げていることを確かめる。

## 経緯

- 2026-09-30 リポジトリに保存する `yt-dlp` の実出力を、再配布が許諾された CC BY の動画のものに限り、出典とライセンスを `testdata/README.md` に明記することにした。info.json は、取得した人の IP アドレスを含む URL の文字列だけを置き換え、ほかは実出力のまま保存することにした（[0002 設計書](../../tasks/0002_ytdlp_transcript_source/02_architecture.md) §1.4「保存する実データの条件」・付録A）
