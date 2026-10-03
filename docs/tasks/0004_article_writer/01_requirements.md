# 要件定義書：ArticleWriter とプロンプトテンプレート

## Document Status

| Item | Value |
|---|---|
| Status | `approved` |
| Created | 2026-10-03 |
| Review date | 2026-10-03 |
| Reviewer | isseis |
| Comments | - |

## 1. 概要 (Overview)

**目的:** 本書は、`ArticleWriter` の初期実装（`internal/writer`）と、それが使うプロンプトテンプレート（`prompts/`）に関する要件を定義する。`ArticleWriter` は、`Transcript` からプロンプトを組み立てて `LLMClient` を呼び出し、生成テキストを `Article` に変換する。

**背景:**
yt2column は `URL → TranscriptSource → Transcript → ArticleWriter → Article → Publisher` というパイプラインで動作する（[project_overview.md](../../dev/project_overview.md)）。`ArticleWriter` の interface と `Article` 型はパイプラインの骨格（#2、タスク `0001_pipeline_skeleton`）で定義済みである。字幕取得（#3、タスク `0002_ytdlp_transcript_source`）と、`LLMClient` の初期実装である DeepSeek アダプタ（#4、タスク `0003_deepseek_llm_client`）も実装済みである。本タスクは、その間をつなぐ `ArticleWriter` の実装を作る。

`ArticleWriter` はプロバイダに依存しない。プロンプトの組み立てと出力の後処理をここに集約し、全プロバイダで共有する。プロンプトの文面は後で調整する（#8）ため、テンプレートファイルとしてコードから分離し、バイナリに埋め込んだうえで、外部ファイルで上書きできるようにする。

元動画への出典リンクは、記事の末尾に必ず付ける（[project_overview.md](../../dev/project_overview.md)「前提・制約」）。字幕・タイトル・概要欄はプロンプトに埋め込まれ、プロンプトインジェクションで生成結果が操作されうる。そこで、出典リンクは LLM に任せず、コードで付与する（[security.md](../../dev/security.md) §6）。

**本書の記述範囲:** 本書は、観測できる振る舞い（何を受け取り、LLM に何を送り、何を返し、何をどの番兵エラーで拒否するか）を定める。番兵エラーとは、`errors.Is` で判別できる、あらかじめ定めたエラー値である。その振る舞いを実現する手段（型や関数の名前、テンプレートで参照できる値の名前、上限値、出典ブロック（F-006）の正確な書式など）は設計（`02_architecture.md`）で決める。要件の作成過程で挙がった、設計で決めるべき事項は [design_handoff.md](design_handoff.md) に、実装計画とテストの組み立てで扱う事項は [implementation_handoff.md](implementation_handoff.md) に申し送る。

対応 issue: #5

## 2. 目的とスコープ (Goals and Scope)

### 2.1. 目的 (Goals)

-   `Transcript` から、字幕本文（F-003）とメタ情報を埋め込んだ system プロンプトと user プロンプトを組み立て、`LLMClient` を 1 回呼び出せる。
-   LLM の生成テキストを、タイトルと Markdown 本文からなる `Article` に変換できる。
-   記事の末尾に、LLM の出力に依存せず、元動画への出典リンクを必ず付ける。
-   プロンプトテンプレートをバイナリに埋め込み、外部ファイルで上書きできる。
-   LLM の出力という信頼できない入力を、補正せずに検証する。記事に変換できない出力は番兵エラーで拒否し、不完全な記事を返さない。
-   LLM の API を使わずに（fake の `LLMClient` で）、`ArticleWriter` の振る舞いをテストできる。

### 2.2. スコープ (In Scope)

-   `internal/writer` パッケージの、`writer.ArticleWriter` を実装する型
-   構築時の入力（`LLMClient`、テンプレートの上書きファイル）の検証
-   仮のプロンプトテンプレート（system 用と user 用）と、そのバイナリへの埋め込み
-   外部ファイルによるテンプレートの上書き
-   `Transcript` の検証と、プロンプトの組み立て
-   LLM の生成テキストの検証と、`Article` への変換
-   出典リンクの付与
-   `writer.Article` への、モデルの版の識別子（`ModelVersion`）の追加
-   fake の `LLMClient` を使うユニットテスト

### 2.3. スコープ外 (Out of Scope)

