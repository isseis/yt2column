# 実装計画書：パイプラインの中間生成物の保存と再開

## Document Status

| Item | Value |
|---|---|
| Status | `draft` |
| Created | 2026-10-14 |
| Review date | - |
| Reviewer | - |
| Comments | - |

本計画の F-NNN・AC-NN は [01_requirements.md](01_requirements.md)（以下「requirements」）の項番、H-NN は [design_handoff.md](design_handoff.md) の項目、節番号は [02_architecture.md](02_architecture.md)（以下「architecture」）の節を指す。既存のファイルの行番号は、断りがない限り HEAD `65868ea` で確かめた。Go のソースのコメント・識別子・文字列リテラルは英語で書き（`_context.md`）、`AC-NN`・`F-NNN`・`H-NN` は Go ソースに書かず本計画にだけ記録する（`requirements_process.md` §4）。

## 1. 実装の概要 (Implementation Overview)

### 1.1. 目的

requirements が定める機能要件（F-001〜F-008）を、architecture の設計どおりに実装する。パイプラインの各段階（生成・字数の調整・検証・推敲）の出力を「成果物」として `--artifacts` のディレクトリへ保存し、`--after generation|shorten|review` で保存した段階から再開して、その後の段階だけを実行できるようにする。

実装は architecture §8 の実装優先順位に従い、6 つのフェーズに分ける。フェーズ 1〜5 が本タスクの作業単位であり、フェーズ 6（評価の再実行）は requirements §2.3 のとおり本タスクの対象外である。用語（段階、中間生成物、成果物、来歴、入力の識別子、成果物の識別子、素材、文面、文面の文字列、テンプレートの識別子）は architecture §1 の定義に従う。

### 1.2. 実装原則

- architecture §1.1 の設計原則 9 項目に従う。新しいパッケージは `internal/artifact` の 1 つだけであり、`internal/job`・`internal/pipeline`・`internal/publisher`・`internal/llm/provider` は変更しない（architecture §1.3・§2.1）。
- 保存するかどうかと再開する段階は、`writer.Options` の `Artifacts` で明示する（原則 2・3）。設定の値や保存済みの中間生成物の内容から実行する段階を推測しない（「Declare, don't infer」）。
- 成果物の形式・識別子・上限・読み書きは `internal/artifact` の 1 か所で定義し、保存と再開は同じ定義を読む（原則 4）。
- 再開の入力は信頼できない入力として境界で検査し、補正しない（原則 5）。文面は `checkGeneratedText`、一覧は検証・推敲の応答の規則、JSON の構文は `internal/strictjson` を再利用する。壊れた入力は部分受理しない。
- 各フェーズの完了時に、`_context.md` の "Build checks"（`make fmt` → `make test` → `make lint`）を通す。グリーンゲートは `make test && make lint`。`make test` は `-tags test` で走る（`Makefile:68-69`）ので、新しいテストのファイルには既存と同じく `//go:build test` を付ける。
- 外部ライブラリは追加しない。成果物の形式は `encoding/json` と `internal/strictjson`、ダイジェストは `crypto/sha256` で扱う（requirements §5）。

### 1.3. 既存コード調査結果

**`internal/writer/writer.go`（フェーズ 1・2 で変える）。** `Article`（`:17-25`）は `==` で比較できる契約を持つ（`:15-16`）ので、成果物を保持するフィールドを足さない（architecture §3.5）。`Options`（`:44-49`）に `Artifacts ArtifactOptions` を加える。`templateWriter`（`:89-95`）と `stepTemplates`（`:71-84`）にテンプレートの識別子を、`templateWriter` に保存と再開の値を加える。`New`（`:101-142`）の nil クライアントの検査（`:102-104`）と `validateStepOptions`（`:148-163`）を architecture §3.1 の組み合わせ検査に改める。`Write`（`:193-240`）に、`Artifacts.Resume` からの段階の導出・`Store.Load`・段階の完了時の `Store.Save`・`Article` の組み立て（architecture §3.5・§3.6）を組み込む。`loadStepTemplates`（`:174-184`）はテンプレートの識別子も返す。

**`internal/writer/output.go`（フェーズ 1 で変える）。** `checkGeneratedText`（`:44-78`）・`checkModelPair`（`:108-116`）・`checkModelString`（`:120-131`）・`textString`（`:167-169`）は `writer` の非公開関数である。成果物の文面とモデル名の検査に再利用するため、公開の補助関数（`CheckStageText`・`CheckModelText`）をこのファイルに加える（architecture §3.2）。`checkGeneratedText` の規則と `maxTextBytes`（`:14`）・`maxModelBytes`（`:16`）は変えない。

**`internal/writer/review.go`・`revision.go`（フェーズ 1 で変える）。** `parseReviewResponse`（`review.go:38-70`）は一覧の規則（`revisionKeys` `review.go:31`、要素数 `maxRevisions` `revision.go:11`、`parseRevision` `review.go:75-125`）を持つ。この一覧の検査を公開の補助関数に切り出し、`parseReviewResponse` と `internal/artifact` の両方が呼ぶ（architecture §3.2 の H-08。規則を 2 か所に持たない）。`ReviewResult`（`revision.go:14-17`）・`Revision`（`revision.go:43-49`）は再開の入力の型と一覧の保存形式に再利用する。

**`internal/writer/template.go`（フェーズ 1 で変える）。** `checkedTemplate[T]`（`:86-88`）に、テンプレートのテキストのダイジェストと、埋め込みの既定か上書きファイルかの真偽値を持たせる（architecture §3.2）。値を設定するのは読み込み時（`loadTemplate` `:92-102`・`parseTemplate` `:145-167`）である。上書きファイルのパスは記録しない（requirements §4.2）。`readOpenedFile`（`:126-139`）の「開いた記述子で種別と大きさを確かめる」考え方は、成果物の読み込み（フェーズ 1）にも取り入れる。

**`internal/writer/prompt.go`（変更しない。再利用する）。** `validateTranscript`（`:50-82`）は `Write` が既に呼んでいる。`newTemplateData`（`:87-98`）が作る字幕の文字列（`templateData.Transcript`）は、素材の識別子（`MaterialID`）の入力に再利用する。`maxPromptBytes`（`:14`）は変更しない。

**`internal/writer` のテスト補助（フェーズ 2 で変える）。** `test_helpers.go`（`//go:build test`）の目印（`markGenTitle` など `:32-38`）・`validTranscript`（`:51-63`）・`validResponse`（`:68-73`）・`mustNew`（`:119-126`）・`withStepsEnabled`（`:99-104`）・`newStepClient`（`:108-111`）と、`internal/llm/testutil` の `FakeLLMClient`・`FakeLLMSequence`（呼び出し回数は `len(Calls)`）を再利用する。fake の `ArtifactStore` は `internal/writer` のテストだけが使うので、`test_organization.md` の分類 B（「同じパッケージのテストだけが使う補助は `test_helpers.go` に置く」）に従い `testutil/` の外（このファイル）に置く（architecture §3.11 も同じ置き場所を定める）。そのため `TestFakesCarryBuildTag` が数える `testutil/` のファイル数（`pipeline_test.go:558` の 17）は変わらない。

**`internal/writer` の既存のテスト（フェーズ 2 で通すことを確かめる）。** `writer_test.go` の `TestNewRejectsNilClient`（`:21-40`）は `Options{}`（生成を実行）で nil クライアントを拒否するので、§3.1 の変更後も内容を変えずに通る。`TestNewStepOptions`（`:194-236`）の「review disabled with a client」（`:203`）は「実行しない段階のクライアントが nil でない」拒否に当たり、これも通る。`steps_test.go`・`output_test.go` は `Options{}` または段階を有効にした `Options` を使い、`Article` を `==` で比べる（`steps_test.go:38`・`output_test.go:75-82`）。`Artifacts` のゼロ値は保存も再開もしないので、これらの期待は変わらない（AC-32）。

**`internal/config/config.go`（フェーズ 3 で変える）。** `Load`（`:204-222`）を `LoadFor(lookup, Need{Generation:true, Review:true})` に委譲する。`loadProvider`（`:226-234`）・`parseProvider`（`:243-256`）・`loadReview`（`:264-297`）・`loadProviderVars`（`:308-321`）・`loadModel`（`:324-331`）と API キー・`YT2COLUMN_CLAUDE_EFFORT`・`ANTHROPIC_WORKSPACE_ID` の読み込み（`:336-401`）を、`Need` で絞る（architecture §3.7）。環境変数の読み込みと、API キーなどの秘密情報の変数名は `internal/config` の中に限られる（`envaccess_test.go` が許す。`internal/config` の中の新しいコードはこの規則を破らない）。`config_test.go` の `envLookup`（`:22-27`）・`validEnv`（`:30-39`）・`validClaudeEnv`（`:43-52`）・`with`（`:55-59`）・`without`（`:62-68`）・`assertVarError`（`:578-590`）を再利用する。`Load` を呼ぶ既存のテストは `Need` の両方が真のときの期待なので、内容を変えない（`TestLoadIgnoresOtherProviderVars` `:380-457` と `TestLoadClaudeIgnoresDeepSeekKey` `:341-375` がプロバイダごとの絞り込みを固定する）。`Need.Generation` が偽でも、検証・推敲のプロバイダとモデルの既定は既存の継承規則（`loadReview` `:264-297`。`YT2COLUMN_REVIEW_PROVIDER` が未設定なら `YT2COLUMN_LLM_PROVIDER` を、`YT2COLUMN_REVIEW_MODEL` が未設定なら `YT2COLUMN_MODEL` を引き継ぐ）で解決する必要がある。`LoadFor` はこの既定の解決のために `YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL` を読むが、生成を実行しないときはそれらの未設定を必須の誤りとしない（ステップ 3-1・3-2）。

**`internal/strictjson`（変更しない。再利用する）。** `ParseObject`（`:67-80`）が UTF-8・サロゲート・トップレベルのオブジェクト・末尾の内容を検査し、`CollectOnly`（`:104-116`）が未知のキーと重複したキーを、`AsBool`（`:190-203`）が真偽値の型を、`AsInt64`（`:207-224`）が整数を、`AsArray`（`:240-267`）・`AsObject`（`:227-236`）が配列とオブジェクトの形を検査する。`Required`・`RequiredString`・`OptionalString`（`:130-171`）を成果物の必須・任意のキーに使う。

