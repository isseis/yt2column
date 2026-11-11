# 評価記録：Claude と OpenAI の組み合わせの評価と、本番の構成の見直し

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-11-11 |
| Review date | - |
| Reviewer | - |
| Comments | - |

## 1. 概要

[要件定義書](01_requirements.md)（以下「要件」）の F-001〜F-006 の評価の結果を記録する。手順は[設計書](02_architecture.md)（以下「設計」）§3、作るものと順序は[実装計画書](03_implementation_plan.md)（以下「計画」）による。本書の節と AC の対応は設計 §3.8 の表のとおりである。

**結論の要約:** （ステップ 5-1 で書く）

## 2. 手順

設計 §3 の手順を具体化したコマンドと、スクリプトとサブコマンドの名前を記す。設計が決めていることは繰り返さない。

### 2.1. 置き場所と記法

- `_eval/` は `docs/tasks/0087_openai_combination_eval/_eval/`、`0087/` は `~/yt2column-eval/0087/` を指す。§2.3 のコマンドは、リポジトリのルートで、そこに書いた変数 `X`・`E` と関数 `evaltool` を定めたシェルで実行する。
- `evaltool` は `go run ./$X/evaltool` を指す。サブコマンドは `aggregate`（集計）・`estimate`（見積もりと上限）・`rates`（単価）・`check-record`（照合）・`material`（採点の素材）・`launch`（起動）・`reproduce0018`（0018 の再現）である。
- zsh のスクリプトは `run.sh`（1 件の実行と構成の変数の表）・`hashlist.sh`（ハッシュの一覧）・`retire.sh`（やり直しの前の移動）・`rates-compare.sh`（単価の比較）・`secretscan.sh`（キーの値の検索）・`selftest.sh`・`reproduce0018.sh`・`repo_impact.sh`（確かめ）である。スクリプトは macOS（`shasum`、BSD の `stat`）を前提にする。
- 採点の指示は `_eval/scoring-prompt.md`、採点のエージェントの定義は `_eval/scorer-agent.md` である。値のファイルは `_eval/bounds-input.json`（見積もりと上限の入力と出典）・`_eval/baseline.json`（基準の構成の値と出典）・`_eval/bounds.tsv`（`estimate` が書いた上限の表）である。

### 2.2. リポジトリの外のファイルの形

設計 §2.2 の配置に、次のファイルの形を加える。

- **実行の記録（`log/<構成 ID>/<記事>.when`）:** `run.sh` が 1 行に 1 項目、`<鍵>=<値>` で書く。鍵は `config`・`article`・`after`（`none`・`generation`・`shorten`・`review`）・`llm`・`review_llm`・`ytdlp_path`・`keep_cache`・`binary_sha256`・`start`・`end`・`exit` で、時刻は UTC の RFC 3339 である。`end` と `exit` は CLI が終わった後に加わる。
- **承認の記録（`0087/approval`）:** 1 行に 1 つの承認を `<時刻（RFC 3339）>\t<構成 ID のカンマ区切り>\t<上限（USD）>` で追記する（計画 §1.2 の決めたこと 8）。
- **起動の記録（`0087/launch.log`）:** `launch` が起動の前に `<時刻>\t<構成 ID>\t<記事のカンマ区切り>` を追記する。
- **起動のロック（`0087/launch.lock`）と各実行の出力（`0087/launch-output/`）:** `launch` は、確かめの前にロックのファイルを排他的に作り、起動ごとのトークンを書く。起動したすべての実行が終わるまで持ち、その後に消す。ロックがあれば `launch` は止まるので、2 つの起動が同じ使った額で上限を確かめることはない。`launch` はトークンを `run.sh` の引数 `--launch-token` で渡し、`run.sh` は、`--preflight` のない実行（API を呼ぶ実行）を、トークンがロックの中身と一致する場合だけ始める。`run.sh` 自身の出力（止まった理由など）は `launch-output/<時刻>-<構成 ID>-<記事>.txt` に残る。`launch` が途中で止まってロックが残った場合は、実行中のものがないことを確かめてから人が消す。`launch` は、確かめた評価のルート（`-root` の親）も `--eval-root` で `run.sh` に渡す。
- **やり直しの前の移動:** `retire.sh` は、`prepayment.tsv` が今の実行（`cur`）を指していれば、その行を移した先の `f<N>` に書き換える。
- **支払い前と確かめた実行（`0087/prepayment.tsv`）:** 1 行に 1 つの実行を `<構成 ID>\t<記事>\t<回>` で書く。回は `cur`（今の実行）か `f<N>`（`failed/` に移した実行）である（計画 §1.2 の決めたこと 7）。
- **判定のファイル（`0087/score/<段>/judgments.tsv`）:** 1 行目は `article	config	criterion	interp	literal	judge	human_check`。観点の鍵は `format_choice`（形式の選択）・`format_rules`（形式の規則）・`evidence`（根拠）・`length`（長さ）・`notation`（表記）・`coverage`（要点の網羅）・`importance`（重要度の順）・`structure`（構成）・`readability`（読みやすさ）・`title_lead`（タイトル・リード文）・`redundancy`（冗長さ）・`constraint1`〜`constraint3`（製品の制約 1〜3）である。判定者は `script`（長さだけ）・`llm`・`human`（人が LLM の判定を変えた）である。
- **人の確認の印（`0087/score/1/human-confirmed`）:** 1 段目の判定を人が確認した時刻（RFC 3339）を書く。`launch` は、これがなければ `B-*` を起動しない（計画 §1.2 の決めたこと 10）。
- **機械的な判定の出力（`0087/score/<段>/check_article.txt`）:** 構成ごとに `config <構成 ID>` の行と、その構成の記事に当てた `check_article.py` の出力を並べる。
- **採点の素材（`0087/score/<段>/material/<動画 ID>/`）と名前の対応（`0087/score/<段>/mapping.tsv`）:** `material` が作る。対応のファイルは素材のディレクトリの外にある。

### 2.3. コマンド