-   本番用のプロンプトの文面（文体・長さ・見出しの構成）と、その評価（#8）。本タスクのテンプレートの文面は仮のものでよい。ただし、既定のテンプレートは、動画タイトル・チャンネル名・概要欄・字幕本文の 4 つの値をすべて埋め込み、F-005 が定める出力の形（先頭行がタイトルの見出し）を LLM に指示する内容を含める（F-001・AC-30）。
-   上書きファイルのパスを、環境変数や CLI 引数から受け取ること（#6）。本タスクの `ArticleWriter` は環境変数も CLI 引数も読まず、上書きファイルのパスを構築時に受け取る。
-   プロバイダごとのプロンプトの微調整（共通テンプレート＋プロバイダ別の上書きなど。[project_overview.md](../../dev/project_overview.md)「未確定事項」）。
-   長い動画のチャンク分割（#13）。字幕の全文を 1 回のプロンプトに埋め込む。ただし、展開したプロンプトの大きさには有限の上限を設ける（F-003）。上限値は `02_architecture.md` で固定し、想定する 40 分前後の動画（[project_overview.md](../../dev/project_overview.md)「前提・制約」）の字幕を埋め込んでも十分な余裕がある値とする。上限を超える長い動画への対応は #13 で扱う。
-   見出しごとの、動画の該当時刻へのリンク（#10）。本タスクはタイムスタンプをプロンプトにも記事にも使わない。
-   出力トークン数の上限（`MaxOutputTokens`）の設定。本タスクの `ArticleWriter` は `MaxOutputTokens` に 0 を渡し、上限をプロバイダの既定に任せる。
-   リトライ。`LLMClient` の失敗は 1 回で呼び出し元へ報告する。
-   出典ブロックへの、動画タイトルやチャンネル名の記載。出典ブロックは元動画の URL だけを示す（F-006）。
-   投稿先ごとの書式の変換（Slack の `markdown` ブロックへの分割など。#7）。
-   `Transcript` の型の変更（動画 ID を検証済みの型にするなど）。本タスクは、現行の `transcript.Transcript` を受け取り、`ArticleWriter` の側で検証する（F-002）。

## 3. 機能要件 (Functional Requirements)

### 3.1. 機能一覧

#### F-001: 構築とプロンプトテンプレートの読み込み

`ArticleWriter` の値を、`LLMClient` と、任意のテンプレートの上書きファイルから構築する。構築時の入力は補正せず、不正なら構築をエラーにする。

-   `LLMClient` は `llm.LLMClient` として受け取る。nil、および動的な値が nil の interface 値（typed nil）は拒否する。
-   プロンプトテンプレートは、system プロンプト用と user プロンプト用の 2 つとする。
-   既定のテンプレートは、リポジトリの `prompts/` に置いたファイルをバイナリに埋め込んだものとし、実行時に外部のファイルを必要としない（埋め込みの手段は [design_handoff.md](design_handoff.md) H-01）。既定のテンプレートは、system 用と user 用を合わせて、参照できる 4 つの値（後述）をすべて埋め込み、F-005 が定める出力の形（先頭行が `# ` で始まるタイトルの見出し）を LLM に指示する（2.3）。
-   system 用と user 用のそれぞれについて、上書きファイルのパスを構築時に任意で与えることができる。パスを与えたテンプレートは、既定のものの代わりにそのファイルの内容から作る。与えなかったテンプレートは既定のものから作る。上書きファイルは構築時に 1 回だけ読み、`Write` のたびには読み直さない。
-   テンプレートの構文は Go の `text/template` とする。テンプレートから参照できる値は、動画タイトル・チャンネル名・概要欄・字幕本文（F-003）の 4 つとする。参照するときの名前は設計で確定し、利用者がテンプレートを書けるよう文書に記す。本書の例では、仮に `.Title`・`.Description` などと書く。`text/template` の組み込み関数以外の関数は提供しない。
-   次のいずれかに当てはまるテンプレートは、構築時に `ErrInvalidTemplate` で拒否する。既定のテンプレートにも同じ規則を適用する。
    -   上書きファイルを読めない（存在しない、権限がないなど）。
    -   上書きファイルが通常のファイルでない（ディレクトリ、名前付きパイプ（FIFO）、デバイス、ソケットなど）。この場合、構築は書き込み側や終わりのない入力を待ち続けずに、エラーを返す。
    -   内容が空、または空白文字（§6）だけである。
    -   正しい UTF-8 のバイト列でない。
    -   サイズが上限を超える。上限値は `02_architecture.md` で固定する。
    -   `text/template` の構文として解析できない。
    -   参照できる 4 つの値以外を参照している。条件分岐の中など、展開時に実行されるとは限らない位置にある参照も対象とする。この判定は構築時に行い、`Write` の時点まで持ち越さない。テンプレートの誤りを、字幕の取得や LLM の呼び出しの後ではなく、構築時に報告するためである。
-   値によって展開時にだけ起きる失敗（展開時のエラー、展開結果が空になる場合、展開結果が大きさの上限を超える場合）は、構築時には一般に判定できないので、`Write` の時点で検出する（F-003）。ただし、AC-28 の `printf` の例のように構築時に判定できるものは、構築時に拒否してもよい。
-   構築した値は `writer.ArticleWriter` として使える。

