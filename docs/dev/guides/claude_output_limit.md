# Claude の出力の上限の測り直し

リクエストが出力の上限をプロバイダに任せるとき（`MaxOutputTokens` が 0）、Claude のアダプタは自身の既定の `max_tokens`、すなわち `internal/llm/claude/request.go` の定数 `defaultMaxOutputTokens` を送る。記事を書く 3 つの工程（生成、字数の調整、検証・推敲）は、どれもこの既定値を使う。値は、測定した出力の大きさと出力の速さに基づくので、出力を生み出す構成が変われば古くなりうる。本書では、いつ測り直すか、どう測るか、何を更新するかを説明する。値も測定値も本書には転記しない。それらはコードと、「現在の記録」で挙げる記録にある。

## 測り直すとき

本番の構成の 3 つの工程のいずれかで、次のどれかが変わったときに測り直す。

- テンプレート。
- effort（`internal/config/config.go` の `defaultClaudeEffort` の変更を含む）。
- モデル。

## 費用と承認

測定は実際の Anthropic API を呼び、課金される。実行の前に人間の承認を得る（`CLAUDE.md` の "Tool Execution Safety"）。評価用の動画の組を実行する方法は、`docs/tasks/0008_column_prompt/02_evaluation.md` §2.3 と `docs/tasks/0018_article_quality/02_evaluation.md` §3 に従う。

## 測り方

速さは LLM の呼び出しごとに求めるので、CLI の実行はそれぞれ LLM をちょうど 1 回呼ばなければならない。各工程を別々に実行し、前の工程が保存した出力を `--after` で再利用する。どの実行にも、`--artifacts <dir>`（キャッシュのディレクトリの外にある既存のディレクトリ）と `--keep-cache` を付け、`--out` には新しいパスを渡す。

- `--keep-cache` は、成功した実行の後も動画のキャッシュを残す。付けないと、次の実行が文字起こしを取得し直し、測った時間にネットワークの時間が加わる。さらに、取得し直した文字起こしが前と違うと、再開が失敗する。
- `--out` は記事をファイルに書くので、測定の実行で Webhook に投稿されることはない。
- 同じキャッシュのディレクトリを共有する実行は、1 つずつ行う。

工程は次のとおり。

1. 生成: `--no-review --no-shorten`。
2. 字数の調整: `--after generation --no-review`。字数の調整の工程は、生成した本文が長さの上限を超えたときだけ走る。そのため、本文が上限に収まった実行は LLM を呼ばず、字数の調整の測定値が得られない。
3. 検証・推敲（字数の調整の工程が走った実行の後）: `--after shorten`。
4. 検証・推敲（字数の調整の工程が走らなかった実行の後）: `--after generation --no-shorten`。ここでは `--after shorten` を使えない。字数の調整の工程の出力が保存されておらず、読み込みが失敗するためである。この形は字数の調整の工程と最後の長さの検査を飛ばすので、字数の調整の工程が走らなかったときだけ使う。

## 集めるもの

各実行について次を集める。

- その工程の出力のトークン数。LLM を呼んだ工程の `tokens: <stage> run` の行にある `output=`（[cli.md の「トークンの使用量」](cli.md#トークンの使用量)を参照。再開した実行は、前の工程の `reused` の行も書く）か、`--artifacts` のディレクトリに保存されたその工程のファイルの `usage.output` から読み取る。
- CLI の呼び出しの実時間。記録されている測定と同じ方法で、例えば `time` で測る。
- 実行が打ち切られたかどうか。CLI は `llm.ErrTruncated` をラップしたエラー（"generation truncated by the output token limit"）を報告する。

## 判定の仕方

値を、`docs/tasks/0041_claude_effort_max_tokens/01_requirements.md` の F-002 の 3 つの条件と照らす。

- (a) 値が、観測した最大の出力の 2 倍以上である。
- (b) 最も遅い速さでその値のトークン数を生成するのに要する時間が、`provider.LLMTimeout` の半分以下である。速さは、各実行の出力のトークン数を実時間で割って求める。実時間には LLM の呼び出し以外の処理も含まれるので、求めた速さは実際より低く出るが、これは安全側の誤差である。
- (c) F-002 の (c) が挙げる各モデル（現時点では `claude-opus-5-5` と `claude-sonnet-5-5`）が、値を非ストリーミングのリクエストの `max_tokens` として受け付ける。具体的には、値が Anthropic の文書に載るそのモデルの最大出力トークン数以下であるか、そのモデルが値を受け付けることを既存の観測が示している。

打ち切られた実行の出力は、送った上限以上であることしか分からない。その旨を記録する。

(a) と (b) の両方を満たす値がないときは、無理に値を選ばない。呼び出しそのものの変更（ストリーミング、または `provider.LLMTimeout`）は、それだけで独立した決定になるので、新しいタスクとして提起する。

## 更新するもの

値が条件を満たさなくなったときは、同じ変更の中で次をまとめて更新する。

1. `internal/llm/claude/request.go` の定数 `defaultMaxOutputTokens`。
2. `internal/llm/provider/provider_test.go` の、`TestClaudeDefaultMaxTokensFitsBudget` が値と照らし合わせる測定値（`largestObservedOutputTokens`・`slowestRateTokens`・`slowestRateDuration`）と、その出どころを書いたコメント。
3. 根拠の記録: この変更を行うタスクのアーキテクチャ設計書（`02_architecture.md`）。
4. 下の「現在の記録」が、その記録を指すように直す。
5. README の "Configuration" にある、`max` と既定の effort についての注記（新しい測定で記述の内容が変わる場合）。

## 現在の記録

現在の根拠は `docs/tasks/0041_claude_effort_max_tokens/02_architecture.md` §1.4 に記録されている。