```sh
X=docs/tasks/0087_openai_combination_eval/_eval
E=~/yt2column-eval
evaltool() { go run ./$X/evaltool "$@" }
# 構成の変数の表（run.sh だけが持つ）
$X/run.sh configs
# ハッシュの一覧（最初の一覧は前の一覧なし。2 回目以降は前の一覧と diff する）
mkdir -p -m 700 $E/0087/hash
$X/hashlist.sh $E/0087/hash/1-initial.tsv - base=$E/0018/base/B3 work=$E/0018/work/B3 cache=$E/cache
# 見積もりと上限の表
evaltool estimate -input $X/bounds-input.json -runsh $X/run.sh -write-bounds $X/bounds.tsv
# 実行の前の確認（API キーを外し、0087/preflight/ に出力する）
$X/run.sh run <構成 ID> <記事> $E/0087/bin/<コミット>/yt2column --preflight [--after-review]
# 1 つの構成の 10 件（またはやり直しの 1 件）の起動。API を呼ぶ実行は必ずこれで起動する
evaltool launch -root $E/0087 -runsh $X/run.sh -cli $E/0087/bin/<コミット>/yt2column \
  -stage1 $E/0087/score/1 -bounds-input $X/bounds-input.json -bounds $X/bounds.tsv \
  -config <構成 ID> [-retry <記事>] -now "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
# やり直しの前の移動
$X/retire.sh $E/0087 <構成 ID> <記事>
# 採点の素材
evaltool material -root $E/0087 -out $E/0087/score/<段> -runsh $X/run.sh -stage <段>
# 集計（-exclude は F-006 の手順 3 で除いた候補）
evaltool aggregate -root $E/0087 -base0018 $E/0018 -runsh $X/run.sh -bounds $X/bounds.tsv \
  -baseline $X/baseline.json [-exclude <構成 ID>]
# 照合（集計・見積もり・単価の出力のファイルを渡す）
evaltool check-record -record docs/tasks/0087_openai_combination_eval/04_evaluation.md <表のファイル>...
```

### 2.4. 設計からの逸れ

- **採点の素材の記事:** 設計 §3.6 の手順 2 は記事のファイルからタイトルと本文を取り出す。`material` は、同じタイトルと本文を検証・推敲の保存した出力から取り、記事のファイルがそのタイトルと本文を含むことを確かめる。記事のファイルの出典の行（日本語の固定の文言）を解析しないためである。字幕は、実行ごとのキャッシュの写しを CLI と同じ読み取り口（`YtDlpSource`）で読み、時刻つきの文にして素材に置く。採点のサブエージェントはファイルを読む道具だけを持ち、json3 を自分で変換できないためである。
- **キャッシュの間隔:** 設計 §3.5 は、同じ OpenAI のモデルを使う直前の構成の終了から 30 分を待つ。`launch` は、同じ OpenAI のモデルを使うほかのすべての構成の最後の実行の終了から 30 分を待つ。設計より厳しい側である。
- **起動の関門の強化:** `launch` は、ロックで同時の起動を止め、`run.sh` は `launch` のトークンなしに API を呼ぶ実行を始めない（§2.2）。`launch` は、支払い前と確かめた実行を除いて 2 回やり直した記事の 3 回目のやり直しと、最後の承認より前の時刻を止める。どれも設計の関門（§3.5）を手順の誤りから守るためで、設計の条件は変えない。
- **素材の検査:** `material` は、素材のどのファイルにもその段の構成 ID とモデル ID が含まれないことを確かめ、含まれれば止まる。

### 2.5. 道具の確かめ（ステップ 1-5）

PR-1 のブランチのコミット `c4c32567bc69`（squash merge の前のコミット）で実行した。どれも API を呼ばない。`repo_impact.sh` だけは、その 1 つ前のコミット `a8d5b7f37291` で実行した。間のコミットが変えたのは `_eval/evaltool/` の `record.go`・`record_test.go` だけで、`./...` に含まれないので、`repo_impact.sh` の 3 つの出力は変わらない。この後に変えたのは本節と計画書だけである。

**`selftest.sh`:** `evaltool` のテスト（合成の入力だけを使う）・`go vet`・`golangci-lint`（`Makefile` の版）・`gofumpt`、`hashlist.sh` の変更の検出、`secretscan.sh` の検出を確かめた。

```text
selftest: ok   go test ./docs/tasks/0087_openai_combination_eval/_eval/evaltool
selftest: ok   go vet ./docs/tasks/0087_openai_combination_eval/_eval/evaltool
selftest: ok   golangci-lint v2.13.2 ./docs/tasks/0087_openai_combination_eval/_eval/evaltool
selftest: ok   gofumpt ./docs/tasks/0087_openai_combination_eval/_eval/evaltool
selftest: ok   hashlist: first list
selftest: ok   hashlist: unchanged tree gives an empty diff
selftest: ok   hashlist: changed content gives a diff
selftest: ok   hashlist: restored content gives an empty diff
selftest: ok   hashlist: changed mode gives a diff
selftest: ok   secretscan: finds a key value in a file
selftest: ok   secretscan: finds the sk- prefix
selftest: ok   secretscan: passes a clean file
selftest: ok   secretscan: fails with no key variable
selftest: ok   secretscan: fails on a missing path
selftest: ok   secretscan: never prints the value
selftest: all checks passed
selftest exit=0
```

`hashlist.sh` が固定のモードを書くように壊すと「changed mode gives a diff」が、固定のハッシュを書くように壊すと「changed content gives a diff」が、`secretscan.sh` の値の比較を外すと「finds a key value in a file」が失敗することも確かめた（`selftest.sh` を加えたコミットのメッセージに記録し、PR の説明に載せる）。

**`reproduce0018.sh`:** 集計と同じ関数で、0018 の B3・C03 のログから 0018 評価記録 §4.8 の値（B3 の合計 3.615544・平均 0.361554・最大 0.610696 USD、所要時間の平均 1:53.4・最大 3:06.2、C03 の合計 3.572464 USD・1:52.1・3:05.2）を再現した。`baseline.json` の推定コストの平均と所要時間の平均との一致も `evaltool` が確かめた。字数の調整の時間（roNZ-rLxhUE-1 の 54 秒）は、定数ではなく B3 のやり直しのログ（開始から `cost: shorten run` の `at=` まで）から求めている。