**Acceptance Criteria**:
- **AC-01**: 有効な `LLMClient` から、上書きファイルを与えずに構築した `ArticleWriter` は、`writer.ArticleWriter` として使える。このとき、`LLMClient` に渡る system プロンプトと user プロンプトは、それぞれ埋め込みの既定のテンプレートから組み立てたものである。
- **AC-02**: nil の `LLMClient`、および typed nil の `LLMClient` を与えた構築はエラーになり、`ArticleWriter` を返さない。
- **AC-03**: system 用の上書きファイルだけを与えて構築した場合、`LLMClient` に渡る system プロンプトは上書きファイルの内容から組み立てたものであり、user プロンプトは既定のテンプレートから組み立てたものである。user 用だけを与えた場合も同様に、user プロンプトだけが上書きファイルの内容になる。
- **AC-04**: 上書きファイルの内容を構築後に書き換えても、構築済みの `ArticleWriter` が `Write` で使うテンプレートは変わらない。
- **AC-05**: 次のファイルを上書きファイルとして与えた構築は、`errors.Is(err, ErrInvalidTemplate)` が真になるエラーになり、`ArticleWriter` を返さない。
    -   存在しないファイル（この場合は `errors.Is(err, fs.ErrNotExist)` も真になる）
    -   ディレクトリのパス
    -   名前付きパイプ（FIFO）のパス。書き込み側が開かないままでも、構築は待ち続けずに（テストが定める短い時間内に）エラーを返す。
    -   内容が空、または空白文字だけのファイル
    -   不正な UTF-8 のバイト列を含むファイル
    -   サイズが上限を超えるファイル（サイズがちょうど上限のファイルは受理する）
    -   `text/template` として解析できないファイル（例: `{{.Title`）
    -   参照できない値を参照するファイル（例: `{{.APIKey}}`、条件分岐の中にある `{{if .Description}}{{.APIKey}}{{end}}`）
- **AC-06**: リポジトリに含まれる既定のテンプレート（system 用・user 用）は、上書きファイルと同じ規則の検証を通る。この検証は、埋め込まれた実物のテンプレートに対するテストで行い、既定のテンプレートを規則に反する形に編集するとテストが失敗する。
- **AC-30**: 埋め込まれた実物の既定のテンプレートで構築した `ArticleWriter` に、`Title`・`ChannelName`・`Description`・`Segment.Text` にそれぞれ特徴的な目印の文字列を持つ `Transcript` を渡すと、4 つの目印はそれぞれ、`LLMClient` に渡る system プロンプトと user プロンプトの少なくとも一方に現れる。また、既定のテンプレートの文面は、先頭行を `# ` で始まるタイトルの見出しにするという F-005 の出力の形の指示を含む。既定のテンプレートからこれらのいずれかを取り除くと、この AC の検証は失敗する（検証の方法は [implementation_handoff.md](implementation_handoff.md) I-01）。

#### F-002: `Transcript` の検証

`Write` は、受け取った `Transcript` を検証してからプロンプトを組み立てる。`Transcript` は exported なフィールドを持つ構造体であり、`TranscriptSource` 以外からも組み立てられる。そこで、出典リンクの元になる値と、プロンプトに埋め込む値を `ArticleWriter` の側でも検証する。

-   次のいずれかに当てはまる `Transcript` は、`ErrInvalidTranscript` で拒否する。
    -   `VideoID` が `[A-Za-z0-9_-]{11}` に完全一致しない。
    -   `VideoURL` が `https://www.youtube.com/watch?v=<VideoID>` と完全に一致しない（`TranscriptSource` が返す正規化した URL の形。[security.md](../../dev/security.md) §1）。別の形の URL を正規化して受理することはしない。
    -   `Segments` が空である。
    -   いずれかの `Segment.Text` が、空、または空白文字だけである。
    -   `Title`・`ChannelName`・`Description`・各 `Segment.Text` のいずれかが、正しい UTF-8 の文字列でない。
-   `Title`・`ChannelName`・`Description` が空であることは拒否理由にしない。
-   拒否した場合は、`LLMClient` を呼ばない。

**Acceptance Criteria**:
- **AC-07**: 次のいずれかに当てはまる `Transcript` に対し、`Write` は `errors.Is(err, ErrInvalidTranscript)` が真になるエラーを返し、`LLMClient` を呼ばない。
    -   `VideoID` が不正（例: 空、10 文字、`/` や `..` を含む）
    -   `VideoURL` が正規化した形と一致しない（例: `https://youtu.be/<VideoID>`、`http://www.youtube.com/watch?v=<VideoID>`、末尾に `&t=1s` が付いたもの、別の動画 ID のもの、空）
    -   `Segments` が空
    -   `Text` が空または空白文字だけの `Segment` を含む
    -   `Title`・`ChannelName`・`Description`・`Segment.Text` のいずれかに不正な UTF-8 のバイト列を含む（それぞれについて確認する）

    いずれの場合も、返る `Article` はゼロ値である。
- **AC-08**: `Title`・`ChannelName`・`Description` が空で、他の検証を通る `Transcript` に対し、`Write` は `ErrInvalidTranscript` を返さず、`LLMClient` を呼ぶ。

#### F-003: プロンプトの組み立てと LLM の呼び出し

検証を通った `Transcript` から system プロンプトと user プロンプトを組み立て、`LLMClient.Generate` を 1 回呼ぶ。