**`cmd/yt2column/run.go`（フェーズ 4 で変える）。** `newFlagSet`（`:132-150`）に `--artifacts` と `--after` を加え、`cliOptions`（`:113-127`）にフィールドを加える。`flagGiven`（`:153-161`）・`chooseDestination`（`:306-330`）・`checkReviewLog`（`:338-352`）の「どのフラグが現れたかで判定し、値では判定しない」形を組み合わせの検査にも取り入れる。`run`（`:203-299`）に、`Need` の算出と、`--artifacts` が指定されたときの `artifact.NewStore(opts.artifacts)` の呼び出しを加える。`NewStore` の失敗は「実行の失敗」（終了コード 1）であり、`run` は `buildWriter` の失敗（使い方の誤り、終了コード 2）と区別して扱う（ステップ 4-4。`buildWriter` は `run` から渡された `Store` を `writer.Options` に詰めるだけで、開く処理を持たない）。`buildWriter`（`:357-393`）で `writer.Options` に `Artifacts` を渡し、実行する段階の LLM クライアントだけを作って、作らないクライアントは nil で渡す（architecture §3.7）。`outPathInsideCacheDir`（`outpath.go:29`）を AC-08 の判定に再利用する。`reportSuccess`（`:444-464`）に、保存した成果物のディレクトリを報告する 1 行を加える。

**`cmd/yt2column` のテスト（フェーズ 4・5 で変える）。** `run_test.go` の `runEnv`（`:88-106`）・`newRunEnv`（`:113-150`）・`cliArgs`（`:260-269`）・`seedCache`（`:69-77`）・`lookupFrom`（`test_helpers_integration.go:90-95`）・`productionDeps` の差し替えの形（`run_test.go:1277, 1289`）を再利用して、成果物と再開のテストを加える。`docs_test.go` の `TestREADMEDocumentsFlags`（`:37-52`）が `newFlagSet` のすべてのフラグを README の表に求めるので、フラグを加えた時点で README の表にも行を加える。`docs_test.go` の `readDoc`（`:304-311`）・`findTable`（`:368-380`）・`sectionByHeading`（`:455-478`）を、新しい文書の契約の guard に再利用する。

**guard（`internal/pipeline/pipeline_test.go`）。** `TestCommonTypesFieldSets`（`:378-427`）は `Article`（`:406-414`）を含む共通の型の exported フィールドを固定する。`Article` を変えないのでこの guard は変更しない（architecture §3.5）。`TestFakesCarryBuildTag`（`:534-582`）の `testutil/` のファイル数（`:558` の 17）は、fake の `ArtifactStore` を `internal/writer/test_helpers.go` に置くので変わらない。`TestPackageReferenceListsPackages`（`:610-671`）は非テストのソースを持つすべてのパッケージに行を求めるので、`internal/artifact` を作るフェーズ 1 で `package_reference.md` に行を加える（`architecture §3.11`）。`TestWriterImports`（`:771-809`）は `internal/writer` がネットワークと LLM プロバイダを import しないことを固定する。`internal/writer/artifact.go` はこの規則の中で書く（`context`・`crypto/sha256`・`encoding/hex`・`internal/strictjson`・`internal/transcript` だけを使い、ネットワークも `internal/llm/` の実装も import しない）。`TestInterfaceContracts`（`:430-452`）は 4 つの単一メソッドのインタフェースだけを対象とし、`ArtifactStore` は 2 メソッドなので対象に加えない。

**`internal/job`・`internal/pipeline`・`internal/publisher`・`internal/llm/provider`（変更しない）。** `job.Run` は `writer.Options` を参照せず、組み立て済みの `writer.ArticleWriter` を受け取る。`publisher.renderArticle`（`file.go:203-214`）は `Article` からモデルの行を作るので、再開では `Article` に入れた再利用した来歴がそのまま出る（architecture §3.5・§3.9）。`job_test.go` は `Article` を `==` で比べる（`:184`・`:223`）が、`Article` を変えないので通る。

**`internal/transcript/video_id.go`（変更しない。再利用する）。** `NormalizedVideoURL`（`:64-69`）を、`Load`・`Save` が受け取った動画 ID の再検証に `internal/artifact` が使う（architecture §3.5・AC-10）。

**文書（フェーズ 5 で変える）。** `README.md` では Usage のフラグの表（`:39-54`）と `## Post-generation steps` のモデルの行の説明（`:94-99`）を更新する。`project_overview.md` では `## パイプライン`（`:18`）と `ArticleWriter` の項（`:31-33`）、`## 前提・制約` のキャッシュの項（`:58`）を更新する。`security.md` では `## 5. キャッシュディレクトリ`（`:61`）と `## 6. 信頼できないテキストの取り扱い`（`:67`。`--review-log` の規則 `:75` が前例）を更新する。`package_reference.md` では `internal/writer`・`internal/config` の行を更新する。`0010_post_generation_review/02_evaluation.md` では `## 2. 手順`（`:38`。`### 2.2` の `:48-56`・`### 2.3` の `:64-72` のコマンド例が前例）を更新する。

**まだ実装されていない部品。** `internal/artifact`・`writer` の成果物の型と補助関数・`config.Need`/`LoadFor`・CLI のフラグは、HEAD `65868ea` の時点で存在しない（`rg` で確認）。本計画がすべて新設する。

**外部の前提の確認。** モジュールは `go 1.26.5`、ツールチェーンは `go1.27.1`（`go.mod`・`go version`）。`os.OpenRoot`・`os.Root.OpenFile`・`os.Root.Rename`・`os.Root.Remove`・`os.Root.Lstat`・`os.Root.Close` の存在を `go doc os` で確認した。`OpenRoot` はディレクトリ名のシンボリックリンクを辿るので、`--artifacts` のシンボリックリンクの拒否（AC-09）は `NewStore` が `OpenRoot` の前に別に行う（architecture §3.4）。`errors.AsType` は既存コードが使う（`run.go:289`・`:540`）。

**architecture を訂正した箇所（再承認待ち）。** 承認時の architecture §3.4 は、成果物を `O_NOFOLLOW` を付けて `os.Root.OpenFile` で開けばシンボリックリンクを辿らないとしていた（AC-11・§5.1）。しかし `go1.27.1`（darwin）で確かめたところ、`os.Root` の解決は `O_NOFOLLOW` を無視し、ルート内を指す**相対**シンボリックリンク（`target` を指す `link` など）を辿って対象を返す（ルートの外を指すものだけを "path escapes from parent" で拒否する）。したがって承認時の機構では AC-11 の「シンボリックリンクを辿らない」を満たせない。同じ環境で、次のことも確かめた。`os.Root.Lstat` は相対シンボリックリンクを `Lrwxr-xr-x` として報告する。`os.Root.OpenFile` の `O_CREATE|O_EXCL` は、名前がシンボリックリンク（リンク先が無いものを含む）なら `file exists` で拒否し、リンク先を作らない。`os.Root.Rename` は置き換え先のシンボリックリンクそのものを置き換える。FIFO は `O_NONBLOCK` で開けばブロックせず、記述子の `Stat` が `p` の種別を返す。`os.SameFile` は、`Lstat` の結果と、同じファイルを開いた記述子の `Stat` を同じと判定する。`os.Root.Stat(".")` は、`OpenRoot` に渡したディレクトリの `os.Lstat` と `os.SameFile` で一致する。これに基づき、architecture §3.4 を「`Lstat` で通常のファイルに限ってから開き、開いた記述子の `Stat` と `os.SameFile` で同じファイルであることを確かめる」形に訂正し、`Status` を `draft` に戻した（decision change。§2.0）。

### 1.4. 新しい名前の一覧

本タスクが新設する名前である。以降の本文・AC の表・申し送りの対応は、この表の名前を使う（既存の名前の変更・削除はない）。

| 置き場所 | 名前 | 備考 |
|---|---|---|
| `internal/writer` | `Stage`・`StageUnset`・`StageGeneration`・`StageShorten`・`StageReview` | architecture §3.1 |
| `internal/writer` | `ArtifactOptions`（`Store`・`Resume`）・`TemplateIdentity`（`Embedded`・`Digest`）・`StageRecord`・`ArtifactChain`（`Generation`・`Shorten`・`Review`）・`ArtifactStore`（`Load`・`Save`） | architecture §3.1・§3.5 |
| `internal/writer` | `ArtifactError`（`Stage`・`Err`）と `Error`・`Unwrap` | architecture §4.1 |
| `internal/writer` | `Digest`・`MaterialID`・`CheckStageText`・`CheckModelText`・`CheckRevisionList`（一覧の配列を検査して `[]Revision` を返す） | architecture §3.2 |
| `internal/artifact` | `Store`（`Load`・`Save`・`Close`）・`NewStore`・`ErrMissing`・`ErrMismatch`・`ErrMalformed`・`maxArtifactBytes` | architecture §3.2・§4.1 |
| `internal/config` | `Need`（`Generation`・`Review`）・`LoadFor` | architecture §3.7 |
| `cmd/yt2column` | フラグ `--artifacts`・`--after`、`cliOptions.artifacts`・`cliOptions.after` | architecture §3.8 |

成果物のファイル名は `<videoID>.<stage>.json`、一時ファイルの名前は `.yt2column-artifact-<videoID>.<stage>.<ランダム>.tmp` である（architecture §3.2・§3.4）。

### 1.5. 申し送りへの対応

`design_handoff.md` の H-01〜H-11 は architecture §3.10 が対応を記録しており、本計画は architecture のとおりに実装する。`implementation_handoff.md` は存在しない。

| 項目 | 本計画での扱い |
|---|---|
| H-01 保存の形式 | フェーズ 1 のステップ 1-5・1-7（`internal/artifact/artifact.go`・`format.go`）と、文面・一覧の補助関数（ステップ 1-2・1-3） |
| H-02 保存・読み込みの置き場所と `Article` の契約 | フェーズ 1 のステップ 1-1（型）とフェーズ 2 のステップ 2-1・2-2（`Write` に置く。`Article` は変更しない） |
| H-03 保存の完了と公開の順序、中断への強さ | フェーズ 1 のステップ 1-6（一時ファイルと `rename`）とフェーズ 2 のステップ 2-2（段階の完了時に保存） |
| H-04 再開のときの設定の読み込み | フェーズ 3 |
| H-05 不正な入力の検出方法と上限の値 | フェーズ 1 のステップ 1-5・1-6 |
| H-06 フラグの組み合わせの規則 | フェーズ 4 のステップ 4-1・4-2 |
| H-07 `--out` の出力の来歴 | フェーズ 2 のステップ 2-2・2-4 とフェーズ 4 のステップ 4-5 |
| H-08 検証・推敲の一覧の保存と `--review-log` の関係 | フェーズ 1 のステップ 1-3（一覧の補助関数）とフェーズ 4（`--review-log` を読み取りの入力にしない） |
| H-09 評価の手順の更新 | フェーズ 5 のステップ 5-5 |
| H-10 保存を既定で行うか | フェーズ 2・4（`--artifacts` を指定したときだけ `Store` を渡す） |
| H-11 古い中間生成物の扱い | フェーズ 1 のステップ 1-6（連鎖の検証）とフェーズ 2 のステップ 2-2 |