```text
table reproduce0018 keys=1
config	cost_sum_usd	cost_avg_usd	cost_max_usd	cost_max_article	duration_avg	duration_max	duration_max_article
B3	3.615544	0.361554	0.610696	roNZ-rLxhUE-1	1:53.4	3:06.2	roNZ-rLxhUE-1
C03	3.572464	0.357246	0.602652	roNZ-rLxhUE-1	1:52.1	3:05.2	roNZ-rLxhUE-1
end
reproduce0018: matches the 0018 evaluation record
exit=0
```

**`repo_impact.sh`:** マージベースと HEAD で、`go list ./...`・`make test` が実行するテストの名前・`make lint` の出力が同じだった（要件 §4.5、設計 §7.3）。

```text
repo_impact: same list (42 lines)
repo_impact: same tests (1336 lines)
repo_impact: same lint (4 lines)
repo_impact: base 188ed2cac1b0d9414cf17da35978302ee0d37312, head a8d5b7f372914741e0acf1dc811d64d543bc97bc
exit=0
```

**`rates` と `rates-compare.sh`（AC-15）:** `rates` の `claude-opus-5-5` の単価（入力 4、キャッシュの読み取り 0.2、キャッシュの書き込み 5、出力 20 USD/MTok）は、`986449c6` の料金表（`internal/pricing/rates.go:114`）の値と一致した。`rates` の出力は §6 の表である。B3 の CLI のコミット `2fc2e2a` との比較は次のとおりで、同じだった。評価に使うコミットでの比較はステップ 1-7 で行う。

```text
evaluation commit c4c32567bc69: base:rates{input:4000000,cacheRead:200000,cacheWrite:5000000,output:20000000},
B3 commit 2fc2e2a: base:rates{input:4000000,cacheRead:200000,cacheWrite:5000000,output:20000000},
rates-compare: same
exit=0
```

**`secretscan.sh`:** 環境にある API キーの値と `sk-` の接頭辞は、`_eval/` と本書のどちらにもなかった。`OPENAI_API_KEY` はこのときの環境になかったので比べていない。PR の作成の後に `_eval/` か本書を変えた場合は、マージの前に実行し直す。

```text
secretscan: the value of ANTHROPIC_API_KEY is absent
secretscan: the value of DEEPSEEK_API_KEY is absent
secretscan: the value of SLACK_WEBHOOK_URL is absent
secretscan: no sk- key prefix
exit=0
```

**`check-record`（骨組み）:** 本書の骨組み（本節を書いた後のもの）の印が、同じコミットの `aggregate`（0087 の実行はまだない）・`estimate`・`rates` の出力の 28 の表と一致した（`check-record: 28 tables match`）。評価の記録の照合はステップ 5-1 で行う。

## 3. 基準の構成

基準の構成は 0018 の B3 である（要件の決めたこと 3）。判定の出典は [0008 評価記録](../0008_column_prompt/02_evaluation.md) §6.2（物差し 2）で、推定コストと所要時間の出典は [0018 評価記録](../0018_article_quality/02_evaluation.md) §4.7.2・§4.8 である。

**工程ごとの保存した出力の識別子と生成の日付（AC-01）:** 生成は `0018/base/B3/`、字数の調整と検証・推敲は `0018/work/B3/` の保存した出力である。「work/ の生成と同じ」は、`work/B3/` の生成の保存した出力の識別子が `base/B3/` と同じことを示す。日付は保存した出力の `call_time`（UTC）である。

<!-- check-record: baseline-artifacts -->
| 記事 | 生成の識別子 | 生成の日付 | work/ の生成と同じ | 字数の調整の識別子 | 検証・推敲の識別子 |
|---|---|---|---|---|---|
| roNZ-rLxhUE-1 | 03c1ede250eb2c3c47c5852517591a118a9a538bf6dbbed6e1f4f5a0b878d8a2 | 2026-10-23 | yes | 20b0f8a8b5cf47e68aa6bd7f4b5a125a23cf275b34e7b1e850f84d0075d5cf23 | 78624d95ca3c9dc8a5c603ca53323e9c57aa1ea1ee5b74fe0dec4a820fbc5601 |
| roNZ-rLxhUE-2 | 5bda51c848e127581a5de52775a303e88aaab1764a2eb8490a880ea34ae44590 | 2026-10-23 | yes | - | 13dc7ac055c3e327ee41f959d2c69b5034d0cf37b32c53477c45c1c85eecef8e |
| 18tbwwPM91A-1 | 1e3318a27b36f02dd18aca1ddeb8b5b9a0fdaf0f7a5fe7f805bf424d5692180c | 2026-10-23 | yes | - | 17387553f769c32219d5c5aa569a0c6be8a42488da04418190d2064281d71c26 |
| 18tbwwPM91A-2 | e5fed9a5b44e405ea9118cdaad6c906f97ec3e742dba756298bfaa42f979acfd | 2026-10-23 | yes | - | 2db2cc5dfdb67e73a5caa1c2378c59343221aae803c0dd7c5cf0d8839f5d6cbe |
| zW1RBacSqjk-1 | 91df9b370bfebe180a757d97cd616cd6a61be53f4b386b3878ad1379db34d1c0 | 2026-10-23 | yes | - | 21c5886598378c1433c35360931dd645c2cf1aafa9c79be7bed10159ff8e3a18 |
| zW1RBacSqjk-2 | 84a90816f744317836154fd3291ad9900385af7d4a86b2a342cef425f6651371 | 2026-10-23 | yes | - | 7d61102bab1393165bcb492d53e24f3a223c8821f689bb7044cc440028c80c2d |
| leE02uqgaKg-1 | 433784761850152cb0a2bd731486b78ae2c6ced7f8ddde03254610efdeb921da | 2026-10-23 | yes | - | daee50d92b7c33987b83aaa5269c51968437e8f20fc406ce4a6997c9c14d74a5 |
| leE02uqgaKg-2 | 62e7949eaf11428fb1f882ac95d4b0117d461775758d0c62287c1992c3fe3156 | 2026-10-23 | yes | - | 908b77d396be83428da059a7b4465ef4c12e549f6dd50c0a3cbe82c89fdd43c8 |
| EqnGQDU71-Y-1 | b4a117332422b30bf1ddb6d2d45e6bbbe10a7536c25b833b95e398ead8f06f84 | 2026-10-23 | yes | - | a31d260307d6c080bf703a3b59c631d9575f1f1ebeeef448e212891a078d6f2f |
| EqnGQDU71-Y-2 | 5c6d599b6e2401b64cbf8844de7c550848455724bb85401d2b28b7e183d1c4f2 | 2026-10-23 | yes | - | 6452b35de7729b3e2ddda9d62cd090c82e350078d737f910e1fe73612d793215 |