-   字幕本文は、`Segments` の各 `Text` を順序どおりに改行（`\n`）1 つで区切って連結した文字列とする。各 `Text` は加工しない。`StartMs`（タイムスタンプ）は字幕本文に含めない。
-   テンプレートに埋め込む動画タイトル・チャンネル名・概要欄は、`Transcript` の値を加工せずに使う。
-   埋め込む値はテンプレートとして解釈しない。字幕や概要欄が `{{` などのテンプレートの構文に見える文字列を含んでいても、その文字列はそのままプロンプトに現れる。
-   展開した system プロンプトと user プロンプトのそれぞれの大きさ（バイト数）には上限を設ける（2.3）。上限値は `02_architecture.md` で固定する。テンプレートの小さな記述が巨大な出力に膨らむ場合も含め、上限を超える展開は、メモリを使い切ることなくエラーになる（手段は [design_handoff.md](design_handoff.md) H-11）。
-   テンプレートの展開がエラーになった場合、展開したプロンプトが上限を超えた場合、および組み立てた system プロンプトまたは user プロンプトが空、または空白文字だけになった場合は、`LLMClient` を呼ばずに `ErrInvalidTemplate` で失敗する。
-   `GenerateRequest` の `MaxOutputTokens` は 0（プロバイダの既定に任せる）とする（2.3）。
-   `LLMClient.Generate` には、`Write` が受け取った `ctx` を渡す。
-   `Generate` の呼び出しは 1 回だけとし、リトライしない。

**Acceptance Criteria**:
- **AC-09**: `Write` が成功した場合、`LLMClient.Generate` はちょうど 1 回呼ばれ、`Write` に渡した `ctx` を受け取る。`GenerateRequest` の `MaxOutputTokens` は 0 である。
- **AC-10**: テスト用のテンプレート（4 つの値をそれぞれ目印で囲んで埋め込むもの）を system 用・user 用の上書きファイルとして構築した `ArticleWriter` に対し、`LLMClient` に渡る system プロンプトと user プロンプトのそれぞれで、目印の間に `Title`・`ChannelName`・`Description` と同一の文字列、および各 `Segment.Text` を `\n` で連結した文字列が現れる。前後の空白や改行を含む `Text` も変更されない。
- **AC-11**: `StartMs` に特徴的な値（例: `987654321`）を持つ `Segment` から組み立てたプロンプトには、その値の文字列が現れない（テンプレートの文面自体がその文字列を含まない場合）。
- **AC-12**: `Title`・`Description`・`Segment.Text` に `{{.Title}}`・`{{printf "%s" "x"}}`・`{{` などのテンプレートの構文に見える文字列を含む `Transcript` に対し、`LLMClient` に渡るプロンプトにはその文字列がそのまま現れ、展開されない。
- **AC-13**: 構築時の検証は通るが、特定の `Transcript` では展開に失敗するテンプレートを使うと、`Write` は `errors.Is(err, ErrInvalidTemplate)` が真になるエラーを返し、`LLMClient` を呼ばない。失敗には次の 2 種類がある。
    -   展開結果が空白文字だけになる（例: 動画タイトルだけを埋め込むテンプレートと、空白文字だけの `Title`）。
    -   展開がエラーになる（例: `{{index .Title 100}}` と、100 バイト未満の `Title`）。
- **AC-28**: 展開した system プロンプトまたは user プロンプトの大きさについて、次が成り立つ。
    -   ちょうど上限のバイト数になるプロンプトは受理し、`LLMClient` を呼ぶ。
    -   上限＋1 バイトになるプロンプトは、`errors.Is(err, ErrInvalidTemplate)` が真になるエラーになり、`LLMClient` を呼ばない。
    -   `{{printf "%1000000000s" .Title}}` のように展開結果が上限を大きく超えるテンプレートは、メモリを使い切らずに `errors.Is(err, ErrInvalidTemplate)` が真になるエラーになり、`LLMClient` を呼ばない。エラーは構築時に返しても、`Write` の時点で返してもよい（どちらにするかは設計で決める。[design_handoff.md](design_handoff.md) H-11）。

#### F-004: LLM の失敗の報告

-   `LLMClient.Generate` がエラーを返した場合、`Write` はそのエラーをラップして返す。呼び出し元が `errors.Is` で `llm` パッケージの番兵（`ErrTruncated`・`ErrEmptyResponse` など）、プロバイダ固有の番兵、`context.DeadlineExceeded`・`context.Canceled` を判別できるようにする。
-   `ctx` が既にキャンセルされている場合は、`LLMClient` を呼ばずに、`errors.Is(err, context.Canceled)` が真になるエラーを返す（期限切れの場合は `context.DeadlineExceeded`）。
-   いずれの失敗時も、部分的な結果（`LLMClient` が返したテキストなど）を含む `Article` を返さない。