### 1.6. 対象外（requirements §2.3）

パイプラインの各段階のプロンプト・モデル・振る舞いの変更（0010）、`--no-shorten`・`--no-review`・`--review-log` の既存の振る舞いの変更、中間生成物の自動削除・世代管理、字幕・info.json のキャッシュの変更、検証・推敲の各項目の判定、評価の再実行と `02_evaluation.md` §3 の記録の更新（フェーズ 6）、ブラウザ拡張・localhost サーバ、#186・#185 への対処は、本タスクの対象外である。当初案の `--article <path>`・`--review-only` は設けない（requirements F-005）。

### 1.7. 横断検索チェックリスト

本タスクは既存の名前を変更・削除しない純粋な追加であり、新しい名前は `Artifact`・`artifact` の接頭辞を持つか `internal/writer`・`internal/config` の中に閉じる。`make lint`・`make test` で検出できない残存参照・名前の衝突・用語のずれは想定されないので、横断検索チェックリストは設けない。ただし、`--artifacts`・`--after` の文書（README・`project_overview.md`・`security.md`・`02_evaluation.md`）と `package_reference.md` の整合は、フェーズ 5 の実行される guard（`cmd/yt2column/docs_test.go`）で確かめる（§5）。

## 2. 実装ステップ (Implementation Steps)

### 2.0. ブロッカー（フェーズ 1 の着手前に解消する）

承認時の architecture §3.4 は、成果物を `O_NOFOLLOW` を付けて `os.Root.OpenFile` で開けばシンボリックリンクを辿らないとしていたが、これは `go1.27.1`（darwin）では成り立たない（§1.3「architecture を訂正した箇所」）。`os.Root` の解決は `O_NOFOLLOW` を無視し、ルート内を指す相対シンボリックリンクを辿る。

architecture §3.4 を次の形に訂正し、`Status` を `draft` に戻した（`requirements_process.md` の「Editing an approved document」の decision change）。本書のステップ 1-6・1-7、§3 の AC-11 の行、§6.1 のリスクは、訂正後の architecture と同時に更新しており、両文書は同じ機構を述べる。

- **読み込み:** `os.Root.Lstat` で名前が通常のファイルであることを確かめ、`O_RDONLY|O_NONBLOCK` で開き、開いた記述子の `Stat` で通常のファイルであること・`Lstat` の結果と `os.SameFile` で一致すること・大きさが上限以下であることを確かめる。通常のファイルでない場合と一致しない場合は `ErrMalformed`（architecture §4.1 に行を加えた）。
- **保存:** 一時ファイルは `O_CREATE|O_EXCL` で作る（`O_NOFOLLOW` に頼らない。`O_EXCL` は既存のシンボリックリンクの名前を拒否する）。置き換えは `os.Root.Rename` で、置き換え先のシンボリックリンクそのものを置き換える。
- **ディレクトリ:** `os.Lstat` でシンボリックリンクでないディレクトリであることを確かめてから `os.OpenRoot` で開き、`os.Root.Stat(".")` が `Lstat` の結果と `os.SameFile` で一致することを確かめる。

architecture が再承認されるまで、フェーズ 1 に着手しない。

### フェーズ 1: `internal/writer` の成果物の型と `internal/artifact` パッケージ

**対象ファイル:** `internal/writer/artifact.go`（新規）、`internal/writer/output.go`（変更）、`internal/writer/review.go`（変更）、`internal/writer/template.go`（変更）、`internal/artifact/artifact.go`・`format.go`・`store.go`（新規）、`internal/artifact/format_test.go`・`store_test.go`・`chain_test.go`（新規）、`docs/dev/developer_guide/package_reference.md`（変更）

- [ ] **ステップ 1-1**: `internal/writer/artifact.go` を新設し、`Stage`（`StageUnset`・`StageGeneration`・`StageShorten`・`StageReview`）・`TemplateIdentity`・`StageRecord`・`ArtifactChain`・`ArtifactStore` インタフェース・`ArtifactError`（`Error`・`Unwrap`）・`ArtifactOptions` を architecture §3.1・§3.5 の形で定義する。成果物の識別子の共通計算 `Digest(parts ...string) string`（値の長さを表す 10 進の文字列 + `:` + 値の連結の SHA-256）と、素材の識別子 `MaterialID(t transcript.Transcript) string`（動画 ID と `newTemplateData` の 4 つの値）を置く（architecture §3.2）。`Step` の列挙（`errors.go:83-87`）に `StageGeneration` が無いので、`ArtifactError` は `StepError` とは別に段階名を持つ（architecture §4.1）。
- [ ] **ステップ 1-2**: `internal/writer/output.go` に `CheckStageText(title, body string) error` と `CheckModelText(name, value string) error` を加え、`checkGeneratedText`・`checkModelString` を呼ぶ。`CheckStageText` は `textString` を作って `checkGeneratedText` に通し、返ったタイトルと本文が引数と完全に一致することを確かめる（一致しなければ `ErrMalformedOutput`。正規化された値を検査しておきながら、正規化されていない保存値を使う、という食い違いを防ぐため）。`checkGeneratedText` の規則と `maxTextBytes`・`maxModelBytes` は変えない。
- [ ] **ステップ 1-3**: `internal/writer/review.go` の一覧の検査（`parseReviewResponse` `:38-70` の `revisions` の配列から `[]Revision` を作る部分）を公開の補助関数 `CheckRevisionList` に切り出し、`parseReviewResponse` がそれを呼ぶ形にする（`strictjson.Value` の配列を受けて `[]Revision` を返す。`revisionKeys`・`maxRevisions`・`parseRevision` の規則は変えない）。`internal/artifact` が `CheckRevisionList` を呼び、拒否されたら `ErrMalformed` として扱う（architecture §3.2 の H-08）。
- [ ] **ステップ 1-4**: `internal/writer/template.go` の `checkedTemplate[T]`（`:86-88`）に、テキストのダイジェストと埋め込みかどうかの真偽値を持たせ、`loadTemplate`（`:92-102`）と `parseTemplate`（`:145-167`）が読み込み時に設定する。`stepTemplates`（`writer.go:71-84`）と `loadStepTemplates`（`writer.go:174-184`）が識別子を外へ返せるようにする。上書きファイルのパスは記録しない。`parseTemplate` の検査の順序と結果は変えない（`TestPromptsREADMEMatchesContract` の契約は変わらない）。
- [ ] **ステップ 1-5**: `internal/artifact/artifact.go` に番兵 `ErrMissing`・`ErrMismatch`・`ErrMalformed`（architecture §4.1 の文言）と上限 `maxArtifactBytes = 4 MiB` を置く。`internal/artifact/format.go` に、成果物の JSON を `writer.StageRecord` に変換する `Parse` と、`writer.StageRecord` を JSON に組み立てる `Marshal` の両方を書く（保存と再開が同じ定義を読む。1 つの `StageRecord` の往復が一致することをテストする）。構文は `strictjson.ParseObject` で読み、キーと値の規則は architecture §3.2 の表のとおりに検査する。文面は `writer.CheckStageText`、モデル名は `writer.CheckModelText`、一覧は `CheckRevisionList`（ステップ 1-3）で検査する。`id` は内容から `writer.Digest` で再計算して一致を確かめる。`ErrMalformed` は `fmt.Errorf("%w: %w: %s", ErrMalformed, writer.ErrMalformedOutput, reason)` の形で包み、`reason` に成果物の値を含めない（architecture §4.1・§4.2）。
- [ ] **ステップ 1-6**: `internal/artifact/store.go` に `Store`（`Load`・`Save`・`Close`）と `NewStore(dir string) (*Store, error)` を書く。`NewStore` はディレクトリを開き、存在しない・ディレクトリでない・シンボリックリンクである場合を `ErrMissing` で拒否し、ディレクトリは作らず、`Close` は `os.Root.Close` に委譲する。種別は `os.OpenRoot` の前に `os.Lstat` で確かめ、開いた後に `root.Stat(".")` が `Lstat` の結果と `os.SameFile` で一致しなければ、ハンドルを閉じて `ErrMissing` を返す（architecture §3.4 の手順 1）。`Save` は `os.Root.OpenFile`（`O_CREATE|O_EXCL|O_WRONLY`、モード `0o600`。`O_NOFOLLOW` は付けない）で一時ファイルに書き、`Sync` の後 `os.Root.Rename` で `<videoID>.<stage>.json` に置き換え、ディレクトリをベストエフォートで同期する。`Load` は再開する段階から生成の出力まで architecture §3.3 の連鎖を検証する。各成果物は、`root.Lstat` で通常のファイルであることを確かめ（名前が無ければ `ErrMissing`、通常のファイルでなければ `ErrMalformed`）、`root.OpenFile` を `O_RDONLY|O_NONBLOCK` で開き、開いた記述子の `Stat` で通常のファイルであること・`Lstat` の結果と `os.SameFile` で一致すること（一致しなければ `ErrMalformed`）・大きさが `maxArtifactBytes` 以下であることを確かめ、`io.LimitReader` で上限 + 1 バイトまで読む（architecture §3.4 の「読み込み」）。`Lstat` と開く処理の間の置き換えをテストから起こせるように、`Store` に非公開のフック（`beforeOpen func(name string)`。本番では nil で、同じパッケージのテストだけが設定する）を置き、`NewStore` の内部も同じ形のフックを受け取る非公開の関数に委ねる。動画 ID は `transcript.NormalizedVideoURL` で再検証する。その実行で最初の `Load`・`Save` のときに、同じ動画 ID の残った一時ファイルを回収する（`fs.ReadDir(root.FS(), ".")` で一覧する）。gosec がユーザー指定のパスを開く箇所を指摘する場合は、最小のブロックに限った `//nolint:gosec` を根拠のコメント付きで置く（ファイル全体・パッケージ全体には付けない）。
- [ ] **ステップ 1-7**: `internal/artifact/format_test.go`・`store_test.go`・`chain_test.go`（`//go:build test`）に architecture §7.1・§7.3 のテストを書く。`t.TempDir` の実ディレクトリを使い、`NewStore` の直後に `t.Cleanup(store.Close)` を登録する。各拒否の規則（architecture §3.2・§7.1 の規則）を表で並べ、`errors.Is` の判定（`ErrMissing`・`ErrMalformed`・`ErrMismatch`・`writer.ErrMalformedOutput`）を確かめる。上限は上限ちょうどを受理し 1 つ超えを拒否する。`TestLoadRejectsSymlinkAndFIFO` は、ルートの外を指すシンボリックリンクだけでなく、成果物の名前をルート内の別の通常のファイルへ向ける**相対**シンボリックリンク・ディレクトリ・FIFO（ブロックせずに）を `ErrMalformed` で拒否することを確かめる。相対シンボリックリンクの行は、リンク先が単独では受理される正しい成果物であることを先に確かめ、拒否がリンクの検出によるものであることを示す。`TestLoadRejectsSwapAfterLstat` は、`beforeOpen` のフックで `Lstat` の後に名前を相対シンボリックリンクと別の通常のファイルに置き換え、`os.SameFile` の照合で `ErrMalformed` になることを確かめる。`TestNewStoreRejectsBadDir` は、同じ形のフックで `Lstat` の後にディレクトリを別のディレクトリへ置き換えた場合の `ErrMissing` も確かめる。`TestSaveDoesNotFollowSymlink` は、一時ファイルの名前と成果物の名前にシンボリックリンクを置いても、リンク先を作らず書き換えないことを確かめる。連鎖は、素材を変えた場合・前段の識別子を変えた場合・後の段階の成果物を残したまま生成をやり直した場合に `ErrMismatch` になることを確かめる。保存したファイルのモードが `0o600` であること、ディレクトリを作らないことを確かめる。
- [ ] **ステップ 1-8**: `docs/dev/developer_guide/package_reference.md` に `internal/artifact` の行を加える（`TestPackageReferenceListsPackages` が要求する）。`TestFakesCarryBuildTag` のファイル数（`pipeline_test.go:558` の 17）と `TestCommonTypesFieldSets`（`Article`）が変わらないことを確かめる。