**基準の値（AC-21）:** `baseline.json` の値である。推定コストの平均と所要時間の平均は、`reproduce0018.sh` が 0018 のログから求め直して一致を確かめた（§2.5）。

<!-- check-record: baseline -->
| 構成 | 合格（解釈） | 合格（文言） | 推定コストの平均（USD） | 所要時間の平均 |
|---|---|---|---|---|
| B3 | 8 | 7 | 0.361554 | 1:53.4 |

**実行の条件の違い（AC-01）:** （ステップ 1-6 で、CLI のコミットと Claude アダプタの出力の上限の既定の違いを書く）

**動画ごとの必須の観点の不合格の数（AC-21）:** 0008 評価記録 §6.2 の表から、物差し 2 の解釈による判定で数えた。leE02uqgaKg が 1 件（-2 の「根拠」）、残りの 4 本が 0 件である。18tbwwPM91A-2 の不合格は品質の観点「構成」の 1 による。EqnGQDU71-Y-2 の「根拠」は文言どおりの判定だけの不合格である。値は §10 の動画ごとの比較の表の「基準の構成」の列に出る。

**製品の制約 1〜3（AC-23）:** 0008 評価記録 §6.3 のとおり、3 つとも B3 にある（制約 1 は leE02uqgaKg-2、制約 2 は 18tbwwPM91A-2、制約 3 は EqnGQDU71-Y-2）。§10 の「制約が直ったか」の表の B3 の行に出る。

## 4. 素材と保護

**字幕のキャッシュ（AC-02）:** 評価用のキャッシュはリポジトリの外の `~/yt2column-eval/cache/` で、5 本（roNZ-rLxhUE・18tbwwPM91A・zW1RBacSqjk・leE02uqgaKg・EqnGQDU71-Y）とも 2026-10-08 に取得したスロット `a` である（0018 評価記録 §4.9.1）。実行ごとに写しを使い、`YT2COLUMN_YTDLP_PATH=/usr/bin/false` と `--keep-cache` で字幕を取り直さない（設計 §3.1）。

**実行の条件（AC-02）:** すべての実行の記録が `yt-dlp` を呼ばない設定と `--keep-cache` を持つことを数える。

<!-- check-record: run-conditions -->
| 項目 | 値 |
|---|---|
| runs | 0 |
| ytdlp_path_false | 0 |
| keep_cache | 0 |
| all_kept_conditions | - |
| binary_sha256 | - |

**ハッシュの一覧とバックアップ（AC-04）:** （ステップ 1-6・3-1・4-1 で記録する）

**拒否された入力（AC-05）:** （ステップ 3-1・4-1 で記録する。拒否がなければ「なし」と書く）

## 5. 承認と費用

**見積もり（AC-19）:** `estimate` で、`bounds-input.json` の入力（0077 設計書 §1.4 の `roNZ-rLxhUE` の行と B3 の値）から求めた。入力はキャッシュの書き込みの単価で数え、キャッシュの読み取りはないとみなした。Claude の検証・推敲は B3 の 9 件の平均（0.100807 USD）である。字数の調整は「上乗せ」の列に、10 件すべてで走った場合の額を示す。構成 A の字数の調整は走らない見込みなので上乗せを数えない（設計 §3.2）。

<!-- check-record: estimate -->
| 構成 | 段 | 見積もり（USD） | 字数の調整の上乗せ（USD） |
|---|---|---|---|
| C-sol | 1 | 1.181310 | 0.375950 |
| C-astra | 1 | 6.081130 | 1.935750 |
| A-sol | 1 | 0.450030 | - |
| A-astra | 1 | 2.428750 | - |
| B-sol | 2 | 1.739350 | 0.375950 |
| B-astra | 2 | 4.660450 | 1.935750 |
| stage1+B-sol | - | 11.880570 | 2.687650 |
| stage1+B-astra | - | 14.801670 | 4.247450 |
| all | - | 16.541020 | 4.623400 |

設計 §3.5 の概算と比べると、構成ごとの値は同じ（丸めの範囲）である。合計の行は設計の概算（1 段目と `B-astra` 約 14.9、6 構成 約 16.6）より 0.1 USD 低い。設計は丸めた値を足し、本表は丸める前の値を足したためである。

**上限（AC-20）:** 工程ごとの上限である（設計 §3.5）。`launch` は構成の行の和を 1 件の上限とし、使った額と起動する実行の上限の和が承認の上限を超えれば起動しない。1 件の上限は `C-astra` で 1.516588 USD、`B-astra` で 1.928588 USD である（設計の見込みは約 1.6 と約 1.9）。

