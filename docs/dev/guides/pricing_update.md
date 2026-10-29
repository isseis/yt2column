# 料金表の更新

`internal/pricing` は、LLM を呼び出す費用を、プロバイダが公開するトークンあたりの料金から推定する。料金は `internal/pricing/rates.go` の定数の料金表にあり、CLI が出力する推定額はこの料金表だけから求まる。料金が変わったとき、またはモデルを追加したときは、料金表と、その値を固定するテストを 1 つの変更で一緒に更新する。

## 料金を出どころで確かめる

- DeepSeek: <https://api-docs.deepseek.com/quick_start/pricing>
- Anthropic (Claude): <https://platform.claude.com/docs/en/about-claude/pricing>

各ページで、対象のモデルについて次を確かめる。

- 入力、キャッシュの読み取り、キャッシュの書き込み、出力のそれぞれの料金（100 万トークンあたり）。
- 料金の組の選び方（以下「方式」）。単一の料金（一律の方式）、週ごとのピークの時間帯、プロンプトの長さのしきい値のいずれかである。
- 時刻で変わるモデルでは、ピークにあたる曜日と、分単位まで指定した時刻の範囲。CLI はこれらを UTC で読む。DeepSeek のピークの時間帯は月曜日から金曜日の 01:00-04:00 と 06:00-10:00 UTC である。

## 料金表を更新する

1. `internal/pricing/rates.go` の `rateTable` にある該当の項目について、料金と、その方式が使うフィールドだけを編集する。方式が使わないフィールドは 0 のままにする。0 以外の値があると `validate` が拒否する。
2. 項目の上の `Source: <url> (checked <date>)` のコメントと、時刻で変わるモデルのピークの規則の上のコメントを、確かめたページと日付に書き換える。`TestRateTableSourceComments` が、各項目に出どころの行と確認した日付があることを確かめる。
3. 新しいモデルでは、方式とその方式が使うフィールドを持つ項目を加え、モデルを列挙するテスト（`TestRateTableModels`）にモデル名を加える。`TestRateTableValid`（実際の料金表に `validate` をかける）と `TestValidateRejects`（各規則を、その規則だけに違反する表にかける）を実行する。

## 手で計算したテストを更新する

期待値の金額は、出どころで確かめた料金から手で計算する。料金表やコードから導いてはならない。

- `internal/pricing/pricing_test.go` の `TestEstimateCategories`: 各モデルの 1 工程の費用。そのテストのコメントに計算式（`count * rate / 1e6 USD`）を示している。計算式と `want` の文字列を、料金と一緒に更新する。
- `TestEstimatePeakWindow`: 時間帯が変わったときの、DeepSeek のピークの時間帯とそれ以外の境界。
- `TestEstimatePromptTier`: しきい値が変わったときの、Claude Haiku のプロンプトの長さに応じた料金の階層。
- `internal/pricing/amount_test.go` の `TestAmountFormat`: 金額を小数点以下 6 桁と 4 桁に丸める処理。

## 料金の変更がほかに触れるもの

`TestCostDocsContract` は、[cli.md の「推定の料金」](cli.md#推定の料金)の節を、その節の中の一覧ではなくコードの書式の定数と照らす。そのため、料金の変更で `cli.md` のその節を直す必要はない。それでも、料金表の内容に合わせて手で直さなければならない箇所が残るので、自分で更新する。

- README の "Estimated cost" 節は、DeepSeek のピークの時間帯（"Monday through Friday during 01:00-04:00 and 06:00-10:00 UTC"）を述べている。ピークの時間帯が変われば、その文は古くなる。
- README の "Output format" の Slack への投稿の例と、[cli.md の「`--out` のファイル」](cli.md#--out-のファイル)の例は、料金表から求めた推定額（`0.0094`）を示している。`TestOutputLayoutDocsContract` が実際の出力と比べるので、料金が変われば、2 つの例の金額を更新するまでそのテストは失敗する。
- README の "Configuration" の節と `project_overview.md` はモデルの名前を挙げているので、対応するモデルが増減したときだけ更新が必要になる。

`package_reference.md` の `internal/pricing` の行は、パッケージが公開する型を列挙している。料金の変更では変えない。この行が古くなっていないかは、PR のレビューで読んで確かめる。