**完了条件:** `make fmt`・`make test`・`make lint` が通る。`internal/artifact` のテストが architecture §7.1・§7.3 の検証を満たす。`TestWriterImports`・`TestPackageReferenceListsPackages`・`TestFakesCarryBuildTag` が通る。

### PR-1 作成ポイント: artifact format, identifiers, chain, and store

**対象ステップ**: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8

**推奨タイトル**: `feat(0017): add the artifact format, identifiers, and store`

**レビュー観点**: 成果物の形式と識別子の計算が architecture §3.2 のとおりで、各規則をそれぞれ拒否すること / 連鎖の検証が architecture §3.3 のとおりで、`ErrMissing`・`ErrMalformed`・`ErrMismatch` を `errors.Is` で区別できること / 保存が一時ファイルと `rename(2)` で、`0o600` を使い、`link(2)` を使わず、`--artifacts` のディレクトリを作らないこと / 読み込みがシンボリックリンク（ルート内を指す相対のものを含む）と FIFO を拒否し、`os.Root` のハンドルを基準に読み書きすること / 一覧と文面の検査が 0010 の規則を再利用し、規則を 2 か所に持たないこと / `internal/writer` がネットワークと LLM プロバイダを import しないこと（`TestWriterImports`）/ 拒否のエラーが成果物の値を含まないこと / `Store.Close` が `os.Root` のハンドルを解放し、テストが `t.Cleanup` で閉じること

**実装モデル要件**: frontier-required（`_context.md` の実装モデル要件の区分。以降の `frontier-recommended`・`standard` も同じ）

**判定理由**: 成果物は信頼できない入力であり、その構文・識別子・連鎖・上限・ファイルの種別の検査がこのタスクのセキュリティの中核である（AGENTS.md の security 観点）。形式の全体を 1 か所で決め、保存と再開の両方が同じ定義を読む設計であり、誤ると壊れた入力の受理・識別子の偽装・シンボリックリンクや FIFO による読み書きが生じる。前例のない設計判断が多く、分離された高リスクで込み入ったステップである（`frontier-required` の「前例のない設計判断」と「分離された高リスク／複雑なステップ」に該当）。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 2: `internal/writer` の保存と再開

**対象ファイル:** `internal/writer/writer.go`（変更）、`internal/writer/test_helpers.go`（変更）、`internal/writer/artifact_test.go`（新規）

**前提:** フェーズ 1 の `writer.ArtifactStore`・`StageRecord`・`ArtifactChain`・`ArtifactError` と `internal/artifact` が使えること。

- [ ] **ステップ 2-1**: `writer.Options` に `Artifacts ArtifactOptions` を加え、`templateWriter` に `Store` と `Resume` と識別子を持つ `stepTemplates` を渡す。`New`（`:101-142`）と `validateStepOptions`（`:148-163`）を architecture §3.1 の組み合わせ検査に改める。`Resume` が設定されているのに `Store` が nil の場合は拒否する。段階ごとに、実行する段階が使うクライアント（生成の `LLMClient` は生成と字数の調整の両方が使う）が nil なら拒否し、実行しない段階のクライアントが nil でない場合と、実行しない段階のテンプレートのパスがある場合も拒否する（`nilcheck.IsNil` を使う）。
- [ ] **ステップ 2-2**: `Write`（`:193-240`）を architecture §3.6 の順序に改める。`validateTranscript` の後に素材の識別子を計算し、`Resume` が `StageUnset` でなければ `Store.Load` を呼んで連鎖を得る。段階の実行の有無を `Resume` と `Shorten.Enabled`・`Review.Enabled` から導く（§3.1 の表）。生成・字数の調整・検証・推敲の完了時に、検査に通った出力を `Store.Save` で保存する（公開より前。`Save` の失敗は `*ArtifactError` として段階を示し、記事を公開しない）。`Article` を、実行した段階の結果と再利用した段階の来歴から architecture §3.5 のとおりに組み立てる。各段階の `Generate` と `Store.Load`・`Store.Save` の直前に `ctx.Err()` を確かめる。
- [ ] **ステップ 2-3**: `internal/writer/test_helpers.go`（`//go:build test`）に fake の `ArtifactStore` を加える。`Load` の戻り値（`ArtifactChain` とエラー）と `Save` の記録（動画 ID と `StageRecord`・呼び出しの順）を設定できる形にし、`Save` のエラーを返せるようにする。`testutil/` にファイルを加えない。
- [ ] **ステップ 2-4**: `internal/writer/artifact_test.go`（`//go:build test`、`package writer`）に architecture §7.1 のテストを書く。fake の `ArtifactStore`（`test_helpers.go`）と `llmtestutil.FakeLLMClient`・`FakeLLMSequence`、`test_helpers.go` の `validTranscript()` と `transcript.Transcript` の値を使う（`publishertestutil` は `internal/writer` を import するので `package writer` からは循環になり、`transcripttestutil.FakeTranscriptSource` は `Write` が消費しないので使わない。`TranscriptSource` と `Publisher` の fake を使うのは `cmd/yt2column/run_test.go` である）。保存の読み書きはフェーズ 1 の `internal/artifact` のテストが実物で確かめる。対象は、保存（AC-01・AC-02・AC-18）・段階の完了時の保存（AC-04）・保存の失敗（AC-06・AC-29）・再開（AC-12〜AC-14。呼ばない段階の LLM が呼ばれないことを `len(Calls)` で確かめる）・最終の字数の確認と検証・推敲（AC-16）・`Article` の来歴の組み立て（AC-27）・`New` の組み合わせ検査（§3.1）である。`--artifacts` なし（`Store` が nil）では `Save` が呼ばれないこと（AC-03・AC-05）も確かめる。テンプレートの上書きを使う場合は既存の `writeOverrideFile`（`t.TempDir` を使う）を再利用する。
- [ ] **ステップ 2-5**: `internal/writer`・`internal/job` の既存のテストを内容を変えずに通す（`make test`）。`Article` を `==` で比べるテスト（`steps_test.go:38`、`job_test.go:184`・`:223`）が通ることを確かめる（AC-32）。

**完了条件:** `make fmt`・`make test`・`make lint` が通る。`artifact_test.go` が architecture §7.1 の検証を満たし、既存の `internal/writer`・`internal/job` のテストが内容を変えずに通る。

### PR-2 作成ポイント: writer save and resume

**対象ステップ**: 2-1 / 2-2 / 2-3 / 2-4 / 2-5

**推奨タイトル**: `feat(0017): save and resume pipeline artifacts in the writer`

**レビュー観点**: 段階の実行の有無が `Artifacts.Resume` と `Shorten.Enabled`・`Review.Enabled` だけから導かれ、保存された値から推測しないこと / 保存が各段階の完了時（公開より前）に行われ、後の段階の失敗で前の段階の成果物が残ること / `New` の組み合わせ検査が architecture §3.1 のとおりで、実行しない段階のクライアントを nil にでき、実行する段階のクライアントが nil なら拒否すること / `Article` を変更せず（`TestCommonTypesFieldSets`・`==` 比較が通る）、再利用した来歴が `Model`・`Shorten`・`Review` に入ること / `--after review` でどの LLM も呼ばれないこと

**実装モデル要件**: frontier-recommended

**判定理由**: `Write` の段階の導出・保存の位置・再開の適用・`Article` の組み立ては、architecture §3.5・§3.6 に確定しているが、既存の段階の実行の順序と `ctx.Err()` の確認の置き場所に保存と再開を組み込む込み入った変更である。`New` の組み合わせ検査は誤ると nil のクライアント呼び出しや実行しない段階の設定の黙殺を招く。既存テストを無変更で通すことが完了条件であり、分離された込み入ったステップとして `frontier-recommended` とする。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 3: 設定の読み込み（`Need`・`LoadFor`）

**対象ファイル:** `internal/config/config.go`（変更）、`internal/config/config_test.go`（変更）