<!-- check-record: bounds -->
| 構成 | 工程 | モデル | 入力の上限 | 出力の上限 | 上限（USD） |
|---|---|---|---|---|---|
| C-sol | generation | gpt-6.1-sol | 17303 | 6080 | 0.104058 |
| C-sol | shorten | gpt-6.1-sol | 9922 | 6080 | 0.085605 |
| C-sol | review | gpt-6.1-sol | 20985 | 6080 | 0.113263 |
| C-astra | generation | gpt-6-astra | 17303 | 6080 | 0.520288 |
| C-astra | shorten | gpt-6-astra | 9922 | 6080 | 0.428025 |
| C-astra | review | gpt-6-astra | 21142 | 6080 | 0.568275 |
| A-sol | shorten | claude-opus-5-5 | 44663 | 40000 | 1.023315 |
| A-sol | review | gpt-6.1-sol | 31905 | 6080 | 0.140563 |
| A-astra | shorten | claude-opus-5-5 | 44663 | 40000 | 1.023315 |
| A-astra | review | gpt-6-astra | 32062 | 6080 | 0.704775 |
| B-sol | generation | gpt-6.1-sol | 17303 | 6080 | 0.104058 |
| B-sol | shorten | gpt-6.1-sol | 9922 | 6080 | 0.085605 |
| B-sol | review | claude-opus-5-5 | 36055 | 40000 | 0.980275 |
| B-astra | generation | gpt-6-astra | 17303 | 6080 | 0.520288 |
| B-astra | shorten | gpt-6-astra | 9922 | 6080 | 0.428025 |
| B-astra | review | claude-opus-5-5 | 36055 | 40000 | 0.980275 |

**残る危険:** 構成 A・B の検証・推敲の入力の上限は、記事の部分を本文とタイトルの UTF-8 のバイト数の上限（17,000 バイト）で数える。1 トークンが 1 バイト以上に当たることを前提にしており、この前提は確かめていない。外れた場合、1 つの構成の使った額がその上限を超えうる（設計 §3.5）。

**承認（AC-19）:** （ステップ 2-1 で、承認の時刻・示した見積もり・上限を書く）

<!-- check-record: approvals -->
| 順 | 承認（UTC） | 構成 | 上限（USD） |
|---|---|---|---|

**承認より前に始まった実行（AC-19）:**

<!-- check-record: pre-approval-runs -->
| 構成 | 記事 | 回 | 開始（UTC） |
|---|---|---|---|

**使った額（AC-20）:** 「launch」は起動の判定が数える額、「record」は人が支払い前と確かめた実行を除いた、本書の合計である。

<!-- check-record: spend -->
| 範囲 | 分かっている額 | 不明の工程 | 上限で数えた実行 | 上限の額 | 合計 | 除いた実行 | 除いた上限の額 |
|---|---|---|---|---|---|---|---|
| launch | 0.000000 | 0 | 0 | 0.000000 | 0.000000 | 0 | 0.000000 |
| record | 0.000000 | 0 | 0 | 0.000000 | 0.000000 | 0 | 0.000000 |

**支払い前と確かめて除いた実行（AC-20）:**

<!-- check-record: spend-excluded -->
| 構成 | 記事 | 回 | 上限の額 |
|---|---|---|---|

## 6. 単価

**単価（AC-15）:** `rates` が `internal/pricing` の `Estimate` から求めた、100 万トークンあたりの単価（USD）である。`le272000` は入力が 272,000 トークン以下、`gt272000` はそれを超える場合の段である。Claude のモデルは段を持たないので、2 つの行が同じになる。

<!-- check-record: rates -->
| モデル | 段 | 通常の入力 | キャッシュの読み取り | キャッシュの書き込み | 出力 |
|---|---|---|---|---|---|
| gpt-6.1-sol | le272000 | 2.000000 | 0.100000 | 2.500000 | 10.000000 |
| gpt-6.1-sol | gt272000 | 4.000000 | 0.200000 | 5.000000 | 15.000000 |
| gpt-6-astra | le272000 | 10.000000 | 1.000000 | 12.500000 | 50.000000 |
| gpt-6-astra | gt272000 | 20.000000 | 2.000000 | 25.000000 | 75.000000 |
| claude-opus-5-5 | le272000 | 4.000000 | 0.200000 | 5.000000 | 20.000000 |
| claude-opus-5-5 | gt272000 | 4.000000 | 0.200000 | 5.000000 | 20.000000 |

**料金表のコミットと claude-opus-5-5 の単価の比較（AC-15）:** （ステップ 1-7 で、評価に使うコミットの `rates` と `rates-compare.sh` の出力を書く）

## 7. 1 段目の各構成

### 7.0. 実行の順序と並列の組（AC-06）

`launch` の起動の記録である。1 行が、同時に起動した 1 つの構成の記事の組である。

<!-- check-record: launches -->
| 順 | 起動（UTC） | 構成 | 記事 |
|---|---|---|---|

### 7.1. C-sol

**設定（AC-06）:** 生成と字数の調整は `openai/gpt-6.1-sol+medium`、検証・推敲は `openai/gpt-6.1-sol+medium` で、テンプレートは既定である。全工程を実行する。CLI のコミットとバイナリの SHA-256 は §2 に記録する（ステップ 1-6）。

**実行（AC-06）:** 工程ごとの保存した出力の識別子と、実行を始めた時刻である。

<!-- check-record: runs config=C-sol -->
| 構成 | 記事 | 回 | `--after` | `YT2COLUMN_LLM` | `YT2COLUMN_REVIEW_LLM` | 開始（UTC） | 終了（UTC） | 終了コード | 生成の識別子 | 字数の調整の識別子 | 検証・推敲の識別子 |
|---|---|---|---|---|---|---|---|---|---|---|---|

**トークン数と推定コスト（AC-14）:** 工程ごとの CLI の値である。「再利用した実行」は、再開で呼ばなかった工程の値の出どころ（B3 の記事）を示す。

<!-- check-record: stage-usage config=C-sol -->
| 構成 | 記事 | 回 | 工程 | 出どころ | 使用量 | 入力 | キャッシュから読んだ入力 | キャッシュに書いた入力 | 出力 | モデル | 推定コスト（USD） | 不明の理由 | 再利用した実行 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

**記事ごとの推定コストと所要時間（AC-14・AC-16）:** 所要時間は記事を作った工程の時間の和で、10 件を並列に起動して測った値である（基準の構成と同じ。0018 評価記録 §4.8）。

