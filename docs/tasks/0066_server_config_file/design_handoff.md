# 設計への申し送り：サーバが設定ファイルを読むようにする

要件定義（[01_requirements.md](01_requirements.md)）の作成とレビューで挙がった、設計で決める事項。`02_architecture.md` は、各項目にどう対応したか、または対応しない理由を記録する。

## H-01: 検査したファイルと読んだファイルを同じにする手段

F-004 は、検査したファイルと読んだファイルが同じであることを求める。パスで `Lstat` してから `Open` すると、その間にパスの指すものを入れ替えられる。`O_NOFOLLOW` で開き、開いたファイル記述子に `Fstat` して種類・所有者・モードを確かめ、同じ記述子から読む、などの手段を決める。FIFO を開いて待ち続けないこと（AC-16）も、開き方で決まる（`O_NONBLOCK` で開いてから通常のファイルであることを確かめる、など）。大きさの上限（64 KiB）を超えるファイルを全体を読まずに拒否する方法（上限 + 1 バイトまで読む、など）も決める。

## H-02: 環境とファイルの `LookupFunc` の組み立て

サーバは、`HOME` と `GODEBUG` を環境から、6 つのキーをファイルから読む（F-002）。今の `config.LoadServer` は 1 つの `LookupFunc` から、LLM の変数・`SLACK_WEBHOOK_URL`・`HOME`・`GODEBUG`・廃止した変数を読む（`internal/config/config.go:362-383`）。ファイルの `LookupFunc` と環境の `LookupFunc` を分けて渡すか、`LoadServer` の引数の形を変えるかを決める。廃止した変数の検査は削除する（H-07）。CLI の `Load`・`LoadFor` の振る舞いは変えない（AC-10）。`main.go` の「`os.LookupEnv` を渡すのはここだけ」という契約（`cmd/yt2column-server/main.go:18-20`）を保つかも決める。

## H-03: 拒否の型と番兵、メッセージの語

今の `ErrMissing`・`ErrInvalid` の文は「environment variable ...」である（`internal/config/config.go:174-179`）。AC-05・AC-06 は、ファイルのキーの拒否がそれを設定ファイルのキーとして示し、環境変数と呼ばないことを求める。CLI の環境変数の拒否の文を保ったまま、サーバの拒否の文をどう変えるかを決める。要件 4.2 の拒否の分類ごとの番兵のエラーの識別子と、終了コードの選び方（今の `run` は失敗した段階で選ぶ。`cmd/yt2column-server/run.go` の先頭の注釈）を決める。メッセージは値もファイルの中身も含まない（AC-13）。

## H-04: 書式の拒否をいくつ報告するか

AC-13 は、拒否した行のうち少なくとも最初の 1 行を示すことを求める。最初の 1 行で止めるか、すべて（または上限まで）を示すかを決める。値の検証の拒否（AC-05）は今までどおりすべてまとめて報告する。書式の拒否があるファイルについて、値の検証まで進むかも決める（AC-12 は、拒否したファイルの値をどれも使わないことを求める）。

## H-05: 雛形の作り方と文面

`config init` が、umask によらずモード `0600` で作り（AC-21）、既存のものを変えず（AC-24）、書きかけを残さない（AC-25）手段を決める（`O_CREATE|O_EXCL` で開いてから `Fchmod`、または同じディレクトリの一時ファイルに書いてから既存のものを上書きしない形で置く、など）。親のディレクトリを `0700` で作る範囲（AC-23）も決める。雛形の文面（英語）を決め、雛形がサーバの書式の検査を通ること（AC-26）をテストで確かめる方法を決める。

## H-06: ソケットのパスとの規則の共有

設定ファイルのパスは、ソケットのパスと同じく `HOME` と OS から導出する（F-001）。macOS ではソケットのディレクトリの親（`$HOME/Library/Application Support/yt2column`）が設定ファイルの親と同じになる。`internal/config/socketpath.go` の OS ごとの規則（`socketRule`）と共有するか、別に持つかを決める。`serversock` はソケットのディレクトリの親を検査し、なければ `0700` で作る（`internal/serversock/serversock.go:167-185`）。`config init` が作るディレクトリが、その検査を通ることを確かめる。

## H-07: 廃止した変数の検査の削除

CLI とサーバの両方から、廃止した 5 つの変数の検査（`retiredLLMVars`・`retiredLLMErrors` とその理由の文、`internal/config/config.go:515-524` ほか）を削除する（F-002）。それを確かめていたテストの削除は、`CLAUDE.md`「Testing Strategy」の「Deleting a test is a claim that must be checked」に従い、削除だけのコミットと `make coverage-compare` で確かめる。廃止した変数を文書から削除したときに不要になる文書のガードテスト（README の移行の節などを読むもの）も洗い出す。

## H-08: 文書のガードテスト

AC-34 で、どのテストがどの値（設定ファイルのパス、`config init` の終了コード、6 つのキーの名前、大きさの上限）を、どの文書（`README.md`・`README.en.md`・`project_overview.md`・`security.md`・`development.md`）と照らすかを決める。今のテスト（`cmd/yt2column-server/docs_test.go` の `TestServerDocsContract`、`cmd/yt2column` の設定の表の検査など）が読む表や節の形が変わるので、その更新も決める。`CLAUDE.md` の文書のテストの方針（値・構造・実行できる例だけを確かめ、同じ値を複数のテストで確かめない）に従う。`development.md` の開発用のサーバの手順は、実行できる例として確かめるかも決める。

## H-09: 標準ライブラリの落とし穴と、秘密の変数の名前を書ける場所

-   `bufio.Scanner` の `ScanLines` は行末の `\r` を黙って取り除くので、CR の拒否（F-003）がすり抜ける。既定のトークンの上限（64 KiB）では、65,536 バイトの 1 行が書式の拒否ではなく `bufio.ErrTooLong` になる。行の分け方を決める。
-   `internal/config/envaccess_test.go` は、環境を読む関数を呼べるファイルと、秘密の変数の名前を書けるファイルを制限している。ファイルの解析をどのファイルに置くかを、この制限と合わせて決める。
-   拒否と案内の行に書くパスは `HOME` を含む。今の `stderrWriter`（`redact.Line`）を通して書くことを確かめる。

## H-10: macOS の拡張 ACL の検出と、`config init` の ACL

AC-18a は macOS で拡張 ACL を持つファイルの拒否を、AC-21 は `config init` が ACL を持たないファイルを作ることを求める。外部のモジュール（`golang.org/x/sys` を含む）と外部のコマンドを使わずに、標準ライブラリだけで ACL の有無を確かめ、継承した ACL を持たないファイルを作る手段を決める。標準ライブラリだけでは確かめられないと分かったときは、設計を進めずに要件の見直しを求める。