- [ ] **ステップ 3-1**: `internal/config/config.go` に `Need`（`Generation bool`・`Review bool`）と `LoadFor(lookup LookupFunc, need Need) (Config, error)` を architecture §3.7 の形で加え、`Load`（`:204-222`）を `LoadFor(lookup, Need{Generation: true, Review: true})` に委譲する。`Need.Generation` が偽のときは `YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL` の未設定を必須の誤りとせず、生成のプロバイダの API キー・`YT2COLUMN_CLAUDE_EFFORT`・`ANTHROPIC_WORKSPACE_ID` を検査しない。`Need.Review` が偽のときは `YT2COLUMN_REVIEW_PROVIDER`・`YT2COLUMN_REVIEW_MODEL` と検証・推敲のプロバイダの変数を検査しない。**`Need.Generation` が偽でも、検証・推敲のプロバイダとモデルの既定を解決するために `YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL` を読む**（`YT2COLUMN_REVIEW_PROVIDER`／`YT2COLUMN_REVIEW_MODEL` が未設定のときの既存の継承規則 `loadReview` `:264-297` を保つ。読むが、生成を実行しないときはその未設定を誤りにしない）。選んだプロバイダの変数だけを読む既存の規則（0009 の F-006）はそのままにする。
- [ ] **ステップ 3-2**: `config_test.go`（`//go:build test`）に `LoadFor` のテストを加える。architecture §3.7 の表の各行（`--after` なし・`--after generation`（`--no-shorten` あり/なし）・`--after shorten`・`--after review`）について、`envLookup`（`:22-27`）の環境で、必須にしない変数が未設定でも通り、実行する段階のプロバイダの変数だけが検査されることを確かめる。継承の場合（`--after shorten` または `--after generation --no-shorten` で `YT2COLUMN_REVIEW_PROVIDER`／`YT2COLUMN_REVIEW_MODEL` が未設定）に、`ReviewProvider` と `ReviewModel` が `YT2COLUMN_LLM_PROVIDER`・`YT2COLUMN_MODEL` から引き継がれ、検証・推敲のプロバイダの API キーが検査されることも確かめる。`Load` を呼ぶ既存のテストが内容を変えずに通ることを確かめる（`TestLoadIgnoresOtherProviderVars` `:380-457`・`TestLoadClaudeIgnoresDeepSeekKey` `:341-375`）。

**完了条件:** `make fmt`・`make test`・`make lint` が通る。`LoadFor` の各行の検査が architecture §3.7 の表のとおりで、`Load` の期待が変わらない。

### PR-3 作成ポイント: scoped configuration loading

**対象ステップ**: 3-1 / 3-2

**推奨タイトル**: `feat(0017): load only the configuration a resumed run needs`

**レビュー観点**: `LoadFor` が architecture §3.7 の表のとおりに、実行する段階のプロバイダの変数だけを検査すること / `Load` が `Need` の両方を真とする `LoadFor` に委譲し、0010 の振る舞い（`--no-review` でも検証・推敲の設定を検査する）を保つこと / 検証・推敲のプロバイダの既定の解決が既存の規則と同じであること / `internal/config` の外に環境変数の読み込みと、API キーなどの秘密情報の変数名が漏れないこと（`envaccess_test.go`）

**実装モデル要件**: standard

**判定理由**: `Need` の形と各行の検査の範囲は architecture §3.7 の表に確定しており、既存の `load*` 関数を `Need` で絞る作業である。前例のない設計判断・panel-mode トリガー・競合する実装案・分離された高リスク／複雑なステップのいずれにも該当しない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 4: CLI のフラグと組み立て

**対象ファイル:** `cmd/yt2column/run.go`（変更）、`cmd/yt2column/run_test.go`（変更）、`README.md`（変更。フラグの表の行だけを加える。他の記述はフェーズ 5）

**前提:** フェーズ 2 の `writer.Artifacts` とフェーズ 3 の `config.LoadFor` が使えること。

- [ ] **ステップ 4-1**: `newFlagSet`（`:132-150`）に `--artifacts <path>` と `--after <stage>` を加え、`cliOptions`（`:113-127`）に `artifacts`・`after` を加える。`--after` の値を受理する 3 つの値への `switch` で検証し、フラグが現れたかどうかは `flagGiven`（`:153-161`）と同じ `flag.FlagSet.Visit` で判定する（「Declare, don't infer」）。
- [ ] **ステップ 4-2**: `run`（`:203-299`）に組み合わせの検査を加え、次の場合を、yt-dlp と LLM を呼ぶ前に `exitUsage`（終了コード 2）で終わらせる（architecture §3.8・AC-30）。`--after` を `--artifacts` なしで指定した（AC-24）。`--after` に 3 つの値以外を指定した（拒否した値をメッセージに含める。AC-25）。`--system-prompt`・`--user-prompt` を `--after` と同時に指定した（AC-26）。`--review-log` を `--after review` と同時に指定した（AC-16）。`--artifacts` のパスがキャッシュディレクトリの中にある（`outPathInsideCacheDir`（`outpath.go:29`）を使う。AC-08）。`--no-shorten`・`--no-review` は `--after` と同時に指定できる（矛盾しない）。
- [ ] **ステップ 4-3**: `run` で、`--after`・`--no-shorten`・`--no-review` から `config.Need` を算出し（architecture §3.7 の表）、`config.Load` の代わりに `config.LoadFor(lookup, need)` を呼ぶ。`--after` なしのときは既存と同じく `Need` の両方を真とする。
- [ ] **ステップ 4-4**: `run`（`:203-299`）で、`--artifacts` が指定されたときだけ `artifact.NewStore(opts.artifacts)` を呼び、成功したら `defer store.Close()` を登録し、失敗したら「実行の失敗」（終了コード 1）で終わる。`buildWriter`（`:357-393`）には開いた `store` を渡し、`writer.Options.Artifacts` に `Store` と `Resume`（`--after` の値）を設定する。`buildWriter` は、生成または字数の調整を実行するときだけ `provider.New` を、検証・推敲を実行するときだけ `provider.NewReview` を呼び、作らないクライアントは nil で渡す。`writer.New` の組み合わせ検査の失敗は、既存どおり `run` が使い方の誤り（終了コード 2）として報告する（`buildWriter` の失敗を「実行の失敗」と混同しない）。
- [ ] **ステップ 4-5**: `reportSuccess`（`:444-464`）に、`--artifacts` を指定した実行で保存した成果物のディレクトリを報告する 1 行を加える。モデルの行の要約（`reportSuccess`・`reportStepSummary` `:469-477`）は `Article` から作られるので変更しない（再開でも再利用した段階の行が出る。AC-27）。
- [ ] **ステップ 4-6**: `run_test.go`（`//go:build test`）に architecture §7.2 のテストを書く。`runEnv`・`newRunEnv`・`seedCache`・`lookupFrom` と、`newWriter`・`newLLMClient`・`newReviewLLMClient` の差し替えの既存の形（`:1277`・`:1289`）を使う。対象は、`--artifacts` で 4 つの成果物のファイルができること（AC-01）・`--after` の各値で呼ばれない段階の LLM が呼ばれないことと `--after review` で LLM が呼ばれないこと（AC-12〜AC-14）・組み合わせの誤り（AC-16・AC-24〜AC-26）・`--artifacts` がキャッシュディレクトリの中の場合に `exitUsage` で終わること（AC-08）・`--artifacts` のディレクトリが無い場合に終了コード 1 で終わること（AC-09・AC-30）・成果物が壊れている場合と識別子が食い違う場合に終了コード 1 で終わること（AC-30）・成果物が無い場合に番兵で終わること（AC-19）・生成の設定を外した再開で通ること（AC-15）・`--out` のモデルの行（AC-27）・再開で実行した段階が保存されること（AC-18）・再開でも字幕の取得とキャッシュを行うこと（AC-17）・`--review-log` のファイルを置いても再開が成果物の一覧を読むこと（AC-07）である。`TestRunHelp`（`:1198-1226`）の一覧に `--artifacts`・`--after` を加える（AC-28）。
- [ ] **ステップ 4-7**: `cmd/yt2column` の既存のテストを内容を変えずに通す（`make test`。AC-32）。
- [ ] **ステップ 4-8**: `README.md` の Usage のフラグの表（`:39-54`。`-h`/`--help` を最後の行に残す）に `--artifacts <path>` と `--after <generation|shorten|review>` の行を加える。`newFlagSet` のフラグを README に求める `TestREADMEDocumentsFlags`（`docs_test.go:37-52`）が、フラグを加えた同じ PR で通るようにする（PR-4 を単独でグリーンゲートを通すため）。

**完了条件:** `make fmt`・`make test`・`make lint` が通る。`run_test.go` の新しいテストが architecture §7.2 の検証を満たし、既存の CLI のテストが内容を変えずに通る。

### PR-4 作成ポイント: CLI artifact flags and assembly

**対象ステップ**: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8

**推奨タイトル**: `feat(0017): add the --artifacts and --after flags`

**レビュー観点**: `--after` の値を 3 つの値への `switch` で検証し、フラグの有無を値ではなく `flag.FlagSet.Visit` で判定すること / 組み合わせの誤りを使い方の誤り（終了コード 2）として yt-dlp と LLM の前に終わらせ、`--artifacts` のディレクトリを開く失敗は実行の失敗（終了コード 1）とすること（AC-30。事前の確認を別に設けず、`run` が `artifact.NewStore` を 1 回呼ぶ）/ `config.Need` の算出が architecture §3.7 の表のとおりで、実行する段階の LLM クライアントだけを作り、作らないクライアントを nil で渡すこと / `--review-log` のファイルを読み取りの入力にしないこと / README のフラグの表に `--artifacts`・`--after` の行を加え、`TestREADMEDocumentsFlags` が通ること

**実装モデル要件**: frontier-recommended

**判定理由**: 終了コードの分け方（引数の形だけで判定できる誤りを 2、ファイルシステムの状態を確かめて分かる誤りを 1）と、`config.Need`・クライアント・`writer.Options` の組み立ては、複数のフラグと段階の実行の有無が絡む込み入った調整であり、誤ると AC の終了コードや設定の検査が崩れる。文献は architecture §3.7・§3.8 に確定しているが、既存の `run` の手順に組み込む分離された込み入ったステップであり、`frontier-recommended` とする。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 5: 文書

**対象ファイル:** `README.md`（変更）、`docs/dev/project_overview.md`（変更）、`docs/dev/security.md`（変更）、`docs/dev/developer_guide/package_reference.md`（変更）、`docs/tasks/0010_post_generation_review/02_evaluation.md`（変更）、`cmd/yt2column/docs_test.go`（変更）、`cmd/yt2column/run_test.go`（変更）