**Acceptance Criteria**:
- **AC-14**: fake の `LLMClient` が `llm.ErrTruncated`・`llm.ErrEmptyResponse`・`context.DeadlineExceeded`・テスト用の独自の番兵をそれぞれラップしたエラーを返す場合、`Write` が返すエラーに対して、それぞれの `errors.Is` が真になる。返る `Article` はゼロ値である。fake が空でない `Text` をエラーとともに返した場合も同様である。
- **AC-15**: 呼び出し前にキャンセルした `ctx` で `Write` を呼ぶと、`LLMClient` を呼ばずに `errors.Is(err, context.Canceled)` が真になるエラーを返す。期限が既に過ぎた `ctx` で呼ぶと、`LLMClient` を呼ばずに `errors.Is(err, context.DeadlineExceeded)` が真になるエラーを返す。いずれの場合も、返る `Article` はゼロ値である。

#### F-005: 生成テキストの検証と変換

`LLMClient` が返した生成テキスト（`GenerateResponse.Text`）は信頼できない入力である。3.2 の規則で検証し、タイトルと本文に分けて `Article` に変換する。

-   生成テキストの先頭行は、レベル 1 の ATX 見出し（`# ` で始まる行）でなければならない。その見出しの内容をタイトルとし、見出しの行より後をすべて LLM が生成した本文とする。
-   受理する形と拒否の規則は 3.2 に定める。拒否した場合は `ErrMalformedOutput` を返す。
-   `Article` の各フィールドは次のとおりとする。
    -   `Title`: 見出しから取り出したタイトル。
    -   `Body`: LLM が生成した本文の末尾に、出典ブロック（F-006）を付けた Markdown。
    -   `SourceURL`: 出典リンクの URL（F-006）。
    -   `Model`: 3.2 の検証を通った `GenerateResponse.Model` の値を加工せずに入れる。
    -   `ModelVersion`: 3.2 の検証を通った `GenerateResponse.ModelVersion` の値を加工せずに入れる。空文字列も受理する（モデルの版を返さないプロバイダがあるため）。
-   いずれの拒否時も、部分的な結果を含む `Article` を返さない。

**Acceptance Criteria**:
- **AC-16**: 生成テキストが `# 見出しのタイトル\n\n本文の段落\n` で、`Model` が `m-1`、`ModelVersion` が `fp-1` の応答に対し、`Write` はエラーを返さず、`Title` が `見出しのタイトル`、`Body` が `\n本文の段落\n` で始まり出典ブロックで終わる文字列、`Model` が `m-1`、`ModelVersion` が `fp-1` の `Article` を返す。`ModelVersion` が空文字列の応答も受理し、`Article.ModelVersion` は空文字列である。
- **AC-17**: `Article.Body` は、生成テキストの先頭行の `\n` より後の文字列（LLM が生成した本文）と同一の文字列で始まる。前後の空白や改行、本文中の Markdown（見出し・リスト・リンクなど）は変更されない。`Article.Body` は、LLM が生成した本文、区切り（空行を作るための改行）、出典ブロックをこの順に連結した文字列であり、これ以外の文字列を含まない。

#### F-006: 出典リンクの付与

記事の末尾に、元動画への出典リンクを、LLM の出力に依存せずコードで付与する。

-   出典リンクの URL は、検証済みの `VideoID` から組み立てた `https://www.youtube.com/watch?v=<VideoID>` とする（F-002 により `Transcript.VideoURL` と一致する）。この URL を `Article.SourceURL` に入れる。
-   出典ブロックは、固定の見出し語（例: `出典`）と出典リンクの URL からなる、Markdown の独立した段落とする。正確な書式は設計で固定する。LLM が生成した本文がどのような文字列で終わっても（改行で終わらない場合を含む）、出典ブロックは直前の段落とつながらない。
-   出典ブロックは `Body` の最後に置き、その後には何も付けない。
-   LLM が生成した本文が出典や URL を既に含んでいても、出典ブロックを省略しない。生成テキスト中の URL を出典リンクとして採用することもしない。

**Acceptance Criteria**:
- **AC-18**: `Write` が成功した場合、`Article.SourceURL` は `https://www.youtube.com/watch?v=<VideoID>` であり、`Article.Body` は `02_architecture.md` で固定した書式の出典ブロック（`SourceURL` を含む）で終わる。
- **AC-19**: LLM が生成した本文が、改行で終わらない場合、改行が続いて終わる場合、出典らしき行や別の YouTube の URL（例: `出典: https://www.youtube.com/watch?v=XXXXXXXXXXX`）を含む場合のいずれでも、AC-18 が成り立ち、出典ブロックの直前には空行がある。出典ブロックの URL は生成テキスト中の URL ではなく、`Transcript.VideoID` から組み立てたものである。

#### F-007: テスト可能性

**Acceptance Criteria**:
- **AC-21**: `ArticleWriter` のユニットテストは、LLM の API もネットワークも呼ばず、テストの外にあるファイルにも依存しない。fake の `LLMClient` を使い、F-001〜F-006 と 3.2 の各 AC を検証する（テストの組み立て方は [implementation_handoff.md](implementation_handoff.md) I-01）。

