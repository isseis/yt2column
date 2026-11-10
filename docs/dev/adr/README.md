# ADR の索引

この索引は、主要な設計の決定を記録した ADR（Architecture Decision Record）をすべて挙げ、今どの決定が効いているかを示す。各 ADR は、主題ごとに、今効いている決定、その理由、採らなかった案、タスクをまたぐ経緯を持つ。設計を始める前に、ここから関係する ADR を探して読む。

状態が `有効` の ADR が、今効いている決定である。`廃止` の ADR は、主題が終わった決定の記録として残す。ADR の書き方、番号の付け方、状態の意味、どの決定を主要とするかは、規約 [docs/dev/conventions/adr.md](../conventions/adr.md) にある。ADR を足すか状態を変える PR は、同じ PR でこの表を直す。

| 番号 | 題名 | 状態 |
|---|---|---|
| [ADR-0001](0001-llm-adapter-stdlib.md) | LLM のアダプタは SDK を使わず標準ライブラリで書き、共有の型と送信の経路をプロバイダの外に置く | `有効` |
| [ADR-0002](0002-post-generation-steps.md) | 生成の後に字数の調整と検証・推敲の 2 つの工程を置き、検証・推敲は直す箇所の一覧だけを返す | `有効` |
| [ADR-0003](0003-llm-config-contract.md) | LLM のプロバイダ・モデル・effort を 1 つの値 <プロバイダ>/<モデル ID>+<effort> で指定する | `有効` |
| [ADR-0004](0004-production-profile.md) | 本番の構成を 3 つの工程とも claude-opus-5-5（effort high）とし、品質の目標と製品の制約を共通の文書に置く | `有効` |
| [ADR-0005](0005-server-architecture.md) | サーバを Native Messaging のホストと常駐サーバに分け、HTTP を Unix ドメインソケットの上で受ける | `有効` |
| [ADR-0006](0006-threat-model.md) | ブラウザ拡張と localhost サーバの脅威モデルは、同一ユーザーを信頼し、他のユーザーを排除する | `有効` |
| [ADR-0007](0007-manual-transcript-copy.md) | 字幕は利用者が文字起こしパネルから手動でコピーし、拡張はページを読むだけにする | `有効` |
| [ADR-0008](0008-launch-path.md) | 拡張の起動の経路をツールバーのポップアップだけにし、生成の開始は利用者のボタンの押下だけで行う | `有効` |
| [ADR-0009](0009-viewer-opening.md) | ビューアは生成の開始と同時に背面のタブで開き、ポップアップは閉じずに状態を表示する | `有効` |
| [ADR-0010](0010-extension-stores-nothing.md) | 拡張は何も保存せず、権限も加えず、状態の正しい値をサーバに置く | `有効` |
| [ADR-0011](0011-webhook-publishing.md) | 投稿先は Slack 互換の Incoming Webhook とし、投稿は利用者の操作でだけ行う | `有効` |
| [ADR-0012](0012-lightweight-check-in-bash.md) | 軽量プロセスの確かめ直しは、origin/main の版の bash スクリプトで行い、Go のツールにしない | `廃止` |
| [ADR-0013](0013-review-setup.md) | PR レビューは作成時の 1 回だけ Codex に頼み、その後の変更はローカルの CLI レビューで見る | `有効` |
| [ADR-0014](0014-deterministic-checks-in-go.md) | 決定的な判定はエージェントに任せず、テストのある Go の開発ツールに置く | `有効` |
| [ADR-0015](0015-untrusted-review-threads.md) | レビューのスレッドと Bot への依頼は信頼せず、書き込み権限・範囲の規則・interaction limits で守る | `有効` |
| [ADR-0016](0016-document-test-policy.md) | 文書のテストは値・構造・実行できる例だけを確かめ、文の意味は PR のレビューで読む | `有効` |
| [ADR-0017](0017-viewer-markdown-rendering.md) | ビューアは記事を自前の最小限のレンダラーで描画し、HTML を解釈せず、リンクは https の自動リンクだけにする | `有効` |
| [ADR-0018](0018-server-config-file.md) | サーバは設定を決まった場所の env-file 形式のファイルだけから読み、CLI は環境変数を読む | `有効` |
| [ADR-0019](0019-pipeline-component-interfaces.md) | パイプラインの構成要素を interface にし、構成要素ごとに型を持つパッケージを分け、番兵はプロバイダに依存するかで置き場所を分ける | `有効` |
| [ADR-0020](0020-strict-json.md) | 厳格な JSON の部品を internal/strictjson に集め、検証済みの値だけを扱う不透明な型にし、負でない整数の規則を 1 つにする | `有効` |
| [ADR-0021](0021-llm-client-contract.md) | LLMClient の契約は生成テキストにモデル名・モデルの版・使用量を添えて返し、打ち切りなどの失敗でもモデル名と使用量を返す | `有効` |
| [ADR-0022](0022-cost-estimate.md) | 料金の概算を internal/pricing に置き、単価の表から整数で正確に計算し、分かったか不明かを型で区別する | `有効` |
| [ADR-0023](0023-artifact-resume.md) | 中間生成物を工程ごとに保存し、壊れた中間生成物を拒否しながら後の工程から再開できるようにする | `有効` |
| [ADR-0024](0024-env-access-in-config.md) | プロセスの環境変数を設定として読むのは internal/config と決めた入口だけに限る | `有効` |
| [ADR-0025](0025-claude-output-limit.md) | Claude アダプタの出力の上限の既定は 40,000 トークンとし、ストリーミングを使わない | `有効` |
| [ADR-0026](0026-extension-platform.md) | ブラウザ拡張は TypeScript の Manifest V3 の拡張とし、バンドラを使わず、manifest の key で ID を固定し、Chrome と Brave だけを対象にする | `有効` |
| [ADR-0027](0027-article-job-api.md) | 拡張とホストとサーバはジョブの API で生成と投稿を分け、記事をプロセスごとの秘密によるキーでサーバのメモリに持つ | `有効` |
| [ADR-0028](0028-no-automatic-resend.md) | 結果が分からないときは自動で送り直さず、投稿の操作を冪等にする | `有効` |
| [ADR-0029](0029-extension-ui-languages.md) | 拡張の UI を英語と日本語の 2 言語にし、表示の言語は resolveLocale の 1 か所で決める | `有効` |
| [ADR-0030](0030-minimum-chrome-version.md) | 拡張の minimum_chrome_version を 110 にする | `有効` |
| [ADR-0031](0031-secret-closure.md) | 秘密を secret.Secret 型のクロージャで持ち、送信先へのリクエストを除くどの出力にも流さない | `有効` |
| [ADR-0032](0032-testdata-license.md) | testdata に置く実データは再配布できるライセンスの動画のものに限り、取得した人を特定できないようにする | `有効` |
| [ADR-0033](0033-transcript-cache-slots.md) | 字幕のキャッシュを 2 つのスロットとポインタで持ち、記事を出した後は既定で消す | `有効` |
| [ADR-0034](0034-template-allowlist.md) | プロンプトのテンプレートで使える構文を許可リストで絞り、記事の出典のリンクは LLM ではなくコードで付ける | `有効` |
| [ADR-0035](0035-language-policy.md) | 文書の言語は読み手と承認者で決め、README だけを日英の 2 言語で同期し、PR のタイトルは日本語、squash の件名は英語にする | `有効` |
| [ADR-0036](0036-task-document-status-records.md) | タスク文書の Comments には今の状態の理由だけを書き、状態の遷移は変更履歴に、編集上の訂正は PR の説明とコミットに記録する | `有効` |
| [ADR-0037](0037-test-deletion-coverage.md) | テストを削除したことは、削除だけのコミットの前後のカバレッジの比較で確かめ、その出力をコミットメッセージと PR の説明に残す | `有効` |
| [ADR-0038](0038-deadcode-and-race-free-timing.md) | make deadcode はモジュール全体とテストから届かない関数を報告して失敗し、パースの時間の期限は -race なしのビルドだけで確かめる | `有効` |
| [ADR-0039](0039-adr-writing-criteria.md) | ADR の背景には決定が答える問題と前提だけを、決定には守るべき規則を理由と正本の場所とともに書き、決定を変えない直しは編集上の訂正として認める | `有効` |
| [ADR-0040](0040-simple-task-documents.md) | 小さな変更のタスクは、人の判断で 3 文書を簡易な内容にして 1 本の PR で作れるようにし、機械的な判定は設けない | `有効` |
| [ADR-0041](0041-openai-api.md) | OpenAI は Responses API を応答の保存なしで呼び、出力上限は実測で決める | `有効` |