<!-- check-record: run-costs config=C-sol -->
| 構成 | 記事 | 回 | 終了コード | この実行（USD） | 記事（USD） | 実行の時間 | 所要時間 | 比較に使う |
|---|---|---|---|---|---|---|---|---|

**失敗した実行とやり直し（AC-17）:** 番兵は主セッションが標準エラーを読んで下に記録し、人が確認する。

<!-- check-record: failed-runs config=C-sol -->
| 構成 | 記事 | 回 | 終了コード | 失敗した工程 | 入力 | キャッシュから読んだ入力 | キャッシュに書いた入力 | 出力 | 推定コスト（USD） | 支払い前の候補 | 支払い前と確認 |
|---|---|---|---|---|---|---|---|---|---|---|---|

<!-- check-record: retries config=C-sol -->
| 構成 | 記事 | やり直しの回数 | 記事を得た | 2 回を超えた |
|---|---|---|---|---|

**判定（AC-07・AC-09）:** 各セルは「解釈 / 文言」の順の判定である。

<!-- check-record: judgments config=C-sol -->
| 構成 | 記事 | 形式の選択 | 形式の規則 | 根拠 | 長さ | 表記 | 網羅 | 重要度 | 構成（観点） | 読みやすさ | タイトル・リード文 | 冗長さ | 目標（解釈） | 目標（文言） |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

**判定者（AC-10）:** `script` はスクリプト、`llm` は LLM、`llm+human` は人が確認して変えなかった LLM の判定、`human` は人が変えた判定である。人が変えた判定と理由は下に書く。

<!-- check-record: judges config=C-sol -->
| 構成 | 記事 | 形式の選択 | 形式の規則 | 根拠 | 長さ | 表記 | 網羅 | 重要度 | 構成（観点） | 読みやすさ | タイトル・リード文 | 冗長さ | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

**機械的な値（AC-09）:** `check_article.py` の出力である。

<!-- check-record: article-checks config=C-sol -->
| 構成 | 記事 | タイトルの字数 | 節の数 | 長さ |
|---|---|---|---|---|

**引用が要る判定（AC-12）:** 引用、誤りを入れた工程、直さなかった工程は下に書く。

<!-- check-record: quotes-needed config=C-sol -->
| 構成 | 記事 | 観点 | 判定 | 種類 |
|---|---|---|---|---|

**製品の制約 1〜3（AC-13）:** 引用は下に書く。

<!-- check-record: constraints config=C-sol -->
| 構成 | 記事 | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|

### 7.2. C-astra

**設定（AC-06）:** 生成と字数の調整は `openai/gpt-6-astra+medium`、検証・推敲は `openai/gpt-6-astra+medium` で、テンプレートは既定である。全工程を実行する。CLI のコミットとバイナリの SHA-256 は §2 に記録する（ステップ 1-6）。

**実行（AC-06）:** 工程ごとの保存した出力の識別子と、実行を始めた時刻である。

<!-- check-record: runs config=C-astra -->
| 構成 | 記事 | 回 | `--after` | `YT2COLUMN_LLM` | `YT2COLUMN_REVIEW_LLM` | 開始（UTC） | 終了（UTC） | 終了コード | 生成の識別子 | 字数の調整の識別子 | 検証・推敲の識別子 |
|---|---|---|---|---|---|---|---|---|---|---|---|

**トークン数と推定コスト（AC-14）:** 工程ごとの CLI の値である。「再利用した実行」は、再開で呼ばなかった工程の値の出どころ（B3 の記事）を示す。

<!-- check-record: stage-usage config=C-astra -->
| 構成 | 記事 | 回 | 工程 | 出どころ | 使用量 | 入力 | キャッシュから読んだ入力 | キャッシュに書いた入力 | 出力 | モデル | 推定コスト（USD） | 不明の理由 | 再利用した実行 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

**記事ごとの推定コストと所要時間（AC-14・AC-16）:** 所要時間は記事を作った工程の時間の和で、10 件を並列に起動して測った値である（基準の構成と同じ。0018 評価記録 §4.8）。

<!-- check-record: run-costs config=C-astra -->
| 構成 | 記事 | 回 | 終了コード | この実行（USD） | 記事（USD） | 実行の時間 | 所要時間 | 比較に使う |
|---|---|---|---|---|---|---|---|---|

**失敗した実行とやり直し（AC-17）:** 番兵は主セッションが標準エラーを読んで下に記録し、人が確認する。

<!-- check-record: failed-runs config=C-astra -->
| 構成 | 記事 | 回 | 終了コード | 失敗した工程 | 入力 | キャッシュから読んだ入力 | キャッシュに書いた入力 | 出力 | 推定コスト（USD） | 支払い前の候補 | 支払い前と確認 |
|---|---|---|---|---|---|---|---|---|---|---|---|

<!-- check-record: retries config=C-astra -->
| 構成 | 記事 | やり直しの回数 | 記事を得た | 2 回を超えた |
|---|---|---|---|---|

**判定（AC-07・AC-09）:** 各セルは「解釈 / 文言」の順の判定である。

<!-- check-record: judgments config=C-astra -->
| 構成 | 記事 | 形式の選択 | 形式の規則 | 根拠 | 長さ | 表記 | 網羅 | 重要度 | 構成（観点） | 読みやすさ | タイトル・リード文 | 冗長さ | 目標（解釈） | 目標（文言） |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

**判定者（AC-10）:** `script` はスクリプト、`llm` は LLM、`llm+human` は人が確認して変えなかった LLM の判定、`human` は人が変えた判定である。人が変えた判定と理由は下に書く。

<!-- check-record: judges config=C-astra -->
| 構成 | 記事 | 形式の選択 | 形式の規則 | 根拠 | 長さ | 表記 | 網羅 | 重要度 | 構成（観点） | 読みやすさ | タイトル・リード文 | 冗長さ | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

**機械的な値（AC-09）:** `check_article.py` の出力である。

<!-- check-record: article-checks config=C-astra -->
| 構成 | 記事 | タイトルの字数 | 節の数 | 長さ |
|---|---|---|---|---|