### 3.2. 信頼できない入力の境界 (Untrusted Input Boundaries)

`ArticleWriter` が受け取る値のうち、`GenerateResponse` は LLM の出力であり信頼できない入力である。`Transcript` は字幕と info.json に由来し、`TranscriptSource` が検証済みだが、F-002 の規則で再度検証する。テンプレートの上書きファイルは利用者のローカルのファイルであり、F-001 の規則で検証する。

**`GenerateResponse` の受理する形:**

`ArticleWriter` が消費するのは `Text`・`Model`・`ModelVersion` の 3 つである。`GenerateResponse` は Go の構造体であり、JSON のように、メンバーの重複や未知のメンバーが生じることはない。`Text` は、次の形の規則でタイトルと本文の部分に分ける。

-   `Text` の先頭行（最初の `\n` の前まで。`\n` を含まない場合は全体）は、`# `（`#` 1 個と半角空白 1 個）で始まる。先頭行の前に空行や空白を置いた形、`#` が 2 個以上の見出し、`#` の直後が半角空白でない形（例: `#タイトル`）、コードフェンスで囲んだ形（例: 先頭行が ```` ```markdown ````）、Setext 形式の見出し（`===` の下線）は受理しない。
-   タイトルは、先頭行の `# ` を取り除き、さらに前後の半角空白とタブを取り除いた文字列とする。これ以外の正規化はしない。タイトルは `#` で終わらない（閉じの `#` 列を解釈も除去もしないため、`# タイトル #` の形は拒否する）。
-   本文の部分は、先頭行の `\n` より後とする。生成テキストが `\n` を含まない場合、本文の部分は空である。

各値は次の規則をすべて満たす。

| 値 | 正しい UTF-8 | 大きさ（バイト数）の上限 | 必須か | メタ情報の文字列の規則 |
|---|---|---|---|---|
| `Text` | 要 | あり | —（タイトルと本文の部分の規則による） | 適用しない |
| タイトル | 要 | `Text` の上限に含まれる | 必須 | 適用する |
| 本文の部分 | 要 | `Text` の上限に含まれる | 必須 | 適用しない |
| `Model` | 要 | あり | 必須 | 適用する |
| `ModelVersion` | 要 | あり | 任意（空文字列を受理する。空でなければ必須と同じ条件を満たす） | 適用する |

-   **上限:** `Text` の上限と、`Model`・`ModelVersion` の上限の値は `02_architecture.md` で固定する。ちょうど上限の値は受理し、上限を超える値は拒否する。
-   **必須:** 空白文字（§6）以外の文字を 1 つ以上含むことをいう。
-   **メタ情報の文字列**の規則: 制御文字（Unicode の一般カテゴリ Cc の文字。タブ・`\r`・`\n`・ESC（`\x1b`）を含む）を含まない。タイトル・`Model`・`ModelVersion` は、記事の見出しや生成に使ったモデルの記録として、利用者の端末や投稿先にそのまま表示される。そこに端末の制御シーケンスや、改行による偽の行を持ち込ませないためである。

本文の部分には、さらに次の規則を適用する。後ろに付ける出典ブロックが隠れたり、リンクとして表示されなくなったりしないようにするためである。

-   生の HTML を含まない。対象は、CommonMark が生の HTML とみなすもの（ブロックとしてもインラインとしても）のすべてである。コードスパンやコードフェンスの中にある、HTML に見える文字列は生の HTML ではなく、受理する。`<https://example.com/>` のような Markdown の自動リンクも生の HTML ではない。コラム記事に生の HTML は要らず、Slack も生の HTML を表示しない。一方、閉じていない要素（`<details>` など）は、後ろに付ける出典ブロックを中に取り込んで隠しうる。
-   閉じていないコードフェンス（行頭の ```` ``` ```` または `~~~` で開き、閉じていないもの）で終わらない。開いたままだと、出典ブロックがコードブロックの中に入る。

判定の詳細（CommonMark への準拠の程度、境界の例）は設計で決める（[design_handoff.md](design_handoff.md) H-04）。

**拒否時:** 上記に合致しない入力は、補正・正規化・切り詰めをせずに `ErrMalformedOutput` で拒否し、部分的な結果を返さない。エラーには拒否した理由（どの規則に反したか）を含め、いずれの値（タイトル・本文の部分・`Model`・`ModelVersion`）も含めない（信頼できない入力であり、利用者の端末へそのまま出力しないため）。

| 境界 | 受理する形 | 拒否時の番兵 | 主な AC |
|---|---|---|---|
| テンプレート（既定・上書きファイル） | F-001・F-003 | `ErrInvalidTemplate` | AC-05・AC-06・AC-13・AC-28・AC-30 |
| `Transcript` | F-002 | `ErrInvalidTranscript` | AC-07・AC-08 |
| `GenerateResponse` | 3.2 | `ErrMalformedOutput` | AC-22〜AC-27・AC-29・AC-31 |

各 AC に挙げた入力例は、要件の作成時に確認した具体例であり、拒否すべき入力をすべて挙げたものではない。拒否すべき入力の範囲は上の規則が定め、その具体化は設計で行う。

**Acceptance Criteria**:
- **AC-22**: 生成テキストが次のいずれかである応答に対し、`Write` は `errors.Is(err, ErrMalformedOutput)` が真になるエラーを返し、返る `Article` はゼロ値である。
    -   空文字列、または空白文字だけ
    -   不正な UTF-8 のバイト列を含む
    -   先頭行が見出しでない（例: `タイトル\n本文`）
    -   先頭に空行がある（例: `\n# タイトル\n本文`）
    -   レベル 2 以上の見出しで始まる（例: `## タイトル\n本文`）
    -   `#` の直後が半角空白でない（例: `#タイトル\n本文`）
    -   コードフェンスで始まる（例: ```` ```markdown\n# タイトル\n本文\n``` ````）
