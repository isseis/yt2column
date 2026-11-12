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

**結論の要約:** 本番の構成は変えない（要件 F-006 の候補がない。2026-11-12 に人が承認した）。
- 1 段目の 4 構成（C-sol・C-astra・A-sol・A-astra）と 2 段目の B-sol の 50 件は、すべて記事を得た。合格件数（解釈による判定）は 5〜7 件で、どれも基準の構成（B3）の 8 件に届かなかった。
- 費用は C-sol（0.067 USD）と B-sol（0.140 USD）が基準の構成（0.362 USD）より大きく低い。ただし roNZ-rLxhUE でインタビュー記事を選ばなかった記事が C-sol に 1 件、B-sol に 2 件あり、必須の観点の不合格が基準の構成より多い動画がある。
- 構成 A（Claude の生成と OpenAI の検証・推敲）は、合格が 7 件で基準の構成より 1 件少ない。動画ごとの比較は基準の構成以下に収まった。
- 製品の制約 1〜3 は、どの構成でも直らなかった。
- 使った額は CLI の推定で合計 7.733370 USD（承認の上限 30 USD）、失敗した実行はない。

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
- **中断した実行:** `run.sh` は INT・TERM・HUP を受けると CLI に SIGTERM を送り、CLI が使用量と料金の行を書いて終わるのを待ってから、実行の記録に `end=` と `exit=` を書く。`run.sh` 自体が `kill -9` などで止まって記録に `exit=` がない場合は、CLI のプロセスが残っていないこと（`pgrep -f`）を確かめてから `retire.sh --unfinished` で移す。記録に `retired_unfinished=<時刻>` が加わり、`launch` は `failed/` にあるこの実行を終わったものとして扱う（料金の合計の行がないので、使った額には上限の額で数える）。30 分の間隔は、この時刻から数える。
- **1 段目の人の確認の印:** 時刻は、1 段目の構成のすべての実行の終了（`retired_unfinished` の実行はその時刻）以後で、起動の時刻以前でなければならない。1 段目の構成のどの記事も、記事を得たか、最初の実行と 2 回のやり直しがすべて失敗した後でなければならない。確認の後に 1 段目でやり直しをした場合は、確認し直して印を書き直す。
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
# 実行の前の確認（API キーを外し、0087/preflight/ に出力する。--after-review は 0087/preflight/after-review/ に出力する）
$X/run.sh run <構成 ID> <記事> $E/0087/bin/<コミット>/yt2column --preflight [--after-review]
# 1 つの構成の 10 件（またはやり直しの 1 件）の起動。API を呼ぶ実行は必ずこれで起動する
direnv exec . go run ./$X/evaltool launch -root $E/0087 -runsh $X/run.sh \
  -cli $E/0087/bin/<コミット>/yt2column -cli-sha256 <固定したときに記録した SHA-256> \
  -bounds-input $X/bounds-input.json -bounds $X/bounds.tsv \
  -config <構成 ID> [-retry <記事>] -now "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
# やり直しの前の移動
$X/retire.sh [--unfinished] $E/0087 <構成 ID> <記事>
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
- **人の確認の範囲:** 設計 §3.6 の手順 4 は対象の判定をすべて人が確認するとした。人の求めにより、論点ごと・1 件ずつ・一括の 3 段で確認した（要件の実装の際に決めたこと 1、設計 §1.5。セッション中に再承認を受けた）。段ごとの件数は §7 の冒頭にある。
- **1 段目の判定の見直し（`B-sol` の起動の後）:** 設計 §3.6 は、1 段目の人の確認を終えてから 2 段目を実行するとした。1 段目の人の確認の印（2026-11-12T08:04:55Z）の後、`B-sol` を起動した（08:07:25Z）。その後、2 段目の確認（08:33Z まで）で、人が 1 段目の 5 判定を見直した。A-sol・A-astra の zW1RBacSqjk-1 と C-astra の zW1RBacSqjk-2 の制約 1（「あり」から「なし」）と、A-sol・A-astra の zW1RBacSqjk-1 の「根拠」の文言どおりの判定（いったん人が不合格にしたものを合格に戻した）である。B の構成の選択（AC-08）が使うのは、C-astra と C-sol の解釈による判定の合格件数で、この見直しはそれを変えない（見直しの前も後も C-sol 6・C-astra 5）。そのため、B の記事が C の判定の人の確認を終えた後に作られたこと（AC-08）は、選択に使う値について保たれる。人の確認の印は、起動の前の確認の時刻を示すものとして書き直していない。
- **素材の検査:** `material` は、素材のどのファイルにもその段の構成 ID とモデル ID が含まれないことを確かめ、含まれれば止まる。

### 2.5. 道具の確かめ（ステップ 1-5）

PR-1 のブランチのコミット `7107df2dacee`（squash merge の前のコミット）で実行した。どれも API を呼ばない。この後に `secretscan.sh`・`selftest.sh` に `.envrc` のテスト用のキーの変数を加え、この 2 つは加えた後のコミットで実行し直した。ほかに変えたのは本節だけである。

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
selftest: ok   secretscan: finds a test key value
selftest: ok   secretscan: fails with no key variable
selftest: ok   secretscan: fails on a missing path
selftest: ok   secretscan: fails when grep cannot read a file
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
repo_impact: base 188ed2cac1b0d9414cf17da35978302ee0d37312, head 7107df2daceed1ee4b980e916e8c5a20c1d2e8ec
exit=0
```

**`rates` と `rates-compare.sh`（AC-15）:** `rates` の `claude-opus-5-5` の単価（入力 4、キャッシュの読み取り 0.2、キャッシュの書き込み 5、出力 20 USD/MTok）は、`986449c6` の料金表（`internal/pricing/rates.go:114`）の値と一致した。`rates` の出力は §6 の表である。B3 の CLI のコミット `2fc2e2a` との比較は次のとおりで、同じだった。評価に使うコミットでの比較はステップ 1-7 で行う。

```text
evaluation commit 7107df2dacee: base:rates{input:4000000,cacheRead:200000,cacheWrite:5000000,output:20000000},
B3 commit 2fc2e2a: base:rates{input:4000000,cacheRead:200000,cacheWrite:5000000,output:20000000},
rates-compare: same
exit=0
```

**`secretscan.sh`:** リポジトリのルートで `direnv exec . docs/tasks/0087_openai_combination_eval/_eval/secretscan.sh` として、今の `.envrc` を読み込んで実行した。本番の名前のキー（`OPENAI_API_KEY` を含む）と、`.envrc` が定義するテスト用のキー（`YT2COLUMN_TEST_*`）の値は、`_eval/` と本書のどちらにもなく、`sk-` の接頭辞もなかった。PR の作成の後に `_eval/` か本書を変えた場合は、マージの前に実行し直す。

```text
secretscan: the value of ANTHROPIC_API_KEY is absent
secretscan: the value of OPENAI_API_KEY is absent
secretscan: the value of DEEPSEEK_API_KEY is absent
secretscan: the value of SLACK_WEBHOOK_URL is absent
secretscan: the value of YT2COLUMN_TEST_ANTHROPIC_API_KEY is absent
secretscan: the value of YT2COLUMN_TEST_OPENAI_API_KEY is absent
secretscan: the value of YT2COLUMN_TEST_DEEPSEEK_API_KEY is absent
secretscan: the value of YT2COLUMN_TEST_SLACK_WEBHOOK_URL is absent
secretscan: no sk- key prefix
exit=0
```

**`check-record`（骨組み）:** 同じコミットで、0087 の実行がない状態の `aggregate`（`-root` に存在しないディレクトリを渡した）・`estimate`・`rates` の出力をファイルに書き、本書と照合した。本書自身の SHA-256 は、本節を書く前の版のものなので載せない。評価の記録の照合はステップ 5-1 で行う。

```text
sha256 bdac64f70a433d0252afac182d6bbd1b2a4fe12566efe554ea639de77170c0ca <作業ディレクトリ>/agg.txt
sha256 c55fce120901d78757c0c0dbef9a67b6457782259a61703b969c60ad07083e13 <作業ディレクトリ>/est.txt
sha256 4441024ebddce789f8b0ccc23512bf78a776b069fa9b625dc81a67f221c9a097 <作業ディレクトリ>/rates.txt
check-record: 28 tables match
```

### 2.6. CLI のビルドの固定（ステップ 1-6）

2026-11-12 に、未コミットの変更のない作業ツリー（`git status --porcelain` が空）で `make build` を 1 回行い、`build/yt2column` を `0087/bin/<コミット>/yt2column` に写した（設計 §3.1）。すべての構成をこのバイナリで実行する。

| 項目 | 値 |
|---|---|
| ビルドしたコミット | `bc7208cd941fabf3635e10067fa4c58c0dbe9441`（ビルドの時点の main の先頭。`git merge-base --is-ancestor` で main から到達できることを確かめた） |
| 置き場所 | `0087/bin/bc7208cd941fabf3635e10067fa4c58c0dbe9441/yt2column`（モード `0555`） |
| SHA-256 | `6a975731b6aa34be0421dc891f9e365d3957b2d7cb1cffa047420b39f2179a09`（同じディレクトリの `yt2column.sha256` にも置いた） |
| バイナリが持つビルドの情報 | `go version -m` で `vcs.revision=bc7208cd941fabf3635e10067fa4c58c0dbe9441`・`vcs.modified=false` |

**採点のエージェントの定義:** 2026-11-12 に、人の許可を得て `_eval/scorer-agent.md` を `~/.claude/agents/yt2column-0087-scorer.md` に写した（内容は同じ。計画 §1.2 の決めたこと 9）。2 段目の採点の後に消す（ステップ 4-1）。ディレクトリを新しく作ったので、ステップ 1-7 の前にセッションを開き直す。定義が読み込まれたことは、ステップ 1-7 の試しの採点で確かめる。

**`evaltool` を実行するコミット:** 固定したコミット `bc7208cd` の `_eval/evaltool` はビルドできない。PR-1 のマージの前に、#843 が `YtDlpSource` を `internal/transcript` から `internal/ytdlpsource` に移したためである（`_eval/` は `go list ./...` に入らないので、`make test` も CI もこれを検出しなかった。§2.5 の確かめは squash merge の前のコミット `7107df2dacee` のものである）。PR-2 のブランチのコミット `18497a55` で直した。ステップ 1-7 以後の `evaltool`（`estimate`・`rates`・`launch`・`aggregate`・`material`・`check-record`）は PR-2 のブランチで実行する。単価と上限が固定した CLI と食い違わないように、次を守る。
- 実行の前に、`git diff --quiet bc7208cd HEAD -- internal cmd prompts go.mod go.sum Makefile` が成功する（CLI に関わるツリーが固定したコミットと同じ）ことを確かめる。
- PR-2 のブランチには、ステップ 5-1 を終えるまで main を取り込まない。

このコミットのアダプタの出力の上限の既定は、OpenAI が 6,080（`internal/llm/openai/request.go:29`）、Claude が 40,000（`internal/llm/claude/request.go:30`）で、`bounds-input.json` の値と同じである（§5「上限のそのほかの前提」）。

### 2.7. 実行の前の確認（ステップ 1-7）

2026-11-12 に、固定した CLI（§2.6）で設計 §3.4 の項目 1〜3 を行った。項目 4 は §3 に記録した。CLI の起動はすべて `run.sh` の preflight の使い方で、`env -u ANTHROPIC_API_KEY -u OPENAI_API_KEY -u DEEPSEEK_API_KEY` を付けて行った。`run.sh` は preflight の起動でもこの 3 つのキーと `ANTHROPIC_WORKSPACE_ID` を外す。どの起動の標準エラーにも `origin` が `run` の `tokens:` の行はなく、CLI は API を一度も呼んでいない。

**`run.sh` の変更:** 項目 1 の `--after-review` の起動と項目 2 の起動は、同じ構成 ID と記事で同じ出力先（`0087/preflight/<種類>/<構成 ID>/<記事>`）を使うので、後から起動したほうが「already exists」で止まった。`--after-review` の起動の出力先を `0087/preflight/after-review/` に分けた（コミット `b435f279`、計画 §1.4 の決めたこと 2）。API を呼ぶ実行の出力先は変えていない。`selftest.sh` はこの分岐を確かめないので、分岐は下の実際の起動だけで確かめた。項目 1・2 と下の確かめは、この変更の後に `0087/preflight/` を空にして（`trial/` を除く）やり直したものである。

**項目 1（保存した出力を今の CLI が読めること）:** 10 件を構成 `C-sol` の名前で `--after review` で再開した（構成の LLM の値は、この再開では使われない）。10 件とも終了コード `0` で記事を書いた。どの記事も、`cost:` の行はすべて `reused known`（roNZ-rLxhUE-1 は生成・字数の調整・検証・推敲の 3 行、ほかは生成と検証・推敲の 2 行）で、`cost-total: run` は `0.000000`、`cost-total: article` の 10 件の和は 3.615544 USD で、B3 の合計（§2.5 の `reproduce0018` の値）と一致した。また、10 件の `base/B3/` の生成の保存した出力は、`work/B3/` の同じファイルと `cmp` でバイト単位で一致した。

**項目 2（構成ごとの設定と実行の手順）:** 6 構成 × 10 件の 60 回を起動し、60 回とも終了コード `2` で終わった。標準エラーが示した足りない変数は次のとおりで、設計 §3.1 の表の必要な API キーと一致する。どの起動も `is unset or empty` の行だけを出し、変数の値の誤りはなかった。構成 A の roNZ-rLxhUE-1（`--after shorten`）だけが `ANTHROPIC_API_KEY` を要さないのも表のとおりである。

| 構成 | `after` | 起動 | 足りない変数 |
|---|---|---|---|
| C-sol・C-astra | none | 各 10 | `OPENAI_API_KEY` |
| A-sol・A-astra | generation | 各 9 | `ANTHROPIC_API_KEY`・`OPENAI_API_KEY` |
| A-sol・A-astra | shorten（roNZ-rLxhUE-1） | 各 1 | `OPENAI_API_KEY` |
| B-sol・B-astra | none | 各 10 | `ANTHROPIC_API_KEY`・`OPENAI_API_KEY` |

- **`.when` の値:** 60 の記録の `llm`・`review_llm` は構成ごとに 1 組で、`run.sh configs` の表と同じだった。`ytdlp_path` はすべて `/usr/bin/false`、`keep_cache` はすべて `yes`、`binary_sha256` はすべて §2.6 の値である。LLM の変数の値を設計 §3.1 の表と照合した結果は、下の「`.when` と設計の表の照合」に書く。
- **写し:** `run.sh` は写しのたびに SHA-256 を最初の一覧と比べ、違えば CLI を起動せずに止まる。60 回とも CLI まで進んだので、すべての写しが一覧と一致した。写し先のディレクトリはモード `0700`、ファイルは `0600` だった。項目 1 の起動は写し先に検証・推敲の保存した出力を書いたので、写し先に書き込める。
- **出力先:** 項目 2 の起動では `out/` と `review/` に何も書かれていない（CLI は記事の生成の前に止まる）。出力先がすでにある記事（`C-sol` の roNZ-rLxhUE-1）を起動し直すと、`run.sh` は `already exists` で終了コード `1` で止まり、ファイルを 1 つも増やさなかった。

**`.when` と設計の表の照合（項目 2 の 60 の記録）:** 計画は、`.when` の LLM の変数の値を人が設計 §3.1 の表と照合するとしていた。人が、機械的に確かめられるので機械で照合するよう求めた（2026-11-12）。設計書の §3.1 の表の 6 行から構成 ID・`YT2COLUMN_LLM`・`YT2COLUMN_REVIEW_LLM` を `awk` で取り出し、60 の `.when` の `config`・`llm`・`review_llm` の組の一覧（重複を除く）と `diff` した。差はなく、どの構成も 10 の記録を持っていた。

```sh
awk '/^### 3\.1\./{f=1;next} /^### /{f=0} f && /^\| `[ABC]-/' docs/tasks/0087_openai_combination_eval/02_architecture.md |
  awk -F'|' '{for(i=2;i<=4;i++){gsub(/[ `]/,"",$i)}; print $2"\t"$3"\t"$4}' | sort > design.tsv
