# ADR-0018: サーバは設定を決まった場所の env-file 形式のファイルだけから読み、CLI は環境変数を読む

## 状態

| 項目 | 値 |
|---|---|
| 状態 | `有効` |

## 背景

サーバ（`yt2column-server`）は、LLM と Webhook の設定を環境変数から読んできた。エンドユーザー向けの配布では、サーバを `brew services`（macOS は launchd、Linux は systemd の user unit）で常駐させる（[#574](https://github.com/isseis/yt2column/issues/574)・[#578](https://github.com/isseis/yt2column/issues/578)）。そうして起動したサーバには、シェルで `export` した変数や `.envrc` が届かないので、サーバが自分で設定を読める場所が要る。

設定には API キーと Webhook URL という秘密が含まれる。外部のモジュールは使わない方針である（[CLAUDE.md](../../../CLAUDE.md)「Dependencies」）。同一ユーザーのプロセスは信頼し、他のユーザーは排除する（[ADR-0006](0006-threat-model.md)）。

## 決定

-   **サーバは設定ファイルだけから設定を読み、CLI は今までどおり環境変数だけから読む。** 入口ごとに読む場所を 1 つに決め、どの値が効いているかを曖昧にしない。サーバが環境変数から読むのは、パスを導出する `HOME` と、Go のランタイムが出す HTTP/2 のデバッグログについて警告するための `GODEBUG` だけである。
-   **パスは `HOME` と OS だけから導出する。** macOS は `$HOME/Library/Application Support/yt2column/config.env`、その他の Unix は `$HOME/.config/yt2column/config.env` で、`XDG_CONFIG_HOME` は読まない。ソケットのパスと同じ考え方である（[ADR-0005](0005-server-architecture.md)）。
-   **書式は Docker の `--env-file` の規則を土台にした `KEY=VALUE` で、それより厳格にする。** `#` で始まる行と空行を読み飛ばし、値は最初の `=` より後ろを変換せずに使う。キーは今の環境変数と同じ名前の 6 つに限る。受理する形に合わないもの（`export `、引用符、行末のコメントに見える形、空の値などを含む）は直さずに拒否する。ファイルはシェルとして実行しない。値の検証は環境変数のときと同じ規則を使う（[ADR-0003](0003-llm-config-contract.md)）。受理する形と拒否するものの一覧は要件定義書の F-003 にある。
-   **ファイルそのものを検査し、満たさなければ直さずに起動を止める。** 自分が所有する通常のファイルで、シンボリックリンクではなく、モードがちょうど `0600` で、macOS では拡張 ACL を持たないことを、読むファイルそのものについて確かめる。親のディレクトリは検査しない。ファイルがなければ、パスと雛形を作るコマンドを示して止める。
-   **環境に残った設定の変数は、名前だけ警告する。** 6 つのキーと廃止した LLM の変数がプロセスの環境にあれば、値を読まずに名前を警告し、サーバは止まらない。
-   **雛形はサーバのサブコマンド `yt2column-server config init` で作る。** モード `0600` で作り、既存のものは上書きしない。

この決定の仕様の正本は、[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md)、[project_overview.md](../project_overview.md) の設定の節、[security.md](../security.md) の §2「秘密情報」と §8.2「localhost サーバ」である。

## 採らなかった案

-   **秘密情報を Keychain（Linux は libsecret）に置く。** 外部コマンドの呼び出しと OS ごとの実装が要る。平文のファイルが問題になったときに検討する（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **launchd の plist の `EnvironmentVariables` に書く。** `brew upgrade`・`brew services restart` で上書きされる（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **起動用のスクリプトで `.env` を `source` する。** 設定がシェルとして実行され、検証をすり抜ける（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **拡張の設定画面から送る。** API キーがブラウザを通る（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **TOML で書く。** 外部のモジュールが要り、標準ライブラリだけで書く方針を変えることになる。値は 6 つの平らな文字列で、TOML の表や型は要らない（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **JSON で書く。** コメントを書けないので雛形で各キーを説明できず、手で書くと誤りやすい（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **`export`・引用符を受け付けて取り除く。** 拒否せずに直すことになり、`.envrc` から写した誤りが見えなくなる（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **ファイルがなければ、空のファイルとみなして既定値で読む。** API キーがないので結局止まるが、ファイルがないことが利用者に分かりにくい（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **環境に残った変数でサーバを止める。** CLI のために `.envrc` を読み込んだシェルから、サーバを起動できなくなる（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **行末のコメントを値の一部として受け付ける（Docker の規則のまま）。** `KEY=value # comment` が書式も値の検証も通り、最初の LLM の呼び出しで初めて誤りが分かる（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **macOS の拡張 ACL を検査しない（ADR-0006 のソケットの祖先と同じ扱い）。** 秘密を含むファイルでは、モードの検査の意味が失われる（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **環境に残った変数を黙って無視する。** 環境変数を設定したのに効かない理由に、利用者が気づけない（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **サーバがファイルと環境変数の両方を読む。** どちらの値が効いているかが曖昧になる（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **雛形を別のコマンドで作る。** 配布するバイナリが増える（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。
-   **親のディレクトリも検査する。** 読むファイルそのものを検査するので、親の名前を入れ替えられても検査していないファイルは読まない。サーバは設定ファイルの親に何も作らないので、親を検査して守るものがない（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md) §1）。

## 結果

-   サーバは、シェルの環境を受け継がない起動（`brew services` など）でも同じ設定で動く。#578 の `service` ブロックは環境変数を渡さなくてよい。
-   CLI とサーバで設定の置き場所が分かれる。同じ API キーを `.envrc` と設定ファイルの 2 か所に書くことになる。
-   秘密は平文のファイルにあり、同一ユーザーのプロセスは読める。今の `.envrc` と同じ水準として受け入れる。
-   設定のキーを足すときは、ファイルが受け付けるキーと警告する変数の一覧の両方に足す。
-   今の利用者は、更新後に設定ファイルを作るまでサーバを起動できない。互換の仕組みは設けず、README の移行の手順で移る。

## 経緯

- 2026-11-06 サーバは決まった場所の env-file 形式の設定ファイルだけから設定を読み、CLI は環境変数を読み続けることにした（[0066 要件定義書](../../tasks/0066_server_config_file/01_requirements.md)）
