# ADR-0047: 記事を拡張だけで作り、localhost サーバと Native Messaging のホストを廃止する

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

拡張を Chrome Web Store で公開することを見据えている。拡張から Native Messaging のホストを経て localhost サーバで記事を作る構成は、利用者にネイティブのバイナリの導入、ホストの manifest の登録、サーバの起動を求めるので、導入の障壁が大きい。

前提が 3 つある。品質の目標は `claude-opus-5-5` を要するので（[production_profile.md](../production_profile.md)）、ブラウザの中で推論する形は採れない。Go の WASM 版のパイプラインは、拡張のページから Claude API を呼んで記事を完成させられ、本番の構成の 3 本は背面のタブのまま完成した（[調査記録](../../research/wasm_pipeline_poc/README.md)「3 つの問いへの答え」）。利用者は、今は開発者 1 人である。

## 決定

-   **記事の生成は拡張だけで行い、localhost サーバと Native Messaging のホストを廃止する。** 拡張のページから LLM の API を直接呼ぶ。利用者にネイティブのバイナリの導入を求めないためである。規則は [0090 要件定義書](../../tasks/0090_extension_only_architecture/01_requirements.md) の F-001 にある。
-   **利用者は、自分の Anthropic API キーを拡張に設定する。** 開発者は、API キーも、API を中継するサーバも提供しない。サーバを廃止した後に、API を呼ぶ権利を持つのは利用者だけだからである。API キーの保存は [ADR-0010](0010-extension-stores-nothing.md) が定める。規則は 0090 要件定義書の F-001 にある。
-   **拡張が使うプロバイダは Claude だけとし、3 つの工程とも本番の構成に固定する。** 拡張の設定は、API キーと任意のワークスペース ID だけである。品質の目標が本番の構成を要し、設定の画面と権限を最小にできるためである。将来ほかのプロバイダを選べるようにすることを、妨げない構成にする。ただし、そのために今の構成を複雑にはしない。本番の構成は [ADR-0004](0004-production-profile.md) が定める。規則は 0090 要件定義書の F-001 にある。
-   **生成のジョブの寿命は、それを実行する拡張のページの寿命とする。** ブラウザやそのページを閉じれば、生成は止まる。サーバの寿命が効くのは、生成を頼んだ後にブラウザを閉じ、起動し直して同じページで頼み直す経路だけで、現実には起きにくいためである。完成した記事をいつまで持つかは [ADR-0027](0027-article-job-api.md) が定める。規則は 0090 要件定義書の F-001 にある。
-   **CLI は開発用の経路として残し、記事を作る処理を拡張と共有する。** CLI は、今までどおり設定を環境変数から読み、3 つのプロバイダを選べる。LLM の呼び出しと記事の作り方を、拡張と別に開発して確かめられるようにするためである。設定の値の文法は [ADR-0003](0003-llm-config-contract.md) が、環境変数を読む場所は [ADR-0024](0024-env-access-in-config.md) が定める。規則は 0090 要件定義書の F-001 にある。
-   **新旧の経路を併存させず、一括で切り替える。** 版の食い違いを見分ける仕組みと、移行のための互換の仕組みは設けない。利用者が開発者 1 人で、互換の仕組みの費用に見合わないためである。規則は 0090 要件定義書の F-001 にある。
-   **構成は、Chrome Web Store で公開できる形にする。** 導入の障壁を下げる目的が、公開できなければ達せられないためである。構成を縛る条件は、[#772](https://github.com/isseis/yt2column/issues/772) の設計で決める。規則は 0090 要件定義書の F-002 にある。

この決定は、まだコードと仕様の文書（[security.md](../security.md)・[project_overview.md](../project_overview.md)・README・[development.md](../guides/development.md)・[package_reference.md](../conventions/package_reference.md)・[CLAUDE.md](../../../CLAUDE.md)）に反映していない。サーバとホストは今も動き、拡張は API キーを持たない。この差分は [#773](https://github.com/isseis/yt2column/issues/773) の実装で解消する。

[ADR-0006](0006-threat-model.md)・[ADR-0008](0008-launch-path.md)・[ADR-0009](0009-viewer-opening.md)・[ADR-0026](0026-extension-platform.md)・[ADR-0028](0028-no-automatic-resend.md)・[ADR-0031](0031-secret-closure.md) と、ADR-0010・ADR-0027 のうち 2026-11-12 に改めなかった項目は、サーバとホストを前提にしたままである。置き換える決定は設計で決まるので、これらは [#772](https://github.com/isseis/yt2column/issues/772) の設計の PR で改める。

## 採らなかった案

-   **ブラウザの中で推論する。** 品質の目標が `claude-opus-5-5` を要し、ブラウザの中では動かせない（[#769](https://github.com/isseis/yt2column/issues/769)）。
-   **ホストとサーバの構成を保つ。** 利用者にネイティブのバイナリの導入、ホストの manifest の登録、サーバの起動を求めることが変わらない（[#769](https://github.com/isseis/yt2column/issues/769)）。
-   **拡張でモデルと effort を選べるようにする。** 設定の画面と検証が増え、品質の目標を満たさない構成を利用者が選べるようになる（0090 要件定義書「要件定義の際に決めたこと（2026-11-12）」）。
-   **拡張で複数のプロバイダを選べるようにする。** ブラウザから直接呼べることを確かめたのは Claude だけで、送り先ごとに権限が要る。将来の追加は妨げない（0090 要件定義書「要件定義の際に決めたこと（2026-11-12）」）。
-   **新旧の経路を併存させる。** 利用者が開発者 1 人で、互換の仕組みの費用に見合わない（[#769](https://github.com/isseis/yt2column/issues/769)）。

## 結果

-   利用者は、拡張の導入と API キーの設定だけで記事を作れる。
-   生成の途中でページを閉じると、その生成は失われる。中止までの分の料金はかかる（[調査記録](../../research/wasm_pipeline_poc/README.md)の測定 4）。
-   拡張が API キーを持ち、ネットワークへ送るようになる。脅威モデル、権限、Content Security Policy は、この構成に合わせて改める必要がある。
-   拡張では、本番の構成のほかのモデルを選べない。ほかのモデルやプロバイダを試すのは CLI である。
-   `cmd/yt2column-server`・`cmd/yt2column-nmhost` と、サーバだけが使うパッケージは、[#773](https://github.com/isseis/yt2column/issues/773) で撤去する。

## 経緯

- 2026-11-12 記事の生成を拡張だけで行い、localhost サーバと Native Messaging のホストを廃止することにした。拡張が使うプロバイダは Claude だけで本番の構成に固定し、利用者が自分の API キーを設定する。CLI は開発用の経路として残す（[0090 要件定義書](../../tasks/0090_extension_only_architecture/01_requirements.md)）