- **AC-23**: タイトルが空または空白文字だけ（例: `# \n本文`、`#  \t\n本文`、全角空白（U+3000）だけの `# 　\n本文`）、`\r` を含む（例: `# タイトル\r\n本文`）、`#` で終わる（例: `# タイトル #\n本文`）生成テキスト、および本文の部分が空または空白文字だけの生成テキスト（例: `# タイトル`、`# タイトル\n`、`# タイトル\n \n\t\n`）は、`errors.Is(err, ErrMalformedOutput)` が真になるエラーになる。タイトルの前後の半角空白とタブは除き（例: `#  タイトル  \n本文` のタイトルは `タイトル`）、拒否しない。
- **AC-24**: 本文の部分が閉じていないコードフェンスで終わる生成テキスト（例: `# タイトル\n本文\n` に続けて ```` ```go\nfmt.Println() ```` で終わるもの、`~~~` で開いて閉じないもの）は、`errors.Is(err, ErrMalformedOutput)` が真になるエラーになる。フェンスを閉じている生成テキストは受理する。
- **AC-25**: `Model` が空文字列の応答は、生成テキストが正しくても、`errors.Is(err, ErrMalformedOutput)` が真になるエラーになる。
- **AC-26**: AC-22〜AC-25・AC-27・AC-29・AC-31 の `ErrMalformedOutput` の各拒否ケースで、返るエラーを `Error()` で文字列にした結果には、応答のタイトル・本文の部分・`Model`・`ModelVersion` のいずれの値も現れない（検証の方法は [implementation_handoff.md](implementation_handoff.md) I-01）。
- **AC-27**: 本文の部分が生の HTML を含む生成テキストは、`errors.Is(err, ErrMalformedOutput)` が真になるエラーになり、返る `Article` はゼロ値である。例: 閉じていない `<details>`、閉じている `<details>…</details>`、`<div hidden>`、段落中のインラインの `<span hidden>`、`<!-- -->`、`<script>`。一方、コードスパンの中の `<details>`（例: `` `<details>` ``）、閉じたコードフェンスの中の `<details>`、自動リンク（例: `<https://example.com/>`）を含む生成テキストは受理する。
- **AC-29**: 生成テキストが正しくても、次のいずれかに当てはまる応答は、`errors.Is(err, ErrMalformedOutput)` が真になるエラーになり、返る `Article` はゼロ値である。
    -   `Model` が空白文字だけ（例: 全角空白（U+3000）だけの `　`）
    -   `Model` が不正な UTF-8 のバイト列を含む
    -   `Model` が制御文字を含む（例: `m\x1b[31m`、`m-1\nfake`）
    -   `ModelVersion` が空でなく、空白文字だけ（例: 半角空白 1 個）
    -   `ModelVersion` が空でなく、不正な UTF-8 のバイト列を含む
    -   `ModelVersion` が制御文字を含む（例: `fp\x1b[0m`、`fp-1\n`）

    エラーに値が現れないことは AC-26 で確かめる。
- **AC-31**: `Text`・`Model`・`ModelVersion` のそれぞれについて、ほかの規則を満たし、大きさがちょうど上限のバイト数の値は受理する。上限＋1 バイトの値は `errors.Is(err, ErrMalformedOutput)` が真になるエラーになり、返る `Article` はゼロ値である。

## 4. 非機能要件 (Non-Functional Requirements)

### 4.1. 性能 (Performance)

-   LLM の API 呼び出しが実行時間を支配するため、本タスクにローカル処理の性能要件はない。

### 4.2. セキュリティ (Security)