**引用が要る判定（AC-12）:** 引用、誤りを入れた工程、直さなかった工程は下に書く。

<!-- check-record: quotes-needed config=C-astra -->
| 構成 | 記事 | 観点 | 判定 | 種類 |
|---|---|---|---|---|

**製品の制約 1〜3（AC-13）:** 引用は下に書く。

<!-- check-record: constraints config=C-astra -->
| 構成 | 記事 | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|

### 7.3. A-sol

**設定（AC-06）:** 生成と字数の調整は `claude/claude-opus-5-5+high`、検証・推敲は `openai/gpt-6.1-sol+medium` で、テンプレートは既定である。基準の構成の保存した出力から、検証・推敲だけを再実行する（設計 §3.2）。roNZ-rLxhUE-1 は `--after shorten`、残りの 9 件は `--after generation` で再開する。CLI のコミットとバイナリの SHA-256 は §2 に記録する（ステップ 1-6）。

**実行（AC-06）:** 工程ごとの保存した出力の識別子と、実行を始めた時刻である。

<!-- check-record: runs config=A-sol -->
| 構成 | 記事 | 回 | `--after` | `YT2COLUMN_LLM` | `YT2COLUMN_REVIEW_LLM` | 開始（UTC） | 終了（UTC） | 終了コード | 生成の識別子 | 字数の調整の識別子 | 検証・推敲の識別子 |
|---|---|---|---|---|---|---|---|---|---|---|---|

**再開と識別子（AC-03）:** 再利用した保存した出力の識別子が基準の構成と一致すること、字数の調整を実行していないことである。

<!-- check-record: resume-identity config=A-sol -->
| 構成 | 記事 | `--after` | 生成の一致 | 字数の調整の一致 | 字数の調整を実行 | 手順から外れた |
|---|---|---|---|---|---|---|

**トークン数と推定コスト（AC-14）:** 工程ごとの CLI の値である。「再利用した実行」は、再開で呼ばなかった工程の値の出どころ（B3 の記事）を示す。

<!-- check-record: stage-usage config=A-sol -->
| 構成 | 記事 | 回 | 工程 | 出どころ | 使用量 | 入力 | キャッシュから読んだ入力 | キャッシュに書いた入力 | 出力 | モデル | 推定コスト（USD） | 不明の理由 | 再利用した実行 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

**記事ごとの推定コストと所要時間（AC-14・AC-16）:** 所要時間は記事を作った工程の時間の和で、10 件を並列に起動して測った値である（基準の構成と同じ。0018 評価記録 §4.8）。構成 A は B3 の 1 段目の生成の時間を足し、roNZ-rLxhUE-1 には B3 のやり直しの字数の調整の時間も足す。

<!-- check-record: run-costs config=A-sol -->
| 構成 | 記事 | 回 | 終了コード | この実行（USD） | 記事（USD） | 実行の時間 | 所要時間 | 比較に使う |
|---|---|---|---|---|---|---|---|---|

**失敗した実行とやり直し（AC-17）:** 番兵は主セッションが標準エラーを読んで下に記録し、人が確認する。

<!-- check-record: failed-runs config=A-sol -->
| 構成 | 記事 | 回 | 終了コード | 失敗した工程 | 入力 | キャッシュから読んだ入力 | キャッシュに書いた入力 | 出力 | 推定コスト（USD） | 支払い前の候補 | 支払い前と確認 |
|---|---|---|---|---|---|---|---|---|---|---|---|

<!-- check-record: retries config=A-sol -->
| 構成 | 記事 | やり直しの回数 | 記事を得た | 2 回を超えた |
|---|---|---|---|---|

**判定（AC-07・AC-09）:** 各セルは「解釈 / 文言」の順の判定である。

<!-- check-record: judgments config=A-sol -->
| 構成 | 記事 | 形式の選択 | 形式の規則 | 根拠 | 長さ | 表記 | 網羅 | 重要度 | 構成（観点） | 読みやすさ | タイトル・リード文 | 冗長さ | 目標（解釈） | 目標（文言） |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

**判定者（AC-10）:** `script` はスクリプト、`llm` は LLM、`llm+human` は人が確認して変えなかった LLM の判定、`human` は人が変えた判定である。人が変えた判定と理由は下に書く。

<!-- check-record: judges config=A-sol -->
| 構成 | 記事 | 形式の選択 | 形式の規則 | 根拠 | 長さ | 表記 | 網羅 | 重要度 | 構成（観点） | 読みやすさ | タイトル・リード文 | 冗長さ | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

**機械的な値（AC-09）:** `check_article.py` の出力である。

<!-- check-record: article-checks config=A-sol -->
| 構成 | 記事 | タイトルの字数 | 節の数 | 長さ |
|---|---|---|---|---|

**引用が要る判定（AC-12）:** 引用、誤りを入れた工程、直さなかった工程は下に書く。

<!-- check-record: quotes-needed config=A-sol -->
| 構成 | 記事 | 観点 | 判定 | 種類 |
|---|---|---|---|---|

**製品の制約 1〜3（AC-13）:** 引用は下に書く。

<!-- check-record: constraints config=A-sol -->
| 構成 | 記事 | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|

### 7.4. A-astra

**設定（AC-06）:** 生成と字数の調整は `claude/claude-opus-5-5+high`、検証・推敲は `openai/gpt-6-astra+medium` で、テンプレートは既定である。基準の構成の保存した出力から、検証・推敲だけを再実行する（設計 §3.2）。roNZ-rLxhUE-1 は `--after shorten`、残りの 9 件は `--after generation` で再開する。CLI のコミットとバイナリの SHA-256 は §2 に記録する（ステップ 1-6）。

**実行（AC-06）:** 工程ごとの保存した出力の識別子と、実行を始めた時刻である。

<!-- check-record: runs config=A-astra -->
| 構成 | 記事 | 回 | `--after` | `YT2COLUMN_LLM` | `YT2COLUMN_REVIEW_LLM` | 開始（UTC） | 終了（UTC） | 終了コード | 生成の識別子 | 字数の調整の識別子 | 検証・推敲の識別子 |
|---|---|---|---|---|---|---|---|---|---|---|---|