- [ ] **ステップ 5-1**: `README.md` の `## Post-generation steps` のモデルの行と `--review-log` の説明（`:94-99`）と、`## Cache and concurrency`（`:106`）に、成果物の保存と再開（`--artifacts`・`--after`）と、再開でも実行した段階を保存するため `--artifacts` のディレクトリは書き込み可能でなければならないことを加える。フラグの表の行はステップ 4-8 で加え済みである（重複して加えない）。
- [ ] **ステップ 5-2**: `docs/dev/project_overview.md` の `## パイプライン`（`:18`）の `ArticleWriter` の項（`:31-33`）に、段階の出力の保存と再開（`--artifacts`・`--after`）を加える。`## 前提・制約` のキャッシュの項（`:58`）に、成果物はキャッシュの外の利用者が指定したディレクトリに保存され、利用者が削除するまで残ることを加える。
- [ ] **ステップ 5-3**: `docs/dev/security.md` の `## 5. キャッシュディレクトリ`（`:61`）に、成果物のファイルの権限（`0o600`）・保存先（キャッシュの外）・検証済みの動画 ID と固定の段階名から名前を組み立てること・シンボリックリンクを辿らないこと（§2.0 の訂正後の機構）を加える。`## 6. 信頼できないテキストの取り扱い`（`:67`）の `--review-log` の項（`:75`）に倣い、成果物に含めてはならない値（API キー・Webhook URL・共有トークン・上書きファイルのローカルのパス）と、読み込みの境界（未知のキー・重複したキー・末尾の内容・上限・冒頭の U+FEFF の拒否、連鎖の照合）を加える。
- [ ] **ステップ 5-4**: `docs/dev/developer_guide/package_reference.md` の `internal/writer` の行に保存と再開を、`internal/config` の行に `Need`・`LoadFor` を加える。フェーズ 1 で加えた `internal/artifact` の行と整合させる。
- [ ] **ステップ 5-5**: `docs/tasks/0010_post_generation_review/02_evaluation.md` の `## 2. 手順`（`:38`）に、生成を 1 回だけ行って成果物を保存し、同じ生成の出力に別の字数の調整・検証・推敲を当てる手順を加える（`--artifacts`・`--after`）。**加えるコマンドのすべての実行に `--keep-cache` を付ける**（成功した実行の後にキャッシュが既定で削除され、取り直した字幕が変わると次の再開が AC-21 で拒否されるため。requirements AC-17・design_handoff H-09。既存の `### 2.2`・`### 2.3` のコマンド例も `--keep-cache` を使う）。段階の呼び出し回数と料金の記録の方法（再開では呼ばない段階があること）に触れる（requirements F-008・AC-34）。
- [ ] **ステップ 5-6**: `cmd/yt2column/docs_test.go`（`//go:build test`）に、文書の契約を確かめる `TestArtifactDocsContract` を加える。`readDoc`（`:304-311`）・`findTable`（`:368-380`）・`sectionByHeading`（`:455-478`）を使い、README のフラグの表に `--artifacts`・`--after` があること、`--after` が `--artifacts` を要することと `--after review` が `--review-log` を使えないことが README か `-h` の使い方の説明に書かれていること、`project_overview.md` と `security.md` の該当の節に保存と再開・読み込みの境界の記述があること、`02_evaluation.md` の §2 に `--artifacts`・`--after`・`--keep-cache` があることを確かめる。`security.md` が述べる権限（`0o600`）と保存先（キャッシュの外）が、実行されるテスト（`internal/artifact/store_test.go::TestSaveFileMode` と `cmd/yt2column/run_test.go::TestRunArtifactsInsideCacheDir`）が固定する振る舞いと一致することを、`TestArtifactDocsContract` のコメントで対応づける（文書の文言はそれが存在することの確認にとどまり、振る舞いの正否は実行されるテストを基準に判断する）。`TestREADMEDocumentsFlags`（`:37-52`）が通ることを確かめる。
- [ ] **ステップ 5-7**: `02_evaluation.md` に書いた手順のコマンドの**すべての行**（`--artifacts`・`--after`・`--keep-cache`・`--no-shorten`/`--no-review`。最初の実行と各再開の実行の両方）が、yt-dlp と LLM の前に拒否されないことを、フェーズ 4 の fake の組み立て（`run_test.go`）で確かめる（AC-34 の基準となる正解。AC-17 の「再開でもキャッシュを保つ」条件を各行に当てる）。実行するテストは `cmd/yt2column/run_test.go::TestRunEvaluationProcedureFlags` である。

**完了条件:** `make fmt`・`make test`・`make lint` が通る。`TestREADMEDocumentsFlags`・`TestArtifactDocsContract` が通る。`02_evaluation.md` の手順のコマンドの組み合わせが `run_test.go` のテストで受理される。

### PR-5 作成ポイント: documentation and evaluation procedure

**対象ステップ**: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7

**推奨タイトル**: `docs(0017): document saving and resuming artifacts`

**レビュー観点**: README・`project_overview.md`・`security.md` の記述が architecture §3.8・§3.9・§5.2 と整合していること / `security.md` の権限・保存先・名前・シンボリックリンク・読み込みの境界の記述が architecture §5.1・§5.2 のとおりであること / `TestArtifactDocsContract` が実行される guard であり、文書の存在だけでなく `--after` の 2 つの規則と `02_evaluation.md` の手順のコマンドの組み合わせを確かめること / `02_evaluation.md` の手順の**すべての**実行が `run_test.go` のテストで実際に受理されること（基準となる正解）/ `--keep-cache` を**すべての**実行に付ける理由（成功後のキャッシュの削除で取り直した字幕が変わると次の再開が AC-21 で拒否される）が書かれていること / README のフラグの表の行は PR-4 で加え済みで、重複していないこと

**実装モデル要件**: standard

**判定理由**: 文書の更新は architecture §3.9・§5.2 のとおりの内容を既存の節に加える作業であり、基準となる正解の確認はフェーズ 4 の fake の組み立てを再利用したテストで行う。前例のない設計判断・panel-mode トリガー・競合する実装案・分離された高リスク／複雑なステップのいずれにも該当しない。

- [ ] グリーンゲート（`_context.md` の "Green gate" 参照）がパスしていることを確認した
- [ ] PR を作成した
- [ ] PR がマージされた
- [ ] 次のブランチへ切り替えた（次ステップは新しいブランチで作業する）

### フェーズ 6: 評価（対象外）

**対象ファイル:** なし

- [ ] **ステップ 6-1**: この仕組みで評価を切り替えるときは、`02_evaluation.md` §2 の手順（フェーズ 5）を使う。評価の再実行と `02_evaluation.md` §3 の記録の更新は、requirements §2.3 のとおり本タスクの対象外である。実 API の応答は揺れるため、成果物の保存と再開を実 API で通す確認は自動のテストにしない（architecture §7.2）。

**完了条件:** なし（本タスクでは着手しない）。

## 3. 実装の順序とマイルストーン (Implementation Order and Milestones)

### 3.1 マイルストーン

| マイルストーン | このフェーズで作るもの | 対応するフェーズ |
|---|---|---|
| M1: 成果物の形式と読み書きが固まる | `internal/writer` の成果物の型と補助関数、`internal/artifact`（マージ済み） | フェーズ 1 |
| M2: `writer`（`internal/writer`）が保存と再開をする | `writer.Options.Artifacts`・`Write` の保存と再開・fake の `ArtifactStore`・テスト（マージ済み） | フェーズ 2 |
| M3: 実行する段階の設定だけを読む | `config.Need`・`config.LoadFor`（マージ済み） | フェーズ 3 |
| M4: CLI から使える | `--artifacts`・`--after`・組み合わせの検査・組み立て・要約・README のフラグの表の行（マージ済み） | フェーズ 4 |
| M5: 文書と評価の手順が揃う | README の段階の説明・`project_overview.md`・`security.md`・`package_reference.md`・`02_evaluation.md`・`docs_test.go` の guard（マージ済み） | フェーズ 5 |

- フェーズ 1 はフェーズ 2 の前提であり、フェーズ 2・3 はフェーズ 4 の前提である（architecture §8）。フェーズ 1 は他に依存しない。
- フェーズ 1〜5 は architecture §8 の実装優先順位の手順 1〜5 と同じ名前・同じ順序である。フェーズ 6 は requirements §2.3 のとおり対象外である。
- フェーズ 2・3 は互いに独立である。フェーズ 3（`config`）はフェーズ 1・2 に依存しないが、architecture §8 の順序に従いフェーズ 2 の後に置く。

### 3.2 PR 構成

PR はフェーズを単位とする（PR は 5 つ。フェーズ 6 は対象外）。各 PR は単一の関心事（成果物の形式と読み書き / `writer` の保存と再開 / 設定の絞り込み / CLI のフラグ / 文書）を持ち、単独でグリーンゲートを通せる単位とする。

| PR | 対象ステップ | 主な変更内容 | 実装モデル要件 |
|---|---|---|---|
| PR-1 | 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8 | `internal/writer/artifact.go`（新規）・`output.go`・`review.go`・`template.go` と、`internal/artifact` の形式・識別子・連鎖・読み書き・テスト | frontier-required |
| PR-2 | 2-1 / 2-2 / 2-3 / 2-4 / 2-5 | `internal/writer/writer.go` の `Options.Artifacts`・`New` の検査・`Write` の保存と再開・`Article` の組み立て、fake の `ArtifactStore`、`artifact_test.go`（新規） | frontier-recommended |
| PR-3 | 3-1 / 3-2 | `internal/config` の `Need`・`LoadFor` と `config_test.go` | standard |
| PR-4 | 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8 | `cmd/yt2column/run.go` のフラグ・組み合わせの検査・`Need` の算出・`writer.Options` の組み立て・要約、`run_test.go`、README のフラグの表の行 | frontier-recommended |
| PR-5 | 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7 | README の段階の説明、`project_overview.md`・`security.md`・`package_reference.md`・`02_evaluation.md`・`docs_test.go` の guard、`run_test.go` の評価手順のテスト | standard |

PR-1 → PR-2 → PR-4 → PR-5 の順とする。PR-3 は PR-1・PR-2 に依存しないが、architecture §8 の順序（設定を CLI の前）に従い PR-4 の前に置く。`internal/` の変更（PR-1〜PR-3）は、それを使う CLI（PR-4）より先に着地する。README のフラグの表の行は、`TestREADMEDocumentsFlags` を同じ PR で通すため PR-4 に置く（他の文書は PR-5）。

PR-1 は実装（型・形式・読み書き）とテスト（`internal/artifact`）を分けない。成果物の形式と読み書きの契約を観測できるのは `internal/artifact` のテストだけで、テストが契約そのものであるため、実装と同じ PR に置く。

### 3.3 ブランチとコミットの構成

- 各フェーズを 1 つの PR・1 つのコミットにする（スカッシュマージ。`_context.md` の "PR merge method"）。
- PR の境界と実装モデル要件は §3.2 と各 `### PR-N 作成ポイント` のとおりである。

## 4. テスト戦略 (Test Strategy)

### 4.1. ユニットテスト