-   [security.md](../../dev/security.md) §4（LLM プロバイダへ送るデータ）・§6（信頼できないテキスト）に従う。
-   プロンプトに入るのは、テンプレートの文面と、`Transcript` の動画タイトル・チャンネル名・概要欄・字幕本文だけである。`ArticleWriter` は、上書きファイルのパスなどのローカルの情報をプロンプトに加えない（F-001・F-003）。
-   出典リンクは LLM の出力に依存せず、検証済みの動画 ID からコードで組み立てる（F-006・AC-19）。
-   LLM の出力は 3.2 の規則で検証し、補正せずに拒否する。拒否のエラーに生成テキストの内容や `Model`・`ModelVersion` の値を含めない（AC-26）。
-   字幕・概要欄などの値をテンプレートとして解釈しない（AC-12）。
-   生成テキストをコマンドやコードとして実行しない。`ArticleWriter` は生成テキストを文字列として `Article` に入れるだけである。

### 4.3. 信頼性・可用性 (Reliability/Availability)

-   空の出力・記事に変換できない出力・`LLMClient` の失敗を、成功として返さない（F-004・F-005・3.2）。
-   テンプレートの誤りは構築時に報告し、字幕の取得や LLM の呼び出しの後まで持ち越さない（F-001・AC-05）。
-   上書きファイルの読み込みとテンプレートの展開は、待ち続けたりメモリを使い切ったりせずにエラーを返す（F-001・F-003・AC-05・AC-28）。

### 4.4. 互換性 (Compatibility)

-   macOS と Linux でビルド・テストできること。
-   Go のバージョンは `go.mod` に従う。

### 4.5. 保守性 (Maintainability)

-   LLM プロバイダを追加しても `ArticleWriter` を変更せずに済むこと。`ArticleWriter` は `llm.LLMClient` と `llm` パッケージの共通型だけに依存し、`internal/llm/<provider>` を import しない。
-   プロンプトの文面はテンプレートファイルに置き、Go のコードに書かないこと。
-   標準ライブラリ以外のモジュールを追加しないこと（`.golangci.yml` の depguard `deps` ルール）。
-   Go のコメント・識別子・文字列リテラルは英語で書くこと。テンプレートの文面（日本語）はこの規則の対象外とする。

## 5. 制約条件 (Constraints)

-   [project_overview.md](../../dev/project_overview.md) の「決定済みの方針」「前提・制約」に従う。
-   ユニットテストは LLM の API・ネットワークを呼ばない（AC-21）。
-   上書きファイルのパスを環境変数や CLI 引数から受け取るのは #6 の責務とする。本タスクの `ArticleWriter` は、パスを構築時に受け取る。
-   番兵エラー（`ErrInvalidTemplate`・`ErrInvalidTranscript`・`ErrMalformedOutput`。名前は仮称で、設計で確定する）は `internal/writer` に置く。

### 5.1. 他の文書との差分

-   `writer.Article` のフィールドは、`0001_pipeline_skeleton` で `Title`・`Body`・`SourceURL`・`Model` の 4 つと定め、既存のテストがこの組を固定している。本書は `ModelVersion`（文字列）を追加する（F-005）。`0003_deepseek_llm_client` が `llm.GenerateResponse` に `ModelVersion` を追加し、`writer.Article` への記録を本タスク（#5）に委ねたためである。この組を固定している既存のテストと、[project_overview.md](../../dev/project_overview.md) の `ArticleWriter` の説明（`Article` の項目の列挙）を更新する（[implementation_handoff.md](implementation_handoff.md) I-02）。
-   [project_overview.md](../../dev/project_overview.md) の決定済みの方針は変更しない。

## 6. 用語集 (Glossary)

用語は [translation_glossary.md](../../translation_glossary.md) と統一する。本タスクで新たに使う用語は次のとおり。

-   **プロンプトテンプレート（prompt template）:** system プロンプトまたは user プロンプトの文面を、Go の `text/template` の構文で書いたもの。既定のものは `prompts/` に置き、バイナリに埋め込む。
-   **上書きファイル（override file）:** 既定のプロンプトテンプレートの代わりに使う、利用者のローカルのテンプレートファイル。パスを構築時に与える。
-   **字幕本文（transcript text）:** `Transcript.Segments` の各 `Text` を、タイムスタンプを除いて改行で連結した文字列（F-003）。
-   **生成テキスト（generated text）:** `LLMClient` が返す `GenerateResponse.Text`。先頭行のタイトルの見出しと、LLM が生成した本文からなる。
-   **出典ブロック（source block）:** 記事の末尾にコードで付ける、出典リンクを示す Markdown の段落（F-006）。
-   **番兵エラー（sentinel error）:** `errors.Is` で判別できる、あらかじめ定めたエラー値。
-   **空白文字（whitespace）:** Unicode の White_Space プロパティを持つ文字（半角空白・タブ・改行・全角空白など）。本書の「空、または空白文字だけ」の判定はすべてこの定義による（[design_handoff.md](design_handoff.md) H-10）。
-   **`ErrInvalidTemplate` / `ErrInvalidTranscript` / `ErrMalformedOutput`:** それぞれ、不正なプロンプトテンプレート（読めない上書きファイル、通常のファイルでない上書きファイル、展開結果が空になる場合や上限を超える場合を含む）・不正な `Transcript`・記事に変換できない `GenerateResponse` を表す番兵エラー（名前は仮称で、設計で確定する）。