for f in $E/0087/preflight/log/*/*.when; do
  awk -F= '{v[$1]=$2} END{print v["config"]"\t"v["llm"]"\t"v["review_llm"]}' $f
done | sort | uniq -c | awk '{print $2"\t"$3"\t"$4"\t"$1}' > when.tsv
cut -f1-3 when.tsv | diff design.tsv - && print "when-vs-design: same"
awk -F'\t' '$4 != 10 {bad=1} END{exit bad}' when.tsv && print "each config: 10 records"
```

```text
design rows:        6
when files:       60
when-vs-design: same
each config: 10 records
```

**見積もりの再実行:** PR-2 のブランチのコミット `b435f279` で `estimate` を実行し直した。`bounds.tsv` と出力の見積もりの表は変わらなかったので、§5 は書き換えていない。

**`check_article.py`:** 項目 1 の 10 件の記事に当て、10 件とも読めて `pass` だった。出力の行（ファイル名・モデル・タイトル・節の数・字数・判定）は、B3 の記事（`0018/out/B3/`）に当てた出力と同じだった。

**採点のエージェントの試し:** 項目 1 の roNZ-rLxhUE-2 の記事を 1 件だけ、記事・保存した出力・キャッシュの写しとともにリポジトリの外の別のディレクトリ（`0087/preflight/trial/`）に写して `material` で伏せ、`yt2column-0087-scorer` のエージェントで採点させた。採点の結果は評価に使わない。`--after review` の再開は検証・推敲の記録を書かないので、`material` が読む検証・推敲の記録には、B3 の同じ記事の記録（`0018/review/B3/roNZ-rLxhUE-2.md`）を写した。試しは最初の `run.sh` の変更の後の起動の記事で行い、やり直した後の記事（`preflight/after-review/`）と `cmp` で同じであることを確かめた。
- 出力の表は `scoring-prompt.md` の見出しと 14 の観点の行の形で返った。
- エージェントが報告したモデル ID は `claude-opus-5-5`、使える道具は `Read` だけだった（ほかに、結果を返すためのハーネスの道具がある）。シェルのコマンドの実行と素材のディレクトリへの書き込みを試させたところ、どちらもできなかった。
- **わかったこと:** エージェントはディレクトリの一覧を取れないので、記事の名前を推測して読んだ（`-a` を見つけ、`-b`・`-c` がないことを確かめて止まった）。1 段目の採点では 1 本の動画に 8 件の記事がある。ステップ 3-2 で起動するときは、主セッションが素材のディレクトリのファイルの名前の一覧をプロンプトに書く（計画 §1.4 の決めたこと 3）。

**道具の確かめの再実行:** ステップ 1-5 の後に `_eval/run.sh` を変えたので、コミット `b435f279` で次を実行し直した。`selftest.sh` は `direnv exec .` で実行し、§2.5 と同じ 17 の確かめがすべて通った（最後の行は `selftest: all checks passed`、`exit=0`）。

```text
table reproduce0018 keys=1
config	cost_sum_usd	cost_avg_usd	cost_max_usd	cost_max_article	duration_avg	duration_max	duration_max_article
B3	3.615544	0.361554	0.610696	roNZ-rLxhUE-1	1:53.4	3:06.2	roNZ-rLxhUE-1
C03	3.572464	0.357246	0.602652	roNZ-rLxhUE-1	1:52.1	3:05.2	roNZ-rLxhUE-1
end
reproduce0018: matches the 0018 evaluation record
exit=0
```

```text
repo_impact: same list (42 lines)
repo_impact: same tests (1256 lines)
repo_impact: same lint (6 lines)
repo_impact: base bc7208cd941fabf3635e10067fa4c58c0dbe9441, head b435f2795544f29aeb9e15ebe9fcbb247d14e2ae
exit=0
```

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

**実行の条件の違い（AC-01）:** 基準の構成と本評価では、CLI のコミットと Claude のアダプタの出力の上限の既定が違う。
- **CLI のコミット:** B3 は `2fc2e2a` のツリーの CLI（`16680de` でビルド。main から到達できない。0018 評価記録 §4.1.2）、本評価は `bc7208cd`（§2.6）である。
- **Claude のアダプタの出力の上限の既定（`max_tokens`）:** B3 の CLI は 16,000（`2fc2e2a` の `internal/llm/claude/request.go:26`）、本評価の CLI は 40,000（`bc7208cd` の同じファイルの 30 行）である。本評価で Claude が呼ばれるのは、構成 A の字数の調整（走らない見込み。設計 §3.2）と構成 B の検証・推敲である。B3 の Claude の呼び出しの出力は、生成が 10 件で最大 13,688、字数の調整が 7,371（成功したやり直し）と 7,837（失敗した 1 回目）、検証・推敲が 1,087〜4,777 トークンで（0018 評価記録 §4.7.2）、どれも 16,000 に届いていない。B3 の出力のうち、上限で切られたものはない。

**動画ごとの必須の観点の不合格の数（AC-21）:** 0008 評価記録 §6.2 の表から、物差し 2 の解釈による判定で数えた。leE02uqgaKg が 1 件（-2 の「根拠」）、残りの 4 本が 0 件である。18tbwwPM91A-2 の不合格は品質の観点「構成」の 1 による。EqnGQDU71-Y-2 の「根拠」は文言どおりの判定だけの不合格である。値は §10 の動画ごとの比較の表の「基準の構成」の列に出る。

**製品の制約 1〜3（AC-23）:** 0008 評価記録 §6.3 のとおり、3 つとも B3 にある（制約 1 は leE02uqgaKg-2、制約 2 は 18tbwwPM91A-2、制約 3 は EqnGQDU71-Y-2）。§10 の「制約が直ったか」の表の B3 の行に出る。

## 4. 素材と保護

**字幕のキャッシュ（AC-02）:** 評価用のキャッシュはリポジトリの外の `~/yt2column-eval/cache/` で、5 本（roNZ-rLxhUE・18tbwwPM91A・zW1RBacSqjk・leE02uqgaKg・EqnGQDU71-Y）とも 2026-10-08 に取得したスロット `a` である（0018 評価記録 §4.9.1）。実行ごとに写しを使い、`YT2COLUMN_YTDLP_PATH=/usr/bin/false` と `--keep-cache` で字幕を取り直さない（設計 §3.1）。

**実行の条件（AC-02）:** すべての実行の記録が `yt-dlp` を呼ばない設定と `--keep-cache` を持つことを数える。

<!-- check-record: run-conditions -->
| 項目 | 値 |
|---|---|
| runs | 50 |
| ytdlp_path_false | 50 |
| keep_cache | 50 |
| all_kept_conditions | yes |
| binary_sha256 | 6a975731b6aa34be0421dc891f9e365d3957b2d7cb1cffa047420b39f2179a09 |

**ハッシュの一覧とバックアップ（AC-04）:** 一覧は §2.3 の `hashlist.sh` の行で作った。対象は `base`（`0018/base/B3/`）・`work`（`0018/work/B3/`）・`cache`（評価用のキャッシュ）である。2 回目と 3 回目（ステップ 3-1・4-1）は、前の一覧との `diff` を記録する。

| 一覧 | 作った日時（UTC） | ファイルの数 | 一覧の SHA-256 | 前の一覧との `diff` |
|---|---|---|---|---|
| `0087/hash/1-initial.tsv`（実行の前の確認の前） | 2026-11-12T05:16:13Z | 50（`base` 10、`work` 21、`cache` 19。`cache` は評価の 5 本の各 3 ファイルに、評価に使わない qjAz-VOAdz4 の 3 ファイルとロックのファイル 1 つを含む） | `19694f72398b5e630af2ae816496f79ba1adb191328405fba3a8fe24a482b958` | -（最初の一覧） |
| `0087/hash/2-stage1.tsv`（1 段目の後） | 2026-11-12T06:51:32Z | 50 | `19694f72398b5e630af2ae816496f79ba1adb191328405fba3a8fe24a482b958` | 空（`hashlist.sh` が `no difference` を出した。一覧の SHA-256 も最初の一覧と同じ） |
| `0087/hash/3-stage2.tsv`（2 段目の後） | 2026-11-12T08:09:22Z | 50 | `19694f72398b5e630af2ae816496f79ba1adb191328405fba3a8fe24a482b958` | 空（`hashlist.sh` が `no difference` を出した。一覧の SHA-256 も最初の一覧と同じ） |

- **バックアップ:** 最初の一覧と同時に、`0018/` を起点とする `base/B3` と `work/B3` を `tar -czf` でまとめ、`0087/backup/b3-base-work.tar.gz`（モード `0400`）に置いた。アーカイブの SHA-256 は `8f7cba7b3283960f89ce8a3f14a91b68fef7a97f6e6e67edbf4f734cd75da11c` である。リポジトリの外の一時ディレクトリへ `tar -xpzf`（モードを保つ）で展開し、`hashlist.sh` で作った一覧（31 ファイル）が最初の一覧の `base`・`work` の行と、パス・モード・SHA-256 まで一致した（`diff` が空）。

**拒否された入力（AC-05）:** 1 段目も 2 段目も、なし（50 件とも終了コード `0`。失敗した実行はない）。

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

**上限のそのほかの前提:** 次の点も、上限が厳密な上限であることを保証しない。
- **生成などの固定の入力:** 0077 の 1 回の実測（`roNZ-rLxhUE`）をそのまま使い、余裕を足していない。実測は 0077 のアダプタができる前の測定用の入口で行ったので、要求の組み立ての違いで数トークン多くなりうる。超えた分は出力の上限の余裕で吸収される見込みである。ステップ 1-7 の後、1 段目の実行の実際の入力と比べる。
- **記事のバイト数の上限（17,000）:** 本文 4,000 字の各字を 4 バイトとし、タイトルと改行に 1,000 バイトを見込んだ値である。本文の字数は改行を数えず、タイトルには上限がないので、厳密な上限ではない。日本語の本文は 1 字 3 バイト程度なので、余裕は大きい。
- **出力の上限:** `bounds-input.json` の 6,080 と 40,000 は、アダプタの既定（`internal/llm/openai/request.go:29`、`internal/llm/claude/request.go:30`）から写した値で、照合する仕組みはない。`launch` は `-cli` が固定したバイナリ（SHA-256）であることを確かめるが、そのバイナリの既定がこの値であることは、ステップ 1-6 で固定したコミットの値と比べて確かめる。
- **使った額の読み取り:** 工程ごとの料金の行の和を `cost-total: run` の値と照合していない。工程の行が読めなかった場合に、その工程の分だけ少なく数えうる（0018 のログはすべて正しく読めた）。

**承認（AC-19）:** 2026-11-12 に、上の見積もりの表（1 段目の 4 構成と 2 段目の `B-sol`・`B-astra` の構成・推定コスト、1 段目と B の合計、字数の調整の上乗せ）と上限 30 USD を人に示し、6 構成について上限 30 USD で承認を受けた。見積もりは、ステップ 1-7 で `estimate` を実行し直して変わらなかったものである（§2.7）。承認は `0087/approval` に記録した（計画 §1.2 の決めたこと 8）。

<!-- check-record: approvals -->
| 順 | 承認（UTC） | 構成 | 上限（USD） |
|---|---|---|---|
| 1 | 2026-11-12T06:17:03Z | C-sol,C-astra,A-sol,A-astra,B-sol,B-astra | 30.000000 |

**承認より前に始まった実行（AC-19）:**

<!-- check-record: pre-approval-runs -->
| 構成 | 記事 | 回 | 開始（UTC） |
|---|---|---|---|

**使った額（AC-20）:** 「launch」は起動の判定が数える額、「record」は人が支払い前と確かめた実行を除いた、本書の合計である。額は CLI の推定コスト（料金表の単価）で、実際の請求額ではない。2026-11-12 の 1 段目の `C-*` の後、OpenAI のコンソールの請求額は CLI の推定よりずっと低く表示された。人によれば、アカウントの無料のトークン枠で相殺されている可能性がある。評価の比較（基準の構成を含む）はすべて CLI の推定コストで行う。

<!-- check-record: spend -->
| 範囲 | 分かっている額 | 不明の工程 | 上限で数えた実行 | 上限の額 | 合計 | 除いた実行 | 除いた上限の額 |
|---|---|---|---|---|---|---|---|
| launch | 7.733370 | 0 | 0 | 0.000000 | 7.733370 | 0 | 0.000000 |
| record | 7.733370 | 0 | 0 | 0.000000 | 7.733370 | 0 | 0.000000 |

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

**料金表のコミットと claude-opus-5-5 の単価の比較（AC-15）:** 2026-11-12 に、PR-2 のブランチのコミット `b435f279`（CLI に関わるツリーが固定したコミット `bc7208cd` と同じ。§2.6）で `rates` を実行した。出力は上の表と同じである。`rates-compare.sh` に評価に使うコミット（固定したコミット）を渡した結果は次のとおりで、`claude-opus-5-5` の単価は B3 の CLI のコミット `2fc2e2a` と同じだった。基準の構成の推定コストは求め直さず、`baseline.json` も変えていない。`bc7208cd` の `internal/pricing/rates.go` の各モデルの項目は、`claude-opus-5-5` が 112 行、`gpt-6-astra` が 132 行、`gpt-6.1-sol` が 141 行から始まる。

```text
evaluation commit bc7208cd941f: base:rates{input:4000000,cacheRead:200000,cacheWrite:5000000,output:20000000},
B3 commit 2fc2e2a: base:rates{input:4000000,cacheRead:200000,cacheWrite:5000000,output:20000000},
rates-compare: same
exit=0
```

## 7. 1 段目の各構成

**採点と人の確認（AC-09〜AC-11）:** 2026-11-12 に、`check_article.py` が 40 件の長さ・節の数・タイトルの字数を判定した（40 件とも `pass`）。`material` で伏せた素材を作り、`yt2column-0087-scorer` のエージェント（モデル ID `claude-opus-5-5`。5 つのエージェントすべてがこの ID を報告した）が、動画ごとに 1 つのサブエージェントで 8 件ずつ採点した（計画 §1.4 の決めたこと 3 のとおり、素材のファイルの名前の一覧をプロンプトに書いた）。採点の全文と、人の確認の記録は `0087/score/1/` にある。

人の確認の対象は設計 §3.6 の手順 4 の 274 判定である（必須の観点の不合格、品質の 1 と 2、観点「根拠」の合格、製品の制約 1〜3）。数が多いため、人の求めにより、次の 3 段で確認した（計画 §1.4 の決めたこと 4。設計 §3.6 の手順 4 の変更として、セッション中に再承認を受けた）。

- **論点ごとに決めた判定（20 判定）:** roNZ-rLxhUE の形式の選択（インタビュー記事を選ばなかった記事は不合格）、「ロボットが自衛隊」の読みやすさ（意味が取れないので 1）、EqnGQDU71-Y で未来会議の考えを全体の考えとした「根拠」（解釈で合格、文言どおりで不合格）、leE02uqgaKg の「地球サイズ」を「質量」とした誤り（根拠の不合格と制約 1）。
- **1 問ずつ示した判定（65 判定）:** 前後の段落と字幕の該当時刻を示し、人が答えた。同じ動画で本文と理由が同じ判定はまとめて示した。対象は、品質の 1 のうちほかの理由でまだ不合格になっていない記事のもの、採点者が 2 と 1 の境目とした構成、ほかに不合格の判定がない記事の「根拠」の合格のすべて（採点者が人の確認を求めたかどうかによらない）、製品の制約の「あり」である。
- **一括で確認した判定（189 判定）:** 境目と書かれていない品質の 2（100）、ほかの理由ですでに不合格の記事の「根拠」の合格（11）と品質の 1（2）、製品の制約の「なし」（76）。人は個々の判定を見ずに、LLM の判定のままとした。

| 種類 | 論点ごと | 1 問ずつ | 一括 |
|---|---|---|---|
| 必須の観点の不合格 | 7 | 0 | 0 |
| 観点「根拠」の合格 | 7 | 18 | 11 |
| 品質の 1 | 2 | 6 | 2 |
| 品質の 2 | 0 | 1 | 100 |
| 製品の制約の「あり」 | 4 | 37 | 0 |
| 製品の制約の「なし」 | 0 | 3 | 76 |

人が変えたのは 5 判定である。A-sol・A-astra の roNZ-rLxhUE-1 の「根拠」の文言どおりの判定（合格から不合格）と、A-sol・A-astra の zW1RBacSqjk-1 と C-astra の zW1RBacSqjk-2 の制約 1（「あり」から「なし」）である。表の件数は人が確かめた後の判定で数えた（制約 1 を変えた 3 判定は「なし」の行にある）。

**2 段目の確認の後の見直し:** 制約 1 の 3 判定と、A-sol・A-astra の zW1RBacSqjk-1 の「根拠」は、2 段目の確認（2026-11-12、`B-sol` の起動の後）で同じ言い換えを比べたときに、人が見直した。「横開き」を「横に広い」「横長」とした描写は、字幕の「パスポートみたいなサイズ」から導けるので制約 1 に当たらないとした。また、「Snapdragon 8 Elite for Galaxy」は字幕の「スnドラゴン8LEDと4ギx」の音の合う読み替えなので、zW1RBacSqjk-1 の「根拠」の文言どおりの判定を、1 段目でいったん不合格にしたものから採点者の判定（合格）に戻した。どちらも解釈による判定の合格件数を変えないので、B の構成の選択（§8）は変わらない。判定ごとの段（論点ごと・1 問ずつ・一括）と理由は `0087/score/1/human-decisions.tsv` にある。判定者の表の `llm+human` は、人の確認の対象で人が変えなかった LLM の判定で、一括で確認した判定を含む。対象の外の判定（品質の 3、「根拠」以外の必須の観点の合格）は `llm` である。

**A-sol と A-astra の判定がそろった理由:** 2 つの構成の 140 の判定の値は、すべて同じになった。採点の素材は構成ごとに別の名前で、それぞれの構成の保存した出力から作った（`0087/score/1/mapping.tsv`）。2 つの構成は B3 の同じ生成の出力から検証・推敲だけをやり直したもので、記事の本文の違いは記事ごとに 0〜6 行と小さい（leE02uqgaKg-1 と EqnGQDU71-Y-2 は本文が同じ）。

**プロンプトのキャッシュ（AC-14）:** 設計 §3.5 は、同じ構成の 10 件を同時に起動するので同じ動画の 2 件目が 1 件目のキャッシュを読むことは起こりにくいと見込んだ。この見込みは外れた。`C-sol` と `C-astra` では、多くの記事の生成の入力の大部分がキャッシュから読まれた（例: `C-sol` の roNZ-rLxhUE-2 は 17,303 トークンのうち 17,300）。キャッシュを読んだ工程の数と、読んだ入力をキャッシュの書き込みの単価で数え直した補助の推定コストの平均は、§9 の比較の表にある。

1 段目の人の確認の印（`0087/score/1/human-confirmed`）は 2026-11-12T08:04:55Z である（レビューの後に「根拠」の合格 7 判定を 1 問ずつ確認し直したので、最初の印 2026-11-12T07:48:05Z から書き直した）。

### 7.0. 実行の順序と並列の組（AC-06）

`launch` の起動の記録である。1 行が、同時に起動した 1 つの構成の記事の組である。

<!-- check-record: launches -->
| 順 | 起動（UTC） | 構成 | 記事 |
|---|---|---|---|
| 1 | 2026-11-12T06:17:30Z | C-sol | roNZ-rLxhUE-1,roNZ-rLxhUE-2,18tbwwPM91A-1,18tbwwPM91A-2,zW1RBacSqjk-1,zW1RBacSqjk-2,leE02uqgaKg-1,leE02uqgaKg-2,EqnGQDU71-Y-1,EqnGQDU71-Y-2 |
| 2 | 2026-11-12T06:19:07Z | C-astra | roNZ-rLxhUE-1,roNZ-rLxhUE-2,18tbwwPM91A-1,18tbwwPM91A-2,zW1RBacSqjk-1,zW1RBacSqjk-2,leE02uqgaKg-1,leE02uqgaKg-2,EqnGQDU71-Y-1,EqnGQDU71-Y-2 |
| 3 | 2026-11-12T06:49:44Z | A-sol | roNZ-rLxhUE-1,roNZ-rLxhUE-2,18tbwwPM91A-1,18tbwwPM91A-2,zW1RBacSqjk-1,zW1RBacSqjk-2,leE02uqgaKg-1,leE02uqgaKg-2,EqnGQDU71-Y-1,EqnGQDU71-Y-2 |
| 4 | 2026-11-12T06:50:43Z | A-astra | roNZ-rLxhUE-1,roNZ-rLxhUE-2,18tbwwPM91A-1,18tbwwPM91A-2,zW1RBacSqjk-1,zW1RBacSqjk-2,leE02uqgaKg-1,leE02uqgaKg-2,EqnGQDU71-Y-1,EqnGQDU71-Y-2 |
| 5 | 2026-11-12T08:07:25Z | B-sol | roNZ-rLxhUE-1,roNZ-rLxhUE-2,18tbwwPM91A-1,18tbwwPM91A-2,zW1RBacSqjk-1,zW1RBacSqjk-2,leE02uqgaKg-1,leE02uqgaKg-2,EqnGQDU71-Y-1,EqnGQDU71-Y-2 |

- **起動の関門で止まった起動:** `A-astra` の最初の `launch`（2026-11-12T06:50 台、上の 4 の前）は、同じ OpenAI のモデルを使う `C-astra` の最後の実行の終了（06:20:28Z）から 30 分に数秒足りず、実行を 1 件も始めずに止まった（`launch.log` に行はない）。30 分が過ぎたことを確かめて起動し直したのが 4 である。

### 7.1. C-sol

**設定（AC-06）:** 生成と字数の調整は `openai/gpt-6.1-sol+medium`、検証・推敲は `openai/gpt-6.1-sol+medium` で、テンプレートは既定である。全工程を実行する。CLI のコミットとバイナリの SHA-256 は §2 に記録する（ステップ 1-6）。

**実行（AC-06）:** 工程ごとの保存した出力の識別子と、実行を始めた時刻である。

<!-- check-record: runs config=C-sol -->
| 構成 | 記事 | 回 | `--after` | `YT2COLUMN_LLM` | `YT2COLUMN_REVIEW_LLM` | 開始（UTC） | 終了（UTC） | 終了コード | 生成の識別子 | 字数の調整の識別子 | 検証・推敲の識別子 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| C-sol | roNZ-rLxhUE-1 | cur | none | openai/gpt-6.1-sol+medium | openai/gpt-6.1-sol+medium | 2026-11-12T06:17:31Z | 2026-11-12T06:18:43Z | 0 | 636c4210bec438a833c6eb00ed2179060845685f4a28cc5ebd9ed6d97c3b02d2 | - | 6811e0b054c5c154a408c1131c3e9df78cc3023e3448e2c35447d692e48a328c |
| C-sol | roNZ-rLxhUE-2 | cur | none | openai/gpt-6.1-sol+medium | openai/gpt-6.1-sol+medium | 2026-11-12T06:17:30Z | 2026-11-12T06:18:36Z | 0 | 149cf6df529f57905a115710369f84207f996cc3e362479f7f0fc9da8c952750 | - | 7a5b21fa50d92ba9aa7025852b6667062a2dc2b97c74f1350895516f1a8a7dbf |
| C-sol | 18tbwwPM91A-1 | cur | none | openai/gpt-6.1-sol+medium | openai/gpt-6.1-sol+medium | 2026-11-12T06:17:30Z | 2026-11-12T06:18:53Z | 0 | 123a06eb629ad5a36f20e34467efebf761aa36952241be85dda84dd2135da99d | - | cbad7a3ffbab280fbbd5b8b033442fcdcd8e0af452293f8a6aefd48e846cb0ec |
| C-sol | 18tbwwPM91A-2 | cur | none | openai/gpt-6.1-sol+medium | openai/gpt-6.1-sol+medium | 2026-11-12T06:17:30Z | 2026-11-12T06:18:36Z | 0 | 078a57918b0e92a873793025669504e40edd61b02574848047dcf2e03e9dde09 | - | db16f8707c1d6fe944a1c16b0a65e82892d1f73d0acbfb565f004479d4502c57 |
| C-sol | zW1RBacSqjk-1 | cur | none | openai/gpt-6.1-sol+medium | openai/gpt-6.1-sol+medium | 2026-11-12T06:17:30Z | 2026-11-12T06:18:27Z | 0 | 5a7ce1c329e76d104f4c3633e9aeef2be941ec325ef22a8ec65b487e47d81974 | - | 274d83e12c481a7b2cc0653e55693938ce4923f0a0f0e1147c3d046b316f4422 |
| C-sol | zW1RBacSqjk-2 | cur | none | openai/gpt-6.1-sol+medium | openai/gpt-6.1-sol+medium | 2026-11-12T06:17:30Z | 2026-11-12T06:18:40Z | 0 | 73fd08924540ba5fc984964c6451b84b7b4bee8801bf4d4d69a46765c26020f1 | - | f52cdeedd65e4fc4f874df2a7d49ef1f624f60a44307617ee39f6e14c2b3e7b2 |
| C-sol | leE02uqgaKg-1 | cur | none | openai/gpt-6.1-sol+medium | openai/gpt-6.1-sol+medium | 2026-11-12T06:17:30Z | 2026-11-12T06:18:36Z | 0 | e8f6148b06b8bf36df4110e1f4b1fe7ad19c7c992a923bf048746fe290e50a81 | - | d08f1059207b066658dcc0b9faec33473d1f287b8fe2f256e164ce4962ca668e |
| C-sol | leE02uqgaKg-2 | cur | none | openai/gpt-6.1-sol+medium | openai/gpt-6.1-sol+medium | 2026-11-12T06:17:30Z | 2026-11-12T06:18:40Z | 0 | 9b20df2a2b6b9fa04442343bfb7ba2781b990d6f5e08b3a8d95cafaa4cbf4b7c | - | f5882172117b16ead539588c9c6f7f2aedfc5f337ce32a82a9fe0d3229779d61 |
| C-sol | EqnGQDU71-Y-1 | cur | none | openai/gpt-6.1-sol+medium | openai/gpt-6.1-sol+medium | 2026-11-12T06:17:30Z | 2026-11-12T06:18:23Z | 0 | aab59732ec16a9bee00e6f2308cc944b84a2a77c10972fd854d39fe6e9597f07 | - | 3faef40b4b1413cdcd12cc6781fc03163eb5c6c4e62d1507a6a74be5adf05893 |
| C-sol | EqnGQDU71-Y-2 | cur | none | openai/gpt-6.1-sol+medium | openai/gpt-6.1-sol+medium | 2026-11-12T06:17:30Z | 2026-11-12T06:18:21Z | 0 | 147a0a89966e4b5b4d47b7b649575b03b65ab66fb5010a84a523c2c04ba72d10 | - | 06fc4cd59f218f622ff8325987e7fbab512ba4e968d90389fcb90cd98b2c4613 |

**トークン数と推定コスト（AC-14）:** 工程ごとの CLI の値である。「再利用した実行」は、再開で呼ばなかった工程の値の出どころ（B3 の記事）を示す。

<!-- check-record: stage-usage config=C-sol -->
| 構成 | 記事 | 回 | 工程 | 出どころ | 使用量 | 入力 | キャッシュから読んだ入力 | キャッシュに書いた入力 | 出力 | モデル | 推定コスト（USD） | 不明の理由 | 再利用した実行 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| C-sol | roNZ-rLxhUE-1 | cur | generation | run | known | 3 | 5386 | 11914 | 3143 | gpt-6.1-sol | 0.061760 | - | - |
| C-sol | roNZ-rLxhUE-1 | cur | review | run | known | 3 | 0 | 15107 | 112 | gpt-6.1-sol | 0.038894 | - | - |
| C-sol | roNZ-rLxhUE-2 | cur | generation | run | known | 3 | 17300 | 0 | 3056 | gpt-6.1-sol | 0.032296 | - | - |
| C-sol | roNZ-rLxhUE-2 | cur | review | run | known | 3 | 0 | 14927 | 103 | gpt-6.1-sol | 0.038354 | - | - |
| C-sol | 18tbwwPM91A-1 | cur | generation | run | known | 3 | 13328 | 0 | 2860 | gpt-6.1-sol | 0.029939 | - | - |
| C-sol | 18tbwwPM91A-1 | cur | review | run | known | 3 | 0 | 10929 | 627 | gpt-6.1-sol | 0.033599 | - | - |
| C-sol | 18tbwwPM91A-2 | cur | generation | run | known | 3 | 0 | 13328 | 2677 | gpt-6.1-sol | 0.060096 | - | - |
| C-sol | 18tbwwPM91A-2 | cur | review | run | known | 3 | 0 | 10798 | 316 | gpt-6.1-sol | 0.030161 | - | - |
| C-sol | zW1RBacSqjk-1 | cur | generation | run | known | 3 | 5386 | 6468 | 2544 | gpt-6.1-sol | 0.042155 | - | - |
| C-sol | zW1RBacSqjk-1 | cur | review | run | known | 3 | 0 | 9155 | 150 | gpt-6.1-sol | 0.024394 | - | - |
| C-sol | zW1RBacSqjk-2 | cur | generation | run | known | 3 | 11854 | 0 | 2764 | gpt-6.1-sol | 0.028831 | - | - |
| C-sol | zW1RBacSqjk-2 | cur | review | run | known | 3 | 0 | 9323 | 153 | gpt-6.1-sol | 0.024844 | - | - |
| C-sol | leE02uqgaKg-1 | cur | generation | run | known | 3 | 12384 | 0 | 2744 | gpt-6.1-sol | 0.028684 | - | - |
| C-sol | leE02uqgaKg-1 | cur | review | run | known | 3 | 0 | 9665 | 65 | gpt-6.1-sol | 0.024819 | - | - |
| C-sol | leE02uqgaKg-2 | cur | generation | run | known | 3 | 5386 | 6998 | 2710 | gpt-6.1-sol | 0.045140 | - | - |
| C-sol | leE02uqgaKg-2 | cur | review | run | known | 3 | 0 | 9827 | 437 | gpt-6.1-sol | 0.028944 | - | - |
| C-sol | EqnGQDU71-Y-1 | cur | generation | run | known | 3 | 5386 | 4111 | 2559 | gpt-6.1-sol | 0.036412 | - | - |
| C-sol | EqnGQDU71-Y-1 | cur | review | run | known | 3 | 0 | 6855 | 122 | gpt-6.1-sol | 0.018364 | - | - |
| C-sol | EqnGQDU71-Y-2 | cur | generation | run | known | 3 | 9497 | 0 | 2514 | gpt-6.1-sol | 0.026096 | - | - |
| C-sol | EqnGQDU71-Y-2 | cur | review | run | known | 3 | 0 | 6651 | 69 | gpt-6.1-sol | 0.017324 | - | - |

**記事ごとの推定コストと所要時間（AC-14・AC-16）:** 所要時間は記事を作った工程の時間の和で、10 件を並列に起動して測った値である（基準の構成と同じ。0018 評価記録 §4.8）。

<!-- check-record: run-costs config=C-sol -->
| 構成 | 記事 | 回 | 終了コード | この実行（USD） | 記事（USD） | 実行の時間 | 所要時間 | 比較に使う |
|---|---|---|---|---|---|---|---|---|
| C-sol | roNZ-rLxhUE-1 | cur | 0 | 0.100653 | 0.100653 | 1:12.7 | 1:12.7 | yes |
| C-sol | roNZ-rLxhUE-2 | cur | 0 | 0.070650 | 0.070650 | 1:05.5 | 1:05.5 | yes |
| C-sol | 18tbwwPM91A-1 | cur | 0 | 0.063537 | 0.063537 | 1:22.6 | 1:22.6 | yes |
| C-sol | 18tbwwPM91A-2 | cur | 0 | 0.090257 | 0.090257 | 1:05.7 | 1:05.7 | yes |
| C-sol | zW1RBacSqjk-1 | cur | 0 | 0.066548 | 0.066548 | 0:56.3 | 0:56.3 | yes |
| C-sol | zW1RBacSqjk-2 | cur | 0 | 0.053675 | 0.053675 | 1:09.6 | 1:09.6 | yes |
| C-sol | leE02uqgaKg-1 | cur | 0 | 0.053503 | 0.053503 | 1:05.8 | 1:05.8 | yes |
| C-sol | leE02uqgaKg-2 | cur | 0 | 0.074083 | 0.074083 | 1:09.4 | 1:09.4 | yes |
| C-sol | EqnGQDU71-Y-1 | cur | 0 | 0.054776 | 0.054776 | 0:52.1 | 0:52.1 | yes |
| C-sol | EqnGQDU71-Y-2 | cur | 0 | 0.043419 | 0.043419 | 0:51.0 | 0:51.0 | yes |

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
| C-sol | roNZ-rLxhUE-1 | fail/fail | pass/pass | pass/pass | pass/pass | pass/pass | 2/2 | 2/2 | 1/1 | 3/3 | 3/3 | 2/2 | no | no |
| C-sol | roNZ-rLxhUE-2 | pass/pass | pass/pass | pass/fail | pass/pass | pass/pass | 2/2 | 2/2 | 2/2 | 3/3 | 3/3 | 2/2 | yes | no |
| C-sol | 18tbwwPM91A-1 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 2/2 | 2/2 | 1/1 | 3/3 | 2/2 | 2/2 | no | no |
| C-sol | 18tbwwPM91A-2 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 2/2 | 2/2 | 1/1 | 3/3 | 3/3 | 2/2 | no | no |
| C-sol | zW1RBacSqjk-1 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 3/3 | 2/2 | 2/2 | 3/3 | 2/2 | yes | yes |
| C-sol | zW1RBacSqjk-2 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 3/3 | 2/2 | 2/2 | 3/3 | 2/2 | yes | yes |
| C-sol | leE02uqgaKg-1 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 2/2 | 3/3 | 2/2 | 3/3 | 3/3 | 2/2 | yes | yes |
| C-sol | leE02uqgaKg-2 | pass/pass | pass/pass | fail/fail | pass/pass | pass/pass | 3/3 | 3/3 | 2/2 | 3/3 | 3/3 | 2/2 | no | no |
| C-sol | EqnGQDU71-Y-1 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 2/2 | yes | yes |
| C-sol | EqnGQDU71-Y-2 | pass/pass | pass/pass | pass/fail | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 2/2 | yes | no |

**判定者（AC-10）:** `script` はスクリプト、`llm` は人の確認の対象の外の LLM の判定、`llm+human` は人の確認の対象で人が変えなかった LLM の判定（一括で確認した判定を含む。§7 の冒頭）、`human` は人が変えた判定である。人が変えた判定と理由は下に書く。

<!-- check-record: judges config=C-sol -->
| 構成 | 記事 | 形式の選択 | 形式の規則 | 根拠 | 長さ | 表記 | 網羅 | 重要度 | 構成（観点） | 読みやすさ | タイトル・リード文 | 冗長さ | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| C-sol | roNZ-rLxhUE-1 | llm+human | llm | llm+human | script | llm | llm+human | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| C-sol | roNZ-rLxhUE-2 | llm | llm | llm+human | script | llm | llm+human | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| C-sol | 18tbwwPM91A-1 | llm | llm | llm+human | script | llm | llm+human | llm+human | llm+human | llm | llm+human | llm+human | llm+human | llm+human | llm+human |
| C-sol | 18tbwwPM91A-2 | llm | llm | llm+human | script | llm | llm+human | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| C-sol | zW1RBacSqjk-1 | llm | llm | llm+human | script | llm | llm | llm | llm+human | llm+human | llm | llm+human | llm+human | llm+human | llm+human |
| C-sol | zW1RBacSqjk-2 | llm | llm | llm+human | script | llm | llm | llm | llm+human | llm+human | llm | llm+human | llm+human | llm+human | llm+human |
| C-sol | leE02uqgaKg-1 | llm | llm | llm+human | script | llm | llm+human | llm | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| C-sol | leE02uqgaKg-2 | llm | llm | llm+human | script | llm | llm | llm | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| C-sol | EqnGQDU71-Y-1 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| C-sol | EqnGQDU71-Y-2 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |

**機械的な値（AC-09）:** `check_article.py` の出力である。

<!-- check-record: article-checks config=C-sol -->
| 構成 | 記事 | タイトルの字数 | 節の数 | 長さ |
|---|---|---|---|---|
| C-sol | roNZ-rLxhUE-1 | 17 | 5 | 3497 |
| C-sol | roNZ-rLxhUE-2 | 21 | 5 | 3489 |
| C-sol | 18tbwwPM91A-1 | 21 | 4 | 3243 |
| C-sol | 18tbwwPM91A-2 | 21 | 4 | 3076 |
| C-sol | zW1RBacSqjk-1 | 27 | 4 | 3141 |
| C-sol | zW1RBacSqjk-2 | 25 | 4 | 3390 |
| C-sol | leE02uqgaKg-1 | 22 | 3 | 2879 |
| C-sol | leE02uqgaKg-2 | 18 | 4 | 3056 |
| C-sol | EqnGQDU71-Y-1 | 23 | 4 | 2920 |
| C-sol | EqnGQDU71-Y-2 | 23 | 3 | 2664 |

**引用が要る判定（AC-12）:** 引用、誤りを入れた工程、直さなかった工程は下に書く。

<!-- check-record: quotes-needed config=C-sol -->
| 構成 | 記事 | 観点 | 判定 | 種類 |
|---|---|---|---|---|
| C-sol | roNZ-rLxhUE-1 | format_choice | both | required-fail |
| C-sol | roNZ-rLxhUE-1 | structure | both | quality-1 |
| C-sol | roNZ-rLxhUE-2 | evidence | literal | required-fail |
| C-sol | 18tbwwPM91A-1 | structure | both | quality-1 |
| C-sol | 18tbwwPM91A-2 | structure | both | quality-1 |
| C-sol | leE02uqgaKg-2 | evidence | both | required-fail |
| C-sol | EqnGQDU71-Y-2 | evidence | literal | required-fail |

**製品の制約 1〜3（AC-13）:** 引用は下に書く。

<!-- check-record: constraints config=C-sol -->
| 構成 | 記事 | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|
| C-sol | roNZ-rLxhUE-1 | no | yes | no |
| C-sol | roNZ-rLxhUE-2 | no | yes | no |
| C-sol | 18tbwwPM91A-1 | no | yes | no |
| C-sol | 18tbwwPM91A-2 | no | yes | no |
| C-sol | zW1RBacSqjk-1 | no | yes | no |
| C-sol | zW1RBacSqjk-2 | no | yes | no |
| C-sol | leE02uqgaKg-1 | no | yes | no |
| C-sol | leE02uqgaKg-2 | yes | yes | no |
| C-sol | EqnGQDU71-Y-1 | no | yes | yes |
| C-sol | EqnGQDU71-Y-2 | no | yes | yes |

**失敗した実行と番兵（AC-17）:** なし（10 件とも終了コード `0`）。
**人が変えた判定（AC-10）:** なし。

**引用（AC-12）:** 必須の観点の不合格と品質の 1 である。「入った」は誤りを入れた工程、「残した」は直さなかった工程である。
- roNZ-rLxhUE-1 形式の選択（fail / fail）: 「小泉がまず警戒するのは、同盟が正式に解消されることだけではない」（字幕 17:55）。入った: 生成、残した: -（検証・推敲は形式を変えない）。
- roNZ-rLxhUE-1 構成（1 / 1）: 「小泉はGCAPについても、それさえ配備すれば中国の戦闘機を圧倒し」（字幕 09:51）。入った: 生成、残した: 検証・推敲。
- roNZ-rLxhUE-2 根拠（pass / fail）: 「――機械化は、人員を補う以外にも意味があるのでしょうか。」（字幕 29:35）。入った: 生成、残した: 検証・推敲。
- 18tbwwPM91A-1 構成（1 / 1）: 「この差は、ボールを捉える感覚にも表れた」（字幕 10:53）。入った: 生成、残した: 検証・推敲。
- 18tbwwPM91A-2 構成（1 / 1）: 「逆方向への本塁打にも、同じような驚きがある」（字幕 14:03）。入った: 生成、残した: 検証・推敲。
- leE02uqgaKg-2 根拠（fail / fail）: 「一方、地球や太陽ほどの質量になると、月や惑星の運動を左右する」（字幕 02:13）。入った: 生成、残した: 検証・推敲。
- EqnGQDU71-Y-2 根拠（pass / fail）: 「計画で大切にされているのは「町民に愛される場所づくり」である」（字幕 06:21-06:36）。入った: 生成、残した: 検証・推敲。

**製品の制約 1〜3 の引用（AC-13）:**
- roNZ-rLxhUE-1 制約 2: 「小泉はGCAPについても、それさえ配備すれば中国の戦闘機を圧倒し」（字幕 09:51）
- roNZ-rLxhUE-2 制約 2: 「――中国との関係調整を考える一方、台湾問題にはどう向き合うべきですか。」（字幕 26:23）
- 18tbwwPM91A-1 制約 2: 「この差は、ボールを捉える感覚にも表れた」（字幕 10:53）
- 18tbwwPM91A-2 制約 2: 「逆方向への本塁打にも、同じような驚きがある」（字幕 14:03）
- zW1RBacSqjk-1 制約 2: 「今回の製品を理解するうえで注意したいのが、Foldの名称である。」（字幕 00:44）
- zW1RBacSqjk-2 制約 2: 「比較の前提として、今回のモデル名には注意が必要だ。」（字幕 00:44）
- leE02uqgaKg-1 制約 2: 「この限界を越える試みとして紹介されるのが、自然界の四つの相互作用を統一する研究」（字幕 14:25）
- leE02uqgaKg-2 制約 1: 「地球や太陽ほどの質量になると、月や惑星の運動を左右する」（字幕 02:13）
- leE02uqgaKg-2 制約 2: 「その先の目標として示されるのが、重力を含む四つの相互作用を統一する「万物の理論」だ」（字幕 14:25）
- EqnGQDU71-Y-1 制約 2: 「ここに至る道のりは平坦ではなかった。新博物館のプロジェクトは2018年に始まった」（字幕 01:56）
- EqnGQDU71-Y-1 制約 3: 「目指すのは、観光客を呼び込むだけでなく、町民が繰り返し訪れ」（字幕 04:31-04:45）
- EqnGQDU71-Y-2 制約 2: 「町内の事業者にも、新施設を待つ声がある。」（字幕 03:06）
- EqnGQDU71-Y-2 制約 3: 「目指しているのは、観光客を呼び込むだけでなく、町民が繰り返し訪れ」（字幕 04:31-04:45）

### 7.2. C-astra

**設定（AC-06）:** 生成と字数の調整は `openai/gpt-6-astra+medium`、検証・推敲は `openai/gpt-6-astra+medium` で、テンプレートは既定である。全工程を実行する。CLI のコミットとバイナリの SHA-256 は §2 に記録する（ステップ 1-6）。

**実行（AC-06）:** 工程ごとの保存した出力の識別子と、実行を始めた時刻である。

<!-- check-record: runs config=C-astra -->
| 構成 | 記事 | 回 | `--after` | `YT2COLUMN_LLM` | `YT2COLUMN_REVIEW_LLM` | 開始（UTC） | 終了（UTC） | 終了コード | 生成の識別子 | 字数の調整の識別子 | 検証・推敲の識別子 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| C-astra | roNZ-rLxhUE-1 | cur | none | openai/gpt-6-astra+medium | openai/gpt-6-astra+medium | 2026-11-12T06:19:07Z | 2026-11-12T06:20:25Z | 0 | b64fda7cb6996514bd8331c88a2f0630fca514a9870443f7ddbfd70ccc470ab2 | - | d2750c985be02f6609b729d0fb0b3f2ca6e9e2a9bd1ce247b6df94fec0563ad3 |
| C-astra | roNZ-rLxhUE-2 | cur | none | openai/gpt-6-astra+medium | openai/gpt-6-astra+medium | 2026-11-12T06:19:07Z | 2026-11-12T06:20:12Z | 0 | c4d8cf45870dbb80a24f27368d57018e660cbad37d9333f02214469ee14c0bf7 | - | 818b7630d089df29f5caf8cb27cd06437717ef78e8e44b0a2d035299c8c230ea |
| C-astra | 18tbwwPM91A-1 | cur | none | openai/gpt-6-astra+medium | openai/gpt-6-astra+medium | 2026-11-12T06:19:07Z | 2026-11-12T06:20:28Z | 0 | cc676646dc9814c26725ff145aadb007a1804b38433d5c52c23a32b1b3deae69 | - | 4cb5cf76437144227bce4dd6323fbe85772c51add45def91a79892e48dc082d1 |
| C-astra | 18tbwwPM91A-2 | cur | none | openai/gpt-6-astra+medium | openai/gpt-6-astra+medium | 2026-11-12T06:19:07Z | 2026-11-12T06:20:19Z | 0 | 64138a3efc1e7622271a8c2e9e5cb25fdf8b95a2c313da4a6e74bdbcc6db4087 | - | ddee41ef47f46e7ecfaf7b4745d3e4de7da43eab3135b4129787dfef51f7817f |
| C-astra | zW1RBacSqjk-1 | cur | none | openai/gpt-6-astra+medium | openai/gpt-6-astra+medium | 2026-11-12T06:19:07Z | 2026-11-12T06:20:13Z | 0 | 8f6eebd90a68bc38f464d01140729635c452b9f0c0faa4d72bf9bb3926cfd107 | - | bb9b8e3af43e7784b2cc4e4fad69d07aa2d90be24cd97edd770a3b622871c0d4 |
| C-astra | zW1RBacSqjk-2 | cur | none | openai/gpt-6-astra+medium | openai/gpt-6-astra+medium | 2026-11-12T06:19:07Z | 2026-11-12T06:20:03Z | 0 | 04c3e415d1fb04bf72c2e22d19c68323355ed85343648ed58e957caf88d9afb1 | - | 5cce63cdc46c0f6bbff552fbedd5b1d6fc20f8c1005e62be73c37b4258cdf3f0 |
| C-astra | leE02uqgaKg-1 | cur | none | openai/gpt-6-astra+medium | openai/gpt-6-astra+medium | 2026-11-12T06:19:07Z | 2026-11-12T06:20:11Z | 0 | bf283c12b8bdab7197fcfde4afdb15f647fb9febf2226533f83991ed916948b3 | - | f1b6d388c24fdc45cf1e22273acd5fd284a0a29297abd3b44e32f678cca410a0 |
| C-astra | leE02uqgaKg-2 | cur | none | openai/gpt-6-astra+medium | openai/gpt-6-astra+medium | 2026-11-12T06:19:07Z | 2026-11-12T06:20:12Z | 0 | 6880d8f6531c6902da740be1f5f5a4be37377f0bf1ecb5e94d0cde7861e4abe7 | - | 5b43db984f99472254abe5d62a402a46cf83738ae9dd5619c5b24d8c40769485 |
| C-astra | EqnGQDU71-Y-1 | cur | none | openai/gpt-6-astra+medium | openai/gpt-6-astra+medium | 2026-11-12T06:19:07Z | 2026-11-12T06:19:57Z | 0 | 8a6062128a35bddec1fce1624d48f532b0dc08873fe879aff775163a41eb39a5 | - | ee444d2cd03148b9830076bb3ba4f676d41291258fa3083c9dfe1c2d7fd6cd81 |
| C-astra | EqnGQDU71-Y-2 | cur | none | openai/gpt-6-astra+medium | openai/gpt-6-astra+medium | 2026-11-12T06:19:07Z | 2026-11-12T06:19:59Z | 0 | 268f484dbcb6761d08b490569a4fed31ef222dc7622f8c9464e600baac722d7f | - | 8ab2fca07f0547be53cbdea56a4e53887cd85b10262c24a0e4bb0594b982aed9 |

**トークン数と推定コスト（AC-14）:** 工程ごとの CLI の値である。「再利用した実行」は、再開で呼ばなかった工程の値の出どころ（B3 の記事）を示す。

<!-- check-record: stage-usage config=C-astra -->
| 構成 | 記事 | 回 | 工程 | 出どころ | 使用量 | 入力 | キャッシュから読んだ入力 | キャッシュに書いた入力 | 出力 | モデル | 推定コスト（USD） | 不明の理由 | 再利用した実行 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| C-astra | roNZ-rLxhUE-1 | cur | generation | run | known | 3 | 17300 | 0 | 3246 | gpt-6-astra | 0.179630 | - | - |
| C-astra | roNZ-rLxhUE-1 | cur | review | run | known | 3 | 0 | 15308 | 474 | gpt-6-astra | 0.215080 | - | - |
| C-astra | roNZ-rLxhUE-2 | cur | generation | run | known | 3 | 5386 | 11914 | 3047 | gpt-6-astra | 0.306691 | - | - |
| C-astra | roNZ-rLxhUE-2 | cur | review | run | known | 3 | 0 | 15143 | 87 | gpt-6-astra | 0.193668 | - | - |
| C-astra | 18tbwwPM91A-1 | cur | generation | run | known | 3 | 13328 | 0 | 2901 | gpt-6-astra | 0.158408 | - | - |
| C-astra | 18tbwwPM91A-1 | cur | review | run | known | 3 | 0 | 11017 | 633 | gpt-6-astra | 0.169393 | - | - |
| C-astra | 18tbwwPM91A-2 | cur | generation | run | known | 3 | 5386 | 7942 | 2945 | gpt-6-astra | 0.251941 | - | - |
| C-astra | 18tbwwPM91A-2 | cur | review | run | known | 3 | 0 | 11061 | 374 | gpt-6-astra | 0.156993 | - | - |
| C-astra | zW1RBacSqjk-1 | cur | generation | run | known | 3 | 11854 | 0 | 2750 | gpt-6-astra | 0.149384 | - | - |
| C-astra | zW1RBacSqjk-1 | cur | review | run | known | 3 | 0 | 9393 | 281 | gpt-6-astra | 0.131493 | - | - |
| C-astra | zW1RBacSqjk-2 | cur | generation | run | known | 3 | 5386 | 6468 | 2633 | gpt-6-astra | 0.217916 | - | - |
| C-astra | zW1RBacSqjk-2 | cur | review | run | known | 3 | 0 | 9282 | 91 | gpt-6-astra | 0.120605 | - | - |
| C-astra | leE02uqgaKg-1 | cur | generation | run | known | 3 | 0 | 12384 | 2930 | gpt-6-astra | 0.301330 | - | - |
| C-astra | leE02uqgaKg-1 | cur | review | run | known | 3 | 0 | 10065 | 74 | gpt-6-astra | 0.129543 | - | - |
| C-astra | leE02uqgaKg-2 | cur | generation | run | known | 3 | 0 | 12384 | 2916 | gpt-6-astra | 0.300630 | - | - |
| C-astra | leE02uqgaKg-2 | cur | review | run | known | 3 | 0 | 9899 | 67 | gpt-6-astra | 0.127118 | - | - |
| C-astra | EqnGQDU71-Y-1 | cur | generation | run | known | 3 | 0 | 9497 | 2491 | gpt-6-astra | 0.243293 | - | - |
| C-astra | EqnGQDU71-Y-1 | cur | review | run | known | 3 | 0 | 6716 | 58 | gpt-6-astra | 0.086880 | - | - |
| C-astra | EqnGQDU71-Y-2 | cur | generation | run | known | 3 | 9497 | 0 | 2530 | gpt-6-astra | 0.136027 | - | - |
| C-astra | EqnGQDU71-Y-2 | cur | review | run | known | 3 | 0 | 6724 | 48 | gpt-6-astra | 0.086480 | - | - |

**記事ごとの推定コストと所要時間（AC-14・AC-16）:** 所要時間は記事を作った工程の時間の和で、10 件を並列に起動して測った値である（基準の構成と同じ。0018 評価記録 §4.8）。

<!-- check-record: run-costs config=C-astra -->
| 構成 | 記事 | 回 | 終了コード | この実行（USD） | 記事（USD） | 実行の時間 | 所要時間 | 比較に使う |
|---|---|---|---|---|---|---|---|---|
| C-astra | roNZ-rLxhUE-1 | cur | 0 | 0.394710 | 0.394710 | 1:18.0 | 1:18.0 | yes |
| C-astra | roNZ-rLxhUE-2 | cur | 0 | 0.500359 | 0.500359 | 1:05.2 | 1:05.2 | yes |
| C-astra | 18tbwwPM91A-1 | cur | 0 | 0.327801 | 0.327801 | 1:20.4 | 1:20.4 | yes |
| C-astra | 18tbwwPM91A-2 | cur | 0 | 0.408934 | 0.408934 | 1:12.1 | 1:12.1 | yes |
| C-astra | zW1RBacSqjk-1 | cur | 0 | 0.280877 | 0.280877 | 1:05.3 | 1:05.3 | yes |
| C-astra | zW1RBacSqjk-2 | cur | 0 | 0.338521 | 0.338521 | 0:56.1 | 0:56.1 | yes |
| C-astra | leE02uqgaKg-1 | cur | 0 | 0.430873 | 0.430873 | 1:04.2 | 1:04.2 | yes |
| C-astra | leE02uqgaKg-2 | cur | 0 | 0.427748 | 0.427748 | 1:04.6 | 1:04.6 | yes |
| C-astra | EqnGQDU71-Y-1 | cur | 0 | 0.330173 | 0.330173 | 0:49.6 | 0:49.6 | yes |
| C-astra | EqnGQDU71-Y-2 | cur | 0 | 0.222507 | 0.222507 | 0:51.5 | 0:51.5 | yes |

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
| C-astra | roNZ-rLxhUE-1 | fail/fail | pass/pass | pass/pass | pass/pass | pass/pass | 2/2 | 2/2 | 2/2 | 3/3 | 3/3 | 2/2 | no | no |
| C-astra | roNZ-rLxhUE-2 | fail/fail | pass/pass | pass/pass | pass/pass | pass/pass | 2/2 | 2/2 | 2/2 | 3/3 | 3/3 | 1/1 | no | no |
| C-astra | 18tbwwPM91A-1 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 2/2 | 3/3 | 1/1 | 3/3 | 2/2 | 2/2 | no | no |
| C-astra | 18tbwwPM91A-2 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 2/2 | 3/3 | 1/1 | 3/3 | 3/3 | 2/2 | no | no |
| C-astra | zW1RBacSqjk-1 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 3/3 | 2/2 | 3/3 | 3/3 | 2/2 | yes | yes |
| C-astra | zW1RBacSqjk-2 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 3/3 | 2/2 | 3/3 | 3/3 | 2/2 | yes | yes |
| C-astra | leE02uqgaKg-1 | pass/pass | pass/pass | fail/fail | pass/pass | pass/pass | 2/2 | 3/3 | 3/3 | 3/3 | 3/3 | 2/2 | no | no |
| C-astra | leE02uqgaKg-2 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 2/2 | 3/3 | 2/2 | 3/3 | 2/2 | 2/2 | yes | yes |
| C-astra | EqnGQDU71-Y-1 | pass/pass | pass/pass | pass/fail | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 2/2 | yes | no |
| C-astra | EqnGQDU71-Y-2 | pass/pass | pass/pass | pass/fail | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 2/2 | yes | no |

**判定者（AC-10）:** `script` はスクリプト、`llm` は人の確認の対象の外の LLM の判定、`llm+human` は人の確認の対象で人が変えなかった LLM の判定（一括で確認した判定を含む。§7 の冒頭）、`human` は人が変えた判定である。人が変えた判定と理由は下に書く。

<!-- check-record: judges config=C-astra -->
| 構成 | 記事 | 形式の選択 | 形式の規則 | 根拠 | 長さ | 表記 | 網羅 | 重要度 | 構成（観点） | 読みやすさ | タイトル・リード文 | 冗長さ | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| C-astra | roNZ-rLxhUE-1 | llm+human | llm | llm+human | script | llm | llm+human | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| C-astra | roNZ-rLxhUE-2 | llm+human | llm | llm+human | script | llm | llm+human | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| C-astra | 18tbwwPM91A-1 | llm | llm | llm+human | script | llm | llm+human | llm | llm+human | llm | llm+human | llm+human | llm+human | llm+human | llm+human |
| C-astra | 18tbwwPM91A-2 | llm | llm | llm+human | script | llm | llm+human | llm | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| C-astra | zW1RBacSqjk-1 | llm | llm | llm+human | script | llm | llm | llm | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| C-astra | zW1RBacSqjk-2 | llm | llm | llm+human | script | llm | llm | llm | llm+human | llm | llm | llm+human | human | llm+human | llm+human |
| C-astra | leE02uqgaKg-1 | llm | llm | llm+human | script | llm | llm+human | llm | llm | llm | llm | llm+human | llm+human | llm+human | llm+human |
| C-astra | leE02uqgaKg-2 | llm | llm | llm+human | script | llm | llm+human | llm | llm+human | llm | llm+human | llm+human | llm+human | llm+human | llm+human |
| C-astra | EqnGQDU71-Y-1 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| C-astra | EqnGQDU71-Y-2 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |

**機械的な値（AC-09）:** `check_article.py` の出力である。

<!-- check-record: article-checks config=C-astra -->
| 構成 | 記事 | タイトルの字数 | 節の数 | 長さ |
|---|---|---|---|---|
| C-astra | roNZ-rLxhUE-1 | 19 | 5 | 3712 |
| C-astra | roNZ-rLxhUE-2 | 19 | 5 | 3541 |
| C-astra | 18tbwwPM91A-1 | 21 | 4 | 3364 |
| C-astra | 18tbwwPM91A-2 | 22 | 4 | 3386 |
| C-astra | zW1RBacSqjk-1 | 22 | 4 | 3483 |
| C-astra | zW1RBacSqjk-2 | 28 | 4 | 3354 |
| C-astra | leE02uqgaKg-1 | 15 | 4 | 3395 |
| C-astra | leE02uqgaKg-2 | 15 | 4 | 3171 |
| C-astra | EqnGQDU71-Y-1 | 23 | 3 | 2753 |
| C-astra | EqnGQDU71-Y-2 | 23 | 3 | 2755 |

**引用が要る判定（AC-12）:** 引用、誤りを入れた工程、直さなかった工程は下に書く。

<!-- check-record: quotes-needed config=C-astra -->
| 構成 | 記事 | 観点 | 判定 | 種類 |
|---|---|---|---|---|
| C-astra | roNZ-rLxhUE-1 | format_choice | both | required-fail |
| C-astra | roNZ-rLxhUE-2 | format_choice | both | required-fail |
| C-astra | roNZ-rLxhUE-2 | redundancy | both | quality-1 |
| C-astra | 18tbwwPM91A-1 | structure | both | quality-1 |
| C-astra | 18tbwwPM91A-2 | structure | both | quality-1 |
| C-astra | leE02uqgaKg-1 | evidence | both | required-fail |
| C-astra | EqnGQDU71-Y-1 | evidence | literal | required-fail |
| C-astra | EqnGQDU71-Y-2 | evidence | literal | required-fail |

**製品の制約 1〜3（AC-13）:** 引用は下に書く。

<!-- check-record: constraints config=C-astra -->
| 構成 | 記事 | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|
| C-astra | roNZ-rLxhUE-1 | no | yes | no |
| C-astra | roNZ-rLxhUE-2 | no | yes | no |
| C-astra | 18tbwwPM91A-1 | no | yes | no |
| C-astra | 18tbwwPM91A-2 | no | yes | no |
| C-astra | zW1RBacSqjk-1 | no | yes | no |
| C-astra | zW1RBacSqjk-2 | no | yes | no |
| C-astra | leE02uqgaKg-1 | yes | no | no |
| C-astra | leE02uqgaKg-2 | no | yes | no |
| C-astra | EqnGQDU71-Y-1 | no | yes | yes |
| C-astra | EqnGQDU71-Y-2 | no | yes | yes |

**失敗した実行と番兵（AC-17）:** なし（10 件とも終了コード `0`）。
**人が変えた判定（AC-10）:**

- zW1RBacSqjk-2 の制約 1: LLM は yes / -、人の判定は no / -。「横開き」を「横に広い／横長」とした描写は「パスポートみたいなサイズ」から導けるので制約 1 なし（2 段目の確認の際に見直し、人が変えた）。

**引用（AC-12）:** 必須の観点の不合格と品質の 1 である。「入った」は誤りを入れた工程、「残した」は直さなかった工程である。
- roNZ-rLxhUE-1 形式の選択（fail / fail）: 「小泉が日本の安全保障を考えるうえで重視するのは、米国が…関与を減らしていく可能性だ」（字幕 21:21）。入った: 生成、残した: -（検証・推敲は形式を変えない）。
- roNZ-rLxhUE-2 形式の選択（fail / fail）: 「小泉は、米国のNATO脱退はあり得るとしつつ、まず懸念するのは同盟の形骸化だと指摘する」（字幕 17:55）。入った: 生成、残した: -（検証・推敲は形式を変えない）。
- roNZ-rLxhUE-2 冗長さ（1 / 1）: 「各機関が持つ能力を…集約できるようにすることに意義を見いだしている」（字幕 11:24）。入った: 生成、残した: 検証・推敲。
- 18tbwwPM91A-1 構成（1 / 1）: 「では、その力を打球に伝える際には何を意識していたのか」（字幕 12:15）。入った: 生成、残した: 検証・推敲。
- 18tbwwPM91A-2 構成（1 / 1）: 「## 「天才」という評価を生んだ意識と動きのずれ」（字幕 08:05）。入った: 生成、残した: 検証・推敲。
- leE02uqgaKg-1 根拠（fail / fail）: 「一方、地球や太陽ほどの質量になると、月や惑星の運動を左右する力となる」（字幕 02:13）。入った: 生成、残した: 検証・推敲。
- EqnGQDU71-Y-1 根拠（pass / fail）: 「展示の充実とともに大切にされているのは、学校教育や日々の暮らしの中でも利用できる」（字幕 06:21-07:00）。入った: 生成、残した: 検証・推敲。
- EqnGQDU71-Y-2 根拠（pass / fail）: 「新しい博物館の計画で大切にされているのは、「町民に愛される場所づくり」だ」（字幕 06:21-06:36）。入った: 生成、残した: 検証・推敲。

**製品の制約 1〜3 の引用（AC-13）:**
- roNZ-rLxhUE-1 制約 2: 「だからといって、小泉は日本の核保有を望ましい選択肢とは考えていない」（字幕 23:46）
- roNZ-rLxhUE-2 制約 2: 「小泉は、日本の核保有を望ましい選択肢とは考えていない」（字幕 23:46）
- 18tbwwPM91A-1 制約 2: 「では、その力を打球に伝える際には何を意識していたのか」（字幕 12:15）
- 18tbwwPM91A-2 制約 2: 「その違いがよく表れたのが、練習で打球に角度をつけるかどうかという話だった」（字幕 08:05）
- zW1RBacSqjk-1 制約 2: 「ここで整理しておきたいのが製品名である。」（字幕 00:44）
- zW1RBacSqjk-2 制約 2: 「製品名には注意が必要だ。」（字幕 00:44）
- leE02uqgaKg-1 制約 1: 「地球や太陽ほどの質量になると、月や惑星の運動を左右する」（字幕 02:13）
- leE02uqgaKg-2 制約 2: 「一般相対性理論の成果の一つが、重力によって光が曲がって見える現象の説明である」（字幕 13:20）
- EqnGQDU71-Y-1 制約 2: 「町のにぎわいへの期待もある。」（字幕 03:25）
- EqnGQDU71-Y-1 制約 3: 「目指すのは、観光客を呼び込むだけでなく、町民が繰り返し訪れ」（字幕 04:31-04:45）
- EqnGQDU71-Y-2 制約 2: 「町で恐竜を軸に活動する人の期待も大きい。」（字幕 03:25）
- EqnGQDU71-Y-2 制約 3: 「新しい博物館の計画で大切にされているのは、「町民に愛される場所づくり」だ」（字幕 06:21）

### 7.3. A-sol

**設定（AC-06）:** 生成と字数の調整は `claude/claude-opus-5-5+high`、検証・推敲は `openai/gpt-6.1-sol+medium` で、テンプレートは既定である。基準の構成の保存した出力から、検証・推敲だけを再実行する（設計 §3.2）。roNZ-rLxhUE-1 は `--after shorten`、残りの 9 件は `--after generation` で再開する。CLI のコミットとバイナリの SHA-256 は §2 に記録する（ステップ 1-6）。

**実行（AC-06）:** 工程ごとの保存した出力の識別子と、実行を始めた時刻である。

<!-- check-record: runs config=A-sol -->
| 構成 | 記事 | 回 | `--after` | `YT2COLUMN_LLM` | `YT2COLUMN_REVIEW_LLM` | 開始（UTC） | 終了（UTC） | 終了コード | 生成の識別子 | 字数の調整の識別子 | 検証・推敲の識別子 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| A-sol | roNZ-rLxhUE-1 | cur | shorten | claude/claude-opus-5-5+high | openai/gpt-6.1-sol+medium | 2026-11-12T06:49:45Z | 2026-11-12T06:50:08Z | 0 | 03c1ede250eb2c3c47c5852517591a118a9a538bf6dbbed6e1f4f5a0b878d8a2 | 20b0f8a8b5cf47e68aa6bd7f4b5a125a23cf275b34e7b1e850f84d0075d5cf23 | 9e4f68dce0a7642c9a2d77220a5f6a2b6ffabd5f10e9b8ad0228a738ce0c4d7f |
| A-sol | roNZ-rLxhUE-2 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6.1-sol+medium | 2026-11-12T06:49:45Z | 2026-11-12T06:50:06Z | 0 | 5bda51c848e127581a5de52775a303e88aaab1764a2eb8490a880ea34ae44590 | - | c0ff520a41c0cdea5fa234fa4ab9b06c739cf6c20e2d767f9e71f99893cd9af2 |
| A-sol | 18tbwwPM91A-1 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6.1-sol+medium | 2026-11-12T06:49:45Z | 2026-11-12T06:50:05Z | 0 | 1e3318a27b36f02dd18aca1ddeb8b5b9a0fdaf0f7a5fe7f805bf424d5692180c | - | f243538a753f2e44880392ee31e59142c6e611f6e35cdd618338334e13b0dfa3 |
| A-sol | 18tbwwPM91A-2 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6.1-sol+medium | 2026-11-12T06:49:45Z | 2026-11-12T06:50:08Z | 0 | e5fed9a5b44e405ea9118cdaad6c906f97ec3e742dba756298bfaa42f979acfd | - | 97ae484d9bd18f5e0ec493a73419015557253aaf02575bf87b53d066864cb135 |
| A-sol | zW1RBacSqjk-1 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6.1-sol+medium | 2026-11-12T06:49:45Z | 2026-11-12T06:49:57Z | 0 | 91df9b370bfebe180a757d97cd616cd6a61be53f4b386b3878ad1379db34d1c0 | - | 3dfed833bebc34ec9973a43cb2d95cfbaca847dfd3555174a462880aabeea343 |
| A-sol | zW1RBacSqjk-2 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6.1-sol+medium | 2026-11-12T06:49:45Z | 2026-11-12T06:49:58Z | 0 | 84a90816f744317836154fd3291ad9900385af7d4a86b2a342cef425f6651371 | - | 6c1a83ebc2905f78d7acd3ff92f022a39c258de89da993e9d880207118280ae8 |
| A-sol | leE02uqgaKg-1 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6.1-sol+medium | 2026-11-12T06:49:45Z | 2026-11-12T06:49:53Z | 0 | 433784761850152cb0a2bd731486b78ae2c6ced7f8ddde03254610efdeb921da | - | 3c5999215fef189e924209adace17ebf10143a88d4fe134ade35cab38006e8e7 |
| A-sol | leE02uqgaKg-2 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6.1-sol+medium | 2026-11-12T06:49:45Z | 2026-11-12T06:49:59Z | 0 | 62e7949eaf11428fb1f882ac95d4b0117d461775758d0c62287c1992c3fe3156 | - | 81055843a5db6dcea6f022130df72362d48735aacbfac0eaa05e61b8e346f5ef |
| A-sol | EqnGQDU71-Y-1 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6.1-sol+medium | 2026-11-12T06:49:45Z | 2026-11-12T06:49:55Z | 0 | b4a117332422b30bf1ddb6d2d45e6bbbe10a7536c25b833b95e398ead8f06f84 | - | ae050e8736ee1c8724e527312f91f8c44659e0bd467ee9c076c9932a0c2196d6 |
| A-sol | EqnGQDU71-Y-2 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6.1-sol+medium | 2026-11-12T06:49:45Z | 2026-11-12T06:49:53Z | 0 | 5c6d599b6e2401b64cbf8844de7c550848455724bb85401d2b28b7e183d1c4f2 | - | 64fd55e8dba55b01a3524e9d109d5d5e210ca22c0ccdeacf0457cca305dbce7f |

**再開と識別子（AC-03）:** 再利用した保存した出力の識別子が基準の構成と一致すること、字数の調整を実行していないことである。

<!-- check-record: resume-identity config=A-sol -->
| 構成 | 記事 | `--after` | 生成の一致 | 字数の調整の一致 | 字数の調整を実行 | 手順から外れた |
|---|---|---|---|---|---|---|
| A-sol | roNZ-rLxhUE-1 | shorten | yes | yes | no | no |
| A-sol | roNZ-rLxhUE-2 | generation | yes | - | no | no |
| A-sol | 18tbwwPM91A-1 | generation | yes | - | no | no |
| A-sol | 18tbwwPM91A-2 | generation | yes | - | no | no |
| A-sol | zW1RBacSqjk-1 | generation | yes | - | no | no |
| A-sol | zW1RBacSqjk-2 | generation | yes | - | no | no |
| A-sol | leE02uqgaKg-1 | generation | yes | - | no | no |
| A-sol | leE02uqgaKg-2 | generation | yes | - | no | no |
| A-sol | EqnGQDU71-Y-1 | generation | yes | - | no | no |
| A-sol | EqnGQDU71-Y-2 | generation | yes | - | no | no |

**トークン数と推定コスト（AC-14）:** 工程ごとの CLI の値である。「再利用した実行」は、再開で呼ばなかった工程の値の出どころ（B3 の記事）を示す。

<!-- check-record: stage-usage config=A-sol -->
| 構成 | 記事 | 回 | 工程 | 出どころ | 使用量 | 入力 | キャッシュから読んだ入力 | キャッシュに書いた入力 | 出力 | モデル | 推定コスト（USD） | 不明の理由 | 再利用した実行 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| A-sol | roNZ-rLxhUE-1 | cur | generation | reused | known | 21451 | 0 | 0 | 10986 | claude-opus-5-5 | 0.305524 | - | B3:roNZ-rLxhUE-1 |
| A-sol | roNZ-rLxhUE-1 | cur | shorten | reused | known | 4663 | 0 | 0 | 7371 | claude-opus-5-5 | 0.166072 | - | B3:roNZ-rLxhUE-1 |
| A-sol | roNZ-rLxhUE-1 | cur | review | run | known | 3 | 0 | 15213 | 1207 | gpt-6.1-sol | 0.050109 | - | - |
| A-sol | roNZ-rLxhUE-2 | cur | generation | reused | known | 21451 | 0 | 0 | 11122 | claude-opus-5-5 | 0.308244 | - | B3:roNZ-rLxhUE-2 |
| A-sol | roNZ-rLxhUE-2 | cur | review | run | known | 3 | 0 | 15101 | 951 | gpt-6.1-sol | 0.047269 | - | - |
| A-sol | 18tbwwPM91A-1 | cur | generation | reused | known | 16528 | 0 | 0 | 13688 | claude-opus-5-5 | 0.339872 | - | B3:18tbwwPM91A-1 |
| A-sol | 18tbwwPM91A-1 | cur | review | run | known | 3 | 0 | 11017 | 967 | gpt-6.1-sol | 0.037219 | - | - |
| A-sol | 18tbwwPM91A-2 | cur | generation | reused | known | 16528 | 0 | 0 | 13442 | claude-opus-5-5 | 0.334952 | - | B3:18tbwwPM91A-2 |
| A-sol | 18tbwwPM91A-2 | cur | review | run | known | 3 | 0 | 11264 | 1150 | gpt-6.1-sol | 0.039666 | - | - |
| A-sol | zW1RBacSqjk-1 | cur | generation | reused | known | 14937 | 0 | 0 | 7966 | claude-opus-5-5 | 0.219068 | - | B3:zW1RBacSqjk-1 |
| A-sol | zW1RBacSqjk-1 | cur | review | run | known | 3 | 0 | 9298 | 437 | gpt-6.1-sol | 0.027621 | - | - |
| A-sol | zW1RBacSqjk-2 | cur | generation | reused | known | 14937 | 0 | 0 | 7739 | claude-opus-5-5 | 0.214528 | - | B3:zW1RBacSqjk-2 |
| A-sol | zW1RBacSqjk-2 | cur | review | run | known | 3 | 0 | 9398 | 443 | gpt-6.1-sol | 0.027931 | - | - |
| A-sol | leE02uqgaKg-1 | cur | generation | reused | known | 15041 | 0 | 0 | 7712 | claude-opus-5-5 | 0.214404 | - | B3:leE02uqgaKg-1 |
| A-sol | leE02uqgaKg-1 | cur | review | run | known | 3 | 0 | 10287 | 134 | gpt-6.1-sol | 0.027064 | - | - |
| A-sol | leE02uqgaKg-2 | cur | generation | reused | known | 15041 | 0 | 0 | 5623 | claude-opus-5-5 | 0.172624 | - | B3:leE02uqgaKg-2 |
| A-sol | leE02uqgaKg-2 | cur | review | run | known | 3 | 0 | 10193 | 578 | gpt-6.1-sol | 0.031269 | - | - |
| A-sol | EqnGQDU71-Y-1 | cur | generation | reused | known | 11377 | 0 | 0 | 5030 | claude-opus-5-5 | 0.146108 | - | B3:EqnGQDU71-Y-1 |
| A-sol | EqnGQDU71-Y-1 | cur | review | run | known | 3 | 0 | 6430 | 453 | gpt-6.1-sol | 0.020611 | - | - |
| A-sol | EqnGQDU71-Y-2 | cur | generation | reused | known | 11377 | 0 | 0 | 5114 | claude-opus-5-5 | 0.147788 | - | B3:EqnGQDU71-Y-2 |
| A-sol | EqnGQDU71-Y-2 | cur | review | run | known | 3 | 0 | 6422 | 369 | gpt-6.1-sol | 0.019751 | - | - |

**記事ごとの推定コストと所要時間（AC-14・AC-16）:** 所要時間は記事を作った工程の時間の和で、10 件を並列に起動して測った値である（基準の構成と同じ。0018 評価記録 §4.8）。構成 A は B3 の 1 段目の生成の時間を足し、roNZ-rLxhUE-1 には B3 のやり直しの字数の調整の時間も足す。

<!-- check-record: run-costs config=A-sol -->
| 構成 | 記事 | 回 | 終了コード | この実行（USD） | 記事（USD） | 実行の時間 | 所要時間 | 比較に使う |
|---|---|---|---|---|---|---|---|---|
| A-sol | roNZ-rLxhUE-1 | cur | 0 | 0.050109 | 0.521705 | 0:23.0 | 3:03.7 | yes |
| A-sol | roNZ-rLxhUE-2 | cur | 0 | 0.047269 | 0.355513 | 0:21.3 | 2:07.4 | yes |
| A-sol | 18tbwwPM91A-1 | cur | 0 | 0.037219 | 0.377091 | 0:20.1 | 2:35.5 | yes |
| A-sol | 18tbwwPM91A-2 | cur | 0 | 0.039666 | 0.374618 | 0:23.1 | 2:36.2 | yes |
| A-sol | zW1RBacSqjk-1 | cur | 0 | 0.027621 | 0.246689 | 0:11.9 | 1:27.3 | yes |
| A-sol | zW1RBacSqjk-2 | cur | 0 | 0.027931 | 0.242459 | 0:12.8 | 1:24.8 | yes |
| A-sol | leE02uqgaKg-1 | cur | 0 | 0.027064 | 0.241468 | 0:07.8 | 1:19.3 | yes |
| A-sol | leE02uqgaKg-2 | cur | 0 | 0.031269 | 0.203893 | 0:14.0 | 1:07.0 | yes |
| A-sol | EqnGQDU71-Y-1 | cur | 0 | 0.020611 | 0.166719 | 0:09.7 | 0:58.9 | yes |
| A-sol | EqnGQDU71-Y-2 | cur | 0 | 0.019751 | 0.167539 | 0:07.9 | 0:57.6 | yes |

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
| A-sol | roNZ-rLxhUE-1 | pass/pass | pass/pass | pass/fail | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 2/2 | yes | no |
| A-sol | roNZ-rLxhUE-2 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 2/2 | 3/3 | 1/1 | 3/3 | 2/2 | no | no |
| A-sol | 18tbwwPM91A-1 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 2/2 | 2/2 | 2/2 | 3/3 | 3/3 | 3/3 | yes | yes |
| A-sol | 18tbwwPM91A-2 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 2/2 | 1/1 | 3/3 | 3/3 | 3/3 | no | no |
| A-sol | zW1RBacSqjk-1 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 3/3 | yes | yes |
| A-sol | zW1RBacSqjk-2 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 2/2 | 3/3 | 3/3 | yes | yes |
| A-sol | leE02uqgaKg-1 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 2/2 | 3/3 | 3/3 | 3/3 | 2/2 | yes | yes |
| A-sol | leE02uqgaKg-2 | pass/pass | pass/pass | fail/fail | pass/pass | pass/pass | 3/3 | 2/2 | 3/3 | 3/3 | 3/3 | 3/3 | no | no |
| A-sol | EqnGQDU71-Y-1 | pass/pass | pass/pass | pass/fail | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 3/3 | yes | no |
| A-sol | EqnGQDU71-Y-2 | pass/pass | pass/pass | pass/fail | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 3/3 | yes | no |

**判定者（AC-10）:** `script` はスクリプト、`llm` は人の確認の対象の外の LLM の判定、`llm+human` は人の確認の対象で人が変えなかった LLM の判定（一括で確認した判定を含む。§7 の冒頭）、`human` は人が変えた判定である。人が変えた判定と理由は下に書く。

<!-- check-record: judges config=A-sol -->
| 構成 | 記事 | 形式の選択 | 形式の規則 | 根拠 | 長さ | 表記 | 網羅 | 重要度 | 構成（観点） | 読みやすさ | タイトル・リード文 | 冗長さ | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| A-sol | roNZ-rLxhUE-1 | llm | llm | human | script | llm | llm | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| A-sol | roNZ-rLxhUE-2 | llm | llm | llm+human | script | llm | llm | llm+human | llm | llm+human | llm | llm+human | llm+human | llm+human | llm+human |
| A-sol | 18tbwwPM91A-1 | llm | llm | llm+human | script | llm | llm+human | llm+human | llm+human | llm | llm | llm | llm+human | llm+human | llm+human |
| A-sol | 18tbwwPM91A-2 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm | llm+human | llm+human | llm+human |
| A-sol | zW1RBacSqjk-1 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm | human | llm+human | llm+human |
| A-sol | zW1RBacSqjk-2 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human |
| A-sol | leE02uqgaKg-1 | llm | llm | llm+human | script | llm | llm | llm+human | llm | llm | llm | llm+human | llm+human | llm+human | llm+human |
| A-sol | leE02uqgaKg-2 | llm | llm | llm+human | script | llm | llm | llm+human | llm | llm | llm | llm | llm+human | llm+human | llm+human |
| A-sol | EqnGQDU71-Y-1 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm | llm+human | llm+human | llm+human |
| A-sol | EqnGQDU71-Y-2 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm | llm+human | llm+human | llm+human |

**機械的な値（AC-09）:** `check_article.py` の出力である。

<!-- check-record: article-checks config=A-sol -->
| 構成 | 記事 | タイトルの字数 | 節の数 | 長さ |
|---|---|---|---|---|
| A-sol | roNZ-rLxhUE-1 | 23 | 5 | 3735 |
| A-sol | roNZ-rLxhUE-2 | 26 | 5 | 3603 |
| A-sol | 18tbwwPM91A-1 | 28 | 5 | 3392 |
| A-sol | 18tbwwPM91A-2 | 27 | 5 | 3731 |
| A-sol | zW1RBacSqjk-1 | 38 | 4 | 3454 |
| A-sol | zW1RBacSqjk-2 | 38 | 5 | 3642 |
| A-sol | leE02uqgaKg-1 | 26 | 5 | 3548 |
| A-sol | leE02uqgaKg-2 | 31 | 5 | 3399 |
| A-sol | EqnGQDU71-Y-1 | 30 | 4 | 2331 |
| A-sol | EqnGQDU71-Y-2 | 28 | 4 | 2336 |

**引用が要る判定（AC-12）:** 引用、誤りを入れた工程、直さなかった工程は下に書く。

<!-- check-record: quotes-needed config=A-sol -->
| 構成 | 記事 | 観点 | 判定 | 種類 |
|---|---|---|---|---|
| A-sol | roNZ-rLxhUE-1 | evidence | literal | required-fail |
| A-sol | roNZ-rLxhUE-2 | readability | both | quality-1 |
| A-sol | 18tbwwPM91A-2 | structure | both | quality-1 |
| A-sol | leE02uqgaKg-2 | evidence | both | required-fail |
| A-sol | EqnGQDU71-Y-1 | evidence | literal | required-fail |
| A-sol | EqnGQDU71-Y-2 | evidence | literal | required-fail |

**製品の制約 1〜3（AC-13）:** 引用は下に書く。

<!-- check-record: constraints config=A-sol -->
| 構成 | 記事 | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|
| A-sol | roNZ-rLxhUE-1 | no | no | no |
| A-sol | roNZ-rLxhUE-2 | no | no | no |
| A-sol | 18tbwwPM91A-1 | no | no | no |
| A-sol | 18tbwwPM91A-2 | no | yes | no |
| A-sol | zW1RBacSqjk-1 | no | yes | no |
| A-sol | zW1RBacSqjk-2 | no | yes | no |
| A-sol | leE02uqgaKg-1 | no | no | no |
| A-sol | leE02uqgaKg-2 | yes | no | no |
| A-sol | EqnGQDU71-Y-1 | no | yes | yes |
| A-sol | EqnGQDU71-Y-2 | no | yes | yes |

**失敗した実行と番兵（AC-17）:** なし（10 件とも終了コード `0`）。
**人が変えた判定（AC-10）:**

- roNZ-rLxhUE-1 の根拠: LLM は pass / pass、人の判定は pass / fail。英伊の動機の推測を日本の理由とした書き方は文言どおりでは不合格（人が変えた）。
- zW1RBacSqjk-1 の制約 1: LLM は yes / -、人の判定は no / -。「横開き」を「横に広い／横長」とした描写は「パスポートみたいなサイズ」から導けるので制約 1 なし（2 段目の確認の際に見直し、人が変えた）。

**引用（AC-12）:** 必須の観点の不合格と品質の 1 である。「入った」は誤りを入れた工程、「残した」は直さなかった工程である。
- roNZ-rLxhUE-1 根拠（pass / fail）: 「重要な軍事技術を米国に依存し続けるべきでないという考えからでしょう」（字幕 06:42）。入った: 字数の調整（B3 の字数の調整の出力の文）、残した: 検証・推敲。
- roNZ-rLxhUE-2 読みやすさ（1 / 1）: 「## 徴兵制ではなく「ロボットが自衛隊」を」（字幕 31:03）。入った: 生成、残した: 検証・推敲。
- 18tbwwPM91A-2 構成（1 / 1）: 「一方、データにはレフト方向への強い打球も残っている」（字幕 13:45）。入った: 生成、残した: 検証・推敲。
- leE02uqgaKg-2 根拠（fail / fail）: 「一方、地球ほどの質量になると重力は強大で、月をつなぎとめる」（字幕 02:13）。入った: 生成、残した: 検証・推敲。
- EqnGQDU71-Y-1 根拠（pass / fail）: 「新博物館は…それ以上に「町民に愛される場所」になることを大切にしている」（字幕 06:21-06:36）。入った: 生成、残した: 検証・推敲。
- EqnGQDU71-Y-2 根拠（pass / fail）: 「いま関係者が何より大切にしているのは、観光客を呼ぶことに加えて」（字幕 06:21-06:36）。入った: 生成、残した: 検証・推敲。

**製品の制約 1〜3 の引用（AC-13）:**
- 18tbwwPM91A-2 制約 2: 「一方、データにはレフト方向への強い打球も残っている」（字幕 13:45）
- zW1RBacSqjk-1 制約 2: 「名前の付き方には注意が要る。」（字幕 00:44）
- zW1RBacSqjk-2 制約 2: 「価格は、ストレージ256GB・メモリー12GBのモデルが19万2680円から」（字幕 12:27）
- leE02uqgaKg-2 制約 1: 「一方、地球ほどの質量になると重力は強大で」（字幕 02:13）
- EqnGQDU71-Y-1 制約 2: 「むかわ竜の化石は、2003年4月、当時郵便局員だった堀田良幸」（字幕 00:35）
- EqnGQDU71-Y-1 制約 3: 「新博物館は…それ以上に「町民に愛される場所」になることを大切に」（字幕 06:21）
- EqnGQDU71-Y-2 制約 2: 「学校教育ではこれまで以上に博物館を活用してほしいと話す」（字幕 06:43）
- EqnGQDU71-Y-2 制約 3: 「いま関係者が何より大切にしているのは」（字幕 06:21-06:36）

### 7.4. A-astra

**設定（AC-06）:** 生成と字数の調整は `claude/claude-opus-5-5+high`、検証・推敲は `openai/gpt-6-astra+medium` で、テンプレートは既定である。基準の構成の保存した出力から、検証・推敲だけを再実行する（設計 §3.2）。roNZ-rLxhUE-1 は `--after shorten`、残りの 9 件は `--after generation` で再開する。CLI のコミットとバイナリの SHA-256 は §2 に記録する（ステップ 1-6）。

**実行（AC-06）:** 工程ごとの保存した出力の識別子と、実行を始めた時刻である。

<!-- check-record: runs config=A-astra -->
| 構成 | 記事 | 回 | `--after` | `YT2COLUMN_LLM` | `YT2COLUMN_REVIEW_LLM` | 開始（UTC） | 終了（UTC） | 終了コード | 生成の識別子 | 字数の調整の識別子 | 検証・推敲の識別子 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| A-astra | roNZ-rLxhUE-1 | cur | shorten | claude/claude-opus-5-5+high | openai/gpt-6-astra+medium | 2026-11-12T06:50:44Z | 2026-11-12T06:51:08Z | 0 | 03c1ede250eb2c3c47c5852517591a118a9a538bf6dbbed6e1f4f5a0b878d8a2 | 20b0f8a8b5cf47e68aa6bd7f4b5a125a23cf275b34e7b1e850f84d0075d5cf23 | 7982b0fbff08585b21afc347dbb94700cb06c6a0cc417f90575499fde516b77b |
| A-astra | roNZ-rLxhUE-2 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6-astra+medium | 2026-11-12T06:50:44Z | 2026-11-12T06:51:20Z | 0 | 5bda51c848e127581a5de52775a303e88aaab1764a2eb8490a880ea34ae44590 | - | af7828635c9d1456116517f8fb33d27e778ca25b0201435a8f25743b6992e0a3 |
| A-astra | 18tbwwPM91A-1 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6-astra+medium | 2026-11-12T06:50:44Z | 2026-11-12T06:51:09Z | 0 | 1e3318a27b36f02dd18aca1ddeb8b5b9a0fdaf0f7a5fe7f805bf424d5692180c | - | 6c53acff43a1539be537a248d6dad5c30bb871e4ca8f042bb51c6429ee3de7d7 |
| A-astra | 18tbwwPM91A-2 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6-astra+medium | 2026-11-12T06:50:44Z | 2026-11-12T06:51:05Z | 0 | e5fed9a5b44e405ea9118cdaad6c906f97ec3e742dba756298bfaa42f979acfd | - | ffce4621954485bd6770cf0dd8b1f648a22908f2395115aa2565aedf55e5b5f6 |
| A-astra | zW1RBacSqjk-1 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6-astra+medium | 2026-11-12T06:50:44Z | 2026-11-12T06:50:54Z | 0 | 91df9b370bfebe180a757d97cd616cd6a61be53f4b386b3878ad1379db34d1c0 | - | 25bfd32a947f37a811eb44c6be8987ff1c82fc6698aacebf5aef9b346f059187 |
| A-astra | zW1RBacSqjk-2 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6-astra+medium | 2026-11-12T06:50:44Z | 2026-11-12T06:51:02Z | 0 | 84a90816f744317836154fd3291ad9900385af7d4a86b2a342cef425f6651371 | - | 81f6f01659368e07be4db70c6aaaff6a3fc74ace2e9fd70823f0afd1ccf334bd |
| A-astra | leE02uqgaKg-1 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6-astra+medium | 2026-11-12T06:50:44Z | 2026-11-12T06:50:49Z | 0 | 433784761850152cb0a2bd731486b78ae2c6ced7f8ddde03254610efdeb921da | - | 45827b6b15c24e7ed475eaa514465d846fde953c97029299116e186fa19f277b |
| A-astra | leE02uqgaKg-2 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6-astra+medium | 2026-11-12T06:50:44Z | 2026-11-12T06:50:53Z | 0 | 62e7949eaf11428fb1f882ac95d4b0117d461775758d0c62287c1992c3fe3156 | - | 91b5f354a58c8871f8efa0198a9ee3f63c4d61bf2e7b9fad740c2e139e5a2309 |
| A-astra | EqnGQDU71-Y-1 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6-astra+medium | 2026-11-12T06:50:44Z | 2026-11-12T06:50:52Z | 0 | b4a117332422b30bf1ddb6d2d45e6bbbe10a7536c25b833b95e398ead8f06f84 | - | 906db236048b67cd5d504f7f78d2c52086756803f7ab71625afc522d80c9590f |
| A-astra | EqnGQDU71-Y-2 | cur | generation | claude/claude-opus-5-5+high | openai/gpt-6-astra+medium | 2026-11-12T06:50:44Z | 2026-11-12T06:50:49Z | 0 | 5c6d599b6e2401b64cbf8844de7c550848455724bb85401d2b28b7e183d1c4f2 | - | ef28c3625c9dcd4e48184237fa95bbd5ed98af3464135ce034eda65c4fe5ca3b |

**再開と識別子（AC-03）:** 再利用した保存した出力の識別子が基準の構成と一致すること、字数の調整を実行していないことである。

<!-- check-record: resume-identity config=A-astra -->
| 構成 | 記事 | `--after` | 生成の一致 | 字数の調整の一致 | 字数の調整を実行 | 手順から外れた |
|---|---|---|---|---|---|---|
| A-astra | roNZ-rLxhUE-1 | shorten | yes | yes | no | no |
| A-astra | roNZ-rLxhUE-2 | generation | yes | - | no | no |
| A-astra | 18tbwwPM91A-1 | generation | yes | - | no | no |
| A-astra | 18tbwwPM91A-2 | generation | yes | - | no | no |
| A-astra | zW1RBacSqjk-1 | generation | yes | - | no | no |
| A-astra | zW1RBacSqjk-2 | generation | yes | - | no | no |
| A-astra | leE02uqgaKg-1 | generation | yes | - | no | no |
| A-astra | leE02uqgaKg-2 | generation | yes | - | no | no |
| A-astra | EqnGQDU71-Y-1 | generation | yes | - | no | no |
| A-astra | EqnGQDU71-Y-2 | generation | yes | - | no | no |

**トークン数と推定コスト（AC-14）:** 工程ごとの CLI の値である。「再利用した実行」は、再開で呼ばなかった工程の値の出どころ（B3 の記事）を示す。

<!-- check-record: stage-usage config=A-astra -->
| 構成 | 記事 | 回 | 工程 | 出どころ | 使用量 | 入力 | キャッシュから読んだ入力 | キャッシュに書いた入力 | 出力 | モデル | 推定コスト（USD） | 不明の理由 | 再利用した実行 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| A-astra | roNZ-rLxhUE-1 | cur | generation | reused | known | 21451 | 0 | 0 | 10986 | claude-opus-5-5 | 0.305524 | - | B3:roNZ-rLxhUE-1 |
| A-astra | roNZ-rLxhUE-1 | cur | shorten | reused | known | 4663 | 0 | 0 | 7371 | claude-opus-5-5 | 0.166072 | - | B3:roNZ-rLxhUE-1 |
| A-astra | roNZ-rLxhUE-1 | cur | review | run | known | 3 | 0 | 15213 | 1307 | gpt-6-astra | 0.255543 | - | - |
| A-astra | roNZ-rLxhUE-2 | cur | generation | reused | known | 21451 | 0 | 0 | 11122 | claude-opus-5-5 | 0.308244 | - | B3:roNZ-rLxhUE-2 |
| A-astra | roNZ-rLxhUE-2 | cur | review | run | known | 3 | 0 | 15101 | 1303 | gpt-6-astra | 0.253943 | - | - |
| A-astra | 18tbwwPM91A-1 | cur | generation | reused | known | 16528 | 0 | 0 | 13688 | claude-opus-5-5 | 0.339872 | - | B3:18tbwwPM91A-1 |
| A-astra | 18tbwwPM91A-1 | cur | review | run | known | 3 | 0 | 11017 | 1269 | gpt-6-astra | 0.201193 | - | - |
| A-astra | 18tbwwPM91A-2 | cur | generation | reused | known | 16528 | 0 | 0 | 13442 | claude-opus-5-5 | 0.334952 | - | B3:18tbwwPM91A-2 |
| A-astra | 18tbwwPM91A-2 | cur | review | run | known | 3 | 0 | 11264 | 910 | gpt-6-astra | 0.186330 | - | - |
| A-astra | zW1RBacSqjk-1 | cur | generation | reused | known | 14937 | 0 | 0 | 7966 | claude-opus-5-5 | 0.219068 | - | B3:zW1RBacSqjk-1 |
| A-astra | zW1RBacSqjk-1 | cur | review | run | known | 3 | 0 | 9298 | 392 | gpt-6-astra | 0.135855 | - | - |
| A-astra | zW1RBacSqjk-2 | cur | generation | reused | known | 14937 | 0 | 0 | 7739 | claude-opus-5-5 | 0.214528 | - | B3:zW1RBacSqjk-2 |
| A-astra | zW1RBacSqjk-2 | cur | review | run | known | 3 | 0 | 9398 | 792 | gpt-6-astra | 0.157105 | - | - |
| A-astra | leE02uqgaKg-1 | cur | generation | reused | known | 15041 | 0 | 0 | 7712 | claude-opus-5-5 | 0.214404 | - | B3:leE02uqgaKg-1 |
| A-astra | leE02uqgaKg-1 | cur | review | run | known | 3 | 0 | 10287 | 80 | gpt-6-astra | 0.132618 | - | - |
| A-astra | leE02uqgaKg-2 | cur | generation | reused | known | 15041 | 0 | 0 | 5623 | claude-opus-5-5 | 0.172624 | - | B3:leE02uqgaKg-2 |
| A-astra | leE02uqgaKg-2 | cur | review | run | known | 3 | 0 | 10193 | 386 | gpt-6-astra | 0.146743 | - | - |
| A-astra | EqnGQDU71-Y-1 | cur | generation | reused | known | 11377 | 0 | 0 | 5030 | claude-opus-5-5 | 0.146108 | - | B3:EqnGQDU71-Y-1 |
| A-astra | EqnGQDU71-Y-1 | cur | review | run | known | 3 | 0 | 6430 | 417 | gpt-6-astra | 0.101255 | - | - |
| A-astra | EqnGQDU71-Y-2 | cur | generation | reused | known | 11377 | 0 | 0 | 5114 | claude-opus-5-5 | 0.147788 | - | B3:EqnGQDU71-Y-2 |
| A-astra | EqnGQDU71-Y-2 | cur | review | run | known | 3 | 0 | 6422 | 314 | gpt-6-astra | 0.096005 | - | - |

**記事ごとの推定コストと所要時間（AC-14・AC-16）:** 所要時間は記事を作った工程の時間の和で、10 件を並列に起動して測った値である（基準の構成と同じ。0018 評価記録 §4.8）。構成 A は B3 の 1 段目の生成の時間を足し、roNZ-rLxhUE-1 には B3 のやり直しの字数の調整の時間も足す。

<!-- check-record: run-costs config=A-astra -->
| 構成 | 記事 | 回 | 終了コード | この実行（USD） | 記事（USD） | 実行の時間 | 所要時間 | 比較に使う |
|---|---|---|---|---|---|---|---|---|
| A-astra | roNZ-rLxhUE-1 | cur | 0 | 0.255543 | 0.727139 | 0:23.7 | 3:04.4 | yes |
| A-astra | roNZ-rLxhUE-2 | cur | 0 | 0.253943 | 0.562187 | 0:35.4 | 2:21.4 | yes |
| A-astra | 18tbwwPM91A-1 | cur | 0 | 0.201193 | 0.541065 | 0:24.9 | 2:40.4 | yes |
| A-astra | 18tbwwPM91A-2 | cur | 0 | 0.186330 | 0.521282 | 0:20.4 | 2:33.5 | yes |
| A-astra | zW1RBacSqjk-1 | cur | 0 | 0.135855 | 0.354923 | 0:09.8 | 1:25.2 | yes |
| A-astra | zW1RBacSqjk-2 | cur | 0 | 0.157105 | 0.371633 | 0:18.1 | 1:30.1 | yes |
| A-astra | leE02uqgaKg-1 | cur | 0 | 0.132618 | 0.347022 | 0:04.6 | 1:16.1 | yes |
| A-astra | leE02uqgaKg-2 | cur | 0 | 0.146743 | 0.319367 | 0:08.8 | 1:01.7 | yes |
| A-astra | EqnGQDU71-Y-1 | cur | 0 | 0.101255 | 0.247363 | 0:08.1 | 0:57.4 | yes |
| A-astra | EqnGQDU71-Y-2 | cur | 0 | 0.096005 | 0.243793 | 0:04.9 | 0:54.6 | yes |

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
| A-astra | roNZ-rLxhUE-1 | pass/pass | pass/pass | pass/fail | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 2/2 | yes | no |
| A-astra | roNZ-rLxhUE-2 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 2/2 | 3/3 | 1/1 | 3/3 | 2/2 | no | no |
| A-astra | 18tbwwPM91A-1 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 2/2 | 2/2 | 2/2 | 3/3 | 3/3 | 3/3 | yes | yes |
| A-astra | 18tbwwPM91A-2 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 2/2 | 1/1 | 3/3 | 3/3 | 3/3 | no | no |
| A-astra | zW1RBacSqjk-1 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 3/3 | yes | yes |
| A-astra | zW1RBacSqjk-2 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 2/2 | 3/3 | 3/3 | yes | yes |
| A-astra | leE02uqgaKg-1 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 2/2 | 3/3 | 3/3 | 3/3 | 2/2 | yes | yes |
| A-astra | leE02uqgaKg-2 | pass/pass | pass/pass | fail/fail | pass/pass | pass/pass | 3/3 | 2/2 | 3/3 | 3/3 | 3/3 | 3/3 | no | no |
| A-astra | EqnGQDU71-Y-1 | pass/pass | pass/pass | pass/fail | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 3/3 | yes | no |
| A-astra | EqnGQDU71-Y-2 | pass/pass | pass/pass | pass/fail | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 3/3 | yes | no |

**判定者（AC-10）:** `script` はスクリプト、`llm` は人の確認の対象の外の LLM の判定、`llm+human` は人の確認の対象で人が変えなかった LLM の判定（一括で確認した判定を含む。§7 の冒頭）、`human` は人が変えた判定である。人が変えた判定と理由は下に書く。

<!-- check-record: judges config=A-astra -->
| 構成 | 記事 | 形式の選択 | 形式の規則 | 根拠 | 長さ | 表記 | 網羅 | 重要度 | 構成（観点） | 読みやすさ | タイトル・リード文 | 冗長さ | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| A-astra | roNZ-rLxhUE-1 | llm | llm | human | script | llm | llm | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| A-astra | roNZ-rLxhUE-2 | llm | llm | llm+human | script | llm | llm | llm+human | llm | llm+human | llm | llm+human | llm+human | llm+human | llm+human |
| A-astra | 18tbwwPM91A-1 | llm | llm | llm+human | script | llm | llm+human | llm+human | llm+human | llm | llm | llm | llm+human | llm+human | llm+human |
| A-astra | 18tbwwPM91A-2 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm | llm+human | llm+human | llm+human |
| A-astra | zW1RBacSqjk-1 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm | human | llm+human | llm+human |
| A-astra | zW1RBacSqjk-2 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human |
| A-astra | leE02uqgaKg-1 | llm | llm | llm+human | script | llm | llm | llm+human | llm | llm | llm | llm+human | llm+human | llm+human | llm+human |
| A-astra | leE02uqgaKg-2 | llm | llm | llm+human | script | llm | llm | llm+human | llm | llm | llm | llm | llm+human | llm+human | llm+human |
| A-astra | EqnGQDU71-Y-1 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm | llm+human | llm+human | llm+human |
| A-astra | EqnGQDU71-Y-2 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm | llm+human | llm+human | llm+human |

**機械的な値（AC-09）:** `check_article.py` の出力である。

<!-- check-record: article-checks config=A-astra -->
| 構成 | 記事 | タイトルの字数 | 節の数 | 長さ |
|---|---|---|---|---|
| A-astra | roNZ-rLxhUE-1 | 23 | 5 | 3722 |
| A-astra | roNZ-rLxhUE-2 | 26 | 5 | 3609 |
| A-astra | 18tbwwPM91A-1 | 28 | 5 | 3382 |
| A-astra | 18tbwwPM91A-2 | 27 | 5 | 3735 |
| A-astra | zW1RBacSqjk-1 | 38 | 4 | 3446 |
| A-astra | zW1RBacSqjk-2 | 38 | 5 | 3661 |
| A-astra | leE02uqgaKg-1 | 26 | 5 | 3548 |
| A-astra | leE02uqgaKg-2 | 31 | 5 | 3404 |
| A-astra | EqnGQDU71-Y-1 | 30 | 4 | 2338 |
| A-astra | EqnGQDU71-Y-2 | 28 | 4 | 2336 |

**引用が要る判定（AC-12）:** 引用、誤りを入れた工程、直さなかった工程は下に書く。

<!-- check-record: quotes-needed config=A-astra -->
| 構成 | 記事 | 観点 | 判定 | 種類 |
|---|---|---|---|---|
| A-astra | roNZ-rLxhUE-1 | evidence | literal | required-fail |
| A-astra | roNZ-rLxhUE-2 | readability | both | quality-1 |
| A-astra | 18tbwwPM91A-2 | structure | both | quality-1 |
| A-astra | leE02uqgaKg-2 | evidence | both | required-fail |
| A-astra | EqnGQDU71-Y-1 | evidence | literal | required-fail |
| A-astra | EqnGQDU71-Y-2 | evidence | literal | required-fail |

**製品の制約 1〜3（AC-13）:** 引用は下に書く。

<!-- check-record: constraints config=A-astra -->
| 構成 | 記事 | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|
| A-astra | roNZ-rLxhUE-1 | no | no | no |
| A-astra | roNZ-rLxhUE-2 | no | no | no |
| A-astra | 18tbwwPM91A-1 | no | no | no |
| A-astra | 18tbwwPM91A-2 | no | yes | no |
| A-astra | zW1RBacSqjk-1 | no | yes | no |
| A-astra | zW1RBacSqjk-2 | no | yes | no |
| A-astra | leE02uqgaKg-1 | no | no | no |
| A-astra | leE02uqgaKg-2 | yes | no | no |
| A-astra | EqnGQDU71-Y-1 | no | yes | yes |
| A-astra | EqnGQDU71-Y-2 | no | yes | yes |

**失敗した実行と番兵（AC-17）:** なし（10 件とも終了コード `0`）。
**人が変えた判定（AC-10）:**

- roNZ-rLxhUE-1 の根拠: LLM は pass / pass、人の判定は pass / fail。英伊の動機の推測を日本の理由とした書き方は文言どおりでは不合格（人が変えた）。
- zW1RBacSqjk-1 の制約 1: LLM は yes / -、人の判定は no / -。「横開き」を「横に広い／横長」とした描写は「パスポートみたいなサイズ」から導けるので制約 1 なし（2 段目の確認の際に見直し、人が変えた）。

**引用（AC-12）:** 必須の観点の不合格と品質の 1 である。「入った」は誤りを入れた工程、「残した」は直さなかった工程である。
- roNZ-rLxhUE-1 根拠（pass / fail）: 「重要な軍事技術を米国に依存し続けるべきでないという考えからでしょう」（字幕 06:42）。入った: 字数の調整（B3 の字数の調整の出力の文）、残した: 検証・推敲。
- roNZ-rLxhUE-2 読みやすさ（1 / 1）: 「## 徴兵制ではなく「ロボットが自衛隊」を」（字幕 31:03）。入った: 生成、残した: 検証・推敲。
- 18tbwwPM91A-2 構成（1 / 1）: 「一方、データにはレフト方向への強い打球も残っている」（字幕 13:45）。入った: 生成、残した: 検証・推敲。
- leE02uqgaKg-2 根拠（fail / fail）: 「一方、地球ほどの質量になると重力は強大で、月をつなぎとめる」（字幕 02:13）。入った: 生成、残した: 検証・推敲。
- EqnGQDU71-Y-1 根拠（pass / fail）: 「新博物館は…それ以上に「町民に愛される場所」になることを大切にしている」（字幕 06:21-06:36）。入った: 生成、残した: 検証・推敲。
- EqnGQDU71-Y-2 根拠（pass / fail）: 「いま関係者が何より大切にしているのは、観光客を呼ぶことに加えて」（字幕 06:21-06:36）。入った: 生成、残した: 検証・推敲。

**製品の制約 1〜3 の引用（AC-13）:**
- 18tbwwPM91A-2 制約 2: 「一方、データにはレフト方向への強い打球も残っている」（字幕 13:45）
- zW1RBacSqjk-1 制約 2: 「名前の付き方には注意が要る。」（字幕 00:44）
- zW1RBacSqjk-2 制約 2: 「価格は、ストレージ256GB・メモリー12GBのモデルが19万2680円から」（字幕 12:27）
- leE02uqgaKg-2 制約 1: 「一方、地球ほどの質量になると重力は強大で」（字幕 02:13）
- EqnGQDU71-Y-1 制約 2: 「むかわ竜の化石は、2003年4月、当時郵便局員だった堀田良幸」（字幕 00:35）
- EqnGQDU71-Y-1 制約 3: 「新博物館は…それ以上に「町民に愛される場所」になることを大切に」（字幕 06:21）
- EqnGQDU71-Y-2 制約 2: 「学校教育ではこれまで以上に博物館を活用してほしいと話す」（字幕 06:43）
- EqnGQDU71-Y-2 制約 3: 「いま関係者が何より大切にしているのは」（字幕 06:21-06:36）

## 8. 2 段目

**2 段目の採点と人の確認（AC-09〜AC-11）:** 2026-11-12 に、1 段目と同じ手順で採点した（`check_article.py` は 10 件とも `pass`。5 つのエージェントすべてがモデル ID `claude-opus-5-5` を報告した）。人の確認は 1 段目と同じ 3 段で行った。1 段目で論点ごとに決めた論点（roNZ-rLxhUE の形式の選択、leE02uqgaKg の「地球サイズ」、EqnGQDU71-Y の未来会議）は、同じ決定を当てた。zW1RBacSqjk の「横に広い」は、2 段目の確認の中で、1 段目と 2 段目の同じ言い換えを並べて人が論点として決め（制約 1 に当たらない）、1 段目で 1 問ずつ確認した判定もそれに合わせて見直した（§7 の冒頭）。人が変えたのは、18tbwwPM91A-2 の「根拠」（字幕では指摘が説明より前にあるのに、記事は説明を聞いて指摘したと順序ときっかけを逆にした。合格から両方不合格）の 1 判定である。2 段目の人の確認は 2026-11-12T08:33:49Z に終えた。採点がすべて終わったので、`~/.claude/agents/yt2column-0087-scorer.md` を消した（計画 §1.2 の決めたこと 9）。

| 種類 | 論点ごと | 1 問ずつ | 一括 |
|---|---|---|---|
| 必須の観点の不合格 | 4 | 1 | 0 |
| 観点「根拠」の合格 | 2 | 3 | 2 |
| 品質の 2 | 0 | 2 | 24 |
| 製品の制約の「あり」 | 4 | 8 | 0 |
| 製品の制約の「なし」 | 1 | 0 | 17 |

**B のモデルの選択（AC-08）:** C-astra と C-sol の合格件数（解釈による判定）で選ぶ。同数なら両方で作る（要件 F-002）。

<!-- check-record: b-selection -->
| 行 | C-astra の合格 | C-sol の合格 | B の構成 |
|---|---|---|---|
| B | 5 | 6 | B-sol |

`C-sol` の合格（6）が `C-astra`（5）より多いので、`gpt-6.1-sol` で生成する `B-sol` だけを作る（人の確認の印の後の値。§7 の冒頭）。

### 8.1. B-sol

**設定（AC-06）:** 生成と字数の調整は `openai/gpt-6.1-sol+medium`、検証・推敲は `claude/claude-opus-5-5+high` で、テンプレートは既定である。全工程を実行する。CLI のコミットとバイナリの SHA-256 は §2 に記録する（ステップ 1-6）。

**実行（AC-06）:** 工程ごとの保存した出力の識別子と、実行を始めた時刻である。

<!-- check-record: runs config=B-sol -->
| 構成 | 記事 | 回 | `--after` | `YT2COLUMN_LLM` | `YT2COLUMN_REVIEW_LLM` | 開始（UTC） | 終了（UTC） | 終了コード | 生成の識別子 | 字数の調整の識別子 | 検証・推敲の識別子 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| B-sol | roNZ-rLxhUE-1 | cur | none | openai/gpt-6.1-sol+medium | claude/claude-opus-5-5+high | 2026-11-12T08:07:25Z | 2026-11-12T08:09:02Z | 0 | 125da880e9dc52ec2c5945396881c381f33f9a9fc2ad1380f3785bcb6f3a4b32 | - | 59809a515932cca60bfd5a952f22a61275389de2c65b7ec9f81beb9687d8cb0f |
| B-sol | roNZ-rLxhUE-2 | cur | none | openai/gpt-6.1-sol+medium | claude/claude-opus-5-5+high | 2026-11-12T08:07:25Z | 2026-11-12T08:09:03Z | 0 | 396f24cf6844880dbac860ebb448aac7d2e40b548bc11b31aa44c5997927148b | - | c5abf461d93ed31b1dcdcd41636b7dbc94340579faa46de2d9d7a0f812ac495c |
| B-sol | 18tbwwPM91A-1 | cur | none | openai/gpt-6.1-sol+medium | claude/claude-opus-5-5+high | 2026-11-12T08:07:25Z | 2026-11-12T08:09:02Z | 0 | c7ee4e7df3eb21988b23553f92a04fc24b09bf9f52edcc12582b740dbbe3259c | - | 377fb485f0f8fca4ea197574ef6945fce6f08645f6de958d44db2bb1478d95c1 |
| B-sol | 18tbwwPM91A-2 | cur | none | openai/gpt-6.1-sol+medium | claude/claude-opus-5-5+high | 2026-11-12T08:07:25Z | 2026-11-12T08:09:14Z | 0 | 680aa486e47c834a2091e21b2015393b75a90cff81e0ea324d1fdefa05bde5ae | - | 6429e9bce0288688c6d0110372a4355efe5fb815099c291a85051b3d7e42e608 |
| B-sol | zW1RBacSqjk-1 | cur | none | openai/gpt-6.1-sol+medium | claude/claude-opus-5-5+high | 2026-11-12T08:07:25Z | 2026-11-12T08:08:51Z | 0 | ae6b01f51627f98cbb1b54e3777fdfbc547c8da7abe379185c1b96ef8facbcd6 | - | 01e2122f3b4df15793f42d48b2c03d6af03d91961744c847ac6103ad01c07490 |
| B-sol | zW1RBacSqjk-2 | cur | none | openai/gpt-6.1-sol+medium | claude/claude-opus-5-5+high | 2026-11-12T08:07:25Z | 2026-11-12T08:08:59Z | 0 | 45c6e9015fcf27ce024d47cd37af05189161542d65022b171119c002878cfd6c | - | 70cd5833602f82286ab2369706c63a4dd5c4d38671181d1827b8a3bd24b4f16e |
| B-sol | leE02uqgaKg-1 | cur | none | openai/gpt-6.1-sol+medium | claude/claude-opus-5-5+high | 2026-11-12T08:07:25Z | 2026-11-12T08:08:45Z | 0 | 48d4b09c7e2a250477df20e64dfe8c76b306e390bb35b4e141cd7e14843e062b | - | 676db9399f7ced829b027362b993910702dcf6f9b3bedc3f47f82a8cb064e6db |
| B-sol | leE02uqgaKg-2 | cur | none | openai/gpt-6.1-sol+medium | claude/claude-opus-5-5+high | 2026-11-12T08:07:25Z | 2026-11-12T08:08:47Z | 0 | 546cada973794f62cacc31a5b1f343a02e5200a414cbc115d214528b17d9c109 | - | 440cc95620337a3b0ea0ca0d0f74b7605093e6b6d0288b309289df5f57477ebd |
| B-sol | EqnGQDU71-Y-1 | cur | none | openai/gpt-6.1-sol+medium | claude/claude-opus-5-5+high | 2026-11-12T08:07:25Z | 2026-11-12T08:08:32Z | 0 | 6821e92315527590f2a0e429874602e50764cb962e1c35d284a95b17d2a68734 | - | c923abb53c7981b4aa137ac0e297ce0efeeaccad25684cc3426b1a0725cfa5ed |
| B-sol | EqnGQDU71-Y-2 | cur | none | openai/gpt-6.1-sol+medium | claude/claude-opus-5-5+high | 2026-11-12T08:07:25Z | 2026-11-12T08:08:30Z | 0 | 7559f64e0e10fc04da71a24c367afbb42169907248850b6375f311772286eeeb | - | 4cb68f91bb047407b8eab40915ac193b9249bd0a7284f4ab9cbd83821aa84f4a |

**トークン数と推定コスト（AC-14）:** 工程ごとの CLI の値である。「再利用した実行」は、再開で呼ばなかった工程の値の出どころ（B3 の記事）を示す。

<!-- check-record: stage-usage config=B-sol -->
| 構成 | 記事 | 回 | 工程 | 出どころ | 使用量 | 入力 | キャッシュから読んだ入力 | キャッシュに書いた入力 | 出力 | モデル | 推定コスト（USD） | 不明の理由 | 再利用した実行 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| B-sol | roNZ-rLxhUE-1 | cur | generation | run | known | 3 | 5386 | 11914 | 2980 | gpt-6.1-sol | 0.060130 | - | - |
| B-sol | roNZ-rLxhUE-1 | cur | review | run | known | 18738 | 0 | 0 | 2394 | claude-opus-5-5 | 0.122832 | - | - |
| B-sol | roNZ-rLxhUE-2 | cur | generation | run | known | 3 | 17300 | 0 | 3022 | gpt-6.1-sol | 0.031956 | - | - |
| B-sol | roNZ-rLxhUE-2 | cur | review | run | known | 18720 | 0 | 0 | 2797 | claude-opus-5-5 | 0.130820 | - | - |
| B-sol | 18tbwwPM91A-1 | cur | generation | run | known | 3 | 5386 | 7942 | 2717 | gpt-6.1-sol | 0.047570 | - | - |
| B-sol | 18tbwwPM91A-1 | cur | review | run | known | 13514 | 0 | 0 | 3034 | claude-opus-5-5 | 0.114736 | - | - |
| B-sol | 18tbwwPM91A-2 | cur | generation | run | known | 3 | 5386 | 7942 | 2709 | gpt-6.1-sol | 0.047490 | - | - |
| B-sol | 18tbwwPM91A-2 | cur | review | run | known | 13460 | 0 | 0 | 4713 | claude-opus-5-5 | 0.148100 | - | - |
| B-sol | zW1RBacSqjk-1 | cur | generation | run | known | 3 | 11854 | 0 | 2670 | gpt-6.1-sol | 0.027891 | - | - |
| B-sol | zW1RBacSqjk-1 | cur | review | run | known | 12030 | 0 | 0 | 2024 | claude-opus-5-5 | 0.088600 | - | - |
| B-sol | zW1RBacSqjk-2 | cur | generation | run | known | 3 | 5386 | 6468 | 2623 | gpt-6.1-sol | 0.042945 | - | - |
| B-sol | zW1RBacSqjk-2 | cur | review | run | known | 12015 | 0 | 0 | 3283 | claude-opus-5-5 | 0.113720 | - | - |
| B-sol | leE02uqgaKg-1 | cur | generation | run | known | 3 | 5386 | 6998 | 2811 | gpt-6.1-sol | 0.046150 | - | - |
| B-sol | leE02uqgaKg-1 | cur | review | run | known | 12021 | 0 | 0 | 1474 | claude-opus-5-5 | 0.077564 | - | - |
| B-sol | leE02uqgaKg-2 | cur | generation | run | known | 3 | 5386 | 6998 | 2814 | gpt-6.1-sol | 0.046180 | - | - |
| B-sol | leE02uqgaKg-2 | cur | review | run | known | 12050 | 0 | 0 | 1302 | claude-opus-5-5 | 0.074240 | - | - |
| B-sol | EqnGQDU71-Y-1 | cur | generation | run | known | 3 | 0 | 9497 | 2467 | gpt-6.1-sol | 0.048419 | - | - |
| B-sol | EqnGQDU71-Y-1 | cur | review | run | known | 8057 | 0 | 0 | 1163 | claude-opus-5-5 | 0.055488 | - | - |
| B-sol | EqnGQDU71-Y-2 | cur | generation | run | known | 3 | 9497 | 0 | 2505 | gpt-6.1-sol | 0.026006 | - | - |
| B-sol | EqnGQDU71-Y-2 | cur | review | run | known | 8091 | 0 | 0 | 1073 | claude-opus-5-5 | 0.053824 | - | - |

**記事ごとの推定コストと所要時間（AC-14・AC-16）:** 所要時間は記事を作った工程の時間の和で、10 件を並列に起動して測った値である（基準の構成と同じ。0018 評価記録 §4.8）。

<!-- check-record: run-costs config=B-sol -->
| 構成 | 記事 | 回 | 終了コード | この実行（USD） | 記事（USD） | 実行の時間 | 所要時間 | 比較に使う |
|---|---|---|---|---|---|---|---|---|
| B-sol | roNZ-rLxhUE-1 | cur | 0 | 0.182962 | 0.182962 | 1:36.9 | 1:36.9 | yes |
| B-sol | roNZ-rLxhUE-2 | cur | 0 | 0.162776 | 0.162776 | 1:37.6 | 1:37.6 | yes |
| B-sol | 18tbwwPM91A-1 | cur | 0 | 0.162306 | 0.162306 | 1:37.0 | 1:37.0 | yes |
| B-sol | 18tbwwPM91A-2 | cur | 0 | 0.195590 | 0.195590 | 1:48.4 | 1:48.4 | yes |
| B-sol | zW1RBacSqjk-1 | cur | 0 | 0.116491 | 0.116491 | 1:25.6 | 1:25.6 | yes |
| B-sol | zW1RBacSqjk-2 | cur | 0 | 0.156665 | 0.156665 | 1:34.3 | 1:34.3 | yes |
| B-sol | leE02uqgaKg-1 | cur | 0 | 0.123714 | 0.123714 | 1:19.8 | 1:19.8 | yes |
| B-sol | leE02uqgaKg-2 | cur | 0 | 0.120420 | 0.120420 | 1:22.1 | 1:22.1 | yes |
| B-sol | EqnGQDU71-Y-1 | cur | 0 | 0.103907 | 0.103907 | 1:06.4 | 1:06.4 | yes |
| B-sol | EqnGQDU71-Y-2 | cur | 0 | 0.079830 | 0.079830 | 1:05.1 | 1:05.1 | yes |

**失敗した実行とやり直し（AC-17）:** 番兵は主セッションが標準エラーを読んで下に記録し、人が確認する。

<!-- check-record: failed-runs config=B-sol -->
| 構成 | 記事 | 回 | 終了コード | 失敗した工程 | 入力 | キャッシュから読んだ入力 | キャッシュに書いた入力 | 出力 | 推定コスト（USD） | 支払い前の候補 | 支払い前と確認 |
|---|---|---|---|---|---|---|---|---|---|---|---|

<!-- check-record: retries config=B-sol -->
| 構成 | 記事 | やり直しの回数 | 記事を得た | 2 回を超えた |
|---|---|---|---|---|

**判定（AC-07・AC-09）:** 各セルは「解釈 / 文言」の順の判定である。

<!-- check-record: judgments config=B-sol -->
| 構成 | 記事 | 形式の選択 | 形式の規則 | 根拠 | 長さ | 表記 | 網羅 | 重要度 | 構成（観点） | 読みやすさ | タイトル・リード文 | 冗長さ | 目標（解釈） | 目標（文言） |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| B-sol | roNZ-rLxhUE-1 | fail/fail | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 3/3 | no | no |
| B-sol | roNZ-rLxhUE-2 | fail/fail | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 2/2 | no | no |
| B-sol | 18tbwwPM91A-1 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 2/2 | 2/2 | 2/2 | 3/3 | 3/3 | 2/2 | yes | yes |
| B-sol | 18tbwwPM91A-2 | pass/pass | pass/pass | fail/fail | pass/pass | pass/pass | 2/2 | 2/2 | 2/2 | 3/3 | 3/3 | 2/2 | no | no |
| B-sol | zW1RBacSqjk-1 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 3/3 | 2/2 | 2/2 | 3/3 | 2/2 | yes | yes |
| B-sol | zW1RBacSqjk-2 | pass/pass | pass/pass | pass/pass | pass/pass | pass/pass | 3/3 | 3/3 | 2/2 | 3/3 | 3/3 | 3/3 | yes | yes |
| B-sol | leE02uqgaKg-1 | pass/pass | pass/pass | fail/fail | pass/pass | pass/pass | 3/3 | 3/3 | 2/2 | 3/3 | 3/3 | 2/2 | no | no |
| B-sol | leE02uqgaKg-2 | pass/pass | pass/pass | fail/fail | pass/pass | pass/pass | 2/2 | 3/3 | 2/2 | 3/3 | 3/3 | 3/3 | no | no |
| B-sol | EqnGQDU71-Y-1 | pass/pass | pass/pass | pass/fail | pass/pass | pass/pass | 3/3 | 2/2 | 3/3 | 3/3 | 3/3 | 2/2 | yes | no |
| B-sol | EqnGQDU71-Y-2 | pass/pass | pass/pass | pass/fail | pass/pass | pass/pass | 3/3 | 2/2 | 2/2 | 3/3 | 3/3 | 2/2 | yes | no |

**判定者（AC-10）:** `script` はスクリプト、`llm` は人の確認の対象の外の LLM の判定、`llm+human` は人の確認の対象で人が変えなかった LLM の判定（一括で確認した判定を含む。§7 の冒頭）、`human` は人が変えた判定である。人が変えた判定と理由は下に書く。

<!-- check-record: judges config=B-sol -->
| 構成 | 記事 | 形式の選択 | 形式の規則 | 根拠 | 長さ | 表記 | 網羅 | 重要度 | 構成（観点） | 読みやすさ | タイトル・リード文 | 冗長さ | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| B-sol | roNZ-rLxhUE-1 | llm+human | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm | llm+human | llm+human | llm+human |
| B-sol | roNZ-rLxhUE-2 | llm+human | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| B-sol | 18tbwwPM91A-1 | llm | llm | llm+human | script | llm | llm+human | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| B-sol | 18tbwwPM91A-2 | llm | llm | human | script | llm | llm+human | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| B-sol | zW1RBacSqjk-1 | llm | llm | llm+human | script | llm | llm | llm | llm+human | llm+human | llm | llm+human | llm+human | llm+human | llm+human |
| B-sol | zW1RBacSqjk-2 | llm | llm | llm+human | script | llm | llm | llm | llm+human | llm | llm | llm | llm+human | llm+human | llm+human |
| B-sol | leE02uqgaKg-1 | llm | llm | llm+human | script | llm | llm | llm | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |
| B-sol | leE02uqgaKg-2 | llm | llm | llm+human | script | llm | llm+human | llm | llm+human | llm | llm | llm | llm+human | llm+human | llm+human |
| B-sol | EqnGQDU71-Y-1 | llm | llm | llm+human | script | llm | llm | llm+human | llm | llm | llm | llm+human | llm+human | llm+human | llm+human |
| B-sol | EqnGQDU71-Y-2 | llm | llm | llm+human | script | llm | llm | llm+human | llm+human | llm | llm | llm+human | llm+human | llm+human | llm+human |

**機械的な値（AC-09）:** `check_article.py` の出力である。

<!-- check-record: article-checks config=B-sol -->
| 構成 | 記事 | タイトルの字数 | 節の数 | 長さ |
|---|---|---|---|---|
| B-sol | roNZ-rLxhUE-1 | 21 | 5 | 3375 |
| B-sol | roNZ-rLxhUE-2 | 18 | 5 | 3347 |
| B-sol | 18tbwwPM91A-1 | 19 | 4 | 3131 |
| B-sol | 18tbwwPM91A-2 | 21 | 4 | 3128 |
| B-sol | zW1RBacSqjk-1 | 21 | 4 | 3326 |
| B-sol | zW1RBacSqjk-2 | 25 | 4 | 3330 |
| B-sol | leE02uqgaKg-1 | 21 | 4 | 3183 |
| B-sol | leE02uqgaKg-2 | 21 | 4 | 3168 |
| B-sol | EqnGQDU71-Y-1 | 23 | 3 | 2780 |
| B-sol | EqnGQDU71-Y-2 | 23 | 3 | 2815 |

**引用が要る判定（AC-12）:** 引用、誤りを入れた工程、直さなかった工程は下に書く。

<!-- check-record: quotes-needed config=B-sol -->
| 構成 | 記事 | 観点 | 判定 | 種類 |
|---|---|---|---|---|
| B-sol | roNZ-rLxhUE-1 | format_choice | both | required-fail |
| B-sol | roNZ-rLxhUE-2 | format_choice | both | required-fail |
| B-sol | 18tbwwPM91A-2 | evidence | both | required-fail |
| B-sol | leE02uqgaKg-1 | evidence | both | required-fail |
| B-sol | leE02uqgaKg-2 | evidence | both | required-fail |
| B-sol | EqnGQDU71-Y-1 | evidence | literal | required-fail |
| B-sol | EqnGQDU71-Y-2 | evidence | literal | required-fail |

**製品の制約 1〜3（AC-13）:** 引用は下に書く。

<!-- check-record: constraints config=B-sol -->
| 構成 | 記事 | 制約 1 | 制約 2 | 制約 3 |
|---|---|---|---|---|
| B-sol | roNZ-rLxhUE-1 | no | yes | no |
| B-sol | roNZ-rLxhUE-2 | no | no | no |
| B-sol | 18tbwwPM91A-1 | no | yes | no |
| B-sol | 18tbwwPM91A-2 | no | yes | no |
| B-sol | zW1RBacSqjk-1 | no | yes | no |
| B-sol | zW1RBacSqjk-2 | no | yes | no |
| B-sol | leE02uqgaKg-1 | yes | yes | no |
| B-sol | leE02uqgaKg-2 | yes | yes | no |
| B-sol | EqnGQDU71-Y-1 | no | no | yes |
| B-sol | EqnGQDU71-Y-2 | no | yes | yes |

**失敗した実行と番兵（AC-17）:** なし（10 件とも終了コード `0`）。
**人が変えた判定（AC-10）:**

- 18tbwwPM91A-2 の根拠: LLM は pass / pass、人の判定は fail / fail。指摘の順序ときっかけを逆にした書き方は両方不合格（人が変えた）。

**引用（AC-12）:** 必須の観点の不合格と品質の 1 である。「入った」は誤りを入れた工程、「残した」は直さなかった工程である。
- roNZ-rLxhUE-1 形式の選択（fail / fail）: 「聞き手の佐藤友香とのやり取りでは、武器輸出や台湾有事、徴兵制などが取り上げられた」（字幕 00:45）。入った: 生成、残した: -（検証・推敲は形式を変えない）。
- roNZ-rLxhUE-2 形式の選択（fail / fail）: 「小泉悠（こいずみ・ゆう）は、THINKpodで聞き手の佐藤友香に対し、…と説く」（字幕 00:45）。入った: 生成、残した: -（検証・推敲は形式を変えない）。
- 18tbwwPM91A-2 根拠（fail / fail）: 「強くぶつけるように捉えるという説明を聞き、「松井秀喜は全然…」と指摘する」（字幕 11:16）。入った: 生成、残した: 検証・推敲。
- leE02uqgaKg-1 根拠（fail / fail）: 「一方、地球や太陽ほどの質量になれば、月や惑星の運動を左右する」（字幕 02:13）。入った: 生成、残した: 検証・推敲。
- leE02uqgaKg-2 根拠（fail / fail）: 「一方、地球や太陽ほどの質量になると、月や惑星の運動を」（字幕 02:13）。入った: 生成、残した: 検証・推敲。
- EqnGQDU71-Y-1 根拠（pass / fail）: 「計画で重視されているのは、…町民に愛される施設をつくるという考え方である」（字幕 06:21-06:36）。入った: 生成、残した: 検証・推敲。
- EqnGQDU71-Y-2 根拠（pass / fail）: 「新博物館の計画で重視されているのは、観光の目玉となることと、町民に愛されることを両立させる方向だ」（字幕 06:21-06:36）。入った: 生成、残した: 検証・推敲。

**製品の制約 1〜3 の引用（AC-13）:**
- roNZ-rLxhUE-1 制約 2: 「問われるのは、ある空域をどこまで自ら使え、相手にはどこまで使わせないようにできるか」（字幕 09:39）
- 18tbwwPM91A-1 制約 2: 「左打者である松井にとって、前の右足は、身体がそれ以上前へ行かない」（字幕 13:25）
- 18tbwwPM91A-2 制約 2: 「強い打球を生む体の使い方について、松井が挙げたのは下半身だった」（字幕 12:27）
- zW1RBacSqjk-1 制約 2: 「比較した両機種のメモリーは12GBで、1TBモデルでは16GBになる」（字幕 05:54）
- zW1RBacSqjk-2 制約 2: 「なお、今回の製品名には注意が必要だ。」（字幕 00:41-01:23）
- leE02uqgaKg-1 制約 1: 「地球や太陽ほどの質量になれば」（字幕 02:13）
- leE02uqgaKg-1 制約 2: 「ここから話は、自然界の四つの相互作用へと広がる」（字幕 14:46）
- leE02uqgaKg-2 制約 1: 「地球や太陽ほどの質量になると」（字幕 02:13）
- leE02uqgaKg-2 制約 2: 「ところが、精密な観測が進むと、水星の軌道にも計算とのずれが見つかる」（字幕 04:34）
- EqnGQDU71-Y-1 制約 3: 「計画で重視されているのは、…町民に愛される施設をつくるという考え方である」（字幕 06:21-06:36）
- EqnGQDU71-Y-2 制約 2: 「宮崎さんは、全国から注目されれば町もさらに盛り上がるのではないかと話し」（字幕 03:25-03:38）
- EqnGQDU71-Y-2 制約 3: 「新博物館の計画で重視されているのは…町民に愛されることを両立させる方向だ」（字幕 06:21-06:36）

## 9. 比較

**比較の表（AC-18）:** 平均と最大は比較に使う記事（成功し、手順から外れていない実行）だけで求める。品質の観点の分布は解釈による判定の 60 の判定である。

<!-- check-record: comparison -->
| 構成 | 比較に使う | 得られなかった | 合格（解釈） | 合格（文言） | 品質 1 | 品質 2 | 品質 3 | 推定コストの平均（USD） | 推定コストの最大 | 最大の記事 | 所要時間の平均 | 所要時間の最大 | 最大の記事 | 失敗した実行 | 失敗の推定コスト | 推定コストが不明の失敗 | キャッシュを読んだ工程 | 補助の推定コストの平均 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| C-sol | 10 | 0 | 6 | 4 | 3 | 31 | 26 | 0.067110 | 0.100653 | roNZ-rLxhUE-1 | 1:05.1 | 1:22.6 | 18tbwwPM91A-1 | 0 | 0.000000 | 0 | 9 | 0.087728 |
| C-astra | 10 | 0 | 5 | 3 | 3 | 28 | 29 | 0.366250 | 0.500359 | roNZ-rLxhUE-2 | 1:04.7 | 1:20.4 | 18tbwwPM91A-1 | 0 | 0.000000 | 0 | 7 | 0.444608 |
| A-sol | 10 | 0 | 7 | 4 | 2 | 21 | 37 | 0.289769 | 0.521705 | roNZ-rLxhUE-1 | 1:45.8 | 3:03.7 | roNZ-rLxhUE-1 | 0 | 0.000000 | 0 | 0 | - |
| A-astra | 10 | 0 | 7 | 4 | 2 | 21 | 37 | 0.423577 | 0.727139 | roNZ-rLxhUE-1 | 1:46.5 | 3:04.4 | roNZ-rLxhUE-1 | 0 | 0.000000 | 0 | 0 | - |
| B-sol | 10 | 0 | 5 | 3 | 0 | 26 | 34 | 0.140466 | 0.195590 | 18tbwwPM91A-2 | 1:27.3 | 1:48.4 | 18tbwwPM91A-2 | 0 | 0.000000 | 0 | 9 | 0.157498 |

## 10. 結論

**F-006 の条件ごとの判定（AC-21）:**

<!-- check-record: f006 -->
| 構成 | 合格（解釈） | 基準との差 | (i) | 合格が基準以上 | 動画ごとの比較 | 推定コストの平均（USD） | 推定コストが低い | (ii) | 所要時間の平均 | (iii) | 候補 | 補助の値で変わる |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| C-sol | 6 | -2 | no | no | no | 0.067110 | yes | no | 1:05.1 | yes | no | no |
| C-astra | 5 | -3 | no | no | no | 0.366250 | no | no | 1:04.7 | yes | no | no |
| A-sol | 7 | -1 | no | no | yes | 0.289769 | yes | no | 1:45.8 | yes | no | no |
| A-astra | 7 | -1 | no | no | yes | 0.423577 | no | no | 1:46.5 | yes | no | no |
| B-sol | 5 | -3 | no | no | no | 0.140466 | yes | no | 1:27.3 | yes | no | no |

**動画ごとの必須の観点の不合格（AC-21）:**

<!-- check-record: f006-video -->
| 構成 | 動画 | 不合格の記事 | 基準の構成 | 基準以下 |
|---|---|---|---|---|
| C-sol | roNZ-rLxhUE | 1 | 0 | no |
| C-sol | 18tbwwPM91A | 0 | 0 | yes |
| C-sol | zW1RBacSqjk | 0 | 0 | yes |
| C-sol | leE02uqgaKg | 1 | 1 | yes |
| C-sol | EqnGQDU71-Y | 0 | 0 | yes |
| C-astra | roNZ-rLxhUE | 2 | 0 | no |
| C-astra | 18tbwwPM91A | 0 | 0 | yes |
| C-astra | zW1RBacSqjk | 0 | 0 | yes |
| C-astra | leE02uqgaKg | 1 | 1 | yes |
| C-astra | EqnGQDU71-Y | 0 | 0 | yes |
| A-sol | roNZ-rLxhUE | 0 | 0 | yes |
| A-sol | 18tbwwPM91A | 0 | 0 | yes |
| A-sol | zW1RBacSqjk | 0 | 0 | yes |
| A-sol | leE02uqgaKg | 1 | 1 | yes |
| A-sol | EqnGQDU71-Y | 0 | 0 | yes |
| A-astra | roNZ-rLxhUE | 0 | 0 | yes |
| A-astra | 18tbwwPM91A | 0 | 0 | yes |
| A-astra | zW1RBacSqjk | 0 | 0 | yes |
| A-astra | leE02uqgaKg | 1 | 1 | yes |
| A-astra | EqnGQDU71-Y | 0 | 0 | yes |
| B-sol | roNZ-rLxhUE | 2 | 0 | no |
| B-sol | 18tbwwPM91A | 1 | 0 | no |
| B-sol | zW1RBacSqjk | 0 | 0 | yes |
| B-sol | leE02uqgaKg | 2 | 1 | no |
| B-sol | EqnGQDU71-Y | 0 | 0 | yes |

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
| C-sol | no | no | no |
| C-astra | no | no | no |
| A-sol | no | no | no |
| A-astra | no | no | no |
| B-sol | no | no | no |

**判断（AC-21・AC-22）:** 候補になった構成はない。
- (i)（合格が基準の構成より 2 件以上多い）を満たす構成はない。
- (ii) は、どの構成も「合格件数が基準の構成以上」を満たさない。A-sol と A-astra は、動画ごとの比較を満たし、A-sol は推定コストも基準の構成より低いが、合格が 7 件で 1 件足りない。C-sol・C-astra・B-sol は、roNZ-rLxhUE（形式の選択）などで動画ごとの比較も満たさない。
- (iii) はすべての構成が満たす。
- プロンプトのキャッシュから読んだ入力のある構成（C-sol・C-astra・B-sol）は、補助の推定コストの平均（§9）で判定しても結果は変わらない（表の「補助の値で変わる」がすべて `no`）。

F-006 の手順 4 により、本番の構成は変えない。手順 3 で除いた候補はない。

**人の承認（AC-22）:** 2026-11-12 に、上の判断と、失敗した実行の回数（0 回）と費用（0 USD）、使った額の合計（7.733370 USD）を人に示し、「本番の構成を変えない」の承認を受けた。そのため、計画のフェーズ 6（本番の構成の変更）と PR-3 は行わない。

**製品の制約 1〜3（AC-23）:** どの構成でも、3 種類とも 10 件のどれかの記事に残った（上の表がすべて `no`）。基準の構成の値は 0008 評価記録 §6.3 による（§3）。制約 1 は、どの構成でも leE02uqgaKg の「地球サイズ」を「質量」とした言い換えである。