ユニットテストは LLM の API もネットワークも呼ばない（AC-31）。`internal/writer` のテスト（`artifact_test.go`、`package writer`）は、fake の `ArtifactStore`（`internal/writer/test_helpers.go`、`//go:build test`）と `llmtestutil.FakeLLMClient`・`FakeLLMSequence`、`test_helpers.go` の `validTranscript()` を使う（`publishertestutil` は `internal/writer` を import するので循環になり、`transcripttestutil.FakeTranscriptSource` は `Write` が消費しないので使わない）。`internal/artifact` のテスト（`format_test.go`・`store_test.go`・`chain_test.go`）は、`t.TempDir` に作った実ディレクトリと実ファイルで読み書きする。`internal/config` のテストは `envLookup` の fake を使う。`cmd/yt2column` のテストは fake の `LLMClient`・`TranscriptSource`・`Publisher` と `t.TempDir` を使う。対象と主なテストは architecture §7.1 のとおりで、各 AC との対応は §5 の表に記す。各テストは、対象の仕組みを壊すと失敗することを確かめてからコミットする（CLAUDE.md「Every test must be able to fail for its stated reason」）。

### 4.2. 統合テスト

- **CLI の組み立て（`cmd/yt2column`、fake の `LLMClient`）。** `runEnv` と `productionDeps` の差し替えを使い、`--artifacts` で 4 つのファイルができること・`--after` の各値で呼ばれない段階の LLM が呼ばれないこと・組み合わせの誤りとディレクトリの状態の誤りの終了コードを確かめる（architecture §7.2）。
- **既存のテスト（AC-32）。** `--artifacts`・`--after` を付けない既存の CLI のテストは、引数も期待も変えずに通る。`internal/writer`・`internal/job` の既存のテストは、`Options{}` を使い、`Article` を `==` で比べたまま変更しない。
- **実 API の統合テスト。** 既存の `TestIntegrationCLI`（`cmd/yt2column/integration_test.go`）は変更しない。成果物の保存と再開を実 API で通す確認は、評価（AC-34）の実行で行う（architecture §7.2）。

### 4.3. セキュリティテスト

architecture §7.3 の検査を、フェーズ 1 の `internal/artifact` のテストとフェーズ 4 の `run_test.go` に含める。巨大なファイル・項目の数の超過・未知のキー・重複したキー・不正な UTF-8・対になっていないサロゲート・冒頭の U+FEFF・識別子の食い違い・対応しない形式の版・シンボリックリンクのディレクトリ・成果物の名前の FIFO を拒否すること、FIFO を開いてブロックしないこと、成果物のファイルのモードが `0o600` であること、`--artifacts` のディレクトリを作らないこと、成果物に API キー・Webhook URL・上書きファイルのパスが入らないこと、読み込みの失敗のエラーに成果物の値が入らないことを確かめる。

### 4.4. テストヘルパー

新しい `testutil/` ファイルは追加しない。fake の `ArtifactStore` は `internal/writer/test_helpers.go`（`//go:build test`。`test_organization.md` の分類 B）に加える。`internal/artifact` のテストは同パッケージの `_test.go` で、`//go:build test` を付ける。`TestFakesCarryBuildTag` が数える `testutil/` のファイル数（`pipeline_test.go:558` の 17）は変わらない。

## 5. 受け入れ基準の検証 (Acceptance Criteria Verification)

各 AC の検証を実行する。`test` は、実行され、誤った振る舞いで落ちるテストを指す。`static` は、実行される guard・`make` ターゲット・コミットしたスクリプトを指す。`manual` は、PR での観察を指す。ファイルのパスは `path::TestName` 形式で記す。

| AC | 検証の区分 | 検証 |
|---|---|---|
| AC-01 | test | `cmd/yt2column/run_test.go::TestRunArtifactsSavesAllStages`（`--artifacts` で 4 つのファイルができ、生成の出力・字数の調整の出力・適用後の記事・一覧の値が実行の途中の値と一致する）と `internal/writer/artifact_test.go::TestWriteSavesAllArtifacts` |
| AC-02 | test | `internal/writer/artifact_test.go::TestWriteSavesAllArtifacts`（各 `StageRecord` の `Stage`・`Model`・`ModelVersion`・`System`・`User`・`Input` が実行の値と一致する）と `internal/artifact/format_test.go::TestParseRecordRejects`（来歴の必須のキー） |
| AC-03 | test | `internal/writer/artifact_test.go::TestWriteSavesOnlyRunStages`（字数の調整・検証・推敲を実行しないときは保存しない） |
| AC-04 | test | `internal/writer/artifact_test.go::TestWriteSavesBeforeLaterFailure`（検証・推敲を失敗させても生成と字数の調整の `Save` が呼ばれている） |
| AC-05 | test | `internal/writer/artifact_test.go::TestWriteNoStoreSavesNothing`（`Store` が nil で `Save` が呼ばれない） |
| AC-06 | test | `internal/writer/artifact_test.go::TestWriteSaveFailure`（`Save` のエラーで記事がゼロ値になり、メッセージが段階の名前を含む） |
| AC-07 | test | `cmd/yt2column/run_test.go::TestRunReviewLogNotInput`（`--review-log` のファイルを置いても再開は成果物の一覧を読む）と `internal/artifact/store_test.go::TestSaveLoadRoundTrip` |
| AC-08 | test | `cmd/yt2column/run_test.go::TestRunArtifactsInsideCacheDir`（yt-dlp と LLM を呼ばずに終了コード 2） |
| AC-09 | test | `internal/artifact/store_test.go::TestNewStoreRejectsBadDir`（不在・ファイル・シンボリックリンク）と `cmd/yt2column/run_test.go::TestRunArtifactsBadDir`（yt-dlp と LLM を呼ばずに終了コード 1） |
| AC-10 | test | `internal/artifact/store_test.go::TestSaveLoadRoundTrip`（固定の名前・置き換え）と `::TestSaveOnlyValidatedVideoID`（検証を通らない動画 ID を拒否する） |
| AC-11 | test | `internal/artifact/store_test.go::TestSaveFileMode`（`0o600`）と `::TestLoadRejectsSymlinkAndFIFO`（ルートの外を指すものとルート内を指す**相対**のものの両方のシンボリックリンクと FIFO を拒否する）と `::TestLoadRejectsSwapAfterLstat`（`Lstat` と開く処理の間の置き換えを `os.SameFile` で拒否する）と `::TestSaveDoesNotFollowSymlink`（保存がリンク先を作らず書き換えない）と `::TestNewStoreRejectsBadDir`（ディレクトリを作らない） |
| AC-12 | test | `internal/writer/artifact_test.go::TestResumeAfterGeneration`（生成の LLM が呼ばれず、字数の調整・検証・推敲が実行される）と `cmd/yt2column/run_test.go::TestRunAfterGeneration` |
| AC-13 | test | `internal/writer/artifact_test.go::TestResumeAfterShorten`（生成と字数の調整の LLM が呼ばれない）と `cmd/yt2column/run_test.go::TestRunAfterShorten` |
| AC-14 | test | `internal/writer/artifact_test.go::TestResumeAfterReview`（どの LLM も呼ばれない）と `cmd/yt2column/run_test.go::TestRunAfterReview` |
| AC-15 | test | `internal/config/config_test.go::TestLoadFor`（各行の検査の範囲）と `cmd/yt2column/run_test.go::TestRunResumeConfigScoping`（`--after review` で `YT2COLUMN_MODEL` と API キーが未設定でも通る） |
| AC-16 | test | `internal/writer/artifact_test.go::TestResumeFinalLengthAndReviewLog`（再開でも `Shorten.Enabled` に従う最終の字数の確認と、検証・推敲を実行する再開の `ReviewResult`）と `cmd/yt2column/run_test.go::TestRunAfterReviewRejectsReviewLog`（`--after review` と `--review-log` で終了コード 2） |
| AC-17 | test | `cmd/yt2column/run_test.go::TestRunResumeFetchesTranscript`（再開でもキャッシュの取得を行い、キャッシュが無ければ yt-dlp を呼ぶ。`internal/job`・`internal/pipeline` を変更しない） |
| AC-18 | test | `internal/writer/artifact_test.go::TestResumeSavesExecutedStages` と `cmd/yt2column/run_test.go::TestRunResumeSavesExecutedStages`（再開で実行した段階の成果物が保存される） |
| AC-19 | test | `internal/artifact/store_test.go::TestLoadMissing`（`errors.Is(_, ErrMissing)`）と `cmd/yt2column/run_test.go::TestRunAfterMissingArtifact`（LLM を呼ばず、公開しない） |
| AC-20 | test | `internal/artifact/format_test.go::TestParseRecordRejects`（空・不正な UTF-8・冒頭の U+FEFF・空のタイトルと本文・未知のキー・重複したキー・末尾の内容）と `::TestParseRecordLimits`（ファイル・文面・一覧の上限。上限ちょうどを受理し 1 つ超えを拒否する） |
| AC-21 | test | `internal/artifact/chain_test.go::TestLoadRejectsMismatch`（素材の識別子の食い違い・前段の識別子の食い違い・古い後段の成果物を残したままの再開） |
| AC-22 | test | `internal/artifact/format_test.go::TestParseRecordTextRules`（`errors.Is(err, writer.ErrMalformedOutput)` が真。文面とモデル名の形） |
| AC-23 | test | `internal/artifact/format_test.go::TestParseRecordRejects`・`::TestParseRecordTextRules`（標準ライブラリが黙って修復・破棄・無視する入力を、それぞれの入力だけで拒否する） |
| AC-24 | test | `cmd/yt2column/run_test.go::TestRunAfterRequiresArtifacts`（yt-dlp と LLM を呼ばずに終了コード 2） |
| AC-25 | test | `cmd/yt2column/run_test.go::TestRunAfterInvalidValue`（3 つの値以外で終了コード 2。メッセージに拒否した値） |
| AC-26 | test | `cmd/yt2column/run_test.go::TestRunAfterRejectsGenerationFlags`（`--system-prompt`・`--user-prompt` と `--after` で終了コード 2） |
| AC-27 | test | `internal/writer/artifact_test.go::TestResumeArticleProvenance`（再利用した段階の `Model`・`ModelVersion` が `Article` に入る）と `cmd/yt2column/run_test.go::TestRunAfterShortenOutModelLines`（`--out` のモデルの行に段階の区別が付く形で出る。空の ModelVersion は書かない） |
| AC-28 | static | `cmd/yt2column/run_test.go::TestRunHelp`（`-h`/`--help` の使い方の表示に `--artifacts`・`--after`）と `cmd/yt2column/docs_test.go::TestREADMEDocumentsFlags`（README のフラグの表）と `::TestArtifactDocsContract`（`--after` が `--artifacts` を要すること、`--after review` が `--review-log` を使えないこと） |
| AC-29 | test | `internal/writer/artifact_test.go::TestWriteSaveFailure`（保存の失敗が段階を示し、記事を公開しない）と `internal/artifact/store_test.go::TestLoadMissing`・`::TestLoadRejectsSymlinkAndFIFO`（読み込みの失敗が番兵で判定でき、公開しない） |
| AC-30 | test | `cmd/yt2column/run_test.go::TestRunArtifactExitCodes`（引数の形だけの誤りは 2、ファイルシステムの状態の誤りは 1。`--artifacts` のディレクトリを実際に開く 1 回の確認で判定する。成果物が壊れている場合と入力の識別子が食い違う場合も終了コード 1 になることを含める） |
| AC-31 | test | ユニットテストが LLM の API もネットワークも呼ばない。`internal/writer/artifact_test.go`（fake の `ArtifactStore` と `llmtestutil` の fake。`t.TempDir` はテンプレートの上書きだけ）、`internal/artifact/format_test.go`・`store_test.go`・`chain_test.go`（`t.TempDir` の実ディレクトリ。LLM は呼ばない）、`internal/config/config_test.go::TestLoadFor`（`envLookup` の fake）、`cmd/yt2column/run_test.go`（fake の `LLMClient`・`TranscriptSource`・`Publisher` と `t.TempDir`）。`make test` の実行で確かめる |
| AC-32 | static | `make test` が `--artifacts`・`--after` を付けない既存の CLI・`internal/writer`・`internal/job` のテストを内容を変えずに通す（テストの引数と期待を変えないことを同じ PR の差分で確認する） |
| AC-33 | static / test | `cmd/yt2column/docs_test.go::TestREADMEDocumentsFlags`・`::TestArtifactDocsContract`（README・`project_overview.md`・`security.md` の該当の節の記述の存在）と、記述が述べる振る舞いを固定する実行されるテスト（`security.md` の権限は `internal/artifact/store_test.go::TestSaveFileMode`、保存先は `cmd/yt2column/run_test.go::TestRunArtifactsInsideCacheDir`） |
| AC-34 | static / test | `cmd/yt2column/docs_test.go::TestArtifactDocsContract`（`02_evaluation.md` §2 に `--artifacts`・`--after`・`--keep-cache` があること）と `cmd/yt2column/run_test.go::TestRunEvaluationProcedureFlags`（手順の**すべての**実行が受理されること。基準となる正解） |