**再開と識別子（AC-03）:** 再利用した保存した出力の識別子が基準の構成と一致すること、字数の調整を実行していないことである。

<!-- check-record: resume-identity config=A-astra -->
| 構成 | 記事 | `--after` | 生成の一致 | 字数の調整の一致 | 字数の調整を実行 | 手順から外れた |
|---|---|---|---|---|---|---|

**トークン数と推定コスト（AC-14）:** 工程ごとの CLI の値である。「再利用した実行」は、再開で呼ばなかった工程の値の出どころ（B3 の記事）を示す。

<!-- check-record: stage-usage config=A-astra -->
| 構成 | 記事 | 回 | 工程 | 出どころ | 使用量 | 入力 | キャッシュから読んだ入力 | キャッシュに書いた入力 | 出力 | モデル | 推定コスト（USD） | 不明の理由 | 再利用した実行 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

**記事ごとの推定コストと所要時間（AC-14・AC-16）:** 所要時間は記事を作った工程の時間の和で、10 件を並列に起動して測った値である（基準の構成と同じ。0018 評価記録 §4.8）。構成 A は B3 の 1 段目の生成の時間を足し、roNZ-rLxhUE-1 には B3 のやり直しの字数の調整の時間も足す。

<!-- check-record: run-costs config=A-astra -->
| 構成 | 記事 | 回 | 終了コード | この実行（USD） | 記事（USD） | 実行の時間 | 所要時間 | 比較に使う |
|---|---|---|---|---|---|---|---|---|

**失敗した実行とやり直し（AC-17）:** 番兵は主セッションが標準エラーを読んで下に記録し、人が確認する。

<!-- check-record: failed-runs config=A-astra -->
| 構成 | 記事 | 回 | 終了コード | 失敗した工程 | 入力 | キャッシュから読んだ入力 | キャッシュに書いた入力 | 出力 | 推定コスト（USD） | 支払い前の候補 | 支払い前と確認 |
|---|---|---|---|---|---|---|---|---|---|---|---|

<!-- check-record: retries config=A-astra -->
| 構成 | 記事 | やり直しの回数 | 記事を得た | 2 回を超えた |
|---|---|---|---|---|

**判定（AC-07・AC-09）:** 各セルは「解釈 / 文言」の順の判定である。

<!-- check-record: judgments config=A-astra -->
| 構成 | 記事 | 形式の選択 | 形式の規則 | 根拠 | 長さ | 表記 | 網羅 | 重要度 | 構成（観点） | 読みやすさ | タイトル・リード文 | 冗長さ | 目標（解釈） | 目標（文言） |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

**判定者（AC-10）:** `script` はスクリプト、`llm` は LLM、`llm+human` は人が確認して変えなかった LLM の判定、`human` は人が変えた判定である。人が変えた判定と理由は下に書く。

<!-- check-record: judges config=A-astra -->
| 構成 | 記事 | 形式の選択 | 形式の規則 | 根拠 | 長さ | 表記 | 網羅 | 重要度 | 構成（観点） | 読みやすさ | タイトル・リード文 | 冗長さ | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

**機械的な値（AC-09）:** `check_article.py` の出力である。

<!-- check-record: article-checks config=A-astra -->
| 構成 | 記事 | タイトルの字数 | 節の数 | 長さ |
|---|---|---|---|---|

**引用が要る判定（AC-12）:** 引用、誤りを入れた工程、直さなかった工程は下に書く。

<!-- check-record: quotes-needed config=A-astra -->
| 構成 | 記事 | 観点 | 判定 | 種類 |
|---|---|---|---|---|

**製品の制約 1〜3（AC-13）:** 引用は下に書く。

<!-- check-record: constraints config=A-astra -->
| 構成 | 記事 | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|

## 8. 2 段目

**B のモデルの選択（AC-08）:** C-astra と C-sol の合格件数（解釈による判定）で選ぶ。同数なら両方で作る（要件 F-002）。

<!-- check-record: b-selection -->
| 行 | C-astra の合格 | C-sol の合格 | B の構成 |
|---|---|---|---|

（ステップ 4-1 で、作った B の構成ごとに節 7 と同じ形の小節を加える）

## 9. 比較

**比較の表（AC-18）:** 平均と最大は比較に使う記事（成功し、手順から外れていない実行）だけで求める。品質の観点の分布は解釈による判定の 60 の判定である。

<!-- check-record: comparison -->
| 構成 | 比較に使う | 得られなかった | 合格（解釈） | 合格（文言） | 品質 1 | 品質 2 | 品質 3 | 推定コストの平均（USD） | 推定コストの最大 | 最大の記事 | 所要時間の平均 | 所要時間の最大 | 最大の記事 | 失敗した実行 | 失敗の推定コスト | 推定コストが不明の失敗 | キャッシュを読んだ工程 | 補助の推定コストの平均 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

## 10. 結論

**F-006 の条件ごとの判定（AC-21）:**

<!-- check-record: f006 -->
| 構成 | 合格（解釈） | 基準との差 | (i) | 合格が基準以上 | 動画ごとの比較 | 推定コストの平均（USD） | 推定コストが低い | (ii) | 所要時間の平均 | (iii) | 候補 | 補助の値で変わる |
|---|---|---|---|---|---|---|---|---|---|---|---|---|

**動画ごとの必須の観点の不合格（AC-21）:**

<!-- check-record: f006-video -->
| 構成 | 動画 | 不合格の記事 | 基準の構成 | 基準以下 |
|---|---|---|---|---|

**選んだ構成（AC-22）:**

<!-- check-record: f006-selection -->
| 除いた候補 | 選んだ構成 |
|---|---|
| - | none |

**製品の制約 1〜3 が直ったか（AC-23）:** 「yes」は、その構成の 10 件のどの記事にもその種類の誤りがないことを示す。

<!-- check-record: constraints-fixed -->
| 構成 | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|
| B3 | no | no | no |

（ステップ 5-1 で、人の承認と除いた候補を書く）
