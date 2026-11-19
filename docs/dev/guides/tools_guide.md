# tools ディレクトリの道具

リポジトリ直下の `tools/` は、開発者が手元で使う小さな道具の置き場である。製品のコードではない。CI やテストからは呼ばれない。

| ファイル（`tools/` の下） | 用途 |
|---|---|
| `chief.sh` | コストを抑えた設定で `/chief` を起動する |
| `chief_usage_scan.py` | Claude Code の transcript から `/chief` セッションのトークン使用量を集める |
| `chief_usage_cost.py` | 集めた使用量を API 単価で金額に換算する |

## chief.sh

`/chief` を、Sonnet で、自動 compact の閾値を下げて起動する。

```sh
tools/chief.sh
```

- **モデル:** `--model sonnet` で起動する。Opus で動かすと、同じ使い方で費用が約 2 倍になる。
- **compact の閾値:** `CLAUDE_CODE_AUTO_COMPACT_WINDOW=150000` を渡す。文脈がおよそ 15 万トークンを超えると自動 compact が起き、文脈が要約される。1M 文脈のままでは compact がほとんど起きず、`/chief` の 1 回の呼び出しの文脈が平均約 39 万トークンまで育っていた（2026-11-20 の集計）。
- **起動する場所:** chief 専用の Herdr workspace の pane から実行する。chief の使い方は [chief_guide.md](chief_guide.md) を参照。

注意: `CLAUDE_CODE_AUTO_COMPACT_WINDOW` は Claude Code の本体（2.1.329）にあることを確かめただけで、値の単位と効き方は推測である。効いたかどうかは、下の集計で 1 回あたりの文脈を測り直して確かめる。

## chief_usage_scan.py と chief_usage_cost.py

`/chief` の費用を、API で払った場合の金額として見積もる。2 段で動かす。

```sh
python3 -I tools/chief_usage_scan.py /path/to/chief.json
python3 -I tools/chief_usage_cost.py /path/to/chief.json
```

出力先の JSON は、リポジトリの外の一時ディレクトリに置く。

### chief_usage_scan.py

`~/.claude/projects/*/*.jsonl` をすべて読み、`/chief` を起動したセッションのトークン使用量をモデルごとに集める。結果を引数の JSON ファイルに書き、セッションごとの一覧（始まり、終わり、ディレクトリ、セッション ID、モデルごとの呼び出し回数と出力トークン数）を表示する。

- **数える範囲:** 人が打った `/chief` が最初に現れた後の assistant 応答をすべて数える。同じセッションで `/chief` の後に別の作業をした分も含まれる。
- **重複:** 同じ message id の応答は 1 回だけ数える。
- **数えないもの:** サブエージェントの transcript（`subagents/` の下）は読まない。

### chief_usage_cost.py

`chief_usage_scan.py` の JSON を読み、次の 3 つを表示する。

- 日別の金額（Sonnet と Opus）
- モデルごとの合計と、その内訳（出力、キャッシュ読み込み、キャッシュ書き込み、入力）
- Sonnet のセッションごとの所要時間、呼び出し回数、金額

注意:

- **単価:** スクリプトに直接書いてある。対象は Sonnet 5.5 と Opus 5.5 で、単位は 1M トークンあたりのドル（入力、出力、5 分キャッシュ書き込み、1 時間キャッシュ書き込み、キャッシュ読み込み）。単価が変わったら `P` を直す。ほかのモデルは数えない。
- **日付:** セッションが最後に終わった日（UTC）に、そのセッションの全額を数える。日をまたぐセッションは終わった日にまとめて数えられる。
- **金額の意味:** API 単価での換算値である。サブスクリプションで使っている場合の実際の請求額ではない。