- 各フェーズの完了時に `make fmt`・`make test`・`make lint` を実行する（§1.2）。
- AC-32 は「既存のテストを内容を変えない」という差分の性質である。実行される検証は `make test` が既存のテスト（`internal/writer/writer_test.go`・`steps_test.go`、`internal/job/job_test.go`、`cmd/yt2column/run_test.go`）を無変更で通すことで、テストの引数と期待を変えていないことは同じ PR の差分で確認する。

## 6. リスク管理 (Risk Management)

### 6.1. 技術的リスク

| リスク | 影響 | 対策 |
|---|---|---|
| 成果物の読み込みが、標準ライブラリが黙って修復する入力（不正な UTF-8・対になっていないサロゲート・重複したキー・末尾の内容）を受理する | 検査した値と使う値がずれ、AC-20・AC-23 が満たされない | `strictjson.ParseObject`・`CollectOnly`・`AsBool` を再利用し、各入力をその入力だけで拒否するテストを表で並べる（ステップ 1-5・1-7） |
| 成果物の識別子の再計算が、内容の一部（種別のタグなど）を渡し忘れて、異なる一覧が同じ `id` を持つ | 別の実行の成果物を受理する | architecture §3.2 のとおり、タグ（`after`/`delete`）を含む決まった順で `Digest` に渡す。同じ `id` を持ちうる 2 つの一覧を作って拒否されることをテストする（ステップ 1-5・1-7） |
| 連鎖の検証が直前の 1 段で打ち切られ、古い後段の成果物と新しい生成の出力の混在を見逃す | AC-21・H-11 が満たされない | architecture §3.3 のとおり、生成の出力まで入力の識別子を順にたどり、生成の入力が今の素材と一致することを確かめる（ステップ 1-6・1-7） |
| `--artifacts` のディレクトリの種別の検査と読み書きの間にディレクトリが置き換わる | 別の場所を読み書きする | `NewStore` が 1 回開いたハンドル（`os.Root`）を以後の読み書きの基準にする（ステップ 1-6。architecture §3.4） |
| 成果物の名前の FIFO で読み込みがブロックする | 実行が停止する | `root.Lstat` で通常のファイルに限ることを確かめたうえで `O_NONBLOCK` で開く（`Lstat` の後に FIFO へ置き換わってもブロックしない。ステップ 1-6・1-7） |
| `os.Root` がルート内を指す相対シンボリックリンクを辿り、`O_NOFOLLOW` では拒否できない | 成果物の名前のシンボリックリンクを介して別のファイルを読む | §2.0 のとおり訂正した architecture §3.4 に従い、`Lstat` でシンボリックリンクを拒否し、開いた記述子を `os.SameFile` で照合する。`TestLoadRejectsSymlinkAndFIFO` でルート内を指す相対シンボリックリンクを、`TestLoadRejectsSwapAfterLstat` で `Lstat` の後の置き換えを拒否することを確かめる（ステップ 1-6・1-7） |
| `--after generation` で字数の調整が走るのに生成の `LLMClient` が nil で `New` を通る | `runShorten` で nil の呼び出しになる | `New` が、生成または字数の調整を実行し得る（`Resume == StageUnset`、または `Resume == StageGeneration` かつ `Shorten.Enabled`）のに nil なら拒否する（ステップ 2-1。architecture §3.1） |
| `Load` が `LoadFor` の既定の `Need` を変え、0010 の振る舞い（`--no-review` でも検証・推敲の設定を検査する）を変える | 既存の CLI のテストが落ちる | `Load` を `Need` の両方を真とする `LoadFor` に委譲し、`TestLoadIgnoresOtherProviderVars`・`TestLoadClaudeIgnoresDeepSeekKey` を内容を変えずに通す（ステップ 3-1・3-2） |
| `artifact.NewStore` の失敗が使い方の誤り（終了コード 2）として報告される | AC-09・AC-30 が満たされない | `run` で、`NewStore` の失敗を実行の失敗（終了コード 1）として扱う（ステップ 4-4・4-6） |
| フラグを README に書き忘れる | `TestREADMEDocumentsFlags` が落ちる | フラグを加える同じ PR（PR-4）で README のフラグの表に行を加える（ステップ 4-8）。`package_reference.md` の `internal/artifact` の行もパッケージを作るフェーズ 1 で加える（ステップ 1-8） |
| 成果物が記事の本文（信頼できない値）を含むため、読み込みのエラーのメッセージに値が漏れる | AC-20・AC-22・§4.2 が満たされない | 拒否のメッセージは破られた規則だけを示し、成果物の値を含めないテストを書く（ステップ 1-7） |

### 6.2. セキュリティリスク

- 成果物のファイルの権限（`0o600`）・保存先（キャッシュの外）・名前（検証済みの動画 ID と固定の段階名）・シンボリックリンクを辿らないことを、フェーズ 1 のテストと `NewStore`・`Save`・`Load` で確かめる（AC-08・AC-10・AC-11）。
- 読み込みは信頼できない入力として境界で検査し、補正せず、部分受理しない（requirements F-004・§4.2）。成果物の値によって実行する段階を選ばない（`--after` の値だけで決める）。
- 成果物に API キー・Webhook URL・共有トークン・上書きファイルのローカルのパスを含めない（requirements §4.2）。読み込みの失敗のエラーに成果物の値を含めない。

## 7. 実装チェックリスト (Implementation Checklist)

- [x] architecture §3.4 のシンボリックリンクの機構を `Lstat` と `os.SameFile` で拒否する形に訂正した（§2.0）
- [ ] 訂正した architecture の再承認を得た（§2.0 のブロッカーの解消）
- [ ] PR-1 マージ済み（対象ステップ: 1-1 / 1-2 / 1-3 / 1-4 / 1-5 / 1-6 / 1-7 / 1-8）。フェーズ 1 の完了条件を満たす
- [ ] PR-2 マージ済み（対象ステップ: 2-1 / 2-2 / 2-3 / 2-4 / 2-5）。フェーズ 2 の完了条件を満たす
- [ ] PR-3 マージ済み（対象ステップ: 3-1 / 3-2）。フェーズ 3 の完了条件を満たす
- [ ] PR-4 マージ済み（対象ステップ: 4-1 / 4-2 / 4-3 / 4-4 / 4-5 / 4-6 / 4-7 / 4-8）。フェーズ 4 の完了条件を満たす
- [ ] PR-5 マージ済み（対象ステップ: 5-1 / 5-2 / 5-3 / 5-4 / 5-5 / 5-6 / 5-7）。フェーズ 5 の完了条件を満たす
- [ ] §5 のすべての AC の検証の区分（`test`・`static`）が実行され、通る
- [ ] `make fmt`・`make test`・`make lint` がすべてのフェーズの完了時に通る

## 8. 成功基準 (Success Criteria)

- **機能:** `--artifacts` で生成・字数の調整・検証・推敲の出力が保存され、`--after generation|shorten|review` で保存した段階から再開してその後の段階だけを実行できる（AC-01〜AC-18）。再開した段階より前の LLM を呼ばず、その設定を必須にしない（AC-15）。壊れた入力・空の入力・食い違う入力を部分受理せずに拒否する（AC-19〜AC-23）。
- **品質:** `make test`・`make lint` が通り、`internal/artifact` のテストと `internal/config` の `LoadFor` のテストが通る。`--artifacts`・`--after` を付けない既存の実行のテストが内容を変えずに通る（AC-32）。`internal/writer`・`internal/job`・`internal/pipeline`・`internal/publisher`・`internal/llm/provider` の振る舞いを変えていない。
- **セキュリティ:** 成果物のファイルは `0o600` で、キャッシュの外に保存され、シンボリックリンクを辿らない（AC-08・AC-11）。読み込みは信頼できない入力として検査し、エラーに成果物の値・API キー・Webhook URL を含めない（AC-20・AC-22・§4.2）。
- **文書:** README・`project_overview.md`・`security.md`・`package_reference.md` と `02_evaluation.md` §2 を更新し、`cmd/yt2column/docs_test.go` の guard で確かめる（AC-28・AC-33・AC-34）。

## 9. 次のステップ (Next Steps)

- 本書の PR の境界と実装モデル要件（§3.2 と各 `### PR-N 作成ポイント`）に従い、`/runplan 0017` で PR-1 から順に実装する。フェーズ 6（評価の再実行）は本タスクの対象外である。
- 実装中に architecture の記述が実際のコードと合わない箇所が見つかった場合は、本計画を勝手に変えず、`requirements_process.md` の「Editing an approved document」に従って architecture を直す（decision change は再承認、editorial correction は `Comments` に記録して status を保つ）。
